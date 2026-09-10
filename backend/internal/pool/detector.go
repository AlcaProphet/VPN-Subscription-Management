// detector.go：文档级结构探测与唯一适配器选择。
package pool

import (
	"bytes"
	"net"
	"strings"

	"vpn-sub/internal/rulespec"
)

// detectionResult 是探测阶段的格式与命中依据。
type detectionResult struct {
	Format        DetectedFormat
	EvidenceCodes []string
}

// DetectOne 对整份文档进行探测，返回唯一详细格式或硬错误。
// 旧签名保留，供既有调用方使用；需要 evidence codes 时使用 detectOne。
func DetectOne(body []byte, mode SourceMode) (DetectedFormat, error) {
	result, err := detectOne(body, mode)
	return result.Format, err
}

// detectOne 在返回格式的同时返回实际命中分支产生的稳定 evidence codes。
func detectOne(body []byte, mode SourceMode) (detectionResult, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return detectionResult{}, ErrUnrecognizedSource
	}
	lower := strings.ToLower(string(trimmed))

	// 错误页/HTML 直接失败。
	if strings.HasPrefix(lower, "<!doctype html") || strings.HasPrefix(lower, "<html") || strings.Contains(lower, "<html") {
		return detectionResult{}, ErrHTMLSource
	}

	// JSON：仅识别带 version/rules 的 sing-box source。
	if trimmed[0] == '{' {
		if strings.Contains(lower, `"rules"`) && (strings.Contains(lower, `"version"`) || strings.Contains(lower, `"version":`)) {
			return detectionResult{Format: FormatSingBoxSourceJSON, EvidenceCodes: []string{"sing_box_version_and_rules"}}, nil
		}
		return detectionResult{}, ErrUnrecognizedSource
	}

	// YAML：完整读取 Mihomo payload，并按整份内容区分 domain/ipcidr/classical。
	if hasTopLevelPayloadKey(trimmed) {
		return detectMihomoPayloadFormat(trimmed)
	}

	// 显式类型文本。
	if hasTypedRuleLine(trimmed) {
		return detectionResult{Format: FormatTypedRuleText, EvidenceCodes: []string{"typed_rule_marker"}}, nil
	}

	// 纯 IP/CIDR/ASN。
	if isIPOrASNList(trimmed) {
		return detectionResult{Format: FormatPlainIPCIDRText, EvidenceCodes: []string{"all_items_ip_cidr_or_asn"}}, nil
	}

	// legacy/plain 域名文本。
	if hasDomainLines(trimmed) {
		if bytes.Contains(trimmed, []byte("full:")) || bytes.Contains(trimmed, []byte("+.")) {
			return detectionResult{Format: FormatLegacyDomainText, EvidenceCodes: []string{"legacy_domain_prefix"}}, nil
		}
		return detectionResult{Format: FormatPlainDomainText, EvidenceCodes: []string{"plain_domain_candidates"}}, nil
	}

	return detectionResult{}, ErrUnrecognizedSource
}

func hasTopLevelPayloadKey(body []byte) bool {
	for _, raw := range strings.Split(string(body), "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		if strings.TrimSpace(raw) == "" || strings.HasPrefix(strings.TrimSpace(raw), "#") {
			continue
		}
		if len(raw) != len(strings.TrimLeft(raw, " \t")) {
			continue
		}
		colon := strings.IndexByte(raw, ':')
		if colon >= 0 && strings.EqualFold(strings.TrimSpace(raw[:colon]), "payload") {
			return true
		}
	}
	return false
}

func detectMihomoPayloadFormat(body []byte) (detectionResult, error) {
	items, err := decodeMihomoPayload(body)
	if err != nil {
		return detectionResult{}, err
	}
	var detected DetectedFormat
	evidence := ""
	for _, item := range items {
		format := FormatMihomoDomainYAML
		code := "payload_domain_only"
		if strings.Contains(item, ",") {
			format = FormatMihomoClassicalYAML
			code = "payload_classical_only"
		} else if _, err := NormalizeCIDRValue(item); err == nil {
			format = FormatMihomoIPCIDRYAML
			code = "payload_ipcidr_only"
		}
		if detected != "" && detected != format {
			return detectionResult{}, ErrConflictingDocumentFormat
		}
		detected = format
		if evidence == "" {
			evidence = code
		}
	}
	if detected == "" {
		return detectionResult{}, ErrUnrecognizedSource
	}
	return detectionResult{
		Format:        detected,
		EvidenceCodes: []string{"top_level_payload", evidence},
	}, nil
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
