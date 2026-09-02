package assembly

import (
	"context"
	"fmt"
	"strings"

	gyaml "github.com/goccy/go-yaml"

	"vpn-sub/internal/node"
)

// CheckNodeTarget 使用实际输出适配器检查单个节点目标；该方法只构造内存产物。
func (s *Service) CheckNodeTarget(ctx context.Context, target, protocol, renderName, host string, port int, params map[string]any) (node.CheckRenderResult, error) {
	if err := ctx.Err(); err != nil {
		return node.CheckRenderResult{}, err
	}
	switch target {
	case "clash-yaml":
		return s.checkClashNodeTarget(protocol, renderName, host, port, params)
	case "sr-subs", "generic-subs":
		return checkLinkNodeTarget(target, protocol, renderName, host, port, params)
	default:
		return node.CheckRenderResult{}, fmt.Errorf("节点检查不支持目标: %s", target)
	}
}

func (s *Service) checkClashNodeTarget(protocol, renderName, host string, port int, params map[string]any) (node.CheckRenderResult, error) {
	nd := &nodeData{
		Protocol:     protocol,
		RenderName:   renderName,
		Host:         host,
		Port:         port,
		ProtocolJSON: params,
	}
	root := gyaml.MapSlice{
		{Key: "proxies", Value: []any{orderedMapToMapSlice(s.clashProxy(nd))}},
		{Key: "rules", Value: []any{"GEOIP,CN,DIRECT", "MATCH,DIRECT"}},
	}
	content, err := marshalClashYAML(root, nil)
	if err != nil {
		return node.CheckRenderResult{}, fmt.Errorf("序列化 Clash 节点检查片段失败: %w", err)
	}
	issues := CheckClashContent(content)
	diagnostics := make([]node.TargetDiagnostic, 0, len(issues))
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
			Evidence:  "mihomo-1.19.29-yaml",
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
				Evidence:  "mihomo-1.19.29-yaml",
			})
		}
	}
	preview := string(content)
	return node.CheckRenderResult{Preview: preview, Diagnostics: diagnostics}, nil
}

func checkLinkNodeTarget(target, protocol, renderName, host string, port int, params map[string]any) (node.CheckRenderResult, error) {
	diagnostics := linkTargetDiagnostics(target, protocol, params)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" && diagnostic.Code == "core_semantic_unexpressible" {
			return node.CheckRenderResult{Diagnostics: diagnostics}, nil
		}
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
		plugin, _ := params["plugin"].(string)
		if plugin == "obfs" {
			add(&diagnostics, "warn", "plugin_name_mapping", "plugin",
				"内部 obfs 已映射为 obfs-local/obfs-host；CVR 2.5.2 真机导入仍需复核", "cvr-2.5.2-uri")
		} else if plugin != "" && plugin != "v2ray-plugin" && plugin != "shadow-tls" && plugin != "restls" {
			add(&diagnostics, "warn", "plugin_no_verified_mapping", "plugin",
				fmt.Sprintf("插件 %s 暂无已验证的目标映射，当前按原格式透传", plugin), "project-unknown")
		}
		cipher, _ := params["cipher"].(string)
		if strings.HasPrefix(cipher, "2022-") {
			add(&diagnostics, "warn", "unverified_compatibility", "cipher",
				"SS 2022 当前仅登记为待验证兼容项，未宣称完整 URI 支持", "cvr-2.5.2-uri")
		}
	}
	return diagnostics
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}
