# Build32.md — 19 个 manual 协议编辑体验完整化

> **文档定位：** 本文档是下一轮活动构建方案，承接 [Design4.md](Design4.md) 第九章“完成当前全部 19 个已兼容 manual 协议的编辑体验改进”目标，以及用户于 2026-09-21 对研究结论的最新确认。
> **当前状态：** 用户已授权实施并更正 Mihomo 源码以在线仓库 `https://github.com/MetaCubeX/mihomo` 为准；Step 0.5～3 已完成并提交在 `7ecb6d5aa0619d560fcbc786547cf95e0fef7080`（分支 `beta`），Step 3 遗留的前端静态颜色门禁缺口已由 Step 3-fix 修正，Step 3.5 公共字段类型增补、Step 4 HTTP、Step 5 SOCKS5、Step 6 SSH、Step 7 Snell、Step 8 Hysteria、9 Hysteria2、10 TUIC、11 标准 WireGuard、12 Mieru、13 MASQUE、14 Tailscale、15 AnyTLS、16 ShadowQUIC、**17 TrustTunnel**、**18 OpenVPN**、**19 `.ovpn` 只读解析导入** 已验收通过（legacy 待迁移协议 19→4；Step 19 不改变 legacy 计数）；固定 Mihomo v1.19.31 二进制已由用户重新提供官方 `go124` 发布资产并复验通过。用户已一次性授权 Step 17～20 严格串行实施（2026-09-23，并确认 TrustTunnel `connections` 分支要求 `max-connections` 必填／`min-streams` 可选、OpenVPN 枚举按项目侧收紧、不存在外部遗留 `client-config` 数据、OpenVPN 认证组切换采用新增可选字段属性方案）；当前恢复入口为 Step 20 目标诊断与正式装配收口；**Step 19 已完成，进入 Step 20 前不实施 Step 21～22，也不得归档本文档。**
> **编码约束：** [AGENTS.md](AGENTS.md) 是唯一强要求文档。实施时必须一次只执行一个 Step，逐步验收，不并行实施多个协议。
> **最新用户决策：** 新兼容基线为 Mihomo **v1.19.31**（commit `ab405bad5beeeac8b003bb01f60f134f6df54471`），不再把 v1.19.29 作为新设计兼容目标；允许协议级 endpoint policy；OpenVPN 使用“结构化编辑为主＋粘贴 `.ovpn` 解析导入”，不再把 `client-config` 直接作为 Mihomo 输出字段；WireGuard 只覆盖标准单 Peer／多 Peer，AmneziaWG 不纳入 Build32，留作后续独立专项。

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|---|---|---|
| 0.5 | 实施授权、状态复核与范围冻结 | ✅ 验收通过 |
| 1 | 固定 Mihomo v1.19.31 证据门禁并清除活动代码中的 v1.19.29 基线 | ✅ 验收通过 |
| 2 | `CurrentState` v2：通用 selector、条件、清空域与旧状态读取 | ✅ 验收通过 |
| 3 | 协议级 endpoint policy 与统一 Clash wire adapter 骨架 | ✅ 验收通过（静态颜色门禁缺口由 Step 3-fix 补齐） |
| 3.5 | 公共字段类型增补（`multiline`／`secret-multiline`／`byte-sequence`） | ✅ 验收通过 |
| 4 | HTTP 完整条件表单、TLS／认证与输出合同 | ✅ 验收通过 |
| 5 | SOCKS5 完整条件表单、TLS／认证／UDP 与输出合同 | ✅ 验收通过 |
| 6 | SSH 密码／私钥认证、Host Key 与多行凭据 | ✅ 验收通过 |
| 7 | Snell 版本、UDP／reuse 与五类 obfs 分支 | ✅ 验收通过 |
| 8 | Hysteria 认证、带宽、端口跳跃与 TLS 合同 | ✅ 验收通过 |
| 9 | Hysteria2 端口替代、混淆、Realm 与 QUIC 合同 | ✅ 验收通过 |
| 10 | TUIC v4／v5 认证互斥与 QUIC 合同 | ✅ 验收通过 |
| 11 | 标准 WireGuard 单 Peer／多 Peer、稳定凭据与 reserved | ✅ 验收通过 |
| 12 | Mieru 单端口／`port-range` 真互斥与枚举修复 | ✅ 验收通过 |
| 13 | MASQUE 网络模式、L3 参数与 UDP 限制 | ✅ 验收通过 |
| 14 | Tailscale 无 endpoint 模式、认证与状态目录安全边界 | ✅ 验收通过 |
| 15 | AnyTLS TLS 身份与 ShadowTLS／Restls／JLS 互斥 | ✅ 验收通过 |
| 16 | ShadowQUIC 认证、QUIC 版本、0-RTT 与流控 | ✅ 验收通过 |
| 17 | TrustTunnel TLS／QUIC／连接复用互斥 | ✅ 验收通过 |
| 18 | OpenVPN 结构化 schema、wire adapter 与敏感字段 | ✅ 验收通过 |
| 19 | `.ovpn` 只读解析 API、导入预览与前端应用草稿 | ✅ 验收通过 |
| 20 | 全 19 协议目标诊断、URI 支持／稳定 skip 与正式装配一致性 | ☐ 未开始 |
| 21 | 全协议前端交互、375px／桌面、草稿与错误定位回归 | ☐ 未开始 |
| 22 | 联合门禁、隔离浏览器 smoke、证据分层与文档归档 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。只有完成对应 Step 的实施记录与验收命令后才能标记为验收通过；本地验收通过不等同于已提交、联合门禁或真实客户端／人工验收通过。

---

## 二、研究结论与权威基线

### 2.1 固定版本与证据层级

本轮按以下优先级判定协议合同：

1. Mihomo `v1.19.31` tag，commit `ab405bad5beeeac8b003bb01f60f134f6df54471` 的 `adapter/outbound/*Option` 与构造校验；
2. Mihomo 官方协议文档，用于解释字段用途、默认值和用户可读说明；
3. 本地固定二进制 `Mihomo Meta v1.19.31 darwin arm64` 的 `-t` 正反例；
4. 项目自身 schema、wire adapter、`CheckClashContent`、URI 适配器和自动化测试；
5. 隔离 API／浏览器 smoke；
6. 真实客户端导入与连接人工验收。

固定 tag 源码与在线文档冲突时，以 tag 源码决定本 Build 的 wire shape；在线文档的新字段不得静默进入 `complete`。固定二进制 `-t` 只证明配置被内核接收，不替代项目结构检查，也不证明真实连接。

本轮研究已只读确认本机二进制输出：

```text
Mihomo Meta v1.19.31 darwin arm64 with go1.24.13 Mon Sep 14 13:27:07 UTC 2026
```

固定内核二进制身份（2026-09-23 由用户重新提供并经只读复验）：路径 `/Users/kyle/Desktop/Repo/Temp/mihomo-darwin-arm64-go124-v1.19.31`，来源为官方 release 资产 `mihomo-darwin-arm64-go124-v1.19.31.gz`（上游 asset digest `sha256:bc5d5208a94b5ab5089e9fdb23801bf20b3cbcac82c3dd7e872cf417c7c92237`，与本地 `.gz` 逐字节一致），解压后 SHA-256 `3ed36cabe89783acc24dbb3b411a9a0a6fb6353e97ea9d129a707c9bb9b147c5`，内嵌 `vcs.revision=ab405bad5beeeac8b003bb01f60f134f6df54471`、`vcs.time=2026-09-14T11:59:30Z`、`-tags=with_gvisor`（上游官方发布构建的既定 tag）。此前记录的 `~/mihomo-bins/mihomo-v1.19.31-go122`（解压后 SHA-256 `a756bc56…`）在本机已不存在，其记录保留为历史事实；本章输出串已同步为本步实际使用的 `go124` 资产。`.mihomo-test.sh` 只校验 `Mihomo`／`Meta`／`v1.19.31`，不校验 Go 版本，脚本逻辑无需改动。

主要上游依据：

- [Mihomo v1.19.31 release](https://github.com/MetaCubeX/mihomo/releases/tag/v1.19.31)
- [Mihomo 代理公共字段](https://wiki.metacubex.one/en/config/proxies/)
- [Hysteria](https://wiki.metacubex.one/en/config/proxies/hysteria/)、[Hysteria2](https://wiki.metacubex.one/en/config/proxies/hysteria2/)、[TUIC](https://wiki.metacubex.one/en/config/proxies/tuic/)
- [WireGuard](https://wiki.metacubex.one/en/config/proxies/wg/)、[Mieru](https://wiki.metacubex.one/en/config/proxies/mieru/)、[MASQUE](https://wiki.metacubex.one/en/config/proxies/masque/)、[Tailscale](https://wiki.metacubex.one/en/config/proxies/tailscale/)
- [HTTP](https://wiki.metacubex.one/en/config/proxies/http/)、[SOCKS](https://wiki.metacubex.one/en/config/proxies/socks/)、[SSH](https://wiki.metacubex.one/en/config/proxies/ssh/)、[Snell](https://wiki.metacubex.one/en/config/proxies/snell/)
- [AnyTLS](https://wiki.metacubex.one/en/config/proxies/anytls/)、[ShadowQUIC](https://wiki.metacubex.one/en/config/proxies/shadowquic/)、[TrustTunnel](https://wiki.metacubex.one/en/config/proxies/trusttunnel/)
- [OpenVPN](https://wiki.metacubex.one/en/config/proxies/openvpn/)、[TLS 公共字段](https://wiki.metacubex.one/en/config/proxies/tls/)

### 2.2 与 Design4 旧基线的关系

- Design4 中 v1.19.29 的 41 个配置检查、9 个 URI 样例和后续 Build21 证据保留为历史事实，不删除、不改写版本号。
- 用户 2026-09-21 的新决策覆盖 Design4 D16 对“后续新构建”的旧版本选择；Build32 活动代码、测试变量、诊断 evidence、推荐项 `verified` 和新夹具全部使用 v1.19.31。
- Build32 不宣称历史 v1.19.29 结果自动在 v1.19.31 通过；Step 1 必须重新执行首批四协议和 SS 插件的固定内核正反例。

### 2.3 当前代码根因

当前 19 个入口虽然全部走统一保存合同，但其余 15 个仍有以下共同缺口：

- `ConditionRule` 和 `CurrentState` 只有 network/security/plugin/features，不能表达认证模式、版本、混淆模式、endpoint 模式、Peer 模式等通用选择。
- `validateHostPort()`、前端基础信息表单和 `clashProxy()` 假设所有协议始终有 server/port。
- `clashProxy()` 除 SS 插件外基本透传 `protocol_json`，无法保证内部编辑模型与 Mihomo wire shape 相同。
- `all_protocols_test.go` 的统一 `test-value` 只证明存储往返，不证明协议语义、条件必填、互斥或固定内核接受。
- OpenVPN 当前 `client-config` 与 v1.19.31 的结构化 `OpenVPNOption` 不相符。
- SSH `host-key`／`host-key-algorithms` 当前是单文本，但 wire 是数组；`private-key` 既可能是内容也可能被 Mihomo 当作路径，项目不能允许任意主机路径。
- Tailscale 的 wire 根本没有普通 server/port；Mieru 明确要求 `port` 与 `port-range` 二选一；Hysteria2 的 `ports` 可替代 `port`；WireGuard 多 Peer 时顶层 server/port 被忽略。

---

## 三、冻结范围、排除项与停止条件

### 3.1 本 Build 包含

- 保持 manual 协议封闭清单为 19 项，完成其余 15 项的结构化条件编辑、保存重开、清空、敏感字段、Clash YAML wire adapter、检查与自动化证据。
- 将首批四协议的活动兼容元数据和固定二进制门禁统一升级到 v1.19.31。
- `CurrentState` 格式升级到 v2，并兼容读取 v1；旧状态只在内存派生 selector，不在读取时回写。
- 支持 Tailscale 无 host/port、Mieru 单端口／范围二选一、Hysteria2 单端口／端口组二选一、WireGuard 单／多 Peer 的 endpoint policy。
- OpenVPN 结构化编辑以及粘贴 `.ovpn` 到结构化草稿的解析预览。
- 对没有 URI 映射的协议返回稳定 `target_unsupported`／`skip`，不新增虚假的 URI。

### 3.2 本 Build 不包含

- 新增第 20 个协议、恢复 SSR、实现独立 Xray outbound、SS 2022 完整兼容或服务端配置生成。
- 任意 Xray JSON、WireGuard 配置文件或其他配置格式的全文反向导入。
- OpenVPN 全语法解释器、脚本／hook 执行、`include`、外部文件读取、多个远端故障转移或未在 v1.19.31 `OpenVPNOption` 中出现的指令透传。
- 从用户输入读取 SSH 私钥路径、OpenVPN 证书路径、Tailscale 任意绝对目录或任何逃出受控目录的路径；这些字段只接受内容或由系统派生安全相对路径。
- 将自动化、固定二进制 `-t`、隔离浏览器 smoke 表述为真实客户端连接验收。
- 删除 `backend/data`、Docker 卷、外部 `DATA_DIR`、备份或任何既有数据库。

### 3.3 已确认边界：AmneziaWG 不纳入

Mihomo v1.19.31 的 WireGuard option 还包含完整 `amnezia-wg-option`，且不同版本字段互斥复杂。用户已于 2026-09-21 确认采用方案 A：Build32 只完成 Design4 已点名的标准 WireGuard 单／多 Peer；AmneziaWG 不纳入本轮，留作后续独立专项。

| 项目 | 决策 |
|---|---|
| Build32 Step 11 | 只覆盖标准单／多 Peer、reserved、IP stack、DNS |
| AmneziaWG | 明确排除，不添加 v1.5/v2/v3/v3.1 字段、互斥、UI 或固定内核矩阵 |
| 后续处理 | 只有用户另行激活独立专项后才重新研究、设计和建立构建计划 |

### 3.4 全局停止条件

任一步出现以下情况即停止，不进入下一 Step：

- v1.19.31 tag 源码与固定二进制对同一 wire shape 结论冲突；
- 需要修改用户已确认的三项方向或扩大 19 协议边界；
- 协议分支切换会恢复已清空凭据、检查会写库、脱敏预览含明文敏感值；
- 正式装配和节点检查使用不同 wire adapter；
- 普通 `go test ./...` 被改成必须依赖本地 Mihomo 二进制；
- 发现需要清理或迁移既有数据库数据；
- 发现 Design／Build／Issue 与 AGENTS.md 存在未被最新用户决策解决的冲突。

---

## 四、公共数据合同定稿

### 4.1 `CurrentState` v2 与 selector

```go
type CurrentState struct {
    Network   string            `json:"network,omitempty"`
    Security  string            `json:"security,omitempty"`
    Plugin    *string           `json:"plugin"`
    Features  []string          `json:"features,omitempty"`
    Selectors map[string]string `json:"selectors,omitempty"`
}

type ConditionRule struct {
    Network   []string            `json:"network,omitempty"`
    Security  []string            `json:"security,omitempty"`
    Plugin    []string            `json:"plugin,omitempty"`
    PluginNot []string            `json:"plugin_not,omitempty"`
    Features  []string            `json:"features,omitempty"`
    Selectors map[string][]string `json:"selectors,omitempty"`
    Targets   []string            `json:"targets,omitempty"`
}
```

`FieldSchema` 增加：

- `selector_name`：把当前字段的规范值投影到 `CurrentState.selectors[name]`；允许嵌套字段路径。
- `state_only`：仅用于表单选择与状态，不写入 Mihomo wire；例如 `auth-mode`、`endpoint-mode`、`peer-mode`。
- `multiline`／`secret-multiline` 明确类型，停止依赖字段名猜测大文本。
- `byte-sequence`：WireGuard reserved 专用；UI 接受 3 个 0–255 整数或 Base64，保存前统一为 3 字节数组。

selector 名称必须由当前协议 schema 声明，格式为小写字母、数字和下划线；请求不得提交任意 selector。清空域增加 `selector.<name>`。A→B→A 仍不恢复旧分支参数、凭据、扩展或未应用草稿。

`currentStateFormatVersion` 升为 2。读取 v1 时按协议参数派生 selectors；成功保存后以 v2 写回。不得增加整库回填迁移，也不得读取时写库。

selector 注册与持久化规则：

- `Protocol` 必须声明 selector 名称、允许值、默认值和来源字段；注册时检查重名、空允许值、默认值不在允许集合、`selector_name` 指向未声明 selector 等错误。
- v2 中 `current_state.selectors` 是 `state_only` 选择的持久化权威；普通 wire 字段派生的 selector 必须与 `protocol_json` 一致，二者不一致返回 400，不做“任选一边”的修复。
- 新建节点未提交 selector 时使用注册表默认值；读取 v1 时按下表先从现有参数确定，无法唯一判断时使用默认值并附加只读迁移 diagnostic；若现有参数同时命中互斥分支，则阻断保存／检查并要求用户明确选择。
- `state_only` 字段不得写入 `protocol_json`、Clash YAML、URI、未知扩展或检查预览；它只通过 `current_state_json` 保存。
- selector 切换必须同时进入 `reset_scopes`；后端根据旧状态和新状态复核所需 scope，不能只相信前端声明。

| 协议 | selector | 允许值 | 新建／v1 无线索默认 |
|---|---|---|---|
| HTTP、SOCKS5 | `auth_mode` | `none/basic` | `none`；存在 username/password 时派生 `basic` |
| SSH | `auth_mode` | `password/private_key` | 有 private-key 时为 `private_key`，否则 `password` |
| Snell | `version`、`obfs_mode` | `1/2/3/4/5`；`none/http/tls/shadow_tls/restls/jls` | tag 默认版本；无 obfs 对象时 `none` |
| Hysteria | `auth_mode` | `none/base64/string` | 有 `auth` 为 `base64`，有 `auth-str` 为 `string`，均无值为 `none` |
| Hysteria2 | `endpoint_mode`、`obfs_mode` | `single/ports`；`none/salamander/gecko` | 有 ports 时 `ports`，否则 `single`；无 obfs 时 `none` |
| TUIC | `auth_mode` | `v4/v5` | token 为 `v4`，UUID/password 为 `v5`；均无值时按 tag 默认版本 |
| WireGuard | `peer_mode` | `single/peers` | peers 非空时 `peers`，否则 `single` |
| Mieru | `endpoint_mode` | `single/range` | 有 port-range 时 `range`，否则 `single` |
| MASQUE | `network_mode` | `quic/h2/h3_l4proxy` | 空值为 `quic` |
| AnyTLS | `security_mode` | `plain/shadow_tls/restls/jls` | 无附加安全对象时 `plain` |
| TrustTunnel | `reuse_mode` | `none/connections/streams` | 无复用参数时 `none` |
| OpenVPN | `auth_mode`、`tls_key_mode` | `userpass/cert/cert_userpass`；`none/tls_auth/tls_crypt/tls_crypt_v2` | 只有 username/password 为 `userpass`，只有 cert/key 为 `cert`，两组均完整为 `cert_userpass`；两组均空或任一组不完整时阻断 |

Tailscale、ShadowQUIC 不为“只有一个合法分支”的维度制造 selector；普通 feature 继续使用 `features`。

### 4.2 Endpoint policy

`Protocol` 增加有序 `EndpointPolicies`：

```go
type EndpointPolicy struct {
    When     *ConditionRule `json:"when,omitempty"`
    HostMode string         `json:"host_mode"` // required|optional|hidden
    PortMode string         `json:"port_mode"` // required|optional|hidden
    EmitHost bool           `json:"emit_host"`
    EmitPort bool           `json:"emit_port"`
}
```

合同：

- 每个协议／当前 selector 必须且只能命中一条 policy；无匹配或多匹配是注册表错误。
- `hidden` 字段在 UI 不显示，请求值规范化为空字符串／0，输出禁止生成 `server`／`port`。
- `optional` 不是绕过语义校验；替代字段通过 `required_when` 和协议组合校验保证。
- 数据库 `nodes.host`／`port` 继续保持 `NOT NULL`；“无 endpoint”分别存 `''`／`0`，无需 schema migration。
- 列表和移动端对无 endpoint 节点显示“协议自身管理”，不得显示 `:0`。
- create／update／check 共用同一 `ValidateEndpoint`；URI 导入仍只接受带合法 endpoint 的 URI，不能借 endpoint policy 创建 Tailscale 等无 URI 协议。
- policy 从注册表和 selector 计算，不接受请求直接提交；否则客户端可伪造 `hidden` 绕过普通协议必填。

| 协议／模式 | host | port | 输出 |
|---|---|---|---|
| 普通协议 | required | required | `server`＋`port` |
| Hysteria2 `single` | required | required | `server`＋`port` |
| Hysteria2 `ports` | required | hidden | `server`＋`ports` |
| WireGuard `single` | required | required | 顶层 `server`＋`port` |
| WireGuard `peers` | hidden | hidden | 只输出 `peers[]` |
| Mieru `single` | required | required | `server`＋`port` |
| Mieru `range` | required | hidden | `server`＋`port-range` |
| Tailscale | hidden | hidden | 不输出普通 endpoint |

Hysteria v1 即使配置 `ports`，固定 tag 仍要求基本 `port`；不得套用 Hysteria2 的替代规则。

### 4.3 内部模型到 Mihomo wire adapter

Clash YAML 不再由 `clashProxy()` 对 15 个协议直接 map 透传。新增按协议注册的纯函数 adapter：

```go
type ClashNodeDraft struct {
    NodeID    int64
    Persisted bool
    Protocol string
    Name     string
    Host     string
    Port     int
    State    node.CurrentState
    Params   map[string]any
}

type ClashProtocolAdapter func(ClashNodeDraft) (map[string]any, []node.TargetDiagnostic, error)
```

要求：

- 正式装配与 `/api/admin/nodes/check` 调用同一 adapter。
- adapter 只处理已由 schema 验证并经 `ProjectActive()` 投影的副本，不回写数据库参数。
- `NodeID`／`Persisted` 由服务端根据节点生命周期填充，不接受请求伪造：已保存节点的 check 和正式装配必须携带同一正整数 ID；未保存的新建草稿固定为 `NodeID=0`、`Persisted=false`。
- 只有确需稳定运行身份的 adapter 可以消费 `NodeID`。Tailscale 已保存节点据此派生 `tailscale/node-<id>`；新建草稿 check 不生成 `state-dir`，只验证保存后可以派生，正式装配若缺少稳定 ID 必须阻断。
- `state_only`、稳定条目 ID、内部导入元数据、非活动字段不得进入 wire。
- adapter 返回字段级诊断；阻断错误禁止正式输出，warn 不得伪装为 complete。
- 最终 YAML 继续执行 `CheckClashContent`，随后由固定 v1.19.31 `-t` 门禁验证代表性正反例。

### 4.4 OpenVPN `.ovpn` 导入合同

新增管理员只读端点：

```text
POST /api/admin/nodes/openvpn/parse
Content-Type: application/json
{"text":"<粘贴的 .ovpn 内容>"}
```

响应只返回结构化草稿、来源行号和 diagnostics，不创建／更新节点：

```json
{
  "host": "vpn.example.com",
  "port": 1194,
  "protocol_json": {"proto":"udp", "ca":"..."},
  "diagnostics": [{"severity":"warn", "code":"ovpn_unsupported_directive", "line":12}]
}
```

解析边界：

- 后端和前端均限制 256 KiB；超过返回 413。
- 路由必须先设置 `Cache-Control: no-store` 再经过 session／admin 中间件，保证成功、400、401、403、413、500 均不可缓存；只接受 `application/json`。
- 支持 v1.19.31 `OpenVPNOption` 对应的结构化指令和 `<ca>`、`<cert>`、`<key>`、`<tls-auth>`、`<tls-crypt>`、`<tls-crypt-v2>` 内嵌块。
- `remote host [port]` 填充顶层 host/port；多个不同 remote 阻断，不静默选第一项。
- `auth-user-pass` 只确定认证模式；引用本地文件时不读取文件，用户名／密码保持待填写。
- 外部证书／私钥路径、脚本、hook、`include`、inline 之外的文件引用一律拒绝或报告不支持，绝不读取服务器文件。
- 未识别指令只产生结构化 diagnostic，不存入未知扩展，也不原样透传到 YAML。
- 预览和日志对所有内嵌私钥、密码、tls key 脱敏；解析失败不得记录原文。
- 用户点击“应用解析结果”后才覆盖当前草稿；应用前后均不落库，最终仍需普通保存／检查。
- 解析响应可以把内嵌 secret 返回给本次已鉴权管理页面以供显式应用，但不得在 `diagnostics.message`、结构化日志、错误响应、check preview 或浏览器持久化存储中复制；前端关闭面板、切换协议或离开页面时立即丢弃原文和未应用 secret。
- 状态码固定为：JSON／语法错误以及脚本、hook、外部文件读取等危险指令 400，未认证 401，非管理员 403，超限 413，意外内部错误 500；可安全忽略的未知普通指令使用 200＋warn diagnostic，仅当草稿可安全展示且没有阻断错误时返回草稿。
- diagnostic code 至少冻结 `ovpn_unsupported_directive`、`ovpn_external_file_forbidden`、`ovpn_multiple_remotes`、`ovpn_unclosed_inline_block`、`ovpn_conflicting_auth`、`ovpn_conflicting_tls_key`，并带稳定 `severity`、`line`、`field_path`；前端不得解析中文 message 决定行为。

---

## 五、15 个后续协议的冻结矩阵

| 协议 | selector／功能分支 | 关键必填与互斥 | 新增敏感路径 | 输出／目标边界 |
|---|---|---|---|---|
| HTTP | `auth_mode=none/basic`、TLS feature | basic 时 username/password 成对；TLS 开启才允许 SNI、证书、mTLS | `password`、`private-key` | Clash 完整；既有 URI 只输出可表达字段 |
| SOCKS5 | `auth_mode=none/basic`、TLS feature | basic 成对；TLS 条件；UDP 独立 | `password`、`private-key` | Clash 完整；既有 URI 对 TLS/mTLS 不可表达部分诊断 |
| SSH | `auth_mode=password/private_key` | username 必填；私钥只接受 PEM 内容；passphrase 仅私钥模式 | `password`、`private-key`、`private-key-passphrase` | Clash 完整；SR/generic 稳定 skip |
| Snell | `version=1..5`、`obfs_mode=none/http/tls/shadow-tls/restls/jls` | v1/2 禁 UDP；v2 固定 reuse，v4/v5 可编辑；各 obfs 凭据条件必填 | `psk`、obfs password/private-key、JLS password | Clash 完整；SR/generic 稳定 skip |
| Hysteria | `auth_mode=none/base64/string` | 两种认证互斥；`up/down` 规范字符串成对必填；port 始终必填 | `auth`、`auth-str`、`obfs`、`private-key` | Clash 完整；既有 URI 仅可无损子集 |
| Hysteria2 | `endpoint_mode=single/ports`、`obfs_mode=none/salamander/gecko`、Realm feature | password 必填；启用 obfs 时密码必填；包大小仅 gecko；Realm 子字段条件化 | `password`、`obfs-password`、`private-key`、`realm-opts.token`、`realm-opts.private-key` | Clash 完整；既有 URI 对 Realm 等不可表达项诊断 |
| TUIC | `auth_mode=v4/v5` | v4 只允许 token；v5 只允许 UUID＋password；切换清空旧凭据 | `token`、`uuid`、`password`、`private-key` | Clash 完整；既有 URI 只按当前版本分支输出 |
| WireGuard | `peer_mode=single/peers` | private-key 和至少一个本地 IP 必填；多 Peer 每项 allowed-ips 必填且不可冲突；reserved 恰 3 字节 | `private-key`、`pre-shared-key`、`peers[].pre-shared-key` | Clash 完整；已有 URI 仅支持可表达子集；AmneziaWG 明确排除 |
| Mieru | `endpoint_mode=single/range` | port 与 port-range 严格二选一；transport 仅 TCP/UDP；枚举使用完整上游常量 | `password` | Clash 完整；SR/generic 稳定 skip |
| MASQUE | `network_mode=quic/h2/h3-l4proxy` | private/public key、本地 ip/ipv6 至少一个；h3-l4proxy 禁 UDP | `private-key` | Clash 完整；SR/generic 稳定 skip |
| Tailscale | 无普通 endpoint；auth feature | auth-key 可空但返回交互登录 warn；exit-node LAN 开关只有 exit-node 时活动 | `auth-key` | Clash 完整；SR/generic 稳定 skip；state-dir 系统派生 |
| AnyTLS | `security_mode=plain/shadow_tls/restls/jls` | 三种伪装互斥；TLS mTLS 成对；不开放 Reality | `password`、`private-key`、三种伪装密码 | Clash 完整；既有 URI 只输出可表达子集 |
| ShadowQUIC | QUIC feature 组合 | username/password；QUIC versions v1/v2；0-RTT 显示风险提示 | `password` | Clash 完整；SR/generic 稳定 skip |
| TrustTunnel | QUIC feature、`reuse_mode=none/connections/streams` | username/password 成对；max-connections/min-streams 与 max-streams 冲突 | `password`、`private-key` | Clash 完整；SR/generic 稳定 skip |
| OpenVPN | `auth_mode=userpass/cert/cert_userpass`、`tls_key_mode=none/tls_auth/tls_crypt/tls_crypt_v2` | CA 必填；userpass、cert/key 或二者组合至少一组完整；三种 tls key 互斥；tls-auth 关联 key-direction | `password`、`key`、`tls-auth`、`tls-crypt`、`tls-crypt-v2` | Clash 完整；SR/generic 稳定 skip；`.ovpn` 仅解析为草稿 |

补充约束：

- Hysteria2 `hop-interval` 是字符串，可表达 `15-30`，不再使用 number。
- Mieru 枚举必须为 `MULTIPLEXING_OFF/MULTIPLEXING_LOW/MULTIPLEXING_MIDDLE/MULTIPLEXING_HIGH` 和 `HANDSHAKE_STANDARD/HANDSHAKE_NO_WAIT` 的完整常量名；当前 `LOW/MIDDLE/HIGH` 是错误 wire 值。
- SSH `host-key`／`host-key-algorithms` 改为结构化列表；私钥输入包含 PEM 标志才进入 wire，禁止把用户文本当服务器路径读取。
- AnyTLS 的 `client-metadata` 和 `disable-reuse` 属于 v1.19.31 tag 合同，纳入高级区。
- MASQUE `name-cert-verify` 在 tag 源码中只是 placeholder，不作为可编辑已支持字段。
- Tailscale `state-dir` 不提供任意路径输入；adapter 使用服务端注入的稳定 `NodeID` 派生安全相对目录（例如 `tailscale/node-<id>`），避免重命名改变身份或多个节点共享默认目录。已保存节点的 check 与正式装配必须使用同一 ID；未保存的新建草稿固定为 `NodeID=0/Persisted=false`，只显示“保存后分配”，不得输出占位 `state-dir`、启动 tsnet 或制造临时状态目录。
- 所有证书私钥和多行 secret 使用 `secret-multiline`；普通证书／CA 使用 `multiline`，预览仍按路径脱敏。

---

## 六、影响评估与文件清单

| 模块 | 影响 | 预计文件 |
|---|---|---|
| 协议 schema | selector、endpoint policy、15 协议字段与敏感路径 | `backend/internal/node/registry.go`、新增 `registry_extended.go`、`schema.go` |
| 状态／保存／清空 | v2 selectors、协议感知 endpoint、旧 v1 读取 | `backend/internal/node/node.go`、`normalize.go`、`project.go`、`features.go` |
| 节点检查 | endpoint policy、相同 adapter、稳定 skip/evidence | `backend/internal/node/check.go`、`backend/internal/assembly/node_check.go` |
| Clash 输出 | 15 协议显式 wire adapter | `backend/internal/assembly/render_clash.go`、新增 `clash_protocols.go` |
| URI 输出 | 既有映射按活动字段收口；无映射稳定 skip | `backend/internal/assembly/links.go`、`backend/internal/assembly/links/links.go` |
| OpenVPN 导入 | 有界解析、只读 API、脱敏诊断 | 新增 `backend/internal/node/openvpn_import.go`、`backend/internal/server/node.go` |
| 固定内核门禁 | 1.19.31 变量、全协议代表性正反例 | `.mihomo-test.sh`、`backend/internal/assembly/mihomo_ssplugin_test.go`、新增固定版本测试／夹具 |
| 前端 API／状态 | selector、endpoint policy、OpenVPN parse 类型 | `frontend/src/api/node.ts` |
| 前端表单 | 动态 endpoint、当前组合、导入草稿、列表显示 | `frontend/src/views/admin/NodesView.vue`、`ProtocolFieldEditor.vue`、新增 `OpenVPNImportPanel.vue` |
| 前端工具 | selector 匹配、状态推导、清空和测试 | `frontend/src/utils/nodeFormLayout.ts`、`nodeFeatures.ts` |
| 自动化 | 每协议正反分支、敏感字段、输出与 UI | node／assembly／server 对应 `_test.go`、`frontend/tests/*`、`backend/internal/assembly/testdata/node_check/` |

影响处理：

- 不新增数据库列；`state_format_version=2` 仍使用现有列。
- `host=''`、`port=0` 只对命中 policy 的 manual 协议合法；Xray 来源与普通协议仍保持原校验。
- 跨协议切换不能一概“保留服务器和端口”：目标协议 policy 隐藏 endpoint 时必须清空草稿值，切回不恢复；相应更新现有警告文案。
- 历史装配快照不重写；重新装配才得到新 adapter 产物。
- `docs/reports/` 历史文档不改写；完成后只同步当前事实并归档 Build32。

### 6.1 公共产出文件边界

- 只在职责确实独立时新增 `registry_extended.go`、`clash_protocols.go`、`openvpn_import.go`、`OpenVPNImportPanel.vue`；不得把现有逻辑机械拆成大量单协议文件。
- schema／状态／保存规则属于 `backend/internal/node/`，wire shape 属于 `backend/internal/assembly/`，HTTP 只做限流、绑定和错误映射；不得在 Handler 或 Vue 页面复制协议校验。
- 测试可按协议建立表驱动文件，但固定内核用例必须继续由 `.mihomo-test.sh` 统一版本校验后进入 Go 测试，不能由各测试自行寻找任意 `mihomo`。
- 下表是每个 Step 的最小产出边界；实施时若新增文件，必须在对应 Step 完成记录中补入，若不需要预计新增文件也要记录“复用现有文件”。

### 6.2 Step 产出文件与完成定义

| Step | 前置 | 最小产出文件 | 本步完成定义 |
|---|---|---|---|
| 0.5 | 用户明确实施授权 | `Build32.md` | HEAD、既有改动、二进制、遗留 OpenVPN 行和数据目标均已只读复核；本 Step 标为完成后才进入 Step 1 |
| 1 | 0.5 | `.mihomo-test.sh`、`backend/internal/assembly/{mihomo_ssplugin_test.go,node_check.go}`、`backend/internal/node/registry.go`、相关前端测试 | 活动基线全部改为 1.19.31，历史文档未改写，首批四协议与 SS 插件重新通过 |
| 2 | 1 | `backend/internal/node/{schema.go,registry.go,node.go,normalize.go,project.go}`、`frontend/src/api/node.ts`、`frontend/src/utils/{nodeFeatures.ts,nodeFormLayout.ts}` 及定向测试 | v1 只读派生、v2 保存、selector 白名单与后端复核、A→B→A 清空全链通过 |
| 3 | 2 | `backend/internal/node/{registry.go,node.go,check.go}`、`backend/internal/assembly/{render_clash.go,node_check.go,clash_protocols.go}`、`frontend/src/views/admin/NodesView.vue` 及测试 | endpoint policy 不能由请求伪造；check／正式装配共用 adapter；未迁移协议显式标为 legacy adapter |
| 4 | 3 | 协议注册表、adapter、node／assembly／server／frontend HTTP 测试 | HTTP 全分支、清空、凭据和固定内核矩阵通过 |
| 5 | 4 | 同上，SOCKS5 测试 | SOCKS5 全分支和 URI 降级诊断通过 |
| 6 | 5 | 同上，SSH 测试 | 私钥内容边界、Host Key 列表与稳定 skip 通过 |
| 7 | 6 | 同上，Snell 测试 | 版本／五类 obfs／reuse 矩阵与固定内核通过 |
| 8 | 7 | 同上，Hysteria 测试 | 认证、带宽、TLS 和端口跳跃矩阵通过 |
| 9 | 8 | 同上，Hysteria2 测试 | 端口替代、obfs、Realm 和清空矩阵通过 |
| 10 | 9 | 同上，TUIC 测试 | v4/v5 凭据隔离与固定内核正反例通过 |
| 11 | 10 | 同上，WireGuard 测试 | 标准单／多 Peer、稳定 `_credential_id`、reserved 通过，仓库无新增 AmneziaWG 活动字段 |
| 12 | 11 | 同上，Mieru 测试 | endpoint 二选一、完整枚举和 Base64 校验通过 |
| 13 | 12 | 同上，MASQUE 测试 | 三种 network、地址／密钥和 UDP 互斥通过 |
| 14 | 13 | 同上，Tailscale 测试 | 无 endpoint、已保存 check／装配共用稳定 NodeID 派生 state-dir、新建草稿无 state-dir、空 auth-key 警告且检查零副作用通过 |
| 15 | 14 | 同上，AnyTLS 测试 | 三类附加安全互斥、TLS 与主密码生命周期通过 |
| 16 | 15 | 同上，ShadowQUIC 测试 | QUIC／0-RTT／流控及风险提示通过 |
| 17 | 16 | 同上，TrustTunnel 测试 | TLS／QUIC／复用互斥通过 |
| 18 | 17 | `backend/internal/node/registry_extended.go`（若采用）、`backend/internal/assembly/clash_protocols.go`、OpenVPN node／assembly 测试 | 结构化 schema 与 adapter 通过；`client-config` 不再进入任何输出，遗留行边界已处理 |
| 19 | 18 | `backend/internal/node/openvpn_import.go`、`backend/internal/server/node.go`、`frontend/src/api/node.ts`、`OpenVPNImportPanel.vue` 及测试 | 256 KiB、no-store、鉴权、危险指令、secret 生命周期和零落库通过 |
| 20 | 19 | `backend/internal/assembly/{render_clash.go,node_check.go,links.go,links/links.go}`、`CheckClashContent` 与测试夹具 | 15 个 legacy adapter 计数归零；19 协议 check／正式装配一致；8 个 URI skip 稳定 |
| 21 | 20 | `NodesView.vue`、`ProtocolFieldEditor.vue`、`NodeCheckPanel.vue`、前端工具和测试 | 19 协议桌面／375px、草稿阻断、错误定位、明暗主题回归通过 |
| 22 | 21 | `Build32.md`、`Design4.md`、`AGENTS.md`、必要的人工清单 | 联合门禁和隔离 smoke 完成，证据分层记录，文档链接通过后归档 |

### 6.3 所有代码 Step 的共同完成门槛

除各 Step 点名的验收外，Step 1～21 均须满足：

1. 本步新增逻辑有失败优先的正反单元测试；相关包测试、对应前端测试和 `git diff --check` 通过。
2. 本步结束时后端可编译；涉及前端生产代码时 `npm run build` 通过。不得把编译失败留给下一 Step。
3. 敏感字段覆盖 create／update／get／list／check／日志／错误文本／正式输出；检查前后数据库业务表快照一致。
4. schema、活动投影、组合校验、credential keep／replace／clear、reset scope、adapter、目标 evidence 和 UI 使用同一字段路径。
5. 实际文件、命令、测试数量、未执行证据和偏差写回本 Step；失败即保持当前 Step 未通过，不提前启动下一 Step。

---

## 七、串行依赖图

```text
Step 0.5 授权/冻结
  → Step 1 固定 1.19.31 门禁
  → Step 2 selector/state v2
  → Step 3 endpoint/adapter 公共骨架
  → Step 4 HTTP → 5 SOCKS5 → 6 SSH → 7 Snell
  → Step 8 Hysteria → 9 Hysteria2 → 10 TUIC
  → Step 11 WireGuard → 12 Mieru → 13 MASQUE → 14 Tailscale
  → Step 15 AnyTLS → 16 ShadowQUIC → 17 TrustTunnel
  → Step 18 OpenVPN 模型 → 19 .ovpn 导入
  → Step 20 目标/正式装配收口
  → Step 21 前端全协议回归
  → Step 22 联合门禁/证据/归档
```

每个协议 Step 结束后必须保持全仓可编译、相关定向测试通过；不得先批量修改 15 个 schema，再到最后一次性补测试。

---

## 八、分步实施与验收

### Step 0.5：实施授权、状态复核与范围冻结

- **前置：** 用户另行明确授权“开始实施 Build32”；当前范围已冻结，AmneziaWG 无需再次决策。
- 复核 `git status --short --branch`、当前 HEAD、Build32 版本和 Mihomo 二进制版本。
- 重新搜索活动代码中的 `1.19.29`、`MIHOMO_11929_BIN`、`client-config`、统一 `validateHostPort` 和无条件 `server/port` 输出。
- 只读查询本次获授权实施目标中是否存在 `protocol='openvpn'` 且含 `client-config` 的既有节点；默认最多检查当前仓库 `backend/data`，不得自行连接外部 `DATA_DIR`、Docker 卷或生产环境。目标不明确或发现遗留行时只记录数量、不回显正文，并按 Step 18 的停止条件处理。
- 不删除本地数据库，不启动任何代码修改前先记录影响清单。
- **字段逻辑：** 本 Step 只读盘点 `nodes.protocol/host/port/protocol_json/state_format_version/current_state_json/extensions_json`，统计 19 个 manual 协议的行数、v1/v2 状态数量、空 endpoint、未知顶层键和 OpenVPN `client-config` 遗留数量；只记录计数和字段名，不输出任何字段值、凭据或扩展 payload。盘点不得规范化、回写、迁移或触发凭据解密。
- **验收：** 工作区既有改动已识别且不会覆盖；没有未决范围冲突；影响清单与遗留数据结果已写回本文。复核完成后把 Step 0.5 标为验收通过，再单独进入 Step 1。

**实施记录（2026-09-22）**

- 用户已明确授权开始实施 Step 0.5～22，并更正 Mihomo 源码以在线仓库 `https://github.com/MetaCubeX/mihomo` 为准。
- Git：分支 `beta`，HEAD `f00b350c127773b9ee57e4c77c7d98d23121c859`，跟踪 `origin/beta`（ahead/behind 0/0），工作区干净，无 staged/unstaged/untracked；最近 4 个提交均只修改本文件。
- 已阅读 `AGENTS.md`、本文件 v1.4、`Design4.md` 节点编辑器/保存合同/CurrentState/目标兼容相关章节、当前 `TODOLIST.md`、`ProdTestList.md`，以及 Build32 明确引用的 `docs/reports/Build/Build21.md`、`docs/reports/Issue/Issue13.md`、`docs/reports/Issue/Issue14.md` 相关记录。
- 固定二进制：`/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo` 自报 `Mihomo Meta v1.19.31 darwin arm64`，二进制内嵌提交 `ab405bad5beeeac8b003bb01f60f134f6df54471`。
- 在线源码：`git ls-remote --tags https://github.com/MetaCubeX/mihomo.git refs/tags/v1.19.31` 返回 `ab405bad5beeeac8b003bb01f60f134f6df54471 refs/tags/v1.19.31`；随后以 `--depth 1 --branch v1.19.31` 只读克隆到仓库外临时目录 `/tmp/mihomo-v1.19.31`，HEAD 与 tag 一致。原先提示的本地 `/Users/kyle/Desktop/Repo/clash-verge-rev` 仅为 Clash Verge Rev 仓库，其本地文件不作为 Mihomo 源码依据。
- 数据目标：只读检查 `backend/data`；该目录为空，无 `app-dev.db`/`app-prod.db`/其他数据库文件，因此无法执行 nodes 表 SQL 统计。授权目标内可见计数为：数据库文件 0、manual 协议节点行 0、state v1/v2 行 0、空 endpoint 行 0、未知顶层键 0、OpenVPN `client-config` 遗留行 0。未连接或读取任何外部 `DATA_DIR`、Docker 卷、生产/预发布环境或备份。
- 活动代码盘点：`1.19.29`/`MIHOMO_11929_BIN` 出现在 `.mihomo-test.sh`、`backend/internal/assembly/mihomo_ssplugin_test.go`、`backend/internal/assembly/node_check.go`、`backend/internal/node/check.go`、`backend/internal/node/project_test.go`、`backend/internal/node/registry.go`、`frontend/tests/editable-combobox.spec.ts`；活动代码无 `1.19.31`/`MIHOMO_11931_BIN`。`client-config` 仍存在于 `backend/internal/node/node_test.go`、`backend/internal/node/registry.go` 与前端 `ProtocolFieldEditor.vue` 的文本分支特判中。`validateHostPort()` 在 create/update/check/URI 导入四处调用；`clashProxy()` 仍无条件写 `name/type/server/port`，除 SS 插件外基本透传 `protocol_json`。
- 影响评估：Step 1 只替换活动证据版本与门禁变量，不修改字段集合；后续 Step 2～22 的影响范围与本文第六、八章一致。未发现中断实施成果或无法安全合并的工作区改动。
- 停止条件：未触发。在线 tag 源码、本地固定二进制与 Build32 冻结矩阵的当前核对未发现冲突；数据目标未发现遗留行；普通测试仍可在缺少外部二进制时显式 SKIP，严格门禁保持显式外部入口。
- 本 Step 未修改业务代码、测试、Design4/AGENTS 或数据；下一步从 Step 1 开始，严格串行实施。



### Step 1：固定 Mihomo v1.19.31 证据门禁

- 把活动门禁变量改为 `MIHOMO_11931_BIN`，测试函数和错误文本同步版本；不保留同时接受两个版本的 fallback。
- 把活动代码、`TargetEvidence`、diagnostic evidence、前端 verified label 从 v1.19.29 升到 v1.19.31。
- 历史 Design／Build／Issue 中的 v1.19.29 字样不改。
- 首先复跑首批四协议与 SS 插件正反例，确认升级没有静默语义变化。
- 普通 Go 测试未设置外部二进制时仍可 skip；`.mihomo-test.sh` 未设置或版本错误必须失败。
- **字段逻辑：** 本 Step 不改变协议字段集合，只更新字段证据元数据：`OptionItem.verified`、`TargetEvidence.version/entry/status`、检查 diagnostic 的 `evidence` 和固定内核环境变量。历史值只在活动注册表／运行代码中从 `mihomo-1.19.29` 改为 `mihomo-1.19.31`；不得借版本替换改变字段默认值、枚举、必填性、敏感路径或 wire key。若固定 tag 对首批四协议或 SS 插件显示真实字段差异，停止并先把差异补入对应字段合同，不能夹带在版本字符串替换中。
- **验收命令：**
  ```bash
  MIHOMO_11931_BIN='/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo' ./.mihomo-test.sh
  cd backend && go test ./internal/node ./internal/assembly
  cd ../frontend && npm test -- --run editable-combobox node-form-layout nodes-view
  cd .. && git diff --check
  ```


**实施记录（2026-09-22）**

- 活动基线全部改为 `MIHOMO_11931_BIN` / `mihomo-1.19.31`：`.mihomo-test.sh`、`mihomo_ssplugin_test.go`、`node_check.go`、`check.go`、`project_test.go`、`registry.go`、`ssplugin/contract_test.go`、`frontend/tests/editable-combobox.spec.ts`；活动代码和测试中已无 `11929` / `1.19.29` 残留，历史 Design/Build/Issue 文档未改写。
- 新增 `backend/internal/assembly/mihomo_first_batch_test.go`：固定 v1.19.31 下首批四协议正例（VLESS/VMess/Trojan/SS 固定夹具）和四类内核反例（VLESS 缺 uuid、VMess 缺 uuid、Trojan 缺 password、SS 非法 cipher）；`.mihomo-test.sh` 运行范围扩展为 `^TestMihomo11931`。
- 失败优先证据：更新前用 v1.19.31 二进制执行旧脚本，脚本按预期在第 21 行以 `固定验收要求 Mihomo Meta v1.19.29，实际: ...v1.19.31...` 失败。
- 首轮固定内核执行发现 VMess 固定夹具缺少 `alterId`，v1.19.31 报 `proxy 0: '' has unset fields: alterId`。核对 tag 源码 `adapter/outbound/vmess.go`：`AlterID` 与 `Cipher` 均无 `omitempty`。按既有 schema 默认（`alterId=0`、`cipher=auto`）在 `normalizeClashFields()` 的 VMess 输出副本中补齐缺失值；不改字段集合、枚举、必填性、敏感路径、`protocol_json` 持久化或 URI 输出。`links_test.go` 的键序期望同步增加 `alterId`、`cipher`。
- 定向门禁：
  - `MIHOMO_11931_BIN='/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo' ./.mihomo-test.sh`：通过（首批四协议 4 正例/4 反例，SS 插件 4 正例/4 反例，旧拼接字符串仍由项目自检拒绝）。
  - `cd backend && go test ./internal/node ./internal/assembly -count=1`：通过。
  - `cd backend && go build ./...`：通过。
  - `cd backend && go test ./... -count=1`：通过；未设置外部二进制时固定内核测试保持显式 `SKIP`。
  - `cd frontend && npm test -- --run editable-combobox node-form-layout nodes-view`：3 文件 / 50 用例通过。
  - `git diff --check`：退出码 0。
- 未执行项：本 Step 未运行完整 `go test -race`、`go vet`、前端生产构建、Docker 构建、API/浏览器 smoke 或真实连接；保留到后续 Step/Step 22。


### Step 2：`CurrentState` v2、selector 与清空域

- 实现第四章 selector 合同、递归 selector 源路径和 `state_only` 投影排除。
- `Matches`、`RequiredFor`、活动投影、校验、敏感合并、扩展清理和前端 `matchesCondition` 共用 selector 语义。
- `normalizeResetScopes` 只接受当前协议声明的 `selector.<name>`；未知 selector 返回 400。
- v1 状态读取派生但不回写；保存改写为 v2。
- 覆盖 A→B→A、失败保存回滚、检查不落库、凭据 keep/clear、扩展作用域清理。
- **字段逻辑：** 新增 `CurrentState.selectors`、`ConditionRule.selectors`、`FieldSchema.selector_name/state_only` 和 `selector.<name>` reset scope；`network/security/plugin/features` 保持原语义。selector 值必须来自注册表白名单，`state_only` 值只进 `current_state_json`，普通 selector 源字段仍保存在 `protocol_json`。切换 selector 时，后端根据旧值和新值清除所有带相应 `reset_on` 的普通字段、递归敏感路径、未知扩展和未应用草稿；失败保存不得改变 `edit_revision`、密文或状态。v1 派生使用第四章矩阵，互斥字段同时存在时不猜测，返回字段级 400。
- **验收命令：**
  ```bash
  cd backend && go test ./internal/node -run 'Test.*(Selector|CurrentState|Reset|Credential|Extension)'
  cd ../frontend && npm test -- --run node-form-layout node-features protocol-field-editor nodes-view
  cd .. && git diff --check
  ```
**实施记录（2026-09-22）**

- 公共模型：`CurrentState` 增加 `selectors`；`ConditionRule` 增加 `selectors`；新增 `SelectorSchema{name,values,default,source_field}`；`FieldSchema` 增加 `selector_name/state_only`；`Protocol` 增加 `selectors`。
- 注册表：新增 `validateProtocolSelectors()`，在 `protocolIndex` 初始化时校验 selector 名称、允许值、默认值、来源字段、字段引用和 `state_only` 字段类型；非法注册直接阻断启动。
- 新增 `backend/internal/node/selector.go`：selector 值转换、wire 派生、v1 内存派生、请求归一化、`selector.<name>` reset scope 白名单、后端 selector 变化复核、state_only 禁止进入 `protocol_json`、selector 一致性校验。
- 状态生命周期：`currentStateFormatVersion` 升为 2；创建、更新、URI 导入写入 v2；读取 v1 时只在 `scanNode` 内存派生 selector，不写库；`resolveCurrentState()` 统一补全 selector 与普通 selector 的 `source_field`。
- 清空与安全：`normalizeResetScopes(proto, scopes)` 只接受当前协议声明的 `selector.<name>`；更新/检查时后端根据旧、新状态自动补齐 selector reset scope；`ProjectActive`、`validateActiveFields`、`validateProtocolFields`、`validateKnownTopLevel` 排除 `state_only`；敏感字段与 `selector.<name>` 扩展作用域按同一 reset 链清除。
- 前端：`api/node.ts` 增加 selector 类型；`matchesCondition()` 支持 `selectors` 且与其它维度保持 AND；`NodesView.vue` 的扩展清空识别 `selector.<name>`；新增前端 selector 匹配回归。
- 定向测试：
  - `cd backend && go test ./internal/node -count=1`：通过。
  - `cd backend && go test ./... -count=1`：通过。
  - `cd backend && go build ./...`：通过。
  - `cd frontend && npm test -- --run node-form-layout node-features protocol-field-editor nodes-view`：4 文件 / 86 用例通过。
  - `cd frontend && npm run build`：通过（仅既有 chunk 体积提示）。
  - `git diff --check`：退出码 0。
- 覆盖事实：合成协议服务级测试覆盖 v2 创建/读取/重开、未知 selector 与非法值 400、普通 selector 与 wire 不一致 400、A→B→A 不恢复旧参数、失败保存不改库、检查前后数据库快照一致、selector 切换清除敏感字段与所属扩展；v1 读取派生测试断言原状态和 `protocol_json` 不变。
- 边界：本 Step 只实现公共 selector 机制，未给任何真实协议批量预置 selector；HTTP/SSH/Snell 等协议的 selector 声明与 UI 在各自 Step 中串行加入。
- 未执行项：本 Step 未运行 race/vet/Docker/API/浏览器 smoke；保留到 Step 22。



### Step 3：Endpoint policy 与 adapter 骨架

- 实现第四章 endpoint policy；创建／更新／检查统一调用协议感知校验。
- 前端按 policy 显示、必填、清空 host/port；列表不显示 `:0`。
- `clashProxy()` 改为 adapter registry；先提供点名协议的临时 legacy adapter 保持尚未轮到的协议行为，并返回 `legacy_adapter_pending` evidence，不允许静默 default map 透传。每完成一个协议 Step 就删除该协议的 legacy 登记；Step 20 必须归零。
- 节点检查和正式装配必须调用同一函数；增加静态／单元门禁禁止第二套拼装。
- **字段逻辑：** `Protocol.EndpointPolicies` 只由 `protocol + CurrentState` 计算；`host/port` 不进入 `protocol_json`。`required` 模式要求 host 非空且 port 为 1～65535；`hidden` 模式在 create／update／check 中统一规范为 `''/0` 并禁止 wire 输出；替代字段（`ports`、`port-range`、`peers[]`）由协议 Step 自己校验。adapter 公共层统一写 `name/type`，再按 policy 写 `server/port`，最后处理活动协议字段和 `BasicOption` 白名单：`tfo`、`mptcp`、`interface-name`、`routing-mark`、`ip-version`、`dialer-proxy`。公共字段必须有类型／范围校验，`dialer-proxy` 继续走现有名称引用检查；协议不支持的公共字段不得因共享 schema 被盲目输出。
- **验收：** 普通协议空 host/port 仍 400；Tailscale policy 夹具可空；Mieru range 不输出 port；WireGuard peers 不输出顶层 endpoint。
- **验收命令：**
  ```bash
  cd backend && go test ./internal/node ./internal/assembly ./internal/server -run 'Test.*(Endpoint|ClashAdapter|NodeCheck)'
  cd ../frontend && npm test -- --run nodes-view node-form-layout
  cd .. && git diff --check
  ```


**实施记录（2026-09-22）**

- 公共 endpoint policy：新增 `EndpointPolicy{HostMode,PortMode,EmitHost,EmitPort,When}`；`Protocol` 增加 `endpoint_policies`；`validateProtocolEndpointPolicies()` 在注册表初始化时校验 mode、emit 关系和 selector 条件引用；无声明时使用 required/required 的隐式默认策略，保持既有普通协议行为。
- 新增 `backend/internal/node/endpoint.go`：`MatchEndpointPolicy` 要求协议＋CurrentState 恰好命中一条；`NormalizeEndpoint` 对 hidden 统一 `''/0`，required 校验 host/port，optional 允许 0，并返回实际策略；创建、更新、检查统一调用。
- Clash adapter 骨架：新增 `backend/internal/assembly/clash_protocols.go`，定义 `ClashNodeDraft`、`ClashProtocolAdapter`、注册表和 `buildClashProxy()`；正式装配与节点检查均通过该入口。未迁移协议走显式 legacy 投影并返回 `legacy_adapter_pending` info evidence；`legacyAdapterPendingCount()` 用于后续归零统计。
- 节点检查生命周期：新增 `CheckTargetDraft` / `CheckRendererDraft`；生产装配路径注入 `NodeID/Persisted/CurrentState`，`CheckNodeTargetDraft()` 与正式装配共用 `buildClashProxy()`；旧 `CheckNodeTarget()` 签名保留为兼容包装。`load.go` 读取节点 `id/state_format_version`，装配层按 v1/v2 在内存补全 selector。
- 前端：`api/node.ts` 增加 `EndpointPolicy` 与 `endpoint_policies` 类型；`nodeFormLayout.ts` 增加 `endpointPolicyFor()` 与默认 required 策略；`NodesView.vue` 按 policy 显示/必填/隐藏服务器与端口，hidden 时显示“协议自身管理”，列表对 port=0 不再显示 `:0`。
- 定向测试与门禁：
  - `cd backend && go test ./internal/node ./internal/assembly ./internal/server -count=1 -run 'Test.*(Endpoint|ClashAdapter|NodeCheck)'`：通过。
  - `cd backend && go test ./... -count=1`：通过；`go build ./...`：通过。
  - `cd frontend && npm test -- --run nodes-view node-form-layout`：2 文件 / 44 用例通过。
  - `cd frontend && npm run build`：通过（仅既有 chunk 体积提示）。
  - `git diff --check`：退出码 0。
- 覆盖事实：普通协议空 host/port 仍 400；hidden policy 请求残值规范化为 `''/0` 且服务端落库为空；selector 条件下的 policy 匹配；legacy_adapter_pending info 不改变目标 status；注册 adapter 时 check 与正式装配都能读取同一 draft 的 NodeID/Persisted/State；hidden endpoint 公共层不输出 server/port。
- 边界：本 Step 只加入公共 policy 机制和 adapter 骨架，19 个真实协议的 endpoint 替代策略（Hysteria2 ports、Mieru range、WireGuard peers、Tailscale hidden）将在各自协议 Step 声明并用实际协议夹具验收；当前 `legacyAdapterPendingCount()` 为 19，后续每步递减并在 Step 20 归零。
- 未执行项：race、vet、Docker、API/浏览器 smoke、真实连接仍未执行；保留到 Step 22 或后续协议 Step。

**中断恢复检查点（2026-09-22 首次核验，已被提交取代）**

- 首次核验时 Step 0.5～3 成果位于本地未提交工作区（24 个已跟踪文件修改、7 个未跟踪新文件），HEAD `f00b350c127773b9ee57e4c77c7d98d23121c859`。该状态已由后续提交 `7ecb6d5aa0619d560fcbc786547cf95e0fef7080` 取代，本条仅保留当时事实，不作为当前恢复依据。
- 边界核验当时确认尚未进入 Step 4：HTTP 仍使用 `legacy_adapter_pending`，未注册 HTTP 正式 adapter，也未加入 Step 4 要求的 HTTP selector、条件 schema、组合校验和协议固定夹具。
- 当时中断后独立复验通过：`cd backend && go test ./... -count=1 && go build ./... && go vet ./...`；前端 `node-form-layout/node-features/protocol-field-editor/nodes-view/editable-combobox` 共 95 个用例通过且 `npm run build` 通过（仅既有 chunk 体积提示）；固定 Mihomo v1.19.31 门禁通过首批四协议与 SS 插件正反例；Markdown 链接检查和 `git diff --check` 通过。

**二次恢复核验与 Step 3-fix（2026-09-22）**

- **实际恢复点更正：** Step 0.5～3 的成果已提交在 `7ecb6d5`（31 个文件，+1800/−224），分支 `beta`、HEAD 与 `origin/beta` 一致、ahead/behind 0/0；核验开始时工作区干净，`git diff --stat` 为空、`git diff --check` 退出码 0。首次检查点中“未提交工作区／HEAD `f00b350`”的描述已过时。
- **Step 3 静态门禁缺口（Step 3-fix）：** 二次全量复核发现 `frontend/tests/style-tokens.spec.ts` 失败：`src/views/admin/NodesView.vue:810` 的 `text-gray-500` 未登记，而该行由 `7ecb6d5` 写入（Step 3 的“协议自身管理”占位）；Step 3 当时只执行 `nodes-view node-form-layout`（44 用例）与 `npm run build`，未覆盖全量静态门禁，故未暴露。失败优先证据为全量前端门禁 `1 failed | 315 passed (316 tests)`。
- **Step 3-fix 实施：** 仅将该行的 `text-gray-500` 替换为既有设计 Token `text-text-tertiary`（`--ui-text-tertiary`，light `#64748B`／dark `#94A3B8`，与 Tailwind `gray-500` 语义最近的次要文本 Token），不修改组件结构、文案、样式尺寸或其他文件。
- **Step 3-fix 验收：** `cd frontend && npm test -- --run` → 47 文件 / 316 用例全部通过；`npm run build` 通过（仅既有 main chunk 体积提示）；`cd backend && go build ./...` 通过（确认未影响后端编译）；`git diff --check` 退出码 0。本步只改 1 行前端代码，未触及数据库、schema、wire 输出或协议合同。
- **固定 Mihomo v1.19.31 恢复（用户已确认）：** 记录路径 `/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo` 已变为 `Mihomo Meta v1.19.29`（内嵌 `vcs.revision=e26714a181ac0e2fa803453c0a8e9a9ce94e31cb`），原 v1.19.31 固定二进制已不在本机。经用户确认改为从官方 release 获取并在仓库外固定路径保存：主用 `~/mihomo-bins/mihomo-v1.19.31-go122`（`-v` = `Mihomo Meta v1.19.31 darwin arm64 with go1.22.12 Mon Sep 14 13:27:59 UTC 2026`，与本文档第二章记录逐字一致；`vcs.revision=ab405bad5beeeac8b003bb01f60f134f6df54471`；SHA-256 `a756bc56fee64201b3d5b706e946e926d265dd2d34450de8943f83d2f4c797fe`），交叉核对件 `~/mihomo-bins/mihomo-v1.19.31`（官方默认 go1.26.8 构建，同 revision，SHA-256 `fae1f37e28ee53fcf5be7a8bb121099db1fe442e44205734ed49c62579364090`）。`MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh` 退出码 0，首批四协议与 SS 插件正反例全部通过。后续所有 Step 的固定内核证据统一使用该路径，不再使用 Clash Verge 随附内核。
- **数据目标复核：** 当前仓库 `backend/data` 不存在，仓库内无 `*.db`／`*.sqlite*` 文件，授权目标内数据库、manual 节点、state v1/v2、空 endpoint、未知顶层键与 OpenVPN `client-config` 遗留行计数均为 0；未连接外部 `DATA_DIR`、Docker 卷、生产或备份。
- **边界核验：** 尚未进入 Step 4：`clashProtocolAdapters` 注册数为 0，`legacyAdapterPendingCount()` 为 19，HTTP 仍是旧 schema（无 `auth_mode`、无 TLS 条件字段、无 mTLS 成对字段），恢复入口在 Step 3.5 完成后为 Step 4。
- **本检查点未执行：** `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；这些证据仍按后续 Step 与 Step 22 收集，不由本检查点推定通过。


### Step 3.5：公共字段类型增补（multiline／secret-multiline／byte-sequence）

- **来源与授权：** 第四章 4.1 要求 `multiline`／`secret-multiline` 成为明确类型（停止依赖字段名猜测大文本），并新增 WireGuard reserved 专用的 `byte-sequence`；用户于 2026-09-22 确认在 Step 4 之前一次性完成该公共类型增补。本 Step 是 Build32 未编号的公共批次，完成后再进入 Step 4。
- **前置：** Step 3-fix 已完成；Step 2 的 selector 机制与 Step 3 的 endpoint policy／adapter 骨架已就绪；本步不新增协议字段分支，不改变任何 wire key。
- **影响文件：** `backend/internal/node/registry.go`（类型白名单校验、既有字段重定型）、`backend/internal/node/node.go`（类型校验与 byte-sequence 规范化）、`backend/internal/node/normalize.go`（保存前规范化入口）、`backend/internal/node/field_types_test.go`（新增）、`frontend/src/components/ProtocolFieldEditor.vue`（类型渲染，移除 `isLongText` 名称启发式）、`frontend/tests/protocol-field-editor.spec.ts`（新增用例）。
- **类型合同：**

  | 类型 | 语义 | 存储／wire | UI |
  |---|---|---|---|
  | `multiline` | 非敏感多行文本（证书、CA、Host Key、Restls Script、OpenVPN client-config） | string，不加密 | 多行文本域（4 行） |
  | `secret-multiline` | 敏感多行文本（私钥、PEM 内容） | string，按 `SensitiveFields` 既有路径加密 | 多行文本域＋既有凭据状态语义 |
  | `byte-sequence` | 恰 3 字节序列（WireGuard reserved） | 规范化为 `[0..255]` 三整数数组 | 三个 0–255 整数输入＋Base64 应用 |

- **注册与校验合同：**
  - 注册表初始化时递归校验 `FieldSchema.Type` 必须属于已知集合；未知类型直接阻断启动。已知集合为 `text/password/select/text-list/int-list/number/bool/object/multiline/secret-multiline/byte-sequence`。
  - `secret-multiline` 字段必须在其协议的 `SensitiveFields` 中声明对应路径（含嵌套点路径与 `[]` 列表路径），否则阻断注册；`password` 的既有敏感性声明方式不变，本步不为历史字段补声明。
  - `byte-sequence` 接受三种输入并统一规范化为 3 整数数组：`[a,b,c]`（含 JSON 数字序列）、`"1,2,3"`／`"1 2 3"`、Base64 字符串（解码后必须恰 3 字节）。长度不足／超出、越界值、非法 Base64 与非数字串均返回字段级错误；规范化在 `NormalizeProtocolJSON` 内完成，保存、检查与 URI 导入共用同一入口。
- **重定型（保持行为与敏感性不变）：** `certificate`、`ca`、`ca-str`、`host-key`、`restls-script`、`client-config` → `multiline`；既有 `private-key` 字段（WireGuard／AnyTLS／MASQUE／SSH／v2ray-plugin-opts／shadow-tls-opts）→ `secret-multiline`。这些字段此前已由字段名启发式渲染为多行文本，或已声明为敏感路径，因此本步不产生用户可见语义变化，只把隐式约定改为显式类型。
- **不属于本步：** 不新增任何协议字段、selector、endpoint policy 或 adapter；不把 `host-key`／`host-key-algorithms` 改为列表（Step 6）；不改变 SS 插件 `restls-script` 的敏感性（保持普通参数）；不引入 TypeScript 侧联合类型（`api/node.ts` 的 `type` 仍为 string）。
- **失败优先测试：** 后端 `TestValidateProtocolFieldTypes`（未知类型／secret-multiline 未声明敏感／合法嵌套）、`TestNormalizeByteSequence`（三种输入、长度与越界反例）、`TestByteSequenceNormalizedOnSave`（保存前规范化）、`TestMultilineFieldTypes`（字符串接受、非字符串拒绝）、`TestRegistryFieldTypesKnown`（全部已注册协议通过类型门禁）；前端 `multiline`／`secret-multiline` 渲染为多行、`byte-sequence` 三输入与校验、以及"普通 `text` 类型不再按字段名渲染多行"的反向断言。
- **验收命令：**

  ```bash
  cd backend && go test ./internal/node -run 'Test.*(FieldType|ByteSequence|Multiline)' -count=1
  cd backend && go test ./internal/node ./internal/assembly -count=1
  cd ../frontend && npm test -- --run protocol-field-editor node-form-layout node-features nodes-view
  cd ../frontend && npm run build
  cd .. && git diff --check
  ```

- **完成定义：** 三种类型在注册、校验、规范化、前端渲染四处一致；未知类型与未声明敏感的 `secret-multiline` 注册即失败；既有 19 协议全部仍可注册并通过原测试；前端全量测试与生产构建通过。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/field_types_test.go`，仅使用既有入口（`validateFieldValue`／`NormalizeProtocolJSON`）运行定向用例，得到 4 项真实失败：`字段 ca 使用未知类型 multiline`、`字段 reserved 使用未知类型 byte-sequence`、`byte-sequence 规范化结果类型应为 []int，实际 []interface {}{1,2,3}`、嵌套列表 `首条 reserved = "AQID"`；证明三类语义当时均未实现。
- **后端实现：**
  - 新增 `backend/internal/node/field_types.go`：`knownFieldTypes` 类型白名单、`validateProtocolFieldTypes`（递归校验类型集合，并强制 `secret-multiline` 必须在其协议 `SensitiveFields` 中声明，列表内使用 `peers[].pre-*` 形式）、`parseByteSequence`／`parseByteSequenceString`／`byteSequenceFromInts`（整数序列、`"1,2,3"`／`"1 2 3"`、Base64 三种输入统一为 3 个 0-255 整数，长度／越界／非法 Base64／非数字均报错）、`normalizeByteSequenceFields`（递归覆盖对象与列表条目）。
  - `backend/internal/node/node.go`：`validateFieldValue` 新增 `multiline`／`secret-multiline`（字符串语义）与 `byte-sequence`（解析失败返回带字段路径的错误）分支。
  - `backend/internal/node/normalize.go`：`NormalizeProtocolJSON` 在旧别名归一化后调用 `normalizeByteSequenceFields`，使创建、更新、检查与 URI 导入共用同一规范化入口。
  - `backend/internal/node/registry.go`：`protocolIndex` 初始化新增 `validateProtocolFieldTypes` 门禁；既有显式重定型 `certificate`／`ca`／`ca-str`／`host-key`／`restls-script`／`client-config` → `multiline`，`private-key`（WireGuard／AnyTLS／MASQUE／SSH／v2ray-plugin-opts／shadow-tls-opts）→ `secret-multiline`。敏感性声明不变：上述 `private-key` 路径原本已在 `SensitiveFields` 中，`certificate`／`ca`／`restls-script` 仍为普通参数。
- **前端实现：** `frontend/src/components/ProtocolFieldEditor.vue` 新增 `secret-multiline`（多行文本域＋既有凭据状态文案与 `credential-change` 语义）、`multiline`（多行文本域）、`byte-sequence`（三个 0-255 整数输入＋Base64 应用／取消，未完整时发出 `validity-change: false` 阻断保存，完整或 Base64 成功时回写规范整数数组）；`sensitive` 计算纳入 `secret-multiline`；删除 `isLongText()` 字段名启发式。
- **偏差与处理（1 项，属合同更新而非放宽）：** 两处既有测试把 SS 插件 `private-key` 锁定为旧 `password` 类型：`backend/internal/server/node_test.go` 的 SS 固定插件投影断言与 `backend/internal/node/project_test.go` 的 `TestSSPluginFieldsMatchMihomo11931Contract`。按第五章“所有证书私钥和多行 secret 使用 `secret-multiline`”更新为显式类型断言，并保留／加强 `SensitiveFields` 与 `certificate`／`restls-script` 类型断言，未删除任何安全或字段集合检查。
- **边界：** `byte-sequence` 在 Step 3.5 只建立类型机制，没有生产协议使用；WireGuard `reserved` 仍为 `int-list`，其重定型、`peers[].reserved` 与 Base64 语义属 Step 11。`host-key`／`host-key-algorithms` 的列表化属 Step 6；SS 插件 `restls-script` 保持普通参数，Snell 侧 `secret-multiline` 属 Step 7。`api/node.ts` 的 `type` 仍为 string，无需为新增类型改动联合类型。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'Test.*(FieldType|ByteSequence|Multiline|LargeText)' -count=1`：通过（含既有 `TestValidateProtocolFieldTypes`，node 包新增 6 个测试函数、共 8 个匹配用例）。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`。
  - `cd backend && go build ./...`、`go vet ./...`、`gofmt -l ./internal/`（无输出）、`go run ./cmd/errgate ./...`（`OK (0 ignored errors matched baseline; baseline entries 0; 0 unexpected)`）：通过。
  - `cd frontend && npm test -- --run`：47 文件 / 321 用例全部通过（Step 3-fix 后为 316，本次新增 5 个用例）。
  - `cd frontend && npm run build`：通过（仅既有 main chunk 体积提示）。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0（首批四协议与 SS 插件正反例全部通过，证明重定型未改变 wire 输出）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。



### Step 4～7：基础代理组（严格按 Step 号串行）

每个 Step 均按“协议矩阵测试 → schema／组合校验 → wire adapter → check／正式装配 → UI 定向测试”的顺序完成。

#### Step 4：HTTP

- basic 认证成对；TLS 关闭清空 SNI、证书校验、certificate/private-key；mTLS 成对。
- Headers 保持开放 string map；未知复杂值拒绝。
- 正反例覆盖无认证、basic、TLS、mTLS、缺半对凭据。
- **字段逻辑：** 顶层 `server/port` 必填；`auth_mode=none` 时不活动且清空 `username/password`，`basic` 时二者均必填，`password` 为敏感字段。`tls=false` 时清空并禁止输出 `sni`、`skip-cert-verify`、`name-cert-verify`、`fingerprint`、`certificate/private-key`；`tls=true` 时 SNI 可空并由 Mihomo 回退 server，证书与私钥必须成对，只有 `private-key` 为 secret。`headers` 是开放 string map：键去首尾空白后不能为空、大小写不敏感重复键拒绝、值只允许字符串；禁止由用户覆盖 adapter 固定的代理认证内部处理。wire 逐项映射 v1.19.31 `HttpOption`，不输出 `auth_mode`。
- **本步影响（计划）：** `backend/internal/node/registry.go`（HTTP schema：`auth_mode` state_only selector、TLS feature 子字段、headers 声明 `map_value_type=string`、敏感路径新增 `private-key`）、`backend/internal/node/selector.go`（state_only selector 的 v1／缺省派生：存在 username／password → `basic`）、`backend/internal/node/project.go`（selector 分支清空与 HTTP 组合校验：认证成对、mTLS 成对、headers 规则、拒绝覆盖代理认证头）、`backend/internal/node/node.go`（创建／更新／检查在 `resolveCurrentState` 后执行 selector 分支清空）、`backend/internal/assembly/clash_protocols.go`（HTTP 显式 adapter＋BasicOption 公共字段复制，legacy 计数 19→18）、`backend/internal/assembly/node_check.go`／`links/links.go`（HTTP URI 不可表达字段降级诊断）、`frontend/src/views/admin/NodesView.vue`（`state_only` selector 控件读写 `current_state.selectors`、`selector.auth_mode` 清空范围、编辑回填）。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/http_protocol_test.go` 并运行定向用例，实现前 8 个测试函数真实失败：`HTTP 必须声明 auth_mode selector`、`无凭据应派生 none，实际 ""`、`创建后 auth_mode 应为 basic，实际 ""`、basic 缺半对返回 `当前协议不支持 selector: auth_mode`、`auth_mode=none 创建失败: 当前协议不支持 selector`、`tls=false 必须清空 sni`、headers 五类非法输入全部未被拒绝、`字段 certificate 未在协议注册表中声明`。
- **后端 schema 与 selector：**
  - `registry.go` HTTP 条目：`auth-mode`（`state_only` select，`selector_name=auth_mode`，`none/basic`，默认 `none`）＋ `username`／`password`（`when`／`required_when` 为 `selectors.auth_mode=[basic]`，`reset_on` 为 `selector.auth_mode`）＋ `tls`（标量 feature `tls`，`reset_on` 为 `feature.tls`）＋ TLS 子字段 `sni`／`skip-cert-verify`／`name-cert-verify`／`fingerprint`／`certificate`（multiline）／`private-key`（secret-multiline），全部 `when.features=[tls]` 且 `reset_on` 含 `feature.tls`；`headers` 为开放 string Map；协议级 `selectors` 声明 `auth_mode`；`SensitiveFields` 增加 `private-key`。
  - `selector.go` 新增 `deriveStateOnlySelector`：未显式提交 selector 时按 username／password 是否非空派生 `basic`／`none`，v1 读取与新建共用同一规则且不写库；显式提交的 `none` 优先。
- **清空与组合校验：** `project.go` 新增 `clearSelectorScopedFields`（只删除声明了 selector 条件且不匹配的字段，递归对象），接入创建、更新（敏感合并之后）与检查的两条分支，使 `auth_mode=none` 清空并禁止输出 `username/password`；`validateProtocolCombination` 新增 `http` 分支：mTLS 证书／私钥成对、headers 键去空白非空、值只允许字符串、大小写不敏感重复拒绝、禁止覆盖 `Proxy-Authorization`。`tls=false` 的字段清空复用既有标量 feature 机制。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `httpClashAdapter`，逐项映射 v1.19.31 `HttpOption`（`username`／`password`／`tls`／`sni`／`skip-cert-verify`／`name-cert-verify`／`fingerprint`／`certificate`／`private-key`／`headers`）并复制 BasicOption 白名单（`tfo`／`mptcp`／`interface-name`／`routing-mark`／`ip-version`／`dialer-proxy`）；只输出活动且非零值字段，`tls=false` 不输出任何 TLS 键，永不输出 `auth-mode`／`auth_mode`。legacy 待迁移计数由 19 降为 18。
- **URI 降级诊断：** `assembly/node_check.go` 新增 `http` 分支：mTLS（certificate／private-key）→ `core_semantic_unexpressible`（error，目标 skip）；自定义 `headers` → `uri_partial_fields`（warn）；`name-cert-verify`／`fingerprint` → `unverified_compatibility`（warn）。
- **前端：** `NodesView.vue` 新增 `selectorState`，`currentState` 计算输出 `current_state.selectors`；`state_only` 字段经 `fieldModelValue`／`setFieldModelValue` 读写 selector 状态，切回时按差异加入 `selector.<name>` 清空范围并触发既有 `resetProtocolScope` 清空；`openEdit` 从 `current_state.selectors` 回填，协议切换与新建时清空。三处字段渲染绑定统一改走该入口。
- **测试与夹具：** 新增固定夹具 `http-basic-tls.json`（basic＋TLS＋headers 正例）与 `http-mtls.json`（mTLS → URI skip），在 `node_check_test.go` 登记为 `warnURI`／`skipURI` 并把 `http-password` 加入凭据泄漏断言；新增 `mihomo_http_test.go`（固定内核 HTTP 正例＋非法客户端证书与非字符串 headers 两个反例）；`clash_protocols_test.go` 新增 HTTP wire 形状断言与 `legacyAdapterPendingCount()==18` 绝对断言；`server/node_test.go` 新增 HTTP 编辑 schema 断言；`nodes-view.spec.ts` 新增 selector 写入／切换清空／保存载荷与编辑回填两个用例。
- **偏差（2 项，均为契约更新，非放宽）：** ①`clash_protocols_test.go` 的 legacy 可观测测试与“check／正式装配共用 adapter”测试原先以 `http` 作为未迁移协议样本，HTTP 迁移后改用 `socks5`，并新增“HTTP 不再返回 `legacy_adapter_pending`”正向断言；②旧 `nodes-view` 与后端断言未受影响，SS 插件类型断言已在 Step 3.5 同步。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestHTTP' -count=1`：通过（新增 9 个测试函数）。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`。
  - `cd backend && go build ./... && go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）：通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 HTTP 正例 1 项、内核反例 2 项全部通过（非法客户端证书报 `parse certificate failed`）。
  - `cd frontend && npm test -- --run`：47 文件 / 323 用例全部通过（Step 3.5 后为 321，本次新增 2 个用例）。
  - `cd frontend && npm run build`：通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。


#### Step 5：SOCKS5

- 与 HTTP 共用有语义的认证／TLS schema helper，不复制整段字段；保留 UDP 独立开关。
- URI 不能表达的 mTLS 字段返回 diagnostic，不丢字段后仍标 complete。
- **字段逻辑：** `server/port` 必填；`auth_mode=none/basic` 对 `username/password` 的显示、成对必填、清空与敏感处理同 HTTP。wire 字段为 `tls`、`udp`、`skip-cert-verify`、`name-cert-verify`、`fingerprint`、`certificate/private-key`；固定 tag 的 `Socks5Option` 没有独立 `sni`，不得因复用 TLS helper 而下发或输出 `sni`。`tls=false` 清空全部 TLS 子字段；证书／私钥成对。`udp` 独立保存和输出，不因 TLS／认证切换被清除。URI 仅在活动字段可无损表达时 complete，否则对具体字段给出 warn／unsupported。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/socks5_protocol_test.go`，实现前 4 个测试函数真实失败：`SOCKS5 必须声明 state_only auth_mode selector`（selector 缺失、派生为空）、basic 缺半对返回 `当前协议不支持 selector: auth_mode`、`auth_mode=none` 创建失败、`tls=false 必须清空` 与 `sni` schema 断言失败。
- **共享 helper 抽取（Build32 要求不复制整段字段）：** `registry.go` 把 Step 4 的 HTTP 专用 helper 重命名为协议无关的 `basicAuthModeField`、`basicAuthCredential`、`tlsFeatureField`、`tlsSubField`，HTTP 与 SOCKS5 共同复用；`selector.go` 的 `deriveStateOnlySelector` 扩展为 `http`／`socks5` 共用；`project.go` 的 `validateHTTPTLSKeyPair` 重命名为 `validateTLSKeyPair` 并在 `validateProtocolCombination` 新增 `socks5` 分支。
- **SOCKS5 schema：** `auth-mode`（state_only select，none／basic，默认 none）＋ `username`／`password`（`when`／`required_when` 为 `selectors.auth_mode=[basic]`，`reset_on` 为 `selector.auth_mode`）＋ `tls` 标量 feature ＋ TLS 子字段 `skip-cert-verify`／`name-cert-verify`／`fingerprint`／`certificate`（multiline）／`private-key`（secret-multiline），**不声明 `sni`**；`udp` 为独立 bool，无任何 selector／feature 清空归属；协议级 `selectors` 声明 `auth_mode`，`SensitiveFields` 为 `password`／`private-key`。
- **Clash adapter：** 新增 `socks5ClashAdapter` 并注册，逐项映射 v1.19.31 `Socks5Option`（`username`／`password`／`udp`／`tls`／`skip-cert-verify`／`name-cert-verify`／`fingerprint`／`certificate`／`private-key`）＋ BasicOption 白名单；不下发也不输出 `sni`，`tls=false` 不输出任何 TLS 键，`udp` 与 TLS／认证状态无关。legacy 待迁移计数 18→17。
- **URI 降级诊断：** `assembly/node_check.go` 新增 `socks5` 分支：mTLS → `core_semantic_unexpressible`（error，skip）；`name-cert-verify`／`fingerprint` → `unverified_compatibility`（warn）；`tls`／`udp`／认证可在 URI 无损表达，不产生诊断。
- **测试与夹具：** 新增夹具 `socks5-basic-tls.json`（basic＋TLS＋udp 正例，URI 无诊断）与 `socks5-mtls.json`（mTLS → URI skip），在 `node_check_test.go` 登记并把 `socks-password` 加入凭据泄漏断言；新增 `mihomo_socks5_test.go`（固定内核 SOCKS5 正例＋非法客户端证书反例）；`clash_protocols_test.go` 新增 SOCKS5 wire 形状断言（含 `sni` 不输出、`udp` 独立）并把 legacy 计数断言更新为 17；`nodes-view.spec.ts` 新增 SOCKS5 复用条件字段与 UDP 独立用例。
- **偏差（1 项，为可维护性改进）：** `clash_protocols_test.go` 中“未迁移协议”样本改为动态选择（`firstLegacyProtocol` 取第一个未注册 adapter 的协议），避免每个协议 Step 重复改写同一测试；HTTP／SOCKS5 已迁移的正向断言保留。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestSocks5' -count=1`：通过（新增 5 个测试函数）。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；SOCKS5 正例 1 项、内核反例 1 项通过。
  - `cd frontend && npm test -- --run`：47 文件 / 324 用例全部通过；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。


#### Step 6：SSH

- 私钥只接受 PEM 内容；密码和私钥分支切换清除相应凭据，私钥口令只随私钥活动。
- `host-key` 和 `host-key-algorithms` 改为列表；空 host-key 明确显示“接受任意 Host Key”的安全提示。
- 固定内核正例不得引用本机文件。
- **字段逻辑：** `server/port/username` 必填。`auth_mode=password` 只活动 `password`；`private_key` 只活动 `private-key/private-key-passphrase`，私钥必须含合法 PEM 标志且在保存前解析，绝不把普通文本或路径交给 Mihomo；分支切换清除另一组 secret。`host-key` 为公钥文本列表，逐项用 authorized-key 语法验证、去空白去重；`host-key-algorithms` 为非空算法名列表并保持用户顺序。空 `host-key` 允许保存但产生安全 warn，非空时必须至少一项有效。SSH 不支持 UDP；不得输出 `udp=true`。wire 仅输出 `username/password/private-key/private-key-passphrase/host-key/host-key-algorithms` 及适用公共字段，检查预览对三类认证 secret 脱敏。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/ssh_protocol_test.go`（10 个测试函数）并运行 `go test ./internal/node -run 'TestSSH'`，实现前真实失败：`SSH 必须声明 auth_mode selector`、`无 private-key 应派生 password，实际 ""`、password 模式缺半对返回 `当前协议不支持 selector: auth_mode`、私钥路径／普通文本未被拒、口令生命周期无判定、Host Key 列表类型错误（`字段 host-key 类型应为 multiline`、`字段 host-key-algorithms 类型应为 text`）、敏感路径未登记。
- **后端 schema：** `registry.go` 新增 `sshAuthModeField`（`auth-mode` state_only select，`selector_name=auth_mode`，`password/private_key`，默认 `password`）、`sshBranchCredential`（`when`／`required_when` 为 `selectors.auth_mode=[<branch>]`、`reset_on=selector.auth_mode`）、`sshHostKeyField`、`sshHostKeyAlgorithmsField`；SSH 条目改为 `username` 无条件必填 ＋ `password`（password 分支必填）＋ `private-key`／`private-key-passphrase`（private_key 分支，前者必填）＋ `host-key`／`host-key-algorithms`（`text-list`，含留空安全提示）；协议级 `selectors` 声明 `auth_mode`；`SensitiveFields` 保持 `password/private-key/private-key-passphrase`。
- **selector 派生：** `selector.go` 的 `deriveStateOnlySelector` 增加 `ssh`：存在非空 `private-key` 派生 `private_key`，否则 `password`（v1 读取与新建共用，不写库）。
- **列表归一化：** `normalize.go` 新增 `normalizeProtocolListFields`／`normalizeStringListField`／`stringListItems`，对 SSH `host-key`／`host-key-algorithms` 去空白、去重并保持用户顺序；纯空白列表规范化为未设置；非文本类型继续交由 `validateFieldValue` 报字段级错误。
- **PEM 与 Host Key 校验：** `project.go` 新增 `validateSSHPrivateKey`（必须含 PEM `PRIVATE KEY` 标志；使用与固定内核一致的 `golang.org/x/crypto/ssh` 解析；加密私钥缺口令定位 `private-key-passphrase`；未加密私钥带口令也定位 `private-key-passphrase`）与 `validateSSHHostKeys`（逐项 `ssh.ParseAuthorizedKey` 校验 authorized-key 语法，拒绝空项），并在 `validateProtocolCombination` 的 `ssh` 分支接入创建／更新／检查三条路径。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `sshClashAdapter`，逐项映射 v1.19.31 `SshOption`（`username/password/private-key/private-key-passphrase/host-key/host-key-algorithms`）＋ BasicOption 白名单；新增 `copyClashListFields`／`clashStringList` 保证 `host-key*` 以 YAML 数组输出；永不输出 `udp`、`auth-mode/auth_mode`；空 `host-key` 返回 `ssh_host_key_unverified`（warn，`field_path=host-key`）。legacy 待迁移计数由 17 降为 16。
- **URI 稳定 skip：** `internal/assembly/links/links.go` 新增 `SupportsURI`（当前 11 个可表达协议）；`node_check.go` 对无 URI 映射协议返回稳定 `target_unsupported`＋`skip`＋无 preview，不再以 `协议无标准链接映射` 的形式落入 `core_semantic_unexpressible`。SSH 的 `sr-subs`／`generic-subs` 均稳定 skip，正式 SR/generic 装配继续按阻塞诊断跳过该节点。
- **测试与夹具：** 新增 `ssh-password.json`（password＋Host Key＋算法列表；clash 正例、URI skip）与 `ssh-private-key.json`（private-key 无 Host Key；clash warn＋URI skip）并在 `node_check_test.go` 登记 `warnClash` 与凭据泄漏断言；新增 `mihomo_ssh_test.go`（固定内核 password／私钥两正例＋私钥路径文本与非法 Host Key 两反例）；`clash_protocols_test.go` 新增 SSH wire 形状断言并把 legacy 计数断言更新为 16；`server/node_test.go` 新增 SSH 编辑 schema 断言；`nodes-view.spec.ts` 新增 SSH 分支切换／列表提交／编辑回填用例。
- **偏差（2 项，均为契约更新，非放宽）：** ①`field_types_test.go` 的 `TestLargeTextFieldsUseExplicitTypes` 把 `host-key` 从 `multiline` 移入新的 `text-list` 断言，并保留 private-key 的 `secret-multiline` 断言；②`all_protocols_test.go` 的 `minimalProtocolParams` 增加“按注册表默认 selector 组合补齐默认分支条件必填字段”，使 SSH 默认 `password` 分支的最小合法输入成立，HTTP/SOCKS5 默认 `none` 行为不变。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestSSH' -count=1`：通过（10 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestSSH|TestLegacyAdapterPendingCount|TestNodeCheckFixtures' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 SSH 正例 2 项、内核反例 2 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / 325 用例全部通过（Step 5 后为 324，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

#### Step 7：Snell

- 版本 1～5；v5 按 Mihomo tag 的 v4 客户端兼容实现记录诊断，不能伪称独立 v5 wire。
- v1/2 禁 UDP；v2 固定启用 reuse，v4/v5 才显示可编辑 reuse；五类 obfs 的字段、凭据、TLS 选项按模式清空。
- `client-fingerprint` 只在相关伪装分支显示。
- **字段逻辑：** `server/port/psk` 必填，`psk` 为 secret；`version` 允许 1～5，空值按 tag 默认 v1，v5 wire 保留 `version: 5` 但诊断明确内核以 v4 客户端实现。`udp` 在 v1/v2 必须 false，v3/v4/v5 可编辑；`reuse` 在 v1/v3 隐藏并清空、v2 固定 true 且不让用户关闭、v4/v5 可编辑。`obfs_mode` 是 state-only，映射到 `obfs-opts.mode`：`none` 删除整个对象；`http/tls` 活动 `host`；`shadow-tls` 活动 `host/password/version/fingerprint/certificate/private-key/skip-cert-verify/name-cert-verify/alpn`；`restls` 活动 `host/password/version-hint/restls-script/fingerprint/skip-cert-verify/name-cert-verify/force-tls12`；`jls` 活动 `host/username/password/alpn`。各模式的 password、private-key、restls-script 按敏感合同处理，模式切换清空旧对象全部字段。`client-fingerprint` 只在 shadow-tls／restls／jls 活动；证书／私钥成对，结构化 `obfs-opts` 禁止未知键。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/snell_protocol_test.go`（12 个测试函数），实现前真实失败：`Snell 必须声明 version／obfs_mode selector`、空值版本未派生 v1、`字段 version 类型应为 number`、`字段 reuse／obfs-opts／client-fingerprint 未在协议注册表中声明`、五类 obfs 分支条件必填全部未生效、`state_only obfs-mode` 未被专门拒绝。
- **后端 schema：** `registry.go` 新增 `snellVersionField`（普通 selector 来源字段，`selector_name=version`，选项 1～5，默认 1）、`snellUDPField`（`when.selectors.version=[3,4,5]`）、`snellReuseField`（`[4,5]`）、`snellObfsModeField`（state_only select，`none/http/tls/shadow_tls/restls/jls`）、`snellObfsField`／`snellObfsOptsField`（固定对象，13 个模式化子字段，`allow_unknown=false`）、`snellClientFingerprintField`；协议级声明 `version`（`SourceField=version`）与 `obfs_mode` 两个 selector；`SensitiveFields` 为 `psk`、`obfs-opts.password`、`obfs-opts.private-key`、`obfs-opts.restls-script`。
- **selector 派生：** `selector.go` 的 `deriveStateOnlySelector` 增加 `snell` 的 `obfs_mode`：按 `obfs-opts` 独有字段判定 restls／jls／shadow_tls／http，无对象或空对象为 none（`mode` 不落库，只能由字段反推）。
- **组合校验：** `project.go` 新增 `validateSnellCombination`：v1/v2 开启 UDP 返回字段级 400；shadow-tls 的 `certificate`／`private-key` 成对；接入 `validateProtocolCombination` 的 `snell` 分支。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `snellClashAdapter`，逐项映射 v1.19.31 `SnellOption`；`version` 归一化为整数（缺省 v1）；v2 固定输出 `reuse: true`，v4/v5 按用户值输出；v1/v2 不输出 `udp`；`snellObfsWireFields` 由 selector 注入 `obfs-opts.mode`（`shadow_tls`→`shadow-tls`）并输出已投影的活动子字段，`none` 不产生对象；v5 返回 `snell_v5_v4_compat`（info）诊断。legacy 待迁移计数由 16 降为 15。
- **测试与夹具：** 新增夹具 `snell-http.json`（v4＋http 混淆，clash 正例）与 `snell-shadow-tls.json`（v4＋shadow-tls＋client-fingerprint，clash 正例），两者 URI 均稳定 `target_unsupported`／skip，并把 `snell-password`／`snell-obfs-password` 加入凭据泄漏断言；新增 `mihomo_snell_test.go`（固定内核 3 正例＋非法 obfs mode 与 v1 UDP 2 反例）；`clash_protocols_test.go` 新增 Snell wire 形状断言并把 legacy 计数断言更新为 15；`server/node_test.go` 新增 Snell schema 断言；`nodes-view.spec.ts` 新增版本／混淆模式切换与清空用例。
- **基线缺陷修复（Step 7 暴露，含根因与影响）：**
  1. **后端 selector 读取会覆盖显式选择：** 根因是 `hydrateCurrentStateForRead` 在 `version < currentStateFormatVersion` 时无条件用 wire 字段重新派生 selector，而检查／诊断路径（`CheckTargetDraft` 不带状态版本）正是这种情况。影响：Snell 的 `http`/`tls` 无法由 `obfs-opts` 字段唯一反推时，检查预览会猜成 `http`，与正式装配（带 v2 状态版本）的 `tls` 不一致，违反“检查与正式装配同源”。修复：只填充缺失 selector，显式 selector 始终权威；v1 状态无 selector 时仍照旧派生，行为不变。
  2. **前端普通 selector 未投影来源字段：** 根因是 `selectorValueFor` 只读 state_only 的本地状态并回退注册表默认值，忽略 `selectors[].source_field`。影响：Snell `version` 改动后 `current_state.selectors.version` 仍为默认 1，提交时与 `protocol_json.version` 不一致会被后端 400 拒绝，且 `when.selectors.version` 条件分支在 UI 不生效。修复：`selectorValueFor` 优先读 `protocol_json` 的来源字段；`setField` 对 selector 来源字段变化补 `selector.<name>` 清空域，与后端复核一致。
- **偏差（2 项，均为契约更新，非放宽）：** ①`TestLargeTextFieldsUseExplicitTypes` 的 `restls-script` 改为允许 `multiline`／`secret-multiline`，因为 Snell 侧按敏感合同使用 `secret-multiline`，SS 插件侧仍为普通 `multiline`；②`udp` 采用 `when.selectors.version=[3,4,5]` 的分支清空语义实现“v1/v2 必须 false”，与 selector 清空架构一致，测试断言落库值而非 400。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestSnell' -count=1`：通过（12 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestSnell|TestMihomo11931Snell|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 Snell 正例 3 项、内核反例 2 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / 326 用例全部通过（Step 6 后为 325，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

- **每步验收命令模板：**
  ```bash
  cd backend
  go test ./internal/node ./internal/assembly -run 'Test.*<ProtocolName>'
  go test ./internal/server -run 'Test.*Node'
  cd ../frontend
  npm test -- --run nodes-view protocol-field-editor node-form-layout
  cd ..
  git diff --check
  ```

### Step 8～10：QUIC 认证组

#### Step 8：Hysteria

- `auth` 为 Base64、`auth-str` 为普通认证字符串，按 selector 二选一；禁止同时输出。
- `up/down` 作为唯一编辑入口；旧 `up-speed/down-speed` 只允许在兼容输入归一化时转换，不能作为第二套可保存字段。
- `protocol` 只开放 tag 支持值；`obfs-protocol` 仅作为兼容输入别名归一化，不作为第二个编辑入口。
- TLS/ECH/mTLS、端口跳跃和窗口高级项补齐；port 仍必填。
- **字段逻辑：** `server/port` 始终必填，`ports` 是端口跳跃补充字段而非 port 替代；端口／范围语法分别校验。`auth_mode=none` 清空 `auth/auth-str`，`base64` 只活动并校验 `auth`，`string` 只活动 `auth-str`；二者及 `obfs` 都按 secret 处理。固定 tag 的构造函数会先解析 `up/down`，即使存在兼容字段 `up-speed/down-speed` 也不能只填数字字段，因此表单只保存非零带单位的 `up/down`，兼容输入在 Normalize 阶段转成规范字符串后删除旧键。`protocol` 空值按 `udp`，`obfs-protocol` 只读入后归一化到 `protocol`。TLS 字段为 `sni/ech-opts/skip-cert-verify/name-cert-verify/fingerprint/certificate/private-key/alpn`，证书／私钥成对；高级字段为 `recv-window-conn/recv-window/disable-mtu-discovery/fast-open/hop-interval`，整数非负，窗口关系由项目先校验。wire 不输出 `auth_mode`、`up-speed/down-speed` 或 `obfs-protocol`。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/hysteria_protocol_test.go`（9 个测试函数），实现前真实失败：`Hysteria 必须声明 state_only auth_mode selector`、无认证参数未派生 none、base64 分支返回 `当前协议不支持 selector: auth_mode`、`up/down` 未作为必填带宽入口、`protocol` 非法值未校验、`ports` 语法未校验、`certificate` 未在注册表中声明、敏感路径未登记。
- **后端 schema：** `registry.go` 新增 `hysteriaAuthModeField`（state_only select none／base64／string，默认 none）、`hysteriaAuthCredential`（分支条件＋条件必填＋`selector.auth_mode` 清空）、`hysteriaProtocolField`（`udp/wechat-video/faketcp`，默认 udp）、`echOptsField`（`enable` 控制的 ECH 对象，关闭清空 `config/query-server-name`）；Hysteria 条目重写为 `req(up)`／`req(down)`、端口跳跃、TLS 与 mTLS、窗口／Fast Open／Hop 高级项；移除固定 tag 中不存在的 `ca`／`ca-str`，移除 `obfs-protocol`／`up-speed`／`down-speed` 可保存入口，`obfs` 改为敏感 `password`；`SensitiveFields` 为 `auth`、`auth-str`、`obfs`、`private-key`。
- **selector 派生：** `selector.go` 的增加 `hysteria` 的 `auth_mode`：有 `auth` → base64，有 `auth-str` → string，均无 → none。
- **兼容归一化：** `normalize.go` 新增 `canonicalizeHysteriaAliases`：`obfs-protocol` → `protocol`（规范值优先）、`up-speed`／`down-speed`（Mbps 数字）→ 非零 `up`／`down` 规范字符串，旧键一律删除；`numberParam` 供数值读取复用。
- **组合校验：** `project.go` 新增 `bandwidthPattern`／`validBandwidth`（与固定 tag `StringToBps` 一致，纯整数按 Mbps）、`validHysteriaPorts`（1-65535 单端口或 begin-end 范围）、`validateIntegerMinimum` 与 `validateReceiveWindows`（窗口为非负整数且连接窗口不小于流窗口），`validateHysteriaCombination` 覆盖带宽、Base64 auth、端口跳跃、mTLS 成对、窗口关系，并在核验修复中补齐 `hop-interval` 非负整数约束。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `hysteriaClashAdapter`，逐项映射 v1.19.31 `HysteriaOption`（含 `ech-opts` 与 `alpn` 数组）；只输出当前认证分支，绝不输出 `auth-mode`／`obfs-protocol`／`up-speed`／`down-speed`；`ports` 作为 `port` 的补充字段保留。legacy 待迁移计数由 15 降为 14。
- **URI 收口：** `assembly/node_check.go` 的 `linkTargetDiagnostics` 新增 `hysteria` 分支：`auth-str` 与 mTLS → `core_semantic_unexpressible`（skip）；`name-cert-verify`／`fingerprint`／已启用 `ech-opts` → `unverified_compatibility`／`uri_partial_fields`（warn）；窗口／MTU／Fast Open／Hop 等高级项 → `uri_partial_fields`（warn）。`links/links.go` 新增 `mbpsString`，把带单位 `up/down` 归一化为 URI 约定的 Mbps 数字（`100 Mbps`→`100`、`1 Gbps`→`1000`），无法解析的写法原样保留不静默改写。
- **测试与夹具：** 新增夹具 `hysteria-basic.json`（base64 auth＋端口跳跃＋TLS，clash 正例且 URI 正例）与 `hysteria-auth-str.json`（string auth → URI skip），并把 `hysteria-auth-str` 加入凭据泄漏断言；新增 `mihomo_hysteria_test.go`（固定内核 base64／string 两正例＋非法 base64 与非法带宽两反例）；`clash_protocols_test.go` 新增 Hysteria wire 形状断言并把 legacy 计数断言更新为 14；`links_test.go` 新增带宽归一化用例；`server/node_test.go` 新增 Hysteria schema 断言；`nodes-view.spec.ts` 新增认证分支互斥用例。
- **偏差（2 项，均为契约更新，非放宽）：** ①`all_protocols_test.go` 的 `minimalProtocolParams` 为 `up`／`down` 提供内核可解析的 `100 Mbps`；②Hysteria 移除固定 tag 中不存在的 `ca`／`ca-str`，相应断言以 `HysteriaOption` 字段集合为准。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestHysteria' -count=1`：通过（10 个测试函数；核验修复新增 Hop／窗口负数与小数反例及零值正例）。
  - `cd backend && go test ./internal/assembly -run 'TestHysteria|TestMihomo11931Hysteria|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过；`go test ./internal/assembly/links -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 Hysteria 正例 2 项、内核反例 2 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / 327 用例全部通过（Step 7 后为 326，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

#### Step 9：Hysteria2

- `ports` 模式真正替代 port；`hop-interval` 支持单值或单范围字符串。
- obfs none/salamander/gecko；启用即要求密码，包大小仅 gecko。
- Realm 作为 feature object，包含 token、realm-id、STUN 列表和自身 TLS 子树；关闭清空全部子凭据。
- 补 BBR profile、handshake timeout 和 quic-go 高级窗口，默认不写库。
- **字段逻辑：** `server/password` 必填；`endpoint_mode=single` 活动顶层 port 并删除 `ports/hop-interval`，`ports` 活动端口列表／范围字符串并把顶层 port 规范为 0，`hop-interval` 只在 ports 模式活动，接受单值或 `start-end` 且最终最小值按 tag 不低于 5 秒。`up/down` 为可选带单位速率字符串。`obfs_mode=none` 清空 `obfs/obfs-password/obfs-min-packet-size/obfs-max-packet-size`；salamander/gecko 均要求 `obfs-password`，包大小上下界只在 gecko 活动且 min≤max。公共 TLS 字段为 `sni/ech-opts/skip-cert-verify/name-cert-verify/fingerprint/certificate/private-key/alpn`；`udp-mtu/handshake-timeout/cwnd` 为正整数或未设置，`bbr-profile` 仅在相关拥塞配置活动。四个 QUIC window 字段使用非负整数并验证 initial≤max。`realm-opts.enable=false` 清空整个对象；开启时活动 `server-url/token/realm-id/stun-servers` 与独立 TLS 子树，token/private-key 为 secret，URL／STUN／证书成对关系逐项校验。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/hysteria2_protocol_test.go`（7 个测试函数），实现前真实失败：`Hysteria2 必须声明 state_only endpoint_mode selector`、`hop-interval 类型应为 number`、`当前协议不支持 selector: endpoint_mode`、`obfs-min-packet-size`／`certificate`／`realm-opts` 未在协议注册表中声明。
- **后端 schema：** `registry.go` 新增 `hysteria2EndpointModeField`（state_only single／ports，默认 single）、`hysteria2ObfsModeField`（state_only none／salamander／gecko，默认 none）、`hy2ModeField`／`hy2PortsField`／`hy2HopIntervalField`／`hy2ObfsPasswordField`／`hy2ObfsPacketSizeField`（端口与混淆分支条件＋条件必填＋selector 清空）、`hysteria2RealmOptsField`＋`realmSubField`（`enable` 控制的 Realm 对象，关闭清空全部子字段与子凭据）；Hysteria2 条目补全 `up/down`、TLS／mTLS、`cwnd/bbr-profile/udp-mtu/handshake-timeout` 与四个 QUIC window 字段；声明 single／ports 两条 `EndpointPolicies`；移除固定 tag 中不存在的 `ca`／`ca-str`／`protocol`／`obfs-protocol` 与冗余 `obfs` 可保存入口（`obfs` 改由 selector 注入 wire）；`SensitiveFields` 为 `password`、`obfs-password`、`private-key`、`realm-opts.token`、`realm-opts.private-key`。
- **selector 派生：** `selector.go` 增加 `hysteria2` 的 `endpoint_mode`：存在非空 `ports` 派生 `ports`，否则 `single`（避免 API 客户端只提交 ports 时被 single 清空域丢弃）。
- **组合校验：** `project.go` 新增 `validHopInterval`（单值或单范围、最小 5 秒）与 `validateHysteria2Combination`（mTLS 成对、可选 `up/down` 合法非零速率、`cwnd/udp-mtu/handshake-timeout` 为正整数、四个 QUIC window 为非负整数、gecko 包大小正数且 min≤max、流／连接窗口 initial≤max、端口组语法）及 `validateHysteria2Realm`（启用时 server-url 必填且为绝对 HTTP(S)、STUN 逐项 host:port、Realm 证书成对）。速率与整数／小数边界由核验修复补齐。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `hysteria2ClashAdapter`，逐项映射 v1.19.31 `Hysteria2Option`；`obfs` 由 selector 注入；禁用 `ech-opts`／`realm-opts` 通过新增 `copyClashEnabledObject` 不写入 wire；ports 模式由 endpoint policy 隐藏顶层 port。legacy 待迁移计数由 14 降为 13。
- **产物自检收口（Step 9 暴露的基线缺陷）：** `CheckClashContent` 原先无条件要求每个 proxy 具备 `server`／`port`，会把合法的 ports 模式 Hysteria2 判为 `clash_output_invalid`。根因是自检硬编码了 endpoint 必填，未消费协议声明的 endpoint policy。修复：新增 `endpointPolicyRequirements`，仅当协议不存在隐藏该字段的合法状态时才把 `server`／`port` 作为 YAML 自检必填；未知节点类型仍保持原有 server／port 报错。该修复对后续 Mieru range、WireGuard peers、Tailscale 同样必要。
- **URI 收口：** `assembly/node_check.go` 的 `linkTargetDiagnostics` 新增 `hysteria2` 分支：ports 端口组、启用的 Realm 与 mTLS → `core_semantic_unexpressible`（skip，避免生成 `host:0` 的错误链接）；`name-cert-verify`／`fingerprint` → `unverified_compatibility`；ECH 与 `up/down`／包大小／拥塞／MTU／握手超时／四个 QUIC window → `uri_partial_fields`（warn）。
- **测试与夹具：** 新增夹具 `hysteria2-single.json`（single＋salamander，clash 与 URI 双正例）、`hysteria2-ports.json`（ports 模式无顶层 port → URI skip）、`hysteria2-realm.json`（Realm 开启 → URI skip），并把三个凭据值加入泄漏断言；新增 `mihomo_hysteria2_test.go`（固定内核 single＋gecko 与 ports 无 port 两正例＋未知 obfs 与缺混淆密码两反例）；`clash_protocols_test.go` 新增 Hysteria2 wire 形状断言并把 legacy 计数断言更新为 13；`server/node_test.go` 新增 Hysteria2 schema 与 endpoint policy 断言；`nodes-view.spec.ts` 新增端口模式隐藏／切换清空与混淆分支用例。
- **偏差（2 项，均为契约更新，非放宽）：** ①Hysteria2 移除固定 tag 中不存在的 `ca`／`ca-str`／`protocol`／`obfs-protocol`，`obfs` 改为由 `obfs_mode` selector 注入 wire（不落库），因此不再作为可保存字段；②`bbr-profile` 在 `ConditionRule` 现有维度中无法表达“存在拥塞配置时才活动”，保持常驻可编辑并在 Step 记录中说明（内核仅在拥塞控制器生效时消费该值）。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestHysteria2' -count=1`：通过（8 个测试函数；核验修复新增非法速率、零值／负数／小数及合法边界矩阵）。
  - `cd backend && go test ./internal/assembly -run 'TestHysteria2|TestMihomo11931Hysteria2|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 Hysteria2 正例 2 项、内核反例 2 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / 328 用例全部通过（Step 8 后为 327，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

#### Step 10：TUIC

- v4 token 与 v5 UUID/password 强互斥；A→B→A 不恢复凭据。
- UDP relay、拥塞、SNI/ECH/mTLS、UOT version 等枚举／范围按 tag 校验。
- 固定内核正例必须分别覆盖 v4、v5，反例覆盖非法 UOT version 等内核真实拒绝分支；混合凭据由项目 selector／投影层清除，并由 adapter wire 测试证明输出永不混合。固定 v1.19.31 遇到混合凭据会优先 token/v4，并不会拒绝，因此不得把“混合凭据拒绝”记作固定内核证据。
- **字段逻辑：** `server/port` 必填。`auth_mode=v4` 只活动必填 `token` 并清空 UUID/password；`v5` 只活动合法 UUID＋非空 password 并清空 token，三者均为敏感路径。`ip` 是可选直连地址覆盖，仅改变实际拨号地址而不替代 server/SNI；校验为 IP。TLS／QUIC 字段包括 `alpn/reduce-rtt/request-timeout/heartbeat-interval/udp-relay-mode/congestion-controller/disable-sni/max-udp-relay-packet-size/fast-open/max-open-streams/cwnd/bbr-profile/skip-cert-verify/name-cert-verify/fingerprint/certificate/private-key/recv-window-conn/recv-window/disable-mtu-discovery/max-datagram-frame-size/sni/ech-opts`。`disable-sni=true` 必须显示会同时跳过证书主机名验证的风险，不保留冲突 SNI；证书／私钥成对。`udp-over-stream=false` 清空／不输出 version；开启时 version 只允许 tag 支持的 legacy/current 数值，0 只作为输入缺省归一化，不在 UI 作为第三个版本。所有毫秒／窗口／包大小字段非负，datagram 上限和 relay packet 联动在项目层先给出字段错误，不能依赖内核静默截断。

**实施记录（2026-09-22）**

- **失败优先证据：** 先新增 `backend/internal/node/tuic_protocol_test.go`（10 个测试函数），实现前真实失败：`TUIC 必须声明 state_only auth_mode selector`、token 未派生 v4、v4/v5 分支返回 `当前协议不支持 selector: auth_mode`、UUID／IP／UOT／数据报联动／负数校验全部未生效、`certificate` 未在协议注册表中声明。
- **后端 schema：** `registry.go` 新增 `tuicAuthModeField`（state_only v4／v5，默认 v5）、`tuicAuthCredential`（分支条件＋条件必填＋`selector.auth_mode` 清空）、`tuicUDPRelayModeField`（`quic/native` 枚举）、`tuicUDPOverStreamVersionField`（两值枚举，`setScalarFeatures` 自动挂 `feature.udp-over-stream` 条件与清空）；TUIC 条目补全 `name-cert-verify`／`certificate`／`private-key`／`ech-opts`／`bbr-profile` 与 `sni`、`ip`、QUIC 窗口、数据报字段；核验修复新增 `tuicCongestionControllerField`，只允许固定 tag 实际处理的 `cubic/new_reno/bbr_meta_v1/bbr_meta_v2/bbr`，空值继续表示沿用内核默认行为；移除固定 tag 中不存在的 `ca`／`ca-str`；`SensitiveFields` 增加 `private-key`。
- **selector 派生：** `selector.go` 增加 `tuic` 的 `auth_mode`：存在非空 `token` 派生 v4，否则 v5（tag 分支默认）。
- **输入归一化：** `normalize.go` 新增 `canonicalizeTUICGuards`：`disable-sni=true` 时删除冲突 `sni`；`udp-over-stream-version` 的数值 0 归一化为 tag 缺省 legacy `"1"`，不把 0 作为第三个版本落库。
- **组合校验：** `project.go` 新增 `tuicDatagramFrameLimit`（1400）与 `validateTUICCombination`：v5 模式用 `github.com/google/uuid` 校验 UUID；`ip` 必须为合法 IP；心跳／超时／中继包／并发流／cwnd／窗口／数据报帧非负；mTLS 成对；`max-datagram-frame-size > 1400` 与 `max-udp-relay-packet-size > max-datagram-frame-size` 均返回字段级错误，不依赖内核静默截断。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `tuicClashAdapter`，逐项映射 v1.19.31 `TuicOption`；`udp-over-stream-version` 经 `clashIntValue` 以整数输出且仅在 UOT 开启时存在；`disable-sni=true` 时不下发 `sni` 并返回 `tuic_disable_sni_risk`（warn，含中间人风险说明）；禁用 ECH 不写入 wire。legacy 待迁移计数由 13 降为 12。
- **URI 收口：** `assembly/node_check.go` 的 `linkTargetDiagnostics` 新增 `tuic` 分支：v4 token 与 mTLS → `core_semantic_unexpressible`（skip）；`name-cert-verify`／`fingerprint` → `unverified_compatibility`；`ip`／超时／心跳／中继模式／拥塞／disable-sni／包大小／reduce-rtt／Fast Open／并发流／cwnd／bbr-profile／窗口／MTU／数据报帧／UOT 与版本 → `uri_partial_fields`（warn）。
- **测试与夹具：** 新增夹具 `tuic-v5.json`（UUID＋密码＋TLS，clash 与 URI 双正例）与 `tuic-v4.json`（token → URI skip），并把两个凭据加入泄漏断言；新增 `mihomo_tuic_test.go`（固定内核 v5 与 v4＋UOT 两正例，非法 UOT 版本与非法客户端证书两反例）；`clash_protocols_test.go` 的 TUIC wire 形状测试明确断言 v4 不输出 UUID/password、v5 不输出 token，并覆盖 `disable-sni` 风险；核验修复新增拥塞控制器 schema 白名单与未知值反例；legacy 计数断言更新为 12；`server/node_test.go` 新增 TUIC schema 与 UOT 版本条件断言；`nodes-view.spec.ts` 新增 v4/v5 互斥与 UOT 开关清空用例。
- **偏差（2 项，均为契约更新，非放宽）：** ①TUIC 移除固定 tag 中不存在的 `ca`／`ca-str`；②`minimalProtocolParams` 为 `uuid` 提供合法 UUID，使 TUIC v5／VLESS／VMess 的默认分支最小合法输入成立。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestTUIC' -count=1`：通过（11 个测试函数；核验修复新增拥塞控制器固定枚举与未知值反例）。
  - `cd backend && go test ./internal/assembly -run 'TestTUIC|TestMihomo11931TUIC|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41 no_test_files=5 fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 TUIC 正例 2 项、内核反例 2 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / 329 用例全部通过（Step 9 后为 328，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

- **验收：** 每协议执行 Step 4～7 的命令模板，并追加 `MIHOMO_11931_BIN=... ./.mihomo-test.sh` 中该协议正反例。

### Step 11～14：隧道与特殊 endpoint 组

#### Step 11：标准 WireGuard

- 单 Peer 与 peers 模式由 selector 控制；多 Peer 清空／忽略顶层 server、port、public-key、pre-shared-key、reserved、allowed-ips。
- 保留 R27-07 的稳定 `_credential_id`；条目移动、删除、替换只影响对应 PSK。
- reserved 统一为 3 字节；多 Peer `allowed-ips` 必填且不同 Peer 不允许相同网段。
- private-key、ip/ipv6、IP stack、DNS 和 refresh interval 按 tag 校验。
- AmneziaWG 已由第三章明确排除；本 Step 不得添加其字段、UI、输出 adapter 或测试矩阵。
- **字段逻辑：** 顶层 `private-key` 必填且为合法 Base64 WireGuard 私钥；`ip/ipv6` 至少一项存在，允许省略前缀时分别规范为 `/32`、`/128`。`peer_mode=single` 要求顶层 `server/port/public-key`，可选 `pre-shared-key/reserved/allowed-ips`，并清空 `peers`；`peers` 模式要求至少两个结构化条目，每项要求稳定 `_credential_id`、`server/port/public-key/allowed-ips`，可选独立 `pre-shared-key/reserved`，并清空顶层 peer 字段。公钥、PSK 均校验 Base64 长度；reserved 接受三字节数组或 Base64，存储／wire 统一 `[0..255]` 三整数。`allowed-ips` 逐项 CIDR 校验、去重，并拒绝不同 Peer 的相同网段。公共字段 `workers/mtu/udp/persistent-keepalive/refresh-server-ip-interval` 为非负有界整数或 bool。`ip-stack.mode` 只允许 `auto/gvisor/mips`，`congestion-controller` 只允许 `cubic/reno/bbr/bbr3`；gvisor 是否可用属于构建能力诊断。`remote-dns-resolve=false` 时清空／不输出 `dns`，开启时 DNS 列表必填且逐项校验。`amnezia-wg-option` 在 schema、已知字段白名单、adapter 和 UI 中都必须保持不存在。

**实施记录（2026-09-22）**

- **固定 tag 复核（动手前）：** 只读核对 `adapter/outbound/wireguard.go`（`WireGuardOption` 内嵌 `WireGuardPeerOption`；`IPStackOption{Mode,CongestionController}`；`AmneziaWGOption` 存在但本轮排除）。`NewWireGuard` 的 `reserved` 非空必须恰 3 字节、`Prefixes()` 缺省补 `/32`／`/128` 且两者皆空报 `missing local address`、`IPStack.validate()` 只接受 `auto/gvisor/mips` 与 `cubic/reno/bbr/bbr3`、`peers` 非空时忽略顶层 peer 字段且每项缺 `allowed-ips` 报错。**与 Build32 冻结矩阵无冲突**；本机二进制 `MIHOMO_11931_BIN` 自报 `Mihomo Meta v1.19.31`。
- **失败优先证据：** 先新增 `backend/internal/node/wireguard_protocol_test.go`（18 个测试函数）并运行 `go test ./internal/node -run 'TestWireGuard'`，实现前 **14 个测试函数真实失败**（`TestWireGuardPeerModeSelectorDeclaration`、`EndpointPolicies`、`PeersModeClearsTopLevelPeerFields`、`SingleModeClearsPeers`、`PeersRequiresAtLeastTwoEntries`、`PeersRequiresPerEntryFields`、`KeysRequireBase64AndLength`、`RequiresLocalAddress`、`LocalPrefixDefaults`、`ReservedByteSequence`、`AllowedIPsValidation`、`IPStackEnums`、`RemoteDNSResolveCondition`、`IntegerBounds`，含 12 个子用例），根因分别为「当前协议不支持 selector: peer_mode」「必须声明 single／peers 两条 endpoint policy」「顶层 reserved 必须为 byte-sequence，实际 int-list」「WireGuard schema 缺少字段 ip-stack」等。
- **后端 schema：** `registry.go` 新增 `wireGuardPeerModeField()`（`peer-mode` state_only select，`selector_name=peer_mode`，`single/peers`，默认 `single`）、`wireGuardModeField()`（分支条件＋条件必填＋`selector.peer_mode` 清空）、`wireGuardPeersField()`（结构化 list，`ItemIDField=_credential_id`，逐项 `server/port/public-key/allowed-ips` 必填、`pre-shared-key`/`reserved` 可选）、`wireGuardIPStackModeField()`／`wireGuardIPStackControllerField()`／`wireGuardIPStackField()`、`wireGuardDNSField()`；WireGuard 条目重写并声明 `Selectors`（`peer_mode`）与 **两条 `EndpointPolicies`**（single=required/required 输出；peers=hidden/hidden 不输出）。`reserved` 与 `peers[].reserved` 由 `int-list` 改为 Step 3.5 已建立的 `byte-sequence`；顶层 `public-key` 使用 `f()`＋`RequiredWhen` 而非 `req()`，避免 `validateProtocolFields` 把条件必填当无条件必填。
- **selector 派生：** `selector.go` 的 `deriveStateOnlySelector` 增加 `wireguard`／`peer_mode`：`peers` 非空派生 `peers`，否则 `single`（v1 读取与新建共用，不写库）。
- **归一化：** `normalize.go` 新增 `normalizeWireGuardAddresses()`（省略前缀补 `/32`、`/128`，空值删除，非法值留给字段级校验）与 `normalizeWireGuardPeerFields()`（每个 Peer 的 `allowed-ips` 去空白去重保序），并对顶层 `allowed-ips`／`dns` 复用 `normalizeStringListField`。
- **功能域：** `features.go` 的 `setScalarFeatures` 把 `remote-dns-resolve` 纳入标量功能域，并新增 `dns` 的 `feature.remote-dns-resolve` 条件与清空域（关闭开关即清空并不输出 `dns`）。
- **组合校验：** `project.go` 新增 `wireGuardKeyLength=32`、`wireGuardDNSSchemes`、`validateWireGuardCombination`（密钥、本地地址、非负整数、DNS）、`validateWireGuardBase64Key`、`validateWireGuardKeys`（顶层与每个 Peer 的公钥／PSK）、`validateWireGuardLocalAddresses`、`wireGuardNetworkKey`／`validateWireGuardAllowedIPs`（CIDR＋跨 Peer 同网段拒绝）、`validateWireGuardPeers`（≥2 条、每项稳定身份与必填、端口 1-65535、allowed-ips 逐项校验与冲突检测）、`validateWireGuardDNS`（条目非空、无空白、scheme 属固定 tag `parseNameServer` 集合）与 `stringListValues`，接入 `validateProtocolCombination` 的 `wireguard` 分支，覆盖创建／更新／检查／URI 导入四条路径。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `wireguardClashAdapter`，逐项映射 v1.19.31 `WireGuardOption`：single 输出顶层 `public-key/pre-shared-key/reserved/allowed-ips`；peers 只输出结构化 `peers[]` 且由 `wireguardPeerWireList` 仅复制点名 wire key（`_credential_id` 绝不进入）；`reserved` 经新增的 `node.ParseByteSequence` 以三整数数组输出；`ip-stack` 经 `copyClashActiveObject` 输出。新增 `basicOptionWithoutTFOMPTCP`：固定 tag 的 WireGuard 构造器不消费 TFO／MPTCP，因此共享 schema 保留字段但 adapter 不盲目输出。**legacy 待迁移计数 12→11**。
- **URI 收口：** `assembly/node_check.go` 新增 `wireguard` 分支：多 Peer → `core_semantic_unexpressible`（error/skip，不生成错误链接）；`ip-stack/workers/persistent-keepalive/refresh-server-ip-interval/tfo/mptcp/interface-name/routing-mark/ip-version` → `uri_partial_fields`（warn）。`links/links.go` 的 `listString` 增加 `[]int` 分支，使 `reserved` 在项目 `wireguard://` 约定下仍可无损回读（`uriparse.parseWireGuard` 已解析 3 整数 `reserved`）。
- **测试与夹具：** 新增夹具 `wireguard-single.json`（single＋reserved＋ip-stack＋DNS；clash 正例、URI warn）与 `wireguard-peers.json`（peers 模式无顶层 endpoint；clash 正例、URI skip），并在 `node_check_test.go` 登记与加入凭据泄漏断言；新增 `mihomo_wireguard_test.go`（固定内核 single／peers 两正例，reserved 两字节、非法 base64 私钥、Peer 缺 allowed-ips、非法 ip-stack mode、缺本地地址五反例）；`clash_protocols_test.go` 新增 `TestWireGuardClashAdapterWireShape` 并把 legacy 计数断言更新为 11；`server/node_test.go` 补充 peer_mode selector／两条 endpoint policy／`byte-sequence`／AmneziaWG 排除断言；`nodes-view.spec.ts` 新增 single／peers 切换、endpoint 隐藏、清空对侧字段与 reserved 编辑用例。
- **基线缺陷修复（Step 11 暴露，含根因与影响）：** 共享组合校验在**更新／检查**路径会看到保留敏感字段的**项目密文**（`mergeSensitiveWithOps` 将旧密文写回合并基底），而 Hysteria `auth`、SSH `private-key`、TUIC `uuid` 的语义校验直接把密文当明文解析。失败优先证据（新增回归用例在修复前真实失败）：`TestHysteriaKeepsSavedAuthOnUpdate` → `字段 auth: 认证必须是合法的 Base64 字符串`；`TestSSHKeepsSavedPrivateKeyOnUpdate` → `字段 private-key: 私钥必须是 PEM 内容，不能是主机文件路径或普通文本`；`TestTUICKeepsSavedCredentialsOnUpdate` → `字段 uuid: UUID 必须是合法的 UUID`。影响：用户无法在不重填凭据的情况下修改 Hysteria base64 认证、SSH 私钥、TUIC v5 节点的任何其他字段，且后续 MASQUE（EC 私钥结构校验）会重复触发。修复：新增共享判定 `isKeptCredentialCiphertext`（值以 `encPrefix` 开头即视为保留密文，跳过明文语义重解析），并在上述三处与 WireGuard 密钥校验统一使用。属 Step 11 暴露的基线缺陷，不计入本 Step 原计划字段逻辑。
- **偏差（3 项）：** ①`all_protocols_test.go` 的 `minimalProtocolParams` 为 WireGuard 提供 32 字节 Base64 密钥与本地地址（WireGuard 的本地地址是项目级组合必填，schema 中 `ip`／`ipv6` 任一即可，故不在 `Required` 中声明）；②`node_test.go` 既有 `TestWireGuardArrayCredentialsUseStablePeerIdentity`、`check_test.go` 的 WireGuard 脱敏用例、`r28_06_json_whitelist_test.go` 的 WireGuard 固定对象用例按新契约补齐 `port/public-key/allowed-ips/ip` 与合法 Base64 凭据（R27-07 断言全部保留，属契约更新非放宽）；③`links_test.go` 的 `TestNormalizeClashListFields` 不再断言 `reserved` 由 `normalizeClashFields` 归一化为 `[]int`，因为自 Step 11 起 `reserved` 是 `byte-sequence`，归一化入口在 `node.NormalizeProtocolJSON`。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestWireGuard' -count=1`：通过（18 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestWireGuard|TestMihomo11931WireGuard|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`no_test_files=5`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 WireGuard 正例 2 项、内核反例 5 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **330 用例**全部通过（Step 10 后为 329，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **排除边界核验：** 全仓 `amnezia` 命中仅为注释与**否定式**测试断言（断言 schema 不含该字段、顶层白名单拒绝 `amnezia-wg-option`），schema／已知字段白名单／adapter／UI 均无 AmneziaWG 活动字段。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

#### Step 12：Mieru

- 单端口／范围模式严格二选一；范围格式 `1-65535` 且 begin≤end。
- 修复 multiplexing 与 handshake mode 完整枚举；traffic-pattern Base64／语义错误返回字段级错误。
- transport 只允许 `TCP`／`UDP`，不做全局大小写转换。
- **字段逻辑：** `server/username/password/transport` 必填，password 为 secret；`transport` 精确允许 `TCP/UDP`。`endpoint_mode=single` 要求顶层 port 1～65535 并清空 `port-range`；`range` 把顶层 port 规范为 0，要求单个 `begin-end`，两端 1～65535 且 begin≤end。`udp` 是节点转发能力开关，不替代 transport，二者分别保存。`multiplexing` 只允许完整常量 `MULTIPLEXING_OFF/MULTIPLEXING_LOW/MULTIPLEXING_MIDDLE/MULTIPLEXING_HIGH`，`handshake-mode` 只允许 `HANDSHAKE_STANDARD/HANDSHAKE_NO_WAIT`；空值表示由内核使用默认，不将默认值强写入数据库。`traffic-pattern` 先 Base64 解码再执行 tag 语义校验；错误定位到该字段，不在日志中输出原串。wire 按 endpoint 分支严格二选一输出 `port` 或 `port-range`。

**实施记录（2026-09-22）**

- **依赖决策（用户确认，方案 B）：** 引入 `github.com/enfein/mieru/v3 v3.37.0`（与固定 tag 一致）以复用固定 tag 自身的 `apis/trafficpattern` 语义校验。**该模块为 GPL-3.0**（README 明写 "Use of this software is subject to the GPL-3 license"），本项目 `LICENSE` 为 MIT；用户已在开始前明确选择该方案并接受许可证影响。`go.mod` 新增该直接依赖，`go.sum` 相应新增 mieru 条目并把 `github.com/google/btree` 由 1.1.2 提升到 1.1.3（mieru 要求）、新增 `golang.org/x/term`；`go mod tidy` 无其他漂移。
- **固定 tag 复核（动手前）：** 只读核对 `adapter/outbound/mieru.go` 的 `MieruOption{Server,Port(omitempty),PortRange,Transport,UDP,UserName,Password,Multiplexing,HandshakeMode,TrafficPattern}` 与 `validateMieruOption`：port／port-range 严格二选一、两端 1–65535 且 begin≤end、`transport` 必须精确 `TCP`／`UDP`、username／password 必填；`MultiplexingLevel_value` 为 `MULTIPLEXING_DEFAULT/OFF/LOW/MIDDLE/HIGH`、`HandshakeMode_value` 为 `HANDSHAKE_DEFAULT/STANDARD/NO_WAIT`；`TrafficPattern` = `base64.StdEncoding` → `proto.Unmarshal` → 四组语义校验。**与 Build32 冻结矩阵无冲突**（Build32 只要求显式常量＋空值，未把 `*_DEFAULT` 纳入可编辑值）。
- **失败优先证据：** 先新增 `backend/internal/node/mieru_protocol_test.go`（9 个测试函数、31 个子用例）并运行 `go test ./internal/node -run 'TestMieru'`，实现前 **7 个测试函数真实失败**：`TestMieruEndpointModeSelectorDeclaration`（selector 缺失）、`EndpointPolicies`（policy 为空）、`PortRangeMutualExclusion`／`PortRangeValidation`（报 `当前协议不支持 selector: endpoint_mode`，无法到达端口段校验）、`TransportRequiresExactValues`、`EnumCompleteConstants`（枚举断言与 `traffic-pattern` 字段缺失）、`UDPIsIndependentFromTransport`。
- **后端 schema：** `registry.go` 新增 `mieruEndpointModeField()`（`endpoint-mode` state_only select，`selector_name=endpoint_mode`，`single/range`，默认 `single`）、`mieruPortRangeField()`（`when`／`required_when` 为 `selectors.endpoint_mode=[range]`，`reset_on=selector.endpoint_mode`）、`mieruTransportField()`（精确 `TCP/UDP`，无大小写或别名转换）、`mieruMultiplexingField()`（`""`＋四个完整常量，默认 `""`）、`mieruHandshakeModeField()`（`""`＋两个完整常量，默认 `""`）、`mieruTrafficPatternField()`（普通文本，错误不回显原值）。Mieru 条目重写并声明 `Selectors`（`endpoint_mode`）与 **两条 `EndpointPolicies`**（single=required/required 输出；range=host required＋port hidden）；旧 `LOW/MIDDLE/HIGH` 错误 wire 值与 `handshake-mode` 文本入口一并移除。
- **selector 派生：** `selector.go` 增加 `mieru`／`endpoint_mode`：存在非空 `port-range` 派生 `range`，否则 `single`（v1 读取与新建共用，不写库）。
- **归一化：** `normalize.go` 新增通用 `normalizeTrimmedTextFields()`，对 `port-range/traffic-pattern/transport/multiplexing/handshake-mode` 只做首尾去空白（空值删除），**不做大小写或别名转换**。
- **组合校验：** `project.go` 新增 `mieruPortRangeBounds()`（严格单段 begin-end；固定 tag 用 `Sscanf` 会静默接受 `1-2-3`，项目按 Build32 收紧）、`validateMieruTrafficPattern()`（`mierutp.Decode`＋`mierutp.Validate`；固定 tag 的错误文本含原串，因此项目只返回不含原值的字段级错误）与 `validateMieruCombination()`，接入 `validateProtocolCombination` 的 `mieru` 分支，覆盖创建／更新／检查／URI 导入四条路径。port／port-range 的二选一由 endpoint policy＋selector 清空域结构性保证（range 模式顶层 port 规范化为 0，single 模式清空 `port-range`）。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `mieruClashAdapter`，逐项映射 v1.19.31 `MieruOption`；`port` 由 endpoint policy 决定是否输出（range 模式不输出），`port-range` 只在 range 模式输出，`endpoint-mode`／`endpoint_mode` 永不进入 wire；未设置的枚举不写默认值。**legacy 待迁移计数 11→10**。
- **目标边界：** Mieru 无 URI 映射，`SupportsURI` 保持 false，SR／generic 稳定 `target_unsupported`／skip（无新增 URI 伪造）。
- **测试与夹具：** 新增夹具 `mieru-single.json`（single＋完整枚举；clash 正例、URI skip）与 `mieru-range.json`（range 无顶层 port；clash 正例、URI skip），在 `node_check_test.go` 登记并把 `mieru-password` 加入凭据泄漏断言；新增 `mihomo_mieru_test.go`（固定内核 single／range 两正例；port 与 port-range 并存、未知 transport、旧 `LOW` 值、非法 traffic-pattern 四反例；traffic-pattern 由固定 tag 自身编码器生成）；`clash_protocols_test.go` 新增 `TestMieruClashAdapterWireShape` 并把 legacy 计数断言更新为 10；`server/node_test.go` 补充 endpoint_mode selector／两条 endpoint policy／完整枚举／`port-range` 条件必填断言；`nodes-view.spec.ts` 新增端口段切换、隐藏端口、清空 `port-range` 与完整枚举用例。
- **偏差（1 项）：** 既有 `multiplexing` 标量功能域保留（`features.go` 的 `setScalarFeatures` 未改动），因此设置非 OFF 值时 `current_state.features` 必须包含 `multiplexing`，前端由 `activeFeatures` 自动派生；该功能域承载 `DisabledValue=MULTIPLEXING_OFF` 的关闭语义，与空值＝内核默认并不冲突。测试按协议自身派生结果构造 state，避免手写 features。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestMieru' -count=1`：通过（9 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestMieru|TestMihomo11931Mieru|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`no_test_files=5`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 Mieru 正例 2 项、内核反例 4 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **331 用例**全部通过（Step 11 后为 330，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。**依赖引入带来的许可证影响（MIT 项目链接 GPL-3.0 代码）已由用户在实施前确认，未做额外许可证文件变更。**

#### Step 13：MASQUE

- network 明确为默认 QUIC、`h2`、`h3-l4proxy`；L4 proxy 时 UDP 必须关闭且不得保存 true。
- private/public key、ip/ipv6、URI、SNI、IP stack、拥塞和 handshake timeout 按 tag 结构输出。
- 至少一个本地地址必填；密钥和地址错误需在项目校验阶段返回，不依赖内核晚失败。
- **字段逻辑：** `server/port/private-key/public-key` 必填，私钥为 secret；两类密钥必须 Base64 解码并符合 tag 所需 EC key 格式。`ip/ipv6` 至少一项，缺省前缀分别规范为 `/32`、`/128`。`network_mode=quic` 在 wire 省略或写 tag 规范值，`h2`、`h3_l4proxy` 分别映射 `network: h2/h3-l4proxy`；`h3_l4proxy` 强制 `udp=false`，切回其他模式不恢复旧值。`uri/sni/mtu/handshake-timeout/skip-cert-verify` 按类型和范围校验；URI 中的 userinfo、query、fragment 继续走项目统一凭据脱敏，不能完整进入日志或 diagnostic。`name-cert-verify` 因 tag 只是 placeholder，不进入 schema／wire。QUIC 分支活动 `congestion-controller/cwnd/bbr-profile`；h2 分支不输出无效 QUIC 调优字段。`ip-stack` 与 WireGuard 共用枚举合同。`remote-dns-resolve=false` 清空 DNS，开启时 DNS 列表必填。adapter 不允许用户密钥或未脱敏 URI 出现在 diagnostic／日志。

**实施记录（2026-09-22）**

- **固定 tag 复核（动手前）：** 只读核对 `adapter/outbound/masque.go`：`MasqueOption` 含 `Server/Port/PrivateKey/PublicKey/Ip/Ipv6/URI/SNI/MTU/UDP/HandshakeTimeout/SkipCertVerify/NameCertVerify(placeholder)/Network/CongestionController/CWND/BBRProfile/IPStack/RemoteDnsResolve/Dns`；`NewMasque` 先校验 `HandshakeTimeout < 0`，再 `base64` → `x509.ParseECPrivateKey`（私钥）与 `x509.ParsePKIXPublicKey` → 断言 `*ecdsa.PublicKey`（公钥）；`Prefixes()` 缺省补 `/32`／`/128` 且两者皆空报 `missing local address`（该分支在 `h3-l4proxy` 下被跳过）；`Network` 只有 `"h2"` 与 `"h3-l4proxy"` 特殊处理，其余落 QUIC 默认；l4proxy 分支发现 `udp=true` 时打印 warn 并强制 `outbound.udp = false`；`SetCongestionController`（`transport/tuic/common`）实现集合为 `cubic/new_reno/bbr_meta_v1/bbr_meta_v2/bbr`，与 TUIC 相同；`MTU==0 → 1280`；**`MasqueOption` 没有 `alpn` 字段**。**与 Build32 冻结矩阵无冲突**（Build32 要求 QUIC 分支省略或写 tag 规范值，项目选择省略以表达内核默认）。
- **失败优先证据：** 先新增 `backend/internal/node/masque_protocol_test.go`（12 个测试函数、20 个子用例）并运行 `go test ./internal/node -run 'TestMASQUE'`，实现前 **11 个测试函数真实失败**：`TestMASQUENetworkModeSelectorDeclaration`（selector 缺失）、`H3L4ProxyForcesUDPDisabled`／`KeysRequireECStructure`／`RequiresLocalAddress`／`NumericBounds`／`RemoteDNSResolveCondition`（报 `当前协议不支持 selector: network_mode`）、`QUICTuningOnlyActiveInQUICMode`／`URIValidationAndNoEcho`／`NumericBounds`（报字段未在注册表中声明）、`CongestionControllerEnum`／`IPStackReusesSharedEnums`（schema 缺字段）。
- **共享公共件通用化（Build32 要求「共用」而非复制）：** `registry.go` 把 `wireGuardIPStackModeField`／`wireGuardIPStackControllerField`／`wireGuardIPStackField` 重命名为 `ipStackModeField`／`ipStackControllerField`／`ipStackField`，`wireGuardDNSField` → `dnsListField`，`tuicCongestionControllerField` → `quicCongestionControllerField`，WireGuard／TUIC 条目同步改用共享名；`normalize.go` 的 `normalizeWireGuardAddresses` → `normalizeLocalAddressPrefixes`；`project.go` 的 `validateWireGuardLocalAddresses` → `validateLocalAddresses(params, label)`、`validateWireGuardDNS` → `validateDNSList`。未复制第二份枚举或地址规则。
- **后端 schema：** `registry.go` 新增 `masqueNetworkModeField()`（`network-mode` state_only select，`selector_name=network_mode`，`quic/h2/h3_l4proxy`，默认 `quic`）与 `masqueModeField()`（分支条件＋`selector.network_mode` 清空）；MASQUE 条目重写为：必填 `private-key`（secret-multiline）／`public-key`，`ip/ipv6`，`uri/sni/mtu/handshake-timeout/skip-cert-verify`，`udp` 只在 `quic/h2` 活动，`congestion-controller/cwnd/bbr-profile` 只在 `quic` 活动，共享 `ip-stack`，`remote-dns-resolve`＋条件必填 `dns`；声明 `network_mode` selector；`SensitiveFields` 仅 `private-key`。**未声明 `name-cert-verify`**。
- **组合校验：** `project.go` 新增 `validateMASQUEKeys`（私钥 Base64＋`x509.ParseECPrivateKey`；公钥 Base64＋`x509.ParsePKIXPublicKey` 且必须断言为 `*ecdsa.PublicKey`；保留密文按 `isKeptCredentialCiphertext` 跳过）、`validateMASQUEURI`（必须为带 scheme 与 host 的绝对 URL；错误只返回固定文案，不回显可能含 userinfo／query 凭据的原值）与 `validateMASQUECombination`（密钥、本地地址、URI、`mtu/cwnd/handshake-timeout` 非负整数、DNS 列表），接入 `validateProtocolCombination` 的 `masque` 分支，覆盖创建／更新／检查／URI 导入四条路径。`h3_l4proxy` 的 UDP 关闭与 `h2/h3_l4proxy` 的 QUIC 调优清空由 schema 的 `when`＋`reset_on` 与 `clearSelectorScopedFields` 结构性保证（A→B→A 不恢复）。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `masqueClashAdapter`，逐项映射 v1.19.31 `MasqueOption`；新增 `masqueNetworkToWire`（`h2`→`h2`、`h3_l4proxy`→`h3-l4proxy`；`quic` 不写 `network` 以表达内核默认）；`udp` 与 QUIC 调优字段由 selector 清空域决定是否出现；`name-cert-verify`／`network-mode` 永不进入 wire；使用 `basicOptionWithoutTFOMPTCP`（固定 tag 构造器不消费 TFO／MPTCP）。**legacy 待迁移计数 10→9**。
- **目标边界：** MASQUE 无 URI 映射，SR／generic 稳定 `target_unsupported`／skip。
- **测试与夹具：** 新增夹具 `masque-quic.json`（quic＋QUIC 调优＋ip-stack；clash 正例、URI skip）与 `masque-l4proxy.json`（h3_l4proxy 无 udp；clash 正例、URI skip），在 `node_check_test.go` 登记；新增 `mihomo_masque_test.go`（固定内核 quic／h2／h3_l4proxy 三正例；非 EC 私钥、非 ECDSA 公钥、负握手超时、非法 ip-stack mode、缺本地地址五反例）；`clash_protocols_test.go` 新增 `TestMASQUEClashAdapterWireShape` 并把 legacy 计数断言更新为 9；`server/node_test.go` 补充 `network_mode` selector、允许值、四个字段的分支清空、`name-cert-verify` 排除与共享 `ip-stack` 断言；`nodes-view.spec.ts` 新增三种网络模式切换、`h3-l4proxy` 关闭 UDP 与清空 QUIC 调优用例。
- **偏差（2 项）：** ①MASQUE 采用隐式默认 endpoint policy（required/required，未声明 `EndpointPolicies`），因为 Build32 的 policy 表未给 MASQUE 列替代或隐藏模式；②`ip/ipv6` 的「至少一项」在三种网络模式下统一要求，比固定 tag 在 `h3-l4proxy` 分支跳过 `Prefixes()` 更严格，依据 Build32 §五「本地 ip/ipv6 至少一个」与「地址错误需在项目校验阶段返回，不依赖内核晚失败」。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestMASQUE' -count=1`：通过（12 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestMASQUE|TestMihomo11931MASQUE|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`no_test_files=5`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 MASQUE 正例 3 项、内核反例 5 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **332 用例**全部通过（Step 12 后为 331，本次新增 1 个用例）；`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

#### Step 14：Tailscale

- UI 完全隐藏普通 server/port；wire 禁止输出。
- auth-key 可空；节点静态检查只返回“首次真实连接需要交互登录”的 warn，不启动 tsnet、不联网，也不伪造／回显登录 URL。真实 Mihomo 首次启动时才可能在其日志输出官方文档所述 URL。
- hostname、control-url、ephemeral、UDP、accept-routes、exit-node 和 LAN access 条件化。
- 已保存节点的 `state-dir` 由服务端注入 adapter 的稳定 `NodeID` 派生；不得接受用户路径，不在 API 回显主机绝对路径。已保存节点 check 与正式装配必须传入同一 ID；新建草稿检查使用 `NodeID=0/Persisted=false`，不输出目录、不创建目录，正式保存后才具备稳定派生值。
- **字段逻辑：** endpoint policy 固定把 host/port 规范为 `''/0`，wire 永不输出 `server/port`。可编辑字段只有 `hostname/auth-key/control-url/ephemeral/udp/accept-routes/exit-node/exit-node-allow-lan-access` 及适用的 `BasicOption`；`auth-key` 是 secret。`control-url` 为空表示官方控制面，非空必须是合法绝对 HTTP(S) URL；非 HTTPS 值显示安全提示但不擅自禁止本地 Headscale 场景。`hostname` 按 Tailscale 设备名约束。`accept-routes` 和 `exit-node-allow-lan-access` 保留 unset/false/true 三态，不能用普通 false 默认吞掉“未设置”；LAN access 只有 exit-node 非空时活动，关闭／清空 exit-node 时一并清空。`exit-node` 接受合法 IP 或 tag 支持的 `auto:*` 形式。`state-dir` 是 adapter 派生字段，不在 `protocol_json`、API schema 或请求体出现；`Persisted=true` 时要求 `NodeID>0` 并输出 `tailscale/node-<id>`，`Persisted=false` 时要求 `NodeID=0` 且禁止输出 `state-dir`。检查仅构造并验证 wire map，不得调用 Mihomo 构造器、启动 tsnet 或触碰文件系统。

- **验收：** endpoint policy、列表显示、正式装配和固定内核正反例必须一起通过；检查请求前后数据库快照相同。

**实施记录（2026-09-22）**

- **公共条件模型扩展（用户确认方案 B）：** `ConditionRule` 新增第八个维度 `non_empty []string`（规范点路径），语义为「列出的兄弟字段在 `protocol_json` 中必须存在有效值」；`Matches`／`FieldSchema.Matches`／`RequiredFor` 增加 `root map[string]any` 参数，后端 10 处调用点与 5 个测试文件同步；投影与活动校验的辅助函数（`projectFieldValue`／`projectObjectFields`／`validateActiveFieldValue`／`validateActiveObjectFields`／`validateActiveInputMaps`）把根参数向下传递，使嵌套字段也能引用根路径。前端 `ConditionRule.non_empty` ＋ `matchesCondition(rule, state, target, params)` 与后端语义一致；**缺少 params 时按不匹配处理**（fail-closed，避免依赖字段误显示）。`validateEndpointPolicy` 明确拒绝 endpoint policy 声明 `non_empty`：该阶段没有 `protocol_json` 上下文，注册即失败而不是静默忽略。
- **固定 tag 复核（动手前）：** 只读核对 `adapter/outbound/tailscale.go`：`TailscaleOption{Name,Hostname,AuthKey,ControlURL,StateDir,Ephemeral,UDP,AcceptRoutes *bool,ExitNode,ExitNodeAllowLANAccess *bool}`，**没有 server／port**；`NewTailscale` 先 `buildTailscaleMaskedPrefs`（`exit-node` 走 `ipn.ParseAutoExitNodeString` 的 `auto:` ＋非空后缀，否则 `SetExitNodeIP` 按 IP／主机名处理），再把空 `StateDir` 解析为 `"tailscale"` 并经 `C.Path.Resolve`＋`IsSafePath`；`accept-routes`／`exit-node-allow-lan-access` 是 `*bool`（真三态）；构造器**不做 hostname 校验**；`h3-l4proxy` 分支会打印 warn 并强制关闭 UDP。**与 Build32 冻结矩阵无冲突**。
- **失败优先证据：** 先新增 `backend/internal/node/tailscale_protocol_test.go`（11 个测试函数、30 个子用例）并运行 `go test ./internal/node -run 'TestTailscale'`，实现前 **10 个测试函数真实失败**：`EndpointPolicyHidesEndpoint`（policy 为空）、`EditableFieldBoundary`／`AuthKeyOptional`／`HostnameValidation`／`ControlURLValidation`／`ExitNodeValidation`（报字段未在注册表中声明）、`TriStateBooleans`／`LANAccessDependsOnExitNode`（报 schema 缺少字段）；另在实现中途发现并修复「清空文本字段会被归一化删除、导致合并退回旧值」的真问题（`normalizeTrimmedTextFields` 改为只去空白、保留空串作为显式清空信号）。
- **后端 schema：** `registry.go` 新增 `tailscaleLANAccessField()`（`exit-node-allow-lan-access`，`when.non_empty=["exit-node"]`，`group=switches`）；Tailscale 条目重写为 `hostname`／`auth-key`（可空 secret）／`control-url`／`ephemeral`／`udp`／`accept-routes`（三态，无 `default`）／`exit-node`／`exit-node-allow-lan-access`（三态，non_empty 依赖）＋ 共享 BasicOption；声明**唯一一条 hidden／hidden endpoint policy**；`SensitiveFields` 仅 `auth-key`；未声明 selector，`state-dir` 不在 schema。
- **归一化与校验：** `normalize.go` 新增 `canonicalizeTailscaleGuards`（`exit-node` 为空时删除 LAN access）并对 `hostname/control-url/exit-node` 去首尾空白；`project.go` 新增 `tailscaleHostnamePattern`（DNS label：小写字母／数字／连字符，1-63，首尾非连字符）与 `validateTailscaleCombination`（hostname、`control-url` 必须为绝对 HTTP(S) 或空、`exit-node` 必须为合法 IP 或 `auto:`＋非空后缀），接入 `validateProtocolCombination` 的 `tailscale` 分支，覆盖创建／更新／检查／URI 导入四条路径。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `tailscaleClashAdapter`：`state-dir` 只由服务端注入的稳定 `NodeID`／`Persisted` 派生（`Persisted=true` 要求 `NodeID>0` → `tailscale/node-<id>`；`Persisted=true` 且 `NodeID<=0`、或 `Persisted=false` 且 `NodeID!=0` → 直接阻断；未保存草稿不输出任何占位目录）；空 `auth-key` 返回 `tailscale_auth_key_interactive_login`（warn，文案不含任何 URL）；`http://` 控制面返回 `tailscale_control_url_insecure`（warn，不阻断本地 Headscale）；新增 `copyClashTriStateBool`（显式 false 保留、未设置省略）与 `clashTextValue`；wire 永不输出 `server`／`port`，也不输出 TFO／MPTCP。**legacy 待迁移计数 9→8**。
- **装配生命周期修复（本 Step 暴露的基线缺陷）：** `diagnoseNodeForTarget` 原先通过 `CheckNodeTarget` 包装调用，`NodeID=0`／`Persisted=false`，会把已保存节点当作未保存草稿；改为直接调用 `CheckNodeTargetDraft` 并传入装配节点的真实 `NodeID`／`Persisted=true`／`CurrentState`。影响：正式 Clash 装配对 Tailscale 的正确性，以及「正式装配缺失稳定 ID 必须阻断」这一要求的可验证性——修复后 `hasCoreBlockingNodeDiagnostic` 能真正拦住缺 ID 的节点（新增 `TestTailscaleFormalAssemblyBlocksMissingStableID` 覆盖）。
- **目标边界：** Tailscale 无 URI 映射，SR／generic 稳定 `target_unsupported`／skip；列表对 port=0 继续显示「协议自身管理」（Step 3 已实现）。
- **测试与夹具：** 新增 `backend/internal/assembly/tailscale_check_test.go`（state-dir 生命周期与稳定性、非法生命周期阻断、wire 形状与三态保真、空 auth-key warn 且不含 URL、非 HTTPS 控制面 warn、装配阶段缺稳定 ID 阻断、新建草稿检查不落库且不创建目录／文件）；新增 `mihomo_tailscale_test.go`（固定内核 2 正例＋`../escape` 不安全 state-dir 反例；正例额外断言配置检查阶段不创建 Tailscale 状态目录）；新增夹具 `tailscale-draft.json` 与 `tailscale-no-auth.json` 并登记（后者断言 `tailscale_auth_key_interactive_login` warn、URI skip）与凭据泄漏断言；`clash_protocols_test.go` 的 legacy 计数断言更新为 8；`server/node_test.go` 补充 hidden endpoint policy、两个三态 bool、`non_empty` 依赖与 `state-dir` 排除断言；`nodes-view.spec.ts` 新增无 endpoint、exit-node 条件显示、三态 false／true／未设置往返与清空 exit-node 后 LAN access 隐藏用例；`node-form-layout.spec.ts` 新增 `non_empty` 单元用例。
- **前端渲染配套：** 新增 `isTriStateBool(field)`（`bool` 且未声明 `default`）；三态 bool 不再进入集中开关区，改在「独立开关」区域内以 `AppSelect`（未设置／关闭／开启）渲染；`collectSwitchFields` 只收集声明了 `default` 的普通 bool，`groupFields` 保留三态 bool，`ProtocolFieldEditor` 新增 `rootParams` 供嵌套条件求值。既有测试夹具中未声明 `default` 的 bool 统一补 `default: false`（属契约更新：普通开关必须显式声明默认值，否则按三态处理）。
- **偏差（3 项）：** ①`exit-node` 只接受合法 IP 或 `auto:*`，比固定 tag 额外拒绝纯主机名，依据 Build32 Step 14 的点名枚举；②`udp=false` 在 wire 中省略（固定 tag `udp,omitempty` 下与 false 等价），不作为错误；③Tailscale 的 `ip/ipv6` 不适用（无本地地址字段）。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestTailscale' -count=1`：通过（11 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestTailscale|TestMihomo11931Tailscale|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`no_test_files=5`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 Tailscale 正例 2 项、内核反例 1 项（不安全 state-dir）全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **334 用例**全部通过（Step 13 后为 332，本次新增 2 个用例）；`npx vue-tsc --noEmit`、`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实 Tailscale 登录／连接或用户人工验收；保留到后续 Step 与 Step 22。检查路径只构造并校验 wire map，不启动 tsnet、不联网、不创建目录。

### Step 15～17：TLS／QUIC 新协议组

#### Step 15：AnyTLS

- plain TLS 与 ShadowTLS／Restls／JLS 三种附加安全模式互斥；禁止 Reality。
- ECH、mTLS、client fingerprint、client-metadata、session 参数和 disable-reuse 完整建模。
- 分支切换清空各自密码／脚本，不清空 AnyTLS 主密码。
- **字段逻辑：** `server/port/password` 必填，主 password 为 secret 且不随 `security_mode` 切换清除。始终可编辑 TLS 字段 `alpn/sni/ech-opts/client-fingerprint/skip-cert-verify/name-cert-verify/fingerprint/certificate/private-key`，证书／私钥成对；`ech-opts.enable=false` 清空 `config/query-server-name`。`security_mode=plain` 删除三类附加对象；`shadow_tls` 只活动 `shadow-tls-opts.password/version`；`restls` 只活动 `restls-opts.password/version-hint/restls-script`；`jls` 只活动 `jls-opts.username/password`。三个对象不得并存，所有附加 password、restls-script 和 private-key 都是敏感路径。`udp/client-metadata/idle-session-check-interval/idle-session-timeout/min-idle-session/disable-reuse` 独立活动；数值必须非负，并验证 timeout／check interval 的合理关系。wire 不输出 selector，只有当前安全对象进入 YAML。

**实施记录（2026-09-22）**

- **用户决策（Step 14 提问阶段一并确认）：** `idle-session-check-interval`／`idle-session-timeout` 采用方案 A——非负整数；显式取值只能是 0（未设置）或不小于 6 秒；且 `idle-session-timeout ≥ idle-session-check-interval`。
- **固定 tag 复核（动手前）：** 只读核对 `adapter/outbound/anytls.go` 与 `component`：`AnyTLSOption` 含 `Password/ALPN/SNI/ECHOpts/ShadowTLSOpts/RestlsOpts/JLSOpts/ClientFingerprint/SkipCertVerify/NameCertVerify/Fingerprint/Certificate/PrivateKey/UDP/ClientMetadata/IdleSessionCheckInterval/IdleSessionTimeout/MinIdleSession/DisableReuse`，**没有 reality-opts**；构造器调用三个 `Parse()`，其中 `ShadowTLSOptions{Password,Version}` 在两者皆空时返回 nil、`shadowtls.NewConfig` 的 `checkVersion` 只接受 1～3；`RestlsOptions{Password,VersionHint,RestlsScript}` 的 `version-hint` 必须在 `versionMap` 内（仅 `tls12`／`tls13`，否则报 `invalid version hint`）、脚本为空时用内置默认；`JLSOptions{Username,Password}` 要求两者同时非空；三对象互斥由内核强制（>1 报 `security modes are mutually exclusive`）。`anytls.ClientConfig` 的 `idleSessionCheckInterval <= 5s → 30s`、`idleSessionTimeout <= 5s → 30s`（**静默替换，这正是采用 ≥6 规则的依据**）。**与 Build32 冻结矩阵无冲突**。
- **失败优先证据：** 先新增 `backend/internal/node/anytls_protocol_test.go`（9 个测试函数、25 个子用例）并运行 `go test ./internal/node -run 'TestAnyTLS'`，实现前 **8 个测试函数真实失败**：`SecurityModeSelectorDeclaration`（selector 缺失）、`MainPasswordSurvivesModeSwitch`／`TLSKeyPairAndECH`／`IdleSessionRelation`（报 `当前协议不支持 selector: security_mode`）、`CamouflageObjectsMutuallyExclusive`／`CamouflageBranchRequirements`／`SessionFieldsIndependent`（报对象／字段未在注册表中声明）、`CamouflageSensitivePaths`（敏感路径仅 `password/private-key`）。
- **后端 schema：** `registry.go` 新增 **AnyTLS 专用** `anytlsSecurityModeField()`（`security-mode` state_only select，`plain/shadow_tls/restls/jls`，默认 `plain`，`group=connection`）、`anytlsCamouflageField()`／`anytlsCamouflageObject()`（对象与字段都按 `selector.security_mode` 活动、条件必填并声明清空域）与三个对象构造器 `anytlsShadowTLSOptsField()`（`password` 必填＋`version` 枚举 1～3）、`anytlsRestlsOptsField()`（`password`＋`version-hint` 必填，枚举 `tls12/tls13`，`restls-script` 为 `secret-multiline`）、`anytlsJLSOptsField()`（`username`＋`password` 必填）。**刻意不复用 SS 插件同名 `shadow-tls-opts`／`restls-opts`**，避免共享字段集合。AnyTLS 条目重写并复用 `echOptsField()`；**主 `password` 不声明 `selector.security_mode` 清空域**，因此切换分支时保留；`SensitiveFields` 扩为 `password`、`private-key`、`shadow-tls-opts.password`、`restls-opts.password`、`restls-opts.restls-script`、`jls-opts.password`；未声明 `reality-opts`（顶层白名单天然拒绝）。
- **组合校验：** `project.go` 新增 `anytlsIdleSessionMinimum = 6` 与 `validateAnyTLSCombination`（mTLS 成对复用 `validateTLSKeyPair`、三个会话整数非负、check interval／timeout 只能是 0 或不小于 6、timeout ≥ check interval），接入 `validateProtocolCombination` 的 `anytls` 分支，覆盖创建／更新／检查／URI 导入四条路径。三类伪装的互斥、必填与切换清空完全由 schema 的 `when`／`required_when`／`reset_on` ＋ `clearSelectorScopedFields` 结构性保证。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `anytlsClashAdapter`，逐项映射 v1.19.31 `AnyTLSOption`；`alpn` 走数组复制、`ech-opts` 走 `copyClashEnabledObject`（禁用不写 wire）；新增 `anytlsCamouflageWireKeys`，只输出实际存在的那一个伪装对象；`security-mode`／`security_mode` 永不进入 wire；BasicOption 使用完整白名单（固定 tag 构造器消费 TFO／MPTCP）。**legacy 待迁移计数 8→7**。
- **URI 收口（Step 15 暴露的基线缺口）：** 项目 `anytls://` 只表达密码、SNI、ALPN、客户端指纹、`skip-cert-verify` 与 `udp`。此前 `linkTargetDiagnostics` **没有 `anytls` 分支**，mTLS、三类伪装对象与全部会话／ECH 调优字段会被静默丢弃但仍标 complete。修复：新增 `anytls` 分支——mTLS 与任一附加伪装对象 → `core_semantic_unexpressible`（error／skip，不返回 preview）；`name-cert-verify`／`fingerprint` → `unverified_compatibility`（warn）；启用的 `ech-opts` 与 `client-metadata`／`idle-session-*`／`min-idle-session`／`disable-reuse` → `uri_partial_fields`（warn）。新增 `TestAnyTLSURIDiagnosticsNeverSilentlyDropActiveFields` 锁定「可表达不阻断、不可表达必 skip／warn」两类行为。
- **测试与夹具：** 新增 `backend/internal/assembly/mihomo_anytls_test.go`（固定内核 plain＋会话调优、shadow-tls、jls 三正例；ShadowTLS 非法版本、Restls 非法 version-hint、JLS 缺 username 三反例）、`clash_protocols_test.go` 的 `TestAnyTLSClashAdapterWireShape` 与 URI 诊断用例、夹具 `anytls-plain.json`（clash 与 URI 双正例）与 `anytls-shadow-tls.json`（URI skip），并把三个凭据加入泄漏断言；`server/node_test.go` 补充 selector、允许值、三对象清空域、主密码不清空与 Reality 排除断言；`nodes-view.spec.ts` 新增四种安全模式切换、主密码保留、A→B→A 清空与 ECH 关闭清空用例。
- **偏差（2 项）：** ①ShadowTLS `password` 与 Restls `version-hint` 在项目侧条件必填：固定 tag 允许空密码＋版本产生「无实际伪装」的空配置，项目按「必要凭据缺失阻止保存」收紧；`version-hint` 限定为固定 tag 唯一的两种取值 `tls12`／`tls13`（tag 对大小写不敏感，项目按规范小写值枚举）。②三个伪装对象与安全模式控件归入 `connection` 分组（与 Snell 的 obfs 对象一致），避免落入默认折叠的高级区。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestAnyTLS' -count=1`：通过（9 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestAnyTLS|TestAnyTLSURI|TestMihomo11931AnyTLS|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`no_test_files=5`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 AnyTLS 正例 3 项、内核反例 3 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **335 用例**全部通过（Step 14 后为 334，本次新增 1 个用例）；`npx vue-tsc --noEmit`、`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

#### Step 16：ShadowQUIC

- username、password、SNI、ALPN、QUIC v1/v2、UOT、0-RTT、keepalive、拥塞／窗口完整建模。
- 0-RTT 开启时显示重放风险提示；提示不替代服务端验证。
- **字段逻辑：** `server/port/username/password` 必填，password 为 secret；用户名／密码必须成对。TLS 仅暴露 `sni/alpn`，固定 tag option 没有 `skip-cert-verify`、证书或 ECH 字段，前端不得从通用 TLS helper 误加。`quic-versions` 为有序去重列表，只接受 tag parser 支持的 v1/v2 表达；空列表使用内核默认。`udp-over-stream`、`zero-rtt` 为独立 bool；关闭 UOT 不产生附加版本字段。`keep-alive-interval/cwnd/recv-window-conn/recv-window/max-datagram-frame-size/max-open-streams` 为非负整数，`up/down` 为可选带单位速率，`congestion-controller/bbr-profile` 按 tag 枚举／格式校验，`disable-mtu-discovery` 为 bool。开启 0-RTT 产生固定风险提示但不改变保存结果；非活动或默认值不强写入数据库。

**实施记录（2026-09-22）**

- **固定 tag 复核（动手前）：** 只读核对 `adapter/outbound/shadowquic.go`：`ShadowQuicOption` 含 `Server/Port/Username/Password/SNI/ALPN/QUICVersions/UDPOverStream/ZeroRTT/KeepAliveInterval/CongestionController/Up/Down/CWND/BBRProfile/ReceiveWindowConn/ReceiveWindow/DisableMTUDiscovery/MaxDatagramFrameSize/MaxOpenStreams`；**没有 `skip-cert-verify`、证书、ECH 与 UOT 版本字段**；TLS 只使用 `ServerName`（SNI 为空回退 server）与 `ALPN`；`ParseQUICVersions` 只识别 `v1/1/rfc9000/rfc-9000` 与 `v2/2/rfc9369/rfc-9369` 并在内部去重，空列表用 `DefaultQUICVersions()`；`ZeroRTT` 走 `DialQuicOption{Early:true}` 并依赖会话票据缓存（重放语义）；缺省 `MaxDatagramFrameSize=1400`、`MaxOpenStreams=1024`、`CWND=32`；`Up/Down` 经 `utils.StringToBps`；拥塞控制器由 `transport/shadowquic/congestion.go` 提供 `cubic/new_reno/bbr_meta_v1/bbr_meta_v2/bbr`（与 TUIC 同一集合）。**与 Build32 冻结矩阵无冲突**。
- **失败优先证据：** 先新增 `backend/internal/node/shadowquic_protocol_test.go`（7 个测试函数、22 个子用例）并运行 `go test ./internal/node -run 'TestShadowQUIC'`，实现前 **6 个测试函数真实失败**：`CredentialPair`／`VersionsNormalization`／`UOTAndZeroRTT`／`BandwidthAndCongestion`（schema 缺字段）、`TLSBoundary`（缺 `alpn`）、`NonNegativeIntegers`（全部字段未声明）。
- **后端 schema：** `registry.go` 新增 `shadowQUICVersionsField()`（`quic-versions` text-list）；ShadowQUIC 条目重写为 `username`／`password` 双必填，`sni`／`alpn`／`quic-versions`／`udp-over-stream`／`zero-rtt`／`keep-alive-interval`／共享 `quicCongestionControllerField()`／`up`／`down`／`cwnd`／`bbr-profile`／`recv-window-conn`／`recv-window`／`disable-mtu-discovery`／`max-datagram-frame-size`／`max-open-streams`；**未声明 `skip-cert-verify`／`certificate`／`private-key`／`ech-opts`／任何 UOT 版本字段**；`SensitiveFields` 仅 `password`；不声明 LinkMappings。
- **归一化与校验：** `normalize.go` 对 `quic-versions` 复用 `normalizeStringListField`（去空白、去重、保序）；`project.go` 新增 `shadowQUICVersionAllowed`（只接受 `v1`／`v2`，大小写不敏感；固定 tag 另接受的 `1/2/rfc*` 不在 Build32 的规范表达内）与 `validateShadowQUICCombination`（版本列表逐项、六个非负整数、`up/down` 可选非零速率复用 `validBandwidth`），接入 `validateProtocolCombination` 的 `shadowquic` 分支，覆盖创建／更新／检查／URI 导入四条路径。`zero-rtt` 的重放风险只在 adapter 以 warn 提示，不改变保存结果，也不替代服务端验证。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `shadowquicClashAdapter`，逐项映射 v1.19.31 `ShadowQuicOption`；`alpn`／`quic-versions` 走数组复制；使用 `basicOptionWithoutTFOMPTCP`（固定 tag 构造器不消费 TFO／MPTCP）；开启 `zero-rtt` 返回 `shadowquic_zero_rtt_replay_risk`（warn，含重放语义）。**legacy 待迁移计数 7→6**，剩余 legacy 为 ss／vmess／vless／trojan／openvpn／trusttunnel（属 Step 17～20 范围）。
- **目标边界：** ShadowQUIC 无 URI 映射，SR／generic 稳定 `target_unsupported`／skip，不伪造 URI。
- **测试与夹具：** 新增 `mihomo_shadowquic_test.go`（固定内核默认版本＋UOT＋0-RTT 与显式有序版本两正例；`v3`、`v1+bogus` 两反例）与 `clash_protocols_test.go` 的 `TestShadowQUICClashAdapterWireShape`（全字段 wire 形状、版本保序、禁用字段排除、0-RTT warn、未设置不写 wire）；新增夹具 `shadowquic-basic.json`（clash 正例＋0-RTT warn，URI skip）并加入凭据泄漏断言；`server/node_test.go` 补充必填、TLS 边界、禁用字段与无 URI 映射断言；`nodes-view.spec.ts` 新增版本列表、UOT／0-RTT 独立、无附加版本字段与「未设置 ≠ 0」用例。
- **偏差（2 项）：** ①`quic-versions` 只接受规范 `v1`／`v2`，比固定 tag 额外接受 `1/2/rfc9000/rfc9369` 更严格，依据 Build32「只接受 tag parser 支持的 v1／v2 表达」；②`congestion-controller` 复用 TUIC 的共享枚举字段（同一 `SetCongestionController` 实现集合），`bbr-profile` 保持普通文本由固定 tag 透传（Build32 未要求枚举）。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestShadowQUIC' -count=1`：通过（7 个测试函数）。
  - `cd backend && go test ./internal/assembly -run 'TestShadowQUIC|TestMihomo11931ShadowQUIC|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`no_test_files=5`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN=~/mihomo-bins/mihomo-v1.19.31-go122 ./.mihomo-test.sh`：退出码 0；新增 ShadowQUIC 正例 2 项、内核反例 2 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **336 用例**全部通过（Step 15 后为 335，本次新增 1 个用例）；`npx vue-tsc --noEmit`、`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **legacy 计数终值：** `legacyAdapterPendingCount() == 6`，与「12→11→10→9→8→7→6」预期序列一致；**Step 20 的 legacy=0 尚未达到**。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

#### Step 17：TrustTunnel

- username/password 成对；TLS/ECH/mTLS 字段完整。
- QUIC 开关控制拥塞参数；连接复用以 selector 表达互斥模式，`max-connections/min-streams` 与 `max-streams` 不得同时活动。
- health-check、UDP 和复用分支分别验证。
- **字段逻辑：** `server/port` 必填；`username/password` 必须同时为空或同时非空，password 为 secret。TLS 字段为 `alpn/sni/ech-opts/client-fingerprint/skip-cert-verify/name-cert-verify/fingerprint/certificate/private-key`，证书／私钥成对，ECH 关闭清空子字段。`udp/health-check/quic` 独立 bool；`quic=false` 清空并禁止输出 `congestion-controller/cwnd/bbr-profile`。`reuse_mode=none` 清空三个复用数字；`connections` 只活动正整数 `max-connections/min-streams`；`streams` 只活动正整数 `max-streams`。两组不得混合，selector 切换不恢复旧值。adapter 只输出当前分支字段，credential 和 ECH／mTLS 清空必须覆盖 check 与正式装配。

- **验收：** 每协议定向 node／assembly／frontend 测试和固定 v1.19.31 正反例通过，敏感值不进入响应、日志或 check preview。

**实施记录（2026-09-23）**

- **用户决策（Step 17 提问阶段确认）：** ①`reuse_mode=connections` 分支要求正整数 `max-connections` 必填、`min-streams` 可选正整数（依据固定 tag 语义：只填 `min-streams` 时内核每条流都新建连接，该参数实际无效）；`streams` 分支对称要求 `max-streams` 必填。②固定内核二进制改用用户提供的官方 `go124` 资产（见本文第二章更新）。
- **固定 tag 复核（动手前）：** 只读核对 `/private/tmp/mihomo-v1.19.31`（`HEAD=ab405bad5beeeac8b003bb01f60f134f6df54471`、`git describe = v1.19.31`）的 `adapter/outbound/trusttunnel.go`、`transport/trusttunnel/client.go`、`adapter/outbound/ech.go`：`TrustTunnelOption` 含 `Server/Port/UserName/Password/ALPN/SNI/ECHOpts/ClientFingerprint/SkipCertVerify/NameCertVerify/Fingerprint/Certificate/PrivateKey/UDP/HealthCheck/Quic/CongestionController/CWND/BBRProfile/MaxConnections/MinStreams/MaxStreams`，embed `BasicOption`（构造器消费 `TFO`／`MPTCP`）。**关键结论：** ①复用三个数字在 `NewTrustTunnel` 中**没有任何校验或互斥错误**，只有 `transport/trusttunnel/client.go` 的优先级（`max-connections>0` 时忽略 `max-streams`，三值全 0 时默认 8／5）——Build32 的「两组不得混合」因此是项目 selector／adapter 层合同，与 Build32 v1.15 已确立的「无法由固定 Mihomo 证明的混合凭据拒绝改为项目 selector／adapter wire 互斥合同」同类，**不构成停止条件**。②`NewClient` 的 ALPN 规则：`quic=true` 时空 ALPN 默认 `h3`、非空必须含 `h3`（否则 `require alpn h3`）；`quic=false` 时空 ALPN 默认 `h2`、非空必须含 `h2`（否则 `require alpn h2`）——这两条成为本步的内核反例。③`quic` 是普通 bool（`omitempty`，内核默认 false＝HTTP/2 隧道）；`cwnd==0→32`；未知 `congestion-controller` 被内核静默忽略。**与 Build32 冻结矩阵无冲突**。
- **失败优先证据：** 先新增 `backend/internal/node/trusttunnel_protocol_test.go`（7 个测试函数、9 个子用例）并运行 `go test ./internal/node -run 'TestTrustTunnel'`，实现前 **5 个测试函数真实失败**：`CredentialPair`（缺 `username`）、`TLSAndECH`（缺 `alpn`）、`ReuseModeSelector`（`Selectors` 为空）、`ReuseBranchNotRestored`（`min-streams` 未在注册表中声明）、`QUICGate`（缺 `udp`）。`UnsetNotPersisted`／`NoURIMapping` 为锁定型断言，实现前后均通过。
- **后端 schema：** `registry.go` 把占位条目重写为完整 TrustTunnel：新增 `trustTunnelReuseModeField()`（`reuse-mode` state_only select，`none/connections/streams`，默认 `none`，`group=connection`）、`trustTunnelReuseNumber(name,label,mode,required)`（按 `selector.reuse_mode` 活动、`ResetOn=[selector.reuse_mode]`，connections 的 `max-connections` 与 streams 的 `max-streams` 条件必填）、`trustTunnelQUICField()`（`quic` 独立 bool，默认 false＝HTTP/2）与 `trustTunnelQUICTuningField()`（按 `feature.quic` 活动、`ResetOn=[feature.quic]`）；TLS 组复用既有 `echOptsField()`（该 helper 注释本就写明 TrustTunnel 共用）与 `tlsSubField` 无关的多行类型 `certificate`／`private-key`；`udp` 复用项目约定默认 `true`；`SensitiveFields` 扩为 `password`、`private-key`；未声明 LinkMappings（SR／generic 稳定 skip）。`features.go` 的 `setScalarFeatures` 白名单新增 `quic`，使「关闭即清空」走既有 `cleanDisabledFeatures` 链而**不新增第二套机制**。
- **组合校验：** `project.go` 新增 `validateTrustTunnelCombination`（用户名／密码必须同时为空或同时非空且按缺失方报字段级错误、`validateTLSKeyPair` 复用 mTLS 成对、两组复用数字同时命中时阻断、三个复用数字必须是正整数、`cwnd` 非负整数），接入 `validateProtocolCombination` 的 `trusttunnel` 分支，覆盖创建／更新／检查／URI 导入四条路径。分支活动、条件必填与切换清空由 schema 的 `when`／`required_when`／`reset_on` ＋既有 `clearSelectorScopedFields` 结构性保证。
- **selector 派生：** `selector.go` 的 `deriveStateOnlySelector` 新增 `trusttunnel` 分支（`deriveTrustTunnelReuseMode`：命中 `max-connections`／`min-streams` 为 `connections`；仅 `max-streams` 为 `streams`；都无值为 `none`），并新增 `hasNumberParam` 供数字字段的显式存在判断复用。v1 读取只在内存派生、不回写。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `trusttunnelClashAdapter`，逐项映射 v1.19.31 `TrustTunnelOption` 的点名 wire key；`alpn` 走 `copyClashListFields` 数组复制、`ech-opts` 走 `copyClashEnabledObject`（禁用不写 wire）、三个复用数字与 QUIC 调优字段走 `copyClashActiveFields`、BasicOption 使用**完整白名单**（固定 tag 构造器消费 TFO／MPTCP）；`reuse-mode`／`reuse_mode` 与全部 state_only 永不进入 wire。**legacy 待迁移计数 6→5**，剩余 legacy 为 `ss／vmess／vless／trojan／openvpn`。
- **测试与夹具：** 新增 `backend/internal/assembly/mihomo_trusttunnel_test.go`（固定内核正例 3 项：quic+connections+调优、h2+streams+ECH 关闭、全默认；内核反例 2 项：`quic=true` 配 `alpn=["h2"]` 报 `require alpn h3`、`quic=false` 配 `alpn=["h3"]` 报 `require alpn h2`）、`clash_protocols_test.go` 的 `TestTrustTunnelClashAdapterWireShape`（connections／streams 分支键集合、selector 不进 wire、quic 关闭不输出调优字段、ECH 关闭不输出对象、TFO／MPTCP 保留）；新增夹具 `trusttunnel-connections.json`（clash 正例＋URI skip）与 `trusttunnel-streams.json`（streams＋ECH 启用＋mTLS 成对，URI skip），并把 `trusttunnel-password`、`trusttunnel-private-key` 加入凭据泄漏断言；`server/node_test.go` 补充 selector、允许值、分支清空域、必填规则、quic 功能开关、敏感路径与无 URI 映射断言；`nodes-view.spec.ts` 新增 connections／streams 分支显隐、A→B→A 不恢复、`quic` 关闭清空调优字段与 state_only 不进入 `protocol_json` 用例。
- **偏差（3 项）：** ①复用两组互斥由项目 selector／adapter 合同保证（固定 tag 只做优先级，不报错），已在定向测试中用「connections 分支提交 `max-streams` 后不得落库」锁定。②`connections`／`streams` 分支的必填比固定 tag 更严（内核允许只填 `min-streams` 或全空），依据用户本条决策。③`udp` 采用项目既有默认 `true`（固定 tag 的 Go 零值为 false），与其余 13 个已迁移协议保持一致；另外**ECH 启用路径不在固定内核证据内**——固定 tag 在 `enable=true` 且未提供有效 `ECHConfigList` 时会走 resolver 查询，因此与既有 SS 插件门禁一致只验证 `enable=false` 时对象不写入 wire，启用侧由 schema／adapter 的定向测试覆盖。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestTrustTunnel' -count=1`：通过（7 个测试函数、9 个子用例）。
  - `cd backend && go test ./internal/assembly -run 'TestTrustTunnel|TestMihomo11931TrustTunnel|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN='/Users/kyle/Desktop/Repo/Temp/mihomo-darwin-arm64-go124-v1.19.31' ./.mihomo-test.sh`：退出码 0；新增 TrustTunnel 正例 3 项、内核反例 2 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **337 用例**全部通过（Step 16 后为 336，本次新增 1 个用例）；`npx vue-tsc --noEmit`、`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `node scripts/check-md-links.mjs Build32.md`：`missing=0`；`git diff --check`：退出码 0。
- **legacy 计数终值：** `legacyAdapterPendingCount() == 5`，与「12→11→10→9→8→7→6→5」预期序列一致；**Step 20 的 legacy=0 尚未达到**。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

### Step 18：OpenVPN 结构化模型与 adapter

- 移除 editor schema 中 `client-config`，新增第五章字段矩阵对应的结构化字段。
- `auth_mode=userpass/cert/cert_userpass` 与 `tls_key_mode` 为 state-only selector；CA、cert、key、tls key 使用明确 multiline 类型。
- OpenVPN wire adapter 只输出 v1.19.31 `OpenVPNOption` 字段；禁止输出原始 `.ovpn` 或未知指令。
- 旧 `client-config` 不自动解释、不透传。若 Step 0.5 在任何目标数据库发现遗留行，立即停止在 Step 18 前，由用户决定迁移／清除策略；不得靠删除 schema 字段使旧内容在下一次保存时静默丢失。只有确认无遗留行，或另行获得明确处置授权后，才能移除编辑入口。
- **字段逻辑：** `server/port/ca` 必填，CA 为 multiline 非 secret；`proto` 只允许 `udp/tcp`，`dev` 固定为 `tun`。`cipher`／`data-ciphers`／`data-ciphers-fallback` 只允许 tag 支持的 `AES-128/192/256-GCM`、`AES-128/192/256-CBC`、`CHACHA20-POLY1305`；`auth` 只允许 `MD5/SHA1/SHA256/SHA384/SHA512`，`comp-lzo` 只保留 tag 可接受值，列表去空白去重。`auth_mode=userpass` 只活动成对必填 `username/password` 并清空 cert/key；`cert` 只活动成对必填 PEM `cert/key` 并清空 username/password；`cert_userpass` 同时活动且要求完整的 `username/password` 与 PEM `cert/key`，不得清除任一组。两组均空或任一活动组只填一半均返回字段级 400；password、key 为 secret。三种 selector 互相切换时只清除新分支不再活动的凭据，例如 `cert_userpass→cert` 清除 username/password，`cert_userpass→userpass` 清除 cert/key，A→B→A 不恢复。`tls_key_mode=none` 清空全部 TLS key；`tls_auth` 要求 `tls-auth`，`key-direction` 只允许 `0/1/空`；`tls_crypt` 与 `tls_crypt_v2` 分别只活动对应 key，三组严格互斥且均为 secret。`peer-info` 是 string map，键值限制同 headers；`ping/ping-restart/handshake-timeout/mtu` 非负，`tran-window` 必须保留 unset 与显式 0 的区别。`udp` 是代理转发能力，不等同于 `proto`。`ip-stack` 复用 WireGuard 枚举；`remote-dns-resolve=false` 清空 DNS，开启时 DNS 列表必填。adapter 只输出 `OpenVPNOption` 的点名 wire key，绝不输出 selector、导入行号或原文。
- **验收：** user/pass、cert/key、cert＋user/pass 组合、认证组缺半反例、三种 tls key、TLS key 互斥反例、字段脱敏与固定内核正反例通过。

**实施记录（2026-09-23）**

- **用户决策（Step 18 提问阶段确认）：** ①不存在外部遗留 `client-config` 数据、不需要兼容层，因此直接移除编辑入口，不保留只读回显。②OpenVPN 认证组切换采用**新增可选字段属性**方案实现「仅清除非活动凭据」，对既有 13 个协议零影响；`data-ciphers`／`data-ciphers-fallback` 收紧到 `cipher` 同一组 7 值，`comp-lzo` 收紧到 `yes`／`no`／`adaptive`／空。
- **固定 tag 复核（动手前）：** 只读核对 `/private/tmp/mihomo-v1.19.31` 的 `adapter/outbound/openvpn.go`、`transport/openvpn/config.go`、`transport/openvpn/tlscrypt_v2.go`、`adapter/outbound/wireguard.go`：`OpenVPNOption` 含 `Proto/Dev/Cipher/DataCiphers/DataCipherFallback/Auth/CompLZO/CA/Cert/Key/TLSAuth/KeyDirection/TLSCrypt/TLSCryptV2/Username/Password/PeerInfo/Ping/PingRestart/TranWindow/HandshakeTimeout/MTU/UDP/IPStack/RemoteDnsResolve/Dns`，embed `BasicOption`（构造器消费 `TFO`／`MPTCP`）。`NewOpenVPN` → `cfg.Prepare()` 先归一化再 `ValidateInstallScriptSubset`，因此：`ca` 必填且必须 PEM；`cert`／`key` 必须成对，**且允许与 username／password 并存**（`authUser` 始终写入 key-method-2 记录）→ `cert_userpass` 合法；无 cert/key 时 username 必填；`proto` 只接受 udp／tcp（`udp4`／`tcp-client` 等归一化后接受）；`dev` 非 `tun` 直接报错；`cipher` 恰为 7 值、`auth` 恰为 5 值；`key-direction` 仅 `0`／`1`／空；`tls-auth`＋`tls-crypt` 互斥、`tls-crypt-v2` 与二者互斥；静态 key 必须 256 字节；`TranWindow` 是 `*int`，**确实区分未设置与显式 0**。`data-ciphers`／`data-ciphers-fallback` 完全不校验、`comp-lzo` 只把 `yes`／`adaptive` 归一化为 `yes`，故按用户决策由项目侧收紧。**`client-config` 在该 tag 不存在且无兼容路径**。与 Build32 冻结矩阵无冲突。
- **共享基线增补（本步唯一跨协议改动）：** 新增 `FieldSchema.ClearWhenInactive`（`clear_when_inactive`）声明式属性，表达「仅当字段在新 selector 状态下不再活动时才清空」。注册期门禁：`field_types.go` 的 `validateProtocolFieldTypes` 要求声明该属性的字段必须同时声明 `When.Selectors`，否则阻断启动。前端 `api/node.ts` 增加同名类型、`nodeFeatures.ts` 新增 `clearInactiveSelectorFields`、`NodesView.clearScopedFields` 在 `selector.<name>` 作用域内叠加该清空链——与后端既有 `clearSelectorScopedFields` 语义对齐。既有 13 个协议的 selector 字段全部同时声明 `reset_on` 与单/多分支 `when`，因此该改动对它们**行为中性**；全量前端 47 文件/338 用例确认无回归。
- **失败优先证据：** 先新增 `backend/internal/node/openvpn_protocol_test.go`（7 个测试函数、24 个子用例）并运行 `go test ./internal/node -run 'TestOpenVPN'`，实现前 **6 个测试函数真实失败**：`RemovedClientConfig`（`client-config` 仍在 schema）、`AuthModeSelector`（`Selectors` 为空）、`AuthCombinations`（`ca`／`cert`／`username` 未声明）、`AuthBranchDirectionalClearing`（缺字段）、`TLSKeyModes`（缺字段）、`FieldContracts`（缺字段）。`NoURIMapping` 为锁定型断言，实现前后均通过。
- **后端 schema：** `registry.go` 把占位条目重写为完整 OpenVPN：`auth-mode`／`tls-key-mode` 两个 state_only selector 字段；新增 `openvpnAuthField`（按 `auth_mode` 多分支活动、条件必填，**只声明 `clear_when_inactive` 不声明 `reset_on`**，避免在 `cert_userpass↔cert`／`↔userpass` 之间误清仍活动的凭据）、`openvpnTLSKeyField`（三个互斥 TLS key，单分支可用无方向 `reset_on`）、`openvpnKeyDirectionField`（仅 `tls_auth` 分支，选项 `""/0/1`）、`openvpnCipherField`／`openvpnDataCiphersField`／`openvpnFallbackCipherField`／`openvpnAuthDigestField`／`openvpnCompLZOField`；`req("ca","multiline")`、`sel("proto",...,"udp","udp","tcp")`、`sel("dev",...,"tun","tun")`、`dnsListField()`、`ipStackField()`、`openMap("peer-info")`、`def("udp",...)` 与后续由 `setScalarFeatures` 注册的 `remote-dns-resolve` 功能复用既有公共件；`SensitiveFields` = `password`／`key`／`tls-auth`／`tls-crypt`／`tls-crypt-v2`（`ca`／`cert` 明确非敏感）。`normalize.go` 的 `normalizeProtocolListFields` 增加 `openvpn` 分支（`data-ciphers`／`dns` 去空白去重保序）。
- **组合校验：** `project.go` 新增 `validateOpenVPNCombination`（`proto`／`dev` 固定值、`cipher`／`data-ciphers`／`data-ciphers-fallback` 的 7 值集合、`auth` 5 值集合、`comp-lzo` 项目侧枚举、`key-direction` 取值、`peer-info` 键值合同（与 HTTP headers 同规则）、`ping`／`ping-restart`／`tran-window`／`handshake-timeout`／`mtu` 非负整数），接入 `validateProtocolCombination` 的 `openvpn` 分支；认证组完整性与三种 TLS key 互斥由 schema 的 `when`／`required_when`／`clear_when_inactive`／`reset_on` ＋ `clearSelectorScopedFields` 结构性保证。
- **selector 派生：** `selector.go` 的 `deriveStateOnlySelector` 新增 `openvpn` 分支：`deriveOpenVPNAuthMode`（两组都出现→`cert_userpass`；只有 cert／key→`cert`；否则 `userpass`）与 `deriveOpenVPNTLSKeyMode`（`tls-crypt-v2`＞`tls-crypt`＞`tls-auth`／`key-direction`＞`none`）。v1 读取只在内存派生、不回写。
- **Clash adapter：** `assembly/clash_protocols.go` 注册 `openvpnClashAdapter`，逐项映射 v1.19.31 `OpenVPNOption` 的点名 wire key；`data-ciphers`／`dns` 走 `copyClashListFields`、`peer-info`／`ip-stack` 走 `copyClashActiveObject`、BasicOption 使用完整白名单；新增 `copyClashOptionalZeroInt` 让 `tran-window` 的**显式 0 进入 wire**（通用活动值过滤会跳过 0，而固定 tag 用 `*int` 表达该区别）。selector、导入行号、原始 `.ovpn` 与 `client-config` 永不输出。**legacy 待迁移计数 5→4**，剩余 legacy 为 `ss／vmess／vless／trojan`。
- **测试与夹具：** 新增 `backend/internal/assembly/mihomo_openvpn_test.go`（固定内核正例 3 项：userpass＋tls-auth＋调优、cert_userpass＋tls-crypt＋DNS、cert＋tls-crypt-v2；内核反例 10 项：缺 ca、非 PEM ca、cert 缺 key、无证书且无用户名、tls-auth 与 tls-crypt 并存、非法 cipher、非法 dev、非法 proto、静态 key 长度不足、非法 key-direction）、`clash_protocols_test.go` 的 `TestOpenVPNClashAdapterWireShape`（userpass／cert_userpass 两个分支的键集合、selector 与 `client-config` 不进入 wire、`tran-window` 显式 0 保留、未设置整数不写 0）；新增夹具 `openvpn-userpass.json` 与 `openvpn-cert-userpass.json`（含 mTLS＋tls-crypt＋DNS＋peer-info＋显式 tran-window），凭证泄漏断言新增 `openvpn-password`／`openvpn-private-key`／`openvpn-tls-auth`／`openvpn-tls-crypt`；`server/node_test.go` 新增 OpenVPN 分支断言（删除 `client-config`、两个 selector 的允许值、`clear_when_inactive` 与「不得声明无方向 reset_on」、`ca` 必填 multiline、敏感路径集合与非敏感 `ca`／`cert`）；`field_types_test.go` 移除已不存在的 `client-config` 类型断言，`node_test.go` 的空凭据摘要用例改用 `tailscale`；`nodes-view.spec.ts` 新增 `cert_userpass→cert` 只清 userpass 组且凭证操作只含 `password`、`A→B→A` 不恢复、`userpass→cert_userpass` 保留仍活动凭据用例。
- **偏差（3 项）：** ①`data-ciphers`／`data-ciphers-fallback`／`comp-lzo` 由项目侧收紧（固定 tag 完全不校验），依据用户本条决策。②`dev` 以单值 select 暴露（固定 tun），使 Step 19 parser 有同名映射目标，值仍由 `validateOpenVPNCombination` 复核。③项目**不**对 `ca`／`cert`／`key` 做 PEM 与静态 key 长度校验（固定 tag 会拒绝）；该边界由固定内核反例证明，避免过度防御。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'TestOpenVPN' -count=1`：通过（7 个测试函数、24 个子用例）。
  - `cd backend && go test ./internal/assembly -run 'TestOpenVPN|TestMihomo11931OpenVPN|TestNodeCheckFixtures|TestLegacyAdapterPendingCount' -count=1`：通过。
  - `cd backend && go test ./... -count=1`：`ok=41`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `MIHOMO_11931_BIN='/Users/kyle/Desktop/Repo/Temp/mihomo-darwin-arm64-go124-v1.19.31' ./.mihomo-test.sh`：退出码 0；新增 OpenVPN 正例 3 项、内核反例 10 项全部通过。
  - `cd frontend && npm test -- --run`：47 文件 / **338 用例**全部通过（Step 17 后为 337，本次新增 1 个用例）；`npx vue-tsc --noEmit`、`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `node scripts/check-md-links.mjs Build32.md`：`missing=0`；`git diff --check`：退出码 0。
- **legacy 计数终值：** `legacyAdapterPendingCount() == 4`，与「12→…→6→5→4」预期序列一致；**Step 20 的 legacy=0 尚未达到**。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；保留到后续 Step 与 Step 22。

### Step 19：`.ovpn` 解析导入

- 实现第四章有界、只读、无文件系统读取的 parser 和管理员路由。
- 前端 OpenVPN 区提供“粘贴 `.ovpn`”面板：解析、显示 diagnostics、查看脱敏结构、显式应用／取消。
- parser 草稿与现有自定义／JSON 草稿纳入页面级阻断；切换协议或关闭面板清空未应用原文。
- API 日志只记录长度、结果计数和错误 code，不记录原文、remote 凭据或内嵌块。
- `no-store` 中间件必须注册在 session／admin 之前；匿名、普通用户、超限、解析失败和成功响应均测试响应头。
- **字段逻辑：** parser 将 `remote` 映射顶层 host/port，将 `proto/dev/cipher/data-ciphers/data-ciphers-fallback/auth/comp-lzo/ping/ping-restart/peer-info` 映射同名结构化字段，将 `<ca>/<cert>/<key>/<tls-auth>/<tls-crypt>/<tls-crypt-v2>` 去标签后映射内容；`auth-user-pass` 只标记 userpass 能力，不读取引用文件、不制造 username/password。只有 `auth-user-pass` 时选择 `userpass`，只有完整 cert/key 时选择 `cert`，两者同时存在时选择合法的 `cert_userpass`；cert/key 缺半才以 `ovpn_conflicting_auth` 阻断，不得把组合认证误判为冲突。`key-direction` 只与 tls-auth 同时应用。重复同值指令可合并，互相冲突的单值指令、多个不同 remote 或多种 TLS key 必须阻断。响应同时给每个已映射字段来源行号；未知安全普通指令只 warn，脚本／hook／include／外部文件引用 400。点击应用时只覆盖 parser 明确产出的字段，并为被替换 selector 添加 reset scope；未产出的现有草稿字段不应被“空响应”静默删除，除非用户确认全量替换。原文、行号、diagnostics 都不进入最终保存请求。
- **验收命令：**
  ```bash
  cd backend
  go test ./internal/node -run 'Test.*OpenVPN'
  go test ./internal/server -run 'Test.*OpenVPN'
  go run ./cmd/errgate ./...
  cd ../frontend
  npm test -- --run nodes-view protocol-field-editor
  npm run build
  cd ..
  git diff --check
  ```

**实施记录（2026-09-23）**

- **固定 tag 复核（动手前）：** 只读核对 `/private/tmp/mihomo-v1.19.31` 的 `transport/openvpn/config.go`／`tlscrypt_v2.go`：`Prepare()` 先归一化再校验，`ca` 必须 PEM；`tls-auth`／`tls-crypt` 静态 key 必须 256 字节十六进制；`tls-crypt-v2` 客户端 PEM 类型为 `OpenVPN tls-crypt-v2 client key` 且正文须超过 256 字节。这些事实决定了 parser 映射与阻断边界，也说明 parser 只需产出结构化草稿、无需复制内核的 PEM／长度校验。
- **有界只读 parser：** 新增 `backend/internal/node/openvpn_import.go`：`MaxOpenVPNParseBytes = 256 KiB`，纯内存逐行解析，**不读取任何外部文件、不执行脚本／hook／include、不落库**（函数无 store 依赖）。解析结果为 `host/port/protocol_json/selectors/field_sources/diagnostics`，**永不含原文**；`field_sources` 给每个已映射字段的来源行号；阻断时清空可应用草稿并返回 `*OpenVPNParseError{Code}`，诊断仍随结果返回。
- **映射与边界：** `remote host [port]`→顶层 host／port（缺端口给 `ovpn_remote_port_missing` warn）；`proto`（`udp4`／`tcp-client` 等归一化）、`dev`（仅 tun）、`cipher`（含 `AES-CBC`→`AES-128-CBC` 别名）、`data-ciphers`（按 `:` 拆分、去重保序）、`data-ciphers-fallback`、`auth`（含 `SHA-1`→`SHA1`）、`comp-lzo`（无参视为 yes）、`ping`／`ping-restart`／`tran-window`／`handshake-timeout`／`mtu`（非负整数）、`key-direction`（0／1）、`peer-info KEY VALUE`（字符串 Map）、`auth-user-pass`（**只标记能力，引用文件只 warn 不读取、不制造 username/password**）；`<ca>/<cert>/<key>/<tls-auth>/<tls-crypt>/<tls-crypt-v2>` 去标签后映射内容，`ca/cert/key/tls-* inline` 声明视为无操作。认证推导：`auth-user-pass`＋完整 cert/key→`cert_userpass`（**不误报冲突**）、仅完整 cert/key→`cert`、仅 `auth-user-pass`→`userpass`、都没有→默认 `userpass` 并给 `ovpn_no_auth_directive` warn；TLS key 模式按出现的块推导。阻断：多个不同 remote→`ovpn_multiple_remotes`、cert/key 缺半→`ovpn_conflicting_auth`、多种 TLS key→`ovpn_conflicting_tls_key`、互相冲突的单值指令→`ovpn_conflicting_directive`、未闭合内嵌块→`ovpn_unclosed_inline_block`、inline 之外的文件引用→`ovpn_external_file_forbidden`、脚本／hook／include／权限变更→`ovpn_dangerous_directive`、枚举／整数越界→`ovpn_unsupported_value`、超限→`ovpn_size_exceeded`；未知但安全的普通指令只 warn（`ovpn_unsupported_directive`）。除 Build32 冻结的 6 个 code 外新增的 code 同样是稳定英文标识，前端只按 code 与 severity 决策。
- **HTTP 路由与顺序：** `server/node.go` 新增 `POST /api/admin/nodes/openvpn/parse`，注册在**独立分组** `engine.Group("/api/admin/nodes", noStoreMiddleware(), sessionMW, adminMW)`——沿用邮件模板与发送日志的既有做法，保证匿名 401、非管理员 403、超限 413、解析失败 400 与成功 200 都带 `Cache-Control: no-store`。仅接受 `application/json`；handler 级 `http.MaxBytesReader`（256 KiB＋16 KiB JSON 余量）与 parser 的 256 KiB 正文上限**两条路径都映射 413**；400 响应体为 `{code,message,error_code,diagnostics}`。日志只记录 `bytes`／`mapped_fields`／`diagnostics`／`error_code`，**不记录原文、内嵌块或凭据**。
- **前端面板：** 新增 `frontend/src/components/OpenVPNImportPanel.vue`：粘贴→解析→按 `code`＋`severity`＋行号展示诊断→脱敏结构（敏感字段只提示「已解析」不显示取值）→显式「应用解析结果」／「取消」。原文与结果只在组件内存，**不写入 localStorage**；关闭面板、切换协议、应用或取消都会丢弃原文并解除阻断。`api/node.ts` 增加 `parseOpenVPN` 与结果／阻断类型；`request.ts` 的 `ApiError` 增加可选 `details`（携带响应体，用于读取 `error_code`／`diagnostics`，属附加字段、不影响既有调用方）。`NodesView.vue`：OpenVPN 协议下渲染面板（`openvpnImportOpen` 折叠，位于「认证与密钥」与「独立开关」之间）；`applyParsedOpenVPN` **先应用 selector（触发分支清空与 `selector.<name>` reset scope）再合并解析产出的结构化字段与 endpoint**，未产出的既有草稿字段保持不变（不提供全量替换，因此不存在静默删除路径）；面板草稿经 `draft-dirty-change` 进入既有 `unappliedControlPaths`，从而纳入保存／检查阻断与「定位草稿」。行号、diagnostics 与原文都不进入 `NodeForm`。
- **失败优先证据：** 先新增 `backend/internal/node/openvpn_import_test.go`（6 个测试函数、24 个子用例）对未实现的 `ParseOpenVPN` 运行，**6 个测试函数全部真实失败**（`ovpn_not_implemented` 与「阻断时未返回诊断」）；失败优先完成后才实现 parser。
- **测试与夹具：** `node/openvpn_import_test.go` 覆盖映射与来源行号（含 `remote` 第 4 行、`proto` 第 3 行的精确断言）、三种认证推导与组合认证不误报、20 项阻断反例、256 KiB 超限、注释与空行、诊断不回显原文；`server/openvpn_parse_test.go`（4 个测试函数、7 个子用例）覆盖 no-store 先于 session／admin（401／403 均带头）、成功草稿、危险指令 400＋`error_code`＋诊断、正文超限 413、请求体超限 413、非 JSON 400、非法 JSON 400、零落库、以及**日志不含 `ca`／`tls-auth` 标记、`auth-user-pass` 与 remote 主机名**；`frontend/tests/openvpn-import-panel.spec.ts`（5 个用例）覆盖脱敏结构、阻断诊断、应用载荷只含解析产出字段、取消、关闭面板丢弃；`nodes-view.spec.ts` 新增集成用例覆盖「未应用原文阻断保存→显式应用→host/port/字段合并且既有 `username` 草稿保留→切换协议关闭面板并解除阻断」。
- **偏差（2 项）：** ①`ApiError` 增加可选 `details` 字段（附加、非破坏性），用于把 400 的 `error_code`／`diagnostics` 传给面板；否则前端只能显示笼统文案而无法按行号定位。②不实现「全量替换」路径：Build32 允许「未产出字段不删除」或以明确确认做全量替换二者之一，本步取前者（合并语义），因此没有会静默删除既有草稿的入口。
- **定向与联合门禁（全部实际执行）：**
  - `cd backend && go test ./internal/node -run 'Test.*OpenVPN' -count=1`：通过（OpenVPN 协议合同 7 函数＋解析器 6 函数）。
  - `cd backend && go test ./internal/server -run 'Test.*OpenVPN' -count=1`：通过（4 个测试函数、7 个子用例）。
  - `cd backend && go test ./... -count=1`：`ok=41`、`fail=0`；`go build ./...`、`go vet ./...`、`gofmt -l ./internal/ ./cmd/`（无输出）、`go run ./cmd/errgate ./...`（0 违规）通过。
  - `cd backend && go test ./internal/assembly -run 'TestLegacyAdapterPendingCount' -count=1`：通过，**legacy 计数保持 4**（Step 19 不减少 legacy adapter）。
  - `cd frontend && npm test -- --run nodes-view protocol-field-editor openvpn-import-panel`：3 文件 / 101 用例通过；`npm test -- --run`：**48 文件 / 344 用例**全部通过；`npx vue-tsc --noEmit`、`npm run build` 通过（仅既有 main chunk 体积提示）。
  - `git diff --check`：退出码 0。
- **未执行项：** 本步未运行 `go test -race`、Docker 构建、API／浏览器 smoke、真实客户端导入／连接或用户人工验收；也未用真实 `.ovpn`（含真实证书／密钥）做人工验证——解析器测试全部使用合成文本，固定内核不参与本步（Step 19 不产出 wire）。保留到 Step 20 与 Step 22。

### Step 20：目标诊断与正式装配收口

- 19 协议 `clash-yaml` 均走显式 adapter；不支持 URI 的 8 项返回稳定 skip：Snell、Mieru、MASQUE、OpenVPN、SSH、ShadowQUIC、TrustTunnel、Tailscale。
- 对已有 URI 映射的协议只输出无损字段；无法表达的活动字段产生 `core_semantic_unexpressible` 或 `unverified_compatibility`。
- `target_evidence` 必须由检查／装配实际消费，不能只是 UI 标签。
- `CheckClashContent` 增加 15 协议关键 shape 和互斥门禁；内核接受但项目不允许的结构仍由项目拒绝。
- **字段逻辑：** 为每个协议冻结 `clash-yaml/sr-subs/generic-subs` 的字段能力表，状态只能从实际 adapter 结果计算：所有活动字段可无损表达才为 `ok/complete`；存在被舍弃但不阻断的字段为 `warn/partial`；目标无映射为 `skip/unsupported` 且 `preview=null`；必填、互斥或安全边界失败为 `error`。`target_evidence` 的 field path 必须和 schema canonical path 一致，不能只按协议给笼统标签。Clash adapter 必须剥离 `state_only`、`_credential_id`、导入元数据、未知非目标扩展和非活动字段；URI adapter 必须逐字段声明可表达集合，任何活动 secret 或高级字段不能静默丢弃后仍 complete。check 与正式装配使用相同的 name/type/endpoint/协议字段构造函数，仅预览脱敏发生在构造之后的副本上。
- **验收：** 节点检查预览与正式装配单节点片段语义相同；skip 不带 preview，不报 500。

### Step 21：全协议前端回归

- 覆盖 selector、动态 endpoint、当前组合、独立开关、高级折叠、多行 secret、列表、错误定位和草稿阻断。
- 375px 与桌面 920px；明暗主题；长证书／私钥；WireGuard Peer 排序；OpenVPN 导入 diagnostics。
- 切换协议提示改为按 endpoint policy 说明，不再统一声称保留 server/port。
- 前端不得维护第二份协议字段或枚举全集；所有协议差异来自后端 schema／policy。
- **字段逻辑：** `text/password/number/bool/select/object/text-list/int-list/multiline/secret-multiline/byte-sequence` 均由单一 schema 渲染；`state_only` 控件读写 `current_state.selectors`，不混入 `protocol_json`。`when/required_when/reset_on/feature/endpoint policy` 决定显示、必填、清空和错误定位，隐藏字段必须从提交草稿和 credential ops 中同步移除。number 保持“未设置”与 0 区别；三态 bool 保持 unset/false/true；list/object 使用稳定 item ID，secret 只显示 configured／keep／replace／clear 状态。切换 selector、协议、endpoint 模式、OpenVPN 导入应用前统一检查未应用自定义值／JSON／parser 草稿，并展示将清除的字段名称和凭据数量但不展示值。后端返回的 field path 必须能定位折叠区、Peer 条目和 OpenVPN 行级 diagnostic。
- **验收命令：**
  ```bash
  cd frontend
  npm test -- --run node-form-layout node-features protocol-field-editor node-check-panel nodes-view
  npm test -- --run
  npm run build
  ```

### Step 22：联合门禁、隔离 smoke 与归档

```bash
cd backend
go test ./...
go test -race ./...
go build ./...
go vet ./...
go run ./cmd/errgate ./...

cd ../frontend
npm test -- --run
npm run build

cd ..
MIHOMO_11931_BIN='/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo' ./.mihomo-test.sh
docker compose build
git diff --check
```

- 使用全新临时 `DATA_DIR`／隔离账号做 API 和浏览器 smoke，不清理现有数据。
- smoke 至少覆盖：普通 endpoint、Tailscale 无 endpoint、Mieru range、WireGuard peers、TUIC v4/v5、AnyTLS 分支、OpenVPN 解析／应用／保存／重开。
- 证据分层记录：源码／单测、固定内核、API、浏览器、真实客户端／连接、用户人工验收。
- 真实客户端和连接未执行时明确登记未执行；不得以 Docker 或浏览器 smoke 替代。
- 完成后同步 Design4 当前事实、AGENTS 当前入口和必要人工清单，再把 Build32 移入 `docs/reports/Build/`。文档同步属于本 Step，必须基于实际结果，不得预写完成。
- 归档前执行 `node scripts/check-md-links.mjs Build32.md Design4.md AGENTS.md`；归档移动后重新执行链接检查，避免相对路径因目录变化失效。
- **字段逻辑：** 本 Step 不新增字段；以注册表导出的 19 协议 schema manifest 作为最终合同，逐协议比较字段名、类型、默认值、枚举、selector、endpoint policy、敏感路径、reset scope、target evidence 和 wire key。manifest 与 Step 4～20 已验收结果不一致即失败。隔离 smoke 对每个代表分支执行“创建／检查／保存／读取／重开／切换／清除／装配”，并对数据库原始 JSON、API 脱敏响应和最终 YAML 三层分别断言；任何 secret 出现在 list/get/check preview/log/error 或非目标输出即阻断归档。文档只记录实际通过的字段矩阵和未执行人工项，不把预定字段写成已完成。

---

## 九、最低自动化矩阵

每个协议至少有以下测试，不得只复用 `minimalProtocolParams()`：

1. 最小合法结构和固定 v1.19.31 接受；
2. 每个 selector 正分支；
3. 每个 selector 缺必填／互斥反例；
4. A→B→A 不恢复参数、凭据、扩展或草稿；
5. 新建、保存、读取、重开一致；
6. credential replace／keep／clear；
7. 节点检查不落库；
8. 非活动参数不进入 Clash／URI；
9. 固定 Mihomo 正反例；
10. 无 URI 映射时稳定 skip；
11. check preview 与正式 adapter 一致；
12. 375px／桌面关键交互和错误定位。

专项矩阵另加：

- endpoint：普通必填、替代、隐藏、请求残值清空、wire 不输出；
- OpenVPN：大小限制、多 remote、inline block、外部路径拒绝、未知指令、脱敏、不落库，以及 userpass、cert、cert＋userpass 三种认证正例、各组缺半反例、三态切换清空和组合 `.ovpn` 导入；
- WireGuard：条目稳定 ID、重排、删除、PSK keep/clear、reserved 两种输入；
- Tailscale：已保存 check／装配使用同一 NodeID、派生目录稳定、重命名不变、缺失／伪造生命周期拒绝、新建草稿不输出或创建目录、路径安全、空 auth-key warn；
- URI：可表达字段往返、不表达字段阻断／warn、无映射稳定 skip。

---

## 十、文档阶段验收结果

- 已核对当前 19 协议注册表、保存／检查／投影、Clash 输出、URI 路径、前端动态表单和现有测试结构。
- 已以 Mihomo v1.19.31 tag 源码核对 15 个后续协议的 option shape，并用本机二进制确认版本；本轮未执行批量 `-t` 协议验收，该工作属于 Step 1 及各协议 Step。
- 已把用户确认的三项方向转换为可执行合同：新版本证据、endpoint policy、OpenVPN 结构化＋解析导入。
- 已确认不需要数据库 schema migration；状态格式使用现有列升 v2，endpoint 空值使用现有 NOT NULL 列中的 `''`／`0`。
- AmneziaWG 范围已由用户拍板：不并入 Build32，留作后续独立专项；当前没有其他已知的实施前产品决策项。
- 已补齐 selector 注册／v1 读取规则、逐 Step 产出文件与共同完成门槛、OpenVPN API 状态码／no-store／secret 生命周期，以及遗留 `client-config` 的停止条件。
- 已更正 Tailscale 边界：静态检查只提示首次真实连接需要交互登录，不生成登录 URL；`state-dir` 由稳定节点 ID 派生且新建草稿检查零文件系统副作用。
- 已按 v1.19.31 固定 tag 修正 OpenVPN 认证合同：用户名密码、客户端证书以及二者组合均为合法模式；只有认证组缺半或两组均空才阻断，`.ovpn` 导入不得把组合认证误判为冲突。
- 已补齐统一 adapter 的节点生命周期输入：服务端注入 `NodeID/Persisted`，已保存 Tailscale check 与正式装配共用稳定 ID，新建草稿不生成 `state-dir`。
- 已逐 Step 补充字段逻辑，覆盖输入／selector／显示与必填／清空与凭据／wire 输出／诊断；并以固定 tag 源码纠正 Snell reuse、Hysteria 带宽字段和 SOCKS5 无独立 SNI 三处边界。
- 文档定稿阶段只修改本文档；其后用户已授权实施，Step 0.5～3 已产生业务代码、前后端测试与门禁脚本改动，并作为提交 `7ecb6d5` 保存完成；Step 3-fix 已补齐该提交遗漏的前端静态颜色门禁。当前成果不代表 race、Docker、API／浏览器 smoke、真实客户端连接或用户人工验收通过。

---

## 十一、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.24 | 2026-09-23 | 完成 Step 19 `.ovpn` 只读解析导入：新增有界（256 KiB）纯内存 parser（`node/openvpn_import.go`），不读取外部文件、不执行脚本／hook／include、不落库；`remote`→host/port、结构化指令与 `<ca>/<cert>/<key>/<tls-auth>/<tls-crypt>/<tls-crypt-v2>` 去标签映射并给出逐字段来源行号；`auth-user-pass` 只标记能力（引用文件只 warn、不制造用户名密码）；完整 cert/key＋auth-user-pass 推导为合法的 `cert_userpass` 而非冲突；多 remote／cert-key 缺半／多种 TLS key／冲突单值／未闭合内块／inline 之外文件引用／脚本 hook include／枚举越界均按稳定 code 阻断，未知安全指令只 warn。新增 `POST /api/admin/nodes/openvpn/parse`，注册在 `noStoreMiddleware()` 先于 session/admin 的独立分组，401／403／400／413／200 均带 `no-store`；日志只记录长度、映射字段数与 error code。前端新增 `OpenVPNImportPanel.vue`（解析→按 code/行号展示诊断→脱敏结构→显式应用／取消，原文只在内存、不入 localStorage），`NodesView` 应用时先应用 selector 再合并解析产出字段并保留未产出的既有草稿，解析草稿纳入页面级阻断，切换协议／关闭面板丢弃原文；`ApiError` 增加可选 `details` 以读取 400 的 `error_code`／`diagnostics`。失败优先 6 项、node 定向 13 函数、server 定向 4 函数/7 子用例、前端 48 文件/344 用例、后端全量 41 包、legacy 计数保持 4 与 build/vet/gofmt/errgate/`git diff --check` 全部通过。 |
| v1.23 | 2026-09-23 | 完成 Step 18 OpenVPN 结构化模型与 adapter：移除已无兼容路径的原始 `client-config` 编辑入口，按键值矩阵建立 `auth_mode=userpass/cert/cert_userpass` 与 `tls_key_mode=none/tls_auth/tls_crypt/tls_crypt_v2` 两个 state_only selector；`ca` 必填 multiline 非 secret，`password`／`key`／三种 TLS key 为 secret；`proto` 限 udp/tcp、`dev` 固定 tun、`cipher`／`data-ciphers`／`data-ciphers-fallback`／`auth` 按固定 tag 收紧（`comp-lzo` 另按用户决策收紧）；`tran-window` 保留未设置与显式 0 并在 wire 输出显式 0；`remote-dns-resolve` 关闭清空 DNS、开启时 DNS 必填；`peer-info` 与 headers 同规则；OpenVPN 显式 Clash adapter（legacy 5→4）与 SR／generic 稳定 skip。为满足「只清除新分支不再活动的凭据」新增可选字段属性 `clear_when_inactive`（后端注册期门禁＋前端 `clearInactiveSelectorFields`，对既有 13 个协议行为中性）。失败优先 6 项、node 定向 7 函数/24 子用例、assembly 夹具 2 项与 wire 形状测试、后端全量 41 包、前端 47 文件/338 用例、固定内核正例 3＋反例 10 与 `git diff --check`、Markdown 链接检查全部通过。 |
| v1.22 | 2026-09-23 | 完成 Step 17 TrustTunnel：固定内核二进制改由用户提供的官方 `go124` 资产（解压后 SHA-256 `3ed36cab…`、内嵌 revision `ab405bad…`、已复验上游 asset digest）并在第二章记录；`reuse-mode` state_only selector（`none/connections/streams`）表达连接复用互斥，`connections` 分支要求正整数 `max-connections` 必填、`min-streams` 可选，`streams` 分支要求 `max-streams` 必填（依据用户决策与固定 tag「只填 min-streams 时内核失效」语义），两组数字不得同时落库；username／password 必须同时为空或同时非空且 password／private-key 为 secret；TLS／ECH／mTLS 字段完整、证书与私钥成对、ECH 关闭清空子字段；`udp`／`health-check`／`quic` 独立 bool，`quic` 经 `setScalarFeatures` 注册为功能开关使关闭即清空 `congestion-controller/cwnd/bbr-profile`；TrustTunnel 显式 Clash adapter（legacy 6→5）与 SR／generic 稳定 skip。失败优先 5 项、node 定向 7 函数/9 子用例、assembly 夹具 2 项、后端全量 41 包、前端 47 文件/337 用例、固定内核正例 3＋反例 2（ALPN 与 quic 不匹配的两条真实内核拒绝）与 `git diff --check`、Markdown 链接检查全部通过。 |
| v1.0 | 2026-09-21 | 完成 19 个 manual 协议后续专项研究并建立 Build32；冻结 Mihomo v1.19.31、selector/state v2、endpoint policy、15 协议矩阵、显式 wire adapter、OpenVPN 结构化＋`.ovpn` 解析、串行 Step 与验收门禁。所有代码 Step 未获授权、未开始；AmneziaWG 保留为唯一待确认候选。 |
| v1.1 | 2026-09-21 | 按用户确认冻结方案 A：Build32 的 WireGuard 只覆盖标准单 Peer／多 Peer，AmneziaWG 明确排除并留作后续独立专项。本次确认不授权进入 Step 0.5，全部代码 Step 继续保持未开始。 |
| v1.2 | 2026-09-21 | 进一步实施定稿：补齐 selector 持久化与 v1 派生矩阵、endpoint 防伪造、OpenVPN API 安全响应合同、逐 Step 文件／前置／完成定义和共同门槛；增加 legacy adapter 归零与遗留 `client-config` 停止条件；更正 Tailscale 登录提示和稳定 `state-dir` 边界。仍未授权任何代码 Step。 |
| v1.3 | 2026-09-21 | 为 Step 0.5～22 逐项补充字段逻辑：字段集合、selector、条件必填、清空、敏感路径、wire 映射、diagnostic 与最终 manifest；按 Mihomo v1.19.31 固定 tag 纠正 Snell reuse、Hysteria 规范带宽入口和 SOCKS5 无独立 SNI 等细节。仍只修订文档，未授权代码实施。 |
| v1.4 | 2026-09-21 | 核验修正：OpenVPN 认证改为 userpass／cert／cert_userpass 三态，允许固定 tag 支持的证书＋用户名密码组合并同步导入与测试合同；统一 adapter 增加服务端注入的 NodeID/Persisted，明确 Tailscale 已保存 check／装配和新建草稿的稳定 state-dir 生命周期。仍只修订文档，未授权代码实施。 |
| v1.5 | 2026-09-22 | 实施状态同步与中断恢复：记录用户已授权 Step 0.5～22 串行实施，Step 0.5～3 已在本地未提交工作区完成并通过独立复验，Step 4～22 未开始；补充工作区保护、精确恢复入口、已通过门禁与未执行证据边界，清理过期的“未授权”和“仅修改文档”表述。 |
| v1.6 | 2026-09-22 | 二次恢复核验与 Step 3-fix：更正“Step 0.5～3 位于本地未提交工作区／HEAD `f00b350`”的过时描述为提交 `7ecb6d5`（`beta` 与 `origin/beta` 一致、工作区干净）；记录全量前端复核发现的 Step 3 静态颜色门禁缺口（`NodesView.vue:810` `text-gray-500`）及其最小修正为设计 Token `text-text-tertiary`，修正后前端 47 文件/316 用例与生产构建通过；记录固定 Mihomo v1.19.31 二进制失效与经用户确认的官方 release 重新获取路径、SHA-256 与门禁通过结果，以及数据目标零遗留复核结论。仍只修订文档与 1 行前端样式，未进入 Step 3.5／Step 4。 |
| v1.7 | 2026-09-22 | 完成 Step 3.5 公共字段类型增补：新增 `field_types.go`（类型白名单、`secret-multiline` 敏感性门禁、byte-sequence 三输入规范化与递归嵌套处理）、`validateFieldValue`／`NormalizeProtocolJSON`／`protocolIndex` 接入；既有大文本与私钥字段显式重定型为 `multiline`／`secret-multiline` 且不改变敏感性；前端按显式类型渲染并新增 byte-sequence 三整数＋Base64 控件，删除 `isLongText` 名称启发式。失败优先 4 项、node 包定向 8 用例、后端全量 41 包、前端 47 文件/321 用例、build/vet/gofmt/errgate、固定 v1.19.31 门禁与 `git diff --check` 全部通过；`reserved` 重定型等属 Step 11。Step 4 未开始。 |
| v1.9 | 2026-09-22 | 完成 Step 5 SOCKS5：与 HTTP 共用 `basicAuthModeField`／`basicAuthCredential`／`tlsFeatureField`／`tlsSubField` 与 `validateTLSKeyPair`，schema 不声明 `sni`，UDP 保持独立开关；新增 `socks5ClashAdapter`（v1.19.31 `Socks5Option`，legacy 18→17）与 URI 降级诊断（mTLS skip、证书校验 warn）；新增夹具、固定内核正反例、wire 形状与前端用例。后端全量 41 包、前端 47 文件/324 用例、build/vet/gofmt/errgate、固定 v1.19.31 门禁与 `git diff --check` 全部通过；legacy 样本测试改为动态选择。 |
| v1.8 | 2026-09-22 | 完成 Step 4 HTTP：`auth_mode` state-only selector（none／basic，缺省按凭据派生）、TLS 标量 feature 条件字段与关闭清空、认证／mTLS 成对校验、headers 字符串 Map 五类规则、selector 分支清空接入创建／更新／检查、HTTP 显式 Clash adapter（v1.19.31 `HttpOption`＋BasicOption 白名单，legacy 19→18）、HTTP URI 降级诊断（mTLS skip、headers／证书校验 warn）、前端 `state_only` 控件与 `current_state.selectors` 提交。失败优先 8 项、node 定向 9 用例、后端全量 41 包、前端 47 文件/323 用例、固定内核 HTTP 正例＋2 反例与 `git diff --check` 全部通过；legacy 测试样本改用 socks5。 |
| v1.10 | 2026-09-22 | 完成 Step 6 SSH：`auth_mode=password/private_key` state-only selector 与缺省派生、分支互斥与切换清空另一组凭据、`private-key` 只接受可解析 PEM（拒绝主机文件路径，使用与固定内核一致的 `golang.org/x/crypto/ssh`）、加密私钥口令生命周期、`host-key` authorized-key 语法校验与 `host-key-algorithms` 去空白去重保序、空 `host-key` 安全 warn、SSH 显式 Clash adapter（v1.19.31 `SshOption`，`host-key*` 输出 YAML 数组、永不输出 `udp`，legacy 17→16）、无 URI 映射协议稳定 `target_unsupported`／`skip` 与 `SupportsURI` 注册表。失败优先 10 项、node／assembly／server 定向与全量 41 包、前端 47 文件/325 用例、固定内核 SSH 正例 2＋反例 2 与 `git diff --check` 全部通过；`host-key` 显式重定型为 `text-list` 并同步既有测试合同。 |
| v1.11 | 2026-09-22 | 完成 Step 7 Snell：`version` 普通 selector（1～5，缺省 v1）与 `obfs_mode` state-only selector（none／http／tls／shadow_tls／restls／jls）、v1/v2 禁 UDP 与 v2 固定 reuse、v4/v5 可编辑 reuse、五类混淆分支字段与条件必填、`obfs-opts.mode` 由 selector 注入 wire、结构化对象禁未知键、v5 wire 保留 `version: 5` 并给出 v4 兼容诊断、Snell 显式 Clash adapter（v1.19.31 `SnellOption`，legacy 16→15）、SR/generic 稳定 `target_unsupported`／skip。同步修复两项 Step 7 暴露的基线缺陷：`hydrateCurrentStateForRead` 不再覆盖显式 selector（检查／诊断与正式装配对 http/tls 分支保持一致）、前端 `selectorValueFor`／`setField` 支持普通 selector 的 `source_field` 投影与清空域。失败优先 12 项、后端全量 41 包、前端 47 文件/326 用例、固定内核 Snell 正例 3＋反例 2 与 `git diff --check` 全部通过。 |
| v1.12 | 2026-09-22 | 完成 Step 8 Hysteria：`auth_mode=none/base64/string` state-only selector 与 `auth`／`auth-str` 分支互斥、Base64 校验、`up/down` 唯一编辑入口（`up-speed/down-speed` 归一化后删除旧键）、`protocol` 枚举与 `obfs-protocol` 别名收敛、端口跳跃语法校验、TLS／ECH／mTLS、接收窗口关系校验、Hysteria 显式 Clash adapter（v1.19.31 `HysteriaOption`，legacy 15→14）、URI 固定 `auth-str`／mTLS skip 与高级项 warn 诊断、URI 带宽归一化为 Mbps 数字。失败优先 9 项、后端全量 41 包、links 包定向、前端 47 文件/327 用例、固定内核 Hysteria 正例 2＋反例 2 与 `git diff --check` 全部通过。 |
| v1.13 | 2026-09-22 | 完成 Step 9 Hysteria2：`endpoint_mode=single/ports` 与 `obfs_mode=none/salamander/gecko` state-only selector、single／ports 两条 endpoint policy（ports 隐藏顶层 port 并输出 `ports`）、`hop-interval` 单值／单范围且最小 5 秒、gecko 包大小 min≤max、Realm 子树与 token／private-key 敏感路径、四个 QUIC window initial≤max、Hysteria2 显式 Clash adapter（v1.19.31 `Hysteria2Option`，obfs 由 selector 注入、禁用 ECH／Realm 不写入 wire，legacy 14→13）、URI 对端口组／Realm／mTLS 稳定 skip。同步修复 `CheckClashContent` 硬编码 server／port 必填的基线缺陷，改由协议 endpoint policy 推导。失败优先 7 项、后端全量 41 包、前端 47 文件/328 用例、固定内核 Hysteria2 正例 2＋反例 2 与 `git diff --check` 全部通过。 |
| v1.14 | 2026-09-22 | 完成 Step 10 TUIC：`auth_mode=v4/v5` state-only selector 与 token／UUID＋密码强互斥、UUID 与 `ip` 校验、UOT 开关与版本 0 归一化、`disable-sni` 清空冲突 SNI 并给出中间人风险 warn、数据报帧 1400 上限与中继包联动校验、TUIC 显式 Clash adapter（v1.19.31 `TuicOption`，legacy 13→12）、URI 对 v4 token／mTLS 稳定 skip。失败优先 10 项、后端全量 41 包、前端 47 文件/329 用例、固定内核 TUIC 正例 2＋反例 2 与 `git diff --check` 全部通过。 |
| v1.21 | 2026-09-22 | 完成 Step 16 ShadowQUIC（Step 11～16 的最后一步）：`username`／`password` 成对必填且 password 为 secret；TLS 只暴露 `sni`／`alpn`，schema 明确排除固定 tag 没有的 `skip-cert-verify`／证书／私钥／ECH 与任何 UOT 版本字段；`quic-versions` 为有序去重列表且只接受 v1／v2 表达（空列表用内核默认，不强写）；`udp-over-stream`／`zero-rtt`／`disable-mtu-discovery` 为独立 bool；六个窗口／保活／流控字段非负整数、`up/down` 为可选带单位速率、`congestion-controller` 复用共享 QUIC 枚举；开启 0-RTT 只返回固定重放风险 warn，不改变保存结果；ShadowQUIC 显式 Clash adapter（legacy 7→6）与 SR／generic 稳定 skip。**legacy 计数到达预期的 12→11→10→9→8→7→6**。失败优先 6 项、node 定向 7 项、后端全量 41 包、前端 47 文件/336 用例、固定内核 ShadowQUIC 正例 2＋反例 2 与 `git diff --check` 全部通过。Step 11～16 已全部验收通过，按用户授权要求在此停止，不进入 Step 17。 |
| v1.20 | 2026-09-22 | 完成 Step 15 AnyTLS：`security_mode=plain/shadow_tls/restls/jls` state_only selector、三种附加伪装对象严格互斥与切换清空（A→B→A 不恢复）、主 password 不随分支切换清除、始终可编辑 TLS／ECH／client-fingerprint／mTLS 字段且证书与私钥成对、`ech-opts.enable=false` 清空子字段、会话参数独立活动、按用户确认的方案 A 校验 `idle-session-*`（只能是 0 或不小于 6 秒且 timeout ≥ check interval，依据固定 tag 的 5 秒静默替换阈值）、Reality 排除、AnyTLS 显式 Clash adapter（legacy 8→7）。同步修复 Step 15 暴露的 URI 基线缺口：`linkTargetDiagnostics` 新增 `anytls` 分支，mTLS 与三类伪装对象稳定 skip、证书校验与会话调优字段分别 warn，不再静默丢弃后仍标 complete。失败优先 8 项、node 定向 9 项、后端全量 41 包、前端 47 文件/335 用例、固定内核 AnyTLS 正例 3＋反例 3 与 `git diff --check` 全部通过。 |
| v1.19 | 2026-09-22 | 完成 Step 14 Tailscale：按用户确认的方案 B 为 `ConditionRule` 增加第八个维度 `non_empty`（兄弟字段非空依赖），`Matches` 全链增加根参数并同步后端 10 处调用点与前端 `matchesCondition`；endpoint 固定 hidden／hidden 并规范化为 `''/0`，wire 永不输出 server／port；`hostname/auth-key/control-url/ephemeral/udp/exit-node/exit-node-allow-lan-access` 为可编辑边界，`accept-routes`／`exit-node-allow-lan-access` 保留 unset／false／true 三态，LAN access 由 `non_empty=[exit-node]` 条件化并在清空 exit-node 时一并清空；`state-dir` 只由服务端注入的 `NodeID`／`Persisted` 派生（`tailscale/node-<id>`，非法生命周期阻断、新建草稿不输出）；空 auth-key 只返回交互登录 warn（不含 URL、零副作用），非 HTTPS 控制面只给安全 warn；Tailscale 显式 Clash adapter（legacy 9→8）。同步修复 `diagnoseNodeForTarget` 未传 NodeID／Persisted 的基线缺陷，使「正式装配缺失稳定 ID 必须阻断」可验证。前端三态 bool 渲染与 `non_empty` 条件、独立开关区适配。失败优先 10 项、node 定向 11 项、后端全量 41 包、前端 47 文件/334 用例、固定内核 Tailscale 正例 2＋反例 1 与 `git diff --check` 全部通过。 |
| v1.18 | 2026-09-22 | 完成 Step 13 MASQUE：`network_mode=quic/h2/h3_l4proxy` state_only selector、三种网络模式的 wire 映射（quic 省略以表达内核默认、h2／h3-l4proxy 写规范值）、`h3_l4proxy` 强制关闭 UDP 且切回不恢复、QUIC 分支才活动 `congestion-controller/cwnd/bbr-profile`、两类密钥按固定 tag 的 EC 结构校验（SEC1 私钥／PKIX ECDSA 公钥）、`ip/ipv6` 至少一项与缺省 `/32`／`/128`、`uri` 绝对 URL 校验且错误不回显原值、`mtu/handshake-timeout/cwnd` 非负整数、`ip-stack` 复用 WireGuard 枚举、MASQUE 显式 Clash adapter（legacy 10→9）、`name-cert-verify` 排除、SR／generic 稳定 skip。共享公共件通用化：`ipStack*`／`dnsListField`／`quicCongestionControllerField`／`normalizeLocalAddressPrefixes`／`validateLocalAddresses`／`validateDNSList` 由 WireGuard／TUIC 与 MASQUE 共用，未复制第二份枚举或规则。失败优先 11 项、node 定向 12 项、后端全量 41 包、前端 47 文件/332 用例、固定内核 MASQUE 正例 3＋反例 5 与 `git diff --check` 全部通过。 |
| v1.17 | 2026-09-22 | 完成 Step 12 Mieru：`endpoint_mode=single/range` state_only selector 与缺省派生、single／range 两条 endpoint policy（range 保留 host、隐藏并规范化顶层 port 为 0）、`port-range` 严格单段 begin-end 且两端 1-65535、`transport` 精确 `TCP/UDP` 不做大小写转换、`multiplexing`／`handshake-mode` 修复为完整上游常量且空值表示内核默认不强写、`traffic-pattern` 按固定 tag 语义（Base64→proto→四组校验）校验且错误不回显原值、`udp` 与 transport 独立保存、Mieru 显式 Clash adapter（legacy 11→10）与 port／port-range 严格二选一 wire、SR／generic 稳定 skip。按用户确认的方案 B 引入 `github.com/enfein/mieru/v3 v3.37.0`（GPL-3.0，MIT 项目的许可证影响已由用户事前确认）。失败优先 7 项、node 定向 9 项、后端全量 41 包、前端 47 文件/331 用例、固定内核 Mieru 正例 2＋反例 4 与 `git diff --check` 全部通过。 |
| v1.16 | 2026-09-22 | 完成 Step 11 标准 WireGuard：`peer_mode=single/peers` state_only selector 与缺省派生、single／peers 两条 endpoint policy（peers 隐藏并规范化顶层 endpoint 为 `''/0` 且不输出 server／port）、single↔peers 字段互斥与切换清空、Peer≥2 与逐项 `server/port/public-key/allowed-ips`、R27-07 `_credential_id` 稳定身份、private／public／PSK 的 Base64＋32 字节校验、`reserved` 改 `byte-sequence` 并统一三整数、ip／ipv6 至少一项与缺省 `/32`／`/128`、allowed-ips CIDR／去重／跨 Peer 同网段拒绝、`ip-stack{mode,congestion-controller}` 枚举、`remote-dns-resolve`→`dns` feature、`workers/mtu/persistent-keepalive/refresh-server-ip-interval` 非负整数、WireGuard 显式 Clash adapter（legacy 12→11）与 URI 多 Peer 稳定 skip＋高级项 warn。修复 Step 11 暴露的共享基线缺陷：更新／检查路径保留的敏感密文被当作明文重解析，导致 Hysteria `auth`、SSH `private-key`、TUIC `uuid` 无法在不重填凭据的情况下更新（新增 `isKeptCredentialCiphertext` 统一处理）。失败优先 14 项、node 定向 18 项、后端全量 41 包、前端 47 文件/330 用例、固定内核 WireGuard 正例 2＋反例 5 与 `git diff --check` 全部通过；AmneziaWG 活动字段为零。 |
| v1.15 | 2026-09-22 | 修复 Step 6～10 独立核验发现的 Step 8～10 缺口：Hysteria `hop-interval` 与接收窗口收紧为非负整数；Hysteria2 补可选速率字符串、正整数高级字段和非负整数 QUIC window 校验；TUIC 拥塞控制器改为固定 tag 五值枚举并保留空值的内核默认语义；把无法由固定 Mihomo 证明的“混合凭据拒绝”改为项目 selector／adapter wire 互斥合同。新增失败优先用例均先在 `1498513` 失败、修复后通过；后端全量 41 包、build/vet/gofmt/errgate、前端 47 文件/329 用例与 build、固定 Mihomo v1.19.31 门禁全部通过。Docker、API／浏览器 smoke、真实客户端与人工验收未执行。 |
