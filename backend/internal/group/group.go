// Package group 提供用户组业务层：CRUD、每平台选定分发、关联约束与删组迁入默认组。
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
	ErrSubInSelection = errors.New("该组正在选定此订阅，请先在选定区改选")
	ErrSubNotLinked = errors.New("选定的订阅不在该组关联范围内")
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

// Group 用户组
type Group struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	IsDefault     bool   `json:"is_default"`
	NeedsReselect bool   `json:"needs_reselect"`
	SubCount      int64  `json:"sub_count"`  // 关联订阅数
	UserCount     int64  `json:"user_count"` // 组内用户数
}

// Selection 每平台选定（subscription_id=0 表示取消选定）
type Selection struct {
	PlatformID     int64 `json:"platform_id"`
	SubscriptionID int64 `json:"subscription_id"` // 0 = 取消选定
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

// Update 改名（唯一校验）+ 关联订阅多选 + 每平台选定，单事务完成；
// 取消订阅与组的关联时若该组正在选定此订阅 → 拒绝（防悬空选定，Design1 §4.4）
func (s *Service) Update(ctx context.Context, id int64, name string, subIDs []int64, selections []Selection) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkEditable(ctx, tx, id); err != nil {
			return err
		}
		// 改名唯一校验（排除自身）
		var dup int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM groups WHERE name = ? AND id != ?`, name, id).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrNameConflict
		}
		// 关联变更约束：取消订阅与组的关联时，若该订阅仍被本次选定的最终状态引用 → 拒绝（防悬空选定，Design1 §4.4）；
		// 新选定已不再引用它（本次一并改选/取消）→ 视为改选完成，允许
		removed, err := diffRemovedSubs(ctx, tx, id, subIDs)
		if err != nil {
			return err
		}
		selectedInNew := map[int64]bool{}
		for _, sel := range selections {
			if sel.SubscriptionID != 0 {
				selectedInNew[sel.SubscriptionID] = true
			}
		}
		for _, subID := range removed {
			if selectedInNew[subID] {
				return ErrSubInSelection // 「该组正在选定此订阅，请先改选」，接入层 400
			}
		}
		// UPDATE 名称
		if _, err := tx.ExecContext(ctx,
			`UPDATE groups SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, id); err != nil {
			return err
		}
		// 重建 subscription_group_rel
		if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_group_rel WHERE group_id = ?`, id); err != nil {
			return err
		}
		for _, subID := range subIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO subscription_group_rel (subscription_id, group_id) VALUES (?,?)`, subID, id); err != nil {
				return err
			}
		}
		// 重建 group_selections（校验选定必须在关联范围内）
		if err := s.rebuildSelections(ctx, tx, id, selections); err != nil {
			return err
		}
		// 重新选定后清除 needs_reselect 标记
		if _, err := tx.ExecContext(ctx,
			`UPDATE groups SET needs_reselect = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			return err
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

// diffRemovedSubs 计算将被移除的关联订阅
func diffRemovedSubs(ctx context.Context, tx *sql.Tx, id int64, subIDs []int64) ([]int64, error) {
	keep := map[int64]bool{}
	for _, v := range subIDs {
		keep[v] = true
	}
	rows, err := tx.QueryContext(ctx, `SELECT subscription_id FROM subscription_group_rel WHERE group_id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var removed []int64
	for rows.Next() {
		var subID int64
		if err := rows.Scan(&subID); err != nil {
			return nil, err
		}
		if !keep[subID] {
			removed = append(removed, subID)
		}
	}
	return removed, rows.Err()
}

// rebuildSelections 重建选定（校验选定必须在关联范围内；subscription_id=0 表示该平台不选定）
func (s *Service) rebuildSelections(ctx context.Context, tx *sql.Tx, id int64, selections []Selection) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_selections WHERE group_id = ?`, id); err != nil {
		return err
	}
	for _, sel := range selections {
		if sel.SubscriptionID != 0 {
			var linked int
			if err := tx.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM subscription_group_rel WHERE group_id = ? AND subscription_id = ?`,
				id, sel.SubscriptionID).Scan(&linked); err != nil {
				return err
			}
			if linked == 0 {
				return ErrSubNotLinked // 选定必须来自关联订阅
			}
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO group_selections (group_id, platform_id, subscription_id) VALUES (?,?,?)`,
			id, sel.PlatformID, nullIf0(sel.SubscriptionID)); err != nil {
			return err
		}
	}
	return nil
}

// SetSelections 每平台选定（入参 [{platform_id, subscription_id}]）；选定变更需校验订阅在该组关联内；
// 全部平台选定完成后清除 needs_reselect
func (s *Service) SetSelections(ctx context.Context, id int64, selections []Selection) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkEditable(ctx, tx, id); err != nil {
			return err
		}
		if err := s.rebuildSelections(ctx, tx, id, selections); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE groups SET needs_reselect = 0 WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
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
		// 关联/选定由外键 ON DELETE CASCADE 级联清理
		if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
}

// List 组列表：组名、关联订阅数、组内用户数、needs_reselect
func (s *Service) List(ctx context.Context) ([]Group, error) {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id, slug, name, is_default, needs_reselect FROM groups ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("读取用户组列表失败: %w", err)
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		var isDefault, needsReselect int
		if err := rows.Scan(&g.ID, &g.Slug, &g.Name, &isDefault, &needsReselect); err != nil {
			return nil, err
		}
		g.IsDefault = isDefault == 1
		g.NeedsReselect = needsReselect == 1
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM subscription_group_rel WHERE group_id = ?`, out[i].ID).Scan(&out[i].SubCount); err != nil {
			return nil, err
		}
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM users WHERE group_id = ?`, out[i].ID).Scan(&out[i].UserCount); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Get 单个组（编辑回显：含当前每平台选定）
func (s *Service) Get(ctx context.Context, id int64) (*Group, error) {
	var g Group
	var isDefault, needsReselect int
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, slug, name, is_default, needs_reselect FROM groups WHERE id = ?`, id).
		Scan(&g.ID, &g.Slug, &g.Name, &isDefault, &needsReselect)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	g.IsDefault = isDefault == 1
	g.NeedsReselect = needsReselect == 1
	return &g, nil
}

// Selections 组的当前每平台选定（编辑回显用）
func (s *Service) Selections(ctx context.Context, id int64) ([]Selection, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT platform_id, COALESCE(subscription_id,0) FROM group_selections WHERE group_id = ? ORDER BY platform_id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Selection
	for rows.Next() {
		var sel Selection
		if err := rows.Scan(&sel.PlatformID, &sel.SubscriptionID); err != nil {
			return nil, err
		}
		out = append(out, sel)
	}
	return out, rows.Err()
}

// CountAffectedUsers 选定变更影响提示（组内用户数）
func (s *Service) CountAffectedUsers(ctx context.Context, id int64) (int64, error) {
	var n int64
	err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE group_id = ?`, id).Scan(&n)
	return n, err
}

// OnSubscriptionDeleted 删订阅级联（在删订阅事务内调用）：清该订阅的关联与选定；
// 对「因此失去选定」的组置 needs_reselect（平台删除不触发）
func (s *Service) OnSubscriptionDeleted(ctx context.Context, tx *sql.Tx, subscriptionID int64) error {
	// 1) 找出选定此订阅的组
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT group_id FROM group_selections WHERE subscription_id = ?`, subscriptionID)
	if err != nil {
		return err
	}
	var affected []int64
	for rows.Next() {
		var gid int64
		if err := rows.Scan(&gid); err != nil {
			_ = rows.Close()
			return err
		}
		affected = append(affected, gid)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	// 2) 清选定（subscription_id 置 NULL 语义=失去选定；直接删除行更干净）
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_selections WHERE subscription_id = ?`, subscriptionID); err != nil {
		return err
	}
	// 3) 清关联
	if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_group_rel WHERE subscription_id = ?`, subscriptionID); err != nil {
		return err
	}
	// 4) 受影响组置 needs_reselect（失去选定不自动回退，Design1 §4.4）
	for _, gid := range affected {
		if _, err := tx.ExecContext(ctx,
			`UPDATE groups SET needs_reselect = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, gid); err != nil {
			return err
		}
	}
	return nil
}

// nullIf0 0 → NULL（可选标识为 0 时存 NULL，配合三态复用键语义）
func nullIf0(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
