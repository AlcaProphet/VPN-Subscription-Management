// server/log.go：日志端点（接入层，Build3 Step 5）——访问日志查询/清空 + 实时日志流 SSE；
// 全部日志端点（含 SSE）统一叠加会话 + 管理员双中间件；查询 Token 已删除。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/log"
	"vpn-sub/internal/mail"
)

// LogHandler 日志处理器（结构体 Handler + 依赖注入）
type LogHandler struct {
	accessSvc *log.AccessService
	streamSvc *log.StreamService
	users     auth.UserSource
	mailLog   *mail.ActivityLog
	// permissionInterval 流内权限重查间隔；生产默认 15 秒，测试可注入更短间隔。
	permissionInterval time.Duration
}

// RegisterLogRoutes 注册日志端点；访问日志/清空/SSE 全部叠加会话 + 管理员双中间件；
// 邮件发送日志列表/清空按 R32-02 先经过 no-store 再鉴权，确保 401/403 也带 no-store。
func RegisterLogRoutes(engine *gin.Engine, h *LogHandler, sessionMW, adminMW gin.HandlerFunc) {
	g := engine.Group("/api/admin/logs", sessionMW, adminMW)
	g.GET("/access", h.queryAccess) // ?from=&to=&page=&size=
	g.POST("/access/clear", h.clearAccess)
	g.GET("/stream", h.stream) // SSE：Bearer 会话凭据经 fetch/ReadableStream 连接

	mailGroup := engine.Group("/api/admin/logs/mail", noStoreMiddleware(), sessionMW, adminMW)
	mailGroup.GET("", h.queryMail)
	mailGroup.POST("/clear", h.clearMail)
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

// queryMail 邮件发送日志查询：严格分页 + kind/status 过滤；只返回内存快照中的安全字段。
func (h *LogHandler) queryMail(c *gin.Context) {
	if h.mailLog == nil {
		FailSanitized(c, http.StatusInternalServerError, "邮件发送日志服务不可用", errors.New("mailLog 未注入"))
		return
	}
	page, err := mailPositiveQuery(c, "page", 1)
	if err != nil {
		Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	size, err := mailPositiveQuery(c, "size", 20)
	if err != nil || size > 100 {
		Fail(c, http.StatusBadRequest, "size 须为 1–100 的整数")
		return
	}
	// 阻止 (page-1)*size 溢出为负数后触发 slice panic；超出可表示范围的页码按非法分页处理。
	if page > math.MaxInt/size {
		Fail(c, http.StatusBadRequest, "page 超出可处理范围")
		return
	}
	kind := c.Query("kind")
	if kind != "" && !mailKindAllowed(kind) {
		Fail(c, http.StatusBadRequest, "未知邮件类型")
		return
	}
	status := c.Query("status")
	if status != "" && !mailStatusAllowed(status) {
		Fail(c, http.StatusBadRequest, "未知邮件状态")
		return
	}
	all := h.mailLog.Snapshot()
	filtered := make([]mail.ActivityRecord, 0, len(all))
	for _, rec := range all {
		if kind != "" && string(rec.Kind) != kind {
			continue
		}
		if status != "" && string(rec.Status) != status {
			continue
		}
		filtered = append(filtered, rec)
	}
	total := int64(len(filtered))
	start := (page - 1) * size
	if start >= len(filtered) {
		OK(c, ListData{List: make([]mail.ActivityRecord, 0), Total: total})
		return
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	list := make([]mail.ActivityRecord, 0, end-start)
	list = append(list, filtered[start:end]...)
	OK(c, ListData{List: list, Total: total})
}

// clearMail 清空当前可见邮件发送日志；不取消发送、不丢队列、不重置 ID。
func (h *LogHandler) clearMail(c *gin.Context) {
	if h.mailLog == nil {
		FailSanitized(c, http.StatusInternalServerError, "邮件发送日志服务不可用", errors.New("mailLog 未注入"))
		return
	}
	h.mailLog.Clear()
	OK(c, nil)
}

func mailPositiveQuery(c *gin.Context, key string, def int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s 须为正整数", key)
	}
	return n, nil
}

func mailKindAllowed(kind string) bool {
	switch kind {
	case string(mail.JobWelcomeLocal), string(mail.JobWelcomeOIDC),
		string(mail.JobApprovalApproved), string(mail.JobApprovalRejected),
		string(mail.JobPasswordReset), string(mail.ActivityKindSMTPTest):
		return true
	default:
		return false
	}
}

func mailStatusAllowed(status string) bool {
	switch mail.ActivityStatus(status) {
	case mail.ActivityQueued, mail.ActivitySending, mail.ActivityAccepted, mail.ActivityFailed:
		return true
	default:
		return false
	}
}
