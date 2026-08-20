package server

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/xray"
)

// renderUserSubscription 是下载服务注入的用户动态渲染器。
func renderUserSubscription(ctx context.Context, st *store.Store, cfg *config.Service, syncSvc *xray.SyncService, creds *xray.CredentialService, subID, userID int64, content []byte, fileName string) ([]byte, error) {
	var targetSyntax, selectionRaw string
	err := st.DB().QueryRowContext(ctx,
		`SELECT b.target_syntax, b.selection_json
		 FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 WHERE v.owner_type = 'subscription' AND v.owner_id = ?
		   AND v.version_no = (SELECT current_version FROM subscriptions WHERE id = ?)`,
		subID, subID).Scan(&targetSyntax, &selectionRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return content, nil // 直接上传内容，原样返回
	}
	if err != nil {
		return nil, err
	}
	// 高级模式关闭：占位替换为注释，仍保证语法完整。
	if !cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		return replacePlaceholder(content, targetSyntax, "# Xray 高级模式未启用"), nil
	}
	uuid, secret, credErr := creds.Credentials(ctx, userID)
	hasCreds := credErr == nil
	if credErr != nil && !errors.Is(credErr, xray.ErrIncompleteCredentials) {
		return nil, credErr
	}
	targets, err := syncSvc.Targets(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !hasCreds {
		return replacePlaceholder(content, targetSyntax, "# 节点未开通，请联系管理员"), nil
	}
	var lines []string
	for _, t := range targets {
		protocol, params, err := syncSvc.NodeRenderParams(ctx, t.NodeID)
		if err != nil {
			return nil, err
		}
		generic := targetSyntax == "generic-subs"
		link, err := assembly.RenderLink(protocol, t.RenderName, hostOf(t.APIAddr), portOf(t.APIAddr), withCreds(params, protocol, uuid, secret), generic)
		if err != nil {
			continue
		}
		lines = append(lines, link)
	}
	switch targetSyntax {
	case "sr-subs", "generic-subs":
		if len(lines) == 0 {
			// 空目标集：占位整行移除。
			return []byte(base64.StdEncoding.EncodeToString(removePlaceholderLine(content))), nil
		}
		replaced := replacePlaceholderLines(content, lines)
		return []byte(base64.StdEncoding.EncodeToString(replaced)), nil
	case "clash-yaml":
		replaced := replacePlaceholderLines(content, lines)
		return replaced, nil
	default:
		return content, nil
	}
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

func portOf(apiAddr string) int {
	_, portStr, err := netSplit(apiAddr)
	if err != nil {
		return 0
	}
	var p int
	_, _ = fmt.Sscanf(portStr, "%d", &p)
	return p
}

func netSplit(addr string) (string, string, error) {
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
