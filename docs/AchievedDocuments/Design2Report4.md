# Design2Report4.md — Design2.md 正式审阅报告（第四轮独立审核）

> **归档说明：** 本报告全部 A/B 级发现已由后续两轮设计审查与 2026-08-16 多轮修订闭环落文 [Design2.md](../../Design2.md)（见其第六章变更记录），本报告仅作历史核查、不再作为当前问题依据；**以 Design2.md 当前版本为准**（审核 R12 决策，2026-08-16）。

> **报告定位：** 本文档是对 [Design2.md](../../Design2.md)（增量能力：规则素材池 / 装配拼接 / 配置生成与分发 / Xray 对接）的**第四轮正式审阅报告**，承接 Design2Report1~3 存档成果，但不以其结论为默认成立，全部发现均在本轮重新取证。
> **审核对象：** Design2.md 当前版本（410 行，2026-08-16 状态）× Design1.md / Design1-UI.md 基线 × `backend/`、`frontend/` 现有实现 × `docs/Reference/` 研究资料 × `docs/DocTemplates/` 样例。
> **审核约束：** 除本报告文件外，未修改任何代码、配置或文档；结论标注来源（【文档事实】/【代码事实】/【实验】/【推断】）。
> **审核时间：** 2026-08-16

---

## 一、执行摘要

**总体结论：设计方向与技术选型成立，不需要推翻；但 Design2.md 首页所述「全部设计已定稿、无待决事项」在本轮证据下不成立。** 当前存在 **7 项 A 级事项**（建议进入 BuildN 前由用户逐项决策并落文）、**12 项 B 级事项**（BuildN 编写时必须明确口径）、**8 项 C 级备忘**（低风险）。

与前几轮报告的关系：Report1~3 已识别的部分问题仍留在 Design2 现行文本中（本轮独立复核确认），另有若干**前几轮未覆盖的新发现**，其中最有分量的三类是：

1. **Clash 模板的「动态重建 proxy-groups」缺少可落地的数据表示**——模板是纯文本 + 单个节点占位，下载渲染器无法仅凭文本安全重建代理组与规则降级（A2）。
2. **公共节点（is_public）只有「渲染可见」语义，没有「向 Xray 推送账号」语义**——公共节点会被注入订阅，但从未被 AddUser，用户拿到链接也无法连接（A6）。
3. **1009 迁移重建 subscriptions 表的连锁反应比“数据可放弃”复杂**——SQLite 实验证实：DROP TABLE 会级联删除引用旧行的 download_tokens，但 `versions` 无外键成为孤儿，磁盘文件也无人清理（A3）。

本轮同时确认：Design2 的 24 项决策、生命周期触发器、加密复用、下载禁缓存、Xray API 取证等核心论断与代码/参考资料**高度吻合**，可用作 BuildN 的可靠基础。

---

## 二、审核轮次（六轮递进，不预设结论）

| 轮次 | 维度 | 主要工作 | 产出 |
|------|------|---------|------|
| 1 | 文档内部一致性 | Design2 全文编号精读；按关键词复核 Report1~3 疑似未闭环项 | 确认多条旧发现仍开放 |
| 2 | 代码事实核验 | 精读 version / subscription / group / token / download / user / approval / config / export / cron / dataclear / server / 前端 views 与 api | 建立代码扩展点映射 |
| 3 | 行为实验 | Python SQLite 实验验证 DROP TABLE 的级联与孤儿行为（两轮） | 迁移连锁事实 |
| 4 | 外部资料一致性 | 交叉核对 Reference/Xray-Core-API、Node-Link-Standards、SSpanel-Subscribe | 发现 Reference 与决策 #20 的过时表述 |
| 5 | 场景模拟 | 基础模式全链、高级模式全链、空节点/无凭据、公共节点、OFF 清空、导入迁移、规则 conf 流程等 10 条路径 | 提炼 A 级发现 |
| 6 | 工程与安全 | AGENTS 约束逐条对照、前端超时/依赖、路由与构建面、迁移框架能力 | B/C 级清单 |

---

## 三、已核验成立、可作为 BuildN 依据的结论（避免重复争论）

| # | 结论 | 证据 |
|---|------|------|
| 1 | `version.ContentProvider` 已预留装配生成扩展点；`CreateVersion` 当前强制激活（L164），加 `activate` 参数改造点清晰 | 【代码事实】version.go L72-75/L123-176 |
| 2 | `CreateVersion` 全部 5 个调用点明确（server/subscription.go L221、rule.go L95、subscription.go L222、custom.go L93、share.go L76） | 【代码事实】grep |
| 3 | 下载三态解析、组选定 JOIN、平台附加头、禁缓存、200 注释块全部存在，改造落点明确 | 【代码事实】download.go L55-135、server/download.go L38-127 |
| 4 | 生命周期触发器与现有函数一一对应；「事务提交后钩子」已有 `sendWelcomeIf` 同构先例 | 【代码事实】user.go L84、user/oidc.go L70、approval.go L93/125、admin.go 相关方法 |
| 5 | 敏感字段加密可直接复用 `config.Encrypt/Decrypt`（HKDF-SHA256 → AES-256-GCM） | 【代码事实】config.go L222-265 |
| 6 | cron ticker 模式可复用；迁移编号 1009 起正确 | 【代码事实】cron/cleanup.go、migrations/1008 |
| 7 | `dataclear.ClearTablesTx` 为硬编码 16 表清单，新增表必须显式纳入 | 【代码事实】dataclear.go L39-52 |
| 8 | 配置导出当前 `format_version=1`，payload 仅 system_config + 站点信息，决策 #24 的改造点准确 | 【代码事实】export.go L29/L37-45 |
| 9 | Xray API 关键结论（email 为键、错误子串匹配、QueryStats 子串匹配/空 pattern 风险、counter 易失）与 Reference 一致 | 【文档事实】Reference/Xray-Core-API.md §一/§三/§11 |
| 10 | SR subs 整体 base64、vless base64 userinfo 形态与样例/生态取证一致 | 【文档事实】Node-Link-Standards.md 四/六章 |
| 11 | 前端 `AssemblyView.vue` 占位页、AdminLayout 菜单预留、路由懒加载均存在 | 【代码事实】router/index.ts、AdminLayout.vue |
| 12 | 前端 Axios 全局超时 15s，对素材池同步等长任务构成真实约束 | 【代码事实】request.ts L7 |
| 13 | `jsdiff` 依赖尚未引入 package.json | 【代码事实】package.json |

---

## 四、A 级发现（建议进入 BuildN 前由用户决策）

### A1 产物类型、下载路由与「首次入池自动激活」互相矛盾

- **证据**：Design2 L165 规定「平台 + 产物类型（yaml/subs）」唯一校验；L172 复用现有 `GET /subscriptions/{平台标识}/download`，该路由只有 platform 维度；L168「首次入池自动激活按平台判定」与「平台 + 产物类型」模型不匹配——同平台若先入池 subs，则 yaml 首版本将因“平台已有激活版本”永不自动激活。当前代码 `download.go L40` 只按 platform 解析，没有产物类型维度。【文档事实 + 代码事实】
- **影响**：同一平台存在 yaml/subs 两模板时下载路由无法确定返回哪个；首次分发空窗判定错误。
- **分析假设**：现有下载 URL 无产物维度，且用户端 Token 复用键只有 user+platform，不可能静默支持双模板。【推断】
- **建议方案（三选一，推荐 A）**：
  - **A（推荐）**：`subscriptions` 增加 `product_type`（`yaml`/`subs`）作为展示/校验属性，但**每平台仍只有一份订阅模板**（`UNIQUE(platform_id)`）；下载按 platform 定位唯一模板；「首次自动激活」判定改为**该 platform 对应订阅行 `current_version=0` 时**。改动最小、与下载/Token 现有模型完全兼容。
  - B：保持「平台 + 产物类型」双模板，但下载 URL 与 Token 解析增加产物类型维度（`/subscriptions/{platform}/{product}/download` 或 Token 表加列）。改动面大。
  - C：给 platforms 表增加 `product_type` 属性，subscriptions 唯一键仍为 platform；与 A 等价但把属性放在平台侧。
- **待用户决策**：选择哪种模型；若选 A，需确认 `product_type` 列是否仍保留。

### A2 Clash 模板「动态重建 proxy-groups」缺少可落地的数据表示，且空节点/无凭据/OFF 场景会产生无效 YAML

- **证据**：Design2 L321 要求下载时「按用户实际注入节点动态重建 proxy-groups」，但 L160 只定义了 `# {{xray_nodes}}` 一个文本占位，装配产物是纯文本模板；L322 规定无凭据用户仅把占位替换为注释、模板其余部分原样；L24 OFF 后同样只替换为注释。纯文本 + 单个节点占位无法让渲染器可靠地知道 proxy-groups 的成员/子组结构，也无法安全重写 rules 中引用被删组的行。【文档事实 + 推断】
- **影响**：
  - 无凭据用户 / 空注入用户 / OFF 后用户下载的 Clash YAML 中，强制组（如「国外流量」）可能没有任何 proxies，Clash/mihomo 加载失败；
  - 「保证模板可独立预览/校验」与「动态重建」并存但缺少结构化数据来源，BuildN 将无从实现。
- **分析假设**：装配器有能力在生成时保留结构化蓝图（`assembly_blueprints` 已设计），下载渲染时使用蓝图重新生成 Clash 文件比解析/修补 YAML 文本更可靠、更简单。【推断】
- **建议方案（推荐 A）**：
  - **A（推荐）**：装配生成的 Clash YAML 版本同时保存**结构化渲染计划**于 blueprint（头部、manual proxies、proxy-groups、rules、兜底规则）；用户下载时按蓝图全量重渲染（而非仅文本替换）。重渲染统一规则：
    1. 注入节点 = 组分配节点 ∪ 公共节点（过滤 enabled=0 与候选集）；
    2. 所有 proxy-groups 按“可达注入节点”递归重建；
    3. **强制组在注入集为空时降级为 `proxies: [DIRECT]`**；
    4. 引用被剔除组的 rules 行降级为 DIRECT；
    5. 无凭据/高级未启用时占位替换为注释，且同样执行 2~4。
  - B：保持纯文本替换，另加 `# {{proxy_groups}}` / `# {{rules}}` 两个占位块，由渲染器从 blueprint 生成块文本。可行但规则更脆弱。
  - C：维持现状，仅接受“无凭据用户 YAML 可能加载失败”的已知损失。不推荐（AGENTS 安全底线与 3.3 空组硬校验自相矛盾）。
- **待用户决策**：确认 A；若选 A，还需确认「直接上传模板」无 blueprint 时仅做占位文本替换、不重建（模板作者自负其责）这一边界。

### A3 迁移 1009 重建 subscriptions 表的连锁清理未定义（有孤儿数据/文件风险）

- **证据**：
  - 【实验】SQLite `PRAGMA foreign_keys=ON` 下 `DROP TABLE subscriptions`：引用旧订阅行的 `download_tokens` 被级联删除；`subscription_id IS NULL` 的 Token 保留；`versions` 无外键，旧 `owner_type='subscription'` 行成为孤儿。
  - 【代码事实】迁移框架只执行纯 SQL（store.go `applyOne`），没有 Go hook 清理磁盘文件；版本文件位于 `{dataDir}/contents/subscription/{id}/*`。
  - 【文档事实】Design2 L25/L366 只说“存量数据可放弃、清空重建”，未定义删除哪些表、如何清文件。
- **影响**：直接按 L366 重建会留下孤儿 versions 行与磁盘文件（违反 AGENTS §4.7）；若用 DROP TABLE 隐式级联，又容易误删/漏删 Token。
- **建议方案（推荐 A）**：
  - **A（推荐）**：1009 迁移 SQL 显式按序执行：
    1. `DELETE FROM download_tokens WHERE subscription_id IS NOT NULL`（显式订阅 Token 不再新发，存量一并清）；
    2. `DELETE FROM versions WHERE owner_type='subscription'`；
    3. 删除 `group_selections`、`subscription_group_rel`；
    4. DROP + 重建 `subscriptions`（增加 `product_type` 列与唯一约束）；
    5. 保留 `subscription_id IS NULL` 的无标识/自定义 Token（实验证明不被误删）。
  - 同时给迁移框架增加**一次性迁移后钩子**（或 main 启动时按标记）删除 `{dataDir}/contents/subscription/` 整个目录；删除失败记日志但不阻断启动。
  - B：不改迁移框架，在 `version.StartupCheck` 增加一次性目录清理（按 DB 中不存在 owner 行判定）。可行但语义耦合较重。
  - C：升级前要求管理员先「一键清空数据」。最简但零配置升级体验差。
- **待用户决策**：A/B/C；若选 A，确认“显式订阅 Token 全清、无标识 Token 保留”的语义。

### A4 配置导入 × 业务密文：签名密钥替换后 users/nodes 密文不可解

- **证据**：Design2 L250/L359 规定 users.uuid_encrypted、users.proxy_secret_encrypted、nodes.protocol_json 凭据字段用签名密钥派生密钥加密；`export.go L137-173` 导入时**只重写 system_config，不处理任何业务表**，且导出 payload 含新 signing_key；Design1 3.4.8 的导入语义是“整体覆盖配置、不变更业务数据”。二者叠加：向**已有业务数据的实例**导入不同密钥的配置后，所有业务密文不可解。【代码事实 + 文档事实】
- **影响**：Xray 推送失败、下载注入失败、manual 节点渲染失败；且没有安全恢复手段。
- **建议方案（三选一，推荐 A）**：
  - **A（推荐，最小可行）**：导入已配置系统时，若检测到 `signing_key` 将发生变化且存在任一业务密文（users 两列或 nodes 凭据字段），**拒绝导入**并提示“配置导入仅适用于全新部署/同密钥往返；在用实例请使用备份恢复”。安全、简单、语义清晰。
  - B：导入事务内用旧密钥解密全部业务密文 → 写入新 system_config → 用新密钥重加密业务密文。能力最强，但配置导入开始写业务表，复杂度与测试面明显增大。
  - C：允许导入，但清空全部业务密文并置“需重新生成/录入”标记。破坏性大，不推荐。
- **待用户决策**：A/B/C。

### A5 高级开关 OFF 只清面板数据，不清理可达 Xray 实例上的账号

- **证据**：Design2 L22 的 OFF 清空清单全部是面板数据；全文仅在用户禁用/删除、节点/实例删除等触发器提到 RemoveUser，未规定 OFF 时对 Xray 侧 RemoveUser。决策 #5 又明确“Xray 侧存量账号不做 reconcile”。【文档事实】
- **影响**：OFF 后 Xray 上旧账号仍可连接（安全边界问题）；重新 ON 生成新 UUID 后，同实例可能出现新旧账号并存，配额与生命周期从此失控。
- **建议方案（推荐 A）**：
  - **A（推荐）**：OFF 事务提交前收集 `xray_users` 全部（user_id, instance_id, inbound_tag），提交并清库后**逐实例 best-effort RemoveUser**；实例不可达则跳过并记 `warn`，确认弹窗与部署文档明确“不可达实例需手动清理”。与 L325 实例删除的处理口径一致。
  - B：不清 Xray 侧，仅文档警告。不符合安全底线，不建议。
- **待用户决策**：A/B。

### A6 公共节点（is_public）只有注入语义，没有 AddUser 语义，注入的链接无法连接

- **证据**：Design2 L255（决策 #21）与 L299 规定 is_public=1 的节点“对所有组自动可见、下载渲染时注入”；但 L266-276 的全部 AddUser 触发动作都只说“向所属组分配的全部节点”推送，组分配又仅限 `group_nodes`（公共节点无需分配）。于是公共节点会出现在用户订阅中，但 Xray 侧从未 AddUser 该用户。【文档事实 + 推断】
- **影响**：所有启用 is_public 的 xray 节点对用户不可用；公共节点功能名存实亡。
- **建议方案（推荐 A）**：
  - **A（推荐）**：明确 **is_public=1 的 xray 节点自动进入每个 active 用户的 AddUser 集合**（仍受装配候选集与 enabled 过滤），并在 ON 批量初始化、注册/审批/启用、组节点 diff、节点删除/实例删除等触发器与组分配节点同等参与 diff 推送/移除。
  - B：公共节点只用于 manual 静态节点展示，xray 节点不允许 is_public。语义更窄，但限制了决策 #21 的用途。
- **待用户决策**：A/B。

### A7 候选集是“全局一组”还是“每模板一组”，以及候选清理时是否同步 RemoveUser，未定义

- **证据**：Design2 L90 规定“装配器勾选的 Xray 节点构成全局候选集”；L298 组分配只能在候选集内；L320 下载又按“当前激活模板的装配候选集”逐模板过滤。系统存在 Clash YAML 与 SR subs 两个可同时激活的模板，二者候选集可能不同；L298 的清理只删 `group_nodes`，未说对受影响用户 RemoveUser。【文档事实】
- **影响**：
  - 组管理页无法判断“分配了但某些模板不注入”的标注口径；
  - 激活新版本导致节点退出候选集时，`group_nodes` 被删，但用户此前已推送到 Xray 的账号仍在，授权实际未收回。
- **建议方案（推荐 A）**：
  - **A（推荐）**：
    1. 组管理页候选集 = **当前所有已激活装配蓝图的 xray 候选节点并集**；
    2. 下载渲染仍按各模板自身蓝图过滤；
    3. 新版本激活时，比较新旧蓝图的候选集差集，删除 `group_nodes` 后**对受影响 active 用户执行 RemoveUser diff**（事务提交后，幂等失败记同步状态）；
    4. 组管理页对“仅部分模板候选”的节点做提示。
  - B：候选集简化为全局唯一（只允许一个 Clash 模板和一个 SR subs 模板共享同一候选集）。交互简单，但限制灵活性。
- **待用户决策**：A/B。

---

## 五、B 级发现（BuildN 编写时必须明确口径）

| ID | 问题 | 证据/影响 | 建议口径 |
|----|------|-----------|---------|
| B1 | 配额 `0/NULL` 语义、流量单位与月界时区未定义 | Design2 L246/L300/L338；Subscription-Userinfo 为字节，UI 为 GB | 建议：`default_quota`/`quota_override` 为 **NULL 或 0 = 不限流量**（跳过检查，`total` 留空或省略）；`traffic_records.uplink/downlink` 存**字节整数**；`ym` 按 **UTC** 计算并显式落文 |
| B2 | UUID/代理密码并发首建守卫未定义 | ON 批量初始化与注册/审批钩子可能并发命中同一用户 | 复用 Token 首建模式：`BEGIN IMMEDIATE` 事务内条件更新 `... WHERE id=? AND uuid_encrypted IS NULL`，按 RowsAffected 判定；生成与加密在事务内完成 |
| B3 | 高级模式开关的配置键、403 端点清单、基础模式下钩子行为未落文 | L20/L21 只说入口隐藏、高级接口 403 | 建议：`advanced_mode` 配置键；`/api/admin/xray*` 与组节点分配类端点统一中间件 403；所有 Xray 同步钩子入口先检查开关，OFF 时静默跳过；`/api/system/status` 暴露 advanced_mode 供前端隐藏入口 |
| B4 | `xray_users` 主键/外键、实例与节点命名唯一性、inbound 缺失标记未定义 | L364 只有字段；C-新3/C-新13/C-新18 同源 | 建议：`xray_users` 复合 PK/UNIQUE(user_id, instance_id, inbound_tag)，并增加 `node_id` FK 或对 nodes(instance_id,tag) 复合外键实现节点删除级联；`xray_instances.name` 唯一且建议增加 `slug`；`nodes.name` 全局唯一；节点增加 `last_seen_at`/`missing` 标记用于“Xray 侧已删 inbound”提示 |
| B5 | 动态响应头与平台附加头同名冲突、`profile-update-interval` 数值、无激活版本的 HTTP 语义未定义 | L257/L329；当前 clash-verge 预置 `profile-update-interval:300`；当前代码 `ErrVersionNotFound` 返回 404（download.go L47-50 等），Design2 L169 要求 200 注释块 | 建议：高级模式系统头**覆盖**平台附加头（或保存平台附加头时禁止同名键）；高级模式 `profile-update-interval` 定 21600（6h），基础模式沿用平台值；用户订阅路径 `ErrVersionNotFound` 改 200 `# error: no active version`，并同步更新 `download_test.go` 既有 404 断言 |
| B6 | SR subs 模板存储形态与默认文件名未定义 | L318 说“SR 平台模板为 subs（base64）”且占位在解码后明文中；当前 `defaultFileName` 对订阅类统一 `.yaml` | 建议：模板文件**存储解码后的明文**（下载注入后再整体 base64），避免每次解码歧义；`sr-subs` 默认文件名为 `.txt`（`clash-yaml` 为 `.yaml`、`sr-conf` 为 `.conf`），上传模板保留原始扩展名 |
| B7 | 素材池同步的补偿、空响应、行内策略字段、值规范化未定义 | L64/L68；每日 ticker 停机错过时刻无补偿；`IP-CIDR,x,no-resolve` 或 `DOMAIN,x,PROXY` 类行如何解析未定 | 建议：ticker 每次检查 `auto_sync=1 AND (last_synced_at IS NULL OR last_synced_at < 当日 sync_time)`；空响应按该 URL 失败处理（不差删）；标准规则行只取 `规则类型,匹配值` 前两段，多余策略段忽略或按非法行记录；域名 lowercase 规范化、CIDR 做格式校验不强制重写 |
| B8 | `dataclear.ClearTablesTx` 未纳入 9 类新表 | 代码硬编码 16 表清单 | BuildN 必须同步扩展 `rule_pools/pool_entries/xray_instances/nodes/proxy_groups/group_nodes/xray_users/traffic_records/assembly_blueprints`，并补测试 |
| B9 | `CreateVersion` 的 activate 参数、首次自动激活的事务边界与调用点改造未定义 | 5 个调用点；“入池不激活”与“首次自动激活”需在订阅/规则服务内判定 | 建议：`CreateVersion(ctx, ot, ownerID, src, activate bool)`；服务层包装 `CreateAndMaybeActivate` 在**同一 `BEGIN IMMEDIATE` 事务**内判定 `current_version=0`；规则/分享/自定义调用传 true，订阅池与装配调用传 false |
| B10 | 前端/API 工程细节未定义 | request.ts 全局 15s；jsdiff 未安装；装配目标平台选择、规则卡片与 SR 平台映射、新路由/菜单均未规格化 | 建议：同步类长任务（素材池同步/连通性测试/装配生成）按请求覆盖 timeout（如 70s）或后端异步化；新增 `diff`（jsdiff）；BuildN 明确目标平台选择 UI、`api/xray.ts`、AdminLayout 菜单按 advanced_mode 显隐、规则卡片沿用现有 RulesView 列表并修正“一键导入”措辞 |
| B11 | Go 1.26 / xray-core 依赖 / Dockerfile / CI 构建动作未细化 | 当前 go.mod 1.25.0、Dockerfile golang:1.25-alpine；AGENTS 仍写 1.25 | 作为 BuildN Step 0：先升 Go 1.26 + Dockerfile + go mod tidy 并验证三端构建；AGENTS/README 同步版本描述 |
| B12 | 参考文档与 UI/基线文档的同步任务未定义 | Reference/Xray-Core-API.md §四仍写 trojan/ss “本项目不使用”，与决策 #20 冲突；Design1-UI 增量页面待补；Design1 4.1/4.5 旧语义待构建后回填 | BuildN 尾部设置文档同步 Step；冲突按用户决策后统一订正 |

---

## 六、C 级备忘（低风险，随对应 Step 处理）

| ID | 事项 | 说明 |
|----|------|------|
| C1 | `subscription.Update` 现有 TODO（未校验取消正被选定的订阅） | 新模型删除 group_selections 后自然消除，BuildN 不必单独修复 |
| C2 | 迁移文件 1002/1006 注释仍有「手填」旧措辞 | 与已确认的自动生成口径不符，建议随 1009 同批订正（只改注释） |
| C3 | 素材池 URL 可能含查询凭据 | 日志只记 pool_id/状态，不落完整 URL；URL 列表接口回显需注意脱敏 |
| C4 | manual 节点凭据字段的编辑回显语义 | 建议“编辑时凭据留空=保持原值，填写=覆盖”，避免解密明文回显 |
| C5 | `xray_instances.api_tag` 随配置导出 | 该字段非认证凭据，可随导出；但导入后需提醒刷新节点检测 |
| C6 | 规则 Token 一键导入措辞 | Design2 L173 与 Design1 3.5/当前 RulesView 冲突，应按“复制链接”订正 |
| C7 | 访问日志类型与 fail_reason 扩展 | 新模型建议新增 `no_active_version`、`advanced_disabled` 等口径，并保持 `text/plain + 200 注释块` |
| C8 | `profile-update-interval` 基础模式 300 vs 高级 21600 | 与 B5 同源；BuildN 明确两模式取值后补充测试 |

---

## 七、待用户决策清单（建议逐项确认）

| # | 决策点 | 推荐方案 | 建议理由 |
|---|--------|---------|---------|
| D1 | 产物类型/下载路由/首次激活模型（A1） | 每平台一份模板 + `product_type` 元数据，下载按 platform；首次激活按订阅行 `current_version=0` | 与现有 URL/Token 模型兼容，改动最小 |
| D2 | Clash 渲染重建的数据表示（A2） | 装配蓝图存结构化渲染计划，用户下载全量重渲染；空注入强制组降级 DIRECT；直接上传模板仅文本替换 | 消除空组/无凭据/OFF 三类无效 YAML |
| D3 | 1009 迁移清理方式（A3） | 显式 SQL 清旧数据 + 迁移后钩子删 `contents/subscription/`；保留无标识/自定义 Token | 满足 AGENTS 无孤儿 |
| D4 | 配置导入遇业务密文（A4） | 密钥变化且存在业务密文时拒绝导入 | 最安全、实现最简 |
| D5 | OFF 清空是否清理 Xray 侧（A5） | 收集后 best-effort RemoveUser，不可达跳过并提示 | 防 OFF 后账号失控 |
| D6 | 公共节点推送语义（A6） | 公共 xray 节点自动 AddUser 至全部 active 用户 | 否则公共节点不可用 |
| D7 | 候选集与激活清理语义（A7） | 组管理页取并集、下载按模板过滤；激活时差集删分配 + RemoveUser diff | 防止授权残留 |
| D8 | 规则实体首版本（C1 历史项） | 二选一：允许空规则实体（推荐）或删除“首个 conf 自动激活”条款 | 若保留自动激活条款，必须允许空规则实体 |
| D9 | 配额 0/NULL 语义（B1） | 0/NULL=不限流量 | 简单且符合“基础模式不限流量”心智 |
| D10 | 无激活版本 HTTP 语义（B5） | 用户/规则下载返回 200 `# error: no active version`，同步改测试 | 符合 AGENTS 4.8 |
| D11 | 高级模式头与平台附加头冲突（B5） | 系统头覆盖平台附加头；高级 21600/基础沿用 300 | 避免重复响应头歧义 |
| D12 | 素材池定时同步补偿（B7） | 按“当日已过 sync_time 且未同步过”判定补跑；UTC 口径 | 减少停机漏同步 |

> 未列入上表的 B/C 级项按本报告第五、六节建议口径在 BuildN 中落文即可；若你希望其中任何一项升格为 D 决策，请指出编号。

---

## 八、结论

- Design2 的**方向、24 项决策、代码扩展点与外部研究基础经六轮复核总体成立**，不是推倒重来，而是补 7 项 A 级决策、明确 12 项 B 级口径后即可进入 BuildN。
- 本轮最重要的结论是：**「无待决事项」表述需要修正**；A1/A2/A3/A6 属于不改就会在实现期返工的结构性问题。
- 本报告未修改 Design2.md；待你对 D1~D12（至少 A 级对应项）作出选择后，再按你的指示把结论落文到 Design2.md 或直接由 BuildN 承接。

---

## 九、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-16 | 第四轮正式审阅完成：六轮取证，产出 A 级 7 项 / B 级 12 项 / C 级 8 项与 D1~D12 决策清单；除本报告外未修改任何文件 |
