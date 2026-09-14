// parser.go：URL 内容逐行解析与条目元数据校验。
package pool

import (
	"strings"

	"vpn-sub/internal/rulespec"
)

// ParseLine 解析 full/裸域名/标准规则/逻辑规则；URL 内的 policy 只记录提示，不入库。
func ParseLine(raw string) (string, string, string, bool) {
	typ, value, _, reason, ok := parseRuleLine(raw)
	return typ, value, reason, ok
}

// parsedRuleLine 是结构化解析结果：
// SourcePolicy 为 value 后第一个非 no-resolve token，仅用于兼容旧提示；
// UnknownTokens 为其后再次出现的非 no-resolve token，需要生成 warn 诊断。
type parsedRuleLine struct {
	Type          string
	Value         string
	NoResolve     bool
	SourcePolicy  string
	UnknownTokens []string
	Reason        string
	OK            bool
}

// parseRuleLineDetailed 按位置法解析标准规则尾部：
// 1) 任意位置的大小写不敏感 no-resolve 独立 token 设置为实例选项；
// 2) value 后第一个非 no-resolve token 视为 source policy，静默忽略；
// 3) 之后再次出现的非 no-resolve token 视为未知 option。
func parseRuleLineDetailed(raw string) parsedRuleLine {
	line := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return parsedRuleLine{Reason: "空行或注释"}
	}
	if strings.HasPrefix(line, "full:") {
		value := strings.TrimSpace(strings.TrimPrefix(line, "full:"))
		if value == "" {
			return parsedRuleLine{Reason: "full: 前缀缺少域名"}
		}
		return parsedRuleLine{Type: "DOMAIN", Value: value, OK: true}
	}
	if !strings.Contains(line, ",") {
		return parsedRuleLine{Type: "DOMAIN-SUFFIX", Value: line, OK: true}
	}
	comma := strings.IndexByte(line, ',')
	typ := strings.ToUpper(strings.TrimSpace(line[:comma]))
	rest := strings.TrimSpace(line[comma+1:])
	if typ == "AND" || typ == "OR" || typ == "NOT" {
		if !strings.HasPrefix(rest, "(") {
			return parsedRuleLine{Reason: "逻辑规则表达式必须以 ( 开头"}
		}
		end := rulespec.BalancedCloseParen(rest)
		if end < 0 {
			return parsedRuleLine{Reason: "逻辑规则括号不配对"}
		}
		expr := strings.TrimSpace(rest[:end+1])
		remainder := strings.TrimSpace(rest[end+1:])
		if remainder == "" {
			return parsedRuleLine{Type: typ, Value: expr, OK: true}
		}
		if !strings.HasPrefix(remainder, ",") || strings.TrimSpace(remainder[1:]) == "" {
			return parsedRuleLine{Reason: "逻辑规则表达式后存在无法识别的尾部"}
		}
		return parsedRuleLine{Type: typ, Value: expr, Reason: "逻辑规则末尾 policy 已忽略（目标由装配层指定）", OK: true}
	}
	if typ == "MATCH" {
		reason := ""
		if rest != "" {
			reason = "MATCH 行内 policy 已忽略（目标由装配层指定）"
		}
		return parsedRuleLine{Type: typ, Value: "", Reason: reason, OK: true}
	}
	parts := strings.SplitN(rest, ",", 2)
	value := strings.TrimSpace(parts[0])
	if typ == "" || value == "" {
		return parsedRuleLine{Reason: "规则类型或匹配值为空"}
	}
	out := parsedRuleLine{Type: typ, Value: value, OK: true}
	if len(parts) > 1 && strings.TrimSpace(parts[1]) != "" {
		seenPolicy := false
		for _, tok := range strings.Split(parts[1], ",") {
			tok = strings.TrimSpace(tok)
			if tok == "" {
				continue
			}
			if strings.EqualFold(tok, "no-resolve") {
				out.NoResolve = true
				continue
			}
			if !seenPolicy {
				out.SourcePolicy = tok
				seenPolicy = true
				continue
			}
			out.UnknownTokens = append(out.UnknownTokens, tok)
		}
	}
	return out
}

// parseRuleLine 保留既有签名与 ParseLine 的提示语义，供兼容调用方使用。
// URL 源适配器应使用 parseRuleLineDetailed。
func parseRuleLine(raw string) (typ, value string, noResolve bool, reason string, ok bool) {
	d := parseRuleLineDetailed(raw)
	if !d.OK {
		return "", "", d.NoResolve, d.Reason, false
	}
	if d.SourcePolicy != "" || len(d.UnknownTokens) > 0 {
		reason = "行内 policy/未知 option 已忽略（目标由装配层指定）"
	} else {
		reason = d.Reason
	}
	return d.Type, d.Value, d.NoResolve, reason, true
}

// ValidateEntry 调用共享规则元数据校验并返回规范化结果。
func ValidateEntry(ruleType, matchValue string) (string, string, error) {
	return rulespec.ValidateValue(ruleType, matchValue)
}
