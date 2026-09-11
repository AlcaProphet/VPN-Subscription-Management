// Package redact 提供日志与素材池共用的文本/URL 脱敏和 Unicode 限长。
package redact

import (
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	MaxDiagnostics = 20
	MaxFieldRunes  = 200
)

var sensitiveKeys = map[string]bool{
	"token":          true,
	"code":           true,
	"state":          true,
	"password":       true,
	"passwd":         true,
	"secret":         true,
	"client_secret":  true,
	"private-key":    true,
	"private_key":    true,
	"pre-shared-key": true,
	"pre_shared_key": true,
	"psk":            true,
	"auth":           true,
	"auth-key":       true,
	"auth_key":       true,
	"access_token":   true,
	"refresh_token":  true,
	"api_key":        true,
	"apikey":         true,
}

func isSensitiveKey(name string) bool {
	name = strings.TrimSpace(name)
	name = strings.Trim(name, `"'`)
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	return sensitiveKeys[strings.ToLower(name)]
}

func isKeyChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '_' || c == '-' || c == '.' || c == '%'
}

func isAssignmentSeparator(c byte) bool {
	switch c {
	case '?', '&', ';', '#', '@', ' ', '\t', '\n', '\r', ',', '"', '\'', '(', '[', '{', '<':
		return true
	}
	return false
}

func isValueEnd(c byte) bool {
	switch c {
	case '&', ';', ' ', '\t', '\n', '\r', ',', '"', '\'', ')', ']', '}', '>', '#':
		return true
	}
	return false
}

// redactURLUserinfos 将 URL userinfo 中的密码（冒号后内容）替换为 ***。
func redactURLUserinfos(s string) string {
	search := 0
	for {
		scheme := strings.Index(s[search:], "://")
		if scheme < 0 {
			return s
		}
		scheme += search
		after := scheme + 3
		at := strings.IndexByte(s[after:], '@')
		if at < 0 {
			return s
		}
		at += after
		if strings.IndexAny(s[after:at], "/?#") >= 0 {
			// @ 属于后续路径而非 userinfo，继续向后寻找。
			search = at + 1
			continue
		}
		colon := strings.IndexByte(s[after:at], ':')
		if colon >= 0 {
			pos := after + colon
			s = s[:pos+1] + "***" + s[at:]
			search = pos + 4
		} else {
			search = at + 1
		}
	}
}

// encodedSeparatorAt 判断 i 起是否为编码后的 query/赋值边界：
// %26(&)、%23(#)、%3b(;)、%3f(?)。
func encodedSeparatorAt(s string, i int) bool {
	if i < 0 || i+3 > len(s) || s[i] != '%' {
		return false
	}
	switch s[i+1] {
	case '2':
		return s[i+2] == '6' || s[i+2] == '3'
	case '3':
		return s[i+2] == 'b' || s[i+2] == 'B' ||
			s[i+2] == 'f' || s[i+2] == 'F' ||
			s[i+2] == 'd' || s[i+2] == 'D'
	}
	return false
}

// findAssignmentSep 查找普通 = 或编码 %3d/%3D，返回位置与分隔符字节长度。
func findAssignmentSep(s string, from int) (int, int) {
	for i := from; i < len(s); i++ {
		if s[i] == '=' {
			return i, 1
		}
		if i+3 <= len(s) && s[i] == '%' && s[i+1] == '3' &&
			(s[i+2] == 'd' || s[i+2] == 'D') {
			return i, 3
		}
	}
	return -1, 0
}

// keyStartBefore 从赋值分隔符向前回退 key；编码边界（如 %26、%3f）视为 key 起点。
func keyStartBefore(s string, eq int) int {
	i := eq
	for i > 0 {
		if encodedSeparatorAt(s, i-3) {
			break
		}
		if !isKeyChar(s[i-1]) {
			break
		}
		i--
	}
	return i
}

// assignmentBoundaryBefore 判断 key 前是否是合法赋值上下文边界。
func assignmentBoundaryBefore(s string, keyStart int) bool {
	if keyStart == 0 {
		return true
	}
	if encodedSeparatorAt(s, keyStart-3) {
		return true
	}
	return isAssignmentSeparator(s[keyStart-1])
}

// valueEndAt 判断 value 扫描是否应在此结束；编码分隔符与普通分隔符同等处理。
func valueEndAt(s string, i int) bool {
	if encodedSeparatorAt(s, i) {
		return true
	}
	return isValueEnd(s[i])
}

// RedactText 通用文本脱敏：覆盖 URL query/fragment/userinfo 和普通 key=value 文本中的疑似凭据。
// 同时识别 %26/%23/%3b/%3f 等编码分隔符和 %3d 编码赋值符，保留原始编码。
func RedactText(s string) string {
	if s == "" {
		return ""
	}
	s = redactURLUserinfos(s)
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		eq, sepLen := findAssignmentSep(s, i)
		if eq < 0 {
			b.WriteString(s[i:])
			break
		}
		keyStart := keyStartBefore(s, eq)
		// key 前必须是赋值上下文分隔符或文本起点，避免把普通文本中单词后的 = 误判。
		if !assignmentBoundaryBefore(s, keyStart) {
			b.WriteString(s[i : eq+sepLen])
			i = eq + sepLen
			continue
		}
		key := s[keyStart:eq]
		if isSensitiveKey(key) {
			valStart := eq + sepLen
			valEnd := valStart
			for valEnd < len(s) && !valueEndAt(s, valEnd) {
				valEnd++
			}
			b.WriteString(s[i:eq])
			b.WriteString(s[eq : eq+sepLen])
			b.WriteString("***")
			i = valEnd
		} else {
			b.WriteString(s[i : eq+sepLen])
			i = eq + sepLen
		}
	}
	return b.String()
}

// RedactDisplayURL 只用于展示 URL：保留非敏感参数、参数顺序、重复参数和原始编码，
// 仅把敏感 query/fragment 参数值替换为 ***，并隐藏 userinfo 密码。
// 与通用文本脱敏共用同一套解析，避免 ;、嵌套 URL 或编码分隔符绕过。
func RedactDisplayURL(raw string) string {
	return RedactText(redactURLUserinfos(raw))
}

// TruncateText 按 rune 安全截断字符串。
func TruncateText(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes])
}
