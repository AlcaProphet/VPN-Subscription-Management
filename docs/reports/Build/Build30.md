# Build30.md — 邮件终态结果持久化与当前发送队列优化

> **文档定位：** 本文档是 Issue18 R32-03 的已完成构建记录，依据 2026-09-19 用户确认的最终方案实施。编码约束以 [AGENTS.md](../../../AGENTS.md) 为唯一强要求；既有 R32-02 工程记录见 [Build29.md](Build29.md)。
>
> **执行原则：** 每次只执行一个 Step；完成对应验收后才进入下一步。自动化、隔离 smoke 与真实 SMTP/浏览器人工验收分开记录。

## 一、已确认合同

- 现有 `mail.Dispatcher` 的 2 worker、100 等待容量、非阻塞入队、业务结果、SMTP 测试同步语义均不改变。
- 进程内 `ActivityLog` 继续维护 `queued/sending/accepted/failed`，但前端只通过独立 active API 查看 `queued/sending` 快照。
- SQLite 仅保存每封邮件的一条 `accepted/failed` 终态结果；它不是队列、outbox、恢复来源或发送前置条件。
- 终态写入采用容量 128 的单 writer 有界旁路；缓冲满、数据库失败或 writer 停止均允许丢日志，不得影响邮件与业务结果。
- 历史结果保留 90 天；完整数据库备份自然包含，配置导出不包含。
- 页面不轮询、不使用 SSE；当前队列默认折叠，展开和按钮操作才读取快照。
- “清空历史结果”不取消等待中或发送中的邮件；清空时已产生的旧终态结果不得迟到重现，清空后才完成的任务应形成新历史结果。

## 二、构建进度

| Step | 内容 | 状态 |
|---|---|---|
| 0.5 | 基线、最终决策与影响范围核对 | ✅ 验收通过 |
| 1 | 迁移与终态结果存储/旁路 writer | ✅ 验收通过 |
| 2 | ActivityLog 终态接入 | ✅ 验收通过 |
| 3 | Server 装配、退出与清库生命周期 | ✅ 验收通过 |
| 4 | 历史结果 API 与当前队列 API | ✅ 验收通过 |
| 5 | 90 天清理与全量清表边界 | ✅ 验收通过 |
| 6 | 前端双层页面与取消轮询 | ✅ 验收通过 |
| 7 | 联合门禁、隔离 smoke 与文档同步 | ✅ 验收通过 |

## 三、分步构建计划

### Step 0.5：基线、最终决策与影响范围核对

- **目标：** 读取引用任务最终决策，核对 R32-02 当前实现、工作区和文档冲突。
- **验收：** 工作区基线干净；确认迁移号为 1022；确认不需要产品或架构追加决策。

### Step 1：迁移与终态结果存储/旁路 writer

- **目标：** 建立 `mail_result_logs` 和邮件域内单 writer 结果服务。
- **产出：**
  - `backend/migrations/1022_mail_result_logs.sql`
  - `backend/internal/mail/result_log.go`
  - `backend/internal/mail/result_log_test.go`
- **验收：** 迁移约束、数据库分页/筛选、空数组、非阻塞丢弃、写入失败不反向返回、顺序清空及清空时间边界均有定向测试。
- **命令：** `cd backend && go test ./internal/mail`

### Step 2：ActivityLog 终态接入

- **目标：** 只在合法终态转换后向结果服务提交安全快照；运行态不落库。
- **产出：** `backend/internal/mail/activity.go` 及测试。
- **验收：** accepted/failed 每次最多一条；非法/重复终态不写；完整收件人、正文、URL、Token 和原始错误不进入记录。
- **命令：** `cd backend && go test ./internal/mail`

### Step 3：Server 装配、退出与清库生命周期

- **目标：** 注入结果服务；服务退出先停 Dispatcher 再尽力排空 writer；全量清空与 writer 串行化。
- **产出：** `backend/internal/server/server.go` 和生命周期测试。
- **验收：** 清空历史不影响活动队列；全量清空不会被旧缓冲重写；清空后完成任务可形成新记录；日志失败不改变派发结果。
- **命令：** `cd backend && go test ./internal/mail ./internal/server`

### Step 4：历史结果 API 与当前队列 API

- **目标：** 现有列表/清空改为 SQLite；新增 `/api/admin/logs/mail/active`。
- **产出：** `backend/internal/server/log.go`、API 测试。
- **验收：** 双重鉴权、所有响应 no-store、严格参数、空数组、历史只含终态、active 只含 queued/sending 且不分页/不改状态。
- **命令：** `cd backend && go test ./internal/server`

### Step 5：90 天清理与全量清表边界

- **目标：** 启动即清理、每日清理；一键清空包含结果表。
- **产出：** `backend/internal/cron/cleanup.go`、`backend/cmd/server/main.go`、`backend/internal/dataclear/dataclear.go` 及测试。
- **验收：** 90 天前删除、边界内保留、清理失败仅告警；全量清表覆盖新表。
- **命令：** `cd backend && go test ./internal/cron ./internal/dataclear ./cmd/server`

### Step 6：前端双层页面与取消轮询

- **目标：** 默认折叠当前队列、历史终态列表、独立刷新/清空语义，删除全部邮件轮询状态。
- **产出：** `frontend/src/api/log.ts`、`frontend/src/views/admin/LogsView.vue`、`frontend/tests/logs-view.spec.ts`。
- **验收：** 初次进入不请求 active；每次展开请求一次；折叠不请求；无 `setInterval`；筛选/分页只影响历史；桌面与移动端均不暴露敏感字段。
- **命令：** `cd frontend && npm test -- --run tests/logs-view.spec.ts --reporter=dot && npm run build`

### Step 7：联合门禁、隔离 smoke 与文档同步

- **目标：** 完成 AGENTS.md 要求的全量验证，记录自动化与人工边界并归档 Build30。
- **产出：** `Issue18.md`、`ProdTestList.md`、`AGENTS.md`、`docs/reports/Build/Build30.md`。
- **验收命令：**
  ```bash
  cd backend && go test ./... && go build ./... && go vet ./...
  cd frontend && npm test -- --reporter=dot && npm run build
  git diff --check
  ```
- **人工边界：** 真实 SMTP、真实收件箱、浏览器响应式与重启后持久性继续留在 `ProdTestList.md`，不得以自动化替代。

## 四、最终验收结果

- 后端 `go test ./...`、`go test -race ./internal/mail ./internal/server`、`go build ./...`、`go vet ./...` 全部通过。
- `go run ./cmd/errgate ./...` 通过：baseline 0、unexpected 0。
- 前端 `npm test -- --reporter=dot` 通过：47 个测试文件、314 项测试；`npm run build` 通过。
- 定向隔离 smoke 覆盖邮件 API、active 过滤、清空边界、邮件派发/SMTP 生命周期；`git diff --check` 通过。
- 正式 SMTP、真实收件箱、浏览器响应式、重启持久性及外部客户端证据保留在 `ProdTestList.md`，未误标为完成。

## 五、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-19 | 依据引用任务最终决策建立严格串行构建步骤；Step 0.5 完成，进入 Step 1。 |
| v1.1 | 2026-09-19 | Step 1～7 严格串行完成；终态持久化、active 快照、90 天清理、双层 UI、联合门禁与文档同步验收通过并归档。 |
