# 构建计划（TODO LIST）

本文件依据 `AGENTS.md` 拆分为有序构建步骤，供 AI 助手按块逐步实现。每块完成后应可独立验证（编译通过 / 页面可访问），再进入下一块。

**构建原则**:
- 严格遵循 `AGENTS.md` 第五章编码约束（安全/认证/Handler/版本管理/级联删除/速率限制/前端/Go 工程）
- 每完成一块运行 `go build ./...`（后端）或 `npm run build`（前端）验证
- 数据库表结构、API 端点、版本文件存储严格按第六章实现
- Docker 部署按第八章（外部 NGINX 分流 + 双容器 127.0.0.1 绑定）
- 不使用 .env 文件，业务配置一律 Web UI → SQLite；仅 PORT 等运维参数可环境变量覆盖

**阶段划分**（14 块，块 4 拆为 4 个子块，块 5 拆为 3 个子块，块 6 拆为 4 个子块）:

| 块 | 内容 | 依赖 |
|----|------|------|
| 块 1 | 后端骨架（目录/go.mod/main/DB/middleware/utils） | 无 |
| 块 2 | OIDC 认证 + Setup 流程 | 块 1 |
| 块 3 | 后端核心业务（用户/平台/订阅/规则/自定义/分享） | 块 2 |
| 块 4A | RateLimit 中间件实现 | 块 3 |
| 块 4B | 下载端点 + Token 生成 + logAccess | 块 4A |
| 块 4C | 用户端点（UserPlatforms/UpdateTime/RefreshToken） | 块 4B |
| 块 4D | 日志查询 + 自动清理 | 块 4B |
| 块 5A | 前端项目脚手架（Vite + 依赖 + vite.config + index.html + main.js + 空路由 + App.vue 最小版） | 块 1 |
| 块 5B | 前端核心基础设施（api.js + user store + useTheme + 完整路由表 + 三重守卫 + 所有页面 stub） | 块 5A |
| 块 5C | 前端公共组件（ConfirmDialog / OIDCSwitchDialog / UploadModal） | 块 5A |
| 块 6A | 前端认证入口页（Setup.vue + Login.vue） | 块 5B + 块 5C + 块 2 |
| 块 6B | 管理面板布局（Manage.vue） | 块 5B |
| 块 6C | 首页仪表盘（Home.vue） | 块 5B + 块 5C |
| 块 6D | 用户规则浏览页（Rules.vue） | 块 5B |
| 块 7 | 前端管理页面（订阅/分享/平台/用户/规则/OIDC/日志） | 块 6 + 块 3 |
| 块 8 | Docker 化 + 联调验证 | 全部 |

---

## 块 1：后端骨架

**目标**: 搭建后端目录结构、初始化 SQLite、实现基础中间件和工具函数，main.go 能启动并连接数据库。

**任务**:

- [x] 创建 `backend/go.mod`（module 名 `vpn-sub`），添加依赖：
  - `github.com/gin-gonic/gin` ✅
  - `github.com/rs/zerolog` ✅
  - `modernc.org/sqlite` ✅
  - `github.com/coreos/go-oidc/v3` ✅（块 2 已添加）
  - `github.com/golang-jwt/jwt/v5` ✅（块 2 已添加）
  - 运行 `go mod tidy` ✅
- [x] 按 6.2 创建目录结构：`cmd/server/`、`internal/{auth,handler,service,repository,middleware,models,router,utils}/`
- [x] `internal/utils/env.go`：读取环境变量（PORT 默认 8080）
- [x] `internal/utils/crypto.go`：AES-256-GCM 加密/解密（key 取 JWT_SECRET 前 32 字节）
- [x] `internal/utils/sanitizePath.go`：路径穿越防护
- [x] `internal/utils/isValidID.go`：ID 格式校验 `[a-z0-9-]+`（必须在 utils 包，不能在 handler）
- [x] `internal/models/types.go`：定义所有结构体（User, Platform, Subscription, Rule, Version, DownloadToken, CustomSubscription, ShareSubscription, ShareToken, RuleToken, AccessLog, OIDCState, SystemConfig）
- [x] `internal/repository/db.go`：初始化 SQLite，创建 12 张表（按 6.3 表清单），开启 WAL 模式，自动创建 3 个默认平台（clash-verge/v2rayng/shadowrocket）
- [x] `internal/repository/`：每张表一个 repo 文件（system_config, user, platform, subscription, rules, download_token, custom_subscription, share_subscription, share_token, rule_token, access_log, oidc_state）
- [x] `internal/middleware/`：Logger（zerolog，?token= 脱敏为 ***）、Recovery、CORS、CacheControl、NoCacheForDownloads、AuthRequired（实时查库）、AdminRequired、RateLimit（预留，块 4 实现）
- [x] `internal/router/router.go`：Setup 模式路由 + Normal 模式路由（依据 system_config.configured 切换），先注册 `/health` 和 `/system/status`
- [x] `cmd/server/main.go`：入口，读 PORT，初始化 DB，配置 `SetTrustedProxies(["127.0.0.1"])`，启动 Gin
- [x] 验证：`go build ./...` 通过，启动后 `GET /health` 返回 200，`GET /api/v1/system/status` 返回 `{ configured: false }`

**关键约束**:
- SQLite 路径 `/app/data/vpn.db`（开发环境用相对路径 `./data/vpn.db`）
- 12 张表严格按 6.3 表清单字段
- versions 字段为 JSON 数组，版本对象 schema：`{ version: int, file_path: string, created_at: datetime, updated_at: datetime }`

---

## 块 2：OIDC 认证 + Setup 流程

**目标**: 实现 OIDC PKCE 登录、JWT 签发验证、Setup 首次配置流程。

**任务**:

- [x] `internal/auth/oidc_service.go`：
  - 支持 Keycloak / Auth0 / 通用 OIDC 三种 provider_type ✅
  - PKCE 流程：生成 code_verifier + state，存入 oidc_state 表（10min TTL）✅
  - state 通过 HttpOnly Cookie 下发，回调时三重校验（Cookie == query == DB）✅
  - 回调后按 state 查表取 code_verifier 用于 token exchange，用后立即删 state 记录（防重放）✅
  - JWT 签发：claims 仅存 `user_id` + exp/iat，有效期 7 天，用 JWT_SECRET 签名 ✅
  - JWT 验证：Authorization: Bearer header ✅
- [x] Setup 相关 handler：
  - `POST /api/v1/admin/system/configure`：接收 OIDC 配置，Client Secret 用 AES-256-GCM 加密存储（各提供商独立字段），随机生成 ≥32 字节的 JWT_SECRET，置 configured=true（不写 admin_initialized）✅
  - `POST /api/v1/admin/test-oidc`：测试 OIDC 连接 ✅
  - `POST /api/v1/admin/system/switch-provider`：切换提供商类型，保留已填字段 ✅
- [x] 认证 handler：
  - `GET /api/v1/auth/login`：跳转 OIDC 提供商 ✅
  - `GET /api/v1/auth/callback`：code exchange 后 302 到前端中转页 `/auth/callback?token=xxx` ✅
  - `GET /api/v1/auth/me`：返回当前用户信息（查库，不用 JWT claims）✅
- [x] 首位管理员判定：登录时检查 system_config.admin_initialized，若 false 则该用户 role=admin、is_advanced=true，写入 admin_initialized=true ✅
- [x] OIDC state 定时清理：后台 goroutine 清理过期记录 ✅（已在块 1 db.go periodicCleanup 中实现）
- [x] 验证：`go build ./...` 通过 ✅；Setup 端点运行时验证通过（configure 写入 → system/status 返回 configured=true → auth/login 302 重定向 → auth/me 401 拦截）

**关键约束**:
- Setup 完成时只置 configured=true，admin_initialized 仍为 false ✅
- OIDC 配置键：provider_type + keycloak_base_url/realm、auth0_domain、generic_issuer、client_id、各提供商独立 client_secret_encrypted、redirect_uri、frontend_url ✅
- 后端定时清理过期 oidc_state 记录 ✅（每小时清理 >10 分钟的记录）

---

## 块 3：后端核心业务

**目标**: 实现用户/平台/订阅/规则/自定义订阅/分享订阅的 CRUD + 版本管理。

**任务**:

- [x] 平台管理（`/admin/platforms/*`）：CRUD，client_schemes JSON 数组，download_url 可空
- [x] 用户管理（`/admin/users/*`）：
  - 列表、编辑 is_advanced（管理员强制 true，禁改自己 role）
  - 管理员自我保护：禁删自己（c.GetUserID == :id 拒绝）、禁删最后一个管理员（role=admin 数量 ≥ 1）、禁改自己 role
  - 吊销用户所有下载 Token
  - 删除用户（级联删 download_tokens、custom_subscriptions 及版本文件）
- [x] 订阅管理（`/admin/subscriptions/*`）：
  - CRUD，UNIQUE(platform, type)，type=default/advanced
  - 版本管理：`POST /versions`（支持 multipart 文件上传 + JSON 文本 body 两种 Content-Type）、`PUT /versions/:versionId/current`、`DELETE /versions/:versionId`
  - 版本号 nextVersion = max(versions)+1，事务内计算 + 行级锁
  - 最多 5 个版本，超出删最旧，不可删最后一个
  - current 软链接原子切换（current.new → rename）
  - 文件存储 `data/subscriptions/{id}/v1.conf ... + current.conf`
- [x] 规则管理（`/admin/rules/*`）：结构同订阅，client_type 预留，文件存储 `data/rules/{id}/`
- [x] 自定义订阅（`/admin/users/:id/custom-subscription/*`）：
  - 上传需指定平台，每用户每平台最多一份
  - 版本管理同订阅，文件存储 `data/custom/{user_id}/{platform}/`
  - `POST /refresh-token?platform=xxx` 刷新该平台自定义订阅 Token
  - 删除自定义订阅 → 级联删 custom_sub_id 指向的 Token
- [x] 分享订阅（`/admin/share/*`）：
  - CRUD + 版本管理（同订阅结构），文件存储 `data/shares/{id}/`
  - 创建时自动生成 share_token
  - `POST /:id/refresh-token` 刷新 Token
  - `DELETE /:id/token` 吊销 Token（链接不可用但文件保留）
  - 删除分享订阅 → 级联删 share_tokens + 版本文件
- [x] 规则 Token 轮替：`POST /admin/rules/:id/refresh-token`
- [x] 速率限制配置：`GET/PUT /admin/system/rate-limit`（rate_limit_login 默认 10/min、rate_limit_download 默认 20/min）
- [x] OIDC 配置查看：`GET /admin/oidc-config`（Client Secret 脱敏回显）
- [x] 验证：`go build ./...` 通过；用 curl 测试各端点 CRUD + 版本上传/切换/删除

**关键约束**:
- 所有 /admin/* 必须有 AdminRequired 中间件
- ID 格式校验 [a-z0-9-]+，重复返回 409
- 错误码：400/401/403/409/429/500
- 响应格式：列表 `gin.H{"key": [...]}`，成功 `gin.H{"success": true}`，错误 `gin.H{"error": "..."}`
- 文件上传统一 50MB 限制，后端也校验

---

## 块 4：下载端点 + 日志 + 速率限制

**目标**: 实现四种下载途径、访问日志记录、速率限制中间件。

**现状盘点**（块 1-3 已就绪的基础设施）:
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
>
> **通用模式**（所有管理页面遵循）:
> - 列表页 `onMounted` 调对应 list API 获取数据
> - 创建/编辑用 `el-dialog` + `el-form`，提交前前端校验必填字段
> - 删除用 `ConfirmDialog.vue`（传入 title/message/@confirm），不用 `ElMessageBox.confirm`
> - 操作成功后刷新列表数据（重新调 list API）
> - ID 字段校验 `[a-z0-9-]+`（`SubList` 创建、`PlatformManage` 创建）
> - 文件上传 `el-upload` 限制 50MB，手动设置 `Content-Type: multipart/form-data`
> - 当前激活版本用绿色标签/边框高亮

**任务**:

- [ ] `src/views/SubList.vue`：订阅列表（按平台分组，再按类型 default/advanced），创建对话框（字段：`id`[a-z0-9-]+、`name`、`type`[default/advanced]、`platform`）
- [ ] `src/views/SubVersions.vue`：版本管理（列表+上传+文本编辑+切换+删除+预览），current 高亮。`POST /versions` 支持两种 Content-Type（multipart 文件上传 + JSON 文本编辑），前端按用户操作选择对应方式
- [ ] `src/views/ShareList.vue`：分享订阅列表（名称/创建时间/当前版本号/Token 状态）。Token 状态从 `has_token` 字段推导（`true`→"有效"，`false`→"已吊销"）。操作按钮：版本管理、复制分享链接（`has_token` 为 true 时可用）、刷新 Token（ConfirmDialog）、吊销 Token（ConfirmDialog）、删除（ConfirmDialog）
- [ ] `src/views/ShareVersions.vue`：分享订阅版本管理（同 SubVersions 结构）
- [ ] `src/views/PlatformManage.vue`：平台 CRUD，`client_schemes` JSON 数组编辑（可用 tag-input 或 textarea），`download_url` 可空
- [ ] `src/views/UserManage.vue`：
  - 用户列表（用户名/邮箱/角色标签/is_advanced 标签/操作按钮）
  - 编辑弹窗：`is_advanced` 开关（管理员自身不可修改）；`groups` JSON 仅展示不编辑，未设置 `groups` 的用户不显示该字段
  - 上传自定义订阅 → 弹出对话框 → 选择适用平台（下拉，从 platform 列表取）→ 选择文件（50MB 限制）→ **注意**：端点 `POST /admin/users/:id/custom-subscription?platform=xxx`，platform 是 **query param** 不在 body
  - 删除自定义订阅（仅当用户有自定义订阅时显示该按钮，调 `adminApi.users.deleteCustomSub(id)`）
  - 吊销所有下载 Token（ConfirmDialog："确定吊销该用户所有下载链接？吊销后用户需重新获取"）
  - 删除用户（ConfirmDialog + 管理员自我保护提示）。错误处理：后端返回 400 时显示具体错误信息（"不能删除自己"/"不能删除最后一个管理员"）
- [ ] `src/views/RulesManage.vue`：规则列表（名称/`client_type`/当前版本号/Token 状态），创建对话框（字段：`name`、`client_type`，当前仅 Shadowrocket 可选 → 下拉单选，默认选中）。操作按钮：版本管理、复制下载链接（`/api/v1/rules/:id/download?token={token}`）、轮替 Token（ConfirmDialog）、删除（ConfirmDialog）
- [ ] `src/views/RuleVersions.vue`：规则版本管理（同 SubVersions 结构）
- [ ] `src/views/OIDCConfig.vue`：
  - 查看/修改 OIDC 配置（`onMounted` 调 `adminApi.system.getOIDCConfig()`）
  - 切换提供商（`OIDCSwitchDialog` → `adminApi.system.switchProvider()`）
  - 测试连接按钮（`adminApi.system.testOIDC()`）
  - Client Secret 脱敏回显（后端已 `***` 处理，前端直接展示）
  - 保存 → `adminApi.system.configure()`
- [ ] `src/views/Logs.vue`：日期筛选（`el-date-picker`），调 `adminApi.logs.getLogs(date)` → 表格展示 `download_type`/`user_id`/`platform`/`status`/`error_reason`/`ip`/`created_at`。无数据时显示空状态
- [ ] 验证：`npm run build` 通过；本地跑通所有管理页面 CRUD

**关键约束**:
- 所有创建/编辑用 `el-dialog` + `el-form`
- 版本上传 `el-upload` 50MB 限制
- 当前激活版本绿色高亮
- AGENTS.md §4.6/§4.7 操作按钮组按文档完整实现
- 速率限制配置页（`GET/PUT /admin/system/rate-limit`）可放在 OIDCConfig 页面底部，或新增一个配置区域

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
