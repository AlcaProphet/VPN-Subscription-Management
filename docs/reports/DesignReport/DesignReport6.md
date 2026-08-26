# DesignReport6.md — Design2 / Design2-UI / Build4~7 全量只读核验报告

> **文档定位：** 本报告是 Design2.md、Design2-UI.md 与 Build4.md~Build7.md 的构建前**只读核验**结论（第七轮核验报告，承接 DesignReport1~5）。核验过程未开始构建、未修改任何文档/代码/配置；发现的决策项已经用户确认（见第六节），落地修订动作列入「建议后续处理项」，由用户指令触发执行。
> 参考模板：无独立报告模板，按 [Issue 模板](../../DocTemplates/Issue.template.md) 风格归档。

---

## 一、核验范围

| 对象 | 行数 | 核验内容 |
|------|------|---------|
| Design2.md | 431 | 第一~五章全量：模式分层与开关语义、规则素材池、装配拼接、配置生成与分发、Xray 对接（5.1~5.11） |
| Design2-UI.md | 660 | 十~十一章全量：全局风格、布局路由、用户端、管理面板、装配页、节点/代理组/Xray 实例页、前后端契约、交互约定与空态文案 |
| Build4.md | 891 | Step 0~7：Go 1.26、1009 迁移、旧分发拆除、版本语义、前端基线、规则素材池 |
| Build5.md | 515 | Step 1~7：协议注册表与 manual 节点、代理组、装配渲染内核、装配端点、节点/代理组/装配前端、分发收口 |
| Build6.md | 592 | Step 0~6：xray-core 客户端、实例与检测、高级中间件与组模型、生命周期同步、下载动态渲染、流量配额、假 Xray 集成测试 |
| Build7.md | 426 | Step 1~6：独立账号与对账、OFF 清空与导入导出 v2、Xray 实例页、组/用户/设置高级前端、用户端收口、全量验收归档 |

重点闭环核查项（用户指定）：基础/高级模式边界、advanced_mode 开关语义、OFF 清空、ON 初始化、订阅装配、规则素材池、节点/代理组/候选集、下载渲染、流量配额、Xray 对账、独立账号、配置导入导出、首页/个人中心展示。

---

## 二、核验方法

1. **全量通读 + 交叉核对**：六份文档逐节通读；Design2 ↔ Design2-UI 逐条对照承载完整性；Design2 章节 ↔ Build4~7 Step/执行约束表/验收标准逐项对齐；四份 Build 之间范围边界与候选清单互查（各 Build「候选构建项」表指向是否闭环）。
2. **代码基线事实勘察（只读）**：对 Build 文档引用的代码假设逐一验证（清单见第三节）。
3. **双视角动线走查**：以「普通用户」（注册→首页→下载→超限提示→个人中心）与「管理员」（新部署基础模式动线 / 高级模式动线：开关→录实例→检测→组分配→初始化→对账→OFF 清空→重开）模拟全流程，核对功能语义、界面表现、权限控制与提示文案一致性。
4. **八项隐形漏洞核对**：空态、加载态、错误态、权限可见性、移动端适配（<768）、暗色模式、防重复提交、危险确认；另核对超时/轮询、日志脱敏、下载禁缓存、事务边界、幂等性、级联删除、候选集重算、凭据加密、Xray 串行调用与失败重试等专项口径。

---

## 三、代码基线勘察事实清单（全部成立）

| # | Build 文档假设 | 勘察结论 |
|---|---------------|---------|
| 1 | 迁移编号止于 1008，1009 可用 | ✅ migrations/ 止于 `1008_platform_installers_multi.sql` |
| 2 | `idx_subscriptions_platform` 索引存在（Build4 Step1 DROP 前提） | ✅ `1002_subscriptions_versions.sql` 第 12 行 |
| 3 | `group_selections` / `subscription_group_rel` 现存且引用点明确（group.go / home.go×2 / subscription.go / download.go） | ✅ 与设计 §5.10「groupSelected / updatedAt 两处引用」吻合 |
| 4 | `user.BindSubjectIfNull` 条件更新模式存在（凭据首建守卫参照） | ✅ `internal/user/oidc.go` |
| 5 | `config.Encrypt/Decrypt` + `KeySigningKey` 存在（节点凭据/用户凭据加密复用） | ✅ `internal/config/config.go` |
| 6 | `CreateVersion` 移除 FirstContent 后调用点恰为 5 处 | ✅ versionCreate（server/subscription.go）、rule、share、custom 现各 1 处 + assembly 新增 |
| 7 | 前端全局 timeout 15s、无轮询能力（pollTask 属新实现） | ✅ `api/request.ts` |
| 8 | Dockerfile `golang:1.25-alpine`、README 两处 1.25 标识（Build4 Step0 改造对象） | ✅ Dockerfile:10、README:8/223 |
| 9 | `internal/slug` 包存在（实例 slug 生成复用） | ✅ |
| 10 | home.go `updatedAt` 仅查 `owner_type='subscription'`（设计指明需修复 custom 查询） | ✅ home.go:337，修复点成立 |

---

## 四、发现的问题（分级）

> 级别定义：**P0=阻塞构建的硬冲突**（本次未发现）；**P1=需用户决策的缺口/不对齐**；**P2=次要观察项，Build 期澄清或随文档修订顺带处理**。

### 4.1 P1 — 需用户决策项（3 项，已确认，见第六节）

**Q1【Build 未对齐】manual 节点协议清单范围**
- 现象：Design2 §3.2 表述为「支持 `ClashOfficial.yaml.template.md` 中全部代理协议类型（…等；ssr 除外）」；Build5 执行约束表固化为「严格 19 个协议」。经逐行核对模板，除 ssr 外模板还含 `gost-relay`、`hysteria2-realm`、`socks`（与 socks5 并存）、`sudoku` 四类代理协议，未列入 Design2 正文清单与 Build5 约束表。
- 影响：Build5 Step1 协议注册表实现范围存在「按模板全量还是按 19 封闭清单」的歧义。

**Q2【遗漏设计点/归属缺失】subs / generic-subs 下载 base64 编码的 Build 归属**
- 现象：Design2 §4.5 规定装配模板「存储明文、下载注入完成后整体 base64 下发」。Build5 Step7 端到端动线要求「generic-subs 下载内容整体 base64」，但 Build5 各 Step 均未包含「基础模式下载 base64」的实现步骤；Build6 Step4 将「重新 base64」写在 `RenderUserSubscription`（高级渲染路径）内。基础模式无注入时 SR / v2rayNG 客户端同样必须收到 base64 内容。
- 影响：不补齐则 Build5 里程碑「基础模式完整可用」不成立（SR 客户端无法解析明文下载内容）。

**Q3【遗漏设计点/归属缺失】高级模式两处前端数据源无明确 Build 归属**
- 现象①：`/api/home` 的 `traffic` 字段高级模式形态（`used_bytes / quota_bytes / exceeded`）——Build4 Step3 仅实现基础模式 `{unlimited:true}`；Build6 Step5 补的是 `user/admin.go List`（用户管理页）字段；Build7 Step5 为纯前端，仅写「与 Build4/6 后端响应最终对齐」，未有任何 Step 认领 home 端点的高级字段实现。
- 现象②：`traffic_card_enabled`（流量卡片开关）的前端获取通道未定义——`/api/system/status` 仅暴露 `advanced_mode`；UI §3.1.1/§3.2/§4.7 要求首页流量卡与个人中心「本月流量」行受该开关控制。
- 影响：Build7 Step5 前端无法取数，两处展示在高级模式下落空。

### 4.2 P2 — 次要观察项（6 项，无需决策，Build 期澄清/顺带修订）

| # | 类型 | 位置 | 观察与建议 |
|---|------|------|-----------|
| P2-1 | 语义不明 | Design2 §3.3 vs Build5 Step3 | 强制组「无法归属的流量」的成员构成（Build5 伪代码定为 `[DIRECT, 国外流量]`）与「直接连接」组类型（select）由 Build 伪代码首次明确，Design2 正文仅定义其为 MATCH 兜底目标。建议 Design2 §3.3 顺带补一句成员构成口径。 |
| P2-2 | 语义澄清 | Design2 §5.6 口径③ vs UI §4.3 | 组编辑「非候选集已分配节点」红色警示态实际近乎不可达：SetNodes 拒绝候选集外节点，且激活蓝图集合变化事件触发重算自动删除越界分配；该警示为防御性兜底展示（重算失败/时序窗口）。建议 Build6 实现时保留并注释为防御态。 |
| P2-3 | 交互分支未定义 | UI §5.3.1 ④ | Clash 装配手动补充规则行的类型选择是否含 USER-AGENT 未明示（Design §3.5 已定 Clash 跳过 USER-AGENT）。建议 Build5 在 Clash 场景手动行类型下拉直接排除 USER-AGENT，或渲染层统一跳过并计入跳过项提示。 |
| P2-4 | 口径补明 | Build7 Step1 对账 | 对账端点按实例维度，但 Step1 未显式写「期望集与该实例节点取交集」。实际集按实例取、to_push 天然应仅含本实例目标，建议 Build7 实施时在文案中写明，避免误推其他实例目标。 |
| P2-5 | 可实施性 | Design2 §5.9 / Build7 Step2 | `xray_ext_users.node_id`「0/NULL=待重绑」：SQLite 启用外键时插入 0 会违反 FK（无 id=0 节点）。应统一用 NULL（列可空），Build7 实施时按 NULL 落地。 |
| P2-6 | 验收命令细节 | Build4 Step2 | `grep group_selections` 验证命令未排除 `docs/reports` 与 Build 文档自身（Step7 已排除）。执行时补 `--exclude-dir` 即可。 |

### 4.3 已核验通过的关键闭环（抽样列示）

- **高级模式边界**：侧边栏 13 项菜单显隐（用户组/Xray 仅高级）与 Design2-UI §2.1 顺序一致；路由守卫 + 后端 advancedMode 中间件双保险；groups 基础 CRUD 不纳入高级屏蔽（前端隐藏）——两份文档与 Build4/6 口径一致。
- **OFF 清空**：Design 第一章清单 ↔ UI §4.7.1 确认弹窗清单（确认词 `DISABLE`、manual 节点保留标注、两条 warning）↔ Build7 Step2 事务 SQL（置位同事务、快照预收集、提交后 best-effort RemoveUser、保留 proxy_groups/groups/blueprints）三处逐字段一致。
- **ON 初始化**：开关仅置位不推送；「开始初始化」幂等、跳过空推送集合用户不记失败、超限前置拦截——Design 第一章/§5.5 ↔ UI §8.3 ↔ Build6 Step3 一致。
- **候选集**：口径①（组管理并集）/②（下载按各模板蓝图）/③（事件重算三触发点：版本切换、订阅删除、generate 首版自动激活）在 Design §5.6 ↔ Build6 Step2 完整落点。
- **凭据与并发**：首建守卫（`BEGIN IMMEDIATE` + 同 WHERE 条件更新 + RowsAffected）、OFF 三层防护（入口查库/事务复查/AddUser 后补偿 RemoveUser）、`already exists.`/`not found.` 子串幂等、vmess 不设 alter_id——Design §5.4/§5.5 ↔ Build6 Step0/Step3 一致且有 fake 测试要求。
- **下载渲染**：直接上传原样、蓝图全量重渲染、占位注释优先级（OFF 注释 > 无凭据注释）、SR/generic 重新 base64、无占位不残留、`subscription-userinfo` 仅高级仅用户订阅类、`expire=4102444800`、no-store——Design §5.7 ↔ Build6 Step4 一致。
- **流量配额**：逐用户串行 QueryStats、完整前缀 pattern 禁空、原子 UPSERT 增量、NULL/0 不限、ym 按 UTC、超限仅摘除下载不变、重置仅 active——Design §5.8 ↔ Build6 Step5 一致。
- **对账与独立账号**：期望集来源（active×节点 ∪ ext×推送记录，非状态表）、四分区（含 ext 残留默认不勾选）、凭据不一致移除重推、双轨凭据、推送目标强校验——Design §5.10/§5.11 ↔ UI §8.4/§8.5 ↔ Build7 Step1 一致。
- **导入导出 v2**：instances 全字段 + slug 沿用 + 节点命名映射、accounts + push_targets、signing_key 保护、advanced_mode 一致性三分支（带实例自动开/无实例执行 OFF 清空/v1 兼容）——Design §5.4 ↔ UI §4.7.2 ↔ Build7 Step2 一致。
- **八项隐形漏洞**：新增页面均有空态文案表（UI §10.2）、TriStateList 加载态、409/400/200 注释块错误映射（UI §9.4）、<768 卡片态与拖拽降级上移下移、暗色随 ConfigProvider、权限由 advanced_mode 驱动、长操作 loading 防重复、危险确认清单（UI §10.1）——各章自检结论覆盖完整。
- **轮询与超时**：pollTask 契约（1.5s/5min/卸载取消/3 次网络失败）与长请求统一 120s（装配/初始化/对账/检测）——UI §9.2 ↔ Build4 Step6 / Build5 Step4 / Build7 Step3 一致。

---

## 五、双视角动线走查结论

- **普通用户**：注册→首页四卡（流量/分流规则/平台/公告）→下载（无标识 Token 按平台解析）→超限提示可达（首页红色 alert + 个人中心附注，下载不阻断）→个人中心本月流量；基础模式「不限流量」/高级模式用量两态一致；自定义订阅优先、无激活版本占位文案与 200 注释块口径对齐。无歧义。
- **管理员**：新部署动线（建平台→建订阅条目→装配→入池引导→激活/分发→用户可见）与高级动线（开关→录实例→检测命名→组分配→开始初始化→用量/超限→对账→独立账号→导入导出→OFF 清空→重开）均有 UI 规格与 Build Step 落点，提示文案（「已入池未生效，请激活」「首个版本已自动激活」「高级功能未开启」等）两处文档口径一致。无歧义。

---

## 六、用户确认结果（2026-08-19）

| 编号 | 决策项 | 用户确认结论 |
|------|--------|-------------|
| Q1 | manual 协议清单范围 | **维持 19 协议封闭清单**（ssr 除外）；小幅修订 Design2 §3.2 措辞为封闭清单，消除「模板全量」歧义。理由：链接映射取证仅覆盖既有协议，gost-relay/sudoku 等无可靠转换参照，与 ssr 移除同因。 |
| Q2 | subs/generic-subs 下载 base64 归属 | **Build5 实现下载 base64**：在 Build5 下载链路（internal/download）增加「product_type=subs/generic-subs 装配模板下载整体 base64」步骤；Build6 仅在其上叠加注入后重编码。理由：Build5 里程碑声明基础模式完整可用，SR/v2rayNG 客户端必须收到 base64。 |
| Q3 | home 高级字段与流量卡片开关落点 | **home 高级字段归 Build6，开关入 status**：Build6 Step5 补 `/api/home` traffic 高级形态（used_bytes/quota_bytes/exceeded）；`/api/system/status` 新增 `traffic_card_enabled` 布尔，首页与个人中心共用。 |

---

## 七、最终结论

1. **总体结论**：Design2.md 与 Design2-UI.md 设计闭环、自洽、可实施；Build4~7 严格按设计编写，Step 划分、执行约束、验收标准、文件清单、端点契约、数据模型与范围边界（各 Build 候选清单互指闭环）与 Design 对齐；**未发现 P0 级硬冲突**，Build 文档对代码基线的 10 项假设经勘察全部成立。
2. **决策缺口**：3 项 P1 问题（Q1 协议清单、Q2 base64 归属、Q3 数据源落点）已经用户确认，需按第六节结论回写文档后方可开始构建。
3. **次要项**：6 项 P2 观察项不阻塞构建，按 4.2 节建议在建前文档修订或 Build 执行期澄清。
4. **就绪度判定**：完成第八节「建议后续处理项」的文档修订后，Build4 具备启动条件。

---

## 八、建议后续处理项（✅ 已于 2026-08-19 执行完毕，落地明细见第九节）

| # | 动作 | 目标文档 | 内容 | 状态 |
|---|------|---------|------|------|
| 1 | 设计修订 | Design2.md §3.2/§4.5 | manual 协议表述改为 19 协议封闭清单（注明 gost-relay / hysteria2-realm / socks / sudoku 与 ssr 同因不纳入） | ✅ |
| 2 | Build 修订 | Build5.md | Step4 新增子项「subs/generic-subs 装配模板下载整体 base64」实现步骤与单测；Step7 动线 base64 验收保留 | ✅ |
| 3 | Build 修订 | Build6.md Step5 | 补 `/api/home` traffic 高级形态实现；`/api/system/status` 响应新增 `traffic_card_enabled`（Build4 Step4.1 同步注记由 Build6 补键） | ✅ |
| 4 | 顺带修订 | Design2.md §3.3 | 补强制组「无法归属的流量」成员构成 `[DIRECT, 国外流量]` 与三强制组渲染类型 select（P2-1） | ✅ |
| 5 | 顺带修订 | Build7.md Step1/Step2 + Design2 §5.9 | 对账期望集与实例节点取交集写明（P2-4）；xray_ext_users.node_id 待重绑统一 NULL、禁止插 0（P2-5，Design2 §5.9 措辞同步） | ✅ |
| 6 | Build 期澄清提前落盘 | Build5.md/Build6.md/Build4.md | Clash 手动规则行排除 USER-AGENT（P2-3）；组编辑红警为防御态注释（P2-2）；Build4 Step2 grep 限定 backend/frontend 目录（P2-6） | ✅ |

---

## 九、修订落地记录（2026-08-19，用户指令执行）

| 文件 | 位置 | 修订内容 |
|------|------|---------|
| Design2.md | §3.2 双来源表 / 协议范围行 | 「全部代理协议类型（…等）」→「19 项代理协议封闭清单」；补 gost-relay / hysteria2-realm / socks / sudoku 与 ssr 同因不纳入（引本报告 Q1） |
| Design2.md | §3.3 三类强制组 | 补三组渲染类型均为 select；「直接连接」成员固定 `[DIRECT]`；「无法归属的流量」成员固定 `[DIRECT, 国外流量]`（P2-1） |
| Design2.md | §4.5 ssr 条目 | 补四类同因不纳入说明，与 §3.2 封闭清单口径对齐 |
| Design2.md | §5.9 xray_ext_users 行 | node_id「0/NULL=待重绑」→「NULL=待重绑；禁止插入 0——启用外键时违反 FK」（P2-5） |
| Build4.md | Step2 验收命令 | grep 限定 `backend frontend` 目录，避免命中本 Build 与存档文档（P2-6） |
| Build4.md | Step4 项 1 | 注记 `traffic_card_enabled` 不在本 Build 暴露，由 Build6 Step5 补入（Q3）；变更记录 v1.2 |
| Build5.md | Step4 新增项 5 + 里程碑 + 文件清单 | 新增「subs/generic-subs 下载 base64 收口」：装配模板下载整体 base64、直接上传原字节、用户预览同口径、管理员预览明文；Build6 叠加不重复编码；单测补三条断言（Q2）；原单测项顺延为项 6；变更记录 v1.2 |
| Build5.md | Step6 `RulesStep.vue` | Clash 场景手动规则行类型下拉排除 USER-AGENT（P2-3） |
| Build6.md | Step2 项 4 | 注明「非候选集已分配」标注为防御性兜底展示，UI 红警保留不删（P2-2） |
| Build6.md | Step5 新增项 6 + 里程碑 + 文件清单 | home.go 高级 traffic 形态（used_bytes/quota_bytes/exceeded）与 status.go 暴露 `traffic_card_enabled`；单测补断言（Q3）；原单测项顺延为项 7；变更记录 v1.2 |
| Build7.md | Step1 对账口径 | 期望集再与本实例节点（nodes.instance_id=instanceID）取交集，to_push/credential_mismatches 仅含本实例目标（P2-4） |
| Build7.md | Step2 导入口径 | 重绑前 xray_ext_users.node_id 暂置 NULL，禁止插 0（P2-5）；变更记录 v1.2 |

> 以上修订均按第六节用户确认结论与 4.2 节 P2 建议执行，未改动任何代码；各 Build 文档修订后 Build4 具备启动条件。

### 修订后复核（2026-08-19，用户指令二次深度核验）

**复核结论**：上述 6 项修订全部正确落盘且与周边上下文一致（Q1 与 Build5 约束表 19 协议逐项清点吻合；Q2 与 Build6 Step4 职责分层清晰；Q3 与 Build7 参数表闭环；P2-1 与 Build5 Step3 渲染伪代码一致），无回归。复核另发现 5 处轻微同步缺口（R1~R5，均无设计决策成分），经用户确认已一次性补齐：

| # | 缺口 | 补齐落点 |
|---|------|---------|
| R1 | Design2-UI §9.3 契约未同步 Q3：`api/system.ts` 缺 `traffic_card_enabled`；`api/home.ts` traffic 缺 `unlimited`/`exceeded` | Design2-UI §9.3 两行补齐（v1.6） |
| R2 | Build6 Step4 sr-subs 分支未反向写明接管 Build5 base64 收口（仅 Build5 单边声明）；该 bullet 缩进脱格 | Build6 Step4 补接管声明并修复缩进（v1.3） |
| R3 | Build7 Step1 对账单测缺「to_push 不含其他实例目标」断言（实现口径已补、测试未同步） | Build7 Step1 单测项补断言（v1.3） |
| R4 | Build4 Step2 项 9 文字「全仓 grep」与新命令范围（backend/frontend）不一致 | Build4 Step2 项 9 措辞统一为代码目录（归入 P2-6 收口） |
| R5 | Build7 Step5.1 类型仅列高级模式 `unlimited:false`，缺配额不限 `unlimited=true` 分支 | Build7 Step5.1 补齐分支（v1.3） |

**复核终态结论**：Design2 / Design2-UI / Build4~7 全部修订闭环，无未关闭缺口；Build4 具备启动条件。

---

## 十、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 初始版本：Design2/Design2-UI/Build4~7 全量只读核验；3 项 P1 决策经用户确认，6 项 P2 观察项列入后续处理 |
| v1.1 | 2026-08-19 | 按第八节清单执行全部 6 项文档修订并落地（新增第九节修订记录）：Design2 §3.2/§3.3/§4.5/§5.9、Build4 v1.2、Build5 v1.2、Build6 v1.2、Build7 v1.2 |
| v1.2 | 2026-08-19 | 修订后二次深度核验：确认 6 项修订正确落盘无回归；新发现 R1~R5 五处轻微同步缺口经用户确认一次性补齐（Design2-UI v1.6、Build4/Build7 v1.3 措辞与单测、Build6 v1.3 接管声明）；复核终态无未关闭缺口 |
