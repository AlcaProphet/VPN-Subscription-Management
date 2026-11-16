// field_types.go：Build32 Step 3.5 公共字段类型合同。
// 明确 multiline／secret-multiline 多行文本类型，并为 WireGuard reserved 提供
// byte-sequence 类型；规范化与校验在此集中实现，供保存、检查与 URI 导入共用。
package node

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// knownFieldTypes 是注册表允许的字段类型全集。
var knownFieldTypes = map[string]bool{
	"text":             true,
	"password":         true,
	"select":           true,
	"text-list":        true,
	"int-list":         true,
	"number":           true,
	"bool":             true,
	"object":           true,
	"multiline":        true,
	"secret-multiline": true,
	"byte-sequence":    true,
}

// validateProtocolFieldTypes 递归校验字段类型集合与 secret-multiline 的敏感性声明。
// 未知类型或敏感路径缺失直接阻断注册表初始化，避免隐式类型在保存阶段才失败。
func validateProtocolFieldTypes(p Protocol) error {
	sensitive := make(map[string]bool, len(p.SensitiveFields))
	for _, path := range p.SensitiveFields {
		sensitive[path] = true
	}
	var walk func([]FieldSchema, string) error
	walk = func(fields []FieldSchema, prefix string) error {
		for _, field := range fields {
			path := field.Name
			if prefix != "" {
				path = prefix + "." + field.Name
			}
			if !knownFieldTypes[field.Type] {
				return fmt.Errorf("字段 %s 使用未知类型 %s", path, field.Type)
			}
			if field.Type == "secret-multiline" && !sensitive[path] {
				return fmt.Errorf("secret-multiline 字段 %s 必须在 SensitiveFields 中声明", path)
			}
			// clear_when_inactive 只在 selector 维度上有意义；缺失条件会让字段永远不被清空。
			if field.ClearWhenInactive && (field.When == nil || len(field.When.Selectors) == 0) {
				return fmt.Errorf("clear_when_inactive 字段 %s 必须声明 When.Selectors", path)
			}
			if field.Type == "object" {
				childPrefix := path
				if field.ObjectKind == "list" {
					childPrefix = path + "[]"
				}
				if err := walk(field.Properties, childPrefix); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(p.FormSchema, "")
}

// byteSequenceLength 是 byte-sequence 的固定长度（当前仅用于 WireGuard reserved）。
const byteSequenceLength = 3

// ParseByteSequence 导出 byte-sequence 规范化入口，供装配层输出 reserved 时复用同一语义，
// 避免在 adapter 内复制第二份解析规则。
func ParseByteSequence(value any) ([]int, error) {
	return parseByteSequence(value)
}

// parseByteSequence 把三种合法输入统一解析为 3 个 0-255 整数：
// 整数序列、以逗号/空白分隔的整数字符串、Base64 字符串（解码后必须恰 3 字节）。
func parseByteSequence(value any) ([]int, error) {
	switch typed := value.(type) {
	case nil:
		return nil, errors.New("保留字节不能为空")
	case []int:
		return byteSequenceFromInts(typed)
	case []float64:
		values := make([]int, 0, len(typed))
		for i, item := range typed {
			number, ok := byteSequenceInt(item)
			if !ok {
				return nil, fmt.Errorf("第 %d 项不是 0-255 整数", i+1)
			}
			values = append(values, number)
		}
		return byteSequenceFromInts(values)
	case []any:
		values := make([]int, 0, len(typed))
		for i, item := range typed {
			number, ok := byteSequenceInt(item)
			if !ok {
				return nil, fmt.Errorf("第 %d 项不是 0-255 整数", i+1)
			}
			values = append(values, number)
		}
		return byteSequenceFromInts(values)
	case string:
		return parseByteSequenceString(typed)
	default:
		return nil, errors.New("必须为 3 个 0-255 整数或 Base64 字符串")
	}
}

func parseByteSequenceString(raw string) ([]int, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil, errors.New("不能为空")
	}
	if strings.ContainsAny(text, ", ") {
		parts := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
		values := make([]int, 0, len(parts))
		for _, part := range parts {
			number, err := strconv.Atoi(strings.TrimSpace(part))
			if err != nil {
				return nil, errors.New("整数序列格式非法")
			}
			values = append(values, number)
		}
		return byteSequenceFromInts(values)
	}
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return nil, errors.New("既不是整数序列也不是合法 Base64")
	}
	if len(decoded) != byteSequenceLength {
		return nil, fmt.Errorf("Base64 解码后必须为 %d 字节，实际 %d", byteSequenceLength, len(decoded))
	}
	values := make([]int, 0, byteSequenceLength)
	for _, item := range decoded {
		values = append(values, int(item))
	}
	return values, nil
}

func byteSequenceFromInts(values []int) ([]int, error) {
	if len(values) != byteSequenceLength {
		return nil, fmt.Errorf("必须为 %d 个整数，实际 %d", byteSequenceLength, len(values))
	}
	out := make([]int, 0, byteSequenceLength)
	for i, value := range values {
		if value < 0 || value > 255 {
			return nil, fmt.Errorf("第 %d 项 %d 超出 0-255", i+1, value)
		}
		out = append(out, value)
	}
	return out, nil
}

func byteSequenceInt(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float32:
		if math.Trunc(float64(typed)) != float64(typed) {
			return 0, false
		}
		return int(typed), true
	case float64:
		if math.Trunc(typed) != typed {
			return 0, false
		}
		return int(typed), true
	case json.Number:
		number, err := typed.Int64()
		if err != nil {
			return 0, false
		}
		return int(number), true
	default:
		return 0, false
	}
}

// normalizeByteSequenceFields 递归把协议参数中的 byte-sequence 字段规范化为 3 整数数组。
// 规范化只作用于当前请求副本，不回写调用方传入的 map。
func normalizeByteSequenceFields(fields []FieldSchema, params map[string]any, prefix string) error {
	for _, field := range fields {
		if field.StateOnly {
			continue
		}
		path := field.Name
		if prefix != "" {
			path = prefix + "." + field.Name
		}
		value, exists := params[field.Name]
		if !exists || value == nil {
			continue
		}
		switch field.Type {
		case "byte-sequence":
			normalized, err := parseByteSequence(value)
			if err != nil {
				return fmt.Errorf("字段 %s %v", path, err)
			}
			params[field.Name] = normalized
		case "object":
			switch field.ObjectKind {
			case "fields", "map":
				object, ok := value.(map[string]any)
				if !ok {
					continue
				}
				if err := normalizeByteSequenceFields(field.Properties, object, path); err != nil {
					return err
				}
			case "list":
				items, ok := value.([]any)
				if !ok {
					continue
				}
				for i, item := range items {
					object, ok := item.(map[string]any)
					if !ok {
						continue
					}
					if err := normalizeByteSequenceFields(field.Properties, object, fmt.Sprintf("%s[%d]", path, i)); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
