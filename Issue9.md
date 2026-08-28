# Issue9.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考），承接已归档/历史问题记录（Issue1～Issue8，见 `docs/reports/Issue/`）。
> 设计记录见 [Design2.md](Design2.md) 与 [Design2-UI.md](Design2-UI.md)；构建方案见 [Build11.md](Build11.md)、[Build12.md](Build12.md)；编码指令见 [AGENTS.md](AGENTS.md)（**唯一强要求**）。

---

## 〇、本轮说明

- **创建时间：** 2026-08-28
- **创建背景：** 在 Build1～Build12 全量核验中发现 Build11 新增管理员概览相关后端存在可复现死锁；已完成源码研究，尚未修改任何业务代码。
- **当前状态：** 已记录 R24-01，完成根因与修复方向研究，等待确认后实施修复。

---

## 一、进行中问题

### R24-01 Build11 `/api/admin/overview` 及独立账号相关 SQLite 单连接嵌套查询死锁

- **现象：**
  - 执行 `cd backend && go test -timeout 120s ./internal/server/...` 时，`TestOverviewEndpointAggregatesStableData` 超时并 panic：
    ```text
    panic: test timed out after 2m0s
    running tests: TestOverviewEndpointAggregatesStableData
    ```
  - 高级模式开启且存在独立账号时，`/api/admin/overview` 可能一直不返回，前端管理员概览页持续加载。
  - 进一步静态扫描发现以下路径同样存在同类风险：
    - `/api/admin/xray/ext` 独立账号列表；
    - 定时采集 `runXrayCollect` 中的独立账号配额检查；
    - 用户超限时 `PushUser` 内部写入超限原因的路径。
- **根因：**
  - `backend/internal/store/store.go` 中 `Open` 使用 `db.SetMaxOpenConns(1)`，即 SQLite 单连接模型。
  - 在 `for rows.Next()` 尚未结束、外层 `*sql.Rows` 仍占用唯一连接时，再次通过同一个 `*sql.DB` 发起查询/写入，会导致内层操作等待唯一连接，而外层游标又不会结束，形成死锁。
  - 当前确认存在该模式的具体代码：
    1. `backend/internal/xray/ext.go` 的 `ExtService.ListExt`：
       - 外层查询 `xray_ext_accounts` 后，在循环内调用 `pushTargetsFor`（再次查询）和流量汇总查询。
    2. `backend/internal/xray/ext.go` 的 `ExtService.CheckAllExtQuota`：
       - 外层查询独立账号 ID 后，在循环内调用 `CheckExtQuota`，而 `CheckExtQuota` 内部继续执行多个数据库查询/写入。
    3. `backend/internal/xray/sync.go` 的 `SyncService.writeQuotaExceededError`：
       - 外层查询 `xray_users` 后，在循环内逐行执行 `ExecContext` 更新。
- **影响范围：**
  - `/api/admin/overview`：高级模式 + 独立账号场景下管理员首页概览可能挂起。
  - `/api/admin/xray/ext`：存在独立账号时列表接口可能挂起。
  - 定时任务：`CheckAllExtQuota` 可能让采集任务卡死。
  - 用户超限路径：`writeQuotaExceededError` 可能导致推送/配额处理卡死。
  - 当前后端 `go test ./...` 不能全绿，阻塞 Build11 的完整后端验收结论。
- **修复方案（推荐，尚未实施）：**
  - 总体原则：**先读完外层 `Rows` 并关闭游标，再进行任何补充数据库操作**。仓库内 `subscription.Service.List`、`user.AdminService.List`、`home.Service.ListPlatforms`、`config.ExportService.exportInstances` / `exportAccounts` 已采用该正确范式，可作为参照。
  - **`ExtService.ListExt`：**
    1. 先读取全部独立账号基础行到内层 slice；
    2. 调用 `rows.Close()`；
    3. 再遍历 slice，分别调用 `pushTargetsFor` 与流量汇总查询。
    4. 可选优化：将推送目标和流量改为批量 `IN` 查询或 JOIN 聚合，减少 N+1 查询。
  - **`ExtService.CheckAllExtQuota`：**
    1. 先读取全部独立账号 ID 到 slice；
    2. 关闭游标；
    3. 再逐个调用 `CheckExtQuota`。
  - **`SyncService.writeQuotaExceededError`：**
    1. 优先改为单条 SQL 批量更新：
       ```sql
       UPDATE xray_users
       SET sync_status = 'failed',
           last_error = '已超限，请先重置配额',
           updated_at = CURRENT_TIMESTAMP
       WHERE user_id = ?
       ```
    2. 如保留逐条处理，也必须先读取目标到内存、关闭游标后再执行。
  - **测试建议：**
    - 为 `ListExt`、`CheckAllExtQuota`、`writeQuotaExceededError` 增加“非空数据”单测；
    - 增加覆盖“高级模式开启 + 存在独立账号 + 调用 `/api/admin/overview`”的回归测试；
    - 修复后执行：
      ```bash
      cd backend && go build ./... && go vet ./... && go test ./...
      ```
- **涉及文件：**
  - `backend/internal/xray/ext.go`
  - `backend/internal/xray/sync.go`
  - `backend/internal/server/overview.go`
  - `backend/internal/server/overview_test.go`
  - `backend/internal/store/store.go`（理解约束，不必然修改）
- **状态：** ◧ 研究完成，待用户确认后实施修复（未修改任何代码）

---

## 二、已修复/已闭环问题

暂无。

---

## 三、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-28 | 初始版本：记录 Build11 后端 SQLite 单连接嵌套查询死锁的现象、根因、影响范围与推荐修复方案；仅研究，未改动代码。 |
