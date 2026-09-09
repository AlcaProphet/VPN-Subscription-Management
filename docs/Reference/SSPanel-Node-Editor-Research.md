# SSPanel-Node-Editor-Research.md — SSPanel-UIM 节点编辑可借鉴与对照研究

> **文档定位：** 本文是对 SSPanel-UIM 后台节点管理、`custom_config` 多格式字段字典与操作便利层的深度研究，承接 Design1～Design4 的设计链路（当前最新为 [Design4.md](../../Design4.md)）、[Build17.md](../reports/Build/Build17.md)～[Build20.md](../reports/Build/Build20.md)、[Build21.md](../../Build21.md)、[Issue13.md](../reports/Issue/Issue13.md)，以及与 3x-ui 研究平行的 [Node-Editor-3xui-Xray-Research.md](Node-Editor-3xui-Xray-Research.md)。本文只做研究记录，不定义实现，不改动任何业务代码或既有文档，不代表对 SSPanel-UIM 的修改或产品背书。
> **研究状态：** 2026-09-05 创建，2026-09-08 文档交叉审核后同步至 Build21/Design4 v1.14 口径。基于本机仓库 `~/Desktop/Repo/SSPanel-UIM`（HEAD `d55a6071`，VERSION='25.1.0' "The Restoration"，app/predefine.php:9-10）与当前项目源码、Build17～Build21 落地情况、Issue13 未闭环项进行静态分析；并用公开 SSPanel Docs 链接辅助核对。未构建、未改动外部项目与当前项目代码。
> **标注约定：** 【SSPanel 事实】= 本地 SSPanel 源码观察；【项目事实】= 当前项目源码或既有文档观察；【经推理】= 由证据推导、需后续设计验证的方向；【可能】= 对收益/风险的推测，不视为已定稿。
> **文档历史：** 原文件名为 `SSpanel-Node-Editor-Research-2.md`，在 Reference 规范化时改为 `SSPanel-Node-Editor-Research.md`；早期 `SSpanel.md`/`SSpanel-Subscribe.md` 已合并为 [SSPanel-Research.md](SSPanel-Research.md)。

---

## 一、研究目的与范围

### 1.1 为什么在 Build17-21 之后继续研究 SSPanel

当前项目已完成（见 [Issue13.md](../reports/Issue/Issue13.md)、Build21）：

- `nodes` 行内当前状态/扩展/修订列（Build17）；
- `FieldSchema` 条件/选项/重置元数据、活动投影、保存校验与 `/check`（Build18）；
- 前端动态分区、可编辑下拉、局部 JSON、目标检查 UI（Build19）；
- 19 个 manual 协议统一保存契约、URI 导入归一化、Xray 来源适配、输出门槛（Build20）；
- R27-01～R27-08 与 R27-09 Step7～13（SS 插件统一合同、幂等归一化、固定敏感路径、SIP002 与 URI 目标分流、Clash/Mihomo 结构化插件投影、SS 插件专属目标诊断、未知插件前端编辑）已闭环，N-node-3/4 Step15 也已验收；**Step14 全链路回归与文档收口已完成**（见 [Build21.md](../../Build21.md) 与 [Issue14.md](../../Issue14.md)），后续还有“其余 15 个协议完整条件表单、SS2022、独立 Xray outbound”等专项。

因此，本文不是“是否要条件表单/当前状态”的研究，而是：

1. 从 SSPanel-UIM 找出在 Build17-21 之后仍能对当前项目节点编辑产生补充价值的机制；
2. 判断这些机制是“可直接借鉴”还是“仅能作为对照/反例”；
3. 区分 SSPanel 的“服务端节点管理壳”与当前项目的“多目标 manual 节点编辑器”的边界。

### 1.2 研究边界

- SSPanel-UIM 的 `node` 表是“供 XrayR 等后端轮询的服务器节点”，不是当前项目 manual 节点那样面向客户端订阅的代理节点定义；
- SSPanel 的节点编辑页把协议相关字段放在 `custom_config` 自由 JSON 中，后台本身没有协议级条件表单；
- 当前项目是“多目标订阅管理，节点是中间语义对象，输出到 Clash/URI/Xray 等多种目标”；
- 因此本文不把 SSPanel 当作产品蓝本，而当作“节点操作壳、custom_config 协议字典、多格式投影”的证据源。

---

## 二、当前项目 Build17-21 已落地能力（项目事实）

| 能力 | 落地证据 | 说明 |
|---|---|---|
| `nodes` 当前状态/扩展/修订 | [1017_node_editor_state.sql](../../backend/migrations/1017_node_editor_state.sql)、[node.go](../../backend/internal/node/node.go) | `current_state_json`、`extensions_json`、`edit_revision`、`state_format_version` |
| 条件/选项/重置元数据 | [schema.go](../../backend/internal/node/schema.go)、[registry.go](../../backend/internal/node/registry.go) | `When`、`RequiredWhen`、`ResetOn`、`OptionItems`、`AllowCustom`、`TargetEvidence` |
| 活动投影/保存校验 | [project.go](../../backend/internal/node/project.go) | 按当前 `network/security/plugin/features` 递归投影活动字段 |
| 节点检查 | [check.go](../../backend/internal/node/check.go)、[node_check.go](../../backend/internal/assembly/node_check.go) | 不落库，复用实际适配器 |
| 前端动态表单 | [NodesView.vue](../../frontend/src/views/admin/NodesView.vue)、[ProtocolFieldEditor.vue](../../frontend/src/components/ProtocolFieldEditor.vue) | 分区/条件/递归/JSON/凭据状态 |
| URI 导入统一归一化 | [normalize.go](../../backend/internal/node/normalize.go)、[uri_import.go](../../backend/internal/node/uri_import.go)、[uriparse.go](../../backend/internal/uriparse/uriparse.go) | 统一当前状态，逐行回执 |
| SS 插件固定合同 | [ssplugin/contract.go](../../backend/internal/ssplugin/contract.go) | Clash/SR/generic 三目标合同、支持等级 |
| 收口状态 | [Build21.md](../../Build21.md) §7、[Issue13.md](../reports/Issue/Issue13.md) R27-09 | R27-09 Step7～15 与 Step14 收口均已验收；其余 15 协议与独立 Xray outbound 仍后续 |

---

## 三、SSPanel-UIM 中未在前文完整覆盖的真实机制（SSPanel 事实）

### 3.1 后台节点 CRUD 是“服务端节点壳 + 自由 JSON custom_config”

SSPanel 的后台节点路由、控制器与模板分别位于：

- 路由：`app/routes.php:145-160`
- 控制器：`src/Controllers/Admin/NodeController.php`
- 页面：`resources/views/tabler/admin/node/create.tpl`、`edit.tpl`、`index.tpl`
- 模型：`src/Models/Node.php`
- 表：`db/migrations/2023020100-init.php:156-189`

【SSPanel 事实】节点行保存的管理字段包括：`name`、`type`（启用/隐藏）、`server`、`sort`（协议/接入类型）、`traffic_rate`、`is_dynamic_rate`、`dynamic_rate_type`、`dynamic_rate_config`、`node_class`、`node_group`、`node_speedlimit`、`node_bandwidth`、`node_bandwidth_limit`、`bandwidthlimit_resetday`、`node_heartbeat`、`online_user`、`ipv4`、`ipv6`、`node_group`、`online`、`gfw_block`、`password`（NodeAPI 通讯密钥）。协议相关参数没有独立列，全部放 `custom_config` JSON。

- `add()` 从表单取 `custom_config` 字符串，空串写入 `'{}'`，没有逐字段协议校验；`update()` 同样整体覆盖 `custom_config`（NodeController.php:95-160、189-239）。
- 创建/编辑页引入 `jsoneditor`，以 `modes: ['code', 'tree']` 编辑整个 `custom_config`（create.tpl:189-205、edit.tpl:237-242）。
- 接入类型下拉只列 Trojan/Vmess/TUIC/Shadowsocks2022/Shadowsocks，`sort` 值由 `Node::sort()` 解释为 0/1/2/3/11/14（Node.php:81-91）。

【经推理】这套后台是“面向 XrayR 的服务端节点壳”：它把一个节点的通用管理属性和后端通信属性做成了表单，而把真正影响客户端订阅的协议细节完全交给 JSON 编辑。它没有条件表单、没有字段级校验、没有当前组合投影，因此不能直接为当前项目提供“更先进的编辑器”蓝本；但它保留了“节点操作壳 + 自由 JSON”的历史形态，正好反衬当前项目 Build17-21 的 schema 化价值。

### 3.2 围绕节点编辑的操作便利层：复制、重置通讯密钥、重置带宽、动态倍率

【SSPanel 事实】除新增/编辑/删除外，后台节点页还提供：

- **复制节点**：`copy()` 复制一行节点，名称追加 ` (副本)`，重置 `node_bandwidth` 与 `password`（NodeController.php:312-334；index.tpl 中 `copyNode()`）。
- **重置通讯密钥**：`resetPassword()` 生成新的随机 NodeAPI `password`（NodeController.php:251-261）。
- **重置带宽**：`resetBandwidth()` 清零 `node_bandwidth`（NodeController.php:263-275）。
- **列表操作**：index.tpl 的“复制/删除/编辑”按钮与 AJAX 列表（NodeController.php:338-353）。

【经推理】SSPanel 没有把这些“复制/重置密钥/重置带宽”做成协议字段，而是放在编辑页/列表页的操作层。当前项目 manual 节点列表目前没有“复制节点”快捷操作；如果用户经常创建多个仅名称/地址/端口不同的相似节点，复制是一个低风险编辑效率项。但复制时必须处理当前项目新增的 `current_state_json`、`extensions_json`、`edit_revision` 与敏感凭据语义，不能照搬 SSPanel 的无脑 `replicate()`。

【可能】SSPanel 的“重置通讯密钥”对应的是当前项目 Xray 实例/外部账号的凭据管理，而不是 manual 节点；当前项目 Xray 实例页已经有凭据查看/重置等能力，因此不应把 SSPanel 的节点 `password` 概念引入 manual 节点编辑器。

### 3.3 `custom_config` 是事实上的多格式“协议渲染字典”

SSPanel 的订阅渲染器都从 `custom_config` 读取协议参数，再生成不同目标。这是与当前项目节点编辑关系最密切的一层：它给出了一个可观察的“旧生态字段字典”。

| SSPanel sort | 协议 | 主要消费方 | custom_config 中被读取的字段 |
|---|---|---|---|
| 0 | Shadowsocks | SS/SIP002/SIP008/Clash/SingBox/V2RayJson | `plugin`、`plugin_option`、`udp` |
| 1 | Shadowsocks2022 | Clash/SingBox/V2RayJson | `offset_port_user`、`offset_port_node`、`method`、`server_key`、`uot`、`udp` |
| 2 | TUIC | Clash/SingBox | `offset_port_user`、`offset_port_node`、`host`、`allow_insecure`、`congestion_control` |
| 3 | WireGuard | 渲染器基本未消费（Clash/SingBox/V2RayJson 无 case 3） | 无 |
| 11 | Vmess | V2Ray/SIP008?/Clash/SingBox/V2RayJson | `offset_port_user/node`、`security`、`encryption`、`network`、`host`、`path`、`header.*`、`allow_insecure`、`udp`、`ws-opts`/`ws_opts`、`h2-opts`/`h2_opts`、`http-opts`/`http_opts`、`grpc-opts`/`grpc_opts`、`servicename`、`utls`、`method`、`max_early_data`、`early_data_header_name`、`meek_url` |
| 14 | Trojan | Trojan/Clash/SingBox/V2RayJson | `offset_port_user/node`、`host`、`allow_insecure`、`security`、`mux`、`network`、`transport_plugin`、`transport_method`、`servicename`、`path`、`header.*`、`udp`、`ws-opts`、`grpc-opts` |

证据行号：

- Clash：`src/Services/Subscribe/Clash.php:25-165`
- SingBox：`src/Services/Subscribe/SingBox.php:23-165`
- V2RayJson：`src/Services/Subscribe/V2RayJson.php:23-166`
- V2Ray（vmess:// 行格式）：`src/Services/Subscribe/V2Ray.php:27-49`
- SIP002/Trojan 行格式：`SIP002.php:25-33`、`Trojan.php:28-42`

【SSPanel 事实】同一份 `custom_config` 被多个渲染器消费，但每个渲染器使用不同的输出结构：

- Clash 输出扁平 `proxies[]`，把 `httpupgrade` 映射为 `ws`（Clash.php:112-113、147-148），并把 `plugin-opts` 当作字符串/结构透传（Clash.php:29-43）。
- SingBox 输出 `type` + `tls` + `transport` 结构，读取 `servicename`、`max_early_data`、`early_data_header_name` 等字段（SingBox.php:89-132）。
- V2RayJson 输出 Xray wire 风格 `streamSettings`，读取 `header.request.path/headers`、`servicename`、`meek_url` 等（V2RayJson.php:64-166）。
- 行格式（vmess://、trojan://、SIP002）又是另一套编码/字段名（V2Ray.php:27-49、Trojan.php:28-42、SIP002.php:25-33）。

【经推理】这正好从另一个角度支持当前项目 Design4 的“同一节点中间语义 + 按目标投影”方向：SSPanel 已经用“一份 custom_config + 多个渲染器”证明了多格式投影的必要性，但它的投影是硬编码、无能力检查、无未支持字段诊断的。当前项目 Build18/20 的 `FieldSchema` + `diagnostics` + 输出门槛比 SSPanel 更进一层。

### 3.4 SSPanel 的字段别名与“旧 XrayR/SSPanel 形态”可作归一化参考

【SSPanel 事实】渲染器普遍读取以下旧/别名形态：

- `ws-opts` 与 `ws_opts`、`h2-opts` 与 `h2_opts`、`http-opts` 与 `http_opts`、`grpc-opts` 与 `grpc_opts`（Clash.php:107-110、144-145）。
- Xray 老式 `header.request.path[0]`、`header.request.headers.Host[0]`，同时兼容顶层 `host`、`path`（Clash.php:101-102、SingBox.php:92-95、V2RayJson.php:67-70）。
- `servicename`（不是 `serviceName`/`grpc-service-name`）被 SingBox/V2RayJson 读取。
- `offset_port_user`/`offset_port_node` 用于“节点端口 + 用户端口偏移”的服务端多用户模型。
- SS 插件参数历史上就是 `plugin` + `plugin_option` 字符串，不是结构化对象。

【项目事实】当前项目已经实现了规范化路径：`normalize.go`/`project.go` 把 `ws-path/ws-host/ws-headers` 收敛到 `ws-opts`，把 `grpc-service-name` 收敛到 `grpc-opts.grpc-service-name`，把 SS 已知插件 `plugin-opts` 收敛到各插件独立对象（[normalize.go](../../backend/internal/node/normalize.go)、[project.go](../../backend/internal/node/project.go)）。

【经推理】当前项目目前没有覆盖 SSPanel 常见的 `ws_opts`（下划线）、`servicename`、`header.request.*`、`offset_port_*` 这些外部字段名。若未来需要导入“SSPanel/XrayR 时代的 JSON 节点”或把这些渲染器作为旧格式兼容源，可以在规范化层补充这些别名；但如果不做 SSPanel JSON 导入，则它们不是当前 URI 导入的必需项，只能作为“未来别名扩展候选”。

### 3.5 SSPanel 的节点运维状态字段位于编辑页/列表，而非协议表单内

【SSPanel 事实】编辑页展示只读 IPv4/IPv6 解析结果、已用流量与重置按钮、通讯密钥与重置/复制按钮（edit.tpl:53、174-218）；列表页显示节点状态/倍率/在线/分组等列；`Cron::updateNodeIp()`、`detectNodeOffline()` 周期刷新 IP 与在线状态（`src/Services/Cron.php:80-150、682-688`）。

【经推理】这些字段描述“面板所管理的 XrayR 服务端是否在线、用了多少节点带宽”，不是“订阅中的客户端节点连接参数”。当前项目对 Xray 来源节点已经有 `last_seen_at`、`missing`、`allocatable` 等类似健康状态；对 manual 节点没有也不需要这套服务端心跳。因此不建议把这些字段塞进 manual 节点编辑器；最多在 Xray 实例/节点列表中提供状态汇总。

### 3.6 动态倍率与订阅格式开关是商业/运营层，不直接属于节点字段编辑

【SSPanel 事实】`traffic_rate` + `DynamicRate` 实现按小时动态倍率（NodeController.php:102-111、198-205；`src/Services/DynamicRate.php`）；订阅开关（`enable_ss_sub`/`enable_v2_sub`/`enable_trojan_sub`）决定是否渲染某类格式（SS.php:12-17、SIP002.php:12-17、Trojan.php:12-17）。

【经推理】这些属于计费/订阅渠道运营策略，当前项目已在早期 Design 中明确小团队不需要动态倍率、自然月配额等；节点编辑器不必引入。SSPanel 的“格式开关”与当前项目“平台/目标”概念不完全一致，当前项目更适合在目标装配/下载层管理。

---

## 四、对照 Build17-21 后：可借鉴方向（经推理 / 可能）

### 4.1 R27-09 已收口步骤（Step11～Step13、Step15）中 SSPanel 的对照价值

- 【项目事实】Build21 Step11～13 已完成并通过验收：Clash/Mihomo 结构化 SS 插件投影、SS 插件专属目标诊断与未知插件前端编辑均已落地；Step14 全链路回归/文档收口仍在 [Issue14.md](../../Issue14.md) 跟踪。
- 【SSPanel 事实】SSPanel 的 Clash 渲染器只把 `plugin` + `plugin_option` 当作字符串透传（Clash.php:29-43），没有为 `obfs/v2ray-plugin/shadow-tls/restls` 提供结构化字段映射；SIP002 也是把 `plugin`/`plugin_option` 直接拼进 query（SIP002.php:25-33）。
- 【经推理】SSPanel **不能作为 SS 插件结构化输出的正面证据源**。它说明“旧生态长期使用字符串插件形式”，而当前项目已采用结构化 `obfs-opts`/`v2ray-plugin-opts`/`shadow-tls-opts`/`restls-opts` + Mihomo 固定版本证据；SSPanel 可当作“旧字符串形态无法表达结构化参数/易产生静默差异”的反例。
- 【可能】未知插件参数的自由 JSON/键值编辑可参考 SSPanel 的 JSONEditor tree/code 双模式；当前项目 `ProtocolFieldEditor` 的 `map_value_type=string` 已更贴近未知字符串参数，是否需要全屏 JSON tree 编辑器取决于 UX，不是功能缺口。

### 4.2 其余 15 个 manual 协议完整条件表单：SSPanel 可补“最小生态字段字典”

当前项目后续要对 TUIC、SS2022、VMess/Trojan 等协议做完整条件表单。SSPanel 的渲染器虽然只有几类 sort，但它为 TUIC/SS2022/VMess/Trojan 提供了一套可交叉核对的“旧服务端渲染字段”：

- **TUIC**：SSPanel 在 Clash 中输出 `password`、`uuid`、`sni=host`、`congestion-controller`、`reduce-rtt`；在 SingBox 中输出 `uuid/password/congestion_control/zero_rtt_handshake/tls.server_name/insecure`（Clash.php:76-92、SingBox.php:63-86）。当前项目 TUIC schema 已有更多高级字段，但可把 SSPanel 的最小字段集作为“面向 Clash/SingBox 输出基本验证”的参考。
- **SS2022**：SSPanel 的 `method`/`server_key`/`uot`/`offset_port_*` 是与 XrayR 后端交互的旧字段；它不支持 SIP022 完整 URI 语义，但可作为当前项目 SS2022 后续专项的生态字段对照。
- **VMess/Trojan**：SSPanel 对 `ws-opts`/`grpc-opts`/`servicename`/`header.request.path` 的读取，可作为“同一配置被 Clash/SingBox/V2RayJson 读取时的旧别名差异”样本。
- **WireGuard/Hysteria/Hysteria2/AnyTLS/Snell/Mieru/MASQUE/OpenVPN/SSH/ShadowQUIC/TrustTunnel/Tailscale**：SSPanel 渲染器没有对应 case（除 WireGuard 表内 sort=3 但不渲染），不能作为这些协议 schema 来源。

【经推理】当前项目不应把 SSPanel 字典当作最终 FieldSchema 权威，因为 SSPanel 无 VLESS/REALITY/现代插件且缺少客户端版本证据；但它可作为“面向 XrayR/SSPanel 旧节点字段兼容”的补充证据，帮助决定是否需要接受 `servicename`、`ws_opts`、`header.request.path` 等旧别名。

### 4.3 低风险编辑效率：manual 节点“复制/另存为新节点”

- 【SSPanel 事实】SSPanel 提供复制节点，是后台节点页一个很直接的操作（NodeController.php:312-334）。
- 【项目事实】当前 NodesView 对 manual 节点只有“编辑/删除”，没有“复制为新节点”快捷入口；批量 URI 导入能一次创建多个，但不能基于现有节点复制后微调。
- 【经推理】复制节点可以作为后续 Build/UX 候选。它与 Build17-21 的保存契约兼容性需要额外设计：
  1. 新名称必须通过全局 `name`/有效渲染名唯一检查；
  2. `current_state_json` 应复制当前有效状态，不复制非激活分支（本来也没有）；
  3. `extensions_json` 可复制或要求用户确认，若扩展含敏感负载需要重新加密；
  4. `edit_revision` 应重置为 0，`saved_sensitive_paths` 重新按复制结果计算；
  5. 凭据是否原样复制还是留空/清空，是需要用户决策的产品语义，本文不擅自定稿。
- 【可能】对经常维护“同构不同线路”的小团队，复制收益高；对需要防止凭据扩散的场景，可默认复制为模板但不复制敏感值。

### 4.4 编辑页“读 IP/解析结果”与“节点健康摘要”可作为 Xray 节点增强，不作为 manual 节点字段

- 【SSPanel 事实】edit.tpl 展示只读 `ipv4/ipv6`，由 `Node::updateNodeIp()` 通过 DNS 解析填充（Node.php:120-139）。
- 【项目事实】当前 manual 节点编辑页没有“域名解析预览”；Xray 实例页/节点列表已有 `last_seen_at`/`missing`/`allocatable` 等健康信息。
- 【经推理】对 manual 节点加一个“解析预览/连通性提示”可能是轻量辅助（例如提示 host 能否解析、端口范围），但它不是 SSPanel 的服务端节点在线状态；也不应变成保存前的硬校验。
- 【可能】对 Xray 来源节点，可在命名/编辑入口显示最近检测时间与 missing 状态；当前列表已显示，编辑浮层是否也要显示可后续评估。

### 4.5 JSON 编辑的 tree/code 双模式可作为“高级数据”交互参考

- 【SSPanel 事实】后台 `custom_config` 使用 JSONEditor 的 `code`/`tree` 两种模式（create.tpl:189-193、edit.tpl:237-242）。
- 【项目事实】当前 `ProtocolFieldEditor` 的对象级高级 JSON 已有结构化/文本切换和“应用/放弃”草稿；未知扩展目前用表单输入，没有提供树形/代码双模式。
- 【可能】若未来未知扩展数量增多，可在扩展编辑浮层中提供 JSON tree/code 双模式；但必须保持当前项目“扩展负载加密保存、服务端只返回摘要”的边界。SSPanel 是直接明文 JSON 可编辑，当前项目不能照搬明文。

### 4.6 SSPanel 的“服务端节点操作壳”与当前项目 Xray 实例管理已有重叠

【项目事实】当前项目 `XrayInstancesView.vue` 已经覆盖实例测试连接、编辑 api_addr、启停、刷新节点、删除、独立账号凭据、重置配额/修复凭据等能力；后端也有 Xray 实例/账号管理路由。

【经推理】SSPanel 的 `node_heartbeat/online_user/node_bandwidth` 等字段在当前项目应由“Xray 实例检测/采集状态”承担，不需要进入 manual 节点编辑。若未来需要更完整的“节点在线/带宽/最近同步”视图，应放在 Xray 实例或 Xray 节点列表，而不是节点表单。

---

## 五、应保留边界 / 不应照搬（经推理）

1. **SSPanel 的 Node 行是“XrayR 后端节点”，不是客户端代理节点定义**：`node_class`、`node_group`、`node_speedlimit`、`node_bandwidth(_limit)`、`bandwidthlimit_resetday`、`node_heartbeat`、`online_user`、`ipv4/ipv6`、`online`、`gfw_block` 不应进入当前项目 manual 节点编辑器。
2. **SSPanel 的 `offset_port_user`/`offset_port_node` 依赖“每用户端口”服务端模型**：当前项目 manual 节点是单节点连接描述；若未来要支持按用户端口/SS 多用户，应作为独立用户生命周期设计，而不是在节点编辑器里增加“偏移端口”字段。
3. **SSPanel 的 `custom_config` 自由 JSON 是缺乏 schema/校验的旧形态**：当前项目 Build17-21 的 `FieldSchema`、`current_state`、活动投影、未知扩展保护是更优路径，不应为了兼容 SSPanel 而退回自由 JSON 为第一编辑入口。
4. **SSPanel 没有 VLESS/REALITY/Shadowrocket 插件等现代字段**：它不能作为这些协议或 SS 插件结构化输出的最终证据。
5. **SSPanel 的 SS 插件 `plugin/plugin_option` 是字符串拼装**：当前项目 `ssplugin/contract.go` 与 Mihomo 固定版本证据应继续作为 SS 插件权威，不能从 SSPanel 复制字符串形式到 Clash 结构化输出。
6. **SSPanel 渲染器静默跳过不支持的 sort**（如 WireGuard 在 Clash/SingBox/V2RayJson 无输出）：当前项目已经用诊断/输出门槛避免静默丢弃，不应回退。
7. **SSPanel 的明文 JSON、无加密扩展与 read-modify-write 流量累加是反例**：当前项目对凭据/扩展加密和原子更新契约更严格。

---

## 六、候选清单（供后续 Design/Build 前决策）

| 编号 | 候选方向 | 当前状态 | 建议后续处理 |
|---|---|---|---|
| S1 | manual 节点“复制为新节点”快捷操作 | 未开始 | 进入 Design 阶段；需明确凭据是否复制、扩展是否重加密、名称冲突/当前状态/修订处理 |
| S2 | 将 SSPanel/XrayR custom_config 旧字段别名纳入归一化候选（`ws_opts`、`servicename`、`header.request.*`、`offset_port_*`） | 未开始 | 仅在出现“导入 SSPanel/XrayR JSON”或旧配置兼容需求时展开；不要为 URI 导入无依据扩张 |
| S3 | 用 SSPanel TUIC/SS2022/VMess/Trojan 渲染字段作为“后续 15 协议”最小生态字段交叉证据 | 未开始 | 作为 Design 阶段参考资料，不作为唯一权威；仍需 Mihomo/sing-box/Shadowrocket 固定版本证据 |
| S4 | manual 节点编辑浮层增加域名解析/地址提示 | 可能 | 轻量 UX；需避免把 DNS 结果当作保存硬校验 |
| S5 | 未知扩展编辑浮层增加 JSON tree/code 双模式 | 可能 | 可参考 SSPanel/3x-ui JSON 编辑交互；须保持扩展负载加密/摘要边界 |
| S6 | Xray 来源节点/实例增加更完整的在线/带宽/最近同步状态视图 | 后续运营层 | 不属于 manual 节点编辑；可放在 Xray 实例模块，需要用户确认范围 |
| S7 | 不作为候选项：把 SSPanel 服务端节点字段搬进 manual 节点编辑器 | 明确排除 | 保持 manual 节点为多目标客户端节点语义 |

---

## 七、证据索引

### 7.1 SSPanel 本地证据

| 证据 | 位置 |
|---|---|
| VERSION | `SSPanel-UIM/app/predefine.php:9-10` |
| 后台节点路由 | `SSPanel-UIM/app/routes.php:145-160` |
| 节点 CRUD/操作 | `SSPanel-UIM/src/Controllers/Admin/NodeController.php` |
| 节点表结构 | `SSPanel-UIM/db/migrations/2023020100-init.php:156-189` |
| Node 模型与 sort 解释 | `SSPanel-UIM/src/Models/Node.php` |
| 创建/编辑页 JSONEditor | `SSPanel-UIM/resources/views/tabler/admin/node/create.tpl`、`edit.tpl`、`index.tpl` |
| custom_config 多格式字典 | `src/Services/Subscribe/Clash.php`、`SingBox.php`、`V2RayJson.php`、`V2Ray.php`、`SIP002.php`、`Trojan.php` |
| 心跳/在线/IP 更新 | `src/Services/Cron.php`、`src/Models/Node.php` |
| 动态倍率 | `src/Services/DynamicRate.php` |

### 7.2 当前项目证据

| 证据 | 位置 |
|---|---|
| 当前状态/保存契约 | `backend/migrations/1017_node_editor_state.sql`、`backend/internal/node/node.go` |
| FieldSchema 条件/投影 | `backend/internal/node/schema.go`、`registry.go`、`project.go` |
| 节点检查 | `backend/internal/node/check.go`、`backend/internal/assembly/node_check.go` |
| 前端动态表单/JSON | `frontend/src/views/admin/NodesView.vue`、`frontend/src/components/ProtocolFieldEditor.vue`、`frontend/src/components/NodeCheckPanel.vue` |
| URI 导入 | `backend/internal/uriparse/uriparse.go`、`backend/internal/node/uri_import.go`、`normalize.go` |
| SS 插件合同 | `backend/internal/ssplugin/contract.go`、`sip002.go`、`backend/internal/assembly/links/links.go` |
| Clash SS 插件输出 | `backend/internal/assembly/render_clash.go`、`ssplugin/contract.go`（Build21 Step11 后为结构化投影，不再作为旧 URI 字符串透传） |
| 收口跟踪 | `Build21.md` §7、`docs/reports/Issue/Issue13.md` R27-09、`Issue14.md`（后续专项） |

### 7.3 外部资料

- [节点配置 | SSPanel-Docs](https://docs.sspanel.io/docs/configuration/nodes/)
- [SSPanel/XrayR custom_config 对照](https://anonymous-6.gitbook.io/xrayr/dui-jie-sspanel/sspanel/sspanel_custom_config.md)
- [Mihomo / Clash.Meta docs](https://wiki.metacubex.one/en/config/proxies/)
- 既有研究： [SSPanel-Research.md](SSPanel-Research.md)、[Node-Editor-3xui-Xray-Research.md](Node-Editor-3xui-Xray-Research.md)、[Node-Editor-Ecosystem-Research.md](Node-Editor-Ecosystem-Research.md)

> 外部资料仅用于补充语义；本地源码取证优先。引用外部链接不表示对对方项目进行改动或背书。

---

## 八、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-05 | 新建独立 Reference 文档：在 SSPanel.md / SSPanel-Subscribe.md 与 Build17～Build21 落地基础上，对 SSPanel-UIM v25.1.0 后台节点管理、custom_config 多格式字段字典、节点操作壳、运维状态与动态倍率做二轮深度研究；区分可直接借鉴项（复制节点、旧别名参考、多格式投影反证）与明确排除项（服务端字段搬进 manual 编辑器）。仅文档，未改动代码或既有文档。 |
| v1.1 | 2026-09-08 | Reference 规范化：文件名由 `SSpanel-Node-Editor-Research-2.md` 改为 `SSPanel-Node-Editor-Research.md`；同步 Design4 v1.14 / Build21 收口状态（Step11～13、Step15 已验收，Step14 跟踪中）。 |
