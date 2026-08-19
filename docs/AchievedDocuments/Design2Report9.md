用户决策已全部返回。现在输出最终结论报告。

---

# 设计与构建方案核验结论报告

## 一、核验范围

| 对象 | 文件 | 规模 |
|------|------|------|
| 设计文档 | `Design2.md`、`Design2-UI.md` | 431 + 668 行 |
| 构建文档 | `Build4.md`、`Build5.md`、`Build6.md`、`Build7.md` | 898 + 525 + 601 + 434 行 |
| 基准参照 | `AGENTS.md`、`docs/AchievedDocuments/Design1.md`、`Design1-UI.md`、`Design2Report5/7/8.md`、`docs/DocTemplates/*`、`docs/Reference/*` | 按需抽查 |
| 既有代码 | `backend/`（migrations、internal 各包符号）、`frontend/`（api、views） | 只读抽查 |

核验维度：① 设计文档正确性/自洽性/可实施性；② 两份设计文档间冲突/重复定义/引用错误/术语一致性；③ Design2-UI 对 Design2 界面、交互状态、前后端契约、错误处理的承载完整性；④ Build4~7 对设计的对齐度与范围边界；⑤ 遗漏点（空态/加载/错误/权限/移动端/暗色/防重复/危险确认/超时轮询/日志脱敏/禁缓存/事务/幂等/级联/候选集重算/凭据加密/Xray 串行与重试）；⑥ 普通用户与管理员双视角动线。

## 二、核验方法

1. 主审完整通读两份设计文档全文，建立独立理解；
2. 委派 6 个只读子代理并行深核：Build4/5/6/7 逐份对齐核验、设计遗漏点 19 项清单核查、设计文档交叉一致性复核（30+ 处双向引用抽查）；
3. 主审对子代理报告的每条问题**逐一回读原文行号亲验**，剔除误报、修正定级；
4. 只读核对被引用资产的真实性：迁移编号（1008 已占用）、模板组名/兜底规则、既有代码符号（`SetStatus(disable bool)`、`BindSubjectIfNull`、request.ts 15s 超时、AssemblyView 占位页等）——全部属实；
5. 全程零修改、零构建、零服务启动，仅使用只读命令。

## 三、核验发现总览

**总体判断：六份文档质量高、对齐度好。** Build4→5→6→7 范围边界清晰、候选构建项链完全闭环（Build7 候选清零），未发现「设计有、Build 无」的遗漏归属，未发现提前越界实现，四份 Build 均满足 AGENTS §8.2 的 Step 要素要求。预设组种子、1009 DDL、DDL 字段/索引/枚举、端点契约、状态机、错误码、确认词 DISABLE 等关键口径经抽查与设计逐字一致。

共确认问题 **38 项**：高 1 / 中 15 / 低 22。其中 9 项已经您决策定案。

## 四、问题清单与用户确认结果

### 高严重级（1 项）

| # | 类型 | 问题 | 证据 | 您的决策 |
|---|------|------|------|---------|
| H1 | 设计冲突 | 实例停用（enabled=0）三方矛盾：§5.6③ 候选集重算 RemoveUser+摘除组分配 vs §5.9/UI§8.1「既有账号保留」；§5.5 触发器表无实例行 | `Design2.md:297` vs `Design2.md:364`、`Design2-UI.md:451`；`Build6.md:259` | **暂停管理口径**：停用仅暂停检测/推送/采集/注入，保留账号与 group_nodes，重启用恢复同步+对账兜底；§5.6③ 移除实例 enabled 触发 |

### 中严重级（15 项，M1~M7 已经您决策）

| # | 类型 | 问题 | 证据 | 您的决策 / 建议 |
|---|------|------|------|----------------|
| M1 | 设计冲突 | OFF 清空独立账号范围：设计/UI 为「清凭据+推送记录、保留账户行」，Build7 为整行删除；保留则重开后无凭据再生路径 | `Design2.md:14,428`、`Design2-UI.md:254` vs `Build7.md:192,213` | **整行删除**，回写 Design2/UI 措辞 |
| M2 | 设计冲突 | 禁用用户 reset-quota：可重置仅跳过重推 vs 整端点 400 拒绝 | `Design2.md:352` vs `Design2-UI.md:231`、`Build6.md:485` | **整端点拒绝**，回写 Design2 §5.8 |
| M3 | 语义不明 | 对账期望集：候选集过滤是否覆盖独立账号；「xray_ext_users 非期望集来源」与「独立账号×其推送记录」矛盾 | `Design2.md:391` vs `Design2.md:424,374`、`Build7.md:123,26` | **按建议统一**：用户部分经候选集+可用性过滤；独立账号期望集=其 xray_ext_users 推送目标、仅可用性过滤 |
| M4 | 需决策 | 强制组名/兜底与模板 emoji 版不一致且未说明 | `Design2.md:94-97,135` vs `Clash.yaml.template.md:131,142,241,319,321` | **与模板对齐**：强制组改 🚀直接连接/🌎国外流量/🛟无法归属的流量，GEOIP,CN,DIRECT（连锁修订见第六节） |
| M5 | 语义不明 | 蓝图重渲染 ②删空组 vs ③强制组降级的优先级、DIRECT 豁免、④范围 | `Design2.md:323` | **按建议补写**：强制组豁免删除统一降级 `[DIRECT]`；④「被剔除组」不含降级组 |
| M6 | 不可实施风险 | settings 异步任务无持久化机制定义（Design2 无表、UI 要求重启 failed、Build7 进程内 registry） | `Design2.md:361,378,396,399`、`Design2-UI.md:586`、`Build7.md:196` | **进程内 registry + 未知 id 合成 failed**，Design2 补注记 |
| M7 | 风险 | gRPC 无超时 + 串行调用×120s 同步端点 | `Design2.md:213,269,327,347`、`Design2-UI.md:585` | **长操作异步任务化**：初始化/对账执行/实例删除改异步+pollTask（配套见第六节注） |
| M8 | Build 未对齐 | Build5 代理组名校验复用节点禁空格规则，与设计（组名允许空格）冲突 | `Build5.md:219,147` vs `Design2.md:103`、`Design2-UI.md:428` | 列入修订：拆 `ValidateNodeName`（禁空格）/`ValidateProxyGroupName`（允许空格） |
| M9 | Build 未对齐 | Build6 Step5 要求「采集间隔映射+读写单测」，承载端点在 Build7 Step2，归置裂缝 | `Build6.md:499-500` vs `Build7.md:186` | 列入修订：映射读写单测挪 Build7 Step2，Build6 仅读内部键 |
| M10 | Build 未对齐 | SR conf benchmark 未具名；阈值断言测试 Step3/7 重复 | `Build5.md:302,344,481` vs `Design2.md:323` | 列入修订：Step3 具名 `BenchmarkRenderSrConf10kRules`+阈值测试，Step7 删重复 |
| M11 | 遗漏 | 日志/错误串脱敏口径缺失（last_error/collect_error 可能含 api_addr，直接 UI 展示） | `Design2.md:280,348,364`、`Design2-UI.md:46,229`；AGENTS §4.3 仅 token+5xx | 列入修订：补错误串白名单/脱敏口径 |
| M12 | 遗漏 | 节点 enabled 停用无确认（触发 RemoveUser，与 is_public 不对称）；删除订阅确认清单未提示候选集重算副作用 | `Design2-UI.md:385,184` vs `Design2.md:266,297` | 列入修订：补 ConfirmModal 与影响清单项 |
| M13 | 遗漏 | 版本列表空态未定义，且引用的 Design1-UI §7.5 无该条目 | `Design2-UI.md:633` vs `Design1-UI.md:305-318` | 列入修订：§10.2 新增版本列表空态 |
| M14 | 语义不明 | 120s 清单不闭合；diff 钩子同步/异步未定义 | `Design2-UI.md:585`、`Design2.md:252,263-268` | 列入修订（随 M7 异步化统一明确：钩子事务提交后异步执行、状态经 xray_users 可见） |
| M15 | 语义不明 | OFF 重复执行幂等未定义（已 OFF 再保存也要求 DISABLE） | `Design2-UI.md:558` | 列入修订：已 OFF 再保存为幂等 no-op，仅状态翻转才要求 DISABLE |

### 低严重级（22 项，已经您确认全部列入修订清单）

- **引用/术语**：Design2 引 UI §7.1 应为 §7.3（`Design2.md:98`）；api/group.ts 标题与 off 不 403 矛盾（`Design2-UI.md:526,530`）；「四色」与 6 状态 5 色不符（`Design2-UI.md:623,46`）；target_syntax/product_type 建议补显式映射表（`Design2.md:144,379`）。
- **UI 内部**：DiffView 无激活版本分支不可达（`Design2-UI.md:42,359`）；池 Badge 数据源两处表述（`Design2-UI.md:285,507`）。
- **承载缺口**：「流量报表」端点无 UI 落点（`Design2.md:391`）；traffic_card_enabled 入 status 与采集间隔键名仅 UI 定义（`Design2.md:392,400` vs `Design2-UI.md:592,557`）。
- **Build 瑕疵**：Build5 约束表漏「空格」(`Build5.md:26`)、函数命名三处漂移（`Build5.md:161,219`、`Build6.md:188`）、NodeView 未定义（`Build6.md:124`）、protocols 端点 label 字段契约外（`Build5.md:121` vs `Design2-UI.md:521`）、伪代码 YouTube=url-test 与种子 select 不符（`Build5.md:331`）；Build6 文件清单漏 errors.go（`Build6.md:78,130`）、下载去重兜底未显式（`Build6.md:405` vs `:324`）；Build4 Step2 清单漏 2 个测试文件（`Build4.md:467`）；Build7 缺快照重绑注记（`Build7.md:201` vs `Design2.md:364`）、v1 导入兼容未见于设计（`Build7.md:200`）、归档范围未定（`Build7.md:396` vs `AGENTS.md:198`）。
- **小遗漏**：对账单分区空态、检测零新增提示、无用量数据显示「—」、getExtCredentials 补 no-store、DiffView 暗色 variant、检测 upsert 事务边界、平台删除级联链入重算触发、§5.5 与 §5.6 触发清单对齐（随 H1 决策一并处理）、手动规则行「追加在池后」回写设计、1009 旧库执行风险提示、SSRF 部署提示、超限管理员文案微调。

## 五、最终结论

**核验通过（附条件）**：设计文档与构建文档整体正确、自洽、可实施，Build4~7 严格按设计编写且范围闭环，具备进入构建的条件——**前提是先完成下列两项高优先修订**：H1（实例停用语义统一为暂停管理口径）与 M1~M7 决策项的文档回写。其中 M4（强制组对齐模板）与 M7（长操作异步化）为连锁修订，涉及多份文档多处行号，需一次性改齐避免新旧口径并存。中低级别项均已列入修订清单，不阻断构建，但建议在 Build4 开工前一并落盘，以免构建期返工。

**双视角动线结论**：普通用户动线（注册→审批→下载→超限→重置）与管理员动线（开高级→录实例→检测→装配→激活→初始化→对账→OFF）在修订后语义一致、界面表现、权限控制与提示文案无歧义；仅「超限管理员自身表现」「角色变更后首页形态切换」两处建议补一句说明（低级项已覆盖）。

## 六、建议后续处理项（修订清单，按您确认的口径）

**第一批（决策回写，构建前必改）**
1. **H1**：`Design2.md:297` §5.6③ 移除「所属实例 enabled 变化」触发并重写为暂停管理口径；§5.5 触发器表增补说明行；`Build6.md:259` 同步移除实例 enabled 回调触发。
2. **M1**：`Design2.md:14,428`、`Design2-UI.md:254` 改为「删除独立账号整行（含凭据）与推送记录」；Build7 不变。
3. **M2**：`Design2.md:352` 改为「reset-quota 仅 active 用户开放，禁用用户 400 拒绝（前端置灰）」。
4. **M3**：`Design2.md:391` 期望集公式改写（ext 部分=推送目标、仅可用性过滤）；`Build7.md:123` 同步。
5. **M4（连锁）**：强制组名全面换 emoji 版——`Design2.md:94-97,88,105,135,323,366`；`Design2-UI.md:324,395,402,428,430` 等校验提示与装配器锁定展示；`Build4.md:358-366` 种子 `groups:["🚀直接连接"]`；`Build5.md:279-280,321-329` 伪代码与校验文案；兜底改 `GEOIP,CN,DIRECT`+`MATCH,🛟无法归属的流量`；「无法归属」成员 `[🚀直接连接,🌎国外流量]`；并在 §3.3 注明与模板逐字一致（含强制组）。
6. **M5**：`Design2.md:323` 补算法顺序段落（强制组豁免②删除、统一降级、④不含降级组；组名用 emoji 版）。
7. **M6**：`Design2.md:399,401` 补「settings 任务进程内维护、重启后查询一律合成 failed」注记；`Design2-UI.md:586` 补「未知 task id 返回 failed」。
8. **M7（连锁）**：开始初始化/对账执行（push/clean/credentials）/实例删除改异步任务+pollTask——`Design2.md:269,327,391` 与第一章初始化口径、`Design2-UI.md:466,472,454,585`（120s 清单移除这些项、任务 kind 扩展）、`Build6.md` Step3 初始化端点、`Build7.md` Step1 对账执行端点。**配套注意**：异步化后任务体内仍需 gRPC 单次调用 deadline（否则任务挂起无终态），建议落实 M7 时一并在 client.go 定义 dial/call 超时——此点为异步化方案的工程必要配套，如不同意请明示。

**第二批（中低级收口，建议 Build4 开工前落盘）**：M8~M15 与 22 项低级项，按第四节表格「建议」列逐条执行（修订清单已全部含行号，可直接照单修改）。
