# Issue7.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考），承接已归档的 [Issue6.md](Issue6.md)（R21 系列，保留备查）。
> 设计记录见 [Design2.md](Design2.md) 与 [Design2-UI.md](Design2-UI.md)；构建方案见 [Build8.md](Build8.md)（当前构建轮次）；编码指令见 [AGENTS.md](AGENTS.md)（**唯一强要求**）。

---

## 〇、本轮说明

- **核查方式：** 仅做快速静态检测，不执行构建、测试、脚本运行；先分析改进/修复方案，必要时先与用户确认方向。
- **当前状态：** 已记录 8 条进行中问题（R22-01、R22-02、R22-03、R22-04、R22-05、R22-06、R22-07、R22-08），修复方案已整合确认（见 §〇.1），**尚未开始修复代码**。

---

## 〇.1、修复方案整合（2026-08-26 用户确认，未开始实施）

> 本节汇总 R22-01 ~ R22-08 的最终修复方案、用户决策与主要文件落点；状态仍为“待实施”。

### 已确认决策

- **修复范围：** 8 条问题全部一起修复。
- **R22-03：** 目标选择区采用“副 Tab 下方、步骤条上方常驻卡片”，不再保留“类型与目标”步骤。
- **R22-04：** 默认平台（`is_default=1`）的 `product_type` 在后端被修改时**返回 400 拒绝**，前端编辑页禁用产物格式选择。
- **R22-05：** 除查询侧返回显示名称外，**同时补齐成功下载日志中的平台标识写入**，保证平台列可显示名称。
- **R22-06：** 高级模式关闭时不仅隐藏 Xray 节点板块，还**从本次装配表单中剔除 Xray 节点及相关组排序引用**。
- **R22-08：** SR-conf 允许不预建规则实体，装配器输入规则名称，生成时自动创建规则并继续版本装配。

### R22-01 大数据同步锁库修复

- `backend/migrations/1014_pool_sync_optimize.sql`
  - 新增 `pool_entries(pool_id, source, rule_type, match_value)` 联合索引。
- `backend/internal/pool/sync.go`
  - 将 `runSyncTask` 的单一大事务拆成短事务：
    - 批量插入每批约 500 行独立提交；
    - 全部 URL 成功后再建 `_pool_sync_keep_*` 临时表并建索引；
    - 删除前统计、差量删除均分批提交；
    - 任务终态与池快照单独短事务回写。
  - 保留“全部 URL 成功才删除”的业务语义。
  - `SubmitSync` 可选增加只读预检 running。

### R22-02 版本列表缺少 id

- `backend/internal/version/version.go`
  - `ListVersions` SELECT 增加 `v.id`，Scan 到 `Version.ID`。
- `backend/internal/version/version_test.go`
  - 断言版本列表返回真实 `ID > 0`。
- `frontend/src/views/admin/VersionManageView.vue`
  - `reEdit()` 增加 `v.id` 非正整数保护。

### R22-03 去除“类型与目标”步骤

- `frontend/src/views/admin/AssemblyView.vue`
  - 所有 `stepDefs` 移除 `target`，删除 `skipTargetStep`；
  - 副 Tab 下方、`AssemblerShell` 上方常驻目标选择卡片；
  - 路由参数仅预填目标。
- `frontend/src/views/admin/assembly/AssemblerShell.vue`
  - 移除 target 步骤标题和 `#target` 插槽。
- `frontend/src/views/admin/assembly/TypeTargetStep.vue`
  - 改造为顶部紧凑目标选择组件，支持 SR-conf 规则名称输入（配合 R22-08）。

### R22-04 默认平台产物格式锁定

- `backend/migrations/1015_platform_builtin_default.sql`
  - `platforms` 增加 `is_default INTEGER NOT NULL DEFAULT 0`；
  - 按名称回填三个默认平台。
- `backend/internal/setup/setup.go`
  - 预置平台写入 `is_default=1`。
- `backend/internal/platform/platform.go`
  - `Platform` 增加 `IsDefault`，`Get`/`List` 返回；
  - `Update` 对默认平台修改 `product_type` 返回 400。
- `frontend/src/api/platform.ts`、`frontend/src/views/admin/PlatformEditView.vue`
  - 类型增加 `is_default`；
  - 编辑默认平台时禁用产物格式。

### R22-05 日志显示名称与切换

- `backend/internal/log/access.go`
  - `AccessLog` 增加 `platform_name`、`resource_name`、`user_email`；
  - 查询按资源/平台联表派生显示名称。
- `backend/internal/download/download.go`
  - 成功下载写日志时补写平台标识，保证平台列可用。
- `frontend/src/api/log.ts`、`frontend/src/views/admin/LogsView.vue`
  - 增加字段；
  - 新增“显示名称 ↔ 唯一值”切换；
  - 桌面表格和移动端卡片同步。
- `backend/internal/log/log_test.go`
  - 扩展平台名/资源名/用户邮箱断言。

### R22-06 节点与代理组步骤优化

- `frontend/src/views/admin/AssemblyView.vue`
  - 引入 `useSystemStore`，计算 `advancedMode`；
  - 向 `NodesGroupsStep` 传 `showXray`；
  - 高级模式关闭时剔除表单中的 Xray 节点及组排序引用。
- `frontend/src/views/admin/assembly/NodesGroupsStep.vue`
  - 删除未勾选 preset 组的常显 `preset` 标签；
  - `v-if="showXray"` 包裹 Xray 节点板块；
  - 高级模式关闭时 `availableNodes`、国外流量成员仅含 manual 节点。

### R22-07 节点选择与排序弹窗

- `frontend/src/views/admin/assembly/NodesGroupsStep.vue`
  - 弹窗改为“可选节点勾选区在上、已选节点排序区在下”；
  - 删除已选节点行“移除”按钮及 `removeDraft`；
  - 保留上移/下移、拖拽排序和空态提示。

### R22-08 SR-conf 直接创建新规则

- `backend/internal/assembly/models.go`
  - `GenerateInput` 增加 `rule_name`。
- `backend/internal/assembly/validate.go`
  - SR-conf 允许 `RuleID == 0`，无已有规则时要求 `rule_name` 非空。
- `backend/internal/server/assembly.go`
  - 生成时若 SR-conf 且无 `RuleID`，自动调用规则服务创建新规则；
  - 使用新规则 ID 作为版本 owner，写入 `assembly_blueprints.rule_id`；
  - 版本创建失败时清理新规则；
  - generate 响应返回 `rule_id`。
- `frontend/src/api/assembly.ts`、`frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/assembly/TypeTargetStep.vue`
  - 增加 `rule_name`；
  - 目标区改为规则名称输入；
  - `targetReady`/`buildInput`/`buildPreflightMissing`/`goActivation` 适配；
  - 生成后使用返回的 `rule_id` 跳转规则版本管理。
- 测试：后端补充无预建规则 SR-conf Preview/Generate 成功并自动创建规则的用例。

---

## 一、进行中问题

### R22-01 素材池大数据量同步仍长时间占用写锁，导致 API 提交同步报 `database is locked`

- **现象：**
  - 来源地址小于几千行时能正常处理，例如 `https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/google-cn.txt`；
  - 来源大于数万行时（如 `https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/direct-list.txt`）系统仍会卡死并导致数据库锁死；
  - 容器日志：
    ```text
    vpn-sub-1  | time=2026-08-26T06:47:38.527Z level=ERROR msg=内部错误 path=/api/admin/pools/1/sync msg="开启事务失败: database is locked (5) (SQLITE_BUSY)"
    ```
  - 该错误出现在 `POST /api/admin/pools/1/sync` 调用 `SubmitSync` 的 `TxImmediate` 尝试开启写事务时。

- **根因/现状：**
  - `backend/internal/pool/sync.go` 的 `runSyncTask` 仍将“批量插入全部条目 + 构建临时 keep 表 + 统计删除 + 差量删除 + 任务/池快照回写”放在同一个 `bgStore.TxImmediate` 长事务中。
  - 虽然已使用独立 `bgStore`（缓解了 API 唯一连接被长期占用的旧问题），但 SQLite 同一数据库文件仍只有一个写者；该长事务期间会阻塞所有其他写事务，`busy_timeout=5000` 5 秒后其他写请求即以 `SQLITE_BUSY` 失败。
  - 大数据量时：
    - 临时 `_pool_sync_keep_*` 表未建索引，`DELETE ... NOT EXISTS (...)` 和删除前的 `COUNT(*)` 会对每个 `pool_entries` 行扫描临时 keep 表，接近 O(N×M)；
    - 数万行源在单事务内插入、建临时表、删除，写锁持有时长被显著放大。
  - 因此 Issue4 R19-01 的“独立连接 + 批量写入”只是缓解 API 单连接被占用，未解决“单写者长事务锁死”的核心。

- **影响范围：**
  - 大数据量同步期间，所有依赖 SQLite 写事务的 API（`POST /api/admin/pools/:id/sync`、其他写操作）都会在 5 秒后出现 `database is locked`；
  - 用户看到同步“卡死”/失败，并可能阻塞后续管理操作；
  - 与 Issue4 R19-01 复现路径一致，历史修复未完全闭环。

- **修复方案（推荐，用户已确认方向）：**
  1. **临时 keep 表建索引**：批量插入临时表后执行 `CREATE TEMP INDEX ... ON _pool_sync_keep_<taskID>(rule_type, match_value)`，让删除统计与差量删除走索引，避免 O(N×M)。
  2. **拆分长事务为短事务**：
     - 写入阶段：按批次（如 500/1000 行）独立提交 `INSERT OR IGNORE`；
     - 删除阶段：在插入全部成功后，再分批执行差量删除；
     - 每个小事务快速提交，避免长时间持有 SQLite 写锁。
  3. **增加联合索引**：`pool_entries(pool_id, source, rule_type, match_value)`，优化按池+来源的删除/统计查询。
  4. **保持“全部 URL 成功才删除”的业务语义**：可先完成所有插入并汇总状态，再在删除阶段使用 keep 集计算；失败/取消时跳过删除，保留旧数据。
  5. **可选**：`SubmitSync` 先以只读查询检查 `running`，避免在已有大同步持锁时进入 `TxImmediate` 后才触发 `SQLITE_BUSY`，而是更早返回明确的 409。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

### R22-02 版本管理“重新编辑”按钮因版本列表缺少 `id` 导致请求 `/versions/0/blueprint`

- **现象：**
  - 通过装配器生成的订阅/规则版本，在版本管理列表中会出现「重新编辑」按钮；
  - 点击后无响应，并提示系统错误；
  - 容器日志：
    ```text
    vpn-sub-1  | time=2026-08-26T07:18:22.431Z level=INFO msg=http_request method=GET path=/api/admin/versions/0/blueprint ip=192.168.65.1 status=400 latency_ms=0
    ```
  - 后端 `parseID` 对 `0` 返回 400（参数错误），因此前端 `getVersionBlueprint` 收到 400 并报错。

- **根因/现状：**
  - `backend/internal/version/version.go` 的 `ListVersions` 查询仅返回：
    ```sql
    SELECT v.version_no, v.file_path, v.file_name, v.created_at, v.updated_at [, blueprint exists]
    ```
    没有读取 `v.id`；`Version.ID` 是 Go 结构体的零值 `0`，被序列化为 `"id":0`。
  - 前端 `frontend/src/views/admin/VersionManageView.vue` 的 `reEdit()` 使用 `v.id` 调用：
    ```ts
    const data = await getVersionBlueprint(v.id)
    ```
    因此实际请求 `/api/admin/versions/0/blueprint`。
  - `frontend/src/api/version.ts` 的 `VersionItem` 虽然声明了 `id: number`，但后端列表始终返回 0，缺少真实版本主键。
  - 因此「重新编辑」入口链路从“读取蓝图”就开始失败，无法进入装配器编辑态。

- **影响范围：**
  - 所有装配生成的版本（订阅/规则）在版本管理中的「重新编辑」功能不可用；
  - 用户无法从版本管理复用已有装配快照进行二次编辑；
  - 仅影响列表返回；新建/上传/预览/删除/切换当前等操作不受影响。

- **修复方案（推荐）：**
  1. **后端补齐 `id` 字段**：在 `ListVersions` 的 SELECT 中增加 `v.id`，并按列顺序 Scan 到 `v.ID`。
  2. **补充后端单测**：`TestListVersions` 应断言返回的每个 `Version.ID > 0`，并与数据库 `versions.id` 一致。
  3. **前端可选强化**：`reEdit()` 若 `v.id` 非正整数时给出明确错误提示，避免再次请求 `/versions/0/blueprint`。
  4. **回归验证**：对装配生成的订阅版本点击「重新编辑」，应请求真实版本 ID 的 `/blueprint` 并成功跳转装配器编辑态。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

### R22-03 订阅装配面板第一步“类型与目标”与副 Tab 四类装配器重复，应去除该步骤

- **现象：**
  - 订阅装配面板的副 Tab 已经通过 `clash-yaml / sr-subs / generic-subs / sr-conf` 四类页签表达“装配类型”；
  - 进入装配器后第一步仍然是“类型与目标”，类型选择/展示与副 Tab 重复，增加多余操作步骤。

- **根因/现状：**
  - `frontend/src/views/admin/AssemblyView.vue` 的 `stepDefs` 对四类装配均包含：
    ```ts
    { key: 'target', title: '类型与目标' }
    ```
  - 当前 `skipTargetStep` 仅在被 `platform_id`/`rule_id` 参数进入时隐藏该步；从“构建订阅/规则”Tab 直接进入时仍会看到重复的“类型与目标”第一步。
  - `TypeTargetStep.vue` 除了冗余的“类型”展示外，还承担了非冗余信息选择：
    - 非 SR 分流：目标平台选择；
    - SR 分流：规则实体选择 + FINAL 方向。
  - 因此不能直接删除该步骤而不迁移目标选择，否则从装配 Tab 直接进入时无法选择目标。

- **影响范围：**
  - 用户每次从“构建订阅/规则”Tab 进入四类装配器都会先看到与副 Tab 重复的类型步骤；
  - 与“副 Tab 即选类型”的交互预期不一致，增加了一次点击和信息冗余；
  - 若简单删除步骤会导致目标平台/规则实体选择缺失。

- **修复方案（推荐，用户已确认方向）：**
  1. **移除步骤条中的 `target` 步骤**：四类 `stepDefs` 都不再包含 `{ key: 'target', title: '类型与目标' }`。
  2. **将目标选择移到步骤条上方常驻独立选择区（已确认）**：在“构建订阅/规则”Tab 内、`AssemblerShell` 上方放置常驻紧凑目标选择卡片/区域，包含：
     - 非 SR 分流：目标平台 `Select`；
     - SR 分流：规则实体 `Select` + FINAL 方向；
     - 类型仍由副 Tab 决定，不在该区域重复展示。
  3. **复用/改造 `TypeTargetStep`**：将其从步骤插槽中抽出，作为顶部目标选择组件；`AssemblyView.vue` 不再在 `AssemblerShell` 内传 `<template #target>`。
  4. **同步调整 `AssemblerShell`**：移除 `target` 对应步骤标题与渲染槽位；只在目标选择完成后显示/允许进入装配器，或保留顶部选择区始终可见。
  5. **清理 `skipTargetStep` 逻辑**：路由 `platform_id`/`rule_id` 参数改为直接预填顶部目标选择，不再用于过滤步骤。
  6. **保留生成前校验**：`targetReady()` 仍应在生成前校验平台/规则实体已选；可选择禁用“确认生成”直到目标就绪。
  7. **同步文档**：更新 `Design2-UI.md` §5.3.0/§5.3.4 中“步骤① 类型与目标”的描述，改为“副 Tab 选择类型 + 顶部目标选择区”。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

### R22-04 默认三个平台的“产物格式”不应在编辑页可选，应固定随平台；仅新建自定义平台时可选择

- **现象：**
  - 平台编辑页 `PlatformEditView.vue` 中，编辑任何平台都会显示可选的“产物格式” `Radio.Group`；
  - 默认三个平台（Clash Verge / v2rayNG / Shadowrocket）的产物格式本来由 Setup 固定为 `yaml / generic-subs / subs`，但在编辑页仍可被切换；
  - 用户期望：默认三个平台的产物格式在编辑时锁定、随平台固定；只有新建自定义平台时才允许选择产物格式。

- **根因/现状：**
  - `frontend/src/views/admin/PlatformEditView.vue` 的 `form.product_type` 在新建和编辑两个场景都渲染为 `Radio.Group`：
    ```vue
    <Form.Item label="产物格式">
      <Radio.Group v-model:value="form.product_type">
        ...
      </Radio.Group>
    </Form.Item>
    ```
  - 后端 `platform.Update` 也允许修改 `product_type`（仅当已有订阅条目格式不一致时拒绝），没有区分默认平台与自定义平台。
  - 当前平台表没有 `is_default` / `is_preset` 字段，无法在代码中稳定识别 Setup 预置的三个默认平台；仅能靠名称硬编码，不可靠。

- **影响范围：**
  - 管理员可误改默认平台产物格式，导致该平台的订阅/装配/下载链路与客户端实际类型不匹配；
  - 前后端没有共同约束，UI 锁定后仍无法防绕过；
  - 若后续默认平台名称被修改，按名称识别会失效。

- **修复方案（推荐，用户已确认方向）：**
  1. **新增平台内置标记**：迁移 `1015_platform_builtin_default.sql`，为 `platforms` 增加 `is_default INTEGER NOT NULL DEFAULT 0`。
  2. **Seed 默认平台写入 `is_default=1`**：`backend/internal/setup/setup.go` 的 `seedPresets` 插入三个默认平台时标记为 `1`。
  3. **已有库回填**：迁移中对名称仍为 `Clash Verge` / `v2rayNG` / `Shadowrocket` 的三行回填 `is_default=1`；无法通过名称识别的旧库可由管理员在后续确认或手动标记（当前以名称回填为推荐方案）。
  4. **后端返回并约束**：
     - `Platform` 结构体 / `Get` / `List` 增加 `is_default` 字段；
     - `platform.Update` 对 `is_default=1` 的平台拒绝修改 `product_type`（已确认，接入层返回 400）；
     - `platform.Create` 新建自定义平台 `is_default=0`，继续允许选择。
  5. **前端编辑页锁定**：
     - 编辑时若 `p.is_default === true`，将“产物格式”改为只读文本/禁用 `Radio.Group`；
     - 新建平台时保持可选择。
  6. **同步文档**：更新 `Design2-UI.md` / `Design1.md` 平台编辑描述，注明默认平台产物格式固定、仅自定义平台可配置。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

### R22-05 日志页应展示显示名称/昵称，并支持切换唯一值与显示名称

- **现象：**
  - 日志页“平台”列当前展示的是平台唯一资源值（`platform` slug），用户希望默认展示平台显示名称（用户取名）；
  - “资源标识”列应改为“资源”，且同样默认展示资源显示名称，而不是唯一资源值；
  - 日志页需要一个切换按钮，用于整体切换“显示名称 ↔ 唯一资源值”；
  - “用户”列应随切换按钮显示“昵称（username）↔ 邮箱”。

- **根因/现状：**
  - `backend/internal/log/access.go` 的 `AccessLog` 仅返回：
    - `username`
    - `platform`
    - `resource_slug`
  - 查询只 `LEFT JOIN users u ON u.id = a.user_id` 取 `u.username`，没有返回：
    - 平台显示名称 `platform_name`；
    - 资源显示名称 `resource_name`；
    - 用户邮箱 `email`。
  - `access_logs` 表本身只保存 `platform`（平台/资源标识）与 `resource_slug`（资源标识），展示名称需要后端按下载类型联查业务表。
  - `frontend/src/views/admin/LogsView.vue` 直接渲染 `record.platform`、`record.resource_slug`、`record.username`，没有显示名称字段，也没有切换按钮。

- **影响范围：**
  - 管理员在日志页难以直观识别平台/资源/用户（看到的都是随机短码/唯一值）；
  - “资源标识”列名与预期展示语义不符；
  - 缺少切换能力，无法同时满足“可读性”和“唯一定位/排障”两种需求。

- **修复方案（推荐）：**
  1. **后端扩展 `AccessLog` 字段**：
     - `PlatformName string json:"platform_name"`
     - `ResourceName string json:"resource_name"`
     - `UserEmail string json:"user_email"`
     - 保留原有 `platform` / `resource_slug` / `username` 字段供切换显示。
  2. **查询联表/派生名称**：
     - 平台名：`LEFT JOIN platforms p ON p.slug = a.platform`，取 `p.name`；
     - 资源名：按 `download_type` 使用 `CASE`/子查询：
       - `subscription` / `explicit` → `subscriptions.name`
       - `share` → `share_subscriptions.name`
       - `rule` → `rules.name`
       - `custom` → 无名称，可回退为平台名称或唯一值；
       - 未解析成功（`unassigned`）时 `resource_slug` 可能记录的是平台标识，此时资源名回退到平台名。
     - 用户邮箱：`LEFT JOIN users u` 同时返回 `COALESCE(u.email,'')`。
  3. **同步补齐成功下载日志平台标识（已确认）**：`backend/internal/download/download.go` 在成功下载写 `access_logs` 时写入平台标识，避免平台列长期为空。
  4. **后端补充测试**：扩展 `TestAccessQuery`，断言 `platform_name` / `resource_name` / `user_email` 正确回填。
  5. **前端日志页增加切换按钮**：
     - 新增本地状态，如 `displayMode: 'name' | 'unique'`，默认 `'name'`；
     - 平台列：`record.platform_name` ↔ `record.platform`；
     - 资源列标题改为“资源”，内容显示 `record.resource_name` ↔ `record.resource_slug`；
     - 用户列：`record.username`（昵称）↔ `record.user_email`；
     - 移动端卡片同样跟随切换。
  6. **同步文档**：更新 `Design1-UI.md` §5.9 日志页展示口径，补充“可切换显示名称/唯一值”。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

### R22-06 装配“节点与代理组”步骤：去除未勾选预设组常显的 preset 标签，且高级模式关闭时隐藏 Xray 节点板块

- **现象：**
  - 在「构建订阅/规则 → 节点与代理组」步骤中，预设代理组未勾选时仍持续显示 `preset` 标签，界面信息冗余；
  - 用户希望：仅当勾选某个代理组后展示“选择与排序”功能，不再常显 `preset` 类型标签；
  - 当高级模式关闭时，页面仍显示“xray 节点”板块，用户希望其在高级模式关闭下隐藏。

- **根因/现状：**
  - `frontend/src/views/admin/assembly/NodesGroupsStep.vue` 代理组区渲染预设组时：
    ```vue
    <Checkbox v-for="g in presetGroups" ...>
      <span>{{ g.name }}</span>
      <Tag v-if="!form.group_names.includes(g.name)" class="ml-1">preset</Tag>
      <Button v-else ...>选择与排序</Button>
    </Checkbox>
    ```
    未勾选时显示 `preset` Tag，勾选后才显示排序按钮。
  - “xray 节点”板块无条件渲染：
    ```vue
    <div class="text-sm font-medium mb-1">xray 节点</div>
    ...
    ```
    无论高级模式是否开启都会展示；`AssemblyView.vue` 也没有根据 `advanced_mode` 控制传入/渲染。
  - `frontend/src/views/admin/AssemblyView.vue` 目前未读取系统高级模式状态；但 `frontend/src/stores/system.ts` 的 `system.status.advanced_mode` 已可供页面使用。

- **影响范围：**
  - 装配节点步骤代理组区域可见性杂乱，类型标签干扰用户注意力；
  - 高级模式关闭时仍展示 Xray 节点选择区，与“高级模式关闭不应出现 Xray 相关配置”的预期不一致；
  - Xray 节点如继续出现在“节点选择与排序”弹窗或“🌎国外流量成员”中，也可能造成基础模式用户误选 Xray 节点。

- **修复方案（推荐）：**
  1. **移除未勾选预设组的 `preset` Tag**：
     - 删除 `NodesGroupsStep.vue` 中 `Tag v-if="!form.group_names.includes(g.name)" ...>preset</Tag>`；
     - 保留勾选后显示的“选择与排序”按钮；未勾选时仅显示组名，避免持续展示类型标签。
     - 若需保持与自建组区分，可保留“自建”Tag 或改成仅选中后显示；本轮按用户要求先移除 `preset` 常显。
  2. **根据高级模式隐藏 Xray 节点**：
     - `AssemblyView.vue` 引入 `useSystemStore`，计算 `advancedMode = computed(() => system.status?.advanced_mode === true)`；
     - 向 `NodesGroupsStep` 传入 `showXray`/`advancedMode` prop；
     - 在 `NodesGroupsStep.vue` 中：
       - `v-if="showXray"` 包裹“xray 节点”板块；
       - `availableNodes` 在高级模式关闭时仅包含 `manualNodes`，避免排序弹窗出现 Xray 节点；
       - 高级模式关闭时从本次装配表单中剔除 Xray 节点及相关组排序引用（已确认）；
       - “🌎国外流量成员”在高级模式关闭时同样只列出 manual 节点。
  3. **同步文档**：更新 `Design2-UI.md` 节点与代理组步骤描述，注明高级模式关闭时隐藏 Xray 节点；代理组列表不再常显 preset 标签。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

### R22-07 节点“选择与排序”弹窗：移除单个节点“移除”按钮，并把勾选节点区放到排序区上方

- **现象：**
  - 在「节点与代理组 → 选择与排序」弹窗中，已选节点（有序）列表每个节点都带“移除”按钮；
  - 同时下方“可选节点”区可通过取消勾选实现移除，功能重复；
  - 当前布局为“已选节点（有序）在上、可选节点在下”，用户希望改为“勾选节点区在上、排序区在下”。

- **根因/现状：**
  - `frontend/src/views/admin/assembly/NodesGroupsStep.vue` 的 `Modal` 内：
    ```vue
    <div>
      <div>已选节点（有序）</div>
      <div v-for="...">
        <span>{{ nodeLabel(name) }}</span>
        ...
        <Button size="small" danger @click="removeDraft(idx)">移除</Button>
      </div>
    </div>
    <div>
      <div>可选节点</div>
      <div class="grid md:grid-cols-2">
        <Checkbox v-for="n in availableNodes" ... />
      </div>
    </div>
    ```
  - `removeDraft` 用于已选列表中的删除按钮，但用户可以在下方勾选节点区取消勾选达到同样效果。

- **影响范围：**
  - 弹窗操作冗余，移除按钮与下方取消勾选重复；
  - 当前布局不符合“先选择、再排序”的自然操作流，用户需要先看到已选列表再找勾选区。

- **修复方案（推荐）：**
  1. **调整弹窗内部顺序**：
     - 第一块：`可选节点`（勾选/取消勾选）；
     - 第二块：`已选节点（有序）`（排序区）。
  2. **移除单个节点“移除”按钮**：
     - 删除已选节点行中的 `<Button danger @click="removeDraft(idx)">移除</Button>`；
     - 用户取消下方勾选后，该节点自动从上方排序区移除。
  3. **保留排序操作**：
     - 保留上移/下移按钮与拖拽排序；
     - 保留“已选节点”区空态提示“尚未选择节点，将使用子组引用”。
  4. **可选清理**：`removeDraft` 如不再被引用可删除，减少死代码。
  5. **同步文档**：更新 `Design2-UI.md` 节点选择与排序弹窗描述。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

### R22-08 SR-conf 装配不应前置要求已有规则实体，应可直接创建新的规则

- **现象：**
  - 在「构建订阅/规则 → SR 分流规则（sr-conf）」装配中，当前必须先存在“规则实体”（空规则或已有规则）才能进入/生成；
  - 前端预检会提示“至少一个规则实体”并引导前往规则管理；
  - 目标步骤也是“选择规则实体”，没有“直接创建新规则”的入口；
  - 用户期望：SR-conf 装配应能直接创建并添加一份新的规则，不需要先到规则管理创建空规则；SR 规则和各类订阅不是同一个概念，不应共用“必须选已有实体”的前置约束。

- **根因/现状：**
  - `frontend/src/views/admin/AssemblyView.vue`：
    - `buildPreflightMissing` 对 `sr-conf` 要求 `context.rules.length > 0`；
    - `targetReady()` 要求 `form.rule_id` 非空；
    - `buildInput()` 把 `rule_id` 作为 SR-conf 的目标字段。
  - `frontend/src/views/admin/assembly/TypeTargetStep.vue`：
    - SR-conf 分支渲染“规则实体”下拉框，并提示“暂无规则实体，可先创建空规则实体”。
  - 后端：
    - `backend/internal/assembly/validate.go` 在 `SrConf` 分支要求 `ld.rule != nil`；
    - `backend/internal/assembly/load.go` 仅在 `in.RuleID > 0` 时加载规则；
    - `backend/internal/server/assembly.go` 的 `resolveOwner` 要求 `in.RuleID > 0`。
  - 因此从“选择已有规则”到“生成”全链路都依赖预先创建规则实体。

- **影响范围：**
  - 用户第一次使用 SR-conf 装配时，必须先到规则管理创建空规则，再回到装配器选择，流程割裂；
  - 与“直接创建一份新规则”的产品预期不一致；
  - 影响 SR 分流规则独立管理和装配效率。

- **修复方案（推荐，用户已确认方向）：**
  1. **前端 SR-conf 目标区改为“规则名称”输入**：
     - 不再要求选择已有规则实体；
     - 在装配页（顶部目标选择区，见 R22-03）为 SR-conf 提供 `规则名称` 输入框，默认可填写日期/时间规则名；
     - `targetReady()` 对 SR-conf 改为校验“规则名称非空”；
     - `buildPreflightMissing` 不再要求已有规则实体，改为仅提示“规则名称未填写”（如需要）。
  2. **后端支持无 `RuleID` 的 SR-conf**：
     - `GenerateInput` 增加 `rule_name` 字段；
     - `Preview` / `loadData` / `validate` 对 SR-conf 允许 `RuleID == 0`；
     - `Render` 不依赖已有规则即可完成预览。
  3. **生成时自动创建新规则**：
     - 在 `backend/internal/server/assembly.go` 的 `generate`/`resolveOwner` 中，若 `TargetSyntax == SrConf && RuleID == 0`：
       - 调用规则服务创建新规则（`client_type="shadowrocket"`、schemes 默认空、名称为用户输入的规则名称）；
       - 使用新规则 ID 作为版本 owner，创建装配版本并写入 `assembly_blueprints.rule_id`；
       - 创建失败时回滚/清理空规则记录，避免残留。
     - 若从蓝图“重新编辑”进入，仍可携带已有 `rule_id`，此时沿用原规则继续追加版本。
  4. **同步调整文案**：
     - 装配页 SR-conf 不再显示“前往规则管理创建空规则”；
     - 生成成功后引导“前往规则版本管理/激活”（保留现有 `goActivation` 路径）。
  5. **补充测试**：
     - 后端：无预建规则情况下 `Preview`/`Generate` SR-conf 成功，并自动创建规则；
     - 前端：SR-conf 不再因 `rules` 为空而阻塞；规则名称必填校验生效。

- **状态：** 修复方案已确认（见 §〇.1），待开始实施（2026-08-26 仅记录，未改代码）

---

## 二、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-26 | 创建 Issue7，承接 Issue6（R21 系列）；当前为空模板，等待后续讨论记录 |
| v1.1 | 2026-08-26 | 新增 R22-01：素材池大数据量同步长时间占用 SQLite 写锁，导致 `POST /api/admin/pools/1/sync` 报 `database is locked`；根因为单事务内完成大量插入/删除且临时 keep 表无索引；用户确认推荐“短事务分批 + 临时表索引”修复方向；本次仅记录，未改代码 |
| v1.2 | 2026-08-26 | 新增 R22-02：版本管理“重新编辑”按钮请求 `/api/admin/versions/0/blueprint` 返回 400；根因为 `ListVersions` 查询未返回 `v.id`，列表 `id` 恒为 0；推荐后端补齐 `v.id` 并补单测；本次仅记录，未改代码 |
| v1.3 | 2026-08-26 | 新增 R22-03：订阅装配面板第一步“类型与目标”与副 Tab 四类装配器重复；用户曾先选“合并进头部”后撤回，最终确认“移到步骤条上方独立选择区”；本次仅记录，未改代码 |
| v1.4 | 2026-08-26 | 新增 R22-04：默认三个平台“产物格式”在编辑页不应可选，应固定随平台；用户确认采用新增 `platforms.is_default` 标记来识别默认平台；本次仅记录，未改代码 |
| v1.5 | 2026-08-26 | 新增 R22-05：日志页应展示平台/资源显示名称与用户昵称，并新增切换按钮在“显示名称 ↔ 唯一资源值”之间切换；推荐后端返回 `platform_name/resource_name/user_email` 并在前端统一切换；本次仅记录，未改代码 |
| v1.6 | 2026-08-26 | 新增 R22-06：装配“节点与代理组”步骤应移除未勾选预设组常显的 `preset` 标签，并在高级模式关闭时隐藏 Xray 节点板块；本次仅记录，未改代码 |
| v1.7 | 2026-08-26 | 新增 R22-07：节点“选择与排序”弹窗应移除单个节点“移除”按钮，并将“可选节点”勾选区放到排序区上方；本次仅记录，未改代码 |
| v1.8 | 2026-08-26 | 新增 R22-08：SR-conf 装配不应前置要求已有规则实体，应允许直接创建新规则；用户确认采用“装配器输入规则名称，生成时自动创建规则”；本次仅记录，未改代码 |
| v1.9 | 2026-08-26 | 整合 R22-01 ~ R22-08 最终修复方案到 §〇.1，并确认关键决策：全部修复、默认平台 product_type 后端拒绝修改、日志成功下载补平台标识、高级模式关闭剔除 Xray、目标选择区常驻卡片；仍仅记录方案，未开始改代码 |
