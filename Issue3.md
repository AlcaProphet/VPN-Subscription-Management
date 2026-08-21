# Issue3.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考），承接 [Issue2.md](./Issue2.md)（R12~R15 系列，保留备查）。
> 设计记录见 [Design2.md](./Design2.md) 与 [Design2-UI.md](./Design2-UI.md)；构建方案见 [Build4.md](./Build4.md)～[Build7.md](./Build7.md)；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 〇、本轮核查背景（2026-08-21 Build 1~7 全量核查）

- **核查范围：** Build1~7 构建合规性、验收目标达成度、代码健壮性与引用完整性、Issue2 已知问题修复状态复核。
- **验收命令实测：** 后端 `go build ./... && go vet ./... && go test ./...` 全绿；前端 `npm run build` 通过、`npm test` 16 文件 46 例通过；`TestRenderClash10kRulesThreshold` / `TestRenderSrConf10kRulesThreshold` 均 PASS；旧分发表 `group_selections`/`subscription_group_rel` 已在迁移 1009 DROP、业务代码零引用；Go 1.26（go.mod / Dockerfile `golang:1.26-alpine`）一致。
- **Build1~3（第一期基线）：** 安全基线抽查全部成立（token 日志脱敏、下载 no-store、/public 路径穿越防护、enc:v1 加密存储、sessionMW+adminMW 双层管理端校验），无新发现。
- **Build4/5：** R12/R13/R14 系列修复全部核实落地（含 R14-06 自包含 ClashPlan、R14-07 xray_candidates 填充），无新发现。
- **Build6：** R15-01/02/03/04/06/09/11/12 的「✅ 已修复」声明经抽查**全部属实**；遗留问题集中在 error 处理与测试缺口（并入 R16-07/08）。
- **Build7：** R15-05 修复**部分落地**（见 R16-04/05）；独立账号状态机存在两处新语义缺陷（R16-01/02）；Step3/4 前端完成度约 50%/55%（R16-06）；Step5 已落地属实。
- **Issue2 未闭环项复核：** R15-07/08/14 标记 ◧ 与代码现状一致，无虚报；R15-13 待用户决定。
- **新发现问题编号：** R16 系列（延续 Issue2 编号体系）。与 Issue2 未闭环项重叠的部分（前端未落地簇/测试缺口/error 残留），本文只记录**新检测到的增量**，并在条目中注明归属关系。

---

## 一、进行中问题

### R16-01 独立账号超限摘除销毁推送目标行，重置配额后无法恢复推送

- **现象：** `internal/xray/ext.go` `CheckExtQuota` 超限时对每个目标调用 `removeOne`（:602-604）；`removeOne` 在 RemoveUser 成功（或 not found）后 `DELETE FROM xray_ext_users`（:807-809）。而 `xray_ext_users` 正是独立账号推送目标载体：`ResetExtQuota`（:502）与对账期望集均通过 `pushTargetsFor` JOIN 该表取目标。超限摘除一次后目标行全部消失，重置配额 `pushTargetsFor` 返回空集——**账号无法恢复推送，对账也永久看不到这些目标**，只能靠管理员重新编辑推送目标修复。
- **根因：** 「超限摘除」语义只应作用于 Xray 侧（RemoveUser + quota_exceeded=1），不应删除本地期望集记录；`removeOne` 被复用在了「删除目标」与「超限摘除」两种语义上。
- **影响范围：** Build6 Step5/Build7 Step1 的「超限摘除 + 重置配额恢复」闭环断裂；对账期望集失真。面板用户侧（xray_users 仅状态记录）无此问题。
- **修复方案（用户已确认 2026-08-21，采用方案 A）：** `CheckExtQuota` 改用不删行的摘除路径（仅 RemoveUser，保留 xray_ext_users 目标行并置状态标记，如 sync_status='removed'/action 标记；超限置位不变），`ResetExtQuota` 与对账仍按该表重推；`removeOne` 的删行语义仅保留给「编辑移除目标/删除账号」路径。补「超限摘除→重置→目标仍在且重推成功」单测。（决策后同步更新至 Build6 Step5/Build7 Step1）
- **状态：** ☐ 待修复

### R16-02 manual 接管模式 already-exists 接管成功语义未落地

- **现象：** `internal/xray/ext.go:772-792` `pushOne` 仅 `overwrite=true`（generate）分支处理 `IsAlreadyExists`（先 Remove 再 Add 覆盖）；`manual`（overwrite=false）模式 AddUser 返回 `already exists.` 时直接 `markExtFailed`。Build7 Step1 item 明确「`manual`（手填接管）模式保持原口径——`already exists.` 视为接管成功」，Build7 Step1 单测清单同样点名该口径。
- **根因：** 凭据接管双模式语义只实现了 generate 分支。
- **影响范围：** 管理员手填既有凭据接管 Xray 侧已存在账号时必然标记 failed，接管流程不可用。
- **修复方案：** `pushOne` 在非 overwrite 分支对 `IsAlreadyExists(err)` 视为成功（`markExtSynced`）；补 manual 模式 already-exists 单测（Build7 Step1 单测清单已列）。
- **状态：** ☐ 待修复

### R16-03 v2 导入 ext node_id 重绑失配时置 NULL+pending 永久悬挂

- **现象：** `internal/config/export.go:514-519` 重绑用子查询 `UPDATE xray_ext_users SET node_id = (SELECT id FROM nodes WHERE instance_id=? AND tag=? AND source='xray' LIMIT 1)`，未匹配时 node_id 落 NULL 且 sync_status 保持 pending。而 `pushTargetsFor` 以 `JOIN nodes ON n.id = xu.node_id` 取目标——NULL 行永远查不到：**该目标既不推送、不失败、不进对账，永久悬挂**。Build7 Step2 item4 口径为「按 instance slug+tag 重绑 xray_ext_users.node_id（**未匹配置 failed**）」。
- **根因：** 重绑后未按失配口径回写 failed。
- **影响范围：** 导入 v2 后节点缺失/改名场景下，对应独立账号目标静默丢失且不可观测、不可重试。
- **修复方案：** 重绑 UPDATE 后检查 `node_id IS NULL` 的行并置 `sync_status='failed'`、`last_error='导入重绑未匹配节点'`（或先 SELECT 解析再分支写入）；补失配单测。
- **状态：** ☐ 待修复

### R16-04 v2 导入任务终态 result 丢失完成提示（hints 仅写日志）

- **现象：** `internal/config/export.go:301` 任务成功仅 `Succeed(taskID, {"imported": true})`；导入后收集的 hints（旧账号清理计数、跳过检测实例、显示名回填失败、重绑失败、重推结果，:447-464）只写入 `log.Warn`。Build7 Step2 要求「返回完成提示字段」，决策口径为终态 result 返回 `{imported, hints, skipped_instances, reconcile_errors}`；前端 SettingsView 轮询终态也因此只能展示固定文案。
- **根因：** `importV2` 返回值为 error，hints 未随结果上抛。
- **影响范围：** 管理员导入后无法得知跳过实例/未匹配映射/重推失败等关键提示；v2 往返闭环不完整。
- **修复方案：** `importV2` 改为返回 `(hints []string, err error)`（或结构体），`Succeed` 写入 `{imported:true, hints:[...]}`；前端轮询终态消费 hints 展示。与 R15-05 遗留项一并收口。
- **状态：** ☐ 待修复

### R16-05 v2 导入后装配快照 xray 引用重绑与账号对账未实现

- **现象：** `internal/server/server.go:402-409` `SetPostImportRebindReconcile` 实际执行 `SyncAllActive + SyncAllExt` 全量重推，payload 参数被忽略；**装配快照（selection_json / render_plan_json）xray 引用按节点稳定名重绑、以及 Build7 Step1 账号对账，全代码库无实现**（grep 无命中）。Build7 Step2 item4 明确要求「装配快照 xray 引用重绑（同名重绑、失配悬空容错）→ 执行账号对账（best-effort，失败记入完成提示）」。
- **根因：** R15-05 修复以全量重推替代了重绑+对账环节。
- **影响范围：** 导入后历史装配快照中的 xray 引用可能指向已不存在/已改名的节点而无人重绑；导入后账号与 Xray 侧一致性无对账兜底。
- **修复方案（待确认）：** 按 Issue2「R15 决策与修复方法」R15-05 既定口径补齐：装配快照按节点稳定名重绑（失配悬空容错并计入 hints）+ best-effort 对账（失败不阻断 succeeded、原因记入 hints）。
- **状态：** ☐ 待修复

### R16-06 Build7 Step3/4 前端新检测缺失簇（R15-07 增量）

- **现象：** 在 Issue2 R15-07 已记录范围之外，本轮逐项对照 Design2-UI §8/§4.3/§4.5/§4.7 新确认以下缺失：
  - `XrayInstancesView.vue`：行内启停开关（状态列仅 Tag，updateInstance 恒传原 enabled）、检测四项全 0 → Notify.info「节点无变化」分支、added_nodes 逐行命名区（API 类型已定义但无 UI 与 setNodeDisplayName 调用）、对账四分区明细/清理勾选/行内 push-one/credentials-one（API 已封装零调用）、对账执行 task_id 未轮询、一键清理无危险 ConfirmModal、ext 推送目标多选 UI（push_targets 声明但提交恒空）、ext 配额 InputNumber、generate 一次性凭据即焚展示弹窗、ext 编辑入口（updateExtAccount 未 import）、ext 删除/重置 ConfirmModal、初始化三点确认 ConfirmModal 与 synced/failed 计数回执、测试连接仅新增弹窗有且非 a-alert。
  - `GroupsView.vue`：分配排序区（§4.3 第 3 区整区缺失）、候选集公共节点灰置「公共·免分配」、非候选集已分配节点红警、并集为空 a-empty+「前往装配」按钮、render_name 副行。
  - `UsersView.vue`：§4.5 整体未落地（高级列用量/同步状态四态 Badge/超限红标与行高亮、配额覆盖/重置 Dropdown、基础模式隐藏所属组列与换组 Select）；后端与 api/user.ts 支撑已就绪，纯页面未接线。
  - `SettingsView.vue`：v2 导入完成 hints 展示（与 R16-04 联动）、导出 v2 文案、导入确认追加影响项、signing_key 保护表单级 alert、OFF 确认弹窗内嵌 DISABLE 确认词输入框（当前硬编码提交）；v2 导入 DISABLE 第二步对**所有导入无条件弹出**（规格仅「无实例/账号且高级关闭」破坏分支追加）。
  - `frontend/src/views/admin/PlaceholderView.vue` 文件残留（router 已无引用）。
- **根因：** Build7 Step3/4 标记 ✅ 时前端仅完成约一半交互（Step3 约 50%、Step4 约 55%，UsersView 为 0）。
- **影响范围：** 高级模式管理面 UI 不可完整使用；Build7 Step3/4 验收不达标。
- **修复方案：** 随 R15-07 既有方案逐项补齐；本条目作为 R15-07 的细化缺失清单，修复时逐项勾选。
- **状态：** ☐ 待修复（归属 R15-07 收口）

### R16-07 Build6/7 专项测试缺口新检测项（R15-08 增量）

- **现象：** 在 Issue2 R15-08 清单之外新确认：`internal/cron/xray.go`（流量采集核心：interval 校验/防重入/advanced_mode 实时判定）无单测（cron 目录仅 pool_test.go）；`internal/xray/errors.go` 的 `IsOpError` 仅被测试文件引用（测试辅助边界项）；`frontend/tests/xray-instances-view.spec.ts` 仅 1 个空态用例，未覆盖 Build7 Step3 item6 要求的开关回滚/测试连接/检测回执/对账四分区/ext 双轨/重试 loading；`TestRenderUserSubscription10kRules` 仍不存在（复验 `no tests to run`）。
- **根因：** 同 R15-08——构建文档单测清单未随实现执行。
- **影响范围：** 采集逻辑与前端核心交互无回归保护；R16-01~05 类缺陷无法被测试拦截。
- **修复方案：** 并入 R15-08 测试补齐清单：新增 cron/xray 采集单测、扩展 xray-instances-view.spec、落地 TestRenderUserSubscription10kRules（1 万规则 <500ms）。
- **状态：** ☐ 待修复（归属 R15-08 收口）

### R16-08 忽略 error 与死代码残留新检测簇（R15-14 增量）

- **现象：** 在 Issue2 R15-14 已列位置之外，本轮新确认：
  - `internal/xray/ext.go:778`（generate 覆盖前 GetInboundUsers 失败静默）、`:785`（覆盖前 RemoveUser `_ =`）、`:807-809`（removeOne 成功路径 DELETE `_ , _ =`）；
  - `internal/xray/detect.go:366/411` `raw, _ := json.Marshal`，失败时传输/安全参数合并被静默跳过；
  - `internal/server/render.go:110-118` 动态节点 NodeRenderParams/RenderLink 出错静默 `continue`（无日志，用户订阅可能缺节点而无任何可观测信号）；
  - `internal/server/render.go:227-233` netSplit 以 `strings.LastIndex(":")` 分割 host:port，无括号包裹的 IPv6 字面量会误切（边界存疑项）；
  - `internal/approval/approval.go:53/187` `SetOnRejected` 定义与调用点仍在，server.go:322 注入空实现（半死代码，注释称由 onUserDeleting 承接——应删除接口或明确注记）；
  - `internal/xray/reconcile.go:92` `_ = instID` 无效扫描变量；
  - 已确认修复属实项（本轮核实，不再追踪）：FormatVersionV1/StoreDB 已删、cron stop 已保存调用、userUsage 重复行已删、trafficPayload 已返回 error。
- **根因：** R15-14 收口未覆盖上述位置。
- **影响范围：** 关键失败静默（尤其导出/检测/渲染路径）；违反 AGENTS「所有 error 必须处理」。
- **修复方案：** 随 R15-14 收尾逐项处理：显式 error/至少 warn 日志；render.go 节点渲染失败补 warn；SetOnRejected 删除或接线；netSplit 兼容 IPv6（`net.SplitHostPort`）。
- **状态：** ☐ 待修复（归属 R15-14 收口）

### R16-09 .smoke-test.sh 未覆盖 Build4+ 新模型功能路径

- **现象：** `.smoke-test.sh`（66 行）仅覆盖 setup/注册/订阅/版本/规则/分享/首页等 Build1~3 基础路径；素材池、节点、代理组、装配、Xray、导入导出 v2 等 Build4~7 核心端点零覆盖（grep assembly|pools|nodes|xray 命中 0）。Build7 Step6 验收要求「必须先重写 .smoke-test.sh」。
- **根因：** smoke 脚本最后一次更新停留在 Build4/5 基础模型（N4），Build6/7 未续写。
- **影响范围：** 端到端冒烟无法拦截 R15/R16 级链路缺陷。
- **修复方案：** 按当前 API 重写 smoke 脚本：素材池 CRUD+同步、manual 节点、代理组、四类装配生成、版本激活、下载（含动态渲染前置）、Xray 实例 fake 可选分支、v2 导出导入往返。
- **状态：** ☐ 待修复

### R16-10 Build6/Build7 文档勾选状态与代码现状不一致

- **现象：** Issue2「R15 决策与修复方法」决策 3 要求「R15 修复开始时，将 Build6/Build7 中受影响的 Step 回退为 ◧；修复完成并执行该 Step 原验收命令通过后，才恢复 ✅」。首轮修复已执行（多项 ✅），但 Build6.md Step0~6 与 Build7.md Step1~5 的 TODOLIST 与进度表**仍全部标 ✅**；其中 Build7 Step3/4 实测完成度约 50%/55%（R16-06）、Step1/2 含 R16-01~05 未闭环缺陷，Build6 Step5 含测试缺口（R16-07），按决策口径均应回退 ◧。
- **根因：** 修复执行后未回写 Build 文档勾选。
- **影响范围：** 新会话按文档状态会误判 Build6/7 已全量达标。
- **修复方案（已执行）：** 按决策 3 回退受影响 Step 为 ◧：Build6 Step5/6、Build7 Step1~4（Build6.md v2.6 / Build7.md v1.16 变更记录已同步）；归档与 AGENTS/README 状态更新仍按 R15-13 由用户决定时机。
- **状态：** ✅ 已执行（2026-08-21，文档勾选回退完成；各 Step 复验通过后恢复 ✅）

---

## 附、后续修复顺序（交接给下一轮）

> 用户决策（2026-08-21）：**本轮仅记录问题、修复方案与顺序，不改动代码**；修复在用户下令后执行。Build6/7 受影响 Step 勾选已回退 ◧，各 Step 复验通过后才恢复 ✅。

### 优先级 1：后端功能性缺陷（阻塞高级模式下半链路）

1. **R16-01**：ext 超限摘除改不删行（方案 A 已确认）；涉及 `internal/xray/ext.go` CheckExtQuota/removeOne 语义拆分；补摘除→重置单测。
2. **R16-02**：pushOne manual 分支 `IsAlreadyExists` 视为接管成功；补单测。
3. **R16-03**：v2 导入重绑失配行置 `failed`+last_error；补失配单测。
4. **R16-04**：`importV2` 返回 hints，`Succeed` 写入 `{imported, hints, ...}`；前端 SettingsView 轮询终态消费 hints。
5. **R16-05**：按 Issue2 R15-05 既定口径补齐装配快照 xray 引用重绑（同名重绑、失配悬空容错）与 best-effort 对账，替换 server.go 的 SyncAllActive/SyncAllExt 全量重推实现。

### 优先级 2：前端补齐（R15-07 + R16-06 合并执行）

6. 按 R16-06 缺失清单逐项补齐 XrayInstancesView/GroupsView/UsersView/SettingsView；同步补 `groups-view.spec.ts`、`users-view.spec.ts`、`settings-view.spec.ts`，扩展 `xray-instances-view.spec.ts`。

### 优先级 3：质量收口（R15-08/14 + R16-07/08/09）

7. 测试补齐：`TestRenderUserSubscription10kRules`、cron/xray 采集单测、xray 包缺失测试（按 R15-08 原清单）。
8. error/死代码清理：按 R16-08 与 Issue2 R15-14 剩余清单逐项处理。
9. 重写 `.smoke-test.sh`（R16-09）覆盖 Build4~7 核心路径。

### 验收口径

- 每项修复完成后执行对应 Build Step 的原始验收命令；全部完成后：
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm test
  cd backend && go test ./internal/assembly -run 'TestRenderClash10kRulesThreshold|TestRenderSrConf10kRulesThreshold' -v
  ```
- 复验通过后恢复 Build6/7 对应 Step 勾选 ✅；归档（R15-13）等用户指令。

---

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
5. 非问题的优化候选 / 已知遗留事项归 [Design2.md](./Design2.md) §三「后续设计候选」记录，不记录在本文件。

---

## 三、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-21 | 初始版本：Build1~7 全量核查。验收命令全绿实测（后端 build/vet/test、前端 build/46 单测、渲染阈值双测试 PASS、迁移 1009 旧表 DROP 确认、Go 1.26 一致）；R15 已修复声明抽查属实（R15-01/02/03/04/06/09/11/12），R15-07/08/14 ◧ 与现状一致；新追加 R16-01~R16-10（ext 超限摘除销毁期望集、manual 接管语义、v2 导入重绑失配悬挂/hints 丢失/装配重绑与对账缺失、前端与测试缺口增量、error 残留增量、smoke 脚本、文档勾选状态不一致）。 |
| v1.1 | 2026-08-21 | 用户决策落盘：① R16-01 采用方案 A（摘除不删行）；② Build6 Step5/6、Build7 Step1~4 勾选已回退 ◧（Build6.md v2.6 / Build7.md v1.16），R16-10 标为已执行；③ 本轮仅记录问题与修复方案/顺序，不改代码（新增「附、后续修复顺序」交接章节）；归档仍按 R15-13 由用户决定。 |
