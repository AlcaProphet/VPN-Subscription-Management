package oidc

import (
	"context"
	"database/sql"
	"errors"

	"vpn-sub/internal/user"
)

// ResolveResult 用户查建结果
type ResolveResult struct {
	User     *user.User // 登录成功时非空
	Pending  bool       // 进入待审批（不签发会话，302 /pending）
	Message  string     // 冲突/待审批提示文案
	FlowHash string     `json:"-"` // R31-07：本次登录固定的流程指纹（仅供同进程接入层签发直连会话）
}

// ResolveLogin 兼容入口：以调用时刻的当前流程为边界执行查建。
// 生产 OIDC 回调必须使用 ResolveLoginForFlow，把发起授权时固定的流程指纹传进来。
func (s *Service) ResolveLogin(ctx context.Context, id *Identity) (*ResolveResult, error) {
	flowHash, err := s.currentFlowHash(ctx)
	if err != nil {
		return nil, err
	}
	res, err := s.resolveLoginWithGuard(ctx, id, s.flowGuard(flowHash))
	if err != nil {
		return nil, err
	}
	res.FlowHash = flowHash
	return res, nil
}

// ResolveLoginForFlow 使用 state 固定的流程指纹执行查建/合并/建号；
// 停用后即使快速重新启用，旧 rec 的流程指纹也不再匹配，最终用户写入与会话签发都会失败。
func (s *Service) ResolveLoginForFlow(ctx context.Context, rec *StateRecord, id *Identity) (*ResolveResult, error) {
	if rec == nil || rec.ConfigHash == "" {
		return nil, ErrOidcFlowInvalid
	}
	res, err := s.resolveLoginWithGuard(ctx, id, s.flowGuard(rec.ConfigHash))
	if err != nil {
		return nil, err
	}
	res.FlowHash = rec.ConfigHash
	return res, nil
}

// resolveLoginWithGuard 用户查建逻辑（Design1 §4.6，关键约束）；最终用户写入统一在 guard 事务内执行。
func (s *Service) resolveLoginWithGuard(ctx context.Context, id *Identity, guard func(context.Context, *sql.Tx) error) (*ResolveResult, error) {
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
		if err := s.users.RefreshUsernameGuarded(ctx, u.ID, id.Username, guard); err != nil {
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
				if err := s.users.BindSubjectGuarded(ctx, eu.ID, id.Subject, guard); err != nil {
					return nil, err
				}
				return &ResolveResult{Pending: true, Message: "已提交，等待审批"}, nil
			}
			// 自动合并：条件更新防并发覆盖；合并即激活（OIDC 视同可信，可绕过审批）
			n, err := s.users.BindSubjectIfNullGuarded(ctx, eu.ID, id.Subject, guard)
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
	// R14-25：OIDC 新用户创建是鉴权入口，审批开关读取失败必须显式返回，不能静默按“不审批”放行。
	approvalOn, err := s.cfg.GetBoolStrict(ctx, KeyOidcApproval, false)
	if err != nil {
		return nil, err
	}
	hitWhitelist := s.matchWhitelist(ctx, id) // 白名单为空时跳过校验直接激活
	pending := approvalOn && !hitWhitelist
	u, err = s.users.CreateFromOidcGuarded(ctx, id.Username, id.Email, id.Subject, id.RawClaims, pending, guard)
	if err != nil {
		return nil, err
	}
	if pending {
		return &ResolveResult{Pending: true, Message: "账号已创建，等待审批"}, nil
	}
	return &ResolveResult{User: u}, nil
}

// ResolveBind 手动绑定（intent=bind）：在单个写事务内完成流程守卫、subject 重复检查与条件绑定；不签发会话。
func (s *Service) ResolveBind(ctx context.Context, rec *StateRecord, id *Identity) error {
	if rec == nil || rec.ConfigHash == "" {
		return ErrOidcFlowInvalid
	}
	guard := func(ctx context.Context, tx *sql.Tx) error {
		if err := s.assertFlowCurrentTx(ctx, tx, rec.ConfigHash); err != nil {
			return err
		}
		other, err := s.users.GetBySubjectTx(ctx, tx, id.Subject)
		if err != nil {
			return err
		}
		if other != nil && other.ID != rec.BindUserID {
			return errors.New("该 OIDC 身份已绑定其他账号")
		}
		return nil
	}
	n, err := s.users.BindSubjectIfNullGuarded(ctx, rec.BindUserID, id.Subject, guard)
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("目标账号已绑定其他 OIDC 身份")
	}
	return nil
}
