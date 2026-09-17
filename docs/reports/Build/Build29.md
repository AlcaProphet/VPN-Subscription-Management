# Build29.md — R32-02 业务邮件异步派发与短期发送日志

> **状态：** 已按用户一次性授权严格串行完成 Step 1～11 工程实现、定向/全量/race 门禁、errgate、前端构建与测试、本地隔离 smoke；正式 SMTP、真实收件箱与外部客户端人工项保留在 [ProdTestList.md](../../../ProdTestList.md)。

## 一、目标与合同

- 注册、OIDC 首次激活、管理员创建、审批、密码重置的 HTTP 响应不等待任何 SMTP 网络阶段；前端 15 秒超时不变。
- 所有业务邮件经唯一 `mail.Dispatcher` 异步派发；固定 2 worker、等待队列 100、非阻塞入队。
- 短期 `mail.ActivityLog` 最近 500 条、ID 单调、收件人掩码、不落库；SMTP 测试同步且写日志。
- best-effort、无 outbox/自动重试；邮件失败不回滚业务事务。
- 管理员密码邮件统一可用性、入队拒绝精确删除本次 token、批量回执字段调整。
- 服务退出 Stop；全量清空前后 PauseAndDrain/Resume；活动 SMTP 取消、等待任务丢弃、运行代次隔离。
- 管理 API 与前端“邮件发送日志”页签；真实 SMTP/收件箱/客户端验收另行人工。

详细决策与接口合同见 [Issue18.md](../../../Issue18.md) §2.2～§2.13。

## 二、实际影响文件

| 模块 | 文件 |
|---|---|
| 邮件领域 | `backend/internal/mail/mail.go`、新增 `failure.go`、`activity.go`、`dispatcher.go` 及对应 `_test.go` |
| 配置严格读取 | `backend/internal/config/config.go`、`smtp.go`、`admin.go` |
| 注册/OIDC/管理员创建 | `backend/internal/user/user.go`、`user/oidc.go`、`user/admin.go` 及测试 |
| 审批 | `backend/internal/approval/approval.go`、`approval_test.go` |
| 密码重置 | `backend/internal/auth/reset.go`、`reset_test.go`；`user/admin/*_test.go`；`server/auth.go`、`server/user.go` |
| 服务装配/生命周期 | `backend/internal/server/server.go`、`approval.go`、`settings.go`、`settings_ops.go`、`dataclear/dataclear.go` 及测试 |
| 邮件日志 API | `backend/internal/server/log.go`、新增日志 API 测试 |
| 前端 | `frontend/src/api/log.ts`、`settings.ts`、`user.ts`、`views/admin/LogsView.vue`、`UsersView.vue`、`ForgotView.vue`、`ApprovalsView.vue`、`SettingsView.vue`、对应测试 |

## 三、串行步骤与验收

| Step | 目标 | 结果 |
|---|---|---|
| 1 | 类型化失败阶段、严格可用性、传输拆分 | ✅ 阶段枚举/分类、`SMTPConfiguredStrict`、`EffectiveSiteNameStrict`、`sendJob` 等完成 |
| 2 | ActivityLog、掩码、状态转换 | ✅ 500 容量、单调 ID、迟到更新跳过、掩码、race 测试 |
| 3 | Dispatcher 队列/worker/生命周期 | ✅ 2 worker、100 队列、103rd rejected、panic 隔离、Stop/Pause/Resume、代次隔离 |
| 4 | 欢迎三入口迁移 | ✅ 注册/OIDC/管理员创建只入队；阻塞 SMTP 下 HTTP 快速返回 |
| 5 | 审批路径迁移 | ✅ 审批事务后只派发通知；业务统计不受邮件结果影响 |
| 6 | 密码重置全路径 | ✅ 公共/单用户/批量统一可用性、精确 token 补偿、批量新回执字段 |
| 7 | SMTP 测试同步接入日志 | ✅ `TestSMTP` 不走队列、sending→accepted/failed |
| 8 | 退出与全量清空生命周期 | ✅ `Server.Run` Stop；清库前后 hook；前置失败 503 且不清库 |
| 9 | 邮件日志后端 API | ✅ GET/clear、401/403/400/no-store、筛选分页、空数组、幂等 |
| 10 | 前端第三页签/可用性/文案 | ✅ 5 秒单飞轮询、筛选/分页/清空/移动卡片、密码邮件文案 |
| 11 | 联合门禁与文档收口 | ✅ 全量门禁、race、errgate、隔离 smoke、前端 build/test、文档同步 |

## 四、自动化证据

```bash
cd backend
go build ./...
go vet ./...
go test ./... -count=1
go test -race ./internal/mail ./internal/auth ./internal/user ./internal/approval ./internal/dataclear ./internal/server -count=1
go run ./cmd/errgate ./...
```

- 定向 smoke 覆盖：阻塞 SMTP 下注册快速返回；2 worker/100 队列/第 103 个 rejected；SMTP 测试无 queued 且写日志；邮件日志 API 鉴权/no-store/筛选/清空幂等；清空邮件日志不取消发送；全量清空成功清日志并恢复派发器、前置暂停失败 503 且不清库。
- 前端：`npm test -- --reporter=dot`（47 个测试文件、314 项测试）与 `npm run build` 通过。
- 结果：以上命令均通过；未新增数据库迁移或持久化邮件日志。

## 五、验收边界与后续

- 本构建的 `accepted` 只表示 SMTP 会话成功，不表示真实收件箱投递。
- 正式 SMTP/收件箱/外部邮件客户端/真实浏览器验证属于 [ProdTestList.md](../../../ProdTestList.md) 人工项，需在隔离环境使用合成账号和用户提供的 SMTP 凭据执行，不得用本地 mock 替代。
- 停止条件继续有效：若需改变 2/100/500 容量、无重试、SMTP 测试同步、公共防枚举、token 补偿、批量语义或清空失败 503 映射，必须暂停并由用户重新决策。
