# SSpanel-Subscribe.md — SSPanel-UIM 订阅生成与分发机制研究

> **文档定位：** 本文档是对成熟面板 SSPanel-UIM（PHP 8.2 + Slim 4 + Eloquent）订阅分发链路及其配套管理功能的取证研究，为 Design2.md 的配置生成与分发（第四章）与 Xray 对接的用户生命周期/流量配额机制（第五章）提供参照。与姊妹文档 [SSpanel.md](./SSpanel.md) 的关系：SSpanel.md 为早期速览，本文档为深度取证版（以其为准）。
> **核验来源：** 本地仓库 `/Users/kylechen/Desktop/Repo/SSPanel-UIM`（VERSION='25.1.0' 代号 "The Restoration"，app/predefine.php:9-10；HEAD d55a607，2026-07-11）。框架为 Slim 4（非 Laravel），仅复用 illuminate/database。
> **标注约定：** 【源码事实】= 直接引自代码行；【推断】= 由源码逻辑推出；【猜测】= 基于领域经验的推测。

---

## 一、订阅请求链路（SubController）

`GET /sub/{token}/{subtype}` → `SubController::index`（app/routes.php:26），处理顺序【源码事实】：

1. **subtype 白名单**：json/clash/sip008/singbox/v2rayjson/sip002/ss/v2ray/trojan 共 9 种（SubController.php:33）
2. **全局开关 + Host 校验**：`$_ENV['Subscribe']` 开关 + `'https://'.Host === $_ENV['subUrl']` 完全匹配（:35-38）——防盗链
3. **限流**：token 经 antiXss 清洗后按 IP 与 token 双维度可选限速（:42-49）
4. **token 查 Link 表** + `Link::isValid()`（含 `is_banned===0` 校验，src/Models/Link.php:26-33）
5. **渲染分发**：`Subscribe::getContent($user, $subtype)` → `getClient()` 用 `match` 返回渲染器实例（src/Services/Subscribe.php:64-77）；抽象基类 `Base::getContent($user): string`（src/Services/Subscribe/Base.php:7-9）
6. **节点选取**：`getUserNodes()`（Subscribe.php:38-57）统一过滤——type=1 启用、node_class≤用户等级、node_group 匹配（0=公共组）、节点带宽未超限；所有格式共享此过滤

**响应头**【源码事实】（SubController.php:66-92）：

| 头 | 值与语义 | 适用格式 |
|----|---------|---------|
| `Subscription-Userinfo` | ` upload={u}; download={d}; total={transfer_enable}; expire={unix_ts}`（注意 upload= 前有前导空格）；u/d=累计字节、total=**总配额非剩余**、expire=等级过期 Unix 秒 | **全部 9 种格式无条件携带** |
| `Profile-Update-Interval: 6` | 更新间隔小时数（硬编码） | 仅 clash |
| `Profile-Web-Page-Url` | `$_ENV['baseUrl']` | 仅 clash |
| `Content-Disposition` | `attachment; filename={appName}` | 仅 clash |

**全仓无 Cache-Control/ETag/Expires 订阅缓存头**【源码事实】——与 Design2「下载禁缓存 no-store」方向一致（AGENTS §4.5），但 SSPanel 是「无头」而非「显式 no-store」，Design2 口径更严谨。

## 二、凭据注入与多格式渲染

用户凭据字段：`passwd/uuid/port/method`（src/Models/User.php:23-24,39）。各格式注入点【源码事实】：

| 格式 | 凭据注入 | 位置 |
|------|---------|------|
| ss | `base64(method:passwd@server:port)#name` | SS.php:26 |
| sip002 | `method:passwd@server:port/?plugin=...#name`（**未 urlencode/base64——passwd 含特殊字符会坏链**） | SIP002.php:31-33 |
| v2ray（vmess://） | JSON `id=>user->uuid`，base64 包裹 | V2Ray.php:38-52 |
| trojan | `trojan://uuid@server:port`（password 即 uuid） | Trojan.php:39-42 |
| clash | ss：port/password/cipher=用户三元组；vmess/trojan/tuic：`uuid=>user->uuid` | Clash.php:34-44,87-88,121,157 |
| clash SS2022 | 密码派生 `Tools::genSs2022UserPk`：sha3-256(passwd) 截 16/32 字节 base64，可拼 server_key 前缀 | Clash.php:48-72、Utils/Tools.php:210-232 |
| sip008 | JSON 直入 + `bytes_used/bytes_remaining` 附带用量 | SIP008.php:30-50 |

**Clash YAML 生成机制**【源码事实】：`array_merge($_ENV['Clash_Config'], ['proxies'=>$nodes], $_ENV['Clash_Group_Config'])` 后 `yaml_emit`（PHP 原生扩展，Clash.php:182-189）；骨架（全局 misc + proxy-groups 骨架 + rules）是**静态 PHP 数组模板**（config/appprofile.example.php:446-455+）；动态部分 = 每生成一个节点把节点名追加进 `Clash_Group_Indexes`（默认 `[0,1,2,4,6,7,8,11]`）指定的 proxy-groups 的 proxies 数组（Clash.php:177-179）——**「骨架模板 + 运行时按索引插入」，非占位符字符串替换**。SingBox 同理（outbounds 追加，SingBox.php:179-184）。

**base64 时机**【源码事实】：**无任何格式对响应体整体 base64**；仅行内编码（ss 每行 userinfo、v2ray 每个 vmess JSON）。——与 Design2 SR subs「整体 base64」口径不同，两者均为生态真实形态（Shadowrocket 客户端两种都接受；本项目样例 Shadowrocket.subs.template.md 为整体 base64 形态，见 Node-Link-Standards.md 第六章）。

**注意**：v25.1.0 **不支持 VLESS**（无 vless 渲染器）【源码事实】。

## 三、边界行为

| 场景 | 行为 | 证据 |
|------|------|------|
| 封禁用户 | 订阅返回通用错误「订阅链接无效」（HTTP 200 错误页，非 4xx） | Link.php:26-33、SubController.php:53-54 |
| 流量超限 | **订阅端无任何检查**，照样渲染完整订阅（拦截发生在节点侧 /mod_mu 下发过滤） | SubController.php 全文无此逻辑【源码事实】 |
| 等级过期 | 不过滤，仅 expire 头反映过期时间戳（可为过去时间） | SubController.php:69 |
| 空节点 | 行格式返回空串；clash 返回 `proxies: []` 的 YAML（**客户端导入会失败**）；sip008 返回 `servers: []` 合法 JSON | SS.php:16-19、Clash.php:182-189 |
| 格式子开关关闭 | enable_ss_sub/enable_v2_sub/enable_trojan_sub 关时对应渲染器返回空串；**clash/singbox 不受约束** | SS.php:18、V2Ray.php:20 |
| 不支持的节点 sort | 渲染器内静默 `continue` 跳过（如 WireGuard sort=3 不出现在订阅） | Clash.php:166-173 |
| 错误响应风格 | 所有失败统一 200 + 文案、不区分原因——利于防探测，调试困难 | SubController.php:31-54【推断】 |

**已知 bug**【源码事实】：SIP008.php:27-42 的 `$nodes[] = $node` 在循环末尾无条件执行（应位于 sort===0 分支内），非 SS 节点会重复 push 上一个节点；V2RayJson.php:104 存在 `$security === ('tls' || 'auto')` 恒等表达式 bug。

## 四、配套管理链路（"如何实现类似功能"全景）

### 4.1 用户生命周期

- **创建统一工厂**：自助注册（`POST /auth/register`，reg_mode=open/close/invite）与管理员创建（`POST /admin/user/create`）**共用 `registerHelper`**（AuthController.php:218-298,304-380；Admin/UserController.php:103-152 传 `is_admin_reg=1`）——注册时一次性初始化 passwd/uuid/api_token/port、transfer_enable=reg_traffic、class/class_expire、node_iplimit/node_speedlimit、auto_reset 参数、随机 node_group（:238-282）
- **无审批环节**【源码事实】：注册即生效登录（:286-294）——Design2 的审批流为自有增量，无参照
- **封禁横切开关**：`is_banned` 一个布尔位覆盖四层读取点——Web 中间件 302（Middleware/User.php:33-37）、订阅校验（Link.php:28）、节点下发过滤（WebAPI/UserController.php:50-53）、奖励冻结（Reward.php:23）；自动封禁路径 = 审计累计违规（Detect::ban，Detect.php:115-181，支持定时解封）；另有 `is_shadow_banned` 影子封禁（仅禁签到，订阅不受影响【推断】）
- **删除级联**：`User::kill()` 手动枚举 8 张表（User.php:245-259）；**财务/工单/小时流量表故意残留**（无外键级联）——Design2 用外键 ON DELETE CASCADE（AGENTS §4.7）更优

### 4.2 节点管理与双通道架构

- **Node 模型**（Node.php:16-38）：type（显隐）/sort（协议：0=SS、1=SS2022、2=TUIC、3=WireGuard、11=Vmess、14=Trojan，:83-91）/server/node_class/node_group（0=公共）/node_bandwidth 三件套/node_speedlimit/node_heartbeat/custom_config（**每节点 JSON 覆盖**）/traffic_rate（静态+动态时段倍率）/password（WebAPI 通讯密钥）
- **双通道互不相通**【源码事实】：①机器通道 `GET /mod_mu/nodes/{id}/info`（带 ETag，NodeToken 中间件校验 muKey+Host+可选 IP 白名单，Middleware/NodeToken.php:22-68）供节点后端（XrayR 类）轮询；②客户端通道仅 `/sub/{token}/{subtype}`。**无面向客户端的节点配置端点**——Design2 同为单下载通道，方向一致
- **声明式节点控制**【推断】：面板只存状态不反向控制节点，封禁/超限在「下次轮询自然生效」——与 Design2 的 gRPC 主动推送（AddUser/RemoveUser）是**两种相反架构**：SSPanel 拉模型（延迟=轮询周期、无需节点凭证管理）vs Design2 推模型（即时、需串行 gRPC 客户端）

### 4.3 流量统计与配额

- **采集=节点推送**：`POST /mod_mu/users/traffic`（WebAPI/UserController.php:135-213）按 traffic_rate 折算后更新 user.u/d/transfer_total/transfer_today——**计费口径（u/d 乘倍率）与物理口径（transfer_total）分离**；小时图表双写 hourly_usage（原始流量不乘倍率）
- **u/d 更新为 read-modify-write**（:191-197）【源码事实】——并发上报丢更新；Design2 的差值 UPSERT 应使用原子增量（AGENTS §4.6 已有要求，此为反面参照）
- **配额执行点在下发侧**：`/mod_mu/users` 剔除 `transfer_enable<=u+d` 用户；`keep_connect` 开关可改为限速 1Mbps 保留连接（:92-99）；节点级 `node_bandwidth_limit` 用尽直接拒绝（:46-48）
- **无主动对账**【推断】：节点故障期流量漏计无补偿——Design2 的逐用户 QueryStats 拉取+差值落库在准确性上更强，已知损失（Xray 重启清零）已接受（Design2 5.8）
- **重置任务**：resetTodayBandwidth（每日清零 transfer_today）、resetFreeUserBandwidth（auto_reset_day 到期重置 u/d 与配额）、expirePaidUserAccount（等级过期降级清零）、节点带宽按月重置日（Cron.php:446-483,155-184）——Design2 选自然月聚合+手动重置，更简

### 4.4 骨架配置存储

- Clash/SingBox 骨架存 **PHP 文件**（config/appprofile.php 的 `$_ENV` 数组），**管理员无 UI 可改**【源码事实】（设置中心只覆盖数据库 config 表的订阅开关类键值，Admin/Setting/SubController.php:19-54）——**这是 Design2 的差异化优势点：头部表单 + 装配快照入库可编辑（4.2/4.4）恰好补齐 SSPanel 的空白**
- 节点级覆盖放 DB JSON（custom_config），用户级订阅覆盖不存在（渲染器硬编码 9 种，无自定义模板）

### 4.5 定时任务

外部 crontab 每 5 分钟 `php xcat Cron`（Command/Cron.php:25-135）：每轮订单状态机/过期降级/余量提醒/IP 解析/离线检测/邮件队列（`FOR UPDATE SKIP LOCKED` 防重）；整点 GFW 探测与审计封禁；每日任务用 `last_daily_job_time` 时间戳闸门幂等（:54-91）。【推断】无内部调度器，多实例部署会重复执行每日任务——Design2 用进程内 ticker（现有 cron/cleanup.go 模式）单实例无此问题。

## 五、功能映射总表（SSPanel 机制 → Design2 关注点）

| SSPanel-UIM 机制（file:line） | Design2 对应设计 | 评估 |
|------|------|------|
| registerHelper 统一初始化配额（AuthController.php:218-298） | 5.5 生命周期同步触发器（事务提交后钩子） | 借鉴「单一工厂」思想；Design2 钩子点已枚举完整且多出 OIDC/换组路径 |
| is_banned 横切四层（Middleware/User.php:35 等） | 基础模式禁用即停推送（5.5 RemoveUser） | Design2 推模型需主动同步，语义等价 |
| /mod_mu 拉模型 + NodeToken（muKey+Host+IP 白名单） | gRPC 推模型 + IP 白名单（5.3 路径 A） | 架构相反但鉴权思路一致；gRPC 无鉴权需部署者白名单（已定稿） |
| Subscription-Userinfo 头四字段（SubController.php:66-73） | 决策 #23 用量响应头 | **完全对齐**：total=总配额、expire=Unix 秒；Design2 用远未来时间戳表无到期（SSPanel 用真实过期时间，本期无到期概念故合理） |
| Profile-Update-Interval / Profile-Web-Page-Url（仅 clash） | 已纳入 Design2 §5.4/§5.7（基础模式沿用平台附加头，高级模式系统注入并覆盖平台同键） | 平台附加头机制已存在（Design1 4.3），Design2 已落文 |
| Clash 骨架+按索引插入节点名（Clash.php:177-189） | 装配器生成完整 YAML（第四章） | Design2 的装配快照+重新编辑强于 SSPanel 静态骨架 |
| 下发侧配额过滤（WebAPI/UserController.php:92-99） | 决策 #3/#17：超限仅移除 Xray 账号、下载内容不变 | 语义一致（订阅内容不受超限影响） |
| 无缓存头（全仓无 Cache-Control） | no-store 禁缓存（AGENTS §4.5） | Design2 更严谨 |
| 错误统一 200+文案防探测 | 下载错误返回 404 / 200+注释块（Design1 4.3、AGENTS §4.8） | 各自自洽 |

## 六、可借鉴模式与需规避陷阱（汇总）

**借鉴**：①配额初始化单一工厂；②封禁作横切开关；③计费/物理流量口径分离；④「骨架模板+运行时插入」渲染思想（Design2 以装配快照实现更优版本）；⑤心跳即健康检查（隐式心跳+离线告警）；⑥双维度限流（IP+token）；⑦Host 白名单防盗链。

**规避**：①read-modify-write 流量累加（用原子增量）；②手动枚举级联删除（用外键级联）；③骨架配置不可 UI 编辑（Design2 已规避）；④空节点输出 `proxies: []`（Design2 4.1 预览阶段应提示空产物）【推断】；⑤超限/过期用户订阅端无提示（Design2 决策 #17 已在用户面板提示，更优）；⑥SIP002 未编码（Design2 链接生成按 Node-Link-Standards.md 规范编码）；⑦外部 crontab 时间戳闸门幂等（单实例进程内 ticker 更简）。

---

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-15 | 新建：SSPanel-UIM v25.1.0 订阅链路深度取证（两轮：渲染层 + 管理链路全景），含功能映射总表与 Design2 对照评估；核验版本 VERSION=25.1.0（HEAD d55a607） |
| 2026-08-18 | 同步 Design2 修订：Profile-Update-Interval / Profile-Web-Page-Url 已纳入 Design2 §5.4/§5.7，本表旧「未设计」口径订正 |
