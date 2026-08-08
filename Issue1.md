# Issue1.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考）。
> 设计记录见 [Design1.md](./Design1.md)；构建方案见 [Build1.md](./Build1.md)、[Build2.md](./Build2.md)（已归档）、[Build3.md](./Build3.md)；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 一、进行中问题

当前无进行中问题。已闭环问题见下：

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
| v1.5 | 2026-08-09 | 追加 R06-01~08：审阅报告 8 项问题修复闭环（用户管理下拉不可用/SSE 401/DB 损坏未进应急/站点名不生效/验证码未接入/上传缺 50MB 预校验/smoke 契约滞后/邮箱即时误报） |
