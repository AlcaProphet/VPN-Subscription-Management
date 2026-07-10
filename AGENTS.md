# AGENTS.md — VPN Subscription Management

本文件是 AI 编程助手的项目上下文。用简洁的陈述句描述产品需求、用户操作流程、UI 设计意图和编码约束。技术细节（数据库、API、路由）列为参考，不作为强制规范。

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

- 后续通过 OIDC 登录的用户，首次登录默认为普通用户
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
- 一个用户最多一份自定义订阅，再次上传则覆盖（更新版本）
- 自定义订阅**不在首页作为独立模块显示**
- **管理员首页显示逻辑**：管理员可看到默认 + 高级 + 自定义（三组按钮），其中默认和高级用于预览，自定义是管理员自身的自定义订阅（如有）

### 2.5 独立公开分享订阅

管理员可以创建不关联任何用户的独立订阅链接。这些链接像分享链接一样工作：任何人持有该链接即可下载订阅配置，无需登录。

**独立特殊订阅是管理面板中的独立模块**，不集成到首页的平台卡片体系中。管理员在后台创建和管理自定义订阅，然后手动将下载链接分发给对应用户。用户通过链接直接获取订阅内容（无需登录）。

**使用场景**: 管理员想给外部人员临时访问。

**设计规则**:
- 自定义订阅独立于平台，不区分 Clash/Shadowrocket 等格式 — 管理员上传什么，用户就获得什么
- 管理员在管理面板创建"分享订阅"：填写名称、上传订阅文件
- 每个分享订阅自动生成一个独立的下载 Token
- 分享订阅同样支持版本管理（同平台订阅逻辑）
- 管理员可随时刷新 Token（旧链接立即失效）或吊销 Token（删除该分享链接）
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
6. 测试通过后点击"完成配置" → 系统自动生成 JWT_SECRET，写入 system_config 表，标记 configured=true
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
4. 认证成功后回调 → 前端提取 JWT，存入 localStorage → 进入首页
5. 首页展示所有平台卡片（如 Clash Verge、v2rayNG、Shadowrocket）
6. 每个平台卡片显示**一份**订阅 — 由用户 is_advanced 决定：
   - 普通用户 (is_advanced=false) → 显示"默认订阅"
   - 高级用户 (is_advanced=true) → 显示"高级订阅"
7. 用户操作（对每个平台）：
   - **一键导入**: 拼接客户端 scheme URL → window.location.href 跳转 → 浏览器唤起 VPN 客户端
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
- 订阅管理页需要有简单的文本编辑能力，以便应对临时或细微的调整。编辑当前版本的文本内容后保存 → 自动创建新版本并切换

**分享订阅管理** (独立公开链接):
- 独立列表页面和路由（如 `/admin/shares`），与平台订阅分开
- 创建分享订阅：填写名称、上传订阅文件
- 每个分享订阅自动生成独立的下载 Token
- 支持版本管理（独立路由 `/admin/shares/:id/versions`：上传、切换、删除，最多 5 个版本）
- 操作按钮：复制分享链接、刷新 Token（旧链接失效）、删除
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
- 可删除用户（级联删除其下载 Token 和自定义订阅）
- 可吊销用户所有下载 Token（强制用户重新获取）
- 管理员自身的 is_advanced 不可修改（始终 true）

**规则管理**:
- Shadowrocket 分流规则（.list 格式）
- 版本管理逻辑同订阅
- 规则下载通过独立 Token 管理，默认无需登录即可下载（`?token=` 验证）
- 管理员可在规则管理页面轮替（刷新）下载 Token，旧 Token 立即失效
- Token 在未手动轮替时保持长期有效
- 公开页面 `/rules` 可供所有登录用户浏览，用户可选择不同版本单独下载
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
- 默认保留 90 天，后端自动清理超过 90 天的日志记录

### 3.4 订阅下载逻辑详解

**所有下载端点统一行为**: 无论通过何种途径访问下载链接，后端始终以 `Content-Type: text/plain; charset=utf-8` 输出订阅配置的纯文本内容（类似 GitHub RAW 输出），不触发浏览器文件下载（不使用 `Content-Disposition: attachment`）。VPN 客户端可直接读取该纯文本响应。

下载订阅有四种途径：

**途径一：JWT 下载（Web UI 预览，需登录）**
- Authorization: Bearer header → 验证 JWT → 读取用户 is_advanced 决定类型 → 返回 current 版本纯文本
- 管理员可通过 ?type= 参数预览 default 或 advanced
- 此端点仅用于 Web UI 中预览订阅内容，非客户端实际使用的下载地址
- ?type= 与 ?token= 不会同时出现（JWT 端点走 Bearer header，不走 query param token）

**途径二：Download Token 下载（客户端用，无需登录）**
- ?token={download_token} → 查 download_tokens 表验证 → 返回 current 版本纯文本
- Token 绑定了用户+平台+订阅类型（或自定义订阅 ID）

**途径三：分享订阅下载（公开，无需登录）**
- ?token={share_token} → 查 share_tokens 表验证 → 返回 current 版本纯文本
- 与途径二独立，使用单独的表和端点
- 端点: GET /api/v1/share/:id/download?token=

**途径四：Preview 下载（Web UI 预览，需登录）**
- 同途径一，用于浏览器中直接查看订阅内容

**Download Token 机制（用户订阅）**:
- 用户首次在首页点击"一键导入"或"复制链接"时生成
- 同一用户+平台+订阅类型的 Token 会复用
- Token 无过期时间，除非管理员吊销或用户被删除
- 用户点击"刷新链接"时旧 Token 立即失效，生成新 UUID
- 下载时始终返回 current 软链接指向的最新版本

**Share Token 机制（分享订阅）**:
- 管理员创建分享订阅时自动生成
- 管理员可手动刷新 Token（旧 Token 立即失效）
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
- 订阅区段（普通用户只看到一种，管理员可看到两种用于预览），区段内容取决于：
  - 普通用户 (is_advanced=false) → 显示"默认订阅"标签 + 三个按钮
  - 高级用户 (is_advanced=true) → 显示"高级订阅"标签 + 三个按钮
  - 管理员 → 显示"默认订阅"和"高级订阅"两组按钮（均用于预览）
  - 三个按钮：一键导入 (primary)、复制链接 (default)、刷新链接 (warning, text, small)
- **下载客户端按钮**：每个平台卡片底部显示一个"下载客户端"链接按钮（仅当管理员配置了该平台的 download_url 时显示），点击跳转到管理员设定的外部下载地址

"一键导入"按钮行为：直接触发 window.location.href 跳转到拼接的 scheme URL。"复制链接"按钮：弹出对话框，显示完整 URL，点击输入框自动复制到剪贴板。"刷新链接"按钮：显示 loading 状态，调用 API 刷新 Token，成功后刷新平台列表。

### 4.3 管理面板 (Manage.vue)

左侧固定宽度侧边栏（200px），使用 Element Plus el-menu 组件，router 模式。菜单项：订阅管理、分享订阅、平台管理、用户管理、规则管理、OIDC配置、日志查看。当前路由对应的菜单项高亮（渐变紫色背景）。

移动端：侧边栏默认隐藏，通过顶部栏的汉堡按钮切换显示。

### 4.4 表单交互模式

所有创建/编辑使用 Element Plus el-dialog 弹窗，内嵌 el-form。提交前前端校验必填字段。删除操作统一使用 ConfirmDialog.vue 组件（需传入标题、提示文字、确认回调），不使用 ElMessageBox.confirm。

版本管理页面使用 el-upload 组件上传文件，限制文件大小 10MB。上传后自动刷新版本列表。当前激活版本在列表中用绿色标签或边框高亮标识。

### 4.5 用户管理页面 (UserManage.vue)

用户列表每行显示：用户名、邮箱、角色标签、is_advanced 标签（普通/高级）、操作按钮。

操作按钮组：
- 编辑（设置 is_advanced；groups 字段仅存储不编辑，若用户未设置 groups 则在 UI 中不显示该字段）
- 上传自定义订阅（弹出对话框 → 选择适用平台 → 选择文件 → 上传 → 覆盖该平台已有自定义订阅）
- 删除自定义订阅（仅当用户有自定义订阅时显示，恢复默认/高级自动分配）
- 吊销 Token
- 删除用户

### 4.6 分享订阅管理页面

独立列表，每行显示：名称、创建时间、当前版本号、Token 状态。

操作按钮组：
- 版本管理（进入版本管理页面）
- 复制分享链接
- 刷新 Token（确认对话框："刷新后旧链接立即失效，确定？"）
- 删除（ConfirmDialog，级联删除文件+Token）

创建分享订阅对话框：填写名称 → 上传第一个版本的文件 → 自动生成 Token。

### 4.7 暗色模式

通过 useTheme.js composable 实现。切换时同步更新：document.documentElement.classList、localStorage 偏好、Element Plus 全局主题。所有页面和组件自动跟随。

---

## 五、编码约束

以下条目是强制性的，修改代码时必须遵守。

**安全**:
- 所有含用户输入的路径操作必须经过 sanitizePath()（防路径穿越）
- 所有 /api/v1/admin/* 路由必须有 AdminRequired 中间件
- 不可硬编码密钥，使用 os.Getenv 或自动生成后存 SQLite
- Logger 必须将 ?token= 查询参数值脱敏为 ***
- OIDC ClientSecret 必须 AES-256-GCM 加密存储

**OIDC/认证**:
- 管理员 is_advanced 始终为 true（后端 UpdateUser 强制设置）
- 首个通过 OIDC 登录的用户自动成为管理员
- OIDC state 必须持久化到 SQLite（不能用内存 map），10 分钟 TTL，后端定时清理过期记录
- JWT 认证仅通过 Authorization: Bearer header
- 用户下载 Token 认证仅通过 ?token= query param（/subscriptions/:platform/download-token）
- 分享订阅 Token 认证仅通过 ?token= query param（/share/:id/download）
- AuthRequired 中间件必须实时查库，不许缓存用户权限
- GetCurrentUser 必须读数据库，不许直接用 JWT claims

**后端 Handler**:
- 创建：BindJSON → 校验必填字段 → 校验 ID 格式 [a-z0-9-]+ → 重复检查冲突返回 409
- 更新：至少校验名称非空
- 错误码：400=校验错误，409=重复，500=服务器内部错误
- 列表响应用 gin.H{"key": value} 包裹
- 成功操作用 gin.H{"success": true}
- 下载端点必须调用 logAccess() 记录访问日志

**版本管理**:
- 版本号用 nextVersion() 取最大编号+1（不可用 len(versions)+1）
- 最多保留 MAX_VERSIONS 个版本（默认 5），超出删最旧的
- 不可删除最后一个版本
- 编辑当前版本文本内容后保存 → 自动创建新版本并切换

**订阅逻辑**:
- 用户订阅类型由 is_advanced 自动决定，前端不可让用户选择类型
- 自定义订阅绑定平台、优先级高于默认/高级自动分配
- 管理员删除用户自定义订阅后，用户自动恢复到 is_advanced 对应的默认/高级订阅
- 分享订阅与用户无关，下载端点仅检查Token是否有效

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

## 六、参考：技术设计参考**不作为必须遵守的架构规范，仅作为设计概念的参考案例**

以下内容来自之前项目的设计经验，仅作为规划和构建时的参考，**不作为必须遵守的架构规范**。所有字段设计、数据库表结构、API 端点、UI 界面描述均为参考性质，最终实现以实际编码时的决策为准。

### 6.1 技术选型

后端 Go + Gin + zerolog。前端 Vue 3 (Composition API + script setup) + Vite + Element Plus + Pinia + Vue Router。存储 SQLite (modernc.org/sqlite 纯 Go 驱动，零 CGO)。OIDC 认证库 coreos/go-oidc/v3 + golang-jwt/jwt/v5。

### 6.2 当前目录结构

```
backend/
├── cmd/server/main.go             入口，Setup/Normal 双模式
├── internal/
│   ├── auth/oidc_service.go       OIDC 认证 + PKCE + JWT
│   ├── handler/                   6 个 handler（Auth, User, Subscription, Platform, Rules, Log）
│   ├── service/                   5 个 service（User, DownloadToken, Subscription, Platform, Rules）
│   ├── repository/                9 个 repository（db, user, subscription, platform, rules, download_token, system_config, log, oidc_state）
│   ├── middleware/                Logger, Recovery, CORS, CacheControl, NoCacheForDownloads, AuthRequired, AdminRequired, RateLimit
│   ├── models/types.go           所有结构体
│   ├── router/router.go          Setup 和 SetupModeRouter
│   └── utils/                    env, crypto (AES-256-GCM)
frontend/
└── src/
    ├── router/index.js           beforeEach 守卫（Setup 检测 + 登录恢复 + Admin 校验）
    ├── services/api.js           Axios 封装 + 7 组 API + 401 拦截
    ├── stores/user.js            Pinia 用户状态
    ├── composables/useTheme.js   暗色模式
    ├── components/               ConfirmDialog, OIDCSwitchDialog, UploadModal
    └── views/                    13 个页面
```

### 6.3 当前数据库表

SQLite 表结构如下：system_config（key-value 存储 OIDC 配置、JWT_SECRET、速率限制参数 — **JWT_SECRET 在 Setup 完成时自动随机生成，若 system_config 表被清空则所有用户 Token 立即失效**），users（user_id PK，role 为 admin/user，is_advanced 布尔，groups JSON），platforms（id PK，默认 3 个平台，**含 download_url TEXT 字段**），subscriptions（id PK，UNIQUE(platform, type)，type 值为 default/advanced，versions 字段存储 JSON），rules（versions 字段存储 JSON，**含 download_token TEXT 字段用于公开下载**），access_logs（id AUTOINC，按日期查询，**后端自动清理超过 90 天的记录**），oidc_state（state PK，存储 PKCE code_verifier + nonce，10min TTL），download_tokens（token UNIQUE，user_id FK，绑定 platform+订阅类型，**含 custom_sub_id 字段可空，用于自定义订阅的 token**）。

**需新增的表**:
- custom_subscriptions 表（id PK, user_id FK, platform TEXT, file_path TEXT, versions JSON, created_at, updated_at）
- share_subscriptions 表（id PK, name, description, file_path, versions JSON, created_at, updated_at）
- share_tokens 表（token UNIQUE, share_subscription_id FK, created_at）
- system_config 表存储速率限制键：rate_limit_login（默认 10/min）、rate_limit_download（默认 20/min）

### 6.4 当前 API 端点

公开: GET /api/v1/health, GET /api/v1/system/status, GET /api/v1/platforms, GET /api/v1/rules, GET /api/v1/rules/:id/download

OIDC（速率限制，管理员可在后台配置）: GET /api/v1/auth/login, GET /api/v1/auth/callback, GET /api/v1/auth/me

用户（需 JWT）: GET /api/v1/user/platforms（含 download_token）, GET /api/v1/user/update-time, POST /api/v1/user/refresh-token

订阅下载（速率限制，管理员可在后台配置）: GET /api/v1/subscriptions/:platform/download（JWT）, GET /api/v1/subscriptions/:platform/download/preview（JWT）, GET /api/v1/subscriptions/:platform/download-token（?token=）

管理员（需 JWT+Admin）: /api/v1/admin/users/*、/api/v1/admin/subscriptions/*（含版本管理）、/api/v1/admin/platforms/*、/api/v1/admin/rules/*（含版本管理）、/api/v1/admin/oidc-config、/api/v1/admin/test-oidc、/api/v1/admin/system/configure、/api/v1/admin/system/switch-provider、/api/v1/admin/logs

**需新增的端点**:
- 分享订阅 CRUD + 版本管理: /api/v1/admin/share/*
- 分享订阅下载（公开，无需认证）: GET /api/v1/share/:id/download?token=
- 管理分享 Token: POST /api/v1/admin/share/:id/refresh-token, DELETE /api/v1/admin/share/:id/token
- 自定义订阅上传（需指定平台）: POST /api/v1/admin/users/:id/custom-subscription
- 自定义订阅上传新版本: POST /api/v1/admin/users/:id/custom-subscription/versions
- 自定义订阅删除: DELETE /api/v1/admin/users/:id/custom-subscription
- 自定义订阅版本管理: GET/PUT/DELETE /api/v1/admin/users/:id/custom-subscription/versions/:versionId
- 规则下载（公开，无需认证）: GET /api/v1/rules/:id/download?token=
- 规则 Token 轮替: POST /api/v1/admin/rules/:id/refresh-token
- 速率限制配置: GET/PUT /api/v1/admin/system/rate-limit

响应格式：列表 gin.H{"key": [...]}，单项直接返回对象，成功 gin.H{"success": true}，错误 gin.H{"error": "描述"}

### 6.5 版本文件存储

data/subscriptions/{id}/ 下存放 v1.conf、v2.conf... 和 current.conf 软链接指向当前版本。规则类似，在 data/rules/{id}/ 下。

**重构需新增**:
- data/custom/{user_id}/ 存放自定义订阅文件
- data/shares/{id}/ 存放分享订阅文件
- 版本号取已有最大编号+1。

---

## 七、参考：CI/CD 设计

项目使用 GitHub Actions 自动构建 Docker 镜像并推送到 GHCR。

**触发条件**: push 到 main 或 beta 分支，push v* 标签（如 v1.0.0），手动 workflow_dispatch。

**构建策略**: matrix build 同时构建 backend 和 frontend 两个镜像。使用 docker/metadata-action 生成标签，docker/build-push-action 构建推送。

**镜像标签**: main 分支 → {service}:main 和 {service}:latest。beta 分支 → {service}:beta。版本标签 v1.0.0 → {service}:1.0.0、{service}:1.0、{service}:1。

**Dockerfile 结构**: 多阶段构建。后端：golang 编译 → distroless 运行。前端：node 构建 → 静态文件服务。前端将 /api/ 请求代理到后端服务。

---

## 八、Docker 部署

项目使用**单一 Docker Volume**（`vpn-data`）挂载所有持久化数据：SQLite 数据库、订阅配置文件（`data/subscriptions/`）、规则文件（`data/rules/`）、自定义订阅文件（`data/custom/`）、分享订阅文件（`data/shares/`）。不拆分多个 volume，所有存储内容统一挂载到 `/app/data`。

---

