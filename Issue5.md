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
  - 本轮新发现若干 **error 处理、死代码、参数校验、文档/README 缺口**，记录为 R20 系列。

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

### R20-04 死代码：`SyncAllActive`、`SyncAllExt`、`InstanceService.CloseAll` 无调用方

- **现象：**
  - `backend/internal/xray/sync.go:310` `SyncAllActive` 全库无调用。
  - `backend/internal/xray/ext.go:636` `SyncAllExt` 全库无调用。
  - `backend/internal/xray/instance.go:78` `CloseAll` 全库无调用（服务停机未关闭缓存 gRPC 连接）。
- **根因：** 精确 diff 与异步任务改造后，旧的全量同步入口未被清理；`CloseAll` 预留后未接线。
- **影响范围：** 死代码增加维护成本；gRPC 连接在进程退出时由 OS 回收，但优雅退出未显式关闭。
- **修复方案：** 删除无调用方的 `SyncAllActive`/`SyncAllExt`（或补充调用）；在 `Server.Run` 优雅退出或 `main` defer 中调用 `InstanceService.CloseAll`。
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
- **状态：** ◧ 进行中（与 Issue3 R17-06 一致）

### R20-10 Issue3 R17-07 smoke 脚本 Production 自动拉起仍未闭环

- **现象：** `.smoke-test.sh` 已改为非 Production 直接退出并要求手动验证，但未自动拉起 Production 临时实例；且当前 smoke 仅覆盖 Clash YAML 装配，未覆盖 SR subs / generic-subs / sr-conf / 导入 v2 完整往返。
- **影响范围：** Build7 Step6 的端到端验收仍依赖人工；CI/自动化不能完整拦截。
- **修复方案：** 按 R17-07 计划增加 Production 临时实例自动拉起（或提供明确的一键脚本），并扩展四类装配器路径。
- **状态：** ◧ 进行中（与 Issue3 R17-07 一致）

### R20-11 Issue4 R19-05 “重启后平台数据疑似丢失”仍未现场排查

- **现象：** Issue4 R19-05 状态虽标 ✅，但其补充现象“重启后平台数据丢失/平台列表为空”仍注明“需现场数据库/WAL/数据卷排查”；当前代码静态分析无法确认该现象已消除。
- **影响范围：** 若真实存在数据卷/WAL 异常，可能导致用户数据丢失。
- **修复方案：** 需要用户提供现场数据库/WAL/数据卷状态，或按部署环境做数据持久化验证（非本静态分析范围）。
- **状态：** ◧ 待用户复核/现场排查

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

### R20-15 Build7 Step6 文档状态与归档状态不一致

- **现象：** `docs/AchievedDocuments/Build7.md` 的 TODOLIST 与进度表中 Step6 仍为 ◧；但 AGENTS.md 已把 Build4~7 列为“已存档”，Issue3 R15-13 也标 ✅“已执行”。文档之间存在状态不一致。
- **影响范围：** 新会话可能误判 Build7 已全量交付。
- **修复方案：** 由用户确认是否将 Build7 Step6 恢复 ✅/标注“部分收口”，或回写文档说明 R17-06/07 未闭环前不视为完成。
- **状态：** ◧ 待用户决策复核

---

## 附、待用户再次决策复核项（推测/模糊点，未直接改码）

1. **R20-08**：素材池同步是否要增加整体超时/取消，还是接受“独立连接 + 批量写入”作为最终方案？
2. **R20-11**：R19-05 数据丢失现象是否已在实际部署环境排除？是否需要现场数据卷/WAL 检查？
3. **R20-15**：Build7 Step6 的文档状态如何定稿？
4. 其余 R20-01~07/09~14 均为确定性问题，建议按上述方案修复。

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
