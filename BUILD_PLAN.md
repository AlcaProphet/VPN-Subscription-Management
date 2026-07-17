# 构建计划（TODO LIST）

本文件依据 `AGENTS.md` 拆分为有序构建步骤，供 AI 助手按块逐步实现。每块完成后应可独立验证（编译通过 / 页面可访问），再进入下一块。

**构建原则**:
- 严格遵循 `AGENTS.md` 第五章编码约束（安全/认证/Handler/版本管理/级联删除/速率限制/前端/Go 工程）
- 每完成一块运行 `go build ./...`（后端）或 `npm run build`（前端）验证
- 数据库表结构、API 端点、版本文件存储严格按第六章实现
- Docker 部署按第八章（外部 NGINX 分流 + 双容器 127.0.0.1 绑定）
- 不使用 .env 文件，业务配置一律 Web UI → SQLite；仅 PORT 等运维参数可环境变量覆盖

**阶段划分**（共 31 块，块 1 拆为 4 个子块，块 2 拆为 2 个子块，块 3 拆为 6 个子块，块 4 拆为 4 个子块，块 5 拆为 3 个子块，块 6 拆为 4 个子块，块 7 拆为 7 个子块）:

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
| 块 8 | Docker 化 + 联调验证 | 全部 | ⬜ |

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

**目标**: 编写 Dockerfile，验证 docker-compose，端到端联调。

> **注意**: 根目录 `docker-compose.yml` 已存在且端口绑定（`127.0.0.1:8080:8080` / `127.0.0.1:8081:80`）和 volume（`vpn-data:/app/data`）均正确。本块重点是编写 Dockerfile + nginx.conf + `.dockerignore`，并端到端验证。

**任务**:

- [ ] `backend/.dockerignore`：排除 `data/`、`.git/`、`__debug*`、`*.test`
- [ ] `frontend/.dockerignore`：排除 `node_modules/`、`.git/`、`dist/`
- [ ] `backend/Dockerfile`：多阶段构建
  - 阶段 1（builder）：`golang:1.25-alpine` → `COPY . .` → `CGO_ENABLED=0 go build -o server ./cmd/server`
  - 阶段 2（runtime）：`gcr.io/distroless/static-debian12:nonroot` → `COPY --from=builder /app/server /server` → `ENTRYPOINT ["/server"]`
- [ ] `frontend/Dockerfile`：多阶段构建
  - 阶段 1（builder）：`node:22-alpine` → `COPY package*.json .` → `npm ci` → `COPY . .` → `npm run build`
  - 阶段 2（runtime）：`nginx:1.27-alpine` → `COPY --from=builder /app/dist /usr/share/nginx/html` → `COPY nginx.conf /etc/nginx/conf.d/default.conf`
- [ ] `frontend/nginx.conf`（按 §8.4）：
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
  只服务静态文件 + SPA 回退，**无任何 proxy_pass**
- [ ] 验证根目录 `docker-compose.yml`（已存在）：
  - backend 端口 `127.0.0.1:8080:8080` ✅
  - frontend 端口 `127.0.0.1:8081:80` ✅
  - volume `vpn-data:/app/data` ✅
  - frontend `depends_on: backend` ✅
- [ ] 端到端联调：
  - `docker compose up -d --build` 启动
  - 外部 NGINX 配置 `/api/` → `127.0.0.1:8080`，`/` → `127.0.0.1:8081`（参考 §8.2）
  - 访问网站 → Setup → 登录 → 首页 → 管理面板全功能验证
- [ ] 验证项清单：
  - [ ] Setup 流程（OIDC 配置 → 测试连接 → 完成）
  - [ ] 首位用户自动成为管理员
  - [ ] 创建订阅（default + advanced）+ 版本上传/切换
  - [ ] 普通用户/高级用户首页显示正确订阅
  - [ ] 一键导入 URL 正确拼接
  - [ ] 下载 Token 下载返回纯文本
  - [ ] 自定义订阅上传/下载/删除
  - [ ] 分享订阅创建/刷新/吊销/删除
  - [ ] 规则管理 + 轮替 Token
  - [ ] 用户 is_advanced 变更后旧 Token 失效
  - [ ] 速率限制触发 429
  - [ ] 日志记录正确（status/error_reason）
  - [ ] 暗色模式切换
  - [ ] 移动端响应式

**关键约束**:
- 对外只暴露外部 NGINX 一个端口
- backend/frontend 端口必须 `127.0.0.1:` 前缀绑定
- frontend 容器内 nginx 不得有 proxy_pass
- 单一 `vpn-data` volume 挂载 `/app/data`

---

## 验收标准

完成后应满足：
1. `go build ./...` 和 `npm run build` 均通过
2. docker compose up -d 启动正常
3. 上述端到端验证项全部通过
4. 代码严格遵守 AGENTS.md 第五章所有编码约束
5. 数据库 12 张表、API 端点、版本文件存储严格按第六章实现
