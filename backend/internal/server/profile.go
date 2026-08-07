// Package server 个人中心端点（接入层）：会话。
package server

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

// ProfileHandler 个人中心处理器（结构体 Handler + 依赖注入）
type ProfileHandler struct {
	store *store.Store
	users *user.Service
}

// RegisterProfileRoutes 注册个人中心端点；全部需会话
func RegisterProfileRoutes(engine *gin.Engine, h *ProfileHandler, sessionMW gin.HandlerFunc) {
	g := engine.Group("/api/profile", sessionMW)
	g.PUT("/username", h.updateUsername)
	g.PUT("/email", h.updateEmail)
	g.PUT("/password", h.updatePassword)
}

// updateUsername 改用户名：即时生效（OIDC 用户下次 OIDC 登录会被提供商最新值覆盖，Design1 §4.6）
func (h *ProfileHandler) updateUsername(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=1,max=64"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	userID := c.GetInt64(auth.CtxUserID)
	if _, err := h.store.DB().ExecContext(c.Request.Context(),
		`UPDATE users SET username = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, req.Username, userID); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"message": "用户名已更新"})
}

// updateEmail 改邮箱：新邮箱被占用拒绝 409；成功递增 credential_version（所有设备会话立即失效）
func (h *ProfileHandler) updateEmail(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,max=254"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	email, err := auth.NormalizeEmail(req.Email)
	if err != nil {
		Fail(c, http.StatusBadRequest, "邮箱格式无效")
		return
	}
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	if err := h.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ? AND id != ?`, email, userID).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return user.ErrEmailConflict
		}
		// 同事务：改邮箱 + 递增凭据版本号（旧会话全部失效）
		_, err := tx.ExecContext(ctx,
			`UPDATE users SET email = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			email, userID)
		return err
	}); err != nil {
		if errors.Is(err, user.ErrEmailConflict) {
			Fail(c, http.StatusConflict, "该邮箱已被使用")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"message": "邮箱已修改，请重新登录"}) // 前端清凭据跳登录
}

// updatePassword 设置/修改密码：已设密码需验证当前密码；OIDC 用户首次设置免旧密码但须已登录；成功递增 credential_version
func (h *ProfileHandler) updatePassword(c *gin.Context) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password" binding:"required,max=128"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if err := auth.ValidatePassword(req.NewPassword); err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	err := h.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 查当前 password_hash：非空 → 必须验证 CurrentPassword；空（OIDC 首设）→ 免旧密码
		var hash string
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(password_hash,'') FROM users WHERE id = ?`, userID).Scan(&hash); err != nil {
			return err
		}
		if hash != "" && !auth.CheckPassword(hash, req.CurrentPassword) {
			return user.ErrAuthFailed // 「当前密码不正确」
		}
		newHash, err := auth.HashPassword(req.NewPassword)
		if err != nil {
			return err
		}
		// 同事务：写新哈希 + credential_version + 1
		_, err = tx.ExecContext(ctx,
			`UPDATE users SET password_hash = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			newHash, userID)
		return err
	})
	if errors.Is(err, user.ErrAuthFailed) {
		Fail(c, http.StatusBadRequest, "当前密码不正确")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"message": "密码已更新，请重新登录"})
}
