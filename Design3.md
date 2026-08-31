# Design3.md — VPN 订阅管理系统增量设计（规则来源识别、结构化素材与跨平台装配）

> **文档定位：** 本文定义规则素材池下一阶段设计：管理员为每个 URL 选择 Clash 规则源、Shadowrocket（下文简称 SR）规则源或“我不确定”，系统以单 URL 单主方言为边界识别格式、提取平台无关规则、形成可追踪快照，再由 Clash/SR 目标适配器过滤和渲染。本文承接 [Design2.md](Design2.md) 第二～四章；第一期基线见 [Design1.md](docs/reports/Design/Design1.md)。编码约束遵循 [AGENTS.md](AGENTS.md)（**唯一强要求**）。
> **设计状态：** 截至 2026-08-31，本设计已经完成研究和用户决策，作为 [Build16.md](Build16.md) 的设计依据；Build16 完成前，当前运行事实仍以 Design2 与现有代码为准。
> **范围边界：** 本期只重构“规则素材 URL/手工素材 → Canonical Rule → Clash/SR 渲染”链路，不重定义节点、代理组、装配版本、订阅分发、Xray 或权限体系。

---

## 一、背景、目标与原则

### 1.1 当前问题

当前 URL 内容仍以逐行解析为主：`full:` 被识别为完整域名，不含逗号的文本一律当作域名后缀，含逗号的规则直接保存为 `rule_type + match_value`。多 URL 同步还会分批写入活动 `pool_entries`，后续来源失败时可能留下部分新数据；后端和前端分别维护目标平台类型列表，能力口径也会漂移。

该方式不能可靠区分 Mihomo domain/classical YAML、sing-box source JSON、显式类型文本和纯 CIDR 列表，也可能把 YAML 键、HTML 错误页或完整子域名误识别。以 [DailyData.txt.template2.md](docs/DocTemplates/DailyData.txt.template2.md) 为例：

```yaml
payload:
  - 'a1.mzstatic.com'
  - '+.001wifi.com'
```

在 Mihomo domain provider 中，前者是完整域名，后者是覆盖主域及子域的后缀。素材必须先按整份文档格式解释，不能只看单行外观。

### 1.2 设计目标

1. 每个 URL 提供 `clash`、`shadowrocket`、`auto` 三种来源模式。
2. 自动识别只服务正常的单平台 URL；异常或混合 URL 可以直接报错。
3. 分离源文档语法、规则语义、来源证据和目标平台语法。
4. 让素材池同时携带通用、Clash 私有、SR 私有素材，并在装配时正确过滤。
5. 以来源快照隔离同步过程，完整校验后才原子切换活动数据。
6. 让来源准入、手工编辑、列表徽标和目标渲染共享中央能力注册表。
7. 分别展示解析错误、来源模式剔除、重复、目标不支持和等价转换。
8. 不兼容地替换旧素材池业务数据，但保留已生成历史版本。

### 1.3 术语

| 术语 | 含义 |
|------|------|
| `source_mode` | 管理员选择的 `clash`、`shadowrocket` 或 `auto` |
| `detected_format` | 整份文档的详细格式，如 `mihomo-domain-yaml` |
| `detected_profile` | 解析结果的平台特征：`common`、`clash`、`shadowrocket`；不存在合法“双平台”值 |
| Canonical Rule | 与源/目标平台字符串名称解耦的规则语义 |
| Rule Origin | Canonical Rule 来自哪个手工来源或 URL 快照的位置证据 |
| active snapshot | 当前参与素材查询和装配的某 URL 完整结果 |
| pending snapshot | 语法有效但触发格式变化或缩量保护、尚未替换 active 的结果 |

### 1.4 核心原则

- **单 URL、单主方言：** 每次同步只选择一个详细格式适配器解释整份文档。
- **自动识别不负责抢救异常输入：** 无法可靠归类、结构冲突或双方私有语义混合时直接失败。
- **显式语义优先：** `full:`、`+.`、`DOMAIN,` 等有效标记覆盖 PSL 推断。
- **文档级识别优先：** 先识别 JSON/YAML/typed text/plain list，再解释条目。
- **来源选择不锁定输出：** Clash 来源中的通用素材仍可用于 SR，反之亦然。
- **不做有损偷换：** 不把子域通配、正则、AND 或逻辑规则扩大为简单规则。
- **快照完整后再可见：** staging 不能影响活动素材；单个 URL 只能原子切换。
- **中央能力注册表是唯一事实来源：** 平台范围不是独立可编辑字段。
- **错误可见且边界简单：** 不为不应出现的混合 URL 建立逐行 fallback。

### 1.5 非目标

- 不承诺转换任意网络文本，不读取二进制 `.mrs`/`.srs`。
- 不把节点订阅、完整 Clash 配置或 SR 节点订阅当作规则素材。
- 不执行脚本或递归下载 `RULE-SET`、`DOMAIN-SET`、provider 等依赖。
- 素材池不引入依赖型规则、终结规则或目标原生引用。
- 不移除高级装配中独立存在的 `RULE-SET` 能力，两条链路保持隔离。
- 不锁定某个 SR 版本；项目能力注册表随项目验证的客户端要求更新。
- 不兼容迁移旧素材池、URL、手工条目或同步任务业务数据。

---

## 二、总体架构与单来源状态机

### 2.1 数据链路

```text
URL 配置（url + source_mode + order）
  → HTTP/内容安全检查
  → 文档级探测与候选评分
  → 选择唯一详细格式适配器
  → 整份文档解析
  → Canonical Rule + Rule Origin
  → 规范化与中央能力分类
  → 来源模式准入
  → 语义去重与完整性检查
  → staging snapshot
  → 失败 / pending / 原子激活
  → 素材池活动视图
  → Clash/SR 目标过滤
  → 目标语法渲染 + 转换回执
```

装配器不得重新读取或解释原始 URL。完整响应只在本次同步内存中使用，不持久化正文。

### 2.2 分层职责

| 层级 | 职责 |
|------|------|
| 拉取层 | 60 秒单 URL 超时、50 MB 上限、状态码、重定向、安全和错误页检查 |
| 探测层 | 根据结构、内容和 `source_mode` 选出唯一格式或硬错误 |
| 源适配层 | 只解释已选格式，输出 Canonical Rule 候选和逐项诊断 |
| 清洗/准入层 | IDNA、PSL、CIDR、正则、选项校验、能力分类和来源模式过滤 |
| 快照层 | staging 隔离、阈值检查、active/pending 指针和来源追踪 |
| 素材查询层 | 聚合 active origins，语义去重并稳定排序 |
| 目标适配层 | Clash/SR 能力检查、等价转换、跳过和渲染回执 |

### 2.3 单 URL 单主方言

探测器可让多个候选评分，但一次同步最终只能：

1. 由一个适配器解释整份文档；
2. 对双方通用的纯文本/显式规则识别为 `common`；
3. 不同适配器产生不同语义时返回 `ambiguous_source_format`；
4. 同一文档包含冲突结构时返回 `conflicting_document_format`；
5. 同时出现 Clash 私有和 SR 私有语义时返回 `mixed_platform_source`；
6. 无格式达到要求时返回 `unrecognized_source`。

禁止将某条失败规则交给第二个适配器、分别解析拼接的 YAML/文本片段，或在自动模式分别收集双方私有内容。“通用 + 某一个平台私有”不是混合来源，例如 `DOMAIN-SUFFIX` 与 `USER-AGENT` 应识别为 SR 来源并全部接受。

### 2.4 来源配置变化

- 同池 URL 规范化后不得重复，不允许用不同模式重复添加相同 URL。
- 修改 URL 或 `source_mode` 视为语义变化，旧 active 立即停止参与素材池。
- 保存后自动提交同步，成功前显示“待同步/无活动快照”。
- 仅调整顺序不重新下载，但原子更新活动查询顺序。

---

## 三、Canonical Rule、来源证据与能力注册表

### 3.1 Canonical Rule

| 字段 | 含义 | 示例 |
|------|------|------|
| `family` | 匹配对象 | `domain`、`ip`、`user_agent`、`process`、`network`、`port`、`geo` |
| `matcher` | 匹配方式 | `exact`、`suffix`、`keyword`、`route_wildcard`、`subdomain_only`、`provider_label_wildcard`、`regex`、`cidr`、`asn`、`equals` |
| `value` | 规范化值 | `a1.mzstatic.com`、`1.2.3.0/24`、`13335` |
| `options` | 真实匹配选项 | `no_resolve=true`，不含源 policy |
| `semantic_key` | 稳定去重键 | family、matcher、value、稳定 options 的编码/摘要 |

`domain/exact/a.example.com` 与 suffix 是不同规则；`full:a.example.com`、`DOMAIN,a.example.com` 和 Mihomo domain YAML 裸条目归一为相同 exact；`+.` 与 `DOMAIN-SUFFIX` 归一为 suffix。大小写、外层引号、源平台类型名和 JSON 属性顺序不改变语义键。

### 3.2 Rule Origin

Rule Origin 独立保存 `source_id`、可空 `snapshot_id`、原始行号/JSON 路径、URL 顺序、来源内顺序和原始条目摘要。多个来源产生相同 `semantic_key` 时只渲染一个规则，但保留全部 active origins；删除一个来源后，其他来源仍存在则规则继续有效。

### 3.3 中央能力注册表

现有 `RuleDef.SR bool` 改为同时表达规范化校验、目标支持、目标类型、完全等价转换和选项能力的注册表，概念接口为：

```go
func SupportsAndMap(rule CanonicalRule, target Target) MappingResult
```

注册表同时服务来源准入、手工素材类型、范围徽标、装配过滤、目标渲染和前端元数据。`supports_no_resolve` 与某条规则实际设置的 `no_resolve` 必须分离，禁止因“支持”而自动附加。

动态范围为：

| Clash | SR | `target_scope` |
|-------|----|----------------|
| 支持 | 支持 | `common` |
| 支持 | 不支持 | `clash_only` |
| 不支持 | 支持 | `sr_only` |
| 不支持 | 不支持 | 拒绝进入素材池 |

`USER-AGENT` 是 SR 私有示例；`IP-ASN` 在双方均有对应能力，应为通用。能力变更必须同步更新目标语料和映射测试。

### 3.4 手工素材

- 手工类型取中央注册表的双方能力并集，并动态展示范围。
- 手工条目也转换为 Canonical Rule，不执行 PSL 推断。
- 依赖型、终结型和双方都不能作为独立素材表达的规则不可选。
- 手工来源不需要 URL 快照，但参与相同去重、排序和目标过滤。

---

## 四、来源模式、探测与适配器

### 4.1 三种来源模式

| 实际内容 | Clash 规则源 | SR 规则源 | 我不确定 |
|----------|---------------|-----------|----------|
| 只有通用规则 | 接受 | 接受 | 识别为通用并接受 |
| 通用 + Clash 私有 | 全部接受 | 接受通用，私有项记模式剔除 | 识别为 Clash 并接受 |
| 通用 + SR 私有 | 接受通用，私有项记模式剔除 | 全部接受 | 识别为 SR 并接受 |
| 双方私有同时出现 | 硬失败 | 硬失败 | 硬失败 |
| 冲突结构或无法可靠识别 | 硬失败 | 硬失败 | 硬失败 |

`source_mode` 既影响候选适配器优先级，也在 Canonical Rule 形成后执行准入；它不是输出目标，也不能绕过结构校验。剔除后 accepted 为 0 时同步失败。

### 4.2 探测与识别率

探测顺序：错误页/二进制检查 → 已知 JSON → 已知 YAML → typed text → IP/CIDR → legacy domain → plain domain。结构化格式必须完整解析，不能把键名送入裸文本适配器。

候选分母不含空行、合法注释和结构键。固定阈值为：

- 少于 10 条时必须 100% 被识别；
- 10 条及以上至少 90%；
- `excluded_by_source_mode` 属于已识别；
- 结构冲突、双方私有混合和语义歧义始终硬失败；
- 来源准入后 accepted 必须大于 0。

### 4.3 首期适配器

| 格式 | 语义 |
|------|------|
| `legacy-domain-text` | `full:` 为 exact；裸域名按 §5.2 PSL 推断 |
| `plain-domain-text` | 按 §5.2 推断 exact/suffix |
| `mihomo-domain-yaml` | 严格按 provider domain 语义 |
| `mihomo-classical-yaml` | `payload` 中按显式类型解释 |
| `typed-rule-text` | 按显式类型解释，policy 不进入素材 |
| `plain-ipcidr-text` | IP 转单主机 CIDR，CIDR 归一网络地址 |
| `sing-box-source-json` | 仅 `auto` 识别首期简单子集 |

详细格式只作为同步结果展示，不作为 UI 选择器。

### 4.4 各适配器边界

**Legacy/纯域名：** 只有整份文档已判定为域名文本时，裸域名才使用 PSL。显式 `full:`、`+.`、`DOMAIN` 和 `DOMAIN-SUFFIX` 优先。

**Mihomo provider：** 一个文档只允许一种 behavior；YAML 完整解析并只读取 `payload`。domain provider 裸域名为 exact，`+.` 为 suffix，`.` 保留仅子域语义。provider 标签通配与 route `DOMAIN-WILDCARD` 不可混同。冲突 behavior、异常嵌套和非字符串条目使文档失败。

**显式类型文本：** `DOMAIN`、`DOMAIN-SUFFIX`、`DOMAIN-KEYWORD`、`IP-CIDR/6`、`IP-ASN`、`USER-AGENT` 等先映射 Canonical Rule，再由注册表分类。源 policy 忽略但计入诊断；只有源实际声明时才保存 `no_resolve`。未知或依赖型类型拒绝。

**sing-box：** 仅读取 source JSON，校验 `version` 和 `rules`。只接受单个条件 family 的简单 default rule；同一字段多值按 OR 展开。多个不同条件字段、`invert=true`、logical rule 或无法证明等价时整项拒绝，不能把 AND 扁平化。action/route target 不进入素材池。

hosts、AdGuard、dnsmasq、Surge/Loon 变体和其他格式不属于 Build16；未来必须以独立适配器和正反语料接入。

---

## 五、规范化、通配语义与结果分类

### 5.1 域名与 PSL

域名依次去空白/合法尾点、按 IDNA lookup 转 ASCII、转小写、校验标签和长度，再确定 matcher。关键字、通配和正则使用各自校验器。

仅 plain/legacy 裸域名使用包含 PRIVATE 区段的 Public Suffix List eTLD+1：

1. 输入等于 eTLD+1 → suffix；
2. eTLD+1 前还有标签 → exact；
3. 单标签或无法可靠计算 → 拒绝；
4. 显式格式始终覆盖。

| 输入 | 结果 |
|------|------|
| `mzstatic.com` | suffix |
| `a1.mzstatic.com` | exact |
| `example.co.uk` | suffix |
| `www.example.co.uk` | exact |
| `foo.github.io`（PRIVATE） | suffix |
| `www.foo.github.io` | exact |
| `localhost` | 拒绝 |

实现复用 `golang.org/x/net/publicsuffix` 和 `idna`；依赖升级必须运行固定分类语料。

### 5.2 通配、IP 与 ASN

至少区分 `route_wildcard`、`subdomain_only`、`provider_label_wildcard` 和 `regex`。`+.` 可规范化为 suffix；`.`、provider `*` 和 route wildcard 不得仅改名互换。只有经测试证明完全等价的转换才能执行并计入回执。

IPv4/IPv6 单地址转 `/32`/`/128`，CIDR 归一网络地址；两者统一保存为 `ip/cidr`，目标渲染器选择类型名。ASN 接受正整数，适配器可显式允许 `AS` 前缀。IP 类不进入 PSL。

### 5.3 处理结果

| 状态 | 含义 | 活动素材 |
|------|------|----------|
| `accepted` | 合法且通过来源模式 | 是 |
| `excluded_by_source_mode` | 合法但被用户选择剔除 | 否，仅回执 |
| `rejected_at_source` | 结构、类型、值或依赖不合法 | 否 |
| `deduplicated` | 语义重复 | 不新增规则，保留 origin |
| `unsupported_for_target` | 素材合法但当前目标不能表达 | 素材保留，渲染时跳过 |

清洗顺序固定为“提取 → 类型和值校验 → 规范化 → 能力分类 → 来源准入 → 语义去重”。不同 matcher 不能合并。

### 5.4 排序

手工来源在前；URL 按配置顺序；来源内按原始位置和展开顺序。多来源重复使用最早 active origin 排序；某来源消失后按剩余最早 origin 稳定排序。

---

## 六、快照、同步保护与迁移

### 6.1 概念数据模型

| 表/实体 | 职责 |
|---------|------|
| `rule_pools` | 池名称、定时设置和聚合状态，不再保存 `urls_json` |
| `rule_pool_sources` | manual/url 来源、URL、模式、顺序、配置修订和 active/pending 指针 |
| `pool_source_snapshots` | 格式、平台、统计、状态和时间 |
| `pool_canonical_rules` | 池级规则实体和唯一 `semantic_key` |
| `pool_rule_origins` | 规则与手工来源/URL 快照的多对多证据 |
| `pool_sync_tasks` | 池级异步任务和限量逐 URL 回执 |

活动查询只选择手工 origins，以及 `origin.snapshot_id = source.active_snapshot_id` 的 URL origins。staging、pending、failed 不可被装配器读取。

staging 可以复用或创建池级 Canonical Rule，但只要它没有手工 origin 或 active snapshot origin 就不可见；failed/pending 丢弃后必须垃圾回收无任何有效 origin 的孤立 Canonical Rule，避免候选数据长期膨胀。

### 6.2 单 URL 原子激活

```text
fetching → parsing → staging
  ├─ hard failure → failed（旧 active 不变）
  ├─ valid + anomaly → pending（旧 active 不变）
  └─ valid + normal → active pointer swap
```

指针切换、旧 active 解除和聚合状态在一个 `BEGIN IMMEDIATE` 事务完成。池级任务允许部分成功，但单 URL 不得部分可见。pending 只允许对语法有效结果“激活/丢弃”；混合平台、结构冲突、格式歧义和结构不完整不可人工强制激活。

### 6.3 异常保护（已确认）

- 上次 active 至少 20 条且新 accepted 少于旧值 70% → pending；
- `detected_format` 或 `detected_profile` 改变 → pending；
- 空响应、HTML/登录页、零 accepted、截断响应、结构冲突和解析器错误 → failed；
- 首次同步达到识别率且 accepted 大于 0即可 active；
- pending 激活记录管理员操作时间，不修改解析结果。

### 6.4 回执、诊断与保留

逐 URL 回执包含来源模式、详细格式、平台、置信依据、input/recognized/accepted/excluded/rejected/duplicates、family/matcher/范围统计、前后数量和格式变化、最终状态与原因。

最多保留 20 条代表性诊断，每条 200 字符；不保存完整响应；URL 查询凭据、Token 和疑似凭据必须脱敏。完成任务和诊断保留 7 天，active/pending 不受任务清理影响。现有 URL 数量 50、单 URL 60 秒/50 MB、任务整体 30 分钟继续有效。

### 6.5 不兼容迁移（已确认）

- 删除旧 URL、手工/URL 条目和同步任务，不转换旧 `pool_entries`。
- 重建素材池相关表和约束。
- 新 `rule_pools.id` 从迁移前旧池最大 ID 之后分配，禁止旧 ID 被新池复用。
- 旧蓝图重新编辑时显示旧池引用失效，不自动绑定新池。
- 已生成配置和自包含 `render_plan_json` 保留，历史下载不依赖新素材池。
- 更新数据清理、cron、概览、导入导出和测试夹具。
- 迁移在单事务内完成，不支持降级。

不保留旧池壳是有意设计：ID 复用会让旧蓝图静默绑定语义不同的新池。

---

## 七、Clash/SR 装配

### 7.1 目标映射

| Canonical Rule | Clash/Mihomo | SR |
|----------------|---------------|----|
| `domain/exact` | `DOMAIN` | `DOMAIN` |
| `domain/suffix` | `DOMAIN-SUFFIX` | `DOMAIN-SUFFIX` |
| `domain/keyword` | `DOMAIN-KEYWORD` | `DOMAIN-KEYWORD` |
| `ip/cidr` | `IP-CIDR` / `IP-CIDR6` | 对应 CIDR 类型 |
| `ip/asn` | `IP-ASN` | `IP-ASN` |
| `user_agent/*` | 跳过 | `USER-AGENT` |
| 其他 | 按注册表映射 | 按注册表映射 |

项目不锁定 SR 版本，但能力新增/移除必须有目标渲染语料，不能只修改布尔值。

### 7.2 渲染和回执

- 装配器只读取 active Canonical Rule，不依据来源模式或详细格式决定输出。
- 最终 policy 仍由装配页面指定，源 policy 不覆盖。
- `no_resolve` 仅在规则实际设置且目标支持时输出。
- 默认不做语义降级；等价转换必须有测试和回执。
- 池可供两个目标选择，只要该目标最终至少输出一条。

预览/生成返回输入数、直接输出数、等价转换数、目标不支持跳过数、目标校验失败数和最终输出数。最终输出为 0时禁止生成；大于 0但存在跳过时允许生成并明确警告，不增加第二个确认框。素材快照或其他输出字段变化继续触发 `previewStale`，必须重新预览；`render_plan_json` 固化实际规则，历史版本不漂移。

---

## 八、管理页面与 API

### 8.1 URL 输入

旧 `urls: string[]` 改为：

```json
{"url":"https://example.com/rules.txt","source_mode":"auto"}
```

每行只显示“Clash 规则源”“SR 规则源”“我不确定”。自动模式提示：系统会自动识别，混杂、异常或无法可靠识别的来源将失败。详细格式只作为同步结果展示。

输入区紧邻说明：只有纯域名文本才按 PSL 判断，主域为 suffix、额外子域为 exact；显式标记和结构化格式始终优先；相同行的含义可能受整份文档格式影响。

### 8.2 状态、回执与详情

每个 URL 显示用户模式、检测格式/平台、active/pending/failed/待同步、accepted/模式剔除/rejected/duplicates、前后差异和有限样例。pending 提供“激活/丢弃”。池级任务为 `running/succeeded/partial/failed`，逐 URL 独立展示。

池列表和详情增加 common/clash_only/sr_only 数量、family/matcher/规范化值、动态范围徽标、active origin 数和来源入口，并保持后端分页与懒加载。对当前目标输出为 0的池显示不可用；非零时可选择并在预览显示跳过数。

### 8.3 API 原则

- 后端返回来源、格式、平台、快照状态和动态范围，前端不自行推断。
- 提供中央能力注册表只读元数据端点。
- 不兼容旧 `urls: string[]` 请求。
- URL/模式/顺序使用完整来源列表提交并事务校验。
- pending 激活/丢弃携带 source/snapshot ID，防止操作过期结果。

---

## 九、影响范围、覆盖关系与验收

### 9.1 受影响模块

| 模块 | 影响 |
|------|------|
| `backend/internal/rulespec/` | Canonical Rule、中央能力注册表、目标映射和元数据 |
| `backend/internal/pool/` | 来源、探测/适配、清洗、快照、诊断和活动查询 |
| `backend/internal/assembly/` | 活动规则加载、目标过滤、回执和蓝图失效引用 |
| `backend/internal/server/` | 素材池/装配 API、pending 操作、概览 |
| `backend/migrations/` | 不兼容 schema 和 ID 防复用 |
| cron/dataclear | 新来源调度、清理顺序和测试 |
| 前端 API/assembly views | 三模式选择、来源状态、徽标、pending 和预览回执 |
| 测试夹具 | 迁移、解析语料、原子性、装配、蓝图、概览和前端交互 |

### 9.2 与 Design2 的关系

Build16 完成后，本文覆盖 Design2 中的 `urls_json string[]`、裸域名一律 suffix、`rule_type + match_value` 直接入库、URL 分批写活动条目及前后端静态能力列表。URL 数量 50、单 URL 60 秒/50 MB、任务 30 分钟、manual 在前/URL 在后、异步取消、池级选择、装配版本和分发机制继续有效。

### 9.3 实施边界与验收

- Build16 每次只执行一个 Step，验收通过后等待下一步授权。
- 不顺带修改节点、代理组、Xray 或权限体系，不新增后续适配器。
- 不因素材池限制删除高级装配现有能力。
- 语法变化优先更新语料和注册表，不增加无证据 fallback。

验收至少覆盖三模式矩阵、单适配器、混合/冲突硬失败、DailyData/Mihomo/typed/CIDR/sing-box 语料、PSL/IDNA/通配/IP-ASN、识别率和 pending、staging 不可见与原子切换、多来源去重排序、中央注册表、Clash/SR 回执、零输出门槛、不兼容迁移、旧 ID/蓝图/历史下载，以及后端 build/vet/test、前端 test/build、`git diff --check`。

---

## 十、研究依据

- [Mihomo 路由规则](https://wiki.metacubex.one/en/config/rules/)
- [Mihomo rule-providers 内容](https://wiki.metacubex.one/en/config/rule-providers/content/)
- [Mihomo 域名通配符](https://wiki.metacubex.one/en/handbook/syntax/#domain-wildcards)
- [sing-box Source Format](https://sing-box.sagernet.org/configuration/rule-set/source-format/) 与 [Headless Rule](https://sing-box.sagernet.org/configuration/rule-set/headless-rule/)
- [Shadowrocket Wiki 规则类型](https://github.com/LOWERTOP/Shadowrocket/wiki/#%E8%A7%84%E5%88%99%E7%B1%BB%E5%9E%8B)
- [Public Suffix List](https://publicsuffix.org/)
- 项目内两份 DailyData 模板、Design2 和当前 pool/rulespec/assembly 实现

外部格式可能变化。实现不锁定 SR 版本，但必须把项目实际能力固化为可执行测试，上游语法或依赖升级时运行回归。

---

## 十一、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-31 | 设计定稿：确认三种来源模式、单 URL 单主方言、异常混合直接失败、Canonical Rule/origin 分离、中央能力注册表、per-source 快照、不兼容迁移、sing-box 简单子集、固定保护阈值和装配门槛，作为 Build16 依据。 |
| v0.1 | 2026-08-31 | 初始预想稿：提出多格式识别、PSL 推断和 Clash/SR 双目标渲染方向。 |
