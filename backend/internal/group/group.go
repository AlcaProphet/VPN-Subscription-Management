// Package group 提供用户组业务层：CRUD、默认配额与删组迁入默认组。
package group

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrNameConflict = errors.New("组名已存在")
	ErrDefaultGroup = errors.New("预置默认组不可删除")
	ErrNotFound     = errors.New("用户组不存在")
)

// Service 用户组服务
type Service struct {
	store *store.Store
	log   *slog.Logger
}

func NewService(st *store.Store, lg *slog.Logger) *Service {
	return &Service{store: st, log: lg}
}

// Group 用户组（高级模式下 default_quota 才可能有值；base 模式为 NULL，JSON 省略）
type Group struct {
	ID           int64    `json:"id"`
	Slug         string   `json:"slug"`
	Name         string   `json:"name"`
	IsDefault    bool     `json:"is_default"`
	DefaultQuota *float64 `json:"default_quota,omitempty"` // 组默认月度配额（NULL=不限流量）
	NodeCount    int64    `json:"node_count"`              // 已分配节点数（Build6 起有数据）
	UserCount    int64    `json:"user_count"`              // 组内用户数
}

// Create 名称全局唯一校验；slug 自动生成（group- 前缀，独立命名空间）
func (s *Service) Create(ctx context.Context, name string) (*Group, error) {
	var created *Group
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE name = ?`, name).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrNameConflict // 接入层映射 409
		}
		value, err := slug.Generate(ctx, tx, "group-", func(v string) (bool, error) {
			return slug.TableHasSlug(ctx, tx, "groups", v)
		})
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO groups (slug, name) VALUES (?,?)`, value, name)
		if err != nil {
			return fmt.Errorf("创建用户组失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Group{ID: id, Slug: value, Name: name}
		return nil
	})
	return created, err
}

// Update 仅改名（名称全局唯一校验）；节点分配与默认配额写入由 Build6 高级端点承接
func (s *Service) Update(ctx context.Context, id int64, name string) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkEditable(ctx, tx, id); err != nil {
			return err
		}
		var dup int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM groups WHERE name = ? AND id != ?`, name, id).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrNameConflict
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE groups SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, id); err != nil {
			return fmt.Errorf("更新用户组失败: %w", err)
		}
		return nil
	})
}

// checkEditable 组存在性校验
func (s *Service) checkEditable(ctx context.Context, tx *sql.Tx, id int64) error {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE id = ?`, id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete 默认组不可删；组内用户自动迁入默认组（Token 无需清理，实时解析自动跟随）
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var isDefault int
		if err := tx.QueryRowContext(ctx, `SELECT is_default FROM groups WHERE id = ?`, id).Scan(&isDefault); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if isDefault == 1 {
			return ErrDefaultGroup // 「预置默认组不可删除」，接入层 400
		}
		var defaultID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE is_default = 1 LIMIT 1`).Scan(&defaultID); err != nil {
			return fmt.Errorf("预置默认组缺失: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET group_id = ? WHERE group_id = ?`, defaultID, id); err != nil {
			return fmt.Errorf("迁入默认组失败: %w", err)
		}
		// 节点分配由 group_nodes 外键 ON DELETE CASCADE 级联清理
		if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除用户组失败: %w", err)
		}
		return nil
	})
}

// List 组列表：组名、是否默认组、默认配额、节点分配数、组内用户数
func (s *Service) List(ctx context.Context) ([]Group, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT g.id, g.slug, g.name, g.is_default, g.default_quota,
        (SELECT COUNT(*) FROM group_nodes gn WHERE gn.group_id = g.id),
        (SELECT COUNT(*) FROM users u WHERE u.group_id = g.id)
 FROM groups g ORDER BY g.id`)
	if err != nil {
		return nil, fmt.Errorf("读取用户组列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Group, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Get 单个组（基础信息 + 默认配额 + 节点数 + 用户数）
func (s *Service) Get(ctx context.Context, id int64) (*Group, error) {
	row := s.store.DB().QueryRowContext(ctx,
		`SELECT g.id, g.slug, g.name, g.is_default, g.default_quota,
        (SELECT COUNT(*) FROM group_nodes gn WHERE gn.group_id = g.id),
        (SELECT COUNT(*) FROM users u WHERE u.group_id = g.id)
 FROM groups g WHERE g.id = ?`, id)
	g, err := scanGroup(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &g, nil
}

// rowScanner 兼容 *sql.Row 与 *sql.Rows 的扫描接口
type rowScanner interface {
	Scan(dest ...any) error
}

func scanGroup(row rowScanner) (Group, error) {
	var g Group
	var isDefault int
	var quota sql.NullFloat64
	if err := row.Scan(&g.ID, &g.Slug, &g.Name, &isDefault, &quota, &g.NodeCount, &g.UserCount); err != nil {
		return Group{}, err
	}
	g.IsDefault = isDefault == 1
	if quota.Valid {
		g.DefaultQuota = &quota.Float64
	}
	return g, nil
}
