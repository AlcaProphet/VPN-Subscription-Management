// Package server 提供 HTTP 服务装配（接入层）。
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/captcha"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/ratelimit"
	"vpn-sub/internal/response"
	"vpn-sub/internal/setup"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
)

// Response 统一响应结构（类型别名，定义见 internal/response）
type Response = response.Response

// ListData 列表包裹结构（类型别名）
type ListData = response.ListData

// OK 成功响应（便捷包装）
func OK(c *gin.Context, data any) { response.OK(c, data) }

// Fail 错误响应（便捷包装）；httpStatus 与业务码同步取值（400/401/403/409/429/500）
func Fail(c *gin.Context, httpStatus int, msg string) { response.Fail(c, httpStatus, msg) }

type Server struct {
	engine  *gin.Engine
	httpSrv *http.Server
	cfg     *config.Service
	mode    string
	log     *slog.Logger
	// 后续 Step 的 Handler 经构造函数追加注入（setup/oidc...）
}

// New 构造注入装配：全部依赖经参数传入，禁止包级全局变量持有服务实例
func New(st *store.Store, cfg *config.Service, users *user.Service, lg *slog.Logger, mode, trustProxy, port, dataDir string) (*Server, error) {
	engine := gin.New() // 不用 gin.Default，避免默认 logger/recovery 绕过脱敏与统一响应
	if err := applyTrustProxy(engine, trustProxy); err != nil {
		return nil, err
	}
	engine.Use(requestLogger(), panicRecovery())
	s := &Server{engine: engine, cfg: cfg, mode: mode, log: lg,
		httpSrv: &http.Server{Addr: ":" + port, Handler: engine}}
	registerHealth(engine)
	// 依赖装配：auth/setup/oidc/captcha/ratelimit 服务（构造注入）
	authSvc := auth.NewService(cfg, users, lg)
	setupSvc := setup.NewService(st, cfg, lg, trustProxy)
	oidcSvc := oidc.NewService(st, cfg, authSvc, users, mode, lg)
	captchaSvc := captcha.NewService(cfg, lg)
	limiter := ratelimit.New(cfg, lg)
	resetSvc := auth.NewResetService(st, users, lg)
	registerStatus(engine, cfg, users, oidcSvc, captchaSvc, mode)
	// 认证路由（本 Build Step 4/7）：后续业务域路由同样在此按序注册
	RegisterAuthRoutes(engine, &AuthHandler{authSvc: authSvc, userSvc: users, cfg: cfg, resetSvc: resetSvc}, limiter, captchaSvc)
	// Setup 路由（本 Build Step 5/6）
	RegisterSetupRoutes(engine, &SetupHandler{setupSvc: setupSvc, oidcSvc: oidcSvc})
	// OIDC 路由（本 Build Step 6）
	RegisterOidcRoutes(engine, &OidcHandler{oidcSvc: oidcSvc, authSvc: authSvc}, authSvc.SessionMiddleware())
	if err := registerStatic(engine, dataDir); err != nil {
		return nil, err
	}
	return s, nil
}

// Engine 暴露 gin 引擎（供各业务域注册路由）
func (s *Server) Engine() *gin.Engine { return s.engine }

// applyTrustProxy auto=仅信任回环+私有网段转发头；on=全信任；off=不信任
func applyTrustProxy(engine *gin.Engine, mode string) error {
	switch mode {
	case "on":
		// 信任所有代理（gin v1.12 中 SetTrustedProxies(nil) 表示不信任任何代理，
		// 全信任须显式 0.0.0.0/0 + ::/0；gin 会输出不安全 WARNING，符合 on 档设计语义）
		return engine.SetTrustedProxies([]string{"0.0.0.0/0", "::/0"})
	case "off":
		return engine.SetTrustedProxies([]string{})
	default: // "auto"
		return engine.SetTrustedProxies([]string{"127.0.0.1/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
	}
}

// requestLogger 方法/路径/状态/耗时；路径中 ?token= 值由 slog 脱敏 Handler 统一处理
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Info("http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.RequestURI(),
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
		)
	}
}

// panicRecovery panic 统一转 500 通用信息（详情仅入日志）
func panicRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic 恢复", "err", fmt.Sprint(r), "path", c.Request.URL.Path)
				Fail(c, http.StatusInternalServerError, "服务器内部错误")
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Run 以非阻塞优雅退出方式启动 HTTP 服务
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case err := <-errCh:
		return fmt.Errorf("HTTP 服务异常退出: %w", err)
	case <-ctx.Done(): // 非阻塞优雅退出：等待在途请求收尾
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("优雅退出失败: %w", err)
		}
		return nil
	}
}
