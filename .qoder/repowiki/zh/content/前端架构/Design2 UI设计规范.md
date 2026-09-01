# Design2 UI设计规范

<cite>
**本文引用的文件**
- [Design2-UI.md](../../../../../docs/reports/Design/Design2-UI.md)
- [Design2.md](../../../../../docs/reports/Design/Design2.md)
- [Xray-Core-API.md](../../../../../docs/Reference/Xray-Core-API.md)
- [Design1-UI.md](../../../../../docs/reports/Design/Design1-UI.md)
- [theme.ts](file://frontend/src/theme.ts)
- [style.css](file://frontend/src/style.css)
- [App.vue](file://frontend/src/App.vue)
- [AdminLayout.vue](file://frontend/src/layouts/AdminLayout.vue)
- [HomeView.vue](file://frontend/src/views/HomeView.vue)
- [SubscriptionsView.vue](file://frontend/src/views/admin/SubscriptionsView.vue)
- [PlatformsView.vue](file://frontend/src/views/admin/PlatformsView.vue)
- [router/index.ts](file://frontend/src/router/index.ts)
- [package.json](file://frontend/package.json)
</cite>

## 更新摘要
**变更内容**
- 新增独立 Xray 账号管理功能：双轨凭据管理（自动生成/手填接管）、配额管理、推送目标选择
- 增强三区域对账界面：待补推、无头用户、疑似独立账号残留分区，ext前缀账号默认不勾选防护
- 完善Xray实例管理：实例级对账、批量初始化、节点检测刷新、采集状态监控
- 新增错误处理增强：超限前置拦截、409冲突提示、高级模式权限控制
- 改进用户体验：空状态文案统一、移动端响应式适配、暗色模式支持
- **新增 admin-assembly 路由分组规范**：将装配器、节点管理、代理组管理、Xray 实例等页面统一归入 admin-assembly 懒加载分组
- **增强 product_type 字段支持**：正确处理 yaml、subs 和 generic-subs 三种格式的平台产品类型
- **改进订阅管理界面**：平铺双态列表展示、product_type 标签显示、内容形态标识
- **优化节点分配体验**：候选集并集计算、公共节点免分配标注、缺失节点处置引导

## 目录
1. [引言](#引言)
2. [项目结构](#项目结构)
3. [核心组件](#核心组件)
4. [架构总览](#架构总览)
5. [详细组件分析](#详细组件分析)
6. [依赖分析](#依赖分析)
7. [性能考虑](#性能考虑)
8. [故障排查指南](#故障排查指南)
9. [结论](#结论)
10. [附录](#附录)

## 引言
本规范承接 Design2.md 的增量能力（规则素材池、装配拼接与 Xray 对接），对受影响的全部界面给出可落地的 GUI 样式规格：布局结构、Ant Design Vue 组件映射、状态分支、关键交互与响应式规则。本文档为全量重写式自包含文档，不与存档的 Design1-UI.md 做增量拼接；Design1-UI.md 冻结不回写。范围红线：仅覆盖 UI 层、前端实现与前后端连接契约（端点形状/字段/轮询协议）；功能行为以 Design2.md 为准。

**更新** 本次更新重点增强了Xray实例管理功能，包括独立的账户管理系统、三区域对账接口和全面的UI改进，特别是双轨凭据管理和增强的错误处理机制。同时新增了admin-assembly路由分组规范和增强的product_type字段支持。

## 项目结构
- 前端采用 Vue 3 + Ant Design Vue + Tailwind CSS；主题通过 ConfigProvider 全局注入中文与主色，暗色模式由 useTheme composable 管理并持久化到 localStorage。
- 管理面板使用侧边栏 + 顶栏 + 内容区布局；移动端 Drawer 抽屉菜单，锁背景滚动。
- 用户首页按平台卡片网格展示，新增流量卡片与分流规则卡片。
- 新增路由与菜单项：订阅装配、节点管理、代理组管理、Xray 实例（高级模式）。
- **新增 admin-assembly 路由分组**：将装配器、节点管理、代理组管理、Xray 实例等页面统一归入 admin-assembly 懒加载分组，提升打包效率。

```mermaid
graph TB
A["App.vue<br/>ConfigProvider 全局主题"] --> B["AdminLayout.vue<br/>侧边栏+顶栏+内容区"]
A --> C["HomeView.vue<br/>用户首页"]
B --> D["SubscriptionsView.vue<br/>订阅管理"]
B --> E["PlatformsView.vue<br/>平台管理"]
A --> F["theme.ts<br/>主题与暗色切换"]
A --> G["style.css<br/>Tailwind 基础样式"]
B --> H["AssemblyView.vue<br/>订阅装配admin-assembly分组"]
B --> I["NodesView.vue<br/>节点管理admin-assembly分组"]
B --> J["ProxyGroupsView.vue<br/>代理组管理admin-assembly分组"]
B --> K["XrayInstancesView.vue<br/>Xray实例admin-assembly分组"]
```

**图表来源**
- [App.vue:1-43](file://frontend/src/App.vue#L1-L43)
- [AdminLayout.vue:1-130](file://frontend/src/layouts/AdminLayout.vue#L1-L130)
- [HomeView.vue:1-231](file://frontend/src/views/HomeView.vue#L1-L231)
- [SubscriptionsView.vue:1-188](file://frontend/src/views/admin/SubscriptionsView.vue#L1-L188)
- [PlatformsView.vue:1-150](file://frontend/src/views/admin/PlatformsView.vue#L1-L150)
- [router/index.ts:27-45](file://frontend/src/router/index.ts#L27-L45)
- [theme.ts:1-27](file://frontend/src/theme.ts#L1-L27)
- [style.css:1-16](file://frontend/src/style.css#L1-L16)

**章节来源**
- [App.vue:1-43](file://frontend/src/App.vue#L1-L43)
- [AdminLayout.vue:1-130](file://frontend/src/layouts/AdminLayout.vue#L1-L130)
- [HomeView.vue:1-231](file://frontend/src/views/HomeView.vue#L1-L231)
- [router/index.ts:27-45](file://frontend/src/router/index.ts#L27-L45)
- [theme.ts:1-27](file://frontend/src/theme.ts#L1-L27)
- [style.css:1-16](file://frontend/src/style.css#L1-L16)

## 核心组件
- 通用顶栏 AppHeader：站点 ICON/名称、更新时间戳、管理面板入口、用户名 Dropdown、暗色切换开关。
- 三态列表 TriStateList：加载中 Skeleton / 空 Empty+引导 / 错误重试。
- 确认弹窗 ConfirmModal：统一危险操作二次确认。
- 通知封装 Notify：message/notification 统一调用。
- MarkdownView：公开公告/页脚 Markdown 渲染（html:false）。
- 新增 DiffView：文本差异视图（jsdiff 行级三色高亮，等宽字体、纵向滚动容器）。
- 拖拽排序交互：桌面端拖拽手柄，<768 降级为「上移/下移」按钮。
- pollTask 轮询任务封装：异步任务提交 → 间隔轮询 → 终态返回结果；卸载自动取消。

**更新** 新增了独立账号管理相关的组件，包括双轨凭据输入、配额设置、推送目标选择等功能模块。

**章节来源**
- [Design2-UI.md:25-51](../../../../../docs/reports/Design/Design2-UI.md#L25-L51)
- [Design1-UI.md:45-55](../../../../../docs/reports/Design/Design1-UI.md#L45-L55)

## 架构总览
- 页面骨架：管理面板（Sider + Header + Content）、用户端（顶栏 + 主体卡片）。
- 路由与菜单：新增订阅装配、节点、代理组、Xray 实例；高级模式驱动显隐。
- **新增 admin-assembly 路由分组**：将装配器、节点管理、代理组管理、Xray 实例等页面统一归入 admin-assembly 懒加载分组，提升打包效率和代码组织性。
- 数据流：前端通过 api/* 模块调用后端端点；长任务使用 pollTask 轮询；超时与失败有兜底文案。
- 主题与国际化：ConfigProvider 全局 zh_CN，主色 #1677FF；暗色模式随 ConfigProvider 联动。

```mermaid
sequenceDiagram
participant U as "用户"
participant V as "HomeView.vue"
participant API as "api/home.ts"
participant S as "useSystemStore"
U->>V : 打开用户首页
V->>API : 获取平台卡片与更新时间
API-->>V : 平台数据
V->>S : 拉取站点信息
S-->>V : 站点名/图标
V-->>U : 渲染平台卡片网格/公告/分流规则入口
```

**图表来源**
- [HomeView.vue:1-231](file://frontend/src/views/HomeView.vue#L1-L231)
- [App.vue:1-43](file://frontend/src/App.vue#L1-L43)

**章节来源**
- [Design2-UI.md:54-108](../../../../../docs/reports/Design/Design2-UI.md#L54-L108)
- [Design2-UI.md:467-552](../../../../../docs/reports/Design/Design2-UI.md#L467-L552)

## 详细组件分析

### 用户首页（改造）
- 卡片堆叠顺序：流量卡片 → 分流规则卡片 → 平台卡片网格 → 公告栏卡片。
- 流量卡片：基础模式显示「不限流量」；高级模式显示已用/配额进度条，超限红色提示；受面板配置「流量卡片」开关控制。
- 分流规则卡片：全体用户可见，展示默认规则版本信息与复制链接；空态提示管理员暂未设置。
- 平台卡片：普通用户三态（一键导入/复制链接/刷新链接）；管理员预览形态（模板信息 + 按平台预览当前版本）。

```mermaid
flowchart TD
Start(["进入首页"]) --> LoadData["加载平台卡片/公告/更新时间"]
LoadData --> RenderCards{"是否管理员?"}
RenderCards --> |否| UserMode["普通用户态<br/>流量卡/分流规则卡/平台卡"]
RenderCards --> |是| AdminMode["管理员预览态<br/>模板信息+预览按钮"]
UserMode --> End(["完成渲染"])
AdminMode --> End
```

**图表来源**
- [HomeView.vue:1-231](file://frontend/src/views/HomeView.vue#L1-L231)
- [Design2-UI.md:114-170](../../../../../docs/reports/Design/Design2-UI.md#L114-L170)

**章节来源**
- [Design2-UI.md:114-170](../../../../../docs/reports/Design/Design2-UI.md#L114-L170)

### 管理面板布局与路由骨架（增量）
- 侧边栏菜单新增：订阅装配、节点、代理组、Xray 实例（高级模式）。
- **新增 admin-assembly 路由分组**：将 `/admin/assembly`、`/admin/nodes`、`/admin/proxy-groups`、`/admin/xray` 四个路由迁至 admin-assembly 懒加载分组，提升代码分割效率。
- 路由守卫：高级模式未开启时访问高级路由重定向并提示。

```mermaid
graph LR
M["菜单项"] --> R["路由表"]
R --> L["懒加载分组<br/>admin-assembly"]
R --> G["路由守卫<br/>高级模式检查"]
```

**图表来源**
- [AdminLayout.vue:1-130](file://frontend/src/layouts/AdminLayout.vue#L1-L130)
- [router/index.ts:27-45](file://frontend/src/router/index.ts#L27-L45)
- [Design2-UI.md:54-108](../../../../../docs/reports/Design/Design2-UI.md#L54-L108)

**章节来源**
- [Design2-UI.md:54-108](../../../../../docs/reports/Design/Design2-UI.md#L54-L108)

### 订阅管理（改造）
- **列表页改为平铺双态列表**（不再按平台分组折叠）；列含平台名称、product_type、内容形态、当前版本、操作。
- **新增 product_type 支持**：正确处理 yaml、subs 和 generic-subs 三种格式，使用不同颜色标签区分（yaml 蓝 / subs 青 / generic-subs 紫）。
- 新建订阅弹窗：平台选择 + 名称；被占用平台禁用并标注「已有订阅」。
- 编辑弹窗：仅名称修改；组关联多选移除。
- 「已入池未生效」引导：行内高亮 + 快捷激活链接。
- 装配入口：PageHeader 右侧「前往装配」按钮。

```mermaid
sequenceDiagram
participant U as "管理员"
participant SV as "SubscriptionsView.vue"
participant API as "api/subscription.ts"
U->>SV : 点击「新建订阅」
SV->>API : createSubscription({platform_id, name})
API-->>SV : 成功/冲突(409)
SV-->>U : 成功提示/错误提示
U->>SV : 编辑/删除/版本管理
```

**图表来源**
- [SubscriptionsView.vue:1-188](file://frontend/src/views/admin/SubscriptionsView.vue#L1-L188)
- [Design2-UI.md:177-184](../../../../../docs/reports/Design/Design2-UI.md#L177-L184)

**章节来源**
- [Design2-UI.md:177-184](../../../../../docs/reports/Design/Design2-UI.md#L177-L184)
- [SubscriptionsView.vue:1-188](file://frontend/src/views/admin/SubscriptionsView.vue#L1-L188)

### 平台管理（改造）
- **列表新增 product_type 列**：显示平台的产品类型（yaml/subs/generic-subs），使用不同颜色标签区分。
- **新建平台表单新增 product_type 单选**：yaml「Clash YAML 订阅」/ subs「Shadowrocket 节点订阅」/ generic-subs「通用节点订阅（v2rayNG/v2rayN 等）」，默认 yaml。
- 平台编辑页：product_type 可校正；若与既有订阅条目不一致则拒绝并提示。
- 删除 ConfirmModal 影响清单沿用。

**章节来源**
- [Design2-UI.md:214-219](../../../../../docs/reports/Design/Design2-UI.md#L214-L219)
- [PlatformsView.vue:1-150](file://frontend/src/views/admin/PlatformsView.vue#L1-L150)

### 订阅装配页（占位页 → 实现）
- 单菜单入口页内 Tabs：规则素材池 / Clash YAML / SR 节点订阅 / SR 分流规则。
- 三类装配器共用框架：步骤条形态（默认）与单页多分区形态切换；localStorage 记忆。
- 预览步：全文纯文本预览；可选与当前激活版本 diff 对比（jsdiff 行级三色高亮）。
- 重新编辑流：从版本入口载入快照，悬空引用标记并剔除后再生成新版本。

```mermaid
flowchart TD
A["进入装配页"] --> B{"选择装配器"}
B --> |Clash YAML| C["六步流程：类型目标→头部→节点与组→规则→预览→生成"]
B --> |SR 节点订阅| D["四步流程：类型目标→头部→节点→预览→生成"]
B --> |SR 分流规则| E["四步流程：类型目标→头部→跳过→规则→预览→生成"]
C --> F["预览：纯文本 + diff 对比"]
D --> F
E --> F
F --> G["生成：入池未生效，去激活"]
```

**图表来源**
- [Design2-UI.md:273-366](../../../../../docs/reports/Design/Design2-UI.md#L273-L366)

**章节来源**
- [Design2-UI.md:273-366](../../../../../docs/reports/Design/Design2-UI.md#L273-L366)

### 节点管理页（新独立菜单）
- 列表：节点名称、来源标签、协议、地址:端口、公共节点标记、启用开关、allocatable/missing 标注、操作。
- manual 节点新增/编辑：协议注册表动态表单；凭据字段脱敏输入；重名冲突 409。
- xray 节点只读为主：enabled/is_public 两开关 + 删除；缺失节点提供删除处置。

**章节来源**
- [Design2-UI.md:369-397](../../../../../docs/reports/Design/Design2-UI.md#L369-L397)

### 代理组管理页（新独立菜单）
- 列表：组名称、类型 Tag、组类型、成员摘要、启用勾选（预设组）、操作。
- 自建组创建/编辑：基本信息、节点引用（有序列表支持拖拽）、子组引用（DAG 环形校验）、保存约束。

**章节来源**
- [Design2-UI.md:400-426](../../../../../docs/reports/Design/Design2-UI.md#L400-L426)

### Xray 实例页（新独立菜单，仅高级模式）
- 实例列表：名称、slug、api_addr、enabled 开关、采集状态、操作。
- 节点检测：刷新节点回执（新增/更新/missing/撞名跳过）。
- 开始初始化：幂等批量初始化，计数回执。
- 账号对账：待补推/无头用户，一键补推/清理。

**更新** 增强了Xray实例管理功能，新增了独立的账户管理系统和三区域对账界面。

**章节来源**
- [Design2-UI.md:429-464](../../../../../docs/reports/Design/Design2-UI.md#L429-L464)

### 账号对账区（实例级）
- 实例行「对账」按钮 → 展开对账面板（`a-drawer` 或页内分区，按实例维度）：调对账端点获取期望集比对结果。
- **比对结果表**（双态列表）：**三分区** —— ①「待补推」（期望集有 / 实例无，含面板用户与独立账号两类，来源 Tag 区分 user / ext）：用户名或账号名 + 节点 + 「补推」行操作；②「无头用户」（实例有 / 期望集无，**`user-` 前缀**）：Xray email + 所在 inbound + 清理勾选；③「**疑似独立账号残留**」（**`ext-` 前缀或无法匹配前缀者**，黄色警示分区 + 说明文案「此类账号可能为手动维护的独立账号或配置导入/重装后的残留，请确认后再清理」，**清理勾选默认不勾选**，Design2.md §5.11 对账防护）：Xray email + 所在 inbound + 清理勾选。
- **一键补推**（对待补推全集，loading 防重复；超限独立账号跳过并提示，同用户超限前置拦截口径）与 **一键清理**（对 ②③ 分区勾选项，**ConfirmModal 危险确认**「将从 Xray 删除 N 个无主账号，不可恢复」）；执行结果计数回执；单条补推行内可操作。
- 空态：三分区均空时 `a-result success` 风格提示「账号已一致，无需处理」。

**更新** 实现了三区域对账界面，特别增强了对独立账号的保护机制，防止误删手动维护的账号。

**章节来源**
- [Design2-UI.md:454-459](../../../../../docs/reports/Design/Design2-UI.md#L454-L459)

### 独立账号 Tab（Design2.md §5.11）
**列表**（双态列表）：列：名称（备注名）、email（`code` 风格）、配额（GB 或「不限」）、本月用量（字节→GB 换算见 10.1）、推送摘要（聚合 Badge 同 4.5 口径：N 个 inbound 已同步绿 / 失败红 + 行内重试 / 同步中橙）、超限标记（红色 `a-tag`「已超限」+ 行警示底色）、操作（**复制凭据** / 编辑 / **重置配额** / 删除）。

- **创建/编辑弹窗**（720px）自上而下四区：
  1. **基本信息**：名称（重名 409 提示；email 系统分配，创建后只读展示）
  2. **凭据区（双轨）**：`a-radio-group`「自动生成 / 手填接管」——自动生成：提交时生成（创建成功弹窗一次性展示，见下）；手填接管：UUID + 代理密码输入（`a-input-password`）+ 说明文案「系统分配 email 并推送至所选入站；若 Xray 侧已存在同 email 账号则按幂等口径接管成功，请确保所填凭据与 Xray 侧一致」；**编辑时凭据字段留空 = 保留原凭据**（同 6.2 manual 节点编辑口径）
  3. **推送目标**：按实例分组的 inbound 多选（仅列四协议、allocatable=1、enabled 节点；无可用节点时空态「请先在实例页检测节点」）
  4. **配额**：`a-input-number`（GB）+ 副说明「0 或留空 = 不限流量」
- **凭据展示与复制**：创建成功弹窗一次性展示 UUID / 代理密码（`code` 风格 + 复制按钮）+ 警示文案「凭据即该账号的唯一凭证，请妥善保管」；列表「复制凭据」按钮复制解密凭据（专用端点，见 9.1）+ Toast 同款警示。
- **超限与重置**：超限行红色标记 + 副文案「账号已从 Xray 移除，重置配额可恢复」；重置配额 ConfirmModal「将清空该账号本月流量累计并恢复 Xray 账号（凭据不变）」；结果回执同 4.5。
- **删除账号**：ConfirmModal 危险样式「将从全部已推入站（N 个）移除该账号并删除面板记录，不可恢复」。
- **空态**：「还没有独立账号」+「创建独立账号」按钮 + 一句话用途说明「用于向面板账号体系之外的人员/场景分发凭据（可手写入自定义订阅内容）」（见 10.2）。

**更新** 新增了完整的独立账号管理功能，包括双轨凭据管理、配额控制和推送目标选择。

**章节来源**
- [Design2-UI.md:461-473](../../../../../docs/reports/Design/Design2-UI.md#L461-L473)

## 依赖分析
- 前端依赖：ant-design-vue、@ant-design/icons-vue、axios、dayjs、markdown-it、pinia、vue、vue-router。
- 新增依赖：diff（jsdiff）用于预览 diff。
- 主题与国际化：ConfigProvider 全局 zh_CN，主色 #1677FF；暗色模式通过 useTheme 管理。

```mermaid
graph TB
P["package.json<br/>依赖声明"] --> A["ant-design-vue"]
P --> B["@ant-design/icons-vue"]
P --> C["axios"]
P --> D["dayjs"]
P --> E["markdown-it"]
P --> F["pinia"]
P --> G["vue"]
P --> H["vue-router"]
P --> I["diff<br/>jsdiff"]
```

**图表来源**
- [package.json:1-37](file://frontend/package.json#L1-L37)

**章节来源**
- [package.json:1-37](file://frontend/package.json#L1-L37)
- [Design2-UI.md:540-541](../../../../../docs/reports/Design/Design2-UI.md#L540-L541)

## 性能考虑
- 列表分页：素材池详情条目分页（默认 20 条/页），避免整表加载。
- 轮询优化：pollTask 支持网络抖动容忍（连续失败 3 次才报错），卸载自动取消。
- 渲染性能：预览产物纯文本渲染，避免引入重型编辑器；DiffView 行级高亮轻量实现。
- 主题切换：ConfigProvider 全局联动，无逐组件手工适配。
- **路由懒加载优化**：admin-assembly 分组内的装配器、节点管理、代理组管理、Xray 实例页面统一懒加载，减少初始包体积。

[本节为通用指导，不直接分析具体文件]

## 故障排查指南
- 同步失败：素材池同步失败回执展示原因（拉取失败/空响应/零条目保护/解析跳过行数）。
- 权限不足：高级模式未开启访问高级路由重定向并提示；后端 403 统一处理。
- 重复冲突：池名/节点名/平台订阅占用/实例名/代理组名 409 提示。
- 轮询超时：任务仍在后台执行，请稍后刷新查看结果。
- 移动端兼容：<768 表格卡片化，拖拽降级为「上移/下移」按钮。
- **product_type 冲突**：平台格式校正时若与既有订阅条目 product_type 不一致，后端 400 拒绝，前端表单级错误提示。

**更新** 新增了独立账号相关的故障排查指南，包括凭据冲突、配额超限等情况的处理方法。

**章节来源**
- [Design2-UI.md:542-552](../../../../../docs/reports/Design/Design2-UI.md#L542-L552)
- [Design2-UI.md:554-591](../../../../../docs/reports/Design/Design2-UI.md#L554-L591)

## 结论
本规范完整覆盖了 Design2 受影响界面的 GUI 样式规格，包括用户首页改造、管理面板增强、订阅装配实现、节点/代理组/Xray 管理等。**更新** 本次更新特别增强了Xray实例管理功能，包括独立的账户管理系统、三区域对账界面和全面的功能改进。通过统一的组件约定、响应式策略与前后端契约，确保实施一致性与可维护性。建议构建前核对空态、加载态、错误态与移动端兼容性，遵循 AGENTS.md 编码约束。

**新增特性总结**：
- admin-assembly 路由分组规范提升了代码组织和打包效率
- 增强的 product_type 字段支持正确处理 yaml、subs 和 generic-subs 三种格式
- 改进的订阅管理界面提供更好的用户体验
- 优化的节点分配和组管理功能提升了管理效率

[本节为总结，不直接分析具体文件]

## 附录
- 全局风格基础：主色 #1677FF，浅色布局，暗色模式手动切换 + localStorage 持久化。
- 响应式断点：台式 ≥1200 / 平板 768~1199 / 手机 <768。
- 空状态文案：新增页面空态统一口径，见 Design2-UI §10.2。
- 错误码映射：对齐 AGENTS.md §4.8，沿用 Design1-UI §7.3 基线。
- **新增 admin-assembly 路由分组**：将装配器、节点管理、代理组管理、Xray 实例等页面统一归入 admin-assembly 懒加载分组。

**更新** 新增了独立账号相关的空状态文案和错误处理规范。

**章节来源**
- [Design2-UI.md:25-51](../../../../../docs/reports/Design/Design2-UI.md#L25-L51)
- [Design2-UI.md:554-591](../../../../../docs/reports/Design/Design2-UI.md#L554-L591)
- [Design1-UI.md:8-36](../../../../../docs/reports/Design/Design1-UI.md#L8-L36)