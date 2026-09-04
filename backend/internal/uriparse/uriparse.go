// Package uriparse 提供 manual 节点批量导入的 URI 解析能力。
// 解析规则对齐 clash-verge-rev src/utils/uri-parser，ssr 按项目决策排除。
package uriparse

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"vpn-sub/internal/ssplugin"
)

// Result 是单条 URI 解析结果；Params 直接对应节点 protocol_json。
type Result struct {
	Protocol string         `json:"protocol"`
	Name     string         `json:"name"`
	Host     string         `json:"host"`
	Port     int            `json:"port"`
	Params   map[string]any `json:"params"`
}

// Skip 记录无法解析或跳过的行。
type Skip struct {
	Line   int    `json:"line"`
	Raw    string `json:"raw"`
	Reason string `json:"reason"`
}

// Parse 解析单行节点 URI。
func Parse(raw string) (*Result, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, errors.New("空行")
	}
	scheme := strings.ToLower(strings.SplitN(s, "://", 2)[0])
	switch scheme {
	case "ss":
		return parseSS(s)
	case "vmess":
		return parseVMess(s)
	case "vless":
		return parseVLESS(s)
	case "trojan":
		return parseTrojan(s)
	case "anytls":
		return parseAnyTLS(s)
	case "hysteria2", "hy2":
		return parseHysteria2(s)
	case "hysteria", "hy":
		return parseHysteria(s)
	case "tuic":
		return parseTUIC(s)
	case "wireguard", "wg":
		return parseWireGuard(s)
	case "http", "https":
		return parseHTTP(s)
	case "socks5", "socks":
		return parseSocks5(s)
	default:
		return nil, fmt.Errorf("暂不支持导入的 scheme: %s", scheme)
	}
}

// ParseBlock 解析多行/整块 Base64 文本，返回逐行结果与跳过原因。
func ParseBlock(text string) ([]Result, []Skip) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	lines := strings.Split(text, "\n")
	if !strings.Contains(text, "://") {
		joined := strings.Join(lines, "")
		if decoded := decodeBase64OrOriginal(joined); strings.Contains(decoded, "://") {
			lines = strings.Split(decoded, "\n")
		}
	}
	var results []Result
	var skips []Skip
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		res, err := Parse(trimmed)
		if err != nil {
			skips = append(skips, Skip{Line: i + 1, Raw: trimmed, Reason: err.Error()})
			continue
		}
		results = append(results, *res)
	}
	return results, skips
}

func parseSS(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "ss://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	if main == "" {
		return nil, errors.New("ss 缺少主体")
	}
	if !strings.Contains(main, "@") {
		main = decodeBase64OrOriginal(main)
	}
	at := strings.LastIndex(main, "@")
	if at < 0 {
		return nil, errors.New("ss 缺少 @")
	}
	userinfoRaw := main[:at]
	hostport := main[at+1:]
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	userinfo := decodeBase64OrOriginal(userinfoRaw)
	ci := strings.Index(userinfo, ":")
	if ci < 0 {
		return nil, errors.New("ss userinfo 缺少分隔符")
	}
	cipher := userinfo[:ci]
	password := userinfo[ci+1:]
	params := map[string]any{"cipher": cipher, "password": password}
	q := query
	if plugin := q.Get("plugin"); plugin != "" {
		name, opts, err := parseSSPlugin(plugin)
		if err != nil {
			return nil, fmt.Errorf("ss plugin 参数无效: %w", err)
		}
		params["plugin"] = name
		setSSPluginOpts(params, name, opts)
	}
	if v := q.Get("v2ray-plugin"); v != "" && params["plugin"] == nil {
		params["plugin"] = "v2ray-plugin"
		var opts map[string]any
		_ = json.Unmarshal([]byte(decodeBase64OrOriginal(v)), &opts)
		setSSPluginOpts(params, "v2ray-plugin", opts)
	}
	if _, ok := q["uot"]; ok && parseBoolPresence(q.Get("uot")) {
		params["udp-over-tcp"] = true
	}
	if _, ok := q["tfo"]; ok && parseBoolPresence(q.Get("tfo")) {
		params["tfo"] = true
	}
	return &Result{Protocol: "ss", Name: defaultName(name, "SS", host, port), Host: host, Port: port, Params: params}, nil
}

func parseSSPlugin(raw string) (string, map[string]any, error) {
	plugin, rawOpts, err := ssplugin.ParsePluginString(raw)
	if err != nil {
		return "", nil, err
	}
	switch plugin {
	case "obfs-local", "simple-obfs":
		plugin = "obfs"
	}
	opts := stringAnyMap(rawOpts)
	switch plugin {
	case "obfs":
		if value, ok := opts["obfs"]; ok {
			opts["mode"] = value
			delete(opts, "obfs")
		}
		if value, ok := opts["obfs-host"]; ok {
			opts["host"] = value
			delete(opts, "obfs-host")
		}
	case "v2ray-plugin":
		if _, ok := opts["mode"]; !ok {
			opts["mode"] = "websocket"
		}
		if _, ok := opts["host"]; !ok {
			if value, exists := opts["obfs-host"]; exists {
				opts["host"] = value
			}
		}
		delete(opts, "obfs-host")
		if value, ok := opts["tls"].(string); ok {
			parsed, err := parsePluginBool(value)
			if err != nil {
				return "", nil, fmt.Errorf("v2ray-plugin tls 参数无效: %w", err)
			}
			opts["tls"] = parsed
		}
	case "shadow-tls":
		if value, ok := opts["version"].(string); ok {
			parsed, err := strconv.ParseFloat(value, 64)
			if err != nil {
				return "", nil, fmt.Errorf("shadow-tls version 参数无效: %w", err)
			}
			opts["version"] = parsed
		}
		if value, ok := opts["alpn"].(string); ok {
			opts["alpn"] = splitCSV(value)
		}
		if err := parsePluginBoolField(opts, "skip-cert-verify"); err != nil {
			return "", nil, fmt.Errorf("shadow-tls skip-cert-verify 参数无效: %w", err)
		}
	case "restls":
		if err := parsePluginBoolField(opts, "skip-cert-verify"); err != nil {
			return "", nil, fmt.Errorf("restls skip-cert-verify 参数无效: %w", err)
		}
	}
	return plugin, opts, nil
}

func parsePluginBoolField(opts map[string]any, key string) error {
	value, ok := opts[key].(string)
	if !ok {
		return nil
	}
	parsed, err := parsePluginBool(value)
	if err != nil {
		return err
	}
	opts[key] = parsed
	return nil
}

func parsePluginBool(value string) (bool, error) {
	switch strings.ToLower(value) {
	case "", "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	default:
		return false, fmt.Errorf("应为 bare flag、true、false、1 或 0")
	}
}

func stringAnyMap(values map[string]string) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func setSSPluginOpts(params map[string]any, plugin string, opts map[string]any) {
	if len(opts) == 0 {
		return
	}
	switch plugin {
	case "obfs":
		params["obfs-opts"] = opts
	case "v2ray-plugin":
		params["v2ray-plugin-opts"] = opts
	case "shadow-tls":
		params["shadow-tls-opts"] = opts
	case "restls":
		params["restls-opts"] = opts
	default:
		params["plugin-opts"] = opts
	}
}

func parseVMess(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "vmess://")
	if after == "" {
		return nil, errors.New("vmess 缺少主体")
	}
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	decoded := decodeBase64OrOriginal(main)
	var res *Result
	var err error
	if strings.HasPrefix(strings.TrimSpace(decoded), "{") {
		res, err = parseVMessJSON(decoded, query)
	} else {
		res, err = parseVMessSR(decoded, query)
	}
	if err != nil {
		return nil, err
	}
	if decodedName, unescapeErr := url.QueryUnescape(name); unescapeErr == nil && decodedName != "" {
		res.Name = decodedName
	}
	return res, nil
}

func parseVMessJSON(decoded string, query url.Values) (*Result, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(decoded), &m); err != nil {
		return nil, fmt.Errorf("vmess JSON 解析失败: %w", err)
	}
	host := strAny(m["add"])
	port := parseIntAny(m["port"])
	if host == "" || port == 0 {
		return nil, errors.New("vmess 缺少地址或端口")
	}
	tlsRaw := strings.ToLower(strAny(m["tls"]))
	tls := tlsRaw == "tls" || tlsRaw == "true" || tlsRaw == "1"
	params := map[string]any{
		"uuid":    strAny(m["id"]),
		"alterId": intAny(m["aid"]),
		"cipher":  firstNonEmpty(strAny(m["scy"]), "auto"),
		"network": firstNonEmpty(strAny(m["net"]), "tcp"),
		"tls":     tls,
	}
	if v := strAny(m["host"]); v != "" {
		params["servername"] = v
	}
	if v := strAny(m["sni"]); v != "" {
		params["servername"] = v
	}
	if v := strAny(m["fp"]); v != "" {
		params["client-fingerprint"] = v
	}
	if v := strAny(m["alpn"]); v != "" {
		params["alpn"] = splitCSV(v)
	}
	if v := strAny(m["path"]); v != "" {
		params["path"] = v
	}
	if v := strAny(m["host"]); v != "" {
		params["host"] = v
	}
	if v := strAny(m["type"]); v != "" && v != "none" {
		params["mode"] = v
	}
	if b := m["udp"]; b != nil {
		params["udp"] = boolOrString(b)
	}
	if m["skip-cert-verify"] != nil {
		params["skip-cert-verify"] = boolOrString(m["skip-cert-verify"])
	}
	applyVMessTransport(params)
	name := firstNonEmpty(strAny(m["ps"]), strAny(m["remarks"]), strAny(m["remark"]))
	return &Result{Protocol: "vmess", Name: defaultName(name, "VMess", host, port), Host: host, Port: port, Params: params}, nil
}

func parseVMessSR(decoded string, query url.Values) (*Result, error) {
	// 形如 cipher:uuid@host:port
	at := strings.LastIndex(decoded, "@")
	if at < 0 {
		return nil, errors.New("vmess Shadowrocket 缺少 @")
	}
	user := decoded[:at]
	hostport := decoded[at+1:]
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	ci := strings.Index(user, ":")
	if ci < 0 {
		return nil, errors.New("vmess Shadowrocket userinfo 缺少分隔符")
	}
	params := map[string]any{
		"uuid":    user[ci+1:],
		"cipher":  user[:ci],
		"network": "tcp",
		"udp":     true,
	}
	if v := query.Get("remarks"); v != "" {
		// name from query
	}
	if query.Get("udp") != "" {
		params["udp"] = parseBoolPresence(query.Get("udp"))
	}
	if v := query.Get("alterId"); v != "" {
		params["alterId"] = parseIntString(v)
	}
	if v := query.Get("tls"); v != "" {
		params["tls"] = parseBoolPresence(v)
	}
	if v := query.Get("peer"); v != "" {
		params["servername"] = v
	} else if v := query.Get("sni"); v != "" {
		params["servername"] = v
	}
	if v := query.Get("type"); v != "" {
		params["network"] = normalizeNetwork(v)
	}
	if v := query.Get("path"); v != "" {
		params["path"] = v
	}
	if v := query.Get("host"); v != "" {
		params["host"] = v
	}
	if v := query.Get("mode"); v != "" {
		params["mode"] = v
	}
	if v := query.Get("fp"); v != "" {
		params["client-fingerprint"] = v
	}
	applyVMessTransport(params)
	name := firstNonEmpty(query.Get("remarks"), query.Get("remark"))
	return &Result{Protocol: "vmess", Name: defaultName(name, "VMess", host, port), Host: host, Port: port, Params: params}, nil
}

func applyVMessTransport(params map[string]any) {
	network := strAny(params["network"])
	switch network {
	case "ws":
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = path
		}
		if host, ok := params["host"]; ok {
			opts["headers"] = map[string]any{"Host": host}
		}
		if len(opts) > 0 {
			params["ws-opts"] = opts
		}
	case "grpc":
		if path, ok := params["path"]; ok {
			params["grpc-opts"] = map[string]any{"grpc-service-name": path}
		}
	case "h2":
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = path
		}
		if host, ok := params["host"]; ok {
			opts["host"] = host
		}
		if len(opts) > 0 {
			params["h2-opts"] = opts
		}
	case "http":
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = []any{path}
		}
		if host, ok := params["host"]; ok {
			opts["headers"] = map[string]any{"Host": []any{host}}
		}
		if len(opts) > 0 {
			params["http-opts"] = opts
		}
	}
}

func parseVLESS(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "vless://")
	if after == "" {
		return nil, errors.New("vless 缺少主体")
	}
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	decodedOrMain := main
	if !strings.Contains(main, "@") {
		decodedOrMain = decodeBase64OrOriginal(main)
	}
	at := strings.LastIndex(decodedOrMain, "@")
	if at < 0 {
		return nil, errors.New("vless 缺少 @")
	}
	userRaw := decodedOrMain[:at]
	hostport := decodedOrMain[at+1:]
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	uuid := userRaw
	if ci := strings.Index(userRaw, ":"); ci >= 0 {
		uuid = userRaw[ci+1:]
	}
	params := map[string]any{
		"uuid":       uuid,
		"network":    "tcp",
		"encryption": "none",
		"tls":        false,
	}
	q := query
	if v := q.Get("type"); v != "" {
		params["network"] = normalizeNetwork(v)
	} else if v := q.Get("headerType"); v != "" {
		params["network"] = normalizeNetwork(v)
	}
	security := q.Get("security")
	if security != "" && security != "none" {
		params["tls"] = true
	}
	if v := q.Get("sni"); v != "" {
		params["servername"] = v
	} else if v := q.Get("peer"); v != "" {
		params["servername"] = v
	}
	if v := q.Get("flow"); v != "" && v != "none" {
		params["flow"] = v
	}
	if v := q.Get("fp"); v != "" {
		params["client-fingerprint"] = v
	}
	if v := q.Get("alpn"); v != "" {
		params["alpn"] = splitCSV(v)
	}
	if v := q.Get("path"); v != "" {
		params["path"] = v
	}
	if v := q.Get("host"); v != "" {
		params["host"] = v
	}
	if v := q.Get("serviceName"); v != "" {
		params["path"] = v
		params["network"] = "grpc"
	}
	if v := q.Get("mode"); v != "" {
		params["mode"] = v
	}
	if _, ok := q["allowInsecure"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("allowInsecure"))
	}
	if _, ok := q["skip-cert-verify"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("skip-cert-verify"))
	}
	if security == "reality" {
		opts := map[string]any{}
		if v := q.Get("pbk"); v != "" {
			opts["public-key"] = v
		}
		if v := q.Get("sid"); v != "" {
			opts["short-id"] = v
		}
		if len(opts) > 0 {
			params["reality-opts"] = opts
		}
	}
	applyVLESSTransport(params)
	decodedName, _ := url.QueryUnescape(name)
	if decodedName == "" {
		decodedName = firstNonEmpty(q.Get("remarks"), q.Get("remark"))
	}
	return &Result{Protocol: "vless", Name: defaultName(decodedName, "VLESS", host, port), Host: host, Port: port, Params: params}, nil
}

func applyVLESSTransport(params map[string]any) {
	network := strAny(params["network"])
	switch network {
	case "ws":
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = path
		}
		if host, ok := params["host"]; ok {
			opts["headers"] = map[string]any{"Host": host}
		}
		if len(opts) > 0 {
			params["ws-opts"] = opts
		}
	case "grpc":
		if path, ok := params["path"]; ok {
			params["grpc-opts"] = map[string]any{"grpc-service-name": path}
		}
	case "h2":
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = path
		}
		if host, ok := params["host"]; ok {
			opts["host"] = host
		}
		if len(opts) > 0 {
			params["h2-opts"] = opts
		}
	case "http":
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = []any{path}
		}
		if host, ok := params["host"]; ok {
			opts["headers"] = map[string]any{"Host": []any{host}}
		}
		if len(opts) > 0 {
			params["http-opts"] = opts
		}
	case "xhttp":
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = path
		}
		if host, ok := params["host"]; ok {
			opts["host"] = host
		}
		if mode, ok := params["mode"]; ok {
			opts["mode"] = mode
		}
		if len(opts) > 0 {
			params["xhttp-opts"] = opts
		}
	}
}

func parseTrojan(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "trojan://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	at := strings.LastIndex(main, "@")
	if at < 0 {
		return nil, errors.New("trojan 缺少 @")
	}
	password := main[:at]
	host, port, err := splitHostPort(main[at+1:])
	if err != nil {
		return nil, err
	}
	params := map[string]any{"password": urlQueryUnescapeOrRaw(password)}
	q := query
	if v := q.Get("sni"); v != "" {
		params["sni"] = v
	} else if v := q.Get("peer"); v != "" {
		params["sni"] = v
	}
	if v := q.Get("alpn"); v != "" {
		params["alpn"] = splitCSV(v)
	}
	if v := q.Get("type"); v != "" {
		params["network"] = normalizeNetwork(v)
	}
	if v := q.Get("path"); v != "" {
		params["path"] = v
	}
	if v := q.Get("host"); v != "" {
		params["host"] = v
	}
	if _, ok := q["allowInsecure"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("allowInsecure"))
	}
	if _, ok := q["skip-cert-verify"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("skip-cert-verify"))
	}
	if v := q.Get("fp"); v != "" {
		params["fingerprint"] = v
	}
	if v := q.Get("client-fingerprint"); v != "" {
		params["client-fingerprint"] = v
	}
	applyTrojanTransport(params)
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "trojan", Name: defaultName(decodedName, "Trojan", host, port), Host: host, Port: port, Params: params}, nil
}

func applyTrojanTransport(params map[string]any) {
	network := strAny(params["network"])
	if network == "ws" {
		opts := map[string]any{}
		if path, ok := params["path"]; ok {
			opts["path"] = path
		}
		if host, ok := params["host"]; ok {
			opts["headers"] = map[string]any{"Host": host}
		}
		if len(opts) > 0 {
			params["ws-opts"] = opts
		}
	} else if network == "grpc" {
		if path, ok := params["path"]; ok {
			params["grpc-opts"] = map[string]any{"grpc-service-name": path}
		}
	}
}

func parseAnyTLS(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "anytls://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	at := strings.LastIndex(main, "@")
	if at < 0 {
		return nil, errors.New("anytls 缺少 @")
	}
	auth := main[:at]
	host, port, err := splitHostPort(main[at+1:])
	if err != nil {
		return nil, err
	}
	params := map[string]any{"password": firstNonEmpty(auth, ""), "udp": true}
	q := query
	if v := q.Get("sni"); v != "" {
		params["sni"] = v
	}
	if v := q.Get("alpn"); v != "" {
		params["alpn"] = splitCSV(v)
	}
	if v := q.Get("fp"); v != "" {
		params["client-fingerprint"] = v
	}
	if v := q.Get("client-fingerprint"); v != "" {
		params["client-fingerprint"] = v
	}
	if v := q.Get("fingerprint"); v != "" {
		params["fingerprint"] = v
	} else if v := q.Get("hpkp"); v != "" {
		params["fingerprint"] = v
	}
	if _, ok := q["allowInsecure"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("allowInsecure"))
	} else if _, ok := q["insecure"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("insecure"))
	}
	if v := q.Get("udp"); v != "" {
		params["udp"] = parseBoolPresence(v)
	}
	if v := q.Get("idle-session-check-interval"); v != "" {
		params["idle-session-check-interval"] = parseIntString(v)
	}
	if v := q.Get("idle-session-timeout"); v != "" {
		params["idle-session-timeout"] = parseIntString(v)
	}
	if v := q.Get("min-idle-session"); v != "" {
		params["min-idle-session"] = parseIntString(v)
	}
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "anytls", Name: defaultName(decodedName, "AnyTLS", host, port), Host: host, Port: port, Params: params}, nil
}

func parseHysteria2(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "hysteria2://")
	after = strings.TrimPrefix(after, "hy2://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	at := strings.LastIndex(main, "@")
	if at < 0 {
		return nil, errors.New("hysteria2 缺少 @")
	}
	password := main[:at]
	host, port, err := splitHostPort(main[at+1:])
	if err != nil {
		return nil, err
	}
	params := map[string]any{"password": urlQueryUnescapeOrRaw(password)}
	q := query
	if v := q.Get("sni"); v != "" {
		params["sni"] = v
	} else if v := q.Get("peer"); v != "" {
		params["sni"] = v
	}
	if v := q.Get("obfs"); v != "" && v != "none" {
		params["obfs"] = v
	}
	if v := q.Get("obfs-password"); v != "" {
		params["obfs-password"] = v
	}
	if v := q.Get("mport"); v != "" {
		params["ports"] = v
	}
	if v := q.Get("alpn"); v != "" {
		params["alpn"] = splitCSV(v)
	}
	if _, ok := q["insecure"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("insecure"))
	}
	if _, ok := q["fastopen"]; ok {
		params["tfo"] = parseBoolPresence(q.Get("fastopen"))
	}
	if v := q.Get("pinSHA256"); v != "" {
		params["fingerprint"] = v
	}
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "hysteria2", Name: defaultName(decodedName, "Hysteria2", host, port), Host: host, Port: port, Params: params}, nil
}

func parseHysteria(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "hysteria://")
	after = strings.TrimPrefix(after, "hy://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	hostport := main
	if at := strings.LastIndex(main, "@"); at >= 0 {
		hostport = main[at+1:]
	}
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	params := map[string]any{"protocol": "udp"}
	q := query
	if v := q.Get("auth"); v != "" {
		params["auth"] = v
	}
	if v := q.Get("mport"); v != "" {
		params["ports"] = v
	}
	if v := q.Get("obfsParam"); v != "" {
		params["obfs"] = v
	}
	if v := q.Get("upmbps"); v != "" {
		params["up"] = v
	}
	if v := q.Get("downmbps"); v != "" {
		params["down"] = v
	}
	if v := q.Get("obfs"); v != "" {
		params["obfs"] = v
	}
	if v := q.Get("sni"); v != "" {
		params["sni"] = v
	} else if v := q.Get("peer"); v != "" {
		params["sni"] = v
	}
	if v := q.Get("alpn"); v != "" {
		params["alpn"] = splitCSV(v)
	}
	if _, ok := q["insecure"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("insecure"))
	}
	if v := q.Get("protocol"); v != "" {
		params["protocol"] = v
	}
	if _, ok := q["fast-open"]; ok {
		params["fast-open"] = parseBoolPresence(q.Get("fast-open"))
	}
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "hysteria", Name: defaultName(decodedName, "Hysteria", host, port), Host: host, Port: port, Params: params}, nil
}

func parseTUIC(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "tuic://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	at := strings.LastIndex(main, "@")
	if at < 0 {
		return nil, errors.New("tuic 缺少 @")
	}
	auth := main[:at]
	host, port, err := splitHostPort(main[at+1:])
	if err != nil {
		return nil, err
	}
	ci := strings.Index(auth, ":")
	if ci < 0 {
		return nil, errors.New("tuic userinfo 缺少分隔符")
	}
	params := map[string]any{"uuid": auth[:ci], "password": urlQueryUnescapeOrRaw(auth[ci+1:])}
	q := query
	if v := q.Get("token"); v != "" {
		params["token"] = v
	}
	if v := q.Get("ip"); v != "" {
		params["ip"] = v
	}
	if v := q.Get("sni"); v != "" {
		params["sni"] = v
	}
	if v := q.Get("alpn"); v != "" {
		params["alpn"] = splitCSV(v)
	}
	if v := q.Get("heartbeat-interval"); v != "" {
		params["heartbeat-interval"] = parseIntString(v)
	}
	if v := q.Get("request-timeout"); v != "" {
		params["request-timeout"] = parseIntString(v)
	}
	if v := q.Get("udp-relay-mode"); v != "" {
		params["udp-relay-mode"] = v
	}
	if v := q.Get("congestion-controller"); v != "" {
		params["congestion-controller"] = v
	}
	if v := q.Get("max-udp-relay-packet-size"); v != "" {
		params["max-udp-relay-packet-size"] = parseIntString(v)
	}
	if v := q.Get("max-open-streams"); v != "" {
		params["max-open-streams"] = parseIntString(v)
	}
	for _, key := range []string{"disable-sni", "reduce-rtt", "fast-open"} {
		if _, ok := q[key]; ok {
			params[key] = parseBoolPresence(q.Get(key))
		}
	}
	if _, ok := q["allow-insecure"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("allow-insecure"))
	} else if _, ok := q["skip-cert-verify"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("skip-cert-verify"))
	}
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "tuic", Name: defaultName(decodedName, "TUIC", host, port), Host: host, Port: port, Params: params}, nil
}

func parseWireGuard(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "wireguard://")
	after = strings.TrimPrefix(after, "wg://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	at := strings.LastIndex(main, "@")
	if at < 0 {
		return nil, errors.New("wireguard 缺少 @")
	}
	privateKey := main[:at]
	host, port, err := splitHostPort(main[at+1:])
	if err != nil {
		return nil, err
	}
	params := map[string]any{"private-key": urlQueryUnescapeOrRaw(privateKey), "udp": true}
	q := query
	if v := q.Get("address"); v != "" {
		setWireGuardIPs(params, v)
	} else if v := q.Get("ip"); v != "" {
		setWireGuardIPs(params, v)
	}
	if v := q.Get("public-key"); v != "" {
		params["public-key"] = v
	} else if v := q.Get("publickey"); v != "" {
		params["public-key"] = v
	}
	if v := q.Get("allowed-ips"); v != "" {
		params["allowed-ips"] = splitCSV(v)
	}
	if v := q.Get("pre-shared-key"); v != "" {
		params["pre-shared-key"] = v
	}
	if v := q.Get("reserved"); v != "" {
		parts := strings.Split(v, ",")
		out := make([]int, 0, 3)
		for _, p := range parts {
			if n, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				out = append(out, n)
			}
		}
		if len(out) == 3 {
			params["reserved"] = out
		}
	}
	if v := q.Get("udp"); v != "" {
		params["udp"] = parseBoolPresence(v)
	}
	if v := q.Get("mtu"); v != "" {
		params["mtu"] = parseIntString(v)
	}
	if v := q.Get("dialer-proxy"); v != "" {
		params["dialer-proxy"] = v
	}
	if v := q.Get("dns"); v != "" {
		params["dns"] = splitCSV(v)
	}
	if _, ok := q["remote-dns-resolve"]; ok {
		params["remote-dns-resolve"] = parseBoolPresence(q.Get("remote-dns-resolve"))
	}
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "wireguard", Name: defaultName(decodedName, "WireGuard", host, port), Host: host, Port: port, Params: params}, nil
}

func setWireGuardIPs(params map[string]any, raw string) {
	for _, item := range strings.Split(raw, ",") {
		ip := strings.TrimSpace(item)
		ip = strings.TrimPrefix(ip, "[")
		ip = strings.TrimSuffix(ip, "]")
		if idx := strings.Index(ip, "/"); idx >= 0 {
			ip = ip[:idx]
		}
		if strings.Contains(ip, ":") {
			params["ipv6"] = ip
		} else if ip != "" {
			params["ip"] = ip
		}
	}
}

func parseHTTP(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "http://")
	after = strings.TrimPrefix(after, "https://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	at := strings.LastIndex(main, "@")
	var auth string
	hostport := main
	if at >= 0 {
		auth = main[:at]
		hostport = main[at+1:]
	}
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	params := map[string]any{}
	if at >= 0 {
		if ci := strings.Index(auth, ":"); ci >= 0 {
			params["username"] = urlQueryUnescapeOrRaw(auth[:ci])
			params["password"] = urlQueryUnescapeOrRaw(auth[ci+1:])
		}
	}
	if strings.HasPrefix(strings.ToLower(s), "https://") {
		params["tls"] = true
	}
	q := query
	if _, ok := q["tls"]; ok {
		params["tls"] = parseBoolPresence(q.Get("tls"))
	}
	if v := q.Get("sni"); v != "" {
		params["sni"] = v
	}
	if _, ok := q["skip-cert-verify"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("skip-cert-verify"))
	}
	if v := q.Get("fingerprint"); v != "" {
		params["fingerprint"] = v
	}
	if v := q.Get("ip-version"); v != "" {
		params["ip-version"] = v
	}
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "http", Name: defaultName(decodedName, "HTTP", host, port), Host: host, Port: port, Params: params}, nil
}

func parseSocks5(s string) (*Result, error) {
	after := strings.TrimPrefix(s, "socks5://")
	after = strings.TrimPrefix(after, "socks://")
	rest, name := splitFragment(after)
	main, query := splitQuery(rest)
	at := strings.LastIndex(main, "@")
	var auth string
	hostport := main
	if at >= 0 {
		auth = main[:at]
		hostport = main[at+1:]
	}
	host, port, err := splitHostPort(hostport)
	if err != nil {
		return nil, err
	}
	params := map[string]any{}
	if at >= 0 {
		if ci := strings.Index(auth, ":"); ci >= 0 {
			params["username"] = urlQueryUnescapeOrRaw(auth[:ci])
			params["password"] = urlQueryUnescapeOrRaw(auth[ci+1:])
		}
	}
	q := query
	if _, ok := q["tls"]; ok {
		params["tls"] = parseBoolPresence(q.Get("tls"))
	}
	if _, ok := q["udp"]; ok {
		params["udp"] = parseBoolPresence(q.Get("udp"))
	}
	if _, ok := q["skip-cert-verify"]; ok {
		params["skip-cert-verify"] = parseBoolPresence(q.Get("skip-cert-verify"))
	}
	if v := q.Get("fingerprint"); v != "" {
		params["fingerprint"] = v
	}
	if v := q.Get("ip-version"); v != "" {
		params["ip-version"] = v
	}
	decodedName, _ := url.QueryUnescape(name)
	return &Result{Protocol: "socks5", Name: defaultName(decodedName, "SOCKS5", host, port), Host: host, Port: port, Params: params}, nil
}

// --- 通用 helper ---

func splitFragment(s string) (string, string) {
	if idx := strings.Index(s, "#"); idx >= 0 {
		return s[:idx], s[idx+1:]
	}
	return s, ""
}

func splitQuery(s string) (string, url.Values) {
	if idx := strings.Index(s, "?"); idx >= 0 {
		q, _ := url.ParseQuery(s[idx+1:])
		return s[:idx], q
	}
	return s, url.Values{}
}

func parseQuery(raw string) url.Values {
	if raw == "" {
		return url.Values{}
	}
	q, _ := url.ParseQuery(raw)
	return q
}

func splitHostPort(hostport string) (string, int, error) {
	hostport = strings.TrimSpace(hostport)
	if strings.HasPrefix(hostport, "[") {
		end := strings.Index(hostport, "]")
		if end < 0 {
			return "", 0, errors.New("IPv6 地址缺少右括号")
		}
		host := hostport[1:end]
		rest := hostport[end+1:]
		if strings.HasPrefix(rest, ":") {
			port, err := parsePort(rest[1:])
			if err != nil {
				return "", 0, err
			}
			return host, port, nil
		}
		return host, 443, nil
	}
	idx := strings.LastIndex(hostport, ":")
	if idx < 0 {
		return hostport, 443, nil
	}
	port, err := parsePort(hostport[idx+1:])
	if err != nil {
		return "", 0, err
	}
	host := hostport[:idx]
	if strings.HasPrefix(host, "[") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	return host, port, nil
}

func parsePort(raw string) (int, error) {
	p, err := strconv.Atoi(raw)
	if err != nil || p < 1 || p > 65535 {
		return 0, errors.New("端口无效")
	}
	return p, nil
}

func decodeBase64OrOriginal(s string) string {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := enc.DecodeString(s); err == nil {
			return string(decoded)
		}
	}
	return s
}

func defaultName(name, prefix, host string, port int) string {
	if name != "" {
		return name
	}
	return fmt.Sprintf("%s %s:%d", prefix, host, port)
}

func parseBoolPresence(v string) bool {
	if v == "" {
		return true
	}
	return strings.EqualFold(v, "1") || strings.EqualFold(v, "true")
}

func parseIntString(v string) int {
	n, _ := strconv.Atoi(v)
	return n
}

func parseIntAny(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	case string:
		return parseIntString(x)
	default:
		return 0
	}
}

func intAny(v any) int {
	return parseIntAny(v)
}

func strAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprint(v)
	}
}

func boolOrString(v any) any {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return parseBoolPresence(x)
	default:
		return x
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeNetwork(v string) string {
	switch strings.ToLower(v) {
	case "websocket":
		return "ws"
	case "sw":
		return "ws"
	case "httpupgrade":
		return "ws"
	default:
		return strings.ToLower(v)
	}
}

func firstNonEmpty(vals ...any) string {
	for _, v := range vals {
		if s := strAny(v); s != "" {
			return s
		}
	}
	return ""
}

func urlQueryUnescapeOrRaw(s string) string {
	decoded, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}
