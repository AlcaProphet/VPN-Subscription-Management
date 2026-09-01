---
kind: logging_system
name: 基于 slog 的结构化日志系统（分级、脱敏、SSE 实时流）
category: logging_system
scope:
    - '**'
source_files:
    - backend/internal/log/log.go
    - backend/internal/log/stream.go
    - backend/internal/log/access.go
    - backend/cmd/server/main.go
---

## 1. 使用的框架与工具

- 标准库 `log/slog`：整个后端统一使用 Go 1.21+ 内置结构化日志，无第三方日志库。
- 自定义 Handler 链：通过组合 `slog.Handler` 实现 token 脱敏（`RedactHandler`）、内存环形缓冲 + SSE 推送（`RingHandler` + `RingBuffer`）。
- 输出目标：默认 `os.Stdout`；JSON 或 Text 两种格式由环境变量控制。
- 访问日志持久化：独立 `access_logs` 表，通过 `AccessService` 提供按日期范围分页查询与清空能力。

## 2. 核心文件与职责

| 文件 | 职责 |
|---|---|
| `backend/internal/log/log.go` | 包级默认 logger、`New/SetLevel` 工厂、token 脱敏 `RedactHandler`、包级 `Info/Warn/Error/Debug` 快捷函数 |
| `backend/internal/log/stream.go` | 内存环形缓冲（最近 500 条）、`RingHandler`、短期 Token 签发与校验、SSE 连接管理（全局 8 连接上限）、`StreamService` |
| `backend/internal/log/access.go` | 访问日志的数据库读写（`AccessService`），含日期范围解析、分页、清空 |
| `backend/cmd/server/main.go` | 启动时创建 `RingBuffer` → `log.New(LOG_LEVEL, LOG_FORMAT, logBuf)` → `SetDefault` → 注入各 Service |

## 3. 架构与约定

### 3.1 初始化流程（单例 + 依赖注入）

- 入口 `main.go` 读取环境变量 `LOG_LEVEL`（默认 `info`）和 `LOG_FORMAT`（默认 `console`），构造一个带环形缓冲的 logger，并通过 `log.SetDefault` 设为包级默认。
- 业务模块通过构造函数注入 `*slog.Logger`（如 `user.NewService(st, cfg, logger)`），避免全局状态耦合；仅日志设施自身例外使用包级默认。
- 测试中通过 `log.New("error", "console")` 构造低级别 logger 以抑制输出。

### 3.2 日志级别与运行时切换

- 支持 `debug / info / warn / error` 四级，通过 `slog.LevelVar` 实现运行时切换：调用 `log.SetLevel(level)` 后立即生效，无需重启。
- 默认级别为 `info`；生产环境通常保持该值，调试时临时调至 `debug`。

### 3.3 输出格式

- `LOG_FORMAT=console`：使用 `slog.NewTextHandler`，人类可读。
- `LOG_FORMAT=json`：使用 `slog.NewJSONHandler`，便于 ELK/Fluentd 等收集。
- 无论哪种格式，消息与字符串属性都会经过 `RedactHandler` 统一处理。

### 3.4 Token 脱敏（安全约束）

- `RedactHandler` 在每条记录上扫描消息与所有 `string` 类型属性，将 URL 查询参数中的 `?token=xxx` / `&token=xxx` 替换为 `***`。
- 注释明确引用 AGENTS §4.3，作为强制约束：任何包含 token 的日志输出必须经此处理器，防止敏感信息泄露到 stdout。

### 3.5 实时日志流（SSE）

- `RingBuffer` 维护最近 500 条日志，满后覆盖最旧；广播给活跃订阅者采用非阻塞 `select default`，消费慢则丢弃，保证不阻塞日志管道。
- `StreamService` 签发一次性短期 Token（≥128 位 base64.RawURLEncoding，TTL 5 分钟，用后即删），限制全局最多 8 个 SSE 连接。
- 前端通过 `/api/log/stream` 获取短期 Token 并建立 SSE 连接，先接收历史快照再接收实时增量。
- 重置运行时状态时会 `Reset()` 清空 Token、缓冲与全部活跃连接。

### 3.6 访问日志持久化

- 资源标识记录口径：显式/自定义 Token 记订阅标识；无标识 Token 记解析出的订阅标识；解析失败（unassigned）记平台标识。
- 90 天自动清理由 cron 后台任务负责（`cron.StartAccessLogCleanup`），前端提供按日期范围分页查询与清空操作。

## 4. 约定与约束

- **所有日志走 slog**：禁止直接使用 `fmt.Println` / `log.Print` 等业务日志；仅基础设施错误可写 stderr。
- **结构化字段**：通过 `slog.With(...)` 传入键值对（如 `"user_id", id`、`"err", err`），而非拼接字符串。
- **Token 必脱敏**：任何可能携带 `?token=` 的 URL 或字符串都必须经 `RedactHandler`（外层包裹保证 stdout 与缓冲内容均脱敏）。
- **级别可控**：通过 `LOG_LEVEL` 环境变量或 `SetLevel` 运行时调整，测试中统一降级为 `error`。
- **SSE 连接上限**：全局最多 8 个并发日志流连接，超出返回 false，接入层应提示“连接数已达上限”。
- **环形缓冲容量**：固定 500 条，超过后左移覆盖最旧；当底层切片容量膨胀到缓冲大小 4 倍以上时执行紧凑化拷贝，防止内存缓慢增长。
- **访问日志时间**：存储与查询统一使用 UTC，日期格式限定为 `YYYY-MM-DD`，空串表示不限。
- **依赖注入方向**：`AccessService` 直接依赖 `*sql.DB` 而非 `store.Store`，避免 `store→log` 循环依赖。