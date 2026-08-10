# Issue1.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考）。
> 设计记录见 [Design1.md](./Design1.md)；构建方案见 [Build1.md](./Build1.md)、[Build2.md](./Build2.md)（已归档）、[Build3.md](./Build3.md)；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 一、进行中问题

### R10-05 Favicon 未加载：dev server 不代理 /public + 默认 favicon.svg 缺失

- **现象：** 首页 favicon 区域存在但图片未加载（自定义 ICON 与默认回退均如此）；浏览器标签页无站点图标。
- **根因（双）：** ① **dev server**：`vite.config.js`（编译产物，vite 加载优先级高于 `.ts`）的 proxy 未配置 `/public`——该路径被 SPA fallback 吞掉返回 `index.html`（`text/html`），图片加载必然失败；生产 8080 的 `/public` 静态服务正常（GET 200 `image/png`，此前 curl -I 的 404 为 gin 对 HEAD 不匹配 GET 路由的正常行为，浏览器用 GET 不受影响）；② **默认 favicon 缺失**：`frontend/public/` 目录不存在，`index.html` 引用的 `/favicon.svg` 在构建产物中 404（此前验证 favicon 回退成功仅因 dev 下 store 状态更新，回退目标本身 404）。
- **影响范围：** 浏览器 favicon 不可用（顶栏 ICON 与登录页 ICON 走 img 标签不受影响）；dev/prod 双环境。
- **修复方案（已实施）：** ① `vite.config.ts` 与 `vite.config.js` 同步补 proxy `/public → 127.0.0.1:8080`（js 为实际生效配置，ts 保持一致防回归）；② 新建 `frontend/public/favicon.svg` 默认图标（主色圆角底 + 白色 V 形），Vite 构建自动拷入 dist。
- **状态：** ✅ 已修复（2026-08-10；验收：dev `curl /public/site/icon.png?v=1` 200 `image/png`（原 text/html）；浏览器实测 favicon 加载成功 192×192；生产 GET 200；`docker compose build` 后 dist 含 favicon.svg、8080 访问 200；`npm run build` + `vitest` 20/20）

### R10-03 OIDC 启用规则白名单预填空格：零值回显被 AntD Select 渲染为空 tag（UI 体验）

- **现象：** 面板配置「OIDC 启用规则」的 Role/Group 声明路径与白名单共 4 个 Select 各预填一个空格（空 tag）；未配置过 OIDC 启用规则时必现。
- **根因（三层）：** ① 后端 `GetOidcRules` 对未配置键返回零值（`claim_path=""`、`role_values/group_values` 为 nil → JSON null）；② 前端 `loadOidcRules` 用 `Object.assign` 将零值覆盖预设默认值（`realm_access.roles`/`groups`/`[]`）；③ AntD Select 把 `value=""` 当作有效选中值渲染空 tag（视觉空格）——与 R02-01「nil slice 序列化 null」同类模式。
- **影响范围：** 仅视觉（未配置态显示空格 tag + 声明路径默认值丢失）；白名单为空时 `Empty()` 判定仍正确，无功能损害。
- **修复方案（已实施，方案 A 前端归一化）：** `loadOidcRules` 改为逐字段归一化——`role_claim_path || 'realm_access.roles'`、`group_claim_path || 'groups'`、`role_values ?? []`、`group_values ?? []`；后端零改动。
- **状态：** ✅ 已修复（2026-08-10；验收：`npm run build` + `vitest` 20/20；浏览器实测 4 个 Select 空 tag 全部消失、声明路径恢复默认值、白名单显示 placeholder）

### R10-01 用户组编辑弹窗组名空白且保存必败：getGroup 未解包嵌套响应（UI 不可用）

- **现象：** 用户组管理点「编辑」→ 弹窗标题显示「编辑组：」（空白）+ 组名输入框空白（不回显组名）；输入框可输入文字，但点保存必然失败（400「参数错误」）。
- **根因：** 后端 `GET /api/admin/groups/:id`（`server/group.go` get）返回嵌套结构 `data: {group:{...}, selections:[...]}`，前端 `api/group.ts` `getGroup` 类型标注为扁平 `GroupDetail` 并直接取 `body.data` → `detail.name`/`detail.id` 均为 undefined → 组名回显缺失；保存时 `updateGroup(undefined, ...)` 请求 `PUT /api/admin/groups/undefined` → parseID 失败 400。
- **影响范围：** 用户组编辑弹窗全部不可用（改名/关联/选定均无法保存；新建与删除正常）。
- **修复方案（已实施，方案 A 前端最小改动）：** `getGroup` 改为显式声明响应类型 `{group, selections}` 并解包 `({...d.group, selections: d.selections})` 后返回扁平 `GroupDetail`；后端零改动。
- **状态：** ✅ 已修复（2026-08-10；验收：`npm run build` + `vitest` 20/20；browser-use 实测弹窗标题「编辑组：默认组」、组名输入框回显「默认组」、`PUT /api/admin/groups/2` 200 保存链路打通；问题 2「关联订阅改选提示」经解释确认设计正确，文案优化待用户决策）

### R08-01 面板「一键清空所有数据」UI 提交空确认词导致 400（UI 不可用）

- **现象：** 面板配置「危险操作区」点击一键清空 → 输入确认词 RESET（确认按钮解锁）→ 确定 → 请求 `POST /api/admin/settings/clear_all` 返回 400「确认词不正确」，清空无法执行；API 层直接调用（带 confirm_word=RESET）正常。
- **复现步骤：** 管理面板 → 面板配置 → 危险操作区 → 一键清空所有数据 → 输入 RESET → 确定（Network 面板 clear_all 400）。
- **根因：** `SettingsView.vue` `doClearAll()` 提交 `confirmWordInput.value`，但 `confirmWordInput`（L461 声明）**从未绑定任何输入框**——`ConfirmModal.vue` 内部确认词输入（`word` ref）仅用于控制 ok 按钮禁用（`okDisabled`），`emit('confirm')` 不带确认词，父组件无接收参数 → 提交 `confirm_word: ""` → 400。
- **影响范围：** 面板一键清空 UI 操作不可用（API 可用）；R07-08 中「清空成功后清理前端凭据」链路因清空从未成功而未生效。
- **修复方案（已实施）：** `doClearAll` 直接 `clearAll('RESET')`（硬编码，确认词由 ConfirmModal 按钮禁用保证输入正确，后端二次校验兜底），删除 `confirmWordInput` 依赖；与 SetupView 硬编码 `IMPORT` 的模式一致。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；`doClearAll` 提交恒为 `RESET`，前端无空确认词提交路径）

### R08-02 面板「配置导入」UI 无法提交：密码输入框缺失 + 确认词不回传（UI 不可用）

- **现象：** 面板配置「配置导入/导出 → 导入」上传文件 → 点击导入 → IMPORT 确认弹窗输入确认词 → 确定 → 提示「请选择文件并输入导出密码」，请求无法发出（Production 模式必现）。
- **根因（双缺陷，2026-08-09 核查修正）：** ① `SettingsView.vue` 导入区模板**无导出密码输入框**——`importForm.password`（L363 声明）从未绑定任何输入框恒为空，`doImport` 前置校验（L412）直接拦截，**请求从未发出**（原记载「提交空确认词致 400」不准确——确认词缺陷被密码框缺陷前置掩盖；对照 SetupView L200 有密码输入框）；② `importForm.confirmWord`（L363 声明、L421 使用）同样未绑定输入框，即使补密码框也会因确认词为空被后端 400「导出密码与确认词必填」。Setup 页导入（`SetupView.doSetupImport` 硬编码 `'IMPORT'`）不受影响。
- **影响范围：** 面板导入 UI 不可用（Setup 导入正常；API 正常）。R07-06 验收「面板导入 403」因 Dev 模式先被模式检查拦截，未暴露此缺陷。
- **修复方案（已实施）：** ① 导入区补 `<Input.Password v-model:value="importForm.password" placeholder="导出密码（≥8 字符）">`（参照 SetupView 模式）；② `doImport` 提交 `fd.append('confirm_word', 'IMPORT')`（确认词由 ConfirmModal 按钮禁用保证，后端二次校验兜底），删除 `importForm.confirmWord` 依赖；与 SetupView 硬编码模式统一。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；导入区密码框已绑定 `importForm.password`，提交确认词恒为 `IMPORT`）

### R08-03 版本列表接口 file_name 恒空（低）

- **现象：** 四类资源版本列表 `GET /api/admin/*/:id/versions` 返回的 `file_name` 恒为 `""`，即使上传文件（如 `my-sub.yaml`）或文本模式（应补默认名）。
- **根因：** `version.go ListVersions` SQL 仅 SELECT `version_no, file_path, created_at, updated_at`，未包含 `file_name` 列（`readCurrentWithName` 下载链路已正确读取，下载文件名不受影响）。
- **影响范围：** API 契约字段缺失（前端版本列表不展示文件名，当前无实际影响）；与 R03-03「记录原始文件名」的设计意图（列表可展示）不一致。
- **修复方案（已实施）：** `ListVersions` SELECT 补 `file_name` 列并 Scan 到 `v.FileName`。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿含新增 `TestListVersionsFileName`——文件模式返回原始名 `my-sub.yaml`、文本模式补默认名 `subscription.yaml`、当前激活标记正确）

### R08-04 unassigned 状态仍生成下载 Token（设计文字冲突，低）

- **现象：** 普通用户首页 `GET /api/home/platforms` 对 `status=unassigned`（组未选定）的平台仍返回非空 `download_token` 与 `download_url`；Design1 §2.2 要求「不生成 Token，不兜底不回退，三按钮隐藏（无 Token 可操作）」。
- **根因：** `server/home.go` L142 注释「仍返无标识 Token（下载时返回 unassigned 注释块）」为**有意实现**：Token 生成与分发解析解耦，下载时实时解析返回 `# error: unassigned`（HTTP 200 注释块），无越权。
- **影响范围：** 仅多生成一条 Token 记录（DB 冗余）；前端已按 status 正确隐藏三按钮，行为安全。与设计文字描述存在出入。
- **修复方案（已决策）：** 保持现状，Design1 §2.2 表述已同步修订——unassigned 仍生成无标识 Token，下载时实时解析返回 `# error: unassigned` 注释块（与 §4.2/§4.3 机制一致）；若改不生成需拆分 Token 生成与卡片渲染逻辑（未采纳）。
- **状态：** ✅ 已闭环（2026-08-09；用户决策：保持现状 + 修订 Design1 §2.2，无代码变更）

### R08-05 同机导出→导入（密钥相同）旧会话不失效（边界行为，低）

- **现象：** 同一实例导出配置后立即导入（签名密钥相同），旧会话凭据仍有效（me 200）；跨实例导入（密钥不同）旧会话必然 401。Design1 §3.4.8 字面「导入后签名密钥替换 → 全部会话立即失效」。
- **根因：** 导入=整体覆盖写入导出文件内容（含签名密钥）；同机往返密钥无变化，验签自然通过。
- **影响范围：** 同机往返场景会话不失效无安全风险（导入操作本身由管理员执行）；跨实例迁移场景（设计主场景）会话失效机制验证通过（TestD2 跨实例导入 401）。
- **修复方案（已决策）：** 保持现状，Design1 §3.4.8 表述已同步修订——跨实例迁移（密钥必然变化）会话失效机制不变；同机往返密钥未变化时旧会话保持有效（强制轮换密钥需同时重加密全部敏感密文，破坏「导出=精确还原」语义，未采纳）。
- **状态：** ✅ 已闭环（2026-08-09；用户决策：保持现状 + 修订 Design1 §3.4.8，无代码变更）

### R09-01 主界面用户名区域为裸文本非按钮（UI 体验）

- **现象：** 主页顶栏用户名是裸文本 span，与「管理面板」按钮风格不一致，无按钮质感（无边框/无 hover 反馈）。
- **根因：** `HomeView.vue` header 用户名用 `<span class="cursor-pointer text-sm">`，未使用 Button 组件；「管理面板」为 `ant-btn-primary`。
- **影响范围：** 视觉一致性（功能正常）。
- **修复方案：** 提取通用顶栏组件 `AppHeader.vue`，用户名改为 Button 组件（与管理面板按钮同款组件形式）。
- **状态：** ✅ 已修复（2026-08-09；验收：浏览器实测顶栏 `Test` 为 ant-btn；`npm run build` + `vitest` 20/20）

### R09-02 用户组名折叠在 dropdown 而非顶栏（UI 体验）

- **现象：** 组名 Tag 在用户名 dropdown 内，顶栏不展示；Design1-UI §4.1 要求「所属组名标签」直接展示于顶栏右侧。
- **根因：** `HomeView.vue` dropdown overlay 内 `<Tag v-if="groupName">`，header 无组名。
- **修复方案：** `AppHeader` 顶栏直接渲染组名 Tag（cyan，`<640` 隐藏防溢出）。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；组名 Tag 已在顶栏渲染逻辑，`<640` 隐藏）

### R09-03 用户名 Dropdown hover 即展开（UI 体验）

- **现象：** AntD Dropdown 默认 `trigger="hover"`，鼠标浮动即展开，误触率高；触屏端无 hover 语义，行为异常。
- **根因：** `HomeView.vue` `<Dropdown>` 未指定 trigger，取 AntD 默认 hover。
- **修复方案：** `AppHeader` Dropdown 显式 `trigger="click"`（点击后显示）。
- **状态：** ✅ 已修复（2026-08-09；验收：浏览器实测点击用户名按钮才展开 dropdown，hover 不触发）

### R09-04 暗色切换为文字按钮且不常驻顶栏（UI 体验）

- **现象：** 主页暗色切换折叠在 dropdown 内、管理面板在内容区右上角、登录页为文字按钮——均非「emoji + 开关」形式且未常驻顶栏。
- **根因：** 三处实现均为文字按钮（`{{ dark ? '切换到浅色' : '切换到暗色' }}`）。
- **修复方案：** 三处统一为 emoji + Switch（🌙 暗色 / ☀️ 浅色）：主页/管理面板常驻顶栏（AppHeader）、登录页卡片下方；Design1-UI §1.3 同步修订（原「图标按钮」表述）。
- **状态：** ✅ 已修复（2026-08-09；验收：浏览器实测主页/管理面板/登录页三处均为 🌙/☀️ Switch 且状态同源）

### R09-05 管理面板无主界面式顶栏（UI 体验）

- **现象：** 管理面板（桌面 + 移动）无站点名/用户名/暗色切换顶栏，内容区右上角仅「切换到浅色」+「退出」；Design1-UI §1.1「浅色侧边栏 + 浅色顶栏，用户端与管理端风格统一」未落地。
- **根因：** `AdminLayout.vue` 桌面端无 header，移动端仅简易 AntD Header（汉堡 + 暗色文字按钮）。
- **修复方案：** AdminLayout 接入通用 AppHeader（移动端带 ☰ 汉堡 prop 唤出 Drawer），内容区右上角暗色/退出按钮移除（迁入顶栏用户名 Dropdown）。
- **状态：** ✅ 已修复（2026-08-09；验收：浏览器实测桌面 1044px/移动 575px 双视口均显示站点名 + 用户名按钮 + 暗色开关）

### R09-06 个人中心缺返回主界面按钮（UI 体验）

- **现象：** `/profile` 无返回入口，只能浏览器后退或手改 URL。
- **根因：** `ProfileView.vue` 无 header、无返回按钮。
- **修复方案：** 标题行加「← 返回主界面」按钮（ArrowLeft 图标 + `router.push('/')`）。
- **状态：** ✅ 已修复（2026-08-09；验收：浏览器实测按钮存在且点击跳转首页成功）

### R09-07 移动端浅色 header 背景被 AntD 默认样式覆盖（UI 体验）

- **现象：** `<768` 管理面板顶栏浅色模式实际为深蓝黑底（`#001529`），☰ 汉堡按钮黑色文字在深底上不可见（与底色重叠）。
- **根因：** `.ant-layout-header { background: #001529 }`（AntD）与 `.bg-white`（Tailwind）同为单类选择器，构建后 AntD CSS 后加载胜出，Tailwind 底色类失效；按钮文字为浅色主题黑色。
- **修复方案：** 废弃 AntD `Layout.Header`，改为自定义 `<header>` 元素（Tailwind 类正常生效，与主页顶栏一致）。
- **状态：** ✅ 已修复（2026-08-09；验收：浏览器计算样式实证——移动端浅色 header 背景 `rgb(255,255,255)`、☰ 按钮黑色文字清晰可见）

### R09-08 深色模式顶栏文字与背景重叠（UI 体验）

- **现象：** 深色模式顶栏深灰底（gray-800）+ 站点名/用户名纯黑文字（继承 body 色）不可见；影响所有视口（手机端更明显）。
- **根因：** 顶栏文字无 `dark:` 文字颜色类；Tailwind preflight body 文字色为黑色 `rgb(0,0,0)`，深色模式下背景变深灰但文字不变。
- **修复方案：** 站点名补 `dark:text-gray-100`、更新时间补 `dark:text-gray-400`；用户名按钮由 AntD dark 主题自动适配白字。
- **状态：** ✅ 已修复（2026-08-09；验收：浏览器计算样式实证——深色模式站点名 `rgb(243,244,246)`、用户名白字 `rgba(255,255,255,0.85)`）

### R09-09 面板配置页滚动时双侧栏滚出屏幕（UI 体验）

- **现象：** 面板配置界面（/admin/settings）页面滚动时双侧栏均发生滚动：左侧主边栏（订阅/用户组/分享/平台…菜单）与右侧辅边栏（OIDC/SMTP/站点信息/危险操作区…锚点目录）滚出屏幕；期望主边栏固定不滚、辅边栏自带滚动条。
- **根因：** ① 主边栏 `Layout.Sider` 在文档流中（AntD 默认 `position: relative`），无吸顶定位，随页面滚动滚出屏幕；② 辅边栏 Anchor 锚点目录容器无 sticky、无限高、无 overflow 设置，随页面滚动滚出屏幕且无独立滚动条。
- **影响范围：** 管理面板配置页长内容滚动时导航失效（功能正常，体验缺陷）；其他管理页面仅主边栏受影响（无辅边栏）。
- **修复方案：** ① 主边栏 Sider 加内联样式 `position: sticky; top: 0; height: 100vh; overflow-y: auto`——用内联样式而非 Tailwind sticky 类，规避与 AntD `position: relative` 的同特异性覆盖冲突（R09-07 同类坑）；② 辅边栏 Anchor 外层包 `sticky top-20 max-h-[calc(100vh-6rem)] overflow-y-auto`——吸顶于顶栏下方、限高、内容超限时出现独立滚动条。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；浏览器实测滚动 2953px 后主边栏 top 保持 ≈0、辅边栏 top=80px 均吸顶；注入超限内容后 clientH 受限 834px、可独立滚动）

### R09-10 面板配置锚点跳转目标被 sticky 顶栏遮挡（UI 体验）

- **现象：** 点击辅边栏（锚点目录）任意链接，右侧滚动目标卡片顶部被 sticky AppHeader（64px）遮挡；URL 直链 `#hash` 访问同样遮挡。
- **根因：** AntD Anchor 点击与浏览器原生 hash 跳转均将目标滚动到视口顶部，未预留 sticky 顶栏高度。
- **影响范围：** 面板配置页锚点导航定位不准（功能正常，体验缺陷）。
- **修复方案（双保险）：** ① `Anchor` 补 `:offset-top="80"`（AntD 点击路径：目标位置 - 80px = 顶栏 64px + 间距 16px）；② 内容区容器加 `settings-scroll` 类 + scoped style 给全部分区卡片（`:deep([id])`）设 `scroll-margin-top: 80px`（原生 `#hash` 直链兜底，AntD offset-top 不介入原生跳转）。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 无回归；浏览器实测点击「SMTP」锚点与 URL 直链 `#smtp` 双路径目标 Card top 均 = 80.14px、与顶栏间距 16px 不被遮挡）

### R09-11 主界面池内订阅展示与对齐调整（UI 体验，用户确认最终方案）

- **现象：** ① 管理员主界面池内订阅行显示订阅标识 slug（如 `test-sub-1`）与名称并排——视觉错位且暴露系统内部 ID；② 逐行分割线（border-b）与「一键导入/复制链接」按钮存在重叠；③ 用户名 Dropdown 固定宽 192px 偏大；④ 圆角块行内文本与按钮中心偏移 4px 未对齐。
- **根因：** ① slug 与名称字体/基线差异（code 样式基线偏差 2px）产生错位感；② 行内 flex 容器被压缩为 20px，按钮（24px）底边向下溢出 2px、越过分割线 1px；③ dropdown 固定 `w-48`；④ **AntD `Space` 组件（inline-flex + css-in-js 注入）在 flex 行内存在 4px 垂直偏移异常**——实验验证：父容器 align-items:center 与强制 align-self:center 均无法消除。
- **修复方案（用户确认最终方案）：** ① 移除 slug，仅展示订阅名称（truncate 防溢出）；② 逐行 border-b 改为**圆角浅色块行**（`rounded-md` + `bg-gray-100`/暗色 `bg-gray-700/50` + 块间 `space-y-2`；浅色底色经用户微调加深一档提升可读性）；③ dropdown `w-48` → `w-auto min-w-32`（自适应）；④ 按钮容器 `Space` → **普通 flex div**（消除 4px 偏移，实测文本与按钮中心 diff=0）。
- **状态：** ✅ 已修复（2026-08-09；用户确认效果满意，定为最终方案；验收：`npm run build` + `vitest` 20/20、浏览器实测两行文本/按钮中心完全对齐 diff=0、浅色/暗色块样式正常）

### R09-12 分享/用户组/规则管理/日志缺移动端卡片（UI 体验）

- **现象：** 管理面板分享订阅、用户组、规则管理、日志（访问日志）四页在移动端（<768）仍为纯表格渲染——8 列表格（日志）在窄屏严重溢出难用；Design1-UI §1.1「<768 统一卡片化」及 §5.2/5.3/5.7/5.9「双态列表」规格此前仅平台/订阅两页落地。
- **根因：** 四处实现仅渲染 `a-table`，无 <768 卡片分支。
- **修复方案：** 参照 PlatformsView 双态模式补齐四处：`a-table` 加 `hidden md:block`（≥768 表格）+ 新增 `md:hidden` 卡片网格（`border rounded-lg p-3 bg-white dark:bg-gray-800`，与平台/订阅卡片风格一致）；日志 8 列精简为卡片（类型+状态 Badge 头部，用户/平台/IP/资源标识/失败原因/时间信息区）；需重选组卡片橙色描边（`border-orange-300`）。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20、浏览器 575px 移动端实测四页卡片正常渲染且表格隐藏（高度 0）、卡片字段完整；Design1-UI.md 变更记录 v1.5）

### R09-13 用户名 dropdown 暗色模式可读性差（UI 体验）

- **现象：** 暗色模式下用户名 dropdown 背景（gray-800）与顶栏同色，仅基础阴影（暗色下不明显）、无边框，边界模糊可读性差。
- **根因：** `AppHeader.vue` overlay 仅有 `shadow`（阴影弱、暗色模式几乎不可见），无边框——暗色下与同色顶栏难以区分。
- **修复方案：** overlay 样式增强——`shadow` → `shadow-lg`（多层增强阴影）+ 新增 `border border-gray-200 dark:border-gray-600`（浅色浅灰边 / 暗色深灰边，暗色下与背景形成清晰边界）。
- **状态：** ✅ 已修复（2026-08-09；用户确认满意；验收：生产构建 `docker compose build` + `up -d` 后实测——暗色模式 1px solid gray-600 边框 + shadow-lg 多层阴影、浅色 gray-200 边框均生效，dropdown 正常展开；`vitest` 20/20。备注：dev server 环境存在 popup 渲染异常（面板高度 0/overlay 空，无控制台报错，生产构建正常），验证改走生产构建路径）

### R07-02 公告功能缺失：首页无公告展示与公开端点

- **现象：** 面板配置可保存公告内容，但用户首页无公告栏卡片，后端也无公告公开端点（Design1 §5.2 要求「公告数据接口公开，未登录可获取」）。
- **根因：** Build3 Step 3 只实现了 `/api/admin/settings/announcement` 管理端读写；`HomeView.vue` 无公告渲染，后端未注册公开公告端点——Design1 §3.3「公告栏卡片（有内容才显示，仅登录后首页展示）」/ §3.4.8「首页展示」未落地。
- **影响范围：** 公告功能整体不可用（配置内容无处展示），设计验收失败项。
- **修复方案：** ① 后端 `server/status.go` 新增公开端点 `GET /api/public/announcement`（复用 StatusHandler 的 cfg 依赖，返回 `{content: cfg.Get("announcement")}`，无敏感信息），在 `server.go New()` 的 registerStatus 旁注册；**不注册到 NewEmergency**（应急页不依赖，保持最小面，符合 §3.8）；② 前端 `api/settings.ts` 新增 `getPublicAnnouncement()`；③ `HomeView.vue` main 区平台网格前加公告卡片，`v-if="announcement"`（有内容才显示），纯文本插值 `{{ announcement }}` 天然转义禁 HTML（§3.4.8），与 `homePlatforms` 并行获取。验证：未配置不渲染/配置后首页展示/含 `<script>` 内容按文本显示。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿含新增 `TestPublicAnnouncement`（未配置空串/配置后返回）；`npm run build` + `vitest` 20/20（home-view.spec 补 `getPublicAnnouncement` mock）；Dev 实例实测：未配置返回 `{"content":""}`，面板保存公告后公开端点返回内容，含 `<script>` 原样返回（前端插值转义禁 HTML））

### R07-03 访问日志日期筛选时区错位

- **现象：** 面板日志页选择「今天」（本地日期）查询不到当天 00:00~08:00（东八区）产生的日志；实测本地 05:13 产生的下载日志用 `from=2026-08-09&to=2026-08-09` 查询 total=0，改用 `2026-08-08` 命中 16 条。
- **根因：** `log/access.go parseRange` 以 `time.UTC` 解析 `YYYY-MM-DD` 边界，与 `access_logs.created_at`（UTC 存储）比较；前端 `LogsView.vue` 直接传本地日期字符串（dayjs `format('YYYY-MM-DD')`），两端时区口径不一致。
- **影响范围：** 非 UTC 时区部署的访问日志日期筛选结果不准确（列表展示不受影响——created_at 前端本地化正常）。
- **修复方案：** **仅改前端** `LogsView.vue` 传参（后端 `parseRange` 保持 UTC 解析不变——容器时区通常为 UTC，后端依赖本地时区不可靠）：将本地日期转换为该时刻对应的 UTC 日期再传，纯原生实现（不引入 dayjs utc 插件）：
  ```ts
  const toUtcDate = (d: dayjs.Dayjs) => {
    const t = d.toDate()
    return new Date(t.getTime() - t.getTimezoneOffset() * 60000).toISOString().slice(0, 10)
  }
  q.from = toUtcDate(range.value[0]); q.to = toUtcDate(range.value[1])
  ```
  原理：本地 08-09 00:00（+08:00）→ UTC 日期 "2026-08-08"；本地 08-09 23:59 → "2026-08-09"，后端 UTC 解析后恰好覆盖本地全天。验证：本地 00:00~08:00 产生的日志用「今天」可筛出；跨月/跨年边界（本地 1/1 00:30 → UTC 上年 12/31）正确。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；仅改前端 `LogsView.vue` 传参为 UTC 日期，后端 `parseRange` 保持 UTC 解析不变）

### R07-04 首页订阅更新时间戳错位

- **现象：** 首页顶栏显示「订阅更新于 2026-08-08 21:21」，实际本地时间为 08-09 05:21（差 8 小时）。
- **根因：** `/api/home/updated_at` 直接返回 SQLite `MAX(updated_at)` 的无时区字符串（`YYYY-MM-DD HH:MM:SS`），前端 `dayjs(updatedAt)` 按本地时区解析；版本列表等其他接口返回 RFC3339（带 `Z`）无此问题——**接口格式不一致**。
- **影响范围：** 首页更新时间戳展示错误（分发功能不受影响）。
- **修复方案：** 同类无时区问题共 3 处（access_logs/approval/version 已用 `time.Time` scan 按 UTC 解析，正确），统一改 `time.Time` 返回 RFC3339：① `server/home.go updatedAt`：`ts.String` → `time.Parse("2006-01-02 15:04:05", ts.String)` 后返回 `time.Time`（空值保持 nil）；② `share/share.go` `CreatedAt` 字段 `string` → `time.Time`，Scan 目标直接用 `time.Time`（与 access.go 已验证的 scan 模式一致）；③ `rule/rule.go` `RefreshedAt` 字段改 `time.Time`，**注意空串无法 scan 到 time.Time**——`COALESCE(...,'')` 改 LEFT JOIN 原生 NULL，用 `sql.NullTime` 接收，空值输出 null/前端 `'—'`。前端配套：`SharesView.vue:191`、`RulesView.vue:171` 原样展示改 `fmtTime()`（dayjs 本地化，对齐 VersionManageView 已有模式）；HomeView 的 updatedText 已用 dayjs 自动修正。验证：本地时区四处时间差 8 小时问题消除；分享/规则空时间显示 `—`。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿；Dev 实例实测 `/api/home/updated_at`、分享 `created_at`、规则 `refreshed_at` 均返回 RFC3339 带 `Z`（如 `2026-08-09T06:00:10Z`）；`npm run build` + `vitest` 20/20；SharesView/RulesView 空时间显示 `—`）

### R07-05 无版本订阅管理员预览返回 500

- **现象：** 管理员预览无版本的订阅（刚创建未上传版本）返回 HTTP 500「服务器内部错误」。
- **复现步骤：** 创建订阅（无版本）→ 管理员 `GET /api/subscriptions/preview?platform=platform-xxx&subscription_id=N` → 500。
- **根因：** `download.PreviewForUser` 管理员分支对无版本资源返回 `version.ErrVersionNotFound`，`server/download.go preview` 未映射该错误（非 ErrTokenInvalid/ErrUnassigned → 落入 500 分支）。
- **期望行为：** 与版本管理页一致映射 404（或返回空内容）。
- **修复方案：** `server/download.go` 四个 handler（preview/userDownload/shareDownload/ruleDownload）统一补 `errors.Is(err, version.ErrVersionNotFound) → 404`（与无效 Token 同 404，不泄露差异）：`WriteAccessLog(entry, false)` 记 fail_reason=`version_missing` + 禁缓存头 + `Fail(404, "资源不存在")`。覆盖场景：管理员预览无版本订阅、显式 Token 下载无版本订阅（管理员首页预览生成显式 Token 后订阅仍无版本时）。验证：无版本订阅预览/显式 Token 下载均 404 且日志记 version_missing；有版本场景回归正常。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿含新增 `TestPreviewNoVersion`（无版本 404 + 日志 `version_missing` + 有版本回归 200）；Dev 实例实测无版本订阅预览 HTTP 404（原 500）、访问日志 `('subscription','fail','version_missing')`、有版本预览回归 200）

### R07-06 Dev 模式配置导入返回 400 非 403

- **现象：** Dev 模式调用 `POST /api/admin/settings/import` 返回 400，而 `POST /api/admin/settings/export` 返回 403——错误码契约不一致。
- **根因：** `settings_ops.go importCommon` 对 `ExportService` 的「仅 Production 模式提供」错误统一映射 400（export handler 单独映射 403）。
- **影响范围：** 仅错误码语义不一致，功能正确（Dev 模式导入确实被拒绝）。
- **修复方案：** ① `config/export.go` 新增哨兵错误 `var ErrModeRestricted = errors.New("配置导入导出仅 Production 模式提供")`，导出/导入两处模式校验返回哨兵（替换裸 errors.New）；② `server/settings_ops.go` export handler 兜底分支与 `importCommon` 兜底分支均改为先判 `errors.Is(err, config.ErrModeRestricted) → 403`，其余保持原映射；③ Setup 导入（importSetup）经 importCommon 自动获得 403（Dev 模式 Setup 导入同样应拒绝，与「仅 Production 提供」边界一致）。验证：Dev 模式导出/面板导入/Setup 导入均 403；Prod 往返正常；`export_test.go` 补 import 403 断言。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿含 `TestExportDevModeDenied` 哨兵断言（Export/Import/Setup 导入三路径）；Dev 实例实测面板导入与 Setup 导入均 HTTP 403「配置导入导出仅 Production 模式提供」（原 400））

### R07-07 模拟 OIDC 表单缺 role/group 附加属性

- **现象：** Dev 模式登录页模拟登录表单只有邮箱/用户名/email_verified，无 Design1-UI §2.2 要求的 role/group 附加属性勾选输入。
- **根因：** `LoginView.vue` mock 表单未实现 role/group 输入项（`mockForm` 仅 email/username/email_verified）。
- **影响范围：** 无法通过 UI 测试 Role/Group 白名单与审批逻辑（API 层不受影响）。
- **修复方案：** **仅改前端**（后端 `MockLogin(ctx, email, username, emailVerified, roles, groups)` 与 `api/oidc.ts mockLogin` 均已支持 roles/groups）：`LoginView.vue` ① `mockForm` 加 `role: ''`、`group: ''`；② 按 UI §2.2 交互补两个 `Checkbox`「附加 role」「附加 group」，勾选后显示对应 `Input`；③ `onMockLogin` 透传 `roles: mockForm.role ? [mockForm.role] : undefined`（空则 undefined，保持「可留空」语义）。验证：勾选 role 输入值模拟登录 → 数据库 `oidc_claims` 含 role 快照（审批中心可展示）；白名单命中/未命中分支可经 UI 测试。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；勾选「附加 role/group」输入值后模拟登录，后端 `oidc_claims` 含 role/group 快照可供审批中心展示）

### R07-08 登录页瞬态「会话凭据无效或已过期」提示（根因已确认）

- **现象：** Setup 完成点击「前往登录」后，登录页短暂出现 message.error「会话凭据无效或已过期」，刷新后消失；无法稳定复现。
- **根因（2026-08-09 深查确认）：** 一键清空（`SettingsView.doClearAll`）与 Setup 导入（`SetupView.doSetupImport` 的 onOk）**密钥轮换后未清理前端 localStorage 凭据**（面板导入已 `logoutAction`，这两处遗漏）。残留失效 token 触发完整链路：登录页 onMounted `if (auth.token) router.replace('/')` 带失效 token 跳首页 → `HomeView` 调 `me()` 返回 401 → 401 拦截器清凭据并跳回 /login，HomeView catch 弹全局 message「会话凭据无效或已过期」——页面已回到登录页，表现即「登录页短暂提示」；刷新后 token 已被拦截器清除，不再出现。
- **影响范围：** 体验瑕疵（仅「一键清空/Setup 导入 → Setup 完成 → 前往登录」且浏览器残留旧凭据时出现），无功能影响。
- **修复方案：** ① `SettingsView.doClearAll` 成功后先 `auth.logoutAction()` 再跳 /setup；② `SetupView` 导入完成 onOk 先 `logoutAction()` 再跳 /login；③ `LoginView` onMounted 两个公开请求补 `.catch(() => {})` 防 unhandled rejection 噪音；④ `request.ts` 401 拦截器加 Dev 诊断日志（`console.warn('[request] 401:', url)`）。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；链路静态验证：三处密钥轮换操作后前端凭据均被清除，`LoginView` 不再携带失效 token 跳转首页）

已闭环问题见下：

### R06-01 用户管理「操作 ▾」下拉菜单不可用：Menu 未导入 + custom_subs 序列化为 null

- **现象：** `/admin/users` 每行「操作 ▾」下拉打开后无任何菜单项，控制台报 `Cannot read properties of null (reading 'map')`；Build3 Step 1 全部用户操作（换组/角色/自定义订阅/重置密码/吊销 Token/清 OIDC/禁用/删除）UI 不可达（API 层正常）。
- **根因（双）：** ① `UsersView.vue` import 列表缺 `Menu` 组件（模板 `<Menu>` 渲染为原生空元素，`main.ts` 无全量注册）；② 后端 `AdminUser.CustomSubs`（无 omitempty）对无自定义订阅用户保持 nil，序列化为 `null`，前端 `menuItems()` 中 `u.custom_subs.map` 无兑底抛错（Dropdown overlay 惰性渲染，点击时才求值）。
- **影响范围：** 全部用户（无自定义订阅用户报 TypeError；有自定义订阅用户 Menu 未注册渲染为空），用户管理全操作入口不可用。
- **修复方案：** ① 前端 import 补 `Menu`；② 后端 `List()` 构造时 `u.CustomSubs = make([]CustomSubItem, 0)` 保证 `[]` 而非 `null`（遵循「不过度防御」不叠加前端兜底）；③ `admin_test.go` 新增守护断言（custom_subs 非 nil）。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿含守护测试；browser-use 实测下拉菜单完整显示（编辑/角色/自定义订阅/吊销/重置密码/禁用/删除），点击「设置/重置密码」弹窗正常）。

### R06-02 实时日志流 SSE 不可用：/stream 注册在会话中间件组内

- **现象：** 日志页「实时日志流」持续「连接断开重连」；`GET /api/admin/logs/stream?token=` 返回 401「会话凭据缺失」；Build3 Step 5 的「8 连接上限」无法端到端验证。
- **根因：** `RegisterLogRoutes` 将 `/stream` 注册在 `sessionMW+adminMW` 路由组内，而 `SessionMiddleware` 仅从 `Authorization` header 取凭据（无 cookie 兑底），EventSource 无法携带自定义 header → 请求到不了短期 Token 校验即被拒；与 Design1 §4.8「认证方式：一次性短期 Token」设计意图矛盾。
- **影响范围：** 实时日志流功能不可用（访问日志查询正常）；8 连接上限/重连语义无法验收。
- **修复方案：** ① `/stream` 移出会话中间件组，独立注册仅靠一次性短期 Token 鉴权（`/stream/token` 保留双中间件）；② 顺带修复：历史推送循环后立即 `Flush()` 一次（否则无新事件时响应头不下发、`EventSource.onopen` 延迟触发）。
- **状态：** ✅ 已修复（2026-08-09；验收：browser-use 实测实时流连接稳定、历史+增量持续输出、无重连提示；curl 验证历史帧即时返回；路由表确认 `/stream` 仅 3 中间件）。

### R06-03 数据库损坏未自动进入应急模式：Open/Migrate 失败直接 exit

- **现象：** DB 文件完全损坏 → 「初始化 PRAGMA 失败」exit 1；中间页损坏 → 「迁移失败，拒绝启动」exit 1，均未进入应急恢复页；compose `unless-stopped` 下形成重启风暴。
- **根因：** `main` 中 `store.Open`/`Migrate` 失败直接 `os.Exit(1)`，先于 `emergency.Detect` 执行——`TriggerDBCorrupt` 自动触发分支在真实启动路径上不可达；单测仅覆盖 Detect 的 manual/key_missing，未覆盖启动路径。
- **影响范围：** 数据库损坏场景失去「重新初始化」救援入口（Design1 §3.8 要求自动触发应急页）。
- **修复方案：** ① `main` 提取 `runEmergencyMode`：Open 失败（st/cfg 不可用）与 Migrate 失败均转入应急装配（reason=db_corrupt、dbReadable=false），Open 失败传空配置 Service；② nil store 守卫：`config.Get`（store nil 按未设置返回）、`user.IsTableEmpty`（按非空降级）、`emergency.ListAdmins`（dbReadable=false 拒绝查询）；③ 新增 `TestDetectDBCorrupt` + `TestNewEmergencyNilStore`（nil store 应急装配、status 200 emergency=true、业务端点 503）。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿含两个新增用例）。

### R06-04 站点名称配置不生效：三处展示均硬编码/未消费

- **现象：** 面板设置站点名保存成功（`/api/site/info` 已返回新值），但浏览器标题、登录页、首页顶栏仍显示硬编码「VPN 订阅管理」。
- **根因：** ① `index.html` title 硬编码；② `HomeView.vue` 顶栏硬编码；③ `LoginView.vue` 无站点 ICON/名称展示（UI §2.2 要求）；④ `siteInfoPublic()` 已定义但全站无调用。违反 Design1 §3.4.8「站点名称——浏览器标题/登录页/首页展示」。
- **影响范围：** 站点自定义品牌展示失效（登录页/首页/浏览器标签）。
- **修复方案：** 共用逻辑入 `stores/system.ts`（`fetchSiteInfo`/`siteName`/`siteIconUrl`，未设置回退默认标题）；三处接入：App.vue 挂载后 `document.title`、HomeView 顶栏站点名、LoginView 顶部「ICON + 名称」；测试补 mock `@/api/settings`。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20；browser-use 实测登录页/首页/浏览器标题均显示「验收测试站」）。

### R06-05 登录/注册页未接验证码组件：启用后请求必被拒

- **现象：** 管理员在面板启用登录/注册验证码后，前端无组件取 token，提交请求被后端拒绝（400「请完成验证码校验」）；仅 Forgot 页已接通。
- **根因：** `LoginView.vue`/`RegisterView.vue` 未接入 `CaptchaWidget`、表单无 `captcha_token` 字段；后端 `captchaSvc.Middleware` 在 `Enforced`（页面启用且密钥已配置）时强制校验 token。默认关闭不触发。
- **影响范围：** 启用验证码的登录/注册场景不可用（默认配置无影响）。
- **修复方案：** 两页接入 `CaptchaWidget`（page=login/register）+ 表单 `captcha_token`；`api/auth.ts` login/register 与 `stores/auth.ts` loginAction/registerAction 入参透传。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `vitest` 20/20 全绿，默认关闭无回归）。

### R06-06 订阅/规则/分享/自定义上传缺 50MB 前端预校验

- **现象：** 版本上传（订阅/规则/分享共用）、分享/规则创建首版本、用户自定义订阅上传均无前端大小预校验（仅安装包有 300MB 预校验），违反 AGENTS §4.1「前端 + 后端双重校验」（后端 `MaxContentSize` 50MB 硬限制存在）。
- **修复方案：** 四处补 `file.size > 50<<20` 预校验并拦截提示：VersionManageView.onUpload、SharesView/RulesView.beforeUpload、UsersView.onFileChange。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` 通过）。

### R06-07 .smoke-test.sh 契约滞后：按旧裸数组解析中断

- **现象：** 脚本步骤 6 起中断（`set -e`）：`/api/home/platforms` 已统一 `data.list` 包裹，脚本仍按 `['data'][0]` 解析（python 对 dict 取索引抛错）；步骤 4/11 同样滞后。
- **根因：** R02-01 列表接口统一 ListData 包裹后脚本未同步；另步骤 9 的 JSON 文本跨行导致解析失败（既有缺陷）。
- **修复方案：** 三处改 `['data']['list'][...]` 解包；步骤 9 文本改单行 JSON。
- **状态：** ✅ 已修复（2026-08-09；验收：本地起服务跑通全流程至「SMOKE ALL DONE」，含步骤 9 版本切换下载 node2）。

### R06-08 邮箱输入过程误报「不是一个有效的 email」（结论经修正）

- **现象：** 登录/找回密码页邮箱输入过程中途即报邮箱格式错误。
- **根因：** `LoginView`/`ForgotView` 邮箱规则未指定 `trigger`，antd 4.x 默认 `validateTrigger='change'` 实时校验；`RegisterView` 显式 `trigger: 'blur'` 无此问题（核查修正范围：仅 Login/Forgot 两页；现象为持续提示至输入合法而非瞬时）。
- **修复方案：** 两处规则补 `trigger: 'blur'`（对齐 RegisterView 既有写法）。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` 通过）。

### R07-01 新用户未自动加入默认组：三条创建路径 INSERT 缺 group_id

- **现象：** 管理员创建/自注册/OIDC 登录创建的用户 `users.group_id` 均为 NULL，用户首页该平台显示「未分配，请联系管理员」；全库无默认组归属逻辑。
- **复现步骤：** 任意路径创建新用户后 `SELECT group_id FROM users` → NULL；新用户首页平台卡片状态 unassigned。
- **根因：** user 包三条创建路径（`Register` 自注册 / `CreateFromOidc` / `AdminService.Create`）的 `INSERT INTO users` 均未写入 `group_id` 列，且无 `is_default` 查询助手——违反 Design1 §2.2「新用户（OIDC / 自注册 / 管理员创建）自动加入默认组」。
- **影响范围：** 所有新建用户无法获得组分配订阅（核心分发链路断裂）；存量用户需手动换组；首页/下载均受影响。
- **修复方案：** ① `user.go` 新增 `defaultGroupIDTx` 共享助手（查 `groups WHERE is_default=1`；`err == nil` 短路避免空指针、`no such table` 降级 NULL）；② 三处 INSERT 统一补 `group_id` 列；③ 新增守护测试 `TestRegisterJoinsDefaultGroup`（建 groups 表+默认组 → 注册 → 断言 group_id 非空）。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 25 包全绿含守护测试；Dev 实例实测自注册/管理员创建/OIDC 三路径 `group_id=1`，首页分发与下载复验正常）。

### R05-01 Setup 导入前端入口缺失：配置导入双入口仅实现面板侧

- **现象：** Build3 全部 Step 验收通过后审查发现，首次配置向导（Setup 页）无「导入已有配置」入口——`SetupView.vue` 仅有注释占位（「本 Step 直接隐藏（Build3 补充）」），模板无任何导入 UI；Design1 §3.4.8 明确要求配置导入为双入口（面板 + Setup）。
- **根因：** Build3 Step 4 前端产出仅要求扩展 SettingsView.vue 三分区，Setup 页导入入口遗漏（后端 `POST /api/setup/import` 端点已实现且含限流，但无前端调用方）；Design1-UI §2.1「导入已有配置」卡片（仅 Production 渲染）未落地。
- **影响范围：** 一键清空/新部署场景无法经 GUI 从备份配置恢复（仅可手动 curl API）；Build3 Step 4 手动验收路径「导出配置 → 一键清空 → Setup 导入恢复配置」无法完整走通。
- **修复方案：** ① `api/settings.ts` 新增 `setupImportConfig`（POST `/setup/import`）；② SetupView 新增「导入已有配置」卡片（仅 Production 渲染，Dev 显示说明文案）+ `a-upload` 选文件 + 导出密码输入 + `IMPORT` 确认词 ConfirmModal（复用通用组件）+ 成功后提示「请立即重启容器后再重新登录」并由守卫跳转登录。
- **状态：** ✅ 已修复（2026-08-09；验收：`npm run build` + `npm run test` 20/20 全绿）。

### R05-02 Setup 导入限流缺专门单测：验收项「第 6 次/分钟 429」无覆盖

- **现象：** Build3 Step 4 验收标准列明后端单测须覆盖「Setup 导入限流（第 6 次/分钟 429）」，但现有测试仅覆盖限流中间件的通用机制（FixedWindow/阈值读取/scope 隔离），`setup_import` 作用域无集成级验证。
- **根因：** 限流中间件为 Build1 通用机制复用，Build3 验收时未为 Setup 导入端点补充端点级测试。
- **影响范围：** 无功能缺陷（端点限流实际生效，经 R05-01 修复验证），仅验收标准的单测覆盖项缺失。
- **修复方案：** `server_test.go` 新增 `TestSetupImportRateLimit`：对 `/api/setup/import` 连续 6 次 POST，断言前 5 次非 429、第 6 次 429。
- **状态：** ✅ 已修复（2026-08-09；验收：`go build/vet/test` 全绿，`TestSetupImportRateLimit` 通过——前 5 次 400、第 6 次 429）。

### R04-01 版本子页无返回入口 + 预览空白：拦截器破坏 text 响应

- **现象：** ① 四类资源版本管理子页无返回上级列表按钮；② 版本「预览」与用户端规则「预览」弹窗空白。
- **根因：** ① UI §1 PageHeader（含面包屑）设计未落地，版本子页页头仅标题+创建按钮；② `request.ts` 响应拦截器对 `responseType: 'text'` 的响应无条件执行 `return body.data`——字符串无 `.data` 属性 → 预览内容为 undefined（后端 text/plain 返回正常，已实测）。
- **影响范围：** 订阅/分享/规则/自定义四类版本预览 + 用户端规则预览全部空白；版本子页导航体验缺失。
- **修复方案：** ① 拦截器对非 JSON 响应（`responseType` 非 json）原样返回 `resp.data`；② VersionManageView 新增 `backPath` prop + 页头「返回」按钮（箭头图标），路由四类传参（custom 暂回订阅管理，Build3 用户管理接通后改 /admin/users）。
- **状态：** ✅ 已修复（2026-08-09；验收：browser-use 实测订阅/规则版本预览内容正常显示、返回按钮跳转对应列表页成功）。

### R03-01 版本管理页崩溃与版本无法创建：路由 prefix 双重 /api + 文本模式缺 mode 参数

- **现象：** ① 进入版本管理页报 `Cannot read properties of undefined (reading 'list')`；② 在线编辑保存报「未接收到文件」，任何版本无法创建。
- **根因：** ① 路由 `versionRoutes` 的 `prefix` 传了 `/api/admin/...`（带 /api），与 axios baseURL `/api` 拼接为 `/api/api/...`，后端 NoRoute 对 GET 一律 SPA 回退 200 + HTML，拦截器解包出 undefined；② 前端 `versionApi.create` 文本模式发 JSON 但未带 `?mode=text`，后端按查询参数区分文件/文本双模式，误走文件分支报「未接收到文件」。
- **影响范围：** 四类资源（订阅/分享/规则/自定义）的版本管理页全部崩溃、文本建版本全部失败。
- **修复方案：** ① 路由 prefix 改相对路径（4 条）；② 前端文本模式显式拼接 `?mode=text`。
- **状态：** ✅ 已修复（2026-08-09；验收：browser-use 真实浏览器创建文本版本 v1 成功、版本列表正常渲染）。

### R03-02 订阅/规则标识改为系统自动生成（用户决策）

- **现象：** 新建订阅/规则需手填标识，用户要求自动生成（标识作为系统内部唯一 ID）。
- **根因/决策：** 与 Design1 §2.2/§3.4.1/§3.4.7 原「手填标识」设计冲突，经用户确认变更设计：四类资源（订阅/分享/规则/自定义）全部自动生成（`subscription-`/`share-`/`rule-`/`custom-` 前缀 + 8 位随机短码，跨四类唯一性校验、冲突重试 3 次）；创建接口兼容手填（非空仍交叉校验）。
- **修复方案：** 后端 `GenerateSlugTx` 事务内生成（subscription/rule 空 slug 时）；前端表单移除标识输入；同步 Design1/UI 文档。
- **状态：** ✅ 已修复（2026-08-09；验收：browser-use 创建订阅自动生成 `subscription-4mn9jy2n`、规则 `rule-tzmurcws`，表单无标识输入）。

### R03-03 下载文件丢失原始扩展名（用户决策：保留原始扩展名）

- **现象：** 分享/规则/订阅下载的文件无扩展名（文件名 = 资源名），原始上传格式丢失。
- **根因：** 下载文件名 = 资源名（无扩展名）；版本表未记录原始文件名；分享/规则下载不应用平台附加头（仅 clash-verge 预置 `subscription.yaml`）。
- **修复方案：** 迁移 1007 新增 `versions.file_name`；上传时记录原始文件名（文本模式按类型补默认：订阅/分享/自定义 `.yaml`、规则 `.conf`）；下载统一「资源名 + 原始扩展名」（平台附加头优先覆盖）；用户下载端点补 Content-Disposition。
- **状态：** ✅ 已修复（2026-08-09；验收：分享下载 `我的分享.yaml`、规则下载 `分流规则B.conf`，Content-Disposition 验证通过）。

### R03-04 管理面板无返回主界面入口（用户决策：侧边栏底部）

- **现象：** 管理面板仅「切换暗色/退出」，无法一键返回用户端首页。
- **修复方案：** 侧边栏底部新增「返回主界面」+ 自定义「收起菜单」按钮（AntD Sider 默认 trigger 与底部按钮重叠导致误触，一并置空 trigger）。
- **状态：** ✅ 已修复（2026-08-09；验收：browser-use 点击返回主界面跳转 `/` 成功）。

### R02-01 订阅面板报错「a.map is not a function」：列表接口契约不一致 + 空列表返回 null

- **现象：** 进入管理端「订阅」面板提示 `a.map is not a function`，列表无法渲染（未创建任何订阅时必现）；用户组/分享/平台/规则面板同样受影响。
- **根因（两个叠加问题）：**
  1. **前后端契约不一致**：`/api/admin/groups`、`/api/admin/shares`、`/api/admin/rules`、`/api/admin/platforms` 返回 `{code,data:{list,total}}`（ListData 包裹），而前端 `group.ts`/`share.ts`/`rule.ts`/`platform.ts` 按裸数组解包，对 `{list,total}` 对象调用 `.map()` 崩溃；`subscription/home/rules-user/versions` 则返回裸数组，与 AGENTS §4.8「列表统一包裹结构」不一致。
  2. **nil slice 序列化为 null**：各 List 方法 `var out []T` 空库时返回 nil slice，`response.OK`（`Data any json:"data,omitempty"`）序列化为 `null`，前端 `.map()` 同样崩溃。
- **影响范围：** 全部列表类接口与页面（订阅/用户组/分享/平台/规则/版本/用户端首页），空数据场景必现。
- **修复方案：**
  1. 后端 4 处裸数组列表改 ListData 包裹（subscription list、rule userList、home platforms、版本 list 共享组件）；
  2. 后端 10 处列表构建点 `var out []T` → `make([]T, 0)`（subscription×2、group×2、share/custom/version/rule/platform/home 各×1）；
  3. 前端 8 处列表封装统一解包 `.list`（subscription/group/platform/share/rule×2/home/version）；
  4. 新增守护测试 `TestListEmpty`（空库 List 返回非 nil 空数组）；home-view.spec mock 保持调用方视角（数组）。
- **状态：** ✅ 已修复（2026-08-09）
  - 验收命令与实际结果：`go build/vet/test` 全绿（20 包 + TestListEmpty）；`npm run build` + `npm run test` 全绿（20/20）；`docker compose build` + `up -d` 后 browser-use 真实浏览器遍历订阅/用户组/平台/分享/规则/首页全部正常渲染、无错误提示。

### R01-01 生产构建白屏：antd/vendor manualChunks 拆包触发跨 chunk 循环引用

- **现象：** Docker 部署（`docker compose up`）后浏览器访问首页一片空白；DevTools Console 报 `Cannot access 'Q' before initialization`；Network 面板 index.html 与全部 `/assets/*` 均 200 加载成功，仅 JS 执行中断导致 Vue 未挂载。Vite dev server（`npm run dev`）模式下无此问题。
- **根因：** [vite.config.ts](./frontend/vite.config.ts) 中 `manualChunks` 将 `ant-design-vue` 独立拆为 antd chunk、`vue` 等拆为 vendor chunk。ant-design-vue 4.2.x 内部存在模块循环依赖，rollup 拆包后形成**跨 chunk 循环引用**，模块初始化顺序触发 TDZ（暂时性死区）错误。Build1 Step 2 验收时仅执行构建命令、未做真实浏览器验证，导致漏检。
- **影响范围：** 所有生产构建产物（`npm run build` / `docker compose build`）均白屏，覆盖 Build1+Build2 全部前端功能；开发模式不受影响。
- **修复方案：** 移除 `manualChunks` 自定义拆分，交由 rollup 自动分割（antd 4.x 循环依赖在单 chunk 内可安全处理）。同步更新 Build1.md Step 2 的 manualChunks 描述。
- **状态：** ✅ 已修复（2026-08-09）
  - 验收命令与实际结果：`npm run build` 通过（vue-tsc + vite 无错误）；`npm run test` 20/20 通过；`docker compose build` + `up -d` 后真实浏览器验证 `http://127.0.0.1:8080/` 正常跳转 `/setup`，Setup 页完整渲染，Console 零错误。

---

## 二、格式说明（新问题记录模板）

发现问题时，按以下结构追加到「进行中问题」：

```
### RXX-01 问题标题

- **现象：** ...
- **根因：** ...
- **影响范围：** ...
- **修复方案：** ...（决策后同步更新至 BuildN 对应 Step）
- **状态：** ☐ 待修复 / ◧ 修复中 / ✅ 已修复（日期 + 验收命令）
```

**流程约定：**

1. 问题发现 → 记录现象/根因/影响范围；
2. 存在方案取舍时，使用提问工具附推荐选项与用户确认；
3. 修复方案确定后，由 [BuildN.md](./BuildN.md) 承接为构建 Step；
4. 修复完成并验收通过后，更新状态为 ✅ 并记录验收命令与实际结果；
5. 非问题的优化候选 / 已知遗留事项归 [Design1.md](./Design1.md) 记录，不记录在本文件。

---

## 三、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-09 | 初始版本：记录 R01-01 生产构建白屏（manualChunks 跨 chunk 循环依赖）修复闭环 |
| v1.1 | 2026-08-09 | 追加 R02-01 空列表接口返回 null 导致前端 .map 崩溃修复闭环（10 处列表构建点 + 守护测试） |
| v1.2 | 2026-08-09 | 追加 R03-01~04：版本页双重 /api 前缀 + 文本模式缺 mode 参数、订阅/规则标识自动生成（用户决策）、下载保留原始扩展名（用户决策）、返回主界面按钮 |
| v1.3 | 2026-08-09 | 追加 R04-01：版本子页返回入口 + 拦截器破坏 text 预览响应 |
| v1.4 | 2026-08-09 | 追加 R05-01/02：Setup 导入前端入口缺失（配置导入双入口仅面板侧）+ Setup 导入限流缺专门单测（Build3 全量审查补漏） |
| v1.5 | 2026-08-09 | 追加 R06-01~08：审阅报告 8 项问题修复闭环（用户管理下拉不 可用/SSE 401/DB 损坏未进应急/站点名不生效/验证码未接入/上传缺 50MB 预校验/smoke 契约滞后/邮箱即时误报） |
| v1.6 | 2026-08-09 | 追加 R07-01~08：端到端核查发现——R07-01 新用户未自动加入默认组（三路径 INSERT 缺 group_id）已修复闭环；R07-02~08 待修复（公告首页缺失/日志日期筛选时区错位/首页更新时间戳无时区/无版本订阅预览 500/Dev 导入 400 非 403/模拟 OIDC 表单缺 role-group 属性/登录页瞬态 401 存疑） |
| v1.7 | 2026-08-09 | R07-02~08 全部修复闭环：公告公开端点 + 首页展示（TestPublicAnnouncement）、日志日期 UTC 转换（仅前端）、三处时间戳统一 RFC3339（home/share/rule + 前端 fmtTime）、无版本预览/下载 404 + version_missing 日志（TestPreviewNoVersion）、Dev 导入 403 哨兵映射（ErrModeRestricted）、模拟 OIDC 表单补 role/group、瞬态 401 根因确认（一键清空/Setup 导入未清失效凭据 → 三处清理）。验收：`go build/vet/test` 25 包全绿、`npm run build` + `vitest` 20/20、Dev 实例端到端冒烟全部符合预期 |
| v1.8 | 2026-08-09 | 追加 R08-01~05：全量端到端复验（五阶段）发现——R08-01/02 面板一键清空与面板配置导入 UI 提交空确认词致 400（ConfirmModal 确认词不回传父组件，API 层正常；Setup 导入硬编码不受影响）、R08-03 版本列表 file_name 恒空、R08-04 unassigned 仍生成 Token（设计文字冲突，待决策）、R08-05 同机往返导入会话不失效（边界行为，待决策）。复验结论：AGENTS 强要求全部合规、Build3 六步验收标准复验通过、R07 系列回归通过（含新增模块 A~G 动态测试 178 项断言 + 应急/破坏性全链路） |
| v1.9 | 2026-08-09 | R08-01~05 修复闭环：R08-01 一键清空硬编码 RESET（删 confirmWordInput 依赖）、R08-02 面板导入补密码输入框 + 硬编码 IMPORT（核查修正：原记载「提交空确认词 400」实为密码框缺失请求未发出，双缺陷一并修复）、R08-03 ListVersions 补 file_name（新增 TestListVersionsFileName 守护）；R08-04/R08-05 用户决策保持现状，Design1 §2.2/§3.4.8 表述同步修订（无代码变更）。验收：`go build/vet/test` 25 包全绿、`npm run build` + `vitest` 20/20 |
| v1.10 | 2026-08-09 | UI 体验 8 项修复闭环（R09-01~08）：新增通用顶栏组件 AppHeader（用户名按钮/组名直示/点击展开 Dropdown/暗色 emoji+开关），主页与管理面板共用；管理面板补顶栏（桌面+移动）；个人中心补返回按钮；登录页暗色改 emoji+开关；深色顶栏文字补 dark 配色；移动端浅色背景覆盖根治（废弃 AntD Layout.Header）。验收：`npm run build` + `vitest` 20/20、浏览器实测（桌面 1044px/移动 575px 双视口计算样式取证）全部符合预期；Design1-UI.md 同步 v1.3 |
| v1.11 | 2026-08-09 | R09-09 面板配置页双侧栏滚动修复：主边栏 Sider 内联 `position:sticky` 吸顶（规避 AntD position:relative 同特异性覆盖）+ 高 100vh + overflow-y-auto；辅边栏 Anchor 容器 sticky top-20 + 限高 calc(100vh-6rem) + 独立滚动条。验收：`npm run build` + `vitest` 20/20、浏览器实测滚动 2953px 后双侧栏吸顶（top 保持 0/80px）、注入超限内容出现独立滚动条 |
| v1.12 | 2026-08-09 | R09-10 面板配置锚点跳转被顶栏遮挡修复：Anchor 补 `:offset-top="80"`（点击路径）+ 分区卡片 `scroll-margin-top: 80px`（原生 #hash 直链兜底）。验收：`npm run build` + `vitest` 无回归、浏览器实测点击锚点与 URL 直链双路径目标 top=80px 不被遮挡 |
| v1.13 | 2026-08-09 | R09-11 主界面池内订阅展示与对齐调整（用户确认最终方案）：移除 slug 仅展示名称；逐行 border-b 改圆角浅色块行（方案 C：rounded-md + 浅灰/暗色半透明底 + space-y-2）；用户名 Dropdown 宽自适应（w-auto min-w-32）；按钮容器 AntD Space 改普通 flex div（消除 4px 垂直偏移异常，实测 diff=0）。验收：`npm run build` + `vitest` 20/20、双行文本/按钮中心完全对齐、浅/暗色样式正常；同步 Design1-UI.md v1.4、清理 HomeLayout 过时注释 |
| v1.14 | 2026-08-09 | R09-12 移动端易用性：补齐分享订阅/用户组/规则管理/访问日志四处 <768 卡片双态实现（此前仅平台/订阅有卡片；日志 8 列精简展示、需重选组橙色描边）。验收：`npm run build` + `vitest` 20/20、浏览器 575px 移动端实测四页卡片渲染正常表格隐藏；Design1-UI.md v1.5 |
| v1.15 | 2026-08-09 | R09-13 用户名 dropdown 暗色模式可读性增强：overlay 加 shadow-lg + border（浅色 gray-200 / 暗色 gray-600 边框）。验收：生产构建实测暗色 1px gray-600 边框 + 多层阴影、浅色 gray-200 边框、vitest 20/20；备注 dev server 环境 popup 渲染异常（生产正常） |
| v1.16 | 2026-08-09 | 代码质量核验清理：修复 HomeView 文件头过期注释（「替换 Build1 占位」）与 AdminLayout 菜单注释（「Build3 实现，本 Step 隐藏」——Build3 已验收，与实际不符）两处历史遗留注释；核验结论：全部改动无未使用 import、无中间方案残留、无魔法数、符合 AGENTS 规范。验收：`npm run build` + `vitest` 20/20 |
| v1.17 | 2026-08-10 | 追加 R10-01：用户组编辑弹窗组名空白且保存必败（getGroup 未解包嵌套响应 {group,selections}，detail.name/id 为 undefined）。修复：api/group.ts getGroup 解包嵌套结构为扁平 GroupDetail（方案 A，前端最小改动，后端零改动）。验收：`npm run build` + `vitest` 20/20、browser-use 实测弹窗回显「默认组」+ 保存 200 |
| v1.18 | 2026-08-10 | 追加 R10-03：OIDC 启用规则 4 个 Select 预填空格（未配置态零值回显被 AntD 渲染为空 tag）。修复：loadOidcRules 逐字段归一化（声明路径空→默认值、白名单 null→[]）。验收：`npm run build` + `vitest` 20/20、浏览器实测空 tag 消失 |
| v1.19 | 2026-08-10 | 追加 R10-05：favicon 未加载（dev proxy 缺 /public + 默认 favicon.svg 缺失，双根因）。修复：vite.config.ts/js 同步补 /public 代理、新建 public/favicon.svg。验收：dev/prod 均 200 图片类型、浏览器实测 favicon 192×192 加载成功 |
