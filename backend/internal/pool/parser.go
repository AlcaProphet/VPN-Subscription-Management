// parser.go：URL 内容逐行解析与条目元数据校验。
package pool

import (
	"strings"

	"vpn-sub/internal/rulespec"
)

// ParseLine 解析 full/裸域名/标准规则/逻辑规则；URL 内的 policy 只记录提示，不入库。
func ParseLine(raw string) (string, string, string, bool) {
	line := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", "空行或注释", false
	}
	if strings.HasPrefix(line, "full:") {
		value := strings.TrimSpace(strings.TrimPrefix(line, "full:"))
		if value == "" {
			return "", "", "full: 前缀缺少域名", false
		}
		return "DOMAIN", value, "", true
	}
	if !strings.Contains(line, ",") {
		return "DOMAIN-SUFFIX", line, "", true
	}
	comma := strings.IndexByte(line, ',')
	typ := strings.ToUpper(strings.TrimSpace(line[:comma]))
	rest := strings.TrimSpace(line[comma+1:])
	if typ == "AND" || typ == "OR" || typ == "NOT" {
		if !strings.HasPrefix(rest, "(") {
			return "", "", "逻辑规则表达式必须以 ( 开头", false
		}
		end := rulespec.BalancedCloseParen(rest)
		if end < 0 {
			return "", "", "逻辑规则括号不配对", false
		}
		expr := strings.TrimSpace(rest[:end+1])
		remainder := strings.TrimSpace(rest[end+1:])
		if remainder == "" {
			return typ, expr, "", true
		}
		if !strings.HasPrefix(remainder, ",") || strings.TrimSpace(remainder[1:]) == "" {
			return "", "", "逻辑规则表达式后存在无法识别的尾部", false
		}
		return typ, expr, "逻辑规则末尾 policy 已忽略（目标由装配层指定）", true
	}
	if typ == "MATCH" {
		reason := ""
		if rest != "" {
			reason = "MATCH 行内 policy 已忽略（目标由装配层指定）"
		}
		return typ, "", reason, true
	}
	parts := strings.SplitN(rest, ",", 2)
	value := strings.TrimSpace(parts[0])
	if typ == "" || value == "" {
		return "", "", "规则类型或匹配值为空", false
	}
	reason := ""
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		reason = "行内 policy 已忽略（目标由装配层指定）"
	}
	return typ, value, reason, true
}

// ValidateEntry 调用共享规则元数据校验并返回规范化结果。
func ValidateEntry(ruleType, matchValue string) (string, string, error) {
	return rulespec.ValidateValue(ruleType, matchValue)
}
