// Package custom 提供自定义订阅业务层：管理员为特定用户+平台上传/覆盖订阅，覆盖组分配。
package custom

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrBadRequest = errors.New("参数错误")
	ErrNotFound   = errors.New("自定义订阅不存在")
)

// Service 自定义订阅服务
type Service struct {
	store    *store.Store
	versions *version.Service
	tokens   *token.Service
	log      *slog.Logger
}

func NewService(st *store.Store, versions *version.Service, tokens *token.Service, lg *slog.Logger) *Service {
	return &Service{store: st, versions: versions, tokens: tokens, log: lg}
}

// Custom 自定义订阅
type Custom struct {
	ID             int64  `json:"id"`
	Slug           string `json:"slug"`
	UserID         int64  `json:"user_id"`
	PlatformID     int64  `json:"platform_id"`
	PlatformName   string `json:"platform_name,omitempty"`
	CurrentVersion int64  `json:"current_version"`
}

// Upsert 上传/覆盖——每用户每平台最多一份，再次上传即覆盖：
// 复用原记录与标识，仅创建新版本（Token 复用键 user+platform+custom_sub_id 保持稳定，Design1 §2.3）
func (s *Service) Upsert(ctx context.Context, userID, platformID int64, src version.ContentProvider) (*Custom, error) {
	if userID <= 0 || platformID <= 0 {
		return nil, fmt.Errorf("%w: 用户与平台必填", ErrBadRequest)
	}
	var c *Custom
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var plat int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM platforms WHERE id = ?`, platformID).Scan(&plat); err != nil {
			return err
		}
		if plat == 0 {
			return ErrBadRequest // 平台不存在
		}
		var id int64
		err := tx.QueryRowContext(ctx,
			`SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, platformID).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			// 首次创建：生成标识（custom- 前缀，参与四类命名空间校验；rules 表 Step 6 才建，缺失则跳过）
			value, err := slug.Generate(ctx, tx, "custom-", func(v string) (bool, error) {
				return slug.ExistsInFourTables(ctx, tx, v)
			})
			if err != nil {
				return err
			}
			res, err := tx.ExecContext(ctx,
				`INSERT INTO custom_subscriptions (slug, user_id, platform_id) VALUES (?,?,?)`, value, userID, platformID)
			if err != nil {
				return fmt.Errorf("创建自定义订阅失败: %w", err)
			}
			id, _ = res.LastInsertId()
			c = &Custom{ID: id, Slug: value, UserID: userID, PlatformID: platformID}
		} else if err != nil {
			return err
		} else {
			var slugVal string
			if err := tx.QueryRowContext(ctx, `SELECT slug FROM custom_subscriptions WHERE id = ?`, id).Scan(&slugVal); err != nil {
				return err
			}
			c = &Custom{ID: id, Slug: slugVal, UserID: userID, PlatformID: platformID}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 版本创建在独立事务（版本组件自带 BEGIN IMMEDIATE）；失败需回滚首次创建的空记录
	v, _, err := s.versions.CreateVersion(ctx, version.OwnerCustom, c.ID, src, version.CreateOptions{Activate: true})
	if err != nil {
		s.rollbackEmptyRecord(ctx, c.ID)
		return nil, err
	}
	c.CurrentVersion = v.No
	// 上传/覆盖后：删该用户在该平台原有的无标识（组解析）Token（旧链接立即失效，Design1 §2.3）
	if err := s.tokens.DeleteGroupTokens(ctx, userID, platformID); err != nil {
		s.log.Error("删除无标识 Token 失败", "err", err) // 不阻断，记日志
	}
	return c, nil
}

// rollbackEmptyRecord 首建且版本创建失败时删空记录（失败清理模式）
func (s *Service) rollbackEmptyRecord(ctx context.Context, id int64) {
	if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM custom_subscriptions WHERE id = ?`, id)
		return err
	}); err != nil {
		s.log.Error("回滚自定义订阅创建失败", "id", id, "err", err)
	}
}

// Delete 级联删 custom_sub_id 指向的 Token + 版本文件；用户下次访问首页重新生成无标识 Token
func (s *Service) Delete(ctx context.Context, userID, platformID int64) error {
	var id int64
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if err := tx.QueryRowContext(ctx,
			`SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, platformID).Scan(&id); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if err := s.tokens.DeleteByCustomTx(ctx, tx, id); err != nil { // 级联删指向它的 Token
			return err
		}
		if err := s.versions.DeleteVersionsTx(ctx, tx, version.OwnerCustom, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM custom_subscriptions WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.versions.RemoveOwnerDir(version.OwnerCustom, id); err != nil { // 事务提交后删版本目录（失败记日志）
		s.log.Warn("删除自定义订阅版本目录失败", "id", id, "err", err)
	}
	return nil
}

// Get 单个自定义订阅
func (s *Service) Get(ctx context.Context, id int64) (*Custom, error) {
	var c Custom
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, slug, user_id, platform_id, COALESCE(current_version,0) FROM custom_subscriptions WHERE id = ?`, id).
		Scan(&c.ID, &c.Slug, &c.UserID, &c.PlatformID, &c.CurrentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListByUser 用户的自定义订阅列表（Build3 用户管理用）
func (s *Service) ListByUser(ctx context.Context, userID int64) ([]Custom, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT c.id, c.slug, c.user_id, c.platform_id, COALESCE(c.current_version,0), COALESCE(p.name,'')
		 FROM custom_subscriptions c LEFT JOIN platforms p ON p.id = c.platform_id
		 WHERE c.user_id = ? ORDER BY c.id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Custom, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for rows.Next() {
		var c Custom
		if err := rows.Scan(&c.ID, &c.Slug, &c.UserID, &c.PlatformID, &c.CurrentVersion, &c.PlatformName); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
