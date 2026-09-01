# DesignReport10.md — Design2 / Design2-UI / Build4~7 第十一轮只读设计核验报告

> **报告定位：** 本文档是对 [Design2.md](../Design/Design2.md)、[Design2-UI.md](../Design/Design2-UI.md) 与 [Build4.md](../Build/Build4.md)～[Build7.md](../Build/Build7.md) 的**第十一轮完整只读设计核验报告**（构建前最后一轮系统核验）。
> **审核对象：** 当前工作区版本的六份活跃文档（Design2.md、Design2-UI.md、Build4.md、Build5.md、Build6.md、Build7.md）。
> **审核约束：** 核验阶段全程只读，未开始构建，未修改任何设计/构建文档、代码或配置；发现问题后先整理清单经提问工具与用户逐项确认，再按用户确认结果创建本报告并执行修复（修复记录见第七章）。
> **审核时间：** 2026-08-19
> **当前执行状态：** Build4～7 均为「活跃（未验收）」规划文档；后端尚无 1009 迁移与 `internal/xray`/`internal/node`/`internal/assembly`/`internal/pool` 实现。

---

## 一、核验范围

| 对象 | 文件 | 规模（行） | 核验内容 |
|------|------|-----------|---------|
| 设计文档 | `Design2.md` | 435 | 第一~五章全量：模式分层与开关语义、规则素材池、装配拼接、配置生成与分发、Xray 对接（5.1~5.11） |
| GUI 规格 | `Design2-UI.md` | 673 | 全量十一章：用户端、管理面板改造页、订阅装配页、节点/代理组/Xray 实例页、前后端契约、交互约定与空态文案、变更记录 |
| 构建文档 | `Build4.md` | 902 | Step 0~7：Go 1.26、1009 迁移、旧分发模型拆除、版本激活语义、前端基线、规则素材池 |
| 构建文档 | `Build5.md` | 539 | Step 1~7：协议注册表与 manual 节点、display-name 服务、代理组、四装配器渲染与分发 |
| 构建文档 | `Build6.md` | 612 | Step 0~6：xray-core 客户端、实例与节点检测、组分配与候选集、用户同步、下载动态渲染、流量配额 |
| 构建文档 | `Build7.md` | 440 | Step 1~6：对账、独立账号、导入导出 v2、OFF 清空、高级 UI 收口 |
| 基准参照 | `AGENTS.md`、`docs/reports/*`（Design1 系列、DesignReport1~10）、`docs/DocTemplates/*`、`docs/Reference/*` | 按需抽查 | 既有决策闭环、模板/参考事实、报告连续性 |
| 既有代码 | `backend/`（migrations、version/download/platform/group/subscription/home/emergency/dataclear 及各测试）、`frontend/`（router、api、views） | 只读抽查 | 验证 Build 文档对现状代码事实的假设是否成立 |

重点闭环核查项（用户指定）：基础/高级模式边界、advanced_mode 开关语义、OFF 清空、ON 初始化、订阅装配、规则素材池、节点/代理组/候选集、下载渲染、流量配额、Xray 对账、独立账号、配置导入导出、首页/个人中心展示；以及 Design2-UI 对 Design2 全部界面、交互状态、前后端契约与错误处理要求的承载完整性。

---

## 二、核验方法

1. **主审全量通读**：主审逐行通读 Design2.md 与 Design2-UI.md 全文，建立设计权威基准。
2. **四路并行对照审阅**：每份 Build 文档由独立只读审阅流对照两份设计逐条核对（Step 划分/执行约束/验收标准/文件清单/端点设计/数据模型/UI 落点/测试要求；引用章节号、字段名、状态机、错误码、文案逐字比对；范围边界互查是否提前实现或遗漏），并抽查仓库实际代码验证可实施性。
3. **16 项清单式遗漏点专项**：空态/加载态/错误态、权限可见性、移动端适配、暗色模式、防重复提交、危险确认、超时/轮询、日志脱敏、下载禁缓存、事务边界、幂等性、级联删除、候选集重算、凭据加密、Xray 串行调用与失败重试，外加 Design2 §5.10 后端 16 项与前端 11 项清单逐项打勾、双视角动线走查、设计文档内部一致性与 Build 间冲突核查。
4. **主审独立复核关键交叉点**：reconcile 端点归属（Build6 红线划归 Build7）、OFF 三层钩子检查归属（Build6 承担）、占位注释优先级（`# Xray 高级模式未启用` 优先于「节点未开通」）、anytls 链接映射出处（Node-Link-Standards.md §二已有细则，Build5 纳入可转有据）等。
5. **双视角动线走查**：以「普通用户」（注册→首页四卡→下载注入/无凭据注释/超限不变→个人中心本月流量→/rules）与「管理员」（Setup→平台 product_type→订阅条目→素材池→装配→激活→开高级→实例→检测→组分配→初始化→对账→独立账号→导入导出→OFF 清空）两条主链逐步核对功能语义、界面表现、权限控制与提示文案一致性。
6. **用户确认闭环**：全部发现按类型（设计冲突/语义不明/Build 未对齐/遗漏设计点/不可实施风险/需要用户决策）整理为分级清单，经提问工具与用户逐项确认（14 个决策单选 + 2 组修正多选）后方才定稿。

---

## 三、核验结论总览

- **高级问题（阻塞构建/执行即冲突）：0 项。** 设计文档主体正确、自洽、可实施；Design2-UI 完整承载 Design2 的界面与契约要求；Build4~7 严格按设计编写、范围边界清晰、无提前实现、无关键遗漏。
- **中级别问题：15 项**（Build 未对齐 5、遗漏设计点 4、语义不明 4、不可实施风险 2），详见 4.2。
- **低级别问题：20 项**（编辑性冲突/文案/验收命令/零星落点），详见 4.3。
- **经核验无问题的主体面**：模式分层与 OFF/ON 语义（确认词 DISABLE/IMPORT、幂等 no-op、三层钩子检查、收集-提交-清理顺序）、装配全链路与四类装配器、规则素材池全部细则（两段排序/逐行解析/失败语义/pool_sync_tasks 持久化）、节点双来源与有效渲染名跨命名空间校验、代理组（强制组/预设库/DAG/空组硬校验/子组输出集合校验）、候选集重算事件全集、下载渲染①~⑤与占位注释优先级、流量配额（采集/超限/重置/UTC 月界/已知损失口径）、对账四分区与期望集公式、独立账号全细则、导入导出 v2 三分支与双确认词、级联 FK 链、凭据加密、权限可见性（菜单/守卫/列显隐/403 中间件）、pollTask 与全局任务端点两套机制、§5.10 后端 16 项+前端 11 项清单落点、双视角动线——均闭环一致。

---

## 四、发现的问题与级别

### 4.1 高级问题

无。

### 4.2 中级别问题（15 项）

| # | 类型 | 位置 | 问题 |
|---|------|------|------|
| M1 | Build 未对齐 | Build4 Step2 | 验收要求 grep 旧表名（group_selections/subscription_group_rel）=0，但 `emergency_test.go`/`platform_test.go` 夹具含旧表且不在任何文件清单，验收必卡 |
| M2 | 遗漏设计点 | Build4 Step4 | 订阅上传后「已入池未生效，请激活」引导（Design2 §4.4 / UI §4.1 明示）无落点 |
| M3 | Build 未对齐 | Build4 Step2/4 | 组详情未规定 advanced_mode=off 时省略 `default_quota`，与 Design2 §5.10 / UI §9.1 契约冲突 |
| M4 | 不可实施风险 | Build4 Step5 | 素材池差量删除 `NOT IN (...)` 数万行超 SQLite 参数上限，与 §2.4「数万行规模」冲突 |
| M5 | 语义不明 | Build5 Step4 | 用户预览 subs/generic-subs 装配模板的编码形态（明文 vs base64）两份设计均未定义 |
| M6 | 需要用户决策 | Build5 Step1/3 | 两处「Build 期决策」未经确认：①manual 节点协议变更=重新填表；②停用预设组 400 拒绝生成 |
| M7 | 遗漏设计点 | Build5 Step3/4 | `assembly_blueprints` 四列（fixed_params/selection/custom_rules/render_plan）写入与重编辑恢复映射未闭环，selection_json 全文未出现 |
| M8 | Build 未对齐 | Build6 Step1 | 引用 `node.ValidateName`，Build5 实际导出为 `ValidateNodeName`，按文档执行编译不过 |
| M9 | 遗漏设计点 | Build6 Step4 | Design2 §5.7「下载文件名按 target_syntax 区分」（.yaml/.txt/.conf）在 Build6 无任何落点 |
| M10 | 语义不明+安全 | Build6 Step1 | xray 节点检测归一时 REALITY 私钥是否入库/是否加密未定义，处理不当违反「密钥加密存储」硬约束 |
| M11 | 语义不明 | Build6 Step3 | missing 0→1（RemoveUser diff）与 1→0（AddUser diff）合并表述，diff 方向与清单来源易误读 |
| M12 | 重复定义 | Build7 Step1 | 「实例/节点删除钩子补丁」与 Build6 Step1/3 逐字重复 |
| M13 | 需要用户决策 | Build7 Step1 | 独立账号 generate 模式「already exists 视为接管成功」超出 Design2 §5.11 手填接管范围，静默接管将致面板复制凭据与 Xray 侧不一致 |
| M14 | 不可实施风险 | Build6 Step2 | 组分配保存 SetNodes「先删后插」无事务/写锁声明，与 AGENTS §4.6 原子性要求存在隐患 |
| M15 | 遗漏设计点（2 项） | Build5 Step5/Step7 | 节点 enabled 1→0 停用 ConfirmModal（UI §6.1/§10.1）无落点；VersionManageView 空态「暂无版本」+装配生成引导（UI §10.2 新定义）无落点 |

### 4.3 低级别问题（20 项）

| # | 类型 | 位置 | 问题 |
|---|------|------|------|
| L1 | 设计冲突（编辑性） | Design2 §5.10 | 构建注记「Build6 Step 0 引入 xray-core 依赖之后进入 1009 增量 DDL」与 Build4 Step1 即执行迁移不一致 |
| L2 | 设计冲突（编辑性） | Design2-UI §5.3.3/5.3.4 | 目标规则实体选择「⑥ 生成前」与「步骤①或⑥前置」并存，Build5 实现落在步骤① |
| L3 | 设计冲突（编辑性） | Design2-UI 变更记录 v1.1 | 「8.4 对账区改三分区」与现行 8.4 四分区结构不符 |
| L4 | 语义不明 | Design2 §5.11/§5.9 vs UI §8.5、Build7 Step1 | 独立账号推送目标过滤：UI/Build 比设计正文多 `nodes.enabled=1`（可视为 §5.5 口径继承，字面不一致） |
| L5 | Build 未对齐 | Build5 约束表 | 节点名称规则漏写「空格」禁止项（与 Design2 §3.2/§5.9 及 Build5 自身伪代码冲突） |
| L6 | Build 未对齐 | Build5 Step3 伪代码 | 🌎国外流量组输出含「🚀直接连接」子组，与 Q8「仅节点」口径冲突 |
| L7 | Build 未对齐 | Build4 Step3/4 | 后端「产物格式不可变更」文案不含类型插值，与 UI §4.4 定稿文案（含 {yaml|subs|generic-subs}）不一致 |
| L8 | 语义不明 | Build4 Step5/6 | 「同步进行中」409 前端特判缺失（UI §5.2.3 要求 message.warning，§9.4 通用 409 为 Notify.error） |
| L9 | 语义不明 | Build4 Step4 | 路由守卫 `status.advanced_mode === false` 在状态未加载时不重定向（深链短暂可见） |
| L10 | 语义不明 | Build4 Step3 | home_rule/traffic 挂 `/api/home/platforms` 响应顶层为 Build4 单方钉死的契约（UI §9.3 仅说「首页端点」） |
| L11 | 需要用户决策 | Build4 Step2 | 空池下载注释两态拆分（无订阅行 `# error: unassigned` / 无激活版本 `# error: no active version`）vs Design2 §4.4 单一口径 |
| L12 | 需要用户决策 | Build4 Step5 | 池 URL http/https scheme 校验 vs Design2 §2.4「目标地址不设限制」字面 |
| L13 | 语义不明 | Build6 Step5 / UI §9.3 | 不限流量时 quota_bytes 省略键还是 null，两文档措辞不一 |
| L14 | 语义不明 | Build6 Step4 | Clash 蓝图重渲染下「# 节点未开通，请联系管理员」注释的输出落点未定义（重渲染产物天然不含占位行） |
| L15 | 语义不明 | Build6 Step3 | 超限跳过「记原因」落点（xray_users.last_error 还是仅日志）未明确，影响用户管理页失败红标口径 |
| L16 | 语义不明 | Build6 Step1 | xray 节点 host 取实例 api_addr host 部分与「不虚构默认值」原则存在张力，设计未定义 host 来源 |
| L17 | 需要用户决策（Build 边界） | Build6 / Build7 Step1 | 独立账号采集与配额检查推迟至 Build7 Step1（Design2 §5.8 字面属采集任务本体） |
| L18 | Build 未对齐（编辑性） | Build6/Build7 | Build6：kind「全量预留给 Build7」措辞（instance_delete/xray_init 本 Build 即落地）、Step3/5 重复声明 List 字段、`grpc.WithTimeout` 已移除（有 context 备选）、profile-web-page-url 未写明取站点 URL；Build7：120s 清单误纳异步提交端点、参数表漏「节点 enabled=1」、「期望集来源」措辞歧义 |
| L19 | 遗漏落点 | Build4/5/7 | 平台卡无版本占位文案+三按钮隐藏、新建订阅轻提示、实例/独立账号/装配 xray 区三处空态引导文案（policy.stats 前置提示等）、DiffView「整体新增」提示、删除默认规则回空态断言、api/settings.ts 的 importConfig/getAdminTask 函数点名、订阅列表「内容形态标签」列未消费 |
| L20 | 验收/命令 | Build5 Step7 / Build4 Step4 | Build5 验收 grep「即将推出」与 Build4 实际占位文案「将在下一轮构建实现」不符（恒通过）；Build4 home_test/rule_test 实为新建而非「同步更新」；Build5 约束表 go test 路径笔误（缺 `/...`） |

> 备注：遗漏点专项原列 anytls 链接映射依据不足一项，经核查 `docs/Reference/Node-Link-Standards.md` §二已有 anytls 映射细则，Build5 纳入可转有据，**从问题清单移除**。

---

## 五、用户确认结果（2026-08-19，提问工具逐项确认）

### 5.1 决策项（14 项，12 项采纳推荐、2 项选择非推荐方案）

| # | 问题 | 确认结论 |
|---|------|---------|
| Q1 | 用户预览 subs/generic-subs 装配模板编码形态 | **明文原文**（与存储形态、管理员预览一致；base64 仅为下载下发编码） |
| Q2 | manual 节点编辑是否允许变更协议 | **允许，变更=整体重新填表**（不保留不兼容旧字段；凭据留空=保留原凭据） |
| Q3 | 勾选到已停用预设组的装配行为 | **400 拒绝生成并提示**「预设组已停用，请先启用或移除勾选」，不纳入失效项剔除 |
| Q4 | xray REALITY 私钥处理 | **私钥不落库，仅存推导公钥 public_key** |
| Q5 | 不限流量时 quota_bytes | **统一返回 null** |
| Q6 | Clash 重渲染无凭据注释 | **输出注释行**（proxies 区首行），与 SR/generic 语义一致 |
| Q7 | 超限跳过记因落点 | **保持 xray_users 行并写 last_error** |
| Q8 | xray 节点 host 来源 | **取实例 api_addr 的 host 部分** |
| Q9 | 独立账号采集与配额归属 | **接受 Build7 Step1 补入**（与 ext CRUD 同轮闭环） |
| Q10 | generate 模式同 email 已存在 | **先 RemoveUser 再 AddUser**（以新凭据覆盖；确认文案须提示踢除风险）——**非推荐项，用户决策** |
| Q11 | 平台无订阅行下载注释 | **保留 Build4 两态拆分**（`# error: unassigned` / `# error: no active version`） |
| Q12 | 池 URL 校验 | **保留 http/https scheme 校验**（仅基本合法性校验，不设地址白名单） |
| Q13 | home_rule/traffic 承载端点 | **独立端点 `GET /api/home/summary`**（/api/home/platforms 保持纯列表）——**非推荐项，用户决策** |
| Q14 | 独立账号推送目标过滤 | **含 nodes.enabled=1，回写 Design2 §5.11** |

### 5.2 修正项（多选，全部采纳）

- **中级别修正 12 项全选**：M1 测试清单补两文件；M2 入池引导补 Build4 Step4；M3 组详情 off 省略 default_quota；M4 差量删除改临时表 JOIN/分批；M7 蓝图四列映射明示；M8 函数名修正 ValidateNodeName；M9 下载文件名映射落 Build6 Step4；M11 missing 表述拆分；M12 Build7 删除钩子去重（仅补 ext 单测）；G1 版本空态落 Build5 Step7；G2 enabled 1→0 ConfirmModal 落 Build5 Step5；G3 组分配保存显式 BEGIN IMMEDIATE 事务。
- **低级别修正 13 组全选**：Build5 伪代码去子组/约束表补空格/验收 grep 修正/DiffView 提示文案；Build4 后端文案含类型插值/409 前端特判/守卫 `!== true`/空态文案与轻提示等零星落点；Build6 措辞与 WithTimeout 等四处编辑性修正；Build7 120s 清单剔除异步端点/参数表补 enabled=1/空态显式列；Design2 §5.10 迁移时点注记；UI §5.3.3/5.3.4 统一步骤①；UI v1.1 变更记录勘误。

---

## 六、最终结论

**核验通过，六份文档经本轮定点修复后达到「构建前无疑点」状态（满足 AGENTS §3.3），可按 Build4 Step 0 启动构建。** 全部发现均不涉及主体方案返工；Q10（generate 模式先 Remove 再 Add）与 Q13（独立 /api/home/summary 端点）两项非推荐决策已按用户选择落盘，其连锁影响（Build7 Step1 覆盖语义与确认文案；Build4 Step3/4、Build6 Step5、Build7 Step5 的端点调整）已纳入修复范围。

---

## 七、修复记录

> 修复时间：2026-08-19。依据第五章用户确认结果执行，仅修改六份活跃文档，未触碰任何代码与配置。各 Build 文档变更记录表已同步追加新版本行（Build4 v1.7 / Build5 v1.6 / Build6 v1.7 / Build7 v1.8；Design2-UI v2.1）。

### 7.1 Design2.md（14 处）

| # | 位置 | 修复内容 | 对应 |
|---|------|---------|------|
| 1 | §5.10（cmd/server/main.go 注记） | 「Build6 Step 0 之后进入 1009 增量 DDL」修正为「1009 增量 DDL 于 Build4 Step 1 执行」 | L1 |
| 2 | §5.11 推送集合口径 | 补「节点 enabled=1」过滤条件 | Q14/L4 |
| 3 | §5.9 xray_ext_users 行 | 同步补「节点 enabled=1」 | Q14/L4 |
| 4 | §4.3 与 §5.7 预览口径（2 处） | 用户预览 subs/generic-subs 装配模板返回明文原文（base64 仅为下载下发编码） | Q1/M5 |
| 5 | §5.7 注入条件 | Clash 模板按蓝图全量重渲染时在 proxies 区首行输出 `# 节点未开通，请联系管理员` 注释行 | Q6/L14 |
| 6 | §5.5 超限前置拦截 | 「记原因」明确为写 `xray_users.last_error`（UI 经失败 Tooltip 提示先重置配额） | Q7/L15 |
| 7 | §5.9 nodes 行 | 新增：xray 节点 host 取实例 api_addr 的 host 部分；REALITY 私钥不落库、仅写推导公钥 `public_key` | Q8/L16、Q4/M10 |
| 8 | §5.8 独立账号采集与配额 | 补构建落点注记：ext 采集与配额检查随 Build7 Step1 补入 | Q9/L17 |
| 9 | §5.11 凭据来源（双轨） | 新增：generate 模式同 email 已存在 → 先 RemoveUser 再 AddUser 覆盖（须提示踢除风险） | Q10/M13 |
| 10 | §4.4 空池下载口径 | 单一口径拆为两态：无订阅行 `# error: unassigned`、无激活版本 `# error: no active version` | Q11/L11 |
| 11 | §2.4 约束 | 「目标地址不设限制」改为「不设白名单限制（仅做 http/https scheme 与基本合法性校验）」 | Q12/L12 |
| 12 | §3.2 协议可扩展注册 | 新增：manual 节点编辑允许变更协议=整体重新填表、不保留不兼容旧字段 | Q2/M6① |
| 13 | §3.3 预设组库 | 新增：勾选已停用预设组拒绝生成（400），不纳入失效项剔除容错 | Q3/M6② |
| 14 | §5.10 internal/server/home.go | 改为新增独立汇总端点 `GET /api/home/summary`（traffic 含 `quota_bytes=null` 口径 + 首页默认规则卡片字段）；`/api/home/platforms` 保持纯列表 | Q13/L10/L13 |

### 7.2 Design2-UI.md（8 处）

| # | 位置 | 修复内容 | 对应 |
|---|------|---------|------|
| 1 | §9.3 api/home.ts | 改为独立端点 `GET /api/home/summary`；`quota_bytes` 不限时统一 `null`（删「省略」）；`/api/home/platforms` 纯列表仅承载平台卡片 | Q13/L10、Q5/L13 |
| 2 | §5.3.3 | 「⑥ 生成前选目标规则实体」改为「① 类型与目标步骤内选择」 | L2 |
| 3 | §5.3.4 标题 | 「步骤①或⑥前置」改为「固定于步骤①」 | L2 |
| 4 | 变更记录 v1.1 行 | 「对账区改三分区」勘误为「四分区」 | L3 |
| 5 | §6.2 协议选择 | 补：编辑允许变更协议=整体重新填表（凭据留空=保留原凭据） | Q2/M6① |
| 6 | §5.3.0 生成严格校验 | 补：勾选已停用预设组 400 拒绝生成，不纳入失效项剔除 | Q3/M6② |
| 7 | §9.3 api/subscription.ts | 补：用户会话预览 subs/generic-subs 装配模板返回明文原文 | Q1/M5 |
| 8 | §11 变更记录 | 追加 v2.1 行（本轮修订摘要） | — |

### 7.3 Build4.md（11 项，变更记录 v1.7）

1. Step2 item9 测试清单补 `emergency_test.go`/`platform_test.go`（fstest 夹具含旧表），措辞改「新增/更新单测」并明确 server/home_test.go、server/rule_test.go 为新建（M1/L20）。
2. Step4 补「已入池未生效，请激活」引导（行内 a-alert 标签 + 去激活链接）与 auto_activated=false 回执同口径提示（M2）。
3. 组详情/列表 `default_quota` 改 `omitempty`，advanced_mode=off 时省略（M3）。
4. Step5 差量删除改临时表 JOIN（方案 A）/分批（方案 B），禁止单条 NOT IN 大列表（M4）。
5. Step3 `ErrProductTypeInUse` 文案含既有 product_type 插值（L7）。
6. Step6 同步进行中 409 前端特判 `message.warning`（L8）。
7. Step4 路由守卫改 `system.status?.advanced_mode !== true`（L9）。
8. home_rule/traffic 移独立端点 `GET /api/home/summary`（Step3 后端 + Step4 `getHomeSummary()`），`/api/home/platforms` 恢复纯列表（Q13/L10）。
9. 零星落点：平台卡无版本占位文案+三按钮隐藏、新建订阅轻提示、删除默认规则 home_rule 回 null 断言、内容形态标签列注记归 Build5（L19）。
10. 空池两态拆分与池 URL http/https 校验补「DesignReport10 确认」注记（Q11/Q12）。
11. Step7 变更记录措辞修正 + 追加 v1.7 行（L20）。

### 7.4 Build5.md（12 项，变更记录 v1.6）

1. Step4 用户预览 subs/generic-subs 明文口径定稿（Q1/M5）。
2. Step1 协议变更=整体重新填表定稿（去「Build 期决策」标注）（Q2/M6①）。
3. Step3 停用预设组 400 拒绝生成定稿注记（Q3/M6②）。
4. 蓝图四列映射三处明示（models.go 列映射条目 + SaveBlueprintTx 落库说明 + getBlueprint 回填说明），含 selection_json 的 Xray 候选集（M7）。
5. Step3 伪代码 🌎国外流量去子组（L6）。
6. 约束表节点名称规则补「空格」禁止项（L5）。
7. Step5 enabled 1→0 停用 ConfirmModal 产出点+验收（G2/M15）。
8. Step7 版本列表空态「暂无版本」+ 引导验收（G1/M15）。
9. 验收 grep 改「将在下一轮构建实现」；Step1 go test 路径补 `/...`（L20）。
10. Step6 DiffView「整体新增」提示文案；Step7 订阅列表内容形态标签列消费 content_kind（L19）。
11. Step2 字符集共享表述澄清（共享基础函数、空格策略参数化）（L18）。
12. 变更记录追加 v1.6 行（初次误占 v1.4，已重编号）。

### 7.5 Build6.md（13 项，变更记录 v1.7）

1. Step1 `ValidateName` → `ValidateNodeName`（M8）。
2. Step4 默认下载文件名按 target_syntax 映射（.yaml/.txt/.txt/.conf，直传保留原扩展名）+ 单测 + 验收（M9）。
3. Step1 REALITY 私钥不落库、仅推导写入 `public_key`（Q4/M10）。
4. Step3 missing 0→1 / 1→0 拆分两条独立子弹（M11）。
5. Step5 profile/home traffic 不限流量时 `quota_bytes=null`（Q5/L13）。
6. Step5 traffic 经 Build4 已落地的 `/api/home/summary` 补高级模式数值，platforms 纯列表（Q13/L10；并行修订冲突口径已二次校准）。
7. Step4 Clash 重渲染无凭据/OFF 场景 proxies 区首行注释行（Q6/L14）。
8. Step3 超限跳过保持 xray_users 记录并写 last_error（Q7/L15）。
9. Step1 xray 节点 host 取 api_addr host 定稿（实例派生字段，非 inbound 解析字段）（Q8/L16）。
10. 四处编辑性修正：kind 预留措辞、Step5 List 仅补有效配额、`grpc.WithTimeout` 改 context、`profile-web-page-url` 取站点 URL（L18）。
11. 范围红线注记 ext 采集与配额检查归 Build7 Step1（Q9/L17）。
12. Step2 SetNodes 显式 `BEGIN IMMEDIATE` 事务（先删后插+受影响用户清单收集，提交后回调）（G3/M14）。
13. 变更记录追加 v1.7 行（初次指示 v1.1 与既有行冲突，按顺位 v1.7）。

### 7.6 Build7.md（12 项，变更记录 v1.8）

1. Step1 实例/节点删除钩子去重：改为「Build6 已实现，本 Step 仅补 ext 维度单测」（M12）。
2. generate 模式同 email 已存在 → 先 RemoveUser 再 AddUser 覆盖（确认文案提示踢除风险）；manual 模式保持原接管口径（Q10/M13）。
3. 参数表推送范围补「节点 enabled=1」（Q14/L4）。
4. 120s 清单剔除异步提交端点（runInit/reconcile 三执行/deleteInstance）（L18）。
5. 参数表期望集措辞修正（xray_users 仅承载状态/重试）（L18）。
6. Step3 实例/独立账号空态显式列（policy.stats 前置提示、用途说明）（L19）。
7. Step3 创建弹窗补 generate 模式覆盖接管提示文案（Q10）。
8. Step4 补 `importConfig`（v2 适配）与 `getAdminTask` 函数点名（L19）。
9. Step5 home 数据源改 `getHomeSummary()`（沿用 Build4 已落地函数，平台卡片字段仍来自 platforms 端点）（Q13）。
10. Step5 traffic 形状「省略 quota_bytes」改 `quota_bytes=null`（回归阶段二次校准）（Q5）。
11. 单测补 generate 覆盖与 ext 维度断言（Q10/M12 联动）。
12. 变更记录追加 v1.8 行（v1.1 已被占用，按顺位 v1.8）。

### 7.7 回归核对结论

- 六份文档交叉 grep 复核：`/api/home/summary` 端点在 Design2 §5.10、UI §9.3、Build4 Step3/4、Build6 Step5、Build7 Step5 五处口径一致（Build4 落地端点、Build6 补高级数值、Build7 前端消费）；`quota_bytes=null` 四处一致，无「省略 quota_bytes」残留；`getHomeSummary` 无重复创建。
- 新增设计结论（Q1~Q14）在设计与 Build 文档间双向一致；各 Build 变更记录版本号无重复（Build4 v1.7 / Build5 v1.6 / Build6 v1.7 / Build7 v1.8 / UI v2.1）。
- 并行修订产生的 3 处跨文档措辞冲突（Build6「原 Build4 规划」、Build7「省略 quota_bytes」、Build7「新增 getHomeSummary」）已在回归阶段修正。
- **六份文档达到「构建前无疑点」状态，可按 Build4 Step 0 启动构建。**
