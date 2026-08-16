# Xray-Core-API.md — Xray-core API 研究与参考资料

> **文档定位：** 本文档是 Xray-core gRPC API 的研究结论与参考资料库，供 vpn-sub 项目 Xray 对接（高级模式）开发与排查时查阅。设计决策与方案见 [Design2.md](../../Design2.md) 第五章；SSPanel-UIM 参照研究见 [SSpanel-Subscribe.md](./SSpanel-Subscribe.md)；节点链接标准见 [Node-Link-Standards.md](./Node-Link-Standards.md)。
> **核验来源：** 本地仓库 `/Users/kylechen/Desktop/Repo/Xray-core`（**v26.7.28**，core/core.go:21-23；go.mod module github.com/xtls/xray-core）源码核验 + `/Users/kylechen/Desktop/Repo/Xray-API-documents`（docs/ 与 examples/）交叉验证 + 互联网公开资料（XTLS 官方讨论、Marzban、3x-ui、vless URI 社区规范）。
> **标注约定：** 【源码事实】= 直接引自代码行；【推断】= 由源码逻辑推出；【猜测】= 基于领域经验的推测。

---

## 一、API 服务机制（`app/commander`）

- 配置 `api` 块开启：`{tag, listen, services}`；services 可选 `reflectionservice / handlerservice / loggerservice / statsservice / observatoryservice / routingservice`
- `listen` 支持 TCP 地址（如 `127.0.0.1:10085`）或 unix socket（`/`、`@` 开头）
- **无认证、无 TLS**（`grpc.NewServer()` 裸启动）——安全边界由部署者控制（本项目方案：IP 白名单）
- **并发限制**：官方文档提示约第 10 个并发线程后内核丢弃多余请求——客户端必须串行化/限并发
- 可内嵌为 Go 库（`core.New(config)`），但要求 Go 1.26 + 数十个重型依赖（quic-go/utls/sing 等）——本项目**不采用**内嵌方式，选择独立实例 + gRPC 远程管理（路径 A）；仅引入 `github.com/xtls/xray-core` 模块的 command 包 proto 生成代码

## 二、可用 API 能力

| 服务 | 方法 | 用途 |
|------|------|------|
| HandlerService | `AddInbound` / `RemoveInbound` / `AlterInbound` / `ListInbounds` | 入站管理 |
| HandlerService | `AlterInbound` + `AddUserOperation` | **向入站添加用户**（vless/vmess=UUID、trojan=密码、SS=密码+加密方式；均需 `Level` + `Email`——本项目四协议均使用，见 Design2 决策 #20） |
| HandlerService | `AlterInbound` + `RemoveUserOperation` | **按 email 移除用户** |
| HandlerService | `GetInboundUsers` / `GetInboundUsersCount` | 查询入站用户（email 空=全部） |
| HandlerService | `AddOutbound` / `RemoveOutbound` / `AlterOutbound` / `ListOutbounds` | 出站管理 |
| StatsService | `QueryStats{pattern, reset}` / `GetStats{name}` | 流量查询（counter 名如 `user>>>{email}>>>traffic>>>uplink`），支持查询后重置 |
| StatsService | `GetUsersStats{include_traffic, reset}` | **批量**获取在线用户 + 上下行流量（**仅覆盖在线用户**，见第七节） |
| StatsService | `GetAllOnlineUsers` / `GetStatsOnlineIpList` / `GetStatsOnline` | 在线用户 / 在线 IP / 在线人数 |
| StatsService | `GetSysStats` | 进程运行时（goroutine/内存/uptime） |
| LoggerService | `restartLogger` | 重启日志 |
| RoutingService | `TestRoute` / `SubscribeRoutingStats` | 路由测试 / 路由统计流 |
| ObservatoryService | `GetOutboundStatus` | 节点观测（延迟/可用性，需另配 observatory） |

## 三、硬约束与边界（对接必须知晓）

1. **统计需显式开启**：Xray 配置 `policy` 块 `statsUserUplink/Downlink/Online`（用户级）、`systemPolicy` 块 `statsInbound*/statsOutbound*`（入出站级），否则查询恒为空
2. **Counter 易失**：流量 counter 为内存态，**重启清零**；`reset=true` 归零——必须短周期采集差值落库（重启导致的差值丢失为业界通病）
3. **官方版本无带宽限速 / 无流量配额 / 无到期时间**：`policy/config.proto` 仅 timeout/stats/buffer；协议 Account（vless/vmess/trojan）无速率与额度字段——配额必须面板侧实现
4. **幂等性**（`proxy/vless/validator.go` 核验）：
   - `AddUser`：同 email 重复添加**报错** `"User xxx already exists."`；同 UUID 不同 email **静默覆盖**（危险，email 为键）
   - `RemoveUser`：email 不存在**报错** `"User xxx not found."`
   - email 匹配**大小写不敏感**（内部 ToLower）——email 规则用全小写
   - → 面板侧同步必须 DB 维护状态 + 容忍特定错误（已存在/不存在视为幂等成功）

## 四、协议 Account 结构

| 协议 | Account 结构 | 用户凭据 |
|------|-------------|---------|
| vless | `vless.Account{id=UUID, flow}` | UUID |
| vmess | `vmess.Account{id=UUID}` | UUID |
| trojan | `trojan.Account{password}` | 密码（本项目使用：每用户统一代理密码 users.proxy_secret_encrypted，见 Design2 §5.5） |
| shadowsocks | 密码 + 加密方式 | 密码 + 加密方式（本项目使用：每用户统一代理密码 + 节点 cipher，见 Design2 决策 #20） |

## 五、传输字段全集（节点行渲染所需，`transport/internet/*/config.proto`）

- **StreamConfig**：`protocol_name`（network：tcp/ws/grpc/httpupgrade/kcp/splithttp 等）+ `security_type`（none/tls/reality）+ `socket_settings`
- **TLS**：`server_name`（sni）、`fingerprint`（uTLS 指纹）
- **Reality**：客户端字段 `server_name` / `public_key` / `short_id` / `Fingerprint` / `spider_x`（vless:// 链接的 sni/pbk/sid/fp 参数来源）
- **ws**：`path`、`header`（Host）；**grpc**：`service_name`；**httpupgrade**：`host`、`path`
- **inbound JSON 结构**（`infra/conf/xray.go InboundDetourConfig`）：`protocol/port/listen/settings/tag/streamSettings/sniffing`——面板侧节点表字段设计可完整覆盖

## 六、GetUsersStats 覆盖范围核验（`app/stats/command/command.go`）

- 批量接口从 `VisitOnlineMaps` 构建用户集合、`om.Count()==0` 跳过——**仅返回在线用户**
- 即使 `include_traffic=true` 也只遍历在线用户 counter，**离线用户流量无法经此获取**
- → 全量流量采集必须逐用户 `QueryStats` 串行执行（本项目采集方案依据）

## 七、订阅链接规范

**vless:// 链接规范**（[XTLS/Xray-core#716](https://github.com/XTLS/Xray-core/issues/716) 及社区实践）：

```
vless://{uuid}@{server}:{port}?encryption=none&flow=xtls-rprx-vision&security=reality&sni={域名}&fp=chrome&pbk={公钥}&sid={shortid}&type=tcp#{节点名}
```

- `security` 取值：none / tls / reality
- `type` 取值：tcp / ws / grpc / httpupgrade
- `flow` 仅 `xtls-rprx-vision`（Reality 唯一支持）

**vmess:// 链接规范**：base64(JSON)，字段 `ps/add/port/id/aid/net/type/host/path/tls`

## 八、订阅响应头标准（XTLS 官方）

[XTLS/Xray-core Discussion #4877](https://github.com/XTLS/Xray-core/discussions/4877)（2025-07）确立的事实标准，v2rayNG / Clash Meta / SingBox / Exclave 等客户端均支持：

| 响应头 | 说明 |
|--------|------|
| `subscription-userinfo` | `upload=; download=; total=; expire=`（字段均可选，字节数） |
| `profile-web-page-url` | 订阅页地址 |
| `announce` | 公告 |
| `support-url` | 支持链接 |
| `profile-update-interval` | 客户端自动更新间隔（小时） |

## 九、生态面板实践对照

| 面板 | 机制 | 对本项目的验证 |
|------|------|---------------|
| **Marzban**（Python+React） | `app/jobs/record_usages.py` 定时任务从 Xray API 拉取用户流量写入数据库（`JOB_RECORD_USER_USAGES_INTERVAL` 可配，默认 15 分钟量级） | 验证「定时 QueryStats 拉取 + 落库」方案；本项目采集间隔默认 10 分钟 |
| **3x-ui / X-Panel**（内嵌 Xray） | 客户端字段模型 `id/email/limitIp/totalGB/expiryTime` + `delDepletedClients`（批量删除流量耗尽客户端）+ `resetClientTraffic`（重置流量） | 验证「面板侧记录配额、超限移除客户端、手动重置」方案；限速/IP 限制由面板生态自行实现（非 Xray API） |

## 十、跟踪项：Reality `LimitFallback` 限速字段

- `transport/internet/reality/config.proto` 含 `limit_fallback_upload/download{after_bytes, bytes_per_sec, burst_bytes_per_sec}`
- JSON 配置层可解析，但**运行时实现未找到**（`reality/reality.go` 无 `AfterBytes` 引用）
- **结论【推断】：为限速特性预留字段，当前版本未生效，勿依赖；后续 Xray 版本可能激活，保持跟踪**

## 十一、v26.7.28 深度取证补充（对接实现必读）

### 11.1 HandlerService 调用形态与错误匹配

- 增删用户**不是独立 RPC**，统一走 `AlterInbound{tag, operation(TypedMessage)}`：服务端按 `request.Tag` 经 `inbound.Manager.GetHandler` 定位 inbound 后应用操作（app/proxyman/command/command.go:85-101）；官方示例形态见 Xray-API-documents/examples/alterInbound.go:15-35（添加）、:82-90（删除）
- proto 定义：AddUserOperation{user=1}、RemoveUserOperation{email=1}、AlterInboundRequest{tag=1, operation=2}、AlterInboundResponse 为空消息（command.proto:13-38）；User{level, email, account(TypedMessage)}（common/protocol/user.proto:12-18）
- **错误匹配表**（幂等处理依据）【源码事实】：错误码均为 `codes.Unknown`（HandlerService 未用 status.Error，**只能靠字符串匹配**【推断】）

| 场景 | 错误字符串 | 位置 |
|------|-----------|------|
| tag 不存在 | `failed to get handler: <tag>` | command.go:97 |
| operation 类型未知 | `unknown operation` | command.go:88 |
| 重复 AddUser（vmess/vless 同文案） | `User <email> already exists.` | proxy/vmess/inbound/inbound.go:172、proxy/vless/validator.go:39 |
| 删除不存在用户（同文案） | `User <email> not found.` | inbound.go:182、validator.go:54 |
| email 为空删除 | `Email must not be empty.` | inbound.go:179、validator.go:49 |

- 幂等建议：对 `already exists.` / `not found.` 做子串匹配视为成功；vless 侧 email 先 `strings.ToLower`（validator.go:37,51），面板侧 email 必须规范化全小写（与 Design2 5.5 `user-{id}@vpn.local` 口径一致）
- 对账能力：`GetInboundUsers`（email 空=全量，command.go:125-147）+ `GetInboundUsersCount` 可做面板与节点 diff【推断：可作同步失败后修复手段】

### 11.2 StatsService 行为细节（采集实现关键）

- **QueryStats 的 pattern 是子串包含**（`strings.Contains(name, pattern)`，app/stats/command/command.go:168），非 glob/正则；**空 pattern 返回全部 counters**（含 inbound/outbound 维度），reset=true 会全部清零——面板务必传完整前缀如 `user>>>`
- **GetStats vs QueryStats 空态差异**：GetStats 对不存在的 counter 报 `codes.NotFound`（`"<name> not found."`，:33）；QueryStats 返回空列表无错误（:164-184）；「无数据」≠「零流量」（新用户/重启后 counter 尚未注册）
- **reset 为原子 swap**（`c.Set(0)` 返回旧值，counter.go / command.go:37,171）——reset=true 不会丢并发流量【源码事实】
- **counter 惰性注册且永不注销**：由 dispatcher 在首条流量到达时创建（`GetOrRegisterCounter`，app/stats/stats.go:52-63；注册点 app/dispatcher/default.go:164,173,202,208）；`UnregisterCounter` 全仓无调用方——**删除用户后历史 counter 残留，QueryStats 仍可查到旧值，面板解析时按 email 过滤归一**【源码事实】
- **counter 注册前提**：用户 email 非空 且 `policy.ForLevel(user.Level).Stats.UserUplink/UserDownlink=true`（default.go:161-172）；默认 policy 的 Stats 全为 false【推断：字段零值语义】——Xray 配置必须显式开启，否则永远查不到用户流量
- 在线维度另有 counter 名 `user>>>{email}>>>online`（OnlineMap，非流量 counter，default.go:225-229）；inbound 维度 `inbound>>>{tag}>>>traffic>>>uplink/downlink`（app/proxyman/inbound/always.go:28,36）
- **服务名兼容**：同一服务同时注册 `xray.app.stats.command.StatsService` 与旧名 `v2ray.core.app.stats.command.StatsService`（command.go:219-221）；HandlerService 同理（app/proxyman/command/command.go:224-226）——客户端用新名即可

### 11.3 Account 结构版本变化（v26.7.28）

- **vmess `alter_id` 字段已移除**（proxy/vmess/account.proto:11-19 仅剩 id/security_settings/tests_enabled）——旧教程/旧面板代码设置 alterId 在新核心上无效【源码事实】
- vless Account 新增实验字段 xorMode/seconds/padding/reverse/testpre/testseed（proxy/vless/account.proto:16-30），官方文档仓库未记载；面板只需传 id/flow/encryption【源码事实】
- `AsAccount()` 仅校验 UUID 可解析（`failed to parse ID`），不校验 flow/encryption 取值（proxy/vless/account.go:12-28）——非法 flow 会静默接受【源码事实，运行时行为待验证】【猜测：可能降级为无 flow】

### 11.4 commander 监听与并发

- 监听配置 config.proto:12-21：`tag/listen/services`；listen 空时走 outbound-tag 模式（commander.go:101-115），非空支持 TCP 与 unix socket（`/` 或 `@` 前缀，:81-82）
- **无并发/连接限制、无鉴权、无 TLS**：`grpc.NewServer()` 不带任何 ServerOption（commander.go:66）——第一节「约 10 并发后丢弃」来自官方文档提示，v26.7.28 源码未见显式限制实现【推断：限制可能在 gRPC 默认参数或 HTTP/2 层，保守起见仍串行化】
- 官方示例用 `grpc.Dial + WithInsecure`（Xray-API-documents/examples/init.go:16，两者在新 grpc-go 已弃用）——佐证官方也不启用 TLS；客户端侧只需 Uuid/Level/Email/InTag 四字段（examples/structures.go:17-30）

### 11.5 文档仓库滞后清单（Xray-API-documents vs 源码）

- docs/StatsService.md:5-8 只列 QueryStats/GetSysStats，实际还有 GetStats/GetStatsOnline/GetStatsOnlineIpList/GetAllOnlineUsers/GetUsersStats（command.proto:85-93）
- docs/HandlerService.md:4-13 漏列 ListInbounds/GetInboundUsersCount
- vless 实验字段与 vmess alter_id 移除均未记载
- → 【推断】对接实现以源码为准，文档仅作参考

---

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-16 | 同步修订 §二/§四：trojan / shadowsocks 由「本项目不使用」改为四协议使用口径（每用户统一代理密码 users.proxy_secret_encrypted + 节点 cipher），对齐 Design2 决策 #20（xray 来源支持 vless/vmess/trojan/shadowsocks） |
| 2026-08-15 | 新增第十一节 v26.7.28 深度取证：HandlerService 错误匹配表（codes.Unknown+字符串匹配）、QueryStats 子串匹配与空 pattern 风险、counter 惰性注册不注销、vmess alter_id 移除、文档仓库滞后清单；核验版本升级为 v26.7.28（core/core.go:21-23）；链接指向更新为 Design2.md 与新增 Reference 文档 |
| 2026-08-13 | 从 DesignOnHold.md（v1.2~v1.4）提取整理：Xray-core 源码核验结论（3.2/4.3）、互联网生态研究（4.5）、Reality LimitFallback 跟踪项（4.7 #1） |
