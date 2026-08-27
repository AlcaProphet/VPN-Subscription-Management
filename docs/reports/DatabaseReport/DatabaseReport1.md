# DatabaseReport1.md — VPN 订阅管理系统 数据库选型与素材池锁库问题研究报告（第一期）

> **文档定位：** 本文档记录对 R22-01（素材池大数据量同步锁库）的**深入研究与数据库选型分析**，包括实测基准、根因、候选方案对比、SQLite 适用性评估与最终建议。属于存档研究报告，不作为当前强要求；编码强要求仍见 [AGENTS.md](../../../AGENTS.md)。
> **研究方式：** 静态代码走读 + 使用项目实际依赖（`modernc.org/sqlite`）做隔离基准测试。**未修改任何代码、未执行任何修复**。
> **研究基线：** `beta` 分支 HEAD `f20b3b1`，工作区除 `Issue7.md` 文档外无业务代码改动。
> **归档说明：** 本文档于 2026-08-26 新建于 `docs/reports/DatabaseReport/`，供后续 Issue7 / Build / 数据库架构决策引用。

---

## 一、结论前置

1. **R22-01 的本质不是“SQLite 放不下 10 万行”，而是当前数据模型 + 事务模式错误**：把每个规则条目作为一行，并在一个 `BEGIN IMMEDIATE` 长事务里完成“10 万行 upsert + 临时 keep 表 + NOT EXISTS 删除”。
2. **URL 快照表是正确且足够根本的方向**：继续使用 SQLite 时，建议把 URL 条目从逐行存储改为“每个 URL 一行 JSON/压缩快照”，彻底移除 URL 条目的逐行写入与逐行删除。
3. **当前不建议换 PostgreSQL/MySQL**：换库能消灭“全局写锁”这一类问题，但对当前小团队、单容器、SQLite 内嵌的项目来说，破坏面和运维成本远超收益。
4. **SQLite 仍适合当前项目**；换库应是在多实例、高并发写、HA、在线备份等硬需求出现后，由产品规模驱动的架构升级，而不是为了修这一个 bug。
5. **本报告仅探索与记录，不执行任何修复。** 已与用户确认“当前已倾向 URL 快照表方案”，但后续是否落库仍需单独确认。

---

## 二、问题背景

### 2.1 现象

- 来源地址小于几千行时能正常处理，例如 `https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/google-cn.txt`；
- 来源大于数万行时（如 `https://raw.githubusercontent.com/Loyalsoldier/v2ray-rules-dat/release/direct-list.txt`）系统卡死并导致数据库锁死；
- 容器日志：
  ```text
  vpn-sub-1  | time=2026-08-26T06:47:38.527Z level=ERROR msg=内部错误 path=/api/admin/pools/1/sync msg="开启事务失败: database is locked (5) (SQLITE_BUSY)"
  ```
- 该错误出现在 `POST /api/admin/pools/1/sync` 调用 `SubmitSync` 的 `TxImmediate` 尝试开启写事务时。

### 2.2 当前实现事实

`backend/internal/pool/sync.go` 的 `runSyncTask` 仍把以下工作放在同一个 `bgStore.TxImmediate` 长事务中：

1. 批量 `INSERT OR IGNORE` 全部 URL 条目；
2. 创建临时 keep 表并填充全部条目；
3. 统计将被删除的行数；
4. `DELETE ... NOT EXISTS` 差量删除；
5. 任务状态、池快照、历史清理回写。

问题点：

- SQLite 是**单写者**：无论多少连接，同一时刻只能有一个写事务，写锁粒度是**整个数据库文件**；
- `busy_timeout=5000`，其他写事务等待 5 秒后即 `SQLITE_BUSY`；
- 临时 keep 表无索引时，`NOT EXISTS` 与删除前统计接近 O(N×M)；
- 10 万行源在单事务内插入、建临时表、删除，写锁持有时长被显著放大；
- 此前 Issue4 R19-01 的“独立连接 + 批量写入”只缓解 API 单连接被占用，未解决“单写者长事务锁死”的核心。

---

## 三、实测基准

测试环境：macOS ARM、Go 1.26、`modernc.org/sqlite v1.56.0`、WAL、`busy_timeout=5000`。使用项目同款 Go SQLite 驱动模拟 2 万 ~ 50 万行规则条目。

| 方案 | 规模 | 结果 |
|---|---:|---|
| 当前实现：单长事务 + keep 表**无索引** | 2 万行 | 25.7s；20k→40k 约 4 倍增长，属 O(N²)，10 万行预计数分钟以上 |
| 单长事务 + keep 表加索引 | 10 万行 | 1.44s，锁持续整个事务 |
| 单长事务 + keep 索引 + 10 万旧行替换删除 | 10 万行 | 1.79s，锁持续整个事务 |
| 短事务分批（每批 500 行） | 10 万行 | 总 1.31s，单批最长 10.9ms |
| 短事务分批 | 50 万行 | 总 6.51s，单批最长 14.6ms |
| 短事务分批 + 另一连接持续竞争写 | 50 万行 | 竞争写入最大等待 **4.6s**，虽未失败但已接近 5s 阈值 |
| **JSON 快照：一次 UPDATE/INSERT 一行** | 10 万行 | 28.8ms，JSON 4.6MB |
| **JSON 快照：一次 UPDATE/INSERT 一行** | 50 万行 | 136.6ms（默认同步模式约 157ms），JSON 23.4MB |
| JSON 快照 + 另一连接持续竞争写 | 50 万行 | 竞争写入最大等待 **57ms** |
| 旧 10 万行一次性迁移为 JSON 快照 | 10 万行 | 149ms |
| 50 万行 JSON 反序列化 | 50 万行 | 约 377ms |
| gzip(BestSpeed) 压缩 50 万行 JSON | 50 万行 | 23.4MB → 1.3MB，压缩 34ms，解压 27ms |

### 3.1 关键结论

- 当前锁死的直接放大因素是 keep 表无索引导致的 O(N²)。
- 即使“加索引 + 短事务”，50 万行写入仍会产生大量 WAL 页和事务竞争；在持续并发写测试中曾出现近 4.6s 的等待。它大概率能救回 10 万行场景，但**没有从数据模型上消灭锁风险**。
- URL 快照方案把“每行一个 SQL 写入”变成“每个 URL 一个快照写入”，50 万行也只是一次约 100~160ms 的短事务；并发竞争下其他写请求最长只等约 57ms。这才是根本性变化。

---

## 四、根因分析

### 4.1 SQLite 的锁模型

- SQLite 是**单写者**：同一时刻只能有一个写事务；写锁粒度为整个数据库文件。
- 任何模块的长写事务（本次是素材池同步）都会阻塞**所有**其他写事务，包括用户管理、日志写入、平台保存等，5 秒后 `SQLITE_BUSY`。
- WAL 模式解决“读写互相阻塞”，但**不解决“写写互相阻塞”**。
- 大事务结束后还会触发 WAL checkpoint，极端情况下下一次写事务仍可能短暂等待。

### 4.2 为什么 PostgreSQL/MySQL 不会这样

- PostgreSQL / MySQL（InnoDB）采用 MVCC + 行级锁：素材池写入 10 万行时，其他表（用户、日志、平台）的写操作通常不受影响。
- 不会出现 SQLite 这种“整个数据库被某个写事务锁住”的问题。
- 代价是引入外部数据库服务、连接池、认证、备份、迁移方言、运维复杂度，以及网络往返开销。

### 4.3 结论

如果当前数据模型不做快照化，PostgreSQL 确实能绕开 R22-01；但一旦采用 URL 快照模型，SQLite 的全局单写者对这个项目就不再构成实际瓶颈。

---

## 五、候选方案对比

### 方案 A：当前 Issue7 方案（keep 索引 + 短事务 + pool_entries 索引）

- 优点：改动最小，10 万行大概率可用。
- 缺点：
  - 仍是 10 万 ~ 50 万次逐行 INSERT/DELETE，WAL 与索引写入放大；
  - 事务竞争和 checkpoint 可能带来偶发近 5s 等待；
  - 短事务拆分后失去“整次同步全部数据变更原子生效”的性质；
  - 数据规模继续增长时问题会重现。

### 方案 B：URL 快照表（推荐）

将 URL 来源条目从 `pool_entries` 逐行表迁移到快照表：

```sql
CREATE TABLE pool_url_snapshots (
    pool_id      INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
    source_url   TEXT    NOT NULL,
    entries_json TEXT    NOT NULL,  -- 或 entries_blob BLOB 存 gzip
    entry_count  INTEGER NOT NULL,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (pool_id, source_url)
);
```

`pool_entries` 仅保留 manual 条目；URL 条目按 `source_url` 存 JSON 快照，每个条目携带 `rule_type / match_value / sort_order`。

同步流程变为：

1. 拉取并解析所有 URL，结果留在内存；
2. 读取旧快照与 manual 条目；
3. 在内存中计算每个 URL 的 added / removed / 最终条目序列；
4. **一个短事务**完成：
   - 成功 URL 快照 upsert；
   - 全部成功时删除已不存在 URL 的快照行；
   - 任务状态与池快照回写。
5. 没有临时 keep 表，没有 `NOT EXISTS`，没有 10 万行逐行删除。

优点：

- 写锁时间从“数分钟/数秒”降到“几十到一百多毫秒”；
- 数据变更仍可在单个事务内原子生效；
- 保留“部分失败不删除旧数据”的现有语义；
- 每池最多 50 个 URL，因此最多操作 50 行；
- 50 万行反序列化约 0.4s，装配渲染本身会消耗更多时间，因此读取代价可接受。

可选增强：

- 快照用 `gzip(BestSpeed)` 存 BLOB：50 万行从 23.4MB 降到 1.3MB，写锁和 WAL 压力进一步下降；
- 每池快照增加 `checksum` / `content_hash`，内容未变化时跳过写库；
- 列表页若嫌每次解码大 JSON 较慢，可增加按“快照版本号”失效的内存缓存。

### 方案 C：不可变快照 + 当前指针（更激进）

```text
pool_url_snapshots(id, pool_id, data, created_at)
rule_pools.current_snapshot_id
```

每次同步：

1. 追加写入一个不可变快照；
2. 一个极短事务切换 `current_snapshot_id`；
3. 旧快照后台保留 1~2 代或 7 天清理。

优点：

- 快照只追加、从不原地更新；
- 切换指针事务极短；
- 天然支持“回滚到上次同步结果”。

缺点：

- 每次同步都保留一份完整历史快照，存储增长较快；
- 需要额外清理任务；
- 当前项目已有 `pool_sync_tasks` 历史，再保留数据快照历史有些重复。

### 方案 D：素材池条目落文件系统

同步解析后写 `data/pool/<pool_id>/url.json.gz`，DB 只存元数据与状态。

优点：大 JSON 完全离开 SQLite，DB 写锁彻底不受条目数量影响。

缺点：

- DB 与文件是两套真相，需处理崩溃一致性；
- 备份、一键清空、池删除、恢复都要扩展文件目录处理；
- 比快照表更分散，当前项目没有必要引入。

### 方案 E：独立素材池 SQLite 数据库

把 `pool_entries` / 快照放入独立 `pool.db`，主库 API 不受池同步写锁影响。

优点：主库彻底隔离。

缺点：

- 备份、清空、应急模式、迁移、外键关系都要多库适配；
- 若仍保留逐行模型，只是把锁搬到 pool.db，没有根本解决问题；
- 若同时采用快照模型，独立库的收益又变得很小。

### 方案 F：换 PostgreSQL / MySQL

见第六节。基本能消灭“全局 database is locked”，但成本较高。

---

## 六、数据库选型分析

### 6.1 如果换 PostgreSQL 会怎样

#### 风险是否会消失

基本会。即使继续沿用当前“逐行 upsert + 删除”模型，PostgreSQL 的行级锁/MVCC 也不会让素材池同步阻塞用户表、日志表等无关写入。

但注意：

- 同一个超长事务在 PostgreSQL 中仍可能产生大量 WAL、长事务导致 vacuum 压力、以及同表/同行的锁等待；
- 只是“全局 database is locked”这类故障消失，不等于性能问题自动消失。

#### 迁移成本

| 项目 | 影响 |
|---|---|
| 部署 | `docker-compose` 增加 PostgreSQL 服务、数据卷、健康检查，不再“零依赖单容器” |
| 驱动/连接 | `modernc.org/sqlite` → `pgx`/`lib/pq`，连接池管理 |
| SQL 方言 | `AUTOINCREMENT`、`INSERT OR IGNORE`、`datetime()`、`VACUUM INTO` 等均需改写 |
| 迁移框架 | 当前 1013 个 SQLite SQL 迁移需转换为 PostgreSQL 迁移 |
| 测试 | 当前所有 Go 测试直接创建 SQLite 临时库，需重构为测试数据库 |
| 备份 | 现有 `VACUUM INTO` + tar 逻辑要改为 pg_dump/WAL 备份 |
| 一键清空/应急 | 事务语义可保留，但需要重写 |
| 资源占用 | 容器镜像/内存/CPU 显著增加 |

#### 结论

- 为修 R22-01 换库不划算。
- 换库合理的前提是：多实例部署、高并发管理写入、需要数据库 HA、在线备份/时间点恢复、团队规模扩大。
- 如果未来真要换，优先 **PostgreSQL**，而不是 MySQL，原因是 JSONB、MVCC、约束能力和生态更适合当前 JSON 快照模型。

### 6.2 其他候选

- **MySQL / MariaDB（InnoDB）**：行级锁，也能绕开 SQLite 的全局写锁；但 JSON 能力、约束能力不如 PostgreSQL，运维形态类似外部服务，因此优先级低于 PostgreSQL。
- **bbolt / Badger / Pebble 等嵌入式 KV**：仍是嵌入式 Go 方案，但要抛弃 SQL 与关系模型，重写面极大；bbolt 本身仍是单写者，收益有限。
- **CockroachDB / TiDB 等分布式数据库**：适合横向扩展/多地域，但对当前小团队严重过度设计。
- **rqlite / dqlite / libSQL**：本质仍是 SQLite 或分布式变体，复杂度和收益不匹配。

### 6.3 SQLite 是否仍最适合当前项目

**是，至少当前阶段仍应继续使用 SQLite。**

理由：

1. 项目定位是小团队自托管、单容器、零配置；
2. 管理端写入并发很低，下载端以只读为主；
3. 数据规模小；快照方案下 50 万行也只是一个 1~23MB 的 BLOB/TEXT；
4. 备份、迁移、测试、应急恢复都已经围绕 SQLite 完成；
5. 换外部数据库会直接违背 AGENTS 中“零配置启动、单二进制、嵌入式存储、docker compose 单服务”的架构取向。

SQLite 的适用边界依然明确：单节点、低并发写、GB 级数据库、少量管理员。当前项目恰好落在边界内。

### 6.4 建议的换库触发条件

出现以下任一条件时，再评估迁移 PostgreSQL：

- 需要多实例 / 水平扩展；
- 管理端并发写 QPS 持续升高（如数十写/s 以上）；
- 需要数据库 HA / 主从 / 时间点恢复；
- 需要在线备份而不接受 `VACUUM INTO` 的短暂锁定；
- 数据规模进入数十 GB 以上并出现 SQLite 维护痛点；
- 团队/组织规模扩大，需要专职 DBA 或成熟数据库运维。

---

## 七、推荐最终形态（供后续决策，暂不实施）

1. **维持 SQLite**。
2. R22-01 采用 **URL 快照表**，而不是继续优化逐行 `pool_entries`。
3. 快照格式建议：
   - 每 URL 一行；
   - JSON 或 gzip BLOB；
   - 保留每条 `rule_type / match_value / sort_order`；
   - 内存中计算 added/removed，保持现有“全部成功才删除、部分失败只新增不删除”语义；
   - 一个短事务写入快照、任务状态与池快照。
4. 可选增强：
   - 内容 hash 不变时跳过写库；
   - gzip 压缩；
   - 同步后后台 checkpoint，避免 WAL 检查点对下一次写事务造成尖峰。

---

## 八、若采用 URL 快照表的影响范围（评估）

- `backend/migrations`：新增快照表迁移；将现有 URL 行聚合为 JSON 后删除 URL 行（实测 10 万行迁移约 149ms）。
- `backend/internal/pool/sync.go`：重写同步写入阶段，改为内存计算 + 快照 upsert。
- `backend/internal/pool/pool.go`：`List` / `ListEntries` / manual 冲突检查适配快照读取。
- `backend/internal/assembly/load.go`：素材池读取改为 manual + URL 快照合并。
- `backend/internal/dataclear`：一键清空清单增加 `pool_url_snapshots`。
- `backend/*_test.go`：素材池/装配/清空/应急测试适配。
- 前端 API 形状基本不变；URL 条目可继续以 `source=url` 返回，ID 使用合成值或前端复合 key。
- 备份/恢复仍为单 DB，无需改备份模型。

---

## 九、状态与后续

- 本报告为**探索性研究**，未修改任何代码、未执行任何修复。
- 用户当前倾向 URL 快照表方案，但仍希望继续探索数据库选型（本报告已覆盖）。
- 下一步（需用户确认后再执行）：
  1. 若确认采用 URL 快照表，将把本报告结论整合进 Issue7 的 R22-01 方案；
  2. 若确认仍维持“keep 索引 + 短事务”最小改动，则保留 Issue7 既有方案；
  3. 若确认评估换库，则需先产出独立的 PostgreSQL 迁移/部署/备份专项方案。

---

## 变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-26 | 新建 DatabaseReport1：R22-01 锁库问题的深入研究与数据库选型分析，含实测基准、方案对比与 SQLite 适用性评估；仅研究，未实施 |
