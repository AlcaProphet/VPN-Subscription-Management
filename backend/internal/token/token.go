// Package token 提供下载 Token 业务层：三态复用键先查后建、刷新轮替与生命周期联动。
package token

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"

	"vpn-sub/internal/store"
)

// 业务错误（接入层映射 HTTP 状态码）
var ErrTokenNotFound = errors.New("Token 不存在")

// Service Token 服务
type Service struct {
	store *store.Store
	log   *slog.Logger
}

func NewService(st *store.Store, lg *slog.Logger) *Service {
	return &Service{store: st, log: lg}
}

// generate ≥128 位加密安全随机值（32 字节 = 256 位，base64url）
func generate() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成 Token 失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// UserToken 用户下载 Token（三态）
type UserToken struct {
	ID             int64  `json:"id"`
	Token          string `json:"token"`
	UserID         int64  `json:"user_id"`
	PlatformID     int64  `json:"platform_id"`
	CustomSubID    int64  `json:"custom_sub_id"`   // 0 = NULL
	SubscriptionID int64  `json:"subscription_id"` // 0 = NULL
}

// GetOrCreateUserToken 并发首建——单个 BEGIN IMMEDIATE 事务内先查后建，复用键命中即复用（Design1 §4.2）。
// 复用键：无标识 user+platform；自定义 user+platform+custom_sub_id；显式 user+platform+subscription_id。
// 新业务路径优先使用 ResolveUserToken（按自定义/组激活状态协调并清理历史双 Token）；本方法保留给显式
// Token 和底层复用键调用方。
func (s *Service) GetOrCreateUserToken(ctx context.Context, userID, platformID, customSubID, subscriptionID int64) (*UserToken, error) {
	if customSubID != 0 && subscriptionID != 0 {
		return nil, errors.New("custom_sub_id 与 subscription_id 互斥")
	}
	var t *UserToken
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var err error
		t, err = getOrCreateUserTokenTx(ctx, tx, userID, platformID, customSubID, subscriptionID)
		return err
	})
	return t, err
}

// getOrCreateUserTokenTx 在调用方事务内按复用键先查后建（NULL 语义：可选标识为 0 时匹配 IS NULL）。
func getOrCreateUserTokenTx(ctx context.Context, tx *sql.Tx, userID, platformID, customSubID, subscriptionID int64) (*UserToken, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT id, token, user_id, platform_id,
		        COALESCE(custom_sub_id,0), COALESCE(subscription_id,0)
		 FROM download_tokens
		 WHERE user_id = ? AND platform_id = ?
		   AND COALESCE(custom_sub_id,0) = ? AND COALESCE(subscription_id,0) = ?`,
		userID, platformID, customSubID, subscriptionID)
	var found UserToken
	if err := row.Scan(&found.ID, &found.Token, &found.UserID, &found.PlatformID, &found.CustomSubID, &found.SubscriptionID); err == nil {
		return &found, nil // 复用键命中 → 复用既有 Token
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	// 后建（冲突重试兜底：UNIQUE(token) 失败时重新生成）
	for attempt := 0; attempt < 3; attempt++ {
		value, err := generate()
		if err != nil {
			return nil, err
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES (?,?,?,?,?)`,
			value, userID, platformID, nullIf0(customSubID), nullIf0(subscriptionID))
		if err == nil {
			return &UserToken{Token: value, UserID: userID, PlatformID: platformID, CustomSubID: customSubID, SubscriptionID: subscriptionID}, nil
		}
	}
	return nil, errors.New("Token 创建冲突超过重试上限")
}

// resolveApplicableTokenKeyTx 在事务内确定当前用户/平台的唯一适用业务键：
// 自定义订阅存在时返回 custom_sub_id；否则平台有激活版本时返回组 Token（0/true）；两者都无返回 0/false。
func resolveApplicableTokenKeyTx(ctx context.Context, tx *sql.Tx, userID, platformID int64) (customSubID int64, group bool, err error) {
	err = tx.QueryRowContext(ctx,
		`SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`,
		userID, platformID).Scan(&customSubID)
	if err == nil {
		return customSubID, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return 0, false, err
	}
	var currentVersion int64
	err = tx.QueryRowContext(ctx,
		`SELECT COALESCE(current_version,0) FROM subscriptions WHERE platform_id = ?`,
		platformID).Scan(&currentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return 0, currentVersion > 0, nil
}

// cleanupNonApplicableUserTokensTx 清理同用户/平台下已不适用业务键的用户 Token：
// 保留显式订阅 Token（subscription_id IS NOT NULL），只删无标识组 Token 或指定自定义 Token 之外的旧记录。
func cleanupNonApplicableUserTokensTx(ctx context.Context, tx *sql.Tx, userID, platformID, customSubID int64) error {
	if customSubID != 0 {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM download_tokens
			 WHERE user_id = ? AND platform_id = ? AND subscription_id IS NULL
			   AND (custom_sub_id IS NULL OR custom_sub_id <> ?)`,
			userID, platformID, customSubID)
		return err
	}
	_, err := tx.ExecContext(ctx,
		`DELETE FROM download_tokens
		 WHERE user_id = ? AND platform_id = ? AND subscription_id IS NULL AND custom_sub_id IS NOT NULL`,
		userID, platformID)
	return err
}

// ResolveUserToken 按业务键原子解析用户下载 Token（Design1 §2.3/§4.2）：
// 自定义订阅优先；存在自定义时只创建/复用自定义 Token，并清理历史无标识组 Token；
// 无自定义且平台有激活版本时创建/复用组 Token；两者都不存在时清理孤儿自定义 Token 并返回 nil。
func (s *Service) ResolveUserToken(ctx context.Context, userID, platformID int64) (*UserToken, error) {
	var out *UserToken
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		customSubID, group, err := resolveApplicableTokenKeyTx(ctx, tx, userID, platformID)
		if err != nil {
			return err
		}
		if customSubID == 0 && !group {
			return cleanupNonApplicableUserTokensTx(ctx, tx, userID, platformID, 0)
		}
		if err := cleanupNonApplicableUserTokensTx(ctx, tx, userID, platformID, customSubID); err != nil {
			return err
		}
		out, err = getOrCreateUserTokenTx(ctx, tx, userID, platformID, customSubID, 0)
		return err
	})
	return out, err
}

// RefreshUserTokenByBusinessKey 按业务键原子轮替用户下载 Token：自定义优先、组回退；
// 同一事务内清理历史双 Token 后只保留/重建当前适用 Token。无适用业务内容返回 ErrTokenNotFound。
func (s *Service) RefreshUserTokenByBusinessKey(ctx context.Context, userID, platformID int64) (*UserToken, error) {
	var out *UserToken
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		customSubID, group, err := resolveApplicableTokenKeyTx(ctx, tx, userID, platformID)
		if err != nil {
			return err
		}
		if customSubID == 0 && !group {
			if err := cleanupNonApplicableUserTokensTx(ctx, tx, userID, platformID, 0); err != nil {
				return err
			}
			return ErrTokenNotFound
		}
		if err := cleanupNonApplicableUserTokensTx(ctx, tx, userID, platformID, customSubID); err != nil {
			return err
		}
		// 同一业务键若有历史重复行先全部清理，保证轮替后至多一条。
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM download_tokens
			 WHERE user_id = ? AND platform_id = ? AND subscription_id IS NULL AND COALESCE(custom_sub_id,0) = ?`,
			userID, platformID, customSubID); err != nil {
			return err
		}
		value, err := generate()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES (?,?,?,?,NULL)`,
			value, userID, platformID, nullIf0(customSubID)); err != nil {
			return err
		}
		out = &UserToken{Token: value, UserID: userID, PlatformID: platformID, CustomSubID: customSubID}
		return nil
	})
	return out, err
}

// --- 生命周期联动（Design1 §4.2，全部物理删除，无标记态）---

// DeleteGroupTokens 上传/覆盖自定义订阅 → 删该用户在该平台无标识（组解析）Token
func (s *Service) DeleteGroupTokens(ctx context.Context, userID, platformID int64) error {
	_, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM download_tokens WHERE user_id = ? AND platform_id = ? AND custom_sub_id IS NULL AND subscription_id IS NULL`,
		userID, platformID)
	return err
}

// DeleteBySubscriptionTx 删订阅 → 级联删指向它的 Token（含显式预览 Token），在删订阅事务内调用
func (s *Service) DeleteBySubscriptionTx(ctx context.Context, tx *sql.Tx, subscriptionID int64) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE subscription_id = ?`, subscriptionID)
	return err
}

// DeleteByCustomTx 删自定义 → 删 custom_sub_id 指向的 Token
func (s *Service) DeleteByCustomTx(ctx context.Context, tx *sql.Tx, customID int64) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE custom_sub_id = ?`, customID)
	return err
}

// DeleteExplicit 角色降级（admin→user）→ 清全部显式 Token（Build3 用户管理调用）
func (s *Service) DeleteExplicit(ctx context.Context, userID int64) error {
	_, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM download_tokens WHERE user_id = ? AND subscription_id IS NOT NULL`, userID)
	return err
}

// DeleteAllForUserTx 删除用户/禁用用户 → 物理删全部 Token（禁用时与 credential_version 递增同一事务，Build3 调用）
func (s *Service) DeleteAllForUserTx(ctx context.Context, tx *sql.Tx, userID int64) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, userID)
	return err
}

// RefreshUserToken 轮替（旧失效新生效）——同事务删旧建新，复用键不变
func (s *Service) RefreshUserToken(ctx context.Context, tokenValue string) (*UserToken, error) {
	var t *UserToken
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var rec UserToken
		if err := tx.QueryRowContext(ctx,
			`SELECT id, user_id, platform_id, COALESCE(custom_sub_id,0), COALESCE(subscription_id,0)
			 FROM download_tokens WHERE token = ?`, tokenValue).
			Scan(&rec.ID, &rec.UserID, &rec.PlatformID, &rec.CustomSubID, &rec.SubscriptionID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrTokenNotFound
			}
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE id = ?`, rec.ID); err != nil {
			return err
		}
		// 同复用键重建（直接 INSERT，事务内无并发）
		value, err := generate()
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES (?,?,?,?,?)`,
			value, rec.UserID, rec.PlatformID, nullIf0(rec.CustomSubID), nullIf0(rec.SubscriptionID)); err != nil {
			return err
		}
		t = &UserToken{Token: value, UserID: rec.UserID, PlatformID: rec.PlatformID, CustomSubID: rec.CustomSubID, SubscriptionID: rec.SubscriptionID}
		return nil
	})
	return t, err
}

// FindByToken 按 Token 值查记录（home 刷新等场景用）
func (s *Service) FindByToken(ctx context.Context, tokenValue string) (*UserToken, error) {
	var t UserToken
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, token, user_id, platform_id, COALESCE(custom_sub_id,0), COALESCE(subscription_id,0)
		 FROM download_tokens WHERE token = ?`, tokenValue).
		Scan(&t.ID, &t.Token, &t.UserID, &t.PlatformID, &t.CustomSubID, &t.SubscriptionID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// --- 分享/规则 Token（Step 5/6 使用）：创建时自动生成；刷新=物理轮替（旧删新写同事务）；吊销=物理删除 ---

// CreateShareTokenTx 创建分享 Token（分享创建事务内调用）
func (s *Service) CreateShareTokenTx(ctx context.Context, tx *sql.Tx, shareID int64) (string, error) {
	value, err := generate()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO share_tokens (token, share_id) VALUES (?,?)`, value, shareID); err != nil {
		return "", err
	}
	return value, nil
}

// RotateShareTokenTx 轮替分享 Token（旧删新写同事务；RevokeToken 恢复场景共用）
func (s *Service) RotateShareTokenTx(ctx context.Context, tx *sql.Tx, shareID int64) (string, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM share_tokens WHERE share_id = ?`, shareID); err != nil {
		return "", err
	}
	return s.CreateShareTokenTx(ctx, tx, shareID)
}

// CreateRuleTokenTx 创建规则 Token（规则创建事务内调用）
func (s *Service) CreateRuleTokenTx(ctx context.Context, tx *sql.Tx, ruleID int64) (string, error) {
	value, err := generate()
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO rule_tokens (token, rule_id) VALUES (?,?)`, value, ruleID); err != nil {
		return "", err
	}
	return value, nil
}

// RotateRuleTokenTx 轮替规则 Token（旧删新写同事务 + 刷新 refreshed_at）
func (s *Service) RotateRuleTokenTx(ctx context.Context, tx *sql.Tx, ruleID int64) (string, error) {
	if _, err := tx.ExecContext(ctx, `DELETE FROM rule_tokens WHERE rule_id = ?`, ruleID); err != nil {
		return "", err
	}
	value, err := s.CreateRuleTokenTx(ctx, tx, ruleID)
	if err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE rule_tokens SET refreshed_at = CURRENT_TIMESTAMP WHERE rule_id = ?`, ruleID); err != nil {
		return "", err
	}
	return value, nil
}

// RotateRuleToken 轮替规则 Token（独立事务包装，Step 6 规则服务调用）
func (s *Service) RotateRuleToken(ctx context.Context, ruleID int64) (string, error) {
	var value string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var err error
		value, err = s.RotateRuleTokenTx(ctx, tx, ruleID)
		return err
	})
	return value, err
}

// nullIf0 0 → NULL（可选标识为 0 时存 NULL，配合三态复用键语义）
func nullIf0(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}
