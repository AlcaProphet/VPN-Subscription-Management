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
