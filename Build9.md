# Build9.md — VPN 订阅管理系统 当前构建方案（第一阶段：基础扩展与前置修复）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第一阶段当前构建方案（v1.4）**，承接已验收的 [Build8.md](Build8.md)、已归档的 [docs/reports/Build/Build4~7](docs/reports/Build)，并基于：
> - 第二阶段见 [Build10.md](Build10.md)（承接本卷 Step 1~6 验收后的核心装配与收口）。
> - 项目内研究资料：[docs/Reference/Clash-Subscription-Validation-Emoji-API.md](docs/Reference/Clash-Subscription-Validation-Emoji-API.md)、[docs/Reference/Clash-Verge-Rev-Node-Parameters.md](docs/Reference/Clash-Verge-Rev-Node-Parameters.md)、[docs/Reference/Clash-Verge-Rev-Subscription-Assembly.md](docs/Reference/Clash-Verge-Rev-Subscription-Assembly.md)；
> - 第三方代码仓库：`~/Desktop/Repo/clash-verge-rev`（本地 git 提交 `3503a2da29d68a4398c0b8e9234cffb711e65783`，2026-08-26）；
> - 当前项目代码与文档（本地 git 提交 `1745eae`，2026-08-27；`c681742` 为最近一次业务代码提交，其后提交仅修改 Build9.md，代码与 `c681742` 一致）。
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 当前设计：[Design2.md](Design2.md) / [Design2-UI.md](Design2-UI.md)
> - 当前问题：[Issue7.md](Issue7.md)（R22 系列，部分开放项纳入本 Build 前置修复）
>
> **标注约定：**
> - 【源码事实】= 直接从本地仓库源码或可复现实测得到；
> - 【推断】= 由源码逻辑或生态约定合理推出；
> - 【假设】= 本轮用户离开后，按授权做出的未确认决策，实施前应复核；
> - 本文档“参考代码”均为 Build 期实施代码/伪代码，**本 Build 只写文档，不构建、不改任何代码与既有文档**。

---

## 〇、本轮用户确认的探索基线

| # | 决策项 | 结论 |
|---|--------|------|
| 1 | 文档落点 | 仓库根目录 `Build9.md`，作为下一轮当前构建方案；不改动 Build8 与 AGENTS 引用 |
| 2 | 研究范围 | 校验/格式/Emoji/对外 API、节点参数与协议注册表、分层订阅装配/扩展机制、UI/交互细节、后端任务/持久化架构，五项全选 |
| 3 | 兼容策略 | 增量演进优先；若 clash-verge-rev 设计更优，允许破坏性重构；当前无活跃用户与需兼容数据，兼容性不作为约束 |
| 4 | 内容深度 | 每个 Step 给出文件级参考代码 + 单测验收命令（Build8 风格） |
| 5 | 事实来源 | 本地 clash-verge-rev 源码 + docs/Reference 为主，互联网作补充 |
| 6 | 平台范围 | 以 Clash 路线为主，Shadowrocket 仅在共用层顺带覆盖 |

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 0 | 创建 Build9 文档与事实基线 | ✅ 已完成（本文档） |
| 1 | 前置修复：Issue7 R22-02/03/06/07/08（版本 id、装配 UI、SR-conf 直建规则） | ◧ 进行中：R22-02/06/07/08 已实施（a0cd819）；R22-03 仅剩“条件化目标选择区” |
| 2 | Clash 产物静态自检 + YAML 输出迁移到 goccy（方案B，方案A回退） | ☐ 未开始（已选定 goccy 全迁移方向，见 §4.5） |
| 3 | 订阅响应头语义层与 RFC 5987 文件名（Clash Verge 导入兼容） | ☐ 未开始 |
| 4 | 节点协议注册表/字段补全 + Clash 渲染与 URI 参数对齐 | ☐ 未开始 |
| 5 | 规则类型与素材池解析扩展（mihomo 规则全集） | ☐ 未开始 |
| 6 | 代理组类型与字段扩展（load-balance/relay/use/健康检查等） | ☐ 未开始 |
| 7~11 | 分层装配、URI 导入、池补跑、原子写、收口 | 已拆至 [Build10.md](Build10.md) Step 1~5 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/assembly/{TypeTargetStep.vue,AssemblerShell.vue}`、`frontend/tests/{assembly-view.spec.ts,home-view.spec.ts}`（其余 R22-02/06/07/08 已实施，见 §4.5） | 剩余仅：R22-03 条件化目标选择区；同步修复 R22-03/Issue8 后的两个过期前端测试；其余为核验与回归 |
| 2 | `backend/internal/assembly/{goccy_yaml.go,selfcheck.go,render_clash.go,clash_plan.go,service.go,models.go}`、`backend/go.mod`、`backend/internal/assembly/*_test.go` | goccy 全量迁移：MapSlice 输出、CommentMap 注释、自检、静态校验、预览告警/生成阻断 |
| 3 | `backend/internal/download/download.go`、`backend/internal/server/download.go`、`backend/internal/platform/platform.go`、`backend/internal/setup/setup.go`、`frontend/src/views/admin/PlatformEditView.vue` 及测试 | RFC 5987 Content-Disposition、profile 头语义校验（兼容 {frontend_url}、interval 允许 0）、系统头优先级 |
| 4 | `backend/internal/node/{registry.go,node.go}`、`backend/internal/assembly/links/links.go`、`backend/internal/assembly/render_clash.go`、`frontend/src/views/admin/NodesView.vue` 及测试 | 协议字段补全、嵌套敏感字段、数组字段归一化、传输参数链接；新注册表为唯一口径，不做旧字段兼容 |
| 5 | `backend/internal/rulespec/spec.go`（新增）、`backend/internal/pool/{parser.go,sync.go}`、`backend/internal/assembly/{selfcheck.go,validate.go,render_clash.go,render_sr.go}`、`frontend/src/views/admin/AssemblyView.vue` 及测试 | mihomo 规则全集、Clash no-resolve 元数据、SR 支持面、逻辑规则括号配平解析、自检同步扩展 |
| 6 | `backend/internal/proxygroup/proxygroup.go`、`backend/internal/assembly/{selfcheck.go,models.go,load.go,clash_plan.go,render_clash.go}`、`frontend/src/views/admin/ProxyGroupsView.vue` 及测试 | 组类型 5 枚举、use/健康检查/过滤等字段、渲染与自检同步扩展 |
| 7~11 | 见 [Build10.md](Build10.md) 文件清单 | 分层装配覆盖层、URI 导入、池补跑、版本原子写、全量收口 |

---

## 三、构建顺序依赖图

```
Step 0（本文档）
  → Step 1（R22-03 条件化目标区收口，其余 R22 已实施）
  → Step 2（输出自检与 Emoji，后续所有 Clash 改动的安全网）
  → Step 3（响应头兼容，可独立，但建议在 2 后）
  → Step 4（节点字段，Build10 Step1/2 依赖其字段模型）
  → Step 5（规则元数据，Build10 Step1 覆盖层与渲染依赖）
  → Step 6（代理组字段，Build10 Step1 覆盖层与渲染依赖）
  → Build10 Step 1（分层装配覆盖层，核心，见 Build10.md）
  → Build10 Step 2（URI 导入，依赖 Build9 Step4）
  → Build10 Step 3（池同步启动补跑，独立，可并行）
  → Build10 Step 4（版本原子写 + 下载覆盖层接线，依赖 Build10 Step1/3）
  → Build10 Step 5（收口）
```

---

## 四、事实基线与关键差异（研究结论）

### 4.1 研究来源与版本

| 来源 | 标识/路径 | 说明 |
|------|-----------|------|
| 第三方客户端源码 | `~/Desktop/Repo/clash-verge-rev` @ `3503a2d`（2026-08-26） | 本文所有 CVR 事实的第一来源 |
| 本项目源码 | `~/Desktop/Repo/VPN-Subscription-Management` @ `1745eae`（2026-08-27） | 当前实现对照；业务代码与 `c681742` 一致（其后仅 Build9.md 变更）；核验时同时参考了 `a0cd819`（R22 系列修复） |
| Reference 1 | `docs/Reference/Clash-Subscription-Validation-Emoji-API.md` | 导入校验、格式、Emoji、响应头 |
| Reference 2 | `docs/Reference/Clash-Verge-Rev-Node-Parameters.md` | 节点字段定义 |
| Reference 3 | `docs/Reference/Clash-Verge-Rev-Subscription-Assembly.md` | 订阅/扩展装配机制 |
| 互联网补充 | [官方 GitHub](https://github.com/clash-verge-rev/clash-verge-rev)、[DeepWiki Profile 架构](https://deepwiki.com/clash-verge-rev/clash-verge-rev/5-profile-management-system)、[DeepWiki Timer](https://deepwiki.com/clash-verge-rev/clash-verge-rev/5.3-hotkeys-and-lightweight-mode)、[官方 URL Schemes](https://clashvergerev.com/en/guide/url_schemes)（2026-08-27 检索） | 仅作核对；未能逐文件比对的结论以本地源码为准 |

### 4.2 关键【源码事实】清单（Build9 各 Step 的设计依据）

1. **远程订阅导入只做轻量校验**：`src-tauri/src/config/prfitem.rs::from_url` 下载后去 UTF-8 BOM，`serde_yaml_ng::from_str::<Mapping>` 解析，必须含顶层 `proxies` 或 `proxy-providers` 之一；不校验 `proxy-groups`、`rules`、节点字段完整性。
2. **激活才做完整校验**：`src-tauri/src/core/validate.rs` 调用内核 `<core> -t -d <app_dir> -f <config_path>`；失败关键字 `FATA/fatal/Parse config error/level=fatal`；保存扩展文件（`cmd/save_profile.rs`）失败会恢复原文件；切换订阅失败会丢弃候选并恢复旧 current（`cmd/profile.rs`）。
3. **最终装配管线顺序**（`src-tauri/src/enhance/mod.rs::enhance`）：
   `rules` seq → `proxies` seq → `proxy-groups` seq → 合并 App 默认配置 → 内建脚本 → TUN → DNS → 捕获控制面快照 → 全局 Merge/Script → 订阅 Merge/Script → 恢复控制面 → `cleanup_proxy_groups` → `use_sort`。
4. **Merge 语义**：`enhance/merge.rs::use_merge` 仅 Mapping 递归合并，其余类型以 merge 侧覆盖；merge 顶层 key 先小写化。
5. **SeqMap 语义**：`enhance/seq.rs::use_seq`，最终列表 = `prepend + (原列表 - delete) + append`；proxies 场景会把新增节点名插入**第一个 selector/select 组**的最前面（源码并未要求该组 proxies 非空）；删除 proxies 时同步从所有组 `proxies` 移除。
6. **Script 限制**：`enhance/script.rs::use_script` 使用 boa_engine，1000 条日志、日志 1MB、JSON 10MB、1000 万次循环、5 秒超时；`main(config, profileName)` 必须返回 object。
7. **控制面保护**：`enhance/mod.rs` 的 `CONTROL_PLANE_KEYS` 包含 `external-controller`、平台相关控制通道、`secret`、`mixed-port`、`socks-port`、`port`、`tun`、`mode`、`allow-lan`、`log-level`、`ipv6`、`unified-delay` 等；在用户 merge/script 后强制恢复 App 权威值；`ensure_lan_bind_address` 在 allow-lan 且 bind 回环时改为 `*`。
8. **清理悬空引用**：`enhance/mod.rs::cleanup_proxy_groups` 合法名 = proxies 名 ∪ proxy-groups 名 ∪ proxy-providers 名 ∪ 内置策略 `DIRECT/REJECT/REJECT-DROP/PASS`（源码未把 `COMPATIBLE` 列入该处白名单）；从组 `use` 删不存在 provider，从组 `proxies` 删不存在的节点/组/provider；非字符串成员保留。
9. **顶层排序**：`enhance/field.rs::use_sort` 先按 `HANDLE_FIELDS`，再普通键，最后按 `proxies → proxy-providers → proxy-groups → rule-providers → rules` 固定输出。
10. **响应头读取**：`prfitem.rs` 读取 `subscription-userinfo`（兼容 `x-amz-meta-subscription-userinfo` 等后缀形式）、`profile-update-interval`（**单位小时**，`u64` 解析后乘 60）、`profile-web-page-url`（须 http/https 且带 host）、`Content-Disposition` 先 `filename*` 百分号解码再 `filename`。
11. **自动更新**：`src-tauri/src/core/timer.rs` 仅调度 `remote + allow_auto_update=true + update_interval>0`；`TaskSchedule::new` 对已过期项给 0 延迟；启动时会立即补跑 overdue 项；`constants.rs::profile::MIN_UPDATE_INTERVAL = 1440` 分钟；更新失败保留旧文件，成功才覆盖 `updated`。
12. **更新重试**：`feat/profile.rs` 正常 → 失败后 `self_proxy`（Clash 内核代理）→ 再失败 `with_proxy`（系统代理）。
13. **节点类型全集**：`src/types/global.d.ts::IProxyConfig.type` 为 `ss | ssr | direct | dns | snell | http | trojan | anytls | hysteria | hysteria2 | tuic | wireguard | ssh | socks5 | masque | vmess | vless | mieru | sudoku`；当前项目 manual 注册表 19 项（含 openvpn/shadowquic/trusttunnel/tailscale，不含 ssr/direct/dns/sudoku）。
14. **节点基础字段**：`IProxyBaseConfig` 含 `tfo/mptcp/interface-name/routing-mark/ip-version/dialer-proxy`；各协议还有大量 transport/reality/插件字段（详见 Reference 2）。
15. **代理组类型与字段**：`IProxyGroupConfig.type` 支持 `select/url-test/fallback/load-balance/relay`；还含 `use/url/expected-status/interval/timeout/max-failed-times/lazy/disable-udp/interface-name/routing-mark/filter/exclude-filter/exclude-type/include-all*/hidden/icon`。
16. **规则类型全集**：`rules-editor-viewer.tsx` 列出的 mihomo 规则类型 30+，并带 `noResolve`/validator 元数据；`MATCH` 无匹配值，输出 `MATCH,policy`。
17. **URI 导入与去重**：`proxies-editor-viewer.tsx` 多行 URI 粘贴，先 `atob` 尝试 Base64 解码，逐行 `parseUri`，解析失败跳过不阻塞，按 name 去重保留第一条，空 name 节点从可视化列表过滤但保留原文；`uri-parser/` 支持 `ss/ssr/vmess/vless/trojan/anytls/hysteria2(h2)/hysteria(hy)/tuic/wireguard(wg)/http(s)/socks5(socks)`，并识别 VLESS 的 Shadowrocket base64 userinfo 形态。
18. **Emoji 实测与方案选择**：本项目 `gopkg.in/yaml.v3 v3.0.1` 会把 4 字节 emoji 输出为 `"\U0001F680..."`；Go yaml.v3 与 js-yaml 均能解析回原串；但为可读性与老解析器兼容，需要真实 UTF-8。实测 `github.com/goccy/go-yaml v1.19.2`（当前 `go.mod` 中已有 indirect）原生输出真实 emoji，并支持 `MapSlice` 保序、`UseOrderedMap` 嵌套保序、`CommentMap` 注释、`AutoInt` 整数归一。**【用户已确认】Step2 选择方案B：全量迁移到 goccy；方案A（yaml.v3 + restoreYAMLUnicodeEscapes）保留为回退记录。**
19. **当前项目已有对齐点**：下载端点已注入 `profile-update-interval/profile-web-page-url/subscription-userinfo` 与禁缓存头；`RenderClashPlan` 已有可达性收敛；节点已有 `display_name/有效渲染名` 全局唯一；规则素材池已有 URL 同步、30 分钟超时、取消端点。
20. **当前项目主要差距（核验后更新）**：无输出静态自检；Emoji 被 yaml.v3 转义；Content-Disposition 现状为“系统端点只写单行 `filename=`，默认 Clash Verge 平台模板只写单行 `filename*=UTF-8''subscription.yaml`，且 server 先写系统头后写平台头，平台旧模板会覆盖系统值”；协议字段明显少于 CVR；规则类型 8 类；组类型 3 类且无 `use/健康检查`；无 merge/seq 覆盖层；无 URI 导入；版本文件非原子写入。**已闭环的差距另行注明**：素材池同步长事务（R22-01）已由短事务 + 联合索引修复，仅缺启动补跑；版本列表缺 `id`（R22-02）已修复；R22-03 仅剩“条件化目标选择区”。

### 4.3 关键【推断】（有事实依据，仍需测试确认）

- 对装配生成内容做“CVR 导入校验 + 引用/结构静态检查”的组合，可以等价覆盖绝大多数 `core -t` 能发现的配置错误；不能替代内核对协议字段语义的最终校验。【推断】
- 覆盖层必须在“完整基础文档（蓝图 + 动态 Xray 节点注入）组装之后、可达性收敛之前”应用，才能保证 `delete` 与 `prepend/append` 以及 Merge 注入的 provider 参与最终收敛；这偏离了 CVR“seq 先于默认配置”的严格顺序，但适合本项目的动态渲染模型。【推断】
- 若在生成预览与下载重渲染使用同一覆盖层实现，`RenderClashPlan` 的自包含结构保持成立，历史蓝图缺省覆盖层时天然等价于当前行为。【推断】
- CVR 的 `use_sort` 输出顺序可作为本项目 Clash 输出的固定键序；管理员头部 JSON 的原始顺序不再作为唯一顺序依据，但可保留为未声明键的次序。【推断】
- 对平台 `extra_headers` 增加已知头语义校验不会破坏既有能力；系统生成的 Content-Disposition 应覆盖平台手填的旧模板值，否则无法提供动态订阅名。【推断】
- URI 导入的解析器可先覆盖 CVR `uri-parser` 支持的 scheme，但 **ssr 按 Design2 §4.5 排除（归入候选 C2）**；snell/mieru/masque/openvpn/ssh 等无标准 URI 的协议继续保持“不可导入”并在回执中说明。【推断】

### 4.4 本轮【假设】清单（用户离开后按授权做出，实施前建议复核）

- **A1 兼容性不设限**：允许改动现有表结构、接口字段与前端布局；当前无活跃用户与有价值数据（用户确认口径，2026-08-27 二次确认数据库可重新初始化）。因此 Build10 Step 1 的覆盖层存储可直接落地，Build10 Step 3 的同步事务拆分已落地、仅剩启动补跑，Step 4 不做旧字段兼容。
- **A2 不内置 mihomo 二进制**：本轮只做静态自检；“可选 `core -t` 真校验”进入候选列表，需用户决定二进制分发与资源占用。
- **A3 不实现 JS Script 扩展**：与 AGENTS“简单轻量化”冲突较大；Build10 Step 1 只实现 Merge + Rules/Proxies/Groups Seq 四层，Script 保留扩展点并进入候选。
- **A4 覆盖层首轮只服务 Clash YAML**：SR subs/generic-subs/sr-conf 暂不应用 Merge/Seq；共用层（规则元数据、响应头）仍顺带覆盖。
- **A5 Issue7 纳入范围裁剪（核验后更新）**：R22-02/03/06/07/08 因与装配 UI/重新编辑强相关纳入 Step 1；其中 R22-02/06/07/08 已在 `a0cd819` 实施，R22-01 已在 `a0cd819` 完成主体修复；R22-03 仍开放，本次仅需按 Issue7 v1.11 后补修订实现“条件化目标选择区”；R22-04/R22-05 已在 `a0cd819` 闭环，不再纳入本 Build。
- **A6 池同步补跑口径**：当前池是“每日固定时刻”模型，不引入 per-pool 相对间隔列；启动补跑仅补偿“今天应跑但停机错过”的任务，保持现有数据模型简单。
- **A7 覆盖层输入为 YAML 文本（v1.2 更新为 goccy）**：与 CVR 一致，前端不做 JSON 表单化；后端统一使用 `goccy/go-yaml` 的 `MapSlice` + `UseOrderedMap` 解析，并在预览时给出定位错误。
- **A8 Step 顺序可执行**：每步完成后均可编译测试，不跳步；执行者仍应遵守 AGENTS“每次仅执行一个 Step，验收后再下一步”。

### 4.5 核验后修订要点与用户确认（v1.1；v1.3 增补）

- **核验基于当前 HEAD：** `1745eae`（2026-08-27；业务代码与 `c681742` 一致），并对照 `a0cd819`（R22 系列修复）与 CVR `3503a2d`。核验期间未修改任何代码。
- **已实施确认：**
  - R22-02：`ListVersions` 已返回 `v.id`，前端已加保护；
  - R22-06：高级模式关闭时隐藏/剔除 Xray 节点，预设组不再常显 `preset`；
  - R22-07：节点选择弹窗已改为“可选在上、已选在下”，无移除按钮；
  - R22-08：SR-conf 支持新建规则名并自动创建规则；
  - R22-01：`pool/sync.go` 已改为短事务分批 + 临时 keep 表索引 + `1014` 联合索引。
- **R22-03 仍未闭环：** 当前 `AssemblyView.vue` 仍无条件显示“目标选择”卡片；Issue7 v1.11 要求：
  1. 若平台中不存在自定义平台（`is_default=false`），隐藏目标选择区，并自动选中匹配当前副 Tab 的原生默认平台；
  2. 若存在自定义平台，显示目标平台选择器，并默认选中匹配当前类型且已有订阅的平台；
  3. SR-conf 不受该条件影响，仍保留“新建规则/选择已有规则”入口。
- **用户已确认决策（2026-08-27 核验问答）：**
  - **Build9 先修订再继续**：已实施项不再重复实现，R22-03 按 Issue7 最新方案修订；
  - **Step2 选择方案B（goccy 全量迁移）**：经小实验后确认，YAML 序列化与后续解析统一迁移到 `goccy/go-yaml`；方案A（yaml.v3 + `restoreYAMLUnicodeEscapes`）保留作为回退记录，若方案B出现不可接受问题可回滚；
  - **接受方案B的文本差异**：goccy 会对 `on/yes/空字符串` 等自动加引号，使用 `AutoInt()` 保持整数输出；与现有 yaml.v3 输出为语义等价但文本可能不同，需同步调整少量测试断言；
  - **注释用 CommentMap 管理**：`# {{xray_nodes}}` 和下载提示注释改用 `goccy.CommentMap` 在序列化时生成，不再依赖 `yaml.Node.HeadComment`；
  - **Step7 控制面保护完整采纳 CVR**：Merge 不能覆盖完整 `CONTROL_PLANE_KEYS`（`external-controller`、`external-controller-cors`、平台相关控制通道、`secret`、`mixed-port`、`socks-port`、`port`、`redir-port/tproxy-port`（按平台门控）、`tun`、`mode`、`allow-lan`、`log-level`、`ipv6`、`unified-delay`），并按 CVR 单独保护 `dns.ipv6`；merge 顶层 key 先小写化；
  - **Step8 URI 导入遇同名节点**：跳过并回执，不覆盖已有节点。
- **用户已确认决策（2026-08-27 二次核验，v1.3 增补）：**
  - **Step7 覆盖层先于可达性收敛**：下载重渲染先组装完整基础文档并应用覆盖层，再基于最终文档做组可达性收敛与规则降级；新增“覆盖层 prepend 节点/provider 可救活依赖组”测试；
  - **Step4 不做旧字段兼容**：当前数据库可以重新初始化；删除 `normalizeLegacyTransport` 与旧键兼容策略，新协议注册表为唯一口径；
  - **Step5 逻辑规则括号配平解析**：AND/OR/NOT 按括号配平取表达式并剥离末尾 policy；URL 源仅存表达式、手动源 target 入目标字段；`selfcheck.go` 同步使用共享解析逻辑；
  - **Step3 响应头校验兼容生态**：`profile-web-page-url` 允许 `{frontend_url}` 占位符；`profile-update-interval` 允许 0（u64 口径，0=不自动更新）；`subscription-userinfo` 各值要求非负整数；
  - **Step1 同步修复前端过期测试**：`assembly-view.spec.ts`（R22-03 后下一步行为变化）与 `home-view.spec.ts`（Issue8 后分流规则空态文案变化）列入 Step1 产出，Step1 起前端测试必须全绿；
  - **selfcheck 随元数据演进**：`selfcheck.go` 以 `rulespec`/`proxygroup` 共享元数据为最终口径（Step2 先落地基础检查，Step5/6 删除临时白名单并切换共享定义），并列入 Step5/6 产出清单与测试，避免逻辑规则与 `use/include-all` 误报/漏报。
- **写入 Step 计划的技术提醒：**
  - Step5 的 `AND/OR/NOT` 表达式允许逗号，解析与校验需单独处理；
  - Step7 `cleanupProxyGroups` 需保留 CVR 的 `has_valid_provider` 细节，并保持本项目 `COMPATIBLE` 白名单扩展；
  - Step9 启动补跑必须避免与当前分钟命中的定时检查重复提交同一池；
  - Step10 版本原子写需明确临时文件失败清理。


---

## 五、候选构建项（本轮不实施，待后续用户决策）

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| C1 | 可选 mihomo `-t` 真校验 | 内置/可配置内核路径，生成与激活前执行；需决策二进制体积、平台与超时 | Reference 1 §1.3、CVR `validate.rs` |
| C2 | 协议扩展：`ssr/sudoku/direct/dns` 等 | CVR 支持；`ssr` 仍受 Design2 链接转换决策约束，建议单独评估 | CVR `global.d.ts`、Design2 §4.5 |
| C3 | JS Script 覆盖层 | boa_engine 或 Go JS 引擎，5 秒超时/输出上限；安全性需单独设计 | CVR `script.rs` |
| C4 | 订阅级 `extra_headers` | 资源自身头 > 平台头 > 系统头，三级合并 | Reference 1 §4.5 |
| C5 | 远程订阅完整导入模型（URL + 5 扩展子项） | 将 CVR `PrfItem` 模型引入订阅管理 | CVR `prfitem.rs` |
| C6 | proxy-providers / rule-providers / sub-rules 管理 | 与 Step 6 的 `use`、Step 5 的 `RULE-SET/SUB-RULE` 配套的下一层能力 | CVR 编辑器与 `enhance` |
| C7 | 池同步代理退避（直连 → 内核代理 → 系统代理） | 服务器侧需先定义“可用代理”来源，安全边界待定 | CVR `feat/profile.rs` |
| C8 | ~~Issue7 R22-04 / R22-05~~ | 已在 `a0cd819` 修复并验证，不再属于本 Build 候选；保留此处供历史追踪 | Issue7 |

---

## 六、分步构建计划

> 每步格式遵循 [docs/DocTemplates/Build.template.md](docs/DocTemplates/Build.template.md)：目标 → 前置条件 → 产出文件与操作（含参考代码）→ 测试与验收命令 → 验收标准。执行者每次仅执行一个 Step。

### Step 1：R22-03 收口（其余 R22 项已实施，核验后修订）

- **目标：** 当前 HEAD 已完成 R22-02/06/07/08 与 R22-01 主体；本步只处理仍开放的 R22-03：把“目标选择”从无条件常驻改为按 Issue7 v1.11 条件化展示，并做回归确认，避免 Build10 Step 1/2 的覆盖层与 URI 导入叠加在旧交互上。
- **前置条件：** Step 0；当前代码已通过后端相关包测试与前端构建；R22-03 方向已按 Issue7 v1.11 确认。核验确认 `npm test -- --run` 现有 2 个失败用例由 R22-03/Issue8 改造产生，本步一并修复，Step 1 结束时前端测试必须全绿。
- **现状（核验确认，不再重复实现）：**
  - R22-02：`ListVersions` 已返回 `v.id`，前端已加保护；
  - R22-06：高级模式关闭时隐藏/剔除 Xray，预设组标签已调整；
  - R22-07：节点选择弹窗已改为“可选在上、已选在下”，无移除按钮；
  - R22-08：SR-conf 支持新建规则名并自动创建规则，失败路径清理已存在；
  - R22-01：素材池同步已短事务分批 + 临时 keep 表索引 + 联合索引。
- **本步产出文件与操作：**

  1. R22-02 已实施：`ListVersions` 已返回 `v.id`，本步无需再改 `version.go`。

  2. R22-08 已实施：`GenerateInput.RuleName`、后端自动建规则与失败清理、前端规则名入口均已落地，无需再改后端 `assembly.go`/`models.go`/`validate.go`。
  3. R22-06/07 已实施：`NodesGroupsStep.vue` 的 Xray 隐藏、preset 标签移除、节点弹窗排序/移除按钮调整均已落地。

  4. `frontend/src/views/admin/AssemblyView.vue`：R22-03 条件化目标选择区。
     - 增加 `hasCustomPlatform` 计算：
       ```ts
       const hasCustomPlatform = computed(() =>
         (context.value?.platforms ?? []).some((p) => p.is_default === false)
       )
       ```
     - 非 SR-conf 时：
       - 如果 `hasCustomPlatform` 为 true，渲染 `<Card title="目标选择">`；
       - 如果只有内置默认平台，隐藏目标选择卡片，并在 `loadContext` / `watch(targetSyntax)` 中自动选中匹配当前副 Tab、且已有订阅的原生默认平台。
     - SR-conf 始终显示“新建规则 / 选择已有规则”入口，不依赖平台卡片。
     - 路由 `platform_id/rule_id` 仅预填，不控制步骤条。
  5. `frontend/src/views/admin/assembly/TypeTargetStep.vue`：保持现有紧凑选择组件；若后续需要，可增加“默认平台自动匹配”提示文案。
  6. `frontend/src/views/admin/assembly/AssemblerShell.vue`：复核 target 步骤已移除、无遗留 `#target` 插槽。
  7. `frontend/tests/assembly-view.spec.ts`：更新“未选择目标时下一步被拦截”为 R22-03 后的真实行为（仅默认平台时自动选中目标；自定义平台场景按本步目标就绪规则断言）。
  8. `frontend/tests/home-view.spec.ts`：将“管理员暂未设置分流规则”断言更新为 Issue8 后的当前空态/引导文案（`分流规则为 Shadowrocket 客户端专用…` 口径），与 `HomeView.vue` 实际输出一致。

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test -- --run
  cd backend && go test ./internal/version ./internal/assembly ./internal/server
  ```

- **验收标准：**
  - 仅有内置默认平台时，非 SR 装配器不出现“目标选择”卡片，并自动选中匹配当前类型的默认平台；
  - 存在自定义平台时显示目标选择器，默认选中匹配当前类型且已有订阅的平台；
  - SR-conf 始终保留规则名称/已有规则入口，不受平台条件影响；
  - 已实施的 R22-02/06/07/08 回归通过；
  - `assembly-view.spec.ts` 与 `home-view.spec.ts` 按新交互/新文案更新后，`npm test -- --run` 全绿。


### Step 2：Clash YAML 输出迁移到 goccy + 静态自检（方案B；方案A为回退记录）

- **目标：** 将 Clash YAML 渲染与后续 YAML 解析统一迁移到 `goccy/go-yaml`：输出真实 UTF-8 emoji，同时补齐 CVR 可静态表达的顶层结构、必填字段、组/规则引用、空 select 组自检。
- **前置条件：** Step 1；用户已确认采用方案B（goccy 全量迁移），接受与现有 yaml.v3 文本差异。
- **关键结论（2026-08-27 研究）：**
  - `goccy/go-yaml` 原生输出 emoji，不需要后处理还原；
  - `goccy.MapSlice` 可保持顶层/嵌套映射顺序；
  - `gyaml.UseOrderedMap()` 可让嵌套映射解析为 `MapSlice`，适合覆盖层与自检；
  - `gyaml.AutoInt()` 可避免浮点数被输出为 `7890.0`；
  - `gyaml.CommentMap` 可生成 `# {{xray_nodes}}` 与用户提示注释；
  - goccy 不能直接序列化 `*gopkg.in/yaml.v3.Node`，因此需要迁移到 `MapSlice` / `[]any`，不能保留 Node 树直接输出。
  - **方案A（yaml.v3 + restoreYAMLUnicodeEscapes）保留在本文档下方作为回退记录；当前方向为方案B。**
- **产出文件与操作（当前方案B）：**

> 下面先写方案B整合步骤；从“【方案A回退记录】”开始的是回退备选，不参与本次构建。

  **B.1 依赖与统一封装**
  - `backend/go.mod`：将 `github.com/goccy/go-yaml v1.19.2` 从 indirect 提升为直接依赖；移除 `gopkg.in/yaml.v3` 的直接引用（代码全量迁移后）。
  - 新增 `backend/internal/assembly/goccy_yaml.go`，统一提供：
    ```go
    // orderedMapToMapSlice 将现有 OrderedMap 转为 goccy MapSlice（保序）。
    func orderedMapToMapSlice(m *OrderedMap) gyaml.MapSlice

    // toGoccyValue 将当前 toYAMLNode 支持的值转为 goccy 可序列化值。
    // 支持 nil/string/bool/int/int64/float64/[]string/[]any/map[string]any/*OrderedMap。
    func toGoccyValue(v any) any

    // marshalClashYAML 统一 goccy 序列化：AutoInt + 可选 CommentMap。
    func marshalClashYAML(doc gyaml.MapSlice, comments gyaml.CommentMap) ([]byte, error)
    ```

  **B.2 改造 `render_clash.go` 与 `clash_plan.go`**
  - 不再创建 `*yaml.Node` 树；顶层、`proxies`、`proxy-groups`、`rules` 全部用 `gyaml.MapSlice` + `[]any` 构建。
  - `clashProxy` / `dynamicClashProxy` 仍返回 `*OrderedMap`，在序列化时通过 `orderedMapToMapSlice` 转成 goccy 映射。
  - 注释不再写入 `yaml.Node.HeadComment`，而是构造 `gyaml.CommentMap`：
    ```go
    // 首个 proxy 前注释：$.proxies[0]；无 proxy 时：$.proxies
    func proxyCommentMap(comment string, hasProxies bool) gyaml.CommentMap {
        text := goccyHeadComment(comment) // 去掉已有 '#', 统一加一个前导空格
        path := "$.proxies"
        if hasProxies { path = "$.proxies[0]" }
        return gyaml.CommentMap{path: {gyaml.HeadComment(text)}}
    }
    ```
  - 使用 `gyaml.MarshalWithOptions(doc, gyaml.AutoInt(), gyaml.WithComment(comments))` 输出。
  - 注意：goccy 的 `HeadComment` 若文本已含 `#` 会输出 `##`，因此统一用 `goccyHeadComment` 规范化。

  **B.3 静态自检 `selfcheck.go`**
  - 用 goccy 解析，而不是 yaml.v3：
    ```go
    var root gyaml.MapSlice
    if err := gyaml.UnmarshalWithOptions(content, &root, gyaml.UseOrderedMap()); err != nil {
        return []OutputIssue{{Severity: "error", Path: "$", Message: "YAML 解析失败: " + err.Error()}}
    }
    ```
  - 后续通过 `mapGet(root, "proxies")` / `seqOf(...)` / `scalarString(...)` 等 helper 完成：
    - 顶层 `proxies` 或 `proxy-providers` 必须存在；
    - proxies 必填项、协议注册表必填字段；
    - 组类型/引用、空 select 组；
    - rules 目标引用；
    - warning 级：兜底规则缺失。
  - **自检必须元数据驱动**：规则类型/取值/no-resolve 判据最终消费 `rulespec`，代理组类型/use/include-all 判据最终消费 `proxygroup` 的共享定义。Step2 先落地基础检查（当前 node 注册表 + 现有组类型），Step5/6 引入/扩展共享元数据后**必须删除 selfcheck 内的临时规则/组白名单并切到共享定义**；Step5/6 会同步扩展本文件并补测试。

  **B.4 覆盖层/后续 YAML 解析全量统一**
  - Step7 的 `OverlayInput` 解析也改为 `gyaml.UnmarshalWithOptions(..., gyaml.UseOrderedMap())`；
  - `applySeq` / `deepMerge` / `cleanupProxyGroups` / `sortTopLevel` 全部基于 `gyaml.MapSlice` 和 `[]any` 实现；
  - 不再使用 `gopkg.in/yaml.v3` 的 `yaml.Node`。


  **【方案A回退记录】** 1. 若方案B出现不可接受问题，可回退为：新增 `backend/internal/assembly/yaml_text.go`，保留 `gopkg.in/yaml.v3`，通过 `restoreYAMLUnicodeEscapes` 还原 YAML 双引号标量中的 Unicode 转义。以下为方案A完整参考实现，仅作回退，不用于当前构建。

  ```go
  // 【源码事实】yaml.v3 v3.0.1 把 🚀 输出为 "\U0001F680"；Go yaml.v3 与 js-yaml
  // 能解析回原值，但直接查看可读性差，且部分旧解析器不识别 8 位 Unicode 转义。
  // 实现采用“YAML 双引号标量上下文”扫描：只还原双引号标量内的 \U/\u 转义；
  // 普通标量中出现的字面引号（如 name: a"b）不会被误判为引号定界符。
  func restoreYAMLUnicodeEscapes(in []byte) []byte {
      var out bytes.Buffer
      out.Grow(len(in))
      inDQ := false
      for i := 0; i < len(in); i++ {
          c := in[i]
          if c == '"' {
              if inDQ && isClosingDQ(in, i) {
                  inDQ = false
              } else if !inDQ && isOpeningDQ(in, i) {
                  inDQ = true
              }
              out.WriteByte(c)
              continue
          }
          if inDQ && c == '\\' && i+1 < len(in) {
              n := in[i+1]
              digits := 0
              if n == 'U' && i+9 < len(in) { digits = 8 }
              if n == 'u' && i+5 < len(in) { digits = 4 }
              if digits > 0 {
                  hexs := string(in[i+2 : i+2+digits])
                  if v, err := strconv.ParseUint(hexs, 16, 32); err == nil && utf8.ValidRune(rune(v)) {
                      r := rune(v)
                      if r == '\t' || r == '\n' || r == '\r' ||
                         (r >= 0x20 && r != 0x7f && r != 0xfffe && r != 0xffff) {
                          out.WriteRune(r)
                          i += 1 + digits
                          continue
                      }
                  }
              }
              // 其他转义原样保留（\" \\ \n \t ...）
              out.WriteByte(c)
              out.WriteByte(n)
              i++
              continue
          }
          out.WriteByte(c)
      }
      return out.Bytes()
  }

  // prevNonSpace/nextNonSpace 只回看/前瞻空格与 Tab。
  // 开启定界：行首引号，或前一个非空白字符是 : - [ { ,
  // 关闭定界：行尾引号，或后一个非空白字符是 : , ] } #
  // 【推断】本项目 renderClash 产物为块式 YAML + 少量 flow 集合，该判定覆盖其标量形态。
  func isOpeningDQ(in []byte, i int) bool {
      switch prevNonSpace(in, i) {
      case 0, ':', '-', '[', '{', ',':
          return true
      }
      return false
  }
  func isClosingDQ(in []byte, i int) bool {
      switch nextNonSpace(in, i) {
      case 0, '\n', ':', ',', ']', '}', '#':
          return true
      }
      return false
  }
  ```

  > 说明：还原后的原始 emoji 出现在双引号标量内仍是合法 YAML UTF-8；若后续实测某客户端不兼容，`goccy/go-yaml v1.19.2`（当前已是 indirect 依赖，实测原生输出 emoji）是备选方案。【推断】

  2. 【方案A回退中的自检参考】新增 `backend/internal/assembly/selfcheck.go`：以下自检校验逻辑可复用；当前方案B中解析部分必须改为 goccy，见上文 B.3。

  ```go
  type OutputIssue struct {
      Severity string `json:"severity"` // error / warning
      Path     string `json:"path"`     // $ / proxies[0] / proxy-groups[0] / rules[3]
      Message  string `json:"message"`
  }

  // CheckClashContent 对装配产物做静态自检。
  // 等价覆盖 CVR prfitem 导入校验 + 内核常见解析错误中可静态表达的部分（【推断】）。
  func CheckClashContent(content []byte) []OutputIssue {
      var issues []OutputIssue
      var root map[string]any
      if err := yaml.Unmarshal(content, &root); err != nil {
          return []OutputIssue{{Severity: "error", Path: "$", Message: "YAML 解析失败: " + err.Error()}}
      }
      proxies, hasProxies := root["proxies"]
      _, hasProviders := root["proxy-providers"]
      if !hasProxies && !hasProviders {
          issues = append(issues, OutputIssue{Severity: "error", Path: "$",
              Message: "必须包含顶层 proxies 或 proxy-providers"}) // 【源码事实】CVR prfitem.rs 同口径
      }
      // 1) proxies：name/type/server/port + 协议注册表必填字段（direct/dns 例外）
      proxyNames := map[string]bool{}
      if list, ok := proxies.([]any); ok {
          for i, raw := range list {
              m, ok := raw.(map[string]any)
              if !ok {
                  issues = append(issues, OutputIssue{"error", fmt.Sprintf("proxies[%d]", i), "节点必须是映射"})
                  continue
              }
              name, _ := m["name"].(string)
              typ, _ := m["type"].(string)
              if name == "" {
                  issues = append(issues, OutputIssue{"error", fmt.Sprintf("proxies[%d]", i), "节点缺少 name"})
              }
              if proxyNames[name] {
                  issues = append(issues, OutputIssue{"error", fmt.Sprintf("proxies[%d]", i), "节点名重复: " + name})
              }
              proxyNames[name] = true
              if typ != "direct" && typ != "dns" {
                  for _, k := range []string{"server", "port"} {
                      if _, ok := m[k]; !ok {
                          issues = append(issues, OutputIssue{"error", fmt.Sprintf("proxies[%d]", i), "缺少 " + k})
                      }
                  }
              }
              if proto, err := node.GetProtocol(typ); err == nil {
                  for _, f := range proto.FormSchema {
                      if f.Required {
                          if _, ok := m[f.Name]; !ok {
                              issues = append(issues, OutputIssue{"error", fmt.Sprintf("proxies[%d]", i),
                                  typ + " 缺少必填字段 " + f.Name})
                          }
                      }
                  }
              }
          }
      }
      // 2) 合法名集合 = proxies 名 ∪ 组名 ∪ provider 名 ∪ 内置策略。
      // 【源码事实】CVR cleanup_proxy_groups 白名单为 DIRECT/REJECT/REJECT-DROP/PASS；
      // 本项目另保留 COMPATIBLE（AGENTS/Design2 既有口径）。
      allowed := map[string]bool{
          node.ReservedDirect: true, node.ReservedReject: true,
          node.ReservedRejectDrop: true, node.ReservedPass: true, node.ReservedCompatible: true,
      }
      // 收集 proxies[*].name、proxy-groups[*].name、proxy-providers 键。
      // 3) 每个组：type ∈ select/url-test/fallback/load-balance/relay；
      //    proxies/use 引用必须命中 allowed；select 组最终成员不能为空。
      // 4) rules：解析 TYPE,value,target[,no-resolve]；target 必须命中 allowed。
      // 5) warning 级：GEOIP,CN,DIRECT / MATCH,兜底组缺失（当前装配器总会输出，此处防回归）。
      return issues
  }

  func HasError(issues []OutputIssue) bool {
      for _, it := range issues {
          if it.Severity == "error" {
              return true
          }
      }
      return false
  }
  ```

  > 说明：`node.GetProtocol` 的 Required 字段清单即 Step 4 扩展后的同一注册表；Step 4 完成后自检能力自动增强。

  3. `backend/internal/assembly/models.go`：`RenderResult` 增加 `Issues []OutputIssue`。
  4. `backend/internal/assembly/render_clash.go` / `clash_plan.go`：序列化改为 `marshalClashYAML(doc, comments)`，不再调用 `yaml.Marshal`，也不再调用 `restoreYAMLUnicodeEscapes`：

  ```go
  content, err := marshalClashYAML(doc, comments)
  if err != nil { return nil, fmt.Errorf("序列化 Clash YAML 失败: %w", err) }
  res := &RenderResult{Content: content, Skipped: skipped, RenderPlan: planRaw}
  res.Issues = CheckClashContent(content)
  ```

  5. `backend/internal/assembly/service.go`：`Preview` 把 issues 追加到 Warnings；`Render` 保持纯渲染。
  6. `backend/internal/server/assembly.go`：`generate` 在 `Render` 成功后、创建版本前调用 `assembly.HasError(res.Issues)`，error 级返回 400。

  ```go
  if assembly.HasError(res.Issues) {
      Fail(c, http.StatusBadRequest, "产物自检未通过："+res.Issues[0].Message)
      return
  }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/assembly ./internal/server
  ```

- **验收标准：**
  - `TestClashEmojiRaw`：生成含 🚀/🌎/🛟/😀 节点的 Clash YAML，输出包含真实 emoji 原文，不出现 `\U0001F680`；用 `gyaml.UnmarshalWithOptions(content, &doc, gyaml.UseOrderedMap())` 回读名称一致；
  - `TestGoccyAutoInt`：`float64(7890)` 输出为 `port: 7890`，不是 `7890.0`；
  - `TestGoccyCommentMap`：有 proxies 时 `# {{xray_nodes}}` 出现在首个 proxy 前；无 proxies 时出现在 `proxies` 键前，且注释不会变成 `##`；
  - `TestSelfCheckRejectsDangling`：组引用不存在节点/规则目标不存在/空 select 组返回 error 级 issue；Preview 返回 warnings；Generate 返回 400；
  - `TestSelfCheckPassesGenerated`：合法最小 Clash 产物 0 个 error；
  - 既有 `TestClashHeaderOrder` 等按 goccy 输出语义等价文本更新后继续通过。

---

### Step 3：订阅响应头语义层与 RFC 5987 文件名（Clash Verge 导入兼容）

- **目标：** 让 Clash Verge Rev 的远程订阅下载得到“正确文件名 + 可解析的 profile 头”；避免非 ASCII 文件名在 HTTP 头中裸传。
- **前置条件：** Step 2（可选，建议同批）。
- **产出文件与操作：**

  1. `backend/internal/download/download.go`：新增系统生成的 Content-Disposition 构造器与 RFC 5987 百分号编码。

  ```go
  // rfc5987Value 对 UTF-8 字节做 RFC 5987 百分号编码（保留 [a-zA-Z0-9] 与 !#$&+-.^_`|~）。
  func rfc5987Value(s string) string {
      const hex = "0123456789ABCDEF"
      var b strings.Builder
      for i := 0; i < len(s); i++ {
          c := s[i]
          if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
              strings.ContainsRune("!#$&+-.^_`|~", rune(c)) {
              b.WriteByte(c)
              continue
          }
          b.WriteByte('%')
          b.WriteByte(hex[c>>4])
          b.WriteByte(hex[c&0x0f])
      }
      return b.String()
  }

  // BuildContentDisposition 生成 CVR 可优先解析的双形态头。
  // 【源码事实】CVR 先解析 filename*（RFC 5987 百分号解码），没有才回退 filename。
  func BuildContentDisposition(displayName, fallback string) string {
      if fallback == "" { fallback = "subscription.yaml" }
      return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
          sanitizeFilename(fallback), rfc5987Value(displayName))
  }
  ```

  2. `backend/internal/server/download.go`：系统生成的 Content-Disposition 在平台 `extra_headers` **之后**设置（系统头覆盖旧模板值）。三个下载端点的顺序统一改为：

  ```go
  for k, v := range res.ExtraHeaders {
      c.Header(k, v)
  }
  if res.Filename != "" {
      // fallback 用资源类型默认扩展名，displayName 用动态文件名
      c.Header("Content-Disposition", download.BuildContentDisposition(res.Filename, fallbackName(res)))
  }
  ```

  3. `backend/internal/platform/platform.go`：`ValidateExtraHeaders` 增加**已知头语义校验**，防止写入 CVR 无法解析的 profile 头：

  ```go
  func validateKnownHeader(k, v string) error {
      switch strings.ToLower(k) {
      case "profile-update-interval":
          // 【源码事实】CVR 按 u64 解析，单位小时，乘以 60 转分钟；0 会被解析，
          // 但 CVR 调度层以 interval>0 过滤，等价于“不自动更新”，故允许 0。
          if _, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64); err != nil {
              return errors.New("profile-update-interval 必须是非负整数小时（u64）")
          }
      case "profile-web-page-url":
          v = strings.TrimSpace(v)
          // 平台附加头允许 {frontend_url} 占位符；下载时由 withPlatformHeaders 替换为真实地址。
          if v == "{frontend_url}" { return nil }
          u, err := url.Parse(v)
          if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
              return errors.New("profile-web-page-url 必须是带 host 的 http/https 地址或 {frontend_url}")
          }
      case "subscription-userinfo":
          // upload/download/total/expire 值均须为非负整数；未知键/缺失项由 CVR 忽略，本系统只校验已出现项。
          for _, part := range strings.Split(v, ";") {
              kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
              if len(kv) != 2 { return errors.New("subscription-userinfo 格式须为 key=value; ...") }
              if _, err := strconv.ParseUint(strings.TrimSpace(kv[1]), 10, 64); err != nil {
                  return fmt.Errorf("subscription-userinfo 的 %s 必须是非负整数", kv[0])
              }
          }
      }
      return nil
  }
  ```

  4. `backend/internal/setup/setup.go`：默认 Clash Verge 平台的 `extra_headers` 去掉手写 `Content-Disposition`（文件名由下载端点动态生成，避免旧模板写死 `subscription.yaml`）；保留 `profile-update-interval` 与 `profile-web-page-url`。既有数据库中的旧模板无需迁移：系统生成的 Content-Disposition 最后写入，天然覆盖平台旧值。
  5. `frontend/src/views/admin/PlatformEditView.vue`：附加响应头区增加「Clash 生态预设」快捷行：`profile-update-interval`（小时，允许 0）、`profile-web-page-url`（允许 `{frontend_url}`）、`subscription-userinfo` 三键的可视化开关/输入；手工键值编辑器保留。保存时仍以键值对提交，前端按行内校验即时提示，与后端 `validateKnownHeader` 同口径。
  6. `backend/internal/download/download.go`：高级模式系统注入的 `subscription-userinfo` 改用 `strings.Join` 已有逻辑不变；仅补 `expire` 可由 `users.expire_at` 推导的 TODO 注释，不改变本步语义。

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/download ./internal/server ./internal/platform
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 下载响应同时含 `filename` 与 `filename*=UTF-8''...`，中文/emoji 文件名被百分号编码；
  - 平台手写 `Content-Disposition` 不覆盖系统生成值（系统值最后写入）；
  - `profile-update-interval: -1/abc`、`profile-web-page-url: ftp://x` 等被 400 拒绝；`profile-update-interval: 0` 与 `profile-web-page-url: {frontend_url}` 被接受；
  - `subscription-userinfo` 中 `upload=-1` 或非整数被 400 拒绝；
  - 默认 Clash Verge 平台仍下发 `profile-update-interval: 6` 与合法主页头。


### Step 4：节点协议注册表 / 字段补全 + Clash 渲染与 URI 参数对齐

- **目标：** 以 CVR `src/types/global.d.ts` 的字段定义为准，把 manual 协议注册表从“能跑的最小字段集”补到“与 CVR 编辑/渲染语义对齐”；新注册表为唯一口径，旧库中的非 mihomo 原生字段（扁平 `path/host`、`mport/insecure/allow_insecure/allowInsecure/address` 等）不做兼容，当前数据库允许重新初始化（用户确认）。
- **前置条件：** Step 2（自检依赖注册表 Required 字段）。
- **产出文件与操作：**

  1. `backend/internal/node/registry.go`：`FieldSchema` 增加 `text-list`（逗号分隔 → 数组）与 `int-list` 类型；新增公共基础字段；协议字段按下列差异补全。

  ```go
  type FieldSchema struct {
      Name     string   `json:"name"`
      Type     string   `json:"type"` // text / password / number / bool / select / object / text-list / int-list
      Required bool     `json:"required"`
      Default  any      `json:"default,omitempty"`
      Label    string   `json:"label"`
      Help     string   `json:"help,omitempty"`
      Options  []string `json:"options,omitempty"`
  }

  // 公共基础字段（【源码事实】IProxyBaseConfig）
  func commonFieldSchema() []FieldSchema {
      return []FieldSchema{
          {Name: "tfo", Type: "bool", Default: false, Label: "TCP Fast Open"},
          {Name: "mptcp", Type: "bool", Default: false, Label: "MPTCP"},
          {Name: "interface-name", Type: "text", Label: "出站网卡"},
          {Name: "routing-mark", Type: "number", Label: "路由标记"},
          {Name: "ip-version", Type: "select", Default: "dual", Label: "IP 版本",
              Options: []string{"dual", "ipv4", "ipv6", "ipv4-prefer", "ipv6-prefer"}},
          {Name: "dialer-proxy", Type: "text", Label: "拨号代理"},
      }
  }

  // 示例：vless 在现有基础上补齐 packet-addr/xudp/packet-encoding/skip-cert-verify/fingerprint/
  // http-opts/h2-opts/grpc-opts/ws-opts/xhttp-opts/ws-path/ws-headers/smux/encryption。
  // 字段名与 CVR IProxyVlessConfig 逐字一致；transport 对象用 FieldSchema.Type=="object"。
  ```

  **本轮字段差异表（CVR → 本项目 registry）：**

  | 协议 | 新增/调整字段 |
  |------|---------------|
  | ss | `plugin`（select: 空/obfs/v2ray-plugin/shadow-tls/restls）、`plugin-opts`（object）、`udp-over-tcp`、`udp-over-tcp-version`、`client-fingerprint`、`smux` |
  | vmess | `packet-addr`、`xudp`、`packet-encoding`、`skip-cert-verify`、`fingerprint`、`reality-opts`、`http-opts`、`h2-opts`、`grpc-opts`、`ws-opts`、`global-padding`、`authenticated-length`、`smux`；删除扁平 `path/host`（transport 只走 `ws-opts/http-opts` 等对象） |
  | vless | 见上方代码注释；`reality-opts` 对象为唯一口径，不再保留既有顶层 `public-key/private-key/short-id` 双形态 |
  | trojan | `alpn` 改 text-list、`fingerprint`、`network`、`reality-opts`、`grpc-opts`、`ws-opts`、`ss-opts`、`client-fingerprint` |
  | hysteria / hysteria2 | `ports`、`hop-interval`、`up`/`down`、`obfs-protocol`、`skip-cert-verify`、`fingerprint`、`ca`/`ca-str`、`recv-window*`、`fast-open`、`cwnd`、`udp-mtu` 等按协议子集补齐 |
  | tuic | `token`、`ip`、`heartbeat-interval`、`reduce-rtt`、`request-timeout`、`udp-relay-mode`、`congestion-controller`、`disable-sni`、`max-open-streams`、`cwnd`、`ca`/`ca-str`、`recv-window*`、`disable-mtu-discovery`、`udp-over-stream*` |
  | wireguard | `ip`、`ipv6`、`workers`、`persistent-keepalive`、`peers`（object）、`remote-dns-resolve`、`refresh-server-ip-interval`；`allowed-ips` 改 text-list（逗号分隔），渲染时输出数组 |
  | http / socks5 | http 增加 `sni`、`fingerprint`、`headers`（object）；socks5 增加 `fingerprint`（CVR `IProxySocks5Config` 无 `sni`，不新增） |
  | snell | `udp` |
  | anytls | `certificate`、`ech-opts`、`idle-session-check-interval`、`idle-session-timeout`、`min-idle-session`；`alpn` 改 text-list |
  | mieru / masque | 按 Reference 2 全量补齐（transport/multiplexing/handshake-mode；ip/ipv6/mtu/remote-dns-resolve/dns 等） |
  | ssh | `password`、`private-key-passphrase`、`host-key`、`host-key-algorithms` |
  | openvpn / shadowquic / trusttunnel / tailscale | 维持现状（CVR 无对应 interface，本项目独有） |

  **删除旧键（v1.3，不做兼容）：** 新 schema 不得再声明 hysteria 的 `mport/insecure`、hysteria2 的 `insecure`、tuic 的 `allow_insecure`、anytls 的 `allowInsecure`、wireguard 的 `address` 等旧键；`mergeSensitive` 的“仅保留 schema 字段”过滤可自然清理重建后的数据。

  2. `backend/internal/node/node.go`：敏感字段支持**点路径**（如 `plugin-opts.password`、`private-key-passphrase`），加密/合并/解密函数统一走 `getPath/setPath`；`validateProtocolFields` 支持 `text-list/int-list/object` 的 JSON 类型校验。

  ```go
  // getPath/setPath 支持 "a.b" 路径；中间值不存在时创建 map[string]any。
  func getPath(m map[string]any, path string) (any, bool) { /* 按 strings.Split(path, ".") 逐层 */ }
  func setPath(m map[string]any, path string, v any) { /* 同上，中间层自动建 map */ }

  // encryptProtocolJSON：遍历 proto.SensitiveFields，对点路径加密。
  // mergeSensitive：同一路径语义；edit 留空 = 保留旧密文。
  ```

  3. `backend/internal/assembly/render_clash.go`：`clashProxy` 与 `dynamicClashProxy` 先做字段归一化，保证输出给 mihomo 的是数组/对象而不是逗号字符串；新注册表之外的历史字段因数据库可重建，不再做旧键兼容。

  ```go
  // normalizeClashFields：把 registry 标注为 text-list 的字符串切分为 []string；
  // int-list 切分为 []int；其余原样透传（Clash 零转换渲染）。
  func (s *Service) normalizeClashFields(protocol string, m map[string]any) map[string]any {
      out := make(map[string]any, len(m))
      for k, v := range m {
          out[k] = v
      }
      proto, err := node.GetProtocol(protocol)
      if err != nil { return out }
      for _, f := range proto.FormSchema {
          switch f.Type {
          case "text-list":
              if str, ok := out[f.Name].(string); ok && str != "" {
                  parts := strings.Split(str, ",")
                  for i := range parts { parts[i] = strings.TrimSpace(parts[i]) }
                  out[f.Name] = parts
              }
          case "int-list":
              if str, ok := out[f.Name].(string); ok && str != "" {
                  out[f.Name] = parseIntList(str)
              }
          }
      }
      return out
  }
  ```

  4. `backend/internal/assembly/links/links.go`：链接生成补齐 transport/Reality/插件参数；`realityOpts` 只读 `reality-opts` 对象（v1.3：删除顶层 `public-key/private-key/short-id` 旧口径）；`genericLink` 的 vmess 补 `udp`，vless 读取 `ws-opts/h2-opts/grpc-opts/http-opts/xhttp-opts` 输出 `type/path/host/serviceName/mode` 等标准 query；`srLink` 的 vmess/vless 在 SR 形态参数基础上补 `tfo`（若存在）等 CVR URI parser 能回读的键。

  ```go
  // 伪代码：vless 标准链接的 transport 段
  func vlessTransportQuery(p map[string]any, q url.Values) {
      switch str(p, "network", "tcp") {
      case "ws":
          if ws, ok := p["ws-opts"].(map[string]any); ok {
              q.Set("path", str(ws, "path", "/"))
              if h, ok := ws["headers"].(map[string]any); ok {
                  if host, ok := h["Host"].(string); ok && host != "" { q.Set("host", host) }
              }
          }
      case "grpc":
          if grpc, ok := p["grpc-opts"].(map[string]any); ok {
              q.Set("serviceName", str(grpc, "grpc-service-name", ""))
          }
      // h2/http/xhttp 同理，键名与 CVR uri-parser/vless.ts 的回读键一致
      }
  }
  ```

  5. `frontend/src/views/admin/NodesView.vue`：动态表单支持 `text-list`（Input + helper 文案“逗号分隔”）与 `int-list`；`object` 字段继续沿用失焦 JSON.parse 的现有处理；敏感字段密文空值提示沿用。
  6. 兼容策略：**不做旧字段兼容**（用户确认当前数据库可重新初始化）。`mergeSensitive` 现有“仅保留新协议 schema 声明字段”的过滤继续保留，使更新/重建后的节点只含注册表字段；不实现 `normalizeLegacyTransport`，旧扁平 `path/host` 节点在重建数据库后自然消失。

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/node ./internal/assembly ./internal/assembly/links
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 每个协议 `ManualProtocols()` 返回的字段与 CVR interface 的映射表核对通过（新增字段齐全、敏感路径正确）；
  - Clash YAML 中 `allowed-ips/alpn` 等以数组形态输出；`ws-opts`/`grpc-opts` 结构体输出正确；
  - `plugin-opts.password` 等嵌套敏感字段加密存储、编辑留空保留旧密文；
  - URI 生成的 vless WS/gRPC 链接可被本地 Go 端到端反解析（Build10 Step 2 的 uriparse 复用作回读器）；
  - 数据库重建后，按新注册表创建的节点 Clash 渲染不产生 `path/host/mport/insecure/allow_insecure/allowInsecure` 等旧键。

---

### Step 5：规则类型与素材池解析扩展（mihomo 规则全集）

- **目标：** 把规则白名单从 8 类扩展到 CVR `rules-editor-viewer.tsx` 所列的 mihomo 全集，并为每条规则维护 `no-resolve` 与取值校验元数据。
- **前置条件：** Step 2；Build10 Step 1 覆盖层会复用本步元数据。
- **产出文件与操作：**

  1. 新增 `backend/internal/rulespec/spec.go`（无 DB 依赖的纯元数据包，供 `pool` 与 `assembly` 共用）：

  ```go
  package rulespec

  type RuleDef struct {
      SR            bool   // Shadowrocket 是否支持
      NoResolve     bool   // Clash 渲染是否可追加 no-resolve（SR 渲染不适用）
      ValueRequired bool   // false 仅 MATCH
      ValueLabel    string
  }

  // 类型全集取自 CVR rules-editor-viewer.tsx（【源码事实】）。
  // ValueRequired 为 false 仅 MATCH；其余 33 类必须显式写 true（Go 零值陷阱）。
  // NoResolve 为 Clash/mihomo 渲染元数据；SR 渲染仍按既有口径仅 IP-CIDR/IP-CIDR6 追加。
  var Definitions = map[string]RuleDef{
      "DOMAIN":             {SR: true, ValueRequired: true},
      "DOMAIN-SUFFIX":      {SR: true, ValueRequired: true},
      "DOMAIN-KEYWORD":     {SR: true, ValueRequired: true},
      "DOMAIN-REGEX":       {SR: false, ValueRequired: true}, // 【推断】SR 不支持，Clash 渲染保留
      "GEOSITE":            {SR: false, ValueRequired: true},
      "GEOIP":              {SR: true, ValueRequired: true, NoResolve: true},
      "SRC-GEOIP":          {SR: false, ValueRequired: true},
      "IP-ASN":             {SR: false, ValueRequired: true, NoResolve: true},
      "SRC-IP-ASN":         {SR: false, ValueRequired: true},
      "IP-CIDR":            {SR: true, ValueRequired: true, NoResolve: true},
      "IP-CIDR6":           {SR: true, ValueRequired: true, NoResolve: true},
      "SRC-IP-CIDR":        {SR: false, ValueRequired: true},
      "IP-SUFFIX":          {SR: false, ValueRequired: true, NoResolve: true},
      "SRC-IP-SUFFIX":      {SR: false, ValueRequired: true},
      "SRC-PORT":           {SR: false, ValueRequired: true},
      "DST-PORT":           {SR: false, ValueRequired: true},
      "IN-PORT":            {SR: false, ValueRequired: true},
      "DSCP":               {SR: false, ValueRequired: true},
      "PROCESS-NAME":       {SR: true, ValueRequired: true},
      "PROCESS-PATH":       {SR: false, ValueRequired: true},
      "PROCESS-NAME-REGEX": {SR: true, ValueRequired: true},
      "PROCESS-PATH-REGEX": {SR: false, ValueRequired: true},
      "NETWORK":            {SR: false, ValueRequired: true},
      "UID":                {SR: false, ValueRequired: true},
      "IN-TYPE":            {SR: false, ValueRequired: true},
      "IN-USER":            {SR: false, ValueRequired: true},
      "IN-NAME":            {SR: false, ValueRequired: true},
      "SUB-RULE":           {SR: false, ValueRequired: true},
      "RULE-SET":           {SR: false, ValueRequired: true, NoResolve: true},
      "AND":                {SR: false, ValueRequired: true},
      "OR":                 {SR: false, ValueRequired: true},
      "NOT":                {SR: false, ValueRequired: true},
      "MATCH":              {SR: false, ValueRequired: false}, // SR 使用 FINAL，不接受 MATCH 条目
      "USER-AGENT":         {SR: true, ValueRequired: true},   // Clash 渲染跳过并提示，沿用现有口径
  }

  // ValidateValue 按类型校验：IP-ASN/SRC-IP-ASN/UID 数值；端口 1-65535；
  // NETWORK ∈ tcp/udp；CIDR 类用 net.ParseCIDR；DOMAIN-REGEX 用 regexp.Compile（RE2）；
  // AND/OR/NOT 至少校验括号配对；MATCH 值必须为空。
  ```

  2. `backend/internal/pool/parser.go` / `sync.go`：`ParseLine` 增加逻辑规则与 MATCH 的处理；`ValidateEntry` 改为调 `rulespec`；`parseURLBody` 在 `ok==true && reason!=""` 时把 reason 作为信息提示追加（不计数、不跳过），保证“行内 policy 已忽略”进入同步回执。

  ```go
  // 【源码事实】CVR 规则编辑器对 AND/OR/NOT 保存的是含逗号的完整表达式；
  // 现有 SplitN(line, ",", 3) 会截断这些类型，必须按括号配平解析。
  // 返回 (ruleType, matchValue, skip, valid)；第三返回值保持为 skip 原因。
  // 逻辑规则若带行内 policy，先按括号配平剥离该 policy（URL 池目标由装配层另行指定）。
  func ParseLine(raw string) (string, string, string, bool) {
      // 去注释/空行逻辑不变...
      typ := strings.ToUpper(strings.TrimSpace(parts[0]))
      if typ == "AND" || typ == "OR" || typ == "NOT" {
          rest := strings.TrimSpace(line[len(parts[0])+1:])
          if rest == "" { return "", "", "逻辑规则缺少表达式", false }
          if !strings.HasPrefix(rest, "(") { return "", "", "逻辑规则表达式必须以 ( 开头", false }
          // 括号配平定位表达式结束位置；end 为匹配右括号下标。
          end := balancedCloseParen(rest)
          if end < 0 { return "", "", "逻辑规则括号不配对", false }
          expr := strings.TrimSpace(rest[:end+1])
          remainder := strings.TrimSpace(rest[end+1:])
          // 仅当匹配右括号之后恰好还有 ,policy 时剥离末尾 target；
          // ParseLine 第三返回值为 skip 原因：URL 池目标由装配层指定，因此丢弃行内 policy 并回执。
          if strings.HasPrefix(remainder, ",") {
              target := strings.TrimSpace(strings.TrimPrefix(remainder, ","))
              if target == "" { return "", "", "逻辑规则目标为空", false }
              return typ, expr, "逻辑规则末尾 policy 已忽略（目标由装配层指定）", true
          }
          if remainder != "" { return "", "", "逻辑规则表达式后存在无法识别的尾部", false }
          return typ, expr, "", true
      }
      if typ == "MATCH" {
          return "MATCH", "", "", true
      }
      // 其余沿用现有两段解析
  }
  ```

  `ValidateEntry` 同步调整：逻辑规则允许表达式内逗号（以括号配平结果为准）；其余类型维持现有“匹配值禁止逗号/控制字符”口径。

  3. `backend/internal/assembly/validate.go`：删除 `validRuleTypes`，使用 `rulespec.Definitions`；Clash 允许全部 34 类（CVR 33 类 + 本项目保留的 USER-AGENT；USER-AGENT 在渲染层跳过并提示），SrConf 只允许 `SR==true` 的类型（`MATCH` SR=false，SR 继续使用 `FINAL`），对不支持类型在 Preview/Generate 回执中列出（跳过语义与 USER-AGENT 同口径）。
  4. `backend/internal/assembly/render_clash.go`：`appendRule` 使用 `RuleDef.NoResolve`；MATCH 输出 `MATCH,<target>`（无匹配值）；逻辑规则输出 `TYPE,<表达式>,<target>`。
  5. `backend/internal/assembly/render_sr.go`：SR 渲染**不使用** CVR 的 `NoResolve` 全集，保持既有口径仅 `IP-CIDR/IP-CIDR6` 追加 `no-resolve`；逻辑/MATCH 等非 SR 类型跳过并回执。
  6. `backend/internal/assembly/selfcheck.go`：规则行解析改用 `rulespec` 共享逻辑（逻辑规则按括号配平解析），规则目标校验、`no-resolve` 合法性与必填值校验统一消费 `RuleDef`，补充对应测试。
  7. `frontend/src/views/admin/AssemblyView.vue`：`RULE_TYPES` 扩展为 34 类；Clash 下拉排除 `USER-AGENT`（既有逻辑保持），SR 下拉由 `rulespec` 的 SR 集合映射（后端 context 可增加 `rule_type_options` 以避免前端重复维护，二者取其一，建议后端下发）。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/rulespec ./internal/pool ./internal/assembly
  ```

- **验收标准：**
  - `GEOSITE`、`RULE-SET`、`SRC-*`、端口类、逻辑类规则可入库/渲染；MATCH 在 Clash 输出无空逗号，且 SrConf 拒绝 MATCH 条目；
  - Clash 中 IP-CIDR/IP-CIDR6 保持 `no-resolve`；新增 `GEOIP/IP-ASN/IP-SUFFIX/SRC-IP-CIDR/RULE-SET` 的 Clash `no-resolve` 行为符合 CVR 元数据；SR 渲染仍仅 IP-CIDR/IP-CIDR6 带 `no-resolve`；
  - AND/OR/NOT URL 源解析不再被截断，行内 policy 被剥离并在回执说明；SrConf 对不支持类型跳过并提示；
  - 每个非 MATCH 规则类型缺匹配值均被 `rulespec` 拒绝（防止 `ValueRequired` 零值失效）；
  - 既有 8 类规则行为不回归。


### Step 6：代理组类型与字段扩展（load-balance / relay / use / 健康检查等）

- **目标：** 把全局代理组定义从 `select/url-test/fallback + groups[]` 扩展到 CVR `IProxyGroupConfig` 的 5 类型与常用渲染字段。
- **前置条件：** Step 2；Build10 Step 1 依赖本步的 `Definition` 与 `ClashPlanGroup` 结构。
- **产出文件与操作：**

  1. `backend/internal/proxygroup/proxygroup.go`：`validGroupTypes` 增加 `load-balance`、`relay`；`Definition` 增加字段（全部带 `omitempty`，零值不输出）。

  ```go
  type Definition struct {
      GroupType            string   `json:"type"`
      Groups               []string `json:"groups"` // 节点引用已移至装配时，本字段仍是子组引用
      Use                  []string `json:"use,omitempty"`
      URL                  string   `json:"url,omitempty"`
      ExpectedStatus       string   `json:"expected-status,omitempty"`
      Interval             int      `json:"interval,omitempty"`
      Timeout              int      `json:"timeout,omitempty"`
      MaxFailedTimes       int      `json:"max-failed-times,omitempty"`
      Lazy                 bool     `json:"lazy,omitempty"`
      DisableUDP           bool     `json:"disable-udp,omitempty"`
      InterfaceName        string   `json:"interface-name,omitempty"`
      RoutingMark          int      `json:"routing-mark,omitempty"`
      Filter               string   `json:"filter,omitempty"`
      ExcludeFilter        string   `json:"exclude-filter,omitempty"`
      ExcludeType          string   `json:"exclude-type,omitempty"`
      IncludeAll           bool     `json:"include-all,omitempty"`
      IncludeAllProxies    bool     `json:"include-all-proxies,omitempty"`
      IncludeAllProviders  bool     `json:"include-all-providers,omitempty"`
      Hidden               bool     `json:"hidden,omitempty"`
      Icon                 string   `json:"icon,omitempty"`
  }
  ```

  `validateDefinition` 规则调整：
  - `select` 组必须至少满足 `len(Groups)>0 || len(Use)>0 || IncludeAll*` 之一；
  - `url-test/fallback/load-balance` 允许配置 `url/interval/timeout/max-failed-times/expected-status`；`url` 只允许 http/https；
  - `relay` 组保持现有 DAG 校验，`Groups` 为代理链；
  - `exclude-type` 按 `|` 分割并对照 CVR 的枚举名白名单（Direct/Reject/.../Ssh，不区分大小写）；
  - `Use` 不校验具体 provider 存在性（provider 可能由 Build10 Step 1 的 Merge 覆盖层注入），生成/下载自检时校验。

  2. `backend/internal/assembly/models.go` / `load.go`：`groupData` 与 `ClashPlanGroup` 增加相同字段；`loadGroups` 解析新的 `definition_json`。
  3. `backend/internal/assembly/render_clash.go` / `clash_plan.go`：渲染 `proxy-groups` 条目按固定键序输出：

  ```go
  // Clash 组输出键序（【推断】，参考 CVR IProxyGroupConfig 常见书写顺序）：
  // name, type, proxies, use, url, expected-status, interval, timeout,
  // max-failed-times, lazy, disable-udp, interface-name, routing-mark,
  // filter, exclude-filter, exclude-type, include-all, include-all-proxies,
  // include-all-providers, hidden, icon
  func orderedGroupFields(g *ClashPlanGroup) *OrderedMap {
      p := NewOrderedMap().Set("name", g.Name).Set("type", g.Type)
      if len(g.Proxies) > 0 { p.Set("proxies", g.Proxies) }
      if len(g.Use) > 0 { p.Set("use", g.Use) }
      // ...其余非零字段按上面键序 Set...
      return p
  }
  ```

  4. `RenderClashPlan` 的 `clashGroupReachable` 同步考虑 `Use`：组若有 `use` 且最终 providers 非空，即可达（否则下载渲染会误删组）。
  5. `backend/internal/assembly/selfcheck.go`：同步消费 `proxygroup` 的组类型/字段元数据——空 select 组仅在既无 `proxies` 又无 `use/include-all*` 时判 error；`use` 引用命中最终 `proxy-providers`；补充对应测试。
  6. `frontend/src/views/admin/ProxyGroupsView.vue`：组类型下拉增加 `load-balance/relay`；编辑弹窗增加“高级字段”折叠区：`use`（逗号分隔标签输入）、健康检查四件套、`filter/exclude-filter/exclude-type/include-all*/disable-udp/hidden/icon`；预设组与自建组共用同一表单，name/preset_key 仍不可改。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/proxygroup ./internal/assembly
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 5 种组类型可创建/编辑；非法 `url`、未知 `exclude-type` 被 400 拒绝；
  - Clash YAML 输出完整 `url-test/fallback/load-balance/relay` 字段，且 `select` 组不再强制要求 `groups` 非空（有 `use` 或 include-all 即合法）；
  - `RenderClashPlan` 不会删除仅有 `use` 可达的组；`CheckClashContent` 不把合法 `use/include-all` 组误判为空 select；
  - 历史 `definition_json`（只有 type/groups）加载与渲染不回归。

---
## 七、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-27 | 基于 clash-verge-rev 本地源码 `3503a2d`、三份 Reference 文档与 Issue7 开放项研究生成 Build9；含事实基线、假设清单、候选构建项与 Step 1~11 分步计划；未改动任何代码与既有文档。 |
| v1.1 | 2026-08-27 | 深入核验后修订：以当前 HEAD `c681742` 对齐代码；标记 R22-02/06/07/08 与 R22-01 主体已实施；Step 1 收窄为 R22-03 条件化目标区；Step 9 收窄为启动补跑；新增 §4.5 核验结论与用户确认（Emoji 小实验、完整控制面保护、URI 导入同名跳过并回执）；同步候选与文档收口口径。未改动任何代码。 |
| v1.2 | 2026-08-27 | Step2 方案定稿：选择方案B（goccy 全量迁移），接受语义等价文本差异，注释用 CommentMap；方案A（yaml.v3 + restoreYAMLUnicodeEscapes）保留为回退记录；Step7/后续 YAML 解析同步改为 goccy MapSlice + UseOrderedMap；更新 Step2、Step7、Step10 与变更记录。未改动任何代码。 |
| v1.3 | 2026-08-27 | 二次深入核验后修订（仅改本文件，未改代码）：Step1 纳入两个过期前端测试修复；Step3 响应头校验兼容 `{frontend_url}`、interval 允许 0、subscription-userinfo 非负整数；Step4 明确不做旧字段兼容并修正 socks5/sni；Step5 修复 `ValueRequired` 零值、SR 支持面（GEOIP/MATCH）与逻辑规则括号配平解析，selfcheck 纳入共享元数据；Step6 selfcheck 同步 use/include-all；Step7 定稿“覆盖层先于可达性收敛”并补齐完整 CVR Merge/控制面语义；Step8 注明排除 ssr；Step10 同步新管线；更新事实 20、版本锚点与文件清单。 |
| v1.4 | 2026-08-27 | 按用户确认拆分：Build9 保留 Step 0~6，原 Step 7~11 拆至 [Build10.md](Build10.md) 并重编号为 Build10 Step 1~5；更新本卷进度表、文件清单、依赖图与交叉引用。仅改文档，未改任何代码。 |

