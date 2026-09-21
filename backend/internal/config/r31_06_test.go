package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"testing"

	"golang.org/x/crypto/argon2"
)

// sealR3106Payload 用与 Export 相同的加密参数生成 v1/v2 导入文件，供 R31-06 导入拒绝测试使用。
func sealR3106Payload(t *testing.T, payload ExportPayload, password string) []byte {
	t.Helper()
	plain, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("序列化测试导入内容失败: %v", err)
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		t.Fatalf("生成测试 salt 失败: %v", err)
	}
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("创建测试 AES cipher 失败: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("创建测试 GCM 失败: %v", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		t.Fatalf("生成测试 nonce 失败: %v", err)
	}
	out := append(salt, nonce...)
	out = gcm.Seal(out, nonce, plain, nil)
	return out
}

// TestR3106ProductionAdminRejectsMock SaveOidc/GetOidc 在 Production 下拒绝 mock 且保留只读历史配置。
func TestR3106ProductionAdminRejectsMock(t *testing.T) {
	ctx := context.Background()
	st, svc := newTestAdminWithMode(t, &mockOidcOps{configured: true}, "prod")
	before := readSystemConfigSnapshot(t, st)

	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "mock", FrontendURL: "https://app.example.com"}); !errors.Is(err, ErrMockModeRestricted) {
		t.Fatalf("Production SaveOidc(mock) 应返回 ErrMockModeRestricted，实际: %v", err)
	}
	after := readSystemConfigSnapshot(t, st)
	if !equalStringMap(before, after) {
		t.Fatalf("Production 拒绝 mock 后不得写配置:\nbefore=%v\nafter=%v", before, after)
	}

	// 直接造历史 mock 配置：管理端只读回显应保留，并给出 Production 警示。
	if err := svc.cfg.Set(ctx, oidcKeyProviderType, "mock"); err != nil {
		t.Fatalf("写入历史 mock provider 失败: %v", err)
	}
	if err := svc.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		t.Fatalf("写入 oidc_configured 失败: %v", err)
	}
	got, err := svc.GetOidc(ctx)
	if err != nil {
		t.Fatalf("GetOidc 应只读返回历史 mock 配置: %v", err)
	}
	if got.ProviderType != "mock" || got.ParamsWarning != OidcWarningMockProduction {
		t.Fatalf("Production mock GET 应只读保留并提示切换，实际: %+v", got)
	}
	if stored, _ := svc.cfg.Get(ctx, oidcKeyProviderType); stored != "mock" {
		t.Fatalf("只读查看不得自动删除历史 mock 参数，provider=%q", stored)
	}
}

// TestR3106DevMockSaveStillWorks Dev 下 SaveOidc 原有 mock 路径保持可用。
func TestR3106DevMockSaveStillWorks(t *testing.T) {
	ctx := context.Background()
	ops := &mockOidcOps{configured: true}
	_, svc := newTestAdminWithMode(t, ops, "dev")
	if err := svc.SaveOidc(ctx, OidcSettings{ProviderType: "mock", FrontendURL: "https://app.example.com"}); err != nil {
		t.Fatalf("Dev SaveOidc(mock) 应成功: %v", err)
	}
	if len(ops.saveCalls) != 1 {
		t.Fatalf("Dev mock 保存应调用一次 SaveParams，实际 %d", len(ops.saveCalls))
	}
	if stored, _ := svc.cfg.Get(ctx, oidcKeyProviderType); stored != "mock" {
		t.Fatalf("Dev mock provider 应写入，实际 %q", stored)
	}
}

// TestR3106ValidateImportedNoMock 导入校验独立于本地登录与 oidc_configured，按 key 存在性整体拒绝。
func TestR3106ValidateImportedNoMock(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]string
	}{
		{
			name: "生效类型 mock",
			cfg: map[string]string{
				KeyConfigured:        "true",
				KeyAllowLocalLogin:   "true",
				"oidc_configured":    "false",
				"oidc_provider_type": "mock",
			},
		},
		{
			name: "仅空 oidc_params_mock",
			cfg: map[string]string{
				KeyConfigured:      "true",
				KeyAllowLocalLogin: "true",
				"oidc_params_mock": "",
			},
		},
		{
			name: "非空 oidc_params_mock 且本地登录关闭",
			cfg: map[string]string{
				KeyConfigured:      "true",
				KeyAllowLocalLogin: "false",
				"oidc_configured":  "true",
				"oidc_params_mock": `{"base_url":"","client_id":""}`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateImportedAuthUsable(tc.cfg); !errors.Is(err, ErrMockModeRestricted) {
				t.Fatalf("含 mock 配置应返回 ErrMockModeRestricted，实际: %v", err)
			}
		})
	}
}

// TestR3106ImportRejectsMockBeforeOverwrite v1/v2 两个导入入口均在任何覆盖写入前整体拒绝 mock。
func TestR3106ImportRejectsMockBeforeOverwrite(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		version int
		cfg     map[string]string
	}{
		{
			name:    "v1 生效类型 mock",
			version: 1,
			cfg: map[string]string{
				KeyConfigured:        "true",
				KeyAllowLocalLogin:   "true",
				"oidc_configured":    "true",
				"oidc_provider_type": "mock",
			},
		},
		{
			name:    "v2 仅空 mock 键",
			version: FormatVersion,
			cfg: map[string]string{
				KeyConfigured:      "true",
				KeyAllowLocalLogin: "true",
				"oidc_params_mock": "",
			},
		},
		{
			name:    "v2 生效类型 mock 且本地登录关闭",
			version: FormatVersion,
			cfg: map[string]string{
				KeyConfigured:        "true",
				KeyAllowLocalLogin:   "false",
				"oidc_configured":    "false",
				"oidc_provider_type": "mock",
			},
		},
		{
			name:    "v2 非空 mock 键且本地登录关闭",
			version: FormatVersion,
			cfg: map[string]string{
				KeyConfigured:      "true",
				KeyAllowLocalLogin: "false",
				"oidc_configured":  "false",
				"oidc_params_mock": `{"base_url":"","client_id":""}`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name+" / Import", func(t *testing.T) {
			target, svc := newTestExport(t, "prod")
			seedExportConfig(t, target, map[string]string{"site_name": "保留站点", "target_only": "keep"})
			before := readSystemConfigSnapshot(t, target)
			data := sealR3106Payload(t, ExportPayload{FormatVersion: tc.version, Config: tc.cfg}, "export-pass-123")
			if err := svc.Import(ctx, data, "export-pass-123", ConfirmWordImport, false); !errors.Is(err, ErrMockModeRestricted) {
				t.Fatalf("Import 应拒绝 mock 配置，实际: %v", err)
			}
			after := readSystemConfigSnapshot(t, target)
			if !equalStringMap(before, after) {
				t.Fatalf("Import 拒绝后不得写库:\nbefore=%v\nafter=%v", before, after)
			}
		})
		t.Run(tc.name+" / ImportV2", func(t *testing.T) {
			target, svc := newTestExport(t, "prod")
			seedExportConfig(t, target, map[string]string{"site_name": "保留站点", "target_only": "keep"})
			before := readSystemConfigSnapshot(t, target)
			data := sealR3106Payload(t, ExportPayload{FormatVersion: tc.version, Config: tc.cfg}, "export-pass-123")
			if _, err := svc.ImportV2(ctx, data, "export-pass-123", ConfirmWordImport, "", true); !errors.Is(err, ErrMockModeRestricted) {
				t.Fatalf("ImportV2 应同步拒绝 mock 配置，实际: %v", err)
			}
			after := readSystemConfigSnapshot(t, target)
			if !equalStringMap(before, after) {
				t.Fatalf("ImportV2 拒绝后不得写库:\nbefore=%v\nafter=%v", before, after)
			}
		})
	}
}
