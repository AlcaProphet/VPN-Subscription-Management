# Build16.md — 规则来源识别、结构化素材与跨平台装配构建计划

> **文档定位：** 本文是 VPN 订阅管理系统第十六轮当前构建方案，将 [Design3.md](../../../Design3.md) 的已确认设计转化为可逐步执行和验收的实现手册。本文已完成八个 Step 实施并通过全量验证。
> - 设计依据：[Design3.md](../../../Design3.md)（本轮规则来源、Canonical Rule、快照和跨平台装配设计）
> - 现行基线：[Design2.md](../Design/Design2.md)、[Design2-UI.md](../Design/Design2-UI.md)
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 前一轮构建：[Build15.md](Build15.md)（已完成）
> - 编制日期：2026-08-31
>
> **执行原则：**
> - 每次仅执行一个 Step；完成本 Step 的测试、编译和差异检查并经用户确认后，再进入下一 Step。
> - 不并行实施多个 Step，不因顺手重构修改节点、代理组、Xray、权限或其他不在范围的模块。
> - 每个 Step 必须保持可编译、可测试；跨 Step 的临时兼容投影必须在对应收口 Step 删除。
> - Build16 不兼容保留旧素材池业务数据，但不得破坏已生成版本文件和历史 `render_plan_json`。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|----------|------|
| 1 | Canonical Rule 与中央能力注册表 | Design3 §三、§七 | ✅ 验收通过 |
| 2 | 单 URL 单适配器探测、解析与规范化 | Design3 §二、§四、§五 | ✅ 验收通过 |
| 3 | 1016 不兼容迁移、来源模型与素材池 CRUD | Design3 §三、§六、§八 | ✅ 验收通过 |
| 4 | per-source 快照同步、异常保护与 pending 操作 | Design3 §六 | ✅ 验收通过 |
| 5 | 装配目标过滤、转换回执与蓝图失效引用 | Design3 §七、§六.5 | ✅ 验收通过 |
| 6 | 素材池 API 与前端三模式交互 | Design3 §八 | ✅ 验收通过 |
| 7 | cron、数据清理、概览、备份与跨模块回归 | Design3 §九 | ✅ 验收通过 |
| 8 | 全量验证、文档同步与构建收口 | AGENTS.md §3.4～§3.6 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要（文件清单总览）

| Step | 主要涉及文件 | 要点 |
|------|--------------|------|
| 1 | `backend/internal/rulespec/spec.go`、新增 canonical/capability 文件及测试 | 统一语义、目标映射、范围计算、`no_resolve` 能力/实例分离 |
| 2 | `backend/internal/pool/parser.go`、新增 detector/adapters/normalize/testdata | 唯一适配器、三模式准入、PSL/IDNA、Mihomo domain/ipcidr/classical、sing-box、typed/CIDR 语料 |
| 3 | `backend/migrations/1016_rule_pool_snapshots.sql`、`backend/internal/pool/pool.go`、`backend/internal/store/store_test.go`、`backend/internal/server/pool.go` | 新 schema、旧数据清除、ID 防复用、来源对象和手工条目 CRUD |
| 4 | `backend/internal/pool/sync.go`、新增 snapshot 文件、`backend/internal/cron/pool.go` 及测试 | staging、active/pending、阈值、原子指针、诊断和任务状态 |
| 5 | `backend/internal/assembly/load.go`、`models.go`、`render*.go`、`service.go`、`blueprint.go`、`clash_plan.go`、`selfcheck.go`、`validate.go`、服务端装配测试 | 全量后端装配渲染统一迁移到 Canonical 注册表、动态目标过滤、回执、零输出口径、历史 plan 稳定和旧池引用失效 |
| 6 | `backend/internal/server/pool.go`、`assembly.go`、新增只读能力元数据端点、`frontend/src/api/rulespec.ts`、`pool.ts`、`assembly.ts`、`PoolTab.vue`、`PoolDetail.vue`、`RulesStep.vue`、`PreviewStep.vue`、`AssemblyView.vue` 及测试 | 三模式选择、格式/平台状态、徽标、pending 操作、能力元数据消费和预览回执 |
| 7 | `backend/internal/cron/pool.go`、`dataclear/dataclear.go`、`server/overview.go`、相关测试和测试迁移夹具 | 生命周期、清理顺序、概览、备份恢复后的迁移与旧表/旧能力真值表引用清除 |
| 8 | `Design3.md`、`Build16.md`、`AGENTS.md` 及必要的当前文档 | 全量门禁、实际结果、设计状态和文档登记 |

新增文件名可在保持职责不变的前提下按 Go 包结构微调；不得把探测、适配、快照和目标渲染重新集中到一个巨型文件。

---

## 三、构建顺序依赖图

```text
Step 1 中央语义/能力
  → Step 2 纯解析管线
    → Step 3 新 schema 与 CRUD
      → Step 4 快照同步
        → Step 5 装配目标过滤
          → Step 6 API/UI
            → Step 7 生命周期与跨模块回归
              → Step 8 全量验证和文档收口
```

依赖理由：

- 来源解析必须先产出稳定 Canonical Rule，数据库才能确定唯一键和列约束。
- 新 schema 必须先可用，快照同步才能建立 staging/active/pending 状态机。
- 装配回执需要活动规则查询稳定后实施。
- 前端最后消费已经稳定的 API，避免先做页面再反复迁移类型。

---

## 四、分步构建计划

### Step 1：Canonical Rule 与中央能力注册表

- **目标：** 建立规则语义和 Clash/SR 能力的后端唯一事实来源，为解析、手工素材、装配和前端元数据提供共同接口。
- **前置条件：** 只修改纯规则语义层；现有装配仍可通过临时兼容投影调用旧规则名，不能在本 Step 改数据库。
- **产出文件与操作：**
  - `backend/internal/rulespec/canonical.go`（新增）：定义 `Family`、`Matcher`、`CanonicalRule`、稳定 options 和 `SemanticKey()`。
  - `backend/internal/rulespec/capability.go`（新增）：定义 `Target`、`TargetScope`、`MappingResult`、`SupportsAndMap` 和只读前端元数据。
  - `backend/internal/rulespec/spec.go`：把现有 `Definitions` 收口为兼容投影；拆开 `SupportsNoResolve` 与规则实例的 `NoResolve`。
  - `backend/internal/rulespec/spec_test.go` 及新增测试：覆盖规范化键、双方能力、目标映射、依赖型规则排除和旧渲染解析兼容。
- **参考结构：**
  ```go
  type CanonicalRule struct {
      Family  Family
      Matcher Matcher
      Value   string
      Options RuleOptions
  }

  type MappingResult struct {
      Supported       bool
      RenderType      string
      ConversionKind  string
      SupportsNoResolve bool
  }
  ```
- **必须验证的能力：**
  - `DOMAIN`/suffix/keyword、CIDR、`IP-ASN` 为双方可用的基础映射。
  - `USER-AGENT` 为 `sr_only`。
  - 双方均不支持或依赖外部对象的规则不能进入素材池可选集合。
  - 注册表必须区分**素材池可选能力**与**高级装配 custom/overlay 能力**：`RULE-SET`、`AND/OR/NOT`、`MATCH` 等高级能力继续可用，但不能被素材池误选。
  - `SupportsNoResolve=true` 本身不会自动让规则 options 变成 `no_resolve=true`。
  - 后端元数据顺序稳定，前端无需复制静态规则表。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/rulespec/*.go
  cd backend && go test ./internal/rulespec
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：** rulespec 测试和全后端编译通过；现有装配仍可编译；能力与 Design3 的 `IP-ASN`/`USER-AGENT`/`no_resolve` 口径一致。

### Step 2：单 URL 单适配器探测、解析与规范化

- **目标：** 以纯函数管线实现详细格式探测、唯一适配器选择、Canonical Rule 提取、来源模式准入和限量诊断；尚不接入数据库活动同步。
- **前置条件：** Step 1 验收通过；保持现有同步入口可编译，使用新解析器的接入延后到 Step 4。
- **产出文件与操作：**
  - `backend/internal/pool/detector.go`：结构探测、候选评分、唯一适配器选择和硬错误码。
  - `backend/internal/pool/adapter_legacy.go`、`adapter_mihomo.go`、`adapter_typed.go`、`adapter_ip.go`、`adapter_singbox.go`：独立适配器；Mihomo YAML 的 domain/ipcidr/classical behavior 按整份 `payload` 唯一分类。
  - `backend/internal/pool/normalize.go`：IDNA、PSL、CIDR、ASN、通配 matcher 和来源准入。
  - `backend/internal/pool/parser.go`：缩减为管线编排或兼容入口，不保留“无逗号即 suffix”的旧事实。
  - `backend/internal/pool/testdata/` 或项目内文档夹具：四份 DailyData、Mihomo domain/ipcidr/classical、typed、CIDR、sing-box、HTML、冲突和混合私有语料。
  - `backend/internal/pool/parser_test.go`：表驱动正例、反例和误识别回归。
- **参考伪代码：**
  ```go
  format := DetectOne(body, sourceMode)
  candidates := Adapter(format).ParseWhole(body)
  canonical := NormalizeAndClassify(candidates, registry)
  result := ApplySourceMode(canonical, sourceMode)
  if result.HasConflictingPrivateScopes() { return ErrMixedPlatformSource }
  if !result.MeetsRecognitionThreshold() || result.Accepted == 0 { return ErrUnrecognizedSource }
  return result
  ```
- **禁止实现：**
  - 单条失败后调用另一个适配器；
  - 一个 YAML payload 混合 domain/classical/ipcidr behavior；
  - 自动模式合并 Clash 私有和 SR 私有规则；
  - sing-box 多条件 AND 扁平化；
  - 依赖型规则递归抓取。
- **测试矩阵：**
  - 三来源模式的通用/Clash 私有/SR 私有准入矩阵。
  - template3 在三模式下唯一识别为 `mihomo-ipcidr-yaml`，只接受 IPv4/IPv6 CIDR；template4 在三模式下识别为双方通用 `typed-rule-text`，保留 `IP-ASN` 与 `IP-CIDR`。
  - 双方私有同时出现、YAML+文本拼接、多个适配器不同语义均硬失败。
  - 小于 10条时 100%，10条以上 90%；模式剔除计入 recognized 但不计 accepted。
  - PSL 含 `co.uk`、PRIVATE `github.io`，以及 IDNA、单标签拒绝。
  - Mihomo `+.`、`.`、标签通配与 route wildcard 不误合并。
  - sing-box 单 family 多值展开，跨 family、invert、logical 整项拒绝。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/pool/*.go
  cd backend && go test ./internal/pool ./internal/rulespec
  cd backend && go test ./internal/pool -run 'TestDetect|TestParse|TestNormalize|TestSourceMode'
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：** 语料得到唯一且可解释的格式结果；所有异常输入直接失败；没有跨适配器逐条 fallback；旧错误页和裸文本误识别回归被覆盖。

### Step 3：1016 不兼容迁移、来源模型与素材池 CRUD

- **目标：** 替换旧素材池表结构，落地 pool/source/snapshot/canonical/origin/task 模型，完成结构化来源与手工素材 CRUD，但尚不启用完整 URL 快照同步。
- **前置条件：** Step 2 产出的 Canonical Rule 字段和语义键已经稳定；执行前再次核对所有引用旧表的 SQL。
- **产出文件与操作：**
  - `backend/migrations/1016_rule_pool_snapshots.sql`（新增）：捕获旧池最大 ID，按外键顺序删除旧素材池业务表，创建新表、索引、CHECK/UNIQUE 约束并设置 `sqlite_sequence`。
  - `backend/internal/store/store_test.go`：从 1015 旧 schema/数据升级，验证旧数据清除、新首个 ID 大于旧最大值、重复 URL/semantic key/指针约束。
  - `backend/internal/pool/pool.go`：`PoolSource`、`Entry`、范围统计、来源列表和手工 Canonical Rule CRUD。
  - `backend/internal/server/pool.go`：先完成新请求/响应结构和后端校验；前端适配在 Step 6。
  - `backend/internal/pool/pool_test.go`、`backend/internal/server/rule_test.go`：更新 CRUD 和分页测试。
- **参考数据关系：**
  ```text
  rule_pools 1─N rule_pool_sources
  rule_pool_sources 1─N pool_source_snapshots
  rule_pools 1─N pool_canonical_rules
  pool_canonical_rules N─N source/snapshot（pool_rule_origins）
  source.active_snapshot_id / pending_snapshot_id → 本 source 的 snapshot
  ```
- **迁移硬要求：**
  - 不转换旧 `urls_json`、manual/url `pool_entries` 或任务。
  - 新池 ID 从旧最大值之后开始；不能因清表回到 1 并碰撞旧蓝图 JSON。
  - migration 在单事务内执行；失败不留下半套 schema。
  - active/pending 指针必须只能引用同一 source 的快照，由事务服务和测试共同保证。
  - 活动条目查询只读 manual origin 或 source active snapshot origin。
- **阶段兼容：** Step 3 允许保留仅供编译的旧同步方法签名，但必须将其做成明确 no-op/返回“暂未支持”的桥接，且不得查询或写入已删除的旧 `pool_entries`/`urls_json`；Step 4 必须删除该桥接并接入新快照同步。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/pool/*.go internal/server/*.go
  cd backend && go test ./internal/store ./internal/pool ./internal/server -run 'Test.*Pool|Test.*Migration'
  cd backend && go build ./...
  cd backend && go vet ./internal/pool ./internal/server ./internal/store
  git diff --check
  ```
- **验收标准：** 旧 schema 可原子升级；旧业务数据不存在；新池 ID 不复用；来源对象、手工规则、分页、范围统计和 origins 查询正确；不存在写旧 `pool_entries` 的可达路径。

### Step 4：per-source 快照同步、异常保护与 pending 操作

- **目标：** 把 Step 2 解析管线接入异步任务，实现每个 URL staging → active/pending/failed 状态机和原子活动指针切换。
- **前置条件：** Step 3 schema/CRUD 验收通过；现有 60 秒、50 MB、50 URL、30 分钟任务、取消和启动恢复契约继续有效。
- **产出文件与操作：**
  - `backend/internal/pool/sync.go`：按提交时来源配置快照执行，不再按 URL 直接批量写活动条目。
  - `backend/internal/pool/snapshot.go`（新增）：staging 写入、阈值判断、active swap、pending 激活/丢弃和孤儿清理。
  - `backend/internal/pool/pool_test.go`、新增 `sync_test.go`：原子性、部分成功、取消、重启、阈值和并发测试。
  - `backend/internal/cron/pool.go`：按新来源/任务表提交定时同步，继续防止重复 running。
- **参考事务边界：**
  ```text
  网络与解析：事务外
  staging snapshot + canonical/origins：短事务
  anomaly 判断：基于旧 active + staging 统计
  active pointer swap / pending pointer set：BEGIN IMMEDIATE
  task/per-source aggregate：短事务
  ```
- **异常规则：**
  - old active ≥20 且 new accepted <70% → pending。
  - format/profile 变化 → pending。
  - 首次合法同步 → active。
  - mixed/format conflict/ambiguity、HTML、零 accepted、截断或解析错误 → failed，不创建可激活 pending。
  - pending 激活/丢弃必须校验 source ID、snapshot ID 和当前 pointer，防止陈旧操作。
- **诊断要求：** 每 URL 最大 20条、每条 200字符；URL 查询参数和 Token 脱敏；不保存完整响应；任务 7天清理不删除 active/pending。
- **原子性测试：** 使用阻塞/失败 fetcher 或事务钩子证明 staging 中途不可被 `ListEntries`/assembly 读到；切换后同一次查询只能看到完整旧或完整新集合。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/pool/*.go internal/cron/*.go
  cd backend && go test -race ./internal/pool ./internal/cron
  cd backend && go test ./internal/pool -run 'Test.*Snapshot|Test.*Pending|Test.*Atomic|Test.*Threshold'
  cd backend && go build ./...
  cd backend && go vet ./internal/pool ./internal/cron
  git diff --check
  ```
- **验收标准：** 每个 URL 独立原子切换；池级 partial 不产生单来源半结果；所有硬失败保留旧 active；有效异常进入 pending；限量诊断和 7天清理正确。

### Step 5：装配目标过滤、转换回执与蓝图失效引用

- **目标：** 装配器改读活动 Canonical Rule，通过中央注册表为 Clash/SR 输出并返回可核对回执；同时把全部装配渲染路径统一迁移到 Canonical 注册表，不再保留 `Definitions`/`SR bool` 双轨，并确保旧池 ID 不会静默重绑。
- **前置条件：** Step 4 活动视图和排序稳定；保留现有预览指纹、`previewStale`、强制代理组和自包含 render plan 行为；确认 `clash_plan.go`、`selfcheck.go`、`validate.go` 等所有调用方均已纳入迁移范围。
- **产出文件与操作：**
  - `backend/internal/assembly/load.go`：查询活动 origins 去重视图，读取 Canonical Rule 和稳定顺序。
  - `backend/internal/assembly/models.go`、`service.go`：增加 `ConversionReceipt` 并纳入预览/生成结果及预览指纹需要的快照标识。
  - `backend/internal/assembly/render_clash.go`、`render_sr.go`：调用 `SupportsAndMap`，删除 `USER-AGENT` 等散落特判。
  - `backend/internal/assembly/clash_plan.go`：render plan 重建、降级、`no-resolve` 追加全部改为读取 Canonical 注册表能力，删除对 `Definitions` 的依赖。
  - `backend/internal/assembly/selfcheck.go`、`validate.go`：规则类型校验、高级自定义规则校验改为走 Canonical/advanced 能力标记，不再直接查 `Definitions` 或 `SR bool`。
  - `backend/internal/rulespec/spec.go`：确认所有调用方迁移完成后删除 Step 1 的临时 `Definitions`/`SR bool` 兼容投影；高级装配需要的 `RULE-SET`/`AND`/`OR`/`NOT`/`MATCH` 等能力由中央注册表以“advanced-only”标记承载，不再维护独立静态真值表。
  - `backend/internal/assembly/blueprint.go`：旧池引用返回明确 invalid reference；历史 `render_plan_json` 仍自包含可下载。
  - 相关 assembly/server/download 测试：双目标、回执、零输出、旧蓝图、历史下载，以及 `clash_plan`/`selfcheck`/`validate` 的 Canonical 迁移回归。
- **参考回执：**
  ```json
  {
    "input": 120,
    "direct_output": 100,
    "equivalent_conversions": 4,
    "skipped_unsupported": 16,
    "target_validation_failed": 0,
    "final_output": 104
  }
  ```
- **生成门槛：** `final_output` 只统计**素材池规则 + 自定义规则**，不包含内置 `GEOIP,CN,DIRECT`、`MATCH`、`FINAL` 等系统兜底；`final=0` 阻止生成；`final>0` 即使存在 skip 也允许，但必须在预览明确警告，不增加二次确认框。
- **回归边界：** 素材池不接受依赖规则不能影响高级装配现有 `RULE-SET`/overlay/custom rule；高级能力与素材池能力在中央注册表中通过能力标记隔离；强制代理组、节点命名和版本生成逻辑不变。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/assembly/*.go internal/server/*.go internal/rulespec/*.go
  cd backend && go test ./internal/assembly ./internal/server
  cd backend && go test ./internal/assembly -run 'Test.*Canonical|Test.*Receipt|Test.*Target|Test.*Blueprint'
  cd backend && go build ./...
  cd backend && go vet ./internal/assembly ./internal/server
  git diff --check
  ```
- **验收标准：** `IP-ASN` 双目标、`USER-AGENT` SR-only、目标跳过和等价转换统计正确；`final_output` 按“排除内置兜底”口径统计，零输出被阻止，非零跳过可生成；`clash_plan.go`/`selfcheck.go`/`validate.go` 等全部调用方已迁移，后端不存在 `Definitions`/`SR bool` 运行时依赖；高级 `RULE-SET`/overlay/custom rule 回归通过；旧蓝图明确失效，历史生成内容不漂移。

### Step 6：素材池 API 与前端三模式交互

- **目标：** 完成新 API 的前端接入，提供每 URL 三模式选择、检测结果、范围徽标、pending 操作和装配转换回执；同时提供中央能力注册表只读元数据端点，移除前端静态规则类型真值表。
- **前置条件：** Step 5 后端响应稳定；Step 1 的 Canonical/advanced 能力数据已就绪；不得在前端复制中央能力矩阵。
- **产出文件与操作：**
  - `backend/internal/server/rulespec.go`（新增或并入 `pool.go`）：只读能力元数据端点，返回素材池可选能力、高级装配能力、范围徽标、目标支持与稳定顺序元数据。
  - `frontend/src/api/rulespec.ts`（新增）：能力元数据 API 类型与请求。
  - `frontend/src/api/pool.ts`：`PoolSource`、`SourceMode`、snapshot/result/diagnostic、范围统计和 pending API。
  - `frontend/src/api/assembly.ts`：转换回执类型。
  - `frontend/src/views/admin/assembly/PoolTab.vue`：每 URL 的 URL 输入 + 三模式选择、帮助、保存后待同步状态。
  - `PoolDetail.vue`：检测格式/平台、active/pending、统计、诊断、范围徽标、origins、激活/丢弃；手工类型下拉从能力元数据端点生成。
  - `RulesStep.vue`：池对当前目标的可输出数、零输出禁用；手工与自定义规则类型均从后端元数据生成。
  - `AssemblyView.vue`：删除 `RULE_TYPES`/`CLASH_RULE_TYPES`/`SR_RULE_TYPES` 静态真值表，统一使用能力元数据。
  - `PreviewStep.vue`/`AssemblyView.vue`：回执、跳过警告和 stale 状态。
  - `frontend/tests/pool-tab.spec.ts`、`pool-detail.spec.ts`、`rules-step.spec.ts`、`assembly-view.spec.ts`、后端元数据端点测试：交互、状态和能力口径回归。
- **参考状态映射：**
  ```text
  source_mode（用户输入）
    + detected_format/profile（最近快照）
    + active/pending/failed（来源状态）
    → PoolTab 摘要
    → PoolDetail 诊断与 pending 操作
    → RulesStep 目标可用数
    → PreviewStep 转换回执
  ```
- **交互要求：**
  - 只出现“Clash 规则源”“SR 规则源”“我不确定”，不暴露详细适配器下拉框。
  - 自动模式明确告知可能失败，不承诺修复混合来源。
  - URL/mode 变化保存前提示旧活动素材会停止参与，并在保存后展示待同步。
  - pending 确认弹窗展示旧/新数量、格式、平台和差异；硬失败没有强制激活入口。
  - 桌面和 `<768px` 手机布局都能操作来源选择、诊断和 pending 按钮；触控目标沿用现有 Token/组件规范。
- **测试与验收命令：**
  ```bash
  cd frontend && npm test -- --run pool-tab.spec.ts pool-detail.spec.ts rules-step.spec.ts assembly-view.spec.ts
  cd frontend && npm run build
  git diff --check
  ```
- **验收标准：** 后端提供只读能力元数据端点；新建/编辑来源、三模式提示、同步状态、范围统计、pending 操作和装配回执完整；前端所有规则类型下拉（手工素材、自定义规则）均来自能力元数据；源码中无 `RULE_TYPES`/`CLASH_RULE_TYPES`/`SR_RULE_TYPES` 静态真值表；窄屏无横向不可达操作。

### Step 7：cron、数据清理、概览、备份与跨模块回归

- **目标：** 清理所有旧表/字段假设，保证定时任务、清空数据、管理员概览、备份恢复和测试夹具适配新 schema。
- **前置条件：** Step 6 主链路可用；先用 `rg` 建立所有 `rule_pools/pool_entries/pool_sync_tasks/urls_json` 以及 `rulespec.Definitions`/`NoResolve`/`SR bool`/前端 `RULE_TYPES` 的引用清单，逐项判定而非盲目替换。
- **产出文件与操作：**
  - `backend/internal/cron/pool.go`、`pool_test.go`：新来源调度、防重、无活动来源和 pending 场景。
  - `backend/internal/dataclear/dataclear.go`、测试：按外键顺序清理 origins/rules/snapshots/sources/tasks/pools，并验证精确表集合。
  - `backend/internal/server/overview.go`、测试：池数和相关概览不依赖旧 `pool_entries`。
  - `backend/internal/backup/backup_test.go` 或 store 恢复测试：SQLite 全库备份仍包含新表，恢复启动可迁移/读取。
  - emergency/download/version/xray 等自建旧迁移 fixture：更新最低必要 schema，避免测试继续伪造旧表。
  - 若配置导入导出实际包含素材池，则升级格式并明确不兼容；若不包含，以测试固定“不新增隐式导出”的现状。
  - 能力元数据/装配相关测试：确认 Step 5 全量迁移后不再存在 `Definitions`/`SR bool` 等旧能力真值表的运行时依赖。
- **参考核查流程：**
  ```text
  rg 建立旧引用清单
    → 按 migration history / test fixture / runtime 分类
    → runtime 命中迁移到新表或中央注册表
    → fixture 对齐真实 1016 schema
    → 再次 rg，仅允许历史迁移和明确的旧版本升级语料
  ```
- **核查命令：**
  ```bash
  rg -n 'pool_entries|urls_json|RuleDef.*SR|CLASH_RULE_TYPES|SR_RULE_TYPES|RULE_TYPES|rulespec\.Definitions|rulespec\.NoResolve|\.SR\b' backend frontend
  ```
  命中项必须是迁移历史、明确兼容投影、旧版本升级语料或已由中央能力注册表替代的临时命名；运行时代码不得继续依赖旧表/旧能力真值表。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/cron/*.go internal/dataclear/*.go internal/server/*.go
  cd backend && go test ./internal/cron ./internal/dataclear ./internal/server ./internal/backup ./internal/store
  cd backend && go build ./...
  cd backend && go vet ./...
  git diff --check
  ```
- **验收标准：** cron、清理、概览和备份正常；运行时代码不再查询旧表/字段，且不存在 `Definitions`/`SR bool`/静态 `RULE_TYPES` 等旧能力真值表的运行时依赖；测试夹具与真实迁移一致；旧蓝图和历史下载回归仍通过。

### Step 8：全量验证、文档同步与构建收口

- **目标：** 运行项目完整质量门禁，记录实际结果并同步当前设计/构建清单；本 Step 不新增功能。
- **前置条件：** Steps 1～7 均已验收；任何失败先修复对应 Step，不以更新文档掩盖失败。
- **产出文件与操作：**
  - `Design3.md`：将实现相关描述核对为真实落点，保持设计与代码一致。
  - `Build16.md`：逐 Step 更新状态、实际文件、命令和验收结果。
  - `AGENTS.md`：仅在 Build16 全部完成后登记当前 Build16 状态。
  - 如实现中发现真实 bug，按文档体系决定是否新增/更新 Issue；不得把新问题只写在 Build 结果里。
- **参考收口流程：**
  ```text
  Steps 1-7 已验收
    → 全量自动门禁
    → 人工主链路/异常路径/窄屏核验
    → 核对 Design3 与真实代码
    → 记录实际结果
    → 最后更新 AGENTS.md 和 Build16 状态
  ```
- **自动验证命令：**
  ```bash
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test -timeout 180s ./...
  cd frontend && npm test -- --run
  cd frontend && npm run build
  docker compose build
  git diff --check
  ```
- **人工/集成核验：**
  - 以 Clash、SR、自动三模式导入正常来源。
  - 以三模式导入 template3/template4，确认来源模式只调整识别优先级/范围，不把 Canonical 内容绑定到输出平台；验证 IPv6 CIDR 分别输出正确目标类型。
  - 验证通用 + `USER-AGENT` 在 auto 识别为 SR、选择 Clash 时仅通用素材进入。
  - 验证双方私有混合、HTML、结构拼接和 sing-box 复杂条件直接失败。
  - 验证异常缩量/格式变化进入 pending，旧 active 继续参与装配。
  - 验证 Clash/SR 预览数量和下载正文一致，历史版本下载不变化。
  - 在窄屏检查 URL 来源行、同步诊断、pending 操作和预览回执。
- **验收标准：** 所有自动门禁通过，人工主链路与错误路径符合 Design3；文档只记录实际结果；Build16 才可标记完成。

---

## 五、固定决策与范围清单

以下内容已经由用户确认，执行时不得重新变成待定项：

| # | 固定决策 |
|---|----------|
| 1 | UI 每 URL 只有 Clash、SR、“我不确定”三种模式；详细格式由系统识别并展示。 |
| 2 | 单 URL 只允许一个主适配器，不做逐行跨解析器 fallback；异常/混合 URL 可直接报错。 |
| 3 | 选择 Clash/SR 影响解析优先级和来源准入，但不锁定最终装配目标。 |
| 4 | 双方通用素材可用于两个目标；`USER-AGENT` 为 SR 私有；`IP-ASN` 为通用。 |
| 5 | 不添加目标原生依赖型素材规则，不影响高级装配已有独立功能。 |
| 6 | sing-box 仅 auto 检测简单单 family 子集；AND/invert/logical 整项拒绝。 |
| 7 | 少于 10条需 100% 识别，其他至少 90%；旧值 ≥20且新值 <70% 或格式/平台变化进入 pending。 |
| 8 | 诊断最多 20条×200字符，不保存完整响应，任务保留 7天。 |
| 9 | 目标最终输出为 0 阻止生成；`final_output` 只统计素材池规则与自定义规则，不包含内置 `GEOIP`/`MATCH`/`FINAL` 等系统兜底；非零但有跳过时允许并警告，不增加第二确认框。 |
| 10 | 不兼容旧素材池业务数据；旧 ID 不复用；历史版本和自包含 render plan 保留。 |
| 11 | 不锁定 Shadowrocket 版本，能力以项目注册表和回归语料维护。 |
| 12 | 所有装配渲染（含 `clash_plan.go`/`selfcheck.go`/`validate.go` 与前端规则类型下拉）统一迁移到 Canonical 中央注册表；高级 `RULE-SET`/`AND`/`OR`/`NOT`/`MATCH` 能力以 advanced-only 标记隔离，不进入素材池可选集合。 |
| 13 | Step 6 提供中央能力注册表只读元数据端点；前端不再维护 `RULE_TYPES`/`CLASH_RULE_TYPES`/`SR_RULE_TYPES` 静态真值表。 |
| 14 | 来源模式只调整识别器优先级/范围与既有准入，不绑定 Canonical 内容或最终输出平台；Mihomo ipcidr YAML 只接受 IPv4/IPv6 CIDR，SR 显式 IP 文本继续复用双方通用 typed 适配器。 |

明确不在 Build16 范围：

- hosts、AdGuard/Adblock、dnsmasq、Surge/Loon 等后续适配器；
- 二进制 `.mrs`/`.srs`；
- 对错误 URL 的混合平台抢救；
- 节点、代理组、Xray、认证、权限或分发架构重构；
- 为兼容旧 API 保留 `urls: string[]` 或旧素材池数据转换。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.3 | 2026-08-31 | 补充 template3/template4 来源识别：新增严格的 `mihomo-ipcidr-yaml`、整份 payload behavior 冲突检查、IPv4/IPv6 CIDR 规范化与动态目标类型；SR 显式 IP 文本继续作为双方通用 `typed-rule-text`。后端 build/vet/全量 test、前端 35 个文件 126 项测试与生产构建、Docker Compose 镜像构建及 `git diff --check` 均通过。 |
| v1.2 | 2026-08-31 | 完成八个 Step 实施：新增 1016 迁移、Canonical/能力注册表、单来源解析、快照同步、装配回执、能力元数据端点与前端三模式入口；后端 build/vet/test、前端 test/build 均通过。 |
| v1.1 | 2026-08-31 | 按用户确认补充执行口径：Step 5 全量迁移所有装配渲染到 Canonical 注册表并隔离 advanced-only 能力；Step 6 新增只读能力元数据端点并彻底移除前端静态规则类型真值表；`final_output` 只统计素材池+自定义规则、排除内置兜底；Step 7 扩展旧能力真值表清理核查。 |
| v1.0 | 2026-08-31 | 初始构建计划：依据 Design3 已确认设计拆分中央能力、单来源解析、新 schema、快照同步、装配回执、API/UI、生命周期回归和全量收口八个 Step；全部尚未开始。 |
