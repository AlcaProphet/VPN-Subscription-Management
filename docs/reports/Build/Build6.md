# Build6.md — 高级模式核心：Xray 对接后端（当前构建方案·第六轮）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第六轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则），承接 [Build4.md](Build4.md)、[Build5.md](Build5.md)（基础模式全部验收通过后本轮方可启动）。
> - 设计基线：[Design2.md](../Design/Design2.md)（与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design2-UI.md](../Design/Design2-UI.md)（已归档；本 Build 主要实现其 §9.1 后端契约，前端落点在 Build7）
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 研究资料：[Xray-Core-API.md](../../Reference/Xray-Core-API.md)（**必读，尤其 §11**）、[Node-Link-Standards.md](../../Reference/Node-Link-Standards.md)
> - 前置轮次：[Build4.md](Build4.md)、[Build5.md](Build5.md)；后续轮次：[Build7.md](Build7.md)（高级模式管理面与交付收口）
>
> **里程碑：本 Build 全部 Step 完成后，高级模式后端闭环可用：系统可连接建议 1~5 台 Xray-core 实例（latest，当前 v1.260327.0）、检测入站生成 xray 节点、组节点分配与候选集约束生效、用户生命周期自动 AddUser/RemoveUser、批量初始化幂等、装配生成模板下载时按用户动态注入节点与凭据、Subscription-Userinfo 响应头正确、流量采集/配额/超限摘除/手动重置全部工作。**
>
> **范围红线：** 本 Build **只做后端与单元/集成测试，不写 XrayInstancesView / GroupsView 高级 UI / SettingsView 高级分区等前端页面**（Build7）；实例级对账、独立 Xray 账号、配置导入导出 v2、高级模式 OFF 清空确认及其 UI 也在 Build7；**独立账号采集与配额检查于 Build7 Step1 补入**（与 ext CRUD 同轮闭环，DesignReport10 确认，本 Build 采集任务仅覆盖面板用户）。

---

## 执行约束（执行 AI 必须严格遵守）

1. **严格按 Step 顺序执行**；完成一个 Step 并验收通过后进入下一个；**禁止跳步、并行、合并、跨 Build 提前实现后续功能**。
2. **每个 Step 完成后必须运行该 Step 的「验证命令」**；任一失败修复后重验，禁止带错进入下一 Step。
3. **遇到模糊、歧义或设计未覆盖的细节，必须停止并使用提问工具向用户询问**。Xray API 行为一律以 [Xray-Core-API.md](../../Reference/Xray-Core-API.md) §11 源码取证为准，禁止凭旧教程实现。
4. **依赖白名单**：本 Build 只新增 `github.com/xtls/xray-core`（以远端 latest 为准，不锁定具体版本号；当前 latest 为 `v1.260327.0`，及 `go mod tidy` 拉取的既有传递依赖）；gRPC 客户端库使用 xray-core 依赖图中的 `google.golang.org/grpc` 版本，禁止另选 gRPC 框架。
5. **关键设计参数必须严格按下表取值**，与 Design2.md 保持一致，禁止修改：

| 参数 | 取值 | 出处 |
|------|------|------|
| Xray 版本 | `github.com/xtls/xray-core`（远端 latest，不锁定版本号；当前 latest 为 `v1.260327.0`） | 用户决策：以 latest 为准 |
| Xray 协议范围 | 用户增删仅 vless/vmess/trojan/shadowsocks；其他 inbound 检测入库但 `allocatable=0` | Design2 §3.2/§5.2 |
| Xray email | 面板用户 `user-{id}@vpn.local`；**全小写**（vless 侧 ToLower） | Design2 §5.5 |
| 用户凭据 | 每用户一个 UUID v4 + 一个高熵代理密码；AES-256-GCM 存 users.uuid_encrypted / proxy_secret_encrypted；**首建同事务同生同灭** | Design2 §5.5 |
| 幂等错误 | 仅子串 `already exists.` / `not found.` 视为成功；其余错误保留 last_error | Xray-Core-API.md §11.1 |
| gRPC 调用 | **串行化**（每实例或全局互斥锁）；API 无认证无 TLS，安全由部署者 IP 白名单控制 | Design2 §5.2 |
| 推送集合 | 所属组分配节点 ∪ 公共节点；过滤 `enabled=1 AND allocatable=1 AND missing=0 AND xray_instances.enabled=1` 与候选集 | Design2 §5.5/§5.6 |
| 超限语义 | 超限仅 RemoveUser + quota_exceeded=1；下载内容不变；管理员重置恢复；超限用户所有 AddUser 类钩子前置跳过 | Design2 §5.4/§5.5 |
| 采集 | 逐用户 `QueryStats` 串行；pattern 必须完整前缀 `user>>>{email}>>>traffic`；`reset=true`；差值原子 UPSERT 落库；**禁止空 pattern** | Design2 §5.8，Xray-Core-API.md §11.2 |
| 配额 | groups.default_quota / users.quota_override 为 NULL 或 0 均不限流量；自然月 ym 按 UTC | Design2 §5.8 |
| 用量响应头 | `subscription-userinfo` 仅高级模式、仅用户订阅类下载携带；upload/download 取当月累计字节，total 取有效配额字节，expire=`4102444800`；`profile-update-interval=6`（小时）与 `profile-web-page-url` 由系统注入并覆盖平台同键 | Design2 §5.7 |
| 注入判定 | `users.uuid_encrypted` 非空即注入；未生成凭据占位替换为注释 `# 节点未开通，请联系管理员`；直接上传内容原样返回 | Design2 §5.7 |
| 高级开关 | 配置键 `advanced_mode`；**所有同步钩子入口与凭据/xray_users 写事务内实时查 DB**；AddUser 完成后复查，读到 off 立即补偿 RemoveUser | Design2 §一/§5.5 |
| 采集间隔 | 配置键默认 10 分钟，≥1；Build7 提供设置 UI | Design2 §5.8 |

6. **注释使用中文**；所有 error 必须处理；构造注入；业务层不感知 HTTP；接入层不直接操作存储；日志 token 脱敏；下载禁缓存。
7. **严禁**：在 gRPC 调用前缓存 advanced_mode；用 `GetUsersStats` 替代逐用户采集；用空 pattern QueryStats；对 vmess Account 设置已移除的 `alter_id` 字段；删除用户/实例前不收集连接信息就执行 RemoveUser（会查不到目标）。

---

## TODOLIST CheckList（构建进度核对）

> 执行 AI 必须逐个完成并勾选。状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

- ✅ Step 0：xray-core 依赖与 internal/xray gRPC 客户端封装
- ✅ Step 1：Xray 实例 CRUD、连通性测试与节点检测入库
- ✅ Step 2：advancedMode 中间件、组节点分配/候选集/公共节点/默认配额
- ✅ Step 3：用户凭据首建、生命周期同步、xray_users 状态机与批量初始化
- ✅ Step 4：下载动态渲染（蓝图全量重渲染/占位替换）与 Subscription-Userinfo
- ✅ Step 5：流量采集、配额检查、超限摘除与重置配额
- ✅ Step 6：假 Xray 服务集成测试与 Build6 端到端验收

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 0 | xray-core 依赖与客户端封装 | Design2 §5.2/§5.3，Xray-Core-API §11 | ✅ 验收通过 |
| 1 | 实例 CRUD/连通性/节点检测 | Design2 §5.9/§3.2，UI §8.1/§8.2 | ✅ 验收通过 |
| 2 | 高级中间件与组节点模型 | Design2 §一/§5.6/§5.10 | ✅ 验收通过 |
| 3 | 用户生命周期同步与初始化 | Design2 §5.5 | ✅ 验收通过 |
| 4 | 下载动态渲染与响应头 | Design2 §5.7 | ✅ 验收通过 |
| 5 | 流量采集与配额 | Design2 §5.8 | ✅ 验收通过 |
| 6 | 假 Xray 集成测试与验收 | Design2 第五章全量 | ✅ 验收通过 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件（核心） | 要点 |
|------|----------------|------|
| 0 | `backend/go.mod`、`backend/internal/xray/{client,account,errors}.go`、测试 | 依赖远端 latest（当前 v1.260327.0）；串行 gRPC；dial/调用超时；错误幂等映射；四协议 Account 构造 |
| 1 | `backend/internal/xray/{instance,detect}.go`、`backend/internal/node/node.go`（删除清理钩子）、`backend/internal/tasks/registry.go`（中性任务包）、`backend/internal/server/{xray,tasks,server}.go`、测试 | 实例 CRUD/slug/连接测试；ListInbounds 解析与 nodes upsert、missing/allocatable 标记；全局任务 registry 落地 |
| 2 | `backend/internal/group/group.go`、`backend/internal/platform/platform.go`（删除后触发回调）、`backend/internal/subscription/subscription.go`（删除后触发回调）、`backend/internal/server/{group,subscription}.go`（版本切换触发回调）、`backend/internal/server/middleware.go`、`backend/internal/assembly/service.go`（xray_candidates 填充修补）、测试 | advancedMode 中间件；group_nodes/候选集并集/公共节点/default_quota |
| 3 | `backend/internal/xray/{credentials,sync}.go`、`backend/internal/user/admin.go`、`backend/internal/user/user.go`、`backend/internal/approval/approval.go`、`backend/internal/group/group.go`、`backend/internal/node/node.go`、`backend/internal/server/xray.go`、测试 | 凭据首建事务；触发器 wiring；xray_users 状态机；补偿 RemoveUser；批量初始化 |
| 4 | `backend/internal/download/download.go`、`backend/internal/download/render.go`（定稿）、`backend/internal/assembly/links/` 共享子包（自 Build5 links.go 抽取，用户决策）、测试/benchmark | 直接上传原样；蓝图全量重渲染；占位替换/base64；Subscription-Userinfo |
| 5 | `backend/internal/xray/{stats,quota}.go`、`backend/internal/cron/`、`backend/cmd/server/main.go`、`backend/internal/server/{xray,home,status}.go`、测试 | 逐用户 QueryStats、原子增量、超限摘除、重置配额、采集状态、home 高级流量与流量卡片开关 |
| 6 | `backend/internal/xray/fake_test.go` 或 `backend/tests/`、集成测试 | 假 HandlerService/StatsService；全链路验证 |

---

## 三、构建顺序依赖图

```
Step 0（客户端）──▶ Step 1（实例/检测）──▶ Step 2（组分配/高级中间件）──▶ Step 3（同步/凭据）
Step 3 ──▶ Step 4（下载渲染依赖凭据/蓝图/候选集）
Step 3 ──▶ Step 5（采集依赖实例/客户端/状态机）
Step 4+5 ──▶ Step 6（集成验收）
```

> 线性执行序：Step 0 → 1 → 2 → 3 → 4 → 5 → 6。

---

## 四、分步构建计划


---

### Step 0：xray-core 依赖与 internal/xray gRPC 客户端封装

**本 Step 完成后，项目依赖远端 latest xray-core（当前为 v1.260327.0，不锁定版本号）且可编译；`internal/xray` 提供串行 gRPC 客户端与四协议 Account 构造，幂等错误映射通过单测。**

- **目标：** 引入 Xray 模块并封装 HandlerService/StatsService 调用。
- **前置条件：** Build5 全部验收通过；Go 1.26（Build4 Step 0）。
- **产出文件与操作：**

  1. **依赖引入**：`cd backend && go get github.com/xtls/xray-core@latest && go mod tidy`。若网络受限，用内网 GOPROXY 或与现有 go.sum 缓存一致镜像；**不得手写 require 不存在的版本**。完成后 `go build ./...` 必须通过。
  2. **`backend/internal/xray/client.go`**：
     - `type Client struct { conn *grpc.ClientConn; handler handlercmd.HandlerServiceClient; stats statscmd.StatsServiceClient; mu sync.Mutex }`。
     - `Dial(apiAddr string) (*Client, error)`：校验 TCP 地址；`grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))`；**`grpc.NewClient` 为懒建连（dial 不阻塞），10s「拨号超时」施加于首个 RPC：TestConnection 与检测探测显式携带 10s deadline 的 ctx，不可达实例在该 deadline 内快速失败（注意：`grpc.NewClient` 官方文档明示忽略 `WithBlock`/`WithTimeout`/`WithReturnConnectionError`/`FailOnNonTempDialError` 等阻塞类选项，快速失败由 ctx deadline 达成，勿引入这些选项）；其余 RPC 调用携带单次 30s deadline（`context.WithTimeout`），避免异步长任务挂起无终态（Design2 §5.4）**；包导入别名 `handlercmd "github.com/xtls/xray-core/app/proxyman/command"`、`statscmd "github.com/xtls/xray-core/app/stats/command"`。
     - 所有方法持 `c.mu.Lock()` 串行执行（每实例串行、多实例间可并行，符合 Xray-Core-API §11.4 结论）。
     - `AddUser(ctx, tag string, u *protocol.User) error`：`serial.ToTypedMessage(&handlercmd.AddUserOperation{User: u})` → `AlterInbound`；错误 `already exists.` 子串视为 nil。
     - `RemoveUser(ctx, tag, email string) error`：`serial.ToTypedMessage(&handlercmd.RemoveUserOperation{Email: email})`；错误 `not found.` 子串视为 nil。
     - `ListInbounds(ctx)`、`GetInboundUsers(ctx, tag, email string)`、`QueryStats(ctx, pattern string, reset bool) (*statscmd.QueryStatsResponse, error)` 直接透传并包装错误（错误信息必须带 api_addr/tag 上下文）。
     - 任何错误不得用 `codes` 判定（全部 Unknown，只做字符串子串匹配）。
  3. **`backend/internal/xray/account.go`**：
     - `BuildUser(userID int64, uuid, proxySecret string, node NodeView) *protocol.User`；email 固定 `user-{id}@vpn.local` 全小写；`Level: 0`。**`NodeView` 在本文件定义**（Account 构造所需的节点视图，字段含 `Protocol/Cipher/Flow/Tag/Host/Port` 等；Step 1 检测归一化产物与 Step 3 推送目标复用同一类型，避免再引 node 包形成依赖环）。
     - vless：`&vless.Account{Id: uuid, Flow: nodeFlow, Encryption: "none"}`（flow 为空则空串；vision 节点必须与节点 flow 一致）。
     - vmess：`&vmess.Account{Id: uuid}`（**不要设置 alter_id**）。
     - trojan：`&trojan.Account{Password: proxySecret}`。
     - shadowsocks：`&shadowsocks.Account{Password: proxySecret, CipherType: cipherTypeOf(node.Cipher)}`；cipher 映射函数按 shadowsocks 枚举常量实现（chacha20-ietf-poly1305 / aes-256-gcm / aes-128-gcm 等常用值必须覆盖；未知 cipher 返回错误）。若编译时枚举名与预期不符，以 xray-core 包内生成代码为准调整并补注释，不得保留编译错误。
     - `AccountFromNode(protocol string, ...) (*serial.TypedMessage, error)` 返回 `serial.ToTypedMessage(account)`。
  4. **`backend/internal/xray/errors.go`**：`IsAlreadyExists(err)` / `IsNotFound(err)`；统一错误类型 `OpError{Op, Instance, Tag, Err}`。
  5. **单测**：不连接真实 Xray 即可测——幂等错误映射、email 全小写、四协议 Account 的 TypedMessage type URL、cipher 映射、地址校验、**dial/调用超时配置存在性断言**。**本 Step 不写 HTTP 端点。**

- **参考代码/伪代码：**

  **AddUser 错误处理核心**

  ```go
  func (c *Client) AddUser(ctx context.Context, tag string, u *protocol.User) error {
      c.mu.Lock(); defer c.mu.Unlock()
      op, err := serial.ToTypedMessage(&handlercmd.AddUserOperation{User: u})
      if err != nil { return fmt.Errorf("序列化 AddUser 操作失败: %w", err) }
      _, err = c.handler.AlterInbound(ctx, &handlercmd.AlterInboundRequest{Tag: tag, Operation: op})
      if err == nil || strings.Contains(err.Error(), "already exists.") {
          return nil // 幂等：同 email 已存在视为成功
      }
      return fmt.Errorf("xray AddUser 失败: %w", err)
  }
  ```

  **email 构造**

  ```go
  func UserEmail(id int64) string { return fmt.Sprintf("user-%d@vpn.local", id) }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go get github.com/xtls/xray-core@latest && go mod tidy
  cd backend && go build ./... && go vet ./... && go test ./internal/xray/... ./...
  ```

- **验收标准：** go.mod 使用远端 latest xray-core（当前为 v1.260327.0，不锁定版本号）；编译/静态检查/测试通过；`go list -m all | grep xtls/xray-core` 显示当前 latest 版本；客户端所有 gRPC 方法均有串行锁与错误上下文。



---

### Step 1：Xray 实例 CRUD、连通性测试与节点检测入库

**本 Step 完成后，`/api/admin/xray/instances` CRUD、`/api/admin/xray/instances/test`、`/api/admin/xray/instances/:id/detect` 可用；检测结果以 instance_id+tag 为键 upsert 到 nodes，撞名/非法名跳过不崩溃；全局长任务 registry 与 `GET /api/admin/tasks/:id` 在本 Step 落地。**

- **目标：** 实现实例数据模型与 ListInbounds 解析入库。
- **前置条件：** Step 0 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/xray/instance.go`**：实例服务。
     - 结构 `Instance {ID, Name, Slug, APIAddr, APITag, Enabled, LastCollectAt, CollectStatus, CollectError}`。
     - `Create/Update`：name 唯一（409）、api_addr 非空 TCP 地址、api_tag 可空；slug 自动生成 `instance-` 前缀（用 `internal/slug`，唯一冲突重试；**注意现有 `TableHasSlug` 白名单不含 xray_instances，本 Step 必须把 `xray_instances` 加入该白名单，并同步加入共享 slug 命名空间的交叉查重表清单（`subscription.slugExistsTx` 与 `slug.ExistsInFourTables` 的四表清单），否则 instance- slug 不会与 subscriptions/rules/custom_subscriptions/share_subscriptions 互斥**）。**实例服务按实例缓存同一个 `internal/xray.Client`（含其互斥锁），检测/推送/采集/对账全部复用该 Client，确保每实例串行**（DesignReport9 Q13）。**api_tag 作为展示/日志/导出的实例标签原样保存；gRPC 调用定位用 api_addr，入站定位用 nodes.tag**（Xray-Core-API §一/§11.1）；**enabled 变化时事务提交后仅刷新内存可见状态，不触发候选集重算与 AddUser/RemoveUser diff（暂停管理口径：停用仅暂停检测/推送/采集/注入，账号与 group_nodes 保留，重启用后由同步与实例级对账兜底，Design2 §5.9/H1）**。
     - `List/Get`；`Delete`：**端点异步化——提交即返回 `{task_id}`（全局任务 registry，kind=instance_delete，Design2 §5.4）**，任务体内：事务前收集 `xray_users` 与 `xray_ext_users` 的（email, instance_id, inbound_tag, api_addr）清单；事务内删实例行（nodes / xray_users / xray_ext_users 随 FK 级联）；提交后逐条 best-effort `RemoveUser`（面板用户与独立账号两类），不可达跳过记 warn。**Step 3 接入 sync 后**，把收集口径升级为「受影响 active 用户 × 该实例节点」∪「既有 xray_ext_users 推送目标 × 该实例」期望集，而不是只依赖既有 xray_users 状态行；本 Step 先预留回调并在 Step 3 补接。
     - `TestConnection(ctx, apiAddr)`：拨号 + `ListInbounds`，返回 ok/error 摘要；不落库。
  2. **`backend/internal/xray/detect.go`**：`DetectNodes(ctx, instanceID)`。
     - 实例 enabled=0 拒绝（400「实例已停用，不参与节点检测」）。
     - `ListInbounds` 后逐个解析：
       - `protocol`：由 `ProxySettings.TypeUrl` 的最后一段映射（如 `xray.proxy.vless.inbound.Config` → `vless`；无法识别也保留协议名）。
       - `tag` 取 inbound.Tag；`port` 取 inbound.Port；`host` **定稿为实例派生字段、非 inbound 解析字段**（DesignReport10 决策）：取实例 api_addr 的 host 部分（若 api_addr 为 `host:port`），与「解析不到的字段不虚构默认值」原则区分——host 来源为实例连接地址而非入站配置。
       - `protocol_json`：从 ProxySettings 与 StreamSettings 归一化为渲染所需字段（network / security / tls server_name / fingerprint / reality pbk-sid / ws path+host / grpc service_name / httpupgrade path / ss cipher / vless flow 等）。**仅把解析得到的字段写入；解析不到的字段不虚构默认值；私钥（private-key）属凭据类字段，检测归一不写入 protocol_json**（DesignReport10 安全口径：REALITY 渲染只需 pbk/sid/sni/fp，私钥仅存于 Xray 服务端，面板不落库）。REALITY：若配置含私钥，**不落库**，仅用 x25519 公钥推导写入 `public_key`；`short_id` 取配置首个短 id；`server_name` 取首个 serverNames。
       - 稳定名 `{实例slug}-{tag}`；复用 `internal/node` 导出的 `ValidateNodeName` 与 `CheckRenderNameNamespaceTx`（禁止复制实现）；校验失败，或与**任一节点有效渲染名**（`display_name` 非空则 display_name，否则 name）撞名（非自身），或与 `proxy_groups.name`/强制组名/Clash-mihomo 内建保留代理名（DIRECT / REJECT / REJECT-DROP / PASS / COMPATIBLE）重复 → 记错误日志并跳过该 inbound，返回 skipped 项，**不中断检测、不崩溃**。
       - 四协议（vless/vmess/trojan/shadowsocks）`allocatable=1`；其余 `allocatable=0`。
     - upsert 键 `UNIQUE(instance_id, tag)`：新行插入（enabled=1, is_public=0, missing=0, **display_name=NULL**，last_seen_at=now）；已有行仅更新 protocol/host/port/protocol_json/last_seen_at 与 allocatable（**不覆盖 enabled/is_public/display_name**；蓝图勾选状态存在 assembly_blueprints，本就不在节点行），若 tag 消失后重现则 missing=0。
     - **allocatable 变化清单**：检测事务内收集 `allocatable_changed`（1→0 或 0→1 的节点 id/tag）；事务提交后逐节点调用注入的 `OnNodeVisibilityChanged`（本 Step 预留 nil 安全跳过，Build6 Step3 接线为候选集重算与 Add/Remove diff，DesignReport9 Q2）。
     - 本实例既有节点不在本次响应集合 → `missing=1`（**事务提交后调用 `OnNodeVisibilityChanged`/候选集重算回调，摘除对应 group_nodes 并 RemoveUser diff**）。
     - **missing 恢复清单**：检测事务内收集 `recovered_nodes`（missing 1→0 的节点 id/tag）；事务提交后逐节点调用注入的 `OnNodeVisibilityChanged`（本 Step 预留 nil 安全跳过，Build6 Step3 接线为 AddUser diff）。
     - 返回 `{added, updated, missing, skipped:[{tag, reason}], added_nodes:[{node_id, tag, name}]}`（added_nodes 供 UI 检测回执行内命名；recovered_nodes 仅内部/单测使用，不在 HTTP 响应暴露）。
  3. **`backend/internal/server/xray.go`**：路由（Build2 会话+管理员中间件）
     - `GET/POST/PUT/DELETE /api/admin/xray/instances(/:id)`；
     - `POST /api/admin/xray/instances/test`；
     - `POST /api/admin/xray/instances/:id/detect`。
     - 统一响应结构；错误 400/404/409。
  4. **`backend/internal/server/server.go`** 注册实例路由（暂不注册高级中间件，Step 2 引入后再套用）。
  5. **节点删除的 Xray 清理钩子**：`node.Service.Delete` 删除 `source=xray AND missing=1` 行时，事务前先按既有 `xray_users` 与 `xray_ext_users` 记录收集（email/instance_id/inbound_tag/api_addr）清单；提交后 best-effort `RemoveUser`（面板用户与独立账号两类；Step 3 接入 sync 服务后把口径升级为「受影响 active 用户 × 该节点」∪「既有 xray_ext_users 推送目标 × 该节点」期望集）。**非 missing 的 xray 节点仍禁止删除**（Build5 已实现 UI/后端约束，保持）。
  6. **全局长任务 registry（本 Build 落地，DesignReport9 Q1；用户决策 2026-08-20：registry 下沉为中性包）**：新增 **`backend/internal/tasks`** 中性包——`Registry` 结构（构造注入，禁止包级全局变量持有实例）提供任务登记/终态写回/查询能力；`backend/internal/server/tasks.go` 只做 `GET /api/admin/tasks/:id` 的 session+admin 接入层适配。响应 `{id, kind, status: running/succeeded/failed, result, error}`；kind 枚举 `instance_delete|xray_init|reconcile_exec|off_clear|import`，其中 `instance_delete`（本 Step）/`xray_init`（Step 3）本 Build 落地，`reconcile_exec|off_clear|import` 预留给 Build7。`Registry` 实例由 `server.New` 构造后注入 xray 实例/初始化、Build7 的 OFF 清空与配置导入等服务，业务包 import `internal/tasks`（不 import server）。**registry 进程内维护、不落库；未知 task id 或服务重启后查询一律返回 failed「服务重启，任务中断」**。**Build7 OFF 清空采用「事务内登记」的字面同事务口径：若 DB 事务回滚，内存中会残留幽灵任务，该边界已被用户确认接受（重启后统一 failed 兜底）**。本 Step 的实例删除、Step 3 的初始化、Build7 的对账/OFF/导入全部复用此 registry。
  7. **单测**：地址校验、slug 生成、稳定名撞名跳过（含与 display_name/代理组名/强制组名/Clash-mihomo 内建保留代理名冲突）、detect upsert 不覆盖 enabled/is_public/display_name、missing 置位与恢复、**recovered_nodes 与 allocatable_changed 清单收集与回调出口（nil 安全）**、返回 added_nodes、xray 节点删除前收集 xray_users/xray_ext_users 清单（用假 `Lister` 接口注入解析函数，不依赖真实 gRPC）；**任务 registry 提交/终态/未知 id 合成 failed 单测**。

- **参考代码/伪代码：**

  **upsert 核心**

  ```go
  INSERT INTO nodes (source, name, instance_id, tag, protocol, host, port, protocol_json,
                     is_public, enabled, allocatable, last_seen_at, missing)
  VALUES ('xray', ?, ?, ?, ?, ?, ?, ?, 0, 1, ?, CURRENT_TIMESTAMP, 0)
  ON CONFLICT(instance_id, tag) DO UPDATE SET
      protocol = excluded.protocol, host = excluded.host, port = excluded.port,
      protocol_json = excluded.protocol_json, allocatable = excluded.allocatable,
      last_seen_at = CURRENT_TIMESTAMP, missing = 0
  -- 注意：enabled/is_public/display_name 不在 UPDATE 列中（Design2 §3.2 检测不覆盖）
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/xray/... ./internal/server/... ./...
  ```

- **验收标准：** 全部测试通过；检测 upsert 语义符合 Design2 §3.2；删除实例在 FK 级联后无孤儿 rows；测试连接不落库；`GET /api/admin/tasks/:id` 对合法 task_id 返回任务状态、未知 task_id 返回 failed「服务重启，任务中断」。

---



---

### Step 2：advancedMode 中间件、组节点分配/候选集/公共节点/默认配额

**本 Step 完成后，高级端点统一 403 保护（advanced_mode=off）；组模型从「选定订阅」转为「节点分配 + 默认配额」；候选集并集、公共节点标注与分配排序全部后端生效。**

- **目标：** 落地 Design2 §一/§5.6/§5.10 的组与开关后端。
- **前置条件：** Step 1 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/server/middleware.go`**：`AdvancedMode(cfg)` 中间件——每次请求 `cfg.Get(ctx, config.KeyAdvancedMode)` 实时查 DB；false 返回 403 统一文案「高级功能未开启」。**禁止缓存布尔值**。`/api/admin/xray/*` 全部套用；groups 的节点分配与默认配额端点套用；groups 基础 CRUD 不套用。
  2. **`backend/internal/group/group.go`**：
     - `Group` 增加 `DefaultQuota *float64`、`NodeCount int64`（列表聚合）。
     - 新增 `GroupNode {NodeID, NodeName, DisplayName, RenderName, SortOrder, IsPublic, Source}`（`RenderName` 复用 node 包 `RenderName()` 计算，契约对齐 Design2-UI §9.1 `getGroupDetail` 响应的 `render_name` 字段）；`SetNodes(ctx, id, nodeIDs []int64)`：**拒绝 is_public=1 节点**（400）；只允许 source=xray、enabled=1、allocatable=1、missing=0、实例 enabled=1 的节点；写 group_nodes（**在 `BEGIN IMMEDIATE` 事务内完成「先删后插 + 受影响 active 用户清单收集」，AGENTS §4.6**，sort_order=数组下标）；事务提交后执行 `onNodesChanged` 回调（Step 3 注入同步 diff；本 Step 留 nil 安全跳过）。
     - 新增 `SetDefaultQuota(ctx, id, quota *float64)`（NULL 或 0 均不限；负数拒绝）。
     - 删除 `SetSelections` 残留与旧字段（若 Build4 未删尽，本 Step 清干净）。
     - `CandidateSet(ctx)`：**当前所有已激活 clash-yaml / sr-subs / generic-subs 装配蓝图的 xray 候选节点并集**。SQL 思路：
       ```sql
       SELECT b.selection_json FROM assembly_blueprints b
       JOIN versions v ON v.id = b.version_id
       JOIN subscriptions s ON s.id = v.owner_id AND v.owner_type='subscription'
       WHERE s.current_version = v.version_no AND b.target_syntax IN ('clash-yaml','sr-subs','generic-subs')
       ```
       逐行解析 selection_json 的 `xray_candidates` 字段（xray 节点稳定名 nodes.name 数组），按名查 nodes；display_name 不参与解析。**注意：Build5 `assembly/service.go` 的 `SaveBlueprintTx` 当前把 `selection_json.xray_candidates` 恒写为空数组（占位注释「Build6 注入前为空候选集」）；本 Step 必须先将其改为按生成时勾选节点中 `source='xray'` 的子集填充（node.source 创建后不可变，与 `node_names ∩ xray` 等价），候选集解析统一读 `xray_candidates` 字段——否则候选集恒空、组分配会被全部摘除。**
     - `RecomputeCandidateSet(ctx)`：事务后重算并集；删除 group_nodes 中不在并集或不再满足可用性过滤的分配；返回受影响用户/节点清单供回调。**公共节点退出并集或取消 is_public 时对全部 active 用户 RemoveUser，新增/恢复时 AddUser**——具体推送在 Step 3 回调中执行，本 Step 只把变更事实收集进回调参数。
  3. **候选集重算触发点**（本 Step 接线，回调先为空）：
     - 订阅版本激活切换（`server/subscription.go` 的 versionSwitch，owner=subscription 分支）；
     - 订阅删除（在 `subscription.Service` 注入「候选集重算」函数字段，或由 server handler 在 `Delete` 提交后调用；禁止业务包反向依赖 server）；
     - **平台删除**（`platform.Service.Delete` 事务后，对事务前收集到的全部订阅 ID 逐个触发同一候选集重算回调；现有 platform.Delete 是直接 SQL 删订阅，必须在此补接线，DesignReport9 Q2）；
     - assembly generate 首版自动激活后（Build5 handler，若 target_syntax 非 sr-conf 且 auto_activated）；
     - **节点 enabled/allocatable/missing 变化**（Step 1/Step 3 注入回调），用于摘除不可用 group_nodes；**实例 enabled 变化不触发重算（暂停管理口径，见 Step 1 实例服务与 Design2 §5.9）**；
     - 每次调用幂等：全量重算并只删多余/不可用分配。
  4. **`backend/internal/server/group.go`**：新增 `PUT /api/admin/groups/:id/nodes`、`PUT /api/admin/groups/:id/quota`；**这两个写入路由套 advancedMode**；GET 详情 advanced_mode=on 返回节点分配（含 is_public 标注与 display_name，供 UI 展示有效渲染名）、`candidate_nodes`（当前候选集并集，含 `in_partial_blueprint` 标注供 UI 提示）、default_quota，**advanced_mode=off 不 403，仅返回基础组信息并省略上述高级字段**。**「非候选集已分配」标注为防御性兜底展示**：SetNodes 拒绝候选集外节点、候选集重算事件自动删除越界/不可用分配，该态仅在重算失败/时序窗口出现，UI 红警保留不删（DesignReport6 P2-2/DesignReport7 Q1）。
  5. **`backend/internal/server/server.go`**：`/api/admin/xray` 路由组套 advancedMode（Step 1 的实例路由现在收口）；构造 group service 注入后续同步回调字段。
  6. **`backend/internal/server/status.go`**：确认 advanced_mode 暴露（Build4 已做，本 Step 加单测保证 off 时 false、on 时 true）。
  7. **单测**：off 时 xray/组分配端点 403；候选集并集解析（空并集/多蓝图并集/仅部分模板候选）；SetNodes 越候选集拒绝；is_public 拒绝；RecomputeCandidateSet 删除多余分配且不删公共节点以外的合法分配；**SetNodes 先删后插与受影响用户清单收集同事务（BEGIN IMMEDIATE）**。

- **参考代码/伪代码：**

  **AdvancedMode 中间件**

  ```go
  func AdvancedMode(cfg *config.Service) gin.HandlerFunc {
      return func(c *gin.Context) {
          if !cfg.GetBool(c.Request.Context(), config.KeyAdvancedMode, false) {
              Fail(c, http.StatusForbidden, "高级功能未开启")
              c.Abort()
              return
          }
          c.Next()
      }
  }
  ```

  **group_nodes 写入校验**

  ```go
  // 事务内逐节点：
  SELECT source, is_public, enabled, allocatable, missing, i.enabled
  FROM nodes n JOIN xray_instances i ON i.id = n.instance_id
  WHERE n.id = ?
  // 通过条件：source='xray' AND is_public=0 AND enabled=1 AND allocatable=1
  //          AND missing=0 AND i.enabled=1 AND n.name ∈ 当前候选集
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/group/... ./internal/server/... ./...
  ```

- **验收标准：** 全部测试通过；off/on 状态端点与 403 行为正确；候选集三类场景与公共节点约束单测覆盖；无 group_selections 引用（Build4 已清零，本 Step 复查）；SetNodes 事务性（先删后插 + 受影响用户清单收集同事务）单测覆盖。

---



---

### Step 3：用户凭据首建、生命周期同步、xray_users 状态机与批量初始化

**本 Step 完成后，全部 Design2 §5.5 触发器闭环：用户激活/禁用/删除/换组/组分配变化/节点 enabled 与 is_public 变化均触发 diff 推送或移除；凭据首建并发安全；xray_users 状态 pending/synced/failed 可重试；「开始初始化」幂等批量补推。**

- **目标：** 实现高级模式用户生命周期同步核心。
- **前置条件：** Step 2 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/xray/credentials.go`**：
     - `EnsureCredentials(ctx, userID)`：`BEGIN IMMEDIATE` 事务内：
       1. 读 advanced_mode（off → 返回 ErrAdvancedOff，中止）；
       2. 条件更新 `UPDATE users SET uuid_encrypted=?, proxy_secret_encrypted=? WHERE id=? AND uuid_encrypted IS NULL AND proxy_secret_encrypted IS NULL`（两字段同事务同生同灭，RowsAffected=1 表示本次生成）；UUID v4 + 高熵随机密码在事务内生成并 AES-256-GCM 加密；
       3. 若两字段均已有值直接复用；一有一无视为数据异常返回错误（不自动补齐）。
       > **事务内读取实现约束**：advanced_mode 与签名密钥在事务内必须使用既有 `config.Service.GetTx` / `config.Service.GetSigningKeyTx`；禁止在 `TxImmediate` 闭包内调用 `cfg.Get`/`store.DB()`（当前 `MaxOpenConns=1`，会因连接被事务占用而死锁）。
     - `Credentials(ctx, userID)`：解密返回（uuid, proxySecret）。
  2. **`backend/internal/xray/sync.go`**：`SyncService`（**写入 last_error 前统一截断至 200 字符，不做地址脱敏，Design2 §5.4 错误串口径**）。
     - `type API interface { AddUser/RemoveUser }` 便于 fake 测试；真实实现包 `Client`。
     - `Targets(ctx, userID)`：查「组分配 ∪ 公共」xray 节点，过滤 enabled/allocatable/missing/实例 enabled **以及候选集并集**（以 `nodes.name` 稳定名匹配当前已激活蓝图 xray 候选集，display_name 不参与，口径同 Step 2）；去重（node_id）；排序：组分配按 sort_order、公共节点排后；返回项含 node_id/name/display_name 供状态展示与日志。
     - `PushUser(ctx, userID) (synced, failed int, err)`：入口查 advanced_mode（off 静默跳过）**并校验 users.status=active（非 active 返回 0/0，不推送）**；quota_exceeded=1 时跳过推送但**保持 xray_users 记录并写 last_error（如「已超限，请先重置配额」，DesignReport10 决策）**（Step 5 写字段，本 Step 先查列）；EnsureCredentials；**目标集为空（组分配与公共节点均为空）时直接返回 0/0，不记失败**；对每个 target：写 pending（事务内复查 advanced_mode，off 中止）→ AddUser → 复查 advanced_mode，off 则立即 RemoveUser 补偿 → 成功置 synced，失败置 failed+last_error。
     - `RemoveUserFromTargets(ctx, userID, targets []Target) (removed, failed int)`：每个 target RemoveUser；成功删 xray_users 行，失败置 failed+last_error。删除用户路径必须传入事务提交前收集的 targets。
     - `DiffPush(ctx, userID, oldTargets, newTargets)`：旧 − 新 RemoveUser；新 − 旧 PushUser（同凭据不变）；交集不动；**user 非 active 时只执行旧 − 新 RemoveUser，不执行新 − 旧 AddUser（DesignReport7 P2-11）**。
     - `CollectTargetsTx(ctx, tx, userID)` 供删除/禁用事务内收集。
     - `AfterAdvancedOff(ctx)` 补偿辅助（供 Build7 OFF 清空使用）。
  3. **触发器 wiring（全部在 DB 事务提交后异步执行（goroutine，不阻塞主请求），失败记日志不阻断主流程；Xray 侧结果经 xray_users 状态与手动重试可见）**：
     - `user.Service.Register` 的 active 分支：调用注入的 `onUserActive(ctx, userID)`。
     - `user.Service.CreateFromOidc` 的 pending=false 分支：同回调。
     - `approval.Service.Approve`：同回调；`Reject` 与 `user.AdminService.Delete`：事务提交前收集 targets，提交后 `RemoveUserFromTargets`。
     - `user.AdminService.Create`（active）与 `SetStatus`：active→disabled 收集后移除；disabled→active 推送。
     - `user.AdminService.UpdateGroup`：事务提交后按 diff（旧组分配∪公共 − 新组分配∪公共）执行 Remove/Add。
     - `group.Service.Delete`：用户迁默认组后 diff（旧组节点移除 + 默认组节点推送）；`group.Service.SetNodes` 与 `RecomputeCandidateSet`：受影响 active 用户 diff。
     - `node.Service.SetEnabled`/`SetPublic`：复用 Build5 已预留的 `SetOnXrayChanged` 注入点（`XrayChangedFunc(ctx, node, oldEnabled, oldPublic)`）接线 → **先按 Step 2 口径重算候选集并摘除不可用 group_nodes**，再执行 enabled 1→0 对受影响 active 用户 RemoveUser diff；enabled 0→1 仅对公共节点及仍有 group_nodes 分配的用户 AddUser diff（组分配已被候选集重算摘除的用户须先重新分配，口径同 Design2 §5.6/§5.7）；is_public 变化对全部 active 用户 diff。（检测路径的 `OnNodeVisibilityChanged` 是 xray detect 服务回调，与本注入点分别接线，勿混用。）
     - `node missing 0→1`（本次响应缺失的既有节点置 missing=1，Step 1 检测）：事务提交后回调内收集受影响节点清单，先按 Step 2 口径重算候选集（missing=1 摘除对应 group_nodes），再对受影响 active 用户执行 RemoveUser diff（幂等，失败记同步状态可重试）。
     - `node missing 1→0`（Step 1 检测恢复，recovered_nodes）：事务提交后逐节点按「组分配节点 ∪ 公共节点」口径对受影响 active 用户 AddUser diff（幂等；超限前置拦截同其他 AddUser 钩子；组分配已被候选集重算摘除的用户须先重新分配；advanced_mode off 时入口跳过）。
     - `node allocatable 1→0 / 0→1`（Step 1 检测收集的 allocatable_changed）：先按 Step 2 口径重算候选集（allocatable=0 摘除分配），再对受影响 active 用户执行 RemoveUser/AddUser diff（DesignReport9 Q2；超限前置拦截同其他 AddUser 钩子）。
     - `node.Service.Delete`（仅 `source=xray AND missing=1` 可删）：删除事务提交前按「受影响 active 用户 × 该节点」∪「既有 xray_ext_users 推送目标 × 该节点」收集连接信息与 email（不依赖 xray_users 状态行），提交后 `RemoveUserFromTargets`（幂等；不可达/不存在容忍）；xray_users/xray_ext_users 行由 FK 级联清理。
     - `xray.InstanceService.Delete`：本 Step 起按期望集口径收集「受影响 active 用户 × 该实例节点」（组分配 ∪ 公共）∪「既有 xray_ext_users 推送目标 × 该实例」，删除事务提交后 best-effort `RemoveUser`（实例不可达跳过记 warn）。
     - `user.AdminService.ChangeRole`：**无操作**（代理账号与面板角色无关，Design2 §5.5 触发器表）。
     - 回调注入在 `server.New` 中按依赖顺序完成；**禁止业务包 import server 或形成环**，均用函数字段注入。
  4. **`backend/internal/server/xray.go`** 新增：
     - `POST /api/admin/xray/init`：**异步长任务**——提交即返回 `{task_id}`（**复用 Step 1 落地的全局任务 registry**，kind=xray_init），任务体对全部 active 用户 PushUser，终态写入 `{synced, failed}` 供 `GET /api/admin/tasks/:id` 轮询；幂等。
     - `GET /api/admin/xray/users/:id/sync`：该用户 xray_users 聚合状态与 last_error 摘要。
     - `POST /api/admin/xray/users/:id/retry`：对 failed 记录逐个重试（AddUser 或 RemoveUser 按状态语义；failed 行若期望集仍有则重推，不在期望集则移除）。
  5. **`backend/internal/user/admin.go` 的 List** 增加高级字段（仅 advanced_mode=on 时查询）：本月用量字节、聚合同步状态（含 last_error 摘要，对齐 Design2-UI §9.3）、quota_exceeded（Build7 前端消费）。
  6. **单测（fake API）**：凭据首建并发守卫（两 goroutine 仅一个 RowsAffected=1）；AddUser 成功/失败状态迁移；`already exists.` 幂等；RemoveUser 成功删行失败保留；advanced off 时入口跳过、事务复查中止、gRPC 后补偿 RemoveUser（用可控 fake 验证调用序）；批量 init 幂等；换组 diff 只碰差异集；**非 active 用户换组/组删除迁移不 AddUser、仅清理旧目标**；**missing 1→0 检测恢复触发 AddUser diff**；**allocatable 1→0 重算并 RemoveUser、0→1 对仍分配用户 AddUser**；**实例/节点删除收集面板用户 + 既有 xray_ext_users 两类目标并 RemoveUser**。

- **参考代码/伪代码：**

  **凭据首建事务核心**

  ```go
  err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
      if !advancedOnTx(ctx, tx) { return ErrAdvancedOff } // 事务内复查（第一层）
      uuidVal, secret := randomUUID(), randomSecret()
      uuidEnc, err := config.Encrypt([]byte(uuidVal), signingKeyTx(ctx, tx))
      // ... 同样加密 secret
      res, err := tx.ExecContext(ctx, `UPDATE users SET uuid_encrypted=?, proxy_secret_encrypted=?
          WHERE id=? AND uuid_encrypted IS NULL AND proxy_secret_encrypted IS NULL`,
          uuidEnc, secretEnc, userID)
      if err != nil { return err }
      if n, _ := res.RowsAffected(); n == 1 { return nil } // 本次生成
      // 0 行：并发已建，读回既有密文复用
  })
  ```

  **AddUser 三阶段（入口复查 → 事务复查 → gRPC 后补偿）**

  ```go
  if !s.cfg.GetBool(ctx, config.KeyAdvancedMode, false) { return nil } // 入口实时查库
  // 写 pending 的事务内再次 SELECT advanced_mode；off 则回滚
  err := client.AddUser(ctx, tag, user)
  if s.cfg.GetBool(ctx, config.KeyAdvancedMode, false) == false {
      _ = client.RemoveUser(ctx, tag, email) // 补偿：兜住 OFF 提交前已发出的 AddUser
  }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/xray/... ./internal/user/... ./internal/approval/... ./internal/group/... ./...
  ```

- **验收标准：** 全部测试通过；触发器表 Design2 §5.5 逐行有测试或明确单测覆盖；补偿语义有测试；无缓存 advanced_mode 的全局变量。

---



---

### Step 4：下载动态渲染（蓝图全量重渲染/占位替换）与 Subscription-Userinfo

**本 Step 完成后，用户下载装配生成模板时得到按自身组/凭据动态渲染的专属配置；直接上传内容原样返回；高级模式正确携带 usage 与 profile 响应头；无凭据/开关关闭场景产物仍语法完整。**

- **目标：** 落地 Design2 §5.7 全部下载语义。
- **前置条件：** Step 3 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/download/render.go`**（定稿 download 包，避免反向依赖）：
     - **链接渲染共享化（用户决策）**：Step 实施时先把 Build5 `assembly/links.go` 中非导出的 `srLink`/`genericLink` 及其辅助函数抽取到 `backend/internal/assembly/links/` 共享子包并导出（assembly 包改为调用子包）。**抽取前必须同步关闭 Issue2 R14-03/R14-04/R14-05 的链接缺口**：按 `docs/Reference/Node-Link-Standards.md` §2 补齐 anytls `alpn`/`client-fingerprint`、tuic `alpn`、wireguard `pre-shared-key`、hysteria `alpn`/`mport`/`insecure`、hysteria2 `alpn`、vless `alpn`、vmess(generic) `alpn`/`fp`、trojan `alpn` 等参数（registry 缺失字段一并补齐），并落实「空查询不输出 `?`」「http/socks5 空凭据不输出 userinfo」；补齐 registry↔渲染双向一致性单测后再抽取。本 Step 的 SR/generic 动态节点链接渲染复用该子包，禁止重复实现。
     - `RenderUserSubscription(ctx, subID, userID, content, fileName) ([]byte, error)`：
       - 读当前激活版本是否有 blueprint；无 → 直接返回 content（**直接上传静态成品，不识别占位**）；blueprint 查询本身出错返回 500，不静默按直接上传处理。
       - 读用户组节点 ∪ 公共节点，按可用性过滤 + 候选集过滤（以 `nodes.name` 稳定名匹配 blueprint.selection_json 的 `xray_candidates`（Step 2 已修补为生成时填充），display_name 不参与候选身份判定），**按 node_id 去重兜底（组分配与公共节点重叠时只注入一次）**；manual 静态节点不注入（已在模板/渲染计划中）。
        - **前置修补（Build5 遗留缺陷，版本快照语义前提）：Build5 `render_clash.go` 的 render plan 当前仅存引用（`manual_proxies`=节点名数组、`proxy_groups`=组名数组、`rules`=素材池引用数组），不满足 Build5 Step3「自包含且能无状态重建全文」与本 Step 重渲染需求。实施本 Step 前必须先扩展 render plan 生成：`manual_proxies` 存完整 Clash 条目（map 含 type/server/port/解密后协议字段，节点键为 nodes.name 稳定键）、`proxy_groups` 存生成时点结构（名称+类型+有序成员：节点稳定键与子组名）、`rules` 存冻结规则行（type,value,target，IP 类含 no-resolve 后缀）、`fallback` 两行兜底。版本快照语义（Design2 §2.5「生成的版本为渲染时点快照，不随后续池内容更新而回改」）要求 rules、manual proxies 与 proxy-groups 结构均冻结在计划内，**禁止下载重渲染时重读 pool_entries / proxy_groups 当前定义**；`nodes` 表仅用于：①xray 动态节点行自身的 protocol_json/可用性过滤，②计划内稳定键→当前 `renderName` 映射，**不得用当前 nodes/proxy_groups 定义重建组结构或 manual 条目**。**
       - 解密用户 UUID/代理密码；**UUID 为空** → 占位替换为注释 `# 节点未开通，请联系管理员`，并执行 Clash 蓝图空组降级；**UUID 非空但过滤后的组分配 ∪ 公共节点为空** → SR subs/generic-subs 占位**移除整行**（Clash 仍按蓝图全量重渲染，DesignReport9 Q10）；**advanced_mode=off** → 占位统一替换为 `# Xray 高级模式未启用`（**优先于「节点未开通」**），同样执行蓝图空组降级。
       - 按 target_syntax 分支：
         - `clash-yaml`：按 `render_plan_json` **全量重渲染**。proxies = manual 节点（计划中原样，名称用 `renderName`）+ 动态 xray 节点（vless/vmess/trojan/ss 按协议构造 Clash 条目，`name` 用节点当前 `renderName`）；**render_plan 中的节点引用为 `nodes.name` 稳定键，渲染时通过节点表映射为当前 `renderName`**；所有 proxy-groups 按可达注入节点递归重建（可达集合含 manual 静态节点与 DIRECT；**单个成员不在可达集合内时逐项剔除**；剔除后完全不可达的组整体删除（**强制组豁免删除，成员不可达或注入集为空时统一降级 `[DIRECT]`**）；强制组「🚀直接连接」保留；rules 引用**被删除组**（不含降级保留的强制组）的目标降级 DIRECT 并保留行；**无凭据（UUID 为空）或 advanced_mode=off 场景，在重渲染产物 proxies 区首行输出注释行 `# 节点未开通，请联系管理员`（off 时输出 `# Xray 高级模式未启用`，优先于前者）**（与 SR/generic「占位替换为注释」语义一致，DesignReport10 决策））。
         - `sr-subs` / `generic-subs`：**装配生成模板下载时无论有无占位都必须整体重新 base64**（存储明文、下发 base64，Design2 §5.7）；占位存在时先替换 `# {{xray_nodes}}` 为动态节点行（SR/generic 链接形态，复用上述共享子包的注入渲染函数，凭据为用户 UUID/代理密码），无占位则只重新 base64。**本 Step 起蓝图内容的编码由本分支接管：Build5 Step4 的 base64 收口步骤对蓝图内容不再生效（改由本分支统一执行），避免重复编码（DesignReport6 Q2/R2）**。
         - `sr-conf`：无节点/占位，原样。
     - 动态节点渲染名统一取 `renderName(node)`（display_name 非空则用之，否则 `{实例slug}-{入站tag}`）；同名冲突由 Build4 表达式唯一索引 + 应用层校验保证；改名实时生效，不触发快照改写。
  2. **`backend/internal/download/download.go`**：
     - `ResolveUserDownload`：无标识订阅分支（平台唯一订阅）调用 `RenderUserSubscription`（装配模板动态渲染，直接上传字节原样）；**custom 分支只原样返回自定义内容，不注入节点、不重建**，但仍按用户订阅类附加 usage 头。
     - `PreviewForUser`：管理员预览返回当前激活版本**原文**（装配模板含占位，直接上传原样）；普通用户预览按自身渲染（装配模板走 `RenderUserSubscription`，直接上传原样）——与 Design2 §5.7「管理员预览原文、用户预览按自身渲染」一致。
     - `withPlatformHeaders` 增加高级模式系统注入：`profile-update-interval=6`、`profile-web-page-url`（取站点 URL，覆盖平台同键）；`subscription-userinfo` 仅用户订阅类（subscription/custom）且 advanced_mode=on 时携带；`upload/download` 为 traffic_records 当月累计字节，`total` 为有效配额（quota_override ?? group default_quota；NULL/0 省略），`expire=4102444800`。
     - 分享/规则下载不加 usage 头且内容原样（不渲染）。
     - **默认下载文件名**（Design2 §5.7）：按当前激活装配产物的 target_syntax 区分——clash-yaml → `.yaml`、sr-subs → `.txt`、generic-subs → `.txt`、sr-conf → `.conf`；**直接上传内容（无 blueprint）保留原始扩展名**；装配生成模板按映射计算文件名并随 Content-Disposition 附件名下发（`RenderUserSubscription` 的 `fileName` 入参按此计算后传入）。
  3. **无占位/无凭据/高级 off 场景**：Clash 仍执行蓝图全量重渲染；SR/generic 装配模板仍重新 base64（跳过替换与否都不影响 base64 步骤）；**任何路径不得返回半截 `{{xray_nodes}}` 文本**——占位必须替换为实际节点、注释或整行移除（空目标集，DesignReport9 Q10）。
  4. **性能**：新增 `TestRenderUserSubscription10kRules` 断言 <500ms 级（1 万规则 + 20 用户规模最坏口径）。
  5. **单测**：直接上传原样（字节级比对）；Clash 蓝图剔除不可达组/空组降级/rules 降级；**组内混合有效/失效成员时逐项过滤且输出 YAML 无未定义代理名**；**manual 节点删除后下载仍可解析**；SR/generic 占位替换与重新 base64；无凭据注释；**有凭据但空目标集时占位整行移除**；usage 头四字段与省略规则；advanced off 时无 usage 头、平台头恢复；分享/规则不注入；**下载文件名按 target_syntax 映射（clash-yaml→.yaml、sr-subs/generic-subs→.txt、sr-conf→.conf，直接上传保留原始扩展名）**。

- **参考代码/伪代码：**

  **Clash 蓝图重渲染骨架**

  ```go
  type plan struct {
      Header   yaml.MapSlice
      Manual   []map[string]any
      Groups   []planGroup // 含强制组结构与引用
      Rules    []ruleLine
      Fallback []string
  }
  func (r *Renderer) RenderClash(plan plan, dynamic []nodeLine, reachable map[string]bool) ([]byte, error) {
      // 1) proxies: manual + dynamic（name 一律用 renderName；plan 内节点引用是 nodes.name 稳定键，先经 name→renderName 映射替换）
      // 2) groups: 递归判定可达（DIRECT 恒可达）；剔除不可达；强制组空时 proxies=[DIRECT]
      // 3) rules: 目标组被剔除 → 目标改 DIRECT，行保留
  }
  ```

  **usage 头构造**

  ```go
  headers["subscription-userinfo"] =
      fmt.Sprintf("upload=%d; download=%d; total=%d; expire=4102444800", up, down, total)
  // total=0 或配额 NULL/0 时省略 total 字段；advanced_mode=off 时不设置该头
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/download/... ./internal/xray/... ./...
  cd backend && go test ./internal/download -run TestRenderUserSubscription10kRules -v
  ```

- **验收标准：** 全部测试通过；benchmark 达标；三种订阅下载端点保持 `no-store`；无激活版本 200 注释块回归通过；下载文件名 target_syntax 映射生效。

---



---

### Step 5：流量采集、配额检查、超限摘除与重置配额

**本 Step 完成后，cron 每 10 分钟（可配）逐用户串行采集 Xray 流量并原子累加；超限自动 RemoveUser + quota_exceeded=1；管理员重置配额恢复推送；实例采集状态可观测；首页流量字段高级模式形态与流量卡片开关暴露可用。**

- **目标：** 落地 Design2 §5.8。
- **前置条件：** Step 3 验收通过（可用客户端与同步状态机）。
- **产出文件与操作：**

  1. **`backend/internal/xray/stats.go`**：
     - `CollectInstance(ctx, instance)`：`QueryStats("user>>>{email}>>>traffic", true)` 逐用户串行（**pattern 必须完整前缀，禁止空 pattern**）；返回计数器按 `user>>>{email}>>>traffic>>>uplink/downlink` 解析；解析时按 email 过滤并归一（历史残留 counter 忽略非面板 email）。
     - 差值落库**原子增量**：
       ```sql
       INSERT INTO traffic_records (user_id, ym, uplink, downlink) VALUES (?, ?, ?, ?)
       ON CONFLICT(user_id, ym) DO UPDATE SET
         uplink = uplink + excluded.uplink, downlink = downlink + excluded.downlink,
         updated_at = CURRENT_TIMESTAMP
       ```
       禁止先读后写。UPSERT 外键失败（用户已删）静默跳过，不计采集失败。
     - 更新 xray_instances.last_collect_at / collect_status / collect_error（**collect_error 写库前截断 200 字符，Design2 §5.4**）；失败保留上次成功时间并写错误（连续失败告警 UI 在 Build7）。
  2. **`backend/internal/xray/quota.go`**：
     - `EffectiveQuota(ctx, userID)`：`users.quota_override` 非 NULL 用之，否则 groups.default_quota；**NULL/0 不限流量**。
     - `CheckQuota(ctx, userID)`：SUM(traffic_records WHERE user_id AND ym=当前 UTC 月) > quota → `RemoveUserFromTargets` + `UPDATE users SET quota_exceeded=1`；RemoveUser 失败并入用户同步状态机（failed+last_error）。
     - `ResetQuota(ctx, userID)`：仅 active 用户；`DELETE FROM traffic_records WHERE user_id=? AND ym=?` + `quota_exceeded=0` + 重新 PushUser（凭据不变）；禁用用户 400。
  3. **`backend/internal/cron/`**：
     - `StartXrayCollect(st, xraySvc, cfg, lg)`：ticker 每分钟检查，距上次执行 ≥ `xray_collect_interval_minutes`（默认 10，≥1）才执行；**任务入口每次实时查 advanced_mode，off 跳过**；执行中禁止重入（互斥标记）。
     - 采集顺序：仅 `enabled=1` 实例参与；**每实例先做一次廉价连通性探测（如带 deadline 的 `ListInbounds`），探测失败则跳过该实例本轮并写实例级 collect_error/连续失败告警，不逐用户重试**（DesignReport9 Q14）；探测成功后逐 active 用户串行（全局串行即可）；采集完成后逐用户 CheckQuota。
     - **`backend/cmd/server/main.go` 启动与优雅退出时接入 `StartXrayCollect`/stop**（与访问日志清理同模式）；独立账号采集与配额检查于 Build7 Step1 追加进同一任务（与 ext CRUD 同轮闭环，DesignReport10 确认）。
  4. **`backend/internal/server/xray.go`** 新增：
     - `GET /api/admin/xray/instances/:id/stats`（采集状态与最近成功时间，Build7 UI 用）；
     - `POST /api/admin/xray/users/:id/reset-quota`；
     - `PUT /api/admin/users/:id/quota`（**定稿在 `server/user.go` 注册**并套 advancedMode）：请求 `{quota_override: number|null}`，NULL=继承组默认配额、0=不限流量；写 users.quota_override（Design2 §5.10 users 配额覆盖端点）。
  5. **`backend/internal/server/profile.go`** 新增 `GET /api/profile/traffic`（会话凭据，不受 advancedMode 屏蔽）：返回 `{unlimited, used_bytes, quota_bytes|null, exceeded}`；基础模式 `{unlimited:true}`；高级模式按 traffic_records 当月聚合 + EffectiveQuota 计算（**不限流量（NULL/0）时 `unlimited=true` 且 `quota_bytes=null`**，DesignReport10 决策；DesignReport7 Q3/Design2-UI §9.3）。
  6. **`backend/internal/user/admin.go` List** 补「有效配额」与「quota_override 原值」字段（供 Build7 配额覆盖弹窗回显，对齐 Design2-UI §9.3「配额覆盖值与有效配额」；本月用量/聚合同步状态/quota_exceeded 等其余高级字段 Step 3 已声明，不重复）。
  7. **首页流量高级形态与流量卡片开关暴露**（Design2 §5.10，DesignReport6 Q3；DesignReport10 决策：traffic 移出平台列表、改独立汇总端点）：
     - `internal/server/home.go`：**Build4 Step3 已落地的独立汇总端点 `GET /api/home/summary`（会话凭据），本 Step 补高级模式数值**：`advanced_mode=on` 时 `traffic` 返回 `{unlimited, used_bytes, quota_bytes|null, exceeded}`（used_bytes 取 traffic_records 当月 ym 聚合；quota_bytes 取 EffectiveQuota，**不限流量（NULL/0）时 `unlimited=true` 且 `quota_bytes=null`**；exceeded 取 users.quota_exceeded）；基础模式恒 `{unlimited:true}`（与 Build4 基础模式口径对齐）；**`/api/home/platforms` 保持纯列表包裹、不携带 traffic 字段**（DesignReport10 决策）。
     - `internal/server/status.go`：`/api/system/status` 响应新增 `traffic_card_enabled` 布尔（`cfg.GetBool(ctx, "traffic_card_enabled", true)`，补 Build4 Step4 预留注记），首页流量卡与个人中心「本月流量」行显隐共用。
     - **`xray_collect_interval_minutes` 为内部配置键，本 Step 采集任务直接读取（默认 10，≥1）；API 字段 `collect_interval_minutes` ↔ 内部键的映射归设置服务层，随 Build7 Step2（config/admin.go）实现并补读写单测，本 Step 不提前实现（DesignReport7 Q8 / DesignReport8 M9）**。
  8. **单测（fake StatsAPI）**：pattern 完整前缀断言（fake 拒绝空 pattern）；reset=true 原子语义（fake 返回旧值后清零）；差值累加 UPSERT；重启后 counter 重置差值口径；超限摘除与失败状态；NULL/0 不限；重置恢复并清当月；OFF 跳过采集；已删用户外键失败静默跳过；**实例探测失败跳过本轮且不产生逐用户超时**；**`/api/home/summary` 与 `/api/profile/traffic` 两模式形态（不限流量 `quota_bytes=null`）、`/api/home/platforms` 纯列表包裹及 status traffic_card_enabled 默认 true**；**采集间隔内部键 xray_collect_interval_minutes 读取默认 10 与 ≥1 生效（API 字段映射单测在 Build7 Step2）**。

- **参考代码/伪代码：**

  **采集主循环（单实例）**

  ```go
  // 实例级廉价探测：失败即跳过本实例本轮（DesignReport9 Q14）
  if _, err := client.ListInbounds(probeCtx); err != nil {
      recordCollectError(instance, err)
      continue
  }
  for _, u := range activeUsers {
      email := xray.UserEmail(u.ID)
      resp, err := client.QueryStats(ctx, "user>>>"+email+">>>traffic", true)
      if err != nil { recordCollectError(instance, err); break } // 探测后仍失败：中止该实例本轮
      var up, down uint64
      for _, stat := range resp.Stat {
          name := stat.Name // user>>>{email}>>>traffic>>>uplink|downlink
          if !strings.HasPrefix(name, "user>>>"+email+">>>traffic>>>") { continue }
          if strings.HasSuffix(name, ">>>uplink") { up += uint64(stat.Value) }
          if strings.HasSuffix(name, ">>>downlink") { down += uint64(stat.Value) }
      }
      upsertTraffic(ctx, u.ID, currentYM(), up, down)
  }
  ```

  **配额比较**

  ```go
  quotaGB := effectiveQuota(user) // nil 或 0 直接跳过
  usedBytes := sumTraffic(user.ID, currentYM())
  if float64(usedBytes) > quotaGB*1024*1024*1024 { markExceededAndRemove(user) }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/xray/... ./internal/cron/... ./...
  ```

- **验收标准：** 全部测试通过；采集/配额/重置/超限前置拦截（Step 3 PushUser 与 Step 5 联动）均有 fake 测试；无空 pattern 调用路径；`/api/home/summary` 携带 traffic（两模式形态）且 `/api/home/platforms` 为纯列表包裹。

---



---

### Step 6：假 Xray 服务集成测试与 Build6 端到端验收

**本 Step 完成后，全部后端闭环通过假 Xray gRPC 服务验证；真实 Xray 不可用的环境也可完成验收；Build6 里程碑达成。**

- **目标：** 用内存假服务端到端验证本 Build 全部调用链。
- **前置条件：** Step 5 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/xray/fake_test.go` 或独立测试包**：实现 xray-core 生成的 `HandlerServiceServer` 与 `StatsServiceServer` 接口的最小 fake：
     - 内存 `map[inboundTag]map[email]*UserAccount`；`AlterInbound` 识别 AddUser/RemoveUser 操作，返回与真实 Xray 一致的错误字符串（`already exists.` / `not found.`）。
     - `ListInbounds` 返回 2 个 inbound：vless（REALITY）与 shadowsocks；`QueryStats` 返回 `user>>>{email}>>>traffic>>>uplink/downlink` 计数器，`reset=true` 返回后清零；`GetInboundUsers` 返回当前账号。
     - 用 `bufconn` 或 `net.Listen("tcp","127.0.0.1:0")` 起 gRPC server，注册 fake 服务。
  2. **集成测试剧本（全部自动化）**：
     - 建实例（api_addr 指向 fake）→ 检测节点：vless 与 ss 入库，ss allocatable=1；
     - 开启 advanced_mode → 激活用户 → 断言 fake 收到 vless/ss 两个 AddUser（email 全小写）；
     - 换组/禁用/删除 → 断言 RemoveUser 目标集正确；
     - 批量初始化重复执行幂等（fake 返回 already exists）；
     - 装配生成 Clash 模板（含 xray 占位）→ 用户下载 → 全文含用户 UUID/密码且 YAML 可解析、无占位残留；
     - QueryStats 采集两次 → traffic_records 原子累加；配额 0.000001GB → 超限 RemoveUser + quota_exceeded；重置 → 重新 AddUser；
     - advanced_mode=off → 新激活用户不推送；AddUser 完成后切 off 的并发补偿场景用 fake 延迟注入验证至少一次 RemoveUser。
  3. **回归**：全部既有单测；`go test -race ./internal/xray/... ./internal/download/...`（凭据并发守卫与回调路径重点）。
  4. 更新 Build6 进度表与变更记录；Build7 候选清单指向 Build7。

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd backend && go test -race ./internal/xray/... ./internal/download/... ./internal/group/...
  cd ../frontend && npm run build   # 前端未改，但需确认后端契约变更未破坏 Build5 产物
  ```

- **验收标准：** 全部命令通过；集成测试剧本逐项断言通过；无 `t.Skip` 掩盖关键路径；无真实 Xray 环境依赖（fake 全自动）。

---

## 五、候选构建项（已确认归属 Build7）

| # | 候选 | 说明 |
|---|------|------|
| 1 | 实例级账号对账（四分区：待补推/无头用户/疑似 ext 残留/凭据不一致） | Design2 §5.10 |
| 2 | 独立 Xray 账号（ext 账号 CRUD/双轨凭据/推送目标/配额/采集） | Design2 §5.11 |
| 3 | 配置导入导出 format_version=2（instances/accounts/signing_key 保护） | Design2 §5.4/§5.10 |
| 4 | 高级模式 OFF 清空（确认清单、事务内置位、逐实例 best-effort 清理） | Design2 §一 |
| 5 | 高级模式设置后端（advanced_mode/采集间隔/流量卡片开关） | Design2 §5.10 |
| 6 | 全部高级模式前端（XrayInstancesView/GroupsView/UsersView/SettingsView/Home/Profile） | Design2-UI §3/§4/§8 |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 初始版本：Build6 构建方案（Xray 后端核心），7 个 Step（Step 0~6）；对账/独立账号/OFF 清空/导入导出与前端明确划归 Build7 |
| v1.1 | 2026-08-19 | DesignReport5 核验修订：missing 1→0 检测恢复自动补推接线；实例/节点删除收集面板用户 + 既有 xray_ext_users 两类目标；检测复用 node 包命名校验；下载重渲染逐项过滤悬空成员并补单测 |
| v1.2 | 2026-08-19 | DesignReport6 核验修订：Step5 新增 home traffic 高级模式形态实现与 `/api/system/status` 暴露 `traffic_card_enabled`（Q3）；Step2 注明非候选集标注为防御性兜底展示（P2-2） |
| v1.3 | 2026-08-19 | DesignReport6 复核补齐：Step4 sr-subs 分支写明蓝图编码由本分支接管、Build5 base64 收口对蓝图内容不再生效（R2），并修复该 bullet 缩进 |
| v1.4 | 2026-08-19 | DesignReport7 修订：每实例串行措辞（P2-17）；组详情 off 不 403、省略高级字段（P2-12）；PushUser/DiffPush 统一 active 校验（P2-11）；新增 GET /api/profile/traffic（Q3）；collect_interval_minutes ↔ xray_collect_interval_minutes 映射与单测（Q8）；实例规模建议 1~5 台（P2-15） |
| v1.5 | 2026-08-19 | DesignReport8 修订：实例停用改暂停管理口径，移除实例 enabled 的候选集重算/diff 触发（H1）；初始化端点异步化返回 task_id 与全局任务 registry（M7）；gRPC dial 10s/调用 30s deadline（M7 配套）；文件清单补 errors.go；下载注入按 node_id 去重兜底显式化；采集间隔映射读写单测挪 Build7 Step2（M9）；钩子异步执行口径（M14） |
| v1.6 | 2026-08-19 | DesignReport9 修订：全局任务 registry 与 GET /api/admin/tasks/:id 在本 Build Step1 落地（Q1）；候选集重算补平台删除与 allocatable 变化回调（Q2）；实例服务按实例缓存 Client 确保串行（Q13）；下载空目标集占位整行移除（Q10）；采集实例级探测失败快速中止（Q14） |
| v1.7 | 2026-08-19 | DesignReport10 核验修订：ValidateNodeName 引用修正；下载文件名 target_syntax 映射落实；REALITY 私钥不落库；missing 双向表述拆分；quota_bytes 统一 null；traffic 改独立端点 /api/home/summary；Clash 无凭据注释行；超限写 last_error；host 取 api_addr 定稿；kind 措辞/List 字段/WithTimeout/站点 URL 修正；SetNodes 显式事务 |
| v1.8 | 2026-08-20 | 用户决策：xray-core 依赖改为远端 latest（当前 v1.260327.0），不锁定 v26.7.28；同步更新 Step0 命令与验收标准 |
| v1.9 | 2026-08-20 | 构建前核验修订（代码事实对照 Build4/5 已落地产物）：Step0 dial 懒连接口径澄清（grpc.NewClient 不阻塞，10s 施加于 TestConnection/探测首个 RPC 并加 WithReturnConnectionError）；Step1 slug 白名单扩展注记；Step4 render.go 定稿 download 包、链接渲染抽取 assembly/links 共享子包（用户决策）；Step5 配额端点定稿 server/user.go；里程碑 Xray 版本措辞与约束表统一 |
| v2.0 | 2026-08-20 | 第二轮构建前深度核验修订（Build4/5 验收后代码事实对照）：① Step2 候选集解析钉死读 `selection_json.xray_candidates`，并注明 `SaveBlueprintTx` 当前恒写空数组必须先修补为按 xray 勾选子集填充（否则候选集恒空、组分配被全部摘除）；② Step4 补「render plan 自包含前置修补」——Build5 render plan 仅存名字/引用，不满足 Build5 Step3 自包含要求与重渲染需求，必须先扩展为 manual proxies 完整条目/组生成时点结构/冻结规则行，禁止下载时重读 pool_entries 等当前定义（Design2 §2.5 快照语义）；③ Step0 修正 `WithReturnConnectionError` 措辞（grpc.NewClient 官方文档明示忽略该类选项，快速失败由 ctx deadline 达成）；④ Step0 补 `NodeView` 定义位置；⑤ Step2 `GroupNode` 补 `RenderName`（对齐 UI §9.1 getGroupDetail 契约）；⑥ Step3/Step5 List 高级字段补「last_error 摘要」与「quota_override 原值」（对齐 UI §9.3） |
| v2.1 | 2026-08-20 | 第三轮 Build6/7 事前预检确定性问题修订：① Step1 文件清单补 `detect.go`/`node.go`，slug 共享命名空间补「TableHasSlug 白名单 + subscription.slugExistsTx / slug.ExistsInFourTables 四表清单」双处扩展；② Step2 文件清单补 `platform.go`/`subscription.go`/`server/subscription.go`，订阅删除触发改为注入函数字段或 server handler 调用；③ Step3 node 启停触发改为复用 Build5 已预留 `SetOnXrayChanged`/`XrayChangedFunc`（与 detect 的 OnNodeVisibilityChanged 分开），事务内配置读取点名使用既有 `GetTx`/`GetSigningKeyTx` 防 MaxOpenConns=1 死锁；④ Step4 链接抽取前必须先关闭 Issue2 R14-03/04/05（Node-Link-Standards §2 参数与 `?`/userinfo 形态）并补一致性单测；render plan 冻结口径明确 nodes 表仅用于动态 xray 行/可用性/命名映射 |
| v2.2 | 2026-08-20 | 用户决策落盘：Q2 全局任务 registry 下沉为中性包 `backend/internal/tasks`（server 只做查询路由适配，Registry 构造注入，业务包不 import server）；Q3 OFF 清空按「事务内登记」字面同事务执行，接受 DB 回滚残留幽灵任务边界（重启后统一 failed 兜底）。Step1 文件清单与 item 6 同步修订 |
| v2.3 | 2026-08-20 | 依据 Build6-2 完成 Build6 内部补强：Clash `render_plan_json` 自包含化与 `RenderClashPlan` 全量重渲染；`assembly/links` 共享子包抽取；用户删除/审批拒绝、组删除/组节点变更、节点 missing/allocatable 精确 diff；实例/节点删除期望集清理；假 Xray 端到端剧本。详见 [Build6-2.md](Build6-2.md) |
| v2.4 | 2026-08-21 | Build6/Build7 交付核查未完全达标：对应问题见 [Issue2.md](../Issue/Issue2.md) R15-01~R15-14（异步任务请求 Context、动态节点端口、protocol_json flow/cipher、Clash ss 映射、候选集契约、同步回调/超时、SR/generic 注释与预览、测试缺口、error/死代码）。用户决策：归档由用户决定；`AfterAdvancedOff` 补实现并接线；R15 修复开始时本 Build 受影响 Step 回退 ◧，复验通过后恢复 ✅。暂不修改代码。 |
| v2.5 | 2026-08-21 | 执行 R15 首轮修复：异步任务 Context、节点端口、protocol_json、Clash ss 映射、OFF 事务、Client 30s deadline、回调异步、SR/generic 注释与预览、候选集对象、AfterAdvancedOff、cron stop、trafficPayload 错误处理等已落地；后端 `go test ./...` 通过。R15-07/08/14 仍部分进行，见 Issue2 状态表。 |
| v2.6 | 2026-08-21 | Build1~7 全量核查（见 [Issue3.md](../Issue/Issue3.md) R16 系列）：按用户决策与 Issue2 决策 3，Step5/6 勾选回退 ◧（R16-01 超限摘除销毁期望集、R16-07 测试缺口、R16-08 error 残留、R16-09 smoke 缺口）；R16-01 修复方案用户已确认采用方案 A（摘除不删行）；修复仅记录于 Issue3，未改代码。 |
| v2.7 | 2026-08-21 | 执行 Issue3 优先级 1~3：R16-01/07/08 已闭环，Step5/6 勾选恢复 ✅；新增 cron/xray 测试、10k 用户渲染测试、smoke 脚本重写。 |

