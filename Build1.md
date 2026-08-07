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

- **参考代码/伪代码：**

  > 本节为参考实现骨架：给出包结构、函数签名与关键逻辑，省略常规错误分支与样板代码；「// ...」表示此处展开常规实现。编写顺序：log（脱敏先行，被全模块依赖）→ store（迁移框架 + 事务助手）→ 0001_init.sql → config（派生与加解密）→ server（响应结构与中间件）→ health/status → main（构造注入装配收尾）。

  **1. `backend/go.mod`（依赖清单）**

  ```go
  module vpn-sub

  go 1.25

  require (
      github.com/gin-gonic/gin v1.10.x
      github.com/golang-jwt/jwt/v5 v5.x   // 会话凭据签发/验证（HS256）
      golang.org/x/crypto v0.x.x          // bcrypt / hkdf（Step 4/5 起使用，本 Step 先引入）
      modernc.org/sqlite v1.x.x           // 纯 Go SQLite 驱动（零 CGO）
  )
  ```

  > 创建 go.mod 后执行 `go mod tidy` 生成 go.sum（Step 3 的 Dockerfile 需 COPY go.sum 做依赖缓存层）。

  **2. `backend/internal/log/`（结构化日志 + token 脱敏）**

  选型：Go 标准库 `log/slog`，通过自定义 Handler 在输出层统一脱敏（调用方无感知）。

  ```go
  // 包级默认 logger（仅日志设施自身例外；业务服务实例一律构造注入）
  var defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, nil))

  func SetDefault(l *slog.Logger)       { defaultLogger = l }
  func Info(msg string, args ...any)    { defaultLogger.Info(msg, args...) }
  func Error(msg string, args ...any)   { defaultLogger.Error(msg, args...) }
  // Warn/Debug 同理；业务包统一经本包输出，禁止散落 fmt.Println

  // New 构建分级 + 双格式 logger：format="json" 用 JSONHandler，否则 TextHandler，均输出 stdout
  func New(level, format string) *slog.Logger {
      var h slog.Handler // 按 format 构建，opts.Level = 解析后的 slog.Level
      return slog.New(NewRedactHandler(h))
  }

  // token 脱敏：?token=xxx / &token=xxx 的值一律替换为 ***
  var tokenValueRe = regexp.MustCompile(`([?&]token=)[^&\s]*`)

  func Redact(s string) string { return tokenValueRe.ReplaceAllString(s, "${1}***") }

  // RedactHandler 包装任意 slog.Handler：消息与字符串属性统一经脱敏（关键约束 AGENTS §4.3）
  type RedactHandler struct{ inner slog.Handler }

  func NewRedactHandler(inner slog.Handler) *RedactHandler { return &RedactHandler{inner: inner} }

  func (h *RedactHandler) Enabled(ctx context.Context, l slog.Level) bool { return h.inner.Enabled(ctx, l) }
  func (h *RedactHandler) WithAttrs(attrs []slog.Attr) slog.Handler       { return &RedactHandler{inner: h.inner.WithAttrs(redactAttrs(attrs))} }
  func (h *RedactHandler) WithGroup(name string) slog.Handler             { return &RedactHandler{inner: h.inner.WithGroup(name)} }

  func (h *RedactHandler) Handle(ctx context.Context, r slog.Record) error {
      r.Message = Redact(r.Message)
      r.Attrs(func(a slog.Attr) bool { r.AddAttr(redactAttr(a)); return true })
      return h.inner.Handle(ctx, r)
  }

  // redactAttr：仅对 string 值脱敏，其余类型原样返回
  func redactAttr(a slog.Attr) slog.Attr {
      if a.Value.Kind() == slog.KindString {
          return slog.String(a.Key, Redact(a.Value.String()))
      }
      return a
  }
  ```

  **3. `backend/internal/store/`（SQLite 数据层：连接 / 版本化迁移 / 事务助手）**

  ```go
  // migrationsFS 由 backend/migrations 目录 go:embed 嵌入（//go:embed migrations/*.sql），
  // 由 main 注入，保证单二进制分发不依赖磁盘 SQL 文件
  type Store struct {
      db         *sql.DB
      mu         sync.Mutex // 迁移串行化
      maxVersion int        // 迁移框架执行后回填：当前代码支持的最高版本
  }

  func Open(dataDir, dbFile string, migrationsFS fs.FS) (*Store, error) {
      if err := os.MkdirAll(dataDir, 0o755); err != nil {
          return nil, fmt.Errorf("创建数据目录失败: %w", err)
      }
      db, err := sql.Open("sqlite", filepath.Join(dataDir, dbFile))
      if err != nil {
          return nil, fmt.Errorf("打开数据库失败: %w", err)
      }
      db.SetMaxOpenConns(1) // SQLite 单写者模型：规避并发写 busy，配合 busy_timeout
      if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;"); err != nil {
          _ = db.Close()
          return nil, fmt.Errorf("初始化 PRAGMA 失败: %w", err)
      }
      return &Store{db: db}, nil
  }

  func (s *Store) DB() *sql.DB { return s.db }

  // Migrate 版本化迁移（关键约束，Design1 §7.4）：
  // 文件命名 NNNN_<name>.sql，按版本号升序执行未应用项；
  // 单条迁移与其版本记录在同一事务内写入，任一失败即拒绝启动，不进入半迁移状态；
  // 数据库版本高于代码支持版本 → 拒绝启动（回滚边界）
  func (s *Store) Migrate(ctx context.Context, migrationsFS fs.FS) error {
      s.mu.Lock()
      defer s.mu.Unlock()
      // 1) 自建迁移登记表（与 0001_init.sql 中显式建表幂等共存）
      if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
          version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)`); err != nil {
          return fmt.Errorf("创建 schema_migrations 失败: %w", err)
      }
      // 2) 读取已应用版本集合
      applied := map[int]bool{}
      rows, err := s.db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
      if err != nil {
          return fmt.Errorf("读取迁移记录失败: %w", err)
      }
      for rows.Next() {
          var v int
          if err := rows.Scan(&v); err != nil {
              _ = rows.Close()
              return fmt.Errorf("解析迁移记录失败: %w", err)
          }
          applied[v] = true
      }
      if err := rows.Close(); err != nil {
          return fmt.Errorf("关闭迁移记录游标失败: %w", err)
      }
      // 3) 扫描嵌入迁移文件（按文件名排序），逐个应用
      for _, name := range sortedEntries(migrationsFS) { // 实现：fs.ReadDir + 按名排序，解析前缀版本号
          version := parseVersion(name) // 前 4 位数字，解析失败视为非法迁移文件并报错
          if applied[version] {
              s.maxVersion = max(s.maxVersion, version)
              continue
          }
          content, err := fs.ReadFile(migrationsFS, name)
          if err != nil {
              return fmt.Errorf("读取迁移文件 %s 失败: %w", name, err)
          }
          if err := s.applyOne(ctx, version, string(content)); err != nil {
              return fmt.Errorf("迁移 %s 失败，拒绝启动: %w", name, err)
          }
          s.maxVersion = version
          log.Info("迁移已应用", "file", name, "version", version)
      }
      // 4) 回滚边界校验
      var dbVersion int
      if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&dbVersion); err != nil {
          return fmt.Errorf("读取数据库 schema 版本失败: %w", err)
      }
      if dbVersion > s.maxVersion {
          return fmt.Errorf("数据库 schema 版本 %d 高于当前代码支持版本 %d，拒绝启动（请升级程序，禁止降级运行）", dbVersion, s.maxVersion)
      }
      return nil
  }

  func (s *Store) applyOne(ctx context.Context, version int, sqlText string) error {
      tx, err := s.db.BeginTx(ctx, nil)
      if err != nil {
          return err
      }
      if _, err := tx.ExecContext(ctx, sqlText); err != nil {
          _ = tx.Rollback()
          return err
      }
      if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (?)`, version); err != nil {
          _ = tx.Rollback()
          return err
      }
      return tx.Commit()
  }

  // TxImmediate 以 BEGIN IMMEDIATE 开启事务（先读后写场景专用，Design1 §4.1）：
  // 开启即持有写锁，「读 → 判定 → 写」全程串行化；fn 返回非 nil 自动回滚
  func (s *Store) TxImmediate(ctx context.Context, fn func(tx *sql.Tx) error) error {
      tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
      if err != nil {
          return fmt.Errorf("开启事务失败: %w", err)
      }
      if _, err := tx.ExecContext(ctx, "ROLLBACK; BEGIN IMMEDIATE"); err != nil {
          _ = tx.Rollback()
          return fmt.Errorf("升级为 IMMEDIATE 写事务失败: %w", err)
      }
      if err := fn(tx); err != nil {
          _ = tx.Rollback()
          return err
      }
      if err := tx.Commit(); err != nil {
          return fmt.Errorf("提交事务失败: %w", err)
      }
      return nil
  }
  ```

  **4. `backend/migrations/0001_init.sql`**

  ```sql
  -- 版本化迁移登记表（与 store.Migrate 的 CREATE TABLE IF NOT EXISTS 幂等共存）
  CREATE TABLE IF NOT EXISTS schema_migrations (
      version    INTEGER PRIMARY KEY,
      applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  -- 系统配置键值表：承载全部系统配置（签名密钥、认证参数、开关等，Design1 §5.3）
  CREATE TABLE system_config (
      key        TEXT PRIMARY KEY,
      value      TEXT,
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  ```

  **5. `backend/internal/config/`（系统配置服务，业务层）**

  ```go
  // 配置键常量（本 Step 起步集合，后续 Step 按需补充）
  const (
      KeyConfigured = "configured"  // 系统是否已完成 Setup（"true"/"false"）
      KeySigningKey = "signing_key" // 签名密钥（Setup 时生成，明文落库，Design1 §6.2）
      KeyLogLevel   = "log_level"
      KeyAppMode    = "app_mode"
  )

  // sensitiveKeys 敏感配置键集合（值以 AES-256-GCM 密文落库）；
  // 本 Step 预留框架，Build3 的 oidc_client_secret/smtp_password/captcha_secret_key 等在此登记
  var sensitiveKeys = map[string]bool{}

  type Service struct {
      store *store.Store
      log   *slog.Logger
  }

  func NewService(st *store.Store, lg *slog.Logger) *Service { return &Service{store: st, log: lg} }

  func (s *Service) Get(ctx context.Context, key string) (string, error) {
      var v string
      err := s.store.DB().QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, key).Scan(&v)
      if errors.Is(err, sql.ErrNoRows) {
          return "", nil // 未设置返回空串，调用方自行判定
      }
      if err != nil {
          return "", fmt.Errorf("读取配置 %s 失败: %w", key, err)
      }
      return v, nil
  }

  func (s *Service) Set(ctx context.Context, key, value string) error {
      v := value
      if sensitiveKeys[key] {
          enc, err := s.EncryptSensitive(ctx, value) // 失败即中断，禁止明文落库
          if err != nil {
              return err
          }
          v = enc
      }
      _, err := s.store.DB().ExecContext(ctx,
          `INSERT INTO system_config (key, value) VALUES (?, ?)
           ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, key, v)
      if err != nil {
          return fmt.Errorf("写入配置 %s 失败: %w", key, err)
      }
      return nil
  }

  // SetTx 事务内写入（供 Setup 快速开始等多键原子写入场景复用）
  func (s *Service) SetTx(ctx context.Context, tx *sql.Tx, key, value string) error { /* 同 Set 语义，SQL 在 tx 上执行 */ }

  // GetBool/GetInt：类型化读取包装（解析失败按默认值并记 warn 日志）
  func (s *Service) GetBool(ctx context.Context, key string, def bool) bool { /* ... */ }

  // --- 敏感配置加解密：AES-256-GCM，密钥由签名密钥经 HKDF-SHA256 派生（用户已确认选型）---

  // deriveKey 由签名密钥派生 32 字节 AES-256 密钥（info 固定，全程统一）
  func deriveKey(signingKey []byte) ([]byte, error) {
      r := hkdf.New(sha256.New, signingKey, nil, []byte("vpn-sub/config-encryption"))
      key := make([]byte, 32)
      if _, err := io.ReadFull(r, key); err != nil {
          return nil, fmt.Errorf("派生配置加密密钥失败: %w", err)
      }
      return key, nil
  }

  // Encrypt 输出格式：base64url(nonce ‖ 密文)
  func Encrypt(plain, signingKey []byte) (string, error) {
      key, err := deriveKey(signingKey)
      if err != nil {
          return "", err
      }
      block, err := aes.NewCipher(key)
      if err != nil {
          return "", fmt.Errorf("初始化 AES 失败: %w", err)
      }
      gcm, err := cipher.NewGCM(block)
      if err != nil {
          return "", fmt.Errorf("初始化 GCM 失败: %w", err)
      }
      nonce := make([]byte, gcm.NonceSize())
      if _, err := rand.Read(nonce); err != nil {
          return "", fmt.Errorf("生成 nonce 失败: %w", err)
      }
      return base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, plain, nil)), nil
  }

  // Decrypt：base64 解码失败或 GCM 校验失败均返回明确错误（防篡改）
  func Decrypt(encoded string, signingKey []byte) ([]byte, error) { /* Encrypt 的逆过程 */ }

  // EnsureSigningKey 确保签名密钥存在：为空则生成 32 字节加密安全随机值写入（明文，256 位熵）。
  // 供 Step 4 会话签发前置调用；Step 5 Setup 完成事务复用同一密钥，不重复生成（Design1 §3.1/6.2）
  func (s *Service) EnsureSigningKey(ctx context.Context) ([]byte, error) { /* Get → 为空则 rand.Read 后 Set */ }
  ```

  **6. `backend/internal/server/`（HTTP 服务装配，接入层）**

  ```go
  // 统一响应结构（错误码约定 AGENTS §4.8）
  type Response struct {
      Code    int    `json:"code"`
      Message string `json:"message,omitempty"`
      Data    any    `json:"data,omitempty"`
  }

  // 列表包裹结构：{ "code":0, "data": { "list": [...], "total": N } }
  type ListData struct {
      List  any   `json:"list"`
      Total int64 `json:"total"`
  }

  func OK(c *gin.Context, data any) { c.JSON(http.StatusOK, Response{Code: 0, Data: data}) }

  // Fail：httpStatus 与业务码同步取值（400/401/403/409/429/500）
  func Fail(c *gin.Context, httpStatus int, msg string) {
      if httpStatus >= 500 {
          log.Error("内部错误", "path", c.Request.URL.Path, "msg", msg) // 经脱敏 Handler 输出
          msg = "服务器内部错误"                                          // 5xx 对外脱敏
      }
      c.JSON(httpStatus, Response{Code: httpStatus, Message: msg})
  }

  type Server struct {
      engine *gin.Engine
      httpSrv *http.Server
      cfg    *config.Service
      mode   string
      // 后续 Step 的 Handler 经构造函数追加注入（auth/setup/oidc...）
  }

  // New 构造注入装配：全部依赖经参数传入，禁止包级全局变量持有服务实例
  func New(cfg *config.Service, mode, trustProxy, port string) (*Server, error) {
      engine := gin.New() // 不用 gin.Default，避免默认 logger/recovery 绕过脱敏与统一响应
      if err := applyTrustProxy(engine, trustProxy); err != nil {
          return nil, err
      }
      engine.Use(requestLogger(), panicRecovery())
      s := &Server{engine: engine, cfg: cfg, mode: mode,
          httpSrv: &http.Server{Addr: ":" + port, Handler: engine}}
      registerHealth(engine)
      registerStatus(engine, cfg, mode)
      return s, nil
  }

  // applyTrustProxy：auto=仅信任回环+私有网段转发头；on=全信任；off=不信任
  func applyTrustProxy(engine *gin.Engine, mode string) error {
      switch mode {
      case "on":
          return engine.SetTrustedProxies(nil)
      case "off":
          return engine.SetTrustedProxies([]string{})
      default: // "auto"
          return engine.SetTrustedProxies([]string{"127.0.0.1/8", "::1/128", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
      }
  }

  // requestLogger：方法/路径/状态/耗时；路径中 ?token= 值由 slog 脱敏 Handler 统一处理
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

  // panicRecovery：panic 统一转 500 通用信息（详情仅入日志）
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
  ```

  **7. `backend/internal/server/health.go` / `status.go`**

  ```go
  // health.go
  // GET /health 公开端点；应急模式返回 503 的判定在 Build3 Step 6 接入（预留注释）
  func registerHealth(engine *gin.Engine) {
      engine.GET("/health", func(c *gin.Context) {
          // TODO(Build3 Step 6)：应急模式下返回 503 {"status":"emergency"}
          c.JSON(http.StatusOK, gin.H{"status": "ok"})
      })
  }

  // status.go
  type StatusHandler struct{ cfg *config.Service } // 结构体 Handler + 依赖注入

  func registerStatus(engine *gin.Engine, cfg *config.Service, mode string) {
      h := &StatusHandler{cfg: cfg}
      engine.GET("/api/system/status", h.handle(mode)) // 公开端点，无需鉴权
  }

  // 本 Step 字段：configured / app_mode / emergency；
  // allow_local_login 等字段在 Step 4~7 按需补充
  func (h *StatusHandler) handle(mode string) gin.HandlerFunc {
      return func(c *gin.Context) {
          configured := h.cfg.GetBool(c.Request.Context(), config.KeyConfigured, false)
          OK(c, gin.H{
              "configured": configured,
              "app_mode":   mode,
              "emergency":  false, // TODO(Build3 Step 6)：替换为应急服务真实判定
          })
      }
  }
  ```

  **8. `backend/cmd/server/main.go`（程序入口 + 构造注入装配）**

  ```go
  //go:embed migrations/*.sql 的嵌入变量置于 backend/migrations/embed.go（var FS embed.FS），
  // 便于 internal/store 与测试复用；main 直接引用 migrations.FS。

  func main() {
      // 环境变量：APP_MODE(dev|prod 默认 prod)、LOG_LEVEL(默认 info)、LOG_FORMAT(默认 console)、
      // PORT(默认 8080)、TRUST_PROXY(默认 auto)、DATA_DIR(默认 ./data)、
      // RESET_ADMIN_PASSWORD（本 Build 仅读取留存，应急逻辑在 Build3 实现）
      mode := envOr("APP_MODE", "prod")
      if mode != "dev" && mode != "prod" {
          fmt.Fprintln(os.Stderr, "APP_MODE 仅支持 dev|prod")
          os.Exit(1)
      }
      logger := log.New(envOr("LOG_LEVEL", "info"), envOr("LOG_FORMAT", "console"))
      log.SetDefault(logger)
      _ = os.Getenv("RESET_ADMIN_PASSWORD") // 留存读取点，Build3 接通

      // 数据库文件按模式分离（Design1 §5.5）
      dbFile := map[string]string{"dev": "app-dev.db", "prod": "app-prod.db"}[mode]
      st, err := store.Open(envOr("DATA_DIR", "./data"), dbFile, migrations.FS)
      if err != nil {
          log.Error("打开数据库失败", "err", err)
          os.Exit(1)
      }
      if err := st.Migrate(context.Background(), migrations.FS); err != nil {
          log.Error("数据库迁移失败，拒绝启动", "err", err)
          os.Exit(1)
      }

      cfg := config.NewService(st, logger)
      if err := cfg.Set(context.Background(), config.KeyAppMode, mode); err != nil {
          log.Error("记录运行模式失败", "err", err)
          os.Exit(1)
      }

      srv, err := server.New(cfg, mode, envOr("TRUST_PROXY", "auto"), envOr("PORT", "8080"))
      if err != nil {
          log.Error("装配 HTTP 服务失败", "err", err)
          os.Exit(1)
      }

      // 信号驱动优雅退出
      ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
      defer stop()
      if err := srv.Run(ctx); err != nil {
          log.Error("HTTP 服务退出异常", "err", err)
          os.Exit(1)
      }
      log.Info("服务已退出")
  }

  func envOr(key, def string) string {
      if v := os.Getenv(key); v != "" {
          return v
      }
      return def
  }
  ```

  **9. 单元测试要点（随代码一并提交）**

  - `log/log_test.go`：`Redact("/x?token=abc")` → `/x?token=***`；`a=1&token=abc&b=2` 与消息体内嵌 token 参数同样脱敏（覆盖验收项）。
  - `config/config_test.go`：Encrypt/Decrypt 往返一致；篡改密文返回错误；敏感键 Set/Get 自动加解密。
  - `store/store_test.go`：`t.TempDir()` 建临时库 → 迁移成功且 `system_config` 存在；注入伪造更高版本记录 → Migrate 拒绝启动；并发两个 `TxImmediate` 串行完成不报 busy。
  - `server/server_test.go`：`httptest` 验证 `/health` 与 `/api/system/status` 返回结构。

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

- **参考代码/伪代码：**

  > 本节给出前端骨架的目录结构、关键配置与核心模块参考实现；「...」表示常规样板省略。编写顺序：工程配置（package/vite/vitest/tailwind）→ theme → stores → api/request（401 拦截）→ router（守卫）→ 通用组件 → 布局/404。

  **1. 工程初始化与依赖（`frontend/package.json` 等）**

  ```jsonc
  // package.json（核心字段）
  {
    "name": "vpn-sub-frontend",
    "scripts": {
      "dev": "vite",
      "build": "vue-tsc -b && vite build",
      "test": "vitest run",
      "test:watch": "vitest"
    },
    "dependencies": {
      "ant-design-vue": "^4.x",   // 按需引入，唯一 UI 组件库
      "axios": "^1.x",
      "dayjs": "^1.x",
      "pinia": "^2.x",
      "vue": "^3.x",
      "vue-router": "^4.x"
    },
    "devDependencies": {
      "@vitejs/plugin-vue": "^5.x",
      "@vue/test-utils": "^2.x",
      "autoprefixer": "^10.x",
      "jsdom": "^24.x",
      "postcss": "^8.x",
      "tailwindcss": "^3.x",
      "typescript": "^5.x",
      "vite": "^5.x",
      "vitest": "^2.x",
      "vue-tsc": "^2.x"
    }
  }
  ```

  ```ts
  // vite.config.ts：别名 + 开发代理 + vendor 分包
  import { defineConfig } from 'vite'
  import vue from '@vitejs/plugin-vue'
  import { fileURLToPath, URL } from 'node:url'

  export default defineConfig({
    plugins: [vue()],
    resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
    server: {
      proxy: {
        '/api': 'http://127.0.0.1:8080',
        '/health': 'http://127.0.0.1:8080',
      },
    },
    build: {
      rollupOptions: {
        output: {
          manualChunks: { antd: ['ant-design-vue'], vendor: ['vue', 'vue-router', 'pinia', 'axios', 'dayjs'] },
        },
      },
    },
  })
  ```

  ```ts
  // vitest.config.ts：jsdom 环境测试基建
  import { defineConfig } from 'vitest/config'
  import vue from '@vitejs/plugin-vue'

  export default defineConfig({
    plugins: [vue()],
    test: {
      environment: 'jsdom',
      globals: true,
    },
  })
  ```

  ```js
  // tailwind.config.js：class 策略暗色模式（配合 AntD darkAlgorithm，Design1 §3.7）
  /** @type {import('tailwindcss').Config} */
  export default {
    darkMode: 'class',
    content: ['./index.html', './src/**/*.{vue,ts}'],
    theme: { extend: {} },
    plugins: [],
  }
  // postcss.config.js：{ plugins: { tailwindcss: {}, autoprefixer: {} } }
  ```

  **2. 主题与暗色模式（`src/theme.ts`）**

  ```ts
  import { computed, ref, watch } from 'vue'
  import { theme as antTheme } from 'ant-design-vue'
  import zhCN from 'ant-design-vue/es/locale/zh_CN'

  export const antdLocale = zhCN               // ConfigProvider 全局中文
  export const primaryColor = '#1677FF'        // AntD 默认科技蓝，零定制（UI §1.1）

  const STORAGE_KEY = 'theme'
  const dark = ref(localStorage.getItem(STORAGE_KEY) === 'dark')

  // useTheme：暗色切换 composable；localStorage 持久化 + document 类名联动 Tailwind
  export function useTheme() {
    watch(dark, (v) => {
      localStorage.setItem(STORAGE_KEY, v ? 'dark' : 'light')
      document.documentElement.classList.toggle('dark', v)
    }, { immediate: true })
    return {
      dark,
      toggle: () => { dark.value = !dark.value },
      // App.vue 中传入 a-config-provider：:theme="antdTheme" :locale="antdLocale"
      antdTheme: computed(() => ({
        algorithm: dark.value ? antTheme.darkAlgorithm : antTheme.defaultAlgorithm,
        token: { colorPrimary: primaryColor },
      })),
    }
  }
  ```

  **3. 状态（`src/stores/`）**

  ```ts
  // stores/auth.ts：凭据存取（localStorage 键 token）+ 当前用户信息
  import { defineStore } from 'pinia'
  import { ref } from 'vue'

  export const useAuthStore = defineStore('auth', () => {
    const token = ref(localStorage.getItem('token') ?? '')
    const user = ref<UserInfo | null>(null)

    // setSession：登录/注册成功写入；Step 4 接通登录接口
    function setSession(t: string, u?: UserInfo) {
      token.value = t
      localStorage.setItem('token', t)
      if (u) user.value = u
    }
    // logout：清除本地凭据（退出为客户端语义，Design1 §5.4）
    function logout() {
      token.value = ''
      user.value = null
      localStorage.removeItem('token')
    }
    return { token, user, setSession, logout }
  })
  ```

  ```ts
  // stores/system.ts：缓存 /api/system/status（守卫与页面共用）
  export const useSystemStore = defineStore('system', () => {
    const status = ref<SystemStatus | null>(null)
    async function fetchStatus(force = false) {
      if (status.value && !force) return status.value
      status.value = await getSystemStatus() // api/system.ts
      return status.value
    }
    return { status, fetchStatus }
  })
  ```

  **4. HTTP 封装（`src/api/`）**

  ```ts
  // api/request.ts：Axios 实例 + Bearer 注入 + 401 拦截 + 错误码→UI 映射（UI §7.3）
  import axios, { AxiosError } from 'axios'
  import { message } from 'ant-design-vue'
  import router from '@/router'
  import { useAuthStore } from '@/stores/auth'

  export const http = axios.create({ baseURL: '/api', timeout: 15000 })

  // 请求拦截：自动携带会话凭据
  http.interceptors.request.use((cfg) => {
    const token = localStorage.getItem('token')
    if (token) cfg.headers.Authorization = `Bearer ${token}`
    return cfg
  })

  // 响应拦截：解包统一响应结构 + 401 清凭据跳登录
  http.interceptors.response.use(
    (resp) => {
      const body = resp.data
      if (body && typeof body.code === 'number' && body.code !== 0) {
        return Promise.reject(new ApiError(body.code, body.message ?? '请求失败'))
      }
      return body.data // 调用方直接拿到 data
    },
    (err: AxiosError<{ code: number; message: string }>) => {
      const st = err.response?.status ?? 0
      const msg = err.response?.data?.message
      if (st === 401) {
        const auth = useAuthStore()
        auth.logout()
        // 登录页自身的 401（密码错误）不跳转，由页面展示统一措辞
        if (router.currentRoute.value.path !== '/login') {
          void router.push('/login')
        }
      }
      return Promise.reject(new ApiError(st, msg ?? defaultMsg(st)))
    },
  )

  function defaultMsg(st: number): string {
    switch (st) {
      case 400: return '输入校验失败，请检查表单'
      case 403: return '权限不足'
      case 409: return '数据冲突，请刷新后重试'
      case 429: return '操作过于频繁，请稍后再试'
      case 500: return '服务器内部错误'
      default:  return '网络异常，请重试'
    }
  }

  // ApiError：携带 HTTP 状态码，供页面区分「表单定位」与「全局提示」
  export class ApiError extends Error {
    constructor(public status: number, message: string) { super(message) }
  }

  // handleApiError：错误码 → UI 映射统一入口（UI §7.3）
  // 400：页面优先做表单定位/message.error；403/409：message.error；
  // 429：message.warning（有 Retry-After 时提示等待秒数）；500：通用文案
  export function handleApiError(err: unknown, fallback?: () => void) { /* ... */ }
  ```

  ```ts
  // api/system.ts：公开系统状态（守卫专用，不携带 Bearer）
  export interface SystemStatus {
    configured: boolean
    app_mode: 'dev' | 'prod'
    emergency: boolean
    allow_local_login?: boolean   // Step 4 起
    allow_selfreg?: boolean
    user_table_empty?: boolean
    oidc_configured?: boolean     // Step 6 起
    oidc_provider_type?: string
  }

  import axios from 'axios'
  export async function getSystemStatus(): Promise<SystemStatus> {
    const resp = await axios.get('/api/system/status') // 独立实例，不走拦截器
    return resp.data.data
  }
  ```

  **5. 路由与守卫（`src/router/index.ts`）**

  ```ts
  import { createRouter, createWebHistory } from 'vue-router'
  import { useAuthStore } from '@/stores/auth'
  import { useSystemStore } from '@/stores/system'

  // 路由表骨架（本 Build 范围）：全部路由级懒加载（代码分割）
  const routes = [
    { path: '/setup', component: () => import('@/views/SetupView.vue'), meta: { layout: 'blank', public: true } },
    { path: '/login', component: () => import('@/views/LoginView.vue'), meta: { layout: 'blank', public: true } },
    { path: '/register', component: () => import('@/views/RegisterView.vue'), meta: { layout: 'blank', public: true } },
    { path: '/forgot', component: () => import('@/views/ForgotView.vue'), meta: { layout: 'blank', public: true } }, // Step 7 建视图
    { path: '/reset/:token', component: () => import('@/views/ResetView.vue'), meta: { layout: 'blank', public: true } },
    { path: '/pending', component: () => import('@/views/PendingView.vue'), meta: { layout: 'blank', public: true } },
    { path: '/', component: () => import('@/views/HomeView.vue'), meta: { layout: 'home' } },
    { path: '/:pathMatch(.*)*', component: () => import('@/views/NotFoundView.vue'), meta: { layout: 'blank', public: true } },
  ]

  const router = createRouter({ history: createWebHistory(), routes })

  // 顶部进度条：轻量自实现（2px 主色蓝固定定位条；禁止引入 NProgress 库，Build1 约束 4）
  let barEl: HTMLElement | null = null
  function progressStart() { /* 创建/显示固定定位进度条 */ }
  function progressDone() { /* 隐藏并移除 */ }

  // 路由守卫（UI §7.2）：emergency → configured → 登录态，顺序执行
  router.beforeEach(async (to) => {
    progressStart()
    const system = useSystemStore()
    const auth = useAuthStore()
    let status = system.status
    try {
      status = await system.fetchStatus() // 守卫内调 /api/system/status
    } catch {
      // 状态获取失败不阻断：仅依赖本地凭据判断，避免死循环
    }
    // 1) emergency：为 true 强制跳 /emergency（本 Build 恒 false，结构预留；白名单：/emergency 自身）
    if (status?.emergency && to.path !== '/emergency') return '/emergency'
    // 2) configured：未配置时任意路径跳 /setup；已配置时访问 /setup 跳 /login
    if (status && !status.configured && to.path !== '/setup') return '/setup'
    if (status?.configured && to.path === '/setup') return '/login'
    // 3) 登录态：无凭据访问受保护路由跳 /login
    if (!to.meta.public && !auth.token) return '/login'
    // 4) 登录页跳过：已登录访问 /login 跳 /
    if (to.path === '/login' && auth.token) return '/'
    return true
  })
  router.afterEach(() => progressDone())

  export default router
  ```

  **6. 通用组件（`src/components/`，UI §1.5）**

  ```ts
  // Notify.ts：message/notification 统一封装（全项目禁止直接调 message.*）
  import { message, notification } from 'ant-design-vue'

  export const Notify = {
    success: (msg: string) => message.success(msg),
    error: (msg: string) => message.error(msg),
    warning: (msg: string) => message.warning(msg),
    info: (msg: string) => message.info(msg),
    detail: (title: string, desc: string) => notification.error({ message: title, description: desc }),
  }
  ```

  ```vue
  <!-- ConfirmModal.vue：删除/危险操作统一确认对话框（禁止浏览器原生 confirm） -->
  <script setup lang="ts">
  import { computed, ref } from 'vue'
  import { Modal, Input } from 'ant-design-vue'

  const props = defineProps<{
    open: boolean
    title: string
    content?: string        // 影响提示文案（支持插槽扩展）
    danger?: boolean        // 危险操作红色确认按钮
    confirmWord?: string    // 需输入确认词时传入（如 RESET/IMPORT，Build3 使用）
    loading?: boolean
  }>()
  const emit = defineEmits<{ confirm: []; cancel: []; 'update:open': [boolean] }>()

  const word = ref('')
  // 确认词不正确时确认按钮禁用
  const okDisabled = computed(() => !!props.confirmWord && word.value !== props.confirmWord)
  </script>

  <template>
    <Modal :open="open" :title="title" :ok-button-props="{ danger, disabled: okDisabled, loading }"
           @ok="emit('confirm')" @cancel="emit('update:open', false); emit('cancel')">
      <p v-if="content">{{ content }}</p>
      <slot />
      <Input v-if="confirmWord" v-model:value="word" :placeholder="`请输入 ${confirmWord} 以确认`" class="mt-3" />
    </Modal>
  </template>
  ```

  ```vue
  <!-- TriStateList.vue：加载中/空/列表三态封装 -->
  <script setup lang="ts">
  defineProps<{
    loading: boolean
    empty: boolean          // 数据为空
    error?: string          // 错误信息（存在时展示错误态 + 重试）
    emptyText?: string      // 空状态引导文案（UI §7.5）
  }>()
  const emit = defineEmits<{ retry: [] }>()
  </script>

  <template>
    <Skeleton v-if="loading" active />
    <Result v-else-if="error" status="error" :sub-title="error">
      <template #extra><Button @click="emit('retry')">重试</Button></template>
    </Result>
    <Empty v-else-if="empty" :description="emptyText ?? '暂无数据'" />
    <slot v-else />
  </template>
  ```

  **7. 布局与 404（`src/layouts/`、`src/views/NotFoundView.vue`）**

  ```vue
  <!-- BlankLayout.vue：无登录态居中布局（setup/login/register/forgot/reset/pending 共用） -->
  <template>
    <div class="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900 px-4">
      <slot />
    </div>
  </template>
  <!-- HomeLayout.vue：本 Build 仅首页占位容器，Build2 替换为完整用户端布局 -->
  ```

  ```vue
  <!-- NotFoundView.vue：a-result 404 + 「返回首页」 -->
  <template>
    <Result status="404" title="404" sub-title="页面不存在">
      <template #extra><Button type="primary" @click="$router.push('/')">返回首页</Button></template>
    </Result>
  </template>
  ```

  ```vue
  <!-- App.vue：ConfigProvider 全局中文/主题 + 按 meta.layout 切换布局 -->
  <script setup lang="ts">
  import { ConfigProvider } from 'ant-design-vue'
  import { useRoute } from 'vue-router'
  import { useTheme, antdLocale } from '@/theme'
  import BlankLayout from '@/layouts/BlankLayout.vue'
  import HomeLayout from '@/layouts/HomeLayout.vue'

  const { antdTheme } = useTheme()
  const route = useRoute()
  const layouts: Record<string, unknown> = { blank: BlankLayout, home: HomeLayout }
  </script>

  <template>
    <ConfigProvider :locale="antdLocale" :theme="antdTheme">
      <component :is="layouts[(route.meta.layout as string) ?? 'home']">
        <RouterView />
      </component>
    </ConfigProvider>
  </template>
  ```

  **8. 单元测试要点（验收要求）**

  - `theme.spec.ts`：`useTheme().toggle()` 后 localStorage `theme` 键值翻转且 `document.documentElement` 带 `dark` 类。
  - `request.spec.ts`：mock 401 响应 → auth store 被清空、`router.push('/login')` 被调用；登录页内的 401 不跳转。
  - `router-guard.spec.ts`：mock `getSystemStatus` 返回 `configured:false` → 任意路径跳 `/setup`；`configured:true` 且无 token 访问受保护路由跳 `/login`。

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

- **参考代码/伪代码：**

  > 选型已确认：前端产物以 `go:embed` 嵌入后端单二进制（Design1 §5.6）。为使仓库内 `go build` 始终可用，后端源码内置占位目录 `backend/web/dist/index.html`（内容「前端未构建」），容器构建时由 Dockerfile 用真实产物覆盖。

  **1. `Dockerfile`（仓库根目录，多阶段构建）**

  ```dockerfile
  # ============ 阶段一：前端构建 ============
  FROM node:22-alpine AS frontend
  WORKDIR /build
  COPY frontend/package.json frontend/package-lock.json ./
  RUN npm ci
  COPY frontend/ ./
  RUN npm run build

  # ============ 阶段二：后端静态编译（CGO_ENABLED=0）============
  FROM golang:1.25-alpine AS backend
  WORKDIR /build
  COPY backend/go.mod backend/go.sum ./
  RUN go mod download
  COPY backend/ ./
  # 用真实前端产物替换 embed 占位目录，再编译进二进制
  COPY --from=frontend /build/dist ./web/dist
  RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

  # ============ 阶段三：最小运行时 ============
  FROM gcr.io/distroless/static-debian12:nonroot
  # nonroot 变体自带 uid 65532 非 root 用户，无需额外 USER 指令
  COPY --from=backend /out/server /server
  # 预建 /public 目录结构（安装包/站点资源，Build2 填充内容）；数据卷承载全部持久化
  ENV DATA_DIR=/data
  VOLUME ["/data"]
  EXPOSE 8080
  ENTRYPOINT ["/server"]
  ```

  **2. `.dockerignore`**

  ```
  node_modules
  frontend/node_modules
  frontend/dist
  data
  *.db
  *.db-*
  .git
  *.md
  ```

  **3. `docker-compose.yml`（双接入方式注释模板，Design1 §7.1）**

  ```yaml
  services:
    vpn-sub:
      build: .
      # ====== 接入方式二选一（Design1 §7.1）======
      # 方式 A（推荐，公网部署）：仅绑回环，由外部反代承担 TLS 与域名接入
      ports:
        - "127.0.0.1:8080:8080"
      # 方式 B（局域网直连）：注释掉上面一行，改用下面一行（注意 HTTP 明文风险，见 README）
      #   - "8080:8080"
      volumes:
        - vpn-data:/data          # 单一数据卷：数据库 + 内容文件 + /public 资源
      environment:
        APP_MODE: prod
        LOG_LEVEL: info
        TRUST_PROXY: auto
        # 应急恢复：管理员密码救援时取消下行注释并重启容器（Design1 §3.8，逻辑在 Build3 实现）
        # RESET_ADMIN_PASSWORD: "1"
      restart: unless-stopped
      healthcheck:
        # distroless 无 curl，用 wget（static 镜像自带 busybox wget；实际路径可用 docker inspect 确认）；仅状态展示，不触发重启动作
        test: ["CMD", "wget", "-qO-", "http://127.0.0.1:8080/health"]
        interval: 30s
        timeout: 5s
        retries: 3

  volumes:
    vpn-data:
  ```

  **4. 前端产物嵌入与静态服务（`backend/web/` + `internal/server/static.go`）**

  ```go
  // backend/web/web.go：嵌入占位目录，容器构建时被真实 dist 覆盖
  //go:embed all:dist
  var DistFS embed.FS

  // backend/web/dist/index.html（占位，保证仓库内 go build 可用）：
  //   <!doctype html><meta charset="utf-8"><title>前端未构建</title>

  // internal/server/static.go：静态资源分级 + SPA 回退（Design1 §5.6，缓存策略见 Design1 表格）
  func registerStatic(engine *gin.Engine, dataDir string) error {
      distFS, err := fs.Sub(web.DistFS, "dist")
      if err != nil {
          return fmt.Errorf("提取嵌入前端产物失败: %w", err)
      }
      httpDist := http.FS(distFS)

      // /assets/*：前端产物（文件名含哈希）→ immutable 长期缓存
      engine.GET("/assets/*filepath", func(c *gin.Context) {
          c.Header("Cache-Control", "public, max-age=31536000, immutable")
          c.FileFromFS("assets"+c.Param("filepath"), httpDist)
      })

      // /public/*：数据卷内可缓存资源（安装包/站点 ICON）→ public + max-age；
      // 路径穿越防护：Clean 后校验仍在 dataDir/public 内（禁止 .. 与绝对路径逃逸）
      publicRoot := filepath.Join(dataDir, "public")
      engine.GET("/public/*filepath", func(c *gin.Context) {
          rel := filepath.Clean(strings.TrimPrefix(c.Param("filepath"), "/"))
          if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
              server.Fail(c, http.StatusNotFound, "资源不存在")
              return
          }
          full := filepath.Join(publicRoot, rel)
          if !strings.HasPrefix(full, publicRoot+string(os.PathSeparator)) {
              server.Fail(c, http.StatusNotFound, "资源不存在")
              return
          }
          c.Header("Cache-Control", "public, max-age=86400")
          c.File(full) // 文件不存在时由 gin 返回 404
      })

      // 其余非 API GET 路径：SPA 回退到 index.html（不缓存，保证新版本即时生效）
      engine.NoRoute(func(c *gin.Context) {
          if c.Request.Method != http.MethodGet {
              server.Fail(c, http.StatusNotFound, "接口不存在")
              return
          }
          c.Header("Cache-Control", "no-store")
          c.FileFromFS("index.html", httpDist)
      })
      return nil
  }
  // 在 server.New 中 registerStatic 最后注册（在 health/status 之后）
  ```

  **5. `README.md` 部署向导（结构大纲，Design1 §7.1）**

  ```markdown
  # VPN 订阅管理系统 — 部署向导
  ## 快速开始：docker compose up -d → 浏览器访问 → Setup 引导
  ## 接入方式 A：外部反代（推荐，公网部署）
    - ports 绑 127.0.0.1；Nginx/Caddy 反代示例（TLS 终止 + 域名）
    - OIDC 回调与验证码依赖公网可达的 HTTPS 域名
  ## 接入方式 B：局域网直连
    - ports 直接暴露 8080；⚠️ HTTP 明文风险提示：凭据与订阅链接明文传输，仅限可信内网
  ## 抢注窗口提示：Setup 完成后请立即注册/登录成为管理员（首个完成认证的用户自动成为管理员，存在被他人抢注的风险）
  ## 应急恢复：RESET_ADMIN_PASSWORD 环境变量用法（Build3）
  ```

  > 开发联调说明：仓库内直接 `go run ./cmd/server` 时嵌入的是占位前端，开发期前后端分离走 Vite dev server 代理（见 Step 2 vite.config.ts）。

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

- **参考代码/伪代码：**

  > 编写顺序：0002_users.sql → internal/auth（密码/邮箱/JWT/双中间件）→ internal/user（首管理员原子事务）→ server/auth.go + status 扩展 → 前端页面。auth 与 user 包通过接口解耦，避免循环依赖。

  **1. `backend/migrations/0002_users.sql`**

  ```sql
  CREATE TABLE users (
      id                 INTEGER PRIMARY KEY AUTOINCREMENT, -- 内部稳定 user_id
      oidc_subject       TEXT UNIQUE,          -- OIDC 身份（本 Step 建列，Step 6 使用）
      username           TEXT NOT NULL,
      email              TEXT UNIQUE,          -- SQLite 原生允许多 NULL，NULL 不冲突
      role               TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
      group_id           INTEGER,              -- 所属组（默认组在 Step 5 创建后回填，Build2 完整使用）
      password_hash      TEXT,                 -- 空 = 不可本地登录（OIDC-only 用户）
      user_source        TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
      status             TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
      credential_version INTEGER NOT NULL DEFAULT 0,   -- 凭据版本号（会话失效机制，Design1 §5.4）
      oidc_claims        TEXT,                 -- 待审批 OIDC 用户 claims 快照 JSON（Step 6 使用）
      created_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at         TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_users_group_id ON users(group_id);
  ```

  **2. `backend/internal/auth/`（业务层：认证服务）**

  ```go
  // session.go：会话时长与密码规则常量（关键设计参数，禁止修改）
  const (
      SessionRemember   = 7 * 24 * time.Hour // 记住我：7 天
      SessionNoRemember = 24 * time.Hour     // 不勾选：24 小时
      OidcSession       = 7 * 24 * time.Hour // OIDC 固定 7 天，无记住我
      MinPasswordLen    = 8                  // 所有本地密码入口统一
  )

  // password.go
  func HashPassword(plain string) (string, error) {
      b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
      if err != nil {
          return "", fmt.Errorf("密码哈希失败: %w", err)
      }
      return string(b), nil
  }

  func CheckPassword(hash, plain string) bool {
      return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
  }

  // ValidatePassword：复杂度校验（≥8 字符，所有本地密码入口统一，Design1 §4.6）
  func ValidatePassword(p string) error {
      if utf8.RuneCountInString(p) < MinPasswordLen {
          return errors.New("密码长度至少 8 个字符")
      }
      return nil
  }

  // NormalizeEmail：所有写入入口统一 trim + 小写化；拒绝控制字符（防 SMTP 头注入）
  func NormalizeEmail(raw string) (string, error) {
      e := strings.ToLower(strings.TrimSpace(raw))
      if e == "" || len(e) > 254 || !strings.Contains(e, "@") {
          return "", errors.New("邮箱格式无效")
      }
      for _, r := range e {
          if r < 0x20 || r == 0x7f {
              return "", errors.New("邮箱含非法控制字符")
          }
      }
      return e, nil
  }

  // claims.go：会话凭据载荷仅含 user_id + credential_version + 标准声明；
  // 角色/组等权限信息禁止入凭据，每次请求实时查库（Design1 §3.2/5.4）
  type Claims struct {
      jwt.RegisteredClaims
      UserID            int64 `json:"uid"`
      CredentialVersion int   `json:"cv"`
  }

  // UserSnapshot：凭据校验所需的用户最小信息（由 user 包实现 UserSource 接口注入，避免循环依赖）
  type UserSnapshot struct {
      ID                int64
      Role              string
      Status            string
      CredentialVersion int
  }

  type UserSource interface {
      SnapshotByID(ctx context.Context, id int64) (*UserSnapshot, error)
  }

  type Service struct {
      cfg   *config.Service
      users UserSource
      log   *slog.Logger
  }

  func NewService(cfg *config.Service, users UserSource, lg *slog.Logger) *Service {
      return &Service{cfg: cfg, users: users, log: lg}
  }

  // Issue：用 signing_key 以 HS256 签名；签发前确保签名密钥存在（Setup 前兜底，不重复生成）
  func (s *Service) Issue(ctx context.Context, userID int64, credVersion int, dur time.Duration) (string, time.Time, error) {
      key, err := s.cfg.EnsureSigningKey(ctx)
      if err != nil {
          return "", time.Time{}, err
      }
      now := time.Now()
      exp := now.Add(dur)
      claims := Claims{
          RegisteredClaims:  jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(exp)},
          UserID:            userID,
          CredentialVersion: credVersion,
      }
      token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
      if err != nil {
          return "", time.Time{}, fmt.Errorf("签发会话凭据失败: %w", err)
      }
      return token, exp, nil
  }

  func (s *Service) Parse(ctx context.Context, tokenStr string) (*Claims, error) {
      key, err := s.cfg.GetSigningKey(ctx) // 未生成时验签必然失败 → 401
      if err != nil {
          return nil, err
      }
      var claims Claims
      _, err = jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
          if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
              return nil, errors.New("签名算法不匹配")
          }
          return key, nil
      })
      if err != nil {
          return nil, fmt.Errorf("凭据解析失败: %w", err)
      }
      return &claims, nil
  }

  // middleware.go
  const (
      CtxUserID   = "auth_user_id"
      CtxUserRole = "auth_user_role"
  )

  // SessionMiddleware（会话校验层）：解析凭据 → 实时查库取用户 → 比对 credential_version → 校验 status=active
  func (s *Service) SessionMiddleware() gin.HandlerFunc {
      return func(c *gin.Context) {
          header := c.GetHeader("Authorization")
          tokenStr := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
          if tokenStr == "" || tokenStr == header {
              server.Fail(c, http.StatusUnauthorized, "会话凭据缺失")
              c.Abort()
              return
          }
          claims, err := s.Parse(c.Request.Context(), tokenStr)
          if err != nil {
              server.Fail(c, http.StatusUnauthorized, "会话凭据无效或已过期")
              c.Abort()
              return
          }
          snap, err := s.users.SnapshotByID(c.Request.Context(), claims.UserID) // 实时查库，禁止缓存
          if err != nil || snap == nil {
              server.Fail(c, http.StatusUnauthorized, "会话凭据无效或已过期")
              c.Abort()
              return
          }
          if snap.CredentialVersion != claims.CredentialVersion {
              server.Fail(c, http.StatusUnauthorized, "会话凭据已失效，请重新登录")
              c.Abort()
              return
          }
          if snap.Status != "active" {
              server.Fail(c, http.StatusUnauthorized, "账号未激活或已被禁用")
              c.Abort()
              return
          }
          c.Set(CtxUserID, snap.ID)
          c.Set(CtxUserRole, snap.Role)
          c.Next()
      }
  }

  // AdminMiddleware（角色校验层）：叠加在会话校验之后，两中间件独立可组合（Build2/3 管理端点叠加使用）
  func AdminMiddleware() gin.HandlerFunc {
      return func(c *gin.Context) {
          if role, _ := c.Get(CtxUserRole); role != "admin" {
              server.Fail(c, http.StatusForbidden, "权限不足")
              c.Abort()
              return
          }
          c.Next()
      }
  }
  ```

  **3. `backend/internal/user/`（业务层：用户服务）**

  ```go
  var (
      ErrEmailConflict   = errors.New("邮箱已被注册")
      ErrAuthFailed      = errors.New("邮箱或密码错误")      // 统一措辞，防枚举
      ErrAccountInactive = errors.New("账号未激活或已被禁用")
  )

  type Service struct {
      store *store.Store
      cfg   *config.Service
      log   *slog.Logger
  }

  func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger) *Service {
      return &Service{store: st, cfg: cfg, log: lg}
  }

  type User struct {
      ID                int64
      Username          string
      Email             string // 空串表示 NULL
      Role              string
      Status            string
      Source            string
      GroupID           int64  // 0 表示 NULL
      CredentialVersion int
      HasPassword       bool
  }

  // Register 自注册 + 首管理员机制（Design1 §2.5，关键约束）：
  // 「邮箱唯一预查 → 空表判定 → 写入（首管理员）→ 置位标记」全程单个 BEGIN IMMEDIATE 事务（并发串行化）
  func (s *Service) Register(ctx context.Context, username, emailRaw, password string) (*User, error) {
      email, err := auth.NormalizeEmail(emailRaw)
      if err != nil {
          return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
      }
      if err := auth.ValidatePassword(password); err != nil {
          return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
      }
      hash, err := auth.HashPassword(password)
      if err != nil {
          return nil, err
      }
      var created *User
      err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          // 1) 邮箱唯一冲突 → 409（基于规范化值）
          var dup int
          if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ?`, email).Scan(&dup); err != nil {
              return err
          }
          if dup > 0 {
              return ErrEmailConflict
          }
          // 2) 空表判定口径：任何用户记录（含待审批）存在即算非空
          var total int
          if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&total); err != nil {
              return err
          }
          first := total == 0
          // 3) 默认状态：自注册审批开关本 Build 默认关闭 → 注册即 active；
          //    首管理员检查先于审批开关判定：空表时永远免审批直接激活（防死锁，Design1 §2.6）
          role, status, source := "user", "active", "selfreg"
          if !first && s.selfRegApprovalEnabled(ctx) { // 开关读取路径预留（Build3 接通配置键）
              status = "pending"
          }
          if first {
              role, status = "admin", "active" // 首管理员免审批，不受任何审批开关影响
          }
          res, err := tx.ExecContext(ctx,
              `INSERT INTO users (username, email, password_hash, role, user_source, status) VALUES (?,?,?,?,?,?)`,
              username, email, hash, role, source, status)
          if err != nil {
              return ErrEmailConflict // 并发下 UNIQUE 约束失败同样按 409 处理
          }
          id, err := res.LastInsertId()
          if err != nil {
              return err
          }
          created = &User{ID: id, Username: username, Email: email, Role: role, Status: status, Source: source, HasPassword: true}
          // 4) 首管理员：同事务置位「已初始化」标记（用户表为空时忽略该标记）
          if first {
              if err := s.cfg.SetTx(ctx, tx, config.KeyAdminInitialized, "true"); err != nil {
                  return err
              }
          }
          return nil
      })
      if err != nil {
          return nil, err
      }
      s.log.Info("用户注册成功", "user_id", created.ID, "role", created.Role, "first_admin", created.Role == "admin")
      return created, nil
  }

  // selfRegApprovalEnabled：读配置键 selfreg_approval（默认 false；Build3 面板接通）
  func (s *Service) selfRegApprovalEnabled(ctx context.Context) bool {
      return s.cfg.GetBool(ctx, config.KeySelfRegApproval, false)
  }

  // Login：邮箱 + 密码校验；失败提示统一措辞（不区分「邮箱不存在」与「密码错误」，防枚举）
  func (s *Service) Login(ctx context.Context, emailRaw, password string) (*User, error) {
      email, err := auth.NormalizeEmail(emailRaw)
      if err != nil {
          return nil, ErrAuthFailed // 格式非法也归入统一措辞
      }
      u, err := s.getByEmail(ctx, email)
      if err != nil {
          return nil, err
      }
      if u == nil || !u.HasPassword || !auth.CheckPassword(u.PasswordHash, password) {
          return nil, ErrAuthFailed
      }
      if u.Status != "active" { // 仅 active 可登录；待审批/已禁用统一提示
          return nil, ErrAccountInactive
      }
      return u, nil
  }

  // SnapshotByID 实现 auth.UserSource（供凭据校验中间件实时查库）
  func (s *Service) SnapshotByID(ctx context.Context, id int64) (*auth.UserSnapshot, error) { /* SELECT id, role, status, credential_version */ }

  // IsTableEmpty：注册入口可见性用（空表 = 0 行，含待审批）
  func (s *Service) IsTableEmpty(ctx context.Context) (bool, error) { /* COUNT(*) == 0 */ }

  func (s *Service) GetByID(ctx context.Context, id int64) (*User, error)  { /* ... */ }
  func (s *Service) getByEmail(ctx context.Context, email string) (*User, error) { /* ... */ }
  ```

  **4. `backend/internal/server/auth.go`（认证端点，接入层）**

  ```go
  type AuthHandler struct {
      authSvc *auth.Service
      userSvc *user.Service
      cfg     *config.Service
  }

  func RegisterAuthRoutes(engine *gin.Engine, h *AuthHandler) {
      g := engine.Group("/api/auth")
      g.POST("/register", h.register)
      g.POST("/login", h.login)
      g.GET("/me", h.authSvc.SessionMiddleware(), h.me)
      g.POST("/logout", h.authSvc.SessionMiddleware(), h.logout)
      // Step 7 将在 register/login 上叠加限流与验证码中间件
  }

  // 表单入参统一长度限制（AGENTS §八-6）
  type registerReq struct {
      Username string `json:"username" binding:"required,min=1,max=64"`
      Email    string `json:"email" binding:"required,max=254"`
      Password string `json:"password" binding:"required,max=128"`
  }

  func (h *AuthHandler) register(c *gin.Context) {
      var req registerReq
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      ctx := c.Request.Context()
      // 注册入口可见性：allow_selfreg 开启，或用户表为空（例外，Design1 §5.2）
      allowSelf := h.cfg.GetBool(ctx, config.KeyAllowSelfreg, false)
      empty, err := h.userSvc.IsTableEmpty(ctx)
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      if !allowSelf && !empty {
          server.Fail(c, http.StatusForbidden, "未开放注册")
          return
      }
      u, err := h.userSvc.Register(ctx, req.Username, req.Email, req.Password)
      if errors.Is(err, user.ErrEmailConflict) {
          server.Fail(c, http.StatusConflict, "该邮箱已被注册")
          return
      }
      if errors.Is(err, user.ErrBadRequest) {
          server.Fail(c, http.StatusBadRequest, err.Error())
          return
      }
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      if u.Status == "active" {
          // 直接激活：签发会话（注册无记住我选项，按 24 小时）
          token, exp, err := h.authSvc.Issue(ctx, u.ID, u.CredentialVersion, auth.SessionNoRemember)
          if err != nil {
              server.Fail(c, http.StatusInternalServerError, err.Error())
              return
          }
          server.OK(c, gin.H{"token": token, "expires_at": exp.Unix(), "status": u.Status, "is_admin": u.Role == "admin"})
          return
      }
      server.OK(c, gin.H{"status": "pending", "message": "账号已提交，等待管理员审批"})
  }

  type loginReq struct {
      Email    string `json:"email" binding:"required,max=254"`
      Password string `json:"password" binding:"required,max=128"`
      Remember bool   `json:"remember"`
  }

  func (h *AuthHandler) login(c *gin.Context) {
      var req loginReq
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      u, err := h.userSvc.Login(c.Request.Context(), req.Email, req.Password)
      if errors.Is(err, user.ErrAuthFailed) {
          server.Fail(c, http.StatusUnauthorized, "邮箱或密码错误") // 统一措辞
          return
      }
      if errors.Is(err, user.ErrAccountInactive) {
          server.Fail(c, http.StatusUnauthorized, "账号未激活或已被禁用")
          return
      }
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      dur := auth.SessionNoRemember
      if req.Remember {
          dur = auth.SessionRemember // 7 天 / 24 小时
      }
      token, exp, err := h.authSvc.Issue(c.Request.Context(), u.ID, u.CredentialVersion, dur)
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, gin.H{"token": token, "expires_at": exp.Unix(), "user": userInfo(u)})
  }

  // me：返回 username/email/role/group/status/user_source
  func (h *AuthHandler) me(c *gin.Context) { /* 取 CtxUserID 查库组装 userInfo 返回 */ }

  // logout：退出为客户端语义（Design1 §5.4：无服务端会话存储），仅返回成功，前端清除本地 token
  func (h *AuthHandler) logout(c *gin.Context) { server.OK(c, nil) }

  // userInfo：组装对外用户信息（group 字段在 Step 5 前可返回空）
  func userInfo(u *user.User) gin.H { /* ... */ }
  ```

  **5. `status.go` 扩展（本 Step 新增字段）**

  ```go
  // 在 Step 1 基础字段上追加：
  empty, err := users.IsTableEmpty(ctx)
  if err != nil {
      server.Fail(c, http.StatusInternalServerError, err.Error())
      return
  }
  data := gin.H{
      // ... configured / app_mode / emergency（Step 1）
      "allow_local_login": h.cfg.GetBool(ctx, config.KeyAllowLocalLogin, true),
      "allow_selfreg":     h.cfg.GetBool(ctx, config.KeyAllowSelfreg, false),
      "user_table_empty":  empty, // 注册入口可见性所需，有意公开（Design1 §5.2）
  }
  ```

  新增配置键常量：`KeyAllowLocalLogin = "allow_local_login"`、`KeyAllowSelfreg = "allow_selfreg"`、`KeySelfRegApproval = "selfreg_approval"`、`KeyAdminInitialized = "admin_initialized"`。

  **6. 前端（`src/api/auth.ts` + stores + 页面）**

  ```ts
  // api/auth.ts
  import { http } from './request'

  export interface LoginResult { token: string; expires_at: number; user: UserInfo }
  export interface RegisterResult { token?: string; status: 'active' | 'pending'; is_admin?: boolean; message?: string }

  export const register = (data: { username: string; email: string; password: string }) =>
    http.post<any, RegisterResult>('/auth/register', data)
  export const login = (data: { email: string; password: string; remember: boolean }) =>
    http.post<any, LoginResult>('/auth/login', data)
  export const me = () => http.get<any, UserInfo>('/auth/me')
  export const logout = () => http.post('/auth/logout')
  ```

  ```ts
  // stores/auth.ts：补充 action（Step 2 已建 token/user 存取）
  async function loginAction(form: { email: string; password: string; remember: boolean }) {
    const data = await login(form)
    setSession(data.token, data.user)
  }
  async function registerAction(form: { username: string; email: string; password: string }) {
    const data = await register(form)
    if (data.token) setSession(data.token) // 直接激活
    return data
  }
  async function logoutAction() {
    try { await logout() } catch { /* 退出为客户端语义，接口失败不阻断 */ }
    logout()
  }
  ```

  ```vue
  <!-- LoginView.vue 骨架（UI §2.2）：本地登录区块 + 注册入口可见性 + 表空提示 + OIDC 占位 -->
  <script setup lang="ts">
  import { computed, onMounted, reactive, ref } from 'vue'
  import { useRouter } from 'vue-router'
  import { Form, Input, Button, Checkbox, Alert, Divider } from 'ant-design-vue'
  import { useAuthStore } from '@/stores/auth'
  import { useSystemStore } from '@/stores/system'
  import { useTheme } from '@/theme'
  import { handleApiError } from '@/api/request'

  const router = useRouter()
  const auth = useAuthStore()
  const system = useSystemStore()
  const { dark, toggle } = useTheme()
  const form = reactive({ email: '', password: '', remember: false })
  const submitting = ref(false)
  const errorMsg = ref('')

  // 注册入口可见性：allow_selfreg 开启，或 user_table_empty 且 allow_local_login（始终显示）
  const showRegister = computed(() => {
    const st = system.status
    return !!st && (st.allow_selfreg || (st.user_table_empty && st.allow_local_login))
  })
  const tableEmpty = computed(() => system.status?.user_table_empty ?? false)

  async function onSubmit() {
    submitting.value = true
    errorMsg.value = ''
    try {
      await auth.loginAction(form)
      await router.push('/')
    } catch (err) {
      errorMsg.value = (err as Error).message // 后端统一措辞直接展示
    } finally {
      submitting.value = false
    }
  }
  onMounted(() => { if (auth.token) router.replace('/') }) // 已登录访问自动跳 /
  </script>

  <template>
    <div class="w-full max-w-md">
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow p-8">
        <h1 class="text-xl font-semibold mb-6">登录</h1>
        <!-- 表空提示：系统尚未配置管理员，首个注册用户将成为管理员 -->
        <Alert v-if="tableEmpty" type="info" show-icon class="mb-4"
               message="系统尚未配置管理员，首个注册用户将成为管理员" />
        <Form layout="vertical" @finish="onSubmit">
          <Form.Item label="邮箱" name="email" :rules="[{ required: true, type: 'email' }]">
            <Input v-model:value="form.email" autocomplete="email" />
          </Form.Item>
          <Form.Item label="密码" name="password" :rules="[{ required: true }]">
            <Input.Password v-model:value="form.password" autocomplete="current-password" />
          </Form.Item>
          <div class="flex items-center justify-between mb-4">
            <Checkbox v-model:checked="form.remember">记住我</Checkbox>
            <RouterLink to="/forgot" class="text-sm">忘记密码？</RouterLink>
          </div>
          <Alert v-if="errorMsg" type="error" :message="errorMsg" class="mb-4" />
          <Button type="primary" html-type="submit" block :loading="submitting">登录</Button>
        </Form>
        <!-- OIDC 区块占位（Step 6 填充：oidc_configured 时渲染「使用 OIDC 登录」，mock 时渲染模拟表单） -->
        <Divider v-if="showRegister" plain />
        <div v-if="showRegister" class="text-center">
          还没有账号？<RouterLink to="/register">立即注册</RouterLink>
        </div>
      </div>
      <div class="text-right mt-3"><Button size="small" @click="toggle">{{ dark ? '浅色模式' : '暗色模式' }}</Button></div>
    </div>
  </template>
  ```

  ```vue
  <!-- RegisterView.vue 骨架：用户名 + 邮箱 + 密码 + 确认密码 -->
  <script setup lang="ts">
  // 表单规则：密码 ≥8 字符；确认密码与密码一致
  async function onSubmit() {
    const data = await auth.registerAction(form)
    if (data.status === 'active') await router.push('/')       // 直接激活：token 已存 → 首页
    else await router.push('/pending')                          // 待审批
  }
  </script>
  <!-- 模板：a-form 四字段 + 提交按钮 + 返回登录链接，错误展示同 LoginView -->

  <!-- PendingView.vue：a-result info「账号待审批，等待管理员激活」+ 返回登录按钮（无凭据独立页） -->
  <!-- HomeView.vue（占位）：展示当前用户信息（me 接口）+ 退出按钮（auth.logoutAction → /login） -->
  ```

  **7. 单元测试要点（验收要求）**

  - 密码：哈希/校验往返、`ValidatePassword` 拒绝 <8 字符。
  - 邮箱规范化：trim/小写/控制字符拒绝；唯一约束基于规范化值。
  - 会话：Issue/Parse 往返；伪造签名拒绝；修改 `credential_version` 后旧凭据经中间件返回 401。
  - 首管理员原子性：并发 N 个 `Register`（goroutine + 临时库）只产生一个 `role=admin`。
  - 统一失败措辞：不存在邮箱与错误密码返回同一文案（均为 401「邮箱或密码错误」）。
  - 前端：登录/注册表单提交（mock api）；401 后 auth store 清空并跳 `/login`。

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

- **参考代码/伪代码：**

  > 编写顺序：0003_groups_platforms.sql → internal/setup（slug 生成器 → 快速开始事务）→ server/setup.go → SetupView.vue。Setup 状态不单独建端点，复用 `/api/system/status`（执行约束：不重复）。

  **1. `backend/migrations/0003_groups_platforms.sql`**

  ```sql
  CREATE TABLE groups (
      id             INTEGER PRIMARY KEY AUTOINCREMENT,
      slug           TEXT NOT NULL UNIQUE,      -- group- + 8 位随机短码（独立命名空间）
      name           TEXT NOT NULL UNIQUE,
      is_default     INTEGER NOT NULL DEFAULT 0,
      needs_reselect INTEGER NOT NULL DEFAULT 0, -- Build2 使用
      created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE platforms (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      slug            TEXT NOT NULL UNIQUE,     -- platform- + 8 位随机短码（独立命名空间）
      name            TEXT NOT NULL,            -- 不强制唯一
      description     TEXT NOT NULL DEFAULT '',
      schemes         TEXT NOT NULL DEFAULT '[]', -- JSON 数组，有序；含 {url} 占位符
      extra_headers   TEXT NOT NULL DEFAULT '{}', -- JSON 键值对；值支持 {frontend_url} 占位符
      installer_file  TEXT,                     -- 本地上传安装包（带时间戳文件名，Build2 使用）
      installer_url   TEXT,                     -- 外部链接
      created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  ```

  **2. `backend/internal/setup/`（业务层：Setup 服务）**

  ```go
  type Service struct {
      store *store.Store
      cfg   *config.Service
      log   *slog.Logger
  }

  func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger) *Service {
      return &Service{store: st, cfg: cfg, log: lg}
  }

  func (s *Service) IsConfigured(ctx context.Context) (bool, error) {
      return s.cfg.GetBool(ctx, config.KeyConfigured, false), nil
  }

  // --- 标识自动生成器（Design1 §2.2）---

  // slug 短码字符集：小写字母数字，去除易混淆字符（与密码字符集规则一致）
  const slugCharset = "abcdefghjkmnpqrstuvwxyz23456789"

  // GenerateSlug：类型前缀 + 8 位加密安全随机短码；冲突自动重试最多 3 次，仍冲突报错并记日志
  func (s *Service) GenerateSlug(ctx context.Context, tx *sql.Tx, prefix string, exists func(slug string) (bool, error)) (string, error) {
      for attempt := 0; attempt < 3; attempt++ {
          code, err := randomCode(8) // crypto/rand 从 slugCharset 取 8 字符；失败返回 err
          if err != nil {
              return "", err
          }
          slug := prefix + code
          dup, err := exists(slug)
          if err != nil {
              return "", err
          }
          if !dup {
              return slug, nil
          }
      }
      s.log.Error("标识生成冲突超过重试上限", "prefix", prefix)
      return "", errors.New("标识生成失败：连续冲突，请重试")
  }

  func randomCode(n int) (string, error) {
      b := make([]byte, n)
      if _, err := rand.Read(b); err != nil {
          return "", fmt.Errorf("生成随机短码失败: %w", err)
      }
      for i := range b {
          b[i] = slugCharset[int(b[i])%len(slugCharset)]
      }
      return string(b), nil
  }

  // --- 快速开始（关键约束：单个 BEGIN IMMEDIATE 事务，任一步失败整体回滚）---

  // CompleteQuickStart：确保签名密钥 → 预置默认组 → 3 个默认平台 → configured 置位 → frontend_url 推导初始值
  func (s *Service) CompleteQuickStart(ctx context.Context, r *http.Request) error {
      configured, err := s.IsConfigured(ctx)
      if err != nil {
          return err
      }
      if configured {
          return ErrAlreadyConfigured // 接入层映射 409
      }
      frontendURL := DeriveFrontendURL(r) // 事务前推导（只读请求头）
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          // 1) 确保签名密钥存在（复用 auth 阶段的 EnsureSigningKey 逻辑，不重复生成）
          if err := s.cfg.EnsureSigningKeyTx(ctx, tx); err != nil {
              return err
          }
          // 2) 预置默认组（is_default=1，不可删除；Design1 §2.2）
          groupSlug, err := s.GenerateSlug(ctx, tx, "group-", func(slug string) (bool, error) {
              return tableHasSlug(tx, "groups", slug)
          })
          if err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx,
              `INSERT INTO groups (slug, name, is_default) VALUES (?, '默认组', 1)`, groupSlug); err != nil {
              return fmt.Errorf("创建预置默认组失败: %w", err)
          }
          // 3) 预置 3 个默认平台（Design1 §3.4.4）
          for _, p := range defaultPlatforms(frontendURL) {
              slug, err := s.GenerateSlug(ctx, tx, "platform-", func(slug string) (bool, error) {
                  return tableHasSlug(tx, "platforms", slug)
              })
              if err != nil {
                  return err
              }
              if _, err := tx.ExecContext(ctx,
                  `INSERT INTO platforms (slug, name, description, schemes, extra_headers) VALUES (?,?,?,?,?)`,
                  slug, p.Name, p.Description, p.Schemes, p.ExtraHeaders); err != nil {
                  return fmt.Errorf("创建默认平台 %s 失败: %w", p.Name, err)
              }
          }
          // 4) configured 置位 + frontend_url 初始值（手动覆盖优先的缓存语义在 Build3 面板实现）
          if err := s.cfg.SetTx(ctx, tx, config.KeyConfigured, "true"); err != nil {
              return err
          }
          if err := s.cfg.SetTx(ctx, tx, config.KeyFrontendURL, frontendURL); err != nil {
              return err
          }
          return nil
      })
  }

  // defaultPlatforms：预置平台的 scheme 与附加头（Design1 §3.4.4/4.3）；
  // v2rayNG 与 Shadowrocket 取各自客户端常用导入 scheme
  func defaultPlatforms(frontendURL string) []struct{ Name, Description, Schemes, ExtraHeaders string } {
      return []struct{ Name, Description, Schemes, ExtraHeaders string }{
          {"Clash Verge", "桌面端 Clash 内核客户端",
              `["clash://install-config?url={url}"]`,
              // 三条兼容附加头；Content-Disposition 文件名在下载时按订阅名动态生成，此处存模板
              `{"Content-Disposition":"attachment; filename*=UTF-8''subscription.yaml","profile-update-interval":"300","profile-web-page-url":"{frontend_url}"}`},
          {"v2rayNG", "Android 端 V2Ray 客户端",
              `["v2rayng://install-config?url={url}"]`, `{}`},
          {"Shadowrocket", "iOS 端代理客户端",
              `["shadowrocket://add/{url}"]`, `{}`},
      }
  }

  // DeriveFrontendURL（Design1 §3.1/6.4）：TRUST_PROXY 信任来源时优先取 X-Forwarded-Host，否则取 Host 头；
  // scheme 按 X-Forwarded-Proto / TLS 状态推导（gin 的 ForwardedByClientIP 已按 TRUST_PROXY 配置生效）
  func DeriveFrontendURL(r *http.Request) string {
      host := r.Host
      if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" && trustedForwarded(r) {
          host = strings.TrimSpace(strings.Split(xfh, ",")[0])
      }
      scheme := "http"
      if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
          scheme = "https"
      }
      return scheme + "://" + host
  }
  // trustedForwarded：接入层根据 TRUST_PROXY 策略判定远端是否可信（auto=回环+私有网段；on=真；off=假）
  ```

  新增配置键常量：`KeyFrontendURL = "frontend_url"`。

  **3. `backend/internal/server/setup.go`（Setup 端点，接入层）**

  ```go
  type SetupHandler struct{ setupSvc *setup.Service }

  func RegisterSetupRoutes(engine *gin.Engine, h *SetupHandler) {
      // GET /api/setup/status 不单独实现：复用 /api/system/status 的 configured 字段，避免重复端点
      engine.POST("/api/setup/quickstart", h.quickstart) // 仅在未配置状态暴露（处理器内校验）
      // Step 6 追加 POST /api/setup/oidc；Build3 追加 POST /api/setup/import
  }

  func (h *SetupHandler) quickstart(c *gin.Context) {
      ctx := c.Request.Context()
      configured, err := h.setupSvc.IsConfigured(ctx)
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      if configured {
          server.Fail(c, http.StatusConflict, "系统已完成配置") // 已配置返回 409
          return
      }
      if err := h.setupSvc.CompleteQuickStart(ctx, c.Request); err != nil {
          if errors.Is(err, setup.ErrAlreadyConfigured) {
              server.Fail(c, http.StatusConflict, "系统已完成配置")
              return
          }
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, gin.H{"configured": true, "message": "配置完成，请立即注册成为管理员"})
  }
  ```

  **4. 前端 `src/views/SetupView.vue`（UI §2.1）**

  ```vue
  <script setup lang="ts">
  import { computed, ref } from 'vue'
  import { useRouter } from 'vue-router'
  import { Steps, Card, Button, Tag, Alert, Result } from 'ant-design-vue'
  import { useSystemStore } from '@/stores/system'
  import { useTheme } from '@/theme'
  import { http } from '@/api/request'
  import { Notify } from '@/components/Notify'

  const router = useRouter()
  const system = useSystemStore()
  const { dark, toggle } = useTheme()
  const current = ref(0)                     // a-steps 当前步：认证方式 → 完成
  const submitting = ref(false)
  const done = ref(false)
  const isProd = computed(() => system.status?.app_mode === 'prod')

  async function quickStart() {
    submitting.value = true
    try {
      await http.post('/setup/quickstart')
      current.value = 1
      done.value = true
      await system.fetchStatus(true)         // 刷新守卫状态，后续访问 /setup 将被守卫跳到 /login
    } catch (err) {
      Notify.error((err as Error).message)
    } finally {
      submitting.value = false
    }
  }

  function advanced() { Notify.info('请先完成 OIDC 配置（Step 6 提供）') } // 高级配置占位
  // 「导入已有配置」卡片：仅 Production 模式渲染；本 Step 直接隐藏（Build3 补充）
  </script>

  <template>
    <!-- 独立全屏路由：居中单列卡片 max-w-720px；顶部 ICON + 「首次配置」+ 模式徽标；右上角暗色切换 -->
    <div class="w-full max-w-3xl">
      <div class="flex justify-end mb-2">
        <Button size="small" @click="toggle">{{ dark ? '浅色模式' : '暗色模式' }}</Button>
      </div>
      <Card>
        <div class="flex items-center gap-3 mb-6">
          <h1 class="text-xl font-semibold">首次配置</h1>
          <Tag :color="isProd ? 'green' : 'blue'">{{ isProd ? 'Production' : 'Dev' }}</Tag>
        </div>
        <Steps :current="current" :items="[{ title: '认证方式' }, { title: '完成' }]"
               class="mb-6" direction="horizontal" />
        <!-- <768 步骤条转竖向：:direction="isMobile ? 'vertical' : 'horizontal'"（Tailwind useBreakpoint 或 matchMedia） -->

        <template v-if="!done">
          <!-- 快速开始卡片（本 Step 唯一可用入口） -->
          <Card class="mb-4" hoverable>
            <div class="flex items-center justify-between">
              <div>
                <div class="font-medium">快速开始 <Tag color="processing">推荐</Tag></div>
                <div class="text-gray-500 text-sm mt-1">本地账号模式，零配置一键完成</div>
              </div>
              <Button type="primary" :loading="submitting" @click="quickStart">完成配置</Button>
            </div>
          </Card>
          <!-- 高级配置卡片：渲染但点击提示（Step 6 填充） -->
          <Card hoverable @click="advanced">
            <div class="font-medium">高级配置</div>
            <div class="text-gray-500 text-sm mt-1">接入单点登录（OIDC）</div>
          </Card>
        </template>

        <!-- 完成页：显著步骤式提示 + 抢注风险（Design1 §3.1） -->
        <Result v-else status="success" title="配置完成" sub-title="请部署者本人立即注册成为管理员">
          <template #extra>
            <Alert type="warning" show-icon class="text-left mb-4"
                   message="抢注风险提示"
                   description="首个完成注册的用户将自动成为管理员。公网部署下，请尽快完成注册以关闭抢注窗口。" />
            <Button type="primary" @click="router.push('/login')">前往登录</Button>
          </template>
        </Result>
      </Card>
    </div>
  </template>
  ```

  **5. 路由接线验证**

  Step 2 守卫已实现 `configured=false → /setup`、`已配置访问 /setup → /login`；本 Step 仅验证链路（quickstart 成功后 `fetchStatus(true)` 使守卫立即生效）。前端无需新增守卫代码。

  **6. 单元测试要点（验收要求）**

  - `GenerateSlug`：格式匹配 `^(group-|platform-)[a-z0-9]{8}$`；注入冲突 exists 函数验证 3 次重试与超限报错。
  - `CompleteQuickStart`：临时库执行 → 默认组（is_default=1）+ 3 平台（clash-verge 含 3 条附加头）+ configured=true + frontend_url 写入；注入中途失败 → 整体回滚（表内无残留）；重复调用返回 ErrAlreadyConfigured。
  - `DeriveFrontendURL`：X-Forwarded-Host 优先（可信时）、Host 兜底、https 推导。

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

- **参考代码/伪代码：**

  > 编写顺序：0004_oidc.sql → internal/oidc（配置 → 授权发起 → 回调/查建 → 模拟 → 绑定 → 测试连接）→ server/oidc.go → Setup 高级分支 → 前端。用户查建逻辑依赖 user 包新增方法（GetBySubject/BindSubjectIfNull 等），同 Step 补充。

  **1. `backend/migrations/0004_oidc.sql`**

  ```sql
  CREATE TABLE oidc_states (
      state        TEXT PRIMARY KEY,           -- ≥128 位随机值
      code_verifier TEXT NOT NULL,             -- PKCE S256 验证器
      intent       TEXT NOT NULL CHECK (intent IN ('login','bind')),
      bind_user_id INTEGER,                    -- intent=bind 时记录目标用户
      created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_oidc_states_created ON oidc_states(created_at); -- TTL 过期清理用
  ```

  **2. `backend/internal/oidc/`（业务层：OIDC 服务）**

  ```go
  // config.go：OIDC 参数存 system_config；各提供商参数独立存储（切换类型保留已填字段）
  const (
      KeyProviderType = "oidc_provider_type" // keycloak/auth0/generic/mock
      KeyConfigured   = "oidc_configured"
      // 各提供商参数以 JSON 存于独立键（敏感字段在 JSON 内单独加密）：
      //   oidc_params_keycloak / oidc_params_auth0 / oidc_params_generic / oidc_params_mock
      // 结构：{ base_url, realm, client_id, client_secret(密文) }
  )

  type Params struct {
      BaseURL      string `json:"base_url"`
      Realm        string `json:"realm"`        // keycloak 专用
      ClientID     string `json:"client_id"`
      ClientSecret string `json:"client_secret"` // 落库前经 config.Encrypt 加密
  }

  type Service struct {
      store    *store.Store
      cfg      *config.Service
      authSvc  *auth.Service
      users    *user.Service
      mode     string // APP_MODE：模拟 OIDC 仅 dev 可用
      log      *slog.Logger
      httpCli  *http.Client
  }

  // --- 授权发起（PKCE + state 持久化 + Cookie 由接入层下发）---

  // StartFlow：生成 state（≥128 位）与 code_verifier（PKCE S256）→ 持久化 → 返回授权页 URL
  func (s *Service) StartFlow(ctx context.Context, intent string, bindUserID int64) (authURL, state string, err error) {
      stateBytes := make([]byte, 32) // 256 位 ≥ 128 位要求
      if _, err := rand.Read(stateBytes); err != nil {
          return "", "", fmt.Errorf("生成 state 失败: %w", err)
      }
      state = base64.RawURLEncoding.EncodeToString(stateBytes)
      verifierBytes := make([]byte, 32)
      if _, err := rand.Read(verifierBytes); err != nil {
          return "", "", fmt.Errorf("生成 code_verifier 失败: %w", err)
      }
      verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
      // TTL 10 分钟：写入前顺带清理过期记录（代替独立定时器，简单可靠）
      if err := s.saveState(ctx, state, verifier, intent, bindUserID); err != nil {
          return "", "", err
      }
      p, err := s.currentParams(ctx)
      if err != nil {
          return "", "", err
      }
      sum := sha256.Sum256([]byte(verifier))
      challenge := base64.RawURLEncoding.EncodeToString(sum[:])
      q := url.Values{
          "response_type":         {"code"},
          "client_id":             {p.ClientID},
          "redirect_uri":          {s.callbackURL(ctx)},
          "scope":                 {"openid email profile"},
          "state":                 {state},
          "code_challenge":        {challenge},
          "code_challenge_method": {"S256"},
      }
      return p.AuthorizationEndpoint + "?" + q.Encode(), state, nil // endpoint 取自发现文档（带缓存）
  }

  // ConsumeState：回调时校验存储记录存在并用后即删（防重放）；
  // 三重校验（Cookie state == 回调参数 state == 存储记录）由接入层比对 Cookie 后调用本方法
  func (s *Service) ConsumeState(ctx context.Context, state string) (*StateRecord, error) {
      var rec StateRecord
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := tx.QueryRowContext(ctx,
              `SELECT state, code_verifier, intent, bind_user_id, created_at FROM oidc_states WHERE state = ?`, state).
              Scan(&rec.State, &rec.CodeVerifier, &rec.Intent, &rec.BindUserID, &rec.CreatedAt); err != nil {
              return err
          }
          if time.Since(rec.CreatedAt) > 10*time.Minute {
              return sql.ErrNoRows // 过期视同不存在
          }
          _, err := tx.ExecContext(ctx, `DELETE FROM oidc_states WHERE state = ?`, state) // 用后即删
          return err
      })
      if err != nil {
          return nil, errors.New("state 无效或已过期")
      }
      return &rec, nil
  }

  // --- 回调：code 换 token → 解析身份 ---

  type Identity struct {
      Subject       string
      Email         string
      EmailVerified bool
      Username      string
      RoleClaims    []string
      GroupClaims   []string
      RawClaims     string // JSON 快照（待审批用户存 oidc_claims 列）
  }

  // Exchange：用 code + code_verifier 换 token，解析 id_token/userinfo 提取身份（含 role/group claims）
  func (s *Service) Exchange(ctx context.Context, rec *StateRecord, code string) (*Identity, error) {
      // POST token_endpoint：grant_type=authorization_code + code_verifier；
      // 解析 id_token（jwt 库验签，key 取自发现文档 jwks_uri）；email_verified 缺省视为 false
      // role/group claims 路径：默认 realm_access.roles / groups，可在面板配置（Build3）
      ...
  }

  // --- 用户查建逻辑（Design1 §4.6，关键约束）---

  type ResolveResult struct {
      User    *user.User // 登录成功时非空
      Pending bool       // 进入待审批（不签发会话，302 /pending）
      Message string     // 冲突/待审批提示文案
  }

  func (s *Service) ResolveLogin(ctx context.Context, id *Identity) (*ResolveResult, error) {
      // 1) subject 命中 → 直接登录（username 每次刷新为提供商最新值；email 首次写入后不自动覆盖）
      u, err := s.users.GetBySubject(ctx, id.Subject)
      if err != nil {
          return nil, err
      }
      if u != nil {
          if u.Status == "pending" {
              return &ResolveResult{Pending: true, Message: "已提交，等待审批"}, nil // 待审批重复登录
          }
          if u.Status == "disabled" {
              return &ResolveResult{Message: "账号未激活或已被禁用"}, nil
          }
          if err := s.users.RefreshUsername(ctx, u.ID, id.Username); err != nil {
              return nil, err
          }
          return &ResolveResult{User: u}, nil
      }
      // 2) subject 未命中但邮箱命中
      if id.Email != "" {
          eu, err := s.users.GetByEmail(ctx, id.Email)
          if err != nil {
              return nil, err
          }
          if eu != nil {
              if eu.Status == "disabled" {
                  return &ResolveResult{Message: "目标账号已禁用，无法合并"}, nil
              }
              if eu.OidcSubject != "" {
                  return &ResolveResult{Message: "目标账号已绑定其他 OIDC 身份"}, nil
              }
              if !id.EmailVerified {
                  return &ResolveResult{Message: "邮箱未验证，无法自动合并"}, nil
              }
              if eu.Status == "pending" {
                  // 待审批命中：不创建新记录，将新 subject 绑定到该待审批账号
                  if err := s.users.BindSubject(ctx, eu.ID, id.Subject); err != nil {
                      return nil, err
                  }
                  return &ResolveResult{Pending: true, Message: "已提交，等待审批"}, nil
              }
              // 自动合并：条件更新防并发覆盖；合并即激活（OIDC 视同可信，可绕过审批）
              n, err := s.users.BindSubjectIfNull(ctx, eu.ID, id.Subject) // WHERE oidc_subject IS NULL
              if err != nil {
                  return nil, err
              }
              if n == 0 {
                  return &ResolveResult{Message: "目标账号已被并发绑定其他 OIDC 身份"}, nil
              }
              return &ResolveResult{User: eu}, nil
          }
      }
      // 3) 均不存在 → 创建新用户（首管理员机制同样生效，复用 user 包原子事务）
      //    OIDC 审批开关默认关闭 → 直接激活；开启且未命中白名单 → pending + 存 claims + 不签发会话
      approvalOn := s.cfg.GetBool(ctx, config.KeyOidcApproval, false) // 读取路径预留（Build3 接通）
      hitWhitelist := s.matchWhitelist(ctx, id)                       // 白名单为空时跳过校验直接激活
      pending := approvalOn && !hitWhitelist
      u, err := s.users.CreateFromOidc(ctx, id.Username, id.Email, id.Subject, id.RawClaims, pending)
      if err != nil {
          return nil, err
      }
      if pending {
          return &ResolveResult{Pending: true, Message: "账号已创建，等待审批"}, nil
      }
      return &ResolveResult{User: u}, nil
  }

  // ResolveBind（intent=bind）：校验 subject 未绑定其他账号 → 写入目标账号；不签发会话
  func (s *Service) ResolveBind(ctx context.Context, rec *StateRecord, id *Identity) error {
      other, err := s.users.GetBySubject(ctx, id.Subject)
      if err != nil {
          return err
      }
      if other != nil && other.ID != rec.BindUserID {
          return errors.New("该 OIDC 身份已绑定其他账号")
      }
      n, err := s.users.BindSubjectIfNull(ctx, rec.BindUserID, id.Subject)
      if err != nil {
          return err
      }
      if n == 0 {
          return errors.New("目标账号已绑定其他 OIDC 身份")
      }
      return nil
  }

  // --- 模拟 OIDC（仅 Dev + provider=mock）---

  // MockLogin：subject 固定为输入邮箱，走与真实 OIDC 一致的查建/合并逻辑
  func (s *Service) MockLogin(ctx context.Context, email, username string, emailVerified bool, roles, groups []string) (*ResolveResult, error) {
      providerType, _ := s.cfg.Get(ctx, KeyProviderType)
      if s.mode != "dev" || providerType != "mock" {
          return nil, errors.New("模拟登录仅 Dev 模式且选择模拟 OIDC 时可用")
      }
      normalized, err := auth.NormalizeEmail(email)
      if err != nil {
          return nil, err
      }
      if username == "" {
          username = strings.SplitN(normalized, "@", 2)[0] // 留空取邮箱 @ 前缀
      }
      id := &Identity{Subject: normalized, Email: normalized, EmailVerified: emailVerified,
          Username: username, RoleClaims: roles, GroupClaims: groups}
      return s.ResolveLogin(ctx, id)
  }

  // --- 测试连接（Design1 §3.1）---

  type TestResult struct {
      OK       bool     `json:"ok"`
      Message  string   `json:"message"`
      Warnings []string `json:"warnings"`
  }

  func (s *Service) TestConnection(ctx context.Context, providerType string, p Params) (*TestResult, error) {
      if providerType == "mock" {
          return &TestResult{OK: true, Message: "模拟模式始终通过"}, nil
      }
      // ① 发现文档可达性 + 配置完整性（base_url/realm/client_id/回调地址）
      disc, err := s.fetchDiscovery(ctx, providerType, p)
      if err != nil {
          return &TestResult{OK: false, Message: "发现文档不可达：" + err.Error()}, nil
      }
      res := &TestResult{OK: true, Message: "配置有效"}
      // ② client_credentials 换 token 验证 Client ID/Secret；不支持该授权类型时降级为警告不阻断
      if err := s.verifyClientCredentials(ctx, disc.TokenEndpoint, p); err != nil {
          if isGrantUnsupported(err) {
              res.Warnings = append(res.Warnings, "提供商不支持 client_credentials，未验证 Client Secret："+err.Error())
          } else {
              return &TestResult{OK: false, Message: "Client ID/Secret 验证失败：" + err.Error()}, nil
          }
      }
      return res, nil
  }
  ```

  user 包同 Step 新增方法：`GetBySubject` / `GetByEmail`（返回含 OidcSubject 字段的完整记录）/ `RefreshUsername` / `BindSubject` / `BindSubjectIfNull`（`UPDATE users SET oidc_subject=? WHERE id=? AND oidc_subject IS NULL`，返回受影响行数）/ `CreateFromOidc`（复用 Step 4 首管理员 BEGIN IMMEDIATE 事务：空表判定 → 首管理员免审批；pending 时存 oidc_claims）。

  **3. `backend/internal/server/oidc.go`（OIDC 端点，接入层）**

  ```go
  type OidcHandler struct{ oidcSvc *oidc.Service; authSvc *auth.Service }

  func RegisterOidcRoutes(engine *gin.Engine, h *OidcHandler, sessionMW gin.HandlerFunc) {
      g := engine.Group("/api/auth/oidc")
      g.GET("/login", h.login)                       // 发起授权（302），不限流
      g.GET("/callback", h.callback)                 // 回调，不限流（state 一次性 + 三重校验已防重放）
      g.POST("/mock/login", h.mockLogin)             // 模拟登录（仅 Dev + mock）
      g.POST("/bind", sessionMW, h.bind)             // 发起绑定（需会话）
      engine.POST("/api/oidc/test", h.test)          // 本 Step 不加鉴权；Build3 新增管理员专用测试端点
  }

  const stateCookie = "oidc_state"

  func (h *OidcHandler) login(c *gin.Context) {
      authURL, state, err := h.oidcSvc.StartFlow(c.Request.Context(), "login", 0)
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      c.SetSameSite(http.SameSiteLaxMode)
      c.SetCookie(stateCookie, state, 600, "/", "", c.Request.TLS != nil, true) // HttpOnly，HTTPS 下 Secure
      c.Redirect(http.StatusFound, authURL)
  }

  func (h *OidcHandler) callback(c *gin.Context) {
      ctx := c.Request.Context()
      cookieState, err := c.Cookie(stateCookie)
      paramState := c.Query("state")
      // 三重校验前两层：Cookie state == 回调参数 state（第三层：存储记录存在）
      if err != nil || cookieState == "" || cookieState != paramState {
          c.Redirect(http.StatusFound, "/login?oidc_error=state_mismatch")
          return
      }
      rec, err := h.oidcSvc.ConsumeState(ctx, paramState) // 用后即删
      if err != nil {
          c.Redirect(http.StatusFound, "/login?oidc_error=state_expired")
          return
      }
      id, err := h.oidcSvc.Exchange(ctx, rec, c.Query("code"))
      if err != nil {
          c.Redirect(http.StatusFound, "/login?oidc_error=exchange_failed")
          return
      }
      switch rec.Intent {
      case "bind":
          if err := h.oidcSvc.ResolveBind(ctx, rec, id); err != nil {
              c.Redirect(http.StatusFound, "/profile?oidc_bind_error="+url.QueryEscape(err.Error()))
              return
          }
          c.Redirect(http.StatusFound, "/profile?oidc_bound=1") // 不签发会话
      case "login":
          res, err := h.oidcSvc.ResolveLogin(ctx, id)
          if err != nil {
              c.Redirect(http.StatusFound, "/login?oidc_error=resolve_failed")
              return
          }
          if res.Pending {
              c.Redirect(http.StatusFound, "/pending") // 不签发会话，不经凭据中转页
              return
          }
          if res.User == nil { // 冲突（已绑定其他 OIDC/已禁用/邮箱未验证）
              c.Redirect(http.StatusFound, "/login?oidc_error="+url.QueryEscape(res.Message))
              return
          }
          // OIDC 会话固定 7 天，无记住我（Design1 §3.2）
          token, _, err := h.authSvc.Issue(ctx, res.User.ID, res.User.CredentialVersion, auth.OidcSession)
          if err != nil {
              c.Redirect(http.StatusFound, "/login?oidc_error=issue_failed")
              return
          }
          c.Redirect(http.StatusFound, "/login/callback?token="+url.QueryEscape(token))
      }
  }

  // mockLogin：仅 Dev + mock；入参 email/username/roles/groups/email_verified → 成功签发 7 天会话
  func (h *OidcHandler) mockLogin(c *gin.Context) { /* 调 MockLogin；pending/冲突映射对应响应；成功 Issue + OK 返回 token */ }

  // bind：会话内发起绑定授权（StartFlow("bind", userID) → Cookie + 302）
  func (h *OidcHandler) bind(c *gin.Context) { /* ... */ }

  // test：测试连接（不落库）；入参 provider_type + 参数（Setup 与面板共用）
  func (h *OidcHandler) test(c *gin.Context) { /* 绑定参数 → TestConnection → OK 返回结果 */ }
  ```

  **4. Setup 高级配置分支（扩展 `internal/setup` 与 `server/setup.go`）**

  ```go
  // CompleteOidcSetup：与快速开始同一事务语义：保存 OIDC 参数（Secret 加密）→ 预置默认组/平台 → configured 置位
  func (s *Service) CompleteOidcSetup(ctx context.Context, r *http.Request, providerType string, p oidc.Params) error {
      // 事务前：推导 frontend_url 与 callback_url 初始值（frontend_url + "/api/auth/oidc/callback"）
      frontendURL := DeriveFrontendURL(r)
      callbackURL := frontendURL + "/api/auth/oidc/callback"
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := s.cfg.EnsureSigningKeyTx(ctx, tx); err != nil { return err }       // 复用不重复生成
          secretCipher, err := s.cfg.EncryptWithTx(ctx, tx, p.ClientSecret)            // Secret 加密落库
          if err != nil { return err }
          paramsJSON := marshalParams(p, secretCipher)                                  // 各提供商参数存独立键
          if err := s.cfg.SetTx(ctx, tx, "oidc_params_"+providerType, paramsJSON); err != nil { return err }
          if err := s.cfg.SetTx(ctx, tx, oidc.KeyProviderType, providerType); err != nil { return err }
          if err := s.cfg.SetTx(ctx, tx, oidc.KeyConfigured, "true"); err != nil { return err }
          if err := s.seedPresets(ctx, tx, frontendURL); err != nil { return err }     // 抽取快速开始的预置逻辑复用
          if err := s.cfg.SetTx(ctx, tx, config.KeyConfigured, "true"); err != nil { return err }
          if err := s.cfg.SetTx(ctx, tx, config.KeyFrontendURL, frontendURL); err != nil { return err }
          if err := s.cfg.SetTx(ctx, tx, config.KeyCallbackURL, callbackURL); err != nil { return err }
          return nil
      })
  }
  // server/setup.go：POST /api/setup/oidc（未配置状态可调，已配置 409）；入参 provider_type + 各提供商字段
  ```

  **5. 前端**

  ```vue
  <!-- OidcCallbackView.vue（中转页 /login/callback）：提取 token → 存 store → 立即清空 URL → 跳 / -->
  <script setup lang="ts">
  import { onMounted } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { useAuthStore } from '@/stores/auth'

  const route = useRoute()
  const router = useRouter()
  const auth = useAuthStore()

  onMounted(() => {
    const token = route.query.token as string | undefined
    if (token) {
      auth.setSession(token)
      history.replaceState(null, '', '/login/callback') // 立即清空 URL（含 token），Design1 §3.2
    }
    router.replace('/')
  })
  </script>
  <template><div class="text-center text-gray-500 py-20">正在登录…</div></template>
  ```

  ```vue
  <!-- LoginView.vue 扩展（Step 4 占位区填充）：oidc_configured 时渲染 OIDC 区块 -->
  <template>
    <!-- ... Step 4 本地登录表单 ... -->
    <template v-if="system.status?.oidc_configured">
      <Divider plain>或</Divider>
      <!-- mock 提供商（仅 Dev）：模拟登录表单，标题标注「Dev 模拟登录」 -->
      <Form v-if="system.status.oidc_provider_type === 'mock'" layout="vertical" @finish="onMockLogin">
        <div class="text-sm text-gray-400 mb-2">Dev 模拟登录</div>
        <Form.Item label="邮箱" :rules="[{ required: true, type: 'email' }]">
          <Input v-model:value="mockForm.email" />
        </Form.Item>
        <Form.Item label="用户名（可留空，默认取邮箱前缀）"><Input v-model:value="mockForm.username" /></Form.Item>
        <Checkbox v-model:checked="mockForm.email_verified" class="mb-3">email_verified（默认勾选）</Checkbox>
        <!-- role/group 附加属性：勾选后输入值（测试白名单用） -->
        <Button block @click="onMockLogin">模拟登录</Button>
      </Form>
      <!-- 真实提供商：主按钮直接跳后端发起授权 -->
      <Button v-else block type="primary" ghost @click="window.location.href = '/api/auth/oidc/login'">
        使用 OIDC 登录
      </Button>
    </template>
  </template>
  <!-- 回调错误（oidc_error/冲突文案）从 route.query 读取后以 a-alert 展示 -->
  ```

  ```vue
  <!-- SetupView.vue 高级配置扩展（Step 5 占位卡片接通） -->
  <script setup lang="ts">
  // 选中「高级配置」→ 步骤 1：a-radio-group 提供商（Keycloak/Auth0/通用 OIDC/模拟 OIDC，模拟仅 Dev 可选）
  // → 步骤 2：参数表单（按提供商动态渲染 Base URL/Realm/域名/Client ID/Client Secret；
  //          「高级」折叠面板展示前端地址/回调地址预填推导值，只读供核对）
  // → 步骤 3：测试连接（POST /api/oidc/test，结果 a-alert：成功绿/失败红/警告黄）
  // → 完成：POST /api/setup/oidc → 成功走 Step 5 同一完成页（抢注提示）
  </script>
  ```

  ```ts
  // api/oidc.ts
  export const oidcTest = (data: { provider_type: string } & Record<string, string>) =>
    http.post<any, { ok: boolean; message: string; warnings: string[] }>('/oidc/test', data)
  export const setupOidc = (data: { provider_type: string } & Record<string, string>) =>
    http.post('/setup/oidc', data)
  export const mockLogin = (data: { email: string; username?: string; email_verified: boolean; roles?: string[]; groups?: string[] }) =>
    http.post<any, { token: string; expires_at: number; status?: string }>('/auth/oidc/mock/login', data)
  export const startBind = () => http.post<any, { auth_url: string }>('/auth/oidc/bind') // 后端返回授权页 URL，前端以 window.location.href 跳转（后端亦可直接 302）
  ```

  **6. status 端点扩展**：`oidc_configured`（读 `oidc_configured` 配置键）、`oidc_provider_type`（未配置时为空串）。

  **7. 单元测试要点（验收要求）**

  - state 三重校验：Cookie/参数/存储任一不一致拒绝；ConsumeState 后再用同 state 失败（用后即删）；超 10 分钟记录视同不存在。
  - PKCE：challenge = BASE64URL(SHA256(verifier))，授权 URL 含 S256 方法声明。
  - 自动合并：email_verified=true 且未绑定 → BindSubjectIfNull 命中并激活；已绑定其他 OIDC/已禁用/邮箱未验证 → 返回对应冲突文案；并发合并仅一个成功（n==0 分支）。
  - 待审批分支：subject 命中 pending → 「已提交」提示不重复创建；邮箱命中 pending → 新 subject 绑到既有记录。
  - 手动绑定：ResolveBind 成功后不签发会话；subject 已绑其他账号拒绝。
  - 模拟 OIDC：Dev+mock 可用；prod 或非同提供商拒绝；subject 固定为邮箱可复现合并。

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

- **参考代码/伪代码：**

  > 编写顺序：0005_reset_tokens.sql → auth/reset.go → internal/captcha → internal/ratelimit → 端点接通 → 前端页面/组件。

  **1. `backend/migrations/0005_reset_tokens.sql`**

  ```sql
  CREATE TABLE password_reset_tokens (
      token      TEXT PRIMARY KEY,             -- ≥128 位加密安全随机值
      user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      expires_at TIMESTAMP NOT NULL,           -- 1 小时 TTL
      used       INTEGER NOT NULL DEFAULT 0,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_reset_tokens_user ON password_reset_tokens(user_id);
  ```

  **2. `backend/internal/auth/reset.go`（密码重置服务）**

  ```go
  type ResetService struct {
      store *store.Store
      users *user.Service
      log   *slog.Logger
      // sendMail 预留：Build3 SMTP 接通前以日志记录代替（标注 Build3 接通）
      sendMail func(ctx context.Context, to, resetURL string) error
  }

  const resetTokenTTL = time.Hour // 一次性、1 小时 TTL

  // Request：生成一次性重置令牌；无论邮箱是否存在均返回统一提示（防枚举）
  func (s *ResetService) Request(ctx context.Context, emailRaw string) error {
      email, err := NormalizeEmail(emailRaw)
      if err != nil {
          return nil // 格式非法也归入统一响应，不泄露信息
      }
      u, err := s.users.GetByEmail(ctx, email)
      if err != nil {
          return err
      }
      if u != nil && u.HasPassword {
          buf := make([]byte, 32) // 256 位 ≥ 128 位熵（Design1 §4.2）
          if _, err := rand.Read(buf); err != nil {
              return fmt.Errorf("生成重置令牌失败: %w", err)
          }
          token := base64.RawURLEncoding.EncodeToString(buf)
          if _, err := s.store.DB().ExecContext(ctx,
              `INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES (?,?,?)`,
              token, u.ID, time.Now().Add(resetTokenTTL)); err != nil {
              return fmt.Errorf("写入重置令牌失败: %w", err)
          }
          // 已配置 SMTP 时发送（Build3 接通）；未配置 → 日志记录并提示联系管理员
          if s.sendMail != nil {
              if err := s.sendMail(ctx, email, resetLink(token)); err != nil {
                  s.log.Warn("重置邮件发送失败", "err", err) // 不阻断主流程
              }
          } else {
              s.log.Info("重置令牌已生成（SMTP 未接通，Build3 替换）", "user_id", u.ID)
          }
      }
      return nil // 接入层统一返回「若该邮箱已注册，重置链接已发送」
  }

  // Complete：校验令牌（存在 + 未过期 + 未使用）→ 设新密码 → 用后即删 → 递增 credential_version
  func (s *ResetService) Complete(ctx context.Context, token, newPassword string) error {
      if err := ValidatePassword(newPassword); err != nil {
          return fmt.Errorf("%w: %v", ErrBadRequest, err)
      }
      hash, err := HashPassword(newPassword)
      if err != nil {
          return err
      }
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error { // 先读后写：IMMEDIATE 防并发双消费
          var userID int64
          var expiresAt time.Time
          var used int
          err := tx.QueryRowContext(ctx,
              `SELECT user_id, expires_at, used FROM password_reset_tokens WHERE token = ?`, token).
              Scan(&userID, &expiresAt, &used)
          if errors.Is(err, sql.ErrNoRows) || used == 1 || time.Now().After(expiresAt) {
              return ErrTokenInvalid // 统一返回「重置链接无效或已过期」
          }
          if err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE token = ?`, token); err != nil { // 用后即删
              return err
          }
          if _, err := tx.ExecContext(ctx,
              `UPDATE users SET password_hash = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
              hash, userID); err != nil { // 递增凭据版本号：全部现有会话立即失效
              return err
          }
          return nil
      })
  }
  ```

  **3. `backend/internal/captcha/`（验证码服务）**

  ```go
  const (
      KeyProvider  = "captcha_provider"  // recaptcha/turnstile/off，默认 off
      KeySiteKey   = "captcha_site_key"
      KeySecretKey = "captcha_secret_key" // 敏感加密（登记入 config.sensitiveKeys）
      KeyPages     = "captcha_pages"      // JSON 数组：register/login/forgot
  )

  type Service struct {
      cfg     *config.Service
      log     *slog.Logger
      httpCli *http.Client
  }

  // Enforced：某页面是否强制校验（页面在 captcha_pages 且密钥已配置）
  func (s *Service) Enforced(ctx context.Context, page string) bool {
      provider, _ := s.cfg.Get(ctx, KeyProvider)
      if provider == "off" || provider == "" {
          return false
      }
      pages := s.cfg.GetJSONStringSlice(ctx, KeyPages) // 解析 JSON 数组
      if !slices.Contains(pages, page) {
          return false
      }
      secret, _ := s.cfg.Get(ctx, KeySecretKey)
      if secret == "" {
          // 运行中密钥配置缺失 → 跳过校验兜底并记 warn（Design1 §3.2）
          s.log.Warn("验证码密钥未配置，跳过校验", "page", page, "provider", provider)
          return false
      }
      return true
  }

  // Verify：调用提供商验证接口；recaptcha → https://www.google.com/recaptcha/api/siteverify；
  // turnstile → https://challenges.cloudflare.com/turnstile/v0/siteverify；
  // 入参 secret + response（前端 token），解析 success 字段；网络/解析失败返回 error → 接入层 400
  func (s *Service) Verify(ctx context.Context, page, captchaToken string) error {
      if !s.Enforced(ctx, page) {
          return nil
      }
      if captchaToken == "" {
          return errors.New("请完成验证码校验")
      }
      // POST 提供商验证接口（表单编码），失败返回 error
      ...
  }

  // Middleware：接入层包装，按页面名强制校验（captchaToken 从请求体 captcha_token 字段取）
  func (s *Service) Middleware(page string) gin.HandlerFunc { /* 绑定体校验 → Verify 失败 400 */ }
  ```

  **4. `backend/internal/ratelimit/`（速率限制中间件）**

  ```go
  // 按 IP 固定窗口计数（分钟槽）；阈值存 system_config，每次请求读当前配置（修改立即生效）
  type Limiter struct {
      cfg  *config.Service
      log  *slog.Logger
      mu   sync.Mutex
      buckets map[string]bucket // key = 作用域+IP+分钟槽
  }

  type bucket struct{ count int }

  const (
      KeyLogin    = "ratelimit_login"    // 默认 10/min
      KeyRegister = "ratelimit_register" // 默认 5/min
      KeyForgot   = "ratelimit_forgot"   // 默认 5/min
      // Build2 追加 KeyDownload = "ratelimit_download"，默认 20/min
  )

  func New(cfg *config.Service, lg *slog.Logger) *Limiter {
      return &Limiter{cfg: cfg, log: lg, buckets: map[string]bucket{}}
  }

  // Middleware：作用于登录/注册/找回密码端点（OIDC 回调与当前用户端点不限流，Design1 §5.2）
  func (l *Limiter) Middleware(scope, configKey string, defaultLimit int) gin.HandlerFunc {
      return func(c *gin.Context) {
          ip := c.ClientIP() // 真实客户端 IP：gin 已按 TRUST_PROXY 策略解析转发头
          slot := time.Now().UTC().Truncate(time.Minute)
          limit := l.cfg.GetInt(c.Request.Context(), configKey, defaultLimit)
          l.mu.Lock()
          key := scope + "|" + ip + "|" + slot.Format("200601021504")
          b := l.buckets[key]
          b.count++
          l.buckets[key] = b
          l.gc(slot) // 顺带清理过期槽，防内存泄漏
          l.mu.Unlock()
          if b.count > limit {
              c.Header("Retry-After", strconv.Itoa(int(time.Until(slot.Add(time.Minute)).Seconds())+1))
              server.Fail(c, http.StatusTooManyRequests, "操作过于频繁，请稍后再试")
              c.Abort()
              return
          }
          c.Next()
      }
  }
  ```

  **5. 端点接通（`server/auth.go` 扩展）**

  ```go
  // 注册路由时叠加中间件链（顺序：限流 → 验证码 → 处理器）
  g.POST("/register", limiter.Middleware("register", ratelimit.KeyRegister, 5), captcha.Middleware("register"), h.register)
  g.POST("/login", limiter.Middleware("login", ratelimit.KeyLogin, 10), captcha.Middleware("login"), h.login)
  g.POST("/forgot", limiter.Middleware("forgot", ratelimit.KeyForgot, 5), captcha.Middleware("forgot"), h.forgot)
  g.POST("/reset", h.reset) // 重置凭令牌保护，不额外限流

  func (h *AuthHandler) forgot(c *gin.Context) {
      var req struct{ Email string `json:"email" binding:"required,max=254"` }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      if err := h.resetSvc.Request(c.Request.Context(), req.Email); err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, gin.H{"message": "若该邮箱已注册，重置链接已发送"}) // 统一防枚举响应
  }

  func (h *AuthHandler) reset(c *gin.Context) {
      var req struct {
          Token    string `json:"token" binding:"required,max=256"`
          Password string `json:"password" binding:"required,max=128"`
      }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      if err := h.resetSvc.Complete(c.Request.Context(), req.Token, req.Password); err != nil {
          if errors.Is(err, auth.ErrTokenInvalid) {
              server.Fail(c, http.StatusBadRequest, "重置链接无效或已过期")
              return
          }
          if errors.Is(err, auth.ErrBadRequest) {
              server.Fail(c, http.StatusBadRequest, err.Error())
              return
          }
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, gin.H{"message": "密码已重置，请使用新密码登录"})
  }
  ```

  **6. status 端点扩展**：`captcha_provider`、`captcha_site_key`、`captcha_pages`（供前端渲染；**secret_key 禁止返回**）。

  **7. 前端**

  ```vue
  <!-- CaptchaWidget.vue：按 provider/pages 渲染 reCAPTCHA/Turnstile；脚本加载失败显示明确错误文案（不静默卡死） -->
  <script setup lang="ts">
  import { onMounted, ref, watch } from 'vue'
  import { Alert } from 'ant-design-vue'
  import { useSystemStore } from '@/stores/system'

  const props = defineProps<{ page: 'register' | 'login' | 'forgot' }>()
  const emit = defineEmits<{ 'update:token': [string] }>()
  const system = useSystemStore()
  const loadError = ref(false)

  // 启用判定与后端 Enforced 对齐：provider 非 off 且页面在 captcha_pages
  const enabled = computed(() => {
    const st = system.status
    return !!st && !!st.captcha_provider && st.captcha_provider !== 'off'
      && (st.captcha_pages ?? []).includes(props.page) && !!st.captcha_site_key
  })

  onMounted(() => {
    if (!enabled.value) return
    // 动态加载提供商脚本（recaptcha: api.js / turnstile: api.js），onerror → loadError=true
    const script = document.createElement('script')
    script.src = providerScriptURL(system.status!.captcha_provider!, system.status!.captcha_site_key!)
    script.async = true
    script.onerror = () => { loadError.value = true }
    script.onload = () => renderWidget() // 渲染组件，回调中 emit('update:token', token)
    document.head.appendChild(script)
  })
  </script>

  <template>
    <div v-if="enabled" class="mb-4">
      <Alert v-if="loadError" type="error" message="验证码加载失败，请检查网络后刷新重试" />
      <div v-else id="captcha-container" />
    </div>
  </template>
  ```

  ```vue
  <!-- ForgotView.vue：邮箱输入 → 提交 → a-result 统一提示 -->
  <script setup lang="ts">
  const submitted = ref(false)
  async function onSubmit() {
    try {
      await forgot({ email: form.email })
      submitted.value = true // 无论邮箱是否存在，展示同一提示（防枚举）
    } catch (err) { Notify.error((err as Error).message) }
  }
  </script>
  <template>
    <Result v-if="submitted" status="success" title="若该邮箱已注册，重置链接已发送"
            sub-title="请查收邮件（1 小时内有效）">
      <template #extra><Button type="primary" @click="$router.push('/login')">返回登录</Button></template>
    </Result>
    <Form v-else layout="vertical" @finish="onSubmit">
      <!-- 邮箱输入 + CaptchaWidget(page="forgot") + 提交按钮 -->
    </Form>
  </template>

  <!-- ResetView.vue（/reset/:token）：新密码 + 确认密码 → 提交 → 成功跳 /login -->
  ```

  ```ts
  // api/auth.ts 补充
  export const forgot = (data: { email: string; captcha_token?: string }) =>
    http.post<any, { message: string }>('/auth/forgot', data)
  export const resetPassword = (data: { token: string; password: string }) =>
    http.post<any, { message: string }>('/auth/reset', data)
  ```

  LoginView/RegisterView/ForgotView 表单提交时携带 `captcha_token`（CaptchaWidget 产出）；提交被 429 拦截时按 Retry-After 提示等待。

  **8. 单元测试要点（验收要求）**

  - 重置令牌：一次性（二次 Complete 失败）、TTL 过期拒绝、用后即删（DELETE 而非置位）。
  - 重置成功后旧会话 401（credential_version 递增）。
  - 防枚举：存在/不存在邮箱的 forgot 响应一致。
  - 限流：固定窗口计数（第 N+1 次 429 + Retry-After 头存在）；阈值从配置实时读取（改配置立即生效）。
  - TRUST_PROXY 三档 IP 解析：auto 信任回环转发头、off 忽略转发头（用 httptest 伪造 X-Forwarded-For 验证 c.ClientIP()）。
  - 验证码：Enforced 在密钥缺失时返回 false 并记 warn；provider=off 时 Middleware 直接放行。

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
