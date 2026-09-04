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

	"vpn-sub/internal/ssplugin"
)

// nodeData 是链接渲染所需的最小节点视图。
type nodeData struct {
	Protocol     string
	RenderName   string
	Host         string
	Port         int
	ProtocolJSON map[string]any
}

func ssLink(nd *nodeData, target string) (string, error) {
	host, err := punycode(nd.Host)
	if err != nil {
		return "", err
	}
	pluginValue := ""
	if plugin := str(nd.ProtocolJSON, "plugin", ""); plugin != "" {
		pluginValue, err = renderPluginForTarget(plugin, PluginOpts(nd.ProtocolJSON, plugin), target)
		if err != nil {
			return "", err
		}
	}
	cipher := str(nd.ProtocolJSON, "cipher", "auto")
	password := str(nd.ProtocolJSON, "password", "")
	userinfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
	q := url.Values{}
	if pluginValue != "" {
		q.Set("plugin", pluginValue)
	}
	return fmt.Sprintf("ss://%s@%s:%d%s#%s", userinfo, host, nd.Port, querySuffix(q), url.QueryEscape(nd.RenderName)), nil
}

// srLink 生成 Shadowrocket 原生参数风格节点链接。
func srLink(nd *nodeData) (string, error) {
	if nd.Protocol == "ss" {
		return ssLink(nd, ssplugin.TargetShadowrocket)
	}
	host, err := punycode(nd.Host)
	if err != nil {
		return "", err
	}
	name := url.QueryEscape(nd.RenderName)
	switch nd.Protocol {
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
		return ssLink(nd, ssplugin.TargetGeneric)
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

// PluginOpts 从拆分后的协议参数中读取当前插件对应的独立对象。
func PluginOpts(params map[string]any, plugin string) map[string]any {
	switch plugin {
	case "obfs":
		return object(params, "obfs-opts")
	case "v2ray-plugin":
		return object(params, "v2ray-plugin-opts")
	case "shadow-tls":
		return object(params, "shadow-tls-opts")
	case "restls":
		return object(params, "restls-opts")
	default:
		return object(params, "plugin-opts")
	}
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

func renderPluginForTarget(name string, opts map[string]any, target string) (string, error) {
	definition, known := ssplugin.Lookup(name)
	if !known {
		if target != ssplugin.TargetShadowrocket {
			return "", fmt.Errorf("%s 不支持未知 SS 插件 %s", target, name)
		}
		stringOpts, err := stringPluginOpts(name, opts, nil)
		if err != nil {
			return "", err
		}
		return ssplugin.SerializePluginString(name, stringOpts)
	}
	contract, ok := definition.Target(target)
	if !ok || contract.Support == ssplugin.SupportUnsupported {
		return "", fmt.Errorf("%s 不支持 SS 插件 %s", target, name)
	}
	allowed := make(map[string]struct{}, len(contract.ExpressibleFields))
	for _, field := range contract.ExpressibleFields {
		allowed[field] = struct{}{}
	}
	stringOpts, err := stringPluginOpts(name, opts, allowed)
	if err != nil {
		return "", err
	}
	pluginName := name
	if name == "obfs" {
		pluginName = "obfs-local"
		mode := stringOpts["mode"]
		if mode == "" {
			mode = "http"
		}
		delete(stringOpts, "mode")
		stringOpts["obfs"] = mode
		if host, ok := stringOpts["host"]; ok {
			delete(stringOpts, "host")
			stringOpts["obfs-host"] = host
		}
	}
	return ssplugin.SerializePluginString(pluginName, stringOpts)
}

func stringPluginOpts(plugin string, opts map[string]any, allowed map[string]struct{}) (map[string]string, error) {
	out := make(map[string]string, len(opts))
	for key, value := range opts {
		if allowed != nil {
			if _, ok := allowed[key]; !ok {
				return nil, fmt.Errorf("SS 插件 %s 参数 %s 无法在 URI 中无损表达", plugin, key)
			}
		}
		switch typed := value.(type) {
		case string:
			out[key] = typed
		case bool:
			if !isPluginBoolField(plugin, key) {
				return nil, fmt.Errorf("SS 插件 %s 参数 %s 的 bool 类型没有 URI 映射", plugin, key)
			}
			out[key] = strconv.FormatBool(typed)
		case float64:
			if plugin != "shadow-tls" || key != "version" {
				return nil, fmt.Errorf("SS 插件 %s 参数 %s 的 number 类型没有 URI 映射", plugin, key)
			}
			out[key] = strconv.FormatFloat(typed, 'f', -1, 64)
		case int:
			if plugin != "shadow-tls" || key != "version" {
				return nil, fmt.Errorf("SS 插件 %s 参数 %s 的 number 类型没有 URI 映射", plugin, key)
			}
			out[key] = strconv.Itoa(typed)
		case []string:
			joined, err := joinPluginList(plugin, key, typed)
			if err != nil {
				return nil, err
			}
			out[key] = joined
		case []any:
			values := make([]string, 0, len(typed))
			for _, item := range typed {
				text, ok := item.(string)
				if !ok {
					return nil, fmt.Errorf("SS 插件 %s 参数 %s 的列表包含非字符串值", plugin, key)
				}
				values = append(values, text)
			}
			joined, err := joinPluginList(plugin, key, values)
			if err != nil {
				return nil, err
			}
			out[key] = joined
		default:
			return nil, fmt.Errorf("SS 插件 %s 参数 %s 的 %T 类型无法在 URI 中无损表达", plugin, key, value)
		}
	}
	return out, nil
}

func isPluginBoolField(plugin, key string) bool {
	return plugin == "v2ray-plugin" && key == "tls" ||
		(plugin == "shadow-tls" || plugin == "restls") && key == "skip-cert-verify"
}

func joinPluginList(plugin, key string, values []string) (string, error) {
	if plugin != "shadow-tls" || key != "alpn" {
		return "", fmt.Errorf("SS 插件 %s 参数 %s 的 list 类型没有 URI 映射", plugin, key)
	}
	for _, value := range values {
		if strings.Contains(value, ",") {
			return "", fmt.Errorf("SS 插件 %s 参数 %s 的列表项包含无法无损转义的逗号", plugin, key)
		}
	}
	return strings.Join(values, ","), nil
}

// RenderPluginForClashLegacy 保留 Step 11 前的 Clash 旧投影行为。
func RenderPluginForClashLegacy(name string, opts map[string]any) string {
	switch name {
	case "obfs":
		parts := []string{"obfs-local", "obfs=" + str(opts, "mode", "http")}
		if host := str(opts, "host", ""); host != "" {
			parts = append(parts, "obfs-host="+host)
		}
		return strings.Join(parts, ";")
	case "v2ray-plugin":
		parts := []string{"v2ray-plugin"}
		for _, key := range []string{"mode", "host", "path"} {
			if value := str(opts, key, ""); value != "" {
				parts = append(parts, key+"="+value)
			}
		}
		if boolVal(opts, "tls", false) {
			parts = append(parts, "tls=true")
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
