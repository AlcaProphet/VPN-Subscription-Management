# VPN 订阅管理系统 功能构建计划（Build22：当前构建方案）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前构建方案**（依据 AGENTS.md：Build 文档为详细构建方案，非强规则），承接已完成的 [Build17.md](Build17.md)～[Build21.md](Build21.md)；本轮针对 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 的**未闭环项 1** 进行深入研究并制定修复计划。
> - 设计记录：[Design3.md](Design3.md)（Build16 的目标设计，当前仍有效）、[Build16.md](docs/reports/Build/Build16.md)（原构建计划）
> - 问题来源：[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md)（全量核验报告，未闭环项 1）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 历史构建与问题记录：见 [docs/reports/](docs/reports/)（均已存档，仅核查）
>
> **用户已确认的决策：**
> 1. Build22.md 放仓库根目录，作为当前构建方案。
> 2. 研究范围覆盖 BuildReport4 §4.2 的 **D3-1～D3-10** 全部缺口。
> 3. 文档采用“研究依据 + 可执行 Build 计划”形态，按本模板重新排版。
> 4. per-URL failed 状态采用**持久化 failed 快照行**，不只在任务 JSON 中临时记录。
> 5. 来源原始证据**暂不新增 `origin_path` 列**，使用现有 `line_no`、`raw_line`；JSON/YAML 路径信息放入 `raw_line` 或诊断说明。
>
> **执行原则（与 Build17～Build21 一致）：**
> - 每一步完成后均可编译、可测试。不跳步、不并行多步。
> - AI 执行指令：每次仅执行一个 Step，完成后运行验收命令，确认通过后再进入下一步。
> - **排序原则：先修复后构建、先安全后优化、先依赖后独立**。
> - 每步的新增逻辑必须配套单元测试；测试应先复现缺口，再修改实现。
> - 本文档当前仅完成研究、方案与排版，**未修改任何业务代码**。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 修复来源统计计数（D3-2） | Design3 §5.3、§6.4 | ☐ 未开始 |
| 2 | 来源原始证据采集、排序与装配去重（D3-5） | Design3 §3.2、§5.4、§6.1 | ☐ 未开始 |
| 3 | `no_resolve` 实例语义贯通（D3-1） | Design3 §7.2 | ☐ 未开始 |
| 4 | 后端素材池能力白名单（D3-4） | Design3 §3.3、§3.4、§8.3 | ☐ 未开始 |
| 5 | 手工编辑不污染共享 Canonical（D3-3） | Design3 §3.2、§6.1 | ☐ 未开始 |
| 6 | 零输出门槛补全（D3-6） | Design3 §7.2 | ☐ 未开始 |
| 7 | failed 快照持久化 + per-URL 状态/诊断 API（D3-7） | Design3 §6.4、§8.2、§8.3 | ☐ 未开始 |
| 8 | 前端来源状态、诊断与 pending 操作（D3-7 UI、D3-8） | Design3 §8.2 | ☐ 未开始 |
| 9 | 装配回执前端展示（D3-9） | Design3 §7.2、§8.2 | ☐ 未开始 |
| 10 | 1016 迁移 store 级回归测试（D3-10） | Design3 §6.5、§9.3 | ☐ 未开始 |
| 11 | 全量回归、文档同步与 Build16/Design3 状态收口 | AGENTS.md §3.4～§3.6 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。
> 当前没有进行中的构建 Step；所有 Step 均待按本文档逐步执行。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/pool/pipeline.go`、`backend/internal/pool/parser_test.go` | 修正 `Excluded` 重复累加；补充统计回归测试 |
| 2 | `backend/internal/pool/types.go`、`adapter_*.go`、`pipeline.go`、`sync.go`、`pool.go`、`backend/internal/assembly/load.go` 及相关测试 | 新增 RuleOriginMeta；真实行号/原始行/来源内顺序落库；查询按 origin 排序并按 canonical 去重 |
| 3 | `backend/internal/assembly/load.go`、`render_clash.go`、`render_sr.go` 及相关测试 | 装配层保留 `no_resolve`；只在源实际设置且目标支持时输出 |
| 4 | `backend/internal/rulespec/capability.go` 或新增 helper、`backend/internal/pool/pool.go`、`pool_test.go` | 后端强制 `MaterialPool` 白名单，禁止 advanced-only 类型进入素材池 |
| 5 | `backend/internal/pool/pool.go`、`pool_test.go` | 手工编辑改为换绑 canonical origin，不直接修改共享 canonical 行 |
| 6 | `backend/internal/server/assembly.go`、`server/assembly_test.go` | 规则型目标无条件执行 `FinalOutput==0` 禁止生成 |
| 7 | `backend/internal/pool/sync.go`、新增 `snapshot.go`（或同类文件）、`backend/internal/server/pool.go`、`frontend/src/api/pool.ts`、后端/前端测试 | 失败时写入 failed snapshot；增加来源状态与快照列表 API |
| 8 | `frontend/src/views/admin/assembly/PoolDetail.vue`、`frontend/tests/pool-detail.spec.ts` | 每 URL 展示状态/统计/诊断；pending 激活与丢弃 |
| 9 | `frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/assembly/PreviewStep.vue`、`frontend/tests/assembly-view.spec.ts`、`preview-step.spec.ts` | 保存并渲染装配转换回执 |
| 10 | 新增 `backend/internal/store/migration_1016_test.go` 或 `backend/internal/pool/migration_1016_test.go` | 验证旧数据清除、ID 防复用、无关历史保留 |
| 11 | `Build16.md`、`Design3.md`、`AGENTS.md`、本文件 | 全量验证与文档状态收口 |

---

## 三、构建顺序依赖图

```text
Step 1（统计修正）      Step 4（白名单）      Step 10（迁移测试）
        │                      │                      │
        ▼                      ▼                      │
Step 2（origin/排序/去重）→ Step 5（手工换绑）            │
        │                                              │
        ▼                                              │
Step 3（no_resolve 贯通）                               │
        │                                              │
        ▼                                              │
Step 6（零输出门槛）                                    │
        │                                              │
        ▼                                              │
Step 7（failed 快照 + per-URL API）                     │
        │                                              │
        ▼                                              │
Step 8（前端来源状态/pending）                          │
        │                                              │
        ▼                                              │
Step 9（装配回执展示）                                  │
        │                                              │
        ▼                                              │
Step 11（全量回归/文档收口） ←──────────────────────────┘
```

依赖说明：

- Step 1 先修正统计，避免后续 origin/receipt 相关测试被错误计数干扰。
- Step 2 产出真实 origin 数据，是 Step 3 与最终排序/去重的基础。
- Step 3 依赖 Step 2 的 Canonical 携带能力。
- Step 4 与 Step 5 可并行，但 Step 5 复用 Step 4 的白名单校验更稳妥。
- Step 6 依赖 Step 2/3 后的 receipt 可正确区分真实规则与内置兜底。
- Step 7 是 Step 8 的前置。
- Step 10 独立，可先做，但建议在 Step 11 前完成。
- Step 11 在全部 Step 通过后执行，只做验证与文档同步，不新增功能。

---

## 四、分步构建计划

### Step 1：修复来源统计计数（D3-2）

- **背景/根因：**
  `pool/pipeline.go finalizeParseResult()` 在循环中已对“因来源模式被排除”的规则执行 `res.Excluded++`，循环后又执行 `res.Excluded += len(rules) - res.Accepted - res.Rejected`。后者把已排除项再累加一次，同时把重复项也计入 excluded，导致同步回执与前端统计失真。

- **目标：** 让 `ParseResult.Excluded` 只统计“合法但被来源模式剔除”的规则，`Duplicates` 单列，不混入 excluded。

- **前置条件：** 无。

- **产出文件与操作：**
  - `backend/internal/pool/pipeline.go`：
    - 删除循环后的 `res.Excluded += len(rules) - res.Accepted - res.Rejected`；
    - 在循环内 Clash/SR 模式排除时精确 `res.Excluded++`；
    - 保持 `res.Duplicates++` 独立。
  - `backend/internal/pool/parser_test.go`：新增表驱动用例，覆盖普通排除、重复+排除、auto 模式不产生被来源模式排除等场景。

- **参考伪代码：**
  ```go
  // 循环内
  if mode == SourceModeClash && !supportsTarget(rule, rulespec.TargetClash) {
      res.Excluded++
      continue
  }
  if mode == SourceModeShadowrocket && !supportsTarget(rule, rulespec.TargetSR) {
      res.Excluded++
      continue
  }
  // 删除下面的重复累加：
  // res.Excluded += len(rules) - res.Accepted - res.Rejected
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/pool -run 'TestParseSource|TestFinalize'
  cd backend && go build ./...
  ```

- **验收标准：**
  Clash/SR 模式下 excluded 与 duplicates 互不污染；auto 模式不出现来源模式排除；现有解析测试全部通过。

---

### Step 2：来源原始证据采集、排序与装配去重（D3-5）

- **背景/根因：**
  - 各 adapter 只返回 `[]rulespec.CanonicalRule`，接受的规则没有携带行号、原始行、来源内顺序。
  - `sync.go applyParseResultTx()` 写入 `sort_order=int64(len(parsed.Rules))`、`raw_line=rule.SemanticKey()`、`line_no=0`。
  - `ListEntries()` 与 `assembly/load.go loadPoolEntries()` 只按 `src.sort_order, cr.id` 排序，未使用 `pool_rule_origins` 的真实顺序。
  - `loadPoolEntries()` 未按 canonical 去重，多个 active origin 指向同一 canonical 时可能重复渲染。

- **目标：** 让 accepted 规则携带完整来源证据并正确落库；列表与装配按“手工在前 → URL 配置顺序 → 来源内原始顺序”稳定输出，且同一 canonical 只渲染一次。

- **前置条件：** Step 1 通过。

- **产出文件与操作：**
  - `backend/internal/pool/types.go`：
    - 新增：
      ```go
      type RuleOriginMeta struct {
          Line  int    `json:"line"`
          Raw   string `json:"raw"`
          Order int    `json:"order"`
      }
      ```
    - `ParseResult` 增加 `Origins []RuleOriginMeta`，与 `Rules` 一一对应。
  - `adapter_legacy.go`、`adapter_typed.go`、`adapter_mihomo.go`、`adapter_ip.go`、`adapter_singbox.go`：
    - 每个 accepted rule 记录 Line/Raw/Order。
    - 对纯文本适配器，Line 为真实 1-based 行号，Raw 为原始行。
    - 对 Mihomo YAML，Line 可为 0，Order 为 `payload` 内索引，Raw 保存原始条目；如后续需要可在适配器内使用 YAML 节点行号增强。
    - 对 sing-box JSON，Line 可为 0，Order 使用 `ruleIndex*1000+valueIndex` 之类稳定序号，Raw 保存 JSON 路径如 `rules[0].domain[1]`。
  - `pool/pipeline.go finalizeParseResult()`：
    - 去重时保留首个 accepted origin；重复项只计数，不新增 origin。
  - `pool/sync.go applyParseResultTx()`：
    - `sort_order` 写 `origin.Order`；
    - `line_no` 写 `origin.Line`；
    - `raw_line` 写 `origin.Raw`。
  - `pool/pool.go ListEntries()`：
    - 查询增加 `o.sort_order, o.line_no, o.raw_line`；
    - 排序改为 `CASE WHEN src.kind='manual' THEN 0 ELSE 1 END, src.sort_order, o.sort_order, o.line_no, o.id`；
    - 保留现有 Go 侧 canonical 去重，并确保分页前先压制重复（推荐使用子查询/窗口方式选定每个 canonical 的最早 origin 后再 LIMIT）。
  - `backend/internal/assembly/load.go loadPoolEntries()`：
    - `poolEntry` 增加 `NoResolve bool`、`Canonical rulespec.CanonicalRule`、`OriginOrder`、`OriginLine`、`RawLine` 等字段（按 Step 3 需要）；
    - 查询增加 origin 字段并按上述顺序排序；
    - Go 侧按 canonical id 去重，保留最早 origin。

- **参考数据写入伪代码：**
  ```go
  for i, pr := range parsed.Items { // pr 为 rule + origin
      canonicalID, err := ensureCanonicalTx(ctx, tx, poolID, pr.Rule)
      if err != nil { return err }
      if _, err := tx.ExecContext(ctx,
          `INSERT INTO pool_rule_origins
             (pool_id, canonical_rule_id, source_id, snapshot_id, sort_order, raw_line, line_no)
           VALUES (?,?,?,?,?,?,?)`,
          poolID, canonicalID, sourceID, snapshotID,
          pr.Origin.Order, pr.Origin.Raw, pr.Origin.Line); err != nil {
          return err
      }
  }
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/pool ./internal/assembly ./internal/server
  cd backend && go build ./...
  ```

- **验收标准：**
  `pool_rule_origins.sort_order/line_no/raw_line` 不再全部是占位值；多来源同语义只渲染一次；手工在前、URL 按配置顺序、来源内按原始顺序；ListEntries 分页不因重复 origin 少条。

---

### Step 3：`no_resolve` 实例语义贯通（D3-1）

- **背景/根因：**
  `loadPoolEntries()` 读取 `options_json` 后仅构造 `poolEntry{RuleType, MatchValue}`，丢失 `Options.NoResolve`。渲染层随后依赖 `mapped.SupportsNoResolve`，只要类型支持就追加 `no-resolve`。这违反 Design3 “仅当规则实际设置且目标支持时输出”。

- **目标：** 装配层保留规则实例的 `no_resolve` 选项，并据此决定 Clash/SR 输出。

- **前置条件：** Step 2 通过（装配层已携带 Canonical 能力）。

- **产出文件与操作：**
  - `backend/internal/assembly/load.go`：
    - `poolEntry` 明确保存 `NoResolve bool` 或完整 `Canonical rulespec.CanonicalRule`。
  - `backend/internal/assembly/render_clash.go`：
    - `appendRule` 增加 `noResolve bool`；
    - 仅在 `noResolve && mapped.SupportsNoResolve` 时追加 `,no-resolve`。
  - `backend/internal/assembly/render_sr.go`：
    - `formatRuleLine` 增加 `noResolve bool`，同样按 `mapped.SupportsNoResolve` 判定。
  - 相关测试：
    - 池内 `IP-CIDR` 未写 no-resolve → 不输出；
    - 池内 `IP-CIDR` 写了 no-resolve → 输出；
    - 自定义规则默认不继承 no-resolve（本轮不新增自定义规则 no-resolve 输入）；
    - SR 侧同样覆盖。

- **参考伪代码：**
  ```go
  if noResolve && mapped.SupportsNoResolve {
      line += ",no-resolve"
  }
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/assembly ./internal/server
  cd backend && go build ./...
  ```

- **验收标准：**
  Clash YAML 与 SR 分流规则中的 `no-resolve` 仅出现在源实际设置且目标支持的行；既有默认“IP 类型必加 no-resolve”的测试期望改为按实例断言。

---

### Step 4：后端素材池能力白名单（D3-4）

- **背景/根因：**
  `pool.CanonicalFromLegacyInput()` 只做 `ValidateValue` 和 `CanonicalizeLegacyType`，未检查 `MaterialPool`。前端下拉虽已过滤，但直接调用后端 API 可以创建 `RULE-SET`、`AND`、`OR`、`NOT`、`MATCH` 等 advanced-only 素材。

- **目标：** 后端成为素材池可选能力的最终约束。

- **前置条件：** 无。

- **产出文件与操作：**
  - `backend/internal/rulespec/`：
    - 新增可复用 helper：
      ```go
      func IsMaterialPool(rule CanonicalRule) bool
      ```
    - 实现在 `capability.go` 中，基于 `capabilityRegistry` 或 `findCapability` 判定 `MaterialPool`。
  - `backend/internal/pool/pool.go`：
    - `canonicalFromLegacyInput()` 或 `CreateEntry/UpdateEntry` 公共入口调用 `rulespec.IsMaterialPool(rule)`；
    - 非素材池能力返回 `ErrBadRequest`，错误信息明确“不是素材池可选能力”。
  - `pool_test.go`：新增反例：`RULE-SET`、`AND`、`OR`、`NOT`、`MATCH`、`GEOSITE` 等创建/更新均拒绝；正例：`DOMAIN`、`DOMAIN-SUFFIX`、`IP-CIDR`、`USER-AGENT` 等允许。

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/rulespec ./internal/pool
  cd backend && go build ./...
  ```

- **验收标准：**
  后端能力、解析器准入、前端下拉与手工 CRUD 共用同一 `MaterialPool` 事实来源；任何 advanced-only 规则不能进入素材池。

---

### Step 5：手工编辑不污染共享 Canonical（D3-3）

- **背景/根因：**
  `UpdateEntry()` 直接 `UPDATE pool_canonical_rules`。由于 `pool_canonical_rules` 对 `pool_id + semantic_key` 唯一，手工 origin 与 URL origin 可能共用同一 canonical 行；直接修改会改变 URL 派生规则。

- **目标：** 手工编辑只改动手工 origin 的绑定关系，不修改共享 canonical 行。

- **前置条件：** Step 4 通过（复用白名单校验）。

- **产出文件与操作：**
  - `backend/internal/pool/pool.go UpdateEntry()`：
    1. 事务内定位手工 origin（`o.snapshot_id IS NULL AND src.kind='manual'`）；
    2. 若新语义与旧 canonical 相同，直接返回；
    3. 对新 canonical 执行 `ensureCanonicalTx()`（已存在则复用）；
    4. 更新该 origin 的 `canonical_rule_id` 和 `raw_line`；
    5. 调用 `cleanupOrphanCanonicalTx()` 清理旧 canonical 孤儿。
  - 不修改 shared canonical 行的 `family/matcher/value/options_json/semantic_key`。
  - `pool_test.go`：新增“手工与 URL 同语义，手工修改后 URL 仍保持旧值；旧 canonical 无有效 origin 时被清理”的回归。

- **参考伪代码：**
  ```go
  newCanonicalID, err := ensureCanonicalTx(ctx, tx, poolID, canonical)
  if err != nil { return err }
  if _, err := tx.ExecContext(ctx,
      `UPDATE pool_rule_origins SET canonical_rule_id=?, raw_line=? WHERE id=?`,
      newCanonicalID, ruleType+","+value, manualOriginID); err != nil {
      return err
  }
  return s.cleanupOrphanCanonicalTx(ctx, tx, poolID)
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/pool ./internal/assembly
  cd backend && go build ./...
  ```

- **验收标准：**
  手工规则更新只影响该手工来源；同语义 URL 规则保持原值；无孤儿 canonical 残留。

---

### Step 6：零输出门槛补全（D3-6）

- **背景/根因：**
  `server/assembly.go generate()` 只在 `len(in.Pools)>0 || len(in.CustomRules)>0` 时检查 `FinalOutput==0`。用户不选任何素材/自定义规则时，仍可仅依赖内置 `GEOIP`/`MATCH`/`FINAL` 生成。

- **目标：** 对 Clash YAML 与 SR 分流规则，最终非系统规则输出必须 >= 1；否则禁止生成。

- **前置条件：** Steps 2～3 通过（receipt 能正确区分真实规则与系统兜底）。

- **产出文件与操作：**
  - `backend/internal/server/assembly.go generate()`：
    - 删除 `len(in.Pools)>0 || len(in.CustomRules)>0` 条件；
    - 对 `clash-yaml`、`sr-conf` 检查 `res.Receipt != nil && res.Receipt.FinalOutput == 0`，命中即返回 400。
  - `server/assembly_test.go`：
    - 无池无自定义 → 拒绝；
    - 只有不支持的自定义规则 → 拒绝；
    - 至少一条有效素材或自定义规则 → 通过；
    - `sr-subs/generic-subs` 不适用该门槛。

- **参考伪代码：**
  ```go
  if res.Receipt != nil && res.Receipt.FinalOutput == 0 {
      Fail(c, http.StatusBadRequest, "当前目标没有可输出的非系统规则")
      return
  }
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/server ./internal/assembly
  cd backend && go build ./...
  ```

- **验收标准：**
  空素材/空自定义不能生成 Clash YAML 或 SR conf；有效的单条规则可生成；预览仍可返回 receipt。

---

### Step 7：failed 快照持久化 + per-URL 状态/诊断 API（D3-7）

- **背景/根因：**
  - `pool_source_snapshots` 有 `status='failed'`，但 `syncOne()` 在 HTTP/解析失败时直接返回，不写 failed 行。
  - `PoolSource` 仅回传 active/pending ID；`server/pool.go` 没有读取快照的服务与路由。
  - 前端无法展示格式、平台、统计、诊断、failed/待同步等来源状态。

- **目标：** 持久化每次失败的来源尝试，并提供 per-URL 当前状态与快照历史 API。

- **前置条件：** Steps 2～3 通过（统计与 origin 已修正）。

- **产出文件与操作：**
  - `backend/internal/pool/sync.go`：
    - 新增 `recordFailedSnapshotTx(ctx, tx, poolID, sourceID, errMsg)`；
    - 写入 `pool_source_snapshots(status='failed', format='', profile='', counts=0, diagnostic_json=[{kind:"error", message:...}], stats_json={error:...})`；
    - 不修改 active/pending 指针。
  - `syncOne()`：
    - HTTP 状态码错误、超时、读取超限、解析错误等失败路径均调用上述方法。
  - 新增 `backend/internal/pool/snapshot.go`（或同类文件）：
    - `SourceSnapshot` 模型：ID、source_id、format、profile、status、各类计数、`Diagnostics`、`Stats`、`Error`、CreatedAt；
    - `SourceStatus` 模型：source_id、url、source_mode、active、pending、latest_failed、never_synced 标记；
    - `ListSourceStatuses(ctx, poolID)`；
    - `ListSourceSnapshots(ctx, poolID, sourceID, page, pageSize)`。
  - `backend/internal/server/pool.go`：
    - 新增路由：
      ```
      GET /api/admin/pools/:id/sources/status
      GET /api/admin/pools/:id/sources/:sourceId/snapshots
      ```
    - 路由继续叠加 session + admin 双中间件。
  - `frontend/src/api/pool.ts`：
    - 新增 `SourceSnapshot`、`SourceStatus` 类型与 `listSourceStatuses`、`listSourceSnapshots` 请求函数。
  - 清理策略：
    - 在 `CleanupOldTasks()` 或新增清理逻辑中，同时清理超过 7 天的 failed 快照；active/pending 快照和仍被 active/pending 引用的快照不得删除。
  - 测试：
    - HTTP 失败生成 failed snapshot；
    - 解析失败生成 failed snapshot；
    - active/pending 不变；
    - 状态 API 返回 active/pending/latest_failed；
    - 快照历史分页正确；
    - 7 天清理不删 active/pending。

- **参考 API 响应：**
  ```json
  {
    "source_id": 1,
    "url": "https://example.com/rules.txt",
    "source_mode": "auto",
    "active": {
      "id": 10,
      "format": "typed-rule-text",
      "profile": "common",
      "accepted": 3,
      "excluded": 1,
      "rejected": 0,
      "duplicates": 0
    },
    "pending": {
      "id": 11,
      "format": "mihomo-domain-yaml",
      "profile": "common",
      "accepted": 50
    },
    "latest_failed": {
      "id": 12,
      "error": "HTTP 500"
    }
  }
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/pool ./internal/server
  cd backend && go build ./...
  cd frontend && npm run build
  ```

- **验收标准：**
  失败可持久化并可查询；来源状态 API 能区分 active/pending/failed/从未同步；failed 快照不会进入 active 查询；清理不会误删有效快照。

---

### Step 8：前端来源状态、诊断与 pending 操作（D3-7 UI、D3-8）

- **背景/根因：**
  `frontend/src/api/pool.ts` 已有 `activatePending/discardPending`，但没有任何页面调用；`PoolDetail.vue` 只显示“URL 数量”与同步历史摘要，不展示每个 URL 的格式、平台、状态、统计和诊断。

- **目标：** 在素材池详情中增加每个 URL 的来源状态卡片，并支持 pending 激活/丢弃。

- **前置条件：** Step 7 通过。

- **产出文件与操作：**
  - `frontend/src/views/admin/assembly/PoolDetail.vue`：
    - onMounted 与同步完成后调用 `listSourceStatuses`；
    - 每个 URL 展示：URL、来源模式、检测格式/平台、active/pending/failed/待同步徽标、接受/排除/拒绝/重复统计、诊断摘要；
    - `pending_snapshot_id` 存在时显示“激活/丢弃”按钮；
    - 激活前 ConfirmModal 展示旧 active 与新 pending 的数量、格式、平台差异；
    - 调用 `activatePending/discardPending` 后刷新来源状态与条目。
  - `frontend/src/api/pool.ts`：
    - 复用 Step 7 新增类型。
  - `frontend/tests/pool-detail.spec.ts`：
    - mock `listSourceStatuses`、`activatePending`、`discardPending`；
    - 覆盖 pending 展示、激活确认、丢弃、同步后刷新。

- **测试与验收命令：**
  ```bash
  cd frontend && npm test -- --run tests/pool-detail.spec.ts
  cd frontend && npm run build
  ```

- **验收标准：**
  每个 URL 的状态可见；pending 可人工激活/丢弃；激活前能看到旧/新差异；桌面与 <768px 窄屏均可操作。

---

### Step 9：装配回执前端展示（D3-9）

- **背景/根因：**
  后端 preview 已返回 `receipt`，`frontend/src/api/assembly.ts` 也已有 `ConversionReceipt` 类型；但 `AssemblyView.vue` 未保存 `res.receipt`，`PreviewStep.vue` 也未接收该 prop，因此用户看不到回执。

- **目标：** 在预览步骤展示输入数、直接输出、等价转换、目标不支持跳过、目标校验失败、最终输出。

- **前置条件：** Step 6 通过（回执统计口径已正确）。

- **产出文件与操作：**
  - `frontend/src/views/admin/AssemblyView.vue`：
    - 新增 `previewReceipt` ref；
    - `doPreview()` 中 `previewReceipt.value = res.receipt ?? null`；
    - 将 `previewReceipt` 传给 `PreviewStep`。
  - `frontend/src/views/admin/assembly/PreviewStep.vue`：
    - 新增 `receipt?: ConversionReceipt | null` prop；
    - 在警告/跳过区域附近渲染回执摘要卡或列表。
  - 可选：
    - generate 响应也返回 `receipt`，生成成功结果页同步展示。
  - `frontend/tests/assembly-view.spec.ts`、`frontend/tests/preview-step.spec.ts`：
    - 验证保存、传递与渲染数字。

- **参考 UI 文案：**
  ```text
  转换回执：输入 N · 直接输出 N · 等价转换 N · 目标不支持跳过 N · 校验失败 N · 最终输出 N
  ```

- **测试与验收命令：**
  ```bash
  cd frontend && npm test -- --run tests/assembly-view.spec.ts tests/preview-step.spec.ts
  cd frontend && npm run build
  ```

- **验收标准：**
  用户能看到完整转换回执；数字与后端 `receipt` 一致；已有预览/差异功能无回归。

---

### Step 10：1016 迁移 store 级回归测试（D3-10）

- **背景/根因：**
  现有测试只验证“全新迁移后旧表不存在”，未验证旧数据清除、旧 ID 不复用、无关历史数据保留。

- **目标：** 用迁移级测试保护 1016 不兼容迁移语义。

- **前置条件：** 无。

- **产出文件与操作：**
  - 新增 `backend/internal/store/migration_1016_test.go` 或 `backend/internal/pool/migration_1016_test.go`。
  - 测试流程：
    1. 构造包含 `0001_init.sql`（至少含 schema_migrations）和旧 `1009` 素材池 schema 的临时 `fstest.MapFS`；
    2. `Store.Migrate` 到旧结构；
    3. 插入旧 `rule_pools`、`pool_entries`、`pool_sync_tasks` 数据；
    4. 再使用包含真实 `1016_rule_pool_snapshots.sql` 的 FS 继续 `Migrate`；
    5. 断言：
       - 旧表 `pool_entries`、旧 `pool_sync_tasks` 已删除；
       - 新表 `rule_pool_sources`、`pool_source_snapshots`、`pool_canonical_rules`、`pool_rule_origins` 存在；
       - 旧池数据不残留；
       - 新 `rule_pools.id` 从旧最大 ID 之后开始（旧 ID 不复用）；
       - 若测试 schema 包含 `versions`/`assembly_blueprints`，确认其数据保留。
  - 可选：从 `migrations.FS` 读取真实 1016 SQL，避免测试 SQL 与迁移文件漂移。

- **参考流程：**
  ```go
  // 第一次 Migrate：旧 schema
  _ = st.Migrate(ctx, oldFS)
  // 插入旧数据
  // 第二次 Migrate：包含 1016
  _ = st.Migrate(ctx, fullFS)
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/store ./internal/pool -run 'Migration|1016'
  cd backend && go build ./...
  ```

- **验收标准：**
  旧数据清除、ID 防复用、无关历史保留均有自动断言。

---

### Step 11：全量回归、文档同步与 Build16/Design3 状态收口

- **目标：** 完成全量自动验证，并按实际结果同步文档状态；本 Step 不新增功能。

- **前置条件：** Steps 1～10 全部通过。

- **产出文件与操作：**
  - 自动门禁：
    ```bash
    cd backend && go build ./...
    cd backend && go vet ./...
    cd backend && go test ./... -count=1 -timeout 180s
    cd frontend && npm run build
    cd frontend && npm test -- --run
    git diff --check
    ```
  - 文档同步：
    - `Build16.md`：修正 Step 3～6 的实际状态，不再把缺失项标记为已验收。
    - `Design3.md`：记录实现与设计的实际落点，尤其是 failed 快照持久化、来源证据存储方式和 per-URL API 形态。
    - `AGENTS.md`：仅在全部实际完成后登记 Build22。
    - 本文件：更新进度表与验收结果。

- **验收标准：**
  所有自动命令通过；文档只记录实际证据；Build16/Design3 未闭环项不虚标已闭环。

---

## 五、候选构建项（待用户决策，逐项转 Step）

> 以下候选均来自 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §4.2，并已经用户确认纳入 Build22。后续实施时按上述 Step 顺序逐项执行。

| # | 候选 | 说明 | 来源 | 对应 Step |
|---|------|------|------|-----------|
| 1 | `no_resolve` 语义丢失 | 装配层丢失 Options，渲染按类型支持度无条件追加 | Design3 §7.2 | Step 3 |
| 2 | `excluded` 重复计算 | 统计循环与后置累加导致排除数错误，且混入重复 | Design3 §5.3、§6.4 | Step 1 |
| 3 | 手工更新污染 URL Canonical | UpdateEntry 直接改共享 canonical 行 | Design3 §3.2、§6.1 | Step 5 |
| 4 | 后端素材池白名单未强制 | 只做值校验，未校验 `MaterialPool` | Design3 §3.3、§3.4 | Step 4 |
| 5 | 来源原始证据/排序未落库 | 行号/原始行/来源内顺序均为占位；查询未按 origin 排序且未去重 | Design3 §3.2、§5.4 | Step 2 |
| 6 | 零输出门槛漏网 | 无池/自定义时绕过 `FinalOutput==0` 检查 | Design3 §7.2 | Step 6 |
| 7 | per-URL 快照状态/诊断 API 缺失 | 无快照读取服务与路由；failed 未持久化 | Design3 §6.4、§8.2、§8.3 | Step 7 |
| 8 | pending 激活/丢弃无 UI | 前端已有 API 但无页面调用 | Design3 §8.2 | Step 8 |
| 9 | 装配回执未展示 | 后端返回 receipt，前端未保存/渲染 | Design3 §7.2、§8.2 | Step 9 |
| 10 | 1016 迁移测试缺失 | 只验新库无旧表，未验旧数据/ID/历史保留 | Design3 §6.5、§9.3 | Step 10 |

> 候选转 Step 流程：本文档已按用户确认转为上述 Step；后续若发现新的候选，再按同样流程追加。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-05 | 根据 BuildReport4 未闭环项 1 完成 D3-1～D3-10 根因研究、修复方向与候选清单。 |
| v1.1 | 2026-09-05 | 进一步深入研究并按照 `docs/DocTemplates/Build.template.md` 重排：新增构建进度追踪、构建概要、顺序依赖图、分步构建计划、候选构建项与变更记录；补充 failed 快照持久化、来源证据实现细节、迁移测试方法与清理策略。未修改任何业务代码。 |

---

## 附录 A：深入研究记录（代码证据与修复依据）

> 本附录用于保留详细代码证据，作为后续执行时的排查依据；不属于模板必需节。

### A.1 D3-1：`no_resolve`

当前链路：

```text
pool_canonical_rules.options_json
  → assembly/load.go loadPoolEntries()（读出 options 后丢弃）
  → poolEntry{RuleType, MatchValue}
  → render_clash.go appendRule() 在 mapped.SupportsNoResolve 时追加
  → render_sr.go formatRuleLine() 对 IP-CIDR/IP-CIDR6 无条件追加
```

关键代码位置：

- `backend/internal/assembly/load.go`：`loadPoolEntries()` 读取 `cr.options_json` 但只构造 `poolEntry`。
- `backend/internal/assembly/render_clash.go`：`appendRule` 中 `if mapped.SupportsNoResolve { line += ",no-resolve" }`。
- `backend/internal/assembly/render_sr.go`：`formatRuleLine` 对 `IP-CIDR`/`IP-CIDR6` 无条件追加。

### A.2 D3-2：统计重复

`backend/internal/pool/pipeline.go finalizeParseResult()`：

- 循环内对 Clash/SR 排除项执行 `res.Excluded++`；
- 循环后执行 `res.Excluded += len(rules)-res.Accepted-res.Rejected`；
- 该后置公式会把已排除项与重复项一并计入 excluded。

### A.3 D3-3：共享 Canonical 污染

`backend/internal/pool/pool.go UpdateEntry()`：

```sql
UPDATE pool_canonical_rules SET family=?, matcher=?, value=?, options_json=?, semantic_key=? WHERE id=?
```

由于 `pool_canonical_rules` 以 `pool_id + semantic_key` 唯一，手工 origin 与 URL origin 会共享同一 canonical 行，直接 UPDATE 会污染 URL 派生规则。

### A.4 D3-4：白名单

`backend/internal/pool/pool.go canonicalFromLegacyInput()` 只做：

```go
typ, normalized, err := rulespec.ValidateValue(ruleType, matchValue)
family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)
```

未检查 `rulespec.Capabilities()` 或 `LegacyMetadata()` 的 `MaterialPool` 标志。前端 `PoolDetail.vue` 虽已按 `material_pool` 过滤，但后端无强制。

### A.5 D3-5：来源证据

当前 `ParseResult`：

```go
type ParseResult struct {
    Format      DetectedFormat
    Profile     string
    Rules       []rulespec.CanonicalRule
    Diagnostics []ParseDiagnostic
    ...
}
```

`sync.go applyParseResultTx()` 写入：

```sql
sort_order = int64(len(parsed.Rules))  -- 所有规则相同
raw_line   = rule.SemanticKey()
line_no    = 0
```

查询排序：

```sql
ORDER BY CASE WHEN src.kind='manual' THEN 0 ELSE 1 END, src.sort_order, cr.id
```

没有使用 origin 的真实顺序；`loadPoolEntries()` 也没有按 canonical 去重，存在同规则重复渲染风险。

### A.6 D3-6：零输出门槛

`backend/internal/server/assembly.go generate()`：

```go
if res.Receipt != nil && res.Receipt.FinalOutput == 0 && (len(in.Pools) > 0 || len(in.CustomRules) > 0) {
    Fail(...)
}
```

当池和自定义规则都为空时绕过检查，与 Design3 §7.2 不符。

### A.7 D3-7：per-URL API 与 failed

当前：

- `pool_source_snapshots.status` 已包含 `failed`，但 `syncOne()` 失败时不会写该行；
- `server/pool.go` 只有 pending 激活/丢弃路由，没有快照读取路由；
- `PoolSource` 只返回 `active_snapshot_id`、`pending_snapshot_id`；
- `frontend/src/api/pool.ts` 只有 `PerURLResult`、`SyncTaskItem`，没有当前来源状态与快照详情类型。

### A.8 D3-8：pending UI

`frontend/src/api/pool.ts` 已导出：

```ts
activatePending
discardPending
```

但全前端没有任何调用点；`PoolDetail.vue` 只显示“待激活”文字。

### A.9 D3-9：回执展示

`backend/internal/server/assembly.go preview()` 返回：

```json
{ "receipt": ... }
```

`frontend/src/api/assembly.ts` 已定义：

```ts
receipt?: ConversionReceipt
```

但 `AssemblyView.vue` 未保存 `res.receipt`，`PreviewStep.vue` 未接收/渲染。

### A.10 D3-10：迁移测试

现有 `backend/internal/pool/pool_test.go` 仅在全新迁移后检查 `pool_entries` 表不存在，未覆盖旧数据、ID 防复用、历史版本保留。`Store.Migrate` 支持多次调用，可通过两段 `fstest.MapFS` 模拟旧库再应用 1016。
