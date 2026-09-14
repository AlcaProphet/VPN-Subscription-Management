# VPN 订阅管理系统功能构建计划（Build28：业务邮件内容定制）

> **文档定位：** 本文仅承接 [Design5.md](Design5.md) §七已确认的业务邮件内容定制。§一～§六整站迁移仍是候选，不属于 Build28。编码约束见 [AGENTS.md](AGENTS.md)，历史 SMTP 预留见 [Issue16.md](docs/reports/Issue/Issue16.md)。
> **执行状态（2026-09-14）：** 只创建计划，未授权执行代码构建；Step 0 已完成文档准备，Step 0.5～8 全部未开始。以后获授权执行时严格按顺序逐 Step 构建和验收，不跳步、不并行，不把计划写成已验证功能。

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

## 三、串行 TODO 与依赖

| Step | 目标 | 状态 |
|---|---|---|
| 0 | 创建 Design5 §七与 Build28，冻结范围 | ✅ 文档准备 |
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

---

## 四、逐 Step 构建与验收

### Step 0：计划与决策冻结

- **目标：** 创建 Design5 §七和 Build28，将用户确认范围与邮件/整站迁移边界写清。
- **前置条件：** 用户明确要求写入 Design5 并在根目录创建下一个 Build。
- **产出文件与参考方案：** Design5.md、Build28.md。核对五分支、审批通过一封及必需登录链接、scope 归属、纯文本编辑、完整 URL 链接与无图片/附件范围均有准确条款。
- **验证命令：**

  ~~~bash
  git diff --check
  git status --short
  ~~~

- **验收标准：** 仅文档变更；没有代码、配置或测试执行结果被标为已完成。

### Step 0.5：前置影响与现状核验

- **目标：** 执行前重核事实，防止按过期快照实施。
- **前置条件：** Step 0 文档已完成；获得执行本 Step 的授权。
- **产出文件与参考方案：** 在 Build28 状态栏记录当时 Git 分支/HEAD/脏文件、现有 SMTP/MIME/审批路径、链接生成与 API/测试布局；核对 Design5 §七、AGENTS 及 Issue16 历史预留，列出影响处理。不修改业务代码。
- **验证命令：**

  ~~~bash
  git status --short
  rg -n 'SendWelcome|SendApprovalNotify|SendPasswordReset|SendTest|ScopeEnabled' backend/internal
  rg -n 'frontend_url|system_config|Content-Type|text/plain' backend/internal/config backend/internal/mail
  ~~~

- **验收标准：** 现状差异已记录；没有未决的行为/文档冲突；保留原工作区改动。若冲突存在，本 Step 不通过并向用户确认。

### Step 1：模板默认值、白名单与持久化

- **目标：** 为五个固定分支建立默认主题/正文、变量校验和按分支独立的覆盖存储。
- **前置条件：** Step 0.5 通过。
- **产出文件与参考方案：** backend/internal/mail 中定义五个稳定模板 ID、内置默认值、领域校验与模板服务；backend/internal/config 只补必要的通用配置删除/读取能力，不反向导入 mail，维持当前 mail → config 单向依赖。每个分支用一个 JSON 配置键保存 subject/body，键不存在即返回当前内置默认；保存时拒绝未知 ID、主题中的 URL 变量、正文跨分支变量、主题换行/空值、正文超长及缺少 reset_url/login_url 的必需分支，旧草稿中的 `{{site_logo}}` 作为未知变量拒绝。审批通过的内置默认正文也须补登录链接。上限在本 Step 固定并记录，避免前后端漂移。读取到损坏覆盖或数据库错误时，发送回退默认并输出不含正文/链接的安全告警；管理员读取同时获得该分支异常状态。参考伪代码：

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

- **验收标准：** 失败优先测试证明现状无覆盖能力；实现后五分支默认/覆盖/恢复、主题/正文变量白名单、审批通过默认链接和损坏覆盖回退通过。保存任一分支不改其他分支或 smtp_password；无自定义配置的旧站点除审批通过已确认变化外仍使用原文案。

### Step 2：统一渲染与安全预览

- **目标：** 实际发送和管理员预览使用同一渲染规则。
- **前置条件：** Step 1 通过。
- **产出文件与参考方案：** backend/internal/mail 中以固定占位符扫描/替换构建 Render(kind, draft, values)，不执行通用模板语法；分别生成主题、纯文本正文和最小 HTML 正文。预览只注入 example.invalid 的完整示例链接及合成站点名，不获取真实 reset token，也不持久化草稿。主题渲染后再次校验单行；普通正文文本进入 HTML 前必须转义并保留换行。reset_url/login_url 先校验为 HTTP(S) URL，再在纯文本中展开为完整 URL，在 HTML 中生成链接目标与可见文字均为该完整 URL 的真实锚点；直接书写且以空白/换行分隔的完整 HTTP(S) URL 同样生成锚点，编辑区提示将其独立成行。属性与可见文字分别编码，客户端解码后的两者须与原 URL 完全一致，不得截短、改写或换成标签文字。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/mail)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 五分支渲染、未知变量/控制字符、HTML 转义、占位符及独立成行直接书写的长路径/含查询参数链接在 HTML 解码后其 href/可见文字与纯文本正文均为完整原 URL、实际必需链接为空或非法 scheme 时渲染拒绝且错误不含令牌、示例链接和预览无真实令牌测试通过；预览结果与发送调用同一变量解析及转义规则。

### Step 3：双版本 MIME 与完整 URL 发信

- **目标：** 每封业务邮件同时提供完整 URL 的纯文本版本，以及链接文字和目标相同的可点击 HTML 版本。
- **前置条件：** Step 2 通过。
- **产出文件与参考方案：** backend/internal/mail 的报文构造从 SMTP 会话中分离，复用现有发送阶段和超时处理。所有业务模板的同一次渲染结果生成 `text/plain` 与安全转义的 `text/html` 两种正文，按 `multipart/alternative` 顺序发送；不读取站点图标或生成图片/附件部分。统一处理 MIME 唯一边界、Content-Transfer-Encoding、CRLF/行长和中文主题的 RFC 2047 编码；From/To/Subject 保持邮件头注入防护。HTML 源码按规范转义，但客户端解码后的 href 和链接可见文字必须与纯文本中的同一完整 URL 一致。参考结构：

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

- **验收标准：** 解析实际生成的 MIME 字节及 HTML DOM，而非仅匹配字符串；占位符与独立成行直接书写的 URL、长路径、查询参数、编码字符、长文本、中文主题/正文及传输编码均有定向断言。链接的 HTML href、可见文字和纯文本 URL 解码后完全一致；报文只有两个正文部分，没有图片、CID 或附件；固定 SMTP 测试邮件仍是原文案。

### Step 4：真实业务发送路径与 scope

- **目标：** 使五分支真正映射到业务事件，审批通过只发一封且受 approval_notify 控制。
- **前置条件：** Step 3 通过。
- **产出文件与参考方案：** backend/internal/mail/mail.go、backend/internal/approval/approval.go、必要的 user/auth/server 装配。审批通过改调用 approved 通知一次，将当前 siteContext 的 loginURL 传入模板，不再在同一事件调用欢迎；审批拒绝仍走 rejected；其他首次激活根据来源选 local/OIDC welcome；重置保持一次性、1 小时链接和失败不阻断主流程语义。提示已有站点仅开启 welcome 时审批通过将不再发信，不自动迁移 scope。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/mail ./internal/approval ./internal/user ./internal/auth ./internal/server)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 用 mock 断言三类 scope 的开/关、通过审批单封且带登录链接、通过后不再发欢迎、其他激活仍按来源选欢迎、拒绝/重置不回归；实际必需链接无效时不调用 SMTP，SMTP/渲染失败不阻断审批/激活/重置主流程。

### Step 5：管理员模板 API

- **目标：** 提供独立于 SMTP 配置的读取、保存、恢复、草稿预览接口。
- **前置条件：** Step 4 通过。
- **产出文件与参考方案：** backend/internal/server/settings.go 或按邮件域拆分的独立 Handler；仅在管理组路由下提供 GET/PUT/DELETE/preview。读取返回五分支可编辑的默认/有效文案、customized 与异常状态，不返回任何真实令牌；预览接受草稿而不落库、不发送，响应禁止缓存。请求进入邮件业务服务，Handler 不直接操作 system_config。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/server ./internal/mail ./internal/config)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 匿名 401、非管理员 403、非法模板 400；保存与恢复只改变目标模板键；预览使用合成值且不触发 SMTP；原 SMTP GET/PUT/test API 合同不变。

### Step 6：邮件内容卡片

- **目标：** 在现有通知页实现单分支编辑和即时视觉预览。
- **前置条件：** Step 5 通过。
- **产出文件与参考方案：** frontend/src/api/settings.ts 的模板接口类型；frontend/src/views/admin/SettingsView.vue 中引入邮件内容卡片（可抽独立组件，避免继续扩张设置页）。提供五项选择、状态、主题/正文、按分支插入变量、展示完整示例 URL 的草稿预览、保存、恢复默认和未保存切换提醒，并接入页面现有 dirtyParts 离开保护。编辑区提示直接书写的 URL 独立成行；预览调用后台同一渲染器并做短延迟合并请求。旧响应不能覆盖新草稿，校验或请求失败时清除旧预览并显示原因，标注客户端显示仅供参考。SMTP 连接测试区保持独立，提示仅欢迎开关的站点审批通过行为已变、重置链接实际只有 1 小时。
- **验证命令：**

  ~~~bash
  (cd frontend && npm run build)
  (cd frontend && npm run test)
  ~~~

- **验收标准：** 组件定向测试以真实输入/切换/延迟响应覆盖五分支、变量插入、即时草稿预览、异常配置提示、保存/恢复和离开保护；仅在实际保存后状态变化，错误响应不留下过期预览，浏览器预览显示完整示例 URL 且不展示真实重置 token。不无故引入编辑器框架。

### Step 7：导入导出与旧数据回退

- **目标：** 模板覆盖在现有配置迁移和完整备份中保留，旧配置仍用默认文案。
- **前置条件：** Step 6 通过。
- **产出文件与参考方案：** backend/internal/config/export_test.go、backend/internal/backup/backup_test.go、backend/internal/server/server.go 与必要的最小实现。核对执行时的配置导入导出实现；有覆盖时往返后逐分支保留，无覆盖/旧文件缺键时回退；恢复默认后导出不再含该覆盖。导入含非法模板覆盖时在整体覆盖事务前拒绝，现存数据库中的损坏覆盖则按 Step 1 回退与提示。沿用 ExportService 已有的构造注入/回调模式，由 server.New 注入邮件层的模板校验函数；config 不反向导入 mail，校验函数不读取或回显真实令牌。
- **验证命令：**

  ~~~bash
  (cd backend && go test ./internal/config ./internal/backup)
  (cd backend && go build ./... && go vet ./...)
  ~~~

- **验收标准：** 模板键的隔离往返、旧文件缺键、非法模板导入前拒绝且原配置不变、完整备份保留均通过；不扩大为整站迁移。

### Step 8：联合门禁、运行核验与文档收口

- **目标：** 汇总代码、测试与 UI 的真实结果，更新设计和构建状态。
- **前置条件：** Step 1～7 均通过，且没有未决冲突。
- **产出文件与参考方案：** Build28.md 的逐 Step 结果、Design5.md 实施状态、AGENTS.md 文档入口、必要的 ProdTestList.md 人工待核验项目。使用隔离数据和本地 SMTP mock 验证实际线格式、审批发送次数及模板预览，不使用真实账号/SMTP/Production 代替自动化。若用户提供真实服务和客户端，再单独记录真实 SMTP 接受、收件箱投递、完整链接的点击与显示。
- **验证命令：**

  ~~~bash
  (cd backend && go build ./... && go vet ./... && go test ./...)
  (cd frontend && npm run build && npm run test)
  git diff --check
  ~~~

- **验收标准：** 全部工程门禁通过；失败或未执行的证据单列；无真实浏览器/客户端证据时保留人工待核验，不能标完整验收。Build28 仅在本范围全部完成且文档同步后按项目归档规则处理。

---

## 五、停止条件与变更记录

出现以下情况时先停止受影响 Step 并与用户确认：五分支或 scope 语义需要变化；需要图片/附件、自由 HTML、链接缩写/替代文字或新 SMTP 路径；Design5/AGENTS 出现无法调和的合同冲突；现有工作区变更可能被覆盖；真实导入导出问题迫使 Build28 越界修复。常规实现细节在已确认合同内由执行者选择，并记录依据。

| 版本 | 日期 | 说明 |
|---|---|---|
| v0.1 | 2026-09-14 | 按用户请求创建 Build28：承接 Design5 §七已确认的五分支模板、纯文本编辑预览、审批通过单封、可选 PNG/JPEG 站点 Logo；Step 0.5～8 尚未执行。 |
| v0.2 | 2026-09-14 | 设计复核后补审批通过登录链接、领域依赖、模板异常回退与导入校验、HTML 链接和 MIME 编码、预览失效与设置页脏状态的分步验收。 |
| v0.3 | 2026-09-14 | 按用户决定撤回 Logo/CID 设计，改为完整 URL 可见文字与目标相同的 HTML 超链接、完整 URL 纯文本备选；重写 MIME、预览及验收步骤，不执行代码。 |
| v0.4 | 2026-09-14 | 清理独立配置导出问题对本计划的引用，保留邮件模板导入导出与备份的既定验收范围。 |
