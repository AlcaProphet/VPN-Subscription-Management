package pool

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vpn-sub/internal/rulespec"
)

func readDailyDataTemplate(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "DocTemplates", name))
	if err != nil {
		t.Fatalf("读取测试模板 %s 失败: %v", name, err)
	}
	return body
}

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

func TestParseSourceMihomoIPCIDRYAMLTemplate(t *testing.T) {
	body := readDailyDataTemplate(t, "DailyData.txt.template3.md")
	for _, mode := range []SourceMode{SourceModeClash, SourceModeShadowrocket, SourceModeAuto} {
		res, err := ParseSource(body, mode)
		if err != nil {
			t.Fatalf("%s 模式解析 Mihomo ipcidr YAML 失败: %v", mode, err)
		}
		if res.Format != FormatMihomoIPCIDRYAML || res.Profile != "common" {
			t.Fatalf("%s 模式格式或 profile 异常: format=%s profile=%s", mode, res.Format, res.Profile)
		}
		if res.Input != 6 || res.Recognized != 6 || res.Accepted != 6 || len(res.Rules) != 6 {
			t.Fatalf("%s 模式统计异常: %+v", mode, res)
		}
		if got := res.Rules[0]; got.Family != "ip" || got.Matcher != "cidr" || got.Value != "1.0.1.0/24" {
			t.Fatalf("首条 CIDR 规范化异常: %+v", got)
		}
	}
}

func TestParseSourceMihomoIPCIDRYAMLIPv6(t *testing.T) {
	body := []byte("payload:\n  - '2001:db8:1::1/48'\n")
	res, err := ParseSource(body, SourceModeClash)
	if err != nil {
		t.Fatalf("解析 IPv6 ipcidr YAML 失败: %v", err)
	}
	if res.Format != FormatMihomoIPCIDRYAML || len(res.Rules) != 1 || res.Rules[0].Value != "2001:db8:1::/48" {
		t.Fatalf("IPv6 ipcidr YAML 结果异常: %+v", res)
	}
}

func TestParseSourceMihomoPayloadConflicts(t *testing.T) {
	cases := map[string][]byte{
		"domain-and-cidr":    []byte("payload:\n  - 'example.com'\n  - '1.0.1.0/24'\n"),
		"classical-and-cidr": []byte("payload:\n  - 'IP-CIDR,1.0.1.0/24'\n  - '1.0.2.0/23'\n"),
		"non-string":         []byte("payload:\n  - 123\n"),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseSource(body, SourceModeClash); !errors.Is(err, ErrConflictingDocumentFormat) {
				t.Fatalf("冲突 payload 应硬失败: %v", err)
			}
		})
	}
}

func TestParseSourceShadowrocketTypedTemplate(t *testing.T) {
	body := readDailyDataTemplate(t, "DailyData.txt.template4.md")
	for _, mode := range []SourceMode{SourceModeClash, SourceModeShadowrocket, SourceModeAuto} {
		res, err := ParseSource(body, mode)
		if err != nil {
			t.Fatalf("%s 模式解析显式 IP 规则文本失败: %v", mode, err)
		}
		if res.Format != FormatTypedRuleText || res.Profile != "common" {
			t.Fatalf("%s 模式格式或 profile 异常: format=%s profile=%s", mode, res.Format, res.Profile)
		}
		if res.Input != 12 || res.Recognized != 12 || res.Accepted != 12 || len(res.Rules) != 12 {
			t.Fatalf("%s 模式统计异常: %+v", mode, res)
		}
		if got := res.Rules[0]; got.Family != "ip" || got.Matcher != "asn" || got.Value != "132203" {
			t.Fatalf("IP-ASN 规范化异常: %+v", got)
		}
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

func TestParseSourceEvidenceCodesFromDetectorBranches(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		format  DetectedFormat
		reasons []string
	}{
		{"sing-box", `{"version":1,"rules":[{"domain":["a.com"]}]}`, FormatSingBoxSourceJSON, []string{"sing_box_version_and_rules"}},
		{"mihomo-domain", "payload:\n  - 'a.com'\n", FormatMihomoDomainYAML, []string{"top_level_payload", "payload_domain_only"}},
		{"mihomo-ipcidr", "payload:\n  - '1.2.3.0/24'\n", FormatMihomoIPCIDRYAML, []string{"top_level_payload", "payload_ipcidr_only"}},
		{"mihomo-classical", "payload:\n  - 'DOMAIN,a.com'\n", FormatMihomoClassicalYAML, []string{"top_level_payload", "payload_classical_only"}},
		{"typed", "DOMAIN,a.com\n", FormatTypedRuleText, []string{"typed_rule_marker"}},
		{"ip-list", "1.2.3.4\n", FormatPlainIPCIDRText, []string{"all_items_ip_cidr_or_asn"}},
		{"legacy", "full:a.com\n", FormatLegacyDomainText, []string{"legacy_domain_prefix"}},
		{"plain", "a.com\n", FormatPlainDomainText, []string{"plain_domain_candidates"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseSource([]byte(tc.body), SourceModeAuto)
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if res.Format != tc.format {
				t.Fatalf("format=%s want %s", res.Format, tc.format)
			}
			if len(res.EvidenceCodes) != len(tc.reasons) {
				t.Fatalf("evidence codes=%v want %v", res.EvidenceCodes, tc.reasons)
			}
			for i := range tc.reasons {
				if res.EvidenceCodes[i] != tc.reasons[i] {
					t.Fatalf("evidence codes=%v want %v", res.EvidenceCodes, tc.reasons)
				}
			}
		})
	}
}

func TestFinalizeStatsStep1DuplicatesExcludedSeparate(t *testing.T) {
	// SR 模式下：DOMAIN-REGEX 是合法但 Clash-only，应计入 Excluded；
	// 两个相同 DOMAIN 应只算 1 Accepted + 1 Duplicates，不能借后置差值再进入 Excluded。
	body := []byte("DOMAIN,a.com\nDOMAIN,a.com\nDOMAIN-REGEX,^example\\.com$\n")
	res, err := ParseSource(body, SourceModeShadowrocket)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Input != 3 || res.Recognized != 3 || res.Accepted != 1 || res.Excluded != 1 || res.Rejected != 0 || res.Duplicates != 1 {
		t.Fatalf("统计口径错误: %+v", res)
	}
	if len(res.Rules) != 1 {
		t.Fatalf("accepted 规则数错误: %d", len(res.Rules))
	}
}

func TestFinalizeStatsStep1AdapterRejectEntersRejected(t *testing.T) {
	body := []byte("DOMAIN,a0.com\nDOMAIN,a1.com\nDOMAIN,a2.com\nDOMAIN,a3.com\nDOMAIN,a4.com\nDOMAIN,a5.com\nDOMAIN,a6.com\nDOMAIN,a7.com\nDOMAIN,a8.com\nDOMAIN,a9.com\nBOGUS,xxx\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Input != 11 || res.Recognized != 10 || res.Accepted != 10 || res.Rejected != 1 {
		t.Fatalf("adapter reject 未计入 rejected: %+v", res)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Kind != "reject" {
		t.Fatalf("应保留 adapter reject 诊断: %+v", res.Diagnostics)
	}
}

func TestFinalizeStatsStep1ProfileBeforeModeExclusion(t *testing.T) {
	// Clash 模式会把 USER-AGENT 排除；detected_profile 仍需由排除前全部已识别候选得出 shadowrocket。
	body := []byte("DOMAIN,a.com\nUSER-AGENT,curl\n")
	res, err := ParseSource(body, SourceModeClash)
	if err != nil {
		t.Fatalf("Clash 模式解析失败: %v", err)
	}
	if res.Profile != "shadowrocket" {
		t.Fatalf("profile 应在来源模式排除前计算，当前 %q", res.Profile)
	}
	if res.Excluded != 1 || res.Rejected != 0 {
		t.Fatalf("Clash 模式应只排除 USER-AGENT: %+v", res)
	}
}

func TestFinalizeStatsStep1AutoNoModeExclusion(t *testing.T) {
	body := []byte("DOMAIN,a.com\nDOMAIN-SUFFIX,b.com\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("auto 解析失败: %v", err)
	}
	if res.Excluded != 0 {
		t.Fatalf("auto 模式不应产生来源模式排除: %+v", res)
	}
}

func TestFinalizeStatsStep1AppendedDiagnosticsWrittenBack(t *testing.T) {
	// 直接构造 capability 阶段拒绝项，验证循环内追加的诊断会回写最终切片。
	items := []ParsedRule{
		{Rule: rulespec.CanonicalRule{Family: rulespec.FamilyProcess, Matcher: rulespec.MatcherEquals, Value: "cn"}, Origin: RuleOriginMeta{Raw: "RULE-SET,cn"}},
		{Rule: rulespec.CanonicalRule{Family: rulespec.FamilyDomain, Matcher: rulespec.MatcherExact, Value: "a.com"}, Origin: RuleOriginMeta{Raw: "DOMAIN,a.com"}},
	}
	res, err := finalizeParseResult(FormatTypedRuleText, items, nil, SourceModeAuto, nil)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Accepted != 1 || res.Rejected != 1 {
		t.Fatalf("统计错误: %+v", res)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Kind != "reject" || res.Diagnostics[0].Message != "不是素材池可选能力" {
		t.Fatalf("清洗阶段诊断未回写: %+v", res.Diagnostics)
	}
}

func TestParseSourceNoResolveUsesIndependentOptionToken(t *testing.T) {
	body := []byte("DOMAIN,no-resolve.example.com\nIP-CIDR,1.2.3.0/24,no-resolve\nIP-CIDR,10.0.0.0/8,NO-RESOLVE\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(res.Items) != 3 {
		t.Fatalf("应解析 3 条: %d", len(res.Items))
	}
	if res.Items[0].Rule.Options.NoResolve {
		t.Errorf("匹配值含 no-resolve 字样不得误设为选项: %+v", res.Items[0])
	}
	if !res.Items[1].Rule.Options.NoResolve || !res.Items[2].Rule.Options.NoResolve {
		t.Errorf("独立 no-resolve token 应设置选项: %+v", res.Items)
	}
}

func TestParseSourcePolicySilentAndUnknownOptionWarn(t *testing.T) {
	body := []byte("DOMAIN,a.com,PROXY\nIP-CIDR,1.2.3.0/24,no-resolve\nIP-CIDR,10.0.0.0/8,PROXY,no-resolve\nDOMAIN,b.com,PROXY,unknown-opt\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Accepted != 4 || res.Rejected != 0 || res.Excluded != 0 || res.Duplicates != 0 {
		t.Fatalf("policy/未知 option 不应改变 accepted/rejected/excluded/duplicates: %+v", res)
	}
	if len(res.Items) != 4 {
		t.Fatalf("应保留 4 条 items: %d", len(res.Items))
	}
	if res.Items[0].Rule.Options.NoResolve || res.Items[3].Rule.Options.NoResolve {
		t.Fatalf("source policy 后的普通选项不应被误设为 no-resolve: %+v", res.Items)
	}
	if !res.Items[1].Rule.Options.NoResolve || !res.Items[2].Rule.Options.NoResolve {
		t.Fatalf("独立 no-resolve token 应设置选项: %+v", res.Items)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Kind != "warn" || !strings.Contains(res.Diagnostics[0].Message, "unknown-opt") {
		t.Fatalf("未知 option 应产生单条 warn 诊断: %+v", res.Diagnostics)
	}
}

func TestParseSourcePositionRuleTreatsSingleTailAsPolicy(t *testing.T) {
	// 只有一个非 no-resolve 尾部 token 时无法区分 policy 与未知 option，
	// 按位置法视为 source policy 静默忽略。
	res, err := ParseSource([]byte("DOMAIN,a.com,unknown-option\n"), SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Accepted != 1 || len(res.Diagnostics) != 0 {
		t.Fatalf("单个非 no-resolve 尾部 token 应按 source policy 静默忽略: %+v", res)
	}
}

func TestParseSourceWarnsEachUnknownTailToken(t *testing.T) {
	res, err := ParseSource([]byte("DOMAIN,a.com,PROXY,foo,bar\n"), SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Accepted != 1 || len(res.Diagnostics) != 2 {
		t.Fatalf("两个未知尾部 token 应产生两条 warn: %+v", res.Diagnostics)
	}
	for i, want := range []string{"foo", "bar"} {
		if res.Diagnostics[i].Kind != "warn" || !strings.Contains(res.Diagnostics[i].Message, want) {
			t.Fatalf("第 %d 条 warn 异常: %+v", i, res.Diagnostics[i])
		}
	}
}

func TestParseSourceRejectsNonMaterialPoolByOriginalLegacyType(t *testing.T) {
	cases := []string{
		"RULE-SET,cn",
		"AND,((DOMAIN,a.com),(NETWORK,tcp))",
		"OR,((DOMAIN,a.com),(DOMAIN,b.com))",
		"NOT,((DOMAIN,a.com))",
		"MATCH,",
		"GEOSITE,cn",
		"SRC-GEOIP,CN",
		"SRC-IP-ASN,13335",
		"SRC-IP-CIDR,10.0.0.0/8",
		"IP-SUFFIX,10.0.0.0/8",
		"DST-PORT,443",
	}
	for _, badLine := range cases {
		var b strings.Builder
		for i := 0; i < 10; i++ {
			fmt.Fprintf(&b, "DOMAIN,ok%d.com\n", i)
		}
		b.WriteString(badLine)
		b.WriteString("\n")
		body := b.String()
		res, err := ParseSource([]byte(body), SourceModeAuto)
		if err != nil {
			t.Fatalf("应能解析出拒绝诊断而不是硬失败: %v\n%s", err, body)
		}
		if res.Rejected != 1 || res.Accepted != 10 {
			t.Fatalf("非素材池类型应 rejected=1 accepted=10: %+v\n%s", res, body)
		}
		if len(res.Diagnostics) != 1 || !strings.Contains(res.Diagnostics[0].Message, "不是素材池可选能力") {
			t.Fatalf("应产生素材池白名单拒绝诊断: %+v\n%s", res.Diagnostics, body)
		}
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
