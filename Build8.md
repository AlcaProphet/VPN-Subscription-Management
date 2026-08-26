# Build8.md — VPN 订阅管理系统 当前构建方案（Issue5 R20 修复）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前构建方案**，承接已存档 Build4~7，针对 [Issue5.md](Issue5.md) 中 R20-01~R20-27 的修复执行。
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 问题追踪：[Issue5.md](Issue5.md)（历史 Issue1~4 已归档至 [docs/reports/](docs/reports)）
> - 用户决策（2026-08-22）：
>   1. R20-08：增加 30 分钟整体超时 + 取消端点 + 前端取消按钮；
>   2. R20-11：环境已不可用，按“未能复现”闭环并转 ProdTestList 人工验证；
>   3. R20-15/R20-25：Build6/7 勾选、AGENTS 文档清单、Design2-UI 装配结构统一回写；
>   4. R20-27：独立账号编辑凭据变更后对所有保留目标 Remove+Add 重推；
>   5. R20-26：重复 push_targets / node_ids 返回 400 并指出重复项；
>   6. R20-09：按 R17-06 原始清单完整补齐前端专项测试；
>   7. R20-10：新增一键脚本自动拉起临时 Production 容器，跑四类装配器 + v2 导出/导入往返；
>   8. R20-19：v1/v2、含空 signing_key 全部启用导入保护；
>   9. 先写本 Build8，按批次 1→2→3→4 执行。

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 0 | 创建 Build8 文档与决策回写 | ✅ 验收通过 |
| 1 | 数据安全/一致性：R20-16/23/19/05/21/22/12 | ✅ 验收通过 |
| 2 | Xray/高级模式后端语义与错误处理：R20-02/17/18/03/01/07/13/24/04/26/27 | ✅ 验收通过 |
| 3 | 素材池同步超时/取消：R20-08 | ✅ 验收通过 |
| 4 | 前端/测试/文档：R20-06/20/09/10/14/15/25 | ✅ 验收通过 |
| 5 | R20-11 状态闭环与 ProdTestList 补充 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/pool/sync.go`、`backend/internal/group/group.go`、`backend/internal/xray/sync.go`、`backend/internal/config/export.go`、`backend/internal/assembly/{validate.go,load.go,service.go,rebind.go}`、`backend/internal/server/assembly.go`、`backend/internal/xray/instance.go`、`internal/server/xray.go` 及对应测试 | Scanner 防误删、候选集 fail-closed、导入保护、装配校验/错误码、实例删除 404 |
| 2 | `backend/internal/xray/{reconcile.go,credentials.go,sync.go,ext.go,offclear.go,detect.go,stats.go,instance.go}`、`backend/internal/custom/custom.go`、`backend/internal/rule/rule.go`、`backend/internal/share/share.go`、`backend/internal/server/{subscription.go,custom.go,settings.go,render.go,server.go}` 及测试 | 凭据首建/OFF 补偿、OFF 下载短路、错误处理、死代码、参数校验、ext 凭据重推 |
| 3 | `backend/internal/pool/{sync.go,pool.go}`、`backend/internal/server/pool.go`、`frontend/src/api/pool.ts`、`frontend/src/views/admin/assembly/PoolTab.vue`、测试 | 同步超时/取消 |
| 4 | `frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/XrayInstancesView.vue`、`frontend/tests/*.spec.ts`、`.smoke-test-prod.sh`、`.smoke-test.sh`、`README.md`、`docs/reports/Build/Build6.md`、`docs/reports/Build/Build7.md`、`AGENTS.md`、`Design2-UI.md`、`Issue5.md`、`ProdTestList.md` | 预检、Xray UI、测试、smoke、README/文档同步 |

---

## 三、分步构建计划

### Step 1：数据安全/一致性

- **目标：** 修复可能导致静默丢数据/错误码混淆的问题。
- **产出文件与操作：**
  - `backend/internal/pool/sync.go`：`parseURLBody` 返回 Scanner 错误；`syncURL` 将读取/解析错误标记为 URL 失败并保留旧数据。
  - `backend/internal/group/group.go`、`backend/internal/xray/sync.go`：候选集解析失败返回错误，避免空集误删。
  - `backend/internal/config/export.go`：`checkImportProtection` 改为 `newKey != currentKey`，v1 `Import` 与 v2 `ImportV2` 均调用。
  - `backend/internal/assembly/validate.go`、`load.go`：`GroupNodeOrders` 校验；缺失资源统一包 `ErrBadRequest`。
  - `backend/internal/server/assembly.go`：preview/generate/resolveOwner 按 `errors.Is` 映射 400/500。
  - `backend/internal/assembly/rebind.go`：render_plan 解析失败追加 hint。
  - `backend/internal/xray/instance.go`、`backend/internal/server/xray.go`：删除前存在性校验与任务内 RowsAffected 兜底。
- **测试与验收：**
  ```bash
  cd backend && go test ./internal/pool ./internal/group ./internal/xray ./internal/assembly ./internal/config ./internal/server
  ```
- **验收标准：** 新增单测覆盖：超长行不误删；损坏 selection_json 不删分配；v1/v2 导入保护；group_node_orders 非法 400；装配内部错误 500；删除不存在实例 404。

### Step 2：Xray/高级模式后端语义与错误处理

- **目标：** 消除凭据假成功、OFF 竞态、忽略 error 与死代码。
- **产出文件与操作：**
  - `reconcile.go`：`CredentialsOne` 用户分支检查 `failed>0`；`pushUserTarget` 补 `EnsureCredentials` 与 OFF 补偿。
  - `ext.go`：`pushOne` AddUser 成功后复查 advanced_mode 并补偿；`UpdateExt` 凭据变更重推保留目标；错误读取/移除补日志。
  - `render.go`：`!advancedOn` 跳过解密。
  - `offclear.go`、`settings.go`：确认词错误映射 400。
  - `detect.go`、`stats.go`、`sync.go`：补 warn。
  - `custom.go`、`rule.go`、`share.go`、`subscription.go`、`custom.go`：LastInsertId/ParseInt 错误处理。
  - `instance.go`、`server.go`：删除死代码并接线 `CloseAll`。
  - `client.go`、`ext.go`、`group.go`：端口校验与重复项 400。
- **测试与验收：**
  ```bash
  cd backend && go test ./internal/xray ./internal/custom ./internal/rule ./internal/share ./internal/server
  ```
- **验收标准：** 单测覆盖：CredentialsOne 不假成功；pushUserTarget 首建凭据；OFF 竞态补偿；OFF 下载不依赖解密；确认词 400；ext 凭据重推；重复项 400。

### Step 3：素材池同步超时/取消

- **目标：** 大列表同步不再无限挂起，用户可取消。
- **产出文件与操作：**
  - `pool/sync.go`、`pool/pool.go`：`context.WithTimeoutCause(30m)` + 可取消 map；`CancelSync(taskID)`；终态复用 `failed` 并写“任务已取消/超时”。
  - `server/pool.go`：新增 `POST /:id/sync/tasks/:taskId/cancel`。
  - `frontend/src/api/pool.ts`、`PoolTab.vue`：同步中显示“取消”按钮。
  - 测试：取消后终态 failed、旧数据保留、其他 API 可访问。
- **测试与验收：**
  ```bash
  cd backend && go test ./internal/pool ./internal/server
  cd frontend && npm test -- --run
  ```
- **验收标准：** 取消端点只允许取消 running 任务；超时/取消都落到 failed 终态。

### Step 4：前端、测试、文档、smoke

- **目标：** 补齐前端预检/UI、测试、README、文档同步、Production smoke。
- **产出文件与操作：**
  - `AssemblyView.vue`：预检增加目标平台已有订阅条目。
  - `XrayInstancesView.vue`：补齐 R20-20 列出的 UI 分支。
  - `frontend/tests/*`：按 R17-06 完整补齐。
  - `.smoke-test-prod.sh`：自动拉起临时 Production 容器跑四类装配器 + v2 往返。
  - `.smoke-test.sh`：补充四类装配器路径。
  - `README.md`：高级模式部署章节。
  - `docs/reports/Build/Build6.md`、`docs/reports/Build/Build7.md`、`AGENTS.md`、`Design2-UI.md`、`Issue5.md`、`ProdTestList.md`：状态与文档同步。
- **测试与验收：**
  ```bash
  cd frontend && npm run build && npm test -- --run
  bash .smoke-test-prod.sh
  ```
- **验收标准：** 前端构建/测试通过；smoke 全绿；文档状态与代码一致。

### Step 5：R20-11 闭环

- **目标：** 将“重启后平台数据丢失”按用户决策闭环。
- **操作：** Issue5 状态更新为“环境已不可用，未能复现；转 ProdTestList 人工验证”；ProdTestList 增加重启后数据完整性检查项。
- **验收：** 文档状态一致，无代码改动。

---

## 四、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-22 | 初始版本：基于 Issue5 R20 系列与用户 2026-08-22 决策创建。 |
