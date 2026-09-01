package rulespec

import "testing"

func TestDefinitionsRequireValuesExceptMatch(t *testing.T) {
	if len(Definitions) != 34 {
		t.Fatalf("规则定义应为 34 项，实际 %d", len(Definitions))
	}
	for typ, def := range Definitions {
		if typ != "MATCH" && !def.ValueRequired {
			t.Errorf("%s 必须显式要求匹配值", typ)
		}
	}
}

func TestValidateValue(t *testing.T) {
	valid := []struct{ typ, value string }{
		{"GEOSITE", "cn"}, {"IP-ASN", "45102"}, {"SRC-PORT", "443"}, {"NETWORK", "UDP"},
		{"DOMAIN-REGEX", `^example\.(com|net)$`}, {"AND", "((DOMAIN,a.com),(NETWORK,tcp))"}, {"MATCH", ""},
	}
	for _, item := range valid {
		if _, _, err := ValidateValue(item.typ, item.value); err != nil {
			t.Errorf("%s,%s 应合法: %v", item.typ, item.value, err)
		}
	}
	invalid := []struct{ typ, value string }{{"MATCH", "x"}, {"DST-PORT", "70000"}, {"NETWORK", "icmp"}, {"AND", "((DOMAIN,a.com)"}, {"DOMAIN-REGEX", "["}}
	for _, item := range invalid {
		if _, _, err := ValidateValue(item.typ, item.value); err == nil {
			t.Errorf("%s,%s 应被拒绝", item.typ, item.value)
		}
	}
}

func TestParseRenderedLogicalAndNoResolve(t *testing.T) {
	typ, value, target, noResolve, err := ParseRendered("AND,((DOMAIN,a.com),(NETWORK,tcp)),代理组")
	if err != nil || typ != "AND" || value != "((DOMAIN,a.com),(NETWORK,tcp))" || target != "代理组" || noResolve {
		t.Fatalf("逻辑规则解析异常: %q %q %q %v %v", typ, value, target, noResolve, err)
	}
	_, _, _, noResolve, err = ParseRendered("IP-ASN,45102,代理组,no-resolve")
	if err != nil || !noResolve {
		t.Fatalf("no-resolve 解析异常: %v %v", noResolve, err)
	}
}
