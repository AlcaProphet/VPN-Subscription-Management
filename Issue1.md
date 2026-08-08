# Issue1.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考）。
> 设计记录见 [Design1.md](./Design1.md)；构建方案见 [Build1.md](./Build1.md)、[Build2.md](./Build2.md)（已归档）、[Build3.md](./Build3.md)；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 一、进行中问题

当前无进行中问题。已闭环问题见下：

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
