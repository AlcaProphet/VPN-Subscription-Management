package node

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
)

// MigrateSensitiveArrayCredentials 为历史对象数组补稳定身份，并加密此前未纳入契约的凭据。
// 迁移在 HTTP 路由开放前完成，不改变节点业务语义和编辑修订号。
func (s *Service) MigrateSensitiveArrayCredentials(ctx context.Context) error {
	available, err := nodesProtocolJSONAvailable(ctx, s.store.DB())
	if err != nil {
		return err
	}
	if !available {
		return nil
	}
	proto, err := GetProtocol("wireguard")
	if err != nil {
		return err
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, protocol_json FROM nodes WHERE source = 'manual' AND protocol = 'wireguard'`)
	if err != nil {
		return fmt.Errorf("读取历史 WireGuard 节点失败: %w", err)
	}
	type change struct {
		id  int64
		raw string
	}
	type legacyNode struct {
		id  int64
		raw string
	}
	legacyNodes := []legacyNode{}
	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			_ = rows.Close()
			return err
		}
		legacyNodes = append(legacyNodes, legacyNode{id: id, raw: raw})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}

	changes := []change{}
	for _, legacy := range legacyNodes {
		var before map[string]any
		if err := json.Unmarshal([]byte(legacy.raw), &before); err != nil {
			return fmt.Errorf("解析历史 WireGuard 节点 %d 失败: %w", legacy.id, err)
		}
		normalized, err := NormalizeProtocolJSON(proto, before)
		if err != nil {
			return fmt.Errorf("升级历史 WireGuard 节点 %d 失败: %w", legacy.id, err)
		}
		encrypted, err := s.encryptProtocolJSON(ctx, normalized, proto.SensitiveFields)
		if err != nil {
			return fmt.Errorf("加密历史 WireGuard 节点 %d 凭据失败: %w", legacy.id, err)
		}
		if reflect.DeepEqual(before, encrypted) {
			continue
		}
		encoded, err := json.Marshal(encrypted)
		if err != nil {
			return fmt.Errorf("序列化历史 WireGuard 节点 %d 失败: %w", legacy.id, err)
		}
		changes = append(changes, change{id: legacy.id, raw: string(encoded)})
	}
	if len(changes) == 0 {
		return nil
	}
	if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		for _, item := range changes {
			if _, err := tx.ExecContext(ctx, `UPDATE nodes SET protocol_json = ? WHERE id = ?`, item.raw, item.id); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("提交 WireGuard 数组凭据升级失败: %w", err)
	}
	s.log.Info("历史 WireGuard 数组凭据已升级", "nodes", len(changes))
	return nil
}

func nodesProtocolJSONAvailable(ctx context.Context, db *sql.DB) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(nodes)`)
	if err != nil {
		return false, fmt.Errorf("检查 nodes 表结构失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == "protocol_json" {
			return true, nil
		}
	}
	return false, rows.Err()
}
