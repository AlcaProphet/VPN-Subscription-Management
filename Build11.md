# Build11.md — VPN 订阅管理系统 UI/UX 改进当前构建方案

> **文档定位：** 本文档是承接 UI/UX 研究报告（[UIReport1.md](docs/reports/UI/UIReport1.md)、[UIReport2.md](docs/reports/UI/UIReport2.md)）后的**当前构建方案**，用于指导后续 UI/UX 改进的实施、测试与文档同步。
> - 研究依据：`docs/reports/UI/UIReport1.md`、`docs/reports/UI/UIReport2.md`
> - 当前设计：`Design2.md`、`Design2-UI.md`（UI 改进涉及主色、导航等原设计结论，需按本文件同步修改）
> - 编码指令：[AGENTS.md](AGENTS.md)（唯一强要求）
> - 构建记录归档：`docs/reports/Build/`（历史 Build1~Build10 均已存档，仅核查）
>
> **执行原则：**
> - 每一步完成后均可编译、可测试。不跳步、不并行多步。
> - AI 执行指令：每次仅执行一个 Step，完成后运行验收命令，确认通过后再进入下一步。
> - 排序原则：先修复后构建、先安全后优化、先依赖后独立。
> - UI/UX 改进原则上不改变产品功能、权限模型或后端契约；本卷获用户确认允许**最小后端调整**。
>
> **本版状态：** v1.0 仅创建本文档，未改动任何代码/文档。

---

## 〇、本卷已确认决策

| ID | 决策项 | 用户选择 | 与现有设计的关系 |
|----|--------|----------|------------------|
| D1 | 改进优先级 | 按 UIReport2 分阶段：先可信状态 → 手机可操作 → 结构统一 → 视觉系统 | 不冲突 |
| D2 | 管理导航 | 分组但不折叠 | 与 `Design2-UI.md §2.1`“平铺不分组”冲突，实施时需同步修改 |
| D3 | 视觉 Token | 采用 `#2563EB` 主色 + 完整 Token | 与 `Design2-UI.md §1.1`“默认科技蓝、零定制”冲突，实施时需同步修改 |
| D4 | 设置页承载 | 同一路由 + 桌面分组导航 + 手机 Select | 与当前长页锚点不同，属信息架构调整 |
| D5 | 手机复杂表单 | 手机全屏 Drawer + 桌面宽 Modal/独立页共用表单 | 与当前固定宽度 Modal 不同 |
| D6 | 首次发布引导 | 新增 `/admin` 概览页 | 新增管理端概览路由 |
| D7 | 页面覆盖 | 包含高级模式页面（Xray 实例、用户组） | 需补充高级模式页面的 UI 审计与实施 |
| D8 | 后端边界 | 允许最小后端调整 | 仅限重置 token 状态、OIDC 错误映射等必要项 |

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 0 | 创建 Build11 文档，汇总 UI 报告与用户决策 | ✅ 已完成（本文档） |
| 1 | Phase 0：可信状态修复 | ☐ 未开始 |
| 2 | Phase 1：手机任务可完成性 | ☐ 未开始 |
| 3 | Phase 2：信息架构与结构统一（含 `/admin` 概览、高级模式） | ☐ 未开始 |
| 4 | Phase 3：视觉 Token 与主题统一 | ☐ 未开始 |
| 5 | Phase 4：效率润色与全量收口 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/assembly/PreviewStep.vue`、`AssemblerShell.vue`、`frontend/src/api/assembly.ts`、`frontend/src/views/EmergencyView.vue`、`frontend/src/views/ResetView.vue`、`frontend/src/views/OidcCallbackView.vue`、`frontend/src/views/LoginView.vue`、`frontend/src/api/auth.ts`、`frontend/src/api/oidc.ts`、相关后端文件 | 预览指纹、按目标过滤警告、应急/重置/OIDC 状态可信、装配跨页上下文恢复 |
| 2 | `frontend/src/components/AppHeader.vue`、`frontend/src/layouts/AdminLayout.vue`、`frontend/src/views/admin/NodesView.vue`、各管理列表页/手机卡片、装配步骤组件 | 手机触控目标、顶栏重构、全屏 Drawer、手机分段选择、危险操作隔离 |
| 3 | 新增 `PageShell.vue`、`EmptyState.vue`、`FormSection.vue`、`ResponsiveCollection.vue`、`PreviewState.vue`、`ContextBar.vue`、`AdminOverviewView.vue`；改造 `PageHeader.vue`、`TriStateList.vue`、`AdminLayout.vue`、`SettingsView.vue`、`VersionManageView.vue`、`XrayInstancesView.vue`、`GroupsView.vue` | 页面骨架、空态、设置六分组、管理导航分组、`/admin` 概览、版本标题、高级模式重组 |
| 4 | `frontend/src/theme.ts`、`frontend/src/style.css`、`frontend/tailwind.config.js`、`frontend/src/App.vue`、各组件类名 | CSS 变量、AntD/Tailwind 对齐、主色 `#2563EB`、暗色表面分层、排版/圆角/阴影 |
| 5 | `frontend/src/views/admin/assembly/PreviewStep.vue`、`frontend/src/components/DiffView.vue`、各空态/加载态、`frontend/tests/`、`.smoke-test.sh`、`.smoke-test-prod.sh`、`Design2-UI.md`、`AGENTS.md` | 预览/差异增强、轻动效、测试补强、文档同步 |

---

## 三、构建顺序依赖图

```
UIReport1/UIReport2 已完成
  → Build11 Step 1（可信状态修复，独立优先）
  → Build11 Step 2（手机任务可完成性，可与 Step 1 并行观察，但建议 Step 1 先行）
  → Build11 Step 3（信息架构/结构统一，依赖 Step 1/2 的交互和表单形态稳定）
  → Build11 Step 4（视觉 Token 与主题统一，依赖 Step 3 组件骨架）
  → Build11 Step 5（效率润色、测试与文档收口，依赖 Step 1~4）
```

> 若有后端最小调整，统一在对应 Step 中完成，并在本文件变更记录中注明。

---

## 四、分步构建计划

### Step 1：Phase 0 — 可信状态修复

- **目标：** 让用户相信屏幕上的状态，避免误判、误操作或暴露内部错误。
- **前置条件：** 无；需要时可先补充后端最小接口。
- **产出文件与操作：**

  1. `frontend/src/views/admin/AssemblyView.vue`：
     - 增加 `previewFingerprint`，覆盖目标类型、平台、规则、头部、节点、代理组、规则素材、覆盖层等影响产物的字段。
     - 记录最近一次成功预览的 fingerprint；任一字段变化后标记预览过期，旧预览不能作为当前预览。
     - “确认生成”仅在最新预览可用时允许。
     - 仅对 `clash-yaml` 与 `sr-conf` 渲染规则素材类警告，节点订阅不再出现无关警告。

  ```ts
  // 参考实现（伪代码）
  const previewFingerprint = computed(() => JSON.stringify({
    targetSyntax: targetSyntax.value,
    platformId: form.platform_id,
    ruleId: form.rule_id,
    ruleName: form.rule_name,
    fixedParams: form.fixed_params_text,
    nodes: form.node_names,
    groups: form.group_names,
    groupOrders: form.group_node_orders,
    overseas: form.overseas_members,
    pools: form.pools,
    customRules: form.custom_rules,
    final: form.final_direction,
    overlay: form.overlay,
  }))
  const previewStale = computed(() =>
    !!previewText.value && previewFingerprint.value !== lastPreviewFingerprint.value,
  )
  ```

  2. `frontend/src/views/admin/assembly/PreviewStep.vue`、`AssemblerShell.vue`：
     - 增加预览四态：未生成 / 生成中 / 最新 / 已过期。
     - 已过期时显示“配置已变化，请重新预览”，旧正文可折叠查看但不可确认生成。
  3. `frontend/src/views/EmergencyView.vue`：
     - 先读取权威应急状态；非应急状态显示普通 Result，不展示“正常服务已暂停”和操作码表单。
  4. `frontend/src/views/ResetView.vue` + `frontend/src/api/auth.ts` + 后端最小调整：
     - 无 token 直接显示无效链接 Result；
     - 如需区分缺失/过期/已使用，增加最小校验接口后再渲染表单。
  5. `frontend/src/views/OidcCallbackView.vue`、`frontend/src/views/LoginView.vue` + 后端最小调整：
     - 将 `exchange_failed` 等内部 key 映射为用户可读错误；
     - 提供重新 OIDC、本地登录、联系管理员等恢复路径。
  6. 装配跨页上下文：
     - 从装配跳转节点/订阅时保存 `tab`、目标对象、步骤和必要草稿；
     - 返回时通过 `ContextBar` 展示来源任务，并支持一次操作回到原装配类型和步骤。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  cd backend && go build ./... && go vet ./... && go test ./...
  ```
- **验收标准：**
  - 切换四种装配类型或修改任何影响产物字段后，旧预览不再作为当前预览。
  - 通用节点/SR 节点装配不出现规则素材类警告。
  - 非应急状态访问 `/emergency` 不展示应急操作。
  - 无 Token 重置页不渲染密码表单；无效状态明确可恢复。
  - OIDC 失败页面不出现内部 key，至少有可用的恢复动作。
  - 从装配跳转补前置条件后，能回到原装配位置。

---

### Step 2：Phase 1 — 手机任务可完成性

- **目标：** 让 390px 下能可靠完成主要操作，消除小控件、拥挤顶栏和不可见底部操作。
- **前置条件：** Step 1 完成（若并行，需先确认不互相影响）。
- **产出文件与操作：**

  1. `frontend/src/components/AppHeader.vue`：
     - 手机顶栏只保留汉堡、站点名、账户入口；主题切换收进账户菜单或独立图标。
     - 账户按钮、菜单项、主题切换均提供 ≥44px 命中区。
  2. `frontend/src/layouts/AdminLayout.vue`：
     - 手机 Drawer 顶部显示当前页面和关闭按钮，底部固定“返回主界面”。
     - 收起侧栏时每个图标带 Tooltip。
  3. 所有管理列表与用户页手机卡片：
     - 每卡片只保留一个主操作，次要/危险操作进“更多”并按风险分组。
     - 危险操作不与高频操作并排。
  4. `frontend/src/views/admin/NodesView.vue`：
     - 手机复杂表单改为全屏 Drawer；
     - 固定顶部标题/关闭、底部“取消/保存”；
     - 字段按“必填连接信息 / 协议扩展 / 传输与 TLS / 高级网络”渐进展开；
     - UUID 等关键字段独占一行，避免挤压。
  5. 装配器手机端：
     - 手机 Tabs 改中文分段/Select；
     - 步骤条紧凑，固定底部“上一步/下一步”。
  6. 全项目触控基线：
     - 320、360、390、430、768、1024、1440 无横向溢出；
     - 所有可交互控件命中区 ≥44×44px。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - 390×844 下可完成节点必填区、账户菜单、装配类型选择和版本查看。
  - 所有触控目标 ≥44px；危险操作无并排误触风险。
  - 320–430px 无横向页面溢出。

---

### Step 3：Phase 2 — 信息架构与结构统一

- **目标：** 统一页面骨架、空态、表单、浮层和导航；完成设置页重组、管理导航分组、`/admin` 概览和高级模式页面改造。
- **前置条件：** Step 1/2 完成。
- **产出文件与操作：**

  1. 新增通用组件：
     - `PageShell.vue`：唯一 `h1`、描述、最多一个主操作、ContextBar、StateContainer。
     - `EmptyState.vue`：标题/描述/最多两个动作，限制在合理高度内。
     - `FormSection.vue`：每组 6–8 个字段，帮助文本不只放 placeholder。
     - `ResponsiveCollection.vue`：桌面表格 + 手机卡片统一操作收口。
     - `PreviewState.vue`：未生成/生成中/最新/已过期四态。
     - `ContextBar.vue`：来源任务、未保存、后台任务、前置条件。
  2. `frontend/src/components/PageHeader.vue`：
     - 标题 24px，页面唯一 `h1`；危险操作不进入主操作位。
  3. `frontend/src/components/TriStateList.vue`：
     - 升级为 StateContainer；空态隐藏分页、批量、清空。
  4. `frontend/src/layouts/AdminLayout.vue`：
     - 管理导航改为分组但不折叠：
       - 分发：订阅、分享、平台、规则；
       - 装配：订阅装配、节点、Xray 实例（高级）；
       - 成员：用户、审批中心、用户组（高级）；
       - 系统：面板配置、日志。
     - 收起态用分隔线保留分组关系，每个图标有 Tooltip。
  5. `frontend/src/views/admin/SettingsView.vue`：
     - 重组为 6 个任务分组：身份与访问、通知、外观与内容、运行与安全、数据管理、危险操作。
     - 桌面左侧分组导航，手机 Select；
     - 每组独立保存，顶部显示未保存分区数，离开时仅在确有修改时提示。
  6. 新增 `frontend/src/views/admin/AdminOverviewView.vue` 与路由 `/admin`：
     - 使用现有状态推导，不新增多余后端统计；
     - 包含首次发布任务清单、平台/订阅/节点/规则概况、快捷入口。
  7. `frontend/src/views/admin/VersionManageView.vue`：
     - 标题使用真实资源名 + 中文类型 Badge；
     - 当前版本突出，高频操作直显，其余进更多。
  8. 高级模式页面：
     - `XrayInstancesView.vue`：顶部实例状态摘要，详情内按“节点 / 账号 / 对账”分区，高风险动作进更多。
     - `GroupsView.vue`：页头解释用户组影响，编辑按“基础信息 / 流量 / 可用节点”分段。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - 所有管理页面唯一 `h1`；空态均有可执行下一步。
  - 设置分区可在 10 秒内找到，未保存状态明确。
  - 管理导航分组清晰，收起态 Tooltip 完整。
  - `/admin` 概览可进入且不依赖新增指标接口。
  - 高级模式页面基础流程可读、可操作。

---

### Step 4：Phase 3 — 视觉 Token 与主题统一

- **目标：** 让 AntD 与 Tailwind 共用同一套 CSS 变量，消除暗色多套表面断层。
- **前置条件：** Step 3 完成（PageShell/卡片/状态组件已落地）。
- **产出文件与操作：**

  1. `frontend/src/theme.ts`：
     - 主色调整为 `#2563EB`；
     - `ConfigProvider` token 映射 `colorPrimary`、`colorBgLayout`、`colorBgContainer`、`colorBorder`、`colorText*`。
  2. `frontend/src/style.css`：
     - 定义 `--ui-page`、`--ui-surface`、`--ui-surface-raised`、`--ui-border`、`--ui-text`、`--ui-text-secondary` 等 CSS 变量；
     - `.dark` 下定义深色变量，禁止使用纯黑。
  3. `frontend/tailwind.config.js`：
     - 自定义颜色引用 CSS 变量，取代散落的 `gray-*`。
  4. 全局组件样式：
     - 统一圆角：control 8px、card 12px、modal 14px；
     - 统一阴影：卡片轻阴影、浮层高阴影；
     - 统一排版：页面标题 24/32、区块 18/26、正文 14/22、辅助 13/20。

  ```css
  /* 参考实现（伪代码） */
  :root {
    --ui-page: #f1f5f9;
    --ui-surface: #ffffff;
    --ui-surface-raised: #ffffff;
    --ui-border: #cbd5e1;
    --ui-text: #0f172a;
    --ui-text-secondary: #475569;
  }
  .dark {
    --ui-page: #0b1120;
    --ui-surface: #111827;
    --ui-surface-raised: #1e293b;
    --ui-border: #334155;
    --ui-text: #f8fafc;
    --ui-text-secondary: #cbd5e1;
  }
  ```

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - 浅/暗主题核心页面视觉快照通过；
  - 暗色无纯黑断层，至少两个稳定的表面层级；
  - 主色、语义色、辅助文字通过 WCAG AA 对比度检查；
  - 页面背景/卡片/表格使用同一套 Token。

---

### Step 5：Phase 4 — 效率润色、测试与文档收口

- **目标：** 完成预览/差异增强、统一状态反馈，并补齐测试与文档同步。
- **前置条件：** Step 1~4 完成。
- **产出文件与操作：**

  1. `frontend/src/views/admin/assembly/PreviewStep.vue`：
     - 增加行号、搜索、复制、换行切换；
     - 预览与 Diff 使用 Tabs 或分栏，避免长内容重复。
  2. `frontend/src/components/DiffView.vue`：
     - 显示新增/删除计数和定位按钮；
     - 错误行可定位。
  3. 空态与加载态：
     - 统一 Skeleton、空状态插图和说明；
     - 限制空状态展示高度，避免巨大空白容器。
  4. 交互反馈：
     - 相同 Toast 去重；
     - 浮层互斥；
     - Modal/Drawer 焦点管理；
     - `prefers-reduced-motion` 支持。
  5. 测试补强：
     - `frontend/tests/` 增加装配 preview stale、移动端/响应式、重置/OIDC/应急、设置未保存等相关用例。
  6. 文档同步：
     - 更新 `Design2-UI.md §1.1`、`§2.1`；
     - 新增/更新 `AGENTS.md` 如果涉及约束；
     - 更新 `.smoke-test.sh`、`.smoke-test-prod.sh` 覆盖关键 UI 流程。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  cd backend && go build ./... && go vet ./... && go test ./...
  ```
- **验收标准：**
  - 所有新增逻辑有对应测试；
  - 关键 UI 流程无回归；
  - 文档与代码状态一致。

---

## 五、候选/待细化项

以下项已有方向，但进入具体 Step 前需再确认实现细节：

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| 1 | `/admin` 概览页具体卡片与指标范围 | 使用现有状态推导；需确认是否包含发布清单、平台/订阅/节点/规则概况、快捷入口 | 用户选择 D6 |
| 2 | 高级模式页面详细 UI 审计 | UI 报告对高级模式实机验证有限；实施前需补充 Xray/用户组页面走查 | 用户选择 D7 |
| 3 | 最小后端接口具体契约 | 重置 token 状态校验、OIDC 错误映射等；进入 Step 1 前细化请求/响应 | 用户选择 D8 |
| 4 | 设置页“未保存”状态实现方式 | 按组保存，需要确认哪些字段纳入脏检查；推荐前端本地脏检查 | UIReport2 §5.4 |
| 5 | 视觉 Token 最终数值 | 需以实际组件做 WCAG 对比度回归后定稿 | UIReport2 §6.2 |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-28 | 初始版本：汇总 UIReport1/UIReport2、当前源码审计与用户决策，作为后续 UI/UX 改进的 Build11 当前构建方案；本版未改动代码/文档 |
