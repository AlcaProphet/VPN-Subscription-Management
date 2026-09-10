# Issue14.md — VPN 订阅管理系统问题追踪（当前）

> **文档定位：** 本文承接已归档的 [Issue13.md](docs/reports/Issue/Issue13.md) 的 R27-09 / Build21 收口核验，并汇总 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 全量核验中仍未闭环的工程问题；只记录除用户真机人工验收之外，当前仍未完成、待处理或新发现的工程问题与验收证据缺口。用户需要亲自执行的 Production、浏览器和客户端人工测试在执行期间由 [ProdTestList.md](ProdTestList.md) 记录，人工测试中发现的问题由 [Issue15.md](Issue15.md) 记录；已完成项目从当前清单移除，结论保留在本文件和 Build21 中，当前仍待人工复验的项目继续由 ProdTestList 跟踪。
> 关联构建：[Build21.md](Build21.md) §7.11 Step 14、[Build22.md](Build22.md)（D3 实施计划）；交接说明：[Build23.md](Build23.md)；设计基线：[Design3.md](Design3.md)、[Design4.md](Design4.md)；编码约束：[AGENTS.md](AGENTS.md)。

---

## 一、当前总体结论

- **创建时间：** 2026-09-08
- **来源：** [核验 Build21 构建问题](thread://01a080de-c26e-7540-9db2-6b7b808f5eaa)、[继续 Build21 Step 14 测试](thread://01a080ce-4c83-7830-b7c2-aca36ba501b9)、[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 及当前工作区文档核对。
- **已通过：** Build21 Step 7～15 的自动化与验收证据已收口；后端全量测试、指定竞态测试、编译、`go vet`，前端 41 个测试文件 / 210 个用例和生产构建，以及固定 Mihomo 1.19.29 严格正反例门禁均有通过记录。
- **当前状态：** Build21 Step 14 的 R28-01～R28-04 工程问题、正式 Production smoke、固定 Mihomo 1.19.29 证据门禁及 PT-28-01～PT-28-05 人工项目均已完成，Step 14 已验收收口；Build22 R28-05 的 D3-1～D3-10 已完成代码与自动化验收，实际浏览器/真机项目迁移至 ProdTestList。R28-06 步骤四已由 [Build25.md](Build25.md) 完成代码、自动化、Docker build、Production smoke 和文档工程闭环；R28-07 已完成只读研究和方案确认，其中 R28-07F 经用户确认属于设计取向、不作为问题整改；R28-08 的 N01～N07 经用户确认整体属于设计取向，已从工程实施范围关闭。剩余历史人工项目见 [ProdTestList.md](ProdTestList.md)，不在本文件重复登记为缺陷。
- **当前执行步骤：** 步骤一、步骤二、步骤三、步骤四已完成代码与自动化工程验收；步骤四的浏览器、手机和真实客户端人工项目已迁移 [ProdTestList.md](ProdTestList.md)，尚未形成人工通过结论。步骤五（R28-07）仍未开始代码实施，本任务不进入；R28-08 已按设计取向关闭，R28-09 仍待后续。
- **本轮边界：** Build25 只处理 R28-06A/B/C，未实施 R28-07/R28-08/R28-09，未新增未知扩展输出适配器，未自动迁移未知字段到 extensions，未改变数据库 schema。Build25 Step 0～4 的后端定向/全量测试、build、vet、前端定向/全量测试、build、Docker Compose build、正式 Production smoke 与 `git diff --check` 均通过；人工/真机项目已登记但未标记为通过。

---

## 二、操作步骤（唯一执行入口）

以下“步骤”表示实际操作顺序；`R28-*`、`D3-*`、`N-core-*` 和 `N01～N07` 只表示问题或缺口标号，不表示操作顺序。当前步骤未完成前，不进入下一步骤。

### 步骤一：收口 Build21 Step 14 的工程问题

- **关联问题标号：** R28-01、R28-02、R28-03、R28-04。
- **操作内容：**
  - 处理 R28-01：复现并修复动态插件输入组件异常，完成控制台、输入保留和保存重开回归；
  - 处理 R28-02，然后处理 R28-03：修订正式 Production smoke 夹具，修复布尔断言并补齐 `fallback_group_members`；
  - 处理 R28-04：显式校验 Mihomo Meta v1.19.29，确保正反例实际执行，缺少二进制时不能静默跳过。
- **进入条件：** 当前工作区基于 Build21 Step 14 的未完成证据继续核验。
- **完成条件：** R28-01～R28-04 的工程修复和重新验证均有可复查证据；正式 smoke 使用仓库正式脚本完整通过。
- **状态：** ☑ 已完成（R28-01～R28-04 均已修复并验证；正式 Production smoke 已通过）

### 步骤二：完成 Build21 Step 14 的人工验收与最终收口（人工项目已完成）

- **关联问题标号：** R28-01、R28-02、R28-03、R28-04；已完成的 PT-28 人工结论已同步到本文件和 [Build21.md](Build21.md)，当前仍待人工复验的项目见 [ProdTestList.md](ProdTestList.md)。
- **已完成内容：** 用户已确认 Mihomo/Clash Verge、Shadowrocket、Production 浏览器、动态插件输入、控制台、保存重开、移动端交互和正式 smoke 相关 PT-28-01～PT-28-05 均已测试，未发现问题；这些项目已从 [ProdTestList.md](ProdTestList.md) 当前待办中移除。
- **收口结果：** R28-04 固定 Mihomo 证据门禁已完成，Step 14 已收口；本文件 R28-05～R28-09 继续按步骤三至步骤八独立处理。
- **注意：** URI 生成、单元测试、固定版本离线证据或离线 YAML 解析不能替代真机结论。

### 步骤三：实施 Build22 的 Design3/Build16 缺口

- **关联问题标号：** R28-05；具体缺口为 D3-1～D3-10。
- **前置条件：** 步骤一、步骤二全部完成，Build21 Step 14 已按证据收口。
- **操作内容：** 按 [Build22.md](Build22.md) 的构建计划，依次执行 Build22 子步骤；本次用户已授权 Step 8～11 连续串行实施，不并行。
- **研究状态：** 已完成当前代码复核、修复方案细化和 Build22 Step 1～11 代码与自动化验收；D3-1～D3-10 全部达到工程闭环，实际浏览器/真机项目按用户授权迁移至 [ProdTestList.md](ProdTestList.md)。
- **完成条件：** D3-1～D3-10 全部验收通过，Build16/Design3 状态完成同步收口。

Build22 子步骤只用于定位构建计划，不改变本文件的操作步骤命名：

| Build22 子步骤 | 工作内容 | 关联问题标号 | 状态 |
|---|---|---|---|
| Step 1 | 来源统计计数 | D3-2 | ✅ 验收通过 |
| Step 2 | 来源原始证据、全部 origin 保留、分页前去重与稳定排序 | D3-5 | ✅ 验收通过 |
| Step 3 | `no_resolve` 结构化解析、实例语义、新旧 Clash plan 兼容 | D3-1 | ✅ 补修通过 |
| Step 4 | 后端素材池能力白名单 | D3-4 | ✅ 验收通过 |
| Step 5 | 手工 origin 换绑、共享 Canonical 保护与重复 409 | D3-3 | ✅ 验收通过 |
| Step 6 | 零输出门槛补全 | D3-6 | ✅ 验收通过 |
| Step 7 | failed 快照、v1 强类型统计、1018 激活时间、log/pool 共用脱敏、存量输出清洗、`display_url` 状态 API | D3-7 | ✅ 补修通过 |
| Step 8 | 前端来源状态、诊断与 pending 操作 | D3-8 | ✅ 自动化验收通过；真实浏览器项目转 ProdTestList |
| Step 9 | 装配回执前端展示 | D3-9 | ✅ 自动化验收通过；真实页面项目转 ProdTestList |
| Step 10 | 真实 1015→1016 store 级迁移、幂等与回滚测试 | D3-10 | ✅ 验收通过 |
| Step 11 | 全量回归、文档同步与 Build16/Design3 状态收口 | D3-1～D3-10 | ✅ 验收通过 |

### 步骤四：处理未知扩展与局部 JSON 边界

- **关联问题标号：** R28-06（N-node-6）。
- **前置条件：** 步骤三完成。
- **操作内容：** 按已确认方案分别处理三个边界：未知扩展只作加密存档与诊断，不进入客户端产物；局部 JSON 未知键从所有对象默认允许改为显式白名单；子对象存在未应用 JSON 草稿时阻止父对象切换到高级 JSON，并展开、定位该子草稿。同步修正 Design4 的冲突表述、前端文案、后端检查语义和相应回归，不把 Build21 已完成的 R27-08/09 重复纳入。
- **完成条件：** R28-06A～R28-06C 的代码、自动化与文档同步全部完成：空/有 `targets` 的扩展均不会进入产物且诊断准确；只有明确白名单对象允许普通未知键；父子 JSON 草稿不会并存、静默丢失或留下不可见保存阻断；Design4、Build/Issue 记录与实际行为一致。
- **执行结果（2026-09-11）：** 已按 [Build25.md](Build25.md) Step 0～4 串行完成并通过自动化工程验收：Step 0 完成合同冻结、35 行对象白名单盘点与 Design4 冲突同步；Step 1 完成扩展 targets 白名单、`unknown_extension_not_targeted` / `unknown_extension_not_rendered` 诊断与三类产物 sentinel 负向证据；Step 2 完成 `obj()` 默认拒绝、开放 Map 显式白名单、历史未知键读取保留/保存检查阻断/显式删除；Step 3 完成页面级父子 JSON 草稿阻断、展开定位、保存/检查定位与失效清理；Step 4 通过后端全量测试/build/vet、前端 42 文件/253 用例/build、Docker Compose build、正式 Production smoke 与 `git diff --check`；审计补强后新增用户下载重渲染与日志 sentinel 测试、历史未知子键清理回归。
- **研究状态：** ☑ 只读研究与用户决策完成 / ✅ Build25 Step 0～4 代码、自动化与文档工程闭环 / ◐ 浏览器、手机和真实客户端人工项已迁移 [ProdTestList.md](ProdTestList.md)，未标记为人工通过

### 步骤五：整改核心工程约束与一致性问题

- **关联问题标号：** R28-07A～R28-07I；其中 R28-07F 为已确认设计取向，不进入整改范围。
- **前置条件：** 步骤四完成。
- **操作内容：** 按 R28-07 已确认方案依次处理初始化冗余标记、隐藏组解析 Token、被忽略错误、接入层越层访问、可变包级状态、导入体积边界、遗留颜色类和 SSE 管理端鉴权；验证码 Secret 明文存储/原值回显按 R28-07F 保留现有设计，不纳入代码变更。
- **完成条件：** R28-07A～R28-07E、R28-07G～R28-07I 均完成代码、定向回归和受影响全量门禁；R28-07F 的非问题决策保留可追踪记录，不得误记为已修复。
- **研究状态：** ☑ 只读研究与用户决策完成 / ☐ 待步骤五单独构建授权后实施（步骤四已完成工程闭环）

### 步骤六：确认安全报告历史项的设计取向（已决策关闭）

- **关联问题标号：** R28-08（N01～N07）。
- **用户决策：** 2026-09-09 确认 N01～N07 整体不视为问题，属于当前产品/部署模型的设计取向；不启动 SecurityReport2/3 所建议的专项代码整改。
- **操作结果：** 保留安全报告作为历史审计与风险背景，不把其中建议转写为工程缺陷、修复完成或强制验收门槛；未来若改变取向，应先进入新的 Design/Build 决策，不在 R28-08 下直接恢复实施。
- **状态：** ☑ 用户确认设计取向 / ☑ 从本轮工程实施范围关闭

### 步骤七：完成项目级工程与文档收尾

- **关联问题标号：** R28-09。
- **前置条件：** 步骤三、步骤四、步骤五、步骤六全部完成。
- **操作内容：** 处理运行镜像与 digest、许可证、失效参考链接、未引用前端文件、PoolTab 文案等剩余项目级事项；已修正的文档项只保留实际证据，不重复实施。
- **完成条件：** 每一项关闭，或迁移到对应专项并保留可追踪链接。

### 步骤八：全量复核与 Issue14 关闭

- **关联问题标号：** R28-01～R28-09，以及步骤三中的 D3-1～D3-10、步骤五实际整改的 R28-07A～E/G～I；R28-07F 与步骤六 R28-08 仅核对设计决策记录，不纳入代码整改验收。
- **操作内容：** 按实际变更范围重新执行后端全量测试、竞态测试、`go build ./...`、`go vet ./...`、前端全量测试、`npm run build`、正式 Production smoke 和 `git diff --check`；核对 Build21、Build22、Design3、Design4、ProdTestList 和本文件的状态。
- **完成条件：** 所有纳入本轮范围的问题均有关闭证据或明确迁移记录；Issue14 不再存在无归属、无状态或无后继文档的问题。

补充说明：`Build23.md` 不作为新的操作步骤启动。其 R27-09/N-node-3/4 内容已归入 Build21；N-node-6 已在步骤四作为 R28-06 跟踪，并已由 [Build25.md](Build25.md) 完成代码与自动化工程闭环；Build23 继续保留为交接与边界说明。

---

## 三、问题详细记录（按关联步骤归档）

### 问题 R28-01：动态插件输入操作产生 Ant Design 控制台异常

- **关联步骤：** 步骤一、步骤二。

- **来源：** Step 14 隔离浏览器真实 API 流程；后续定位见 [研究 Issue14 R28-01](thread://01a081b4-58ea-7f42-be23-e03bff0402d2?hostId=local)。
- **现象：** 在未知 SS 插件动态参数切换或输入期间捕获两次相同异常：

  ```text
  TypeError: Cannot read properties of null (reading 'input')
  ```

- **定位证据：** 失败优先的父子闭环测试在父级收到 `update:modelValue` 后即时回写，并逐字符修改参数名；修复前第一次输入即替换参数名 Input DOM、丢失原焦点，同时 Vitest 捕获两条与真实浏览器相同的 Ant Design 未处理异常。
- **根因：** `ProtocolFieldEditor` 将可编辑参数名同时作为动态 Map 行的 Vue `key`。每次合法改名都会重建参数对象并触发父级同步回写，行 `key` 随参数名改变，旧 Ant Design Input 在自身 `nextTick` 回调执行前被卸载，回调随后访问空输入引用。
- **影响范围：** 未知插件参数名的逐字符编辑会重建输入框，破坏焦点、光标和输入法状态并产生控制台异常；此前一次性填充仍能保存重开，未取得已保存数据丢失证据。插件切换只负责显示或移除参数区，不是异常的直接根因。
- **修复结果：** `ProtocolFieldEditor` 为 Map 行维护仅存在于组件本地、与参数名解耦的稳定行身份；新增时分配、合法改名时迁移、父级或外部值回填时按当前键集合对齐、删除或分支清空后清理。参数对象、即时回写、非法改名阻断、数据库/API 和插件输出合同均未改变，也未修改或补丁化 Ant Design 依赖。
- **自动化验证：** 新增组件宿主闭环和 `NodesView` 真实父级回写回归，逐字符改名后输入 DOM 与焦点保持、最终对象完成原子改名且无未处理异常；`2` 个定向文件 / `53` 个用例、前端全量 `41` 文件 / `210` 用例、`npm run build`、后端全量测试、`go build ./...` 与 `go vet ./...` 均通过。
- **人工验证边界：** 用户已确认 PT-28-01～PT-28-05 的浏览器走查、控制台、响应式、客户端和 Production 项目完成且未发现问题；自动化修复证据与用户人工结果均已记录在本轮收口记录中。
- **状态：** ☑ 已复现定位 / ☑ 已修复 / ☑ 自动化验收通过 / ☑ 用户人工复核通过

### 问题 R28-02：Production smoke 应急状态断言丢失 JSON 布尔类型

- **关联步骤：** 步骤一、步骤二。

- **来源：** 实际运行 `bash .smoke-test-prod.sh` 的 Step 14 核验。
- **现象：** `.smoke-test.sh` 的通用 `J()` 使用 Python `print()` 输出解析结果，JSON 布尔值 `false` 会变成 `False`，再与小写字符串 `false` 比较，导致正常响应失败；相反，错误类型的 JSON 字符串 `"false"` 会输出为 `false` 并被旧断言接受，存在假绿。
- **证据：** 修复前失败优先矩阵确认 JSON 布尔 `false` 被拒绝，而字符串 `"false"` 被接受；当前严格断言见 [.smoke-test.sh](.smoke-test.sh:94)～[.smoke-test.sh](.smoke-test.sh:106)。服务端 [status.go](backend/internal/server/status.go:56)～[status.go](backend/internal/server/status.go:79) 始终以 Go `bool` 返回该字段，定向普通态/应急态测试通过，未发现后端合同异常。
- **影响范围：** 原始 Production smoke 在正常实例上被误阻断，并且旧字符串比较没有严格验证接口类型；不影响服务端应急状态计算、API 结构或前端行为。
- **修复结果：** 仅收紧 `.smoke-test.sh` 的 10d 断言，不修改通用 `J()`。Python 直接读取 `data.emergency`，用 `value is False` 同时校验 JSON 布尔类型和值，并用 `json.dumps()` 输出规范诊断值；`true`、字符串 `"false"`、`0`、`null`、缺失字段和非法 JSON 均不能通过。
- **自动化验证：** `bash -n .smoke-test.sh .smoke-test-prod.sh` 通过；独立输入矩阵确认仅 JSON 布尔 `false` 通过，`true`、字符串 `"false"`、`0` 与 `null` 均失败；服务端普通/应急状态定向测试、后端 `go test ./... -count=1`、`go build ./...`、`go vet ./...` 及前端 `npm run build` 均通过。
- **重新执行结果：** R28-03 修复后，未做临时转换的仓库正式脚本 `bash .smoke-test-prod.sh` 已完整通过，10d 严格布尔断言在 Production 全链路中实际通过。
- **状态：** ☑ 已修复 / ☑ 定向自动化验证通过 / ☑ 正式 Production smoke 全链路复验通过

### 问题 R28-03：Production smoke Clash 请求缺少 `fallback_group_members`

- **关联步骤：** 步骤一、步骤二。

- **来源：** R28-02 修正后的临时执行流继续运行时发现。
- **现象：** `.smoke-test.sh` 的步骤 13 基础 Clash 装配和步骤 13f 覆盖层 Clash 装配均没有传当前接口要求的 `fallback_group_members`；第一处会先触发无法归属流量组缺少成员的 400 错误，若只修第一处，脚本会在第二处以同一原因再次失败。
- **证据：** 两处已修正请求见 [.smoke-test.sh](.smoke-test.sh:126)～[.smoke-test.sh](.smoke-test.sh:129) 与 [.smoke-test.sh](.smoke-test.sh:176)～[.smoke-test.sh](.smoke-test.sh:179)；请求模型字段见 [models.go](backend/internal/assembly/models.go:168)～[models.go](backend/internal/assembly/models.go:176)。仓库其余活跃 Clash API 测试夹具未发现同类遗漏。
- **影响范围：** 原始 smoke 无法完成基础 Clash，并会连带阻断其他三类装配器、URI 导入、覆盖层 Clash 与 v2 往返；问题仅在 smoke 请求夹具，不影响服务端装配实现、接口合同或前端行为。
- **修复结果：** 两处请求均显式补齐 `fallback_group_members:["🚀直接连接","🌎国外流量"]`，顺序与当前前端初始值及既有强制组合同一致；未修改 `GenerateInput`、后端非空校验、渲染逻辑、前端默认值、数据库或非 Clash 请求。
- **自动化验证：** `bash -n .smoke-test.sh .smoke-test-prod.sh` 与强制组合同定向测试通过；未做临时转换的 `bash .smoke-test-prod.sh` 完整通过步骤 13、generic-subs、sr-subs、sr-conf、URI 导入 2 ok / 1 skip、步骤 13f 覆盖层、v2 导出/导入，并输出 `SMOKE ALL DONE` 与 `PROD SMOKE ALL DONE`。后端 `go test ./... -count=1`、`go build ./...`、`go vet ./...` 及前端 `npm run build` 均通过。
- **状态：** ☑ 已修复 / ☑ 定向自动化验证通过 / ☑ 正式 Production smoke 全链路复验通过

### 问题 R28-04：固定 Mihomo 验收测试允许未设置二进制时静默跳过

- **关联步骤：** 步骤一、步骤二。

- **来源：** 两个 Step 14 任务结论之间的证据复核。
- **现象：** [mihomo_ssplugin_test.go](backend/internal/assembly/mihomo_ssplugin_test.go:15) 在 `MIHOMO_11929_BIN` 未设置时调用 `t.Skip`，测试进程仍以成功退出。因此全量测试通过不能单独证明固定 Mihomo 1.19.29 正反例实际执行。
- **影响范围：** Step 14 固定版本验收证据；可能把“未执行”误读为“通过”。
- **修复方向：** 在正式验收入口显式提供并校验 Mihomo Meta v1.19.29 二进制；必要时为验收命令增加强制模式，使缺少二进制时失败而不是跳过。此前带显式环境变量的定向通过记录可保留，但需与本次全量测试区分。
- **修复前核对证据（2026-09-09）：** `/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo` 报告 `Mihomo Meta v1.19.29 darwin arm64`；显式设置 `MIHOMO_11929_BIN` 后 `obfs`、`v2ray-plugin`、`shadow-tls`、`restls` 四个正例均通过。未设置该变量的同一测试仍为 `SKIP`，说明当时固定版本内容已有通过证据，但“缺少二进制不得静默通过”的验收门禁尚未完成。
- **修复结果：** 新增根目录严格入口 [.mihomo-test.sh](.mihomo-test.sh)：未设置 `MIHOMO_11929_BIN`、路径不可执行、版本读取失败或产品/版本不是精确的 `Mihomo Meta v1.19.29` 时均非零退出；普通 `go test ./...` 仍允许外部二进制测试明确 `SKIP`，不把外部依赖强加给日常回归。固定测试改用产品名与版本字段精确匹配，并补齐四个生成正例、`shadow-tls`/`restls` 缺少 `host`、obfs/v2ray-plugin 非法 mode 四个内核反例，以及“Mihomo 接受旧拼接字符串、项目 `CheckClashContent` 仍拒绝”的双层边界。
- **自动化验证：** `bash -n .mihomo-test.sh` 通过；缺少变量和 `/usr/bin/true` 错误程序均按预期失败；显式使用 `/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo` 时 4 个正例、4 个反例和旧格式双层边界全部实际执行并通过。后端定向测试、`go test ./... -count=1`、指定五包 `go test -race`、`go build ./...`、`go vet ./...`，前端 41 文件/210 用例及 `npm run build` 均通过；仅保留既有大 chunk 提示。R28-03 已记录正式 Production smoke 原样全链路通过。
- **状态：** ☑ 已修复 / ☑ 严格门禁正反例通过 / ☑ Step 14 固定版本证据收口

### 问题 R28-05：Build16/Design3 未闭环（D3-1～D3-10）

- **关联步骤：** 步骤三。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §4.2 未闭环项 1；实施计划见 [Build22.md](Build22.md)。
- **原始现象/范围（修复前）：** Build16/Design3 当时存在以下实质缺口：
  - D3-1 `no_resolve` 实例语义丢失；
  - D3-2 被来源模式排除的数量重复计算；
  - D3-3 手工规则更新污染共享 URL Canonical；
  - D3-4 后端未强制素材池能力白名单；
  - D3-5 来源原始证据/顺序未正确落库；
  - D3-6 零输出门槛不完整；
  - D3-7 per-URL 快照/状态/诊断 API 缺失；
  - D3-8 pending 激活/丢弃无前端 UI；
  - D3-9 装配回执未展示；
  - D3-10 1015→1016 迁移缺少 store 级测试。
- **当前证据：** 2026-09-10 完成 [Build22.md](Build22.md) Step 1～11 代码与自动化验收：Step 8 组件测试 22 项与前端生产构建通过；Step 9 后端 server 定向测试、前端 25 项定向测试与前后端构建通过，generate 回执已用原始 JSON 固定；Step 10 真实 1015→1016 store 级迁移、幂等和失败回滚测试通过；Step 11 后端 build/vet/全量测试、前端 42 文件/244 用例全量测试、生产构建、Docker Compose build 与正式 Production smoke 均通过。实际浏览器、真实设备和真实客户端项目已迁移至 [ProdTestList.md](ProdTestList.md)，未标记为人工通过。
- **深入研究新增结论：**
  - Design3 要求语义去重时保留 origin；已确认同一 URL 内重复位置和跨来源重复均保留 origin，`accepted` 统计唯一 Canonical，`duplicates` 统计额外 origin，列表/装配必须在 SQL 分页前按最早有效 origin 去重排序。
  - `no_resolve` 不仅在装配加载中丢失，来源解析还使用整行子串判断，Clash `render_plan_json` 和下载重渲染也按类型补加；已确认改为结构化 token，并固定采用逐规则 `NoResolve *bool` 三态编码：字段缺失或 JSON `null` 维持历史按类型推断，新计划每条规则显式写入 boolean true/false，不为这一单字段引入顶层 plan schema version；覆盖层降级只保留原行选项。
  - 手工修改必须换绑 origin；目标仅有 URL origin 时允许共享，目标已有另一 manual origin 时返回 409，不静默合并或覆盖。
  - per-URL 主状态由最近一次同步尝试决定；最近失败可以与继续生效的旧 active 同时展示，后续成功后旧 failed 只进入历史。诊断统一在持久化边界执行 20 条×200 字符限额和敏感信息脱敏。
  - `stats_json` 已确认采用 version 1 强类型合同：以稳定 evidence/reason codes 表达确定性检测依据和初始同步决策，不引入数值置信分数；按 family/matcher/scope 记录 accepted/excluded/rejected/duplicates 分项，顶层列继续作为六项计数、format/profile/当前 status 的唯一事实来源；旧 `{}`/无版本 JSON 兼容但不补造证据。
  - `detected_profile` 必须在 `source_mode` 排除前基于全部已识别、规范化候选计算；adapter reject 必须完整进入 rejected。pending 人工激活时间使用下一号 1018 迁移新增的 nullable `activated_at` 保存，激活不得改写初始决策和解析统计。
  - 1016 回归必须使用真实 0001～1015→1016 迁移链，覆盖旧业务数据清除、ID 防复用、无关历史保留、重复迁移幂等和失败整体回滚，不能以简化旧 schema 代替。
  - Build22 Step 10 已进一步完成“真实 1015 迁移测试夹具最小可行构造”只读研究并按用户确认细化：`migrationsThrough` helper 按解析版本过滤且排除 1017；最小夹具为 ID 10/100 两个旧池、manual/URL 条目、旧同步任务、versions、assembly_blueprints，不额外插入 owner 记录；成功断言覆盖 `pool_sync_tasks` 重建为空表、`sqlite_sequence`、精确新 ID 101、close/reopen 幂等；失败回滚在真实 1016 文件末尾追加失败语句，并在回滚后用真实 1016 重试。同步记录见 Build22 Step 10 与 Build16 后续勘误。
  - Build22 Step 7“脱敏机制复用范围”已按用户确认更新 Build22/Design3/AGENTS：新建 `backend/internal/redact` 公共脱敏包并由 log/pool 共用；`SourceStatus` 使用 `display_url`；现有 `/sync/status`、`/sync/tasks`、`Pool.sync_error` 纳入读时清洗；历史 `pool_sync_tasks`/`rule_pools.sync_error` 做非破坏性清洗；诊断超 20 条采用 19 条真实 + 1 条截断摘要；所有进入持久化/展示 API 的字符串字段按 200 rune 限长并统一脱敏。
- **前置条件：** 步骤一、步骤二已完成，Build21 Step 14 已按证据收口；不得在此前开始 Build22 的任何子步骤。
- **修复过程：** 按已详细修订的 [Build22.md](Build22.md) Step 1～11 串行实施并验收；实际浏览器/真机项目按用户授权迁移至 ProdTestList，未提前实施 R28-06 及后续步骤。
- **状态：** ☑ Build22 Step 1～11 代码与自动化验收完成 / ☑ D3-1～D3-10 工程闭环 / ☑ Build16/Design3 状态已同步 / ◐ 实际浏览器与真机项目已迁移 ProdTestList，待用户人工核验。R28-05 达到工程实现闭环；人工浏览器项目不构成代码/自动化未完成，也不得声称已人工验证。

### 问题 R28-06：未知扩展/局部 JSON 边界（N-node-6）

- **关联步骤：** 步骤四。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §5.3 N-node-6；原始详细证据见 [BuildReport3.md](docs/reports/BuildReport/BuildReport3.md) §6.4；交接说明 [Build23.md](Build23.md) §二.7；当前代码与 Design4 §6.4/§12.1～§12.3 的交叉核验。
- **现象/范围：** 本项不是 R27-08/09 的重复问题，而是三个尚未形成一致合同的独立边界：
  1. **R28-06A 未知扩展目标语义：** `extensions_json` 已实现整体加密、摘要回显、替换/清除和按 scope 清空，但没有任何输出适配器解密或消费扩展负载；`targets` 命中时检查固定报告 `unknown_extension_not_rendered`，`targets` 为空时反而没有扩展诊断。当前实际行为与 Design4“明确指定目标时参与输出、未指定时提示未参与输出”的表述冲突。
  2. **R28-06B 局部 JSON 未知键边界：** 注册表通用 `obj()` 对所有对象默认设置 `allow_unknown=true`；已知对象内的未知键会留在活动 `protocol_json`、按活动投影进入后续适配器，且只对注册表已声明的敏感路径加密/脱敏。除已确认按普通字符串处理的未知 SS `plugin-opts` 外，其他对象没有明确合同证明未知键一定非敏感，因而与未知扩展“无法识别敏感路径时整体加密”的边界不清。顶层未知字段则由服务端明确拒绝并保持零写入，URI 导入也整行拒绝，当前没有自动转换为扩展的安全归属依据。
  3. **R28-06C 父子局部 JSON 草稿：** 单对象的草稿、校验、应用、放弃和分支失效已有实现；但子对象存在未应用草稿时，父对象仍可切换到高级 JSON 并卸载子编辑器。父级没有检查后代 dirty 路径，页面级 `unappliedJsonPaths` 也没有组件卸载清理，可能同时形成覆盖同一区域的父草稿、静默丢失子草稿，或留下不可见的保存阻断。
- **影响范围：** 影响手工节点创建/编辑、URI 导入失败反馈、`extensions_json` 摘要与目标检查、递归协议对象高级 JSON、保存前错误定位以及未知普通参数的存储/脱敏/输出边界；不改变 Build21 已完成的 SS 插件结构化输出、`diagnostics: []`、Shadowrocket URI 参数或固定 Mihomo 门禁结论。
- **用户确认决策（2026-09-09）：** 三项均采用研究推荐方案：
  1. 未知扩展定位为**受保护的加密存档块**，当前只保存、回显摘要和诊断，不进入 Clash、Shadowrocket 或 generic 客户端产物；不在缺少目标专属 payload 结构、注入路径、冲突规则、脱敏和自检合同的情况下直接透传任意字符串。
  2. 局部 JSON 未知键采用**显式白名单**；固定结构对象默认拒绝未知键，只有业务上明确开放的 Map/对象继续允许。Headers 等开放 Map 与已确认的未知 SS `plugin-opts` 保留普通参数合同，不按键名猜测敏感性。顶层未知字段本项继续明确拒绝且零写入，不自动迁入扩展块。
  3. 父子草稿冲突采用**阻止覆盖**；发现后代未应用草稿时阻止父对象切换到高级 JSON，展开并定位首个后代草稿，要求用户先应用或放弃，不静默清除，也不实现父子草稿自动合并。
- **修复方向：**
  - **R28-06A：** 保持 `extensions_json` 与 API 字段兼容；统一前端、Design4 和检查语义为“存档/诊断、不输出”。明确 `targets` 仅表示期望/关联目标而非已支持输出，校验允许的目标值；空 `targets` 也必须在请求的每个目标上提示未参与输出，命中目标时继续给出“未渲染”警告，不能因扩展存在而返回虚假 `ok`。补充负载类型/大小等非敏感摘要属于实现时的可读性收口，不回显密文或明文。未来若需目标输出，必须另立 Design/Build，逐目标定义 payload schema、注入位置、冲突策略、脱敏、自检和固定客户端证据。
  - **R28-06B：** 取消 `obj()` 全局默认 `allow_unknown=true`，改为 schema 显式声明；逐项盘点现有对象，只对白名单容器开放，并为其记录值类型、敏感性边界和是否进入输出。实施前补充现有存量未知键的兼容读取/更新测试，禁止用全局删除或按 `password`/`token`/`secret` 键名猜测替代明确合同。顶层未知字段和 URI 导入保持显式拒绝、逐行原因与零写入；自动带入扩展编辑器/待处理队列明确排除在本项之外。
  - **R28-06C：** 建立页面级路径层次草稿协调：父对象进入高级 JSON 前检查后代 dirty 路径，冲突时阻止切换、展开祖先区域并聚焦后代编辑器；schema 条件移除、分支重置和组件卸载时同步清理已失效的 dirty/validity 状态，不能清理仍有效但尚未处理的用户草稿。补父子双层/三层、条件隐藏、分支清空、保存定位和无幽灵阻断回归。
- **验收证据要求：** 后端覆盖扩展空/已指定/非法目标、加密摘要、检查诊断、不进入三类产物、白名单对象接受/固定对象拒绝和失败零写入；前端覆盖扩展文案/目标校验、父子草稿阻断/定位/应用/放弃/清空；按影响范围执行节点与装配定向测试、后端全量测试/编译/vet、前端全量测试与生产构建、`git diff --check`。自动化只证明合同与产物边界，不新增真实客户端连接结论。
- **补充确认（2026-09-11）：** 空 `targets` 采用 `unknown_extension_not_targeted`，命中当前 target 保留 `unknown_extension_not_rendered`；固定结构对象历史未知键读取保留并显示、检查/保存阻断、用户在高级 JSON 显式删除后才能保存，不自动删除、不自动迁移、不进入输出投影。
- **实施与验收结果（2026-09-11）：** 按 [Build25.md](Build25.md) Step 0～4 完成。Step 1：`ssplugin.TargetNames()` 权威集合、扩展 targets trim/去重/白名单、空/命中/非命中诊断矩阵、`status` 不虚假 `ok`、前端受控多选与“不进入任何输出产物”文案、Clash/SR/generic preview/generate、用户下载重渲染和日志 sentinel 负向断言。Step 2：`obj()` 默认拒绝未知键，Headers/未知 SS `plugin-opts` 等开放 Map 显式 `allow_unknown=true`，schema JSON 固定下发布尔值；历史未知键按补充确认处理；固定对象未知键创建/更新失败零写入且 revision 不变。Step 3：`ProtocolFieldEditor` 后代 dirty 阻断事件、`NodesView` 稳定排序/展开/聚焦/保存与检查定位、条件/reset 清理和折叠保留回归。Step 4：后端 `go test ./... -count=1 -timeout 180s`、定向 4 包、`go build ./...`、`go vet ./...`，前端 42 文件/253 用例、`npm run build`，`docker compose build`、`bash .smoke-test-prod.sh` 与 `git diff --check` 全部通过；审计补强新增服务端下载重渲染和日志 sentinel 测试、历史未知子键清理回归。
- **文档同步：** Design4 §6.4/§10/§12 已改为加密存档/诊断、不输出与显式白名单合同并追加 v1.17/v1.18；[Build25.md](Build25.md) 已记录全部 Step 与自主决策；[AGENTS.md](AGENTS.md)、[Build23.md](Build23.md)、[ProdTestList.md](ProdTestList.md) 已同步。Issue14 不再将 Design4 旧表述作为当前事实。
- **前置条件：** 步骤三 R28-05 / Build22 Step 1～11 已完成；本项未进入步骤五。
- **状态：** ☑ 只读研究完成 / ☑ 用户决策确认 / ☑ 文档基线已同步 / ☑ 代码与自动化验收通过 / ◐ 浏览器、手机和真实客户端人工项已迁移 ProdTestList，未标记为通过

### 问题 R28-07：核心工程约束与一致性问题（R28-07A～R28-07I）

- **关联步骤：** 步骤五。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §3.2、§6.3，以及当前代码对首页订阅、规则同步、下载渲染、配置导入、日志 SSE 和前端视觉 Token 的交叉核验。
- **研究结论与已确认范围：** 原 N-core-1～6 不能直接作为同质缺陷批量整改：N-core-1 是无效元数据而非当前越权漏洞；N-core-2 的准确行为是自定义订阅存在时可能补生一个随后被隐藏的组 Token，并非每次首页请求都生成新 Token；N-core-3～5 仍有实质工程缺口；N-core-6 对应的 R28-07F 已由用户确认为设计取向。另将同一核验发现的导入上限、遗留颜色类和 SSE 管理鉴权分别列为 R28-07G～I。N-core-7 已被后续 Build4/Design2 口径覆盖，不重复登记。
- **分项修复方案：**
  1. **R28-07A（首管理员初始化标记）：** 当前密码注册和 OIDC 建号均以 `users` 表是否为空决定首个管理员，并写入一个没有读取方的初始化标记；没有证据表明该标记参与权限判断。实施时删除两条无效写入，继续以同一事务中的用户计数/创建逻辑为唯一事实来源；历史库中已有键不做破坏性迁移，保留为无害旧数据。补密码注册、OIDC 首次建号、后续普通用户和并发首建回归，保证角色结果不因清理元数据而改变。
  2. **R28-07B（自定义订阅与隐藏组 Token）：** 当前首页先为有效平台订阅调用组 Token 的 `GetOrCreate`，随后才查询自定义订阅并覆盖卡片；上传自定义订阅时又会删除组 Token，所以首次回到首页会重新补生一个不可见组 Token，后续请求通常复用而非持续新增。将“查询自定义订阅”前移到业务分支：存在自定义订阅时只创建/复用自定义 Token，不触碰组 Token；不存在自定义订阅且平台有激活版本时才创建/复用组 Token。把遗留的同用户/平台双 Token 清理纳入基于业务键的幂等协调方法，避免接入层直接 SQL 和先读后写竞态。覆盖上传后首次/重复首页、自定义删除后恢复组 Token、历史双 Token 修复和并发请求。
  3. **R28-07C（被忽略的 error）：** 不限于报告列出的三处；当前至少还包括规则同步结果 JSON 序列化、终态任务清理和失败状态回写，装配自动建规则后的补偿删除，以及 OIDC 过期换票凭据删除。实施一次生产代码全量审计并按语义分类：影响正确性的错误必须返回并回滚；补偿/清理错误必须记录结构化日志，必要时用 `errors.Join` 与主错误共同返回；异步状态写入失败必须记录可定位上下文；仅明确无错误语义的 comma-ok 类型判断不作误报。增加可识别空白赋值/显式丢弃 error 的静态门禁和各失败注入回归。
  4. **R28-07D（接入层直接访问存储）：** `server/render.go` 同时执行蓝图/节点 SQL 与整段用户下载业务，`server/traffic.go` 直接查询流量和用户状态，OIDC Handler 直接管理换票事务。实施时把用户动态下载渲染整体迁入独立业务服务（不是只包一层 SQL），把月流量/配额汇总迁入 Xray 业务层并返回不感知 Gin 的结构体，把 OIDC ticket 创建、消费和过期清理收回 OIDC 服务；Handler 只解析协议、调用服务并映射响应。增加架构检查，禁止非测试 `internal/server` 直接调用 `DB()`/`TxImmediate()`，并保持现有 HTTP 合同和下载产物不变。
  5. **R28-07E（可变包级状态）：** 当前运行期可变状态包括 `response.debugProvider` 回调、默认 Logger/Level 控制器及配置敏感键注册表。将调试标志通过 Gin 请求上下文中间件传递；Logger 与级别控制器作为运行实例构造注入；敏感键改为固定判定规则/不可变集合，不再运行期注册。只读常量、编译期注册表和不可变正则不机械改写。补多 Server 实例隔离、并发切换日志级别、5xx 调试响应和敏感值加密/脱敏回归，并运行相关竞态测试。
  6. **R28-07F（验证码 Secret 明文存储与原值回显）：** Design1/既有构建明确采用明文保存 Site/Server Key 并允许管理页原值回显。用户于 2026-09-09 确认这是设计取向、不视为问题；本轮保持数据库、导入导出和 API 行为不变，不实施加密迁移、掩码回显或只写更新，也不得把本项记为安全修复完成。若未来调整，须先在新的 Design/Build 中定义兼容迁移和管理交互。
  7. **R28-07G（导入上传上限）：** `/api/setup/import` 与 `/api/admin/settings/import` 当前绕过全局 body limit，并把 multipart 文件无界追加到内存，读取循环还会把非 EOF 错误当作正常结束。按用户决策将**文件硬上限定为 20 MiB（20 × 1024 × 1024 字节）**：前端选择文件时提前拒绝超限；后端同时限制完整请求体（20 MiB 文件加有界 multipart 余量）和文件字段本身，使用 `MaxBytesReader`/有界读取识别无 Content-Length、分块传输和伪造长度，超过上限统一返回 413；区分 `io.EOF` 与真实读取错误，后者返回 500/对应导入错误且不创建任务。现有 AES-GCM 整体格式暂以最多 20 MiB 的有界缓冲兼容，不在本项改变导入文件格式。补边界值、超 1 字节、分块上传、截断读取、Setup/管理员双入口及前端校验回归。
  8. **R28-07H（遗留 Tailwind gray/white 类）：** R29-06 将 `EditableCombobox.vue` 改为复用标准 `AppSelect` 后，该组件原有输入禁用态/下拉背景/悬停态/空态硬编码颜色已随旧自研浮层删除；当前仍剩 `NodeCheckPanel.vue` 的预览背景，以及“增加前端静态扫描并补两主题组件快照或 DOM 类断言”尚未实施。本项继续保持未完成，不由 R29-06 扩张处理。
  9. **R28-07I（SSE 管理端鉴权例外）：** 当前 `/api/admin/logs/stream` 独立于会话/管理员中间件，只消费管理端预换取的一次性查询 Token；该方案虽是 Design1 为原生 `EventSource` 不能附带 Authorization Header 作出的兼容设计，但仍与“所有管理端点叠加会话校验 + 角色校验”不一致。本项按推荐方案整改：前端改用 `fetch` + `ReadableStream` 携带现有 Authorization 凭据并解析 SSE 帧；流端点移入管理员路由组，删除 `/stream/token`、一次性 Token 状态及相关复位逻辑；保留历史缓冲、增量推送、断线重连、`AbortController` 清理和连接数限制。覆盖未登录 401、非管理员 403、权限实时变化、流分帧/多行 data、重连、组件卸载及日志查询/清空不回归；同步修订 Design1 的历史例外说明。
- **实施顺序与门禁：** 步骤五获得单独构建授权后，先补失败优先回归与架构/静态门禁，再完成 A/B/D 的业务边界调整、C/E 的工程治理、G/I 的接口与前端联动，最后处理 H 并执行受影响全量回归。每次仍须按 Build 文档规则拆成一个可验收 Step；本次研究不授权任何代码修改。最终至少执行后端定向/全量测试、相关 `go test -race`、`go build ./...`、`go vet ./...`、前端定向/全量测试、`npm run build`、接口级 401/403/413 回归和 `git diff --check`。自动化不替代 Production 浏览器或真实部署人工结论。
- **文档同步要求：** R28-07F 与 AGENTS.md 的敏感配置原则、R28-07I 与 Design1 的 SSE 例外存在已知口径差异；用户已对 F 作出保留设计的决定，I 则决定按工程约束整改。实施时仅同步受实际变更影响的 Design/Build 文档，不回写归档报告为“原报告错误”。
- **状态：** ☑ 只读研究完成 / ☑ 用户决策确认 / ☐ R28-07A～E、G～I 待步骤五单独构建授权后实施（步骤四已完成） / ☑ R28-07F 按设计取向排除

### 问题 R28-08：安全历史项复核（SecurityReport2/3 N01～N07，设计取向）

- **关联步骤：** 步骤六。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §6.5、[SecurityReport2.md](docs/reports/SecurityReport/SecurityReport2.md)、[SecurityReport3.md](SecurityReport3.md)。
- **历史范围：** N01～N07 分别涉及依赖漏洞/CI 门禁、备份加密、验证码 fail-open、重置端点限流与令牌存储、JWT 浏览器存储、安全头/CSP、OIDC 首设密码通知；安全报告中的建议和证据继续保留供未来风险评估。
- **用户确认决策（2026-09-09）：** R28-08 整体不视为当前工程问题，N01～N07 属于现阶段产品、部署与运维模型的设计取向，不纳入 Issue14 修复范围；本次不修改相关代码、配置或测试，也不将“未实施”写成“已修复”。
- **后续边界：** 若未来产品模型、部署暴露面或安全目标变化，应重新进行风险评估，并通过新的 Design/Build 明确范围、兼容策略和验收证据；不得直接沿用本项“关闭”结论推断新环境仍可接受。
- **状态：** ☑ 已复核 / ☑ 用户确认为设计取向 / ☑ 非问题关闭

### 问题 R28-09：其他 BuildReport4 项目级工程/文档收尾

- **关联步骤：** 步骤七。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §6.1、§6.4、§6.6。
- **现象/范围：**
  - 运行镜像未显式安装 `ca-certificates`，基础镜像/GHCR 未固定 digest；
  - AGENTS 文档清单原先缺 BuildReport2/3/4、SecurityReport2/3 等报告（已在本轮同步修正）；
  - README 提及 LICENSE 但仓库未发现 LICENSE；
  - `docs/Reference/Xray-Server-Config-Research.md` 存在指向仓库外 `Xray-examples` 的失效链接；
  - 前端存在未引用文件清理候选（`GenerateStep.vue`、`PreviewState.vue`、`ResponsiveCollection.vue`、`CopyField.vue`）；
  - `PoolTab.vue` 的“停机错过不补跑”文案与当前启动补跑实现不一致；该文案已由 Build22 Step 11 顺带修正为“服务启动时补跑今日错过”，不表示 R28-09 其余项已完成。
- **修复方向：** 文档类问题在本轮文档交叉审核中同步修正或登记；代码清理/镜像加固作为后续工程项处理。
- **状态：** ☐ 文档/工程收尾待办

---

## 四、关闭条件

- **步骤一关闭：** ☑ R28-01～R28-04 的工程修复和重新验证均有证据，正式 smoke 使用仓库正式脚本完整通过。
- **步骤二关闭：** ☑ PT-28-01～PT-28-05 已由用户确认完成且未发现问题，相关人工项目已从 ProdTestList 当前待办移除；Build21 Step 14 已完成验收收口。
- **步骤三关闭：** Build22 子步骤全部验收通过，D3-1～D3-10 已完成，Build16/Design3 状态已同步。
- **步骤四关闭：** ☑ R28-06A～R28-06C 均按已确认合同完成实现、自动化与文档同步；扩展不进入产物且空/有目标诊断准确，局部 JSON 只在显式白名单容器保留普通未知键，父子草稿不会并存、静默丢失或留下不可见保存阻断；Design4 与实际行为一致。Build25 Step 0～4 定向/全量测试、build/vet、Docker build、Production smoke 与 `git diff --check` 均通过；浏览器/手机/真实客户端人工项已迁移 ProdTestList，尚未形成人工通过结论。
- **步骤五关闭：** R28-07A～R28-07E、R28-07G～R28-07I 已按确认方案完成代码、定向回归、全量门禁和受影响文档同步；R28-07F 保持设计取向且未误记为已修复。
- **步骤六关闭：** ☑ R28-08 的 N01～N07 已由用户确认为设计取向并从工程实施范围关闭；历史风险背景保留，未把“未实施”写成“已修复”。
- **步骤七关闭：** R28-09 的每项已关闭，或已迁移到对应专项并保留链接。
- **步骤八关闭：** 按实际变更范围重新执行后端全量测试、竞态测试、`go build ./...`、`go vet ./...`、前端全量测试、`npm run build`、正式 Production smoke 和 `git diff --check`；Issue14 中每个问题均有关闭证据或明确后继文档。

---

## 五、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.19 | 2026-09-11 | 完成 Issue14 步骤四 R28-06（N-node-6）代码、自动化与文档工程闭环：按 [Build25.md](Build25.md) Step 0～4 串行实施；空 targets 新增 `unknown_extension_not_targeted`、命中保留 `unknown_extension_not_rendered`；扩展 targets 白名单和 sentinel 不进入三类产物；`obj()` 改为显式 `allow_unknown` 白名单并处理历史未知键读取/阻断/显式删除；父子 JSON 草稿阻止覆盖、展开定位、保存/检查定位和 reset 清理。后端定向 4 包、全量测试、build、vet，前端 42 文件/253 用例、build，Docker Compose build、正式 Production smoke 与 `git diff --check` 全部通过；审计补强后补充服务端下载重渲染 sentinel、日志 sentinel 和历史未知子键清理回归。Design4、AGENTS、Build23、ProdTestList 已同步；人工浏览器/手机/真实客户端项目已迁移 ProdTestList，未标记为通过；未进入步骤五。 |
| v1.18 | 2026-09-10 | 完成 Build22 Step 8～11 代码与自动化验收：Step 8 每 URL 来源状态/诊断/pending UI 组件测试 22 项；Step 9 generate receipt 原始 JSON 合同、前后端回执展示测试 25 项；Step 10 真实 1015→1016 store 级迁移、幂等与失败回滚测试；Step 11 后端 build/vet/全量测试、前端 42 文件/244 用例、生产构建、Docker Compose build、正式 Production smoke 与 `git diff --check` 全部通过。Smoke 陈旧夹具与 Step 6 零输出门槛冲突按 B-3 修正并重跑通过。D3-1～D3-10 工程闭环，R28-05 工程实现关闭；实际浏览器/真机项目迁移至 ProdTestList，未标记为人工通过。 |
| v1.17 | 2026-09-10 | 按用户确认的深入检查方案完成 Build22 Step 1～7 代码补修与验证：Step3 SR `no-resolve` 目标能力判断、未知 option warn/位置法尾部解析；Step7 detector evidence codes 来源、sentinel reason_code、严格 v1 stats 形状、显式 null/空串 wire shape、snapshots 严格分页、URL 200 rune 限长与 failed 写入失败后缀保留。后端全量测试/build/vet、前端 build 均通过；Step 8 进行中，Step 9～11 未开始。 |
| v1.16 | 2026-09-10 | 完成 Build22 Step 10“真实 1015 迁移测试夹具最小可行构造”只读研究并按用户确认同步 Build22/Issue14/Build16：细化 `migrationsThrough` 按版本过滤并排除 1017；最小夹具固定为 ID 10/100 两个旧池、manual/URL 条目、旧同步任务、versions、assembly_blueprints，不额外插入 owner；成功断言覆盖 `pool_sync_tasks` 重建为空表、`sqlite_sequence`、精确新 ID 101、close/reopen 幂等；失败回滚在真实 1016 末尾追加失败语句并在回滚后重试。仅更新文档，Build22 Step 1～11 代码仍未开始。 |
| v1.15 | 2026-09-10 | 完成 Build22 Step 7“脱敏机制复用范围”只读研究并按用户确认同步 Build22/Design3/AGENTS/Issue14：新建 `backend/internal/redact` 公共脱敏包并由 log/pool 共用；`SourceStatus` 使用 `display_url`；现有 sync/status、sync/tasks、Pool.sync_error 纳入读时清洗；历史同步输出非破坏性清洗；诊断 19+1 截断摘要；字符串字段 200 rune 限长。仅更新文档，Build22 Step 1～11 代码仍未开始。 |
| v1.14 | 2026-09-09 | 完成 R28-05 Step 3 Clash render plan 兼容编码专项研究并按用户确认同步 Design3/Build22/Issue14：固定采用逐规则 `NoResolve *bool` 三态，缺失或 null 保持历史推断，新计划逐条显式冻结 boolean，不为单字段引入顶层 plan schema version；补齐原始 JSON、历史夹具、目标能力和覆盖层降级验收边界。仅更新文档，Build22 Step 3 代码仍未开始。 |
| v1.13 | 2026-09-09 | 完成 R28-05 `stats_json` 专项研究并按用户确认同步 Design3/Build22/Issue14：冻结 v1 强类型统计、确定性检测依据码、能力分项、旧 active 比较、初始决策原因与旧 JSON 兼容；补记 profile 计算和 adapter reject 统计缺口，pending 激活时间采用 1018 nullable `activated_at`。仅更新文档，Build22 Step 1～11 代码仍未开始。 |
| v1.12 | 2026-09-09 | 完成 R28-07 只读研究并按用户决策写入分项修复方案：R28-07A～E、G～I 待后续实施，导入文件硬上限确认为 20 MiB，R28-07F 保留为设计取向；R28-08 N01～N07 整体确认为设计取向并从工程实施范围关闭。本次仅更新 Issue14，未修改业务代码。 |
| v1.11 | 2026-09-09 | 完成 R28-06 只读研究并按用户确认细化修复方案：未知扩展定位为加密存档/诊断且不进入产物，局部 JSON 未知键改为显式白名单，父子草稿冲突采用阻止父级切换并定位子草稿；记录 Design4 冲突、实施前置条件与完整验收证据，本次仅更新 Issue14，未改业务代码。 |
| v1.10 | 2026-09-09 | 完成 R28-05 只读研究并按用户确认修订 Design3/Build22/Issue14：固化全部 origin 保留、manual 重复 409、latest-attempt 来源状态、新旧 Clash render plan 兼容、统一诊断限额/脱敏及真实 1015→1016 迁移测试口径；仅完成文档，Build22 Step 1～11 代码均未开始。 |
| v1.0 | 2026-09-08 | 从 Build21 Step 14 与 Issue13 的当前核验结论建立 Issue14；迁移除真机人工验收外的未完成项、新发现错误和固定版本证据缺口；人工项目统一转由 ProdTestList 管理。 |
| v1.1 | 2026-09-08 | 文档交叉审核扩展 Issue14 范围：按用户确认将 BuildReport4 中仍未闭环的工程问题归入本文件（R28-05～R28-09），覆盖 D3-1～D3-10、N-node-6、N-core-1～6、安全 N01～N07 及项目级收尾项；未修改 BuildReport4 归档文件。 |
| v1.2 | 2026-09-08 | 按当前核验结论重排唯一处理顺序：先收口 Build21 Step 14 的 R28-01～R28-04 与 ProdTestList 人工门槛，再执行 Build22 R28-05，最后依次处理 R28-06～R28-09；明确 Build23 不作为新的构建阶段。 |
| v1.3 | 2026-09-08 | 按操作步骤重新规范标题和跟踪结构：使用“步骤一～步骤八”表示执行顺序，将 R28、D3、N-core 和安全编号统一作为关联问题标号；移除重复的 Build22 跟踪表和旧的顺序编号表达。 |
| v1.4 | 2026-09-09 | 完成 R28-01：以父子即时回写失败回归确认可编辑参数名充当 Vue 行 key 导致 Input 卸载时序异常；改用本地稳定行身份并补组件/NodesView 逐字符输入、DOM、焦点与最终模型回归，全量前后端验证通过；人工浏览器复核仍归 ProdTestList PT-28-05。 |
| v1.5 | 2026-09-09 | 完成 R28-02：将 Production smoke 应急正常态从 Python 显示字符串比较改为严格 JSON 布尔类型和值断言；失败优先矩阵、脚本语法、后端全量测试/编译/vet及前端生产构建通过，完整正式 smoke 复验仍待 R28-03。 |
| v1.6 | 2026-09-09 | 完成 R28-03：补齐步骤 13 与 13f 两处 Clash 请求的 `fallback_group_members`，不改变装配合同；正式 Production smoke 原样全链路、定向合同测试及全量构建验证通过。 |
| v1.7 | 2026-09-09 | 用户确认 PT-28-01～PT-28-05 已完成且未发现问题；人工项目从 ProdTestList 当前待办移除，R28-04 及其余工程问题继续保留。 |
| v1.8 | 2026-09-09 | 交叉核验固定 Mihomo 证据：显式 `MIHOMO_11929_BIN` 的四个插件正例通过，未设置变量时仍静默 SKIP；将 R28-04 收窄为验收门禁补强问题，不再把固定版本正例本身标为未执行。 |
| v1.9 | 2026-09-09 | 完成 R28-04：新增严格 `.mihomo-test.sh` 验收入口，精确校验 Mihomo Meta v1.19.29，并补齐四个生成正例、四个固定内核反例和旧格式双层自检边界；缺少/错误二进制失败、全量前后端门禁通过，Build21 Step 14 正式收口。 |
