# Build7.md — 高级模式管理面与交付收口：对账/独立账号/导入导出 v2/OFF 清空/高级 UI（当前构建方案·第七轮）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第七轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则），承接 [Build4.md](./Build4.md)、[Build5.md](./Build5.md)、[Build6.md](./Build6.md)（前三轮须全部验收通过后本轮方可启动）。
> - 设计基线：[Design2.md](./Design2.md)（与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design2-UI.md](./Design2-UI.md)（本 Build 重点落地其 §3/§4/§8 高级模式 UI 与 §9 契约）
> - 编码指令：[AGENTS.md](./AGENTS.md)（**唯一强要求**）
> - 前置轮次：[Build4.md](./Build4.md)、[Build5.md](./Build5.md)、[Build6.md](./Build6.md)
>
> **里程碑：本 Build 全部 Step 完成后，Design2 全量能力交付：Xray 实例页（检测/初始化/对账）与独立账号 Tab 完整可用；用户组高级管理（节点分配/排序/默认配额/候选集引导）、用户管理高级列（用量/同步状态/配额覆盖/重置）、面板设置高级分区（开关/采集间隔/流量卡片/OFF 清空）可用；配置导入导出升级 format_version=2；首页流量卡片与个人中心本月流量按两模式正确展示；高级 OFF 清空与重新开启闭环。**
>
> **范围红线：** 本 Build 不得回改 Design2/Design2-UI 未涉及页面；不得把高级能力下沉到基础模式；不得在 OFF 清空之外删除 proxy_groups、groups 行与用户组归属、assembly_blueprints。

---

## 执行约束（执行 AI 必须严格遵守）

1. **严格按 Step 顺序执行**；每个 Step 验收通过后进入下一个；**禁止跳步、并行、合并、跨 Build 补做**。
2. **每个 Step 完成后必须运行该 Step 的「验证命令」**；任一失败修复后重验。
3. **遇到模糊、歧义或设计未覆盖的细节，必须停止并使用提问工具向用户询问**；禁止自行假设。
4. **依赖白名单**：本 Build 不新增依赖；前端沿用 Build5 引入的 jsdiff。
5. **关键设计参数必须严格按下表取值**，与 Design2.md 保持一致，禁止修改：

| 参数 | 取值 | 出处 |
|------|------|------|
| 独立账号 email | `ext-{id}@vpn.local`，全小写；与 `user-` 前缀体系区分 | Design2 §5.11 |
| 独立账号推送范围 | 手动勾选实例/inbound；仅四协议、allocatable=1、missing=0、节点 enabled=1、实例 enabled=1；**不参与组分配/公共节点/候选集** | Design2 §5.11 |
| 对账期望集 | 用户部分 = 全部 active 用户 ×（组分配 ∪ 公共），经候选集与可用性过滤；独立账号部分 = 其 xray_ext_users 推送目标，**仅可用性过滤、不经候选集**（独立账号不参与候选集口径）；用户部分期望集由 active×分配/公共实时计算（xray_users 仅承载状态/重试、非期望集来源），xray_ext_users 为独立账号推送目标载体 | Design2 §5.10/§5.11 |
| 对账分区 | 待补推 / 无头用户（user- 前缀）/ 疑似独立账号残留（ext- 或无法匹配前缀，默认不勾选）/ 凭据不一致 | Design2 §5.10/§5.11 |
| 导出格式 | `format_version=2`；instances 全字段导出（slug 导入沿用）并附带节点命名映射 `nodes: [{tag, display_name}]`（仅非空显示名）；accounts 含 **quota_exceeded** 与 push_targets（instance slug + inbound tag）；**带实例/账号导入且 advanced_mode=false 时自动置为 true**；**无实例/账号且 advanced_mode=false 导入时要求 DISABLE**；v1 导入兼容仅配置、同步执行 | Design2 §5.4 |
| 导入保护 | 若 signing_key 将变化且存在业务密文（users.uuid_encrypted / users.proxy_secret_encrypted / nodes.protocol_json 凭据字段 / xray_ext_accounts 两个密文字段），拒绝导入并提示「配置导入仅适用全新部署/同密钥往返，在用实例请使用备份恢复」 | Design2 §5.4 |
| OFF 清空 | 开关关闭一并移除：xray_instances、source=xray 节点、group_nodes、xray_users、独立账号（凭据+推送记录）、traffic_records、用户 UUID/代理密码/配额字段；保留 proxy_groups/groups 行/用户组归属/assembly_blueprints（悬空引用容错） | Design2 §一 |
| OFF 清理口径 | **确认词 `DISABLE`**；**提交返回 task_id 异步执行**；事务内置 advanced_mode=off；事务提交前收集 user/ext 推送记录与连接信息；提交后逐实例 best-effort RemoveUser；不可达跳过记 warn | Design2 §一 |
| 重新开启 | 仅置位 advanced_mode=on，**不执行任何推送**；管理员重新录实例/检测/组分配后手动「开始初始化」 | Design2 §一 |
| 流量卡片开关 | 配置键 `traffic_card_enabled` 默认 true；off 时首页流量卡与个人中心本月流量行隐藏 | Design2 §5.10/UI §3.1/§4.7 |
| 采集间隔 | 配置键 `xray_collect_interval_minutes` 默认 10，≥1 | Design2 §5.8 |
| 超限文案 | 用户端「本月流量已超限，代理账号已暂停，请联系管理员重置」；管理端「已超限」标签 | UI §10.1 |

6. **注释使用中文**；所有 error 必须处理；构造注入；业务层不感知 HTTP；日志 token 脱敏；危险操作一律 ConfirmModal/确认词。
7. **对账清理的 ext 残留默认不勾选**，防止手动维护的独立账号被误清理；一键清理仅删除用户勾选项。

---

## TODOLIST CheckList（构建进度核对）

> 执行 AI 必须逐个完成并勾选。状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

- ☐ Step 1：独立 Xray 账号后端 + 实例级账号对账后端
- ☐ Step 2：高级设置后端、OFF 清空与配置导入导出 format_version=2
- ☐ Step 3：Xray 实例页前端（实例/检测/初始化/对账/独立账号）
- ☐ Step 4：用户组/用户管理/面板设置高级前端
- ☐ Step 5：首页/个人中心/AppHeader 高级展示收口
- ☐ Step 6：全量端到端验收、文档核对与归档

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 独立账号 + 实例级对账后端 | Design2 §5.10/§5.11 | ☐ 未开始 |
| 2 | 高级设置后端 + OFF 清空 + 导入导出 v2 | Design2 §一/§5.4/§5.10 | ☐ 未开始 |
| 3 | Xray 实例页前端 | Design2-UI §8 | ☐ 未开始 |
| 4 | 组/用户/设置高级前端 | Design2-UI §4.3/§4.5/§4.7 | ☐ 未开始 |
| 5 | 首页/个人中心/AppHeader 收口 | Design2-UI §2.2/§3 | ☐ 未开始 |
| 6 | 全量验收与归档 | Design2 全量 + AGENTS §8.2 | ☐ 未开始 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件（核心） | 要点 |
|------|----------------|------|
| 1 | `backend/internal/xray/{ext,reconcile}.go`、`backend/internal/server/xray.go`、`backend/internal/cron/`、测试 | ext 账号 CRUD/双轨凭据/推送目标/配额采集；对账四分区与修复执行 |
| 2 | `backend/internal/config/{export,admin}.go`、`backend/internal/server/settings.go`、`backend/internal/server/xray.go`、`backend/internal/xray/offclear.go`、测试 | format_version=2、导入保护、OFF 清空事务、高级设置三键 |
| 3 | `frontend/src/api/xray.ts`、`frontend/src/views/admin/XrayInstancesView.vue` | UI §8 双页签全部交互 |
| 4 | `frontend/src/api/{group,user,settings}.ts`、`frontend/src/views/admin/{GroupsView,UsersView,SettingsView}.vue` | 高级列/配额/节点分配/高级分区 |
| 5 | `frontend/src/api/home.ts`、`frontend/src/views/{HomeView,ProfileView}.vue`、`frontend/src/components/AppHeader.vue` | 流量卡/默认规则卡/超限提示/组名标签 |
| 6 | 全部文件、`docs/AchievedDocuments/` 归档动作 | 全量验收、Design 覆盖矩阵、归档 |

---

## 三、构建顺序依赖图

```
Step 1（后端）──▶ Step 2（后端）──▶ Step 3（Xray UI）──▶ Step 4（高级管理 UI）
Step 4 ──▶ Step 5（用户端收口）──▶ Step 6（验收/归档）
```

> 线性执行序：Step 1 → 2 → 3 → 4 → 5 → 6。

---

## 四、分步构建计划


---

### Step 1：独立 Xray 账号后端 + 实例级账号对账后端

**本 Step 完成后，`/api/admin/xray/ext` CRUD、凭据查询/复制、配额重置与 ext 流量采集可用；`/api/admin/xray/instances/:id/reconcile` 及 push/clean/credentials 执行端点可用，四分区语义与 ext 防护落地。**

> **承接 Build6-2 #5（独立账号采集与配额检查）**：Build6 已接入面板用户采集任务，但独立账号（ext）采集与配额检查未在 Build6 实现，原规划归属本 Step。本 Step 必须补齐：`CollectExtTraffic` 写入 `xray_ext_traffic(ext_account_id, ym)`、ext 超限摘除、重置恢复，并在 `cron.StartXrayCollect` 的用户采集之后追加 ext 采集与超限检查；相关口径沿用 Build6 Step5，仅数据表与账号维度切换为 ext。

- **目标：** 实现 Design2 §5.11 独立账号与 §5.10 实例级对账，并闭环 Build6-2 #5。
- **前置条件：** Build6 全部验收通过（含 Build6-2 补强）。
- **产出文件与操作：**

  1. **`backend/internal/xray/ext.go`**：独立账号服务。
     - `ExtAccount {ID, Name, Email, Quota *float64, QuotaExceeded, PushTargets}`；`CreateExt(ctx, name, credentialMode, uuid, proxySecret, quota, targets)`：
       - **基础校验**：name 唯一（409）；quota 为 NULL/0/正数，负数 400。
       - **推送目标后端强校验**（Design2 §5.11）：每个 target 必须满足 source=xray、protocol ∈ {vless,vmess,trojan,shadowsocks}、allocatable=1、missing=0、nodes.enabled=1、所属实例 enabled=1；非法目标 400 并指明具体节点。**请求形状为 `push_targets: [{instance_id, inbound_tag}]`**（Design2Report10 Q12-6）。
       - 事务插入账号（email 先写**唯一临时占位值**——例如 `ext-pending-{随机串}@vpn.local`，不得使用固定占位串；取 LastInsertId 后回填为 `ext-{id}@vpn.local`；全小写）。**并发创建时固定占位串会命中 `xray_ext_accounts.email` UNIQUE 约束，必须唯一化**。
       - 凭据双轨：`generate` 面板生成 UUID v4 + 高熵密码；`manual` 用传入值（非空校验）。两者均 AES-256-GCM 落库。
       - 写 `xray_ext_users`（pending，**node_id 按 (instance_id, inbound_tag) 从 nodes 表解析写入**）与 `xray_ext_traffic` 不预建（按需）；事务内复查 advanced_mode，off 中止。
       - 提交后对 targets 逐个 AddUser；成功 synced、失败 failed。**凭据接管按模式区分（Design2Report11 决策）**：`generate` 模式若 Xray 侧已存在同 email 账号（AddUser 返回 `already exists.`），**先 RemoveUser 再以新生成凭据 AddUser 覆盖**（会踢除 Xray 侧既有账号，创建弹窗/确认文案须提示该风险）；`manual`（手填接管）模式保持原口径——`already exists.` 视为接管成功（管理员保证手填凭据一致，面板不校验 Xray 侧原值）。
       - **返回形状**：`generate` 模式响应含 `{account, credentials:{uuid, proxy_secret}}`（一次性明文，前端展示后即焚）；`manual` 模式仅返回 `{account}`（Design2Report8 P2-14）。
     - `UpdateExt`：名称可改（唯一 409）；凭据字段留空保留；push_targets 全量 diff（Remove/Add，超限账号 AddUser 类跳过；**目标校验同 CreateExt**）；配额更新（NULL/0/正数，负数 400）。
     - `RetryExt(ctx, extAccountID)`：对 `xray_ext_users.sync_status='failed'` 记录逐个重试——期望集仍含该目标则 AddUser（超限账号跳过），否则 RemoveUser；返回计数回执（Design2Report8 Q4）。
     - `GetExtCredentials`：解密返回 `{uuid, proxy_secret}`（敏感端点，日志不得输出值；**响应必须携带 no-store 禁缓存头**，Design2Report10 Q12-1）。
     - `DeleteExt`：事务前收集 targets；删除记录；提交后 RemoveUser；不可达跳过记 warn。
     - `CollectExtTraffic(ctx, instance)` 与 `ResetExtQuota`：沿用 Build6 Step5 用户口径，但写入 `xray_ext_traffic(ext_account_id, ym)`；超限判定按 ext 自己的 quota；超限 → 全部已推 inbound RemoveUser + quota_exceeded=1；重置 → 清当月 + 重新 AddUser（凭据不变）。
     - **实例/节点删除钩子（去重，仅补单测）**：Build6 Step1/Step3 已实现实例删除与 missing 节点删除时面板用户 + 既有 `xray_ext_users` 两类目标的收集与 RemoveUser（含单测）；本 Step **不重复实现**，仅在 ext 服务落地后补充独立账号维度（ext 推送目标收集与调用序）的单测断言。
  2. **`backend/internal/xray/reconcile.go`**：
     - `Reconcile(ctx, instanceID)`：
       1. 期望集 =【全部 active 用户 ×（组分配 ∪ 公共节点），经**候选集与可用性过滤**】∪【独立账号 × 其 xray_ext_users 推送目标，**仅可用性过滤、不经候选集**（独立账号不参与候选集口径，Design2 §5.11）】；**再与本实例节点（nodes.instance_id=instanceID）取交集，to_push / credential_mismatches 仅含本实例目标，防止误推其他实例**（Design2Report7 P2-4 / Design2Report9 M3）。
       2. 实际集 = 对实例全部 inbound 调 `GetInboundUsers`（可逐个 tag，email 空传空）。
       3. 四分区：
          - `to_push`：期望有/实例无（user/ext 来源标记）；
          - `orphans`：实例有/期望无且 email 前缀 `user-`；
          - `ext_orphans`：实例有/期望无且 `ext-` 前缀或无法匹配前缀（默认不勾选由前端负责，后端数据原样返回）；
          - `credential_mismatches`：期望集命中 email，但 Account 与面板 UUID/代理密码/ss cipher 不一致。
       4. 返回形状按 UI §9.1 `reconcile` 行：`{to_push, orphans, ext_orphans, credential_mismatches}`。
     - **三个执行端点（RepairPush / CleanOrphans / RepairCredentials）均为异步长任务**：提交即返回 `{task_id}`（**复用 Build6 Step1 落地的全局任务 registry**，kind=reconcile_exec，Design2 §5.4），任务体串行执行，终态写入计数回执与跳过项（超限等）；补推时超限面板用户与超限独立账号跳过并记入结果。
     - **单条同步端点（Design2Report10 Q9）**：`POST /api/admin/xray/instances/:id/reconcile/push-one`（单条补推）与 `POST /api/admin/xray/instances/:id/reconcile/credentials-one`（单条凭据修复，先 RemoveUser 再 AddUser）；请求传单项目标，同步执行 + 统一 120s，返回计数/统一成功结构。
     - `CleanOrphans(ctx, instanceID, emails []string)`：只允许清理请求中显式勾选的 email；逐个 RemoveUser；**后端不自动全清**。
     - `RepairCredentials(ctx, instanceID, targets)`：对勾选项先 RemoveUser 再 AddUser（幂等）。
  3. **`backend/internal/server/xray.go`**：新增路由（均 advancedMode）：
     - `GET/POST/PUT/DELETE /api/admin/xray/ext(/:id)`；
     - `GET /api/admin/xray/ext/:id/credentials`；
     - `POST /api/admin/xray/ext/:id/retry`；
     - `POST /api/admin/xray/ext/:id/reset-quota`；
     - `GET /api/admin/xray/instances/:id/reconcile`；
     - `POST /api/admin/xray/instances/:id/reconcile/push|clean|credentials`；
     - `POST /api/admin/xray/instances/:id/reconcile/push-one|credentials-one`（单条同步，Design2Report10 Q9）。
  4. **`backend/internal/cron/`**：采集任务在用户采集后追加逐独立账号采集与超限检查（同一任务、同样 advanced_mode 入口检查；**承接 Build6-2 #5，必须在本 Step 闭环**）。
  5. **单测（fake API）**：ext 创建 generate/manual、email 前缀、**name 唯一与 quota 负数拒绝**、**创建响应含一次性凭据（generate）且库内为密文**、**generate 模式同 email 已存在 → 先 RemoveUser 再 AddUser 以新凭据覆盖（manual 模式保持 already exists 视为成功）**、**retry 对 failed 记录 AddUser/RemoveUser 计数**、**push_targets 非法目标（非 xray/非四协议/allocatable=0/missing=1/节点或实例停用）400**、编辑留空保留、target diff、超限摘除与重置；对账四分区（含 `ext-` 残留与不匹配前缀）、**期望集实例交集断言（to_push / credential_mismatches 不含其他实例节点目标，P2-4）**、clean 仅清理勾选项、credential mismatch 修复顺序（先 Remove 后 Add）、**push-one/credentials-one 单条同步端点**；**实例/节点删除收集 user+ext 两类目标并 RemoveUser（user+ext 收集与调用序断言沿用 Build6 Step1/Step3 单测，本 Step 仅补 ext 服务落地后的独立账号维度断言）**。

- **参考代码/伪代码：**

  **对账核心**

  ```go
  desired := map[email]desiredAccount{} // 由期望集计算
  actual  := map[email]actualAccount{}  // 由 GetInboundUsers 归一
  for email, exp := range desired {
      act, ok := actual[email]
      if !ok { toPush = append(...); continue }
      if !accountMatches(exp, act) { mismatch = append(...) }
  }
  for email := range actual {
      if _, ok := desired[email]; !ok {
          if strings.HasPrefix(email, "user-") { orphans = append(...) }
          else { extOrphans = append(...) } // ext- 或无法匹配前缀，前端默认不勾选
      }
  }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/xray/... ./internal/cron/... ./...
  ```

- **验收标准：** 全部测试通过；对账四分区与 ext 防护有 fake 测试；GET credentials 响应不落日志明文；错误文案符合 UI §9.1/§9.4。

---



---

### Step 2：高级设置后端、OFF 清空与配置导入导出 format_version=2

**本 Step 完成后，`/api/admin/settings/advanced` 读写三键（advanced_mode / 采集间隔 / 流量卡片开关）；关闭高级模式按确认词执行 OFF 清空事务与 best-effort Xray 清理；导入导出升级 v2 并带 signing_key 保护。**

- **目标：** 落地 Design2 §一/§5.4/§5.10 的运维语义。
- **前置条件：** Step 1 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/config/admin.go`**：新增 `AdvancedSettings {AdvancedMode bool; CollectIntervalMinutes int; TrafficCardEnabled bool}` 与 Get/Save 方法；保存 `advanced_mode` 时必须走下面 OFF/ON 分支，禁止普通 Set 绕过；**`CollectIntervalMinutes` ↔ 配置键 `xray_collect_interval_minutes` 在 Get/Save 内显式映射并补读写单测（自 Build6 承接，Design2Report9 M9）**。
  2. **`backend/internal/xray/offclear.go`**：`SubmitAdvancedMode(ctx, on bool, confirmWord string) (taskID string, err error)`。
     - `on=true`：同步只置位 `advanced_mode=true`，**不推送**；返回提示文案（无 task_id）。
     - **幂等 no-op：当前已 OFF 时再次以 advanced_mode=false 保存，直接返回当前状态，不要求确认词、不建任务**（Design2Report9 M15）。
     - `on=false`（**仅 ON→OFF 状态翻转**）：确认词固定 `DISABLE`（与 RESET/IMPORT 区分，前端同常量）；**状态翻转判定与任务登记在同一 `BEGIN IMMEDIATE` 事务内完成（先读当前 advanced_mode，仅 ON→OFF 翻转才落位并登记任务；并发第二次提交读到已 OFF/已有任务时按幂等 no-op 返回，不建第二个任务）**（Design2Report10 Q15）。任务登记使用 Build6 落地的 **`internal/tasks.Registry`**（由 server 构造注入，业务包不 import server）；**用户决策 2026-08-20：按字面同事务执行，若 DB 事务回滚而内存任务已登记，接受残留幽灵任务边界（该任务重启后统一 failed 兜底）**；**创建异步任务并返回 `task_id`**，任务体内事务执行：
       1. 置 `advanced_mode=false`（与清空同事务）；
       2. 收集 `xray_users`（user_id/instance_id/inbound_tag）与 `xray_ext_users` 及实例连接信息（api_addr/api_tag）快照；
       3. `DELETE FROM xray_instances`（nodes/group_nodes/xray_users/xray_ext_users 随 FK 级联）；`DELETE FROM xray_ext_accounts`；`DELETE FROM traffic_records`、`xray_ext_traffic`；`UPDATE users SET uuid_encrypted=NULL, proxy_secret_encrypted=NULL, quota_override=NULL, quota_exceeded=0`；`UPDATE groups SET default_quota=NULL`；
       4. **保留** proxy_groups、groups 行、用户组归属、assembly_blueprints、**manual 节点**（manual 行 instance_id 为 NULL，不随 xray_instances 级联）；装配快照中 xray 引用成为悬空引用，按重新编辑容错处理。
     - 事务提交后按快照逐实例 best-effort `RemoveUser`（user 与 ext 都清；**复用 Build6 Step3 落地的 `AfterAdvancedOff` 补偿辅助**，避免重复实现）；不可达跳过并记 warn；任务状态写入注入的 **`internal/tasks.Registry`（Build6 Step1 已落地）**，服务重启后未完成任务置 failed（「服务重启，任务中断」），残留 Xray 账号由对账/部署文档手动清理口径兜底。
     - 并发送口径：所有同步钩子入口实时查 DB（Build6 已实现）；AddUser 完成后补偿（Build6 已实现）；本方法事务内置位后提交，钩子任一复查读到即最新标记。
  3. **`backend/internal/server/settings.go`**：新增 `GET/PUT /api/admin/settings/advanced`（**只叠加 session+admin，不得套 advancedMode 中间件——否则 OFF 状态下无法关闭/重新开启高级模式，且 OFF 清空任务轮询会被自身挡死**）；**全局任务查询端点 `GET /api/admin/tasks/:id` 已在 Build6 Step1 落地，本 Step 只复用、不重复定义**（Design2Report10 Q1）；关闭请求含 `{advanced_mode:false, confirm_word}`，响应 `{task_id}`；任务查询返回 `{id, kind: off_clear|import|xray_init|reconcile_exec|instance_delete, status: running/succeeded/failed, result, error}`；开启与幂等 no-op 响应不返回 task_id。**任务状态由全局长任务 registry 进程内维护（不落库），服务重启后任何查询（含未知 task id）一律返回 failed（「服务重启，任务中断」），未完成 Xray 清理由实例对账与手动清理兜底（Design2 §5.4，Design2Report9 M6/M7）**。
  4. **`backend/internal/config/export.go`**：升级 v2。
     - `ExportPayload` 增 `Instances []ExportedInstance`（name/slug/api_addr/api_tag/enabled + **`Nodes []ExportedNodeName`（tag/display_name，仅导出 display_name 非空项）**）与 `Accounts []ExportedExtAccount`（name/email/uuid_encrypted/proxy_secret_encrypted/quota/**quota_exceeded**/push_targets `[{instance_slug,inbound_tag}]`）；`FormatVersion=2`。
     - 导出时读 xray_instances（含其 xray 节点 display_name 非空项）与 xray_ext_accounts/xray_ext_users（按 slug+tag 关联）；密文原样导出，不导出明文；display_name 为明文展示属性，原样导出。
     - `Import`：接受 v1（仅配置，保持旧语义、同步执行；**v1 兼容为本 Build 保留口径：Design2 §5.4 仅定义 v2 语义，v1 为现行 prod 实现，导入导出须保证 v1 往返可用**）与 v2；**v2 改为异步任务并返回 `task_id`**，任务体在配置覆盖事务内：先收集旧实例连接信息与旧 user/ext 推送清单 → 删除旧 `xray_instances` 与 `xray_ext_accounts`（FK 级联）→ 按 payload 重建 instances（slug 原样沿用；**冲突/重复为任务内校验失败：任务终态置 failed，`error` 按 400 口径文案返回给轮询方，HTTP 提交阶段无法同步返回 400**）与 ext accounts（name/email/密文/quota/quota_exceeded 原样写入，email 全小写）→ 按 push_targets 写 `xray_ext_users`（node_id 暂置 NULL，sync_status=pending；**禁止插入 0——启用外键时违反 FK，Design2Report7 P2-5**）。**advanced_mode 一致性**：payload 携带非空 instances/accounts 且 advanced_mode=false 时，事务内置 advanced_mode=true 并在完成提示中说明「检测到 Xray 实例/独立账号，已自动开启高级模式」；payload 无 instances/accounts 且 advanced_mode=false 时，按 OFF 清空口径清理旧高级数据（用户 UUID/代理密码/配额字段、traffic_records、xray_ext_traffic、旧推送记录）——**该分支采用双确认词分步：先按既有导入流程校验 `IMPORT`，再校验 `DISABLE`（前端分步弹窗要求输入，Design2Report10 Q5）**。**导入保护**：比较 payload 中 signing_key 与当前库 signing_key；若将变化且存在任一业务密文（users.uuid_encrypted / users.proxy_secret_encrypted / nodes.protocol_json 中 `enc:v1:` 敏感字段 / xray_ext_accounts 两密文字段），整个导入拒绝，不做任何变更。**响应口径：v1 导入保持同步返回 `{message}`；v2 导入异步返回 `{task_id}` 供前端轮询**（Design2Report10 Q5）。
     - **依赖方向**：`config/export.go` 不 import xray/assembly/server；`internal/tasks.Registry` 与「导入后处理回调」均由 server 构造注入（与现有 `SetSeedPresets` 同模式；v2 导入任务经注入 Registry 登记）。事务提交后执行：对旧实例 best-effort RemoveUser → 自动节点检测刷新（**enabled=0 实例跳过并在完成提示中列出**）→ **按 (instance slug, inbound tag) 回填 nodes.display_name（未匹配映射计入完成提示，不阻断）** → 按 instance slug+tag 重绑 xray_ext_users.node_id（未匹配置 failed）→ **装配快照（selection_json / render_plan_json）xray 引用按节点稳定名重绑（同名重绑、失配悬空容错，Design2 §5.9 xray_instances 行口径）** → **执行账号对账（Build7 Step1，best-effort：失败不阻断任务终态 succeeded，失败原因记入完成提示，管理员可在实例页手动再触发对账，用户决策）**。返回完成提示字段。
  5. **单测**：OFF 清空保留/删除清单逐表断言、确认词错误（DISABLE 之外 400）、**异步任务提交→轮询终态**、**已 OFF 再保存幂等 no-op（不建任务、不要求确认词）**、**并发两次 OFF 提交只产生一个任务**、未知 task id 返回 failed、不可达清理记 warn；重新开启不推送；导入保护命中拒绝且库不变；v2 往返（导出→清库→导入）恢复 instances（含 display_name 命名映射）/accounts（含 quota_exceeded）与 push_targets、未匹配命名映射计入提示；**off 导入带实例→自动置 advanced_mode=true 并提示**；**off 导入无实例→先 IMPORT 后 DISABLE 双确认词均通过才执行，缺任一 400，且旧高级数据完整清理**；**enabled=0 实例导入后跳过检测并提示**；v1 兼容同步执行。

- **参考代码/伪代码：**

  **OFF 清空事务骨架**

  ```go
  return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
      if _, err := tx.ExecContext(ctx, `UPDATE system_config SET value='false' ... WHERE key='advanced_mode'`); err != nil { return err }
      snapshot = collectXrayTargetsTx(ctx, tx) // xray_users + xray_ext_users + instance conns
      for _, stmt := range []string{
          `DELETE FROM xray_ext_users`, `DELETE FROM xray_ext_traffic`, `DELETE FROM xray_ext_accounts`,
          `DELETE FROM xray_users`, `DELETE FROM traffic_records`, `DELETE FROM xray_instances`,
          `UPDATE users SET uuid_encrypted=NULL, proxy_secret_encrypted=NULL, quota_override=NULL, quota_exceeded=0`,
          `UPDATE groups SET default_quota=NULL`,
      } { if _, err := tx.ExecContext(ctx, stmt); err != nil { return err } }
      return nil
  })
  // 提交后：for snapshot targets -> best-effort RemoveUser；实例不可达 skip+warn
  ```

  **导入保护判定**

  ```go
  if payload.Config["signing_key"] != currentSigningKey && hasBusinessCiphertext(ctx) {
      return errors.New("配置导入仅适用全新部署/同密钥往返，在用实例请使用备份恢复")
  }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/xray/... ./internal/config/... ./internal/server/... ./...
  ```

- **验收标准：** 全部测试通过；OFF 清空后 `advanced_mode=false`、xray 表清零、用户凭据与配额字段清空、proxy_groups/groups/blueprints 保留；v2 往返与保护测试通过。

---



---

### Step 3：Xray 实例页前端（实例/检测/初始化/对账/独立账号）

**本 Step 完成后，`/admin/xray` 双页签页面按 Design2-UI §8 完整可用。**

- **目标：** 落地高级模式最大管理页。
- **前置条件：** Step 2 验收通过。
- **产出文件与操作：**

  1. **`frontend/src/api/xray.ts`**：按 UI §9.1 `api/xray.ts` 表逐函数实现（listInstances/createInstance/updateInstance/deleteInstance/testConnection/detectNodes/runInit/reconcile/pushRepair/cleanOrphans/repairCredentials/**pushOne/repairCredentialsOne**/retryUserSync/resetQuota/getUserSync/getInstanceStats/listExtAccounts/createExtAccount/updateExtAccount/deleteExtAccount/**retryExtSync**/getExtCredentials/resetExtQuota）；**detectNodes 类型含 `added_nodes: [{node_id, tag, name}]`**；**createExtAccount generate 响应含一次性凭据，展示后即焚**；**同步长操作统一 `timeout: 120_000`**（Design2Report10 Q12-2）：对账查询（reconcile GET）、节点检测（detectNodes）、createExtAccount/updateExtAccount/deleteExtAccount、retry*/resetQuota、pushOne/repairCredentialsOne 及 testConnection（装配生成/预览 120s 同口径见 Build5）；**runInit、reconcile 三执行端点（push/clean/credentials）与 deleteInstance 为异步提交端点，提交即返回 task_id、走 pollTask 轮询，不适用 120s**。
     - **`frontend/src/api/request.ts` 同步适配 UI §9.4**：403 且后端 message 为「高级功能未开启」时，`message.warning('高级功能未开启')` 并刷新 system status（区别于普通「权限不足」）；409 冲突直接展示后端描述（实例名/独立账号名/节点显示名等）。
  2. **`frontend/src/views/admin/XrayInstancesView.vue`**：页面骨架 `a-tabs` 双页签（Xray 实例 / 独立账号）；`PageHeader` 右侧常驻「开始初始化」按钮（仅作用于面板用户）。
  3. **实例 Tab**（按 UI §8.1~§8.4）：
     - 双态列表：名称/slug/api_addr/enabled 行内开关（Tooltip 停用语义）/采集状态 Badge + 连续失败告警/操作（编辑/刷新节点/对账/删除）；**列表空态 `a-empty`「还没有 Xray 实例」+「新增实例」按钮 + 前置提示「需先在 Xray 服务器开启 gRPC API 与流量统计（policy.stats）」（UI §10.2）**。
     - 新增/编辑弹窗（480px）：名称 + api_addr + api_tag + 「测试连接」（loading → 成功/失败 alert；不落库）；**编辑保存成功后提示「已保存，建议执行『刷新节点』以同步 api_addr 变化后的节点信息」**（Design2Report10 Q12-7）。
     - 「刷新节点」→ detectNodes → 结果回执 Modal：新增 N / 更新 M / missing K / 撞名跳过 J（列出 tag+reason）；**四项全 0 时以 `Notify.info`「节点无变化」提示，不弹回执**（UI §8.2）；**新增节点命名区**：`added_nodes` 逐行 `tag + 系统名 + display_name 输入框`，留空=暂不命名，「保存显示名」逐行调用 `setNodeDisplayName`（**复用 Build5 已实现的 `api/node.ts` 同名函数**，409 字段级提示）；完成后刷新节点管理页同源数据；**实例不可达/gRPC 失败时 `a-alert error` 展示错误摘要与「检查 api_addr/实例状态后重试」引导，不弹回执**（Design2Report10 Q12-4）。
     - **删除实例**：ConfirmModal（影响清单：xray 来源节点级联删除、组分配级联清理、推送记录清理；附 `a-alert warning`「实例不可达时 Xray 侧残留账号需手动清理」，UI §8.1）确认后调用 `deleteInstance`，响应 `task_id` 按 pollTask 轮询全局任务端点（见 9.2），按钮 loading 防重复，终态后刷新列表（Design2Report10 Q12-3）。
     - 「开始初始化」→ ConfirmModal 三点说明 → 执行 runInit → `Notify.success` 计数；failed>0 提示用户管理页重试。
     - 「对账」→ `a-drawer` 对账面板四分区：
       ① 待补推（user/ext 来源 Tag）行内补推 + 一键补推（超限提示）；
       ② 无头用户（user- 前缀）清理勾选；
       ③ 疑似独立账号残留（黄色警示，清理勾选默认不勾选）；
       ④ 凭据不一致（面板账号 + 节点 + 移除并重推行操作）；
       一键清理危险 ConfirmModal；执行结果计数回执；四区全空 success 空态。**行内补推调用 `pushOne`、行内凭据修复调用 `repairCredentialsOne`（同步 120s）**（Design2Report10 Q9）；**对账 GET 失败/实例不可达时以 `a-alert error` 展示错误摘要与「检查实例状态后重试」引导，不渲染四分区**（Design2Report10 Q12-4）。
  4. **独立账号 Tab**（按 UI §8.5）：
     - 双态列表：名称/email/配额/本月用量/推送摘要聚合 Badge/超限标记/操作（复制凭据/编辑/重置配额/删除）；**列表空态 `a-empty`「还没有独立账号」+「创建独立账号」按钮 + 用途说明「用于向面板账号体系之外的人员/场景分发凭据（可手写入自定义订阅内容）」（UI §10.2）**。
     - 创建/编辑弹窗（720px）四区：基本信息；凭据区双轨（自动生成/手填接管，`a-input-password`，编辑留空=保留）；推送目标按实例分组 inbound 多选（过滤规则；**无可用节点时空态提示「请先在实例页检测节点」**（UI §8.5）；inbound 标签展示有效渲染名，有自定义显示名时副行系统标识名；**提交形状 `push_targets: [{instance_id, inbound_tag}]`**，Design2Report10 Q12-6）；配额 input-number（0/留空不限）；**编辑移除已推送目标在保存前提示「将同步从 Xray 移除已取消目标的账号」**（Design2Report10 Q12-6）。
     - 创建成功弹窗一次性展示 `createExtAccount` 响应中的明文凭据（自动生成模式）+ 复制 + 警示文案；**创建（自动生成模式）确认文案提示「若 Xray 侧已存在同 email 账号，将先移除旧账号并以新生成凭据重新推送（覆盖接管，Xray 侧旧账号被踢除）」**（Design2Report11）；「复制凭据」按钮调专用端点 + Toast 警示；**失败推送行内重试调 `retryExtSync`**；删除/重置 ConfirmModal 文案按 UI。
  5. **路由/菜单**：Build4 已加 `/admin/xray` 路由与高级可见性，当前占位为通用 `PlaceholderView.vue`（仓库无 XrayInstancesView.vue）；本 Step **新建 `XrayInstancesView.vue` 并将 router 引用从 PlaceholderView 改为新组件**。
  6. **前端单测**：实例列表加载/开关失败回滚、测试连接、检测回执、初始化确认、对账四分区渲染与 ext 默认不勾选、独立账号双轨表单与一次性凭据展示、**ext 行内重试 loading**。

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test
  cd ../backend && go test ./...
  ```

- **验收标准：** 前端构建与测试通过；UI §8.6 自检结论（空态/加载/错误/权限/防重复/危险确认）逐项走查通过；所有长操作按钮 loading 防重复。

---



---

### Step 4：用户组/用户管理/面板设置高级前端

**本 Step 完成后，GroupsView 按 UI §4.3 完整重构；UsersView 新增高级列与配额操作；SettingsView 新增高级分区与导入导出 v2 文案。**

- **目标：** 落地 Design2-UI §4.3/§4.5/§4.7。
- **前置条件：** Step 3 验收通过。
- **产出文件与操作：**

  1. **`frontend/src/api/group.ts`**：新增/调整——`GroupItem.default_quota/node_count`；`getGroupDetail(id)` 返回 `GroupDetail`（含 `nodes:[{node_id,node_name,display_name,render_name,is_public,source,sort_order}]` 与 `candidate_nodes`/`in_partial_blueprint`）；`updateGroupNodes(id,{node_ids})`、`updateGroupQuota(id,{default_quota})`。移除 Build4 临时最小字段。
  2. **`frontend/src/views/admin/GroupsView.vue`**：按 UI §4.3 重写：
     - 列表：组名/默认配额（GB 或「不限流量」）/分配节点数/用户数/操作（编辑/删除；默认组不可删）。
     - 编辑弹窗四区：改名；节点分配（候选集=后端返回已激活蓝图 xray 并集；节点标签显示 render_name，有自定义 display_name 时副行系统名；公共节点灰置 +「公共·免分配」；非候选集已分配红警；仅部分模板候选橙警；并集为空 `a-empty` + 前往装配）；分配排序（拖拽/上移下移）；默认配额（0/留空不限）；**保存成功 `Notify.success`「已保存，节点变更将同步至 Xray」；列表预置默认组带 a-tag**（UI §4.3）。
     - 删除 ConfirmModal 迁默认组文案。
  3. **`frontend/src/api/user.ts`**：`AdminUser` 增加高级字段（本月用量字节、有效配额、聚合同步状态、last_error 摘要、quota_override、quota_exceeded）；新增 `setUserQuota(id, {quota_override})`（路径与语义按 UI §9.1 `api/user.ts` 扩展的 setUserQuota 行）。
  4. **`frontend/src/views/admin/UsersView.vue`**：按 UI §4.5 扩展：
     - 基础模式隐藏「所属组」列与移动卡片中的所属组字段（advanced_mode；**编辑弹窗换组 Select 同步隐藏**，Design2-UI §4.5 定稿口径）；高级模式显示用量 `X / Y GB`（**不限流量显示 `X GB / 不限`；无任何流量记录显示 `—`**），同步状态聚合 Badge（synced/pending/failed + Tooltip + 行内重试；**无任何推送记录时显示灰「未推送」第四态**），超限红标与行高亮（UI §4.5）。
     - Dropdown 新增「配额覆盖」（input-number，留空=继承；0=不限）与「重置配额」（ConfirmModal；禁用用户按钮置灰 + Tooltip）。
  5. **`frontend/src/api/settings.ts`**：新增 `AdvancedSettings` 类型与 `getAdvancedSettings/saveAdvancedSettings`（关闭时请求含 `confirm_word: "DISABLE"`）；**`importConfig` 已存在（Build5 产物），本 Step 适配 v2 响应分支**（POST 现有导入端点；**响应含 `task_id` 为 v2 走 pollTask 轮询，不含为 v1 同步完成处理**）；新增 **`getAdminTask`**（GET /api/admin/tasks/:id，全局任务端点，kind 含 off_clear/import/xray_init/reconcile_exec/instance_delete）（UI §9.3）。
  6. **`frontend/src/views/admin/SettingsView.vue`**：按 UI §4.7：
     - 新增「高级模式」分区（锚点置于运行模式信息之后）：开关 + 状态 Tag；开启保存轻提示；关闭触发确认词弹窗（**确认词 DISABLE**，清单式展示将被移除内容，manual 节点保留说明，两条 warning：不可达实例手动清理/重开全量重配），**提交后按 task_id 轮询 OFF 清空任务**；采集间隔（分钟，默认 10，**仅 advanced_mode=on 时展示该控件**，Design2Report10 Q12-5）；流量卡片开关（默认开，两模式均展示）。
     - 配置导入/导出分区文案更新：导出说明追加 v2 内容（实例清单含节点显示名映射 + 独立账号推送目标/超限标记）；导入确认弹窗追加「实例整体覆盖、组节点分配将被级联清空」与「带实例/账号导入且高级关闭时将自动开启」；**无实例/账号且高级关闭分支采用双确认词分步：先按现有流程校验 IMPORT，再追加 DISABLE 并显著警告**（Design2Report10 Q5）；signing_key 保护错误以 `a-alert error` 展示后端文案；**v1 导入响应无 task_id 按同步完成处理，v2 导入提交后按 task_id 轮询**，完成提示自动检测（enabled=0 实例跳过并提示）/回填节点显示名/重绑/对账（**对账执行失败不阻断任务，失败原因在提示中展示**）。
  7. **前端单测**：组编辑四态与排序；用户高级列显隐与重试/重置 loading；设置关闭确认词清单与导入错误展示。

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test
  cd ../backend && go test ./...
  ```

- **验收标准：** 前端构建与测试通过；三页在 advanced_mode on/off 下显隐正确；八项自检（空态/加载/错误/暗色/权限/防重复/危险确认）走查通过。

---



---

### Step 5：首页/个人中心/AppHeader 高级展示收口

**本 Step 完成后，用户端两模式展示完全符合 Design2-UI §2.2/§3：流量卡片三态、分流规则卡片、平台卡片两形态、个人中心本月流量与所属组显隐全部正确。**

- **目标：** 收口用户端与全局头部。
- **前置条件：** Step 4 验收通过。
- **产出文件与操作：**

  1. **`frontend/src/api/home.ts`**：沿用 Build4 Step4 已落地的 `getHomeSummary()`（**`GET /api/home/summary`**，Design2Report11 Q13 决策：独立汇总端点）——响应含顶层 `traffic`：基础模式 `{unlimited:true}`；高级模式 `{unlimited, used_bytes, quota_bytes|null, exceeded:boolean}`（**配额不限时亦 `unlimited=true` 且 `quota_bytes=null`**，与 Build6 Step5 实现及 UI §9.3 形状对齐）；含顶层 `home_rule`：`null` 或 `{rule_id, name, current_version, token, download_url}`。**平台卡片字段（普通用户 `status: 'custom'|'ready'|'unassigned'`；管理员 `status:'admin_preview'` + `subscription` 预览字段）仍来自 platforms 端点**（`/api/home/platforms` 保持纯列表，不承载汇总字段）。
  2. **`frontend/src/views/HomeView.vue`**：按 UI §3.1：
     - 卡片顺序流量 → 分流规则 → 平台卡 → 公告栏；流量卡受 `traffic_card_enabled` 控制；高级未超限进度条（<80 蓝 / 80~99 橙 / 100 红），超限红色 alert 文案统一；字节→GB 两位小数。
     - 分流规则卡全体用户可见；正常/空态；SR 双内容引导；点击跳 `/rules`。
     - 平台卡普通用户三态三按钮；管理员预览形态（模板信息 + 内容形态标签 + 激活状态 + 预览按钮禁用 Tooltip）。
  3. **`frontend/src/api/profile.ts`**：新增 `getProfileTraffic()` 调 `GET /api/profile/traffic`，类型与 home.traffic 同形；供 ProfileView 消费（Design2Report8 Q3）。
  4. **`frontend/src/views/ProfileView.vue`**：基本信息「所属组」行基础模式隐藏；「本月流量」行数据源改为 `getProfileTraffic()`，两模式（不限 / 已用 X GB / 配额 Y GB；超限红色附注）；**`traffic_card_enabled=false` 时该行一并隐藏**（Design2-UI §4.7 流量卡片开关口径）。
  5. **`frontend/src/components/AppHeader.vue`**：所属组 `a-tag` 仅高级模式显示（Build4 已做，若为临时实现则按 UI §2.2 收口）。
  6. **`frontend/src/stores/system.ts` / `api/system.ts`**：确保 `advanced_mode` 在守卫、菜单、页面显隐三处共用；`fetchStatus` 在高级设置保存后强制刷新。
  7. **前端单测**：流量卡三态、规则卡空态、管理员预览按钮禁用、Profile 两模式行显隐、AppHeader 组标签显隐。

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test
  cd ../backend && go test ./...
  ```

- **验收标准：** 前端构建与测试通过；UI §3.3 用户端走查结论与 §2.2 头部走查通过；两模式截图/手测无串味。

---



---

### Step 6：全量端到端验收、文档核对与归档

**本 Step 完成后，Design2 全量能力验收通过；Build4~7 全部执行完毕，按 AGENTS §8.2 归档；系统达到可交付状态。**

- **目标：** 全链路回归与文档收口。
- **前置条件：** Step 5 验收通过。
- **产出文件与操作：**

  1. **自动验证**：后端 build/vet/test（含 race 重点包）、前端 build/test；`docker compose build` 使用 Go 1.26 镜像完整构建一次。
  2. **全新部署端到端手工剧本（基础模式段）**：Build4/5 剧本回归：Setup → 平台三格式 → 素材池同步 → 节点/代理组 → 四装配器生成与激活 → 用户端下载。
  3. **全新部署端到端手工剧本（高级模式段）**（可用真实 Xray v26 一台；若无真实实例，使用 Build6 假服务或单独 `xray run` 测试实例，至少验证配置与 API 契约）：
     - 设置开高级 → 侧边栏解锁「用户组/Xray 实例」；
     - 录实例 → 测试连接 → 刷新节点（vless/vmess/trojan/ss 入库；其他协议 allocatable=0）；
     - **节点命名链路**：检测回执给新增 inbound 命名（如 `🇺🇸US-1(电信移动联通)`）→ 节点页再次改名 → 用户下载 Clash YAML/SR/generic 产物中节点名与代理组成员名均为自定义名；
     - 组编辑：候选集引导 → 分配节点排序 → 默认配额；
     - 节点页：is_public 切换确认 → enabled 切换同步提示；
     - 「开始初始化」→ 全部 active 用户 synced（用户页可见）；
     - 用户下载 Clash/SR/generic 装配模板：注入节点与 UUID/密码；YAML 可解析；subscription-userinfo 正确；
     - 采集周期或手动触发：用户页用量增长；把配额调极小 → 下周期超限 → Xray 侧账号移除 + 用户页超限提示 + 下载仍可用；
     - 重置配额 → 重新推送恢复；
     - 实例对账四分区：人为在 Xray 删除一个账号 → 待补推；人为加 ext- 残留 → 疑似分区默认不勾选；凭据不一致 → 移除并重推；
     - 独立账号：自动生成/手填接管、复制凭据、超限与重置；
     - 配置导出（含实例/账号）→ 清空重建 → 导入 v2 → 自动检测/重绑/对账；**并验证节点显示名随导入恢复**；
     - 关闭高级模式：确认词 `DISABLE` + 清单 → 清空生效（用户端回到基础模式、xray 表清空、proxy_groups/blueprints 保留）→ 重新开启不推送 → 重配后初始化恢复。
  4. **设计覆盖矩阵核对**（逐条对照 Design2 第一~五章与 Design2-UI 十章）：
     - Build4：第一~二章、§4.4 版本/分发基础、§5.9 迁移、§5.10 基础改造；
     - Build5：第三章、§4.1~4.5、UI §5~7、UI §9.1 基础 API；
     - Build6：§5.1~5.8 后端；
     - Build7：§5.4/§5.10/§5.11 剩余后端（对账/独立账号/导入导出 v2/OFF 清空/高级设置） + UI §2~4/§8/§9/§10。
     - 任何「设计有、Build 无」的条目必须列出并让用户决策（补齐或挂起），禁止静默忽略；**Design2Report5 的 A1–A12 / B1–B14 修订项必须逐条勾销后方可验收**。
  5. **文档同步**：
     - 四个 Build 文档勾选 ✅、变更记录补执行完成日期；
     - 按 AGENTS §8.2，构建验收完成后将 Build4~7 移入 `docs/AchievedDocuments/`；当前活跃 Build 索引（如 AGENTS.md 文档清单或 README 引用）更新为「已构建完成，见存档」；**Design2.md / Design2-UI.md 保持活跃不归档（本期产品设计基线，后续增量设计另起 DesignN）；Design2Report 系列核验报告随同 Build 归档**；
     - 若执行中出现 bug，按 Issue 模板创建 IssueN.md；若产生新的设计结论，落 Design 文档并经用户确认。
  6. **发布前检查**：`grep -R "TODO\|FIXME\|占位" backend frontend` 无 Build 期遗留；**本 Step 必须同步重写 `.smoke-test.sh` 至 Build4/5 新模型**（现行脚本仍为 Build2/3 旧模型：固定 18080 端口、组选定 `sub_ids/selections`、旧首页 `subscriptions[]` 形状，按当前 API 执行必然失败；重写为：Setup → 注册 → 平台三格式 → 订阅/版本 → 规则/分享 → 首页平台卡新形状，并保持脚本通过）；README 部署文档与高级模式前置条件（Xray policy.stats、IP 白名单、建议 1~5 台实例、OFF 手动清理提示）同步。

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd backend && go test -race ./internal/xray/... ./internal/download/... ./internal/group/...
  cd ../frontend && npm run build && npm test
  cd .. && docker compose build
  bash .smoke-test.sh   # 已按 Step 6 item 6 重写至 Build4/5 新模型；失败必须修复
  ```

- **验收标准：** 全部命令通过；两份手工剧本无未关闭问题；覆盖矩阵经用户确认无遗漏；文档归档与 Issue/Design 同步完成。

---

## 五、候选构建项（本 Build 为最后一轮，验收后清零）

| # | 候选 | 说明 |
|---|------|------|
| 1 | Xray 限速/IP 限制等生态扩展 | 本期不实现；如需要作为新 Design 立项 |
| 2 | 到期时间 expire_at 业务化 | Design2 §5.9 明确预留，本期不使用 |
| 3 | 旧库平滑升级路径 | Design2 §一明确全新部署，不做迁移 |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 初始版本：Build7 构建方案（对账/独立账号/导入导出 v2/OFF 清空/高级 UI/交付收口），6 个 Step；为 Design2 最后一轮 |
| v1.1 | 2026-08-19 | Design2Report5 核验修订：ext 目标后端强校验与 name/quota 校验；实例/节点删除补 ext 目标清理；导入带实例自动开高级、off 无实例执行 OFF 清空；OFF 确认词固定 DISABLE；长请求统一 120s；端到端剧本补节点命名与导入恢复名称 |
| v1.2 | 2026-08-19 | Design2Report7 核验修订：Step1 对账期望集与本实例节点取交集写明（P2-4）；Step2 导入重绑前 xray_ext_users.node_id 统一置 NULL，禁止插 0（P2-5） |
| v1.3 | 2026-08-19 | Design2Report7 复核补齐：Step1 单测补期望集实例交集断言（R3）；Step5 home traffic 类型补高级模式配额不限 `unlimited=true` 分支（R5） |
| v1.4 | 2026-08-19 | Design2Report8 修订：ext retry 端点与一次性凭据创建响应（Q4/P2-14）；导入/OFF 清空异步任务+轮询与 DISABLE 分支（Q5/Q9）；导入 quota_exceeded 导出、disabled 实例跳过检测（P2-13/P2-19）；实例/独立账号长操作 120s；ProfileView 接入 getProfileTraffic；smoke 测试失败即验收失败（P2-18） |
| v1.5 | 2026-08-19 | 构建前核验修订：Step5 条目编号修正（原「1a」并入序列，HomeView 子项归位）；Step6 覆盖矩阵 Build7 表述修正为 §5.4/§5.10/§5.11 剩余后端 |
| v1.6 | 2026-08-19 | Design2Report9 修订：对账期望集拆分用户部分（候选集+可用性）与独立账号部分（仅可用性，M3）；对账执行三端点与实例删除异步化、全局任务端点 /api/admin/tasks/:id 与 kind 枚举（M7/M6）；OFF 幂等 no-op（M15）；采集间隔映射读写单测承接（M9）；v1 导入兼容与装配快照重绑注记；归档范围写明 Design2 保持活跃 |
| v1.7 | 2026-08-19 | Design2Report10 修订：全局任务 registry 改为复用 Build6（Q1）；对账单条同步端点 push-one/credentials-one（Q9）；导入双确认词 IMPORT→DISABLE 与 v1 同步/v2 异步响应口径（Q5）；OFF 翻转判定与任务登记同事务互斥（Q15）；getExtCredentials no-store、testConnection 120s、实例删除轮询 UI、检测/对账错误态、api_addr 变更提示、采集间隔仅高级展示、push_targets 形状与移除确认（Q12） |
| v1.8 | 2026-08-19 | Design2Report11 核验修订：删除钩子补丁去重（仅补 ext 单测）；generate 模式同 email 先 Remove 再 Add 定稿；参数表补节点 enabled=1；120s 清单剔除异步端点；期望集措辞修正；实例/独立账号空态显式列；home 数据改 /api/home/summary；importConfig/getAdminTask 点名 |
| v1.9 | 2026-08-20 | 构建前核验修订（代码事实对照 Build4/5 已落地产物）：Step3 新建组件并更新 router 引用（占位为 PlaceholderView）、setNodeDisplayName 复用 api/node.ts、删实例 ConfirmModal 影响清单+不可达 warning、检测四项全 0 Notify.info 分支、推送目标无可用节点空态（UI §8.1/§8.2/§8.5）；Step4 importConfig 改已存在适配措辞、用量「—/不限」与「未推送」第四态、组保存提示与预置默认组 Tag（UI §4.3/§4.5）；Step2 导入后自动对账失败口径定稿（任务 succeeded+完成提示记录，用户决策） |
| v1.10 | 2026-08-20 | 第二轮构建前深度核验修订（Build4/5 验收后代码事实对照）：① Step1 CreateExt email 占位措辞修正（先写临时占位值、LastInsertId 后回填 `ext-{id}@vpn.local`，原措辞字面上用未知 id 占位不可实施）；② Step4 补 UI §4.5「编辑弹窗换组 Select 同步隐藏」（基础模式组概念全面隐藏口径的遗漏点）；③ Step2 OFF 清空提交后清理循环注明复用 Build6 Step3 的 `AfterAdvancedOff` 补偿辅助（闭合跨 Build 衔接，防 Build6 预建辅助成死代码） |
| v1.11 | 2026-08-20 | 第三轮 Build6/7 事前预检确定性问题修订：① Step1 CreateExt 临时 email 必须唯一化（固定占位串在并发创建时撞 UNIQUE），并明确 `xray_ext_users.node_id` 按 (instance_id, inbound_tag) 解析写入；② Step2 `/api/admin/settings/advanced` 明确只叠加 session+admin、不得套 advancedMode（否则 OFF 态无法开启/关闭与轮询）；③ Step2 Import v2 的 slug 冲突/重复明确为「任务终态 failed + error 文案按 400 口径」，HTTP 提交阶段无法同步返回 400；④ Step4 基础模式所属组隐藏补移动卡片态；⑤ Step6 明确必须先重写 `.smoke-test.sh`（现行脚本仍为 Build2/3 旧模型，按当前 API 必然失败） |
| v1.12 | 2026-08-20 | 用户决策落盘：Q2 全局任务 registry 下沉为中性包 `internal/tasks`（server 构造注入；OFF 清空与配置导入的业务包经注入 Registry 登记，不 import server）；Q3 OFF 清空保持「状态翻转+任务登记同事务」字面口径，接受 DB 回滚残留幽灵任务边界（重启后 failed 兜底）。Step2 item2/item3/item4 同步修订 |
| v1.13 | 2026-08-20 | 将 Build6-2 #5（独立账号采集与配额检查）显式整理进 Step1：补充承接说明、目标/前置条件更新，并在 cron 条目标注必须在本 Step 闭环 |

