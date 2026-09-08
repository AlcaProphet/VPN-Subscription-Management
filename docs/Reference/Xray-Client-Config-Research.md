# Xray 客户端配置需求与字段取证

> **文档定位：** 本文是 Xray 客户端 `client.jsonc` 配置的只读研究资料，提取 `/Users/kyle/Desktop/Repo/Xray-examples` 中客户端出站配置的实际字段、组合方式和样本边界。本文只记录 Xray client JSONC 的知识与信息，不定义 vpn-sub 的代码实现、数据模型、表单方案或产品决策。
> **研究范围：** 遍历该目录下文件名或路径包含 `client` 的 JSONC 文件，包括 `client.jsonc`、`config_client.jsonc`、`client_tcp.jsonc`、`client_ws.jsonc`、`client-bypass-cn.jsonc` 以及 `All-in-One-fallbacks-Nginx/client.configs/` 下的客户端变体；不读取 `server.jsonc` / `config_server.jsonc` 作为客户端字段证据。
> **统计口径：** 样本中一个客户端文件可能含有多个 `outbounds`；统计时只计入远端代理出站，排除客户端本地入站以及 `freedom`、`blackhole`、`dns` 等非节点出站。JSONC 注释和示例模板占位符仅为结构统计做预处理，不改变原文件。
> **标注约定：** 【样本事实】= 直接来自 Xray-examples 客户端文件；【结构观察】= 对多个样本的字段结构归纳；【版本提示】= 可能随 Xray-core 版本或传输实现变化；【证据边界】= 当前样本不能证明的内容。

---

## 一、研究结论摘要

### 1.1 客户端节点的核心结构

Xray 客户端配置的远端节点主要位于顶层 `outbounds[]` 中。一个代理出站通常由以下部分组成：

```text
outbound.protocol
  ├─ settings                 协议认证与远端地址
  └─ streamSettings           传输方式与外层安全
       ├─ network
       ├─ security
       ├─ tlsSettings
       ├─ realitySettings
       └─ *Settings             当前传输方式的专用参数
```

客户端文件中的 `inbounds[]` 通常是本机 SOCKS/HTTP 监听端口，`routing` 通常是本机流量分流规则，`freedom` / `blackhole` 是本地出站。它们不属于远端节点的连接填写信息。

### 1.2 样本统计

本次共找到 55 个客户端 JSONC 文件，识别出 55 个远端代理出站，全部成功完成结构解析：

| Xray `outbound.protocol` | 客户端出站数 | 样本中的 `streamSettings.network` |
|---|---:|---|
| `vless` | 29 | `tcp`、`ws`、`grpc`、`http`、`h2`、`xhttp`、`splithttp`、`kcp` |
| `vmess` | 12 | `tcp`、`ws`、`http`、`grpc`、`kcp` |
| `trojan` | 6 | `tcp`、`ws`、`http`、`grpc` |
| `shadowsocks` | 6 | `tcp`、`ws`、`http`、`grpc`，另有 1 个未设置外层 network |
| `hysteria` | 1 | `hysteria` |
| `socks` | 1 | `tcp` |

组合数量只描述这个示例目录的覆盖情况，不代表 Xray 生态中的真实使用率，也不代表所有组合在所有 Xray-core 版本中都具有相同能力。

### 1.3 最稳定的客户端字段

【结构观察】跨样本最稳定的字段分为四组：

1. **远端地址：** `address` 与 `port`；部分协议放在 `settings` 顶层，部分协议放在 `settings.servers[]`。
2. **认证凭据：** VLESS/VMess 的 `id`，Trojan 的 `password`，Shadowsocks 的 `method` + `password`，SOCKS 的 `users[].user` + `users[].pass`，Hysteria 的 `hysteriaSettings.auth`。
3. **传输选择：** `streamSettings.network`，常见值包括 `tcp`、`ws`、`grpc`、`http`、`h2`、`xhttp`、`splithttp`、`kcp`、`hysteria`。
4. **外层安全：** `streamSettings.security`，样本中出现缺省/无安全、`tls` 和 `reality`；TLS 与 REALITY 的详细字段位于对应的设置对象中。

### 1.4 读取时必须区分的同名字段

【结构观察】Xray JSON 中不同层级可能存在同名字段，不能只按字段名判断语义：

| 字段位置 | 语义 |
|---|---|
| VMess `settings.security` | VMess 内部加密方式，例如 `none`；不是外层 TLS/REALITY 选择 |
| `streamSettings.security` | 外层传输安全方式，样本中为 `tls` 或 `reality` |
| `tlsSettings.serverName` | TLS SNI |
| `realitySettings.serverName` | REALITY 使用的 server name |
| `wsSettings.path` | WebSocket 路径 |
| `httpSettings.path` | HTTP/H2 传输路径 |
| `grpcSettings.serviceName` | gRPC 服务名称 |
| Hysteria `settings.version` | Hysteria 协议版本选择 |
| Hysteria `hysteriaSettings.version` | Hysteria 传输设置中的版本字段 |

---

## 二、客户端 JSONC 的顶层配置边界

### 2.1 `outbounds[]`：远端节点主体

客户端样本中的代理节点通常是 `outbounds[]` 中 `protocol` 为 VLESS、VMess、Trojan、Shadowsocks、Hysteria 或 SOCKS 的对象：

```json
{
  "protocol": "vless",
  "settings": {
    "address": "example.com",
    "port": 443,
    "id": "...",
    "encryption": "none"
  },
  "streamSettings": {
    "network": "tcp",
    "security": "tls",
    "tlsSettings": {
      "serverName": "example.com"
    }
  }
}
```

【样本事实】VLESS、VMess 多数使用 `settings.address` / `settings.port`；Trojan、Shadowsocks、SOCKS 多数使用 `settings.servers[0].address` / `settings.servers[0].port`。Hysteria 同时使用 `settings.address` / `settings.port` 与 `streamSettings.hysteriaSettings`。

### 2.2 `inbounds[]`：本地监听，不是远端节点字段

几乎所有完整客户端文件都包含本机监听，例如：

```json
{
  "listen": "127.0.0.1",
  "port": 1080,
  "protocol": "socks",
  "settings": {
    "udp": true
  }
}
```

【样本事实】本地入站常见为 SOCKS 或 HTTP，端口常见为 `1080`、`1081`、`10800`、`10808`、`2080`、`2081`。这些端口是客户端本地服务端口，不是远端 Xray 服务端口。

### 2.3 `routing`、`freedom`、`blackhole`：本地运行策略

【样本事实】很多客户端模板包含：

- `routing.domainStrategy`；
- 将 `geoip:private` 指向 `direct` 的规则；
- `freedom` 直连出站；
- `blackhole` 阻断出站。

这些配置控制本机客户端如何使用节点，不构成远端节点自身的协议认证、传输或安全参数。

---

## 三、按协议提取的客户端配置

### 3.1 VLESS

【样本事实】共 29 个 VLESS 客户端出站，字段覆盖最广：

- `settings.address`、`settings.port`、`settings.id`、`settings.encryption` 全部出现；
- `streamSettings.network` 全部出现；
- 外层 `streamSettings.security` 在 27 个样本中显式出现，其中 TLS 22 个、REALITY 5 个；另外 2 个样本缺省外层安全字段；
- `settings.flow` 出现在 7 个样本中；
- `realitySettings` 出现在 5 个样本中；
- WebSocket 路径出现在 4 个样本中；
- gRPC ServiceName 出现在 3 个样本中；
- HTTP/H2 路径出现在 4 个样本中；
- SplitHTTP 路径出现在 2 个样本中；
- XHTTP 路径出现在 3 个样本中；
- mKCP Seed 出现在 1 个样本中。

基础结构：

```json
{
  "protocol": "vless",
  "settings": {
    "address": "example.com",
    "port": 443,
    "id": "UUID",
    "encryption": "none"
  },
  "streamSettings": {
    "network": "tcp"
  }
}
```

【结构观察】VLESS 的稳定认证字段是 `id` 与 `encryption`。`flow` 并非所有 VLESS 节点都有，样本中的典型值为 `xtls-rprx-vision`，主要出现在 TCP + Vision / REALITY 组合。

#### VLESS + TCP

【样本事实】TCP 是 VLESS 样本最多的传输方式，共 12 个，其中包含无显式安全、TLS 和 REALITY：

- 无额外 TCP 设置时，只声明 `network: "tcp"`；
- TLS 使用 `security: "tls"` 与 `tlsSettings`；
- REALITY 使用 `security: "reality"` 与 `realitySettings`；
- Vision 样本在 `settings.flow` 中写入 `xtls-rprx-vision`。

【样本事实】少数 TCP 客户端还使用 `tcpSettings.header` 做 HTTP 请求伪装：

```json
{
  "tcpSettings": {
    "header": {
      "type": "http",
      "request": {
        "path": ["/path"],
        "method": "GET"
      }
    }
  }
}
```

该结构只在少量兼容性样本出现，不是普通 TCP VLESS 的稳定公共字段。

#### VLESS + WebSocket

【样本事实】4 个 VLESS 样本使用 `network: "ws"`，每个都包含：

```json
"wsSettings": {
  "path": "/websocket"
}
```

部分路径带有 Early Data 查询参数，例如 `/Path2WS?ed=2560`。样本还可能在 TLS 中设置 `serverName` 和 `fingerprint`。

【证据边界】本次 29 个 VLESS 客户端样本没有观察到有效的 `wsSettings.headers` 配置；不能据此认为 Xray WebSocket 客户端永远不支持请求头，只能说明当前目录的客户端样本没有使用该字段。

#### VLESS + gRPC

【样本事实】3 个 VLESS 样本使用 `network: "grpc"`，均包含：

```json
"grpcSettings": {
  "serviceName": "..."
}
```

其中 2 个样本还出现 `multiMode`；另有 1 个 REALITY gRPC 样本包含 `idle_timeout`、`health_check_timeout`、`permit_without_stream`、`initial_windows_size` 等 gRPC 扩展参数。

这些扩展参数的出现频率低于 `serviceName`，且部分位于示例注释或特殊场景配置中。

#### VLESS + HTTP / H2

【样本事实】客户端目录中同时出现 `network: "http"` 与 `network: "h2"`：

- `network: "http"` 样本使用 `httpSettings.path`，部分使用 `httpSettings.host`；
- `network: "h2"` 样本也使用 `httpSettings`，其配置目录名称为 `VLESS-H2C-Caddy`；
- All-in-One 样本中的 H2 变体同样使用 `network: "http"` 与 `httpSettings.path`。

典型结构：

```json
"httpSettings": {
  "host": ["example.com"],
  "path": "/path"
}
```

【版本提示】样本目录名、Xray `network` 值和 HTTP/2、H2C、HTTP/3 前置方式之间不是一一对应关系；应以具体 client JSONC 中的 `network` 和设置对象为准，不要只按目录名称推断传输。

#### VLESS + XHTTP

【样本事实】3 个 VLESS 样本使用 `network: "xhttp"`：

```json
"xhttpSettings": {
  "path": "/yourpath"
}
```

其中：

- 2 个样本为 XHTTP + REALITY，并使用 `realitySettings`；
- 1 个样本为 XHTTP + TLS，并设置 `alpn: ["h3"]`；
- 1 个样本显式包含 `mode: "stream-one"`；
- XHTTP3 样本通过以 `#` 开头的键保留了注释/关闭状态的 `#xmux` 和 `#downloadSettings` 示例。

`#xmux` 和 `#downloadSettings` 的键名带有 `#`，表示示例作者要求移除 `#` 后才启用，不应与当前有效的 XHTTP 配置字段混为一谈。

#### VLESS + SplitHTTP

【样本事实】2 个 VLESS 客户端使用：

```json
"network": "splithttp",
"splithttpSettings": {
  "path": "/splithttp"
}
```

其中 1 个还设置了 `host`，两个样本都使用 TLS。一个样本的 TLS ALPN 为 `h3`。

#### VLESS + mKCP

【样本事实】1 个 VLESS 客户端使用：

```json
"network": "kcp",
"kcpSettings": {
  "seed": "..."
}
```

样本目录名称使用 `mKCPSeed`，但 JSON 中的 network 值为 `kcp`。该样本没有显式外层 TLS 或 REALITY。

### 3.2 VMess

【样本事实】共 12 个 VMess 客户端出站：

- `settings.address`、`settings.port`、`settings.id`、`streamSettings.network` 全部出现；
- `settings.security` 出现在 7 个样本中，值为 VMess 内部加密方式；
- 外层 TLS 出现在 7 个样本中；
- 没有发现 VMess + REALITY 客户端样本；
- 出现 TCP、WS、HTTP、gRPC 和 mKCP。

典型结构：

```json
{
  "protocol": "vmess",
  "settings": {
    "address": "example.com",
    "port": 443,
    "id": "UUID",
    "security": "none"
  },
  "streamSettings": {
    "network": "tcp",
    "security": "tls"
  }
}
```

【结构观察】VMess 的 `settings.security` 与 `streamSettings.security` 分属不同层级：前者是 VMess 加密算法，后者是外层传输安全。部分样本省略 `settings.security`，部分显式写为 `none`。

#### VMess + TCP HTTP 伪装

【样本事实】VMess-HTTP 样本使用 `network: "tcp"`，并将 HTTP 伪装写入：

```json
"tcpSettings": {
  "header": {
    "type": "http",
    "request": {
      "version": "1.1",
      "method": "GET",
      "path": ["/path"],
      "headers": {
        "Host": ["example.com"]
      }
    }
  }
}
```

【证据边界】不能仅根据目录名 `VMess-HTTP` 把该配置归类为 `network: "http"`；该文件的有效 JSON 使用的是 TCP + `tcpSettings.header`。

#### VMess + WebSocket / gRPC / HTTP / mKCP

【样本事实】

- WebSocket 使用 `wsSettings.path`；
- gRPC 使用 `grpcSettings.serviceName`；
- HTTP 使用 `httpSettings.path`；
- mKCP 使用 `kcpSettings.seed`；
- TLS 样本可能设置 `tlsSettings.fingerprint`、`allowInsecure`、`serverName` 或 `alpn`，但这些字段并非所有 TLS 样本都同时存在。

### 3.3 Trojan

【样本事实】共 6 个 Trojan 客户端出站，全部使用外层 TLS：

```json
{
  "protocol": "trojan",
  "settings": {
    "servers": [
      {
        "address": "example.com",
        "port": 443,
        "password": "..."
      }
    ]
  },
  "streamSettings": {
    "network": "tcp",
    "security": "tls"
  }
}
```

传输分布为 TCP 2 个、gRPC 2 个、HTTP 1 个、WS 1 个。

【样本事实】按传输方式的核心字段为：

| network | 专用配置 |
|---|---|
| `tcp` | 通常无额外传输对象 |
| `ws` | `wsSettings.path` |
| `grpc` | `grpcSettings.serviceName` |
| `http` | `httpSettings.path` |

TLS 样本中 4 个显式设置 `allowInsecure`，4 个设置 TLS 指纹，只有部分样本设置 `serverName` 或 `alpn`。

【证据边界】本次 Trojan 客户端样本没有观察到 REALITY；样本不足以证明 Trojan 在其他版本或其他配置方式下不支持 REALITY。

### 3.4 Shadowsocks

【样本事实】共 6 个 Shadowsocks 客户端出站，基础认证结构均为：

```json
{
  "protocol": "shadowsocks",
  "settings": {
    "servers": [
      {
        "address": "example.com",
        "port": 443,
        "method": "chacha20-ietf-poly1305",
        "password": "..."
      }
    ]
  }
}
```

其中 5 个显式使用 `streamSettings.network`，出现 TCP、WS、gRPC 和 HTTP；4 个显式使用外层 TLS。

【样本事实】Shadowsocks 客户端样本中出现以下外层结构：

- TCP + TLS，部分带 `tcpSettings.header.type: "http"`；
- WS + TLS，使用 `wsSettings.path`；
- gRPC + TLS，使用 `grpcSettings.serviceName`；
- HTTP + TLS，使用 `httpSettings.path` 与部分 `serverName`；
- `ReverseProxy/Shadowsocks-2022/client.jsonc` 使用 `method: "2022-blake3-aes-256-gcm"`，且没有显式外层 `streamSettings`。

【证据边界】当前样本没有出现 Shadowsocks `plugin` 字段。该事实只说明 Xray-examples 的这些 client JSONC 没有使用 Xray outbound 的 plugin 结构，不代表 Shadowsocks 生态的所有客户端配置都没有插件字段。

### 3.5 Hysteria（版本 2 配置）

【样本事实】目录中的 Hysteria 客户端只有 1 个，使用：

```json
{
  "protocol": "hysteria",
  "settings": {
    "address": "server.example.com",
    "port": 443,
    "version": 2
  },
  "streamSettings": {
    "network": "hysteria",
    "security": "tls",
    "tlsSettings": {
      "serverName": "..."
    },
    "hysteriaSettings": {
      "version": 2,
      "auth": "..."
    }
  }
}
```

【结构观察】该样本在三个位置体现版本信息：

- `settings.version: 2`；
- `streamSettings.network: "hysteria"`；
- `hysteriaSettings.version: 2`。

客户端认证字段为 `hysteriaSettings.auth`。样本还使用了：

```json
"finalmask": {
  "quicParams": {
    "congestion": "bbr",
    "brutalUp": "30 mbps",
    "brutalDown": "100 mbps"
  }
}
```

`finalmask.quicParams` 属于该示例中的 QUIC 性能参数，不是所有 Hysteria 客户端连接都必然需要的字段。

### 3.6 SOCKS / SOCKS5

【样本事实】Xray-examples 的文件名为 `Socks5-TLS`，但 Xray 客户端出站协议名为 `socks`：

```json
{
  "protocol": "socks",
  "settings": {
    "servers": [
      {
        "address": "example.com",
        "port": 1234,
        "users": [
          {
            "user": "...",
            "pass": "..."
          }
        ]
      }
    ]
  },
  "streamSettings": {
    "network": "tcp",
    "security": "tls",
    "tlsSettings": {
      "serverName": "example.domain",
      "allowInsecure": false
    }
  }
}
```

【结构观察】该客户端样本同时包含：

- `servers[].address`、`servers[].port`；
- 可选的 `users[]` 用户名/密码；
- TCP；
- TLS、SNI 和证书校验策略。

【证据边界】该样本只覆盖带账号用户和 TLS 的 SOCKS 客户端出站，不能从一个文件推断无认证、无 TLS 或 UDP 组合的完整字段集合。

---

## 四、按传输方式提取的字段

### 4.1 TCP

【样本事实】TCP 是 VLESS、VMess、Trojan、Shadowsocks 和 SOCKS 中都出现的传输方式。

普通 TCP 客户端通常只写：

```json
"streamSettings": {
  "network": "tcp"
}
```

部分 VMess、VLESS 和 Shadowsocks 样本使用 `tcpSettings.header` 搭配 HTTP 请求伪装，常见字段包括：

- `header.type`；
- `header.request.version`；
- `header.request.method`；
- `header.request.path`；
- `header.request.headers`。

【结构观察】HTTP Header 伪装是 TCP 的可选扩展，不是 TCP 节点的普遍必要字段。

### 4.2 WebSocket

【样本事实】WebSocket 客户端配置使用：

```json
"network": "ws",
"wsSettings": {
  "path": "/path"
}
```

观察到的字段：

- `path`：所有当前样本中的 WS 配置均出现；
- 路径可能包含 `?ed=...` 形式的 Early Data 参数；
- 当前样本中没有有效的 `wsSettings.headers`；
- WS 常与 TLS 一起出现，也有无显式安全的 VMess/反向代理样本。

### 4.3 gRPC

【样本事实】gRPC 客户端配置使用：

```json
"network": "grpc",
"grpcSettings": {
  "serviceName": "service-name"
}
```

部分样本还包含：

- `multiMode`；
- `idle_timeout`；
- `health_check_timeout`；
- `permit_without_stream`；
- `initial_windows_size`。

其中 `serviceName` 是最稳定的 gRPC 专用字段，其余字段属于特定客户端或 CDN/连接保持场景的扩展。

### 4.4 HTTP / H2

【样本事实】HTTP/H2 客户端配置使用 `httpSettings`：

```json
"httpSettings": {
  "path": "/path",
  "host": ["example.com"]
}
```

当前样本中：

- `httpSettings.path` 在 HTTP/H2 变体中稳定出现；
- `httpSettings.host` 只在部分 VLESS 样本出现；
- `network` 既有 `http`，也有 `h2`；
- TLS SNI 可能通过 `tlsSettings.serverName` 单独设置；
- HTTP `host` 与 TLS `serverName` 是不同层级的字段，不能合并理解。

### 4.5 XHTTP

【样本事实】XHTTP 客户端配置使用：

```json
"network": "xhttp",
"xhttpSettings": {
  "path": "/path"
}
```

少数样本中还出现：

- `mode`，例如 `stream-one`；
- 以 `#xmux` 表示的关闭状态复用参数；
- 以 `#downloadSettings` 表示的关闭状态下行配置；
- 下行配置中再次嵌套 `address`、`port`、`network`、`security` 和 `xhttpSettings`。

【版本提示】XHTTP 是目录中较新的传输样本，`mode`、下行设置及复用参数可能随 Xray-core 版本变化。当前文件中的注释明确要求根据使用场景移除 `#` 后再启用这些对象。

### 4.6 SplitHTTP

【样本事实】SplitHTTP 客户端配置使用：

```json
"network": "splithttp",
"splithttpSettings": {
  "path": "/splithttp",
  "host": "example.com"
}
```

`host` 并非两个样本都出现；`path` 在两个样本中都出现。

### 4.7 mKCP

【样本事实】目录名称使用 mKCP，但有效 JSON 使用 `network: "kcp"`：

```json
"network": "kcp",
"kcpSettings": {
  "seed": "..."
}
```

本次客户端样本中稳定可观察到的是 `seed`；没有形成对 Header、伪装类型、MTU、拥塞等其他 KCP 字段的完整覆盖。

### 4.8 Hysteria

【样本事实】Hysteria 使用独立的：

```json
"network": "hysteria"
```

其认证和 QUIC 参数位于 `hysteriaSettings`、`finalmask.quicParams` 等对象中，而不是普通 TCP/WS/gRPC 传输对象。

---

## 五、按外层安全方式提取的字段

### 5.1 缺省或无显式安全

【样本事实】部分 VLESS、VMess 和 Shadowsocks 客户端只声明 `network`，不声明 `streamSettings.security`；VMess HTTP 样本也出现显式：

```json
"security": "none"
```

【结构观察】“没有 `streamSettings.security`”与“显式 `security: "none"`”在文本结构上不同。样本记录时应保留这种差异，不要在取证阶段直接合并为同一种写法。

### 5.2 TLS

【样本事实】TLS 客户端配置的基本形态为：

```json
"security": "tls",
"tlsSettings": {
  "serverName": "example.com",
  "allowInsecure": false,
  "alpn": ["h2", "http/1.1"],
  "fingerprint": "chrome"
}
```

实际样本中的 TLS 字段出现情况：

| 协议 | TLS 出站数 | 显式 `tlsSettings` | 显式 SNI | 显式 ALPN | 显式指纹 | 显式 `allowInsecure` |
|---|---:|---:|---:|---:|---:|---:|
| VLESS | 22 | 18 | 13 | 6 | 7 | 9 |
| VMess | 7 | 6 | 1 | 1 | 6 | 4 |
| Trojan | 6 | 4 | 1 | 1 | 4 | 4 |
| Shadowsocks | 4 | 4 | 1 | 1 | 4 | 4 |
| Hysteria | 1 | 1 | 1 | 0 | 0 | 0 |
| SOCKS | 1 | 1 | 1 | 0 | 0 | 1 |

【结构观察】SNI、ALPN、指纹和 `allowInsecure` 并非每个 TLS 样本都显式写出：

- SNI 有时被省略，或与 `address` 保持相同；
- ALPN 主要在 HTTP/2、HTTP/3、gRPC 或特定 TLS 示例中出现；
- `allowInsecure: false` 有时是显式写出的默认安全策略；
- `fingerprint: "chrome"` 在多个样本中出现，但并非所有 TLS 示例都显式写出。

【样本事实】VLESS TCP TLS 样本中还出现 `disableSessionResumption: true`，属于 TLS 的附加选项，不是普遍字段。

### 5.3 REALITY

【样本事实】5 个 VLESS 客户端使用 REALITY：

```json
"security": "reality",
"realitySettings": {
  "fingerprint": "chrome",
  "serverName": "example.com",
  "publicKey": "...",
  "shortId": "...",
  "spiderX": "/"
}
```

5 个 REALITY 样本均包含这些配置键：

- `serverName`；
- `publicKey`；
- `shortId`；
- `fingerprint`；
- `spiderX`。

部分模板故意将 `serverName`、`publicKey`、`shortId` 或 `spiderX` 留空，并在 JSONC 注释中说明其来源或填写方式。留空模板不能单独证明该字段在协议层面一定是可选的。

【样本事实】REALITY 客户端样本没有使用服务端私钥字段；样本中使用的是客户端侧 `publicKey`。部分 REALITY 客户端还使用：

- VLESS `settings.flow`；
- gRPC 的 `grpcSettings.serviceName`；
- XHTTP 的 `xhttpSettings.path`；
- `realitySettings.show`，但只在一个 gRPC REALITY 样本中出现。

### 5.4 Hysteria TLS

【样本事实】Hysteria 客户端使用：

```json
"security": "tls",
"tlsSettings": {
  "serverName": "..."
}
```

该样本没有显式设置 ALPN、TLS 指纹或 `allowInsecure`，但不能由单一样本推断这些字段对所有 Hysteria 客户端都不存在。

---

## 六、各协议组合覆盖矩阵

以下矩阵只列出本次客户端文件中实际出现的组合：

| 协议 | TCP | WS | gRPC | HTTP | H2 | XHTTP | SplitHTTP | mKCP/KCP | Hysteria |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| VLESS | 12 | 4 | 3 | 3 | 1 | 3 | 2 | 1 | 0 |
| VMess | 5 | 3 | 1 | 2 | 0 | 0 | 0 | 1 | 0 |
| Trojan | 2 | 1 | 2 | 1 | 0 | 0 | 0 | 0 | 0 |
| Shadowsocks | 2 | 1 | 1 | 1 | 0 | 0 | 0 | 0 | 0 |
| Hysteria | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| SOCKS | 1 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 |

【证据边界】表中的 HTTP/H2 分类按 client JSONC 的 `streamSettings.network` 字段统计。目录名称中出现的 H2、H3、H2C、HTTP 等文字不作为替代字段来源。

---

## 七、客户端文件中的典型配置路径

以下路径用于复核样本事实，值均使用示例占位符表示：

### 7.1 VLESS TCP + TLS

来源：`/Users/kyle/Desktop/Repo/Xray-examples/VLESS-TCP-TLS/config_client.jsonc`

```text
outbounds[].protocol                         = vless
outbounds[].settings.address                 = ...
outbounds[].settings.port                    = 443
outbounds[].settings.id                      = ...
outbounds[].settings.encryption              = none
outbounds[].streamSettings.network           = tcp
outbounds[].streamSettings.security          = tls
outbounds[].streamSettings.tlsSettings.serverName
outbounds[].streamSettings.tlsSettings.allowInsecure
outbounds[].streamSettings.tlsSettings.alpn
```

### 7.2 VLESS TCP + REALITY + Vision

来源：`/Users/kyle/Desktop/Repo/Xray-examples/VLESS-TCP-XTLS-Vision-REALITY/config_client.jsonc`

```text
outbounds[].settings.id
outbounds[].settings.encryption              = none
outbounds[].settings.flow                    = xtls-rprx-vision
outbounds[].streamSettings.network           = tcp
outbounds[].streamSettings.security          = reality
outbounds[].streamSettings.realitySettings.fingerprint
outbounds[].streamSettings.realitySettings.serverName
outbounds[].streamSettings.realitySettings.publicKey
outbounds[].streamSettings.realitySettings.shortId
outbounds[].streamSettings.realitySettings.spiderX
```

### 7.3 VLESS gRPC + TLS

来源：`/Users/kyle/Desktop/Repo/Xray-examples/VLESS-GRPC/client.jsonc`

```text
outbounds[].settings.id
outbounds[].settings.encryption              = none
outbounds[].streamSettings.network           = grpc
outbounds[].streamSettings.security          = tls
outbounds[].streamSettings.grpcSettings.serviceName
```

### 7.4 VLESS XHTTP + REALITY

来源：`/Users/kyle/Desktop/Repo/Xray-examples/VLESS-XHTTP-Reality/minimal-steal_others/client.jsonc`

```text
outbounds[].settings.id
outbounds[].settings.encryption              = none
outbounds[].streamSettings.network           = xhttp
outbounds[].streamSettings.xhttpSettings.path
outbounds[].streamSettings.security          = reality
outbounds[].streamSettings.realitySettings.serverName
outbounds[].streamSettings.realitySettings.publicKey
outbounds[].streamSettings.realitySettings.shortId
outbounds[].streamSettings.realitySettings.spiderX
outbounds[].streamSettings.realitySettings.fingerprint
```

### 7.5 VMess WebSocket + TLS

来源：`/Users/kyle/Desktop/Repo/Xray-examples/VMess-Websocket-TLS/config_client.jsonc`

```text
outbounds[].protocol                         = vmess
outbounds[].settings.address
outbounds[].settings.port
outbounds[].settings.id
outbounds[].settings.security
outbounds[].streamSettings.network           = ws
outbounds[].streamSettings.security          = tls
outbounds[].streamSettings.wsSettings.path
outbounds[].streamSettings.tlsSettings.fingerprint
```

### 7.6 Trojan gRPC + TLS

来源：`/Users/kyle/Desktop/Repo/Xray-examples/Trojan-gRPC-Caddy2／Nginx/client.jsonc`

```text
outbounds[].protocol                         = trojan
outbounds[].settings.servers[0].address
outbounds[].settings.servers[0].port
outbounds[].settings.servers[0].password
outbounds[].streamSettings.network           = grpc
outbounds[].streamSettings.security          = tls
outbounds[].streamSettings.grpcSettings.serviceName
```

### 7.7 Shadowsocks WS + TLS

来源：`/Users/kyle/Desktop/Repo/Xray-examples/All-in-One-fallbacks-Nginx/client.configs/ShadowSocks-WS-TLS.jsonc`

```text
outbounds[].protocol                         = shadowsocks
outbounds[].settings.servers[0].address
outbounds[].settings.servers[0].port
outbounds[].settings.servers[0].method
outbounds[].settings.servers[0].password
outbounds[].streamSettings.network           = ws
outbounds[].streamSettings.security          = tls
outbounds[].streamSettings.wsSettings.path
```

### 7.8 Hysteria version 2 + TLS

来源：`/Users/kyle/Desktop/Repo/Xray-examples/Hysteria2/client.jsonc`

```text
outbounds[].protocol                         = hysteria
outbounds[].settings.address
outbounds[].settings.port
outbounds[].settings.version                 = 2
outbounds[].streamSettings.network           = hysteria
outbounds[].streamSettings.security          = tls
outbounds[].streamSettings.tlsSettings.serverName
outbounds[].streamSettings.hysteriaSettings.version = 2
outbounds[].streamSettings.hysteriaSettings.auth
```

### 7.9 SOCKS + TLS

来源：`/Users/kyle/Desktop/Repo/Xray-examples/Socks5-TLS/config_client.jsonc`

```text
outbounds[].protocol                         = socks
outbounds[].settings.servers[0].address
outbounds[].settings.servers[0].port
outbounds[].settings.servers[0].users[0].user
outbounds[].settings.servers[0].users[0].pass
outbounds[].streamSettings.network           = tcp
outbounds[].streamSettings.security          = tls
outbounds[].streamSettings.tlsSettings.serverName
outbounds[].streamSettings.tlsSettings.allowInsecure
```

---

## 八、样本阅读中的版本与语义边界

### 8.1 空值模板不等于字段可选

很多客户端文件为了让用户填写而保留空字符串，例如：

- VLESS/VMess `settings.id`；
- VLESS `realitySettings.publicKey`、`shortId`；
- gRPC `serviceName`；
- XHTTP `path`；
- 地址、SNI 和证书相关字段。

【证据边界】空值只表示示例模板尚未填入真实部署值。应结合配置所在对象、注释和服务端配套配置判断用途，不能把所有空字符串都归类为“可省略”。

### 8.2 缺省、显式 `none` 和显式对象需要分别记录

客户端样本同时存在：

- 不写 `streamSettings.security`；
- 写 `streamSettings.security: "none"`；
- 写 `streamSettings.security: "tls"`；
- 写 `streamSettings.security: "reality"`。

同样，TLS 设置对象有时整体省略，有时只写 `serverName`，有时还写 `alpn`、`fingerprint` 或 `allowInsecure`。研究记录应保留键是否存在和键值是否为空这两层信息。

### 8.3 目录名称不是最终字段语义

本目录存在以下命名与有效 JSON 可能不完全相同的情况：

- `VMess-HTTP` 使用 `network: "tcp"` + `tcpSettings.header`；
- `VMess-HTTP2` 使用 `network: "http"`；
- `VLESS-H2C-Caddy` 使用 `network: "h2"` + `httpSettings`；
- `VLESS-TLS-SplitHTTP-*` 使用 `network: "splithttp"`；
- `VLESS-mKCPSeed` 使用 `network: "kcp"`；
- 文件名 `Socks5-TLS` 对应 Xray `outbound.protocol: "socks"`。

【结构观察】应以 JSON 中实际的 `protocol`、`streamSettings.network`、`streamSettings.security` 和专用设置对象为准。

### 8.4 新旧 Xray 版本样本混合

【版本提示】目录内既有较早的基础 TCP/WS/gRPC 示例，也有 XHTTP、REALITY、HTTP/3 相关的新式示例。README 中的版本说明也不一致：例如 All-in-One 示例注明曾以 Xray 1.7.2 测试，而 XHTTP REALITY 示例要求较新的 Xray-core 版本。

因此：

- 旧传输字段不能直接推断新传输字段的完整能力；
- 新传输字段的样本不能反向证明旧版本支持；
- `mode`、XHTTP 下行设置、REALITY 默认值等内容应结合对应 Xray-core 版本文档核对。

### 8.5 客户端与服务端字段属于不同配置面

本文只提取客户端出站字段。服务端的监听地址、证书/私钥文件、REALITY 私钥、fallback、入站账号列表、路由和出站不属于本文的客户端字段集合。

客户端样本中可看到的远端连接字段，通常是服务端配置的对应连接侧参数，例如：

- 服务端账号对应客户端 UUID、密码或认证字符串；
- 服务端传输路径对应客户端 `path`；
- 服务端 gRPC service name 对应客户端 `serviceName`；
- 服务端 REALITY 公私钥配置对应客户端侧的公钥及连接参数。

本文不进一步定义这些字段在其他项目中的存储、转换或校验方式。

---

## 九、样本索引

### 9.1 VLESS

- `VLESS-TCP/config_client.jsonc`
- `VLESS-TCP-TLS/config_client.jsonc`
- `VLESS-TCP-REALITY (without being stolen)/config_client.jsonc`
- `VLESS-TCP-XTLS-Vision/config_client.jsonc`
- `VLESS-TCP-XTLS-Vision-REALITY/config_client.jsonc`
- `VLESS-GRPC/client.jsonc`
- `VLESS-gRPC-REALITY/config_client.jsonc`
- `VLESS-WSS-Nginx/client.jsonc`
- `VLESS-TLS-SplitHTTP-CaddyNginx/client.jsonc`
- `VLESS-TLS-SplitHTTP-H3/client.jsonc`
- `VLESS-XHTTP-Reality/minimal-steal_others/client.jsonc`
- `VLESS-XHTTP3-Nginx/client.jsonc`
- `VLESS-mKCPSeed/config_client.jsonc`
- `VLESS-HTTP-Caddy/` 下的 H2C、H3、H3-to-H2C 客户端文件
- `All-in-One-fallbacks-Nginx/client.configs/` 下的 VLESS 变体
- `ReverseProxy/VLESS-TCP-XTLS-WS/` 下的 TCP、WS 客户端文件

### 9.2 VMess

- `VMess-TCP/config_client.jsonc`
- `VMess-TCP-TLS/config_client.jsonc`
- `VMess-HTTP/config_client.jsonc`
- `VMess-HTTP2/config_client.jsonc`
- `VMess-Websocket/config_client.jsonc`
- `VMess-Websocket-TLS/config_client.jsonc`
- `VMess-mKCPSeed/config_client.jsonc`
- `All-in-One-fallbacks-Nginx/client.configs/` 下的 VMess 变体
- `ReverseProxy/Vmess-TCP/client.jsonc`

### 9.3 Trojan、Shadowsocks、Hysteria、SOCKS

- `Trojan-TCP-TLS (minimal)/config_client.jsonc`
- `Trojan-gRPC-Caddy2／Nginx/client.jsonc`
- `All-in-One-fallbacks-Nginx/client.configs/` 下的 Trojan 变体
- `Shadowsocks-TCP/client.jsonc`
- `ReverseProxy/Shadowsocks-2022/client.jsonc`
- `All-in-One-fallbacks-Nginx/client.configs/` 下的 Shadowsocks 变体
- `Hysteria2/client.jsonc`
- `Socks5-TLS/config_client.jsonc`

---

## 十、研究依据与变更记录

### 10.1 研究依据

- 外部样本目录：`/Users/kyle/Desktop/Repo/Xray-examples/`；
- 客户端文件：所有符合本文范围的 `client*.jsonc` 与 `client.configs/*.jsonc`；
- 配套 README：各协议目录中的 README，用于理解样本注释、前置代理和版本说明；
- Xray client JSON 的字段层级：以每个客户端文件的 `outbounds[].settings`、`outbounds[].streamSettings` 及其子对象为直接依据。

### 10.2 变更记录

| 日期 | 说明 |
|---|---|
| 2026-09-02 | 新建：提取 Xray-examples 客户端 JSONC，记录 VLESS、VMess、Trojan、Shadowsocks、Hysteria 和 SOCKS 的认证、传输、安全字段、组合覆盖和版本语义边界。 |
