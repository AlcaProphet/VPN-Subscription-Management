# VPN 订阅管理系统 功能构建计划（Build22：当前构建方案）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前构建方案**（依据 AGENTS.md：Build 文档为详细构建方案，非强规则），承接已完成的 [Build17.md](docs/reports/Build/Build17.md)～[Build20.md](docs/reports/Build/Build20.md) 以及已验收的 [Build21.md](Build21.md)；Build21 Step 14 已于 2026-09-09 收口，工程问题见 [Issue14.md](Issue14.md)，用户人工结果见 [ProdTestList.md](ProdTestList.md)。本轮针对 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 的**未闭环项 1** 进行深入研究并制定修复计划。**该未闭环项当前已登记于 [Issue14.md](Issue14.md) R28-05；本文件仍为后续实施 D3-1～D3-10 的唯一分步计划。**
> - 设计记录：[Design3.md](Design3.md)（Build16 的目标设计，当前仍有效）、[Build16.md](docs/reports/Build/Build16.md)（原构建计划）
> - 问题来源：[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md)（全量核验报告，未闭环项 1）
> - 问题追踪：[Issue14.md](Issue14.md)（R28-05）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 历史构建与问题记录：见 [docs/reports/](docs/reports/)（均已存档，仅核查）
>
> **用户已确认的决策：**
> 1. Build22.md 放仓库根目录，作为当前构建方案。
> 2. 研究范围覆盖 BuildReport4 §4.2 的 **D3-1～D3-10** 全部缺口。
> 3. 文档采用“研究依据 + 可执行 Build 计划”形态，按本模板重新排版。
> 4. per-URL failed 状态采用**持久化 failed 快照行**，不只在任务 JSON 中临时记录。
> 5. 来源原始证据**暂不新增 `origin_path` 列**，使用现有 `line_no`、`raw_line`；JSON/YAML 路径信息放入 `raw_line` 或诊断说明。
> 6. 相同语义只建立一个 Canonical Rule，但**保留每一个 origin**，包括同一 URL 内的重复位置；`accepted` 统计唯一 Canonical，`duplicates` 统计额外 origin。
> 7. 手工条目修改到另一个已存在的手工语义时返回 409；若目标 Canonical 仅有 URL origin，则允许换绑并共享。
> 8. 每 URL 主状态以**最近一次同步尝试**为准；最近失败但旧 active 仍有效时同时显示“同步失败”和“继续使用旧活动快照”，之后成功则旧 failed 只保留在历史中。
> 9. 新 Clash render plan 显式冻结实例级 `no_resolve`；旧计划缺失该字段时维持历史按类型推断，禁止历史下载漂移。
> 10. 素材池能力白名单以后端入口的**原始 legacy 类型**为准，拒绝 `SRC-GEOIP`、`SRC-IP-ASN`、`SRC-IP-CIDR` 等 `material_pool=false` 类型；不能只按 Canonical `family/matcher` 判定，避免与 `GEOIP`/`IP-ASN`/`IP-CIDR` 同语义碰撞后被放行。
>
> **执行原则（与 Build17～Build21 一致）：**
> - 每一步完成后均可编译、可测试。不跳步、不并行多步。
> - AI 执行指令：每次仅执行一个 Step，完成后运行验收命令，确认通过后再进入下一步。
> - **排序原则：先修复后构建、先安全后优化、先依赖后独立**。
> - 每步的新增逻辑必须配套单元测试；测试应先复现缺口，再修改实现。
> - 本文档当前仅完成研究、方案与排版，**未修改任何业务代码**。
> - **执行顺序注意：** Build21 已完成；Build22 仍按本文 Step 1～11 串行执行。若期间出现其他任务修改 `pool`、`rulespec`、`assembly` 或素材池前端文件，必须先重新核对差异和同文件冲突。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 修复来源统计计数（D3-2） | Design3 §5.3、§6.4 | ☐ 未开始 |
| 2 | 来源原始证据采集、排序与装配去重（D3-5） | Design3 §3.2、§5.4、§6.1 | ☐ 未开始 |
| 3 | `no_resolve` 实例语义贯通（D3-1） | Design3 §7.2 | ☐ 未开始 |
| 4 | 后端素材池能力白名单（D3-4，按原始 legacy 类型拒绝 `SRC-*`） | Design3 §3.3、§3.4、§8.3 | ☐ 未开始 |
| 5 | 手工编辑不污染共享 Canonical（D3-3） | Design3 §3.2、§6.1 | ☐ 未开始 |
| 6 | 零输出门槛补全（D3-6） | Design3 §7.2 | ☐ 未开始 |
| 7 | failed 快照持久化 + per-URL 状态/诊断 API（D3-7） | Design3 §6.4、§8.2、§8.3 | ☐ 未开始 |
| 8 | 前端来源状态、诊断与 pending 操作（D3-7 UI、D3-8） | Design3 §8.2 | ☐ 未开始 |
| 9 | 装配回执前端展示（D3-9） | Design3 §7.2、§8.2 | ☐ 未开始 |
| 10 | 1016 迁移 store 级回归测试（D3-10） | Design3 §6.5、§9.3 | ☐ 未开始 |
| 11 | 全量回归、文档同步与 Build16/Design3 状态收口 | AGENTS.md §3.4～§3.6 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。
> 当前没有进行中的构建 Step；所有 Step 均待按本文档逐步执行。
> 工程状态追踪：上述 D3-1～D3-10 未闭环项已登记至 [Issue14.md](Issue14.md) R28-05；本文档作为实施计划，不替代问题追踪。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/pool/pipeline.go`、`backend/internal/pool/parser_test.go` | 修正 `Excluded` 重复累加与清洗阶段诊断回写；补充统计/诊断回归测试 |
| 2 | `backend/internal/pool/types.go`、`adapter_*.go`、`pipeline.go`、`sync.go`、`pool.go`、`backend/internal/assembly/load.go` 及相关测试 | 使用 `ParsedRule{Rule, Origin}`；真实证据落库；保留全部 origin；查询在分页前按 Canonical 去重并稳定排序 |
| 3 | `backend/internal/pool/parser.go`、`adapter_typed.go`、`adapter_mihomo.go`、`backend/internal/assembly/load.go`、`render_clash.go`、`render_sr.go`、`clash_plan.go` 及相关测试 | 结构化解析并贯通 `no_resolve`；新计划显式冻结，旧计划兼容；生成与下载语义一致 |
| 4 | `backend/internal/rulespec/legacy.go`（或新增 helper）、`backend/internal/pool/pool.go`、`adapter_typed.go`、`adapter_mihomo.go`、`pool_test.go` | 后端按**原始 legacy 类型**强制 `MaterialPool` 白名单；手工 CRUD 与来源解析均拒绝 advanced-only/`SRC-*` 等非素材池类型 |
| 5 | `backend/internal/pool/pool.go`、`backend/internal/pool/pool_test.go`、`backend/internal/server/pool_test.go` | 手工编辑改为换绑 canonical origin；manual 重复返回 409；后端服务与 HTTP 409 合同均有回归，不直接修改共享 canonical 行 |
| 6 | `backend/internal/server/assembly.go`、`server/assembly_test.go` | 规则型目标无条件执行 `FinalOutput==0` 禁止生成 |
| 7 | `backend/internal/pool/sync.go`、新增 `snapshot.go`（或同类文件）、`backend/internal/server/pool.go`、`frontend/src/api/pool.ts`、后端/前端测试 | 失败时写入 failed snapshot；统一限额/脱敏；增加 latest-attempt 来源状态与快照历史 API |
| 8 | `frontend/src/views/admin/assembly/PoolDetail.vue`、`frontend/tests/pool-detail.spec.ts` | 每 URL 展示状态/统计/诊断；pending 激活与丢弃 |
| 9 | `backend/internal/server/assembly.go`、`backend/internal/server/assembly_test.go`、`frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/assembly/PreviewStep.vue`、`frontend/tests/assembly-view.spec.ts`、`preview-step.spec.ts` | generate 返回本次回执；前端保存并渲染 preview/generate 装配转换回执 |
| 10 | 新增 `backend/internal/store/migration_1016_test.go` | 使用真实 0001～1016 两阶段迁移，验证旧数据清除、ID 防复用、无关历史保留及失败回滚 |
| 11 | `docs/reports/Build/Build16.md`（仅追加勘误/后续闭环说明）、`Design3.md`、`Issue14.md`、`AGENTS.md`、本文件 | 全量验证与 R28-05/Design3 文档状态收口，不倒改归档 Build16 的历史进度 |

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
- Step 4 是 Step 5 的前置校验来源；按“不并行多步”的执行原则，建议按 Step 4 → Step 5 串行实施。
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
    - 清洗/能力分类阶段追加诊断后，将最终切片回写 `res.Diagnostics`，避免 `res := &ParseResult{Diagnostics: diagnostics}` 后继续 append 局部切片导致结果缺项。
  - `backend/internal/pool/parser_test.go`：新增表驱动用例，覆盖普通排除、重复+排除、auto 模式不产生被来源模式排除、非素材池能力诊断实际出现在结果中等场景。

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
  Clash/SR 模式下 excluded 与 duplicates 互不污染；auto 模式不出现来源模式排除；清洗阶段新增诊断不丢失；现有解析测试全部通过。

---

### Step 2：来源原始证据采集、排序与装配去重（D3-5）

- **背景/根因：**
  - 各 adapter 只返回 `[]rulespec.CanonicalRule`，接受的规则没有携带行号、原始行、来源内顺序。
  - `sync.go applyParseResultTx()` 写入 `sort_order=int64(len(parsed.Rules))`、`raw_line=rule.SemanticKey()`、`line_no=0`。
  - `ListEntries()` 与 `assembly/load.go loadPoolEntries()` 只按 `src.sort_order, cr.id` 排序，未使用 `pool_rule_origins` 的真实顺序。
  - `loadPoolEntries()` 未按 canonical 去重，多个 active origin 指向同一 canonical 时可能重复渲染。

- **目标：** 让每个通过来源准入的候选都携带完整来源证据并正确落库；相同语义只建立一个 Canonical Rule，但包括同一来源内重复位置在内的每个 origin 都保留；列表与装配按“手工在前 → URL 配置顺序 → 来源内原始顺序”稳定输出，且同一 Canonical 只展示/渲染一次。

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

      type ParsedRule struct {
          Rule   rulespec.CanonicalRule
          Origin RuleOriginMeta
      }
      ```
    - `ParseResult` 以 `Items []ParsedRule`（名称可按实现调整）承载规则与 origin，避免 `Rules`/`Origins` 平行切片长度或排序发生错位；如为兼容测试暂时保留 `Rules` 投影，投影只能由 `Items` 派生，不能成为第二事实来源。
  - `adapter_legacy.go`、`adapter_typed.go`、`adapter_mihomo.go`、`adapter_ip.go`、`adapter_singbox.go`：
    - 每个 accepted rule 记录 Line/Raw/Order。
    - 对纯文本适配器，Line 为真实 1-based 行号，Raw 为原始行。
    - 对 Mihomo YAML，Line 可为 0，Order 为 `payload` 内索引，Raw 保存原始条目；如后续需要可在适配器内使用 YAML 节点行号增强。
    - 对 sing-box JSON，Line 可为 0，Order 使用全局递增序号（避免 `ruleIndex*1000+valueIndex` 在大量规则下碰撞），Raw 保存 JSON 路径如 `rules[0].domain[1]`。
  - `pool/pipeline.go finalizeParseResult()`：
    - 第一个 `semantic_key` 计入 `Accepted`，后续相同语义计入 `Duplicates`；
    - 所有通过来源准入的 item 均保留 origin，重复 item 不新增 Canonical 计数但仍进入 origin 持久化集合；
    - `Accepted + Duplicates` 表示通过来源准入的 origin 数，`Accepted` 单独表示唯一 Canonical 数。
  - `pool/sync.go applyParseResultTx()`：
    - `sort_order` 写 `origin.Order`；
    - `line_no` 写 `origin.Line`；
    - `raw_line` 写 `origin.Raw`。
  - `pool/pool.go ListEntries()`：
    - 查询增加 `o.sort_order, o.line_no, o.raw_line`；
    - 排序改为 `CASE WHEN src.kind='manual' THEN 0 ELSE 1 END, src.sort_order, o.sort_order, o.line_no, o.id`；
    - 在 SQL 中先从当前有效 origins 为每个 `canonical_rule_id` 选定排序最早的一条，再执行 `LIMIT/OFFSET`；推荐使用 `ROW_NUMBER() OVER (PARTITION BY canonical_rule_id ORDER BY ...) = 1` 或等价 CTE；
    - `COUNT` 与分页查询必须复用同一个有效 origin 条件和去重口径；Go 侧 `seen` 只能作为非必要的安全断言，不得再承担分页后的主去重职责；
    - `source=manual/url` 筛选先限定来源集合，再在该集合内选最早 origin，确保来源筛选总数与列表一致。
  - `backend/internal/assembly/load.go loadPoolEntries()`：
    - `poolEntry` 增加 `NoResolve bool`、`Canonical rulespec.CanonicalRule`、`OriginOrder`、`OriginLine`、`RawLine` 等字段（按 Step 3 需要）；
    - 查询增加 origin 字段，以相同的有效 origin CTE/窗口口径按 Canonical 去重并排序；
    - 返回顺序直接代表最终渲染顺序，不依赖 Canonical 自增 ID。

- **参考数据写入伪代码：**
  ```go
  for _, pr := range parsed.Items { // 包含唯一与重复语义的全部有效 origin
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
  `pool_rule_origins.sort_order/line_no/raw_line` 不再全部是占位值；同一 URL 内重复和跨来源重复均保留 origin，但相同 Canonical 只展示/渲染一次；`accepted` 为唯一 Canonical 数、`duplicates` 为额外 origin 数；手工在前、URL 按配置顺序、来源内按原始顺序；删除最早 origin 后按剩余最早 origin 稳定重排；ListEntries 任意分页和来源筛选均不因重复 origin 少条或出现总数不一致。

---

### Step 3：`no_resolve` 实例语义贯通（D3-1）

- **背景/根因：**
  - `loadPoolEntries()` 读取 `options_json` 后仅构造 `poolEntry{RuleType, MatchValue}`，丢失 `Options.NoResolve`。渲染层随后依赖 `mapped.SupportsNoResolve`，只要类型支持就追加 `no-resolve`。
  - `parseTypedText()`、`parseMihomoClassicalYAML()` 和手工兼容入口使用整行/值的 `strings.Contains(..., "no-resolve")` 推断选项，可能把 `no-resolve.example.com` 等匹配值误判为选项。
  - `ClashPlanRule` 没有保存实例级选项，下载重渲染继续按类型自动追加；即使预览 renderer 修正，下载仍会恢复错误后缀。
  - `downgradeRuleLines()` 在重写目标组时也按类型补加 `no-resolve`。以上共同违反 Design3 “仅当规则实际设置且目标支持时输出”和历史下载不漂移合同。

- **目标：** 从来源结构化解析、Canonical、装配加载、首次渲染到 Clash 下载重渲染全链路保留规则实例的 `no_resolve`；新计划严格按实例输出，旧计划维持历史行为。

- **前置条件：** Step 2 通过（装配层已携带 Canonical 能力）。

- **产出文件与操作：**
  - `backend/internal/pool/parser.go`、`adapter_typed.go`、`adapter_mihomo.go`：
    - 扩展解析结果，结构化区分 type、value、源 policy 和尾部 options；
    - 仅独立且大小写规范化后等于 `no-resolve` 的 option token 设置 `RuleOptions.NoResolve=true`；
    - 不再对整行或 `matchValue` 做子串搜索；源 policy 继续忽略并记录诊断，未知 option 进入诊断或按既有严格策略拒绝，不静默转义为 `no_resolve`；
    - 手工 CRUD 当前没有独立 options 输入，默认 `NoResolve=false`，值中出现 `no-resolve` 文本不得改变选项。
  - `backend/internal/assembly/load.go`：
    - `poolEntry` 明确保存 `NoResolve bool` 或完整 `Canonical rulespec.CanonicalRule`。
  - `backend/internal/assembly/render_clash.go`：
    - `appendRule` 增加 `noResolve bool`；
    - 仅在 `noResolve && mapped.SupportsNoResolve` 时追加 `,no-resolve`。
  - `backend/internal/assembly/render_sr.go`：
    - `formatRuleLine` 增加 `noResolve bool`，同样按 `mapped.SupportsNoResolve` 判定。
  - `backend/internal/assembly/clash_plan.go`：
    - `ClashPlanRule` 增加可区分“字段缺失”与显式 `false` 的实例字段（推荐 `NoResolve *bool` 或带 schema version 的等价实现）；
    - 新生成计划对每条规则都显式冻结 true/false；读取旧计划时字段缺失，沿用旧版按类型推断以保持历史下载正文；读取新计划时只按实例字段输出；
    - `downgradeRuleLines()` 只保留已解析到的 `noResolve`，不得因类型支持再次追加。
  - 相关测试：
    - 池内 `IP-CIDR` 未写 no-resolve → 不输出；
    - 池内 `IP-CIDR` 写了 no-resolve → 输出；
    - 自定义规则默认不继承 no-resolve（本轮不新增自定义规则 no-resolve 输入）；
    - 内置 `GEOIP` 等系统规则没有实例设置时不因类型能力自动追加；
    - SR 侧同样覆盖；
    - `DOMAIN,no-resolve.example.com,...` 不误设选项，独立尾部 `no-resolve` 正确设置；
    - 新 Clash plan 的显式 false/true 在下载重渲染后与预览一致；旧 plan 字段缺失时输出保持修复前兼容行为；覆盖层降级目标组不凭类型新增后缀。

- **参考伪代码：**
  ```go
  if noResolve && mapped.SupportsNoResolve {
      line += ",no-resolve"
  }

  // Clash plan：nil 表示旧计划，非 nil 表示新计划显式实例值。
  effective := mapped.SupportsNoResolve // 仅用于兼容旧计划
  if planRule.NoResolve != nil {
      effective = *planRule.NoResolve && mapped.SupportsNoResolve
  }
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/pool ./internal/assembly ./internal/server
  cd backend && go build ./...
  ```

- **验收标准：**
  Clash YAML 与 SR 分流规则中的 `no-resolve` 仅出现在源实际设置且目标支持的行；值内同名子串不触发选项；新版本预览、生成正文和用户下载重渲染一致；既有历史 Clash plan 下载不漂移；既有默认“IP 类型必加 no-resolve”的新计划测试期望改为按实例断言。

---

### Step 4：后端素材池能力白名单（D3-4）

- **背景/根因：**
  - `pool.go` 的 `canonicalFromLegacyInput()` 只做 `ValidateValue` 和 `CanonicalizeLegacyType`，未检查 `MaterialPool`。前端下拉虽已过滤，但直接调用后端 API 可以创建 `RULE-SET`、`AND`、`OR`、`NOT`、`MATCH`、`GEOSITE` 等 advanced-only 素材。
  - 进一步核对发现：`CanonicalizeLegacyType()` 会把 `SRC-GEOIP`、`SRC-IP-ASN`、`SRC-IP-CIDR` 分别映射成与 `GEOIP`、`IP-ASN`、`IP-CIDR` 相同的 `family/matcher`；因此若只按 Canonical Rule 查 `capabilityRegistry.MaterialPool`，这 3 个 `material_pool=false` 类型仍会被后端放行。**白名单必须以后端入口的原始 legacy 类型为准。**

- **目标：** 后端成为素材池可选能力的最终约束；手工 CRUD、来源解析准入、前端下拉共用同一份“原始 legacy 类型 `MaterialPool`”事实来源。

- **前置条件：** 无。

- **产出文件与操作：**
  - `backend/internal/rulespec/legacy.go`（或新增 helper）：
    - 新增可复用 helper：
      ```go
      // IsMaterialPoolType 按原始 legacy 规则类型判断是否可进入素材池。
      // 事实来源与前端下拉一致：legacyCapabilityMap / LegacyMetadata() 的 MaterialPool。
      func IsMaterialPoolType(ruleType string) bool
      ```
    - 不得只用 `capabilityRegistry`/`findCapability` 按 Canonical `family/matcher` 判定，避免 `SRC-GEOIP`/`SRC-IP-ASN`/`SRC-IP-CIDR` 与素材池通用类型碰撞。
    - `legacy_test.go` 补充 helper 正反例，尤其是 `SRC-*` 反例。
  - `backend/internal/pool/pool.go`：
    - `canonicalFromLegacyInput()` 先由 `ValidateValue()` 校验类型和值并取得规范化的原始 legacy 类型 `typ`，再调用 `rulespec.IsMaterialPoolType(typ)`，最后才执行 `CanonicalizeLegacyType()`；这样既保留未知类型/非法值的既有错误语义，又确保 `SRC-*` 在有损 Canonical 映射前被拒绝；
    - 非素材池类型返回 `ErrBadRequest`，错误信息明确“不是素材池可选能力”，且不得写入任何 canonical/origin。
  - `backend/internal/pool/adapter_typed.go`、`adapter_mihomo.go`：
    - 解析到显式类型 `typ` 后、映射 Canonical 前先调用 `rulespec.IsMaterialPoolType(typ)`；
    - 非素材池类型按当前解析器惯例追加 `reject` 诊断（`Kind:"reject", Message:"不是素材池可选能力", Raw:line`），不再进入后续 Canonical/统计流程。
  - `backend/internal/pool/pipeline.go`：
    - 保留现有“不是素材池可选能力”的 capability 检查作为第二道防线，但 Step 4 不把它作为唯一判定来源；待 Step 2 引入 `ParsedRule{Rule, Origin}` 后如需保留类型证据可再增强，不在本步扩大 Canonical 模型。
  - `pool_test.go` 及相关解析测试：
    - 新增反例：`RULE-SET`、`AND`、`OR`、`NOT`、`MATCH`、`GEOSITE`、`SRC-GEOIP`、`SRC-IP-ASN`、`SRC-IP-CIDR`、`IP-SUFFIX`、`DST-PORT` 等创建/更新/来源解析均拒绝；
    - 正例：`DOMAIN`、`DOMAIN-SUFFIX`、`IP-CIDR`、`USER-AGENT` 等允许。

- **参考伪代码：**
  ```go
  // pool 手工 CRUD 入口
  typ, normalized, err := rulespec.ValidateValue(ruleType, matchValue)
  if err != nil {
      return ..., err
  }
  if !rulespec.IsMaterialPoolType(typ) {
      return ..., fmt.Errorf("不是素材池可选能力: %s", typ)
  }
  family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)

  // adapter_typed.go / adapter_mihomo.go 显式类型解析
  if !rulespec.IsMaterialPoolType(typ) {
      diagnostics = append(diagnostics, ParseDiagnostic{
          Line: i + 1, Kind: "reject", Message: "不是素材池可选能力", Raw: line,
      })
      continue
  }
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/rulespec ./internal/pool
  cd backend && go build ./...
  ```

- **验收标准：**
  - 后端手工 CRUD 与 URL 来源解析均拒绝所有 `LegacyMetadata().MaterialPool == false` 的类型，包括 `SRC-GEOIP`、`SRC-IP-ASN`、`SRC-IP-CIDR`；
  - `RULE-SET`、`AND/OR/NOT`、`MATCH`、`GEOSITE` 等 advanced-only 类型不能进入素材池；
  - `DOMAIN`、`DOMAIN-SUFFIX`、`IP-CIDR`、`USER-AGENT` 等素材池类型可正常创建/解析；
  - 前端下拉、手工 CRUD、来源解析共用同一原始 legacy 类型白名单，不再出现“前端不可选但后端可创建”的偏差。

---

### Step 5：手工编辑不污染共享 Canonical（D3-3）

- **背景/根因：**
  `UpdateEntry()` 直接 `UPDATE pool_canonical_rules`。由于 `pool_canonical_rules` 对 `pool_id + semantic_key` 唯一，手工 origin 与 URL origin 可能共用同一 canonical 行；直接修改会改变 URL 派生规则。

- **目标：** 手工编辑只改动手工 origin 的绑定关系，不修改共享 canonical 行。

- **前置条件：** Step 4 通过（复用白名单校验）。

- **产出文件与操作：**
  - `backend/internal/pool/pool.go UpdateEntry()`：
    1. 事务内按传入 Canonical ID 定位唯一手工 origin（`o.snapshot_id IS NULL AND src.kind='manual'`），同时取得 origin ID、旧 Canonical ID、pool ID 和 manual source ID；
    2. 若新语义与旧 canonical 相同，直接返回；
    3. 对新 canonical 执行 `ensureCanonicalTx()`（已存在则复用）；
    4. 换绑前检查目标 Canonical 是否已有另一个 `snapshot_id IS NULL` 的 manual origin：存在则返回 `ErrEntryConflict`，由 server 映射为 HTTP 409；仅有 URL origins 时允许共享；
    5. 更新当前 origin 的 `canonical_rule_id`、`raw_line`，保留原 `sort_order`，避免编辑导致手工顺序变化；
    6. 调用 `cleanupOrphanCanonicalTx()` 清理旧 canonical 孤儿。
  - 不修改 shared canonical 行的 `family/matcher/value/options_json/semantic_key`。
  - `backend/internal/pool/pool_test.go` 与 `backend/internal/server/pool_test.go`：新增“手工与 URL 同语义，手工修改后 URL 仍保持旧值；目标仅有 URL origin 时允许共享；目标已有另一 manual origin 时服务层返回 `ErrEntryConflict`、HTTP 层返回 409 且两条原记录不变；换绑保留排序；旧 canonical 无有效 origin 时被清理”的回归。

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
  cd backend && go test ./internal/pool ./internal/assembly ./internal/server
  cd backend && go build ./...
  ```

- **验收标准：**
  手工规则更新只影响指定 manual origin；同语义 URL 规则保持原值；manual→manual 重复稳定返回 409 且事务无部分写入；manual→仅 URL Canonical 可共享；编辑前后手工顺序不变；无孤儿 canonical 残留。

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
    - 提取统一的 snapshot 持久化规范化 helper：所有成功/失败诊断在写 `diagnostic_json`、`stats_json`、任务 `per_url_json/error` 前执行脱敏和限额，最多 20 条、每条字段/消息最多 200 字符；URL 查询中的 `token`、`code`、`state` 和疑似凭据不得写入；
    - 不保存响应正文、HTML 或整份 parser 输入；failed 仅保存有限错误分类与摘要；
    - 不修改 active/pending 指针。
  - `syncOne()`：
    - 请求构造、HTTP 状态码错误、超时、读取失败/超限、HTML/空内容、格式冲突和解析错误等可归属到具体 source 的失败路径均调用上述方法；
    - failed snapshot 写入本身失败时不能伪称已持久化，单 URL 结果应返回原始业务错误的脱敏摘要并附带“失败快照写入失败”，详细数据库错误只进脱敏日志；不得修改旧 active/pending；
    - `applyParseResultTx()` 等数据库基础设施失败无法可靠再在同一失败事务中落 failed 行，归为任务/日志基础设施错误，并由测试固定边界。
  - 新增 `backend/internal/pool/snapshot.go`（或同类文件）：
    - `SourceSnapshot` 模型：ID、source_id、format、profile、status、`input/recognized/accepted/excluded/rejected/duplicates` 等完整计数、`Diagnostics`、`Stats`、`Error`、CreatedAt；`Diagnostics` 固定复用 `ParseDiagnostic{line,kind,message,raw}`，不另增未定义的 `severity` 字段，空诊断序列化为 `[]` 而不是 `null`；
    - `SourceStatus` 模型：source_id、脱敏后的展示 URL、source_mode、`latest_attempt`、active、pending、latest_failed、never_synced 标记，并要求 active/pending/latest_attempt/latest_failed 对象都使用同一个 `SourceSnapshot` 摘要形状，携带对应快照计数与有限诊断；原始 URL 只用于数据库配置和实际拉取，不得由状态/历史 API 回传或被脱敏值覆盖；
    - `latest_attempt` 取该 source 按 `created_at DESC, id DESC` 的最近快照，用它决定主状态；`latest_failed` 仅作为历史快捷信息，不单独决定主状态；
    - 最近失败但 active 仍存在时同时返回 failed latest_attempt 与 active；失败后又有更新的 active/pending 时，主状态按新尝试恢复，旧 failed 只在历史/latest_failed 中可查；
    - `ListSourceStatuses(ctx, poolID)`；
    - `ListSourceSnapshots(ctx, poolID, sourceID, page, pageSize)`。
  - `backend/internal/server/pool.go`：
    - 新增路由：
      ```
      GET /api/admin/pools/:id/sources/status
      GET /api/admin/pools/:id/sources/:sourceId/snapshots
      ```
    - 路由继续叠加 session + admin 双中间件。
    - `sources/status` 是当前池全部 URL 来源的非分页列表，统一返回 `{ "list": SourceStatus[], "total": number }`，无 URL 来源时必须返回 `list: []`；
    - `snapshots` 接受 `page`、`page_size`，使用列表默认值 1/20 和 `MaxPageSize` 上限，按 `created_at DESC, id DESC` 返回 `{ "list": SourceSnapshot[], "total": number }`；pool/source 不匹配返回 404，非法 ID 或非数字分页参数返回 400；
    - server 原始 JSON 测试必须固定列表包裹、snake_case 字段、`[]` 非 `null`、分页总数以及 URL/诊断脱敏，避免只依赖 Go 类型或前端 mock。
  - `frontend/src/api/pool.ts`：
    - 新增 `SourceSnapshot`、`SourceStatus` 类型与 `listSourceStatuses`、`listSourceSnapshots` 请求函数。
  - 清理策略：
    - 在 `CleanupOldTasks()` 或新增清理逻辑中，以单个事务同时清理超过 7 天、状态为 failed 且未被任何 active/pending 指针引用的快照；active/pending 快照和所有仍被指针引用的快照不得删除；
    - 删除 failed snapshot 后执行孤儿 Canonical 清理，但不得影响任何手工或 active/pending origin；手动“清理同步历史”保持现有语义，只清任务，不扩大为快照删除。
  - 测试：
    - HTTP 失败生成 failed snapshot；
    - 解析失败生成 failed snapshot；
    - active/pending 不变；
    - 状态 API 返回统一包裹的 latest_attempt/active/pending/latest_failed/never_synced，四类快照摘要形状一致；
    - 最近失败+旧 active 时主状态为 failed 且 active 同时返回；随后成功时主状态恢复 active，旧 failed 不再控制徽标；
    - 同时间戳时使用 ID 稳定选择 latest_attempt；
    - 快照历史分页、稳定倒序、总数以及 pool/source 归属校验正确；
    - failed/成功诊断、展示 URL、任务 JSON 和两个 API 的原始 JSON 均不包含 URL 查询凭据/Token，且诊断严格满足 20×200 限额；
    - failed 写库失败不会改变 active/pending，也不会报告虚假的 snapshot ID；
    - 7 天清理不删 active/pending/被引用快照，不误删有效 origin。

- **参考状态列表 API 响应：**
  ```json
  {
    "list": [{
      "source_id": 1,
      "url": "https://example.com/rules.txt?token=***",
      "source_mode": "auto",
      "never_synced": false,
      "latest_attempt": {
        "id": 12,
        "source_id": 1,
        "format": "",
        "profile": "",
        "status": "failed",
        "input": 0,
        "recognized": 0,
        "accepted": 0,
        "excluded": 0,
        "rejected": 0,
        "duplicates": 0,
        "diagnostics": [{"line": 0, "kind": "error", "message": "HTTP 500", "raw": ""}],
        "stats": {"error": "HTTP 500"},
        "error": "HTTP 500",
        "created_at": "2026-09-09T12:00:00+08:00"
      },
      "active": {
        "id": 10,
        "source_id": 1,
        "format": "typed-rule-text",
        "profile": "common",
        "status": "active",
        "input": 4,
        "recognized": 4,
        "accepted": 3,
        "excluded": 1,
        "rejected": 0,
        "duplicates": 0,
        "diagnostics": [],
        "stats": {},
        "error": "",
        "created_at": "2026-09-09T11:00:00+08:00"
      },
      "pending": null,
      "latest_failed": {
        "id": 12,
        "source_id": 1,
        "format": "",
        "profile": "",
        "status": "failed",
        "input": 0,
        "recognized": 0,
        "accepted": 0,
        "excluded": 0,
        "rejected": 0,
        "duplicates": 0,
        "diagnostics": [{"line": 0, "kind": "error", "message": "HTTP 500", "raw": ""}],
        "stats": {"error": "HTTP 500"},
        "error": "HTTP 500",
        "created_at": "2026-09-09T12:00:00+08:00"
      }
    }],
    "total": 1
  }
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/pool ./internal/server
  cd backend && go build ./...
  cd frontend && npm run build
  ```

- **验收标准：**
  可归属到来源的拉取/内容/解析失败可持久化并查询；来源状态 API 以最近尝试区分 active/pending/failed/从未同步，并能表达“最近失败但旧 active 继续生效”；失败后成功不会永久标红；failed 快照不会进入 active 查询；诊断限额和脱敏在所有持久化出口一致；清理不会误删有效快照、origin 或 Canonical。

---

### Step 8：前端来源状态、诊断与 pending 操作（D3-7 UI、D3-8）

- **背景/根因：**
  `frontend/src/api/pool.ts` 已有 `activatePending/discardPending`，但没有任何页面调用；`PoolDetail.vue` 只显示“URL 数量”与同步历史摘要，不展示每个 URL 的格式、平台、状态、统计和诊断。

- **目标：** 在素材池详情中增加每个 URL 的来源状态卡片，并支持 pending 激活/丢弃。

- **前置条件：** Step 7 通过。

- **产出文件与操作：**
  - `frontend/src/views/admin/assembly/PoolDetail.vue`：
    - onMounted 与同步完成后调用 `listSourceStatuses`；
    - 每个 URL 展示：URL、来源模式、检测格式/平台、由 `latest_attempt.status` 决定的 active/pending/failed/待同步主徽标、input/recognized/接受/排除/拒绝/重复统计、诊断摘要与有限样例；
    - 最近失败但 active 仍存在时显示“同步失败，继续使用旧活动快照”，并分别展示失败尝试与当前 active 摘要；成功发生在 failed 之后时不得仅因 `latest_failed` 非空继续显示失败主徽标；
    - `pending_snapshot_id` 存在时显示“激活/丢弃”按钮；
    - 激活前 ConfirmModal 展示旧 active 与新 pending 的 input/accepted、格式、平台、诊断差异；
    - 调用 `activatePending/discardPending` 后刷新来源状态与条目。
  - `frontend/src/api/pool.ts`：
    - 复用 Step 7 新增类型。
  - `frontend/tests/pool-detail.spec.ts`：
    - mock `listSourceStatuses`、`activatePending`、`discardPending`；
    - 覆盖 pending 展示、激活确认、丢弃、同步后刷新；
    - 覆盖 failed+active 并存、failed 后成功恢复、从未同步、同一来源历史 failed 不永久控制主状态。

- **测试与验收命令：**
  ```bash
  cd frontend && npm test -- --run tests/pool-detail.spec.ts
  cd frontend && npm run build
  ```

- **浏览器验收证据：**
  - 自动化组件测试只证明状态分支、按钮和刷新逻辑；另以实际浏览器至少核对 1440px 桌面与 390px 窄屏，覆盖 failed+active、pending 确认、诊断长文本和激活/丢弃后的刷新；
  - 记录可复查的视口、操作结果与控制台状态；该证据只证明浏览器交互，不外推为真实客户端导入/连接兼容。

- **验收标准：**
  每个 URL 的最近尝试状态及实际生效 active 均可辨认；失败后恢复不会永久标红；pending 可人工激活/丢弃，激活前能看到旧/新差异；所有诊断只展示后端已脱敏的有限数据；桌面与 <768px 窄屏均可操作。

---

### Step 9：装配回执前端展示（D3-9）

- **背景/根因：**
  后端 preview 已返回 `receipt`，`frontend/src/api/assembly.ts` 也已有 `ConversionReceipt` 类型；但 `AssemblyView.vue` 未保存 `res.receipt`，`PreviewStep.vue` 也未接收该 prop，因此用户看不到回执。

- **目标：** 在预览步骤与生成成功页展示输入数、直接输出、等价转换、目标不支持跳过、目标校验失败、最终输出。

- **前置条件：** Step 6 通过（回执统计口径已正确）。

- **产出文件与操作：**
  - `frontend/src/views/admin/AssemblyView.vue`：
    - 新增 `previewReceipt` ref；
    - `doPreview()` 中 `previewReceipt.value = res.receipt ?? null`；
    - 将 `previewReceipt` 传给 `PreviewStep`；目标、输入或预览发生失效/清空时同步清空回执，禁止把旧目标统计显示为当前结果。
  - `frontend/src/views/admin/assembly/PreviewStep.vue`：
    - 新增 `receipt?: ConversionReceipt | null` prop；
    - 在警告/跳过区域附近渲染回执摘要卡或列表。
  - 必做（Design3 §7.2 要求 preview/generate 都返回回执）：
    - 后端 `generate` 响应同样返回本次 `res.Receipt`；
    - `backend/internal/server/assembly_test.go` 增加 generate 成功响应的原始 JSON 合同断言，固定 `receipt` 六项 snake_case 数值来自本次 `Render` 结果，不用前端 mock 代替后端 wire shape；
    - `frontend/src/api/assembly.ts` 的 generate 响应类型增加 `receipt?: ConversionReceipt`；
    - `generateResult` 保存该字段，生成成功结果页同步展示，不复用可能已经 stale 的 previewReceipt。
  - `frontend/tests/assembly-view.spec.ts`、`frontend/tests/preview-step.spec.ts`：
    - 验证 preview 保存、传递与六项数字渲染；
    - 验证目标/输入变化后旧回执不再显示；
    - 验证 generate 使用服务端本次返回的回执，且缺省 receipt 时界面保持兼容。

- **参考 UI 文案：**
  ```text
  转换回执：输入 N · 直接输出 N · 等价转换 N · 目标不支持跳过 N · 校验失败 N · 最终输出 N
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/server
  cd backend && go build ./...
  cd frontend && npm test -- --run tests/assembly-view.spec.ts tests/preview-step.spec.ts
  cd frontend && npm run build
  ```

- **验收标准：**
  用户能在预览和生成结果中看到完整转换回执；数字分别与对应 preview/generate 响应一致；过期回执不会冒充当前结果；已有预览、差异、previewStale 和生成成功流程无回归。

---

### Step 10：1016 迁移 store 级回归测试（D3-10）

- **背景/根因：**
  现有测试只验证“全新迁移后旧表不存在”，未验证旧数据清除、旧 ID 不复用、无关历史数据保留。

- **目标：** 用迁移级测试保护 1016 不兼容迁移语义。

- **前置条件：** 无。

- **产出文件与操作：**
  - 新增 `backend/internal/store/migration_1016_test.go`，由 store 层直接验证迁移框架与真实 SQL；pool 测试只保留迁移后的业务行为，不复制迁移脚本。
  - 测试流程：
    1. 从 `migrations.FS` 读取真实迁移文件，构造包含真实 0001～1015 的 `fstest.MapFS`，不得复制或手写简化版 1009 素材池 schema；
    2. `Store.Migrate` 到真实 1015 结构，并断言数据库 schema 版本确为 1015；
    3. 插入多个旧 `rule_pools`（包含非连续/高 ID）、`pool_entries`、`pool_sync_tasks`，并在 `versions`、`assembly_blueprints` 等不应删除的表中插入可识别历史数据；
    4. 再使用包含真实 0001～1016 的 FS 对同一 Store 继续 `Migrate`；
    5. 断言：
       - 旧表 `pool_entries`、旧 `pool_sync_tasks` 已删除；
       - 新表 `rule_pool_sources`、`pool_source_snapshots`、`pool_canonical_rules`、`pool_rule_origins` 存在；
       - 旧池数据不残留；
       - 新 `rule_pools.id` 从旧最大 ID 之后开始（旧 ID 不复用）；
       - `versions`、`assembly_blueprints` 及其他明确无关历史数据的内容和关联保持不变；
       - `schema_migrations` 只新增 1016，重复调用 `Migrate` 幂等。
    6. 失败回滚子用例必须使用独立的新临时数据库：先应用真实 0001～1015 并插入可识别夹具，再在真实 1016 SQL 后附加一个必然失败语句形成测试专用迁移；确认 1016 的删表、建表、序列更新、数据变化与 schema 版本记录全部回滚，且 1015 夹具仍可读取。不得复用已成功应用 1016 的数据库，也不得修改生产迁移文件。
  - 测试 helper 必须直接读取嵌入的真实迁移内容并按版本过滤，避免测试 SQL 与生产迁移漂移。

- **参考流程：**
  ```go
  // 第一次 Migrate：真实 0001～1015
  _ = st.Migrate(ctx, migrationsThrough(1015))
  // 插入旧数据
  // 第二次 Migrate：真实 0001～1016
  _ = st.Migrate(ctx, migrationsThrough(1016))
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/store ./internal/pool -run 'Migration|1016'
  cd backend && go build ./...
  ```

- **验收标准：**
  真实 1015→1016 链路下，旧素材池业务数据清除、ID 防复用、无关历史保留、schema 版本、重复迁移幂等和失败事务回滚均有自动断言。

---

### Step 11：全量回归、文档同步与 R28-05/Design3 状态收口

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
    docker compose build
    bash .smoke-test-prod.sh
    git diff --check
    ```
  - 文档同步：
    - `docs/reports/Build/Build16.md`：保留归档构建的历史 Step 状态；如需消除“当时已全部闭环”的歧义，只追加后续勘误/关联说明，记录 D3-1～D3-10 经 BuildReport4 发现并最终由 Build22 闭环，不倒改历史进度或把 Build16 重新作为当前构建入口。
    - `Design3.md`：记录实现与设计的实际落点，尤其是 failed 快照持久化、来源证据存储方式和 per-URL API 形态；将 §9.3 的当前串行执行入口从 Build16 更新为 Build22。
    - `Issue14.md`：仅在 Steps 1～10 均有验收证据后，同步步骤三表格、R28-05 状态和关闭条件；不得提前标记 D3-1～D3-10 完成。
    - 顺带修正 `PoolTab.vue` 中“停机错过不补跑”的陈旧文案，与当前启动补跑实现保持一致。
    - `AGENTS.md`：仅在全部实际完成后登记 Build22。
    - 本文件：更新进度表与验收结果。

- **验收标准：**
  所有自动命令和正式 Production smoke 通过；D3-1 的新计划实例语义与旧计划兼容均有下载证据，D3-5 的排序/分页有数据库级证据，D3-7 的状态恢复和脱敏有限诊断有 API/UI 证据，D3-10 的真实迁移有 store 级证据；Build22、Design3、Issue14 与 AGENTS 状态一致，归档 Build16 只保留历史记录和后续勘误，不倒改或虚标验收状态。

---

## 五、已确认构建项映射

> 以下候选均来自 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §4.2，并已经用户确认纳入 Build22。工程跟踪见 [Issue14.md](Issue14.md) R28-05；后续实施时按上述 Step 顺序逐项执行。

| # | 候选 | 说明 | 来源 | 对应 Step |
|---|------|------|------|-----------|
| 1 | `no_resolve` 语义丢失 | 装配层丢失 Options，渲染按类型支持度无条件追加 | Design3 §7.2 | Step 3 |
| 2 | `excluded` 重复计算 | 统计循环与后置累加导致排除数错误，且混入重复 | Design3 §5.3、§6.4 | Step 1 |
| 3 | 手工更新污染 URL Canonical | UpdateEntry 直接改共享 canonical 行 | Design3 §3.2、§6.1 | Step 5 |
| 4 | 后端素材池白名单未强制 | 只做值校验，未按原始 legacy 类型校验 `MaterialPool`；且不能仅按 Canonical `family/matcher` 判定，需拒绝 `SRC-*` 碰撞类型 | Design3 §3.3、§3.4 | Step 4 |
| 5 | 来源原始证据/排序未落库 | 行号/原始行/来源内顺序均为占位；查询未按 origin 排序且未去重 | Design3 §3.2、§5.4 | Step 2 |
| 6 | 零输出门槛漏网 | 无池/自定义时绕过 `FinalOutput==0` 检查 | Design3 §7.2 | Step 6 |
| 7 | per-URL 快照状态/诊断 API 缺失 | 无快照读取服务与路由；failed 未持久化 | Design3 §6.4、§8.2、§8.3 | Step 7 |
| 8 | pending 激活/丢弃无 UI | 前端已有 API 但无页面调用 | Design3 §8.2 | Step 8 |
| 9 | 装配回执未展示 | 后端返回 receipt，前端未保存/渲染 | Design3 §7.2、§8.2 | Step 9 |
| 10 | 1016 迁移测试缺失 | 只验新库无旧表，未验旧数据/ID/历史保留 | Design3 §6.5、§9.3 | Step 10 |

> D3-1～D3-10 及本轮补充的重复 origin、manual 409、latest-attempt 状态、旧 render plan 兼容口径和“原始 legacy 类型素材池白名单（拒绝 `SRC-*`）”均已由用户确认并写入对应 Step。后续若发现改变产品语义或兼容边界的新候选，仍须先研究并由用户决策，不能直接并入构建。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.6 | 2026-09-09 | 构建前文档核验补强：冻结 Step 7 列表包裹、分页、统一快照摘要、诊断字段与展示 URL 脱敏合同；补齐 Step 5/9 后端测试和编译门禁、Step 8 双视口浏览器证据、Step 10 独立数据库失败回滚；Step 11 改为同步 Issue14/Design3/AGENTS，并仅向归档 Build16 追加后续勘误而不倒改历史状态。仅完善文档，未修改业务代码、未执行构建。 |
| v1.5 | 2026-09-09 | 按用户确认微调 Step 4：素材池白名单改为在手工 CRUD 与来源解析入口按**原始 legacy 类型**判定，拒绝 `SRC-GEOIP`/`SRC-IP-ASN`/`SRC-IP-CIDR` 等与 `GEOIP`/`IP-ASN`/`IP-CIDR` Canonical 碰撞的非素材池类型；同步修订构建概要、决策清单、候选映射与附录 A.4。仅完善文档，未修改业务代码、未执行构建。 |
| v1.4 | 2026-09-09 | R28-05 第二次只读研究后按用户确认详细修订：补充相同语义保留全部 origin 与分页前去重；`no_resolve` 扩展至结构化来源解析、实例渲染、新旧 Clash render plan 和覆盖层重写；manual→manual 重复返回 409；来源主状态以 latest attempt 为准并可同时保留旧 active；统一诊断限额/脱敏；迁移测试改用真实 0001～1016 两阶段链路并覆盖幂等/回滚。Build21 Step 14 前置已完成，Build22 Step 1～11 仍全部未开始，本次未修改业务代码。 |
| v1.0 | 2026-09-05 | 根据 BuildReport4 未闭环项 1 完成 D3-1～D3-10 根因研究、修复方向与候选清单。 |
| v1.1 | 2026-09-05 | 进一步深入研究并按照 `docs/DocTemplates/Build.template.md` 重排：新增构建进度追踪、构建概要、顺序依赖图、分步构建计划、候选构建项与变更记录；补充 failed 快照持久化、来源证据实现细节、迁移测试方法与清理策略。未修改任何业务代码。 |
| v1.2 | 2026-09-05 | 按审阅建议补强：generate 回执改为必做；per-URL SourceStatus/SourceSnapshot 补全 input/recognized、诊断摘要与有限样例；failed 快照增加脱敏与诊断限额；依赖说明改为串行；补充与 Build21 同文件区域的串行执行提醒。 |
| v1.3 | 2026-09-08 | 文档交叉审核：将 D3-1～D3-10 未闭环项登记至 Issue14 R28-05；本文档保留为实施计划并在进度追踪中补充 Issue14 链接。未修改业务代码。 |

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
- `backend/internal/pool/adapter_typed.go`、`adapter_mihomo.go` 与手工兼容入口：使用整行/值的子串搜索识别 `no-resolve`，没有区分匹配值、policy 和独立 option token。
- `backend/internal/assembly/clash_plan.go`：`ClashPlanRule` 未冻结实例选项，下载重渲染和目标组降级仍按类型追加。修复必须覆盖 plan 的新旧兼容，不能只让管理员首次预览正确。

### A.2 D3-2：统计重复

`backend/internal/pool/pipeline.go finalizeParseResult()`：

- 循环内对 Clash/SR 排除项执行 `res.Excluded++`；
- 循环后执行 `res.Excluded += len(rules)-res.Accepted-res.Rejected`；
- 该后置公式会把已排除项与重复项一并计入 excluded。
- `ParseResult` 在循环前保存了 `Diagnostics` slice，循环内新增“不是素材池可选能力”诊断后没有回写最终 slice，存在诊断缺项；Step 1 一并修正，Step 7 再在持久化边界统一限额和脱敏。

### A.3 D3-3：共享 Canonical 污染

`backend/internal/pool/pool.go UpdateEntry()`：

```sql
UPDATE pool_canonical_rules SET family=?, matcher=?, value=?, options_json=?, semantic_key=? WHERE id=?
```

由于 `pool_canonical_rules` 以 `pool_id + semantic_key` 唯一，手工 origin 与 URL origin 会共享同一 canonical 行，直接 UPDATE 会污染 URL 派生规则。

换绑还必须处理目标 Canonical 已经存在的情况：只有 URL origin 时允许共享；已经存在另一个 manual origin 时不能静默合并，否则一次编辑会让两条手工记录在列表去重后看似丢失。已确认维持现有手工唯一语义并返回 409，事务失败后两条旧记录均保持不变。

### A.4 D3-4：白名单

`backend/internal/pool/pool.go canonicalFromLegacyInput()` 只做：

```go
typ, normalized, err := rulespec.ValidateValue(ruleType, matchValue)
family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)
```

未检查 `rulespec.LegacyMetadata()` 的 `MaterialPool` 标志。前端 `PoolDetail.vue` 虽已按 `material_pool` 过滤，但后端无强制。

**本轮补充确认的 Canonical 碰撞：** 当前 `CanonicalizeLegacyType()` 存在有损映射：

```text
SRC-GEOIP   → geo/equals      （与 GEOIP 相同）
SRC-IP-ASN  → ip/asn          （与 IP-ASN 相同）
SRC-IP-CIDR → ip/cidr         （与 IP-CIDR 相同）
```

因此不能以“Canonical Rule 的 family/matcher 查 capabilityRegistry.MaterialPool”作为唯一白名单依据，否则上述 `SRC-*` 会被误判为可进入素材池。已确认改为：在手工 CRUD 和显式类型来源解析入口，先按**原始 legacy 类型**的 `MaterialPool` 做白名单，再 Canonical 化；`capabilityRegistry` 检查仅作为第二道防线。`RULE-SET`、`AND/OR/NOT`、`MATCH`、`GEOSITE` 及 `SRC-*` 等 `material_pool=false` 类型统一拒绝进入素材池。

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

初版 Build22 曾提出“重复项只计数、不新增 origin”，与 Design3 §3.2/§5.3 的“语义去重但保留 origin”冲突。本轮已按用户确认修正：所有原始位置均落 origin，`accepted` 计唯一 Canonical，`duplicates` 计额外 origin；查询必须先从有效 origin 集合选最早 origin，再分页/渲染。

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

只返回 `latest_failed` 会让历史失败长期控制 UI。本轮已确认以 `latest_attempt` 决定主状态，同时独立返回 active/pending/latest_failed：最近失败可以与旧 active 并存，后续成功后旧 failed 只作为历史。所有 snapshot/task 诊断必须在共同持久化边界执行 20 条×200 字符限制和敏感信息脱敏。

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

现有 `backend/internal/pool/pool_test.go` 仅在全新迁移后检查 `pool_entries` 表不存在，未覆盖旧数据、ID 防复用、历史版本保留。`Store.Migrate` 支持多次调用，但使用手写最小 1009 schema 会绕过真实历史迁移间的表、索引和外键关系；Step 10 必须从嵌入的 `migrations.FS` 过滤出真实 0001～1015，再对同一数据库应用真实 1016，并覆盖重复迁移幂等和单迁移失败整体回滚。
