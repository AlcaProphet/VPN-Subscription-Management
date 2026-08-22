# Issue5.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考），承接 [Issue4.md](./Issue4.md)（R19 系列，保留备查）。
> 设计记录见 [Design2.md](./Design2.md) 与 [Design2-UI.md](./Design2-UI.md)；构建方案见 [Build4.md](./docs/AchievedDocuments/Build4.md)～[Build7.md](./docs/AchievedDocuments/Build7.md)（已归档）；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 〇、本轮核查背景（Build1~7 + Issue1~4 双重核验）

- **核查范围：** 依据 Build1~7、Issue1~4、Design2/Design2-UI 对当前代码做静态核验；不执行构建/测试/压力测试。
- **总体结论：**
  - Build4/5 主体能力（规则素材池、manual 节点、代理组、四类装配器、分发 UI）与 Build6/7 主体后端（Xray 实例/同步/下载渲染/独立账号/导入导出/OFF 清空）均已落地，旧分发表已清零，Go 1.26/xray-core 版本与文档一致。
  - Issue1 历史问题基本已修复；Issue2 中 R12~R15 已闭环项经抽查基本属实。
  - Issue3 中 **R17-06（前端专项测试覆盖不足）与 R17-07（smoke Production 自动拉起未闭环）仍为 ◧**，与文档状态一致。
  - Issue4 中 R19 系列大部分已修复，但 **R19-05 的“重启后平台数据疑似丢失/平台列表为空”补充现象仍未现场排查**，不能视为完全闭环。
  - 本轮（二轮）逐文件复扫后新增 **R20-16~R20-27**：素材池 Scanner 错误误删风险、OFF 下载仍依赖凭据解密、单条/独立账号 AddUser 缺首建与 OFF 补偿、导入保护绕过、Xray 页 UI 分支缺失、实例删除 404 契约、装配 400/500 映射、候选集解析 fail-open、error 处理残余、Design2-UI 未同步 Issue4、Build6/7 文档勾选不一致、参数校验缺口等。

---

## 一、进行中问题

### R20-01 独立账号服务多处忽略错误返回值，违反 AGENTS「所有 error 必须处理」

- **现象：** `backend/internal/xray/ext.go` 中仍存在多处 `_ =` 忽略错误：
  - `GetExt`/`ListExt` 读取本月用量：`_ = s.store.DB().QueryRowContext(...).Scan(&acc.UsedBytes)`（约 :287/:315），查询失败时用量静默为 0。
  - `UpdateExt` 读取 `quota_exceeded`：`_ = ...Scan(&exceeded)`（:287）；`_ = s.removeOne(...)`（:290/:339）忽略移除失败。
  - `RetryExt` 读取 `quota_exceeded`：`_ = ...Scan(&exceeded)`（:376）。
  - `DeleteExt` 逐个 `_ = s.removeOne(...)`（:339）。
- **根因：** R16-08/R17 修复未覆盖这些 best-effort 清理/统计读取路径。
- **影响范围：** 独立账号用量可能显示 0；编辑/删除时 Xray 侧清理失败无日志、无提示；超限状态读取失败时可能错误推送。
- **修复方案：** 对这些 DB 读取至少 `log.Warn` 并保留可观测性；`removeOne` 失败应记录 warn/error，`UpdateExt`/`DeleteExt` 可继续返回成功但必须有日志。
- **状态：** ☐ 待修复

### R20-02 用户凭据修复路径忽略 RemoveUser 失败计数，可能“假成功”

- **现象：** `backend/internal/xray/reconcile.go:237` 的 `CredentialsOne` 用户分支：
  ```go
  _, _, _ = s.RemoveUserFromTargets(ctx, *item.UserID, []Target{target})
  return s.pushUserTarget(ctx, *item.UserID, target)
  ```
  `RemoveUserFromTargets` 返回 `removed, failed, nil`；当移除失败（failed>0）时仍继续 AddUser。由于 `Client.AddUser` 对 `already exists.` 视为幂等成功，若旧账号仍在 Xray 侧且凭据不一致，AddUser 会“成功”但实际未修复。
- **根因：** 独立账号分支已修复（R17-02），用户分支漏改。
- **影响范围：** 对账“凭据不一致”修复可能假成功，用户继续使用旧凭据。
- **修复方案：** 用户分支检查 `failed > 0` 则返回错误/跳过 Add；或复用 ext 分支的“先 Remove 成功后再 Add”语义。
- **状态：** ☐ 待修复

### R20-03 关闭高级模式时确认词错误被映射为 500，而非 400

- **现象：** `backend/internal/xray/offclear.go:63` 返回 `errors.New("确认词不正确")`；`backend/internal/server/settings.go` 的 `saveAdvanced` 统一走 `mapSettingsErr`，该函数默认分支返回 `500 Internal Server Error`，未将该错误归类为参数错误。
- **影响范围：** 用户输入错误 DISABLE 时看到 500，而不是表单校验类 400；与 AGENTS 错误码约定不符。
- **修复方案：** 在 `mapSettingsErr` 中增加“确认词不正确/高级模式开关参数错误”映射为 400；或将 `OffClearService.SubmitAdvancedMode` 返回包装为 `ErrBadRequest`。
- **状态：** ☐ 待修复

### R20-04 死代码：`SyncAllActive`、`SyncAllExt`、`InstanceService.CloseAll`、`nextURLOrderTx` 无调用方

- **现象：**
  - `backend/internal/xray/sync.go:310` `SyncAllActive` 全库无调用。
  - `backend/internal/xray/ext.go:636` `SyncAllExt` 全库无调用。
  - `backend/internal/xray/instance.go:78` `CloseAll` 全库无调用（服务停机未关闭缓存 gRPC 连接）。
  - `backend/internal/pool/pool.go:412` `nextURLOrderTx` 全库无调用（批量同步改造后遗留）。
- **根因：** 精确 diff 与异步任务改造后，旧的全量同步入口未被清理；`CloseAll` 预留后未接线；URL 段排序计算改为内存递增后旧辅助函数未删除。
- **影响范围：** 死代码增加维护成本；gRPC 连接在进程退出时由 OS 回收，但优雅退出未显式关闭。
- **修复方案：** 删除无调用方的 `SyncAllActive`/`SyncAllExt`/`nextURLOrderTx`（或补充调用）；在 `Server.Run` 优雅退出或 `main` defer 中调用 `InstanceService.CloseAll`。
- **状态：** ☐ 待修复

### R20-05 装配输入 `group_node_orders` 缺少后端校验

- **现象：** `backend/internal/assembly/validate.go` 校验了 `GroupNames`、`OverseasMembers`、规则目标等，但未校验 `GenerateInput.GroupNodeOrders`：
  - key 是否属于本次勾选的代理组；
  - value 是否仅为已勾选节点/合法子组；
  - 是否包含重复或未知节点。
  渲染时 `render_clash.go` 对未知节点静默丢弃，可能生成与用户预期不符的代理组成员。
- **影响范围：** 异常/篡改请求可绕过前端 UI 产生不完整代理组；R19-09 的“本次装配覆盖顺序”缺少后端兜底。
- **修复方案：** 在 `validate` 中增加 `GroupNodeOrders` 与勾选组/勾选节点集合的一致性校验，非法即 400。
- **状态：** ☐ 待修复

### R20-06 装配页“构建前必选配置预检”未真正校验目标平台已有订阅条目

- **现象：** `frontend/src/views/admin/AssemblyView.vue` 的 `buildPreflightMissing`（约 :104-115）只检查“存在匹配 product_type 的平台”，未检查该平台是否已有订阅条目；后端会在 preview/generate 时返回“请先在订阅管理为该平台创建订阅条目”。
- **根因：** R19-04 修复时 AssemblyContext 不包含 subscriptions，预检未补充订阅数据。
- **影响范围：** 用户仍可能在填写大量表单后才被后端拒绝，R19-04 目标未完全达成。
- **修复方案：** 在加载装配上下文时额外拉取 `listSubscriptions()`，预检中增加“目标平台已有订阅条目”；或后端 context 增加订阅摘要。
- **状态：** ☐ 待修复（与 R19-04 关联）

### R20-07 `mergeProtoMap`/`mergeStreamSettings` 部分序列化错误仍静默丢弃

- **现象：** `backend/internal/xray/detect.go`：
  - `mergeProtoMap` 在 `json.Marshal`/`Unmarshal` 失败时直接 `return`（:335-342），无日志；
  - `mergeStreamSettings` 在多个 `json.Unmarshal` 失败时 `continue`，部分无日志（:372-374 等）。
- **影响范围：** 节点检测时若个别 proto 序列化异常，相关字段静默丢失，难以排查。
- **修复方案：** 至少 `slog.Warn` 记录错误；或返回 error 由上层处理。
- **状态：** ☐ 待修复

### R20-08 素材池同步仍无任务级超时/取消（R19-01 缓解但未完全消除）

- **现象：** `backend/internal/pool/sync.go:196` 仍使用 `context.Background()`，数据库写入阶段无整体超时/取消；Issue4 R19-01 根因中的“缺少任务级超时/取消”未直接修复，当前仅通过独立连接 + 批量写入缓解 API 读阻塞。
- **影响范围：** 极端大列表/异常 SQLite 锁等待时，同步任务仍可能长时间占用独立写连接；定时任务也可能无限挂起。
- **修复方向（推测，待用户复核）：** 为 `runSyncTask` 增加整体超时（如 30 分钟）或可取消信号；在任务状态中体现取消。
- **状态：** ◧ 待复核/待用户决策（当前缓解措施已落地，但根因未完全闭合）

### R20-09 Issue3 R17-06 测试补齐仍未完整闭环

- **现象：** 现有前端专项测试仍以空态/基础渲染为主：
  - `frontend/tests/xray-instances-view.spec.ts` 仅 2 个用例（63 行）；
  - `groups-view.spec.ts` 仅空态 1 例；
  - `users-view.spec.ts` 仅空态 1 例；
  - `settings-view.spec.ts` 仅“高级模式卡片”1 例。
- **根因：** R17-06 中“开关回滚/测试连接/检测回执/对账四分区/ext 双轨/重试 loading”等交互覆盖未补全。
- **影响范围：** 高级模式前端核心交互缺少回归保护。
- **修复方案：** 按 R17-06/R15-08 清单扩展上述 spec。
- **状态：** ✅ 已修复（2026-08-22，前端专项测试扩展至 65 例，覆盖实例开关回滚、测试连接、检测回执、对账四分区、ext 双轨选项/重试调用、组编辑候选集与排序区、用户高级列、设置 DISABLE/导入导出提示；loading 态以接口调用断言兜底）

### R20-10 Issue3 R17-07 smoke 脚本 Production 自动拉起仍未闭环

- **现象：** `.smoke-test.sh` 已改为非 Production 直接退出并要求手动验证，但未自动拉起 Production 临时实例；且当前 smoke 仅覆盖 Clash YAML 装配，未覆盖 SR subs / generic-subs / sr-conf / 导入 v2 完整往返。
- **影响范围：** Build7 Step6 的端到端验收仍依赖人工；CI/自动化不能完整拦截。
- **修复方案：** 按 R17-07 计划增加 Production 临时实例自动拉起（或提供明确的一键脚本），并扩展四类装配器路径。
- **状态：** ◧ 进行中（与 Issue3 R17-07 一致）

### R20-11 Issue4 R19-05 “重启后平台数据疑似丢失”仍未现场排查

- **现象：** Issue4 R19-05 状态虽标 ✅，但其补充现象“重启后平台数据丢失/平台列表为空”仍注明“需现场数据库/WAL/数据卷排查”；当前代码静态分析无法确认该现象已消除。
- **影响范围：** 若真实存在数据卷/WAL 异常，可能导致用户数据丢失。
- **修复方案：** 需要用户提供现场数据库/WAL/数据卷状态，或按部署环境做数据持久化验证（非本静态分析范围）。
- **状态：** ✅ 已闭环（2026-08-22，用户确认环境不可用，未能复现；转 ProdTestList 人工验证）

### R20-12 `CheckXrayReferences` 忽略 render_plan 解析错误

- **现象：** `backend/internal/assembly/rebind.go:85` `_ = json.Unmarshal([]byte(b.plan), &plan)` 忽略错误；若 render_plan_json 损坏，引用核对会静默跳过该蓝图的 plan 部分。
- **影响范围：** 导入后装配快照 Xray 引用漏检。
- **修复方案：** 解析失败时追加 hint 或返回错误。
- **状态：** ☐ 待修复

### R20-13 `writeQuotaExceededError` 与 `recordCollectError` 写库失败静默

- **现象：**
  - `backend/internal/xray/sync.go:569` `_, _ = s.store.DB().ExecContext(...)` 写超限原因失败无日志；
  - `backend/internal/xray/stats.go:86` `_, _ = s.store.DB().ExecContext(...)` 写采集错误状态失败无日志。
- **影响范围：** 状态记录可能丢失，用户/运维看不到超限或采集错误。
- **修复方案：** 写库失败至少 `log.Warn`。
- **状态：** ☐ 待修复

### R20-14 README 缺少高级模式部署说明

- **现象：** `README.md` 中 grep 不到 `Xray`/`高级模式`/`policy.stats`/`IP 白名单`/`DISABLE` 等关键字；Build7 Step6 要求 README 同步高级模式前置条件（Xray gRPC API、policy.stats、IP 白名单、建议 1~5 台实例、OFF 手动清理提示）尚未落地。
- **根因：** R15-13 只更新了 AGENTS/归档，README 未补。
- **影响范围：** 部署者缺少高级模式必要前置信息。
- **修复方案：** 在 README 增加高级模式部署章节。
- **状态：** ☐ 待修复

### R20-15 Build6/Build7 文档勾选状态与归档状态多处不一致

- **现象：**
  - `docs/AchievedDocuments/Build7.md` 当前 TODOLIST/进度表中 **Step1~4 与 Step6 仍为 ◧**（备注还指向“Issue3 R17-01/02/04/05/06/08 未闭环”），但其变更记录 v1.17~v1.20 已写明 Step1/2/3/4 恢复 ✅；Issue3 对应条目也已标 ✅/部分修复。
  - `docs/AchievedDocuments/Build6.md` 当前 Step5/Step6 仍为 ◧，但其变更记录 v2.7 已写明恢复 ✅。
  - AGENTS.md 已把 Build4~7 整体列为“已存档/已验收”，与 Build7 Step6 ◧ 冲突；且 AGENTS.md 文档清单仍只列 Issue3/Issue4 为当前问题记录，未索引本轮新增的 [Issue5.md](./Issue5.md)。
- **影响范围：** 新会话可能误判 Build6/7 各 Step 尚未完成或已完成；问题追踪链条（Issue3→Issue5）入口缺失。
- **修复方案：** 由用户确认后统一回写：Build6 Step5/6 与 Build7 Step1~4 按最新修复状态恢复 ✅；Build7 Step6 明确标“部分收口/待 ProdTestList 人工验收”；AGENTS.md 文档清单补 Issue5（及 Build6-2）索引。
- **状态：** ◧ 待用户决策复核

### R20-16 素材池解析忽略 `bufio.Scanner` 错误，超长行会导致后续条目被误删

- **现象：** `backend/internal/pool/sync.go:445` `parseURLBody` 使用 `bufio.Scanner`（缓冲区 1MB），循环结束后未检查 `sc.Err()`。若响应中存在超过 1MB 的行，`Scan()` 会在该行中止且 `ErrTooLong`，函数仍把已解析前缀当成功结果返回。
- **根因：** 解析函数只统计 skip，没有把 Scanner 错误作为该 URL 失败处理。
- **影响范围：** 超长行之后的全部条目不会被解析；因该 URL 仍被标记为 `ok=true`，若所有 URL 都“成功”，差量删除会执行并删除源中仍存在、只是未被扫描到的 url 条目——静默丢规则。
- **修复方案：** `parseURLBody` 返回 Scanner 错误（或第三个状态），`syncURL` 将 `scanner 读取失败` 记为 URL 失败并保留旧数据；或放大缓冲区并把读取错误显式记入 `skip_reasons` 且不执行差量删除。
- **状态：** ☐ 待修复

### R20-17 高级模式关闭时下载渲染仍解密用户凭据，OFF 注释优先级可被 500 破坏

- **现象：** `backend/internal/server/render.go:42` 在判断 `advancedOn` 后仍无条件调用 `creds.Credentials`；若高级模式已关闭但密文损坏/签名密钥不一致导致解密失败，`renderUserSubscription` 直接返回 500。
- **根因：** 未按“OFF 优先于无凭据”的口径短路；关闭高级模式时本不应依赖用户凭据。
- **影响范围：** OFF 后装配模板下载在凭据异常时不可用；与 Design2 §一/§5.7 的 `# Xray 高级模式未启用` 注释语义冲突。
- **修复方案：** `!advancedOn` 时跳过 `Credentials` 解密，以空凭据/`hasCreds=false` 进入渲染分支；`advancedOn=true` 时才解密。
- **状态：** ☐ 待修复

### R20-18 单条补推/独立账号推送路径缺少凭据首建与 AddUser 后 OFF 补偿

- **现象：**
  - `backend/internal/xray/reconcile.go:317` `pushUserTarget` 只读 `Credentials`，不调用 `EnsureCredentials`；对账补推遇到“active 但尚无 UUID/代理密码”的用户会直接失败，无法像 `PushUser` 一样首建凭据。
  - `pushUserTarget` 在 `AddUser` 成功后没有复查 `advanced_mode` 并补偿 `RemoveUser`（对比 `sync.go:189-195` 的 `PushUser` 已有该逻辑）。
  - `backend/internal/xray/ext.go:759` `pushOne` 同样没有 AddUser 后 OFF 复查/补偿；`RetryExt`、`ResetExtQuota`、对账补推等经此入口的路径存在 OFF 竞态窗口。
- **根因：** Build6 约束“全部 AddUser 类钩子统一先 `EnsureCredentials`，AddUser 完成后复查 off 并补偿”只落到 `PushUser`，单条/独立账号入口漏改。
- **影响范围：** 对账“补推”无法为首建用户开通账号；OFF 提交前已发出的单条/独立账号 AddUser 在 OFF 后可能残留 Xray 孤儿账号。
- **修复方案：** `pushUserTarget` 在 quota 检查后调用 `EnsureCredentials`（`ErrAdvancedOff` 时静默跳过）；`pushUserTarget`/`ext.pushOne` 在 `AddUser` 成功后实时查 `advanced_mode`，为 off 时立即 `RemoveUser` 并记 warn/状态。独立账号路径还应先写 pending（事务内复查 advanced_mode）。
- **状态：** ☐ 待修复

### R20-19 配置导入保护可被“signing_key 缺失”与 v1 兼容路径绕过

- **现象：**
  - `backend/internal/config/export.go:554` `checkImportProtection` 在 payload 中 `signing_key` 为空时直接放行；空 key 导入后当前库密钥被清除（严格整体覆盖），但已有业务密文仍留在 users/nodes/xray_ext_accounts，之后全部不可解密。
  - `export.go:284-289` 对 format_version≠2 的旧文件走 `Import()` 同步兼容分支，该分支完全不调用 `checkImportProtection`；若当前库已有 UUID/代理密码/节点凭据密文，v1 文件携带不同 signing_key 也会造成同样不可逆损坏。
- **根因：** 保护条件只覆盖“非空且不同”的 v2 密钥变化，遗漏“空密钥”与 v1 路径。
- **影响范围：** 误导入旧版/缺密钥配置后用户凭据与节点凭据永久无法解密；与 Build7 Step2/Design2 §5.4“密钥将变化且存在业务密文时拒绝整个导入”不符。
- **修复方案：** 保护判定改为 `newKey != currentKey`（含 newKey 为空）且存在业务密文即拒绝；v1 兼容导入也复用同一保护检查后再覆盖。
- **状态：** ☐ 待修复

### R20-20 Xray 实例页仍缺多项 Design2-UI §8 / Build7 Step3 交互分支

- **现象：** `frontend/src/views/admin/XrayInstancesView.vue`：
  - 实例列表空态只有「新增实例」，缺前置提示「需先在 Xray 服务器开启 gRPC API 与流量统计（policy.stats）」；独立账号空态缺用途说明「用于向面板账号体系之外的人员/场景分发凭据（可手写入自定义订阅内容）」。
  - `doDetect`（:191）四项全 0 时虽 `Notify.info('节点无变化')`，但仍保留 `detectResult` 并打开回执 Modal（Build7 Step3 要求“不弹回执”）。
  - 对账四分区全部为空时没有 `a-result`「账号已一致，无需处理」成功态。
  - 独立账号创建：手填接管 UUID/代理密码使用普通 `Input` 而非 `a-input-password`；自动生成模式创建前没有“Xray 侧已存在同 email 时先移除旧账号并踢除”的危险确认。
  - 独立账号编辑弹窗只有名称/配额/推送目标，缺少凭据区双轨（`a-input-password`，编辑留空=保留原凭据）；后端 `PUT /api/admin/xray/ext/:id` 虽接受 `uuid/proxy_secret`，但 UI 无法使用。
  - 推送目标 `targetOptions` 只过滤节点 enabled/allocatable/missing，未过滤实例 `enabled`；停用实例的节点仍可勾选，保存被后端 400。
- **根因：** Build7 Step3 在 R17-04 修复后仍以“主要路径”为主，空态/边界/凭据编辑分支未完全按 UI §8.5/§10.2 落地。
- **影响范围：** 部署指引缺失；零变化误弹回执；空对账无成功反馈；独立账号凭据输入泄露风险与编辑能力缺失；停用实例目标误选。
- **修复方案：** 按 Build7 Step3/UI §8 逐项补齐上述分支；`targetOptions` 需后端列表附带 `instance_enabled`（或前端合并 instances 过滤）。
- **状态：** ☐ 待修复

### R20-21 删除不存在实例的异步任务返回成功，而非 404

- **现象：** `backend/internal/xray/instance.go:211` `DeleteAsync` 只登记任务，不校验实例存在；`deleteAndClean` 对不存在 id 执行 `DELETE` 后 RowsAffected=0 仍视为成功，任务终态 `succeeded`。`server/xray.go` delete handler 因此永远走不到 404 分支。
- **根因：** 异步化时把存在性校验遗漏在任务外/任务内。
- **影响范围：** 对已删除/错误 id 的 DELETE 返回 task_id 并最终“成功”，与 Build6 Step1 的 404 错误契约不符，前端误报成功。
- **修复方案：** `DeleteAsync` 在 `Register` 前用事务或查询校验存在性，不存在返回 `ErrInstanceNotFound` 由接入层映射 404；任务体内也保留 RowsAffected=0 时置 failed 兜底。
- **状态：** ☐ 待修复

### R20-22 装配 preview/generate 将内部数据库错误统一映射为 400

- **现象：** `backend/internal/server/assembly.go:82/102` 对 `Preview`/`Render` 的所有 error 一律 `Fail(400)`，包括 `loadData` 的 SQL 读取失败、JSON 损坏、签名密钥缺失等内部错误；`resolveOwner` 的 DB 错误也被映射 400。
- **根因：** 校验错误与内部错误未在业务层区分哨兵。
- **影响范围：** 500 语义被破坏、SQL/内部错误文案可能泄露给客户端；错误码不符合 AGENTS §4.8。
- **修复方案：** 为 `assembly` 增加可识别校验错误（如包装 `ErrBadRequest`）的哨兵；handler 用 `errors.Is/As` 仅将校验/目标缺失类映射 400/404，其余映射 500（debug 模式下再透出详情）。
- **状态：** ☐ 待修复

### R20-23 候选集 JSON 解析失败被静默当空集，重算会误删全部分配

- **现象：** `backend/internal/group/group.go:375` 与 `backend/internal/xray/sync.go:487` 解析 `assembly_blueprints.selection_json` 失败时 `continue` 静默跳过；`RecomputeCandidateSet` 因此可能得到空候选集，进而把全部 `group_nodes` 判为越界删除。
- **根因：** 蓝图数据损坏被当成“该蓝图无候选节点”，未区分“无候选”与“解析失败”。
- **影响范围：** 单条蓝图 JSON 损坏（或历史格式异常）即可在下次节点/版本/平台变更时清空所有组节点分配，并触发批量 RemoveUser。
- **修复方案：** 解析失败时返回错误（fail closed），或在重算时把解析失败的蓝图标记为异常并跳过删除；至少 `log.Warn` 且不得让该蓝图把候选集降为空。
- **状态：** ☐ 待修复

### R20-24 第二轮 error 处理残余（AGENTS「所有 error 必须处理」）

- **现象：**
  - `backend/internal/custom/custom.go:76` `id, _ = res.LastInsertId()`：失败时 `c.ID=0`，后续 `CreateVersion` 失败后的回滚删除按 id=0 执行，可能遗留孤儿 custom_subscriptions 行。
  - `backend/internal/rule/rule.go:86` `id, _ = res.LastInsertId()`：错误被吞，靠后续外键失败间接回滚，错误原因被掩盖。
  - `backend/internal/share/share.go:88` 回滚路径首个 DELETE 用 `_, _ =`，忽略 share_tokens 清理失败。
  - `backend/internal/xray/sync.go:192` OFF 补偿 `RemoveUser` 结果被忽略：补偿失败时只写 failed 状态，Xray 侧孤儿账号无日志。
  - `backend/internal/xray/ext.go:782` generate 覆盖前 `GetInboundUsers` 探测失败静默（随后 AddUser already exists 才走二次移除）。
  - `backend/internal/server/subscription.go` 的 `checkSlug` 与 `backend/internal/server/custom.go` 的 `ParseInt` 失败被忽略，非法 id 被当作 0 处理。
- **根因：** 第二轮扫描补充；多数为“罕见失败/清理路径”未按约束显式处理或记录。
- **影响范围：** 清理/回滚不彻底、补偿失败不可观测、错误原因被掩盖。
- **修复方案：** 逐处显式处理：LastInsertId/ParseInt 失败返回错误；回滚删除与补偿 RemoveUser 失败记 warn/error；ext 覆盖前探测失败记 warn 或直接走 already-exists 覆盖分支。
- **状态：** ☐ 待修复

### R20-25 Design2-UI 未同步 Issue4 已落地的装配信息架构变更

- **现象：** Issue4 R19-02/R19-08 已将装配页改为“规则素材池 / 代理组 / 构建订阅·规则”三个一级 Tab，并移除 `/admin/proxy-groups` 独立路由与侧边栏入口；但 `Design2-UI.md` §2.3/§5.1/§7 仍写五页签与独立代理组路由，当前代码与设计文档不一致。
- **根因：** 修复只回写 Issue4，未按 AGENTS §3.6 同步 Design2-UI。
- **影响范围：** 后续会话按 Design2-UI 恢复开发时会“回退”已被用户否决的交互结构。
- **修复方案：** 更新 Design2-UI §2.3 路由表、§5.1 页签结构与 §7 入口说明（代理组并入订阅装配 Tab），变更记录注明 R19-02/R19-08 决策。
- **状态：** ◧ 待用户决策复核（代码与 Issue4 已一致，仅设计文档回写需要用户确认时机）

### R20-26 输入校验缺口：api_addr 端口不合法与重复推送目标/节点导致 500

- **现象：**
  - `backend/internal/xray/client.go:66` `ValidateAddr` 只做 `net.SplitHostPort` 与空值检查；`host:abc`、`host:99999` 均通过校验。
  - `ExtService.CreateExt/UpdateExt` 对 `push_targets` 不去重，重复 `(instance_id, inbound_tag)` 触发 `xray_ext_users` PK 冲突，错误返回 500 而非 400。
  - `group.Service.SetNodes` 对 `node_ids` 不去重，重复节点触发 `group_nodes` PK 冲突，错误返回 500 而非 400。
- **根因：** 参数规范化/去重校验未覆盖边界输入。
- **影响范围：** 错误码与 AGENTS §4.8 不符；异常客户端可制造 500。
- **修复方案：** `ValidateAddr` 校验端口为 1~65535 的数字；`CreateExt/UpdateExt` 与 `SetNodes` 先按目标键/节点 ID 去重，重复项返回 400 并指明重复项（是否静默去重由用户确认）。
- **状态：** ☐ 待修复

### R20-27 独立账号编辑凭据变更后未重推不变目标（推测，待用户复核）

- **现象：** `backend/internal/xray/ext.go` `UpdateExt` 接受 `uuid/proxy_secret` 并加密落库，但提交后只对 `oldMap-newMap` 新增目标 `pushOne`，对仍保留的目标不做 Remove+Add；若管理员修改凭据且目标不变，Xray 侧仍是旧凭据。当前前端编辑弹窗也没有凭据区（见 R20-20）。
- **根因/影响：** Build7 Step1 只字面写“凭据留空保留、push_targets 全量 diff”，未明确“非空修改凭据是否重推不变目标”；Design2-UI §8.5 又明确编辑弹窗有凭据区，因此按设计意图推测应重推，但需用户确认。
- **修复方案（推测）：** `UpdateExt` 在 `uuidVal` 或 `proxySecret` 非空时，对全部保留目标执行“先 Remove 再 Add”；或在 UI 隐藏凭据编辑并文档化“创建后凭据不可改”。请用户二选一。
- **状态：** ◧ 待用户决策复核

---

## 附、待用户再次决策复核项（已决策，见变更记录 v2.0）

1. **R20-08**：已决策为“30 分钟整体超时 + 取消端点 + 前端取消按钮”。
2. **R20-11**：已决策为“环境不可用，未能复现；转 ProdTestList 人工验证”。
3. **R20-15**：已决策为“Build6/7 勾选、AGENTS 文档清单统一回写”。
4. **R20-25**：已决策为“Design2-UI 装配页结构按 Issue4 回写”。
5. **R20-27**：已决策为“凭据变更后对所有保留目标 Remove+Add 重推”。
6. 其余 R20-01~07/09~14/16~24/26 均为确定性问题，已按 Build8 方案修复。

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
| v1.0 | 2026-08-22 | 初始版本：Build1~7 + Issue1~4 双重静态核验。确认 Build4/5 主体与 Build6/7 后端主体已落地；确认 Issue3 R17-06/07、Issue4 R19-05 补充现象未完全闭环；新增 R20-01~R20-15（error 处理、死代码、装配参数校验、前端预检缺口、README/文档状态不一致等）。未修改代码，仅记录。 |
| v1.1 | 2026-08-22 | 二轮逐文件复扫：复核 R20-01~15 全部仍成立；R20-04 增补死代码 `nextURLOrderTx`；R20-15 扩为 Build6/7 多处勾选与归档不一致；新增 R20-16~R20-27（Scanner 误删、OFF 下载解密、AddUser 首建/补偿、导入保护绕过、Xray 页 UI 分支、实例删除 404、装配错误码、候选集 fail-open、error 残余、Design2-UI/AGENTS 文档同步、参数校验、ext 凭据编辑重推待决策）。未修改代码。 |
| v2.0 | 2026-08-22 | 用户决策已确认（R20-08 超时+取消、R20-11 转人工、R20-15/25 文档统一回写、R20-27 凭据重推、R20-26 重复项 400、R20-09 完整测试、R20-10 一键 Production smoke、R20-19 严格导入保护、先写 Build8）。已创建 [Build8.md](./Build8.md) 并完成 Step1~4 代码/前端/文档/测试；后端 build/vet/test 全绿，前端 65 例单测通过。 |
