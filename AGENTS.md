# AGENTS.md — VPN Subscription Management

本文件是 AI 编程助手的项目上下文。用简洁的陈述句描述产品需求、用户操作流程、UI 设计意图和编码约束。技术设计（第六章）、Docker 部署（第八章）均为构建时遵循的规范。

---

## 一、产品定义

这是一个自托管的 VPN 订阅管理系统，面向 ≤10 人的小团队。管理员通过 Web UI 配置 OIDC 认证、管理用户、上传订阅配置文件（Clash/V2Ray/Shadowrocket 格式）和分流规则。普通用户登录后可通过"一键导入"将订阅添加到 VPN 客户端。

**核心价值**: 用户获取一次订阅链接后，即使不登录也能持续获得最新配置（通过长期下载 Token）。管理员更新订阅版本，所有用户自动生效。

**设计原则**: 零配置启动（docker compose up -d），所有运行时配置通过 Web UI 完成，不使用 .env 文件。SQLite 嵌入式存储，无外部依赖。

---

## 二、用户角色与权限

系统有两种角色，权限来源于 SQLite users 表（非 JWT claims），每次请求实时查库。

### 2.1 管理员 (admin)

- 首个通过 OIDC 登录的用户自动成为管理员
- 始终拥有 is_advanced=true（后端强制，无法通过 UI 修改）
- 可访问管理面板：管理用户、订阅、平台、规则、OIDC 配置、查看日志
- 在首页可看到两种订阅类型（默认和高级，如果均已配置），用于预览
- 可创建独立的公开分享订阅链接（不绑定用户，任何人持有链接即可下载）

### 2.2 普通用户 (user)

- 首位通过 OIDC 登录的用户自动成为管理员（详见 2.1），此后登录的用户首次登录默认为普通用户
- 仅能访问首页和规则浏览页
- 看不到管理面板入口
- 订阅内容由管理员配置的全局级别决定，用户无权自行选择

### 2.3 订阅分级体系（基于用户级别）

订阅分为两个级别：**默认** 和 **高级**。用户级别由 `is_advanced` 字段决定，该字段由管理员在用户管理页面设置。

**用户级别规则**:
- 新用户首次登录 → `is_advanced=false` → **普通用户** → 在所有平台只能获得**默认订阅**
- 管理员将某用户设为 `is_advanced=true` → **高级用户** → 在所有平台只能获得**高级订阅**
- 用户无权自行选择或切换订阅级别。高级用户不能降级到默认订阅，普通用户不能获取高级订阅
- 管理员由于 `is_advanced` 始终为 true，在首页可预览两种订阅，但这是管理目的而非使用

**管理员设置流程**:
1. 管理员在用户管理页面对某个用户点击"编辑"
2. 切换 is_advanced 开关（普通 ↔ 高级）
3. 保存后立即生效，用户下次访问首页即可看到对应订阅

### 2.4 管理员强制覆盖（自定义订阅）

管理员可以为特定用户上传一份完全独立的订阅配置文件，覆盖该用户的默认/高级自动分配。该功能用于例外场景（如某个高级用户需要与其他高级用户不同的节点配置）。

**覆盖规则**:
- 管理员在用户管理页面为某个用户的某个平台上传自定义订阅文件，**上传时必须指定适用平台**
- 上传后该用户在被上传自定义文件的平台的订阅被替换为此自定义内容
- 一个用户可拥有多份自定义订阅（每个平台最多一份），同一平台再次上传则覆盖（更新版本）
- 自定义订阅同样支持版本管理（上传新版本 → 切换 → 保留最多 5 个历史版本）
- 自定义订阅如普通或高级订阅的处理流程一样，使用下载 Token，且一样能手动刷新 Token
- 管理员可删除自定义订阅 → 用户恢复到原本的默认/高级自动分配
- **用户首页显示逻辑**：自定义订阅替换对应平台卡片中的默认/高级订阅，该平台卡片显示"已被分配自定义订阅"提示 + 自定义订阅的三个操作按钮。没有自定义订阅的平台照常显示默认或高级订阅
- **管理员首页显示逻辑**：管理员在已有自定义订阅的平台上，同时显示默认 + 高级 + 自定义三组按钮（默认和高级用于预览）。没有自定义订阅的平台显示默认 + 高级两组按钮

### 2.5 独立分享订阅

管理员可以创建不关联任何用户的独立分享订阅链接。这些链接像分享链接一样工作：任何人持有该链接即可下载订阅配置，无需登录。

**独立分享订阅是管理面板中的独立模块**，与自定义订阅（2.4 节）是不同的功能。自定义订阅绑定用户+平台，替换用户的默认/高级自动分配；独立分享订阅不绑定任何用户，管理员手动将下载链接分发给需要的人员。

**使用场景**: 管理员想给外部人员临时访问；或创建一个团队公共链接方便分发。

**设计规则**:
- 独立分享订阅不区分平台，不区分 Clash/Shadowrocket 等格式 — 管理员上传什么，用户就获得什么
- 管理员在管理面板创建"分享订阅"：填写名称、上传订阅文件
- 每个分享订阅自动生成一个独立的下载 Token
- 分享订阅同样支持版本管理（同平台订阅逻辑）
- 管理员可随时刷新 Token（旧链接立即失效，生成新 Token）；也可吊销 Token（`DELETE /admin/share/:id/token`，使该分享订阅链接立即不可用，但保留订阅文件与版本历史）；或删除整个分享订阅（级联删除文件 + Token）
- 下载端点无需认证，仅通过 `?token=` 验证
- 分享订阅不区分默认/高级 — 它就是一份独立的订阅内容
- 分享订阅在管理面板中有**独立的列表页面和路由**（如 `/admin/shares`），与平台订阅分开管理
- 分享订阅的版本管理也在独立路由下（如 `/admin/shares/:id/versions`）

### 2.6 字段说明

- **is_advanced** (users 表): 布尔字段，决定用户是普通用户(false)还是高级用户(true)。管理员由后端强制设为 true。这是订阅分级的核心字段。
- **groups** (users 表): JSON 数组，预留扩展用。当前版本未使用。

---

## 三、用户操作流程

### 3.1 首次部署（管理员视角）

管理员执行 docker compose up -d 启动服务后：

1. 访问网站任意路径 → 后端检测到系统未配置（system_config 表中 configured=false）→ 前端路由守卫自动跳转到 /setup 首次配置页
2. 在 Setup 页面选择 OIDC 提供商类型（Keycloak/Auth0/通用 OIDC）
3. 填写提供商参数（Base URL、Realm/域名、Client ID、Client Secret）
4. 填写回调地址和前端地址（用于 OIDC 回调重定向）
5. 点击"测试连接"验证 OIDC 配置是否正确
6. 测试通过后点击"完成配置" → 系统自动生成 JWT_SECRET，写入 system_config 表，标记 configured=true（注意：admin_initialized 仍为 false，由首个用户登录时触发，详见 5.x OIDC/认证约束）
7. 跳转到登录页 → 自动重定向到 OIDC 提供商登录
8. 首次登录的用户自动成为管理员 → 进入首页
9. 管理员进入管理面板：
   - 创建订阅（为每个平台添加默认订阅和高级订阅）
   - 设置用户 is_advanced 标记
   - （可选）创建分享订阅

### 3.2 日常使用流程（普通用户视角）

1. 用户访问网站 → 未登录则跳转到登录页
2. 点击登录 → 重定向到 OIDC 提供商
3. 在提供商页面输入用户名密码（或已有 SSO 会话则自动完成）
4. 认证成功后回调 → 后端完成 code exchange，302 重定向到前端中转页 `/auth/callback?token=xxx` → 前端提取 JWT 存入 localStorage，replaceState 清空 URL → 进入首页
5. 首页展示所有平台卡片（如 Clash Verge、v2rayNG、Shadowrocket）
6. 每个平台卡片显示**一份**订阅 — 由用户 is_advanced 决定：
   - 普通用户 (is_advanced=false) → 显示"默认订阅"
   - 高级用户 (is_advanced=true) → 显示"高级订阅"
   - 若管理员为该平台的该用户分配了自定义订阅 → 显示"已被分配自定义订阅"提示 + 自定义订阅按钮（替换默认/高级自动分配）
7. 用户操作（对每个平台）：
   - **一键导入**: 拼接客户端 scheme URL → window.location.href 跳转 → 浏览器唤起 VPN 客户端。拼接规则：取平台 `client_schemes` 中第一个 scheme，将订阅下载 URL 经 `encodeURIComponent` 编码后拼接到 `?url=` 参数，如 `clash://install-config?url=https%3A%2F%2Fexample.com%2Fapi%2Fv1%2Fsubscriptions%2Fclash-verge%2Fdownload-token%3Ftoken%3Dxxx`
   - **复制链接**: 弹出对话框显示订阅 URL → 用户手动复制到客户端
   - **刷新链接**: 旧下载 Token 失效 → 生成新 Token（用于 Token 泄露后重置）
8. 用户此后无需登录，VPN 客户端通过下载 Token 持续获取最新配置

### 3.3 管理员操作流程

管理员通过首页顶部"管理面板"按钮进入管理后台。管理后台使用侧边栏导航布局。

**订阅管理**:
- 列表页展示所有平台订阅（先按平台分组，再按类型分组：默认/高级）
- 创建订阅：填写 ID（小写字母数字连字符）、名称、类型（default/advanced）、平台
- 每个平台应同时配置默认订阅和高级订阅，分别服务普通用户和高级用户
- 点击订阅进入版本管理：上传新版本 → 预览 → 切换当前版本 → 删除旧版本
- 最多保留 5 个历史版本，超过自动删除最旧的
- 当前激活版本有视觉高亮标识
- 不可删除最后一个版本
- 订阅管理页支持两种方式创建新版本：
  - **前端文本编辑**：在 textarea 中直接编辑当前版本的订阅配置文本，保存后自动创建新版本并切换（适用于临时或细微的调整）
  - **文件上传**：通过 el-upload 上传新的订阅配置文件，上传后自动创建新版本并切换
  - 两种方式均遵循相同的版本管理逻辑（自动创建新版本、最多保留 5 个版本、超出删最旧的）

**分享订阅管理** (独立分享订阅):
- 独立列表页面和路由（如 `/admin/shares`），与平台订阅分开
- 创建分享订阅：填写名称、上传订阅文件
- 每个分享订阅自动生成独立的下载 Token
- 支持版本管理（独立路由 `/admin/shares/:id/versions`：上传、切换、删除，最多 5 个版本）
- 操作按钮：复制分享链接、刷新 Token（旧链接失效）、吊销 Token（链接不可用但文件保留）、删除
- 下载端点无需认证

**平台管理**:
- 展示和编辑 VPN 客户端平台
- 每个平台配置多个 Client Scheme（如 clash://install-config?url=、v2rayng://install-config?url=）
- 一键导入时使用 client_schemes 中第一个 scheme 拼接 URL
- 系统初始化时自动创建 3 个默认平台：clash-verge、v2rayng、shadowrocket
- **每个平台可配置一个下载链接**（download_url），管理员在平台管理页面设置，用于首页"下载客户端"按钮。该链接存储在 platforms 表的 download_url 字段

**用户管理**:
- 用户列表（自动通过 OIDC 登录创建，管理员不可手动创建）
- 可编辑用户：设置 is_advanced 标记（普通/高级切换）
- groups 字段仅存储不编辑，若用户未设置 groups 则在 UI 中不显示该字段
- 可为用户上传自定义订阅文件（需指定适用平台，覆盖默认/高级自动分配）
- 可删除用户的自定义订阅（恢复默认/高级自动分配）
- 可删除用户（级联删除其下载 Token 和自定义订阅）。**管理员自我保护**：禁止删除自己、禁止删除最后一个管理员、禁止修改自己 role（详见 5.x）
- 可吊销用户所有下载 Token（强制用户重新获取）
- 管理员自身的 is_advanced 不可修改（始终 true）

**分流规则管理**:
- 分流规则模块为通用设计，当前版本支持 Shadowrocket（.conf 格式），表结构与 API 保留扩展性（rules 表可加 client_type 字段以支持更多客户端）
- 版本管理逻辑同订阅
- **每条规则拥有独立的下载 Token**，可独立轮替。规则下载 API 无需登录（`?token=` 验证 rule_tokens）
- 管理员可在规则管理页面轮替（刷新）某条规则的下载 Token，旧 Token 立即失效
- Token 在未手动轮替时保持长期有效
- 前端 `/rules` 页面需登录后访问（路由守卫拦截），对所有登录用户（含普通用户）可见，用户可选择不同版本单独下载
- 普通用户仅可浏览和下载，不可管理版本

**速率限制配置**:
- 在管理面板中可查看和修改速率限制参数
- 默认值：登录 API 同 IP 每分钟 10 次，下载 API 同 IP 每分钟 20 次
- 配置存储在 system_config 表中

**OIDC 配置**:
- 查看和修改 OIDC 提供商参数
- 测试连接按钮
- 切换提供商类型（Keycloak ↔ Auth0 ↔ 通用 OIDC）时**保留已填写字段**，仅切换显示对应提供商的特定字段
- 切换提供商后自动尝试发现端点（Discovery URL）自动填充
- Client Secret 以 AES-256-GCM 加密存储在 SQLite 中，UI 回显时脱敏

**日志查看**:
- 按日期筛选访问日志
- 记录所有下载请求（用户订阅下载、分享订阅下载、自定义订阅下载、规则下载，均需记录；分享订阅和规则下载 user_id 可为空）
- 日志含 status（success/failed）+ error_reason 字段，可区分下载成功/失败及失败原因（token_invalid/file_not_found/version_not_found/rate_limited 等）
- 默认保留 90 天，后端自动清理超过 90 天的日志记录

### 3.4 订阅下载逻辑详解

**所有下载端点统一行为**: 无论通过何种途径访问下载链接，后端始终以 `Content-Type: text/plain; charset=utf-8` 输出订阅配置的纯文本内容（类似 GitHub RAW 输出），不触发浏览器文件下载（不使用 `Content-Disposition: attachment`）。VPN 客户端可直接读取该纯文本响应。

下载订阅有三种途径：

**途径一：JWT 下载（Web UI 预览，需登录）**
- Authorization: Bearer header → 验证 JWT → 读取用户 is_advanced 决定类型 → 返回 current 版本纯文本
- 管理员可通过 ?type= 参数预览 default 或 advanced
- 此端点用于 Web UI 中预览订阅内容（含浏览器直接查看），非客户端实际使用的下载地址
- ?type= 与 ?token= 不会同时出现（JWT 端点走 Bearer header，不走 query param token）

**途径二：Download Token 下载（客户端用，无需登录）**
- ?token={download_token} → 查 download_tokens 表验证 → 返回 current 版本纯文本
- Token 绑定了用户+平台+订阅类型。当 Token 的 custom_sub_id 非空时，返回该用户在该平台的自定义订阅内容，而非默认/高级订阅

**途径三：独立分享订阅下载（公开，无需登录）**
- ?token={share_token} → 查 share_tokens 表验证 → 返回 current 版本纯文本
- 与途径二独立，使用单独的表和端点
- 端点: GET /api/v1/share/:id/download?token=

**Download Token 机制（用户订阅）**:
- download_tokens 表包含字段：token, user_id, platform, type, custom_sub_id（可空）
- 用户首次在首页点击"一键导入"或"复制链接"时生成
- 同一用户+平台+订阅类型的 Token 会复用
- Token 始终绑定 platform，custom_sub_id 仅标识该 Token 是否关联了一份自定义订阅
- 当 custom_sub_id 非空时，type 字段置为 NULL（自定义订阅不区分默认/高级，type 不参与下载判定），Token 复用唯一键为 user+platform+custom_sub_id
- 当 custom_sub_id 为空时，type 为 default/advanced，下载该 Token 返回的是该 platform+type 对应的默认/高级订阅
- Token 无过期时间，除非管理员吊销或用户被删除
- 用户点击"刷新链接"时旧 Token 立即失效，生成新 UUID
- **自定义订阅 Token 刷新**：用户在首页对自定义订阅点击"刷新链接"时，同样触发 Token 刷新（旧 Token 失效，生成新 UUID）。后端通过 download_tokens 表中 custom_sub_id 字段定位并替换对应 Token
- 下载时始终返回 current 软链接指向的最新版本
- **用户 is_advanced 变更时（升级/降级）**：后端自动删除该用户所有旧 Token，用户下次访问首页时按新级别重新生成 Token（防止旧级别 Token 绕过分级限制）
- **删除用户自定义订阅时**：级联删除所有 custom_sub_id 指向该订阅的 Token，用户下次访问该平台时重新生成默认/高级 Token

**Share Token 机制（分享订阅）**:
- 管理员创建分享订阅时自动生成
- 管理员可手动刷新 Token（旧 Token 立即失效，生成新 Token）
- 管理员可吊销 Token（`DELETE /admin/share/:id/token`，使该分享订阅链接立即不可用，但保留订阅文件与版本历史）
- 管理员可删除分享订阅（级联删除文件 + Token）
- 下载时始终返回 current 版本

---

## 四、UI 设计意图

### 4.1 整体风格

使用 Element Plus 组件库。支持暗色模式（通过 useTheme composable 切换，localStorage 持久化偏好）。深色/浅色主题通过 body class + Element Plus 暗色变量实现。

### 4.2 首页 (Home.vue)

顶部水平栏：左侧标题"VPN 订阅"+订阅更新时间戳。右侧：管理面板按钮（仅管理员可见）、用户名称+角色标签（普通用户/高级用户/管理员）、退出按钮、暗色模式切换按钮。

主体：平台卡片网格布局（响应式，大屏 3 列，中等 2 列，小屏 1 列）。每个平台卡片从上到下：
- 平台名称（文字，不使用图标）
- 平台描述
- 订阅区段（普通用户只看到一种，管理员可看到多种用于预览），区段内容取决于：
  - 普通用户 (is_advanced=false) + 无自定义 → 显示"默认订阅"标签 + 三个按钮
  - 高级用户 (is_advanced=true) + 无自定义 → 显示"高级订阅"标签 + 三个按钮
  - 普通/高级用户 + 该平台有自定义订阅 → 显示"已被分配自定义订阅"提示 + 自定义订阅的三个按钮（自定义订阅替换默认/高级自动分配）
  - 管理员 + 该平台无自定义 → 显示"默认订阅"+"高级订阅"两组按钮（均用于预览）
  - 管理员 + 该平台有自定义 → 显示"默认订阅"+"高级订阅"+"自定义订阅"三组按钮
  - 订阅未配置降级：若某平台未配置 advanced 订阅，高级用户访问该平台时卡片显示"高级订阅未配置，请联系管理员"，不生成 Token；管理员首页该平台 advanced 按钮显示"未配置"。不降级到 default。对称地，若某平台未配置 default 订阅，普通用户访问该平台时卡片显示"默认订阅未配置，请联系管理员"，不生成 Token；管理员首页该平台 default 按钮显示"未配置"。不升级到 advanced
  - 三个按钮：一键导入 (primary)、复制链接 (default)、刷新链接 (warning, text, small)
- **下载客户端按钮**：每个平台卡片底部显示一个"下载客户端"链接按钮（仅当管理员配置了该平台的 download_url 时显示），点击跳转到管理员设定的外部下载地址

"一键导入"按钮行为：直接触发 window.location.href 跳转到拼接的 scheme URL。"复制链接"按钮：弹出对话框，显示完整 URL，点击输入框自动复制到剪贴板。"刷新链接"按钮：显示 loading 状态，调用 API 刷新 Token，成功后刷新平台列表。

### 4.3 管理面板 (Manage.vue)

左侧固定宽度侧边栏（200px），使用 Element Plus el-menu 组件，router 模式。菜单项：订阅管理、分享订阅、平台管理、用户管理、规则管理、OIDC配置、日志查看。当前路由对应的菜单项高亮（渐变紫色背景）。

移动端：侧边栏默认隐藏，通过顶部栏的汉堡按钮切换显示。

### 4.4 表单交互模式

所有创建/编辑使用 Element Plus el-dialog 弹窗，内嵌 el-form。提交前前端校验必填字段。删除操作统一使用 ConfirmDialog.vue 组件（需传入标题、提示文字、确认回调），不使用 ElMessageBox.confirm。

版本管理页面使用 el-upload 组件上传文件，限制文件大小 50MB（适用于所有上传场景：订阅版本、规则版本、自定义订阅、分享订阅版本）。后端也校验文件大小防绕过前端。上传后自动刷新版本列表。当前激活版本在列表中用绿色标签或边框高亮标识。

### 4.5 用户管理页面 (UserManage.vue)

用户列表每行显示：用户名、邮箱、角色标签、is_advanced 标签（普通/高级）、操作按钮。

操作按钮组：
- 编辑（设置 is_advanced；groups 字段仅存储不编辑，若用户未设置 groups 则在 UI 中不显示该字段）
- 上传自定义订阅（弹出对话框 → 选择适用平台 → 选择文件 → 上传 → 覆盖该平台已有自定义订阅）
- 删除自定义订阅（仅当用户有自定义订阅时显示，恢复默认/高级自动分配）
- 吊销所有下载 Token（强制用户下次访问首页重新获取链接，与首页的"刷新单个平台 Token"区分）
- 删除用户（**管理员自我保护**：禁止管理员删除自己、禁止删除最后一个管理员、禁止管理员修改自己的 role，后端三重校验，详见 5.x）

### 4.6 分享订阅管理页面

独立列表，每行显示：名称、创建时间、当前版本号、Token 状态（有效/已吊销）。

操作按钮组：
- 版本管理（进入版本管理页面）
- 复制分享链接（仅 Token 状态为"有效"时可用）
- 刷新 Token（ConfirmDialog："刷新后旧链接立即失效，确定？"）
- 吊销 Token（ConfirmDialog："吊销后该分享链接立即不可用，订阅文件保留。确定？"）
- 删除（ConfirmDialog，级联删除文件+Token）

创建分享订阅对话框：填写名称 → 上传第一个版本的文件 → 自动生成 Token。

### 4.7 规则管理页面 (RulesManage.vue)

规则列表，每行显示：规则名称、客户端类型（client_type，当前显示 Shadowrocket）、当前版本号、Token 状态（有效/已轮替）。

操作按钮组：
- 版本管理（进入 RuleVersions 页面）
- 复制下载链接（含 ?token=，供用户填入客户端）
- 轮替 Token（ConfirmDialog："轮替后旧链接立即失效，确定？"）
- 删除（ConfirmDialog，级联删除文件 + rule_tokens）

创建规则对话框：填写名称、选择客户端类型（当前仅 Shadowrocket 可选）→ 上传第一个版本文件 → 自动生成 rule_token。

### 4.8 规则版本管理页面 (RuleVersions.vue)

布局与 SubVersions 一致：版本列表（版本号、创建时间、更新时间、当前标识）、上传新版本、文本编辑创建新版本、切换 current、删除旧版本、预览内容。

### 4.9 用户规则页面 (Rules.vue)

面向所有登录用户（含普通用户）的规则浏览页面。路由 `/rules`，需登录（路由守卫拦截）。

页面展示规则列表，每条规则显示：规则名称、客户端类型、当前版本信息。用户可选择不同版本单独下载（仅浏览和下载，不可管理版本）。

### 4.10 暗色模式

通过 useTheme.js composable 实现。切换时同步更新：document.documentElement.classList、localStorage 偏好、Element Plus 全局主题。所有页面和组件自动跟随。

---

## 五、编码约束

以下条目是强制性的，修改代码时必须遵守。

**安全**:
- 所有含用户输入的路径操作必须经过 sanitizePath()（防路径穿越）
- 所有 /api/v1/admin/* 路由必须有 AdminRequired 中间件
- 不可硬编码密钥，使用自动生成后存 SQLite（业务配置如 OIDC/密钥/速率限制一律通过 Web UI 存 SQLite，不用 .env）。仅监听端口等运维参数可选通过环境变量覆盖（如 PORT，默认 8080）
- Logger 必须将 ?token= 查询参数值脱敏为 ***
- OIDC ClientSecret 必须 AES-256-GCM 加密存储。**加密密钥复用 JWT_SECRET**（Setup 完成时自动生成的 JWT_SECRET 同时用于 JWT 签名和 AES-256-GCM 加密，取前 32 字节做 AES-256 key）。简单单密钥管理，无需额外密钥配置
- **真实客户端 IP 获取**：部署在反向代理后，后端必须信任并解析 X-Forwarded-For / X-Real-IP header 获取真实客户端 IP（用于速率限制和日志记录）。Gin 通过 (gin.Engine).SetTrustedProxies(["127.0.0.1"]) 配置信任本机反代，c.ClientIP() 自动解析
- **下载缓存策略**：所有下载端点必须返回 `Cache-Control: no-store, no-cache, must-revalidate` + `Pragma: no-cache`，确保 VPN 客户端和中间代理每次请求获取最新 current 版本，不缓存旧配置

**OIDC/认证**:
- 管理员 is_advanced 始终为 true（后端 UpdateUser 强制设置）
- 首个通过 OIDC 登录的用户自动成为管理员。**判定逻辑**：登录时检查 system_config.admin_initialized 标记，若不为 true 则该用户 role=admin、is_advanced=true，并写入 admin_initialized=true。即使 users 表被清空，因标记已存在也不会再产生新管理员（更安全）。**Setup 完成时只置 configured=true，不写 admin_initialized**（admin_initialized 由首个用户登录时触发）。JWT 有效期 7 天，过期后需重新 OIDC 登录
- OIDC state 必须持久化到 SQLite（不能用内存 map），10 分钟 TTL，后端定时清理过期记录。**CSRF 防护**：state 存入 oidc_state 表的同时通过 HttpOnly Cookie 下发给浏览器，回调时三重校验（Cookie state == query state == DB 记录）
- **PKCE code_verifier 关联**：code_verifier 与 state 一同存入 oidc_state 表，回调时按 state 查表取 code_verifier 用于 token exchange，用后立即删除该 state 记录（防重放）
- JWT 认证仅通过 Authorization: Bearer header
- JWT claims 最小集：仅存 `user_id` + 标准声明（exp/iat）。role、is_advanced 等权限信息不放入 claims，每次请求由 AuthRequired 中间件查库获取
- 用户下载 Token 认证仅通过 ?token= query param（/subscriptions/:platform/download-token）
- 分享订阅 Token 认证仅通过 ?token= query param（/share/:id/download）
- AuthRequired 中间件必须实时查库，不许缓存用户权限
- GetCurrentUser 必须读数据库，不许直接用 JWT claims

**后端 Handler**:
- 创建：BindJSON → 校验必填字段 → 校验 ID 格式 [a-z0-9-]+ → 重复检查冲突返回 409
- 更新：至少校验名称非空
- 错误码：400=校验错误，401=JWT 缺失/无效/过期（前端 401 拦截自动登出），403=普通用户访问 /admin/*（AdminRequired 中间件），409=重复，429=速率限制，500=服务器内部错误
- **管理员自我保护**：禁止管理员删除自己（c.GetUserID == :id 拒绝）、禁止删除最后一个管理员（后端校验 role=admin 数量 ≥ 1）、禁止管理员修改自己的 role。三重保护防误锁
- 列表响应用 gin.H{"key": value} 包裹
- 成功操作用 gin.H{"success": true}
- 下载端点必须调用 logAccess() 记录访问日志

**版本管理**:
- 版本号用 nextVersion() 取最大编号+1（不可用 len(versions)+1）
- 最多保留 MAX_VERSIONS 个版本（默认 5），超出删最旧的
- 不可删除最后一个版本
- 编辑当前版本文本内容后保存 → 自动创建新版本并切换
- **并发安全**：nextVersion 计算与 versions JSON 更新必须在单个 SQLite 事务中完成，对 subscriptions/rules/custom_subscriptions/share_subscriptions 行级锁（UPDATE ... WHERE id=?），防止并发上传导致版本号冲突或文件覆盖
- **current 软链接原子切换**：切换 current 时用临时 symlink + rename() 原子替换（创建 current.new → rename(current.new, current)），避免下载过程中读到半切换状态

**订阅逻辑**:
- 用户订阅类型由 is_advanced 自动决定，前端不可让用户选择类型
- 自定义订阅绑定平台、优先级高于默认/高级自动分配
- 管理员删除用户自定义订阅后，用户自动恢复到 is_advanced 对应的默认/高级订阅
- 分享订阅与用户无关，下载端点仅检查Token是否有效

**级联删除规则**:
- 删除用户 → 级联删除其所有 download_tokens、custom_subscriptions 及对应版本文件
- 删除自定义订阅 → 级联删除 custom_sub_id 指向该订阅的 download_tokens（用户下次访问重新生成默认/高级 Token）
- 删除 subscriptions 订阅 → 级联删除该 platform+type 对应的 download_tokens（custom_sub_id 为空的 Token）
- 删除 rules 规则 → 级联删除 rule_tokens
- 删除 share_subscriptions 分享订阅 → 级联删除 share_tokens + 版本文件
- 删除 platforms 平台 → 级联删除该平台的 subscriptions（含版本文件）、download_tokens、custom_subscriptions（含版本文件）。删除前需 ConfirmDialog 二次确认

**速率限制响应**:
- 超出速率限制时返回 HTTP 429 Too Many Requests + `Retry-After` header（秒数，提示何时可重试）
- 登录端点（/auth/login）：响应体附 JSON `{"error":"请求过于频繁，请稍后再试"}` 供前端弹提示
- 下载端点（订阅/分享/规则/自定义下载）：响应体返回纯文本错误信息（如 `rate limit exceeded, retry after N seconds`），供 VPN 客户端识别。日志记录 status=failed, error_reason=rate_limited

**前端**:
- Vue 模板属性中不可使用双引号转义 \”，必须用「」或计算属性
- v-model 中不可使用可选链 ?. ，用 v-if 守卫
- 删除确认必须用 ConfirmDialog.vue 组件
- 登出必须调用 userStore.logout(router)，传入 router 实例
- 文件上传必须手动设置 Content-Type: multipart/form-data

**Go 工程**:
- 修改 go.mod 后必须运行 go mod tidy
- 修改代码后必须运行 go build ./... 验证编译通过
- isValidID() 必须在 utils 包，不能放在 handler 包里

---

## 六、技术设计大纲

本章是项目的工程蓝图，技术选型、目录结构、数据模型、API 契约、文件存储均为构建时遵循的规范。如实际编码中发现不可行，需先与本项目维护者确认后再调整。

### 6.1 技术选型

**后端**: Go + Gin + zerolog
- SQLite: `modernc.org/sqlite`（纯 Go 驱动，零 CGO，便于静态编译到 distroless）
- OIDC: `coreos/go-oidc/v3` + `golang-jwt/jwt/v5`（PKCE 流程 + JWT 签发验证）

**前端**: Vue 3 (Composition API + `<script setup>`) + Vite + Element Plus + Pinia + Vue Router
- HTTP: Axios（统一 baseURL `/api/v1`，401 拦截自动登出）
- 主题: 自实现 `useTheme` composable（Element Plus 暗色变量 + localStorage 持久化）

**约束**: 所有依赖须可纯静态编译/打包，不依赖 CGO、不依赖外部数据库进程。

### 6.2 目录结构

```
backend/
├── cmd/server/main.go             入口，Setup/Normal 双模式（依据 system_config.configured 切换）
├── internal/
│   ├── auth/oidc_service.go       OIDC 认证 + PKCE + JWT 签发/验证
│   ├── handler/                   HTTP handler（按业务域拆分）
│   ├── service/                   业务逻辑层（按业务域拆分）
│   ├── repository/                数据访问层（每张表一个 repo）
│   ├── middleware/                Logger, Recovery, CORS, CacheControl, NoCacheForDownloads, AuthRequired, AdminRequired, RateLimit
│   ├── models/types.go            所有结构体定义
│   ├── router/router.go           Setup 模式路由 + Normal 模式路由
│   └── utils/                     env, crypto (AES-256-GCM), sanitizePath, isValidID
frontend/
└── src/
    ├── router/index.js           beforeEach 守卫（Setup 检测 + 登录恢复 + Admin 校验）
    ├── services/api.js           Axios 封装 + 分组 API + 401 拦截
    ├── stores/user.js            Pinia 用户状态
    ├── composables/useTheme.js   暗色模式
    ├── components/               ConfirmDialog, OIDCSwitchDialog, UploadModal
    └── views/                    15 个页面：Setup, Login, Home, Rules(用户), Manage(布局), SubList, SubVersions, ShareList, ShareVersions, PlatformManage, UserManage, RulesManage, RuleVersions, OIDCConfig, Logs
```

**分层职责**: handler（HTTP 协议层，BindJSON/响应）→ service（业务规则、版本管理逻辑）→ repository（SQL、文件读写）。handler 不直接操作数据库，service 不感知 HTTP。

### 6.3 数据库设计

SQLite 数据库文件路径：`/app/data/vpn.db`（位于 vpn-data volume 内，与其他数据文件同目录）。

**表清单**:

| 表名 | 主键 | 用途 |
|------|------|------|
| system_config | key | key-value 存储：OIDC 配置、JWT_SECRET、速率限制参数、admin_initialized 标记、configured 标记 |
| users | user_id | 用户：username、email、role(admin/user)、is_advanced、groups(JSON) |
| platforms | id | 平台：name、description、client_schemes(JSON 数组)、download_url，默认 3 个（clash-verge、v2rayng、shadowrocket） |
| subscriptions | id | 订阅：name、UNIQUE(platform, type)，type=default/advanced，versions JSON |
| rules | id | 分流规则：name、client_type(预留扩展)、versions JSON、created_at |
| access_logs | id (AUTOINC) | 访问日志：user_id(可空)、ip、download_type、platform(可空)、share_subscription_id(可空)、rule_id(可空)、status(success/failed)、error_reason(可空)、created_at；按日期查询，自动清理 90 天以上 |
| oidc_state | state | OIDC PKCE：code_verifier + nonce，10min TTL |
| download_tokens | token | 用户下载令牌：user_id + platform + type(可空，custom_sub_id 非空时为 NULL) + custom_sub_id(可空) |
| custom_subscriptions | id | 用户自定义订阅：user_id + platform、UNIQUE(user_id, platform)、versions JSON、created_at |
| share_subscriptions | id | 独立分享订阅：name、versions JSON、created_at |
| share_tokens | token | 分享订阅下载令牌：share_subscription_id |
| rule_tokens | token | 规则下载令牌：rule_id（独立表，便于未来多 Token 扩展） |

**关键字段语义**:

- **system_config.configured**: 布尔标记。Setup 完成时置 true，后端据此切换 Setup/Normal 模式路由。
- **system_config.JWT_SECRET**: Setup 完成时随机生成，同时用于 JWT 签名和 AES-256-GCM 加密。若该表被清空，所有用户 Token 立即失效。
- **system_config.admin_initialized**: 布尔标记。首位管理员诞生后置 true，此后即使 users 表被清空也不再产生新管理员。
- **system_config 速率限制键**: `rate_limit_login`（默认 10/min）、`rate_limit_download`（默认 20/min），管理员可在后台修改。下载速率限制统一应用于所有下载端点（订阅/分享/规则/自定义）。
- **system_config OIDC 配置键**: `provider_type`（keycloak/auth0/generic）+ 各提供商独立字段（keycloak_base_url、keycloak_realm、auth0_domain、generic_issuer、client_id、**keycloak_client_secret_encrypted / auth0_client_secret_encrypted / generic_client_secret_encrypted**（各提供商 Client Secret 独立加密存储，切换时保留已填值）、redirect_uri、frontend_url）。切换提供商类型时按 provider_type 读取对应字段。
- **users.username / users.email**: 由 OIDC 提供商返回的 claims 提取，UserManage 页面展示。同一 OIDC 用户重复登录按 user_id（OIDC sub）去重。
- **users.is_advanced**: 布尔。决定用户获得默认(false)还是高级(true)订阅。管理员由后端强制为 true。
- **platforms.client_schemes**: JSON 数组，如 `["clash://install-config?url=", "clash-verge://install-config?url="]`。一键导入使用第一个 scheme。
- **platforms.download_url**: 可空。非空时首页平台卡片显示"下载客户端"按钮。
- **download_tokens.custom_sub_id**: 可空。非空时下载返回该用户在该平台的自定义订阅内容，此时 type 字段置为 NULL（不参与下载判定），Token 复用唯一键为 user+platform+custom_sub_id；为空时 type 为 default/advanced，返回 platform+type 对应的默认/高级订阅。
- **access_logs.download_type**: 枚举值（subscription/share/custom/rule），标识下载途径。分享订阅和规则下载的 user_id 为空。
- **access_logs.share_subscription_id / rule_id**: 可空。分别用于 share 和 rule 下载日志标识具体被下载的分享订阅/规则（因这两类下载无 platform 字段，需独立标识用于日志可观测性）。
- **access_logs.status / error_reason**: status 枚举（success/failed），标识下载是否成功。失败时 error_reason 记录原因（token_invalid/file_not_found/version_not_found 等），便于日志页排查问题。
- **versions JSON 数组结构**: subscriptions/rules/custom_subscriptions/share_subscriptions 表的 versions 字段均为 JSON 数组，每个版本对象含：`{ version: int, file_path: string, created_at: datetime, updated_at: datetime }`。version 为版本号（取已有最大编号+1），file_path 为版本文件相对路径，updated_at 在切换 current 或编辑时更新。`GET /user/update-time` 取所有订阅 current 版本的 updated_at 最大值。

### 6.4 API 端点

所有端点前缀 `/api/v1`。响应格式：列表 `gin.H{"key": [...]}`，单项直接返回对象，成功 `gin.H{"success": true}`，错误 `gin.H{"error": "描述"}`。错误码：400=校验错误，401=JWT 缺失/无效/过期，403=普通用户访问 /admin/*，409=重复，429=速率限制，500=服务器内部错误。

**公开（无认证）**:
- `GET /health` — 健康检查
- `GET /system/status` — 系统状态，返回 `{ configured: bool }`，Setup 和 Normal 模式下均可用，前端路由守卫据此判断是否跳转 /setup
- `GET /platforms` — 平台列表（返回 id、name、description、client_schemes、download_url，不含 Token）
- `GET /rules` — 规则列表（含当前版本信息）。此 API 公开供客户端调用；前端 `/rules` 页面本身需登录后访问（路由守卫拦截）
- `GET /rules/:id/download?token=` — 规则下载（?token= 验证 rule_tokens，速率限制）

**OIDC 认证（速率限制）**:
- `GET /auth/login` — 跳转 OIDC 提供商
- `GET /auth/callback` — OIDC 回调，code exchange 后 302 重定向到前端中转页 `/auth/callback?token=xxx`（前端提取 JWT 存 localStorage 后 replaceState 清空 URL，注意此后端 API 路径 `/api/v1/auth/callback` 与前端中转页路由 `/auth/callback` 不同）
- `GET /auth/me` — 当前用户信息

**用户（需 JWT Bearer）**:
- `GET /user/platforms` — 平台列表（返回 id、name、description、client_schemes、download_url + 当前用户的 download_token + 是否有自定义订阅标记，首页使用此端点）
- `GET /user/update-time` — 首页更新时间戳（所有订阅当前版本 updated_at 的最大值）
- `POST /user/refresh-token` — 刷新指定平台下载 Token。请求体 `{ platform, type }`，type 为 default/advanced。当用户在该平台有自定义订阅时，后端自动刷新自定义订阅的 Token（通过 download_tokens 表中 custom_sub_id 字段定位），而非默认/高级 Token

**订阅下载（速率限制）**:
- `GET /subscriptions/:platform/download` — JWT 下载（Web UI 预览，管理员可用 ?type= 切换 default/advanced）
- `GET /subscriptions/:platform/download/preview` — 浏览器直接预览（响应格式与 /download 相同，均为 text/plain，无 HTML 包裹）
- `GET /subscriptions/:platform/download-token?token=` — Token 下载（客户端实际使用）

**独立分享订阅下载（无认证，速率限制）**:
- `GET /share/:id/download?token=` — 验证 share_tokens，返回 current 版本纯文本

**管理员（需 JWT + AdminRequired）**:
- 用户管理: `GET/POST/PUT/DELETE /admin/users/*`
  - 自定义订阅: `POST /admin/users/:id/custom-subscription`（上传，需指定平台）、`POST /admin/users/:id/custom-subscription/versions`（新版本，支持文件上传或 JSON 文本）、`DELETE /admin/users/:id/custom-subscription`、`GET/PUT/DELETE /admin/users/:id/custom-subscription/versions/:versionId`、`POST /admin/users/:id/custom-subscription/refresh-token?platform=xxx`（刷新该平台自定义订阅 Token）
- 订阅管理: `/admin/subscriptions/*`（含版本管理：`POST /versions` 上传新版本或编辑文本创建新版本、`PUT /versions/:versionId/current` 切换 current、`DELETE /versions/:versionId` 删除旧版本）
- 分享订阅管理: `/admin/share/*`（CRUD + 版本管理，端点结构同订阅管理）、`POST /admin/share/:id/refresh-token`、`DELETE /admin/share/:id/token`
- 平台管理: `/admin/platforms/*`
- 规则管理: `/admin/rules/*`（含版本管理，端点结构同订阅管理）、`POST /admin/rules/:id/refresh-token`（轮替 rule_tokens）
- 系统配置: `GET /admin/oidc-config`、`POST /admin/test-oidc`、`POST /admin/system/configure`、`POST /admin/system/switch-provider`、`GET/PUT /admin/system/rate-limit`
- 日志: `GET /admin/logs`（按日期筛选）

**版本创建端点说明**: `POST /:resource/versions` 同时支持两种 Content-Type：
- `multipart/form-data`：文件上传方式，file 字段为订阅/规则配置文件
- `application/json`：文本编辑方式，body 为 `{ content: string }`，后端写入新版本文件
两种方式均创建新版本号并自动切换为 current。

**自定义订阅下载途径**: 自定义订阅复用 `GET /subscriptions/:platform/download-token?token=`（download_tokens.custom_sub_id 非空时返回该用户在该平台的自定义订阅内容），无需独立下载端点。

### 6.5 版本文件存储

所有版本文件存储在 `/app/data/` 下，按业务域分目录。版本号取已有最大编号 + 1（不可用 `len(versions)+1`）。每个版本独立文件，`current` 软链接指向当前激活版本。

```
data/
├── vpn.db                          SQLite 数据库
├── subscriptions/{id}/             v1.conf, v2.conf, ... + current.conf (软链接)
├── rules/{id}/                     v1.conf, v2.conf, ... + current.conf (软链接)
├── custom/{user_id}/{platform}/    自定义订阅：v1.conf, v2.conf, ... + current.conf (软链接，按用户+平台隔离)
└── shares/{id}/                    分享订阅：v1.conf, v2.conf, ... + current.conf (软链接)
```

**版本管理统一规则**（适用于订阅、规则、自定义订阅、分享订阅）:
- 上传新版本 → 自动创建新版本号 → 切换为 current
- 最多保留 5 个历史版本，超出自动删除最旧的
- 不可删除最后一个版本
- 当前激活版本有视觉高亮标识
- 编辑当前版本文本内容后保存 → 自动创建新版本并切换
- 删除订阅/规则/分享 → 级联删除其所有版本文件

---

## 七、CI/CD 设计

> **实现优先级**: CI/CD 和 Docker 部署将在核心功能开发完成后实施。当前阶段优先实现完整的业务功能。

项目使用 GitHub Actions 自动构建 Docker 镜像并推送到 GHCR。

**触发条件**: push 到 main 或 beta 分支，push v* 标签（如 v1.0.0），手动 workflow_dispatch。

**构建策略**: matrix build 同时构建 backend 和 frontend 两个镜像。使用 docker/metadata-action 生成标签，docker/build-push-action 构建推送。

**镜像标签**: main 分支 → {service}:main 和 {service}:latest。beta 分支 → {service}:beta。版本标签 v1.0.0 → {service}:1.0.0、{service}:1.0、{service}:1。

**Dockerfile 结构**: 多阶段构建。后端：golang 编译 → distroless 运行。前端：node 构建 → nginx 静态文件服务（前端容器**只服务静态文件，不做反代**；`/api/` 的分流由部署机外部 NGINX 完成，详见第八章）。

---

## 八、Docker 部署

### 8.1 部署架构（最终决策）

采用**外部 NGINX 分流 + 双容器（端口绑 127.0.0.1）**架构。`/api/` 与静态文件的分流由部署机上已有的外部 NGINX 承担，容器内**不做任何反代**。两个容器的端口只绑定宿主机的 `127.0.0.1`，外部网络不可达，仅同机外部 NGINX 能转发到它们。

```
用户浏览器
   ↓ HTTPS
外部 NGINX (部署机已有, vpn.example.com)   ← 在此处做 /api 分流
   ├─ /api/*   → http://127.0.0.1:8080  (backend 容器, Gin API)
   └─ /*       → http://127.0.0.1:8081  (frontend 容器, 纯静态文件)
   ↓
docker-compose (两个容器, 端口均绑 127.0.0.1)
   ├─ backend  :8080  → Gin API (不对外)
   └─ frontend :8081  → nginx 纯静态文件服务 (无 proxy_pass)
```

**架构约束（强制）**:
- 对外只暴露一个端口（外部 NGINX 的 443/80）。backend 的 8080 和 frontend 的 80 必须以 `127.0.0.1:` 前缀绑定，禁止直接映射到宿主机公网接口
- frontend 容器内的 nginx **只服务静态文件**，不得包含任何 `proxy_pass` 或 `/api/` location
- `/api/` 的反代职责完全由外部 NGINX 承担
- 前端代码统一使用相对路径 `/api/v1/...`，禁止硬编码 host:port，确保开发/生产一致

### 8.2 外部 NGINX 配置（参考）

外部 NGINX 承担 TLS 终止与 `/api` 分流，将所有流量转发到本机两个容器端口：

```nginx
server {
    listen 443 ssl;
    server_name vpn.example.com;
    # ssl_certificate ... (已有的 TLS 配置)

    # API 请求转发到后端容器
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # 其他请求转发到前端容器 (静态文件)
    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### 8.3 docker-compose.yml（参考）

两个容器端口均绑定 `127.0.0.1`，不对外暴露：

```yaml
services:
  backend:
    build: { context: ./backend }
    ports:
      - "127.0.0.1:8080:8080"     # 只绑本机, 外部不可达
    volumes:
      - vpn-data:/app/data
    restart: unless-stopped

  frontend:
    build: { context: ./frontend }
    ports:
      - "127.0.0.1:8081:80"       # 只绑本机, 外部不可达
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  vpn-data:
```

### 8.4 前端容器 nginx.conf（参考）

前端容器内 nginx **只服务静态文件 + SPA 回退**，无任何反代：

```nginx
server {
    listen 80;
    server_name _;
    root /usr/share/nginx/html;

    location / {
        try_files $uri $uri/ /index.html;   # SPA history 模式回退
    }
}
```

### 8.5 开发环境

本地开发时前端用 Vite dev server (5173)，后端 `go run` (8080)。`vite.config.js` 配置 proxy 将 `/api/` 转发到本地后端，前端代码与生产完全一致（均用相对路径）：

```js
// vite.config.js
server: {
  proxy: {
    '/api': 'http://localhost:8080'
  }
}
```

### 8.6 数据流示例

部署在 `https://vpn.example.com` 时，以登录为例：

1. `GET https://vpn.example.com/` → 外部 NGINX `/` → frontend:8081 → 返回 index.html
2. `GET https://vpn.example.com/api/v1/auth/login` → 外部 NGINX `/api/` → backend:8080 → OIDC 跳转
3. OIDC 回调 `GET https://vpn.example.com/api/v1/auth/callback?code=xxx` → 外部 NGINX `/api/` → backend:8080 → 302 到 `/`
4. `GET https://vpn.example.com/` → frontend:8081 → 首页加载完成

全程同源（同一域名），无跨域问题。

### 8.7 持久化存储

项目使用**单一 Docker Volume**（`vpn-data`）挂载所有持久化数据：SQLite 数据库（`/app/data/vpn.db`）、订阅配置文件（`data/subscriptions/`）、规则文件（`data/rules/`）、自定义订阅文件（`data/custom/`）、分享订阅文件（`data/shares/`）。不拆分多个 volume，所有存储内容统一挂载到 `/app/data`。

