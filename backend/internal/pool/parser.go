// parser.go：URL 内容逐行解析与条目白名单校验（Design2 §2.3）
package pool

import (
	"fmt"
	"net"
	"strings"
	"unicode/utf8"
)

// supportedTypes 规则类型白名单（Design2 §2.3/§3.5）
var supportedTypes = map[string]bool{
	"DOMAIN":             true,
	"DOMAIN-SUFFIX":      true,
	"DOMAIN-KEYWORD":     true,
	"IP-CIDR":            true,
	"IP-CIDR6":           true,
	"PROCESS-NAME":       true,
	"PROCESS-NAME-REGEX": true,
	"USER-AGENT":         true,
}

// ParseLine 解析单行：full:<域名> → DOMAIN；裸域名 → DOMAIN-SUFFIX；
// 标准规则行 `TYPE,VALUE`（逗号多于两段仅取前两段）；注释/空行跳过。
// 返回 (规则类型, 匹配值, skip 原因, 是否有效)。
func ParseLine(raw string) (string, string, string, bool) {
	line := strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(raw, "\n"), "\r"))
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", "空行或注释", false
	}
	if strings.HasPrefix(line, "full:") {
		v := strings.TrimSpace(strings.TrimPrefix(line, "full:"))
		if v == "" {
			return "", "", "full: 前缀缺少域名", false
		}
		return "DOMAIN", v, "", true
	}
	if !strings.Contains(line, ",") {
		return "DOMAIN-SUFFIX", line, "", true // 裸域名 → DOMAIN-SUFFIX
	}
	parts := strings.SplitN(line, ",", 3)
	typ := strings.TrimSpace(parts[0])
	val := strings.TrimSpace(parts[1])
	if typ == "" || val == "" {
		return "", "", "规则类型或匹配值为空", false
	}
	skip := ""
	if len(parts) > 2 && strings.TrimSpace(parts[2]) != "" {
		skip = "逗号多于两段，多余段已忽略"
	}
	return typ, val, skip, true
}

// ValidateEntry 入库白名单校验（域名 lowercase；CIDR 归一；控制字符/逗号/换行拒绝防拼接注入）
func ValidateEntry(ruleType, matchValue string) (string, error) {
	typ := strings.ToUpper(strings.TrimSpace(ruleType))
	val := strings.TrimSpace(matchValue)
	if !supportedTypes[typ] {
		return "", fmt.Errorf("不支持的规则类型: %s", typ)
	}
	if val == "" || val != strings.TrimSpace(val) {
		return "", fmt.Errorf("%s 匹配值不能为空且禁止首尾空白", typ)
	}
	if containsControl(val) || strings.ContainsAny(val, ",\r\n") {
		return "", fmt.Errorf("%s 匹配值含非法字符", typ)
	}
	switch typ {
	case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD":
		v := strings.ToLower(val) // 域名统一 lowercase 规范化
		if strings.Contains(v, "/") || strings.Contains(v, ":") || strings.Contains(v, " ") {
			return "", fmt.Errorf("%s 匹配值不是合法域名", typ)
		}
		if !utf8.ValidString(v) {
			return "", fmt.Errorf("%s 匹配值含非法 UTF-8", typ)
		}
		return v, nil
	case "IP-CIDR", "IP-CIDR6":
		_, ipNet, err := net.ParseCIDR(val)
		if err != nil {
			return "", fmt.Errorf("%s 匹配值不是合法 CIDR: %v", typ, err)
		}
		return ipNet.String(), nil // CIDR 按规范格式归一
	case "PROCESS-NAME", "PROCESS-NAME-REGEX", "USER-AGENT":
		if len(val) > 512 {
			return "", fmt.Errorf("%s 匹配值超过 512 字符", typ)
		}
		return val, nil
	default:
		return "", fmt.Errorf("不支持的规则类型: %s", typ)
	}
}
