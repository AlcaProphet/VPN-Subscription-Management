# Clash 订阅校验、格式要求、Emoji 处理与对外 API 参考

> **文档定位：** 本文继续从 `clash-verge-rev` 源码中提取“订阅是否正常”的检测逻辑、Clash 订阅格式要求，以及 Emoji/非 ASCII 内容在 YAML、HTTP 头、URI 中的处理方式；同时结合当前项目已有的订阅下载 API，整理“如何让 Clash Verge Rev 正确接收本系统下发的订阅与自定义响应头”。
> **主要来源：**
> - `clash-verge-rev/src-tauri/src/core/validate.rs`：核心校验器
> - `clash-verge-rev/src-tauri/src/config/prfitem.rs`：远程订阅导入校验与响应头解析
> - `clash-verge-rev/src-tauri/src/enhance/mod.rs`：最终配置装配与清理
> - 当前项目 `backend/internal/download/download.go`、`backend/internal/platform/platform.go`、`backend/internal/server/download.go`
> - 本机实测：Go `gopkg.in/yaml.v3` 与 `js-yaml` 对 `\U0001F680` 的处理

---

## 一、Clash Verge Rev 如何检测一份订阅是否“正常”

### 1.1 远程订阅导入时的格式校验（`prfitem.rs::from_url`）

【源码事实】导入远程订阅时，Clash Verge Rev 只做轻量“可导入”校验：

1. HTTP 下载必须成功。
2. 响应体去除 UTF-8 BOM 后必须能被 `serde_yaml_ng` 解析为 YAML Mapping。
3. **必须包含顶层 `proxies` 或 `proxy-providers` 之一**，否则报错：
   ```text
   profile does not contain `proxies` or `proxy-providers`
   ```
4. 不强制要求 `proxy-groups`、`rules`、`dns`、`tun` 等项存在。
5. 下载内容直接保存为本地 YAML 文件，此时不会用 Clash 内核做完整验证。

结论：**对订阅分发系统的要求，至少保证输出 YAML 顶层有 `proxies` 或 `proxy-providers`。** 如果只给空 `proxies: []`，也算有 `proxies` 键，可以通过导入，但实际激活时仍可能被内核拒绝。

### 1.2 文件保存时的验证（`cmd/save_profile.rs` + `core/validate.rs`）

【源码事实】用户在 Clash Verge Rev 中编辑 Merge、Script、Rules、Proxies、Groups 文件并保存时，会调用 `CoreConfigValidator::validate_config_file_outcome`：

- **Merge 文件**：只做 YAML 语法检查，不启动内核。
- **Script 文件**：用内置 JS 引擎检查语法，并检查是否包含 `function main` / `const main` / `let main`。
- **普通 YAML/配置文件**：会调用 Clash 内核进行完整验证。

### 1.3 最终激活订阅/更新配置时的完整验证

【源码事实】切换订阅、编辑影响运行时的扩展文件、删除当前订阅时，流程是：

1. `enhance::enhance` 把“原始订阅 + Merge + Script + Rules + Proxies + Groups + 全局 Merge/Script + 默认配置”合成为最终运行时配置。
2. 最终配置写入 `clash-verge-check.yaml`。
3. 调用内核二进制：
   ```text
   <core> -t -d <app_data_dir> -f <config_path>
   ```
4. 如果进程退出码非 0，或 stderr 中出现 `FATA` / `fatal` / `Parse config error` / `level=fatal`，则判定失败。
5. 验证失败时，Clash Verge Rev 会回滚到之前的配置，不切换订阅。

【源码事实】`ValidationOutcome` 的状态有：

| 状态 | 含义 |
|------|------|
| `Valid` | 通过 |
| `Invalid` | 失败，带 `kind` 和 `message` |
| `Skipped` | 跳过（退出中 / 防抖） |
| `Busy` | 已有一次校验在跑 |

`ValidationErrorKind` 包括：`FileMissing`、`FileRead`、`YamlSyntax`、`YamlMapping`、`ScriptSyntax`、`ScriptMissingMain`、`CoreRejected`、`ProcessTerminated`、`Timeout`。

【建议】当前项目装配器在“生成版本”之前已经做了空产物/引用校验，但仍建议增加一个“可选的自检/试跑”能力：
- 若后端环境内置或可调用 mihomo/meta core，可生成临时 YAML 后执行 `core -t`。
- 如果无法内置 core，至少应在生成前做以下静态检查：
  - YAML 能否被 `yaml.v3` 解析（避免字段类型/缩进错误）。
  - 顶层是否存在 `proxies` 或 `proxy-providers`。
  - 所有 `proxy-groups[*].proxies` 引用的节点/组/内置策略是否存在。
  - 所有 `rules[*]` 的目标是否指向存在的组或内置策略。
  - 所有节点是否有 `name`、`type`、`server`、`port` 等该协议必需字段。
  - 空 `select` 组应被拒绝（当前项目已有）。

---

## 二、Clash 订阅格式要求

### 2.1 最小可导入结构

【源码事实】Clash Verge Rev 内置本地模板（`src-tauri/src/utils/tmpl.rs`）：

```yaml
proxies: []

proxy-groups: []

rules: []
```

远程导入只要求：

```yaml
proxies: []          # 或
proxy-providers: {}  # 之一存在即可
```

但“能导入”不等于“能激活”。真正能被 Clash/mihomo 加载并正常使用的订阅，至少要能被内核 `-t` 接受。

### 2.2 推荐订阅结构（当前项目模板可作为基线）

```yaml
# 可选：监听/模式/DNS/TUN 等
mixed-port: 7890
mode: rule
allow-lan: false
log-level: info
ipv6: false
dns:
  enable: true
  enhanced-mode: fake-ip
  fake-ip-range: 198.18.0.1/16

# 必需或强烈建议
proxies:
  - name: "节点A"
    type: vless
    server: example.com
    port: 443
    uuid: "xxxx"
    network: tcp
    tls: true
    servername: example.com

proxy-groups:
  - name: "🌎国外流量"
    type: select
    proxies:
      - "节点A"

rules:
  - DOMAIN-SUFFIX,example.com,🌎国外流量
  - GEOIP,CN,DIRECT
  - MATCH,🌎国外流量

# 可选扩展
proxy-providers: {}
rule-providers: {}
sub-rules: {}
```

### 2.3 Clash Verge Rev 对订阅内容的具体读取依赖

【源码事实】以下是 Clash Verge Rev 自身会直接读取/操作的顶层键：

| 顶层键 | 用途 |
|--------|------|
| `proxies` | 节点列表；没有则必须要有 `proxy-providers` |
| `proxy-providers` | 代理集合；代理组可通过 `use` 引用 |
| `proxy-groups` | 代理组列表 |
| `rules` | 规则列表 |
| `rule-providers` | 规则集，供 `RULE-SET` 引用 |
| `sub-rules` | 子规则，供 `SUB-RULE` 引用 |
| `mode` / `mixed-port` / `socks-port` / `port` / `external-controller` / `secret` / `allow-lan` / `log-level` / `ipv6` / `tun` / `dns` 等 | 运行时配置，Clash Verge Rev 会合并自己的默认值/控制面 |

【建议】当前项目生成 Clash 订阅时，应至少输出：
- 安全可用的头部参数（端口、mode、DNS、TUN 等至少不给冲突值）；
- `proxies`（手动节点或 `# {{xray_nodes}}` 占位）；
- `proxy-groups`（至少包含可直接引用的组，并保证所有节点引用有效）；
- `rules`（不能为空也可，但至少要有 `GEOIP,CN,DIRECT` + `MATCH,兜底组`）。

### 2.4 规则格式

已整理在 [Clash-Verge-Rev-Subscription-Assembly.md](Clash-Verge-Rev-Subscription-Assembly.md)。补充强调：

- 规则行是 `TYPE,value,target`，IP 规则可追加 `no-resolve`。
- Clash 目标必须为存在的代理组名或内置策略 `DIRECT` / `REJECT` / `REJECT-DROP` / `PASS`。
- 当前项目已有 `GEOIP,CN,DIRECT` + `MATCH,🛟无法归属的流量` 兜底，符合常见模板。

### 2.5 编码要求

【源码事实】远程订阅导入时会：
- 去除 UTF-8 BOM；
- 按 UTF-8 字符串读取并解析 YAML。

因此订阅内容必须是 **UTF-8**，不要用 GBK/UTF-16。

---

### 2.6 Clash 顶层高级参数（装配器可扩展的头部/全局设置）

【源码事实】结合 `src-tauri/src/config/clash.rs` 的默认模板与当前项目 `docs/DocTemplates/ClashOfficial.yaml.template.md`，Clash/mihomo 顶层常见高级参数包括：

| 分类 | 常见键 |
|------|--------|
| 监听/端口 | `mixed-port`、`port`、`socks-port`、`redir-port`、`tproxy-port`、`allow-lan`、`bind-address`、`authentication`、`skip-auth-prefixes`、`lan-allowed-ips`、`lan-disallowed-ips` |
| 模式/日志 | `mode`（`rule`/`global`/`direct`）、`log-level`、`ipv6`、`unified-delay`、`find-process-mode` |
| 控制面 | `external-controller`、`external-controller-tls`、`external-controller-cors`、`external-controller-unix`、`external-controller-pipe`、`secret`、`external-ui`、`external-ui-name`、`external-ui-url`、`external-doh-server` |
| DNS | `dns.enable`、`dns.listen`、`dns.ipv6`、`dns.enhanced-mode`、`dns.fake-ip-range`、`dns.fake-ip-range6`、`dns.fake-ip-filter`、`dns.fake-ip-filter-mode`、`dns.nameserver`、`dns.fallback`、`dns.default-nameserver`、`dns.proxy-server-nameserver`、`dns.direct-nameserver`、`dns.direct-nameserver-follow-policy`、`dns.nameserver-policy`、`dns.use-hosts`、`dns.use-system-hosts`、`dns.fallback-filter`、`dns.prefer-h3`、`dns.respect-rules` |
| TUN | `tun.enable`、`tun.stack`、`tun.device`、`tun.auto-route`、`tun.auto-redirect`、`tun.auto-detect-interface`、`tun.dns-hijack`、`tun.route-exclude-address`、`tun.strict-route`、`tun.mtu` |
| 规则/数据 | `proxies`、`proxy-groups`、`proxy-providers`、`rules`、`rule-providers`、`sub-rules`、`geox-url`、`geo-auto-update`、`geo-update-interval`、`sniffer` |
| 杂项 | `hosts`、`tunnels`、`profile.store-selected`、`profile.store-fake-ip`、`experimental` |

【建议】当前装配器的“头部表单”目前只暴露少量顶层键。若要做通用 Clash 装配，可把上述参数拆成“基础/高级”两组，高级以 JSON/Merge 形式透传，同时保留系统控制面保护（端口、external-controller、secret、tun、mode 等不应被订阅内容随意覆盖）。

### 2.7 Proxy Provider / Rule Provider 格式要点

【源码事实】`docs/DocTemplates/ClashOfficial.yaml.template.md` 中给出了可用的 `proxy-providers` 结构：

```yaml
proxy-providers:
  provider1:
    type: http            # 或 file
    url: "https://..."
    interval: 3600        # 秒
    path: ./provider1.yaml
    proxy: DIRECT         # 可选：拉取时使用的代理
    header:               # 可选：自定义请求头（值为数组）
      User-Agent:
        - "Clash/v1.18.0"
    health-check:
      enable: true
      interval: 600
      url: https://cp.cloudflare.com/generate_204
    override:
      skip-cert-verify: true
      udp: true
      additional-prefix: "[provider1]"
      proxy-name:
        - pattern: "test"
          target: "TEST"
```

规则提供者（`rule-providers`）常见字段包括 `type`（`http`/`file`）、`behavior`（`domain`/`ipcidr`/`classical`）、`url`、`path`、`interval`、`format`（`yaml`/`text`/`mrs`）等。当前项目如未来支持 provider，建议只允许管理员配置 `http(s)` 类型，并结合现有安全校验限制路径与请求头注入。

【建议】如果当前拼装器只做“直接下发节点”，可以暂不引入 provider；但若代理组需要引用外部节点集合，建议至少支持“透传” `proxy-providers` 和 `rule-providers` 区块，而不是把 provider 展开成静态节点。

## 三、Emoji / 非 ASCII 内容的处理

### 3.1 当前项目中 YAML 输出会怎样

【实测事实】使用当前项目依赖的 `gopkg.in/yaml.v3` 序列化包含 emoji 的字符串时，例如：

```go
yaml.Marshal("🚀直接连接")
```

输出为：

```yaml
"\U0001F680直接连接"
```

原因是 `yaml.v3` 的 `is_printable` 只承认到 `U+FFFD` 附近的 1~3 字节 UTF-8，4 字节的 emoji 会被视为不可直接输出而转义为 `\U0001F...`。

【实测事实】这个输出可以被：
- Go `gopkg.in/yaml.v3` 正确解析回 `🚀直接连接`；
- `js-yaml` 正确解析回 `🚀直接连接`。

所以它在“严格 YAML 语义”上通常可行。但在实际使用中仍可能带来两个问题：
1. **可读性差**：用户或客户端直接查看订阅文本时看到的是 `\U0001F680`，不是 emoji。
2. **兼容性风险**：部分较老的 YAML 解析器、工具链、日志展示可能不识别 `\U` 8 位转义，或把它显示为字面 `U0001F680`。

### 3.2 建议的 YAML Emoji 输出方案

建议在装配器最终 YAML 序列化后增加一个 **安全的 Unicode 转义还原步骤**，把 YAML 中的 `\Uhhhhhh` / `\uhhhh` 转义还原为真实 UTF-8 字符。

实现时需注意：
- 不能简单全局字符串替换，否则会误伤用户名称中真实存在的字面 `\U...` 文本。
- 应基于 YAML 双引号标量中的转义序列做识别，且要区分：
  - `\U0001F680` = 真实 emoji 转义；
  - `\\U0001F680` = 字面反斜杠 + U...（不应还原）。
- 也可以选择“只对 `name` / 代理组名 / 规则目标等已知业务字符串字段”，在写 YAML 前自行做一次手工编码，例如写单引号原始值或使用自定义 emitter。

如果采用 post-process，需要保留 YAML 合法转义（`\"`、`\\`、`\n` 等），只还原 `\U` / `\u` 中的 Unicode 字符。

【建议】最稳妥的验收方式：
1. 生成一份包含 `🚀直接连接`、`🌎国外流量`、`😀节点` 的 Clash YAML。
2. 在 Go/JS/Clash 客户端中分别解析一次。
3. 确认解析后的名称和代理组引用完全一致，且不会出现 `U0001F680` 这类断裂文本。

### 3.3 文件名 / HTTP 头中的 Emoji

当前项目在下载端点设置 `Content-Disposition` 时使用：

```go
c.Header("Content-Disposition", `attachment; filename="`+SanitizeFilename(name)+`"`)
```

【风险】如果 `name` 含中文/emoji，HTTP 头中的非 ASCII 字符可能：
- 不符合 RFC 6266 / RFC 5987 的推荐做法；
- 在部分客户端中显示为乱码；
- 与 Clash Verge Rev 的 `Content-Disposition` 解析逻辑不完全匹配。

【源码事实】Clash Verge Rev 解析顺序是：
1. 在 `Content-Disposition` 中按 `;` 分段找 `filename*`，取到值后做百分号解码，再取 `''` 后的部分（即 RFC 5987 的 `filename*=UTF-8''...`）。
2. 如果没有 `filename*`，回退到 `filename`。

因此当前项目应同时输出两种形式：

```http
Content-Disposition: attachment; filename="fallback.yaml"; filename*=UTF-8''%E6%88%91%E7%9A%84%E8%AE%A2%E9%98%85.yaml
```

建议后端封装一个函数：
- `filename`：只放 ASCII 安全回退名（例如 `subscription.yaml` 或去掉非 ASCII 的简化名）。
- `filename*`：用 RFC 5987 百分号编码原始 UTF-8 文件名，支持中文/emoji。

### 3.4 节点链接 / URI 中的 Emoji

- SR/通用节点订阅的节点名会进入 `remarks` / `#fragment` / `ps`。
- 必须使用 `encodeURIComponent` 或等价百分号编码，而非直接写入原始 emoji。
- 当前项目已有 [Node-Link-Standards.md](Node-Link-Standards.md) 支撑，生成时应继续遵循。
- 代理组名、规则目标名不会出现在 URI 中，只出现在 YAML 或 conf 文本中。

### 3.5 名称校验与显示一致性

当前项目已经允许中文/emoji，并禁止控制字符、逗号、空格（节点名）和首尾空白。建议：
- 所有渲染层（Clash YAML、SR subs、generic subs、preview、diff、下载文件名）使用同一个“有效渲染名”。
- 在名称写入前做 NFD/NFC？不需要强制，但要确保 UTF-8 有效。
- 代理组引用和节点引用均使用稳定键，显示名变化只影响输出层，不破坏内部引用。

---

## 四、对外 API：如何让 Clash Verge Rev 接收本系统订阅和自定义响应头

### 4.1 当前项目已有的订阅下载 API

【源码事实】当前项目已有以下公开下载端点：

| 端点 | 用途 |
|------|------|
| `GET /subscriptions/:platform/download?token=...` | 用户订阅下载，按平台唯一订阅 + Token |
| `GET /share/:slug/download?token=...` | 分享订阅 |
| `GET /rules/:slug/download?token=...` | 分流规则下载 |
| `GET /api/subscriptions/preview?platform=...` | 会话预览 |

这些端点会返回：
- `text/plain; charset=utf-8`
- `Cache-Control: no-store` 等禁缓存头
- 平台附加的 `extra_headers`
- `Content-Disposition`

### 4.2 如何让 Clash Verge Rev 导入本系统订阅

Clash Verge Rev 的“订阅”导入只需一个 HTTP/HTTPS URL。因此：

1. 管理员为用户/平台生成订阅下载链接，形如：
   ```
   https://vpn.example.com/subscriptions/clash-verge/download?token=<long_token>
   ```
2. 该 URL 的响应体必须是 UTF-8 YAML，且含 `proxies` 或 `proxy-providers`。
3. 用户把该 URL 粘贴到 Clash Verge Rev 的“新建配置/导入订阅”即可。
4. 如果使用平台 `schemes`，当前项目可生成一键导入链接，例如：
   ```
   clash://install-config?url=<encoded_subscription_url>
   ```

### 4.3 自定义响应头：当前项目已有的存储机制

【源码事实】平台表有 `extra_headers TEXT NOT NULL DEFAULT '{}'`，业务层 `platform.Service` 的 `Create/Update` 接收 `ExtraHeaders map[string]string`，校验规则：
- 键必须符合 HTTP 头名 token（RFC 7230）；
- 键和值都禁止 `\r`、`\n` 等控制字符；
- 值支持 `{frontend_url}` 占位符，下载时替换为前端地址。

当前默认预置的 Clash Verge 平台附加头为：

```json
{
  "Content-Disposition": "attachment; filename*=UTF-8''subscription.yaml",
  "profile-update-interval": "6",
  "profile-web-page-url": "{frontend_url}"
}
```

### 4.4 Clash Verge Rev 实际会读取哪些响应头

根据 `prfitem.rs`，Clash Verge Rev 会读取：

| 响应头 | 格式 | 作用 |
|--------|------|------|
| `subscription-userinfo` 或 `*‑subscription-userinfo`（如 `x-amz-meta-subscription-userinfo`） | `upload=...; download=...; total=...; expire=...` | 展示已用/总流量和到期时间 |
| `profile-update-interval` | 整数，**单位小时** | 自动更新间隔（乘以 60 变成分钟） |
| `profile-web-page-url` | 必须是带 host 的 `http/https` URL | 订阅主页 |
| `Content-Disposition` 的 `filename*` | `UTF-8''percent-encoded` | 下载文件名 |
| `Content-Disposition` 的 `filename` | ASCII 回退 | 下载文件名回退 |

【注意】`profile-update-interval` 单位是**小时**，不是分钟。当前项目预置为 `"6"`，即每 6 小时自动更新。如果要改为每 30 分钟，不能写 `30`，而应按小时写 `0.5`，但 Clash Verge Rev 解析为 `u64 * 60`，因此它只接受正整数小时。这是生态约定：设置过小的分钟级更新没有意义，Clash Verge Rev 自身也把最短自动更新间隔限制为 1440 分钟。

【建议】在当前项目下载逻辑中：
- 若希望 Clash Verge Rev 自动更新，应输出 `profile-update-interval` 整数小时（如 `6`、`12`、`24`）。
- 若希望用户手动更新，可不输出该头。
- 输出 `profile-web-page-url` 时确保是合法绝对 URL，且 `{frontend_url}` 已替换。
- 输出 `subscription-userinfo` 时仅使用整数 `upload/download/total/expire`，用 `;` 分隔。
- 输出 `Content-Disposition` 时同时给 `filename` 和 `filename*`，避免中文/emoji 文件名问题。

### 4.5 建议新增/调整的自定义响应头能力

当前项目的自定义响应头已能做到“平台级”。如果需要“每个订阅/分享/规则单独配置响应头”，可以参考该模式：

1. 在订阅、分享或版本上增加 `extra_headers`（JSON 字段），继承或覆盖平台级头。
2. 下载时按“资源自身头 > 平台头 > 系统注入头”合并。
3. 继续沿用现有防注入校验。
4. 对 `Content-Disposition` 做特殊处理：不再让用户手填原始头，而是保存“文件名”字段，由后端统一生成 RFC 5987 头，避免用户传入非法头。

### 4.6 推荐的响应头示例

```http
HTTP/1.1 200 OK
Content-Type: text/plain; charset=utf-8
Cache-Control: no-store, no-cache, must-revalidate
Pragma: no-cache
Content-Disposition: attachment; filename="subscription.yaml"; filename*=UTF-8''%E6%88%91%E7%9A%84%E8%AE%A2%E9%98%85.yaml
profile-update-interval: 6
profile-web-page-url: https://vpn.example.com
subscription-userinfo: upload=1024; download=2048; total=1073741824; expire=4102444800
```

---

## 五、面向当前拼装器的落地检查清单

1. **导入检查**
   - [ ] 输出 YAML 顶层含 `proxies` 或 `proxy-providers`。
   - [ ] 输出 UTF-8，无 BOM 问题（可容忍输入 BOM，但生成端统一 UTF-8）。
2. **结构检查**
   - [ ] `proxies` 每个节点都有 `name/type/server/port` 及该协议必填参数。
   - [ ] `proxy-groups` 每个组都有非空成员，且类型为 Clash 支持的 `select/url-test/fallback/load-balance/relay`。
   - [ ] 所有规则目标都存在。
3. **引用检查**
   - [ ] `proxy-groups[*].proxies` 引用的节点/组/内置策略均有效。
   - [ ] `proxy-groups[*].use` 引用的 provider 均存在（若未来支持）。
   - [ ] `rules` 或 `RULE-SET`/`SUB-RULE` 的引用均存在。
4. **Emoji/文件名检查**
   - [ ] YAML 中 emoji 名称可被 Go/JS/Clash 解析回原始字符串。
   - [ ] 下载文件名同时提供 ASCII `filename` 和 RFC 5987 `filename*`。
   - [ ] SR/generic 链接中的节点名已百分号编码。
5. **可交付性检查**
   - [ ] 最好能用 `mihomo -t` 做一次真实校验；不能的话至少完成上面的静态检查。
   - [ ] 提供可直接粘贴的 URL，且 URL 带 token。
   - [ ] 对 Clash Verge Rev 提供 `profile-update-interval` 等可选头。
