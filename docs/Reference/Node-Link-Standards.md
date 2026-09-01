# Node-Link-Standards.md — 节点链接标准与 urlclash-converter 转换规则研究

> **文档定位：** 本文档是节点分享链接（URI）生成标准的取证研究，为 Design2.md 的 SR 节点订阅生成（4.5）与占位注入（5.7）提供映射规则依据。核心参照实现为 urlclash-converter（Clash ↔ Link 互转工具），并与 Shadowrocket 生态实际样例、互联网公开实践交叉验证。
> **核验来源：** 本地仓库 `/Users/kylechen/Desktop/Repo/urlclash-converter`（工作区快照，解析器移植自 clash-verge-rev `uri-parser.ts`，converter.ts:2）源码逐行取证；SR 原生参数格式经 `Shadowrocket.subs.template.md` 样例与互联网脚本实践交叉验证。
> **标注约定：** 【源码事实】= 直接引自代码行；【推断】= 由源码逻辑推出；【猜测】= 基于领域经验的推测。

---

## 一、urlclash-converter 总体结构

| 功能 | 入口 | 位置 |
|------|------|------|
| Link→Clash | `linkToClash` | src/converter.ts:20 |
| Clash→Link | `clashToLink` | src/converter.ts:83 |
| URI 解析分发 | `parseUri` | src/converter.ts:155（scheme 分发 :157-187，未知 scheme 抛异常 :186，上层捕获后丢弃该节点 :47-53） |
| URI 生成 | `generateUri` | src/converter.ts:1391 |
| base64 订阅探测 | `tryDecodeBase64SubscriptionLinks` | src/converter.ts:58-80 |

支持的 scheme：ss / ssr / vmess / vless / trojan / anytls / hysteria2(hy2) / hysteria(hy) / tuic / wireguard(wg) / http / socks5。

## 二、各协议 URI 生成规则（generateUri，converter.ts:1391-1569）

**公共规则**【源码事实】：

- 名称统一 `encodeURIComponent(node.name || "Node")` 作为 `#fragment`（:1392）
- server 统一过 `punycodeDomain`（:1393；utils.ts:12-22 仅对含非 ASCII 的 label 加 `xn--` 编码）
- 查询参数用 `URLSearchParams`（自动百分号编码）；布尔统一输出 `"1"`；无参数不输出 `?`

| 协议 | scheme 与凭据（编码） | 查询参数 | fragment |
|------|---------------------|---------|----------|
| ss（:1397-1400） | SIP002：`ss://utf8Base64(cipher:password)@server:port`；cipher 缺省 `auto` | **无**（不输出 plugin——【推断】带混淆插件的 ss 节点经此转换必丢 plugin） | 标准 |
| vmess（:1402-1424） | `vmess://utf8Base64(JSON)`；JSON：v=2/ps/add/port/id/aid(0)/scy(auto)/net(tcp)/type=none/host/path/tls/sni/alpn/fp | 无 | **不追加**（# 非 base64 合法字符破坏回读，注释 :1420-1422） |
| vless（:1426-1456） | `vless://uuid@server:port`（uuid 不编码） | 固定 `type`(tcp)/`encryption=none`；条件 `flow`/`security`(reality|tls)/`sni`/`fp`/`allowInsecure=1`/`alpn`；reality 专属 `pbk`/`sid`/`spx`/`pqv`/`ech` | 标准 |
| trojan（:1458-1465） | `trojan://urlenc(password)@server:port` | `type`(仅非 tcp)/`sni`/`allowInsecure=1`/`fp` | 标准 |
| anytls（:1467-1479） | 同 trojan 凭据形态 | `sni`/`alpn`/`client-fingerprint`/`allowInsecure=1`/`udp=1`/idle-session 三项 | 标准 |
| hysteria2（:1481-1495） | 同上 | `sni`/`obfs`/`obfs-password`/`insecure=1`/`alpn` | 标准 |
| tuic（:1497-1503） | `tuic://uuid:urlenc(password)@server:port` | `sni`/`alpn`/`allow_insecure=1`（下划线命名） | 标准 |
| hysteria（:1505-1518） | `hysteria://server:port`（**无凭据段**） | `protocol`/`auth`/`sni`/`upmbps`/`downmbps`/`alpn`/`obfs`/`mport`/`insecure=1` | 标准 |
| wireguard（:1520-1534） | `wireguard://urlenc(private-key)@server:port` | `public-key`/`address`(ip+ipv6)/`allowed-ips`/`pre-shared-key`/`reserved`(须 3 个)/`mtu`/`dns` | 标准 |
| http / socks5（:1536-1563） | `scheme://urlenc(user):urlenc(pass)@server:port`（有才输出） | `tls=1`/`fingerprint`/`skip-cert-verify=1`/（socks5 另有 `udp=1`）/`ip-version` | 标准 |

**重大事实**【源码事实】：`generateUri` **没有 ssr 分支**（switch 落 default :1565-1567 返回空串被 :140 `filter(Boolean)` 丢弃）——SSR 只进不出，Clash→链接时 SSR 节点被静默丢弃。

## 三、反向解析（parseUri）关键事实

**参数别名与大小写**【源码事实】：

- vless（:603-727）：key 统一转小写（:654）；`sni`/`peer`→servername（:666）；`fp`→client-fingerprint（:668）；`allowInsecure`/`skip-cert-verify`/`allowinsecure` 三写法（:670）；名称回退 fragment→remarks→remark（:657）
- trojan（:729-821）：key **不**转小写；仅识别 `skip-cert-verify`（:781），**不识别 `allowInsecure`**
- vmess（:406-598）：三格式并存——Quantumult / V2rayN JSON / **Shadowrocket base64+query**（:462-490）；`verify_cert` 取反→skip-cert-verify（:511）
- ss（:274-358）：SIP002 与旧式整串 base64 均接受；plugin 仅认 obfs-local/simple-obfs/v2ray-plugin，其它抛异常（:347）

**vless 的 TLS/REALITY 判定（对 SR 参数风格最关键的结论）**【源码事实】（:659-683）：

1. tls 开关完全由 `security` 参数驱动：`proxy.tls = security && security !== "none"`（:660）
2. `pbk`/`sid`/`spx`/`pqv`/`ech` **只在 `security === "reality"` 时读取**（:673-683）
3. Shadowrocket base64 专属分支（:608-624）：仅当 URI 形如 `vless://<base64>?...`（无 `@`，:610）时先 base64 解码再解析；此时 `tls=1/TRUE` 开启 tls 且 security 缺省**兜底为 `reality`**（:663）
4. 全文件无 `xtls` 字样——**不识别 `xtls=2`**
5. 非 base64 形态下 `peer` 可读出为 servername，但不触发 tls

**结论（对 Design2 4.5 的直接含义）**【推断】：SR 原生参数风格链接 `vless://uuid@host:port?tls=1&peer=&pbk=&sid=`（非 base64）经 urlclash-converter **只能部分回读**（uuid/server/port/name/peer 可读，tls 不开启、pbk/sid 丢失）；能被完整回读的形态只有两种：①标准风格 `security=reality&sni&fp&pbk&sid`；②Shadowrocket base64 userinfo 形态。**这不否定 SR 原生风格对 Shadowrocket 客户端本身的有效性**（见第六章交叉验证），只说明该工具不能作为 SR 原生风格的回读验证器。

## 四、编码细节与往返陷阱

**编码实现**【源码事实】：

- `utf8ToBase64`（:1383-1388）：TextEncoder→逐字节 Latin-1→btoa；中文/emoji 安全
- `decodeBase64OrOriginal`（:229-244）：atob 后用 `TextDecoder("utf-8",{fatal:true})` 试解，非法回退二进制串（保护 SS 二进制密码）
- punycode：含 IDN 的 server 一律改写为 punycode（生成 Clash 节点 :1338 与 URI :1393 两处）

**往返陷阱清单**【源码事实】+【推断】：

1. SSR 无生成支持（Clash→链接静默丢弃，:1565-1567）
2. trojan 往返丢 skip-cert-verify：生成写 `allowInsecure=1`（:1463）但解析不认（:761-807）
3. vless-ws/grpc 往返丢 host/path：生成侧完全不输出（:1426-1456），解析侧依赖（:696、:707）
4. ss 往返丢 plugin（:1397-1400 不输出）
5. 2022-blake3 系列加密被 `getCipher` 静默降级为 `auto`（:260-272）
6. SR base64 vless 兜底激进：`tls=1` 时 security 缺省填 `reality`（:663），普通 tls 节点会被误判为 reality
7. URLSearchParams 空格编码为 `+`，解析侧手工 `decodeURIComponent` 不还原 `+`（vless/trojan/tuic/hysteria/wireguard，如 :652-653）——【推断】参数值含空格的链接往返有损
8. 端口缺省一律兜底 443（trojan/anytls/hysteria/tuic/wireguard/http/socks，如 :737-739）

**base64 订阅探测判定链**【源码事实】（:58-80）：仅当首轮解析 0 个有效节点时触发 → 去空白、URL-safe 字符归一（`-`→`+`、`_`→`/`）→ 整体匹配 `^[A-Za-z0-9+/=]+$` → `=` 补齐 → atob 严格解码 → 按行拆分 → 至少一行匹配 `^[a-zA-Z][a-zA-Z0-9+.-]*:\/\/` 才认定。**Design2 的 SR subs（整体 base64、逐行 scheme URI、含 STATUS=/REMARKS= 非 URI 行）满足该判定，可被此类工具识别**。

## 五、对 Design2 装配器的可复用规则

**生成侧照此实现即可与主流生态互操作**【推断】：

1. **Clash YAML 输出**：manual 节点按存储字段原样输出（Design2 4.3 零转换口径成立）；深度清理空对象时注意保留 `false`/`0`（converter.ts:1366 只拦空串，可参照）
2. **fragment 名称**：`encodeURIComponent(名称)`，UTF-8 中文名安全
3. **ss**：SIP002 `ss://base64(cipher:password)@server:port#name`；避免 2022-blake3 系（工具链降级损坏）
4. **vmess**：V2rayN JSON + UTF-8 base64，名称放 `ps`，不追加 fragment
5. **端口必须显式**（缺省 443 兜底会掩盖配置错误）；参数值避免空格
6. **SSR 已从 Design2 移除**：urlclash-converter 对 SSR 只收不生成，自研编码无验证基准；本系统不再输出 ssr://，parseUri :360-404 仅作历史参照

## 六、SR 原生参数风格交叉验证（Design2 4.5 定稿口径的可靠性）

**两种 SR vless 链接形态并存**【源码事实（样例/公开脚本）】：

| 形态 | 特征 | 证据 |
|------|------|------|
| A. base64 userinfo + query | `vless://base64(cipher:uuid@host:port)?remarks=&tls=1&peer=&xtls=2&pbk=&sid=` | 本项目样例 `Shadowrocket.subs.template.md`（作者真实机场订阅）；fscarmen/sing-box 脚本的 SHADOWROCKET_SUBSCRIBE 生成段（互联网取证） |
| B. 标准 userinfo + query | `vless://uuid@host:port?remarks=&obfs=none&tls=1&peer=&pbk=` | fscarmen/sing-box 脚本同一函数的另一分支（互联网取证） |

**关键参数语义**【推断，基于样例与工具解析逻辑互证】：

- `xtls=2` = REALITY（SR 内部 TLS 类型枚举的数值化表达；converter 仅在 SR base64 分支兜底识别 tls=1→reality，不识别 xtls=2 字面量）
- `peer` = SNI（converter parseUri :666 将 peer 映射为 servername，佐证 SR 语义）
- `pbk`/`sid` = REALITY 公钥 / short-id（与标准风格同名，跨风格一致）
- SR base64 userinfo 首段 `cipher:`（如 `auto:`/`none:`）为 SR 兼容占位【猜测，基于样例解码结构与 vmess SR 格式类比】

**结论**【推断】：Design2 4.5 定稿的「SR 原生参数风格」有真实生态背书（样例 + 公开脚本双源），对 Shadowrocket 客户端有效；但需注意：①此类链接对 urlclash-converter 等标准风格解析器**不可完整回读**（第三章结论）；②本项目系统内部**不需要回读自己生成的链接**（节点数据始终存于 nodes 表，链接仅为输出形态），故该限制不构成设计风险；③Build 阶段建议以「形态 A（base64 userinfo）」为主输出形态，与作者样例一致。

## 七、Design2 装配器设计可行性验证汇总（对照第二~五章）

| Design2 设计点 | 验证结论 | 证据链 |
|---------------|---------|--------|
| **SR 双产物拆分**（3.4：subs 入订阅池、conf 入规则实体） | ✅ 可行且符合生态：SR 的节点与分流规则本就独立导入；SSPanel 亦将节点信息与规则骨架分开处理（无 SR conf 输出，clash 格式才含 rules） | 样例 Shadowrocket.subs.template.md（纯节点无规则）+ Shadowrocket.conf.template.md（纯规则无节点）+ SSpanel-Subscribe.md 第二章 |
| **subs 整体 base64 输出**（4.5） | ✅ 可行：生态主流形态（样例 + fscarmen 公开脚本双源）；base64 探测判定链兼容性已验证（本文第四章）；注意 SSPanel 采用「不整体 base64」形态，两种均被 SR 接受 | Node-Link-Standards.md 四/六章 |
| **Clash YAML 零转换渲染**（4.3：manual 节点按 protocol_json 原样输出） | ✅ 可行且优于参照系：urlclash-converter 的字段全量拷贝+空对象深度清理可参照（converter.ts:1333-1380）；SSPanel 的 array_merge+yaml_emit 同思路 | Node-Link-Standards.md 第五章 + SSpanel-Subscribe.md 第二章 |
| **占位标记 `# {{xray_nodes}}` 注入**（4.3/5.7） | ✅ 可行：注释行在 YAML 与 subs 明文（链接行列表）中均语法无害；SR 容忍非 URI 行（样例中 STATUS=/REMARKS= 头部行先例）；注入后全文重新 base64 仍满足探测判定链；SSPanel 不用占位而用「骨架+运行时插入」，两种均可行，占位方案与模板预览/上传兼容更好 | 样例头部行 + 本文第四章判定链 + SSpanel-Subscribe.md 第二章 |
| **SR 原生参数风格渲染**（4.5） | ✅ 对 SR 客户端有效（双源背书）；⚠️ 限制：不可被标准风格解析器完整回读（第三章），但本系统不回读自产链接，风险不成立；建议主输出用形态 A（base64 userinfo）与样例一致 | Node-Link-Standards.md 三/六章 |
| **不可转协议跳过+提示**（4.5） | ✅ 必要：snell/mieru/masque 等无 URI 标准；urlclash-converter 对 SSR 的处理（静默丢弃）是反面教材，本设计的「提示」更优 | 本文第二章重大事实 + 第七章 |
| **Xray 节点注入渲染**（5.7：vless/vmess 两种客户端表达） | ✅ 可行：Clash 端 XTLS 标准参数与 SR 端原生参数映射均已取证（本文二/六章 + Xray-Core-API.md §五传输字段）；vless Account 只需 id/flow/encryption（Xray-Core-API.md §11.3） | 交叉三源 |
| **用量响应头**（决策 #23） | ✅ 与生态完全对齐：SSPanel 四字段实现逐字段核验一致（total=总配额、expire=Unix 秒）；可低成本增配 Profile-Update-Interval 等头 | SSpanel-Subscribe.md 一/五章 + Xray-Core-API.md §八 |

**总体结论**【推断】：Design2 装配器设计（SR 双产物 / Clash YAML 渲染 / 占位注入）经三源交叉验证（源码取证 + 真实样例 + 生态面板对照）技术可行，无需设计层变更；实现层需遵守的细则（编码规则、陷阱规避、API 调用形态）已分载于本文档与 SSpanel-Subscribe.md、Xray-Core-API.md。

## 八、陷阱总结（装配器实现必避）

1. **不可转链接的协议跳过并提示**（Design2 4.5 已定稿）：snell/mieru/masque/openvpn/ssh/shadowquic/trusttunnel/tailscale 等无 URI 标准
2. 中文节点名必须 URL 编码（#fragment 与 remarks 两处）；IDN 域名转 punycode
3. ss 带混淆插件的节点在 subs 中丢失 plugin——【推断】可接受（此类节点建议仅走 Clash YAML 产物）或提示管理员
4. base64 订阅编码前确保逐行合法（STATUS/REMARKS 头部行非 URI，客户端按行忽略——样例证实 SR 容忍）
5. URLSearchParams 风格的 `+`/空格不对称问题：本系统自行生成时用标准百分号编码（`%20`）可规避

---

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-15 | 新增第七节 Design2 装配器设计可行性验证汇总（SR 双产物/Clash YAML 渲染/占位注入/SR 原生参数/Xray 渲染/用量头逐项对照证据链） |
| 2026-08-15 | 新建：urlclash-converter 全协议 URI 生成/解析取证（converter.ts 逐行核验）+ SR 原生参数风格双源交叉验证（Shadowrocket.subs.template.md 样例 + 互联网公开脚本），服务于 Design2.md 4.5/5.7 |
