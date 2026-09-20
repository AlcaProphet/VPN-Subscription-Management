# Build31.md — 首版数据库基线合并

> **文档定位：** 本文档是已完成并归档的构建记录，依据已归档的 [V4UpgradeRelated.md](../Others/V4UpgradeRelated.md) 定案范围，将开发期 `0001`～`1022` 迁移链等价压缩为首版 `0001_initial_schema.sql`。
> **归档状态：** ✅ Build31 已归档于 `docs/reports/Build/`；关联方案文档已归档至 `docs/reports/Others/`。本 Build 未实施 V4UpgradeRelated.md 第五节的历史兼容清理候选。
> **编码约束：** [AGENTS.md](../../../AGENTS.md) 是唯一强要求文档。
> **授权边界：** 用户已于 2026-09-20 明确授权开始实施合并。该授权覆盖 Build、SQL、Go 测试及必要文档同步；不授权删除 `backend/data`、Docker 卷、外部 `DATA_DIR` 或备份。

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|---|---|---|
| 0.5 | 启动实施、复核状态、冻结范围 | ✅ 验收通过 |
| 1 | 冻结旧链最终 schema manifest | ✅ 验收通过 |
| 2 | 编写隔离候选基线 | ✅ 验收通过 |
| 3 | 双库等价验证 | ✅ 验收通过 |
| 4 | 替换生产迁移链 | ✅ 验收通过 |
| 5 | 替换历史升级专项测试 | ✅ 验收通过 |
| 6 | 临时空库、Setup 与独立 Docker 空卷 smoke | ✅ 验收通过（未删除任何既有数据） |
| 7 | 联合自动化与隔离回归 | ✅ 验收通过 |
| 8 | 文档同步、归档与交付 | ✅ 验收通过 |

## 二、影响评估与冻结边界

- 影响 `backend/migrations/`、`backend/internal/store/` 的迁移合同测试，以及当前事实文档。
- 保留迁移器、`schema_migrations`、事务、幂等和高版本拒绝逻辑；不改服务器启动接线。
- 新基线必须语义等价复现当前 34 张表、25 个显式索引和 9 个代理组种子；不增删业务字段、约束或级联。
- 旧数据库最高版本 1022，新代码最高版本 1；两者明确不兼容，不增加升级桥或滚动升级能力。
- Setup 创建的默认组与三个默认平台不进入迁移种子。
- 不修改外部协议、URI、客户端格式、配置导入格式或其他运行时兼容路径。
- 不改写 `docs/reports/` 和第三期安全审查的历史证据。

前置复核结果：工作区启动时干净；当前迁移仍为 27 个 SQL、611 行、最高版本 1022；最终对象仍为 34 张表和 25 个显式索引；当前无其他活跃 Build。没有发现需要重新决策的 schema 漂移。

## 三、串行实施与验收

### Step 0.5：实施启动与范围冻结

- 建立本 Build 并登记 `AGENTS.md` 当前入口。
- 只确认持久化目标边界，不执行任何数据删除。
- 验收：迁移计数、对象计数、授权层级和排除项均与定稿一致。

### Step 1：冻结旧链 manifest

- 在临时目录通过当前 `migrations.FS` 创建旧链数据库。
- 稳定采集表、列、外键、索引、约束 SQL、种子和非种子行数。
- 验证 `foreign_key_check` 为空、`integrity_check=ok`。
- 验收：34 表、25 个显式索引、9 个代理组种子；不得出现其他数据或敏感值。

### Step 2：隔离候选基线

- 在 `backend/internal/store/testdata/0001_initial_schema.sql` 编写直接建立最终结构的候选。
- 禁止 `ALTER TABLE`、历史回填、临时表、旧 ID 保留和 OIDC 旧行处理。
- 验收：空库单事务成功应用，结构与种子数量正确。

### Step 3：双库等价验证

- A 库应用旧链，B 库应用候选基线。
- 比较列、默认值、主键、外键、索引、CHECK/UNIQUE 行为、种子、空表和完整性。
- 允许差异仅限迁移版本记录、直接建表 SQL 文本布局和被删除的旧素材池序列历史。
- 验收：语义 manifest 与约束探针无额外差异；否则停止在旧链状态。

### Step 4：替换生产迁移链

- 候选验证通过后，删除 27 个旧 SQL，新增 `backend/migrations/0001_initial_schema.sql`，保留 `embed.go`。
- 只更新 `store.go` 中点名旧基线文件的注释，不改变生产迁移逻辑。
- 验收：迁移目录仅含基线 SQL 与 `embed.go`，正式嵌入 FS 通过等价测试和 store 测试。

### Step 5：测试替换

- 删除 `migration_1016_test.go`、`migration_1018_test.go`～`migration_1021_test.go` 和 `migration_helpers_test.go`。
- 新增首版合同测试：空库、幂等、重开、manifest、种子、事务回滚、测试专用 `0002`、高版本拒绝、完整性和序列起点。
- 保留使用最小 `fstest.MapFS` 的模块隔离测试。
- 验收：store 测试不再依赖真实 1015～1021 升级链，覆盖未来迁移机制。

### Step 6：临时空库 smoke

- 因数据删除未授权，不操作 `backend/data`、卷、外部路径或备份。
- 使用 `t.TempDir()`／独立临时目录验证真实 `migrations.FS`、Setup 种子和重开幂等。
- Docker 只可使用全新、明确隔离的临时卷；若无法无副作用验证则记录未执行。
- 验收：临时空库进入版本 1，Setup 后 1 个默认组和 3 个默认平台且不重复。

### Step 7：联合门禁

```bash
cd backend
go test ./...
go test -race ./...
go build ./...
go vet ./...
go run ./cmd/errgate ./...

cd ../frontend
npm test
npm run build

cd ..
git diff --check
```

- 另查旧迁移文件名、历史 helper 和迁移目录最终内容。
- 验收：全部命令通过；自动化与隔离 smoke 不冒充真实服务或人工验收。

### Step 8：同步与归档

- 更新本文档、`V4UpgradeRelated.md`、`AGENTS.md` 和 `Issue18.md` 的当前事实。
- 完成后已将本文档归档至 `docs/reports/Build/Build31.md` 并更新链接。
- 验收：记录实际证据、未执行项、旧数据库不兼容边界和未触碰的数据目标。

## 四、实际实施与验收证据

- 旧链与隔离候选双库语义对比通过：34 张表、25 个显式索引、全部列序／类型／NULL／默认值／主键／外键、自动索引、9 个代理组种子和非种子空表一致。
- 22 组约束探针覆盖 users、OIDC、versions、share、nodes、proxy group、Xray、assembly、rule pool、mail result 及主要唯一／部分索引行为，旧链与候选结果一致。
- 生产迁移目录最终仅含 `0001_initial_schema.sql` 与 `embed.go`；迁移器生产逻辑未改变，只有旧文件名注释更新。
- 删除 6 个历史升级专项测试文件，新增首版 schema digest 合同、约束探针、同实例／重开幂等、失败回滚、测试专用 `0002`、序列起点和正式基线 Quick Start smoke。
- `go test ./... -count=1` 首轮通过；后续包级并发复跑曾因既有 `<500ms` 墙钟性能测试受机器负载影响波动，最终以 `go test -p 1 ./... -count=1` 串行全仓复核通过。`go build ./...`、`go vet ./...`、`go run ./cmd/errgate ./...` 通过。
- `go test -race ./...` 的功能／竞态部分通过；仓库既有三项 `<500ms` 墙钟性能测试在 race 插桩下超时，因此另以普通全量测试证明性能门槛，并以 `-skip` 仅排除这三项后完成全仓 race。未发现 data race。
- 前端 `npm test -- --run` 通过（47 个文件、314 项），`npm run build` 通过。
- `docker compose build` 通过；使用新建的独立临时卷启动生产镜像，`/health` 返回 ok，`/api/system/status` 返回 `configured=false`，日志只应用 `0001_initial_schema.sql` version 1。临时容器和卷已清理。
- `PRAGMA foreign_key_check` 无结果，`PRAGMA integrity_check` 返回 `ok`；`git diff --check` 通过。
- 未删除或修改 `backend/data`、项目 Compose `vpn-data`、外部 `DATA_DIR` 或任何备份；未执行真实 SMTP、真实 OIDC、真实客户端或浏览器人工验收。

## 五、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-20 | 根据用户实施授权建立；Step 0.5 完成，开始冻结旧链 manifest。 |
| v1.1 | 2026-09-20 | Step 0.5～8 完成：基线等价替换、合同测试、联合门禁和独立 Docker 空卷 smoke 通过；既有数据目标未触碰，Build 归档。 |
| v1.2 | 2026-09-20 | 关联 V4UpgradeRelated.md 已归档至 `docs/reports/Others/`；Build31 继续保留在 `docs/reports/Build/` 作为历史构建记录。 |
