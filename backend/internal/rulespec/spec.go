// Package rulespec 提供 mihomo 规则类型、取值和输出语义的共享元数据。
package rulespec

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// RuleDef 是旧规则类型的兼容投影；新代码应优先使用 CanonicalRule/Capability。
type RuleDef struct {
	SR                bool
	NoResolve         bool // 兼容投影：旧渲染默认追加 no-resolve
	SupportsNoResolve bool // 能力：该类型是否允许实例级 no_resolve
	ValueRequired     bool
	ValueLabel        string
}

// Definitions 是 CVR 规则编辑器全集，并保留本项目的 USER-AGENT。
var Definitions = map[string]RuleDef{
	"DOMAIN": {SR: true, ValueRequired: true}, "DOMAIN-SUFFIX": {SR: true, ValueRequired: true}, "DOMAIN-KEYWORD": {SR: true, ValueRequired: true},
	"DOMAIN-REGEX": {ValueRequired: true}, "GEOSITE": {ValueRequired: true}, "GEOIP": {SR: true, NoResolve: true, ValueRequired: true},
	"SRC-GEOIP": {ValueRequired: true}, "IP-ASN": {NoResolve: true, ValueRequired: true}, "SRC-IP-ASN": {ValueRequired: true},
	"IP-CIDR": {SR: true, NoResolve: true, ValueRequired: true}, "IP-CIDR6": {SR: true, NoResolve: true, ValueRequired: true}, "SRC-IP-CIDR": {ValueRequired: true},
	"IP-SUFFIX": {NoResolve: true, ValueRequired: true}, "SRC-IP-SUFFIX": {ValueRequired: true},
	"SRC-PORT": {ValueRequired: true}, "DST-PORT": {ValueRequired: true}, "IN-PORT": {ValueRequired: true}, "DSCP": {ValueRequired: true},
	"PROCESS-NAME": {SR: true, ValueRequired: true}, "PROCESS-PATH": {ValueRequired: true}, "PROCESS-NAME-REGEX": {SR: true, ValueRequired: true}, "PROCESS-PATH-REGEX": {ValueRequired: true},
	"NETWORK": {ValueRequired: true}, "UID": {ValueRequired: true}, "IN-TYPE": {ValueRequired: true}, "IN-USER": {ValueRequired: true}, "IN-NAME": {ValueRequired: true},
	"SUB-RULE": {ValueRequired: true}, "RULE-SET": {NoResolve: true, ValueRequired: true}, "AND": {ValueRequired: true}, "OR": {ValueRequired: true}, "NOT": {ValueRequired: true},
	"MATCH": {ValueRequired: false}, "USER-AGENT": {SR: true, ValueRequired: true},
}

// ValidateValue 校验并规范化规则取值。
func ValidateValue(ruleType, value string) (string, string, error) {
	typ := strings.ToUpper(strings.TrimSpace(ruleType))
	def, ok := Definitions[typ]
	if !ok {
		return "", "", fmt.Errorf("不支持的规则类型: %s", typ)
	}
	value = strings.TrimSpace(value)
	if !def.ValueRequired {
		if value != "" {
			return "", "", fmt.Errorf("%s 不接受匹配值", typ)
		}
		return typ, "", nil
	}
	if value == "" {
		return "", "", fmt.Errorf("%s 匹配值不能为空", typ)
	}
	if strings.ContainsAny(value, "\r\n") || containsControl(value) {
		return "", "", fmt.Errorf("%s 匹配值含非法字符", typ)
	}
	if typ == "AND" || typ == "OR" || typ == "NOT" {
		if !strings.HasPrefix(value, "(") || balancedCloseParen(value) != len(value)-1 {
			return "", "", fmt.Errorf("%s 逻辑表达式括号不配对", typ)
		}
		return typ, value, nil
	}
	if strings.Contains(value, ",") {
		return "", "", fmt.Errorf("%s 匹配值含非法逗号", typ)
	}
	switch typ {
	case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD":
		value = strings.ToLower(value)
		if strings.ContainsAny(value, "/: ") {
			return "", "", fmt.Errorf("%s 匹配值不是合法域名", typ)
		}
	case "DOMAIN-REGEX", "PROCESS-NAME-REGEX", "PROCESS-PATH-REGEX":
		if _, err := regexp.Compile(value); err != nil {
			return "", "", fmt.Errorf("%s 正则无效: %v", typ, err)
		}
	case "IP-CIDR", "IP-CIDR6", "SRC-IP-CIDR", "IP-SUFFIX", "SRC-IP-SUFFIX":
		_, network, err := net.ParseCIDR(value)
		if err != nil {
			return "", "", fmt.Errorf("%s 匹配值不是合法 CIDR: %v", typ, err)
		}
		value = network.String()
	case "IP-ASN", "SRC-IP-ASN", "UID":
		if number, err := strconv.ParseUint(value, 10, 64); err != nil || number == 0 {
			return "", "", fmt.Errorf("%s 匹配值必须是正整数", typ)
		}
	case "SRC-PORT", "DST-PORT", "IN-PORT":
		port, err := strconv.Atoi(value)
		if err != nil || port < 1 || port > 65535 {
			return "", "", fmt.Errorf("%s 端口必须在 1-65535", typ)
		}
	case "DSCP":
		dscp, err := strconv.Atoi(value)
		if err != nil || dscp < 0 || dscp > 63 {
			return "", "", fmt.Errorf("DSCP 必须在 0-63")
		}
	case "NETWORK":
		value = strings.ToLower(value)
		if value != "tcp" && value != "udp" {
			return "", "", fmt.Errorf("NETWORK 仅支持 tcp/udp")
		}
	}
	if len(value) > 4096 {
		return "", "", fmt.Errorf("%s 匹配值过长", typ)
	}
	return typ, value, nil
}

// ParseRendered 解析已渲染的 Clash 规则行，逻辑表达式中的逗号不会被截断。
func ParseRendered(line string) (typ, value, target string, noResolve bool, err error) {
	comma := strings.IndexByte(line, ',')
	if comma < 1 {
		return "", "", "", false, fmt.Errorf("规则缺少分隔符")
	}
	typ = strings.ToUpper(strings.TrimSpace(line[:comma]))
	rest := strings.TrimSpace(line[comma+1:])
	if typ == "MATCH" {
		if rest == "" || strings.Contains(rest, ",") {
			return "", "", "", false, fmt.Errorf("MATCH 目标无效")
		}
		return typ, "", rest, false, nil
	}
	if typ == "AND" || typ == "OR" || typ == "NOT" {
		end := balancedCloseParen(rest)
		if end < 0 {
			return "", "", "", false, fmt.Errorf("逻辑规则括号不配对")
		}
		value = strings.TrimSpace(rest[:end+1])
		rest = strings.TrimSpace(rest[end+1:])
		if !strings.HasPrefix(rest, ",") {
			return "", "", "", false, fmt.Errorf("逻辑规则缺少目标")
		}
		rest = strings.TrimSpace(rest[1:])
	} else {
		parts := strings.Split(rest, ",")
		if len(parts) < 2 || len(parts) > 3 {
			return "", "", "", false, fmt.Errorf("规则字段数量无效")
		}
		value = strings.TrimSpace(parts[0])
		rest = strings.Join(parts[1:], ",")
	}
	parts := strings.Split(rest, ",")
	if len(parts) == 2 && strings.TrimSpace(parts[1]) == "no-resolve" {
		noResolve = true
		parts = parts[:1]
	}
	if len(parts) != 1 || strings.TrimSpace(parts[0]) == "" {
		return "", "", "", false, fmt.Errorf("规则目标无效")
	}
	target = strings.TrimSpace(parts[0])
	return typ, value, target, noResolve, nil
}

// BalancedCloseParen 返回从首字符开始配平的右括号位置。
func BalancedCloseParen(value string) int { return balancedCloseParen(value) }

func balancedCloseParen(value string) int {
	depth := 0
	for i, r := range value {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
			if depth < 0 {
				return -1
			}
		}
	}
	return -1
}

func containsControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
