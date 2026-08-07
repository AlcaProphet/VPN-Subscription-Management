package oidc

import (
	"context"
	"errors"

	"vpn-sub/internal/user"
)

// ResolveResult 用户查建结果
type ResolveResult struct {
	User    *user.User // 登录成功时非空
	Pending bool       // 进入待审批（不签发会话，302 /pending）
	Message string     // 冲突/待审批提示文案
}

// ResolveLogin 用户查建逻辑（Design1 §4.6，关键约束）
func (s *Service) ResolveLogin(ctx context.Context, id *Identity) (*ResolveResult, error) {
	// 1) subject 命中 → 直接登录（username 每次刷新为提供商最新值；email 首次写入后不自动覆盖）
	u, err := s.users.GetBySubject(ctx, id.Subject)
	if err != nil {
		return nil, err
	}
	if u != nil {
		if u.Status == "pending" {
			return &ResolveResult{Pending: true, Message: "已提交，等待审批"}, nil // 待审批重复登录
		}
		if u.Status == "disabled" {
			return &ResolveResult{Message: "账号未激活或已被禁用"}, nil
		}
		if err := s.users.RefreshUsername(ctx, u.ID, id.Username); err != nil {
			return nil, err
		}
		return &ResolveResult{User: u}, nil
	}
	// 2) subject 未命中但邮箱命中
	if id.Email != "" {
		eu, err := s.users.GetByEmail(ctx, id.Email)
		if err != nil {
			return nil, err
		}
		if eu != nil {
			if eu.Status == "disabled" {
				return &ResolveResult{Message: "目标账号已禁用，无法合并"}, nil
			}
			if eu.OidcSubject != "" {
				return &ResolveResult{Message: "目标账号已绑定其他 OIDC 身份"}, nil
			}
			if !id.EmailVerified {
				return &ResolveResult{Message: "邮箱未验证，无法自动合并"}, nil
			}
			if eu.Status == "pending" {
				// 待审批命中：不创建新记录，将新 subject 绑定到该待审批账号
				if err := s.users.BindSubject(ctx, eu.ID, id.Subject); err != nil {
					return nil, err
				}
				return &ResolveResult{Pending: true, Message: "已提交，等待审批"}, nil
			}
			// 自动合并：条件更新防并发覆盖；合并即激活（OIDC 视同可信，可绕过审批）
			n, err := s.users.BindSubjectIfNull(ctx, eu.ID, id.Subject) // WHERE oidc_subject IS NULL
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return &ResolveResult{Message: "目标账号已被并发绑定其他 OIDC 身份"}, nil
			}
			return &ResolveResult{User: eu}, nil
		}
	}
	// 3) 均不存在 → 创建新用户（首管理员机制同样生效，复用 user 包原子事务）
	//    OIDC 审批开关默认关闭 → 直接激活；开启且未命中白名单 → pending + 存 claims + 不签发会话
	approvalOn := s.cfg.GetBool(ctx, KeyOidcApproval, false) // 读取路径预留（Build3 接通）
	hitWhitelist := s.matchWhitelist(ctx, id)                       // 白名单为空时跳过校验直接激活
	pending := approvalOn && !hitWhitelist
	u, err = s.users.CreateFromOidc(ctx, id.Username, id.Email, id.Subject, id.RawClaims, pending)
	if err != nil {
		return nil, err
	}
	if pending {
		return &ResolveResult{Pending: true, Message: "账号已创建，等待审批"}, nil
	}
	return &ResolveResult{User: u}, nil
}

// ResolveBind 手动绑定（intent=bind）：校验 subject 未绑定其他账号 → 写入目标账号；不签发会话
func (s *Service) ResolveBind(ctx context.Context, rec *StateRecord, id *Identity) error {
	other, err := s.users.GetBySubject(ctx, id.Subject)
	if err != nil {
		return err
	}
	if other != nil && other.ID != rec.BindUserID {
		return errors.New("该 OIDC 身份已绑定其他账号")
	}
	n, err := s.users.BindSubjectIfNull(ctx, rec.BindUserID, id.Subject)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("目标账号已绑定其他 OIDC 身份")
	}
	return nil
}
