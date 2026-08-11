# DesignOnHold.md — 暂缓项详细设计与 Xray 对接设计

> **文档定位：** 本文档承载三类内容：①已确认设计构想、但明确暂缓开发的功能的详细设计；②**Xray-core 对接（新模型）的当前设计与本地源码研究结论**（设计进行中，待决事项见各章末尾，定稿后落入 Design 文档）；③**深入研究记录**（以 SSPanel-UIM 与 Xray-core 源码为参照的多轮可行性研究，见第四章）。
> 设计基线见 [Design1.md](./Design1.md)；编码约束遵循 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 一、暂缓项清单

| # | 功能 | 设计状态 | 说明 |
|---|------|---------|------|
| 1 | 模块化订阅装配器 | **已激活**（Xray 对接新模型的核心组件，详见第二章） | 原「暂缓 + 仅预留接口」状态于 v1.1 变更：新模型下装配器负责动态生成每平台全局模板，进入开发计划 |

---

## 二、模块化订阅装配器（已激活）

### 2.1 功能定位（v1.1 更新）

管理员在管理面板通过**勾选预置的模块/子模块**动态拼接出订阅模板内容，作为**每平台唯一全局模板**的版本写入订阅池（与文件上传、文本编辑并列的**第三种版本创建方式**，Design1.md 4.1 版本规则统一适用）。

**新模型下的定位变化**（v1.1，Xray 对接新模型核心）：
- 装配产物 = **平台全局模板**（固定参数 + 策略组 + 规则），**不含具体节点行**——节点层由下载时按「用户所属组分配的 Xray 节点」动态注入（见 2.3 与第三章 3.7）
- 支持 Clash YAML 与 Shadowrocket .conf 双语法，管理员可逐级勾选，也可一键采用默认值直接生成
- 参考样例：`DocTemplates/Clashyaml.template.md`（Clash YAML 全量结构）与 `DocTemplates/shadowrocket.template.md`（Shadowrocket .conf 结构）

### 2.2 已确认决策（用户确认，构建时不得偏离）

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 产物归属 | 装配生成是订阅池的**第三种版本创建方式**，产物走 Design1.md 4.1 版本管理，**装配蓝图随版本保存**，支持「重新编辑」生成新版本（v1.1 补充：订阅池已简化为**每平台一份全局模板**，组选定机制删除，装配产物即该平台模板的版本） |
| 2 | 语法策略 | **语法中立的中间模型（RAW）+ 双渲染器**（Clash YAML / Shadowrocket .conf），一套模块数据按目标平台语法渲染 |
| 3 | 外部数据源 | 定义**标准 RAW 格式**（兼容解析常见逐行规则文本），管理员**手动点「同步」**拉取 URL 并缓存，装配时只用缓存 |
| 4 | 级联范围 | 勾选子模块**自动补齐所需策略组**；**固定参数区（port / dns / geox-url / fake-ip-filter 等）完全不参与自动生成，纯手动维护** |
| 5 | 外部 URL 约束 | **仅管理员可配置；拉取超时预置 1 分钟；内容大小上限 50MB；目标地址不设限制——安全边界由部署者自行决策，暂不做过多规划** |
| 6 | **节点层输出（v1.1 新增）** | 装配器**不生成节点行**，节点层输出占位标记 `# {{xray_nodes}}`（注释行）；下载时由系统按「组分配的 Xray 节点 + 用户 UUID」替换为实际节点行（见第三章 3.7） |
| 7 | **策略组节点引用（v1.1 新增）** | 策略组引用的节点名采用系统约定名 `{实例标识}-{入站tag}`（如 `tokyo-a-vless`），与下载注入的节点名保持一致，保证 Clash `proxies[].name` 与 `proxy-groups[].proxies` 引用闭环 |

### 2.3 总体结构：四层模型 + 依赖级联（v1.1 更新节点层）

```
装配蓝图（Blueprint）
├── ① 固定参数层：port / dns / geox-url / fake-ip-filter / ntp 等 —— 模板化，值纯手动填写（不自动生成）
├── ② 节点层：v1.1 起不生成具体节点，仅输出占位标记 `# {{xray_nodes}}`
│            （节点行由下载时按组分配动态注入，见第三章 3.7）
├── ③ 策略组层：组引用节点（引用名 = 约定名「{实例标识}-{入站tag}」，如「国外流量」「苹果国内服务」「DIRECT」）
└── ④ 规则层：按场景拆分为若干子模块，每行规则指向一个策略组
     例：子模块「苹果国内服务」= 一组规则行 → 指向策略组「苹果国内服务」
```

**依赖级联规则**：

- 勾选子模块 → 自动补齐其依赖声明中的策略组（组已存在则跳过）→ 策略组引用的候选节点随组定义一并带入
- 级联**只向策略组方向补齐**；固定参数层任何键值不因勾选而变化（决策 #4）
- 多个子模块依赖同名策略组时按同一组合并（冲突处理细则见「待决事项」）

### 2.4 数据模型（暂缓实体，本阶段不建表）

> 以下为功能需求层面的实体清单；具体表名与字段名由实现层面决定（同 Design1.md 5.3 口径）。

| 实体 | 关键属性 | 关系语义 |
|------|---------|---------|
| 大模块 | 标识、名称、类别（固定参数模板 / 策略组集 / 规则场景集；v1.1：节点集类别取消，节点由组分配驱动）、来源（内置 / 自建 / 外部源）、标准 RAW 片段 | 1:N 子模块 |
| 子模块 | 标识、所属大模块、名称、标准 RAW 片段、**依赖声明（所需策略组列表）** | N:1 大模块 |
| 外部数据源 | 名称、URL（可改动）、上次同步时间、同步状态、缓存的标准 RAW 内容 | 同步成功后其内容转化为可选购的大模块/子模块（与自建模块同构） |
| 装配蓝图 | 所属平台模板版本、目标语法（随平台）、固定参数值集合、勾选的模块/子模块集合、自定义规则组列表 | 1:1 订阅版本（装配生成的版本附带蓝图快照） |

### 2.5 双渲染器映射

| 中间层 | Clash YAML | Shadowrocket .conf |
|--------|-----------|-------------------|
| 固定参数 | 顶层键（`port` / `dns` / `geox-url` / `fake-ip-filter` …） | `[General]` 段键值 |
| 节点 | `proxies` 列表（v1.1：输出占位标记行） | `[Proxy]` 段（v1.1：输出占位标记行） |
| 策略组 | `proxy-groups` 列表 | `[Proxy Group]` 段 |
| 规则 | `rules` 列表 | `[Rule]` 段 |
| 附加段 | — | `[Host]` / `[URL Rewrite]`（是否纳入装配范围待定，见「待决事项」） |

**渲染规则**：

- 输出始终为纯文本；预览遵循 Design1.md 3.4.1「纯文本/代码视图渲染，禁止 HTML」
- 规则行按「规则类型,匹配值,策略组[,附加参数]」拼接，如选择 DOMAIN-SUFFIX 模式、策略组「代理」、域名 aliyun.com → `- DOMAIN-SUFFIX,aliyun.com,代理`（Clash）/ `DOMAIN-SUFFIX,aliyun.com,代理`（Shadowrocket）
- YAML 输出对特殊字符做必要转义；控制字符按 Design1.md 6.3 输入安全口径处理
- **节点占位标记输出**（v1.1）：节点层固定输出 `# {{xray_nodes}}` 注释行（各语法以注释形式，保证模板可独立预览/校验）

### 2.6 装配流程（管理员 UI）

1. 从平台模板版本管理页选择「装配生成」（第三种创建方式）→ 目标平台已确定，**语法由平台决定**
2. 勾选大模块 → 展开勾选子模块（依赖策略组自动补齐并提示；策略组引用节点用约定名）
3. 填写固定参数值（提供模板默认值，纯手动，可一键采用默认）
4. 自定义规则组（可选）：选规则类型（DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / IP-CIDR / PROCESS-NAME 等）+ 选目标策略组 + 填写匹配值列表，逐行拼接
5. 预览（纯文本全文，节点区显示占位标记）→ 确认生成
6. 生成走 4.1 事务（写渲染产物文件 + 保存蓝图快照 + 更新版本列表），自动切换为当前版本
7. 「重新编辑装配」：装配版版本提供入口，加载蓝图快照修改后生成**新版本**（不改写旧版本）

### 2.7 外部数据源机制

- **配置**：仅管理员可增删改 URL；URL 可随时改动
- **同步**：手动触发（点击「同步」）；拉取超时**预置 1 分钟**；内容大小上限 **50MB**；目标地址不设限制（决策 #5，安全边界由部署者自行决策，暂不做过多规划）
- **流程**：点击同步 → 拉取 → 解析为标准 RAW（兼容解析常见逐行规则文本，解析失败记录原因）→ 替换本地缓存 → 记录同步时间与状态；失败时保留旧缓存
- **使用**：装配时只读本地缓存，不发起实时外联；外部源内容入库后与自建模块同构（可勾选、可删除）

### 2.8 与现有机制的衔接（v1.1 更新）

- **版本管理**：装配产物统一适用 Design1.md 4.1（5 版本上限、`BEGIN IMMEDIATE` 事务、不可删当前激活版本等）
- **订阅池（v1.1 简化）**：订阅池从「多份订阅 + 组选定」**简化为每平台一份全局模板**（创建时平台唯一校验）；`group_selections` / `subscription_group_rel` 表删除，`needs_reselect` 标记机制随之消失；组不再选定订阅，只分配 Xray 节点与默认配额（见第三章 3.6）
- **自定义订阅**：保留（用户上传完整配置覆盖，优先级最高，**不注入节点**，内容原样返回）
- **与分流规则资源的边界**：分流规则（Design1.md 3.4.7，Shadowrocket .conf）保持独立资源；装配输出的 .conf 归属订阅池（作为 Shadowrocket 平台的模板版本），两者不混同
- **下载/日志/限流**：装配产物下载复用现有下载端点、访问日志与限流；下载时执行节点注入渲染（见第三章 3.7）

### 2.9 预留接口状态（v1.1 解锁）

1. 管理面板侧边栏「订阅装配」入口与空白占位页已存在（Build3 已建），**本阶段实现为实际装配功能**
2. 版本创建方式的第三种创建方式扩展点已预留（`ContentProvider` 策略接口），**本阶段新增「装配生成」实现**
3. 后端此前不新增端点/表——**本阶段随装配器落地新增**（装配蓝图表等，见 2.4 与第三章 3.9）

### 2.10 分期建议（参考，非承诺；v1.1 调整）

| 期 | 范围 |
|----|------|
| 一期 | Clash YAML 渲染器（含占位标记输出）+ 内置/自建模块库 + 装配编辑器（含蓝图快照与重新编辑）+ 平台模板化改造 |
| 二期 | Shadowrocket .conf 渲染器 |
| 三期 | 外部数据源同步 |

### 2.11 待决事项（恢复开发时再与用户确认）

| # | 事项 |
|---|------|
| 1 | 标准 RAW 格式的字段定义与格式版本号 |
| 2 | 模块/子模块标识的命名空间归属（独立命名空间，还是并入订阅/分享/规则/自定义四类全局唯一命名空间，见 Design1.md 2.2） |
| 3 | 子模块依赖声明的具体结构与同名策略组重复合并的冲突处理 |
| 4 | Shadowrocket `[Host]` / `[URL Rewrite]` 段是否纳入装配范围 |
| 5 | 装配预览与旧版本内容差异对比的交互细节 |
| 6 | 外部数据源解析器的具体兼容格式清单 |

---

## 三、Xray-core 对接设计（新模型，v1.1 新增）

> 本设计基于对本地仓库 `/Users/kyle/Desktop/Repo/Xray-core`（go 1.26 版本）的源码核验。**设计进行中**：已确认决策见 3.4，待决事项见 3.11；定稿后落入 Design 文档。

### 3.1 背景与目标

本系统为订阅管理系统（管理 Clash YAML / Shadowrocket .conf 客户端配置并通过 Token 分发）。新增目标：**对接自托管的 Xray-core 服务端，实现用户生命周期自动同步、流量配额管控与每用户专属订阅内容生成**。

核心能力目标：

1. 面板新用户（注册/审批/管理员创建）激活后**自动推送账号到 Xray**（用户组分配的节点）
2. 用户禁用/删除时**自动从 Xray 移除**
3. **流量配额**管控（自然月按真实日历累计；超限仅移除 Xray 账号；管理员手动重置）
4. **每用户专属订阅**：用户下载的配置 = 平台全局模板 + 组分配节点的节点行（含该用户 UUID，动态注入）

### 3.2 本地 Xray-core 研究结论（源码核验）

#### 3.2.1 API 服务机制（`app/commander`）

- 配置 `api` 块开启：`{tag, listen, services}`；services 可选 `reflectionservice / handlerservice / loggerservice / statsservice / observatoryservice / routingservice`
- `listen` 支持 TCP 地址（如 `127.0.0.1:10085`）或 unix socket（`/`、`@` 开头）
- **无认证、无 TLS**（`grpc.NewServer()` 裸启动）——安全边界由部署者控制（本方案：IP 白名单）
- **并发限制**：官方文档提示约第 10 个并发线程后内核丢弃多余请求——客户端必须串行化/限并发
- 可内嵌为 Go 库（`core.New(config)`），但要求 Go 1.26 + 数十个重型依赖（quic-go/utls/sing 等），与「简单轻量化」原则冲突，**不采用**（选路径 A，见 3.3）

#### 3.2.2 可用 API 能力

| 服务 | 方法 | 用途 |
|------|------|------|
| HandlerService | `AddInbound` / `RemoveInbound` / `AlterInbound` / `ListInbounds` | 入站管理 |
| HandlerService | `AlterInbound` + `AddUserOperation` | **向入站添加用户**（VMess=UUID / Trojan=密码 / SS=密码+加密方式；均需 `Level` + `Email`） |
| HandlerService | `AlterInbound` + `RemoveUserOperation` | **按 email 移除用户** |
| HandlerService | `GetInboundUsers` / `GetInboundUsersCount` | 查询入站用户（email 空=全部） |
| HandlerService | `AddOutbound` / `RemoveOutbound` / `AlterOutbound` / `ListOutbounds` | 出站管理 |
| StatsService | `QueryStats{pattern, reset}` / `GetStats{name}` | 流量查询（counter 名如 `user>>>{email}>>>traffic>>>uplink`），支持查询后重置 |
| StatsService | `GetUsersStats{include_traffic, reset}` | **批量**获取在线用户 + 上下行流量 |
| StatsService | `GetAllOnlineUsers` / `GetStatsOnlineIpList` / `GetStatsOnline` | 在线用户 / 在线 IP / 在线人数 |
| StatsService | `GetSysStats` | 进程运行时（goroutine/内存/uptime） |
| LoggerService | `restartLogger` | 重启日志 |
| RoutingService | `TestRoute` / `SubscribeRoutingStats` | 路由测试 / 路由统计流 |
| ObservatoryService | `GetOutboundStatus` | 节点观测（延迟/可用性，需另配 observatory） |

#### 3.2.3 硬约束与边界（对接必须知晓）

1. **统计需显式开启**：Xray 配置 `policy` 块 `statsUserUplink/Downlink/Online`（用户级）、`systemPolicy` 块 `statsInbound*/statsOutbound*`（入出站级），否则查询为空
2. **Counter 易失**：流量 counter 为内存态，**重启清零**；`reset=true` 归零——必须短周期采集差值落库
3. **官方版本无带宽限速 / 无流量配额 / 无到期时间**：`policy/config.proto` 仅 timeout/stats/buffer；协议 Account（vless/vmess/trojan）无速率与额度字段——配额必须面板侧实现（本方案 3.8）
4. **幂等性**（`proxy/vless/validator.go` 核验）：
   - `AddUser`：同 email 重复添加**报错** `"User xxx already exists."`；同 UUID 不同 email **静默覆盖**（危险，email 为键）
   - `RemoveUser`：email 不存在**报错** `"User xxx not found."`
   - email 匹配**大小写不敏感**（内部 ToLower）——email 规则用全小写
   - → 同步必须 DB 维护状态 + 容忍特定错误（已存在/不存在视为幂等成功）
5. **协议 Account 结构**：vless.Account{id=UUID, flow}；vmess.Account{id=UUID}；trojan.Account{password}——vless/vmess 均以 **UUID 为用户凭据**

### 3.3 架构选型：路径 A（已确认）

```
┌─────────────────────────┐        gRPC（公网/内网，IP 白名单防护）         ┌─────────────────────────┐
│ vpn-sub（管理面）        │ ◄──────────────────────────────────────────► │ Xray-core（独立进程）     │
│  - 用户/组/配额/订阅模板  │    internal/xray 客户端封装                    │  - 独立服务器（1-5 台）    │
│  - 下载动态装配（节点注入）│    （串行调用 + 流量持久化 + 幂等同步）          │  - api.listen 对外开放     │
└─────────────────────────┘                                               │  - policy.stats 开启      │
                                                                          └─────────────────────────┘
```

- Xray 已在**另一台服务器**运行（1-5 台实例范围），API 安全由**部署者 IP 白名单**控制（已确认）
- 本系统作为 gRPC 管理面：用户推送/移除、流量采集、配额检查
- 依赖增量：`google.golang.org/grpc` + protobuf 轻量依赖，不动 Go 版本

### 3.4 已确认决策（用户确认，构建时不得偏离）

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 带宽限速 | **不做**（官方 API 不支持）；只做**流量配额**（面板侧软限） |
| 2 | 订阅内容形态 | **每用户专属订阅**（方案 B）：下载 = 平台全局模板 + 组分配节点行（含用户 UUID）动态装配 |
| 3 | 超限动作 | **仅移除 Xray 账号**（面板用户保留，可登录/下载/查用量） |
| 4 | 配额恢复 | 超限后由**管理员手动重置**（清当月累计 + 重新推送，UUID 不变） |
| 5 | 存量用户 | **不考虑**（无存量用户，不做全量同步/reconcile） |
| 6 | 配额周期 | **自然月，跟随真实日历**（按月聚合，无需定时重置表） |
| 7 | 用户侧展示 | 展示**已用流量 / 总分配流量**（首页平台卡片 + 个人中心） |
| 8 | 管理员侧 | 复用现有用户管理面板（UsersView 扩展：用量/同步状态/覆盖配额/重置），**不新增独立用户数据页** |
| 9 | 推送粒度 | **参考用户组模式**：组分配节点（inbound 级），用户激活向所属组分配的全部节点推送 |
| 10 | 用户组改造 | 组分配从「每平台选定订阅」**改为「用户可用的 Xray 节点 + 用户默认流量配额」**；订阅池简化为每平台一份全局模板（见第二章） |
| 11 | 模板生成 | 配置文档（YAML 等）由**订阅装配功能动态生成**（装配器激活，见第二章），下载时自动按组节点注入 |
| 12 | 配额粒度 | **组默认配额 + 用户可覆盖**（users.quota_override 可空，NULL=继承组） |
| 13 | 换组处理 | **自动迁移**：换组事务提交后，旧组节点移除该用户、新组节点推送（同 UUID） |
| 14 | 实例范围 | 1-5 台 Xray 实例，多实例独立连接/采集/推送，单实例失败隔离 |

### 3.5 用户生命周期同步设计

**同步触发器**（全部在业务层、DB 事务提交后执行，失败记日志不阻断主流程——与现有「欢迎邮件」模式一致）：

| 触发点 | 现有代码位置 | Xray 动作 |
|--------|-------------|----------|
| 自注册免审批（status=active） | `user.Service.Register` | 向所属组全部节点 `AddUser` |
| 审批通过（pending→active） | `approval.Service.Approve` | 同上 |
| 管理员创建用户（直接 active） | `user.AdminService.Create` | 同上 |
| 管理员启用（disabled→active） | `user.AdminService.SetStatus(false)` | 同上 |
| 管理员禁用（active→disabled） | `user.AdminService.SetStatus(true)` | 全部节点 `RemoveUser` |
| 删除用户 / 审批拒绝 | `user.AdminService.Delete` / `approval.Reject` | 全部节点 `RemoveUser`（幂等容忍不存在） |
| 换组 | `user.AdminService.UpdateGroup` | 旧组节点移除 + 新组节点推送（同 UUID） |
| 组节点分配变更 | `group` 服务 | 受影响组内 active 用户 diff 推送/移除 |
| 角色变更 admin⇄user | `user.AdminService.ChangeRole` | 无操作（代理账号与面板角色无关） |

**账号映射规则**：

- Xray email：`user-{id}@vpn.local`（与面板邮箱解耦，改邮箱不影响映射；全小写）
- UUID：用户首次激活时生成（crypto/rand v4），**AES-256-GCM 加密落库**（复用 `config` 服务的签名密钥派生机制）
- 推送状态：`xray_users.sync_status = pending/synced/failed`，失败记 `last_error`，管理面板可见、可手动重试

### 3.6 用户组新模型（组 = 节点授权 + 配额）

```
groups（+ default_quota 默认月度配额 GB）
  └── group_xray_inbounds（组 ↔ Xray 节点多对多：group_id, instance_id, inbound_tag）
        ← 替代原 group_selections（组选定订阅）与 subscription_group_rel
```

- **节点分配**：组可勾选多个节点（实例 × inbound）；用户激活/换组时按此推送
- **默认配额**：组内用户默认月度流量（GB）；用户级可覆盖（users.quota_override）
- **级联**：组删除 → 节点分配级联删；组内用户迁默认组（现有逻辑保留），其 Xray 账号随新组自动迁移
- **删除项**：`group_selections` / `subscription_group_rel` 表删除；`needs_reselect` 标记机制随订阅选定消失
- **换组即时生效**：与现有「换组 Token 实时解析跟随」语义一致（Token 无需清理，下载解析实时跟随新组节点）

### 3.7 下载渲染机制（每用户专属订阅）

```
用户下载 → Token 校验（现有链路）→ 定位平台全局模板（当前版本文件）
  → 读用户所属组 → 组分配的 inbound 列表
  → 生成节点行（每条 inbound 一行：name={实例标识}-{入站tag}，
    vless://{用户UUID}@host:port?flow=...&...，按协议渲染字段）
  → 替换模板中的节点占位标记 `# {{xray_nodes}}` → no-store 返回
```

关键规则：

- **模板** = 装配器生成或管理员上传的**平台全局模板**（含 `# {{xray_nodes}}` 占位标记；无标记的模板原样返回，兼容纯规则模板）
- **节点名约定**：`{实例标识}-{入站tag}`，与装配器策略组引用名闭环（决策 2.2 #7）
- **无 Xray 账号的用户**（推送失败/未推送）：占位标记替换为注释 `# 节点未开通，请联系管理员`，模板其余部分原样——订阅链接始终可用、YAML 语法完整
- **自定义订阅**（用户上传）：**不注入节点**，内容原样返回（用户自包含配置）
- **分享/规则下载**：不渲染（无用户概念），保持现状
- **预览**：管理员预览显示模板原文（含占位标记）；用户预览按自身渲染
- 下载端点现有 `no-store` 禁缓存保证 per-user 内容无缓存污染；附加响应头/文件名/访问日志口径不变

### 3.8 流量配额机制

```
采集（cron，5-10 分钟）：QueryStats("user>>>{email}>>>traffic>>>uplink/downlink", reset=true)
  → 差值 UPSERT traffic_records(user_id, ym '2026-08')
超限检查（同任务）：SUM(本月累计) vs 用户配额（quota_override ?? 组 default_quota）
  → 超限 → RemoveUser + 状态标记「quota_exceeded」
管理员手动重置：POST /api/admin/xray/users/:id/reset-quota
  → 清当月累计 + 重新 AddUser（UUID 不变）+ 状态复位
```

- **自然月**：`traffic_records` 按 `ym`（YYYY-MM）聚合，跟随真实日历滚动，**无需定时重置表**；上月超限用户本月不会自动恢复（恢复 = 管理员手动重置，决策 #4）
- **已知损失**：Xray 重启 counter 清零导致差值丢失（业界通病，接受）；超限生效延迟 ≤ 1 个采集周期
- **Xray 侧前提**：服务器须开启 `policy.statsUserUplink/Downlink`（未开则统计恒为空，管理面板提示）

### 3.9 数据模型草案（迁移 1008_xray.sql 等）

| 表 | 关键字段 | 说明 |
|----|---------|------|
| `xray_instances` | id, name, api_addr, api_tag, enabled | 实例（1-5 台），api_addr 为 TCP 地址（IP 白名单防护） |
| `xray_inbounds` | id, instance_id, tag, protocol(vless/vmess/trojan/ss), host, port, flow, network, path, security, sni, fingerprint, enabled | 节点清单（客户端表达字段，供节点行渲染） |
| `group_xray_inbounds` | group_id, instance_id, inbound_tag | 组 ↔ 节点分配（替代 group_selections） |
| `groups`（改） | + default_quota | 组默认月度配额 |
| `users`（改） | + quota_override（可空） | 用户配额覆盖（NULL=继承组） |
| `xray_users` | user_id, instance_id, inbound_tag, email, uuid_encrypted, sync_status, last_error | 用户 Xray 账号（UUID 加密存储） |
| `traffic_records` | user_id, ym, uplink, downlink | 自然月流量累计（采集差值 UPSERT） |
| `subscriptions`（改） | 平台唯一约束 | 每平台一份全局模板（版本管理保留） |
| 删除 | group_selections / subscription_group_rel | 组选定机制移除 |

### 3.10 管理端点与 UI 影响

**后端新增/改造**：

- `internal/xray/`（新包）：client.go（gRPC 封装 + 调用串行化 + 幂等错误映射）/ sync.go（生命周期同步）/ stats.go（流量采集）/ quota.go（配额检查）
- `internal/server/xray.go`：实例与节点 CRUD、连通性测试、用户同步状态、手动重试、配额重置、流量报表端点
- `internal/download/`：渲染插入（模板 + 组节点 + UUID）
- `internal/group/`、`internal/subscription/`：组改造（节点分配 + 默认配额）、订阅池简化（每平台一份）
- `internal/approval/`、`internal/user/admin.go`：推送钩子（事务提交后）
- `internal/server/home.go`：平台卡片携带已用/总流量
- `internal/cron/`：流量采集与配额检查任务
- `cmd/server/main.go`：组装 + 启动任务

**前端**：

- `views/admin/XrayInstancesView.vue`（新增）：实例/节点管理
- `views/admin/GroupsView.vue`（重构）：节点分配 + 默认配额（替代订阅选定）
- `views/admin/SubscriptionsView.vue`（改造）：平台模板管理（每平台一份）
- `views/admin/AssemblyView.vue`（占位页 → 实现）：装配器
- `views/admin/UsersView.vue`（扩展）：用量、Xray 同步状态、覆盖配额、手动重置
- `views/HomeView.vue` / `ProfileView.vue`（扩展）：已用/总流量展示
- `api/xray.ts`（新增）、`layouts/AdminLayout.vue` 菜单、`router/index.ts` 路由

### 3.11 待决事项（需用户确认后定稿）

| # | 事项 | 推荐 |
|---|------|------|
| 1 | 自定义订阅去留 | **保留**（用户上传完整配置覆盖，优先级最高、不注入节点） |
| 2 | 分享订阅去留 | **暂缓**（分享无「用户」概念，无法替换 UUID，与新模型冲突） |
| 3 | 装配器本期范围 | Clash YAML 渲染器为必需；Shadowrocket .conf 渲染器建议同期或紧随 |
| 4 | 订阅池迁移 | 现有多份订阅 → 每平台一份的收敛方式（合并保留最新版本，或重建） |
| 5 | 节点行渲染字段集 | 按协议定义客户端表达字段（host/port/flow/network/path/security/sni/fingerprint…） |
| 6 | 规则资源（rules） | 确认保持独立不动（分流规则 .conf 独立下载，与订阅模板无耦合） |

---

## 四、深入研究记录（v1.2 新增）

> 多轮深入研究记录：以成熟面板 SSPanel-UIM 的设计为参照、以本地 Xray-core 实际代码为核验依据，结合当前项目实际与互联网公开知识，对第三章方案的可行性进行交叉验证。**本记录包含推测性结论（均已标注），需用户审阅确认。**

### 4.1 研究输入与来源

| 来源 | 类型 | 用途 |
|------|------|------|
| `/Users/kyle/Desktop/Repo/SSPanel-UIM` | 本地源码（PHP 面板） | 成熟设计参照：订阅生成/用户节点模型/流量限额 |
| `/Users/kyle/Desktop/Repo/Xray-core` | 本地源码（go 1.26） | API 能力与配置结构核验 |
| 当前项目 vpn-sub | 本地代码 | 架构适配评估 |
| 互联网（XTLS 讨论 #4877、Marzban、3x-ui、vless URI 规范等） | 公开资料 | 生态实践补充 |

### 4.2 轮次 1：SSPanel-UIM 设计研究结论

**核心机制核验（源码：`src/Services/Subscribe/*`、`src/Controllers/SubController.php`、`src/Models/Node.php`、`src/Models/User.php`）**：

1. **订阅生成模型**：SSPanel 为 9 种订阅格式（clash/json/sip008/singbox/v2rayjson/sip002/ss/v2ray/trojan）提供生成器，全部遵循同一模式：**全局基础模板 + 按用户过滤节点 + 注入用户凭据（uuid/port/passwd/method）**——与本方案「平台全局模板 + 组节点注入 + 用户 UUID」完全同构，验证了第三章 3.7 的设计方向。
2. **Clash 生成细节**（`Subscribe/Clash.php`）：`Clash_Config` 为全局 YAML 基础配置（规则/策略组），`Clash_Group_Indexes` 指定需追加节点名的策略组索引，生成时按节点名追加 `proxies[]` 并 `yaml_emit` 结构合并——**采用「结构化合并」而非文本占位替换**（我们的 `# {{xray_nodes}}` 文本标记更简单，但结构合并更健壮，见 4.7 疑问 #6）。
3. **用户节点可见性**（`Services/Subscribe.php getUserNodes`）：`type=1`（启用）+ `node_class <= user.class`（等级）+ `node_group in [0, user.node_group]`（分组，0=公共节点）+ 节点带宽未超限——**双维度（等级+分组）过滤**；本方案「组→节点多对多」为单维度，更简单（疑问 #7）。
4. **用户凭据模型**（`Models/User.php`）：每用户固定 `uuid/port/passwd/method`（SS 按端口区分用户、vmess/trojan 按 UUID）；另有 `node_speedlimit`（用户限速，由后端实现）、`node_iplimit`（IP 数限制）、`class_expire`（到期）、`transfer_enable`（可用流量）。
5. **订阅 Token**（`Models/Link.php`）：每用户一条 token 记录——与本项目 `download_tokens` 同构。
6. **流量模型**：`u/d` 字段 + `HourlyUsage`（小时粒度 JSON 数组）——SSPanel 为「节点主动上报」模式（push）；本方案为「面板定时拉取」模式（pull，Xray API 决定），存储可参考 HourlyUsage 聚合思想。
7. **Subscription-Userinfo 标准头**（`SubController.php`）：响应头返回 `upload=; download=; total=; expire=`——**客户端（Clash/SingBox/v2rayNG）原生展示流量信息的标准做法**，本方案应采纳（见 4.6 改进 #1）。

### 4.3 轮次 2：Xray-core 交叉核验结论

在第三章 3.2 基础上补充（源码核验）：

1. **传输字段全集**（节点行渲染所需，`transport/internet/*/config.proto`）：
   - StreamConfig：`protocol_name`（network：tcp/ws/grpc/httpupgrade/kcp/splithttp 等）+ `security_type`（none/tls/reality）+ `socket_settings`
   - TLS：`server_name`（sni）、`fingerprint`（uTLS 指纹）
   - Reality：客户端字段 `server_name`/`public_key`/`short_id`/`Fingerprint`/`spider_x`（vless:// 链接的 sni/pbk/sid/fp 参数来源）
   - ws：`path`、`header`（Host）；grpc：`service_name`；httpupgrade：`host`、`path`
2. **inbound JSON 结构**（`infra/conf/xray.go InboundDetourConfig`）：`protocol/port/listen/settings/tag/streamSettings/sniffing`——`xray_inbounds` 表字段设计可完整覆盖。
3. **LimitFallback 字段发现**（`transport/internet/reality/config.proto`）：Reality 配置含 `limit_fallback_upload/download{after_bytes, bytes_per_sec, burst_bytes_per_sec}`，JSON 配置层可解析，但**运行时实现未找到（reality.go 无使用点）**——推测为限速特性预留字段，未生效（疑问 #1）。

### 4.4 轮次 3：方案差距分析（SSPanel 对照）

| 维度 | SSPanel | 本方案 | 差距/启示 |
|------|---------|--------|----------|
| 订阅内容 | 纯动态生成（模板来自环境变量，无版本管理） | 平台模板 + 版本管理（更可控） | 本方案占优；模板来源（装配/上传）更灵活 |
| 用户凭据 | uuid/port/passwd/method | UUID（AES 加密落库） | 一致；SS 协议需额外密码语义（疑问 #2） |
| 节点可见性 | 等级+分组双维度 | 组→节点多对多 | 单维度已满足 1-5 台规模（疑问 #7） |
| 流量获取 | 节点上报（push） | 面板拉取（pull） | Xray API 仅支持 pull，方向已定 |
| 限额 | 总量制 + 自定义重置日 | 自然月制 + 手动重置 | 用户已确认自然月；SSPanel 重置日设计可作远期参考 |
| 到期 | class_expire（标配） | 未设计 | 疑问 #3 |
| 限速 | node_speedlimit（后端实现） | 不做（官方 Xray 无此能力） | 一致；Xray 侧仅 Reality LimitFallback 预留字段 |
| IP 数限制 | node_iplimit | 未设计 | 疑问 #4 |
| 用量展示 | Subscription-Userinfo 头 | 计划 UI 展示 | **应补充标准头**（改进 #1） |
| 订阅限流 | IP+Token 双限流 | 已有下载限流 | 一致 |
| 流量倍率 | traffic_rate | 未设计 | 疑问 #5 |

### 4.5 轮次 4：互联网知识补充

1. **XTLS 官方订阅标准**（[Xray-core Discussion #4877](https://github.com/XTLS/Xray-core/discussions/4877)，2025-07）：`subscription-userinfo`（upload/download/total/expire，字段均可选）、`profile-web-page-url`、`announce`、`support-url`、`profile-update-interval` 为事实标准，v2rayNG/Clash Meta/SingBox/Exclave 等客户端均支持——**本项目平台附加头机制（已预置 profile-update-interval/profile-web-page-url）与官方标准天然对齐**。
2. **Marzban**（主流 Xray 面板，Python+React）：通过 `app/jobs/record_usages.py` 定时任务从 Xray API 拉取用户流量写入数据库（`JOB_RECORD_USER_USAGES_INTERVAL` 可配）——**与本方案「定时 QueryStats 拉取 + 落库」完全一致**，验证 3.8 设计。
3. **3x-ui/X-Panel**（内嵌 Xray 面板）：客户端字段模型 `id/email/limitIp/totalGB/expiryTime` + `delDepletedClients`（批量删除流量耗尽客户端）+ `resetClientTraffic`（重置流量）——**「面板侧记录配额、超限移除客户端、手动重置」与我们的决策 #3/#4 完全一致**；限速/IP 限制由面板生态自行实现（非 Xray API）。
4. **vless:// 链接规范**（[XTLS/Xray-core#716](https://github.com/XTLS/Xray-core/issues/716) 及社区实践）：`vless://{uuid}@{server}:{port}?encryption=none&flow=xtls-rprx-vision&security=reality&sni={域名}&fp=chrome&pbk={公钥}&sid={shortid}&type=tcp#{节点名}`；security 取值 none/tls/reality；type 取值 tcp/ws/grpc/httpupgrade；flow 仅 `xtls-rprx-vision`（Reality 唯一支持）——节点行渲染字段集据此定义。

### 4.6 综合可行性评估（结论）

**第三章方案总体可行，与成熟生态（SSPanel/Marzban/3x-ui）的核心机制一致**：每用户专属订阅（SSPanel 同构）、定时拉取流量（Marzban 同构）、超限移除+手动重置（3x-ui 同构）、订阅头标准（XTLS 官方标准）。

研究带来的设计改进建议：

1. **采纳 Subscription-Userinfo 标准头**：下载响应按用户注入 `upload=; download=; total=; expire=`（expire 本期可省略），与现有平台附加头机制融合（`withPlatformHeaders` 扩展）；用户侧 UI 用量展示保留，双通道。
2. **节点行渲染字段集定稿**：vless（encryption/flow/security/sni/fp/pbk/sid/type/network/path/host）、vmess（base64 JSON：ps/add/port/id/aid/net/type/host/path/tls）、trojan（peer/sni/security/type/path/serviceName/allowInsecure）——`xray_inbounds` 表按此定义字段。
3. **采集间隔默认 10 分钟**（Marzban 默认 15 分钟量级，可配置），超限生效延迟 ≤ 1 个周期。
4. **装配器策略组引用**保持约定节点名（`{实例标识}-{入站tag}`），节点行 name 字段输出同一约定名（与 SSPanel 按索引注入等价效果，更简单）。

### 4.7 疑问清单（推测记录，需用户审阅）

| # | 疑问 | 推测 | 依据 |
|---|------|------|------|
| 1 | Reality `LimitFallback` 限速字段是否生效 | **推测未实现**（配置可解析、运行时无使用点），勿依赖；后续版本可能激活，需跟踪 | Xray-core `reality/reality.go` 无 AfterBytes 引用 |
| 2 | 协议范围：是否支持 SS | **推测本期仅 vless/vmess/trojan**（SS 需每用户密码 + 端口语义，xray_users 模型需扩展 password 字段）；装配器节点类型表同样收敛 | SSPanel SS 生成依赖 user.port/passwd；Xray vless/vmess/trojan 均以 UUID 为凭据 |
| 3 | 到期时间是否纳入本期 | **推测不纳入**（用户未要求）；users 表预留 `expire_at` 字段成本低，后续可加「到期自动移除」 | SSPanel class_expire、3x-ui expiryTime 均为标配 |
| 4 | IP/设备数限制 | **推测不做**：Xray API 无此能力，需访问日志分析（GetStatsOnlineIpList 可看在线 IP 但无法封禁），超出本期范围 | x-ui 系需 fail2ban/独立实现 |
| 5 | 节点流量倍率 | **推测不需要**（1-5 台小规模，配额按实际流量计） | SSPanel traffic_rate 服务于商业计费 |
| 6 | 节点注入机制：文本占位 vs 结构合并 | **推测占位标记先行**（`# {{xray_nodes}}`，实现简单、模板可独立预览）；结构化合并（YAML 解析后注入）作为装配器二期优化 | SSPanel 用结构合并（yaml_emit），更健壮但依赖 YAML 解析 |
| 7 | 公共节点概念（group=0） | **推测不需要**：组→节点多对多可覆盖「所有组可见」场景（勾选全部节点），无需特殊语义 | SSPanel node_group=0 为公共节点 |
| 8 | 节点顺序 | **推测按节点名排序输出**（确定性、可预期） | SSPanel 按 node_class+name 排序 |

---

## 五、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-07 | 初始版本：记录模块化订阅装配器的设计预想、5 项已确认决策与第一次构建预留接口 |
| v1.1 | 2026-08-11 | 整合 Xray-core 对接设计（新模型）：①装配器从暂缓**激活**为核心组件（订阅池简化为每平台一份全局模板、节点层改为占位标记 + 下载时按组注入）；②新增第三章「Xray-core 对接设计」（本地源码研究结论、路径 A 架构、13 项已确认决策、生命周期同步、组=节点授权+配额、下载渲染机制、流量配额机制、数据模型草案、待决事项 6 项） |
| v1.2 | 2026-08-11 | 新增第四章「深入研究记录」：以 SSPanel-UIM 源码与 Xray-core 源码为参照的多轮研究（订阅生成同构验证、Marzban 定时拉取验证、3x-ui 超限移除验证、XTLS 订阅标准、vless URI 规范）；产出 4 项设计改进（Subscription-Userinfo 标准头、节点渲染字段集定稿、采集间隔 10 分钟、节点命名保持约定名）；记录 8 项推测性疑问（含 Reality LimitFallback 限速字段未实现、协议范围、到期/IP 限制/倍率等）待用户审阅 |
