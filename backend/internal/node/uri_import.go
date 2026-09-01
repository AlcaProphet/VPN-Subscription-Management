// uri_import.go：manual 节点批量 URI 导入。
package node

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"vpn-sub/internal/uriparse"
)

// ImportLineResult 单行导入回执。
type ImportLineResult struct {
	Line   int    `json:"line"`
	Raw    string `json:"raw"`
	OK     bool   `json:"ok"`
	Name   string `json:"name,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ImportURIs 解析 → 全局 name 去重 → 单事务批量创建；任一单行失败仅跳过，不中断整批。
func (s *Service) ImportURIs(ctx context.Context, text string) ([]ImportLineResult, error) {
	lines := importLines(text)
	out := make([]ImportLineResult, 0, len(lines))

	// 已有节点 name 去重集合（含全部来源，沿用跨命名空间口径）。
	existing := map[string]bool{}
	rows, err := s.store.DB().QueryContext(ctx, `SELECT name FROM nodes`)
	if err != nil {
		return nil, fmt.Errorf("读取已有节点失败: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return nil, err
		}
		existing[name] = true
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 加密等无事务操作统一放在事务外，避免单连接库内嵌套查询死锁。
	type prepared struct {
		line     int
		raw      string
		name     string
		protocol string
		host     string
		port     int
		params   map[string]any
		skip     bool
		reason   string
	}
	preparedList := make([]prepared, 0, len(lines))
	for i, raw := range lines {
		r, parseErr := uriparse.Parse(raw)
		if parseErr != nil {
			preparedList = append(preparedList, prepared{line: i + 1, raw: raw, skip: true, reason: parseErr.Error()})
			continue
		}
		name := r.Name
		if name == "" {
			name = defaultImportName(*r)
		}
		item := prepared{line: i + 1, raw: raw, name: name, protocol: r.Protocol, host: r.Host, port: r.Port}
		if err := ValidateNodeName(name); err != nil {
			item.skip = true
			item.reason = err.Error()
			preparedList = append(preparedList, item)
			continue
		}
		proto, err := GetProtocol(r.Protocol)
		if err != nil {
			item.skip = true
			item.reason = err.Error()
			preparedList = append(preparedList, item)
			continue
		}
		if err := validateHostPort(r.Host, r.Port); err != nil {
			item.skip = true
			item.reason = err.Error()
			preparedList = append(preparedList, item)
			continue
		}
		if err := validateProtocolFields(proto, r.Params, false); err != nil {
			item.skip = true
			item.reason = err.Error()
			preparedList = append(preparedList, item)
			continue
		}
		encrypted, err := s.encryptProtocolJSON(ctx, r.Params, proto.SensitiveFields)
		if err != nil {
			item.skip = true
			item.reason = err.Error()
			preparedList = append(preparedList, item)
			continue
		}
		item.params = encrypted
		preparedList = append(preparedList, item)
	}

	seen := map[string]bool{}
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		for _, p := range preparedList {
			if p.skip {
				out = append(out, ImportLineResult{Line: p.line, Raw: p.raw, OK: false, Name: p.name, Reason: p.reason})
				continue
			}
			if existing[p.name] || seen[p.name] {
				out = append(out, ImportLineResult{Line: p.line, Raw: p.raw, OK: false, Name: p.name, Reason: "名称重复，已跳过"})
				continue
			}
			if err := CheckRenderNameNamespaceTx(ctx, tx, p.name); err != nil {
				out = append(out, ImportLineResult{Line: p.line, Raw: p.raw, OK: false, Name: p.name, Reason: err.Error()})
				continue
			}
			rawJSON, err := json.Marshal(p.params)
			if err != nil {
				out = append(out, ImportLineResult{Line: p.line, Raw: p.raw, OK: false, Name: p.name, Reason: "序列化节点参数失败"})
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO nodes (source, name, display_name, instance_id, tag, protocol, host, port, protocol_json, is_public, enabled, allocatable, missing)
				 VALUES ('manual', ?, NULL, NULL, '', ?, ?, ?, ?, 0, 1, 1, 0)`,
				p.name, p.protocol, p.host, p.port, string(rawJSON)); err != nil {
				if isUniqueViolation(err) {
					out = append(out, ImportLineResult{Line: p.line, Raw: p.raw, OK: false, Name: p.name, Reason: "名称重复，已跳过"})
					continue
				}
				return fmt.Errorf("批量导入写入失败: %w", err)
			}
			existing[p.name] = true
			seen[p.name] = true
			out = append(out, ImportLineResult{Line: p.line, Raw: p.raw, OK: true, Name: p.name})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// importLines 拆分为待解析行；单行整块 Base64 时先解码。
func importLines(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	if !strings.Contains(text, "://") {
		compact := strings.ReplaceAll(strings.ReplaceAll(text, "\n", ""), "\r", "")
		for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
			if decoded, err := enc.DecodeString(compact); err == nil && strings.Contains(string(decoded), "://") {
				return strings.Split(strings.TrimSpace(string(decoded)), "\n")
			}
		}
	}
	return lines
}

func defaultImportName(r uriparse.Result) string {
	if r.Name != "" {
		return r.Name
	}
	return fmt.Sprintf("%s-%s-%d", r.Protocol, r.Host, r.Port)
}
