---
kind: error_handling
name: Go/Gin 后端统一错误处理体系：领域哨兵错误 + 中间件 panic 恢复 + 响应脱敏
category: error_handling
scope:
    - '**'
source_files:
    - backend/internal/response/response.go
    - backend/internal/server/server.go
    - backend/internal/server/auth.go
    - backend/internal/store/store.go
    - backend/internal/auth/reset.go
    - backend/internal/group/group.go
    - backend/internal/platform/platform.go
    - backend/internal/rule/rule.go
    - backend/internal/config/admin.go
    - backend/internal/download/download.go
    - backend/internal/setup/setup.go
    - frontend/src/api/request.ts
---

## 1. 整体方案

后端基于 Gin 框架，采用「业务层返回哨兵 error → Handler 用 `errors.Is` 匹配 → 调用 `response.OK/Fail` 输出统一 JSON」的分层错误模型；全局通过 Gin 中间件 `panicRecovery()` 兜底捕获 panic，并统一将 5xx 对外消息脱敏为「服务器内部错误」（调试模式可开启详细详情）。

## 2. 关键文件与包

- `backend/internal/response/response.go`：定义统一响应结构 `Response{Code, Message, Data}`、列表包装 `ListData`，以及 `OK` / `Fail` 两个写入函数。`Fail` 在 `httpStatus >= 500` 时记录结构化日志并通过 `debugProvider`（由 server 启动时注入 `debug_mode` 配置）决定是否对外暴露真实错误信息。
- `backend/internal/server/server.go`：装配入口，注册全局中间件链 `requestLogger()` + `panicRecovery()`；提供 `Server.New` 与 `NewEmergency` 两套引擎构造。`panicRecovery` 使用 `defer recover()` 捕获 panic，记录 `log.Error("panic 恢复", ...)` 后以 500 + 通用消息结束请求。
- 各业务域包（`auth/`, `group/`, `platform/`, `rule/`, `share/`, `custom/`, `config/`, `download/`, `setup/`, `approval/` 等）：集中声明包级哨兵错误变量（如 `ErrNotFound`、`ErrBadRequest`、`ErrTokenInvalid`、`ErrAlreadyConfigured`、`ErrModeRestricted`、`ErrAuthDeadlock`、`ErrSubInSelection` 等），供上层 Handler 做语义化分支。
- `backend/internal/store/store.go`：数据访问层封装 SQLite，提供 `TxImmediate`（BEGIN IMMEDIATE 事务）和 `IsNoRows` 辅助；迁移失败直接返回 error 阻止启动，保证数据库版本一致性。
- `backend/internal/server/auth.go` 等 Handler 文件：示例展示错误传播路径——调用业务 Service 返回 error → `errors.Is(err, user.ErrEmailConflict)` 等判断 → 调用 `server.Fail(c, http.StatusConflict, "该邮箱已被注册")`。

## 3. 架构与约定

### 3.1 错误类型分层
- **业务哨兵错误**：每个 domain 包以 `var ErrXxx = errors.New(...)` 形式导出语义化错误常量，便于跨包比较（如 `user.ErrEmailConflict`、`auth.ErrTokenInvalid`、`group.ErrDefaultGroup`）。
- **底层错误透传**：DB/网络/序列化等底层错误通过 `fmt.Errorf("...: %w", err)` 包装向上返回，Handler 仅对已知的业务哨兵错误做精确分支，其余一律按 500 处理。
- **HTTP 状态码约定**：400（参数校验失败）、401（认证失败/会话过期/账号未激活）、403（权限不足/本地登录关闭）、409（冲突，如邮箱重复/标识冲突）、429（限流）、500（内部错误）。这些状态码与 `Response.Code` 字段保持一致。

### 3.2 中间件与全局兜底
- `requestLogger()`：记录 method/path/status/latency_ms，路径中的 `?token=` 值经 slog 脱敏处理器过滤。
- `panicRecovery()`：Gin 全局 panic 恢复，避免进程崩溃；panic 堆栈仅入日志，对外一律返回 500 + 通用消息。
- 限流中间件 `ratelimit.Middleware`：超限返回 429。
- 验证码中间件 `captcha.Middleware`：校验失败返回 400。
- 应急模式 `emergencyGate()`：非正常模式下拦截业务路由返回 503。

### 3.3 响应脱敏策略
`response.Fail` 对 5xx 响应进行脱敏：生产环境对外固定为「服务器内部错误」，仅在 `debug_mode=true` 时通过 `debugProvider` 注入的回调返回真实错误详情。该机制在 `server.New` 中通过 `response.SetDebugProvider` 接通配置服务。

### 3.4 事务与并发安全
- `store.TxImmediate` 强制「读→判定→写」串行化，防止竞态条件导致的业务错误。
- SQLite 单写者模型（`MaxOpenConns=1`）+ WAL + busy_timeout，避免并发写冲突。
- 迁移失败直接返回 error 阻止启动，禁止进入半迁移状态。

## 4. 约束与规则

- **禁止直接使用 `gin.Default()`**：server 使用 `gin.New()` 再手动挂载中间件，确保日志脱敏与 panic 恢复不被默认 logger/recovery 绕过（见 `server.go` 注释）。
- **所有 HTTP 响应必须经 `response.OK` / `response.Fail`**：避免 Handler 直接调用 `c.JSON` 导致响应格式不一致或遗漏脱敏逻辑。
- **业务错误必须使用哨兵 error + `errors.Is` 分支**：Handler 不得直接比较 error 字符串；新增业务错误应在对应 domain 包顶部声明 `var ErrXxx = errors.New(...)`。
- **5xx 对外不可泄露细节**：除非显式开启 `debug_mode`，否则 `response.Fail` 会覆盖原始错误消息。
- **panic 不向上传播**：`panicRecovery` 是最后防线，任何未捕获 panic 都会降级为 500 + 通用消息，不会让客户端看到 Go 堆栈。
- **无记录场景统一映射**：DB 查询无结果通过 `sql.ErrNoRows` 检测（`store.IsNoRows` 或 `errors.Is(err, sql.ErrNoRows)`），业务层将其转换为对应的 `ErrNotFound` 哨兵错误。
- **前端 API 层错误处理**：前端 `src/api/request.ts` 统一拦截响应，根据 `code` 字段判断成功/失败，并在失败时通过 `Notify` 组件弹出用户可读提示（如「操作过于频繁，请稍后再试」）。

## 5. 适用性说明

该仓库为 Go + Gin 后端 + Vue 前端的订阅管理平台，具备完整的错误定义、传播、兜底与呈现链路，因此本类别完全适用。