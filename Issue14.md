# Issue14.md — VPN 订阅管理系统问题追踪（当前）

> **文档定位：** 本文承接 [Issue13.md](Issue13.md) 的 R27-09 / Build21 收口核验，并汇总 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 全量核验中仍未闭环的工程问题；只记录除用户真机人工验收之外，当前仍未完成、待处理或新发现的工程问题与验收证据缺口。用户需要亲自执行的 Production、浏览器和客户端人工测试在执行期间由 [ProdTestList.md](ProdTestList.md) 记录；已完成项目从当前清单移除，结论保留在本文件和 Build21 中，当前仍待人工复验的项目继续由 ProdTestList 跟踪。
> 关联构建：[Build21.md](Build21.md) §7.11 Step 14、[Build22.md](Build22.md)（D3 实施计划）；交接说明：[Build23.md](Build23.md)；设计基线：[Design3.md](Design3.md)、[Design4.md](Design4.md)；编码约束：[AGENTS.md](AGENTS.md)。

---

## 一、当前总体结论

- **创建时间：** 2026-09-08
- **来源：** [核验 Build21 构建问题](thread://01a080de-c26e-7540-9db2-6b7b808f5eaa)、[继续 Build21 Step 14 测试](thread://01a080ce-4c83-7830-b7c2-aca36ba501b9)、[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 及当前工作区文档核对。
- **已通过：** Build21 Step 7～15 的自动化与验收证据已收口；后端全量测试、指定竞态测试、编译、`go vet`，前端 41 个测试文件 / 210 个用例和生产构建，以及固定 Mihomo 1.19.29 严格正反例门禁均有通过记录。
- **当前状态：** Build21 Step 14 的 R28-01～R28-04 工程问题、正式 Production smoke、固定 Mihomo 1.19.29 证据门禁及 PT-28-01～PT-28-05 人工项目均已完成，Step 14 已验收收口；BuildReport4 中仍未闭环的工程问题（D3-1～D3-10、N-core-1～6、N-node-6、安全 N01～N07 等）继续按后续步骤处理。剩余历史人工项目见 [ProdTestList.md](ProdTestList.md)，不在本文件重复登记为缺陷。
- **当前执行步骤：** 步骤一、步骤二已完成；下一步为步骤三，尚未开始。
- **本轮边界：** 步骤一已按 R28-01～R28-04 顺序完成，未改动后端业务接口、应急状态合同、装配合同、数据库或前端；步骤三及后续问题不在本次修复范围。

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
- **操作内容：** 按 [Build22.md](Build22.md) 的构建计划，每次只执行一个 Build22 子步骤；不得在步骤二完成前开始 Build22。
- **完成条件：** D3-1～D3-10 全部验收通过，Build16/Design3 状态完成同步收口。

Build22 子步骤只用于定位构建计划，不改变本文件的操作步骤命名：

| Build22 子步骤 | 工作内容 | 关联问题标号 | 状态 |
|---|---|---|---|
| Step 1 | 来源统计计数 | D3-2 | ☐ 未开始 |
| Step 2 | 来源原始证据、排序与装配去重 | D3-5 | ☐ 未开始 |
| Step 3 | `no_resolve` 实例语义贯通 | D3-1 | ☐ 未开始 |
| Step 4 | 后端素材池能力白名单 | D3-4 | ☐ 未开始 |
| Step 5 | 手工编辑不污染共享 Canonical | D3-3 | ☐ 未开始 |
| Step 6 | 零输出门槛补全 | D3-6 | ☐ 未开始 |
| Step 7 | failed 快照持久化与 per-URL 状态/诊断 API | D3-7 | ☐ 未开始 |
| Step 8 | 前端来源状态、诊断与 pending 操作 | D3-8 | ☐ 未开始 |
| Step 9 | 装配回执前端展示 | D3-9 | ☐ 未开始 |
| Step 10 | 1016 迁移 store 级回归测试 | D3-10 | ☐ 未开始 |
| Step 11 | 全量回归、文档同步与 Build16/Design3 状态收口 | D3-1～D3-10 | ☐ 未开始 |

### 步骤四：处理未知扩展与局部 JSON 边界

- **关联问题标号：** R28-06（N-node-6）。
- **前置条件：** 步骤三完成。
- **操作内容：** 对未知扩展和局部 JSON 的剩余边界完成独立澄清、实现处理或明确排除范围；不要因 Build21 已处理 R27-08/09 相关问题而自动关闭本项。
- **完成条件：** 有独立的设计/问题结论、实现证据或明确排除依据，并更新本条状态。

### 步骤五：整改核心工程约束与一致性问题

- **关联问题标号：** R28-07（N-core-1～N-core-6 等）。
- **前置条件：** 步骤四完成。
- **操作内容：** 按 AGENTS.md 逐项处理初始化标记、隐藏组解析 Token、错误处理、接入层与存储分层、包级全局状态、验证码 Secret 及其他列出的工程约束，并为每项补充回归。
- **完成条件：** N-core-1～N-core-6 等实际纳入范围的问题逐项关闭，或迁移到明确的专项文档并保留链接。

### 步骤六：实施安全专项整改

- **关联问题标号：** R28-08（N01～N07）。
- **前置条件：** 步骤五完成。
- **操作内容：** 按 SecurityReport2/3 的实际范围，逐项处理依赖与 CI、备份加密、验证码 fail-open、重置端点、JWT 存储、安全头/CSP、OIDC 首设密码通知等问题。
- **完成条件：** N01～N07 逐项有修复、验证和状态记录；未实施项必须明确保留为待办，不得写成已完成。

### 步骤七：完成项目级工程与文档收尾

- **关联问题标号：** R28-09。
- **前置条件：** 步骤三、步骤四、步骤五、步骤六全部完成。
- **操作内容：** 处理运行镜像与 digest、许可证、失效参考链接、未引用前端文件、PoolTab 文案等剩余项目级事项；已修正的文档项只保留实际证据，不重复实施。
- **完成条件：** 每一项关闭，或迁移到对应专项并保留可追踪链接。

### 步骤八：全量复核与 Issue14 关闭

- **关联问题标号：** R28-01～R28-09，以及步骤三中的 D3-1～D3-10、步骤五中的 N-core-1～N-core-6、步骤六中的 N01～N07。
- **操作内容：** 按实际变更范围重新执行后端全量测试、竞态测试、`go build ./...`、`go vet ./...`、前端全量测试、`npm run build`、正式 Production smoke 和 `git diff --check`；核对 Build21、Build22、Design3、Design4、ProdTestList 和本文件的状态。
- **完成条件：** 所有纳入本轮范围的问题均有关闭证据或明确迁移记录；Issue14 不再存在无归属、无状态或无后继文档的问题。

补充说明：`Build23.md` 不作为新的操作步骤启动。其 R27-09/N-node-3/4 内容已归入 Build21；N-node-6 已在步骤四作为 R28-06 跟踪；Build23 继续保留为交接与边界说明。

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
- **现象/范围：** Build16/Design3 仍不能视为全部闭环，当前代码仍存在以下实质缺口：
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
- **当前证据：** [Build22.md](Build22.md) 进度表 Step 1～11 全部为“☐ 未开始”；代码中 `render_clash.go`/`render_sr.go` 仍按类型支持度无条件追加 `no-resolve`，`load.go` 仍丢弃 `Options.NoResolve`，`pipeline.go` 仍存在重复累加，`sync.go` 仍写入占位 `sort_order/raw_line/line_no`，后端素材池白名单/快照 API/前端 pending UI/回执展示均未实现。
- **前置条件：** 步骤一、步骤二已完成，Build21 Step 14 已按证据收口；不得在此前开始 Build22 的任何子步骤。
- **修复方向：** 按 [Build22.md](Build22.md) 的 Step 1～11 串行实施并验收；完成前不得将 Build16/Design3 标记为“全部闭环”。
- **状态：** ☐ 待实施（Build22 未开始）

### 问题 R28-06：未知扩展/局部 JSON 边界（N-node-6）

- **关联步骤：** 步骤四。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §5.3 N-node-6；交接说明 [Build23.md](Build23.md) §二.7。
- **现象/范围：** 未知扩展/局部 JSON 的边界未单独闭环；与 R27-08/09 相关的面板崩溃和输出语义已由 Build21 处理，但“未知扩展/局部 JSON 的剩余边界”仍需明确处理或排除。
- **修复方向：** 作为独立 Issue/Design 项明确处理或排除，不随 R27-09 主体自动关闭。
- **状态：** ☐ 待独立澄清 / ☐ 待处理

### 问题 R28-07：核心工程约束与一致性问题（BuildReport4 N-core-1～6 等）

- **关联步骤：** 步骤五。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §3.2、§6.3。
- **现象/范围：**
  - N-core-1：首管理员初始化标记只写不读；
  - N-core-2：自定义订阅用户每次首页加载重新生成隐藏组解析 Token；
  - N-core-3：仍存在忽略 error；
  - N-core-4：接入层直接访问存储；
  - N-core-5：少量包级全局状态；
  - N-core-6：验证码 Secret 明文存储/明文返回，与 AGENTS 冲突；
  - 另有文件上传/导入端点体积豁免且整读内存、部分历史 Tailwind 类/SSE 例外等 AGENTS 边界项。
- **修复方向：** 按 AGENTS.md 工程约束逐项整改并补回归；其中 N-core-7 已被后续 Build4/Design2 口径覆盖，不作为缺陷重复登记。
- **状态：** ☐ 待整改（N-core-1～6 等）

### 问题 R28-08：安全历史未落地项（SecurityReport2/3 N01～N07）

- **关联步骤：** 步骤六。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §6.5、[SecurityReport2.md](docs/reports/SecurityReport/SecurityReport2.md)、[SecurityReport3.md](SecurityReport3.md)。
- **现象/范围：** N01～N07 仍未落地：依赖漏洞/CI 门禁、备份未加密、验证码 fail-open、重置端点无限流且令牌明文、JWT 存 localStorage、安全头/CSP 缺口、OIDC 首设密码无邮件通知。
- **修复方向：** 按安全报告修复方案分步实施；相关工程跟踪以本文件为准，安全报告保留历史与证据。
- **状态：** ☐ 待实施（N01～N07）

### 问题 R28-09：其他 BuildReport4 项目级工程/文档收尾

- **关联步骤：** 步骤七。

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §6.1、§6.4、§6.6。
- **现象/范围：**
  - 运行镜像未显式安装 `ca-certificates`，基础镜像/GHCR 未固定 digest；
  - AGENTS 文档清单原先缺 BuildReport2/3/4、SecurityReport2/3 等报告（已在本轮同步修正）；
  - README 提及 LICENSE 但仓库未发现 LICENSE；
  - `docs/Reference/Xray-Server-Config-Research.md` 存在指向仓库外 `Xray-examples` 的失效链接；
  - 前端存在未引用文件清理候选（`GenerateStep.vue`、`PreviewState.vue`、`ResponsiveCollection.vue`、`CopyField.vue`）；
  - `PoolTab.vue` 的“停机错过不补跑”文案与当前启动补跑实现不一致。
- **修复方向：** 文档类问题在本轮文档交叉审核中同步修正或登记；代码清理/镜像加固作为后续工程项处理。
- **状态：** ☐ 文档/工程收尾待办

---

## 四、关闭条件

- **步骤一关闭：** ☑ R28-01～R28-04 的工程修复和重新验证均有证据，正式 smoke 使用仓库正式脚本完整通过。
- **步骤二关闭：** ☑ PT-28-01～PT-28-05 已由用户确认完成且未发现问题，相关人工项目已从 ProdTestList 当前待办移除；Build21 Step 14 已完成验收收口。
- **步骤三关闭：** Build22 子步骤全部验收通过，D3-1～D3-10 已完成，Build16/Design3 状态已同步。
- **步骤四关闭：** R28-06 已有独立澄清、处理或明确排除依据。
- **步骤五关闭：** R28-07 的实际纳入范围已逐项关闭，或已迁移到明确的专项文档。
- **步骤六关闭：** R28-08 的 N01～N07 已逐项验证，未完成项仍明确标记为待办。
- **步骤七关闭：** R28-09 的每项已关闭，或已迁移到对应专项并保留链接。
- **步骤八关闭：** 按实际变更范围重新执行后端全量测试、竞态测试、`go build ./...`、`go vet ./...`、前端全量测试、`npm run build`、正式 Production smoke 和 `git diff --check`；Issue14 中每个问题均有关闭证据或明确后继文档。

---

## 五、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
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
