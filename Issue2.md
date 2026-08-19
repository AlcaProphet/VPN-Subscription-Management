# Issue2.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考），承接已存档的 [Issue1.md](./docs/AchievedDocuments/Issue1.md)。
> 设计记录见 [Design2.md](./Design2.md) 与 [Design2-UI.md](./Design2-UI.md)；构建方案见 [Build4.md](./Build4.md)；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 一、进行中问题

### R12-01 素材池同步任务未固化提交时 URL 快照，编辑 URL 可能影响已启动任务

- **现象：** `backend/internal/pool/sync.go` 的 `SubmitSync` 仅在同一事务内检查池存在性与 running 任务并插入 `pool_sync_tasks`，随后 `go s.runSyncTask(poolID, taskID)`；`runSyncTask` 启动后才从 `rule_pools.urls_json` 读取 URL 列表。若管理员在提交同步后、goroutine 读取前编辑池 URL，本次任务会使用新 URL，而不是提交时的 URL。
- **根因：** 任务提交时没有把 URL 列表作为快照传给后台任务；Build4 Step5 明确要求「删除池或编辑池 URL 不取消已启动任务；URL 编辑只影响下一次同步」。
- **影响范围：** 素材池同步结果可能不符合管理员点击「同步」时的意图，存在差量删除/新增与预期不一致的竞态。
- **修复方案（待实施）：** 在 `SubmitSync` 的 `BEGIN IMMEDIATE` 事务内读取 `urls_json`，将 URL 快照作为参数传入 `runSyncTask`（或写入任务行快照字段），后台任务只使用该快照；编辑池仅影响后续提交的任务。
- **状态：** ☐ 待修复

### R12-02 素材池同步回执 removed 恒为 0，且“逗号多于两段”的 skip 原因未记录

- **现象：** `PerURLResult.Removed` 字段仅声明，从未赋值；`runSyncTask` 执行差量删除后没有回写各 URL 的删除数量，`GET /api/admin/pools/:id/sync/status` 与同步历史中的 `removed` 永远为 0。同时 `parser.go` 对 `TYPE,VALUE,extra` 行返回 skip 原因，但 `parseURLBody` 丢弃该原因且不计入 `skipped`，与 Design2 §2.3「逗号多于两段时仅取前两段，多余段忽略并记录」不一致。
- **根因：** 后端同步统计与解析逻辑未完整实现 Build4 Step5 的契约：差量删除未统计 removed；解析跳过原因未贯通到 per_url 回执。
- **影响范围：** 前端同步回执无法展示真实的删除/跳过明细，影响管理员对同步结果的判断；Build4 Step6 要求「逐 URL 结果回执（含 added/removed/skipped）」。
- **修复方案（已确认）：** 新增 `pool_entries.source_url` 列（增量迁移 1010），同步写入每条 URL 条目的来源；差量删除时按 `source_url` 精确统计各 URL 的 `Removed` 并写入 `PerURLResult.Removed`；`parseURLBody` 对多余逗号段行计入 `skipped`，并新增 `PerURLResult.SkipReasons` 记录 skip 原因摘要，前端同步历史展示逐 URL 明细。
- **状态：** ☐ 待修复

### R12-03 素材池前端未按 Design2-UI §5.2 完成双态列表、分段标题与时间选择器

- **现象：** `PoolTab.vue` 池列表只有 `a-table`，没有 `<768` 卡片态；`PoolDetail.vue` 条目列表也只有 `a-table`，且没有「手动条目（前段）/ URL 同步条目（后段）」分隔标题行；池新建/编辑弹窗的定时同步时刻使用 `Input` 文本框，未使用 Build4 Step6/UI 要求的 `a-time-picker`。
- **根因：** 素材池前端实现只覆盖了桌面表格基本功能，未落地 Design2-UI §5.2.1/§5.2.2 的移动端双态与分段展示规格。
- **影响范围：** 移动端素材池列表/详情可用性与信息层级不符合设计；定时时刻输入缺少时间选择器校验体验。
- **修复方案（待实施）：** 池列表与详情条目列表补 `<768` 卡片分支；详情条目列表增加 manual/url 段间分隔标题行；池编辑弹窗改用 `a-time-picker`（默认 04:00，UTC 说明保留）；详情顶部信息条按 UI 补「编辑」入口与面包屑式返回。
- **状态：** ☐ 待修复

### R12-04 素材池同步轮询的卸载取消、409 特判与历史逐 URL 明细不完整

- **现象：** `PoolTab.vue` 声明了 `pollHandles` 但没有 `onUnmounted` 统一 `cancel()`，组件卸载后轮询仍继续；`PoolDetail.vue` 的 `doSync` catch 未对 `ApiError(409)` 做 `message.warning` 特判；同步历史列表只展示状态、时间与错误 Tooltip，没有逐 URL 明细摘要。
- **根因：** Build4 Step6 的轮询卸载取消、进行中再触发 warning、同步历史逐 URL 明细摘要三项交互未完整落地；`pollHandles` 仅为残留声明。
- **影响范围：** 用户切走页面后仍产生轮询请求；详情页重复触发同步时提示形态不符合 UI §5.2.3（应为 warning）；历史任务无法直接查看各 URL 成功/失败明细。
- **修复方案（待实施）：** `PoolTab` 增加 `onUnmounted` 遍历 `pollHandles` 调用 `cancel()`；`PoolDetail` 捕获 `ApiError` 且 `status === 409` 时 `Notify.warning('同步进行中，请等待完成')`；同步历史每行渲染 `per_url` 摘要（成功/失败、added/removed/skipped、失败原因）。
- **状态：** ☐ 待修复

### R12-05 Build4 Step6 前端单测覆盖未达验收要求

- **现象：** 现有 `frontend/tests/pool-tab.spec.ts` 与 `request-poll.spec.ts` 仅覆盖空态/列表展示与 pollTask 基本终态/取消/网络失败；未覆盖 Build4 Step6 要求的「进行中重复触发提示」「同步历史列表分页」「组件卸载取消轮询」等场景。
- **根因：** 测试随实现同步简化，未按 Build4 Step6 验收标准补齐素材池交互用例。
- **影响范围：** 上述前端交互缺陷缺少回归保护，后续 Build5/6/7 改动时容易再次破坏。
- **修复方案（待实施）：** 补充 `PoolTab`/`PoolDetail` 相关单测：进行中再次点击提示 warning、同步历史分页加载、组件卸载调用 poll cancel。
- **状态：** ☐ 待修复

---

### R12-06 素材池无历史任务时同步状态接口返回 404，空状态分支不可达

- **现象：** `GET /api/admin/pools/:id/sync/status` 在池存在但尚无任何同步任务时返回 `404 素材池不存在`；`server/pool.go` 中 `syncStatus` 虽写了空状态 `{task_id:0,status:""}` 分支，但 `pool.GetStatus` 对 `sql.ErrNoRows` 直接返回 `ErrNotFound`，该空分支实际永远不可达。
- **根因：** `GetStatus` 未区分“无任务”与“池不存在”，接入层按同一 404 处理。
- **影响范围：** 前端首次进入详情/轮询前无法取得空状态，可能误报资源不存在；与 Build4 Step5/UI 的状态契约不符。
- **修复方案：** `GetStatus` 对无任务返回 `nil,nil`（或新增 `ErrNoTask`），接入层对无任务返回空状态；对池不存在仍 404。
- **状态：** ☐ 待修复

### R12-07 素材池无 URL 时同步会清空全部 URL 段条目

- **现象：** 对 `urls_json=[]` 的池调用 `POST /sync`，`runSyncTask` 因“全部 URL 成功”的真空条件进入差量删除，临时 keep 表为空，导致所有 `source='url'` 条目被删除，任务却记为 `succeeded`。
- **根因：** `runSyncTask` 未对 `len(urls)==0` 做保护，空 URL 列表被当作全成功处理。
- **影响范围：** 管理员误同步空池时丢失已有 URL 同步条目，与“空响应视为失败/零有效条目保护”的语义不一致。
- **修复方案（已确认）：** 无 URL 时任务直接返回 `failed`，错误信息为“未配置 URL”，不进入差量删除分支，保留旧数据。
- **状态：** ☐ 待修复

### R12-08 素材池规则类型未统一大写规范化，远程小写类型原样入库

- **现象：** `ValidateEntry` 内部用 `strings.ToUpper` 判断白名单，但只返回规范化后的匹配值，不返回规范化类型；`CreateEntry`/`parseURLBody` 仍用原始 `ruleType` 入库。远程文件若写 `domain,example.com`，池条目会存为 `domain`，装配产物输出 `domain,example.com,...` 而非 `DOMAIN,...`。
- **根因：** 类型规范化只在校验函数内部生效，未贯通到入库与渲染。
- **影响范围：** 规则类型大小写不一致，可能影响客户端识别；违反 Build4 Step5/Design2 白名单口径。
- **修复方案：** 让 `ValidateEntry` 返回规范化后的类型（或所有入库点统一 `strings.ToUpper`），并补小写类型入库单测。
- **状态：** ☐ 待修复

### R12-09 素材池回执 added 统计为解析条目数而非实际新增数

- **现象：** `syncURL` 将 `r.Added = len(entries)`，即使条目已存在或与 manual 冲突未真正插入，也计为新增；`removed` 仍恒为 0（后者已在 R12-02 记录）。
- **根因：** 未按实际 INSERT/UPDATE 行数统计 added。
- **影响范围：** 同步回执“新增”数量虚高，管理员无法准确判断差量变化。
- **修复方案（已确认）：** `added` 只统计实际 INSERT 的新增行数；已存在条目、与 manual 冲突未插入的行不计入。可在入库事务中按 URL 统计 `RowsAffected` 或插入前后计数，写回 `PerURLResult.Added`。
- **状态：** ☐ 待修复

### R12-10 素材池列表/历史端点对不存在池返回 200 空列表

- **现象：** `GET /api/admin/pools/999/entries` 与 `GET /api/admin/pools/999/sync/tasks` 对不存在的池返回 `200 {list:[],total:0}`，而非 404。
- **根因：** `ListEntries`/`ListTasks` 未先校验池存在性。
- **影响范围：** 管理端 API 语义不一致，前端可能把不存在池误当空池。
- **修复方案：** 两个列表方法先校验池存在，不存在返回 `ErrNotFound`，接入层映射 404。
- **状态：** ☐ 待修复

### R12-11 素材池同步启动阶段查询失败会遗留 running 任务

- **现象：** `runSyncTask` 启动时 `SELECT urls_json` 若因非“无此池”原因失败（如数据库瞬时错误），函数仅记日志直接 return，任务行保持 `running`，后续同步永久被 `ErrSyncRunning` 阻塞。
- **根因：** 启动阶段缺少失败终态写回。
- **影响范围：** 同步任务卡死，需人工修库；违反“所有 error 必须处理”的执行约束。
- **修复方案：** 启动查询失败时尝试将任务置为 `failed` 并写错误信息（池仍存在时），再返回。
- **状态：** ☐ 待修复

### R12-12 Clash 渲染未保留头部表单键序

- **现象：** `renderClash` 直接遍历 `map[string]any` 输出头部，`toYAMLNode` 对 map 也无序；Build5 Step3 明确要求用 `yaml.MapSlice` 或自定义有序结构“必须保留管理员填写顺序”。
- **根因：** 使用 Go map 导致键序随机。
- **影响范围：** 同一输入多次生成的 Clash YAML 头部字段顺序不稳定，不符合模板/用户填写顺序要求。
- **修复方案：** 改用有序头部结构（`yaml.MapSlice`/自定义 `orderedMap`），并在 golden 测试中断言键序。
- **状态：** ☐ 待修复

### R12-13 装配蓝图未持久化 platform_id/rule_id，重新编辑无法回填目标

- **现象：** `SaveBlueprintTx` 的 `fixed_params_json`、`selection_json`、`render_plan_json` 均未保存 `platform_id`/`rule_id`；`AssemblyView.loadEditIfAny` 因此把 `platform_id`/`rule_id` 置为 undefined，重新编辑无法恢复目标平台/规则。
- **根因：** 蓝图四列映射遗漏装配目标信息。
- **影响范围：** Build5 Step4/Step6 的重新编辑流不可用；从版本页进入装配后必须重新选择目标。
- **修复方案（已确认）：** 新增 `assembly_blueprints.platform_id`、`assembly_blueprints.rule_id` 两列（增量迁移 1010），并用 `versions` 的 owner 信息回填历史蓝图；`SaveBlueprintTx` 写入目标；blueprint 接口返回；前端重新编辑从 blueprint 回填 `platform_id`/`rule_id`。
- **状态：** ☐ 待修复

### R12-14 blueprint 端点未校验悬空引用与 name_changed

- **现象：** `GET /api/admin/versions/:id/blueprint` 始终返回 `invalid_refs: []`、`name_changed: null`，未读取当前节点/代理组/素材池校验快照引用是否失效，也未对比显示名变化。
- **根因：** Build5 Step4 的引用校验与名称对照未实现。
- **影响范围：** 重新编辑时无法提示失效节点/组/显示名变化，前端“失效引用红标/一键剔除”失去数据支撑。
- **修复方案：** 按蓝图中的 `node_names/group_names/pools` 与当前库比对，返回 `invalid_refs`；对比快照名与当前 `render_name` 返回 `name_changed`。
- **状态：** ☐ 待修复

### R12-15 装配校验缺少 overseas_members 子集与素材池存在性校验

- **现象：** `validate` 对 Clash 只检查 `OverseasMembers` 非空，不校验成员是否为已勾选/存在的节点；`loadPoolEntries` 对不存在的池返回空条目且不报错，导致“🌎国外流量”可能渲染为空、不存在的池静默无规则。
- **根因：** 输入存在性与子集校验不完整。
- **影响范围：** 可生成空“🌎国外流量”组或静默丢规则的产物，绕过空产物硬校验。
- **修复方案：** 校验每个 `overseas_members` 必须属于 `node_names` 且节点存在；校验每个 `pool_id` 在 `rule_pools` 中存在。
- **状态：** ☐ 待修复

### R12-16 节点协议变更后旧敏感字段残留

- **现象：** `node.mergeSensitive` 会把旧协议的全部敏感字段复制到新 `protocol_json`，即使新协议已不再包含该字段；例如 vmess 改为 ss 后，旧的 `uuid` 密文仍保留，并可能被 Clash 渲染输出。
- **根因：** 协议变更未“等价整体重新填表、不保留不兼容旧字段”。
- **影响范围：** 节点参数脏数据、产物中可能出现无关/敏感残留字段；违反 Build5 Step1 协议变更口径。
- **修复方案：** 仅保留新旧协议同名字段（且输入为空时沿用旧密文），新协议不包含的旧敏感字段一律删除。
- **状态：** ☐ 待修复

### R12-17 vless REALITY 参数以字符串存储时无法解析，链接缺 pbk/sid

- **现象：** 协议注册表中 `reality-opts` 是 `text` 类型，前端按 JSON 字符串提交；`realityOpts` 只识别 `map[string]any`，字符串不会被解析，导致 SR/通用 vless 链接缺少 `pbk`/`sid`/`peer` 等 REALITY 参数。
- **根因：** 链接编码未兼容注册表实际字段类型。
- **影响范围：** REALITY 节点生成的订阅链接不完整，无法正常连接。
- **修复方案（已确认）：** 将 `reality-opts` 注册表字段改为结构化对象（`object`/JSON 编辑器），前端动态表单发送对象；`realityOpts` 直接处理 `map[string]any`，链接正常生成 `pbk`/`sid`/`peer` 等参数。
- **状态：** ☐ 待修复

### R12-18 节点创建/更新/切换接口返回未脱敏密文

- **现象：** `CreateManual` 返回 `ProtocolJSON` 为加密后的 `enc:v1:...`；`UpdateManual`/`SetDisplayName`/`SetEnabled`/`SetPublic` 返回 `getRaw` 也含密文，服务端均未像 `List`/`Get` 那样 `redactSensitive`。
- **根因：** 服务层对写操作响应未做统一脱敏。
- **影响范围：** 管理 API 响应泄露凭据密文（虽非明文，但破坏“敏感字段返回空串/***”的脱敏契约），前端若直接使用响应可能误存密文。
- **修复方案：** 所有对外返回的节点对象统一走脱敏（敏感字段置空），或在接入层统一 redact。
- **状态：** ☐ 待修复

### R12-19 代理组允许引用「🛟无法归属的流量」作为子组，超出 Build5 Step2 明确范围

- **现象：** `proxygroup.validateDefinitionWithDAG` 与前端子组选择器均允许 `🛟无法归属的流量` 作为子组；Build5 Step2 明确“子组引用允许 🚀直接连接、🌎国外流量（强制组常量）或 proxy_groups 中其他组”，未包含兜底组。
- **根因：** 强制组白名单误包含全部三个强制组。
- **影响范围：** 可构造引用兜底组的代理组，语义上可能不符合设计（兜底组为 MATCH 终点）。
- **修复方案（已确认）：** 代理组不允许引用「🛟无法归属的流量」作为子组；后端子组引用白名单收窄为 `🚀直接连接`/`🌎国外流量`，前端子组选择器移除该选项，并补充“引用兜底组被拒绝”的单测。该强制组仍可作为规则目标组使用。
- **状态：** ☐ 待修复

### R12-20 Build5 Step5 节点/代理组前端交互未完整落地

- **现象：** `NodesView.vue` 的 `is_public` 开关切换前没有 UI 要求的 ConfirmModal；`ProxyGroupsView.vue` 只有上移/下移按钮，没有 ≥768 拖拽（`HolderOutlined`），也没有前端 DAG 即时检测与悬空引用红标剔除。
- **根因：** 前端实现只覆盖了基础列表/表单，未完成 Design2-UI §6/§7 的全部交互。
- **影响范围：** Build5 Step5 验收自检（危险确认、拖拽排序、DAG 提示）无法通过。
- **修复方案：** 补 `is_public` 切换确认；在 `useSortableList` 接入拖拽并在桌面端使用；增加前端环检测与悬空引用红标提示。
- **状态：** ☐ 待修复

### R12-21 Build5 Step6 订阅装配前端严重未达标

- **现象：** `AssemblyView.vue` 仍是单一通用表单：没有 `a-segmented` 分步/单页双形态、没有六步/五步流程、没有 Clash 头部默认值与“一键采用默认值”、没有 manual/xray 分组与 missing/allocatable 置灰、没有 Clash 手动规则排除 USER-AGENT、没有预览时拉取当前激活版本做 Diff、没有生成回执页，也没有重编辑失效引用剔除。无效 `?tab=` 也未回退 `pool`。
- **根因：** Build5 Step6 的装配器前端主体未按设计实现。
- **影响范围：** 四类装配器无法按设计走通，Build5 里程碑前端验收不达标。
- **修复方案：** 按 Build5 Step6 完整实现四类装配器子组件、步骤流、预览 Diff、生成回执与重新编辑流。
- **状态：** ☐ 待修复

### R12-22 Build5 Step7 装配入口与重新编辑参数未接通

- **现象：** `SubscriptionsView`“前往装配”未带 `platform_id`，`RulesView`“装配生成”未带 `rule_id`，`AssemblyView` 也未读取这些 query 参数；`VersionManageView.reEditUrl` 对非 rule 蓝图一律使用 `clash-yaml`，导致 sr-subs/generic-subs 重新编辑打开错误装配器。
- **根因：** 列表页/版本页与装配页之间的 query 契约未实现。
- **影响范围：** 用户从订阅/规则/版本页进入装配无法预选目标，重新编辑可能进入错误页签。
- **修复方案：** 各入口传递 `platform_id`/`rule_id`，`AssemblyView` 读取并预填；`reEditUrl` 通过 blueprint 的 `target_syntax` 决定页签。
- **状态：** ☐ 待修复

### R12-23 装配蓝图列映射不完整（FINAL 方向未入 fixed_params、缺 Xray 候选集）

- **现象：** `SaveBlueprintTx` 仅把 `final_direction` 放入 `selection_json`，未按 Build5 Step4“fixed_params_json（SR conf 含 FINAL 方向）”写入；`selection_json` 也未包含“Xray 候选集”字段。
- **根因：** 蓝图四列映射与 Build5 文档不完全一致。
- **影响范围：** 蓝图持久化不完整，后续重新编辑/Build6 恢复时缺少必要上下文。
- **修复方案：** SR conf 生成时把 `FINAL` 方向并入 `fixed_params_json`；`selection_json` 增加 Xray 候选集字段（本 Build 可空数组）。
- **状态：** ☐ 待修复

### R12-24 链接编码未将 url.Values.Encode 的 `+` 替换为 `%20`

- **现象：** `links.go` 多处直接使用 `url.Values.Encode()`，未按 Build5 Step3 要求将 `+` 替换为 `%20`；含空格凭据/参数生成的链接中空格会编码为 `+`。
- **根因：** 缺少统一的后处理。
- **影响范围：** 含空格密码等参数的节点订阅链接可能被客户端错误解析。
- **修复方案：** 对 `Encode()` 结果统一 `strings.ReplaceAll(s, "+", "%20")`，并补含空格参数单测。
- **状态：** ☐ 待修复

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
5. 非问题的优化候选 / 已知遗留事项归 [DesignN.md](./DesignN.md) §三「后续设计候选」记录，不记录在本文件。

---

## 三、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 初始版本：Build4 静态核查发现素材池后端/前端及测试未完整落地，记录 R12-01~R12-05 |
| v1.1 | 2026-08-20 | Build4/Build5 全面核查：确认 R12-01~R12-05 仍开放，并追加 R12-06~R12-24（素材池状态/空 URL/类型规范化/added 口径/列表存在性/启动失败遗留 running、Clash 键序、蓝图目标与引用校验、装配校验、节点敏感字段/REALITY、代理组子组白名单、Step5/6/7 前端缺失、蓝图列映射、链接编码） |
| v1.2 | 2026-08-20 | 用户确认修复决策：R12-19 代理组不允许引用「🛟无法归属的流量」作为子组；R12-02 新增 `pool_entries.source_url` 精确统计 removed；R12-07 空 URL 同步返回 failed“未配置 URL”；R12-09 added 仅统计实际新增行；R12-13 新增 `assembly_blueprints.platform_id/rule_id` 列；R12-17 `reality-opts` 改为结构化对象字段。同步更新对应 Issue 的修复方案与状态。 |
