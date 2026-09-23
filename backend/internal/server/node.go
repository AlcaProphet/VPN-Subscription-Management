package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
)

// NodeHandler 节点处理器。
type NodeHandler struct {
	nodeSvc *node.Service
}

// maxOpenVPNParseBodyBytes 是 `.ovpn` 解析端点完整请求体的上限：
// 正文上限 256 KiB 加 JSON 包装余量；正文自身超限仍由解析器给出 413。
const maxOpenVPNParseBodyBytes = node.MaxOpenVPNParseBytes + (16 << 10)

// parseOpenVPN 把粘贴的 `.ovpn` 文本解析为结构化草稿。
// 只读、无外部文件读取、无脚本执行、不落库；日志只记录长度、映射字段数与错误 code。
func (h *NodeHandler) parseOpenVPN(c *gin.Context) {
	if contentType := c.GetHeader("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		Fail(c, http.StatusBadRequest, "只接受 application/json 请求")
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOpenVPNParseBodyBytes)
	var input struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		if isRequestBodyTooLarge(err) {
			Fail(c, http.StatusRequestEntityTooLarge, "`.ovpn` 文本超过 256 KiB 上限")
			return
		}
		Fail(c, http.StatusBadRequest, "请求体必须是 {\"text\":\"...\"} 形式的 JSON")
		return
	}

	result, parseErr := node.ParseOpenVPN(input.Text)
	logger := log.FromContext(c.Request.Context())
	if parseErr != nil {
		var typed *node.OpenVPNParseError
		code := "ovpn_parse_failed"
		if errors.As(parseErr, &typed) {
			code = typed.Code
		}
		// 只记录长度、结果数与错误 code，绝不记录原文、内嵌块或凭据。
		logger.Warn("解析 .ovpn 被阻断", "bytes", len(input.Text), "mapped_fields", len(result.ProtocolJSON), "error_code", code)
		if code == node.OpenVPNDiagSizeExceeded {
			Fail(c, http.StatusRequestEntityTooLarge, "`.ovpn` 文本超过 256 KiB 上限")
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"code":        http.StatusBadRequest,
			"message":     "`.ovpn` 解析被阻断",
			"error_code":  code,
			"diagnostics": result.Diagnostics,
		})
		return
	}
	logger.Info("解析 .ovpn 完成", "bytes", len(input.Text), "mapped_fields", len(result.ProtocolJSON), "diagnostics", len(result.Diagnostics))
	OK(c, result)
}

// RegisterNodeRoutes 注册节点管理路由（会话 + 管理员双中间件）。
func RegisterNodeRoutes(engine *gin.Engine, h *NodeHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/nodes", sessionMW, adminMW)
	admin.GET("", h.list)
	admin.POST("", h.create)
	admin.POST("/import", h.importNodes)
	admin.POST("/check", h.check)
	admin.PUT("/:id", h.update)
	admin.DELETE("/:id", h.delete)
	admin.PUT("/:id/toggle", h.toggle)
	admin.PUT("/:id/display-name", h.setDisplayName)
	admin.GET("/protocols", h.protocols)
	admin.GET("/:id", h.get)
	// `.ovpn` 只读解析：no-store 必须先于 session/admin 执行，保证匿名、非管理员、超限、
	// 解析失败与成功响应都带禁止缓存头（沿用邮件模板与发送日志的既有分组方式）。
	openvpnParse := engine.Group("/api/admin/nodes", noStoreMiddleware(), sessionMW, adminMW)
	openvpnParse.POST("/openvpn/parse", h.parseOpenVPN)
}

func (h *NodeHandler) list(c *gin.Context) {
	source := c.Query("source")
	list, err := h.nodeSvc.List(c.Request.Context(), source)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *NodeHandler) protocols(c *gin.Context) {
	OK(c, gin.H{"list": h.nodeSvc.GetProtocols()})
}

func (h *NodeHandler) get(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	n, err := h.nodeSvc.Get(c.Request.Context(), id)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) importNodes(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	list, err := h.nodeSvc.ImportURIs(c.Request.Context(), req.Text)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: int64(len(list))})
}

func (h *NodeHandler) create(c *gin.Context) {
	var req node.CreateManualInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	n, err := h.nodeSvc.CreateManual(c.Request.Context(), req)
	if errors.Is(err, node.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, node.ErrConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) check(c *gin.Context) {
	var req node.CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	resp, err := h.nodeSvc.Check(c.Request.Context(), req)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrRevisionConflict) {
		current, _ := node.CurrentRevisionFromError(err)
		c.JSON(http.StatusConflict, gin.H{
			"error":            node.ErrRevisionConflict.Error(),
			"code":             "revision_conflict",
			"current_revision": current,
		})
		return
	}
	if errors.Is(err, node.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, resp)
}

func (h *NodeHandler) update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req node.UpdateManualInput
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	n, err := h.nodeSvc.UpdateManual(c.Request.Context(), id, req)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrRevisionConflict) {
		current, _ := node.CurrentRevisionFromError(err)
		c.JSON(http.StatusConflict, gin.H{
			"error":            node.ErrRevisionConflict.Error(),
			"code":             "revision_conflict",
			"current_revision": current,
		})
		return
	}
	if errors.Is(err, node.ErrBadRequest) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusForbidden, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	err := h.nodeSvc.Delete(c.Request.Context(), id)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

func (h *NodeHandler) toggle(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		Enabled  *bool `json:"enabled"`
		IsPublic *bool `json:"is_public"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	if req.Enabled == nil && req.IsPublic == nil {
		Fail(c, http.StatusBadRequest, "缺少切换字段")
		return
	}
	var n *node.Node
	var err error
	if req.Enabled != nil {
		n, err = h.nodeSvc.SetEnabled(c.Request.Context(), id, *req.Enabled)
	} else {
		n, err = h.nodeSvc.SetPublic(c.Request.Context(), id, *req.IsPublic)
	}
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrBadRequest) || errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}

func (h *NodeHandler) setDisplayName(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	n, err := h.nodeSvc.SetDisplayName(c.Request.Context(), id, req.DisplayName)
	if errors.Is(err, node.ErrNotFound) {
		Fail(c, http.StatusNotFound, "节点不存在")
		return
	}
	if errors.Is(err, node.ErrConflict) {
		Fail(c, http.StatusConflict, err.Error())
		return
	}
	if errors.Is(err, node.ErrBadRequest) || errors.Is(err, node.ErrForbidden) {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, n)
}
