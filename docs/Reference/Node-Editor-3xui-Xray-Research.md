# Node-Editor-3xui-Xray-Research.md — 3x-ui / Xray 客户端样例 / 项目节点处理对照研究

> **文档定位：** 本文是节点编辑器改进研究的新增资料，承接 [Node-Editor-Design-Research.md](Node-Editor-Design-Research.md)、[Node-Editor-Improvement-Directions.md](Node-Editor-Improvement-Directions.md)、[Design4.md](../../Design4.md) 以及 [xray-client-side.md](xray-client-side.md)、[Node-Link-Standards.md](Node-Link-Standards.md) 等既有证据。本文只做研究记录，不定义实现，不改动项目业务代码，也不代表对 3x-ui 或 Xray-examples 的修改或产品背书。
> **研究状态：** 2026-09-02。基于本机仓库 `~/Desktop/Repo/3x-ui`、`~/Desktop/Repo/Xray-examples`、`~/Desktop/Repo/clash-verge-rev` 和本项目当前代码静态分析；同时参考公开资料中的 3x-ui 页面与 Xray 配置说明。未构建、未联机调试、未改动外部分享项目。
> **标注约定：** 【3x-ui 事实】= 本地 3x-ui 源码观察；【样例事实】= Xray-examples 文件内容；【项目事实】= 当前项目代码或既有文档；【外部事实】= 公开文档或 Issue；【结论】= 由证据推导、供后续研究采纳；【候选】= 需要后续决策，不视为已定稿。

---

## 一、研究目的与范围

本轮研究的目的是回答以下问题：

1. 3x-ui 怎样组织节点/入站/出站编辑表单，表单、校验、持久化和 Xray 配置之间如何分工；
2. Xray-examples 中的客户端样例，怎样定义一个“远端节点”所需的最小字段集；
3. 当前项目的手工节点信息处理，与上述两种形态有哪些结构差异；
4. 哪些模式可以进入项目节点编辑器后续设计，哪些只能作为对照而不直接复制。

### 1.1 三个对象的边界

| 对象 | 所属系统 | 作用 | 本项目对应物 |
|---|---|---|---|
| 3x-ui Inbound | 3x-ui /Xray 服务端 | 服务端监听、账号、证书、Reality 私钥等 | 不是手工节点本身；可作为服务端字段边界参考 |
| 3x-ui Outbound | 3x-ui / Xray 客户端侧模板 | 远端代理连接的 Xray outbound 描述 | 与项目后续“独立 Xray outbound 输出”最接近 |
| Xray-examples client.jsonc | Xray 配置样例 | 展示 Xray 客户端实际可用的远端节点结构 | 是项目 Xray target adapter 的字段语义来源 |
| 项目 manual Node | VPN-Subscription-Management | 手工维护、供 Clash/URI 等输出使用的节点 | 是本次编辑器改造的主体 |

### 1.2 关键结论摘要

- 【结论】3x-ui 的条件表单、集中化能力判断、按协议/传输/安全拆分子表单、表单到 wire JSON 的适配层，均可以作为“当前组合驱动”编辑器的可参考实现。
- 【结论】3x-ui 的 Inbound 和 Outbound 在表单结构上高度同构：`protocol → settings → streamSettings.network → per-network settings → security → tls/realitySettings → sockopt/mux/advanced`。
- 【结论】3x-ui 保存的是「当前唯一活动配置」，不保存非激活传输/安全分支；切换 network 会清理旧 network 子对象并写入 schema 默认值，切换 security 会删除旧 security 子对象。这与 Design4 已确认的路线 B（活动配置 + 独立版本化编辑状态、保留非激活分支）不同。
- 【结论】Xray 客户端样例把“远端连接”稳定表达为：协议、地址/端口、协议认证（settings）、传输（network + 对应子对象）、外层安全（security + tls/realitySettings）。项目当前 `protocol_json` 是 Mihomo/Clash 风格扁平字段，不是该嵌套结构。
- 【结论】3x-ui 的 OutboundFormModal 同时提供“表单 + 完整 JSON 编辑 + Link 导入”，是一个与项目未来 Xray outbound 输出/编辑非常接近的交互先例。已确认：本轮将其记录为后续候选，不改变 Design4 当前的“局部 JSON 草稿 + 完整目标 JSON 只读检查”边界。
- 【结论】项目当前节点表已经具备可复用的递归编辑器、后端 schema、凭据加密和多个输出器；主要差距是缺少“当前组合条件元数据”、活动/非激活状态区分和按目标投影的能力检查。

---

## 二、3x-ui 工作方式分析

### 2.1 3x-ui 的“节点”与“入站/出站”

3x-ui 是 Xray 服务端面板。它包含：

- 管理 Xray `inbounds` 的入站表单，编辑的是服务端监听与用户账号；
- 管理 Xray 模板配置中的 `outbounds` 出站表单，编辑的是面板/服务端使用的上游代理，也接近“客户端节点”的 Xray 表示；
- 单独的 Node 表单，用于管理远端 3x-ui 面板节点（API 地址、TLS、同步策略），与“代理节点字段”不是同一个概念。

【3x-ui 事实】`frontend/src/pages/nodes/NodeFormModal.tsx` 是面板子节点管理，字段为 name/address/basePath/apiToken 等；`frontend/src/pages/inbounds/form/InboundFormModal.tsx` 是 Xray 入站编辑；`frontend/src/pages/xray/outbounds/OutboundFormModal.tsx` 是 Xray 出站编辑。本项目“节点编辑器”更接近后两者的组合视角，而不是面板子节点表单。

### 2.2 InboundFormModal：按 Tabs 组织的条件表单

【3x-ui 事实】`InboundFormModal.tsx` 使用 React Hook Form + `useWatch` 监听 `protocol`、`streamSettings.network`、`streamSettings.security`、`settings.method` 等字段，并由这些值决定显示哪个子组件。

结构如下：

```text
Basic
  enable / remark / deployTo / protocol / address / share strategy / port / traffic reset / expire
Protocol
  protocol-specific fields（VlessFields、ShadowsocksFields、WireguardFields…）
  fallbacks（VLESS/Trojan TCP + TLS/Reality 时）
Stream
  network selector（tcp/kcp/ws/grpc/httpupgrade/xhttp）
  per-network forms（RawForm、WsForm、GrpcForm…）
  Sockopt
  FinalMask
Security
  Radio：none / TLS / Reality（受 canEnableTls、canEnableReality 控制）
  TlsForm 或 RealityForm
Sniffing
  sniffing fields
Advanced
  多级 JSON 编辑器：全部、settings、stream、sniffing
```

对应源码位置：

- Tab 定义与 `forceRender`：`InboundFormModal.tsx:1105-1173`
- 条件 watch：`InboundFormModal.tsx:251-268`
- 传输选择与子表单：`InboundFormModal.tsx:870-935`
- 安全选择与子表单：`InboundFormModal.tsx:941-985`
- 高级 JSON：`InboundFormModal.tsx:987-1069`

### 2.3 能力判断集中为纯函数

【3x-ui 事实】`frontend/src/lib/xray/protocol-capabilities.ts` 提供：

- `canEnableTls({ protocol, streamSettings: { network, security } })`
- `canEnableReality({ protocol, streamSettings: { network, security } })`
- `canEnableTlsFlow({ protocol, streamSettings, settings })`
- `canEnableStream({ protocol })`
- `canEnableSniffing({ protocol })`
- `isSS2022({ protocol, settings })`

这些函数仅接收最小切片，不依赖完整表单对象，因此能同时用于 Inbound 表单、Outbound 表单和测试。常量表也集中维护（如 TLS 可用协议/网络、Reality 可用协议/网络）。【结论】这是「同一套能力判断多处复用、避免前后端/入站出站漂移」的可参考做法，与 Design4 §6.2 希望前端、Go 后端、适配器共享规则数据的方向一致；3x-ui 用 TypeScript 纯函数实现，本项目可考虑用后端下发共享规则数据，但仍可借鉴“能力判断单一来源”的职责划分。

### 2.4 切换 network / security / protocol 时的数据行为

【3x-ui 事实】Inbound `onNetworkChange` 明确：

```ts
const cleaned = { ...current, network: next };
for (const k of ALL) {
  if (k !== `${next}Settings`) delete cleaned[k];
}
cleaned[`${next}Settings`] = newStreamSlice(next);
```

即切换到新 network 时，删除其它 network 子对象，并写入新 network 的 schema 默认值；不保留旧 network 参数。`InboundFormModal.tsx:825-868`。安全切换由 `useSecurityActions` 处理，通常也会删除另一个安全子对象并写入目标分支的对象。Outbound 的 `onSecurityChange` 同样删除 `tlsSettings`/`realitySettings` 再写入新分支默认值，`OutboundFormModal.tsx:207-233`；`applyNetworkChange` 会尽量保留当前 security 及 security 子对象，但不会保留被替换的 network 子对象，`outbound-form-helpers.ts`。

【结论】3x-ui 的“切换即清空旧分支”与 Design4 已确认的“保留非激活分支、只输出当前激活配置”是两种不同取舍。3x-ui 因为保存的是最终 Xray wire 配置，不需要在面板内恢复被替换分支；本项目面向多目标输出且用户可能往返切换，保留非激活分支与 Design4 已确认方向一致。本研究只记录该差异，不改变 Design4 方向。

### 2.5 表单、存储与 wire payload 的适配层

【3x-ui 事实】`inbound-form-adapter.ts` 负责：

- `rawInboundToFormValues()`：把数据库行中的 JSON 字符串解析为表单值，修复旧字段缺失、`method`→`network` 别名、TLS 证书 `useFile` 派生、XHTTP 旧 key 迁移等；
- `formValuesToWirePayload()`：把表单值 `pruneEmpty`、按协议白名单归一化 clients、调用 `normalizeStreamSettingsForWire`、删除仅用于 UI 的标志、序列化 JSON 字符串；
- `normalizeStreamSettingsForWire`：按 side（inbound/outbound）、mode、xmux 互斥、sockopt 空值等规则清理 wire 输出。

对应源码：`inbound-form-adapter.ts:169-222,348-380`；`stream-wire-normalize.ts:220-300` 附近。

【结论】即使采用声明式 schema，仍需要一个“表单值 -> 目标 wire 值”的显式归一化/投影层。3x-ui 的这个层集中在适配器，而不是散落在表单组件。本项目后续 Xray 适配器、Clash 适配器、URI 适配器应沿用同样职责：编辑器持有语义状态，适配器负责目标格式的字段映射、别名、默认值剔除与能力差异。

### 2.6 Advanced JSON 与错误定位

【3x-ui 事实】

- Inbound 高级 Tab 内部再分“全部 / settings / stream / sniffing”，分别编辑对应 JSON 切片，`InboundFormModal.tsx:987-1069`。
- Outbound 使用两个主 Tab：表单和 JSON；进入 JSON 时快照当前 wire payload，回到表单时 JSON 可回灌表单，`OutboundFormModal.tsx:260-320`。
- 校验错误会自动切换到出错 Tab 并提示首个错误，`InboundFormModal.tsx:650-665` 附近（submit 处理）。
- `rawOutboundToFormValues` 会把 JSON 解析回表单值，因此 JSON 与表单共享同一套 schema/适配器。

【结论】这是本项目“结构化编辑 + JSON”的重要对照。3x-ui 的完整 JSON 编辑是可写且可回灌的；Design4 当前计划是“局部 JSON 草稿 + 完整目标 JSON 只读检查”。经本轮确认：3x-ui 的完整 JSON 可编辑模式先记录为后续候选，不立即改写 Design4。后续若采用，需要明确：完整 JSON 的对象归属、与表单之间谁是权威、非激活分支如何表达、敏感字段如何避免明文回显。

### 2.7 OutboundFormModal：与未来 Xray outbound 最接近的先例

【3x-ui 事实】`OutboundFormModal.tsx` 已支持：

- 协议选择：`vmess`、`vless`、`trojan`、`shadowsocks`、`socks`、`http`、`wireguard`、`hysteria`、以及 freedom/blackhole/dns/loopback 等非代理出站；
- 共享 `ServerTarget`（address+port）或各协议专有 settings；
- 与 Inbound 相同的 `streamSettings.network` 选择、per-network 子表单、security Radio、TLS/Reality 子表单；
- VLESS flow + Vision testpre/testseed 条件显示；
- Mux、Sockopt、FinalMask；
- 完整 JSON Tag 页 + Link 导入（`parseOutboundLink`）；
- tag 唯一性、保留前缀 `_bl_` 等校验。

对应组件位置：`frontend/src/pages/xray/outbounds/`、`frontend/src/schemas/forms/outbound-form.ts`、`frontend/src/lib/xray/outbound-form-adapter.ts`、`frontend/src/lib/xray/outbound-link-parser.ts`。

【结论】3x-ui 的 OutboundFormModal 是“在一个面板中维护 Xray 出站对象”的完整实现，而非仅服务端入站。本项目后续“独立 Xray target adapter / 固定验证 profile”可以参照它：先固定协议/传输/安全字段，再做 link 导入和 JSON 检查，但不需要照搬其全量 Xray 配置管理（DNS、routing、balancer、subscription 等）。

---

## 三、Xray-examples 客户端样例与字段语义

### 3.1 客户端 outbound 的最小分层

【样例事实】`Xray-examples` 中所有客户端文件都有一条或多条 `outbounds[]`，远端节点字段可归纳为：

```text
outbounds[]
└─ protocol
└─ settings
   ├─ vmess/vless: address, port, users[].id/security, encryption, flow
   ├─ trojan/shadowsocks: servers[].address/port/password/method
   └─ hysteria: address, port, version
└─ streamSettings
   ├─ network
   ├─ per-network settings（tcpSettings/wsSettings/grpcSettings/xhttpSettings/…）
   ├─ security
   └─ tlsSettings / realitySettings
└─ mux（可选）
```

典型文件：

- `VLESS-TCP-XTLS-Vision-REALITY/config_client.jsonc`：VLESS + TCP + Reality + Vision；
- `VLESS-TCP-TLS-WS/config_client_ws_tls.jsonc`：VLESS + WS + TLS；
- `VLESS-gRPC-REALITY/config_client.jsonc`：VLESS + gRPC + Reality + mux；
- `VMess-Websocket-TLS/config_client.jsonc`：VMess + WS + TLS；
- `Trojan-gRPC-Caddy2／Nginx/client.jsonc`：Trojan + gRPC + TLS；
- `Shadowsocks-TCP/client.jsonc`：SS + TCP（无外层安全）；
- `Hysteria2/client.jsonc`：Hysteria2 专用 `network=hysteria` + TLS + `hysteriaSettings`；
- `VLESS-XHTTP3-Nginx/client.jsonc`：XHTTP + TLS + xmux。

### 3.2 协议与传输、安全是正交层

【样例事实】在 Xray 客户端中：

- 协议设置只描述“这台远端代理的认证方式”：VLESS 的 `id/encryption/flow`、VMess 的 `id/security`、Trojan 的 `password`、SS 的 `method/password`、Hysteria 的 `auth/version`。
- `streamSettings.network` 描述连接传输：`tcp`、`ws`、`grpc`、`httpupgrade`、`xhttp`、`kcp`、`hysteria`。
- `streamSettings.security` 描述外层安全：`none`、`tls`、`reality`。
- 不同传输的专用对象均放在 `streamSettings.<network>Settings`；不同安全方式的对象放在 `streamSettings.<security>Settings`。

【结论】这正好支持 Design4 的“当前组合驱动”表单：协议确定认证、传输确定传输参数、安全确定身份参数，三者正交但不任意组合。项目注册表当前用扁平字段表达这些层，需要设计显式的“当前组合状态”而非仅靠字段存在性。

### 3.3 客户端字段与服务端字段的边界

【样例事实/既有研究】Xray 客户端样例中 Reality 只包含：

```json
"realitySettings": {
  "serverName": "",
  "publicKey": "",
  "shortId": "",
  "spiderX": "",
  "fingerprint": "chrome"
}
```

服务端 Reality 才会出现 `privateKey`、`target/dest`、`serverNames`、`shortIds`、`minClientVer` 等。TLS 客户端字段主要是 `serverName`、`alpn`、`fingerprint`、`verifyPeerCertByName`、`pinnedPeerCertSha256` 等；证书文件、私钥和监听配置属于服务端/入站侧。

【结论】与 [xray-server-side.md](xray-server-side.md) 的结论一致：客户端节点编辑器不得把服务端证书、私钥、Reality target/privateKey、fallback、sniffing 等当成普通节点字段。3x-ui Inbound 表单包含这些服务端字段，但它是入站编辑器；本项目应参考其客户端 Outbound 表单，而不是把 Inbound 字段全盘搬来。

### 3.4 样例对项目字段映射的启示

| Xray 客户端字段 | 项目当前近似字段 | 映射注意 |
|---|---|---|
| `settings.address/port` | `host` / `port`（Node 顶层） | 已经是公共字段 |
| VLESS `id` / VMess `id` | `uuid` | 语义一致 |
| VMess `security`（内部加密） | `cipher` | 不能与外层 TLS security 合并 |
| VLESS `encryption` | `encryption` | 当前 generic 输出固定 `none`，需高级/只读表达 |
| VLESS `flow` | `flow` | 需按 network+security 条件展示 |
| `streamSettings.network` | `network` | 当前为 select 字段，无条件组合校验 |
| `wsSettings.path/host/headers` | `ws-opts.path` / `ws-opts.headers` | 项目已有规范路径；需统一旧 `ws-path/ws-headers` |
| `grpcSettings.serviceName` | `grpc-opts.grpc-service-name` | 按目标决定提示或必填 |
| `xhttpSettings.mode` | `xhttp-opts.mode` | 项目当前默认 `none`，与 Xray 枚举不一致（Design4 已记录） |
| `tlsSettings.serverName` | `sni` / `servername` | 双字段别名需统一 |
| `tlsSettings.alpn` | `alpn` | 文本列表，注意数组/逗号 |
| `realitySettings.serverName` | `servername` / `sni` | 与 TLS SNI 同义但归属不同分支 |
| `realitySettings.publicKey/shortId/spiderX/fingerprint` | `reality-opts.public-key/short-id` | `spiderX` 目前项目未建模，需后续补齐或诊断 |
| `streamSettings.sockopt` | 顶层 `tfo/mptcp/interface-name/routing-mark` 等 | 需建立 Xray sockopt -> 项目字段映射，不能混用 |
| `mux` | `smux` / `multiplexing` | Mihomo SMux 与 Xray Mux 是不同语义，不能直接同名映射 |

### 3.5 与 Clash Verge Rev / Mihomo 已有研究互证

【项目事实】既有 Reference 文档 [Clash-Verge-Rev-Node-Parameters.md](Clash-Verge-Rev-Node-Parameters.md) 已按 Mihomo/Clash 客户端侧字段定义整理 VLESS、VMess、Trojan、SS 等协议的字段，包括 `reality-opts`、`grpc-opts`、`ws-opts`、`plugin-opts`、`client-fingerprint`、`smux` 等；[Clash-Verge-Rev-Subscription-Assembly.md](Clash-Verge-Rev-Subscription-Assembly.md) 则记录了订阅装配与覆盖层思路。

与 Xray-examples 对照后可以确认：

- Xray 使用 `realitySettings.publicKey/shortId/spiderX`，Mihomo 使用 `reality-opts.public-key/short-id`；两者是同一连接语义在不同客户端模型下的字段名差异，不是两个独立功能。
- Xray 的 `streamSettings.<network>Settings` 是嵌套对象，Mihomo 的 `grpc-opts` / `ws-opts` 是项目存储中的嵌套对象；字段路径不同但分层思想一致。
- Clash Verge Rev / Mihomo 资料提供的是“项目当前存储与 Clash 输出”的字段口径；Xray-examples 提供的是“Xray 客户端字段”口径。二者必须分开建模，不能让 UI 或适配器把 Xray 字段名当作 Mihomo 字段名直接透传。
- Design4 §8 已记录的 41 个 Mihomo 内核配置检查和 9 个 URI 样例检查，进一步证明同一字段在不同入口（YAML / URI / 解析器）下可能被接受、改写或拒绝；3x-ui 的 Xray 单一 wire 路径不能替代这些多入口验证。

---

## 四、当前项目节点信息处理现状

### 4.1 数据模型

【项目事实】`Node` 行包含 `source/manual|xray`、`name`、`protocol`、`host`、`port`、`protocol_json map[string]any`、`render_name`、`enabled`、`is_public` 等。manual 节点把全部协议参数存于 `protocol_json`，按后端 `FieldSchema` 结构保存。

### 4.2 注册表与表单

【项目事实】`registry.go` 的 `FieldSchema` 仅表达：类型、是否必填、默认值、选项、分区（static `section`）、对象类型、属性、是否允许未知键。`fieldSection()` 是静态函数：`bool` 一律归 `switches`，部分字段名归 `auth/security/transport`，其余归 `advanced`。

【项目事实】`NodesView.vue` 的 `sectionFields()` 按静态 section 过滤，然后顺序渲染六个区；`updateProtocol()` 在协议切换时直接清空 `protocol_json`。`ProtocolFieldEditor.vue` 支持对象/Map/List、对象级 JSON 和未知键提示，但没有“当前组合”条件下的显隐。

### 4.3 校验、加密与合并

【项目事实】

- 创建和更新都调用 `validateProtocolFields()`：只检查 schema 声明的 `Required` 和基本类型；`select` 不校验枚举成员，也不校验 network/security 组合。
- 更新时 `mergeSensitive()` 仅保留新协议 schema 声明的顶层字段，并沿用旧协议同名敏感字段（空值留空保留）。
- 加密、脱敏基于 `SensitiveFieldsOf(protocol)` 的点路径列表；当前 GetPath/SetPath 只遍历 map，数组内密钥路径覆盖不完整（已由 Improvement-Directions 记录）。

【结论】当前项目是“静态字段全集 + 静态必填 + 输出时全量透传”，3x-ui 是“当前活动分支 + 目标 wire 归一化”。要达成 Design4，需要把条件显隐、条件必填、目标能力映射纳入同一套规则，而不是在现有静态 schema 上追加更多普通字段。

### 4.4 输出适配现状

【项目事实】`render_clash.go` 的 `clashProxy()` 会把 `protocol_json` 顶层字段全部写入 Clash proxy 对象（只跳过 name/type/server/port），并仅对 text-list/int-list 做逗号拆分。因此 UI 隐藏不能防止非激活参数进入 Clash 产物。

【项目事实】`links.go` 的 `srLink`/`genericLink` 针对不同协议写不同的 URI 分支。VLESS reality 判断以 `reality-opts` 是否为对象为准；Trojan 的 SR/generic 分支没有输出 WS/gRPC 参数；SS plugin 使用 `pluginString()` 简单拼接，未按 SIP003/目标客户端逐字段映射。

【结论】这印证 Design4 的“活动投影 + 适配器投影”必要：当前输出器直接消费 `protocol_json`，没有“当前激活组合”这一中间层。3x-ui 的 Outbound 表单则相反：它直接把编辑结果转成 Xray outbound wire，只有一种主目标；本项目必须服务多种目标，因此不能照搬其单一 wire 模型。

### 4.5 项目与 3x-ui 的主要差异表

| 维度 | 3x-ui | 当前项目 | Design4 方向 |
|---|---|---|---|
| 编辑对象 | Xray inbound/outbound JSON | manual 节点（多目标中间表示） | 多目标节点语义模型 |
| 主存储 | settings/streamSettings 等 wire JSON 字符串 | Mihomo 风格 `protocol_json` | 活动 `protocol_json` + 独立编辑状态 |
| 当前选择表达 | `streamSettings.network/security` 显式字段 | 依赖 `network`、`tls`、`reality-opts` 存在性 | 显式活动状态 |
| 非激活分支 | 切换时清除旧分支 | 旧字段可能在输出里残留 | 保留但隔离，不输出 |
| 条件规则 | TypeScript 纯函数 + 组件条件渲染 | 后端静态 section | 服务端下发声明式规则 |
| 校验 | Zod schema + RHF + 字段路径 | 后端静态必填/类型 | 前后端共享条件校验 |
| JSON | 可编辑完整 JSON、可回灌 | 对象级 JSON、未知键保留 | 局部 JSON 草稿 + 完整目标只读 |
| 目标输出 | Xray 单一格式 | Clash YAML、SR/generic URI | 多目标投影和诊断 |
| 凭据 | 3x-ui 面板自身处理 | 后端加密 + 留空保留 | 保留/替换/清除三分 |
| 历史版本 | 实时模板配置 | 版本快照 + 动态 Xray 注入 | 历史快照边界保留 |

---

## 五、对 Design4 的整合建议

### 5.1 可以借鉴的模式

1. **按当前组合拆分子表单**：3x-ui 的 protocol/transport/security 三个维度分别有组件。项目可以继续由后端 schema 驱动，但应把“协议核心认证、当前传输参数、当前安全参数”作为三个条件区块。
2. **集中能力判断**：与 Design4 §6.2 一致。项目可以定义 `canEnableTls`、`canEnableReality`、`canEnableFlow` 这类规则数据而非散落组件；3x-ui 证明纯函数形式能同时服务多个入口。
3. **服务端投影与 wire 清理层**：3x-ui 的 `formValuesToWirePayload + normalizeStreamSettingsForWire` 说明“用户看到的值”和“最终目标值”不能等同。项目应在活动 `protocol_json` 后增加目标投影层，而不是让 UI 直接控制输出。
4. **错误自动定位与分区跳转**：3x-ui 在提交失败时跳到出错 Tab 并给出首个字段路径。Design4 已计划展开所属区域并定位首个错误，3x-ui 是该交互的可参考实现证据。
5. **Link 导入 + 表单 + JSON 三入口并存**：3x-ui OutboundFormModal 允许从 `vmess://`、`vless://` 等链接导入并直接生成表单/JSON。项目 URI 导入已有逐行结果，未来 Xray outbound 输出若带编辑功能，可采用类似“导入后进入表单/JSON”的流程。
6. **旧数据兼容的显式归一化**：3x-ui 在 adapter 中处理旧 `method`→`network`、XHTTP 旧 key、Reality `dest`→`target` 等别名。项目已有的 `ws-opts`/`ws-path` 冲突、Trojan 内层 SS 旧字段等也需要类似显式归一化，不能在 UI 中无提示覆盖。

### 5.2 不应照搬或需要保留边界的内容

1. **3x-ui 切换即清空旧分支**：与 Design4 保留非激活分支冲突；不采纳为项目行为。
2. **Inbound 服务端字段**：证书、私钥、Reality 私钥/target、fallback、sniffing 等不能进入普通节点编辑器。
3. **单一 Xray wire 作为节点真值**：项目仍需要多目标输出；不能把 Xray outbound 当作唯一持久化模型。
4. **Tabs 布局**：3x-ui 用 Tabs 分隔 Basic/Protocol/Stream/Security/Advanced。Design4 已确认沿用现有浮层 + 分区展开/折叠；Tabs 仅作参考，不强制改变。
5. **完整 JSON 可编辑**：已按用户确认记录为“后续候选”，当前不改变 Design4 的局部草稿/只读全文边界。后续若采纳，需补充分区权威、敏感字段和非激活分支表达。
6. **3x-ui 的“面板节点”概念**：其 NodeFormModal 是管理远端 3x-ui 面板连接，不是代理节点字段编辑；不与项目 manual 节点混淆。

### 5.3 本轮确认的候选事项

| 候选 | 当前状态 | 建议后续处理 |
|---|---|---|
| 完整 Xray outbound JSON 可编辑 Tab | 已记录为后续候选 | 若未来 Xray outbound 输出升级为可编辑，再决定与局部 JSON/表单的权威关系 |
| 能力规则采用纯函数还是声明式数据 | 均为候选 | 可继续以后端下发 schema 为主线，也可以借鉴 3x-ui 纯函数分层；需在 Design 阶段定契约 |
| 是否在编辑器内直接编辑 Xray outbound 产物 | Design4 当前为只读检查 | 3x-ui 提供可编辑先例，但会影响“项目节点与目标产物”边界，需后续决策 |
| SS 插件/mode 的逐目标映射 | 仍为待细化 | 3x-ui Outbound 不覆盖 SIP003 插件，因此不能以它为 SS 插件映射证据 |

---

## 六、待决策/待细化问题

以下问题在本轮研究中出现，但不属于本次 Reference 的定稿内容；后续进入 Design/Build 前需按 AGENTS.md 与用户确认。

1. **Xray outbound 的“编辑”边界**：是只做固定 profile 的只读/差异检查，还是提供类似 3x-ui OutboundFormModal 的可编辑表单 + JSON？
2. **完整 JSON 的可写程度**：如果后续采用 3x-ui 的 JSON 回灌模式，如何与局部 JSON 草稿、活动/非激活状态、敏感字段保护共存？
3. **能力规则载体**：继续扩展 Go `FieldSchema` 下发声明式条件，还是引入类似 3x-ui 的集中式能力函数并由后端同步/共享？这影响前后端是否保持单一规则来源。
4. **Xray 客户端固定验证版本**：需要确定用于 Xray outbound 检查的 core/客户端版本，以及用 Xray-examples 中哪些样例作为正反例。
5. **多目标输出的状态投影**：活动 `protocol_json` + 独立编辑状态保存后，Clash、SR/generic、Xray outbound 各自需要哪些字段，仍按 Design4 §10.2 继续细化。

---

## 七、证据索引

### 7.1 3x-ui 本地证据

| 证据 | 位置 |
|---|---|
| Inbound 表单 Tab/条件/高级 JSON | `3x-ui/frontend/src/pages/inbounds/form/InboundFormModal.tsx` |
| 能力判断纯函数 | `3x-ui/frontend/src/lib/xray/protocol-capabilities.ts` |
| Inbound 表单/存储适配 | `3x-ui/frontend/src/lib/xray/inbound-form-adapter.ts` |
| Stream wire 归一化 | `3x-ui/frontend/src/lib/xray/stream-wire-normalize.ts` |
| Inbound/Form 类型 schema | `3x-ui/frontend/src/schemas/forms/inbound-form.ts`、`3x-ui/frontend/src/schemas/protocols/...` |
| Outbound 表单/JSON/Link 导入 | `3x-ui/frontend/src/pages/xray/outbounds/OutboundFormModal.tsx` |
| Outbound 表单适配与 link parser | `3x-ui/frontend/src/lib/xray/outbound-form-adapter.ts`、`outbound-link-parser.ts` |
| Outbound schema | `3x-ui/frontend/src/schemas/forms/outbound-form.ts`、`3x-ui/frontend/src/schemas/protocols/outbound/...` |
| Protocol/security/stream schema | `3x-ui/frontend/src/schemas/protocols/security/...`、`3x-ui/frontend/src/schemas/protocols/stream/...` |

### 7.2 Xray-examples 客户端样例

| 证据 | 位置 |
|---|---|
| VLESS TCP + Reality + Vision | `Xray-examples/VLESS-TCP-XTLS-Vision-REALITY/config_client.jsonc` |
| VLESS WS + TLS | `Xray-examples/VLESS-TCP-TLS-WS/config_client_ws_tls.jsonc` |
| VLESS gRPC + Reality | `Xray-examples/VLESS-gRPC-REALITY/config_client.jsonc` |
| VLESS XHTTP + Reality | `Xray-examples/VLESS-XHTTP-Reality/minimal-steal_others/client.jsonc` |
| VLESS XHTTP3 + TLS | `Xray-examples/VLESS-XHTTP3-Nginx/client.jsonc` |
| VMess WS + TLS | `Xray-examples/VMess-Websocket-TLS/config_client.jsonc` |
| Trojan gRPC + TLS | `Xray-examples/Trojan-gRPC-Caddy2／Nginx/client.jsonc` |
| Shadowsocks TCP | `Xray-examples/Shadowsocks-TCP/client.jsonc` |
| Hysteria2 | `Xray-examples/Hysteria2/client.jsonc` |

### 7.3 项目本地证据

| 证据 | 位置 |
|---|---|
| 协议注册表/静态 schema | `VPN-Subscription-Management/backend/internal/node/registry.go` |
| 节点服务/加密/校验/合并 | `VPN-Subscription-Management/backend/internal/node/node.go` |
| Clash 渲染 | `VPN-Subscription-Management/backend/internal/assembly/render_clash.go` |
| 链接输出 | `VPN-Subscription-Management/backend/internal/assembly/links/links.go` |
| 节点表单页面 | `VPN-Subscription-Management/frontend/src/views/admin/NodesView.vue` |
| 递归字段编辑器 | `VPN-Subscription-Management/frontend/src/components/ProtocolFieldEditor.vue` |
| 既有研究 | `Node-Editor-Design-Research.md`、`Node-Editor-Improvement-Directions.md`、`Design4.md` |

### 7.4 外部资料

- [3x-ui InboundFormModal / outbound 相关公开页面与 Issue](https://github.com/MHSanaei/3x-ui)
- [Xray REALITY 文档](https://xtls.github.io/en/config/transports/reality.html)
- [Xray WebSocket 文档](https://xtls.github.io/en/config/transports/websocket.html)
- [Xray gRPC 文档](https://xtls.github.io/en/config/transports/grpc.html)
- [Mihomo SS 文档](https://wiki.metacubex.one/en/config/proxies/ss/)
- [Mihomo VLESS 文档](https://wiki.metacubex.one/en/config/proxies/vless/)
- [Mihomo VMess 文档](https://wiki.metacubex.one/en/config/proxies/vmess/)
- [Mihomo TLS 文档](https://wiki.metacubex.one/en/config/proxies/tls/)

> 外部资料仅用于补充语义；本地源码取证优先。引用外部链接不表示对对方项目进行改动或背书。

---

## 八、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-02 | 新建独立 Reference 文档：分析 3x-ui Inbound/Outbound 表单架构、能力判断、wire 适配和 JSON 模式；对照 Xray-examples 客户端样例字段；梳理当前项目节点处理与差异；将 3x-ui 完整 JSON 可编辑模式记录为后续候选。仅文档，未改动项目代码或外部项目。 |
