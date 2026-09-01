package xray

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
)

// ConfirmWordDisable OFF 清空确认词。
const ConfirmWordDisable = "DISABLE"

// OffClearTarget 是 OFF 清空后需要从 Xray 侧移除的账号快照。
type OffClearTarget struct {
	Email   string
	Tag     string
	APIAddr string
}

// OffClearService 处理高级模式关闭/OFF 清空。
type OffClearService struct {
	store    *store.Store
	cfg      *config.Service
	registry *tasks.Registry
	log      *slog.Logger
	// afterOff 提交后 best-effort 清理；由 server.New 接入 SyncService.AfterAdvancedOff。
	afterOff func(ctx context.Context, targets []OffClearTarget)
}

// NewOffClearService 构造 OFF 清空服务。
func NewOffClearService(st *store.Store, cfg *config.Service, reg *tasks.Registry, lg *slog.Logger) *OffClearService {
	return &OffClearService{store: st, cfg: cfg, registry: reg, log: lg}
}

// SetAfterAdvancedOff 注入 OFF 清空提交后的补偿清理函数。
func (s *OffClearService) SetAfterAdvancedOff(fn func(ctx context.Context, targets []OffClearTarget)) {
	s.afterOff = fn
}

// SubmitAdvancedMode 提交高级模式开关。
// on=true 同步置位，不推送；on=false 校验 DISABLE 并异步执行 OFF 清空，返回 task_id。
// 状态翻转与任务登记在同一 BEGIN IMMEDIATE 事务内完成，防止并发重复创建任务。
func (s *OffClearService) SubmitAdvancedMode(ctx context.Context, on bool, confirmWord string) (string, error) {
	current := s.cfg.GetBool(ctx, config.KeyAdvancedMode, false)
	if on {
		if current {
			return "", nil
		}
		if err := s.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
			return "", err
		}
		return "", nil
	}
	// 关闭
	if !current {
		return "", nil // 幂等 no-op
	}
	if confirmWord != ConfirmWordDisable {
		return "", fmt.Errorf("%w: 确认词不正确", config.ErrBadRequest)
	}
	var taskID string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		advanced, err := s.cfg.GetTx(ctx, tx, config.KeyAdvancedMode)
		if err != nil {
			return err
		}
		if advanced != "true" {
			return nil
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE system_config SET value = 'false', updated_at = CURRENT_TIMESTAMP WHERE key = ?`, config.KeyAdvancedMode); err != nil {
			return err
		}
		taskID = s.registry.Register(tasks.KindOffClear)
		return nil
	})
	if err != nil {
		return "", err
	}
	if taskID == "" {
		return "", nil
	}
	bg := context.WithoutCancel(ctx)
	go func() {
		if err := s.offClear(bg); err != nil {
			s.registry.Fail(taskID, err.Error())
			return
		}
		s.registry.Succeed(taskID, map[string]any{"cleared": true})
	}()
	return taskID, nil
}

func (s *OffClearService) offClear(ctx context.Context) error {
	var targets []OffClearTarget
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 状态翻转已在 SubmitAdvancedMode 的同一事务内完成；这里只收集快照并清空数据。
		// 收集面板用户推送记录
		rows, err := tx.QueryContext(ctx,
			`SELECT xu.email, xu.inbound_tag, i.api_addr
			 FROM xray_users xu JOIN xray_instances i ON i.id = xu.instance_id`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var t OffClearTarget
			if err := rows.Scan(&t.Email, &t.Tag, &t.APIAddr); err != nil {
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
			`SELECT a.email, xu.inbound_tag, i.api_addr
			 FROM xray_ext_users xu
			 JOIN xray_ext_accounts a ON a.id = xu.ext_account_id
			 JOIN xray_instances i ON i.id = xu.instance_id`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var t OffClearTarget
			if err := rows.Scan(&t.Email, &t.Tag, &t.APIAddr); err != nil {
				_ = rows.Close()
				return err
			}
			targets = append(targets, t)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		// 清空高级模式数据
		if _, err := tx.ExecContext(ctx, `DELETE FROM xray_instances`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM xray_ext_accounts`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM traffic_records`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM xray_ext_traffic`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET uuid_encrypted = NULL, proxy_secret_encrypted = NULL, quota_override = NULL, quota_exceeded = 0, updated_at = CURRENT_TIMESTAMP`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE groups SET default_quota = NULL, updated_at = CURRENT_TIMESTAMP`); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 提交后 best-effort 清理 Xray 侧账号；优先复用 SyncService.AfterAdvancedOff。
	if s.afterOff != nil {
		s.afterOff(ctx, targets)
		return nil
	}
	// 兜底：未注入时保持原直接清理逻辑。
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
	return nil
}
