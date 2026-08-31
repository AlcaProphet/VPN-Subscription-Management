# Design3.md — VPN 订阅管理系统增量设计预想（多源规则识别、清洗与跨平台装配）

> **文档定位：** 本文档记录规则素材池下一阶段的设计预想：系统读取不同来源、不同语法的 URL 数据源，先识别格式并转换为平台无关的结构化规则，再由 Clash 与 Shadowrocket（下文简称 SR）各自的输出适配器生成目标规则。本文档承接 [Design2.md](Design2.md) 第二～四章已经建成的规则素材池与订阅装配能力；第一期产品基线见 [Design1.md](docs/reports/Design/Design1.md)。编码约束遵循 [AGENTS.md](AGENTS.md)（**唯一强要求**）。
> **设计状态：** 截至 2026-08-31，本文件是**待评审、待拆分 Build 的设计预想稿**，尚未构建，不代表当前系统已经支持本文列出的格式或行为。当前实现仍以 Design2.md 及现有代码为准。
> **范围边界：** 本期只改进「规则素材 URL → 结构化规则 → Clash/SR 渲染」链路，不重定义节点模型、代理组模型、装配版本管理、订阅分发、Xray 对接或用户权限体系。

---

## 一、背景、目标与设计原则

### 1.1 当前问题

Design2 已实现规则素材池，但 URL 内容仍以逐行解析为主：

- `full:<域名>` 被识别为完整域名；
- 不含逗号的普通文本行一律被识别为域名后缀；
- 含逗号的规则行按首段规则类型和次段匹配值解析；
- 数据库和拼装器直接使用 `rule_type + match_value`。

该方式能够处理 [DailyData.txt.template.md](docs/DocTemplates/DailyData.txt.template.md) 和一部分规则文本，但不能可靠区分 Mihomo domain provider YAML、Mihomo classical YAML、sing-box source JSON、纯 CIDR 列表等来源。尤其是把所有无逗号文本都当作 `DOMAIN-SUFFIX`，可能将 YAML 键、HTML 错误页、完整子域名或其他非规则内容大面积误识别。

新增样例 [DailyData.txt.template2.md](docs/DocTemplates/DailyData.txt.template2.md) 已体现第二种语义：

```yaml
payload:
  - 'a1.mzstatic.com'
  - '+.001wifi.com'
```

在该 Mihomo domain provider 语境中：

- `a1.mzstatic.com` 是完整域名匹配；
- `+.001wifi.com` 是域名后缀匹配，覆盖主域及其子域。

因此，素材内容必须先按**文档格式**解释，不能只按单行外观猜测。

### 1.2 设计目标

1. 支持从多种公开 URL 数据源自动识别并提取规则，降低管理员必须预先理解源格式的门槛。
2. 将“源语法”“规则语义”“目标平台语法”解耦，拼装器不再直接使用原始数据。
3. 统一提取“规则对象 + 匹配方式 + 匹配值 + 附加参数 + 来源信息”，例如：
   - 域名 + 精确匹配 + `a1.mzstatic.com`；
   - 域名 + 后缀匹配 + `001wifi.com`；
   - 域名 + 关键字匹配 + `openaiapi`；
   - IP + CIDR 匹配 + `1.2.3.0/24`；
   - IP + ASN 匹配 + `13335`。
4. 在同步前完成规范化、合法性校验、过滤和语义去重；不能识别或目标平台不支持的内容必须可观测，不能静默错配。
5. 通过格式置信度、异常内容识别、识别率和历史规模变化保护旧数据，避免错误响应导致大面积覆盖或删除。
6. Clash 与 SR 分别按自身能力输出；源格式不决定最终输出格式，同一份结构化素材可供两个平台复用。

### 1.3 核心原则

- **显式语义优先于推断**：`full:`、`+.`、`DOMAIN,`、`DOMAIN-SUFFIX,` 等显式标记一旦有效，必须按其语义处理。
- **文档级识别优先于行级猜测**：先识别 JSON/YAML/typed text/plain list 等文档格式，再由对应适配器解释行或字段。
- **源适配与目标渲染分离**：源适配器只生成统一结构，目标渲染器只消费统一结构。
- **不做有损偷换**：不能把“不含主域”的子域通配、正则或复杂逻辑，为了输出方便直接扩大成域名后缀。
- **错误可见、旧数据优先**：低置信度、疑似 HTML、零有效条目、异常缩量等情况不更新该 URL 的旧条目。
- **渐进扩展**：先支持已有样例和常见规则集；新增格式通过独立适配器接入，不在一个巨型逐行解析器中不断叠加特例。

### 1.4 非目标

- 不承诺把任意网络文本自动转换成规则。
- 不执行远端脚本，不解析二进制规则集（如 `.mrs`、`.srs`）为首期能力。
- 不把代理节点订阅、完整 Clash 配置或 SR 节点订阅误当成规则素材。
- 不保证每种源规则都能无损输出到每个目标平台；无法表示的规则由目标能力检查明确拒绝或跳过并生成回执。

---

## 二、总体架构

### 2.1 数据链路

规则素材同步统一经过以下阶段：

```text
URL 响应
  → 响应安全检查
  → 文档格式探测
  → 源格式适配器
  → 平台无关规则（Canonical Rule）
  → 规范化 / 校验 / 过滤 / 语义去重
  → 结构化数据落库
  → 装配器读取结构化数据
  → Clash 或 SR 目标能力检查
  → 目标语法渲染 + 转换回执
```

**强制边界：** 装配器不得重新读取或重新解释原始 URL 文本；原始内容只用于本次探测、解析和问题追踪。

### 2.2 分层职责

| 层级 | 输入 | 输出 | 职责 |
|------|------|------|------|
| 拉取层 | URL | 原始字节、HTTP 元数据 | 超时、大小上限、状态码、内容类型、字符编码与基础响应检查 |
| 探测层 | 原始字节 | 候选格式、置信度、探测依据 | 判断 JSON/YAML/typed text/plain domain/plain CIDR 等，不生成平台规则 |
| 源适配层 | 已选格式 + 原始内容 | Canonical Rule 列表、逐项诊断 | 解释源格式的显式语义，保留来源行/路径 |
| 清洗层 | Canonical Rule | 规范化且可入库的规则 | IDNA、PSL、CIDR 规范化、白名单校验、过滤、去重 |
| 存储层 | 规范化规则 | 素材池结构化快照 | 保存稳定语义与来源元数据，不保存目标平台拼装结果 |
| 目标适配层 | 结构化规则 + 目标 | Clash/SR 规则及回执 | 能力检查、目标语法映射、附加参数生成、跳过说明 |

### 2.3 统一规则模型

统一模型以“匹配语义”为中心，不以某个平台的字符串规则名为中心。概念字段如下：

| 字段 | 含义 | 示例 |
|------|------|------|
| `family` | 匹配对象类别 | `domain` / `ip` / `process` / `user_agent` / `network` / `port` / `geo` / `logic` |
| `matcher` | 匹配方式 | `exact` / `suffix` / `keyword` / `wildcard` / `regex` / `cidr` / `asn` / `equals` |
| `value` | 规范化后的匹配值 | `a1.mzstatic.com`、`001wifi.com`、`1.2.3.0/24` |
| `options` | 不属于值本身的语义参数 | `no_resolve`、来源/目标方向等可枚举参数 |
| `source_format` | 实际识别的源格式 | `legacy-domain-text`、`mihomo-domain-yaml`、`typed-rule-text` |
| `source_url` | 条目来源 URL | 管理员配置的 URL |
| `source_position` | 原始位置 | 文本行号或 JSON/YAML 路径 |
| `raw_fingerprint` | 原始条目摘要 | 用于诊断和变更追踪，不作为渲染输入 |

数据库的最终列拆分与迁移方式由后续 Build 设计确定，但必须满足以下语义：

- `domain/exact/a.example.com` 与 `domain/suffix/a.example.com` 是两条不同规则；
- `full:a.example.com`、`DOMAIN,a.example.com` 与 Mihomo domain YAML 中的 `a.example.com` 应归一为同一条精确规则；
- `+.example.com`、`DOMAIN-SUFFIX,example.com` 与纯文本主域推断结果应归一为同一条后缀规则；
- 语义去重键至少包含 `family + matcher + normalized value + semantic options`；
- 原始拼写、引号、大小写和平台规则名不参与语义去重。

### 2.4 与现有 `rule_type + match_value` 的关系

现有字段可作为迁移期 API 投影或目标规则映射，但不再是源解析与存储设计的唯一事实来源。目标适配器负责将统一模型映射为：

- Clash/Mihomo：`DOMAIN`、`DOMAIN-SUFFIX`、`DOMAIN-KEYWORD`、`DOMAIN-WILDCARD`、`DOMAIN-REGEX`、`IP-CIDR`、`IP-ASN` 等；
- SR：按项目验证过的 SR 支持子集生成相应规则；不支持项进入跳过/拒绝回执。

---

## 三、格式探测与源适配器

### 3.1 探测顺序

探测器不得只依据文件扩展名或 HTTP `Content-Type`。建议按以下优先级组合判断：

1. 响应体是否为空、是否疑似 HTML/登录页/网关错误页；
2. 是否为可解析 JSON，并命中已知结构；
3. 是否为可解析 YAML，并命中 `payload` 等已知结构；
4. 是否为显式规则文本（大量行满足 `TYPE,VALUE[,POLICY...]`）；
5. 是否为纯 CIDR/IP 列表；
6. 是否为 legacy `full:` + 裸域名文本；
7. 是否为无显式标记的纯域名文本。

结构化格式必须先完整解析结构，再提取叶子条目。不得把 YAML 的 `payload:`、JSON 的键名或 HTML 标签送入裸域名解析器。

每个候选适配器返回：

- 格式标识；
- 置信度及命中依据；
- 输入候选数、接受数、拒绝数；
- 结构错误和逐项错误摘要。

当多个适配器均可能匹配但无法形成明确优先级时，自动同步不落库，要求管理员检查或显式指定格式。具体置信度阈值在 Build 阶段以真实样本集确定，不在本预想稿中拍定任意数值。

### 3.2 首期适配器范围

| 格式标识 | 典型外观 | 首期语义 |
|----------|----------|----------|
| `legacy-domain-text` | `full:a.example.com`、裸域名 | 显式 `full:` 为精确；裸域名按 §4.2 的 PSL 规则推断 |
| `plain-domain-text` | 每行一个域名，无 `full:` | 按 §4.2 的 PSL 规则推断精确/后缀 |
| `mihomo-domain-yaml` | `payload:` 下为域名、`+.`、`.` 或通配表达式 | 按 Mihomo domain provider 语义解释，不使用纯文本 PSL 推断 |
| `mihomo-classical-yaml` | `payload:` 下为 `DOMAIN,...`、`IP-CIDR,...` | 按显式规则类型解释 |
| `typed-rule-text` | `DOMAIN-SUFFIX,openai.com` | 按显式规则类型解释；行内 policy 不进入素材语义，由装配器指定目标 |
| `plain-ipcidr-text` | 每行 IP 或 CIDR | IP 归一为单主机 CIDR，CIDR 归一为网络地址 |
| `sing-box-source-json` | `version` + `rules`，字段含 `domain`、`domain_suffix`、`ip_cidr` 等 | 展开已支持的 headless rule 字段为结构化规则；复杂组合保留组合关系或明确拒绝，不静默扁平化 |

### 3.3 Legacy 与纯文本域名适配器

显式前缀优先：

| 原始条目 | 统一语义 |
|----------|----------|
| `full:a1.mzstatic.com` | `domain + exact + a1.mzstatic.com` |
| `+.001wifi.com` | `domain + suffix + 001wifi.com` |
| `DOMAIN,a1.mzstatic.com` | `domain + exact + a1.mzstatic.com` |
| `DOMAIN-SUFFIX,001wifi.com` | `domain + suffix + 001wifi.com` |

只有在整份文档已经被判定为 `legacy-domain-text` 或 `plain-domain-text` 后，无显式标记的域名才执行 PSL 推断。不得把“任意无逗号行”直接送入该规则。

### 3.4 Mihomo domain provider YAML

以 [DailyData.txt.template2.md](docs/DocTemplates/DailyData.txt.template2.md) 为样例：

| `payload` 条目 | 统一语义 | 说明 |
|----------------|----------|------|
| `a1.mzstatic.com` | `domain + exact` | Mihomo domain provider 中裸条目是完整域名；即使它恰好是可注册主域，也不改判为 suffix |
| `+.001wifi.com` | `domain + suffix` | 匹配主域及其子域 |
| `.example.com` | `domain + subdomain-wildcard` | 只匹配其子域的语义应独立保留，不能扩大为包含主域的 suffix |
| `*.*.microsoft.com` | `domain + wildcard` | 保留标签通配结构，不提前改写为 suffix |

YAML 适配器必须：

- 只读取预期的 `payload` 序列；
- 拒绝对象、嵌套异常和非字符串条目；
- 正确处理引号、注释与 YAML 转义；
- 在同一 `payload` 中根据条目形态区分 domain、classical 或 ipcidr；明显混杂且无法确定整体语义时不猜测。

### 3.5 显式规则文本

[AINA.txt](https://raw.githubusercontent.com/iab0x00/ProxyRules/main/Rule/AINA.txt) 的典型内容为：

```text
DOMAIN-SUFFIX,openai.com
DOMAIN,api.statsig.com
DOMAIN-KEYWORD,openaiapi
```

这种文本不是 SR 独占格式，更准确地说是 Clash/Mihomo 与 SR 生态均常见的**显式类型规则文本**。适配器应按规则类型读取匹配语义，而不是根据域名层级再次推断：

- `DOMAIN` → `domain/exact`；
- `DOMAIN-SUFFIX` → `domain/suffix`；
- `DOMAIN-KEYWORD` → `domain/keyword`；
- `IP-CIDR` / `IP-CIDR6` → `ip/cidr`；
- `IP-ASN` → `ip/asn`；
- 其他已知类型按统一模型映射；未知类型拒绝并记录。

若源行带有 policy（如 `DOMAIN,example.com,PROXY`）或 `no-resolve`：

- policy 属于源文件的目标策略，不进入素材池目标；最终目标仍由本系统装配步骤指定；
- `no-resolve` 等影响匹配行为的附加参数应结构化保留，并由目标适配器决定是否输出；
- 被忽略的 policy 必须计入诊断提示，不能无提示丢弃。

### 3.6 sing-box source JSON

首期只读取 source format JSON，不读取已编译 `.srs`。适配原则：

- 校验顶层 `version` 与 `rules`；
- 支持明确可映射的 `domain`、`domain_suffix`、`domain_keyword`、`domain_regex`、`ip_cidr`、`ip_is_private`、进程与网络等字段；
- 一个规则对象含多个同类值时展开为多个 Canonical Rule；
- 一个对象同时包含多个条件、反向条件或逻辑关系时，不能当作若干互不相关的普通规则；必须保留逻辑组合，或在首期不支持时整项拒绝并说明原因；
- sing-box action/route target 不进入素材池目标，由本系统装配器决定最终策略。

### 3.7 后续可扩展适配器

以下格式只列为扩展方向，不属于首期承诺：

- hosts：`0.0.0.0 example.com` / `127.0.0.1 example.com`；
- AdGuard/Adblock：`||example.com^`、例外规则与修饰符；
- dnsmasq：`server=/example.com/...`；
- Surge/Loon 等显式类型规则变体；
- 其他 JSON/YAML 规则集；
- 二进制 `.mrs` / `.srs`（仅在有稳定官方解码能力时考虑）。

每种格式必须作为独立适配器接入，并提供正例、反例与误识别回归样本。

---

## 四、规范化、纯文本推断与过滤

### 4.1 域名规范化

域名进入统一模型前按以下顺序处理：

1. 去除条目外层空白和允许的尾随根点；
2. 使用 IDNA lookup 语义转换为 ASCII/Punycode；
3. 统一小写；
4. 校验标签长度、总长度、空标签和非法字符；
5. 根据显式语义或 §4.2 的 PSL 规则确定 matcher；
6. 保存规范化值用于去重与渲染。

原始 Unicode/大小写只作为诊断显示，不参与语义判断。通配符、关键字与正则必须使用各自校验器，不能先当普通域名走 IDNA。

### 4.2 无显式标记纯域名的 PSL 推断（已确认）

“主域名”采用 [Public Suffix List](https://publicsuffix.org/)（包含 PRIVATE 区段）的 eTLD+1 定义，禁止按点号数量或标签数量猜测：

1. 计算输入域名的可注册主域（eTLD+1）；
2. 输入恰好等于 eTLD+1 → `domain/suffix`；
3. 输入在 eTLD+1 前仍有一个或多个标签 → `domain/exact`；
4. PSL 无法可靠得到 eTLD+1 的单标签、异常域名或不完整地址 → 不猜测，拒绝并记录为歧义/非法条目；
5. 显式格式语义始终覆盖本推断。

示例：

| 纯文本输入 | eTLD+1 | 结果 |
|------------|--------|------|
| `mzstatic.com` | `mzstatic.com` | `domain/suffix` |
| `a1.mzstatic.com` | `mzstatic.com` | `domain/exact` |
| `example.co.uk` | `example.co.uk` | `domain/suffix` |
| `www.example.co.uk` | `example.co.uk` | `domain/exact` |
| `foo.github.io` | `foo.github.io`（PRIVATE 规则） | `domain/suffix` |
| `www.foo.github.io` | `foo.github.io` | `domain/exact` |
| `localhost` | 无可靠 eTLD+1 | 拒绝，不猜测 |

实现依据可直接复用项目已有 `golang.org/x/net` 中的 `publicsuffix` 与 `idna`，无需另建“常见后缀数量表”。PSL 是随依赖版本更新的数据快照；同步回执和测试应记录/锁定关键用例，避免依赖升级造成未察觉的分类变化。

### 4.3 IP 与 CIDR 规范化

- IPv4/IPv6 单地址分别转为 `/32`、`/128` 的单主机 CIDR；
- CIDR 统一为网络地址形式，消除主机位差异；
- IPv4 与 IPv6 保留同一 `family=ip, matcher=cidr` 语义，由目标适配器选择 `IP-CIDR` 或 `IP-CIDR6` 输出名；
- ASN 只接受正整数，源中的 `AS` 前缀是否允许由对应适配器显式定义；
- IP、CIDR、ASN 不能进入域名 PSL 推断。

### 4.4 过滤与语义去重

清洗顺序固定为：

```text
源条目提取 → 类型校验 → 值规范化 → 语义校验 → 语义去重 → 目标能力统计
```

过滤项至少包括：

- 空行和合法注释；
- 控制字符、换行注入、非法逗号和超长值；
- HTML/XML 标签、脚本片段和明显错误页文本；
- 不符合所选适配器结构的键、标题与元数据；
- 未知规则类型；
- 非法域名、CIDR、正则或逻辑表达式；
- 同一同步结果中的语义重复项。

去重不得跨 matcher 合并。例如 `DOMAIN,example.com` 与 `DOMAIN-SUFFIX,example.com` 必须同时保留，因为前者只匹配完整域名，后者还匹配子域。

### 4.5 不支持规则的处置

区分三种结果：

| 状态 | 含义 | 处理 |
|------|------|------|
| `rejected_at_source` | 源结构或值无效，无法形成可靠语义 | 不入库，记录原因 |
| `accepted_canonical` | 已形成合法统一规则 | 入库，可供装配 |
| `unsupported_for_target` | 统一规则合法，但目标平台不能表达 | 仍可保留在素材池；针对该目标渲染时跳过并生成回执 |

不得因为 SR 当前不支持某个规则类型，就在导入阶段删除一条 Clash 可用的合法规则；也不得为了同时支持 SR 而把复杂规则扩大成语义不同的简单规则。

---

## 五、Clash 与 SR 目标适配

### 5.1 目标能力矩阵

目标适配器以能力表驱动，而不是在源解析器中写平台分支。初始映射方向如下：

| 统一语义 | Clash/Mihomo 输出 | SR 输出 |
|----------|-------------------|---------|
| `domain/exact` | `DOMAIN` | `DOMAIN` |
| `domain/suffix` | `DOMAIN-SUFFIX` | `DOMAIN-SUFFIX` |
| `domain/keyword` | `DOMAIN-KEYWORD` | `DOMAIN-KEYWORD` |
| `domain/wildcard` | `DOMAIN-WILDCARD`（经语法校验） | 按已验证能力输出；当前项目能力表不支持时跳过 |
| `domain/regex` | `DOMAIN-REGEX` | 当前项目能力表不支持时跳过 |
| `ip/cidr` | `IP-CIDR` / `IP-CIDR6` | `IP-CIDR` / `IP-CIDR6` |
| `ip/asn` | `IP-ASN` | 当前项目能力表不支持时跳过 |
| `process/exact` | `PROCESS-NAME` 等 | 按现有 SR 支持子集输出 |

该表是设计方向，不等同于最终完整平台清单。Build 前必须以项目目标版本的 Clash/Mihomo 与 SR 语法样本逐项验证，形成可测试的能力注册表。

### 5.2 渲染规则

- 装配器只读取结构化字段，不读取 `source_format` 或原始文本决定输出。
- 装配页面指定的代理组/PROXY/DIRECT 仍是最终 policy；源文件 policy 不覆盖管理员选择。
- `no_resolve` 仅在目标规则类型支持时输出；不支持时进入转换提示。
- 目标语法的引号、转义、逗号和 YAML 编码由目标渲染器统一负责。
- 不做默认语义降级。只有经设计证明完全等价的转换才可自动执行，并须在回执中计数。
- 同一结构化素材分别生成 Clash 与 SR 时，允许有效输出数量不同，但差异必须在预览/生成回执中可见。

### 5.3 目标转换回执

每次预览与生成至少返回：

- 输入结构化规则数；
- 成功输出数；
- 自动等价转换数；
- 因目标不支持而跳过的数量，按规则类型/匹配方式分组；
- 因目标约束再次校验失败的数量与样例；
- 最终输出规则数。

若目标不支持项占比异常高，界面应显示明显警告；是否阻止生成由后续 Build 的交互决策确定，不能静默生成一份与管理员预期差异巨大的结果。

---

## 六、同步安全、结构化存储与兼容

### 6.1 单 URL 同步回执

现有新增/删除/跳过回执扩展为：

| 字段 | 说明 |
|------|------|
| `detected_format` | 实际选中的源格式 |
| `confidence` | 格式置信度或等级 |
| `input_items` | 适配器看到的候选条目数 |
| `accepted` | 形成合法 Canonical Rule 的数量 |
| `rejected` | 无法形成可靠规则的数量 |
| `duplicates` | 规范化后的语义重复数量 |
| `by_family_matcher` | 按对象类别与匹配方式统计 |
| `unsupported_by_target` | 对 Clash/SR 的预估不支持数量 |
| `diagnostics` | 有上限的错误原因与脱敏样例 |
| `previous_accepted` | 该 URL 上一次成功快照规模，用于变化对比 |

逐项诊断需要限量、聚合和截断，避免数万条错误撑大任务 JSON；完整原文不直接写入日志或 API 回执。

### 6.2 更新保护

保留 Design2 的“空响应、零有效条目、部分失败时保留旧数据”原则，并增加：

- HTTP 成功但响应疑似 HTML、登录页或网关错误页 → 失败，保留旧数据；
- 格式置信度不足或多个格式冲突 → 失败，保留旧数据；
- 有效识别比例过低 → 失败或进入待确认，保留旧数据；
- 检测到源格式相较上次发生变化 → 明确告警，必要时等待确认；
- 有效条目相较上次突然大幅下降 → 不直接执行差量删除，进入保护状态；
- 解析器内部错误、超长行或结构读取不完整 → 失败，保留旧数据。

“识别率阈值”“突然下降阈值”和“格式变化是否必须人工确认”仍是待定交互参数，必须基于样本测试后由用户确认，不在本文中擅自给出数值。

### 6.3 来源追踪与多 URL 去重

结构化条目应能追踪到源 URL、源格式和源位置。多个 URL 产生同一语义规则时：

- 素材池内只渲染一次；
- 来源关系不能因去重完全丢失，否则单个 URL 消失时无法判断条目是否仍由其他 URL 提供；
- 后续数据模型应把“规则实体”与“规则—来源关系”分开，或提供等价的多来源关联设计；
- 差量删除以来源快照为依据，只在所有成功来源都不再提供该语义规则时删除 URL 来源条目。

该项比现有单条 `source_url` 更严格，是 Design3 数据模型调整的重点之一。

### 6.4 迁移与兼容原则

- 现有 manual 条目必须保留，按其显式 `rule_type + match_value` 转换为统一语义，不对手动条目执行 PSL 猜测。
- 现有 URL 条目不能仅凭当前结果反推原始格式；迁移后应在下一次成功同步时按新适配器重建来源快照。
- 在新同步首次成功前保留旧 URL 条目，防止升级即清空素材池。
- 现有装配蓝图引用素材池 ID，不改变素材池选择方式；新结构化条目在渲染时保持现有 `sort_order` 与池级顺序语义。
- 同步切换新解析器后，因 `a1.mzstatic.com` 等裸子域从 suffix 改为 exact，可能产生真实输出差异；迁移预览必须展示类型变化计数与样例。

### 6.5 排序语义

继续保留 Design2 的 manual 段在前、URL 段在后以及源内首次出现顺序。结构化展开时：

- 一个源条目展开多条规则，按源位置和字段顺序稳定排列；
- 去重后保留最早出现位置作为主排序位置；
- 多来源共同提供同一规则时，不因某一来源暂时消失而无故改变其稳定排序；
- 规则顺序仍具有实际匹配语义，任何重新排序必须有确定规则并可测试。

---

## 七、管理页面设计要求

### 7.1 URL 输入区必须直接告知纯文本推断规则（已确认）

素材池新建/编辑页面的 URL 区域必须常驻或通过紧邻输入框的帮助说明直接展示以下口径，不能只放在外部文档：

> 系统会先识别数据源格式。对于没有 `full:`、`+.`、`DOMAIN` 等显式标记的纯域名文本，将依据 Public Suffix List 判断：主域名按后缀匹配，例如 `mzstatic.com`；带额外子域的完整地址按精确匹配，例如 `a1.mzstatic.com`。显式格式标记始终优先。无法可靠判断的条目不会导入，并会显示在同步回执中。

同时提供可展开示例：

| 输入 | 识别结果 |
|------|----------|
| `full:a1.mzstatic.com` | 精确域名 |
| `a1.mzstatic.com`（纯文本） | 精确域名 |
| `mzstatic.com`（纯文本） | 域名后缀 |
| `a1.mzstatic.com`（Mihomo domain YAML payload） | 精确域名 |
| `+.001wifi.com` | 域名后缀 |
| `DOMAIN-KEYWORD,openaiapi` | 域名关键字 |

页面必须强调“同一行的含义可能受整份文档格式影响”，避免用户把 Mihomo YAML 的裸主域规则与纯文本推断混为一谈。

### 7.2 URL 格式设置

每个 URL 默认使用“自动识别”。为处理低置信度或特殊来源，建议提供显式格式覆盖选项：

- 自动识别（默认）；
- Legacy/纯域名文本；
- Mihomo domain YAML；
- Mihomo classical YAML；
- 显式类型规则文本；
- 纯 IP/CIDR；
- sing-box source JSON。

格式覆盖只选择源适配器，不允许跳过条目校验。该交互是否首期实现，需在 Build 设计前确认。

### 7.3 同步回执与历史

同步完成后每个 URL 展示：

- 检测格式与置信度；
- 接受/拒绝/重复/新增/删除数量；
- exact/suffix/keyword/wildcard/regex/CIDR/ASN 等分类统计；
- 与上次成功同步的数量、格式和类型分布变化；
- 有上限的拒绝原因与代表样例；
- Clash 与 SR 的目标能力预估；
- 是否触发保护、是否保留旧数据。

“同步成功”不能只表示 HTTP 200；至少要表示格式已可靠识别且生成了通过校验的结构化规则。

### 7.4 条目列表

URL 条目列表从“规则类型 + 匹配值”扩展为：

- 对象类别；
- 匹配方式；
- 规范化值；
- 来源格式与 URL；
- 多来源数量；
- Clash/SR 支持状态；
- 诊断信息入口。

仍采用后端分页与 URL 区懒加载，不能因增加元数据而整表加载。

### 7.5 预览与生成提示

装配预览应在正文外显示目标转换摘要。存在不支持规则时：

- 明确展示“结构化规则 N 条，目标输出 M 条，跳过 K 条”；
- 可按类型查看跳过原因；
- 不把跳过项混入生成正文；
- 不以普通成功提示掩盖大量规则未输出。

---

## 八、影响范围与构建前置

### 8.1 预计受影响模块

| 模块 | 影响 |
|------|------|
| `backend/internal/pool/` | 将单一逐行解析拆为格式探测器、源适配器、清洗器与同步保护 |
| `backend/internal/rulespec/` | 从目标平台规则白名单扩展为统一语义校验 + 目标能力映射，或拆分职责 |
| `backend/internal/assembly/` | 只读取统一结构；Clash/SR 渲染按能力表输出并生成回执 |
| 数据库迁移 | 结构化字段、格式元数据、多来源关系与同步诊断 |
| 素材池 API | URL 格式设置、扩展同步回执、结构化条目字段 |
| `frontend/src/views/admin/assembly/` | URL 帮助说明、格式识别结果、结构化列表与目标转换提示 |
| 测试样本 | 两份 DailyData、AINA、Mihomo classical/domain、CIDR、sing-box JSON、HTML/错误页与歧义反例 |

### 8.2 与 Design2 的关系

本文经确认并构建后，将覆盖 Design2 §2.3 中以下现行口径：

- “裸域名一律 DOMAIN-SUFFIX”改为“先识别文档格式；仅纯文本无显式语义时按 PSL/eTLD+1 推断”；
- “标准规则行直接入库”改为“先转统一结构，经清洗与语义校验后入库”；
- 拼装器从读取原平台式 `rule_type + match_value` 改为读取 Canonical Rule 并按目标平台渲染。

Design2 §2.2 的 manual/url 来源隔离、排序原则，§2.4 的异步同步、大小/超时限制和失败保留原则，以及第三～四章的池级选择、版本快照和分发机制继续有效，并由本文补充而非删除。

在 Design3 尚未评审、Build 尚未完成前，Design2 和当前实现仍是运行事实，不得仅依据本文修改生产数据或宣称已经支持新格式。

### 8.3 构建前必须补齐的研究与决策

1. 用真实样本集确定探测评分、最低识别率和异常缩量阈值。
2. 逐项验证目标版本 SR 的规则支持范围，形成能力矩阵测试，不凭印象扩充。
3. 决定复杂 sing-box 组合规则是首期保留逻辑结构还是整项拒绝。
4. 确认 URL 显式格式覆盖是否进入首期。
5. 确认目标不支持项占比达到何种条件时阻止生成。
6. 设计现有 `pool_entries` 向统一规则与多来源关系迁移的事务和回滚方案。
7. 确认是否持久化有限原始样例；默认建议只保留位置、摘要和受限诊断，避免保存完整第三方响应。

以上事项存在产品或数据兼容取舍，编写 Build 前必须按 AGENTS.md 与用户确认，不在实现阶段自行选择。

### 8.4 验收方向

后续 Build 至少覆盖：

- 两份 DailyData 样例得到预期 exact/suffix 分类；
- PSL 多级公共后缀与 PRIVATE 后缀用例；
- AINA typed text 的 DOMAIN/DOMAIN-SUFFIX/DOMAIN-KEYWORD 保真；
- Mihomo domain/classical YAML 的结构化解析与 YAML 键防误识别；
- sing-box JSON 基础字段与复杂组合拒绝路径；
- IPv4、IPv6、单地址和 CIDR 规范化；
- HTML 200、空响应、登录页、超长行、低识别率和异常缩量保护；
- 同义语法去重、不同 matcher 不误合并、多 URL 同源关系；
- Clash/SR 双目标映射、跳过回执和无有损降级；
- 现有 manual 条目、排序、差量同步、装配快照与版本分发回归；
- 后端编译、静态检查、全量测试及前端构建。

---

## 九、研究依据

### 9.1 项目内样例与现状

- [DailyData.txt.template.md](docs/DocTemplates/DailyData.txt.template.md)：现有 legacy `full:` + 裸域名样例。
- [DailyData.txt.template2.md](docs/DocTemplates/DailyData.txt.template2.md)：新增 Mihomo `payload` domain provider 样例。
- [Design2.md](Design2.md) §2～§4：现有素材池、同步、装配和分发设计。
- 当前实现的素材池解析、同步、规则校验与 Clash/SR 渲染链路。

### 9.2 外部格式依据（核验日期：2026-08-31）

- [Mihomo 路由规则](https://wiki.metacubex.one/config/rules/)：DOMAIN、DOMAIN-SUFFIX、DOMAIN-WILDCARD、DOMAIN-REGEX、IP-CIDR、IP-ASN 等规则语义。
- [Mihomo rule-providers 文件内容](https://wiki.metacubex.one/config/rule-providers/content/)：classical、domain、ipcidr 的 YAML/text 结构。
- [sing-box rule-set Source Format](https://sing-box.sagernet.org/configuration/rule-set/source-format/)：source JSON 顶层版本与 rules 结构。
- [Public Suffix List](https://publicsuffix.org/)：纯文本主域/eTLD+1 判定依据。
- [AINA.txt](https://raw.githubusercontent.com/iab0x00/ProxyRules/main/Rule/AINA.txt)：显式类型规则文本实例。

外部格式可能随上游版本变化。实现时必须锁定目标版本或测试语料，并在依赖/规则语法升级时运行回归测试。

---

## 十、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v0.1 | 2026-08-31 | 初始预想稿：确认 URL 多格式自动识别 → 平台无关结构化规则 → 清洗校验 → Clash/SR 双目标渲染主链路；确认纯文本域名采用 PSL（含 PRIVATE）eTLD+1 区分 exact/suffix，并要求页面直接告知用户；纳入 legacy、Mihomo domain/classical、显式类型文本、CIDR 与 sing-box JSON 的适配方向 |
