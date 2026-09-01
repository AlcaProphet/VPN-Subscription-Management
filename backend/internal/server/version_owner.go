// Package server 的版本归属端点：按通用版本记录定位真实资源名称。
package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/custom"
	"vpn-sub/internal/rule"
	"vpn-sub/internal/share"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/version"
)

// VersionOwnerHandler 仅持有版本归属解析所需的最小只读依赖。
type VersionOwnerHandler struct {
	versionSvc *version.Service
	subSvc     *subscription.Service
	ruleSvc    *rule.Service
	shareSvc   *share.Service
	customSvc  *custom.Service
}

type versionOwnerResponse struct {
	OwnerType string `json:"owner_type"`
	OwnerID   int64  `json:"owner_id"`
	Name      string `json:"name"`
	TypeLabel string `json:"type_label"`
	BackPath  string `json:"back_path"`
}

// RegisterVersionOwnerRoutes 独立注册版本归属端点，避免向装配 Handler 注入无关依赖。
func RegisterVersionOwnerRoutes(engine *gin.Engine, h *VersionOwnerHandler, sessionMW, adminMW gin.HandlerFunc) {
	engine.GET("/api/admin/versions/:id/owner", sessionMW, adminMW, h.get)
}

func (h *VersionOwnerHandler) get(c *gin.Context) {
	versionID, ok := parseID(c, "id")
	if !ok {
		return
	}
	ownerType, ownerID, err := h.versionSvc.OwnerByVersionID(c.Request.Context(), versionID)
	if errors.Is(err, version.ErrVersionNotFound) {
		Fail(c, http.StatusNotFound, "版本不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}

	response, err := h.resolve(c, ownerType, ownerID)
	if errors.Is(err, subscription.ErrNotFound) || errors.Is(err, rule.ErrNotFound) ||
		errors.Is(err, share.ErrNotFound) || errors.Is(err, custom.ErrNotFound) {
		Fail(c, http.StatusNotFound, "资源不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, response)
}

func (h *VersionOwnerHandler) resolve(c *gin.Context, ownerType version.OwnerType, ownerID int64) (versionOwnerResponse, error) {
	ctx := c.Request.Context()
	response := versionOwnerResponse{OwnerType: string(ownerType), OwnerID: ownerID}
	switch ownerType {
	case version.OwnerSubscription:
		sub, err := h.subSvc.Get(ctx, ownerID)
		if err != nil {
			return versionOwnerResponse{}, err
		}
		response.Name = sub.Name
		response.TypeLabel = "订阅"
		response.BackPath = "/admin/subscriptions"
	case version.OwnerRule:
		r, err := h.ruleSvc.Get(ctx, ownerID)
		if err != nil {
			return versionOwnerResponse{}, err
		}
		response.Name = r.Name
		response.TypeLabel = "规则"
		response.BackPath = "/admin/rules"
	case version.OwnerShare:
		sh, err := h.shareSvc.Get(ctx, ownerID)
		if err != nil {
			return versionOwnerResponse{}, err
		}
		response.Name = sh.Name
		response.TypeLabel = "分享"
		response.BackPath = "/admin/shares"
	case version.OwnerCustom:
		info, err := h.customSvc.Get(ctx, ownerID)
		if err != nil {
			return versionOwnerResponse{}, err
		}
		response.Name = fmt.Sprintf("%s / %s", info.Username, info.PlatformName)
		response.TypeLabel = "自定义订阅"
		response.BackPath = "/admin/users"
	default:
		return versionOwnerResponse{}, fmt.Errorf("未知版本归属类型: %s", ownerType)
	}
	return response, nil
}
