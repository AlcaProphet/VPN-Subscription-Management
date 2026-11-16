// snell_protocol_test.go：Build32 Step 7 Snell 协议合同测试。
// 覆盖 version/obfs_mode selector、v1/v2 禁 UDP、v2 固定 reuse、五类 obfs 分支的
// 字段活动与凭据条件必填、结构化对象拒绝未知键、敏感路径与 state_only 边界。
package node

import (
	"context"
	"strings"
	"testing"
)

func snellCreateInput(name string, params map[string]any, state *CurrentState) CreateManualInput {
	return CreateManualInput{
		Name: name, Protocol: "snell", Host: "example.com", Port: 443,
		ProtocolJSON: params, CurrentState: state,
	}
}

func snellObfsState(mode string) *CurrentState {
	return &CurrentState{Selectors: map[string]string{"obfs_mode": mode}}
}

func TestSnellSelectorDeclaration(t *testing.T) {
	proto, err := GetProtocol("snell")
	if err != nil {
		t.Fatal(err)
	}
	version, ok := selectorSchemaByName(proto, "version")
	if !ok || version.SourceField != "version" || version.Default != "1" || len(version.Values) != 5 {
		t.Fatalf("version 必须为 source_field=version、默认 1、允许 1～5 的普通 selector: %+v", version)
	}
	obfs, ok := selectorSchemaByName(proto, "obfs_mode")
	if !ok || obfs.SourceField != "" || obfs.Default != "none" {
		t.Fatalf("obfs_mode 必须为 state_only、默认 none: %+v", obfs)
	}
	want := []string{"none", "http", "tls", "shadow_tls", "restls", "jls"}
	if len(obfs.Values) != len(want) {
		t.Fatalf("obfs_mode 允许值异常: %+v", obfs.Values)
	}
	for i := range want {
		if obfs.Values[i] != want[i] {
			t.Fatalf("obfs_mode 允许值顺序异常: %+v", obfs.Values)
		}
	}
	for _, name := range []string{"version", "obfs-mode"} {
		field, ok := findSchemaField(proto.FormSchema, name)
		if !ok {
			t.Fatalf("Snell 缺少字段 %s", name)
		}
		if field.SelectorName == "" {
			t.Fatalf("字段 %s 必须投影 selector: %+v", name, field)
		}
	}
}

func TestSnellVersionDefaultAndSelectorDerivation(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-version-default",
		map[string]any{"psk": "psk-secret"}, nil))
	if err != nil {
		t.Fatalf("创建默认 Snell 节点失败: %v", err)
	}
	if got := created.CurrentState.Selectors["version"]; got != "1" {
		t.Fatalf("空值版本必须按 tag 默认 v1 派生，实际 %q", got)
	}
	if created.CurrentState.Selectors["obfs_mode"] != "none" {
		t.Fatalf("无 obfs 对象必须派生 none，实际 %q", created.CurrentState.Selectors["obfs_mode"])
	}
}

func TestSnellUDPOnlyInV3Plus(t *testing.T) {
	svc, _, _ := newTestService(t)
	// v1/v2 不活动：提交 udp=true 必须被 selector 清空域归零，不能落库为 true。
	for _, version := range []string{"1", "2"} {
		state := &CurrentState{Selectors: map[string]string{"version": version, "obfs_mode": "none"}}
		created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-udp-v"+version,
			map[string]any{"psk": "psk-secret", "version": version, "udp": true}, state))
		if err != nil {
			t.Fatalf("v%s 创建失败: %v", version, err)
		}
		if created.ProtocolJSON["udp"] == true {
			t.Fatalf("v%s 必须把 UDP 归零: %+v", version, created.ProtocolJSON)
		}
	}
	// v3/v4/v5 可编辑并保留。
	for _, version := range []string{"3", "4", "5"} {
		state := &CurrentState{Selectors: map[string]string{"version": version, "obfs_mode": "none"}}
		created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-udp-v"+version,
			map[string]any{"psk": "psk-secret", "version": version, "udp": true}, state))
		if err != nil {
			t.Fatalf("v%s 开启 UDP 应通过: %v", version, err)
		}
		if created.ProtocolJSON["udp"] != true {
			t.Fatalf("v%s 必须保留 UDP: %+v", version, created.ProtocolJSON)
		}
	}
}

func TestSnellReuseBranchClearedAndForced(t *testing.T) {
	svc, _, _ := newTestService(t)
	// v1/v3 不活动：提交 reuse 必须被清空。
	for _, version := range []string{"1", "3"} {
		state := &CurrentState{Selectors: map[string]string{"version": version, "obfs_mode": "none"}}
		created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-reuse-v"+version,
			map[string]any{"psk": "psk-secret", "version": version, "reuse": true}, state))
		if err != nil {
			t.Fatalf("v%s 创建失败: %v", version, err)
		}
		if _, exists := created.ProtocolJSON["reuse"]; exists {
			t.Fatalf("v%s 不应保留 reuse: %+v", version, created.ProtocolJSON)
		}
	}
	// v4/v5 可编辑。
	state := &CurrentState{Selectors: map[string]string{"version": "4", "obfs_mode": "none"}}
	created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-reuse-v4",
		map[string]any{"psk": "psk-secret", "version": "4", "reuse": true}, state))
	if err != nil {
		t.Fatalf("v4 创建失败: %v", err)
	}
	if created.ProtocolJSON["reuse"] != true {
		t.Fatalf("v4 应保留用户 reuse 选择: %+v", created.ProtocolJSON)
	}
	// v1/v2 切换清空 reuse。
	switched, err := svc.UpdateManual(context.Background(), created.ID, UpdateManualInput{
		Protocol: "snell", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"psk": "psk-secret", "version": "1", "reuse": true},
		CurrentState: &CurrentState{Selectors: map[string]string{"version": "1", "obfs_mode": "none"}},
		BaseRevision: created.EditRevision, ResetScopes: []string{"selector.version"},
	})
	if err != nil {
		t.Fatalf("切到 v1 失败: %v", err)
	}
	if _, exists := switched.ProtocolJSON["reuse"]; exists {
		t.Fatalf("切到 v1 必须清空 reuse: %+v", switched.ProtocolJSON)
	}
}

func TestSnellObfsNoneClearsObject(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-obfs-none",
		map[string]any{
			"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "bing.com", "password": "obfs-pass"},
		}, snellObfsState("none")))
	if err != nil {
		t.Fatalf("obfs_mode=none 创建失败: %v", err)
	}
	if _, exists := created.ProtocolJSON["obfs-opts"]; exists {
		t.Fatalf("obfs_mode=none 必须清空整个 obfs-opts: %+v", created.ProtocolJSON)
	}
}

func TestSnellObfsBranchRequirements(t *testing.T) {
	svc, _, _ := newTestService(t)
	cases := []struct {
		name   string
		mode   string
		params map[string]any
		want   string
	}{
		{name: "shadow_tls missing host", mode: "shadow_tls", params: map[string]any{"password": "p"}, want: "host"},
		{name: "shadow_tls missing password", mode: "shadow_tls", params: map[string]any{"host": "h"}, want: "password"},
		{name: "restls missing version hint", mode: "restls", params: map[string]any{"host": "h", "password": "p"}, want: "version-hint"},
		{name: "jls missing username", mode: "jls", params: map[string]any{"host": "h", "password": "p"}, want: "username"},
		{name: "jls missing password", mode: "jls", params: map[string]any{"host": "h", "username": "u"}, want: "password"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateManual(context.Background(), snellCreateInput(
				"snell-obfs-"+strings.ReplaceAll(tc.name, " ", "-"),
				map[string]any{"psk": "psk-secret", "version": "4", "obfs-opts": tc.params}, snellObfsState(tc.mode)))
			if err == nil {
				t.Fatal("缺少条件必填必须被拒绝")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("错误应定位到 %s，实际: %v", tc.want, err)
			}
		})
	}

	// http/tls 只要求 host 之外无凭据；host 可省略。
	for _, mode := range []string{"http", "tls"} {
		if _, err := svc.CreateManual(context.Background(), snellCreateInput("snell-obfs-"+mode+"-ok",
			map[string]any{"psk": "psk-secret", "version": "4", "obfs-opts": map[string]any{"host": "bing.com"}}, snellObfsState(mode))); err != nil {
			t.Fatalf("%s 合法配置应通过: %v", mode, err)
		}
	}
	// shadow-tls 证书与私钥成对。
	_, err := svc.CreateManual(context.Background(), snellCreateInput("snell-obfs-shadow-tls-mtls",
		map[string]any{"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "h", "password": "p", "certificate": "CERT"}}, snellObfsState("shadow_tls")))
	if err == nil || !strings.Contains(err.Error(), "private-key") {
		t.Fatalf("shadow-tls mTLS 缺半对应定位 private-key，实际: %v", err)
	}
	// jls 合法配置。
	if _, err := svc.CreateManual(context.Background(), snellCreateInput("snell-obfs-jls-ok",
		map[string]any{"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "h", "username": "u", "password": "p"}}, snellObfsState("jls"))); err != nil {
		t.Fatalf("jls 合法配置应通过: %v", err)
	}
	// restls 合法配置。
	if _, err := svc.CreateManual(context.Background(), snellCreateInput("snell-obfs-restls-ok",
		map[string]any{"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "h", "password": "p", "version-hint": "tls13"}}, snellObfsState("restls"))); err != nil {
		t.Fatalf("restls 合法配置应通过: %v", err)
	}
}

func TestSnellObfsModeSwitchClearsOldFields(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-obfs-switch",
		map[string]any{"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "h", "username": "u", "password": "p"}}, snellObfsState("jls")))
	if err != nil {
		t.Fatalf("创建 jls 分支失败: %v", err)
	}
	switched, err := svc.UpdateManual(context.Background(), created.ID, UpdateManualInput{
		Protocol: "snell", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "h", "username": "u", "password": "p", "version-hint": "tls13"}},
		CurrentState: snellObfsState("restls"), BaseRevision: created.EditRevision,
		ResetScopes: []string{"selector.obfs_mode"},
	})
	if err != nil {
		t.Fatalf("切换到 restls 失败: %v", err)
	}
	opts, _ := switched.ProtocolJSON["obfs-opts"].(map[string]any)
	if _, exists := opts["username"]; exists {
		t.Fatalf("切到 restls 必须清空 jls 的 username: %+v", opts)
	}
	if opts["version-hint"] != "tls13" {
		t.Fatalf("切到 restls 必须保留 restls 字段: %+v", opts)
	}
}

func TestSnellObfsObjectRejectsUnknownKeys(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), snellCreateInput("snell-obfs-unknown",
		map[string]any{"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "h", "unknown-key": "v"}}, snellObfsState("http")))
	if err == nil || !strings.Contains(err.Error(), "unknown-key") {
		t.Fatalf("结构化 obfs-opts 必须拒绝未知键，实际: %v", err)
	}
}

func TestSnellClientFingerprintOnlyInCamouflageBranches(t *testing.T) {
	svc, _, _ := newTestService(t)
	for _, mode := range []string{"none", "http", "tls"} {
		created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-fp-"+mode,
			map[string]any{"psk": "psk-secret", "version": "4", "client-fingerprint": "chrome"}, snellObfsState(mode)))
		if err != nil {
			t.Fatalf("%s 创建失败: %v", mode, err)
		}
		if _, exists := created.ProtocolJSON["client-fingerprint"]; exists {
			t.Fatalf("%s 不应保留 client-fingerprint: %+v", mode, created.ProtocolJSON)
		}
	}
	created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-fp-shadow-tls",
		map[string]any{"psk": "psk-secret", "version": "4", "client-fingerprint": "chrome",
			"obfs-opts": map[string]any{"host": "h", "password": "p"}}, snellObfsState("shadow_tls")))
	if err != nil {
		t.Fatalf("shadow_tls 创建失败: %v", err)
	}
	if created.ProtocolJSON["client-fingerprint"] != "chrome" {
		t.Fatalf("shadow_tls 必须保留 client-fingerprint: %+v", created.ProtocolJSON)
	}
}

func TestSnellStateOnlyObfsModeRejectedInProtocolJSON(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), snellCreateInput("snell-state-only",
		map[string]any{"psk": "psk-secret", "obfs-mode": "http"}, nil))
	if err == nil || !strings.Contains(err.Error(), "obfs") {
		t.Fatalf("state_only obfs-mode 不得写入 protocol_json，实际: %v", err)
	}
}

func TestSnellSensitivePathsDeclared(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), snellCreateInput("snell-sensitive",
		map[string]any{"psk": "psk-secret", "version": "4",
			"obfs-opts": map[string]any{"host": "h", "password": "obfs-pass",
				"certificate": "CERT", "private-key": "KEY"}}, snellObfsState("shadow_tls")))
	if err != nil {
		t.Fatalf("创建 shadow-tls 节点失败: %v", err)
	}
	for _, want := range []string{"psk", "obfs-opts.password", "obfs-opts.private-key"} {
		found := false
		for _, saved := range created.SavedSensitivePaths {
			if saved == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("敏感路径 %s 未登记为已保存密文: %+v", want, created.SavedSensitivePaths)
		}
	}
}
