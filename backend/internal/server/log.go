// server/log.go：日志端点（接入层，Build3 Step 5）——访问日志查询/清空 + 实时日志流 SSE；
// 会话 + 管理员双中间件；SSE 认证用一次性短期 Token（EventSource 无法带 Header，Design1 §4.8）。
package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/log"
)

// LogHandler 日志处理器（结构体 Handler + 依赖注入）
type LogHandler struct {
	accessSvc *log.AccessService
	streamSvc *log.StreamService
}

// RegisterLogRoutes 注册日志端点；全部叠加会话 + 管理员双中间件
func RegisterLogRoutes(engine *gin.Engine, h *LogHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/logs", sessionMW, adminMW)
	g.GET("/access", h.queryAccess)             // ?from=&to=&page=&size=
	g.POST("/access/clear", h.clearAccess)
	g.POST("/stream/token", h.issueStreamToken) // 换一次性短期 Token（会话凭据鉴权）
	g.GET("/stream", h.stream)                  // SSE：?token= 短期 Token（EventSource 无法带 Header）
}

// queryAccess 访问日志查询（日期范围 + 后端分页）
func (h *LogHandler) queryAccess(c *gin.Context) {
	list, total, err := h.accessSvc.Query(c.Request.Context(),
		c.Query("from"), c.Query("to"),
		atoiDefault(c.Query("page"), 1), atoiDefault(c.Query("size"), 20))
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	OK(c, ListData{List: list, Total: total}) // 分页列表保留统一包裹结构（R02-01）
}

// clearAccess 清空访问日志（二次确认由前端 ConfirmModal 负责）
func (h *LogHandler) clearAccess(c *gin.Context) {
	if err := h.accessSvc.Clear(c.Request.Context()); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, nil)
}

// issueStreamToken 换取一次性短期 Token（仅管理员——双中间件已保证）
func (h *LogHandler) issueStreamToken(c *gin.Context) {
	token, err := h.streamSvc.IssueToken()
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"token": token})
}

// stream SSE 端点——先推缓冲历史，再实时推增量；Token 单次连接建立后即删；连接断开自动清理
func (h *LogHandler) stream(c *gin.Context) {
	if !h.streamSvc.ConsumeToken(c.Query("token")) { // 一次性校验（用后即删）
		Fail(c, http.StatusUnauthorized, "短期 Token 无效或已过期")
		return
	}
	ch, history, ok := h.streamSvc.Subscribe()
	if !ok {
		Fail(c, http.StatusTooManyRequests, "连接数已达上限，请关闭其他日志页后重试")
		return
	}
	defer h.streamSvc.Unsubscribe(ch)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// 先推历史
	for _, e := range history {
		writeSSE(c, e)
	}
	// 再推增量（断开检测：请求上下文取消）
	flusher, _ := c.Writer.(http.Flusher)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return // 缓冲复位（一键清空）时通道关闭
			}
			writeSSE(c, e)
			if flusher != nil {
				flusher.Flush()
			}
		case <-c.Request.Context().Done(): // 客户端断开
			return
		}
	}
}

// writeSSE 按 SSE 协议输出：data: <json>\n\n
func writeSSE(c *gin.Context, e log.Entry) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
}
