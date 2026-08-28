# ProjectWideReport2 — VPN 订阅管理系统 Build1～Build12 全量核验报告

> 核验日期：2026-08-28  
> 核验对象：`~/Desktop/Repo/VPN-Subscription-Management`（当前分支 `beta`，HEAD `4169860`）  
> 核验方式：结合 Build 文档进度、源码/测试文件存在性、Git 历史、后端/前端实际构建与测试进行全量核查。  
> 约束遵守：本次仅生成核验报告，未修改任何业务代码。  

---

## 一、结论摘要

**不能无条件宣布“Build1～Build12 已全部真实完成”。** 当前仍有一个阻塞点：

1. **当前后端全量测试存在可复现的挂起/失败**：`go test ./internal/server/...` 中 `TestOverviewEndpointAggregatesStableData` 超时（2 分钟 panic）。根因是 `ExtService.ListExt` 在持有 `rows` 游标时继续调用 `pushTargetsFor` 发起新的 SQL 查询，而 `store.Open` 设置了 `db.SetMaxOpenConns(1)`，单连接被外层游标占用后，内层查询永久等待，形成数据库连接死锁。该问题会直接影响 Build11 新增的 `/api/admin/overview`：当高级模式开启且存在独立账号时，概览接口会挂起。因此 Build11 的“后端验收通过”在本次实机核验中不成立。

已按用户决策处理：**Build7 Step6 已暂标记为完成**，文档进度表与 TODO 清单已更新为 ✅；剩余 Production 人工验收由用户依据 `ProdTestList.md` 后续执行。

其他核验结果：

- 后端 `go build ./...`、`go vet ./...` 通过。
- 后端除上述单个挂起测试外，其余包（含 `xray`、`assembly`、`uriparse`、`cron`、`version`、`server` 其它测试等）均通过；`go test -skip '^TestOverviewEndpointAggregatesStableData$' ./internal/server/...` 通过。
- 前端 `npm run build` 通过。
- 前端 `npm test -- --run` 通过：30 个测试文件、104 个用例全部通过。
- Build1～Build10、Build12 的主要功能代码均有真实落点，未发现“只有文档没有代码”的 Step；Build7 已按用户决策暂标记完成，人工验收转 ProdTestList。
- Build11 的 UI/交互/文档/前端测试基本落地，但受上述后端死锁影响，不能视为完整验收通过。
- AGENTS.md 的文档索引已补充 Build11/Build12 的“已完成、尚未归档”状态。

---

## 二、核验环境与方法

| 项目 | 值 |
|------|----|
| 仓库路径 | `~/Desktop/Repo/VPN-Subscription-Management` |
| Git 分支 | `beta` |
| Git HEAD | `4169860 完成 Build12：全量 gray/white 类迁移到 Token、新增全局浮层管理器与统一焦点管理、补充测试与文档` |
| 后端 | Go `go1.26.4 darwin/arm64`，module `vpn-sub` |
| 前端 | Node `v26.7.0`，npm `11.19.0`，Vue 3 + Vite + Tailwind |
| 容器 | 本次已实际执行 `docker compose build`，结果 PASS，已生成/更新 `vpn-subscription-management-vpn-sub:latest`；Production smoke 未在本轮重跑，沿用 `BuildReport2` 记录 |

已执行命令：

```bash
# 后端
cd backend && go build ./...          # PASS
cd backend && go vet ./...            # PASS
cd backend && go test ./...           # FAIL/挂起：server 包 TestOverviewEndpointAggregatesStableData
cd backend && go test -timeout 120s -skip '^TestOverviewEndpointAggregatesStableData$' ./internal/server/...  # PASS
cd backend && go test -timeout 120s ./internal/setup/... ./internal/share/... ./internal/slug/... ./internal/store/... ./internal/subscription/... ./internal/tasks/... ./internal/token/... ./internal/uriparse/... ./internal/user/... ./internal/version/... ./internal/xray/...  # PASS

# 前端
cd frontend && npm run build           # PASS
cd frontend && npm test -- --run       # PASS：30 files / 104 tests

# 容器
docker compose build                   # PASS（已生成 vpn-subscription-management-vpn-sub:latest）
```

源码/文档核验内容包括：

- 每个 Build 文档中的进度表、TODO 清单和变更记录；
- 关键后端包、路由、迁移文件、前端页面和组件是否存在；
- 旧分发模型是否已从业务代码移除；
- Git 提交历史中是否存在对应 Build 的完成记录；
- 已有验收报告（`BuildReport1`、`BuildReport2`、`ProjectWideReport1`、`Issue1~8`）的交叉佐证。

---

## 三、Build1～Build12 逐项核验

### 3.1 Build1 —— 工程骨架与认证闭环

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 1~7 均 ✅ 验收通过 |
| 核心落点 | `backend/internal/`（store/log/auth/setup/oidc/captcha/ratelimit/slug/token）、`backend/internal/server/`（auth/setup/status/health）、迁移 `0001~0005`、前端 Setup/Login/Register/OIDC 页面、Dockerfile/Compose |
| 实测/交叉 | 相关后端包测试通过；Git 历史存在 `feat(build1): 完成工程骨架与认证闭环（7 个 Step 全部验收通过）` |
| 判定 | ✅ 已真实落地 |

### 3.2 Build2 —— 订阅核心与用户端价值链

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 1~7 均 ✅ 验收通过 |
| 核心落点 | platform/version/subscription/token/custom/share/rule/home 等后端域；前端管理端订阅/分享/规则/平台/版本等页面；下载端点与 Token 体系 |
| 实测/交叉 | 相关后端包测试通过；Git 历史存在 `feat(build2): 完成管理端核心业务模块与数据迁移` |
| 判定 | ✅ 已真实落地 |

### 3.3 Build3 —— 管理面补全与运维能力

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 1~6 均 ✅ 验收通过 |
| 核心落点 | 用户管理/审批/邮件、面板配置、危险操作与导入导出/备份、访问日志与 SSE、应急恢复模式；对应后端包和前端页面均存在 |
| 实测/交叉 | 相关后端包测试通过；Git 历史存在 `feat(build3): 完成 Build3 六个核心功能模块开发` |
| 判定 | ✅ 已真实落地 |

### 3.4 Build4 —— Go 1.26 + 1009 迁移 + 旧分发拆除 + 规则素材池

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 0~7 均 ✅ 验收通过 |
| 核心落点 | `go.mod` 为 `go 1.26.0`；`migrations/1009_xray.sql` 存在；`migrations/1009_xray.sql` 中 DROP 旧表；业务代码中已无 `group_selections` / `subscription_group_rel` 引用；`pool` 包与素材池前后端均存在 |
| 实测/交叉 | `pool` 相关测试通过；Git 历史存在 `完成 Build4` |
| 判定 | ✅ 已真实落地 |

### 3.5 Build5 —— manual 节点、代理组、四类装配器与分发

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 1~7 均 ✅ 验收通过 |
| 核心落点 | `node/registry.go`、`proxygroup/proxygroup.go`、`assembly/render_clash.go`、`render_sr.go`、`server/assembly.go`、装配页前端 |
| 实测/交叉 | assembly/node/proxygroup/server 相关测试通过；Git 历史存在 `完成 Build 5 的构建` |
| 判定 | ✅ 已真实落地 |

### 3.6 Build6 —— Xray 对接后端核心

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 0~6 均 ✅ 验收通过；Build6-2 补强项也已补强或转入 Build7 |
| 核心落点 | `go.mod` 引入 `github.com/xtls/xray-core`；`internal/xray/`（client/instance/detect/sync/quota/stats/credentials）、`internal/tasks`、`server/xray.go`、`cron/xray.go` |
| 实测/交叉 | `xray`、`cron`、`tasks` 等测试通过；Git 历史存在 `完成 Build 6`、`完成 Build6-2` |
| 判定 | ✅ 已真实落地 |

### 3.7 Build7 —— 高级模式管理面与交付收口

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 1~6 ✅；Step 6 已按用户决策暂标记完成，Production 人工验收转 `ProdTestList.md` |
| 核心落点 | `xray/ext.go`、`xray/reconcile.go`、`xray/offclear.go`、`config/export.go` v2、`server/settings.go`、Xray/用户组/设置等前端 |
| 实测/交叉 | 相关后端包测试通过；功能代码基本落地；Step6 的最终人工验收由用户后续依据 ProdTestList 执行 |
| 判定 | ✅（按用户决策暂完成） |

### 3.8 Build8 —— Issue5 R20 系列修复

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 0~5 均 ✅ 验收通过 |
| 核心落点 | `pool/sync.go` 防误删、`config/export.go` 导入保护、`assembly/validate.go`、`xray/reconcile.go`、同步取消、前端补测、smoke 修复 |
| 实测/交叉 | `BuildReport2` 已对 Build8/9/10 做过后事验收；相关包测试通过 |
| 判定 | ✅ 已真实落地 |

### 3.9 Build9 —— 第一阶段基础扩展

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 0~6 均 ✅ 验收通过；Step 7~11 拆至 Build10 |
| 核心落点 | `assembly/goccy_yaml.go`、`selfcheck.go`、`download` RFC 5987、`node/registry.go`、`rulespec/spec.go`、`proxygroup` 扩展 |
| 实测/交叉 | 对应专项测试通过；`BuildReport2` 记录通过 |
| 判定 | ✅ 已真实落地 |

### 3.10 Build10 —— 第二阶段核心装配与收口

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 0~5 均 ✅ 验收通过 |
| 核心落点 | `assembly/overlay.go`、`uriparse/`、`node/uri_import.go`、`cron/pool.go`、`version/version.go` 原子写、前端 `OverlayStep.vue` |
| 实测/交叉 | overlay/uriparse/node/cron/version 测试通过；`BuildReport2` 记录 Production smoke 通过 |
| 判定 | ✅ 已真实落地 |

### 3.11 Build11 —— UI/UX 改进与管理员概览

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 0~7 文档均标记 ✅ 验收通过 |
| 核心落点 | 后端重置 token 三态、`/api/admin/overview`、版本归属接口、应急网关；前端可信状态修复、ContextBar、FormOverlay、PageShell/StateContainer、AdminOverviewView、DiffView/PreviewStep、uiTokens 等 |
| 实测/交叉 | 前端 build/test 通过；后端 `go build`/`go vet` 通过；**但后端全量测试被 `TestOverviewEndpointAggregatesStableData` 挂起阻塞** |
| 判定 | ⚠️ **功能代码基本落地，但不能判定为完整验收通过**：新增概览端点在高级模式 + 独立账号场景下会因 SQLite 单连接嵌套查询死锁 |

### 3.12 Build12 —— 全量 Token 化与全局浮层/焦点管理

| 项 | 结论 |
|----|------|
| 文档步骤 | Step 0~3 均 ✅ 验收通过 |
| 核心落点 | `frontend/src/utils/overlayManager.ts`、`AppDropdown/AppModal/AppSelect/AppRangePicker/AppTimePicker`、FormOverlay/ConfirmModal 接入、全量移除 `gray-*`/`bg-white` 等旧类 |
| 实测/交叉 | `frontend/src` 中旧 gray/white 类检索为 0；前端 104 个测试通过；Build12 文档记录与测试数一致 |
| 判定 | ✅ 前端侧已真实落地（本 Build 无后端改动） |

---

## 四、实测发现的主要问题

### 4.1 【阻塞】Build11 后端 SQLite 单连接嵌套查询死锁研究

- **直接现象**：`cd backend && go test -timeout 120s ./internal/server/...` 中 `TestOverviewEndpointAggregatesStableData` 超时，`panic: test timed out after 2m0s`。
- **问题记录**：已按 Issue 模板创建根目录 [Issue9.md](../../../Issue9.md)，状态为“研究完成，待实施修复”。
- **调用链**（从 panic 栈提取）：
  - `OverviewHandler.get`（`overview.go`）
  - → `ExtService.ListExt`（`ext.go:502`）
  - → `ExtService.pushTargetsFor`（`ext.go:722`）
  - → `database/sql.(*DB).conn` 等待连接
- **根因**：
  - `store.Open` 设置了 `db.SetMaxOpenConns(1)`（SQLite 单连接模型）。
  - `ExtService.ListExt` 先用 `QueryContext` 打开外层 `rows` 游标，再在 `for rows.Next()` 循环内调用 `pushTargetsFor` 和用量查询，这些函数/代码又用同一个 `*sql.DB` 发起新的查询。
  - 由于 MaxOpenConns=1，外层游标占用唯一连接，内层查询只能等待，而外层游标又不会结束，形成死锁。
- **同类风险（经源码扫描确认）**：
  - `ExtService.ListExt`：在 `xray_ext_accounts` 游标未关闭时嵌套查询推送目标/用量。
  - `ExtService.CheckAllExtQuota`：在 `xray_ext_accounts` ID 游标未关闭时调用 `CheckExtQuota`，后者会继续查询/写库。
  - `SyncService.writeQuotaExceededError`：在 `xray_users` 游标未关闭时逐行执行 `ExecContext` 更新。
- **影响面**：
  - `/api/admin/overview`：高级模式开启且存在独立账号时会挂起，管理员概览页可能一直加载。
  - `/api/admin/xray/ext` 列表端点：存在独立账号时同样可能挂起。
  - 定时采集 `runXrayCollect`：存在独立账号时调用 `CheckAllExtQuota` 可能挂起。
  - 用户超限路径 `PushUser`：调用 `writeQuotaExceededError` 时若存在 `xray_users` 记录可能挂起。
- **项目已有正确范式**：仓库内多个服务已经意识到该问题并采用“先读完外层 `Rows`、关闭游标，再进行补充查询”的模式，例如：
  - `subscription.Service.List`（显式 `rows.Close()` 后再查询内容形态）
  - `user.AdminService.List`（关闭后再填充高级模式字段）
  - `home.Service.ListPlatforms`（先读入内存再查询）
  - `config.ExportService.exportInstances` / `exportAccounts`（先读列表再逐项查询）
  这说明 `ExtService` 的现状属于遗漏，而不是需要推翻单连接模型的架构问题。
- **修复方向（本次仅研究，未改代码）**：
  1. `ListExt`：先收集基础行并关闭游标，再循环调用 `pushTargetsFor` 与用量查询；或改为批量 JOIN/IN 查询。
  2. `CheckAllExtQuota`：先收集 ID 列表并关闭游标，再逐个调用 `CheckExtQuota`。
  3. `writeQuotaExceededError`：可先收集目标并关闭游标；更优的是改成单条 `UPDATE xray_users SET ... WHERE user_id = ?`，不需要逐行循环。
  4. 补充覆盖“非空 `xray_ext_accounts` / `xray_users`”的单元测试，防止回归。

### 4.2 【已处理】Build7 Step6 已按用户决策暂标记完成

- `docs/reports/Build/Build7.md` 的 TODO 清单和进度表已更新为 ✅。
- 原“部分收口/待 ProdTestList 人工验收”中的 Production 人工验收项继续由用户依据 `ProdTestList.md` 后续执行，不再作为 Build7 文档层面的未完成项。

### 4.3 【已处理】AGENTS.md 已补充 Build11/Build12 状态

- 已按本次核验发现同步 `AGENTS.md`：顶部、§8.1 Build 文档行、§8.4 文档清单均补充 Build11/Build12“已完成、尚未归档”的状态。
- 当前仓库中 `Build11.md`、`Build12.md` 仍位于根目录，后续归档动作可由用户另行决定。

### 4.4 其他非阻断观察

- 前端测试存在少量 Vue Router / InputNumber 组件解析警告，但不影响测试结果（104/104 通过）。
- 后端除 4.1 外，其余包单独执行均通过；说明该问题是局部、确定的代码缺陷，而非环境性波动。
- 本次已重新执行 `docker compose build` 并通过；未重新执行 Production smoke，相关结论沿用 `BuildReport2` 的记录。

---

## 五、总体判定

| 范围 | 判定 |
|------|------|
| Build1～Build6 | ✅ 主要功能真实完成，当前代码与测试证据充足 |
| Build7 | ✅（按用户决策暂完成，Production 人工验收转 ProdTestList） |
| Build8～Build10 | ✅ BuildReport2 已事后验收，当前代码/专项测试佐证存在 |
| Build11 | ⚠️ 前端与大部分功能已落地，但后端概览测试挂起，不能算完整验收通过 |
| Build12 | ✅ 前端 Token 化与浮层/焦点管理已落地，前端测试全绿 |
| 全量结论 | ❌ 尚不能给出“Build1～Build12 全部真实完成”的无条件结论 |

如果要恢复“全部真实完成”的绿灯，建议按优先级处理：

1. 修复 `ExtService.ListExt` 的 SQLite 单连接嵌套查询死锁，并同步检查同类的 `CheckAllExtQuota`、`writeQuotaExceededError`；补充覆盖“高级模式 + 独立账号 + `/admin/overview`”及非空数据场景的后端测试；
2. 由用户后续按 `ProdTestList.md` 完成 Build7/Build11 等 Production 人工验收，当前不再作为 Build7 文档层面的阻塞；
3. AGENTS.md 索引已补充 Build11、Build12 状态；后续是否归档由用户决定。

本次核验与后续文档修订未修改任何业务代码；仅按用户决策更新了 Build7 状态与本报告结论。
