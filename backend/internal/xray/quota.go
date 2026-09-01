package xray

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

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
