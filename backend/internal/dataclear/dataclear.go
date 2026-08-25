// Package dataclear 提供数据清理服务（Build3 Step 4）：一键清空所有数据（RESET 确认词 + 二次确认）。
// 执行顺序：先清库（事务）再删数据文件；文件删除失败记录错误日志并提示，不阻断回到 Setup 状态（Design1 §4.8）。
package dataclear

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"vpn-sub/internal/store"
)

const ConfirmWordReset = "RESET" // 一键清空确认词（固定，二次确认由前端负责）

// Service 数据清理服务
type Service struct {
	store   *store.Store
	dataDir string
	log     *slog.Logger
	// resetRuntimeState 内存态复位回调（server.New 装配注入）：限流计数、实时日志缓冲等
	resetRuntimeState func()
}

func NewService(st *store.Store, dataDir string, lg *slog.Logger) *Service {
	return &Service{store: st, dataDir: dataDir, log: lg}
}

// SetResetRuntimeState 注入内存态复位回调（Build3 Step 5 追加 SSE 连接与短期 Token 复位）
func (s *Service) SetResetRuntimeState(fn func()) {
	s.resetRuntimeState = fn
}

// ClearTablesTx 清空全部业务数据表 + 系统配置（单事务内；应急重新初始化与一键清空共用，Build3 Step 6）。
// schema_migrations 保留（迁移框架自持）
func (s *Service) ClearTablesTx(ctx context.Context, tx *sql.Tx) error {
	tables := []string{
		// Build4 新增（先子后父，避免外键扫描差异）
		"pool_sync_tasks", "pool_entries", "rule_pools",
		"xray_ext_traffic", "xray_ext_users", "xray_ext_accounts",
		"traffic_records", "xray_users", "group_nodes", "assembly_blueprints",
		"nodes", "xray_instances", "proxy_groups",
		// 既有表（保留）
		"download_tokens", "share_tokens", "rule_tokens", "password_reset_tokens", "oidc_login_tickets", "oidc_states",
		"access_logs", "versions",
		"custom_subscriptions", "share_subscriptions", "rules", "subscriptions",
		"users", "groups", "platforms", "system_config",
	}
	for _, t := range tables {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
			return fmt.Errorf("清空表 %s 失败: %w", t, err)
		}
	}
	return nil
}

// ClearAll 一键清空所有数据——先清库（事务）再删数据文件；内存态复位；
// 旧会话凭据因签名密钥轮换验签失败自然失效；系统回到未配置状态，无需重启
func (s *Service) ClearAll(ctx context.Context, confirmWord string) error {
	if confirmWord != ConfirmWordReset {
		return errors.New("确认词不正确")
	}
	// 1) 清库：单事务删除全部业务数据 + 系统配置（含签名密钥、configured 标记）
	//    地址启动缓存为全清特例——回 Setup 重新推导写入新值
	if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		return s.ClearTablesTx(ctx, tx)
	}); err != nil {
		return err
	}
	// 2) 删数据文件（版本文件目录 + /public 资源）；失败记错误日志并提示，不阻断回 Setup
	var fileErrs []string
	for _, dir := range []string{"contents", "public"} {
		if err := os.RemoveAll(filepath.Join(s.dataDir, dir)); err != nil {
			fileErrs = append(fileErrs, dir)
			s.log.Error("删除数据文件目录失败", "dir", dir, "err", err)
		}
	}
	// 3) 内存态复位：限流计数、实时日志缓冲（SSE 连接与短期 Token 由 Step 5 追加）同步重置
	if s.resetRuntimeState != nil {
		s.resetRuntimeState()
	}
	// 旧会话凭据因签名密钥轮换（configured 清除后重新 Setup 生成新密钥）验签失败自然失效
	s.log.Warn("一键清空所有数据已执行", "file_errors", fileErrs)
	return nil
}
