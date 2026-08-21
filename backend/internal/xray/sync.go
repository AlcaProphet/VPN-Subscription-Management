package xray

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/xtls/xray-core/common/protocol"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
)

// API 抽象 Xray gRPC 客户端，便于 fake 测试。
type API interface {
	AddUser(ctx context.Context, tag string, u *protocol.User) error
	RemoveUser(ctx context.Context, tag, email string) error
}

// Target 是用户推送目标节点。
type Target struct {
	NodeID      int64   `json:"node_id"`
	Name        string  `json:"name"`
	DisplayName *string `json:"display_name,omitempty"`
	RenderName  string  `json:"render_name"`
	Tag         string  `json:"tag"`
	InstanceID  int64   `json:"instance_id"`
	APIAddr     string  `json:"api_addr"`
	Port        int     `json:"port"`
}

// SyncService 处理用户生命周期 Xray 推送/移除。
type SyncService struct {
	store     *store.Store
	cfg       *config.Service
	creds     *CredentialService
	instances *InstanceService
	registry  *tasks.Registry
	log       *slog.Logger
	apiFor    func(ctx context.Context, instanceID int64) (API, error)
	ext       *ExtService
}

// SetExtService 注入独立账号服务（对账/修复使用）。
func (s *SyncService) SetExtService(e *ExtService) { s.ext = e }

func NewSyncService(st *store.Store, cfg *config.Service, creds *CredentialService, instances *InstanceService, reg *tasks.Registry, lg *slog.Logger) *SyncService {
	return &SyncService{
		store: st, cfg: cfg, creds: creds, instances: instances, registry: reg, log: lg,
		apiFor: func(ctx context.Context, id int64) (API, error) {
			return instances.ClientFor(ctx, id)
		},
	}
}

// SetAPIFactory 供测试注入 fake。
func (s *SyncService) SetAPIFactory(fn func(ctx context.Context, instanceID int64) (API, error)) {
	s.apiFor = fn
}

// Targets 返回用户当前推送目标（组分配 ∪ 公共，经候选集与可用性过滤）。
func (s *SyncService) Targets(ctx context.Context, userID int64) ([]Target, error) {
	return s.targetsDB(ctx, s.store.DB(), userID)
}

func (s *SyncService) targetsDB(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}, userID int64) ([]Target, error) {
	candidates, err := s.candidateNames(ctx, q)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	rows, err := q.QueryContext(ctx,
		`SELECT n.id, n.name, n.display_name, n.tag, n.instance_id, i.api_addr, n.port, n.is_public,
		        COALESCE(gn.sort_order, 999999)
		 FROM nodes n
		 JOIN xray_instances i ON i.id = n.instance_id
		 LEFT JOIN group_nodes gn ON gn.node_id = n.id AND gn.group_id = (SELECT group_id FROM users WHERE id = ?)
		 WHERE n.source = 'xray' AND n.enabled = 1 AND n.allocatable = 1 AND n.missing = 0
		   AND i.enabled = 1 AND (n.is_public = 1 OR gn.group_id IS NOT NULL)
		 ORDER BY CASE WHEN n.is_public = 1 THEN 1 ELSE 0 END, gn.sort_order`, userID)
	if err != nil {
		return nil, fmt.Errorf("读取用户推送目标失败: %w", err)
	}
	defer rows.Close()
	candidateMap := map[string]bool{}
	for _, c := range candidates {
		candidateMap[c] = true
	}
	out := make([]Target, 0)
	for rows.Next() {
		var t Target
		var display sql.NullString
		var isPublic int
		var sortOrder int
		if err := rows.Scan(&t.NodeID, &t.Name, &display, &t.Tag, &t.InstanceID, &t.APIAddr, &t.Port, &isPublic, &sortOrder); err != nil {
			return nil, err
		}
		if !candidateMap[t.Name] {
			continue
		}
		if display.Valid && display.String != "" {
			t.DisplayName = &display.String
			t.RenderName = display.String
		} else {
			t.RenderName = t.Name
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// PushUser 推送单个用户到全部目标。返回 synced/failed 计数。
func (s *SyncService) PushUser(ctx context.Context, userID int64) (int, int, error) {
	if !s.cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		return 0, 0, nil
	}
	var status string
	if err := s.store.DB().QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, userID).Scan(&status); err != nil {
		return 0, 0, err
	}
	if status != "active" {
		return 0, 0, nil
	}
	var exceeded int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT quota_exceeded FROM users WHERE id = ?`, userID).Scan(&exceeded); err != nil {
		return 0, 0, err
	}
	if exceeded == 1 {
		// 超限用户不推送，但保留记录并写原因。
		s.writeQuotaExceededError(ctx, userID)
		return 0, 0, nil
	}
	if err := s.creds.EnsureCredentials(ctx, userID); err != nil {
		if errors.Is(err, ErrAdvancedOff) {
			return 0, 0, nil
		}
		return 0, 0, err
	}
	targets, err := s.Targets(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if len(targets) == 0 {
		return 0, 0, nil
	}
	uuid, secret, err := s.creds.Credentials(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	synced, failed := 0, 0
	for _, t := range targets {
		client, err := s.apiFor(ctx, t.InstanceID)
		if err != nil {
			s.markFailed(ctx, userID, t, err)
			failed++
			continue
		}
		// 写 pending（事务内复查 advanced_mode）
		if err := s.writePending(ctx, userID, t); err != nil {
			if errors.Is(err, ErrAdvancedOff) {
				return synced, failed, nil
			}
			s.markFailed(ctx, userID, t, err)
			failed++
			continue
		}
		nv, err := s.nodeViewForTarget(ctx, t.NodeID)
		if err != nil {
			s.markFailed(ctx, userID, t, err)
			failed++
			continue
		}
		nv.Tag = t.Tag
		user, err := BuildUser(userID, uuid, secret, nv)
		if err != nil {
			s.markFailed(ctx, userID, t, err)
			failed++
			continue
		}
		err = client.AddUser(ctx, t.Tag, user)
		if err == nil && !s.cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
			// AddUser 完成后复查，off 则立即补偿 RemoveUser。
			_ = client.RemoveUser(ctx, t.Tag, UserEmail(userID))
			s.markFailed(ctx, userID, t, errors.New("高级模式已关闭，已补偿移除"))
			failed++
			continue
		}
		if err != nil {
			s.markFailed(ctx, userID, t, err)
			failed++
			continue
		}
		s.markSynced(ctx, userID, t)
		synced++
	}
	return synced, failed, nil
}

// RemoveUserFromTargets 从指定目标移除用户。
func (s *SyncService) RemoveUserFromTargets(ctx context.Context, userID int64, targets []Target) (int, int, error) {
	if !s.cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		return 0, 0, nil
	}
	removed, failed := 0, 0
	for _, t := range targets {
		client, err := s.apiFor(ctx, t.InstanceID)
		if err != nil {
			s.markFailed(ctx, userID, t, err)
			failed++
			continue
		}
		err = client.RemoveUser(ctx, t.Tag, UserEmail(userID))
		if err != nil && !IsNotFound(err) {
			s.markFailed(ctx, userID, t, err)
			failed++
			continue
		}
		if err := s.deleteRecord(ctx, userID, t); err != nil {
			s.log.Warn("删除 xray_users 记录失败", "user", userID, "tag", t.Tag, "err", err)
		}
		removed++
	}
	return removed, failed, nil
}

// DiffPush 按旧新目标集合执行差异推送/移除。
func (s *SyncService) DiffPush(ctx context.Context, userID int64, oldTargets, newTargets []Target) error {
	oldMap := map[int64]Target{}
	newMap := map[int64]Target{}
	for _, t := range oldTargets {
		oldMap[t.NodeID] = t
	}
	for _, t := range newTargets {
		newMap[t.NodeID] = t
	}
	var removeTargets, addTargets []Target
	for id, t := range oldMap {
		if _, ok := newMap[id]; !ok {
			removeTargets = append(removeTargets, t)
		}
	}
	for id, t := range newMap {
		if _, ok := oldMap[id]; !ok {
			addTargets = append(addTargets, t)
		}
	}
	if len(removeTargets) > 0 {
		_, _, _ = s.RemoveUserFromTargets(ctx, userID, removeTargets)
	}
	if len(addTargets) > 0 {
		var status string
		_ = s.store.DB().QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, userID).Scan(&status)
		if status == "active" {
			_, _, _ = s.PushUser(ctx, userID)
		}
	}
	return nil
}

// CollectTargetsTx 在事务内收集用户目标（删除/禁用前使用）。
func (s *SyncService) CollectTargetsTx(ctx context.Context, tx *sql.Tx, userID int64) ([]Target, error) {
	return s.targetsDB(ctx, tx, userID)
}

// StartInit 注册并启动全量初始化任务。
func (s *SyncService) StartInit(ctx context.Context) (string, error) {
	taskID := s.registry.Register(tasks.KindXrayInit)
	bg := context.WithoutCancel(ctx)
	go func() {
		ids, err := s.activeUserIDs(bg)
		if err != nil {
			s.registry.Fail(taskID, err.Error())
			return
		}
		totalSynced, totalFailed := 0, 0
		for _, id := range ids {
			synced, failed, err := s.PushUser(bg, id)
			if err != nil {
				s.registry.Fail(taskID, err.Error())
				return
			}
			totalSynced += synced
			totalFailed += failed
		}
		s.registry.Succeed(taskID, map[string]any{"synced": totalSynced, "failed": totalFailed})
	}()
	return taskID, nil
}

// ReconcileUser 将用户当前 xray_users 记录对齐到期望目标：移除陈旧、补推缺失。
func (s *SyncService) ReconcileUser(ctx context.Context, userID int64) error {
	targets, err := s.Targets(ctx, userID)
	if err != nil {
		return err
	}
	targetSet := map[int64]bool{}
	for _, t := range targets {
		targetSet[t.NodeID] = true
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT instance_id, inbound_tag, node_id FROM xray_users WHERE user_id = ?`, userID)
	if err != nil {
		return err
	}
	var stale []Target
	for rows.Next() {
		var instanceID int64
		var tag string
		var nodeID int64
		if err := rows.Scan(&instanceID, &tag, &nodeID); err != nil {
			_ = rows.Close()
			return err
		}
		if !targetSet[nodeID] {
			stale = append(stale, Target{NodeID: nodeID, InstanceID: instanceID, Tag: tag})
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(stale) > 0 {
		if _, _, err := s.RemoveUserFromTargets(ctx, userID, stale); err != nil {
			return err
		}
	}
	if len(targets) > 0 {
		if _, _, err := s.PushUser(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}

// SyncAllActive 对全部 active 用户执行 ReconcileUser（节点/组变化后简化 diff 使用）。
func (s *SyncService) SyncAllActive(ctx context.Context) error {
	ids, err := s.activeUserIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.ReconcileUser(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// SyncUsersForNodes 仅对受指定节点影响的 active 用户执行 ReconcileUser（精确 diff）。
func (s *SyncService) SyncUsersForNodes(ctx context.Context, nodeIDs []int64) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	ids, err := s.affectedActiveUserIDsForNodes(ctx, nodeIDs)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.ReconcileUser(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *SyncService) affectedActiveUserIDsForNodes(ctx context.Context, nodeIDs []int64) ([]int64, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(nodeIDs)), ",")
	args := make([]any, len(nodeIDs))
	for i, id := range nodeIDs {
		args[i] = id
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT DISTINCT u.id
		 FROM users u
		 WHERE u.status = 'active' AND (
		   EXISTS (SELECT 1 FROM nodes n WHERE n.id IN (`+placeholders+`) AND n.is_public = 1)
		   OR EXISTS (SELECT 1 FROM group_nodes gn WHERE gn.group_id = u.group_id AND gn.node_id IN (`+placeholders+`))
		 )`, append(args, args...)...)
	if err != nil {
		return nil, fmt.Errorf("查询节点影响用户失败: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// UserSyncStatus 返回用户 xray_users 聚合状态。
func (s *SyncService) UserSyncStatus(ctx context.Context, userID int64) ([]map[string]any, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT instance_id, inbound_tag, node_id, sync_status, last_error FROM xray_users WHERE user_id = ? ORDER BY instance_id, inbound_tag`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var instanceID int64
		var tag string
		var nodeID int64
		var status, lastErr string
		if err := rows.Scan(&instanceID, &tag, &nodeID, &status, &lastErr); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"instance_id": instanceID,
			"inbound_tag": tag,
			"node_id":     nodeID,
			"sync_status": status,
			"last_error":  lastErr,
		})
	}
	return out, rows.Err()
}

// RetryUser 重试该用户的 failed 记录：仍在期望集则重推，不在则移除。
func (s *SyncService) RetryUser(ctx context.Context, userID int64) (map[string]any, error) {
	targets, err := s.Targets(ctx, userID)
	if err != nil {
		return nil, err
	}
	targetSet := map[int64]bool{}
	for _, t := range targets {
		targetSet[t.NodeID] = true
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT instance_id, inbound_tag, node_id FROM xray_users WHERE user_id = ? AND sync_status = 'failed'`, userID)
	if err != nil {
		return nil, err
	}
	var removeTargets []Target
	for rows.Next() {
		var instanceID int64
		var tag string
		var nodeID int64
		if err := rows.Scan(&instanceID, &tag, &nodeID); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if !targetSet[nodeID] {
			removeTargets = append(removeTargets, Target{NodeID: nodeID, InstanceID: instanceID, Tag: tag})
		}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	removed, removeFailed, _ := s.RemoveUserFromTargets(ctx, userID, removeTargets)
	synced, failed, err := s.PushUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"synced": synced, "failed": failed, "removed": removed, "remove_failed": removeFailed}, nil
}

// ActiveUserIDs 返回全部 active 用户 ID。
func (s *SyncService) ActiveUserIDs(ctx context.Context) ([]int64, error) {
	return s.activeUserIDs(ctx)
}

func (s *SyncService) activeUserIDs(ctx context.Context) ([]int64, error) {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id FROM users WHERE status = 'active' ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *SyncService) candidateNames(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}) ([]string, error) {
	return candidateNamesFrom(ctx, q)
}

func candidateNamesFrom(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}) ([]string, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT b.selection_json
		 FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 JOIN subscriptions s ON s.id = v.owner_id AND v.owner_type = 'subscription'
		 WHERE s.current_version = v.version_no
		   AND b.target_syntax IN ('clash-yaml','sr-subs','generic-subs')`)
	if err != nil {
		return nil, fmt.Errorf("读取候选集失败: %w", err)
	}
	defer rows.Close()
	var out []string
	seen := map[string]bool{}
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
		for _, name := range sel.XrayCandidates {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	return out, rows.Err()
}

func (s *SyncService) writePending(ctx context.Context, userID int64, t Target) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		advanced, err := s.cfg.GetTx(ctx, tx, config.KeyAdvancedMode)
		if err != nil {
			return err
		}
		if advanced != "true" {
			return ErrAdvancedOff
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO xray_users (user_id, instance_id, inbound_tag, node_id, email, sync_status)
			 VALUES (?,?,?,?,?,'pending')
			 ON CONFLICT(user_id, instance_id, inbound_tag) DO UPDATE SET
			   node_id = excluded.node_id, sync_status = 'pending', last_error = '', updated_at = CURRENT_TIMESTAMP`,
			userID, t.InstanceID, t.Tag, t.NodeID, UserEmail(userID))
		return err
	})
}

func (s *SyncService) markSynced(ctx context.Context, userID int64, t Target) {
	_, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO xray_users (user_id, instance_id, inbound_tag, node_id, email, sync_status)
		 VALUES (?,?,?,?,?,'synced')
		 ON CONFLICT(user_id, instance_id, inbound_tag) DO UPDATE SET
		   node_id = excluded.node_id, sync_status = 'synced', last_error = '', updated_at = CURRENT_TIMESTAMP`,
		userID, t.InstanceID, t.Tag, t.NodeID, UserEmail(userID))
	if err != nil {
		s.log.Warn("更新 xray_users synced 失败", "user", userID, "tag", t.Tag, "err", err)
	}
}

func (s *SyncService) markFailed(ctx context.Context, userID int64, t Target, err error) {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	_, dbErr := s.store.DB().ExecContext(ctx,
		`INSERT INTO xray_users (user_id, instance_id, inbound_tag, node_id, email, sync_status, last_error)
		 VALUES (?,?,?,?,?,'failed',?)
		 ON CONFLICT(user_id, instance_id, inbound_tag) DO UPDATE SET
		   node_id = excluded.node_id, sync_status = 'failed', last_error = excluded.last_error, updated_at = CURRENT_TIMESTAMP`,
		userID, t.InstanceID, t.Tag, t.NodeID, UserEmail(userID), msg)
	if dbErr != nil {
		s.log.Warn("更新 xray_users failed 失败", "user", userID, "tag", t.Tag, "err", dbErr)
	}
}

func (s *SyncService) deleteRecord(ctx context.Context, userID int64, t Target) error {
	_, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM xray_users WHERE user_id = ? AND instance_id = ? AND inbound_tag = ?`,
		userID, t.InstanceID, t.Tag)
	return err
}

func (s *SyncService) writeQuotaExceededError(ctx context.Context, userID int64) {
	// 为已有目标记录写入超限原因；没有记录则跳过。
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT instance_id, inbound_tag, node_id FROM xray_users WHERE user_id = ?`, userID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var instanceID int64
		var tag string
		var nodeID int64
		if err := rows.Scan(&instanceID, &tag, &nodeID); err != nil {
			continue
		}
		_, _ = s.store.DB().ExecContext(ctx,
			`UPDATE xray_users SET sync_status='failed', last_error='已超限，请先重置配额', updated_at=CURRENT_TIMESTAMP
			 WHERE user_id=? AND instance_id=? AND inbound_tag=?`, userID, instanceID, tag)
	}
}

// NodeRenderParams 返回节点协议与明文渲染参数（供下载渲染使用）。
func (s *SyncService) NodeRenderParams(ctx context.Context, nodeID int64) (string, map[string]any, error) {
	var protocol, rawJSON string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT protocol, protocol_json FROM nodes WHERE id = ?`, nodeID).Scan(&protocol, &rawJSON)
	if err != nil {
		return "", nil, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return "", nil, fmt.Errorf("解析节点参数失败: %w", err)
	}
	return protocol, m, nil
}

// nodeViewForTarget 查询节点协议与渲染所需参数。
func (s *SyncService) nodeViewForTarget(ctx context.Context, nodeID int64) (NodeView, error) {
	var protocol, rawJSON string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT protocol, protocol_json FROM nodes WHERE id = ?`, nodeID).Scan(&protocol, &rawJSON)
	if err != nil {
		return NodeView{}, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return NodeView{}, fmt.Errorf("解析节点参数失败: %w", err)
	}
	nv := NodeView{Protocol: protocol}
	if v, ok := m["cipher"].(string); ok {
		nv.Cipher = v
	}
	if v, ok := m["flow"].(string); ok {
		nv.Flow = v
	}
	return nv, nil
}

// AfterAdvancedOff 是 Build7 OFF 清空提交后的补偿辅助：按快照直连 Xray 实例移除 user/ext 账号。
func (s *SyncService) AfterAdvancedOff(ctx context.Context, targets []OffClearTarget) {
	for _, t := range targets {
		if t.APIAddr == "" || t.Tag == "" || t.Email == "" {
			continue
		}
		client, err := Dial(t.APIAddr)
		if err != nil {
			s.log.Warn("OFF 清空后清理 Xray 用户失败（拨号）", "email", t.Email, "addr", t.APIAddr, "err", err)
			continue
		}
		rctx, cancel := context.WithTimeout(ctx, RPCTimeout)
		err = client.RemoveUser(rctx, t.Tag, t.Email)
		cancel()
		_ = client.Close()
		if err != nil && !IsNotFound(err) {
			s.log.Warn("OFF 清空后清理 Xray 用户失败", "email", t.Email, "tag", t.Tag, "err", err)
		}
	}
}

