# BuildReport6.md — Design1～Design4 全量只读实现核验报告

> **核验日期：** 2026-09-14 12:18 CST
> **分支与 HEAD：** `beta` / `2300369daf876729b91979763b3e1575c3676156`（2026-09-14 11:54:37 +0800，标题：改进邮件配置的安全性与灵活性）
> **起始工作区状态：** 2026-09-14 开始核验时 `git status --short` 无输出，工作区干净。
> **核验对象：** 当前工作区实际代码、配置、测试、迁移与文档，不采用任何旧 Build/BuildReport 的“已完成”表述代替当前实现证据。
> **核验范围：** Design1～Design4 有效合同、AGENTS.md 强要求、BuildReport1～5 发现项回归、活跃跟踪项排除矩阵、当前工作区证据层级边界。
> **核验原则：** 只读、证据驱动、先建立需求—实现—证据矩阵再判定；区分静态代码、自动化测试、隔离运行、Docker、Production、真实浏览器/手机/固定版本客户端/真实连接和用户人工验收。
> **明确排除：** SecurityScanPlan1/SecurityReport3 第三期安全扫描步骤与结论、用户指定的 Issue16 当前跟进项及其未授权未来方案、核验期间外部新增的 Issue17 已跟踪项、ProdTestList/TODOLIST 人工核验动作、Design5 候选构想，以及其他明确属于未来版本的内容。
> **重要环境说明：** 本次核验开始后约 12:18，工作区外部并行出现 `AGENTS.md`/`Issue16.md` 修改和 `Issue17.md` 新增；这不是本报告操作产生的。本报告不对其内容作设计判定，仅将其登记为活跃排除/当前跟踪来源；对这些文件的判定以其外部修改后的当前状态和本报告核验时点为准。

---

## 一、执行摘要

### 1. 可以确认的范围

1. **Design1 第一期基线主路径当前仍落地。** Setup/认证/权限/资源 CRUD/四类资源共用版本管理/三类下载 Token/下载分发/平台安装包/配置中心/日志与数据清理/迁移与部署约束，均有当前代码位置与自动化测试证据；后端全量测试和前端全量测试通过。
2. **Design2/Design3 主体已实现并经本次当前代码回归。** 规则素材池、三来源模式、Canonical Rule/Origin/能力注册表、快照与原子激活、装配预览与生成、SR/generic/Clash 渲染、Xray 实例检测/用户同步/配额/对账/OFF 清空、配置导出 v2，均能在当前代码中找到实现和定向测试。BuildReport4 的 D3-1～D3-10、N-node-1～N-node-6 在本次静态复核与全量自动化中未发现回退。
3. **Design4 首批四协议与统一节点编辑契约当前仍落地。** `nodes` 当前状态/修订号/扩展列、409 修订冲突、作用域清空、凭据 keep/clear/替换、局部 JSON `allow_unknown` 白名单、`item_id_field`、目标检查与输出门槛、URI 导入/Xray 来源适配、§12.7 A/B/D/E/G/I 合同均有代码和自动化测试证据。
4. **AGENTS.md 的核心安全底线基本存在：** 路径穿越防护、AES-GCM 敏感数据加密、Token/URL 凭证脱敏、实时查库权限、管理员双层中间件、下载 no-store、事务/写锁、级联删除、构造注入、接入层不直接访问存储、SSE 管理员 Bearer 鉴权、导入 20 MiB/21 MiB 上限、前端路由级代码分割、单容器非 root 部署等。
5. **本次在隔离副本中通过后端构建/静态检查/全量测试、关键并发包 race、前端测试/生产构建。** 详细命令见第十一章。

### 2. 不能确认的范围

1. **当前 HEAD 的 `errgate` 静态错误门禁失败。** `go run ./cmd/errgate ./...` 以退出码 1 结束，报告 `backend/internal/mail/mail.go (*Service).Send [assign] conn.SetDeadline(deadline)`。这与 Build26 记录的“errgate 清零”不一致，并违反 AGENTS.md「所有 error 必须处理」。见新发现 **N-01**。
2. **SMTP STARTTLS 扩展检测错误被 comma-ok 分支丢弃。** `client.Extension("STARTTLS")` 的错误未处理：legacy 模式可能在探测失败时继续走未升级通道，starttls 模式会把读取错误误报为“服务器未提供 STARTTLS”。见新发现 **N-02**。
3. **Design1～Design4 的真实客户端/真实连接层未由本次证明。** Docker build/compose、Production smoke、真实浏览器、真实手机、固定版本客户端（Mihomo/Clash Verge/Shadowrocket）、真实 SMTP/OIDC/Xray 连接均未执行或未由本次复现。
4. **ProdTestList 的人工项目仍未由本次代替。** R30-01 邮件重置链接四态、R26-07 Xray 禁止编辑错误码、以及工作区当前登记的其它人工项，本次均未执行。
5. **第三期安全扫描范围仍被排除。** SecurityScanPlan1 SecurityReport3 的未完成步骤不纳入本报告作为 Design 核验缺口。
6. **Issue16/Issue17 的当前问题不由本报告重复登记。** 其中 R30-01～R30-05 仍分别由 Issue16/Issue17/ProdTestList 跟踪；本报告只在排除矩阵中登记。

### 3. 新发现与观察项数量

| 类别 | 数量 | 说明 |
|---|---:|---|
| 确定性新发现 | **2** | N-01 errgate 回归；N-02 SMTP Extension 错误未处理 |
| 待确认/观察项 | 5 | 版本文件失败清理、归档文档内链、AGENTS 状态文字、代码注释/死代码、外部并行工作区变化 |
| 文档冲突/待用户决策 | 3 | 重置令牌“用后即删”与四态状态口径；SMTP GET 脱敏合同变化；AGENTS 对 Issue14/errgate 的状态文字 |
| 排除项 | 12 | 见第十章；不计入新发现数量 |

### 4. 自动化验证摘要（本次实际执行，隔离副本）

| 验证项 | 命令 | 结果 |
|---|---|---|
| 后端构建 | `cd backend && go build ./...` | PASS |
| 后端静态检查 | `cd backend && go vet ./...` | PASS |
| 后端全量测试 | `cd backend && go test ./... -count=1 -timeout 300s` | PASS，40 个有测试包全部 ok，另 5 个包无测试文件 |
| 关键并发包 race | `go test -race ./internal/user ./internal/oidc ./internal/server ./internal/version ./internal/xray ./internal/token -count=1 -timeout 300s` | PASS，无 DATA RACE |
| 架构门禁 | `go test ./internal/server -run 'Architecture|NoDirect|Store' -count=1 -v` | PASS |
| 后端错误门禁 | `cd backend && go run ./cmd/errgate ./...` | **FAIL**：1 项未在空基线中的被丢弃 error（N-01） |
| 前端测试 | `cd frontend && npm test -- --run` | PASS，46 文件 / 275 用例 |
| 前端定向门禁 | `npx vitest run tests/style-tokens.spec.ts tests/import-limit.spec.ts tests/sse-parser.spec.ts tests/logs-view.spec.ts` | PASS，4 文件 / 14 用例 |
| 前端生产构建 | `cd frontend && npm run build` | PASS，`vue-tsc -b` + Vite 构建成功 |
| 仓库内链检查 | `node scripts/check-md-links.mjs <全部 Markdown>` | FAIL：921 条相对链接中 97 条缺失，主要含模板占位符与 Issue14 归档后的历史相对路径；见 OBS-02 |
| Git 空检查 | `git diff --check` | PASS，无空白错误 |

### 5. 总体结论

**“Design1～Design4 主体代码与自动化主路径已经落地”可以成立；“当前 HEAD 全部工程门禁全绿、Design1～Design4 当前实现无缺口”不能成立。**

原因是：当前 HEAD 的 `errgate` 失败是确定可复核的新回归；SMTP 扩展探测错误处理仍有 AGENTS 强要求缺口；版本文件失败清理边界和文档内链仍有待确认/整理项；Docker/Production/浏览器/手机/固定客户端/真实连接和 ProdTestList 人工项仍未由本次证明。

---

## 二、核验基线与证据边界

### 2.1 Git 基线

| 项目 | 值 |
|---|---|
| 分支 | `beta` |
| HEAD | `2300369daf876729b91979763b3e1575c3676156` |
| HEAD 标题 | 改进邮件配置的安全性与灵活性 |
| HEAD 时间 | 2026-09-14 11:54:37 +0800 |
| 开始核验时 `git status --short` | 无输出（干净） |
| 核验期间工作区变化 | 外部并行出现 `M AGENTS.md`、`M Issue16.md`、`?? Issue17.md`；HEAD 未变 |
| 本次新报告 | `docs/reports/BuildReport/BuildReport6.md` |

### 2.2 工具链

| 工具 | 版本/环境 |
|---|---|
| Go | go1.26.4 darwin/arm64 |
| Node.js | v26.7.0 |
| npm | 11.19.0 |
| 操作系统 | Darwin 25.6.0 arm64 |
| 自动化隔离副本 | `/tmp/vpnsub-audit.zA2Ird`（rsync 源仓库，排除 `.git`；Go 缓存与临时目录均在 `/tmp`） |

### 2.3 证据层级

| 层级 | 本次状态 | 说明 |
|---|---|---|
| 静态源码/配置/迁移 | 已完成 | 作为主要实现证据 |
| 自动化测试 | 已完成 | 隔离副本全量 + 定向 + 关键 race |
| 隔离运行/API | 部分完成 | 测试内 `httptest`、临时 SQLite、mock SMTP；未启动独立常驻服务 |
| 数据库迁移 | 已完成自动化 | 0001～1018 迁移文件存在，store 级迁移测试通过 |
| Docker build | 未执行 | 为避免干扰当前镜像/服务，且用户要求不触及运行中服务 |
| Production smoke | 未执行 | 需隔离 Production 环境与持久化数据，本轮不满足安全执行条件 |
| 真实浏览器/手机 | 未执行 | 由 ProdTestList 跟踪 |
| 固定客户端与真实连接 | 未执行 | 无 Mihomo 1.19.29 固定二进制/真实客户端/真实服务凭据；不执行真实连接 |
| 用户人工验收 | 未执行 | ProdTestList、Issue16/Issue17 当前项不在本次代替范围 |

### 2.4 范围与排除口径

- **Design1 现行基线：** [Design1.md](../Design/Design1.md) 作为第一期入口，后续 Design2/Design3/Design4 明确覆盖的内容以后续设计有效合同为准。
- **Design2/Design3：** 使用归档设计 [Design2.md](../Design/Design2.md)、[Design3.md](../Design/Design3.md)；Design3 的不兼容来源模型覆盖 Design2 的 `urls_json` 旧模型。
- **Design4：** 使用根目录 [Design4.md](../../../Design4.md)；Build26 的 §12.7 作为 R28-07A/B/D/E/G/I 的现行工程合同补充。
- **AGENTS.md：** [AGENTS.md](../../../AGENTS.md) 是唯一强要求，本次逐项核验与实现直接相关的工程/安全/架构/部署要求。
- **排除：** SecurityScanPlan1/SecurityReport3 第三期扫描、用户指定的 Issue16 当前跟踪项及其后续平台方案、核验期间新增且已由 Issue17 跟踪的 R30-05、ProdTestList/TODOLIST 人工核验动作、Design5 候选构想、Design4 明确列为后续专项的剩余 15 协议完整表单/SS 2022 完整兼容/独立 Xray outbound。
- **核验期间的文档变化：** `AGENTS.md` 的外部状态修订、`Issue16.md` 的 v1.3 外部修订和 `Issue17.md` 的 R30-05 新增，均按外部并行变更处理；其中 Issue17 按“活跃跟踪来源”排除，不重复登记为新发现；本报告不占用、不修改这些文件。

---

## 三、文档与范围盘点

### 3.1 本次主要读取

| 类别 | 文档 |
|---|---|
| 强要求 | AGENTS.md |
| 设计 | Design1.md、Design2.md、Design2-UI.md、Design3.md、Design4.md；Design5.md 仅确认候选/未定稿边界 |
| 历史核验 | BuildReport1.md～BuildReport5.md |
| 构建 | Build16.md、Build17.md～Build27.md 中与本次回归相关的归档记录；Build26.md、Build27.md 全文入口与验收段 |
| 问题 | Issue13.md～Issue15.md；Issue16.md/Issue17.md 用于划清当前排除边界 |
| 活跃跟踪 | TODOLIST.md、ProdTestList.md |
| 代码/测试 | backend、frontend/src、frontend/tests、migrations、Dockerfile、compose 模板、CI workflow、README |

### 3.2 有效合同关系

| 关系 | 处理 |
|---|---|
| Design2 规则 URL `urls_json` 旧模型 → Design3 三模式/source 模型 | 以后者（Design3）有效；当前代码使用 1016 迁移后的来源/快照模型 |
| Design1 SSE 一次性 Query Token → Design4 §12.7 I Bearer 会话 SSE | 以后者（Design4 v1.23）有效；当前代码无 `/stream/token` |
| Design1 导入“不设大小上限” → Design4 §12.7 G 20 MiB 文件/21 MiB 请求体 | 以后者（Design4 §12.7）有效；当前代码和前端都按新合同实现 |
| Design1 首管理员 `admin_initialized` 写入 → Design4 §12.7 A 不再写入/读取 | 以后者有效；当前代码只在测试/常量中保留历史键 |
| Design1 自定义/组 Token 优先级 → Design4 §12.7 B 业务键协调 | 以后者有效；当前 `token.ResolveUserToken` 实现 |
| Design1 验证码明文/CSP 等历史安全建议 | 由 Issue14 R28-07F/R28-08 用户确认的设计取向关闭；本报告按排除处理，不扩为第三期扫描 |
| Design4 §9/§10.3/§11.3 后续专项 | 明确不作为本次首批完整表单/客户端连接证据；不计为当前缺口 |

---

## 四、Design1 有效需求与逐项证据矩阵

> 判定用词：**符合 / 不符合 / 证据不足 / 不适用 / 已排除**。本表覆盖 Design1 产品主路径和强约束，不逐字复述 925 行原文。

| ID | 设计依据 | 当前实现/代码位置 | 本次测试或实际证据 | 判定 |
|---|---|---|---|---|
| D1-01 | Design1 §3.1 首次部署、Setup 分支、预置默认组/平台 | `backend/internal/setup/setup.go`；`frontend/src/views/SetupView.vue`；`backend/migrations/0003_groups_platforms.sql`、`1009_xray.sql`、`1015_platform_builtin_default.sql` | `go test ./internal/setup ./internal/server` PASS；前端 `setup`/`import-limit` 定向 PASS；未执行 Docker 首次启动 | 符合（自动/静态） |
| D1-02 | Design1 §3.1、§4.6 首用户管理员、本地密码、会话时长/版本失效 | `backend/internal/user/user.go`；`backend/internal/auth/auth.go`；`backend/internal/user/admin.go`；迁移 0002/1009 | 全量 `go test ./internal/user ./internal/auth` PASS；并发/首管理员测试通过；race 关键包 PASS | 符合 |
| D1-03 | Design1 §3.2、§4.6、§6.1 OIDC PKCE/state/绑定/待审批/模拟 | `backend/internal/oidc/*`；`backend/internal/server/oidc.go`；迁移 0004/1012/1013 | `go test ./internal/oidc ./internal/server` PASS；mock/真实提供商未执行 | 符合（自动/静态；真实 OIDC 未验证） |
| D1-04 | Design1 §2.5、§3.4.5、§3.4.6、§5.4 五重管理员保护、实时角色、双层中间件 | `backend/internal/user/admin.go`（`checkNotSelf`/`countActiveAdmins`/`ChangeRole`/`SetStatus`/`Delete`）；`backend/internal/auth/auth.go`；各 `Register*Routes` 叠加 `SessionMiddleware+AdminMiddleware` | 全量 `go test ./internal/user ./internal/auth ./internal/server` PASS；`snapshot` 实时查库路径存在；前端分组/用户测试 PASS | 符合 |
| D1-05 | Design1 §2.2、§2.6、§4.6 账号状态/来源/默认组/禁用清 Token | 迁移 0002/1009；`user.Service.Register`、`CreateFromOidc`；`AdminService.SetStatus` | 全量测试 PASS；禁用与 credential_version/Token 删除同事务代码审查 | 符合 |
| D1-06 | Design1 §4.1、§5.5、§8 四类版本管理、5 版上限、原子切换、DB 为事实源 | `backend/internal/version/version.go`；`subscription/rule/custom/share` 服务；`version_test.go` | `go test ./internal/version ./internal/subscription ./internal/rule ./internal/custom ./internal/share` PASS；并发/5 版/回滚测试 PASS | 符合（主路径）；文件删除失败边界见 OBS-01 |
| D1-07 | Design1 §4.2、§4.4 三类 Token、三态复用、刷新/吊销/级联 | `backend/internal/token/token.go`；`backend/internal/download/download.go`；Build26 §12.7 B | `go test ./internal/token ./internal/download` PASS；R28-07B 定向/并发测试 PASS；race `token` PASS | 符合 |
| D1-08 | Design1 §4.3、§6.4 下载分发、404/200 注释、no-store、限流、附加头、文件名 | `server/download.go`、`internal/download/download.go`、`internal/platform/platform.go`；`headers_test.go` | 全量测试 PASS；`setNoCache`、`BuildContentDisposition`、`ErrTokenInvalid`/`ErrUnassigned` 路径存在 | 符合 |
| D1-09 | Design1 §3.4.4、§4.7、§5.5、§6.3 平台/安装包/静态路径分级/穿越 | `internal/platform/platform.go`；`server/static.go`；`internal/config/admin.go` | 平台/静态路径测试 PASS；`filepath.Clean`/`filepath.Base`/大小上限检查通过静态核验 | 符合 |
| D1-10 | Design1 §3.3、§3.5、§3.6 用户首页/规则页/个人中心 | `home`、`rules`、`profile` 服务与 Vue 视图；`frontend/tests/home-view.spec.ts`、`rules-view.spec.ts`、`profile` 相关 | 前端 46 文件/275 用例 PASS；后端对应包 PASS；真实浏览器未执行 | 符合（自动/组件；浏览器未验证） |
| D1-11 | Design1 §3.4.6、§4.6 审批、欢迎/审批/重置邮件 scope | `internal/approval/approval.go`；`internal/mail/mail.go`；`internal/server/approval.go`；Build26/Issue16 | Go 全量测试 PASS；邮件 4 类业务模板与 scope 测试 PASS；真实 SMTP 未执行 | 符合（自动；真实邮件已排除为 Issue16/ProdTestList） |
| D1-12 | Design1 §3.4.8 面板配置中心（OIDC/本地/验证码/SMTP/站点/限流/日志/公告/调试） | `internal/config/admin.go`；`server/settings.go`；`SettingsView.vue` | 配置/服务测试 PASS；前端设置测试 PASS；SMTP 合同变化见 DC-02 与排除 EX-01 | 符合（自动；SMTP/OIDC 当前项已排除） |
| D1-13 | Design1 §3.4.8、§4.8 导入导出、备份、一键清空、限流/日志重置 | `server/settings_ops.go`、`internal/config/export.go`、`internal/backup/backup.go`、`internal/dataclear/dataclear.go` | 全量测试 PASS；导入 20/21 MiB 测试 PASS；未执行真实 Production 导入/备份 | 符合（自动/静态） |
| D1-14 | Design1 §3.8、§4.8 应急恢复、访问日志、SSE、90 天清理 | `internal/emergency/*`、`internal/log/*`、`internal/server/emergency*.go`、`server/log.go`、`cron` | 全量测试 PASS；SSE 路由/权限测试 PASS；Design4 §12.7 I 已覆盖查询 Token 旧口径 | 符合（自动/静态；真实流未执行） |
| D1-15 | Design1 §5.1、§5.5、§5.6、§7.2、§7.4 纯 Go/CGO/迁移/单卷/部署 | `backend/go.mod`、`backend/migrations/0001～1018`、`Dockerfile`、`docker-compose.yml.example`、`store.Migrate` | `go build`/`go vet`/全量测试 PASS；迁移文件与 store 测试 PASS；Docker build 未执行 | 符合（静态/自动；Docker 未验证） |
| D1-16 | Design1 §8 工程质量：拆分、复用、构造注入、前端按需 | `server/*` 分域 Handler；`frontend/src/router/index.ts` 动态 import；共享 `VersionManageView` 组件 | `server` 架构门禁 PASS；前端构建产物多 chunk；无 `server` 生产文件直接 `DB()` | 符合 |
| D1-17 | Design1 §6 安全底线：路径、加密、凭据脱敏、实时权限、下载禁缓存 | `config`/`node`/`log`/`redact`/`auth`/`download`/`static` | 定向测试、全量测试、race、静态 grep 均通过；验证码明文/CSP 等历史项按 EX-08/EX-09 排除 | 符合（核验范围内；排除项另列） |
| D1-18 | Design1 §9 范围外事项 | 设计明确列出 | 不纳入本报告判定 | 不适用/已排除 |

### 4.1 Design1 关键限制说明

- **D1-03 OIDC 真实提供商**：本次仅运行 mock/自动化；真实 Keycloak/Auth0/通用 OIDC 回调未执行，不能据此宣称真实 SSO 可用。该结论与历史 Build 一致。
- **D1-11 邮件**：真实 SMTP 和重置链接四态属于 Issue16/ProdTestList 活跃范围，本次只验证代码/自动化层，不代替人工结果。
- **D1-12 SMTP/OIDC 字段合同**：当前 SMTP GET 已不再回显 `***`，OIDC GET 仍可能存在占位符回存路径；分别由 Issue16 R30-02 和 Issue17 R30-05 跟踪，本次不重复登记。
- **D1-15 Docker/Production**：未执行不等于失败；但也不得写成“通过”。

---

## 五、Design2 有效需求与逐项证据矩阵

> Design3 已覆盖 Design2 的规则来源/`urls_json` 旧合同；以下以 Design2 中仍有效的模式分层、装配、配置生成、下载、Xray 和分发合同为准。

| ID | 设计依据 | 当前实现/代码位置 | 本次测试或实际证据 | 判定 |
|---|---|---|---|---|
| D2-01 | Design2 §1、§5.10 基础/高级模式、高级端点 403、OFF 清空 | `server/middleware.go` `AdvancedMode`；`server/xray.go`、`server/group.go`、`server/user.go` 高级子路由；`internal/xray/offclear.go` | `go test ./internal/xray ./internal/server ./internal/config` PASS；路由静态核验；未执行真实 OFF 清空 API | 符合（自动/静态） |
| D2-02 | Design2 §2 规则素材池 | 当前实现已由 Design3/1016 演进：`internal/pool/*`；`rule_pool_sources`/snapshot/canonical/origin | `go test ./internal/pool` PASS；旧 `urls_json` 合同已按 Design3 覆盖 | 已由 Design3 覆盖（符合现行） |
| D2-03 | Design2 §3 节点统一模型、显示名、三类强制组、预设/自建、DAG | `internal/node/*`；`internal/proxygroup/*`；`internal/assembly/*`；迁移 1009 | `go test ./internal/node ./internal/proxygroup ./internal/assembly` PASS；显示名/组/DAG/强制组测试覆盖 | 符合 |
| D2-04 | Design2 §4 四装配器、预览/生成、蓝图/重编辑 | `internal/assembly/*`；`server/assembly.go`；`frontend/src/views/admin/assembly/*` | Go 全量/定向 PASS；前端 `assembly-view`、`preview-step`、`header-step` 等测试 PASS；真实浏览器未执行 | 符合（自动/组件；浏览器未验证） |
| D2-05 | Design2 §5 Xray 对接：实例/检测/同步/配额/对账/OFF/独立账号 | `internal/xray/*`；`server/xray.go`；迁移 1009/1011 | `go test ./internal/xray ./internal/server` PASS；race `xray` PASS；无真实 Xray 实例 | 符合（自动/静态；真实连接未验证） |
| D2-06 | Design2 §5.6～§5.8 组=授权+配额、公共节点、流量采集、超限 | `internal/group/*`、`internal/xray/*`、`internal/cron/*`、`internal/home/*`、`internal/profile/*` | 全量测试 PASS；配额/流量汇总定向测试 PASS；真实采集未执行 | 符合（自动；真实 Xray 未验证） |
| D2-07 | Design2 §5.7 动态下载、占位替换、蓝图重渲染、响应头、no-store | `internal/userrender/*`；`internal/download/download.go`；`internal/assembly/*` | `go test ./internal/userrender ./internal/download ./internal/assembly` PASS；race server/userrender 覆盖；真实客户端未执行 | 符合（自动；真实客户端未验证） |
| D2-08 | Design2 §5.9、§5.11 配置导出 v2/advanced/import/OFF 双确认/独立账号 | `internal/config/export.go`、`export_v2_test.go`、`server/settings_ops.go`；`server/server.go` 注入导入后处理 | 全量测试 PASS；导入接口 401/403/413 定向测试 PASS；未执行真实 Production 迁移 | 符合（自动/静态） |
| D2-09 | Design2-UI 受影响的页面布局/交互/响应式 | `frontend/src/views/*`、`components/*`、`utils/*` | 前端 46 文件/275 用例 PASS；构建 PASS；真实浏览器/手机未执行 | 符合（组件/构建；人工未验证） |

---

## 六、Design3 有效需求与逐项证据矩阵

| ID | 设计依据 | 当前实现/代码位置 | 本次测试或实际证据 | 判定 |
|---|---|---|---|---|
| D3-01 | Design3 §1、§2、§4.1 三来源模式、单 URL 单主方言、混合/冲突硬失败 | `internal/pool/detector.go`、`adapter_*.go`、`pipeline.go`、`sync.go`；`parser_test.go`、`sync_new_test.go` | `go test ./internal/pool` PASS；三模式/单适配器/硬失败测试覆盖 | 符合 |
| D3-02 | Design3 §3.1、§3.2 Canonical Rule、Origin、去重/排序/证据 | `internal/rulespec/canonical.go`；`pool/snapshot.go`、`sync.go`；`pool_rule_origins` 1016 表 | `go test ./internal/rulespec ./internal/pool` PASS；Build22 D3-1～D3-5 回归测试通过 | 符合 |
| D3-03 | Design3 §3.3 中央能力注册表、`supports_no_resolve`、advanced-only 边界 | `internal/rulespec/spec.go`、`capability.go`、`legacy.go`；`server/rulespec.go`；前端 `api/rulespec.ts` | `go test ./internal/rulespec ./internal/server` PASS；前端装配/池详情测试使用元数据端点 | 符合 |
| D3-04 | Design3 §4、§5 适配器、识别率、IDNA/PSL/CIDR/ASN/通配 | `internal/pool/adapter_*.go`、`normalize.go`、`parser.go`；`rulespec` 校验 | `go test ./internal/pool ./internal/rulespec` PASS；语料/PSL/IDNA/IPv4/IPv6 测试覆盖 | 符合 |
| D3-05 | Design3 §6.1～§6.4 快照、staging、原子激活、pending 保护、v1 stats、脱敏限额 | `internal/pool/snapshot.go`、`sync.go`、`sanitize.go`；迁移 1016/1018 | `go test ./internal/pool ./internal/redact` PASS；Build22 Step 7 专属测试矩阵在当前代码中仍通过 | 符合 |
| D3-06 | Design3 §7、§9.3 Clash/SR 目标过滤、转换回执、零输出门槛、`no_resolve` 冻结 | `internal/assembly/render_*.go`、`receipt.go`、`clash_plan.go`、`no_resolve_test.go`；`server/assembly.go:137-140` | 全量/定向测试 PASS；空规则/全不支持/零输出 SR/generic 门槛测试 PASS | 符合 |
| D3-07 | Design3 §8 三模式 UI、状态/回执/pending、只展示 display_url | `frontend/src/views/admin/assembly/PoolDetail.vue`、`PoolTab.vue`、`api/pool.ts` | 前端 `pool-detail.spec.ts`、`pool-tab.spec.ts`、`assembly-view.spec.ts` PASS；真实浏览器未执行 | 符合（组件/自动；浏览器未验证） |
| D3-08 | Design3 §6.5 不兼容迁移、旧 ID 防复用、历史蓝图/下载保留 | 迁移 `1016_rule_pool_snapshots.sql`；`store`/`pool`/`assembly` 测试 | 迁移测试与全量 pool/assembly 测试 PASS；实际旧库迁移未执行 | 符合（自动/静态；真实旧库未验证） |
| D3-09 | Design3 §8.4 Clash YAML 头部四分区、默认值、作用域 JSON | `frontend/src/views/admin/assembly/HeaderStep.vue`、`clashHeaderDefaults.ts`；`frontend/tests/header-step.spec.ts` | 前端 `header-step.spec.ts` PASS；构建 PASS | 符合（组件/自动） |

---

## 七、Design4 有效需求与逐项证据矩阵

| ID | 设计依据 | 当前实现/代码位置 | 本次测试或实际证据 | 判定 |
|---|---|---|---|---|
| D4-01 | Design4 §3 表单阅读顺序、动态显示、独立开关区、分支清空/折叠 | `frontend/src/components/ProtocolFieldEditor.vue`、`NodeCheckPanel.vue`；`frontend/src/views/admin/NodesView.vue`；`utils/nodeFormLayout.ts`、`nodeFeatures.ts` | 前端 `nodes-view.spec.ts` 38 用例、`node-form-layout.spec.ts`、`protocol-field-editor.spec.ts` PASS；构建 PASS；真实浏览器/手机未执行 | 符合（组件/自动；人工未验证） |
| D4-02 | Design4 §4 推荐下拉、自定义草稿、列表推荐、未设置数字、旧值回显、`allow_custom` 三态 | `registry.go` `OptionItems`/`AllowCustom *bool`；`ProtocolFieldEditor.vue`；`EditableCombobox.vue`；`project_test.go`、`nodes-view.spec.ts` | 后端 `node` 测试 PASS；前端可编辑下拉/节点表单测试 PASS | 符合 |
| D4-03 | Design4 §5、§12.4 首批四协议条件表单/校验、SS 2022 后置、XHTTP `none` | `registry.go` `enrichVLESS/VMess/Trojan/SS`；`project.go` `validateProtocolCombination`；`node_test.go`、`all_protocols_test.go` | `go test ./internal/node` PASS；四协议组合/正反例测试覆盖 | 符合 |
| D4-04 | Design4 §6.1～§6.3、§12.1、§12.3 保存契约、`nodes` 新列、修订 409、reset/credential 操作、读取脱敏 | 迁移 `1017_node_editor_state.sql`；`node.Node`/`UpdateManual`/`mergeSensitiveWithOps`；`server/node.go` | `go test ./internal/node ./internal/server` PASS；修订冲突/凭据 keep/clear/加密/读取脱敏测试 PASS；race server PASS | 符合 |
| D4-05 | Design4 §6.4、§12.1 当前状态投影、未知扩展整体加密/摘要/targets 诊断/不输出 | `node.ProjectActive`、`prepareExtension*`、`extensionDiagnostics`；`assembly/r28_06_extension_output_test.go`、`node/r28_06_test.go` | `go test ./internal/node ./internal/assembly` PASS；扩展不进入 Clash/SR/generic 的负向测试 PASS | 符合 |
| D4-06 | Design4 §6.4、§12.2 `allow_unknown` 显式白名单、`item_id_field`、父子 JSON 草稿、历史未知键 | `project.go` `validateActiveObjectFields`；`sensitive_paths.go` `ensureSensitiveItemIDs`；`ProtocolFieldEditor.vue` `knownFieldNames`；`r28_06_json_whitelist_test.go` | 后端白名单/历史未知键测试 PASS；前端 `protocol-field-editor.spec.ts`/`nodes-view.spec.ts` 定向 PASS | 符合 |
| D4-07 | Design4 §7、§12.4、§12.6 目标检查、固定目标集合、Clash/SR/generic 生成门槛、脱敏预览、迟到响应 | `node.Check`、`assembly.CheckNodeTarget`、`linkTargetDiagnostics`、`diagnose.go`、`render_sr.go`、`render_clash.go`；`frontend NodeCheckPanel` | `go test ./internal/node ./internal/assembly ./internal/server` PASS；目标检查/门槛测试 PASS；真实客户端未执行 | 符合（自动；真实客户端未验证） |
| D4-08 | Design4 §6.5、§12.5 全部 19 manual 协议统一保存、URI 导入、Xray 来源适配 | `registry.go` 19 协议；`uri_import.go`；`normalize.go`；`assembly/load.go` `decryptNode`/Xray 字段适配 | `go test ./internal/node ./internal/assembly` PASS；`all_protocols_test.go` 与导入测试 PASS | 符合 |
| D4-09 | Design4 §10.1、§11.3、§12.1 迁移、备份恢复、配置导入密钥保护、历史快照边界 | `1017` 迁移；`config/export.go` 加密/节点快照；`backup`；`assembly` 历史 plan 兼容测试 | 全量测试 PASS；旧 plan/迁移测试 PASS；真实备份恢复与 Production 未执行 | 符合（自动/静态；真实环境未验证） |
| D4-10 | Design4 §12.7 A（首管理员）、B（Token 业务键）、D（分层）、E（Logger/包级状态）、G（导入上限）、I（SSE） | `user/user.go`/`user/oidc.go`；`token/token.go`；`userrender`、`xray.TrafficSummary`、`oidc.Ticket`；`log.Runtime`；`hardening.go`；`server/log.go` | 后端全量/race PASS；前端 `import-limit.spec.ts`、`sse-parser.spec.ts`、`logs-view.spec.ts` PASS；架构门禁 PASS | 符合 |
| D4-11 | Design4 §12.7 F 验证码明文（设计取向） | 保持现有行为 | 未实施、未记为修复 | 已排除（EX-09） |
| D4-12 | Design4 §9、§11.3 后续 15 协议完整表单、SS 2022 完整兼容、独立 Xray outbound | 设计明确为后续专项 | 不作为本次缺口 | 不适用/已排除（EX-10） |
| D4-13 | Design4 §10.3、§11.3 真实浏览器/手机/客户端人工证据 | ProdTestList | 未执行 | 证据不足（人工层） |

---

## 八、AGENTS.md 强要求符合度

| ID | AGENTS 约束 | 当前实现/证据 | 本次结果 | 判定 |
|---|---|---|---|---|
| A-01 | §4.1 输入与路径安全、资源标识格式、上传大小/流式 | `static.go:52-72`、`platform.go:401-419`、`platform.go:485-501`、`hardening.go`、`import_upload.go`；前后端上传测试 | 静态 + 定向测试通过 | 符合 |
| A-02 | §4.2 密钥/敏感配置加密、素材池/诊断/API 不保存凭据、脱敏按参数名/路径 | `config.go` `isSensitiveKey/Encrypt/Decrypt`；`node` 敏感路径加密；`redact` 包；`pool/sanitize.go` | 全量/定向测试通过；OIDC Secret 占位符问题已由 Issue17 单独跟踪 | 符合（活动项另列） |
| A-03 | §4.3 日志脱敏与 stdout/分级/JSON/Console、5xx 脱敏 | `log/log.go` `RedactHandler`/`Runtime`；`redact/redact.go`；`server/server.go` 中间件；`response.go` | 定向测试与静态审查通过 | 符合 |
| A-04 | §4.4 权限实时查库、双层管理员、下载仅 Token、不泄露存在性 | `auth/auth.go` `SessionMiddleware/AdminMiddleware`；各路由注册；`download.go` | 全量/SSE/权限测试通过 | 符合 |
| A-05 | §4.5 下载 no-store、静态可缓存路径分级 | `download.go:33-36`；`static.go` `/assets`/`/public`/SPA；`headers_test.go` | 静态 + 自动化通过 | 符合 |
| A-06 | §4.6 版本号/指针事务与写锁、原子切换、业务键原子操作 | `store.TxImmediate`、`version.CreateVersion/SwitchVersion`、`token.Resolve/Refresh`；并发测试 | race/并发测试通过 | 符合（数据库事务层） |
| A-07 | §4.7 级联删除与文件写入失败清理 | `platform.Delete`、`user.AdminService.Delete`、`custom/share/rule` 级联；版本文件删除/驱逐 | 主路径测试通过；文件系统删除失败边界见 OBS-01 | 部分符合（见 OBS-01） |
| A-08 | §4.8 错误码、成功/列表响应、下载业务错误 200 注释块 | `response.go`；各 Handler；`download.go` | 全量测试通过 | 符合 |
| A-09 | §五 所有 error 必须处理、构造注入、禁止包级可变服务、拆分与前端按需 | `server` 无直接 `DB()/TxImmediate`，架构门禁 PASS；`log.Runtime`/请求上下文；前端动态 import | **errgate FAIL：N-01；Extension 错误丢弃：N-02** | **不符合** |
| A-10 | §五 注释中文、结构化日志、无散落 `fmt.Println`、接入/业务/数据分层 | `server` 路由注册 + 服务包分层；`grep` 未发现生产 `fmt.Println`；`architecture_test` PASS | 静态/自动化通过 | 符合 |
| A-11 | §六 构建/测试命令在改动后执行 | 本次隔离副本执行 `go build`/`go vet`/`go test`/前端 test/build | 均 PASS；但 `errgate` 失败 | 部分符合（errgate 除外） |
| A-12 | §七 多阶段静态编译、单容器单端口/单卷、非 root、迁移、compose 模板 | `Dockerfile`、`docker-compose.yml.example`；迁移 0001～1018 | 静态审查通过；Docker build 未执行 | 符合（静态；Docker 未验证） |
| A-13 | §一/§二 零配置启动、业务配置入 DB、不依赖 CGO/外部 DB | `go.mod` CGO-free、`system_config`、Setup 流程 | `go build`/测试通过 | 符合 |
| A-14 | §三 不确定时提问、无明确指令不改动、构建前后检查 | 本次用户明确授权只读核验；未改代码/配置 | 流程遵守 | 不适用（流程，非代码项） |

**AGENTS 核验结论：** 安全底线、架构分层、部署静态约束总体存在；**A-09 因 N-01/N-02 不成立**，A-07 的文件删除失败边界先按观察项处理。

---

## 九、既有 BuildReport 发现项当前回归

### 9.1 BuildReport1～3（历史阶段）

| 来源 | 历史发现/结论 | 当前代码/测试复核 | 回归结论 |
|---|---|---|---|
| BuildReport1 | Build4/Build5 事后验收、Build6/Build7 预检、Issue2 R14 | 相关服务/测试当前存在；全量 `go test ./...` PASS | 未发现当前回退（未逐条重放旧人工项） |
| BuildReport2 | Build8～10 验收、smoke 脚本问题 | `.smoke-test.sh` 静态存在，旧布尔/覆盖样本修复点未在本次发现回退；未执行 smoke | 静态未见回退；运行证据不足 |
| BuildReport3 | Build17～20 表单结构 P0/P1/P2 | Build21 已修复；当前 `node`/`nodes-view`/`protocol-field-editor` 测试 PASS；真实浏览器未执行 | 自动化未见回退；人工层证据不足 |

### 9.2 BuildReport4 未闭环项回归

| 原编号 | 当前证据 | 判定 |
|---|---|---|
| D3-1 | `pool` no_resolve/结构化解析与测试存在 | 已闭环 |
| D3-2 | `Excluded`/`Rejected`/`Duplicates` 独立计数与测试存在 | 已闭环 |
| D3-3 | 手工 origin 换绑/409 保护与测试存在 | 已闭环 |
| D3-4 | 素材池能力白名单校验与测试存在 | 已闭环 |
| D3-5 | `ParsedRule{Rule, Origin}`、`line_no/raw_line/sort_order` 与测试存在 | 已闭环 |
| D3-6 | `assembly` 零输出门槛与 server generate 测试存在 | 已闭环 |
| D3-7 | per-URL 快照/状态/诊断、1018、display_url 与 Build22 Step 7 测试存在 | 已闭环 |
| D3-8 | `PoolDetail.vue` pending/状态/诊断 UI 测试存在；真实浏览器未执行 | 自动化已闭环；人工证据不足 |
| D3-9 | `PreviewStep` 回执渲染与测试存在 | 已闭环（自动化）；人工证据不足 |
| D3-10 | 1015→1016 store 级迁移测试存在 | 已闭环 |
| N-core-1 | 首管理员只以 users 空表判定；`admin_initialized` 无生产写入 | 已闭环 |
| N-core-2 | `token.ResolveUserToken` 自定义优先并清理隐藏组 Token；定向/并发测试 PASS | 已闭环 |
| N-core-3 | 全量 errgate 当前 FAIL：`mail.go conn.SetDeadline`；SMTP Extension 错误未处理 | **部分回归**（N-01/N-02） |
| N-core-4 | `server` 生产文件无直接 `DB()`/`TxImmediate()`；架构门禁 PASS | 已闭环 |
| N-core-5 | `response.debugProvider` 已移除；`log.Runtime`/请求上下文/Store Option 注入 | 已闭环 |
| N-core-6 | 验证码明文按 R28-07F 设计取向排除 | 已排除 |
| N-node-1 | 未知 SS 插件 `plugin-opts` 保留结构化输出；测试存在 | 已闭环 |
| N-node-2 | SS 插件诊断接入；测试存在 | 已闭环（限设计范围） |
| N-node-3 | VMess SR TLS 参数输出；`links` 测试存在 | 已闭环 |
| N-node-4 | VLESS SR ALPN/fp/flow 输出；测试存在 | 已闭环 |
| N-node-5 | SS v2ray-plugin 检查告警；测试存在 | 已闭环 |
| N-node-6 | R28-06 未知扩展/局部 JSON 边界、`allow_unknown`、`item_id_field` 与测试存在 | 已闭环 |
| 安全 N01～N07 | Issue14 R28-08 用户确认整体设计取向关闭 | 已排除（EX-08） |
| smoke 脚本 | BuildReport5 记录已修复；本次未执行、仅静态核查 | 运行证据不足 |

### 9.3 BuildReport5 的当前回归

BuildReport5 基于旧 HEAD `a09dc5...`，其中以下结论在当前 HEAD 需要更新：

1. **BuildReport5 记录“构建与自动化主体完成”，但其未包含 Build26 后新增的 `errgate` 强制门禁。** 当前 HEAD 的 `go run ./cmd/errgate ./...` 失败，说明“工程静态门禁全部绿色”的旧状态已被 `2300369` 新增的 `mail.go` 代码打破。
2. **BuildReport5 记录的后端 39 包、前端 42 文件/259 用例是旧快照。** 本次当前 HEAD 隔离回归为后端 40 个有测试包全 ok（另 5 个包 no test files）、前端 46 文件/275 用例，生产构建 PASS。
3. **BuildReport5 的活跃排除矩阵中 R28-07A～I、R28-09 在当前 HEAD 已由 Build26/Build27 关闭，不再属于当前未完成工程项；R28-07C 因 N-01/N-02 仍有回归。**
4. **BuildReport5 的 ProdTestList、Issue16、SecurityScanPlan1 排除结论仍有效；此外核验期间新增 Issue17，需一并作为活跃排除源。**
5. **BuildReport5 未覆盖当前工作区的外部文档并行修改**（Issue16 v1.3、Issue17 R30-05），本报告已单列。

---

## 十、活跃排除矩阵

> 排除项不计入本报告新发现数量，也不把其未完成状态写成 Design1～Design4 的实现缺口。

| 编号 | 活跃跟踪来源 | 当前内容/观察 | 排除理由 | 后续主跟踪 |
|---|---|---|---|---|
| EX-01 | Issue16 R30-01 | Build11 邮件人工测试/重置链接四态未完成；测试邮件单项通过 | 用户指定排除的人工跟进问题 | Issue16/ProdTestList |
| EX-02 | Issue16 R30-02 | SMTP 密码占位符覆盖问题；工程修复完成，真实服务复验未完成 | 与 Design1 SMTP GET `***` 旧合同冲突已由当前授权修复处理 | Issue16/ProdTestList |
| EX-03 | Issue16 R30-03、R30-04 | SMTP TLS/STARTTLS 表单、超期、测试收件人；真实腾讯云复验未完成 | 当前跟踪的邮件交互/人工复验范围 | Issue16/ProdTestList |
| EX-04 | Issue16 第二节及 v1.3 平台方案 | Mailgun/AWS SES/SendGrid API 接入研究与待决策合同 | Issue16 明确未授权、未实施的范围 | Issue16/未来 Build/Design |
| EX-05 | Issue17 R30-05 | OIDC GET 占位符可能被保存为新 Client Secret；仅静态路径，待单独授权修复 | 核验期间新增的活跃跟踪项；已有独立 Issue，不重复登记为新发现 | Issue17 |
| EX-06 | ProdTestList 全部未勾选项 | R30-01、R26-07 等人工/浏览器/客户端/Production 项 | 用户指定排除的人工验收动作 | ProdTestList |
| EX-07 | TODOLIST 当前待办 | Issue16 R30-01、R26-07 | 与 EX-01/EX-06 重叠的人工项 | TODOLIST |
| EX-08 | Issue14 R28-08 | SecurityReport2/3 N01～N07：依赖/备份加密/验证码 fail-open/重置与 JWT/安全头 CSP/OIDC 首设密码通知 | 用户 2026-09-09 确认为设计取向并关闭工程整改 | Issue14（已归档） |
| EX-09 | Issue14 R28-07F | 验证码 Site/Secret Key 明文存储/回显 | 用户确认设计取向；不得记为已修复 | Issue14（已归档） |
| EX-10 | SecurityScanPlan1/SecurityReport3 | 第三期安全扫描 Step 4～28 未开始 | 独立安全扫描任务，非 Design1～4 构建缺口 | SecurityScanPlan1 |
| EX-11 | Design5.md | 加密整站导出/Setup 一步迁移候选 | 未研究定稿、未创建 Build、未实施 | Design5 |
| EX-12 | Design4 §9/§11.3 后续专项 | 其余 15 协议完整条件表单、SS 2022 完整 URI/密钥/客户端兼容、独立 Xray outbound | 设计明确后置，不作为首批当前缺口 | 后续 Design/Build |


---

## 十一、本次实际执行的命令、结果与未执行边界

### 11.1 实际执行的只读/隔离命令

| # | 工作目录 | 命令 | 结果 |
|---:|---|---|---|
| 1 | 原仓库根 | `git branch --show-current && git rev-parse HEAD && git show -s --format='%ci %s' HEAD && git status --short` | `beta` / `2300369...` / 2026-09-14 11:54:37 / 起始无输出 |
| 2 | `/tmp` | `rsync -a --exclude='.git' <仓库>/ /tmp/vpnsub-audit.zA2Ird/` | 创建隔离副本；未修改原仓库 |
| 3 | 副本 `backend/` | `GOCACHE=/tmp/vpnsub-audit-gocache GOTMPDIR=/tmp/vpnsub-audit-gotmp go build ./...` | 退出码 0 |
| 4 | 副本 `backend/` | `go vet ./...` | 退出码 0 |
| 5 | 副本 `backend/` | `go test ./... -count=1 -timeout 300s` | 退出码 0，40 个有测试包 ok，另 5 个包 `[no test files]` |
| 6 | 副本 `backend/` | `go test -race ./internal/user ./internal/oidc ./internal/server ./internal/version ./internal/xray ./internal/token -count=1 -timeout 300s` | 退出码 0，无 DATA RACE |
| 7 | 副本 `backend/` | `go test ./internal/server -run 'Architecture\|NoDirect\|Store' -count=1 -v` | 退出码 0，`TestArchitectureNoDirectStore` PASS |
| 8 | 副本 `backend/` | `GOCACHE=/tmp/vpnsub-audit-gocache go run ./cmd/errgate ./...` | **退出码 1**，遗漏 error：`internal/mail/mail.go (*Service).Send [assign] conn.SetDeadline(deadline)` |
| 9 | 副本 `frontend/` | `npm test -- --run` | 退出码 0，46 文件/275 用例全通过 |
| 10 | 副本 `frontend/` | `npm run build` | 退出码 0，`vue-tsc -b` + Vite 构建成功 |
| 11 | 副本 `frontend/` | `npx vitest run tests/style-tokens.spec.ts tests/import-limit.spec.ts tests/sse-parser.spec.ts tests/logs-view.spec.ts` | 退出码 0，4 文件/14 用例 |
| 12 | 当前原仓库 | `node scripts/check-md-links.mjs <全部 md>` | 退出码 1，921 检查/97 缺失（含模板占位符与归档历史路径） |
| 13 | 当前原仓库 | `git diff --check` | 退出码 0，无空白错误 |

### 11.2 未执行项目及原因

| 验证项 | 未执行原因 | 不得写作 |
|---|---|---|
| `docker compose build` / `docker build` | 用户要求不得干扰正在运行的服务/容器；本次未确认镜像构建对当前环境的隔离安全条件 | “Docker 通过” |
| `docker compose up` / Production smoke | 需要全新隔离数据库、端口和持久化环境；用户明确禁止触及现有持久化数据与控制服务 | “Production 通过” |
| `.mihomo-test.sh` | 需要固定版本 Mihomo 1.19.29 二进制/环境；本次未配置；未执行真实连接 | “固定客户端门禁通过” |
| 真实浏览器 UI / 手机 | 无安全隔离的真实浏览器、手机与 Production 环境 | “浏览器通过/手机通过” |
| 真实 Clash Verge/Shadowrocket/客户端导入与连接 | 无真实客户端、真实凭据与真实服务器 | “客户端兼容通过/连接通过” |
| 真实 SMTP/OIDC/Xray 连接 | 未提供隔离真实服务与授权测试范围；用户排除当前邮件/OIDC 活跃项 | “真实服务通过” |
| ProdTestList 人工项 | 用户明确排除人工核验动作与结果等人工验收 | “人工验收通过” |
| `go test ./...` 在原仓库直接运行 | 为避免在正在使用的工作区生成任何测试副产物；改为隔离副本执行 | 已执行（副本） |

### 11.3 证据边界声明

- 本报告所有 `PASS` 均来自上表可复跑命令或可定位代码；未执行项目已显式标注，不以历史 Build 证据替代当前运行层。
- 隔离副本与原仓库代码内容一致（HEAD/工作区无代码差异；副本排除 `.git`），但副本于外部文档修改前创建；外部修改仅涉及 `Issue16.md`/新增 `Issue17.md`，不影响后端/前端代码测试结果。
- 真实浏览器、真实手机、固定版本客户端、真实连接和用户人工验收仍为空白证据层。

---

## 十二、新发现

### N-01：当前 HEAD 的 `errgate` 静态错误门禁失败（确定性）

- **触发条件：** 在 `backend/` 执行 `go run ./cmd/errgate ./...`。
- **预期：** Build26 Step 13/Step 20 记录 `errgate` 基线清零，输出 `OK (0 ignored errors ...; 0 unexpected)`；AGENTS.md §五要求所有 error 被处理。
- **实际：** 退出码 1，输出 `internal/mail/mail.go (*Service).Send [assign] conn.SetDeadline(deadline)`；`backend/errgate_baseline.json` 当前为空数组。
- **影响范围：** 工程静态门禁在当前 HEAD 不通过；Build26 的“errgate 清零”结论在当前 HEAD 失效；`SetDeadline` 失败被静默忽略，虽有 `context.AfterFunc` 30 秒关闭连接兜底，但不符合“所有 error 必须处理”。
- **准确定位：** `backend/internal/mail/mail.go:97-101`；`backend/errgate_baseline.json`；`docs/reports/Build/Build26.md` Step 13/Step 20 的 errgate 验收记录。
- **可复核证据：** 本次实际命令与输出（退出码 1）；错误类型由 `errgate` 基于真实类型信息判定为 error 返回被赋给空白标识符；该代码由 HEAD `2300369` 的邮件改动引入。
- **严重程度：** 中。运行时影响受 30 秒上下文取消限制，主要影响是强要求/门禁失败，而非立即功能故障。
- **为什么不属于本次排除范围：** Issue16 当前跟踪的是 SMTP 人工行为、密码占位符、TLS 表单/收件人/平台研究；Issue17 跟踪 OIDC Secret 占位符；两者均未登记 `SetDeadline` 被忽略这一工程错误。ProdTestList 是人工项；SecurityScanPlan1 是独立安全扫描；均不覆盖本项。

### N-02：SMTP `client.Extension` 错误被 comma-ok 分支丢弃（确定性静态缺口）

- **触发条件：** 使用 `security=legacy` 或 `security=starttls` 发送邮件，且 SMTP 服务器在读取 EHLO/STARTTLS 扩展结果时发生网络/协议错误，使 `client.Extension("STARTTLS")` 返回 `err != nil`。
- **预期：** 按 AGENTS.md §五“所有 error 必须处理”，至少应返回结构化错误或按 Build26 用户决策 9 对合法 fail-safe 读取增加 warn/allow 理由；在拿不到可靠扩展结果时不得把失败当成“服务器明确未提供 STARTTLS”。
- **实际：** `if ok, _ := client.Extension("STARTTLS"); ok` 丢弃了 error。`legacy` 模式在 `ok=false` 时继续执行后续 `Auth/Mail/Rcpt/Data`，可能继续使用未升级连接；`starttls` 模式会返回“服务器未提供 STARTTLS，已停止发送”，把读取错误误报为能力缺失。
- **影响范围：** SMTP 认证/邮件发送的传输安全下降路径和错误诊断准确性；`errgate` 按设计不检查 comma-ok bool，因此该错误不会被当前门禁发现。
- **准确定位：** `backend/internal/mail/mail.go:108-116`；现有 `TestStartTLSRequired` 只覆盖服务器明确未宣告 `STARTTLS` 的场景，未覆盖 `Extension` 读取错误。
- **可复核证据：** 静态代码路径；`go test ./internal/mail` 通过但未覆盖该分支；`go test ./...` 不失败是因为 errgate 明确排除 comma-ok bool，而非该错误已被处理。
- **严重程度：** 中低。实际触发需要网络/协议读错误；一旦触发，legacy 模式可能继续未升级连接，starttls 模式返回错误原因不准确。
- **为什么不属于本次排除范围：** Issue16 R30-03 的已知问题是“表单混淆端口/加密方式、超时与投递验证”，其修复要求是“不得把未升级 TLS 误报为加密”；本项是 `Extension` 读取错误被丢弃的独立错误处理缺口，没有在 Issue16/Issue17/ProdTestList 登记，也没有 `errgate:allow` 理由。

> **说明：** 本报告不把版本文件失败清理路径列为“确定性新发现”，因其与 Build2/Build26 已记录的清理语义和用户对 `os.Remove` 的允许理由存在交叉，且本次未进行故障注入；将其列在 OBS-01/待确认。

---

## 十三、观察项、文档冲突与待用户决策

### 13.1 观察项

#### OBS-01：版本文件删除/驱逐失败清理存在非事务边界（待确认）

- **观察：** `version.DeleteVersion` 先删除 DB 记录，再 `os.Remove` 文件；失败只 `Warn` 不阻断（`backend/internal/version/version.go:402-408`）。`evictOldest` 同样在事务内删 DB 行后 `os.Remove`，失败只 warn（`version.go:274-288`）。`CreateVersion` 在 `evictOldest` 返回错误或事务提交失败时，未统一清理已写文件/已删文件，可能出现“文件已删、DB 记录回滚”或“DB 记录已删、文件残留”。
- **影响：** 极端失败条件下可能出现孤儿文件或悬空版本记录，违反 AGENTS §4.7“不留孤儿/失败完整清理”的目标。
- **待确认理由：** Build2/Build26 已记录 `os.Remove` 的失败语义属于明确清理/无业务错误；用户决策 6/9 允许该语义不被 errgate 报告。但“事务回滚后文件已删/新版本文件残留”的具体失败路径未见现有跟踪项，也未见故障注入测试。
- **建议后续：** 在获得授权后以故障注入测试确认，再决定是否更新设计/缺陷记录；本报告不实施修复。

#### OBS-02：全仓 Markdown 内链检查有 97 条缺失

- **证据：** 本次对当前工作区 108 个 Markdown 文件运行 `scripts/check-md-links.mjs`：`checked_links=921 missing=97 files=108`。
- **分类：** 大部分为模板占位符（`BuildN.md`/`DesignN.md`/`IssueN.md`）、Issue14 归档后仍指向根目录 `Issue14.md` 的历史相对路径、以及指向已删除 `backend/internal/server/render.go` 的 Reference 文档路径。并非全是现行用户文档缺陷。
- **影响：** 历史归档/模板链接可读性下降，影响审计追溯；不构成本次 Design1～4 功能缺口。
- **相关位置示例：** `docs/reports/Build/Build21.md`～`Build27.md` 中的 `../../../Issue14.md`；`docs/Reference/Node-Editor-Research.md:496`；`docs/reports/Issue/Issue15.md` 的多个相对链接。

#### OBS-03：AGENTS.md 中关于 errgate 的状态文字仍过时

- **位置：** [AGENTS.md](../../../AGENTS.md) 第 255 行仍写 Build26 “errgate 与架构/颜色门禁清零”。
- **当前事实：** 核验开始后外部并行修订了 AGENTS.md（新增 Issue17 入口并修正 Issue14 当前状态）；但当前 HEAD 的 errgate 失败（N-01），该行状态文字仍与实测不符。
- **影响：** 强要求文档的入口状态不准确，可能误导后续 AI/构建者。该修正属于外部/后续文档变更，本报告不实施。

#### OBS-04：代码注释/死代码残留

- **`backend/internal/server/settings_ops.go:84`** 注释仍写“multipart 文件不设大小上限”，与 Build26/Design4 §12.7 G 的 20 MiB/21 MiB 合同不一致。
- **`backend/internal/server/rulespec.go:22-23`** 保留 `var _ = http.StatusOK` 与“防止未来重构误删注册入口”注释，属无语义死代码。
- **影响：** 不直接影响功能，增加审计歧义。

#### OBS-05：核验期间外部并行修改了文档工作区

- 起始状态干净；约 12:18 原仓库出现 `M AGENTS.md`、`M Issue16.md` 与 `?? Issue17.md`。本报告操作只新建 `BuildReport6.md`，未修改这些文件；无法代表外部进程/用户确认其内容完整性。该变化已纳入本报告“活跃排除”与最终工作区声明。

### 13.2 文档冲突与待用户决策

#### DC-01：密码重置令牌“用后即删”与“used 四态”口径冲突

- **Design1 原文：** Design1 §4.6“一次性令牌，1 小时有效，**用后即删**”；§4.2 也是“用后即删”。
- **当前实现/活跃记录：** `backend/internal/auth/reset.go` 使用 `used=1` 保留记录；`ProdTestList.md` §1 与 Issue16 R30-01 要求 `valid/missing/used/expired` 四态，且已使用链接不得渲染表单。`Issue16.md` 当前为外部 v1.3。
- **影响：** “四态可区分”必须保留已用记录（或等价 tombstone），与“用后即删”字面冲突。若按 Design1 字面删除，前端无法区分 used 与 missing。
- **待用户决策点：** 由用户确认是更新 Design1 口径为“用后不可再用，记录可保留用于状态区分”，还是调整实现/验收四态定义。本报告不自行选边，也不实施。
- **排除说明：** 该人工四态核验本身已由 ProdTestList/Issue16 跟踪，本报告不把未完成人工项计为新缺口；此条只登记文档合同冲突。

#### DC-02：SMTP 配置 GET 的脱敏合同与当前实现冲突

- **Design1 原文：** Design1 §3.4.8 描述 SMTP 密码“加密存储”、面板回显 `***`（以及“空=不修改”旧语义）。
- **当前实现/活跃记录：** `config/admin.go` 的 `GetSMTP` 当前返回 `Password: ""` + `PasswordConfigured`，并在保存时拒绝字面 `***`；`Issue16.md` R30-02 记录此前的 `***` 覆盖缺陷与修复。
- **影响：** Design1 文档仍是旧合同；若后续构建按 Design1 旧文实现，会重新引入密码覆盖风险。
- **待用户决策点：** 由用户决定何时把该现行合同写入后续 Design/AGENTS 状态文档。当前修复已在 Issue16 单独授权下实施，本报告按排除项 EX-02 处理，不重复登记新缺口。

#### DC-03：AGENTS.md 的 errgate 状态文字与当前实测不一致

- **冲突位置：** AGENTS.md 第 255 行称 Build26 “errgate 与架构/颜色门禁清零”；当前 HEAD 实测 `errgate` 失败，且 Issue14 归档记录中的 Step 13/20 结论也被当前 HEAD 的后来代码打破。
- **影响：** 入口状态不准确，可能影响后续构建/审计；Issue14 相关状态文字已由核验期间外部修订处理。
- **待用户决策点：** 由用户决定何时更新 AGENTS.md 的状态文字（或修复 N-01 后回归）；本报告不修改强要求文档。

---

## 十四、证据层级完成情况与无法证明的范围

| 证据层级 | 本次完成情况 | 可证明的内容 | 不能证明的内容 |
|---|---|---|---|
| 静态源码/配置/迁移 | 完成 | Design1～4 主路径代码、AGENTS 约束位置、排除项边界 | 运行正确性、性能、真实兼容性 |
| 自动化测试 | 完成（隔离副本） | 后端 40 个有测试包 ok（另 5 个 no test）、前端 46 文件/275 用例、race、构建、部分 API 行为 | 真实数据库/Production/浏览器/客户端行为 |
| 隔离运行/API | 部分（测试内 httptest/临时 SQLite/mock SMTP） | Handler/中间件/迁移/装配/池/Xray mock 行为 | 独立常驻服务、真实端口、真实网络 |
| Docker/容器 | 未执行 | — | 镜像构建、运行、非 root、volume、healthcheck 实际行为 |
| Production smoke | 未执行 | — | 全新 Production 库、导入导出、实际响应头 |
| 真实浏览器/手机 | 未执行 | — | 响应式、焦点、视觉、人工交互、真实下载 |
| 固定版本客户端/真实连接 | 未执行 | — | Mihomo/CVR/Shadowrocket 实际导入/连接 |
| 用户人工验收 | 未执行 | — | ProdTestList、Issue16/Issue17 当前人工结论 |
| 静态错误门禁 | 完成但失败 | 当前 HEAD 存在 N-01 | 不能宣称 errgate 清零 |
| Markdown 内链 | 完成但失败 | 文档内链存在 97 条缺失 | 不能宣称全仓文档链接完整 |

**无法证明的范围摘要：** 所有真实服务、真实连接、真实客户端、真实浏览器/手机、人工验收和 Docker/Production 运行层均不能由本报告证明；只能证明当前代码路径、隔离自动化测试和静态结构。

---

## 十五、最终判定

### 15.1 代码实现符合度

**中高，但不能判定为全部符合。**

- Design1 第一期主路径、Design2/Design3 主体、Design4 首批四协议与统一节点编辑契约，在当前代码和隔离自动化中均有实现证据。
- BuildReport4 的 D3-1～D3-10、N-node-1～6 未发现回退；N-core-1/2/4/5/6 已闭环或按设计取向排除。
- **N-01/N-02 使当前 HEAD 不能通过“AGENTS §五所有 error 必须处理”和 Build26 errgate 门禁。**
- OBS-01 的版本文件失败清理边界仍待故障注入/设计确认。

### 15.2 自动化验证结果

**主体通过，但存在门禁失败。**

- `go build ./...`、`go vet ./...`、`go test ./...`（40 个有测试包 ok，另 5 个包无测试）、关键包 race、前端 275 用例、前端生产构建均通过。
- `errgate` 失败（N-01），因此“全部自动化质量门禁通过”不成立。
- 全仓 Markdown 链接检查失败（97 条缺失），但它主要反映模板/归档历史文档链接问题，不作为 Design1～4 功能缺陷计数。

### 15.3 仍待人工确认

- ProdTestList 的 R30-01 邮件重置链接四态、R26-07 Xray PUT 错误码与前端提示。
- Issue16 R30-02～R30-04 的改动后真实腾讯云/服务商复验、Issue17 R30-05 的 OIDC 修复决策。
- 真实 Docker/Production smoke、浏览器/手机、固定版本客户端导入与真实连接。
- 这些人工项本次一律未执行，不能由本报告判定通过或失败。

### 15.4 是否可以直接宣布 Design1～Design4 全部完成

**不能。**

理由：
1. `errgate` 在当前 HEAD 失败，存在确定性工程回归 N-01；
2. SMTP `client.Extension` 错误处理存在确定性静态缺口 N-02；
3. 版本文件失败清理边界未由测试证明（OBS-01）；
4. Docker/Production/真实浏览器/手机/固定客户端/真实连接/用户人工验收未执行；
5. Issue16/Issue17 活跃项仍由各自文档跟踪，不能提前关闭。

### 15.5 后续最小动作建议（不实施）

1. 由用户决定是否授权修复 N-01（处理或显式 allow `SetDeadline`）与 N-02（处理 `Extension` 错误），并重新执行 `go run ./cmd/errgate ./...`。
2. 由用户决定 OBS-01 版本文件失败清理是否进入独立缺陷/设计决策；在未确认前不要把它写成“已修复”。
3. 由用户决定 DC-01～DC-03 的文档合同更新时机。
4. 在隔离 Production/浏览器/手机/客户端环境中执行 ProdTestList 对应人工项并保留独立证据。

---

## 十六、变更记录与操作声明

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-14 | 首次创建：对当前 HEAD 的 Design1～Design4、AGENTS.md、BuildReport1～5 回归、活跃排除矩阵、命令证据、证据层级和最终判定进行全量只读核验；记录 N-01/N-02 新发现、OBS/DC 待确认项。 |
| v1.1 | 2026-09-14 | 追加“用户决策确认记录”：通过 `ask_user_question` 确认 N-01/N-02 修复、OBS-01 best-effort 设计说明、DC-01/DC-02 文档同步、DC-03 修复后更新 AGENTS、OBS-02 有限内链修复、OBS-04 后续清理、OBS-05 保持外部改动、Issue17 保持独立跟踪、人工项暂不安排。未实施代码或报告外文档修改。 |

### 本次操作声明

- **仅新建：** `docs/reports/BuildReport/BuildReport6.md`。
- **本审计未修改：** 任何代码、测试、配置、脚本、数据库迁移、设计文档、Build 文档、Issue 文档、TODOLIST、ProdTestList、安全报告或其他既有文件。
- **未提交、未重置、未清理工作区、未改动运行中服务。**
- **工作区外部变化声明：** 核验开始后外部并行出现 `M AGENTS.md`、`M Issue16.md` 与 `?? Issue17.md`；这些不是本审计产生的修改，本审计也未触碰它们。因此当前 `git status --short` 不能呈现为“只有本报告一个文件”，而是同时含上述外部变化。
- **本报告自身检查：** 写入后仅对新报告内容、链接和 Markdown 格式进行检查，并执行 `git diff --check` 与 `git status --short`；检查结果见报告开头的执行摘要和本次操作声明。
- **`git diff --check`：** 本次执行通过（无空白错误）。
- **`git status --short`：** 见本报告核验时的外部变化说明；新报告创建后状态会额外包含 `?? docs/reports/BuildReport/BuildReport6.md`。


---

## 十七、用户决策确认记录（2026-09-14）

> 本节通过 `ask_user_question` 工具向用户确认后续处置方向；以下仅为决策输入记录。截至本报告写入时，未实施任何代码或报告外文档修改，当前 HEAD/工作区实现状态仍以第十六章为准。

| 决策项 | 用户选择 | 后续含义 |
|---|---|---|
| N-01 / N-02 错误处理 | **修复两项并重跑 errgate** | 后续需在获得实施窗口后修改 `mail.go` 的 `SetDeadline` 与 `Extension` 错误处理并补测试，重新执行 `go run ./cmd/errgate ./...`。 |
| OBS-01 版本文件失败清理 | **接受 best-effort 并写入设计说明** | 后续需在相应设计/说明中明确 `os.Remove` 失败语义和“不宣称无孤儿/完整回滚”，本次未改代码。 |
| DC-01 重置令牌四态 | **确认现行四态并更新设计文字** | 后续需把“用后失效、保留状态记录以区分 used/missing”写入设计合同；本次未改 `Design1.md`。 |
| DC-02 SMTP GET 脱敏合同 | **确认当前安全合同并同步设计文档** | 后续需把“空密码 + `password_configured`、拒绝 `***` 落库”写入设计/状态文档；本次未改其他文档。 |
| DC-03 AGENTS errgate 状态 | **先修复 N-01，再更新 AGENTS** | 修复并重跑门禁后，再同步 AGENTS 第 255 行；不在门禁未通过时写成已清零。 |
| OBS-02 Markdown 内链 | **只修复真实失效的归档/现行链接** | 后续需先分类 97 条缺失项，跳过模板占位符，只修归档迁移造成的实际断链；本次未改文档。 |
| OBS-04 注释与死代码 | **并入下一次获授权的代码/文档收尾** | 后续与真实代码改动一起清理 `settings_ops.go` 旧注释和 `rulespec.go` 无常量；本次未改代码。 |
| OBS-05 外部并行改动 | **确认预期，保持不动** | `AGENTS.md`、`Issue16.md` 的修改和 `Issue17.md` 的新增按外部工作保留，本报告不覆盖、不回退。 |
| Issue17 R30-05 OIDC Secret | **先只做静态复核，不修复** | 继续由 Issue17 独立跟踪，不在本次授权范围内修改 OIDC 代码或配置。 |
| ProdTestList 人工核验 | **暂时不安排** | R30-01、R26-07 等人工项继续挂起；项目不能据此宣称人工验收完成。 |

**边界说明：** 以上决策不改变第十六章“当前代码实现符合度、自动化验证结果、仍待人工确认”三者的分离结论；后续实施需要新的授权/构建/问题记录承载，不得在本只读核验中直接展开。

