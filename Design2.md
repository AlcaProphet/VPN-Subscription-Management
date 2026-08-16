# Design2.md — VPN 订阅管理系统增量能力设计（订阅装配与 Xray 对接）

> **文档定位：** 本文档是本系统增量能力的设计文档，由 [DesignOnHold.md](./docs/AchievedDocuments/DesignOnHold.md) 的多轮确认定稿内容规范化整理而成。经 Design2Report1~4 四轮审核报告合并核验与两轮设计审查（2026-08-16，决策与勘误落文见第六章变更记录），全部设计已定稿、无待决事项，构建时不得偏离。能力构成：第二章**规则素材池**、第三章**装配拼接**、第四章**配置生成与分发**归属**基础模式**（不依赖 Xray）；第五章 **Xray-core 对接**归属**高级模式**。研究与核验结论见 [Reference/Xray-Core-API.md](./docs/Reference/Xray-Core-API.md)、[Reference/Node-Link-Standards.md](./docs/Reference/Node-Link-Standards.md)、[Reference/SSpanel-Subscribe.md](./docs/Reference/SSpanel-Subscribe.md) 与 [Reference/SSpanel.md](./docs/Reference/SSpanel.md)。
> **术语约定**：「**规则素材池**」（第二章）= 规则条目素材池（域名/IP/进程名等，供规则拼接）；「**订阅地址池**」（第四章）= 每平台存放订阅文件的池（**单模板** + 版本历史，即既有「订阅池」的**单模板化**形态——每平台仅一份，见 4.4 审核 A1 决策）；「**分流规则**」= Shadowrocket 装配产出的 .conf 分流配置（[General] + [Rule]），归入现有「规则」实体共享分发（见 4.4）。各池/实体职责分离，不混用。
> **文档关系**：设计基线见 [Design1.md](./docs/AchievedDocuments/Design1.md)，编码约束遵循 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。本文档与 Design1.md 冲突时以本文档为准，构建完成后定稿结论同步落入基线。
>
> **参考样例**（位于 `docs/DocTemplates/`）：
> - `ClashOfficial.yaml.template.md`：Clash（mihomo）官方 YAML 全字段参考
> - `Clash.yaml.template.md`：作者个人实际 Clash YAML 配置（表单默认值与代理组预设库参考）
> - `Shadowrocket.conf.template.md`：作者个人实际 Shadowrocket .conf 分流规则参考（[General] + [Rule] + [Host] + [URL Rewrite]）
> - `Shadowrocket.subs.template.md`：Shadowrocket 节点订阅内容样例（STATUS=/REMARKS= 头部行 + 逐行节点链接，整体 base64 编码）
> - `DailyData.txt.template.md`：规则素材池订阅 URL 返回数据内容样例

---

## 一、模式分层与开关语义

基础模式（默认，零配置启动）= Design1.md 现有功能（订阅地址池 / 分享 / 规则 / 自定义订阅 / 平台 / 用户 / 审批 / 配置 / 日志）+ 第二~四章能力；高级模式（第五章 Xray 对接）由面板配置「高级模式」显式开关解锁（面板配置新增『高级模式』分区，见 5.10），开关开启才解锁 Xray 实例管理页与多用户组（启用时提供足够警告与提示即可，不过多考虑开关与实例存在性的状态协调）：

- **功能归属**：第二~四章不依赖 Xray；仅第五章能力属高级模式。开关开启后侧边栏新增「Xray 实例」入口、**解锁「用户组」入口**（组入口基础模式隐藏，见下），用户管理扩展用量/同步状态/配额覆盖列；开关关闭时入口与扩展列全部隐藏，后端高级接口返回 403（**高级端点清单与统一 advancedMode 中间件见 5.10**；`/api/system/status` 暴露 advanced_mode 字段供前端隐藏入口）
- **组概念**：基础模式全面隐藏（侧边栏无入口、用户首页/个人中心不显示所属组、用户列表隐藏「所属组」列，数据层保留默认组关联不变）；高级模式解锁多组 CRUD（组 = Xray 节点授权 + 默认配额，见 5.6）
- **高级开关 OFF（清空）**：开关关闭**一并移除所有由高级模式产生的配置**：Xray 实例数据、**source=xray 的节点数据**（**manual 节点属基础模式能力，保留**）、组节点分配、Xray 用户推送记录、流量记录、用户 UUID（users.uuid_encrypted）、**用户代理密码（users.proxy_secret_encrypted）**、配额字段（users.quota_override / groups.default_quota），系统回到纯基础模式形态；**Xray 侧账号清理**：OFF 清空事务提交前收集 xray_users 全部记录（user_id / instance_id / inbound_tag）**与各实例连接信息（api_addr / api_tag）**（清空后实例一并被清，须预先锁定连接目标），事务提交并清库后逐实例 best-effort `RemoveUser`——实例不可达则跳过并记 warn，确认弹窗与部署文档明确提示「不可达实例需手动清理」（与 5.7 实例删除口径一致，审核 A5 决策）；**proxy_groups、groups 行与用户组归属保留**（基础模式数据层保留默认组关联不变）；**用量响应头（subscription-userinfo 等）停止携带**（无流量数据可报）；**并发口径**：OFF 清空事务内置位 advanced_mode=off（与配置写入同事务提交），全部同步钩子入口改为**实时查 DB 标记**（非内存开关），OFF 提交后并发触发的钩子读到 off 即静默跳过，防「清空后又被补推」；收集清单按事务时点快照（审核 F3-4）；关闭前给予足够警告提示并要求**如同清空数据的二次输入确认**（确认弹窗清单展示区分 xray/manual 节点），开启时同样提示需重新录入实例与分配；关闭后无任何高级数据保留，重新开启须全量重新配置，用户重新生成 UUID 并重新推送；**装配快照（assembly_blueprints）保留不清理**：快照内对已删 xray 节点的勾选引用成为悬空引用，重新编辑时复用 4.4「失效项标记并提示剔除」容错机制（与「实体删除不阻断、快照为历史参考」原则一致）
- **开关 ON 时批量初始化**：开关开启（含 OFF 清空后重新开启）的事务提交后，系统自动对全部 active 用户执行批量初始化：无 UUID 与代理密码者生成（users.uuid_encrypted / users.proxy_secret_encrypted）+ 向所属组分配节点 ∪ 公共节点全量推送（AddUser，推送集合口径见 5.5），失败记 xray_users.failed + last_error 可手动重试；一次机制同时覆盖基础模式期存量用户与重新开启的全量重推送；**组无节点分配时跳过该用户不记失败**（审核 C-新11 口径）
- **开关关闭后的占位行为**：当前激活模板若含节点占位标记，下载时将占位替换为注释（OFF 场景统一替换为 `# Xray 高级模式未启用`，**优先于 5.7「节点未开通」无凭据注释**）返回，并同样执行 5.7「proxy-groups 蓝图全量重渲染」（空强制组降级 DIRECT、rules 降级 DIRECT），保证 YAML 语法与 Clash 加载语义双完整（审核 A2 决策）
- **存量数据**：升级**不迁移存量订阅数据**（项目内既有订阅数据均视为可放弃，管理员重新上传/装配每平台模板）；**连锁清理口径**：1009 迁移显式按序清理 download_tokens（显式订阅 Token）/ versions（owner_type='subscription'）/ group_selections / subscription_group_rel 后以「新表 + rename」切换重建 subscriptions（含 UNIQUE(platform_id) 与 product_type），迁移框架新增「迁移后钩子注册表」机制并注册 1009 钩子删除 contents/subscription/ 目录，无标识/自定义 Token 保留（见 5.9，审核 A3/F1-3 决策）
- **显式 Token**：仅保留于分享订阅与规则；订阅地址池单模板仅走无标识组解析 Token；**落地处置**：download_tokens.subscription_id 列与订阅删除级联清理逻辑保留不动（兼容既有库，该态不再新发即可）；管理员指定订阅预览端点简化为**按平台预览当前模板**（新模型每平台一份，subscription_id 参数移除、不再接受）；**管理员首页平台卡片改为预览形态**：仅展示模板信息与「按平台预览当前模板」按钮（会话凭据预览端点），不再提供一键导入/复制链接（审核 D2 决策）
- **用户首页**：两模式布局完全一致，流量条为独立 Card；基础模式**仅显示「不限流量」**（无流量采集数据源，隐藏已用数值）；高级模式强制显示「已用 X / 配额 Y GB」进度条 + 超限提示（保证超限提示可达）（流量卡片可经**面板配置新增的「流量卡片」开关**隐藏，默认开启，见 5.10）
- **自定义订阅**：两模式均完整保留（用户级覆盖，优先级最高，不注入节点，内容原样返回）
- **配额字段**：高级开关 OFF 时配额字段随其余高级配置一并清空（见上）；高级模式开启期间配额字段静态保留，基础模式不执行配额逻辑

---

## 二、规则素材池（基础模式）

### 2.1 功能定位

管理员在管理面板维护「规则素材池」。每个素材池可挂多个订阅 URL（由管理员自行提供）；系统同步时请求各 URL，将返回内容逐行解析为规则条目并更新本地数据库中的对应素材池。规则素材池作为装配拼接（第三章）的规则素材。例：管理员订阅「苹果域名」URL，本地即生成/更新「苹果域名」素材池，供装配或拼接使用。

### 2.2 池模型与条目来源

- **素材池**：名称、挂接 URL 列表（可多个、可随时改动）、上次同步时间、同步状态、可选定时同步开关
- **条目**：所属池、规则类型、匹配值、**来源（url / manual）**、**排序（sort_order）**；按（规则类型 + 匹配值）去重合并；**排序口径**：URL 同步按源文件行序分配 sort_order（多 URL 按挂接顺序拼接），手工条目可在池管理页拖拽调序；装配渲染按 sort_order 输出（分流规则顺序有实际语义：首条匹配生效，见 2.5/3.5）
- **两种来源同池使用**：
  - **URL 同步**：点击「同步」拉取池内全部 URL，按 2.3 逐行解析入库（仅差量更新 url 来源条目，见 2.4）
  - **手工维护**：管理员在池管理页手动增删改条目（条目来源标记 manual，**永不受 URL 同步影响**），支持全部规则类型（DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / IP-CIDR / IP-CIDR6 / PROCESS-NAME / PROCESS-NAME-REGEX / USER-AGENT）；**手工添加的条目若与既有条目（含 url 来源）同（规则类型 + 匹配值），拒绝并提示已存在**（去重唯一约束，避免双来源同值歧义）

### 2.3 URL 内容解析规则

参考 `DailyData.txt.template.md` 样例，逐行解析：

| 行形态 | 解析结果 |
|--------|---------|
| `full:<域名>` | DOMAIN 条目 |
| 裸域名（无前缀） | DOMAIN-SUFFIX 条目 |
| 标准规则行（如 `IP-CIDR,1.2.3.0/24`、`PROCESS-NAME,WeChat`） | **分行直接导入**：按行内声明的规则类型 + 匹配值入库（**逗号多于两段时仅取前两段，多余段忽略并记录**；域名统一 lowercase 规范化，CIDR 按规范格式归一） |
| 无法识别的行 | 跳过并记录原因，不阻断同步 |

第三方订阅源为开放性接口、形态多变（例如有源仅返回 IP-CIDR），故保留「分行直接导入」能力：解析口径保守，可识别的规则行一律按原类型收纳，供用户在拼接时选择使用。

- **条目白名单校验**：URL 同步与手工录入的条目入库前均按规则类型做白名单校验（域名格式 / CIDR 格式 / 进程名合法字符集等），防止匹配值中的逗号、换行等内容在拼接时伪造额外规则行；非法内容判定为非法条目：URL 同步跳过并记录原因（不阻断同步），手工录入拒绝并提示；入库后的值均已通过校验，拼接（见 3.5）可直接使用

### 2.4 同步机制

- **触发方式**：手动触发为主（点击「同步」）；每个池可选开启定时自动同步（**每日执行，每池可配执行时刻，默认凌晨 04:00 低峰，按 UTC**——与 5.8 流量月界口径一致；进程内 ticker 检查到期池执行）；**停机错过不补偿**：错过当日执行时刻则等待下一周期，不做补跑（用户决策，审核 D12 推荐方案未采纳）；**同步执行采用异步任务 + 轮询模式**（提交任务后前端轮询状态查询端点，避免同步耗时超过前端请求超时；任务端点见 5.10，审核 F1-4）
- **同步更新语义（差量 + 来源隔离）**：URL 同步仅对 **url 来源**条目做差量更新——按本次同步结果新增/更新，源中已消失的行删除；**manual 来源条目永不受同步影响**；事务批量 UPSERT + 差量删除，支持数万行规模（暂不增加条目来源 URL 追踪列，origin_url 不纳入本期）
- **数万行规模应对**：条目去重索引 UNIQUE(pool_id, rule_type, match_value)；批量写入事务化；条目管理页分页展示（不整表加载）；装配时读池内全部条目拼接（规则行数达数万时产物文件体积可接受，无额外限制）
- **约束**：拉取超时预设 1 分钟；内容大小上限 50MB；目标地址不设限制——安全边界由部署者自行决策
- **失败处理**：单个 URL 拉取/解析失败时保留旧数据，记录同步状态与原因，不影响池内其他 URL；**空响应视为该 URL 失败**（保留旧数据、不差量删除，记录原因）；**部分失败时不执行差量删除**（防止不完整结果误删）；手动与定时同步对同一池不并发执行（进行中再次触发则提示等待）
- **权限**：仅管理员可增删改素材池与 URL

### 2.5 与装配的衔接

装配时管理员勾选规则素材池并指定其目标（Clash 的代理组 / SR 分流规则的 PROXY/DIRECT 双态，见 3.4/3.5）；系统读取池内当前全部条目（**按 sort_order 排序**），逐条拼接为规则行（见 3.5）。装配只读库内数据；生成的版本为渲染时点快照，不随后续池内容更新而回改。

---

## 三、装配拼接（基础模式）

### 3.1 功能定位

Clash YAML 与 Shadowrocket .conf 订阅语法与产物形态均不同：Clash 单文件即含节点（proxies）、代理组（proxy-groups）与分流规则（rules）；Shadowrocket 则拆分为**节点订阅**（base64 编码的节点链接列表，见 `Shadowrocket.subs.template.md`）与**分流规则**（.conf，见 `Shadowrocket.conf.template.md`）两份独立内容。装配拼接让管理员勾选同一套节点与规则素材池，由系统按目标平台形态自动生成对应产物。管理面板侧边栏「订阅装配」入口（现为占位页）即本功能落地点，SR 平台提供**两个独立装配器**（节点订阅 / 分流规则），可单独生成（见 3.4）。

### 3.2 节点：统一模型与双来源

节点统一一张表存储（`nodes`，见 5.9），`source` 字段区分来源：

| 来源 | 录入方式 | 输出行为 |
|------|---------|---------|
| `manual` | 未配置 Xray 服务（或需补充节点）时，管理员按页面模板表单手动添加；**协议支持 `ClashOfficial.yaml.template.md` 中全部代理协议类型**（ss / vmess / vless / trojan / hysteria / hysteria2 / tuic / wireguard / http / socks5 / snell / anytls / mieru / masque / openvpn / ssh / shadowquic / trusttunnel / tailscale 等；**ssr 除外**，见 4.5）；节点参数按所选协议以 JSON 存储该协议的 Clash 原生字段集（含凭据字段：uuid / password / private-key 等） | 静态节点：Clash YAML 按存储字段**原样内联渲染**（proxies 条目，零转换）；SR 节点订阅按 4.5 链接映射转为节点链接，**无法转为链接的协议跳过并在生成结果中提示**；**凭据以 AES-256-GCM 加密存储（复用签名密钥派生机制，与决策 #16 同口径）** |
| `xray` | 已配置 Xray（高级模式）时，由**实例检测入库**：实例保存时 + XrayInstancesView 手动「刷新节点」触发 `ListInbounds` 检测（手动为主，不做定时轮询，避免 Xray API 并发受限压力）；以 instance_id+tag 为 upsert 键（nodes UNIQUE(instance_id, tag)，见 5.9）：新 inbound 入库（默认 enabled）、字段变更更新、**已入库节点的 enabled / allocatable / 装配勾选状态不被检测覆盖**；Xray 侧已删除的 inbound 标记提示由管理员处置；装配页侧边自动提示检测到的 Xray 节点供选用；手动添加仍并行可用 | 动态节点，下载时按用户凭据注入节点行（UUID / 代理密码，见 5.5/5.7）；**装配器勾选的 Xray 节点构成装配时点候选集**（模板可注入的节点上限；组管理候选集以已激活蓝图并集为准，见 5.6），组在候选集内为每组分配子集，下载按组分配注入（见 5.6） |

- **节点命名约定**（Xray 节点）：`{实例slug}-{入站tag}`（如 `tokyo-a-vless`；实例 slug 为 xray_instances.slug 列，`instance-` 前缀短码，见 5.9，审核 F4-6），代理组引用名与下载注入节点名保持一致，保证引用闭环；manual 节点手工录入重名（nodes.name 全局唯一）时拒绝并提示 409
- **协议可扩展注册**：协议类型不硬编码——应用层维护协议注册表（表单 schema + SR 链接映射规则 + **每协议敏感字段清单**：uuid / password / private-key 等凭据字段，仅清单内字段加密存储与解密渲染；**编辑回显时密文字段空值 = 保留原凭据**，见 5.9），节点 protocol 存字符串（无硬编码枚举 CHECK，由应用层按注册表校验）；Clash YAML 按存储字段原样渲染天然支持新协议（零转换）；SR 节点链接按注册表映射转换，无映射的协议跳过并提示（同 4.5 口径）；新增协议仅需扩展注册表，无 schema 迁移
- 协议范围：manual 来源支持 ClashOfficial 全量代理协议（见上表，ssr 除外，见 4.5）；**xray 来源支持 vless / vmess / trojan / shadowsocks 四协议**（Xray UserManager 用户增删 API 的源码能力边界，决策 #20 修订）；检测到的其他协议 inbound（无 per-user 能力）以 **nodes.allocatable=0** 标记不可分配并在 UI 提示（见 5.9）

### 3.3 代理组（Clash）

- **三类强制组**（系统强制勾选，不可移除）：
  - **直接连接**：内含内置 DIRECT
  - **国外流量**
  - **无法归属的流量**：兜底组（见 3.6 兜底规则）
- **预设组库**：内置参考 `Clash.yaml.template.md` 个人配置的可选组（YouTube / Netflix / 哔哩哔哩 / 国外流媒体 / 苹果海外服务 / 苹果国内服务 / AI / Steam / Steam下载 等），管理员勾选启用，作为更丰富的细节化配置
- **自建组**：支持管理员自定义新建代理组
- **组类型（Clash proxy-group 类型）**：组定义含类型字段，枚举 **select / url-test / fallback**（其他类型如 load-balance 按需 Build 期扩展）；自建组创建时选择类型（默认 select），预设库组类型随作者配置参考内置（如 YouTube=url-test）；渲染时 proxy-group 按定义类型输出
- **持久化**：代理组定义存全局 `proxy_groups` 表（见 5.9），与装配快照分离；预设组库以种子数据随迁移内置（参考 `Clash.yaml.template.md`），管理员自建组入库同表；装配快照仅记录组的勾选引用
- **组内容约束**：每个代理组须至少包含节点、「直接连接」组、「国外流量」组三者之一；管理员可勾选此前添加过的节点；各组可再包含「直接连接」「国外流量」组作为可切换项（同个人配置形态）
- **组内节点顺序**：组定义（definition_json）的节点引用列表**有序**，组编辑页支持拖拽调整顺序（如 US1 调至 US2 之前）；Clash 渲染时 proxy-group 按定义顺序输出节点——**select 类组的第一个节点即默认选中节点**
- **空组硬校验**：Clash YAML 装配生成时，强制组（尤其「国外流量」）须至少含 1 个节点，否则**拒绝生成**并提示具体缺节点的组（空 select 组会导致 Clash/mihomo 加载失败）；见 4.1 空产物硬校验

### 3.4 Shadowrocket 双装配器

Shadowrocket 的节点与分流规则是两份独立内容（见 3.1），故提供两个可单独使用的装配器，不复用 Clash 代理组，保证各端语法原生、交互最简：

- **节点订阅装配器**：勾选节点（manual / xray 来源）+ 填写订阅头部（STATUS / REMARKS，见 4.2），产出 subs 内容（头部行 + 逐行节点链接，base64 编码，见 4.5）；**不含规则**
- **分流规则装配器**：填写 [General] 表单 + 勾选规则素材（**素材池池级勾选**，与 3.5 口径一致；手工规则行逐条补充）+ **选择兜底流量方向**（FINAL,DIRECT / FINAL,PROXY 二选一，默认 PROXY，见 3.6），每个素材池仅需勾选**代理或不代理**，池内条目整体继承目标，渲染为 `PROXY` / `DIRECT`，产出 .conf（[General] + [Rule] + 兜底规则，见 3.6）；**不含节点**；装配时**选择目标规则实体**（管理员预先在规则页手动新建，如「Shadowrocket 分流规则」；**允许创建无版本的空规则实体**——放宽 Design1「创建必带首版」校验，审核 D8 决策），生成的 conf 作为该实体的新版本入池（不自动生效，见 4.4；空实体首个 conf 自动激活除外）
- 两装配器各自生成各自版本（subs 入订阅地址池、conf 入规则实体，见 4.4），不强制同时操作

### 3.5 规则拼接规则

- 管理员勾选规则素材池（可另加手工补充规则行）并指定目标后，系统逐条拼接规则行：`规则类型,匹配值,目标`
  - Clash 的目标为管理员指定的代理组名；Shadowrocket 的目标为 PROXY / DIRECT
  - 例：勾选「苹果域名」池指向「苹果国内服务」组 → `- DOMAIN-SUFFIX,aaplimg.com,🍏苹果国内服务`（`full:` 前缀解析的条目渲染为 `DOMAIN,...`）
- **规则类型支持**：DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / IP-CIDR / IP-CIDR6 / PROCESS-NAME / PROCESS-NAME-REGEX；**USER-AGENT 为 Shadowrocket 专属类型**（如 `USER-AGENT,AppleNews*,PROXY`）；**Clash YAML 装配时跳过 USER-AGENT 条目并在预览/生成结果中列出提示**（Clash 不支持该类型，与 4.5 不可转协议跳过提示同口径）
- **IP 规则**：IP-CIDR / IP-CIDR6 规则行一律附加 `no-resolve`
- DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD 两端逻辑通用，语法差异由渲染层映射，管理员只维护一套勾选
- 输出始终为纯文本；预览遵循 Design1.md 3.4.1「纯文本/代码视图渲染，禁止 HTML」；控制字符按 Design1.md 6.3 输入安全口径处理

### 3.6 兜底规则

兜底规则自动追加于规则列表末尾：

| 语法 | 兜底内容 |
|------|---------|
| Clash | 固定：`GEOIP,CN,直接连接` + `MATCH,无法归属的流量`（管理员无需操作） |
| Shadowrocket | `GEOIP,CN,DIRECT` 固定；**`FINAL` 方向由管理员手选**（分流规则装配器表单，DIRECT / PROXY 二选一，**默认 PROXY**——与既有行为和作者样例一致，兜底走代理防漏直连）；选择结果随装配快照保存 |

---

## 四、配置生成与分发（基础模式）

### 4.1 生成流程

1. 选择装配类型（语法与产物形态随之确定）与**目标平台**（平台 product_type 须与装配类型匹配：Clash YAML→clash-yaml、SR 节点订阅→sr-subs、SR 分流规则→目标规则实体；同平台两种格式时建两个平台，见 4.4）：**Clash YAML**（Clash 平台，单文件含节点+代理组+规则）/ **SR 节点订阅**（subs）/ **SR 分流规则**（conf，见 3.4 双装配器）
2. 以类 form 表单填写头部（见 4.2：Clash 顶层键 / SR conf 的 [General] 键值 / SR subs 的 STATUS 与 REMARKS），提供预填写默认值，可一键采用
3. Clash YAML：勾选节点（manual / xray 来源，见 3.2）并配置代理组（见 3.3）；SR 节点订阅：勾选节点；SR 分流规则：跳过本步（conf 不含节点）
4. Clash YAML 与 SR 分流规则：勾选规则素材池并指定目标（见 2.5），可手工补充规则行；SR 节点订阅：跳过本步（subs 不含规则）
5. 预览：全文纯文本渲染，可与当前激活版本做 diff 对比（文本差异高亮，前端实现，**采用 jsdiff**（npm diff 包：轻量行级 diff，前端自渲染三色高亮；选型已定），避免引入 monaco 等重型编辑器，防止重现 Issue1 的 manualChunks 循环依赖问题）；SR subs 预览直接显示明文原文（模板按明文存储，见 5.7）；**空产物硬校验**（拒绝生成）：Clash YAML 装配强制组（尤其「国外流量」）至少含 1 个节点，SR 节点订阅至少勾选 1 个节点，否则拒绝生成并提示具体原因（空 select 组/空节点列表会导致客户端加载失败，见 3.3）；规则为空允许生成（见 3.6 兜底规则仍有效）并在预览中提示
6. 确认生成 → 事务创建对应实体的新版本（写渲染产物文件 + 保存生成参数快照 + 更新版本列表）；**新版本仅入池不自动生效**（subs/YAML 入订阅地址池、conf 入规则实体），由管理员显式分发（见 4.4 首次入池自动激活例外条款）

### 4.2 配置头部表单与默认值

- **Clash**：顶层键 port / socks-port / mixed-port / allow-lan / mode / geox-url / geo-auto-update / log-level / ipv6 / ntp / dns（含 fallback-filter / fake-ip-filter）等，字段范围参考 `ClashOfficial.yaml.template.md`；**默认值以作者个人配置（`Clash.yaml.template.md` 头部）预填写**
- **SR 分流规则**：`[General]` 段键值（bypass-system / skip-proxy / tun-excluded-routes / dns-server / fallback-dns-server / ipv6 等），默认值参考 `Shadowrocket.conf.template.md`；另含**兜底流量方向**二选一（默认 PROXY，见 3.6）
- **SR 节点订阅**：头部两行——STATUS（自动生成建议格式：日期 + 版本标识，管理员可改）与 REMARKS（订阅显示名，管理员填写，默认取站点名）；预填默认值可一键采用（见 `Shadowrocket.subs.template.md` 样例）；基础模式与高级模式的头部表单一致（占位注入在下载时进行，见 4.3）
- `[Host]` / `[URL Rewrite]` 不纳入装配范围，如需可在表单扩展区自行补充

### 4.3 节点输出形态

- **manual 节点**：静态渲染——Clash YAML 内联 proxies 条目（原样字段）；SR 节点订阅按 4.5 链接映射转为节点链接行；不可转协议跳过并提示
- **xray 节点**：节点区输出占位标记 `# {{xray_nodes}}`（注释行，保证模板可独立预览/校验）；下载时系统按「用户所属组分配的节点 ∪ 公共节点 + 用户凭据（UUID / 代理密码）」替换为实际节点行——**SR subs 走占位文本替换；装配生成的 Clash 模板按 5.7 蓝图全量重渲染（非文本替换）**；无占位标记的模板原样返回（**适用于直接上传模板**）
- 管理员预览显示模板原文（含占位标记）；用户预览按自身渲染

### 4.4 订阅地址池衔接、分发与重新编辑

- 生成产物按**平台**归属：**Clash YAML 与 SR 节点订阅（subs）入订阅地址池**（**每平台仅一份订阅模板，UNIQUE(platform_id)**；模板携带 product_type 属性（yaml/subs），用于创建/上传时的格式校验、默认下载扩展名与 UI 展示——同平台需要两种格式时建两个平台，审核 A1 决策）+ 版本历史，版本管理统一适用 Design1.md 4.1（5 版本上限、`BEGIN IMMEDIATE` 事务、不可删当前激活版本等）；**SR 分流规则（conf）入现有「规则」实体**（规则的版本管理、激活与规则 Token 下载机制沿用 Design1.md，装配生成的 conf 作为该规则的新版本，不自动生效、需显式切换；首次入池自动激活规则除外，见下）
- **分发机制（入池 + 显式分发）**：生成与上传均只作为新版本入池，不自动生效；管理员在订阅地址池页面（或规则页面）显式「激活/分发」某版本后，该版本成为当前激活版本并对全体用户生效（基础模式全体用户获得同一份订阅；用户级例外仍由自定义订阅承载，见第一章）
- **版本组件改造**：`CreateVersion` 增加 `activate` opt-in 参数——**订阅地址池（Clash YAML / SR subs）与装配产物（含 SR conf 入规则版本）的生成/上传调用一律传 false**，不再沿用现有「事务内强制切换激活指针」行为；激活动作仅由订阅地址池页面（或规则页面）的「激活/分发」触发（复用 `SwitchVersion`）；**适用范围边界：规则/分享订阅/自定义订阅的手工上传与在线编辑保持 Design1 既有「创建即激活」行为不变**（传 true），避免四类资源行为剧变
- **首次入池自动激活**：**按订阅行判定**——该订阅行（或规则实体）`current_version=0` 时，首个入池版本（无论生成或上传）自动成为激活版本，避免新部署后用户下载无可用版本的空窗（不按「平台」判定，与每平台一份模板模型配套，审核 A1 决策）；**规则实体同规则适用**：目标规则实体尚无激活版本时，首个装配 conf 版本自动激活（避免新规则实体「装配完但规则 Token 无激活版本」的空窗）；后续版本均须显式分发/切换
- **空池下载口径**：平台（或规则实体）尚无任何版本/无激活版本时，下载端点按 AGENTS §4.8 返回 HTTP 200 + 纯文本注释块（如 `# error: no active version`），不返回 JSON/HTML
- 生成参数（头部表单值 + 节点/代理组/素材池勾选（SR 分流规则为池级勾选）+ 手工规则行）随版本快照保存（`assembly_blueprints` 表，version_id 1:1；SR 分流规则的快照随规则版本同表存储）；**Clash YAML 装配产物另存结构化渲染计划**（头部 / manual proxies / proxy-groups 结构 / rules / 兜底规则）至 `assembly_blueprints.render_plan_json` 列（见 5.9，审核 F1-1），供下载时蓝图全量重渲染使用（见 5.7，审核 A2 决策）
- **重新编辑**：生成过的版本提供重新编辑入口，加载快照修改后生成**新版本**（不改写旧版本；新版本仅入池，需再次显式分发）；**快照悬空容错**：加载快照时逐项校验引用（proxy_groups / 素材池 / 节点），失效项标记并提示管理员剔除或替换后再生成；实体删除不阻断（快照为历史参考，不反向约束实体生命周期；下载侧已有「候选集之外不注入」兜底，见 5.7）
- 下载/日志/限流：Clash YAML 与 SR subs 复用现有订阅下载端点、访问日志与限流；SR conf 走规则下载端点（规则 Token）
- **SR conf 用户分发（用户端规则卡片 + 引导）**：SR 平台用户在首页/个人中心除订阅卡片外，另见「分流规则」卡片（复用现有规则卡片机制）：展示当前激活 conf 版本信息与规则 Token 复制链接（用户自行粘贴导入 SR；不使用一键 scheme 唤起，与 Design1 3.5「移除一键导入 UI」口径一致，审核 C6 订正），并提供 SR 双内容导入引导文案（先添加订阅获取节点、再导入分流规则）；**卡片仅做入口**——点击跳转现有 /rules 列表页（全部 Shadowrocket 规则，不引入规则平台归属模型，审核 F4-4）

### 4.5 Shadowrocket 输出编码

SR 双产物的编码方式不同：

- **节点订阅（subs）**：明文内容为头部行（STATUS/REMARKS）+ 逐行节点链接，**模板文件存储解码后明文、下载注入完成后整体 base64 编码下发**（存储与下发区分，见 5.7；样例见 `Shadowrocket.subs.template.md`）；节点链接按 **SR 原生参数风格**渲染，映射规则与生态验证结论见 [Reference/Node-Link-Standards.md](./docs/Reference/Node-Link-Standards.md)（参照 urlclash-converter 转换逻辑取证）：
  - **manual 节点**：按协议转为对应链接——ss：`ss://base64(cipher:password)@server:port#name`（SIP002）；**vless：base64 userinfo 形态（与样例一致）**：`vless://base64(cipher:uuid@server:port)?remarks={节点名}&tls=1&peer={SNI}`，REALITY 附加 `xtls=2&pbk=公钥&sid=short-id`（SR 原生 REALITY 参数；**cipher 用空占位（`:uuid@...`，对齐样例实测形态）**）；**vmess：V2rayN JSON 格式**：`vmess://base64(JSON)`（v/ps/add/port/id/aid/scy/net/type/host/path/tls/sni/alpn/fp）+ `?remarks=&udp=1&alterId=`（生态兼容性最广，SR/Clash/V2rayN 均识别）；trojan / hysteria / hysteria2 / tuic / wireguard / http / socks5 等同理映射（细则见 Node-Link-Standards.md 第二章）；**无法转为链接的协议（snell / mieru / masque / openvpn / ssh / shadowquic / trusttunnel / tailscale 等）跳过，在生成结果中列出跳过节点与原因**
  - **xray 节点**：注入时按上述同形态渲染（vless REALITY：base64 userinfo + tls=1/xtls=2/peer/pbk/sid；vmess：V2rayN JSON + remarks/udp/alterId；**trojan：`trojan://{用户代理密码}@host:port?remarks=...`；ss：`ss://base64(cipher:{用户代理密码})@host:port#...`**，凭据模型见 5.5），与 Clash YAML 端的渲染（见 5.7）为同一节点数据的两种客户端表达
  - 节点名（remarks / #fragment）经 URL 编码，支持中文与 emoji；域名非 ASCII 字符转 Punycode；**参数值避免空格**（URLSearchParams 的 `+` 编码与部分解析器不对称，见 Node-Link-Standards.md 第四章）
- **分流规则（conf）**：.conf 正文以纯文本下发（无特殊编码）
- **ssr 协议不纳入**：manual 节点协议范围不含 ssr——生态无可靠的 SSR 链接生成参照（urlclash-converter 对 SSR 只收不生成、静默丢弃），自研编码无验证基准，故移除（变更记录 2026-08-15 研究整合项）
- 两类下载端点均返回禁缓存头（`no-store` 等，AGENTS §4.5）

---

## 五、Xray-core 对接（高级模式）

> 本章全部能力归属**高级模式**（见第一章）。API 能力、硬约束与生态研究结论见 [Reference/Xray-Core-API.md](./docs/Reference/Xray-Core-API.md)。设计已定稿（决策见 5.4，共 24 项），无待决事项。

### 5.1 背景与目标

本系统为订阅管理系统（管理 Clash YAML / Shadowrocket .conf 客户端配置并通过 Token 分发）。新增目标：**对接自托管的 Xray-core 服务端，实现用户生命周期自动同步、流量配额管控与每用户专属订阅内容生成**。

核心能力目标：

1. 面板新用户（注册/审批/管理员创建）激活后**自动推送账号到 Xray**（所属组分配的节点 ∪ 公共节点）
2. 用户禁用/删除时**自动从 Xray 移除**
3. **流量配额**管控（自然月按真实日历累计；超限仅移除 Xray 账号；管理员手动重置）
4. **每用户专属订阅**：用户下载的配置 = 平台全局模板 + （组分配节点 ∪ 公共节点）的节点行（含该用户凭据：UUID / 代理密码，动态注入，见 5.7）

### 5.2 Xray-core API 能力与硬约束（设计相关要点）

> 完整研究结论（API 服务机制、全量 API 能力表、传输字段、幂等性核验、生态验证）见 [Reference/Xray-Core-API.md](./docs/Reference/Xray-Core-API.md)。影响本设计的关键要点：

- API **无认证无 TLS**（裸 gRPC），安全边界由部署者 IP 白名单控制；**并发保守串行化**（官方文档提示约 10 并发后丢弃请求；v26.7.28 源码未见显式限制实现，见 Xray-Core-API.md §11.4，保守起见客户端串行化）
- 流量统计需 Xray 配置显式开启（`policy.statsUserUplink/Downlink`），未开则查询恒为空
- 流量 counter 为内存态，**重启清零**——必须短周期采集差值落库
- 官方无带宽限速/无流量配额/无到期时间——**配额必须面板侧实现**（见 5.8）
- 用户增删以 **email 为键**，幂等同步须容忍「已存在/不存在」特定错误（见 5.5）；错误码均为 codes.Unknown，只能靠错误字符串子串匹配（见 Xray-Core-API.md §11.1）；vmess Account 的 alter_id 字段在新版 Xray 已移除，推送仅需 id（见 Xray-Core-API.md §11.3）
- `GetUsersStats` **仅覆盖在线用户**，全量流量采集必须逐用户 `QueryStats`（见 5.8）
- 可内嵌为 Go 库但依赖过重，**不采用**；选独立实例 + gRPC 远程管理（路径 A，见 5.3）

### 5.3 架构选型：路径 A

```
┌─────────────────────────┐        gRPC（公网/内网，IP 白名单防护）         ┌─────────────────────────┐
│ vpn-sub（管理面）        │ ◄──────────────────────────────────────────► │ Xray-core（独立进程）     │
│  - 用户/组/配额/订阅模板  │    internal/xray 客户端封装                    │  - 独立服务器（1-5 台）    │
│  - 下载动态装配（节点注入）│    （串行调用 + 流量持久化 + 幂等同步）          │  - api.listen 对外开放     │
└─────────────────────────┘                                               │  - policy.stats 开启      │
                                                                          └─────────────────────────┘
```

- Xray 已在**另一台服务器**运行（1-5 台实例范围），API 安全由**部署者 IP 白名单**控制
- 本系统作为 gRPC 管理面：用户推送/移除、流量采集、配额检查
- 依赖增量：**引入 `github.com/xtls/xray-core` 模块**，直接使用其 command 包 proto 生成代码（HandlerService/StatsService 等）；go.mod/go.sum 依赖膨胀可接受（编译仅用部分包，不内嵌 Xray 运行时）；**项目 Go 版本升级至 1.26**（xray-core v26 系列要求 go 1.26，原「不动 Go 版本」口径经核验不可行，审核 B1 决策修正；AGENTS.md 已同步更新 Go 版本表述，Dockerfile 基础镜像同步升级 golang:1.26-alpine 与构建核验）

### 5.4 已确认决策（构建时不得偏离）

| # | 决策点 | 结论 |
|---|--------|------|
| 1 | 带宽限速 | **不做**（官方 API 不支持）；只做**流量配额**（面板侧软限） |
| 2 | 订阅内容形态 | **每用户专属订阅**：下载 = 平台全局模板 + （组分配节点 ∪ 公共节点）节点行（含用户凭据：UUID / 代理密码）动态装配 |
| 3 | 超限动作 | **仅移除 Xray 账号**（面板用户保留，可登录/下载/查用量） |
| 4 | 配额恢复 | 超限后由**管理员手动重置**（清当月累计 + 重新推送，凭据不变：UUID 与代理密码） |
| 5 | 存量用户 | **Xray 侧存量账号不考虑**（不做全量同步/reconcile 既有 Xray 账号）；**面板侧存量用户由开关 ON 批量初始化覆盖**（生成 UUID + 全量推送，见第一章） |
| 6 | 配额周期 | **自然月，跟随真实日历**（按月聚合，无需定时重置表） |
| 7 | 用户侧展示 | 展示**已用流量 / 总分配流量**（首页平台卡片 + 个人中心） |
| 8 | 管理员侧 | 复用现有用户管理面板（UsersView 扩展：用量/同步状态/覆盖配额/重置），**不新增独立用户数据页** |
| 9 | 推送粒度 | **参考用户组模式**：组分配节点（节点级），用户激活向所属组分配的全部节点 ∪ 公共节点推送（见 5.5 推送集合口径） |
| 10 | 用户组改造 | 组分配从「每平台选定订阅」**改为「用户可用的 Xray 节点 + 用户默认流量配额」**；订阅地址池简化为**每平台一份订阅模板**（见 4.4；UNIQUE(platform_id)，产物类型为模板属性，审核 A1 决策） |
| 11 | 模板生成 | 配置文档（YAML 等）由**订阅装配功能动态生成**（见第三/四章），下载时自动按组节点注入 |
| 12 | 配额粒度 | **组默认配额 + 用户可覆盖**（users.quota_override 可空，NULL=继承组） |
| 13 | 换组处理 | **自动迁移**：换组事务提交后，旧组节点移除该用户、新组节点推送（同凭据：UUID 与代理密码不变） |
| 14 | 实例范围 | 1-5 台 Xray 实例，多实例独立连接/采集/推送，单实例失败隔离 |
| 15 | **注入判定** | **有凭据即注入**：用户首次激活生成凭据（UUID 与代理密码）后，无论推送状态（pending/failed），下载均注入节点行；管理员重试成功后立即生效，无需用户重新下载 |
| 16 | **UUID 存储** | **users 表新增 uuid_encrypted 列**（每用户一个 UUID，跨节点/实例共用，AES-256-GCM 加密复用签名密钥派生机制）；xray_users 仅记录推送状态（不再存 UUID） |
| 17 | **超限渲染** | **下载内容不更改任何参数**：超限用户订阅照常注入节点行；仅在用户面板提示「流量已超限」 |
| 18 | **节点停用** | **渲染过滤 + 分配保留**：渲染只注入 enabled=1 节点；组分配记录保留，重新启用即恢复注入 |
| 19 | **组删除联动** | **自动迁移**：组删除后组内用户迁默认组，Xray 账号自动迁移（旧组节点移除 + 默认组节点推送，同凭据：UUID 与代理密码不变），与换组语义一致 |
| 20 | **协议范围** | **xray 来源支持 vless / vmess / trojan / shadowsocks 四协议**（Xray UserManager 用户增删 API 的源码能力边界；经用户决策放宽，原「仅 VLESS/VMESS」口径废弃）：**每用户凭据注入**——vless/vmess = 每用户 UUID（users.uuid_encrypted），trojan/ss = 每用户统一代理密码（users.proxy_secret_encrypted，见 5.5）；节点行渲染字段集（**Clash proxies 条目形态**）：vless（encryption/flow/security/sni/fp/pbk/sid/type/network/path/host）、vmess（Clash 原生字段：uuid/alterId/cipher/udp/tls/network/ws-opts 等）、trojan（password=用户代理密码）、ss（cipher 随 inbound 检测 + password=用户代理密码）；**SR 链接形态见 4.5**（vless 为 base64 userinfo、vmess 为 V2rayN JSON base64）；检测到的其他协议 inbound 无 per-user 能力，以 allocatable=0 标记不可分配并提示；协议类型整体可扩展注册（见 3.2） |
| 21 | **公共节点** | **增加 is_public 标记**：nodes.is_public=1 的节点对所有组自动可见（下载渲染 = 组分配节点 ∪ 公共节点），组分配 UI 标注公共节点无需分配；**公共 xray 节点自动进入每个 active 用户的 AddUser 集合**（与组分配节点同等参与 ON 批量初始化/注册审批启用/换组 diff/删除移除等全部触发器，仍受候选集与 enabled 过滤，审核 A6 决策）；**公共节点集合变化纳入激活 diff 与节点编辑触发器**：公共节点退出已激活蓝图候选集或被取消 is_public 时，对全部 active 用户 RemoveUser；新增/恢复 is_public 同理 AddUser（审核 F3-2） |
| 22 | **节点顺序** | **按组分配顺序输出**（group_nodes 记录顺序，UI 支持排序调整）；公共节点排于组分配节点之后 |
| 23 | **用量响应头** | 采纳 **Subscription-Userinfo 标准头**（XTLS 官方事实标准，见 docs/Reference/Xray-Core-API.md）：下载响应注入 `upload=/download=/total=/expire=` 四字段，expire 输出**远未来时间戳 4102444800（2100-01-01）表达无到期**（不用 0，避免部分客户端按 Unix 时间戳解释为「已过期」）；与平台附加头机制融合；**增配两个约定头**：`profile-update-interval`（建议值 6 小时，客户端自动更新间隔）与 `profile-web-page-url`（站点 URL）（SSPanel 实现核验仅 clash 格式携带，本系统对**用户订阅类下载（无标识组解析 + 自定义订阅）**携带；分享/规则下载不携带，见 5.7；v2rayNG/Clash Meta/SingBox 等客户端支持，见 SSpanel-Subscribe.md）；**同名冲突优先级：高级模式系统注入头覆盖平台附加头同名键**（或保存平台附加头时拒绝同名键），高级模式 profile-update-interval 定 21600（6 小时）、基础模式沿用平台值（审核 D11/F3-5 决策） |
| 24 | **配置导入导出** | **xray_instances 数据纳入配置导入导出**：export.go 扩展导出实例连接数据，导出格式升级 format_version=2；**导入语义：整体覆盖 xray_instances 表**——导入时先按导出内容重建实例行（避免 xray_users 外键级联误删），xray_users / traffic_records 等业务数据不触碰，导入完成提示执行节点检测刷新；format_version=2 payload 新增 instances 数组字段（见 5.9，审核 F3-6） |

### 5.5 用户生命周期同步

**同步触发器**（全部在业务层、DB 事务提交后执行，失败记日志不阻断主流程——与现有「欢迎邮件」模式一致）：

| 触发点 | 现有代码位置 | Xray 动作 |
|--------|-------------|----------|
| 自注册免审批（status=active） | `user.Service.Register` | 向所属组全部节点 ∪ 公共节点 `AddUser`（见下方推送集合口径） |
| OIDC 直接激活（审批开关关闭/命中白名单） | `user.Service.CreateFromOidc`（pending=false 分支） | 向所属组全部节点 ∪ 公共节点 `AddUser`（见下方推送集合口径） |
| 审批通过（pending→active） | `approval.Service.Approve` | 同上 |
| 管理员创建用户（直接 active） | `user.AdminService.Create` | 同上 |
| 管理员启用（disabled→active） | `user.AdminService.SetStatus(false)` | 同上 |
| 管理员禁用（active→disabled） | `user.AdminService.SetStatus(true)` | 所属组全部节点 ∪ 公共节点 `RemoveUser` |
| 删除用户 / 审批拒绝 | `user.AdminService.Delete` / `approval.Reject` | 所属组全部节点 ∪ 公共节点 `RemoveUser`（幂等容忍不存在） |
| 换组 | `user.AdminService.UpdateGroup` | 按 diff 迁移：旧集合（旧组分配 ∪ 公共节点）− 新集合（新组分配 ∪ 公共节点）执行 RemoveUser/AddUser（同凭据：UUID 与代理密码不变） |
| 组删除 | `group.Service.Delete` | 组内用户迁默认组后自动迁移：按换组同款 diff 口径（旧组节点移除 + 默认组节点推送，决策 #19） |
| 组节点分配变更 | `group` 服务 | 受影响组内 active 用户 diff 推送/移除（按推送集合口径：组分配 ∪ 公共节点） |
| 节点 enabled 切换 | `nodes` 服务（节点编辑） | enabled 0→1：受影响组内 active 用户 AddUser diff；1→0：RemoveUser diff（审核 F3-3） |
| 公共节点 is_public 变化 | `nodes` 服务（节点编辑） | 对全部 active 用户 AddUser/RemoveUser diff（审核 F3-2） |
| 角色变更 admin⇄user | `user.AdminService.ChangeRole` | 无操作（代理账号与面板角色无关） |

**账号映射规则**：

- Xray email：`user-{id}@vpn.local`（与面板邮箱解耦，改邮箱不影响映射；全小写——Xray vless 侧对 email 先 ToLower，面板侧必须规范化全小写否则流量 counter 名对不上，见 Xray-Core-API.md §11.1）
- UUID：用户首次激活时生成（crypto/rand v4），**AES-256-GCM 加密后存 users.uuid_encrypted**（每用户一个 UUID，跨节点/实例共用，复用 `config` 服务的签名密钥派生机制；决策 #16）
- **代理密码**（trojan/ss 每用户凭据）：用户首次激活时与 UUID 同步生成（crypto/rand 高熵随机串），**AES-256-GCM 加密后存 users.proxy_secret_encrypted**（每用户一个统一密码，跨密码类协议共用，跨节点/实例共用；同 UUID 机制）；开关 OFF 清空 / ON 重新生成（见第一章）
- **Account 形态（按协议）**：AddUser 的 Account 按节点协议构造——vless：`vless.Account{Id: UUID}`；vmess：`vmess.Account{Id: UUID}`；trojan：`trojan.Account{Password: 代理密码}`；shadowsocks：`shadowsocks.Account{Password: 代理密码, Cipher: 节点 cipher}`（决策 #20）
- 推送状态：`xray_users.sync_status = pending/synced/failed`，失败记 `last_error`，管理面板可见、可手动重试
- **超限标记**：超限状态存 `users.quota_exceeded`（布尔，见 5.9），超限置 1、管理员重置配额时复位；用于面板超限提示（决策 #17）与「上月超限本月不自动恢复」判定（5.8）
- **超限前置拦截**：所有 AddUser 类钩子（注册/审批/管理员创建/启用/换组/组删除迁移/组节点分配变更 diff 推送/**节点 enabled 切换 diff 推送/公共节点 is_public 变化 diff 推送**/开关批量初始化，**含实例级对账补推**）执行前统一检查 `quota_exceeded`：**超限用户不推送**（xray_users 记录保持，推送跳过并记原因），UI 提示先重置配额；重置配额时恢复推送——防止换组/启用/对账补推等操作绕过配额管控（审核 G2/C-新16/F3-2/F3-3 收敛）
- **推送集合口径（审核 A6 决策）**：全部 AddUser/RemoveUser 类触发器的目标节点 = 「所属组分配的全部 xray 节点 ∪ 公共 xray 节点（is_public=1）」，仍受候选集与 enabled=1 过滤
- **凭据并发首建守卫（审核 B2/F5 口径）**：UUID 与代理密码首建参照 Token 首建模式——`BEGIN IMMEDIATE` 事务内按**同一 WHERE 条件**（`... WHERE id=? AND uuid_encrypted IS NULL AND proxy_secret_encrypted IS NULL`）条件更新两字段（同事务同生同灭，审核 F3-7），按 RowsAffected 判定，生成与加密在事务内完成；**适用范围：全部 AddUser 类钩子统一前置**（注册/审批/管理员创建/启用/换组/组删除迁移/节点与公共节点变更 diff/开关 ON 批量初始化/对账补推）——无凭据用户先建凭据再推送（审核 D4 决策），防止 ON 批量初始化与注册/审批钩子并发命中同一用户
- **高级开关检查（审核 B3/F3-4 口径）**：全部 Xray 同步钩子入口先**实时查 DB** 高级模式标记（advanced_mode 配置键，每次查库不缓存），OFF 时静默跳过；OFF 清空事务内置位后提交，钩子在事务提交后执行时读到的即最新标记，防清空与钩子并发交错补推（见第一章并发口径）
- **级联**：用户删除时 `xray_users` / `traffic_records` 随外键 ON DELETE CASCADE 清理，无孤儿数据（AGENTS §4.7）；**删除用户/审批拒绝路径**：RemoveUser 所需的节点信息（instance_id / inbound_tag）在删除事务提交前收集、提交后执行（同 OFF 清空 N1 收集模式，审核 P3-11）

### 5.6 用户组模型（组 = 节点授权 + 配额）

```
groups（+ default_quota 默认月度配额 GB）
  └── group_nodes（组 ↔ 节点多对多分配：group_id, node_id, sort_order）
        ← 替代原 group_selections（组选定订阅）与 subscription_group_rel
```

- **节点分配**：组可勾选多个节点（**仅限 xray 来源**：manual 节点为共享静态凭据、无 per-user 语义，仅通过装配勾选静态渲染进模板，不参与组级差异化分配与下载注入，见 5.7）；用户激活/换组时按此推送；组内记录保持**排序字段**（下载渲染按此顺序输出，UI 支持排序调整，决策 #22）；**组管理页对不属于当前所有已激活蓝图候选集并集的分配节点做标注提示**（参考 Design1 needs_reselect 高亮模式，避免「分配了但下载没有」的静默过滤困惑；仅部分模板候选的节点另行提示，见下方候选集口径 ④）
- **候选集约束（见 3.2）**：组的节点分配只能在装配器勾选的候选集内选择（装配器定范围、组定授权）；装配器取消勾选某候选节点时，各组该节点的分配记录级联清理并在 UI 提示（AGENTS §4.7 无悬空引用）；**清理时机：级联清理仅在新版本被显式激活/分发时执行**——生成入池未激活期间不动组分配，避免旧激活版本（旧快照）的注入结果被未生效的新版本追溯改变；未激活版本不产生副作用（候选集之外的节点本就不注入，见 5.7）；单组场景下装配器勾选与组分配自然重合；**候选集口径与激活回收（审核 A7/F3-2 决策）**：①组管理页候选集 = **当前所有已激活装配蓝图的 xray 候选节点并集**；②下载渲染仍按各模板自身蓝图过滤；③新版本激活时比较新旧蓝图候选集差集（**含公共 xray 节点集合变化**），删除 group_nodes 后**对受影响 active 用户执行 RemoveUser diff**（事务提交后，幂等失败记同步状态，防授权残留；公共节点退出候选集/取消 is_public 时对全部 active 用户 RemoveUser，新增/恢复同理 AddUser）；④组管理页对「仅部分模板候选」的节点做提示；⑤组管理页候选集并集为空时提示「请先装配并激活 Clash 模板」
- **公共节点（决策 #21）**：nodes.is_public=1 的节点**对所有组自动可见**（无需分配，渲染时排于组分配节点之后）；组分配 UI 标注公共节点无需分配
- **默认配额**：组内用户默认月度流量（GB）；用户级可覆盖（users.quota_override）
- **级联**：组删除 → 节点分配级联删；组内用户迁默认组（现有逻辑保留），其 Xray 账号随新组自动迁移（旧组节点移除 + 默认组节点推送，决策 #19）
- **删除项**：`group_selections` / `subscription_group_rel` 表删除；`needs_reselect` 标记机制随订阅选定消失
- **换组即时生效**：与现有「换组 Token 实时解析跟随」语义一致（Token 无需清理，下载解析实时跟随新组节点）

### 5.7 下载渲染机制（每用户专属订阅）

```
用户下载 → Token 校验（现有链路）→ 定位平台全局模板（当前版本文件；SR 平台为 subs 模板）
  → 读用户所属组 → 组分配的节点列表（按分配顺序，决策 #22）∪ 公共节点（is_public=1，排后）
  → 生成节点行（每个节点一行：name={实例slug}-{入站tag}，按协议与目标语法渲染字段；跳过 enabled=0）：
    · Clash YAML 模板 → proxies 条目（按四协议：vless encryption/flow/security/sni/fp/pbk/sid/…；vmess uuid/network/tls/…；trojan password=用户代理密码；ss cipher（节点检测）+password=用户代理密码，见决策 #20）
    · SR subs 模板 → SR 原生节点链接（vless：`vless://base64(cipher:{用户UUID}@host:port)?remarks={节点名}&tls=1&peer={SNI}&xtls=2&pbk={公钥}&sid={short-id}`（cipher 用空占位，见 4.5）；vmess：V2rayN JSON 形态；trojan：`trojan://{用户代理密码}@host:port?...`；ss：`ss://base64(cipher:{用户代理密码})@host:port#...`，见 4.5）
  → 替换模板中的节点占位标记 `# {{xray_nodes}}` → no-store 返回（SR subs 为替换后全文 base64）
```

关键规则：

- **模板** = 装配生成的**平台全局模板**（含 `# {{xray_nodes}}` 占位标记，见 4.3；无标记的模板原样返回，兼容纯规则模板与基础模式静态节点模板）；**SR 平台的模板为 subs**：**模板文件存储解码后明文**（上传的可解码 base64 模板解码为明文存储，不可解码视为明文原样存储；下载注入完成后全文重新 base64 返回，避免每次解码歧义，审核 B6 口径），占位标记位于明文中；**默认下载文件名按产物类型区分**（clash-yaml → `.yaml`、sr-subs → `.txt`、sr-conf → `.conf`，上传模板保留原始扩展名）；SR conf 不含节点、无占位
- **节点名约定**：`{实例slug}-{入站tag}`，与装配器节点命名约定闭环（见 3.2）
- **注入范围（候选集约束，见 5.6）**：注入节点 = 组分配节点 ∪ 公共节点；装配生成的模板两者均须属于当前激活模板的装配候选集（装配器勾选快照，见 4.4），候选集之外的节点不注入；**无装配快照的直接上传模板不受候选集约束**，按「组分配节点 ∪ 公共节点」注入
- **proxy-groups 蓝图全量重渲染（审核 A2 决策）**：装配生成的 Clash YAML 版本同时保存**结构化渲染计划**于 blueprint（见 4.4），用户下载时按蓝图**全量重渲染**（而非仅文本替换）：①注入节点 = 组分配节点 ∪ 公共节点（过滤 enabled=0 与候选集之外）；②所有 proxy-groups 按「可达注入节点」递归重建（剔除成员中不含任何已注入节点、且不含子组间接可达已注入节点的组，manual 静态节点计入已注入集合防纯 manual 组误剔除）；③**强制组在注入集为空时降级为 `proxies: [DIRECT]`**（与 rules 降级口径统一，防空 select 组 Clash/mihomo 加载失败）；④rules 中引用被剔除组的规则行同步降级为 DIRECT 并保留行（保证规则链完整）；⑤无凭据/高级未启用/OFF 等「占位仅替换为注释」场景同样执行 ②~④；**直接上传的模板（无蓝图）仅做占位文本替换、不重建**（模板作者自负其责）；SR conf 无代理组概念不受影响；**性能口径**：小团队量级（≤20 用户 × 数千规则）全量重渲染延迟应可接受，BuildN 增加渲染 benchmark 验收（如 1 万规则 <500ms 级，审核 F1-7）
- **注入条件（决策 #15）**：**users.uuid_encrypted 非空即注入**（用户激活时生成，trojan/ss 注入使用的 proxy_secret_encrypted 与 UUID 同步生成/清空，判定口径一致）——推送状态（pending/failed）不影响注入；管理员重试成功后立即生效，无需用户重新下载；未生成凭据的用户（从未激活/推送未启动）占位标记替换为注释 `# 节点未开通，请联系管理员`（**仅适用于直接上传模板**；装配生成的 Clash 模板仍按蓝图全量重渲染，见上），模板其余部分原样——订阅链接始终可用、YAML 语法完整
- **节点停用（决策 #18）**：渲染只注入 enabled=1 节点；组分配记录保留，重新启用即恢复注入；**Xray 侧同步**：enabled 0→1 对受影响 active 用户 AddUser diff、1→0 RemoveUser diff（与组分配变更同口径，事务提交后执行，审核 F3-3）
- **节点删除级联**：xray 来源节点删除 → group_nodes 分配记录外键 CASCADE；xray_users 中该节点（instance_id+inbound_tag）的推送记录级联删除，同时触发对受影响 active 用户的 `RemoveUser`（幂等容忍不存在，失败记同步状态可重试）；manual 节点删除仅影响装配引用（重新编辑按快照悬空容错处理，见 4.4）
- **实例删除级联**（审核 C7）：删除 Xray 实例 → 级联删除其 xray 来源节点（nodes.instance_id）→ group_nodes 随之 CASCADE → xray_users 对应记录级联删除；受影响 active 用户的 `RemoveUser` 仅在实例仍可达时尝试（已删除/不可达则跳过并记因，Xray 侧残留账号由部署文档提示手动清理）
- **超限用户（决策 #17）**：**下载内容不更改任何参数**（照常注入节点行）；仅在用户面板提示「流量已超限」
- **自定义订阅**（用户上传）：**不注入节点**，内容原样返回（用户自包含配置）
- **分享/规则下载**：**原样返回**（含占位标记，不做处理；分享无用户概念，不注入 UUID）
- **用量响应头（决策 #23）**：**仅高级模式携带**（基础模式无流量采集与配额概念，不携带，审核 D4；高级开关 OFF 后同样停止携带，见第一章）；**携带范围：仅用户订阅类下载**（无标识组解析 + 自定义订阅）携带，分享/规则下载不携带（与「原样返回」口径一致），存量显式 Token 下载按普通订阅处理（审核 F3-5）；下载响应注入 `subscription-userinfo: upload=; download=; total=; expire=4102444800`（远未来时间戳表达无到期，不用 0 避免客户端误解为已过期）+ `profile-update-interval` + `profile-web-page-url`，与平台附加头机制融合；超限用户 total 仍返回配额值
- **预览**：管理员预览显示模板原文（含占位标记）；用户预览按自身渲染
- 下载端点现有 `no-store` 禁缓存保证 per-user 内容无缓存污染；附加响应头/文件名/访问日志口径不变

### 5.8 流量配额机制

```
采集（cron，默认 10 分钟，面板配置可调，见 5.10）：QueryStats("user>>>{email}>>>traffic>>>uplink/downlink", reset=true)
  → 差值 UPSERT traffic_records(user_id, ym '2026-08')
超限检查（同任务）：SUM(本月累计) vs 用户配额（quota_override ?? 组 default_quota）
  → 超限 → RemoveUser + users.quota_exceeded=1（面板超限提示，见 5.5）
管理员手动重置：POST /api/admin/xray/users/:id/reset-quota
  → 清当月累计 + 重新 AddUser（凭据不变）+ 状态复位
```

- **自然月**：`traffic_records` 按 `ym`（YYYY-MM）聚合，跟随真实日历滚动，**无需定时重置表**；上月超限用户本月不会自动恢复（恢复 = 管理员手动重置，决策 #4）；**单位与口径（审核 B9 决策）**：uplink/downlink 存**字节整数**（Subscription-Userinfo 按字节输出，UI 展示时换算 GB）；`ym` 月界按 **UTC** 计算
- **采集方式**：Xray `GetUsersStats` 仅覆盖在线用户（核验结论见 docs/Reference/Xray-Core-API.md），故**必须逐用户 `QueryStats` 串行采集**（实例规模 1-5 台 × 用户量可控，配合并发限制串行化）；单次采集可合并 pattern 为 `user>>>{email}>>>traffic` 一次取上下行
- **采集状态与告警**：采集任务对每实例记录最近成功/失败状态与原因（xray_instances 扩展字段，见 5.9）；连续失败在 XrayInstancesView 展示告警标记（复用同步状态 UI 模式），使流量漏计从静默变为可观测；超限 `RemoveUser` 失败并入用户同步状态机（failed + last_error + 手动重试）
- **采集实现细则**（Xray-Core-API.md §11.2 取证）：QueryStats 的 pattern 为子串匹配，**禁止传空 pattern**（会返回并 reset 全部 counters，含 inbound/outbound 维度），必须传完整前缀；**counter 惰性注册且删除用户后不注销**（残留历史值），解析时按 email 过滤并与面板用户归一；reset=true 为原子 swap，不丢并发流量；「查无数据」≠「零流量」（counter 未注册）；**落库使用原子增量（UPSERT 累加），禁止先读后写**（AGENTS §4.6）；**采集与用户删除并发**：UPSERT 外键失败（用户已删）静默跳过、不记采集失败（审核 P3-10）
- **超限用户**：RemoveUser 后该用户 counter 停止产生，**本月累计保留**（不删除）；管理员重置时重新 AddUser（凭据不变，quota_exceeded 复位），counter 从 0 重新累计；超限期间任何 AddUser 类钩子被前置拦截（见 5.5）
- **配额 0/NULL 语义（审核 D9 决策）**：groups.default_quota / users.quota_override 为 **NULL 或 0 均视为不限流量**（跳过配额检查，Subscription-Userinfo 的 total 留空/省略），与基础模式「不限流量」心智一致
- **重置配额前置校验（审核 C-2 口径）**：reset-quota 仅对 status=active 用户执行重新 AddUser（禁用用户不推送，符合禁用语义）
- **已知损失**：Xray 重启 counter 清零导致差值丢失（业界通病，接受）；超限生效延迟 ≤ 1 个采集周期
- **Xray 侧前提**：服务器须开启 `policy.statsUserUplink/Downlink`（未开则统计恒为空，管理面板提示）

### 5.9 数据模型（迁移自 1009_xray.sql 起编号，1008 编号已被占用）

| 表 | 关键字段 | 说明 |
|----|---------|------|
| `rule_pools`（新增） | id, **name（UNIQUE，防重名歧义）**, urls_json（挂接 URL 列表）, last_synced_at, sync_status, sync_error, auto_sync（定时同步开关）, **sync_time**（定时同步每日执行时刻，默认 04:00 按 UTC，见 2.4） | 规则素材池（第二章；审核 G3 补齐）；仅管理员可增删改（2.4） |
| `pool_entries`（新增） | id, pool_id, rule_type, match_value, **source（url/manual）**, **sort_order（渲染顺序，见 2.2）**；**UNIQUE(pool_id, rule_type, match_value)**（2.2 去重合并） | 素材池条目；入库前白名单校验（2.3）；**URL 同步仅差量更新 url 来源，manual 条目永不受影响**（2.4）；渲染按 sort_order 输出（分流规则顺序有语义，见 2.2/3.5）；外键 ON DELETE CASCADE（池删除级联） |
| `xray_instances` | id, **name（UNIQUE）**, **slug（UNIQUE，`instance-` 前缀短码；节点名 {实例slug}-{入站tag} 的实例 slug 来源，审核 F4-6）**, api_addr, api_tag, enabled, **last_collect_at / collect_status / collect_error**（采集状态与告警，见 5.8） | 实例（1-5 台），api_addr 为 TCP 地址（IP 白名单防护）；**纳入配置导入导出**（决策 #24，format_version=2；**导入语义：整体覆盖 xray_instances 表**——先按导出内容重建实例行避免 xray_users 外键级联误删，xray_users/traffic_records 等业务数据不触碰，导入完成提示执行节点检测刷新，审核 D10/F3-6；**导入保护：向已配置系统导入时若 signing_key 将变化且存在任一业务密文（users.uuid_encrypted / users.proxy_secret_encrypted / nodes.protocol_json 凭据字段），拒绝导入并提示「配置导入仅适用全新部署/同密钥往返，在用实例请使用备份恢复」，审核 A4 决策**） |
| `nodes` | id, source（**CHECK 仅 manual/xray**）, **name（全局唯一，防 Clash proxies 同名冲突，审核 C-7）**, instance_id（xray 来源必填，manual 可空）, tag（xray 来源）, **UNIQUE(instance_id, tag)（xray 检测 upsert 唯一键，审核 F1-2）**, protocol（字符串，**无硬编码枚举 CHECK，由应用层协议注册表校验**（可扩展注册，见 3.2）；xray 来源限 vless/vmess/trojan/shadowsocks 四协议（决策 #20）；manual 来源 ClashOfficial 全量协议（ssr 除外）；**xray ss 节点的 cipher 随 inbound 检测入库**）, host, port, **protocol_json**（协议完整参数，manual 来源为 Clash 原生字段集 JSON；**仅凭据字段（uuid / password / private-key 等，清单由协议注册表定义，见 3.2）以 AES-256-GCM 加密存储，非凭据字段明文**；替代原按协议平铺的 flow/network/path/security/sni/fingerprint 等列）, is_public 默认 0, enabled, **allocatable（xray 来源 per-user 能力标记，默认 1；非四协议 inbound 置 0 并在 UI 提示，审核 F3-9）**, **last_seen_at / missing（Xray 侧已删 inbound 标记，检测未覆盖时提示管理员处置，审核 C-新18）** | **统一节点表**：manual 节点由管理员表单录入，xray 节点由实例检测入库；客户端表达字段供节点行渲染（Clash proxies 条目 / SR 节点链接，见 4.5/5.7）；is_public=1 对所有组自动可见（决策 #21） |
| `proxy_groups` | id, **name（UNIQUE）**, type（preset/custom）, preset_key（预设组标识，可空）, definition_json（**组类型（Clash proxy-group 类型：select/url-test/fallback，见 3.3）**、有序节点引用与子组引用） | Clash 代理组全局定义：预设库以种子数据随迁移内置（参考 `Clash.yaml.template.md`，见 3.3）；管理员自建组入库同表；装配快照仅记录勾选引用 |
| `group_nodes` | **PK(group_id, node_id)**, group_id, node_id, sort_order | 组 ↔ 节点分配（替代 group_selections；**仅 xray 来源节点可分配**，见 5.6）；sort_order 为组内顺序，UI 可调（决策 #22）；外键 ON DELETE CASCADE（**group_id / node_id 双侧**：组删除与节点删除均级联清理分配记录，见 5.6/5.7） |
| `groups`（改） | + default_quota | 组默认月度配额（**NULL 或 0 = 不限流量**，审核 D9 决策） |
| `users`（改） | + quota_override（可空）、+ uuid_encrypted、+ expire_at（预留，NULL）、+ **quota_exceeded**（超限标记，重置复位，见 5.5/5.8）、+ **proxy_secret_encrypted**（每用户统一代理密码，trojan/ss 每用户凭据，AES-256-GCM，见 5.5） | 用户配额覆盖（NULL=继承组，**0/NULL 不限流量**）；每用户一个 UUID 加密存储（决策 #16）；expire_at 为到期语义预留字段，本期不使用（到期不纳入本期） |
| `xray_users` | user_id, instance_id, inbound_tag, **node_id（REFERENCES nodes(id) ON DELETE CASCADE，节点删除时推送记录级联清理，审核 F1-2）**, email, sync_status, last_error；**复合 PK (user_id, instance_id, inbound_tag)**（审核 B4/C-新3） | 用户 Xray 账号推送状态（**不含 UUID**，决策 #16）；外键 ON DELETE CASCADE（用户删除级联） |
| `traffic_records` | **PK(user_id, ym)**, user_id, ym, uplink, downlink | 自然月流量累计（采集差值 UPSERT；**字节整数，ym 按 UTC**，审核 B9）；外键 ON DELETE CASCADE（用户删除级联） |
| `subscriptions`（改） | **UNIQUE(platform_id)** + product_type（yaml/subs）展示/校验属性 | **每平台一份订阅模板**（版本管理保留；同平台需两种格式时建两个平台，审核 A1 决策）；**升级不迁移存量订阅数据**（既有数据视为可放弃，1009 以「新表 + rename」切换重建，见第一章存量数据项/下方 1009 口径）；SR conf 不入本表（入规则实体，见 4.4） |
| **1009 迁移清理口径（审核 A3/D1/F1-3 决策）** | 显式 SQL 按序执行 + 框架钩子 | ①`DELETE FROM download_tokens WHERE subscription_id IS NOT NULL`（显式订阅 Token 不再新发，存量一并清；无标识/自定义 Token 保留）；②`DELETE FROM versions WHERE owner_type='subscription'`；③删 group_selections / subscription_group_rel；④**以「新表 + rename」切换重建 subscriptions**（含 UNIQUE(platform_id) 与 product_type）：**先重建 download_tokens**（保留 subscription_id 列、去掉指向 subscriptions 的外键）→ **创建新 subscriptions 表** → **rename 替换** → **DROP 旧表**（避免 foreign_keys=ON 下 DROP 被外键定义拒绝；注意 SQLite rename 会改写引用外键，须先去外键再 rename，审核 D1）；⑤**迁移框架新增「迁移后钩子注册表」机制**：注册 1009 钩子删除 contents/subscription/ 目录（幂等——目录不存在即跳过；失败记日志不阻断启动，审核 F1-3） |
| `assembly_blueprints`（新增） | version_id 1:1（NOT NULL REFERENCES versions **ON DELETE CASCADE**，版本驱逐/删除时快照同级联，审核 D11）, target_syntax（**clash-yaml / sr-subs / sr-conf**）, fixed_params_json, selection_json（节点/代理组/素材池勾选（SR 分流规则为池级勾选），含 Xray 候选集）, custom_rules_json, **render_plan_json（Clash YAML 装配产物的结构化渲染计划：头部 / manual proxies / proxy-groups 结构 / rules / 兜底规则，供下载时蓝图全量重渲染，见 4.4/5.7，审核 F1-1）** | 装配生成参数快照（见 4.4）；装配生成的版本行 1:1 快照，重新编辑读此恢复；**SR 分流规则的快照随规则版本存储**（version_id 指向规则版本行） |
| 删除 | group_selections / subscription_group_rel | 组选定机制移除 |

### 5.10 管理端点与 UI 影响

**后端新增/改造**：

- `internal/xray/`（新包）：client.go（gRPC 封装，依赖 **xtls/xray-core command 包** + 调用串行化 + 幂等错误映射：`already exists.` / `not found.` 子串匹配视为成功，见 Xray-Core-API.md §11.1）/ sync.go（生命周期同步）/ stats.go（流量采集）/ quota.go（配额检查）
- `internal/pool/`（新包，审核 G3）：规则素材池与条目 CRUD、URL 同步（逐行解析 + 白名单校验 + 单 URL 失败隔离，第二章）、可选定时同步（复用 cron ticker 模式）；**同步执行采用异步任务 + 轮询**：`internal/server/pool.go` 提供同步任务提交与状态查询端点（审核 F1-4）
- `internal/assembly/`（新包）：三类装配器渲染（Clash YAML / SR subs / SR conf）与快照存取（第四章）
- `internal/server/xray.go`：实例与节点 CRUD、连通性测试、**节点检测刷新（ListInbounds upsert，见 3.2）**、用户同步状态、手动重试、配额重置、流量报表、**实例级账号对账**（GetInboundUsers 与面板 xray_users 比对，展示差异：面板有/实例无→补推、实例有/面板无→建议清理，支持一键执行；**比对基数为全部 active 用户**，覆盖 ON 批量初始化中断遗漏的恢复路径，见第一章；见 Xray-Core-API.md §11.1）端点
- `internal/server/`：**高级模式中间件（advancedMode）**——读取 advanced_mode 配置键，OFF 时高级端点统一 403；**高级端点清单**：`/api/admin/xray/*` 全部、groups 节点分配与默认配额、users 配额覆盖/重置/同步状态相关端点；`/api/system/status` 暴露 advanced_mode 字段（审核 F2-4）
- `internal/download/`：渲染插入（模板 + 组节点 + UUID）+ **Subscription-Userinfo 响应头注入**（决策 #23；**携带范围：仅用户订阅类下载，见 5.7**）+ **无激活版本改 HTTP 200 注释块**（现有 ErrVersionNotFound 404 口径改造；**改造范围：订阅/分享/规则三个下载端点与 preview 会话预览端点逐一映射**——规则端点必改（空规则实体）、分享端点同口径，同步更新 download_test.go 断言，审核 D13/F3-14）+ **无标识解析改为按平台读当前激活模板**（group_selections 已删除，替换原组选定解析链路，审核 F1-5）
- `internal/group/`、`internal/subscription/`：组改造（节点分配 + 排序 + 默认配额 + 公共节点标注 + 候选集约束）、订阅地址池简化（每平台一份订阅模板，product_type 为模板属性，审核 A1 决策）；**`internal/rule/`、`server/rule.go`：放宽「创建必带首版」校验（允许空规则实体）、空态列表/Token/下载联动**（审核 D8/F3-11）；**`internal/version/`：CreateVersion 增加 activate opt-in 参数**（5 个调用点逐一传参：订阅池生成/上传与装配产物一律 false、规则/分享/自定义手工上传与在线编辑保持 true，审核 C1/F1-5）
- `internal/approval/`、`internal/user/admin.go`：推送钩子（事务提交后，入口先检查 advanced_mode 开关，见 5.5）
- `internal/dataclear/`：**ClearTablesTx 表清单扩展 9 张增量新表**（rule_pools / pool_entries / xray_instances / nodes / proxy_groups / group_nodes / xray_users / traffic_records / assembly_blueprints）**并同步移除 1009 迁移已删除的 group_selections / subscription_group_rel 两表**（否则对不存在的表 DELETE FROM 报错），一键清空与应急重新初始化共用（审核 G-4/B8）
- `internal/server/home.go`：平台卡片携带已用/总流量（traffic_records 按月聚合）；用户面板超限提示（决策 #17）；**groupSelected / adminPool / updatedAt 三处 group_selections 引用适配新分发链路**（审核 F1-5）；**管理员平台卡片改为预览形态**：仅模板信息 + 「按平台预览当前模板」按钮，移除一键导入/复制链接（审核 D2 决策）
- `internal/config/export.go`：**xray_instances 纳入导入导出**（format_version=2：payload 新增 instances 数组字段；导入整体覆盖实例表并提示重检测，决策 #24/F3-6）
- `internal/cron/`：流量采集（逐用户 QueryStats 串行，**间隔面板可配默认 10 分钟**）与配额检查任务
- `cmd/server/main.go`：组装 + 启动任务（含**迁移后钩子注册表**驱动，审核 F1-3）

**前端**：

- `views/admin/XrayInstancesView.vue`（新增）：实例/节点管理（含 is_public 标记、allocatable 标注）
- `views/admin/GroupsView.vue`（重构）：节点分配（含公共节点标注）+ **排序调整** + 默认配额（替代订阅选定）；**候选集并集为空引导**（提示先装配并激活 Clash 模板，见 5.6）
- `views/admin/SubscriptionsView.vue`（改造）：平台模板管理（Clash YAML / SR subs，各平台一份）；**含版本内容查看**——模板正文（含占位标记）可展开预览，装配蓝图生成的版本可辨识并提供重新编辑入口（见 4.4）；**VersionManageView 同步改造**：装配生成第三种创建方式入口、装配版本辨识与重新编辑入口的 UI 落点（审核 F1-5）
- `views/admin/AssemblyView.vue`（占位页 → 实现）：规则素材池管理（**含条目排序调整**）+ 三类装配器（Clash YAML / SR 节点订阅 / SR 分流规则，含重新编辑，见第二~四章）；SR 分流规则产物在规则管理页面展示与激活
- `views/admin/UsersView.vue`（扩展）：用量、Xray 同步状态、覆盖配额、手动重置
- `views/admin/RulesView.vue`（改造）：**空规则实体空态展示**与「无激活版本」提示（审核 F3-11）
- `views/admin/SettingsView.vue`（扩展）：新增「高级模式」分区——**高级模式开关、流量采集间隔、流量卡片显示开关**（审核 F1-5/P1.3）
- `views/HomeView.vue` / `ProfileView.vue`（扩展）：已用/总流量展示（**基础模式仅显「不限流量」，隐藏已用数值**，见第一章）+ **超限提示**；SR 平台用户另见「分流规则」卡片与双内容导入引导（**卡片仅做入口，点击跳转现有 /rules 列表页**，见 4.4，审核 F4-4）；**管理员首页平台卡片预览形态**（审核 D2）
- `api/xray.ts`、`api/pool.ts`（新增），`api/home|subscription|group|settings` 类型适配（group_selections 移除/高级字段），`layouts/AdminLayout.vue` 菜单（**高级模式开关驱动「Xray 实例」「用户组」入口显隐**）与 `AppHeader` 组名标签基础模式隐藏、`router/index.ts` 路由、`package.json` 增 jsdiff 依赖（4.1）、`utils/request.ts` 长任务轮询适配（审核 F1-5/F1-4）

> **UI 规格待补**：上述新增/重构页面的 GUI 规格尚未纳入 Design1-UI.md，待设计完善后补齐，本期不处理 UI 实现细节。

---

## 六、变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-16 | 两轮设计审查合并核验修订（16 项决策 + 勘误，经用户提问工具三轮确认）：决策类：①1009 迁移改「新表 + rename」切换重建 subscriptions（先重建 download_tokens 去外键 → 建新表 → rename 替换 → DROP 旧表，防 foreign_keys=ON 下 DROP 被外键定义拒绝；注意 rename 会改写引用外键）；②管理员首页卡片改预览形态（显式 Token 不再新发）；③ON 初始化中断由实例级账号对账兜底（比对基数为全部 active 用户）；④全部 AddUser 钩子前置凭据首建守卫（uuid + proxy_secret 同事务同 WHERE）；⑤pool_entries 增 sort_order（同步按源行序、手工可拖拽）；⑥迁移框架新增「迁移后钩子注册表」（1009 钩子删 contents/subscription/ 目录）；⑦修订前更新 AGENTS（Go 1.26、BuildN/IssueN 清单占位）；⑧高级端点清单 + advancedMode 中间件 + status 暴露 advanced_mode；⑨公共节点集合变化纳入激活 diff（RemoveUser/AddUser）；⑩节点 enabled 切换同步 Xray diff；⑪OFF 与钩子并发：钩子实时查 DB 标记；⑫用量响应头仅用户订阅类下载携带；⑬xray_instances 导入整体覆盖 + 提示重检测（format_version=2 增 instances 字段）；⑭分流规则卡片仅做入口跳转 /rules；⑮素材池同步改异步任务 + 轮询（后端任务端点）；⑯xray_instances 增 slug 列（节点名 {实例slug}-{入站tag}）。勘误类：assembly_blueprints 补 render_plan_json；xray_users 补 node_id 列 + nodes 增 UNIQUE(instance_id,tag) 与 allocatable；group_nodes/traffic_records 补主键、proxy_groups.name/rule_pools.name UNIQUE；5.1 目标 1 补「∪ 公共节点」；4.3 渲染语义限定（SR subs 占位替换 / 装配 Clash 蓝图重渲染）；第一章存量数据措辞修订（不迁移存量数据、1009 执行结构重建）；「预置」→「预设」全文统一；审核编号 B1/D10 重复引用修正（流量单位→B9、200 注释块→D13）；4.1 例外措辞；第一章入口表述（新增 Xray 实例入口/解锁用户组入口）；Reference §二/§四 trojan/SS 对齐决策 #20；5.7 无凭据分支限定直传模板；OFF 占位注释优先级；「双态」术语清除；2.3 三段式规则行取前两段；sync_time 按 UTC；采集与用户删除并发跳过；preview 端点 subscription_id 参数移除；5.10 联动清单补齐（rule 空实体/version activate 五调用点/home.go 三处适配/SettingsView 高级分区/RulesView 空态/api 类型/jsdiff/VersionManageView/request 轮询） |
| 2026-08-16 | 第五轮完整核验修正（6 项，均由已确认决策推导，无新设计内容）：①N1 OFF 清 Xray 侧收集清单补实例连接信息（api_addr/api_tag，防清库后 RemoveUser 失去连接目标）；②N2 dataclear 扩展 9 新表同时移除 1009 已删的 group_selections/subscription_group_rel；③N3 公共节点措辞统一：ON 初始化/5.1 目标/决策 #2/#9/5.5 触发器表补齐「∪ 公共节点」；④N4 SR subs 预览改「直接显示明文原文」对齐明文存储口径；⑤N5 组管理页标注口径改为「已激活蓝图候选集并集」；⑥N6 「无占位模板原样返回」限定为直接上传模板，装配 Clash 模板走蓝图全量重渲染 |
| 2026-08-16 | Design2Report1~4 四轮审核报告合并核验，经用户逐轮决策确认 15 项（提问工具四轮确认）：①产物类型模型：**每平台一份订阅模板 UNIQUE(platform_id)**，product_type 为模板属性，下载路由/Token 零改动，首次激活改按订阅行 current_version=0 判定（审核 A1/G-3/F1，R2 与 R4 推荐方案冲突由用户裁决）；②**Clash proxy-groups 改蓝图全量重渲染**：装配存结构化渲染计划，空强制组降级 DIRECT、rules 降级 DIRECT，无凭据/OFF 场景同重建，直传模板仅文本替换（审核 A2/G-新1/G-1/G-2/F3）；③**1009 迁移显式 SQL 清理 + 一次性钩子删 contents/subscription/ 目录**，无标识/自定义 Token 保留（审核 A3/F6）；④**配置导入保护**：signing_key 变化且存在业务密文时拒绝导入（审核 A4/G-新3）；⑤**OFF 清空后 best-effort RemoveUser 清 Xray 侧账号**，不可达跳过记 warn 并提示（审核 A5/G-新2）；⑥**公共 xray 节点自动 AddUser 至全部 active 用户**（审核 A6/C-新5）；⑦**候选集 = 已激活蓝图并集 + 下载按模板过滤 + 激活时差集删分配并 RemoveUser diff**（审核 A7/C-新12）；⑧允许空规则实体（审核 C-1/D8）；⑨配额 0/NULL = 不限流量（审核 C-新8/F2/D9）；⑩无激活版本下载返 200 注释块并改测试（审核 D13）；⑪系统注入头覆盖平台附加头同名键，高级 profile-update-interval=21600（审核 D-新6/C-4/F7/D11）；⑫素材池定时同步**停机错过不补偿**（用户决策未采纳审核 D12 推荐）；⑬B 级口径批量采纳（xray_users 复合 PK/凭据并发首建守卫/SR subs 明文存储+默认扩展名/dataclear 补 9 表/advanced_mode 开关与钩子静默跳过/流量字节与 UTC 月界等）；⑭Go 1.26 升级维持 BuildN Step 0 先行；⑮规则 Token「一键导入」措辞订正为「复制链接」（审核 C6） |
| 2026-08-16 | 全文核验审查收敛（7 决策 + 10 错误修正，经用户逐项决策）：决策类：①条目去重冲突→手工添加与既有条目同（类型+值）时拒绝并提示（2.2 隐含于去重合并语义）；②空产物从警示升级为硬校验拒绝生成（强制组至少 1 节点/SR subs 至少 1 节点，3.3/4.1）；③vless cipher 改空占位对齐样例（4.5/5.7）；④首次入池自动激活扩展到规则实体（首个 conf 版本，4.4）；⑤代理组类型枚举 select/url-test/fallback（3.3/5.9）；⑥基础模式流量卡隐藏已用数值仅显「不限流量」（第一章/5.10）；⑦OFF 保留装配快照、悬空引用复用 4.4 容错（第一章）。错误类：凭据措辞残留修正（4.3/5.1→决策 #2 口径、决策 #2/#4/#13/#15/#19 UUID→凭据）；rule_pools 补 sync_time 字段；5.2 并发表述对齐 Reference §11.4；5.5 超限拦截枚举补组节点分配变更；4.5 删 SSpanel 不当引用；group_nodes 外键双侧 CASCADE；补空池下载口径（AGENTS §4.8）；另：同步并发互斥、「预置」改「预设」 |
| 2026-08-16 | 设计细节完善（用户 7 点改进 + 未决策项清点，经用户决策）：①素材池条目增加 source（url/manual）标记，URL 同步差量更新仅作用于 url 来源、manual 永不受影响，部分失败不差删（2.2/2.4/5.9）；②数万行规模应对（去重索引/批量事务/分页/拼接无限制，2.4）；③协议可扩展注册（应用层注册表 + protocol 字符串无硬编码 CHECK，3.2/5.9）；④决策 #20 修订：xray 来源放宽为 vless/vmess/trojan/shadowsocks 四协议（UserManager 源码边界），每用户凭据 = UUID（vless/vmess）+ 统一代理密码 users.proxy_secret_encrypted（trojan/ss），注入渲染/开关清空/批量初始化/Account 形态同步扩展（3.2/5.4/5.5/5.7/4.5/第一章/5.9）；⑤代理组内节点顺序可调（definition_json 有序 + 拖拽，select 组首节点默认选中，3.3）；⑥SR 兜底 FINAL 改手选 DIRECT/PROXY 二选一默认 PROXY（3.4/3.6/4.2）；⑦订阅管理页含版本内容查看（蓝图模板可见，5.10）；⑧清点决策：定时同步每日可配时刻默认 04:00（2.4）、diff 库定 jsdiff（4.1） |
| 2026-08-15 | 外部审核报告收敛（B1/G1~G4/C1~C8/D 级逐项核对，经用户决策）：①B1 Go 版本冲突修正：项目 Go 升级 1.26 + 引入 xray-core 模块（原「不动 Go 版本」口径废弃，5.3）；②G1 quota_exceeded 标记落 users 表（5.5/5.8/5.9）；③G2 全部 AddUser 钩子前置拦截超限用户（5.5）；④G3 补齐 rule_pools/pool_entries 表定义（5.9）；⑤C1 activate opt-in 范围界定：仅订阅池+装配产物 false，规则/分享/自定义保持创建即激活（4.4）；⑥C2 显式 Token 落地处置（表列保留/预览端点简化，第一章）；⑦C3 Clash 装配跳过 USER-AGENT 并提示（3.5）；⑧C4 组分配仅限 xray 节点，manual 仅静态渲染（5.6）；⑨C5b 候选集外分配组页标注（5.6）；⑩C6 素材勾选统一池级（3.4）；⑪C7 实例删除级联补齐（5.7）；⑫D4 用量头仅高级模式携带（5.7）；⑬D10 导入范围仅实例连接配置（5.9）；⑭D11 blueprint 外键 ON DELETE CASCADE（5.9）；已修项确认：G4/C5a/C8/节点删除级联均已在前轮决策落文 |
| 2026-08-15 | 设计验证收敛（五轮多角度研究：代码扩展点 9/9 吻合、Design1 兼容矩阵、15 场景模拟、边界排查；经用户逐项决策）：①阻断级：proxy-groups 悬空引用改为**渲染层按用户实际注入节点动态重建**（5.7）；Xray 节点**检测入库机制补齐**（手动检测 + instance_id+tag upsert + 状态保留，3.2）；②候选集级联清理时机改为**新版本激活时才执行**（5.6）；③开关 ON 批量初始化（存量用户 UUID + 全量推送，含重新开启重推送，第一章）；④OFF 清空范围明确（仅 source=xray 节点，manual/proxy_groups/groups 保留；用量头停止携带）；⑤SR conf 目标规则实体改为**手动新建后选择**（3.4）；⑥SR conf 用户端规则卡片 + 双内容导入引导（4.4）；⑦快照悬空容错：重新编辑加载时标记失效引用（4.4）；⑧节点删除全链路级联 + RemoveUser（5.7）；⑨实例级账号对账端点（5.10）；⑩采集状态与告警入库展示（5.8/5.9） |
| 2026-08-15 | 研究整合（依据 docs/Reference/ 四项目深度研究成果，经用户决策）：①SR subs 的 vless 链接改为 base64 userinfo 形态（与真实样例一致：vless://base64(cipher:uuid@host:port)?tls=1&peer=&xtls=2&pbk=&sid=）；②vmess 链接维持 V2rayN JSON 格式（生态兼容性最广）；③manual 节点协议范围移除 ssr（生态无可靠链接生成参照，urlclash-converter 只收不生成）；④决策 #23 增配 profile-update-interval 与 profile-web-page-url 两个约定头；⑤5.8 整合采集实现细则（pattern 完整前缀/counter 残留过滤/原子增量落库）；⑥5.5 整合幂等匹配细节（codes.Unknown 字符串匹配、email 全小写、vmess alter_id 移除）；⑦4.1 新增空产物警示；⑧修正 protocol_json 加密边界为「仅凭据字段加密」；⑨参考文档链接补齐 Node-Link-Standards.md 与 SSpanel-Subscribe.md |
| 2026-08-15 | 核验改进（参照 ClashOfficial 协议全集、Shadowrocket.subs.template.md 样例与 urlclash-converter 转换逻辑）：①manual 节点协议范围扩展为 ClashOfficial 全量代理协议，参数改 protocol_json 存储（Clash 原生字段集，YAML 原样输出零转换，凭据 AES-256-GCM 加密），xray 来源仍仅 VLESS/VMESS（决策 #20 不变）；②产物归属修正：装配产物按「平台 + 产物类型」分流——Clash YAML 与 SR subs 入订阅地址池，SR conf 入现有规则实体；③SR 拆分为节点订阅/分流规则两个独立装配器，可单独生成；④subs 输出形态定稿：STATUS/REMARKS 头部行表单可配 + 逐行节点链接，整体 base64；节点链接按 SR 原生参数风格渲染（vless REALITY：tls=1/xtls=2/peer/pbk/sid），映射参照 urlclash-converter；不可转链接协议跳过并提示；⑤占位注入同步支持 SR subs（注入链接行后全文重新 base64） |
| 2026-08-15 | 由 DesignOnHold.md 定稿内容规范化整理为 Design2：全部设计与决策（模式分层、规则素材池、装配拼接、配置生成与分发、Xray 对接 24 项决策）转入本文档；源文档演进历史见 DesignOnHold.md 变更记录 |
