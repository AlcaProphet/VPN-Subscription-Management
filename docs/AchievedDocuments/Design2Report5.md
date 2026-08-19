# Design2Report5.md — Design2 / Design2-UI 与 Build4–7 完整设计核验报告（第五轮）

> **报告定位：** 本文档是对 [Design2.md](../../Design2.md)、[Design2-UI.md](../../Design2-UI.md) 与 [Build4.md](../../Build4.md)～[Build7.md](../../Build7.md) 的**第五轮完整设计核验报告**。本轮从**用户视角**与**管理员视角**双向走查，重点核验四份 Build 文档是否忠实承接 Design2/Design2-UI，并检查设计冲突、语义不明与遗漏点。
> **审核对象：** 当前工作区版本的 Design2.md、Design2-UI.md、Build4.md、Build5.md、Build6.md、Build7.md、AGENTS.md（含上一轮已确认并落盘的 Xray 节点 `display_name` 命名设计）。
> **审核约束：** 本轮只读核验，**未修改任何现有设计/构建文档，未开始构建，未运行构建命令**；仅在本报告文件落盘审核结论。终端操作均为短时只读检索（无网络、无长任务）。
> **审核时间：** 2026-08-19
> **当前执行状态：** Build4–7 均为“活跃（未验收）”规划文档；后端尚无 1009 迁移与 `internal/xray` 实现，设计变更尚处于构建前阶段。

---

## 一、执行摘要

**总体结论：Design2/Design2-UI 主线设计成立，Build4–7 的章节覆盖与步骤拆分基本对齐；但存在 12 项需要用户决策的问题（已全部确认）与 14 项不需要决策、需在后续文档修订中补齐的文档缺口/一致性缺陷。**

最重要的四类问题：

1. **Clash 名称空间校验只做了一半**：节点侧已校验“不得撞代理组名/强制组名/内建保留名”，但 `proxy_groups.name` 创建/编辑没有反向校验，仍可制造 Clash 无法加载的重复名。
2. **预设组种子 JSON 与 Build5 的数据结构互相矛盾**：种子里默认成员「直接连接」写在 `nodes` 数组，而 Build5 明确强制组必须走 `groups` 数组，编辑/校验/重渲染都会踩坑。
3. **Xray 生命周期有两处后端断点**：节点 missing 1→0 的自动补推没有在检测实现中接线；实例/节点删除没有收集独立账号（xray_ext_users）目标，可达实例会残留 ext 账号。
4. **配置导入 v2 可能破坏“OFF 即无 Xray 数据”不变量**：payload 携带实例但 `advanced_mode=false` 时，Build7 未定义处理口径。

本轮 12 项决策均已与用户确认，见第二章决策记录；每项决策对应的落盘动作见第三章。

---

## 二、用户决策记录（2026-08-19）

| # | 问题 | 用户决策 |
|---|------|---------|
| D1 | `proxy_groups.name` 是否增加对称跨命名空间校验 | **采用双向校验**：组名不得与任一节点有效渲染名、三个强制组名、Clash/mihomo 内建保留代理名重复 |
| D2 | 预设组种子中「直接连接」写入 `nodes` 数组的冲突 | **种子改为 `groups:["直接连接"]`**，节点数组置空 |
| D3 | 代理组组类型（select/url-test/fallback）创建后是否可改 | **允许修改**；name/preset_key 仍不可改；旧蓝图快照保持旧类型，新生成版本用新类型 |
| D4 | 节点 missing 1→0 恢复补推的触发点 | **检测时自动补推**：DetectNodes 事务提交后对恢复节点调用可见性回调；对账继续兜底 |
| D5 | 实例/节点删除是否同步清理 ext 账号 Xray 侧推送 | **同步清理**：删除事务前同时收集 xray_users 与 xray_ext_users，提交后两类 RemoveUser |
| D6 | 独立账号 push_targets 是否由后端强制校验可用性 | **后端强制校验**：source=xray、四协议、allocatable=1、missing=0、实例 enabled=1；非法 400 |
| D7 | v2 导入 payload `advanced_mode=false` 但携带实例/账号 | **自动置为开启**：检测到实例或独立账号时，导入将 advanced_mode 置为 true 并在结果中提示 |
| D8 | OFF 清空确认词 | **固定 `DISABLE`**，前后端使用同一常量，与 RESET/IMPORT 区分 |
| D9 | 订阅地址池直接上传版本是否做 product_type 内容格式校验 | **不校验内容**；同步修订 Design2 措辞，product_type 仅用于展示/扩展名/装配目标过滤，不再声称“上传格式校验” |
| D10 | 下载重渲染时代理组内个别悬空/不可用成员如何处理 | **过滤悬空成员**：只保留可达注入节点、DIRECT 与可达子组；完全不可达组删除，空强制组降级 DIRECT |
| D11 | 代理组内容约束口径 | **按 Design2/UI 三选一口径**：每个组至少直接包含节点或「直接连接」「国外流量」之一；Build5 从“非空即可”收紧 |
| D12 | 装配/初始化/对账/检测等长请求 timeout | **统一 120s**；普通请求维持 15s；素材池同步继续用 pollTask（默认 5 分钟轮询窗口） |

---

## 三、A 级发现与落盘动作（对应 12 项决策）

### A1（D1）代理组名称空间校验不对称

- 现状：Design2 §3.2 与 Build5 Step1 已对节点有效渲染名做跨命名空间校验；但 Design2-UI §7.2、Build5 Step2 创建自建组时只校验 `proxy_groups.name` 组内唯一。
- 风险：管理员可创建名为 `US-1`（撞 manual 节点）、`DIRECT`（撞 Clash 内建代理）或 `直接连接`（撞强制组）的代理组，Clash/mihomo 加载时名称冲突。
- 落盘动作：
  1. Design2 §3.3/§5.9：补“组名同样不得与节点有效渲染名、强制组名、内建保留代理名重复”的对称规则。
  2. Design2-UI §7.2：创建组时前端提示与 409 文案同步；§9.4 冲突映射补充“代理组名冲突”。
  3. Build5 Step2：`CreateCustom` 在事务内执行反向校验（查 `nodes` 的 `COALESCE(NULLIF(display_name,''), name)`、强制组常量、内建保留名），冲突 409；补单测。
  4. Build5 伪代码把字符集校验与命名空间校验拆分为两个共享函数（`ValidateName` 只做字符集，`CheckNameNamespace` 做跨命名空间），避免 node/proxygroup 重复实现。

### A2（D2）预设组种子 JSON 与子组引用 schema 冲突

- 现状：Build4 Step1 种子 9 组全部写 `definition_json = {"type":...,"nodes":["直接连接"],"groups":[]}`；Build5 Step2 规定「直接连接/国外流量」必须作为子组引用（groups 数组），且节点引用必须存在于 `nodes` 表。
- 后果：种子数据按 Build5 校验属于“引用不存在的节点”；代理组编辑页会把「直接连接」误显示为悬空节点；重渲染与 DAG 判定也会误判。
- 落盘动作：
  1. Build4 Step1 参考 SQL 的 9 条 INSERT 全部改为 `{"type":...,"nodes":[],"groups":["直接连接"]}`。
  2. Build4 Step1 文字“节点数组 + 空子组数组”改为“空节点数组 + 子组数组「直接连接」”。
  3. Build5 Step2/Step3 单测增加“种子定义可加载、可编辑、可渲染”的回归用例。

### A3（D3）组类型是否可编辑前后不一致

- 现状：Design2-UI §7.2 编辑弹窗保留组类型 `a-radio-group`（暗示可改）；Build5 Step2 `Update` 明确 `name/preset_key/type 不可改`。
- 落盘动作：
  1. 按用户决策：**组类型允许修改**。Build5 Step2 `Update(id, groupType, def)` 接收新 groupType 并校验三枚举。
  2. Design2-UI §7.2 明确“名称只读、组类型可编辑”；预设组编辑同一规则（preset_key 仍不可改）。
  3. Build5 单测增加“组类型修改成功”“非法类型 400”。
  4. 语义注明：历史装配快照保存生成时类型，不改写；新生成版本使用新类型。

### A4（D4）missing 1→0 恢复补推缺少实现接线

- 现状：Design2 §5.5 触发器表要求“节点 missing 1→0（检测恢复）→ AddUser diff”；Build6 Step3 只写了语义“同 enabled 恢复口径”，但 Build6 Step1 `DetectNodes` 没有收集恢复节点、也没有调用回调的出口。
- 落盘动作：
  1. Build6 Step1 `DetectNodes` 返回并内部收集 `recovered: [{node_id, tag}]`（missing 1→0 清单）。
  2. Build6 Step3 明确：DetectNodes 在事务提交后对 recovered 节点逐节点调用 `OnNodeVisibilityChanged`（与 enabled 0→1 同口径，幂等，超限前置拦截）。
  3. 单测：fake 检测触发 missing 复位后断言回调目标集合；advanced_mode off 时回调入口跳过。

### A5（D5）实例/节点删除未覆盖独立账号的 Xray 侧清理

- 现状：Build6 Step1/Step3 的实例删除与节点删除只收集 `xray_users`（面板用户）目标；`xray_ext_users` 只靠 FK 级联删除面板记录。Design2 §5.11 明确“节点/实例删除时推送记录级联清理（RemoveUser 口径同 xray_users）”。
- 风险：实例可达时，独立账号仍留在 Xray 侧；对账的 ext 残留分区虽能发现，但不应成为删除路径的默认兜底。
- 落盘动作：
  1. Build6 Step1 文字注明：删除收集清单先按既有 `xray_users` + `xray_ext_users` 收集（Build7 ext 表接入后天然覆盖），Step3 升级为“受影响 active 用户 × 节点 + 独立账号 × 该实例/节点的推送目标”。
  2. Build7 Step1 增补“实例/节点删除钩子接入 ext 目标”的明确步骤与单测（fake API 断言 user/ext 两类 RemoveUser 调用序）。

### A6（D6）独立账号 push_targets 缺少后端可用性校验

- 现状：Design2 §5.11 给出可用性口径（四协议、allocatable=1、missing=0、实例 enabled=1）；Design2-UI §8.5 前端过滤，但 Build7 Step1 后端只写“对 targets 逐个 AddUser”，未写校验。
- 落盘动作：
  1. Build7 Step1 `CreateExt/UpdateExt` 在写 `xray_ext_users` 前事务内逐目标校验：节点存在、source=xray、protocol ∈ {vless,vmess,trojan,shadowsocks}、allocatable=1、missing=0、`xray_instances.enabled=1`；非法目标 400 并指明具体节点。
  2. Design2-UI §8.5 的过滤文案补全 missing=0 与所属实例 enabled=1（与 §5.11 完全一致）。
  3. 单测覆盖全部非法目标分支。

### A7（D7）v2 导入与 advanced_mode OFF 不变量

- 现状：Build7 Step2 v2 导入直接按 payload 重建 instances/accounts，同时配置键整体覆盖（可能把 advanced_mode 写成 false）；若 payload 含实例/账号且 advanced_mode=false，会形成“开关关闭但 Xray 数据存在”状态，违背 Design2 第一章 OFF 清空不变量。
- 落盘动作：
  1. 按用户决策：**v2 导入时，若 payload 携带非空 instances 或 accounts 且 payload 的 advanced_mode=false，导入事务内将 advanced_mode 置为 true**，并在导入结果/完成提示中显著提示“检测到 Xray 实例/独立账号，已自动开启高级模式”。
  2. 若 payload 无 instances/accounts 且 advanced_mode=false：按 OFF 清空口径处理旧高级数据（收集旧 user/ext 推送目标、删除实例与 ext 账号、清空用户 UUID/代理密码/配额字段、清空 traffic_records 与 xray_ext_traffic），保证最终状态无高级运行时残留。
  3. Build7 Step2 单测补两条：off 导入带实例→自动开；off 导入无实例→旧高级数据完整清理。

### A8（D8）OFF 清空确认词未定案

- 现状：Build7 Step2 写“确认词固定 `OFF`（或与前端一致，建议 `DISABLE` 单独常量）”；Design2-UI §4.7.1 只有确认词输入框，没有词面。
- 落盘动作：
  1. 统一固定常量 `DISABLE`；Design2-UI §4.7.1 关闭弹窗文案明确“请输入 DISABLE”。
  2. Build7 Step2 删除“OFF 或 DISABLE”二选一表述；§4 设置前端与 Step6 手工剧本同步使用 DISABLE。
  3. 单测断言错误确认词 400。

### A9（D9）直接上传内容格式校验声明与 Build 不符

- 现状：Design2 §4.4/§5.9 说 product_type 用于“创建/上传时的格式校验”；Build4/5 对直接上传版本只做归属与命名，未定义内容校验。
- 落盘动作（按用户决策“不校验内容”）：
  1. 修订 Design2 §4.4 与 §5.9 `subscriptions` 行措辞：product_type 用于 UI 展示、默认下载扩展名、装配目标过滤与平台/订阅一致性校验，**不对直接上传内容做格式解析**。
  2. Build4/5 不增加内容校验实现；版本上传仍按原始字节存储与返回。

### A10（D10）下载重渲染对“部分悬空成员”语义不明

- 现状：Design2 §5.7 与 Build6 Step4 都只说“剔除完全不可达组”，未明确组内个别悬空/不可用节点成员（例如 manual 节点已删、xray 节点 missing=1）是否从成员数组移除。
- 风险：如果只删整个组、不删组内失效成员，输出 YAML 会引用未定义代理名，客户端加载失败。
- 落盘动作：
  1. Design2 §5.7 ②与 Build6 Step4 明确：重建每个 group 时，成员节点必须 ∈（manual 已注入 ∪ 动态 xray 已注入 ∪ DIRECT）；成员子组必须属于仍保留的组；不满足者逐项移除。
  2. 完全移除后为空的组按既有“完全不可达组删除；强制组空时降级 [DIRECT]”处理。
  3. Build6 Step4 单测增加“组内混合有效/失效成员”与“manual 节点删除后下载仍可解析”两例。

### A11（D11）代理组内容约束 Build5 与 Design2/UI 不一致

- 现状：Design2 §3.3 与 Design2-UI §7.2 要求“至少含节点 / 直接连接 / 国外流量三者之一”；Build5 Step2 只校验 `len(nodes)+len(groups) > 0`，允许“只含一个任意自定义子组”的组。
- 落盘动作（按用户决策收紧到三选一口径）：
  1. Build5 Step2 校验改为：节点数组非空，或子组数组包含「直接连接」/「国外流量」；否则 400。
  2. 单测覆盖“只有自定义子组→拒绝”“只有节点→通过”“只有直接连接→通过”。
  3. Design2/UI 文案保持现状（已一致）。

### A12（D12）长请求 timeout 未在 Build 落地

- 现状：Design2-UI §9.2 写“建议 60s 档位；具体值 Build 期定”；Build5/6/7 均未再指定装配预览/生成、初始化、对账、检测的具体 timeout。
- 落盘动作（按用户决策统一 120s）：
  1. Design2-UI §9.2 删除“具体值 Build 期定”，改为固定 120s。
  2. Build5 Step4/6 的 preview/generate 请求、Build7 Step3 的 runInit/reconcile/detect 请求显式 `timeout: 120_000`；普通请求仍 15s。
  3. Build4 已实现的 `pollTask`（默认 5 分钟）保持不变，素材池同步仍走轮询。

---

## 四、B 级发现（文档缺口，不需再次决策，后续修订时统一补齐）

| # | 发现 | 证据位置 | 处理建议 |
|---|------|---------|---------|
| B1 | Design2-UI §9.1/§9.3 缺少用户配额覆盖端点契约（`PUT /api/admin/users/:id/quota`），Build6 Step5 已定义后端路由，前端 API 表无对应函数 | UI §9.3 `api/user.ts`；Build6 Step5 | 在 `api/user.ts`（或 `api/xray.ts`）补 `setUserQuota(userId, {quota_override})` 契约，与 UI §4.5 配额覆盖弹窗对应 |
| B2 | Design2-UI §9.1 缺少组节点分配与默认配额端点路径（`PUT /api/admin/groups/:id/nodes`、`PUT /api/admin/groups/:id/quota`），Build6 Step2 已定义 | UI §9.3 `api/group.ts`；Build6 Step2 | 在 `api/group.ts` 契约中补两条端点形状与 400/403 口径 |
| B3 | Design2-UI §9.1/§9.3 缺少高级设置端点契约（`GET/PUT /api/admin/settings/advanced`），Build7 Step2 已定义 | UI §9.3 `api/settings.ts`；Build7 Step2 | 补 `AdvancedSettings` 类型与端点路径/请求响应字段 |
| B4 | Design2-UI §8.5 独立账号推送目标过滤文案漏了 `missing=0` 与所属实例 enabled=1，与 Design2 §5.11 口径不一致 | UI §8.5 | 文案改为“仅列四协议、allocatable=1、missing=0、所属实例 enabled=1 且节点 enabled=1 的 inbound” |
| B5 | 首页默认规则字段名 Build4 Step3（`rule_name`）与 Build7 Step5/UI（`rule_id + name`）不一致 | Build4 Step3；Build7 Step5 | 统一为 `{rule_id, name, current_version, token, download_url}`；同步 Build4 文字与单测 |
| B6 | Build4–7 变更记录未反映上一轮已落盘的 display_name 设计与本轮待修项 | 各 Build 第六章变更记录仍为 v1.0 | 修订完成后为 Build4–7 各补 v1.1 变更记录 |
| B7 | 管理员预览版本原文在 display_name 改名后可能显示旧名（用户下载为新名），UI 未提示 | Design2 §5.7 已注明语义；UI §5.3.5 未提示 | 在预览步与版本管理页增加 Tooltip：“显示名已变更时，预览原文可能为旧名，用户下载按当前显示名渲染” |
| B8 | Build7 Step6 端到端手工剧本未覆盖 xray 节点命名链路 | Build7 Step6 | 增加：检测回执命名 → 节点页改名 → 用户下载 YAML/SR 名称为自定义名 → 导出导入后名称恢复 |
| B9 | Build6 Step1 检测撞名校验需复用节点包跨命名空间校验，但 Build 未指定导出哪个函数 | Build6 Step1 | 明确由 `internal/node` 导出 `ValidateNodeName` 与 `CheckRenderNameNamespaceTx`，Build6 复用，禁止复制实现 |
| B10 | Build5 Step3 后端未校验装配勾选的预设组是否 enabled，仅校验存在性 | Build5 Step3 `validate.go` | 生成/预览时拒绝 `type=preset AND enabled=0` 的组，前端只展示 enabled 组作为预过滤 |
| B11 | Build6 提供 `GET /api/admin/xray/users/:id/sync` 与 `GET /api/admin/xray/instances/:id/stats`，Design2-UI §9.1 未列契约 | Build6 Step3/Step5；UI §9.1 | 二选一：若 UI 不需要，标注为内部/诊断端点；若需要，补契约与调用点 |
| B12 | Build7 Step1 ext 账号名称唯一与 quota 校验未显式写出 | Build7 Step1 | 补“name 唯一 409；quota 为 NULL/0/正数，负数 400”，与表结构/配额语义一致 |
| B13 | `xray_instances.api_tag` 在 Design2 §5.9 无用途说明 | Design2 §5.9 | 补一句“api_tag 为展示/日志/导出标签；gRPC 定位用 api_addr，入站定位用 nodes.tag”（与 Build6 Step1 一致） |
| B14 | Build5 Step6 `NodesGroupsStep` 未写 xray 节点“显示名 + 系统名双行”，UI §5.3.1 已写 | Build5 Step6 | 组件说明补 render_name 展示，避免实现走样 |

---

## 五、用户视角走查结论

### 5.1 已闭环且设计正确的链路

- 普通用户下载：无标识 Token → 平台唯一订阅 → 当前激活版本；无版本返回 200 注释块；直接上传原样返回；装配模板动态注入（Build4 Step2 + Build6 Step4）。
- 自定义订阅优先级最高且原样返回，不注入节点，与 Design2 第一章一致。
- 基础模式首页流量卡“不限流量”；高级模式已用/配额/超限三态；超限不阻断下载（UI §3.1，Build7 Step5）。
- 分流规则卡全体用户可见，未设置时保留入口空态；规则 Token 复制与 `/rules` 跳转闭环（Build4 Step3/4、Build7 Step5）。
- 个人中心基础模式隐藏所属组；高级模式显示本月流量；流量卡开关关闭时同步隐藏（UI §3.2，Build7 Step5）。
- `subscription-userinfo` 仅用户订阅类且高级模式携带；分享/规则下载不加 usage 头（Build6 Step4）。

### 5.2 用户侧残余风险（本次审阅识别）

- 管理员在用户已导入订阅后修改节点显示名，客户端会把改名视为节点变化；旧选择可能失效。建议在命名弹窗文案中增加一句“改名后用户端刷新订阅可能需重新选择节点”（随 B7 一并落盘）。
- 若管理员长期不激活订阅版本，普通用户平台卡隐藏三按钮、仅显示占位；下载 URL 直接访问返回 200 注释块。行为一致，但需部署文档向用户解释（不属设计缺陷，列为运维提示）。

---

## 六、管理员视角走查结论

### 6.1 已闭环且设计正确的链路

- 基础模式零配置：订阅/平台/规则/素材池/节点/代理组/装配全部可用，无 Xray 依赖。
- 高级模式开启仅置位不推送；关闭执行二次确认 + DISABLE 确认词 + 清单式清空 + best-effort Xray 清理（待 D8 落盘）。
- 实例 CRUD → 连通性测试 → 节点检测（含 display_name 保留与撞名跳过）→ 组分配（候选集约束）→ 开始初始化 → 对账兜底，主线完整。
- 独立账号双轨凭据、推送目标、配额、超限与重置在 Design2 §5.11 / Build7 Step1 均有落点（待 A5/A6 补强）。
- 配置导出 v2 含实例、节点命名映射与独立账号；导入保护、检测后重绑、对账回调主链完整（待 A7 补 off 不变量）。
- 装配生成 → 入池不自动生效 → 显式激活 → 重新编辑悬空容错 → 版本上限/首版自动激活，Build4/5 已闭环。

### 6.2 管理员侧残余风险

- 组名与节点名的双向命名空间校验落地前，管理员可能无意创建 Clash 无法加载的配置（A1，已决策修复）。
- 预设组种子修复前，代理组编辑页会显示“直接连接”为悬空节点引用（A2，已决策修复）。
- 实例/节点删除 ext 清理与 missing 恢复补推未落地前，管理员必须依赖对账手工兜底（A4/A5，已决策修复）。
- 导入 v2 未定义 off 不变量前，恢复备份可能造成“开关关着但实例数据存在”的隐藏状态（A7，已决策修复）。

---

## 七、Design2 章节 → Build 覆盖矩阵（核验结论）

| Design2 章节 | 主要承接 Build | 结论 |
|---|---|---|
| 一、模式分层与开关语义 | Build4 Step3/4；Build7 Step2/4/5 | 基本对齐；缺口 A7（导入 off）、A8（确认词）、D12（超时） |
| 二、规则素材池 | Build4 Step5/6 | 对齐，未发现 A 级问题 |
| 三、装配拼接 | Build5 Step1/2/3/6 | 对齐主线；缺口 A1/A2/A3/A11、B10/B14 |
| 四、配置生成与分发 | Build4 Step2/3；Build5 Step3/4/6/7 | 对齐主线；缺口 A9（上传格式措辞）、B5/B7 |
| 5.1–5.4 Xray 背景与约束 | Build6 Step0；Build7 Step2 | 对齐；A7 影响 §5.4 |
| 5.5 用户生命周期同步 | Build6 Step3 | 对齐主体；缺口 A4（missing 恢复接线）、B11 |
| 5.6 用户组模型 | Build6 Step2 | 对齐 |
| 5.7 下载渲染机制 | Build6 Step4 | 对齐主体；缺口 A10（悬空成员）、B7 |
| 5.8 流量配额机制 | Build6 Step5 | 对齐 |
| 5.9 数据模型 | Build4 Step1 | 对齐主体；缺口 A2（种子）、B13（api_tag 说明） |
| 5.10 管理端点与 UI 影响 | Build4/5/6/7 分散承接 | 功能覆盖齐全；契约缺口 B1/B2/B3/B11 |
| 5.11 独立 Xray 账号 | Build7 Step1/3 | 对齐主体；缺口 A5/A6/B4/B12 |

Design2-UI 十章的 GUI 覆盖：第三章（用户端）由 Build4 Step4 + Build7 Step5 承接；第四章由 Build4 Step4 + Build7 Step4 承接；第五~七章由 Build5 Step5/6 承接；第八章由 Build7 Step3 承接；第九章契约分散在 Build4/5/6/7；第十章由 Build7 Step4/5 收口。未发现整章漏项。

---

## 八、本轮已执行的核验手段

1. **文档一致性检索**：对 7 份文档做了 `display_name / render_name / added_nodes / 有效渲染名` 与旧表述矛盾词扫描，确认上一轮命名设计已贯通。
2. **表结构核验**：提取 Build4 1009 SQL 中全部 13 张新表，与 Design2 §5.9 逐一比对，表名、关键列、外键方向一致；SQLite 内存库实验验证 `idx_nodes_render_name` 表达式唯一索引可拦截 `name` 与 `display_name` 交叉冲突。
3. **端点覆盖核验**：逐条比对 Design2-UI §9.1 与 Build4–7 路由，发现 B1/B2/B3/B11 契约缺漏。
4. **Markdown 结构核验**：全部文档表格列数与标题层级检查通过；无 `git diff --check` 类空白错误。
5. **用户/管理员双视角场景走查**：按下载、首页、个人中心、配额、装配、激活、初始化、对账、导入导出、OFF 清空、独立账号等 20 余条路径逐条追踪设计→Build 落点。

---

## 九、后续行动建议

1. 按第三章 A1–A12 与第四章 B1–B14，修订 Design2.md、Design2-UI.md、Build4–7.md；修订后同步各文档变更记录（Build4–7 补 v1.1）。
2. 修订完成后，Build7 Step6“设计覆盖矩阵核对”应按本报告更新核对清单，确保 A/B 项全部关闭。
3. 修订期间不得开始构建；全部文档修订经用户确认后，再从 Build4 Step 0 按序执行。
4. 本报告归档于 `docs/AchievedDocuments/Design2Report5.md`，与 Report1–4 同一用途：历史核查，当前状态以 Design2/Design2-UI 与 Build4–7 最终修订版为准。

---

## 十、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 第五轮完整设计核验：12 项用户决策、14 项 B 级文档缺口、用户/管理员双视角走查、Design2→Build4–7 覆盖矩阵；未修改现有文档，仅落盘本报告 |
| v1.1 | 2026-08-19 | 按本报告 A1–A12 / B1–B14 完成 Design2、Design2-UI、Build4–7 修订；各文档变更记录同步 |
