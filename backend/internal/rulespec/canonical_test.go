package rulespec

import "testing"

func TestCanonicalSemanticKey(t *testing.T) {
	a := CanonicalRule{Family: FamilyDomain, Matcher: MatcherExact, Value: "a.example.com"}
	b := CanonicalRule{Family: FamilyDomain, Matcher: MatcherExact, Value: "a.example.com"}
	if a.SemanticKey() != b.SemanticKey() {
		t.Fatalf("相同语义应产生相同 key: %q != %q", a.SemanticKey(), b.SemanticKey())
	}
	c := CanonicalRule{Family: FamilyDomain, Matcher: MatcherSuffix, Value: "a.example.com"}
	if a.SemanticKey() == c.SemanticKey() {
		t.Fatalf("exact 与 suffix 不应相同")
	}
	d := CanonicalRule{Family: FamilyDomain, Matcher: MatcherExact, Value: "a.example.com", Options: RuleOptions{NoResolve: true}}
	if a.SemanticKey() == d.SemanticKey() {
		t.Fatalf("带 no_resolve 的语义键应不同")
	}
}

func TestSupportsAndMapBasics(t *testing.T) {
	domain := CanonicalRule{Family: FamilyDomain, Matcher: MatcherExact, Value: "a.com"}
	if r := SupportsAndMap(domain, TargetClash); !r.Supported || r.RenderType != "DOMAIN" || r.TargetScope != ScopeCommon {
		t.Fatalf("domain/clash 映射异常: %+v", r)
	}
	if r := SupportsAndMap(domain, TargetSR); !r.Supported || r.RenderType != "DOMAIN" || r.TargetScope != ScopeCommon {
		t.Fatalf("domain/sr 映射异常: %+v", r)
	}

	ua := CanonicalRule{Family: FamilyUserAgent, Matcher: MatcherExact, Value: "curl"}
	if r := SupportsAndMap(ua, TargetClash); r.Supported {
		t.Fatalf("USER-AGENT 不应支持 Clash: %+v", r)
	}
	if r := SupportsAndMap(ua, TargetSR); !r.Supported || r.RenderType != "USER-AGENT" || r.TargetScope != ScopeSrOnly {
		t.Fatalf("USER-AGENT/SR 映射异常: %+v", r)
	}

	asn := CanonicalRule{Family: FamilyIP, Matcher: MatcherASN, Value: "13335"}
	if r := SupportsAndMap(asn, TargetClash); !r.Supported || r.RenderType != "IP-ASN" || !r.SupportsNoResolve {
		t.Fatalf("IP-ASN/Clash 映射异常: %+v", r)
	}
	if r := SupportsAndMap(asn, TargetSR); !r.Supported || r.RenderType != "IP-ASN" {
		t.Fatalf("IP-ASN/SR 映射异常: %+v", r)
	}

	ipv4 := CanonicalRule{Family: FamilyIP, Matcher: MatcherCIDR, Value: "1.0.1.0/24"}
	if r := SupportsAndMap(ipv4, TargetClash); !r.Supported || r.RenderType != "IP-CIDR" {
		t.Fatalf("IPv4 CIDR/Clash 映射异常: %+v", r)
	}
	ipv6 := CanonicalRule{Family: FamilyIP, Matcher: MatcherCIDR, Value: "2001:db8::/32"}
	if r := SupportsAndMap(ipv6, TargetClash); !r.Supported || r.RenderType != "IP-CIDR6" {
		t.Fatalf("IPv6 CIDR/Clash 映射异常: %+v", r)
	}
	if r := SupportsAndMap(ipv6, TargetSR); !r.Supported || r.RenderType != "IP-CIDR6" {
		t.Fatalf("IPv6 CIDR/SR 映射异常: %+v", r)
	}
}

func TestAdvancedOnlyNotMaterial(t *testing.T) {
	for _, c := range Capabilities() {
		if c.Family == FamilyProcess && c.Matcher == MatcherEquals && c.Advanced && !c.MaterialPool {
			return
		}
	}
	t.Fatal("应存在 advanced-only 且不可进入素材池的能力")
}

func TestCanonicalizeLegacyType(t *testing.T) {
	family, matcher, ok := CanonicalizeLegacyType("DOMAIN-SUFFIX")
	if !ok || family != FamilyDomain || matcher != MatcherSuffix {
		t.Fatalf("DOMAIN-SUFFIX 映射异常: %q %q %v", family, matcher, ok)
	}
	_, _, ok = CanonicalizeLegacyType("NOPE")
	if ok {
		t.Fatal("未知类型不应映射")
	}
}

func TestMetadataStable(t *testing.T) {
	a := Metadata()
	b := Metadata()
	if len(a) != len(b) {
		t.Fatalf("元数据长度不稳定: %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("元数据顺序不稳定: %d", i)
		}
	}
	if len(a) == 0 {
		t.Fatal("元数据不能为空")
	}
}
