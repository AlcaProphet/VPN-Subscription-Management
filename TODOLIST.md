# TODOLIST.md — 临时工作顺序跟踪（2026-09-10）

> **文件性质：** 本文件是仓库根目录的临时跟踪清单，不是 AGENTS.md 正式文档体系的一部分，不替代 [Issue14.md](Issue14.md)、[Issue15.md](Issue15.md)、[Build22.md](Build22.md)、[Build25.md](docs/reports/Build/Build25.md)、[Design3.md](Design3.md)、[Design4.md](Design4.md)、[ProdTestList.md](ProdTestList.md) 等正式文档的验收与状态记录。
> **冻结声明（2026-09-10）：** 用户要求暂不处理任何问题。本文件只登记、排序和跟踪，不授权修复、不修改代码、不修改正式文档状态。
> **使用方式：** 每项完成后，先回写对应主跟踪文档并完成归档，再在本文件勾选；本文件不产生独立完成结论。

---

## 0. 当前活跃文档快照

| 文档 | 位置 | 当前状态 |
|---|---|---|
| [Issue14.md](Issue14.md) | 根目录 | 活跃。步骤一/二/四/六已关闭；步骤三（Build22 Step 7 证据缺口）、步骤五（R28-07A～E/G～I 未实施）、步骤七（R28-09 未完成）、步骤八（最终复核未执行）仍未关闭 |
| [Issue15.md](Issue15.md) | 根目录 | 活跃。已登记 R29-01～R29-12；其中 R29-01、R29-06（待人工）、R29-09/R29-10（工程已修复、待人工）、R29-11～R29-12 未关闭 |
| [Build22.md](Build22.md) | 根目录 | 活跃未归档。Step 1～6、8～10 代码与测试成立；Step 7 专属自动化证据缺口待补，Step 11 不能声明 D3 全验收 |
| [Build25.md](docs/reports/Build/Build25.md) | 已归档 | 已归档。R28-06 缺口修复、定向/全量/build/vet/Docker/Production smoke 重新通过；人工项仍在 ProdTestList |
| [Design3.md](Design3.md) | 根目录 | 活跃未归档。已经 Build16 构建；因 Build22 Step 7 证据缺口未补而暂不归档 |
| [Design4.md](Design4.md) | 根目录 | 当前最新设计；已同步 R28-06 缺口修复闭环状态，人工/真机项由 ProdTestList 跟踪 |
| [ProdTestList.md](ProdTestList.md) | 根目录 | 活跃。仅保留人工核验项；R28-06 自动化缺口已闭环，人工项仍待执行 |
| [SecurityScanPlan1.md](SecurityScanPlan1.md) + [SecurityReport3.md](SecurityReport3.md) | 根目录 | 活跃。Step 1～3 已完成；Step 4～28 尚未开始，报告未完成 |
| [Build21.md](docs/reports/Build/Build21.md)、[Build23.md](docs/reports/Build/Build23.md)、[Build24.md](docs/reports/Build/Build24.md)、[Build25.md](docs/reports/Build/Build25.md) | 已归档 | 已归档，仅核查 |

---

## 1. 先行确认项（未确认前不进入对应实施）

- [ ] **T-000-1（R29-01 原始证据）** 确认 Issue15 R29-01 的原始入口是 Setup 新库导入、管理面板导入还是旧镜像；确认报告中的 `RESET` 与当前代码 `DISABLE` 的差异。主跟踪：Issue15 R29-01 / ProdTestList §三。
- [ ] **T-000-2（R29-12 Dockerfile 变更）** 确认提交 `f8d7474` 将 `Dockerfile` 从 `node:22-alpine` 改为 `node:24-alpine` 是否有意；决定补记验证还是回滚。主跟踪：Issue15 R29-12 / Dockerfile。
- [x] **T-000-3（R29-10 排序预期）** 已确认采用方案 A：高级 JSON 保存定位统一为“路径长度+字典序”稳定排序。主跟踪：Issue15 R29-10 / Issue14 R28-06C。
- [x] **T-000-4（执行顺序门禁）** 用户已明确授权只执行 Build25/R28-06，不进入 R28-05；本轮按定向顺序例外处理。

---

## 2. 工作流

### T-100 Build25 / R28-06 关闭（优先：用户可见缺陷）

**目标：** 修复 R28-06B 前端白名单缺口与 R28-06C 的排序/证据缺口，重跑 Step 4 门禁，关闭 Issue14 步骤四，归档 Build25。

- [x] **T-101 R29-09：** `ProtocolFieldEditor.vue` 的固定对象 JSON 白名单加入 `field.item_id_field`，使 WireGuard `peers._credential_id` 可正常应用/保存/检查，同时不进入客户端产物；补单测、多 Peer、重排回归。
- [x] **T-102 R29-10：** 已按 T-000-3 决策统一保存定位为稳定排序；补多草稿保存定位测试。
- [x] **T-103 R28-06C 证据：** 已补“条件隐藏清理”专项回归；Build25 中保存/检查/父阻断稳定排序表述已修正。
- [x] **T-104 Build25 Step 4 门禁：** 后端全量/build/vet、前端全量/build、`docker compose build`、`bash .smoke-test-prod.sh`、`git diff --check` 均已执行通过。
- [x] **T-105 文档关闭：** 已回写 Issue14 R28-06/步骤四、Build25、Design4、ProdTestList、AGENTS、Issue15、Build23；Build25 已移入 `docs/reports/Build/`。
- [ ] **T-106 人工复验（用户）：** 按 ProdTestList `R28-06` §B/§C 执行 WireGuard 高级 JSON 与多草稿定位核验。

### T-200 Build22 / R28-05 关闭（D3 证据补齐）

**目标：** 补齐 Build22 Step 7 专属自动化证据，重跑 Step 11，关闭 Issue14 R28-05，归档 Build22 与 Design3。

- [ ] **T-201** 补 `ActivatePending` / `DiscardPending` 与 `activated_at` 写入、激活不改 stats/decision、历史 active/failed 不补造时间。
- [ ] **T-202** 补 `SanitizeStoredSyncOutputs` 幂等、非破坏性清洗测试。
- [ ] **T-203** 补 `/sync/status`、`/sync/tasks`、`Pool.sync_error` 读时脱敏 raw JSON 测试。
- [ ] **T-204** 补 `NormalizeDiagnostics` 19+1 截断与 200 rune 限额测试。
- [ ] **T-205** 补 v1 `rule_counts` 分项合计不变量、`previous_active` 比较、旧 stats `version 0` 解析测试。
- [ ] **T-206** 补 `latest_failed` 恢复与同时间戳 ID 排序后端测试。
- [ ] **T-207** 补 failed 快照写失败时 active/pending 不变、不虚报 snapshot ID 的测试。
- [ ] **T-208** 补旧字符串数组 Clash plan 下载重渲染回退路径测试。
- [ ] **T-209 Build22 Step 11 门禁：** 后端 build/vet/全量、前端 build/全量、Docker build、Production smoke、`git diff --check`。
- [ ] **T-210 文档关闭：** 回写 Issue14 R28-05/步骤三、Build22、Design3、AGENTS；归档 `Build22.md` 与 `Design3.md`。
- [ ] **T-211 人工复验（用户）：** 按 ProdTestList Build22 Step 8/9 章节执行真实浏览器/装配回执核验。

### T-300 导入路径合并处理（R29-01 + R28-07G）

**目标：** 一次性收口 Setup/管理端导入确认词与上传上限，避免重复改导入路径。

- [ ] **T-301 R29-01：** 修复 Setup 新库导入无法满足 `disable_confirm_word=DISABLE` 的问题，使高级模式关闭时的 v2 导入流程可用。
- [ ] **T-302 R29-01 测试：** 补 `ImportV2` 入口级测试，覆盖 Setup 与面板两条入口。
- [ ] **T-303 R28-07G：** 20 MiB 文件/请求体上限、`MaxBytesReader`/有界读取、EOF 与真实读取错误区分、413 映射、前端提前拒绝。
- [ ] **T-304 门禁：** 后端定向/全量、前端定向/全量、接口级 413 回归、`git diff --check`。
- [ ] **T-305 人工复验（用户）：** 按 ProdTestList §三执行 R29-01 修复后新库导入与引用关系核对。

### T-400 Issue14 步骤五：R28-07 其余整改

- [ ] **T-401 R28-07A：** 删除首管理员初始化冗余标记写入，保留用户计数/创建为唯一事实来源；补注册/OIDC/并发首建回归。
- [ ] **T-402 R28-07B：** 修正首页自定义订阅与隐藏组 Token 的创建/清理顺序，做业务键幂等协调；补上传后首次返回、删除自定义订阅恢复组 Token、历史双 Token 修复回归。
- [ ] **T-403 R28-07C：** 生产代码忽略 error 全量审计、分类处理、结构化日志/回滚与静态门禁。
- [ ] **T-404 R28-07D：** 将用户下载渲染、流量汇总、OIDC 换票事务移出接入层；补架构检查禁止 `internal/server` 直接访问 `DB()`/`TxImmediate()`。
- [ ] **T-405 R28-07E：** 去除可变包级状态（调试回调、默认 Logger/Level、敏感键注册表），改为构造注入/不可变集合；补多实例与并发回归。
- [ ] **T-406 R28-07G：** 见 T-303，完成后在本项互链勾选。
- [ ] **T-407 R28-07H：** 清理 `NodeCheckPanel.vue` 遗留 gray/white 类，补静态扫描和双主题断言。
- [ ] **T-408 R28-07I：** SSE 日志流改走管理员路由 + `fetch`/`ReadableStream` Bearer 鉴权，删除一次性 token 路径；补 401/403、权限变化、重连、卸载回归。
- [ ] **T-409 门禁与文档：** 每小项按受影响范围执行后端/前端门禁，并同步 Issue14、Design1（SSE 例外）、AGENTS。

### T-500 Issue14 步骤七：R28-09 项目级收尾

- [ ] **T-501** 按 T-000-2 结论处理 Dockerfile Node 版本记录；同时补 `ca-certificates`、基础镜像/GHCR digest 固定。
- [ ] **T-502** 补 `LICENSE`，或修正 README 中与许可证相关的描述。
- [ ] **T-503** 修复 `docs/Reference/Xray-Server-Config-Research.md` 指向仓库外 `Xray-examples` 的失效链接。
- [ ] **T-504** 清理未引用前端文件：`GenerateStep.vue`、`PreviewState.vue`、`ResponsiveCollection.vue`、`CopyField.vue`；逐项确认无引用后删除。
- [ ] **T-505** 执行受影响构建/扫描门禁，回写 Issue14 R28-09 与 AGENTS。

### T-600 第三期安全审查（SecurityScanPlan1 Step 4～28）

> 建议在本轮代码整改基本冻结后启动，避免审查基线漂移。

- [ ] **T-601** 执行 Step 4～24 静态/领域审查。
- [ ] **T-602** 执行 Step 25～27 匿名、角色越权、恶意输入与历史修复回归。
- [ ] **T-603** 执行 Step 28：发现去重、风险校准、OWASP 总结、形成最终 SecurityReport3。
- [ ] **T-604** 根据结论决定修复/Build/Issue 候选，并决定 SecurityScanPlan1 / SecurityReport3 是否归档。

### T-700 Issue14 步骤八：最终复核与关闭

> 必须在 T-100～T-600 全部关闭后执行。

- [ ] **T-701** 重新执行后端全量/竞态、`go build`/`go vet`、前端全量/build、Production smoke、`git diff --check`。
- [ ] **T-702** 核对 Issue14、Issue15、Build22、Build25、Design3、Design4、ProdTestList、AGENTS 状态一致。
- [ ] **T-703** 关闭 Issue14 并归档；若 Issue15 所有 R29 也关闭，则一并归档。
- [ ] **T-704** 更新 AGENTS 文档清单与本 TODOLIST 的最终状态。

### T-800 后续设计与新构建（当前不授权）

- [ ] **T-801 Design4 后续范围：** 全量 19 协议条件表单、SS 2022、独立 Xray outbound 等后续设计/构建。
- [ ] **T-802 Build23 候选：** 非 SS 字段级 `target_evidence` 全局诊断等候选的后续决策。
- [ ] **T-803** 以上均须先有新 Design/Build 与用户授权，不并入当前 Issue 收尾。

---

## 3. 人工测试队列（ProdTestList 跟踪，不计为工程未完成）

| 项目 | 触发时点 | 当前状态 |
|---|---|---|
| R29-01 修复后 Production 新库导入 | T-301/T-302 完成后 | 待人工 |
| R28-06 §B WireGuard peers 高级 JSON（R29-09） | T-101 完成后 | 待人工 |
| R28-06 §C 多草稿定位与条件隐藏（R29-10） | T-102/T-103 完成后 | 待人工 |
| R28-06 全量未知扩展/局部 JSON 核验 | T-105 完成后 | 待人工 |
| Build22 Step 8/9 真实浏览器与装配回执 | T-209/T-210 完成后 | 待人工 |
| R29-06 节点动态表单真实运行 | 当前已具备条件 | 待人工 |
| R29-04 真实手机/Production | 当前待安排 | 待人工 |
| R27-05 额外尺寸/焦点/折叠 | 当前待安排 | 待人工 |
| R26-07 Xray PUT 403/400/409 | 当前待安排 | 待人工 |
| R20-11 重启数据保留 | 需要真实环境 | 待人工 |
| OIDC Mock 登录 | 当前待安排 | 待人工 |
| Build11 重置链接四态 | 当前待安排 | 待人工 |

---

## 4. 每项完成后的文档动作

每个工作流关闭时必须：

1. 更新主跟踪文档：Issue14 / Issue15 / Build22 / Build25 / Design3 / Design4 / ProdTestList / AGENTS。
2. 工程验收通过后，才把对应 Build/Design/Issue 文档移入 `docs/reports/` 对应目录。
3. 运行 Markdown 本地链接检查，确保没有失效链接。
4. 在本 TODOLIST 勾选对应项，并记录归档后的路径。
5. 本文件只是临时跟踪；正式结论以主跟踪文档为准。

---

## 5. 变更记录

| 日期 | 变更 |
|---|---|
| 2026-09-10 | 创建临时 TODOLIST，登记当前活跃文档快照、4 个先行确认项、8 个工作流、人工测试队列和文档收尾规则。用户要求暂不处理任何问题，本文件只跟踪不执行。 |
