# Build6-2.md — Build6 未完成/简化项补充构建记录

> **文档定位：** 本文档记录 Build6 主流程已验收但未完全达到 Build6.md 原始深度要求的内容，作为后续补强与 Build7 衔接的追踪清单。
> - 关联文档：[Build6.md](./Build6.md)、[Design2.md](./Design2.md)、[Design2-UI.md](./Design2-UI.md)
> - 编码指令：[AGENTS.md](./AGENTS.md)（**唯一强要求**）

---

## 一、背景

Build6 已按步骤完成并通过现有 `go build ./...`、`go vet ./...`、`go test ./...` 与前端 `npm run build`。  
但在实现过程中，部分 Build6.md 中描述的“深度语义”采用了可运行的最小实现，尚未完全达到文档原始要求。以下内容不属于 Build7 已规划范围，或属于 Build6 内部应补齐但当前未闭环的部分。

---

## 二、未完成/简化项清单

| # | 关联 Build6 Step | 未完成/简化内容 | 当前状态 | 建议补强位置 |
|---|------------------|----------------|----------|--------------|
| 1 | Step 3 | 用户删除、审批拒绝路径的 Xray 清理未完全按“期望集”收集；当前主要依赖 `ReconcileUser` 与后续对账兜底 | ✅ 已完成（本轮补强） | Build6-2 |
| 2 | Step 3 | 组删除、组节点变更、节点 missing/allocatable 变化等触发器的 diff 推送未逐条精确实现，当前以 `SyncAllActive` / `ReconcileUser` 做整体收敛 | ✅ 已完成（本轮补强） | Build6-2 |
| 3 | Step 4 | Clash 下载动态渲染未完整实现“render_plan 自包含 + 全量重渲染 + 组可达性递归重建 + 强制组降级 + rules 降级”的完整逻辑；当前为占位替换 + 基础行注入 | ✅ 已完成（本轮补强） | Build6-2 |
| 4 | Step 4 | `assembly/links` 共享子包抽取未完成；当前通过 `assembly.RenderLink` 导出函数复用，未按 Build6.md 要求拆分为独立子包 | ✅ 已完成（本轮补强） | Build6-2 |
| 5 | Step 5 | 采集任务已接入，但独立账号采集与配额检查未在本 Build 实现（原规划归 Build7 Step1） | 📋 已整理进 Build7 Step1 | Build7 Step1 |
| 6 | Step 6 | 假 Xray 集成测试仅覆盖 gRPC 客户端核心链路（AddUser/RemoveUser/ListInbounds/QueryStats），未完整跑通文档中的端到端剧本 | ✅ 已完成（本轮补强） | Build6-2 |
| 7 | Step 1/3 | 实例删除、节点删除的清理目标收集仍以既有 `xray_users` / `xray_ext_users` 为主，未完全升级为“受影响 active 用户 × 节点/实例”期望集口径 | ✅ 已完成（本轮补强） | Build6-2 |

---

## 二-A、本轮补强完成说明（2026-08-20）

- **#1 用户删除/审批拒绝清理**：在 `AdminService.Delete` 与 `approval.Service.Reject` 中新增“删除前收集当前期望目标、删除后执行 RemoveUser”的钩子；`server.go` 已接线到 `syncSvc.Targets` / `RemoveUserFromTargets`。配套单测：`TestAdminDeleteCallsXrayCleanupHooks`、`TestRejectCallsXrayCleanupHooks`。
- **#2 精确 diff**：`group.Service.Delete` 现在会收集受影响 active 用户并在提交后回调；`server.go` 接通 `groupSvc.SetOnNodesChanged`；节点启停/公共/检测可见性变化改为 `SyncUsersForNodes`（仅同步受影响的 active 用户），并修复检测中 `missing 1→0` 的 `recovered` 回调遗漏。配套单测：`TestGroupDeleteNotifiesAffectedUsers`、`TestSetNodesNotifiesAffectedUsers`。
- **#3 Clash 全量重渲染**：`assembly/render_clash.go` 的 `render_plan_json` 升级为自包含 `ClashPlan`（manual 完整条目、组结构、冻结规则、fallback）；新增 `assembly.RenderClashPlan` 实现可达性递归重建、普通组删除、强制组降级、rules 降级、无凭据/OFF 注释。`server/render.go` 的 Clash 分支改为调用该函数，并对旧版非自包含 plan 保留占位替换兜底。配套单测：`clash_plan_test.go`。
- **#4 链接子包抽取**：新增 `backend/internal/assembly/links` 共享子包，导出 `links.Render`；`assembly/links.go` 改为薄封装调用子包，`server/render.go` 仍复用 `assembly.RenderLink`（内部已委托子包）。
- **#6 假 Xray 端到端剧本**：扩展 fake gRPC 为 vless + shadowsocks 两个 inbound，并支持可复位流量计数器；新增 `TestBuild6EndToEndFakeXray` 覆盖“建实例→检测→PushUser→移除节点 Reconcile→CollectInstance 流量累加”。
- **#7 删除清理期望集**：实例删除在既有 `xray_users`/`xray_ext_users` 之外，按“active 用户 × 候选集内节点”收集期望清理目标；节点删除额外按“active 用户 × group_nodes/公共标记”收集期望清理目标。

**本轮推断/标注**：
- Clash 旧版蓝图（非自包含 `render_plan_json`）无法无损升级为全量重渲染，本轮保留旧路径的占位替换兜底；新建/重新生成蓝图后即为自包含计划。
- 节点删除时即使节点已 `missing=1`，只要仍存在 `group_nodes`/公共标记分配，也纳入清理目标，以避免 Xray 侧残留。
- 独立账号采集与配额检查（#5）不在本轮实现，已按用户要求整理进 Build7 Step1（含承接说明与验收口径）。

---

## 三、建议补强顺序

1. **先补 Step 4 Clash 全量重渲染**：这是下载语义的核心，影响用户实际订阅内容。
2. **再补 Step 3 删除/禁用/组变更的精确 diff**：避免依赖全量 Reconcile 带来的多余调用。
3. **然后补 Step 1/3 删除清理期望集**：与对账逻辑闭环。
4. **最后补 Step 6 端到端剧本与 Step 4 链接子包抽取**：作为质量收口。

---

## 四、验收口径

以下补强项完成时应满足：

- 对应 Build6.md 原始 Step 的“测试与验收命令”全部通过。
- 新增/修改逻辑均配套单元测试。
- 不破坏当前已通过的 `go build ./...`、`go vet ./...`、`go test ./...`、`npm run build`。
- 若涉及 Design2 行为变更，需先与用户确认后再实施。

**本轮验收（2026-08-20）已执行**：
- `cd backend && go build ./... && go vet ./... && go test ./...` 全部通过。
- `cd frontend && npm run build` 已执行并通过（本轮未改前端，确认后端契约变更未破坏 Build5 产物）。
- `cd backend && go test -race ./internal/xray/... ./internal/download/... ./internal/group/...` 已执行并通过。

---

## 五、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-20 | 初始版本：记录 Build6 主流程中未完成/简化项，作为后续补强与 Build7 衔接清单 |
| v1.1 | 2026-08-20 | 完成 #1/#2/#3/#4/#6/#7 补强；#5 继续按规划延后至 Build7 Step1；补充单测与端到端 fake 剧本 |
| v1.2 | 2026-08-20 | 按用户要求将 #5 需要实现的内容整理进 Build7 Step1，并更新状态为“已整理进 Build7 Step1” |
