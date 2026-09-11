# TODOLIST.md — 后续工作顺序跟踪（2026-09-11）

> **性质：** 本文件是临时顺序清单，不替代 [Issue14.md](Issue14.md)、[Issue15.md](Issue15.md)、[Build26.md](Build26.md)、[Build22.md](docs/reports/Build/Build22.md)、[SecurityScanPlan1.md](SecurityScanPlan1.md)、[SecurityReport3.md](SecurityReport3.md)、[Design3.md](docs/reports/Design/Design3.md)、[Design4.md](Design4.md) 与 [ProdTestList.md](ProdTestList.md) 的正式状态和验收记录。
> **授权边界：** 本次只授权同步 Build26/TODOLIST/Issue14/AGENTS 的文档指向，不授权执行 Step 1～20 的代码修复、测试、归档或正式验收状态变更。进入各 Step 前仍须按 AGENTS.md 完成影响评估、文档疑点检查并取得对应授权。
> **排序原则：** 先关闭已有活跃 Build 的验收缺口，再处理 Issue14 的工程整改和项目收尾；工程基线冻结后继续第三期安全审查；人工结论单独由 ProdTestList 跟踪。

---

## 0. 活跃文档快照

| 文档 | 当前状态 | 下一入口 |
|---|---|---|
| [docs/reports/Build/Build22.md](docs/reports/Build/Build22.md) | 已归档；Step 1～11 全部验收通过，D3-1～D3-10 闭环 | ✅ 已完成 |
| [docs/reports/Design/Design3.md](docs/reports/Design/Design3.md) | 已随 Build22 收口归档 | ✅ 已完成 |
| [Build26.md](Build26.md) | **活跃构建记录**；Step 0 已完成，Step 1～20 未开始，等待逐 Step 授权 | **Build26 Step 1** |
| [Issue14.md](Issue14.md) | 步骤一、二、三、四、六已关闭；步骤五已创建 Build26/Step 0，代码实施未开始；步骤七、八未关闭 | **Issue14 步骤五 / Build26 Step 1** |
| [Issue15.md](Issue15.md) | R29-11 已关闭；R29-12 已完成；R29-01、R29-06、R29-09、R29-10 待人工复验 | R29-01 与 R28-07G 联动 |
| [Design4.md](Design4.md) | 当前最新设计；Build17～25 主体已完成 | 仅在实际变更影响其合同时同步 |
| [ProdTestList.md](ProdTestList.md) | 保留 Production、浏览器、真机和真实客户端人工项 | 按工程前置分批执行 |
| [SecurityScanPlan1.md](SecurityScanPlan1.md) / [SecurityReport3.md](SecurityReport3.md) | Step 1～3 已完成；Step 4～28 未开始，报告未完成 | 工程冻结后执行 **Step 4** |

已归档的 Build21、Build22、Build23、Build24、Build25 只用于核查，不再作为执行入口；当前执行入口为根目录 [Build26.md](Build26.md)，Step 1 尚未授权。

---

## 1. 推荐主线顺序

### P0 — 开工前确认

- [x] **P0-1｜Issue15 R29-12：** 已确认 `Dockerfile` 从 `node:22-alpine` 升至 `node:24-alpine` 是有意的安全性更新；保留该变更。验证边界限定为前端构建镜像、依赖安装和相关构建/测试链，不将其表述为业务运行时行为变更。结论已回写 [Issue15.md](Issue15.md)，供 Issue14 步骤七使用。
- [x] **P0-2｜Issue15 R29-01：** 已完成实施前复核：Setup 新库导入只发送 `IMPORT`；管理端已有库导入继续要求 `IMPORT → DISABLE`；2026-09-11 用户截图和当前代码确认报告中的 `RESET` 为历史误记，当前合同为 `DISABLE`。复核、修复和隔离 Production 真实文件证据已记录在 [Issue15.md](Issue15.md)，当前 Docker 镜像重建后的浏览器复验仍由 [ProdTestList.md](ProdTestList.md) 跟踪。

Build22 Step 7 已收口，不阻塞后续主线；P0-1、P0-2 已完成。P0-2 的后续 R28-07G 工程实施仍须按 P2-7 单独授权和验收，不能以本次复核替代。

### P1 — Build22 Step 7 → Step 11 → Issue14 步骤三（已完成）

主跟踪：Build22 Step 7、Issue14 步骤三/R28-05、Issue15 R29-11。

- [x] **P1-1｜Build22 Step 7：** 补 `ActivatePending` / `DiscardPending` 与 `activated_at` 测试：激活原子更新、不改 stats/decision、历史 active/failed 不补造时间。
- [x] **P1-2｜Build22 Step 7：** 补 `SanitizeStoredSyncOutputs` 幂等、非破坏性清洗测试。
- [x] **P1-3｜Build22 Step 7：** 补 `/sync/status`、`/sync/tasks`、`Pool.sync_error` 读时脱敏 raw JSON 测试，并证明编辑用 URL 保留原值。
- [x] **P1-4｜Build22 Step 7：** 补 `NormalizeDiagnostics` 19 条真实 + 1 条截断摘要、200 rune 限额和空数组合同。
- [x] **P1-5｜Build22 Step 7：** 补 v1 `rule_counts` 合计不变量、`previous_active`、稳定 reason/evidence codes 与旧 stats `version 0` 兼容测试。
- [x] **P1-6｜Build22 Step 7：** 补 `latest_failed` 恢复、失败后成功不永久标红、同时间戳按 ID 稳定选择测试。
- [x] **P1-7｜Build22 Step 7：** 补 failed 快照写失败时指针不变、不虚报 snapshot ID、详细 DB error 不进入展示字段的测试。
- [x] **P1-8｜Build22 Step 7：** 补旧字符串数组 Clash plan 用户下载重渲染回退测试。
- [x] **P1-9｜Build22 Step 7 验收：** 跑定向测试、后端 build、前端 build，逐条核对专属证据矩阵；不能只以全量绿灯替代。
- [x] **P1-10｜Build22 Step 11：** 重跑后端全量/build/vet/相关 race、前端全量/build、Docker build、正式 Production smoke、`git diff --check`。
- [x] **P1-11｜Issue14 步骤三：** 回写 R28-05、Issue15 R29-11、Build22、Design3、AGENTS；满足条件后归档 Build22 与 Design3。

> 2026-09-11 完成记录：P1-1～P1-11 已全部执行；额外修复 `RedactDisplayURL` 编码分隔符/嵌套 URL 脱敏、`SanitizeStoredSyncOutputs` 事务化与 fallback 限额、`isLegacyClashPlan` 字符串 `rules` 识别。后端定向/全量/race/build/vet、前端 42 文件/259 用例/build、Docker build、Production smoke、`git diff --check` 通过；Build22/Design3 已归档，Issue14 步骤三与 Issue15 R29-11 已关闭。

### P2 — Issue14 步骤五（R28-07）

主跟踪：Issue14 步骤五/R28-07A～E、G～I；R28-07F 已确认为设计取向，不改代码。

> 先新建或确认专门 Build 文档，把下列内容拆为可独立验收的 Steps；不得把本清单当作 Build 手册。具体 Step 编号、前置、影响评估、失败优先测试和验收命令以 [Build26.md](Build26.md) 为准。
>
> **当前进展（2026-09-11）：** 用户已确认先创建完整 Build26 文档；[Build26.md](Build26.md) 已创建并完成 Step 0（范围、决策、Step 1～20、静态门禁和最终联合门禁冻结）。用户确认 E 先于 C、G 采用 21 MiB 请求体上限、I 采用流内每 15 秒权限重查、D1 使用 `internal/userrender` 独立包等；P2-1 保持未完成，Step 1 尚未授权执行。本清单保留为上层顺序索引，具体逐 Step 状态以 Build26 为准。

- [ ] **P2-1｜步骤五前置：** 建立失败优先回归、架构/静态门禁，并冻结各 Build Step 的范围、回滚边界和验收命令。
- [ ] **P2-2｜R28-07A：** 删除首管理员初始化冗余标记写入；覆盖密码注册、OIDC、后续用户和并发首建。
- [ ] **P2-3｜R28-07B：** 修正自定义订阅与隐藏组 Token 创建/清理顺序，实现业务键幂等协调；覆盖历史双 Token 和并发。
- [ ] **P2-4｜R28-07D：** 将用户下载渲染、流量汇总、OIDC 换票事务移出接入层；增加禁止 `internal/server` 直访 `DB()` / `TxImmediate()` 的检查。
- [ ] **P2-5｜R28-07C：** 全量审计生产代码忽略的 error，按回滚、补偿日志、异步状态写入分类处理，补失败注入和静态门禁。
- [ ] **P2-6｜R28-07E：** 去除运行期可变包级状态，改为实例注入或不可变规则；补多实例和 race 回归。
- [ ] **P2-7｜R28-07G：** 固定双导入入口 20 MiB 文件 / 21 MiB 请求体上限，覆盖边界值、超 1 字节、分块、伪造长度、截断读取、413 和前端提前拒绝；仅保护 Issue15 R29-01 既有 IMPORT/DISABLE 语义，不顺带处理其问题状态或人工复验。
- [ ] **P2-8｜R28-07I：** SSE 改为管理员路由 + `fetch`/`ReadableStream` Bearer 鉴权，移除一次性查询 Token；覆盖 401/403、权限变化、分帧、重连、卸载。
- [ ] **P2-9｜R28-07H：** 清理 `NodeCheckPanel.vue` 遗留 gray/white 类，补静态扫描与双主题断言。
- [ ] **P2-10｜步骤五验收：** 跑后端定向/全量/race/build/vet、前端定向/全量/build、接口级 401/403/413、Production smoke、`git diff --check`，同步受影响文档。

### P3 — Issue14 步骤七（R28-09）

前置：Issue14 步骤三、四、五、六全部关闭。

- [ ] **P3-1｜步骤七/R28-09：** 按 P0-1 已确认的 Node 24 记录继续处理；安装/验证 `ca-certificates`，评估并固定基础镜像/GHCR digest。
- [ ] **P3-2｜步骤七/R28-09：** 补 `LICENSE` 或修正 README 许可证描述；涉及授权选择时先请用户决策。
- [ ] **P3-3｜步骤七/R28-09：** 修复 `docs/Reference/Xray-Server-Config-Research.md` 的仓库外失效链接。
- [ ] **P3-4｜步骤七/R28-09：** 逐项确认 `GenerateStep.vue`、`PreviewState.vue`、`ResponsiveCollection.vue`、`CopyField.vue` 无引用后再清理。
- [ ] **P3-5｜步骤七验收：** 跑受影响构建、镜像、链接和静态扫描门禁，回写 R28-09、Build 与 AGENTS。

### P4 — Issue14 步骤八（全量复核与关闭）

- [ ] **P4-1｜步骤八：** 跑后端全量/race/build/vet、前端全量/build、正式 Production smoke、`git diff --check`。
- [ ] **P4-2｜步骤八：** 核对 Issue14、Issue15、Build22、Design3/4、ProdTestList、AGENTS 的状态、链接和证据边界。
- [ ] **P4-3｜步骤八：** 确认每个 R28 项有关闭证据或明确后继文档后，关闭并归档 Issue14。

### P5 — SecurityScanPlan1 Step 4～28

> 建议在 P1～P4 后执行，减少基线漂移。必须串行，每次仅一个 Step 进行中；每步向 SecurityReport3 追加证据胶囊和交接摘要，审查阶段不直接修业务代码。

- [ ] **P5-04｜Step 4：** 后端 API、前端路由与保护链全量清点。
- [ ] **P5-05｜Step 5：** 本地隔离动态环境与合成数据基线。
- [ ] **P5-06｜Step 6：** Go/npm/容器/CI 软件供应链审查。
- [ ] **P5-07｜Step 7：** 应用交付与运行时安全配置审查。
- [ ] **P5-08｜Step 8：** Setup、本地认证、注册与应急模式审查。
- [ ] **P5-09｜Step 9：** OIDC 发现、登录、绑定与换票链审查。
- [ ] **P5-10｜Step 10：** 会话、Bearer/Cookie、CSRF 与凭据失效审查。
- [ ] **P5-11｜Step 11：** RBAC、对象所有权、管理员与高级模式授权审查。
- [ ] **P5-12｜Step 12：** 下载 Token、分享、公开内容与 SSE 凭据审查。
- [ ] **P5-13｜Step 13：** 密码重置、验证码、审批与邮件安全审查。
- [ ] **P5-14｜Step 14：** SQL、命令、配置、CRLF 与解析器注入审查。
- [ ] **P5-15｜Step 15：** 前端 XSS、DOM、Markdown、URL 与第三方组件审查。
- [ ] **P5-16｜Step 16：** CSP、安全响应头、CORS、跳转与缓存策略审查。
- [ ] **P5-17｜Step 17：** 路径穿越、上传下载、静态资源与临时文件审查。
- [ ] **P5-18｜Step 18：** 配置导入导出、备份、清空与数据完整性审查。
- [ ] **P5-19｜Step 19：** 密码学、密钥派生、凭据存储与敏感数据生命周期审查。
- [ ] **P5-20｜Step 20：** SSRF、DNS rebinding、重定向与全部出站网络审查。
- [ ] **P5-21｜Step 21：** 请求限制、超时、限流、任务与小规模 DoS 审查。
- [ ] **P5-22｜Step 22：** 业务一致性、并发事务、级联、预览/生成竞态审查。
- [ ] **P5-23｜Step 23：** Xray 集成、协议参数、凭据与对账副作用审查。
- [ ] **P5-24｜Step 24：** 日志脱敏、错误处理、调试、审计与告警能力审查。
- [ ] **P5-25｜Step 25：** 匿名、公开、Setup 与异常状态黑盒验证。
- [ ] **P5-26｜Step 26：** 普通用户、管理员与跨角色黑盒验证。
- [ ] **P5-27｜Step 27：** 恶意输入、历史修复与全量验证。
- [ ] **P5-28｜Step 28：** 发现去重、风险校准、OWASP 总结和最终交付；后续修复另行进入 Issue/Design/Build 决策。
- [ ] **P5-29｜安全文档收口：** Step 1～28 全部验收，或阻塞项明确且获用户接受后，决定两份安全文档的最终状态与归档。

---

## 2. 人工测试队列

未执行项目不得标为通过；发现新问题登记到 Issue15，并记录环境、版本、入口和结果。

| 顺序 | ProdTestList 项目 | 开始条件 |
|---|---|---|
| M1 | R28-06 §A/§B/§C/§D（含 WireGuard peers、多草稿与条件隐藏） | Build25 已归档，当前可安排 |
| M2 | R29-06 节点动态表单真实运行 | 当前可安排 |
| M3 | R29-04 真实手机/Production | 当前可安排；既有浏览器核验不替代 |
| M4 | R27-05 额外尺寸/焦点/折叠；R26-07 Xray PUT 403/400/409 | 当前可安排 |
| M5 | Build22 Step 8 来源状态、Step 9 装配回执 | P1 完成后按最新基线执行 |
| M6 | R29-01 修复后 Production 新库导入与引用关系 | P2-7 完成后 |
| M7 | OIDC Mock 登录、Build11 重置链接四态 | 隔离环境具备时 |
| M8 | R20-11 重启数据保留 | 仅在明确真实可用环境执行 |

---

## 3. 后续设计候选（不并入当前收尾）

- [ ] **F1｜Design4 后续范围：** 全量 19 协议条件表单、SS 2022、独立 Xray outbound 等须先形成新 Design/Build 并经用户决策。
- [ ] **F2｜历史候选：** 非 SS 字段级 `target_evidence` 全局诊断等不得借 Issue14/15 收尾顺带实施。

---

## 4. 回写规则

1. 先在主文档分层记录事实、根因、实施、自动化和人工证据，再勾选本文件。
2. Build/Design/Issue 满足关闭条件后才能归档；归档后同步链接。
3. 每阶段执行 Markdown 本地链接检查和 `git diff --check`；代码阶段执行 AGENTS.md 对应门禁。
4. 不改相邻问题状态，不把代码完成、自动化、Production smoke、人工或真实连接证据互相替代。

---

## 5. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-09-10 | 创建临时 TODOLIST，登记当时活跃文档、确认项、工程工作流与人工队列。 |
| 2026-09-11 | 按最新活跃文档重排：以 Build22 Step 7 → Step 11 → Issue14 步骤三为首要主线；细化 Issue14 步骤五、七、八；逐项列出 SecurityScanPlan1 Step 4～28；更新人工测试开始条件与授权边界。 |
| 2026-09-11 | 完成 P1：Build22 Step 1～11 全部证据补齐并重新通过全量门禁，修复脱敏/存量清洗/旧 plan 回退边界；问题 R28-05/R29-11 关闭，Build22/Design3 归档。 |
| 2026-09-11 | 完成 P0-1/P0-2：确认 Node 22→24 为有意的安全性更新并限定验证边界；复核 Setup/管理端导入合同及 `RESET`→`DISABLE` 历史差异。P5 SecurityScanPlan1 仍作为构建之外的独立审查清单，不并入任何 Build/Design/Issue。 |
| 2026-09-11 | 同步步骤五文档指向：用户确认先创建完整 [Build26.md](Build26.md)，Step 0 已完成；TODOLIST 活跃快照、P2 入口和 Issue14 状态已同步。E 先于 C、G 21 MiB、I 15 秒权限重查等决策记录在 Build26 §一；Step 1 未授权，P2-1 仍未完成。 |
