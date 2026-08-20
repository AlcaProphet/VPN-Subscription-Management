package server

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/node"
	"vpn-sub/internal/platform"
	"vpn-sub/internal/pool"
	"vpn-sub/internal/proxygroup"
	"vpn-sub/internal/rule"
	"vpn-sub/internal/version"
)

// AssemblyHandler 装配端点处理器。
type AssemblyHandler struct {
	assemblySvc   *assembly.Service
	nodeSvc       *node.Service
	proxyGroupSvc *proxygroup.Service
	poolSvc       *pool.Service
	platformSvc   *platform.Service
	ruleSvc       *rule.Service
	versionSvc    *version.Service
}

// RegisterAssemblyRoutes 注册装配相关路由（会话 + 管理员双中间件）。
func RegisterAssemblyRoutes(engine *gin.Engine, h *AssemblyHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/assembly", sessionMW, adminMW)
	admin.GET("/context", h.context)
	admin.POST("/preview", h.preview)
	admin.POST("/generate", h.generate)

	versions := engine.Group("/api/admin/versions", sessionMW, adminMW)
	versions.GET("/:id/blueprint", h.blueprint)
}

// context 一次性返回装配器候选数据。
func (h *AssemblyHandler) context(c *gin.Context) {
	ctx := c.Request.Context()
	nodes, err := h.nodeSvc.List(ctx, "")
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	groups, err := h.proxyGroupSvc.List(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	pools, err := h.poolSvc.List(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	platforms, err := h.platformSvc.List(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	rules, err := h.ruleSvc.List(ctx)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{
		"nodes":        nodes,
		"proxy_groups": groups,
		"pools":        pools,
		"platforms":    platforms,
		"rules":        rules,
	})
}

// preview 渲染预览，不落库。
func (h *AssemblyHandler) preview(c *gin.Context) {
	var in assembly.GenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	res, err := h.assemblySvc.Preview(c.Request.Context(), in)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, gin.H{
		"content":      string(res.Content),
		"skipped":      res.Skipped,
		"warnings":     res.Warnings,
		"name_changed": res.NameChanged,
	})
}

// generate 渲染并创建版本（入池不激活；首版自动激活）。
func (h *AssemblyHandler) generate(c *gin.Context) {
	ctx := c.Request.Context()
	var in assembly.GenerateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	res, err := h.assemblySvc.Render(ctx, in)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	ownerType, ownerID, fileName, err := h.resolveOwner(ctx, in)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	created, activated, err := h.versionSvc.CreateVersion(ctx, ownerType, ownerID,
		version.TextContent{Text: res.Content, Name: fileName},
		version.CreateOptions{
			Activate: false,
			AfterCreate: func(tx *sql.Tx, versionID int64, content []byte) error {
				return h.assemblySvc.SaveBlueprintTx(ctx, tx, versionID, in, res.RenderPlan)
			},
		})
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{
		"version_id":     created.ID,
		"version_no":     created.No,
		"auto_activated": activated,
		"skipped":        res.Skipped,
		"warnings":       h.assemblySvc.Warnings(in, res),
	})
}

// resolveOwner 按装配类型定位版本 owner（sr-conf→rule；其余→platform 唯一订阅）。
func (h *AssemblyHandler) resolveOwner(ctx context.Context, in assembly.GenerateInput) (version.OwnerType, int64, string, error) {
	if in.TargetSyntax == assembly.SrConf {
		if in.RuleID <= 0 {
			return "", 0, "", errors.New("请选择分流规则实体")
		}
		return version.OwnerRule, in.RuleID, "rule.conf", nil
	}
	if in.PlatformID <= 0 {
		return "", 0, "", errors.New("请选择目标平台")
	}
	subID, err := h.assemblySvc.FindSubscriptionByPlatform(ctx, in.PlatformID)
	if errors.Is(err, assembly.ErrSubscriptionNotFound) {
		return "", 0, "", errors.New("请先在订阅管理为该平台创建订阅条目")
	}
	if err != nil {
		return "", 0, "", err
	}
	name := "subscription.yaml"
	switch in.TargetSyntax {
	case assembly.SrSubs, assembly.GenericSubs:
		name = "subscription.txt"
	}
	return version.OwnerSubscription, subID, name, nil
}

// blueprint 读取装配快照并校验引用，返回失效引用与显示名变化。
func (h *AssemblyHandler) blueprint(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	data, err := h.assemblySvc.GetBlueprint(ctx, id)
	if err != nil {
		if err.Error() == "蓝图不存在" {
			Fail(c, http.StatusNotFound, "蓝图不存在")
			return
		}
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	blueprint := gin.H{
		"target_syntax": data.TargetSyntax,
		"version_no":    data.VersionNo,
		"fixed_params":  data.FixedParams,
		"selection":     data.Selection,
		"custom_rules":  data.CustomRules,
		"render_plan":   data.RenderPlan,
		"platform_id":   data.PlatformID,
		"rule_id":       data.RuleID,
	}
	OK(c, gin.H{"blueprint": blueprint, "invalid_refs": data.InvalidRefs, "name_changed": data.NameChanged})
}
