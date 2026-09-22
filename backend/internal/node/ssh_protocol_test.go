// ssh_protocol_test.go：Build32 Step 6 SSH 协议合同测试。
// 覆盖 auth_mode 密码／私钥互斥、PEM 内容边界（拒绝主机文件路径）、私钥口令生命周期、
// Host Key 与算法列表的 authorized-key 语法／去空白去重／保序、敏感路径与 state_only 边界。
package node

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"

	"golang.org/x/crypto/ssh"
)

func sshCreateInput(name string, params map[string]any, state *CurrentState) CreateManualInput {
	return CreateManualInput{
		Name: name, Protocol: "ssh", Host: "example.com", Port: 22,
		ProtocolJSON: params, CurrentState: state,
	}
}

// sshTestPrivateKeyPEM 生成一次性测试用 PKCS#8 明文私钥，不作为任何真实凭据使用。
func sshTestPrivateKeyPEM(t *testing.T) []byte {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成测试私钥失败: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("序列化测试私钥失败: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// sshTestEncryptedPrivateKeyPEM 生成一次性测试用口令加密私钥。
func sshTestEncryptedPrivateKeyPEM(t *testing.T, passphrase string) []byte {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成测试 RSA 私钥失败: %v", err)
	}
	block, err := x509.EncryptPEMBlock(rand.Reader, "RSA PRIVATE KEY",
		x509.MarshalPKCS1PrivateKey(privateKey), []byte(passphrase), x509.PEMCipherAES256)
	if err != nil {
		t.Fatalf("加密测试私钥失败: %v", err)
	}
	return pem.EncodeToMemory(block)
}

// sshTestAuthorizedKey 生成一次性测试用 authorized-key 行。
func sshTestAuthorizedKey(t *testing.T) string {
	t.Helper()
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成测试公钥失败: %v", err)
	}
	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		t.Fatalf("转换测试公钥失败: %v", err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublicKey)))
}

func sshStringList(t *testing.T, params map[string]any, key string) []string {
	t.Helper()
	value, ok := params[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				t.Fatalf("字段 %s 含非字符串条目: %+v", key, value)
			}
			out = append(out, text)
		}
		return out
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return strings.Split(typed, ",")
	default:
		t.Fatalf("字段 %s 类型异常: %T", key, value)
		return nil
	}
}

func TestSSHAuthModeSelectorDeclaration(t *testing.T) {
	proto, err := GetProtocol("ssh")
	if err != nil {
		t.Fatal(err)
	}
	selector, ok := selectorSchemaByName(proto, "auth_mode")
	if !ok {
		t.Fatal("SSH 必须声明 auth_mode selector")
	}
	if selector.Default != "password" || selector.SourceField != "" {
		t.Fatalf("auth_mode 必须是 state_only、默认 password: %+v", selector)
	}
	if len(selector.Values) != 2 || selector.Values[0] != "password" || selector.Values[1] != "private_key" {
		t.Fatalf("auth_mode 允许值必须为 password/private_key: %+v", selector)
	}
	var modeField *FieldSchema
	for i := range proto.FormSchema {
		if proto.FormSchema[i].SelectorName == "auth_mode" {
			modeField = &proto.FormSchema[i]
		}
	}
	if modeField == nil || !modeField.StateOnly || modeField.Type != "select" {
		t.Fatalf("auth_mode 字段必须为 state_only select: %+v", modeField)
	}
}

func TestSSHAuthModeDerivation(t *testing.T) {
	proto, _ := GetProtocol("ssh")
	if got := DeriveCurrentState(proto, map[string]any{"username": "u"}).Selectors["auth_mode"]; got != "password" {
		t.Fatalf("无 private-key 应派生 password，实际 %q", got)
	}
	withKey := DeriveCurrentState(proto, map[string]any{"private-key": string(sshTestPrivateKeyPEM(t))})
	if withKey.Selectors["auth_mode"] != "private_key" {
		t.Fatalf("存在 private-key 应派生 private_key，实际 %q", withKey.Selectors["auth_mode"])
	}
}

func TestSSHPasswordModeRequiresUsernameAndPassword(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"auth_mode": "password"}}
	_, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-missing-username",
		map[string]any{"password": "pw-secret"}, &state))
	if err == nil || !strings.Contains(err.Error(), "username") {
		t.Fatalf("password 模式缺少 username 必须返回字段级错误，实际: %v", err)
	}
	_, err = svc.CreateManual(context.Background(), sshCreateInput("ssh-missing-password",
		map[string]any{"username": "u"}, &state))
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Fatalf("password 模式缺少 password 必须返回字段级错误，实际: %v", err)
	}
	if _, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-password-ok",
		map[string]any{"username": "u", "password": "pw-secret"}, &state)); err != nil {
		t.Fatalf("password 模式成对凭据应通过: %v", err)
	}
}

func TestSSHPrivateKeyModeRequiresPEMContent(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"auth_mode": "private_key"}}
	cases := []struct {
		name string
		key  string
	}{
		{name: "host file path", key: "/home/user/.ssh/id_rsa"},
		{name: "relative path", key: "keys/id_rsa"},
		{name: "plain text", key: "not-a-pem-key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateManual(context.Background(), sshCreateInput(
				"ssh-bad-key-"+strings.ReplaceAll(tc.name, " ", "-"),
				map[string]any{"username": "u", "private-key": tc.key}, &state))
			if err == nil {
				t.Fatal("非 PEM 内容必须被拒绝，绝不能作为主机文件路径交给内核")
			}
			if !strings.Contains(err.Error(), "private-key") {
				t.Fatalf("错误应定位到 private-key，实际: %v", err)
			}
		})
	}

	valid := string(sshTestPrivateKeyPEM(t))
	if _, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-key-ok",
		map[string]any{"username": "u", "private-key": valid}, &state)); err != nil {
		t.Fatalf("合法 PEM 私钥应通过: %v", err)
	}
}

func TestSSHPrivateKeyPassphraseLifecycle(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"auth_mode": "private_key"}}
	encrypted := string(sshTestEncryptedPrivateKeyPEM(t, "correct-pass"))
	plain := string(sshTestPrivateKeyPEM(t))

	_, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-enc-missing-pass",
		map[string]any{"username": "u", "private-key": encrypted}, &state))
	if err == nil || !strings.Contains(err.Error(), "private-key-passphrase") {
		t.Fatalf("加密私钥缺少口令必须定位到 private-key-passphrase，实际: %v", err)
	}
	_, err = svc.CreateManual(context.Background(), sshCreateInput("ssh-enc-wrong-pass",
		map[string]any{"username": "u", "private-key": encrypted, "private-key-passphrase": "wrong-pass"}, &state))
	if err == nil || !strings.Contains(err.Error(), "private-key-passphrase") {
		t.Fatalf("错误口令必须定位到 private-key-passphrase，实际: %v", err)
	}
	if _, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-enc-ok",
		map[string]any{"username": "u", "private-key": encrypted, "private-key-passphrase": "correct-pass"}, &state)); err != nil {
		t.Fatalf("加密私钥＋正确口令应通过: %v", err)
	}
	_, err = svc.CreateManual(context.Background(), sshCreateInput("ssh-plain-with-pass",
		map[string]any{"username": "u", "private-key": plain, "private-key-passphrase": "some-pass"}, &state))
	if err == nil || !strings.Contains(err.Error(), "private-key-passphrase") {
		t.Fatalf("未加密私钥不应接受口令，实际: %v", err)
	}
}

func TestSSHBranchSwitchClearsOtherCredentials(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	passwordState := CurrentState{Selectors: map[string]string{"auth_mode": "password"}}
	keyState := CurrentState{Selectors: map[string]string{"auth_mode": "private_key"}}

	created, err := svc.CreateManual(ctx, sshCreateInput("ssh-branch-switch",
		map[string]any{"username": "u", "password": "first-pass"}, &passwordState))
	if err != nil {
		t.Fatalf("创建 password 分支失败: %v", err)
	}

	// A→B：切到 private_key 必须清除 password 密文。
	switched, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "ssh", Host: "example.com", Port: 22,
		ProtocolJSON: map[string]any{
			"username": "u", "password": "first-pass", "private-key": string(sshTestPrivateKeyPEM(t)),
		},
		CurrentState: &keyState, BaseRevision: created.EditRevision, ResetScopes: []string{"selector.auth_mode"},
	})
	if err != nil {
		t.Fatalf("切换到 private_key 分支失败: %v", err)
	}
	if _, exists := switched.ProtocolJSON["password"]; exists {
		t.Fatalf("切到 private_key 必须清除 password: %+v", switched.ProtocolJSON)
	}
	for _, path := range switched.SavedSensitivePaths {
		if path == "password" {
			t.Fatalf("切到 private_key 不得保留 password 密文: %+v", switched.SavedSensitivePaths)
		}
	}

	// B→A：切回 password 必须清除 private-key 与口令，且不恢复旧凭据。
	backState := CurrentState{Selectors: map[string]string{"auth_mode": "password"}}
	back, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "ssh", Host: "example.com", Port: 22,
		ProtocolJSON: map[string]any{"username": "u", "password": "second-pass"},
		CurrentState: &backState, BaseRevision: switched.EditRevision, ResetScopes: []string{"selector.auth_mode"},
	})
	if err != nil {
		t.Fatalf("切回 password 分支失败: %v", err)
	}
	for _, key := range []string{"private-key", "private-key-passphrase"} {
		if _, exists := back.ProtocolJSON[key]; exists {
			t.Fatalf("切回 password 必须清除 %s: %+v", key, back.ProtocolJSON)
		}
	}
	for _, path := range back.SavedSensitivePaths {
		if path == "private-key" || path == "private-key-passphrase" {
			t.Fatalf("切回 password 不得保留 %s 密文: %+v", path, back.SavedSensitivePaths)
		}
	}
}

func TestSSHHostKeyAuthorizedKeyValidation(t *testing.T) {
	svc, _, _ := newTestService(t)
	valid := sshTestAuthorizedKey(t)
	_, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-host-key-invalid",
		map[string]any{"username": "u", "password": "pw-secret", "host-key": []string{"ssh-rsa not-base64"}}, nil))
	if err == nil || !strings.Contains(err.Error(), "host-key") {
		t.Fatalf("非法 Host Key 必须返回字段级错误，实际: %v", err)
	}
	created, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-host-key-valid",
		map[string]any{"username": "u", "password": "pw-secret", "host-key": []string{valid}}, nil))
	if err != nil {
		t.Fatalf("合法 authorized-key 应通过: %v", err)
	}
	if got := sshStringList(t, created.ProtocolJSON, "host-key"); len(got) != 1 || got[0] != valid {
		t.Fatalf("host-key 列表未按结构化列表保存: %+v", created.ProtocolJSON["host-key"])
	}
	// 空 host-key 允许保存（改由 Clash 目标给出安全 warn）。
	if _, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-host-key-empty",
		map[string]any{"username": "u", "password": "pw-secret"}, nil)); err != nil {
		t.Fatalf("空 host-key 允许保存: %v", err)
	}
}

func TestSSHHostKeyAlgorithmsTrimDedupeOrder(t *testing.T) {
	svc, _, _ := newTestService(t)
	created, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-host-key-algos",
		map[string]any{
			"username": "u", "password": "pw-secret",
			"host-key-algorithms": []string{" rsa-sha2-256 ", "ssh-ed25519", "rsa-sha2-256"},
		}, nil))
	if err != nil {
		t.Fatalf("host-key-algorithms 保存失败: %v", err)
	}
	got := sshStringList(t, created.ProtocolJSON, "host-key-algorithms")
	want := []string{"rsa-sha2-256", "ssh-ed25519"}
	if len(got) != len(want) {
		t.Fatalf("host-key-algorithms 应去空白去重，实际: %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("host-key-algorithms 必须保持用户顺序，实际: %+v", got)
		}
	}
	// 纯空白算法列表规范化为未设置，不写库。
	blank, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-host-key-algos-blank",
		map[string]any{"username": "u", "password": "pw-secret", "host-key-algorithms": []string{"  "}}, nil))
	if err != nil {
		t.Fatalf("纯空白算法列表应按去空白语义规范化为未设置: %v", err)
	}
	if _, exists := blank.ProtocolJSON["host-key-algorithms"]; exists {
		t.Fatalf("纯空白算法列表不得落库: %+v", blank.ProtocolJSON)
	}
	// 非字符串条目仍必须被拒绝为类型错误。
	if _, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-host-key-algos-type",
		map[string]any{"username": "u", "password": "pw-secret", "host-key-algorithms": []any{7}}, nil)); err == nil {
		t.Fatal("host-key-algorithms 非字符串条目必须被拒绝")
	}
}

func TestSSHStateOnlyFieldRejectedInProtocolJSON(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-state-only",
		map[string]any{"username": "u", "password": "pw-secret", "auth-mode": "private_key"}, nil))
	if err == nil || !strings.Contains(err.Error(), "auth") {
		t.Fatalf("state_only 字段不得写入 protocol_json，实际: %v", err)
	}
}

func TestSSHSensitivePathsDeclared(t *testing.T) {
	svc, _, _ := newTestService(t)
	state := CurrentState{Selectors: map[string]string{"auth_mode": "private_key"}}
	created, err := svc.CreateManual(context.Background(), sshCreateInput("ssh-sensitive",
		map[string]any{
			"username": "u", "private-key": string(sshTestEncryptedPrivateKeyPEM(t, "k-pass")),
			"private-key-passphrase": "k-pass",
		}, &state))
	if err != nil {
		t.Fatalf("创建 SSH 私钥节点失败: %v", err)
	}
	for _, want := range []string{"private-key", "private-key-passphrase"} {
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

// TestSSHKeepsSavedPrivateKeyOnUpdate 回归 Step 11 暴露的共享基线缺陷：
// 更新时保留的私钥是项目密文，不得再按 PEM 明文语义解析。
func TestSSHKeepsSavedPrivateKeyOnUpdate(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	state := &CurrentState{Selectors: map[string]string{"auth_mode": "private_key"}}
	created, err := svc.CreateManual(ctx, sshCreateInput("ssh-keep-key", map[string]any{
		"username": "root", "private-key": string(sshTestPrivateKeyPEM(t)),
	}, state))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.UpdateManual(ctx, created.ID, UpdateManualInput{
		Protocol: "ssh", Host: "example.com", Port: 22, BaseRevision: created.EditRevision,
		ProtocolJSON: map[string]any{"username": "root", "private-key": ""}, CurrentState: state,
	}); err != nil {
		t.Fatalf("保留已保存私钥的更新必须成功: %v", err)
	}
}
