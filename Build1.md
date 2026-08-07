# Build1.md — 工程骨架与认证闭环（当前构建方案）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第一轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则）。本系统为零基础全新构建，无历史 Build 文档。
> - 设计基线：[Design1.md](./Design1.md)（与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design1-UI.md](./Design1-UI.md)
> - 编码指令：[AGENTS.md](./AGENTS.md)（**唯一强要求**）
> - 后续轮次：Build2.md（订阅核心与用户端）、Build3.md（管理面与运维），本轮全部验收归档后再启动
>
> **里程碑：本 Build 全部 Step 完成后，系统必须能够启动、完成 Setup 首次配置、并完成本地账号与 OIDC 的注册/登录/登出闭环。**

---

## 执行约束（执行 AI 必须严格遵守）

1. **严格按 Step 顺序执行**，完成一个 Step 并验收通过后，方可进入下一个 Step。**禁止跳步、禁止并行执行多个 Step、禁止自行合并步骤**。
2. **每个 Step 完成后必须运行该 Step 的「验证命令」**，全部通过才算完成；任一命令失败必须修复后重验，禁止带错进入下一 Step。
3. **遇到模糊、歧义或设计文档未覆盖的细节，必须停止并向用户询问，禁止自行假设或自由发挥**。本文件未明确的技术选型，以 Design1.md §5.1 技术选型表为准。
4. **禁止引入设计文档未提及的框架、库或架构模式**。后端仅可使用 Design1.md §5.1 列明的技术栈（Go + Gin + SQLite 纯 Go 驱动 + 结构化日志库 + OIDC 客户端库 + JWT 库 + bcrypt + AES-256-GCM）；前端仅可使用 Vue 3 + Vite + Ant Design Vue + Tailwind CSS + Pinia + Vue Router + Axios + Vitest + Vue Test Utils。
5. **关键设计参数必须严格按下表取值**，与 Design1.md 保持一致，禁止修改：

| 参数 | 取值 | 出处 |
|------|------|------|
| 会话时长（记住我勾选） | 7 天 | Design1 §3.2 |
| 会话时长（记住我不勾选） | 24 小时 | Design1 §3.2 |
| OIDC 会话时长 | 固定 7 天（无记住我选项） | Design1 §3.2 |
| 密码复杂度 | ≥8 字符（所有本地密码入口统一） | Design1 §4.6 |
| 密码哈希算法 | bcrypt | Design1 §6.1 |
| 密码重置令牌 TTL | 1 小时，一次性，用后即删 | Design1 §4.6 |
| OIDC state TTL | 10 分钟，用后即删 | Design1 §3.2 |
| 凭据熵 | ≥128 位加密安全随机值（下载 Token/重置令牌/SSE Token 统一） | Design1 §4.2 |
| 限流默认值 | 登录 10/min、注册 5/min、找回密码 5/min（按 IP，固定窗口） | Design1 §3.4.8 |
| 运行模式 | `APP_MODE=dev\|prod`，默认 prod，启动时决定 | Design1 §1.3 |
| 数据库文件 | `app-dev.db` / `app-prod.db`（按模式分离） | Design1 §5.5 |
| 敏感配置加密 | AES-256-GCM，密钥由签名密钥派生 | Design1 §6.2 |
| 签名密钥 | Setup 时自动随机生成，明文落库 | Design1 §6.2 |
| 默认平台 | Clash Verge / v2rayNG / Shadowrocket（Setup 事务内预置） | Design1 §3.4.4 |
| 预置默认组 | Setup 完成事务内创建，不可删除 | Design1 §2.2 |
| 手填标识格式 | 小写字母数字连字符，长度 3~64 字符 | Design1 §2.2 |
| 自动生成标识 | 类型前缀 + 8 位随机短码（share-/custom-/group-/platform-） | Design1 §2.2 |

6. **注释使用中文**；所有 error 必须处理，禁止忽略返回值；禁止散落的 `fmt.Println` 调试输出，统一使用结构化日志库。
7. **服务实例一律构造注入**（结构体 Handler + 依赖传入），禁止包级全局变量持有服务实例。
8. **HTTP 处理器按业务域拆分文件**，每个文件独立管理依赖，禁止单文件集中全部处理器。
9. **职责分层**：接入层只做协议解析与响应，业务层承载规则，数据层封装 SQL 与文件读写；接入层不直接操作存储，业务层不感知 HTTP。
10. **日志必须将 `?token=` 查询参数值脱敏为 `***`**；5xx 内部错误默认脱敏返回通用信息。
11. **错误码约定**：400=校验错误，401=会话凭据缺失/无效/过期，403=权限不足，409=重复冲突，429=速率限制，500=服务器内部错误；成功操作用统一成功响应结构，列表响应用统一包裹结构。

---

## TODOLIST CheckList（构建进度核对）

> 执行 AI 必须逐个完成并勾选，便于核对构建进度。状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

- ☐ Step 1：后端工程骨架与基础中间件
- ☐ Step 2：前端工程骨架与测试基建
- ☐ Step 3：Docker 多阶段构建与 Compose 模板
- ☐ Step 4：本地认证闭环（注册/登录/会话/首管理员）
- ☐ Step 5：Setup 首次配置引导
- ☐ Step 6：OIDC 认证与模拟 OIDC
- ☐ Step 7：密码重置、验证码与速率限制

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 后端工程骨架与基础中间件 | Design1 §5.1/5.2/6.5 | ☐ 未开始 |
| 2 | 前端工程骨架与测试基建 | Design1 §5.1/3.7，UI §一/七 | ☐ 未开始 |
| 3 | Docker 多阶段构建与 Compose 模板 | Design1 §5.6/7.1 | ☐ 未开始 |
| 4 | 本地认证闭环（注册/登录/会话/首管理员） | Design1 §3.2/4.6/5.4/2.5 | ☐ 未开始 |
| 5 | Setup 首次配置引导 | Design1 §3.1/2.2/3.4.4 | ☐ 未开始 |
| 6 | OIDC 认证与模拟 OIDC | Design1 §3.2/4.6/5.3 | ☐ 未开始 |
| 7 | 密码重置、验证码与速率限制 | Design1 §3.2/4.6/6.1/3.4.8 | ☐ 未开始 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件（核心） | 要点 |
|------|----------------|------|
| 1 | `backend/go.mod`、`backend/cmd/server/main.go`、`backend/internal/{config,store,log,server,middleware}/...`、`backend/migrations/0001_init.sql` | Go 骨架 + SQLite 迁移 + 配置存储 + 日志 + 健康检查 |
| 2 | `frontend/package.json`、`frontend/vite.config.ts`、`frontend/src/{main.ts,App.vue,router/,stores/,api/,components/}`、`frontend/vitest.config.ts`、`frontend/tailwind.config.js` | Vue 骨架 + AntD/Tailwind + Vitest 基建 + 路由守卫 + Axios 拦截器 |
| 3 | `Dockerfile`、`docker-compose.yml`、`.dockerignore`、`README.md`（部署向导） | 多阶段构建 + 双接入 compose + healthcheck |
| 4 | `backend/internal/{auth,user}/...`、`backend/migrations/0002_users.sql`、`frontend/src/views/{LoginView,RegisterView}.vue`、`frontend/src/stores/auth.ts` | 本地注册/登录/会话/凭据版本号/首管理员 |
| 5 | `backend/internal/setup/...`、`backend/migrations/0003_groups_platforms.sql`、`frontend/src/views/SetupView.vue` | 未配置检测 + 快速开始 + 预置默认组/平台（预置数据由代码事务写入） |
| 6 | `backend/internal/oidc/...`、`backend/migrations/0004_oidc.sql`、`frontend/src/views/{OidcCallbackView,PendingView}.vue` | PKCE + state + 回调 + 合并/绑定 + 模拟 OIDC |
| 7 | `backend/internal/{auth/reset.go,captcha,ratelimit}/...`、`backend/migrations/0005_reset_tokens.sql`、`frontend/src/views/{ForgotView,ResetView}.vue`、`frontend/src/components/CaptchaWidget.vue` | 重置令牌 + 验证码 + 限流 |

---

## 三、构建顺序依赖图

```
Step 1（后端骨架）──┐
                    ├──▶ Step 3（Docker 化，依赖前后端骨架产出物）
Step 2（前端骨架）──┘
Step 1 ──▶ Step 4（本地认证，依赖后端骨架/迁移/配置/日志）
Step 4 ──▶ Step 5（Setup，依赖用户体系与首管理员机制；Setup 完成才产生 configured）
Step 2 ──▶ Step 4/5/6/7（各前端页面依赖前端骨架/路由/状态/拦截器）
Step 4 ──▶ Step 6（OIDC 复用会话签发与用户查建逻辑）
Step 4 ──▶ Step 7（密码重置/验证码/限流依附于本地认证端点）
```

> 线性执行序：Step 1 → Step 2 → Step 3 → Step 4 → Step 5 → Step 6 → Step 7。Step 3 虽仅依赖 1/2，但为保持线性仍按序号执行。

---

## 四、分步构建计划

---

### Step 1：后端工程骨架与基础中间件

**本 Step 完成后，系统应具备：以 Go 单二进制启动 HTTP 服务、连接 SQLite 并执行版本化迁移、提供健康检查与系统状态占位端点、输出分级结构化日志（含 token 脱敏）的能力。**

- **目标：** 搭建可编译、可运行、可测试的 Go 后端骨架，建立配置存储、数据库迁移、结构化日志与基础中间件。
- **前置条件：** 无（首个 Step）。环境要求 Go 1.25。
- **产出文件与操作：**

  1. **创建 `backend/go.mod`**：module 名必须为 `vpn-sub`，Go 版本 1.25。引入依赖：Gin、SQLite 纯 Go 驱动（`modernc.org/sqlite`，零 CGO）、结构化日志库（`log/slog` 标准库或 `zap`，二者择一并全程统一）、JWT 库、bcrypt（`golang.org/x/crypto/bcrypt`）。**禁止引入 CGO 依赖**。

  2. **创建 `backend/cmd/server/main.go`**：程序入口。必须实现：
     - 读取环境变量：`APP_MODE`（dev/prod，默认 prod）、`LOG_LEVEL`（debug/info/warn/error，默认 info）、`LOG_FORMAT`（console/json，默认 console）、`PORT`（默认 8080）、`TRUST_PROXY`（auto/on/off，默认 auto）、`RESET_ADMIN_PASSWORD`（本 Build 仅读取留存，应急逻辑在 Build3 实现）。
     - 按 `APP_MODE` 选择数据库文件：数据目录下 `app-dev.db` 或 `app-prod.db`（数据目录默认为 `./data`，可用环境变量 `DATA_DIR` 覆盖）。
     - 初始化日志、数据库连接、执行迁移、注册路由、以非阻塞优雅退出方式启动 HTTP 服务。

  3. **创建 `backend/internal/log/`**：结构化日志封装。必须实现：
     - 分级输出（debug/info/warn/error），支持 console 与 JSON 双格式（由 `LOG_FORMAT` 决定）。
     - **token 脱敏中间件/包装器**：任何日志字段或消息中出现 `?token=` 或 `&token=` 查询参数时，其值必须替换为 `***`。提供单元测试覆盖。
     - 日志输出到 stdout。

  4. **创建 `backend/internal/store/`**：SQLite 数据层封装。必须实现：
     - 打开数据库（WAL 模式：`PRAGMA journal_mode=WAL`；外键：`PRAGMA foreign_keys=ON`）。
     - **版本化迁移框架**：创建 `schema_migrations` 表（列：`version INTEGER PRIMARY KEY`、`applied_at TIMESTAMP`）；启动时按版本号顺序执行 `backend/migrations/` 下未应用的迁移；**迁移失败必须拒绝启动并输出明确错误，禁止进入半迁移状态**；**若数据库 schema 版本高于当前代码支持版本，拒绝启动并输出明确错误**（回滚边界，Design1 §7.4）。
     - 提供事务助手：显式支持以 `BEGIN IMMEDIATE` 开启事务的方法（供先读后写场景使用，Design1 §4.1）。

  5. **创建 `backend/migrations/0001_init.sql`**：初始迁移。必须创建：
     - `system_config` 表：键值配置存储。列：`key TEXT PRIMARY KEY`、`value TEXT`、`updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP`。本表承载全部系统配置（含签名密钥、认证参数、开关等，Design1 §5.3）。
     - `schema_migrations` 表（若迁移框架未自建）。

  6. **创建 `backend/internal/config/`**：系统配置服务（业务层）。必须实现：
     - 基于 `system_config` 表的键值读写：`Get(key)` / `Set(key, value)` / `GetBool` / `GetInt` 等类型化读取。
     - **敏感字段加密**：对标记为敏感的配置键（本 Build 预留，Build3 使用），以 AES-256-GCM 加密后存储；加密密钥由签名密钥派生（HKDF 或 SHA-256 派生，全程统一一种）。签名密钥的生成在 Step 5（Setup）实现，本 Step 仅提供派生与加解密函数及单元测试。
     - 配置键常量定义：本 Step 至少定义 `configured`（系统是否已配置）、`signing_key`（签名密钥）、`log_level`、`app_mode`（冗余记录当前模式）。后续 Step 按需补充键名。

  7. **创建 `backend/internal/server/`**：HTTP 服务装配（接入层）。必须实现：
     - Gin 引擎初始化；根据 `TRUST_PROXY` 配置可信代理（auto=回环+私有网段，on=全部，off=不信任）。
     - 全局中间件：请求日志（调用 log 包，输出方法/路径/状态/耗时，**必须经 token 脱敏**）、panic 恢复（返回 500 通用信息）。
     - 统一响应结构助手：成功响应 `{ "code": 0, "data": ... }`、错误响应 `{ "code": <错误码>, "message": "<可展示信息>" }`；列表响应包裹 `{ "code": 0, "data": { "list": [...], "total": N } }`。

  8. **创建健康检查与系统状态端点**（接入层，文件 `backend/internal/server/health.go` 与 `status.go`）：
     - `GET /health`：返回 200 `{ "status": "ok" }`。无需鉴权。（应急模式返回 503 的逻辑在 Build3 实现，本 Step 预留注释说明。）
     - `GET /api/system/status`：返回系统状态。字段必须包含：`configured`（bool，读 `system_config.configured`）、`app_mode`（dev/prod）、`emergency`（bool，本 Build 恒为 false，Build3 实现真实判定）。本 Step 其余字段（本地登录开关/OIDC 是否配置/注册入口可见性/验证码启用页面）在对应 Step 补充。**无需鉴权（公开端点）**。

  9. **构造注入装配**：在 `main.go` 中以构造函数注入方式装配 store / config / log / server，禁止包级全局变量持有这些实例。

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test ./...
  ```
- **验收标准：**
  - 三条命令全部通过，无编译错误、无 vet 警告、测试全绿。
  - 日志 token 脱敏、配置加解密、迁移框架（含高版本拒绝启动）均有对应单元测试且通过。
  - 手动验证：`go run ./cmd/server` 启动后，`curl http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`；`curl http://127.0.0.1:8080/api/system/status` 返回含 `configured:false`、`app_mode`、`emergency:false` 的 JSON。
  - 数据目录生成对应模式的数据库文件，且含 `system_config` 与 `schema_migrations` 表。

---

### Step 2：前端工程骨架与测试基建

**本 Step 完成后，系统应具备：Vue 3 前端可开发、可构建、可测试，具备 AntD 中文/暗色主题、Tailwind、路由守卫框架、Pinia 状态、Axios 401 拦截与统一通知封装的能力。**

- **目标：** 搭建可构建、可测试的前端骨架，建立主题、路由、状态、HTTP 与通用组件基础。
- **前置条件：** 无（与 Step 1 并行可建，但按线性序在 Step 1 后执行）。环境要求 Node.js LTS 与 npm。
- **产出文件与操作：**

  1. **初始化 `frontend/`**：Vite + Vue 3（Composition API + `<script setup>`）+ TypeScript。创建 `package.json`、`vite.config.ts`、`tsconfig.json`、`index.html`、`src/main.ts`、`src/App.vue`。

  2. **安装并配置依赖**：Ant Design Vue（按需引入）、Tailwind CSS、Pinia、Vue Router、Axios、dayjs；开发依赖：Vitest、@vue/test-utils、jsdom。**禁止引入其他 UI 框架或状态库**。

  3. **创建 `frontend/tailwind.config.js` 与 `postcss.config.js`**：配置 Tailwind，启用 `darkMode: 'class'`（配合暗色模式，Design1 §3.7）。

  4. **创建 `frontend/vite.config.ts`**：配置 `@` 指向 `src`；配置 dev server 代理 `/api` 与 `/health` 到 `http://127.0.0.1:8080`（开发联调用）；配置构建产物分包（AntD 等 vendor 分离）。

  5. **创建 `frontend/vitest.config.ts`**：配置 Vitest + jsdom 环境，建立前端单元测试基建。

  6. **创建主题与全局配置 `frontend/src/theme.ts`（或 `src/config/theme.ts`）**：必须实现：
     - AntD `ConfigProvider` 全局 `locale=zh_CN`。
     - 主色 `#1677FF`（AntD 默认科技蓝，零定制）。
     - **暗色模式**：手动切换 + localStorage 持久化（键名 `theme`），通过 `ConfigProvider theme.darkAlgorithm` 全局联动；提供 `useTheme` composable 封装切换逻辑（UI §1.1/1.2）。

  7. **创建路由 `frontend/src/router/index.ts`**：必须实现：
     - 路由表骨架：本 Build 仅落地 `/setup`、`/login`、`/register`、`/forgot`、`/reset/:token`、`/pending`、`/`（首页占位）、`/:pathMatch(.*)*`（404）。管理面板与用户端完整路由在 Build2/3 补充。
     - **路由级代码分割**：各页面 `() => import(...)` 懒加载。
     - **路由守卫框架**（UI §7.2）：实现 `emergency`（读 `/api/system/status` 的 `emergency`，为 true 强制跳 `/emergency`，本 Build 恒 false，仅留结构）、`configured`（`configured=false` 任意路径跳 `/setup`；`/setup` 在已配置时跳 `/login`）、`登录态`（无凭据访问受保护路由跳 `/login`）、`登录页跳过`（已登录访问 `/login` 跳 `/`）。守卫须调用 `/api/system/status` 获取 configured/emergency。
     - 顶部进度条（NProgress 风格，主色蓝，UI §7.6）。

  8. **创建状态 `frontend/src/stores/`**：建立 `auth.ts`（凭据存取：localStorage 键 `token`；登录/登出 action；当前用户信息）、`system.ts`（缓存 `/api/system/status` 结果：configured/app_mode/emergency 等）。

  9. **创建 HTTP 封装 `frontend/src/api/`**：必须实现：
     - Axios 实例：统一 `baseURL=/api`；请求拦截器自动携带 `Authorization: Bearer <token>`（从 auth store）。
     - **401 拦截器**：收到 401 响应时清除本地凭据并跳转 `/login`（UI §7.3）。
     - 错误码 → UI 映射封装（UI §7.3）：400 表单定位/message.error、403 message.error、409 message.error、429 message.warning（读 Retry-After）、500 通用文案。
     - `request.ts`（Axios 实例与拦截器）、`system.ts`（getSystemStatus）、`auth.ts`（本 Build 后续 Step 填充登录/注册接口）。

  10. **创建通用组件 `frontend/src/components/`**（UI §1.5，本 Build 仅建基础件）：
      - `Notify.ts`：message/notification 统一封装。
      - `ConfirmModal.vue`：删除/危险操作统一确认对话框（标题 + 影响提示 + 确认回调；支持内嵌确认词输入框，词不正确时确认按钮禁用）。**禁止使用浏览器原生 confirm**。
      - `TriStateList.vue`：加载中/空/列表三态封装（加载=Skeleton，空=Empty+引导文案，错误=message.error+重试）。
      - `PageHeader.vue`、`CopyField.vue` 可在本 Step 建立空壳或待 Build2 用到时再建（执行时以「用到即建」为准，但 ConfirmModal/TriStateList/Notify 本 Step 必须建立）。

  11. **创建 `frontend/src/views/NotFoundView.vue`**：`a-result 404` + 「返回首页」按钮。

  12. **创建 `frontend/src/layouts/`**：建立 `BlankLayout.vue`（无登录态居中布局，用于 setup/login/register/forgot/reset/pending）与占位 `HomeLayout.vue`（本 Build 仅首页占位用）。管理面板布局在 Build2/3 建立。

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm run test
  ```
- **验收标准：**
  - 两条命令全部通过（`npm run test` 执行 Vitest）。
  - 主题切换、401 拦截器、路由守卫（configured 跳转逻辑）须有对应前端单元测试且通过。
  - 手动验证：`npm run dev` 启动后访问 `/` 正常渲染；localStorage 无 token 时访问受保护路由被守卫重定向；切换暗色模式全局生效并持久化。

---

### Step 3：Docker 多阶段构建与 Compose 模板

**本 Step 完成后，系统应具备：通过 `docker compose build` 构建单镜像、`docker compose up -d` 一键启动完整应用（前端产物 + 后端 API + 静态资源 + SPA 回退）的能力。**

- **目标：** 建立多阶段 Dockerfile 与双接入方式的 compose 模板，实现单容器交付。
- **前置条件：** Step 1（后端可编译）、Step 2（前端可构建）已完成并验收。
- **产出文件与操作：**

  1. **创建 `Dockerfile`（仓库根目录）**：多阶段构建，必须包含：
     - 阶段一（前端构建）：Node 镜像，`COPY frontend/`，`npm ci && npm run build`，产出 `dist/`。
     - 阶段二（后端编译）：Go 镜像，`CGO_ENABLED=0` 静态编译 `backend/`，产出单二进制；**必须将前端 `dist/` 嵌入或拷贝至运行时镜像**（可用 `go:embed` 嵌入 dist，或运行时从磁盘提供，二者择一并全程统一）。
     - 阶段三（最小运行时）：`scratch` 或 `alpine`/`distroless` 最小镜像；**非 root 用户运行**；暴露端口 8080；声明数据卷挂载点（如 `/data`）；日志输出 stdout。
     - 后端必须同时服务：API 路由、`/assets`（前端产物，immutable 缓存）、`/public`（可缓存资源，本 Build 建目录结构，Build2 填充）、其余路径 SPA 回退到 `index.html`（Design1 §5.6）。

  2. **创建 `.dockerignore`**：排除 `node_modules`、`data`、`*.db*`、`.git` 等。

  3. **创建 `docker-compose.yml`**：单服务，必须包含：
     - 端口映射提供**两种接入方式注释模板**（Design1 §7.1）：外部反代（`127.0.0.1:8080:8080`）与局域网直连（`8080:8080`），默认启用其一并以注释说明另一种。
     - 数据卷挂载（命名卷或 bind mount 到 `/data`）。
     - 环境变量示例（注释形式）：`APP_MODE=prod`、`LOG_LEVEL=info`、`TRUST_PROXY=auto`。
     - **`RESET_ADMIN_PASSWORD` 注释示例行**（Design1 §3.8：部署者取消注释并重启即可进入应急模式；应急逻辑在 Build3 实现，本 Step 仅提供注释行与说明）。
     - `restart: unless-stopped`。
     - **healthcheck 配置**（`curl -f http://127.0.0.1:8080/health || exit 1`），仅作状态展示，不配置任何由健康检查触发的重启动作（Design1 §7.1）。

  4. **SPA 回退与静态资源服务**：在后端 `internal/server/` 补充静态资源服务与 SPA 回退逻辑（若 Step 1 未建）。必须实现：`/assets/*` 返回前端产物（`Cache-Control: immutable`）；`/public/*` 返回可缓存资源（`Cache-Control: public, max-age=...`）；API 路径之外的 GET 请求回退到 `index.html`。

  5. **创建 `README.md` 部署向导**：必须包含（Design1 §7.1）：两种接入方式的部署步骤、域名+反代+HTTPS 配置示例、局域网直连说明、**HTTP 明文风险提示**、**Setup 完成后请立即注册/登录成为管理员的抢注窗口提示**、OIDC 回调与验证码依赖公网可达的说明。

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  docker compose build
  docker compose up -d
  curl http://127.0.0.1:8080/health
  curl http://127.0.0.1:8080/api/system/status
  docker compose down
  ```
- **验收标准：**
  - `docker compose build` 成功，产出单镜像。
  - `up -d` 后 `/health` 返回 ok，`/api/system/status` 返回正常；访问 `/` 返回前端 `index.html`（SPA 回退生效）；访问 `/assets/...` 有 immutable 缓存头。
  - 容器以非 root 运行（`docker compose exec <svc> id` 验证非 uid 0）。
  - 数据卷挂载生效（`up` 后卷内生成数据库文件）。

---

### Step 4：本地认证闭环（注册/登录/会话/首管理员）

**本 Step 完成后，系统应具备：本地账号注册、邮箱+密码登录、会话凭据签发与校验（含凭据版本号失效机制）、首管理员自动产生、前端登录/注册页与登录态管理的能力。**

- **目标：** 实现本地认证全链路（后端端点 + 会话机制 + 前端页面），并落地首管理员机制。
- **前置条件：** Step 1（后端骨架/迁移/配置/日志）、Step 2（前端骨架/路由/状态/拦截器）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/0002_users.sql`**：创建 `users` 表。列必须包含（Design1 §2.7/5.3）：
     - `id`（主键，内部稳定 user_id）、`oidc_subject`（TEXT UNIQUE 可空，本 Step 建列，Step 6 使用）、`username`（TEXT，无唯一约束）、`email`（TEXT UNIQUE 可空，NULL 不冲突）、`role`（TEXT，`admin`/`user`）、`group_id`（INTEGER 可空，本 Step 建列，默认组在 Step 5 创建后回填，Build2 完整使用）、`password_hash`（TEXT 可空）、`user_source`（TEXT，`oidc`/`local`/`selfreg`）、`status`（TEXT，`pending`/`active`/`disabled`）、`credential_version`（INTEGER 默认 0）、`oidc_claims`（TEXT 可空 JSON，Step 6 使用）、`created_at`、`updated_at`。
     - 邮箱唯一约束对 NULL 不生效（SQLite 原生支持多 NULL）。

  2. **创建 `backend/internal/auth/`（业务层）**：认证服务。必须实现：
     - **密码**：bcrypt 哈希与校验；密码复杂度校验（≥8 字符，所有本地密码入口统一）。
     - **邮箱规范化**：所有写入入口统一 trim + 小写化 + 基本格式校验（拒绝控制字符，防 SMTP 头注入），唯一约束基于规范化值（Design1 §4.6）。
     - **会话凭据签发/验证**：JWT，载荷仅含 `user_id` + `credential_version` + 标准声明（iat/exp）；**角色/组等权限信息禁止入凭据**；用 `signing_key` 签名（本 Step 若 signing_key 未生成，需先在 Setup 前提供「确保签名密钥存在」的内部函数——若 `system_config.signing_key` 为空则自动生成随机密钥写入，供会话签发使用；Setup 完成时复用该密钥，不重复生成，Design1 §3.1/6.2）。
     - **会话时长**：记住我勾选=7 天，不勾选=24 小时。
     - **凭据校验中间件**：解析凭据 → 查库取用户 → 比对 `credential_version`（不符返回 401）→ 校验 `status=active`（禁用/待审批返回 401）。**每次请求实时查库，禁止缓存权限信息**。
     - **角色校验中间件**：在会话校验之上，校验 `role=admin`（本 Build 建立，Build2/3 管理端点使用）。两个中间件独立、可组合。

  3. **创建 `backend/internal/user/`（业务层）**：用户服务。必须实现：
     - 创建用户（邮箱规范化、bcrypt 密码、来源标注、状态）。
     - **首管理员机制**（Design1 §2.5，关键约束）：
       - 「用户表为空（0 行）时」首个完成认证的用户自动成为管理员并置位「已初始化」标记（存 `system_config`）。
       - **空表判定口径**：任何用户记录（含待审批）存在即算非空。
       - **原子性**：「检查空表 → 写入管理员 + 置位标记」必须在单个以 `BEGIN IMMEDIATE` 开启的事务内完成（并发串行化），并发首登录/首注册只产生一个管理员。
       - 首管理员**免审批直接激活**，不受任何审批开关影响。
     - 注册（自注册）：邮箱唯一冲突返回 409；默认状态逻辑——本 Build 自注册审批开关默认关闭，注册即 `active`（开关配置在 Build3 面板实现，本 Step 按「关闭」默认行为实现，预留读取开关的代码路径）。
     - 登录：邮箱+密码校验；**失败提示统一措辞**（不区分「邮箱不存在」与「密码错误」，防枚举）；待审批/已禁用返回通用提示「账号未激活或已被禁用」；仅 `active` 可登录。

  4. **创建认证端点（接入层，`backend/internal/server/auth.go`）**：
     - `POST /api/auth/register`：入参 `username`/`email`/`password`；校验邮箱唯一、密码复杂度；创建用户（首管理员逻辑生效）；返回成功（是否直接签发会话按「注册后直接激活可登录」处理——直接激活则签发会话返回，待审批则不签发并返回待审批提示）。
     - `POST /api/auth/login`：入参 `email`/`password`/`remember`；校验成功签发会话凭据返回（含 token 与过期时间）；失败统一 401 通用措辞。
     - `GET /api/auth/me`：会话校验，返回当前用户信息（username/email/role/group/status/user_source）。
     - `POST /api/auth/logout`：**退出为客户端语义**（Design1 §5.4：无服务端会话存储，退出仅清除本地凭据）；本端点可仅返回成功，前端清除本地 token。
     - **参数长度校验**：所有表单类端点入参做长度限制（AGENTS §八-6）。

  5. **扩展系统状态端点**（`status.go`）：补充字段 `allow_local_login`（bool，默认 true）、`allow_selfreg`（bool，默认 false）、`user_table_empty`（bool，注册入口可见性所需，**有意公开**，Design1 §5.2）。

  6. **创建前端页面**：
     - `frontend/src/views/LoginView.vue`（UI §2.2）：本地登录区块（邮箱、密码、`a-checkbox` 记住我、「忘记密码？」链接、登录按钮）；注册入口（`allow_selfreg` 开启，或 `user_table_empty` 且 `allow_local_login` 时**始终显示**）；表空时区块顶部 `a-alert info`「系统尚未配置管理员，首个注册用户将成为管理员」；失败统一措辞直接展示后端文案；暗色模式切换常驻；OIDC 区块本 Step 留占位（Step 6 填充）。已登录访问自动跳 `/`。
     - `frontend/src/views/RegisterView.vue`：用户名 + 邮箱 + 密码 + 确认密码表单；提交调注册接口；成功（直接激活）→ 存 token 跳 `/`；（待审批）→ 跳 `/pending`。
     - `frontend/src/views/PendingView.vue`：`a-result info`「账号待审批，等待管理员激活」+ 返回登录按钮（无凭据独立页）。
     - `frontend/src/api/auth.ts`：register/login/me/logout 接口封装。
     - `frontend/src/stores/auth.ts`：补充 login/register/logout action（登录成功存 token 与用户信息，登出清除）。

  7. **首页占位**：`frontend/src/views/HomeView.vue` 本 Step 仅渲染「登录成功 + 当前用户信息 + 退出按钮」占位（完整首页在 Build2）。

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：密码哈希/复杂度、邮箱规范化、会话签发/验证、凭据版本号失效（修改 credential_version 后旧凭据 401）、首管理员原子性（并发注册只产生一个 admin）、统一失败措辞。
  - 前端单测覆盖：登录/注册表单提交、401 后清除凭据跳登录。
  - 手动验证：注册首个用户 → 自动成为 admin 并登录；第二个注册用户为 user；错误密码与不存在邮箱返回相同文案；登出后再访问受保护接口 401。

---

### Step 5：Setup 首次配置引导

**本 Step 完成后，系统应具备：未配置状态自动进入 Setup 向导、快速开始一键完成配置、事务内预置默认组与 3 个默认平台、完成后跳转登录的能力。**

- **目标：** 实现 Setup 引导后端与前端向导页，完成系统初始化（签名密钥确保 + 预置数据 + configured 置位）。
- **前置条件：** Step 4（用户体系与首管理员机制）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/0003_groups_platforms.sql`**：创建预置数据所需表：
     - `groups` 表：`id`（主键）、`slug`（TEXT UNIQUE，自动生成 `group-` + 8 位随机短码）、`name`（TEXT UNIQUE）、`is_default`（INTEGER 0/1）、`needs_reselect`（INTEGER 默认 0，Build2 使用）、`created_at`、`updated_at`。
     - `platforms` 表：`id`（主键）、`slug`（TEXT UNIQUE，`platform-` + 8 位随机短码）、`name`、`description`、`schemes`（TEXT JSON 数组）、`extra_headers`（TEXT JSON 键值对）、`installer_file`（TEXT 可空）、`installer_url`（TEXT 可空）、`created_at`、`updated_at`。
     - （用户组与平台的完整业务逻辑在 Build2 实现，本 Step 仅建表与预置。）

  2. **创建 `backend/internal/setup/`（业务层）**：Setup 服务。必须实现：
     - `IsConfigured()`：读 `system_config.configured`。
     - **标识自动生成器**：`generateSlug(prefix)` = 类型前缀 + 8 位随机短码（小写字母数字，去除易混淆字符可复用密码字符集规则），冲突自动重试最多 3 次，仍冲突则报错并记录日志（Design1 §2.2）。本 Step 用于 group-/platform- 前缀。
     - `CompleteQuickStart()`（快速开始）：**单个以 `BEGIN IMMEDIATE` 开启的事务内**完成：确保签名密钥存在（复用 Step 4 的「确保签名密钥」逻辑，不重复生成）→ 创建预置默认组（`is_default=1`，名称「默认组」）→ 创建 3 个默认平台（Clash Verge / v2rayNG / Shadowrocket，各自内置客户端导入 scheme，见下）→ 置 `configured=true` → 推导并写入 `frontend_url` 初始值（见下）。**任一步失败整体回滚**。
     - **前端地址推导**（Design1 §3.1）：`TRUST_PROXY` 信任来源时优先取 `X-Forwarded-Host`，否则取 `Host` 头，推导 `frontend_url` 写入配置（手动值优先、重启后不再被推导覆盖的缓存语义在 Build3 面板完整实现，本 Step 仅写入推导初始值）。
     - **默认平台 scheme**（Design1 §3.4.4）：clash-verge 预置 `clash://install-config?url={url}`；v2rayNG 与 Shadowrocket 预置各自常用导入 scheme；clash-verge 预置三条附加响应头（`Content-Disposition: attachment; filename*=UTF-8''...yaml`、`profile-update-interval: 300`、`profile-web-page-url: {frontend_url}`），其余默认平台附加头为空。

  3. **创建 Setup 端点（接入层，`backend/internal/server/setup.go`）**：
     - `GET /api/setup/status`：返回 `configured` 与 `app_mode`（公开，供前端守卫；可与 `/api/system/status` 复用，若复用则本端点可省略，执行时以不重复为准）。
     - `POST /api/setup/quickstart`：未配置状态下可调；调用 `CompleteQuickStart()`；已配置返回 409。**本端点仅在未配置状态暴露**。
     - （OIDC 高级配置分支的 Setup 在 Step 6 补充端点；导入分支在 Build3 实现。）

  4. **创建前端 `frontend/src/views/SetupView.vue`**（UI §2.1）：必须实现：
     - 独立全屏路由，居中单列卡片（max-width 720px）；顶部站点 ICON + 「首次配置」标题 + 当前模式徽标（`a-tag`：Dev 蓝 / Production 绿）；右上角暗色模式切换。
     - `a-steps` 步骤条（认证方式 → 完成）。本 Step 仅实现「快速开始」卡片（「本地账号模式，零配置一键完成」+ 推荐 `a-tag`）；「高级配置」卡片渲染但点击提示「请先完成 OIDC 配置」（Step 6 填充）；「导入已有配置」卡片**仅 Production 模式渲染**且本 Step 置灰标注「Build3 提供」（或直接隐藏，执行时选择隐藏并在 Build3 补充）。
     - 点击「完成配置」→ 调 `quickstart` → 成功跳 `/login`。
     - **完成页/成功后显著步骤式提示「请部署者本人立即注册/登录成为管理员」（含抢注风险提示）**（Design1 §3.1）。
     - `<768` 步骤条转竖向、卡片纵向堆叠。

  5. **路由接线**：将 `/setup` 路由与 `configured` 守卫接通（Step 2 已建守卫框架，本 Step 验证：`configured=false` 访问任意路径跳 `/setup`；Setup 完成后访问 `/setup` 跳 `/login`）。

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：标识自动生成器（格式/重试/冲突）、快速开始事务（预置默认组 + 3 平台 + configured 置位 + 失败回滚）、前端地址推导。
  - 手动验证：全新数据目录启动 → 访问 `/` 自动跳 `/setup` → 快速开始完成 → 跳 `/login` → 注册首个用户成为 admin；数据库存在默认组（is_default=1）与 3 个默认平台（clash-verge 含三条附加头）。

---

### Step 6：OIDC 认证与模拟 OIDC

**本 Step 完成后，系统应具备：标准 OIDC 授权码 + PKCE 登录、state 三重校验防重放、自动合并与手动绑定、Dev 模式模拟 OIDC 登录、Setup 高级配置分支的能力。**

- **目标：** 实现 OIDC 认证全链路（含模拟 OIDC），与本地认证共用会话签发机制。
- **前置条件：** Step 4（会话签发与用户查建）、Step 5（Setup 基础）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/0004_oidc.sql`**：创建 `oidc_states` 表：`state`（TEXT PRIMARY KEY）、`code_verifier`（TEXT）、`intent`（TEXT，`login`/`bind`）、`bind_user_id`（INTEGER 可空）、`created_at`（TIMESTAMP）。10 分钟 TTL，用后即删，过期定时清理。

  2. **创建 `backend/internal/oidc/`（业务层）**：OIDC 服务。必须实现：
     - **配置管理**：OIDC 参数存 `system_config`（键：`oidc_provider_type`、`oidc_base_url`、`oidc_realm`、`oidc_client_id`、`oidc_client_secret`（敏感加密）、`oidc_configured`）；提供商类型：keycloak/auth0/generic/mock。
     - **授权发起**：生成随机 `state` 与 `code_verifier`（PKCE S256）→ 持久化到 `oidc_states`（带 intent 与可选 bind_user_id）→ state 下发浏览器 Cookie（HttpOnly，SameSite=Lax，HTTPS 下 Secure）→ 302 重定向到提供商授权页。
     - **回调处理**：三重校验（Cookie state == 回调参数 state == 存储记录）→ state 用后即删 → 用 code_verifier 换 token → 解析身份信息（subject/email/email_verified/username + role/group claims）→ 查/建用户 → 签发会话（OIDC 会话固定 7 天，无记住我）。
     - **用户查建逻辑**（Design1 §4.6，关键约束）：
       - subject 命中 → 直接登录（每次 OIDC 登录刷新 username 为提供商最新值；email 首次写入后不自动覆盖）。
       - subject 未命中但邮箱命中且 `email_verified=true`、目标账号未绑定其他 OIDC、状态为待审批/已激活 → **自动合并**：将 subject 写入该账号 `oidc_subject`（**条件更新 `WHERE oidc_subject IS NULL` 防并发覆盖**），合并即激活（OIDC 视同可信，可绕过审批），清除审批中心待审批记录关联。
       - 邮箱未验证/未返回、目标已绑定其他 OIDC、账号已禁用 → 不合并，提示冲突。
       - 均不存在 → 创建新用户（OIDC 审批开关默认关闭，直接激活；开关在 Build3 面板实现，本 Step 按「关闭」默认行为，预留读取开关路径；若开启且未命中白名单则创建待审批用户、存 claims 快照、**不签发会话、302 到 `/pending`**）。
       - **待审批用户重复 OIDC 登录**：subject 命中待审批 → 提示「已提交，等待审批」不重复创建；subject 未命中但邮箱命中待审批 → 不创建新记录，将新 subject 绑定到该待审批账号。
       - 首管理员机制对 OIDC 首个登录者同样生效（复用 Step 4 原子事务）。
     - **手动绑定**（intent=bind）：登录用户在个人中心发起 → state 记录 bind 意图与目标 user_id → 回调校验该 subject 未绑定其他账号 → 写入目标账号 `oidc_subject` → **不签发会话**，302 回前端个人中心提示绑定成功。
     - **模拟 OIDC**（仅 Dev 模式且 `oidc_provider_type=mock`）：不跳转真实提供商；提供模拟登录端点，输入邮箱（作为 subject 与默认用户名，用户名可留空取邮箱 @ 前缀）+ 可选 role/group 附加属性 + `email_verified` 勾选（默认勾选）→ 走与真实 OIDC 一致的查建/合并逻辑（subject 固定为输入邮箱，可复现合并/冲突测试）。
     - **测试连接**（Design1 §3.1）：验证发现文档可达性与配置完整性；以 client_credentials 换 token 验证 Client ID/Secret；不支持该授权类型时降级为警告不阻断；模拟模式始终通过。

  3. **创建 OIDC 端点（接入层，`backend/internal/server/oidc.go`）**：
     - `GET /api/auth/oidc/login`：发起授权（302）。
     - `GET /api/auth/oidc/callback`：回调处理（成功 302 到前端中转页 `/login/callback?token=...`；待审批 302 到 `/pending`；绑定成功 302 到 `/profile`）。
     - `POST /api/auth/oidc/mock/login`：模拟登录（仅 Dev + mock）。
     - `POST /api/auth/oidc/bind`：发起绑定（需会话）。
     - `POST /api/oidc/test`：测试连接（不落库；Setup 与面板共用，本 Build 不要求鉴权——Setup 阶段无会话；面板复用时 Build3 再加管理员校验，或本 Step 即加并在 Setup 场景放行，执行时选择「本 Step 不加鉴权，Build3 面板复用时新增管理员专用测试端点」）。
     - **OIDC 回调与当前用户端点不限流**（state 一次性 + 三重校验已防重放，Design1 §5.2）。

  4. **Setup 高级配置分支**：扩展 `backend/internal/setup/` 与 `server/setup.go`：
     - `POST /api/setup/oidc`：未配置状态下保存 OIDC 参数（含提供商类型/Base URL/Realm/Client ID/Client Secret）→ 同事务完成预置数据 + configured 置位（与快速开始同一事务语义）。
     - 回调地址 `callback_url` 与 `frontend_url` 同样推导写入初始值。
     - 切换提供商类型保留已填字段（各提供商参数独立存储）。

  5. **创建前端页面/组件**：
     - `frontend/src/views/OidcCallbackView.vue`（中转页 `/login/callback`）：从 URL 提取 token 存入 auth store → 清空 URL → 跳 `/`。（Design1 §3.2：中转页立即清空 URL。）
     - 扩展 `LoginView.vue`：OIDC 登录区块（`oidc_configured` 时渲染「使用 OIDC 登录」主按钮）；**模拟 OIDC 时替换为模拟登录表单**（邮箱 + 用户名（可留空）+ role/group 勾选输入 + `email_verified` 勾选（默认勾选），标题标注「Dev 模拟登录」）。
     - 扩展 `SetupView.vue`：「高级配置」卡片可用——选中展开提供商 `a-radio-group`（Keycloak/Auth0/通用 OIDC/模拟 OIDC，模拟仅 Dev 可选）→ 步骤 2 参数表单（按提供商动态渲染 Base URL/Realm/域名/Client ID/Client Secret，「高级」折叠面板含前端地址与回调地址预填推导值）→ 步骤 3 测试连接（结果 `a-alert`）→ 完成。
     - `frontend/src/api/oidc.ts`：相关接口封装。

  6. **扩展系统状态端点**：补充 `oidc_configured`（bool）、`oidc_provider_type`（供登录页渲染模拟表单判断）。

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  APP_MODE=dev go run ./cmd/server   # 手动验证模拟 OIDC
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：state 三重校验与用后即删、PKCE 流程、自动合并（email_verified + 条件更新防并发）、冲突分支（已绑定其他 OIDC/已禁用）、手动绑定不签发会话、模拟 OIDC 查建。
  - 手动验证（Dev + mock）：Setup 高级配置选模拟 OIDC 完成 → 登录页模拟表单输入邮箱登录成功成为用户；`email_verified` 勾选时对既有本地账号触发自动合并；未勾选时提示冲突。
  - 手动验证（真实 OIDC 如有环境）：完整授权码流程登录成功。

---

### Step 7：密码重置、验证码与速率限制

**本 Step 完成后，系统应具备：忘记/重置密码（一次性令牌）、reCAPTCHA/Turnstile 验证码防护、登录/注册/找回按 IP 限流的能力。**

- **目标：** 补全认证体系的救援与防护能力。
- **前置条件：** Step 4（本地认证）已完成并验收；Step 6（OIDC）已完成（验证码/限流需覆盖全部表单端点）。
- **产出文件与操作：**

  1. **创建 `backend/migrations/0005_reset_tokens.sql`**：创建 `password_reset_tokens` 表：`token`（TEXT PRIMARY KEY，≥128 位随机值）、`user_id`（INTEGER）、`expires_at`（TIMESTAMP）、`used`（INTEGER 0/1）、`created_at`。一次性、1 小时 TTL、用后即删。

  2. **创建 `backend/internal/auth/reset.go`（业务层）**：密码重置服务。必须实现：
     - 生成一次性重置令牌（≥128 位加密安全随机值，1 小时 TTL）。
     - 请求重置：无论邮箱是否存在均返回统一提示「若该邮箱已注册，重置链接已发送」（防枚举）；**已配置 SMTP 时发送邮件**（SMTP 配置与发送在 Build3 面板完整实现，本 Step 预留发送接口与「未配置 SMTP 时提示联系管理员」分支，邮件发送函数可先以日志记录代替，标注 Build3 接通）。
     - 校验令牌（存在 + 未过期 + 未使用）→ 设置新密码（bcrypt，≥8 字符）→ 用后即删 → **递增该用户 `credential_version`（全部现有会话立即失效）**。

  3. **创建 `backend/internal/captcha/`（业务层）**：验证码服务。必须实现：
     - 配置存 `system_config`（键：`captcha_provider`（recaptcha/turnstile/off）、`captcha_site_key`、`captcha_secret_key`（敏感加密）、`captcha_pages`（JSON 数组：register/login/forgot））。本 Step 提供配置读写；管理面板配置界面在 Build3。
     - 服务端校验：调用对应提供商验证接口校验 token。
     - **启用判定**：仅当某页面在 `captcha_pages` 且密钥已配置时，该页面端点强制校验；**运行中密钥配置缺失则跳过校验兜底并记录 warn 日志**（Design1 §3.2）。
     - 默认不启用（`captcha_provider=off`）。

  4. **创建 `backend/internal/ratelimit/`（中间件）**：速率限制。必须实现：
     - 按 IP 维度固定窗口计数（按分钟槽）；**真实客户端 IP 解析遵循 `TRUST_PROXY` 策略**（auto=回环+私有网段信任转发头，on=全信任，off=不信任）。
     - 可配置阈值（存 `system_config`：`ratelimit_login`=10、`ratelimit_register`=5、`ratelimit_forgot`=5，每分钟；修改后立即生效——每次请求读取当前配置）。
     - 超限返回 429 + `Retry-After` 头。
     - 作用于：登录、注册、找回密码端点（OIDC 回调与当前用户端点不限流）。

  5. **创建认证防护端点（接入层，`backend/internal/server/auth.go` 扩展）**：
     - `POST /api/auth/forgot`：入参 `email`；生成重置令牌并（预留）发送；统一防枚举响应。受找回限流 + 验证码（若启用 forgot 页）。
     - `POST /api/auth/reset`：入参 `token`/`password`；校验令牌重置密码；成功递增凭据版本号。
     - 登录/注册端点接入限流与验证码校验中间件。

  6. **扩展系统状态端点**：补充 `captcha_provider`、`captcha_site_key`、`captcha_pages`（供前端渲染验证码组件；**secret_key 禁止返回**）。

  7. **创建前端页面/组件**：
     - `frontend/src/views/ForgotView.vue`：邮箱输入 → 提交 → `a-result` 统一提示「若该邮箱已注册，重置链接已发送」。
     - `frontend/src/views/ResetView.vue`（`/reset/:token`）：新密码 + 确认密码 → 提交 → 成功跳 `/login`。
     - `frontend/src/components/CaptchaWidget.vue`：按 `captcha_provider`/`captcha_pages` 在对应表单提交按钮上方渲染 reCAPTCHA/Turnstile 组件；**脚本加载失败显示明确错误文案（不静默卡死）**（UI §2.2）。
     - 在 LoginView/RegisterView/ForgotView 接入 CaptchaWidget（按启用页面）。
     - `frontend/src/api/auth.ts`：补充 forgot/reset 接口。

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：重置令牌一次性/TTL/用后即删、重置后凭据版本号递增旧会话失效、防枚举统一响应、限流固定窗口计数与 429+Retry-After、TRUST_PROXY 三档 IP 解析、验证码密钥缺失兜底跳过并记 warn。
  - 手动验证：请求重置（存在/不存在邮箱响应一致）；超 5 次/分钟找回触发 429；启用验证码后未通过校验的登录被拒绝。

---

## 五、候选构建项（待用户决策，逐项转 Step）

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| — | 本 Build 无候选项 | Build1 范围为已确认的固定 7 Step；后续能力见 Build2.md / Build3.md | — |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-07 | 初始版本：工程骨架与认证闭环（7 Step），作为第一轮构建方案 |
