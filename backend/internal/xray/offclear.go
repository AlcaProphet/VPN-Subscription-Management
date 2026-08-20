package xray

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
)

// ConfirmWordDisable OFF 清空确认词。
const ConfirmWordDisable = "DISABLE"

// OffClearService 处理高级模式关闭/OFF 清空。
type OffClearService struct {
	store    *store.Store
	cfg      *config.Service
	registry *tasks.Registry
	log      *slog.Logger
}

// NewOffClearService 构造 OFF 清空服务。
func NewOffClearService(st *store.Store, cfg *config.Service, reg *tasks.Registry, lg *slog.Logger) *OffClearService {
	return &OffClearService{store: st, cfg: cfg, registry: reg, log: lg}
}

// SubmitAdvancedMode 提交高级模式开关。
// on=true 同步置位，不推送；on=false 校验 DISABLE 并异步执行 OFF 清空，返回 task_id。
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
		return "", errors.New("确认词不正确")
	}
	taskID := s.registry.Register(tasks.KindOffClear)
	go func() {
		if err := s.offClear(ctx); err != nil {
			s.registry.Fail(taskID, err.Error())
			return
		}
		s.registry.Succeed(taskID, map[string]any{"cleared": true})
	}()
	return taskID, nil
}

func (s *OffClearService) offClear(ctx context.Context) error {
	type target struct {
		Email   string
		Tag     string
		APIAddr string
	}
	var targets []target
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 状态翻转判定在事务内再做一次，防止并发重复执行。
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
		// 收集面板用户推送记录
		rows, err := tx.QueryContext(ctx,
			`SELECT xu.email, xu.inbound_tag, i.api_addr
			 FROM xray_users xu JOIN xray_instances i ON i.id = xu.instance_id`)
		if err != nil {
			return err
		}
		for rows.Next() {
			var t target
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
			var t target
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
	// 提交后 best-effort 清理 Xray 侧账号
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
