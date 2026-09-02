# Node-Editor-Design-Research.md — 节点编辑器分层与多目标适配研究汇总

> **文档定位：** 本文汇总节点管理模块的节点编辑器改进研究，承接 [xray-server-side.md](xray-server-side.md)、[xray-client-side.md](xray-client-side.md)、[Xray-Core-API.md](Xray-Core-API.md)、[Node-Link-Standards.md](Node-Link-Standards.md) 和 [Issue12.md](../../Issue12.md) 的相关证据。本文用于后续进一步研究，不是 Design/Build 文档，不直接定义代码实现，也不代表最终产品决策。
> **研究状态：** 2026-09-02；用户已确认研究方向，但协议范围、数据契约、输出器兼容矩阵和具体校验强度仍需后续分析与定稿。
> **标注约定：** 【项目事实】= 当前仓库代码或既有参考资料；【样本事实】= Xray-examples 中的实际配置；【外部事实】= 官方文档或公开项目/Issue；【候选方案】= 供后续讨论的设计方向；【待确认】= 尚未形成最终决策。

---

## 一、用户已确认的研究方向

以下内容是本轮研究的范围约束，不等同于已经进入 Design 或 Build：

| 方向 | 已确认内容 | 对后续研究的约束 |
|---|---|---|
| Xray 输出 | 可以增加独立的 Xray target adapter | 不把 Xray 客户端 JSON 直接混入当前 Mihomo `protocol_json` 编辑模型 |
| 隐藏值 | 保留隐藏值，只输出当前激活值 | 编辑器需要区分“保存的数据”和“当前目标输出的数据” |
| 协议范围 | 需要拓展到更多协议 | 不能只为 VLESS/VMess 设计一次性表单；应提取可复用的条件字段模型 |
| 复杂扩展 | 先进入高级区或兼容区 | XHTTP 扩展、KCP、SplitHTTP、SMux、TCP 伪装等不占用首屏空间 |
| 文档用途 | 整合后存入 `docs/Reference/`，待进一步分析 | 本文不改代码，不把候选方案标记为已实现或最终设计 |

## 二、研究结论摘要

### 2.1 推荐的总体编辑模型

节点编辑器应由静态“协议字段全集”改为状态驱动的条件编辑器：

```text
协议
  → 协议核心凭据
  → 传输方式
      → 当前传输参数
  → 外层安全方式
      → TLS / REALITY 参数
  → 常用开关
  → 高级兼容参数 / 未知键 / 高级 JSON
```

首屏重点是“当前节点怎样连接”，不是“某协议全部可能的字段”。核心信息与当前组合的必填字段始终可见，非当前组合字段保留在数据中但不显示、不校验、不输出。

### 2.2 不应把服务端配置当成节点编辑模型

Xray 服务端 `inbounds` 中的监听、证书文件、Reality 私钥、`target/dest`、fallback、sniffing、routing 和 outbounds 只说明服务端如何提供服务；节点编辑器需要的是远端客户端连接描述。服务端字段必须先转换为客户端语义，不能照搬 JSON 层级。

这条边界已经在 [xray-server-side.md](xray-server-side.md) 中取证：服务端账号可以映射为 UUID/密码，传输对象可以映射为项目的客户端侧对象，但证书、私钥和 fallback 等不进入普通节点字段。

### 2.3 Xray、Mihomo 和项目输出是不同目标

当前项目手工节点保存的是 Mihomo/Clash 风格字段，并由不同输出器处理。Xray 客户端资料中的 `settings`、`streamSettings`、`tlsSettings`、`realitySettings` 等结构只应作为语义参考。

候选的数据流如下：

```text
用户编辑语义
  ├─ Mihomo/Clash target adapter → Clash 原生代理对象
  ├─ Shadowrocket/generic adapter → URI 或订阅链接
  └─ Xray target adapter → Xray 客户端 outbound
```

当前 `protocol_json` 可以继续作为兼容存储，但新的编辑元数据、条件规则和目标能力不能只靠 Mihomo 字段名推断。

---

## 三、当前项目实现审计

### 3.1 已有基础

【项目事实】当前节点编辑器已经具备以下可复用基础：

- 节点编辑使用桌面 920px Modal，移动端使用全屏 Drawer。
- 协议字段由后端注册表下发，而不是前端硬编码完整协议字段名单。
- 对象支持固定属性、Map、对象数组和对象级高级 JSON。
- 敏感字段编辑时留空可以保留原凭据。
- `protocol_json` 已被 Clash、Shadowrocket 和 generic 链路使用。

参考：[NodesView.vue](../../frontend/src/views/admin/NodesView.vue)、[ProtocolFieldEditor.vue](../../frontend/src/components/ProtocolFieldEditor.vue)、[registry.go](../../backend/internal/node/registry.go)。

### 3.2 主要缺口

当前表单已有六个逻辑分区，但 `FormSection` 本身是静态 section；除最末尾高级参数外，协议字段会按照 `section` 连续渲染。`FieldSchema` 主要表达静态 `required`、`section`、类型和对象结构，后端验证也主要执行静态必填与基本类型检查。

因此目前无法可靠表达：

- `network=ws` 才显示 WebSocket 参数；
- `network=grpc` 才要求 ServiceName；
- `security=reality` 才要求公钥和 Short ID；
- 只在 TLS 下显示 SNI、ALPN 和证书校验设置；
- 某参数只适用于某个输出目标；
- 隐藏值保留后不参与当前目标输出。

参考：[node.ts](../../frontend/src/api/node.ts)、[node.go](../../backend/internal/node/node.go)、[NodesView.vue](../../frontend/src/views/admin/NodesView.vue)。

### 3.3 当前存储和输出的语义风险

1. 当前安全状态由 `tls: boolean` 和 `reality-opts` 间接表达，没有显式的无安全/TLS/Reality 三态。
2. Clash 渲染器目前会把 `protocol_json` 中的字段整体写入代理对象。单纯在 UI 隐藏字段，不能阻止旧的 WS、TLS 或 Reality 参数进入产物。
3. 当前 VLESS schema 同时存在 `ws-opts` 和顶层 `ws-path/ws-headers`，但链接输出主要读取 `ws-opts`，需要确定规范路径。
4. 嵌套对象编辑器能够提示并保留未知键，但后端编辑保存会按照新协议顶层 schema 重建对象，顶层未知键的保留语义仍需补充。
5. 当前 `updateProtocol()` 切换协议会清空前端 `protocol_json`；后端更新时也只保留新协议 schema 声明的顶层字段。协议切换与同一协议内部的传输/安全切换应分开处理。

相关代码：[registry.go](../../backend/internal/node/registry.go)、[render_clash.go](../../backend/internal/assembly/render_clash.go)、[links.go](../../backend/internal/assembly/links/links.go)。

---

## 四、Xray 资料与外部资料的共同结论

### 4.1 Xray 客户端的稳定分层

【样本事实】55 个客户端文件中识别出 55 个远端代理出站，其中 VLESS 29 个、VMess 12 个。跨协议最稳定的连接字段可以归纳为：

1. 远端地址和端口；
2. 协议认证凭据；
3. 传输方式；
4. 外层安全方式；
5. 当前传输方式的专用参数。

Xray 客户端资料中还明确区分了 VMess 内部 `settings.security` 与外层 `streamSettings.security`，不能把两者都翻译成一个“加密方式”字段。[xray-client-side.md](xray-client-side.md)

### 4.2 Xray 当前传输兼容性

【外部事实】Xray 当前传输文档将传输方法和传输安全分开，并给出了以下兼容关系：

| Xray 传输方法 | 无安全 | TLS | Reality |
|---|---:|---:|---:|
| RAW/TCP | 支持 | 支持 | 支持 |
| XHTTP | 支持 | 支持 | 支持 |
| gRPC | 支持 | 支持 | 支持 |
| WebSocket | 支持 | 支持 | 不支持 |
| HTTPUpgrade | 支持 | 支持 | 不支持 |
| mKCP | 支持 | 支持 | 不支持 |
| Hysteria | 不支持 | 必须 | 不支持 |

参考：[Xray Transport Configuration](https://xtls.github.io/en/config/transport.html)。

该表只能作为 Xray target adapter 的输入，不能直接作为 Mihomo、Shadowrocket 或 generic 的全量能力表。目标客户端和版本需要分别验证。

### 4.3 客户端和服务端字段不能混用

Reality 客户端需要 SNI、公钥、Short ID、客户端指纹和可选的 `spiderX`；服务端的私钥、`target/dest`、`serverNames` 和 `shortIds` 列表不应进入普通客户端编辑器。

Xray 当前文档将 Reality 客户端公钥称为 `password`，旧样本中仍可能出现 `publicKey`；Mihomo 使用 `reality-opts.public-key`。因此 UI 应使用稳定的语义标签“Reality 公钥”，由不同 target adapter 负责字段映射。[Xray REALITY](https://xtls.github.io/en/config/transports/reality.html)

### 4.4 实际项目验证了条件表单方向

开源 3x-ui 的 Xray 入站表单使用协议/传输能力判断、当前传输子表单、安全方式 Radio、TLS/Reality 条件面板和独立高级区域。它编辑的是服务端入站，不应直接复制字段，但可以作为“只展示当前组合”的交互案例。[3x-ui InboundFormModal.tsx](https://github.com/MHSanaei/3x-ui/blob/main/frontend/src/pages/inbounds/form/InboundFormModal.tsx)

Mihomo issue #2533 还记录了 VLESS + gRPC 在缺少 `grpc-service-name` 时连接失败、补充 ServiceName 后恢复的实际案例。该 Issue 不是协议规范，但足以支持在 Mihomo target 下对 ServiceName 做强提示或条件必填。[Mihomo issue #2533](https://github.com/MetaCubeX/mihomo/issues/2533)

---

## 五、候选 UI 信息架构

### 5.1 页面层级

沿用 Issue12 的分区卡片、结构化编辑、分区级高级 JSON 和未知键提示，但节点表单只让基础区与当前连接区默认展开：

```text
节点编辑
├─ 当前组合摘要                         始终可见
│   └─ VLESS · TCP · REALITY
│
├─ 基础信息                              默认展开
│   ├─ 协议
│   ├─ 节点名称
│   ├─ 服务器
│   ├─ 端口
│   └─ 当前协议核心凭据
│
├─ 连接方式                              默认展开
│   ├─ 传输方式
│   ├─ 外层安全方式
│   ├─ 当前传输的必填参数
│   ├─ 当前安全方式的常用参数
│   └─ 当前区的常用开关
│
├─ 兼容与性能参数                        默认折叠
│   ├─ TCP HTTP 伪装、Early Data、HTTP Upgrade
│   ├─ XHTTP 扩展、KCP、SplitHTTP
│   ├─ SMux、TFO、MPTCP
│   └─ 协议特有的旧版或目标相关参数
│
└─ 高级 JSON                            默认折叠
    ├─ 当前分区高级 JSON
    └─ 完整 protocol_json / target JSON
```

“基础信息”与“连接方式”默认展开，是因为用户需要先看到节点身份和当前连接组合；高级区遵循 Issue12 的默认折叠方向。参考：[Issue12.md](../../Issue12.md)。

### 5.2 开关的位置

不建议把所有 `bool` 字段继续集中成一个跨协议的大开关区。可以保留统一的 Switch 样式，但按语义放回所属分区：

| 开关 | 候选位置 |
|---|---|
| UDP | 连接方式中的常用开关 |
| 跳过证书校验 | TLS/Reality 的证书校验子区，并显示风险提示 |
| TLS/Reality 选择 | 安全方式 Radio/Segmented，不作为普通 Switch |
| TFO、MPTCP | 兼容与性能区 |
| HTTP Upgrade | WS 或传输扩展区 |
| XHTTP padding/mux | XHTTP 高级区 |

这样仍遵循“区内集中开关”的 Issue12 方向，但避免用户在全局开关区寻找只对当前安全方式有效的参数。

### 5.3 结构化表格

普通字段优先采用“参数 / 值 / 说明”的结构化行：

| 参数 | 控件 | 默认展示 |
|---|---|---|
| UUID、密码 | 密码输入 | 编辑时留空保留原凭据 |
| 传输方式 | Select 或 Segmented | 只显示当前传输对象 |
| 外层安全 | Radio/Segmented | 无 / TLS / Reality |
| Path | 文本输入 | WS、H2、HTTP、XHTTP 条件显示 |
| Host | 文本或列表 | 与 TLS SNI 分开 |
| ALPN | 可增删列表 | 不长期依赖逗号分隔文本 |
| Headers | Key/Value 表格 | 可增删、保留未知键 |
| Boolean | 右侧 Switch | 归属当前分区 |

对象的结构化编辑应继续支持固定属性、Map 和对象数组；对象级 JSON 可以保留作为局部逃生舱。高级 JSON 需要明确作用域，不能让多个编辑器互相无提示地覆盖。

### 5.4 当前组合摘要和目标提示

表单顶部可以显示：

```text
VLESS · TCP · REALITY
Clash/Mihomo：可输出
Shadowrocket：部分字段可表达
Xray：可由 Xray target adapter 转换
```

普通用户只看到必要的警告；详细的目标字段差异放在高级区或输出预览中，避免把兼容矩阵本身变成新的编辑负担。

---

## 六、VLESS 与 VMess 候选表单

### 6.1 VLESS

VLESS 首屏候选字段：

| 区域 | 字段 | 处理方式 |
|---|---|---|
| 基础信息 | UUID | 直接展示；创建时必填；编辑时空值保留旧凭据 |
| 连接方式 | TCP、WS、gRPC、H2、HTTP、XHTTP | 选择后只显示当前传输参数 |
| 安全方式 | 无、TLS、Reality | UI 三态；映射到现有 `tls` 与 `reality-opts` |
| 常用参数 | UDP、Flow | Flow 只在适用组合出现 |
| TLS | SNI、ALPN、客户端指纹 | TLS 选择后显示 |
| Reality | SNI、公钥、Short ID、客户端指纹 | Reality 选择后显示 |
| 高级 | `encryption`、TCP 伪装、Early Data、XHTTP 扩展、SMux | 默认折叠 |

传输专用字段候选：

| 当前传输 | 常用字段 | 条件校验候选 |
|---|---|---|
| TCP | 无 | 无；TCP HTTP 伪装进入高级区 |
| WS | Path、Host | Path 缺失时提示或阻止 |
| gRPC | ServiceName | Mihomo target 下强提示或条件必填 |
| H2 | Path、Host | Path 缺失时提示 |
| HTTP | Path 列表、Host、Method | 至少一个有效 Path |
| XHTTP | Path、Host、Mode | Path 直接展示；复杂扩展折叠 |

VLESS 的 `encryption` 当前项目默认为 `none`，generic 链接也固定输出 `encryption=none`。因此不建议把它作为普通自由文本字段；候选做法是显示有效值但放入高级区，未来若支持 VLESS Encryption，再设计独立的高级能力。

Xray 官方还提示，未启用外层安全的 VLESS 通常只适合私有地址/受信网络，或已启用 VLESS Encryption 的场景。[Xray VLESS outbound](https://xtls.github.io/en/config/outbounds/vless.html)

### 6.2 VMess

VMess 首屏候选字段：

| 区域 | 字段 | 处理方式 |
|---|---|---|
| 基础信息 | UUID | 直接展示；创建时必填 |
| 常用兼容 | Cipher | 默认 `auto`，可放常用层 |
| 连接方式 | TCP、WS、gRPC、H2、HTTP | 按当前传输展示参数 |
| 安全方式 | 无、TLS | Reality 是否首屏显示由目标能力决定 |
| TLS | SNI、ALPN、客户端指纹 | TLS 选择后显示 |
| 常用开关 | UDP | 连接区显示 |
| 高级兼容 | AlterId、TCP HTTP 伪装、Padding、SMux | 默认折叠 |

VMess 的内部 Cipher 与外层 TLS/Reality 必须用不同标签。当前 Xray outbound 文档的基本配置不把 `alterId`作为现代核心字段，但项目的 Mihomo 模板和 generic VMess 链接仍使用 `alterId=0`，因此项目应保留它但标注为兼容字段。[Xray VMess outbound](https://xtls.github.io/en/config/outbounds/vmess.html)

### 6.3 个人模板预设

个人模板适合作为明确命名的快速预设，不应变成所有节点的协议默认值：

| 预设 | 建议预填 | 说明 |
|---|---|---|
| VLESS TCP + Reality + Vision | 443、TCP、Reality、UDP、`xtls-rprx-vision`、`client-fingerprint=random`、SNI、公钥、Short ID | 对应个人模板中的常用 VLESS 节点 |
| VMess TCP 基础兼容 | 1234、`cipher=auto`、`alterId=0`、UDP、TCP、无外层 TLS | 对应个人模板中的 VMess 备用节点 |

应区分“界面显示默认值”和“实际写入 `protocol_json` 的值”。用户未使用预设或未修改字段时，可以保留省略语义，避免把个人偏好强制写入所有节点。

参考：[Clash.yaml.template.md](../DocTemplates/Clash.yaml.template.md)。

---

## 七、更多协议的扩展策略

用户已确认需要拓展到更多协议，但不建议把所有协议强行套进 VLESS/VMess 的“传输 + 外层安全”模型。候选按语义分组：

| 协议组 | 首屏模型 | 高级区内容 |
|---|---|---|
| VLESS、VMess、Trojan | 协议凭据 → 传输 → 外层安全 → 当前传输参数 | Flow、TCP 伪装、SMux、版本扩展 |
| Shadowsocks | Cipher/密码 → 插件选择 → 插件参数 | UDP over TCP、SMux、插件全部扩展 |
| Hysteria、Hysteria2、TUIC | 协议认证 → UDP/QUIC 特有参数 → TLS/SNI | 窗口、MTU、拥塞、Hop、心跳 |
| WireGuard、MASQUE | 密钥和隧道地址 → Peer/Allowed IPs | Reserved、PSK、MTU、DNS、Worker |
| HTTP、SOCKS5、SSH | 认证模式 → 地址/端口 → TLS 或 Host Key | 认证扩展、证书、Host Key 算法 |
| OpenVPN | 客户端配置全文 | 解析出的辅助字段和未知扩展 |
| Snell、Mieru、ShadowQUIC、TrustTunnel、Tailscale | 各自核心凭据 | 仅展示已有注册表能够证明的扩展 |

Xray-examples 没有覆盖当前项目所有 19 个手工协议。未被 Xray 样本覆盖的协议应继续以项目注册表、Mihomo 客户端字段和各自权威资料为证据，不应从 VLESS/VMess 推断安全、传输或必填关系。参考：[xray-server-side.md](xray-server-side.md) §3.2。

### 7.1 Shadowsocks 的特殊边界

不能仅因为 Xray 服务端样本存在 Shadowsocks + WS/gRPC/H2，就把这些选项直接加入项目 Shadowsocks 的普通传输选择。项目当前主要通过 `plugin`/`plugin-opts` 表达插件型连接，SR/generic 输出也有兼容限制。插件应当是条件子表单，而不是复用 VLESS 的 Reality/Flow 字段。

### 7.2 Hysteria 与 Hysteria2 的特殊边界

Xray 样本中的 `protocol=hysteria` 与 `version=2`，不能直接等价为项目的 `hysteria2`。两个协议应保持不同入口和 schema；版本映射、认证字段和输出目标需要另行确认。

### 7.3 配置全文型协议

OpenVPN 等无法用少量通用字段安全表达的协议，应优先提供专用文本/JSON 编辑器和语法校验，不为了“表单完整”伪造一组不完整的字段。

---

## 八、条件元数据与校验模型

### 8.1 FieldSchema 的候选扩展

后续设计可以在现有 `FieldSchema` 基础上增加以下概念，不要求现在确定具体 JSON 命名：

| 元数据 | 作用 |
|---|---|
| `visible_if` | 当前协议、传输、安全或认证状态满足时显示 |
| `required_if` | 当前状态满足时变为必填 |
| `warning_if` | 产生可见的兼容性或安全提示 |
| `default_expanded` | 区或字段首次进入时是否展开 |
| `target_support` | Clash、SR、generic、Xray 的支持/转换/部分支持状态 |
| `canonical_path` | 规范存储路径，例如 VLESS 统一使用 `ws-opts.path` |
| `legacy_paths` | 旧字段兼容读取路径 |
| `preserve_when_hidden` | 隐藏时是否继续保存，默认候选为是 |

条件示例：

```text
ws-opts                 visible_if network == ws
ws-opts.path            required_if network == ws

grpc-opts               visible_if network == grpc
grpc-opts.service-name  required_if network == grpc

TLS fields             visible_if security == tls
Reality fields         visible_if security == reality

VLESS flow             visible_if protocol == vless
                        且当前 network/security 属于适用组合
```

前端显示、后端校验、目标输出过滤应共用同一套条件元数据，避免“前端隐藏但后端仍按静态规则拒绝”或“界面允许保存但输出器无法表达”。

### 8.2 安全方式的虚拟字段

候选在编辑器内引入虚拟状态：

```text
security = none | tls | reality
```

初步可以不新增数据库字段，而映射到当前存储：

| UI 状态 | 当前 Mihomo 存储候选 |
|---|---|
| `none` | `tls=false`；Reality 不参与当前输出 |
| `tls` | `tls=true`；Reality 不参与当前输出 |
| `reality` | `tls=true`；启用 `reality-opts` |

如果未来 Xray target adapter 需要明确表达“省略 security”和显式 `security=none` 的差别，该差别应由 Xray adapter 的输出策略处理，不要为了复刻 Xray JSON 而把 `streamSettings.security` 写进现有 Mihomo 存储。

### 8.3 必填、警告和目标限制

| 类型 | 示例 | 候选行为 |
|---|---|---|
| 绝对必填 | UUID、密码、WireGuard 私钥 | 阻止保存 |
| 当前组合必填 | WS Path、gRPC ServiceName、Reality 公钥/Short ID | 根据目标能力阻止或强提示 |
| 危险项 | 跳过证书校验 | 警告，默认关闭 |
| 连接风险 | 公网 VLESS 无 TLS/Reality | 警告，不必一律禁止 |
| 目标差异 | SR/generic 无法表达某复杂字段 | 保存可允许，但输出预览明确提示 |
| 版本差异 | XHTTP 新 Mode、Reality 新参数 | 高级提示或 target-specific 警告 |
| 非当前组合旧值 | 旧 WS、TLS 或 KCP 对象 | 保留，不参与当前校验和输出 |

编辑时敏感字段的“空值=保留旧密文”语义必须继续存在，且要与 `required_if` 结合：创建时空值不允许，编辑时空值可以保留同路径旧凭据。

---

## 九、隐藏值保留与激活输出

用户已确认采用：

```text
保留隐藏值
  → 当前编辑恢复时可重新显示
  → 当前校验只检查激活字段
  → 当前 target adapter 只输出激活字段
```

这与“切换即清空”的简单方案相比，更适合多次尝试 TLS、Reality、WS 和 gRPC 参数，也更有利于未来扩展字段兼容。但它需要所有输出器都支持 active-field filter：

| 场景 | 数据保存 | 当前输出 |
|---|---|---|
| VLESS 从 Reality 切换到 TLS | 保留 Reality 对象 | 只输出 TLS 字段 |
| WS 切换到 gRPC | 保留 WS Path/Host | 只输出 gRPC ServiceName |
| 切回原传输 | 恢复原来的隐藏参数 | 再次输出当前激活字段 |
| 未知高级键 | 保留 | 只有目标能力允许且当前区激活时输出 |

需要特别区分两种切换：

1. 同一协议内切换传输/安全：候选采用保留隐藏值；
2. 切换协议：当前实现倾向于重置不兼容字段，但是否保留一个可回退的编辑草稿、是否提示用户确认，仍待确认。

### 9.1 存储规范和旧字段迁移

应为每类常用对象确定一个规范路径：

- WebSocket：优先 `ws-opts`；旧 `ws-path/ws-headers` 只兼容读取；
- gRPC：优先 `grpc-opts.grpc-service-name`；
- H2/HTTP：保留项目当前 `h2-opts` 与 `http-opts` 的区别；
- Reality：保留 Mihomo `reality-opts`，由 Xray adapter 转换为 `realitySettings`；
- XHTTP：扩展字段进入对象高级区，不复制为多个顶层字段。

旧字段是否在保存时自动归一化，或只在读取时兼容，属于后续迁移设计，不在本文中决定。

---

## 十、独立 Xray target adapter 候选设计

### 10.1 适配器职责

独立 Xray target adapter 不负责改变节点编辑器的用户体验，主要负责：

- 把项目节点的协议、地址、凭据、传输和安全语义转换为 Xray client outbound；
- 将项目 Mihomo 字段映射为 Xray 的 `settings`、`streamSettings` 和专用 `*Settings`；
- 根据 Xray 目标版本选择字段名和兼容策略；
- 对不能表达的字段返回“转换、部分支持或不支持”的回执；
- 只读取当前激活组合的字段；
- 不把服务端私钥、证书文件和 `target/dest` 误当成客户端字段。

### 10.2 目标能力矩阵

现有 `LinkMapping` 只有 SR/generic 布尔值和参数名称，不足以描述多目标状态。候选能力模型需要至少区分：

```text
支持原生
等价转换
部分转换
目标不支持
仅高级/版本相关
```

例如：

| 字段 | Mihomo/Clash | Shadowrocket/generic | Xray client |
|---|---|---|---|
| VLESS UUID | 原生 | URI 凭据 | Xray outbound user id |
| `network=ws` | `network: ws` + `ws-opts` | `type=ws` + path/host | `network=websocket` + `wsSettings` |
| `tls` | `tls: true` | `security=tls` 或目标原生参数 | `security=tls` + `tlsSettings` |
| Reality 公钥 | `reality-opts.public-key` | `pbk` | Xray client Reality 公钥字段 |
| XHTTP 扩展 | 依 Mihomo 版本 | 可能部分支持 | 依 Xray 版本和目标 schema |
| SMux | Mihomo 对象 | 多数 URI 不可完整表达 | 需要单独 Xray 出站映射 |

该矩阵应由中央能力注册表或目标适配器提供，不能让前端只根据字段是否出现在 `form_schema` 判断“可导出”。

### 10.3 Xray target adapter 的版本边界

本地 Xray-examples 同时包含早期 TCP/WS/gRPC 配置和较新的 XHTTP/Reality/HTTP3 配置；目录名也不能替代有效 JSON 中的 `network`/`security`。因此 Xray adapter 需要明确目标版本或能力 profile，不能把所有样本字段无条件合并为一个“最新 Xray 表单”。参考：[xray-client-side.md](xray-client-side.md) §8.3–§8.5。

---

## 十一、复杂扩展进入高级/兼容区的范围

以下内容候选默认折叠，不占用 VLESS/VMess 常用首屏：

- TCP HTTP header 伪装、PROXY protocol、socket 扩展；
- WebSocket Early Data、HTTP Upgrade 和额外 headers；
- gRPC 多路复用、心跳和窗口参数；
- XHTTP Mode 之外的 padding、session、reuse、download-settings；
- SplitHTTP、mKCP/KCP 的版本扩展；
- VMess Padding、Authenticated Length、AlterId；
- SMux、TFO、MPTCP、interface、routing mark、IP version；
- ECH、证书固定、mTLS、证书/私钥内容；
- 未知键和未来客户端扩展。

复杂扩展进入高级区不代表删除能力。结构化对象仍可编辑，无法稳定建模的内容可以通过作用域明确的高级 JSON 保留。

对于 XHTTP，当前项目注册表的 mode 选项与现行 Mihomo 文档存在漂移：项目代码含 `none`，而 Mihomo 文档列出的主要模式为 `auto`、`stream-one`、`stream-up`、`packet-up`。后续应把空值作为“未指定”，不要把 `none` 当作用户可见的真实 Mode。[Mihomo Transport 文档](https://github.com/MetaCubeX/Meta-Docs/blob/main/docs/config/proxies/transport.en.md)

---

## 十二、待进一步分析的问题

以下问题仍不在本文中擅自决定：

1. Xray target adapter 是只负责 Xray client JSON 导出，还是还要承担 Xray 节点导入、反向解析和回填编辑器。
2. target adapter 的版本 profile 如何选择：项目固定版本、手动选择版本，还是按能力探测。
3. 隐藏值保留时，未知顶层键是否全部保留，还是只保留带有 target/协议来源证据的键。
4. 协议切换时是否提供草稿恢复；同协议传输/安全切换与协议切换不能使用同一清理策略。
5. WS Path、gRPC ServiceName、H2/HTTP Path、Reality 公钥/Short ID 的校验，哪些目标采用硬错误，哪些只显示警告。
6. VLESS/VMess/Trojan 的共享传输编辑器与 Shadowsocks 插件编辑器的边界。
7. Hysteria/Hysteria2、TUIC、WireGuard、SSH、OpenVPN 等协议的条件认证规则和 target 输出能力。
8. SplitHTTP、mKCP、HTTPUpgrade 是否进入正式传输选择，还是长期作为兼容区字段。
9. 个人模板预设的默认值是仅在 UI 草稿中生效，还是点击“使用预设”后显式写入 `protocol_json`。
10. 是否需要增加“目标输出预览”来展示当前激活字段、转换字段和被跳过字段。

---

## 十三、后续研究建议

候选研究顺序如下，不代表已经授权构建：

1. 先为 VLESS、VMess、Trojan 画出共享条件元数据和目标能力矩阵。
2. 以项目个人模板、现有节点测试数据、Xray client/server 样本和 Mihomo 配置样本建立正反组合语料。
3. 核对 `protocol_json` 旧字段、顶层未知键、重复 WS 字段和当前 Clash 全量输出行为。
4. 单独研究 Xray client adapter 的目标 JSON 版本和导入/导出边界。
5. 再将条件模型扩展到 Shadowsocks 插件、Hysteria/TUIC/WireGuard、SOCKS/SSH 等协议。
6. 最后才决定是否把候选方案转化为 Design 文档和分步 Build 文档。

---

## 十四、研究依据

### 项目内资料

- [xray-server-side.md](xray-server-side.md)
- [xray-client-side.md](xray-client-side.md)
- [Xray-Core-API.md](Xray-Core-API.md)
- [Node-Link-Standards.md](Node-Link-Standards.md)
- [Clash-Verge-Rev-Node-Parameters.md](Clash-Verge-Rev-Node-Parameters.md)
- [Clash.yaml.template.md](../DocTemplates/Clash.yaml.template.md)
- [Issue12.md](../../Issue12.md)
- [Design3.md](../../Design3.md)（当前明确不重定义节点，本研究不改变该范围）
- [NodesView.vue](../../frontend/src/views/admin/NodesView.vue)
- [ProtocolFieldEditor.vue](../../frontend/src/components/ProtocolFieldEditor.vue)
- [registry.go](../../backend/internal/node/registry.go)
- [render_clash.go](../../backend/internal/assembly/render_clash.go)
- [links.go](../../backend/internal/assembly/links/links.go)

### 外部资料

- [Xray Transport Configuration](https://xtls.github.io/en/config/transport.html)
- [Xray VLESS outbound](https://xtls.github.io/en/config/outbounds/vless.html)
- [Xray VMess outbound](https://xtls.github.io/en/config/outbounds/vmess.html)
- [Xray REALITY](https://xtls.github.io/en/config/transports/reality.html)
- [Xray TLS](https://xtls.github.io/en/config/transports/tls.html)
- [Xray WebSocket](https://xtls.github.io/en/config/transports/websocket)
- [Xray gRPC](https://xtls.github.io/en/config/transports/grpc.html)
- [Mihomo TLS 配置](https://github.com/MetaCubeX/Meta-Docs/blob/main/docs/config/proxies/tls.en.md)
- [Mihomo Transport 文档](https://github.com/MetaCubeX/Meta-Docs/blob/main/docs/config/proxies/transport.en.md)
- [3x-ui InboundFormModal.tsx](https://github.com/MHSanaei/3x-ui/blob/main/frontend/src/pages/inbounds/form/InboundFormModal.tsx)
- [Mihomo issue #2533](https://github.com/MetaCubeX/mihomo/issues/2533)

## 变更记录

| 日期 | 说明 |
|---|---|
| 2026-09-02 | 新建研究汇总：记录独立 Xray target adapter、隐藏值保留与激活输出、多协议条件表单、高级/兼容区以及结构化表格和高级 JSON 方向；保留未决问题，待进一步分析。 |
