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

	"vpn-sub/internal/approval"
	"vpn-sub/internal/assembly"
	"vpn-sub/internal/auth"
	"vpn-sub/internal/backup"
	"vpn-sub/internal/captcha"
	"vpn-sub/internal/config"
	"vpn-sub/internal/cron"
	"vpn-sub/internal/custom"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/download"
	"vpn-sub/internal/emergency"
	"vpn-sub/internal/group"
	"vpn-sub/internal/home"
	"vpn-sub/internal/log"
	"vpn-sub/internal/mail"
	"vpn-sub/internal/node"
	"vpn-sub/internal/oidc"
	"vpn-sub/internal/platform"
	"vpn-sub/internal/pool"
	"vpn-sub/internal/proxygroup"
	"vpn-sub/internal/ratelimit"
	"vpn-sub/internal/response"
	"vpn-sub/internal/rule"
	"vpn-sub/internal/setup"
	"vpn-sub/internal/share"
	"vpn-sub/internal/store"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/tasks"
	"vpn-sub/internal/token"
	"vpn-sub/internal/user"
	"vpn-sub/internal/version"
	"vpn-sub/internal/xray"
)

// Response 统一响应结构（类型别名，定义见 internal/response）
type Response = response.Response

// ListData 列表包裹结构（类型别名）
type ListData = response.ListData

// OK 成功响应（便捷包装）
func OK(c *gin.Context, data any) { response.OK(c, data) }

// Fail 错误响应（便捷包装）；httpStatus 与业务码同步取值（400/401/403/409/429/500）
func Fail(c *gin.Context, httpStatus int, msg string) { response.Fail(c, httpStatus, msg) }

// detach 将事务提交后的副作用回调放入后台 goroutine，并解除请求 Context 的取消绑定。
func detach(ctx context.Context, fn func(context.Context)) {
	bg := context.WithoutCancel(ctx)
	go fn(bg)
}

type Server struct {
	engine          *gin.Engine
	httpSrv         *http.Server
	cfg             *config.Service
	store           *store.Store
	mode            string
	log             *slog.Logger
	stopXrayCollect func()
	xrayInstances   *xray.InstanceService
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
	RegisterOidcRoutes(engine, &OidcHandler{oidcSvc: oidcSvc, authSvc: authSvc}, authSvc.SessionMiddleware(), limiter)
	// 版本组件 + 订阅池路由（Build2 Step 2；会话 + 管理员双中间件）
	versionSvc := version.NewService(st, dataDir, lg)
	// 平台路由（Build2 Step 1；会话 + 管理员双中间件；Step 5 起持有版本组件用于完整级联）
	platformSvc := platform.NewService(st, dataDir, versionSvc, lg)
	RegisterPlatformRoutes(engine, &PlatformHandler{platformSvc: platformSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	subSvc := subscription.NewService(st, versionSvc, lg)
	subHandler := &SubscriptionHandler{subSvc: subSvc, verSvc: versionSvc}
	RegisterSubscriptionRoutes(engine, subHandler, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 用户组服务与路由（旧分发模型已拆除：组仅保留基础 CRUD，节点分配/默认配额由 Build6 接入）
	groupSvc := group.NewService(st, lg)
	RegisterGroupRoutes(engine, &GroupHandler{groupSvc: groupSvc, cfg: cfg}, authSvc.SessionMiddleware(), auth.AdminMiddleware(), AdvancedMode(cfg))
	// 候选集重算接线：平台删除、订阅删除、版本切换
	subHandler.onVersionSwitched = func(ctx context.Context, ot version.OwnerType, ownerID int64) {
		if ot != version.OwnerSubscription {
			return
		}
		detach(ctx, func(ctx context.Context) {
			if _, err := groupSvc.RecomputeCandidateSet(ctx); err != nil {
				lg.Warn("版本切换后候选集重算失败", "err", err)
			}
		})
	}
	platformSvc.SetOnAfterDelete(func(ctx context.Context) {
		detach(ctx, func(ctx context.Context) {
			if _, err := groupSvc.RecomputeCandidateSet(ctx); err != nil {
				lg.Warn("平台删除后候选集重算失败", "err", err)
			}
		})
	})
	subSvc.SetOnAfterDelete(func(ctx context.Context, _ int64) {
		detach(ctx, func(ctx context.Context) {
			if _, err := groupSvc.RecomputeCandidateSet(ctx); err != nil {
				lg.Warn("订阅删除后候选集重算失败", "err", err)
			}
		})
	})
	// 规则素材池（Build4 Step 5：CRUD / 条目 / 异步同步与历史任务）
	poolSvc := pool.NewService(st, lg)
	RegisterPoolRoutes(engine, &PoolHandler{poolSvc: poolSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 节点服务（Build5 Step 1：manual 节点 CRUD + 协议注册表 + xray 显示名/启停占位）
	nodeSvc := node.NewService(st, cfg, lg)
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// 全局长任务 registry + Xray 实例/检测路由（Build6 Step1）
	taskReg := tasks.NewRegistry()
	RegisterTasksRoutes(engine, &TasksHandler{registry: taskReg}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	xraySvc := xray.NewInstanceService(st, lg, taskReg)
	s.xrayInstances = xraySvc
	credsSvc := xray.NewCredentialService(st, cfg)
	extSvc := xray.NewExtService(st, cfg, xraySvc, lg)
	syncSvc := xray.NewSyncService(st, cfg, credsSvc, xraySvc, taskReg, lg)
	syncSvc.SetExtService(extSvc)
	// 组节点变化/删除后对受影响用户做精确 diff（Build6-2 补强）
	groupSvc.SetOnNodesChanged(func(ctx context.Context, _ int64, userIDs []int64) {
		detach(ctx, func(ctx context.Context) {
			for _, uid := range userIDs {
				if err := syncSvc.ReconcileUser(ctx, uid); err != nil {
					lg.Warn("组节点变化后同步失败", "user_id", uid, "err", err)
				}
			}
		})
	})
	s.stopXrayCollect = cron.StartXrayCollect(st, xraySvc, syncSvc, extSvc, cfg, lg)
	RegisterXrayRoutes(engine, &XrayHandler{instanceSvc: xraySvc, syncSvc: syncSvc, extSvc: extSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware(), AdvancedMode(cfg))
	// 候选集重算与同步 diff 接线：节点启停/公共变化、检测可见性变化
	nodeSvc.SetOnXrayChanged(func(ctx context.Context, n node.Node, oldEnabled, oldPublic bool) {
		detach(ctx, func(ctx context.Context) {
			if _, err := groupSvc.RecomputeCandidateSet(ctx); err != nil {
				lg.Warn("节点变化后候选集重算失败", "err", err)
			}
			// Build6-2 精确 diff：只同步受该节点影响的 active 用户
			if err := syncSvc.SyncUsersForNodes(ctx, []int64{n.ID}); err != nil {
				lg.Warn("节点变化后同步失败", "err", err)
			}
		})
	})
	xraySvc.SetOnNodeVisibilityChanged(func(ctx context.Context, changes []xray.NodeChange) {
		detach(ctx, func(ctx context.Context) {
			if _, err := groupSvc.RecomputeCandidateSet(ctx); err != nil {
				lg.Warn("节点检测后候选集重算失败", "err", err)
			}
			ids := make([]int64, 0, len(changes))
			seen := map[int64]bool{}
			for _, ch := range changes {
				if ch.NodeID == 0 || seen[ch.NodeID] {
					continue
				}
				seen[ch.NodeID] = true
				ids = append(ids, ch.NodeID)
			}
			if err := syncSvc.SyncUsersForNodes(ctx, ids); err != nil {
				lg.Warn("节点检测后同步失败", "err", err)
			}
		})
	})
	// 节点删除后的 Xray 清理钩子（Step1 先落地，Step3 可升级为期望集口径）
	nodeSvc.SetOnXrayNodeDeleted(func(ctx context.Context, targets []node.XrayDeleteTarget) {
		detach(ctx, func(ctx context.Context) {
			for _, t := range targets {
				if t.APIAddr == "" || t.Tag == "" || t.Email == "" {
					continue
				}
				client, err := xray.Dial(t.APIAddr)
				if err != nil {
					lg.Warn("节点删除后清理 Xray 用户失败（拨号）", "email", t.Email, "addr", t.APIAddr, "err", err)
					continue
				}
				rctx, cancel := context.WithTimeout(ctx, xray.RPCTimeout)
				err = client.RemoveUser(rctx, t.Tag, t.Email)
				cancel()
				_ = client.Close()
				if err != nil && !xray.IsNotFound(err) {
					lg.Warn("节点删除后清理 Xray 用户失败", "email", t.Email, "tag", t.Tag, "err", err)
				}
			}
		})
	})
	// 代理组服务（Build5 Step 2：预设/自建组 CRUD + DAG + 内容约束）
	proxyGroupSvc := proxygroup.NewService(st, lg)
	RegisterProxyGroupRoutes(engine, &ProxyGroupHandler{groupSvc: proxyGroupSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	// Token 服务 + 下载解析 + 用户端数据：订阅删除 Token 级联回调注入
	tokenSvc := token.NewService(st, lg)
	subSvc.SetOnTokenDeleted(tokenSvc.DeleteBySubscriptionTx)
	dlSvc := download.NewService(st, versionSvc, cfg, lg)
	dlSvc.SetRenderUser(func(ctx context.Context, subID, userID int64, content []byte, fileName string) ([]byte, error) {
		return renderUserSubscription(ctx, st, cfg, syncSvc, credsSvc, subID, userID, content, fileName)
	})
	homeSvc := home.NewService(st, tokenSvc, cfg)
	homeHandler := &HomeHandler{homeSvc: homeSvc, st: st, cfg: cfg, syncSvc: syncSvc}
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
	// 装配端点（Build5 Step 4：context/preview/generate/blueprint）
	assemblySvc := assembly.NewService(st, cfg, lg)
	RegisterAssemblyRoutes(engine, &AssemblyHandler{
		assemblySvc: assemblySvc, nodeSvc: nodeSvc, proxyGroupSvc: proxyGroupSvc,
		poolSvc: poolSvc, platformSvc: platformSvc, ruleSvc: ruleSvc, versionSvc: versionSvc, subSvc: subSvc,
		onGenerateActivated: func(ctx context.Context) {
			detach(ctx, func(ctx context.Context) {
				if _, err := groupSvc.RecomputeCandidateSet(ctx); err != nil {
					lg.Warn("装配首版激活后候选集重算失败", "err", err)
				}
			})
		},
	}, authSvc.SessionMiddleware(), auth.AdminMiddleware())
	RegisterProfileRoutes(engine, &ProfileHandler{userSvc: users, st: st, cfg: cfg, syncSvc: syncSvc}, authSvc.SessionMiddleware())
	// 用户管理（Build3 Step 1）：五重管理员保护 + 全生命周期操作；复用 Token/重置令牌/版本组件
	adminUserSvc := user.NewAdminService(st, users, tokenSvc, resetSvc, cfg, versionSvc, lg)
	RegisterUserAdminRoutes(engine, &UserAdminHandler{adminSvc: adminUserSvc}, authSvc.SessionMiddleware(), auth.AdminMiddleware(), AdvancedMode(cfg))
	// 用户生命周期 Xray 同步接线（Build6 Step3）
	users.SetOnUserActive(func(ctx context.Context, userID int64) {
		detach(ctx, func(ctx context.Context) {
			if err := syncSvc.ReconcileUser(ctx, userID); err != nil {
				lg.Warn("用户激活后 Xray 同步失败", "user_id", userID, "err", err)
			}
		})
	})
	adminUserSvc.SetOnUserActive(func(ctx context.Context, userID int64) {
		detach(ctx, func(ctx context.Context) {
			if err := syncSvc.ReconcileUser(ctx, userID); err != nil {
				lg.Warn("管理员激活用户后 Xray 同步失败", "user_id", userID, "err", err)
			}
		})
	})
	adminUserSvc.SetOnUserDisabled(func(ctx context.Context, userID int64) {
		detach(ctx, func(ctx context.Context) {
			targets, err := syncSvc.Targets(ctx, userID)
			if err != nil {
				lg.Warn("读取禁用用户目标失败", "user_id", userID, "err", err)
				return
			}
			if _, _, err := syncSvc.RemoveUserFromTargets(ctx, userID, targets); err != nil {
				lg.Warn("禁用用户后 Xray 移除失败", "user_id", userID, "err", err)
			}
		})
	})
	adminUserSvc.SetOnUserGroupChanged(func(ctx context.Context, userID int64) {
		detach(ctx, func(ctx context.Context) {
			if err := syncSvc.ReconcileUser(ctx, userID); err != nil {
				lg.Warn("换组后 Xray 同步失败", "user_id", userID, "err", err)
			}
		})
	})
	// 用户删除前按“当前期望目标集”收集清理目标，删除后执行 RemoveUser（Build6-2 补强）
	adminUserSvc.SetOnUserDeleting(func(ctx context.Context, userID int64) ([]xray.Target, error) {
		return syncSvc.Targets(ctx, userID)
	})
	adminUserSvc.SetOnUserDeleted(func(ctx context.Context, userID int64, targets []xray.Target) {
		detach(ctx, func(ctx context.Context) {
			if _, _, err := syncSvc.RemoveUserFromTargets(ctx, userID, targets); err != nil {
				lg.Warn("删除用户后 Xray 清理失败", "user_id", userID, "err", err)
			}
		})
	})
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
	approvalSvc.SetOnApproved(func(ctx context.Context, userID int64) {
		detach(ctx, func(ctx context.Context) {
			if err := syncSvc.ReconcileUser(ctx, userID); err != nil {
				lg.Warn("审批通过后 Xray 同步失败", "user_id", userID, "err", err)
			}
		})
	})
	approvalSvc.SetOnUserDeleting(func(ctx context.Context, userID int64) ([]xray.Target, error) {
		return syncSvc.Targets(ctx, userID)
	})
	approvalSvc.SetOnUserDeleted(func(ctx context.Context, userID int64, targets []xray.Target) {
		detach(ctx, func(ctx context.Context) {
			if _, _, err := syncSvc.RemoveUserFromTargets(ctx, userID, targets); err != nil {
				lg.Warn("审批拒绝后 Xray 清理失败", "user_id", userID, "err", err)
			}
		})
	})
	// 面板配置（Build3 Step 3）：分区读写 + 死锁防护 + 加密脱敏；接通调试模式 5xx 详情
	response.SetDebugProvider(func(ctx context.Context) bool {
		return cfg.GetBool(ctx, "debug_mode", false)
	})
	offClearSvc := xray.NewOffClearService(st, cfg, taskReg, lg)
	offClearSvc.SetAfterAdvancedOff(syncSvc.AfterAdvancedOff)
	adminCfgSvc := config.NewAdminService(cfg, st, oidcOpsAdapter{svc: oidcSvc}, dataDir, lg)
	adminCfgSvc.SetAdvancedModeSwitcher(offClearSvc)
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
	exportSvc.SetTaskRegistry(taskReg)
	// v2 导入后处理：旧 Xray 清理、自动检测、显示名/ext 重绑、装配重绑与对账
	exportSvc.SetCleanupXrayTargets(func(ctx context.Context, targets []config.ImportCleanupTarget) {
		for _, t := range targets {
			if t.APIAddr == "" || t.Tag == "" || t.Email == "" {
				continue
			}
			client, err := xray.Dial(t.APIAddr)
			if err != nil {
				lg.Warn("导入后清理旧 Xray 账号失败（拨号）", "email", t.Email, "addr", t.APIAddr, "err", err)
				continue
			}
			rctx, cancel := context.WithTimeout(ctx, xray.RPCTimeout)
			err = client.RemoveUser(rctx, t.Tag, t.Email)
			cancel()
			_ = client.Close()
			if err != nil && !xray.IsNotFound(err) {
				lg.Warn("导入后清理旧 Xray 账号失败", "email", t.Email, "tag", t.Tag, "err", err)
			}
		}
	})
	exportSvc.SetDetectImportedInstances(func(ctx context.Context, payload *config.ExportPayload) []string {
		var hints []string
		instances, err := xraySvc.List(ctx)
		if err != nil {
			return append(hints, "读取导入实例列表失败: "+err.Error())
		}
		bySlug := map[string]xray.Instance{}
		for _, inst := range instances {
			bySlug[inst.Slug] = inst
		}
		for _, exp := range payload.Instances {
			inst, ok := bySlug[exp.Slug]
			if !ok {
				hints = append(hints, "实例 "+exp.Slug+" 未找到，跳过检测")
				continue
			}
			if !inst.Enabled {
				hints = append(hints, "实例 "+exp.Name+" 已停用，跳过自动检测")
				continue
			}
			if _, err := xraySvc.DetectNodes(ctx, inst.ID); err != nil {
				hints = append(hints, "实例 "+exp.Name+" 自动检测失败: "+err.Error())
			}
		}
		return hints
	})
	exportSvc.SetPostImportRebindReconcile(func(ctx context.Context, _ *config.ExportPayload) []string {
		var hints []string
		if refHints, err := assemblySvc.CheckXrayReferences(ctx); err != nil {
			hints = append(hints, "装配快照 Xray 引用核对失败: "+err.Error())
		} else {
			hints = append(hints, refHints...)
		}
		// best-effort 账号对账：遍历启用实例执行 Reconcile + PushOne，失败不阻断导入任务。
		instances, err := xraySvc.List(ctx)
		if err != nil {
			return append(hints, "读取实例列表失败: "+err.Error())
		}
		for _, inst := range instances {
			if !inst.Enabled {
				continue
			}
			res, err := syncSvc.Reconcile(ctx, inst.ID)
			if err != nil {
				hints = append(hints, fmt.Sprintf("实例 %s 对账失败: %v", inst.Name, err))
				continue
			}
			for _, item := range res.ToPush {
				if err := syncSvc.PushOne(ctx, item); err != nil {
					hints = append(hints, fmt.Sprintf("实例 %s 补推 %s/%s 失败: %v", inst.Name, item.Email, item.InboundTag, err))
				}
			}
		}
		return hints
	})
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
		if s.stopXrayCollect != nil {
			s.stopXrayCollect()
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpSrv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("优雅退出失败: %w", err)
		}
		if s.xrayInstances != nil {
			s.xrayInstances.CloseAll()
		}
		return nil
	}
}
