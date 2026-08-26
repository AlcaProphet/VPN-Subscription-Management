# Clash Verge Rev 节点/代理参数参考（Clash 客户端侧字段定义）

> **文档定位：** 本文提取 `clash-verge-rev` 前端类型定义中“节点（Proxy）”的完整参数结构，作为本项目装配模块中 manual 节点表单、协议注册表、Clash YAML 渲染的参考资料。
> **主要来源：** `src/types/global.d.ts`（`IProxyBaseConfig` 及各协议 interface）、`src/components/profile/proxies-editor-viewer.tsx`、`src/utils/uri-parser.ts`（如可另查 Node-Link-Standards.md）。
> **与已有文档的关系：** [Node-Link-Standards.md](Node-Link-Standards.md) 侧重于 URI 生成/解析；本文侧重于客户端 YAML 中节点对象的字段定义与语义。

> **姊妹篇：** 订阅编辑/规则/代理组/扩展机制见 [Clash-Verge-Rev-Subscription-Assembly.md](Clash-Verge-Rev-Subscription-Assembly.md)；订阅校验/格式/Emoji/API 见 [Clash-Subscription-Validation-Emoji-API.md](Clash-Subscription-Validation-Emoji-API.md)。

---

## 一、支持的协议类型总览

【源码事实】`IProxyConfig.type` 联合类型（`src/types/global.d.ts`）：

```ts
'ss' | 'ssr' | 'direct' | 'dns' | 'snell' | 'http' | 'trojan'
| 'anytls' | 'hysteria' | 'hysteria2' | 'tuic' | 'wireguard'
| 'ssh' | 'socks5' | 'masque' | 'vmess' | 'vless' | 'mieru' | 'sudoku'
```

对应的 interface 包括：

- `IProxyDirectConfig` / `IProxyDnsConfig`
- `IProxyHttpConfig`
- `IProxySocks5Config`
- `IProxySshConfig`
- `IProxyTrojanConfig`
- `IProxyAnyTLSConfig`
- `IProxyTuicConfig`
- `IProxyMieruConfig`
- `IProxyMasqueConfig`
- `IProxyVlessConfig`
- `IProxyVmessConfig`
- `IProxyWireguardConfig`
- `IProxyHysteriaConfig`
- `IProxyHysteria2Config`
- `IProxyShadowsocksConfig`
- `IProxySudokuConfig`
- `IProxyshadowsocksRConfig`
- `IProxySmuxConfig`
- `IProxySnellConfig`

当前项目 manual 协议注册表为 19 项封闭清单，且明确排除 `ssr`。Clash Verge Rev 客户端作为参照实现支持更多类型，可作为后续扩展候选（但要结合实际链接转换能力）。

---

## 二、所有节点共有的基础字段

【源码事实】`IProxyBaseConfig`：

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 节点名称，必填；节点编辑器中名称也用于拖拽/React key，空名称会被过滤 |
| `type` | string | 协议类型 |
| `tfo` | boolean | TCP Fast Open |
| `mptcp` | boolean | MPTCP |
| `interface-name` | string | 出站网卡名 |
| `routing-mark` | number | 路由标记（Linux fwmark） |
| `ip-version` | `'dual' \| 'ipv4' \| 'ipv6' \| 'ipv4-prefer' \| 'ipv6-prefer'` | IP 版本偏好 |
| `dialer-proxy` | string | 拨号代理（可指向另一个代理/组） |

【建议】当前项目节点表单可增加这些通用高级字段：`tfo`、`mptcp`、`interface-name`、`routing-mark`、`ip-version`、`dialer-proxy`。

---

## 三、各协议字段详细定义

### 3.1 HTTP（`http`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `username` | string | 用户名 |
| `password` | string | 密码 |
| `tls` | boolean | 是否 TLS |
| `sni` | string | SNI |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `headers` | object | 自定义请求头 |

### 3.2 SOCKS5（`socks5`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `username` | string | 用户名 |
| `password` | string | 密码 |
| `tls` | boolean | 是否 TLS |
| `udp` | boolean | 是否支持 UDP |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |

### 3.3 SSH（`ssh`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `username` | string | 用户名 |
| `password` | string | 密码 |
| `private-key` | string | 私钥 |
| `private-key-passphrase` | string | 私钥口令 |
| `host-key` | string | Host Key |
| `host-key-algorithms` | string | Host Key 算法 |

### 3.4 Trojan（`trojan`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `password` | string | 密码 |
| `alpn` | string[] | ALPN |
| `sni` | string | SNI |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `udp` | boolean | UDP |
| `network` | NetworkType | 传输：`ws`/`http`/`h2`/`grpc`/`tcp`/`xhttp` |
| `reality-opts` | RealityOptions | REALITY 配置 |
| `grpc-opts` | GrpcOptions | gRPC 配置 |
| `ws-opts` | WsOptions | WebSocket 配置 |
| `ss-opts` | object | 内层 SS 配置 |
| `client-fingerprint` | ClientFingerprint | 客户端指纹 |

`RealityOptions`：`public-key`、`short-id`。
`GrpcOptions`：`grpc-service-name`。
`WsOptions`：`path`、`headers`、`max-early-data`、`early-data-header-name`、`v2ray-http-upgrade`、`v2ray-http-upgrade-fast-open`。
`ClientFingerprint`：`chrome`/`firefox`/`safari`/`iOS`/`android`/`edge`/`360`/`qq`/`random`。

### 3.5 AnyTLS（`anytls`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `password` | string | 密码 |
| `alpn` | string[] | ALPN |
| `sni` | string | SNI |
| `client-fingerprint` | ClientFingerprint | 客户端指纹 |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `certificate` | string | 证书 |
| `private-key` | string | 私钥 |
| `ech-opts` | object | ECH：`enable`、`config` |
| `udp` | boolean | UDP |
| `idle-session-check-interval` | number | 空闲会话检查间隔 |
| `idle-session-timeout` | number | 空闲会话超时 |
| `min-idle-session` | number | 最小空闲会话 |

### 3.6 TUIC（`tuic`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `token` | string | Token |
| `uuid` | string | UUID |
| `password` | string | 密码 |
| `ip` | string | IP |
| `heartbeat-interval` | number | 心跳间隔 |
| `alpn` | string[] | ALPN |
| `reduce-rtt` | boolean | 减少 RTT |
| `request-timeout` | number | 请求超时 |
| `udp-relay-mode` | string | UDP 中继模式 |
| `congestion-controller` | string | 拥塞控制器 |
| `disable-sni` | boolean | 禁用 SNI |
| `max-udp-relay-packet-size` | number | 最大 UDP 中继包 |
| `fast-open` | boolean | Fast Open |
| `max-open-streams` | number | 最大并发流 |
| `cwnd` | number | 拥塞窗口 |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `ca` / `ca-str` | string | CA |
| `recv-window-conn` / `recv-window` | number | 接收窗口 |
| `disable-mtu-discovery` | boolean | 禁用 MTU 发现 |
| `max-datagram-frame-size` | number | 最大数据报帧 |
| `sni` | string | SNI |
| `udp-over-stream` | boolean | UDP over Stream |
| `udp-over-stream-version` | number | UDP over Stream 版本 |

### 3.7 Mieru（`mieru`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `port-range` | string | 端口范围 |
| `transport` | `'TCP' \| 'UDP'` | 传输 |
| `udp` | boolean | UDP |
| `username` | string | 用户名 |
| `password` | string | 密码 |
| `multiplexing` | `'MULTIPLEXING_OFF' \| 'LOW' \| 'MIDDLE' \| 'HIGH'` | 多路复用 |
| `handshake-mode` | string | 握手模式 |

### 3.8 Masque（`masque`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `private-key` | string | 私钥 |
| `public-key` | string | 公钥 |
| `ip` | string | IP |
| `ipv6` | string | IPv6 |
| `mtu` | number | MTU |
| `udp` | boolean | UDP |
| `remote-dns-resolve` | boolean | 远端 DNS 解析 |
| `dns` | string[] | DNS 列表 |

### 3.9 VLESS（`vless`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `uuid` | string | UUID |
| `flow` | string | Flow（如 `xtls-rprx-vision`） |
| `tls` | boolean | TLS |
| `alpn` | string[] | ALPN |
| `udp` | boolean | UDP |
| `packet-addr` | boolean | Packet Address |
| `xudp` | boolean | XUDP |
| `packet-encoding` | string | 包编码 |
| `network` | NetworkType | 传输 |
| `reality-opts` | RealityOptions | REALITY |
| `http-opts` | HttpOptions | HTTP 传输：`method`、`path[]`、`headers` |
| `h2-opts` | H2Options | H2：`path`、`host` |
| `grpc-opts` | GrpcOptions | gRPC |
| `ws-opts` | WsOptions | WebSocket |
| `xhttp-opts` | XHttpOptions | XHTTP：`path`、`host`、`mode` |
| `ws-path` | string | 兼容旧 WebSocket path |
| `ws-headers` | object | 兼容旧 WebSocket headers |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `servername` | string | SNI |
| `client-fingerprint` | ClientFingerprint | 客户端指纹 |
| `smux` | boolean | Smux |
| `encryption` | string | 加密 |

### 3.10 VMess（`vmess`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `uuid` | string | UUID |
| `alterId` | number | AlterId |
| `cipher` | CipherType | 加密方式 |
| `udp` | boolean | UDP |
| `network` | NetworkType | 传输 |
| `tls` | boolean | TLS |
| `alpn` | string[] | ALPN |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `servername` | string | SNI |
| `reality-opts` | RealityOptions | REALITY |
| `http-opts` / `h2-opts` / `grpc-opts` / `ws-opts` | object | 各传输配置 |
| `packet-addr` | boolean | Packet Address |
| `xudp` | boolean | XUDP |
| `packet-encoding` | string | 包编码 |
| `global-padding` | boolean | 全局填充 |
| `authenticated-length` | boolean | 认证长度 |
| `client-fingerprint` | ClientFingerprint | 客户端指纹 |
| `smux` | boolean | Smux |

### 3.11 WireGuard（`wireguard`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `public-key` | string | 公钥 |
| `pre-shared-key` | string | 预共享密钥 |
| `reserved` | number[] | 保留字节 |
| `allowed-ips` | string[] | 允许 IP |
| `ip` | string | 本机 IP |
| `ipv6` | string | 本机 IPv6 |
| `private-key` | string | 私钥 |
| `workers` | number | Worker 数 |
| `mtu` | number | MTU |
| `udp` | boolean | UDP |
| `persistent-keepalive` | number | 持久 Keepalive |
| `peers` | WireGuardPeerOptions[] | 多 Peer |
| `remote-dns-resolve` | boolean | 远端 DNS 解析 |
| `dns` | string[] | DNS |
| `refresh-server-ip-interval` | number | 刷新服务器 IP 间隔 |

### 3.12 Hysteria（`hysteria`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `ports` | string | 端口组 |
| `protocol` | string | 协议 |
| `obfs-protocol` | string | 混淆协议 |
| `up` / `up-speed` | string/number | 上行 |
| `down` / `down-speed` | string/number | 下行 |
| `auth` / `auth-str` | string | 认证 |
| `obfs` | string | 混淆 |
| `sni` | string | SNI |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `alpn` | string[] | ALPN |
| `ca` / `ca-str` | string | CA |
| `recv-window-conn` / `recv-window` | number | 接收窗口 |
| `disable-mtu-discovery` | boolean | 禁用 MTU 发现 |
| `fast-open` | boolean | Fast Open |
| `hop-interval` | number | Hop 间隔 |

### 3.13 Hysteria2（`hysteria2`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `ports` | string | 端口组 |
| `hop-interval` | number | Hop 间隔 |
| `protocol` | string | 协议 |
| `obfs-protocol` | string | 混淆协议 |
| `up` / `down` | string | 上/下行 |
| `password` | string | 密码 |
| `obfs` | string | 混淆 |
| `obfs-password` | string | 混淆密码 |
| `sni` | string | SNI |
| `skip-cert-verify` | boolean | 跳过证书校验 |
| `fingerprint` | string | TLS 指纹 |
| `alpn` | string[] | ALPN |
| `ca` / `ca-str` | string | CA |
| `cwnd` | number | 拥塞窗口 |
| `udp-mtu` | number | UDP MTU |

### 3.14 Shadowsocks（`ss`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `password` | string | 密码 |
| `cipher` | CipherType | 加密方式 |
| `udp` | boolean | UDP |
| `plugin` | `'obfs' \| 'v2ray-plugin' \| 'shadow-tls' \| 'restls'` | 插件 |
| `plugin-opts` | object | 插件参数：`mode`、`host`、`password`、`path`、`tls`、`fingerprint`、`headers`、`skip-cert-verify`、`version`、`mux`、`v2ray-http-upgrade`、`v2ray-http-upgrade-fast-open`、`version-hint`、`restls-script` |
| `udp-over-tcp` | boolean | UDP over TCP |
| `udp-over-tcp-version` | number | UDP over TCP 版本 |
| `client-fingerprint` | ClientFingerprint | 客户端指纹 |
| `smux` | boolean | Smux |

### 3.15 加密方式（`CipherType`）参考

【源码事实】包含但不限于：

```ts
'none' | 'auto' | 'dummy' | 'aes-128-gcm' | 'aes-192-gcm' | 'aes-256-gcm'
| '2022-blake3-aes-128-gcm' | '2022-blake3-aes-256-gcm'
| 'chacha20-ietf-poly1305' | '2022-blake3-chacha20-poly1305'
| 'xchacha20-ietf-poly1305' | 'aegis-128l' | 'aegis-256' | ...
```

### 3.16 Sudoku（`sudoku`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `key` | string | 密钥 |
| `aead-method` | `'chacha20-poly1305' \| 'aes-128-gcm' \| 'none'` | AEAD |
| `padding-min` / `padding-max` | number | 填充范围 |
| `table-type` | `'prefer_ascii' \| 'prefer_entropy'` | 表类型 |
| `enable-pure-downlink` | boolean | 纯下行 |
| `http-mask` | boolean | HTTP 伪装 |
| `http-mask-mode` | `'legacy' \| 'stream' \| 'poll' \| 'auto'` | HTTP 伪装模式 |
| `http-mask-tls` | boolean | HTTP 伪装 TLS |
| `http-mask-host` | string | 伪装 Host |
| `http-mask-strategy` | `'random' \| 'post' \| 'websocket'` | 伪装策略 |
| `custom-table` | string | 自定义表 |
| `custom-tables` | string[] | 自定义表组 |

### 3.17 ShadowsocksR（`ssr`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `password` | string | 密码 |
| `cipher` | CipherType | 加密 |
| `obfs` | string | 混淆 |
| `obfs-param` | string | 混淆参数 |
| `protocol` | string | 协议 |
| `protocol-param` | string | 协议参数 |
| `udp` | boolean | UDP |

当前项目已明确不纳入 SSR（无可靠链接转换参照），保留此表仅供未来评估。

### 3.18 Snell（`snell`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `server` | string | 服务器 |
| `port` | number | 端口 |
| `psk` | string | PSK |
| `udp` | boolean | UDP |
| `version` | number | 版本 |

### 3.19 Sing-Mux（`smux`）

`IProxySmuxConfig` 是一个嵌套对象：

```ts
smux?: {
  enabled?: boolean
  protocol?: 'smux' | 'yamux' | 'h2mux'
  'max-connections'?: number
  'min-streams'?: number
  'max-streams'?: number
  padding?: boolean
  statistic?: boolean
  'only-tcp'?: boolean
  'brutal-opts'?: { enabled?: boolean; up?: string; down?: string }
}
```

可用于 VLESS/VMess/SS 等协议上叠加多路复用。

### 3.20 Direct / DNS（`direct` / `dns`）

- `direct`：仅 `name` + `type: direct`。
- `dns`：仅 `name` + `type: dns`。

---

## 四、节点编辑器的导入与去重

【源码事实】`src/components/profile/proxies-editor-viewer.tsx`：

- 支持多行粘贴 URI，支持 Base64 文本自动解码（`atob` 尝试，失败则按明文）。
- 每行调用 `parseUri`；解析失败的行跳过并 `console.warn`，不阻塞。
- 使用 `names` 数组按 `name` 去重，重复节点只保留第一条。
- 可视化编辑中会过滤没有有效 `name` 的节点，避免拖拽组件崩溃；原始 YAML 仍可在高级文本模式查看和修复。
- 编辑结果保存为 `prepend` / `append` / `delete` 三个序列，存在对应的 proxies 扩展文件中。

【建议】当前项目的节点管理表单以结构化登记为主，可增加“从多行 URI/Base64 批量导入”的入口，并沿用“解析失败跳过 + 按名称去重 + 回显冲突提示”的策略。

---

## 五、对当前装配模块的改进候选

1. **扩展节点字段**：当前协议注册表的字段较为精简，可以从上面各协议字段中按需增加（尤其是 `client-fingerprint`、`reality-opts`、`grpc-opts`、`ws-opts`、`xhttp-opts`、`smux`、`ip-version`、`dialer-proxy`、`interface-name`、`routing-mark`）。
2. **增加高级通用字段**：`tfo`、`mptcp` 等不会破坏现有渲染，可作为可选开关。
3. **考虑协议扩展**：在链接转换能力允许时，可评估 `anytls`、`tuic`、`mieru`、`masque`、`sudoku`、`ssh`、`wireguard` 等协议；当前项目部分协议已有链接映射，可对照 `Node-Link-Standards.md` 补齐。
4. **批量导入节点**：可实现类似 Clash Verge Rev 的 URI 粘贴导入，减少手工录入成本。
5. **敏感字段密文存储口径**：当前项目已有 `enc:v1:` 加密与“空值保留原凭据”逻辑；Clash Verge Rev 的字段清单可作为敏感字段清单（`uuid`、`password`、`private-key`、`token`、`psk` 等）的参考。
