# Design3.md — VPN 订阅管理系统增量设计（规则来源识别、结构化素材与跨平台装配）

> **文档定位：** 本文定义规则素材池下一阶段设计：管理员为每个 URL 选择 Clash 规则源、Shadowrocket（下文简称 SR）规则源或“我不确定”，系统以单 URL 单主方言为边界识别格式、提取平台无关规则、形成可追踪快照，再由 Clash/SR 目标适配器过滤和渲染。本文承接 [Design2.md](Design2.md) 第二～四章；第一期基线见 [Design1.md](Design1.md)。编码约束遵循 [AGENTS.md](../../../AGENTS.md)（**唯一强要求**）。
> **设计状态：** 截至 2026-08-31，本设计已经完成研究和用户决策，并经 [Build16.md](../Build/Build16.md) 构建；后续同日补充 Mihomo ipcidr YAML 与 SR 显式 IP 规则文本识别口径。2026-09-09 经 R28-05 复核确认 Build16 仍有 D3-1～D3-10 未闭环项，实施以 [Build22.md](../Build/Build22.md) 为准；本文已补充重复 origin、手工编辑冲突、来源当前状态、v1 快照统计/激活时间和历史 Clash 渲染计划兼容口径。2026-09-10 交叉审核曾确认 Build22 Step 1～6、8～10 的代码与测试声明成立，但 Step 7 专属自动化证据仍有缺口；2026-09-11 Build22 已补齐 Step 7 全部证据并重新通过 Step 11 门禁，D3-1～D3-10 工程闭环成立，本文件按归档规则移入 `docs/reports/Design/`。Build16 只保留历史记录与后续勘误；实际浏览器、真实设备和真实客户端项目见 [ProdTestList.md](../../../ProdTestList.md)，均未标记为人工通过。剩余工程问题见 [Issue14.md](../../../Issue14.md)。
> **范围边界：** 本期只重构“规则素材 URL/手工素材 → Canonical Rule → Clash/SR 渲染”链路，不重定义节点、代理组、装配版本、订阅分发、Xray 或权限体系。

---

## 一、背景、目标与原则

### 1.1 当前问题

当前 URL 内容仍以逐行解析为主：`full:` 被识别为完整域名，不含逗号的文本一律当作域名后缀，含逗号的规则直接保存为 `rule_type + match_value`。多 URL 同步还会分批写入活动 `pool_entries`，后续来源失败时可能留下部分新数据；后端和前端分别维护目标平台类型列表，能力口径也会漂移。

该方式不能可靠区分 Mihomo domain/ipcidr/classical YAML、sing-box source JSON、显式类型文本和纯 CIDR 列表，也可能把 YAML 键、HTML 错误页或完整子域名误识别。以 [DailyData.txt.template2.md](../../DocTemplates/DailyData.txt.template2.md) 为例：

```yaml
payload:
  - 'a1.mzstatic.com'
  - '+.001wifi.com'
```

在 Mihomo domain provider 中，前者是完整域名，后者是覆盖主域及子域的后缀。素材必须先按整份文档格式解释，不能只看单行外观。

[DailyData.txt.template3.md](../../DocTemplates/DailyData.txt.template3.md) 使用相同 `payload` 容器承载纯 CIDR，属于 Mihomo ipcidr provider；[DailyData.txt.template4.md](../../DocTemplates/DailyData.txt.template4.md) 则是 `IP-ASN`/`IP-CIDR` 显式类型文本。来源模式只调整识别器优先级或适用范围，不能把识别后的 Canonical Rule 绑定到单一输出平台。

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

Rule Origin 独立保存 `source_id`、可空 `snapshot_id`、原始行号/JSON 路径、URL 顺序、来源内顺序和原始条目摘要。多个来源产生相同 `semantic_key` 时只渲染一个规则，但保留全部 active origins；同一来源内相同语义在不同原始位置重复出现时，也必须为每个位置保留 origin，并用 `duplicates` 统计首个之外的重复项。`accepted`/`accepted_count` 统计去重后的 Canonical Rule 数，不把额外 origin 重复计入；删除一个来源或其中一个重复位置后，其他有效 origin 仍存在则规则继续有效。

解析层必须让 Canonical Rule 与 Rule Origin 保持不可错位的组合关系，例如使用 `ParsedRule{Rule, Origin}`，而不是由调用方自行维护两条可能长度不同的平行切片。活动查询在 SQL 分页前按 Canonical Rule 压制重复，并为每个 Canonical 选取排序最早的有效 origin；不能先 `LIMIT/OFFSET` 再在 Go 中去重，否则页面可能少条且总数与列表长度不一致。

### 3.3 中央能力注册表

现有 `RuleDef.SR bool` 改为同时表达规范化校验、目标支持、目标类型、完全等价转换和选项能力的注册表，概念接口为：

```go
func SupportsAndMap(rule CanonicalRule, target Target) MappingResult
```

注册表同时服务来源准入、手工素材类型、范围徽标、装配过滤、目标渲染和前端元数据。`supports_no_resolve` 与某条规则实际设置的 `no_resolve` 必须分离，禁止因“支持”而自动附加。注册表需用能力标记区分**素材池可选能力**与**高级装配 advanced-only 能力**：`RULE-SET`、`AND/OR/NOT`、`MATCH` 等高级能力继续可用于高级装配，但不进入素材池可选集合。

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
- 编辑手工条目时不得原地修改共享 Canonical Rule；应让该手工 origin 换绑到新建或已存在的目标 Canonical，再清理没有任何有效 origin 的旧 Canonical。
- 若目标语义只由 URL origin 提供，允许手工 origin 换绑并与 URL 共享 Canonical；若目标语义已经有另一个手工 origin，返回 HTTP 409，不静默合并、删除或覆盖另一条手工记录。

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

`source_mode` 既影响候选适配器优先级/识别范围，也在 Canonical Rule 形成后执行准入；它不是 `detected_profile` 或输出目标，不能把已识别内容限定为单一平台，也不能绕过结构校验。双方通用的 typed/IP 内容在三种模式下保持通用，剔除后 accepted 为 0 时同步失败。

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
| `mihomo-ipcidr-yaml` | `payload` 只接受 IPv4/IPv6 CIDR 并归一网络地址 |
| `mihomo-classical-yaml` | `payload` 中按显式类型解释 |
| `typed-rule-text` | 按显式类型解释，policy 不进入素材 |
| `plain-ipcidr-text` | IP 转单主机 CIDR，CIDR 归一网络地址 |
| `sing-box-source-json` | 仅 `auto` 识别首期简单子集 |

详细格式只作为同步结果展示，不作为 UI 选择器。

### 4.4 各适配器边界

**Legacy/纯域名：** 只有整份文档已判定为域名文本时，裸域名才使用 PSL。显式 `full:`、`+.`、`DOMAIN` 和 `DOMAIN-SUFFIX` 优先。

**Mihomo provider：** 一个文档只允许一种 behavior；YAML 完整解析并只读取顶层非空字符串数组 `payload`。domain provider 裸域名为 exact，`+.` 为 suffix，`.` 保留仅子域语义；ipcidr provider 只接受合法 IPv4/IPv6 CIDR，不把裸 IP、ASN、域名或显式类型行隐式转入该 behavior。provider 标签通配与 route `DOMAIN-WILDCARD` 不可混同。domain/ipcidr/classical 混合、异常嵌套、空条目和非字符串条目使文档失败。

**显式类型文本：** `DOMAIN`、`DOMAIN-SUFFIX`、`DOMAIN-KEYWORD`、`IP-CIDR/6`、`IP-ASN`、`USER-AGENT` 等先映射 Canonical Rule，再由注册表分类。template4 的 `IP-ASN`/`IP-CIDR` 在 SR 模式下由本适配器识别，但二者仍是双方通用能力，不因此标为 SR 私有。尾部 token 按位置法解释：value 后第一个非 `no-resolve` token 为 source policy，静默忽略；其后再次出现的非 `no-resolve` token 为未知 option，生成 `kind:"warn"` 诊断但不改变 accepted/rejected 统计。只有源实际声明 `no-resolve` 独立 token 时才保存该 option；未知或依赖型类型拒绝。

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
| `deduplicated` | 语义重复 | 不新增 Canonical Rule，但保留每个原始位置的 origin |
| `unsupported_for_target` | 素材合法但当前目标不能表达 | 素材保留，渲染时跳过 |

清洗顺序固定为“提取 → 类型和值校验 → 规范化 → 能力分类 → 来源准入 → 语义去重”。不同 matcher 不能合并。

统计口径固定如下：`excluded` 只计合法但被 `source_mode` 剔除的候选，`rejected` 只计结构、能力或值不合法的候选，`duplicates` 只计首个相同 `semantic_key` 之外的重复 origin；三者不得通过后置差值公式互相推算。清洗/能力分类阶段追加的诊断必须写回最终结果，不能因切片复制或截断时机而丢失。

### 5.4 排序

手工来源在前；URL 按配置顺序；来源内按原始位置和展开顺序。多来源重复使用最早 active origin 排序；某来源消失后按剩余最早 origin 稳定排序。

---

## 六、快照、同步保护与迁移

### 6.1 概念数据模型

| 表/实体 | 职责 |
|---------|------|
| `rule_pools` | 池名称、定时设置和聚合状态，不再保存 `urls_json` |
| `rule_pool_sources` | manual/url 来源、URL、模式、顺序、配置修订和 active/pending 指针 |
| `pool_source_snapshots` | 格式、平台、统计、状态、创建时间和 pending 人工激活时间 |
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
- pending 激活记录管理员操作时间，不修改解析结果；该时间使用快照表 nullable `activated_at` 字段保存，不通过重写 `stats_json` 混入解析统计。

### 6.4 回执、诊断与保留

逐 URL 回执包含来源模式、详细格式、平台、置信依据、input/recognized/accepted/excluded/rejected/duplicates、family/matcher/范围统计、前后数量和格式变化、最终状态与原因。

`stats_json` 使用带 `schema_version` 的强类型对象，不再重复保存已经位于快照表顶层列中的格式、平台、状态和六项计数，也不接受任意 `map[string]any` 作为稳定 API 合同。v1 结构固定包含：

```json
{
  "schema_version": 1,
  "source_mode": "auto",
  "detection": {
    "evidence_codes": ["top_level_payload", "payload_domain_only"],
    "recognition_required_percent": 90
  },
  "rule_counts": [{
    "family": "domain",
    "matcher": "suffix",
    "scope": "common",
    "accepted": 48,
    "excluded": 0,
    "rejected": 0,
    "duplicates": 2
  }],
  "unclassified_rejected": 1,
  "comparison": {
    "previous_active": {
      "snapshot_id": 10,
      "format": "typed-rule-text",
      "profile": "common",
      "accepted": 100
    },
    "format_changed": false,
    "profile_changed": false,
    "accepted_drop_threshold_percent": 70,
    "accepted_drop_triggered": true
  },
  "decision": {
    "initial_status": "pending",
    "reason_codes": ["accepted_below_threshold"]
  }
}
```

- `detection.evidence_codes` 是 detector 在实际命中分支时产生的稳定枚举依据，例如 `sing_box_version_and_rules`、`top_level_payload`、`payload_domain_only`、`payload_ipcidr_only`、`payload_classical_only`、`typed_rule_marker`、`all_items_ip_cidr_or_asn`、`legacy_domain_prefix`、`plain_domain_candidates`；不得只根据最终格式反向猜测。当前探测器是确定性规则而非概率模型，因此不增加未经校准的数值 `confidence_score`。
- `rule_counts` 按 `family`、`matcher`、`scope` 稳定排序，记录已经形成 Canonical 能力归属的 accepted/excluded/rejected/duplicates；不能形成 family/matcher 的结构、类型或值错误计入 `unclassified_rejected`。六项顶层计数是唯一事实来源，并要求 `rule_counts` 汇总与顶层 accepted/excluded/duplicates 一致，`sum(rule_counts.rejected) + unclassified_rejected = rejected`。
- `comparison.previous_active` 在首次同步时为 `null`；比较保存作出 pending 决策时实际使用的旧 active 摘要和 70% 阈值，不保存浮点比例。`decision.initial_status` 是快照创建时的同步决策，`reason_codes` 允许同时记录 `format_changed`、`profile_changed`、`accepted_below_threshold` 等多个原因；快照顶层 `status` 才是 pending 被人工激活后可变化的当前生命周期状态。
- `decision.reason_codes` 的首期稳定枚举为：成功/保护类 `first_success`、`normal`、`format_changed`、`profile_changed`、`accepted_below_threshold`；失败类 `request_invalid`、`network_error`、`http_status_error`、`body_read_error`、`body_too_large`、`html_source`、`unrecognized_source`、`ambiguous_format`、`conflicting_format`、`mixed_platform`、`no_accepted_rules`、`recognition_threshold_not_met`、`parse_error`。同一次 pending 可包含多个保护原因；未知新码由前端回退显示通用说明。
- active、pending、failed 使用同一个 v1 类型。无法进入解析阶段的 failed 快照使用空 `evidence_codes`/`rule_counts`、nullable `recognition_required_percent: null` 和稳定失败原因码，具体脱敏错误摘要只保存于快照顶层 `error`，不在 `stats_json` 再复制一份。历史 `{}` 或旧无版本计数 JSON 统一规范化为 `{schema_version:0, source_mode:"", detection:null, rule_counts:[], unclassified_rejected:0, comparison:null, decision:null}`；缺失字段表示不可用，不反向编造检测依据、比较数据或原因，也不使同一 API 中的新 v1 快照读取失败。
- `detected_profile` 必须依据全部已识别、规范化候选计算，再执行 `source_mode` 排除；显式模式不得先丢弃另一平台私有规则后把真实平台误写为 `common`。adapter 产生的每条 `reject` 诊断也必须进入 `rejected`，保证 `input/recognized/rejected` 与上述分项统计可核对。

限制和脱敏在持久化边界统一执行，不依赖各 adapter 自行遵守，并由日志与 pool 共用同一套脱敏实现：

- 最多保留 20 条代表性诊断；若原始超过 20 条，保留前 19 条真实诊断，第 20 条固定为截断摘要（例如 `kind:"truncated"`、`message:"另有 N 条诊断未展示"`），总条数不得超过 20。
- 每条字符串字段按 rune 截断到 200 字符；空诊断必须序列化为 `[]`，不得输出 `null`。
- 脱敏与限额适用于所有进入持久化或展示 API 的字符串字段：`ParseDiagnostic.Message`、`ParseDiagnostic.Raw`、`PerURLResult.URL`、`PerURLResult.Error`、`SyncTask.Error`、`Pool.SyncError`、`SourceStatus.display_url`。
- 疑似凭据按参数名/字段路径判断，不按值特征猜测。至少覆盖：`token`、`code`、`state`、`password`、`passwd`、`secret`、`client_secret`、`private-key`/`private_key`、`pre-shared-key`/`pre_shared_key`、`psk`、`auth`、`auth-key`/`auth_key`、`access_token`、`refresh_token`、`api_key`、`apikey`；参数名比较前先做 URL 解码并忽略大小写。
- `RedactDisplayURL` 只用于展示/任务输出，不得写回 `rule_pool_sources.url`，也不得用于编辑表单回填；管理员编辑仍使用原始 `url` 配置。
- 现有 `/sync/status`、`/sync/tasks` 与 `Pool.sync_error` 也执行同一读时清洗；存量 `pool_sync_tasks.per_url_json/error`、`rule_pools.sync_error` 做非破坏性清洗，不删除任务/池记录，不修改 `rule_pool_sources.url`。
- 不保存完整响应；URL 查询凭据、Token、`code`/`state` 及疑似凭据必须在进入 `diagnostic_json`、`stats_json`、任务 JSON 或 API 前脱敏。

完成任务和未被指针引用的 failed 诊断快照保留 7 天；active/pending 及其诊断不受任务清理影响。现有 URL 数量 50、单 URL 60 秒/50 MB、任务整体 30 分钟继续有效。

每个 URL 的“当前状态”由最近一次同步尝试决定，而不是只看是否存在历史 failed 行：

- 最近尝试失败且旧 active 仍存在：显示“同步失败，继续使用旧活动快照”，active 指针和装配内容不变；
- 最近尝试产生 pending：显示 pending，并同时展示仍生效的旧 active；
- failed 之后再次成功：当前状态恢复为 active，旧 failed 只保留在有限历史中，不再让来源永久标红；
- 从未同步且无 active/pending/failed：显示“待同步/从未同步”。

状态 API 应同时返回 `latest_attempt`、`active`、`pending` 和有限的 `latest_failed` 摘要；`SourceStatus` 使用 `display_url` 作为脱敏后的展示 URL，状态/历史 API 不得回传 `rule_pool_sources.url` 原始值。前端以 `latest_attempt.status` 决定主徽标，不能仅凭 `latest_failed != null` 推断当前失败。数据库写入本身失败时无法可靠持久化 failed snapshot，此类基础设施错误仍由同步任务错误和日志报告；网络、HTTP、读取上限、内容检查及解析失败则必须尽力写入 failed snapshot。

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
- 池可供两个目标选择，只要该目标的素材池规则与自定义规则合计至少输出一条（不含内置 `GEOIP`/`MATCH`/`FINAL` 等系统兜底）。

预览/生成返回输入数、直接输出数、等价转换数、目标不支持跳过数、目标校验失败数和最终输出数。最终输出只统计**素材池规则 + 自定义规则**，不包含内置 `GEOIP`/`MATCH`/`FINAL` 等系统兜底；最终输出为 0时禁止生成；大于 0但存在跳过时允许生成并明确警告，不增加第二个确认框。素材快照或其他输出字段变化继续触发 `previewStale`，必须重新预览；`render_plan_json` 固化实际规则，历史版本不漂移。

`no_resolve` 必须按结构化尾部 token 解析，仅独立且目标能力允许的 `no-resolve` token 才设置 Canonical option；不得通过对整行或匹配值做子串搜索推断，否则 `no-resolve.example.com` 等合法值会被误判。source policy 静默忽略；未知尾部 option 生成 `kind:"warn"` 诊断，但不得静默变成 `no_resolve`。

Clash `render_plan_json` 必须冻结每条新规则的显式 `no_resolve=true/false`，下载重渲染与生成时保持一致。兼容编码固定采用逐规则 nullable boolean（Go 字段为 `NoResolve *bool`，JSON tag 为 `json:"no_resolve,omitempty"`），不为这一单字段引入整份 Clash plan 的顶层 schema version：字段缺失或 JSON `null` 表示既有历史计划，沿用创建该计划时的按类型推断行为；字段存在时严格按实例 true/false 与目标能力的交集渲染。新计划的每条规则都必须写出 boolean，包括 false，不得借 `omitempty` 省略；生产端应使用单一构造入口避免漏设后误入历史分支。覆盖层或目标组删除导致规则重写时，只保留已渲染行/显式计划中的 `no-resolve`，不得再次根据类型补加。未来只有在整份 plan 出现新的结构演进需求时，才另行设计顶层版本号。

Build22 实现收口（2026-09-10）：`preview` 响应与 `generate` 成功响应均返回本次 `receipt`，wire 字段为 `input`、`direct_output`、`equivalent_conversions`、`skipped_unsupported`、`target_validation_failed`、`final_output`。前端仅在当前预览指纹有效时显示 preview 回执；目标、输入、选中素材池、自定义规则等导致 `previewStale` 的变化会清除旧回执。`generate` 成功页只使用本次 generate 响应中的回执，不复用 preview 回执；缺省 receipt 时界面不显示虚假零值。

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

每个 URL 显示用户模式、检测格式/平台、由最近一次尝试决定的 active/pending/failed/待同步主状态、当前仍生效的 active、accepted/模式剔除/rejected/duplicates、前后差异和有限样例。最近失败但仍有旧 active 时必须同时表达“失败”和“继续使用旧活动快照”；之后成功时旧 failed 仅进入历史。pending 提供“激活/丢弃”。池级任务为 `running/succeeded/partial/failed`，逐 URL 独立展示。

Build22 实现收口（2026-09-10）：详情页卡片只展示后端返回的 `display_url`，编辑路径继续使用 `Pool.sources[].url`/`Pool.urls[]` 原始 URL，`display_url` 不进入 create/update 请求。主徽标严格读取 `latest_attempt.status`，`latest_failed` 只作为历史信息；failed 与旧 active 并存时显示“同步失败，继续使用旧活动快照”，后续成功恢复 active 主徽标。激活前使用项目 `ConfirmModal` 展示旧 active 与新 pending 的 input/accepted/format/profile 和诊断差异，`activated_at` 只展示服务端值；激活/丢弃成功后刷新来源状态与素材条目。v1 stats 展示 evidence codes、rule_counts、comparison 与 decision；`schema_version=0` 显示“历史统计不可用”，不把空字段解释为零变化或成功依据。验收时实际浏览器双视口与真实交互仍需用户执行，登记于 ProdTestList。

池列表和详情增加 common/clash_only/sr_only 数量、family/matcher/规范化值、动态范围徽标、active origin 数和来源入口，并保持后端分页与懒加载。对当前目标输出为 0的池显示不可用；非零时可选择并在预览显示跳过数。

### 8.3 API 原则

- 后端返回来源、格式、平台、快照状态和动态范围，前端不自行推断。
- 提供中央能力注册表只读元数据端点。
- 不兼容旧 `urls: string[]` 请求。
- URL/模式/顺序使用完整来源列表提交并事务校验。
- pending 激活/丢弃携带 source/snapshot ID，防止操作过期结果。

### 8.4 Clash YAML 头部参数分区编辑（2026-09-02）

本节是 Clash YAML 装配器头部表单的 UI 增量，限定于 `fixed_params` 的编辑和默认值；不改变节点、代理组、规则、覆盖层、生成 API、蓝图快照或 YAML 渲染语义。后端继续将 `fixed_params` 作为有序顶层对象写入最终 Clash YAML。

1. 结构化表单分为“端口配置”“Geo 数据”“DNS 配置”“更多参数”四个可折叠分区，首次进入和使用默认值后四个分区均保持折叠，避免 DNS 等长配置压缩后续装配步骤。
2. “端口配置”预填 `port`、`socks-port`、`redir-port`、`tproxy-port` 的个人模板值 `7890/7891/7892/7893`；`mixed-port` 可按需填写，但不写入默认头部，避免默认同时开启混合与独立监听。
3. “Geo 数据”预填个人模板的 `geox-url.geoip/geosite/mmdb` 地址、`geo-auto-update=true` 和 `geo-update-interval=168`；其 bool 控件位于本分区“开关参数”区。
4. “DNS 配置”结构化编辑 `dns.enable/ipv6/listen/enhanced-mode/fake-ip-range`、默认 DNS、fallback、fake-ip-filter 和 fallback-filter；列表项用可增删的行表格表示，`dns.enable`、`dns.ipv6` 与 `fallback-filter.geoip` 集中在本分区开关区。
5. “更多参数”预填个人模板其余头部值：`allow-lan`、`find-process-mode`、`mode`、`log-level`、全局 `ipv6` 和 `ntp`；本分区同样默认折叠，且集中 `allow-lan`、`ipv6`、NTP 启用/写入系统时间等开关。未被前三个分区识别的既有顶层参数也保留在该分区，供高级用户处理。
6. 四个分区各自提供“结构化编辑 / 高级 JSON”切换。高级 JSON 的作用域仅为当前分区：端口和 Geo 为相应顶层键、DNS 为 `dns` 对象、更多参数为其余顶层键。切换或保存时必须验证 JSON 对象形状；无效 JSON 不得静默丢弃。结构化操作保留其他分区和“更多参数”内未知键，从而兼容旧蓝图和未来 Mihomo 扩展。

默认值来自 [Clash.yaml.template.md](../../DocTemplates/Clash.yaml.template.md) 的头部（不包含由装配后续步骤拥有的 `proxies`、`proxy-groups`、`rules`）。[ClashOfficial.yaml.template.md](../../DocTemplates/ClashOfficial.yaml.template.md) 仅用于字段含义与可选能力参考，不能以其样例值覆盖个人默认配置。

---

## 九、影响范围、覆盖关系与验收

### 9.1 受影响模块

| 模块 | 影响 |
|------|------|
| `backend/internal/rulespec/` | Canonical Rule、中央能力注册表、目标映射和元数据 |
| `backend/internal/pool/` | 来源、探测/适配、清洗、快照、诊断和活动查询 |
| `backend/internal/redact/` | 日志与素材池共用的文本/URL 脱敏、Unicode 限长与敏感键规则 |
| `backend/internal/log/` | 日志输出脱敏接入公共 redact 规则 |
| `backend/internal/assembly/` | 活动规则加载、目标过滤、回执和蓝图失效引用 |
| `backend/internal/server/` | 素材池/装配 API、pending 操作、概览 |
| `backend/migrations/` | 不兼容 schema 和 ID 防复用 |
| cron/dataclear | 新来源调度、清理顺序和测试 |
| 前端 API/assembly views | 三模式选择、来源状态、徽标、pending 和预览回执 |
| 测试夹具 | 迁移、解析语料、原子性、装配、蓝图、概览和前端交互 |

### 9.2 与 Design2 的关系

Build16 完成后，本文覆盖 Design2 中的 `urls_json string[]`、裸域名一律 suffix、`rule_type + match_value` 直接入库、URL 分批写活动条目及前后端静态能力列表。URL 数量 50、单 URL 60 秒/50 MB、任务 30 分钟、manual 在前/URL 在后、异步取消、池级选择、装配版本和分发机制继续有效。

### 9.3 实施边界与验收

- Build16 的原始构建已归档；其 D3-1～D3-10 后续缺口以 Build22 为唯一分步计划。Build22 已完成 Step 1～11 的代码实现与运行门禁，Step 7 专属自动化证据也已于 2026-09-11 补齐并重新通过 Step 11 门禁，D3-1～D3-10 工程闭环成立，Build22 与本设计均已归档。实际浏览器、真实设备和真实客户端项目整体迁移至 [ProdTestList.md](../../../ProdTestList.md)，不构成代码/自动化验收的未完成阻断，也不得表述为已人工通过。
- 不顺带修改节点、代理组、Xray 或权限体系，不新增后续适配器。
- 不因素材池限制删除高级装配现有能力。
- 语法变化优先更新语料和注册表，不增加无证据 fallback。

验收至少覆盖三模式矩阵、单适配器、混合/冲突硬失败、四份 DailyData/Mihomo domain/ipcidr/classical/typed/CIDR/sing-box 语料、PSL/IDNA/通配/IP-ASN、IPv4/IPv6 目标类型、识别率和 pending、staging 不可见与原子切换、多来源去重排序、中央注册表、Clash/SR 回执、零输出门槛、不兼容迁移、旧 ID/蓝图/历史下载，以及后端 build/vet/test、前端 test/build、`git diff --check`。

---

## 十、研究依据

- [Mihomo 路由规则](https://wiki.metacubex.one/en/config/rules/)
- [Mihomo rule-providers 内容](https://wiki.metacubex.one/en/config/rule-providers/content/)
- [Mihomo 域名通配符](https://wiki.metacubex.one/en/handbook/syntax/#domain-wildcards)
- [sing-box Source Format](https://sing-box.sagernet.org/configuration/rule-set/source-format/) 与 [Headless Rule](https://sing-box.sagernet.org/configuration/rule-set/headless-rule/)
- [Shadowrocket Wiki 规则类型](https://github.com/LOWERTOP/Shadowrocket/wiki/#%E8%A7%84%E5%88%99%E7%B1%BB%E5%9E%8B)
- [Public Suffix List](https://publicsuffix.org/)
- 项目内四份 DailyData 模板、Design2 和当前 pool/rulespec/assembly 实现

外部格式可能变化。实现不锁定 SR 版本，但必须把项目实际能力固化为可执行测试，上游语法或依赖升级时运行回归。

---

## 十一、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.12 | 2026-09-11 | Build22 Step 7 证据补齐并重新通过 Step 11 全量门禁：激活/activated_at、存量清洗、现有 sync API 读时脱敏、19+1/200 rune、v1 stats/version 0、latest_failed 恢复/排序、failed 写失败保护、旧 Clash plan 回退及额外边界均有自动化证据；D3-1～D3-10 闭环，本设计随 Build22 归档。 |
| v1.11 | 2026-09-10 | 文档交叉审核修正：Build22 Step 7 专属自动化测试矩阵大面积未落地，Step 11 不能声明 D3-1～D3-10 全部验收通过；Design3 因此保持根目录活跃、不归档，待 Build22 补齐 Step 7 证据并重新执行 Step 11 门禁后再归档。 |
| v1.10 | 2026-09-10 | Build22 Step 8～11 按实际实现收口：补充来源状态卡片的 display_url 边界、latest_attempt 主状态、failed+旧 active 提示、ConfirmModal 新旧差异、服务端 activated_at、v1 stats/version 0 展示，以及 preview/generate 回执的生成与清除规则；§9.3 更新为 Build22 已完成代码与运行门禁，人工浏览器/真机项目迁移 ProdTestList。未写入未实现能力；Step 7 自动化证据缺口由 v1.11 修正。 |
| v1.9 | 2026-09-10 | 按用户确认的补修方案同步 Step3/Step7 设计口径并落地代码：source policy 按位置法静默忽略、未知尾部 option 生成 warn 而不改变统计；SR `no-resolve` 与 Clash 一致按实例和目标能力求交集；evidence codes 在实际 detector 分支产生；failed 使用 sentinel 稳定 reason_code 与严格 v1 stats 形状；snapshots 严格分页、URL 先脱敏后 200 rune 限长、wire null/空串固定。D3-1～D3-7 已完成并通过自动化回归，D3-8～D3-10 仍待实施。 |
| v1.8 | 2026-09-10 | 按 Build22 Step 7 脱敏研究结论与用户确认同步 §6.4：日志与 pool 共用统一脱敏规则；`SourceStatus` 使用 `display_url`；脱敏/限长扩展至所有持久化与展示 API 字符串字段；明确疑似凭据 key 清单、19+1 截断摘要、200 rune、空诊断 `[]`、历史同步输出非破坏性清洗及现有 sync API 读时清洗。仅更新设计文档，代码仍待 Build22 实施。 |
| v1.7 | 2026-09-09 | Clash render plan 兼容编码经专项研究确认：采用逐规则 nullable boolean/Go `*bool` 三态，缺失或 null 维持历史按类型推断，新计划对每条规则显式冻结 true/false；不为单字段引入整份 plan schema version，并保留未来整体结构演进时再版本化的空间。仅更新设计文档，代码仍待 Build22 Step 3 实施。 |
| v1.6 | 2026-09-09 | R28-05 `stats_json` 专项研究并经用户确认：冻结 version 1 强类型统计、确定性检测依据码、family/matcher/scope 分项、旧 active 比较与初始决策原因；顶层列保持计数/格式/profile/当前状态的唯一事实来源，旧 `{}`/无版本 JSON 兼容但不补造证据；明确 profile 在来源排除前计算、adapter reject 完整计数，并使用 1018 nullable `activated_at` 记录 pending 人工激活时间。仅更新设计文档，代码仍待 Build22 实施。 |
| v1.5 | 2026-09-09 | 构建前文档核验同步：§9.3 的当前串行执行入口由已归档 Build16 更新为 Build22；不改变 D3-1～D3-10 的既有设计结论，也不表示代码已经开始实施。 |
| v1.4 | 2026-09-09 | R28-05 研究补充并经用户确认：相同语义保留全部 origin、accepted 统计唯一 Canonical；手工编辑通过 origin 换绑且 manual→manual 重复返回 409；来源主状态以最近尝试为准并可同时保留旧 active；`no_resolve` 使用结构化 token，新的 Clash render plan 显式冻结实例值，旧计划缺失字段时保持历史兼容；明确诊断统一限额/脱敏和迁移回归边界。本文只修订设计，D3-1～D3-10 代码仍待 Build22 实施。 |
| v1.3 | 2026-09-02 | 新增 Clash YAML 头部参数 UI 增量：以个人模板预填端口、Geo、DNS、更多参数四个默认折叠分区；每区提供结构化编辑与作用域明确的高级 JSON，组内 bool 集中，保留未知顶层键；`mixed-port` 为可选项而非默认监听。 |
| v1.2 | 2026-08-31 | 补充来源识别：新增严格的 `mihomo-ipcidr-yaml`，Mihomo `payload` 按整份内容唯一归类且 ipcidr 仅接受 IPv4/IPv6 CIDR；template4 继续作为双方通用的 `typed-rule-text`；明确来源模式只调整识别优先级/范围与既有准入，不绑定识别内容或最终输出平台。 |
| v1.1 | 2026-08-31 | 补充实现口径：中央注册表区分素材池能力与 advanced-only 高级装配能力；`final_output` 只统计素材池+自定义规则，排除内置 `GEOIP`/`MATCH`/`FINAL` 兜底；能力元数据端点明确由前端消费并移除静态规则表。 |
| v1.0 | 2026-08-31 | 设计定稿：确认三种来源模式、单 URL 单主方言、异常混合直接失败、Canonical Rule/origin 分离、中央能力注册表、per-source 快照、不兼容迁移、sing-box 简单子集、固定保护阈值和装配门槛，作为 Build16 依据。 |
| v0.1 | 2026-08-31 | 初始预想稿：提出多格式识别、PSL 推断和 Clash/SR 双目标渲染方向。 |
