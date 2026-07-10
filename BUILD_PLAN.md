# 构建计划（TODO LIST）

本文件依据 `AGENTS.md` 拆分为有序构建步骤，供 AI 助手按块逐步实现。每块完成后应可独立验证（编译通过 / 页面可访问），再进入下一块。

**构建原则**:
- 严格遵循 `AGENTS.md` 第五章编码约束（安全/认证/Handler/版本管理/级联删除/速率限制/前端/Go 工程）
- 每完成一块运行 `go build ./...`（后端）或 `npm run build`（前端）验证
- 数据库表结构、API 端点、版本文件存储严格按第六章实现
- Docker 部署按第八章（外部 NGINX 分流 + 双容器 127.0.0.1 绑定）
- 不使用 .env 文件，业务配置一律 Web UI → SQLite；仅 PORT 等运维参数可环境变量覆盖

**阶段划分**（8 块）:

| 块 | 内容 | 依赖 |
|----|------|------|
| 块 1 | 后端骨架（目录/go.mod/main/DB/middleware/utils） | 无 |
| 块 2 | OIDC 认证 + Setup 流程 | 块 1 |
| 块 3 | 后端核心业务（用户/平台/订阅/规则/自定义/分享） | 块 2 |
| 块 4 | 下载端点 + 日志 + 速率限制 | 块 3 |
| 块 5 | 前端骨架（Vite/Router/Pinia/api/useTheme/公共组件） | 块 1 |
| 块 6 | 前端核心页面（Setup/Login/Home/Manage 布局） | 块 5 + 块 2 |
| 块 7 | 前端管理页面（订阅/分享/平台/用户/规则/OIDC/日志） | 块 6 + 块 3 |
| 块 8 | Docker 化 + 联调验证 | 全部 |

---

## 块 1：后端骨架

**目标**: 搭建后端目录结构、初始化 SQLite、实现基础中间件和工具函数，main.go 能启动并连接数据库。

**任务**:

- [ ] 创建 `backend/go.mod`（module 名如 `vpn-sub`），添加依赖：
  - `github.com/gin-gonic/gin`
  - `github.com/rs/zerolog`
  - `modernc.org/sqlite`
  - `github.com/coreos/go-oidc/v3`
  - `github.com/golang-jwt/jwt/v5`
  - 运行 `go mod tidy`
- [ ] 按 6.2 创建目录结构：`cmd/server/`、`internal/{auth,handler,service,repository,middleware,models,router,utils}/`
- [ ] `internal/utils/env.go`：读取环境变量（PORT 默认 8080）
- [ ] `internal/utils/crypto.go`：AES-256-GCM 加密/解密（key 取 JWT_SECRET 前 32 字节）
- [ ] `internal/utils/sanitizePath.go`：路径穿越防护
- [ ] `internal/utils/isValidID.go`：ID 格式校验 `[a-z0-9-]+`（必须在 utils 包，不能在 handler）
- [ ] `internal/models/types.go`：定义所有结构体（User, Platform, Subscription, Rule, Version, DownloadToken, CustomSubscription, ShareSubscription, ShareToken, RuleToken, AccessLog, OIDCState, SystemConfig）
- [ ] `internal/repository/db.go`：初始化 SQLite，创建 12 张表（按 6.3 表清单），开启 WAL 模式，自动创建 3 个默认平台（clash-verge/v2rayng/shadowrocket）
- [ ] `internal/repository/`：每张表一个 repo 文件（system_config, user, platform, subscription, rules, download_token, custom_subscription, share_subscription, share_token, rule_token, access_log, oidc_state）
- [ ] `internal/middleware/`：Logger（zerolog，?token= 脱敏为 ***）、Recovery、CORS、CacheControl、NoCacheForDownloads、AuthRequired（实时查库）、AdminRequired、RateLimit（预留，块 4 实现）
- [ ] `internal/router/router.go`：Setup 模式路由 + Normal 模式路由（依据 system_config.configured 切换），先注册 `/health` 和 `/system/status`
- [ ] `cmd/server/main.go`：入口，读 PORT，初始化 DB，配置 `SetTrustedProxies(["127.0.0.1"])`，启动 Gin
- [ ] 验证：`go build ./...` 通过，启动后 `GET /health` 返回 200，`GET /api/v1/system/status` 返回 `{ configured: false }`

**关键约束**:
- SQLite 路径 `/app/data/vpn.db`（开发环境用相对路径 `./data/vpn.db`）
- 12 张表严格按 6.3 表清单字段
- versions 字段为 JSON 数组，版本对象 schema：`{ version: int, file_path: string, created_at: datetime, updated_at: datetime }`

---

## 块 2：OIDC 认证 + Setup 流程

**目标**: 实现 OIDC PKCE 登录、JWT 签发验证、Setup 首次配置流程。

**任务**:

- [ ] `internal/auth/oidc_service.go`：
  - 支持 Keycloak / Auth0 / 通用 OIDC 三种 provider_type
  - PKCE 流程：生成 code_verifier + state，存入 oidc_state 表（10min TTL）
  - state 通过 HttpOnly Cookie 下发，回调时三重校验（Cookie == query == DB）
  - 回调后按 state 查表取 code_verifier 用于 token exchange，用后立即删 state 记录（防重放）
  - JWT 签发：claims 仅存 `user_id` + exp/iat，有效期 7 天，用 JWT_SECRET 签名
  - JWT 验证：Authorization: Bearer header
- [ ] Setup 相关 handler/service：
  - `POST /api/v1/admin/system/configure`：接收 OIDC 配置，Client Secret 用 AES-256-GCM 加密存储（各提供商独立字段），随机生成 ≥32 字节的 JWT_SECRET（同时用于 JWT 签名和 AES-256-GCM 加密，取前 32 字节做 key），置 configured=true（不写 admin_initialized）
  - `POST /api/v1/admin/test-oidc`：测试 OIDC 连接
  - `POST /api/v1/admin/system/switch-provider`：切换提供商类型，保留已填字段
- [ ] 认证 handler：
  - `GET /api/v1/auth/login`：跳转 OIDC 提供商
  - `GET /api/v1/auth/callback`：code exchange 后 302 到前端中转页 `/auth/callback?token=xxx`
  - `GET /api/v1/auth/me`：返回当前用户信息（查库，不用 JWT claims）
- [ ] 首位管理员判定：登录时检查 system_config.admin_initialized，若 false 则该用户 role=admin、is_advanced=true，写入 admin_initialized=true
- [ ] OIDC state 定时清理：后台 goroutine 清理过期记录
- [ ] 验证：`go build ./...` 通过；本地配置一个测试 OIDC（或 mock），完成登录流程拿到 JWT

**关键约束**:
- Setup 完成时只置 configured=true，admin_initialized 仍为 false
- OIDC 配置键：provider_type + keycloak_base_url/realm、auth0_domain、generic_issuer、client_id、各提供商独立 client_secret_encrypted、redirect_uri、frontend_url
- 后端定时清理过期 oidc_state 记录

---

## 块 3：后端核心业务

**目标**: 实现用户/平台/订阅/规则/自定义订阅/分享订阅的 CRUD + 版本管理。

**任务**:

- [ ] 平台管理（`/admin/platforms/*`）：CRUD，client_schemes JSON 数组，download_url 可空
- [ ] 用户管理（`/admin/users/*`）：
  - 列表、编辑 is_advanced（管理员强制 true，禁改自己 role）
  - 管理员自我保护：禁删自己（c.GetUserID == :id 拒绝）、禁删最后一个管理员（role=admin 数量 ≥ 1）、禁改自己 role
  - 吊销用户所有下载 Token
  - 删除用户（级联删 download_tokens、custom_subscriptions 及版本文件）
- [ ] 订阅管理（`/admin/subscriptions/*`）：
  - CRUD，UNIQUE(platform, type)，type=default/advanced
  - 版本管理：`POST /versions`（支持 multipart 文件上传 + JSON 文本 body 两种 Content-Type）、`PUT /versions/:versionId/current`、`DELETE /versions/:versionId`
  - 版本号 nextVersion = max(versions)+1，事务内计算 + 行级锁
  - 最多 5 个版本，超出删最旧，不可删最后一个
  - current 软链接原子切换（current.new → rename）
  - 文件存储 `data/subscriptions/{id}/v1.conf ... + current.conf`
- [ ] 规则管理（`/admin/rules/*`）：结构同订阅，client_type 预留，文件存储 `data/rules/{id}/`
- [ ] 自定义订阅（`/admin/users/:id/custom-subscription/*`）：
  - 上传需指定平台，每用户每平台最多一份
  - 版本管理同订阅，文件存储 `data/custom/{user_id}/{platform}/`
  - `POST /refresh-token?platform=xxx` 刷新该平台自定义订阅 Token
  - 删除自定义订阅 → 级联删 custom_sub_id 指向的 Token
- [ ] 分享订阅（`/admin/share/*`）：
  - CRUD + 版本管理（同订阅结构），文件存储 `data/shares/{id}/`
  - 创建时自动生成 share_token
  - `POST /:id/refresh-token` 刷新 Token
  - `DELETE /:id/token` 吊销 Token（链接不可用但文件保留）
  - 删除分享订阅 → 级联删 share_tokens + 版本文件
- [ ] 规则 Token 轮替：`POST /admin/rules/:id/refresh-token`
- [ ] 速率限制配置：`GET/PUT /admin/system/rate-limit`（rate_limit_login 默认 10/min、rate_limit_download 默认 20/min）
- [ ] OIDC 配置查看：`GET /admin/oidc-config`（Client Secret 脱敏回显）
- [ ] 验证：`go build ./...` 通过；用 curl 测试各端点 CRUD + 版本上传/切换/删除

**关键约束**:
- 所有 /admin/* 必须有 AdminRequired 中间件
- ID 格式校验 [a-z0-9-]+，重复返回 409
- 错误码：400/401/403/409/429/500
- 响应格式：列表 `gin.H{"key": [...]}`，成功 `gin.H{"success": true}`，错误 `gin.H{"error": "..."}`
- 文件上传统一 50MB 限制，后端也校验

---

## 块 4：下载端点 + 日志 + 速率限制

**目标**: 实现四种下载途径、访问日志记录、速率限制中间件。

**任务**:

- [ ] 订阅下载端点（速率限制）：
  - `GET /subscriptions/:platform/download`（JWT，管理员可用 ?type= 切换）
  - `GET /subscriptions/:platform/download/preview`（浏览器预览）
  - `GET /subscriptions/:platform/download-token?token=`（Token 下载，客户端用）
  - Token 下载逻辑：custom_sub_id 非空返回自定义订阅内容；为空返回 platform+type 对应默认/高级订阅
- [ ] 分享订阅下载：`GET /share/:id/download?token=`（验证 share_tokens，无认证）
- [ ] 规则下载：`GET /rules/:id/download?token=`（验证 rule_tokens，无认证）
- [ ] 自定义订阅下载：复用 `/subscriptions/:platform/download-token?token=`（custom_sub_id 非空时返回自定义内容）
- [ ] 所有下载统一行为：
  - `Content-Type: text/plain; charset=utf-8`
  - 不使用 Content-Disposition: attachment
  - `Cache-Control: no-store, no-cache, must-revalidate` + `Pragma: no-cache`
  - 返回 current 软链接指向的最新版本
- [ ] 用户端点：
  - `GET /user/platforms`：返回平台列表 + 当前用户 download_token + 是否有自定义订阅标记（首页用）
  - `GET /user/update-time`：所有订阅 current 版本 updated_at 最大值
  - `POST /user/refresh-token`：请求体 `{ platform, type }`，轮替 Token。当用户在该平台有自定义订阅时，自动刷新自定义订阅 Token（通过 custom_sub_id 定位）
- [ ] Download Token 生成逻辑：
  - 用户首次访问首页时按 user+platform+type 复用生成
  - custom_sub_id 非空时 type 置 NULL，复用唯一键 user+platform+custom_sub_id
  - is_advanced 变更时自动删除该用户所有旧 Token
- [ ] RateLimit 中间件实现：
  - 登录端点同 IP 每分钟 10 次，下载端点同 IP 每分钟 20 次（配置可改）
  - 超限返回 429 + Retry-After header
  - 登录端点附 JSON 错误，下载端点返回纯文本错误
  - 使用 c.ClientIP()（已配置 SetTrustedProxies）
- [ ] logAccess()：所有下载端点调用，记录 user_id(可空)/ip/download_type/platform(可空)/share_subscription_id(可空)/rule_id(可空)/status(success/failed)/error_reason(可空)/created_at
- [ ] 日志查询：`GET /admin/logs`（按日期筛选）
- [ ] 日志自动清理：后台 goroutine 清理 90 天以上记录
- [ ] 验证：`go build ./...` 通过；curl 测试下载流程，检查日志记录

**关键约束**:
- 所有 /admin/* 必须有 AdminRequired 中间件（含 /admin/logs）
- 错误码：400/401/403/409/429/500
- ?token= 查询参数值在 Logger 中脱敏为 ***
- 下载失败时 status=failed + error_reason（token_invalid/file_not_found/version_not_found/rate_limited）

---

## 块 5：前端骨架

**目标**: 搭建 Vue3 + Vite + Element Plus + Pinia + Router 工程，实现基础设施。

**任务**:

- [ ] 创建 `frontend/` Vite 工程，安装依赖：vue, vue-router, pinia, element-plus, axios
- [ ] `vite.config.js`：配置 proxy `/api` → `http://localhost:8080`
- [ ] `src/services/api.js`：Axios 封装，baseURL `/api/v1`，401 拦截自动登出，分组 API（auth/user/admin/download）
- [ ] `src/stores/user.js`：Pinia 用户状态（user_id/role/is_advanced/JWT），login/logout 方法
- [ ] `src/composables/useTheme.js`：暗色模式（document.documentElement.classList + localStorage + Element Plus 暗色变量）
- [ ] `src/router/index.js`：路由定义 + beforeEach 守卫（Setup 检测 + 登录恢复 + Admin 校验）
- [ ] `src/components/ConfirmDialog.vue`：通用确认对话框（标题/提示/回调）
- [ ] `src/components/OIDCSwitchDialog.vue`：OIDC 提供商切换对话框
- [ ] `src/components/UploadModal.vue`：文件上传组件（50MB 限制）
- [ ] `src/App.vue`：根组件，集成 useTheme
- [ ] 验证：`npm run build` 通过；dev server 启动，访问 / 显示空白页（路由守卫跳转 /setup）

**关键约束**:
- Vue 模板属性中不可用双引号转义 \"，用「」或计算属性
- v-model 中不可用可选链 ?，用 v-if 守卫
- 文件上传必须手动设置 Content-Type: multipart/form-data
- 前端代码统一用相对路径 `/api/v1/...`，不硬编码 host:port

---

## 块 6：前端核心页面

**目标**: 实现 Setup/Login/Home/Manage 布局，跑通主流程。

**任务**:

- [ ] `src/views/Setup.vue`：
  - 选择 OIDC 提供商类型（Keycloak/Auth0/通用）
  - 填写参数（按 provider_type 显示对应字段，切换时保留已填值）
  - 填写回调地址和前端地址
  - 测试连接按钮
  - 完成配置 → 跳转登录
- [ ] `src/views/Login.vue`：点击登录 → 调 `/auth/login` → OIDC 跳转
- [ ] `src/views/auth/callback`（中转页路由，非独立文件）：提取 URL 的 token 存 localStorage，replaceState 清空 URL，跳转首页
  - 注意：此路由对应 `/auth/callback`，与后端 API `/api/v1/auth/callback` 不同
- [ ] `src/views/Home.vue`：
  - 顶部栏：标题+更新时间戳、管理面板按钮（仅管理员）、用户名+角色标签、退出、暗色切换
  - 平台卡片网格（响应式 3/2/1 列）
  - 每个卡片按 is_advanced + 自定义订阅情况显示对应订阅区段（4.2 全部规则，含未配置降级）
  - 三个按钮：一键导入（scheme URL 拼接）、复制链接（对话框）、刷新链接
  - 下载客户端按钮（download_url 非空时显示）
- [ ] `src/views/Manage.vue`：布局，左侧 el-menu 侧边栏（200px，router 模式，7 个菜单项），移动端汉堡切换
- [ ] `src/views/Rules.vue`：用户规则页面（所有登录用户可见），展示规则列表 + 下载
- [ ] 验证：`npm run build` 通过；本地配 OIDC 跑通 Setup → Login → Home 主流程

**关键约束**:
- 一键导入 URL 格式：`scheme://path?url={encodedURL}`，如 `clash://install-config?url=https%3A%2F%2F...`
- 删除确认必须用 ConfirmDialog.vue，不用 ElMessageBox.confirm
- 登出必须调用 userStore.logout(router)，传入 router 实例

---

## 块 7：前端管理页面

**目标**: 实现管理后台 7 个功能页面。

**任务**:

- [ ] `src/views/SubList.vue`：订阅列表（按平台分组，再按类型），创建对话框
- [ ] `src/views/SubVersions.vue`：版本管理（列表+上传+文本编辑+切换+删除+预览），current 高亮
- [ ] `src/views/ShareList.vue`：分享订阅列表（名称/创建时间/当前版本/Token 状态），操作按钮（版本管理/复制/刷新/吊销/删除）
- [ ] `src/views/ShareVersions.vue`：分享订阅版本管理（同 SubVersions）
- [ ] `src/views/PlatformManage.vue`：平台 CRUD，client_schemes 编辑，download_url 设置
- [ ] `src/views/UserManage.vue`：
  - 用户列表（用户名/邮箱/角色/is_advanced/操作）
  - 编辑（is_advanced 切换；groups 仅存储不编辑，未设置不显示）
  - 上传自定义订阅（选平台+文件）
  - 删除自定义订阅（仅有时显示）
  - 吊销所有下载 Token
  - 删除用户（管理员自我保护提示）
- [ ] `src/views/RulesManage.vue`：规则列表（名称/client_type/当前版本/Token 状态），操作按钮（版本管理/复制链接/轮替Token/删除），创建对话框
- [ ] `src/views/RuleVersions.vue`：规则版本管理（同 SubVersions）
- [ ] `src/views/OIDCConfig.vue`：查看/修改 OIDC 配置，测试连接，切换提供商（保留字段），Client Secret 脱敏
- [ ] `src/views/Logs.vue`：按日期筛选日志，显示下载类型/状态/错误原因
- [ ] 验证：`npm run build` 通过；本地跑通所有管理页面 CRUD

**关键约束**:
- 所有创建/编辑用 el-dialog + el-form
- 版本上传 el-upload 50MB 限制
- 当前激活版本绿色高亮
- 4.6/4.7 操作按钮组按文档完整实现

---

## 块 8：Docker 化 + 联调验证

**目标**: 编写 Dockerfile，配置 docker-compose，端到端联调。

**任务**:

- [ ] `backend/Dockerfile`：多阶段构建（golang 编译 → distroless 运行）
- [ ] `frontend/Dockerfile`：多阶段构建（node 构建 → nginx 静态文件服务）
- [ ] `frontend/nginx.conf`：只服务静态文件 + SPA 回退（`try_files $uri $uri/ /index.html`），无任何 proxy_pass
- [ ] `docker-compose.yml`（按 8.3）：
  - backend: `127.0.0.1:8080:8080`，挂载 vpn-data:/app/data
  - frontend: `127.0.0.1:8081:80`，depends_on backend
- [ ] 更新根目录 `docker-compose.yml`（当前已有的需修正端口绑定和 volume）
- [ ] 端到端联调：
  - docker compose up -d 启动
  - 外部 NGINX 配置 `/api/` → 127.0.0.1:8080，`/` → 127.0.0.1:8081（参考 8.2）
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
- 单一 vpn-data volume 挂载 /app/data

---

## 验收标准

完成后应满足：
1. `go build ./...` 和 `npm run build` 均通过
2. docker compose up -d 启动正常
3. 上述端到端验证项全部通过
4. 代码严格遵守 AGENTS.md 第五章所有编码约束
5. 数据库 12 张表、API 端点、版本文件存储严格按第六章实现
