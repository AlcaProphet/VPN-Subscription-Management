# BuildReport3.md — Build17~Build20 节点编辑器表单结构与 Design4 对齐审查报告

> 核验日期：2026-09-03（以当前工作区代码为准）。
> 核验对象：当前工作区中 Build17~Build20 的落地产物、[Design4.md](../../../Design4.md)、`docs/Reference/Clash-Verge-Rev-Node-Parameters.md`、`Clash-Verge-Rev-Subscription-Assembly.md`、`Node-Editor-3xui-Xray-Research.md`、`Node-Editor-Design-Research.md`、`Node-Editor-Improvement-Directions.md`，以及 `~/Desktop/Repo` 下 clash-verge-rev / 3x-ui / Xray-examples 等对应源码。
> 结论前置：**Build17~Build20 的“可编译、可运行、自动化通过”是成立的；但“达到 Design4 预期效果”不成立。** 主要缺口集中在 Build19 的 UI 信息架构与状态回显：安全/TLS/REALITY 字段被归入“认证与密钥”，且安全选择在编辑已保存 TLS/REALITY 节点时回显为默认 `none`；开关区、高级区/目标检查区、列表类控件也没有按 Design4 和 Reference 的交互预期落地。
> v2.0 补充：已针对用户提出的“连接参数不符 / 文案混乱 / UI 排版结构混乱 / 未知拓展意义不明 / 选项框错误内容”五类问题逐项深挖，结论见第六章。

---

## 一、总览

| 维度 | 结果 |
|------|------|
| 后端抽查 | ✅ `go test ./internal/node ./internal/assembly ./internal/assembly/links ./internal/xray -count=1` 全绿 |
| 前端构建 | ✅ `npm run build` 通过（仅有既有 chunk 体积提示） |
| Build17 后端保存契约 | ✅ 与代码事实一致（迁移/修订/凭据/扩展均存在） |
| Build18 条件元数据/检查接口 | ✅ 后端元数据与检查接口已落地；但前端消费方式有偏差 |
| Build19 前端动态表单 | ⚠️ **未完全达标**：分区归属/编辑回显/开关折叠/高级数据区与 Design4 不一致 |
| Build20 全协议过渡/输出门槛 | ⚠️ **基本落地但 UI 侧“效果好”不成立**；部分输出映射仍为“暂保留/待验证”状态 |
| 代码改动 | 本报告只审查和记录，不改动代码/文档外的构建产物 |

---

## 二、最重要的代码事实

### 2.1 安全/TLS/REALITY 字段被归入“认证与密钥”，不是“连接方式与当前参数”

Design4 §3.1/§5 以及 Reference `Node-Editor-Design-Research.md` §5.1、`Node-Editor-Improvement-Directions.md` §3.1 均把“安全选择、TLS/REALITY 身份参数”放在“连接方式”/当前连接区，而不是凭据区。

当前代码事实：

- `backend/internal/node/registry.go`
  - `setSecurityCondition()` 会把 TLS/REALITY 相关字段设置为 `Group: "auth"`（第 307~311 行）。
  - `enrichVLESS/enrichVMess/enrichTrojan` 调用后，`servername/alpn/client-fingerprint/skip-cert-verify/fingerprint/reality-opts` 大量落入 `auth` 组。
  - VLESS/VMess 的 `security` 选择字段也显式 `Group: "auth"`。
- `frontend/src/views/admin/NodesView.vue`
  - `fieldGroup()` 只要 `field.group` 存在就返回该值（第 122~131 行），因此后端下发的 `group=auth` 会直接进入“认证与密钥”区。
  - 模板先渲染“认证与密钥”，再渲染“连接方式与当前参数”（第 715~731 行）。
- 实际 schema 输出抽样（以当前 registry 运行结果为准）：
  - VLESS：`uuid=auth`、`reality-opts=auth`、`skip-cert-verify=auth`、`fingerprint=auth`、`servername=auth`、`client-fingerprint=auth`、`security=auth`，而 `network/ws-opts/grpc-opts/...` 才在 `connection`。
  - Trojan：`password=auth`、`alpn/sni/skip-cert-verify/fingerprint/client-fingerprint=auth`，`network/ws-opts/grpc-opts=connection`。
  - VMess：`uuid/servername/alpn/skip-cert-verify/fingerprint/client-fingerprint/security=auth`，`cipher/network/ws-opts/...=connection`。

由此 UI 上“认证与密钥”混入大量非凭据的 TLS/REALITY 参数；而且 `security` 字段在 schema 中追加在尾部，当切换到 TLS/REALITY 后，SNI/ALPN/指纹等会插在“安全选择”之前，用户会看到安全选择被推到下方，阅读和操作顺序混乱。

### 2.2 编辑已保存 VLESS/VMess TLS/REALITY 节点时，安全选择回显错误

根因链路：

1. `backend/internal/node/project.go` 的 `protocolParamsForStorage()`（第 547~566 行）在保存 VLESS/VMess 时，把表单层 `security` 转为既有 `tls: true/false` 并删除 `security`。
2. 因此数据库/API 返回的 `protocol_json` 通常只有 `tls:true` + `reality-opts`，没有 `security`。
3. `frontend/src/views/admin/NodesView.vue` 的 `openEdit()`（第 425~437 行）只把 `n.protocol_json` 拷入表单，没有从 `n.current_state.security` 补回 `form.protocol_json.security`。
4. `frontend/src/components/ProtocolFieldEditor.vue` 的 select/可编辑下拉使用 `modelValue ?? field.default`（第 312 行），`security` 字段默认是 `"none"`，于是编辑 TLS/REALITY 节点时下拉显示为“none”，而不是 TLS/REALITY。
5. 页面顶部的“当前组合”又通过 `currentState` 正确显示为 TLS/REALITY，于是出现“摘要显示 TLS、表单控件却显示 none”的割裂。

这不是纯展示问题：若用户不重新选择安全方式就保存，`protocol_json` 中仍是旧 `tls:true`，服务端可接受；但表单给用户的反馈是错误且危险的，REALITY 节点甚至会把默认“none”显示给用户，容易造成误切。

### 2.3 独立开关区缺少“常用/更多开关”分层，高级区与目标检查区也没有按 Design4 折叠

- `NodesView.vue` 第 733~739 行把所有 `groupFields('switches')` 平铺在同一 `FormSection` 中；没有 Build19 Step 3 要求的“常用开关直接呈现、更多开关区内默认折叠”。
- `commonFieldSchema()` 中的 `tfo/mptcp` 等也是 bool，`setDefaultFieldGroup()` 会把它们归入 `switches`，因此 VLESS/VMess/SS 的开关区会混入大量跨协议通用开关，而 TLS 证书校验类的 bool 却被归入 `auth`，位置和分类均不合理。
- `NodesView.vue` 第 741~800 行是“更多功能/高级参数”的 `<details>`，内部放未知扩展；第 802 行的 `<NodeCheckPanel />` 在 `<details>` 外面、始终展开。Design4 §3.1 的“高级数据与目标检查 默认折叠”没有实现。

### 2.4 列表类字段仍是“逗号分隔单行输入”，未达到 Reference 的增删列表预期

- `frontend/src/components/ProtocolFieldEditor.vue` 第 316~318 行：`text-list` / `int-list` 直接渲染单行 `Input`，用逗号分隔字符串承载。
- 这与 `Node-Editor-Design-Research.md` §5.3、`Node-Editor-Improvement-Directions.md` §3.3 的“ALPN、Allowed IPs 等使用可增删列表；Headers 使用键值行；顺序和类型有意义时必须保留”不一致。
- 对 ALPN、Allowed IPs、reserved、DNS 等多值字段，用户无法直观增删，顺序与空值也难管理。

---

## 三、逐 Build 未达标/未执行核对

### Build17（后端保存契约）

| 项 | 结论 | 证据 |
|---|---|---|
| 1017 迁移、行内当前状态/扩展/修订 | ✅ | `backend/migrations/1017_node_editor_state.sql` 与 `node.go` 均存在 |
| 创建/更新/读取 API、409 | ✅ | `UpdateManual` 使用 `WHERE edit_revision=?` 原子更新 |
| 凭据/扩展加密与导入保护 | ✅ | `enc:v1:`/`enc:ext:v1:` 逻辑与测试存在 |
| 表单结构 | 本轮不负责前端 | 不判定 |

Build17 后端基本达标。

### Build18（FieldSchema/检查接口）

| 项 | 结论 | 证据 |
|---|---|---|
| `when/required_when/reset_on/option_items/...` | ✅ | `schema.go`、`registry.go` 存在 |
| 四协议注册表条件矩阵 | ✅ 后端数据存在 | `enrichVLESS/enrichVMess/enrichTrojan/enrichSS` |
| 活动投影与保存校验 | ✅ | `project.go` |
| `/api/admin/nodes/check` | ✅ | `server/node.go`、`node/check.go`、`assembly/node_check.go` |
| 固定版本正反例 | ✅ | `assembly/testdata/node_check/` |
| 前端分组消费 | ❌ | 安全/TLS 字段仍因 `Group:"auth"` 落入凭据区，见 §2.1 |

### Build19（前端动态表单）

| Step | 文档要求 | 当前状态 | 主要缺口 |
|---|---|---|---|
| Step 1 类型/API | 扩展前端类型与检查 API | ✅ | `api/node.ts` 已扩展 |
| Step 2 可编辑下拉 | 候选/自定义/失焦不改写 | ✅ | `EditableCombobox.vue` 已实现 |
| Step 3 分区/当前组合摘要 | connection 应含 `section=transport/security` | ⚠️ | `fieldGroup()` 优先使用 `group`，而后端把 security/TLS 设成 `auth`；原计划的 security 回退逻辑实际不生效 |
| Step 3 开关区 | 常用直接、更多折叠 | ❌ | 没有“更多开关”内部分层 |
| Step 3 高级区 | 默认折叠、配置摘要 | ✅ 基本 | `<details>` 存在，摘要仅计数，无诊断摘要 |
| Step 4 分支清空/凭据/JSON | 切换清空、凭据状态、局部 JSON | ✅ 基本 | Toast 是事后提示；凭据状态与 security 回显问题相关 |
| Step 5 目标检查 UI | 高级数据默认折叠、迟到保护 | ⚠️ | 迟到保护有；但检查面板没有放在折叠的“高级数据”内，始终展开 |
| Step 6 保存/409 | 409 保留草稿 | ✅ | 测试存在 |

### Build20（全协议过渡/输出门槛）

| Step | 文档要求 | 当前状态 | 主要缺口 |
|---|---|---|---|
| Step 1 URI/表单统一归一化 | 统一入口、当前状态初始化 | ✅ | `normalize.go` 存在 |
| Step 2 全 19 协议统一保存 | 枚举 create/update/read | ✅ | 测试覆盖 |
| Step 3 Xray 来源适配/SS 插件目标映射 | `renderPluginForTarget(plugin, opts, target)` | ⚠️ | 仅 `obfs` 在 URI 侧映射为 `obfs-local`；`v2ray-plugin/shadow-tls/restls` 仍“暂保留原格式”；Clash 侧没有看到等价的 plugin 字符串映射 |
| Step 4 输出生成门槛 | SR 跳过/Clash 阻止/零链接拒绝 | ✅ | `diagnose.go`、`render_sr.go`、`render_clash.go` 已接入 |
| Step 5 回归/文档收口 | 全量通过 | ✅ 自动化 | UI 视觉/交互仍未达到 Design4 预期，自动化未覆盖 |

---

## 四、与 Reference 和上游源码的对照

- `clash-verge-rev/src/types/global.d.ts` 对 VLESS/VMess/SS 的字段分层非常清晰：基础字段、`tls`/`reality-opts`/传输对象并列，并不把 SNI/ALPN 当“凭据”。当前项目 registry 的 `Group:"auth"` 与之相悖。
- `3x-ui/frontend/src/pages/inbounds/form/InboundFormModal.tsx` 使用独立 Tabs/分区把协议、传输、安全、高级分开，并在切换协议/传输/安全时只渲染当前组合。当前项目虽然是单浮层而非 Tabs，但至少应在分区上把“传输/安全/当前参数”放在同一连接区。
- `Node-Editor-3xui-Xray-Research.md` §2.4/§2.5 强调切换 network/security 时的数据行为与 UI 条件渲染一致；当前后端清空/投影逻辑已做，但前端 `security` 回显没有与 `current_state` 接上，导致切换模型在 UI 上不可信。
- `Node-Editor-Design-Research.md` §5.1 建议“基础信息/连接方式/兼容与性能/高级 JSON”四层；当前多了“认证与密钥”却把安全字段放进去，本质是层次标签错位。

---

## 五、改进意见（按优先级）

### P0：先修复“编辑回显”和“分组错位”，这是表单看起来混乱的最大来源

1. **编辑打开时，从 `current_state` 回填表单选择字段**：
   - 在 `openEdit()` 中，若 `currentSchema()` 含 `security` 且 `n.current_state?.security` 存在，则把 `form.protocol_json.security = n.current_state.security` 写入草稿；network/plugin/features 也建议显式补一次，避免默认值与真实状态不一致。
   - 或者在 `fieldValue()`/`ProtocolFieldEditor` 中对 `security` 这类“只保存在 current_state 的虚拟选择字段”提供 fallback，而不是 fallback 到 `field.default`。

2. **修正 `FieldSchema.group` 归属**：
   - `auth` 只放真正凭据（UUID/password/token/private-key/预共享密钥等）。
   - `security` 选择、TLS/REALITY 子参数（SNI/ALPN/client-fingerprint/reality-opts/证书校验）应归 `connection`。
   - Trojan 的 TLS 身份参数也放入连接区；SS 插件参数应与 plugin 选择保持在同一上下文。
   - 可保留 `basic` 分组给基础信息；前端再把“基本信息/认证”按 Design4 允许的小标题拆开，而不是把 TLS 参数塞进“认证与密钥”。

3. **调整字段排序**：
   - `security` 选择应位于其依赖参数之前（如 UUID 之后、SNI/REALITY 子参数之前）。
   - `network` 应位于 `packet-encoding/ws-opts` 等传输参数之前；目前 schema 原始顺序里部分高级/传输参数会先于选择控件出现。

### P1：按 Design4/Reference 补交互层级

4. **独立开关区做“常用/更多开关”折叠**：
   - 将每个 bool 的常用性/分组下沉到 schema（如 `group:"switches"` + `advanced?:true`），或在前端按名称白名单分两层。
   - TLS `skip-cert-verify` 不应作为全局常用开关，建议放入 TLS 安全子区。

5. **把“高级数据与目标检查”做成独立默认折叠区**：
   - 未知扩展、局部 JSON 入口、目标检查结果统一放到一个可折叠的“高级数据与目标检查”区域。
   - 当前 `NodeCheckPanel` 不要默认占满表单尾部；用户需要时才展开。

6. **补齐列表/多值字段编辑器**：
   - 为 `text-list/int-list` 实现可增删行，ALPN/Allowed IPs/DNS/reserved 不再用逗号单行。
   - 多行文本（OpenVPN client-config、证书、私钥等）使用 `TextArea`，不能统一单行 Input。

### P2：继续收口输出与协议元数据

7. **SS 插件目标映射继续按 Build20 Step 3 契约收口**：
   - 把 `renderPluginString` 升级为 `renderPluginForTarget(plugin, opts, target)`，URI、Clash YAML 使用不同目标表达；`v2ray-plugin/shadow-tls/restls` 至少要输出诊断/映射状态，不能只靠“暂保留原格式”。
   - 检查 `clashProxy()` 是否在 Clash 产物中生成 Mihomo 可消费的 plugin 字符串或结构；若采用结构体，需用夹具锁定。

8. **为非首批 15 个协议补最小“连接组合”元数据**：
   - 现阶段可接受静态表单，但如果目标是“全 19 协议都能有清晰分区”，至少应把 Hysteria/TUIC/WireGuard 等协议的 bool 开关按语义分组，而不是所有 bool 平铺。

### 测试建议

9. **增加前端回归测试，锁定表单结构而不是只测“能找到某 label”**：
   - 断言 VLESS 新建时 `security` 显示在 TLS 字段之前、属于“连接方式”区而非“认证与密钥”。
   - 断言编辑 `protocol_json:{tls:true}` + `current_state:{security:"tls"}` 的节点时，安全下拉显示 TLS。
   - 断言开关区存在“更多开关”折叠（如果实施）。
   - 增加列表编辑器单测。

---

## 六、针对用户补充五类问题的逐项研究（v2.0）

> 本部分按用户补充的 5 个问题分别记录：确认的问题原因、代码证据、影响、解决方向。不修改代码。

### 6.1 问题 1：各类连接方式的常用参数与先前的调查结果不符合

#### 6.1.1 根因

当前表单并不是真正按“当前连接组合”投影字段；`registry.go` 里保留了大量无条件/旧版字段，且部分字段类型和上游 CVR/Mihomo 不一致。

#### 6.1.2 已确认证据

| # | 代码事实 | 与调查结果的偏差 |
|---|---|---|
| 1 | `backend/internal/node/registry.go` 的 VLESS/VMess schema 仍含 legacy `ws-path/ws-headers`（VLESS 第 168 行附近），且没有 `when.network=ws`；`smux` 和 `packet-encoding` 也没有条件 | VLESS TCP/gRPC/H2 等连接区仍会出现“WebSocket Path/Headers”“多路复用”“包编码”等无关内容；Design4/Reference 预期只有当前传输的专用参数 |
| 2 | `registry.go` 将 `security`、SNI/ALPN/指纹/REALITY 对象归入 `auth` 组 | TLS/REALITY 身份参数跑到“认证与密钥”，不在“连接方式与当前参数” |
| 3 | `registry.go` 的 `setSecurityCondition(field, "tls", "reality")` 被用于 `skip-cert-verify/fingerprint` | REALITY 下也会显示“跳过证书校验/指纹”等不适用项；Design4 §3.3 明确证书校验类选项不能自动套用于 REALITY |
| 4 | `registry.go` 的 H2 `host` 是 `text-list`，HTTP `headers` 是普通 map；上游 `clash-verge-rev/src/types/global.d.ts` 中 H2Options.host 是 `string`、HttpOptions.headers 的值是 `string[]` | 表单允许的结构与上游客户端实际消费结构不一致 |
| 5 | `client-fingerprint` 只是普通 `text`，没有 `option_items`；`alpn` 没有推荐项；插件 `mode/version` 没有按插件提供候选 | 与 Design4 §4.3、Reference 中“浏览器指纹候选、ALPN 推荐+增删、插件 mode/version 按插件显示候选”不符 |
| 6 | Trojan 高级区仍有无条件 `reality-opts`；VMess 的 `reality-opts` 也无条件可见但服务端禁止保存 | 会出现“看得到/能填，但保存被拒或语义未验证”的矛盾控件 |
| 7 | 装配输出层没有调用 `ProjectActive`，只有节点检查草稿入口调用 | Design4 的服务端投影“权威”目前主要作用于保存/检查，尚未覆盖真实 Clash/SR/generic 装配输出 |

#### 6.1.3 解决方向

- 清理/隐藏 legacy 字段：`ws-path/ws-headers`、`tls bool` 不再作为新建/编辑主字段；旧值由归一化与诊断处理。
- 将 `security`、TLS/REALITY、传输 opts 放到 `connection`，并按 `network/security/plugin` 条件显示。
- 单独定义 TLS 与 REALITY 的条件：`skip-cert-verify/fingerprint` 只进 TLS，不套 REALITY。
- 按 CVR `global.d.ts` 修正字段类型：H2 host 用单值 text；HTTP headers 支持数组值；插件 version 用 number。
- 为 `client-fingerprint`、ALPN、插件 mode/version 等补推荐项与可编辑下拉/列表。
- 将装配输出的 `nodeData` 加载/渲染前也统一应用 `ProjectActive`，或至少为 legacy 存量数据补投影/回归夹具。

### 6.2 问题 2：文案描述不准确混乱

#### 6.2.1 根因

文案大多沿用了旧“凭据区/高级区”的说法，没有随 Build18/19 的条件表单模型更新；部分描述与实际代码行为不一致。

#### 6.2.2 已确认证据

| 位置 | 现状 | 问题 |
|---|---|---|
| `NodesView.vue` “认证与密钥” help | “凭据编辑时留空将保留原值” | 该区实际包含 SNI/ALPN/REALITY 等非凭据；新建时也没有“原值”；被重置后也不能留空保留 |
| `NodesView.vue` 未知扩展说明 | “仅在明确指定的目标中参与输出” | 当前没有任何输出适配器读取扩展负载，节点检查反而固定提示 `unknown_extension_not_rendered`；文案与实现不符 |
| `NodesView.vue` 连接方式 help | “按当前协议与传输组合动态展示” | 实际仍有 `ws-path/ws-headers/smux/packet-encoding` 等无条件字段，说明不准确 |
| `NodesView.vue` 独立开关 help | “只展示当前协议与组合适用的布尔开关” | 大量 `commonFieldSchema()` 的通用 bool 无条件出现在开关区；“当前组合适用”并不成立 |
| `registry.go` 的 `fingerprint` label | “TLS 指纹” | Design4 §4.3 将其定义为“证书指纹实际值输入”，与 `client-fingerprint`（浏览器/客户端指纹）不是同一语义 |
| `registry.go` 的 `encryption` label | “加密” | 在 VLESS 中容易与 VMess 的“加密方式/Cipher”混淆，且 VLESS generic 输出固定 `none`，缺少限制说明 |
| `ProtocolFieldEditor.vue` | `field.help` 只在普通 text/text-list 分支显示 | select/bool/number/combobox 的关键 help（如 Trojan network 的“h2/http/xhttp 可手填但非普通组合”）用户根本看不到 |
| `EditableCombobox.vue` | 直接回显原始 `value` | 用户看到 `none`/`aes-256-gcm` 等内部值，而不是“无/TLS/AES-256-GCM”等友好文案 |

#### 6.2.3 解决方向

- 统一文案术语：真正的凭据区叫“凭据/认证”，非凭据连接参数归连接区。
- “留空保留原值”只在编辑且未重置敏感凭据时显示；新建/重置后改为“未配置/必须填写”。
- 未知扩展文案先改为符合当前实现的“仅保存加密摘要并提示尚未透传”，或在真正支持输出后再写“参与输出”。
- 让 `help` 对所有控件类型渲染；把 raw value 显示改为 label 显示。

### 6.3 问题 3：UI 排版和结构混乱

#### 6.3.1 根因

分区逻辑、字段顺序、折叠层级三者都没有按 Design4/Reference 的“基础/连接/开关/高级/检查”收敛；多个 legacy/无条件字段又放大了视觉噪声。

#### 6.3.2 已确认证据

| # | 证据 | 具体表现 |
|---|---|---|
| 1 | `NodesView.vue` 固定先渲染“认证与密钥”，再渲染“连接方式” | 安全/TLS 参数被错误塞进第一个区，用户第一屏看到的不是连接组合 |
| 2 | `registry.go` 把 `security` append 到 schema 尾部 | TLS 参数会插在安全选择之前，控件顺序颠倒 |
| 3 | `registry.go` 的 VLESS/VMess `smux/packet-encoding/legacy ws-path/ws-headers` 无 `when` 或仍在 connection | “连接方式与当前参数”区出现不属于当前组合的大对象/旧字段 |
| 4 | `NodesView.vue` 开关区只做一层平铺 | 没有“常用开关 + 更多开关折叠”，TFO/MPTCP/UDP/XUDP 等全部挤在一起 |
| 5 | `NodesView.vue` 的“高级数据与目标检查”不独立 | 未知扩展塞在“更多功能/高级参数”里，`NodeCheckPanel` 在 `<details>` 外始终展开 |
| 6 | `ProtocolFieldEditor.vue` 对每个对象都渲染整块 bordered card，标量字段又不用 Form.Item 统一布局 | 对象嵌套时出现多层边框/卡片，字段行高不一致，help/error 展示不统一 |
| 7 | `commonFieldSchema()` 把 TFO/MPTCP 等 bool 追加到几乎所有协议 | 独立开关区成为跨协议大杂烩 |

#### 6.3.3 解决方向

- 重排分区：基础与凭据 → 连接方式（network/security/当前传输与安全参数）→ 独立开关（常用/更多）→ 兼容与性能（折叠）→ 高级数据与目标检查（折叠）。
- 隐藏 legacy 和未激活字段；把 smux、packet-encoding 等归到高级或功能开关条件区。
- 用统一字段行组件替代“每个 scalar 一块裸 div”，统一 help/error/焦点。
- 将“未知扩展 + 局部 JSON + 目标检查”放到独立可折叠区域，避免检查面板常驻。

### 6.4 问题 4：“未知拓展”模块意义不明

#### 6.4.1 根因

该模块从数据模型上看是“加密保存未知扩展块”，但当前没有输出适配器消费它；UI 又用 scope/targets/label/payload 等抽象字段让用户手工填写，缺少与“校验失败/未识别字段/实际产物”的关联。

#### 6.4.2 已确认证据

| 项 | 代码事实 |
|---|---|
| 保存 | `nodes.extensions_json` 保存加密块，读取只返回摘要（`backend/internal/node/node.go` 1398~1406） |
| 检查 | `backend/internal/node/check.go` 的 `extensionDiagnostics()` 只对命中 targets 的扩展输出 `warn: unknown_extension_not_rendered` |
| 输出 | `render_clash.go`、`render_sr.go`、`links/links.go`、`diagnose.go` 均没有读取扩展负载并渲染到产物 |
| 前端 | `NodesView.vue` 751~799 只提供“新增/替换/删除 + scope/targets/label/payload”表单；保存后无内容回显 |
| 关联 | 后端 `validateKnownTopLevel()` 报“请将其归入 extensions”，但前端没有从报错字段一键生成扩展的入口 |

#### 6.4.3 影响

- 用户填写“未知扩展”后看不到任何产物变化，只会在目标检查里看到一条“不会透传”的警告。
- “目标/负载/作用域”缺乏业务解释；不知道何时该用、填什么、保存后能否再查看。
- 看起来像“高级功能”，实际是“当前仅存档+警告”的黑盒。

#### 6.4.4 解决方向

- 先明确产品语义：要么实现目标适配器可消费扩展，要么在 UI 中如实表述为“当前仅保存和诊断，不进入任何产物”。
- 增加自动关联：当保存/校验发现未声明字段时，提示“可作为未知扩展保存”，并带入字段路径/JSON 片段。
- 增加可读性：允许替换时回显脱敏/结构化内容，至少展示负载类型、大小、影响目标。
- 将未知扩展 UI 移到“高级数据与目标检查”折叠区，而不是藏在“更多功能/高级参数”里。

### 6.5 问题 5：各类选项框中出现错误内容

#### 6.5.1 根因

选项框主要问题是：值回显没有绑定真实状态，而是 fallback 到 `field.default`；部分带 `option_items` 的字段没有走可编辑下拉；空值选项/原始 value/分组文本显示不友好。

#### 6.5.2 已确认证据

| # | 触发场景 | 证据 | 说明 |
|---|---|---|---|
| 1 | 编辑 VLESS/VMess TLS/REALITY 节点 | `protocolParamsForStorage()` 保存时删除 `security`，`openEdit()` 不回填 `current_state.security`，`ProtocolFieldEditor.vue` 用 `modelValue ?? field.default` | 安全下拉显示默认 `none`，与顶部“当前组合 TLS/REALITY”冲突 |
| 2 | SS cipher、VMess cipher、VLESS flow、Trojan 内层 SS method | registry 中这些字段是 `text` + `option_items`，但 `ProtocolFieldEditor.vue` 只在 `type === 'select' && option_items` 时使用 EditableCombobox | 推荐项实际不显示，用户看到的是普通文本输入 |
| 3 | EditableCombobox 显示值 | `EditableCombobox.vue` 直接把 `props.value` 作为 input value | 用户看到 `none`、`auto` 等内部值，而非“无”“Auto”等可读 label |
| 4 | 下拉分组 | `EditableCombobox.vue` 直接显示 `common/extended/legacy/pending` | 用户看到英文技术分组，不是“常用/扩展/旧版兼容/待验证” |
| 5 | SS 插件切回“不使用插件” | 当当前插件为 `obfs` 时，打开下拉默认过滤为空值选项；必须先清空输入才能选“不使用插件” | 空值选项缺少明确的清除/无插件按钮 |
| 6 | `client-fingerprint`、ALPN、plugin mode/version | registry 未提供 `option_items`，前端也没有对应列表/推荐组件 | 选项框为空或只能手输，容易拼错 |
| 7 | text-list/int-list | `ProtocolFieldEditor.vue` 在 array 和逗号 string 之间反复转换 | 保存形态不稳定，顺序/空值/转义易错 |
| 8 | 字段不存在于 protocol_json | 多数 scalar 使用 `modelValue ?? field.default` | 默认值被当成真实值显示，无法区分“未设置/展示默认/显式值” |

#### 6.5.3 解决方向

- 编辑回填时以 `current_state`/真实存储值为准，不使用默认值掩盖“未设置”。
- 让带 `option_items` 的 `text` 字段也进入可编辑下拉；或统一改为 `select`。
- 可编辑下拉显示 label，并在输入框文本中优先显示 label。
- 将分组文案本地化为常用/扩展/旧版兼容/待验证，或至少增加中文 group label 映射。
- 为“不使用插件/无”提供可点击的清除选项，空值选项不应依赖手动清空。
- 实现真正的 list 编辑器，避免 array/string 互转。

---

## 七、开放问题/需用户确认点

1. **是否调整 Design4 已定稿的“独立开关区”与 Reference 研究“把 UDP 等常用开关放回连接区”的差异？**
   - 本报告按 Design4 优先保留“独立开关区”，但建议在其内部做常用/更多折叠；若希望完全采用 Reference 5.2 的“开关放回所属分区”，需要用户拍板。
2. **`security` 是否改为真正持久化字段而不是仅 current_state/UI 虚拟字段？**
   - 当前为 UI 虚拟字段并落 `tls` 兼容；修复回显可不动后端。若后续 Xray outbound 等多目标扩展，可能需要重新评估是否保留独立 `security`。
3. **是否把本次发现单独立 Issue，还是作为 Build21 的改造范围？**
   - 建议先由用户确认，再按 AGENTS.md 文档体系进入 Issue/Design/Build。

---

## 八、验证记录

本次报告未修改代码。抽查命令：

```text
cd backend && go test ./internal/node ./internal/assembly ./internal/assembly/links ./internal/xray -count=1
# ok  vpn-sub/internal/node
# ok  vpn-sub/internal/assembly
# ok  vpn-sub/internal/assembly/links
# ok  vpn-sub/internal/xray

cd frontend && npm run build
# vue-tsc + vite build 通过
```

---

## 九、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-03 | 首次创建：Build17~Build20 表单结构与 Design4 对齐审查；发现安全字段分组/回显、开关区/高级区/列表控件等未达标项，输出改进建议。 |
| v2.0 | 2026-09-03 | 根据用户补充的 5 类问题逐项深挖：连接参数与调查结果不符、文案混乱、UI 结构混乱、未知扩展意义不明、选项框错误内容；补充代码证据与解决方向，未修改业务代码。 |
