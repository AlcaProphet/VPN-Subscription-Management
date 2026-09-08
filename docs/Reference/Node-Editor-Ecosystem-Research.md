# Node-Editor-ThirdParty-Supplement-Research.md — 节点编辑器第三方生态补充研究（Build17-21 / Build23 基础上）

> **文档定位：** 本文是 [Node-Editor-3xui-Xray-Research-2.md](Node-Editor-3xui-Xray-Research-2.md) 与 [SSpanel-Node-Editor-Research-2.md](SSpanel-Node-Editor-Research-2.md) 的后续补充研究资料。前两轮已分别深挖 3x-ui 与 SSPanel；本文把视野扩展到当前项目“还需要跟随/可借鉴”的其它生态：Mihomo / Clash Verge Rev、sing-box、v2rayN、NekoBox、Sub-Store、Hiddify、Marzban，并回到当前仓库自身代码与 `Build17`～`Build21`、当前 `Build23` 计划做第二轮交叉盘点。
> **研究状态：** 2026-09-05。基于当前仓库 `~/Desktop/Repo/VPN-Subscription-Management`（HEAD `bde43de`）、`Build17.md`～`Build23.md`、`Design4.md`、`docs/reports/BuildReport/BuildReport4.md`，以及本机 `~/Desktop/Repo/3x-ui`、`SSPanel-UIM`、`clash-verge-rev`、`Xray-core` 等仓库静态分析；同时使用 web 搜索补充外部资料。只做研究记录，不修改业务代码或既有文档，不把推测写成已定稿。
> **标注约定：** 【项目事实】= 当前项目源码/文档观察；【生态事实】= 第三方源码/文档观察；【经推理】= 由证据推导、需后续设计验证；【可能】= 收益/风险推测。
> **本次决策（由执行代理自行作出并标注）：** 仅新建本文件到 `docs/Reference/`，不改动其它文档/代码。文件名采用与既有两轮平行的补充命名 `Node-Editor-ThirdParty-Supplement-Research.md`；若用户希望合并进既有某份 Reference，可再调整。

---

## 一、研究目的与范围

### 1.1 为什么还要做第三轮

- 【项目事实】Build17～Build21 已完成：`nodes` 行内当前状态/扩展/修订、`FieldSchema` 条件/选项/目标证据、活动投影、`/check`、前端动态表单、19 协议统一保存契约、URI/Xray 来源归一化、SS 插件统一合同等。
- 【项目事实】`Build23.md` 是当前尚未开始执行的构建方案，覆盖 Build21 遗留的 **Step 1～5**：Clash/Mihomo 结构化 SS 插件投影与自检、SS 插件统一目标诊断、未知插件前端编辑、VMess/VLESS SR URI TLS 参数补全、全链路回归。
- 【项目事实】`BuildReport4.md` 列出 N-node-1～N-node-6 等节点输出/诊断缺口，也提示“其余 15 个 manual 协议完整条件表单、SS2022、独立 Xray outbound”仍是后续专项。
- 【项目事实】既有 Reference-2 两篇主要回答“3x-ui / SSPanel 还能提供什么”，尚未系统回答：**如果进一步看向 v2rayN、NekoBox、Sub-Store、Hiddify、Marzban、sing-box 等更广生态，哪些能力值得参考？** 以及 **当前协议注册表相对本仓库自己的 Clash/Mihomo 模板还有多少字段级缺口？**

因此本文的定位不是替代 Build23，也不是重复 3x-ui/SSPanel 二轮，而是给 **Build23 完成之后的“协议全量表单、编辑效率、导入/导出、模板/复制、远程资源”** 等方向提供候选素材。

### 1.2 研究边界

- 当前项目是“多目标订阅管理”，manual 节点是中间语义对象，输出到 Clash YAML / Shadowrocket URI / generic URI / 未来 Xray outbound。
- 因此服务端面板（Hiddify、Marzban、SSPanel）中面向 **Xray inbound / 服务端入站** 的字段只能作对照，不能直接进入 manual 节点编辑器。
- 客户端编辑器（v2rayN、NekoBox）和订阅资源工具（Sub-Store、Clash Verge Rev）可作为 **交互/导入/导出/资源组织** 的证据源，不是产品蓝本。
- 本文的“经推理/可能”内容不进入任何 Build 的验收清单；如需转化为设计必须先经用户确认。

---

## 二、当前代码/设计二次盘点：不是只差 UI，而是“条件模型仍只覆盖首批四协议”

### 2.1 已落地但与后续协议扩展相关的关键代码

| 层 | 代码事实 | 说明 |
|---|---|---|
| 当前状态 | [node.go](../../backend/internal/node/node.go) 的 `CurrentState` 只保存 `network/security/plugin/features`；[DeriveCurrentState](../../backend/internal/node/node.go) 也围绕这四个维度派生 | 对 VLESS/VMess/Trojan/SS 够用；对 TUIC 的 v4/v5、Snell 的版本/obfs 模式、Mieru 的 transport/multiplexing、MASQUE/OpenVPN 的模式等没有“变体/认证模式”字段 |
| 条件规则 | [schema.go](../../backend/internal/node/schema.go) 的 `ConditionRule` 仅支持 `network/security/plugin/plugin_not/features/targets` | 前端 [nodeFormLayout.ts](../../frontend/src/utils/nodeFormLayout.ts) 与后端使用同一组维度；无法表达“当 `auth_mode=v5` 时显示 uuid+password、隐藏 token”这类互斥分支 |
| 清空作用域 | [node.go](../../backend/internal/node/node.go) `normalizeResetScopes` 只允许 `protocol/network/security/plugin/feature.*` | 后续协议若要按 `auth_mode/version/obfs-mode` 清空，需要扩展合法 scope 或建立通用 `variant` 维度 |
| 协议注册表 | [registry.go](../../backend/internal/node/registry.go) 只对 VLESS/VMess/Trojan/SS 执行 `enrich*` 与 `organizeFirstBatchForm` | 其余 15 个协议仍是早期平铺 schema，没有条件显隐/推荐项/目标证据 |
| Clash 输出 | [render_clash.go](../../backend/internal/assembly/render_clash.go) 的 `normalizeClashFields` 仍调用 `RenderPluginForClashLegacy` | Build23 Step 1 的直接对象；`plugin-opts` 被拍平成 URI 字符串 |
| 检查链路 | [node_check.go](../../backend/internal/assembly/node_check.go) 的 `linkTargetDiagnostics` 对 SS 插件是硬编码 | Build23 Step 2 的直接对象；`target_evidence` 仍未被通用运行时消费 |
| 未知插件前端 | [ProtocolFieldEditor.vue](../../frontend/src/components/ProtocolFieldEditor.vue) 已支持 `map_value_type=string` 的字符串 map 编辑 | Build23 Step 3 的基底已存在，但完整回归未验收 |
| URI 往返 | [links.go](../../backend/internal/assembly/links/links.go)、[uriparse.go](../../backend/internal/uriparse/uriparse.go) | Build23 Step 4 将补 SR VMess/VLESS TLS 参数 |

### 2.2 一个容易忽略的事实：本仓库自带 Mihomo/Clash 官方模板，已经能作为“剩余 15 协议字段缺口”的第一手字典

【项目事实】`docs/DocTemplates/ClashOfficial.yaml.template.md` 收录了大量当前 Mihomo/Clash 生态支持的协议与完整字段；它比 `registry.go` 中非首批协议的字段集更完整。逐项对照后，能看到剩余 15 协议“不是 UI 没做完，而是 schema 本身尚未完整建模”。

| 协议 | 当前 registry 已有字段（摘） | 本仓库 Mihomo 模板中可补充/需核对字段（摘） | 判断 |
|---|---|---|---|
| Snell | `psk/udp/version` | `version` 支持 1～5、`reuse`、`obfs-opts`（shadow-tls/restls/jls 子结构）、`client-fingerprint` | 【经推理】若以 Clash 为目标，当前 Snell 编辑器不足以表达模板中已支持的常见变体 |
| Hysteria | `auth/auth-str/obfs/ports/protocol/up/down/...` | `ech-opts`、`name-cert-verify`、mTLS `certificate/private-key` 等 | 【可能】多数是高级可选，是否纳入看产品范围 |
| Hysteria2 | `password/obfs/obfs-password/.../cwnd/udp-mtu` | `obfs` 还支持 `gecko`，`obfs-min/max-packet-size`、`bbr-profile`、`ech-opts`、`realm-opts`、mTLS 与 QUIC 窗口 | 【经推理】至少应补充 gecko 混淆参数的分支条件；`realm-opts` 等可作为后续 |
| TUIC | `token/uuid/password/...` 三者并存且无互斥 | 模板明确 **tuicV4 必须 token，V5 必须 uuid+password 且不可同填**；另有 `bbr-profile/ech-opts/name-cert-verify` | 【项目事实】当前条件模型无法表达认证版本互斥；这是最典型的“需要新增 variant 维度”的例子 |
| WireGuard | `private-key/public-key/peers/...` | 模板还支持 `amnezia-wg-option`、`reserved` 可写 Base64 字符串或 int 数组、peers 段落语义 | 【经推理】若需要 AmneziaWG 或保留字符串 reserved，需要 schema/UI 扩展 |
| Tailscale | 只有 `auth-key` | `hostname/control-url/state-dir/ephemeral/udp/accept-routes/exit-node/exit-node-allow-lan-access` 等 | 【项目事实】当前字段远不足以编辑 Tailscale 类型节点 |
| OpenVPN | 只有 `client-config` 全文 | 模板是结构化字段（`proto/cipher/auth/ca/cert/key/tls-auth/tls-crypt/...`） | 【经推理】两种编辑路线需要用户决策：保留全文 or 提供结构化/导入解析 |
| MASQUE | `private-key/public-key/ip/ipv6/mtu/udp/...` | `network: h2/h3-l4proxy`、`congestion-controller/handshake-timeout` 等 | 【可能】当前建模为“类 WireGuard”，缺少自身协议变体 |
| Mieru | `username/password/transport/multiplexing/handshake-mode` | `traffic-pattern`；multiplexing 值前缀需核对（`MULTIPLEXING_LOW` 等） | 【经推理】当前 select 值与模板/生态命名可能不完全一致 |
| AnyTLS | `password/ech-opts/idle-*` 等 | 模板还展示 `shadow-tls-opts/restls-opts/jls-opts` 内层伪装选项 | 【可能】可作为后续高级字段，不是首批必要 |
| ShadowQUIC | 只有 `password/sni` | 模板有 `username/password/alpn/quic-versions/udp-over-stream/zero-rtt/keep-alive/congestion-controller/...` | 【项目事实】当前 schema 明显不完整 |
| TrustTunnel | 只有 `password` | 模板有 `username/password/client-fingerprint/health-check/udp/sni/alpn/quic/reuse` 等 | 【项目事实】同上 |
| SSH | `username/password/private-key/...` | 字段名在模板/类型定义间存在 `privateKey` 与 `private-key` 差异；需以固定版本为准核对 | 【经推理】不只是补字段，还要做字段名规范化 |
| HTTP/SOCKS5 | 基础认证/TLS | CVR 类型定义字段与当前基本一致；可补 headers/name-cert-verify 等 | 【可能】低风险小补 |

> 上述“当前 registry”是指 `backend/internal/node/registry.go` 中 `ManualProtocols()` 的直接字段；部分字段可能在 Build21 后的代码中已存在但未展开条件，因此该表只用于研究对照，不作为实施清单。

### 2.3 因此真正的“下一阶段主线”可能是

1. 先把 Build23 的 SS 插件/SR TLS 问题收口；
2. 再把 `FieldSchema/CurrentState/ResetScope` 从“首批四协议专用”扩展成可表达 **协议内互斥模式（variant/auth mode/version/transport type）** 的通用模型；
3. 然后以本仓库 `ClashOfficial.yaml.template.md` + 固定版本客户端/内核证据为字典，逐协议补齐 15 个 manual 协议的 schema；
4. 再根据编辑效率需要评估复制节点、导入 YAML/JSON、模板预设、远程刷新等“非协议字段”能力。

---

## 三、第三方/上游生态第二轮可借鉴方向

### 3.1 Mihomo / Clash Verge Rev：最近、最直接的字段与编辑操作证据

【生态事实】
- Clash Verge Rev 的 `src/types/global.d.ts` 定义了 `IProxyVlessConfig`、`IProxyVmessConfig`、`IProxyTrojanConfig`、`IProxyShadowsocksConfig`、`IProxyHysteria2Config`、`IProxyWireguardConfig`、`IProxySnellConfig` 等客户端侧字段；当前项目很多字段名与它一致，说明方向正确。
- CVR 的 `src/components/profile/proxies-editor-viewer.tsx` 提供多行 URI/Base64 批量解析：按行解析、解析失败不阻塞、按 `name` 去重、异步分批（每 50 行）避免阻塞 UI；可视化视图与 Monaco 高级 YAML 可切换。
- CVR 的 profile 体系把扩展编辑保存为 `prepend/append/delete` 序列，而不是把整个配置重写；这适合“在不破坏原订阅的前提下增删节点”的场景。
- 本仓库 `docs/DocTemplates/ClashOfficial.yaml.template.md` 是当前项目自身携带的 Mihomo 生态字段字典，尤其 SS 插件 `plugin/plugin-opts` 正确形态为结构化对象，而不是 URI 字符串。

【经推理】
- Build23 Step 1 的 Clash SS 插件输出，应直接采用本仓库模板中的 `plugin: obfs` + `plugin-opts: {mode: http}` 形态；这也是 CVR 类型定义接受的结构。
- 当前项目的 URI 批量导入已有“逐行回执/跳过”，但没有“从 Clash YAML `proxies:` 片段粘贴并转换为节点候选”的入口。CVR 的异步解析 + 去重逻辑可作为该功能的交互参考。
- CVR 的“可视化 + 高级 YAML 可切换”可启发节点高级区：现有对象级 JSON 已存在，未来若做“完整目标 YAML/JSON 只读预览”，可复用 CVR Monaco 经验，但业务校验仍必须走服务端。
- CVR 的 `prepend/append/delete` 模式适合“订阅扩展层”，当前项目装配已有覆盖层概念，因此不是 manual 节点编辑器需要照搬的模型。

### 3.2 v2rayN / NekoBox(NekoRay)：协议覆盖面与“服务器配置对话框”的交叉验证

【生态事实】
- v2rayN 是 Windows 客户端，有“服务器管理/配置对话框”，能编辑 VLESS/VMess/Trojan/SS/Hysteria2/WireGuard 等；DeepWiki 显示它有配置生成系统，可从分享链接导入并生成内核配置。
- NekoBoxForAndroid / NekoRay 的文档强调“Protocol Support”和“DNS/Protocol Registries”；它对移动端较少常见协议（AnyTLS、Mieru、MASQUE、ShadowQUIC、WireGuard、Hysteria2 等）有实际配置入口。

【经推理/可能】
- 这两个项目不能直接提供“schema 驱动的条件表单”参考（它们多为静态页面/代码生成），但可作为 **某个协议是否真实被客户端支持、字段如何命名、导入链接能带哪些参数** 的生态证据。
- 如果后续要补 `uriparse` 对 ShadowQUIC/TrustTunnel/MASQUE 等链接的支持，v2rayN/NekoBox 的导入解析与协议文档是比 3x-ui 更贴近“客户端可用性”的来源。
- 这两个项目把“服务器列表”和“订阅分组”分开管理；当前项目节点表已经有 manual/xray 来源与组分配，概念上不需要新增，但其“从剪贴板批量添加、分组标记、测速/排序”等低风险 UX 可继续观察。

### 3.3 Sub-Store：资源层操作管线，而不是节点字段编辑器

【生态事实】
- Sub-Store 面向 QX/Loon/Surge/Stash/Shadowrocket 等客户端，核心是订阅资源管理、过滤/脚本操作、合并/覆盖/上传/同步/定时刷新。
- DeepWiki 显示它有 Script Execution System；社区脚本能对节点集合做 rename/filter/merge。

【经推理】
- Sub-Store 的价值不在“单个节点字段怎么编辑”，而在 **当节点数量多、需要从远程订阅反复同步时，如何把原始资源与最终展示分开，并给用户提供可组合操作管线**。
- 当前项目已有规则素材池的“来源/快照/同步/诊断”模型，但节点侧还没有“远程节点订阅源”或“节点资源操作”层。若未来要把别人的订阅作为节点源管理，Sub-Store 的“filter/script/override + 稳定 key + 定时刷新”是更完整的参考系。
- 在实现远程节点源之前，不必引入 Sub-Store 式操作脚本；那会超出当前“manual 节点 + Xray 检测节点”的范围。可作为候选，不作为近期 Build。

### 3.4 Hiddify Manager / Marzban：服务端面板的“模板/用户/入站”经验

【生态事实】
- Hiddify Manager 支持多协议服务端配置与用户/流量管理；文档中有 Configuring Proxy Protocols。
- Marzban 是 Xray 面板，文档有 Xray Inbounds、hosts、user template 等；可对多个 host/inbound 统一生成用户配置。

【经推理/可能】
- 它们解决的是“服务端入站 + 多用户 + 订阅链接生成”，与当前项目 manual 节点编辑器边界不同；不宜把它们的 inbound 字段搬进来。
- 但“用户模板/预设”和“从模板快速创建一组相似配置”是当前 manual 节点编辑器缺少的效率能力。SSPanel-2 已提出“复制节点”，Hiddify/Marzban 的模板思路可进一步扩展为“节点预设/协议模板库”。
- 如果未来做独立 Xray outbound 输出，Marzban/Hiddify 的“host 覆盖同一 inbound”模型可作为“同节点多域名/多入口”的参考，但当前没有该需求，只记录。

### 3.5 sing-box：可能的下一目标或校验工具

【生态事实】
- sing-box 支持 VLESS/VMess/Trojan/SS/Hysteria2/TUIC/WireGuard/SSH/AnyTLS 等，且有 `sing-box check` 命令可用于配置校验；其 outbound 类型和 JSON schema 在官方文档与 DeepWiki 中有完整说明。

【经推理/可能】
- 当前项目没有 sing-box 输出目标；如果把 sing-box 纳入目标，需要像 Mihomo/CVR 一样固定版本并建立适配器，不能只加渲染。
- sing-box 更适合作为 **离线校验工具/对照字典**：在本地/CI 用固定版本 `sing-box check` 验证“由当前字段映射出的 sing-box 配置”是否合法，帮助发现字段拼写/结构错误。但这需要先有 sing-box target adapter 或仅用于研究夹具，不能直接让项目运行时依赖外部二进制。
- 与 `mihomo -t`、Xray core 校验类似，任何“真实内核校验”都只应作为自动化验收或可选高级检查，不能成为普通用户保存节点的强制门槛。

---

## 四、候选改进方向（供 Build23 之后决策；非实施承诺）

以下编号继续使用 `C10+` / `S8+` 之外的“T”前缀，避免与前两轮 Reference 混淆。所有“经推理/可能”均需用户确认后再进入 Design/Build。

| 编号 | 候选方向 | 来源/证据 | 当前状态 | 建议 |
|---|---|---|---|---|
| T1 | 将 `FieldSchema/CurrentState/ResetScope` 扩展出通用 `variant/auth_mode/version/transport` 维度，使 TUIC v4/v5、Snell 版本、Hysteria2 obfs、Mieru transport 等可表达互斥分支 | 当前 [schema.go](../../backend/internal/node/schema.go)、[node.go](../../backend/internal/node/node.go) 只支持四维度；Mihomo 模板 TUIC 明确互斥 | 未开始 | 作为后续 15 协议表单的前置设计，建议先做“条件模型演进”专项 |
| T2 | 以本仓库 `ClashOfficial.yaml.template.md` 为第一字典，逐协议补齐 15 个 manual 协议 schema（至少先补 Tailscale/ShadowQUIC/TrustTunnel/OpenVPN 等明显不足项） | [ClashOfficial.yaml.template.md](../DocTemplates/ClashOfficial.yaml.template.md) | 未开始 | 进入 Design 前先固定版本并整理字段差异表；不要直接全量照搬模板 |
| T3 | 提供“复制节点/另存为新节点”操作；当前 manual 名称创建后不可改，复制是创建相似节点的最低成本路径 | [node.go](../../backend/internal/node/node.go) `UpdateManual` 禁止改名；SSPanel copy node | 未开始 | 需明确凭据是否复制、扩展是否重加密、`edit_revision/saved_sensitive_paths/current_state` 重置规则 |
| T4 | 在 URI 导入之外，增加“粘贴 Clash YAML `proxies:` / v2rayN JSON”的候选导入入口；可借鉴 CVR 异步批量解析与按名去重 | CVR `proxies-editor-viewer.tsx`；当前 [uri_import.go](../../backend/internal/node/uri_import.go) 只解析 URI | 可能 | 先做“解析为草稿/预览”，不直接大批落库；敏感/未知字段需显式提示 |
| T5 | 提供“节点模板/预设库”，用户可保存常用协议参数模板并一键预填；比默认对象工厂更贴近 Hiddify/Marzban/SSPanel 的运营习惯 | 3x-ui 默认对象工厂、Hiddify/Marzban 模板、SSPanel 复制 | 可能 | 模板只显式预填，不把展示默认值静默写入数据库 |
| T6 | 对 OpenVPN 等“全文型”协议增加结构化导入/解析，或至少提供 .ovpn 粘贴后的关键字段摘要；避免用户在超大 textarea 里手工排错 | Mihomo 模板 OpenVPN 结构；当前 registry 只有 `client-config` | 可能 | 是产品路线决策：保留全文 or 结构化字段 or 两者并存 |
| T7 | 将 Build23 收口后的 SS 插件合同继续扩展为“可注册插件目录”，把 Mihomo 模板中 gost-plugin/jls/kcptun 等作为未来 known plugin 候选 | [ClashOfficial.yaml.template.md](../DocTemplates/ClashOfficial.yaml.template.md) 中的 `ss5`、`ss-jls`、`ss-kcptun` | 可能 | 在未知字符串 map 已成立后，新增 known plugin 只是合同+表单问题，不需要重新设计存储 |
| T8 | 把“目标证据”从前端元数据升级为检查/装配消费证据，并做成用户可见能力标签；Build23 只限定 SS 插件，未来可扩展但不默认全局 | BuildReport4 N-node-2；[registry.go](../../backend/internal/node/registry.go) `TargetEvidence` | 后续独立项 | 先完成 Build23 Step 2，再评估非 SS 字段是否需要 UI 展示 |
| T9 | 提供“从远程节点源/订阅源同步节点”的能力，借鉴 3x-ui 稳定 identity + Sub-Store 资源操作；身份摘要必须加密/哈希 | 3x-ui-2 C3、Sub-Store | 后续可能 | 需要用户确认是否引入“节点源”概念，不是 manual 编辑器自身范围 |
| T10 | 引入固定版本内核/客户端校验作为开发期/CI 夹具（`mihomo -t`、`sing-box check`、Xray core Build），不作为运行时强制依赖 | Build21/Build23 固定版本思路；sing-box check | 已部分在测试夹具 | 继续保留为自动化验收；不要增加运行时子进程 |
| T11 | 在节点编辑高级区增加“完整目标 YAML/JSON 只读预览 + 文本 Diff”，复用 CVR Monaco/现有 DiffView 经验；不开放第二份可写真值 | CVR proxies-editor-viewer、[DiffView.vue](../../frontend/src/components/DiffView.vue) | 可能 | 低风险 UX；需先解决 Build23 输出正确性再开放预览 |
| T12 | WireGuard `reserved` 同时接受 int 数组与 Base64 字符串，MASQUE/Tailscale/ShadowQUIC 等协议补充网络模式/高级参数 | Mihomo 模板 | 可能 | 属于 T2 的细节，不单独推进 |

---

## 五、应保留边界 / 不应照搬（经推理）

1. **Hiddify/Marzban/SSPanel 的 Xray inbound / 服务端字段不能进入 manual 节点编辑器**：它们描述服务端监听与用户，不是客户端节点连接。
2. **不要因 Sub-Store 强大就引入节点操作脚本引擎**：当前项目已有规则素材池，节点侧没有远程订阅源；脚本引擎会显著扩大复杂度。
3. **v2rayN/NekoBox 的静态编辑页不是 schema 驱动条件表单的参考**：可以用于字段/链接证据，但不应照搬其 UI。
4. **sing-box 不能只靠“它也支持这些协议”就新增输出目标**：必须固定版本、建 adapter、做往返与正反例。
5. **Mihomo 模板字段不能自动全量搬进 registry**：模板包含大量服务端/高级/实验性字段，需要按“当前项目支持哪些客户端入口、是否影响核心语义”裁剪。
6. **OpenVPN 结构化改造若实施，必须处理内嵌凭据**：`ca/cert/key/tls-auth/tls-crypt` 都可能含私钥，不能因为变成结构化输入就忽略加密/脱敏。
7. **Clash Verge Rev 的 `prepend/append/delete` 扩展模型面向订阅文件**：当前 manual 节点是数据行，不应把序列操作硬套成节点编辑保存模型。

---

## 六、证据索引

### 6.1 当前项目证据

| 证据 | 位置 |
|---|---|
| Build17～Build21 已落地/遗留 | [Build17.md](../reports/Build/Build17.md)～[Build20.md](../reports/Build/Build20.md)、[Build21.md](../../Build21.md)、[Build23.md](../../Build23.md) |
| 全量核验与新增缺口 | [BuildReport4.md](../reports/BuildReport/BuildReport4.md) §5.3 |
| 当前状态/作用域限制 | [backend/internal/node/node.go](../../backend/internal/node/node.go)、[backend/internal/node/schema.go](../../backend/internal/node/schema.go) |
| 协议注册表 | [backend/internal/node/registry.go](../../backend/internal/node/registry.go) |
| Clash SS 旧投影 | [backend/internal/assembly/render_clash.go](../../backend/internal/assembly/render_clash.go) |
| SS 插件合同 | [backend/internal/ssplugin/contract.go](../../backend/internal/ssplugin/contract.go) |
| 前端条件匹配/类型 | [frontend/src/api/node.ts](../../frontend/src/api/node.ts)、[frontend/src/utils/nodeFormLayout.ts](../../frontend/src/utils/nodeFormLayout.ts) |
| Mihomo/Clash 官方模板（本地字典） | [docs/DocTemplates/ClashOfficial.yaml.template.md](../DocTemplates/ClashOfficial.yaml.template.md) |

### 6.2 本地第三方仓库证据

| 证据 | 位置 |
|---|---|
| 3x-ui 详细研究 | [Node-Editor-3xui-Xray-Research-2.md](Node-Editor-3xui-Xray-Research-2.md) |
| SSPanel 详细研究 | [SSpanel-Node-Editor-Research-2.md](SSpanel-Node-Editor-Research-2.md) |
| Clash Verge Rev 代理字段/类型 | `~/Desktop/Repo/clash-verge-rev/src/types/global.d.ts` |
| Clash Verge Rev proxies 编辑器 | `~/Desktop/Repo/clash-verge-rev/src/components/profile/proxies-editor-viewer.tsx` |

### 6.3 外部资料

- [Mihomo Wiki](https://wiki.metacubex.one/en/config/proxies/)
- [Clash Verge Rev GitHub](https://github.com/clash-verge-rev/clash-verge-rev)
- [sing-box Documentation](https://sing-box.sagernet.org/)
- [Sub-Store GitHub](https://github.com/sub-store-org/Sub-Store)
- [Hiddify-Manager GitHub](https://github.com/hiddify/Hiddify-Manager)
- [Marzban Xray Inbounds](https://gozargah.github.io/marzban/en/docs/xray-inbounds)
- [NekoBoxForAndroid 文档](https://matsuridayo.github.io/nb4a-configuration/)
- [v2rayN GitHub / DeepWiki](https://github.com/2dust/v2rayN)

> 外部资料仅补充语义/生态证据；本地源码取证优先。引用链接不表示对上述项目改动或背书。

---

## 七、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-05 | 新建第三方生态补充研究：在 3x-ui/SSPanel 二轮之外，补充 Mihomo/CVR、sing-box、v2rayN/NekoBox、Sub-Store、Hiddify/Marzban 等生态的可借鉴方向；对照当前代码与 `ClashOfficial.yaml.template.md` 找出剩余 15 协议 schema 缺口；列出 Build23 之后候选方向。仅文档，未改动代码或其它文档。 |
