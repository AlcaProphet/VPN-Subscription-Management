# Build22.md — BuildReport4 未闭环项 1：Build16/Design3 实质缺口研究与构建计划

> **文档定位：** 本文档是依据 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 中“未闭环项 1”开展的研究与下一轮构建计划。当前只完成研究与文档撰写，未修改任何业务代码。
> - 设计依据：[Design3.md](Design3.md)（Build16 的目标设计）、[Build16.md](docs/reports/Build/Build16.md)（原构建计划）、[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md)（缺口核验报告）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序构建：[Build17.md](Build17.md)～[Build21.md](Build21.md)、历史构建存档于 [docs/reports/Build/](docs/reports/Build)
> - 用户已确认（见文末决策记录）：Build22 放仓库根目录；研究范围覆盖 BuildReport4 §4.2 的 D3-1～D3-10；文档形式为“研究依据 + 可执行 Build 计划”；失败快照采用持久化 failed 行；来源证据暂不新增 JSON 路径列，先用现有 `line_no`/`raw_line`。
>
> **执行原则：**
> - 本文档只描述根因、修复方向、可选方案与后续可执行 Step；实施前仍应逐 Step 执行、验收，未完成前不得把 Build16/Design3 标记为“全部完成”。
> - 每步新增逻辑必须配套单元测试；测试应先复现缺口，再修改实现。
> - 不处理 BuildReport4 中未闭环项 2～6（R27-08/R27-09、节点输出诊断、smoke 脚本、安全报告、人工验收）以及 Build21 后续 Step，避免范围扩散。

---

## 一、研究结论摘要

BuildReport4 未闭环项 1 的实质是：**Build16 的 Step 3～6 虽然完成了表结构、解析管线、快照状态机和部分 API 骨架，但没有把“Design3 的数据语义完整贯穿到数据落库、装配读取、API 与前端展示”。** 后端已具备 Canonical Rule、能力注册表、快照表、pending 后端操作和回执结构，但多个关键链路仍在使用旧的 `rule_type + match_value` 简化模型，或只在前端做了过滤而缺少后端约束。

已确认的事实缺口：

| # | 缺口 | 根因定位 | 影响 |
|---|------|---------|------|
| D3-1 | `no_resolve` 语义丢失 | `assembly/load.go` 读取 `options_json` 后丢弃；`render_clash.go`/`render_sr.go` 按类型支持度无条件追加 `no-resolve` | 源中未设置 no-resolve 的 IP 规则也会被输出；源中设置 no-resolve 的规则也无法区分 |
| D3-2 | 被来源模式排除数量重复计算 | `pool/pipeline.go` 循环内已 `Excluded++`，随后又用 `len(rules)-accepted-rejected` 再累加 | 同步回执/UI 中 excluded 统计错误，且把重复项也计入 excluded |
| D3-3 | 手工规则更新污染共享 URL Canonical | `pool/pool.go UpdateEntry` 直接 `UPDATE pool_canonical_rules`，而该行可能同时被 URL origin 引用 | 手工改一条规则会改变同语义 URL 派生规则，违反 Rule Origin 隔离 |
| D3-4 | 后端未强制素材池能力白名单 | `pool/pool.go` 的 `canonicalFromLegacyInput` 只做值/类型合法校验，不检查 `MaterialPool` | API 可绕过前端过滤创建 `RULE-SET`/`AND`/`OR`/`NOT`/`MATCH` 等 advanced-only 素材 |
| D3-5 | 来源原始证据/排序未正确落库 | 各 adapter 只返回 `[]CanonicalRule`；`sync.go` 写入 `sort_order=len(rules)`、`line_no=0`、`raw_line=SemanticKey()`；装配/列表查询未按 origin 顺序排序且未去重 | 无法追溯来源行/原始条目；多来源同语义可能重复渲染；来源内顺序丢失 |
| D3-6 | 零输出门槛存在漏网 | `server/assembly.go` 仅在 `len(Pools)>0 || len(CustomRules)>0` 时才检查 `FinalOutput==0` | 用户不选任何素材池/自定义规则时，可仅以内置 `GEOIP`/`MATCH`/`FINAL` 生成，违反 Design3 §7.2 |
| D3-7 | per-URL 快照状态/诊断 API 缺失 | `pool` 包没有读取 `pool_source_snapshots` 的服务方法；`server/pool.go` 只有 pending 操作路由；`PoolSource` 仅回传 active/pending ID | 前端无法展示格式、平台、统计、诊断、failed/待同步等来源状态 |
| D3-8 | pending 激活/丢弃无前端 UI | `frontend/src/api/pool.ts` 已有 `activatePending/discardPending`，但没有任何页面调用 | pending 只能看到文字“待激活”，无法人工激活/丢弃 |
| D3-9 | 装配回执未展示 | 后端 preview 返回 `receipt`，前端 AssemblyView 只保存 `previewSkipped/previewWarnings`，未保存/传递/渲染 `receipt` | 用户看不到直接输出、等价转换、跳过、最终输出等关键回执 |
| D3-10 | 1015→1016 迁移缺少 store 级测试 | 现有测试只验证“新库无旧表”，未验证旧数据清除、旧 ID 不复用、历史版本保留等迁移语义 | 迁移风险缺少回归保护 |

另有两个与 D3-5 紧密相关、建议一并纳入本 Build 的观察项：

- `assembly/load.go` 的 `loadPoolEntries` 没有对共享 canonical 的多个 active origin 去重，可能把同一规则渲染多次。
- 失败同步目前不写入 `pool_source_snapshots`，无法支撑 per-URL failed 状态长期展示；用户已确认采用持久化 failed 快照行。

---

## 二、现状核验与根因分析

### 2.1 D3-1：`no_resolve` 为什么丢失

当前数据链路：

```text
pool_canonical_rules.options_json 保存规则实例的 no_resolve
  → assembly/load.go loadPoolEntries() 读出 options_json
  → 仅构造 poolEntry{RuleType, MatchValue}（load.go:191-223），丢弃 Options
  → render_clash.go appendRule() 只看 mapped.SupportsNoResolve（render_clash.go:85）
  → render_sr.go formatRuleLine() 对 IP-CIDR/IP-CIDR6 无条件追加（render_sr.go:135-138）
```

Design3 §7.2 的要求是：**`no_resolve` 仅在规则实际设置且目标支持时输出**。当前实现把“类型支持”当成了“实例已设置”，因此：

- 池内 `IP-CIDR,1.2.3.0/24`（源未写 no-resolve）会被 Clash/SR 均追加 `no-resolve`；
- 即使源写了 no-resolve，也因为 options 被丢弃，无法在装配层按实例保留/输出。

**修复方向：** 让装配层继续携带 CanonicalRule（至少携带 `Options.NoResolve`），渲染时使用 `noResolve && mapped.SupportsNoResolve` 决定是否追加；不能再用类型支持度直接追加。

### 2.2 D3-2：excluded 统计为什么重复

`pool/pipeline.go finalizeParseResult()`：

- 循环中，对 `SourceModeClash`/`SourceModeShadowrocket` 且目标不支持时已经执行 `res.Excluded++`（pipeline.go:70/75）；
- 循环后，又执行 `res.Excluded += len(rules) - res.Accepted - res.Rejected`（pipeline.go:108）。

这个后置加法在 Clash/SR 模式下会把已排除项再数一遍，同时还会把重复项也计入 excluded。例如 `[a.com, a.com, USER-AGENT]` 在 Clash 模式下会得到 excluded=2 甚至更大，而实际只有 1 条被来源模式排除、1 条重复。

**修复方向：** 删除后置累加，改成在循环内准确记录“因来源模式被排除”的数量；重复项继续单独计入 `Duplicates`，不进入 `Excluded`。

### 2.3 D3-3：手工更新为什么污染共享 Canonical

`pool/pool.go UpdateEntry()` 接收的是 `entryID`，但实际以 canonical 行 ID 定位：

```go
UPDATE pool_canonical_rules SET family=?, matcher=?, value=?, options_json=?, semantic_key=? WHERE id=?
```

`pool_canonical_rules` 是池级规则实体，`pool_id + semantic_key` 唯一，意味着手工 origin 和 URL origin 如果语义相同会共用同一行。直接更新该行会：

- 改变 URL 来源派生规则的语义；
- 即使手工只删除/修改，URL 来源的 origin 仍指向被改写的 canonical。

**修复方向：** 手工编辑不应改 shared canonical 行，而应改为“给手工 origin 换绑 canonical”。即：

1. 校验新语义仍是素材池可选能力；
2. 为新语义 `ensureCanonicalTx()`（若已存在则复用）；
3. 仅更新该手工 origin 的 `canonical_rule_id`（以及必要的 `raw_line`）；
4. 对旧 canonical 执行 `cleanupOrphanCanonicalTx()`，若没有其他 active/manual origin 再删除。

### 2.4 D3-4：后端素材池能力白名单为何未强制

`pool/pool.go` 的 `canonicalFromLegacyInput()` 只调用：

```go
typ, normalized, err := rulespec.ValidateValue(ruleType, matchValue)
family, matcher, ok := rulespec.CanonicalizeLegacyType(typ)
```

它没有查 `rulespec.Capabilities()` 或 `LegacyMetadata()` 的 `MaterialPool` 标志。前端 `PoolDetail.vue` 虽然从 `listCapabilityMeta().legacy.filter(m => m.material_pool)` 生成下拉，但直接调用后端 API 可以提交 `RULE-SET`、`AND`、`OR`、`NOT`、`MATCH` 等 advanced-only 规则。

**修复方向：** 在 `canonicalFromLegacyInput()` 或 `CreateEntry/UpdateEntry` 公共校验中，根据 family/matcher 查中央能力注册表，要求 `MaterialPool == true`；否则返回 `ErrBadRequest`。建议在 rulespec 增加一个可复用 helper，例如 `IsMaterialPool(CanonicalRule) bool`，避免后端/解析器各自复制。

### 2.5 D3-5：来源证据/排序为什么没有正确落库

当前解析结果只有：

```go
type ParseResult struct {
    Rules       []rulespec.CanonicalRule
    Diagnostics []ParseDiagnostic
    ...
}
```

各 adapter（`adapter_legacy.go`、`adapter_typed.go`、`adapter_mihomo.go`、`adapter_ip.go`、`adapter_singbox.go`）虽然知道行号/原始条目/展开顺序，但只把“拒绝”写入 diagnostics，**接受的规则没有携带 origin 元数据**。

随后 `sync.go applyParseResultTx()` 写 origin 时只能使用占位值：

```go
sort_order = int64(len(parsed.Rules))  // 所有规则相同
raw_line   = rule.SemanticKey()         // 不是原始行
line_no    = 0
```

装配读取时：

```sql
ORDER BY CASE WHEN src.kind='manual' THEN 0 ELSE 1 END, src.sort_order, cr.id
```

没有使用 `pool_rule_origins.sort_order/line_no/raw_line`，且在 `loadPoolEntries()` 中也没有按 canonical 去重。

**修复方向（用户已确认不新增 JSON 路径列）：**

- 引入 `RuleOriginMeta`（`Line`、`Raw`、`Order`、可选 `Path` 存到 Raw 的说明文本），让解析管线与 accepted rule 一一对齐。
- `ParseResult` 增加 `Origins []RuleOriginMeta`，`finalizeParseResult()` 在去重时保留首个 accepted origin。
- `applyParseResultTx()` 写入真实 `sort_order`/`raw_line`/`line_no`。
- `ListEntries()` 与 `loadPoolEntries()` 都改为：
  - 按“手工在前 → URL 配置顺序 → 来源内 origin 顺序 → line_no/id”稳定排序；
  - 按 canonical ID 去重，保证多个 active origin 的同一规则只渲染一次；
  - 返回/保留最早 origin 的排序依据。

### 2.6 D3-6：零输出门槛为什么有漏网

`server/assembly.go generate()` 当前：

```go
if res.Receipt != nil && res.Receipt.FinalOutput == 0 && (len(in.Pools) > 0 || len(in.CustomRules) > 0) {
    Fail(...)
}
```

当用户不选任何池/手工规则时，`len(in.Pools)==0 && len(in.CustomRules)==0` 为真，条件整体为 false。于是 Clash YAML 仍会生成“空素材 + 内置 GEOIP + MATCH”，SR conf 仍会生成“空素材 + GEOIP + FINAL”。这违反 Design3 §7.2 的“最终输出只统计素材池规则 + 自定义规则；最终输出为 0 时禁止生成”。

**修复方向：** 对会输出规则正文的 `clash-yaml` 和 `sr-conf`，无条件检查 `Receipt == nil || Receipt.FinalOutput == 0` 并拒绝生成；`sr-subs/generic-subs` 是纯节点订阅，不适用该规则。预览仍可渲染并返回 receipt，但生成必须被阻止。

### 2.7 D3-7：per-URL 快照/诊断 API 为什么缺失

数据库已保存：

- `pool_source_snapshots`：format、profile、status、五类计数、`diagnostic_json`、`stats_json`、创建时间；
- `rule_pool_sources` 的 active/pending 指针；
- `pool_sync_tasks.per_url_json`：每次任务的逐 URL 汇总。

但代码中：

- `pool` 包没有读取 snapshots 的 Service 方法；
- `server/pool.go` 没有“读取来源快照/状态”路由；
- `PoolSource` 只序列化 `active_snapshot_id` 与 `pending_snapshot_id`；
- 前端 `PoolDetail.vue` 只显示同步任务的历史 per_url 文本，不展示当前来源状态、诊断和范围统计。

更关键的是：当前 `syncOne()` 在 HTTP 错误、超时、解析错误时**不会**插入 `status='failed'` 的 snapshot 行，失败信息只存在于当次任务的 `per_url_json` 中，7 天清理后即消失。用户已确认应持久化 failed 快照行。

**修复方向：**

1. 后端增加快照读取模型与方法：
   - `SourceSnapshot`：包含 ID、source_id、format、profile、status、计数、诊断、stats、错误、时间；
   - `SourceStatus`：按来源聚合 active/pending/latest failed；
   - `ListSourceSnapshots(ctx, poolID, sourceID, page, pageSize)`。
2. 失败时写一条 `status='failed'` 的 snapshot（不需要新增列，把错误编码进 `diagnostic_json`/`stats_json`），不更新 active/pending 指针。
3. 新增 API：
   - `GET /api/admin/pools/:id/sources/status`：返回全部 URL 来源的当前状态摘要；
   - `GET /api/admin/pools/:id/sources/:sourceId/snapshots`：返回该来源快照历史（分页），供详情/诊断展示。
4. 前端新增来源状态区，展示格式、平台、状态、五类计数、诊断，并对 pending 提供激活/丢弃按钮。

### 2.8 D3-8：pending 前端 UI 缺失

后端已有：

```text
POST /api/admin/pools/:id/sources/:sourceId/pending/:snapshotId/activate
DELETE /api/admin/pools/:id/sources/:sourceId/pending/:snapshotId
```

`frontend/src/api/pool.ts` 也已导出 `activatePending/discardPending`。但全前端搜索仅有状态文字“待激活”，没有调用这两个函数。

**修复方向：** 在 `PoolDetail.vue` 的来源状态卡片中根据 `pending_snapshot_id` 展示 pending 条目，并提供“激活/丢弃”按钮；激活前弹窗显示旧 active 与新 pending 的数量/格式/平台差异，确认后调用对应 API 并刷新。

### 2.9 D3-9：装配回执为何未展示

后端 `POST /api/admin/assembly/preview` 已返回 `receipt`（`server/assembly.go:114`），`frontend/src/api/assembly.ts` 也定义了 `ConversionReceipt` 与 `PreviewResponse.receipt`。但 `AssemblyView.vue` 的 `doPreview()` 只保存：

```ts
previewText, previewHash, previewSkipped, previewWarnings
```

没有保存 `res.receipt`；`PreviewStep.vue` 的 props 也没有 receipt，因此页面不渲染回执。

**修复方向：** AssemblyView 增加 `previewReceipt` 状态并在 `doPreview()` 赋值；传给 PreviewStep；PreviewStep 增加回执摘要区（输入数、直接输出、等价转换、目标不支持跳过、目标校验失败、最终输出）。可进一步在 generate 响应中也返回 receipt，供生成后的结果页展示。

### 2.10 D3-10：1016 迁移测试缺失

现有 `pool/pool_test.go` 只在全新迁移后检查 `pool_entries` 表不存在，未覆盖：

- 旧数据被清除；
- `rule_pools.id` 不复用旧 ID；
- 旧 `pool_sync_tasks`/`pool_entries` 不残留；
- 历史版本、蓝图等非素材池数据不受影响。

**修复方向：** 增加 store 级迁移回归测试：先迁移到旧 1009 结构并写入旧池数据，再应用真实 1016 SQL，验证旧表删除、旧数据清除、新 `rule_pools` 自增从旧最大值之后开始，且版本/蓝图等无关数据保留。

---

## 三、修复方案汇总

| 缺口 | 首选修复 | 备选/说明 |
|------|---------|-----------|
| D3-1 | 装配层携带 Canonical/NoResolve，渲染按实例选项追加 | 暂不给自定义规则新增 no-resolve UI；若后续需要，再扩展 `RuleLine.NoResolve` |
| D3-2 | 删除后置累加，循环内精确计数 | 补充表驱动统计测试 |
| D3-3 | 手工编辑改为换绑 origin，不更新共享 canonical | 保留 canonical 唯一约束，不做数据迁移 |
| D3-4 | `canonicalFromLegacyInput`/CRUD 增加 `MaterialPool` 校验 | 建议提供 `rulespec.IsMaterialPool()` 复用 |
| D3-5 | 解析管线携带 origin 元数据；落库真实 line/raw/order；查询按 origin 排序并去重 | 不新增 `origin_path` 列；JSON 路径放 raw_line/诊断说明 |
| D3-6 | Clash/SR 规则目标无条件阻止 `FinalOutput==0` | 预览仍可返回 receipt |
| D3-7 | 持久化 failed snapshot + 新增来源状态/快照 API | failed 错误写入 diagnostic_json/stats_json，不新增 error 列 |
| D3-8 | PoolDetail 来源状态卡片 + pending 激活/丢弃交互 | 确认弹窗展示旧/新差异 |
| D3-9 | AssemblyView 保存 receipt 并传入 PreviewStep 渲染 | 可选在 generate 结果中也返回 receipt |
| D3-10 | store 级 1016 迁移回归测试 | 使用两段自定义迁移 FS 模拟旧库 |

---

## 四、构建计划（候选 Step，需逐项实施）

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。下列 Step 为后续实施时的推荐拆分；实际执行仍按“一次一个 Step、先失败回归、后实现、再全量验证”的规则。

### Step 1：修复来源统计计数（D3-2）

- **目标：** 让 `ParseResult.Excluded` 只统计“因来源模式被排除”的规则，重复项单列。
- **前置条件：** 无。
- **产出文件与操作：**
  - `backend/internal/pool/pipeline.go`：删除循环后的 `res.Excluded += len(rules)-...`；在循环中精确计数；补充注释。
  - `backend/internal/pool/parser_test.go`：新增表驱动用例，覆盖 Clash/SR 模式下的 excluded/duplicates 混合。
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
  // 删除循环后的重复累加
  ```
- **测试与验收：**
  ```bash
  cd backend && go test ./internal/pool -run 'TestParseSource.*|TestFinalize'
  cd backend && go build ./...
  ```
- **验收标准：** excluded 不重复、不包含 duplicates；现有解析测试全部通过。

### Step 2：来源原始证据采集与排序（D3-5 前半 + 装配去重）

- **目标：** 让 accept 规则携带 line_no/raw_line/来源内顺序，并让查询按 origin 顺序稳定返回、按 canonical 去重。
- **前置条件：** Step 1 通过。
- **产出文件与操作：**
  - `backend/internal/pool/types.go`：新增 `RuleOriginMeta`，`ParseResult` 增加 `Origins []RuleOriginMeta`。
  - `adapter_legacy.go`、`adapter_typed.go`、`adapter_mihomo.go`、`adapter_ip.go`、`adapter_singbox.go`：每个 accepted rule 同时返回其 Line/Raw/Order。对 JSON/YAML 用条目索引作为 Order，Line 可为 0，Raw 可存原始条目或 JSON 路径说明。
  - `pool/pipeline.go`：`finalizeParseResult()` 在去重时保留首个 origin。
  - `pool/sync.go applyParseResultTx()`：写真实 `sort_order`、`line_no`、`raw_line`。
  - `pool/pool.go ListEntries()`、`assembly/load.go loadPoolEntries()`：排序加入 origin 字段，Go 侧按 canonical 去重。
- **参考结构：**
  ```go
  type RuleOriginMeta struct {
      Line  int    `json:"line"`
      Raw   string `json:"raw"`
      Order int    `json:"order"`
  }
  ```
- **测试与验收：**
  - 新增 parser/origin 测试：确认 origin 顺序、行号、原始行正确。
  - 新增 pool/assembly 测试：多来源同语义只渲染一次，且按“手工先、URL 配置顺序、来源内顺序”输出。
  ```bash
  cd backend && go test ./internal/pool ./internal/assembly ./internal/server
  cd backend && go build ./...
  ```
- **验收标准：** `pool_rule_origins` 不再全是同值 sort_order/line_no=0；装配结果无重复规则；来源内顺序影响最终规则顺序。

### Step 3：no_resolve 实例语义贯通（D3-1）

- **目标：** 装配只在实际设置 no_resolve 且目标支持时输出。
- **前置条件：** Step 2（依赖装配层携带 Canonical 或至少 Options）。
- **产出文件与操作：**
  - `backend/internal/assembly/load.go`：`poolEntry` 增加 `NoResolve bool` 或直接保存 `Canonical rulespec.CanonicalRule`。
  - `backend/internal/assembly/render_clash.go`：`appendRule` 增加 `noResolve bool` 参数；仅在 `noResolve && mapped.SupportsNoResolve` 时追加。
  - `backend/internal/assembly/render_sr.go`：`formatRuleLine` 增加 `noResolve bool` 参数，同样按支持度追加。
  - 相关测试：包含“源未设置 no-resolve”“源设置 no-resolve”“目标不支持 no-resolve”三组。
- **参考伪代码：**
  ```go
  if noResolve && mapped.SupportsNoResolve {
      line += ",no-resolve"
  }
  ```
- **测试与验收：**
  ```bash
  cd backend && go test ./internal/assembly ./internal/server
  cd backend && go build ./...
  ```
- **验收标准：** Clash/SR 输出中 no-resolve 只出现在源实际设置且目标支持的行；现有默认追加行为不再作为正确预期。

### Step 4：后端素材池能力白名单（D3-4）

- **目标：** 手工条目 CRUD 只允许 `MaterialPool=true` 的能力。
- **前置条件：** 无。
- **产出文件与操作：**
  - `backend/internal/rulespec/`：新增 `IsMaterialPool(CanonicalRule) bool` 或在 capability.go 暴露 helper。
  - `backend/internal/pool/pool.go`：`canonicalFromLegacyInput()` 或 `CreateEntry/UpdateEntry` 调用该校验，失败返回 `ErrBadRequest`。
- **参考伪代码：**
  ```go
  if !rulespec.IsMaterialPool(rule) {
      return ..., fmt.Errorf("%w: 不是素材池可选能力", ErrBadRequest)
  }
  ```
- **测试与验收：**
  - 新增 pool 测试：`RULE-SET`、`AND`、`OR`、`NOT`、`MATCH`、`GEOSITE` 等非 material 类型应被拒绝；`DOMAIN`、`IP-CIDR`、`USER-AGENT` 等可创建。
  ```bash
  cd backend && go test ./internal/pool ./internal/rulespec
  cd backend && go build ./...
  ```
- **验收标准：** 后端与前端下拉同样只允许素材池可选能力。

### Step 5：手工编辑不再污染共享 Canonical（D3-3）

- **目标：** 手工规则更新改为换绑 origin。
- **前置条件：** Step 4（白名单校验复用）。
- **产出文件与操作：**
  - `backend/internal/pool/pool.go UpdateEntry()`：事务内定位手工 origin，构建新 canonical，`ensureCanonicalTx()` 后更新 origin 的 `canonical_rule_id`，最后 `cleanupOrphanCanonicalTx()`。
  - 如果新语义与旧相同，直接返回；如果新语义已存在，复用该 canonical。
- **参考伪代码：**
  ```go
  // 不再 UPDATE pool_canonical_rules
  newCanonicalID, err := ensureCanonicalTx(ctx, tx, poolID, canonical)
  _, err = tx.ExecContext(ctx,
      `UPDATE pool_rule_origins SET canonical_rule_id=?, raw_line=? WHERE id=?`,
      newCanonicalID, ruleType+","+value, manualOriginID)
  return s.cleanupOrphanCanonicalTx(ctx, tx, poolID)
  ```
- **测试与验收：**
  - 新增回归：手工和 URL 同语义，手工改为新值后 URL 仍输出旧值；旧 canonical 无引用时被清理。
  ```bash
  cd backend && go test ./internal/pool ./internal/assembly
  ```
- **验收标准：** 手工编辑不影响 URL 派生规则；无孤儿 canonical 膨胀。

### Step 6：零输出门槛补全（D3-6）

- **目标：** 规则型目标无任何非系统规则时禁止生成。
- **前置条件：** Steps 1～3（receipt 已可正确统计/去重）。
- **产出文件与操作：**
  - `backend/internal/server/assembly.go generate()`：移除 `len(in.Pools)>0 || len(in.CustomRules)>0` 条件，改为对 `clash-yaml`/`sr-conf` 的 `Receipt.FinalOutput==0` 直接拒绝。
  - 相关 server 测试：无池无自定义、只有不支持规则、至少一条有效规则。
- **参考伪代码：**
  ```go
  if res.Receipt != nil && res.Receipt.FinalOutput == 0 {
      Fail(c, http.StatusBadRequest, "当前目标没有可输出的非系统规则")
      return
  }
  ```
- **测试与验收：**
  ```bash
  cd backend && go test ./internal/server ./internal/assembly
  cd backend && go build ./...
  ```
- **验收标准：** 空素材池/空自定义规则无法生成 Clash YAML 或 SR conf；有效的单条规则可生成。

### Step 7：failed 快照持久化 + per-URL 状态/快照 API（D3-7）

- **目标：** 让每个 URL 都可查询当前 active/pending/latest failed 状态、格式、平台、统计与诊断。
- **前置条件：** Steps 1～2（origin/统计已修正）。
- **产出文件与操作：**
  - `backend/internal/pool/sync.go`：
    - 新增 `recordFailedSnapshotTx()`，在 HTTP/解析失败时写入 `pool_source_snapshots(status='failed', diagnostic_json=错误摘要, stats_json=错误详情)`，不改变 active/pending。
    - `syncOne()` 在失败路径调用。
  - `backend/internal/pool/snapshot.go`（或 sync.go 内）：定义 `SourceSnapshot`、`SourceStatus`，实现 `ListSourceSnapshots`、`ListSourceStatuses`。
  - `backend/internal/server/pool.go`：新增两个只读路由：
    - `GET /api/admin/pools/:id/sources/status`
    - `GET /api/admin/pools/:id/sources/:sourceId/snapshots`
  - `frontend/src/api/pool.ts`：新增类型与请求函数。
- **参考响应结构：**
  ```json
  {
    "source_id": 1,
    "url": "https://...",
    "source_mode": "auto",
    "active": { "id": 10, "format": "typed-rule-text", "profile": "common", "accepted": 3, ... },
    "pending": { "id": 11, ... },
    "latest_failed": { "id": 12, "error": "HTTP 500", ... }
  }
  ```
- **测试与验收：**
  - 新增测试：HTTP 失败/解析失败会生成 failed snapshot；active 不变；快照 API 能读取；pending/active 状态正确。
  ```bash
  cd backend && go test ./internal/pool ./internal/server
  cd backend && go build ./...
  ```
- **验收标准：** 失败可持久化；per-URL 状态/诊断 API 有真实数据可查。

### Step 8：前端来源状态、诊断与 pending 操作（D3-7 UI + D3-8）

- **目标：** PoolDetail 展示每个 URL 的来源状态、检测格式、平台、统计、诊断，并提供 pending 激活/丢弃。
- **前置条件：** Step 7 API 可用。
- **产出文件与操作：**
  - `frontend/src/views/admin/assembly/PoolDetail.vue`：新增“URL 来源状态”区，加载 `source status`/snapshots；展示 active/pending/failed 徽标、格式/平台、计数、诊断；pending 行显示“激活/丢弃”按钮。
  - 激活/丢弃前用 ConfirmModal 展示旧 active 与新 pending 的数量、格式、平台差异；调用已导出的 `activatePending/discardPending`。
  - `frontend/tests/pool-detail.spec.ts`：mock 新 API，覆盖 pending 展示、激活确认、丢弃调用。
  - 同步完成后刷新来源状态。
- **测试与验收：**
  ```bash
  cd frontend && npm test -- --run tests/pool-detail.spec.ts
  cd frontend && npm run build
  ```
- **验收标准：** 每个 URL 均有清晰的当前状态；pending 可人工激活/丢弃；桌面与窄屏可操作。

### Step 9：装配回执前端展示（D3-9）

- **目标：** 预览页展示转换回执。
- **前置条件：** 后端已返回 receipt。
- **产出文件与操作：**
  - `frontend/src/views/admin/AssemblyView.vue`：增加 `previewReceipt`，在 `doPreview()` 中赋值 `res.receipt`。
  - `frontend/src/views/admin/assembly/PreviewStep.vue`：新增 `receipt` prop 并渲染输入数/直接输出/等价转换/跳过/校验失败/最终输出。
  - 可选：generate 响应中也返回 receipt 并在 Result 中展示。
  - 新增前端测试。
- **测试与验收：**
  ```bash
  cd frontend && npm test -- --run tests/assembly-view.spec.ts tests/preview-step.spec.ts
  cd frontend && npm run build
  ```
- **验收标准：** 预览或生成后能清晰看到回执数字。

### Step 10：1016 迁移回归测试（D3-10）

- **目标：** 用 store 级测试保护 1016 不兼容迁移语义。
- **前置条件：** 无。
- **产出文件与操作：**
  - 新增 `backend/internal/store/migration_1016_test.go` 或 `backend/internal/pool/migration_1016_test.go`。
  - 测试流程：
    1. 构造包含 0001 基本表 + 1009 旧素材池 schema 的临时 FS；
    2. 迁移到 1009，插入旧 `rule_pools`、`pool_entries`、`pool_sync_tasks` 数据；
    3. 换成包含真实 1016 SQL 的 FS 继续 `Migrate`；
    4. 断言旧表/旧数据不存在，新表存在，新池 ID 大于旧最大 ID，历史版本/蓝图（若在测试 schema 中）保留。
  - 如需更精确，可导入 `migrations.FS` 读取真实 1016 SQL，避免测试与迁移文件漂移。
- **测试与验收：**
  ```bash
  cd backend && go test ./internal/store ./internal/pool -run 'Migration|1016'
  cd backend && go build ./...
  ```
- **验收标准：** 旧数据移除、ID 防复用、无关历史数据保留均有断言。

### Step 11：全量回归与文档收口

- **目标：** 全量构建/测试通过，并把 Build16/Design3 实际状态同步到文档。
- **前置条件：** Steps 1～10 通过。
- **验证命令：**
  ```bash
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test ./... -count=1 -timeout 180s
  cd frontend && npm run build
  cd frontend && npm test -- --run
  git diff --check
  ```
- **文档同步（实施完成后再做）：**
  - Build16.md：修正 Step 3～6 的验收状态，将未完成项标记清楚。
  - Design3.md：记录实现与设计的实际差异（如失败快照持久化、错误存放方式）。
  - AGENTS.md：仅在实际全部完成后再登记 Build22 状态。
- **验收标准：** 所有自动门禁通过；文档不虚标“Build16 已全部完成”。

---

## 五、已确认决策与待办边界

| 决策点 | 用户确认 |
|--------|---------|
| Build22 位置 | 仓库根目录 `Build22.md` |
| 研究范围 | 覆盖 BuildReport4 §4.2 全部 D3-1～D3-10 |
| 文档形态 | 研究依据 + 可执行 Build 计划 |
| failed 快照 | 持久化 `status='failed'` 快照行 |
| 来源证据字段 | 暂不新增 `origin_path` 列，使用现有 `line_no`/`raw_line` |

**不在本 Build 范围：**

- BuildReport4 未闭环项 2～6（R27-08/R27-09、节点输出诊断、smoke 脚本、安全报告、人工验收）。
- Build21 尚未实施的 R27-09 Step 11～14。
- 为自定义装配规则新增 no-resolve 输入控件（除非后续用户明确要求）。
- 新增 `urls: string[]` 兼容、旧素材池数据迁移或后续新适配器。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-05 | 根据 BuildReport4 未闭环项 1 完成根因研究、修复方向与分步构建计划；未修改任何业务代码。 |
