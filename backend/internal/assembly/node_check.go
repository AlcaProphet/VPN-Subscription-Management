package assembly

import (
	"context"
	"fmt"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	assemblylinks "vpn-sub/internal/assembly/links"
	"vpn-sub/internal/node"
	"vpn-sub/internal/ssplugin"
)

// CheckNodeTarget 使用旧签名调用实际输出适配器（兼容既有测试与调用方）。
func (s *Service) CheckNodeTarget(ctx context.Context, target, protocol, renderName, host string, port int, params map[string]any) (node.CheckRenderResult, error) {
	return s.CheckNodeTargetDraft(ctx, node.CheckTargetDraft{
		Target: target, Protocol: protocol, RenderName: renderName, Host: host, Port: port, Params: params,
	})
}

// CheckNodeTargetDraft 使用服务端注入的完整草稿检查单个节点目标；该方法只构造内存产物。
func (s *Service) CheckNodeTargetDraft(ctx context.Context, in node.CheckTargetDraft) (node.CheckRenderResult, error) {
	if err := ctx.Err(); err != nil {
		return node.CheckRenderResult{}, err
	}
	switch in.Target {
	case "clash-yaml":
		return s.checkClashNodeTarget(in.Protocol, in.RenderName, in.Host, in.Port, in.Params, in.NodeID, in.Persisted, in.State)
	case "sr-subs", "generic-subs":
		return checkLinkNodeTarget(in.Target, in.Protocol, in.RenderName, in.Host, in.Port, in.Params)
	default:
		return node.CheckRenderResult{}, fmt.Errorf("节点检查不支持目标: %s", in.Target)
	}
}

func (s *Service) checkClashNodeTarget(protocol, renderName, host string, port int, params map[string]any, nodeID int64, persisted bool, state node.CurrentState) (node.CheckRenderResult, error) {
	diagnostics := diagnoseSSPluginForTarget("clash-yaml", protocol, params)
	if hasBlockingTargetDiagnostic(diagnostics) {
		return node.CheckRenderResult{Diagnostics: diagnostics}, nil
	}
	nd := &nodeData{
		NodeID:       nodeID,
		Protocol:     protocol,
		RenderName:   renderName,
		Host:         host,
		Port:         port,
		ProtocolJSON: params,
		CurrentState: state,
	}
	proxy, adapterDiagnostics, err := s.buildClashProxy(nd, persisted)
	if err != nil {
		return node.CheckRenderResult{}, fmt.Errorf("构造 Clash 节点检查片段失败: %w", err)
	}
	diagnostics = append(diagnostics, adapterDiagnostics...)
	root := gyaml.MapSlice{
		{Key: "proxies", Value: []any{orderedMapToMapSlice(proxy)}},
		{Key: "rules", Value: []any{"GEOIP,CN,DIRECT", "MATCH,DIRECT"}},
	}
	content, err := marshalClashYAML(root, nil)
	if err != nil {
		return node.CheckRenderResult{}, fmt.Errorf("序列化 Clash 节点检查片段失败: %w", err)
	}
	// 自检针对真实产物；预览是构造完成之后的脱敏副本，两者结构完全一致。
	issues := CheckClashContent(content)
	for _, issue := range issues {
		severity := issue.Severity
		if severity != "info" && severity != "warn" && severity != "error" {
			severity = "warn"
		}
		code := "clash_output_warning"
		if severity == "error" {
			code = "clash_output_invalid"
			if strings.Contains(issue.Message, "不支持的节点类型") {
				code = "core_semantic_unexpressible"
			}
		}
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity:  severity,
			Code:      code,
			Target:    "clash-yaml",
			FieldPath: canonicalClashIssuePath(protocol, params, issue.Path),
			Message:   issue.Message,
			Evidence:  "mihomo-1.19.31-yaml",
		})
	}
	if protocol == "trojan" {
		network, _ := params["network"].(string)
		if network == "" {
			network = "tcp"
		}
		if network != "tcp" && network != "ws" && network != "grpc" {
			diagnostics = append(diagnostics, node.TargetDiagnostic{
				Severity:  "warn",
				Code:      "trojan_transport_fallback",
				Target:    "clash-yaml",
				FieldPath: "network",
				Message:   "Trojan 自定义传输不作为普通组合；目标内核可能按 TCP 处理或静默回退",
				Evidence:  "mihomo-1.19.31-yaml",
			})
		}
	}
	previewContent, err := marshalClashYAML(gyaml.MapSlice{
		{Key: "proxies", Value: []any{orderedMapToMapSlice(redactClashProxy(protocol, params, proxy))}},
		{Key: "rules", Value: []any{"GEOIP,CN,DIRECT", "MATCH,DIRECT"}},
	}, nil)
	if err != nil {
		return node.CheckRenderResult{}, fmt.Errorf("序列化 Clash 节点检查预览失败: %w", err)
	}
	return node.CheckRenderResult{Preview: string(previewContent), Diagnostics: diagnostics}, nil
}

// redactClashProxy 在 Clash 条目构造完成之后按具体敏感值替换字段叶子。
// 该函数只产出预览副本，不参与自检、正式装配或持久化。
func redactClashProxy(protocol string, params map[string]any, proxy *OrderedMap) *OrderedMap {
	proto, err := node.GetProtocol(protocol)
	if err != nil {
		return proxy
	}
	secrets := node.SensitiveValues(proto, params)
	if len(secrets) == 0 {
		return proxy
	}
	set := make(map[string]bool, len(secrets))
	for _, secret := range secrets {
		set[secret] = true
	}
	out := NewOrderedMap()
	for _, key := range proxy.Keys() {
		value, _ := proxy.Get(key)
		out.Set(key, redactClashValue(value, set))
	}
	return out
}

// redactClashValue 递归替换与具体敏感值完全相同的字符串叶子（不改动结构与其它字段）。
func redactClashValue(value any, secrets map[string]bool) any {
	switch typed := value.(type) {
	case string:
		if secrets[typed] {
			return "REDACTED"
		}
		return typed
	case []string:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactClashValue(item, secrets)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = redactClashValue(item, secrets)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = redactClashValue(item, secrets)
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = redactClashValue(item, secrets)
		}
		return out
	default:
		return value
	}
}

// canonicalClashIssuePath 把自检产物的 YAML 路径映射回协议 schema 的规范字段路径，
// 使 target evidence 的 field_path 可以回指 schema，而不是只给出笼统标签。
func canonicalClashIssuePath(protocol string, params map[string]any, yamlPath string) string {
	remainder := yamlPath
	if strings.HasPrefix(remainder, "$.proxies[") {
		if close := strings.Index(remainder, "]"); close >= 0 {
			remainder = strings.TrimPrefix(remainder[close+1:], ".")
		}
	}
	if remainder == "" || strings.HasPrefix(remainder, "$") || strings.HasPrefix(remainder, "proxies") {
		return ""
	}
	proto, err := node.GetProtocol(protocol)
	if err != nil {
		return ""
	}
	if protocol == "ss" && strings.HasPrefix(remainder, "plugin-opts.") {
		// SS 已知插件的真实存储对象是 schema 的规范路径；未知插件保持开放 Map 路径。
		plugin, _ := params["plugin"].(string)
		if definition, known := ssplugin.Lookup(plugin); known {
			remainder = definition.StorageKey + strings.TrimPrefix(remainder, "plugin-opts")
		}
	}
	if !node.SchemaPathExists(proto, remainder) {
		return ""
	}
	return remainder
}

func checkLinkNodeTarget(target, protocol, renderName, host string, port int, params map[string]any) (node.CheckRenderResult, error) {
	if !assemblylinks.SupportsURI(protocol) {
		return node.CheckRenderResult{
			Status: "skip",
			Diagnostics: []node.TargetDiagnostic{{
				Severity: "error", Code: "target_unsupported", Target: target, FieldPath: "protocol",
				Message:  fmt.Sprintf("协议 %s 没有 %s URI 映射，目标不可用", protocol, target),
				Evidence: "build32-uri-registry",
			}},
		}, nil
	}
	diagnostics := linkTargetDiagnostics(target, protocol, params)
	diagnostics = append(diagnostics, diagnoseSSPluginForTarget(target, protocol, params)...)
	if hasBlockingTargetDiagnostic(diagnostics) {
		return node.CheckRenderResult{Diagnostics: diagnostics}, nil
	}
	generic := target == "generic-subs"
	link, err := RenderLink(protocol, renderName, host, port, params, generic)
	if err != nil {
		return node.CheckRenderResult{Diagnostics: diagnostics}, fmt.Errorf("生成 %s 节点链接失败: %w", target, err)
	}
	return node.CheckRenderResult{Preview: redactLinkPreview(protocol, params, link, renderName, host, port, generic), Diagnostics: diagnostics}, nil
}

// redactLinkPreview 在真实链接构造完成之后，以同一构造器和敏感字段占位副本重建预览。
// 这样 URL 转义与 Base64 等编码仍由正式构造器负责，同时不会在整条链接中误替换
// 与短凭据碰巧相同的 host、节点名或普通参数片段。对照构造失败时不返回预览。
func redactLinkPreview(protocol string, params map[string]any, link, renderName, host string, port int, generic bool) string {
	proto, err := node.GetProtocol(protocol)
	if err != nil {
		return link
	}
	if len(node.SensitiveValues(proto, params)) == 0 {
		return link
	}
	redactedLink, renderErr := RenderLink(protocol, renderName, host, port, node.RedactSensitiveParams(proto, params), generic)
	if renderErr != nil {
		return ""
	}
	return redactedLink
}

func linkTargetDiagnostics(target, protocol string, params map[string]any) []node.TargetDiagnostic {
	add := func(out *[]node.TargetDiagnostic, severity, code, path, message, evidence string) {
		*out = append(*out, node.TargetDiagnostic{
			Severity: severity, Code: code, Target: target, FieldPath: path, Message: message, Evidence: evidence,
		})
	}
	var diagnostics []node.TargetDiagnostic
	switch protocol {
	case "vless":
		if encryption, _ := params["encryption"].(string); encryption != "" && encryption != "none" {
			add(&diagnostics, "error", "core_semantic_unexpressible", "encryption",
				"当前 URI 适配器不会保留 VLESS 非 none 的 encryption 语义", "cvr-2.5.2-uri")
		}
		// REALITY 分支不携带证书校验开关，普通 TLS 分支两个目标都能表达。
		if hasConfiguredObject(params, "reality-opts") && hasActiveParam(params, "skip-cert-verify") {
			add(&diagnostics, "warn", "uri_partial_fields", "skip-cert-verify",
				"REALITY 分支的 URI 不携带证书校验开关，导入后需复核", "project-uri-capability")
		}
		addURIPartialFields(&diagnostics, target, params,
			"当前 URI 适配器不表达该字段，导入后连接行为可能不同",
			"packet-addr", "xudp", "packet-encoding", "smux")
		if path := linkDroppedHeaderPath(params); path != "" {
			add(&diagnostics, "warn", "uri_partial_fields", path,
				"当前 URI 适配器只携带 Host 请求头，其余自定义请求头会被丢弃", "project-uri-capability")
		}
	case "vmess":
		switch cipher, _ := params["cipher"].(string); cipher {
		case "chacha20-poly1305":
			add(&diagnostics, "warn", "uri_algorithm_rewrite", "cipher",
				"CVR 2.5.2 URI 入口对 chacha20-poly1305 可能发生算法改写，导入后需复核", "cvr-2.5.2-uri")
		case "zero":
			add(&diagnostics, "warn", "uri_algorithm_rewrite", "cipher",
				"CVR 2.5.2 URI 入口对 zero 可能回退或改写，导入后需复核", "cvr-2.5.2-uri")
		}
		// SR 链接以 allowInsecure 表达跳过证书校验，通用 vmess:// JSON 不携带该开关。
		if target == "generic-subs" && hasActiveParam(params, "skip-cert-verify") {
			add(&diagnostics, "warn", "uri_partial_fields", "skip-cert-verify",
				"当前通用 URI 适配器不携带证书校验开关，导入后需复核", "project-uri-capability")
		}
		// 两个目标都不表达 REALITY，静默降级为普通 TLS 会改变连接语义。
		if hasConfiguredObject(params, "reality-opts") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "reality-opts",
				"当前 URI 适配器不能表达 VMess REALITY 语义", "project-uri-capability")
		}
		addURIPartialFields(&diagnostics, target, params,
			"当前 URI 适配器不表达该字段，导入后连接行为可能不同",
			"packet-addr", "xudp", "packet-encoding", "global-padding", "authenticated-length", "smux")
		if path := linkDroppedHeaderPath(params); path != "" {
			add(&diagnostics, "warn", "uri_partial_fields", path,
				"当前 URI 适配器只携带 Host 请求头，其余自定义请求头会被丢弃", "project-uri-capability")
		}
	case "trojan":
		network, _ := params["network"].(string)
		if network == "" {
			network = "tcp"
		}
		if network != "tcp" {
			add(&diagnostics, "error", "core_semantic_unexpressible", "network",
				fmt.Sprintf("当前 %s URI 适配器不能表达 Trojan %s 传输参数", target, network), "cvr-2.5.2-uri")
		}
		if opts, ok := params["ss-opts"].(map[string]any); ok && boolValue(opts["enabled"]) {
			add(&diagnostics, "error", "core_semantic_unexpressible", "ss-opts",
				"当前 URI 适配器不能表达 Trojan 内层 SS 参数", "cvr-2.5.2-uri")
		}
		if hasConfiguredObject(params, "reality-opts") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "reality-opts",
				"当前 URI 适配器不能表达 Trojan REALITY 语义", "project-uri-capability")
		}
		if udp, ok := params["udp"].(bool); ok && !udp {
			add(&diagnostics, "warn", "uri_partial_fields", "udp",
				"当前 URI 适配器不表达显式关闭的 UDP，导入后可能重新启用 UDP", "project-uri-capability")
		}
		addURIPartialFields(&diagnostics, target, params,
			"当前 URI 适配器不表达该字段，导入后连接行为可能不同",
			"client-fingerprint")
		if path := linkDroppedHeaderPath(params); path != "" {
			add(&diagnostics, "warn", "uri_partial_fields", path,
				"当前 URI 适配器只携带 Host 请求头，其余自定义请求头会被丢弃", "project-uri-capability")
		}
	case "ss":
		cipher, _ := params["cipher"].(string)
		if strings.HasPrefix(cipher, "2022-") {
			add(&diagnostics, "warn", "unverified_compatibility", "cipher",
				"SS 2022 当前仅登记为待验证兼容项，未宣称完整 URI 支持", "cvr-2.5.2-uri")
		}
		// SIP002 链接不携带 UDP 开关与 UDP over TCP 参数。
		if udp, ok := params["udp"].(bool); ok && !udp {
			add(&diagnostics, "warn", "uri_partial_fields", "udp",
				"当前 URI 适配器不表达显式关闭的 UDP，导入后可能重新启用 UDP", "project-uri-capability")
		}
		addURIPartialFields(&diagnostics, target, params,
			"当前 URI 适配器不表达该传输／调优字段，导入后连接行为可能不同",
			"udp-over-tcp", "udp-over-tcp-version", "client-fingerprint")
		if hasEnabledObject(params, "smux") {
			add(&diagnostics, "warn", "uri_partial_fields", "smux",
				"当前 URI 适配器不携带多路复用参数，导入后连接行为可能不同", "project-uri-capability")
		}
	case "http":
		// URI 只携带代理地址、认证与 TLS 开关；mTLS 与自定义请求头不可表达。
		if hasTextParamValue(params, "certificate") || hasTextParamValue(params, "private-key") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "certificate",
				"当前 URI 适配器不能表达 HTTP 客户端证书（mTLS）参数", "cvr-2.5.2-uri")
		}
		if headers, ok := params["headers"].(map[string]any); ok && len(headers) > 0 {
			add(&diagnostics, "warn", "uri_partial_fields", "headers",
				"当前 URI 适配器不会携带自定义请求头，导入后连接行为可能不同", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"name-cert-verify", "fingerprint"} {
			if hasTextParamValue(params, path) {
				add(&diagnostics, "warn", "unverified_compatibility", path,
					"当前 URI 适配器不表达该证书校验参数，导入后需复核", "cvr-2.5.2-uri")
			}
		}
	case "socks5":
		// SOCKS5 URI 可表达认证、TLS 开关与 UDP，但不能表达 mTLS 与证书校验参数。
		if hasTextParamValue(params, "certificate") || hasTextParamValue(params, "private-key") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "certificate",
				"当前 URI 适配器不能表达 SOCKS5 客户端证书（mTLS）参数", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"name-cert-verify", "fingerprint"} {
			if hasTextParamValue(params, path) {
				add(&diagnostics, "warn", "unverified_compatibility", path,
					"当前 URI 适配器不表达该证书校验参数，导入后需复核", "cvr-2.5.2-uri")
			}
		}
	case "hysteria":
		// URI 只能表达 Base64 auth、端口跳跃与 TLS 开关，不能表达 auth-str、mTLS 与高级调优。
		if hasTextParamValue(params, "auth-str") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "auth-str",
				"当前 URI 适配器只能表达 Base64 auth，不能表达 auth-str 认证", "cvr-2.5.2-uri")
		}
		if hasTextParamValue(params, "certificate") || hasTextParamValue(params, "private-key") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "certificate",
				"当前 URI 适配器不能表达 Hysteria 客户端证书（mTLS）参数", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"name-cert-verify", "fingerprint"} {
			if hasTextParamValue(params, path) {
				add(&diagnostics, "warn", "unverified_compatibility", path,
					"当前 URI 适配器不表达该证书校验参数，导入后需复核", "cvr-2.5.2-uri")
			}
		}
		if ech, ok := params["ech-opts"].(map[string]any); ok && boolValue(ech["enable"]) {
			add(&diagnostics, "warn", "uri_partial_fields", "ech-opts",
				"当前 URI 适配器不携带 ECH 参数，导入后需复核", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"recv-window-conn", "recv-window", "disable-mtu-discovery", "fast-open", "hop-interval"} {
			if hasActiveParam(params, path) {
				add(&diagnostics, "warn", "uri_partial_fields", path,
					"当前 URI 适配器不表达该高级调优字段，导入后连接行为可能不同", "cvr-2.5.2-uri")
			}
		}
	case "hysteria2":
		// URI 只能表达单端口、认证、obfs 与 TLS 开关；端口组、Realm、mTLS 与高级调优不可表达。
		if hasTextParamValue(params, "ports") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "ports",
				"当前 URI 适配器不能表达 Hysteria2 端口组（端口跳跃）语义", "cvr-2.5.2-uri")
		}
		if realm, ok := params["realm-opts"].(map[string]any); ok && boolValue(realm["enable"]) {
			add(&diagnostics, "error", "core_semantic_unexpressible", "realm-opts",
				"当前 URI 适配器不能表达 Hysteria2 Realm 服务发现语义", "cvr-2.5.2-uri")
		}
		if hasTextParamValue(params, "certificate") || hasTextParamValue(params, "private-key") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "certificate",
				"当前 URI 适配器不能表达 Hysteria2 客户端证书（mTLS）参数", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"name-cert-verify", "fingerprint"} {
			if hasTextParamValue(params, path) {
				add(&diagnostics, "warn", "unverified_compatibility", path,
					"当前 URI 适配器不表达该证书校验参数，导入后需复核", "cvr-2.5.2-uri")
			}
		}
		if ech, ok := params["ech-opts"].(map[string]any); ok && boolValue(ech["enable"]) {
			add(&diagnostics, "warn", "uri_partial_fields", "ech-opts",
				"当前 URI 适配器不携带 ECH 参数，导入后需复核", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"up", "down", "obfs-min-packet-size", "obfs-max-packet-size",
			"cwnd", "bbr-profile", "udp-mtu", "handshake-timeout",
			"initial-stream-receive-window", "max-stream-receive-window",
			"initial-connection-receive-window", "max-connection-receive-window"} {
			if hasActiveParam(params, path) {
				add(&diagnostics, "warn", "uri_partial_fields", path,
					"当前 URI 适配器不表达该高级调优字段，导入后连接行为可能不同", "cvr-2.5.2-uri")
			}
		}
	case "tuic":
		// URI 只能表达 v5 的 UUID／密码、SNI、ALPN 与证书校验开关。
		if hasTextParamValue(params, "token") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "token",
				"当前 URI 适配器只表达 TUIC v5 的 UUID／密码，不能表达 v4 Token 认证", "cvr-2.5.2-uri")
		}
		if hasTextParamValue(params, "certificate") || hasTextParamValue(params, "private-key") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "certificate",
				"当前 URI 适配器不能表达 TUIC 客户端证书（mTLS）参数", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"name-cert-verify", "fingerprint"} {
			if hasTextParamValue(params, path) {
				add(&diagnostics, "warn", "unverified_compatibility", path,
					"当前 URI 适配器不表达该证书校验参数，导入后需复核", "cvr-2.5.2-uri")
			}
		}
		if ech, ok := params["ech-opts"].(map[string]any); ok && boolValue(ech["enable"]) {
			add(&diagnostics, "warn", "uri_partial_fields", "ech-opts",
				"当前 URI 适配器不携带 ECH 参数，导入后需复核", "cvr-2.5.2-uri")
		}
		for _, path := range []string{"ip", "request-timeout", "heartbeat-interval", "udp-relay-mode",
			"congestion-controller", "disable-sni", "max-udp-relay-packet-size", "reduce-rtt",
			"fast-open", "max-open-streams", "cwnd", "bbr-profile", "recv-window-conn", "recv-window",
			"disable-mtu-discovery", "max-datagram-frame-size", "udp-over-stream", "udp-over-stream-version"} {
			if hasActiveParam(params, path) {
				add(&diagnostics, "warn", "uri_partial_fields", path,
					"当前 URI 适配器不表达该高级调优字段，导入后连接行为可能不同", "cvr-2.5.2-uri")
			}
		}
	case "anytls":
		// URI 只表达密码、SNI、ALPN、客户端指纹与证书校验开关；
		// 活动但不可表达的字段必须给出明确诊断，不得静默丢弃后仍标 complete。
		if hasTextParamValue(params, "certificate") || hasTextParamValue(params, "private-key") {
			add(&diagnostics, "error", "core_semantic_unexpressible", "certificate",
				"当前 URI 适配器不能表达 AnyTLS 客户端证书（mTLS）参数", "project-uri-anytls")
		}
		for _, path := range []string{"shadow-tls-opts", "restls-opts", "jls-opts"} {
			if options, ok := params[path].(map[string]any); ok && len(options) > 0 {
				add(&diagnostics, "error", "core_semantic_unexpressible", path,
					"当前 URI 适配器不能表达 AnyTLS 附加伪装安全对象", "project-uri-anytls")
			}
		}
		for _, path := range []string{"name-cert-verify", "fingerprint"} {
			if hasTextParamValue(params, path) {
				add(&diagnostics, "warn", "unverified_compatibility", path,
					"当前 URI 适配器不表达该证书校验参数，导入后需复核", "project-uri-anytls")
			}
		}
		if ech, ok := params["ech-opts"].(map[string]any); ok && boolValue(ech["enable"]) {
			add(&diagnostics, "warn", "uri_partial_fields", "ech-opts",
				"当前 URI 适配器不携带 ECH 参数，导入后需复核", "project-uri-anytls")
		}
		for _, path := range []string{"client-metadata", "idle-session-check-interval", "idle-session-timeout",
			"min-idle-session", "disable-reuse"} {
			if hasActiveParam(params, path) {
				add(&diagnostics, "warn", "uri_partial_fields", path,
					"当前 URI 适配器不表达该会话调优字段，导入后连接行为可能不同", "project-uri-anytls")
			}
		}
	case "wireguard":
		// 项目 wireguard:// 只表达单 endpoint 的可无损回读子集；多 Peer 结构无法表达。
		if peers, ok := params["peers"].([]any); ok && len(peers) > 0 {
			add(&diagnostics, "error", "core_semantic_unexpressible", "peers",
				"当前 URI 适配器不能表达 WireGuard 多 Peer 结构", "project-uri-wireguard")
		}
		for _, path := range []string{"ip-stack", "workers", "persistent-keepalive", "refresh-server-ip-interval"} {
			if hasActiveParam(params, path) {
				add(&diagnostics, "warn", "uri_partial_fields", path,
					"当前 URI 适配器不表达该高级调优字段，导入后连接行为可能不同", "project-uri-wireguard")
			}
		}
	}
	addBasicOptionURIDiagnostics(&diagnostics, target, protocol, params)
	return diagnostics
}

// hasActiveParam 判断调优字段是否设置了非零／非 false 值。
func hasActiveParam(params map[string]any, key string) bool {
	value, ok := params[key]
	if !ok {
		return false
	}
	return paramValueActive(value)
}

// paramValueActive 判断单个参数值是否处于活动状态（非空、非 false、非零）。
func paramValueActive(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case string:
		return strings.TrimSpace(typed) != ""
	case int:
		return typed != 0
	case int32:
		return typed != 0
	case int64:
		return typed != 0
	case float32:
		return typed != 0
	case float64:
		return typed != 0
	case map[string]any:
		return len(typed) > 0
	case []any:
		return len(typed) > 0
	default:
		return true
	}
}

// hasConfiguredObject 判断对象字段是否存在且至少有一个有效子值。
func hasConfiguredObject(params map[string]any, key string) bool {
	object, ok := params[key].(map[string]any)
	if !ok {
		return false
	}
	for _, value := range object {
		if paramValueActive(value) {
			return true
		}
	}
	return false
}

// hasEnabledObject 判断带 enabled 开关的对象字段是否真的启用。
func hasEnabledObject(params map[string]any, key string) bool {
	object, ok := params[key].(map[string]any)
	if !ok {
		return false
	}
	return boolValue(object["enabled"])
}

// linkDroppedHeaderPath 返回因 URI 只表达 Host 而被丢弃的请求头 schema 路径；
// 没有额外请求头时返回空串。
func linkDroppedHeaderPath(params map[string]any) string {
	network, _ := params["network"].(string)
	var path string
	switch network {
	case "ws":
		path = "ws-opts"
	case "http":
		path = "http-opts"
	default:
		return ""
	}
	opts, ok := params[path].(map[string]any)
	if !ok {
		return ""
	}
	headers, ok := opts["headers"].(map[string]any)
	if !ok || len(headers) == 0 {
		return ""
	}
	for key, value := range headers {
		if strings.EqualFold(key, "host") {
			continue
		}
		if paramValueActive(value) {
			return path + ".headers"
		}
	}
	return ""
}

// addBasicOptionURIDiagnostics 报告 URI 不表达的内核公共字段。
// tfo 只在 SR 的 vmess／vless 链接中可表达，其余目标与字段一律按非阻断丢失告警。
func addBasicOptionURIDiagnostics(out *[]node.TargetDiagnostic, target, protocol string, params map[string]any) {
	tfoExpressed := target == "sr-subs" && (protocol == "vmess" || protocol == "vless")
	for _, path := range []string{"tfo", "mptcp", "interface-name", "routing-mark", "ip-version", "dialer-proxy"} {
		if path == "tfo" && tfoExpressed {
			continue
		}
		if !hasActiveParam(params, path) {
			continue
		}
		*out = append(*out, node.TargetDiagnostic{
			Severity: "warn", Code: "uri_partial_fields", Target: target, FieldPath: path,
			Message:  "当前 URI 适配器不表达该内核公共字段，导入后连接行为可能不同",
			Evidence: "project-uri-common",
		})
	}
}

// addURIPartialFields 批量报告非阻断丢失的字段。
func addURIPartialFields(out *[]node.TargetDiagnostic, target string, params map[string]any, message string, paths ...string) {
	for _, path := range paths {
		if !hasActiveParam(params, path) {
			continue
		}
		*out = append(*out, node.TargetDiagnostic{
			Severity: "warn", Code: "uri_partial_fields", Target: target, FieldPath: path,
			Message:  message,
			Evidence: "project-uri-capability",
		})
	}
}

// hasTextParamValue 判断协议参数中的字符串字段是否已配置非空值。
func hasTextParamValue(params map[string]any, key string) bool {
	value, _ := params[key].(string)
	return strings.TrimSpace(value) != ""
}

// diagnoseSSPluginForTarget 把叶子合同诊断投影为节点检查的公共响应类型。
func diagnoseSSPluginForTarget(target, protocol string, params map[string]any) []node.TargetDiagnostic {
	if protocol != "ss" {
		return nil
	}
	plugin, _ := params["plugin"].(string)
	issues := ssplugin.AssessTarget(plugin, params, target)
	diagnostics := make([]node.TargetDiagnostic, 0, len(issues))
	for _, issue := range issues {
		evidence := "cvr-2.5.2-uri"
		if target == ssplugin.TargetClash {
			evidence = "mihomo-1.19.31-yaml"
		} else if issue.Code == "plugin_no_verified_mapping" {
			evidence = "project-unknown"
		}
		diagnostics = append(diagnostics, node.TargetDiagnostic{
			Severity: issue.Severity, Code: issue.Code, Target: target,
			FieldPath: issue.FieldPath, Message: issue.Message, Evidence: evidence,
		})
	}
	return diagnostics
}

func hasBlockingTargetDiagnostic(diagnostics []node.TargetDiagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}
