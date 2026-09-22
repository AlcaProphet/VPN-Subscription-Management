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
			FieldPath: issue.Path,
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
	preview := string(content)
	return node.CheckRenderResult{Preview: preview, Diagnostics: diagnostics}, nil
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
	link, err := RenderLink(protocol, renderName, host, port, params, target == "generic-subs")
	if err != nil {
		return node.CheckRenderResult{Diagnostics: diagnostics}, fmt.Errorf("生成 %s 节点链接失败: %w", target, err)
	}
	return node.CheckRenderResult{Preview: link, Diagnostics: diagnostics}, nil
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
	case "vmess":
		switch cipher, _ := params["cipher"].(string); cipher {
		case "chacha20-poly1305":
			add(&diagnostics, "warn", "uri_algorithm_rewrite", "cipher",
				"CVR 2.5.2 URI 入口对 chacha20-poly1305 可能发生算法改写，导入后需复核", "cvr-2.5.2-uri")
		case "zero":
			add(&diagnostics, "warn", "uri_algorithm_rewrite", "cipher",
				"CVR 2.5.2 URI 入口对 zero 可能回退或改写，导入后需复核", "cvr-2.5.2-uri")
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
	case "ss":
		cipher, _ := params["cipher"].(string)
		if strings.HasPrefix(cipher, "2022-") {
			add(&diagnostics, "warn", "unverified_compatibility", "cipher",
				"SS 2022 当前仅登记为待验证兼容项，未宣称完整 URI 支持", "cvr-2.5.2-uri")
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
	}
	return diagnostics
}

// hasActiveParam 判断调优字段是否设置了非零／非 false 值。
func hasActiveParam(params map[string]any, key string) bool {
	value, ok := params[key]
	if !ok {
		return false
	}
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
	default:
		return true
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
