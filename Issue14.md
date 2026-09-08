# Issue14.md — VPN 订阅管理系统问题追踪（当前）

> **文档定位：** 本文承接 [Issue13.md](Issue13.md) 的 R27-09 / Build21 收口核验，并汇总 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 全量核验中仍未闭环的工程问题；只记录除用户真机人工验收之外，当前仍未完成、待处理或新发现的工程问题与验收证据缺口。用户需要亲自执行的 Production、浏览器和客户端人工测试统一迁移至 [ProdTestList.md](ProdTestList.md)，结果以该清单为准。
> 关联构建：[Build21.md](Build21.md) §7.11 Step 14、[Build22.md](Build22.md)（D3 实施计划）；交接说明：[Build23.md](Build23.md)；设计基线：[Design3.md](Design3.md)、[Design4.md](Design4.md)；编码约束：[AGENTS.md](AGENTS.md)。

---

## 一、当前总体结论

- **创建时间：** 2026-09-08
- **来源：** [核验 Build21 构建问题](thread://01a080de-c26e-7540-9db2-6b7b808f5eaa)、[继续 Build21 Step 14 测试](thread://01a080ce-4c83-7830-b7c2-aca36ba501b9)、[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 及当前工作区文档核对。
- **已通过：** Build21 Step 7～13、Step 15 的现有自动化回归未发现新的编译或测试失败；后端全量测试、指定竞态测试、编译、`go vet`，前端 41 个测试文件 / 209 个用例和生产构建均有通过记录。
- **当前状态：** Build21 Step 14 仍未完成；BuildReport4 中仍未闭环的工程问题（D3-1～D3-10、N-core-1～6、N-node-6、安全 N01～N07 等）已按来源归入本文件。需要用户亲自执行的真机/人工项目见 [ProdTestList.md](ProdTestList.md)，不在本文件重复登记为缺陷。
- **本轮边界：** 未修改业务代码、前端组件或测试脚本；只做问题迁移、文档同步和验收口径收口。

---

## 二、进行中问题

### R28-01 动态插件输入操作产生 Ant Design 控制台异常

- **来源：** Step 14 隔离浏览器真实 API 流程。
- **现象：** 在未知 SS 插件动态参数切换或输入期间捕获两次相同异常：

  ```text
  TypeError: Cannot read properties of null (reading 'input')
  ```

- **当前证据：** 异常堆栈落在当前生产构建的 Ant Design 输入组件事件处理路径。保存、重新打开、未知参数保留及 375px 无横向溢出仍已验证，但不能据此宣称浏览器控制台无错误通过。
- **可能根因：** 动态分支切换时输入组件被销毁或重建，事件处理器仍访问已失效的输入引用；具体组件时序尚未通过最小复现确认。
- **影响范围：** 未知插件参数的新增、编辑、切换和删除交互；可能影响输入稳定性，是否造成数据丢失尚未确认。
- **修复方向：** 先以真实用户操作和最小动态分支复现，确认组件卸载/重建与事件绑定时序，再修复并补充浏览器控制台清洁、输入保留和保存重开回归。
- **人工验证边界：** 用户手动浏览器走查、控制台结果和响应式结果记录在 [ProdTestList.md](ProdTestList.md) 的 Build21 Step 14 人工验收区；本问题本身仍需工程复现与修复。
- **状态：** ☐ 待复现定位 / ☐ 待修复

### R28-02 Production smoke 应急状态断言大小写不匹配

- **来源：** 实际运行 `bash .smoke-test-prod.sh` 的 Step 14 核验。
- **现象：** `.smoke-test.sh` 使用 Python 读取 JSON 布尔值后得到 `False`，脚本却将其与小写字符串 `false` 比较，正常返回会被判定失败。
- **证据：** [.smoke-test.sh](.smoke-test.sh:94)～[.smoke-test.sh](.smoke-test.sh:100)。
- **影响范围：** 原始 Production smoke 无法作为直接全绿的验收脚本；当前服务端应急状态并未因此被证明异常。
- **修复方向：** 统一脚本 JSON 值解析与比较方式，保留对 `true`/`false` 的严格布尔断言；修复后重新运行原始 Production smoke。
- **状态：** ☐ 待修复 / ☐ 待重新执行

### R28-03 Production smoke Clash 请求缺少 `fallback_group_members`

- **来源：** R28-02 修正后的临时执行流继续运行时发现。
- **现象：** `.smoke-test.sh` 的 Clash 装配请求没有传当前接口要求的 `fallback_group_members`，服务端返回无法归属流量组缺少成员的 400 错误。
- **证据：** [.smoke-test.sh](.smoke-test.sh:120)～[.smoke-test.sh](.smoke-test.sh:123)；请求模型字段见 [models.go](backend/internal/assembly/models.go:168)～[models.go](backend/internal/assembly/models.go:176)。
- **影响范围：** 原始 smoke 无法完成 Clash 及后续装配/v2 往返步骤。
- **修复方向：** 按当前装配契约补齐固定组成员请求夹具，修复后重新执行未做临时转换的正式脚本；不修改业务接口契约。
- **状态：** ☐ 待修复 / ☐ 待重新执行

### R28-04 固定 Mihomo 验收测试允许未设置二进制时静默跳过

- **来源：** 两个 Step 14 任务结论之间的证据复核。
- **现象：** [mihomo_ssplugin_test.go](backend/internal/assembly/mihomo_ssplugin_test.go:15) 在 `MIHOMO_11929_BIN` 未设置时调用 `t.Skip`，测试进程仍以成功退出。因此全量测试通过不能单独证明固定 Mihomo 1.19.29 正反例实际执行。
- **影响范围：** Step 14 固定版本验收证据；可能把“未执行”误读为“通过”。
- **修复方向：** 在正式验收入口显式提供并校验 Mihomo Meta v1.19.29 二进制；必要时为验收命令增加强制模式，使缺少二进制时失败而不是跳过。此前带显式环境变量的定向通过记录可保留，但需与本次全量测试区分。
- **状态：** ☐ 待补强验收门禁 / ☐ 待重新执行

### R28-05 Build16/Design3 未闭环（D3-1～D3-10）

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
- **修复方向：** 按 [Build22.md](Build22.md) 的 Step 1～11 串行实施并验收；完成前不得将 Build16/Design3 标记为“全部闭环”。
- **状态：** ☐ 待实施（Build22 未开始）

### R28-06 未知扩展/局部 JSON 边界（N-node-6）

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §5.3 N-node-6；交接说明 [Build23.md](Build23.md) §二.7。
- **现象/范围：** 未知扩展/局部 JSON 的边界未单独闭环；与 R27-08/09 相关的面板崩溃和输出语义已由 Build21 处理，但“未知扩展/局部 JSON 的剩余边界”仍需明确处理或排除。
- **修复方向：** 作为独立 Issue/Design 项明确处理或排除，不随 R27-09 主体自动关闭。
- **状态：** ☐ 待独立澄清 / ☐ 待处理

### R28-07 核心工程约束与一致性问题（BuildReport4 N-core-1～6 等）

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

### R28-08 安全历史未落地项（SecurityReport2/3 N01～N07）

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §6.5、[SecurityReport2.md](docs/reports/SecurityReport/SecurityReport2.md)、[SecurityReport3.md](SecurityReport3.md)。
- **现象/范围：** N01～N07 仍未落地：依赖漏洞/CI 门禁、备份未加密、验证码 fail-open、重置端点无限流且令牌明文、JWT 存 localStorage、安全头/CSP 缺口、OIDC 首设密码无邮件通知。
- **修复方向：** 按安全报告修复方案分步实施；相关工程跟踪以本文件为准，安全报告保留历史与证据。
- **状态：** ☐ 待实施（N01～N07）

### R28-09 其他 BuildReport4 项目级工程/文档收尾

- **来源：** [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §6.1、§6.4、§6.6。
- **现象/范围：**
  - 运行镜像未显式安装 `ca-certificates`，基础镜像/GHCR 未固定 digest；
  - AGENTS 文档清单原先缺 BuildReport2/3/4、SecurityReport2/3 等报告（已在本轮同步修正）；
  - README 提及 LICENSE 但仓库未发现 LICENSE；
  - `docs/Reference/xray-server-side.md` 存在指向仓库外 `Xray-examples` 的失效链接；
  - 前端存在未引用文件清理候选（`GenerateStep.vue`、`PreviewState.vue`、`ResponsiveCollection.vue`、`CopyField.vue`）；
  - `PoolTab.vue` 的“停机错过不补跑”文案与当前启动补跑实现不一致。
- **修复方向：** 文档类问题在本轮文档交叉审核中同步修正或登记；代码清理/镜像加固作为后续工程项处理。
- **状态：** ☐ 文档/工程收尾待办

---

## 三、已迁移至 ProdTestList 的人工验收范围

以下内容不是本文件的工程缺陷条目，统一由用户在 [ProdTestList.md](ProdTestList.md) 执行并记录结果：

- Mihomo 1.19.29 / Clash Verge Rev 2.5.2 的代表配置实际导入与连接；
- Shadowrocket 真机导入、连接及 SS 插件和 VMess/VLESS SR TLS 字段生效情况；
- 最新 Production 构建的浏览器人工走查、动态插件输入、控制台无异常、保存重开和移动端交互；
- 修订正式 smoke 夹具后的用户侧 Production 执行结果。

人工结果填写完成后，由 [Build21.md](Build21.md) 和本文件引用 [ProdTestList.md](ProdTestList.md) 的结果更新状态，不以 URI 生成、单元测试或离线 YAML 解析替代真机结论。

---

## 四、验收与关闭条件

1. R28-01 完成最小复现、修复和浏览器控制台清洁回归。
2. R28-02、R28-03 修订正式 smoke 夹具，并在不做临时转换的情况下完整运行通过。
3. R28-04 在显式 Mihomo Meta v1.19.29 环境执行正反例，缺少二进制时不得把跳过当作通过。
4. R28-05 按 [Build22.md](Build22.md) 完成 D3-1～D3-10，并将 Build16/Design3 状态收口。
5. R28-06 对未知扩展/局部 JSON 边界完成独立澄清或处理。
6. R28-07 按 AGENTS 完成核心工程约束整改。
7. R28-08 按安全报告完成 N01～N07。
8. R28-09 的文档/工程收尾项逐项关闭或移入对应专项。
9. [ProdTestList.md](ProdTestList.md) 中用户人工项目填写实际环境、日期、结果和证据；Build21 Step 14 再按清单结果完成最终收口。
10. 完成后至少重新执行后端全量测试、竞态测试、`go build ./...`、`go vet ./...`、前端全量测试、`npm run build`、正式 Production smoke 和 `git diff --check`。

---

## 五、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-08 | 从 Build21 Step 14 与 Issue13 的当前核验结论建立 Issue14；迁移除真机人工验收外的未完成项、新发现错误和固定版本证据缺口；人工项目统一转由 ProdTestList 管理。 |
| v1.1 | 2026-09-08 | 文档交叉审核扩展 Issue14 范围：按用户确认将 BuildReport4 中仍未闭环的工程问题归入本文件（R28-05～R28-09），覆盖 D3-1～D3-10、N-node-6、N-core-1～6、安全 N01～N07 及项目级收尾项；未修改 BuildReport4 归档文件。 |
