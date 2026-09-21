package config

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

const r3105TestSigningKey = "test-signing-key-32bytes!!"

// buildR3105Export 构造带完整 OIDC 参数/Secret 的 v2 导出文件，oidc_configured 与本地登录开关可控。
func buildR3105Export(t *testing.T, allowLocal, oidcConfigured string) []byte {
	t.Helper()
	ctx := context.Background()
	source, svc := newTestExport(t, "prod")
	seedExportConfig(t, source, nil)
	cfg := NewService(source, log.New("error", "console"))
	secretCipher, err := Encrypt([]byte("oidc-secret"), []byte(r3105TestSigningKey))
	if err != nil {
		t.Fatalf("加密 OIDC Secret 失败: %v", err)
	}
	for k, v := range map[string]string{
		KeyAllowLocalLogin:   allowLocal,
		"oidc_configured":    oidcConfigured,
		"oidc_provider_type": "generic",
		"frontend_url":       "https://app.example.com",
		"oidc_params_generic": fmt.Sprintf(
			`{"base_url":"https://idp.example.com","client_id":"client","client_secret":%q}`, secretCipher),
	} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("写入导出配置 %s 失败: %v", k, err)
		}
	}
	data, err := svc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("生成导出文件失败: %v", err)
	}
	return data
}

// TestR3105ImportRejectsBeforeOverwrite v1/v2 在 oidc_configured 缺失或 false 时覆盖前拒绝，配置不变；
// 本地登录开启时不因 OIDC 未启用而拒绝。
func TestR3105ImportRejectsBeforeOverwrite(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name             string
		oidcConfigured   string
		allowLocal       string
		wantErr          bool
		wantSiteOverride bool
	}{
		{name: "local off marker false", oidcConfigured: "false", allowLocal: "false", wantErr: true},
		{name: "local off marker missing", oidcConfigured: "", allowLocal: "false", wantErr: true},
		{name: "local on marker false", oidcConfigured: "false", allowLocal: "true", wantErr: false, wantSiteOverride: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := buildR3105Export(t, tc.allowLocal, tc.oidcConfigured)
			target, svc := newTestExport(t, "prod")
			seedExportConfig(t, target, map[string]string{"site_name": "保留站点", "target_only": "keep"})
			before := readSystemConfigSnapshot(t, target)
			err := svc.Import(ctx, data, "export-pass-123", ConfirmWordImport, false)
			if tc.wantErr {
				if !errors.Is(err, ErrAuthDeadlock) {
					t.Fatalf("应返回 ErrAuthDeadlock，实际: %v", err)
				}
				after := readSystemConfigSnapshot(t, target)
				if !equalStringMap(before, after) {
					t.Fatalf("拒绝导入后配置应不变:\nbefore=%v\nafter=%v", before, after)
				}
				return
			}
			if err != nil {
				t.Fatalf("本地登录开启时不应因 OIDC 标记拒绝导入: %v", err)
			}
			after := readSystemConfigSnapshot(t, target)
			if tc.wantSiteOverride && after["site_name"] != "测试站点" {
				t.Fatalf("导入成功后站点配置应被覆盖: %v", after)
			}
			if _, ok := after["target_only"]; ok {
				t.Fatalf("严格整体覆盖应清除目标独有键: %v", after)
			}
		})
	}
}

// TestR3105ImportV2RejectsSynchronously v2 在注册异步任务前同步拒绝；registry 为 nil 仍先返回认证错误，证明未进入异步路径。
func TestR3105ImportV2RejectsSynchronously(t *testing.T) {
	ctx := context.Background()
	data := buildR3105Export(t, "false", "false")
	for _, setupMode := range []bool{true, false} {
		target, svc := newTestExport(t, "prod")
		seedExportConfig(t, target, map[string]string{"site_name": "保留站点", "target_only": "keep"})
		before := readSystemConfigSnapshot(t, target)
		disableWord := ""
		if !setupMode {
			disableWord = ConfirmWordDisable
		}
		_, err := svc.ImportV2(ctx, data, "export-pass-123", ConfirmWordImport, disableWord, setupMode)
		if !errors.Is(err, ErrAuthDeadlock) {
			t.Fatalf("v2 setup=%v 应在注册任务前返回 ErrAuthDeadlock，实际: %v", setupMode, err)
		}
		after := readSystemConfigSnapshot(t, target)
		if !equalStringMap(before, after) {
			t.Fatalf("v2 拒绝后配置应不变:\nbefore=%v\nafter=%v", before, after)
		}
	}
}

// readSystemConfigSnapshot 读取 system_config 全表（测试辅助）。
func readSystemConfigSnapshot(t *testing.T, st *store.Store) map[string]string {
	t.Helper()
	rows, err := st.DB().Query(`SELECT key, value FROM system_config`)
	if err != nil {
		t.Fatalf("读取 system_config 失败: %v", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatalf("扫描 system_config 失败: %v", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历 system_config 失败: %v", err)
	}
	return out
}

func equalStringMap(a, b map[string]string) bool {
	return reflect.DeepEqual(a, b)
}
