# DesignReport7.md — Design2 / Design2-UI / Build4~7 第八轮只读设计核验报告

> **报告定位：** 本文档是对 [Design2.md](../../../Design2.md)、[Design2-UI.md](../../../Design2-UI.md) 与 [Build4.md](../Build/Build4.md)～[Build7.md](../Build/Build7.md) 的**第八轮完整只读设计核验报告**。承接 [DesignReport6.md](DesignReport6.md) 已闭环的修订（Q1/Q2/Q3 与 P2-1~P2-6、R1~R5），本轮聚焦修订后六份文档的残余冲突、遗漏交互分支、Build 边界归置与可实施性风险。
> **审核对象：** 当前工作区版本的 Design2.md、Design2-UI.md、Build4.md、Build5.md、Build6.md、Build7.md。
> **审核约束：** 本轮全程只读，未修改任何现有设计/构建文档，未修改代码/配置，未开始构建，未运行会改变仓库状态的命令。终端操作均为短时只读检索。
> **审核时间：** 2026-08-19
> **当前执行状态：** Build4–7 均为「活跃（未验收）」规划文档；后端尚无 1009 迁移与 `internal/xray` 实现；本报告落盘前工作区 `git status` 干净。

---

## 一、核验范围

| 对象 | 行数（约） | 核验内容 |
|------|-----------|---------|
| Design2.md | 430 | 第一~五章全量：模式分层与开关语义、规则素材池、装配拼接、配置生成与分发、Xray 对接（5.1~5.11） |
| Design2-UI.md | 660 | 全量：全局风格、面板布局与路由、用户端、管理面板改造页、订阅装配页、节点/代理组/Xray 实例页、前后端契约、交互约定与空态文案 |
| Build4.md | 893 | Step 0~7：Go 1.26、1009 迁移、旧分发模型拆除、版本语义、前端基线、规则素材池 |
| Build5.md | 520 | Step 1~7：协议注册表与 manual 节点、代理组、装配渲染内核、装配端点、节点/代理组/装配前端、分发收口 |
| Build6.md | 597 | Step 0~6：xray-core 客户端、实例与节点检测、高级中间件与组模型、生命周期同步、下载动态渲染、流量配额、假 Xray 集成测试 |
| Build7.md | 428 | Step 1~6：独立账号与对账、OFF 清空与导入导出 v2、Xray 实例页、组/用户/设置高级前端、用户端收口、全量验收 |

重点闭环核查项（用户指定）：基础/高级模式边界、advanced_mode 开关语义、OFF 清空、ON 初始化、订阅装配、规则素材池、节点/代理组/候选集、下载渲染、流量配额、Xray 对账、独立账号、配置导入导出、首页/个人中心展示。

---

## 二、核验方法

1. **全量通读 + 交叉核对**：六份文档逐节通读；Design2 ↔ Design2-UI 逐条对照承载完整性；Design2 章节 ↔ Build4~7 Step/执行约束表/验收标准逐项对齐；四份 Build 之间范围边界与候选清单互查。
2. **结构化只读检索**：对端点清单（GET/POST/PUT/DELETE `/api/*`）、关键字段（advanced_mode / product_type / target_syntax / display_name / allocatable / quota / traffic / sort_order / pool_sync_tasks 等）、状态机与错误码做跨文档比对；核对引用章节号是否指向正确内容。
3. **代码基线事实抽样（只读）**：抽样核对当前 `backend/internal/server/download.go` 预览端点、`dataclear_test.go` fstest 结构、`custom_subscriptions` 平台级联、`/api/auth/me` 现有字段等 Build 文档依赖的事实假设。
4. **双视角动线走查**：以「普通用户」（注册→首页→平台卡→分流规则卡→流量卡→超限提示→个人中心）与「管理员」（新部署基础模式动线 / 高级模式动线：开关→录实例→检测→组分配→初始化→对账→独立账号→导入导出→OFF 清空→重开）模拟全流程，核对功能语义、界面表现、权限控制与提示文案一致性。
5. **八项隐形漏洞与专项口径核对**：空态、加载态、错误态、权限可见性、移动端适配、暗色模式、防重复提交、危险确认；另核超时/轮询、日志脱敏、下载禁缓存、事务边界、幂等性、级联删除、候选集重算、凭据加密、Xray 串行调用与失败重试。

---

## 三、发现的问题（分级）

> 级别定义沿用前轮：**P0**=阻塞构建的硬冲突；**P1**=需用户决策的缺口/不对齐；**P2**=次要观察项，Build 期澄清或随文档修订顺带处理。

### 3.1 P0

本轮未发现 P0。

### 3.2 P1（12 项，均已与用户确认）

| # | 类型 | 位置 | 问题摘要 |
|---|------|------|---------|
| Q1 | 设计冲突 | Build6 Step2 vs Design2 §5.6/§5.7 | `RecomputeCandidateSet` 删除「不再满足可用性过滤」的 group_nodes，与「节点停用后组分配记录保留、重新启用即恢复注入」冲突。 |
| Q2 | 设计冲突 | Build6 Step1 vs Design2 §3.2 | Design2 规定检测不覆盖已有节点的 `allocatable`；Build6 明确更新 `allocatable`。 |
| Q3 | 遗漏设计点 | UI §3.2 / Build6 Step5 / Build7 Step5 | 个人中心「本月流量」行只有 UI 规格，没有任何 Build Step 定义后端数据源（/api/auth/me、/api/home、独立端点均未落点）。 |
| Q4 | 遗漏设计点 | Design2 §5.11 / UI §8.5 vs UI §9.1 / Build7 Step1 | 独立账号失败推送要求「行内重试」，但 ext 域无 retry 端点与 API 契约。 |
| Q5 | 遗漏设计点/危险确认 | Design2 §5.4 / UI §4.7.2 / Build7 Step2 | v2 导入在「无 instances/accounts 且 advanced_mode=false」时执行 OFF 清空（破坏性），导入确认 UI 未披露该后果，且未要求 DISABLE 确认词。 |
| Q6 | 不可实施风险 | Build4 Step5 | 素材池两段排序算法在 URL 段存在后新增 manual 条目会取全局 MAX+1，破坏「manual 段恒前」。 |
| Q7 | 语义不明 | Design2 §3.2 vs Build6 Step1 / UI §8.1~8.2 | Design2 写「实例保存时 + 手动刷新节点触发检测」，Build/UI 实际只做手动检测。 |
| Q8 | Build 未对齐 | UI §9.1 / Build6 Step5 / Build7 约束表 | 契约字段 `collect_interval_minutes` 与内部配置键 `xray_collect_interval_minutes` 命名不一致，无映射定义。 |
| Q9 | 遗漏设计点/超时 | UI §9.2 / Build7 Step2/Step3 | 导入、OFF 清空、实例删除、独立账号增改会同步串行调用 Xray，但 120s 覆盖清单不含这些请求，前端仍为全局 15s。 |
| Q10 | 遗漏设计点/装配校验 | Design2 §3.5/§4.1 / Build5 Step3 / UI §5.3 | 装配 generate 未定义对悬空代理组引用、未勾选目标组、不可用 xray 节点、平台有而无订阅行的校验/UI 分支；未明确强制组能否作为规则目标。 |
| Q11 | 错误归置 | Build4 Step4 vs Build7 Step4 | 基础模式隐藏用户管理「所属组」列/换组 Select（Design2 §一/UI §4.5）被放到 Build7，Build4 已隐藏菜单/个人中心/顶栏组信息却漏了 UsersView。 |
| Q12 | 遗漏设计点 | Design2 §2.4/§5.9 vs UI §5.2 / Build4 Step5/Step6 | `pool_sync_tasks` 历史任务「保留供 UI 展示」，但无历史任务端点/列表，只有最近一次状态。 |

### 3.3 P2（19 项，均已逐项确认）

| # | 类型 | 位置 | 问题摘要 |
|---|------|------|---------|
| P2-1 | Build 文档自相矛盾 | Build4 Step1 | 正文写「本 Step 之后不要启动旧业务服务」，验收命令却 `go run ./cmd/server`。 |
| P2-2 | Build 文档命令错误 | Build5 Step3/Step7 | 只定义了 `BenchmarkRenderClash10kRules`，验收命令却是 `-run TestRender10kRules`，无法命中 benchmark。 |
| P2-3 | 数据一致性 | Build4 Step5 | 服务重启只把 `pool_sync_tasks.running` 置 failed，`rule_pools.sync_status/sync_error` 快照可能残留 running。 |
| P2-4 | 契约缺字段 | UI §9.1 vs Build4 Step5 | `getSyncStatus.per_url` UI 缺 `skipped` 字段。 |
| P2-5 | 契约缺字段 | UI §9.1 vs Build5 Step4 | assembly `preview` 的 `skipped/warnings`、`generate` 的 `skipped/warnings` 未写入 UI 契约。 |
| P2-6 | 交互数据源未定义 | UI §5.3.5 / Build5 Step6 | 预览「与当前激活版本对比」需要当前版本原文，未定义数据来源。 |
| P2-7 | 交互数据源未定义 | UI §5.3.5 | 「显示名变更提示」无判定字段来源。 |
| P2-8 | UI 内部矛盾 | UI §4.2 vs §5.3.0 / Build4 Step4 | §4.2 写「首次自动激活 UI 无需分支」，§5.3.0/Build4 又要求提示。 |
| P2-9 | 级联/悬空引用文案不全 | Design2 §5.7 / UI §6.1/§7.1 / Build5 | manual 节点/代理组删除后，其他代理组定义与装配快照可能悬空；删除影响清单与生成时校验未闭环（与 Q10 联动）。 |
| P2-10 | 链接编码风险 | Build5 Step3 links.go | `#fragment` 用 `url.QueryEscape`，节点名含空格会被编码为 `+`。 |
| P2-11 | 生命周期边界 | Build6 Step3 | 换组、组删除迁移的 diff 钩子未限定 status=active，可能给禁用用户 AddUser。 |
| P2-12 | 权限口径三文不一致 | Design2 §5.10 / UI §9.1 / Build6 Step2 | `GET /api/admin/groups/:id` 是否受 advancedMode 保护：Design2 说基础 CRUD 不保护，UI 写 off 403，Build6 表述模糊。 |
| P2-13 | 导入边界 | Design2 §5.4 / Build7 Step2 | 导入后处理写「自动节点检测刷新」，但 Build6 检测拒绝 enabled=0 实例，disabled 实例导入后的分支未定义。 |
| P2-14 | 契约缺口 | UI §8.5/§9.1 / Build7 Step1 | ext 创建成功一次性展示 UUID/代理密码，但 createExt 响应形状未定义。 |
| P2-15 | 边界条件 | Design2 §5.2/§5.3 / Build6 Step1 | 文档多处写实例 1~5 台，但无上限校验。 |
| P2-16 | 引用不一致 | Design2 §3.3 / Build4 Step1 vs Clash.yaml.template.md | 预设种子 YouTube=url-test，但被引用的作者模板中 YouTube=select。 |
| P2-17 | 措辞与实现不符 | Build6 Step0 | 写「全实例保守串行化」，实现为每 Client 一把锁（每实例串行）。 |
| P2-18 | 验收口径松动 | Build7 Step6 | `.smoke-test.sh` 后加 `\|\| true`，与「全部命令通过」矛盾。 |
| P2-19 | 导入导出口径 | Design2 §5.4 / Build7 Step2 | `xray_ext_accounts.quota_exceeded` 运行态未纳入 v2 导出，导入后对账可能误推超限账号。 |

---

## 四、用户确认结果（2026-08-19）

### 4.1 P1 决策项

| # | 决策项 | 用户确认结论 |
|---|--------|-------------|
| Q1 | 候选集重算与节点停用冲突 | **采用 Build6 现状语义**：候选集重算删除「不在并集或不再满足可用性过滤」的 group_nodes；同步修订 Design2 §5.6/§5.7（停用/缺失/不可分配节点将自动摘除组分配，重新启用后需重新分配），UI §4.3 的红色警示保留为防御性兜底展示。 |
| Q2 | 检测是否覆盖 allocatable | **检测按协议变化更新 allocatable**，保留 enabled/is_public/display_name/装配勾选状态不覆盖；修订 Design2 §3.2 措辞。 |
| Q3 | 个人中心本月流量数据源 | **新增独立流量端点**（建议 `GET /api/profile/traffic`，返回与首页一致的 `{unlimited, used_bytes, quota_bytes|null, exceeded}`）；后端归 Build6 Step5，前端归 Build7 Step5，UI §9.1/§9.3 补契约。 |
| Q4 | 独立账号行内重试 | **新增 `POST /api/admin/xray/ext/:id/retry`**；UI §9.1 `api/xray.ts` 补 `retryExtSync`，Build7 Step1 补实现与单测，Step3 前端接入行内重试。 |
| Q5 | 导入触发 OFF 清空的危险确认 | **导入确认弹窗增加显著警告，且该分支要求输入确认词 `DISABLE`**（与 OFF 清空同口径）；UI §4.7.2 与 Build7 Step2/Step4 同步。 |
| Q6 | 素材池两段排序算法 | **manual 新增取 manual 段内 `MAX(sort_order)+1`；URL 新增取 URL 段内追加**，两段各自维护；Build4 Step5 文字/伪代码/单测同步。 |
| Q7 | 实例保存是否自动检测 | **保存不自动检测**；修订 Design2 §3.2 为「保存后由管理员手动『刷新节点』触发 ListInbounds 检测」，Build6/UI 维持手动检测。 |
| Q8 | 采集间隔字段命名 | **显式定义 API 字段 `collect_interval_minutes` ↔ 配置键 `xray_collect_interval_minutes` 的映射**，并补读写单测；UI 契约不变。 |
| Q9 | 长请求超时覆盖范围 | **导入与 OFF 清空改为异步任务 + 轮询**（新增任务状态端点，复用或扩展 pollTask 契约）；实例删除、独立账号创建/编辑/删除等同步长操作统一 120s；UI §9.2 与 Build7 Step2/Step3 同步。 |
| Q10 | 装配生成校验缺口 | **generate/preview 严格校验**：悬空代理组引用、未勾选目标组拒绝；不可用 xray 节点拒绝或明确跳过（实现时二选一并在 UI 提示）；**强制组「直接连接/国外流量/无法归属的流量」允许作为规则目标**；平台存在但无订阅行时 UI 前置提示并禁用生成。 |
| Q11 | 基础模式隐藏所属组列的 Build 归置 | **维持 Build7 实现**，接受 Build4~6 期间用户管理页仍显示组列的中间态；最终交付一致。 |
| Q12 | 同步历史任务 UI 落点 | **新增池详情历史任务列表端点与 UI（保留最近 N 条）**；Build4 Step5 补 `GET /api/admin/pools/:id/sync/tasks` 形状，UI §5.2 补历史列表规格。 |

### 4.2 P2 逐项确认

| # | 用户确认结论 |
|---|-------------|
| P2-1 | **改写 Build4 Step1 措辞**：允许本 Step 启动旧业务服务做迁移/健康验证；若旧代码因已 DROP 两表报错，允许清空已有数据重新开始（全新部署口径），不再写「不要启动旧业务服务」。 |
| P2-2 | 验收命令改为 `go test -bench BenchmarkRenderClash10kRules -benchtime=1x`，另补一个阈值断言测试；Build5 Step7 同步。 |
| P2-3 | 启动重置 running 任务时，**同步更新 `rule_pools.sync_status='failed'`、`sync_error='服务重启，任务中断'`**。 |
| P2-4 | UI §9.1 `getSyncStatus.per_url` 补 `skipped` 字段。 |
| P2-5 | UI §9.1 `preview/generate` 响应补 `skipped` / `warnings` 字段与含义。 |
| P2-6 | 明确前端复用既有版本预览端点拉取当前激活版本原文做 jsdiff，写入 UI §5.3.5 与 Build5 Step6。 |
| P2-7 | `getBlueprint` / `preview` 契约补 `name_changed` 对照信息（或等价字段），前端据此显示 Tooltip。 |
| P2-8 | 统一为「版本列表无特判；装配生成回执与订阅版本上传成功提示在 `auto_activated=true` 时显示『首个版本已自动激活』」。 |
| P2-9 | 严格校验按 Q10 落地；另补 manual 节点删除影响清单与代理组删除影响清单，写明可能使其他代理组/快照产生悬空引用，编辑页红标剔除。 |
| P2-10 | **节点名与显示名规则追加「禁止空格」**（URL 编码不再依赖 QueryEscape 空格行为），Build5 名称校验与单测同步。 |
| P2-11 | `PushUser`/`DiffPush` 统一先校验 `status=active`；换组、组删除迁移对非 active 用户只清理旧目标不 AddUser。 |
| P2-12 | `GET /api/admin/groups/:id` **不纳入高级屏蔽**；advanced_mode=off 时仅返回基础组信息，省略 `nodes/candidate_nodes/default_quota` 等高级字段；UI §9.1 删除该端点 off 403 表述，Build6 Step2 中间件口径同步。 |
| P2-13 | 导入后仅对 enabled=1 实例自动检测；enabled=0 实例跳过并在完成提示中列出，待启用后手动刷新。 |
| P2-14 | createExt 响应直接返回 account 与一次性明文凭据（generate 模式），前端展示后即焚；manual 模式不返回明文。 |
| P2-15 | **不做硬上限校验**，将文档中「1~5 台」表述修订为建议规模口径。 |
| P2-16 | 以 Clash.yaml.template.md 为准，**YouTube 预设组统一为 select**；Build4 种子与 Design2 §3.3 示例同步。 |
| P2-17 | 明确「**每实例串行、多实例间可并行**」，修正 Build6 Step0「全实例保守串行化」措辞。 |
| P2-18 | 删除 `\|\| true`；`.smoke-test.sh` 不存在可跳过并注明，脚本存在则失败必须修复。 |
| P2-19 | v2 导出/导入 accounts 增加 `quota_exceeded` 字段，避免导入后对账误推超限账号。 |

---

## 五、最终结论

1. **总体结论**：Design2.md 与 Design2-UI.md 主线设计自洽、可实施；Build4~7 的章节覆盖、Step 划分、端点清单、数据模型与候选清单整体对齐，**未发现 P0 级硬冲突**。
2. **残余缺口处置**：本轮发现 12 项 P1 与 19 项 P2。全部 P1/P2 已经用户确认处理口径（见第四章），并已于 2026-08-19 回写 Design2/Design2-UI/Build4~7（落地明细见第七节）。
3. **就绪度判定**：修订已落盘并同步各文档变更记录；**Build4 具备启动条件**。建议按第七节修订记录做一次快速复核后启动构建。
4. **范围边界确认**：Build4~7 的四轮范围边界总体清晰；Build5/6/7 候选清单互指闭环。用户已确认 Build4~6 期间 UsersView 组列显隐的中间态可接受，不作为缺陷阻塞。

---

## 六、建议后续处理项

按以下清单回写文档（**已按用户指令于 2026-08-19 执行，落地明细见第七节**）：

| # | 动作 | 目标文档 | 内容 |
|---|------|---------|------|
| 1 | 设计修订 | Design2 §5.6/§5.7 | 候选集重算删除不可用分配的新语义；节点停用/缺失/不可分配自动摘除组分配、重新启用后需重新分配（Q1）。 |
| 2 | 设计修订 | Design2 §3.2 | 检测可按协议变化更新 allocatable；删除「allocatable 不被检测覆盖」表述；保存不自动检测，仅手动刷新（Q2/Q7）。 |
| 3 | 设计修订 | Design2 §5.10 / §5.11 / §5.4 | 个人中心独立流量端点；ext 重试端点；导入无实例且 off 分支的 DISABLE 确认词；quota_exceeded 纳入 v2 导出（Q3/Q4/Q5/P2-19）。 |
| 4 | 设计修订 | Design2 §3.2/§3.3 | 节点名/显示名禁止空格；YouTube 预设组 select；实例 1~5 为建议规模（P2-10/P2-16/P2-15）。 |
| 5 | UI 修订 | Design2-UI §4.7.2/§5.2/§5.3.5/§9.1/§9.2/§9.3 | 导入危险确认与完成提示；池历史任务列表；diff 数据来源与 name_changed 字段；preview/generate skipped/warnings；per_url skipped；retryExtSync；collect_interval 映射说明；长请求 120s 范围与导入/OFF 异步轮询契约（Q4/Q5/Q8/Q9/Q12/P2-4~P2-7/P2-14）。 |
| 6 | UI 修订 | Design2-UI §4.2/§4.3/§8.5 | 首次自动激活文案统一；组编辑不可用分配防御性警示口径随 Q1 修订；ext 行内重试接入（P2-8/Q1/Q4）。 |
| 7 | Build 修订 | Build4 Step1/Step5/Step6 | Step1 启动旧服务措辞与全新部署口径；两段排序算法 manual 段内追加；重启同步 rule_pools 快照；池历史任务端点（P2-1/Q6/P2-3/Q12）。 |
| 8 | Build 修订 | Build5 Step1/Step3/Step6/Step7 | 名称禁空格校验；benchmark 验收命令与阈值断言；diff 数据来源；删除影响清单；严格校验含强制组目标（P2-2/P2-6/P2-9/P2-10/Q10）。 |
| 9 | Build 修订 | Build6 Step0/Step2/Step3/Step5 | 每实例串行措辞；group_nodes 不可用分配删除语义与中间件口径；active 用户钩子过滤；collect_interval 映射与测试；`GET /api/profile/traffic`（P2-17/Q1/P2-11/P2-12/Q8/Q3）。 |
| 10 | Build 修订 | Build7 Step1/Step2/Step3/Step4/Step6 | ext retry 端点与单测；导入/OFF 异步任务+轮询；导入 DISABLE 确认词、disabled 实例跳过、quota_exceeded 导出；ext 创建一次性凭据响应；120s 同步长操作；smoke 失败即验收失败（Q4/Q5/Q9/P2-13/P2-14/P2-18/P2-19）。 |
| 11 | 复核 | 全部六份文档 | 修订后二次只读复核，确保无回归；同步各文档变更记录版本号；确认 `git status` 仅含预期报告与修订。 |

---

## 七、修订落地记录（2026-08-19 执行）

| 决策/观察项 | 修订位置 | 落盘内容 |
|-------------|---------|---------|
| Q1 | Design2 §5.6/§5.7；Build6 Step2 | 候选集重算删除「不再属于并集或不再满足可用性过滤」的 group_nodes；节点停用/不可分配/缺失自动摘除分配，重新启用需重新分配；UI 红警保留为防御态。 |
| Q2/Q7 | Design2 §3.2 | 检测触发改为「保存后手动刷新节点」；allocatable 为系统派生标记、检测按协议变化更新；enabled/装配勾选/display_name 仍不覆盖。 |
| Q3 | Design2 §5.10；Design2-UI §9.3；Build6 Step5；Build7 Step5 | 新增 `GET /api/profile/traffic`；`api/profile.ts` + ProfileView 接入。 |
| Q4 | Design2 §5.10；Design2-UI §9.1/§8.5；Build7 Step1/Step3 | 新增 `POST /api/admin/xray/ext/:id/retry`；UI 契约补 `retryExtSync`；后端 RetryExt + 单测。 |
| Q5 | Design2 §5.4；Design2-UI §4.7.2/§10.1；Build7 Step2/Step4 | 导入「无实例/账号且高级关闭」分支显著警告 + 确认词 DISABLE。 |
| Q6 | Build4 Step5 | 两段排序各自维护：manual 段内 MAX+1、URL 段内追加，不再穿越段界。 |
| Q8 | Design2-UI §9.1；Build6 Step5 | `collect_interval_minutes`（API）↔ `xray_collect_interval_minutes`（配置键）显式映射 + 单测。 |
| Q9 | Design2 §5.10；Design2-UI §9.2/§4.7；Build7 Step2/Step3 | 导入与 OFF 清空异步任务 + `GET /api/admin/settings/tasks/:id` 轮询；实例删除/ext 增改删等同步长操作 120s。 |
| Q10 | Design2-UI §5.3.0；Build5 Step3/Step4 | 生成严格校验：悬空引用/未勾选目标拒绝、不可用 xray 节点拒绝、平台无订阅行禁用；强制组允许作规则目标。 |
| Q11 | 未改文档 | 用户确认维持 Build7 实现，中间态可接受（记入本报告结论）。 |
| Q12 | Design2 §5.10；Design2-UI §5.2.2/§9.1；Build4 Step5/Step6 | 新增 `GET /api/admin/pools/:id/sync/tasks` 历史任务端点与池详情历史列表 UI。 |
| P2-1 | Build4 Step1 | 允许启动旧服务做迁移/健康验证；报错时按全新部署清空数据重来。 |
| P2-2 | Build5 Step3/Step7 | 命令改为 `-bench BenchmarkRenderClash10kRules -benchtime=1x` + `TestRenderClash10kRulesThreshold`。 |
| P2-3 | Build4 Step5 | 启动重置 running 任务时同步刷新 `rule_pools.sync_status/sync_error`。 |
| P2-4 | Design2-UI §9.1/§5.2.3 | `getSyncStatus.per_url` 补 `skipped`。 |
| P2-5 | Design2-UI §9.1；Build5 Step4 | preview/generate/blueprint 响应补 `skipped/warnings/name_changed`。 |
| P2-6 | Design2-UI §5.3.5；Build5 Step6 | 预览 diff 明确复用既有版本预览端点拉取当前激活版本原文。 |
| P2-7 | Design2-UI §5.3.5/§9.1；Build5 Step4 | 契约补 `name_changed` 对照信息。 |
| P2-8 | Design2-UI §4.2 | 统一为版本列表无分支，生成/上传回执在 `auto_activated=true` 时提示。 |
| P2-9 | Design2 §5.7；Design2-UI §6.1/§7.1；Build5 Step1/Step2/Step5 | manual 节点删除影响清单与代理组删除影响清单补悬空引用说明。 |
| P2-10 | Design2 §3.2/§5.9；Design2-UI §6.2；Build5 Step1 | 节点名/显示名禁止空格；校验函数与单测同步。 |
| P2-11 | Build6 Step3 | `PushUser` 校验 active；`DiffPush` 对非 active 用户只 Remove 不 Add；补单测。 |
| P2-12 | Design2 §5.10；Design2-UI §9.1/§9.3；Build6 Step2 | GET 组详情 off 不 403，仅省略 nodes/candidate_nodes/default_quota；仅 PUT nodes/quota 受 advancedMode 保护。 |
| P2-13 | Design2 §5.10；Design2-UI §4.7.2；Build7 Step2 | 导入后仅对 enabled=1 实例自动检测，enabled=0 跳过并在完成提示列出。 |
| P2-14 | Design2-UI §9.1/§8.5；Build7 Step1/Step3 | createExt generate 响应一次性返回 `{account, credentials}`；前端展示后即焚。 |
| P2-15 | Design2 §5.3/§5.8/§5.9；Build6 里程碑 | 「1~5 台」改为建议规模、不硬限制。 |
| P2-16 | Design2 §3.3；Build4 Step1 | YouTube 预设组统一为 select。 |
| P2-17 | Build6 Step0 | 明确每实例串行、多实例间可并行。 |
| P2-18 | Build7 Step6 | 删除 `\|\| true`；smoke 脚本存在则失败必须修复。 |
| P2-19 | Design2 §5.4；Design2-UI §4.7.2；Build7 约束表/Step2 | v2 导出/导入 accounts 补 `quota_exceeded`。 |

---

## 八、变更记录


| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 第八轮只读核验：P0=0；P1=12 项（Q1~Q12）经用户确认；P2=19 项逐项确认；形成结论与建议后续处理项。 |
| v1.1 | 2026-08-19 | 按用户指令执行全部文档修订：Design2 / Design2-UI / Build4~7 已按第四章确认结果落盘，新增第七节修订落地记录。 |
