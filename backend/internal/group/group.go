// Package group 提供用户组业务层：CRUD、默认配额与删组迁入默认组。
package group

import (
	"context"
	"database/sql"
	"encoding/json"
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
	ErrBadRequest   = errors.New("参数错误")
)

// GroupNode 组节点分配项。
type GroupNode struct {
	NodeID      int64   `json:"node_id"`
	NodeName    string  `json:"node_name"`
	DisplayName *string `json:"display_name,omitempty"`
	RenderName  string  `json:"render_name"`
	SortOrder   int     `json:"sort_order"`
	IsPublic    bool    `json:"is_public"`
	Source      string  `json:"source"`
}

// CandidateNode 候选集节点项；InPartialBlueprint 表示该节点仅出现在部分已激活蓝图中。
type CandidateNode struct {
	NodeID             int64  `json:"node_id"`
	Name               string `json:"name"`
	InPartialBlueprint bool   `json:"in_partial_blueprint"`
}


// NodesChangedFunc 组节点分配变化后的回调（Step3 注入同步 diff）。
type NodesChangedFunc func(ctx context.Context, groupID int64, userIDs []int64)

// Service 用户组服务
type Service struct {
	store          *store.Store
	log            *slog.Logger
	onNodesChanged NodesChangedFunc
}

func NewService(st *store.Store, lg *slog.Logger) *Service {
	return &Service{store: st, log: lg}
}

// SetOnNodesChanged 注入组节点变化回调（Build6 Step2 先留空，Step3 接线）。
func (s *Service) SetOnNodesChanged(fn NodesChangedFunc) {
	s.onNodesChanged = fn
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
	var affected []int64
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
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
		// 收集受影响 active 用户（删除后迁入默认组，推送目标会变化）
		rows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE group_id = ? AND status = 'active'`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err != nil {
				_ = rows.Close()
				return err
			}
			affected = append(affected, uid)
		}
		if err := rows.Close(); err != nil {
			return err
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
	if err != nil {
		return err
	}
	if s.onNodesChanged != nil && len(affected) > 0 {
		s.onNodesChanged(ctx, id, affected)
	}
	return nil
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

// SetDefaultQuota 设置组默认月度配额；NULL/0 均不限流量，负数拒绝。
func (s *Service) SetDefaultQuota(ctx context.Context, id int64, quota *float64) error {
	if quota != nil && *quota < 0 {
		return fmt.Errorf("%w: 配额不能为负数", ErrBadRequest)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkEditable(ctx, tx, id); err != nil {
			return err
		}
		var q any
		if quota != nil {
			q = *quota
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE groups SET default_quota = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, q, id); err != nil {
			return fmt.Errorf("更新组默认配额失败: %w", err)
		}
		return nil
	})
}

// SetNodes 设置组节点分配：仅允许候选集内且满足可用性过滤的 xray 非公共节点。
// 先删后插 + 受影响 active 用户清单收集在同一 BEGIN IMMEDIATE 事务内完成。
func (s *Service) SetNodes(ctx context.Context, id int64, nodeIDs []int64) error {
	var affected []int64
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := s.checkEditable(ctx, tx, id); err != nil {
			return err
		}
		candidates, err := s.candidateSetTx(ctx, tx)
		if err != nil {
			return err
		}
		candidateMap := map[string]bool{}
		for _, cand := range candidates {
			candidateMap[cand.Name] = true
		}
		// 校验每个节点
		for _, nid := range nodeIDs {
			var name, source string
			var isPublic, enabled, allocatable, missing, instEnabled int
			err := tx.QueryRowContext(ctx,
				`SELECT n.name, n.source, n.is_public, n.enabled, n.allocatable, n.missing, COALESCE(i.enabled,0)
				 FROM nodes n LEFT JOIN xray_instances i ON i.id = n.instance_id WHERE n.id = ?`, nid).
				Scan(&name, &source, &isPublic, &enabled, &allocatable, &missing, &instEnabled)
			if err != nil {
				return fmt.Errorf("%w: 节点不存在", ErrBadRequest)
			}
			if source != "xray" || isPublic == 1 || enabled != 1 || allocatable != 1 || missing != 0 || instEnabled != 1 {
				return fmt.Errorf("%w: 节点不可分配", ErrBadRequest)
			}
			if !candidateMap[name] {
				return fmt.Errorf("%w: 节点不在当前候选集", ErrBadRequest)
			}
		}
		// 收集受影响 active 用户
		rows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE group_id = ? AND status = 'active'`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var uid int64
			if err := rows.Scan(&uid); err != nil {
				_ = rows.Close()
				return err
			}
			affected = append(affected, uid)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		// 先删后插
		if _, err := tx.ExecContext(ctx, `DELETE FROM group_nodes WHERE group_id = ?`, id); err != nil {
			return fmt.Errorf("清空组节点分配失败: %w", err)
		}
		for i, nid := range nodeIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO group_nodes (group_id, node_id, sort_order) VALUES (?,?,?)`, id, nid, i); err != nil {
				return fmt.Errorf("写入组节点分配失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.onNodesChanged != nil && len(affected) > 0 {
		s.onNodesChanged(ctx, id, affected)
	}
	return nil
}

// CandidateSet 返回当前所有已激活装配蓝图的 xray 候选节点并集（含 partial 标注）。
func (s *Service) CandidateSet(ctx context.Context) ([]CandidateNode, error) {
	return s.candidateSetTx(ctx, nil)
}

// CandidateNames 返回候选节点稳定名列表（供内部分配校验/重算使用）。
func (s *Service) CandidateNames(ctx context.Context) ([]string, error) {
	nodes, err := s.CandidateSet(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Name)
	}
	return out, nil
}

func (s *Service) candidateSetTx(ctx context.Context, tx *sql.Tx) ([]CandidateNode, error) {
	query := func(q interface {
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	}) (*sql.Rows, error) {
		return q.QueryContext(ctx,
			`SELECT b.selection_json
			 FROM assembly_blueprints b
			 JOIN versions v ON v.id = b.version_id
			 JOIN subscriptions s ON s.id = v.owner_id AND v.owner_type = 'subscription'
			 WHERE s.current_version = v.version_no
			   AND b.target_syntax IN ('clash-yaml','sr-subs','generic-subs')`)
	}
	var rows *sql.Rows
	var err error
	if tx != nil {
		rows, err = query(tx)
	} else {
		rows, err = query(s.store.DB())
	}
	if err != nil {
		return nil, fmt.Errorf("读取候选集失败: %w", err)
	}
	defer rows.Close()
	set := map[string]bool{}
	count := map[string]int{}
	var order []string
	blueprintsWithCandidates := 0
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var sel struct {
			XrayCandidates []string `json:"xray_candidates"`
		}
		if err := json.Unmarshal([]byte(raw), &sel); err != nil {
			continue
		}
		if len(sel.XrayCandidates) == 0 {
			continue
		}
		blueprintsWithCandidates++
		for _, name := range sel.XrayCandidates {
			if !set[name] {
				set[name] = true
				order = append(order, name)
			}
			count[name]++
		}
	}
	out := make([]CandidateNode, 0, len(order))
	for _, name := range order {
		var nodeID int64
		var nrows *sql.Rows
		var qerr error
		if tx != nil {
			nrows, qerr = tx.QueryContext(ctx, `SELECT id FROM nodes WHERE name = ? AND source = 'xray'`, name)
		} else {
			nrows, qerr = s.store.DB().QueryContext(ctx, `SELECT id FROM nodes WHERE name = ? AND source = 'xray'`, name)
		}
		if qerr != nil {
			return nil, qerr
		}
		if nrows.Next() {
			if err := nrows.Scan(&nodeID); err != nil {
				_ = nrows.Close()
				return nil, err
			}
		}
		if err := nrows.Close(); err != nil {
			return nil, err
		}
		out = append(out, CandidateNode{
			NodeID:             nodeID,
			Name:               name,
			InPartialBlueprint: blueprintsWithCandidates > 0 && count[name] < blueprintsWithCandidates,
		})
	}
	return out, rows.Err()
}

// RecomputeCandidateSet 重算候选集并集并删除越界/不可用分配；返回受影响 active 用户 ID。
func (s *Service) RecomputeCandidateSet(ctx context.Context) ([]int64, error) {
	candidates, err := s.CandidateSet(ctx)
	if err != nil {
		return nil, err
	}
	candidateMap := map[string]bool{}
	for _, cand := range candidates {
		candidateMap[cand.Name] = true
	}
	type removed struct {
		groupID int64
		userIDs []int64
	}
	var removedGroups []removed
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT gn.group_id, gn.node_id, n.name, n.source, n.is_public, n.enabled, n.allocatable, n.missing, COALESCE(i.enabled,0)
			 FROM group_nodes gn
			 JOIN nodes n ON n.id = gn.node_id
			 LEFT JOIN xray_instances i ON i.id = n.instance_id`)
		if err != nil {
			return err
		}
		type rowInfo struct {
			groupID                                              int64
			nodeID                                               int64
			name                                                 string
			source                                               string
			isPublic, enabled, allocatable, missing, instEnabled int
		}
		var toDelete []rowInfo
		for rows.Next() {
			var r rowInfo
			if err := rows.Scan(&r.groupID, &r.nodeID, &r.name, &r.source, &r.isPublic, &r.enabled, &r.allocatable, &r.missing, &r.instEnabled); err != nil {
				_ = rows.Close()
				return err
			}
			// 公共节点不可写入 group_nodes；越界或不可用删除
			if r.source != "xray" || r.isPublic == 1 || r.enabled != 1 || r.allocatable != 1 || r.missing != 0 || r.instEnabled != 1 || !candidateMap[r.name] {
				toDelete = append(toDelete, r)
			}
		}
		if err := rows.Close(); err != nil {
			return err
		}
		groupSet := map[int64]bool{}
		for _, r := range toDelete {
			groupSet[r.groupID] = true
		}
		for gid := range groupSet {
			urows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE group_id = ? AND status = 'active'`, gid)
			if err != nil {
				return err
			}
			var uids []int64
			for urows.Next() {
				var uid int64
				if err := urows.Scan(&uid); err != nil {
					_ = urows.Close()
					return err
				}
				uids = append(uids, uid)
			}
			if err := urows.Close(); err != nil {
				return err
			}
			removedGroups = append(removedGroups, removed{groupID: gid, userIDs: uids})
		}
		for _, r := range toDelete {
			if _, err := tx.ExecContext(ctx, `DELETE FROM group_nodes WHERE group_id = ? AND node_id = ?`, r.groupID, r.nodeID); err != nil {
				return fmt.Errorf("删除越界组节点分配失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var all []int64
	seen := map[int64]bool{}
	for _, g := range removedGroups {
		if s.onNodesChanged != nil && len(g.userIDs) > 0 {
			s.onNodesChanged(ctx, g.groupID, g.userIDs)
		}
		for _, uid := range g.userIDs {
			if !seen[uid] {
				seen[uid] = true
				all = append(all, uid)
			}
		}
	}
	return all, nil
}

// GroupNodes 返回组内节点分配（含公共节点标注由上层合并）。
func (s *Service) GroupNodes(ctx context.Context, id int64) ([]GroupNode, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT gn.node_id, n.name, n.display_name, gn.sort_order, n.is_public, n.source,
		        COALESCE(NULLIF(n.display_name,''), n.name)
		 FROM group_nodes gn JOIN nodes n ON n.id = gn.node_id
		 WHERE gn.group_id = ? ORDER BY gn.sort_order`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GroupNode, 0)
	for rows.Next() {
		var gn GroupNode
		var display sql.NullString
		var isPublic int
		if err := rows.Scan(&gn.NodeID, &gn.NodeName, &display, &gn.SortOrder, &isPublic, &gn.Source, &gn.RenderName); err != nil {
			return nil, err
		}
		if display.Valid && display.String != "" {
			gn.DisplayName = &display.String
		}
		gn.IsPublic = isPublic == 1
		out = append(out, gn)
	}
	return out, rows.Err()
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
