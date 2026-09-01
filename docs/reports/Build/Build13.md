# Build13.md — R24 已决策问题修复构建记录

> **文档定位：** 本文记录 `Issue9.md` 中已由用户确认的 R24-02、R24-12、R24-15 修复及验收结果；不扩大到其他待决策项。
> - 当前设计：`Design2.md`、`Design2-UI.md`
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 问题追踪：[Issue9.md](Issue9.md)
> - 前一轮构建：[Build12.md](Build12.md)
> - 实施日期：2026-08-28

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 1 | R24-02：首页桌面端摘要区并列布局 | ✅ 验收通过 |
| 2 | R24-12：素材池条目按来源筛选与 URL 区懒加载 | ✅ 验收通过 |
| 3 | R24-15：头部 bool 参数独立开关区域 | ✅ 验收通过 |
| 4 | 定向测试、全量前端验证与文档同步 | ✅ 已完成 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、范围与未处理项

本构建只实施用户已决策的三项：R24-02、R24-12、R24-15。

以下问题明确不在本次改动范围内：

- R24-01：SQLite 单连接嵌套查询死锁，需要更多分析。
- R24-18、R24-19、R24-20：需要更多分析。
- Issue9 中其余尚未决策或未授权实施的问题。

---

## 三、实施内容

### Step 1：R24-02 首页响应式摘要区

- **目标：** 让流量卡片与分流规则卡片在桌面端并列，减少信息密度偏低造成的首屏纵向空间占用。
- **改动文件：**
  - `frontend/src/views/HomeView.vue`
  - `frontend/tests/home-view.spec.ts`
- **实现：** 两张摘要卡片包裹在 `grid grid-cols-1 md:grid-cols-2` 中；移动端纵向堆叠，平台卡片网格与公告栏保持原顺序和数据行为。
- **验收：** `frontend/tests/home-view.spec.ts` 共 5 项通过。

### Step 2：R24-12 素材池 URL 条目折叠与按来源分页

- **目标：** 素材池详情默认只查询手动条目；URL 同步条目区域默认折叠，在用户首次展开时才请求对应来源的数据。
- **改动文件：**
  - `backend/internal/pool/pool.go`
  - `backend/internal/pool/pool_test.go`
  - `backend/internal/server/pool.go`
  - `frontend/src/api/pool.ts`
  - `frontend/src/views/admin/assembly/PoolDetail.vue`
  - `frontend/tests/pool-detail.spec.ts`
- **实现：**
  - 既有 `GET /api/admin/pools/:id/entries` 新增可选 `source=manual|url`；未传时保持全来源列表的兼容语义。
  - 来源条件同时用于 `COUNT(*)` 和列表查询，保证来源区各自的总数与分页准确。
  - 前端手动区和 URL 区分别维护加载、页码、总数与错误状态；URL 区首次展开后加载并缓存成功结果，失败时可重试。
- **验收：** `go test ./internal/pool` 通过；`frontend/tests/pool-detail.spec.ts` 共 3 项通过。

### Step 3：R24-15 头部 bool 参数开关分区

- **目标：** 将 bool 型头部字段从输入网格中分离，形成统一的“开关参数”区域。
- **改动文件：**
  - `frontend/src/views/admin/assembly/HeaderStep.vue`
  - `frontend/tests/header-step.spec.ts`
- **实现：** 按字段类型将输入型字段和 bool 字段拆分渲染；Clash YAML 的 `Allow LAN` 与 SR 分流规则的 `IPv6` 统一进入“开关参数”区域。`fixed_params_text` 的键名、默认值、JSON 序列化和生成接口均未改动。
- **验收：** `frontend/tests/header-step.spec.ts` 共 2 项通过。

---

## 四、验证记录

| 范围 | 命令 | 结果 |
|------|------|------|
| 首页定向测试 | `cd frontend && npm test -- --run tests/home-view.spec.ts` | 通过，5 项 |
| 素材池详情定向测试 | `cd frontend && npm test -- --run tests/pool-detail.spec.ts` | 通过，3 项 |
| 头部参数定向测试 | `cd frontend && npm test -- --run tests/header-step.spec.ts` | 通过，2 项 |
| 前端全量测试 | `cd frontend && npm test -- --run` | 通过，31 个测试文件、107 项测试 |
| 前端构建 | `cd frontend && npm run build` | 通过 |
| 后端素材池定向测试 | `cd backend && go test ./internal/pool` | 通过 |
| 后端编译与静态检查 | `cd backend && go build ./... && go vet ./...` | 通过 |

### 已知未决阻塞

`cd backend && go test -timeout 20s ./internal/server -run '^TestOverviewEndpointAggregatesStableData$' -count=1` 在 20 秒超时。堆栈显示 `ExtService.ListExt` 持有 `pool_entries` 查询结果时继续调用 `pushTargetsFor`，后者等待唯一 SQLite 连接；该现象与 R24-01 的既有记录一致。本轮按用户决定不处理 R24-01，因此不宣称 `go test ./...` 已通过。

---

## 五、文档同步

- `Design2-UI.md` 已包含用户确认的桌面摘要区、URL 区懒加载/来源分页、bool 开关分区规格。
- `Issue9.md` 已将 R24-02、R24-12、R24-15 更新为已实施并补充验收结果。
- R24-01、R24-18、R24-19、R24-20 继续保留“需要更多分析”状态，未修改相关业务代码。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-28 | 实施并验收 R24-02、R24-12、R24-15；记录验证结果与未决 R24-01 对全量后端测试的既有阻塞。 |
