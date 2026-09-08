# Node-Editor-3xui-Xray-Research-2.md — 3x-ui 深度研究：Build17-21 之后仍可借鉴的节点编辑器增强方向

> **文档定位：** 本文是 [Node-Editor-3xui-Xray-Research.md](Node-Editor-3xui-Xray-Research.md) 的后续深度研究资料，承接 Design1～Design4 的设计链路（当前最新为 [Design4.md](../../Design4.md)）、[Build17.md](../reports/Build/Build17.md)～[Build20.md](../reports/Build/Build20.md)、[Build21.md](../../Build21.md)、[Issue13.md](../../Issue13.md) 以及 [Node-Editor-Design-Research.md](Node-Editor-Design-Research.md)、[Node-Editor-Improvement-Directions.md](Node-Editor-Improvement-Directions.md)。本文只做研究记录，不定义实现，不改动任何业务代码或既有文档，不代表对 3x-ui 的修改或产品背书。
> **研究状态：** 2026-09-05。基于本机仓库 `~/Desktop/Repo/3x-ui`（HEAD `f727d04f`，v3.7.0）与当前项目 `~/Desktop/Repo/VPN-Subscription-Management` 的当前源码、Build17～Build21 落地情况、Issue13 未闭环项进行静态分析；同时使用多个子代理并行取证。未构建、未改动外部项目与当前项目代码。
> **标注约定：** 【3x-ui 事实】= 本地 3x-ui 源码观察；【项目事实】= 当前项目源码或既有文档观察；【经推理】= 由证据推导、需后续设计验证的方向；【可能】= 对收益/风险的推测，不视为已定稿。
> **与前文关系：** 前文已经覆盖 3x-ui Inbound/Outbound 表单结构、能力纯函数、wire 适配、Link 导入、JSON 模式、条件表单方向等结论。本文不再重复这些基础结论，重点回答：在 Build17～Build21 已经把“条件元数据、当前状态、活动投影、目标检查、SS 插件基础合同”落到项目后，3x-ui v3.7.0 的真实代码里还有哪些**后续可借鉴机制**，以及哪些机制应仅作对照。

---

## 一、研究目的与范围

### 1.1 为什么在 Build17-21 之后继续研究 3x-ui

当前项目已完成（见 [Issue13.md](../../Issue13.md)、Build21）：

- `nodes` 行内当前状态/扩展/修订列（Build17）；
- `FieldSchema` 条件/选项/重置元数据、活动投影、保存校验与 `/check`（Build18）；
- 前端动态分区、可编辑下拉、局部 JSON、目标检查 UI（Build19）；
- 19 个 manual 协议统一保存契约、URI 导入归一化、Xray 来源适配、输出门槛（Build20）；
- R27-01～R27-08 与 R27-09 Step7～10（SS 插件统一合同、幂等归一化、固定敏感路径、SIP002 与 URI 目标分流）已闭环，但 **R27-09 Step11～14 尚未实施**（Clash/Mihomo 结构化插件投影、SS 插件专属目标诊断、未知插件前端编辑、全链路收口），且后续还有“其余 15 个协议完整条件表单、SS2022、独立 Xray outbound”等专项。

因此，本文不是“是否需要条件表单/当前状态”的研究，而是：

1. 从 3x-ui 找出当前项目尚未系统吸收的工程机制；
2. 判断这些机制能否帮助补齐 R27-09 剩余步骤与后续 15 协议/独立 Xray outbound；
3. 区分“可直接借鉴的机制”与“因多目标模型不同只能对照的机制”。

### 1.2 研究边界

- 3x-ui 是“Xray 服务端面板 + 面向订阅用户的单面板输出”；
- 当前项目是“多目标订阅管理，节点是中间语义对象，输出到 Clash/URI/Xray 等多种目标”；
- 因此本文不把 3x-ui 当作产品蓝本，而当作“协议边界行为与工程模式”的证据源。

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
| 尚未完成 | [Build21.md](../../Build21.md) §7.8～§7.11、[Issue13.md](../../Issue13.md) R27-09 | Step11 Clash 结构化插件投影、Step12 正式诊断、Step13 未知插件前端、Step14 全链路收口；其余 15 协议与独立 Xray outbound 仍后续 |

---

## 三、3x-ui v3.7.0 中未在前文完整覆盖的真实机制（3x-ui 事实）

### 3.1 分支“默认对象工厂”与切换时对安全层的精细保留

3x-ui 不只做“切换传输时删除旧子对象”，而是为每个 network/security 分支准备“最小可用默认对象工厂”。

- `frontend/src/lib/xray/outbound-form-helpers.ts`：`newStreamSlice(network)`、`hysteriaStreamSlice()`、`applyNetworkChange()`。
- `frontend/src/pages/inbounds/form/InboundFormModal.tsx`：Inbound 侧使用 `TcpStreamSettingsSchema.parse(...)`、`XHttpStreamSettingsSchema.parse(...)` 等，把 Zod 默认值作为新分支种子；`onNetworkChange()` 删除其它 `xxxSettings` 后写入新 network 默认对象。
- `frontend/src/schemas/protocols/stream/xhttp.ts`：`XMUX_FRESH_DEFAULTS` 跟踪 xray-core 默认值变化；`OutboundFormModal.tsx` 的 `onXmuxToggle()` 只在空对象时写默认，避免覆盖用户已填内容。

【3x-ui 事实】它解决的是“切到新分支后表单不残留旧分支值，也不因缺少默认子对象而空指针/校验崩溃”；同时在跨传输仍支持的安全字段（如 TLS SNI）上避免直接删除导致用户配置丢失。

【经推理】当前项目已用 `reset_on` + `current_state` 实现“切换即清空”，但字段级默认值仍是散落的。后续覆盖其余 15 个协议时，可考虑为每个协议/传输/安全/插件分支提供“默认对象工厂”概念：它不是把默认值批量写库，而是用于 UI 新分支的初始展示、空值判断和复杂对象的合法骨架。

### 3.2 表单 shape 与 wire shape 双层分离，adapter 是唯一转换层

3x-ui 的 Xray outbound 编辑不是直接编辑 wire JSON：

- `frontend/src/schemas/forms/outbound-form.ts`：表单层把 VMess/Trojan/SS/SOCKS/HTTP/WireGuard 压成单服务器、扁平、带 UI 标志的结构；
- `frontend/src/schemas/protocols/outbound/*.ts`：wire 层保持 Xray 需要的 `vnext[]/servers[]/peers[]` 结构；
- `frontend/src/lib/xray/outbound-form-adapter.ts`、`inbound-form-adapter.ts`、`stream-wire-normalize.ts`：负责 raw↔form 转换、旧字段迁移、UI-only 字段剥离、空值语义、XHTTP 互斥与 sockopt 默认值清理。

【3x-ui 事实】例如 WireGuard `pubKey` 是 UI 派生只读字段，adapter 不输出；XHTTP `enableXmux` 是 UI-only，`stripUiOnlyStreamFields()` 会删除；VMess 在 wire 中是 `vnext[].users[]`，表单层只是单个 address/id。

【经推理】当前项目把 `protocol_json` 作为内部中间对象，已经比“直接存 Xray wire”更接近表单层。但如果未来新增“独立 Xray outbound 目标”，不应在 `render_xray.go` 中临时拼 wire，而应建立一组“节点中间对象 ↔ Xray wire 对象”的显式 adapter，并让 Link 导入、JSON 编辑、保存前校验、正式输出共用该层。

### 3.3 Link 导入统一走“parse → canonical wire → adapter 回填”，而非把 URL 参数直接塞控件

3x-ui 的 OutboundFormModal 从链接导入时：

- `frontend/src/pages/xray/outbounds/OutboundFormModal.tsx`：`parseOutboundLink()` 得到 wire raw，再 `rawOutboundToFormValues()` 后 `methods.reset(next)`，同时刷新 JSON tab；
- `frontend/src/lib/xray/outbound-link-parser.ts`：解析 vmess/vless/trojan/ss/hysteria2/wireguard；XHTTP 高级字段从 query 与 `extra` JSON 合并；Hysteria2 的 `obfs/mport` 会还原为 finalmask/QUIC hop。

【3x-ui 事实】这种“先到 wire，再进表单”的方式保证了导入、表单、JSON 看到的对象是同一个规范形态，而不是每类 URL 写一套表单填充逻辑。

【经推理】当前项目 `uriparse → NormalizeProtocolJSON → InitCurrentState` 已经具有“先归一化再进存储”的思想。后续若做独立 Xray outbound 导入/导出，可沿用“parse → canonical wire → internal semantic object → target renderer”的单一路径；但要注意 3x-ui 的 adapter 会丢弃未建模未知键，而当前项目对未知 SS 插件参数已选择“保留未知参数”，因此不能整体照搬其有损回填。

### 3.4 后端 Go link parser 提供稳定 identity 与 tag 复用策略

3x-ui 后端除前端 parser 外，还有面向订阅刷新的 Go parser：

- `internal/util/link/outbound.go`：`ParseLink()` 支持 vmess/vless/trojan/ss/hysteria2/wireguard；每个 `ParseResult` 带 `Identity`（去掉 remark 的规范化身份）；
- `internal/web/service/outbound_subscription.go`：`assignStableTags()` 按“上次 identity→tag 映射”优先，其次按上一轮位置，最后新建；同批冲突追加 `-N`；
- `internal/sub/clash_yaml.go`、`clash_service.go`：Clash 名称唯一化与 YAML 歧义标量引号化。

【3x-ui 事实】该策略解决“同一远端服务器改名后，订阅刷新不把 tag/路由引用打断”的问题。

【经推理】当前项目 URI 导入仍按全局 `name` 去重；如果未来需要“远程订阅源反复刷新/同步”或“独立 Xray outbound 的 tag 长期稳定”，这套 identity + 位置回退策略很有参考价值。但 3x-ui 的 identity 明文包含凭据（UUID/password/私钥等），当前项目若采用，必须存加密或不可逆摘要，不能照搬明文 `LinkIdentities`。

### 3.5 JSON 编辑有“三种模型”，不是只有一种

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

### 3.6 校验错误路径映射到 Tab/列表身份，而不是只报一个字段名

- `frontend/src/pages/inbounds/form/formatValidationError.ts`：把 `settings.clients.<index>.<field>` 转成客户端 `email` 提示；只报首条并附 `+N more`；
- `frontend/src/pages/inbounds/form/InboundFormModal.tsx`：`firstRhfValidationIssue()` 找首个叶子错误，`tabForValidationPath()` 映射到 Basic/Protocol/Stream/Security/Sniffing 等 Tab，保存失败自动切 Tab。

【3x-ui 事实】它解决“错误在隐藏 Tab/数组里，用户不知道去哪找”的问题。

【经推理】当前项目已有 `revealField()` 展开 `<details>` 并滚动聚焦。后续可以为后端错误 path 增加“按 group/section 定位”的统一映射，并对列表错误用稳定条目身份（如 WireGuard `_credential_id`、3x-ui 的 email）替代索引文案。

### 3.7 数组/列表编辑的稳定 key、本地空行、类型默认工厂

3x-ui 对复杂数组使用多种补充机制：

- `useFieldArray` / `Form.List` 用稳定 `field.id` / `field.key` 作为 React key；
- `HeaderMapEditor.tsx` 本地 state 保留空行，只有填写有效 name 才进入 form，避免“新行一出现就被过滤”；
- `FinalMaskForm.tsx` 用 `defaultTcpMaskSettings(type)` / `defaultUdpMaskSettings(type)` 等“类型默认工厂”，并在挂载时迁移旧格式；
- 数组条目中条件子字段（如 `blockDelay` 仅在 action=block 时显示）在 `freedom.tsx` 等组件里实现。

【3x-ui 事实】这比“一行通用字段编辑所有对象”更适合 polymorphic 数组。

【经推理】当前项目 `ProtocolFieldEditor.vue` 已支持 `list` + `item_id_field`，但通用递归编辑器仍是 schema 驱动的平铺。若要支持未来复杂协议（XHTTP/FinalMask/AmneziaWG 混淆参数），可能需要“类型工厂 + 专用子编辑器 + 空行本地态”作为递归编辑器的补充形态。

### 3.8 后端订阅/Clash/JSON 生成的“每格式裁剪 + 自检”

3x-ui 的 raw/Clash/JSON 订阅都从同一 inbound stream 出发，但每个格式用独立裁剪逻辑剔除服务端专用字段：

- `internal/sub/service.go` 生成 raw 链接；
- `internal/sub/clash_service.go` / `clash_yaml.go` 生成 Clash，处理名称唯一与 YAML 标量引号；
- `internal/sub/json_service.go` 生成 Xray JSON outbound，处理 SS2022 多用户、WireGuard、Hysteria2 等；
- `internal/sub/host_sub.go` / `endpoint.go`：把 Host 与 legacy externalProxy 统一为 endpoint 覆盖层，raw/JSON/Clash 共用。

【3x-ui 事实】例如 Hysteria2 的 `obfs=salamander` 不是散在协议设置里，而是来自 finalmask UDP salamander；`mport` 来自 QUIC hop。3x-ui 的 JSON 生成器对 SS2022 使用 `method:serverKey:clientKey` 的 SIP022 URI 形式，Clash 侧则把 server+client password 拼成 `password`。

【经推理】当前项目已经有自己的多目标生成器与诊断；3x-ui 主要可借鉴“一个覆盖层对象供多个 renderer 共用”的思想，以及“每个 renderer 各自处理 server-only 字段裁剪、空值语义”的做法。但因为 3x-ui 是面向 Inbound+Client 的订阅分发，当前项目不要照搬其 SubService 业务链。

### 3.9 Write-only 凭据契约

3x-ui 对“面板节点”（远端 3x-ui API 凭据）采用明确的 write-only 契约：

- `internal/web/service/node_contract.go`：`NodeView` 只暴露 `HasApiToken`，不返回明文；更新用 `ApiToken *string`（省略/空白=保留），显式 `ClearApiToken` 才清除；
- `node_credentials_writeonly_test.go` 验证 API 不泄露 `apiToken` 字段。

【3x-ui 事实】这与当前项目的 `saved_sensitive_paths` / 留空保留 / credential_ops 思路一致；3x-ui 只覆盖单凭据字段，当前项目已覆盖嵌套与数组凭据，更细。

【经推理】若未来当前项目新增“远程订阅源/外部 API”等面板级节点管理，可直接把这种 `presence + pointer + explicit clear` 契约作为标准。

### 3.10 真实 Xray core 校验与版本 gate

- `internal/xray/api.go`：`ValidateOutboundConfig()` 通过 vendored xray-core 做真实启动级校验；
- `internal/web/service/xray_setting.go`：保存前逐 outbound 校验，并用 `shouldSkipLegacyUnencryptedOutboundRejection()` 对“旧版本可接受、新版本拒绝”的 vless/trojan 做版本 gate；
- `internal/web/service/outbound_subscription.go`：远端订阅落库前丢弃会阻止整个 core 启动的 outbound。

【3x-ui 事实】这比仅靠 schema/正则校验更接近真实运行边界。

【经推理】当前项目若做“独立 Xray outbound 输出/验证”，可把固定版本 client/core 作为检查参数，并在可行时调用真实 core 校验或固定版本离线 build，而不是只靠启发式诊断。版本 gate 需要注意其错误文本匹配较脆弱，需要封装成可测试分类器。

### 3.11 近期协议与高级字段（v3.7.0 新能力）

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

## 四、对照 Build17-21 后：可借鉴方向（经推理 / 可能）

### 4.1 R27-09 剩余步骤（Step11～Step14）

- 【项目事实】当前 `render_clash.go` 仍调用 `RenderPluginForClashLegacy`，把 SS 插件转成 `obfs-local;obfs=http` 字符串并删除结构化对象；这正对应 Build21 Step11 尚未实施。
- 【3x-ui 事实】3x-ui 的 SS/Clash 输出并不覆盖 obfs/v2ray-plugin/shadow-tls/restls 这套插件体系，因此它**不能直接作为 SS 插件 Clash 结构化输出的字段证据**；其相关代码只在 raw 链接生成时把 TCP HTTP header 重编码成 `obfs-local`。
- 【经推理】Step11 的权威仍应是当前项目 `ssplugin/contract.go` 与 Mihomo 1.19.29 固定版本源码/离线证据，而不是 3x-ui。3x-ui 可作为“不要把 URI 字符串插件形式用于 Clash 结构化输出”的反面佐证。
- 【经推理】Step13 未知插件参数前端编辑可借鉴 3x-ui `HeaderMapEditor`/Host 子树的“本地空行 + map 序列化 + 保留未知键”模式；当前项目 `ProtocolFieldEditor` 对 `map_value_type=string` 已有基本编辑，可补充“复杂值逐 key JSON 校验”与“空行不立刻写回”的交互。
- 【可能】若 Step12 需要目标诊断与正式装配一致，可参考 3x-ui “保存前/刷新前用真实生成器或 core validator 自检，而不是只做 YAML 生成成功检查”的工程模式；但当前项目的目标不是 Xray core，而是 Clash/SR/generic，因此更应继续复用 `selfcheck.go`/`diagnose.go`，并把诊断结果接入正式装配路径。

### 4.2 其余 15 个 manual 协议完整条件表单

- 【经推理】当前项目已经有通用 FieldSchema 递归编辑器；3x-ui 的“分支默认工厂”最适合用来为每个协议定义“新建时/切换后的合法空骨架”，例如 WireGuard peers、Hysteria2 obfs、TUIC 认证互斥。
- 【3x-ui 事实】Hysteria2 的 `obfs`/`ports` 在 3x-ui 不是简单顶层字段，而是分层到 finalmask UDP salamander 与 QUIC hop。当前项目 Hysteria/Hysteria2 schema 目前用顶层 `obfs/obfs-password/ports/hop-interval`，后续完整条件表单应评估是否需要“分层表达”或维持顶层中间对象但让目标投影负责转换。
- 【3x-ui 事实】WireGuard 在 3x-ui 中严格区分服务端密钥/入站与客户端 Peer；当前项目 R27-07 已实现稳定 Peer 身份，后续可继续借用“地址/保留字节 UI 逗号字符串、wire 数组”的 adapter 模式，以及 Peer 数组的稳定 key/错误定位。
- 【3x-ui 事实】SS2022 的方法枚举与密钥长度/多用户边界可作为当前项目 SS2022 “pending” 专项的参考；URI 侧 SIP022 的 percent-encode 语义比 base64 更准确。
- 【可能】TUIC/AnyTLS/Snell/Mieru/MASQUE/OpenVPN/SSH/ShadowQUIC/TrustTunnel/Tailscale 在 3x-ui 中没有直接 schema，因此仍需以项目现有 Reference、Mihomo/客户端生态和真实客户端文档为准；3x-ui 只能提供“表单/服务端/客户端字段分离”的方法论。

### 4.3 独立 Xray outbound 输出与固定验证 profile

- 【经推理】3x-ui 的 `schemas/forms/outbound-form.ts` + `outbound-form-adapter.ts` + `internal/sub/json_service.go` 是“把节点/入站转成单条 Xray outbound”的现成工程样例。
- 【经推理】当前项目若推进 Design4 §9.2 的独立 Xray outbound，可采用：
  1. 节点中间对象 → Xray wire adapter（扁平设置转 `vnext/servers/peers`）；
  2. 固定验证 profile 中调用真实 Xray core `Build()` 或离线固定版本校验；
  3. 将版本作为检查参数，做版本 gate。
- 【可能】若未来需要多 outbound 编排，可参考 3x-ui `json_service.go` 的 balancer/observatory tag 规则，但当前项目应以“单节点可验证 outbound”为起点，不直接引入 balancer。

### 4.4 URI 导入与远程刷新

- 【经推理】当前项目可按 3x-ui 的稳定 identity 思路，为“同一远端服务器换名后仍可识别”引入身份摘要；但必须用加密/哈希，不能明文持久化。
- 【经推理】当前 `uriparse` 协议覆盖面已比 3x-ui 更广，可补的是 3x-ui 在这些重叠协议上的健壮边界：SS2022 SIP022、Hysteria2 `obfs/mport/fm`、XHTTP 的 `extra`/snake_case 别名、WireGuard 参数别名。
- 【可能】3x-ui 有 `outbound_fuzz_test.go`；当前项目 `uriparse` 若有 fuzz 测试，能低代价发现编码/别名边界问题。

### 4.5 JSON 编辑与错误 UX

- 【经推理】当前项目保留“局部 JSON 草稿 + 完整目标只读检查”是合理的；若未来开放“完整 JSON 可写/可回灌”，应明确 adapter 是否丢弃未知键。3x-ui 的 Outbound 完整 JSON 回灌是有损的，不适合当前项目保留未知参数的目标。
- 【经推理】路径级 JSON 编辑（例如只编辑 `ws-opts`、`plugin-opts`、`reality-opts`）可借鉴 Host 子树 JSON 覆盖；它让高级用户不写整段节点 JSON。
- 【可能】引入 CodeMirror 类 JSON 编辑器可提供语法 lint/行列定位，但业务校验仍必须走服务端/共享规则，不能把 JSON 语法通过当成目标通过。

---

## 五、应保留边界 / 不应照搬（经推理）

1. **3x-ui 的 Inbound 服务端字段不能进入当前 manual 节点编辑器**：`clients[]`、fallbacks、sniffing、入站限流/流量重置、Reality privateKey/target、TLS 证书私钥等仍属于服务端；前文已确认，本文继续保留。
2. **3x-ui 的 hysteria 命名是 Hysteria2**：引用时必须显式区分当前项目的 Hysteria1 与 Hysteria2，避免协议映射错位。
3. **3x-ui 的完整 JSON 回灌可能丢未知字段**：当前项目对未知 SS 插件参数“明文保留/回显”是已确认方向，不应因借鉴 JSON 编辑而改丢。
4. **3x-ui 的 SubService 是单面板、按 subscriber 的订阅分发**：当前项目是多目标管理，不应整体搬入。
5. **3x-ui 的 identity 明文含凭据**：若要借鉴，需改成加密存储或不可逆指纹。
6. **3x-ui 不覆盖 SS 插件（obfs/v2ray-plugin/shadow-tls/restls）**：SS 插件仍以当前项目 `ssplugin` 合同与 Mihomo 固定版本证据为准。
7. **3x-ui 的协议 schema 面向 Xray wire**：不能直接替换当前项目的 Mihomo 风格 `protocol_json`，只能作为 Xray target adapter 的输入来源。

---

## 六、候选清单（供后续 Design/Build 前决策）

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

## 七、证据索引

### 7.1 3x-ui 本地证据（本文新增/深化）

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

### 7.2 当前项目证据

| 证据 | 位置 |
|---|---|
| 当前状态/保存契约 | `VPN-Subscription-Management/backend/migrations/1017_node_editor_state.sql`、`backend/internal/node/node.go` |
| FieldSchema 条件/投影 | `backend/internal/node/schema.go`、`registry.go`、`project.go` |
| 节点检查 | `backend/internal/node/check.go`、`backend/internal/assembly/node_check.go` |
| 前端动态表单/JSON | `frontend/src/views/admin/NodesView.vue`、`frontend/src/components/ProtocolFieldEditor.vue`、`frontend/src/components/NodeCheckPanel.vue` |
| URI 导入 | `backend/internal/uriparse/uriparse.go`、`backend/internal/node/uri_import.go`、`normalize.go` |
| SS 插件合同 | `backend/internal/ssplugin/contract.go`、`sip002.go`、`backend/internal/assembly/links/links.go` |
| Clash 旧投影遗留 | `backend/internal/assembly/render_clash.go`（`RenderPluginForClashLegacy` 调用点） |
| 未闭环计划 | `Build21.md` §7.8～§7.11、`Issue13.md` R27-09 |

### 7.3 外部资料

- [3x-ui GitHub](https://github.com/MHSanaei/3x-ui)
- [Xray-core config docs](https://xtls.github.io/en/config/)
- [Mihomo / Clash.Meta docs](https://wiki.metacubex.one/en/config/proxies/)
- [Node-Editor 既有研究汇总](Node-Editor-Design-Research.md)、[改进方向](Node-Editor-Improvement-Directions.md)、[3x-ui/Xray 前文研究](Node-Editor-3xui-Xray-Research.md)

> 外部资料仅用于补充语义；本地源码取证优先。引用外部链接不表示对对方项目进行改动或背书。

---

## 八、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-05 | 新建独立 Reference 文档：在 Build17～Build21 基础上，针对 3x-ui v3.7.0 真实代码做第二轮深度研究，归纳分支默认工厂、wire/form adapter、稳定 identity、JSON 三种模型、错误路径映射、真实 core 校验、协议演进等可借鉴机制；给出与当前项目后续专项（R27-09 剩余步骤、15 协议、独立 Xray outbound）的对照结论。仅文档，未改动代码或既有文档。 |
