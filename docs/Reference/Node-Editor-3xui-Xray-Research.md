# Node-Editor-3xui-Xray-Research.md — 3x-ui / Xray 客户端样例对照与 Build17-21 后深度研究

> **文档定位：** 本文是节点编辑器改进研究的 3x-ui/Xray 专项资料，合并两阶段研究：第一轮（2026-09-02）分析 3x-ui Inbound/Outbound 表单结构、Xray-examples 客户端字段与项目节点处理差异；第二轮（2026-09-05）在 Build17～Build21 落地后，继续从 3x-ui v3.7.0 提炼可借鉴机制。本文承接 [Node-Editor-Research.md](Node-Editor-Research.md)、[Design4.md](../../Design4.md)、[Xray-Client-Config-Research.md](Xray-Client-Config-Research.md)、[Node-Link-Standards.md](Node-Link-Standards.md) 等既有证据。本文只做研究记录，不定义实现，不改动项目业务代码，也不代表对 3x-ui 或 Xray-examples 的修改或产品背书。
> **研究状态：** 2026-09-02 创建第一轮，2026-09-05 新增第二轮，2026-09-08 文档交叉审核后同步至 Design4 v1.14 / Build21 收口口径。基于本机仓库 `~/Desktop/Repo/3x-ui`（第二轮 HEAD `f727d04f`，v3.7.0）、`~/Desktop/Repo/Xray-examples`、`~/Desktop/Repo/clash-verge-rev` 和本项目当前代码静态分析；未构建、未联机调试、未改动外部项目与当前项目代码。
> **标注约定：** 【3x-ui 事实】= 本地 3x-ui 源码观察；【样例事实】= Xray-examples 文件内容；【项目事实】= 当前项目代码或既有文档；【外部事实】= 公开文档或 Issue；【结论】= 由证据推导、供后续研究采纳；【候选】= 需要后续决策，不视为已定稿。
> **文档历史：** 原 `Node-Editor-3xui-Xray-Research-2.md` 已并入本文作为“八、第二阶段”；本文中早期“保留非激活分支 / 独立编辑状态 / Design4 路线 B”表述均已按 Design4 v1.2+ 口径改写。

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
- 【结论】3x-ui 保存的是「当前唯一活动配置」，不保存非激活传输/安全分支；切换 network 会清理旧 network 子对象并写入 schema 默认值，切换 security 会删除旧 security 子对象。Design4 v1.2 同样采用切换即清空、不保存非激活分支，与 3x-ui 的这一行为方向一致；差异在于项目需要显式保存当前选择元数据供多目标输出解释。
- 【结论】Xray 客户端样例把“远端连接”稳定表达为：协议、地址/端口、协议认证（settings）、传输（network + 对应子对象）、外层安全（security + tls/realitySettings）。项目当前 `protocol_json` 是 Mihomo/Clash 风格扁平字段，不是该嵌套结构。
- 【结论】3x-ui 的 OutboundFormModal 同时提供“表单 + 完整 JSON 编辑 + Link 导入”，是一个与项目未来 Xray outbound 输出/编辑非常接近的交互先例。已确认：本轮将其记录为后续候选，不改变 Design4 当前的“局部 JSON 草稿 + 完整目标 JSON 只读检查”边界。
- 【结论】项目当前节点表已经具备可复用的递归编辑器、后端 schema、凭据加密和多个输出器；主要差距是缺少“当前组合条件元数据”、显式当前选择状态和按目标投影的能力检查。

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

【结论】3x-ui 的“切换即清空旧分支”与 Design4 v1.2 的“清空分支且不保留恢复副本”一致。3x-ui 保存最终 Xray wire 配置，不需要恢复被替换分支；本项目也采用相同的清空取舍，并另存最小当前选择元数据以便多目标输出正确解释当前激活组合。本研究保留该观察，不改变 Design4 方向。

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

【结论】这是本项目“结构化编辑 + JSON”的重要对照。3x-ui 的完整 JSON 编辑是可写且可回灌的；Design4 当前计划是“局部 JSON 草稿 + 完整目标 JSON 只读检查”。经本轮确认：3x-ui 的完整 JSON 可编辑模式先记录为后续候选，不立即改写 Design4。后续若采用，需要明确：完整 JSON 的对象归属、与表单之间谁是权威、被切离分支已清空后的草稿边界、敏感字段如何避免明文回显。

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

【结论】与 [Xray-Server-Config-Research.md](Xray-Server-Config-Research.md) 的结论一致：客户端节点编辑器不得把服务端证书、私钥、Reality target/privateKey、fallback、sniffing 等当成普通节点字段。3x-ui Inbound 表单包含这些服务端字段，但它是入站编辑器；本项目应参考其客户端 Outbound 表单，而不是把 Inbound 字段全盘搬来。

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
| 主存储 | settings/streamSettings 等 wire JSON 字符串 | Mihomo 风格 `protocol_json` | 活动 `protocol_json` + `nodes` 行内当前状态/扩展/修订 |
| 当前选择表达 | `streamSettings.network/security` 显式字段 | 依赖 `network`、`tls`、`reality-opts` 存在性 | 显式当前状态 |
| 非激活分支 | 切换时清除旧分支 | 旧字段可能在输出里残留 | v1.2：清空且不保存非激活分支 |
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

1. **3x-ui 切换即清空旧分支**：Design4 v1.2 采用相同取舍；本项目同时显式保存当前选择元数据，但不保存非激活分支恢复副本。
2. **Inbound 服务端字段**：证书、私钥、Reality 私钥/target、fallback、sniffing 等不能进入普通节点编辑器。
3. **单一 Xray wire 作为节点真值**：项目仍需要多目标输出；不能把 Xray outbound 当作唯一持久化模型。
4. **Tabs 布局**：3x-ui 用 Tabs 分隔 Basic/Protocol/Stream/Security/Advanced。Design4 已确认沿用现有浮层 + 分区展开/折叠；Tabs 仅作参考，不强制改变。
5. **完整 JSON 可编辑**：已按用户确认记录为“后续候选”，当前不改变 Design4 的局部草稿/只读全文边界。后续若采纳，需补充分区权威、敏感字段和被切离分支清空后的边界。
6. **3x-ui 的“面板节点”概念**：其 NodeFormModal 是管理远端 3x-ui 面板连接，不是代理节点字段编辑；不与项目 manual 节点混淆。

### 5.3 本轮确认的候选事项

| 候选 | 当前状态 | 建议后续处理 |
|---|---|---|
| 完整 Xray outbound JSON 可编辑 Tab | 已记录为后续候选 | 若未来 Xray outbound 输出升级为可编辑，再决定与局部 JSON/表单的权威关系 |
| 能力规则采用纯函数还是声明式数据 | 均为候选 | 可继续以后端下发 schema 为主线，也可以借鉴 3x-ui 纯函数分层；需在 Design 阶段定契约 |
| 是否在编辑器内直接编辑 Xray outbound 产物 | Design4 当前为只读检查 | 3x-ui 提供可编辑先例，但会影响“项目节点与目标产物”边界，需后续决策 |
| SS 插件/mode 的逐目标映射 | Design4 v1.3 已列出普通组合和已知风险 | 3x-ui Outbound 不覆盖 SIP003 插件，因此不能以它为 SS 插件映射证据；Build 仍需实现映射与诊断样例 |

---

## 六、待决策/待细化问题

以下问题在本轮研究中出现，但不属于本次 Reference 的定稿内容；后续进入 Design/Build 前需按 AGENTS.md 与用户确认。

1. **Xray outbound 的“编辑”边界**：后续专项；当前仅记录为只读/差异检查，是否提供可编辑表单 + JSON 仍需用户决策。
2. **完整 JSON 的可写程度**：如果后续采用 3x-ui 的 JSON 回灌模式，如何与局部 JSON 草稿、当前状态/清空边界、敏感字段保护共存。
3. **能力规则载体**：Design4 v1.3 已定为后端 `FieldSchema` 下发声明式条件；集中式能力函数仅作参考，不改变单一规则来源方向。
4. **Xray 客户端固定验证版本**：后续独立 Xray outbound 专项确定。
5. **多目标输出的状态投影**：Design4 v1.3 已按普通组合与已知风险列出首批矩阵；Build 仍需实现字段级映射与诊断样例。

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
| 既有研究 | `Node-Editor-Research.md`、`Design4.md` |

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

## 八、第二阶段：Build17-21 后的 3x-ui v3.7.0 深度研究（原 Node-Editor-3xui-Xray-Research-2.md）

### 一、研究目的与范围

#### 1.1 为什么在 Build17-21 之后继续研究 3x-ui

当前项目已完成（见 [Issue13.md](../reports/Issue/Issue13.md)、Build21）：

- `nodes` 行内当前状态/扩展/修订列（Build17）；
- `FieldSchema` 条件/选项/重置元数据、活动投影、保存校验与 `/check`（Build18）；
- 前端动态分区、可编辑下拉、局部 JSON、目标检查 UI（Build19）；
- 19 个 manual 协议统一保存契约、URI 导入归一化、Xray 来源适配、输出门槛（Build20）；
- R27-01～R27-08 与 R27-09 Step7～13（SS 插件统一合同、幂等归一化、固定敏感路径、SIP002 与 URI 目标分流、Clash/Mihomo 结构化插件投影、SS 插件专属目标诊断、未知插件前端编辑）已闭环，N-node-3/4 Step15 也已验收；**Step14 全链路回归与文档收口已完成**（见 [Build21.md](../reports/Build/Build21.md) 与 [Issue14.md](../../Issue14.md)），后续还有“其余 15 个协议完整条件表单、SS2022、独立 Xray outbound”等专项。

因此，本文不是“是否需要条件表单/当前状态”的研究，而是：

1. 从 3x-ui 找出当前项目尚未系统吸收的工程机制；
2. 判断这些机制能否帮助补齐 R27-09 剩余步骤与后续 15 协议/独立 Xray outbound；
3. 区分“可直接借鉴的机制”与“因多目标模型不同只能对照的机制”。

#### 1.2 研究边界

- 3x-ui 是“Xray 服务端面板 + 面向订阅用户的单面板输出”；
- 当前项目是“多目标订阅管理，节点是中间语义对象，输出到 Clash/URI/Xray 等多种目标”；
- 因此本文不把 3x-ui 当作产品蓝本，而当作“协议边界行为与工程模式”的证据源。

---

### 二、当前项目 Build17-21 已落地能力（项目事实）

| 能力 | 落地证据 | 说明 |
|---|---|---|
| `nodes` 当前状态/扩展/修订 | [1017_node_editor_state.sql](../../backend/migrations/1017_node_editor_state.sql)、[node.go](../../backend/internal/node/node.go) | `current_state_json`、`extensions_json`、`edit_revision`、`state_format_version` |
| 条件/选项/重置元数据 | [schema.go](../../backend/internal/node/schema.go)、[registry.go](../../backend/internal/node/registry.go) | `When`、`RequiredWhen`、`ResetOn`、`OptionItems`、`AllowCustom`、`TargetEvidence` |
| 活动投影/保存校验 | [project.go](../../backend/internal/node/project.go) | 按当前 `network/security/plugin/features` 递归投影活动字段 |
| 节点检查 | [check.go](../../backend/internal/node/check.go)、[node_check.go](../../backend/internal/assembly/node_check.go) | 不落库，复用实际适配器 |
| 前端动态表单 | [NodesView.vue](../../frontend/src/views/admin/NodesView.vue)、[ProtocolFieldEditor.vue](../../frontend/src/components/ProtocolFieldEditor.vue) | 分区/条件/递归/JSON/凭据状态 |
| URI 导入统一归一化 | [normalize.go](../../backend/internal/node/normalize.go)、[uri_import.go](../../backend/internal/node/uri_import.go)、[uriparse.go](../../backend/internal/uriparse/uriparse.go) | 统一当前状态，逐行回执 |
| SS 插件固定合同 | [ssplugin/contract.go](../../backend/internal/ssplugin/contract.go) | Clash/SR/generic 三目标合同、支持等级 |
| 收口状态 | [Build21.md](../reports/Build/Build21.md) §7、[Issue13.md](../reports/Issue/Issue13.md) R27-09 | R27-09 Step7～15 与 Step14 收口均已验收；其余 15 协议与独立 Xray outbound 仍后续 |

---

### 三、3x-ui v3.7.0 中未在前文完整覆盖的真实机制（3x-ui 事实）

#### 3.1 分支“默认对象工厂”与切换时对安全层的精细保留

3x-ui 不只做“切换传输时删除旧子对象”，而是为每个 network/security 分支准备“最小可用默认对象工厂”。

- `frontend/src/lib/xray/outbound-form-helpers.ts`：`newStreamSlice(network)`、`hysteriaStreamSlice()`、`applyNetworkChange()`。
- `frontend/src/pages/inbounds/form/InboundFormModal.tsx`：Inbound 侧使用 `TcpStreamSettingsSchema.parse(...)`、`XHttpStreamSettingsSchema.parse(...)` 等，把 Zod 默认值作为新分支种子；`onNetworkChange()` 删除其它 `xxxSettings` 后写入新 network 默认对象。
- `frontend/src/schemas/protocols/stream/xhttp.ts`：`XMUX_FRESH_DEFAULTS` 跟踪 xray-core 默认值变化；`OutboundFormModal.tsx` 的 `onXmuxToggle()` 只在空对象时写默认，避免覆盖用户已填内容。

【3x-ui 事实】它解决的是“切到新分支后表单不残留旧分支值，也不因缺少默认子对象而空指针/校验崩溃”；同时在跨传输仍支持的安全字段（如 TLS SNI）上避免直接删除导致用户配置丢失。

【经推理】当前项目已用 `reset_on` + `current_state` 实现“切换即清空”，但字段级默认值仍是散落的。后续覆盖其余 15 个协议时，可考虑为每个协议/传输/安全/插件分支提供“默认对象工厂”概念：它不是把默认值批量写库，而是用于 UI 新分支的初始展示、空值判断和复杂对象的合法骨架。

#### 3.2 表单 shape 与 wire shape 双层分离，adapter 是唯一转换层

3x-ui 的 Xray outbound 编辑不是直接编辑 wire JSON：

- `frontend/src/schemas/forms/outbound-form.ts`：表单层把 VMess/Trojan/SS/SOCKS/HTTP/WireGuard 压成单服务器、扁平、带 UI 标志的结构；
- `frontend/src/schemas/protocols/outbound/*.ts`：wire 层保持 Xray 需要的 `vnext[]/servers[]/peers[]` 结构；
- `frontend/src/lib/xray/outbound-form-adapter.ts`、`inbound-form-adapter.ts`、`stream-wire-normalize.ts`：负责 raw↔form 转换、旧字段迁移、UI-only 字段剥离、空值语义、XHTTP 互斥与 sockopt 默认值清理。

【3x-ui 事实】例如 WireGuard `pubKey` 是 UI 派生只读字段，adapter 不输出；XHTTP `enableXmux` 是 UI-only，`stripUiOnlyStreamFields()` 会删除；VMess 在 wire 中是 `vnext[].users[]`，表单层只是单个 address/id。

【经推理】当前项目把 `protocol_json` 作为内部中间对象，已经比“直接存 Xray wire”更接近表单层。但如果未来新增“独立 Xray outbound 目标”，不应在 `render_xray.go` 中临时拼 wire，而应建立一组“节点中间对象 ↔ Xray wire 对象”的显式 adapter，并让 Link 导入、JSON 编辑、保存前校验、正式输出共用该层。

#### 3.3 Link 导入统一走“parse → canonical wire → adapter 回填”，而非把 URL 参数直接塞控件

3x-ui 的 OutboundFormModal 从链接导入时：

- `frontend/src/pages/xray/outbounds/OutboundFormModal.tsx`：`parseOutboundLink()` 得到 wire raw，再 `rawOutboundToFormValues()` 后 `methods.reset(next)`，同时刷新 JSON tab；
- `frontend/src/lib/xray/outbound-link-parser.ts`：解析 vmess/vless/trojan/ss/hysteria2/wireguard；XHTTP 高级字段从 query 与 `extra` JSON 合并；Hysteria2 的 `obfs/mport` 会还原为 finalmask/QUIC hop。

【3x-ui 事实】这种“先到 wire，再进表单”的方式保证了导入、表单、JSON 看到的对象是同一个规范形态，而不是每类 URL 写一套表单填充逻辑。

【经推理】当前项目 `uriparse → NormalizeProtocolJSON → InitCurrentState` 已经具有“先归一化再进存储”的思想。后续若做独立 Xray outbound 导入/导出，可沿用“parse → canonical wire → internal semantic object → target renderer”的单一路径；但要注意 3x-ui 的 adapter 会丢弃未建模未知键，而当前项目对未知 SS 插件参数已选择“保留未知参数”，因此不能整体照搬其有损回填。

#### 3.4 后端 Go link parser 提供稳定 identity 与 tag 复用策略

3x-ui 后端除前端 parser 外，还有面向订阅刷新的 Go parser：

- `internal/util/link/outbound.go`：`ParseLink()` 支持 vmess/vless/trojan/ss/hysteria2/wireguard；每个 `ParseResult` 带 `Identity`（去掉 remark 的规范化身份）；
- `internal/web/service/outbound_subscription.go`：`assignStableTags()` 按“上次 identity→tag 映射”优先，其次按上一轮位置，最后新建；同批冲突追加 `-N`；
- `internal/sub/clash_yaml.go`、`clash_service.go`：Clash 名称唯一化与 YAML 歧义标量引号化。

【3x-ui 事实】该策略解决“同一远端服务器改名后，订阅刷新不把 tag/路由引用打断”的问题。

【经推理】当前项目 URI 导入仍按全局 `name` 去重；如果未来需要“远程订阅源反复刷新/同步”或“独立 Xray outbound 的 tag 长期稳定”，这套 identity + 位置回退策略很有参考价值。但 3x-ui 的 identity 明文包含凭据（UUID/password/私钥等），当前项目若采用，必须存加密或不可逆摘要，不能照搬明文 `LinkIdentities`。

#### 3.5 JSON 编辑有“三种模型”，不是只有一种

3x-ui 内部实际存在三种 JSON 编辑形态：

1. **Inbound Advanced：合法即回灌**
   - `frontend/src/pages/inbounds/form/advanced-editors.tsx`：`AdvancedSliceEditor` 编辑 `settings/streamSettings/sniffing` 等路径；本地 text buffer，每次输入 `JSON.parse` 合法就 `setValue(path, parsed)`，非法 JSON 只留在缓冲；`AdvancedAllEditor` 从当前已确认值派生出“将被保存”的完整 wire 预览，并按字段回写。
2. **Outbound JSON Tab：显式 dirty + 离开/保存时回灌**
   - `frontend/src/pages/xray/outbounds/OutboundFormModal.tsx`：进 JSON Tab 时从表单生成 wire JSON；出 Tab/保存时若 dirty，则 `JSON.parse` → `rawOutboundToFormValues` → `methods.reset(next)`。
3. **Host 子树：JSON 字符串用独立结构化子表单覆盖**
   - `frontend/src/pages/hosts/json-forms/OutboundSubtreeJsonForm.tsx`、`HostMuxForm.tsx`、`HostSockoptForm.tsx`、`HostFinalMaskForm.tsx`：把 `muxParams/sockoptParams/finalMask` 等 JSON 字符串解析后交给可复用的子表单编辑，序列化回字符串；空串表示“继承/不覆盖”。

【3x-ui 事实】它没有“统一 JSON 权威”结论：Inbound 是表单为权威、JSON 合法即同步；Outbound 是 JSON Tab 可临时为权威、离开时整对象经 adapter 重建；Host 是每个覆盖字符串为权威。

【经推理】当前项目 `ProtocolFieldEditor.vue` 的对象级“结构化 / 高级 JSON + Apply/放弃”属于另一种局部模型。未来若增加“完整目标 JSON 只读检查”或“路径级 JSON 切片”，可参考：
- 只读完整检查应从“当前已确认 model”派生，而不是直接读未应用草稿文本；
- 路径级 JSON 编辑可借鉴 Host 的“子表单 + 可插拔 serialize”模式，特别适合未知插件参数这类自由 map；
- 从 JSON 回灌到表单前要明确 adapter 是否“有损”，否则完整 JSON 编辑会丢弃当前项目要保留的未知参数。

#### 3.6 校验错误路径映射到 Tab/列表身份，而不是只报一个字段名

- `frontend/src/pages/inbounds/form/formatValidationError.ts`：把 `settings.clients.<index>.<field>` 转成客户端 `email` 提示；只报首条并附 `+N more`；
- `frontend/src/pages/inbounds/form/InboundFormModal.tsx`：`firstRhfValidationIssue()` 找首个叶子错误，`tabForValidationPath()` 映射到 Basic/Protocol/Stream/Security/Sniffing 等 Tab，保存失败自动切 Tab。

【3x-ui 事实】它解决“错误在隐藏 Tab/数组里，用户不知道去哪找”的问题。

【经推理】当前项目已有 `revealField()` 展开 `<details>` 并滚动聚焦。后续可以为后端错误 path 增加“按 group/section 定位”的统一映射，并对列表错误用稳定条目身份（如 WireGuard `_credential_id`、3x-ui 的 email）替代索引文案。

#### 3.7 数组/列表编辑的稳定 key、本地空行、类型默认工厂

3x-ui 对复杂数组使用多种补充机制：

- `useFieldArray` / `Form.List` 用稳定 `field.id` / `field.key` 作为 React key；
- `HeaderMapEditor.tsx` 本地 state 保留空行，只有填写有效 name 才进入 form，避免“新行一出现就被过滤”；
- `FinalMaskForm.tsx` 用 `defaultTcpMaskSettings(type)` / `defaultUdpMaskSettings(type)` 等“类型默认工厂”，并在挂载时迁移旧格式；
- 数组条目中条件子字段（如 `blockDelay` 仅在 action=block 时显示）在 `freedom.tsx` 等组件里实现。

【3x-ui 事实】这比“一行通用字段编辑所有对象”更适合 polymorphic 数组。

【经推理】当前项目 `ProtocolFieldEditor.vue` 已支持 `list` + `item_id_field`，但通用递归编辑器仍是 schema 驱动的平铺。若要支持未来复杂协议（XHTTP/FinalMask/AmneziaWG 混淆参数），可能需要“类型工厂 + 专用子编辑器 + 空行本地态”作为递归编辑器的补充形态。

#### 3.8 后端订阅/Clash/JSON 生成的“每格式裁剪 + 自检”

3x-ui 的 raw/Clash/JSON 订阅都从同一 inbound stream 出发，但每个格式用独立裁剪逻辑剔除服务端专用字段：

- `internal/sub/service.go` 生成 raw 链接；
- `internal/sub/clash_service.go` / `clash_yaml.go` 生成 Clash，处理名称唯一与 YAML 标量引号；
- `internal/sub/json_service.go` 生成 Xray JSON outbound，处理 SS2022 多用户、WireGuard、Hysteria2 等；
- `internal/sub/host_sub.go` / `endpoint.go`：把 Host 与 legacy externalProxy 统一为 endpoint 覆盖层，raw/JSON/Clash 共用。

【3x-ui 事实】例如 Hysteria2 的 `obfs=salamander` 不是散在协议设置里，而是来自 finalmask UDP salamander；`mport` 来自 QUIC hop。3x-ui 的 JSON 生成器对 SS2022 使用 `method:serverKey:clientKey` 的 SIP022 URI 形式，Clash 侧则把 server+client password 拼成 `password`。

【经推理】当前项目已经有自己的多目标生成器与诊断；3x-ui 主要可借鉴“一个覆盖层对象供多个 renderer 共用”的思想，以及“每个 renderer 各自处理 server-only 字段裁剪、空值语义”的做法。但因为 3x-ui 是面向 Inbound+Client 的订阅分发，当前项目不要照搬其 SubService 业务链。

#### 3.9 Write-only 凭据契约

3x-ui 对“面板节点”（远端 3x-ui API 凭据）采用明确的 write-only 契约：

- `internal/web/service/node_contract.go`：`NodeView` 只暴露 `HasApiToken`，不返回明文；更新用 `ApiToken *string`（省略/空白=保留），显式 `ClearApiToken` 才清除；
- `node_credentials_writeonly_test.go` 验证 API 不泄露 `apiToken` 字段。

【3x-ui 事实】这与当前项目的 `saved_sensitive_paths` / 留空保留 / credential_ops 思路一致；3x-ui 只覆盖单凭据字段，当前项目已覆盖嵌套与数组凭据，更细。

【经推理】若未来当前项目新增“远程订阅源/外部 API”等面板级节点管理，可直接把这种 `presence + pointer + explicit clear` 契约作为标准。

#### 3.10 真实 Xray core 校验与版本 gate

- `internal/xray/api.go`：`ValidateOutboundConfig()` 通过 vendored xray-core 做真实启动级校验；
- `internal/web/service/xray_setting.go`：保存前逐 outbound 校验，并用 `shouldSkipLegacyUnencryptedOutboundRejection()` 对“旧版本可接受、新版本拒绝”的 vless/trojan 做版本 gate；
- `internal/web/service/outbound_subscription.go`：远端订阅落库前丢弃会阻止整个 core 启动的 outbound。

【3x-ui 事实】这比仅靠 schema/正则校验更接近真实运行边界。

【经推理】当前项目若做“独立 Xray outbound 输出/验证”，可把固定版本 client/core 作为检查参数，并在可行时调用真实 core 校验或固定版本离线 build，而不是只靠启发式诊断。版本 gate 需要注意其错误文本匹配较脆弱，需要封装成可测试分类器。

#### 3.11 近期协议与高级字段（v3.7.0 新能力）

3x-ui v3.7.0 的实质新增与既有但覆盖不足的能力：

- **AmneziaWG native inbound**（commit `effcccce`）：独立于 Xray 的进程式协议，前端有大量混淆参数 schema、客户端 `.conf` 生成与随机参数 helper；
- **Hysteria2 标准 obfs/mport**：通过 finalmask UDP salamander + QUIC hop 承载，前端 link parser 与后端订阅生成都能反向还原；
- **XHTTP advanced**：`xPadding*`、sessionID table/length、seq、uplink data、xmux、downloadSettings 等；
- **sockopt 精细化**：`happyEyeballs`、`customSockopt`、`addressPortStrategy`、`tcpcongestion` 等；
- **finalmask**：TCP/UDP/QUIC 的分类型编辑器；
- **WireGuard**：UI 逗号字符串 ↔ wire 数组、peer 数组稳定 key、`secretKey` 派生 `pubKey`；
- **SS2022**：Xray 方法枚举与多用户/单用户边界，SIP022 URI 采用 percent-encode 而非 base64；
- **sub balancer / observatory**：JSON 订阅里把多个 proxy outbound 收进 `routing.balancers` + `observatory`。

【经推理】这些对当前项目后续协议建模有帮助，尤其 Hysteria2、WireGuard、SS2022。但 3x-ui 的“hysteria 协议实际是 Hysteria2”与当前项目同时存在 Hysteria1/Hysteria2 的命名不同；3x-ui 也没有 TUIC/AnyTLS/Snell/Mieru/MASQUE/OpenVPN/SSH/ShadowQUIC/TrustTunnel/Tailscale，不能作为这些协议 schema 的唯一来源。

---

### 四、对照 Build17-21 后：可借鉴方向（经推理 / 可能）

#### 4.1 R27-09 已收口步骤（Step11～Step13、Step15）

- 【项目事实】Build21 Step11～13 已完成并通过验收：Clash/Mihomo 结构化 SS 插件投影、SS 插件专属目标诊断与未知插件前端编辑均已落地；Step14 全链路回归/文档收口仍在 [Issue14.md](../../Issue14.md) 跟踪。
- 【3x-ui 事实】3x-ui 的 SS/Clash 输出并不覆盖 obfs/v2ray-plugin/shadow-tls/restls 这套插件体系，因此它**不能作为 SS 插件 Clash 结构化输出的字段证据**；其相关代码只在 raw 链接生成时把 TCP HTTP header 重编码成 `obfs-local`。
- 【经推理】SS 插件输出权威仍是当前项目 `ssplugin/contract.go` 与 Mihomo 1.19.29 固定版本源码/离线证据，而不是 3x-ui。3x-ui 可作为“不要把 URI 字符串插件形式用于 Clash 结构化输出”的反面佐证。
- 【经推理】未知插件参数前端编辑已落地，后续可继续借鉴 3x-ui `HeaderMapEditor`/Host 子树的“本地空行 + map 序列化 + 保留未知键”模式；当前项目 `ProtocolFieldEditor` 对 `map_value_type=string` 已有基本编辑，可补充“复杂值逐 key JSON 校验”与“空行不立刻写回”的交互。
- 【可能】后续目标诊断与正式装配一致性可参考 3x-ui “保存前/刷新前用真实生成器或 core validator 自检”的工程模式；但当前项目目标是 Clash/SR/generic，因此应继续复用 `selfcheck.go`/`diagnose.go`，并保持诊断结果与正式装配路径一致。

#### 4.2 其余 15 个 manual 协议完整条件表单

- 【经推理】当前项目已经有通用 FieldSchema 递归编辑器；3x-ui 的“分支默认工厂”最适合用来为每个协议定义“新建时/切换后的合法空骨架”，例如 WireGuard peers、Hysteria2 obfs、TUIC 认证互斥。
- 【3x-ui 事实】Hysteria2 的 `obfs`/`ports` 在 3x-ui 不是简单顶层字段，而是分层到 finalmask UDP salamander 与 QUIC hop。当前项目 Hysteria/Hysteria2 schema 目前用顶层 `obfs/obfs-password/ports/hop-interval`，后续完整条件表单应评估是否需要“分层表达”或维持顶层中间对象但让目标投影负责转换。
- 【3x-ui 事实】WireGuard 在 3x-ui 中严格区分服务端密钥/入站与客户端 Peer；当前项目 R27-07 已实现稳定 Peer 身份，后续可继续借用“地址/保留字节 UI 逗号字符串、wire 数组”的 adapter 模式，以及 Peer 数组的稳定 key/错误定位。
- 【3x-ui 事实】SS2022 的方法枚举与密钥长度/多用户边界可作为当前项目 SS2022 “pending” 专项的参考；URI 侧 SIP022 的 percent-encode 语义比 base64 更准确。
- 【可能】TUIC/AnyTLS/Snell/Mieru/MASQUE/OpenVPN/SSH/ShadowQUIC/TrustTunnel/Tailscale 在 3x-ui 中没有直接 schema，因此仍需以项目现有 Reference、Mihomo/客户端生态和真实客户端文档为准；3x-ui 只能提供“表单/服务端/客户端字段分离”的方法论。

#### 4.3 独立 Xray outbound 输出与固定验证 profile

- 【经推理】3x-ui 的 `schemas/forms/outbound-form.ts` + `outbound-form-adapter.ts` + `internal/sub/json_service.go` 是“把节点/入站转成单条 Xray outbound”的现成工程样例。
- 【经推理】当前项目若推进 Design4 §9.2 的独立 Xray outbound，可采用：
  1. 节点中间对象 → Xray wire adapter（扁平设置转 `vnext/servers/peers`）；
  2. 固定验证 profile 中调用真实 Xray core `Build()` 或离线固定版本校验；
  3. 将版本作为检查参数，做版本 gate。
- 【可能】若未来需要多 outbound 编排，可参考 3x-ui `json_service.go` 的 balancer/observatory tag 规则，但当前项目应以“单节点可验证 outbound”为起点，不直接引入 balancer。

#### 4.4 URI 导入与远程刷新

- 【经推理】当前项目可按 3x-ui 的稳定 identity 思路，为“同一远端服务器换名后仍可识别”引入身份摘要；但必须用加密/哈希，不能明文持久化。
- 【经推理】当前 `uriparse` 协议覆盖面已比 3x-ui 更广，可补的是 3x-ui 在这些重叠协议上的健壮边界：SS2022 SIP022、Hysteria2 `obfs/mport/fm`、XHTTP 的 `extra`/snake_case 别名、WireGuard 参数别名。
- 【可能】3x-ui 有 `outbound_fuzz_test.go`；当前项目 `uriparse` 若有 fuzz 测试，能低代价发现编码/别名边界问题。

#### 4.5 JSON 编辑与错误 UX

- 【经推理】当前项目保留“局部 JSON 草稿 + 完整目标只读检查”是合理的；若未来开放“完整 JSON 可写/可回灌”，应明确 adapter 是否丢弃未知键。3x-ui 的 Outbound 完整 JSON 回灌是有损的，不适合当前项目保留未知参数的目标。
- 【经推理】路径级 JSON 编辑（例如只编辑 `ws-opts`、`plugin-opts`、`reality-opts`）可借鉴 Host 子树 JSON 覆盖；它让高级用户不写整段节点 JSON。
- 【可能】引入 CodeMirror 类 JSON 编辑器可提供语法 lint/行列定位，但业务校验仍必须走服务端/共享规则，不能把 JSON 语法通过当成目标通过。

---

### 五、应保留边界 / 不应照搬（经推理）

1. **3x-ui 的 Inbound 服务端字段不能进入当前 manual 节点编辑器**：`clients[]`、fallbacks、sniffing、入站限流/流量重置、Reality privateKey/target、TLS 证书私钥等仍属于服务端；前文已确认，本文继续保留。
2. **3x-ui 的 hysteria 命名是 Hysteria2**：引用时必须显式区分当前项目的 Hysteria1 与 Hysteria2，避免协议映射错位。
3. **3x-ui 的完整 JSON 回灌可能丢未知字段**：当前项目对未知 SS 插件参数“明文保留/回显”是已确认方向，不应因借鉴 JSON 编辑而改丢。
4. **3x-ui 的 SubService 是单面板、按 subscriber 的订阅分发**：当前项目是多目标管理，不应整体搬入。
5. **3x-ui 的 identity 明文含凭据**：若要借鉴，需改成加密存储或不可逆指纹。
6. **3x-ui 不覆盖 SS 插件（obfs/v2ray-plugin/shadow-tls/restls）**：SS 插件仍以当前项目 `ssplugin` 合同与 Mihomo 固定版本证据为准。
7. **3x-ui 的协议 schema 面向 Xray wire**：不能直接替换当前项目的 Mihomo 风格 `protocol_json`，只能作为 Xray target adapter 的输入来源。

---

### 六、候选清单（供后续 Design/Build 前决策）

| 编号 | 候选方向 | 当前状态 | 建议后续处理 |
|---|---|---|---|
| C1 | 为协议/分支建立“默认对象工厂”，辅助其余 15 协议完整表单 | 未开始 | 进入 Design 阶段，结合 `FieldSchema.Default` 与新建/切换空骨架 |
| C2 | 为独立 Xray outbound 引入“节点中间对象 ↔ Xray wire adapter” | 后续专项 | 在 Design4 §9.2 推进时先定 wire profile/字段映射 |
| C3 | URI/订阅刷新引入稳定 identity（加密/摘要版）与 tag 复用 | 后续可能 | 需用户确认是否需要“远程订阅源反复刷新”，不用于一次性导入 |
| C4 | 路径级 JSON 切片编辑/完整目标 JSON 只读派生预览 | 可作 R27-09 Step13/后续 UI 增强 | 需明确 JSON 权威模型与未知参数保留边界 |
| C5 | 错误路径统一映射到 group/section + 列表稳定身份 | 已部分具备 | 作为剩余 15 协议表单验收增强 |
| C6 | 固定版本 client/core 作为检查参数并接入真实/离线校验 | 后续 Xray outbound | 不扩展到所有目标；版本 gate 需封装 |
| C7 | Host/endpoint 覆盖层 | 后续可能 | 仅当出现“同节点多入口/多 CDN”需求时再评估 |
| C8 | 3x-ui 的 CodeMirror JSON 编辑器 | 可评估 | 不替代业务校验；可作为高级 JSON 交互升级 |
| C9 | 3x-ui 的 AmneziaWG/Hysteria2 obfs/mport/SS2022 字段参考 | 后续协议专项 | 不直接照搬 schema，先做目标映射与命名核对 |

---

### 七、证据索引

#### 7.1 3x-ui 本地证据（本文新增/深化）

| 证据 | 位置 |
|---|---|
| 分支默认对象工厂 | `3x-ui/frontend/src/lib/xray/outbound-form-helpers.ts`、`3x-ui/frontend/src/pages/inbounds/form/InboundFormModal.tsx`、`3x-ui/frontend/src/schemas/protocols/stream/xhttp.ts` |
| 表单/wire adapter | `3x-ui/frontend/src/schemas/forms/outbound-form.ts`、`3x-ui/frontend/src/lib/xray/outbound-form-adapter.ts`、`inbound-form-adapter.ts`、`stream-wire-normalize.ts` |
| Link 导入与 parser | `3x-ui/frontend/src/lib/xray/outbound-link-parser.ts`、`3x-ui/frontend/src/pages/xray/outbounds/OutboundFormModal.tsx` |
| Go link parser/identity/tag | `3x-ui/internal/util/link/outbound.go`、`3x-ui/internal/web/service/outbound_subscription.go` |
| 后端订阅/Clash/JSON | `3x-ui/internal/sub/service.go`、`clash_service.go`、`clash_yaml.go`、`json_service.go`、`host_sub.go`、`endpoint.go` |
| JSON 三种模型 | `3x-ui/frontend/src/pages/inbounds/form/advanced-editors.tsx`、`3x-ui/frontend/src/pages/xray/outbounds/OutboundFormModal.tsx`、`3x-ui/frontend/src/pages/hosts/json-forms/*` |
| 错误路径映射 | `3x-ui/frontend/src/pages/inbounds/form/formatValidationError.ts`、`InboundFormModal.tsx` |
| 数组稳定 key/空行 | `3x-ui/frontend/src/components/form/HeaderMapEditor.tsx`、`3x-ui/frontend/src/lib/xray/forms/transport/FinalMaskForm.tsx`、`3x-ui/frontend/src/pages/xray/outbounds/protocols/wireguard.tsx` |
| write-only 凭据 | `3x-ui/internal/web/service/node_contract.go`、`node_credentials_writeonly_test.go` |
| 真实 core 校验/版本 gate | `3x-ui/internal/xray/api.go`、`internal/web/service/xray_setting.go`、`outbound_subscription.go` |
| 协议 schema 与近期新增 | `3x-ui/frontend/src/schemas/protocols/**`、`frontend/src/lib/xray/protocol-capabilities.ts`、`outbound-defaults.ts`、`stream-defaults.ts`、`inbound-defaults.ts`、`amneziawg-obfuscation.ts` |

#### 7.2 当前项目证据

| 证据 | 位置 |
|---|---|
| 当前状态/保存契约 | `VPN-Subscription-Management/backend/migrations/1017_node_editor_state.sql`、`backend/internal/node/node.go` |
| FieldSchema 条件/投影 | `backend/internal/node/schema.go`、`registry.go`、`project.go` |
| 节点检查 | `backend/internal/node/check.go`、`backend/internal/assembly/node_check.go` |
| 前端动态表单/JSON | `frontend/src/views/admin/NodesView.vue`、`frontend/src/components/ProtocolFieldEditor.vue`、`frontend/src/components/NodeCheckPanel.vue` |
| URI 导入 | `backend/internal/uriparse/uriparse.go`、`backend/internal/node/uri_import.go`、`normalize.go` |
| SS 插件合同 | `backend/internal/ssplugin/contract.go`、`sip002.go`、`backend/internal/assembly/links/links.go` |
| Clash SS 插件输出 | `backend/internal/assembly/render_clash.go`、`ssplugin/contract.go`（Build21 Step11 后为结构化投影） |
| 收口跟踪 | `Build21.md` §7、`docs/reports/Issue/Issue13.md` R27-09、`Issue14.md`（后续专项） |

#### 7.3 外部资料

- [3x-ui GitHub](https://github.com/MHSanaei/3x-ui)
- [Xray-core config docs](https://xtls.github.io/en/config/)
- [Mihomo / Clash.Meta docs](https://wiki.metacubex.one/en/config/proxies/)
- [Node-Editor 既有研究汇总](Node-Editor-Research.md)、[3x-ui/Xray 前文研究](Node-Editor-3xui-Xray-Research.md)

> 外部资料仅用于补充语义；本地源码取证优先。引用外部链接不表示对对方项目进行改动或背书。

---

## 九、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-02 | 新建独立 Reference 文档：分析 3x-ui Inbound/Outbound 表单架构、能力判断、wire 适配和 JSON 模式；对照 Xray-examples 客户端样例字段；梳理当前项目节点处理与差异；将 3x-ui 完整 JSON 可编辑模式记录为后续候选。仅文档，未改动项目代码或外部项目。 |
| v1.1 | 2026-09-02 | 按 Design4 v1.2 同步早期“保留非激活分支 / 独立编辑状态”表述，改为“切换即清空、不保存恢复副本、行内当前状态”。仅文档同步，未改动项目代码或外部项目。 |
| v2.0 | 2026-09-05 | 新建第二轮深度研究（原 `Node-Editor-3xui-Xray-Research-2.md`），在 Build17～Build21 基础上分析 3x-ui v3.7.0 后续可借鉴机制。 |
| v2.1 | 2026-09-08 | Reference 规范化：将第二轮文档并入本文“八、第二阶段”；同步 Design4 v1.14 / Build21 收口状态，更新过期链接与“尚未实施”表述。 |
