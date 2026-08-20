// Package share 提供分享订阅业务层：创建/改名/版本管理/Token 刷新与吊销（吊销矩阵）。
package share

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrBadRequest = errors.New("参数错误")
	ErrNotFound   = errors.New("分享订阅不存在")
)

// Service 分享订阅服务
type Service struct {
	store    *store.Store
	versions *version.Service
	tokens   *token.Service
	log      *slog.Logger
}

func NewService(st *store.Store, versions *version.Service, tokens *token.Service, lg *slog.Logger) *Service {
	return &Service{store: st, versions: versions, tokens: tokens, log: lg}
}

// Share 分享订阅
type Share struct {
	ID             int64      `json:"id"`
	Slug           string     `json:"slug"`
	Name           string     `json:"name"`
	TokenStatus    string     `json:"token_status"` // active/revoked
	Token          string     `json:"token"`        // 有效时返回（吊销后为空）
	CurrentVersion int64      `json:"current_version"`
	CreatedAt      *time.Time `json:"created_at"` // UTC RFC3339；空值 null（R07-04）
}

// Create 名称 + 首版本上传 → 自动生成标识（share- 前缀）与分享 Token（同一事务语义）
func (s *Service) Create(ctx context.Context, name string, src version.ContentProvider) (*Share, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: 名称必填", ErrBadRequest)
	}
	var sh *Share
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		value, err := slug.Generate(ctx, tx, "share-", func(v string) (bool, error) {
			return slug.ExistsInFourTables(ctx, tx, v) // rules 表 Step 6 才建，缺失则跳过
		})
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO share_subscriptions (slug, name) VALUES (?,?)`, value, name)
		if err != nil {
			return fmt.Errorf("创建分享订阅失败: %w", err)
		}
		id, _ := res.LastInsertId()
		tk, err := s.tokens.CreateShareTokenTx(ctx, tx, id) // 创建时自动生成 Token
		if err != nil {
			return err
		}
		sh = &Share{ID: id, Slug: value, Name: name, TokenStatus: "active", Token: tk}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 首版本创建（版本组件事务）；失败回滚分享记录与 Token（失败清理模式）
	v, _, err := s.versions.CreateVersion(ctx, version.OwnerShare, sh.ID, src, version.CreateOptions{Activate: true})
	if err != nil {
		s.rollbackRecord(ctx, sh.ID) // DELETE share_tokens + share_subscriptions
		return nil, err
	}
	sh.CurrentVersion = v.No
	return sh, nil
}

// rollbackRecord 首版本创建失败时回滚分享记录与 Token
func (s *Service) rollbackRecord(ctx context.Context, id int64) {
	if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx, `DELETE FROM share_tokens WHERE share_id = ?`, id)
		_, err := tx.ExecContext(ctx, `DELETE FROM share_subscriptions WHERE id = ?`, id)
		return err
	}); err != nil {
		s.log.Error("回滚分享创建失败", "id", id, "err", err)
	}
}

// Rename 创建后仅可改名
func (s *Service) Rename(ctx context.Context, id int64, name string) error {
	if name == "" {
		return fmt.Errorf("%w: 名称必填", ErrBadRequest)
	}
	res, err := s.store.DB().ExecContext(ctx,
		`UPDATE share_subscriptions SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("改名失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RefreshToken 物理轮替（旧删新写同事务）；同时清除 revoked 标记并新建 Token（恢复手段，Design1 §3.4.3）
func (s *Service) RefreshToken(ctx context.Context, id int64) (string, error) {
	var newToken string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		tk, err := s.tokens.RotateShareTokenTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE share_subscriptions SET token_status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			return err
		}
		newToken = tk
		return nil
	})
	if err != nil {
		return "", err
	}
	return newToken, nil
}

// RevokeToken 物理删除 Token 记录 + 置 token_status=revoked；链接立即失效，文件与版本保留
func (s *Service) RevokeToken(ctx context.Context, id int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM share_tokens WHERE share_id = ?`, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE share_subscriptions SET token_status = 'revoked', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
}

// Delete 级联删版本文件 + Token
func (s *Service) Delete(ctx context.Context, id int64) error {
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM share_subscriptions WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM share_tokens WHERE share_id = ?`, id); err != nil {
			return err
		}
		if err := s.versions.DeleteVersionsTx(ctx, tx, version.OwnerShare, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM share_subscriptions WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.versions.RemoveOwnerDir(version.OwnerShare, id); err != nil { // 事务提交后删版本目录
		s.log.Warn("删除分享版本目录失败", "id", id, "err", err)
	}
	return nil
}

// List 分享列表（Token 值仅 active 时返回）
func (s *Service) List(ctx context.Context) ([]Share, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT s.id, s.slug, s.name, s.token_status, COALESCE(s.current_version,0), s.created_at,
		        COALESCE((SELECT st.token FROM share_tokens st WHERE st.share_id = s.id LIMIT 1), '')
		 FROM share_subscriptions s ORDER BY s.id`)
	if err != nil {
		return nil, fmt.Errorf("读取分享列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Share, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for rows.Next() {
		var sh Share
		var created sql.NullTime
		if err := rows.Scan(&sh.ID, &sh.Slug, &sh.Name, &sh.TokenStatus, &sh.CurrentVersion, &created, &sh.Token); err != nil {
			return nil, err
		}
		if created.Valid {
			sh.CreatedAt = &created.Time
		}
		if sh.TokenStatus != "active" {
			sh.Token = "" // 吊销后不返回 Token
		}
		out = append(out, sh)
	}
	return out, rows.Err()
}

// Get 单个分享
func (s *Service) Get(ctx context.Context, id int64) (*Share, error) {
	var sh Share
	var created sql.NullTime
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, slug, name, token_status, COALESCE(current_version,0), created_at FROM share_subscriptions WHERE id = ?`, id).
		Scan(&sh.ID, &sh.Slug, &sh.Name, &sh.TokenStatus, &sh.CurrentVersion, &created)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if created.Valid {
		sh.CreatedAt = &created.Time
	}
	return &sh, nil
}
