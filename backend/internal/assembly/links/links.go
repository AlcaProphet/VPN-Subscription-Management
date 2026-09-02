package links

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/idna"
)

// nodeData 是链接渲染所需的最小节点视图。
type nodeData struct {
	Protocol     string
	RenderName   string
	Host         string
	Port         int
	ProtocolJSON map[string]any
}

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
		q := url.Values{}
		if plugin := str(nd.ProtocolJSON, "plugin", ""); plugin != "" {
			q.Set("plugin", renderPluginString(plugin, object(nd.ProtocolJSON, "plugin-opts")))
		}
		return fmt.Sprintf("ss://%s@%s:%d%s#%s", userinfo, host, nd.Port, querySuffix(q), name), nil
	case "vmess":
		cipher := str(nd.ProtocolJSON, "cipher", "auto")
		uuid := str(nd.ProtocolJSON, "uuid", "")
		userinfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + uuid + "@" + host + ":" + strconv.Itoa(nd.Port)))
		q := url.Values{}
		q.Set("remarks", nd.RenderName)
		if boolVal(nd.ProtocolJSON, "udp", true) {
			q.Set("udp", "1")
		}
		q.Set("alterId", str(nd.ProtocolJSON, "alterId", "0"))
		addCommonSRQuery(nd.ProtocolJSON, q)
		transportQuery(nd.ProtocolJSON, q)
		return "vmess://" + userinfo + querySuffix(q), nil
	case "vless":
		uuid := str(nd.ProtocolJSON, "uuid", "")
		userinfo := base64.StdEncoding.EncodeToString([]byte(":" + uuid + "@" + host + ":" + strconv.Itoa(nd.Port)))
		q := url.Values{}
		q.Set("remarks", nd.RenderName)
		if boolVal(nd.ProtocolJSON, "udp", true) {
			q.Set("udp", "1")
		}
		addCommonSRQuery(nd.ProtocolJSON, q)
		transportQuery(nd.ProtocolJSON, q)
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
		return "vless://" + userinfo + querySuffix(q), nil
	case "trojan":
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if alpn := listString(nd.ProtocolJSON, "alpn"); alpn != "" {
			q.Set("alpn", alpn)
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("allowInsecure", "1")
		}
		return "trojan://" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
	case "anytls":
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if alpn := listString(nd.ProtocolJSON, "alpn"); alpn != "" {
			q.Set("alpn", alpn)
		}
		if fp := str(nd.ProtocolJSON, "client-fingerprint", ""); fp != "" {
			q.Set("client-fingerprint", fp)
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("allowInsecure", "1")
		}
		if boolVal(nd.ProtocolJSON, "udp", true) {
			q.Set("udp", "1")
		}
		return "anytls://" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
	case "hysteria2":
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if alpn := listString(nd.ProtocolJSON, "alpn"); alpn != "" {
			q.Set("alpn", alpn)
		}
		if obfs := str(nd.ProtocolJSON, "obfs", ""); obfs != "" {
			q.Set("obfs", obfs)
		}
		if obfsPwd := str(nd.ProtocolJSON, "obfs-password", ""); obfsPwd != "" {
			q.Set("obfs-password", obfsPwd)
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("insecure", "1")
		}
		return "hysteria2://" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
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
		if alpn := listString(nd.ProtocolJSON, "alpn"); alpn != "" {
			q.Set("alpn", alpn)
		}
		if ports := str(nd.ProtocolJSON, "ports", ""); ports != "" {
			q.Set("mport", ports)
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("insecure", "1")
		}
		if obfs := str(nd.ProtocolJSON, "obfs", ""); obfs != "" {
			q.Set("obfs", obfs)
		}
		return "hysteria://" + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
	case "tuic":
		uuid := str(nd.ProtocolJSON, "uuid", "")
		password := url.QueryEscape(str(nd.ProtocolJSON, "password", ""))
		q := url.Values{}
		if sni := firstNonEmpty(nd.ProtocolJSON, "sni", "servername"); sni != "" {
			q.Set("sni", sni)
		}
		if alpn := listString(nd.ProtocolJSON, "alpn"); alpn != "" {
			q.Set("alpn", alpn)
		}
		if boolVal(nd.ProtocolJSON, "skip-cert-verify", false) {
			q.Set("allow_insecure", "1")
		}
		return "tuic://" + uuid + ":" + password + "@" + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
	case "wireguard":
		privateKey := url.QueryEscape(str(nd.ProtocolJSON, "private-key", ""))
		q := url.Values{}
		for _, k := range []string{"public-key", "ip", "ipv6", "allowed-ips", "pre-shared-key", "mtu", "dns", "reserved"} {
			if v := listString(nd.ProtocolJSON, k); v != "" {
				q.Set(k, v)
			}
		}
		return "wireguard://" + privateKey + "@" + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
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
		return "http://" + userinfoPart(user, pass) + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
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
		return "socks5://" + userinfoPart(user, pass) + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
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
		tls := ""
		if boolVal(nd.ProtocolJSON, "tls", false) {
			tls = "tls"
		}
		network := str(nd.ProtocolJSON, "network", "tcp")
		path, transportHost, mode := transportValues(nd.ProtocolJSON, network)
		obj := map[string]any{
			"v": "2", "ps": nd.RenderName, "add": host, "port": strconv.Itoa(nd.Port),
			"id": str(nd.ProtocolJSON, "uuid", ""), "aid": str(nd.ProtocolJSON, "alterId", "0"),
			"scy": str(nd.ProtocolJSON, "cipher", "auto"), "net": network,
			"type": mode, "host": transportHost, "path": path, "udp": boolVal(nd.ProtocolJSON, "udp", true),
			"tls": tls,
		}
		if sni := firstNonEmpty(nd.ProtocolJSON, "servername", "sni"); sni != "" {
			obj["sni"] = sni
		}
		if alpn := listString(nd.ProtocolJSON, "alpn"); alpn != "" {
			obj["alpn"] = alpn
		}
		if fp := str(nd.ProtocolJSON, "client-fingerprint", ""); fp != "" {
			obj["fp"] = fp
		}
		raw, err := json.Marshal(obj)
		if err != nil {
			return "", fmt.Errorf("序列化 vmess 链接失败: %w", err)
		}
		return "vmess://" + base64.StdEncoding.EncodeToString(raw), nil
	case "vless":
		uuid := str(nd.ProtocolJSON, "uuid", "")
		q := url.Values{}
		q.Set("encryption", "none")
		q.Set("type", str(nd.ProtocolJSON, "network", "tcp"))
		transportQuery(nd.ProtocolJSON, q)
		if alpn := listString(nd.ProtocolJSON, "alpn"); alpn != "" {
			q.Set("alpn", alpn)
		}
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
		return "vless://" + uuid + "@" + host + ":" + strconv.Itoa(nd.Port) + querySuffix(q) + "#" + name, nil
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

func object(m map[string]any, key string) map[string]any {
	value, _ := m[key].(map[string]any)
	return value
}

func listString(m map[string]any, key string) string {
	value, ok := m[key]
	if !ok {
		return ""
	}
	switch items := value.(type) {
	case []string:
		return strings.Join(items, ",")
	case []any:
		parts := make([]string, 0, len(items))
		for _, item := range items {
			parts = append(parts, fmt.Sprint(item))
		}
		return strings.Join(parts, ",")
	default:
		return str(m, key, "")
	}
}

func addCommonSRQuery(params map[string]any, q url.Values) {
	if boolVal(params, "tfo", false) {
		q.Set("tfo", "1")
	}
}

func transportQuery(params map[string]any, q url.Values) {
	network := str(params, "network", "tcp")
	q.Set("type", network)
	path, host, mode := transportValues(params, network)
	if path != "" {
		if network == "grpc" {
			q.Set("serviceName", path)
		} else {
			q.Set("path", path)
		}
	}
	if host != "" {
		q.Set("host", host)
	}
	if mode != "" && mode != "none" {
		q.Set("mode", mode)
	}
}

func transportValues(params map[string]any, network string) (path, host, mode string) {
	mode = "none"
	switch network {
	case "ws":
		ws := object(params, "ws-opts")
		path = str(ws, "path", "")
		headers := object(ws, "headers")
		host = firstNonEmpty(headers, "Host", "host")
	case "grpc":
		grpc := object(params, "grpc-opts")
		path = str(grpc, "grpc-service-name", "")
	case "h2":
		h2 := object(params, "h2-opts")
		path = str(h2, "path", "")
		host = listString(h2, "host")
	case "http":
		httpOpts := object(params, "http-opts")
		path = listString(httpOpts, "path")
		headers := object(httpOpts, "headers")
		host = firstNonEmpty(headers, "Host", "host")
	case "xhttp":
		xhttp := object(params, "xhttp-opts")
		path = str(xhttp, "path", "")
		host = str(xhttp, "host", "")
		mode = str(xhttp, "mode", "none")
	}
	return path, host, mode
}

func pluginString(name string, opts map[string]any) string {
	if len(opts) == 0 {
		return name
	}
	keys := make([]string, 0, len(opts))
	for key := range opts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{name}
	for _, key := range keys {
		parts = append(parts, key+"="+fmt.Sprint(opts[key]))
	}
	return strings.Join(parts, ";")
}

// renderPluginString 按目标客户端偏好渲染 SS 插件参数。
// 内部 obfs 映射为 SIP003/CVR 偏好形态；v2ray-plugin/shadow-tls/restls 暂保留原格式。
func renderPluginString(name string, opts map[string]any) string {
	switch name {
	case "obfs":
		parts := []string{"obfs-local", "obfs=" + str(opts, "mode", "http")}
		if host := str(opts, "host", ""); host != "" {
			parts = append(parts, "obfs-host="+host)
		}
		return strings.Join(parts, ";")
	default:
		return pluginString(name, opts)
	}
}

// encodeQuery 编码查询参数，并按 Build5 要求把 `+` 替换为 `%20`，避免空格不对称。
func encodeQuery(q url.Values) string {
	return strings.ReplaceAll(q.Encode(), "+", "%20")
}

// querySuffix 空参数时不输出 `?`（R14-05）。
func querySuffix(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	return "?" + encodeQuery(q)
}

// userinfoPart http/socks5 空凭据时不输出 `user:pass@`（R14-05）。
func userinfoPart(user, pass string) string {
	if user == "" && pass == "" {
		return ""
	}
	return user + ":" + pass + "@"
}

// Render 导出给下载渲染复用：按协议生成 SR 或通用标准链接。
func Render(protocol, renderName, host string, port int, params map[string]any, generic bool) (string, error) {
	nd := &nodeData{
		Protocol:     protocol,
		RenderName:   renderName,
		Host:         host,
		Port:         port,
		ProtocolJSON: params,
	}
	if generic {
		return genericLink(nd)
	}
	return srLink(nd)
}

func realityOpts(m map[string]any) map[string]any {
	// v1.3 起只接受 mihomo 原生 reality-opts 对象。
	if v, ok := m["reality-opts"]; ok {
		if mm, ok := v.(map[string]any); ok {
			return mm
		}
	}
	return nil
}
