# Issue6.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考），承接已归档的 [Issue5.md](./Issue5.md)（R20 系列，保留备查）。
> 设计记录见 [Design2.md](./Design2.md) 与 [Design2-UI.md](./Design2-UI.md)；构建方案见 [Build8.md](./Build8.md)（当前构建轮次）；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 一、进行中问题

### R21-01 素材池详情弹窗首行沿用旧版内联标题样式，按钮与关闭 X 重复

- **现象：**
  - 在「订阅装配 → 规则素材池 → 素材池详情」弹窗中，首行仍使用旧版页面内钻取样式：`素材池 / {池名}` 是一个可点击的 Button 面包屑，而不是只读标题。
  - 首行右侧同时出现「同步 / 编辑 / 返回」操作按钮，与弹窗右上角的关闭 X 形成重复返回出口；「新增条目」按钮也紧邻首行工具区，视觉层级混杂。
  - 用户期望：弹窗首行仅包含只读标题和更新时间等元信息；「同步 / 编辑 / 返回 / 新增」等按钮需要重新安排展示位置，避免与退出打叉键重复。

- **根因：**
  - `frontend/src/views/admin/assembly/PoolTab.vue` 的详情 Modal 未设置 `title`，弹窗没有使用 Ant Design Vue 的原生标题栏。
  - `frontend/src/views/admin/assembly/PoolDetail.vue` 自绘首行时使用了 `Button type="link"` 渲染「素材池 / {池名}」面包屑，并把「同步 / 编辑 / 返回」都放在同一行右侧。
  - 详情弹窗改为 Modal 后（Issue4 R19-03），旧的内联钻取式头部未被同步改造，导致保留「返回」+ Modal 关闭 X 双重出口。

- **影响范围：**
  - 素材池详情弹窗 UI 与当前弹窗规范不一致；
  - 可点击面包屑误导用户以为标题是导航入口，实际上只是关闭详情；
  - 「返回」与「X」行为重复，增加无效出口；
  - 首行操作按钮过多，弱化了只读标题和更新时间等核心信息。

- **修复方案（用户已确认推荐方向）：**
  1. **使用 Modal 原生标题栏**：在 `PoolTab.vue` 的详情 Modal 增加只读 `title`，建议为 `素材池详情 · {{ currentDetail()?.name }}` 或直接使用池名；保留默认关闭 X。
  2. **移除旧版面包屑按钮**：`PoolDetail.vue` 首行不再渲染 `<Button type="link">` 形式的「素材池 / {池名}」；只读标题和元信息由只读文本/标题元素展示。
  3. **首行仅保留只读信息**：池名 + URL 数 + 条目数 + 上次同步时间 + 同步状态 Badge；如需上下文，可用非交互文本前缀「素材池」。
  4. **移除首行「返回」按钮**：关闭详情统一走 Modal 的 X / 取消 / ESC；`PoolDetail` 的 `back` emit 可保留供未来扩展，但不再在 UI 上重复出现。
  5. **操作按钮分区**：
     - 「同步」「编辑」移到页头右侧（或内容区工具条），保持轻量；
     - 「新增条目」作为列表工具条的主操作，保留在手动条目列表上方；
     - 首行不再堆叠「同步 / 编辑 / 返回 / 新增」多个按钮。
  6. **同步文档**：更新 `Design2-UI.md` §5.2.2，将“页内钻取视图、面包屑可返回”的描述改为“Modal 详情弹窗 + 只读标题/元信息 + 无重复返回出口”。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）

### R21-02 素材池详情弹窗未自适应显示区域，滚轮会滚动整个 Modal

- **现象：**
  - 「订阅装配 → 规则素材池 → 素材池详情」弹窗内容较长时，弹窗整体没有限制在可视区域内；用户在弹窗内滚动滚轮时，会上下移动整个 POPUP/Modal，而不是只滚动弹窗内容。
  - 期望：弹窗自适应当前显示区域；滚动范围仅限于弹窗内容物，标题/关闭按钮保持在固定位置。

- **根因：**
  - `frontend/src/views/admin/assembly/PoolTab.vue` 中详情 Modal 只设置了 `:width="760"` 和 `:footer="null"`，未约束 `maxHeight` / 内容区滚动。
  - Ant Design Vue 的 Modal 默认 `wrap` 为 `overflow: auto`，内容超高时会滚动整个遮罩/弹窗容器；素材池详情包含条目表格、分页、同步历史等多段内容，容易超出视口。

- **影响范围：**
  - 素材池详情弹窗在内容较多/小屏幕下体验差；
  - 用户会误以为弹窗被“移动”，焦点和阅读位置不稳；
  - 与常规“弹窗固定、内容内滚”的交互不一致。

- **修复方案（推荐）：**
  1. 在 `PoolTab.vue` 的详情 Modal 增加高度约束：
     - `:body-style="{ maxHeight: 'calc(100vh - 220px)', overflowY: 'auto' }"`（或等价 Tailwind/自定义类，如 `max-h-[calc(100vh-220px)] overflow-y-auto`）。
     - 可同时设置 `:centered="true"`，让弹窗在可视区域内居中而不随内容上移。
  2. 保持 Modal 头部标题与关闭 X 固定，只让 `.ant-modal-body` 滚动。
  3. 若后续统一规范长内容弹窗，可在全局样式中为 `.ant-modal-body` 增加 `max-height: calc(100vh - 220px); overflow-y: auto;`，但需评估是否影响其他 Modal；当前 Issue 先按素材池详情弹窗记录。
  4. 同步更新 `Design2-UI.md` §5.2.2：补充“弹窗自适应视口，内容区内部滚动，Modal 本体不随滚轮移动”。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）

### R21-03 节点管理新建/编辑节点表单动态字段样式混乱，布尔开关旁多出文本输入框

- **现象：**
  - 「节点管理 → 新建节点 / 编辑节点」弹窗中，协议动态表单布局混乱。
  - 布尔型字段（如 UDP、TLS、跳过证书校验等）本应显示为 `Switch` 开关，但同字段还会额外渲染一个普通文本输入框，视觉上像“开关被当成 Input 处理”。

- **根因：**
  - `frontend/src/views/admin/NodesView.vue` 动态字段模板中的 `v-else` 配对错误：
    ```html
    <Switch v-else-if="f.type === 'bool'" ... />
    <Input.TextArea v-else-if="f.type === 'object'" ... />
    <div v-if="objectError && objectErrorField === f.name" ...></div>
    <Input v-else ... />
    ```
  - 最后的 `<Input v-else>` 紧跟在错误提示 `<div v-if>` 之后，因此被 Vue 当作“错误提示 div 的 else 分支”；当无错误时，错误 div 不渲染，这个 `<Input>` 反而总会渲染。
  - 结果：每个非 object 字段（含 bool 开关字段）都会多出一个通用文本输入框，导致表单混乱。

- **影响范围：**
  - 新建/编辑 manual 节点的动态协议表单（VMess / VLESS / Trojan / Hysteria 等含 bool 字段的协议）；
  - 用户看到重复控件，可能误填布尔字段为文本值；
  - 提交时 `protocol_json` 中布尔字段可能被输入框覆盖为字符串，影响后端渲染。

- **修复方案：**
  1. 调整模板顺序，让 `<Input v-else>` 继续作为字段类型链的最后分支：
     ```html
     <Input.Password v-if="f.type === 'password'" ... />
     <InputNumber v-else-if="f.type === 'number'" ... />
     <Select v-else-if="f.type === 'select'" ... />
     <Switch v-else-if="f.type === 'bool'" ... />
     <Input.TextArea v-else-if="f.type === 'object'" ... />
     <Input v-else ... />
     <div v-if="objectError && objectErrorField === f.name" ...></div>
     ```
  2. 将动态字段改用 `Form.Item`（或至少统一 label 布局）以进一步规范样式；本轮核心先修复 `v-else` 配对问题。
  3. 补充前端单测：新建节点选择含 bool 字段的协议时，页面应渲染 1 个 Switch 且不渲染对应文本输入框。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）

### R21-04 节点管理新建/编辑节点表单整体布局需按“协议 → 基础字段 → 协议细节”优化

- **现象：**
  - 当前「节点管理 → 新建节点 / 编辑节点」弹窗的基础表单顺序为：名称 → 协议 / 服务器 / 端口 → 协议动态细节。
  - 用户反馈表单层级不够清晰，希望调整为：
    1. **协议**；
    2. **名称、服务器、端口**（并列一级）；
    3. **各协议细节**。

- **根因/现状：**
  - `frontend/src/views/admin/NodesView.vue` 模板中，`Form.Item label="名称"` 在协议之前；协议/服务器/端口被放在同一 `grid-cols-3`（协议 1 列、服务器 2 列、端口 1 列），与用户期望的“协议作为第一级、基础字段并列”不一致。
  - 动态协议字段虽已使用两列网格，但整体层次缺少分区引导。

- **影响范围：**
  - 新建/编辑 manual 节点的表单可读性；
  - 用户先看到名称再选协议，选择协议后基础字段/细节才变化，不符合“先选协议再填信息”的自然操作流。

- **修复方案（推荐）：**
  1. **第一级：协议**
     - 将协议 `Select` 置于表单最上方，建议全宽显示（或至少单独一行），并默认选择 `vless`。
  2. **第二级：基础字段并列**
     - 名称、服务器、端口放在同一行：
       - 名称（编辑时只读）
       - 服务器
       - 端口
     - 建议 `grid-cols-1 md:grid-cols-3` 或移动端纵向堆叠。
  3. **第三级：协议细节**
     - 协议动态字段（密码 / 数字 / 选择 / 开关 / 对象 JSON）放在基础字段下方，保持当前 `md:grid-cols-2` 双列网格；
     - 长字段（如 object/TextArea）继续 `col-span-2`。
  4. **配合修复**：先落地 R21-03 的 `v-else` 配对修复，避免开关字段旁多出输入框，再调整布局层级。
  5. 可在细节区前加分隔/分组标题（如「协议细节」），进一步提升层次感。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）

### R21-05 订阅装配预检显示模式优化：未满足项编号分条 + 超链接引导

- **现象：**
  - 订阅装配开始前的预检目前把缺失项拼成一行文案展示（`构建前缺少：…、…`），用户反馈不够清晰，容易只看到其中一条。
  - 希望优化为：把未满足的前置条件以编号分条展示，并给每一条添加“去对应页面修改/创建”的超链接。
  - 文案需调整：`目标平台订阅条目` 改为 `目标平台未创建订阅池`，更明确表达缺失动作。

- **根因/现状：**
  - `frontend/src/views/admin/AssemblyView.vue` 中 `buildPreflightMissing` 只返回 `string[]`，模板用单个 `Alert` 的 `message` + `join('、')` 聚合展示。
  - 未满足项没有独立条目，也没有对应的管理页跳转入口。
  - 部分条件存在 `else if` 链（如无匹配平台时不再检查订阅池），虽然避免无效提示，但整体展示层级较弱。

- **影响范围：**
  - 构建 Clash YAML / SR 节点订阅 / 通用节点订阅 / SR 分流规则前的用户引导体验；
  - 新管理员从空系统开始装配时，需要手动逐个页面排查前置缺失项；
  - 文案“目标平台订阅条目”语义不直观。

- **修复方案（用户已确认：仅未满足项编号分条）：**
  1. 将 `buildPreflightMissing` 从 `string[]` 改为结构化的 `Array<{ id: string; text: string; actionText: string; to: string }>`。
  2. 未满足项按 1、2、3… 编号逐条列出，每项包含：
     - 前置条件描述；
     - “去修改/去创建”超链接按钮（`router-link` 或 `Button type="link"`），点击跳转到对应管理页。
  3. 建议映射：
     - 缺少可用节点 → 「前往节点管理」 `/admin/nodes`
     - 缺少匹配的目标平台 → 「前往平台管理」 `/admin/platforms`
     - 目标平台未创建订阅池 → 「前往订阅管理」 `/admin/subscriptions`
     - 缺少至少一个规则实体（sr-conf）→ 「前往规则管理」 `/admin/rules`
  4. 文案调整：
     - `目标平台订阅条目` → `目标平台未创建订阅池`；
     - 其余保留现有描述，并补充动作文案。
  5. 预检区仍保持阻止进入装配器；当无未满足项时继续显示装配器。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）

### R21-06 从订阅/规则页进入装配时无需再选择“类型与目标”

- **现象：**
  - 在订阅管理点击某个订阅的「装配生成」时，已经携带该订阅的平台/产物格式进入订阅装配；但装配器第一步仍显示「类型与目标」，要求用户再次选择目标平台。
  - 从规则管理/规则版本页进入 SR 分流规则装配时，同样已携带规则实体，却仍需再次选择。
  - 用户反馈：创建订阅时已经选择过目标，装配时不需要再选择“类型与目标”。

- **根因/现状：**
  - `frontend/src/views/admin/SubscriptionsView.vue` 的 `goAssembly(sub)` 跳转 `/admin/assembly?tab=…&platform_id=…`，`AssemblyView.vue` 的 `loadContext()` 也已将 `route.query.platform_id` / `rule_id` 写入 `form.platform_id` / `form.rule_id`。
  - 但 `AssemblyView.vue` 的 `stepDefs` 仍无条件包含 `{ key: 'target', title: '类型与目标' }`，`AssemblerShell` 仍渲染 Target 步骤。
  - 因此用户需要重复选择已确定的目标。

- **影响范围：**
  - 从订阅管理「前往装配/装配生成」进入的 Clash YAML / SR 节点订阅 / 通用节点订阅装配；
  - 从规则管理进入的 SR 分流规则装配；
  - 重复选择降低效率，且容易在已带入目标后误改。

- **修复方案（用户已确认：直接跳过 Target 步骤）：**
  1. 在 `AssemblyView.vue` 中识别“已带目标进入”：
     - `platform_id` 来自 `route.query.platform_id` 且 > 0；
     - 或 `rule_id` 来自 `route.query.rule_id` 且 > 0；
  2. 当命中上述任一情况时，从 `stepDefs` 中过滤掉 `target` 步骤：
     - Clash YAML：`header → nodes → rules → preview → generate`
     - SR 节点/通用订阅：`header → nodes → preview → generate`
     - SR 分流规则：`header → rules → preview → generate`
  3. 调整 `nextStep()`：跳过 target 后不再依赖 `targetReady()` 检查（因为目标已由来源页带入并校验）。
  4. 在装配器顶部增加只读提示，明确“目标已从订阅/规则页带入：平台/规则实体 xxx”，避免用户不知道目标来源。
  5. 直接进入 `/admin/assembly`（无 `platform_id` / `rule_id`）时仍保留「类型与目标」步骤。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）

### R21-07 Clash 装配“节点与代理组”步骤：代理组选中后应支持“选择与排序”节点

- **现象：**
  - 在订阅装配 Clash 板块的「节点与代理组」步骤中，用户勾选某个代理组（如 YouTube）后，当前只看到 `preset` 标签和勾选态，无法直观地针对该组选择可用节点并排序。
  - 已有能力仅在下方“已勾选代理组的节点引用顺序”区域以 `上移/下移/移除` 操作展示，入口不显眼、拖拽体验缺失。
  - 期望：勾选代理组后，组名侧面出现「选择与排序」按钮（替代当前 `preset` 文案的位置）；点击后打开弹窗，可从 ABCDE 等可用节点中勾选节点并拖拽排序（如 ACD）。

- **根因/现状：**
  - `frontend/src/views/admin/assembly/NodesGroupsStep.vue` 中，勾选组只显示 `{{ g.name }}<Tag class="ml-1">preset</Tag>`，没有直接的“配置该组节点引用”入口。
  - 节点引用顺序的编辑入口被放在页面底部的独立分区，用户需要先勾选组、再向下滚动寻找，且目前仅支持按钮上移/下移，不支持拖拽。
  - 底层数据已具备：`form.group_node_orders[g.name]` 与 `update-group-node-order` 事件，可扩展。

- **影响范围：**
  - Clash YAML 装配的“节点与代理组”步骤；
  - 对预设组和自建组均适用；
  - 影响装配生成的代理组内部节点顺序，当前用户只能依赖代理组管理页的全局定义，无法在一次装配中灵活覆盖。

- **修复方案（推荐）：**
  1. **入口**：在 `presetGroups` / `customGroups` 的勾选项内，当 `form.group_names.includes(g.name)` 时，将 `preset/自建` 标签替换或并列呈现为「选择与排序」小按钮；
     - 预设组：替换当前 `preset` 标签位置；
     - 自建组：同样提供入口，标签可保留在按钮旁。
  2. **弹窗**：点击「选择与排序」打开 Modal（或 Drawer），标题如 `节点选择与排序 · {{ g.name }}`：
     - 上方/左侧：可用节点多选列表（manual 节点 + 可用 xray 节点），当前已选节点高亮；
     - 下方/右侧：已选节点有序列表；
     - 支持桌面端 HTML5 拖拽排序，移动端保留上移/下移按钮（可复用 `RulesStep.vue` 的 DnD 模式）。
  3. **保存**：弹窗确认后 emit `update-group-node-order(g.name, selectedNodes)`，更新 `form.group_node_orders`；
     - 若清空选择，则移除 `group_node_orders[g.name]`，回退到代理组全局定义。
  4. **展示**：保留或简化页面底部“已勾选代理组的节点引用顺序”作为只读概览，避免与组内弹窗重复。
  5. **校验**：只允许选择当前已勾选的可用节点集合，避免选择未勾选/不可用节点；排序只影响本次装配。

- **状态：** ✅ 已修复（2026-08-25，前后端测试全绿）

### R21-08 移除代理组“节点引用（有序）”全局设计，节点改为装配时按组选择/排序

- **现象/需求：**
  - 用户希望与 R21-07 配套：
    1. 直接去掉装配步骤中底部“已勾选代理组的节点引用顺序”区域；
    2. 去掉代理组管理下每个代理组编辑弹窗中的“节点引用（有序）”设计；
    3. 节点引用/排序能力改为在 Clash 装配的代理组勾选项内，通过「选择与排序」弹窗按组配置。
  - 已确认范围：**前后端一起移除**代理组全局节点引用支持；代理组全局定义仍至少需要一个子组。

- **现状（需移除/改造点）：**
  - `frontend/src/views/admin/assembly/NodesGroupsStep.vue`：
    - 底部 `已勾选代理组的节点引用顺序` 区块（上移/下移/移除）需删除；
    - 代理组勾选项内新增「选择与排序」入口（见 R21-07）。
  - `frontend/src/views/admin/ProxyGroupsView.vue`：
    - 表单中 `节点引用（有序）` Form.Item 需删除；
    - `form.node_names`、`nodeList`、`useSortableList`、拖拽相关、`staleNodeNames`、`availableNodeNames` 等仅服务于节点引用的逻辑可移除；
    - `memberSummary` 只展示子组，不再拼接 `definition.nodes`；
    - 创建/编辑提交的 `definition` 不再包含 `nodes`。
  - `backend/internal/proxygroup/proxygroup.go`：
    - `Definition` 去掉 `Nodes` 字段（或 JSON 层忽略，但不再读写）；
    - `validateDefinitionWithDAG` 删除节点存在性校验；
    - 内容约束改为“至少包含一个子组”（🚀直接连接 / 🌎国外流量 / 其他已存在组），不再要求节点；
    - DAG 校验继续只基于 `Groups`。
  - `backend/internal/assembly/load.go` / `render_clash.go` / `validate.go`：
    - `groupData.Nodes` 不再从全局定义加载；
    - Clash 渲染中，组内节点只来自 `in.GroupNodeOrders[group.name]`，子组继续来自 `g.Groups`；
    - 校验 `GroupNodeOrders` 时不再要求节点属于该组全局 `definition.nodes`，只要求是本次已勾选的可用节点、不重复。
  - 历史数据兼容：
    - `definition_json` 中旧 `nodes` 字段可忽略，无需数据迁移；但旧数据中的 `nodes` 不再参与新的装配/渲染。
  - 文档同步：
    - `Design2-UI.md` 全局拖拽口径、§7.1/§7.2/§7.3、§5.3.1 中所有“代理组节点引用（有序）”描述改为“装配时按组选择与排序”；
    - `Design2.md` 相关代理组定义/内容约束描述同步更新。

- **影响范围：**
  - Clash YAML 装配中代理组节点成员来源从“全局定义 + 本次覆盖”改为“本次装配按组指定”；
  - 代理组管理页不再维护节点成员，只维护子组/类型；
  - 预设组种子仍以 `🚀直接连接` 子组作为默认成员，不受影响；
  - 历史装配快照/渲染计划已冻结节点，不受后端移除影响。

- **修复方案（推荐）：**
  1. 先完成 R21-07 的“选择与排序”弹窗，确保装配侧已有替代能力；
  2. 删除 `NodesGroupsStep.vue` 底部旧排序区；
  3. 删除 `ProxyGroupsView.vue` 中节点引用编辑 UI 与相关前端状态；
  4. 后端移除 `Definition.Nodes` 相关读写、校验与渲染逻辑；
  5. 调整后端装配校验：`GroupNodeOrders` 只约束“已勾选、可用、不重复”，不再依赖组全局 `nodes`；
  6. 补充前后端测试：无全局节点引用的代理组可在装配时指定节点并输出；空组或未配置节点且无子组的组在装配校验中拒绝；
  7. 同步 Design2/Design2-UI 文档。

- **状态：** ✅ 已修复（2026-08-25，前后端测试全绿）

### R21-09 装配流程优化：预览直接出结果，去掉“确认生成”步骤

- **现象/需求：**
  - 当前四类装配流程的倒数第二步为「预览」，用户需要点击「预览产物」后才看到结果；最后还有独立的「确认生成」步骤。
  - 用户希望：
    1. 进入「预览」步骤时直接展示装配结果（自动预览，不再要求手动点“预览产物”）；
    2. 「预览」步骤的底部主按钮由「下一步」改为「确认生成」；
    3. 去掉独立的「确认生成」步骤；
    4. 生成成功后直接显示当前结果页，提供「去版本管理激活」和「继续装配」两个操作。
  - 已确认：四类装配（Clash YAML / SR 节点订阅 / 通用节点订阅 / SR 分流规则）统一调整。

- **根因/现状：**
  - `frontend/src/views/admin/AssemblyView.vue` 的 `stepDefs` 对每类装配都包含 `generate` 步骤；
  - `frontend/src/views/admin/assembly/PreviewStep.vue` 需要用户点击「预览产物」才调用 `doPreview()`；
  - `frontend/src/views/admin/assembly/AssemblerShell.vue` 底部根据“是否最后一步”显示「下一步」或「生成版本」；
  - `frontend/src/views/admin/assembly/GenerateStep.vue` 单独承担确认生成按钮；
  - `generateResult` 结果页已存在，但在生成后仍与装配器同时展示，未作为唯一结果页。

- **影响范围：**
  - 四类装配器的用户操作路径；
  - 减少一次点击和一步切换，提升生成效率。

- **修复方案（推荐）：**
  1. `stepDefs` 去掉 `generate` 步骤，让预览成为最后一步：
     - Clash YAML：`header → nodes → rules → preview`
     - SR 节点/通用节点订阅：`header → nodes → preview`
     - SR 分流规则：`header → rules → preview`
     - 若后续 R21-06 跳过 target，则在对应列表中不再包含 `target`。
  2. `PreviewStep.vue`：
     - 进入步骤/切换至预览步骤时自动调用 `doPreview()`；
     - 预览加载中显示 loading，完成后直接展示结果；
     - 保留「与当前激活版本对比」等辅助按钮，不再需要手动「预览产物」主按钮。
  3. `AssemblerShell.vue`：
     - 支持最终步骤按钮文案参数，如 `final-action-text="确认生成"`；
     - 当预览为最后一步时，底部按钮显示「确认生成」并触发 `generate`；
     - 移除/不再渲染独立的 `generate` 步骤卡片。
  4. `AssemblyView.vue`：
     - 点击「确认生成」直接调用 `doGenerate()`；
     - 生成成功后隐藏装配器，仅展示结果页 `Result`：
       - 自动激活：标题「首个版本已自动激活」；
       - 未自动激活：标题「已入池未生效，请激活」；
       - 操作按钮：`去版本管理激活` + `继续装配`。
  5. `GenerateStep.vue` 可删除或保留为未使用组件（建议删除以减少死代码）。
  6. 同步更新 `Design2-UI.md` §5.3.0/§5.3.5 步骤描述。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）

### R21-10 装配生成/激活后系统卡死，订阅列表在唯一 SQLite 连接上进行嵌套查询导致死锁

- **现象：**
  - 本地 Docker 中点击装配「确认生成/激活」后，再访问订阅管理/装配页时系统卡死。
  - 容器日志连续出现：
    ```
    msg=ERROR path=/api/admin/subscriptions msg="检查表 assembly_blueprints 失败: context canceled"
    msg=WARN key=debug_mode err="读取配置 debug_mode 失败: context canceled"
    msg=ERROR path=/api/admin/platforms msg="context canceled"
    msg=INFO path=/api/admin/subscriptions status=500 latency_ms=15011
    msg=INFO path=/api/admin/platforms status=500 latency_ms=15011
    ```
  - 与 Issue4 R19-05 现象一致：不是数据库损坏，而是请求在等待 SQLite 唯一连接，超过前端 15s 后客户端取消导致 `context canceled`。

- **根因（深入检查确认）：**
  - **不是候选集重算死锁**，而是 `backend/internal/subscription/subscription.go` 的 `List` 在遍历结果集时调用了 `contentKind`，其中又对同一个唯一的 `*sql.DB` 发起新查询：
    ```go
    rows, err := s.store.DB().QueryContext(ctx, `SELECT ... FROM subscriptions ...`)
    defer rows.Close()
    for rows.Next() {
        ...
        sub.ContentKind, err = s.contentKind(ctx, sub.ID, sub.CurrentVersion) // 内部再 QueryRowContext
        ...
    }
    ```
  - `contentKind` 内部会先 `tableExists(ctx, s.store.DB(), "assembly_blueprints")`，再执行 `QueryRowContext` 查询 `assembly_blueprints`。此时外层 `rows` 仍持有唯一的 SQLite 连接。

- **深入排查记录（证据）：**
  - Docker 容器：`vpn-subscription-management-vpn-sub-1`，数据库 `/data/app-dev.db`（WAL 模式）。
  - 数据库关键数据：
    ```sql
    SELECT id, name, platform_id, current_version, product_type FROM subscriptions;
    -- Clash | 1 | 1 | yaml
    SELECT COUNT(*) FROM assembly_blueprints;       -- 1
    SELECT COUNT(*) FROM group_nodes;               -- 0
    ```
  - 容器日志时序：
    ```
    07:24:42 POST /api/admin/assembly/generate 200 (12ms)
    07:25:41 GET /api/admin/subscriptions       500 (5285ms)  "检查表 assembly_blueprints 失败: context canceled"
    07:25:57 GET /api/admin/assembly/context    500 (15055ms) "检查表 assembly_blueprints 失败: context canceled"
    07:28:30 GET /api/admin/subscriptions       500 (15011ms) "检查表 assembly_blueprints 失败: context canceled"
    ```
  - 临时复现测试：
    - 在 `backend/internal/subscription` 构造含激活版本的订阅后调用 `List(ctx)`；
    - 结果：`List deadlocked: context deadline exceeded`；
    - 确认根因就是 `subscription.List` 在 `for rows.Next()` 内调用 `contentKind` 导致的单连接死锁。
  - 候选集重算路径已排除：`group_nodes=0`、`xray_candidates=[]`，数据量极小，不构成 75 秒阻塞源。

  - 当前 `store.Open` 设置 `db.SetMaxOpenConns(1)`，内层查询必须等待空闲连接；连接只有在外层 `rows` 关闭后才释放，形成死锁，直到外层请求 Context 被客户端取消。
  - 为什么“装配后必现”：装配生成前，所有订阅 `current_version=0`，`contentKind` 直接返回空，不会进入内层查询；装配首次生成会自动激活版本，`subscription 1.current_version` 从 0 变为 1，之后 `List` 对每个激活订阅都会触发 `contentKind` 内层查询 → 死锁。
  - 相关日志中的 `检查表 assembly_blueprints 失败: context canceled` 正是来自 `contentKind` 的 `tableExists`。
  - 二次验证：容器数据库 `subscriptions` 中 `Clash` 订阅 `current_version=1`、`assembly_blueprints` 1 条；而 `groups.group_nodes=0`，候选集重算路径数据量极小，并非卡死源。

- **影响范围：**
  - 所有读取订阅列表的接口：`GET /api/admin/subscriptions`、`GET /api/admin/assembly/context`（内部调用 `subSvc.List`）；
  - 其他排队等待唯一连接的 API（如 `/api/admin/platforms`）也会被连带阻塞至 `context canceled`；
  - 只要存在已激活版本（`current_version > 0`），每次访问订阅列表都会复现。

- **修复方案（推荐）：**
  1. **修复 `subscription.List`**：先把订阅行全部读取到内存并结束/关闭外层 `rows`，再对每行调用 `contentKind`；或者把 `content_kind` 通过单个 JOIN/EXISTS 子查询并入原 SELECT，消除逐行嵌套查询。
     推荐优先改成单条 SQL：
     ```sql
     SELECT s.id, s.slug, s.name, s.platform_id,
            COALESCE(s.current_version,0), s.product_type, p.name,
            CASE WHEN EXISTS(
              SELECT 1 FROM assembly_blueprints b
              JOIN versions v ON v.id = b.version_id
              WHERE v.owner_type = 'subscription'
                AND v.owner_id = s.id
                AND v.version_no = s.current_version
            ) THEN 'blueprint' ELSE 'upload' END AS content_kind
     FROM subscriptions s
     JOIN platforms p ON p.id = s.platform_id
     ORDER BY p.id, s.id
     ```
  2. **全局审计同类嵌套查询**：搜索 `for rows.Next()` 内调用 `s.store.DB()`/`QueryContext`/`QueryRowContext` 的地方，确保先读完并释放 Rows 再查询。
  3. 后续可考虑把 `CandidateSet` 等已知读多写少逻辑也按同样原则审计，避免单连接下的隐藏死锁。
  4. 补充回归测试：
     - 数据库含 `current_version > 0` 的订阅时，`subscription.List` 必须能在 `SetMaxOpenConns(1)` 下正常返回；
     - 装配生成后访问 `/api/admin/subscriptions`、`/api/admin/assembly/context` 不再 `context canceled`。

- **状态：** ✅ 已修复（2026-08-25，`go test ./...` 全绿，并补回归测试验证 `subscription.List` 不再死锁）

### R21-11 系统初始化后 Dev 模拟登录 UI 永远无法登录

- **现象：**
  - 系统初始化并选择「模拟 OIDC（Dev）」后，登录页显示「Dev 模拟登录」表单；
  - 填写邮箱并点击「模拟登录」后，页面看起来会跳转，但实际未登录成功，回到登录页或仍无会话。

- **根因（静态检查确认）：**
  - `frontend/src/views/LoginView.vue` 的 `onMockLogin` 只调用了 `mockLogin(...)` 并跳转 `/`：
    ```ts
    async function onMockLogin() {
      mockSubmitting.value = true
      errorMsg.value = ''
      try {
        await mockLogin({ ... })
        await router.push('/')
      } catch (err) {
        errorMsg.value = (err as Error).message
      } finally {
        mockSubmitting.value = false
      }
    }
    ```
  - 后端 `/api/auth/oidc/mock/login` 成功返回 `{ token, expires_at, status }`，但前端没有把 `token` 写入 Auth Store / localStorage，也没有处理 `pending` / `conflict` 等状态。
  - 对比本地登录 `onSubmit` 使用 `auth.loginAction(form)` 会调用 `setSession(data.token, data.user)`；OIDC 回调页 `OidcCallbackView.vue` 也会 `auth.setSession(token)`。
  - 因此 mock 登录后端签发成功，但前端未保存会话凭据，跳转 `/` 后仍视为未登录。

- **影响范围：**
  - Dev/本地调试模式下选择 Mock OIDC 的登录流程；
  - 影响初始化向导后通过模拟登录进入系统的体验；
  - 不涉及真实 OIDC 回调流程（该流程已正确保存 token）。

- **修复方案（推荐）：**
  1. 在 `onMockLogin` 中保存 token：
     ```ts
     const res = await mockLogin({ ... })
     if (res.status === 'pending') {
       errorMsg.value = res.message ?? '账号待审批，请等待管理员审核'
       return
     }
     auth.setSession(res.token)
     await router.push('/')
     ```
  2. 若后端返回 `status` 为 `pending` 或 `conflict`，前端应显示后端 message，而不是继续跳转或吞掉错误。
  3. 可将 mock 登录封装为 auth store 的 `mockLoginAction`，与 `loginAction` 保持一致。
  4. 补充前端单测：mock 登录成功后 token 应写入 localStorage，且跳转 `/` 前已设置会话。

- **状态：** ✅ 已修复（2026-08-25，前端 build + 65 例测试通过）











---

## 二、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-24 | 创建 Issue6；记录 R21-01 素材池详情弹窗首行样式与按钮重复问题，确认修复方向：Modal 原生标题 + 只读元信息 + 移除重复返回 + 操作按钮分区；追加 R21-02 弹窗自适应显示区域、滚轮仅滚动内容区；追加 R21-03 节点新建/编辑表单动态字段 `v-else` 配对错误导致布尔开关旁多渲染输入框；追加 R21-04 节点表单按“协议 → 基础字段 → 协议细节”重构布局；追加 R21-05 装配预检未满足项编号分条 + 超链接 + 文案改“目标平台未创建订阅池”；追加 R21-06 从订阅/规则页进入装配时直接跳过“类型与目标”步骤；追加 R21-07 Clash 装配代理组勾选后支持“选择与排序”节点弹窗；追加 R21-08 前后端移除代理组全局“节点引用（有序）”，改为装配时按组选择/排序，并保留至少一个子组约束；追加 R21-09 四类装配“预览直接出结果 + 确认生成 + 结果页”，移除独立确认生成步骤；追加 R21-10 深入排查并确认装配生成/激活后系统卡死根因为 `subscription.List` 在唯一 SQLite 连接上嵌套查询 `contentKind` 导致死锁 |
| v1.1 | 2026-08-24 | 针对 R21-10 继续深入排查：访问运行中 Docker 容器数据库、查看日志时序，并用临时复现测试确认根因为 `subscription.List` 内嵌套查询 `contentKind` 导致单连接死锁；已整合证据写入 R21-10，按用户要求暂不修复 |
| v1.2 | 2026-08-24 | 新增 R21-11：系统初始化后 Dev 模拟登录永远无法登录，根因为前端 `onMockLogin` 未保存后端返回的 token 且未处理 pending/conflict；仅记录方案，未改代码 |
| v1.3 | 2026-08-25 | 按已记载问题实施修复 R21-01~R21-11：素材池详情弹窗改版与滚动、节点表单 `v-else`/布局、装配预检/跳过 Target/代理组选择排序/移除全局节点引用、装配预览直接生成、`subscription.List` 死锁修复、mock 登录保存 token；前端 build + 65 例测试、后端 `go test ./...` 全绿 |
