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
	case '&', ';', ' ', '\t', '\n', '\r', ',', '"', '\'', ')', ']', '}', '>':
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

// RedactText 通用文本脱敏：覆盖 URL query/fragment/userinfo 和普通 key=value 文本中的疑似凭据。
func RedactText(s string) string {
	if s == "" {
		return ""
	}
	s = redactURLUserinfos(s)
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		eq := strings.IndexByte(s[i:], '=')
		if eq < 0 {
			b.WriteString(s[i:])
			break
		}
		abs := i + eq
		keyEnd := abs
		keyStart := keyEnd
		for keyStart > i && isKeyChar(s[keyStart-1]) {
			keyStart--
		}
		// key 前必须是赋值上下文分隔符或文本起点，避免把普通文本中单词后的 = 误判。
		if keyStart > 0 && !isAssignmentSeparator(s[keyStart-1]) {
			b.WriteString(s[i : abs+1])
			i = abs + 1
			continue
		}
		key := s[keyStart:keyEnd]
		if isSensitiveKey(key) {
			valStart := abs + 1
			valEnd := valStart
			for valEnd < len(s) && !isValueEnd(s[valEnd]) {
				valEnd++
			}
			b.WriteString(s[i:keyEnd])
			b.WriteString("=***")
			i = valEnd
		} else {
			b.WriteString(s[i : abs+1])
			i = abs + 1
		}
	}
	return b.String()
}

// redactRawQuery redacts sensitive assignments in a raw query/fragment string while
// preserving parameter order, duplicate parameters and original encoding.
func redactRawQuery(raw string) string {
	if raw == "" {
		return raw
	}
	parts := strings.Split(raw, "&")
	for i, part := range parts {
		eq := strings.IndexByte(part, '=')
		if eq < 0 {
			continue
		}
		key := part[:eq]
		if isSensitiveKey(key) {
			parts[i] = key + "=***"
		}
	}
	return strings.Join(parts, "&")
}

// RedactDisplayURL 只用于展示 URL：保留非敏感参数、参数顺序、重复参数和原始编码，
// 仅把敏感 query/fragment 参数值替换为 ***，并隐藏 userinfo 密码。
func RedactDisplayURL(raw string) string {
	raw = redactURLUserinfos(raw)
	q := strings.IndexByte(raw, '?')
	h := strings.IndexByte(raw, '#')
	if q >= 0 && (h < 0 || q < h) {
		queryEnd := len(raw)
		if h >= 0 {
			queryEnd = h
		}
		var b strings.Builder
		b.WriteString(raw[:q+1])
		b.WriteString(redactRawQuery(raw[q+1 : queryEnd]))
		if h >= 0 {
			b.WriteString("#")
			b.WriteString(RedactText(raw[h+1:]))
		}
		return b.String()
	}
	if h >= 0 {
		return raw[:h+1] + RedactText(raw[h+1:])
	}
	return raw
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
