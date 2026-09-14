# Issue16.md — VPN 订阅管理系统邮件测试与接入问题（当前）

> **文档定位：** 记录 Build11 邮件人工核查的问题、2026-09-14 SMTP 实测发现的缺陷，以及用户已确认的邮件接入范围。R30-02～R30-04 已获单独修复授权并实施；第二节平台接入仍是未实施的研究方案。
> 关联测试清单：[ProdTestList.md](ProdTestList.md)；编码约束：[AGENTS.md](AGENTS.md)（唯一强要求）。

---

## 一、已确认事实与进行中问题

### R30-01 Build11 邮件相关人工测试未完成

- **关联项目：** [ProdTestList.md](ProdTestList.md) §四的密码重置链接 `valid / missing / used / expired` 四态，以及已使用、过期链接不得渲染密码表单。
- **2026-09-14 实测：** 用户确认通用 SMTP 使用腾讯云 SES `smtp.qcloudmail.com:465`、TLS 直连和正确的 SMTP 专用密码后，测试邮件发送通过。这只证明该次测试邮件发送成功，不等于重置链接四态、收件箱投递或其他邮件类型已验收。
- **待验收：** 密码重置邮件的启用范围、触发、到达、链接四态和表单行为仍按 ProdTestList 单独核验；Mailgun 真实账号测试未进行。
- **状态：** ◧ 测试邮件单项通过；Build11 邮件人工核查未完成。

### R30-02 脱敏占位符被保存成 SMTP 密码

- **现象与根因：** `GET /api/admin/settings/smtp` 把已配置密码回显为 `***`；前端加载后直接保存在表单模型中，提交其他字段时将 `***` 一并送回；后端只把空串视为“不修改”，于是将字面值 `***` 加密落库。2026-09-14 曾通过只读核对确认库内密码解密后恰为 `***`，服务商返回 `535 Invalid login or password`。用户重新填写正确的 SMTP 专用密码后，测试邮件通过。
- **影响范围：** SMTP 设置中的任意保存都可能损坏已存密码；新增 API 凭据若沿用同一模式会出现同类问题。OIDC Secret 也使用 `***` 回显，实施前应核查相邻保存边界，但不据此认定 OIDC 已故障。
- **修复要求：** 秘密输入框与“已配置”状态分离；GET 只返回是否配置，输入值始终为空；PUT 明确区分“保持原值”和“设置新值”，后端拒绝把脱敏占位符当作新秘密；响应、日志和导出不泄漏明文。已被覆盖的旧密码不能从 `***` 恢复，须由管理员重新输入。
- **状态：** ◧ 工程修复完成，自动化通过；现场已重新输入密码恢复发送，改动后真实服务仍待复验。

### R30-03 单一 SMTP 表单混淆端口与加密方式

- **现象与根因：** 现有 `tls` 布尔值表示“连接即 TLS”；关闭时先按明文 SMTP 连接，再在服务端宣告支持时尝试 STARTTLS。腾讯云 SES 的 587 端口在本次无凭据握手检查中表现为 TLS 直连，旧配置 `587 + TLS 关闭` 曾导致约 60 秒后端等待；前端统一 15 秒超时先显示网络异常。用户补充确认本次腾讯云 465、587 均需单独开启 TLS 才能发送，不能据此判为代码故障。其他服务商的 587 不应套用腾讯云行为，例如 Mailgun、AWS SES、SendGrid 文档均列出 587 的明文握手/STARTTLS 路径。
- **影响范围：** 面板默认值、连接解释、超时和测试回执；密码重置、欢迎、审批通知及管理员发链接共用发送服务。
- **修复要求：** 通用 SMTP 表单明确区分“连接即 TLS”和“必须 STARTTLS”；禁止将未升级 TLS 的连接误报为加密；网络与请求超时有界且前后端期限协调。现有 `smtp_*` 数据按原语义读取，迁移时不自动改写正在工作的配置或密文。
- **状态：** ◧ 通用 SMTP 工程优化完成，自动化通过；腾讯云 465/587 的改动后真实服务仍待复验。

### R30-04 测试邮件收件人不可指定

- **现象：** 现有测试端点固定发送给当前管理员邮箱，无法验证指定测试地址。
- **修复要求：** 页面增加单个收件邮箱输入框；留空使用当前管理员邮箱，填入则只发送至所填地址。输入仅用于本次请求，不存为系统配置；服务端继续校验管理员权限，并验证地址和请求大小。回执区分默认/指定地址，地址和服务商错误遵循现有日志脱敏边界。
- **状态：** ◧ 工程优化完成，自动化通过；默认和指定收件邮箱的真实发送仍待人工核验。

---

## 二、用户已确认的范围与平台研究

1. 页面参照 OIDC 的“先选类型、再显示对应字段”交互：**通用 SMTP**，或 **Mailgun API / AWS SES API / SendGrid API**。不建设任意 HTTP API 请求模板。
2. 不建设腾讯云、阿里云专用 API 适配或专用预设；需要时由管理员在通用 SMTP 表单填写其官方参数。
3. SMTP 与三个平台 API 均列入接入/优化范围。真实服务或真机验收只规划**通用 SMTP 和 Mailgun**；AWS SES、SendGrid 本轮以隔离 mock、协议契约和自动化回归验证，**不宣称真实服务验收**。
4. 本次只研究并更新 Issue16；不修改代码、页面、数据库或运行配置。

### 官方能力与拟议表单映射（截至 2026-09-14）

| 选项 | 官方发送方式与关键约束 | 拟议表单字段 / 路径 | 证据 |
|---|---|---|---|
| 通用 SMTP | 参数因服务商而异；腾讯云 SES 可经通用 SMTP 对接，465 为 TLS 直连；阿里云 Direct Mail 按地域选主机，465 为 TLS 直连，25/80 可 STARTTLS。通用表单不承诺任意服务商均已实际验收。 | 主机、端口、连接方式、用户名、SMTP 专用密码、发件邮箱、现有启用范围；统一 SMTP 发送。 | [腾讯云 SMTP 地址](https://cloud.tencent.com/document/product/1288/65750/)、[阿里云 SMTP 地址](https://help.aliyun.com/zh/direct-mail/smtp-endpoints) |
| Mailgun API | HTTP Messages API；US/EU 域名分别使用 `api.mailgun.net` / `api.eu.mailgun.net`；可使用域名级 Sending Key（HTTP Basic 用户名 `api`）。Mailgun 另有 SMTP，本轮仅做专用 API。 | 区域、发信域名、发件邮箱、域名 Sending Key；固定 HTTPS 端点，文本发送到 `/v3/{domain}/messages`。 | [区域](https://documentation.mailgun.com/docs/mailgun/api-reference/api-overview)、[认证](https://documentation.mailgun.com/docs/mailgun/api-reference/mg-auth)、[发送](https://documentation.mailgun.com/docs/mailgun/user-manual/sending-messages/send-http) |
| AWS SES API | SES v2 `SendEmail` 支持 Simple 文本邮件；发件身份需验证，地域、身份与权限对应。SMTP 凭据与 API 凭据不同。 | AWS 地域、已验证发件邮箱、API 身份凭据；固定地域 HTTPS 端点，具备 `ses:SendEmail` 权限。静态密钥与角色/临时凭据的支持范围须在 Build 前定稿。 | [SendEmail](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_SendEmail.html)、[权限](https://docs.aws.amazon.com/service-authorization/latest/reference/list_sesv2.html) |
| SendGrid API | v3 Mail Send 为 JSON/HTTPS 接口，Bearer API Key；全局与 EU 子账号端点不同；发件人须为已验证身份或已认证域名。 | 全局/EU 区域、已验证发件邮箱、Mail Send 权限 API Key；固定 `/v3/mail/send` 端点。 | [Mail Send API](https://www.twilio.com/docs/sendgrid/api-reference/mail-send/mail-send)、[发件身份](https://www.twilio.com/docs/sendgrid/for-developers/sending-email/sender-identity) |

> **边界：** 腾讯云/阿里云虽有 API，用户已撤回其专用 API 适配。平台 API 接受请求不等于收件箱投递；SDK、签名、身份凭据和区域可用性在对应 Build Step 锁定官方版本，并以 mock 验证。真实凭据不得进入仓库测试。

---

## 三、目标交互与变更前影响评估

- “通知”区改为“邮件发送”卡片：先选方式，再展示专属字段；沿用 OIDC 的布局、切换确认、未保存提示和响应式行为。隐藏字段不得混入当前方式的保存请求。仅有**一个当前启用方式**；业务邮件和测试邮件共用该方式，保留 `password_reset / approval_notify / welcome` 三类启用范围。切换方式不自动发送，也不静默回退到旧 SMTP 或另一平台。
- 凭据按方式分别加密保存；GET 回显“已配置”状态，不把 `***` 放进秘密输入值。旧 `smtp_*` 作为通用 SMTP 的有效配置，升级只补方式元数据，不修改 host/port/密文/启用范围。旧 `tls=false` 可能是明文或机会性 STARTTLS，不能盲目改判为“必须 STARTTLS”；管理员编辑时引导选择明确方式。
- 通用 SMTP 使用明确的 TLS 直连 / 强制 STARTTLS 枚举；平台 API 只允许已定义的区域端点，不接受任意 URL。邮件地址、地域、端口、域名和 API 响应分别校验/脱敏；SMTP/API 调用有界超时，不在日志或页面回显密钥、认证头、重置链接及服务商原始敏感错误。
- 测试区独立于配置草稿：收件邮箱留空时取当前管理员邮箱，填入时用指定地址；测试读取**已保存的当前方式配置**。前端请求期限覆盖后端有界发送期限，避免先显示笼统“网络错误”。

| 受影响项 | 处理方式 |
|---|---|
| `frontend/src/views/admin/SettingsView.vue`、`frontend/src/api/settings.ts` | 方式选择、条件表单、秘密“已配置”状态、测试收件人、保存/测试反馈；核查 OIDC 相邻占位符边界，邮件改动不无意改变 OIDC。 |
| `backend/internal/config`、`backend/internal/mail`、`backend/internal/server/settings.go`、`approval.go` | 新配置合同与加密、旧 SMTP 兼容、统一发送接口、三个 API 适配、可选收件人与限时调用。 |
| `backend/internal/user`、`backend/internal/approval`、`backend/internal/auth`、server 装配 | 旧 `smtpConfigured` 三键口径改为“当前方式可用”；密码重置、欢迎、审批和批量发链接保持原业务门槛及失败语义。 |
| 导入导出、备份恢复、日志与权限 | 新 API 密钥和方式元数据按现有加密/恢复边界处理；导出不新增明文；管理员端点继续会话 + 角色双校验；错误/日志按字段路径脱敏。 |
| `ProdTestList.md`、后续 Build/Design | 本轮不改其他文档。实施时按分工同步；人工项仅在用户确认真实结果后更新，AWS SES/SendGrid 不以 mock 充当真机验收。 |

---

## 四、拟议分步实施与验收（平台部分未开始）

> 本表保留平台接入的后续实施方案。R30-02～R30-04 已按用户本次单独授权实施；下表中涉及平台 API 的部分均未开始，不能由本次 SMTP 修复推定已完成。

| Step | 目标与先后关系 | 主要验收证据 |
|---|---|---|
| 0. 合同冻结 | 明确当前方式枚举、旧 SMTP 映射、秘密保持/替换语义、SMTP TLS 直连与强制 STARTTLS、API 区域与错误回执；检查导入导出及 OIDC 相邻边界。 | 合同/迁移矩阵可审查；旧配置升级不改变实际发送；无未决设计冲突后才编码。 |
| 1. 占位符修复 | 先修 SMTP 密码 GET/PUT 与前端输入；新增 API 密钥复用安全合同；服务端拒绝 `***` 误存。 | 保存主机、端口或范围不改变原秘密；显式输入新秘密才替换；密文可读但不回显；失败优先回归。 |
| 2. 通用 SMTP | 明确连接方式与限时发送；兼容旧 `smtp_*`，条件表单；以协议 mock 覆盖腾讯云式 465/587 TLS 直连及常见 587 STARTTLS。 | 握手、超时/取消、认证/发件人/DATA 错误单测，旧配置回归，前端构建/页面检查；历史腾讯云实测不替代改动后复验。 |
| 3. 统一邮件服务与迁移 | 测试、重置、欢迎、审批和批量发链接接到同一“当前方式”接口；加密保存 API 凭据与方式元数据。 | 四类业务邮件 mock 回归、范围与失败不阻断语义、旧配置及导入导出/备份恢复回归；无明文或错方式回退。 |
| 4. 平台 API | 依次接入 Mailgun、AWS SES、SendGrid 的固定端点、认证、区域、文本发送与错误映射；各平台单独门禁。 | HTTPS mock 正反例、区域/凭据/发件人/错误/超时/脱敏验证；AWS SES 和 SendGrid 不要求真实账号结果。 |
| 5. 测试收件人与 UI | 测试端点接受可选单一 `to`；页面显示默认管理员邮箱、可改填指定地址和当前已保存方式；切换确认、草稿提示、移动端布局及超时信息收口。 | 未填/已填/非法地址、管理员无邮箱、非管理员、请求体过大与重复点击的定向回归；测试地址不落库。 |
| 6. 联合门禁与有限真实验收 | 执行 `cd backend && go build ./... && go vet ./... && go test ./...`、`cd frontend && npm run build` 及相关前端测试；只安排通用 SMTP 与 Mailgun 的隔离真实服务/页面检查。 | 自动化、隔离 Production/浏览器、真实服务与用户人工结论分层记录；AWS SES/SendGrid 标记“仅接入与自动化”；Build11 重置链接四态仍由 ProdTestList 单独跟踪。 |

---

## 五、当前处理边界

1. R30-02～R30-04 的本次单独修复授权已执行；第二节的平台 API 接入、方式切换和统一迁移仍只保留实施提案，未获本次授权。
2. 不保存或使用真实 SMTP/API 凭据做仓库自动化；真实服务发信需由用户明确提供测试环境与收件范围，结果与 mock、接口级检查分开记录。
3. R30-01 的重置链接四态和人工收件结果仍待核验；仅测试邮件通过，不关闭 Issue16 或 ProdTestList 对应人工项。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-14 | 根据用户反馈创建 Issue16，登记 Build11 邮件相关人工测试不通过。 |
| v1.1 | 2026-09-14 | 补入 SMTP 实测与密码占位符根因；按用户最新决定收敛为通用 SMTP + Mailgun/AWS SES/SendGrid API，登记测试收件人、影响评估与未授权分步方案。 |
| v1.2 | 2026-09-14 | 按单独授权修复 R30-02～04：SMTP 密码安全回显与保持、显式 TLS/STARTTLS 及有界发送、可选测试收件人与表单文案；平台接入未开始，改动后真实服务验收待核验。 |
