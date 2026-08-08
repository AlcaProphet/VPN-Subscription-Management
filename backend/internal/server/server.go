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
	"vpn-sub/internal/approval"
	"vpn-sub/internal/backup"
	"vpn-sub/internal/captcha"
	"vpn-sub/internal/config"
	"vpn-sub/internal/custom"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/download"
	"vpn-sub/internal/emergency"
	"vpn-sub/internal/group"
	"vpn-sub/internal/log"
	"vpn-sub/internal/mail"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/platform"
	"vpn-sub/internal/ratelimit"
	"vpn-sub/internal/response"
	"vpn-sub/internal/rule"
	"vpn-sub/internal/setup"
	"vpn-sub/internal/share"
	"vpn-sub/internal/store"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/token"
	"vpn-sub/internal/user"
	"vpn-sub/internal/version"
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
	store   *store.Store
	mode    string
	log     *slog.Logger
	// 后续 Step 的 Handler 经构造函数追加注入（setup/oidc...）
}

// New 构造注入装配：全部依赖经参数传入，禁止包级全局变量持有服务实例
func New(st *store.Store, cfg *config.Service, users *user.Service, lg *slog.Logger, mode, trustProxy, port, dataDir string, streamSvc *log.StreamService) (*Server, error) {
	engine := gin.New() // 不用 gin.Default，避免默认 logger/recovery 绕过脱敏与统一响应
	if err := applyTrustProxy(engine, trustProxy); err != nil {
		return nil, err
	}
	engine.Use(requestLogger(), panicRecovery())
	s := &Server{engine: engine, cfg: cfg, store: st, mode: mode, log: lg,
		httpSrv: &http.Server{Addr: ":" + port, Handler: engine}}
	registerHealth(engine)
	// 依赖装配：auth/setup/oidc/captcha/ratelimit 服务（构造注入）
	authSvc := auth.NewService(cfg, users, lg)
	setupSvc := setup.NewService(st, cfg, lg, trustProxy)
	oidcSvc := oidc.NewService(st, cfg, authSvc, users, mode, lg)
	captchaSvc := captcha.NewService(cfg, lg)
	limiter := ratelimit.New(cfg, lg)
	resetSvc := auth.NewResetService(st, users, lg)
	registerStatus(engine, cfg, users, oidcSvc, captchaSvc, mode, nil)
	// 认证路由（本 Build Step 4/7）：后续业务域路由同样在此按序注册
	RegisterAuthRoutes(engine, &AuthHandler{authSvc: authSvc, userSvc: users, cfg: cfg, resetSvc: resetSvc}, limiter, captchaSvc)
	// Setup 路由（本 Build Step 5/6）
	RegisterSetupRoutes(engine, &SetupHandler{setupSvc: setupSvc, oidcSvc: oidcSvc})
	// OIDC 路由（本 Build Step 6）
	RegisterOidcRoutes(engine, &OidcHandler{oidcSvc: oidcSvc, authSvc: authSvc}, authSvc.SessionMiddleware())
	// 版本组件 + 订阅池路由（Build2 Step 2；会话 + 管理员双中间件）
	versionSvc := version.NewService(st, dataDir, lg)
	// 平台路由（Build2 Step 1；会话 + 管理员双中间件；Step 5 起持有版本组件用于完整级联）
	platformSvc := platform.NewService(st, dataDir, versionSvc, lg)
	RegisterPlatformRoutes(engine, &PlatformHandler{platformSvc: platformSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	subSvc := subscription.NewService(st, versionSvc, lg)
	RegisterSubscriptionRoutes(engine, &SubscriptionHandler{subSvc: subSvc, verSvc: versionSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 用户组服务与路由（Build2 Step 3）：订阅删除级联回调注入（清选定 + needs_reselect）
	groupSvc := group.NewService(st, lg)
	subSvc.SetOnSubscriptionDeleted(groupSvc.OnSubscriptionDeleted)
	RegisterGroupRoutes(engine, &GroupHandler{groupSvc: groupSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// Token 服务 + 下载解析 + 用户端数据（Build2 Step 4）：订阅删除 Token 级联回调注入
	tokenSvc := token.NewService(st, lg)
	subSvc.SetOnTokenDeleted(tokenSvc.DeleteBySubscriptionTx)
	dlSvc := download.NewService(st, versionSvc, cfg, lg)
	homeHandler := &HomeHandler{store: st, tokenSvc: tokenSvc, dlSvc: dlSvc}
	RegisterDownloadRoutes(engine, &DownloadHandler{dlSvc: dlSvc, limiter: limiter, sessionMW: authSvc.SessionMiddleware()})
	RegisterHomeRoutes(engine, homeHandler, authSvc.SessionMiddleware())
	// 自定义订阅 + 分享订阅（Build2 Step 5；会话 + 管理员双中间件）
	customSvc := custom.NewService(st, versionSvc, tokenSvc, lg)
	shareSvc := share.NewService(st, versionSvc, tokenSvc, lg)
	RegisterCustomRoutes(engine, &CustomHandler{customSvc: customSvc, verSvc: versionSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	RegisterShareRoutes(engine, &ShareHandler{shareSvc: shareSvc, verSvc: versionSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 规则 + 个人中心（Build2 Step 6）：规则 Token 全局共享；改邮箱/密码递增凭据版本号
	ruleSvc := rule.NewService(st, versionSvc, tokenSvc, subSvc, lg)
	RegisterRuleRoutes(engine, &RuleHandler{ruleSvc: ruleSvc, verSvc: versionSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	RegisterProfileRoutes(engine, &ProfileHandler{store: st}, authSvc.SessionMiddleware())
	// 用户管理（Build3 Step 1）：五重管理员保护 + 全生命周期操作；复用 Token/重置令牌/版本组件
	adminUserSvc := user.NewAdminService(st, users, tokenSvc, resetSvc, cfg, versionSvc, lg)
	RegisterUserAdminRoutes(engine, &UserAdminHandler{adminSvc: adminUserSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 邮件服务 + 审批中心（Build3 Step 2）：接通密码重置邮件与欢迎邮件注入点（SMTP 未配置/失败不阻断主流程）
	mailSvc := mail.NewService(cfg, lg)
	resetSvc.SetSendMail(func(ctx context.Context, to, resetURL string) error {
		furl, _ := cfg.Get(ctx, config.KeyFrontendURL)
		return mailSvc.SendPasswordReset(ctx, to, furl+resetURL)
	})
	users.SetWelcomeSender(func(ctx context.Context, to, source string) error {
		siteName, _ := cfg.Get(ctx, "site_name")
		loginURL, _ := cfg.Get(ctx, config.KeyFrontendURL)
		return mailSvc.SendWelcome(ctx, to, siteName, loginURL, source)
	})
	approvalSvc := approval.NewService(st, mailSvc, cfg, lg)
	RegisterApprovalRoutes(engine, &ApprovalHandler{approvalSvc: approvalSvc, mailSvc: mailSvc, users: users},
		authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 面板配置（Build3 Step 3）：分区读写 + 死锁防护 + 加密脱敏；接通调试模式 5xx 详情
	response.SetDebugProvider(func(ctx context.Context) bool {
		return cfg.GetBool(ctx, "debug_mode", false)
	})
	adminCfgSvc := config.NewAdminService(cfg, st, oidcOpsAdapter{svc: oidcSvc}, dataDir, lg)
	RegisterSettingsRoutes(engine, &SettingsHandler{adminCfg: adminCfgSvc, oidcSvc: oidcSvc, trustProxy: trustProxy},
		authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 运维端点（Build3 Step 4）：一键清空/配置导入导出/备份下载；内存态复位回调（Step 5 追加 SSE 复位）
	clearSvc := dataclear.NewService(st, dataDir, lg)
	if streamSvc != nil {
		clearSvc.SetResetRuntimeState(func() { limiter.Reset(); streamSvc.Reset() }) // 限流计数 + SSE 连接/短期 Token/日志缓冲同步重置
	} else {
		clearSvc.SetResetRuntimeState(limiter.Reset) // 限流计数同步重置
	}
	exportSvc := config.NewExportService(st, cfg, dataDir, mode, lg)
	exportSvc.SetSeedPresets(setupSvc.SeedPresetsTx) // Setup 导入分支预置默认组/平台
	backupSvc := backup.NewService(st, dataDir, lg)
	RegisterSettingsOpsRoutes(engine, &SettingsOpsHandler{
		clearSvc: clearSvc, exportSvc: exportSvc, backupSvc: backupSvc, setupSvc: setupSvc, limiter: limiter,
	}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 日志查看（Build3 Step 5）：访问日志查询/清空 + 实时日志流 SSE（短期 Token + 8 连接上限）
	accessLogSvc := log.NewAccessService(st.DB(), lg)
	RegisterLogRoutes(engine, &LogHandler{accessSvc: accessLogSvc, streamSvc: streamSvc},
		authSvc.SessionMiddleware(), auth.AdminMiddleware())
	if err := registerStatic(engine, dataDir); err != nil {
		return nil, err
	}
	return s, nil
}

// NewEmergency 应急模式装配（Build3 Step 6）：仅注册 系统状态/站点信息/应急端点/静态资源（/assets、/public、SPA 回退）；
// 业务 API 与下载端点由 emergencyGate 拦截返回 503；/health 返回 503（Build1 预留注释在此接通）。
// 仅在 main 按 emergency.Detect 分支调用，正常运行时不使用
func NewEmergency(st *store.Store, cfg *config.Service, emSvc *emergency.Service, lg *slog.Logger, mode, trustProxy, port, dataDir string) (*Server, error) {
	engine := gin.New()
	if err := applyTrustProxy(engine, trustProxy); err != nil {
		return nil, err
	}
	engine.Use(requestLogger(), panicRecovery(), emergencyGate())
	s := &Server{engine: engine, cfg: cfg, store: st, mode: mode, log: lg,
		httpSrv: &http.Server{Addr: ":" + port, Handler: engine}}
	// /health 应急模式返回 503（docker compose 仅状态展示，不触发重启）
	engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "emergency"})
	})
	// 系统状态（emergency 标记 + 触发原因 + 可用能力）；站点信息公开端点
	users := user.NewService(st, cfg, lg)
	authSvc := auth.NewService(cfg, users, lg)
	oidcSvc := oidc.NewService(st, cfg, authSvc, users, mode, lg)
	captchaSvc := captcha.NewService(cfg, lg)
	registerStatus(engine, cfg, users, oidcSvc, captchaSvc, mode, emSvc)
	engine.GET("/api/site/info", func(c *gin.Context) {
		ctx := c.Request.Context()
		name, _ := cfg.Get(ctx, "site_name")
		icon, _ := cfg.Get(ctx, "site_icon_url")
		OK(c, gin.H{"site_name": name, "icon_url": icon})
	})
	// 应急端点（仅应急模式注册）
	RegisterEmergencyRoutes(engine, &EmergencyHandler{emSvc: emSvc})
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
