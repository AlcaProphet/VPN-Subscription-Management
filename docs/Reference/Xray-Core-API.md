# Xray-Core-API.md — Xray-core API 研究与参考资料

> **文档定位：** 本文档是 Xray-core gRPC API 的研究结论与参考资料库，供 vpn-sub 项目 Xray 对接（高级模式）开发与排查时查阅。设计决策与方案见 [DesignOnHold.md](../DesignOnHold.md) 第五章；SSPanel-UIM 参照研究见 [SSpanel.md](./SSpanel.md)。
> **核验来源：** 本地仓库 `/Users/kyle/Desktop/Repo/Xray-core`（go 1.26 版本）源码核验 + 互联网公开资料（XTLS 官方讨论、Marzban、3x-ui、vless URI 社区规范）。

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
| HandlerService | `AlterInbound` + `AddUserOperation` | **向入站添加用户**（vless/vmess=UUID；均需 `Level` + `Email`；trojan=密码、SS=密码+加密方式——本项目仅使用 vless/vmess） |
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
| trojan | `trojan.Account{password}` | 密码（本项目不使用） |
| shadowsocks | 密码 + 加密方式 | （本项目不使用） |

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
- **结论：推测为限速特性预留字段，当前版本未生效，勿依赖；后续 Xray 版本可能激活，保持跟踪**

---

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-13 | 从 DesignOnHold.md（v1.2~v1.4）提取整理：Xray-core 源码核验结论（3.2/4.3）、互联网生态研究（4.5）、Reality LimitFallback 跟踪项（4.7 #1） |
