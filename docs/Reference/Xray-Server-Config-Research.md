# Xray-Node-Config-Requirements.md — Xray 节点配置需求与新建表单改进研究

> **文档定位：** 本文是节点管理模块新建节点表单的只读研究资料，提取 Xray-examples 服务端配置中对客户端节点真正有用的配置项，并与当前项目的节点协议注册表、节点链接输出和 Clash 客户端字段进行对照。本文只记录研究结论、证据边界和 UI 改进方向，不直接定义构建步骤，也不替代 Design 文档中的最终设计决策。
> **研究范围：** 当前项目 `ManualProtocols()` 的全部 19 种手工节点协议；`hysteria` 与 `hysteria2` 分开处理。Xray-examples 中没有对应服务端样本的协议，会明确标注为“样本未覆盖”，不把其他协议的配置推断为 Xray 服务端事实。
> **标注约定：** 【样本事实】= 直接来自 Xray-examples 的 `server.jsonc` / `config_server.jsonc`；【项目事实】= 当前项目代码或 Reference 文档已有字段/输出约定；【UI 建议】= 面向后续表单改进的建议；【待确认】= 需要用户在实施前确认的产品或兼容性选择。

---

## 一、研究结论摘要

### 1.1 总体方向

新建节点表单应从“按协议平铺全部字段”改为“协议 → 传输方式 → 安全方式”驱动的条件表单：

1. 节点名称、服务器、端口和当前协议的核心凭据始终直接展示。
2. 传输方式与安全方式作为公共选择项；只有当前组合需要的参数才展示。
3. 当前组合的必填项直接展示，常用但非必填项与其放在同一可见层。
4. 性能、兼容、旧协议扩展和未知参数默认折叠。
5. 沿用 Issue12 中“分区卡片、默认折叠、结构化编辑 / 高级 JSON、未知键保留”的交互方向，但不把服务端配置文件直接复制成客户端节点表单。
6. 服务端证书文件、REALITY 私钥、`dest` / `target`、fallback、sniffing、routing 和 outbounds 等仅属于服务端部署的内容，不进入手工节点的常用填写区。

### 1.2 样本统计

对 `/Users/kyle/Desktop/Repo/Xray-examples/` 下名称为 `server.jsonc` 或 `config_server.jsonc` 的文件进行只读检查，共得到 33 个服务端配置文件和 50 个可识别的业务入站：

| 协议 | 入站数 | 覆盖文件数 | 样本中常见的传输/安全组合 |
|------|-------:|-----------:|---------------------------|
| VLESS | 26 | 21 | TCP、WS、gRPC、H2/HTTP、XHTTP、SplitHTTP、mKCP；无/TLS/REALITY |
| VMess | 11 | 8 | TCP、WS、gRPC、H2/HTTP、mKCP；无/TLS |
| Trojan | 6 | 3 | TCP、WS、gRPC、H2；无/TLS |
| Shadowsocks | 5 | 2 | TCP、WS、gRPC、H2；样本均无外层安全 |
| Hysteria v2 | 1 | 1 | Hysteria 网络 + TLS |
| SOCKS | 1 | 1 | TCP + TLS |

传输样本数量依次为 TCP 20、WS 8、gRPC 7、H2 5、HTTP 3、SplitHTTP 2、XHTTP 2、mKCP 2、Hysteria 1。该数字只反映该目录的样本密度，不代表整个生态的实际使用率。

### 1.3 证据范围边界

当前项目注册表有 19 个手工协议：[registry.go](../../backend/internal/node/registry.go) 中的 `ManualProtocols()` 是运行时表单协议清单。Xray-examples 的服务端样本只直接覆盖其中的 VLESS、VMess、Trojan、Shadowsocks、Hysteria/Hysteria2 和 SOCKS 类配置。

因此本文采用两层证据：

- 对上述六类：使用 Xray 服务端配置提取“服务端配置中对应的客户端连接需求”，再与项目输出字段交叉核对。
- 对其余协议：使用当前项目注册表、[Clash-Verge-Rev-Node-Parameters.md](Clash-Verge-Rev-Node-Parameters.md)、[Node-Link-Standards.md](Node-Link-Standards.md) 和现有 URI/Clash 输出能力，仅给出表单分层建议，不声称这些字段来自 Xray-examples 服务端配置。

---

## 二、服务端配置与节点表单的转换边界

### 2.1 不应直接照搬 `server.jsonc`

Xray 的服务端入站配置同时包含监听、认证、传输、安全证书、路由和出站等多类信息；节点管理模块保存的是“远端节点的客户端连接描述”，二者不是同一个对象。

| 服务端配置位置 | 节点表单处理 | 原因 |
|---|---|---|
| `inbounds[].protocol` | 展示为协议类型 | 决定客户端凭据和输出格式 |
| `inbounds[].port` | 映射为节点端口 | 远端客户端需要连接的端口 |
| `inbounds[].listen` | 不展示 | 是服务端本地监听地址，不是客户端服务器地址 |
| `settings.clients[].id` / `password` | 展示为 UUID / 密码 | 是 VLESS、VMess、Trojan 的客户端身份凭据 |
| `settings.method` + `settings.password` | 展示为加密方式 + 密码 | 是 Shadowsocks 的连接凭据 |
| `settings.accounts[]` | 映射为 SOCKS5 认证信息 | 需要结合认证模式展示 |
| `settings.decryption` | 不照搬；客户端维持 `encryption` 语义 | VLESS 服务端的解密设置与客户端 URI 字段不是同一层含义；样本基本为 `none` |
| `streamSettings.network` | 映射为传输方式 | 客户端需要选择同一传输族 |
| `wsSettings` / `grpcSettings` / `httpSettings` 等 | 映射为项目现有 `ws-opts` / `grpc-opts` / `h2-opts` / `http-opts` 等对象 | 项目存储和输出采用客户端/Clash 侧字段名，不能机械复制 Xray JSON 层级 |
| `tlsSettings.certificates` | 不展示证书文件；展示客户端 TLS 相关字段 | 证书文件由服务端持有，客户端通常只需要安全模式、SNI、ALPN、校验策略和指纹 |
| `realitySettings.privateKey` | 不展示私钥；展示公钥、Short ID、SNI、客户端指纹 | 私钥与 `dest` / `target` 属于服务端；客户端使用对应公钥和参数 |
| `fallbacks` / `sniffing` / `routing` / `outbounds` | 不展示 | 属于服务端流量处理和路由，不构成节点连接参数 |

这一边界与项目已有的节点链接和 Xray 传输字段研究一致：[Xray-Core-API.md](Xray-Core-API.md) 将节点行所需的客户端字段归纳为 network、安全类型、TLS SNI/指纹、REALITY 公钥/Short ID、WS path/header 和 gRPC service name；[Node-Link-Standards.md](Node-Link-Standards.md) 则约束最终 URI 输出字段。

### 2.2 样本中最稳定的必填凭据

| 协议 | 服务端样本中的稳定凭据 | 表单建议 |
|---|---|---|
| VLESS | `clients[].id`，26/26 个 VLESS 入站出现 | UUID 直接展示并作为必填 |
| VMess | `clients[].id`，11/11 个 VMess 入站出现 | UUID 直接展示并作为必填 |
| Trojan | `clients[].password`，6/6 个 Trojan 入站出现 | 密码直接展示并作为必填 |
| Shadowsocks | `method` + `password`，5/5 个 Shadowsocks 入站出现 | 加密方式、密码直接展示并作为必填 |
| Hysteria v2 | `hysteriaSettings.auth`，样本同时有 `settings.version: 2` | 与 Hysteria v1 分开，使用 Hysteria2 的认证字段 |
| SOCKS | `auth: password` + `accounts[].user/pass` | 先选认证模式；用户名密码模式下再显示并要求账号字段 |

`email`、`level` 等出现在 Xray 客户端对象中，但它们是服务端用户管理或统计标识，不是远端节点连接凭据，建议不要放入普通节点填写区。

---

## 三、按协议提取的常用配置需求

### 3.1 有 Xray 服务端样本的协议

#### VLESS

【样本事实】VLESS 是样本最多的协议，覆盖 TCP、WS、gRPC、H2、HTTP、XHTTP、SplitHTTP 和 mKCP，并出现无安全、TLS 和 REALITY 三类安全模式。参考：[VLESS TCP/TLS](../../../Xray-examples/VLESS-TCP-TLS/config_server.jsonc)、[VLESS TCP/REALITY](../../../Xray-examples/VLESS-TCP-REALITY%20(without%20being%20stolen)/config_server.jsonc)、[VLESS gRPC](../../../Xray-examples/VLESS-GRPC/server.jsonc)、[VLESS XHTTP/REALITY](../../../Xray-examples/VLESS-XHTTP-Reality/minimal-steal_others/server.jsonc)。

【UI 建议】

- 直接展示：UUID。
- 连接方式：TCP、WS、gRPC、H2、HTTP、XHTTP、SplitHTTP、mKCP。
- 安全方式：无、TLS、REALITY；不要只用一个模糊的 TLS 布尔值表达三种状态。
- TCP + Vision 或相关配置下显示 `flow`；它不是所有 VLESS 节点的必填项。
- TLS 下显示 SNI、ALPN、跳过证书校验、客户端指纹。
- REALITY 下显示 SNI、公钥、Short ID、客户端指纹；不显示服务端私钥、`dest` / `target`。
- WS 显示路径和可选 Host；gRPC 显示 ServiceName；H2/HTTP 显示路径和 Host；XHTTP 显示路径、Host、模式；mKCP 显示 Seed。

#### VMess

【样本事实】VMess 样本覆盖 TCP、WS、gRPC、H2/HTTP 和 mKCP；安全方式出现无安全和 TLS，没有 VMess REALITY 样本。参考：[VMess TCP/TLS](../../../Xray-examples/VMess-TCP-TLS/config_server.jsonc)、[VMess HTTP 伪装](../../../Xray-examples/VMess-HTTP/config_server.jsonc)、[VMess mKCP](../../../Xray-examples/VMess-mKCPSeed/config_server.jsonc)。

【UI 建议】

- 直接展示：UUID。
- 常用配置：加密方式（默认 `auto`）、传输方式、TLS、SNI、ALPN。
- WS/H2/HTTP/gRPC/mKCP 参数按传输方式条件展示。
- `AlterId` 默认放入兼容/高级区域，默认值保持 `0`；不要让旧字段压缩 UUID 和传输信息。
- VMess 样本中的 TCP HTTP header 伪装属于兼容扩展，默认折叠，不与普通 TCP 基础字段混在一起。

#### Trojan

【样本事实】Trojan 样本覆盖 TCP、WS、gRPC、H2；TCP 样本使用 TLS，WS/gRPC/H2 样本多为反向代理前置后的无安全入站。参考：[Trojan TCP/TLS](../../../Xray-examples/Trojan-TCP-TLS%20(minimal)/config_server.jsonc)、[Trojan gRPC](../../../Xray-examples/Trojan-gRPC-Caddy2%EF%BC%8FNginx/server.jsonc)。

【UI 建议】

- 直接展示：密码。
- 连接方式：TCP、WS、H2、HTTP、gRPC、XHTTP；当前项目注册表中的传输选项保留，但按实际选项渲染。
- 安全方式：无、TLS；REALITY 是否作为 Trojan 的常用选项不能由当前样本确认，建议先放入兼容/高级能力或待确认。
- TLS 下展示 SNI、ALPN、跳过证书校验和客户端指纹。
- WS 显示路径和 Host；gRPC 显示 ServiceName。
- 内层 SS `ss-opts` 默认折叠；它不是普通 Trojan 节点的基础必填信息。

#### Shadowsocks

【样本事实】Shadowsocks 样本出现 TCP、WS、gRPC、H2，基础配置稳定为 `method` + `password`。参考：[Shadowsocks TCP](../../../Xray-examples/Shadowsocks-TCP/server.jsonc)、[All-in-One 多入站样本](../../../Xray-examples/All-in-One-fallbacks-Nginx/server.jsonc)。

【UI 建议】

- 直接展示：加密方式、密码。
- 插件类型作为可选的第二层选择；选择插件后再显示对应的插件参数。
- 插件参数的常用字段为 mode、Host、path、TLS、指纹、跳过证书校验和 headers。
- UDP、UDP over TCP、TFO 和 SMux 默认折叠。
- 不建议仅凭 Xray 服务端的 `streamSettings.network` 增加 Shadowsocks 原生 WS/gRPC/H2 选择。当前项目的订阅输出主要通过 `plugin` / `plugin-opts` 表达；[Node-Link-Standards.md](Node-Link-Standards.md) 也记录了 Shadowsocks 插件在通用链接输出中的兼容限制。

#### Hysteria 与 Hysteria2

【样本事实】目录中的 Hysteria 示例使用：

- `protocol: "hysteria"`
- `settings.version: 2`
- `streamSettings.network: "hysteria"`
- `hysteriaSettings.auth`
- TLS + SNI + ALPN `h3`

参考：[Hysteria2 server.jsonc](../../../Xray-examples/Hysteria2/server.jsonc)。这说明 Xray 服务端配置名称与客户端 URI/项目协议名称存在版本映射，不能据此把两个客户端协议合并。

【UI 建议】

- `hysteria` 与 `hysteria2` 分成两个协议入口和两套字段 schema。
- Hysteria v1 重点展示：认证、协议、端口组、上下行速度、混淆、SNI、ALPN。
- Hysteria2 重点展示：密码、SNI、ALPN、混淆、混淆密码、端口跳跃。
- Hysteria2 的 `h3` 是样本中的常见 ALPN，但不应对所有部署强制写死；可作为默认值或提示值，最终以节点实际服务端为准。
- 接收窗口、MTU、拥塞窗口、Fast Open、Hop 间隔等默认折叠。
- `auth` 与 `auth-str` 的互斥或优先级、以及 Xray `hysteria + version:2` 与项目 `hysteria2` 的存储映射，列入实施前待确认项。

#### SOCKS5

【样本事实】SOCKS 示例为 `protocol: "socks"`，使用 `auth: "password"`、账号数组、UDP 和 TCP + TLS。参考：[SOCKS5 TLS](../../../Xray-examples/Socks5-TLS/config_server.jsonc)。

【UI 建议】

- 先展示认证模式：无认证 / 用户名密码。
- 只有用户名密码模式显示用户名和密码，并在该模式下要求两者。
- UDP 开关直接放在常用层。
- TLS 开关或安全方式选择后显示 SNI、跳过证书校验和指纹。
- 服务端的 `ip` 是服务端入站设置，不应直接当成客户端普通字段。

### 3.2 当前项目支持但 Xray-examples 未覆盖的协议

以下协议在当前项目的手工节点注册表中存在，但在本次 33 个服务端配置文件中没有对应的可用服务端入站样本。下面的字段依据来自项目现有注册表和客户端字段参考，属于【项目事实】与【UI 建议】，不是 Xray-examples 的服务端取证。

| 协议 | 常用/必填层建议 | 默认折叠内容 | 证据边界与备注 |
|---|---|---|---|
| TUIC | Token，或 UUID + 密码的认证信息；SNI、ALPN | 心跳、请求超时、拥塞、窗口、MTU、UDP over Stream | 样本未覆盖；当前注册表把三类凭据都列为可选，认证方式需要条件规则 |
| WireGuard | 私钥、公钥、IP/IPv6、Allowed IPs；服务端和端口仍在基本信息 | Peer 列表、Reserved、PSK、MTU、DNS、Worker、Keepalive | 样本未覆盖；密钥和地址属于结构化配置，不能按普通文本字段平铺 |
| HTTP | 服务器和端口；用户名/密码按服务端认证情况填写 | TLS、SNI、证书校验、指纹、headers | 样本未覆盖；当前项目有标准 URI 映射 |
| SOCKS5 | 见上节 | 见上节 | 当前项目注册表另有 `socks5`，Xray 样本使用 `socks` 入站名称；客户端表单应统一为 SOCKS5 展示名 |
| Snell | PSK | UDP、版本 | 样本未覆盖；当前项目有 PSK 必填定义，但没有通用链接映射 |
| AnyTLS | 密码；SNI、ALPN、客户端指纹 | ECH、证书/私钥、空闲会话参数 | 样本未覆盖；证书和私钥是否作为客户端能力暴露需要单独确认 |
| Mieru | 用户名、密码；传输方式、端口范围 | 多路复用、握手模式、UDP | 样本未覆盖；当前注册表将用户名和密码标为必填 |
| MASQUE | 私钥、公钥；IP/IPv6、MTU | DNS、远端 DNS、UDP | 样本未覆盖；当前注册表将密钥标为必填，无标准 URI 映射 |
| OpenVPN | 客户端配置全文 | 所有额外通用参数 | 样本未覆盖；更适合专用配置文本区域，不适合拆成少量伪字段 |
| SSH | 用户名；密码或私钥二选一 | 私钥口令、Host Key、Host Key 算法 | 样本未覆盖；二选一校验需要条件规则，当前静态 schema 尚未表达 |
| ShadowQUIC | 密码 | SNI、通用网络参数 | 样本未覆盖；当前注册表仅有密码和 SNI 字段，且无通用链接映射 |
| TrustTunnel | 密码（是否必填以实际协议能力为准） | 通用网络参数 | 样本未覆盖；当前注册表仅提供密码字段，证据不足以增加更多前置字段 |
| Tailscale | 认证密钥（按部署方式） | 通用网络参数 | 样本未覆盖；当前注册表仅提供认证密钥字段，不建议凭空增加服务端字段 |

上述协议暂不应为了“看起来完整”而复刻 Xray VLESS/VMess 的 network、TLS 或 Reality 字段。应以各自的客户端配置模型和项目实际输出能力为准。

### 3.3 当前注册表之外的参考协议

- `ssr`：项目已明确不纳入当前 `ManualProtocols()`，且 [Node-Link-Standards.md](Node-Link-Standards.md) 记录了其没有可靠的生成侧标准映射；本文不把它作为新建节点表单的当前对象。
- `sudoku`：出现在 Clash-Verge-Rev 字段参考中，但不在当前项目 19 项手工协议注册表，也没有本次 Xray-examples 服务端样本；不纳入本次表单建议。
- `direct` / `dns`：属于代理或出站能力，不是远端节点类型。

---

## 四、按传输方式提取的条件字段

以下字段是跨 VLESS、VMess、Trojan，以及部分 Shadowsocks 配置最适合抽象为“传输方式选择后显示”的部分。项目存储仍应使用当前 `protocol_json` 和输出端认可的字段名。

| 传输方式 | 常用字段 | 建议必填规则 | 默认折叠字段 |
|---|---|---|---|
| TCP | 通常无额外字段 | 无 | HTTP header 伪装、PROXY protocol、socket 扩展 |
| WebSocket | `path`、Host header | 选择 WS 后 path 通常应要求填写；Host 按部署情况可选 | Early Data、HTTP Upgrade、PROXY protocol |
| gRPC | ServiceName | 选择 gRPC 后 ServiceName 应要求填写 | gRPC 多路复用或未来扩展 |
| H2 | `path`、Host | path 通常应填写；Host 按反向代理配置决定 | 未知 HTTP/2 扩展 |
| HTTP | path 列表、Host/header、method | 选择 HTTP 后至少应提供有效 path；Host 按部署情况可选 | 完整 headers 和响应伪装对象 |
| XHTTP | `path`、Host、mode | path 应直接展示；mode 在当前项目可支持值中选择 | 其他 XHTTP 扩展对象 |
| SplitHTTP | `path`、Host | path 应直接展示；Host 按部署情况可选 | 版本扩展和未知键 |
| mKCP | Seed | 选择 mKCP 后 Seed 应直接展示 | header、伪装类型和其他 KCP 扩展 |
| Hysteria | protocol、认证、上下行、混淆 | 由 Hysteria/Hysteria2 各自 schema 决定 | 窗口、MTU、拥塞和 Hop 参数 |

【样本事实】服务端配置中对应的对象名主要是 `wsSettings`、`grpcSettings`、`httpSettings`、`xhttpSettings`、`splithttpSettings`、`kcpSettings`；【项目事实】当前节点字段使用 `ws-opts`、`grpc-opts`、`h2-opts`、`http-opts`、`xhttp-opts` 等客户端侧对象。两者只做语义映射，不做层级复制。

---

## 五、按安全方式提取的条件字段

### 5.1 无安全

- 不展示 TLS 证书、REALITY 和校验字段。
- 仍保留传输方式及其 path、Host、ServiceName 等参数。
- 反向代理终止 TLS 的 WS/gRPC/H2 节点可落入该模式；服务端样本中已有这种结构。

### 5.2 TLS

常用客户端字段：

- SNI / server name
- ALPN
- 跳过证书校验
- TLS 指纹 / client fingerprint

服务端样本中的 `certificates`、`certificateFile`、`keyFile` 和 `minVersion` 等不应直接作为普通节点字段。证书路径是远端服务端本地路径，客户端无法使用。

### 5.3 REALITY

常用客户端字段：

- SNI
- 公钥
- Short ID
- 客户端指纹
- VLESS 的 `flow`（仅在适用的 Vision 等组合中）

服务端样本中的 `privateKey`、`dest` / `target`、`serverNames` 列表和 `shortIds` 列表需要转换理解：客户端通常使用与之对应的公钥、选定的 server name 和 short ID，而不是把服务端完整 `realitySettings` 展示出来。

### 5.4 安全字段的 UI 建议

建议将当前 VMess/VLESS 使用的 `tls: boolean` 逐步抽象为条件安全选择：

```text
安全方式：无 / TLS / REALITY

无       → 不追加安全字段
TLS      → SNI、ALPN、校验策略、客户端指纹
REALITY  → SNI、公钥、Short ID、客户端指纹、适用时的 Flow
```

这属于后续设计建议，不是本次文档变更后的代码行为。

---

## 六、建议的新建节点 UI 分层

### 6.1 第一层：必填信息

始终展开，保持与当前表单一致的基本信息位置：

- 协议
- 节点名称
- 服务器
- 端口
- 当前协议的核心凭据

例如 VLESS 直接显示 UUID，Trojan 直接显示密码，Shadowsocks 直接显示加密方式和密码；不要让用户先打开“认证与密钥”后才能看到唯一必填凭据。

### 6.2 第二层：连接方式

协议选定后显示：

- 传输方式
- 安全方式（适用于有此概念的协议）
- 当前组合的必填参数

建议在分区标题或摘要中显示组合结果，例如：`VLESS · WS · TLS`、`Trojan · gRPC · 无安全`。切换传输或安全方式时，只切换相关字段的显示，不把所有传输对象同时平铺。

### 6.3 第三层：常用可选参数

与当前组合相关但不是必填的内容可以继续显示在当前分区，例如：

- TLS 的 SNI、ALPN、客户端指纹
- WS 的 Host
- H2/HTTP 的 Host
- VLESS 的 Flow
- UDP 开关

这类字段虽然不是所有节点都必填，但对常见节点的连通性或导出结果有直接影响，不建议全部推入高级区。

### 6.4 第四层：高级参数

默认折叠，按 Issue12 的结构化分区思路处理：

- 兼容参数
- 性能参数
- SMux、TFO、MPTCP
- HTTP header 或 Early Data
- KCP 扩展
- 内层 SS
- 协议未来扩展
- 未知键和高级 JSON

对象类型继续默认结构化编辑，并保留对象级“高级 JSON”切换。已有 [ProtocolFieldEditor.vue](../../frontend/src/components/ProtocolFieldEditor.vue) 的对象、映射、对象数组和未知键保留模式可作为交互参考。

### 6.5 响应式行为

节点新建仍可沿用 [NodesView.vue](../../frontend/src/views/admin/NodesView.vue) 当前的桌面 Modal / 移动端全屏 Drawer。分区折叠状态和字段顺序在桌面、移动端保持一致；移动端只改变网格列数和容器，而不改变“必填优先”的信息层级。

---

## 七、对当前实现的影响评估

### 7.1 当前实现已具备的能力

- 节点表单已经按认证、传输、安全、开关和高级参数划分区域。
- 协议注册表已经通过 `FieldSchema` 提供字段类型、默认值、分区、对象属性和敏感字段。
- 对象、映射和对象数组已有结构化编辑、高级 JSON 和未知键保留能力。
- `protocol_json` 已被 Clash、Shadowrocket 和通用链接渲染链路使用。

### 7.2 当前实现的主要缺口

当前 `FieldSchema` 主要表达静态 `required` 和 `section`，还不能表达：

- `network=ws` 时才显示 WS 对象；
- `network=grpc` 时才要求 ServiceName；
- `security=reality` 时才要求公钥和 Short ID；
- SOCKS5 选择密码认证后才要求用户名和密码；
- SSH 在密码和私钥之间的条件关系；
- 切换协议或传输后，隐藏值是否保留以及何时参与校验。

后续若实施动态表单，前端显示逻辑和后端校验必须共用同一套条件元数据。不能只在前端隐藏字段而保留静态后端必填规则，否则会出现 UI 可保存、服务端拒绝，或 UI 不显示但旧数据仍被错误校验的情况。

### 7.3 兼容性边界

- `protocol_json` 的已有键应继续可读；表单分区只是展示层组织，不应无故改变历史节点数据。
- 已保存但当前表单不认识的对象键继续保留，并在高级 JSON 中可见。
- 切换传输或安全方式时，隐藏字段是否清理属于【待确认】的破坏性语义；初步建议保留数据、仅让当前输出链路按激活组合读取。
- 只有实际有 SR / generic / Clash 输出映射的字段才应标记为“可导出”；不能因为 server.jsonc 中存在字段，就承诺所有目标格式都能表达。
- 当前项目已明确不支持 SSR 的生成侧映射；不应在本次表单改进中顺带恢复 SSR。

---

## 八、待确认与后续设计问题

以下内容不在本次文档写入中擅自决定，实施前应根据实际输出目标确认：

1. **Hysteria 版本映射：** Xray 服务端样本使用 `protocol=hysteria + version=2`，当前项目同时有 `hysteria` 和 `hysteria2`。本文按两个 UI 协议分开，但具体存储和导出映射仍需在设计阶段定稿。
2. **Shadowsocks 原生 Xray 传输：** 是否把 Xray 服务端 WS/gRPC/H2 入站作为独立的 Shadowsocks 表达，还是继续只通过插件表达，需要以 Clash、Shadowrocket 和 generic 三类输出的真实兼容性为准。
3. **条件必填规则：** WS path、gRPC ServiceName、mKCP Seed、REALITY 公钥/Short ID、SOCKS 账号模式和 SSH 凭据二选一，是否全部作为后端强校验，需与协议输出能力一起确认。
4. **未被 server.jsonc 覆盖的 12 类协议：** 本文记录的是当前项目字段基础和 UI 分层建议；若要补充服务端配置级别的精确必填规则，应分别引入各协议的权威服务端/客户端资料，不能继续从 Xray-examples 外推。

---

## 九、研究依据

- 外部样本目录：`/Users/kyle/Desktop/Repo/Xray-examples/` 下所有 `server.jsonc` / `config_server.jsonc` 文件。
- 当前项目协议注册表：[backend/internal/node/registry.go](../../backend/internal/node/registry.go)。
- 当前节点表单：[frontend/src/views/admin/NodesView.vue](../../frontend/src/views/admin/NodesView.vue)。
- 当前递归对象编辑器：[frontend/src/components/ProtocolFieldEditor.vue](../../frontend/src/components/ProtocolFieldEditor.vue)。
- [Xray-Core-API.md](Xray-Core-API.md)：Xray Account、传输和客户端节点字段研究。
- [Clash-Verge-Rev-Node-Parameters.md](Clash-Verge-Rev-Node-Parameters.md)：客户端侧 19 类协议字段参考。
- [Node-Link-Standards.md](Node-Link-Standards.md)：SR / generic 节点链接生成、解析和兼容性边界。
- [Issue12.md](../reports/Issue/Issue12.md)：订阅装配头部表单的分区、默认折叠、结构化编辑和未知键保留方向。

---

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-09-02 | 新建：研究 Xray-examples 服务端配置，覆盖当前 19 种手工节点协议的字段分层建议；区分 hysteria 与 hysteria2，记录服务端字段与客户端节点字段的转换边界。 |
