# Issue16.md — VPN 订阅管理系统邮件测试与 SMTP 优化（当前）

> **文档定位：** 同轮跟踪 Build11 邮件人工核查、SMTP 优化与 OIDC Secret 保存边界。R30-02～R30-05 已获单独修复授权并实施；平台 API 支持方案已由用户撤回。R30-05 真实 OIDC 登录仍需隔离环境单独核验。
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
- **影响范围：** SMTP 设置中的任意保存都可能损坏已存密码；OIDC 的相邻问题独立列为本文件 R30-05（后续已单独授权修复，不属邮件修复范围）。
- **修复要求：** 秘密输入框与“已配置”状态分离；GET 只返回是否配置，输入值始终为空；PUT 明确区分“保持原值”和“设置新值”，后端拒绝把脱敏占位符当作新秘密；响应、日志和导出不泄漏明文。已被覆盖的旧密码不能从 `***` 恢复，须由管理员重新输入。
- **状态：** ◧ 工程修复完成，自动化通过；现场已重新输入密码恢复发送，改动后真实服务仍待复验。

### R30-03 单一 SMTP 表单混淆端口与加密方式

- **现象与根因：** 现有 `tls` 布尔值表示“连接即 TLS”；关闭时先按明文 SMTP 连接，再在服务端宣告支持时尝试 STARTTLS。腾讯云 SES 的 587 端口在本次无凭据握手检查中表现为 TLS 直连，旧配置 `587 + TLS 关闭` 曾导致约 60 秒后端等待；前端统一 15 秒超时先显示网络异常。用户补充确认本次腾讯云 465、587 均需单独开启 TLS 才能发送，不能据此判为代码故障。其他服务商的 587 不应套用腾讯云行为，例如 Mailgun、AWS SES、SendGrid 文档均列出 587 的明文握手/STARTTLS 路径。
- **影响范围：** 面板默认值、连接解释、超时和测试回执；密码重置、欢迎、审批通知及管理员发链接共用发送服务。
- **当时修复要求：** 通用 SMTP 表单明确区分“连接即 TLS”和“必须 STARTTLS”；禁止将未升级 TLS 的连接误报为加密；网络与请求超时有界且前后端期限协调。当时关于保留旧 `smtp_*` 读取语义的要求已被用户后续“不考虑旧设计兼容”的决定取代，现行合同见第三节。
- **状态：** ◧ 通用 SMTP 工程优化完成，自动化通过；腾讯云 465/587 的改动后真实服务仍待复验。

### R30-04 测试邮件收件人不可指定

- **现象：** 现有测试端点固定发送给当前管理员邮箱，无法验证指定测试地址。
- **修复要求：** 页面增加单个收件邮箱输入框；留空使用当前管理员邮箱，填入则只发送至所填地址。输入仅用于本次请求，不存为系统配置；服务端继续校验管理员权限，并验证地址和请求大小。回执区分默认/指定地址，地址和服务商错误遵循现有日志脱敏边界。
- **状态：** ◧ 工程优化完成，自动化通过；默认和指定收件邮箱的真实发送仍待人工核验。

### R30-05 OIDC 脱敏占位符可能被保存为 Client Secret（独立问题）

- **现象：** 静态调用链显示，已配置 OIDC 时 `GetOidc` 返回 `client_secret: "***"`，设置页 `loadOidc` 把它直接写入表单模型，`doSaveOidc` 又将模型原样提交。用户若只修改其他 OIDC 字段并保存，可能把 `***` 当作新 Secret。修复前仅完成代码路径核对，尚无真实 OIDC 现场故障证据。
- **根因：** 后端 `oidc.SaveParams` 将任何非空 `ClientSecret` 加密保存；它仅在入参为空时保留原密文，未区分“脱敏显示”与“明确输入新 Secret”。
- **相邻缺陷（本次一并修复）：** `oidc.loadParams` 注释声明“自动解密”，但实现直接把 `oidc_params_*` JSON 中的密文作为 `ClientSecret` 返回；`flow.Exchange` 会把该密文当作 token 交换的 `client_secret` 发给真实 IdP，导致真实 OIDC 登录失败。R30-05 的“已配置状态”、测试接口复用已保存 Secret 与登录可用性都依赖解密，故本次一并修复。
- **影响范围：** OIDC 提供商参数的保存、测试与登录能力；已有 Secret 时保存其他字段、提供商切换及停用后重启用边界。OIDC 继续按提供商分别保存参数，本次未改变该语义。
- **修复实施（2026-09-14 用户单独授权）：**
  - GET 的 `client_secret` 始终为空，新增只读 `client_secret_configured` 表示当前提供商是否已有可用 Secret；
  - PUT 空值保留当前提供商原密文，显式非空新值才加密替换；
  - 服务端拒绝把 `***` 作为新 Secret（面板与 Setup 入口均校验），历史库内解密后为 `***` 的值按未配置处理，空值保存拒绝并要求重新输入；
  - `loadRawParams` 保留原密文用于空值保存，`loadParams` 用签名密钥解密后供状态判断、登录与测试连接使用；
  - OIDC 测试接口在 Secret 为空时回退到所选提供商已保存明文；无可用 Secret 时返回“未执行凭据校验”警告，不再静默报全通过；
  - 配置导入在本地登录关闭时，用导入包签名密钥解密校验占位符/不可解密密文，避免导入后认证死锁。
- **验收边界：** 使用隔离 mock 覆盖 GET/保存不改密文/显式替换/占位符拒绝、提供商切换、测试接口回退、登录所用 Secret 解密与导入校验；真实 OIDC 登录只在用户提供隔离环境后单独核验。不得把静态路径分析表述成已复现的现场问题。
- **状态：** ◧ 工程修复完成，后端 build/vet/test、errgate 与前端 build/test 通过；真实 OIDC 登录只在用户提供隔离环境后单独核验。

### R30-06 用户管理误判 SMTP 未配置（同轮静态检查发现）

- **现象与根因：** R30-02 修复后 `GET /api/admin/settings/smtp` 的 `password` 始终为空，另以 `password_configured` 表示已配置；但 `UsersView.loadMeta` 仍用 `smtp.host && smtp.user && smtp.password` 判定。由此推断用户管理中的“为所有无密码用户发送密码设置链接”和“触发重置邮件”会持续置灰，即使后端已有有效 SMTP 密码。此处尚未做真实浏览器复现。
- **影响范围：** 用户管理的两个邮件入口；后端原先也按库内三键判断，不能识别无认证中继。
- **修复：** 后端以 `config.SMTPConfigured` 统一面板、发送服务与用户管理判定；GET 直接提供 `configured`，前端两个入口据此决定是否置灰。新增无密码本地中继仍可启用入口的前端定向测试。
- **状态：** ◧ 代码及自动化修复完成；尚无真实浏览器与实际发信验收。

### R30-07 OIDC「暂未启用」未真正落库停用（相邻发现，待单独处理）

- **现象与根因：** 设置页把「暂未启用」映射为 `provider_type=''`，但 `onProviderChange` 只改本地表单并折叠参数区，不调用任何保存接口；`SaveOidc` 的 `validProviders` 也不接受空 provider_type。因此选择「暂未启用」不会真正停用 OIDC，离开页面后后端仍按旧提供商工作，仅形成本地未保存状态。
- **影响范围：** OIDC 生命周期语义与“停用后重启用”的人工核验；不影响 R30-05 的 Secret 空回显、保留/替换与占位符拒绝合同。
- **状态：** ☐ 待单独决策；R30-05 修复按用户确认未改变该语义，留待独立授权后处理。


---

## 二、用户已确认的范围与 SMTP 平台研究

1. 用户撤回 Mailgun、AWS SES、SendGrid 以及其他平台的 HTTP API 适配；统一通过**一套通用 SMTP 配置与发送链路**接入。原“先选 API 类型、条件显示专属字段”、多套凭据和方式迁移方案失效。
2. 腾讯云、阿里云、Mailgun、AWS SES、SendGrid 均按其官方 SMTP 参数由管理员配置；不把某个服务商的端口/加密规则自动套用到其他服务商。服务商示例仅作填写指引，不构成真实互通验收。
3. 2026-09-14 用户进一步明确：后续 SMTP 重设计**不以旧设计或配置兼容为约束**，以成熟开源项目的现行做法为主要参照；当前阶段只研究和确认方案，不改代码、页面、数据库或运行配置。若未来实施新合同，原配置可能需要管理员重新选择连接方式并复验，不能把当前腾讯云测试成功延伸为新方案验收。

### 官方 SMTP 参数核查（2026-09-14）

| 服务商 | SMTP 参数与凭据要点 | 对通用表单的提示 | 官方证据 |
|---|---|---|---|
| 腾讯云 SES | 本次用户以 `smtp.qcloudmail.com:465`、连接即 TLS、SMTP 专用密码成功发送测试邮件；用户还确认本次 587 也需单独开启 TLS。 | 保留本次实测说明，不推断其他服务商的 587 行为，也不把一次测试发送写成收件箱验收。 | [腾讯云 SMTP 地址](https://cloud.tencent.com/document/product/1288/65750/) |
| 阿里云 Direct Mail | SMTP 主机按站点地域选择；465 为 TLS 直连，25/80 可通过 STARTTLS 升级。 | 主机、端口与加密方式分别填写；不假定 587 可用。 | [SMTP 服务地址](https://help.aliyun.com/zh/direct-mail/smtp-endpoints) |
| Mailgun | SMTP 凭据按域名管理；US/EU 出站主机分别为 `smtp.mailgun.org` / `smtp.eu.mailgun.org`；465 需 TLS 直连，587 可 STARTTLS。 | 填该域名的 SMTP 用户名/密码，不填 Mailgun HTTP Sending Key；区域由域名所属环境决定。 | [SMTP 发送](https://documentation.mailgun.com/docs/mailgun/user-manual/sending-messages/send-smtp)、[区域主机](https://documentation.mailgun.com/docs/mailgun/api-reference/api-overview) |
| AWS SES | SMTP 主机和凭据按 AWS 地域选择；SMTP 密码不同于 AWS Secret Access Key，不能直接填 API 密钥；587 为 STARTTLS，465 为 TLS Wrapper。 | 提示管理员创建对应地域的 SMTP 凭据并使用已验证发件身份；sandbox 收件限制仍须在真实测试时核对。 | [SMTP 连接](https://docs.aws.amazon.com/ses/latest/dg/smtp-connect.html)、[SMTP 凭据](https://docs.aws.amazon.com/ses/latest/dg/smtp-credentials.html)、[sandbox](https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html) |
| SendGrid | SMTP 主机 `smtp.sendgrid.net`，用户名固定 `apikey`，密码为有 Mail 权限的 API Key；587 可走 TLS/STARTTLS，465 为 SSL/TLS 直连。 | API Key 在此仅作为 SMTP 密码使用，不接入 SendGrid HTTP Mail Send API；发件身份仍需按服务商要求验证。 | [SMTP 接入](https://www.twilio.com/docs/sendgrid/for-developers/sending-email/integrating-with-the-smtp-api)、[发件身份](https://www.twilio.com/docs/sendgrid/for-developers/sending-email/sender-identity) |

### 开源项目 SMTP 配置对照（2026-09-14，只读样本）

| 项目 | 配置/代码实际做法 | 对本项目的启示 |
|---|---|
| Gitea | `PROTOCOL` 枚举 `smtp`、`smtps`、`smtp+starttls` 等；可在未指定协议时从端口推断，但仍把服务器、端口、账号、密码、发件人独立保存，并提供测试邮件。其 `smtp+starttls` 文档描述为机会性升级，并不等于本项目的“必须 STARTTLS”。[配置源码对应说明](https://docs.gitea.com/administration/config-cheat-sheet/)、[邮件设置](https://docs.gitea.com/1.25/administration/email-setup/) | 连接方式用有语义的枚举，端口只是参考；“STARTTLS”是否强制须在 UI 明确说明。 |
| Vaultwarden | [配置代码](https://github.com/dani-garcia/vaultwarden/blob/main/src/config.rs)以 `smtp_security` 保存 `starttls / force_tls / off`；[配置样例](https://github.com/dani-garcia/vaultwarden/blob/main/.env.template)将三项分别关联 587/465/25，用户名可不填，填写用户名后密码必填；另有独立 host、from、timeout。 | 单一连接方式选项可完整表达三种行为；无认证本地中继有成熟先例。 |
| Mattermost | [配置模型](https://github.com/mattermost/mattermost/blob/master/server/public/model/config.go)有 `PLAIN / TLS / STARTTLS` 常量；[面板文档](https://docs.mattermost.com/administration-guide/configure/smtp-email)分列 host、port、发件地址、账号、密码和 Connection Security，填账号密码时要求 TLS 或 STARTTLS，并提供测试连接；其本地 Postfix 示例为 25/无认证/无加密。 | 三态与可选认证均有现成参照；明文模式须与凭据发送分开。 |
| Nextcloud | [配置样例](https://github.com/nextcloud/server/blob/master/config/config.sample.php)与[发送代码](https://github.com/nextcloud/server/blob/master/lib/private/Mail/Mailer.php)区分 SSL/TLS 直连和未指定时的自动 STARTTLS；[管理文档](https://docs.nextcloud.com/server/30/admin_manual/configuration_server/email_configuration.html)明确自动 STARTTLS 无法强制。 | “自动”可存在，但必须写明支持时才升级，不能与强制 STARTTLS 同名。 |
| GitLab | [SMTP 文档](https://docs.gitlab.com/omnibus/settings/smtp/)分别配置 `smtp_tls` 与 `smtp_enable_starttls_auto`，并明确两者互斥；按服务商给独立 host/port/发件人与凭据示例。 | 多开关容易产生冲突，需要额外互斥规则；不适合作为本项目三态的简化。 |

**样本结论与取舍：** 这五个项目是定向样本，不足以统计证明“多数开源项目”。其中 Gitea、Vaultwarden、Mattermost 用协议/安全性选项；Nextcloud 用两种模式且自动 STARTTLS 不能强制；GitLab 用互斥布尔项。共同结构是主机、端口、认证凭据、发件人与连接安全性分开配置。既然本项目不保留旧设计，**研究推荐**以 Vaultwarden/Mattermost 的三态为蓝本：`STARTTLS（必须升级）`、`连接即 TLS`、`无加密（仅本地无认证中继）`，不提供机会性“自动 STARTTLS”。第三态不是为历史配置保留，而是支持本地中继。单一选择器（下拉或单选组）比开关准确；此前用户倾向开关，但该倾向发生在“第三态需保留”的讨论中，现须按新研究结果由用户重新确认。

### 改进前基线与差距（以下描述改动前代码）

- **已有能力：** 当前 `SMTPSettings` 已分出 `implicit_tls / starttls / legacy`；`smtp_password` 加密落库，GET 返回空输入与“已配置”状态，测试邮件可选择单个收件人。测试、密码重置、欢迎、审批通知共用 `mail.Service`；管理员发链接另有 `smtpConfigured` 判定。无需为了支持上述平台增加发送适配器或新的凭据键。
- **保存一致性：** `SaveSMTP` 目前逐键写入，写到中途失败可能留下新旧字段混合配置；只校验 host 非空、加密方式枚举和 `***`，端口、地址、范围缺少完整的保存前合同。建议先验证整组有效配置，再用同一事务保存；空输入“保留旧值”须按最终合成配置检查，不能把非法旧值静默当作新配置通过。
- **连接与认证差距：** 当前表单初值为 `587 + legacy`，实际发送有机会性 STARTTLS，且 host/user/password 三键缺一便判为未配置。按上述候选，新建表单改为明确 `STARTTLS`/587（仅为常见示例，端口仍独立可改），旧 `legacy` 行为退出新合同；若支持本地无认证中继，还须同步修改 `mail.Configured`、用户管理入口、测试与业务发信的启用判定。不能仅改 UI 标签。
- **反馈与诊断：** 当前测试端点把 SMTP 错误原文拼回页面；服务商回执可能含邮箱、主机或其他诊断内容。建议改为阶段化且脱敏的“连接/握手、TLS、认证、发件人、收件人、DATA、超时”回执，保留服务端可定位的安全错误类别；成功只表示 SMTP 服务端接受，页面再提示检查收件箱。需核对 `smtpTest`、业务日志和前端 40 秒/后端 30 秒期限的实际表现。
- **验收层级：** 协议 mock 分别覆盖 TLS 直连、必须 STARTTLS、本地无认证明文中继、禁止明文认证、凭据保持、部分写入失败与错误脱敏；隔离 Production/浏览器检查表单和测试状态；真实服务仅在用户提供测试条件时按服务商逐项记结果。此前腾讯云测试成功不代替优化后的复验。

---

## 三、已确认的通用 SMTP 方案、实施结果与影响

- “通知”区维持一张通用 SMTP 卡片：主机、端口、连接方式、登录账号、SMTP 密码、发件邮箱和 `password_reset / approval_notify / welcome` 三类启用范围。建议在字段旁提供可查的服务商填写示例，不保存服务商类型、不自动覆盖用户参数；测试区继续使用**已保存配置**，草稿未保存时明确提示。
- **连接方式 UI（2026-09-14 已确认并实施）：** 一个选择器呈现 `STARTTLS（必须升级）`、`连接即 TLS`、`无加密（仅本地无认证中继）`；“需要 SMTP 认证”开关控制账号/密码字段，默认开启。无加密只接受回环 IP 地址且强制无认证；端口独立填写，不按端口静默切换方式。旧 `legacy`/自动尝试不再提供或发送。
- 密码输入保持空值与“已配置”状态分离；留空保存保留有效原密文，明确输入新值才替换，拒绝字面值 `***`。若库内已是历史损坏的 `***`，判为未配置并要求重新输入专用密码。不再读取旧 `smtp_tls` 分支；旧连接方式或缺少新认证状态时报告未配置，管理员须重新选择、保存和测试。切至无认证时清除账号和密码；旧 `smtp_tls` 键在新保存时删除。
- 保存前校验完整配置，事务内一次提交主机、1～65535 端口、单一发件邮箱、连接方式、认证状态、账号/密码和启用范围。是否另设 SMTP 总开关及“清空配置”操作不属于本轮已确认范围，保持候选。
- 发送限时沿用有界 context；连接、握手、TLS、认证、发件人、收件人、内容传输和结束会话按阶段返回安全错误，不把服务商原始响应直接回显给页面或普通错误日志。发送成功仅表示 SMTP 服务端接受，收件箱投递另行核验。
- 测试收件人仍为本次请求的单一 `to`，留空使用当前管理员邮箱，不持久化。发送按钮要区分“保存配置”和“用已保存配置测试”；三类业务邮件及管理员发链接共用新的“SMTP 已配置”判定，不引入平台切换或另一条发送路径。

### 现行合同勘误（DC-01／DC-02，2026-09-14 已确认）

> 本节为 BuildReport6 的 DC-01／DC-02 收敛口径；归档 `Design1.md` 的旧文字继续保留为历史记录，不按旧文实现，也不修改归档原文。

- **DC-01 重置令牌四态：** 重置令牌一次性、1 小时 TTL；使用后不可再次设密；DB 保留 `used=1` 记录，用于区分 `valid / missing / used / expired`；过期历史由既有清理任务处理；不再以 Design1“用后即删”作为当前验收口径。当前实现位置：`backend/internal/auth/reset.go`、`backend/internal/server/auth.go`；定向测试见 `internal/auth/reset_test.go`、`internal/server/reset_validate_test.go`。
- **DC-02 SMTP 三态合同：** `smtp_security` 固定为 `starttls / implicit_tls / plain`；`smtp_auth_required` 为独立开关；`plain` 仅允许回环 IP 且强制无认证；密码 GET 不回显并返回 `password_configured`，PUT 空值保持原密文、拒绝字面 `***`；保存走事务，`configured` 由后端统一判定；旧 `legacy`/机会性 STARTTLS 退出新合同，不静默迁移。当前实现位置：`backend/internal/config/smtp.go`、`backend/internal/config/admin.go`、`backend/internal/mail/mail.go`、`backend/internal/server/settings.go`。

### 面板中的“自定义发信内容”模块预留（只研究，未实施）

- **放置与状态：** 预留在同一“通知”分组、SMTP 设置与测试区之后的独立“邮件内容”卡片；SMTP 连接参数与邮件文案分别保存，避免只改正文时触碰密码和连接方式。当前页面、数据库和发送结果均未新增该模块；正式实施前须先在 Design/Build 中定稿。
- **现有内容盘点：** 邮件服务目前用代码内置的纯文本主题/正文：密码重置一类、审批通过和拒绝两种结果、欢迎邮件的本地登录与 OIDC 登录两个分支；SMTP 测试邮件为独立固定文案。首版候选只开放这五种**业务邮件分支**的主题和纯文本正文，测试邮件继续固定，以免把“连接测试”混同于业务内容测试。三类启用范围继续控制是否发送，自定义文案不覆盖开关。
- **开源参照：** [Gitea 邮件模板](https://docs.gitea.com/administration/mail-templates/)按事件选模板，内置默认内容，管理员可逐项覆盖；未覆盖的事件回落到默认模板。这支持本项目“按业务邮件类型定制、默认内容随程序提供”的方向，但 Gitea 采用文件模板，并不证明多数项目使用 Web 面板编辑。这里的面板入口来自用户明确需求，模板编辑范围仍需另行确认。
- **变量与预览：** 候选白名单为 `{{site_name}}`、`{{login_url}}`、`{{reset_url}}`，按分支只显示可用变量；不引入任意表达式、HTML、附件或用户自填收件人。密码重置正文必须保留 `{{reset_url}}`，欢迎邮件正文建议保留 `{{login_url}}`，否则可能发出无法完成操作的邮件。页面用合成站点名与示例链接预览，绝不取真实一次性重置令牌进入预览、配置或日志。
- **保存与回退：** 每个分支显示默认文案、草稿和“恢复默认”；未定制时继续使用当前代码文案。后端在保存前校验主题单行、正文/变量白名单、长度与必需链接，整组写入与读取失败的回退语义须先定稿；配置导入导出与备份恢复要覆盖模板键，不得因导入缺失新键改变旧站点默认邮件。正式实施需定向验证五分支渲染、scope、链接有效期说明、邮件头清洗和失败不阻断主流程。
- **待确认范围：** 首版是否包含审批通过/拒绝和欢迎本地/OIDC 的分别编辑，以及是否允许自定义测试邮件，仍需用户确认后进入 Build；本次仅预留信息架构和安全边界。

| 受影响项 | 处理方式 |
|---|---|
| `frontend/src/views/admin/SettingsView.vue`、`frontend/src/api/settings.ts` | 三态连接方式选择、无认证字段状态、未保存草稿提示、测试与保存反馈；后续同分组预留独立邮件内容卡片。 |
| `backend/internal/config`、`backend/internal/mail`、`backend/internal/server/settings.go`、`approval.go` | 合成配置校验、事务保存、新连接方式与可选认证、SMTP 阶段化安全错误与测试回执；退出旧机会性 STARTTLS 行为，不新增平台 API 适配器。 |
| `backend/internal/user`、`backend/internal/approval`、`backend/internal/auth`、server 装配 | 核对 `smtpConfigured` 与实际发送配置判定一致；密码重置、欢迎、审批及批量发链接保持原业务门槛与失败语义。 |
| 导入导出、备份恢复、日志与权限 | 旧备份中的 SMTP 连接方式需按新合同重新配置和测试；不得静默推断或继续发送。备份中的其他业务数据处理范围另行核对；管理员端点继续会话 + 角色双校验，服务商回执不泄漏凭据。 |
| `ProdTestList.md`、后续 Build/Design | 后续实施按文档分工同步；人工项目仅在用户确认真实结果后更新，不把 mock 或历史腾讯云结果充作新版本真机验收。 |
| `frontend/src/views/admin/UsersView.vue` | R30-06 已改用后端 `configured` 判定，不依赖永远空的 `password` 回显字段；有认证和无认证中继均使用同一后端合同。该修复与 R30-05 分开验收。 |
| 未来邮件内容设置与 `backend/internal/mail` | 五个业务文案分支的主题/纯文本正文、变量白名单、预览、默认回退、导入导出及发送上下文；当前只预留，不改现有邮件模板。 |

---

## 四、实施与验收状态

> R30-02～R30-04 已按先前授权实施；用户于 2026-09-14 确认本节的三态 SMTP 方案并授权修复。R30-05 另按用户单独授权完成工程修复，见第一条问题。以下区分代码/自动化与真实服务、人工作业，不把前者当成后者。

| Step | 目标与先后关系 | 主要验收证据 |
|---|---|---|
| 0. 合同确认 | 三态选择器、认证开关、仅回环无认证明文、旧配置退出方式。 | 已确认；旧配置不静默迁移，须重新保存并测试。 |
| 1. 保存合同 | 合成配置校验与事务写入，覆盖主机、端口、认证/凭据、发件邮箱与 scope。 | 代码及定向测试通过；旧配置/导入数据不满足新合同时停止发送。 |
| 2. SMTP 发送与诊断 | 强制 STARTTLS、直连 TLS、本地无认证明文、阶段化安全错误、总超时。 | 协议 mock 覆盖未宣告 STARTTLS 必须停止、本地中继不发送 AUTH/STARTTLS、错误回执不泄漏；真实服务仍待复验。 |
| 3. 表单与用户管理 | 三态与认证开关、未配置提示、测试回执、R30-06 邮件入口状态。 | 前端定向测试及构建通过；浏览器人工检查待办。 |
| 4. 联合门禁与真实验收 | 后端 build/vet/test、前端 build/test；隔离环境按实际提供的 SMTP 服务商逐项核验。 | 自动化通过；隔离 Production、真实 SMTP、收件箱及 Build11 重置链接四态仍待分别核验。 |
| 独立问题 R30-05 | OIDC Secret 空回显/状态、空值保持、显式替换、占位符拒绝、加载时解密、测试连接回退与导入占位符校验；不并入 SMTP 连接方式或邮件内容修改。 | 后端 build/vet/test、errgate 与前端 build/test 通过；真实 OIDC 登录待用户提供隔离环境后单独核验。 |
| 后续邮件内容模块 | 在设计定稿和单独授权后，按五个业务分支实现主题/正文、变量、预览、默认回退与保存；不改 SMTP 凭据合同。 | 模板渲染、危险输入、真实令牌不入预览/日志、导入恢复和人工文案验收分别记录。 |

---

## 五、当前处理边界

1. R30-02～R30-05 的单独修复授权已执行；平台 API 方案已撤回。本轮已按用户确认完成通用 SMTP 重设计、R30-06 与 R30-05 工程修复。
2. 不保存或使用真实 SMTP 凭据做仓库自动化；真实服务发信需由用户明确提供测试环境与收件范围，结果与 mock、接口级检查分开记录。
3. R30-01 的重置链接四态和人工收件结果仍待核验；仅测试邮件通过，不关闭 Issue16 或 ProdTestList 对应人工项。
4. R30-05 工程修复已完成，真实 OIDC 登录待用户提供隔离环境单独核验；R30-07「暂未启用」未落库为相邻问题，待单独授权处理。
5. 自定义邮件内容仅作模块预留，尚无页面入口、配置键或可用功能；五分支与测试邮件的编辑范围待定稿。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-14 | 根据用户反馈创建 Issue16，登记 Build11 邮件相关人工测试不通过。 |
| v1.1 | 2026-09-14 | 补入 SMTP 实测与密码占位符根因；按用户最新决定收敛为通用 SMTP + Mailgun/AWS SES/SendGrid API，登记测试收件人、影响评估与未授权分步方案。 |
| v1.2 | 2026-09-14 | 按单独授权修复 R30-02～04：SMTP 密码安全回显与保持、显式 TLS/STARTTLS 及有界发送、可选测试收件人与表单文案；平台接入未开始，改动后真实服务验收待核验。 |
| v1.3 | 2026-09-14 | 仅补平台官方合同与现有调用链、密钥持久化/恢复、OIDC 相邻风险的改进前只读研究；登记 AWS 凭据和切换方式凭据保留规则的待决策项，未进入平台实现。 |
| v1.4 | 2026-09-14 | 按用户最新决定撤回所有平台 API 支持，改为通用 SMTP；补各服务商官方 SMTP 参数与现有保存/诊断差距，重写后续优化候选；OIDC R30-05 移至独立 Issue17。 |
| v1.5 | 2026-09-14 | 按用户决定将 R30-05 移回本文件并撤回 Issue17；连接方式改为两态开关研究并保留 `legacy` 第三态待定稿；预留自定义业务邮件内容模块，另登记用户管理邮件入口置灰风险 R30-06。 |
| v1.6 | 2026-09-14 | 对照 Gitea/Vaultwarden/Mattermost/Nextcloud/GitLab 当前 SMTP 设置；按用户最新指示撤销旧设计兼容约束，推荐三态选择器（含本地无认证中继）并移除机会性自动模式；补对用户管理、导入恢复和邮件模板默认覆盖的影响，待用户确认后才进入实现。 |
| v1.7 | 2026-09-14 | 按用户确认实施三态 SMTP、认证开关、完整校验与事务保存、统一已配置判定、阶段化安全错误和 R30-06 页面修复；后端 build/vet/test、前端 build/test 通过，真实服务及浏览器人工项保留待验收。 |
| v1.8 | 2026-09-14 | 按 BuildReport6 DC-01/DC-02 收敛现行合同：补重置令牌四态与 SMTP 三态合同速查；归档 Design1.md 旧文字不修改、不作为当前实现/验收口径。 |
| v1.9 | 2026-09-14 | 按用户单独授权修复 R30-05：OIDC Secret 空回显与只读状态、空值保持/显式替换/占位符拒绝、加载时解密、测试连接回退与导入占位符校验；后端 build/vet/test、errgate 与前端 build/test 通过；另登记 R30-07「暂未启用」未落库相邻问题，真实 OIDC 登录待隔离环境核验。 |
