---
kind: configuration_system
name: 基于 system_config 表的运行时配置系统（含敏感字段加密与导入导出）
category: configuration_system
scope:
    - '**'
source_files:
    - backend/internal/config/config.go
    - backend/internal/config/admin.go
    - backend/internal/config/export.go
    - backend/cmd/server/main.go
    - backend/internal/mail/mail.go
---

## 1. 总体方案

本项目的配置系统采用「环境变量 + 数据库持久化」的分层模式：
- **启动期配置**：通过 `os.Getenv` 读取 `APP_MODE`、`LOG_LEVEL`、`LOG_FORMAT`、`PORT`、`TRUST_PROXY`、`DATA_DIR`、`RESET_ADMIN_PASSWORD` 等环境变量，由 `cmd/server/main.go` 中的 `envOr` 提供默认值。
- **运行期配置**：所有业务可变的系统配置统一持久化到 SQLite 的 `system_config` 表（key/value 键值对），由 `internal/config` 包提供的 `Service` 暴露 Get/Set/GetBool/GetInt/GetJSONStringSlice 等类型化 API。
- **面板配置分区**：`admin.go` 按功能域划分 OIDC、本地认证、验证码、SMTP、站点信息、限流、日志级别、公告/页脚、调试等分区，每个分区提供 GetXxx/SaveXxx 成对接口，负责参数校验、安全约束与落库。
- **配置导入导出**：`export.go` 在 Production 模式下提供完整配置的加密导出（Argon2id 派生密钥 + AES-256-GCM）与事务内整体覆盖导入，支持迁移整个实例的配置状态。

## 2. 核心文件与职责

| 文件 | 职责 |
|---|---|
| `backend/internal/config/config.go` | 基础配置 Service：DB 读写、敏感字段加解密（AES-256-GCM）、签名密钥管理、类型化读取 |
| `backend/internal/config/admin.go` | 面板配置服务 AdminService：各分区配置读写、业务规则校验（防认证死锁、验证码密钥缺失等） |
| `backend/internal/config/export.go` | 配置导入导出 ExportService：生产模式专属，加密打包全部 system_config 与站点 ICON |
| `backend/cmd/server/main.go` | 入口：解析环境变量、初始化日志、打开 DB、执行迁移、检测应急模式、装配 HTTP 服务 |
| `backend/internal/mail/mail.go` | 示例展示如何调用 `config.RegisterSensitive("smtp_password")` 将 SMTP 密码登记为敏感键 |

## 3. 架构与设计约定

### 3.1 配置存储模型
- 所有配置以 key/value 字符串形式存入 `system_config` 表；布尔/整数/JSON 数组通过 `GetBool`/`GetInt`/`GetJSONStringSlice` 在读取时解析，失败则记录 warn 日志并回退默认值。
- 配置键集中定义在 `config.go`（如 `KeyConfigured`、`KeySigningKey`、`KeyLogLevel`、`KeyAppMode`、`KeyAllowLocalLogin` 等）与 `admin.go`（OIDC、验证码、限流相关键）中，避免散落的字面量。

### 3.2 敏感配置保护
- 敏感键通过 `RegisterSensitive(key)` 显式注册（当前已知：`smtp_password`，由 `mail` 包在 init 时注册；OIDC Client Secret 由 oidc 包自行处理）。
- 写入时自动经 `EncryptWithKey` 使用基于 HKDF-SHA256 从签名密钥派生的 AES-256-GCM 密钥加密；读取时自动解密，解密失败返回错误而非静默降级。
- 签名密钥 `signing_key` 明文落库，由 Setup 或 `EnsureSigningKey` 生成 32 字节随机值；导出时连同其他配置一并加密打包。

### 3.3 事务一致性
- 提供 `GetTx`/`SetTx`/`GetSigningKeyTx`/`EncryptWithTx` 等事务内版本，确保多键原子写入（如 Setup 快速开始、导入时的整体覆盖）。
- 导入操作先 `DELETE FROM system_config` 再逐条插入，实现「严格整体覆盖」语义——导出文件中不存在的键会被清除。

### 3.4 安全约束
- **防认证死锁**：当 `allow_local_login=false` 且 OIDC 不可用时，禁止保存任何会进一步禁用认证的配置（`ErrAuthDeadlock`）。
- **验证码密钥校验**：启用验证码但未配置 site_key/secret_key 时拒绝保存（`ErrCaptchaKeyMissing`）。
- **ICON 上传限制**：仅允许 png/jpeg/jpg/webp/ico，≤2MB，排除 SVG/GIF 以防存储型 XSS。
- **公告/页脚长度限制**：≤2000 字符，前端以 markdown-it html:false 渲染，原始 HTML 被转义。
- **导入导出仅限 prod 模式**：非 prod 调用返回 `ErrModeRestricted`。

### 3.5 运行时生效
- 日志级别通过 `log.SetLevel` 实时切换，无需重启。
- 限流阈值每次请求重新读取配置，修改立即生效。
- 站点 ICON 更新后递增 `site_icon_version`，前端引用带 `?v=版本号` 避免缓存。
- 导入后替换签名密钥导致全部会话失效，需提示管理员重启容器并重新登录。

## 4. 约定与约束总结

- 新增业务开关一律以 key/value 形式持久化到 `system_config`，并通过 `config.Service` 的 GetXxx API 访问，禁止直接查库。
- 需要加密存储的敏感字段必须先调用 `config.RegisterSensitive(key)` 登记，再由 `Set` 自动加密；未登记的键不会触发加密。
- 面板配置变更必须经过对应分区的 SaveXxx 方法，由该方法完成参数校验与安全约束检查。
- 环境变量仅用于启动期参数（模式、端口、数据目录、日志级别等），业务配置不依赖 .env 文件。
- 配置导入导出仅在 `APP_MODE=prod` 下可用，导出文件以 Argon2id + AES-GCM 加密，导入时要求确认词 `IMPORT`。
- 所有配置读取失败均走「默认值 + warn 日志」策略，保证服务可用性优先于配置完整性。