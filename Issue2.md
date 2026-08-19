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
- **修复方案（待实施）：** 在差量删除时按本次成功 URL 并集统计实际删除行数并写入对应 `PerURLResult.Removed`；`parseURLBody` 对多余逗号段行计入 `skipped`，并在条件允许时记录 skip 原因摘要。
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
