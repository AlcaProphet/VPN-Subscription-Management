package assembly

import (
	"fmt"
	"strings"
	"testing"
)

func TestSelfCheckRejectsDangling(t *testing.T) {
	content := []byte(`
proxies:
  - name: 节点A
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
proxy-groups:
  - name: 空组
    type: select
    proxies: []
  - name: 悬空组
    type: select
    proxies: [不存在节点]
rules:
  - DOMAIN-SUFFIX,example.com,不存在目标
`)
	issues := CheckClashContent(content)
	if !HasError(issues) {
		t.Fatalf("悬空引用应产生 error: %+v", issues)
	}
	wants := []string{"select 组不能", "代理组引用不存在", "规则目标不存在"}
	for _, want := range wants {
		found := false
		for _, issue := range issues {
			if strings.Contains(issue.Message, want) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("缺少自检问题 %q: %+v", want, issues)
		}
	}
}

func TestSelfCheckPassesGenerated(t *testing.T) {
	content := []byte(`
proxies:
  - name: 节点A
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
proxy-groups:
  - name: 代理组
    type: select
    proxies: [节点A]
rules:
  - GEOIP,CN,DIRECT
  - MATCH,代理组
`)
	issues := CheckClashContent(content)
	if HasError(issues) {
		t.Fatalf("合法最小产物不应有 error: %+v", issues)
	}
}

func TestSelfCheckUsesRuleMetadata(t *testing.T) {
	content := []byte(`
proxies:
  - name: 节点A
    type: vless
    server: example.com
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
proxy-groups:
  - name: 代理组
    type: select
    proxies: [节点A]
rules:
  - AND,((DOMAIN,a.com),(NETWORK,tcp)),代理组
  - IP-ASN,45102,代理组,no-resolve
  - MATCH,代理组
`)
	if issues := CheckClashContent(content); HasError(issues) {
		t.Fatalf("共享元数据支持的规则不应报错: %+v", issues)
	}
	bad := []byte(strings.Replace(string(content), "AND,((DOMAIN,a.com),(NETWORK,tcp)),代理组", "NETWORK,icmp,代理组", 1))
	if issues := CheckClashContent(bad); !HasError(issues) {
		t.Fatalf("非法 NETWORK 应报错: %+v", issues)
	}
}

func TestSelfCheckProviderAndIncludeGroups(t *testing.T) {
	content := []byte(`
proxy-providers:
  provider-a:
    type: http
    url: https://example.com/sub
proxy-groups:
  - name: Provider组
    type: load-balance
    use: [provider-a]
  - name: 全量组
    type: select
    include-all-providers: true
rules:
  - MATCH,Provider组
`)
	if issues := CheckClashContent(content); HasError(issues) {
		t.Fatalf("use/include-all 组不应误报: %+v", issues)
	}
	bad := []byte(strings.Replace(string(content), "use: [provider-a]", "use: [missing]", 1))
	if issues := CheckClashContent(bad); !HasError(issues) {
		t.Fatalf("不存在 provider 应报错: %+v", issues)
	}
}

func TestSelfCheckSSPluginStructure(t *testing.T) {
	base := `
proxies:
  - name: ss-node
    type: ss
    server: example.com
    port: 443
    cipher: aes-128-gcm
    password: secret
%s
rules:
  - GEOIP,CN,DIRECT
  - MATCH,DIRECT
`
	cases := []struct {
		name     string
		plugin   string
		wantPath string
	}{
		{name: "no plugin", plugin: ""},
		{name: "empty plugin", plugin: "    plugin: \"\"\n", wantPath: "$.proxies[0].plugin"},
		{name: "valid obfs", plugin: "    plugin: obfs\n    plugin-opts:\n      mode: http\n      host: cdn.example.com\n"},
		{name: "valid unknown string map", plugin: "    plugin: custom-plugin\n    plugin-opts:\n      flag: \"\"\n      special: \"a;b=c\"\n"},
		{name: "legacy URI string", plugin: "    plugin: obfs-local;obfs=http\n", wantPath: "$.proxies[0].plugin"},
		{name: "options without plugin", plugin: "    plugin-opts:\n      mode: http\n", wantPath: "$.proxies[0].plugin-opts"},
		{name: "options wrong shape", plugin: "    plugin: obfs\n    plugin-opts: obfs=http\n", wantPath: "$.proxies[0].plugin-opts"},
		{name: "internal object leak", plugin: "    obfs-opts:\n      mode: http\n", wantPath: "$.proxies[0].obfs-opts"},
		{name: "required mode missing", plugin: "    plugin: obfs\n    plugin-opts:\n      host: cdn.example.com\n", wantPath: "$.proxies[0].plugin-opts.mode"},
		{name: "invalid obfs mode", plugin: "    plugin: obfs\n    plugin-opts:\n      mode: websocket\n", wantPath: "$.proxies[0].plugin-opts.mode"},
		{name: "invalid v2ray mode", plugin: "    plugin: v2ray-plugin\n    plugin-opts:\n      mode: http\n", wantPath: "$.proxies[0].plugin-opts.mode"},
		{name: "unknown non-string option", plugin: "    plugin: custom-plugin\n    plugin-opts:\n      enabled: true\n", wantPath: "$.proxies[0].plugin-opts.enabled"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			issues := CheckClashContent(fmt.Appendf(nil, base, tc.plugin))
			if tc.wantPath == "" {
				if HasError(issues) {
					t.Fatalf("合法 SS 插件结构不应报错: %+v", issues)
				}
				return
			}
			for _, issue := range issues {
				if issue.Severity == "error" && issue.Path == tc.wantPath {
					return
				}
			}
			t.Fatalf("缺少路径 %s 的 SS 插件结构错误: %+v", tc.wantPath, issues)
		})
	}
}

// TestSelfCheckProtocolWireShapeGates 锁定 Step 4～18 后续协议在最终 YAML 中的
// 关键 shape、互斥分支、endpoint 形状与项目禁止组合门禁。
// 每项都必须给出准确指向 $.proxies[n]... 的 error 级问题。
func TestSelfCheckProtocolWireShapeGates(t *testing.T) {
	cases := []struct {
		name     string
		proxy    string
		wantPath string
	}{
		{
			name: "tailscale 无 endpoint",
			proxy: `
  - name: ts
    type: tailscale
    server: example.com
    port: 443`,
			wantPath: "$.proxies[0].server",
		},
		{
			name: "hysteria2 port 与 ports 互斥",
			proxy: `
  - name: hy2
    type: hysteria2
    server: example.com
    port: 443
    password: secret
    ports: 443-8443`,
			wantPath: "$.proxies[0].ports",
		},
		{
			name: "hysteria2 缺少 endpoint",
			proxy: `
  - name: hy2
    type: hysteria2
    server: example.com
    password: secret`,
			wantPath: "$.proxies[0].port",
		},
		{
			name: "hysteria2 未启用混淆却输出 obfs 凭据",
			proxy: `
  - name: hy2
    type: hysteria2
    server: example.com
    port: 443
    password: secret
    obfs-password: obfs-secret`,
			wantPath: "$.proxies[0].obfs-password",
		},
		{
			name: "mieru port 与 port-range 互斥",
			proxy: `
  - name: mieru
    type: mieru
    server: example.com
    port: 443
    port-range: 2000-3000
    username: user
    password: secret`,
			wantPath: "$.proxies[0].port-range",
		},
		{
			name: "mieru 缺少 endpoint",
			proxy: `
  - name: mieru
    type: mieru
    server: example.com
    username: user
    password: secret`,
			wantPath: "$.proxies[0].port",
		},
		{
			name: "wireguard peers 不得带顶层 endpoint",
			proxy: `
  - name: wg
    type: wireguard
    server: example.com
    port: 51820
    private-key: priv
    ip: 192.0.2.2/32
    peers:
      - server: peer-a
        port: 51820
        public-key: pub-a
        allowed-ips: [10.0.0.0/24]`,
			wantPath: "$.proxies[0].peers",
		},
		{
			name: "wireguard peer 缺少 allowed-ips",
			proxy: `
  - name: wg
    type: wireguard
    private-key: priv
    ip: 192.0.2.2/32
    peers:
      - server: peer-a
        port: 51820
        public-key: pub-a`,
			wantPath: "$.proxies[0].peers[0].allowed-ips",
		},
		{
			name: "tuic v4 与 v5 凭据互斥",
			proxy: `
  - name: tuic
    type: tuic
    server: example.com
    port: 443
    token: v4-token
    uuid: 11111111-2222-3333-4444-555555555555
    password: v5-password`,
			wantPath: "$.proxies[0].token",
		},
		{
			name: "tuic UOT 关闭却输出版本",
			proxy: `
  - name: tuic
    type: tuic
    server: example.com
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
    password: v5-password
    udp-over-stream-version: 2`,
			wantPath: "$.proxies[0].udp-over-stream-version",
		},
		{
			name: "trusttunnel 复用两组互斥",
			proxy: `
  - name: tt
    type: trusttunnel
    server: example.com
    port: 443
    max-connections: 8
    min-streams: 5
    max-streams: 0`,
			wantPath: "$.proxies[0].max-streams",
		},
		{
			name: "openvpn 认证组缺半",
			proxy: `
  - name: ovpn
    type: openvpn
    server: example.com
    port: 1194
    ca: CA-PEM
    username: only-user`,
			wantPath: "$.proxies[0].username",
		},
		{
			name: "openvpn 两种 TLS key 并存",
			proxy: `
  - name: ovpn
    type: openvpn
    server: example.com
    port: 1194
    ca: CA-PEM
    username: user
    password: pass
    tls-auth: TA
    tls-crypt: TC`,
			wantPath: "$.proxies[0].tls-crypt",
		},
		{
			name: "openvpn key-direction 依赖 tls-auth",
			proxy: `
  - name: ovpn
    type: openvpn
    server: example.com
    port: 1194
    ca: CA-PEM
    username: user
    password: pass
    key-direction: 1`,
			wantPath: "$.proxies[0].key-direction",
		},
		{
			name: "anytls 附加安全对象互斥",
			proxy: `
  - name: anytls
    type: anytls
    server: example.com
    port: 443
    password: secret
    shadow-tls-opts:
      password: st
      version: 3
    restls-opts:
      password: restls
      version-hint: tls13`,
			wantPath: "$.proxies[0].restls-opts",
		},
		{
			name: "shadowquic 不得输出固定 tag 没有的 TLS 字段",
			proxy: `
  - name: sq
    type: shadowquic
    server: example.com
    port: 443
    username: user
    password: secret
    skip-cert-verify: true`,
			wantPath: "$.proxies[0].skip-cert-verify",
		},
		{
			name: "snell v1 不得输出 udp",
			proxy: `
  - name: snell
    type: snell
    server: example.com
    port: 443
    psk: secret
    version: 1
    udp: true`,
			wantPath: "$.proxies[0].udp",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content := fmt.Appendf(nil, "proxies:\n%s\nrules:\n  - GEOIP,CN,DIRECT\n  - MATCH,DIRECT\n", tc.proxy)
			issues := CheckClashContent(content)
			for _, issue := range issues {
				if issue.Severity == "error" && issue.Path == tc.wantPath {
					return
				}
			}
			t.Fatalf("缺少路径 %s 的协议 shape 错误: %+v", tc.wantPath, issues)
		})
	}
}
