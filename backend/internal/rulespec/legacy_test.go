// legacy_test.go：LegacyMetadata 的前端 JSON 契约回归测试。
package rulespec

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLegacyMetadataJSONUsesSnakeCase(t *testing.T) {
	data, err := json.Marshal(LegacyMetadata())
	if err != nil {
		t.Fatalf("序列化 LegacyMetadata 失败: %v", err)
	}
	for _, key := range []string{
		`"rule_type"`,
		`"scope"`,
		`"clash_render_type"`,
		`"sr_render_type"`,
		`"supports_no_resolve"`,
		`"material_pool"`,
		`"advanced"`,
	} {
		if !strings.Contains(string(data), key) {
			t.Errorf("LegacyMetadata JSON 缺少前端契约字段 %s", key)
		}
	}
	if strings.Contains(string(data), `"RuleType"`) || strings.Contains(string(data), `"MaterialPool"`) {
		t.Error("LegacyMetadata JSON 仍包含 Go 默认 PascalCase 字段名")
	}
}

func TestLegacyMetadataIncludesMaterialPoolOptions(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range LegacyMetadata() {
		if m.MaterialPool {
			seen[m.RuleType] = true
		}
	}
	for _, typ := range []string{"DOMAIN", "DOMAIN-SUFFIX", "IP-CIDR"} {
		if !seen[typ] {
			t.Errorf("素材池下拉应包含 %s，实际未返回 MaterialPool=true", typ)
		}
	}
}

func TestIsMaterialPoolTypeUsesOriginalLegacyType(t *testing.T) {
	for _, typ := range []string{"DOMAIN", "DOMAIN-SUFFIX", "IP-CIDR", "IP-CIDR6", "USER-AGENT", "PROCESS-NAME"} {
		if !IsMaterialPoolType(typ) {
			t.Errorf("%s 应允许进入素材池", typ)
		}
	}
	for _, typ := range []string{"RULE-SET", "AND", "OR", "NOT", "MATCH", "GEOSITE", "SRC-GEOIP", "SRC-IP-ASN", "SRC-IP-CIDR", "IP-SUFFIX", "DST-PORT"} {
		if IsMaterialPoolType(typ) {
			t.Errorf("%s 不应允许进入素材池", typ)
		}
	}
	if IsMaterialPoolType("UNKNOWN-TYPE") {
		t.Error("未知类型不应进入素材池")
	}
	if !IsMaterialPoolType("domain") || IsMaterialPoolType("src-geoip") {
		t.Error("大小写规范化后仍应按原始 legacy 类型判定")
	}
}
