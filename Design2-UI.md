# Design2-UI.md — GUI 样式规格（承载 Design2.md 全部界面部件）

> **文档定位：** 本文档是 [Design2.md](./Design2.md) 增量能力（订阅装配与 Xray 对接）落地后**全部受影响界面**的 GUI 样式规格：布局结构、Ant Design Vue 组件映射、状态分支、关键交互与响应式规则。采用**全量重写式、自包含**写法——受影响页面的规格在本文档内完整描述，不与存档的 [Design1-UI.md](./docs/AchievedDocuments/Design1-UI.md) 做增量拼接；Design1-UI.md 冻结不回写。
> **范围红线：** 本文档仅覆盖 UI 层、前端实现与前后端连接契约（端点形状/字段/轮询协议）；**不写数据库 schema 与后端运行逻辑**——功能行为一律以 [Design2.md](./Design2.md) 为准（本文不重复定义，引用其章节），编码约束遵循 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。
> 本规格面向构建可直接实施；与 Design2.md 冲突时以 Design2.md 为准并提示用户。

## 未受影响页面核验结论（不编写规格）

以下页面经逐条对照 Design2.md（第一~五章 + §5.10 前端清单）核验，**无 UI 影响**，规格沿用存档 Design1-UI.md 对应章节，本文档不重复：

| 页面 | Design1-UI 章节 | 核验结论 |
|------|----------------|---------|
| Setup 首次配置向导 | 2.1 | Design2 不涉及 Setup 流程；配置导入保护（signing_key 拒绝提示）落点在面板配置分区，见本文 4.7 |
| 登录 / 注册 / 忘记密码 / 重置密码 | 2.2 | 无影响 |
| 待审批页 /pending | 2.2 | 无影响 |
| 应急恢复页 | 三 | 一键清空的表清单扩展为后端事项，页面交互不变 |
| 404 页 | 2.3 | 无影响 |
| 分享订阅管理 | 5.3 | 分享机制沿用 Design1；Design2 仅「分享下载原样返回」属后端口径，UI 不变 |
| 审批中心 | 5.6 | 审批通过后的 Xray 推送钩子为后端事务提交后行为，页面交互不变 |
| 日志查看 | 5.9 | 无影响 |
| 用户端规则浏览页 /rules | 4.2 | 规则 Token 下载与复制口径不变；首页「分流规则」卡片改造见本文 3.1 |

---

## 一、全局风格基础（沿用 + 增量）

### 1.1 风格基调（全部沿用 Design1-UI §1.1~1.3）

| 项 | 结论 |
|----|------|
| 主色 | AntD 默认科技蓝 `#1677FF`，零定制 |
| 布局基调 | 全浅色布局（浅色侧边栏 + 浅色顶栏），用户端与管理端风格统一 |
| 暗色模式 | 手动切换 + localStorage 持久化，`ConfigProvider theme.darkAlgorithm` 全局联动；新增页面不做逐组件手工适配（实时日志区固定深色底例外，本期不涉及） |
| 移动端长表格 | <768px 统一卡片化，各页面内联 `isMobile`（matchMedia <768）条件渲染双态列表 |
| 响应式三档断点 | 台式 ≥1200 / 平板 768~1199 / 手机 <768 |
| 危险操作 | 统一 `danger` 属性红色 + `ConfirmModal` 二次确认 |

### 1.2 新增通用组件与交互约定（Design2 增量）

| 组件 / 约定 | 职责 | 要点 |
|------------|------|------|
| `DiffView` 文本差异视图 | 装配预览步「与当前激活版本对比」的行级差异高亮 | 采用 **jsdiff**（npm `diff` 包，Design2.md §4.1 已定选型）做行级 diff，前端自渲染**三色高亮**（新增行绿底 / 删除行红底 / 上下文默认色），等宽字体、纵向滚动容器（max-height 60vh），**暗色模式随 ConfigProvider 主题算法（底色用 color-mix/主题 token，不固定浅色）**；**禁止引入 monaco 等重型编辑器**；**目标无激活版本时允许对比，DiffView 以空目标为基准、将预览全文渲染为「整体新增」分支**（Design2Report10 Q4） |
| 拖拽排序交互 | 组节点分配排序、代理组节点引用排序、**装配第④步素材池整体排序**、平台 scheme 排序（沿用） | 桌面端（≥768）使用拖拽手柄（`HolderOutlined` 图标）拖拽排序；**<768 移动端降级为「上移 / 下移」按钮**（触屏拖拽不可靠，统一降级口径，见 10.1）；排序变更即时调更新端点，失败回滚本地顺序并 `Notify.error` |
| `pollTask` 轮询任务封装 | 异步任务（素材池同步）状态轮询 | `api/request.ts` 新增封装，契约见 9.2：提交任务 → 按 1.5s 间隔轮询状态端点 → 终态返回结果；**组件卸载自动取消**（AbortController / 取消标志）；进行中 UI 统一 loading 态 + 防重复触发 |
| 装配器双形态切换 | 四类装配器共用的「步骤条 ⇄ 单页多分区」形态切换 | **默认步骤条形态**；分区右上角 `a-segmented`（「分步 / 单页」）切换；选择持久化 `localStorage('assembly_layout_mode')`，四种装配器共用同一记忆键；切换时**表单数据不丢失**（同一份响应式状态，仅渲染形态变化） |
| 同步状态 Badge 色系 | 素材池同步任务、Xray 推送同步状态、采集状态统一口径 | `a-badge` / `a-tag`：**pending 橙（`#FAAD14`）/ running 蓝（processing）/ synced 与 succeeded 绿（`#52C41A`）/ failed 红（`#FF4D4F`）/ partial 橙（warning）/ missing 灰（AntD 默认灰）**；失败态附「原因」Tooltip（展示后端 last_error / sync_error 可展示信息） |

### 1.3 复用件沿用清单

`TriStateList`（加载中 Skeleton / 空 Empty+引导 / 错误重试）、表格卡片双态列表、`ConfirmModal`、`Notify` 沿用既有实现；**`PageHeader`、`CopyField` 一期未落地为独立组件（各页内联实现），本期按 Design1-UI §1.5 规格新建为通用组件**；本文新增页面一律套用上述组件。

---

## 二、面板整体布局与路由骨架

### 2.1 管理面板布局（沿用 + 增量）

沿用 Design1-UI §5.0：`a-layout` 浅色侧边栏（展开 220px / 收起 64px）+ 通用顶栏 + 白底卡片内容区；<768 顶栏 ☰ 汉堡唤出 Drawer（锁背景滚动沿用）；路由级代码分割沿用。

**侧边栏菜单（13 项，图标 + 文字，平铺不分组——沿用 Design1 决策不分组）**：

| 序号 | 菜单项 | 路由 | 可见性 |
|------|--------|------|--------|
| 1 | 订阅 | `/admin/subscriptions` | 始终 |
| 2 | 用户组 | `/admin/groups` | **仅高级模式（advanced_mode=on）** |
| 3 | 分享 | `/admin/shares` | 始终 |
| 4 | 平台 | `/admin/platforms` | 始终 |
| 5 | 用户 | `/admin/users` | 始终 |
| 6 | 审批中心 | `/admin/approvals` | 始终 |
| 7 | 规则 | `/admin/rules` | 始终 |
| 8 | 面板配置 | `/admin/settings` | 始终 |
| 9 | 日志 | `/admin/logs` | 始终 |
| 10 | 订阅装配 | `/admin/assembly` | 始终（占位页移除，见第五章） |
| 11 | 节点 | `/admin/nodes` | 始终（manual 节点属基础模式能力） |
| 12 | 代理组 | `/admin/assembly?tab=proxy-groups` | 始终（已并入订阅装配，不再独立菜单） |
| 13 | Xray 实例 | `/admin/xray` | **仅高级模式（advanced_mode=on）** |

- 显隐数据源：`useSystemStore` 的 `/api/system/status` 新增 `advanced_mode` 字段（见 9.3）；开关切换后下次拉取系统状态即联动，无需整页刷新机制
- 「订阅装配」项沿用原菜单位置（第 10 位），占位页形态替换为实际功能页

### 2.2 AppHeader 增量

- **所属组名标签**（`a-tag` cyan）：**基础模式隐藏**（组概念基础模式全面隐藏，Design2.md 第一章）；高级模式沿用直示
- 其余（站点 ICON / 站点名 / 订阅更新时间戳 / 管理面板入口 / 用户名 Dropdown / 暗色切换）沿用 Design1-UI §4.1

### 2.3 路由表增量

在 Design1-UI §7.1 路由总表基础上新增两行并迁组一行（其余路由不变；`/admin/assembly` 路径不变、懒加载分组迁入 admin-assembly；代理组不再独立路由，已并入装配页 `?tab=proxy-groups`）：

| 路径 | 页面 | 布局 | 懒加载分组 |
|------|------|------|-----------|
| `/admin/assembly` | 订阅装配（Tabs 单入口，含代理组 Tab，见第五章） | 管理面板 | admin-assembly |
| `/admin/nodes` | 节点管理（见第六章） | 管理面板 | admin-assembly |
| `/admin/xray` | Xray 实例（见第八章） | 管理面板 | admin-assembly |

- **懒加载分组调整**：原 `admin-settings` 分组中的装配占位页迁出，新增 **`admin-assembly` 分组**承载装配与节点域三个页面（装配器含代理组 Tab、节点、Xray 实例共享大量节点/组勾选类子组件，同组打包收益明确）；`admin-settings` 保留面板配置与日志。**注：一期实现未配置 vite manualChunks**（ant-design-vue 4.x 模块循环依赖，手动拆 chunk 会白屏，见 vite.config.ts 注释），路由级懒加载由 rollup 自动分割——本表「懒加载分组」仅为规格归类，**无 vite 配置动作**
- 装配页支持 query 参数带目标进入（如 `/admin/assembly?tab=clash-yaml&platform_id=3&edit_version_id=5`，见 4.2/5.3 重新编辑流）；未知 tab 回退默认第一个页签

### 2.4 路由守卫增量

沿用 Design1-UI §7.2 全部守卫，新增一条：

| 守卫 | 规则 |
|------|------|
| 高级模式路由 | `advanced_mode=off` 时直接访问 `/admin/groups` 或 `/admin/xray` → 重定向 `/admin/subscriptions` + `message.warning`「高级功能未开启，请在面板配置中开启高级模式」（与后端高级端点 403 双保险，后端为准；深链直达场景兜底） |

---

## 三、用户端页面（普通用户视角重点章）

> 本章仅规格化 Design2 产生变化的用户端部件；公告栏（Markdown 渲染）、规则浏览页 /rules 沿用 Design1-UI 与本文开头核验结论。

### 3.1 用户首页 `/`（改造）

**卡片堆叠顺序**（单列纵向排列，卡片间 16px）：**流量卡片 → 分流规则卡片 → 平台卡片网格 → 公告栏卡片**。栅格沿用大屏 3 列 / 中屏 2 列 / 小屏 1 列。

#### 3.1.1 流量卡片（新增独立 Card，两模式布局完全一致）

| 模式 | 内容 |
|------|------|
| 基础模式 | 仅显示文案「**不限流量**」（无流量采集数据源，隐藏已用数值与进度条，Design2.md 第一章） |
| 高级模式（未超限） | 「本月已用 X GB / 配额 Y GB」+ `a-progress` 进度条（百分比着色：<80% 主色蓝 / 80~99% 橙 / 100% 红）；配额为不限（NULL/0）时进度条替换为「不限流量」文案，仅展示已用值 |
| 高级模式（超限） | 进度条 100% 红 + 红色 `a-alert error`「本月流量已超限，代理账号已暂停，请联系管理员重置」；**下载按钮与卡片其余能力不受影响**（超限仅移除 Xray 账号，Design2.md §5.4/5.7） |

- **显隐**：受面板配置「流量卡片」开关控制（默认开启，见 4.7）；开关关闭时整卡不渲染
- 流量数值换算：字节 → GB 保留两位小数（见 10.1）
- `<768`：卡片内文案与进度条纵向堆叠，无横向布局

#### 3.1.2 分流规则卡片（改造，**全体用户可见**，不做平台归属判定）

| 状态 | 内容 |
|------|------|
| 正常（已设置首页默认规则且有激活版本） | 卡片标题「分流规则」+ 规则名称 + 当前激活版本信息（版本号 / 更新时间）+ 「复制规则链接」按钮（`CopyField`，点击即复制 + Toast） |
| 空态（未设置默认规则 / 选中规则无激活版本） | 灰色占位文案「管理员暂未设置分流规则」+ 卡片入口能力保留 |

- **SR 双内容导入引导文案**（卡片内常驻说明行，灰色小字）：「Shadowrocket 使用指引：先添加订阅获取节点，再导入分流规则」
- **卡片入口**：点击卡片（非按钮区域）跳转现有 `/rules` 列表页查看全部 Shadowrocket 规则（不引入规则平台归属模型，Design2.md §4.4）
- 不使用一键 scheme 唤起（沿用 Design1 3.5「移除一键导入 UI」口径）

#### 3.1.3 平台卡片（新模型：每平台一份订阅）

**普通用户**——三态沿用 Design1-UI §4.1 交互形态（一键导入 / 复制链接 / 刷新链接三按钮组不变），订阅区段内容改为**该平台的唯一订阅条目**：

| 状态 | 展示 |
|------|------|
| 有激活版本 | 订阅名称 + product_type 标签（yaml / subs / generic-subs）+ 版本时间戳 |
| 平台无任何版本 / 无激活版本 | 灰色占位「暂无可用版本，请联系管理员」（下载口径为 200 注释块，Design2.md §4.4，用户侧以占位文案前置提示；**一键导入 / 复制链接 / 刷新链接三按钮隐藏**） |
| 有自定义订阅 | `a-alert info`「已被分配自定义订阅」+ 自定义操作按钮（沿用） |

**管理员——预览形态**（替代 Design1「池内订阅平铺 + 一键导入/复制链接」）：

| 区域 | 内容 |
|------|------|
| 订阅区段 | 该平台订阅条目的**模板信息**：订阅名称 + product_type 标签 + 内容形态标签（`a-tag`：装配模板紫 / 直接上传灰）+ 当前激活版本号与时间戳（无激活版本显示灰字「未激活」） |
| 操作 | 仅「**按平台预览当前版本**」按钮 → 预览弹窗（`a-modal` 宽屏纯文本，装配模板含占位标记原文、直接上传内容原样；无激活版本时按钮禁用 + Tooltip「暂无激活版本」）；**移除一键导入 / 复制链接 / 刷新链接**（显式 Token 不再新发，Design2.md 第一章） |
| 底部 | 「下载客户端」沿用 |

### 3.2 个人中心 /profile（改造）

沿用 Design1-UI §4.3 三页签结构，增量：

- **基本信息** `a-descriptions`：**基础模式隐藏「所属组」行**（高级模式展示）；新增「本月流量」行：基础模式显示「不限流量」；高级模式显示「已用 X GB / 配额 Y GB（或不限）」，超限时该行红色 + 「流量已超限，请联系管理员重置」附注
- 密码管理 / OIDC 绑定页签不变

### 3.3 用户端交互走查结论（编写后自检）

- 新注册用户登录 → 首页见流量卡（基础「不限流量」/ 高级 0 用量）→ 分流规则卡（空态或有值）→ 平台卡：无版本占位 / 有版本三按钮可用；超限用户下载按钮仍可见可用（超限不阻断下载）
- 管理员登录 → 首页平台卡为预览形态，预览按钮出口为纯文本弹窗；管理面板入口按钮沿用

---

## 四、管理面板改造页（管理员视角重点章）

> 本章仅规格化 Design2 产生变化的管理页；分享订阅（Design1-UI §5.3）、审批中心（§5.6）、日志查看（§5.9）经核验无 UI 影响，不重复。

### 4.1 订阅管理（改造）

- **列表页**：`PageHeader`（标题 + 「新建订阅」按钮）；**主体为平铺双态列表（不再按平台分组折叠**——每平台至多一份条目，分组结构退化为平台列）；列：平台名称、订阅名称、product_type 标签（`a-tag`：yaml 蓝 / subs 青 / generic-subs 紫）、内容形态标签（装配模板紫 / 直接上传灰）、当前版本（版本号 + 激活 `a-tag`；无激活版本灰字「未激活」）、操作（版本管理 / 编辑 / 删除）
- **新建订阅弹窗**：`a-modal` 表单（平台 `a-select` + 名称；标识后端自动生成）；**平台下拉中被占用的平台（已存在订阅条目）选项禁用 + 后缀「（已有订阅）」**；提交后后端 409 兜底 → `Notify.error` 展示冲突描述（UNIQUE(platform_id) 语义 UI 化，Design2.md §4.4）；**创建成功后不再弹「加入组可用范围 / 设置平台默认」引导**（组选定机制已删除），改为轻提示「可上传内容或前往订阅装配生成模板」
- **编辑弹窗**：仅名称修改（平台只读展示；product_type 只读展示）；**组关联多选移除**
- **「已入池未生效」引导口径**：上传 / 装配生成完成后，列表对应行临时高亮 + 行内 `a-alert info` 风格标签「已入池未生效，请激活」+「去激活」快捷链接（直达该订阅版本管理页，Design2.md §4.4 分发机制）
- **装配入口**：`PageHeader` 右侧「新建订阅」旁新增「前往装配」按钮（跳转 `/admin/assembly`）；版本管理页另有装配生成创建方式，见 4.2
- 删除订阅：ConfirmModal **逐项影响清单**（版本文件 / 指向该订阅的 Token；**装配蓝图级联删除将触发候选集重算，可能摘除受影响的组节点分配并移除对应 Xray 账号（高级模式下）**；**不级联自定义订阅**——自定义订阅随平台删除才级联；现状为单句文案，按本规格新写）

### 4.2 版本管理页 VersionManageView（改造，订阅 / 分享 / 规则 / 自定义四类同构页共用）

沿用 Design1-UI §5.1 双态列表、操作分布（预览 / 激活直显，编辑 / 删除进「更多 ▾」）、预览弹窗与删除约束，增量：

| 改造点 | 规格 |
|--------|------|
| 创建新版本第三方式 | 创建区由「文件上传 / 在线文本编辑」双页签扩展为三入口：页签不变 + 页签栏右侧「**装配生成**」按钮 → 跳转 `/admin/assembly?tab={对应装配器}&platform_id={订阅行平台}` 带目标参数进入装配器（SR 分流规则场景为规则版本页入口，tab=sr-conf 且带 rule_id）；**规则 / 分享 / 自定义三类版本页中，仅订阅池（yaml / subs / generic-subs）与规则页渲染该按钮**（分享与自定义无装配语义，不渲染） |
| 装配版本辨识 | 装配蓝图生成的版本行显示 `a-tag` 紫色「装配」（直接上传版本无此标签） |
| 重新编辑入口 | 「装配」标签版本的操作区显示「**重新编辑**」按钮 → 跳转 `/admin/assembly?tab={target_syntax}&edit_version_id={id}`（流程见 5.4）；直接上传版本不显示 |
| 「激活/分发」文案 | **订阅池（yaml / subs / generic-subs）与规则页**的版本行「设为当前」按钮文案改为「**激活/分发**」，确认弹窗文案同步为「激活后对全体用户生效」；**分享 / 自定义版本页保持「设为当前」不变**（Design2.md §4.4 版本组件改造适用范围边界） |
| 首次自动激活提示 | 版本列表无需特判（照常展示激活 `a-tag`）；**装配生成回执（5.3.0）与订阅版本上传成功提示**在 `auto_activated=true` 时显示「首个版本已自动激活」 |

### 4.3 用户组管理 GroupsView（重构，**仅高级模式可见**）

- **组列表**（双态列表）：组名（预置默认组带 `a-tag` 且无删除操作）、**默认配额**（GB 数值或「不限流量」）、分配节点数、组内用户数、操作（编辑 / 删除——默认组不可删）；**原「可用订阅数 / 重新设置」列与 needs_reselect 机制全部移除**（Design2.md §5.6 删除项）；首管理员「创建第一份订阅」引导条移除（订阅不再经组分发）
- **组编辑弹窗**（720px，`a-modal`）自上而下四区：
  1. **改名**（唯一校验，沿用）
  2. **节点分配**：`a-checkbox-group` 多选，候选集 = 全部已激活 clash-yaml / sr-subs / generic-subs 装配蓝图的 xray 候选节点并集（Design2.md §5.6 候选集口径 ①）；候选节点行展示节点有效渲染名（display_name 非空则用之，否则系统名；有自定义名时副行 `code` 风格展示系统标识名）+ 可用性标注：
     - **公共节点**（is_public=1）：行内 `a-tag`「公共 · 免分配」置灰不可勾选 + 副说明「公共节点对所有组自动可见」
     - **非候选集的已分配节点**：以红色警示标注「不在任何已激活模板候选集内，下载不会注入」（参考 Design1 needs_reselect 高亮模式）
     - **仅部分模板候选**：橙色提示「仅部分激活模板包含该节点」
     - **候选集并集为空**：分配区整体替换为 `a-empty` 引导态「请先装配并激活 Clash YAML / SR 节点订阅 / 通用节点订阅模板」+「前往装配」按钮（Design2.md §5.6 口径 ⑤）
  3. **分配排序**：已勾选节点以有序列表展示（拖拽排序，<768 上移/下移降级，见 1.2）；顺序即下载渲染输出顺序
  4. **默认配额**：`a-input-number`（GB，≥0）+ 副说明「0 或留空 = 不限流量」
- **保存**：提交后后端执行受影响用户 diff 推送/移除（后端行为）；前端 `Notify.success`「已保存，节点变更将同步至 Xray」
- **删除组**：ConfirmModal「组内 N 名用户将自动迁入默认组，其 Xray 账号随新组自动迁移」
- `<768`：编辑弹窗四区纵向堆叠，勾选列表全宽

### 4.4 平台管理 PlatformsView / PlatformEditView（改造）

- **列表**：双态列表新增 **product_type 列**（`a-tag`：yaml 蓝 / subs 青 / generic-subs 紫）；其余（名称 / 标识 / 安装包状态 / 操作）沿用 Design1-UI §5.4
- **新建平台**：表单新增 **product_type `a-radio-group`**（yaml「Clash YAML 订阅」/ subs「Shadowrocket 节点订阅」/ generic-subs「通用节点订阅（v2rayNG/v2rayN 等）」，**默认 yaml**）；新建成功后引导文案更新为「可前往订阅管理为平台创建订阅条目，或前往订阅装配生成模板」（原「为各用户组设置默认订阅」引导移除）
- **平台编辑页**：product_type 以 `a-radio-group` **可校正**展示；**校正提交时若与既有订阅条目 product_type 不一致 → 后端 400 拒绝，前端表单级错误提示「该平台已有 {yaml|subs|generic-subs} 订阅条目，请先处理后再变更产物格式」**；无订阅条目的平台可自由校正
- 删除平台 ConfirmModal 影响清单沿用

### 4.5 用户管理 UsersView（扩展）

沿用 Design1-UI §5.5 双态列表 + 后端分页 + 搜索 + 操作 Dropdown 全部既有项，增量：

| 改造点 | 规格 |
|--------|------|
| **基础模式隐藏「所属组」列** | 两模式布局一致口径：基础模式列表不渲染所属组列（卡片态同步），编辑弹窗换组 Select 同步隐藏 |
| 高级模式新增列：用量 | 「本月用量」列：`X / Y GB`（Y 为有效配额，不限时显示 `X GB / 不限`；无任何流量记录时显示 `—`）；字节→GB 两位小数（见 10.1） |
| 高级模式新增列：Xray 同步状态 | 聚合 Badge（色系见 1.2）：全部 synced → 绿「已同步」；存在 pending → 橙「同步中」；存在 failed → 红「N 条失败」+ Tooltip 展示 last_error 摘要 + 行内「**重试**」按钮（调重试端点，loading 态防重复）；无任何推送记录（如未激活 / 无候选节点）→ 灰「未推送」 |
| 操作 Dropdown 新增：**配额覆盖** | 弹窗：`a-input-number`（GB）+ 说明「留空 = 继承所属组默认配额；0 = 不限流量」；当前生效值（继承组配额或覆盖值）回显 |
| 操作 Dropdown 新增：**重置配额** | ConfirmModal「将清空该用户本月流量累计并恢复代理账号（凭据不变）」；执行后 `Notify.success` 展示重置结果；禁用用户后端拒绝并提示（前端按钮置灰 + Tooltip「仅激活用户可重置」） |
| 超限用户行高亮 | `quota_exceeded=1` 的行警示底色（同 Design1 needs_reselect 高亮模式）+ 用户名后红色 `a-tag`「已超限」+ 操作引导「重置配额」（Dropdown 项前置提示；管理端处置指引：重置配额后自动恢复推送） |
| 批量操作头部按钮 | 沿用（无密码用户发送设置链接），不变 |

### 4.6 规则管理 RulesView（改造）

沿用 Design1-UI §5.7 双态列表与 Token 操作，增量：

- **空规则实体展示**：允许创建无版本的规则实体（放宽「创建必带首版」校验，Design2.md §3.4）——创建弹窗首版本上传区改为**可选**（页签 + 「暂不创建版本」说明）；列表对无激活版本的实体展示灰色「无激活版本」标签（不再必有版本 Tag）
- **「首页默认展示」设置**：列表新增单选列 —— 每行 `a-radio`（至多一条 is_home_default=1）；**切换时 ConfirmModal「设为首页默认后，原默认规则『{旧规则名}』将自动取消默认」**，确认后调设置端点（后端事务内清旧置新）；**取消默认：默认行专设「取消默认」操作按钮（仅默认行显示，不使用再次点击已选中行的交互），ConfirmModal「取消后首页分流规则卡片将回到未设置空态」确认后调设置端点置 false，回到未设置空态**
- **装配目标引导**：空实体行 Tooltip / 副文案「可作为 SR 分流规则装配目标」（装配器步骤六可选空实体，见 5.3.4）
- 版本管理页复用 4.2 改造（含装配生成入口与「激活/分发」文案）

### 4.7 面板设置 SettingsView（扩展）

沿用 Design1-UI §5.8 页面骨架（左侧锚点 + 右侧分区卡片）与全部分区，新增与修订：

#### 4.7.1 新增「高级模式」分区（锚点项，置于「运行模式信息」之后）

| 控件 | 规格 |
|------|------|
| 高级模式开关 | `a-switch`；当前状态旁 `a-tag`（已开启绿 / 未开启灰）+ 分区顶部说明文案「解锁 Xray 实例对接、多用户组与流量配额管控」 |
| **开启（OFF→ON）** | 直接开关 + 保存提示「开启后请前往 Xray 实例页录入实例并完成初始化配置；开启本身不执行任何推送」（保存即生效，刷新系统状态后侧边栏解锁「用户组 / Xray 实例」入口）；**已开启状态重复保存为幂等 no-op（仅返回当前状态，不建任务）** |
| **关闭（ON→OFF，清空）** | 触发**如同清空数据的二次输入确认弹窗**（ConfirmModal 内嵌确认词输入框，**确认词固定 `DISABLE`**，**仅 ON→OFF 状态翻转时要求；已 OFF 再保存为幂等 no-op，不要求确认词、不建任务**）：弹窗内**清单式展示将被移除的内容**——Xray 实例数据、**source=xray 的节点（标注：manual 节点保留）**、组节点分配、用户 Xray 推送记录、**独立 Xray 账号（删除整行含凭据与推送记录）**、流量记录、用户 UUID 与代理密码、配额字段；显著 `a-alert warning` 两条：①「Xray 侧不可达实例的账号需手动清理」②「重新开启须全量重新配置并手动执行『开始初始化』」（Design2.md 第一章 OFF 清空口径）；**提交后返回 `task_id` 进入异步任务轮询（见 4.7.2/9.2）** |
| 流量采集间隔 | `a-input-number`（分钟，默认 10，≥1）+ 说明「逐用户串行采集 Xray 流量，间隔过短会增加实例压力」（仅高级模式展示该控件；API 字段 `collect_interval_minutes`，后端映射配置键 `xray_collect_interval_minutes`） |
| 流量卡片显示开关 | `a-switch`（默认开启）+ 说明「控制首页流量卡片与个人中心「本月流量」行的展示」（两模式均展示该控件；配置键 `traffic_card_enabled`，经 `/api/system/status` 同名字段暴露） |

#### 4.7.2 配置导入/导出分区文案更新（沿用分区，仅文案与提示增量）

- 导出说明追加：「含 Xray 实例清单（含节点显示名映射）与独立账号（含推送目标与超限标记，format_version=2）」
- **导入确认弹窗追加影响项**：「Xray 实例将整体覆盖（slug 沿用），**组节点分配将被级联清空，导入后需重新分配**」；**若导入文件带 Xray 实例/独立账号且高级模式为关，系统将自动开启高级模式并在完成提示中显著说明**；**若导入文件不含实例/独立账号且高级模式为关，将按 OFF 清空口径清理旧高级数据——采用双确认词分步：导入弹窗先按既有流程校验 `IMPORT`，命中该破坏分支时再追加第二个确认词 `DISABLE`（与 4.7.1 关闭高级同口径，Design2Report10 Q5）**
- **signing_key 保护拒绝提示**：后端拒绝导入时（密钥将变化且存在业务密文），前端 `a-alert error` 展示后端文案「配置导入仅适用全新部署/同密钥往返，在用实例请使用备份恢复」，不做任何变更
- **导入/关闭高级异步化**：**v1 导入保持同步响应（`{message}`，沿用现有交互）；v2 导入提交后返回 `task_id`**，前端按 pollTask 轮询全局任务端点 `GET /api/admin/tasks/:id`（见 9.2）；关闭高级提交返回 `task_id` 同上轮询；完成提示追加「系统将自动执行节点检测刷新（enabled=0 实例跳过并提示）、按 (实例, inbound tag) 回填节点显示名（未匹配映射提示）、独立账号推送目标重绑（未匹配目标标记失败）与账号对账」（Design2Report10 Q5）

### 4.8 管理面板交互走查结论（编写后自检）

- 新部署动线：建平台（选 product_type）→ 订阅管理新建条目（占用平台拒绝复建）→ 装配生成入池 → 行内「已入池未生效」引导 → 版本页「激活/分发」→ 用户侧可见
- 高级动线：设置开高级模式 → 侧边栏解锁两入口 → 录实例 / 检测 → 组分配（候选集引导）→ Xray 页「开始初始化」→ 用户页见同步状态
- 八项隐形漏洞核对：各列表空态见 10.2；加载态 TriStateList；错误态重试；全部新表格均含 <768 卡片态（本章 4.1/4.3/4.4/4.5/4.6）；暗色随 ConfigProvider；权限可见性（组页 / Xray 页 / 高级列均 advanced_mode 驱动）；防重复提交（开关保存 / 重试 / 重置配额按钮 loading 禁用）；危险确认（OFF 清空确认词 / 删组 / 重置配额 ConfirmModal）

---

## 五、订阅装配页 AssemblyView（占位页 → 实现，本期最大新增）

### 5.1 页面骨架：单菜单入口页内 Tabs

- `PageHeader`（标题「订阅装配」）+ `a-tabs` **三个一级页签**：**规则素材池 / 代理组 / 构建订阅·规则**（页签 key：`pool` / `proxy-groups` / `build`）；其中「构建订阅·规则」内部再以二级 Tab 提供四个子平台：**Clash YAML / SR 节点订阅 / 通用节点订阅 / SR 分流规则**（key：`clash-yaml` / `sr-subs` / `generic-subs` / `sr-conf`）；页签切换不重新拉取无关数据（各页签独立挂载，`keep-alive` 或惰性渲染二选一由实现决定，对外行为一致）
- URL query 驱动页签：`?tab=` 无效值回退首页签；`?tab=clash-yaml` 等会定位到「构建订阅·规则」对应子平台；四个子平台接受 `platform_id` / `rule_id` / `edit_version_id` 带参进入（见 2.3/4.2）
- `<768`：页签转横向滚动（AntD 默认行为），页内分区纵向堆叠

### 5.2 页签一：规则素材池

#### 5.2.1 池列表（双态列表）

列：池名称、挂接 URL 数、条目数、上次同步时间（本地时区）、同步状态 Badge（色系见 1.2，failed 附原因 Tooltip；**数据源为 listPools 返回的 sync_status/sync_error 字段（最近任务完成后的快照，与 pool_sync_tasks 最近任务等价；服务重启中断任务显示 failed）**）、定时同步（`a-switch` 行内开关 + 时刻展示「每日 04:00 UTC」）、操作（详情 / 同步 / 编辑 / 删除）。**行内「同步」按钮**（loading 态防重复，见 5.2.3 轮询流）；定时开关行内切换即时保存，时刻编辑在池编辑弹窗内（`a-time-picker`，默认 04:00，副说明「按 UTC 每日执行，停机错过不补跑」）。

- **新建/编辑池弹窗**（480px）：名称（重名 409 提示）+ URL 动态列表（`a-form-list`，http/https 校验，逐行增删）+ 定时开关与时刻
- **删除池**：ConfirmModal「池内 N 条条目将级联删除；已装配版本为快照不受影响」
- 空态：「还没有规则素材池」+「新建素材池」按钮（见 10.2）

#### 5.2.2 池详情（点击池名/详情进入，页内钻取视图，面包屑「素材池 / {池名}」可返回）

- **顶部信息条**：池名 + URL 数 + 条目总数 + 上次同步时间 + 状态 Badge + 「同步」「编辑」「返回」按钮
- **条目列表**（双态列表 + **后端分页，默认 20 条/页**——数万行规模不整表加载，Design2.md §2.4）：规则类型 Tag（代码风格 `a-tag`）、匹配值（`code` 样式）、来源标签（**manual / url 分段展示**：列表默认混合按渲染顺序展示，段间以分隔标题行「手动条目（前段）/ URL 同步条目（后段）」区分，见下）
- **两段拼接排序口径**（Design2.md §2.2）：条目按渲染顺序 = manual 段（前）+ url 段（后）展示；**条目顺序由系统维护**（manual 段按创建先后、url 段按同步首次出现顺序），**不提供条目级手动调序**（数万行分页场景不可操作；池间顺序在装配第④步调整，池内规则顺序需求由池级勾选目标与手动补充规则行承载）
- **手动条目增删改**：「新增条目」按钮 → 弹窗（规则类型 `a-select` 全类型含 USER-AGENT + 匹配值输入，白名单格式实时校验）；**去重冲突**：后端 409 → `Notify.error`「同类型同匹配值条目已存在（含 URL 来源）」；行内编辑/删除（删除单条无需确认弹窗，非危险）
- **同步历史列表**（页内分区或弹窗）：调 `listSyncTasks` 分页展示最近 N 条历史任务（状态 Badge / 开始与结束时间 / 逐 URL 明细摘要 / 错误 Tooltip），服务重启中断任务显示 failed「服务重启，任务中断」（Design2 §5.9「历史任务保留供 UI 展示」）

#### 5.2.3 同步轮询状态 UI（pollTask 契约见 9.2）

1. 点击「同步」→ 提交同步任务 → 按钮转 loading + 池行/详情顶部展示「同步中…」内联 Spinner
2. 轮询状态端点（1.5s）；**页面切走/组件卸载自动取消轮询**（后端任务继续执行，再次进入重新拉状态即可）；**任务持久化于 pool_sync_tasks，服务重启时 running 任务置 failed，UI 展示「服务重启，任务中断」**
3. 终态后展示**逐 URL 结果回执**（`a-alert` 列表，一 URL 一行）：成功（绿 success，含条目变化摘要 added/removed/skipped）/ 失败（红 error，展示失败原因：拉取失败 / 空响应 / 零条目保护「响应无有效规则条目，已保留旧数据」/ 解析跳过行数）/ 部分失败（橙 warning 总提示 + 逐行明细，**部分失败时不执行差量删除**属后端行为，UI 仅展示原因）
4. **进行中再触发**：同步未完成时再点「同步」→ `message.warning`「同步进行中，请等待完成」（后端同池不并发，Design2.md §2.4）
5. 轮询超时兜底文案见 10.1

### 5.3 四类装配器共用框架与各自规格

#### 5.3.0 共用框架

- **双形态切换**（见 1.2）：默认步骤条形态；`a-segmented`「分步 / 单页」切换，localStorage 记忆
- **步骤条形态**：`a-steps` 六步（Design2.md §4.1 生成流程）：① 类型与目标 → ② 头部表单 → ③ 节点与代理组 → ④ 规则素材 → ⑤ 预览 → ⑥ 确认生成；步间「上一步/下一步」，已填步骤可点击回跳，数据不丢失；**SR 节点订阅与通用节点订阅跳过④，SR 分流规则跳过③**（步骤条同步隐藏跳过的步骤：前者呈现五步，后者呈现五步）
- **单页形态**：全部步骤转为纵向分区卡片（`a-card` + 锚点侧栏或顺序滚动），底部固定操作条「预览产物」直达预览区；**两形态共享同一份表单状态与校验**
- **步骤①类型与目标**：装配类型由所在页签确定（只读展示）；目标选择：Clash YAML / SR 节点订阅 / 通用节点订阅 → 目标平台 `a-select`（**仅列出 product_type 匹配的 yaml / subs / generic-subs 平台**，无匹配平台时空态提示「请先创建对应格式的平台」+ 直达平台管理链接；**平台存在但尚未创建订阅条目时，该平台选项后缀「（无订阅条目）」并禁止生成，提示先到订阅管理创建订阅条目**）；SR 分流规则 → 目标规则实体选择（见 5.3.4）
- **生成严格校验（前端预检 + 后端兜底）**：悬空代理组引用、**勾选组引用的子组不在本次输出集合（强制组或已勾选组）**、规则目标指向未勾选代理组一律拒绝生成并定位提示（Design2Report10 Q7）；**勾选到已停用预设组（enabled=0）同样拒绝生成（400「预设组已停用，请先启用或移除勾选」），不纳入 5.4 失效项剔除容错（Design2Report11 决策）**；不可用 xray 节点（enabled=0 / allocatable=0 / missing=1 / 实例 enabled=0）前端置灰不可勾选，后端拒绝；**强制组「🚀直接连接 / 🌎国外流量 / 🛟无法归属的流量」允许作为 Clash 规则目标**
- **防重复提交**：生成按钮提交期间 loading 禁用；成功后页内显著 `a-result success` 风格回执：「已入池未生效，请激活」+「去版本管理激活」/「继续装配」两按钮（Design2.md §4.4 引导口径）；首次入池自动激活时后端回执带激活标记，UI 改示「首个版本已自动激活」（订阅行/规则实体无激活版本的例外条款，Design2.md §4.4）

#### 5.3.1 Clash YAML 装配器

| 步骤 | 规格 |
|------|------|
| ② 头部表单 | Clash 顶层键表单（字段范围见 Design2.md §4.2），**默认值以作者个人配置头部预填** + 「一键采用默认值」按钮（覆盖已改字段前 ConfirmModal「将覆盖当前已填头部」）；长表单折叠分区（基础键 / DNS / 其他） |
| ③ 节点勾选 | 双来源分组列表：manual 区（含协议 Tag）与 xray 区（高级模式且有检测节点时展示；基础模式/无实例时 xray 区空态文案「未检测到 Xray 节点（高级模式录入实例后手动刷新节点发现）」）；xray 区节点行展示有效渲染名（display_name 非空则用之，否则系统名；有自定义名时副行 `code` 风格展示系统标识名）；**allocatable=0 节点行置灰不可勾 + Tooltip「该协议无 per-user 能力，不可分配」**；missing=1 节点不列入候选 |
| ③ 代理组配置 | 三区块：强制组（🚀直接连接 / 🌎国外流量 / 🛟无法归属的流量）锁定勾选态（Checkbox 选中禁用 + 锁图标）；预设组库 `a-checkbox-group`（内置参考组，含组类型标注）；自建组列表（引用代理组管理页已建组，勾选 + 「前往代理组管理」链接）；**「🌎国外流量」组成员配置区**：为其**仅勾选节点成员**（Design2Report10 Q8；空组硬校验前置引导；**成员配置随装配快照保存**，重新编辑自快照恢复，见 Design2.md §3.3 强制组落库口径） |
| ④ 规则素材 | **已勾选池为有序列表**（池名 + 条目数 + 每池目标代理组 `a-select`；桌面端拖拽排序，<768 上移/下移，顺序随装配快照保存并供重新编辑恢复）+ 手动补充规则行（动态行：类型 + 匹配值 + 目标组，格式实时校验）；副说明「多个池按此顺序依次拼接；USER-AGENT 条目将被跳过（Clash 不支持）」 |
| ⑤ 预览 | 见 5.3.5 |
| ⑥ 生成 | **空组硬校验**（前端预校验 + 后端兜底）：强制组（尤其「🌎国外流量」）成员为空时拒绝进入生成，`a-alert error` 定位提示「『🌎国外流量』组未包含任何节点，空组将导致 Clash 加载失败」；规则为空允许生成但预览区提示「未勾选任何规则，仅含兜底规则」 |

#### 5.3.2 SR 节点订阅装配器

- ② 头部表单：STATUS（预填建议格式：日期 + 版本标识，可改）+ REMARKS（预填站点名）+ 「一键采用默认值」
- ③ 节点勾选：同 5.3.1 双来源分组列表（无代理组步骤）
- ④ 跳过（subs 不含规则）
- ⑥ **空产物校验**：未勾选任何节点或**跳过不可转协议后有效链接数为 0** 时拒绝生成（提示「请至少勾选 1 个可转换节点」）；**不可转协议跳过提示**在预览/生成回执中列出（如 snell/openvpn 等，见 Design2.md §4.5）

#### 5.3.2a 通用节点订阅装配器

- ② 头部表单：**无**（generic-subs 不输出 STATUS/REMARKS 头部，Design2.md §4.5）
- ③ 节点勾选：同 5.3.1 双来源分组列表（无代理组步骤）
- ④ 跳过（generic-subs 不含规则）
- ⑥ **空产物校验**：与 5.3.2 同口径（至少 1 个可转换节点）

#### 5.3.3 SR 分流规则装配器

- ② 头部表单：`[General]` 键值表单（bypass-system / skip-proxy / dns-server 等，预填默认值可一键采用）
- ③ 跳过（conf 不含节点）
- ④ 规则素材：**已勾选池为有序列表**（桌面端拖拽排序，<768 上移/下移，顺序随装配快照保存并供重新编辑恢复）+ **每池 PROXY / DIRECT 双态切换**（`a-radio-group` 行内，默认 PROXY）+ 手动规则行（动态行：类型含 USER-AGENT + 匹配值 + PROXY/DIRECT）
- **兜底 FINAL 方向**：表单区 `a-radio-group` 二选一（FINAL,PROXY 默认 / FINAL,DIRECT），副说明「GEOIP,CN,DIRECT 固定追加」（Design2.md §3.6）
- ① 类型与目标步骤内选目标规则实体（见 5.3.4）；规则为空允许生成（仅兜底）并提示

#### 5.3.4 目标规则实体选择（SR 分流规则专属，固定于步骤①）

- `a-select` 列出全部规则实体（含无版本空实体，后缀「（空实体）」标注）+ 行内「新建空规则实体」快捷入口（弹窗仅名称 + 客户端类型固定 Shadowrocket，放宽首版校验，Design2.md §3.4）
- 选中空实体时提示「该规则无激活版本，首个装配版本将自动激活」（Design2.md §4.4 首次自动激活）

#### 5.3.5 预览步（⑤，四类共用）

- **全文纯文本预览**（等宽字体只读容器，`a-typography-paragraph code` 风格，禁 HTML，Design2.md §3.5）；SR subs / generic-subs 预览直接显示明文原文（base64 为下发编码，非存储形态）
- **diff 对比**：提供「与当前激活版本对比」开关 → 前端复用既有版本预览端点拉取当前激活版本原文，切换为 `DiffView` 三色行级高亮（见 1.2）；**目标实体无激活版本时开关仍可用，DiffView 以空目标为基准、将预览全文渲染为「整体新增」并注「目标尚无激活版本，本次对比为整体新增」**（Design2Report10 Q4）；**对比基准为目标实体当前激活版本（订阅与规则实体均适用）**
- **跳过项提示清单**：预览区顶部 `a-alert warning`（有跳过项才渲染）：Clash 装配的 USER-AGENT 条目清单 / SR subs 与 generic-subs 的不可转协议节点清单（逐项列出名称与原因）
- **占位标记说明**：勾选了 xray 节点的装配产物预览中可见 `# {{xray_nodes}}` 注释行，旁注 Tooltip「下载时按用户分配节点动态注入」
- **显示名变更提示**：后端 preview/getBlueprint 返回 `name_changed` 对照信息（快照名 → 当前有效渲染名）；存在变更时预览原文旁注 Tooltip「存储原文为生成时快照，用户下载按当前显示名实时渲染」（Design2.md §5.7）

### 5.4 重新编辑流（从版本入口载入快照）

1. 从版本管理页「重新编辑」进入（`?tab={target_syntax}&edit_version_id={id}`）→ 顶部 `a-alert info`「正在重新编辑版本 vN，保存将生成新版本」
2. 载入快照回填全部表单（头部 / 节点与组勾选 / 池勾选与目标 / 手动规则行 / FINAL 方向）
3. **悬空引用容错**（Design2.md §4.4 快照悬空容错）：载入时逐项校验引用（proxy_groups / 素材池 / 节点）——失效项以**红色 `a-tag`「已失效」**标记于原位置 + 页面顶部汇总 `a-alert warning`「N 项引用已失效，请剔除或替换后生成」；失效项不可参与生成（前端预校验拒绝，后端兜底）；剔除操作：逐项「剔除」按钮或顶部「一键剔除全部失效项」
4. 修改后走正常预览与生成流程 → 新版本入池（同 5.3.0 回执与引导）；**旧版本不改写**

### 5.5 装配页自检结论（编写后）

- 功能匹配：素材池（Design2 第二章）/ 四装配器六步（§4.1，SR subs 与 generic-subs 跳过④、SR conf 跳过③）/ 头部表单（§4.2）/ 节点双来源（§3.2）/ 强制组与预设库（§3.3）/ SR 双装配器与通用装配器（§3.4）/ 规则拼接与 USER-AGENT（§3.5）/ 兜底 FINAL（§3.6）/ diff jsdiff（§4.1）/ 入池引导与重新编辑（§4.4）均有着落
- 动线：新建池 → 同步轮询回执 → 手动条目 → Clash 装配六步 → 预览 diff → 生成回执 → 去激活；重新编辑 → 悬空剔除 → 生成新版本
- 八项：空态（无池/无条目/无匹配平台/无候选节点）；加载（列表 TriStateList、同步 Spinner、快照载入 Spin）；错误（同步失败回执、409 重名、生成校验）；<768（页签滚动、条目卡片态）；暗色随主题；权限（整页仅管理员路由）；防重复（同步/生成按钮 loading、进行中再触发提示）；危险确认（删池 ConfirmModal、头部覆盖确认）

---

## 六、节点管理页 NodesView（新独立菜单 `/admin/nodes`）

### 6.1 列表（双态列表）

列：节点名称（**有效渲染名**：manual = name；xray = display_name 非空则 display_name，否则系统名；xray 有自定义名时双行展示，副行系统标识名 `code` 风格）、来源标签（`a-tag`：manual 灰 / xray 紫）、协议（代码风格 Tag）、地址:端口、公共节点标记（is_public=1 时 `a-tag`「公共」）、启用开关（`a-switch` 行内）、allocatable 标注（xray 非四协议节点橙色 `a-tag`「不可分配」+ Tooltip「该协议无 per-user 能力」）、missing 标注（xray 侧已删节点灰色 `a-tag`「实例侧已删除」+ Tooltip「待管理员处置：删除或等待重检测恢复」）、操作（编辑（manual）/ 命名（xray）/ 删除）。

- **行内 enabled 开关**：切换即保存（后端触发受影响用户 AddUser/RemoveUser diff，属后端行为）；**1→0 停用切换前 ConfirmModal「停用该节点将移除受影响用户的 Xray 账号（重新启用后需重新分配）」，0→1 无需确认**；切换中开关 loading 防重复；失败回滚 + `Notify.error`
- **is_public 开关**（仅 source=xray 且 allocatable=1 / missing=0 的行渲染，其余不展示该控件）：切换前 ConfirmModal「公共节点变更将对全部活跃用户同步增删 Xray 账号」
- **删除节点**：**xray 来源节点仅 missing=1 的行可删除**；非 missing 行删除按钮禁用 + Tooltip「该入站仍存在于 Xray 实例，请先删除 Xray 入站并刷新节点检测」；ConfirmModal 影响说明（xray 节点：组分配与推送记录级联清理；manual 节点：影响装配快照与代理组定义中的节点引用，重新编辑/代理组编辑时按悬空容错处理）；危险样式
- 空态：「还没有节点」+ 引导文案「手动添加节点，或在高级模式录入 Xray 实例后自动检测」（见 10.2）
- 筛选工具栏（可选）：来源筛选 `a-segmented`（全部 / manual / xray）——节点量增长后的易用性补充，不强制

### 6.2 manual 节点新增/编辑弹窗（720px）

- **协议选择**：`a-select` 协议注册表清单（由 `GET /api/admin/nodes/protocols` 下发：ClashOfficial 全量代理协议，**ssr 除外**，Design2.md §4.5）；选择后表单字段按协议注册表 schema **动态渲染**（基础字段：名称 / host / port；协议特有字段按注册表展开，敏感字段按注册表标记渲染为 `a-input-password`）；**编辑时允许变更协议：变更等价整体重新填表、不保留不兼容旧字段**（凭据字段仍按「留空=保留原凭据」口径，Design2Report11 决策）
- **凭据字段**（uuid / password / private-key 等注册表敏感清单）：统一 `a-input-password` 脱敏输入；**编辑回显时凭据字段留空，placeholder 提示「留空 = 保留原凭据」**（Design2.md §3.2 编辑回显口径）
- **名称规则**：**创建后不可修改**（编辑弹窗名称只读；后端拒绝改名）；创建时实时校验——禁止控制字符、逗号、空格与首尾空白，允许中文/emoji；重名冲突（与其他节点有效渲染名、proxy_groups.name、强制组名「🚀直接连接 / 🌎国外流量 / 🛟无法归属的流量」或 Clash/mihomo 内建保留代理名「DIRECT / REJECT / REJECT-DROP / PASS / COMPATIBLE」重复）后端 409 → `Notify.error`「节点名称已存在或与代理组/保留名冲突」
- 表单校验：host/port 格式实时校验；提交失败字段级回显

### 6.3 xray 节点展示与命名

- xray 节点除显示名（display_name）外不支持整体编辑（字段由实例检测维护）：操作列无「编辑」按钮，仅「命名」+ 行内 enabled / is_public 两开关（约束见 6.1）+ 删除；**删除按钮仅 missing=1 行可点**（非 missing 行置灰 + Tooltip「该入站仍存在于 Xray 实例，请先删除 Xray 入站并刷新节点检测」）；行内 Tooltip 说明「节点信息由 Xray 实例检测维护，显示名可自定义；系统标识名仅作内部引用，不可修改」
- **命名弹窗（420px）**：
  - 单输入「订阅显示名」（placeholder 显示当前系统名；留空保存 = 清空并恢复系统名）；实时校验同 6.2 名称规则；重名冲突（与其他节点有效渲染名、proxy_groups.name、强制组名「🚀直接连接 / 🌎国外流量 / 🛟无法归属的流量」或 Clash/mihomo 内建保留代理名重复）后端 409 → 字段级 `Notify.error`「显示名已存在或与代理组/保留名冲突」
  - 说明文案：「显示名将用于 Clash `name:`、SR `remarks` 与通用订阅节点名；修改后立即影响所有已激活模板的下载内容，系统内部引用不变；**用户端刷新订阅后可能需重新选择节点**」
  - 保存成功 `Notify.success`「显示名已更新」；保存中按钮 loading 防重复
- missing=1 节点提供「删除」作为处置手段之一（重检测恢复为另一途径，在实例页，见 8.2）；**非 missing 的 xray 节点禁止删除，避免删除后又被检测重建**；missing 节点同样可命名，重检测恢复后显示名保留

### 6.4 自检结论（编写后）

- 功能匹配：双来源展示与约束（Design2.md §3.2）、**manual 名称创建后只读、xray display_name 可编辑与有效渲染名唯一/跨命名空间校验、协议注册表端点**、命名与重名 409（§3.2）、allocatable/missing 标注与处置（§3.2/5.9）、**xray 节点仅 missing 可删**、enabled/is_public 切换同步语义（§5.5 触发器表）、凭据加密回显口径（§3.2/5.9）均有着落
- 八项：空态/加载（TriStateList）/错误（开关失败回滚、409）/<768 卡片态/暗色/权限（管理员路由；manual 能力基础模式可用）/防重复（开关 loading）/危险确认（is_public 切换、删除 ConfirmModal）

---

## 七、代理组管理（订阅装配内 Tab：`/admin/assembly?tab=proxy-groups`）

### 7.1 列表（双态列表）

列：组名称、类型 Tag（`a-tag`：preset 蓝「预设」/ custom 绿「自建」）、组类型（select / url-test / fallback 代码风格 Tag）、成员摘要（N 个节点 · M 个子组）、启用勾选（仅预设组，见下）、操作（编辑 / 删除——预设组不可删；**全部组名创建后不可改**，自建组编辑弹窗名称只读）。

- **预设组启用勾选**：预设库组以行首 Checkbox 启用/停用（启用后才在装配器预设组库中可勾选）；切换即时保存（持久化于 proxy_groups.enabled，预设组默认启用，见 Design2.md §5.9）；**种子成员默认为「🚀直接连接」**（成员摘要至少 1 项），管理员可编辑成员
- **自建组删除**：ConfirmModal「已被装配快照引用的记录不受影响（历史快照悬空容错）；被其他代理组引用为子组时，这些组将产生悬空引用，编辑页按红标剔除处理」
- 空态：预设组种子随迁移内置，列表至少含预设组；自建组为空的理论态不单独处理（见 10.2）

### 7.2 自建组创建/编辑弹窗（720px）

自上而下四区：

1. **基本信息**：名称（**创建后不可修改**：新建可输入、编辑只读，后端拒绝改名；重名 409 提示——重名范围含其他组名、任一节点有效渲染名、强制组名与 Clash/mihomo 内建保留代理名；禁止控制字符、逗号与首尾空白，允许中文/emoji）+ 组类型 `a-radio-group`（select 默认 / url-test / fallback，附各类型一句话说明；**名称只读、组类型创建后可修改**；预设组编辑时名称/preset_key 只读，组类型同样可改）
2. **节点引用**（有序列表）：从全部节点中选入（manual + 可用 xray 节点，搜索过滤）；xray 节点展示有效渲染名（display_name 非空则用之，否则系统名），有自定义名时副行 `code` 风格展示系统标识名；已选节点有序列表支持**拖拽排序**（<768 上移/下移降级）；列表副说明「select 类组的第一个节点即默认选中节点；节点引用按系统标识名持久化，显示名变更不影响引用」（Design2.md §3.3/§5.4）
3. **子组引用**：多选已存在的其他代理组（含强制组「🚀直接连接」「🌎国外流量」作为可切换项）；同样有序列表
4. **校验与保存**：
   - **DAG 环形引用即时校验**：选择子组时前端即时检测成环 → 拒绝选择并 `Notify.error`「检测到环形引用：A → B → A」（保存时后端兜底校验）
   - **组内容约束**：保存时校验至少含节点 / 「🚀直接连接」组 / 「🌎国外流量」组三者之一，否则表单级错误提示（Design2.md §3.3）
   - 悬空引用（节点被删）：载入编辑时对失效节点引用红色标记，同 5.4 剔除模式处理

### 7.3 自检结论（编写后）

- 功能匹配：强制组/预设库/自建组三类（Design2.md §3.3）、组类型三枚举、有序节点引用与 select 首项语义、DAG 校验、组内容约束、**名称创建后只读与字符集校验**均有着落；预设组种子默认成员「🚀直接连接」；强制组不在本页管理（系统内置渲染结构、不入 proxy_groups 表，仅装配器锁定展示，见 Design2.md §3.3 强制组落库口径）
- 八项：空态/加载/错误（环形校验、约束校验、409）/<768 卡片态/暗色/权限（管理员）/防重复（保存 loading）/危险确认（删组 ConfirmModal）

---

## 八、Xray 实例页 XrayInstancesView（新独立菜单 `/admin/xray`，仅高级模式）

> 本页全部能力属高级模式（Design2.md 第五章）；advanced_mode=off 时菜单隐藏 + 路由守卫重定向（见 2.4）。

**页面骨架**：`a-tabs` 双页签 —— **Xray 实例**（8.1~8.4）/ **独立账号**（8.5，Design2.md §5.11）；`PageHeader` 右侧「开始初始化」按钮为页面级常驻（仅作用于面板用户，与独立账号无关）。

### 8.1 实例列表（双态列表）

列：实例名称、slug（`code` 风格只读）、api_addr（`code` 风格）、enabled 开关（行内，切换即时保存；停用提示「暂停管理：不参与检测/推送/采集/注入，既有账号保留」Tooltip，Design2.md §5.9）、采集状态（最近成功时间 + 状态 Badge：成功绿 / 失败红 + 连续失败告警 `a-badge` 红色闪烁样式 + 原因 Tooltip，Design2.md §5.8 告警口径）、操作（编辑 / 刷新节点 / 对账 / 删除）。

- **实例新增/编辑弹窗**（480px）：名称（重名 409）+ api_addr（TCP 地址格式校验）+ api_tag + 「**测试连接**」按钮（loading → 结果 `a-alert`：成功 success / 失败 error 展示 gRPC 错误摘要）；测试不落库；保存前建议测试但不强制；**编辑保存成功后提示「已保存，建议执行『刷新节点』以同步 api_addr 变化后的节点信息」**（Design2Report10 Q12-7）
- **删除实例**：ConfirmModal 危险样式，影响清单：xray 来源节点级联删除、组分配级联清理、推送记录清理；`a-alert warning`「实例不可达时 Xray 侧残留账号需手动清理」（与 Design2.md §5.7 实例删除级联口径一致）；**确认提交后返回 `task_id`，按 pollTask 轮询全局任务端点（见 9.2），按钮 loading 防重复，终态后刷新列表**（Design2Report10 Q12-3）
- 空态：「还没有 Xray 实例」+「新增实例」按钮 + 前置提示「需先在 Xray 服务器开启 gRPC API 与流量统计（policy.stats）」（见 10.2）

### 8.2 节点检测（「刷新节点」）

- 实例行「刷新节点」按钮（loading 防重复）→ 调 ListInbounds 检测端点（**upsert 与 missing 置位在单个数据库事务内完成，回调（候选集重算/补推）事务提交后执行**）→ **检测结果回执**（`a-modal`）：汇总新增 N 个节点 / 更新 M 个节点 / missing 标注 K 个（实例侧已删，待处置）/ 撞名跳过 J 个（列出 inbound tag 与原因，Design2.md §3.2 撞名跳过口径）；**四项全 0 时以 `Notify.info`「节点无变化」提示，不弹回执**；**新增节点命名区**：新增节点数 > 0 时逐行展示 `tag + 系统名 + 显示名输入框`（留空=暂不命名，稍后可在节点管理页命名）；「保存显示名」对已填写行逐行调用 display-name 端点，字段级 409 提示；完成后节点列表数据刷新（节点管理页同源）；**实例不可达/gRPC 调用失败时，以 `a-alert error` 展示错误摘要（last_error 截断口径见 10.1）与「检查 api_addr / 实例状态后重试」引导，不弹回执**（Design2Report10 Q12-4）
- 保存实例（新增/编辑）成功后提示「可执行刷新节点发现入站」引导

### 8.3 「开始初始化」（批量初始化，手动触发）

- `PageHeader` 右侧常驻按钮「**开始初始化**」（`a-button` 主按钮）
- 点击 → ConfirmModal 说明三点：① 幂等可重复执行；② 无凭据的活跃用户将生成 UUID 与代理密码；③ 推送范围 = 所属组分配节点 ∪ 公共节点（超限用户跳过）（Design2.md 第一章）
- 执行：**提交后返回 `task_id`，按 pollTask 轮询全局任务端点（按钮 loading 禁用，契约见 9.2；后端异步长任务，见 Design2 §5.4）** → 终态后计数提示 `Notify.success`「初始化完成：同步成功 X，失败 Y」；Y>0 时附「失败用户可在用户管理页逐条重试」

### 8.4 账号对账区（实例级）

- 实例行「对账」按钮 → 展开对账面板（`a-drawer` 或页内分区，按实例维度）：调对账端点获取期望集比对结果；**实例不可达/GetInboundUsers 失败时以 `a-alert error` 展示错误摘要与「检查实例状态后重试」引导，不渲染四分区**（Design2Report10 Q12-4）
- **比对结果表**（双态列表）：**四分区** —— ①「待补推」（期望集有 / 实例无，含面板用户与独立账号两类，来源 Tag 区分 user / ext）：用户名或账号名 + 节点 + 「补推」行操作；②「无头用户」（实例有 / 期望集无，**`user-` 前缀**）：Xray email + 所在 inbound + 清理勾选；③「**疑似独立账号残留**」（**`ext-` 前缀或无法匹配前缀者**，黄色警示分区 + 说明文案「此类账号可能为手动维护的独立账号或配置导入/重装后的残留，请确认后再清理」，**清理勾选默认不勾选**，Design2.md §5.11 对账防护）：Xray email + 所在 inbound + 清理勾选；④「**凭据不一致**」（期望集命中 email，但实例侧 Account 与面板 UUID/代理密码不一致）：面板账号/独立账号 + 节点 + 「移除并重推」行操作（后端以 GetInboundUsers 返回的 Account 比对，Design2.md §5.5/§5.10）
- **一键补推**（对待补推全集，loading 防重复；**超限面板用户与超限独立账号均跳过并提示**，同 Design2.md §5.5 超限前置拦截口径）、**一键清理**（对 ②③ 分区勾选项，**ConfirmModal 危险确认**「将从 Xray 删除 N 个无主账号，不可恢复」）与**凭据不一致「移除并重推」**（对 ④ 分区勾选项，先 RemoveUser 再 AddUser）：**三项执行端点均为异步长任务，提交返回 `task_id` 按 pollTask 轮询（见 9.2），终态结果计数回执，超限跳过项在任务结果中提示**；**单条补推/单条凭据修复走单条同步端点 `POST .../reconcile/push-one` / `.../credentials-one`（请求 timeout 120s，按钮 loading 防重复，Design2Report10 Q9）**
- 空态：四分区均空时 `a-result success` 风格提示「账号已一致，无需处理」

### 8.5 独立账号 Tab（Design2.md §5.11）

**列表**（双态列表）：列：名称（备注名）、email（`code` 风格）、配额（GB 或「不限」）、本月用量（字节→GB 换算见 10.1）、推送摘要（聚合 Badge 同 4.5 口径：N 个 inbound 已同步绿 / 失败红 + **行内重试（调 `retryExtSync`）** / 同步中橙）、超限标记（红色 `a-tag`「已超限」+ 行警示底色）、操作（**复制凭据** / 编辑 / **重置配额** / 删除）。

- **创建/编辑弹窗**（720px）自上而下四区：
  1. **基本信息**：名称（重名 409 提示；email 系统分配，创建后只读展示）
  2. **凭据区（双轨）**：`a-radio-group`「自动生成 / 手填接管」——自动生成：提交时生成（创建成功弹窗一次性展示，见下）；手填接管：UUID + 代理密码输入（`a-input-password`）+ 说明文案「系统分配 email 并推送至所选入站；若 Xray 侧已存在同 email 账号则按幂等口径接管成功，请确保所填凭据与 Xray 侧一致」；**编辑时凭据字段留空 = 保留原凭据**（同 6.2 manual 节点编辑口径）
  3. **推送目标**：按实例分组的 inbound 多选（**仅列四协议、allocatable=1、missing=0、所属实例 enabled=1 且节点 enabled=1 的 inbound**；**inbound 标签展示有效渲染名，有自定义显示名时副行展示系统标识名**；无可用节点时空态「请先在实例页检测节点」）；**提交形状为 `[{instance_id, inbound_tag}]`**（Design2Report10 Q12-6）；**编辑时移除已推送目标在保存前提示「将同步从 Xray 移除已取消目标的账号」**（Design2Report10 Q12-6）
  4. **配额**：`a-input-number`（GB）+ 副说明「0 或留空 = 不限流量」
- **凭据展示与复制**：`createExtAccount`（自动生成模式）响应直接返回一次性明文凭据 `{uuid, proxy_secret}`，前端创建成功弹窗一次性展示（`code` 风格 + 复制按钮）+ 警示文案「凭据即该账号的唯一凭证，请妥善保管」，随后丢弃前端明文；手填接管模式不返回明文；列表「复制凭据」按钮复制解密凭据（专用端点，见 9.1）+ Toast 同款警示
- **超限与重置**：超限行红色标记 + 副文案「账号已从 Xray 移除，重置配额可恢复」；重置配额 ConfirmModal「将清空该账号本月流量累计并恢复 Xray 账号（凭据不变）」；结果回执同 4.5
- **删除账号**：ConfirmModal 危险样式「将从全部已推入站（N 个）移除该账号并删除面板记录，不可恢复」
- **空态**：「还没有独立账号」+「创建独立账号」按钮 + 一句话用途说明「用于向面板账号体系之外的人员/场景分发凭据（可手写入自定义订阅内容）」（见 10.2）

### 8.6 自检结论（编写后）

- 功能匹配：实例 CRUD 与 enabled 停用语义（Design2.md §5.9）、连通性测试、节点检测刷新与撞名跳过（§3.2）、**新增节点行内命名（display_name）**、手动初始化幂等与计数（第一章）、采集状态与连续失败告警（§5.8）、实例级对账补推/清理（§5.10）、**独立账号模型与凭据双轨 / 手动推送目标 / 配额管理 / 对账 ext 分区防护 / OFF 清单（§5.11）**均有着落
- 动线：实例线：开关 ON → 录实例 → 测试连接 → 保存 → 刷新节点 → 组分配 → 开始初始化 → 对账兜底；独立账号线：创建（生成/接管）→ 勾选推送目标 → 复制凭据 → 写入自定义订阅；配额线：用量展示 → 超限摘除标记 → 重置恢复
- 八项：空态（实例/对账/独立账号各自空态）/加载（列表 TriStateList、测试/检测/初始化/对账/凭据复制各自 loading）/错误（测试失败 alert、409 重名、采集告警、推送失败重试）/<768 卡片态/暗色/权限（advanced_mode 驱动菜单与路由守卫，后端 403 兜底）/防重复（全部长操作按钮 loading）/危险确认（删实例、一键清理、删独立账号、重置配额 ConfirmModal；凭据复制警示 Toast）

---

## 九、前后端连接设计（仅契约，不写后端实现）

> 本章只定义前端 API 层的端点形状与协议约定；后端处理逻辑见 Design2.md §5.10。既有 API 文件（auth/profile/home/emergency/settings/log/subscription/group/platform/rule/share/version/approval/system）沿用现有拆分模式：按领域独立 ts 文件，导出类型 + 同名函数，经 `@/api/request` 的 `http` 实例调用。

### 9.1 新增 API 文件与端点清单

#### `api/pool.ts`（新增，规则素材池域）

| 函数 | 方法/路径 | 请求 | 响应 | 用途 |
|------|-----------|------|------|------|
| listPools | GET /api/admin/pools | — | `{ list: Pool[] }`（含 name/urls/entry_count/last_synced_at/sync_status/sync_error/auto_sync/sync_time） | 池列表（5.2.1） |
| createPool / updatePool / deletePool | POST/PUT/DELETE /api/admin/pools(/:id) | 名称 + urls + auto_sync + sync_time | 统一成功结构 | 池 CRUD |
| listEntries | GET /api/admin/pools/:id/entries?page=&page_size= | 分页参数 | `{ list: Entry[], total }`（rule_type/match_value/source/sort_order） | 条目分页（5.2.2） |
| createEntry / updateEntry / deleteEntry | POST/PUT/DELETE | 类型 + 匹配值 | 统一结构；重名 409 | 手动条目 |
| submitSync | POST /api/admin/pools/:id/sync | — | `{ task_id }` | 提交同步任务 |
| getSyncStatus | GET /api/admin/pools/:id/sync/status | — | `{ task_id, status: running/succeeded/failed/partial, per_url: [{url, ok, added, removed, skipped, error}], started_at, finished_at, error }` | 轮询最近任务终态（9.2；任务持久化，重启中断置 failed） |
| listSyncTasks | GET /api/admin/pools/:id/sync/tasks?page=&page_size= | 分页参数 | `{ list: SyncTask[], total }`（最近 N 条历史任务，后端保留 7 天，形状同 getSyncStatus 单条） | 池详情同步历史列表（5.2.2） |

#### `api/node.ts` / `api/proxyGroup.ts`（新增）

| 函数 | 方法/路径 | 备注 |
|------|-----------|------|
| listNodes / createNode / updateNode / deleteNode | GET/POST/PUT/DELETE /api/admin/nodes(/:id) | 列表含 source/protocol/host/port/is_public/enabled/allocatable/missing/display_name/render_name；manual 创建/编辑（**名称创建后只读**）；xray 行整体字段只读，仅 enabled/is_public 开关与 display_name 可改 |
| setNodeDisplayName | PUT /api/admin/nodes/:id/display-name | 请求 `{ display_name }`（空串=清空并恢复系统名）；仅 source=xray；有效渲染名全局唯一 + 跨命名空间校验，冲突 409；用于节点管理页「命名」与检测回执批量命名（6.3/8.2） |
| getProtocols | GET /api/admin/nodes/protocols | 返回 `{ list: [{ protocol, label, form_schema, link_mappings, sensitive_fields }] }`（label 为协议展示名）；协议注册表：manual 节点表单动态渲染与敏感字段脱敏输入（6.2） |
| toggleNode | PUT /api/admin/nodes/:id/toggle | enabled / is_public 行内开关（400：非法切换如 allocatable=0 置 public） |
| listProxyGroups / createProxyGroup / updateProxyGroup / deleteProxyGroup | GET/POST/PUT/DELETE /api/admin/proxy-groups(/:id) | definition 含组类型 + 有序节点/子组引用名；环形引用/内容约束后端 400 兜底 |
| togglePresetGroup | PUT /api/admin/proxy-groups/:id/preset-toggle | 预设组启用勾选 |

#### `api/group.ts`（节点分配/配额写操作仅高级模式；`getGroupDetail` 不受限，off 时省略高级字段）

| 函数 | 方法/路径 | 备注 |
|------|-----------|------|
| getGroupDetail | GET /api/admin/groups/:id | advanced_mode=on：返回组基础信息 + 节点分配（node_id/node_name/display_name/render_name/is_public/source/sort_order）+ `candidate_nodes`（含 `in_partial_blueprint`）+ default_quota；**advanced_mode=off 不 403，仅返回基础组信息（省略 nodes/candidate_nodes/default_quota 等高级字段）** |
| updateGroupNodes | PUT /api/admin/groups/:id/nodes | 请求 `{ node_ids }`；后端校验候选集/公共节点/可用性（越界 400）；off 时 403 |
| updateGroupQuota | PUT /api/admin/groups/:id/quota | 请求 `{ default_quota: number 或 null }`；0/NULL 不限，负数 400；off 时 403 |

#### `api/xray.ts`（新增，仅高级模式调用）

| 函数 | 方法/路径 | 响应形状要点 |
|------|-----------|--------------|
| listInstances / createInstance / updateInstance / deleteInstance | GET/POST/PUT/DELETE /api/admin/xray/instances(/:id) | 实例行含 name/slug/api_addr/api_tag/enabled/last_collect_at/collect_status/collect_error（**错误字段后端截断 200 字符，不脱敏**）；**deleteInstance 为异步长任务，提交返回 `{ task_id }`，pollTask 轮询（见 9.2）** |
| testConnection | POST /api/admin/xray/instances/test | `{ ok, error? }`（不落库） |
| detectNodes | POST /api/admin/xray/instances/:id/detect | `{ added, updated, missing, skipped: [{tag, reason}], added_nodes: [{node_id, tag, name}] }`（8.2 回执；added_nodes 供新增节点行内命名） |
| runInit | POST /api/admin/xray/init | **异步长任务：提交返回 `{ task_id }`，pollTask 轮询全局任务端点（见 9.2），终态返回 `{ synced, failed }`**（8.3 计数回执） |
| reconcile | GET /api/admin/xray/instances/:id/reconcile | `{ to_push: [...], orphans: [...], ext_orphans: [...], credential_mismatches: [...] }`（8.4 四分区比对结果；ext_orphans = 疑似独立账号残留，默认不勾选；四分区均含节点/inbound 信息供展示） |
| pushRepair / cleanOrphans / repairCredentials | POST …/reconcile/push、…/reconcile/clean、…/reconcile/credentials | **异步长任务：提交返回 `{ task_id }`，pollTask 轮询全局任务端点（见 9.2），终态返回计数回执**；repairCredentials = 对「凭据不一致」勾选项先 RemoveUser 再 AddUser（8.4 ④ 分区） |
| pushOne / repairCredentialsOne | POST …/reconcile/push-one、…/reconcile/credentials-one | **同步端点（120s）**：单条补推与单条凭据修复（请求传单项目标，返回统一成功结构/计数，8.4 行内操作，Design2Report10 Q9） |
| retryUserSync | POST /api/admin/xray/users/:id/retry | 4.5 行内重试 |
| resetQuota | POST /api/admin/xray/users/:id/reset-quota | 4.5 重置配额（沿用 Design2.md §5.8 端点） |
| getUserSync | GET /api/admin/xray/users/:id/sync | 该用户 xray_users 聚合状态与 last_error 摘要（用户详情/重试诊断备用） |
| getInstanceStats | GET /api/admin/xray/instances/:id/stats | 采集状态与最近成功时间（实例列表已聚合该字段，端点备用） |
| listExtAccounts / createExtAccount / updateExtAccount / deleteExtAccount | GET/POST/PUT/DELETE /api/admin/xray/ext(/:id) | 账号行含 name/email/quota/quota_exceeded/本月用量/推送摘要/push_targets；创建请求含 credential_mode（generate/manual）+ 凭据（手填时）+ 推送目标列表 `push_targets: [{instance_id, inbound_tag}]`（8.5，Design2Report10 Q12-6）；**自动生成模式创建响应一次性返回 `{account, credentials:{uuid, proxy_secret}}`，手填模式仅返回 account**；配置导入导出中 push_targets 随账号导出、导入后按 instance+tag 重绑 node_id（Design2.md §5.4） |
| retryExtSync | POST /api/admin/xray/ext/:id/retry | 对 failed 推送记录逐个重试（期望集仍含则 AddUser，否则 RemoveUser），计数回执（8.5 行内重试） |
| getExtCredentials | GET /api/admin/xray/ext/:id/credentials | `{ uuid, proxy_secret }` 解密返回供复制（敏感端点，**响应携带 no-store 禁缓存头**，前端复制警示文案见 8.5） |
| resetExtQuota | POST /api/admin/xray/ext/:id/reset-quota | 同 resetQuota 口径（Design2.md §5.11） |

#### `api/settings.ts`（高级模式分区）

| 函数 | 方法/路径 | 备注 |
|------|-----------|------|
| getAdvancedSettings | GET /api/admin/settings/advanced | `{ advanced_mode, collect_interval_minutes, traffic_card_enabled }`；**`collect_interval_minutes` 为 API 字段，后端映射到配置键 `xray_collect_interval_minutes`（默认 10，≥1）** |
| saveAdvancedSettings | PUT /api/admin/settings/advanced | 请求 `{ advanced_mode, collect_interval_minutes, traffic_card_enabled, confirm_word? }`；**仅 ON→OFF 状态翻转时**必须携带 `confirm_word=DISABLE`（已 OFF 再保存为幂等 no-op，返回当前状态不建任务）；**关闭时返回 `{ task_id }` 进入异步 OFF 清空任务轮询**；开启仅置位不推送（4.7） |

#### `api/user.ts`（高级模式扩展）

| 函数 | 方法/路径 | 备注 |
|------|-----------|------|
| setUserQuota | PUT /api/admin/users/:id/quota | 请求 `{ quota_override: number 或 null }`；NULL=继承组默认配额、0=不限流量；仅高级模式，off 时 403（4.5 配额覆盖） |

#### `api/assembly.ts`（新增，装配域）

| 函数 | 方法/路径 | 备注 |
|------|-----------|------|
| getAssemblyContext | GET /api/admin/assembly/context | 一次性拉取装配器候选数据：节点列表 / 代理组 / 素材池摘要 / 平台（含 product_type 过滤）/ 规则实体（减少多端点并发） |
| getBlueprint | GET /api/admin/versions/:id/blueprint | 重新编辑载入快照（含失效引用标记 `invalid_refs` 与 `name_changed` 对照信息，见 5.4/5.3.5） |
| generate | POST /api/admin/assembly/generate | 请求：target_syntax（clash-yaml / sr-subs / generic-subs / sr-conf）+ 目标实体 + 头部 + 勾选（**规则素材池为有序数组**）+ 手动规则；响应：`{ version_id, auto_activated, skipped, warnings }` |
| preview | POST /api/admin/assembly/preview | 同请求体，返回 `{ content, skipped, warnings, name_changed? }`（不落库，预览步使用）；SR subs / generic-subs 返回明文原文 |

### 9.2 `api/request.ts` 增量：pollTask 轮询封装与 timeout 覆盖

现状：全局 15s timeout、无轮询能力（已勘察）；本期新增：

- **`pollTask({ submit, query, interval=1500, timeout })` 契约**：
  - `submit()` 提交任务得标识 → 按 `interval` 轮询 `query()`；`query` 返回含终态字段的结果
  - **终态判定**：status ∈ {succeeded, failed, partial} 即停；非终态继续
  - **最大等待**：默认 5 分钟（可参覆写），超时抛特定错误 → UI 展示兜底文案「任务仍在后台执行，请稍后刷新查看结果」（10.1）
  - **卸载取消**：返回取消句柄；组件 onUnmounted 调用后停止轮询（仅停前端轮询，后端任务不中断）
  - **网络抖动容忍**：单次轮询请求失败不终止任务，连续失败 3 次才报错
- **按请求 timeout 覆盖**：`http` 实例支持单请求 config 覆写 timeout；**装配生成 / 预览、节点检测、独立账号创建/编辑/删除、用户/独立账号配额重置与重试、连通性测试等同步长操作统一 120s**（普通请求维持全局 15s；素材池同步与下述异步长任务走 pollTask 轮询，不受此限）
- **异步长任务与全局任务端点**：`POST /api/admin/settings/import`（**仅 v2 含 Xray 后处理时；v1 导入保持同步响应 `{message}`，不轮询，Design2Report10 Q5**）、`PUT /api/admin/settings/advanced`（advanced_mode 翻转为 false）、`POST /api/admin/xray/init`、`POST /api/admin/xray/instances/:id/reconcile/{push|clean|credentials}`、`DELETE /api/admin/xray/instances/:id` 均为异步长任务，提交返回 `{ task_id }`；前端用 pollTask 轮询**全局任务端点 `GET /api/admin/tasks/:id`**，响应 `{ id, kind: import|off_clear|xray_init|reconcile_exec|instance_delete, status: running/succeeded/failed, result, error }`；任务由后端进程内 registry 维护（不落库），**服务重启后任务丢失，任何查询（含重启前提交的任务与未知 task id）一律返回 failed（「服务重启，任务中断」）**，未完成的 Xray 清理由实例对账与部署文档手动清理口径兜底

### 9.3 既有 API 适配增量

| 文件 | 增量 |
|------|------|
| `api/system.ts` | `/api/system/status` 响应新增 `advanced_mode` 布尔字段；`useSystemStore` 据此驱动侧边栏与路由守卫（见 2.1/2.4）；**另新增 `traffic_card_enabled` 布尔（默认 true）**：首页流量卡（3.1.1）与个人中心「本月流量」行（3.2）显隐共用（后端 Build6 Step5 暴露） |
| `api/home.ts` | **新增独立汇总端点 `GET /api/home/summary`（会话凭据）**：顶层 `traffic` 字段 `{unlimited, used_bytes, quota_bytes|null, exceeded}`（基础模式 `{unlimited:true}`；高级模式配额不限时亦 `unlimited=true` 且 **`quota_bytes=null`**，见 3.1.1 三态）+ 首页默认规则卡片字段（规则名 / 版本信息 / 规则 Token 链接 / 未设置标记；**仅首页展示，个人中心不展示**）；`/api/home/platforms` 保持纯列表包裹，仅承载平台卡片（含**管理员平台卡片预览形态字段**：模板信息 + 激活版本摘要，替代原池内订阅平铺与一键导入/复制字段）（Design2Report11 决策） |
| `api/profile.ts` | 新增 `getProfileTraffic()` 调 `GET /api/profile/traffic`（会话凭据），响应 `{unlimited, used_bytes, quota_bytes|null, exceeded}`；个人中心「本月流量」行（3.2）数据源，基础模式 `{unlimited:true}`（Design2 §5.10） |
| `api/subscription.ts` | 订阅行增 product_type（yaml / subs / generic-subs）/ 内容形态字段；创建/编辑请求移除组关联字段；按平台预览端点（会话凭据，无 subscription_id 参数，Design2.md 第一章）；**用户会话预览 subs / generic-subs 装配模板返回明文原文**（与存储形态一致，base64 仅为下载下发编码，Design2Report11 决策） |
| `api/group.ts` | group_selections 相关字段/函数全部移除；新增节点分配与 default_quota 函数（`getGroupDetail` / `updateGroupNodes` / `updateGroupQuota`，端点契约见 9.1 `api/group.ts`；**updateGroupNodes/updateGroupQuota off 时 403，getGroupDetail off 不 403 且省略高级字段**） |
| `api/rule.ts` | 增 is_home_default 字段与设置默认端点；创建请求首版本改为可选 |
| `api/settings.ts` | 增 `getAdvancedSettings` / `saveAdvancedSettings`（advanced_mode / 采集间隔 / 流量卡片开关三键；**ON→OFF 翻转**须确认词 DISABLE 且返回 task_id，已 OFF 重复保存幂等 no-op）；**导入与任务状态新增 `importConfig`（POST 现有导入端点；**响应含 `task_id` 为 v2 走 pollTask，不含为 v1 同步完成**，Design2Report10 Q5）与 `getAdminTask`（GET /api/admin/tasks/:id，全局任务端点，kind 含 off_clear/import/xray_init/reconcile_exec/instance_delete）** |
| `api/version.ts` | 版本行增 blueprint 存在标记（驱动「装配」Tag 与重新编辑按钮）；装配生成创建走 assembly.generate |
| `api/user.ts` | 用户列表行新增字段（高级模式）：本月用量字节数、Xray 同步状态聚合（含 last_error 摘要）、配额覆盖值与有效配额、quota_exceeded 标记；新增 `setUserQuota`（端点契约见 9.1 `api/user.ts` 扩展，对应 4.5 四个扩展点） |
| `package.json` | 新增 `diff`（jsdiff）依赖（4.1 预览 diff，见 1.2 DiffView） |

### 9.4 错误码 → UI 映射增量（对齐 AGENTS.md §4.8，沿用 Design1-UI §7.3 基线）

| 响应 | 前端展示（增量） |
|------|------------------|
| 403 高级未开启 | 高级端点返回 403 且系统状态 advanced_mode=off → `message.warning`「高级功能未开启」（区别于普通「权限不足」）；同时刷新系统状态联动菜单隐藏 |
| 409 池名 / 节点名 / 节点显示名 / 平台订阅占用 / 实例名 / 代理组名 / 独立账号名 | `Notify.error` 展示后端冲突描述（对应 5.2.1 / 6.2/6.3/8.2 / 4.1 / 8.1 / 7.2 / 8.5） |
| 409 素材池条目去重 | 手动添加条目冲突提示（见 5.2.2） |
| 400 候选集/约束类校验 | 组分配越候选集、代理组 DAG/内容约束、平台格式校正不一致、**装配悬空引用/未勾选目标组/不可用节点/平台无订阅行** → 表单级/页面级错误定位（见 4.3/4.4/7.2/5.3.0） |
| 200 业务错误注释块 | 沿用 Design1-UI §7.3：预览内容以 `# error:` 开头时弹窗顶部 alert 转人话（覆盖新增「无激活版本」注释块场景） |

---

## 十、全局交互约定与空状态文案表（增量）

### 10.1 增量交互约定

| 场景 | 约定 |
|------|------|
| 流量换算 | 字节 → GB 两位小数（÷1024³）；0 字节显示 0.00；配额 NULL/0 统一显示「不限流量」（Design2.md §5.8 配额 0/NULL 语义） |
| 超限提示统一文案 | 用户端：「本月流量已超限，代理账号已暂停，请联系管理员重置」；管理端行标签：「已超限」；两处同源可维护 |
| 同步状态 Badge | **六状态五色**口径见 1.2（pending 橙 / running 蓝 / synced・succeeded 绿 / failed 红 / partial 橙 / missing 灰）；失败态必附原因 Tooltip（不静默） |
| 轮询兜底文案 | pollTask 超时：「任务仍在后台执行，请稍后刷新查看结果」；轮询网络中断：「状态查询失败，点击重试」 |
| 移动端拖拽降级 | 全部拖拽排序场景 <768 统一降级为「上移 / 下移」按钮（组节点分配 / 代理组节点引用 / 装配第④步素材池顺序 / 平台 scheme），不引入触屏拖拽库 |
| 暗色模式 | 新增页面全部随 ConfigProvider 主题算法，无逐组件手工适配；无固定深色底例外区（实时日志区不在本期范围） |
| 时间展示 | 沿用 Design1-UI §六：后端 UTC → 前端本地时区；定时同步时刻标注 UTC（避免跨时区误解） |
| 防重复提交 | 沿用 Design1-UI §7.4：异步提交按钮 loading 禁用；轮询中同步按钮、行内开关切换、重试/补推/清理等长操作一律 loading 防重复 |
| 危险确认 | 沿用 ConfirmModal；本期新增危险清单：OFF 清空（确认词 DISABLE，仅 ON→OFF 翻转要求）、**配置导入「无实例/账号且高级关闭」分支（确认词 DISABLE）**、删池/删节点/删实例/删自建组/**删独立账号**、一键清理无头用户、is_public 切换、**节点 enabled 1→0 停用切换**、首页默认规则切换；凭据类复制（独立账号凭据）附警示 Toast |
| 错误串展示 | `last_error` / `collect_error` 等 UI 可见错误字段后端截断至 200 字符、不做地址脱敏（仅管理员可见，Design2.md §5.4），Tooltip 直接展示 |

### 10.2 新增页面空状态文案表（Empty 统一口径，沿用 Design1-UI §7.5 风格）

> 改造但非新增的列表（订阅管理「还没有订阅」/ 用户管理 / 规则管理 / 平台）空态文案沿用 Design1-UI §7.5，本表仅列新增项；**版本列表（VersionManageView）空态 Design1-UI §7.5 未定义，由本表补：文案「暂无版本」，引导操作「上传文件 / 在线编辑 / 装配生成（订阅与规则页）」**。

| 页面/部件 | 文案 | 引导操作 |
|-----------|------|---------|
| 版本列表 | 暂无版本 | 上传文件 / 在线编辑 / 装配生成（订阅与规则页） |
| 素材池列表 | 还没有规则素材池 | 「新建素材池」按钮 |
| 池详情条目 | 池内暂无条目 | 「新增条目」+「同步」按钮 |
| 节点管理 | 还没有节点 | 「添加节点」按钮 + 高级模式检测提示 |
| 代理组管理 | 预设组随迁移内置，此态仅理论出现 | — |
| Xray 实例 | 还没有 Xray 实例 | 「新增实例」按钮 + policy.stats 前置提示 |
| 独立账号 | 还没有独立账号 | 「创建独立账号」按钮 + 用途说明（见 8.5） |
| 对账结果 | 账号已一致，无需处理 | — |
| 装配：无匹配目标平台 | 请先创建对应格式的平台 | 直达平台管理链接 |
| 装配：xray 节点区 | 未检测到 Xray 节点 | 高级模式录入实例后手动刷新节点发现 |
| 组编辑：候选集并集为空 | 请先装配并激活 Clash YAML / SR 节点订阅 / 通用节点订阅模板 | 「前往装配」按钮 |
| 首页：分流规则卡 | 管理员暂未设置分流规则 | 卡片入口跳 /rules 保留 |
| 首页：平台卡（管理员预览） | 未激活 | 预览按钮禁用 + Tooltip |

### 10.3 既有约定沿用清单

Design1-UI §六全局交互约定（脱敏回显 / 防枚举措辞 / 时间展示 / 401 拦截 / 复制敏感链接警示 / 暗色模式）与 §7.4 表单弹窗规范、§7.6 全局细节（顶部进度条 / zh_CN locale / AntD 图标 / 无障碍底线）全部沿用，本文不重复。

---

## 十一、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-17 | 初始版本：承载 Design2.md 全部受影响界面的 GUI 规格（十章主体 + 未受影响页面核验结论）；全量重写式自包含文档，Design1-UI.md 冻结不回写 |
| v1.1 | 2026-08-18 | 新增独立 Xray 账号功能规格（Design2.md §5.11）：第八章双页签骨架与 8.5 独立账号 Tab（列表/双轨创建弹窗/凭据复制/配额重置/删除）、8.4 对账区改四分区（新增 ext 残留分区默认不勾选，防误清理）、4.7 OFF 清空清单加项、9.1 增 ext 端点与 reconcile 响应扩展、9.4 冲突映射增独立账号名、10.1/10.2 危险确认与空态补充 |
| v1.2 | 2026-08-18 | 构建前深度审阅修订：强制组落库口径（内置渲染结构、国外流量成员随快照，5.3.1/7.3）与预设组启用状态持久化（proxy_groups.enabled，7.1）；移除素材池条目手动调序（1.2/5.2.2/5.5/9.1/10.1 联动，顺序改系统维护）；xray 节点区空态文案改「手动刷新节点发现」（5.3.1/10.2）；PageHeader/CopyField 改「本期新建」口径（1.3）；懒加载分组补「无 vite 配置动作」注记（2.3）；删除订阅影响清单改「按本规格新写」（4.1）；对账补推超限提示补面板用户（8.4） |
| v1.3 | 2026-08-18 | 构建前决策落盘：新增 generic-subs 产物类型与「通用节点订阅」页签（A1/A2）、首页仅展示分流规则卡片（A5）、素材池整体排序（B7）、全新部署纯增量 1009 DDL、候选集并集重算、名称不可修改与字符集校验、节点 missing 恢复补推、对账凭据不一致分区、协议注册表端点、pool_sync_tasks 持久化、xray 节点仅 missing 可删等（详见本轮审阅修订） |
| v1.4 | 2026-08-19 | Xray 节点显示名（display_name）：xray 节点「命名」入口与检测回执批量命名（6.1/6.3/8.2）、有效渲染名双行展示（4.3/5.3/7.2）、display-name API 与 detectNodes added_nodes（9.1）、显示名 409 冲突映射（9.4）、跨命名空间唯一校验说明（6.2/6.3，含代理组名/强制组名/Clash-mihomo 内建保留代理名） |
| v1.5 | 2026-08-19 | Design2Report5 核验修订：组名双向命名空间校验与组类型可编辑（7.2）、OFF 确认词 DISABLE（4.7）、导入带实例自动开高级提示（4.7）、预览旧名 Tooltip 与命名客户端提示（5.3.5/6.3）、推送目标过滤补全（8.5）、API 契约补 group/settings/userQuota/sync/stats（9.1/9.3）、长请求统一 120s（9.2） |
| v1.6 | 2026-08-19 | Design2Report7 复核补齐：§9.3 `api/system.ts` 增 `traffic_card_enabled` 字段；`api/home.ts` traffic 字段形状补齐 `{unlimited, used_bytes, quota_bytes|null, exceeded}`（与 Build6 Step5 实现对齐，Q3） |
| v1.7 | 2026-08-19 | Design2Report8 修订：导入无实例/账号且高级关闭分支 DISABLE 确认词与异步任务轮询；池同步历史列表；diff 数据来源与 name_changed 契约；preview/generate skipped/warnings；per_url skipped；retryExtSync；ext 创建一次性凭据响应；group 详情 off 不 403；节点/显示名禁止空格；装配严格校验与目标平台无订阅行分支；长请求 120s 范围扩展；api/profile.ts 个人中心流量端点；危险确认清单补导入分支 |
| v1.8 | 2026-08-19 | 构建前核验修订（用户确认）：§4.6 取消首页默认交互定稿为「默认行专设取消默认操作」（不再两案并列） |
| v1.9 | 2026-08-19 | Design2Report9 修订：强制组 emoji 化（🚀直接连接/🌎国外流量/🛟无法归属的流量，5.3.1/6.2/6.3/7.2/7.3）；长操作异步化与全局任务端点 /api/admin/tasks/:id（8.3/8.4/9.1/9.2/9.3，120s 清单闭合）；OFF 幂等 no-op 与独立账号整行删除措辞（4.7）；enabled 停用 ConfirmModal、删订阅清单补候选集副作用（6.1/4.1）；版本列表空态、四色→六态五色、DiffView 不可达分支移除与暗色、池 Badge 数据源注记、用量无数据显示—、检测零新增提示、getExtCredentials no-store、错误串截断口径、超限管理端处置指引、api/group.ts 标题、protocols 契约补 label |
| v2.0 | 2026-08-19 | Design2Report10 修订：DiffView 恢复无目标「整体新增」分支（Q4，按用户决策改 UI）；导入双确认词 IMPORT→DISABLE 与 v1 同步/v2 异步响应口径（Q5）；装配校验补未勾选子组拒绝（Q7）；🌎国外流量成员仅节点（Q8）；对账单条同步端点 push-one/credentials-one 契约与行内交互（Q9）；实例删除轮询、检测/对账不可达错误态、api_addr 变更提示、push_targets 形状与移除确认（Q12） |
| v2.1 | 2026-08-19 | Design2Report11 核验修订：目标规则实体选择固定步骤①（5.3.3/5.3.4）；v1.1 变更记录「三分区」勘误为「四分区」；首页 traffic 与分流规则卡片改独立端点 `GET /api/home/summary`、quota_bytes 不限时统一 `null`（9.3）；manual 节点编辑允许变更协议=整体重新填表（6.2）；停用预设组 400 拒绝生成（5.3.0）；用户预览 subs/generic-subs 装配模板返回明文原文（9.3） |
| v2.2 | 2026-08-22 | 同步 Issue4 R19-02/R19-08 已落地决策：装配页改为「规则素材池 / 代理组 / 构建订阅·规则」三个一级 Tab，四个子平台并入构建 Tab；代理组不再独立路由/侧边栏菜单，改为 `/admin/assembly?tab=proxy-groups`。 |
