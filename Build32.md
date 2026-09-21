# Build32.md — 19 个 manual 协议编辑体验完整化

> **文档定位：** 本文档是下一轮活动构建方案，承接 [Design4.md](Design4.md) 第九章“完成当前全部 19 个已兼容 manual 协议的编辑体验改进”目标，以及用户于 2026-09-21 对研究结论的最新确认。
> **当前状态：** 研究、范围决策与实施方案定稿完成；**尚未获得代码实施授权，所有 Step 均未开始**。本文档建立或继续完善均不等于授权执行 Step 0.5 或修改业务代码。
> **编码约束：** [AGENTS.md](AGENTS.md) 是唯一强要求文档。实施时必须一次只执行一个 Step，逐步验收，不并行实施多个协议。
> **最新用户决策：** 新兼容基线为 Mihomo **v1.19.31**，不再把 v1.19.29 作为新设计兼容目标；允许协议级 endpoint policy；OpenVPN 使用“结构化编辑为主＋粘贴 `.ovpn` 解析导入”，不再把 `client-config` 直接作为 Mihomo 输出字段；WireGuard 只覆盖标准单 Peer／多 Peer，AmneziaWG 不纳入 Build32，留作后续独立专项。以上范围确认不等于代码实施授权。

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|---|---|---|
| 0.5 | 实施授权、状态复核与范围冻结 | ☐ 未开始 |
| 1 | 固定 Mihomo v1.19.31 证据门禁并清除活动代码中的 v1.19.29 基线 | ☐ 未开始 |
| 2 | `CurrentState` v2：通用 selector、条件、清空域与旧状态读取 | ☐ 未开始 |
| 3 | 协议级 endpoint policy 与统一 Clash wire adapter 骨架 | ☐ 未开始 |
| 4 | HTTP 完整条件表单、TLS／认证与输出合同 | ☐ 未开始 |
| 5 | SOCKS5 完整条件表单、TLS／认证／UDP 与输出合同 | ☐ 未开始 |
| 6 | SSH 密码／私钥认证、Host Key 与多行凭据 | ☐ 未开始 |
| 7 | Snell 版本、UDP／reuse 与五类 obfs 分支 | ☐ 未开始 |
| 8 | Hysteria 认证、带宽、端口跳跃与 TLS 合同 | ☐ 未开始 |
| 9 | Hysteria2 端口替代、混淆、Realm 与 QUIC 合同 | ☐ 未开始 |
| 10 | TUIC v4／v5 认证互斥与 QUIC 合同 | ☐ 未开始 |
| 11 | 标准 WireGuard 单 Peer／多 Peer、稳定凭据与 reserved | ☐ 未开始 |
| 12 | Mieru 单端口／`port-range` 真互斥与枚举修复 | ☐ 未开始 |
| 13 | MASQUE 网络模式、L3 参数与 UDP 限制 | ☐ 未开始 |
| 14 | Tailscale 无 endpoint 模式、认证与状态目录安全边界 | ☐ 未开始 |
| 15 | AnyTLS TLS 身份与 ShadowTLS／Restls／JLS 互斥 | ☐ 未开始 |
| 16 | ShadowQUIC 认证、QUIC 版本、0-RTT 与流控 | ☐ 未开始 |
| 17 | TrustTunnel TLS／QUIC／连接复用互斥 | ☐ 未开始 |
| 18 | OpenVPN 结构化 schema、wire adapter 与敏感字段 | ☐ 未开始 |
| 19 | `.ovpn` 只读解析 API、导入预览与前端应用草稿 | ☐ 未开始 |
| 20 | 全 19 协议目标诊断、URI 支持／稳定 skip 与正式装配一致性 | ☐ 未开始 |
| 21 | 全协议前端交互、375px／桌面、草稿与错误定位回归 | ☐ 未开始 |
| 22 | 联合门禁、隔离浏览器 smoke、证据分层与文档归档 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。没有用户新的实施授权，不得把 Step 0.5 改为进行中。

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
Mihomo Meta v1.19.31 darwin arm64 with go1.22.12 Mon Sep 14 13:27:59 UTC 2026
```

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
| Hysteria | `auth_mode`、`bandwidth_mode` | `auth/auth_str`；`text/numeric` | 按已有字段唯一派生；均无值时阻断认证选择 |
| Hysteria2 | `endpoint_mode`、`obfs_mode` | `single/ports`；`none/salamander/gecko` | 有 ports 时 `ports`，否则 `single`；无 obfs 时 `none` |
| TUIC | `auth_mode` | `v4/v5` | token 为 `v4`，UUID/password 为 `v5`；均无值时按 tag 默认版本 |
| WireGuard | `peer_mode` | `single/peers` | peers 非空时 `peers`，否则 `single` |
| Mieru | `endpoint_mode` | `single/range` | 有 port-range 时 `range`，否则 `single` |
| MASQUE | `network_mode` | `quic/h2/h3_l4proxy` | 空值为 `quic` |
| AnyTLS | `security_mode` | `plain/shadow_tls/restls/jls` | 无附加安全对象时 `plain` |
| TrustTunnel | `reuse_mode` | `none/connections/streams` | 无复用参数时 `none` |
| OpenVPN | `auth_mode`、`tls_key_mode` | `userpass/cert`；`none/tls_auth/tls_crypt/tls_crypt_v2` | 按已有字段唯一派生；认证两组均空或同时存在时阻断 |

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
| Snell | `version=1..5`、`obfs_mode=none/http/tls/shadow-tls/restls/jls` | v1/2 禁 UDP；reuse 仅 v4/5；各 obfs 凭据条件必填 | `psk`、obfs password/private-key、JLS password | Clash 完整；SR/generic 稳定 skip |
| Hysteria | `auth_mode=auth/auth_str`、`bandwidth_mode=text/numeric` | 两种认证互斥；up/down 表达按模式成对；port 始终必填 | `auth`、`auth-str`、`private-key` | Clash 完整；既有 URI 仅可无损子集 |
| Hysteria2 | `endpoint_mode=single/ports`、`obfs_mode=none/salamander/gecko`、Realm feature | password 必填；启用 obfs 时密码必填；包大小仅 gecko；Realm 子字段条件化 | `password`、`obfs-password`、`private-key`、`realm-opts.token`、`realm-opts.private-key` | Clash 完整；既有 URI 对 Realm 等不可表达项诊断 |
| TUIC | `auth_mode=v4/v5` | v4 只允许 token；v5 只允许 UUID＋password；切换清空旧凭据 | `token`、`uuid`、`password`、`private-key` | Clash 完整；既有 URI 只按当前版本分支输出 |
| WireGuard | `peer_mode=single/peers` | private-key 和至少一个本地 IP 必填；多 Peer 每项 allowed-ips 必填且不可冲突；reserved 恰 3 字节 | `private-key`、`pre-shared-key`、`peers[].pre-shared-key` | Clash 完整；已有 URI 仅支持可表达子集；AmneziaWG 明确排除 |
| Mieru | `endpoint_mode=single/range` | port 与 port-range 严格二选一；transport 仅 TCP/UDP；枚举使用完整上游常量 | `password` | Clash 完整；SR/generic 稳定 skip |
| MASQUE | `network_mode=quic/h2/h3-l4proxy` | private/public key、本地 ip/ipv6 至少一个；h3-l4proxy 禁 UDP | `private-key` | Clash 完整；SR/generic 稳定 skip |
| Tailscale | 无普通 endpoint；auth feature | auth-key 可空但返回交互登录 warn；exit-node LAN 开关只有 exit-node 时活动 | `auth-key` | Clash 完整；SR/generic 稳定 skip；state-dir 系统派生 |
| AnyTLS | `security_mode=plain/shadow_tls/restls/jls` | 三种伪装互斥；TLS mTLS 成对；不开放 Reality | `password`、`private-key`、三种伪装密码 | Clash 完整；既有 URI 只输出可表达子集 |
| ShadowQUIC | QUIC feature 组合 | username/password；QUIC versions v1/v2；0-RTT 显示风险提示 | `password` | Clash 完整；SR/generic 稳定 skip |
| TrustTunnel | QUIC feature、`reuse_mode=none/connections/streams` | username/password 成对；max-connections/min-streams 与 max-streams 冲突 | `password`、`private-key` | Clash 完整；SR/generic 稳定 skip |
| OpenVPN | `auth_mode=userpass/cert`、`tls_key_mode=none/tls_auth/tls_crypt/tls_crypt_v2` | CA 必填；userpass 或 cert/key 二选一；三种 tls key 互斥；tls-auth 关联 key-direction | `password`、`key`、`tls-auth`、`tls-crypt`、`tls-crypt-v2` | Clash 完整；SR/generic 稳定 skip；`.ovpn` 仅解析为草稿 |

补充约束：

- Hysteria2 `hop-interval` 是字符串，可表达 `15-30`，不再使用 number。
- Mieru 枚举必须为 `MULTIPLEXING_OFF/MULTIPLEXING_LOW/MULTIPLEXING_MIDDLE/MULTIPLEXING_HIGH` 和 `HANDSHAKE_STANDARD/HANDSHAKE_NO_WAIT` 的完整常量名；当前 `LOW/MIDDLE/HIGH` 是错误 wire 值。
- SSH `host-key`／`host-key-algorithms` 改为结构化列表；私钥输入包含 PEM 标志才进入 wire，禁止把用户文本当服务器路径读取。
- AnyTLS 的 `client-metadata` 和 `disable-reuse` 属于 v1.19.31 tag 合同，纳入高级区。
- MASQUE `name-cert-verify` 在 tag 源码中只是 placeholder，不作为可编辑已支持字段。
- Tailscale `state-dir` 不提供任意路径输入；adapter 使用稳定节点 ID 派生的安全相对目录（例如 `tailscale/node-<id>`），避免重命名改变身份或多个节点共享默认目录。未保存的新建草稿只显示“保存后分配”，节点检查不得启动 tsnet 或制造临时状态目录。
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
| 14 | 13 | 同上，Tailscale 测试 | 无 endpoint、稳定 state-dir、空 auth-key 警告且检查零副作用通过 |
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
- **验收：** 工作区既有改动已识别且不会覆盖；没有未决范围冲突；影响清单与遗留数据结果已写回本文。复核完成后把 Step 0.5 标为验收通过，再单独进入 Step 1。

### Step 1：固定 Mihomo v1.19.31 证据门禁

- 把活动门禁变量改为 `MIHOMO_11931_BIN`，测试函数和错误文本同步版本；不保留同时接受两个版本的 fallback。
- 把活动代码、`TargetEvidence`、diagnostic evidence、前端 verified label 从 v1.19.29 升到 v1.19.31。
- 历史 Design／Build／Issue 中的 v1.19.29 字样不改。
- 首先复跑首批四协议与 SS 插件正反例，确认升级没有静默语义变化。
- 普通 Go 测试未设置外部二进制时仍可 skip；`.mihomo-test.sh` 未设置或版本错误必须失败。
- **验收命令：**
  ```bash
  MIHOMO_11931_BIN='/Applications/Clash Verge.app/Contents/MacOS/verge-mihomo' ./.mihomo-test.sh
  cd backend && go test ./internal/node ./internal/assembly
  cd ../frontend && npm test -- --run editable-combobox node-form-layout nodes-view
  cd .. && git diff --check
  ```

### Step 2：`CurrentState` v2、selector 与清空域

- 实现第四章 selector 合同、递归 selector 源路径和 `state_only` 投影排除。
- `Matches`、`RequiredFor`、活动投影、校验、敏感合并、扩展清理和前端 `matchesCondition` 共用 selector 语义。
- `normalizeResetScopes` 只接受当前协议声明的 `selector.<name>`；未知 selector 返回 400。
- v1 状态读取派生但不回写；保存改写为 v2。
- 覆盖 A→B→A、失败保存回滚、检查不落库、凭据 keep/clear、扩展作用域清理。
- **验收命令：**
  ```bash
  cd backend && go test ./internal/node -run 'Test.*(Selector|CurrentState|Reset|Credential|Extension)'
  cd ../frontend && npm test -- --run node-form-layout node-features protocol-field-editor nodes-view
  cd .. && git diff --check
  ```

### Step 3：Endpoint policy 与 adapter 骨架

- 实现第四章 endpoint policy；创建／更新／检查统一调用协议感知校验。
- 前端按 policy 显示、必填、清空 host/port；列表不显示 `:0`。
- `clashProxy()` 改为 adapter registry；先提供点名协议的临时 legacy adapter 保持尚未轮到的协议行为，并返回 `legacy_adapter_pending` evidence，不允许静默 default map 透传。每完成一个协议 Step 就删除该协议的 legacy 登记；Step 20 必须归零。
- 节点检查和正式装配必须调用同一函数；增加静态／单元门禁禁止第二套拼装。
- **验收：** 普通协议空 host/port 仍 400；Tailscale policy 夹具可空；Mieru range 不输出 port；WireGuard peers 不输出顶层 endpoint。
- **验收命令：**
  ```bash
  cd backend && go test ./internal/node ./internal/assembly ./internal/server -run 'Test.*(Endpoint|ClashAdapter|NodeCheck)'
  cd ../frontend && npm test -- --run nodes-view node-form-layout
  cd .. && git diff --check
  ```

### Step 4～7：基础代理组（严格按 Step 号串行）

每个 Step 均按“协议矩阵测试 → schema／组合校验 → wire adapter → check／正式装配 → UI 定向测试”的顺序完成。

#### Step 4：HTTP

- basic 认证成对；TLS 关闭清空 SNI、证书校验、certificate/private-key；mTLS 成对。
- Headers 保持开放 string map；未知复杂值拒绝。
- 正反例覆盖无认证、basic、TLS、mTLS、缺半对凭据。

#### Step 5：SOCKS5

- 与 HTTP 共用有语义的认证／TLS schema helper，不复制整段字段；保留 UDP 独立开关。
- URI 不能表达的 mTLS 字段返回 diagnostic，不丢字段后仍标 complete。

#### Step 6：SSH

- 私钥只接受 PEM 内容；密码和私钥分支切换清除相应凭据，私钥口令只随私钥活动。
- `host-key` 和 `host-key-algorithms` 改为列表；空 host-key 明确显示“接受任意 Host Key”的安全提示。
- 固定内核正例不得引用本机文件。

#### Step 7：Snell

- 版本 1～5；v5 按 Mihomo tag 的 v4 客户端兼容实现记录诊断，不能伪称独立 v5 wire。
- v1/2 禁 UDP；reuse 只在 v4/v5 显示；五类 obfs 的字段、凭据、TLS 选项按模式清空。
- `client-fingerprint` 只在相关伪装分支显示。

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
- `up/down` 文本模式和 `up-speed/down-speed` 数字模式二选一；合法单位与正数校验。
- `protocol` 只开放 tag 支持值；`obfs-protocol` 仅作为兼容输入别名归一化，不作为第二个编辑入口。
- TLS/ECH/mTLS、端口跳跃和窗口高级项补齐；port 仍必填。

#### Step 9：Hysteria2

- `ports` 模式真正替代 port；`hop-interval` 支持单值或单范围字符串。
- obfs none/salamander/gecko；启用即要求密码，包大小仅 gecko。
- Realm 作为 feature object，包含 token、realm-id、STUN 列表和自身 TLS 子树；关闭清空全部子凭据。
- 补 BBR profile、handshake timeout 和 quic-go 高级窗口，默认不写库。

#### Step 10：TUIC

- v4 token 与 v5 UUID/password 强互斥；A→B→A 不恢复凭据。
- UDP relay、拥塞、SNI/ECH/mTLS、UOT version 等枚举／范围按 tag 校验。
- 固定内核正反例必须分别覆盖 v4、v5、混合凭据拒绝和非法 UOT version。

- **验收：** 每协议执行 Step 4～7 的命令模板，并追加 `MIHOMO_11931_BIN=... ./.mihomo-test.sh` 中该协议正反例。

### Step 11～14：隧道与特殊 endpoint 组

#### Step 11：标准 WireGuard

- 单 Peer 与 peers 模式由 selector 控制；多 Peer 清空／忽略顶层 server、port、public-key、pre-shared-key、reserved、allowed-ips。
- 保留 R27-07 的稳定 `_credential_id`；条目移动、删除、替换只影响对应 PSK。
- reserved 统一为 3 字节；多 Peer `allowed-ips` 必填且不同 Peer 不允许相同网段。
- private-key、ip/ipv6、IP stack、DNS 和 refresh interval 按 tag 校验。
- AmneziaWG 已由第三章明确排除；本 Step 不得添加其字段、UI、输出 adapter 或测试矩阵。

#### Step 12：Mieru

- 单端口／范围模式严格二选一；范围格式 `1-65535` 且 begin≤end。
- 修复 multiplexing 与 handshake mode 完整枚举；traffic-pattern Base64／语义错误返回字段级错误。
- transport 只允许 `TCP`／`UDP`，不做全局大小写转换。

#### Step 13：MASQUE

- network 明确为默认 QUIC、`h2`、`h3-l4proxy`；L4 proxy 时 UDP 必须关闭且不得保存 true。
- private/public key、ip/ipv6、URI、SNI、IP stack、拥塞和 handshake timeout 按 tag 结构输出。
- 至少一个本地地址必填；密钥和地址错误需在项目校验阶段返回，不依赖内核晚失败。

#### Step 14：Tailscale

- UI 完全隐藏普通 server/port；wire 禁止输出。
- auth-key 可空；节点静态检查只返回“首次真实连接需要交互登录”的 warn，不启动 tsnet、不联网，也不伪造／回显登录 URL。真实 Mihomo 首次启动时才可能在其日志输出官方文档所述 URL。
- hostname、control-url、ephemeral、UDP、accept-routes、exit-node 和 LAN access 条件化。
- 已保存节点的 `state-dir` 由稳定节点 ID 派生；不得接受用户路径，不在 API 回显主机绝对路径。新建草稿检查不创建目录，正式保存后才具备稳定派生值。

- **验收：** endpoint policy、列表显示、正式装配和固定内核正反例必须一起通过；检查请求前后数据库快照相同。

### Step 15～17：TLS／QUIC 新协议组

#### Step 15：AnyTLS

- plain TLS 与 ShadowTLS／Restls／JLS 三种附加安全模式互斥；禁止 Reality。
- ECH、mTLS、client fingerprint、client-metadata、session 参数和 disable-reuse 完整建模。
- 分支切换清空各自密码／脚本，不清空 AnyTLS 主密码。

#### Step 16：ShadowQUIC

- username、password、SNI、ALPN、QUIC v1/v2、UOT、0-RTT、keepalive、拥塞／窗口完整建模。
- 0-RTT 开启时显示重放风险提示；提示不替代服务端验证。

#### Step 17：TrustTunnel

- username/password 成对；TLS/ECH/mTLS 字段完整。
- QUIC 开关控制拥塞参数；连接复用以 selector 表达互斥模式，`max-connections/min-streams` 与 `max-streams` 不得同时活动。
- health-check、UDP 和复用分支分别验证。

- **验收：** 每协议定向 node／assembly／frontend 测试和固定 v1.19.31 正反例通过，敏感值不进入响应、日志或 check preview。

### Step 18：OpenVPN 结构化模型与 adapter

- 移除 editor schema 中 `client-config`，新增第五章字段矩阵对应的结构化字段。
- `auth_mode` 与 `tls_key_mode` 为 state-only selector；CA、cert、key、tls key 使用明确 multiline 类型。
- OpenVPN wire adapter 只输出 v1.19.31 `OpenVPNOption` 字段；禁止输出原始 `.ovpn` 或未知指令。
- 旧 `client-config` 不自动解释、不透传。若 Step 0.5 在任何目标数据库发现遗留行，立即停止在 Step 18 前，由用户决定迁移／清除策略；不得靠删除 schema 字段使旧内容在下一次保存时静默丢失。只有确认无遗留行，或另行获得明确处置授权后，才能移除编辑入口。
- **验收：** user/pass、cert/key、三种 tls key、互斥反例、字段脱敏与固定内核正反例通过。

### Step 19：`.ovpn` 解析导入

- 实现第四章有界、只读、无文件系统读取的 parser 和管理员路由。
- 前端 OpenVPN 区提供“粘贴 `.ovpn`”面板：解析、显示 diagnostics、查看脱敏结构、显式应用／取消。
- parser 草稿与现有自定义／JSON 草稿纳入页面级阻断；切换协议或关闭面板清空未应用原文。
- API 日志只记录长度、结果计数和错误 code，不记录原文、remote 凭据或内嵌块。
- `no-store` 中间件必须注册在 session／admin 之前；匿名、普通用户、超限、解析失败和成功响应均测试响应头。
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

### Step 20：目标诊断与正式装配收口

- 19 协议 `clash-yaml` 均走显式 adapter；不支持 URI 的 8 项返回稳定 skip：Snell、Mieru、MASQUE、OpenVPN、SSH、ShadowQUIC、TrustTunnel、Tailscale。
- 对已有 URI 映射的协议只输出无损字段；无法表达的活动字段产生 `core_semantic_unexpressible` 或 `unverified_compatibility`。
- `target_evidence` 必须由检查／装配实际消费，不能只是 UI 标签。
- `CheckClashContent` 增加 15 协议关键 shape 和互斥门禁；内核接受但项目不允许的结构仍由项目拒绝。
- **验收：** 节点检查预览与正式装配单节点片段语义相同；skip 不带 preview，不报 500。

### Step 21：全协议前端回归

- 覆盖 selector、动态 endpoint、当前组合、独立开关、高级折叠、多行 secret、列表、错误定位和草稿阻断。
- 375px 与桌面 920px；明暗主题；长证书／私钥；WireGuard Peer 排序；OpenVPN 导入 diagnostics。
- 切换协议提示改为按 endpoint policy 说明，不再统一声称保留 server/port。
- 前端不得维护第二份协议字段或枚举全集；所有协议差异来自后端 schema／policy。
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
- OpenVPN：大小限制、多 remote、inline block、外部路径拒绝、未知指令、脱敏和不落库；
- WireGuard：条目稳定 ID、重排、删除、PSK keep/clear、reserved 两种输入；
- Tailscale：派生目录稳定、路径安全、空 auth-key warn；
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
- 本轮只修改本文档，没有修改业务代码、Design4、AGENTS.md、测试或数据库，也没有执行构建门禁。

---

## 十一、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-21 | 完成 19 个 manual 协议后续专项研究并建立 Build32；冻结 Mihomo v1.19.31、selector/state v2、endpoint policy、15 协议矩阵、显式 wire adapter、OpenVPN 结构化＋`.ovpn` 解析、串行 Step 与验收门禁。所有代码 Step 未获授权、未开始；AmneziaWG 保留为唯一待确认候选。 |
| v1.1 | 2026-09-21 | 按用户确认冻结方案 A：Build32 的 WireGuard 只覆盖标准单 Peer／多 Peer，AmneziaWG 明确排除并留作后续独立专项。本次确认不授权进入 Step 0.5，全部代码 Step 继续保持未开始。 |
| v1.2 | 2026-09-21 | 进一步实施定稿：补齐 selector 持久化与 v1 派生矩阵、endpoint 防伪造、OpenVPN API 安全响应合同、逐 Step 文件／前置／完成定义和共同门槛；增加 legacy adapter 归零与遗留 `client-config` 停止条件；更正 Tailscale 登录提示和稳定 `state-dir` 边界。仍未授权任何代码 Step。 |
