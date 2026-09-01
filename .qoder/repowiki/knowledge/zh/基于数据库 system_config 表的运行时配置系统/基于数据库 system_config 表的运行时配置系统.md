---
kind: configuration_system
name: 基于数据库 system_config 表的运行时配置系统
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
本项目的配置系统采用「环境变量 + 数据库持久化」双层架构：启动期通过 `os.Getenv` 读取少量进程级参数（APP_MODE、LOG_LEVEL、LOG_FORMAT、PORT、TRUST_PROXY、DATA_DIR、RESET_ADMIN_PASSWORD），其余所有业务可配置项统一存储在 SQLite 的 `system_config` 表中，以 key/value 形式按分区（OIDC、本地认证、验证码、SMTP、站点信息、限流、日志级别、公告/页脚、调试等）管理。配置服务位于 `backend/internal/config`，由 `cmd/server/main.go` 在启动时装配并注入各业务模块。

## 2. 核心文件与职责
- `backend/internal/config/config.go`：基础 Service，提供 Get/Set/GetBool/GetInt/GetJSONStringSlice 等读写 API；实现敏感键 AES-256-GCM 加解密（签名密钥经 HKDF-SHA256 派生）、事务内读写（GetTx/SetTx/EncryptWithTx/EnsureSigningKeyTx）、签名密钥自动生成与获取。
- `backend/internal/config/admin.go`：AdminService 面板配置服务，按功能分区封装 OIDC、本地认证、验证码、SMTP、站点信息、限流、日志级别、公告/页脚、调试等配置的 GET/PUT 校验与落库逻辑，含防认证死锁、白名单、长度限制等业务约束。
- `backend/internal/config/export.go`：ExportService 提供生产模式下的配置导出/导入：导出用 Argon2id 派生密钥 + AES-256-GCM 加密整个配置文件（含全部 system_config 行与站点 ICON base64），导入为事务内严格整体覆盖（先 DELETE 再 INSERT）。
- `backend/cmd/server/main.go`：入口解析环境变量，初始化日志、数据库迁移，装配 config.Service 并写入运行模式，检测应急模式后启动 HTTP 服务。
- `backend/internal/mail/mail.go`：通过 `config.RegisterSensitive("smtp_password")` 将 SMTP 密码登记为敏感键，体现「业务包自行注册敏感键」的约定。
- `backend/migrations/*.sql`：`system_config` 表结构定义（key/value 两列）。

## 3. 架构与设计约定
- **配置存储**：所有业务配置集中存于 `system_config` 表，无 YAML/TOML/env 文件持久化；环境变量仅用于进程启动参数。
- **敏感数据保护**：通过 `RegisterSensitive(key)` 显式登记敏感键，Set 自动 AES-256-GCM 加密、Get 自动解密；未登记则明文存取。密文格式为 `base64url(nonce ‖ ciphertext)`，密钥由 `signing_key` 经 HKDF-SHA256 固定 info 派生。
- **事务安全**：提供 `GetTx/SetTx/EncryptWithTx/EnsureSigningKeyTx`，供 Setup/OIDC Setup 等多键原子写入场景复用同一事务中的签名密钥，避免连接池死锁。
- **类型化读取**：GetBool/GetInt/GetJSONStringSlice 在解析失败时记录 warn 日志并回退默认值，保证健壮性。
- **分区化 API**：AdminService 按功能域划分方法（GetOidc/SaveOidc、GetLocalAuth/SaveLocalAuth、GetCaptcha/SaveCaptcha、GetSMTP/SaveSMTP、GetRateLimit/SaveRateLimit 等），每个分区独立校验与落库。
- **导出/导入**：仅 prod 模式开放；导出包含全部 system_config 与站点 ICON；导入为严格整体覆盖，未知键忽略但告警，format_version 不匹配仅警告。
- **前端地址缓存语义**：`frontend_url` / `callback_url` 修改后需重启容器生效，UI 会提示「立即重启容器 → 再重新登录」。

## 4. 约定与约束
- **环境变量白名单**：仅允许 APP_MODE(dev|prod)、LOG_LEVEL、LOG_FORMAT、PORT、TRUST_PROXY、DATA_DIR、RESET_ADMIN_PASSWORD，其他值被忽略。
- **敏感键必须登记**：新增敏感配置需在业务包 init 中调用 `config.RegisterSensitive(key)`，否则不会自动加解密。
- **防认证死锁**：当本地登录关闭且 OIDC 不可用时，禁止保存任何使认证完全不可用的配置（AdminService 返回 `ErrAuthDeadlock`）。
- **验证码密钥强制配置**：启用 recaptcha/turnstile 且勾选页面时必须同时配置 site_key 与 secret_key，否则拒绝保存。
- **ICON 上传限制**：仅允许 png/jpeg/jpg/webp/ico，最大 2MB，路径固定 `/public/site/icon.*`，文件名不变，通过版本号查询参数 `?v=N` 防缓存。
- **公告/页脚长度限制**：首页公告、登录页公告、登录页页脚均 ≤2000 字符，前端使用 markdown-it html:false 渲染，禁用原始 HTML 防 XSS。
- **限流值为正整数**：SaveRateLimit 要求 Login/Register/Forgot/Download > 0。
- **日志级别实时生效**：SetLogLevel 同时更新持久化值与全局 log.LevelVar，无需重启。
- **导出密码最小长度**：≥8 字符，使用 Argon2id(time=1, memory=64MB, threads=4) 派生密钥。
- **导入确认词**：必须传入固定字符串 `IMPORT`，二次确认由前端负责。
- **应急模式降级**：数据库无法打开或迁移失败时自动进入应急模式，仅暴露状态/站点信息/应急端点，config.Get 对 nil store 做空守卫返回未设置。