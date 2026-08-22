package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"strings"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/xray"
)

// renderUserSubscription 是下载服务注入的用户动态渲染器。
func renderUserSubscription(ctx context.Context, st *store.Store, cfg *config.Service, syncSvc *xray.SyncService, creds *xray.CredentialService, subID, userID int64, content []byte, fileName string) ([]byte, error) {
	var targetSyntax, planRaw string
	err := st.DB().QueryRowContext(ctx,
		`SELECT b.target_syntax, COALESCE(b.render_plan_json, '{}')
		 FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 WHERE v.owner_type = 'subscription' AND v.owner_id = ?
		   AND v.version_no = (SELECT current_version FROM subscriptions WHERE id = ?)`,
		subID, subID).Scan(&targetSyntax, &planRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return content, nil // 直接上传内容，原样返回
	}
	if err != nil {
		return nil, err
	}

	// 注释优先级：高级模式关闭 > 无凭据 > 有凭据但空目标集（空目标集仅对 SR/generic 整行移除）。
	advancedOn := cfg.GetBool(ctx, config.KeyAdvancedMode, false)
	comment := ""
	if !advancedOn {
		comment = "# Xray 高级模式未启用"
	}
	uuid, secret := "", ""
	hasCreds := false
	if advancedOn {
		var credErr error
		uuid, secret, credErr = creds.Credentials(ctx, userID)
		hasCreds = credErr == nil
		if credErr != nil && !errors.Is(credErr, xray.ErrIncompleteCredentials) {
			return nil, credErr
		}
		if !hasCreds {
			comment = "# 节点未开通，请联系管理员"
		}
	}
	targets, err := syncSvc.Targets(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Clash 全量重渲染：使用自包含 render_plan_json。
	if targetSyntax == "clash-yaml" {
		if isLegacyClashPlan(planRaw) {
			// 旧版蓝图（Build6 修复前）回退到占位行替换，避免空渲染。
			if comment != "" {
				return replacePlaceholder(content, targetSyntax, comment), nil
			}
			lines := renderLinkLines(ctx, syncSvc, targets, targetSyntax, uuid, secret, hasCreds)
			return replacePlaceholderLines(content, lines), nil
		}
		manualNames, err := manualRenderNames(ctx, st)
		if err != nil {
			return nil, err
		}
		dynamic := make([]assembly.DynamicNode, 0, len(targets))
		for _, t := range targets {
			protocol, params, err := syncSvc.NodeRenderParams(ctx, t.NodeID)
			if err != nil {
				return nil, err
			}
			dynamic = append(dynamic, assembly.DynamicNode{
				Name:         t.Name,
				RenderName:   t.RenderName,
				Protocol:     protocol,
				Host:         hostOf(t.APIAddr),
				Port:         t.Port,
				ProtocolJSON: withCreds(params, protocol, uuid, secret),
			})
		}
		return assembly.RenderClashPlan([]byte(planRaw), dynamic, manualNames, comment)
	}

	// SR / generic 订阅：占位替换或整行移除后整体 base64。
	if comment != "" {
		replaced := replacePlaceholderLines(content, []string{comment})
		return []byte(base64.StdEncoding.EncodeToString(replaced)), nil
	}
	lines := renderLinkLines(ctx, syncSvc, targets, targetSyntax, uuid, secret, true)
	switch targetSyntax {
	case "sr-subs", "generic-subs":
		if len(lines) == 0 {
			// 有凭据但空目标集：占位整行移除。
			return []byte(base64.StdEncoding.EncodeToString(removePlaceholderLine(content))), nil
		}
		replaced := replacePlaceholderLines(content, lines)
		return []byte(base64.StdEncoding.EncodeToString(replaced)), nil
	default:
		return content, nil
	}
}

func renderLinkLines(ctx context.Context, syncSvc *xray.SyncService, targets []xray.Target, targetSyntax, uuid, secret string, hasCreds bool) []string {
	if !hasCreds {
		return nil
	}
	var lines []string
	for _, t := range targets {
		protocol, params, err := syncSvc.NodeRenderParams(ctx, t.NodeID)
		if err != nil {
			slog.Warn("读取动态节点渲染参数失败", "node", t.Name, "err", err)
			continue
		}
		generic := targetSyntax == "generic-subs"
		link, err := assembly.RenderLink(protocol, t.RenderName, hostOf(t.APIAddr), t.Port, withCreds(params, protocol, uuid, secret), generic)
		if err != nil {
			slog.Warn("生成订阅链接失败", "node", t.Name, "err", err)
			continue
		}
		lines = append(lines, link)
	}
	return lines
}

func manualRenderNames(ctx context.Context, st *store.Store) (map[string]string, error) {
	rows, err := st.DB().QueryContext(ctx,
		`SELECT name, COALESCE(NULLIF(display_name,''), name) FROM nodes WHERE source = 'manual'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, render string
		if err := rows.Scan(&name, &render); err != nil {
			return nil, err
		}
		out[name] = render
	}
	return out, rows.Err()
}

func isLegacyClashPlan(raw string) bool {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "{}" {
		return true
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return true
	}
	if v, ok := m["manual_proxies"].([]any); ok && len(v) > 0 {
		if _, ok := v[0].(string); ok {
			return true
		}
	}
	if v, ok := m["proxy_groups"].([]any); ok && len(v) > 0 {
		if _, ok := v[0].(string); ok {
			return true
		}
	}
	if v, ok := m["rules"].([]any); ok && len(v) > 0 {
		if first, ok := v[0].(map[string]any); ok {
			if _, hasType := first["type"]; !hasType {
				return true
			}
		}
	}
	return false
}

func replacePlaceholder(content []byte, targetSyntax, comment string) []byte {
	if targetSyntax == "sr-subs" || targetSyntax == "generic-subs" {
		replaced := removePlaceholderLine(content)
		if !bytesContains(replaced, []byte("# {{xray_nodes}}")) {
			return []byte(base64.StdEncoding.EncodeToString(replaced))
		}
		return []byte(base64.StdEncoding.EncodeToString(replacePlaceholderLines(replaced, []string{comment})))
	}
	return replacePlaceholderLines(content, []string{comment})
}

func replacePlaceholderLines(content []byte, lines []string) []byte {
	text := string(content)
	placeholder := "# {{xray_nodes}}"
	if !strings.Contains(text, placeholder) {
		return content
	}
	var sb strings.Builder
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, placeholder) {
			for _, l := range lines {
				sb.WriteString(l)
				sb.WriteString("\n")
			}
			continue
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return []byte(strings.TrimSuffix(sb.String(), "\n"))
}

func removePlaceholderLine(content []byte) []byte {
	text := string(content)
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, "# {{xray_nodes}}") {
			continue
		}
		out = append(out, line)
	}
	return []byte(strings.Join(out, "\n"))
}

func bytesContains(b []byte, sub []byte) bool {
	return strings.Contains(string(b), string(sub))
}

func hostOf(apiAddr string) string {
	host, _, err := netSplit(apiAddr)
	if err != nil {
		return apiAddr
	}
	return host
}

func netSplit(addr string) (string, string, error) {
	// 带方括号的 IPv6 使用标准库解析；无括号形式按最后一个冒号切分端口。
	if strings.HasPrefix(addr, "[") {
		return net.SplitHostPort(addr)
	}
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return "", "", errors.New("missing port")
	}
	return addr[:idx], addr[idx+1:], nil
}

func withCreds(params map[string]any, protocol, uuid, secret string) map[string]any {
	out := make(map[string]any, len(params)+2)
	for k, v := range params {
		out[k] = v
	}
	switch protocol {
	case "vless", "vmess":
		out["uuid"] = uuid
	case "trojan", "shadowsocks", "ss":
		out["password"] = secret
	}
	return out
}
