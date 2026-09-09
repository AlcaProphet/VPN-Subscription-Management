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
			issues := CheckClashContent([]byte(fmt.Sprintf(base, tc.plugin)))
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
