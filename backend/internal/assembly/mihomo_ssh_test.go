package assembly

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"os/exec"
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"

	"golang.org/x/crypto/ssh"
)

// sshKernelPrivateKeyPEM 生成一次性明文私钥，仅用于固定内核正反例，不作为真实凭据。
func sshKernelPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("生成测试私钥失败: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("序列化测试私钥失败: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// sshKernelAuthorizedKey 生成一次性 authorized-key 行，仅用于固定内核正反例。
func sshKernelAuthorizedKey(t *testing.T) string {
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

// TestMihomo11931SSHProtocolStructures 覆盖 SSH 显式 adapter 在固定 Mihomo v1.19.31
// 上的代表性正反例；普通 go test 未设置外部二进制时跳过，严格入口由 .mihomo-test.sh 负责。
func TestMihomo11931SSHProtocolStructures(t *testing.T) {
	bin := os.Getenv("MIHOMO_11931_BIN")
	if bin == "" {
		t.Skip("未设置 MIHOMO_11931_BIN，跳过固定 Mihomo 1.19.31 SSH 验收")
	}
	version, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		t.Fatalf("读取 Mihomo 版本失败: %v: %s", err, version)
	}
	fields := strings.Fields(string(version))
	if len(fields) < 3 || fields[0] != "Mihomo" || fields[1] != "Meta" || fields[2] != "v1.19.31" {
		t.Fatalf("固定验收要求 Mihomo v1.19.31，实际: %s", version)
	}

	t.Run("accepts generated structures", func(t *testing.T) {
		proxy := clashProxyFixture(t, &nodeData{
			Protocol: "ssh", RenderName: "fixed-ssh-password",
			Host: "192.0.2.1", Port: 22,
			ProtocolJSON: map[string]any{
				"username": "ssh-user", "password": "ssh-password",
				"host-key":            []any{sshKernelAuthorizedKey(t)},
				"host-key-algorithms": []any{"ssh-ed25519", "rsa-sha2-256"},
			},
		})
		content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), "fixed-ssh-password")
		output, err := runMihomoConfigTest(t, bin, content)
		if err != nil {
			t.Fatalf("Mihomo 1.19.31 拒绝生成的 SSH 结构: %v\n%s\nYAML:\n%s", err, output, content)
		}
	})

	t.Run("accepts generated private key structure", func(t *testing.T) {
		proxy := clashProxyFixture(t, &nodeData{
			Protocol: "ssh", RenderName: "fixed-ssh-private-key",
			Host: "192.0.2.1", Port: 22,
			ProtocolJSON: map[string]any{
				"username": "ssh-user", "private-key": sshKernelPrivateKeyPEM(t),
			},
		})
		content := mihomoSSPluginConfig(t, orderedMapToMapSlice(proxy), "fixed-ssh-private-key")
		output, err := runMihomoConfigTest(t, bin, content)
		if err != nil {
			t.Fatalf("Mihomo 1.19.31 拒绝生成的 SSH 私钥结构: %v\n%s\nYAML:\n%s", err, output, content)
		}
	})

	t.Run("rejects invalid structures", func(t *testing.T) {
		cases := []struct {
			name       string
			proxy      gyaml.MapSlice
			wantOutput string
		}{
			{
				name: "ssh private key path text",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-ssh-path"}, {Key: "type", Value: "ssh"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 22},
					{Key: "username", Value: "u"}, {Key: "private-key", Value: "keys/id_rsa"},
				},
				wantOutput: "",
			},
			{
				name: "ssh invalid host key",
				proxy: gyaml.MapSlice{
					{Key: "name", Value: "bad-ssh-hostkey"}, {Key: "type", Value: "ssh"},
					{Key: "server", Value: "192.0.2.1"}, {Key: "port", Value: 22},
					{Key: "username", Value: "u"}, {Key: "password", Value: "p"},
					{Key: "host-key", Value: []string{"ssh-rsa not-base64"}},
				},
				wantOutput: "parse host key",
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				content := mihomoSSPluginConfig(t, tc.proxy, "bad-"+strings.ReplaceAll(tc.name, " ", "-"))
				output, err := runMihomoConfigTest(t, bin, content)
				if err == nil {
					t.Fatalf("Mihomo 1.19.31 应拒绝无效 SSH 结构 %s\n%s", tc.name, content)
				}
				if tc.wantOutput != "" && !strings.Contains(string(output), tc.wantOutput) {
					t.Fatalf("Mihomo 1.19.31 拒绝原因异常，期望包含 %q，实际:\n%s", tc.wantOutput, output)
				}
			})
		}
	})
}
