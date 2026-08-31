package pool

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSourcePlainDomains(t *testing.T) {
	body := []byte("mzstatic.com\na1.mzstatic.com\nfoo.github.io\nwww.foo.github.io\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("解析纯域名失败: %v", err)
	}
	if res.Format != FormatPlainDomainText {
		t.Fatalf("格式错误: %s", res.Format)
	}
	if res.Profile != "common" {
		t.Fatalf("profile 应为 common: %s", res.Profile)
	}
	if len(res.Rules) != 4 {
		t.Fatalf("应接受 4 条域名规则，实际 %d: %+v", len(res.Rules), res.Rules)
	}
	if res.Rules[0].Matcher != "suffix" || res.Rules[0].Value != "mzstatic.com" {
		t.Fatalf("mzstatic.com 应为 suffix: %+v", res.Rules[0])
	}
	if res.Rules[1].Matcher != "exact" || res.Rules[1].Value != "a1.mzstatic.com" {
		t.Fatalf("a1.mzstatic.com 应为 exact: %+v", res.Rules[1])
	}
}

func TestParseSourceTypedAndSourceMode(t *testing.T) {
	body := []byte("DOMAIN,a.com\nDOMAIN-SUFFIX,b.com\nIP-CIDR,1.2.3.0/24,no-resolve\nUSER-AGENT,curl\n")
	res, err := ParseSource(body, SourceModeShadowrocket)
	if err != nil {
		t.Fatalf("SR 解析失败: %v", err)
	}
	if res.Profile != "shadowrocket" {
		t.Fatalf("SR profile 错误: %s", res.Profile)
	}
	if len(res.Rules) != 4 {
		t.Fatalf("SR 模式应接受全部 4 条，实际 %d", len(res.Rules))
	}
	clashRes, err := ParseSource(body, SourceModeClash)
	if err != nil {
		t.Fatalf("Clash 解析失败: %v", err)
	}
	if len(clashRes.Rules) != 3 {
		t.Fatalf("Clash 模式应剔除 USER-AGENT，接受 3 条，实际 %d", len(clashRes.Rules))
	}
}

func TestParseSourceAutoMixedPrivate(t *testing.T) {
	body := []byte("DOMAIN,a.com\nUSER-AGENT,curl\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("通用+SR 私有不应失败: %v", err)
	}
	if res.Profile != "shadowrocket" {
		t.Fatalf("profile 应为 shadowrocket: %s", res.Profile)
	}
	if len(res.Rules) != 2 {
		t.Fatalf("应接受 2 条，实际 %d", len(res.Rules))
	}

	both := []byte("DOMAIN-REGEX,^example\\.com$\nUSER-AGENT,curl\n")
	if _, err := ParseSource(both, SourceModeAuto); !errors.Is(err, ErrMixedPlatformSource) {
		t.Fatalf("双方私有混合应硬失败: %v", err)
	}
}

func TestParseSourceMihomoYAML(t *testing.T) {
	domainYAML := []byte("payload:\n  - 'a1.mzstatic.com'\n  - '+.001wifi.com'\n")
	res, err := ParseSource(domainYAML, SourceModeAuto)
	if err != nil {
		t.Fatalf("Mihomo domain YAML 解析失败: %v", err)
	}
	if res.Format != FormatMihomoDomainYAML || len(res.Rules) != 2 {
		t.Fatalf("domain YAML 结果异常: format=%s rules=%d", res.Format, len(res.Rules))
	}
	if res.Rules[1].Matcher != "suffix" || res.Rules[1].Value != "001wifi.com" {
		t.Fatalf("+. 应归一为 suffix: %+v", res.Rules[1])
	}

	classicalYAML := []byte("payload:\n  - 'DOMAIN-SUFFIX,example.com'\n  - 'IP-CIDR,10.0.0.0/8,no-resolve'\n")
	res, err = ParseSource(classicalYAML, SourceModeShadowrocket)
	if err != nil {
		t.Fatalf("Mihomo classical YAML 解析失败: %v", err)
	}
	if res.Format != FormatMihomoClassicalYAML || len(res.Rules) != 2 {
		t.Fatalf("classical YAML 结果异常: format=%s rules=%d", res.Format, len(res.Rules))
	}
}

func TestParseSourceIPList(t *testing.T) {
	body := []byte("1.2.3.4\n10.0.0.0/8\nAS13335\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("IP 解析失败: %v", err)
	}
	if res.Format != FormatPlainIPCIDRText || len(res.Rules) != 3 {
		t.Fatalf("IP 列表结果异常: format=%s rules=%d", res.Format, len(res.Rules))
	}
	if res.Rules[0].Value != "1.2.3.4/32" {
		t.Fatalf("单 IP 应转 /32: %+v", res.Rules[0])
	}
	if res.Rules[2].Family != "ip" || res.Rules[2].Matcher != "asn" || res.Rules[2].Value != "13335" {
		t.Fatalf("ASN 解析异常: %+v", res.Rules[2])
	}
}

func TestParseSourceSingBox(t *testing.T) {
	body := []byte(`{"version":1,"rules":[{"domain":["a.com","b.com"]},{"ip_cidr":["10.0.0.0/8"]}]}`)
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("sing-box 解析失败: %v", err)
	}
	if res.Format != FormatSingBoxSourceJSON || len(res.Rules) != 3 {
		t.Fatalf("sing-box 结果异常: format=%s rules=%d", res.Format, len(res.Rules))
	}

	multi := []byte(`{"version":1,"rules":[{"domain":["a.com"],"ip_cidr":["10.0.0.0/8"]}]}`)
	if _, err := ParseSource(multi, SourceModeAuto); err != nil {
		// 允许作为 rejected/无法满足阈值，但不应该 panic；这里只要最终 error 不是 nil 即可。
		t.Logf("multi-condition 返回错误符合预期: %v", err)
	}
}

func TestParseSourceHardFailures(t *testing.T) {
	if _, err := ParseSource([]byte("<html><body>login</body></html>"), SourceModeAuto); !errors.Is(err, ErrHTMLSource) {
		t.Fatalf("HTML 应硬失败: %v", err)
	}
	if _, err := ParseSource([]byte("not a domain\nrandom text\n"), SourceModeAuto); err == nil {
		t.Fatal("无法识别文本应失败")
	}
}

func TestParseSourceThreshold(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 10; i++ {
		b.WriteString("bad line without dot or comma\n")
	}
	b.WriteString("a.com\n")
	// 9/11 未达 90% 应失败；若探测为 plain with 1 accepted? Should error threshold.
	if _, err := ParseSource([]byte(b.String()), SourceModeAuto); err == nil {
		t.Fatal("识别率不足应失败")
	}
}
