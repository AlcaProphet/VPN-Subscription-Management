# Issue2.md — VPN 订阅管理系统 问题追踪（已归档）

> **文档定位：** 本文档是 VPN 订阅管理系统的**已归档问题记录**（记录错误与修复方案，非强制，经验参考），承接已存档的 [Issue1.md](Issue1.md)。
>
> **归档说明（2026-08-24）：** R12~R15 系列问题均已修复或迁移至后续 Issue（R15-07/08/13/14 已迁移至 Issue3），不再作为当前问题追踪。
> 设计记录见 [Design2.md](../Design/Design2.md) 与 [Design2-UI.md](../Design/Design2-UI.md)；构建方案见 [Build4.md](../Build/Build4.md)（已归档）；编码指令见 [AGENTS.md](../../../AGENTS.md)（**唯一强要求**）。

---

## 一、进行中问题

### R12-01 素材池同步任务未固化提交时 URL 快照，编辑 URL 可能影响已启动任务

- **现象：** `backend/internal/pool/sync.go` 的 `SubmitSync` 仅在同一事务内检查池存在性与 running 任务并插入 `pool_sync_tasks`，随后 `go s.runSyncTask(poolID, taskID)`；`runSyncTask` 启动后才从 `rule_pools.urls_json` 读取 URL 列表。若管理员在提交同步后、goroutine 读取前编辑池 URL，本次任务会使用新 URL，而不是提交时的 URL。
- **根因：** 任务提交时没有把 URL 列表作为快照传给后台任务；Build4 Step5 明确要求「删除池或编辑池 URL 不取消已启动任务；URL 编辑只影响下一次同步」。
- **影响范围：** 素材池同步结果可能不符合管理员点击「同步」时的意图，存在差量删除/新增与预期不一致的竞态。
- **修复方案（待实施）：** 在 `SubmitSync` 的 `BEGIN IMMEDIATE` 事务内读取 `urls_json`，将 URL 快照作为参数传入 `runSyncTask`（或写入任务行快照字段），后台任务只使用该快照；编辑池仅影响后续提交的任务。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-02 素材池同步回执 removed 恒为 0，且“逗号多于两段”的 skip 原因未记录

- **现象：** `PerURLResult.Removed` 字段仅声明，从未赋值；`runSyncTask` 执行差量删除后没有回写各 URL 的删除数量，`GET /api/admin/pools/:id/sync/status` 与同步历史中的 `removed` 永远为 0。同时 `parser.go` 对 `TYPE,VALUE,extra` 行返回 skip 原因，但 `parseURLBody` 丢弃该原因且不计入 `skipped`，与 Design2 §2.3「逗号多于两段时仅取前两段，多余段忽略并记录」不一致。
- **根因：** 后端同步统计与解析逻辑未完整实现 Build4 Step5 的契约：差量删除未统计 removed；解析跳过原因未贯通到 per_url 回执。
- **影响范围：** 前端同步回执无法展示真实的删除/跳过明细，影响管理员对同步结果的判断；Build4 Step6 要求「逐 URL 结果回执（含 added/removed/skipped）」。
- **修复方案（已确认）：** 新增 `pool_entries.source_url` 列（增量迁移 1010），同步写入每条 URL 条目的来源；差量删除时按 `source_url` 精确统计各 URL 的 `Removed` 并写入 `PerURLResult.Removed`；`parseURLBody` 对多余逗号段行计入 `skipped`，并新增 `PerURLResult.SkipReasons` 记录 skip 原因摘要，前端同步历史展示逐 URL 明细。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-03 素材池前端未按 Design2-UI §5.2 完成双态列表、分段标题与时间选择器

- **现象：** `PoolTab.vue` 池列表只有 `a-table`，没有 `<768` 卡片态；`PoolDetail.vue` 条目列表也只有 `a-table`，且没有「手动条目（前段）/ URL 同步条目（后段）」分隔标题行；池新建/编辑弹窗的定时同步时刻使用 `Input` 文本框，未使用 Build4 Step6/UI 要求的 `a-time-picker`。
- **根因：** 素材池前端实现只覆盖了桌面表格基本功能，未落地 Design2-UI §5.2.1/§5.2.2 的移动端双态与分段展示规格。
- **影响范围：** 移动端素材池列表/详情可用性与信息层级不符合设计；定时时刻输入缺少时间选择器校验体验。
- **修复方案（待实施）：** 池列表与详情条目列表补 `<768` 卡片分支；详情条目列表增加 manual/url 段间分隔标题行；池编辑弹窗改用 `a-time-picker`（默认 04:00，UTC 说明保留）；详情顶部信息条按 UI 补「编辑」入口与面包屑式返回。
- **状态：** ✅ 已修复（2026-08-20，前端构建/单测通过）

### R12-04 素材池同步轮询的卸载取消、409 特判与历史逐 URL 明细不完整

- **现象：** `PoolTab.vue` 声明了 `pollHandles` 但没有 `onUnmounted` 统一 `cancel()`，组件卸载后轮询仍继续；`PoolDetail.vue` 的 `doSync` catch 未对 `ApiError(409)` 做 `message.warning` 特判；同步历史列表只展示状态、时间与错误 Tooltip，没有逐 URL 明细摘要。
- **根因：** Build4 Step6 的轮询卸载取消、进行中再触发 warning、同步历史逐 URL 明细摘要三项交互未完整落地；`pollHandles` 仅为残留声明。
- **影响范围：** 用户切走页面后仍产生轮询请求；详情页重复触发同步时提示形态不符合 UI §5.2.3（应为 warning）；历史任务无法直接查看各 URL 成功/失败明细。
- **修复方案（待实施）：** `PoolTab` 增加 `onUnmounted` 遍历 `pollHandles` 调用 `cancel()`；`PoolDetail` 捕获 `ApiError` 且 `status === 409` 时 `Notify.warning('同步进行中，请等待完成')`；同步历史每行渲染 `per_url` 摘要（成功/失败、added/removed/skipped、失败原因）。
- **状态：** ✅ 已修复（2026-08-20，前端单测通过）

### R12-05 Build4 Step6 前端单测覆盖未达验收要求

- **现象：** 现有 `frontend/tests/pool-tab.spec.ts` 与 `request-poll.spec.ts` 仅覆盖空态/列表展示与 pollTask 基本终态/取消/网络失败；未覆盖 Build4 Step6 要求的「进行中重复触发提示」「同步历史列表分页」「组件卸载取消轮询」等场景。
- **根因：** 测试随实现同步简化，未按 Build4 Step6 验收标准补齐素材池交互用例。
- **影响范围：** 上述前端交互缺陷缺少回归保护，后续 Build5/6/7 改动时容易再次破坏。
- **修复方案（待实施）：** 补充 `PoolTab`/`PoolDetail` 相关单测：进行中再次点击提示 warning、同步历史分页加载、组件卸载调用 poll cancel。
- **状态：** ✅ 已修复（2026-08-20，前端单测通过）

---

### R12-06 素材池无历史任务时同步状态接口返回 404，空状态分支不可达

- **现象：** `GET /api/admin/pools/:id/sync/status` 在池存在但尚无任何同步任务时返回 `404 素材池不存在`；`server/pool.go` 中 `syncStatus` 虽写了空状态 `{task_id:0,status:""}` 分支，但 `pool.GetStatus` 对 `sql.ErrNoRows` 直接返回 `ErrNotFound`，该空分支实际永远不可达。
- **根因：** `GetStatus` 未区分“无任务”与“池不存在”，接入层按同一 404 处理。
- **影响范围：** 前端首次进入详情/轮询前无法取得空状态，可能误报资源不存在；与 Build4 Step5/UI 的状态契约不符。
- **修复方案：** `GetStatus` 对无任务返回 `nil,nil`（或新增 `ErrNoTask`），接入层对无任务返回空状态；对池不存在仍 404。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-07 素材池无 URL 时同步会清空全部 URL 段条目

- **现象：** 对 `urls_json=[]` 的池调用 `POST /sync`，`runSyncTask` 因“全部 URL 成功”的真空条件进入差量删除，临时 keep 表为空，导致所有 `source='url'` 条目被删除，任务却记为 `succeeded`。
- **根因：** `runSyncTask` 未对 `len(urls)==0` 做保护，空 URL 列表被当作全成功处理。
- **影响范围：** 管理员误同步空池时丢失已有 URL 同步条目，与“空响应视为失败/零有效条目保护”的语义不一致。
- **修复方案（已确认）：** 无 URL 时任务直接返回 `failed`，错误信息为“未配置 URL”，不进入差量删除分支，保留旧数据。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-08 素材池规则类型未统一大写规范化，远程小写类型原样入库

- **现象：** `ValidateEntry` 内部用 `strings.ToUpper` 判断白名单，但只返回规范化后的匹配值，不返回规范化类型；`CreateEntry`/`parseURLBody` 仍用原始 `ruleType` 入库。远程文件若写 `domain,example.com`，池条目会存为 `domain`，装配产物输出 `domain,example.com,...` 而非 `DOMAIN,...`。
- **根因：** 类型规范化只在校验函数内部生效，未贯通到入库与渲染。
- **影响范围：** 规则类型大小写不一致，可能影响客户端识别；违反 Build4 Step5/Design2 白名单口径。
- **修复方案：** 让 `ValidateEntry` 返回规范化后的类型（或所有入库点统一 `strings.ToUpper`），并补小写类型入库单测。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-09 素材池回执 added 统计为解析条目数而非实际新增数

- **现象：** `syncURL` 将 `r.Added = len(entries)`，即使条目已存在或与 manual 冲突未真正插入，也计为新增；`removed` 仍恒为 0（后者已在 R12-02 记录）。
- **根因：** 未按实际 INSERT/UPDATE 行数统计 added。
- **影响范围：** 同步回执“新增”数量虚高，管理员无法准确判断差量变化。
- **修复方案（已确认）：** `added` 只统计实际 INSERT 的新增行数；已存在条目、与 manual 冲突未插入的行不计入。可在入库事务中按 URL 统计 `RowsAffected` 或插入前后计数，写回 `PerURLResult.Added`。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-10 素材池列表/历史端点对不存在池返回 200 空列表

- **现象：** `GET /api/admin/pools/999/entries` 与 `GET /api/admin/pools/999/sync/tasks` 对不存在的池返回 `200 {list:[],total:0}`，而非 404。
- **根因：** `ListEntries`/`ListTasks` 未先校验池存在性。
- **影响范围：** 管理端 API 语义不一致，前端可能把不存在池误当空池。
- **修复方案：** 两个列表方法先校验池存在，不存在返回 `ErrNotFound`，接入层映射 404。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-11 素材池同步启动阶段查询失败会遗留 running 任务

- **现象：** `runSyncTask` 启动时 `SELECT urls_json` 若因非“无此池”原因失败（如数据库瞬时错误），函数仅记日志直接 return，任务行保持 `running`，后续同步永久被 `ErrSyncRunning` 阻塞。
- **根因：** 启动阶段缺少失败终态写回。
- **影响范围：** 同步任务卡死，需人工修库；违反“所有 error 必须处理”的执行约束。
- **修复方案：** 启动查询失败时尝试将任务置为 `failed` 并写错误信息（池仍存在时），再返回。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-12 Clash 渲染未保留头部表单键序

- **现象：** `renderClash` 直接遍历 `map[string]any` 输出头部，`toYAMLNode` 对 map 也无序；Build5 Step3 明确要求用 `yaml.MapSlice` 或自定义有序结构“必须保留管理员填写顺序”。
- **根因：** 使用 Go map 导致键序随机。
- **影响范围：** 同一输入多次生成的 Clash YAML 头部字段顺序不稳定，不符合模板/用户填写顺序要求。
- **修复方案：** 改用有序头部结构（`yaml.MapSlice`/自定义 `orderedMap`），并在 golden 测试中断言键序。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-13 装配蓝图未持久化 platform_id/rule_id，重新编辑无法回填目标

- **现象：** `SaveBlueprintTx` 的 `fixed_params_json`、`selection_json`、`render_plan_json` 均未保存 `platform_id`/`rule_id`；`AssemblyView.loadEditIfAny` 因此把 `platform_id`/`rule_id` 置为 undefined，重新编辑无法恢复目标平台/规则。
- **根因：** 蓝图四列映射遗漏装配目标信息。
- **影响范围：** Build5 Step4/Step6 的重新编辑流不可用；从版本页进入装配后必须重新选择目标。
- **修复方案（已确认）：** 新增 `assembly_blueprints.platform_id`、`assembly_blueprints.rule_id` 两列（增量迁移 1010），并用 `versions` 的 owner 信息回填历史蓝图；`SaveBlueprintTx` 写入目标；blueprint 接口返回；前端重新编辑从 blueprint 回填 `platform_id`/`rule_id`。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-14 blueprint 端点未校验悬空引用与 name_changed

- **现象：** `GET /api/admin/versions/:id/blueprint` 始终返回 `invalid_refs: []`、`name_changed: null`，未读取当前节点/代理组/素材池校验快照引用是否失效，也未对比显示名变化。
- **根因：** Build5 Step4 的引用校验与名称对照未实现。
- **影响范围：** 重新编辑时无法提示失效节点/组/显示名变化，前端“失效引用红标/一键剔除”失去数据支撑。
- **修复方案：** 按蓝图中的 `node_names/group_names/pools` 与当前库比对，返回 `invalid_refs`；对比快照名与当前 `render_name` 返回 `name_changed`。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-15 装配校验缺少 overseas_members 子集与素材池存在性校验

- **现象：** `validate` 对 Clash 只检查 `OverseasMembers` 非空，不校验成员是否为已勾选/存在的节点；`loadPoolEntries` 对不存在的池返回空条目且不报错，导致“🌎国外流量”可能渲染为空、不存在的池静默无规则。
- **根因：** 输入存在性与子集校验不完整。
- **影响范围：** 可生成空“🌎国外流量”组或静默丢规则的产物，绕过空产物硬校验。
- **修复方案：** 校验每个 `overseas_members` 必须属于 `node_names` 且节点存在；校验每个 `pool_id` 在 `rule_pools` 中存在。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-16 节点协议变更后旧敏感字段残留

- **现象：** `node.mergeSensitive` 会把旧协议的全部敏感字段复制到新 `protocol_json`，即使新协议已不再包含该字段；例如 vmess 改为 ss 后，旧的 `uuid` 密文仍保留，并可能被 Clash 渲染输出。
- **根因：** 协议变更未“等价整体重新填表、不保留不兼容旧字段”。
- **影响范围：** 节点参数脏数据、产物中可能出现无关/敏感残留字段；违反 Build5 Step1 协议变更口径。
- **修复方案：** 仅保留新旧协议同名字段（且输入为空时沿用旧密文），新协议不包含的旧敏感字段一律删除。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-17 vless REALITY 参数以字符串存储时无法解析，链接缺 pbk/sid

- **现象：** 协议注册表中 `reality-opts` 是 `text` 类型，前端按 JSON 字符串提交；`realityOpts` 只识别 `map[string]any`，字符串不会被解析，导致 SR/通用 vless 链接缺少 `pbk`/`sid`/`peer` 等 REALITY 参数。
- **根因：** 链接编码未兼容注册表实际字段类型。
- **影响范围：** REALITY 节点生成的订阅链接不完整，无法正常连接。
- **修复方案（已确认）：** 将 `reality-opts` 注册表字段改为结构化对象（`object`/JSON 编辑器），前端动态表单发送对象；`realityOpts` 直接处理 `map[string]any`，链接正常生成 `pbk`/`sid`/`peer` 等参数。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-18 节点创建/更新/切换接口返回未脱敏密文

- **现象：** `CreateManual` 返回 `ProtocolJSON` 为加密后的 `enc:v1:...`；`UpdateManual`/`SetDisplayName`/`SetEnabled`/`SetPublic` 返回 `getRaw` 也含密文，服务端均未像 `List`/`Get` 那样 `redactSensitive`。
- **根因：** 服务层对写操作响应未做统一脱敏。
- **影响范围：** 管理 API 响应泄露凭据密文（虽非明文，但破坏“敏感字段返回空串/***”的脱敏契约），前端若直接使用响应可能误存密文。
- **修复方案：** 所有对外返回的节点对象统一走脱敏（敏感字段置空），或在接入层统一 redact。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-19 代理组允许引用「🛟无法归属的流量」作为子组，超出 Build5 Step2 明确范围

- **现象：** `proxygroup.validateDefinitionWithDAG` 与前端子组选择器均允许 `🛟无法归属的流量` 作为子组；Build5 Step2 明确“子组引用允许 🚀直接连接、🌎国外流量（强制组常量）或 proxy_groups 中其他组”，未包含兜底组。
- **根因：** 强制组白名单误包含全部三个强制组。
- **影响范围：** 可构造引用兜底组的代理组，语义上可能不符合设计（兜底组为 MATCH 终点）。
- **修复方案（已确认）：** 代理组不允许引用「🛟无法归属的流量」作为子组；后端子组引用白名单收窄为 `🚀直接连接`/`🌎国外流量`，前端子组选择器移除该选项，并补充“引用兜底组被拒绝”的单测。该强制组仍可作为规则目标组使用。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-20 Build5 Step5 节点/代理组前端交互未完整落地

- **现象：** `NodesView.vue` 的 `is_public` 开关切换前没有 UI 要求的 ConfirmModal；`ProxyGroupsView.vue` 只有上移/下移按钮，没有 ≥768 拖拽（`HolderOutlined`），也没有前端 DAG 即时检测与悬空引用红标剔除。
- **根因：** 前端实现只覆盖了基础列表/表单，未完成 Design2-UI §6/§7 的全部交互。
- **影响范围：** Build5 Step5 验收自检（危险确认、拖拽排序、DAG 提示）无法通过。
- **修复方案：** 补 `is_public` 切换确认；在 `useSortableList` 接入拖拽并在桌面端使用；增加前端环检测与悬空引用红标提示。
- **状态：** ✅ 已修复（2026-08-20，前端构建/单测通过）

### R12-21 Build5 Step6 订阅装配前端严重未达标

- **现象：** `AssemblyView.vue` 仍是单一通用表单：没有 `a-segmented` 分步/单页双形态、没有六步/五步流程、没有 Clash 头部默认值与“一键采用默认值”、没有 manual/xray 分组与 missing/allocatable 置灰、没有 Clash 手动规则排除 USER-AGENT、没有预览时拉取当前激活版本做 Diff、没有生成回执页，也没有重编辑失效引用剔除。无效 `?tab=` 也未回退 `pool`。
- **根因：** Build5 Step6 的装配器前端主体未按设计实现。
- **影响范围：** 四类装配器无法按设计走通，Build5 里程碑前端验收不达标。
- **修复方案：** 按 Build5 Step6 完整实现四类装配器子组件、步骤流、预览 Diff、生成回执与重新编辑流。
- **状态：** ✅ 已修复（2026-08-20，前端构建/单测通过）

### R12-22 Build5 Step7 装配入口与重新编辑参数未接通

- **现象：** `SubscriptionsView`“前往装配”未带 `platform_id`，`RulesView`“装配生成”未带 `rule_id`，`AssemblyView` 也未读取这些 query 参数；`VersionManageView.reEditUrl` 对非 rule 蓝图一律使用 `clash-yaml`，导致 sr-subs/generic-subs 重新编辑打开错误装配器。
- **根因：** 列表页/版本页与装配页之间的 query 契约未实现。
- **影响范围：** 用户从订阅/规则/版本页进入装配无法预选目标，重新编辑可能进入错误页签。
- **修复方案：** 各入口传递 `platform_id`/`rule_id`，`AssemblyView` 读取并预填；`reEditUrl` 通过 blueprint 的 `target_syntax` 决定页签。
- **状态：** ✅ 已修复（2026-08-20，前端构建/单测通过）

### R12-23 装配蓝图列映射不完整（FINAL 方向未入 fixed_params、缺 Xray 候选集）

- **现象：** `SaveBlueprintTx` 仅把 `final_direction` 放入 `selection_json`，未按 Build5 Step4“fixed_params_json（SR conf 含 FINAL 方向）”写入；`selection_json` 也未包含“Xray 候选集”字段。
- **根因：** 蓝图四列映射与 Build5 文档不完全一致。
- **影响范围：** 蓝图持久化不完整，后续重新编辑/Build6 恢复时缺少必要上下文。
- **修复方案：** SR conf 生成时把 `FINAL` 方向并入 `fixed_params_json`；`selection_json` 增加 Xray 候选集字段（本 Build 可空数组）。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R12-24 链接编码未将 url.Values.Encode 的 `+` 替换为 `%20`

- **现象：** `links.go` 多处直接使用 `url.Values.Encode()`，未按 Build5 Step3 要求将 `+` 替换为 `%20`；含空格凭据/参数生成的链接中空格会编码为 `+`。
- **根因：** 缺少统一的后处理。
- **影响范围：** 含空格密码等参数的节点订阅链接可能被客户端错误解析。
- **修复方案：** 对 `Encode()` 结果统一 `strings.ReplaceAll(s, "+", "%20")`，并补含空格参数单测。
- **状态：** ✅ 已修复（2026-08-20，后端单测通过）

### R13-01 DiffView 未使用 jsdiff，手写 LCS 在大规模产物对比下有性能风险

- **现象：** Build5 Step6 要求 DiffView 使用 jsdiff `diffLines`（`diff` 依赖已安装于 `package.json`：`diff ^9.0.0` + `@types/diff`），但实际实现引用 `frontend/src/lib/diff.ts` 的手写 LCS 算法；`diff` 依赖全仓无任何 import（且 node_modules 中从未实际安装，仅 lock 锁定）。手写实现分配 (n+1)×(m+1) 的 DP 数组，对 1 万行级订阅产物 diff 需约 10^8 个数字单元（数百 MB 内存），浏览器可能卡死或崩溃。
- **根因：** R12-21 修复时依赖未实际安装（注释「环境无 npm 依赖时本地实现」），改用本地实现后未切回 jsdiff。
- **影响范围：** 装配预览「与当前激活版本对比」在大规则量场景不可用；`diff`/`@types/diff` 成为死依赖。
- **修复方案（已实施，用户确认 2026-08-20）：** ① `npm install` 按 lock 安装 `diff`/`@types/diff`；② `DiffView.vue` 改为 `import { diffLines } from 'diff'`，删除手写 `src/lib/diff.ts`；③ 新增手动启动开关与防过载机制：新旧总行数 ≤ 5000 行自动计算，超阈值时不自动计算、显示警告与「仍要启动行级对比」按钮，由用户手动启动；④ 所有计算传 `timeout: 2000`，超时返回粗粒度结果并提示；⑤ 新增 `tests/diff-view.spec.ts` 覆盖三色渲染/targetMissing 整体新增/超大文本手动开关三场景。
- **状态：** ✅ 已修复（2026-08-20，验证：`npm run build` 与 `npm test`（35 passed）通过）

### R13-02 装配页未按 Build5 Step6 拆分子组件，步骤模板整段重复

- **现象：** Build5 Step6 item 5 要求在 `frontend/src/views/admin/assembly/` 下新建 `AssemblerShell/TypeTargetStep/HeaderStep/NodesGroupsStep/RulesStep/PreviewStep/GenerateStep` 七个子组件；实际全部逻辑集中在单文件 `AssemblyView.vue`（727 行），该目录下仅有 `PoolTab.vue`/`PoolDetail.vue`。每个步骤的「单页 Card 分支」与「分步 v-else 分支」模板逐字重复（六处大段复制）。另：生成回执「去版本管理激活」按钮跳 `/admin/subscriptions` 列表页，而非该订阅的版本管理页（Build5 Step6 口径为直达版本管理页）。
- **根因：** R12-21 修复时选择了单文件实现，未做组件拆分；重复模板违反 AGENTS.md §五「重复页面逻辑抽取为通用组件复用」。
- **影响范围：** 装配页可维护性差，模板双份易失同步；生成后用户需多一次点击进入版本管理。
- **修复方案：** 按 Build5 Step6 拆分七个子组件，单页/分步形态共用同一份步骤模板（仅外层容器不同）；回执按钮跳转 `订阅的版本管理页`（携带订阅 ID）。
- **状态：** ✅ 已修复（2026-08-20，前端构建/单测通过）

### R13-03 装配/节点包存在死代码与忽略 error 返回值

- **现象：** ① `internal/node/registry.go` 的 `ErrProtocolNotFound` 声明后无任何引用（`GetProtocol` 返回 `fmt.Errorf`）；② `internal/assembly/service.go` 的 `joinStrings` 无调用点；③ `internal/assembly/links.go:321` `var _ = strings.TrimSpace` 与 `render_sr.go:97` `var _ = node.ForceDirect` 为规避未用导入的残留；④ `render_sr.go` 的 `formatRuleLine(ruleType, value, target string, sr bool)` 的 `sr` 参数从未使用；⑤ `render_clash.go:126`、`render_sr.go:52/83` 三处 `planRaw, _ := json.Marshal(...)` 忽略 error 返回值；⑥ `internal/server/health.go:13` 残留 `TODO(Build3 Step 6)` 旧轮次注释。
- **根因：** 构建与 R12 修复过程中的遗留清理不彻底。
- **影响范围：** 违反 AGENTS.md「所有 error 必须处理，不可忽略返回值」与代码整洁要求；无功能影响。
- **修复方案：** 删除未用符号与 `var _=` 残留；删除 `formatRuleLine` 的 `sr` 参数；三处 `json.Marshal` 改为处理错误（返回/包装）；清理 health.go 的 Build3 TODO 或确认其归属。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 通过）

### R13-04 通用 vmess 链接 JSON 的 tls 字段输出 bool 字符串且缺 sni

- **现象：** `internal/assembly/links.go` generic vmess 分支中 `"tls": str(nd.ProtocolJSON, "tls", "")` 将注册表 bool 字段转为 `"true"`/`"false"` 输出；v2rayN 生态惯例为 `"tls"` 或空串，部分客户端按字符串值判定。同时未输出 `Node-Link-Standards.md` 字段表中的 `sni`（节点有 servername 时）。
- **根因：** `str()` 通用取值函数对 bool 直接 `FormatBool`，未针对 vmess tls 语义做映射。
- **影响范围：** 通用节点订阅的 vmess 链接在部分客户端可能不启用 TLS；缺 sni 影响 SNI 校验场景连接。
- **修复方案（用户已确认 2026-08-20）：** tls=true 输出 `"tls"`、false 输出 `""`；servername 非空时补 `sni` 字段；补对应单测。（决策后同步更新至 Build5 Step3 links.go 注记）
- **状态：** ✅ 已修复（2026-08-20，后端 assembly 单测通过）

### R13-05 SetEnabled/SetPublic 未预留 onXrayChanged 副作用钩子接口

- **现象：** Build5 Step1 item 2 要求「副作用钩子留接口 `onXrayChanged func(ctx, node, oldEnabled, oldPublic)`（Build6 注入）」；实际 `node.go` 的 `SetEnabled` 仅有 `_ = existing` 与注释「副作用钩子本 Build 留空」，`Service` 结构体无钩子字段，未定义钩子类型。
- **根因：** R12 系列修复未涉及该项，实现时以注释代替了接口预留。
- **影响范围：** Build6 接入 Xray 推送副作用时需再改 `Service` 结构与构造签名，接口契约未提前固化。
- **修复方案：** 在 `node.Service` 增加 `onXrayChanged` 函数字段与 `SetOnXrayChanged` 注入方法，`SetEnabled`/`SetPublic` 落库成功后携带旧值调用（本 Build 回调可为 nil）。
- **状态：** ✅ 已修复（2026-08-20，后端 node 单测通过）

### R13-06 openvpn 协议 SensitiveFields 为空数组

- **现象：** Build5 Step1 单测验收要求「每协议敏感字段非空且合法」，但注册表中 `openvpn` 的 `SensitiveFields` 为空（唯一字段 client-config 未视为凭据）。
- **根因：** client-config 为配置文本，实现时未当作凭据字段处理。
- **影响范围：** 与 Build5 验收措辞不一致；openvpn 客户端配置不加密存储。
- **修复方案（用户已确认 2026-08-20）：** 视为合理例外，保持现状；建议后续修订 Build5 Step1 验收措辞为「每协议敏感字段合法（openvpn 除外）」。
- **状态：** ✅ 已确认（2026-08-20，维持现状，无需代码修复）

### R13-07 preview 端点不返回 name_changed

- **现象：** Build5 Step4 要求 `/api/admin/assembly/preview` 返回 `{content, skipped, warnings, name_changed}`；实现中 `PreviewResult.NameChanged` 字段存在但 `Preview()` 从不赋值，响应恒为省略。仅 blueprint 端点实现了 name_changed。
- **根因：** 预览时尚无快照，name_changed 语义主要在重新编辑场景成立，实现时做了简化。
- **影响范围：** 与 Build5 Step4 文档字面不一致；预览阶段无显示名变化提示（重新编辑场景已由 blueprint 覆盖）。
- **修复方案（用户已确认 2026-08-20）：** 确认为合理简化，保持现状；建议后续修订 Build5 Step4 将 name_changed 标注为「仅 blueprint 端点返回」。
- **状态：** ✅ 已确认（2026-08-20，维持现状，无需代码修复）

### R13-08 Clash proxies 条目键序不稳定（map 遍历）

- **现象：** `render_clash.go` 的 `clashProxy` 返回 `map[string]any`，`toYAMLNode` 的 map 分支遍历无序，proxies 条目内 name/type/server/port 之后的协议字段键序每次渲染可能不同（R12-12 仅修复了头部键序）。
- **根因：** 头部已改 OrderedMap，但 proxies 条目仍用普通 map。
- **影响范围：** 不影响 Clash 客户端解析（YAML 键序无语义），但同一输入两次生成的产物可能出现虚假 diff，影响预览对比与版本 diff 的可读性。
- **修复方案：** `clashProxy` 改为有序结构输出（先固定 name/type/server/port，其余协议字段按键名排序或按注册表字段顺序）；golden 测试断言条目键序稳定。
- **状态：** ✅ 已修复（2026-08-20，后端 assembly 单测通过）

### R13-09 internal/cron 包无单元测试

- **现象：** Build4 Step5 测试命令含 `./internal/cron/...`，但 `internal/cron`（cleanup.go / pool.go）无任何测试文件，`go test` 输出 `[no test files]`。定时同步的每分钟判重（lastMinute/fired）、ErrSyncRunning 跳过等逻辑无回归保护。
- **根因：** Build4 Step5 item 9 单测清单未显式点名 cron 用例，实现时未补。
- **影响范围：** 定时同步逻辑回归风险；Build4 Step5 验收命令虽通过但实际无覆盖。
- **修复方案：** 将 `run` 逻辑抽出可注入时钟的函数，补「到期触发一次」「同分钟不重复」「running 跳过」单测（可用内存 store）。
- **状态：** ✅ 已修复（2026-08-20，后端 cron 单测通过）

---

### R14-01 后端缺 GET /api/admin/subscriptions/:id 路由，订阅版本管理页每开必报错

- **现象：** `frontend/src/api/subscription.ts:19` `getSubscription` 被 `VersionManageView.vue:43` 在订阅版本页每次打开时调用；`backend/internal/server/subscription.go:23-38` 仅注册 list/create/update/delete 与版本端点，无 GET /:id。请求落入 NoRoute（`static.go:68-75` 对 GET 返回 200+index.html），响应解包得 undefined，`sub.product_type` 抛 TypeError → 每开页弹错误提示，「装配生成」入口丢失 tab/platform_id 预置（退化为通用装配页）。
- **根因：** Build5 Step7 装配入口改造引入前端调用，后端对应路由未同步落地；NoRoute 对 /api/ 前缀 GET 返回 SPA 页面放大了故障形态。
- **影响范围：** 全部订阅的版本管理页；管理员每次进入均见错误提示且装配入口退化。
- **修复方案（用户已决策 2026-08-20）：** 后端补 `GET /api/admin/subscriptions/:id`（与 group/platform 的 GET /:id 同款模式，返回 SubscriptionItem）；同时 NoRoute 对 `/api/` 前缀 GET 未匹配改为返回 404 JSON（与非 GET 一致，SPA 回退不受影响）。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-02 Clash proxy-groups 条目键序不稳定（R13-08 同类遗漏面）

- **现象：** `render_clash.go:53-57`（三强制组）与 `:75`（勾选组）以 `map[string]any` 传入 `toYAMLNode`，其 map 分支（`:211-220`）Go 随机遍历；同一输入多次渲染 proxy-groups 条目出现多种 name/type/proxies 键序（已实测复现）。
- **根因：** R13-08 仅修复 proxies 条目（clashProxy 改 OrderedMap），proxy-groups 条目遗漏。
- **影响范围：** 同一输入重复生成产生虚假 diff，影响预览对比与版本 diff 可读性；不影响客户端解析（YAML 键序无语义）。
- **修复方案：** 组条目改有序结构输出（name/type/proxies 固定序），并补键序稳定单测（同 TestClashProxyKeyOrder 口径）。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-03 链接渲染静默丢弃注册表已填字段（anytls/tuic/wireguard）

- **现象：** `links.go` 渲染白名单缺项——anytls（`:73-85`）丢 `alpn`/`client-fingerprint`（registry.go:181-182 有字段）；tuic（`:123-133`）丢 `alpn`（registry.go:123）；wireguard（`:137` 参数白名单）缺 `pre-shared-key`（registry.go:136 有字段且为敏感字段）。Node-Link-Standards §2 对应行均含这些参数。
- **根因：** 注册表字段与链接渲染白名单未保持同步，逐协议映射遗漏。
- **影响范围：** 管理员已填写的 ALPN/指纹/预共享密钥不进订阅链接，对应节点连接参数缺失。
- **修复方案（用户已决策 2026-08-20，并入 R14-04 统一执行）：** 三协议渲染白名单补齐对应参数并补单测（含「注册表字段 vs 渲染输出」一致性遍历测试）。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-04 链接参数对 Node-Link-Standards 不全（registry 与渲染双缺）

- **现象：** 标准表列出但 registry 无字段、渲染亦无：hysteria `alpn`/`mport`/`insecure=1`、hysteria2 `alpn`、vless `alpn`、vmess（generic JSON）`alpn`/`fp`；反向地，trojan registry 声明 `alpn`（registry.go:85）但标准 trojan 行无此参数、渲染亦不支持（LinkMappings 声明与实际输出不一致）。
- **根因：** 注册表 schema 按常用子集定义，未与标准表逐项对齐。
- **影响范围：** 对应参数用户无从填写；trojan 的 LinkMappings 声明误导。
- **修复方案（用户已决策 2026-08-20）：** 全部按 Node-Link-Standards §2 补齐——registry 缺失字段（hysteria alpn/mport/insecure、hysteria2 alpn、vless alpn、vmess alpn/fp）与渲染输出一并补齐；trojan `alpn` 保留并补渲染支持。统一补双方向一致性单测。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-05 链接空参数集尾部多 `?`，http/socks5 空凭据输出 `:@`

- **现象：** trojan/anytls/hysteria/hysteria2/tuic/wireguard/http/socks5 均 `"?" + encodeQuery(q)` 无条件拼接（links.go:72/85/101/122/133/142/153/167），q 为空时产出如 `trojan://pw@host:443?#name`；http/socks5 空凭据仍输出 `:@` userinfo（`:144-167`）。
- **根因：** 缺「无参数不输出 `?`」「凭据有才输出」的后处理（Node-Link-Standards §2 公共规则与 http/socks5 行明示）。
- **影响范围：** 生成的链接与标准形态不符，部分客户端解析异常。
- **修复方案：** encodeQuery 结果为空时不拼 `?`；http/socks5 凭据均为空时不输出 userinfo 段；补对应单测。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-06 render_plan_json 不自包含，违反 Build5 Step3 与版本快照语义（Build6 Step4 前置依赖）

- **现象：** `render_clash.go:118-126` 的 plan 仅为 `head / manual_proxies(=in.NodeNames 名字数组) / proxy_groups(=in.GroupNames 名字数组) / rules(=in.Pools 池引用) / custom_rules / fallback`。
- **根因：** Build5 Step3 要求「自包含且能无状态重建全文」，实现退化为引用快照。
- **影响范围：** Build6 Step4 下载重渲染无法实施：manual 节点删除后无法重建 proxies 条目；代理组编辑/删除会改变历史版本下载内容；素材池再同步/删除使下载规则与已激活版本漂移（违反 Design2 §2.5「生成的版本为渲染时点快照」）。
- **修复方案（用户已决策 2026-08-20：随 Build6 Step4 实施时修）：** 扩展 plan 生成——manual proxies 存完整 Clash 条目（含解密字段，键为 nodes.name）、proxy_groups 存生成时点结构（名称+类型+有序成员）、rules 存冻结规则行（含 no-resolve）、fallback 两行。Build6.md Step4（v2.0）已钉死此前置修补口径。
- **状态：** ✅ 已修复（随 Build6 Step4 实施；2026-08-21 全量核查复核确认：自包含 ClashPlan 与 RenderClashPlan 已落地于 assembly/clash_plan.go）

### R14-07 selection_json.xray_candidates 恒写空数组（Build6 Step2 前置依赖）

- **现象：** `assembly/service.go:91` `"xray_candidates": []string{}` 硬编码（注释「Build6 注入前为空候选集」）。
- **根因：** Build5 预留字段未填充；Build6 候选集机制依赖该字段。
- **影响范围：** 若 Build6 按字段读取则候选集恒空 → RecomputeCandidateSet 会摘除全部组节点分配。
- **修复方案（用户已决策 2026-08-20：随 Build6 Step2 实施时修）：** 生成时按勾选节点中 source='xray' 子集填充（node.source 不可变，与 node_names∩xray 等价）。Build6.md Step2（v2.0）已钉死口径。
- **状态：** ✅ 已修复（随 Build6 Step2 实施；2026-08-21 全量核查复核确认：xrayCandidateNamesTx 已按勾选节点 source='xray' 子集填充）

### R14-08 RulesView「取消默认」无 ConfirmModal 直调接口

- **现象：** `RulesView.vue:181-192` `cancelDefault` 直接 `setHomeDefault(id,false)`（桌面 :237、移动 :266 同），无确认弹窗。
- **根因：** Build4 Step4 item 13 定稿口径（ConfirmModal「取消后首页分流规则卡片将回到未设置空态」）未落地。
- **影响范围：** 取消首页默认规则无二次确认，误点即生效。
- **修复方案：** 取消默认走 ConfirmModal（文案按 Design2-UI §4.6）确认后调接口。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-09 装配生成流不写 pooled_sub 标记，「已入池未生效」引导对装配流缺失

- **现象：** `pooled_sub_${id}` 仅在 `VersionManageView.vue:141,188`（上传/文本流）写入；装配生成成功流（`AssemblyView.vue:344-347`）只弹 Notify 与回执，不写标记 → 回订阅列表无高亮/无「去激活」入口。
- **根因：** Build5 Step7 item 2「上传/装配生成后」双口径只覆盖了上传流。
- **影响范围：** 装配生成的新版本回列表后缺少入池未生效引导，与上传流行为不一致（Design2 §4.4 / UI §4.1）。
- **修复方案：** 装配生成成功且 auto_activated=false 时，按目标订阅写入 `pooled_sub_${subscriptionId}`（订阅 id 由目标平台唯一条目解析）。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-10 PoolTab 桌面表格未做断点互斥，<768 表格与卡片同时渲染

- **现象：** `PoolTab.vue:162` 桌面 `Table` 无 `hidden md:block`，而卡片栅格 `:206` 为 `md:hidden` → <768 两态同时渲染。
- **根因：** 双态互斥类名漏加。
- **影响范围：** 移动端池列表重复渲染（表格+卡片），布局混乱。
- **修复方案：** 表格容器加 `hidden md:block`（与 NodesView 等页同款互斥）。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-11 订阅管理两处规格偏差

- **现象：** ① 桌面表格态「已入池未生效」仅行底色+「去激活」按钮，无行内 a-alert info 风格标签（移动卡片有，SubscriptionsView.vue:150-183；UI §4.1 口径为行内 a-alert 标签）；② 删除订阅 ConfirmModal 影响清单缺「不级联自定义订阅」项（:236，§4.1 四项清单之一）。
- **根因：** Build5 Step7 item 2/Build4 Step4 item 10 的两处 UI 细节未完整落地。
- **修复方案：** 桌面行内补 a-alert 标签；删除清单补「不级联自定义订阅」。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-12 版本管理页三处规格偏差

- **现象：** ① 移动端卡片「更多 ▾」菜单（VersionManageView.vue:113-125）无「重新编辑」项（桌面 :296 有）；② 空态引导「暂无版本，可通过上传文件 / 在线编辑 / 装配生成创建」为静态串，分享/自定义页也显示「装配生成」（:272；UI §10.2 限定订阅与规则页）；③ 「装配生成」按钮在页头（:266）而非 §4.2 的创建弹窗页签栏右侧。
- **修复方案：** 移动端菜单补「重新编辑」；空态引导按 ownerType 条件渲染；按钮位置按 §4.2 迁移（或确认维持现状并回写 Design2-UI §4.2）。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-13 平台编辑与规则空实体两处引导缺失

- **现象：** ① `PlatformEditView.vue:130-131` 的 400 冲突走全局 Notify.error 而非 §4.4 表单级内联（后端文案正确，platform.go:41-50）；② `RulesView.vue:223,257` 空实体行缺「可作为 SR 分流规则装配目标」Tooltip/副文案（§4.6，现仅创建流程内有引导）。
- **修复方案：** 400 改表单级展示；空实体行补 Tooltip/副文案。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-14 节点/代理组管理页 UI 细节簇

- **现象：** ① manual 标签色 blue 应为灰（NodesView.vue:194,226；§6.1 manual 灰/xray 紫）；② 非 missing xray 删除置灰缺 Tooltip「该入站仍存在于 Xray 实例，请先删除 Xray 入站并刷新节点检测」（:216）；③ manual 名称禁空格无前端实时校验（:68-84,243）；④ 命名 409 为通用 Notify 非字段级（:142-143）；⑤ preset/custom 标签色颠倒（ProxyGroupsView.vue:202,228：实 preset 紫/custom 蓝，§7.1 定 preset 蓝/custom 绿）；⑥ 预设启用 Checkbox 在独立列而非行首（:209-213）；⑦ DAG 环检测在保存时而非选择子组时即时拒绝，文案不符 §7.2（:125-134）；⑧ 拖拽无 HolderOutlined 手柄且未按 ≥768/<768 断点门控（useSortableList.ts；ProxyGroupsView.vue:260-267）。
- **修复方案：** 逐项按 Design2-UI §6.1/§7.1/§7.2/§1.2 口径收口。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-15 装配页 UI 细节簇

- **现象：** ① TypeTargetStep 无「无匹配平台空态引导+直达平台管理链接」（TypeTargetStep.vue:24-26；§5.3.0）；② sr-conf 目标规则实体缺「新建空规则实体」快捷入口（:19-23；§5.3.4）；③ Clash 头部默认值未预填（fixed_params_text 初始 `'{}'`，仅「一键采用默认值」按钮应用，AssemblyView.vue:67；Build5 Step6 要求预填）；④ RulesStep 池有序列表无桌面端拖拽（RulesStep.vue:42-43；§5.3.1）；⑤ PreviewStep 缺 `# {{xray_nodes}}` 占位旁 Tooltip「下载时按用户分配节点动态注入」（§5.3.5）；⑥ 重编辑顶部 alert 缺版本号 vN（AssemblyView.vue:393；§5.4）；⑦ Build5 Step6 item7 所列装配器单测（页签回退/步骤跳过/表单校验/preview 调用/失效剔除）未落地，tests/ 无装配页 spec；⑧ 池弹窗 URL 无前端 http/https 校验（PoolTab.vue:58-78，仅后端 400 兜底）。
- **修复方案：** 逐项按 §5.3/§5.4/Build5 Step6 收口；补装配页 spec。
- **状态：** ✅ 已修复（2026-08-20，RulesStep 桌面拖拽、装配页 spec、重编辑 vN 提示已补齐；后端 build/vet/test 与前端 build/test 通过）

### R14-16 server/home.go 与 profile.go 接入层直接 SQL（AGENTS §五分层红线）

- **现象：** `server/home.go` 12 处、`server/profile.go` 5 处 `h.store.DB().QueryContext/QueryRowContext` 直查 platforms/subscriptions/custom_subscriptions/users 等表；`ProfileHandler.users` 字段构造时未注入且从未读取（profile.go:19，staticcheck U1000）。
- **根因：** Build1~Build4 期间首页/个人中心查询就地编写，未下沉业务层。
- **影响范围：** 违反「接入层不直接操作存储」强约束；业务规则散在接入层，难以复用与测试。
- **修复方案（用户已决策 2026-08-20：本轮一并整改）：** 查询下沉业务层（home 查询并入既有服务或新建 internal/home），profile 走 user 服务；移除 ProfileHandler.users 死字段。
- **状态：** ✅ 已修复（2026-08-20，新增 internal/home 下沉首页全部查询；server 接入层已无直连 SQL；后端 build/vet/test 通过）

### R14-17 CopyField 组件 0 引用、PageHeader 复用不足

- **现象：** `components/CopyField.vue` 全仓 0 引用（含 tests），`HomeView.vue:87-118` 内联实现复制逻辑（Design2-UI §3.1.2 规定复制规则链接用 CopyField）；`PageHeader` 仅 3/13 管理页使用，其余页内联 `<h2>`（Build4 Step4 item 17「后续页面统一使用，禁止各页继续复制实现」）。
- **修复方案：** HomeView 复制规则链接/平台卡链接改用 CopyField；新页面统一 PageHeader（旧页内联头逐步收口）。
- **状态：** ✅ 已修复（2026-08-20，HomeView 已接入 CopyField；管理页 PageHeader 全量复用完成；前端 build/test 通过）

### R14-18 ValidateNodeName/ValidateProxyGroupName 复制实现未共享基础函数

- **现象：** `node.go:100-138` 两函数为逐行复制实现，仅空格策略不同；Build5 Step2 要求「共享基础字符集校验函数（禁止复制两份不同实现），空格策略在调用侧参数化」。
- **影响范围：** 行为等价、无功能影响；结构不符合文档要求，后续改字符集需双处同步。
- **修复方案：** 抽 `validateName(name, allowSpace bool)` 基础函数，两导出函数参数化调用。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-19 error 处理三处遗漏

- **现象：** ① `assembly/models.go:115` `kb, _ := json.Marshal(k)`（实际不可触发，违约束字面）；② `pool/sync.go:332` failTask `_ = s.store.TxImmediate(...)` 兜底终态写回失败无任何感知；③ `rule/rule.go:112` rollbackRecord 首个 DELETE `_, _ =`（边界项，可类比清理白名单）。
- **修复方案：** ①② 改为处理 error（failTask 失败至少记 error 日志）；③ 随修复一并确认口径。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-20 死代码一批

- **现象：** `oidc/flow.go:258` `var _ = log.Info` 残留（R13-03 同类）；`custom.go:148/163` Get/ListByUser；`log/log.go:36/38` 包级 Warn/Debug；`registry.go:277` ProtocolNames；`pool.go:181` Get；`rule.go:229/248` Get/Name；`share.go:225` Name；`store.go:197` IsNoRows；`server/profile.go:19` ProfileHandler.users 死字段；`subscription_test.go:70/81` 死测试辅助 seedPlatform/timeNowUnix；`api/subscription.ts:30` checkSlug；`api/request.ts:64` handleApiError。（`node.SetOnXrayChanged` 属 Build6 预留，不计。）
- **修复方案：** 逐项删除（确认无 Build6/7 消费计划者）；profile 死字段随 R14-16 一并处理。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-21 request.ts 429 Retry-After 提示分支不可达

- **现象：** `request.ts:68-70` 构造 ApiError 时未从 `err.response.headers` 提取 `retry-after`，`retryAfter` 恒 undefined，「请 N 秒后重试」提示永不显示（后端 ratelimit.go:53 确实设置了该头）。
- **修复方案：** 响应拦截器内提取 `retry-after` 头写入 ApiError。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-22 旧模型注释残留三处

- **现象：** ① `download.go:22` ErrUnassigned 注释残留「组未选定」旧语义（实际为「平台无订阅条目」）；② `platform.go:494-495/553` Delete 注释引用已 DROP 的「组的关联与选定/needs_reselect」；③ `PlatformEditView.vue:128` 注释残留「为各用户组选定」。
- **修复方案：** 更新为新模型语义注释。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-23 R12-03 部分回潮：PoolDetail 条目列表 <768 卡片分支未落地

- **现象：** R12-03 修复方案含「详情条目列表补 <768 卡片分支」且已标 ✅；实际 `PoolDetail.vue` 条目区仅 a-table（:186 起），全文件无 md:hidden / hidden md:block 双态分支（manual/url 分段标题行已落地，不受影响）。
- **修复方案：** 条目列表补 <768 卡片分支，或确认表格横滚为可接受形态并回写 R12-03 记录。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-24 路由守卫单测未覆盖系统状态未加载场景

- **现象：** `tests/router-guard.spec.ts:53-64` 仅覆盖 advanced_mode:false，未覆盖 status=null（未加载）时 `!== true` 判定（Build4 Step4 验收标准点名「含系统状态未加载场景，前端路由单测覆盖」）。
- **修复方案：** 补 status=null 重定向用例。
- **状态：** ✅ 已修复（2026-08-20，后端 build/vet/test 与前端 build/test 通过）

### R14-25（已决策）config.Get 错误的系统性静默

- **现象：** 约 40 处 `v, _ := s.cfg.Get(ctx, key)`（config/admin.go、mail、oidc、captcha、emergency、server 等）；config.Get 对缺 key 返回 ("",nil) 属设计，但真实 DB 错误同样被静默为「未配置」。
- **根因：** 配置读取的 fail-safe 惯例。
- **影响范围：** 低危（方向安全）；DB 故障时配置静默回退默认，可能掩盖故障。
- **修复方案（用户决策 2026-08-20）：** 对关键路径改为显式错误——签名密钥读取、advanced_mode 读取（高级模式开关判定）与相关鉴权/同步钩子入口优先显式返回错误或告警；非关键展示类配置（公告、站点名、邮件参数等）维持 fail-safe 并补口径注释。纳入 Build4/5 修复收口执行。
- **补充记录（用户决策 2026-08-20）：** `config/admin.go` 的面板配置读取、`oidc.IsConfigured()` 等暂不改为显式报错，维持 fail-safe / fail-closed；后续如需纳入关键路径再单独处理。
- **状态：** ✅ 已修复（2026-08-20，关键路径已显式报错：签名密钥、advanced_mode、configured、本地登录/注册开关、OIDC 审批开关；非关键展示类维持 fail-safe 并补注释；后端 build/vet/test 通过）

### R14-26（已确认）node/proxygroup 先读后写 TOCTOU 窗口

- **现象：** `node.go:368-425` SetEnabled/SetPublic 先 tx 外 getRaw 再 tx 内 UPDATE；`proxygroup.go:87-91,130-134` 引用校验 SELECT 在 TxImmediate 之外，校验与写入间存在窗口。
- **影响范围：** 低危（单连接 SQLite 窗口极小；node 有重读 NotFound 兜底，装配期有 invalid_refs 二次校验兜底）。
- **修复方案（用户决策 2026-08-20）：** 维持现状；在 Issue2/设计结论中记录「单连接 + 重读 NotFound 兜底 + 装配期 invalid_refs 二次校验」的合理解释，不改动代码。
- **状态：** ✅ 已确认（2026-08-20，维持现状，无需代码修复）


### R15-01 全部 Build6/7 异步长任务复用请求 Context，经 HTTP 提交后任务立即失败

- **现象：** 实际启动服务后调用 `DELETE /api/admin/xray/instances/:id`，接口正常返回 `{"task_id":"task-1"}`，随后 `GET /api/admin/tasks/task-1` 恒为 `failed`，error 为 `开启事务失败: context canceled`。同类实现还包括 `POST /api/admin/xray/init`、`/instances/:id/reconcile/push|clean|credentials`、`PUT /api/admin/settings/advanced`（关闭高级模式的 OFF 清空任务）与 v2 配置导入任务。
- **根因：** 各异步方法（`instance.go:211`、`sync.go:274`、`reconcile.go:210/232/261`、`offclear.go:50`、`config/export.go:261`）的 goroutine 直接使用 handler 传入的 `c.Request.Context()`；Go `net/http` 在 `ServeHTTP` 返回后即取消该 Context，而事务执行发生在 handler 返回之后，故必然失败。`pool/sync.go:192` 已使用 `context.Background()`，但 Build6/7 新增任务未沿用该模式。
- **影响范围：** Build6 Step1 实例删除、Step3 初始化、Build7 Step1 三项对账执行、Step2 OFF 清空与 v2 导入在真实 HTTP 路径下全部不可用；单测均直接传 `context.Background()` 调用业务方法，未覆盖 HTTP 生命周期，所以 `go test ./...` 全绿仍漏检。
- **修复方案（待确认）：** 在各异步服务入口统一使用 `context.WithoutCancel(ctx)`（Go 1.26 可用）或创建 `context.Background()` 派生上下文（保留日志字段），再传给任务 goroutine；同时补 HTTP 集成测试：提交后轮询任务必须达到 `succeeded`（对可完成场景）而非 `context canceled`。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-02 动态订阅渲染错误使用 gRPC API 端口作为节点端口

- **现象：** `backend/internal/server/render.go:77-78` 与 `:114` 用 `hostOf(t.APIAddr)` / `portOf(t.APIAddr)` 生成动态节点的 server/port；`Target` 结构中只有 `APIAddr`，没有节点 `port` 字段。Xray 的 gRPC API 端口与入站监听端口通常不同（例如 API=10085、入站=443）。
- **根因：** Build6 Step1 检测时已把 `inbound.Port` 写入 `nodes.port`，但下载渲染没有读取 `nodes.port`；`NodeRenderParams` 也只返回 protocol 与 protocol_json，不含端口。
- **影响范围：** 所有用户下载的 Clash/SR/generic 动态节点都指向 gRPC 管理端口而非代理端口，配置实际不可用；这是高级模式核心下载链路的阻断级缺陷。
- **修复方案（待确认）：** `Target` 增加 `Port int`（从 `nodes.port` 读取），`NodeRenderParams` 或 Target 查询一并返回端口，`renderUserSubscription`/`renderLinkLines` 改为使用节点端口；补端口来源单测（api_addr 端口 ≠ 入站端口）。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-03 节点检测 protocol_json 未提取 vless flow 与 shadowsocks cipher

- **现象：** 临时单测直接构造 vless（client Flow=`xtls-rprx-vision`）与 shadowsocks（`CipherType_AES_256_GCM`）入站后调用 `buildProtocolJSON`，两者输出均为 `{}`；`protocol_json` 中没有 `flow`/`cipher`。
- **根因：** `backend/internal/xray/detect.go:266-280` 先 `mergeProtoMap` 序列化整个 ProxySettings，随后无条件 `delete(out, "users")`、`delete(out, "clients")`；而 vless 的 `flow` 位于 `clients[].account`，shadowsocks 的 cipher 位于 `users[].account`，删除后字段丢失。
- **影响范围：** ① `PushUser` 构造 Xray Account 时 vless vision 节点 Flow 为空、ss 节点 CipherType 为 NONE，与 Xray 入站实际加密/流控不一致；② 动态下载渲染缺少 flow/cipher，产物不完整或不可用。Build6 Step1/Step4 明确要求归一化 `vless flow` 与 `ss cipher`。
- **修复方案（待确认）：** 在删除 `clients`/`users` 之前，按协议从首个 client/user 的 Account TypedMessage 中解析 `vless.Account.Flow`、`shadowsocks.Account.CipherType` 并写入 `out["flow"]` / `out["cipher"]`；无账号时可省略但不得虚构默认值；补上述两种协议的单测。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-04 Clash 动态渲染把 shadowsocks 输出为 `type: shadowsocks`

- **现象：** `backend/internal/assembly/render_clash.go:251` 的 `dynamicClashProxy` 直接 `p.Set("type", d.Protocol)`；检测协议名为 `shadowsocks`，生成的 YAML 为 `type: shadowsocks`，Clash/Mihomo 只识别 `ss`。
- **根因：** 动态节点构造缺少 `shadowsocks → ss` 的协议名映射。
- **影响范围：** 含 shadowsocks 入站的用户下载 Clash 配置时出现无效节点；Build6 Step4 明确要求按 `vless/vmess/trojan/ss` 构造 Clash 条目。
- **修复方案（待确认）：** 在 `dynamicClashProxy`（或 `renderUserSubscription` 构造 `DynamicNode` 时）增加协议映射：`shadowsocks→ss`、`vless/vmess/trojan` 原样；补动态 ss 节点 YAML 断言。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-05 v2 配置导入仅完成“重建行”，事务前后处理与双确认词均未落地

- **现象：** `backend/internal/config/export.go:272` 的 `importV2` 只做「删配置→写配置→删旧实例/账号→插 payload 实例/账号→ext 推送 node_id 暂置 NULL」，随后写 ICON 即返回 `{imported:true}`。Build7 Step2 要求的事务前旧实例/旧 user+ext 推送快照与 RemoveUser、提交后自动检测、display_name 回填、xray_ext_users.node_id 重绑、装配快照 xray 引用重绑、best-effort 对账均未实现；无实例/账号且 advanced_mode=false 时的 `IMPORT→DISABLE` 双确认词也未实现（`ImportV2` 只校验 `IMPORT`）。
- **根因：** Build7 Step2 的实现被简化为最小行重建；`ExportService` 没有注入检测/同步/装配重绑所需的回调，前端 `SettingsView.vue:503` 也只发送 `IMPORT`。
- **影响范围：** 导入 v2 后独立账号推送目标 `node_id=NULL`，`pushTargetsFor` 内连接 nodes 后查不到目标，独立账号实际不可推送；旧 Xray 侧账号残留；节点显示名映射未恢复；装配快照悬空引用未重绑；带实例/账号关闭态导入不会自动开高级；v2 往返不闭环。Build7 Step2 验收不通过。
- **修复方案（待确认）：** 按 Build7 Step2 item4 补齐导入任务全流程；为 `ExportService` 注入 ImportPostProcessor 回调（检测/命名映射/重绑/对账）或由 server 装配侧注入函数字段；前端 v2 导入按 `task_id` 轮询并展示完成提示，双确认词分支补 `DISABLE` 步骤。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-06 OFF 清空任务登记未与状态翻转同事务，并发关闭会创建两个任务

- **现象：** `backend/internal/xray/offclear.go:50` 在校验确认词后、开启 DB 事务前就调用 `registry.Register`；`offClear` 事务内才复查 advanced_mode。两次并发 OFF 提交若都读到 `advanced_mode=true`，会注册两个 `off_clear` 任务，其中第二个事务读到 off 后空跑并以 `succeeded` 结束。
- **根因：** Build7 Step2 明确要求“状态翻转判定与任务登记在同一 `BEGIN IMMEDIATE` 事务内完成，并发第二次提交不建第二个任务”；代码未按该口径实现。
- **影响范围：** 任务列表出现重复/虚假成功任务，前端轮询两个任务后状态混乱；并发关闭语义不符合文档。
- **修复方案（待确认）：** 先开启 `TxImmediate`，在事务内先查询并翻转 advanced_mode，再调用可注入的 `Registry.RegisterInTx` 式登记接口（或登记动作与 DB 行同一事务后置；用户已接受幽灵任务边界），提交后启动 goroutine；补并发两次 OFF 仅产生一个任务的单测。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-07 Build7 Step3/Step4 前端页面主体未按 Design2-UI §8/§4 实现

- **现象：** `frontend/src/views/admin/XrayInstancesView.vue` 仅为 314 行的最小表格：无测试连接、无编辑/启停、无采集状态与连续失败告警、无 api_tag 展示、检测无四项全 0 提示、无 added_nodes 命名区、无对账四分区明细/勾选/行内 push-one、无 ext 创建的目标多选/配额/一次性凭据展示/编辑/删除确认/重试 loading；`GroupsView.vue` 仍是 Build4 最小 CRUD（133 行，无节点分配/排序/候选集引导/默认配额编辑/节点数）；`UsersView.vue` 无高级列（用量/同步状态/超限）与配额覆盖/重置操作；`SettingsView.vue` 高级区为简化版，导入导出 v2 分支未适配（无 task_id 轮询、无双 DISABLE 确认、无 v2 文案与完成提示）。
- **根因：** Build7 Step3/Step4 标记为 ✅，但实际仅新增了 API 封装与页面骨架，未完成文档列出的交互。
- **影响范围：** 高级模式管理面在 UI 上不可完整使用；Build7 里程碑“Xray 实例页/独立账号 Tab/用户组高级管理/用户高级列/导入导出 v2”未达成。

> ⚠️ **迁移指引（2026-08-21，用户已确认）：** 本条目未闭环，已整体迁移至 [Issue3.md](Issue3.md)（含条目正文、修复决策与进度）；后续跟踪以 Issue3 为准。
- **状态：** ➡ 已迁移至 Issue3.md（2026-08-21）

### R15-08 Build6/Build7 要求的专项测试与性能基准大面积缺失

- **现象：** 后端仅 `internal/xray/{account,client,errors,instance,sync,fake}_test.go`，没有 `credentials_test.go`、`stats_test.go`、`quota_test.go`、`ext_test.go`、`reconcile_test.go`、`offclear_test.go`、v2 导入异步/保护/往返测试；`TestRenderUserSubscription10kRules` 不存在（执行 `go test ./internal/download -run TestRenderUserSubscription10kRules -v` 输出 `no tests to run`）；前端测试仅 15 个文件 45 例，无 Build7 Step3/4/5 要求的 Xray 实例页/独立账号、GroupsView、UsersView 高级列、SettingsView 导入 v2 等测试。
- **根因：** 构建文档中“单测/验收标准”未被执行；Build6/Build7 各 Step 仍标记 ✅。
- **影响范围：** R15-01~R15-07 均因此未被测试发现；后续改动无回归保护。

> ⚠️ **迁移指引（2026-08-21，用户已确认）：** 本条目未闭环，已整体迁移至 [Issue3.md](Issue3.md)（含条目正文、修复决策与测试清单）；后续跟踪以 Issue3 为准。
- **状态：** ➡ 已迁移至 Issue3.md（2026-08-21）

### R15-09 同步/候选集回调同步执行且普通 gRPC 调用未施加 30s deadline

- **现象：** `server.go` 中 `groupSvc.SetOnNodesChanged`、`xraySvc.SetOnNodeVisibilityChanged`、用户激活/禁用/换组/删除等回调直接在当前 HTTP 请求内执行 `syncSvc.ReconcileUser` / `RemoveUserFromTargets`，未按 Build6 Step3 要求 `go func` 异步；`sync.go` 的 AddUser/RemoveUser 直接使用请求 ctx，没有 `context.WithTimeout(ctx, xray.RPCTimeout)`（`client.go` 仅定义常量，测试只断言常量存在）。
- **根因：** 将“事务提交后回调”实现为同步调用，且未在调用点统一施加 30s 超时。
- **影响范围：** 用户激活、组节点变更、节点检测等管理操作可能被 Xray 网络阻塞到 HTTP 超时/断连；多目标时串行累积耗时；与“不阻塞主流程、普通 RPC 30s 超时”的执行约束不符。
- **修复方案（待确认）：** 所有副作用回调统一改为 `context.WithoutCancel` + goroutine（或显式后台任务），所有业务 gRPC 调用在进入 API 前统一包 30s deadline；补回调异步与超时单测。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-10 对账执行与独立账号失败路径未落实“超限跳过”与状态机回写

- **现象：** `reconcile.go:175` 的 `PushOne` 对面板用户直接 `pushUserTarget`，不查 `status=active`/`quota_exceeded`，也不像 `PushUser` 一样对超限写 last_error；异步 `RepairPushAsync` 只把失败计数加一，不区分“超限跳过”；ext 的 `removeOne` 失败不写 `xray_ext_users.failed`，`UpdateExt` 删除目标失败后记录仍为 pending，`RetryExt` 只重试 failed 而看不到这些失败；`CheckExtQuota` 忽略每个目标的 RemoveUser 错误后仍置 `quota_exceeded=1`。
- **根因：** Build7 Step1 的“补推时超限面板用户与超限独立账号跳过并记入结果”“失败并入状态机可重试”未实现完整。
- **影响范围：** 管理员在对账/重试时会绕过超限保护重新推送超限账号；ext 移除失败不可观测、不可重试；对账计数口径与 UI 文案不符。
- **修复方案（待确认）：** `PushOne`/异步补推统一走带 active/quota_exceeded 检查的推送入口并返回 `skipped` 计数；ext `removeOne`/`CheckExtQuota` 失败时写 `failed+last_error`，`RetryExt` 按期望集区分 Add/Remove。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-11 SR/generic 无凭据注释、OFF 优先级与用户预览渲染不符合 Build6 Step4

- **现象：** `server/render.go` 的 `replacePlaceholder` 对 sr-subs/generic-subs 先 `removePlaceholderLine`，无占位后直接 base64，导致无凭据/advanced off 时注释 `# 节点未开通，请联系管理员` / `# Xray 高级模式未启用` 实际不会出现在产物中；无凭据时 `comment` 会覆盖 off 注释（OFF 优先级要求相反）；`download.PreviewForUser` 对所有用户只返回当前版本原文，普通用户预览未调用 `RenderUserSubscription`。
- **根因：** 占位替换分支混淆了“无凭据→注释替换”与“有凭据但空目标集→整行移除”两种语义；预览路径未接入动态渲染。
- **影响范围：** 用户拿到的 SR/generic 产物在未开通/关闭高级时缺少设计要求的说明注释；普通用户预览看到 `{{xray_nodes}}` 明文模板；Build6 Step4 验收不通过。
- **修复方案（待确认）：** 将 `hasCreds`/`advanced_mode`/`len(targets)` 三态分支拆开：off 优先输出 off 注释，无凭据输出未开通注释，有凭据空目标才整行移除；`PreviewForUser` 增加 admin 原文/普通用户动态渲染分支并补测试。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-12 组详情候选节点契约缺少 `in_partial_blueprint` 标注

- **现象：** `backend/internal/server/group.go:77` 返回 `candidate_nodes` 为 `[]string`；`Design2-UI.md:530` 与 Build6 Step2 要求候选集并集“含 `in_partial_blueprint` 标注供 UI 提示”。前端 `api/group.ts` 的 `GroupDetail.in_partial_blueprint` 字段无法获得数据，GroupsView 也未消费。
- **根因：** 候选集实现只计算并集，未统计每个候选是否仅被部分蓝图包含。
- **影响范围：** 前端无法显示“仅部分模板候选”橙色提示，候选集引导信息缺失；前后端契约不一致。
- **修复方案（已确认 2026-08-21）：** 候选集接口改为对象数组 `[{name, in_partial_blueprint}]`；计算口径见「R15 决策与修复方法」R15-12；同步更新前端类型与 GroupsView 橙色提示。
- **状态：** ✅ 已修复（2026-08-21 复验核实，见「R15 当前修复状态」表）

### R15-13 Build7 Step6 文档归档、覆盖矩阵与 README 高级模式部署说明未完成

- **现象：** Build7 TODOLIST Step6 仍为 ◧；`Build4.md`~`Build7.md` 未移入 `docs/reports/`，`AGENTS.md:183-195` 仍将四份 Build 标为“活跃（未验收）”；README 无 Xray 高级模式前置条件（gRPC API、policy.stats、IP 白名单、建议 1~5 台实例、OFF 后手动清理提示）；Design2 全量覆盖矩阵未核对输出；`frontend/src/views/admin/PlaceholderView.vue` 等 Build 期占位文件仍残留。
- **根因：** Step6 仅执行了部分自动验证，收口与归档动作未执行。
- **影响范围：** 交付状态与文档索引不一致，新会话按 AGENTS 文档清单会误判 Build4~7 未验收；部署者缺少高级模式必要前置信息。

> ⚠️ **迁移指引（2026-08-21，用户已确认）：** 本条目未闭环，已整体迁移至 [Issue3.md](Issue3.md)（含条目正文与归档决策：修复完成后再归档）；后续跟踪以 Issue3 为准。
- **状态：** ➡ 已迁移至 Issue3.md（2026-08-21）

### R15-14 新增代码中的忽略 error、死代码与停用清理遗漏

- **现象：** `sync.go:256-262` DiffPush 忽略 Remove/Push 错误与状态查询错误；`sync.go:596-603`、`stats.go:87-89`、`traffic.go:24-32`、`ext.go:284/287/293/336/368/436/465/590/755`、`config/export.go:146-149` 等多处忽略 DB/网络返回错误；`export.go` 的 `exportInstances`/`exportAccounts` 出错时被 `_` 丢弃仍继续导出；`download.go` 的 `userUsage` 中 `total = int64(...)` 重复一行；`FormatVersionV1`、`InstanceService.StoreDB`、`approval.Server.SetOnRejected` 空实现等为死代码/空壳；`server.New:146` 调用 `cron.StartXrayCollect` 但忽略返回的 stop 函数，优雅退出时 goroutine/ticker 不停止；Build6 Step3 提到的 `AfterAdvancedOff` 补偿辅助在代码库中不存在。
- **根因：** 新模块未严格执行 AGENTS「所有 error 必须处理」与 Build6 执行约束。
- **影响范围：** 关键失败可能被静默吞掉（尤其导出 v2 数据不完整仍生成文件、流量展示读库失败返回 0）；存在资源泄漏与误导性死代码；`AfterAdvancedOff` 缺失使 Build7 Step2 的“复用”说明落空（当前 offclear 自行实现了等价清理）。

> ⚠️ **迁移指引（2026-08-21，用户已确认）：** 本条目未闭环，已整体迁移至 [Issue3.md](Issue3.md)（含条目正文、修复决策与 DiffPush 删除决策）；后续跟踪以 Issue3 为准。
- **状态：** ➡ 已迁移至 Issue3.md（2026-08-21）




### R15 决策与修复方法（用户已确认 2026-08-21；先记录、暂不修复）

> 本节是对 R15-01~R15-14 各条目中「修复方案（待确认）」的最终确认与细化。**2026-08-21：已闭环条目（R15-01~06/09~12）决策保留本文件作历史记录；未闭环条目（R15-07/08/13/14）的决策已随条目迁移至 [Issue3.md](Issue3.md)，后续修复以 Issue3 为准。**

**全局决策：**
1. **R15-13 归档时机（用户已确认 2026-08-21）**：待 R15/R16 系列全部修复并复验通过后，再归档 Build4~7、更新 AGENTS.md 文档清单与 README。
2. **AfterAdvancedOff 归属**：补实现该辅助函数并按 Build6 Step3 / Build7 Step2 原意接线到 OFF 清空提交后清理，不采用“修订文档取消该函数”的方案。
3. **Build 文档勾选状态**：R15 修复开始时，将 Build6/Build7 中受影响的 Step 回退为 ◧；修复完成并执行该 Step 原验收命令通过后，才恢复 ✅。Build7 Step6 保持 ◧ 直至全量复验与用户决定归档。
4. **其余问题按核查推荐执行**：R15-12 采用候选节点对象数组 `[{name, in_partial_blueprint}]`；R15-01/02/03/04/05/06/07/08/09/10/11/14 按下方方法修复。

**分项修复方法研究结论：**

- **R15-01（异步任务 Context）**：在 `InstanceService.DeleteAsync`、`SyncService.StartInit`、`RepairPushAsync`、`CleanOrphansAsync`、`RepairCredentialsAsync`、`OffClearService.SubmitAdvancedMode`、`ExportService.ImportV2` 七处启动 goroutine 前增加 `bg := context.WithoutCancel(ctx)`（Go 1.26 原生支持；保留 ctx 值、解除请求生命周期绑定），goroutine 内全部改用 `bg`。不得把这些任务改回同步执行。测试：新增 `internal/server/xray_async_test.go`，使用完整 `migrations.FS` 构造带认证的测试服务，经 HTTP 提交删除实例/初始化/OFF 清空后轮询 `/api/admin/tasks/:id` 必须达到 `succeeded`（可完成场景），并保留 `context canceled` 反例注释；各业务包单测改为先 cancel ctx 再调用异步方法，断言任务仍成功。
- **R15-02（节点端口）**：`xray.Target` 增加 `Port int`（json:`port`）；`sync.go targetsDB` 的 SELECT 增加 `n.port` 并扫描；`server/render.go` 两处 `portOf(t.APIAddr)` 改为 `t.Port`（`hostOf(t.APIAddr)` 维持 host 口径）。禁止保留“Port==0 时回退 API 端口”的兜底，避免再次静默产出错误端口。测试：新增 `internal/server/render_test.go`，构造 `APIAddr="127.0.0.1:10085"`、`Port=443` 的 Target，断言 Clash/SR 产物端口为 443；更新 `sync_test.go` Target 构造。
- **R15-03（protocol_json 提取）**：在 `detect.go buildProtocolJSON` 中先对 `proxyTM.GetInstance()` 做类型分支：`*vlessinbound.Config` 取首个 client 的 `Account` TypedMessage，解析为 `*vless.Account` 后写 `out["flow"]`；`*shadowsocks.ServerConfig` 取首个 user 的 Account，解析为 `*shadowsocks.Account` 后经新的 `cipherNameOf(CipherType)`（与 `account.go cipherTypeOf` 反查表共用一套常量）写 `out["cipher"]`；然后才执行 `mergeProtoMap` 与 `delete(out, "users"/"clients")`。测试：新增 `internal/xray/detect_test.go`，用 fake inbound 直接断言 flow/cipher 落库、私钥/用户列表不落库。
- **R15-04（Clash ss 类型）**：在 `assembly/render_clash.go` 增加 `clashProtocolName(protocol string) string`（`shadowsocks→ss`，`ss/vless/vmess/trojan` 原样），`dynamicClashProxy` 使用映射后的值写 `type`；`clash_plan_test.go` 增加动态 ss 节点 YAML 解析断言。
- **R15-05（v2 导入全流程）**：`config/export.go` 保持不 import xray/assembly/server，由 `server.New` 注入三组函数字段：`CleanupXrayTargets(ctx, []ImportCleanupTarget)`（封装 `xray.Dial`+`RemoveUser`）、`DetectImportedInstances(ctx, payload)`（封装 `InstanceService.DetectNodes`，enabled=0 跳过并记录提示）、`PostImportRebindAndReconcile(ctx)`（封装装配快照重绑与对账，失败不阻断 succeeded、写入 hints）。`importV2` 事务内在删除旧实例/账号前自行收集旧 `xray_users`/`xray_ext_users` 快照；提交后依次执行清理、检测、display_name/ext node_id SQL 回填、快照重绑、对账，终态 result 返回 `{imported, hints, skipped_instances, reconcile_errors}`。slug 冲突/重复在事务内先校验，失败文案按 400 口径写入 task.error。双确认词：`ImportV2` 增加可选 `disableConfirmWord`；当 payload 无 instances/accounts 且 advanced_mode=false 时，必须 `confirm_word=="IMPORT"` 且 `disable_confirm_word=="DISABLE"`，否则任务 failed；前端 `SettingsView.doImport` 改为两步弹窗（先 IMPORT、再 DISABLE）后一次提交两个字段，后端按 payload 分支决定是否强制第二个词。前端 v2 响应含 `task_id` 时走 `pollTask(getAdminTask)`，按终态展示 hints/错误；v1 无 task_id 保持现有同步完成提示。
- **R15-06（OFF 并发）**：`SubmitAdvancedMode` 改为：先普通读取当前值；on=true 同步置位；on=false 且当前 off 幂等返回；on=false 且当前 on 时，在同一个 `store.TxImmediate` 内用 `cfg.GetTx` 再次读取，仅当仍为 true 时执行 `UPDATE advanced_mode='false'` 并调用 `registry.Register(KindOffClear)` 得到 taskID；提交后启动 goroutine 执行 `runOffClear(bg)`。`runOffClear` 不再自行翻转/复查 advanced_mode（避免空跑假成功），只收集快照、清空数据、提交后 best-effort RemoveUser；同时保留事务回滚残留幽灵任务的已确认边界。并发测试：新 `internal/xray/offclear_test.go` 用 barrier 同时发起多次 OFF，断言仅一个非空 taskID、仅一个注册任务且终态 succeeded、清空只执行一次。
- **R15-07（前端页面）**：➡ 已随条目迁移至 Issue3.md。
- **R15-08（测试补齐清单）**：➡ 已随条目迁移至 Issue3.md。
- **R15-09（异步与超时）**：`xray.Client` 的每个 gRPC 方法统一加“无 deadline 则包 `RPCTimeout`”的保护（探测类仍由调用方传 `DialTimeout`）；`server.New` 中所有 Xray 副作用回调（组节点变更、节点可见性变化、用户激活/禁用/换组/删除、审批通过/拒绝等）统一改为 `context.WithoutCancel` + goroutine，goroutine 内按用户逐个执行并 log warn；测试用 fake API 在 AddUser 内断言 `ctx.Deadline()` 存在，并断言回调不阻塞请求返回。
- **R15-10（对账/独立账号状态机）**：新增迁移 1011 为 `xray_ext_users`（可选同步给 `xray_users`）增加 `action TEXT NOT NULL DEFAULT 'add' CHECK(action IN ('add','remove'))`；`removeOne` 失败且账号仍存在时写 `sync_status='failed'`、`action='remove'`、`last_error`；`RetryExt` 按 `action` 决定 RemoveUser 或 AddUser（无 action 列迁移前按当前目标集兼容判定）。`PushOne`/`RepairPushAsync` 统一调用带 `status=active` 与 `quota_exceeded` 检查的 `pushUserTargetChecked`，超限记 `skipped` 与 `last_error`；ext 同口径。`CheckExtQuota` 逐目标记录移除失败但不阻断置 `quota_exceeded=1`，Reset 后重推。
- **R15-11（SR/generic 注释与预览）**：`server/render.go` 将注释判定改为严格优先级：`!advancedOn → # Xray 高级模式未启用`；`advancedOn && !hasCreds → # 节点未开通，请联系管理员`；`advancedOn && hasCreds && len(targets)==0 → SR/generic 整行移除`。`replacePlaceholder` 拆分 `replacePlaceholderWithComment` 与 `removePlaceholderLine`，禁止注释分支先删行。`download.Service.PreviewForUser` 增加 `isAdmin bool` 入参（handler 从 `auth.CtxUserRole` 取）：管理员返回当前版本原文；普通用户有自定义订阅返回自定义原文，否则对订阅走已注入的 `renderUser`；测试分别断言两分支。
- **R15-12（候选集契约）**：`group.Service` 新增 `CandidateNode{Name string; InPartialBlueprint bool}`，`CandidateSet(ctx)` 返回 `[]CandidateNode`；新增 `CandidateNames(ctx)` 供 SetNodes/RecomputeCandidateSet 内部使用。`in_partial_blueprint` 计算口径：分母=当前激活且 `xray_candidates` 非空的 clash-yaml/sr-subs/generic-subs 蓝图数；分子=包含该稳定名的蓝图数；`0<分子<分母` 时为 true，分母=0 时全部 false。`server/group.go` GET 详情返回对象数组，前端 `api/group.ts` 与 `GroupsView.vue` 同步类型和橙色提示。
- **R15-13（归档与文档状态）**：➡ 已随条目迁移至 Issue3.md（用户已确认：修复完成后再归档）。
- **R15-14（error/死代码/AfterAdvancedOff）**：➡ 已随条目迁移至 Issue3.md（用户已确认：DiffPush 删除并同步修订 Build6-2 表述）。


### R15 当前修复状态（2026-08-21 执行后）

| 编号 | 状态 | 说明 |
|---|---|---|
| R15-01 | ✅ 已修复 | 异步任务改用 `context.WithoutCancel`；后端全量测试通过 |
| R15-02 | ✅ 已修复 | `Target.Port` + 渲染使用节点端口 |
| R15-03 | ✅ 已修复 | `buildProtocolJSON` 提取 vless flow / ss cipher；新增 detect_test |
| R15-04 | ✅ 已修复 | `clashProtocolName` 映射 shadowsocks→ss |
| R15-05 | ✅ 已修复 | v2 导入增加旧清理/检测/display_name/ext 重绑/重推/双确认词；前端两步确认与 task_id 轮询 |
| R15-06 | ✅ 已修复 | OFF 状态翻转与任务登记同事务；新增 offclear_test 并发单任务 |
| R15-07 | ◧ 进行中 | XrayInstancesView 已补编辑/测试连接/API Tag/采集异常；GroupsView 已补候选集/节点分配/默认配额；SettingsView 已补 v2 两步导入；UsersView 高级列与配额操作仍待补 |
| R15-08 | ◧ 进行中 | 已新增 detect_test、offclear_test、xray-instances-view.spec；其余测试仍待补 |
| R15-09 | ✅ 已修复 | Client 无 deadline 自动 30s；副作用回调异步化 |
| R15-10 | ✅ 已修复 | ext action 迁移、PushOne/Retry 超限与 remove 状态机 |
| R15-11 | ✅ 已修复 | SR/generic 注释优先级与普通用户预览渲染 |
| R15-12 | ✅ 已修复 | CandidateNode 含 node_id/in_partial_blueprint，前后端契约同步 |
| R15-13 | ◧ 待用户决定 | 未执行归档/AGENTS/README 更新 |
| R15-14 | ◧ 进行中 | 已补 AfterAdvancedOff、protocolFromType、cron stop、trafficPayload 错误、清理部分死代码；DiffPush/SetOnRejected/Admin List 错误处理等仍待收尾 |

> 注：本表为 2026-08-21 首轮修复后的历史快照。未闭环项 R15-07/08/13/14 已于 2026-08-21 整体迁移至 [Issue3.md](Issue3.md)，后续进度以 Issue3 为准。

## 附、后续修复顺序（交接给下一轮）

> 本附件用于下一个新对话快速接续。**最新优先级（用户决策 2026-08-21）：先执行下方「0-R15. Build6/Build7 修复」；Build4/5 修复收口（旧 0~5 项）已全部完成，仅留档备查。**


### 0-R15. Build6/Build7 修复（➡ 已迁移至 Issue3.md）

- 2026-08-21 用户确认：未闭环项 R15-07/08/13/14 连同修复方案与顺序整体迁移至 [Issue3.md](Issue3.md)（「一-A、自 Issue2 迁移的未闭环条目」与「附、后续修复顺序」）；后续修复与进度跟踪以 Issue3 为准。


### 0. Build4/5 修复收口（用户决策 2026-08-20，Build6 前置条件）

- **目标：** 关闭 Issue2 中仍为 `☐` 的 R14-01~24（R14-06/07 按用户既有决策随 Build6 Step2/Step4 实施时修，不在此轮）与 BuildReport1 新发现 N1~N4；R14-25 按已决策口径实施，R14-26 维持现状。
- **P1（先做，阻塞后续验证）：**
  - R14-01：后端补 `GET /api/admin/subscriptions/:id`；`static.go` NoRoute 对 `/api/` 前缀 GET 返回 404 JSON。
  - R14-03/04/05：`links.go`/`registry.go` 按 Node-Link-Standards §2 补齐参数与形态，并补 registry↔渲染双向一致性单测（Build6 Step4 抽取 links 的前置）。
  - R14-16 + N1 + N2：home/profile/assembly 接入层 SQL 下沉业务层；移除 `ProfileHandler.users` 死字段；`download.go` 两处忽略 error 修正。
- **P2（UI/交互与结构项）：** R14-02、R14-08~15、R14-17~24、N3；R14-25 关键路径显式错误。
- **验收：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm test
  cd backend && go test ./internal/assembly -bench 'BenchmarkRenderClash10kRules|BenchmarkRenderSrConf10kRules' -benchtime=1x
  cd backend && go test ./internal/assembly -run 'TestRenderClash10kRulesThreshold|TestRenderSrConf10kRulesThreshold' -v
  ```
  并复验 Build4/Build5 文档中的 grep 检查；重写 `.smoke-test.sh` 至 Build4/5 新模型（N4）。

### 1. R12-05：补齐前端单测（低风险，先做）【已完成的旧交接记录】

- **内容：** 为 `PoolTab`/`PoolDetail` 补充单测：进行中再次点击同步提示 warning、同步历史分页加载、组件卸载取消轮询。
- **涉及文件：** `frontend/tests/pool-tab.spec.ts`、`frontend/tests/request-poll.spec.ts`，必要时新增 `frontend/tests/pool-detail.spec.ts`。
- **验收：** `cd frontend && npm run build && npm test`。

### 2. R12-20：补前端 DAG 即时检测与悬空引用红标

- **当前进度：** 已补 `is_public` ConfirmModal、节点列表/卡片双态、代理组节点拖拽（HTML5 drag）。
- **完成内容：** `ProxyGroupsView.vue` 已增加保存前 DAG 环检测；编辑代理组时对已失效节点/子组引用显示红标并提供一键剔除。
- **涉及文件：** `frontend/src/views/admin/ProxyGroupsView.vue`、`frontend/src/composables/useSortableList.ts`。
- **验收：** 前端构建/测试通过；手工在代理组弹窗制造环/悬空引用可见提示。

### 3. R12-21：装配页完整重构（最大项）

- **当前进度：** `AssemblyView.vue` 已支持无效 tab 回退、读取 `platform_id/rule_id`、重新编辑回填与失效引用/显示名提示；但仍为简化表单。
- **完成内容：**
  - `a-segmented` 分步/单页双形态与 localStorage 记忆；
  - Clash 六步、SR subs/generic 五步、SR-conf 五步的步骤条与动态跳过；
  - 装配器步骤：目标选择、头部默认值/一键采用默认、节点/代理组选择、规则素材池有序选择、预览与 Diff、生成回执；
  - 预览时拉取当前激活版本原文进行 Diff；
  - 重新编辑失效引用红标/一键剔除完整交互。
- **涉及文件：** `frontend/src/views/admin/AssemblyView.vue` 及 `frontend/src/views/admin/assembly/` 下新增子组件。
- **验收：** 前端构建/测试通过；四种装配器均可按 Build5 Step6 手工走通。

### 4. R12-22：装配入口与重新编辑参数收口

- **当前进度：** `AssemblyView` 已能读取 `platform_id/rule_id`，版本页“重新编辑”已按 blueprint 的真实 `target_syntax` 跳转。
- **完成内容：** `SubscriptionsView`“前往装配”带 `platform_id`；`RulesView`“装配生成”带 `rule_id`；各入口与装配页 query 参数完整闭环。
- **涉及文件：** `frontend/src/views/admin/SubscriptionsView.vue`、`frontend/src/views/admin/RulesView.vue`、`frontend/src/views/admin/AssemblyView.vue`。
- **验收：** 从订阅/规则/版本页进入装配均能预选目标，重新编辑打开正确页签。

### 5. R12-03：素材池剩余 UI 细节

- **当前进度：** 已补 `<768` 卡片、条目 manual/url 分段标题、`a-time-picker`。
- **完成内容：** `PoolDetail` 顶部信息条已补“编辑”入口与面包屑式返回；移动端详情卡片完整可用。
- **涉及文件：** `frontend/src/views/admin/assembly/PoolDetail.vue`、`PoolTab.vue`。
- **验收：** 前端构建/测试通过；移动端素材池详情手工走查通过。

### 下一轮开始前必做

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd frontend && npm run build && npm test
cd backend && go test ./internal/assembly -run 'TestRenderClash10kRulesThreshold|TestRenderSrConf10kRulesThreshold' -v
```

先阅读本文件“一、进行中问题”中仍为 `☐` 的条目，再按上述 1→5 顺序执行。


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
3. 修复方案确定后，由 [BuildN.md](../BuildN.md) 承接为构建 Step；
4. 修复完成并验收通过后，更新状态为 ✅ 并记录验收命令与实际结果；
5. 非问题的优化候选 / 已知遗留事项归 [DesignN.md](../DesignN.md) §三「后续设计候选」记录，不记录在本文件。

---

## 三、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 初始版本：Build4 静态核查发现素材池后端/前端及测试未完整落地，记录 R12-01~R12-05 |
| v1.1 | 2026-08-20 | Build4/Build5 全面核查：确认 R12-01~R12-05 仍开放，并追加 R12-06~R12-24（素材池状态/空 URL/类型规范化/added 口径/列表存在性/启动失败遗留 running、Clash 键序、蓝图目标与引用校验、装配校验、节点敏感字段/REALITY、代理组子组白名单、Step5/6/7 前端缺失、蓝图列映射、链接编码） |
| v1.2 | 2026-08-20 | 用户确认修复决策：R12-19 代理组不允许引用「🛟无法归属的流量」作为子组；R12-02 新增 `pool_entries.source_url` 精确统计 removed；R12-07 空 URL 同步返回 failed“未配置 URL”；R12-09 added 仅统计实际新增行；R12-13 新增 `assembly_blueprints.platform_id/rule_id` 列；R12-17 `reality-opts` 改为结构化对象字段。同步更新对应 Issue 的修复方案与状态。 |
| v1.3 | 2026-08-20 | 执行修复并验收：新增迁移 1010；修复 R12-01/02/04/06~19/23/24 并标记 ✅；R12-03/05/20/21/22 仍为 ☐（R12-20 已补 is_public 确认与拖拽，但前端 DAG/悬空红标未完整；R12-21/22 仅部分落地）。验证：`go build/vet/test ./...`、前端 build/test、渲染阈值测试均通过。 |
| v1.4 | 2026-08-20 | 增加“附、后续修复顺序（交接给下一轮）”：记录 R12-05 → R12-20 → R12-21 → R12-22 → R12-03 的继续修复顺序、涉及文件、验收命令，供下一轮对话直接接续。 |
| v1.5 | 2026-08-20 | 按交接顺序完成 R12-05/20/21/22/03 并标记 ✅：补齐素材池交互单测；代理组 DAG 环检测与悬空红标/一键剔除；装配页分步/单页双形态、步骤条、头部默认、节点/组/规则步骤、预览 Diff、生成回执与失效剔除；订阅/规则/版本页装配入口带参闭环；素材池详情顶部编辑与面包屑返回。验证：`vue-tsc --noEmit`、前端单测（32 passed）、`vite build` 通过。 |
| v1.6 | 2026-08-20 | Build4/Build5 深度核查（全部验收命令实测通过：后端 build/vet/test、前端 build/32 单测、渲染 benchmark ≈ 1ms/op、旧表/1.25/占位 grep 均 0 命中；R12-01~24 修复全部核实落地），新追加 R13-01~09：DiffView 未用 jsdiff（手写 LCS 性能风险）、装配页未拆分子组件、死代码与忽略 error、vmess tls 口径（用户确认修正）、onXrayChanged 钩子缺失、openvpn 敏感字段（用户确认合理例外）、preview name_changed（用户确认合理简化）、Clash 条目键序、cron 无单测。 |
| v1.7 | 2026-08-20 | 修复 R13-01：安装 diff/@types/diff，DiffView 切换 jsdiff diffLines，删除手写 src/lib/diff.ts；新增手动启动开关（总行数 >5000 时不自动计算，需点击「仍要启动行级对比」）与 timeout:2000 超时保护；新增 tests/diff-view.spec.ts。验证：`npm run build`、`npm test`（35 passed）。 |
| v1.8 | 2026-08-20 | 完成 R13-02~09 核查与修复：拆分 AssemblerShell/TypeTargetStep/HeaderStep/NodesGroupsStep/RulesStep/PreviewStep/GenerateStep 七个子组件并接入装配页，回执按钮直达版本管理页；清理死代码/`var _=`/未用参数并处理 json.Marshal error；修正 vmess tls/sni 并补单测；预留 onXrayChanged 钩子；Clash proxies 键序稳定；cron 抽取可测 run 并补单测。验证：后端 build/vet/test（assembly/cron/node/server 通过）、前端 vue-tsc、vite build、35 单测通过。 |
| v1.9 | 2026-08-20 | 逐项复核 R12/R13 全部条目与代码事实一致性：R13-01~09 修复全部核实落地（七子组件接入且 AssemblyView 双份模板已消除、vmess tls/sni 含双单测、XrayChangedFunc 钩子定义与两处调用、TestClashProxyKeyOrder、cron 三用例），R12 抽样（URL 快照/source_url/空 URL/蓝图目标列）与事实一致；无 ☐/◧ 残留、无新增问题、无需修复。验证：后端 `go build/vet/test ./...` 全绿、前端 `npm run build` + `npm test`（35 passed）。 |
| v2.0 | 2026-08-20 | Build4/Build5 双重核验（事后验收）追加 R14-01~R14-26：验收命令全绿实测（后端 build/vet/test、前端 build/35 单测、benchmark Clash≈15ms/SR≈1.1ms、grep 四项 0 命中、全新库迁移实测）基础上的深度核查新发现——P1：后端缺 GET /api/admin/subscriptions/:id（R14-01）、render_plan 不自包含（R14-06）、xray_candidates 恒空（R14-07，后两项为 Build6 前置依赖）；P2：proxy-groups 键序（R14-02）、链接参数缺失与形态（R14-03/04/05）、取消默认无确认（R14-08）、装配流 pooled 标记（R14-09）、PoolTab 双态（R14-10）、分层红线（R14-16）；其余为 UI 规格簇与整洁项。用户决策：R14-01 后端补路由 + NoRoute 对 /api/ 前缀 GET 改 404 JSON；R14-03/04 全部按 Node-Link-Standards 标准表补齐；R14-06/07 随 Build6 Step4/Step2 实施时修；R14-16 本轮一并整改；R14-25/26 维持待确认。同步修订 Build6.md v2.0 / Build7.md v1.10 / Build5.md v1.9（事前预检确定性问题闭合）。 |
| v2.1 | 2026-08-20 | 第三轮 Build6/7 事前预检用户决策落盘：Q1 先执行 Build4/5 修复收口，完成后方可启动 Build6（附、后续修复顺序已新增优先级 0）；Q2 全局任务 registry 下沉为中性包 `internal/tasks`（Build6 v2.2 / Build7 v1.12）；Q3 OFF 清空维持“状态翻转+任务登记同事务”字面口径并接受回滚残留幽灵任务边界（Build6 v2.2 / Build7 v1.12）；Q4a R14-25 改为关键路径显式错误（其余 fail-safe 补口径注释），状态更新为 ☐ 待修复（已决策）；Q4b R14-26 维持现状，状态更新为 ✅ 已确认。 |
| v2.2 | 2026-08-20 | Build4/5 修复收口首轮执行：完成 R14-01/02/03/04/05/08/09/10/11/12/13/14/18/19/20/21/22/23/24 与 N1/N2/N4；R14-15/16/17/25 部分落地（见各状态）；R14-06/07 按既有决策随 Build6 实施；N3 前端单测仍待补。验证：后端 `go build/vet/test ./...` 全绿，前端 `npm run build` + `npm test`（36 passed）。 |
| v2.3 | 2026-08-20 | 收口 R14-15/16/17/25：装配页 RulesStep 桌面拖拽、装配页 spec（页签回退/步骤跳过/表单校验/preview/失效剔除）、重编辑 vN 提示；新增 internal/home 下沉首页全部 SQL；管理页 PageHeader 全量复用；config 关键路径（configured/本地登录/注册/OIDC 审批/advanced_mode）显式报错；并记录 `config/admin.go` 面板配置读取与 `oidc.IsConfigured()` 暂维持 fail-safe/fail-closed。验证：后端 `go build/vet/test ./...` 全绿，前端 `npm run build` + `npm test`（42 passed）。 |
| v2.4 | 2026-08-20 | 补齐 N3 前端单测并同步 Build4/5 文档：新增 NodesView/ProxyGroupsView 前端单测（节点动态表单/敏感字段留空、代理组环检测提示、移动端排序降级渲染）；Build4 Step4~7、Build5 Step2~7 恢复为 ✅ 验收通过。验证：后端 `go build/vet/test ./...`、前端 `npm run build` + `npm test`（45 passed）。 |
| v2.5 | 2026-08-21 | Build6/Build7 交付核查：追加 R15-01~R15-14（异步任务请求 Context 必失败、动态节点端口/protocol_json/Clash ss 映射缺陷、v2 导入不完整、OFF 并发任务、Build7 前端未完整落地、专项测试缺失、同步回调/超时、对账超限口径、SR/generic 注释与预览、候选集契约、Step6 归档、error/死代码）。验证事实：后端 `go build/vet/test ./...`、`go test -race ./internal/xray/... ./internal/download/... ./internal/group/...`、前端 `npm run build`+45 单测、Docker 镜像构建与 `.smoke-test.sh` 均通过；HTTP 实测实例删除任务复现 `context canceled`；`TestRenderUserSubscription10kRules` 为 `no tests to run`；动态渲染/检测 protocol_json 缺陷经临时单测复现。 |
| v2.6 | 2026-08-21 | 用户确认 R15 决策：归档 Build4~7 与 AGENTS/README 状态更新由用户决定时机；`AfterAdvancedOff` 补实现并接线；R15 修复开始时 Build6/Build7 受影响 Step 回退 ◧、复验通过后恢复 ✅；其余按核查推荐方案。已新增「R15 决策与修复方法」与「0-R15 交接优先级」，暂不改代码。 |
| v2.7 | 2026-08-21 | 未闭环条目迁移（用户已确认）：R15-07/08/13/14 连同修复决策整体迁移至 Issue3.md，原处留迁移指引；R15-01~06/09~12 正文状态行对齐为 ✅ 已修复（与「R15 当前修复状态」表一致）；R14-06/07 经代码复核实已随 Build6 修复，状态补记 ✅；「附 0-R15」交接段改为迁移指引。本轮新增用户决策：R15-13 归档时机为 R15/R16 全部修复并复验通过后；R15-14 中 DiffPush 无调用方死代码确认删除（同步修订 Build6-2 表述）。 |
| v2.8 | 2026-08-24 | 复核确认 R12~R15 全部修复或迁移；归档至 docs/reports。 |
