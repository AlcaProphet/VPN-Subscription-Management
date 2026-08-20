package assembly

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

// srLink 生成 Shadowrocket 原生参数风格节点链接。
func srLink(nd *nodeData) (string, error) {
	host, err := punycode(nd.Host)
	if err != nil {
		return "", err
	}
	name := url.QueryEscape(nd.RenderName)
	switch nd.Protocol {
	case "ss":
		cipher := str(nd.ProtocolJSON, "cipher", "auto")
		password := str(nd.ProtocolJSON, "password", "")
		userinfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
		return fmt.Sprintf("ss://%s@%s:%d#%s", userinfo, host, nd.Port, name), nil
	case "vmess":
		cipher := str(nd.ProtocolJSON, "cipher", "auto")
		uuid := str(nd.ProtocolJSON, "uuid", "")
		userinfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + uuid + "@" + host + ":" + strconv.Itoa(nd.Port)))
		q := url.Values{}
		q.Set("remarks", nd.RenderName)
		q.Set("udp", "1")
		q.Set("alterId", "0")
		return "vmess://" + userinfo + "?" + encodeQuery(q), nil
	case "vless":
		uuid := str(nd.ProtocolJSON, "uuid", "")
		userinfo := base64.StdEncoding.EncodeToString([]byte(":" + uuid + "@" + host + ":" + strconv.Itoa(nd.Port)))
		q := url.Values{}
		q.Set("remarks", nd.RenderName)
		q.Set("udp", "1")
		reality := realityOpts(nd.ProtocolJSON)
		if reality != nil {
			q.Set("tls", "1")
			q.Set("xtls", "2")
			if sni := firstNonEmpty(nd.ProtocolJSON, "servername", "sni"); sni != "" {
				q.Set("peer", sni)
			}
			if pk, ok := reality["public-key"].(string); ok && pk != "" {
				q.Set("pbk", pk)
			}
			if sid, ok := reality["short-id"].(string); ok && sid != "" {
				q.Set("sid", sid)
			}
		} else if boolVal(nd.ProtocolJSON, "tls", false) {
			q.Set("tls", "1")
			if sni := firstNonEmpty(nd.ProtocolJSON, "servername", "sni"); sni != "" {
				q.Set("peer", sni)
			}
		}
		return "vless://" + userinfo + "?" + encodeQuery(q), nil
	case "trojan":
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("allowInsecure", "1")
		}
		return "trojan://" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "anytls":
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if boolVal(nd.ProtocolJSON, "allowInsecure", false) || boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("allowInsecure", "1")
		}
		if boolVal(nd.ProtocolJSON, "udp", true) {
			q.Set("udp", "1")
		}
		return "anytls://" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "hysteria2":
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if obfs := str(nd.ProtocolJSON, "obfs", ""); obfs != "" {
			q.Set("obfs", obfs)
		}
		if obfsPwd := str(nd.ProtocolJSON, "obfs-password", ""); obfsPwd != "" {
			q.Set("obfs-password", obfsPwd)
		}
		if boolVal(nd.ProtocolJSON, "insecure", false) {
			q.Set("insecure", "1")
		}
		return "hysteria2://" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "hysteria":
		q := url.Values{}
		if p := str(nd.ProtocolJSON, "protocol", "udp"); p != "" {
			q.Set("protocol", p)
		}
		if auth := str(nd.ProtocolJSON, "auth", ""); auth != "" {
			q.Set("auth", auth)
		}
		if up := str(nd.ProtocolJSON, "up", ""); up != "" {
			q.Set("upmbps", up)
		}
		if down := str(nd.ProtocolJSON, "down", ""); down != "" {
			q.Set("downmbps", down)
		}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if obfs := str(nd.ProtocolJSON, "obfs", ""); obfs != "" {
			q.Set("obfs", obfs)
		}
		return "hysteria://" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "tuic":
		uuid := str(nd.ProtocolJSON, "uuid", "")
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if boolVal(nd.ProtocolJSON, "allow_insecure", false) || boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("allow_insecure", "1")
		}
		return "tuic://" + uuid + ":" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "wireguard":
		privateKey := url.QueryEscape(str(nd.ProtocolJSON, "private-key", ""))
		q := url.Values{}
		for _, k := range []string{"public-key", "address", "allowed-ips", "mtu", "dns", "reserved"} {
			if v := str(nd.ProtocolJSON, k, ""); v != "" {
				q.Set(k, v)
			}
		}
		return "wireguard://" + privateKey + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "http":
		user := url.QueryEscape(str(nd.ProtocolJSON, "username", ""))
		pass := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if boolVal(nd.ProtocolJSON, "tls", false) {
			q.Set("tls", "1")
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("skip-cert-verify", "1")
		}
		return "http://" + user + ":" + pass + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "socks5":
		user := url.QueryEscape(str(nd.ProtocolJSON, "username", ""))
		pass := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if boolVal(nd.ProtocolJSON, "tls", false) {
			q.Set("tls", "1")
		}
		if boolVal(nd.ProtocolJSON, "udp", true) {
			q.Set("udp", "1")
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("skip-cert-verify", "1")
		}
		return "socks5://" + user + ":" + pass + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	default:
		return "", fmt.Errorf("协议无标准链接映射: %s", nd.Protocol)
	}
}

// genericLink 生成通用标准节点链接。
func genericLink(nd *nodeData) (string, error) {
	host, err := punycode(nd.Host)
	if err != nil {
		return "", err
	}
	name := url.QueryEscape(nd.RenderName)
	switch nd.Protocol {
	case "ss":
		return srLink(nd)
	case "vmess":
		obj := map[string]any{
			"v": "2", "ps": nd.RenderName, "add": host, "port": strconv.Itoa(nd.Port),
			"id": str(nd.ProtocolJSON, "uuid", ""), "aid": str(nd.ProtocolJSON, "alterId", "0"),
			"scy": str(nd.ProtocolJSON, "cipher", "auto"), "net": str(nd.ProtocolJSON, "network", "tcp"),
			"type": "none", "host": str(nd.ProtocolJSON, "host", ""), "path": str(nd.ProtocolJSON, "path", ""),
			"tls": str(nd.ProtocolJSON, "tls", ""),
		}
		raw, _ := json.Marshal(obj)
		return "vmess://" + base64.StdEncoding.EncodeToString(raw), nil
	case "vless":
		uuid := str(nd.ProtocolJSON, "uuid", "")
		q := url.Values{}
		q.Set("encryption", "none")
		q.Set("type", str(nd.ProtocolJSON, "network", "tcp"))
		reality := realityOpts(nd.ProtocolJSON)
		if reality != nil {
			q.Set("security", "reality")
			if sni := firstNonEmpty(nd.ProtocolJSON, "servername", "sni"); sni != "" {
				q.Set("sni", sni)
			}
			if fp := str(nd.ProtocolJSON, "client-fingerprint", ""); fp != "" {
				q.Set("fp", fp)
			}
			if pk, ok := reality["public-key"].(string); ok && pk != "" {
				q.Set("pbk", pk)
			}
			if sid, ok := reality["short-id"].(string); ok && sid != "" {
				q.Set("sid", sid)
			}
		} else if boolVal(nd.ProtocolJSON, "tls", false) {
			q.Set("security", "tls")
			if sni := firstNonEmpty(nd.ProtocolJSON, "servername", "sni"); sni != "" {
				q.Set("sni", sni)
			}
			if fp := str(nd.ProtocolJSON, "client-fingerprint", ""); fp != "" {
				q.Set("fp", fp)
			}
		}
		if flow := str(nd.ProtocolJSON, "flow", ""); flow != "" {
			q.Set("flow", flow)
		}
		return "vless://" + uuid + "@" + host + ":" + strconv.Itoa(nd.Port) + "?" + encodeQuery(q) + "#" + name, nil
	case "trojan":
		return srLink(nd)
	case "anytls":
		return srLink(nd)
	case "hysteria2":
		return srLink(nd)
	case "hysteria":
		return srLink(nd)
	case "tuic":
		return srLink(nd)
	case "wireguard":
		return srLink(nd)
	case "http":
		return srLink(nd)
	case "socks5":
		return srLink(nd)
	default:
		return "", fmt.Errorf("协议无标准链接映射: %s", nd.Protocol)
	}
}

func punycode(host string) (string, error) {
	ascii, err := idna.Lookup.ToASCII(host)
	if err != nil {
		return "", errors.New("域名转 punycode 失败")
	}
	return ascii, nil
}

func str(m map[string]any, key, def string) string {
	if v, ok := m[key]; ok && v != nil {
		switch val := v.(type) {
		case string:
			if val != "" {
				return val
			}
		case float64:
			return strconv.FormatFloat(val, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(val)
		}
	}
	return def
}

func boolVal(m map[string]any, key string, def bool) bool {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case bool:
			return val
		case string:
			if b, err := strconv.ParseBool(val); err == nil {
				return b
			}
		case float64:
			return val != 0
		}
	}
	return def
}

func firstNonEmpty(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := str(m, k, ""); v != "" {
			return v
		}
	}
	return ""
}

// encodeQuery 编码查询参数，并按 Build5 要求把 `+` 替换为 `%20`，避免空格不对称。
func encodeQuery(q url.Values) string {
	return strings.ReplaceAll(q.Encode(), "+", "%20")
}

func realityOpts(m map[string]any) map[string]any {
	if v, ok := m["reality-opts"]; ok {
		if mm, ok := v.(map[string]any); ok {
			return mm
		}
		if s, ok := v.(string); ok && s != "" {
			var mm map[string]any
			if err := json.Unmarshal([]byte(s), &mm); err == nil {
				return mm
			}
		}
	}
	if v, ok := m["reality"]; ok {
		if mm, ok := v.(map[string]any); ok {
			return mm
		}
	}
	return nil
}

var _ = strings.TrimSpace
