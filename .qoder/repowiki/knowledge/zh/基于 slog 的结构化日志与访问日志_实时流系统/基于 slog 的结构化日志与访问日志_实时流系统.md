---
kind: logging_system
name: 基于 slog 的结构化日志与访问日志/实时流系统
category: logging_system
scope:
    - '**'
source_files:
    - backend/internal/log/log.go
    - backend/internal/log/stream.go
    - backend/internal/log/access.go
    - backend/internal/server/log.go
    - backend/cmd/server/main.go
---

## 1. 使用的框架与整体方案

后端采用 Go 标准库 `log/slog` 作为结构化日志框架，通过自定义 Handler 链实现分级输出、token 脱敏、内存环形缓冲与 SSE 实时推送。日志同时支持 console（Text）与 JSON 两种格式，全部输出到 stdout，便于容器化采集。

核心设计：
- 包级默认 logger + 可注入的 `*slog.Logger`：业务服务一律通过依赖注入获取 logger，仅日志设施自身例外使用包级默认实例。
- 运行时级别切换：内部以 `slog.LevelVar` 持有全局级别变量，暴露 `SetLevel("debug"|"info"|"warn"|"error")`，调用方立即生效（由配置键持久化驱动）。
- 统一脱敏：外层 `RedactHandler` 包装任意下游 handler，对消息与所有 string 类型属性中的 `?token=xxx` / `&token=xxx` 查询参数值替换为 `***`，遵循 AGENTS §4.3 约束。
- 双通道输出：`New(level, format, bufs...)` 根据 format 选择 `JSONHandler` 或 `TextHandler`；可选传入 `*RingBuffer`，经 `RingHandler` 将同一条日志同时写入 stdout 和内存缓冲，供 SSE 消费。

## 2. 关键文件与职责

- `backend/internal/log/log.go`：logger 工厂、包级默认实例、`SetLevel`、`Redact`/`RedactHandler` 脱敏逻辑。
- `backend/internal/log/stream.go`：`RingBuffer`（固定容量 500 条，满覆盖最旧）、`RingHandler`、`StreamService`（短期 Token 签发/消费、SSE 连接上限 8、一次性 token TTL 5 分钟、Reset 一键清空）。
- `backend/internal/log/access.go`：`AccessService` 提供按日期范围分页查询与清空 `access_logs` 表的能力，记录口径遵循 Build2 Step 4（显式/自定义 Token 记订阅标识，无标识记解析出的订阅标识，失败记平台标识）。
- `backend/internal/server/log.go`：Gin 路由注册 `/api/admin/logs/*`，包含访问日志查询、清空、SSE Token 签发与 SSE 流端点；SSE 认证采用一次性短期 Token（EventSource 无法带 Header）。
- `backend/cmd/server/main.go`：启动时读取 `LOG_LEVEL`、`LOG_FORMAT` 环境变量，构造 logger 并注入各服务；错误路径统一用 `log.Error` 记录。

## 3. 架构与约定

- **分层**：基础设施层（`internal/log`）→ 接入层（`internal/server/log.go`）→ 业务域（各 `internal/*/service` 通过注入的 `*slog.Logger` 记录）。`AccessService` 直接依赖 `*sql.DB` 而非 `store.Store`，避免 `store → log` 循环依赖。
- **字段规范**：`Entry` 结构体定义 SSE 推送的 JSON 字段 `time`/`level`/`message`/`attrs`（预格式化的 `key=value` 串），保证前端 `LogsView.vue` 能一致渲染。
- **安全约束**：
  - token 脱敏是强制约束（代码注释标注“关键约束 AGENTS §4.3”），所有 string 属性均经正则 `([?&]token=)[^&\s]*` 替换。
  - SSE 连接限制：全局最多 8 个活跃连接，超限返回 429；Token 单次使用即删，未使用 5 分钟 TTL 自动清理。
- **运维能力**：访问日志支持按 `from`/`to`（YYYY-MM-DD）+ `page`/`size` 分页查询，默认 20 条/页，最大 100；`Clear` 清空全表；`StreamService.Reset` 在运行态重置 Token、缓冲与全部活跃连接。

## 4. 约定与约束总结

- 业务模块不得直接使用包级 `log.Info/Warn/Error/Debug`，必须通过注入的 `*slog.Logger` 记录，以保证测试可替换、实例可隔离。
- 日志级别通过 `SetLevel` 运行时切换，禁止硬编码级别常量；默认 `info`，支持 `debug`/`warn`/`error`。
- 所有字符串型日志属性中的 token 查询参数必须被脱敏，这是由 `RedactHandler` 在 handler 链外层统一保证的，调用方无需手动处理。
- 访问日志记录口径严格遵循：显式/自定义 Token → 订阅标识；无标识 Token → 解析出的订阅标识；解析失败 → 平台标识；90 天定时清理由 cron 任务负责。
- SSE 流认证不使用 Authorization 头，改用一次性短期 Token 通过 URL 参数传递，连接建立后立即删除该 token。
- 日志输出目标固定为 stdout，格式由 `LOG_FORMAT` 控制（`json` 或 `console`），适配 Docker 环境下的日志收集器。