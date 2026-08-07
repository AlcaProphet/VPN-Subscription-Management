// Package rule 提供分流规则业务层：CRUD、全局共享 Token 与级联删除。
package rule

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"vpn-sub/internal/store"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrBadRequest = errors.New("参数错误")
	ErrNotFound   = errors.New("规则不存在")
)

// Service 规则服务
type Service struct {
	store    *store.Store
	versions *version.Service
	tokens   *token.Service
	subs     *subscription.Service // 复用 CheckSlugAvailable（四类全局标识校验）
	log      *slog.Logger
}

func NewService(st *store.Store, versions *version.Service, tokens *token.Service, subs *subscription.Service, lg *slog.Logger) *Service {
	return &Service{store: st, versions: versions, tokens: tokens, subs: subs, log: lg}
}

// Rule 规则
type Rule struct {
	ID             int64    `json:"id"`
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	ClientType     string   `json:"client_type"`
	Schemes        []string `json:"schemes"`
	Token          string   `json:"token"` // 全局共享 Token（每规则一份，不绑定用户）
	CurrentVersion int64    `json:"current_version"`
	CreatedAt      string   `json:"created_at"`
	RefreshedAt    string   `json:"refreshed_at"`
}

// Create 名称 + 手填标识（跨四类校验）+ 客户端类型 + scheme + 首版本上传；自动生成规则 Token
func (s *Service) Create(ctx context.Context, name, slugVal, clientType string, schemes []string, src version.ContentProvider) (*Rule, error) {
	if name == "" || slugVal == "" {
		return nil, fmt.Errorf("%w: 名称与标识必填", ErrBadRequest)
	}
	if clientType != "shadowrocket" {
		return nil, fmt.Errorf("%w: 客户端类型当前仅支持 shadowrocket", ErrBadRequest)
	}
	ok, err := s.subs.CheckSlugAvailable(ctx, slugVal, "", 0)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, subscription.ErrSlugConflict // 409
	}
	var r *Rule
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO rules (slug, name, client_type, schemes) VALUES (?,?,?,?)`,
			slugVal, name, clientType, toJSON(schemes))
		if err != nil {
			return fmt.Errorf("创建规则失败: %w", err)
		}
		id, _ := res.LastInsertId()
		tk, err := s.tokens.CreateRuleTokenTx(ctx, tx, id) // 创建时自动生成
		if err != nil {
			return err
		}
		r = &Rule{ID: id, Slug: slugVal, Name: name, ClientType: clientType, Schemes: schemes, Token: tk}
		return nil
	})
	if err != nil {
		return nil, err
	}
	v, err := s.versions.CreateVersion(ctx, version.OwnerRule, r.ID, src)
	if err != nil {
		s.rollbackRecord(ctx, r.ID) // 失败清理：删 rule_tokens + rules 行
		return nil, err
	}
	r.CurrentVersion = v.No
	return r, nil
}

// rollbackRecord 首版本创建失败时回滚规则与 Token
func (s *Service) rollbackRecord(ctx context.Context, id int64) {
	if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx, `DELETE FROM rule_tokens WHERE rule_id = ?`, id)
		_, err := tx.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id)
		return err
	}); err != nil {
		s.log.Error("回滚规则创建失败", "id", id, "err", err)
	}
}

// Rename 创建后仅可改名（客户端类型与 scheme 不可修改——接入层不接收该字段）
func (s *Service) Rename(ctx context.Context, id int64, name string) error {
	if name == "" {
		return fmt.Errorf("%w: 名称必填", ErrBadRequest)
	}
	res, err := s.store.DB().ExecContext(ctx,
		`UPDATE rules SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, id)
	if err != nil {
		return fmt.Errorf("改名失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// RefreshToken 物理轮替（规则 Token 全局共享，不随用户禁用/删除失效）
func (s *Service) RefreshToken(ctx context.Context, id int64) (string, error) {
	return s.tokens.RotateRuleToken(ctx, id)
}

// Delete 级联删版本文件 + Token
func (s *Service) Delete(ctx context.Context, id int64) error {
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM rules WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM rule_tokens WHERE rule_id = ?`, id); err != nil {
			return err
		}
		if err := s.versions.DeleteVersionsTx(ctx, tx, version.OwnerRule, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM rules WHERE id = ?`, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.versions.RemoveOwnerDir(version.OwnerRule, id); err != nil {
		s.log.Warn("删除规则版本目录失败", "id", id, "err", err)
	}
	return nil
}

// List 规则列表（含全局 Token 与刷新时间）
func (s *Service) List(ctx context.Context) ([]Rule, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT r.id, r.slug, r.name, r.client_type, r.schemes, COALESCE(r.current_version,0), r.created_at,
		        COALESCE((SELECT rt.token FROM rule_tokens rt WHERE rt.rule_id = r.id LIMIT 1), ''),
		        COALESCE((SELECT rt.refreshed_at FROM rule_tokens rt WHERE rt.rule_id = r.id LIMIT 1), '')
		 FROM rules r ORDER BY r.id`)
	if err != nil {
		return nil, fmt.Errorf("读取规则列表失败: %w", err)
	}
	defer rows.Close()
	var out []Rule
	for rows.Next() {
		var r Rule
		var schemesRaw string
		if err := rows.Scan(&r.ID, &r.Slug, &r.Name, &r.ClientType, &schemesRaw, &r.CurrentVersion, &r.CreatedAt, &r.Token, &r.RefreshedAt); err != nil {
			return nil, err
		}
		r.Schemes = parseSchemes(schemesRaw)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Get 单个规则
func (s *Service) Get(ctx context.Context, id int64) (*Rule, error) {
	var r Rule
	var schemesRaw string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, slug, name, client_type, schemes, COALESCE(current_version,0) FROM rules WHERE id = ?`, id).
		Scan(&r.ID, &r.Slug, &r.Name, &r.ClientType, &schemesRaw, &r.CurrentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	r.Schemes = parseSchemes(schemesRaw)
	return &r, nil
}

// Name 取规则名称（下载 Content-Disposition 用）
func (s *Service) Name(ctx context.Context, id int64) (string, error) {
	var name string
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT name FROM rules WHERE id = ?`, id).Scan(&name); err != nil {
		return "", err
	}
	return name, nil
}

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func parseSchemes(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}
