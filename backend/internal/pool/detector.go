// detector.go：文档级结构探测与唯一适配器选择。
package pool

import (
	"bytes"
	"net"
	"strings"

	"vpn-sub/internal/rulespec"
)

// detectionCandidate 是探测阶段的候选结果。
type detectionCandidate struct {
	format DetectedFormat
	score  int
	reason string
}

// DetectOne 对整份文档进行探测，返回唯一详细格式或硬错误。
func DetectOne(body []byte, mode SourceMode) (DetectedFormat, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", ErrUnrecognizedSource
	}
	lower := strings.ToLower(string(trimmed))

	// 错误页/HTML 直接失败。
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") || strings.Contains(lower, "<html") {
		return "", ErrHTMLSource
	}

	// JSON：仅识别带 version/rules 的 sing-box source。
	if trimmed[0] == '{' {
		if strings.Contains(lower, `"rules"`) && (strings.Contains(lower, `"version"`) || strings.Contains(lower, `"version":`)) {
			return FormatSingBoxSourceJSON, nil
		}
		return "", ErrUnrecognizedSource
	}

	// YAML：识别 Mihomo provider 形态。按 payload 内容区分 domain/classical。
	if looksLikeYAMLPayload(trimmed) {
		if yamlHasClassicalPayload(trimmed) {
			return FormatMihomoClassicalYAML, nil
		}
		return FormatMihomoDomainYAML, nil
	}

	// 显式类型文本。
	if hasTypedRuleLine(trimmed) {
		return FormatTypedRuleText, nil
	}

	// 纯 IP/CIDR/ASN。
	if isIPOrASNList(trimmed) {
		return FormatPlainIPCIDRText, nil
	}

	// legacy/plain 域名文本。
	if hasDomainLines(trimmed) {
		if bytes.Contains(trimmed, []byte("full:")) || bytes.Contains(trimmed, []byte("+.")) {
			return FormatLegacyDomainText, nil
		}
		return FormatPlainDomainText, nil
	}

	return "", ErrUnrecognizedSource
}

func looksLikeYAMLPayload(body []byte) bool {
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "payload:") || strings.Contains(lower, "\npayload")
}

func yamlHasClassicalPayload(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "payload") || strings.HasPrefix(line, "name") || line == ":" {
			continue
		}
		if strings.Contains(line, ",") {
			return true
		}
	}
	return false
}

func hasTypedRuleLine(body []byte) bool {
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ",") {
			typ := strings.ToUpper(strings.TrimSpace(line[:strings.IndexByte(line, ',')]))
			if rulespecCanonicalType(typ) {
				return true
			}
		}
	}
	return false
}

func isIPOrASNList(body []byte) bool {
	lines := nonEmptyLines(body)
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !isIPLine(trimmed) && !isASNLine(trimmed) {
			return false
		}
	}
	return true
}

func isIPLine(line string) bool {
	if strings.Contains(line, "/") {
		_, _, err := net.ParseCIDR(line)
		return err == nil
	}
	return net.ParseIP(line) != nil
}

func isASNLine(line string) bool {
	upper := strings.ToUpper(line)
	if strings.HasPrefix(upper, "AS") {
		line = strings.TrimSpace(line[2:])
	}
	for _, r := range line {
		if r < '0' || r > '9' {
			return false
		}
	}
	return line != ""
}

func hasDomainLines(body []byte) bool {
	lines := nonEmptyLines(body)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "full:") || strings.HasPrefix(line, "+.") || strings.HasPrefix(line, ".") {
			return true
		}
		if strings.Contains(line, ".") && !strings.Contains(line, ",") && !strings.Contains(line, ":") {
			return true
		}
	}
	return false
}

func nonEmptyLines(body []byte) []string {
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

func rulespecCanonicalType(typ string) bool {
	_, _, ok := rulespec.CanonicalizeLegacyType(typ)
	return ok
}
