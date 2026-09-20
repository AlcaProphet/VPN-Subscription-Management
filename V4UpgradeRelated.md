# V4UpgradeRelated.md — 首版数据库基线合并实施定稿与历史兼容清理候选

> **文档定位：** 本文档保存 V4／首个正式版本发布前的数据库基线合并实施定稿，并继续保留数据库外历史兼容清理的研究候选。数据库基线合并方案已经完成当前仓库复核；数据库外兼容清理仍不是本次实施范围。
> **状态边界：** 用户已于 2026-09-20 授权并完成数据库基线合并实施，构建与验收记录见 [Build31.md](docs/reports/Build/Build31.md)。本文档不是 `TODOLIST.md`；第五节的数据库外历史兼容清理仍只是研究候选，不因本次合并自动成为活跃工作。
> **实施边界：** 代码、SQL、测试和必要文档同步已经完成；既有本地数据库、外部 `DATA_DIR`、项目 Docker 命名卷和备份均未删除或修改。后续如需切换这些持久化目标，仍须分别核对目标并取得数据清理授权。
> **关联约束：** 实际处理仍须遵循 [AGENTS.md](AGENTS.md)。数据库基线合并不得混入 [SecurityScanPlan1.md](SecurityScanPlan1.md) 的步骤、证据或状态，也不得自行改写已归档的 Design／Build／Issue 历史记录。

---

## 一、背景与结论

当前项目计划以现有状态作为首个正式版本，并已确认：

1. 不存在需要保留的真实用户、真实业务数据或正式发布数据库。
2. 当前本地数据库只承载可丢弃的开发和测试数据。
3. 首版发布前可以不承担开发期旧数据库、旧 API、旧蓝图或旧导入格式的兼容责任；但外部协议、URI、客户端和规则文件格式不能仅因名称带有 `legacy`／`compatibility` 就一并删除。

研究结论：

- 可以清除当前 `backend/migrations/` 中 `0001`～`1022` 的开发期演进历史，并重建为一个首版最终数据库基线。
- 不能在没有替代结构定义的情况下把迁移目录永久清空。全新的 SQLite 文件没有任何业务表，项目仍需要一套 SQL 定义创建 `users`、`groups`、`nodes`、`subscriptions` 等当前表结构。
- 推荐保留迁移目录、`schema_migrations` 和迁移执行器，以一个新的 `0001_initial_schema.sql` 直接创建首版最终结构；首版发布后的结构变化从 `0002` 开始。
- 数据库基线压缩只是第一层。代码中还存在启动期数据升级、旧 API 字段、旧导入格式、旧蓝图／快照和旧响应容错，需要逐项区分“项目历史兼容”与“当前外部格式或安全可靠性能力”。

目标状态：

```text
backend/migrations/
├── 0001_initial_schema.sql
└── embed.go
```

首版首次启动：

```text
空 app-prod.db
  ↓
执行 0001_initial_schema.sql
  ↓
写入 schema_migrations(version = 1)
  ↓
进入 Setup
```

未来第二版：

```text
backend/migrations/
├── 0001_initial_schema.sql
├── 0002_<future_change>.sql
└── embed.go
```

## 二、为什么不能只清空 migrations

### 2.1 空数据库不包含业务表

SQLite 新建的数据库文件最初是一张白纸。若没有执行建表 SQL，它不会自动拥有：

- `system_config`
- `users`
- `groups`
- `platforms`
- `subscriptions`
- `versions`
- `nodes`
- `rule_pools`
- `oidc_states`
- 其他业务表、索引、外键和唯一约束

程序随后执行 `SELECT`、`INSERT` 或 `UPDATE` 时会收到 `no such table`。因此，“重新开始”应理解为删除旧数据库的演进历史并直接建立最终首版结构，而不是删除所有数据库结构定义。

### 2.2 当前迁移文件同时承担初始化和升级

迁移文件包含两类内容：

1. **当前仍必需的初始化定义：** `CREATE TABLE`、`CREATE INDEX`、外键、CHECK、唯一约束和首版种子数据。
2. **只为旧数据库服务的升级步骤：** `ALTER TABLE`、旧数据回填、旧列转换、旧表重建、旧 ID 保留、旧 state／ticket 失效处理。

首版需要第一类，不需要第二类。正确做法是把第一类提炼到新的 `0001_initial_schema.sql`，而不是把两类一起删除。

### 2.3 迁移执行器仍有长期价值

现有迁移执行器提供：

- 按版本顺序执行未应用迁移；
- SQL 与版本记录在同一事务提交；
- 失败时拒绝启动，避免半迁移状态；
- 重复启动幂等；
- 数据库版本高于当前程序支持版本时拒绝降级运行；
- SQL 通过 `go:embed` 进入单二进制，不依赖运行时外部文件。

这些能力并不是历史负担，而是首版发布后维护数据库所需要的基础设施。若现在删除，第二个正式版本需要修改 schema 时仍要重新实现同类机制。

### 2.4 不推荐的替代方式

以下方式技术上可行，但不推荐：

- 把全部 `CREATE TABLE` 写成 Go 字符串：结构难审查，未来仍需版本迁移。
- 随镜像分发预制 SQLite 文件：二进制不可读，容易混入测试数据、序列、配置或密钥，Git 也无法清晰审查结构变化。
- 使用 ORM 自动建表：当前项目没有采用重型 ORM，且自动同步难以准确表达复杂索引、约束和可审查的版本边界。

纯文本首版基线迁移更符合当前单二进制、SQLite、轻量和可审查的工程方向。

## 三、当前迁移链复核基线（2026-09-20）

### 3.1 前置检查结果

本次实施方案定稿前已经完成以下只读检查：

- 已重新阅读当前 `AGENTS.md`、本文档、当前工作入口和 Git 状态。
- 当前无已授权活跃 Build；只读前置检查开始和结束时工作区均无未提交变更，本轮写入后预期只出现本文档的修改。
- 用户已再次确认项目仍处于个人快速开发阶段，不存在真实用户和历史兼容包袱。
- `backend/migrations/` 当前包含 27 个 SQL 文件，共 611 行，版本范围为 `0001`～`0005`、`1001`～`1022`。
- 相比 2026-09-16 快照，新增 `1022_mail_result_logs.sql`；其 `mail_result_logs` 表和时间索引必须并入首版基线。
- 在临时空 SQLite 数据库顺序执行当前完整迁移链，最终得到 34 张表（包含 `schema_migrations`）和 25 个显式命名索引。
- 临时库 `PRAGMA foreign_key_check` 无结果，`PRAGMA integrity_check` 返回 `ok`。
- 迁移层唯一产品种子为 9 个预设代理组；`groups`、`platforms`、`system_config` 初始均为空。
- 默认组和 3 个默认平台由 `backend/internal/setup/setup.go` 的 Setup 事务创建，不属于迁移种子。
- 生产 Go 和 `frontend/src` 中已经没有 `group_selections`、`subscription_group_rel`、`pool_entries`、`urls_json`、旧单值 `installer_file`／`installer_url` 的引用。
- `go test ./internal/store ./internal/setup ./internal/mail ./internal/cron` 通过。

本轮检查只创建了系统临时目录中的一次性 SQLite 校对库，没有修改仓库、项目数据库或外部系统。

### 3.2 当前最终对象清单

最终 34 张表为：

```text
access_logs
assembly_blueprints
custom_subscriptions
download_tokens
group_nodes
groups
mail_result_logs
nodes
oidc_login_tickets
oidc_states
password_reset_tokens
platforms
pool_canonical_rules
pool_rule_origins
pool_source_snapshots
pool_sync_tasks
proxy_groups
rule_pool_sources
rule_pools
rule_tokens
rules
schema_migrations
share_subscriptions
share_tokens
subscriptions
system_config
traffic_records
users
versions
xray_ext_accounts
xray_ext_traffic
xray_ext_users
xray_instances
xray_users
```

最终 25 个显式命名索引为：

```text
idx_access_logs_created
idx_custom_platform
idx_custom_user
idx_dt_user_platform
idx_group_nodes_node
idx_mail_result_logs_recorded_at
idx_nodes_instance
idx_nodes_render_name
idx_oidc_login_tickets_exp
idx_oidc_states_created
idx_pool_canonical_pool
idx_pool_origins_rule
idx_pool_origins_source
idx_pool_snapshots_source
idx_pool_sources_manual
idx_pool_sources_pool
idx_pool_sources_url
idx_pool_sync_tasks_pool
idx_reset_tokens_user
idx_rules_home_default
idx_subscriptions_platform_uniq
idx_users_group_id
idx_versions_owner
idx_xray_ext_users_node
idx_xray_users_node
```

SQLite 为主键和表内 `UNIQUE` 约束自动创建的内部索引不计入上述 25 个显式索引，但必须通过列、唯一约束和行为测试保留其语义。

### 3.3 已确认的开发期历史

已经确认的开发期历史示例：

1. `1001_platforms.sql` 只有说明，没有实际 DDL。
2. `1008_platform_installers_multi.sql` 新增数组列、把旧单值数据转成数组，再删除旧列；首版可直接创建数组列。
3. `1009_xray.sql` 在既有表上追加大量字段，创建早期素材池结构，并删除更早的分发模型；首版可直接建立最终表。
4. `1016_rule_pool_snapshots.sql` 删除早期素材池表，建立 Canonical Rule／Snapshot 模型，并保存旧最大 ID 防止复用；没有旧数据时不需要 ID 保留。
5. `1018`～`1021` 主要为旧行增加字段、保留空值或使旧 OIDC 流程记录失效；首版可以直接把最终字段写入建表定义。
6. `1022_mail_result_logs.sql` 直接建立当前业务仍使用的邮件终态表和时间索引；DDL 必须保留，但不需要保留迁移编号 1022。

## 四、首版基线迁移方案

### 4.1 基线内容

新的 `0001_initial_schema.sql` 应直接定义：

1. 当前最终表及所有最终列。
2. 主键、外键及删除／置空策略。
3. NOT NULL、DEFAULT、CHECK 和唯一约束。
4. 当前最终索引，包括部分唯一索引和业务查询索引。
5. 当前产品需要的首版种子数据。

不应包含：

- 建立后马上删除的表或列；
- `ALTER TABLE` 式开发历史；
- 存量数据回填；
- 旧列到新列的转换；
- 旧 ID／`sqlite_sequence` 保留；
- 旧 OIDC state／ticket 兼容；
- “旧记录为空则按旧行为处理”的升级备注。

### 4.2 冻结的等价边界

本次数据库基线合并只做等价压缩，不顺带改变数据模型。实施时必须遵守：

- 保留当前全部表、列、列顺序、类型、NULL、DEFAULT、主键、外键、CHECK、UNIQUE、部分索引和表达式索引语义。
- 不新增当前 schema 中不存在的外键。例如 `users.group_id` 当前只有普通列和索引，本次不得借机增加 `REFERENCES groups(id)`。
- 不把 `rule_pool_sources.active_snapshot_id`／`pending_snapshot_id`、`assembly_blueprints.platform_id`／`rule_id`、`access_logs.user_id` 或 `mail_result_logs.user_id` 改造成新外键。
- 不改变任何 `ON DELETE` 行为，不调整枚举 CHECK，不改变时间字段类型和默认值。
- 不修复当前设计或安全审查中另行记录的 schema 候选问题；这些问题需要独立决策、设计和测试。
- 不删除 `schema_migrations`、迁移执行器、`go:embed` 或数据库高版本拒绝启动能力。
- 新首版最高 schema 版本固定为 1，正式发布后的第一个结构变化使用 `0002_<name>.sql`。

若候选基线与当前最终结构发生除迁移记录内容之外的任何差异，必须停止实施并报告，不得用“没有历史数据”作为接受意外差异的理由。

### 4.3 种子数据边界

- 九个预设代理组属于当前产品初始数据，应保留其当前名称、`preset_key`、启用状态和 `definition_json`。当前 JSON 仍包含 `"nodes":[]`；这属于现行种子值，本次等价合并不得擅自删除。
- 默认用户组和三个默认平台当前由 Setup 流程创建，不应在基线迁移中重复创建。
- 不得把本地测试账号、测试密码、测试 URL、测试节点、开发序列或任何密钥写入基线。

### 4.4 不应直接使用数据库 dump

SQLite `.dump` 可以作为校对材料，但不应未经审查直接作为基线，因为可能带入：

- 当前测试数据；
- `sqlite_sequence`；
- 开发期默认值或字段顺序；
- 与当前代码无关的临时对象；
- 不适合作为产品初始状态的种子数据。

应以完整迁移后的 schema manifest 为基准，人工整理一份明确、最小、可审查的最终 DDL。

### 4.5 测试替换

当前针对 `1015 → 1016`、`1018 → 1019`、`1019 → 1020`、`1020 → 1021` 的专项测试，在首版基线重建后将失去产品意义。不能只删除测试而不补替代保障。

建议替换为：

- 空库应用 `0001` 成功；
- 第二次运行幂等；
- `schema_migrations` 只登记基线版本；
- 表／列／索引／外键／CHECK／唯一约束 manifest；
- 首版种子数据准确且无开发数据；
- 迁移事务失败完整回滚；
- 数据库版本高于程序支持版本时拒绝启动；
- 一个测试专用的模拟 `0001 → 0002`，验证未来升级机制仍有效；
- `PRAGMA foreign_key_check` 无结果；
- `PRAGMA integrity_check` 返回 `ok`。

### 4.6 本次明确排除的内容

以下内容虽然继续保留在第五节作为未来研究候选，但不属于数据库迁移合并 Build：

- WireGuard 启动期敏感数组升级器及其接线；
- 素材池旧请求字段 `urls`；
- 配置导入 v1／v2 路径；
- Clash 旧蓝图和旧快照统计回退；
- OIDC、节点编辑器和浏览器草稿的旧格式推断；
- 结构体旧字段、旧签名包装函数和同版本前后端容错；
- 外部规则语法、URI、Mihomo／Shadowrocket／Xray 适配；
- 当前安全、错误处理、降级、脱敏和备份恢复能力。

这些内容不得与迁移基线压缩并行实施，也不得因旧迁移测试被删除而连带删除其业务测试。

## 五、历史兼容清理分类

### 5.1 高置信度清理候选

以下是后续应优先复核的候选，不表示已经授权删除：

1. **WireGuard 历史凭据启动迁移**
   - `backend/internal/node/sensitive_migration.go`
   - 启动时扫描历史 WireGuard 节点，为旧数组补稳定 ID 并重新加密。
   - 若首版数据库全部重建，且当前所有创建／导入入口已经规范化和加密，可删除启动迁移及其专项测试。

2. **素材池旧请求字段 `urls`**
   - 后端当前同时接受新 `sources` 和旧 `urls` 请求。
   - 可以清理旧请求输入；但响应中的 `urls` 仍被当前前端用于展示 URL 数量，不能直接连带删除，应先决定保留当前读模型还是把前端改为使用 `sources`。

3. **配置导入 v1 同步路径**
   - 当前 `FormatVersion = 2`，但版本不匹配时仍回退到旧同步导入。
   - 若首版只承诺 v2，推荐严格接受当前格式并明确拒绝其他版本。

4. **旧 Clash 装配蓝图回退**
   - 当前会检测旧 `render_plan_json` 并回退到占位符替换。
   - 没有旧蓝图后，可要求所有当前生产者始终写入完整计划并删除旧渲染路径。

5. **旧快照统计 schema 0**
   - 无效 JSON 或 `schema_version == 0` 当前会返回 legacy 统计模型。
   - 首版可以要求所有新快照使用当前 schema，并把损坏数据作为明确错误处理。

6. **已退役字段与旧签名包装函数**
   - 例如 `proxygroup.Definition.Nodes` 已不序列化、不校验、不渲染。
   - `KeyAdminInitialized` 已不写入、不参与首管理员判断。
   - `DetectOne`、`ValidateURLs`、`failSource`、`recordFailedSnapshotTx`、`mergeSensitive` 等旧签名包装函数需逐一确认是否只剩测试调用。

7. **同镜像前后端之间的旧响应容错**
   - OIDC `enabled` 缺失容错；
   - `params_state` 缺失时按旧布尔字段推导；
   - `reset_validate == 0` 时兼容旧前端未提交字段。
   - 若首版前后端总是同版本发布，可收紧为完整合同，但需要同步更新 DTO、页面和测试。

### 5.2 需要先修正生产者再删除的候选

- `NoResolve == nil` 时按旧计划类型推断；
- 缺少 `group_member_orders` 时按节点顺序生成默认成员；
- OIDC 旧 state／ticket 缺少新字段时的处理；
- 节点读取时对旧传输、安全和插件字段进行规范化；
- Setup／管理端导入对旧配置格式的额外分支；
- 浏览器 `sessionStorage`／`localStorage` 中旧装配草稿版本。

这些路径只能在证明所有当前写入者都生成规范格式后删除。若生产者仍可能生成旧结构，先删消费者会造成新数据也无法读取。

### 5.3 默认必须保留的当前能力

以下名称看似“兼容”，但通常不是项目历史包袱：

- `legacy-domain-text`、`full:`、`+.` 等外部规则来源格式；
- `LegacyMetadata`／`CanonicalizeLegacyType` 当前对外规则类型到 Canonical Rule 的投影；
- VLESS、VMess、WS、Mihomo、Shadowrocket、Xray 等外部协议或 URI 字段别名；
- Mihomo 原生 `COMPATIBLE`、`fallback` 等语义；
- HTTP `Content-Disposition` 的 ASCII fallback；
- SQLite 备份失败后的安全 snapshot fallback；
- 错误处理、脱敏、密钥保护、损坏检测和防锁死逻辑；
- 资源版本、同步历史、访问日志等当前业务功能；
- `format_version`、`state_format_version`、快照 schema 版本等未来演进标识。

若这些名称造成误解，可以评估重命名，但不能仅凭名称删除行为。

## 六、数据库基线合并完整实施方案

> 本节是已经执行的实施定稿，实际逐步记录与证据见 [Build31.md](docs/reports/Build/Build31.md)。如未来重新执行或调整基线，仍须重新建立 Build、复核当前 schema，并按 Step 0.5～8 严格串行处理。

### Step 0.5：实施启动、状态复核与边界冻结

**目标：** 把本文档中的方案转换为唯一活跃 Build，确认实施时仓库没有漂移或未解决决策。

**只读前置检查：**

1. 重新阅读 `AGENTS.md`、本文档、当时的当前工作入口和 Git 状态。
2. 确认没有其他活跃 Build 正在修改 `backend/migrations/`、`backend/internal/store/`、Setup、清库或启动路径。
3. 重新统计迁移文件、最高版本、最终表和索引；若不再是 27 个 SQL、最高 1022、34 表、25 个显式索引，则先更新 manifest 和实施文档。
4. 再次确认没有真实用户、真实业务数据、已发布数据库或必须恢复的旧 SQLite 备份。
5. 区分本地 `backend/data`、外部 `DATA_DIR` 和 Docker `vpn-data`；只确认存在性与归属，不在本 Step 删除。

**文档产出：**

- 创建新的 Build 文档，按本节拆分 Step，不把第五节的兼容清理候选纳入 Build。
- `AGENTS.md` 只登记当前工作入口和 Build 状态，不复制具体设计内容。
- 不把本工作写入 `SecurityScanPlan1.md`、`SecurityReport3.md` 或归档报告。

**验收：** 工作区基线、迁移计数、schema 对象计数、数据边界和执行范围均有当前证据；没有待用户决策项。

**停止条件：** 发现真实数据、外部持久化目标不明、其他分支并行修改迁移、当前 schema 与本文清单不一致，或用户要求兼容任何旧 SQLite 数据库／备份。

### Step 1：冻结旧链最终 schema manifest

**目标：** 在删除旧迁移前建立机器可比较、人工可审查的当前最终结构合同。

**实施方式：**

1. 使用 `store.Open` 和当前 `migrations.FS` 在 `t.TempDir()` 空库执行完整旧链，避免依赖系统 `sqlite3` CLI 的实现差异。
2. 建立测试辅助函数，稳定导出以下信息：
   - `sqlite_schema` 中非 `sqlite_%` 的表和显式索引名称；
   - 每表 `PRAGMA table_xinfo`：列序、名称、类型、NOT NULL、DEFAULT、主键序号、hidden；
   - 每表 `PRAGMA foreign_key_list`：来源列、目标表／列、更新／删除动作；
   - 每表 `PRAGMA index_list` 和每索引 `PRAGMA index_xinfo`：唯一性、来源、部分索引、列序和表达式槽位；
   - 表／索引 `sqlite_schema.sql`，用于人工核对 CHECK、表达式索引和部分索引谓词；
   - 9 个 `proxy_groups` 种子行；
   - 所有其他业务表的初始行数。
3. manifest 排序必须稳定，不包含数据库路径、时间戳、`schema_migrations` 行内容或其他运行相关值。
4. manifest 只描述当前最终 schema，不保存旧迁移 SQL 副本，不成为历史兼容夹具。

**推荐测试文件：**

- `backend/internal/store/baseline_schema_test.go`
- 如确需静态期望文件：`backend/internal/store/testdata/schema_v1_manifest.json`

是否使用 JSON 夹具可在 Build 编写时按可读性决定，但不能降低上述覆盖范围。

**验收：** manifest 明确覆盖 34 张表、25 个显式索引、全部列／外键／约束和 9 个种子；`foreign_key_check` 为空，`integrity_check` 为 `ok`。

**停止条件：** manifest 暴露出无法解释的临时表、测试数据、密钥、URL、非预期种子或损坏约束。

### Step 2：编写隔离候选 `0001_initial_schema.sql`

**目标：** 在不破坏生产嵌入迁移链的前提下编写可独立应用的首版基线。

**实施方式：**

1. 先把候选 SQL 放入 store 测试夹具目录，尚不删除 `backend/migrations/*.sql`。
2. 按依赖和可读性组织直接建表 DDL，避免当前旧链中的先建后删和跨文件 `ALTER TABLE`。
3. 直接创建最终列、外键、CHECK、UNIQUE、部分索引和表达式索引。
4. 在全部结构创建后插入 9 个预设代理组。
5. 不包含任何旧行转换、回填、旧 ID 保留、旧 state／ticket 失效、临时表或 `sqlite_sequence` 手工更新。
6. 可显式包含幂等的 `schema_migrations` 建表语句，与 `Store.Migrate` 的预建行为共存；迁移版本记录仍由 `Store.applyOne` 写入。

**DDL 排序建议：**

1. `schema_migrations`、`system_config`；
2. `groups`、`platforms`、`users`；
3. OIDC 和密码重置短期表；
4. subscriptions／versions／custom／share／rules 及其 token；
5. access logs；
6. Xray、nodes、proxy groups、group assignments 和流量表；
7. rule pool、source、snapshot、canonical 和 origin 表；
8. assembly blueprints；
9. mail result logs；
10. 预设代理组种子。

上述顺序只改善可读性，不得改变现有约束语义。

**验收：** 候选 SQL 可在空库单事务成功执行；初始对象计数和种子边界正确。

**停止条件：** 为了让候选 SQL 通过而需要改变生产代码、放宽约束、增加兼容分支或删除当前字段。

### Step 3：旧链与候选基线双库等价验证

**目标：** 在删除旧迁移前证明候选基线与当前最终结构等价。

**对比模型：**

```text
数据库 A：当前 0001～1022 完整迁移链
数据库 B：隔离候选 0001_initial_schema.sql
```

**必须相等：**

- 34 张最终表；
- 25 个显式索引；
- 每张表的列序、名称、类型、NULL、DEFAULT 和主键；
- 外键目标和动作；
- UNIQUE、CHECK、部分索引和表达式索引语义；
- 9 个代理组种子的全部字段；
- 非种子业务表为空；
- 关键约束的正反例行为；
- `foreign_key_check` 和 `integrity_check`。

**允许的差异只有：**

- A 的 `schema_migrations` 含 27 个历史版本，最高为 1022；
- B 的 `schema_migrations` 只含版本 1；
- A 的 `sqlite_schema.sql` 可能保留 `ALTER TABLE` 形成的文本布局，B 是直接 `CREATE TABLE`；比较应基于结构语义而不是原始文本相等；
- B 不保留 1016 的旧 rule pool ID／`sqlite_sequence` 防复用历史。

**约束行为探针至少覆盖：**

- users role/source/status；
- oidc state intent；
- versions owner_type；
- share token status；
- nodes source／instance 组合；
- proxy group type；
- Xray sync status/action；
- assembly target syntax；
- pool source kind/mode、snapshot status 和 sync task status；
- mail result/failure_stage 组合；
- 节点有效渲染名、默认首页规则、订阅平台、素材池来源的唯一约束。

**验收：** 除允许差异外，双库 manifest 和行为探针完全一致。

**停止条件：** 出现任何额外差异。此时保留旧迁移链，修正候选或提交用户决策，不得进入 Step 4。

### Step 4：替换生产迁移链

**目标：** 将验证通过的候选基线提升为唯一生产迁移。

**文件变更：**

- 新增 `backend/migrations/0001_initial_schema.sql`；
- 删除当前 27 个开发期迁移 SQL；
- 保留 `backend/migrations/embed.go`；
- 更新 `backend/internal/store/store.go` 中点名 `0001_init.sql` 的注释；
- 不改变 `Store.Migrate`、`applyOne`、`parseVersion`、`sortedEntries` 和 `TxImmediate` 的生产逻辑。

**静态检查：**

- `backend/migrations/` 只剩 `0001_initial_schema.sql` 和 `embed.go`；
- 生产代码没有被删除迁移文件名或版本号依赖；
- `go:embed *.sql` 正常包含新基线；
- 新空库执行后 `schema_migrations` 只有版本 1。

**验收：** `go test ./internal/store` 通过，Step 3 的等价测试改为对正式 `migrations.FS` 执行并继续通过。

**停止条件：** 迁移框架需要生产逻辑改造才能识别新基线，或正式嵌入结果与隔离候选不一致。

### Step 5：替换历史升级专项测试

**目标：** 删除失去产品意义的开发期升级测试，同时建立更强的首版基线和未来升级保障。

**删除候选：**

- `backend/internal/store/migration_1016_test.go`
- `backend/internal/store/migration_1018_test.go`
- `backend/internal/store/migration_1019_test.go`
- `backend/internal/store/migration_1020_test.go`
- `backend/internal/store/migration_1021_test.go`
- `backend/internal/store/migration_helpers_test.go`

**必须保留或新增的测试：**

1. 空库应用正式 `0001` 成功。
2. `schema_migrations` 只有版本 1。
3. 同一 Store 重复迁移幂等。
4. 关闭／重新打开后重复迁移幂等。
5. 完整 schema manifest 匹配。
6. 只有 9 个代理组种子，默认组／默认平台／配置尚未创建。
7. Setup 后恰有 1 个默认组和 3 个默认平台，重启不重复插入。
8. 候选基线末尾注入失败语句时，业务 DDL、索引和种子全部回滚；允许迁移器在事务外预建空 `schema_migrations`，但不得有版本 1 记录。
9. 测试专用 `0002_probe.sql` 可从版本 1 升至版本 2，并保持重启幂等。
10. 数据库伪造更高版本时仍拒绝启动。
11. `foreign_key_check` 为空，`integrity_check` 为 `ok`。
12. 9 个代理组种子后创建新 proxy group 得到连续的新 ID；空 `rule_pools` 中创建首个素材池从 ID 1 开始，不继承 1016 的旧素材池 ID 防复用逻辑。

仓库内直接使用 `migrations.FS` 的业务测试继续保留，它们会自然改为覆盖新基线。各包自带最小 `fstest.MapFS` 的隔离测试也默认保留；它们不是旧数据库兼容测试，不能仅因文件名仍叫 `0001_init.sql` 或 `100x_*.sql` 就批量改写。

**验收：** store 测试不再依赖 1015～1021 的真实升级链，同时完整覆盖基线、回滚、幂等、高版本拒绝和未来 `0002`。

**停止条件：** 删除历史测试后出现无法由新合同测试解释的覆盖缺口，或业务测试仍真实依赖旧中间 schema。

### Step 6：本地空库切换与 Setup smoke

**目标：** 在精确限定的本地测试数据范围内验证真实启动路径。

**重要版本边界：** 新程序最高版本为 1。任何含 `schema_migrations=1022` 的旧数据库都会被 `Store.Migrate` 判定为“数据库版本高于程序支持版本”，进入应急模式。这是有意的不兼容边界，不是需要新增升级桥的错误。

**为什么不能使用普通一键清空切换：** `dataclear.ClearTablesTx` 有意保留 `schema_migrations`。旧数据库即使清空全部业务表，版本 1022 仍存在，重启后仍会被新程序拒绝。应急模式在数据库可读时也复用该 SQL 清空路径，因此不能把应急“重新初始化”当作 1022→1 的版本重置工具。

**执行前检查：**

1. 停止所有使用目标数据库的本地进程。
2. 从仓库根目录解析并打印规范化绝对路径。
3. 目标只能是 `/Users/kyle/Desktop/Repo/VPN-Subscription-Management/backend/data`；不得使用 `$HOME`、`~`、未解析变量、宽泛 glob 或工作区根目录。
4. 核对未设置外部 `DATA_DIR`，也未把目标指向 Docker 卷或其他环境。
5. 再次列出将处理的 `app-dev.db`／`app-prod.db` 及其 `-wal`／`-shm` 文件。
6. 删除动作必须依赖用户对代码实施和本地测试数据清理的明确授权；本方案文档授权本身不等于删除授权。

**smoke：**

1. 从空 `backend/data` 启动 dev，确认自动创建 schema version 1 并进入 Setup。
2. 执行 Quick Start，核对 1 个默认组、3 个默认平台、签名密钥、`configured=true` 和 `frontend_url`。
3. 关闭并重启，确认 `0001` 不重复、Setup 种子不重复。
4. 执行一键清空，确认业务数据被清除、schema version 1 保留、系统回到 Setup。
5. 再次完成 Setup，确认清空生命周期仍正常。

**Docker 边界：** Compose 使用独立的 `vpn-data:/data` 命名卷。除非用户单独授权并核对目标，否则不删除或重建该卷。Docker smoke 应使用明确的新空卷或经确认可丢弃的项目卷。

**停止条件：** 目标路径不精确、进程仍占用数据库、存在外部 `DATA_DIR`、卷归属不明、发现真实数据，或旧数据库需要保留。

### Step 7：联合自动化与隔离回归

**后端门禁：**

```bash
cd backend
go test ./...
go test -race ./...
go build ./...
go vet ./...
```

若仓库现行 errgate／架构静态门禁仍可用，应按当前 `AGENTS.md` 和最近 Build 的实际入口一并执行，不得从历史文档猜测命令。

**前端门禁：**

```bash
cd frontend
npm test
npm run build
```

前端没有 schema 代码变更，但完整构建可证明嵌入交付和同镜像前后端没有被文档／构建调整破坏。

**联合 smoke 范围：**

- Setup、本地账号和 OIDC 配置基本路径；
- 配置导入导出当前格式；
- 素材池创建、同步、快照和手工激活；
- 手动节点创建、保存、重新打开、检查和装配；
- 预设代理组；
- 邮件结果日志写入、分页和 90 天清理；
- 一键清空和重启幂等；
- Docker 空卷首次启动；
- `foreign_key_check` 与 `integrity_check`。

**仓库检查：**

```bash
git diff --check
```

另行搜索生产目录中的旧迁移文件名、`migrationsThrough`、1015～1022 迁移测试 helper 和不应存在的旧表／旧列。搜索无匹配和搜索命令失败必须分开判定。

**证据边界：** 自动化、接口级 smoke 和 Docker 空卷只能证明工程行为；不得表述为真实 SMTP、真实 OIDC、真实客户端、真实浏览器或用户人工验收通过。

**停止条件：** 任一门禁失败、出现数据竞态、空卷不能进入 Setup、重启重复种子，或新基线与 manifest 漂移。

### Step 8：文档同步、归档与最终交付

**当前事实文档：**

- 更新本文档的实施状态、最终对象计数、实际测试证据和变更记录；
- 更新 `AGENTS.md` 的当前工作入口、Build 清单和归档状态；
- Build 文档记录每个 Step 的实际文件、命令、结果、未执行人工项和停止条件；
- 根目录 `Issue18.md` 中对 `1022_mail_result_logs.sql` 的当前表述改为“该表最初由 1022 引入，首版基线合并后由 0001 直接创建”，避免留下当前路径误导。

**不得批量改写：**

- `docs/reports/` 下的历史 Design／Build／Issue／SecurityReport；
- 历史报告中当时真实存在的 1001～1022 文件名和版本证据；
- `SecurityScanPlan1.md`／`SecurityReport3.md` 的步骤、证据或状态。

安全报告中的旧路径若需要重新定位，应由安全审查自己的后续 Step 处理；本 Build 只可说明历史报告未被改写，不得替其更新结论。

**最终交付应明确：**

- 代码和 SQL 实际变更；
- 删除的历史迁移与测试；
- 新增的 schema 合同测试；
- 版本重置和旧数据库不兼容边界；
- 自动化／smoke 实际结果；
- Docker 卷、真实服务和人工项目是否执行；
- 工作区是否只含本 Build 变更。

**归档条件：** Step 0.5～8 全部完成、所有自动化门禁通过、当前文档同步完成且不存在待处理代码问题。未执行的真实环境／人工项应迁往其正式跟踪文档，不得用“未执行”阻塞纯数据库基线 Build 的工程归档。

### 6.9 实施时需要用户再次确认的事项

代码实施已经完成，当前没有遗留代码阻塞。后续若要把任何既有环境切换到新基线，仍必须确认实际需要处理的持久化目标：

1. 只处理当前仓库 `backend/data`，还是还要处理 Docker `vpn-data`；
2. 是否存在外部 `DATA_DIR` 或需要保留的 SQLite 备份；
3. 是否要求保留任何旧镜像到新镜像的滚动升级能力。

推荐默认值是：只清理经过路径核对的当前仓库本地测试数据库；Docker 卷、外部 `DATA_DIR` 和备份一律不动；不支持旧镜像滚动升级。若用户选择不同边界，必须在 Step 0.5 修改 Build 后再实施。

## 七、供其他 AI 使用的只读扫描提示词

```text
你正在审查项目：

/Users/kyle/Desktop/Repo/VPN-Subscription-Management

目标：
该项目尚未发布首个正式版本，没有真实用户、真实业务数据或需要保留的历史数据库。计划以当前版本作为首版，因此需要识别并清理“仅为开发期旧版本、旧数据、旧 API、旧蓝图或旧导入格式服务”的兼容代码。

本次任务只允许研究和报告：
- 不修改任何代码、SQL、测试或文档；
- 不创建新文件；
- 不删除数据；
- 不执行会改变仓库、数据库或外部系统状态的命令；
- 遵守仓库根目录 AGENTS.md；
- 如发现需要产品决策的边界，列为“待用户决策”，不要自行决定。

核心原则：
1. 不要因为名称包含 legacy、compatibility、fallback、alias、old、v1、deprecated、历史、兼容、旧版就判断可以删除。
2. 必须区分：
   A. 项目开发期历史数据／历史版本兼容；
   B. 当前 HTTP API 或前后端合同；
   C. 外部协议、URI、客户端、规则文件格式兼容；
   D. 安全、错误处理、降级、默认值或跨平台互操作；
   E. 当前业务本身需要的版本、日志、同步历史；
   F. 仅供测试使用的包装函数或夹具。
3. 只有 A 类是本次直接清理候选。B 类需要证明当前唯一调用方已经使用新合同；C、D、E 类默认不得删除。
4. 每个候选都必须追踪生产者、持久化位置、消费者、API／前端调用方、测试和文档，不能只看注释或函数名。
5. 对数据库字段或 JSON 格式的兼容分支，要证明当前所有写入入口都不会再产生旧格式，才能建议删除读取兼容。
6. 对外部导入格式要判断它是“旧项目格式”还是“仍在使用的行业／客户端格式”。

优先检查范围：

一、数据库迁移
- backend/migrations/
- backend/internal/store/
- backend/internal/store/migration_*_test.go
- schema_migrations 的运行机制
- 先创建后删除的表／列／索引
- UPDATE 回填、旧 ID／sequence 保留
- 仅为旧行提供 DEFAULT 的字段
- 旧 schema 到新 schema 的专项测试
- 启动后执行的数据升级器

重新统计当前迁移数量和最高版本，并判断完整迁移链能否压缩为一个首版 0001_initial_schema.sql；不得沿用本文档中的历史数量而不复核。
迁移框架本身是否应保留要单独评价，不得把“删除旧迁移链”等同于“删除迁移机制”。

二、运行时历史数据升级
重点搜索：
- Migrate*
- migration / migrate
- legacy row / old row
- PRAGMA table_info / sqlite_master / hasTable
- 启动时扫描和重写已有记录
- 缺列、缺字段或旧 JSON 的自动补全
- 旧密文或旧凭据升级

三、API 和前后端合同
检查：
- 同时接受新旧请求字段；
- omitempty、可选字段和缺字段默认值；
- 同一语义的两套 DTO；
- 旧响应／缓存降级推断；
- 已下线端点仍保留；
- 前端是否仍真实使用旧字段；
- 测试是否是唯一调用者。

重点关注但不限于：
- 素材池 sources 与 urls；
- OIDC enabled、params_state 和旧布尔状态；
- reset_validate 缺失默认值；
- proxy group 的旧 nodes 字段；
- admin_initialized；
- 保留旧函数签名的包装函数。

四、导入导出和持久化 JSON
检查：
- format_version 分支；
- v1／v2 双路径；
- 版本不匹配时警告后继续；
- 旧配置键、未知键和旧占位符；
- 旧装配 blueprint／render_plan_json；
- state_format_version；
- snapshot stats schema_version；
- sessionStorage／localStorage 草稿版本；
- 缺字段时推断旧行为；
- 当前生产者是否总能生成最新格式。

五、节点与装配
检查：
- 旧传输、安全、插件字段别名；
- 读取时规范化但不回写；
- 旧蓝图占位符替换；
- NoResolve 缺失推断；
- group_member_orders 缺失回退；
- 历史敏感字段迁移；
- 仅为旧测试保留的函数或字段。

注意：
URI、Mihomo、Shadowrocket、Xray、Clash 等外部格式别名可能仍是当前输入能力，不得误判为项目历史兼容。

六、规则素材与客户端兼容
检查 legacy-domain-text、LegacyMetadata、CanonicalizeLegacyType 等名称。
必须回答：
- 它是在兼容项目旧数据，还是当前外部规则语法／当前 UI API 的正式投影？
- 删除后哪些真实输入格式、客户端或页面会停止工作？
- 如果功能必须保留，是否只是应该重命名以消除误导？

建议搜索词：
legacy|compatib|backward|deprecated|obsolete|old|v1|v2|migration|
migrate|fallback|alias|missing|default|schema_version|format_version|
历史|兼容|旧版|旧版本|旧字段|旧格式|回填|保留旧|降级

不要只依赖关键词；还要检查以下结构模式：
- if version != current
- if field == "" / nil 后推断旧值
- 同时存在 old／new 字段
- wrapper 调用新版实现
- 表／列存在性探测
- JSON Unmarshal 失败后返回旧模型
- 请求新字段为空时读取旧字段
- 旧格式检测后走另一条执行路径
- 启动时扫描并重写全部记录
- 测试专用旧 schema MapFS

输出要求：

先给出总体结论，再提供表格。每一个候选至少包含：

- 编号；
- 分类 A／B／C／D／E／F；
- 文件及精确行号；
- 函数、类型、字段或迁移名；
- 兼容对象是什么；
- 触发条件；
- 当前生产者；
- 当前消费者；
- 是否存在生产调用；
- 关联数据库／API／前端／测试／文档；
- 删除建议：
  - 可直接删除；
  - 先修改生产者再删除；
  - 只删除旧分支、保留当前能力；
  - 保留但重命名；
  - 必须保留；
  - 待用户决策；
- 删除影响和风险；
- 需要新增或调整的回归测试；
- 支撑判断的代码证据，不要只引用注释。

最后整理四张清单：

1. 高置信度可删除项；
2. 需要先迁移当前调用方的项目；
3. 名称像兼容代码、但属于当前外部格式或安全可靠性、必须保留的项目；
4. 需要用户确认的产品边界。

对迁移目录另给出：
- 当前最终表／索引／外键／约束／种子数据清单；
- 旧迁移链中纯历史步骤；
- 单一首版基线迁移应包含的内容；
- 应删除或替换的迁移专项测试；
- 首版基线验证矩阵。

禁止实施任何修改。发现结论不确定时明确说明缺少什么证据。
```

## 八、实施前影响评估与授权闸门

### 8.1 受影响范围

数据库基线合并会直接影响：

- `backend/migrations/` 的全部 SQL 文件布局和嵌入结果；
- `backend/internal/store/` 的迁移专项测试和基线合同测试；
- 新空库首次启动、Setup、重启和一键清空；
- Docker 空卷构建／启动验证；
- 当前文档、Build 记录、`AGENTS.md` 当前工作入口以及 `Issue18.md` 的 1022 当前引用。

它不会直接改变：

- HTTP API、前端 DTO 或页面行为；
- 当前配置 `.enc` 导入／导出格式；
- 节点、素材池、装配、OIDC、SMTP 或 Xray 的业务逻辑；
- 外部协议、URI、客户端和规则格式；
- 已归档历史报告中的迁移编号证据。

### 8.2 已知运行影响

- 新空库直接执行版本 1，不再依次执行 27 个开发期迁移。
- 已含 1022 迁移记录的旧数据库会被新程序拒绝并进入应急模式。
- 普通一键清空和数据库可读时的应急 SQL 重新初始化均保留 `schema_migrations`，不能把旧库转换为新基线。
- 旧 SQLite 备份不能直接恢复给新程序；需要保留时必须停止本方案并另做兼容迁移设计。
- 当前 `.enc` 配置导入是否接受旧格式不由本次工作改变；数据库不兼容不能被表述为配置导入也必然不兼容。
- 版本号重置不影响全新部署，但禁止旧新镜像针对同一数据卷做滚动升级。

### 8.3 授权分层与实际结果

本工作必须保持三层授权分离：

1. **方案文档授权：** 已获得并完成定稿。
2. **代码实施授权：** 已获得并按 [Build31.md](docs/reports/Build/Build31.md) 完成。
3. **数据清理授权：** 未获得；本次没有删除或修改任何既有持久化目标，只使用 `t.TempDir()` 和任务专用 Docker 临时卷完成验证。

代码实施完成后，本文档仍不把第五节兼容清理候选升级为当前 TODO，也不修改 `TODOLIST.md`。

### 8.4 当前未决项结论

方案层面没有需要立即向用户确认的阻塞问题。以下事项已经采用保守默认值并在实施闸门再次核对：

- schema 合并只保持等价，不修正既有约束；
- 只处理数据库迁移链，不处理第五节兼容代码；
- 历史报告不改写；
- 本地仓库数据、Docker 卷、外部 `DATA_DIR` 和备份分别授权；
- 真实服务／客户端／浏览器结果不由自动化推断。

## 九、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-16 | 记录首版数据库基线压缩、迁移框架保留理由、历史兼容分类、未来阶段建议及其他 AI 只读扫描提示词；明确非当前活跃工作且不纳入 TODOLIST。 |
| v2.0 | 2026-09-20 | 重新验证当前 0001～1022 共 27 个 SQL、611 行、34 表和 25 个显式索引；纳入 1022 邮件终态表；冻结等价合并范围、旧数据库不兼容边界、Step 0.5～8 串行实施定稿、测试替换矩阵、空库切换和三层授权闸门。本次仅更新本文档，未授权或实施代码与数据变更。 |
| v2.1 | 2026-09-20 | 数据库基线合并已按 Build31 完成：生产迁移压缩为单一 0001，合同测试、联合门禁和独立 Docker 空卷 smoke 完成；既有数据目标未触碰，第五节兼容清理候选仍未激活。 |
