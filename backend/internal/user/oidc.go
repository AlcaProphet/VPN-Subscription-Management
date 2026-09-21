// user/oidc.go：OIDC 用户查建/合并/绑定支持（Build1 Step 6）。
package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"vpn-sub/internal/auth"
)

// TxGuard 由 OIDC 服务提供，在 user 包自身事务内执行启用/流程指纹校验；
// guard 与最终用户写入处于同一写事务，避免“校验通过后停用已提交”的窗口。
type TxGuard func(ctx context.Context, tx *sql.Tx) error

// GetBySubject 按 OIDC subject 查询用户（未命中返回 nil）
func (s *Service) GetBySubject(ctx context.Context, subject string) (*User, error) {
	return s.scanUser(ctx, `SELECT id, COALESCE(oidc_subject,''), username, COALESCE(email,''), role,
		COALESCE(group_id,0), COALESCE(password_hash,''), user_source, status, credential_version
		FROM users WHERE oidc_subject = ?`, subject)
}

// GetBySubjectTx 在调用方写事务内按 OIDC subject 查询用户，供绑定流程做同事务重复校验。
func (s *Service) GetBySubjectTx(ctx context.Context, tx *sql.Tx, subject string) (*User, error) {
	var u User
	err := tx.QueryRowContext(ctx, `SELECT id, COALESCE(oidc_subject,''), username, COALESCE(email,''), role,
		COALESCE(group_id,0), COALESCE(password_hash,''), user_source, status, credential_version
		FROM users WHERE oidc_subject = ?`, subject).
		Scan(&u.ID, &u.OidcSubject, &u.Username, &u.Email, &u.Role,
			&u.GroupID, &u.PasswordHash, &u.Source, &u.Status, &u.CredentialVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	u.HasPassword = u.PasswordHash != ""
	return &u, nil
}

// GetByEmail 按邮箱查询用户（公开方法，供 OIDC 合并判定；未命中返回 nil）
func (s *Service) GetByEmail(ctx context.Context, email string) (*User, error) {
	normalized, err := auth.NormalizeEmail(email)
	if err != nil {
		return nil, nil // 格式非法按未命中处理（不泄露差异）
	}
	return s.getByEmail(ctx, normalized)
}

// RefreshUsername 每次 OIDC 登录刷新 username 为提供商最新值
func (s *Service) RefreshUsername(ctx context.Context, id int64, username string) error {
	return s.RefreshUsernameGuarded(ctx, id, username, nil)
}

// RefreshUsernameGuarded 在自事务内先执行 guard 再刷新用户名。
func (s *Service) RefreshUsernameGuarded(ctx context.Context, id int64, username string, guard TxGuard) error {
	if username == "" {
		return nil
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if guard != nil {
			if err := guard(ctx, tx); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET username = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, username, id); err != nil {
			return fmt.Errorf("刷新用户名失败: %w", err)
		}
		return nil
	})
}

// BindSubject 无条件绑定 subject（待审批账号场景；仅内部受控调用）
func (s *Service) BindSubject(ctx context.Context, id int64, subject string) error {
	return s.BindSubjectGuarded(ctx, id, subject, nil)
}

// BindSubjectGuarded 在自事务内先执行 guard 再无条件绑定。
func (s *Service) BindSubjectGuarded(ctx context.Context, id int64, subject string, guard TxGuard) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if guard != nil {
			if err := guard(ctx, tx); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE users SET oidc_subject = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, subject, id); err != nil {
			return fmt.Errorf("绑定 OIDC 身份失败: %w", err)
		}
		return nil
	})
}

// BindSubjectIfNull 条件绑定：UPDATE ... WHERE id=? AND oidc_subject IS NULL（防并发覆盖，Design1 §4.6）。
// 返回受影响行数：1=绑定成功，0=已被并发绑定
func (s *Service) BindSubjectIfNull(ctx context.Context, id int64, subject string) (int64, error) {
	return s.BindSubjectIfNullGuarded(ctx, id, subject, nil)
}

// BindSubjectIfNullGuarded 在自事务内先执行 guard 再条件绑定；条件判断与最终写入不可分离。
func (s *Service) BindSubjectIfNullGuarded(ctx context.Context, id int64, subject string, guard TxGuard) (int64, error) {
	var n int64
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if guard != nil {
			if err := guard(ctx, tx); err != nil {
				return err
			}
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE users SET oidc_subject = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND oidc_subject IS NULL`,
			subject, id)
		if err != nil {
			return fmt.Errorf("绑定 OIDC 身份失败: %w", err)
		}
		n, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// CreateFromOidc 从 OIDC 身份创建新用户（复用首管理员 BEGIN IMMEDIATE 事务：空表判定 → 首管理员免审批）。
// 首管理员只以同事务内 users 表计数为唯一事实来源，不再写入 admin_initialized 元数据。
// pending=true 时存 oidc_claims 快照且不激活；否则直接激活
func (s *Service) CreateFromOidc(ctx context.Context, username, email, subject, rawClaims string, pending bool) (*User, error) {
	return s.CreateFromOidcGuarded(ctx, username, email, subject, rawClaims, pending, nil)
}

// CreateFromOidcGuarded 在首管理员/建号事务首步执行 OIDC 流程 guard；
// guard 失败时整笔回滚，不留下停用后的新用户与绑定。
func (s *Service) CreateFromOidcGuarded(ctx context.Context, username, email, subject, rawClaims string, pending bool, guard TxGuard) (*User, error) {
	var created *User
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if guard != nil {
			if err := guard(ctx, tx); err != nil {
				return err
			}
		}
		// 空表判定：首管理员机制对 OIDC 首个登录者同样生效（Design1 §2.5）
		var total int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
			return err
		}
		first := total == 0
		role, status, source := "user", "active", "oidc"
		if pending {
			status = "pending"
		}
		if first {
			role, status = "admin", "active" // 首管理员免审批，不受任何审批开关影响
		}
		// 新用户自动加入预置默认组（Design1 §2.2）
		gid, err := defaultGroupIDTx(ctx, tx)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO users (username, email, oidc_subject, role, user_source, status, oidc_claims, group_id) VALUES (?,?,?,?,?,?,?,?)`,
			username, emailOrNil(email), subject, role, source, status, claimsOrNil(rawClaims, pending), gid)
		if err != nil {
			return fmt.Errorf("创建 OIDC 用户失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &User{ID: id, Username: username, Email: email, Role: role, Status: status, Source: source, OidcSubject: subject}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 欢迎邮件：直接激活（含首管理员/白名单命中）时发送；待审批不发（审批通过时由审批中心发送）
	if created.Status == "active" {
		s.sendWelcomeIf(ctx, created.ID, created.Email, created.Source)
		if s.onUserActive != nil {
			s.onUserActive(ctx, created.ID)
		}
	}
	s.log.Info("OIDC 用户创建成功", "user_id", created.ID, "role", created.Role, "pending", created.Status == "pending")
	return created, nil
}

// emailOrNil 空邮箱转 NULL（SQLite 唯一约束对 NULL 不生效）
func emailOrNil(email string) any {
	if email == "" {
		return nil
	}
	return email
}

// claimsOrNil 仅待审批用户存 claims 快照
func claimsOrNil(rawClaims string, pending bool) any {
	if pending && rawClaims != "" {
		return rawClaims
	}
	return nil
}
