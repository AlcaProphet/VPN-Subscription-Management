# VPN 订阅管理系统功能构建计划（Build28：业务邮件内容定制）

> **文档定位：** 本文仅承接 [Design5.md](Design5.md) §七已确认的业务邮件内容定制。§一～§六整站迁移仍是候选，不属于 Build28。编码约束见 [AGENTS.md](AGENTS.md)，历史 SMTP 预留见 [Issue16.md](docs/reports/Issue/Issue16.md)。
> **执行状态（2026-09-15）：** 实施计划已完成当前项目复核并形成 v0.6 定稿，但仍未授权执行代码构建；Step 0 已完成计划与决策冻结，Step 0.5～8 全部未开始。以后获授权执行时严格按顺序逐 Step 构建和验收，不跳步、不并行，不把计划写成已验证功能。

---

## 一、用户已确认的合同

1. 在“通知”分组的 SMTP 卡片及测试区之后增加独立“邮件内容”卡片，仍用一套通用 SMTP；文案与连接配置分别保存。
2. 五种业务分支分别编辑主题和纯文本正文：密码重置、审批通过、审批拒绝、本地欢迎、OIDC 欢迎。连接测试邮件固定，不受自定义文案影响。
3. 正文直接写文本及有限的占位符，旁边即时预览未保存草稿；不提供 HTML/富文本编辑器、任意表达式或额外图片上传。
4. 审批通过只发送一封审批通过通知，归入 approval_notify 开关，正文必须含 login_url；其他首次激活归入 welcome，并按来源选择本地/OIDC 文案。不得在通过审批时同时发欢迎邮件。
5. 正文的登录/重置链接在 HTML 邮件中是可点击锚点，链接目标与可见文字都是同一个完整 URL；纯文本版本也显示完整 URL。管理员仍只编写纯文本模板。邮件不含 Logo、图片或附件。

---

## 二、现状、影响与边界

| 受影响项 | 当前事实 | Build28 的处理 |
|---|---|---|
| backend/internal/mail/mail.go | 内置纯文本主题/正文，直接拼 RFC822 文本；已有五分支文案函数，但审批通过分支当前未被调用 | 模板选择、白名单渲染、预览复用、纯文本与最小 HTML 双版本 MIME；审批通过新增必需登录链接，所有链接显示完整 URL |
| backend/internal/approval/approval.go、user、auth 与 server 装配 | 审批通过目前只发欢迎；其他入口依赖三类 scope | 改审批通过单封与开关归属，保持其他入口、失败不阻断主流程及令牌时效 |
| backend/internal/config、backend/internal/server/settings.go | system_config 保存 SMTP 参数；管理路由有会话和管理员双中间件 | 模板键按分支保存/恢复；独立管理 API；不触碰 SMTP 密码和连接方式 |
| frontend/src/views/admin/SettingsView.vue、frontend/src/api/settings.ts | 只有 SMTP 连接设置、启用范围与固定测试邮件 | 增加单分支编辑、变量插入、即时预览、草稿保护和状态/错误提示 |
| 配置导入导出、备份 | 邮件模板覆盖需随配置迁移和完整备份保留 | 验证模板覆盖保留与缺键默认 |

**边界：** 不改整站迁移、SMTP 服务商/认证合同、连接测试邮件、营销群发、通知种类、Xray 专项或 SecurityScanPlan1。正式 SMTP 投递、真实收件箱、邮件客户端链接表现及浏览器人工验收是独立证据，不以单元测试冒充。除计划本身外，尚未取得 Step 0.5～8 的执行授权。

---

## 三、实施合同定稿

本章把 Design5 §七的产品结论收敛为可直接编码的接口、数据与行为合同。后续 Step 只能在本章范围内实现；如实施前发现本章与当前代码事实不符，按 Step 0.5 停止并记录差异，不在代码中临时发明另一套合同。

### 3.1 固定模板 ID、配置键与内置默认值

首版模板集合是编译期固定表，不允许通过 API 新增、删除、重命名或排序。模板 ID 同时用于路由参数和前端值；配置键只用于后端持久化，不暴露为可编辑字段。

| 顺序 | 模板 ID | 配置键 | 显示名称 | 内置主题 | 内置正文 | 正文必需变量 | scope |
|---|---|---|---|---|---|---|---|
| 1 | `password_reset` | `mail_template_password_reset` | 密码重置 | `密码重置` | `请在 1 小时内使用以下链接重置密码（一次性）：\n{{reset_url}}` | `{{reset_url}}` | `password_reset` |
| 2 | `approval_approved` | `mail_template_approval_approved` | 审批通过 | `{{site_name}} 审批通知` | `您在 {{site_name}} 的账号已通过审批，现在可以登录：\n{{login_url}}` | `{{login_url}}` | `approval_notify` |
| 3 | `approval_rejected` | `mail_template_approval_rejected` | 审批拒绝 | `{{site_name}} 审批通知` | `您在 {{site_name}} 的账号申请未通过审批。` | 无 | `approval_notify` |
| 4 | `welcome_local` | `mail_template_welcome_local` | 本地欢迎 | `{{site_name}} 账号已激活` | `{{site_name}}\n\n您的账号已激活，请使用邮箱与密码登录：{{login_url}}` | `{{login_url}}` | `welcome` |
| 5 | `welcome_oidc` | `mail_template_welcome_oidc` | OIDC 欢迎 | `{{site_name}} 账号已激活` | `{{site_name}}\n\n您的账号已激活，请使用单点登录（OIDC）登录：{{login_url}}` | `{{login_url}}` | `welcome` |

主题变量白名单为：除 `password_reset` 外的四个分支可用 `{{site_name}}`，`password_reset` 不开放主题变量。正文变量白名单以 Design5 §7.1 为准：密码重置仅 `{{reset_url}}`；审批通过、本地欢迎、OIDC 欢迎为 `{{site_name}}`、`{{login_url}}`；审批拒绝仅 `{{site_name}}`。默认文案刻意保持现有文案，仅为审批通过补入已确认的完整登录链接，并把其发送事件从欢迎分支改到审批通过分支。

### 3.2 持久化格式、读取状态与恢复语义

- 每个配置键保存一个严格 JSON 对象：`{"subject":"...","body":"..."}`。只接受这两个必需字符串字段；未知字段、缺字段、尾随 JSON 值和非字符串值均视为损坏/非法。
- 不新增数据库表或 migration；继续使用 `system_config` 的单键原子 UPSERT。模板不是凭据，不进入 `config.isSensitiveKey`，但日志、错误和审计信息仍不得输出主题、正文或展开后的 URL。
- `mail.Service.LoadTemplate(kind)` 显式调用 `config.Service.Get`，不得用吞掉错误的 `GetOr`。返回有效模板以及 `default`、`customized`、`damaged` 三态：键不存在为 `default`；合法覆盖为 `customized`；JSON 或领域校验失败为 `damaged`，对发送和管理读取均回退该分支当前内置默认值。
- 数据库读取错误也按默认文案继续本封业务邮件，但返回/记录可诊断的安全状态；日志仅含模板 ID、配置键和错误类别，不含配置值。管理 API 若整个列表读取失败，返回 500；单个已读值损坏则列表仍返回 200，并把该项标为 `damaged`。
- 保存先在邮件领域层完整校验，再序列化并只 UPSERT 目标键。恢复默认只删除目标键，操作幂等；删除不存在的键仍成功并返回 `default`。为此在 `config.Service` 增加通用 `Delete(ctx, key)`，不得在 HTTP Handler 中直接执行 SQL。
- 五个键彼此独立，首版采用最后一次成功写入生效；不增加跨管理员乐观锁、草稿表或历史版本。单次保存/恢复只有一个键，因此不存在部分模板写入。

### 3.3 固定校验与规范化规则

后端是唯一权威校验方，前端 `maxlength` 与提示只做即时反馈。所有长度按 Unicode code point（Go `utf8.RuneCountInString`）计算，不按字节数计算。

| 项目 | 定稿规则 |
|---|---|
| API 请求体 | 每个保存/预览请求经 `http.MaxBytesReader` 限制为 64 KiB；超限返回 413 |
| 主题 | 1～200 字符；`TrimSpace` 后不得为空；原值不自动裁剪；禁止 CR、LF、NUL、C0/C1 控制字符及所有 URL 变量 |
| 正文 | 1～10,000 字符；`TrimSpace` 后不得为空；接收时把 CRLF/CR 规范化为 LF；允许 LF 与 Tab，禁止 NUL、其余 C0/C1 控制字符 |
| 占位符 | 仅接受精确的 `{{[a-z_]+}}` 白名单字面量；出现任何其他 `{{...}}`、未闭合 `{{`/`}}`、跨分支变量或主题中的 URL 变量均返回 400 |
| 必需变量 | 保存与预览都要求正文至少出现一次对应链接变量；可重复出现；恢复默认不接收正文，因此不做请求校验 |
| 实际值 | `site_name` 作为普通文本；`reset_url`/`login_url` 必须是绝对 `http` 或 `https` URL 且 host 非空，不接受相对地址、userinfo 或其他 scheme |

校验错误返回稳定、可展示且不含输入正文/URL 的中文信息，并通过可比较的邮件领域错误映射为 400。保存、预览、导入校验和实际发送调用同一个 `ValidateTemplate`；禁止四处复制正则或长度常量。前端从 GET 元数据读取长度上限和变量清单，不另写一份可能漂移的领域规则。

### 3.4 邮件领域对象与职责边界

`backend/internal/mail` 内新增或拆分为 `template.go`、`render.go`、`message.go`，保留 `mail.go` 负责 SMTP 会话和业务入口。名称可在 Step 0.5 按当前目录最小调整，但职责固定：

~~~go
type TemplateKind string
type Template struct { Subject string `json:"subject"`; Body string `json:"body"` }
type TemplateDefinition struct {
    ID TemplateKind; ConfigKey, Label, Scope string
    Default Template; SubjectVariables, BodyVariables, RequiredBodyVariables []string
}
type TemplateState string // default/customized/damaged
type TemplateView struct {
    ID                    TemplateKind  `json:"id"`
    Label                 string        `json:"label"`
    Scope                 string        `json:"scope"`
    Subject               string        `json:"subject"`
    Body                  string        `json:"body"`
    State                 TemplateState `json:"state"`
    Warning               string        `json:"warning"`
    SubjectVariables      []string      `json:"subject_variables"`
    BodyVariables         []string      `json:"body_variables"`
    RequiredBodyVariables []string      `json:"required_body_variables"`
}
type RenderValues struct { SiteName, LoginURL, ResetURL string }
type Rendered struct { Subject, TextBody, HTMLBody string }
~~~

- 固定定义表是模板 ID、默认值、变量、scope 和显示顺序的单一来源；`Definition(kind)` 拒绝未知 ID，列表始终按 3.1 顺序返回，不依赖 map 遍历。
- `ValidateTemplate(kind, template)` 只做模板领域校验；`LoadTemplate`、`SaveTemplate`、`RestoreTemplate` 负责配置读写；`Render(kind, template, values)` 先复用校验，再做值校验与替换。
- 预览和实际发送都只能调用 `Render`；预览不能自行拼字符串，业务发送不能绕过模板校验。发送时损坏覆盖先回退默认，再渲染默认，不允许带损坏正文继续发送。
- `config` 不导入 `mail`。导入前校验通过 `ExportService.SetValidateConfig(func(map[string]string) error)` 由 `server.New` 注入 `mail.ValidateTemplateOverrides`；回调只校验传入 map，不访问数据库或网络。

### 3.5 变量替换与 URL 转换算法

- 渲染器先扫描模板，按出现顺序把普通文本和白名单占位符拆成 token；不使用 `text/template`、`html/template` 的表达式能力，也不做递归替换。变量值中即使含 `{{...}}` 也只作为文本。
- 纯文本正文把变量替换为原值，保留 LF；HTML 正文对普通文本和 `site_name` 做文本转义，并把 LF 转为 `<br>`。不接受管理员 HTML，因此 `<script>`、标签、引号和 `&` 只会显示为文字。
- `reset_url`、`login_url` token 在 HTML 中直接生成 `<a href="经属性转义的完整 URL">经文本转义的完整 URL</a>`；解码后的 href、可见文字及纯文本值必须逐字等于同一个输入 URL。
- 对管理员直接写入正文的 URL，只把“以空白分隔的完整 `http`/`https` token”识别为候选；候选必须经与变量实际值相同的 URL 校验，userinfo、空 host 或解析失败均使保存/预览返回 400，不得降级成可发送的普通文本。紧邻中文/英文标点、括号或其他文字的内容不猜测边界，保持普通文本。前端明确提示“若要自动生成链接，请将完整 URL 独立成行”。
- HTML 只生成最小 UTF-8 文档/片段、`<br>` 和 `<a>`，不生成样式、脚本、图片、远程资源或跟踪参数；不得对 URL 做补全、重编码、去参数、去片段、缩短或显示别名。
- 预览固定使用 `示例站点`、`https://example.invalid/reset/example-token?source=preview` 和 `https://example.invalid/login?source=preview`。响应及日志中不得读取或混入当前数据库的真实 token；`site_name` 也使用合成值，避免把预览变成运行配置探测接口。

### 3.6 MIME 报文与 SMTP 边界

- 把“构造报文字节”和“执行 SMTP 命令”拆开测试。业务邮件使用 `mime/multipart.Writer` 生成唯一 boundary，按 `text/plain` 后 `text/html` 的顺序写入 `multipart/alternative`；两个部分均为 `charset=utf-8`、`Content-Transfer-Encoding: quoted-printable`，正文统一 CRLF。
- 主题在拒绝换行后使用 `mime.QEncoding.Encode("utf-8", subject)`；From/To 继续使用已校验的单一邮箱并防头注入。顶层写入 `MIME-Version: 1.0`，报文尾部和 multipart 结束边界完整。
- 自动化测试用 `net/mail`、`mime/multipart`、`mime/quotedprintable` 和 HTML DOM 解析器解码实际字节，不以手工 `Contains` 代替结构验证。长行、中文、`&`、引号、查询参数、片段和百分号编码均须往返一致。
- SMTP 连接、STARTTLS/implicit TLS/plain、AUTH、30 秒总超时和阶段化 `sendError` 不改。底层发送函数改为接收已构造的完整报文字节，不能把模板正文拼到 SMTP 命令或日志。
- `SendTest` 的主题与正文可见内容保持当前固定值，仍发送单一 `text/plain` 报文；它不读取模板键、不受 scope 控制。其 header、正文换行和传输编码允许复用新的标准报文构造能力，不要求与旧实现逐字节相同。只有五类业务邮件切换到双版本模板报文。

### 3.7 管理 API 定稿

全部路由位于现有 `/api/admin/settings` 管理组，沿用 session + admin 双中间件，并统一写 `Cache-Control: no-store`。响应继续由 `OK`/`Fail` 包裹。

| 方法与路径 | 请求 | 成功数据 | 失败 |
|---|---|---|---|
| `GET /mail-templates` | 无 | `{ templates: TemplateView[], limits: { subject: 200, body: 10000 }, preview_values: { site_name, login_url, reset_url } }` | 整体读取失败 500 |
| `PUT /mail-templates/:kind` | `{ subject, body }` | 更新后的单个 `TemplateView` | 未知 kind/校验失败 400，超限 413，写入失败 500 |
| `DELETE /mail-templates/:kind` | 无 body | 恢复后的单个 `TemplateView` | 未知 kind 400，删除失败 500 |
| `POST /mail-templates/:kind/preview` | `{ subject, body }` | `{ subject, text_body, html_body }` | 未知 kind/校验失败 400，超限 413；绝不发送邮件 |

`TemplateView` 对前端至少返回 `id`、`label`、`scope`、有效 `subject/body`、`state`、`warning`、`subject_variables`、`body_variables`、`required_body_variables`。`damaged` 项返回默认 subject/body 与固定警告，不返回损坏原值。GET 返回的 `preview_values` 只能是 3.5 的固定合成值。DELETE 不使用请求体，重复调用保持成功。

接入层只做 JSON/body limit/状态码/响应头映射，全部领域操作进入注入的 `mail.Service`；因此 `SettingsHandler` 增加邮件模板服务依赖，禁止直接读写 `system_config`。路由级测试同时验证 401、403、400、413、500 和 no-store。

### 3.8 五分支发送接线矩阵

| 业务事件 | 邮件入口 | 模板 ID | 实际值 | scope 与次数 |
|---|---|---|---|---|
| 找回密码生成一次性链接 | `SendPasswordReset` | `password_reset` | `reset_url` | `password_reset`；每次请求至多一封 |
| 管理员单个/批量审批通过 | 新的 `SendApprovalApproved` | `approval_approved` | `site_name`、`login_url` | `approval_notify`；每个成功审批账号至多一封 |
| 管理员拒绝申请 | 新的 `SendApprovalRejected` | `approval_rejected` | `site_name` | `approval_notify`；每个成功拒绝账号至多一封 |
| 非审批路径首次激活，本地来源 | `SendWelcome` 内部分派 | `welcome_local` | `site_name`、`login_url` | `welcome`；维持现有调用次数 |
| 非审批路径首次激活，OIDC 来源 | `SendWelcome` 内部分派 | `welcome_oidc` | `site_name`、`login_url` | `welcome`；维持现有调用次数 |

`approval.MailSender` 去掉含义模糊且缺少登录 URL 的布尔 `SendApprovalNotify`，改成两个具名方法。`Approve` 事务提交后只调用 `SendApprovalApproved`，不再调用 `SendWelcome`；批量审批继续逐个复用 `Approve`。拒绝仍在删除提交后发送。`SendWelcome` 仅在 `source == "oidc"` 时选择 OIDC 分支，其余既有本地来源统一选择 local 分支，不扩大来源枚举。邮件 scope 未启用、无收件邮箱、模板/URL/SMTP 失败均不得回滚已完成的业务事务；现有 `user.Service.sendWelcomeIf` 必须取得用户 ID，三个生产调用点（自注册、OIDC 创建、管理员创建）同步传入，错误日志只记录用户 ID、模板 ID 和安全错误，不记录邮箱、主题、正文、URL 或 token。审批后的 Xray 回调顺序与既有失败隔离不变。

### 3.9 前端卡片、草稿与预览状态机

- 新建 `frontend/src/components/settings/MailTemplateCard.vue`，由 `SettingsView.vue` 在 SMTP 卡片及测试区之后渲染，避免继续扩大单文件业务逻辑。桌面为“编辑区 + 预览区”双栏，手机纵向排列；不引入富文本/Markdown/代码编辑器依赖。
- 首次加载一次 GET，默认选 `password_reset`。组件只维护当前分支一份草稿与服务端基线；主题或正文变化后标记 `mail-template` dirty，并在 300 ms debounce 后请求预览。
- 每次预览递增序号；只有最新序号且草稿仍相同的响应可以落 UI。新请求开始或任一校验/网络错误时立即清空旧预览；组件卸载时取消定时器。可用 `AbortController` 取消旧请求，但序号校验仍是最终防线。
- 切换分支时，如当前草稿未保存，弹窗只提供“丢弃并切换”和“继续编辑”；不悄悄缓存多分支草稿。保存成功后以响应重建基线并清 dirty；失败保持草稿与 dirty。恢复默认须二次确认，成功后用响应默认值重建草稿/基线并清 dirty；`default` 状态下恢复按钮禁用。
- 主题为普通 Input，正文为 Textarea，使用服务端 limits 设置 `maxlength/show-count`。变量按钮按光标位置插入；失焦或无法取得 selection 时插入末尾；只展示当前字段允许的变量，必需变量有明确标记。
- HTML 预览放入无权限的 sandbox iframe（`sandbox=""` + `srcdoc`），不得用 `v-html` 注入主页面；同时提供纯文本预览切换，主题单独显示。预览区域标注合成值和客户端差异，不允许点击后在管理页导航；完整示例 URL 不做视觉省略。
- 卡片固定提示：SMTP 测试邮件不使用这些模板；审批通过改受 `approval_notify` 控制且不会再叠发欢迎；密码重置链接实际有效期 1 小时；直接 URL 应独立成行。损坏状态使用 warning Alert，并允许直接用默认内容重新保存或恢复默认。
- `SettingsView` 的 `settingGroups.notifications.description` 更新为包含邮件内容。模板组件自行完成首次 GET、loading/error/retry，父页不重复取数；组件仅在自身服务端基线加载完成后通过 `dirty-change` 报告状态。父页使用专用 `onMailTemplateDirtyChange` 直接增删 `dirtyParts` 中的 `mail-template`，不得经过会在父页其他配置仍加载时忽略事件的 `settingsLoaded/suppressDirty` 门槛；页面离开确认继续统一统计，不另挂第二套全局事件。

### 3.10 导入、导出、备份与兼容合同

- 当前配置导出会遍历全部 `system_config`，因此有效模板覆盖天然进入加密 `ExportPayload.Config`，不新增格式字段且不提升 `FormatVersion`。Step 7 必须以解密后的测试 payload 证明五键往返，不能只依据实现推断。
- `ValidateTemplateOverrides(map)` 只识别 §3.1 固定的五个配置键：缺失键直接通过，存在键逐项执行与保存相同的严格 JSON/领域校验；任何一个已知键非法时，v1 `Import` 和 v2 `importV2` 都必须在 `DELETE FROM system_config` 之前失败，原库保持不变。其他未知键（包括未知的 `mail_template_` 前缀键）既不由本回调拒绝，也不由本回调改写，其行为继续服从现有配置导入路径。
- 上述边界是 2026-09-15 用户确认的 A 方案：Build28 不建立全局配置键注册表，也不借邮件功能改变未知配置键的兼容策略；当前导入实现与历史 Design1“未知键忽略并警告”描述之间的既有偏差不在本 Build 修复，Build28 的验收不得宣称该偏差已经闭环。
- v2 异步导入可先创建任务，但模板校验失败必须使任务终态为 failed，且不得开始覆盖事务或后处理；v1 同步路径直接返回安全错误。错误可指出模板 ID/字段，不回显内容。
- 完整备份已包含 SQLite 一致性快照，无需为模板增加文件；测试从 tar.gz 中取 `app.db` 并验证模板键，而不是只检查归档文件名。恢复工具不在 Build28 范围，备份证据仅证明快照保留。
- 旧数据库、旧配置导出及恢复默认后的数据库均因键缺失使用当前内置默认。无需迁移或一次性回填，避免默认文案未来更新后被历史副本锁死。
- R31-01 的真实随机密钥导出问题仍由 `ExportRelated1.md` 独立跟踪；Build28 只声明模板覆盖在现有格式内的隔离往返和非法覆盖保护。

### 3.11 文件级变更预算与明确非目标

预计修改/新增范围如下；Step 0.5 可因当前代码移动调整文件名，但不可无说明扩大业务域。

| 领域 | 预计文件 |
|---|---|
| 模板/渲染/MIME/SMTP | `backend/internal/mail/mail.go`、新增 `template.go`/`render.go`/`message.go` 及对应 `_test.go` |
| 通用配置存删 | `backend/internal/config/config.go`、`config_test.go` |
| 审批与业务接线 | `backend/internal/approval/approval.go`、`approval_test.go`、`backend/internal/user/user.go`、`user/oidc.go`、`user/admin.go` 及对应测试、`backend/internal/server/server.go`，必要的 auth 定向测试 |
| 管理 API | `backend/internal/server/settings.go`、`settings_mail_test.go`、架构路由测试 |
| 配置迁移/备份验证 | `backend/internal/config/export.go`、`export_test.go`、`export_v2_test.go`、`backend/internal/backup/backup_test.go` |
| 前端 | `frontend/src/api/settings.ts`、`frontend/src/components/settings/MailTemplateCard.vue`、组件测试、`SettingsView.vue`、`frontend/tests/settings-view.spec.ts` |
| 收口文档 | `Build28.md`、`Design5.md`、`AGENTS.md`，以及仅在仍需人工核验时的 `ProdTestList.md` |

不增加第三方模板引擎、富文本编辑器、后台草稿表、模板历史、站点 Logo、追踪像素、附件、自定义 headers、抄送/密送、自定义发件人/收件人、发送队列或重试机制。首版不解决跨管理员并发覆盖，也不改变 SMTP 连接配置和 scope 数据结构。

---

## 四、串行 TODO 与依赖

| Step | 目标 | 状态 |
|---|---|---|
| 0 | 创建 Design5 §七与 Build28，冻结范围 | ✅ 计划定稿 |
| 0.5 | 重核现状、冲突和受影响项；确认执行条件 | ☐ 未开始 |
| 1 | 五分支默认值、变量校验与独立覆盖存储 | ☐ 未开始 |
| 2 | 单一安全渲染器与合成数据预览 | ☐ 未开始 |
| 3 | 纯文本/HTML 双版本 MIME 报文与完整 URL 超链接 | ☐ 未开始 |
| 4 | 五分支真实发送接线与审批通过单封/开关归属 | ☐ 未开始 |
| 5 | 管理端模板读取、保存、恢复和预览 API | ☐ 未开始 |
| 6 | 通知页邮件内容卡片与前端交互 | ☐ 未开始 |
| 7 | 配置导出/导入、缺键回退和完整备份回归 | ☐ 未开始 |
| 8 | 联合门禁、隔离运行核验与文档收口 | ☐ 未开始 |

顺序固定为 0 → 0.5 → 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8。每一步先编写能暴露现状缺口的定向测试，再实现并执行该步验收；一步完成后才进入下一步。若 Step 0.5 发现与当前设计/Issue 合同冲突、现有工作区变动或需要改变已确认行为，先提交用户决策，不用本计划替代决策。

以后用户若明确授权“执行 Build28 Step 0.5～8”，该一次授权可覆盖这些 Step 的严格串行实施；执行中一旦命中 §六停止条件，该授权对受影响后续步骤暂停，待用户新决定后再继续。文档定稿、历史绿色门禁或单独授权 Step 0.5 均不自动授权 Step 1～8。

---

## 五、逐 Step 构建与验收

### Step 0：计划与决策冻结

- **目标：** 创建 Design5 §七和 Build28，将用户确认范围与邮件/整站迁移边界写清，并把实施接口、固定默认值、状态机和验收矩阵细化为本版定稿。
- **前置条件：** 用户明确要求写入 Design5 并在根目录创建下一个 Build。
- **产出文件与参考方案：** Design5.md、Build28.md。核对五分支、审批通过一封及必需登录链接、scope 归属、纯文本编辑、完整 URL 链接与无图片/附件范围均有准确条款；Build28 §3 固定首版实现合同。
- **验证命令：**

  ~~~bash
  git diff --check
  git status --short
  ~~~

- **验收标准：** 仅文档变更；实施定稿没有未决占位标记或互相冲突的接口描述；没有代码、配置或测试执行结果被标为已完成。

### Step 0.5：前置影响与现状核验

- **目标：** 执行前重核事实，防止按过期快照实施。
- **前置条件：** Step 0 文档已完成；获得执行本 Step 的授权。
- **产出文件与参考方案：** 在本 Step 的执行记录中写入当时 Git 分支、完整 HEAD、脏文件归属及本计划预计触及文件；逐项复核 §3.1～§3.11 对应的 SMTP/MIME、审批、user/auth 注入、settings 路由、导入 v1/v2、备份和前端测试布局。核对 Design5 §七、AGENTS、Issue16 与 ExportRelated1 边界。不修改业务代码。
- **验证命令：**

  ~~~bash
  git status --short
  git rev-parse --abbrev-ref HEAD
  git rev-parse HEAD
  rg -n 'SendWelcome|SendApprovalNotify|SendPasswordReset|SendTest|ScopeEnabled' backend/internal
  rg -n 'frontend_url|system_config|Content-Type|text/plain' backend/internal/config backend/internal/mail
  rg -n 'RegisterSettingsRoutes|NewExportService|ImportV2|CreateBackup' backend/internal/server backend/internal/config backend/internal/backup
  rg -n 'dirtyParts|loadSMTP|settings-view' frontend/src frontend/tests
  ~~~

- **验收标准：** 形成“合同条款 → 当前接线点 → 预计文件 → 定向测试”的核对表；现状差异已记录且没有未决的产品行为/文档冲突；保留原工作区改动。只有文件重排等不改变合同的差异可在记录后继续；若影响 §3 合同，本 Step 不通过并向用户确认。

### Step 1：模板默认值、白名单与持久化

- **目标：** 为五个固定分支建立默认主题/正文、变量校验和按分支独立的覆盖存储。
- **前置条件：** Step 0.5 通过。
- **产出文件与参考方案：** 按 §3.1～§3.4 实现五个定义、五个固定配置键、严格 JSON、200/10,000 字符上限、`default/customized/damaged` 三态及 `config.Service.Delete`。`backend/internal/config` 不反向导入 mail；不新增 migration。旧草稿中的 `{{site_logo}}` 按未知变量拒绝。读取损坏值回退默认并只记录安全元数据；数据库读取错误的发送降级与管理 API 整体失败边界按 §3.2 执行。参考伪代码：

  ~~~text
  Load(kind): valid override if present, otherwise Default(kind) with status
  Save(kind, subject, body): Validate(kind, subject, body); write only key(kind)
  Restore(kind): delete only key(kind)
  ~~~

- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/mail ./internal/config)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 失败优先测试证明现状无覆盖能力；表驱动测试覆盖五分支默认值、稳定顺序、全部边界长度、CRLF 规范化、控制字符、未知/未闭合/跨分支变量、必需变量重复/缺失、严格 JSON、三态、单键覆盖/恢复和数据库错误。保存任一分支不改其他分支、scope 或 `smtp_password`；无自定义配置的旧站点除审批通过已确认变化外仍使用原文案。

### Step 2：统一渲染与安全预览

- **目标：** 实际发送和管理员预览使用同一渲染规则。
- **前置条件：** Step 1 通过。
- **产出文件与参考方案：** 按 §3.4～§3.5 实现 token 扫描、一次替换、实际值 URL 校验、纯文本与最小 HTML 渲染。预览只使用三项固定合成值，不读取配置站点名或真实 token，也不持久化草稿。普通文本、`site_name`、URL 属性和 URL 可见文字分别按其上下文转义；直接 URL 仅按完整空白 token 识别，已识别候选若含 userinfo、空 host 或无法解析则整体校验失败。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/mail)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 五分支渲染测试覆盖变量重复、变量值含模板样式文本而不递归、HTML/属性特殊字符、LF/Tab、静态 URL 的空白与标点边界、长路径、查询参数、片段和百分号编码。解码后 href、可见文字与纯文本 URL 完全一致；空值、相对 URL、userinfo、非法 host/scheme 均拒绝且错误不含输入值。预览值固定、无真实 token，且测试直接证明预览和发送复用同一 `Render`。

### Step 3：双版本 MIME 与完整 URL 发信

- **目标：** 每封业务邮件同时提供完整 URL 的纯文本版本，以及链接文字和目标相同的可点击 HTML 版本。
- **前置条件：** Step 2 通过。
- **产出文件与参考方案：** 按 §3.6 将报文字节构造从 SMTP 会话分离。业务邮件用标准库 multipart + quoted-printable + RFC 2047；保留现有 SMTP 三模式、超时和错误阶段。固定测试邮件继续走单一 `text/plain`，不读取模板。参考结构：

  ~~~text
  multipart/alternative
    text/plain; charset=utf-8
    text/html; charset=utf-8 (a href=完整 URL，链接文字=完整 URL)
  ~~~

- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/mail)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 解析实际生成的 MIME 字节、quoted-printable 和 HTML DOM，而非仅匹配字符串；断言顶层 MIME、唯一 boundary、两个部分的顺序/类型/charset/编码、CRLF、结束边界、200 字符中文主题和 10,000 字符正文均可解码。链接三处一致；报文只有两个正文部分，没有图片、CID、附件、脚本或远程资源。现有 STARTTLS/implicit TLS/plain 定向测试继续通过；固定 SMTP 测试邮件的解码后主题/正文和单一纯文本结构保持不变，同时允许采用 RFC 2047、规范 CRLF 或传输编码。

### Step 4：真实业务发送路径与 scope

- **目标：** 使五分支真正映射到业务事件，审批通过只发一封且受 approval_notify 控制。
- **前置条件：** Step 3 通过。
- **产出文件与参考方案：** 按 §3.8 调整 `mail.Service` 具名入口、`approval.MailSender`、approval 单个/批量路径、reset/user 注入与 `server.New` 装配。审批通过传入当前 `frontend_url`，只走 `approval_approved`；其他首次激活才走 local/OIDC welcome。同步修改 `user.go`、`user/oidc.go`、`user/admin.go` 三个欢迎邮件生产调用点，为安全日志传递用户 ID。不得自动修改现有 scope。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/mail ./internal/approval ./internal/user ./internal/auth ./internal/server)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 用记录模板 ID/调用次数/RenderValues 的 mock 覆盖 §3.8 全矩阵：三类 scope 开关、无邮箱、单个与批量审批、local/OIDC 来源、通过单封、通过不再欢迎、拒绝与重置。非法必需链接时不进入 SMTP；模板读取/渲染/SMTP 失败均不阻断审批、激活或重置主流程，且日志断言不含邮箱、正文、URL/token。

### Step 5：管理员模板 API

- **目标：** 提供独立于 SMTP 配置的读取、保存、恢复、草稿预览接口。
- **前置条件：** Step 4 通过。
- **产出文件与参考方案：** 严格按 §3.7 的四组路由、请求/响应、64 KiB 限制和 no-store 实现；优先在 `settings.go` 现有领域内增加注入，若文件职责明显膨胀才拆 `settings_mail.go`。读取返回五分支有效文案与三态，损坏值不回显；预览不落库、不发送。Handler 不直接操作 `system_config`。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/server ./internal/mail ./internal/config)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 路由表逐项覆盖匿名 401、非管理员 403、未知 kind/非法 JSON/领域校验 400、64 KiB 超限 413、注入的存储错误 500、全部响应 no-store。GET 顺序/limits/变量元数据固定；PUT/DELETE 只改变目标键；DELETE 幂等；preview 使用合成值且数据库写入和 SMTP 调用均为零。原 SMTP GET/PUT/test API 与响应保持不变。

### Step 6：邮件内容卡片

- **目标：** 在现有通知页实现单分支编辑和即时视觉预览。
- **前置条件：** Step 5 通过。
- **产出文件与参考方案：** 按 §3.9 新建 `MailTemplateCard.vue` 及 API 类型/函数，SettingsView 只负责挂载、初始加载协调和 dirty 汇总。实现固定五项选择、三态、主题/正文、光标变量插入、300 ms 预览、最新序号保护、sandbox HTML/纯文本预览、保存、二次确认恢复及未保存切换提示。SMTP 测试区保持独立。
- **验证命令：**

  ~~~bash
  (cd frontend && npm run build)
  (cd frontend && npm test -- tests/mail-template-card.spec.ts tests/settings-view.spec.ts --reporter=dot)
  ~~~

- **验收标准：** 组件定向测试用 fake timers 和可控 Promise 覆盖首次读取/重试、初始分支/顺序、五分支元数据、光标与末尾插入、299/300 ms、后发先至、校验/网络失败清旧预览、切换取消/丢弃、保存失败/成功、恢复确认/失败/成功、damaged 警告、sandbox 属性和 dirty 事件。SettingsView 集成测试使用 memory router 覆盖卡片位于 SMTP 之后、通知说明、挂载和统一离开保护，并专门模拟“邮件模板已加载且开始编辑、父页其他设置请求仍未完成”，证明 dirty 不被初始化门槛吞掉；完整示例 URL 不省略、不含真实 token。不增加编辑器依赖。

### Step 7：导入导出与旧数据回退

- **目标：** 模板覆盖在现有配置迁移和完整备份中保留，旧配置仍用默认文案。
- **前置条件：** Step 6 通过。
- **产出文件与参考方案：** 按 §3.10 为 `ExportService` 增加只读 map 校验回调，并在 v1 `Import` 与 v2 `importV2` 的覆盖事务前调用；由 `server.New` 注入邮件层校验函数。导出格式与版本不变。备份只补 SQLite 快照内容测试，不新增实现文件。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/config ./internal/backup)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 解密 payload 后逐键断言五个合法覆盖；旧文件缺键、恢复默认后缺键、五个已知键的非法 JSON/领域覆盖均有测试。另用未知 `mail_template_` 键证明邮件校验回调既不拒绝也不改写该键，且测试只记录其继续服从现有导入行为，不把它表述为全局未知键策略已经符合 Design1。已知模板非法时，v1 直接失败且原配置不变，v2 task failed 且未调用事务后处理；合法导入后五分支仍为 customized。备份测试从 tar.gz 的 `app.db` 查询键和值。结论仅限模板配置保留，不扩大为整站迁移或宣称 R31-01 闭环。

### Step 8：联合门禁、运行核验与文档收口

- **目标：** 汇总代码、测试与 UI 的真实结果，更新设计和构建状态。
- **前置条件：** Step 1～7 均通过，且没有未决冲突。
- **产出文件与参考方案：** 在 Build28 每个 Step 下追加实际 commit 前工作树、文件、测试命令与结果，不改写原计划合同；同步 Design5 实施状态、AGENTS 入口及必要的 ProdTestList 人工项。使用隔离数据库与本地 SMTP mock 做 API/实际线格式/审批次数/模板预览 smoke；不得使用真实账号或把本地 mock 写成真实 SMTP/客户端证据。
- **验证命令：**

  ~~~bash
  (cd backend && go build ./... && go vet ./... && go test ./...)
  (cd frontend && npm run build && npm test -- --reporter=dot)
  test -d backend/internal/mail
  test -d frontend/src/components/settings
  scan_status=0
  rg -n --glob '!**/*_test.go' --glob '!**/*.spec.ts' 'site_logo|Content-ID|cid:' backend/internal/mail frontend/src/components/settings || scan_status=$?
  test "$scan_status" -eq 1
  git diff --check
  ~~~

- **验收标准：** 定向测试、后端 build/vet/full test、前端 build/full test、静态残留扫描与 diff-check 全部逐项记录；隔离 smoke 记录环境、请求与解码结果。失败或未执行证据单列。无真实 SMTP/收件箱/浏览器/客户端证据时，把对应人工项留在 ProdTestList，不能标完整人工验收。仅在 Step 1～8 全部工程范围完成、文档同步且无未决项后归档 Build28。

---

## 六、停止条件与变更记录

出现以下情况时先停止受影响 Step 并与用户确认：五分支或 scope 语义需要变化；需要图片/附件、自由 HTML、链接缩写/替代文字或新 SMTP 路径；Design5/AGENTS 出现无法调和的合同冲突；现有工作区变更可能被覆盖；真实导入导出问题迫使 Build28 越界修复。常规实现细节在已确认合同内由执行者选择，并记录依据。

| 版本 | 日期 | 说明 |
|---|---|---|
| v0.1 | 2026-09-14 | 按用户请求创建 Build28：承接 Design5 §七已确认的五分支模板、纯文本编辑预览、审批通过单封、可选 PNG/JPEG 站点 Logo；Step 0.5～8 尚未执行。 |
| v0.2 | 2026-09-14 | 设计复核后补审批通过登录链接、领域依赖、模板异常回退与导入校验、HTML 链接和 MIME 编码、预览失效与设置页脏状态的分步验收。 |
| v0.3 | 2026-09-14 | 按用户决定撤回 Logo/CID 设计，改为完整 URL 可见文字与目标相同的 HTML 超链接、完整 URL 纯文本备选；重写 MIME、预览及验收步骤，不执行代码。 |
| v0.4 | 2026-09-14 | 清理独立配置导出问题对本计划的引用，保留邮件模板导入导出与备份的既定验收范围。 |
| v0.5 | 2026-09-15 | 实施定稿：固定五个模板 ID/配置键/默认值、严格 JSON 与长度/变量校验、三态回退、统一渲染和 MIME 算法、管理 API、发送矩阵、前端草稿状态机、导入/备份接线及文件级测试验收；仍未授权代码构建。 |
| v0.6 | 2026-09-15 | 当前项目复核定稿：确认导入 A 方案，仅校验五个已知模板键且不处理全局未知键偏差；补齐 user 三个生产调用点与安全日志、独立 dirty 竞态、API JSON DTO、直接 URL userinfo 拒绝、测试邮件编码边界及不会误杀测试的静态门禁；仍未授权代码构建。 |
