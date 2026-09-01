# DesignOnHold.md — 订阅地址池 / 装配拼接 / 配置生成与 Xray 对接设计（已归档）

> **归档说明**：本文档为增量能力设计的源稿（含修订过程记录），内容已全量规范化转入 [Design2.md](Design2.md)，于 2026-08-15 移入 docs/reports/ 存档；后续构建以 Design2.md 为准。

> **文档定位：** 本文档承载 vpn-sub 增量能力的定稿设计（无暂缓项，经多轮用户确认，构建时不得偏离）：第二章**规则素材池**、第三章**装配拼接**、第四章**配置生成与分发**归属**基础模式**（不依赖 Xray）；第五章 **Xray-core 对接**归属**高级模式**。研究与核验结论见 [Reference/Xray-Core-API.md](../../Reference/Xray-Core-API.md) 与 [Reference/SSpanel.md](../../Reference/SSpanel.md)。
> **术语约定**：「**规则素材池**」（第二章）= 规则条目素材池（域名/IP/进程名等，供规则拼接）；「**订阅地址池**」（第四章）= 每平台存放订阅文件的池（单模板 + 版本历史，即既有「订阅池」）。两池职责分离，不混用。
> 设计基线见 [Design1.md](Design1.md)；编码约束遵循 [AGENTS.md](../../../AGENTS.md)（**唯一强要求**）。本文档与 Design1.md 冲突时以本文档为准，定稿内容后续应同步落入 Design 基线。
>
> **参考样例**（位于 `docs/DocTemplates/`）：
> - `ClashOfficial.yaml.template.md`：Clash（mihomo）官方 YAML 全字段参考
> - `Clash.yaml.template.md`：作者个人实际 Clash YAML 配置（表单默认值与代理组预置库参考）
> - `Shadowrocket.conf.template.md`：作者个人实际 Shadowrocket .conf 分流规则参考
> - `DailyData.txt.template.md`：规则素材池订阅 URL 返回数据内容样例

---

## 一、模式分层与开关语义

基础模式（默认，零配置启动）= Design1.md 现有功能（订阅地址池 / 分享 / 规则 / 自定义订阅 / 平台 / 用户 / 审批 / 配置 / 日志）+ 第二~四章能力；高级模式（第五章 Xray 对接）由面板配置「高级模式」显式开关解锁，开关开启才解锁 Xray 实例管理页与多用户组（启用时提供足够警告与提示即可，不过多考虑开关与实例存在性的状态协调）：

- **功能归属**：第二~四章不依赖 Xray；仅第五章能力属高级模式。开关开启后侧边栏新增「Xray 实例」「用户组」两个入口，用户管理扩展用量/同步状态/配额覆盖列；开关关闭时入口与扩展列全部隐藏，后端高级接口返回 403
- **组概念**：基础模式全面隐藏（侧边栏无入口、用户首页/个人中心不显示所属组、用户列表隐藏「所属组」列，数据层保留默认组关联不变）；高级模式解锁多组 CRUD（组 = Xray 节点授权 + 默认配额，见 5.6）
- **高级开关 OFF（清空）**：开关关闭**一并移除所有由高级模式产生的配置**：Xray 实例与节点数据、组节点分配、Xray 用户推送记录、流量记录、用户 UUID（users.uuid_encrypted）、配额字段（users.quota_override / groups.default_quota），系统回到纯基础模式形态；关闭前给予足够警告提示并要求**如同清空数据的二次输入确认**，开启时同样提示需重新录入实例与分配；关闭后无任何高级数据保留，重新开启须全量重新配置，用户重新生成 UUID 并重新推送
- **开关关闭后的占位行为**：当前激活模板若含节点占位标记，下载时将占位替换为注释（`# Xray 高级模式未启用`）返回，保证 YAML/.conf 语法完整
- **存量数据**：升级重建不做任何迁移，项目内既有订阅数据均视为可放弃，管理员重新上传/装配每平台模板
- **显式 Token**：仅保留于分享订阅与规则；订阅地址池单模板仅走无标识组解析 Token
- **用户首页**：两模式布局完全一致，流量条为独立 Card；基础模式显示「已用 X GB · 不限流量」（可经面板「流量卡片」开关隐藏，默认开启）；高级模式强制显示「已用 X / 配额 Y GB」进度条 + 超限提示（保证超限提示可达）
- **自定义订阅**：两模式均完整保留（用户级覆盖，优先级最高，不注入节点，内容原样返回）
- **配额字段**：高级开关 OFF 时配额字段随其余高级配置一并清空（见上）；高级模式开启期间配额字段静态保留，基础模式不执行配额逻辑

---

## 二、规则素材池（基础模式）

### 2.1 功能定位

管理员在管理面板维护「规则素材池」。每个素材池可挂多个订阅 URL（由管理员自行提供）；系统同步时请求各 URL，将返回内容逐行解析为规则条目并更新本地数据库中的对应素材池。规则素材池作为装配拼接（第三章）的规则素材。例：管理员订阅「苹果域名」URL，本地即生成/更新「苹果域名」素材池，供装配或拼接使用。

### 2.2 池模型与条目来源

- **素材池**：名称、挂接 URL 列表（可多个、可随时改动）、上次同步时间、同步状态、可选定时同步开关
- **条目**：所属池、规则类型、匹配值；按（规则类型 + 匹配值）去重合并
- **两种来源同池使用**：
  - **URL 同步**：点击「同步」拉取池内全部 URL，按 2.3 逐行解析入库
  - **手工维护**：管理员在池管理页手动增删改条目，支持全部规则类型（DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / IP-CIDR / IP-CIDR6 / PROCESS-NAME / PROCESS-NAME-REGEX / USER-AGENT）

### 2.3 URL 内容解析规则

参考 `DailyData.txt.template.md` 样例，逐行解析：

| 行形态 | 解析结果 |
|--------|---------|
| `full:<域名>` | DOMAIN 条目 |
| 裸域名（无前缀） | DOMAIN-SUFFIX 条目 |
| 标准规则行（如 `IP-CIDR,1.2.3.0/24`、`PROCESS-NAME,WeChat`） | **分行直接导入**：按行内声明的规则类型 + 匹配值入库 |
| 无法识别的行 | 跳过并记录原因，不阻断同步 |

第三方订阅源为开放性接口、形态多变（例如有源仅返回 IP-CIDR），故保留「分行直接导入」能力：解析口径保守，可识别的规则行一律按原类型收纳，供用户在拼接时选择使用。

- **条目白名单校验**：URL 同步与手工录入的条目入库前均按规则类型做白名单校验（域名格式 / CIDR 格式 / 进程名合法字符集等），防止匹配值中的逗号、换行等内容在拼接时伪造额外规则行；非法内容判定为非法条目：URL 同步跳过并记录原因（不阻断同步），手工录入拒绝并提示；入库后的值均已通过校验，拼接（见 3.5）可直接使用

### 2.4 同步机制

- **触发方式**：手动触发为主（点击「同步」）；每个池可选开启定时自动同步
- **约束**：拉取超时预置 1 分钟；内容大小上限 50MB；目标地址不设限制——安全边界由部署者自行决策
- **失败处理**：单个 URL 拉取/解析失败时保留旧数据，记录同步状态与原因，不影响池内其他 URL
- **权限**：仅管理员可增删改素材池与 URL

### 2.5 与装配的衔接

装配时管理员勾选规则素材池并指定其目标（Clash 的代理组 / Shadowrocket 的代理双态）；系统读取池内当前全部条目，逐条拼接为规则行（见 3.5）。装配只读库内数据；生成的版本为渲染时点快照，不随后续池内容更新而回改。

---

## 三、装配拼接（基础模式）

### 3.1 功能定位

Clash YAML 与 Shadowrocket .conf 订阅语法不同，但用户期望多端统一的体验。装配拼接让管理员勾选同一套节点、代理组（Clash）/ 代理双态（Shadowrocket）与规则素材池条目，由系统按目标平台语法自动拼接出完整分流配置。管理面板侧边栏「订阅装配」入口（现为占位页）即本功能落地点。

### 3.2 节点：统一模型与双来源

节点统一一张表存储，`source` 字段区分来源：

| 来源 | 录入方式 | 输出行为 |
|------|---------|---------|
| `manual` | 未配置 Xray 服务（或需补充节点）时，管理员按页面模板表单手动添加；协议支持 VLESS / VMESS，字段参考 `ClashOfficial.yaml.template.md`（vless：server / port / uuid / network / tls / udp / flow / client-fingerprint / servername / reality-opts 等；vmess：server / port / uuid / alterId / cipher / udp） | 静态节点，生成配置时直接内联渲染；**UUID 凭据以 AES-256-GCM 加密存储（复用签名密钥派生机制，与决策 #16 同口径）** |
| `xray` | 已配置 Xray（高级模式）时，装配页侧边自动提示检测到的 Xray 节点，供管理员直接选用；手动添加仍并行可用 | 动态节点，下载时按用户 UUID 注入节点行（见 5.7）；**装配器勾选的 Xray 节点构成全局候选集**（模板可注入的节点上限），组在候选集内为每组分配子集，下载按组分配注入（见 5.6） |

- **节点命名约定**（Xray 节点）：`{实例标识}-{入站tag}`（如 `tokyo-a-vless`），代理组引用名与下载注入节点名保持一致，保证引用闭环
- 协议范围与 5.4 决策 #20 一致：仅 VLESS 与 VMESS

### 3.3 代理组（Clash）

- **三类强制组**（系统强制勾选，不可移除）：
  - **直接连接**：内含内置 DIRECT
  - **国外流量**
  - **无法归属的流量**：兜底组（见 3.6 兜底规则）
- **预置组库**：内置参考 `Clash.yaml.template.md` 个人配置的可选组（YouTube / Netflix / 哔哩哔哩 / 国外流媒体 / 苹果海外服务 / 苹果国内服务 / AI / Steam / Steam下载 等），管理员勾选启用，作为更丰富的细节化配置
- **自建组**：支持管理员自定义新建代理组
- **持久化**：代理组定义存全局 `proxy_groups` 表（见 5.9），与装配快照分离；预置组库以种子数据随迁移内置（参考 `Clash.yaml.template.md`），管理员自建组入库同表；装配快照仅记录组的勾选引用
- **组内容约束**：每个代理组须至少包含节点、「直接连接」组、「国外流量」组三者之一；管理员可勾选此前添加过的节点；各组可再包含「直接连接」「国外流量」组作为可切换项（同个人配置形态）

### 3.4 Shadowrocket 双态勾选

Shadowrocket 无代理组概念，装配采用简化双态交互：规则素材（素材池条目 / 手工规则行）仅需勾选**代理或不代理**，渲染为 `PROXY` / `DIRECT`。不复用 Clash 代理组，保证各端语法原生、交互最简。

### 3.5 规则拼接规则

- 管理员勾选规则素材池（可另加手工补充规则行）并指定目标后，系统逐条拼接规则行：`规则类型,匹配值,目标`
  - Clash 的目标为管理员指定的代理组名；Shadowrocket 的目标为 PROXY / DIRECT
  - 例：勾选「苹果域名」池指向「苹果国内服务」组 → `- DOMAIN-SUFFIX,aaplimg.com,🍏苹果国内服务`（`full:` 前缀解析的条目渲染为 `DOMAIN,...`）
- **规则类型支持**：DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / IP-CIDR / IP-CIDR6 / PROCESS-NAME / PROCESS-NAME-REGEX；**USER-AGENT 为 Shadowrocket 专属类型**（如 `USER-AGENT,AppleNews*,PROXY`）
- **IP 规则**：IP-CIDR / IP-CIDR6 规则行一律附加 `no-resolve`
- DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD 两端逻辑通用，语法差异由渲染层映射，管理员只维护一套勾选
- 输出始终为纯文本；预览遵循 Design1.md 3.4.1「纯文本/代码视图渲染，禁止 HTML」；控制字符按 Design1.md 6.3 输入安全口径处理

### 3.6 兜底规则

系统自动固定追加于规则列表末尾，管理员无需操作：

| 语法 | 兜底内容 |
|------|---------|
| Clash | `GEOIP,CN,直接连接` + `MATCH,无法归属的流量` |
| Shadowrocket | `GEOIP,CN,DIRECT` + `FINAL,PROXY` |

---

## 四、配置生成与分发（基础模式）

### 4.1 生成流程

1. 选择目标平台（语法随平台确定：Clash 平台 → YAML；Shadowrocket 平台 → .conf）
2. 以类 form 表单填写配置头部（见 4.2），提供预填写默认值，可一键采用
3. 勾选节点（manual / xray 来源，见 3.2）；配置代理组（Clash，见 3.3）或代理双态（Shadowrocket，见 3.4）
4. 勾选规则素材池并指定目标（见 2.5），可手工补充规则行
5. 预览：全文纯文本渲染，可与当前激活版本做 diff 对比（文本差异高亮，前端实现，**采用成熟 diff 库**；具体选型在 Build 阶段确定，倾向轻量文本 diff 库（如 jsdiff），避免引入 monaco 等重型编辑器，防止重现 Issue1 的 manualChunks 循环依赖问题）
6. 确认生成 → 事务创建该平台订阅新版本（写渲染产物文件 + 保存生成参数快照 + 更新版本列表）；**新版本仅入订阅地址池，不自动生效**，由管理员显式分发（见 4.4）

### 4.2 配置头部表单与默认值

- **Clash**：顶层键 port / socks-port / mixed-port / allow-lan / mode / geox-url / geo-auto-update / log-level / ipv6 / ntp / dns（含 fallback-filter / fake-ip-filter）等，字段范围参考 `ClashOfficial.yaml.template.md`；**默认值以作者个人配置（`Clash.yaml.template.md` 头部）预填写**
- **Shadowrocket**：`[General]` 段键值（bypass-system / skip-proxy / tun-excluded-routes / dns-server / fallback-dns-server / ipv6 等），默认值参考 `Shadowrocket.conf.template.md`
- `[Host]` / `[URL Rewrite]` 不纳入装配范围，如需可在表单扩展区自行补充

### 4.3 节点输出形态

- **manual 节点**：静态内联渲染进生成配置（Clash `proxies` 列表 / Shadowrocket `[Proxy]` 段）
- **xray 节点**：节点区输出占位标记 `# {{xray_nodes}}`（注释行，保证模板可独立预览/校验）；下载时系统按「用户所属组分配的节点 + 用户 UUID」替换为实际节点行（见 5.7）；无占位标记的模板原样返回
- 管理员预览显示模板原文（含占位标记）；用户预览按自身渲染

### 4.4 订阅地址池衔接、分发与重新编辑

- 生成产物归属**订阅地址池**：**每平台一份全局模板**（创建时平台唯一校验）+ 版本历史，版本管理统一适用 Design1.md 4.1（5 版本上限、`BEGIN IMMEDIATE` 事务、不可删当前激活版本等）
- **分发机制（入池 + 显式分发）**：生成与上传均只作为新版本入池，不自动生效；管理员在订阅地址池页面对某平台显式「激活/分发」某版本后，该版本成为当前激活版本并对全体用户生效（基础模式全体用户获得同一份订阅；用户级例外仍由自定义订阅承载，见第一章）
- **版本组件改造（冲突 C1 收敛）**：`CreateVersion` 增加 `activate` opt-in 参数——生成/上传等入池调用一律传 false，不再沿用现有「事务内强制切换激活指针」行为；激活动作仅由订阅地址池页面的「激活/分发」触发（复用 `SwitchVersion`）
- **首次入池自动激活**：平台尚无任何激活版本时，首个入池版本（无论生成或上传）自动成为激活版本，避免新部署后用户下载无可用版本的空窗；后续版本仍须显式分发
- 生成参数（头部表单值 + 节点/代理组/素材池勾选 + 手工规则行）随版本快照保存（`assembly_blueprints` 表，version_id 1:1）
- **重新编辑**：生成过的版本提供重新编辑入口，加载快照修改后生成**新版本**（不改写旧版本；新版本仅入池，需再次显式分发）
- 下载/日志/限流：生成产物下载复用现有下载端点、访问日志与限流

### 4.5 Shadowrocket 输出编码

Shadowrocket 配置生成规则与 Clash 类似（头部 → 节点 → 规则的对应关系，见 3.5），但输出编码不一样：节点链接与订阅内容的编码处理参考 [Reference/SSpanel.md](../../Reference/SSpanel.md) 的订阅输出逻辑；.conf 正文以纯文本下发，下载端点返回禁缓存头（`no-store` 等，AGENTS §4.5）。

---

## 五、Xray-core 对接设计（高级模式）

> 本章全部能力归属**高级模式**（见第一章）。API 能力、硬约束与生态研究结论见 [Reference/Xray-Core-API.md](../../Reference/Xray-Core-API.md)。设计已定稿（决策见 5.4，共 24 项），无待决事项。

### 5.1 背景与目标

本系统为订阅管理系统（管理 Clash YAML / Shadowrocket .conf 客户端配置并通过 Token 分发）。新增目标：**对接自托管的 Xray-core 服务端，实现用户生命周期自动同步、流量配额管控与每用户专属订阅内容生成**。

核心能力目标：

1. 面板新用户（注册/审批/管理员创建）激活后**自动推送账号到 Xray**（用户组分配的节点）
2. 用户禁用/删除时**自动从 Xray 移除**
3. **流量配额**管控（自然月按真实日历累计；超限仅移除 Xray 账号；管理员手动重置）
4. **每用户专属订阅**：用户下载的配置 = 平台全局模板 + 组分配节点的节点行（含该用户 UUID，动态注入）

### 5.2 Xray-core API 能力与硬约束（设计相关要点）

> 完整研究结论（API 服务机制、全量 API 能力表、传输字段、幂等性核验、生态验证）见 [Reference/Xray-Core-API.md](../../Reference/Xray-Core-API.md)。影响本设计的关键要点：

- API **无认证无 TLS**（裸 gRPC），安全边界由部署者 IP 白名单控制；**并发受限**（约 10 并发后丢弃请求），客户端必须串行化
- 流量统计需 Xray 配置显式开启（`policy.statsUserUplink/Downlink`），未开则查询恒为空
- 流量 counter 为内存态，**重启清零**——必须短周期采集差值落库
- 官方无带宽限速/无流量配额/无到期时间——**配额必须面板侧实现**（见 5.8）
- 用户增删以 **email 为键**，幂等同步须容忍「已存在/不存在」特定错误（见 5.5）
- `GetUsersStats` **仅覆盖在线用户**，全量流量采集必须逐用户 `QueryStats`（见 5.8）
- 可内嵌为 Go 库但依赖过重，**不采用**；选独立实例 + gRPC 远程管理（路径 A，见 5.3）

### 5.3 架构选型：路径 A（已确认）

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
- 依赖增量：**引入 `github.com/xtls/xray-core` 模块**，直接使用其 command 包 proto 生成代码（HandlerService/StatsService 等）；go.mod/go.sum 依赖膨胀可接受（编译仅用部分包，不内嵌 Xray 运行时），不动 Go 版本

### 5.4 已确认决策（用户确认，构建时不得偏离）

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
| 10 | 用户组改造 | 组分配从「每平台选定订阅」**改为「用户可用的 Xray 节点 + 用户默认流量配额」**；订阅地址池简化为每平台一份全局模板（见 4.4） |
| 11 | 模板生成 | 配置文档（YAML 等）由**订阅装配功能动态生成**（见第三/四章），下载时自动按组节点注入 |
| 12 | 配额粒度 | **组默认配额 + 用户可覆盖**（users.quota_override 可空，NULL=继承组） |
| 13 | 换组处理 | **自动迁移**：换组事务提交后，旧组节点移除该用户、新组节点推送（同 UUID） |
| 14 | 实例范围 | 1-5 台 Xray 实例，多实例独立连接/采集/推送，单实例失败隔离 |
| 15 | **注入判定** | **有 UUID 即注入**：用户首次激活生成 UUID 后，无论推送状态（pending/failed），下载均注入节点行；管理员重试成功后立即生效，无需用户重新下载 |
| 16 | **UUID 存储** | **users 表新增 uuid_encrypted 列**（每用户一个 UUID，跨节点/实例共用，AES-256-GCM 加密复用签名密钥派生机制）；xray_users 仅记录推送状态（不再存 UUID） |
| 17 | **超限渲染** | **下载内容不更改任何参数**：超限用户订阅照常注入节点行；仅在用户面板提示「流量已超限」 |
| 18 | **节点停用** | **渲染过滤 + 分配保留**：渲染只注入 enabled=1 节点；组分配记录保留，重新启用即恢复注入 |
| 19 | **组删除联动** | **自动迁移**：组删除后组内用户迁默认组，Xray 账号自动迁移（旧组节点移除 + 默认组节点推送，同 UUID），与换组语义一致 |
| 20 | **协议范围** | **仅 VLESS 和 VMESS**：完全移除 SS 与 trojan 选项与设计；nodes.protocol CHECK 约束仅两协议；节点行渲染字段集仅两协议（vless：encryption/flow/security/sni/fp/pbk/sid/type/network/path/host；vmess：base64 JSON：ps/add/port/id/aid/net/type/host/path/tls） |
| 21 | **公共节点** | **增加 is_public 标记**：nodes.is_public=1 的节点对所有组自动可见（下载渲染 = 组分配节点 ∪ 公共节点），组分配 UI 标注公共节点无需分配 |
| 22 | **节点顺序** | **按组分配顺序输出**（group_nodes 记录顺序，UI 支持排序调整）；公共节点排于组分配节点之后 |
| 23 | **用量响应头** | 采纳 **Subscription-Userinfo 标准头**（XTLS 官方事实标准，见 Reference/Xray-Core-API.md）：下载响应注入 `upload=/download=/total=/expire=` 四字段，expire 输出**远未来时间戳 4102444800（2100-01-01）表达无到期**（不用 0，避免部分客户端按 Unix 时间戳解释为「已过期」）；与平台附加头机制融合 |
| 24 | **配置导入导出** | **xray_instances 数据纳入配置导入导出**：export.go 扩展导出实例连接数据，导出格式升级 format_version=2 |

### 5.5 用户生命周期同步设计

**同步触发器**（全部在业务层、DB 事务提交后执行，失败记日志不阻断主流程——与现有「欢迎邮件」模式一致）：

| 触发点 | 现有代码位置 | Xray 动作 |
|--------|-------------|----------|
| 自注册免审批（status=active） | `user.Service.Register` | 向所属组全部节点 `AddUser` |
| OIDC 直接激活（审批开关关闭/命中白名单） | `user.Service.CreateFromOidc`（pending=false 分支） | 向所属组全部节点 `AddUser` |
| 审批通过（pending→active） | `approval.Service.Approve` | 同上 |
| 管理员创建用户（直接 active） | `user.AdminService.Create` | 同上 |
| 管理员启用（disabled→active） | `user.AdminService.SetStatus(false)` | 同上 |
| 管理员禁用（active→disabled） | `user.AdminService.SetStatus(true)` | 全部节点 `RemoveUser` |
| 删除用户 / 审批拒绝 | `user.AdminService.Delete` / `approval.Reject` | 全部节点 `RemoveUser`（幂等容忍不存在） |
| 换组 | `user.AdminService.UpdateGroup` | 旧组节点移除 + 新组节点推送（同 UUID） |
| 组删除 | `group.Service.Delete` | 组内用户迁默认组后自动迁移：旧组节点移除 + 默认组节点推送（同 UUID，决策 #19） |
| 组节点分配变更 | `group` 服务 | 受影响组内 active 用户 diff 推送/移除 |
| 角色变更 admin⇄user | `user.AdminService.ChangeRole` | 无操作（代理账号与面板角色无关） |

**账号映射规则**：

- Xray email：`user-{id}@vpn.local`（与面板邮箱解耦，改邮箱不影响映射；全小写）
- UUID：用户首次激活时生成（crypto/rand v4），**AES-256-GCM 加密后存 users.uuid_encrypted**（每用户一个 UUID，跨节点/实例共用，复用 `config` 服务的签名密钥派生机制；决策 #16）
- 推送状态：`xray_users.sync_status = pending/synced/failed`，失败记 `last_error`，管理面板可见、可手动重试
- **级联**：用户删除时 `xray_users` / `traffic_records` 随外键 ON DELETE CASCADE 清理，无孤儿数据（AGENTS §4.7）

### 5.6 用户组新模型（组 = 节点授权 + 配额）

```
groups（+ default_quota 默认月度配额 GB）
  └── group_nodes（组 ↔ 节点多对多分配：group_id, node_id, sort_order）
        ← 替代原 group_selections（组选定订阅）与 subscription_group_rel
```

- **节点分配**：组可勾选多个节点（nodes 表记录，manual/xray 来源均可）；用户激活/换组时按此推送；组内记录保持**排序字段**（下载渲染按此顺序输出，UI 支持排序调整，决策 #22）
- **候选集约束（见 3.2）**：组的节点分配只能在装配器勾选的候选集内选择（装配器定范围、组定授权）；装配器取消勾选某候选节点时，各组该节点的分配记录级联清理并在 UI 提示（AGENTS §4.7 无悬空引用）；单组场景下装配器勾选与组分配自然重合
- **公共节点（决策 #21）**：nodes.is_public=1 的节点**对所有组自动可见**（无需分配，渲染时排于组分配节点之后）；组分配 UI 标注公共节点无需分配
- **默认配额**：组内用户默认月度流量（GB）；用户级可覆盖（users.quota_override）
- **级联**：组删除 → 节点分配级联删；组内用户迁默认组（现有逻辑保留），其 Xray 账号随新组自动迁移（旧组节点移除 + 默认组节点推送，决策 #19）
- **删除项**：`group_selections` / `subscription_group_rel` 表删除；`needs_reselect` 标记机制随订阅选定消失
- **换组即时生效**：与现有「换组 Token 实时解析跟随」语义一致（Token 无需清理，下载解析实时跟随新组节点）

### 5.7 下载渲染机制（每用户专属订阅）

```
用户下载 → Token 校验（现有链路）→ 定位平台全局模板（当前版本文件）
  → 读用户所属组 → 组分配的节点列表（按分配顺序，决策 #22）∪ 公共节点（is_public=1，排后）
  → 生成节点行（每个节点一行：name={实例标识}-{入站tag}，
    vless://{用户UUID}@host:port?flow=...&...，按协议渲染字段；跳过 enabled=0）
  → 替换模板中的节点占位标记 `# {{xray_nodes}}` → no-store 返回
```

关键规则：

- **模板** = 装配生成的**平台全局模板**（含 `# {{xray_nodes}}` 占位标记，见 4.3；无标记的模板原样返回，兼容纯规则模板与基础模式静态节点模板）
- **节点名约定**：`{实例标识}-{入站tag}`，与装配器节点命名约定闭环（见 3.2）
- **注入范围（候选集约束，见 5.6）**：注入节点 = 组分配节点 ∪ 公共节点；装配生成的模板两者均须属于当前激活模板的装配候选集（装配器勾选快照，见 4.4），候选集之外的节点不注入；**无装配快照的直接上传模板不受候选集约束**，按「组分配节点 ∪ 公共节点」注入（缺口 D3 收敛）
- **注入条件（决策 #15）**：**users.uuid_encrypted 非空即注入**（用户激活时生成）——推送状态（pending/failed）不影响注入；管理员重试成功后立即生效，无需用户重新下载；未生成 UUID 的用户（从未激活/推送未启动）占位标记替换为注释 `# 节点未开通，请联系管理员`，模板其余部分原样——订阅链接始终可用、YAML 语法完整
- **节点停用（决策 #18）**：渲染只注入 enabled=1 节点；组分配记录保留，重新启用即恢复注入
- **超限用户（决策 #17）**：**下载内容不更改任何参数**（照常注入节点行）；仅在用户面板提示「流量已超限」
- **自定义订阅**（用户上传）：**不注入节点**，内容原样返回（用户自包含配置）
- **分享/规则下载**：**原样返回**（含占位标记，不做处理；分享无用户概念，不注入 UUID）
- **用量响应头（决策 #23）**：下载响应注入 `subscription-userinfo: upload=; download=; total=; expire=4102444800`（远未来时间戳表达无到期，不用 0 避免客户端误解为已过期），与平台附加头机制融合；超限用户 total 仍返回配额值
- **预览**：管理员预览显示模板原文（含占位标记）；用户预览按自身渲染
- 下载端点现有 `no-store` 禁缓存保证 per-user 内容无缓存污染；附加响应头/文件名/访问日志口径不变

### 5.8 流量配额机制

```
采集（cron，默认 10 分钟，可配置）：QueryStats("user>>>{email}>>>traffic>>>uplink/downlink", reset=true)
  → 差值 UPSERT traffic_records(user_id, ym '2026-08')
超限检查（同任务）：SUM(本月累计) vs 用户配额（quota_override ?? 组 default_quota）
  → 超限 → RemoveUser + 状态标记「quota_exceeded」
管理员手动重置：POST /api/admin/xray/users/:id/reset-quota
  → 清当月累计 + 重新 AddUser（UUID 不变）+ 状态复位
```

- **自然月**：`traffic_records` 按 `ym`（YYYY-MM）聚合，跟随真实日历滚动，**无需定时重置表**；上月超限用户本月不会自动恢复（恢复 = 管理员手动重置，决策 #4）
- **采集方式**：Xray `GetUsersStats` 仅覆盖在线用户（核验结论见 Reference/Xray-Core-API.md），故**必须逐用户 `QueryStats` 串行采集**（实例规模 1-5 台 × 用户量可控，配合并发限制串行化）；单次采集可合并 pattern 为 `user>>>{email}>>>traffic` 一次取上下行
- **超限用户**：RemoveUser 后该用户 counter 停止产生，**本月累计保留**（不删除）；管理员重置时重新 AddUser（UUID 不变），counter 从 0 重新累计
- **已知损失**：Xray 重启 counter 清零导致差值丢失（业界通病，接受）；超限生效延迟 ≤ 1 个采集周期
- **Xray 侧前提**：服务器须开启 `policy.statsUserUplink/Downlink`（未开则统计恒为空，管理面板提示）

### 5.9 数据模型草案（迁移 1009_xray.sql 等，1008 编号已被占用）

| 表 | 关键字段 | 说明 |
|----|---------|------|
| `xray_instances` | id, name, api_addr, api_tag, enabled | 实例（1-5 台），api_addr 为 TCP 地址（IP 白名单防护）；**纳入配置导入导出**（决策 #24，format_version=2） |
| `nodes` | id, source（**CHECK 仅 manual/xray**）, name, instance_id（xray 来源必填，manual 可空）, tag（xray 来源）, protocol（**CHECK 仅 vless/vmess**，决策 #20）, host, port, flow, network, path, security, sni, fingerprint, is_public 默认 0, enabled, uuid_encrypted（manual 来源，AES-256-GCM 加密，见 3.2） | **统一节点表**（xray_inbounds 升级，冲突 D1 收敛）：manual 节点由管理员表单录入，xray 节点由实例检测入库；客户端表达字段供节点行渲染；is_public=1 对所有组自动可见（决策 #21） |
| `proxy_groups` | id, name, type（preset/custom）, preset_key（预置组标识，可空）, definition_json（组类型、含节点引用与子组引用） | Clash 代理组全局定义（缺口 D2 收敛）：预置库以种子数据随迁移内置（参考 `Clash.yaml.template.md`，见 3.3）；管理员自建组入库同表；装配快照仅记录勾选引用 |
| `group_nodes` | group_id, node_id, sort_order | 组 ↔ 节点分配（替代 group_selections）；sort_order 为组内顺序，UI 可调（决策 #22） |
| `groups`（改） | + default_quota | 组默认月度配额 |
| `users`（改） | + quota_override（可空）、+ uuid_encrypted、+ expire_at（预留，NULL） | 用户配额覆盖（NULL=继承组）；每用户一个 UUID 加密存储（决策 #16）；expire_at 为到期语义预留字段，本期不使用（到期不纳入本期） |
| `xray_users` | user_id, instance_id, inbound_tag, email, sync_status, last_error | 用户 Xray 账号推送状态（**不含 UUID**，决策 #16）；外键 ON DELETE CASCADE（用户删除级联） |
| `traffic_records` | user_id, ym, uplink, downlink | 自然月流量累计（采集差值 UPSERT）；外键 ON DELETE CASCADE（用户删除级联） |
| `subscriptions`（改） | 平台唯一约束 | 每平台一份全局模板（版本管理保留）；**不做任何迁移，项目内既有数据均视为可放弃，升级后订阅地址池清空重建**（见第一章存量数据项） |
| `assembly_blueprints`（新增） | version_id 1:1（NOT NULL REFERENCES versions）, target_syntax, fixed_params_json, selection_json（节点/代理组或双态/素材池勾选，含 Xray 候选集）, custom_rules_json | 装配生成参数快照（见 4.4）；装配生成的版本行 1:1 快照，重新编辑读此恢复 |
| 删除 | group_selections / subscription_group_rel | 组选定机制移除 |

### 5.10 管理端点与 UI 影响

**后端新增/改造**：

- `internal/xray/`（新包）：client.go（gRPC 封装，依赖 **xtls/xray-core command 包** + 调用串行化 + 幂等错误映射）/ sync.go（生命周期同步）/ stats.go（流量采集）/ quota.go（配额检查）
- `internal/server/xray.go`：实例与节点 CRUD、连通性测试、用户同步状态、手动重试、配额重置、流量报表端点
- `internal/download/`：渲染插入（模板 + 组节点 + UUID）+ **Subscription-Userinfo 响应头注入**（决策 #23）
- `internal/group/`、`internal/subscription/`：组改造（节点分配 + 排序 + 默认配额 + 公共节点标注 + 候选集约束）、订阅地址池简化（每平台一份）
- `internal/approval/`、`internal/user/admin.go`：推送钩子（事务提交后）
- `internal/server/home.go`：平台卡片携带已用/总流量；用户面板超限提示（决策 #17）
- `internal/config/export.go`：**xray_instances 纳入导入导出**（format_version=2，决策 #24）
- `internal/cron/`：流量采集（逐用户 QueryStats 串行）与配额检查任务
- `cmd/server/main.go`：组装 + 启动任务

**前端**：

- `views/admin/XrayInstancesView.vue`（新增）：实例/节点管理（含 is_public 标记）
- `views/admin/GroupsView.vue`（重构）：节点分配（含公共节点标注）+ **排序调整** + 默认配额（替代订阅选定）
- `views/admin/SubscriptionsView.vue`（改造）：平台模板管理（每平台一份）
- `views/admin/AssemblyView.vue`（占位页 → 实现）：规则素材池管理 + 装配器（含重新编辑，见第二~四章）
- `views/admin/UsersView.vue`（扩展）：用量、Xray 同步状态、覆盖配额、手动重置
- `views/HomeView.vue` / `ProfileView.vue`（扩展）：已用/总流量展示 + **超限提示**
- `api/xray.ts`（新增）、`layouts/AdminLayout.vue` 菜单、`router/index.ts` 路由

> **UI 规格待补**：上述新增/重构页面的 GUI 规格尚未纳入 Design1-UI.md，待设计完善后补齐，本期不处理 UI 实现细节（Q10）。

---

## 六、变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-15 | 研究报告收敛修订（Q1~Q10）：①xray_inbounds 升级为统一 nodes 表，manual/xray 同表，manual UUID AES-256-GCM 加密（D1/Q1/Q8）；②新增 proxy_groups 全局表 + 预置库种子数据（D2/Q2）；③CreateVersion 增加 activate opt-in 参数、首次入池自动激活（C1/Q3/Q4）；④无蓝图上传模板按组分配∪公共节点注入、无候选集约束（D3/Q5）；⑤高级开关 OFF 改为彻底清空全部高级配置 + 二次输入确认，占位替换为注释（Q6）；⑥素材条目白名单校验（Q7）；⑦装配预览 diff 采用成熟 diff 库（Q9）；⑧新增页面 UI 规格待设计完善后补齐（Q10） |
| 2026-08-15 | 补充确认四项：①两池改名避歧义（规则素材池 / 订阅地址池，文档首部增术语约定）；②生效方式改为「入池 + 显式分发」（生成/上传仅入池不自动生效，4.1/4.4）；③高级模式节点模型定为「装配器候选集 + 组分配子集」（3.2/5.6/5.7，含取消勾选级联清理）；④维持基础/高级模式命名；随改 assembly_blueprints 快照字段（selection_json 含候选集） |
| 2026-08-15 | 文档重构：按「订阅地址池 / 装配拼接 / 配置生成与分发 / Xray 对接」四大版块重组；原「模块化订阅装配器」设计（RAW 中间模型、模块/子模块体系、蓝图四层模型）清理融合为第二~四章（地址池 + 表单装配 + 节点双来源 + 代理组预置库）；原第四章分层设计融入第一章；Xray 对接（原第三章）原样迁移为第五章，随迁移修正：迁移编号 1008→1009（1008 已被占用）、生命周期触发器补充 OIDC 直接激活路径（`CreateFromOidc`） |
| 2026-08-07 ~ 08-13 | v1.0~v1.4 多轮用户确认定稿（装配器 11 项决策 + Xray 对接 24 项决策 + 分层 14 项决策）；研究与核验记录拆分至 [Reference/](../../Reference/)，各版本演进过程见 git 历史 |
