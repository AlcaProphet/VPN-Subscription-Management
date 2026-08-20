package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/tasks"
)

// TasksHandler 全局长任务查询接入层。
type TasksHandler struct {
	registry *tasks.Registry
}

// RegisterTasksRoutes 注册全局长任务查询端点。
func RegisterTasksRoutes(engine *gin.Engine, h *TasksHandler, sessionMW, adminMW gin.HandlerFunc) {
	admin := engine.Group("/api/admin/tasks", sessionMW, adminMW)
	admin.GET("/:id", h.get)
}

func (h *TasksHandler) get(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		Fail(c, http.StatusBadRequest, "参数错误")
		return
	}
	OK(c, h.registry.Get(id))
}
