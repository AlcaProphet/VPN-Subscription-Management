package ssplugin

import (
	"fmt"
	"sort"
	"strings"
)

// ParsePluginString 解析 SIP002 plugin 查询值，并保留 bare flag 为空字符串。
func ParsePluginString(raw string) (string, map[string]string, error) {
	parts, err := splitEscaped(raw, ';')
	if err != nil {
		return "", nil, err
	}
	if len(parts) == 0 {
		return "", nil, fmt.Errorf("SIP002 插件名为空")
	}
	name, err := unescape(parts[0])
	if err != nil {
		return "", nil, err
	}
	if name == "" {
		return "", nil, fmt.Errorf("SIP002 插件名为空")
	}
	opts := make(map[string]string, len(parts)-1)
	for _, part := range parts[1:] {
		keyRaw, valueRaw := part, ""
		if idx, findErr := indexUnescaped(part, '='); findErr != nil {
			return "", nil, findErr
		} else if idx >= 0 {
			keyRaw, valueRaw = part[:idx], part[idx+1:]
		}
		key, err := unescape(keyRaw)
		if err != nil {
			return "", nil, err
		}
		if key == "" {
			return "", nil, fmt.Errorf("SIP002 插件参数键为空")
		}
		if _, exists := opts[key]; exists {
			return "", nil, fmt.Errorf("SIP002 插件参数键重复: %s", key)
		}
		value, err := unescape(valueRaw)
		if err != nil {
			return "", nil, err
		}
		opts[key] = value
	}
	return name, opts, nil
}

// SerializePluginString 生成稳定排序且可逐字符反向解析的 SIP002 plugin 查询值。
func SerializePluginString(name string, opts map[string]string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("SIP002 插件名为空")
	}
	keys := make([]string, 0, len(opts))
	for key := range opts {
		if key == "" {
			return "", fmt.Errorf("SIP002 插件参数键为空")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 1, len(keys)+1)
	parts[0] = escape(name)
	for _, key := range keys {
		part := escape(key)
		if value := opts[key]; value != "" {
			part += "=" + escape(value)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ";"), nil
}

func escape(value string) string {
	var out strings.Builder
	for _, r := range value {
		if isEscapedRune(r) {
			out.WriteRune('\\')
		}
		out.WriteRune(r)
	}
	return out.String()
}

func unescape(value string) (string, error) {
	var out strings.Builder
	escaped := false
	for _, r := range value {
		if escaped {
			if !isEscapedRune(r) {
				return "", fmt.Errorf("SIP002 插件字符串包含无效转义: \\%c", r)
			}
			out.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		out.WriteRune(r)
	}
	if escaped {
		return "", fmt.Errorf("SIP002 插件字符串以孤立反斜杠结尾")
	}
	return out.String(), nil
}

func splitEscaped(value string, separator rune) ([]string, error) {
	parts := make([]string, 0, 4)
	start := 0
	escaped := false
	for idx, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == separator {
			parts = append(parts, value[start:idx])
			start = idx + len(string(r))
		}
	}
	if escaped {
		return nil, fmt.Errorf("SIP002 插件字符串以孤立反斜杠结尾")
	}
	return append(parts, value[start:]), nil
}

func indexUnescaped(value string, separator rune) (int, error) {
	escaped := false
	for idx, r := range value {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == separator {
			return idx, nil
		}
	}
	if escaped {
		return -1, fmt.Errorf("SIP002 插件字符串以孤立反斜杠结尾")
	}
	return -1, nil
}

func isEscapedRune(r rune) bool {
	return r == ':' || r == ';' || r == '=' || r == '\\'
}
