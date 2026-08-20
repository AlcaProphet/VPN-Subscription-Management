// Package xray 的实例服务：Xray 实例 CRUD、连通性测试与删除异步清理。
package xray

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
)

// 业务错误。
var (
	ErrInstanceNotFound = errors.New("Xray 实例不存在")
	ErrInstanceConflict = errors.New("Xray 实例名称已存在")
	ErrBadRequest       = errors.New("参数错误")
	ErrDisabled         = errors.New("实例已停用，不参与节点检测")
)

// Instance 是 Xray 实例数据模型。
type Instance struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Slug          string     `json:"slug"`
	APIAddr       string     `json:"api_addr"`
	APITag        string     `json:"api_tag"`
	Enabled       bool       `json:"enabled"`
	LastCollectAt *time.Time `json:"last_collect_at,omitempty"`
	CollectStatus string     `json:"collect_status,omitempty"`
	CollectError  string     `json:"collect_error,omitempty"`
}

// InstanceService 提供实例管理。每个实例缓存同一个 Client，保证每实例串行。
type InstanceService struct {
	store    *store.Store
	log      *slog.Logger
	registry *tasks.Registry

	mu      sync.Mutex
	clients map[int64]*Client

	// OnNodeVisibilityChanged 检测后可见性变化回调（Step3 注入）。
	OnNodeVisibilityChanged func(ctx context.Context, changes []NodeChange)
}

// NewInstanceService 构造实例服务。
func NewInstanceService(st *store.Store, lg *slog.Logger, reg *tasks.Registry) *InstanceService {
	return &InstanceService{store: st, log: lg, registry: reg, clients: map[int64]*Client{}}
}

// ClientFor 返回实例对应的缓存客户端；未缓存时创建。
func (s *InstanceService) ClientFor(ctx context.Context, id int64) (*Client, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.clients[id]; ok {
		return c, nil
	}
	inst, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	c, err := Dial(inst.APIAddr)
	if err != nil {
		return nil, err
	}
	s.clients[id] = c
	return c, nil
}

// CloseAll 关闭全部缓存客户端（服务停机时调用）。
func (s *InstanceService) CloseAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, c := range s.clients {
		_ = c.Close()
		delete(s.clients, id)
	}
}

// Create 创建实例。
func (s *InstanceService) Create(ctx context.Context, name, apiAddr, apiTag string, enabled bool) (*Instance, error) {
	if name == "" || apiAddr == "" {
		return nil, fmt.Errorf("%w: 名称与 api_addr 必填", ErrBadRequest)
	}
	if err := ValidateAddr(apiAddr); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	var created *Instance
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_instances WHERE name = ?`, name).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrInstanceConflict
		}
		value, err := slug.Generate(ctx, tx, "instance-", func(v string) (bool, error) {
			dup, err := slug.TableHasSlug(ctx, tx, "xray_instances", v)
			if err != nil || dup {
				return dup, err
			}
			return slug.ExistsInFourTables(ctx, tx, v)
		})
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO xray_instances (name, slug, api_addr, api_tag, enabled) VALUES (?,?,?,?,?)`,
			name, value, apiAddr, apiTag, boolInt(enabled))
		if err != nil {
			return fmt.Errorf("创建 Xray 实例失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Instance{ID: id, Name: name, Slug: value, APIAddr: apiAddr, APITag: apiTag, Enabled: enabled}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("Xray 实例已创建", "id", created.ID, "slug", created.Slug)
	return created, nil
}

// Update 更新实例。enabled 变化只刷新内存可见状态，不触发候选集重算与同步 diff。
func (s *InstanceService) Update(ctx context.Context, id int64, name, apiAddr, apiTag string, enabled bool) (*Instance, error) {
	if name == "" || apiAddr == "" {
		return nil, fmt.Errorf("%w: 名称与 api_addr 必填", ErrBadRequest)
	}
	if err := ValidateAddr(apiAddr); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_instances WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrInstanceNotFound
		}
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_instances WHERE name = ? AND id != ?`, name, id).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrInstanceConflict
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE xray_instances SET name = ?, api_addr = ?, api_tag = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			name, apiAddr, apiTag, boolInt(enabled), id); err != nil {
			return fmt.Errorf("更新 Xray 实例失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 地址变化时丢弃旧客户端，下次使用时重建。
	s.mu.Lock()
	if old, ok := s.clients[id]; ok {
		_ = old.Close()
		delete(s.clients, id)
	}
	s.mu.Unlock()
	got, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return got, nil
}

// Get 返回实例详情。
func (s *InstanceService) Get(ctx context.Context, id int64) (*Instance, error) {
	inst, err := s.getRaw(ctx, id)
	if err != nil {
		return nil, err
	}
	return &inst, nil
}

// List 返回全部实例。
func (s *InstanceService) List(ctx context.Context) ([]Instance, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, name, slug, api_addr, api_tag, enabled, last_collect_at, collect_status, collect_error
		 FROM xray_instances ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("读取 Xray 实例列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Instance, 0)
	for rows.Next() {
		inst, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inst)
	}
	return out, rows.Err()
}

// DeleteAsync 异步删除实例：先登记任务，事务内收集清理目标并删除，提交后 best-effort RemoveUser。
func (s *InstanceService) DeleteAsync(ctx context.Context, id int64) (string, error) {
	taskID := s.registry.Register(tasks.KindInstanceDelete)
	go func() {
		err := s.deleteAndClean(ctx, id)
		if err != nil {
			s.registry.Fail(taskID, err.Error())
			return
		}
		s.registry.Succeed(taskID, map[string]any{"id": id})
	}()
	return taskID, nil
}

func (s *InstanceService) deleteAndClean(ctx context.Context, id int64) error {
	type target struct {
		Email    string
		Instance string
		Tag      string
		APIAddr  string
	}
	var targets []target
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 收集面板用户推送记录
		rows, err := tx.QueryContext(ctx,
			`SELECT xu.email, xu.instance_id, xu.inbound_tag, i.api_addr
			 FROM xray_users xu JOIN xray_instances i ON i.id = xu.instance_id
			 WHERE xu.instance_id = ?`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var t target
			if err := rows.Scan(&t.Email, &t.Instance, &t.Tag, &t.APIAddr); err != nil {
				_ = rows.Close()
				return err
			}
			targets = append(targets, t)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		// 收集独立账号推送记录
		rows, err = tx.QueryContext(ctx,
			`SELECT a.email, xu.instance_id, xu.inbound_tag, i.api_addr
			 FROM xray_ext_users xu
			 JOIN xray_ext_accounts a ON a.id = xu.ext_account_id
			 JOIN xray_instances i ON i.id = xu.instance_id
			 WHERE xu.instance_id = ?`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var t target
			if err := rows.Scan(&t.Email, &t.Instance, &t.Tag, &t.APIAddr); err != nil {
				_ = rows.Close()
				return err
			}
			targets = append(targets, t)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		// Build6-2 补强：按“受影响 active 用户 × 当前期望目标集”收集面板用户清理目标，
		// 不依赖 xray_users 是否已有记录；同时按候选集过滤。
		candidates, err := candidateNamesFrom(ctx, tx)
		if err != nil {
			return err
		}
		candidateSet := map[string]bool{}
		for _, c := range candidates {
			candidateSet[c] = true
		}
		if len(candidateSet) > 0 {
			rows, err = tx.QueryContext(ctx,
				`SELECT 'user-' || u.id || '@vpn.local', i.id, n.tag, i.api_addr, n.name
				 FROM users u
				 JOIN group_nodes gn ON gn.group_id = u.group_id
				 JOIN nodes n ON n.id = gn.node_id
				 JOIN xray_instances i ON i.id = n.instance_id
				 WHERE u.status = 'active' AND i.id = ? AND n.source = 'xray'
				   AND n.enabled = 1 AND n.allocatable = 1 AND n.missing = 0 AND i.enabled = 1
				 UNION
				 SELECT 'user-' || u.id || '@vpn.local', i.id, n.tag, i.api_addr, n.name
				 FROM users u
				 JOIN nodes n ON n.is_public = 1
				 JOIN xray_instances i ON i.id = n.instance_id
				 WHERE u.status = 'active' AND i.id = ? AND n.source = 'xray'
				   AND n.enabled = 1 AND n.allocatable = 1 AND n.missing = 0 AND i.enabled = 1`, id, id)
			if err != nil {
				return err
			}
			for rows.Next() {
				var t target
				var name string
				if err := rows.Scan(&t.Email, &t.Instance, &t.Tag, &t.APIAddr, &name); err != nil {
					_ = rows.Close()
					return err
				}
				if !candidateSet[name] {
					continue
				}
				targets = append(targets, t)
			}
			if err := rows.Close(); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM xray_instances WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除 Xray 实例失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 提交后 best-effort 清理；实例已删，按收集到的 api_addr 逐个拨号清理。
	for _, t := range targets {
		if t.APIAddr == "" {
			continue
		}
		client, err := Dial(t.APIAddr)
		if err != nil {
			s.log.Warn("删除实例后清理 Xray 用户失败（拨号）", "email", t.Email, "addr", t.APIAddr, "err", err)
			continue
		}
		ctxTimeout, cancel := context.WithTimeout(ctx, RPCTimeout)
		err = client.RemoveUser(ctxTimeout, t.Tag, t.Email)
		cancel()
		_ = client.Close()
		if err != nil && !IsNotFound(err) {
			s.log.Warn("删除实例后清理 Xray 用户失败", "email", t.Email, "tag", t.Tag, "err", err)
		}
	}
	s.mu.Lock()
	if old, ok := s.clients[id]; ok {
		_ = old.Close()
		delete(s.clients, id)
	}
	s.mu.Unlock()
	return nil
}

// TestConnection 拨号并 ListInbounds，返回是否可达；不落库。
func (s *InstanceService) TestConnection(ctx context.Context, apiAddr string) error {
	if err := ValidateAddr(apiAddr); err != nil {
		return err
	}
	client, err := Dial(apiAddr)
	if err != nil {
		return err
	}
	defer client.Close()
	probeCtx, cancel := context.WithTimeout(ctx, DialTimeout)
	defer cancel()
	_, err = client.ListInbounds(probeCtx)
	return err
}

// getRaw 读取实例原始行。
func (s *InstanceService) getRaw(ctx context.Context, id int64) (Instance, error) {
	row := s.store.DB().QueryRowContext(ctx,
		`SELECT id, name, slug, api_addr, api_tag, enabled, last_collect_at, collect_status, collect_error
		 FROM xray_instances WHERE id = ?`, id)
	inst, err := scanInstance(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Instance{}, ErrInstanceNotFound
	}
	if err != nil {
		return Instance{}, err
	}
	return inst, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanInstance(row rowScanner) (Instance, error) {
	var inst Instance
	var enabled int
	var last sql.NullTime
	var collectStatus, collectError sql.NullString
	if err := row.Scan(&inst.ID, &inst.Name, &inst.Slug, &inst.APIAddr, &inst.APITag, &enabled, &last, &collectStatus, &collectError); err != nil {
		return Instance{}, err
	}
	inst.Enabled = enabled == 1
	if last.Valid {
		inst.LastCollectAt = &last.Time
	}
	if collectStatus.Valid {
		inst.CollectStatus = collectStatus.String
	}
	if collectError.Valid {
		inst.CollectError = collectError.String
	}
	return inst, nil
}

// hostFromAddr 取 api_addr 的 host 部分，作为 xray 节点 host 来源。
func hostFromAddr(apiAddr string) string {
	host, _, err := net.SplitHostPort(apiAddr)
	if err != nil {
		return apiAddr
	}
	return host
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
