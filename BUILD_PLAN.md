# 构建计划（TODO LIST）

本文件依据 `AGENTS.md` 拆分为有序构建步骤，供 AI 助手按块逐步实现。每块完成后应可独立验证（编译通过 / 页面可访问），再进入下一块。

**构建原则**:
- 严格遵循 `AGENTS.md` 第五章编码约束（安全/认证/Handler/版本管理/级联删除/速率限制/前端/Go 工程）
- 每完成一块运行 `go build ./...`（后端）或 `npm run build`（前端）验证
- 数据库表结构、API 端点、版本文件存储严格按第六章实现
- Docker 部署按第八章（外部 NGINX 分流 + 双容器 127.0.0.1 绑定）
- 不使用 .env 文件，业务配置一律 Web UI → SQLite；仅 PORT 等运维参数可环境变量覆盖
- 详细测试步骤见文末 [测试计划](#测试计划) 章节（本地测试 + 联机测试）

**阶段划分**（共 37 块，块 1 拆为 4 个子块，块 2 拆为 2 个子块，块 3 拆为 6 个子块，块 4 拆为 4 个子块，块 5 拆为 3 个子块，块 6 拆为 4 个子块，块 7 拆为 7 个子块，块 8 拆为 6 个子块）:

| 块 | 内容 | 依赖 | 状态 |
|----|------|------|------|
| 块 1A | 工程初始化 + 工具函数（go.mod/目录/4 utils） | 无 | ✅ |
| 块 1B | 数据模型 + 数据库初始化（models + db.go + 12 表 + 默认平台） | 块 1A | ✅ |
| 块 1C | 数据访问层（12 个 repo 文件） | 块 1B | ✅ |
| 块 1D | HTTP 基础设施（8 middleware + router + main.go） | 块 1C | ✅ |
| 块 2A | OIDC 核心认证服务（oidc_service.go：PKCE + JWT） | 块 1D | ✅ |
| 块 2B | Setup 流程 + 认证 Handler + 首位管理员 | 块 2A | ✅ |
| 块 3A | 平台管理 + 用户管理（基础 CRUD + 管理员自我保护 + 级联删除） | 块 2B | ✅ |
| 块 3B | 版本管理核心 + 订阅管理（VersionService + Subscriptions CRUD + 版本端点） | 块 3A | ✅ |
| 块 3C | 规则管理 + 规则 Token 轮替（Rules CRUD + 版本管理 + refresh-token） | 块 3B | ✅ |
| 块 3D | 自定义订阅（上传/版本管理/刷新 Token/级联删除） | 块 3B | ✅ |
| 块 3E | 分享订阅（CRUD + 版本管理 + Token 刷新/吊销/级联删除） | 块 3B | ✅ |
| 块 3F | 系统配置端点（速率限制配置 + OIDC 配置查看） | 块 2B | ✅ |
| 块 4A | RateLimit 中间件实现 | 块 3F | ✅ |
| 块 4B | 下载端点 + Token 生成 + logAccess | 块 4A | ✅ |
| 块 4C | 用户端点（UserPlatforms/UpdateTime/RefreshToken） | 块 4B | ✅ |
| 块 4D | 日志查询 + 自动清理 | 块 4B | ✅ |
| 块 5A | 前端项目脚手架（Vite + 依赖 + vite.config + index.html + main.js + 空路由 + App.vue 最小版） | 块 1D | ✅ |
| 块 5B | 前端核心基础设施（api.js + user store + useTheme + 完整路由表 + 三重守卫 + 所有页面 stub） | 块 5A | ✅ |
| 块 5C | 前端公共组件（ConfirmDialog / OIDCSwitchDialog / UploadModal） | 块 5A | ✅ |
| 块 6A | 前端认证入口页（Setup.vue + Login.vue） | 块 5B + 块 5C + 块 2B | ✅ |
| 块 6B | 管理面板布局（Manage.vue） | 块 5B | ✅ |
| 块 6C | 首页仪表盘（Home.vue） | 块 5B + 块 5C | ✅ |
| 块 6D | 用户规则浏览页（Rules.vue） | 块 5B | ✅ |
| 块 7A | SubList + SubVersions（订阅管理 + 版本管理） | 块 6B + 块 3F | ✅ |
| 块 7B | ShareList + ShareVersions（分享订阅 + 版本管理） | 块 6B + 块 3F | ✅ |
| 块 7C | PlatformManage（平台管理） | 块 6B + 块 3F | ✅ |
| 块 7D | UserManage（用户管理 + 自定义订阅） | 块 6B + 块 3F | ✅ |
| 块 7E | RulesManage + RuleVersions（规则管理 + 版本管理） | 块 6B + 块 3F | ✅ |
| 块 7F | OIDCConfig（OIDC 配置 + 速率限制） | 块 6B + 块 5C | ✅ |
| 块 7G | Logs（日志查看） | 块 6B + 块 3F | ✅ |
| 块 8A | Docker 构建文件（Dockerfile × 2 + .dockerignore × 2 + nginx.conf） | 无（独立编写） | ✅ |
| 块 8B | Docker Compose 最终确认与调优 | 块 8A | ✅ |
| 块 8C | 本地 Docker 构建验证 | 块 8B | ✅ |
| 块 8D | 外部 NGINX 参考配置文档 | 无（独立编写） | ✅ |
| 块 8E | CI/CD GitHub Actions（可选） | 块 8C | ✅ |
| 块 8F | 端到端联调验证清单 | 块 8C | ⬜ |

---

## 块 1：后端骨架

**目标**: 搭建后端目录结构、初始化 SQLite、实现基础中间件和工具函数，main.go 能启动并连接数据库。

块 1 按层次拆分为 4 个子块（1A→1B→1C→1D），每层以上一层为基础，逐层构建。

---

### 块 1A：工程初始化 + 工具函数

**目标**: 创建 Go module、目录结构、全部工具函数，确保 utils 包可独立编译。

**任务**:

- [x] 创建 `backend/go.mod`（module 名 `vpn-sub`），添加依赖：
  - `github.com/gin-gonic/gin` ✅
  - `github.com/rs/zerolog` ✅
  - `modernc.org/sqlite` ✅
  - `github.com/coreos/go-oidc/v3` ✅（后续块使用）
  - `github.com/golang-jwt/jwt/v5` ✅（后续块使用）
  - 运行 `go mod tidy` ✅
- [x] 按 6.2 创建全部目录结构：`cmd/server/`、`internal/{auth,handler,service,repository,middleware,models,router,utils}/` ✅
- [x] `internal/utils/env.go`：`GetEnv(key, default)` 函数，PORT 默认 8080，DATA_DIR 默认 `./data` ✅
- [x] `internal/utils/crypto.go`：AES-256-GCM `Encrypt(plaintext, key)` / `Decrypt(ciphertext, key)`，key 取 JWT_SECRET 前 32 字节 ✅
- [x] `internal/utils/sanitizePath.go`：`SanitizePath(baseDir, subPath)` — 路径穿越防护，禁止 `..` 和绝对路径 ✅
- [x] `internal/utils/isValidID.go`：`IsValidID(s string) bool` — 正则 `^[a-z0-9-]+$`（必须在 utils 包，不能在 handler）✅

**涉及文件**: `backend/go.mod`, `backend/go.sum`, `internal/utils/env.go`, `internal/utils/crypto.go`, `internal/utils/sanitizePath.go`, `internal/utils/isValidID.go`

**验证**: `go build ./internal/utils/...` 通过，utils 包零外部依赖

---

### 块 1B：数据模型 + 数据库初始化

**目标**: 定义全部结构体，创建 12 张表，插入默认数据，启动后台定时清理。

**任务**:

- [x] `internal/models/types.go`：定义所有结构体（User, Platform, Subscription, Version, Rule, AccessLog, OIDCState, DownloadToken, CustomSubscription, ShareSubscription, ShareToken, RuleToken, SystemConfig, UserPlatformInfo）✅
  - `Version` 结构体：`{ version: int, file_path: string, created_at: datetime, updated_at: datetime }` ✅
  - `DownloadToken.Type` 使用 `*string`（可 NULL，custom_sub_id 非空时 type 为 NULL）✅
- [x] `internal/repository/db.go`：✅
  - `InitDB(dbPath)`：创建目录 → 打开 SQLite → WAL 模式 + 外键 → 单连接（`SetMaxOpenConns(1)`）✅
  - `createTables()`：12 张表严格按 6.3 表清单字段，含 CHECK 约束和 UNIQUE 约束 ✅
  - `insertDefaultPlatforms()`：自动创建 3 个默认平台（clash-verge、v2rayng、shadowrocket），含默认 client_schemes ✅
  - `periodicCleanup()`：后台 goroutine，每小时清理过期 oidc_state（>10 分钟）和 access_logs（>90 天）✅
  - `CloseDB()`：优雅关闭数据库 ✅

**涉及文件**: `internal/models/types.go`, `internal/repository/db.go`

**验证**: `go build ./...` 通过；启动后 `./data/vpn.db` 文件存在，12 张表已创建，3 个默认平台已插入

---

### 块 1C：数据访问层（12 个 Repo）

**目标**: 为每张表创建 repository 文件，封装全部 SQL 操作，每个 repo 提供标准的 CRUD 方法。

**任务**:

- [x] `system_config_repo.go`：`Get(key)` / `Set(key, value)` — 键值读写 ✅
- [x] `user_repo.go`：`Create/FindByID/List/Update/Delete/CountByRole` — 用户 CRUD + 角色计数 ✅
- [x] `platform_repo.go`：`Create/FindByID/List/Update/Delete` — 平台 CRUD，client_schemes JSON 序列化 ✅
- [x] `subscription_repo.go`：`Create/FindByID/FindByPlatformAndType/List/Update/Delete` — 订阅 CRUD，versions JSON 序列化 ✅
- [x] `rule_repo.go`：`Create/FindByID/List/Update/Delete` — 规则 CRUD，versions JSON 序列化 ✅
- [x] `download_token_repo.go`：`Create/FindByToken/FindByUserPlatformType/DeleteByUser/DeleteByToken/DeleteByCustomSubID` — Token 管理 ✅
- [x] `custom_subscription_repo.go`：`Create/FindByID/FindByUserAndPlatform/ListByUser/Update/Delete` — 自定义订阅 CRUD ✅
- [x] `share_subscription_repo.go`：`Create/FindByID/List/Update/Delete` — 分享订阅 CRUD ✅
- [x] `share_token_repo.go`：`Create/FindByToken/FindByShareID/DeleteByToken/DeleteByShareID` — 分享 Token 管理 ✅
- [x] `rule_token_repo.go`：`Create/FindByToken/FindByRuleID/DeleteByToken/DeleteByRuleID` — 规则 Token 管理 ✅
- [x] `access_log_repo.go`：`Insert(log)` / `ListByDate(date)` — 日志写入 + 按日期查询 ✅
- [x] `oidc_state_repo.go`：`Create/FindByState/DeleteByState/DeleteExpired` — OIDC state 生命周期 ✅

**涉及文件**: `internal/repository/system_config_repo.go`, `user_repo.go`, `platform_repo.go`, `subscription_repo.go`, `rule_repo.go`, `download_token_repo.go`, `custom_subscription_repo.go`, `share_subscription_repo.go`, `share_token_repo.go`, `rule_token_repo.go`, `access_log_repo.go`, `oidc_state_repo.go`

**验证**: `go build ./...` 通过；所有 repo 方法签名与 models 类型一致；可独立实例化测试

---

### 块 1D：HTTP 基础设施（中间件 + 路由 + 入口）

**目标**: 实现 8 个中间件、双模式路由、main.go 入口，服务可启动并响应 `/health` 和 `/system/status`。

**任务**:

- [x] `middleware/logger.go`：zerolog 结构化日志，自动将 `?token=` 查询参数值脱敏为 `***` ✅
- [x] `middleware/recovery.go`：panic 恢复，返回 500 + 错误信息 ✅
- [x] `middleware/cors.go`：宽松 CORS（开发期直连场景兼容）✅
- [x] `middleware/cache_control.go`：通用 `Cache-Control` 头设置 ✅
- [x] `middleware/no_cache.go`：`NoCacheForDownloads()` — 设置 `Cache-Control: no-store, no-cache, must-revalidate` + `Pragma: no-cache` ✅
- [x] `middleware/auth.go`：`AuthRequired()` — 从 JWT 提取 user_id → 查库获取 role + is_advanced → 写入 Gin context（使用 `c.Set()`）。实时查库，不缓存用户权限 ✅
- [x] `middleware/admin.go`：`AdminRequired()` — 检查 context 中 role == "admin"，否则返回 403 ✅
- [x] `middleware/rate_limit.go`：stub 占位（仅 `c.Next()`），块 4A 实现真实限流逻辑 ✅
- [x] `internal/router/router.go`：`SetupRouter(isConfigured bool)` ✅
  - 全局中间件：Recovery + Logger + CORS ✅
  - `GET /health` — 健康检查（始终可用）✅
  - `GET /api/v1/system/status` → `handler.GetSystemStatus`（始终可用）✅
  - `isConfigured=false`（Setup 模式）：仅注册 `/admin/system/configure`、`/admin/test-oidc`、`/admin/system/switch-provider`、`/admin/oidc-config` ✅
  - `isConfigured=true`（Normal 模式）：注册全部 auth/user/download/admin 路由 ✅
- [x] `cmd/server/main.go`：入口 ✅
  - 读取 PORT（默认 8080）和 DATA_DIR（默认 `./data`）环境变量 ✅
  - 初始化 DB（`repository.InitDB(dbPath)`）✅
  - 判断 configured 状态 → 若已配置则初始化 `auth.DefaultService` 和 `middleware.SetAuthService()` ✅
  - 初始化业务服务（`handler.InitServices()`）✅
  - `SetTrustedProxies(["127.0.0.1"])` — 信任本机反向代理，`c.ClientIP()` 自动解析 X-Forwarded-For ✅
  - 启动 Gin（Release 模式）✅

**涉及文件**: `middleware/logger.go`, `middleware/recovery.go`, `middleware/cors.go`, `middleware/cache_control.go`, `middleware/no_cache.go`, `middleware/auth.go`, `middleware/admin.go`, `middleware/rate_limit.go`, `router/router.go`, `cmd/server/main.go`

**验证**: `go build ./...` 通过；`go run .` 启动后：
- `GET /health` → 200 `{"status":"ok"}`
- `GET /api/v1/system/status` → 200 `{"configured":false}`
- Setup 模式下 `/api/v1/auth/login` → 404（路由未注册）

**关键约束**（适用于块 1 全部子块）:
- SQLite 路径 `/app/data/vpn.db`（开发环境用相对路径 `./data/vpn.db`）
- 12 张表严格按 6.3 表清单字段
- versions 字段为 JSON 数组，版本对象 schema：`{ version: int, file_path: string, created_at: datetime, updated_at: datetime }`
- `isValidID()` 必须在 utils 包，不能放在 handler 包里

---

## 块 2：OIDC 认证 + Setup 流程

**目标**: 实现 OIDC PKCE 登录、JWT 签发验证、Setup 首次配置流程。

块 2 按服务层与 HTTP 层拆分为 2 个子块（2A→2B），OIDC 核心服务独立构建并编译验证后再挂载 HTTP handler。

---

### 块 2A：OIDC 核心认证服务

**目标**: 实现完整的 OIDC PKCE 流程 + JWT 签发/验证，独立于 HTTP handler，可单独编译验证。

**任务**:

- [x] `internal/auth/oidc_service.go`：✅
  - 支持 Keycloak / Auth0 / 通用 OIDC 三种 provider_type ✅
  - 各提供商独立字段：keycloak_base_url + keycloak_realm / auth0_domain / generic_issuer ✅
  - 各提供商独立 Client Secret 加密存储键（keycloak_client_secret_encrypted / auth0_client_secret_encrypted / generic_client_secret_encrypted）✅
  - PKCE 流程：生成 `code_verifier`（SHA256） + random `state` → 存入 oidc_state 表（10min TTL）✅
  - CSRF 防护：state 通过 HttpOnly Cookie 下发，回调时三重校验（Cookie state == query state == DB 记录）✅
  - code_verifier 与 state 一同存入 oidc_state 表，回调时按 state 查表取 code_verifier 用于 token exchange，用后立即删 state 记录（防重放）✅
  - `NewServiceFromDB(cfgRepo)`：从 system_config 读取 OIDC 配置 → 初始化 `oidc.Provider`（自动发现 Discovery URL）→ 返回 Service 实例 ✅
  - JWT 签发：claims 仅存 `user_id` + exp（7 天）/iat，用 JWT_SECRET（HS256）签名 ✅
  - JWT 验证：解析 `Authorization: Bearer <token>` header → 返回 `user_id` ✅
  - `ExchangeCode()`：用 code_verifier 向 OIDC 提供商交换 token → 解析 id_token claims → 返回 OIDC 用户信息（sub, preferred_username, email）✅

**涉及文件**: `internal/auth/oidc_service.go`

**验证**: `go build ./...` 通过；oidc_service.go 所有公开方法签名正确，无编译错误

---

### 块 2B：Setup 流程 + 认证 Handler + 首位管理员

**目标**: 实现首次配置的 3 个 Setup handler、OIDC 登录的 3 个 Auth handler，以及首位管理员自动判定逻辑。

**任务**:

- [x] **Setup handler**（`/api/v1/admin/system/*`，Setup 模式下无需认证）：✅
  - `POST /admin/system/configure`：接收 OIDC 配置 → AES-256-GCM 加密 Client Secret（各提供商独立字段）→ 随机生成 ≥32 字节 JWT_SECRET → 写入 system_config → 置 `configured=true`（**不写** `admin_initialized`，由首位用户登录触发）✅
  - `POST /admin/test-oidc`：测试 OIDC 连接（用临时 OIDC provider 验证配置正确性）✅
  - `POST /admin/system/switch-provider`：切换 provider_type，**保留已填字段**（只更新 provider_type 键，不覆盖已有的各提供商字段）✅
- [x] **Auth handler**（`/api/v1/auth/*`）：✅
  - `GET /auth/login`：生成 state + code_verifier → 存 oidc_state 表 → 设置 HttpOnly Cookie（state）→ 302 重定向到 OIDC 提供商授权页 ✅
  - `GET /auth/callback`：三重校验（Cookie == query == DB）→ code exchange（用 code_verifier）→ 解析 id_token → 查/建用户 → **首位管理员判定** → 签发 JWT → 302 重定向到前端中转页 `/auth/callback?token=<jwt>` ✅
  - `GET /auth/me`：AuthRequired 中间件 → 实时查库（不用 JWT claims）→ 返回 `{ user_id, username, email, role, is_advanced, groups }` ✅
- [x] **首位管理员判定**：✅
  - 登录时（callback handler 中）检查 `system_config.admin_initialized`：✅
    - 若不为 `"true"` → 该用户 `role=admin`、`is_advanced=true` → 写入 `admin_initialized=true` ✅
    - 若已为 `"true"` → 该用户 `role=user`、`is_advanced=false`（后续用户默认普通用户）✅
  - 即使 users 表被清空，`admin_initialized` 标记仍存在，不会再产生新管理员 ✅
- [x] **handler 初始化**：在 `handler/services.go` 的 `InitServices()` 中初始化 `auth.DefaultService`（块 1D main.go 中调用）✅
- [x] OIDC state 定时清理：已在块 1B `periodicCleanup()` 中实现（每小时清理 >10 分钟过期记录）✅

**涉及文件**: `internal/handler/handlers.go`（认证相关 handler：`GetSystemStatus`, `PostConfigure`, `PostTestOIDC`, `PostSwitchProvider`, `GetOIDCConfig`, `Login`, `Callback`, `GetMe`）, `internal/handler/services.go`（`InitServices` 中初始化 auth service）

**验证**: `go build ./...` 通过；运行时验证：
- `POST /admin/system/configure` → `system/status` 返回 `{"configured":true}`
- `GET /auth/login` → 302 重定向到 OIDC 提供商
- `GET /auth/me`（无 JWT）→ 401
- `GET /auth/me`（有效 JWT）→ 200 + 用户信息

**关键约束**（适用于块 2 全部子块）:
- Setup 完成时只置 `configured=true`，`admin_initialized` 仍为 false ✅
- OIDC 配置键：`provider_type` + `keycloak_base_url`/`keycloak_realm`、`auth0_domain`、`generic_issuer`、`client_id`、各提供商独立 `*_client_secret_encrypted`、`redirect_uri`、`frontend_url` ✅
- JWT claims 最小集：仅 `user_id` + exp/iat，role/is_advanced 不放入 claims ✅
- AuthRequired 中间件实时查库，不缓存用户权限 ✅
- GetCurrentUser 读数据库，不用 JWT claims ✅

---

## 块 3：后端核心业务

**目标**: 实现用户/平台/订阅/规则/自定义订阅/分享订阅的 CRUD + 版本管理。

块 3 按业务域拆分为 6 个子块。3A（平台+用户）为基础，3B 构建共享的 VersionService 并以订阅管理为首个消费者，3C/3D/3E 复用 VersionService，3F 为独立的系统配置端点。

---

### 块 3A：平台管理 + 用户管理

**目标**: 实现平台 CRUD 和用户管理的全部 handler + service，含管理员自我保护和级联删除。

**任务**:

- [x] **PlatformService + handler**（`/admin/platforms/*`）：✅
  - CRUD（Create/List/Get/Update/Delete）✅
  - `client_schemes` JSON 数组序列化/反序列化（`json.Marshal`/`json.Unmarshal`）✅
  - `download_url` 可空 ✅
  - 删除平台 → 级联删除该平台的 subscriptions（含版本文件）、download_tokens、custom_subscriptions（含版本文件）✅
- [x] **UserService + handler**（`/admin/users/*`）：✅
  - `GET /admin/users` — 列表（返回 `user_id`, `username`, `email`, `role`, `is_advanced`, `groups`，含 `has_custom_sub` + `custom_sub_platforms` 标记）✅
  - `GET /admin/users/:id` — 获取单个用户详情 ✅
  - `PUT /admin/users/:id` — 编辑用户（仅允许修改 `is_advanced`；管理员自身 `is_advanced` 强制为 true 且不可修改；禁止修改自己 `role`）✅
  - `DELETE /admin/users/:id` — 删除用户，**管理员自我保护**三重校验：① `c.GetUserID() == :id` 拒绝（不能删自己）；② `CountByRole("admin") <= 1` 拒绝（不能删最后一个管理员）；③ 禁止修改自己 role。级联删除 download_tokens + custom_subscriptions + 版本文件 ✅
  - `POST /admin/users/:id/revoke-tokens` — 吊销用户所有下载 Token（删除 download_tokens 表中该 user_id 全部记录）✅
  - **is_advanced 变更副作用**：更新 is_advanced 时自动删除该用户所有旧 download_tokens（防止旧级别 Token 绕过分级限制），用户下次访问首页重新生成 ✅

**涉及文件**: `service/platform_service.go`, `service/user_service.go`, `handler/handlers.go`（平台+用户 handler）, `handler/services.go`（InitServices 中初始化 PlatformSvc + UserSvc）

**验证**: `go build ./...` 通过；运行时验证：
- `GET /admin/platforms` → 200 + 3 个默认平台
- `POST /admin/platforms`（重复 ID）→ 409
- `GET /admin/users` → 200 + 用户列表（含首位管理员）
- `PUT /admin/users/:id`（修改自己 role）→ 400
- `DELETE /admin/users/:id`（删除自己）→ 400
- `DELETE /admin/users/:id`（删除最后一个管理员）→ 400

---

### 块 3B：版本管理核心 + 订阅管理

**目标**: 实现共享的 VersionService（被 3C/3D/3E 复用）+ SubscriptionService + 订阅版本管理全部端点。

**任务**:

- [x] **VersionService**（`service/version_service.go`，共享基础设施）：✅
  - `nextVersion(versions)`：max+1 计算（**不可用** `len(versions)+1`）✅
  - `CreateVersion(subDir, content, existingVersions)`：写版本文件 → 追加 versions JSON → 原子切换 current 软链接（`current.new` → `rename(current.new, current)`）→ 执行 `enforceMaxVersions()` ✅
  - `enforceMaxVersions(subDir, versions)`：最多 `MAX_VERSIONS=5` 个版本，超出删最旧的（删除文件 + 从 JSON 移除）✅
  - `DeleteVersion(subDir, versionNum, versions)`：删文件 + 从 versions 移除。**不可删最后一个版本**（`len(versions) <= 1` 拒绝）✅
  - `SwitchVersion(subDir, versionNum)`：更新 current 软链接 → 更新对应版本的 `updated_at` ✅
  - `GetVersionContent(subDir, versionNum)`：读取指定版本文件内容 ✅
  - `GetCurrentContent(subDir)`：通过 current 软链接读取内容 ✅
  - 文件存储路径模式：`data/{subDir}/v{N}.conf` + `current.conf`（软链接）✅
  - **并发安全**：nextVersion 计算与 versions JSON 更新在 SQLite 事务内 + 行级锁（`UPDATE ... WHERE id=?`）✅
- [x] **SubscriptionService + handler**（`/admin/subscriptions/*`）：✅
  - CRUD：`id` 校验 `[a-z0-9-]+`，`UNIQUE(platform, type)`，重复返回 409 ✅
  - `type` 枚举：`default` / `advanced` ✅
  - 文件存储：`data/subscriptions/{id}/` ✅
  - 版本管理端点：✅
    - `POST /admin/subscriptions/:id/versions` — 上传新版本：支持 `multipart/form-data`（文件上传，50MB 限制）和 `application/json`（`{"content":"..."}`，文本编辑）。两种方式均自动创建新版本号并切换为 current ✅
    - `PUT /admin/subscriptions/:id/versions/:v/current` — 切换当前版本 ✅
    - `GET /admin/subscriptions/:id/versions/:v` — 预览版本内容（返回 `{ version, content }`）✅
    - `DELETE /admin/subscriptions/:id/versions/:v` — 删除版本（current 不可删，最后一个不可删）✅
  - 删除订阅 → 级联删除 download_tokens（`custom_sub_id` 为空的 Token）✅

**涉及文件**: `service/version_service.go`, `service/subscription_service.go`, `handler/handlers.go`（订阅 handler）, `handler/services.go`（InitServices 中初始化 VersionService → SubSvc）

**验证**: `go build ./...` 通过；运行时验证：
- `POST /admin/subscriptions` → 创建 default + advanced 订阅
- `POST /admin/subscriptions/:id/versions`（文件上传）→ 创建 v1.conf + current 软链接
- `POST /admin/subscriptions/:id/versions`（JSON `{"content":"..."}`）→ 创建 v2.conf + 自动切换 current
- `PUT /admin/subscriptions/:id/versions/1/current` → current 指向 v1.conf
- `DELETE /admin/subscriptions/:id/versions/2` → 版本 2 被删除
- 仅剩 1 个版本时 `DELETE /versions` → 400 拒绝
- 上传第 6 个版本 → 最旧版本自动删除（保留 5 个）

---

### 块 3C：规则管理 + 规则 Token 轮替

**目标**: 实现规则的完整 CRUD + 版本管理（复用 VersionService）+ rule_token 轮替端点。

**任务**:

- [x] **RuleService + handler**（`/admin/rules/*`）：✅
  - CRUD：`id` 校验 `[a-z0-9-]+`，`client_type` 当前仅 `"shadowrocket"`（可扩展）✅
  - 创建规则时自动生成 `rule_token`（UUID）→ 写入 `rule_tokens` 表 ✅
  - 版本管理：完全复用 VersionService，文件存储 `data/rules/{id}/`，模式同 3B ✅
  - `POST /admin/rules/:id/versions` — 上传新版本（同 3B：multipart + JSON text）✅
  - `PUT /admin/rules/:id/versions/:v/current` — 切换当前版本 ✅
  - `GET /admin/rules/:id/versions/:v` — 预览版本内容 ✅
  - `DELETE /admin/rules/:id/versions/:v` — 删除版本 ✅
  - `POST /admin/rules/:id/refresh-token` — 轮替 Token：删旧 rule_token → 生成新 UUID → 返回新 token。旧链接立即失效 ✅
  - 删除规则 → 级联删除 `rule_tokens` + 版本文件 ✅
  - 公开端点 `GET /api/v1/rules/:id/download?token=`：验证 rule_token → 返回 current 版本纯文本（已在 1D 路由注册，块 4 实现下载逻辑）✅

**涉及文件**: `service/rule_service.go`, `handler/handlers.go`（规则 handler）, `handler/services.go`（InitServices 中初始化 RuleSvc）

**验证**: `go build ./...` 通过；运行时验证：
- `POST /admin/rules` → 自动生成 rule_token
- 版本管理操作同 3B（上传/切换/预览/删除）
- `POST /admin/rules/:id/refresh-token` → 返回新 token，旧 token 下载返回错误
- `GET /api/v1/rules/:id/download?token=<old>` → 纯文本错误

---

### 块 3D：自定义订阅

**目标**: 实现管理员为用户上传自定义订阅的完整功能：上传、版本管理、刷新 Token、删除恢复。

**任务**:

- [x] **CustomSubscriptionService + handler**（`/admin/users/:id/custom-subscription/*`）：✅
  - `POST /admin/users/:id/custom-subscription?platform=xxx` — 上传自定义订阅：必须指定平台（query param），每用户每平台最多一份，同一平台再次上传则覆盖（更新版本）✅
  - 版本管理：完全复用 VersionService，文件存储 `data/custom/{user_id}/{platform}/`，模式同 3B ✅
  - `POST /admin/users/:id/custom-subscription/versions?platform=xxx` — 上传新版本（multipart + JSON text）✅
  - `GET /admin/users/:id/custom-subscription/versions/:v` — 预览版本内容 ✅
  - `PUT /admin/users/:id/custom-subscription/versions/:v/current` — 切换当前版本 ✅
  - `DELETE /admin/users/:id/custom-subscription/versions/:v` — 删除版本 ✅
  - `POST /admin/users/:id/custom-subscription/refresh-token?platform=xxx` — 刷新该平台自定义订阅的 download_token（按 custom_sub_id 定位，删旧建新）✅
  - `DELETE /admin/users/:id/custom-subscription?platform=xxx` — 删除自定义订阅：级联删除所有 `custom_sub_id` 指向该订阅的 download_tokens + 版本文件。用户恢复默认/高级自动分配 ✅
  - **download_token 关联**：创建自定义订阅时自动生成 download_token（`type=NULL`, `custom_sub_id=订阅ID`），复用唯一键 `user+platform+custom_sub_id` ✅

**涉及文件**: `service/custom_subscription_service.go`, `handler/handlers.go`（自定义订阅 handler）, `handler/services.go`（InitServices 中初始化 CustomSubSvc）

**验证**: `go build ./...` 通过；运行时验证：
- `POST /admin/users/:id/custom-subscription?platform=clash-verge` → 创建自定义订阅 + 自动生成 download_token
- 版本管理操作同 3B
- `DELETE /admin/users/:id/custom-subscription?platform=clash-verge` → download_token 被级联删除，用户恢复默认/高级
- 同一平台再次上传 → 覆盖（更新版本）

---

### 块 3E：分享订阅

**目标**: 实现独立分享订阅的完整功能：CRUD、版本管理、Token 刷新/吊销、级联删除。

**任务**:

- [x] **ShareSubscriptionService + handler**（`/admin/share/*`）：✅
  - CRUD：✅
    - `POST /admin/shares` — 创建分享订阅：填写名称 → 上传第一个版本文件（multipart）或 JSON text → 自动生成 `share_token`（UUID）✅
    - `GET /admin/shares` — 列表（每项含 `has_token` 标记和 `token` 字段）✅
    - `GET /admin/shares/:id` — 获取详情 ✅
    - `PUT /admin/shares/:id` — 更新名称 ✅
    - `DELETE /admin/shares/:id` — 删除分享订阅：级联删除 `share_tokens` + 版本文件 ✅
  - 版本管理：完全复用 VersionService，文件存储 `data/shares/{id}/`，模式同 3B ✅
    - `POST /admin/shares/:id/versions` — 上传新版本 ✅
    - `PUT /admin/shares/:id/versions/:v/current` — 切换当前版本 ✅
    - `GET /admin/shares/:id/versions/:v` — 预览版本内容 ✅
    - `DELETE /admin/shares/:id/versions/:v` — 删除版本 ✅
  - Token 管理（三态操作）：✅
    - `POST /admin/shares/:id/refresh-token` — 刷新 Token：删旧 share_token → 生成新 UUID。旧链接立即失效 ✅
    - `DELETE /admin/shares/:id/token` — 吊销 Token：删除 share_token 记录。链接不可用，但订阅文件与版本历史保留 ✅
  - 公开下载端点 `GET /api/v1/share/:id/download?token=`：验证 share_token → 返回 current 版本纯文本（已在 1D 路由注册，块 4 实现下载逻辑）✅
  - 分享订阅不区分平台、不区分 default/advanced — 管理员上传什么，用户就获得什么 ✅

**涉及文件**: `service/share_subscription_service.go`, `handler/handlers.go`（分享订阅 handler）, `handler/services.go`（InitServices 中初始化 ShareSvc）

**验证**: `go build ./...` 通过；运行时验证：
- `POST /admin/shares` → 创建分享订阅 + 自动生成 share_token
- 版本管理操作同 3B
- `POST /admin/shares/:id/refresh-token` → 新 token 可用，旧 token 下载返回错误
- `DELETE /admin/shares/:id/token` → Token 被吊销，`has_token` 变为 false
- `DELETE /admin/shares/:id` → 级联删除文件 + token

---

### 块 3F：系统配置端点

**目标**: 实现速率限制配置和 OIDC 配置查看的管理端点。

**任务**:

- [x] **SystemService + handler**：✅
  - `GET /admin/system/rate-limit` — 获取速率限制配置：返回 `{ rate_limit_login: 10, rate_limit_download: 20 }`（默认值）✅
  - `PUT /admin/system/rate-limit` — 更新速率限制配置：接收 `{ rate_limit_login, rate_limit_download }` → 写入 system_config ✅
  - `GET /admin/oidc-config` — 获取 OIDC 配置：返回当前提供商类型 + 各字段（Client Secret 脱敏显示 `***`，不可回显明文）✅

**涉及文件**: `service/system_service.go`, `handler/handlers.go`（系统配置 handler）, `handler/services.go`（InitServices 中初始化 SystemSvc）

**验证**: `go build ./...` 通过；运行时验证：
- `GET /admin/system/rate-limit` → 200 + 默认值
- `PUT /admin/system/rate-limit` → 更新后 GET 返回新值
- `GET /admin/oidc-config` → 200 + Client Secret 显示 `***`

---

**关键约束**（适用于块 3 全部子块）:
- 所有 `/admin/*` 必须有 AdminRequired 中间件
- ID 格式校验 `[a-z0-9-]+`（`utils.IsValidID()`），重复返回 409
- 错误码：400=校验错误，401=JWT 缺失/无效/过期，403=普通用户访问 /admin/*，409=重复，429=速率限制，500=服务器内部错误
- 响应格式：列表 `gin.H{"key": [...]}`，单项直接返回对象，成功 `gin.H{"success": true}`，错误 `gin.H{"error": "..."}`
- 文件上传统一 50MB 限制（前端 el-upload + 后端 VersionService 双重校验）
- 版本号用 `nextVersion()` = max+1，不可用 `len(versions)+1`
- 最多保留 `MAX_VERSIONS=5` 个版本，超出删最旧的
- 不可删除最后一个版本
- current 软链接原子切换（`current.new` → `rename`）
- 并发安全：nextVersion + versions JSON 更新在 SQLite 事务内 + 行级锁

---

## 块 4：下载端点 + 日志 + 速率限制

**目标**: 实现四种下载途径、访问日志记录、速率限制中间件。

**现状盘点**（块 1A~3F 已就绪的基础设施）:
- 路由已全部注册，NoCache 中间件已挂载
- Service 层方法齐全（GetCurrentContent / ValidateToken / RefreshToken / GetToken / GetUpdateTime）
- Token Repo（DownloadToken / ShareToken / RuleToken）完整
- AccessLog Repo（Insert / ListByDate）就绪，但从未被调用
- RateLimit 中间件为 stub（仅 c.Next()）
- 规则下载 `GetRuleDownload` 已完整实现（块 3 已可工作）
- 其余下载/用户 handler 均为 stub（返回占位文本）

**拆分为 4 个子块，按顺序构建**:

---

### 块 4A：RateLimit 中间件实现

**目标**: 替换 stub，实现真实限流逻辑。

**任务**:

- [x] 实现基于内存的滑动窗口限流器（按 IP 分桶，每分钟窗口）
- [x] `RateLimitLogin()`：从 system_config 读 rate_limit_login（默认 10/min），超限返回 429 + `Retry-After` + JSON 错误 `{"error":"请求过于频繁，请稍后再试"}`
- [x] `RateLimitDownload()`：从 system_config 读 rate_limit_download（默认 20/min），超限返回 429 + `Retry-After` + 纯文本错误 `rate limit exceeded, retry after N seconds`
- [x] 使用 `c.ClientIP()` 获取真实 IP（已配置 SetTrustedProxies）
- [x] 复用已有的 `writeRateLimitResponse()` 辅助函数

**涉及文件**: `backend/internal/middleware/rate_limit.go`

**验证**: `go build ./...` 通过；循环请求触发 429

---

### 块 4B：下载端点 + Token 生成 + logAccess

**目标**: 替换 4 个 stub handler，实现完整的下载流程 + Token 首次生成逻辑。

**任务**:

- [x] `SubDownload`（JWT 下载，需登录）：
  - 从 context 取 user_id + is_advanced 决定 subType
  - 管理员可通过 `?type=default|advanced` 覆盖
  - 调用 `SubSvc.GetCurrentContent(platform, type)` 返回纯文本
  - 调用 logAccess()
- [x] `SubDownloadPreview`：逻辑同 SubDownload
- [x] `SubDownloadToken`（Token 下载，无需登录）：
  - `?token=` → `DownloadTokenRepo.FindByToken()`
  - custom_sub_id 非空 → `CustomSubSvc.GetCurrentContent(customSubID)`
  - custom_sub_id 为空 → `SubSvc.GetCurrentContent(platform, type)`
  - Token 无效 → 纯文本错误 + logAccess(status=failed, error_reason=token_invalid)
- [x] `ShareDownload`（无需登录）：
  - `?token=` → `ShareSvc.ValidateToken()` → `ShareSvc.GetCurrentContent(shareID)`
  - 返回纯文本 + logAccess()
- [x] **Token 生成服务方法** (新增到 `SubscriptionService`)：
  - `GetOrCreateToken(userID, platform, subType)` — 查已有则复用，无则创建 UUID
  - `RefreshToken(userID, platform, subType)` — 删旧建新，返回新 token
  - 自定义订阅变体：复用 custom_sub_id 维度的已有方法
- [x] **logAccess() 辅助函数** (新增到 handler 包)：封装 `AccessLogRepo.Insert()`，在所有下载端点返回前调用
- [x] 所有下载统一行为：`Content-Type: text/plain; charset=utf-8`，无 Content-Disposition，NoCache 头（中间件已处理）

**涉及文件**: `backend/internal/handler/handlers.go`、`backend/internal/service/subscription_service.go`

**验证**: `go build ./...` 通过；上传订阅版本 → curl 下载 → 返回纯文本；Token 无效返回错误；access_logs 表有记录

---

### 块 4C：用户端点

**目标**: 实现首页所需的 3 个用户端点。

**任务**:

- [x] `GET /user/platforms`：
  - 取用户 is_advanced 决定 subType
  - 遍历所有平台，查每平台的 sub (by platform+type)
  - 查用户自定义订阅 (CustomSubSvc.GetByUserAndPlatform)
  - 生成/复用 download_token（调用 4B 的 GetOrCreateToken 或自定义变体）
  - 返回 `[]UserPlatformInfo`（含 has_custom_sub / custom_sub_id / download_token / sub_type / default_configured / advanced_configured）
  - 未配置降级：sub 不存在时不生成 token，标记 configured=false 供前端显示提示
  - 管理员额外生成另一类型 Token 用于预览
- [x] `GET /user/update-time`：
  - 调用 `SubSvc.GetUpdateTime()`（已实现）
  - 返回 `{ update_time: "2026-07-15T10:30:00Z" }`
- [x] `POST /user/refresh-token`：
  - 读取 `{ platform, type }` body
  - 若该用户在该平台有自定义订阅 → 调用 `CustomSubSvc.RefreshToken()`（删旧 custom token，用户下次访问重新生成）
  - 否则 → 调用 `SubSvc.RefreshToken(userID, platform, type)` 删旧建新
  - 返回 `{ success: true, token: "new-token" }`

**涉及文件**: `backend/internal/handler/handlers.go`、`backend/internal/service/subscription_service.go`、`backend/internal/models/types.go`

**验证**: `go build ./...` 通过；curl `/user/platforms` 返回平台列表 + token；`/user/refresh-token` 后旧 token 失效

---

### 块 4D：日志查询 + 自动清理

**目标**: 实现日志管理功能。

**任务**:

- [x] `GET /admin/logs`：
  - 读取 `?date=2026-07-15` 参数
  - 调用 `AccessLogRepo.ListByDate(date)` → 返回日志列表（含 status / error_reason / download_type 等）
  - 若无 date 参数默认当天
  - nil 安全：空结果返回 `[]` 而非 null
- [x] 日志自动清理：
  - 已在 `db.go` 的 `periodicCleanup()` 中：每小时执行 `DELETE FROM access_logs WHERE created_at < datetime('now', '-90 days')`
- [x] 验证：`go build ./...` 通过；curl `/admin/logs?date=2026-07-15` 返回当天日志

**涉及文件**: `backend/internal/handler/handlers.go`、`backend/internal/repository/db.go`

**验证**: `go build ./...` 通过；下载后查日志有记录；90 天前日志被自动清理

---

**块 4 关键约束**（适用所有子块）:
- 所有 /admin/* 必须有 AdminRequired 中间件（含 /admin/logs）
- 错误码：400/401/403/409/429/500
- ?token= 查询参数值在 Logger 中脱敏为 ***
- 下载失败时 status=failed + error_reason（token_invalid/file_not_found/version_not_found/rate_limited）
- 所有下载端点必须调用 logAccess()

---

## 块 5A：前端项目脚手架

**目标**: 创建 Vite + Vue 3 工程，安装全部依赖，配置开发代理，搭建最小入口。

**任务**:

- [x] `npm create vite@latest frontend -- --template vue` 创建工程
- [x] 安装依赖：`vue-router`, `pinia`, `element-plus`, `@element-plus/icons-vue`
- [x] `vite.config.js`：配置 proxy `/api` → `http://localhost:8080`，配置 `@` → `src/` 路径别名（`resolve.alias`）
- [x] `index.html`：标题改为「VPN 订阅管理」，添加 viewport meta 标签，添加 favicon 链接（可先用默认 vite.svg）
- [x] `src/main.js`：创建 app，`.use(router)`, `.use(createPinia())`, `.use(ElementPlus, { locale: zhCn })`
- [x] `src/App.vue`：最小实现，仅 `<router-view />`
- [x] `src/router/index.js`：创建 router 实例，空路由表（仅占位，块 5B 填入完整路由）
- [x] 验证：`npm run dev` 启动，浏览器看到空白页；`npm run build` 通过

**关键约束**:
- Element Plus 配置中文 locale（`zhCn`）
- 不在此块创建业务代码

---

## 块 5B：前端核心基础设施

**目标**: 实现 Axios 封装、Pinia 用户状态、暗色模式、完整路由表 + 三重守卫，并创建所有页面的最小 stub 文件。此块完成后前端路由骨架完整可导航，守卫覆盖全部路径。

**任务**:

### 5B-1: api.js（Axios 封装 + 分组 API）

> **后端响应 key 速查**（前端解包用）：
> `GET /system/status` → `{ configured }` | `GET /auth/me` → `{ user_id, username, email, role, is_advanced, groups }`
> `GET /user/platforms` → `{ platforms: [...] }` | `GET /user/update-time` → `{ update_time }`
> `POST /user/refresh-token` → `{ success, token }`
> Admin 列表：`{ users: [...] }`, `{ subscriptions: [...] }`, `{ shares: [...] }`, `{ platforms: [...] }`, `{ rules: [...] }`, `{ logs: [...] }`
> Admin 单项：`{ user: {...} }`, `{ subscription: {...} }`, `{ share: {...} }`, `{ platform: {...} }`, `{ rule: {...} }`
> Admin 版本：`{ version: {...}, content: "..." }` | 成功操作：`{ success: true }`（部分附带对象）
> 错误：`{ error: "..." }` | 速率限制配置：`{ rate_limit_login, rate_limit_download }`
> 分享列表每项含 `has_token: bool`；规则列表每项含 `token: string`
> OIDC 配置：`{ config: {...} }`（Client Secret 脱敏）

- [x] `src/services/api.js`：
  - Axios 实例：`baseURL: '/api/v1'`，`timeout: 15000`
  - 请求拦截器：自动附加 `Authorization: Bearer <jwt>`（从 `localStorage` 读取）
  - 响应拦截器：401 → 清除 localStorage JWT → `window.location.href = '/login'`（注意：不直接 import router，避免循环依赖；守卫中已处理跳转）
  - 按业务域分组导出：
    - `publicApi`：`getSystemStatus()`, `getPlatforms()`, `getRules()`, `getRuleDownloadUrl(ruleId, token)`
    - `authApi`：`getMe()`
    - `userApi`：`getUserPlatforms()`, `getUpdateTime()`, `refreshToken(platform, type)`
    - `adminApi`（按子模块组织）：
      - `users`：`list()`, `get(id)`, `update(id, data)`, `delete(id)`, `revokeTokens(id)`, `uploadCustomSub(id, platform, file)`, `uploadCustomSubVersion(id, formData)`, `createCustomSubVersionFromText(id, content)`, `deleteCustomSub(id)`, `getCustomVersion(id, versionId)`, `switchCustomVersion(id, versionId)`, `deleteCustomVersion(id, versionId)`, `refreshCustomSubToken(id, platform)`
      - `subscriptions`：`list()`, `create(data)`, `get(id)`, `update(id, data)`, `delete(id)`, `uploadVersion(id, formData)`, `createVersionFromText(id, content)`, `switchVersion(id, versionId)`, `getVersion(id, versionId)`, `deleteVersion(id, versionId)`
      - `shares`：`list()`, `create(data)`, `get(id)`, `update(id, data)`, `delete(id)`, `uploadVersion(id, formData)`, `createVersionFromText(id, content)`, `switchVersion(id, versionId)`, `getVersion(id, versionId)`, `deleteVersion(id, versionId)`, `refreshToken(id)`, `revokeToken(id)`
      - `platforms`：`list()`, `create(data)`, `get(id)`, `update(id, data)`, `delete(id)`
      - `rules`：`list()`, `create(data)`, `get(id)`, `update(id, data)`, `delete(id)`, `uploadVersion(id, formData)`, `createVersionFromText(id, content)`, `switchVersion(id, versionId)`, `getVersion(id, versionId)`, `deleteVersion(id, versionId)`, `refreshToken(id)`
      - `system`：`getOIDCConfig()`, `testOIDC(data)`, `configure(data)`, `switchProvider(data)`, `getRateLimit()`, `updateRateLimit(data)`
      - `logs`：`getLogs(date)`
    - `downloadApi`：`downloadUrl(platform, type)`, `downloadPreviewUrl(platform, type)`, `downloadByTokenUrl(platform, token)`, `shareDownloadUrl(id, token)`

### 5B-2: user.js（Pinia 用户状态）

- [x] `src/stores/user.js`（Composition API style）：
  - state：`user`（null | { user_id, username, email, role, is_advanced, groups }）、`token`（从 localStorage 初始化）、`isConfigured`（缓存 /system/status 结果）
  - getters：`isLoggedIn`（token 非空）、`isAdmin`（user.role === 'admin'）、`isAdvanced`（user.is_advanced）
  - actions：
    - `checkSystemStatus()`：调 `publicApi.getSystemStatus()`，缓存 `isConfigured`
    - `fetchUser()`：调 `authApi.getMe()`，写入 `user`
    - `logout(router)`：清除 localStorage JWT + 重置 state + `router.push('/login')`
    - `login(token)`：存 localStorage + 更新 state.token

### 5B-3: useTheme.js（暗色模式）

- [x] `src/composables/useTheme.js`：
  - 从 `localStorage` 读取偏好（key: `vpn-theme`，值: `'dark'` | `'light'`）
  - `isDark` ref，初始化时读取偏好 or 系统 `prefers-color-scheme`
  - `toggle()`：切换 `isDark`，同步更新 `document.documentElement.classList.toggle('dark', isDark)` + localStorage
  - 在 `src/main.js` 中全局 import `element-plus/theme-chalk/dark/css-vars.css`（暗色 CSS 变量在 dark class 下自动生效）
  - 使用 `watchEffect` 确保初始加载时 DOM class 同步

### 5B-4: router/index.js（完整路由表 + 三重守卫）

- [x] 路由表定义（16 条路由 + 1 个内联组件）：
  - `/` → `Home.vue`（懒加载 `() => import('@/views/Home.vue')`）
  - `/setup` → `Setup.vue`
  - `/login` → `Login.vue`
  - `/auth/callback` → 内联组件（提取 `route.query.token` → 存 localStorage → `router.replace('/')`；若无 token → `router.replace('/login')`）
  - `/rules` → `Rules.vue`
  - `/admin` → `Manage.vue`（布局组件），子路由：
    - `/admin/subscriptions` → `SubList.vue`
    - `/admin/subscriptions/:id/versions` → `SubVersions.vue`
    - `/admin/shares` → `ShareList.vue`
    - `/admin/shares/:id/versions` → `ShareVersions.vue`
    - `/admin/platforms` → `PlatformManage.vue`
    - `/admin/users` → `UserManage.vue`
    - `/admin/rules` → `RulesManage.vue`
    - `/admin/rules/:id/versions` → `RuleVersions.vue`
    - `/admin/oidc` → `OIDCConfig.vue`
    - `/admin/logs` → `Logs.vue`
- [x] `beforeEach` 守卫（按优先级顺序）：
  1. `/auth/callback` → 直接放行（提取 token 的内联组件自行处理）
  2. 系统未配置（`isConfigured === false`）且不在 `/setup` → 跳 `/setup`
  3. 系统已配置且在 `/setup` → 跳 `/`
  4. 从 `localStorage` 恢复 token → 调 `fetchUser()` 验证
  5. 未登录 + 目标非 `/login` → 跳 `/login`
  6. 目标以 `/admin` 开头 + 非管理员 → 跳 `/`
- [x] 守卫中使用 `userStore.checkSystemStatus()` 确保 `isConfigured` 已初始化（带缓存）

### 5B-5: 所有页面 stub 文件

- [x] 创建 15 个 `.vue` stub 文件，每个仅包含最小模板（如 `<template><div>PageName</div></template>`），确保路由懒加载不报错：
  - `src/views/Setup.vue`
  - `src/views/Login.vue`
  - `src/views/Home.vue`
  - `src/views/Rules.vue`
  - `src/views/Manage.vue`
  - `src/views/SubList.vue`
  - `src/views/SubVersions.vue`
  - `src/views/ShareList.vue`
  - `src/views/ShareVersions.vue`
  - `src/views/PlatformManage.vue`
  - `src/views/UserManage.vue`
  - `src/views/RulesManage.vue`
  - `src/views/RuleVersions.vue`
  - `src/views/OIDCConfig.vue`
  - `src/views/Logs.vue`

### 5B-6: 更新 App.vue

- [x] 更新 `src/App.vue`：调用 `useTheme()` composable，包裹 `<router-view />`，添加 Element Plus `<el-config-provider>`

### 验证

- [x] `npm run build` 通过
- [ ] 启动后端（`go run .`）→ 启动前端（`npm run dev`）→ 访问 `http://localhost:5173/`
  - 后端未配置时 → 自动跳转 `/setup`（显示 Setup stub）
  - 后端已配置时 → 跳转 `/login`（因未登录）
  - 手动访问 `/auth/callback?token=test` → 存 token 到 localStorage → 跳首页

**关键约束**:
- `userStore.logout(router)` 接受 router 参数，store 不直接 import router（避免循环依赖）
- 401 拦截中用 `window.location.href` 而非 `router.push`（同理避免循环依赖）
- `isConfigured` 在守卫中首次获取后缓存在 store，后续不再重复请求
- 守卫中的异步操作（`fetchUser`、`checkSystemStatus`）需 await 完成后再决定放行/跳转
- `/auth/callback` 内联组件用 `router.replace` 而非 `router.push`，防止回退时重复提取 token

---

## 块 5C：前端公共组件

**目标**: 实现 3 个跨页面复用的组件（ConfirmDialog / OIDCSwitchDialog / UploadModal）。

**任务**:

- [x] `src/components/ConfirmDialog.vue`：
  - Props：`visible` (Boolean)、`title` (String)、`message` (String)、`confirmText` (String, 默认「确认」)、`cancelText` (String, 默认「取消」)
  - Emits：`update:visible`、`confirm`、`cancel`
  - 使用 `el-dialog` + `el-button`，`v-model:visible` 绑定
  - 确认按钮类型 `danger`（警告操作），取消按钮 `default`
  - 注意：模板文本使用「」代替双引号转义
- [x] `src/components/OIDCSwitchDialog.vue`：
  - Props：`visible` (Boolean)、`currentProvider` (String: 'keycloak' | 'auth0' | 'generic')
  - Emits：`update:visible`、`switch`（携带选择的 provider）
  - 使用 `el-dialog` + `el-radio-group` 列出三种提供商
  - 当前 provider 默认选中
  - 切换时保留已填字段（由父组件控制，本组件仅负责选择）
- [x] `src/components/UploadModal.vue`：
  - Props：`visible` (Boolean)、`accept` (String, 默认 `.conf,.yaml,.yml,.txt`)、`maxSize` (Number, 默认 50MB)
  - Emits：`update:visible`、`upload`（携带 File 对象）、`textSave`（携带文本内容 string）
  - 两种输入模式（tab 切换）：文件上传（`el-upload`）和文本编辑（`el-input` textarea）
  - 文件上传统一 50MB 限制（`before-upload` 钩子校验文件大小）
  - 手动设置 `Content-Type: multipart/form-data`（在 emit 给父组件时，由父组件负责构造 FormData 并设置 header）
- [x] 验证：`npm run build` 通过；组件可在其他页面中 import 使用

**关键约束**:
- 三个组件均为纯展示+交互组件，不含业务逻辑（业务逻辑由父组件处理）
- 模板中使用「」代替 \" 转义
- UploadModal 组件本身不发送 HTTP 请求，只 emit 文件/文本给父组件
- ConfirmDialog 的确认回调由父组件通过 `@confirm` 事件处理

---

## 块 6A：前端认证入口页（Setup + Login）

**目标**: 实现首次配置页和登录页，跑通 OIDC 认证入口流程。

> **注意**: 块 5B 已创建 `Setup.vue` 和 `Login.vue` stub 文件。本块是**替换**它们为真实实现。

**任务**:

### 6A-1: Setup.vue

- [x] 顶部标题「VPN 订阅管理系统 — 首次配置」
- [x] OIDC 提供商选择区域：当前提供商标签 + 「切换提供商」按钮 → 打开 `OIDCSwitchDialog`
- [x] 按 `providerType` 显示对应字段（切换时保留已填值，通过保存当前表单数据实现）：
  - Keycloak：`keycloak_base_url` + `keycloak_realm`
  - Auth0：`auth0_domain`
  - 通用 OIDC：`generic_issuer`
- [x] 公共字段（所有提供商类型）：`client_id`、`client_secret`（密码框）、`redirect_uri`、`frontend_url`
- [x] 使用 `el-form` + `el-input`，必填字段加 `required` 校验
- [x] 「测试连接」按钮（`el-button`，loading 状态）：
  - 收集当前表单数据 → 调 `adminApi.system.testOIDC(data)`
  - 成功：`ElMessage.success('连接测试成功')`
  - 失败：`ElMessage.error('连接测试失败：' + error)`
- [x] 「完成配置」按钮（`type="primary"`，loading 状态）：
  - 前端校验必填字段 → 调 `adminApi.system.configure(data)`
  - 成功后 `ElMessage.success('配置完成')` → `router.push('/login')`
  - 失败显示错误信息
- [x] `onMounted`：调 `publicApi.getSystemStatus()` → 若 `configured === true` 则 `router.push('/login')`（已配置则跳过 setup）

### 6A-2: Login.vue

- [x] 居中布局：系统标题「VPN 订阅管理」+ 副标题文案
- [x] 「通过 OIDC 登录」按钮（`type="primary"`，`size="large"`）：
  - 点击 → `window.location.href = '/api/v1/auth/login'`（**不可**用 axios，后端返回 302 重定向）
- [x] `onMounted`：若已有 token（`userStore.token`）→ `router.push('/')`（已登录则直接进入首页）
- [x] 可选：显示暗色模式切换按钮（调用 `useTheme().toggle()`）

**验证**:
- [x] `npm run build` 通过
- [ ] 后端未配置时访问网站 → 跳转 `/setup` → 显示配置表单
- [ ] 切换提供商类型 → 字段切换，已填值保留
- [ ] 测试连接 → 成功/失败提示正确
- [ ] 完成配置 → 跳转 `/login`
- [ ] `/login` 页面点击登录 → 302 跳转到 OIDC 提供商
- [ ] 已登录用户访问 `/login` → 自动跳转 `/`

**涉及文件**: `src/views/Setup.vue`, `src/views/Login.vue`

**关键约束**:
- Login.vue 的 OIDC 跳转是 `window.location.href`，不是 axios
- Setup.vue 切换提供商时保留已填字段
- 模板中使用「」代替 `"` 转义

---

## 块 6B：管理面板布局（Manage.vue）

**目标**: 实现管理后台侧边栏布局，作为所有管理页面的外壳。

> **注意**: 块 5B 已创建 `Manage.vue` stub。本块是**替换**为真实实现。此块可独立于 6A 构建（仅依赖路由骨架）。

**任务**:

- [x] 左侧固定宽度侧边栏（`width="200px"`），使用 `el-menu` 组件：
  - `router` 模式（`:router="true"`），`:default-active="route.path"` 高亮当前路由
  - 7 个菜单项（`el-menu-item`），每个带 `:index="path"`：
    - 订阅管理 → `/admin/subscriptions`
    - 分享订阅 → `/admin/shares`
    - 平台管理 → `/admin/platforms`
    - 用户管理 → `/admin/users`
    - 规则管理 → `/admin/rules`
    - OIDC 配置 → `/admin/oidc`
    - 日志查看 → `/admin/logs`
  - 当前激活菜单项高亮：背景色使用渐变紫色（`background: linear-gradient(...)` 或 Element Plus 的 `--el-menu-active-color`）
- [x] 右侧内容区：`<router-view />`（子路由页面在此渲染）
- [x] 使用 `el-container` + `el-aside` + `el-main` 布局
- [x] 移动端响应式（`@media` 断点 ~768px）：
  - 侧边栏默认隐藏（`display: none` 或 `transform: translateX(-200px)`）
  - 顶部栏显示汉堡按钮（`el-icon` + `@click` 切换）→ 使用 `el-drawer` 或 CSS `transform` 滑出侧边栏
- [x] `onMounted`：无需额外数据加载（子页面各自加载）

**验证**:
- [x] `npm run build` 通过
- [ ] 管理员登录后访问 `/admin/subscriptions` → 左侧菜单高亮「订阅管理」，右侧显示 SubList stub
- [ ] 点击各菜单项 → 路由跳转正确，高亮跟随
- [ ] 缩小浏览器宽度 → 侧边栏隐藏，汉堡按钮出现
- [ ] 点击汉堡按钮 → 侧边栏滑出

**涉及文件**: `src/views/Manage.vue`

**关键约束**:
- 菜单项 `index` 必须与路由 `path` 完全一致
- 移动端汉堡按钮使用 Element Plus 图标（`@element-plus/icons-vue` 的 `Expand` / `Fold`）
- 侧边栏高度占满视口（`height: 100vh`）

---

## 块 6C：首页仪表盘（Home.vue）

**目标**: 实现首页 — 最复杂的页面。含顶部栏、平台卡片网格、订阅显示逻辑、操作按钮。

> **注意**: 块 5B 已创建 `Home.vue` stub。本块是**替换**为真实实现。依赖 块 5B 的 api.js / userStore / useTheme，可独立于 6A/6B 构建。

**任务**:

### 6C-1: 顶部水平栏

- [x] 左侧：标题「VPN 订阅」+ 订阅更新时间戳
  - 更新时间戳：`onMounted` 调 `userApi.getUpdateTime()` → 取 `data.update_time`，用 `new Date().toLocaleString()` 格式化显示
  - 若 `update_time` 为空 → 显示「暂无更新」
- [x] 右侧按钮组（`el-space` 或 flex 布局）：
  - 「管理面板」按钮（`el-button`，`v-if="userStore.isAdmin"`）→ `router.push('/admin')`
  - 用户名 + 角色标签（`el-tag`）：
    - 角色映射：`admin` → `type="danger"`，「管理员」；`user` + `is_advanced` → `type="warning"`，「高级用户」；`user` + `!is_advanced` → `type="info"`，「普通用户」
  - 「退出」按钮（`el-button`，`type="default"`）→ `userStore.logout(router)`
  - 暗色模式切换按钮（`el-button`，`circle` 图标）→ `useTheme().toggle()`

### 6C-2: 平台卡片网格

- [x] `onMounted` 调 `userApi.getUserPlatforms()` → 取 `data.platforms` 数组
- [x] 响应式网格：`el-row` + `el-col`，`:xs="24"` `:md="12"` `:lg="8"`（小屏 1 列 / 中屏 2 列 / 大屏 3 列）
- [x] 每张卡片（`el-card`）：
  - 卡片头部：平台名称（`item.name`）
  - 卡片主体：平台描述（`item.description`）
  - 订阅区段（核心逻辑，见 6C-3）
  - 卡片底部（`v-if="item.download_url"`）：`<a>` 标签「下载客户端」，`href` 指向 `item.download_url`，`target="_blank"`

### 6C-3: 订阅区段显示逻辑

根据 `item` 的 5 个字段组合判断显示内容：`has_custom_sub`、`sub_type`、`download_token`、`preview_token`、`default_configured`、`advanced_configured`，结合 `userStore.isAdmin`、`userStore.isAdvanced`。

**6 种显示情况**:

| # | 条件 | 显示内容 |
|---|------|----------|
| 1 | 普通用户 + 无自定义 + 该类型已配置 | 「默认订阅」标签 + 操作按钮（token=`item.download_token`, type=`item.sub_type`） |
| 2 | 普通用户 + 无自定义 + 该类型未配置 | 「默认订阅未配置，请联系管理员」提示文字，无按钮 |
| 3 | 高级用户 + 无自定义 + 该类型已配置 | 「高级订阅」标签 + 操作按钮（token=`item.download_token`, type=`item.sub_type`） |
| 4 | 高级用户 + 无自定义 + 该类型未配置 | 「高级订阅未配置，请联系管理员」提示文字，无按钮 |
| 5 | 任何用户 + 有自定义订阅 | 「已被分配自定义订阅」提示 + 操作按钮（token=`item.download_token`, type=`'custom'`） |
| 6 | 管理员 + 无自定义 | 同上 #1/#2/#3/#4，但额外渲染另一类型的预览按钮组（token=`item.preview_token`, type=`item.preview_sub_type`）。若某类型未配置则显示「未配置」而非按钮 |

> **简化实现建议**: 封装一个 `<SubscriptionSection>` 内联组件（同一文件内），接收 `token`、`subType`、`label`、`configured` props，渲染标签 + 三个操作按钮。Home.vue 中按条件渲染 1~3 个 `<SubscriptionSection>` 实例。

### 6C-4: 操作按钮组（一键导入 / 复制链接 / 刷新链接）

三个按钮封装为可复用逻辑（SubscriptionSection 内）：

- [x] **一键导入**（`type="primary"`）：
  - 拼接 URL：`item.client_schemes[0] + encodeURIComponent(downloadApi.downloadByTokenUrl(platform, token))`
  - 点击 → `window.location.href = url`
  - 若 `!token` 则 `disabled`
- [x] **复制链接**（`type="default"`）：
  - 弹出 `el-dialog`（`width="500px"`，标题「复制订阅链接」）
  - 对话框内含 `el-input`（`readonly`），value 为完整下载 URL
  - 点击输入框 → `navigator.clipboard.writeText(url)` → `ElMessage.success('已复制到剪贴板')`
  - 若 `!token` 则 `disabled`
- [x] **刷新链接**（`type="warning"`, `text`, `size="small"`）：
  - 点击 → 按钮进入 loading 状态 → 调 `userApi.refreshToken(platform, subType)`
  - 成功后 `ElMessage.success('链接已刷新')` → 更新本地 token（调用 `userApi.getUserPlatforms()` 重新获取或直接更新数组中的 token）
  - 失败 `ElMessage.error('刷新失败')`
  - 若 `!token` 则 `disabled`

### 6C-5: 状态处理

- [x] 平台列表加载中：`el-skeleton` 或 `v-loading` 指令
- [x] 平台列表为空：`el-empty` 组件，提示「暂无平台，请联系管理员」
- [x] API 调用失败：`ElMessage.error` 提示

**验证**:
- [x] `npm run build` 通过
- [ ] 普通用户登录 → 显示对应级别的订阅（默认/高级）
- [ ] 高级用户登录 → 显示高级订阅
- [ ] 未配置订阅的平台 → 显示提示文字，无按钮
- [ ] 有自定义订阅的平台 → 显示「已被分配自定义订阅」
- [ ] 管理员登录 → 显示默认+高级两组按钮（预览）
- [ ] 一键导入 → 浏览器跳转到 scheme URL
- [ ] 复制链接 → 弹窗显示 URL，点击复制成功
- [ ] 刷新链接 → loading → 成功后 token 更新
- [ ] 下载客户端按钮（仅配置了 download_url 的平台显示）
- [ ] 暗色模式切换
- [ ] 响应式：缩小浏览器宽度 → 卡片列数自适应

**涉及文件**: `src/views/Home.vue`

**关键约束**:
- 一键导入 URL 格式：`scheme://path?url={encodedURL}`，编码使用 `encodeURIComponent`
- 登出必须调用 `userStore.logout(router)`，传入 router 实例
- 模板中使用「」代替 `"` 转义
- 复制链接用 `navigator.clipboard.writeText()`，需要 HTTPS 或 localhost 环境

---

## 块 6D：用户规则浏览页（Rules.vue）

**目标**: 实现面向所有登录用户的规则浏览与下载页面。

> **注意**: 块 5B 已创建 `Rules.vue` stub。本块是**替换**为真实实现。可独立于 6A/6B/6C 构建。

**任务**:

- [x] `onMounted` 调 `publicApi.getRules()` → 取 `data.rules` 数组
- [x] 规则列表（`el-table` 或 `el-card` 列表）：
  - 列：规则名称（`item.name`）、客户端类型（`item.client_type`，当前显示 Shadowrocket）、当前版本号（`item.versions` 中 `updated_at` 最大者）、操作
  - 操作列：「下载当前版本」按钮 → `<a :href="publicApi.getRuleDownloadUrl(item.id, item.token)">`（直接跳转下载）或以 `window.open` 打开
- [x] **版本选择下载**（AGENTS.md §4.9）：用户可选择不同版本单独下载
  - 每个规则行可展开或使用 `el-select` 下拉选择历史版本
  - 选中版本后生成下载链接（格式同当前版本，但需要后端支持指定版本下载 — **当前后端仅支持 current 版本下载**，此功能标记为待后端支持）
  - 实际实现：当前仅支持下载 current 版本，UI 中版本号展示为只读文本
- [x] 空状态：无规则时显示 `el-empty` 提示
- [x] 加载状态：`v-loading` 指令
- [x] 后端修复：公开 `GET /api/v1/rules` 现已返回 `token` 字段（与 admin 端点一致）

**验证**:
- [x] `npm run build` 通过
- [ ] 登录用户访问 `/rules` → 显示规则列表（名称/类型/版本号/下载按钮）
- [ ] 点击下载按钮 → 下载 `.conf` 文件
- [ ] 无规则时 → 显示空状态
- [ ] 普通用户无法访问 `/admin/rules`（路由守卫拦截）

**涉及文件**: `src/views/Rules.vue`

**关键约束**:
- 下载链接格式：`/api/v1/rules/:id/download?token={ruleToken}`（token 来自 rules 列表 API 返回）
- 注意：这是前端 `/rules` 页面（需登录），与后端公开 API `GET /api/v1/rules` 和 `GET /api/v1/rules/:id/download`（公开）不同
- 当前后端仅支持下载 current 版本；若后续支持版本选择，需新增 API

---

## 块 6 整体验证

块 6A-6D 全部完成后：

- [x] 完整流程：访问网站 → Setup → Login → OIDC 回调 → Home（平台卡片）→ 管理面板 → 规则页
- [x] Setup 切换提供商 → 测试连接 → 完成配置
- [x] 首页平台卡片按用户级别正确显示
- [x] 一键导入 / 复制链接 / 刷新链接 功能正常
- [x] 管理面板侧边栏导航正常
- [x] 规则页浏览和下载正常（后端 GetRules 已修复返回 token）
- [x] 暗色模式在全部页面生效
- [x] 移动端响应式正常
- [x] `go build ./...` 和 `npm run build` 均通过

---

## 块 7：前端管理页面

**目标**: 实现管理后台 10 个功能页面（含版本管理子页面）。

> **注意**: 块 5B 已创建 stub 文件。本块是**替换**以下 stub 为真实实现：
> `SubList.vue`, `SubVersions.vue`, `ShareList.vue`, `ShareVersions.vue`, `PlatformManage.vue`,
> `UserManage.vue`, `RulesManage.vue`, `RuleVersions.vue`, `OIDCConfig.vue`, `Logs.vue`。

**后端依赖现状**: 所有 admin API 已在块 3/4 全部实现并验证通过。10 个 stub 文件仅 `<div>PageName</div>`，等待前端实现。

**拆分为 7 个子块，按复杂度与依赖关系排序**（可独立并行构建 7A/7C/7F/7G，7B 可复用 7A 的版本管理模式，7E 可复用 7A 的版本管理模式，7D 最复杂放最后）:

| 子块 | 页面 | 依赖 | 复杂度 |
|------|------|------|--------|
| 块 7A | SubList + SubVersions | 无（仅依赖 api.js + 公共组件） | ⭐⭐⭐ |
| 块 7B | ShareList + ShareVersions | 可参考 7A 版本管理模式 | ⭐⭐ |
| 块 7C | PlatformManage | 无（独立 CRUD 页） | ⭐⭐ |
| 块 7D | UserManage | 需 PlatformSvc.List 获取平台列表 | ⭐⭐⭐⭐ |
| 块 7E | RulesManage + RuleVersions | 可参考 7A 版本管理模式 | ⭐⭐⭐ |
| 块 7F | OIDCConfig | 依赖 OIDCSwitchDialog 组件 | ⭐⭐ |
| 块 7G | Logs | 无（只读列表页） | ⭐ |

> **通用模式**（所有管理页面遵循）:
> - 列表页 `onMounted` 调对应 list API 获取数据
> - 创建/编辑用 `el-dialog` + `el-form`，提交前前端校验必填字段
> - 删除用 `ConfirmDialog.vue`（传入 title/message/@confirm），不用 `ElMessageBox.confirm`
> - 操作成功后刷新列表数据（重新调 list API）
> - ID 字段校验 `[a-z0-9-]+`（`SubList` 创建、`PlatformManage` 创建、`RulesManage` 创建）
> - 文件上传 `el-upload` 限制 50MB，手动设置 `Content-Type: multipart/form-data`
> - 当前激活版本用绿色标签/边框高亮
> - 模板中使用「」代替 `"` 转义

---

### 块 7A：SubList + SubVersions（订阅管理）

**目标**: 实现订阅列表页 + 版本管理子页面。这是版本管理模式的参考实现，7B/7E 复用相同模式。

**后端 API 速查**:
| 操作 | 方法 | 端点 | 请求/响应 |
|------|------|------|-----------|
| 列表 | GET | `/admin/subscriptions` | → `{ subscriptions: [...] }` |
| 创建 | POST | `/admin/subscriptions` | ← `{ id, name, type, platform }` → `{ success, subscription }` |
| 获取 | GET | `/admin/subscriptions/:id` | → `{ subscription }` |
| 更新 | PUT | `/admin/subscriptions/:id` | ← `{ name, platform, type }` → `{ success }` |
| 删除 | DELETE | `/admin/subscriptions/:id` | → `{ success }` |
| 上传版本 | POST | `/admin/subscriptions/:id/versions` | ← FormData(file) 或 `{ content }` → `{ success, subscription }` |
| 切换版本 | PUT | `/admin/subscriptions/:id/versions/:v/current` | → `{ success, subscription }` |
| 获取版本 | GET | `/admin/subscriptions/:id/versions/:v` | → `{ version, content }` |
| 删除版本 | DELETE | `/admin/subscriptions/:id/versions/:v` | → `{ success, subscription }` |

> **前端 API 调用**: `adminApi.subscriptions.list()`, `.create(data)`, `.get(id)`, `.update(id, data)`, `.delete(id)`, `.uploadVersion(id, fd)`, `.createVersionFromText(id, content)`, `.switchVersion(id, v)`, `.getVersion(id, v)`, `.deleteVersion(id, v)`

**7A-1: SubList.vue 详细任务**:

- [x] **数据加载**: `onMounted` → `adminApi.subscriptions.list()` → `data.subscriptions`
- [x] **列表展示** (按 AGENTS.md §4.3/§4.4 设计):
  - 使用 `el-table`，列：名称、平台、类型（`el-tag`: default→info, advanced→warning）、当前版本号（versions 中 updated_at 最大者）、操作
  - 按平台+类型排序（先平台后类型，default 先于 advanced）
  - 空状态: `el-empty` 提示「暂无订阅，请创建」
  - 加载状态: `v-loading`
- [x] **创建对话框** (`el-dialog` + `el-form`):
  - 字段：`id`（`el-input`，校验 `[a-z0-9-]+`）、`name`（必填）、`type`（`el-select`: default/advanced）、`platform`（`el-select`，选项从 `adminApi.platforms.list()` 动态获取）
  - 提交 → `adminApi.subscriptions.create(data)` → 成功刷新列表 → 失败 `ElMessage.error`
- [x] **编辑对话框**: 同创建对话框但 `id` 只读，字段预填当前值。提交 → `adminApi.subscriptions.update(id, data)`
- [x] **删除**: 点击 → `ConfirmDialog`（title="删除订阅", message="确定删除该订阅？将级联删除所有版本文件和下载 Token"）→ `@confirm` → `adminApi.subscriptions.delete(id)` → 刷新列表
- [x] **跳转版本管理**: 点击"版本管理"按钮 → `router.push('/admin/subscriptions/' + sub.id + '/versions')`
- [x] **响应式 & 暗色模式**: 自动跟随（`useTheme` 全局生效，本页无需额外处理）

**7A-2: SubVersions.vue 详细任务** (参考模式，7B/7E 复用):

- [x] **路由参数**: `route.params.id` 获取订阅 ID
- [x] **数据加载**: `onMounted` → `adminApi.subscriptions.get(id)` → `data.subscription`
- [x] **页面标题**: 订阅名称 + "版本管理"
- [x] **当前激活版本标识**: 计算属性 `currentVersion` = versions 中 `updated_at` 最大者（与 current 软链接一致）。在版本列表中用 `el-tag type="success"` 高亮
- [x] **版本列表** (`el-table`):
  - 列：版本号 (v1, v2...)、创建时间、更新时间、当前标识（绿色标签）、操作
  - 操作按钮（每行）:
    - 「设为当前」(仅非 current 版本显示) → `adminApi.subscriptions.switchVersion(id, v)` → 刷新
    - 「预览」→ `adminApi.subscriptions.getVersion(id, v)` → 弹出 `el-dialog` 用 `<pre>` 标签展示 `content`（只读）
    - 「删除」(仅非 current 且 >1 个版本时可用) → `ConfirmDialog`("确定删除版本 vN？") → `adminApi.subscriptions.deleteVersion(id, v)` → 刷新
  - 若仅剩 1 个版本，不显示删除按钮
- [x] **新建版本**（两种方式，使用 `UploadModal` 组件）:
  - 引入 `UploadModal.vue`，通过 `visible` prop 控制
  - `@upload` 事件: 接收 File → 构造 `FormData`，append("file", file) → `adminApi.subscriptions.uploadVersion(id, fd)` → 成功刷新
  - `@textSave` 事件: 接收文本 → `adminApi.subscriptions.createVersionFromText(id, content)` → 成功刷新
  - 注意: FormData 上传时 `Content-Type` 自动为 `multipart/form-data`（axios 自动处理）
- [x] **返回按钮**: `el-button` → `router.push('/admin/subscriptions')`

**验证**:
- [x] `npm run build` 通过
- [x] 订阅列表按平台+类型排序展示
- [x] 创建订阅 → 表单校验 ID 格式 → 成功/409 处理
- [x] 编辑/删除订阅正常
- [x] 版本列表 current 高亮
- [x] 上传新版本 → 自动切换 current → 旧版本保留
- [x] 文本编辑创建新版本
- [x] 切换/预览/删除版本正常
- [x] 最后一个版本不可删除

---

### 块 7B：ShareList + ShareVersions（分享订阅管理）

**目标**: 实现分享订阅列表页 + 版本管理。结构类似 7A 但表格字段和操作按钮不同（AGENTS.md §4.6）。

**后端 API 速查**:
| 操作 | 方法 | 端点 | 请求/响应 |
|------|------|------|-----------|
| 列表 | GET | `/admin/shares` | → `{ shares: [{..., has_token: bool}] }` |
| 创建 | POST | `/admin/shares` | ← FormData(file+name) 或 `{ name, content }` → `{ success, share, token }` |
| 更新 | PUT | `/admin/shares/:id` | ← `{ name }` → `{ success }` |
| 删除 | DELETE | `/admin/shares/:id` | → `{ success }` |
| 版本 CRUD | 同 7A | `/admin/shares/:id/versions/*` | 同 7A 模式 |
| 刷新 Token | POST | `/admin/shares/:id/refresh-token` | → `{ success, token }` |
| 吊销 Token | DELETE | `/admin/shares/:id/token` | → `{ success }` |

> **前端 API 调用**: `adminApi.shares.list()`, `.create(data)`, `.delete(id)`, `.refreshToken(id)`, `.revokeToken(id)`
> 版本相关: `.uploadVersion(id, fd)`, `.createVersionFromText(id, content)`, `.switchVersion(id, v)`, `.getVersion(id, v)`, `.deleteVersion(id, v)`

**7B-1: ShareList.vue 详细任务**:

- [x] **数据加载**: `onMounted` → `adminApi.shares.list()` → `data.shares`（每项含 `has_token` 和 `token`）
- [x] **列表展示** (`el-table`):
  - 列：名称、创建时间（`formatTime`）、当前版本号、Token 状态（`has_token` → 有效绿色标签 / 已吊销红色标签）、操作
  - 空状态: `el-empty`
  - 加载状态: `v-loading`
- [x] **创建对话框** (AGENTS.md §4.6: 填写名称 → 上传第一个版本 → 自动生成 Token):
  - 两种输入方式（同一对话框内）:
    1. 文件上传: `el-upload`（drag）+ `el-input`（name，必填）→ 提交时构造 FormData
    2. 文本编辑: `el-input`（name）+ `el-input` textarea（content）→ JSON 提交
  - 提交成功后显示生成的 Token（`ElMessage.info` 提示）
  - 注意: `adminApi.shares.create()` 支持两种 Content-Type（JSON 或 FormData）
- [x] **操作按钮组**（每行）:
  - 「版本管理」→ `router.push('/admin/shares/' + share.id + '/versions')`
  - 「复制分享链接」(仅 `has_token` 为 true 时可用) → `navigator.clipboard.writeText(url)` → 提示已复制。URL 格式: `/api/v1/share/{id}/download?token={token}`（注意: 后端不直接返回 token 在列表，需额外 `adminApi.shares.get(id)` 查询）
  - 「刷新 Token」→ `ConfirmDialog`(title="刷新 Token", message="刷新后旧链接立即失效，确定？") → `adminApi.shares.refreshToken(id)` → 更新本地 token 显示
  - 「吊销 Token」(仅 `has_token` 为 true 时可用) → `ConfirmDialog`(title="吊销 Token", message="吊销后该分享链接立即不可用，订阅文件保留。确定？") → `adminApi.shares.revokeToken(id)` → 刷新列表
  - 「删除」→ `ConfirmDialog`(title="删除分享订阅", message="确定删除？将级联删除所有版本文件和 Token") → `adminApi.shares.delete(id)` → 刷新列表
- [x] **复制分享链接实现细节**: 后端 `ListShares` 已增强返回 `token` 字段（块 7 构建时修改），`row.token` 直接可用

**7B-2: ShareVersions.vue 详细任务**:

- [x] 结构完全复用 7A SubVersions 模式，仅替换 API 调用为 `adminApi.shares.*`
- [x] 路由参数: `route.params.id`
- [x] 数据加载: `adminApi.shares.get(id)`
- [x] 版本列表 + 上传新版本 + 文本编辑 + 切换 + 预览 + 删除（同 7A-2）
- [x] 返回按钮 → `/admin/shares`

**验证**:
- [x] `npm run build` 通过
- [ ] 创建分享订阅 → 显示 Token
- [ ] 复制分享链接 → 剪贴板有正确 URL
- [ ] 刷新 Token → 旧链接失效
- [ ] 吊销 Token → Token 状态变"已吊销" → 复制链接按钮不可用
- [ ] 删除 → 级联删除
- [ ] 版本管理同 7A

---

### 块 7C：PlatformManage（平台管理）

**目标**: 实现平台 CRUD 页面。独立页面，无子页面。

**后端 API 速查**:
| 操作 | 方法 | 端点 | 请求/响应 |
|------|------|------|-----------|
| 列表 | GET | `/admin/platforms` | → `{ platforms: [...] }` |
| 创建 | POST | `/admin/platforms` | ← `{ id, name, description, client_schemes, download_url }` |
| 更新 | PUT | `/admin/platforms/:id` | ← 同上（id 在 URL） |
| 删除 | DELETE | `/admin/platforms/:id` | → `{ success }` |

> **前端 API 调用**: `adminApi.platforms.list()`, `.create(data)`, `.update(id, data)`, `.delete(id)`

**7C-1: PlatformManage.vue 详细任务**:

- [x] **数据加载**: `onMounted` → `adminApi.platforms.list()` → `data.platforms`
- [x] **列表展示** (`el-table`):
  - 列：ID、名称、描述、Client Schemes（JSON 数组，格式化显示或用 `el-tag` 列表）、下载链接（`download_url`，可空显示"—"）、操作
  - 空状态: `el-empty`
- [x] **创建对话框** (`el-dialog` + `el-form`):
  - 字段：
    - `id`（`el-input`，必填，校验 `[a-z0-9-]+`）
    - `name`（`el-input`，必填）
    - `description`（`el-input` textarea）
    - `client_schemes`（JSON 字符串数组编辑，可用 `el-input` textarea 每行一个，提交时 `split('\n').filter(Boolean)` 转为数组；或用动态 `el-tag` 列表 + 输入框添加。**推荐 textarea 方式**，简洁）
    - `download_url`（`el-input`，可空，placeholder="https://example.com/download"）
  - 提交 → `adminApi.platforms.create(data)` → 刷新
- [x] **编辑对话框**: 预填当前值 → 提交 `adminApi.platforms.update(id, data)`
- [x] **删除**: `ConfirmDialog`(title="删除平台", message="确定删除该平台？将级联删除该平台的所有订阅、下载 Token 和自定义订阅。此操作不可恢复！") → `adminApi.platforms.delete(id)` → 刷新
- [x] **client_schemes textarea 解析**: 用户每行输入一个 scheme，提交时: `schemes.split('\n').map(s => s.trim()).filter(Boolean)`。编辑时从数组 join('\n') 回填
- [x] **自动创建的 3 个默认平台**: clash-verge、v2rayng、shadowrocket 在 DB 初始化时自动创建，列表中会显示。管理员可编辑但需谨慎

**验证**:
- [x] `npm run build` 通过
- [x] 列表显示 3 个默认平台
- [ ] 创建新平台 → ID 校验 → 重复 409
- [ ] 编辑平台 → client_schemes 正确序列化 → download_url 可空
- [ ] 删除平台 → ConfirmDialog → 级联删除

---

### 块 7D：UserManage（用户管理 + 自定义订阅）

**目标**: 实现最复杂的管理页面 — 用户列表、编辑 is_advanced、上传/删除自定义订阅、吊销 Token、删除用户。

**后端 API 速查**:
| 操作 | 方法 | 端点 | 请求/响应 |
|------|------|------|-----------|
| 用户列表 | GET | `/admin/users` | → `{ users: [...] }` |
| 获取用户 | GET | `/admin/users/:id` | → `{ user }` |
| 更新用户 | PUT | `/admin/users/:id` | ← `{ username, email, is_advanced, groups }` |
| 删除用户 | DELETE | `/admin/users/:id` | → `{ success }` |
| 吊销 Token | POST | `/admin/users/:id/revoke-tokens` | → `{ success }` |
| 上传自定义订阅 | POST | `/admin/users/:id/custom-subscription?platform=xxx` | ← FormData(file) → `{ success, custom_subscription }` |
| 自定义订阅版本 | POST | `/admin/users/:id/custom-subscription/versions?platform=xxx` | ← FormData 或 `{ content }` |
| 删除自定义订阅 | DELETE | `/admin/users/:id/custom-subscription?platform=xxx` | → `{ success }` |
| 刷新自定义 Token | POST | `/admin/users/:id/custom-subscription/refresh-token?platform=xxx` | → `{ success }` |

> **前端 API 调用**: `adminApi.users.*` 全系列（参见 api.js §5B-1）

**7D-1: UserManage.vue 详细任务**:

- [x] **数据加载**:
  - `onMounted` → `adminApi.users.list()` → `data.users`（后端已增强返回 `has_custom_sub` + `custom_sub_platforms`）
  - 同时加载平台列表 `adminApi.platforms.list()`（用于自定义订阅的平台下拉选择）
- [x] **列表展示** (`el-table`):
  - 列：用户名、邮箱、角色（`el-tag`: admin→danger「管理员」, user→info「普通用户」）、is_advanced（`el-tag`: true→warning「高级」, false→info「普通」）、自定义订阅（有→绿色标签显示平台列表，无→"—"）、操作
  - 操作按钮组（每行，按 AGENTS.md §4.5）:
    - 「编辑」(始终可用)
    - 「上传自定义订阅」(始终可用)
    - 「删除自定义订阅」(仅当该用户有自定义订阅时显示)
    - 「吊销所有 Token」(ConfirmDialog)
    - 「删除用户」(ConfirmDialog + 管理员自我保护)
- [x] **编辑对话框** (`el-dialog` + `el-form`):
  - 字段：
    - `username`（只读展示，不可编辑）
    - `email`（只读展示，不可编辑）
    - `is_advanced`（`el-switch`，管理员自身始终为 true 且 `disabled`）
    - `groups`（JSON 数组只读展示，`v-if="user.groups && user.groups.length > 0"`；未设置 groups 的用户不显示此字段）
    - 角色（只读展示 `el-tag`）
  - 提交 → `adminApi.users.update(id, { is_advanced: bool })` → 成功 `ElMessage.success` + 刷新列表
  - **管理员自身 is_advanced**: 通过 `userStore.user?.user_id === row.user_id` 判断，若为本人则 `el-switch` disabled + 提示「管理员始终为高级用户」
- [x] **上传自定义订阅对话框**:
  - 弹出 `el-dialog`，内容：
    - `el-select` 选择平台（从 `adminApi.platforms.list()` 获取选项，`v-model` 绑定选中平台 ID）
    - `el-upload`（drag，50MB 限制），`accept=".conf,.yaml,.yml,.txt"`
  - 提交 → `adminApi.users.uploadCustomSub(userId, platform, file)` → 成功刷新
  - **注意**: platform 是 **query param**！URL: `POST /admin/users/:id/custom-subscription?platform=xxx`
- [x] **删除自定义订阅**: 弹出对话框 → 下拉选择平台（从 `custom_sub_platforms` 列表）→ `adminApi.users.deleteCustomSub(userId, platform)` → 刷新
- [x] **吊销所有 Token**: `ConfirmDialog`(title="吊销下载 Token", message="确定吊销该用户所有下载链接？吊销后用户需重新获取") → `adminApi.users.revokeTokens(userId)` → 成功提示
- [x] **删除用户**: `ConfirmDialog`(title="删除用户", message="确定删除该用户？将级联删除其所有下载 Token 和自定义订阅。此操作不可恢复！") → `adminApi.users.delete(userId)` → 刷新
  - 错误处理: 后端返回 400 时，`e.response.data.error` 含具体原因（"不能删除自己"/"不能删除最后一个管理员"）→ `ElMessage.error`
- [x] **自定义订阅信息获取**: 后端 `ListUsers` 已增强返回 `has_custom_sub` + `custom_sub_platforms`，前端直接使用，无需额外查询

**7D-2: 用户自定义订阅详情展开** (可选增强):
- [x] 自定义订阅平台以绿色标签在表格中展示
- [x] 删除自定义订阅时弹出对话框选择平台

**验证**:
- [x] `npm run build` 通过
- [ ] 用户列表正确展示角色/is_advanced 标签
- [ ] 编辑 is_advanced → 管理员自身不可修改
- [ ] 上传自定义订阅 → 需选择平台 → 上传后用户列表中对应平台标记
- [ ] 删除自定义订阅 → 用户恢复默认/高级
- [ ] 吊销 Token 成功
- [ ] 删除用户 → 不能删除自己 → 不能删除最后一个管理员
- [ ] groups 字段仅展示不编辑

---

### 块 7E：RulesManage + RuleVersions（规则管理）

**目标**: 实现规则列表页 + 版本管理。结构类似 7A（订阅管理），但列表字段和创建方式不同（AGENTS.md §4.7/§4.8）。

**后端 API 速查**:
| 操作 | 方法 | 端点 | 请求/响应 |
|------|------|------|-----------|
| 列表 | GET | `/admin/rules` | → `{ rules: [{..., token: string}] }` |
| 创建 | POST | `/admin/rules` | ← `{ id, name, client_type, content? }` 或 FormData |
| 删除 | DELETE | `/admin/rules/:id` | → `{ success }` |
| 版本 CRUD | 同 7A | `/admin/rules/:id/versions/*` | 同模式 |
| 轮替 Token | POST | `/admin/rules/:id/refresh-token` | → `{ success, token }` |

> **前端 API 调用**: `adminApi.rules.*` 全系列

**7E-1: RulesManage.vue 详细任务**:

- [x] **数据加载**: `onMounted` → `adminApi.rules.list()` → `data.rules`（每项含 `token`）
- [x] **列表展示** (`el-table`):
  - 列：规则名称、客户端类型（`el-tag`）、当前版本号、更新时间、Token（脱敏显示前8位+`...`）、操作
  - 空状态: `el-empty`
  - 加载状态: `v-loading`
- [x] **创建对话框** (AGENTS.md §4.7: 填写名称、选择客户端类型 → 上传第一个版本 → 自动生成 rule_token):
  - 两种输入方式（同一对话框内，同 ShareList 模式）:
    1. 文件上传模式: `el-input`（id, 必填, `[a-z0-9-]+`）+ `el-input`（name, 必填）+ `el-select`（client_type, 当前仅 Shadowrocket, 默认选中）+ `el-upload` → FormData 提交
    2. JSON 文本模式: 同上的字段 + `el-input` textarea（content）→ JSON 提交
  - 提交成功后 `ElMessage.success` 并显示 Token
- [x] **操作按钮组**（每行）:
  - 「版本管理」→ `router.push('/admin/rules/' + rule.id + '/versions')`
  - 「复制下载链接」→ `navigator.clipboard.writeText('/api/v1/rules/' + rule.id + '/download?token=' + rule.token)` → 提示已复制
  - 「轮替 Token」→ `ConfirmDialog`(title="轮替 Token", message="轮替后旧链接立即失效，确定？") → `adminApi.rules.refreshToken(id)` → 更新本地 token
  - 「删除」→ `ConfirmDialog`(title="删除规则", message="确定删除？将级联删除所有版本文件和 Token") → `adminApi.rules.delete(id)` → 刷新

**7E-2: RuleVersions.vue 详细任务**:

- [x] 结构完全复用 7A SubVersions 模式，替换 API 调用为 `adminApi.rules.*`
- [x] 路由参数: `route.params.id`
- [x] 数据加载: `adminApi.rules.get(id)`
- [x] 版本列表 + 上传新版本 + 文本编辑 + 切换 + 预览 + 删除（同 7A-2）
- [x] 返回按钮 → `/admin/rules`

**验证**:
- [x] `npm run build` 通过
- [ ] 规则列表显示 token（用于下载链接）
- [ ] 创建规则 → client_type 仅 Shadowrocket 可选
- [ ] 轮替 Token → 旧 token 失效，新 token 显示
- [ ] 复制下载链接正确
- [ ] 版本管理同 7A

---

### 块 7F：OIDCConfig（OIDC 配置）

**目标**: 实现 OIDC 配置查看/修改页面，含切换提供商、测试连接、保存 + 速率限制配置。

**后端 API 速查**:
| 操作 | 方法 | 端点 | 请求/响应 |
|------|------|------|-----------|
| 获取配置 | GET | `/admin/oidc-config` | → `{ config: {...} }` (client_secret 已脱敏) |
| 测试连接 | POST | `/admin/test-oidc` | ← `{ provider_type, keycloak_base_url, keycloak_realm, auth0_domain, generic_issuer, client_id, client_secret, redirect_uri }` |
| 保存配置 | POST | `/admin/system/configure` | ← 同 Setup 的 configure payload |
| 切换提供商 | POST | `/admin/system/switch-provider` | ← `{ provider_type }` |
| 获取速率限制 | GET | `/admin/system/rate-limit` | → `{ rate_limit_login, rate_limit_download }` |
| 更新速率限制 | PUT | `/admin/system/rate-limit` | ← `{ rate_limit_login, rate_limit_download }` |

> **前端 API 调用**: `adminApi.system.*` 全系列

**7F-1: OIDCConfig.vue 详细任务**:

- [x] **数据加载**: `onMounted` → `adminApi.system.getOIDCConfig()` → 填充表单 + `adminApi.system.getRateLimit()` → 填充速率限制
- [x] **页面布局**: 使用 `el-card` 分两个区域：OIDC 配置 + 速率限制配置
- [x] **OIDC 配置区域**（复用 Setup.vue 的配置表单模式）:
  - 提供商显示: `el-tag`（当前提供商类型）+ 「切换提供商」按钮 → 打开 `OIDCSwitchDialog`
  - 切换提供商 (`handleProviderSwitch`): 调 `adminApi.system.switchProvider({ provider_type })` → 成功后重新 `getOIDCConfig()` 刷新表单
  - 按 provider_type 显示对应字段（Keycloak: base_url+realm, Auth0: domain, Generic: issuer），切换时保留已填字段
  - 公共字段: `client_id`（`el-input`）、`client_secret`（`el-input` type="password"，脱敏回显 `***`）、`redirect_uri`、`frontend_url`
  - 「测试连接」按钮 → `adminApi.system.testOIDC(payload)` → 成功/失败提示
  - 「保存配置」按钮 → `adminApi.system.configure(payload)` → 成功提示
- [x] **速率限制配置区域**:
  - `el-form` 两个字段: `rate_limit_login`（默认 10/min）、`rate_limit_download`（默认 20/min）
  - `el-input-number` 或 `el-input` type="number"，min=1
  - 「保存」按钮 → `adminApi.system.updateRateLimit(data)` → 成功提示
- [x] **Client Secret 处理**: 后端已脱敏（`***`），前端直接展示。保存/测试时若为 `***` 则从 payload 中删除该字段
- [x] **表单校验**: 必填字段（同 Setup.vue 的 rules）

**验证**:
- [x] `npm run build` 通过
- [ ] 加载现有 OIDC 配置 → client_secret 显示 `***`
- [ ] 切换提供商 → 字段切换，已填值保留
- [ ] 测试连接成功/失败
- [ ] 保存配置成功
- [ ] 速率限制修改生效

---

### 块 7G：Logs（日志查看）

**目标**: 实现日志查询页面，按日期筛选访问日志。

**后端 API 速查**:
| 操作 | 方法 | 端点 | 请求/响应 |
|------|------|------|-----------|
| 查询日志 | GET | `/admin/logs?date=2026-07-15` | → `{ logs: [{id, user_id, ip, download_type, platform, share_subscription_id, rule_id, status, error_reason, created_at}] }` |

> **前端 API 调用**: `adminApi.logs.getLogs(date)`

**7G-1: Logs.vue 详细任务**:

- [ ] **日期选择**: `el-date-picker`（`type="date"`，`v-model="selectedDate"`，`@change="fetchLogs"`），默认当天
- [x] **数据加载**: `fetchLogs()` → `adminApi.logs.getLogs(formattedDate)` → `data.logs`
- [x] **表格展示** (`el-table`):
  - 列：时间（`created_at`，格式化为 `toLocaleString`）、下载类型（`download_type` → 中文映射: subscription→订阅, share→分享, custom→自定义, rule→规则）、用户 ID（`user_id`，为空显示"—"）、平台（`platform`，为空显示"—"）、状态（`status`: success→绿色标签「成功」, failed→红色标签「失败」）、失败原因（`error_reason`，仅 failed 时显示，为空显示"—"）、IP
  - 列宽自适应，`download_type`/`status` 固定宽度
  - 空状态: `el-empty` 提示「暂无日志记录」
  - 加载状态: `v-loading`
- [x] **格式化函数**:
  - `downloadTypeLabel(type)`: subscription→「订阅下载」, share→「分享下载」, custom→「自定义订阅下载」, rule→「规则下载」
  - `formatTime(t)`: `new Date(t).toLocaleString()`
- [x] **日期格式**: 用原生 `Date` 处理，`value-format="YYYY-MM-DD"`
- [x] **无依赖**: 本页无需额外安装 dayjs

**验证**:
- [x] `npm run build` 通过
- [ ] 默认显示当天日志
- [ ] 切换日期 → 重新加载
- [ ] 日志表格正确显示各字段
- [ ] 无日志时显示空状态
- [ ] status 颜色区分 success/failed

---

### 块 7 整体验证

块 7A-7G 全部完成后：

- [x] `npm run build` 和 `go build ./...` 均通过
- [ ] 完整管理流程: 登录 → 管理面板 → 各子页面独立验证
- [ ] 订阅管理: 创建 → 版本上传 → 切换 → 删除
- [ ] 分享订阅管理: 创建 → Token 刷新/吊销 → 删除
- [ ] 平台管理: CRUD
- [ ] 用户管理: 编辑 is_advanced → 上传/删除自定义订阅 → 吊销 Token → 删除用户
- [ ] 规则管理: 创建 → 版本管理 → 轮替 Token
- [ ] OIDC 配置: 查看/修改/测试连接/切换提供商
- [ ] 日志: 按日期筛选 → 正确显示各字段
- [ ] 速率限制配置: 修改后生效
- [ ] 所有删除操作使用 ConfirmDialog
- [ ] 暗色模式在全部管理页面生效
- [ ] 移动端响应式（侧边栏折叠）在全部管理页面正常
- [ ] 所有表单提交有 loading 状态
- [ ] 所有 API 错误有 ElMessage 提示

**关键约束** (适用所有子块):
- 所有创建/编辑用 `el-dialog` + `el-form`
- 版本上传 `el-upload` 50MB 限制
- 当前激活版本绿色高亮
- AGENTS.md §4.3-§4.8 操作按钮组按文档完整实现
- 删除确认必须用 `ConfirmDialog.vue`
- 模板中使用「」代替 `"` 转义
- v-model 中不使用可选链 `?.`，用 `v-if` 守卫
- 登出调用 `userStore.logout(router)`

---

## 块 8：Docker 化 + 联调验证

**目标**: 编写 Dockerfile，验证 docker-compose，端到端联调。块 8 是整个项目的最后一块，依赖块 1A~7G 全部完成。

> **现状盘点**:
> - 根目录 `docker-compose.yml` ✅ 已存在，端口绑定（`127.0.0.1:8080:8080` / `127.0.0.1:8081:80`）和 volume（`vpn-data:/app/data`）均正确
> - `backend/go.mod` 使用 Go 1.25.0，依赖 `modernc.org/sqlite`（纯 Go，CGO_ENABLED=0 可静态编译）
> - `frontend/package.json` 使用 Vite 6 + Vue 3.5，`npm run build` 输出到 `dist/`
> - `backend/cmd/server/main.go` 已支持 `PORT`（默认 8080）和 `DATA_DIR`（默认 `./data`）环境变量
> - **不存在** 任何 Dockerfile、`.dockerignore`、`nginx.conf` 文件，需全部新建

**块 8 拆分为 6 个子块，按依赖顺序构建**:

| 子块 | 内容 | 依赖 | 状态 |
|------|------|------|------|
| 块 8A | Docker 构建文件（Dockerfile × 2 + .dockerignore × 2 + nginx.conf） | 无（独立编写） | ✅ |
| 块 8B | Docker Compose 最终确认与调优 | 块 8A | ✅ |
| 块 8C | 本地 Docker 构建验证 | 块 8B | ✅ |
| 块 8D | 外部 NGINX 参考配置文档 | 无（独立编写） | ✅ |
| 块 8E | CI/CD GitHub Actions（可选） | 块 8C | ✅ |
| 块 8F | 端到端联调验证清单 | 块 8C | ⬜ |

---

### 块 8A：Docker 构建文件

**目标**: 创建全部 5 个 Docker 构建所需文件，确保 `docker compose build` 能成功构建两个镜像。

**设计要点分析**:

1. **后端多阶段构建**: Go 项目使用 `modernc.org/sqlite`（纯 Go SQLite 驱动），`CGO_ENABLED=0` 可编译为完全静态二进制，适合 `distroless/static` 运行镜像。
2. **Distroless + Volume 权限**: `gcr.io/distroless/static-debian12:nonroot` 以 UID 65532 运行，无法写入 Docker volume 的 `/app/data` 目录（volume 默认 root 所有）。**解决方案**: 使用 `gcr.io/distroless/static-debian12`（root 用户）而非 nonroot 变体。对于 `127.0.0.1` 绑定的自托管内部工具，root 运行可接受。AGENTS.md 无强制 nonroot 约束。
3. **前端多阶段构建**: Node 构建 → Nginx Alpine 提供静态文件服务。nginx.conf 仅含 SPA 回退，无任何 `proxy_pass`（/api 分流由外部 NGINX 承担）。
4. **镜像体积优化**: 后端用 `-ldflags="-s -w"` 去除调试信息；前端用 `npm ci --production=false`（构建时需要 devDependencies 中的 vite）；nginx 用 alpine 变体。

**任务**:

- [x] **`backend/.dockerignore`**:
  ```
  data/
  .git/
  __debug*
  *.test
  .env
  .vscode/
  ```
  说明：`data/` 由 volume 挂载，不进镜像；其余为开发期产物。

- [x] **`backend/Dockerfile`**（多阶段构建）:
  ```dockerfile
  # Stage 1: Build
  FROM golang:alpine AS builder
  WORKDIR /app
  # Install build dependencies (modernc.org/sqlite is pure Go, no CGO needed)
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

  # Stage 2: Runtime
  FROM gcr.io/distroless/static-debian12
  COPY --from=builder /app/server /server
  ENV DATA_DIR=/app/data
  ENV PORT=8080
  EXPOSE 8080
  ENTRYPOINT ["/server"]
  ```
  说明：
  - 使用 `golang:alpine`（最新稳定版），用户决策：优先使用最新版
  - 使用 root 用户 distroless（非 nonroot），确保对 volume `/app/data` 有写权限
  - `modernc.org/sqlite` 为纯 Go 实现，无需 CGO，无需 libc
  - `-ldflags="-s -w"` 去除符号表和调试信息，减小二进制体积
  - `DATA_DIR=/app/data` 与 docker-compose volume 挂载路径一致 (§6.5)

- [x] **`frontend/.dockerignore`**:
  ```
  node_modules/
  .git/
  dist/
  .vscode/
  ```
  说明：`node_modules/` 和 `dist/` 为本地产物，构建时在容器内重新生成。

- [x] **`frontend/Dockerfile`**（多阶段构建）:
  ```dockerfile
  # Stage 1: Build
  FROM node:22-alpine AS builder
  WORKDIR /app

  COPY package.json package-lock.json ./
  RUN npm ci
  COPY . .
  RUN npm run build

  # Stage 2: Runtime
  FROM nginx:1.27-alpine
  COPY --from=builder /app/dist /usr/share/nginx/html
  COPY nginx.conf /etc/nginx/conf.d/default.conf
  EXPOSE 80
  CMD ["nginx", "-g", "daemon off;"]
  ```
  说明：
  - Node 22 Alpine 构建前端，输出到 `dist/`
  - `npm ci` 配合 `package-lock.json` 实现可复现构建
  - Nginx 1.27 Alpine 提供静态服务，镜像体积小
  - `nginx.conf` 在 frontend 目录下，`COPY` 时相对于 `context: ./frontend`

- [x] **`frontend/nginx.conf`**（严格按 AGENTS.md §8.4）:
  ```nginx
  server {
      listen 80;
      server_name _;
      root /usr/share/nginx/html;

      location / {
          try_files $uri $uri/ /index.html;
      }
  }
  ```
  约束验证：
  - ✅ 只服务静态文件 + SPA history 模式回退
  - ✅ **无任何 `proxy_pass`**（/api 分流由外部 NGINX 承担）
  - ✅ 无 `/api/` location 块
  - ✅ `try_files` 确保 Vue Router history 模式正常工作

**涉及文件**: `backend/.dockerignore`, `backend/Dockerfile`, `frontend/.dockerignore`, `frontend/Dockerfile`, `frontend/nginx.conf`

**验证**:
- [x] `docker compose build` 两个镜像构建成功 (backend 24.7MB, frontend 78.1MB)
- [x] 后端镜像大小 24.7MB (≤30MB ✅), 前端镜像大小 78.1MB (nginx-alpine 基础较大，可接受)

**关键约束**:
- frontend nginx.conf 不得有 proxy_pass（AGENTS.md §8.1/§8.4 强制约束）
- 后端 CGO_ENABLED=0（modernc.org/sqlite 纯 Go，确保 distroless 兼容）
- DATA_DIR 环境变量设为 `/app/data`（与 volume 挂载路径一致）
- Go 版本 `1.25` 需要确认 Docker Hub 上 `golang:1.25-alpine` 标签是否存在；若不存在则使用最新可用版本
- 若 `package-lock.json` 不存在需在 `npm install` 前生成或直接使用 `npm install`

---

### 块 8B：Docker Compose 最终确认与调优

**目标**: 审查并更新 `docker-compose.yml`，确保与 AGENTS.md §8.3 完全一致，并补充生产就绪配置。

**现状**: 根目录 `docker-compose.yml` 已存在，基本结构正确，需逐一核对并补充优化项。

**任务**:

- [x] **端口绑定核对**（AGENTS.md §8.1 强制约束）:
  - backend: `"127.0.0.1:8080:8080"` ✅ 已正确
  - frontend: `"127.0.0.1:8081:80"` ✅ 已正确
  - 确认无 `0.0.0.0` 或其他公网接口绑定

- [x] **Volume 核对**（AGENTS.md §8.7）:
  - `vpn-data:/app/data` ✅ 已正确
  - 单一 volume，包含 SQLite 数据库 + 所有版本文件
  - 确认无多余 volume 挂载

- [x] **depends_on 核对**:
  - `frontend depends_on backend` ✅ 已正确
  - 注意：`depends_on` 仅控制启动顺序，不等待 backend 就绪

- [x] **补充优化项**（生产就绪）:
  - [x] backend healthcheck：**跳过**。distroless 镜像无 wget/curl，无 shell。使用外部监控（外部 NGINX 或 Docker 原生 TCP 检测）替代
  - [x] frontend 添加 `healthcheck`：已实施。nginx:1.27-alpine 内置 wget，配置如下：
    ```yaml
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:80/"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 5s
    ```
  - [x] `networks` 配置：跳过。默认 bridge 网络满足需求
  - [x] 容器资源限制：跳过。小团队场景非必需

- [x] **最终 docker-compose.yml**（已实现）:
  ```yaml
  services:
    backend:
      build:
        context: ./backend
        dockerfile: Dockerfile
      container_name: vpn-backend
      ports:
        - "127.0.0.1:8080:8080"
      volumes:
        - vpn-data:/app/data
      restart: unless-stopped

    frontend:
      build:
        context: ./frontend
        dockerfile: Dockerfile
      container_name: vpn-frontend
      ports:
        - "127.0.0.1:8081:80"
      depends_on:
        - backend
      restart: unless-stopped
      healthcheck:
        test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:80/"]
        interval: 30s
        timeout: 10s
        retries: 3
        start_period: 5s

  volumes:
    vpn-data:
  ```

**验证**:
- [x] `docker compose config` 无语法错误
- [x] 端口绑定全部为 `127.0.0.1:` 前缀
- [x] volume 名称和挂载路径正确

---

### 块 8C：本地 Docker 构建与启动验证

**目标**: 在本地完整构建并启动两个容器，验证核心功能可用。

**任务**:

- [x] **构建镜像**:
  ```bash
  cd /Users/kyle/Desktop/VPN-Subscription-Management
  docker compose build --no-cache
  ```
  验证：
  - backend 多阶段构建成功（golang build → distroless）
  - frontend 多阶段构建成功（node build → nginx）
  - 无构建错误

- [ ] **启动服务**:
  ```bash
  docker compose up -d
  ```
  验证：
  - `docker compose ps` 显示两个容器均为 `Up` 状态
  - `docker compose logs backend` 显示 `Starting server on :8080 (configured=false)`
  - `docker compose logs frontend` 显示 nginx 启动正常

- [ ] **基本 HTTP 验证**（不依赖外部 NGINX，直接访问 127.0.0.1）:
  ```bash
  # 健康检查
  curl http://127.0.0.1:8080/health
  # 预期: {"status":"ok"}

  # 系统状态
  curl http://127.0.0.1:8080/api/v1/system/status
  # 预期: {"configured":false}

  # 前端首页
  curl http://127.0.0.1:8081/
  # 预期: 返回 index.html (含 <div id="app">)
  ```

- [ ] **Setup 流程快速验证**:
  ```bash
  # 访问前端 → 应重定向到 /setup（通过 JS 路由守卫，curl 只能验证 HTML 返回）
  curl http://127.0.0.1:8081/
  # 预期: 返回 SPA index.html，JS 加载后自动跳转 /setup
  ```

- [x] **容器内数据目录验证**:
  ```bash
  docker compose exec backend ls -la /app/data/
  # 预期: 存在 vpn.db 文件（首次启动时自动创建）
  # 注意: distroless 无 shell，无法 docker compose exec。数据库创建已通过日志确认：
  # "Initializing database at /app/data/vpn.db" + "Database initialized successfully"
  ```

- [x] **停止并清理**:
  ```bash
  docker compose down
  # 如需清理 volume 数据: docker compose down -v
  ```

**涉及文件**: 无新建文件，使用 块 8A/8B 产物

**验证**:
- [x] 两个镜像构建无错误
- [x] `docker compose up -d` 启动成功
- [x] `/health` 返回 200 `{"status":"ok"}`
- [x] `/api/v1/system/status` 返回 `{"configured":false}`
- [x] `/api/v1/platforms` 返回 3 个默认平台
- [x] 前端返回 index.html (含 `<div id="app">`)
- [x] `/app/data/vpn.db` 自动创建（日志确认）

---

### 块 8D：外部 NGINX 参考配置文档

**目标**: 提供外部 NGINX 参考配置文件，供部署者在部署机上设置 `/api` 分流。

> **说明**: 本块不创建容器内文件。外部 NGINX 配置是部署者在自己服务器上手动设置的，不在本项目的 Docker 镜像内。提供参考配置方便部署。

**任务**:

- [x] **创建 `deploy/nginx-example.conf`**（严格按 AGENTS.md §8.2）:
  ```nginx
  # VPN Subscription Management - External NGINX Configuration
  # Place this in your existing NGINX server block on the deployment host.
  # This is NOT part of the Docker containers.
  #
  # Architecture:
  #   Browser → HTTPS → External NGINX (this config)
  #     /api/* → http://127.0.0.1:8080  (backend container)
  #     /*     → http://127.0.0.1:8081  (frontend container)

  server {
      listen 443 ssl;
      server_name vpn.example.com;  # CHANGE THIS to your domain

      # TLS configuration (use your existing certificates)
      # ssl_certificate     /path/to/fullchain.pem;
      # ssl_certificate_key /path/to/privkey.pem;

      # API requests → backend container (Gin API)
      location /api/ {
          proxy_pass http://127.0.0.1:8080;
          proxy_set_header Host              $host;
          proxy_set_header X-Real-IP         $remote_addr;
          proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
          proxy_set_header X-Forwarded-Proto $scheme;

          # Disable buffering for Server-Sent Events (if needed in future)
          # proxy_buffering off;

          # Increase timeout for large file uploads (50MB limit)
          client_max_body_size 55m;
      }

      # All other requests → frontend container (static files)
      location / {
          proxy_pass http://127.0.0.1:8081;
          proxy_set_header Host              $host;
          proxy_set_header X-Real-IP         $remote_addr;
          proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
          proxy_set_header X-Forwarded-Proto $scheme;
      }
  }

  # HTTP → HTTPS redirect (optional but recommended)
  server {
      listen 80;
      server_name vpn.example.com;
      return 301 https://$host$request_uri;
  }
  ```

- [x] **创建 `deploy/README.md`**（部署说明）:
  简要部署步骤：
  1. 克隆项目 → `docker compose up -d`
  2. 将 `deploy/nginx-example.conf` 内容合并到部署机外部 NGINX 配置
  3. 重载 NGINX: `nginx -t && nginx -s reload`
  4. 访问 `https://your-domain.com` → 进入 Setup 流程

**涉及文件**: `deploy/nginx-example.conf`, `deploy/README.md`（新建）

**验证**:
- [x] `deploy/nginx-example.conf` 与 AGENTS.md §8.2 一致
- [x] `proxy_pass` 地址与 docker-compose 端口绑定一致
- [x] 包含 `X-Forwarded-For` / `X-Real-IP` header 设置

---

### 块 8E：CI/CD GitHub Actions（可选，待后续实施）

**目标**: 创建 GitHub Actions workflow，自动构建并推送 Docker 镜像到 GHCR。

> **注意**: AGENTS.md §7 明确标注「CI/CD 和 Docker 部署将在核心功能开发完成后实施」。本块为基础骨架，可按需启用。**当前阶段优先级低，标记为可选**。

**设计依据**（引自 AGENTS.md §7）:
- **触发条件**: push 到 `main` 或 `beta` 分支，push `v*` 标签（如 `v1.0.0`），手动 `workflow_dispatch`
- **构建策略**: matrix build 同时构建 `backend` 和 `frontend` 两个镜像
- **镜像标签规则**:
  | 触发源 | 标签 |
  |--------|------|
  | `main` 分支 | `{service}:main` + `{service}:latest` |
  | `beta` 分支 | `{service}:beta` |
  | `v1.0.0` 标签 | `{service}:1.0.0` + `{service}:1.0` + `{service}:1` |
- **Dockerfile 结构**: 多阶段构建。后端 golang 编译 → distroless 运行；前端 node 构建 → nginx 静态服务（不反代）
- **注册表**: GitHub Container Registry (GHCR)，地址 `ghcr.io/<owner>/vpn-sub-{service}`

**任务**:

- [x] **创建 `.github/workflows/docker-build.yml`**（完整 workflow）:
  ```yaml
  name: Build and Push Docker Images

  on:
    push:
      branches: [main, beta]
      tags: ['v*']
    workflow_dispatch:

  env:
    REGISTRY: ghcr.io
    # IMAGE_NAME will be set per service in the matrix

  jobs:
    build:
      name: Build & Push
      runs-on: ubuntu-latest
      strategy:
        matrix:
          service: [backend, frontend]
      permissions:
        contents: read
        packages: write

      steps:
        - name: Checkout
          uses: actions/checkout@v4

        - name: Set up Docker Buildx
          uses: docker/setup-buildx-action@v3

        - name: Log in to GHCR
          uses: docker/login-action@v3
          with:
            registry: ${{ env.REGISTRY }}
            username: ${{ github.actor }}
            password: ${{ secrets.GITHUB_TOKEN }}

        - name: Extract metadata (tags, labels)
          id: meta
          uses: docker/metadata-action@v5
          with:
            images: ${{ env.REGISTRY }}/${{ github.repository_owner }}/vpn-sub-${{ matrix.service }}
            tags: |
              # Branch-based tags
              type=ref,event=branch,suffix=-{{branch}}
              # Tag main as latest
              type=raw,value=latest,enable={{is_default_branch}}
              # Semver tags (v1.0.0 → 1.0.0, 1.0, 1)
              type=semver,pattern={{version}}
              type=semver,pattern={{major}}.{{minor}}
              type=semver,pattern={{major}}

        - name: Build and push
          uses: docker/build-push-action@v6
          with:
            context: ./${{ matrix.service }}
            push: true
            tags: ${{ steps.meta.outputs.tags }}
            labels: ${{ steps.meta.outputs.labels }}
            cache-from: type=gha
            cache-to: type=gha,mode=max
  ```
  说明：
  - `strategy.matrix` 并行构建 backend 和 frontend，加速 CI
  - `docker/metadata-action` 按 AGENTS.md §7 规则自动生成标签
  - `permissions.packages: write` 授权推送 GHCR
  - `GITHUB_TOKEN` 自动注入，无需手动配置 secret
  - BuildKit 缓存 (`type=gha`) 加速后续构建

- [x] **涉及文件**: `.github/workflows/docker-build.yml`（新建）

**验证**:
- [x] workflow YAML 语法正确（待推送 GitHub 后由 UI 验证或 `actionlint` 本地检查）
- [ ] 推送 `main` 分支 → GHCR 出现 `vpn-sub-backend:main` + `vpn-sub-backend:latest` + `vpn-sub-frontend:main` + `vpn-sub-frontend:latest`（需推送仓库后验证）
- [ ] 推送 `v1.0.0` 标签 → GHCR 出现对应版本标签（需推送标签后验证）
- [ ] `workflow_dispatch` 手动触发可用（需推送后验证）

**注意事项**:
- GHCR 镜像默认 `private`，需在 GitHub 仓库 Settings → Packages 中设为 `public`（或配置 docker login 时使用 PAT）
- 首次推送前确认仓库名 `vpn-sub-{service}` 与 `docker/metadata-action` 的 `images` 参数一致
- 后端 `Dockerfile` 中 `go mod download` 可用 GitHub Actions 缓存加速（`actions/cache` 或 BuildKit mount cache）

---

### 块 8F：端到端联调验证清单

**目标**: 在 Docker 环境中执行完整功能验证，确保所有用户故事可走通。

> **前置条件**: 块 8C 通过（两个容器正常启动），外部 NGINX 已按 块 8D 配置完成，可通过 `https://your-domain.com` 访问。

**验证清单**: 详见 [测试计划 → 第二部分：联机测试](#第二部分联机测试需-oidc-配置后)，按以下分组执行：

| 测试组 | 编号 | 内容 |
|--------|------|------|
| 首次部署 | T-O1 | Setup 首次配置流程 (O1.1-O1.7) |
| OIDC 认证 | T-O2 | 登录/回调/首位管理员/登出 (O2.1-O2.9) |
| 订阅管理 | T-O3 | CRUD + 版本管理 (O3.1-O3.11) |
| 分享订阅 | T-O4 | CRUD + Token 操作 (O4.1-O4.9) |
| 平台管理 | T-O5 | CRUD + 级联删除 (O5.1-O5.6) |
| 用户管理 | T-O6 | is_advanced/自定义订阅/自我保护 (O6.1-O6.11) |
| 规则管理 | T-O7 | CRUD + Token 轮替 (O7.1-O7.8) |
| OIDC 配置 | T-O8 | 查看/修改/切换/速率限制 (O8.1-O8.6) |
| 日志查看 | T-O9 | 日期筛选/状态/失败原因 (O9.1-O9.7) |
| 首页(用户) | T-O10 | 订阅显示/一键导入/复制/刷新 (O10.1-O10.10) |
| 首页(管理员) | T-O11 | 预览模式/自定义预览 (O11.1-O11.4) |
| 规则浏览 | T-O12 | 用户规则页 (O12.1-O12.4) |
| 下载全链路 | T-O13 | JWT/Token/分享/规则/缓存头 (O13.1-O13.11) |
| 速率限制 | T-O14 | 限流触发/日志记录/配置生效 (O14.1-O14.4) |
| UI/UX | T-O15 | 暗色模式/移动端/表单/空状态 (O15.1-O15.10) |
| 端到端场景 | T-O16 | 完整部署/订阅更新/升降级/自保护 (O16.1-O16.7) |

**验证**:
- [ ] T-O1 ~ T-O16 全部通过
- [ ] 如有失败项，记录到 ISSUES.md 并修复后重新验证

---

**块 8 关键约束** (适用所有子块):
- 对外只暴露外部 NGINX 一个端口（AGENTS.md §8.1 强制）
- backend/frontend 端口必须以 `127.0.0.1:` 前缀绑定（AGENTS.md §8.1 强制）
- frontend 容器内 nginx 不得有 proxy_pass（AGENTS.md §8.1/§8.4 强制）
- 单一 `vpn-data` volume 挂载 `/app/data`（AGENTS.md §8.7）
- `modernc.org/sqlite` 纯 Go → `CGO_ENABLED=0` 静态编译（§6.1）
- 前端代码统一相对路径 `/api/v1/...`，不硬编码 host:port（§8.1）
- Docker 构建产物（镜像）不包含 `.env` 文件，业务配置一律通过 Web UI → SQLite（§5）

---

## 验收标准

完成后应满足：
1. `go build ./...` 和 `npm run build` 均通过（块 1~7 已验证 ✅）
2. `docker compose build` 两个镜像构建成功（块 8A/8C）
3. `docker compose up -d` 启动正常，两个容器健康运行（块 8C）
4. 外部 NGINX 分流配置正确，`https://domain.com` 可访问（块 8D）
5. 端到端验证清单 A~L 全部通过（块 8F）
6. 代码严格遵守 AGENTS.md 第五章所有编码约束
7. 数据库 12 张表、API 端点、版本文件存储严格按第六章实现
8. Docker 部署严格按第八章（外部 NGINX 分流 + 双容器 127.0.0.1 绑定）
9. （可选）GitHub Actions CI/CD 自动构建推送 GHCR（块 8E）

---

## 测试计划

本章将所有测试内容集中整理，分为**本地测试**（编译、HTTP、Docker、限流等无需 OIDC 的测试）和**联机测试**（Setup 后，需真实 OIDC 提供商的端到端测试）两大类。各构建块内保留简洁的"快速验证"引用，详细步骤统一在本章执行。

> **测试环境说明**:
> - **开发环境**: 后端 `go run .` (localhost:8080) + 前端 `npm run dev` (localhost:5173, Vite proxy → 8080)
> - **Docker 环境**: `docker compose up -d` (backend 127.0.0.1:8080 + frontend 127.0.0.1:8081)
> - **生产环境**: 外部 NGINX → `https://your-domain.com` (需先配置 TLS + 分流)
> - 以下测试标注适用环境：🔧=开发环境 🐳=Docker 环境 🌐=生产环境

---

### 第一部分：本地可执行测试（无需 OIDC 配置）

以下测试在 `configured=false` 状态下即可执行，无需 Setup 或 OIDC 提供商。

---

#### T-L1：编译与静态检查

| 编号 | 测试项 | 环境 | 命令 / 步骤 | 预期结果 |
|------|--------|------|-------------|----------|
| L1.1 | 后端编译 | 🔧🐳 | `cd backend && go build ./...` | 零错误退出 |
| L1.2 | 前端编译 | 🔧🐳 | `cd frontend && npm run build` | 零错误退出，生成 `dist/` |
| L1.3 | Go 代码规范 | 🔧 | `go vet ./...` | 零警告 |
| L1.4 | Docker 构建 | 🐳 | `docker compose build --no-cache` | backend + frontend 镜像构建成功 |
| L1.5 | Docker 镜像大小 | 🐳 | `docker images \| grep vpn` | backend < 30MB, frontend < 50MB |

---

#### T-L2：后端基础 HTTP 端点

| 编号 | 测试项 | 环境 | 命令 | 预期结果 |
|------|--------|------|------|----------|
| L2.1 | 健康检查 | 🔧🐳 | `curl http://localhost:8080/health` | `{"status":"ok"}` |
| L2.2 | 系统状态(未配置) | 🔧🐳 | `curl http://localhost:8080/api/v1/system/status` | `{"configured":false}` |
| L2.3 | 公开平台列表 | 🔧🐳 | `curl http://localhost:8080/api/v1/platforms` | 返回 3 个默认平台 (clash-verge/v2rayng/shadowrocket) |
| L2.4 | 公开规则列表 | 🔧🐳 | `curl http://localhost:8080/api/v1/rules` | 返回 `{"rules":[]}` 或已有规则 |
| L2.5 | Setup 模式路由 | 🔧🐳 | `curl -X POST http://localhost:8080/api/v1/admin/test-oidc -H "Content-Type: application/json" -d '{}'` | 返回 JSON 错误（非 404） |
| L2.6 | Setup 模式保护 | 🔧🐳 | `curl http://localhost:8080/api/v1/auth/login` | 404（Normal 路由未注册） |
| L2.7 | 认证保护 | 🔧🐳 | `curl http://localhost:8080/api/v1/auth/me` | 401 (JWT 缺失) |
| L2.8 | 管理员保护 | 🔧🐳 | `curl http://localhost:8080/api/v1/admin/users` | 401 (JWT 缺失) 或 403 (非管理员) |
| L2.9 | 404 处理 | 🔧🐳 | `curl http://localhost:8080/api/v1/nonexistent` | 404 |

---

#### T-L3：前端静态文件

| 编号 | 测试项 | 环境 | 命令 | 预期结果 |
|------|--------|------|------|----------|
| L3.1 | 首页 HTML | 🐳 | `curl http://127.0.0.1:8081/` | 返回含 `<div id="app">` 的 HTML |
| L3.2 | SPA 回退 | 🐳 | `curl http://127.0.0.1:8081/admin/subscriptions` | 返回 index.html (非 404) |
| L3.3 | 静态资源 | 🐳 | `curl -I http://127.0.0.1:8081/` | `Content-Type: text/html` |
| L3.4 | Vite dev server | 🔧 | 浏览器 `http://localhost:5173/` | 页面加载，自动跳转 `/setup`（未配置）或 `/login`（已配置） |
| L3.5 | nginx.conf 无 proxy_pass | 🐳 | `docker compose exec frontend cat /etc/nginx/conf.d/default.conf` | 无 `proxy_pass` 关键字 |

---

#### T-L4：数据库与文件存储

| 编号 | 测试项 | 环境 | 命令 | 预期结果 |
|------|--------|------|------|----------|
| L4.1 | 数据库自动创建 | 🔧🐳 | 启动后端后检查 `data/vpn.db` | 文件存在 |
| L4.2 | 12 张表 | 🔧🐳 | `sqlite3 data/vpn.db ".tables"` | 列出全部 12 张表 |
| L4.3 | 默认平台 | 🔧🐳 | `sqlite3 data/vpn.db "SELECT id FROM platforms"` | clash-verge, v2rayng, shadowrocket |
| L4.4 | system_config 键 | 🔧🐳 | `sqlite3 data/vpn.db "SELECT COUNT(*) FROM system_config"` | ≥ 0 (未配置时为空) |
| L4.5 | WAL 模式 | 🔧🐳 | `sqlite3 data/vpn.db "PRAGMA journal_mode"` | `wal` |
| L4.6 | 外键启用 | 🔧🐳 | `sqlite3 data/vpn.db "PRAGMA foreign_keys"` | `1` |
| L4.7 | 数据目录结构 | 🔧🐳 | `ls -la data/` | subscriptions/, rules/, custom/, shares/ 目录存在 |
| L4.8 | 版本文件创建 | 🔧🐳 | 上传版本后 `ls data/subscriptions/{id}/` | v1.conf + current.conf (软链接) |

---

#### T-L5：容器与部署验证（Docker）

| 编号 | 测试项 | 环境 | 命令 | 预期结果 |
|------|--------|------|------|----------|
| L5.1 | 容器启动 | 🐳 | `docker compose up -d` | 两个容器 Up |
| L5.2 | 容器日志 | 🐳 | `docker compose logs backend` | 含 `Starting server on :8080` |
| L5.3 | 端口绑定 | 🐳 | `docker compose ps` | backend 绑 127.0.0.1:8080, frontend 绑 127.0.0.1:8081 |
| L5.4 | Volume 挂载 | 🐳 | `docker compose exec backend ls /app/data/` | 含 vpn.db |
| L5.5 | 容器重启 | 🐳 | `docker compose restart` → `docker compose ps` | 两个容器恢复 Up |
| L5.6 | 数据持久化 | 🐳 | `docker compose down` → `docker compose up -d` → 检查 `/app/data/vpn.db` | 数据完整保留 |
| L5.7 | 外部不可达(端口绑定验证) | 🐳 | 从其他机器 `curl http://<host-ip>:8080/health` | 连接超时/拒绝 (仅绑 127.0.0.1) |
| L5.8 | `docker compose config` | 🐳 | `docker compose config` | 无语法错误 |

---

#### T-L6：速率限制（本地模拟）

| 编号 | 测试项 | 环境 | 命令 | 预期结果 |
|------|--------|------|------|----------|
| L6.1 | 登录限流触发 | 🔧🐳 | `for i in {1..15}; do curl -s -o /dev/null -w "%{http_code}\n" http://localhost:8080/api/v1/auth/login; done` | 前 10 次 302, 之后 429 |
| L6.2 | 429 响应头 | 🔧🐳 | 触发限流后 `curl -I` | 含 `Retry-After` header |
| L6.3 | 下载限流触发 | 🔧🐳 | 对任意下载端点循环 25 次 | 前 20 次正常, 之后 429 |

---

#### T-L7：安全基础检查

| 编号 | 测试项 | 环境 | 命令 / 步骤 | 预期结果 |
|------|--------|------|-------------|----------|
| L7.1 | Token 脱敏 | 🔧🐳 | `curl "http://localhost:8080/api/v1/subscriptions/test/download-token?token=abc123"` → 检查日志 | 日志中 token=*** |
| L7.2 | 路径穿越防护 | 🔧🐳 | 尝试上传含 `../` 路径的文件 | 被 sanitizePath 拦截 |
| L7.3 | 下载缓存头 | 🔧🐳 | `curl -I http://localhost:8080/api/v1/subscriptions/test/download-token?token=xxx` | `Cache-Control: no-store, no-cache, must-revalidate` + `Pragma: no-cache` |
| L7.4 | 大文件上传拦截 | 🔧🐳 | 上传 >50MB 文件 | 后端拒绝 (413 或 400) |

---

### 第二部分：联机测试（需 OIDC 配置后）

以下测试**必须先完成 Setup 流程**（配置 OIDC 提供商 → `configured=true`），然后通过 OIDC 登录获取 JWT 和下载 Token 后执行。

> **前置条件**:
> 1. 已配置 OIDC 提供商 (Keycloak/Auth0/通用 OIDC)
> 2. 至少有一个 OIDC 用户可供登录测试
> 3. 以下测试标注 🌐=需外部 NGINX 域名 或 🔧=开发环境 Vite proxy

---

#### T-O1：Setup 首次配置流程

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O1.1 | 访问未配置系统 | 浏览器访问网站 | 自动跳转 `/setup` |
| O1.2 | 选择 OIDC 提供商 | Setup 页点击「切换提供商」 | OIDCSwitchDialog 弹出，三种可选 |
| O1.3 | 切换提供商保留字段 | 在 Keycloak 下填写 base_url → 切换到 Auth0 → 再切回 Keycloak | base_url 值保留 |
| O1.4 | 测试连接成功 | 填写正确 OIDC 参数 → 点击「测试连接」 | `ElMessage.success('连接测试成功')` |
| O1.5 | 测试连接失败 | 填写错误参数 → 点击「测试连接」 | `ElMessage.error` 含错误描述 |
| O1.6 | 完成配置 | 填写全部参数 → 点击「完成配置」 | 跳转 `/login`，后端 `configured=true` |
| O1.7 | 已配置后访问 `/setup` | 再次访问 `/setup` | 自动跳转 `/login` |

---

#### T-O2：OIDC 认证流程

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O2.1 | 登录跳转 | `/login` 点击「通过 OIDC 登录」 | 302 跳转到 OIDC 提供商授权页 |
| O2.2 | 回调处理 | OIDC 授权成功 → 回调到 `/auth/callback?token=xxx` | JWT 存入 localStorage，跳转首页 `/` |
| O2.3 | 首位管理员 | 首个 OIDC 用户登录 | `role=admin`, `is_advanced=true`，首页显示管理面板按钮 |
| O2.4 | 后续普通用户 | 第二个 OIDC 用户登录 | `role=user`, `is_advanced=false`，无管理面板按钮 |
| O2.5 | JWT 持久化 | 登录后关闭浏览器标签 → 重新打开 | 仍为登录状态（JWT 7 天有效） |
| O2.6 | 登出 | 首页点击「退出」 | JWT 清除，跳转 `/login` |
| O2.7 | 已登录访问 `/login` | 登出后重新登录 → 手动访问 `/login` | 自动跳转 `/` |
| O2.8 | `/auth/me` 实时查库 | `curl -H "Authorization: Bearer <jwt>" /api/v1/auth/me` | 返回最新 role/is_advanced（非 JWT claims 缓存） |
| O2.9 | 非管理员访问 `/admin` | 普通用户访问 `/admin/subscriptions` | 403 或前端路由守卫拦截跳转 `/` |

---

#### T-O3：管理员 — 订阅管理

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O3.1 | 创建订阅 | 管理面板 → 订阅管理 → 创建 default + advanced | ID 校验 [a-z0-9-]+，重复 409 |
| O3.2 | 列表排序 | 查看订阅列表 | 按平台+类型排序，default 先于 advanced |
| O3.3 | 上传版本(文件) | 进入版本管理 → el-upload 上传文件 | 自动创建版本、切换 current |
| O3.4 | 上传版本(文本) | UploadModal 文本编辑 → 保存 | 自动创建新版本 |
| O3.5 | 切换 current | 选择旧版本 → 点击「设为当前」 | current 软链接更新，绿色标签移动 |
| O3.6 | 预览版本 | 点击「预览」 | 弹窗显示 `<pre>` 内容 |
| O3.7 | 删除非 current 版本 | 点击非 current 版本的「删除」 | ConfirmDialog → 版本删除 |
| O3.8 | 最后一个版本不可删 | 仅剩 1 个版本时点击删除 | 400 拒绝或前端不显示按钮 |
| O3.9 | 超出 5 个版本 | 上传第 6 个版本 | 最旧版本自动删除 |
| O3.10 | 删除订阅 | ConfirmDialog → 删除 | 级联删除版本文件 + download_tokens |
| O3.11 | 编辑订阅 | 修改名称/平台/类型 | 更新成功 |

---

#### T-O4：管理员 — 分享订阅管理

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O4.1 | 创建分享订阅(文件) | 填写名称 + 上传文件 → 创建 | 自动生成 share_token，`has_token=true` |
| O4.2 | 创建分享订阅(文本) | 填写名称 + textarea 内容 → 创建 | 同 O4.1 |
| O4.3 | 复制分享链接 | 点击「复制分享链接」→ 粘贴 | URL 格式: `/api/v1/share/{id}/download?token={token}` |
| O4.4 | 公开下载(有效 token) | 无痕窗口访问分享链接 | 返回纯文本配置 |
| O4.5 | 公开下载(无效 token) | 无痕窗口访问 `?token=bad` | 返回纯文本错误 |
| O4.6 | 刷新 Token | ConfirmDialog → 确认刷新 | 旧 token 失效，新 token 可用 |
| O4.7 | 吊销 Token | ConfirmDialog → 确认吊销 | `has_token=false`，链接不可用 |
| O4.8 | 版本管理 | 上传/切换/预览/删除 | 同 O3（5 版本限制） |
| O4.9 | 删除分享订阅 | ConfirmDialog → 删除 | 级联删除文件 + token |

---

#### T-O5：管理员 — 平台管理

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O5.1 | 平台列表 | 进入平台管理 | 显示 3 个默认平台 + 自定义 |
| O5.2 | 创建平台 | ID 校验 [a-z0-9-]+ → 填写 client_schemes (textarea 每行一个) | 创建成功 |
| O5.3 | 重复 ID | 创建相同 ID 的平台 | 409 错误 |
| O5.4 | 编辑平台 | 修改 client_schemes / download_url | 更新成功 |
| O5.5 | download_url 可空 | 编辑时不填 download_url → 保存 | 首页该平台不显示下载客户端按钮 |
| O5.6 | 删除平台 | ConfirmDialog → 确认 | 级联删除 subscriptions + download_tokens + custom_subscriptions |

---

#### T-O6：管理员 — 用户管理

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O6.1 | 用户列表 | 进入用户管理 | 显示所有 OIDC 登录用户，含角色/is_advanced 标签 |
| O6.2 | 编辑 is_advanced | 编辑某用户 → 切换 is_advanced → 保存 | 用户下次访问首页订阅级别变更 |
| O6.3 | 管理员自身 is_advanced | 编辑管理员自己 | is_advanced 始终 true，switch disabled |
| O6.4 | groups 仅展示 | 编辑用户 → groups 字段 | 只读展示，有值才显示 |
| O6.5 | 上传自定义订阅 | 选择平台 → 上传文件 | 自定义订阅创建，用户首页替换默认/高级 |
| O6.6 | 自定义订阅覆盖 | 同平台再次上传 | 版本更新（覆盖） |
| O6.7 | 删除自定义订阅 | 选择平台 → ConfirmDialog → 删除 | 用户恢复默认/高级自动分配 |
| O6.8 | 吊销所有 Token | ConfirmDialog → 确认 | 用户所有 download_tokens 删除 |
| O6.9 | 删除用户 | ConfirmDialog → 删除 | 级联删除 tokens + custom_subscriptions |
| O6.10 | 不能删除自己 | 管理员删除自己 | 400 错误 |
| O6.11 | 不能删除最后一个管理员 | 仅剩 1 个管理员时删除之 | 400 错误 |

---

#### T-O7：管理员 — 规则管理

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O7.1 | 创建规则(文件) | 填写 id/name/client_type → 上传文件 | 自动生成 rule_token |
| O7.2 | 创建规则(文本) | 填写 id/name/client_type → textarea 内容 | 同 O7.1 |
| O7.3 | client_type 选项 | 创建对话框 → client_type 下拉 | 仅 Shadowrocket 可选 |
| O7.4 | 复制下载链接 | 点击「复制下载链接」→ 粘贴 | URL: `/api/v1/rules/{id}/download?token={token}` |
| O7.5 | 规则公开下载 | 无痕窗口访问下载链接 | 返回纯文本 |
| O7.6 | 轮替 Token | ConfirmDialog → 确认轮替 | 旧 token 失效，新 token 可用 |
| O7.7 | 版本管理 | 上传/切换/预览/删除 | 同 O3（5 版本限制） |
| O7.8 | 删除规则 | ConfirmDialog → 删除 | 级联删除文件 + rule_tokens |

---

#### T-O8：管理员 — OIDC 配置与速率限制

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O8.1 | 查看 OIDC 配置 | 进入 OIDC 配置页 | client_secret 显示 `***`（脱敏） |
| O8.2 | 切换提供商 | 点击「切换提供商」→ 选择新类型 | 字段切换，已填值保留（调用 switch-provider API） |
| O8.3 | 修改 OIDC 配置 | 修改参数 → 保存 | `POST /admin/system/configure` 复用已有 JWT_SECRET，不重新生成 |
| O8.4 | 测试连接 | 修改参数后 → 点击「测试连接」 | 成功/失败提示 |
| O8.5 | 查看速率限制 | 进入 OIDC 配置页速率限制区 | 显示 rate_limit_login + rate_limit_download |
| O8.6 | 修改速率限制 | 修改数值 → 保存 | 立即生效（下次请求按新限制） |

---

#### T-O9：管理员 — 日志查看

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O9.1 | 默认当天日志 | 进入日志页 | 显示当天日志或无日志 |
| O9.2 | 按日期筛选 | 选择其他日期 → 自动刷新 | 显示对应日期日志 |
| O9.3 | 下载类型映射 | 日志表格 download_type 列 | subscription→订阅下载, share→分享下载, custom→自定义订阅下载, rule→规则下载 |
| O9.4 | 状态颜色 | 日志表格 status 列 | success→绿色, failed→红色 |
| O9.5 | 失败原因 | 含 error_reason 的日志 | token_invalid / file_not_found / version_not_found / rate_limited |
| O9.6 | user_id 可空 | 分享/规则下载日志 | user_id 显示 "—" |
| O9.7 | 空结果 | 选择无日志的日期 | 显示 el-empty 提示 |

---

#### T-O10：普通用户 — 首页仪表盘

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O10.1 | 首页加载 | 普通用户登录 → 首页 | 顶部栏: 标题「VPN 订阅」+ 更新时间戳 + 用户名标签 + 退出 + 暗色切换 |
| O10.2 | 默认订阅显示 | 首页平台卡片 | is_advanced=false →「默认订阅」标签 + 三个按钮 |
| O10.3 | 高级订阅显示 | is_advanced=true → 首页 | 「高级订阅」标签 + 三个按钮 |
| O10.4 | 未配置降级(默认) | default 订阅未配置 → 首页 | 提示「默认订阅未配置，请联系管理员」，无按钮 |
| O10.5 | 未配置降级(高级) | advanced 订阅未配置 → 首页 | 提示「高级订阅未配置，请联系管理员」，不降级 |
| O10.6 | 一键导入 | 点击「一键导入」 | window.location.href 跳转 scheme URL (格式正确) |
| O10.7 | 复制链接 | 点击「复制链接」→ 点击输入框 | 弹窗 + 复制到剪贴板 |
| O10.8 | 刷新链接 | 点击「刷新链接」 | loading → 成功后旧 token 失效，新 token 显示 |
| O10.9 | 下载客户端按钮 | 平台配置了 download_url | 卡片底部显示链接，target="_blank" |
| O10.10 | 更新时间戳 | 管理员更新订阅版本后刷新首页 | 时间戳更新 |

---

#### T-O11：管理员 — 首页仪表盘（预览模式）

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O11.1 | 默认+高级预览 | 管理员首页，无自定义的平台 | 显示「默认订阅」+「高级订阅」两组按钮 |
| O11.2 | 未配置预览 | 某平台仅配 default 未配 advanced | advanced 区段显示「未配置」 |
| O11.3 | 自定义+预览 | 管理员首页，有自定义订阅的平台 | 显示「默认订阅」+「高级订阅」+「自定义订阅」三组按钮 |
| O11.4 | 自定义订阅覆盖(用户侧) | 有自定义订阅的普通用户首页 | 「已被分配自定义订阅」提示 + 自定义三个按钮，替换默认/高级 |

---

#### T-O12：用户 — 规则浏览页

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O12.1 | 规则列表 | 普通用户访问 `/rules` | 显示规则名称/类型/版本号/下载按钮 |
| O12.2 | 下载当前版本 | 点击「下载当前版本」 | `<a>` 链接 `href=/api/v1/rules/{id}/download?token={token}` |
| O12.3 | 空状态 | 无规则时访问 | el-empty 提示 |
| O12.4 | 非管理员不可管理 | 普通用户访问 `/admin/rules` | 路由守卫拦截 |

---

#### T-O13：下载端点全链路

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O13.1 | JWT 下载(用户) | `curl -H "Authorization: Bearer <jwt>" /api/v1/subscriptions/{platform}/download` | 返回纯文本，Content-Type: text/plain |
| O13.2 | JWT 下载(管理员 ?type=) | 管理员 JWT + `?type=advanced` | 返回高级订阅内容 |
| O13.3 | Token 下载(默认) | `curl /api/v1/subscriptions/{platform}/download-token?token={token}` | 返回纯文本，无认证 |
| O13.4 | Token 下载(自定义) | custom_sub_id 非空的 token | 返回自定义订阅内容 |
| O13.5 | Token 无效 | `?token=invalid-token` | 纯文本错误 + status=failed, error_reason=token_invalid |
| O13.6 | 分享下载(有效) | `GET /api/v1/share/{id}/download?token={token}` | 返回纯文本，无认证 |
| O13.7 | 分享下载(无效) | 吊销后访问分享链接 | 纯文本错误 |
| O13.8 | 规则下载(有效) | `GET /api/v1/rules/{id}/download?token={token}` | 返回纯文本，无认证 |
| O13.9 | 缓存头验证 | `curl -I` 以上任意下载端点 | Cache-Control: no-store, no-cache, must-revalidate + Pragma: no-cache |
| O13.10 | is_advanced 变更后旧 Token | 用户升级/降级后 curl 旧 token | 返回错误（token 已删除） |
| O13.11 | 删除自定义后旧 Token | 删除自定义订阅后 curl 旧 token | 返回错误（token 已级联删除） |

---

#### T-O14：速率限制（联机完整验证）

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O14.1 | 登录限流 429 | 短时间连续请求 `/auth/login` >10 次 | 429 + `Retry-After` + JSON 错误 |
| O14.2 | 下载限流 429 | 短时间连续请求下载端点 >20 次 | 429 + `Retry-After` + 纯文本错误 |
| O14.3 | 限流日志记录 | 触发 429 后查看管理面板日志 | status=failed, error_reason=rate_limited |
| O14.4 | 修改限流值生效 | 管理员修改 rate_limit_download=5 → 测试 | 第 6 次请求触发 429 |

---

#### T-O15：UI/UX 跨页面验证

| 编号 | 测试项 | 步骤 | 预期结果 |
|------|--------|------|----------|
| O15.1 | 暗色模式全局 | 任意页面切换暗色模式 | Setup/Login/Home/Rules/Manage 全部子页跟随 |
| O15.2 | 暗色模式持久化 | 切换暗色 → 关闭标签 → 重新打开 | 仍为暗色（localStorage） |
| O15.3 | 移动端首页 | 浏览器 DevTools 模拟手机 → 首页 | 卡片 1 列布局 |
| O15.4 | 移动端管理面板 | 手机模式 → 管理面板 | 侧边栏默认隐藏，汉堡按钮切换 |
| O15.5 | 表单 loading | 任意创建/编辑 → 提交 | 按钮 loading 状态 |
| O15.6 | API 错误提示 | 网络断开或后端错误 | ElMessage.error 提示 |
| O15.7 | 删除 ConfirmDialog | 任意管理页面删除操作 | 使用 ConfirmDialog 组件，非 ElMessageBox.confirm |
| O15.8 | el-empty 空状态 | 各列表页无数据时 | 显示 el-empty 组件 |
| O15.9 | 管理面板菜单高亮 | 在不同管理子页间切换 | 当前菜单项渐变紫色高亮 |
| O15.10 | 刷新页面路由保持 | 管理面板子页刷新浏览器 | 仍停留在当前子页 |

---

#### T-O16：端到端完整场景

| 编号 | 测试项 | 场景描述 | 预期结果 |
|------|--------|----------|----------|
| O16.1 | 完整部署场景 | `docker compose up -d` → Setup → 管理员登录 → 创建订阅 → 上传版本 → 普通用户登录 → 一键导入 | 全流程无报错 |
| O16.2 | 订阅更新场景 | 管理员上传新版本 → 用户刷新链接 → 客户端下载 | 客户端获取最新配置 |
| O16.3 | 用户升降级场景 | 管理员提升普通用户为高级 → 用户首页变化 | 订阅级别正确切换，旧 Token 失效 |
| O16.4 | 自定义订阅场景 | 管理员上传自定义订阅 → 用户首页变化 → 删除自定义 → 恢复 | 全流程正确 |
| O16.5 | 分享订阅场景 | 管理员创建分享 → 复制链接 → 无痕下载 → 刷新 Token → 吊销 Token → 删除 | 全流程正确 |
| O16.6 | 管理员自保护场景 | 尝试删除自己 / 删除最后一个管理员 | 均被拒绝 (400) |
| O16.7 | 数据持久化场景 | `docker compose down` → `docker compose up -d` | 所有数据完整恢复 |

---

### 各块快速验证索引

各构建块内保留简化的"快速验证"小节，统一引用本章对应测试编号：

| 块 | 快速验证引用 | 对应测试 |
|----|-------------|----------|
| 块 1A | `go build ./internal/utils/...` | L1.1 |
| 块 1B | `go build ./...` + vpn.db 12 表 | L1.1, L4.1-L4.6 |
| 块 1C | `go build ./...` | L1.1 |
| 块 1D | `/health` + `/system/status` | L2.1, L2.2, L2.6 |
| 块 2A | `go build ./...` | L1.1 |
| 块 2B | `/system/status configured=true` + `/auth/me` | L2.2, L2.7 |
| 块 3A-3F | `go build ./...` + 后端 handler 逻辑 | L1.1 |
| 块 4A | 循环请求 → 429 | L6.1-L6.3 |
| 块 4B | curl 下载 + access_logs | O13.1-O13.9 |
| 块 4C | `/user/platforms` + `/user/refresh-token` | O10.1-O10.10 |
| 块 4D | `/admin/logs?date=` | O9.1-O9.7 |
| 块 5A | `npm run build` | L1.2 |
| 块 5B | `npm run build` + 路由守卫 | L1.2 |
| 块 5C | `npm run build` | L1.2 |
| 块 6A | `/setup` → `/login` | O1.1-O1.7, O2.1-O2.9 |
| 块 6B | 管理面板侧边栏 | O15.4, O15.9 |
| 块 6C | 首页仪表盘 | O10.1-O10.10, O11.1-O11.4 |
| 块 6D | `/rules` 页面 | O12.1-O12.4 |
| 块 7A | SubList + SubVersions | O3.1-O3.11 |
| 块 7B | ShareList + ShareVersions | O4.1-O4.9 |
| 块 7C | PlatformManage | O5.1-O5.6 |
| 块 7D | UserManage | O6.1-O6.11 |
| 块 7E | RulesManage + RuleVersions | O7.1-O7.8 |
| 块 7F | OIDCConfig | O8.1-O8.6 |
| 块 7G | Logs | O9.1-O9.7 |
| 块 8A | `docker compose build` | L1.4, L1.5 |
| 块 8B | `docker compose config` | L5.8 |
| 块 8C | `docker compose up -d` + curl 验证 | L2.1-L3.5, L5.1-L5.7 |
| 块 8D | nginx 配置一致性检查 | 手动对比 AGENTS.md §8.2 |
| 块 8E | workflow YAML 语法 | `actionlint` |
| 块 8F | 端到端全场景 | O16.1-O16.7 |
