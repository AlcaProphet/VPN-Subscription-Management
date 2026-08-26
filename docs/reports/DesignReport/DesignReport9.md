# Design2Report10.md — Design2 / Design2-UI / Build4~7 第十轮只读设计核验报告

> **报告定位：** 本文档是对 [Design2.md](../../Design2.md)、[Design2-UI.md](../../Design2-UI.md) 与 [Build4.md](../../Build4.md)～[Build7.md](../../Build7.md) 的**第十轮完整只读设计核验报告**。
> **审核对象：** 当前工作区版本的六份活跃文档（Design2.md、Design2-UI.md、Build4.md、Build5.md、Build6.md、Build7.md）。
> **审核约束：** 本轮全程只读，未开始构建，未修改任何设计/构建文档、代码或配置；仅在用户确认后创建本报告。
> **审核时间：** 2026-08-19
> **当前执行状态：** Build4～7 均为「活跃（未验收）」规划文档；后端尚无 1009 迁移与 `internal/xray`/`internal/node`/`internal/assembly`/`internal/pool` 实现；工作区在核验期间 `git status` 干净。

---

## 一、核验范围

| 对象 | 文件 | 规模（行） | 核验内容 |
|------|------|-----------|---------|
| 设计文档 | `Design2.md` | 435 | 第一~五章全量：模式分层与开关语义、规则素材池、装配拼接、配置生成与分发、Xray 对接（5.1~5.11） |
| GUI 规格 | `Design2-UI.md` | 671 | 全量十章 + 未受影响页面核验结论：用户端、管理面板改造页、订阅装配页、节点/代理组/Xray 实例页、前后端契约、交互约定与空态文案 |
| 构建文档 | `Build4.md` | 899 | Step 0~7：Go 1.26、1009 迁移、旧分发模型拆除、版本激活语义、前端基线、规则素材池 |
| 构建文档 | `Build5.md` | 538 | Step 1~7：协议注册表与 manual 节点、代理组、四装配器渲染内核、装配端点、节点/代理组/装配前端、分发收口 |
| 构建文档 | `Build6.md` | 602 | Step 0~6：xray-core 客户端、实例与节点检测、高级中间件与组模型、生命周期同步、下载动态渲染、流量配额、假 Xray 集成测试 |
| 构建文档 | `Build7.md` | 436 | Step 1~6：独立账号与对账、OFF 清空与导入导出 v2、Xray 实例页、组/用户/设置高级前端、用户端收口、全量验收 |
| 基准参照 | `AGENTS.md`、`docs/AchievedDocuments/Design1.md`、`Design1-UI.md`、`Design2Report5/7/8/9.md`、`docs/DocTemplates/*`、`docs/Reference/*` | 按需抽查 | 既有决策闭环、模板/参考事实、报告连续性 |
| 既有代码 | `backend/`（migrations、version/download/platform/config/export/settings 等）、`frontend/`（router、api、SettingsView 等） | 只读抽查 | 验证 Build 文档对现状代码事实的假设是否成立 |

重点闭环核查项：基础/高级模式边界、advanced_mode 开关语义、OFF 清空、ON 初始化、订阅装配、规则素材池、节点/代理组/候选集、下载渲染、流量配额、Xray 对账、独立账号、配置导入导出、首页/个人中心展示。

---

## 二、核验方法

1. **全量通读 + 交叉核对**：六份文档逐节通读；Design2 ↔ Design2-UI 逐条对照承载完整性；Design2 章节 ↔ Build4~7 Step/执行约束表/验收标准逐项对齐；四份 Build 范围边界与候选清单互查。
2. **结构化只读检索**：对端点清单、关键字段（advanced_mode / product_type / target_syntax / display_name / allocatable / quota / traffic / sort_order / pool_sync_tasks / task_id 等）、状态机与错误码做跨文档比对；核对引用章节号指向。
3. **数据模型核验**：提取 Build4 Step1 的 1009 参考 SQL，与 Design2 §5.9 全部表/列/外键/索引逐一比对；对迁移编号、旧表删除、dataclear 清表清单做事实核验。
4. **代码基线事实抽样（只读）**：核对 `backend/go.mod` 与 Dockerfile 的 Go 版本、`version.Service` 现有 CreateVersion/setCurrentLocked 行为、`platform.Service.Delete` 是否走订阅删除回调、`config/export.go` 的 IMPORT 确认词与 v1 导入语义、`frontend/src/views/admin/SettingsView.vue` 的导入交互、`request.ts` 15s 全局超时等 Build 文档依赖的现状事实。
5. **双视角动线走查**：以「普通用户」与「管理员」两条主链模拟全流程，核对功能语义、界面表现、权限控制与提示文案一致性。
6. **专项口径核对**：空态/加载态/错误态/权限可见性/移动端适配/暗色模式/防重复提交/危险确认/超时轮询/日志脱敏/下载禁缓存/事务边界/幂等性/级联删除/候选集重算/凭据加密/Xray 串行调用与失败重试。
7. **执行约束遵守**：全部终端命令为短时只读检索（grep/sed/python 只读脚本），无安装依赖、无启动长服务、无仓库状态变更；发现问题后先提问确认，再按用户确认结果落盘本报告。

---

## 三、核验结论总览

**总体判断：六份文档主线设计正确、自洽、可实施，Build4→7 的章节覆盖、Step 划分、文件清单、验收标准与候选清单总体对齐；未发现 P0 级硬冲突。** 上一轮（Report9）已定案的 38 项问题经抽查均已在当前文档中闭合。

本轮共确认 **15 项问题（P1 11 项 / P2 4 项）**，其中 14 项经用户逐项确认处理口径，1 项（Q6）用户确认该场景不存在、无需处理；另有 Q12 批量低级项 10 项全部采纳。问题主要集中在三类：① 异步任务/候选集重算等 Build 落点裂缝；② 版本事务失败清理与版本 ID 传递等可实施性细节；③ 导入确认词、子组引用、占位替换等语义边界。

**普通用户 / 管理员双视角动线结论**：除上述待修订项外，两条主链语义一致；权限显隐、危险确认、超时轮询、下载禁缓存、级联删除、凭据加密与 Xray 串行/重试均在设计或 Build 中有明确落点。

---

## 四、发现的问题清单与用户确认结果

> 级别定义：**P1** = 构建前必须处理（阻断对应 Step 验收或存在数据一致性/安全风险）；**P2** = 构建前建议处理（文档收口或实现细节，不处理会形成偏差）。
> 类型分为：设计冲突 / 语义不明 / Build 未对齐 / 遗漏设计点 / 不可实施风险 / 需要用户决策项。

### 4.1 P1（11 项，全部经用户确认）

| # | 类型 | 位置 | 问题摘要 | 级别 | 用户确认结果 |
|---|------|------|---------|------|-------------|
| Q1 | Build 未对齐 / 不可实施风险 | Build6.md:180,344 引用「全局任务 registry」；Build7.md:197 才定义查询端点；Build6 各 Step 无实现落点 | Build6 Step1 实例删除、Step3 初始化要求返回 `task_id` 并查询 `GET /api/admin/tasks/:id`，但 registry 组件与查询端点在 Build6 未实现，造成 Build6 无法独立验收。 | P1 | **Build6 补共享 task registry + 查询端点**，Build7 仅复用。 |
| Q2 | 遗漏设计点 | Design2.md:301；Build6.md:190-192,255-259；代码 `platform.Service.Delete`（platform.go:445-496 直接 SQL 删订阅） | 候选集重算缺两处触发：① 平台删除级联删订阅未接 subscription 删除回调/重算；② 检测更新 allocatable 未收集变化节点并回调（只收集 missing/recovered）。 | P1 | **两处都补接线**：platform.Delete 事务后按收集订阅 ID 触发重算；DetectNodes 收集 allocatable 变化节点并回调。 |
| Q3 | 不可实施风险 / 事务边界 | Build4.md:524-527,572-573；Build5.md:381；现有 version.go:231-240 | CreateVersion 先 setCurrent（写 symlink）后 AfterCreate；AfterCreate 失败只删新文件并回滚事务，未恢复 current symlink，会留下悬空指针。 | P1 | **调整顺序：同事务内先 AfterCreate 后 setCurrent**，失败时尚未动 symlink。 |
| Q4 | Build 未对齐 | Design2-UI.md:42,359 vs Build5.md:448 | Build5 DiffView 写「目标版本不存在时整体新增」；UI 写「无激活版本时对比开关隐藏、无整体新增分支」。 | P1 | **按 Build5 修订 UI**：保留 DiffView 无目标「整体新增」能力，UI §1.2/§5.3.5 相应回改交互（无激活版本时对比开关可展示并将空目标视为整体新增）。 |
| Q5 | 设计冲突 / 需要用户决策 | Design2-UI.md:261,598；Build7.md:201,307,310；config/export.go:28,142；SettingsView.vue:448,856 | 导入确认词双轨：现有端点强制 IMPORT；「无实例/账号且高级关闭」分支又要求 DISABLE；且 v1 同步返回 message、v2 异步返回 task_id，前端无分支定义。 | P1 | **双确认词分步**：导入先校验 IMPORT；v2 命中 DISABLE 破坏分支时追加第二个 DISABLE 输入；v1 同步不轮询、v2 有 task_id 才轮询。 |
| Q7 | 语义不明 | Design2.md:104-107；Design2-UI.md:429；Build5.md:291-292 | 勾选组引用了「存在但未勾选」的其他代理组时，产物会引用未输出的 proxy-group；Build5 只写「悬空引用拒绝」，未覆盖该分支。 | P1 | **生成校验拒绝并定位**：凡引用子组不在本次输出集合内，preview/generate 返回 400 并提示「组 X 引用了未勾选的组 Y」；不自动改勾选。 |
| Q8 | 设计冲突 | Design2.md:98（仅节点） vs Design2-UI.md:324、Build5.md:291（节点/子组成员） | 「🌎国外流量」成员范围三份文档不一致。 | P1 | **仅允许节点**：回改 UI §5.3.1 与 Build5 模型/校验，Design2 保持「从本次装配候选节点勾选」。 |
| Q9 | 遗漏设计点 / Build 未对齐 | Design2-UI.md:472（行内同步操作） vs §9.1/Build7.md:131-133（仅批量异步端点） | 对账单条补推/凭据修复的同步端点契约缺失。 | P1 | **新增单条同步端点**：为对账单条补推与单条凭据修复补端点契约，并写入 Build7 Step1/Step3 实现与 120s 口径。 |
| Q11 | 不可实施风险 / Build 未对齐 | Build4.md:524,556；Build5.md:381；1009 SQL assembly_blueprints.version_id REFERENCES versions(id) | AfterCreate 只传 versionNo，而 blueprint 需要 versions.id；伪代码直接把 `no` 当 version_id 使用。 | P1 | **AfterCreate 改为传版本行 ID**：Build4 Step3 签名改为传入新插入行的 `versions.id`，SaveBlueprint 直接写 version_id。 |
| Q13 | 不可实施风险 | Build6.md:115-122（Client 内锁）；Step1/Step5 未定义 client 缓存 | 若每次请求新建 Client，检测/采集/推送并发会绕过「每实例串行」保证。 | P1 | **实例服务持有每实例 Client 并复用**：Build6 明确按实例缓存同一 Client，所有 gRPC 调用共用同一锁。 |
| Q15 | 语义不明 / 幂等性 | Build7.md:189-190 | OFF 状态翻转判定与任务创建未定义在同一事务/互斥内，两个管理员并发提交可能产生两个清空任务。 | P1 | **状态翻转判定与任务创建同事务互斥**：BEGIN IMMEDIATE 事务内读 advanced_mode、判定翻转并落位/登记任务；第二次提交按幂等 no-op 返回。 |

### 4.2 P2（4 项，全部经用户确认）

| # | 类型 | 位置 | 问题摘要 | 级别 | 用户确认结果 |
|---|------|------|---------|------|-------------|
| Q6 | 遗漏设计点 / 不可实施风险 | Build7.md:201,203；config/export.go:135-188（v1 整体替换 signing_key） | v1 配置导入不适用「signing_key 将变化且存在业务密文则拒绝」的保护，理论上可使既有 UUID/代理密码/节点凭据密文不可解密。 | P2 | **用户确认该场景不存在（不存在已有 v2 数据后再导入 v1 的场景），无需处理**；仅在本报告记录口径。 |
| Q10 | 遗漏设计点 | Design2.md:328；Build6.md:405-412 | 有凭据但「组分配 ∪ 公共节点」为空时，SR subs/generic-subs 占位替换结果未定义（仅定义了 UUID 为空与 advanced off 两分支）。 | P2 | **替换为空行/移除占位**：建议实现为移除整行（避免空行噪音），补 Build6 单测。 |
| Q14 | 遗漏设计点 / 性能风险 | Build6.md:507-519 伪代码逐用户 `recordCollectError(...); continue` | 实例不可达时会把一次实例级故障放大为 N 次拨号/30s 超时，采集任务长时间不收敛。 | P2 | **先做一次廉价连通性探测再决定**：探测失败则跳过该实例本轮并记录实例级 collect_error/告警，不逐用户重试。 |
| Q12 | Build 未对齐 / 遗漏设计点 / 语义不明（批量） | 见 4.3 | 10 项低级 Build/UI 不一致与小缺口。 | P2 | **10 项全部采纳**，按推荐口径修订。 |

### 4.3 Q12 批量低级项（10/10 已采纳）

| # | 位置 | 问题摘要 | 用户确认口径 |
|---|------|---------|-------------|
| Q12-1 | Build7.md:117 vs Design2-UI.md:550 | Build7 Step1 `GetExtCredentials` 未写 no-store。 | 补 no-store。 |
| Q12-2 | Build7.md:254 vs Design2-UI.md:585 | Build7 Step3 120s 清单漏 `testConnection`（连通性测试）。 | 补 testConnection 120s。 |
| Q12-3 | Design2-UI.md:454；Build7 Step3 | 实例删除为异步 task_id，但 §8.1 删除弹窗未写 pollTask 轮询分支。 | 补删除实例的 task_id 轮询 UI 流程。 |
| Q12-4 | Build7 Step3 vs Design2-UI §8.2/8.4 | 对账/节点检测在实例不可达时的错误态与重试文案未定义。 | 补 gRPC 失败 error 展示与重试引导。 |
| Q12-5 | Build7 Step4 vs Design2-UI.md:256 | 采集间隔控件「仅高级模式展示」未写入 Build Step。 | Build7 Step4 明确与 UI §4.7 一致。 |
| Q12-6 | Design2-UI.md:547-548；Build7 Step1 | 独立账号 push_targets 请求形状未定义；编辑移除已推送目标无影响提示。 | 明确 `[{instance_id, inbound_tag}]` 形状；移除已推送目标前增加影响提示。 |
| Q12-7 | Build6 Step1；Design2-UI §8.1 | 实例编辑 api_addr 后既有节点 host/port 可能陈旧，无重新检测提示。 | 实例编辑成功后提示「建议刷新节点」。 |
| Q12-8 | Build4.md:693 | 池同步 SubmitSync「查 running → 插入」未明确同一事务，存在并发双任务风险。 | 明确 BEGIN IMMEDIATE 事务内查+插。 |
| Q12-9 | Build4 Step5 | 删除同步中的池 / 同步期间更新 URL 的边界未说明。 | 补边界：终态写回失败仅记日志不崩溃；历史任务保留。 |
| Q12-10 | Design2.md:84,369；Design2.md:118-121 vs Build5.md:292 | ① §5.9「manual 来源 ClashOfficial 全量协议（ssr 除外）」与 §3.2/Build5 的 19 协议封闭清单冲突；② xray 节点名示例 `tokyo-a-vless` 与 `instance-` 前缀 slug 规则不符；③ 手动规则行「追加在池后」只在 Build5 定义、Design2 未写。 | 按 19 协议清单统一措辞；示例改为 instance- 前缀风格；回写 Design2 §3.5 手动规则行顺序。 |

---

## 五、双视角关键动线核验结论

### 5.1 普通用户视角

- 注册/审批/管理员创建激活 → 高级模式下异步钩子生成 UUID/代理密码并 AddUser；基础模式零 Xray 行为：链路完整。
- 首页卡片顺序（流量 → 分流规则 → 平台 → 公告）、基础模式「不限流量」、高级模式三态进度条与超限文案、流量卡片开关显隐：Design2 §一 / UI §3.1 / Build4+7 落点一致。
- 平台卡片三态（ready/custom/unassigned）与「无激活版本隐藏三按钮 + 下载 200 注释块」双口径一致；管理员平台卡改预览形态：落点一致。
- 下载动线：无标识 Token → 平台唯一订阅 → 当前版本；直接上传原样、装配模板动态渲染；超限不阻断下载：落点一致。
- 个人中心：基础隐藏所属组、新增本月流量行、高级超限附注：落点一致。

### 5.2 管理员视角

- 基础模式动线：平台（product_type）→ 订阅条目 → 上传/装配入池 → 显式激活 → 用户可见：闭环。
- 高级模式动线：设置开高级（仅置位不推送）→ 录实例/测试/检测 → 装配蓝图激活 → 组分配（候选集约束）→ 开始初始化（异步 task_id）→ 对账兜底 → 独立账号 → 导入导出 v2 → OFF 清空（DISABLE）→ 重新开启全量重配：除 Q1/Q2 待补接线外主线闭环。
- 危险确认与防重复：OFF/导入/删除/清空/超限重置等均有用 ConfirmModal 或确认词；异步任务按钮 loading + pollTask；Q15 确认后 OFF 并发口径也闭合。

---

## 六、最终结论

1. **核验通过（附修订条件）**：Design2.md 与 Design2-UI.md 主线设计正确、自洽、可实施；Design2-UI 完整承载 Design2 的界面、交互状态、前后端契约与错误处理；Build4~7 严格按设计编写，Step 划分、执行约束、验收标准、文件清单、端点与数据模型总体对齐，范围边界清晰，未发现提前越界实现或「设计有、Build 无」的整体遗漏。
2. **待修订项**：本轮 P1 11 项与 P2 4 项已全部经用户确认处理口径（见第四节），其中 Q6 经用户确认为不存在场景、无需处理。建议按第七节清单在 Build4 开工前完成文档回写，然后进入构建。
3. **双视角结论**：普通用户与管理员两条主链在权限可见性、提示文案、状态分支上无歧义；修订完成后即可满足「文档核验通过、可进入构建」条件。

---

## 七、建议后续处理项（按用户确认口径修订）

### 第一批（构建前必改，P1）

1. **Q1**：Build6 增加共享全局长任务 registry 组件与 `GET /api/admin/tasks/:id` 查询端点（kind：off_clear/import/xray_init/reconcile_exec/instance_delete）；Build7 Step2 改为复用，不再首次定义。
2. **Q2**：Build6 Step2 触发清单补平台删除；`platform.Service.Delete` 事务后对收集到的订阅 ID 触发候选集重算回调；Build6 Step1 `DetectNodes` 增加 allocatable 变化节点清单与 `OnNodeVisibilityChanged` 回调。
3. **Q3**：Build4 Step3 / Build5 Step4 的 CreateVersion 事务顺序改为「写文件 → 插版本行 → AfterCreate → 计算 effectiveCurrent 与 setCurrent → evictOldest」；AfterCreate 失败时无需恢复 symlink。
4. **Q4**：按用户决策回改 Design2-UI §1.2/§5.3.5，保留 DiffView 无目标「整体新增」分支并说明交互；Build5 不变。
5. **Q5**：定稿导入双确认词分步：现有 IMPORT 校验保留；v2 命中破坏分支时前端追加 DISABLE 输入，后端同时校验两词；前端按响应是否含 task_id 分支（v1 同步 / v2 轮询），回写 Design2-UI §4.7.2/§9.2/§9.3 与 Build7 Step2/Step4。
6. **Q7**：Build5 Step3 validate.go 增加「被引用子组必须属于本次输出集合」校验，preview/generate 400 定位；Design2-UI §5.3.0/5.3.1 补校验文案。
7. **Q8**：Design2-UI §5.3.1 与 Build5 models/validate 改为「🌎国外流量」成员仅节点；Design2 §3.3 保持。
8. **Q9**：Design2-UI §9.1 补单条同步端点契约（建议 `POST /api/admin/xray/instances/:id/reconcile/push-one` 与 `.../credentials-one`）；Build7 Step1 实现、Step3 行内操作接入并统一 120s。
9. **Q11**：Build4 Step3 将 `AfterCreate` 签名改为传入新版本行 ID；Build5 Step4 相应改 `SaveBlueprintTx(ctx, tx, versionID, ...)`。
10. **Q13**：Build6 Step1/Step5 明确实例服务按实例缓存同一 `internal/xray.Client`，所有调用复用其互斥锁。
11. **Q15**：Build7 Step2 明确 OFF 翻转判定与任务创建同事务互斥；并发第二次提交返回幂等 no-op。

### 第二批（构建前建议完成，P2）

12. **Q6**：不修改文档；在报告留档「不存在已有 v2 数据后导入 v1 的场景，用户确认无需处理」。
13. **Q10**：Build6 Step4 补「有凭据但目标集为空」分支：SR subs/generic-subs 移除占位行（用户选择「替换为空行/移除占位」），并补单测。
14. **Q14**：Build6 Step5 采集前先做一次廉价连通性探测（如带 deadline 的 ListInbounds）；失败跳过该实例本轮、写 collect_error/告警，不逐用户重试。
15. **Q12-1~Q12-10**：按 4.3 表逐项回写 Build 与 Design 文档。

### 第三批（修订后复核）

16. 全部修订完成后，按 Q1~Q15 逐项勾销，同步各文档变更记录版本号；确认工作区仅含本报告与预期修订。
17. 复核通过后再从 Build4 Step 0 按序执行构建，构建期间不得跨 Build 或跳步。

---

## 八、修订落地记录（2026-08-19 执行）

按第七节口径完成文档回写，明细如下（未修改任何代码/配置）：

| 文件 | 修订要点 | 对应问题 |
|------|---------|---------|
| `Design2.md` | OFF 提交翻转判定与任务登记同事务互斥；装配生成勾选组引用子组必须属于输出集合；手动规则行追加在池后；v2 导入双确认词 IMPORT→DISABLE 与 v1 同步/v2 异步响应口径；空目标集时 SR/generic 占位移除整行；xray 节点名示例改 instance- 前缀；§5.9 manual 协议措辞统一为 19 协议封闭清单；§5.10 补对账单条同步端点 | Q15、Q7、Q12-10、Q5、Q10、Q12-10、Q9 |
| `Design2-UI.md` | DiffView 恢复无目标「整体新增」分支；导入双确认词分步与 v1/v2 响应分支；装配严格校验补未勾选子组；🌎国外流量成员仅节点；§8.4/§9.1 补 push-one/credentials-one 单条同步端点；§8.1 补实例删除 task_id 轮询与 api_addr 变更提示；§8.2/8.4 补实例不可达错误态；§8.5/§9.1 补 push_targets 形状与移除已推送目标提示；变更记录 v2.0 | Q4、Q5、Q7、Q8、Q9、Q12-2~Q12-7 |
| `Build4.md` | CreateVersion 事务顺序改为「插版本行 → AfterCreate(versions.id) → 计算 effectiveCurrent → setCurrent → evict」；伪代码同步；池同步 SubmitSync 查+插同事务；补同步期间删池/改 URL 边界；变更记录 v1.6 | Q3、Q11、Q12-8、Q12-9 |
| `Build5.md` | OverseasMembers 明确仅节点；validate 补未勾选子组拒绝；AfterCreate 回调改为传 versions.id；单测清单同步；变更记录 v1.5 | Q8、Q7、Q11 |
| `Build6.md` | 全局任务 registry 与 `GET /api/admin/tasks/:id` 在 Step1 落地（文件清单、Step1 正文、Step3 复用注记）；Step1 补 allocatable_changed 清单与回调；Step2 触发点补平台删除；Step3 wiring/单测补 allocatable diff；实例服务按实例缓存 Client；Step4 补空目标集占位整行移除与单测；Step5 补实例级廉价探测与失败快速中止、伪代码与单测；变更记录 v1.6 | Q1、Q2、Q13、Q10、Q14 |
| `Build7.md` | 全局任务端点改为复用 Build6；对账增加 push-one/credentials-one 单条同步端点、路由与单测；OFF 翻转判定与任务登记同事务互斥并补并发单测；导入双确认词 IMPORT→DISABLE、v1/v2 响应分支与单测；getExtCredentials 补 no-store；Step3 补 testConnection 120s、实例删除轮询、检测/对账错误态、api_addr 变更提示、push_targets 形状与移除确认；Step4 采集间隔仅高级展示；变更记录 v1.7 | Q1、Q9、Q15、Q5、Q12-1~Q12-7 |

**未修改项**：`Q6` 按用户确认「不存在已有 v2 数据后导入 v1 的场景」不处理，仅记录于本报告第四节。

---


---

## 九、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 第十轮只读核验：P0=0；P1=11（Q1~Q5/Q7~Q9/Q11/Q13/Q15）；P2=4（Q6/Q10/Q12/Q14）；Q12 含 10 项批量低级项。全部问题经用户确认处理口径后落盘本报告；未修改任何设计/构建文档与代码。 |
| v1.1 | 2026-08-19 | 按用户确认口径执行文档修订：Design2 / Design2-UI / Build4 / Build5 / Build6 / Build7 已落盘 Q1~Q5、Q7~Q15（Q6 不处理），明细见第八节修订落地记录。 |
