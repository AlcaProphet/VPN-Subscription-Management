// field_types_test.go：Step 3.5 公共字段类型（multiline／secret-multiline／byte-sequence）合同测试。
package node

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// fieldTypeTestProtocol 构造仅用于 Step 3.5 公共字段类型合同的协议。
// byte-sequence 当前没有生产协议使用（WireGuard reserved 的重定型属于 Step 11）。
func fieldTypeTestProtocol() Protocol {
	return Protocol{
		Protocol: "field-type-test",
		FormSchema: []FieldSchema{
			{Name: "ca", Type: "multiline"},
			{Name: "private-key", Type: "secret-multiline"},
			{Name: "reserved", Type: "byte-sequence"},
			{Name: "peer", Type: "object", ObjectKind: "list", Properties: []FieldSchema{
				{Name: "reserved", Type: "byte-sequence"},
			}},
		},
		SensitiveFields: []string{"private-key"},
	}
}

func TestMultilineFieldValidatesString(t *testing.T) {
	proto := fieldTypeTestProtocol()
	for _, name := range []string{"ca", "private-key"} {
		var field FieldSchema
		for _, candidate := range proto.FormSchema {
			if candidate.Name == name {
				field = candidate
			}
		}
		if field.Name == "" {
			t.Fatalf("测试协议缺少字段 %s", name)
		}
		if err := validateFieldValue(field, "-----BEGIN CERTIFICATE-----\nMIIB\n", name); err != nil {
			t.Fatalf("字段 %s 应接受多行字符串: %v", name, err)
		}
		if err := validateFieldValue(field, 7, name); err == nil {
			t.Fatalf("字段 %s 应拒绝非字符串值", name)
		}
		if err := validateFieldValue(field, map[string]any{"a": 1}, name); err == nil {
			t.Fatalf("字段 %s 应拒绝对象值", name)
		}
	}
}

func TestByteSequenceFieldValidation(t *testing.T) {
	field := FieldSchema{Name: "reserved", Type: "byte-sequence"}
	valid := []any{
		[]any{1, 2, 3},
		[]int{0, 255, 128},
		[]float64{1, 2, 3},
		"1,2,3",
		"1 2 3",
		"AQID",
	}
	for _, value := range valid {
		if err := validateFieldValue(field, value, "reserved"); err != nil {
			t.Fatalf("合法 byte-sequence 输入 %#v 被拒绝: %v", value, err)
		}
	}
	invalid := []any{
		[]any{1, 2},
		[]any{1, 2, 3, 4},
		[]any{256, 0, 0},
		[]any{-1, 0, 0},
		[]any{"1", "2", "3"},
		"AQ",
		"not-base64!!",
		"1,2",
		7,
		nil,
	}
	for _, value := range invalid {
		if err := validateFieldValue(field, value, "reserved"); err == nil {
			t.Fatalf("非法 byte-sequence 输入 %#v 应被拒绝", value)
		}
	}
}

func TestByteSequenceNormalizedOnNormalize(t *testing.T) {
	proto := fieldTypeTestProtocol()
	cases := []struct {
		in   any
		want []int
	}{
		{[]any{1, 2, 3}, []int{1, 2, 3}},
		{[]int{4, 5, 6}, []int{4, 5, 6}},
		{[]float64{7, 8, 9}, []int{7, 8, 9}},
		{"7,8,9", []int{7, 8, 9}},
		{"7 8 9", []int{7, 8, 9}},
		{"AQID", []int{1, 2, 3}},
		{"AAD/", []int{0, 0, 255}},
	}
	for _, tc := range cases {
		out, err := NormalizeProtocolJSON(proto, map[string]any{"reserved": tc.in})
		if err != nil {
			t.Fatalf("规范化 byte-sequence %#v 失败: %v", tc.in, err)
		}
		got, ok := out["reserved"].([]int)
		if !ok {
			t.Fatalf("byte-sequence 规范化结果类型应为 []int，实际 %#v", out["reserved"])
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("byte-sequence 规范化 %#v = %v，期望 %v", tc.in, got, tc.want)
		}
	}

	invalid := []any{"AQ", []any{1, 2}, []any{1, 2, 3, 4}, []any{256, 0, 0}, "not-base64!!", "1,2", 7}
	for _, value := range invalid {
		if _, err := NormalizeProtocolJSON(proto, map[string]any{"reserved": value}); err == nil {
			t.Fatalf("非法 byte-sequence 输入 %#v 应返回字段级错误", value)
		} else if !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("byte-sequence 错误应定位到字段路径，实际: %v", err)
		}
	}
}

// TestValidateProtocolFieldTypeRegistry 覆盖注册表字段类型白名单与 secret-multiline 敏感性门禁。
func TestValidateProtocolFieldTypeRegistry(t *testing.T) {
	if err := validateProtocolFieldTypes(fieldTypeTestProtocol()); err != nil {
		t.Fatalf("合法字段类型协议注册失败: %v", err)
	}
	cases := []struct {
		name  string
		proto Protocol
	}{
		{
			name:  "未知类型",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{{Name: "x", Type: "textarea"}}},
		},
		{
			name: "嵌套未知类型",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{
				{Name: "o", Type: "object", ObjectKind: "fields", Properties: []FieldSchema{{Name: "x", Type: "bad"}}},
			}},
		},
		{
			name:  "secret-multiline 未声明敏感",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{{Name: "k", Type: "secret-multiline"}}},
		},
		{
			name: "列表内 secret-multiline 未声明敏感",
			proto: Protocol{Protocol: "bad", FormSchema: []FieldSchema{
				{Name: "peers", Type: "object", ObjectKind: "list", Properties: []FieldSchema{{Name: "pre-shared-key", Type: "secret-multiline"}}},
			}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateProtocolFieldTypes(tc.proto); err == nil {
				t.Fatalf("非法字段类型协议应注册失败")
			}
		})
	}

	declared := Protocol{
		Protocol: "declared",
		FormSchema: []FieldSchema{
			{Name: "peers", Type: "object", ObjectKind: "list", Properties: []FieldSchema{{Name: "pre-shared-key", Type: "secret-multiline"}}},
		},
		SensitiveFields: []string{"peers[].pre-shared-key"},
	}
	if err := validateProtocolFieldTypes(declared); err != nil {
		t.Fatalf("列表内以 [] 形式声明敏感路径应通过: %v", err)
	}
}

// TestRegistryFieldTypesKnown 保证全部 19 个已注册协议都满足字段类型与敏感性门禁。
func TestRegistryFieldTypesKnown(t *testing.T) {
	for _, proto := range ManualProtocols() {
		if err := validateProtocolFieldTypes(proto); err != nil {
			t.Fatalf("协议 %s 字段类型合同不成立: %v", proto.Protocol, err)
		}
	}
}

// TestLargeTextFieldsUseExplicitTypes 锁定"停止依赖字段名猜测大文本"的显式类型合同：
// 多行文本字段必须为 multiline（Snell 的 restls-script 按敏感合同为 secret-multiline），
// 私钥必须为 secret-multiline；SSH 的 Host Key 字段按 Build32 Step 6 改为结构化 text-list。
func TestLargeTextFieldsUseExplicitTypes(t *testing.T) {
	multilineNames := map[string][]string{
		"certificate": {"multiline"}, "ca": {"multiline"}, "ca-str": {"multiline"},
		"client-config": {"multiline"}, "restls-script": {"multiline", "secret-multiline"},
	}
	textListNames := map[string]bool{
		"host-key": true, "host-key-algorithms": true,
	}
	var walk func(t *testing.T, protocol string, fields []FieldSchema, prefix string)
	walk = func(t *testing.T, protocol string, fields []FieldSchema, prefix string) {
		for _, field := range fields {
			path := field.Name
			if prefix != "" {
				path = prefix + "." + field.Name
			}
			if allowed, ok := multilineNames[field.Name]; ok && !slices.Contains(allowed, field.Type) {
				t.Fatalf("协议 %s 字段 %s 应使用显式多行类型 %v，实际 %s", protocol, path, allowed, field.Type)
			}
			if textListNames[field.Name] && field.Type != "text-list" {
				t.Fatalf("协议 %s 字段 %s 应使用结构化 text-list 类型，实际 %s", protocol, path, field.Type)
			}
			if field.Name == "private-key" && field.Type != "secret-multiline" {
				t.Fatalf("协议 %s 字段 %s 应使用显式 secret-multiline 类型，实际 %s", protocol, path, field.Type)
			}
			if field.Type == "object" {
				walk(t, protocol, field.Properties, path)
			}
		}
	}
	for _, proto := range ManualProtocols() {
		walk(t, proto.Protocol, proto.FormSchema, "")
	}
}

// TestByteSequenceNormalizedInNestedList 覆盖 Step 11 多 Peer 场景所需的嵌套列表规范化。
func TestByteSequenceNormalizedInNestedList(t *testing.T) {
	proto := fieldTypeTestProtocol()
	out, err := NormalizeProtocolJSON(proto, map[string]any{
		"peer": []any{
			map[string]any{"reserved": "AQID"},
			map[string]any{"reserved": []any{4, 5, 6}},
		},
	})
	if err != nil {
		t.Fatalf("嵌套 byte-sequence 规范化失败: %v", err)
	}
	items, ok := out["peer"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("嵌套列表结构被破坏: %#v", out["peer"])
	}
	first, _ := items[0].(map[string]any)
	second, _ := items[1].(map[string]any)
	if !reflect.DeepEqual(first["reserved"], []int{1, 2, 3}) {
		t.Fatalf("首条 reserved = %#v", first["reserved"])
	}
	if !reflect.DeepEqual(second["reserved"], []int{4, 5, 6}) {
		t.Fatalf("次条 reserved = %#v", second["reserved"])
	}
}
