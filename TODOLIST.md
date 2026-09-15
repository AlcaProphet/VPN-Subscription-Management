# TODOLIST.md — 短期待办跟踪（2026-09-15）

> 本文件只保留当前仍需处理的短期事项。已完成的构建、修复、自动化验证和人工验收不在此重复记录。

## 当前待办

### 1. Issue17：OIDC 遗留问题（核验 R30-05 后确认）

- 当前无待处理短期待办；R31-07 已完成代码与自动化隔离验证，详见 [Issue17.md](Issue17.md) 第六节；真实浏览器/隔离 IdP 人工验收待执行。

### 2. Build28 后续硬化与回归补测（2026-09-15 已完成）

> 决策记录（2026-09-15）：不重开 Build28、不新建 Build 文档，按本 TODOLIST 直接承接；严格 JSON 必须修复；implicit TLS、前端交互、smoke 五键、64 KiB/API 负向测试全部纳入；历史文档直接改写为当前状态；ProdTestList 备份项改为快照校验并新增跨反代/部署差异人工项。
> 完成状态：B28-F1～F7 已实施；后端 build/vet/full test、race、前端 build/full test（311 项）、diff-check 与状态检查均通过。以下清单保留为本次闭环记录，按跟踪规则后续维护时移除。

- [x] **B28-F1 严格 JSON 解析（P2）**
  - 范围：`backend/internal/mail/template.go`、`backend/internal/mail/template_test.go`、`backend/internal/server/settings_mail.go`、`backend/internal/server/settings_mail_test.go`、`backend/internal/config/mail_template_import_test.go`。
  - 结果：持久化解析、导入校验与 HTTP DTO 复用同一严格解析器；拒绝大小写变体 key、重复 key、未知字段、缺字段、尾随值和非字符串值。
  - 验收：表驱动负向/合法输入测试、导入前拒绝测试与 HTTP 400 测试通过。
- [x] **B28-F2 implicit TLS SMTP 层测试（P3）**
  - 范围：`backend/internal/mail/mail.go` 增加仅测试可注入的 TLS 配置；新增 `backend/internal/mail/implicit_tls_test.go`。
  - 结果：本地自签 TLS mock 验证 implicit TLS 从连接起 TLS、不会误走明文或 STARTTLS；默认生产行为不变。
  - 验收：`go test ./internal/mail -count=1` 通过。
- [x] **B28-F3 前端组件边界测试（P3）**
  - 范围：`frontend/tests/mail-template-card.spec.ts`。
  - 结果：覆盖组件卸载清定时器、切换分支继续编辑、恢复失败保留草稿、请求开始立即清旧预览。
  - 验收：`npm test -- tests/mail-template-card.spec.ts --reporter=dot` 通过。
- [x] **B28-F4 隔离 smoke 五键 Export→Import（P2）**
  - 范围：`backend/internal/server/mail_template_smoke_test.go`。
  - 结果：五个模板均经管理 API 保存，完成 Export→Import 后逐键断言 `customized` 及值一致；审批通过实际 MIME、仅一封断言保留。
  - 验收：`go test ./internal/server -run TestMailTemplateIsolatedSmoke -count=1` 通过。
- [x] **B28-F5 64 KiB 边界与 API 严格负向测试（P3）**
  - 范围：`backend/internal/server/settings_mail_test.go`。
  - 结果：验证请求体 65536 字节边界与 65537 字节 413；未知字段、大小写变体、重复 key、非字符串均 400。
  - 验收：`go test ./internal/server -count=1` 通过。
- [x] **B28-F6 文档直接改写（用户决策）**
  - 范围：`docs/reports/Build/Build28.md`、`Design5.md`、`docs/reports/Issue/Issue16.md`、`ProdTestList.md`；不重开 Build28 结构。
  - 结果：改写 Build28 边界段、增加 v0.7 记录与归档后补强说明；Design5 升至 v0.7；Issue16 预留段落注明已由 Build28 实现并修复断链；ProdTestList 新增跨反代项并明确备份快照边界。
  - 验收：链接检查通过；`git diff --check` 无输出。
- [x] **B28-F7 联合门禁**
  - 后端：`go build ./...`、`go vet ./...`、`go test ./... -count=1`、`go test -race ./internal/mail ./internal/config ./internal/approval ./internal/user ./internal/server -count=1` 均通过。
  - 前端：`npm run build`、`npm test -- --reporter=dot`（47 个文件、311 项）通过。
  - 工作区：`git diff --check`、`git status --short` 检查通过；Build28 正式人工项保留在 `ProdTestList.md`。

## 跟踪规则

1. 未执行项目不得标为通过；人工发现的问题登记到对应的当前问题记录。
2. 工程实现、自动化门禁和正式构建结果写入对应的 Issue/Build/Design 文档，本文件只保留待处理入口。
3. 完成事项从本文件移除，不保留逐项历史快照。

## 变更记录

| 日期 | 说明 |
|---|---|
| 2026-09-14 | 清理已完成事项和历史展开，仅保留 Issue16 R30-01、R26-07 两项短期待办。 |
| 2026-09-14 | 核验 R30-05 后新增 Issue17：登记 signing_key 导出损坏、真实 OIDC endpoint HTTPS 缺失及提供商切换/损坏占位符残余边界。 |
| 2026-09-14 | 按用户指令将 Issue16 原 R30-07「暂未启用」未落库问题迁入 Issue17，重编号为 R31-07。 |
| 2026-09-14 | 二轮核验补充 Issue17 R31-06：mock 模式校验不完整，Setup/SaveOidc/Exchange 需统一限制 Dev。 |
| 2026-09-14 | 用户确认 Issue16 R30-01 人工核验完成；移除对应短期待办，Issue16 按归档规则关闭。 |
| 2026-09-14 | 用户更正 R26-07 尚未完成；恢复对应短期待办。 |
| 2026-09-14 | 将 R26-07 从本表拆分至 [XrayRelated1.md](XrayRelated1.md) 集中跟踪；本表不再重复列出 Xray 相关待办。 |
| 2026-09-14 | 将 R31-01 从 Issue17 和本表分至 [ExportRelated1.md](ExportRelated1.md) 独立跟踪，并关联 Design5；R31-02 保留独立跟踪。 |
| 2026-09-15 | 按用户确认细化 R31-06：现有 Production 导入拒绝任何 mock 配置；Design5 整站导出/导入仅限 Production，恢复前拒绝非 Production 来源及 mock 配置。 |
| 2026-09-15 | R31-02 已按用户确认实施并完成自动化隔离验证：统一 OIDC HTTPS 校验、discovery endpoint 缓存前校验与实际使用点守卫、token 凭据 POST 禁止重定向、写入口与导入校验、锁死保护；代理维持 D-F06-2，真实 IdP 登录待隔离环境人工验收。 |
| 2026-09-15 | 按用户确认细化 R31-03 实施方案（固定发起 state 快照、目标读取与空 Secret 原子守卫）；同步更新 R31-04 边界并记录 R31-03 风险 4 至 R31-07 深入研究。 |
| 2026-09-15 | R31-03 已完成代码与自动化隔离验证并从本表移除：目标提供商读取/最小损坏重填、空 Secret 原子守卫、固定发起 state 快照、设置页切换与测试；真实 IdP 登录待隔离环境人工验收。 |
| 2026-09-15 | R31-04 已完成代码与自动化隔离验证并从本表移除：六态状态机、SaveOidc 全链单事务、严格 signing_key 读取、测试连接专门失败、真实登录网络前拒绝、设置页统一展示与响应/日志脱敏；真实 IdP 登录待隔离环境人工验收。 |
| 2026-09-15 | R31-06 已完成代码与自动化隔离验证并从本表移除：启动 mode 统一为唯一依据；Production Setup/管理端保存与测试连接拒绝 mock；登录/绑定发起与回调再次拒绝且无新 state/会话；公开状态隐藏 mock；v1/v2 Setup/管理端导入只要含 mock 类型或 `oidc_params_mock` 键（含空值）即整体拒绝且不写库；Dev mock 路径保持。按全新激活裁决不提供旧 mock 键清理兼容路径；真实 IdP 与 Design5 整站恢复均未验收。 |
| 2026-09-15 | R31-07 已完成代码与自动化隔离验证并从本表移除：流程代际指纹、`DisableOidc`/`ClearOidc`/`SaveLocalAuth` 单事务、旧 state/票据不可恢复、直接 API/公开状态守卫、设置页草稿与保存停用/重新启用；真实浏览器与隔离 IdP 人工验收待执行。 |
| 2026-09-15 | 按用户决策完成 Build28 后续硬化与回归补测：严格 JSON、implicit TLS、前端边界、smoke 五键、64 KiB/API 负向、文档改写与联合门禁均通过；TODOLIST 清单保留为本次闭环记录，后续维护时移除。 |
