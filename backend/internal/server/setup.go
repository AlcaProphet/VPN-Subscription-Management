package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/setup"
	"vpn-sub/internal/urlguard"
)

// SetupHandler Setup 端点处理器（接入层）
type SetupHandler struct {
	setupSvc *setup.Service
	oidcSvc  *oidc.Service
	mode     string // 启动时确定的 APP_MODE（dev/prod），Production 下拒绝 mock
}

// RegisterSetupRoutes 注册 Setup 路由
func RegisterSetupRoutes(engine *gin.Engine, h *SetupHandler) {
	// GET /api/setup/status 不单独实现：复用 /api/system/status 的 configured 字段，避免重复端点
	engine.POST("/api/setup/quickstart", h.quickstart) // 仅在未配置状态暴露（处理器内校验）
	engine.POST("/api/setup/oidc", h.oidcSetup)        // 高级配置分支（Step 6）；Build3 追加 POST /api/setup/import
}

func (h *SetupHandler) quickstart(c *gin.Context) {
	ctx := c.Request.Context()
	configured, err := h.setupSvc.IsConfigured(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if configured {
		Fail(c, http.StatusConflict, "系统已完成配置") // 已配置返回 409
		return
	}
	if err := h.setupSvc.CompleteQuickStart(ctx, c.Request); err != nil {
		if errors.Is(err, setup.ErrAlreadyConfigured) {
			Fail(c, http.StatusConflict, "系统已完成配置")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"configured": true, "message": "配置完成，请立即注册成为管理员"})
}

// oidcSetup 高级配置分支：未配置状态下保存 OIDC 参数（含 Secret 加密）→ 同事务完成预置数据 + configured 置位
func (h *SetupHandler) oidcSetup(c *gin.Context) {
	ctx := c.Request.Context()
	configured, err := h.setupSvc.IsConfigured(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if configured {
		Fail(c, http.StatusConflict, "系统已完成配置")
		return
	}
	var req struct {
		ProviderType string `json:"provider_type" binding:"required"`
		BaseURL      string `json:"base_url"`
		Realm        string `json:"realm"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	switch req.ProviderType {
	case "keycloak", "auth0", "generic", "mock":
	default:
		Fail(c, http.StatusBadRequest, "提供商类型无效")
		return
	}
	if req.ProviderType == "mock" && h.mode != "dev" {
		Fail(c, http.StatusBadRequest, "生产模式不支持模拟 OIDC 提供商")
		return
	}
	if req.ProviderType != "mock" && req.BaseURL != "" {
		if err := urlguard.ValidateHTTPS(req.BaseURL); err != nil {
			Fail(c, http.StatusBadRequest, "OIDC Base URL 必须是 HTTPS 地址: "+err.Error())
			return
		}
	}
	if req.ClientSecret == config.MaskedSecret {
		Fail(c, http.StatusBadRequest, "不能将脱敏占位符保存为 Client Secret，请重新输入")
		return
	}
	providerType := req.ProviderType
	err = h.setupSvc.CompleteOidcSetup(ctx, c.Request, providerType, func(tx *sql.Tx) error {
		// OIDC 参数写入（Secret 事务内加密；同一事务读签名密钥）
		secretCipher := ""
		if req.ClientSecret != "" {
			enc, err := h.oidcSvc.EncryptWithTx(ctx, tx, req.ClientSecret)
			if err != nil {
				return err
			}
			secretCipher = enc
		}
		params := oidc.Params{BaseURL: req.BaseURL, Realm: req.Realm, ClientID: req.ClientID, ClientSecret: secretCipher}
		raw, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("序列化 OIDC 参数失败: %w", err)
		}
		if err := h.oidcSvc.SaveRawParamsTx(ctx, tx, providerType, string(raw)); err != nil {
			return err
		}
		if err := h.oidcSvc.SetProviderTx(ctx, tx, providerType); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, setup.ErrAlreadyConfigured) {
			Fail(c, http.StatusConflict, "系统已完成配置")
			return
		}
		if errors.Is(err, config.ErrBadRequest) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, config.ErrMockModeRestricted) {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"configured": true, "message": "OIDC 配置完成，请立即注册成为管理员"})
}
