// server/log.go：日志端点（接入层，Build3 Step 5）——访问日志查询/清空 + 实时日志流 SSE；
// 全部日志端点（含 SSE）统一叠加会话 + 管理员双中间件；查询 Token 已删除。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/log"
)

// LogHandler 日志处理器（结构体 Handler + 依赖注入）
type LogHandler struct {
	accessSvc *log.AccessService
	streamSvc *log.StreamService
	users     auth.UserSource
	// permissionInterval 流内权限重查间隔；生产默认 15 秒，测试可注入更短间隔。
	permissionInterval time.Duration
}

// RegisterLogRoutes 注册日志端点；访问日志/清空/SSE 全部叠加会话 + 管理员双中间件。
func RegisterLogRoutes(engine *gin.Engine, h *LogHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/logs", sessionMW, adminMW)
	g.GET("/access", h.queryAccess) // ?from=&to=&page=&size=
	g.POST("/access/clear", h.clearAccess)
	g.GET("/stream", h.stream) // SSE：Bearer 会话凭据经 fetch/ReadableStream 连接
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

// stream SSE 端点——管理员路由中间件已鉴权；先推缓冲历史，再实时推增量；
// 流内每 15 秒通过 UserSource 实时查库，用户缺失/非 active/非 admin 时关闭连接并清理订阅。
func (h *LogHandler) stream(c *gin.Context) {
	clearWriteDeadline(c)
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
	flusher, _ := c.Writer.(http.Flusher)
	for _, e := range history {
		writeSSE(c, e)
	}
	// 历史推送后立即 flush 一次：连接建立即有响应头/历史下发（即使无新事件）
	if flusher != nil {
		flusher.Flush()
	}
	interval := h.permissionInterval
	if interval <= 0 {
		interval = 15 * time.Second
	}
	permissionTicker := time.NewTicker(interval)
	defer permissionTicker.Stop()
	userID := c.GetInt64(auth.CtxUserID)
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
		case <-permissionTicker.C:
			if !h.streamUserAllowed(c.Request.Context(), userID) {
				log.FromContext(c.Request.Context()).Warn("实时日志流权限变化，关闭连接", "user_id", userID)
				return
			}
		case <-c.Request.Context().Done(): // 客户端断开
			return
		}
	}
}

// streamUserAllowed 实时查库校验：用户存在、active 且 admin。
func (h *LogHandler) streamUserAllowed(ctx context.Context, userID int64) bool {
	if h.users == nil {
		return false
	}
	snap, err := h.users.SnapshotByID(ctx, userID)
	if err != nil || snap == nil {
		return false
	}
	return snap.Status == "active" && snap.Role == "admin"
}

// writeSSE 按 SSE 协议输出：data: <json>\n\n
func writeSSE(c *gin.Context, e log.Entry) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(c.Writer, "data: %s\n\n", data)
}
