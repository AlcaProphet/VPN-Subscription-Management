package assembly

import (
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
