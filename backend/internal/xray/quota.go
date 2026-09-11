package xray

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vpn-sub/internal/config"
)

// TrafficSummary 首页/个人中心统一的月流量与配额业务结构体；基础模式恒为不限流量。
// QuotaBytes 为 nil 表示不限（JSON null），非 nil 时为配额换算后的字节数。
type TrafficSummary struct {
	Unlimited  bool   `json:"unlimited"`
	UsedBytes  int64  `json:"used_bytes"`
	QuotaBytes *int64 `json:"quota_bytes"`
	Exceeded   bool   `json:"exceeded"`
}

// TrafficSummaryForUser 汇总用户当月流量、有效配额和超限标记；不感知 Gin/HTTP。
func (s *SyncService) TrafficSummaryForUser(ctx context.Context, userID int64) (*TrafficSummary, error) {
	if !s.cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		return &TrafficSummary{Unlimited: true, UsedBytes: 0, QuotaBytes: nil, Exceeded: false}, nil
	}
	var used int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(uplink+downlink),0) FROM traffic_records WHERE user_id = ? AND ym = ?`,
		userID, currentYM()).Scan(&used); err != nil {
		return nil, err
	}
	quota, err := s.EffectiveQuota(ctx, userID)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return &TrafficSummary{Unlimited: true, UsedBytes: used, QuotaBytes: nil, Exceeded: false}, nil
	}
	quotaBytes := int64(*quota * 1024 * 1024 * 1024)
	var exceeded int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT quota_exceeded FROM users WHERE id = ?`, userID).Scan(&exceeded); err != nil {
		return nil, err
	}
	return &TrafficSummary{Unlimited: false, UsedBytes: used, QuotaBytes: &quotaBytes, Exceeded: exceeded == 1}, nil
}

// EffectiveQuota 返回用户有效配额（GB）；quota_override 优先，否则组默认；NULL/0 返回 nil 表示不限。
func (s *SyncService) EffectiveQuota(ctx context.Context, userID int64) (*float64, error) {
	var quota sql.NullFloat64
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(u.quota_override, g.default_quota)
		 FROM users u LEFT JOIN groups g ON g.id = u.group_id WHERE u.id = ?`, userID).Scan(&quota)
	if err != nil {
		return nil, err
	}
	if !quota.Valid || quota.Float64 <= 0 {
		return nil, nil
	}
	v := quota.Float64
	return &v, nil
}

// CheckQuota 检查用户当月流量是否超限；超限则 RemoveUser 并置 quota_exceeded=1。
func (s *SyncService) CheckQuota(ctx context.Context, userID int64) error {
	quota, err := s.EffectiveQuota(ctx, userID)
	if err != nil {
		return err
	}
	if quota == nil {
		return nil
	}
	var used int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(uplink+downlink),0) FROM traffic_records WHERE user_id = ? AND ym = ?`, userID, currentYM()).Scan(&used); err != nil {
		return err
	}
	if float64(used) <= *quota*1024*1024*1024 {
		return nil
	}
	targets, err := s.Targets(ctx, userID)
	if err != nil {
		return err
	}
	if _, _, err := s.RemoveUserFromTargets(ctx, userID, targets); err != nil {
		return err
	}
	_, err = s.store.DB().ExecContext(ctx,
		`UPDATE users SET quota_exceeded = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, userID)
	return err
}

// ResetQuota 清当月流量并重新推送；仅 active 用户可重置。
func (s *SyncService) ResetQuota(ctx context.Context, userID int64) error {
	var status string
	if err := s.store.DB().QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, userID).Scan(&status); err != nil {
		return err
	}
	if status != "active" {
		return errors.New("仅激活用户可重置配额")
	}
	if _, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM traffic_records WHERE user_id = ? AND ym = ?`, userID, currentYM()); err != nil {
		return fmt.Errorf("清空当月流量失败: %w", err)
	}
	if _, err := s.store.DB().ExecContext(ctx,
		`UPDATE users SET quota_exceeded = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, userID); err != nil {
		return err
	}
	_, _, err := s.PushUser(ctx, userID)
	return err
}
