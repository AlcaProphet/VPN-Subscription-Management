---
kind: error_handling
name: Go 后端统一错误处理体系：领域哨兵错误 + Gin 中间件 + 响应脱敏
category: error_handling
scope:
    - '**'
source_files:
    - backend/internal/response/response.go
    - backend/internal/server/server.go
    - backend/internal/server/auth.go
    - backend/internal/user/user.go
    - backend/internal/auth/reset.go
    - backend/internal/config/admin.go
    - backend/internal/group/group.go
    - backend/internal/platform/platform.go
    - backend/internal/rule/rule.go
    - backend/internal/share/share.go
    - backend/internal/custom/custom.go
    - backend/internal/download/download.go
    - backend/internal/approval/approval.go
    - backend/internal/setup/setup.go
    - backend/internal/subscription/subscription.go
---

## 1. 整体方案

后端基于 **Gin** 框架，采用「业务层返回哨兵 `error` → Handler 层用 `errors.Is` 匹配 → 调用 `response.OK/Fail` 输出统一 JSON」的分层错误处理模式。所有 HTTP 响应通过 `internal/response` 包的 `Response{Code, Message, Data}` 结构体返回，成功统一为 `{code:0, data:...}`，失败统一为 `{code:httpStatus, message:...}`。

关键设计点：
- 不使用全局 panic/recover 作为业务错误通道；panic 仅由 `panicRecovery()` 中间件兜底，捕获后记录日志并以 500 + 通用消息返回。
- 5xx 响应对外默认脱敏为「服务器内部错误」，仅在配置 `debug_mode=true` 时透出真实错误详情（通过 `response.SetDebugProvider` 注入判定函数）。
- 请求级日志经 `requestLogger()` 中间件统一记录 method/path/status/latency，敏感字段（如 `?token=`）由 slog 的脱敏 Handler 过滤。

## 2. 核心文件与职责

| 文件 | 职责 |
|---|---|
| `backend/internal/response/response.go` | 定义 `Response`/`ListData` 结构、`OK`/`Fail` 助手、调试模式开关 |
| `backend/internal/server/server.go` | 装配 Gin 引擎、注册 `requestLogger` + `panicRecovery` 中间件、提供 `OK`/`Fail` 别名 |
| `backend/internal/server/auth.go` | 认证路由 Handler，示范 `errors.Is` 匹配领域错误并映射到具体 HTTP 状态码 |
| `backend/internal/user/user.go` | 用户服务，定义 `ErrEmailConflict`/`ErrAuthFailed`/`ErrAccountInactive`/`ErrBadRequest` 等哨兵错误 |
| `backend/internal/auth/reset.go` | 重置令牌服务，定义 `ErrTokenInvalid`/`ErrBadRequest` |
| `backend/internal/config/admin.go` | 配置管理，定义 `ErrAuthDeadlock`/`ErrCaptchaKeyMissing`/`ErrBadRequest` |
| `backend/internal/group/group.go` | 组服务，定义 `ErrNameConflict`/`ErrDefaultGroup`/`ErrSubInSelection`/`ErrSubNotLinked`/`ErrNotFound` |
| `backend/internal/platform/platform.go` | 平台服务，定义 `ErrBadRequest`/`ErrNotFound`/`ErrInstallerTooLarge` |
| `backend/internal/rule/rule.go` | 规则服务，定义 `ErrBadRequest`/`ErrNotFound` |
| `backend/internal/share/share.go` | 分享服务，定义 `ErrBadRequest`/`ErrNotFound` |
| `backend/internal/custom/custom.go` | 自定义订阅服务，定义 `ErrBadRequest`/`ErrNotFound` |
| `backend/internal/download/download.go` | 下载服务，定义 `ErrTokenInvalid`/`ErrUnassigned`（后者用于 200 + 注释块） |
| `backend/internal/approval/approval.go` | 审批服务，定义 `ErrNotFound`，并用 `errors.Is(err, sql.ErrNoRows)` 转换为业务哨兵错误 |
| `backend/internal/setup/setup.go` | 初始化服务，定义 `ErrAlreadyConfigured` |
| `backend/internal/subscription/subscription.go` | 订阅服务，定义 `ErrSlugConflict` |

## 3. 架构与约定

### 3.1 领域哨兵错误（Sentinel Errors）
每个业务包以包级变量声明语义化哨兵错误，例如：
- `user.ErrEmailConflict`、`user.ErrAuthFailed`、`user.ErrAccountInactive`
- `auth.ErrTokenInvalid`、`auth.ErrBadRequest`
- `group.ErrNameConflict`、`group.ErrDefaultGroup`、`group.ErrNotFound`
- `platform.ErrInstallerTooLarge`、`platform.ErrNotFound`
- `rule.ErrNotFound`、`share.ErrNotFound`、`custom.ErrNotFound`
- `download.ErrTokenInvalid`、`download.ErrUnassigned`
- `setup.ErrAlreadyConfigured`、`config.ErrAuthDeadlock`、`config.ErrCaptchaKeyMissing`

这些错误是跨层契约：Handler 层通过 `errors.Is(err, user.ErrEmailConflict)` 判断并返回对应 HTTP 状态码（409/401/400），业务层不感知 HTTP 细节。

### 3.2 数据库错误转换
底层 `sql.ErrNoRows` 在业务层被显式转换为领域哨兵错误（如 `approval` 中查不到记录返回 `ErrNotFound`），避免上层依赖 SQL 实现细节。

### 3.3 Handler 层错误映射
以 `server/auth.go` 为例：
```go
if errors.Is(err, user.ErrAuthFailed) {
    Fail(c, http.StatusUnauthorized, "邮箱或密码错误") // 统一措辞，防枚举
}
if errors.Is(err, user.ErrAccountInactive) {
    Fail(c, http.StatusUnauthorized, "账号未激活或已被禁用")
}
```
所有非预期错误最终落入 `Fail(c, http.StatusInternalServerError, err.Error())`，再由 `response.Fail` 根据 `debug_mode` 决定是否暴露内部详情。

### 3.4 中间件链
Gin 引擎使用 `engine.New()`（而非 `gin.Default()`），手动挂载：
1. `requestLogger()` — 记录请求方法、路径、状态码、耗时，路径参数经 slog 脱敏
2. `panicRecovery()` — recover panic，记录日志并以 500 + 「服务器内部错误」返回
3. 业务中间件（限流 `ratelimit.Middleware`、验证码 `captcha.Middleware`、会话/管理员鉴权中间件）
4. Handler

应急模式（`NewEmergency`）额外挂载 `emergencyGate()`，拦截业务 API 返回 503。

### 3.5 前端错误处理
前端 `frontend/src/api/request.ts` 封装 axios 请求，统一处理后端返回的 `{code, message, data}` 结构，将非 0 code 转为 Promise reject，供各视图组件 catch 展示。

## 4. 约束与规范

- **禁止直接 `fmt.Errorf("...")` 作为可匹配错误**：业务层应返回预定义的哨兵错误，或使用 `%w` 包装后再用 `errors.Is` 匹配（如 `fmt.Errorf("%w: %v", ErrBadRequest, err)`）。
- **HTTP 状态码与业务码同步**：`response.Fail` 同时设置 HTTP 状态码和 `Response.Code`，前端据此区分客户端错误（4xx）与服务端错误（5xx）。
- **防枚举安全**：登录失败统一返回 `ErrAuthFailed`（不区分邮箱不存在/密码错误），重置接口返回「若该邮箱已注册，重置链接已发送」（不泄露邮箱是否存在）。
- **5xx 脱敏**：生产环境 `debug_mode=false` 时，`response.Fail` 将 5xx 的 message 强制替换为「服务器内部错误」，真实错误仅写入 slog。
- **无全局状态**：`response.debugProvider` 通过 `SetDebugProvider` 在 `server.New` 中注入，避免包级全局变量持有服务实例。
- **事务内避免二次取连接**：`user.defaultGroupIDTx` 等函数在事务内直接读配置键，注释明确说明「事务内禁止经 store.DB() 二次取连接（MaxOpenConns=1 会死锁）」。
- **可选依赖失败不阻断主流程**：欢迎邮件、SMTP 发送等通过注入 nil-safe 回调实现，失败仅记录 warn 日志，不向上抛出错误。