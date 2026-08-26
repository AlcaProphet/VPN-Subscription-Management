# Build9.md — VPN 订阅管理系统 当前构建方案（Clash Verge Rev 深度借鉴轮）

> **文档定位：** 本文档是 VPN 订阅管理系统的**下一轮当前构建方案**，承接已验收的 [Build8.md](Build8.md)（当前活跃）、已归档的 [docs/reports/Build/Build4~7](docs/reports/Build)，并基于：
> - 项目内研究资料：[docs/Reference/Clash-Subscription-Validation-Emoji-API.md](docs/Reference/Clash-Subscription-Validation-Emoji-API.md)、[docs/Reference/Clash-Verge-Rev-Node-Parameters.md](docs/Reference/Clash-Verge-Rev-Node-Parameters.md)、[docs/Reference/Clash-Verge-Rev-Subscription-Assembly.md](docs/Reference/Clash-Verge-Rev-Subscription-Assembly.md)；
> - 第三方代码仓库：`~/Desktop/Repo/clash-verge-rev`（本地 git 提交 `3503a2da29d68a4398c0b8e9234cffb711e65783`，2026-08-26）；
> - 当前项目代码与文档（本地 git 提交 `ce22a3143f14c4060689bc2cd806d13731ae3398`，2026-08-26）。
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
| 1 | 前置修复：Issue7 R22-02/03/06/07/08（版本 id、装配 UI、SR-conf 直建规则） | ☐ 未开始 |
| 2 | Clash 产物静态自检 + YAML Emoji/UTF-8 输出修复 | ☐ 未开始 |
| 3 | 订阅响应头语义层与 RFC 5987 文件名（Clash Verge 导入兼容） | ☐ 未开始 |
| 4 | 节点协议注册表/字段补全 + Clash 渲染与 URI 参数对齐 | ☐ 未开始 |
| 5 | 规则类型与素材池解析扩展（mihomo 规则全集） | ☐ 未开始 |
| 6 | 代理组类型与字段扩展（load-balance/relay/use/健康检查等） | ☐ 未开始 |
| 7 | 分层装配：Merge + Rules/Proxies/Groups 覆盖层（核心借鉴） | ☐ 未开始 |
| 8 | 节点批量 URI 导入（借鉴 ProxiesEditor 粘贴解析与去重） | ☐ 未开始 |
| 9 | 素材池同步长事务拆分（Issue7 R22-01）+ 启动补跑 | ☐ 未开始 |
| 10 | 版本文件原子写入与下载渲染覆盖层接线 | ☐ 未开始 |
| 11 | 前端/测试/文档/smoke 收口 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/version/version.go`、`backend/internal/assembly/{models.go,validate.go,load.go}`、`backend/internal/server/assembly.go`、`frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/assembly/{TypeTargetStep.vue,NodesGroupsStep.vue}`、`frontend/src/api/assembly.ts` 及测试 | 修复版本列表 `id=0`；目标选择上移；SR-conf 可直接创建规则；节点步骤 UI 收口 |
| 2 | `backend/internal/assembly/{selfcheck.go,yaml_text.go,render_clash.go,service.go,models.go}`、`backend/internal/assembly/*_test.go` | 静态自检、Emoji 还原、预览告警/生成阻断 |
| 3 | `backend/internal/download/download.go`、`backend/internal/server/download.go`、`backend/internal/platform/platform.go`、`backend/internal/setup/setup.go`、`frontend/src/views/admin/PlatformEditView.vue` 及测试 | RFC 5987 Content-Disposition、profile 头语义校验、系统头优先级 |
| 4 | `backend/internal/node/{registry.go,node.go}`、`backend/internal/assembly/links/links.go`、`backend/internal/assembly/render_clash.go`、`frontend/src/views/admin/NodesView.vue` 及测试 | 协议字段补全、嵌套敏感字段、数组字段归一化、传输参数链接 |
| 5 | `backend/internal/rulespec/spec.go`（新增）、`backend/internal/pool/parser.go`、`backend/internal/assembly/{validate.go,render_clash.go,render_sr.go}`、`frontend/src/views/admin/AssemblyView.vue` 及测试 | mihomo 规则全集、no-resolve 元数据、逻辑规则解析 |
| 6 | `backend/internal/proxygroup/proxygroup.go`、`backend/internal/assembly/{models.go,load.go,clash_plan.go,render_clash.go}`、`frontend/src/views/admin/ProxyGroupsView.vue` 及测试 | 组类型 5 枚举、use/健康检查/过滤等字段、渲染 |
| 7 | `backend/internal/assembly/{overlay.go,models.go,service.go,render_clash.go,clash_plan.go,blueprint.go}`、`frontend/src/views/admin/assembly/OverlayStep.vue`（新增）、`AssemblyView.vue` 及测试 | 四类覆盖层、控制面保护、清理悬空、排序、蓝图持久化与下载重渲染 |
| 8 | `backend/internal/uriparse/uriparse.go`（新增）、`backend/internal/node/uri_import.go`（新增）、`backend/internal/server/node.go`、`frontend/src/views/admin/NodesView.vue` 及测试 | URI 批量导入、逐行跳过与去重、事务批量创建 |
| 9 | `backend/internal/pool/sync.go`、`backend/internal/pool/pool.go`、`backend/internal/cron/pool.go` 及测试 | 同步短事务分批、keep 表索引、启动补跑错过的同步 |
| 10 | `backend/internal/version/version.go`、`backend/internal/server/render.go`、`backend/internal/download/download.go` 及测试 | 版本文件 temp+rename 原子写、Clash 下载渲染应用覆盖层 |
| 11 | `frontend/src/views/admin/assembly/`、`frontend/tests/`、`.smoke-test.sh`、`.smoke-test-prod.sh`、`Design2.md`、`Design2-UI.md`、`AGENTS.md`、`Issue7.md`、`ProdTestList.md` | UI 集成、专项测试、smoke、文档状态回写 |

---

## 三、构建顺序依赖图

```
Step 0（本文档）
  → Step 1（Issue7 前置修复，解锁“重新编辑/覆盖层编辑”）
  → Step 2（输出自检与 Emoji，后续所有 Clash 改动的安全网）
  → Step 3（响应头兼容，可独立，但建议在 2 后）
  → Step 4（节点字段，Step 7/8 依赖其字段模型）
  → Step 5（规则元数据，Step 7 覆盖层与渲染依赖）
  → Step 6（代理组字段，Step 7 覆盖层与渲染依赖）
  → Step 7（分层装配覆盖层，核心）
  → Step 8（URI 导入，依赖 Step 4 注册表）
  → Step 9（池同步长事务，独立于 7，可与 8 并行观察）
  → Step 10（版本原子写 + 下载覆盖层接线，依赖 7/9）
  → Step 11（收口）
```

---

## 四、事实基线与关键差异（研究结论）

### 4.1 研究来源与版本

| 来源 | 标识/路径 | 说明 |
|------|-----------|------|
| 第三方客户端源码 | `~/Desktop/Repo/clash-verge-rev` @ `3503a2d`（2026-08-26） | 本文所有 CVR 事实的第一来源 |
| 本项目源码 | `~/Desktop/Repo/VPN-Subscription-Management` @ `ce22a31`（2026-08-26） | 当前实现对照 |
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
18. **Emoji 实测**：本项目 `gopkg.in/yaml.v3 v3.0.1` 会把 4 字节 emoji 输出为 `"\U0001F680..."`；Go yaml.v3 与 js-yaml 均能解析回原串；但为可读性与老解析器兼容，应还原真实 UTF-8。实测 `github.com/goccy/go-yaml v1.19.2`（当前 `go.mod` 中已有 indirect）原生输出真实 emoji，可作为替代方案。
19. **当前项目已有对齐点**：下载端点已注入 `profile-update-interval/profile-web-page-url/subscription-userinfo` 与禁缓存头；`RenderClashPlan` 已有可达性收敛；节点已有 `display_name/有效渲染名` 全局唯一；规则素材池已有 URL 同步、30 分钟超时、取消端点。
20. **当前项目主要差距**：无输出静态自检；Emoji 被 yaml.v3 转义；Content-Disposition 只有单行 `filename*`；协议字段明显少于 CVR；规则类型 8 类；组类型 3 类且无 `use/健康检查`；无 merge/seq 覆盖层；无 URI 导入；素材池同步仍单长事务（Issue7 R22-01）；版本列表缺 `id`（R22-02）。

### 4.3 关键【推断】（有事实依据，仍需测试确认）

- 对装配生成内容做“CVR 导入校验 + 引用/结构静态检查”的组合，可以等价覆盖绝大多数 `core -t` 能发现的配置错误；不能替代内核对协议字段语义的最终校验。【推断】
- 覆盖层在“基础蓝图 + 动态 Xray 节点注入之后”应用，才能保证 `delete` 与 `prepend/append` 对最终用户下载内容生效；这偏离了 CVR“seq 先于默认配置”的顺序，但适合本项目的动态渲染模型。【推断】
- 若在生成预览与下载重渲染使用同一覆盖层实现，`RenderClashPlan` 的自包含结构保持成立，历史蓝图缺省覆盖层时天然等价于当前行为。【推断】
- CVR 的 `use_sort` 输出顺序可作为本项目 Clash 输出的固定键序；管理员头部 JSON 的原始顺序不再作为唯一顺序依据，但可保留为未声明键的次序。【推断】
- 对平台 `extra_headers` 增加已知头语义校验不会破坏既有能力；系统生成的 Content-Disposition 应覆盖平台手填的旧模板值，否则无法提供动态订阅名。【推断】
- URI 导入的解析器可先覆盖 CVR `uri-parser` 支持的主流 10 类 scheme；snell/mieru/masque/openvpn/ssh 等无标准 URI 的协议继续保持“不可导入”并在回执中说明。【推断】

### 4.4 本轮【假设】清单（用户离开后按授权做出，实施前建议复核）

- **A1 兼容性不设限**：允许改动现有表结构、接口字段与前端布局；当前无活跃用户与有价值数据（用户确认口径）。因此 Step 7 的覆盖层存储、Step 9 的同步事务拆分可直接落地。
- **A2 不内置 mihomo 二进制**：本轮只做静态自检；“可选 `core -t` 真校验”进入候选列表，需用户决定二进制分发与资源占用。
- **A3 不实现 JS Script 扩展**：与 AGENTS“简单轻量化”冲突较大；Step 7 只实现 Merge + Rules/Proxies/Groups Seq 四层，Script 保留扩展点并进入候选。
- **A4 覆盖层首轮只服务 Clash YAML**：SR subs/generic-subs/sr-conf 暂不应用 Merge/Seq；共用层（规则元数据、响应头）仍顺带覆盖。
- **A5 Issue7 纳入范围裁剪**：R22-02/03/06/07/08 因与装配 UI/重新编辑强相关纳入 Step 1；R22-01 纳入 Step 9；R22-04/R22-05 与本轮主题弱相关，暂不纳入（保持 Issue7 开放，避免本 Build 范围失控）。
- **A6 池同步补跑口径**：当前池是“每日固定时刻”模型，不引入 per-pool 相对间隔列；启动补跑仅补偿“今天应跑但停机错过”的任务，保持现有数据模型简单。
- **A7 覆盖层输入为 YAML 文本**：与 CVR 一致，前端不做 JSON 表单化；后端统一 `yaml.v3` 解析并在预览时给出定位错误。
- **A8 Step 顺序可执行**：每步完成后均可编译测试，不跳步；执行者仍应遵守 AGENTS“每次仅执行一个 Step，验收后再下一步”。

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
| C8 | Issue7 R22-04 / R22-05 | 默认平台锁定、日志显示名切换；与本轮主题弱相关，留待 Issue7 或下一轮 | Issue7 |

---

## 六、分步构建计划

> 每步格式遵循 [docs/DocTemplates/Build.template.md](docs/DocTemplates/Build.template.md)：目标 → 前置条件 → 产出文件与操作（含参考代码）→ 测试与验收命令 → 验收标准。执行者每次仅执行一个 Step。

### Step 1：前置修复 — Issue7 R22-02 / R22-03 / R22-06 / R22-07 / R22-08

- **目标：** 先把 Issue7 已记录、用户已确认方向的装配入口缺陷修掉，否则 Step 7 的覆盖层“重新编辑”与 Step 8 的 UI 会叠加在坏底座上。本步不引入 CVR 借鉴层。
- **前置条件：** Step 0；Issue7 相关条目方向已确认（变更记录 v1.3/v1.6/v1.7/v1.8）。
- **产出文件与操作：**

  1. `backend/internal/version/version.go`：修复 R22-02。`ListVersions` 查询与扫描补上 `v.id`。

  ```go
  // 【源码事实】当前 SELECT 不含 v.id，导致前端拿到的 Version.ID 恒为 0，
  // “重新编辑”实际请求 /api/admin/versions/0/blueprint（Issue7 R22-02）。
  query := `SELECT v.id, v.version_no, v.file_path, v.file_name, v.created_at, v.updated_at`
  // ...hasBlueprint 分支不变...
  for rows.Next() {
      var v Version
      var blueprint int
      if hasBlueprint {
          if err := rows.Scan(&v.ID, &v.No, &v.FilePath, &v.FileName, &v.CreatedAt, &v.UpdatedAt, &blueprint); err != nil {
              return nil, fmt.Errorf("解析版本行失败: %w", err)
          }
      } else {
          if err := rows.Scan(&v.ID, &v.No, &v.FilePath, &v.FileName, &v.CreatedAt, &v.UpdatedAt); err != nil {
              return nil, fmt.Errorf("解析版本行失败: %w", err)
          }
      }
      v.Current = v.No == currentNo
      v.Blueprint = blueprint == 1
      out = append(out, v)
  }
  ```

  2. `backend/internal/assembly/models.go`：为 R22-08 增加 `RuleName`；Step 7 的 Overlay 字段同步预留（本步不实现逻辑）。

  ```go
  type GenerateInput struct {
      // ...现有字段...
      RuleName string `json:"rule_name,omitempty"` // sr-conf 直建规则名（Issue7 R22-08）
  }
  ```

  3. `backend/internal/assembly/validate.go` / `load.go`：SrConf 分支改为“已有 RuleID 或新规则名二选一”。

  ```go
  case SrConf:
      if ld.rule == nil {
          if strings.TrimSpace(in.RuleName) == "" {
              return fmt.Errorf("%w: 请选择规则实体或填写新规则名称", ErrBadRequest)
          }
          if utf8.RuneCountInString(strings.TrimSpace(in.RuleName)) > 100 {
              return fmt.Errorf("%w: 规则名称不超过 100 字符", ErrBadRequest)
          }
      }
  ```

  `loadData` 保持“`in.RuleID > 0` 才查库”，不把 `RuleName` 当 ID 使用。

  4. `backend/internal/server/assembly.go`：`resolveOwner` 支持直建规则；`generate` 对“新建规则后版本创建失败”做清理。

  ```go
  // resolveOwner 增加 createdRuleID 返回；新建规则在版本事务前落库，失败路径清理。
  func (h *AssemblyHandler) resolveOwner(ctx context.Context, in assembly.GenerateInput) (
      version.OwnerType, int64, string, int64, error) {
      if in.TargetSyntax == assembly.SrConf {
          if in.RuleID > 0 {
              return version.OwnerRule, in.RuleID, "rule.conf", 0, nil
          }
          // 【假设 A5】按 Issue7 R22-08 已确认方向：装配器直接创建空规则实体。
          r, err := h.ruleSvc.Create(ctx, strings.TrimSpace(in.RuleName), "", "shadowrocket", nil, nil)
          if err != nil {
              return "", 0, "", 0, fmt.Errorf("创建规则实体失败: %w", err)
          }
          return version.OwnerRule, r.ID, "rule.conf", r.ID, nil
      }
      // 非 sr-conf 沿用原逻辑...
  }

  // generate 中：
  ownerType, ownerID, fileName, createdRuleID, err := h.resolveOwner(ctx, in)
  // ...
  created, activated, err := h.versionSvc.CreateVersion(...)
  if err != nil {
      if createdRuleID > 0 {
          if cerr := h.ruleSvc.Delete(ctx, createdRuleID); cerr != nil {
              // 记 warn，不掩盖主错误
          }
      }
      Fail(c, http.StatusInternalServerError, err.Error())
      return
  }
  ```

  5. `frontend/src/views/admin/AssemblyView.vue`：
     - R22-03：`stepDefs` 不再含 `{ key: 'target' }`；新增顶部目标选择卡片（复用/改造 `TypeTargetStep.vue`，把它从 AssemblerShell 插槽中移出）。删除 `skipTargetStep` 过滤逻辑，`platform_id/rule_id` query 仅预填。
     - R22-08：`isSrConf` 目标区改为「规则实体（可选）+ 新建规则名称（二选一）」，`targetReady()` 改为 `!!form.rule_id || !!form.rule_name.trim()`；`buildPreflightMissing` 对 sr-conf 不再要求 `rules.length > 0`；`buildInput()` 传 `rule_name`。

  ```vue
  <!-- 顶部目标选择卡片（替换原步骤①） -->
  <Card v-if="!generateResult" size="small" class="mb-4">
    <Space wrap>
      <template v-if="!isSrConf">
        <span class="text-sm text-gray-500">目标平台</span>
        <Select v-model:value="form.platform_id" class="w-64"
                :options="filteredPlatforms.map(p => ({ label: p.name, value: p.id }))" />
      </template>
      <template v-else>
        <span class="text-sm text-gray-500">规则实体（可选）</span>
        <Select v-model:value="form.rule_id" class="w-56" allow-clear
                :options="(context?.rules ?? []).map(r => ({ label: r.name, value: r.id }))" />
        <span class="text-sm text-gray-500">或新建规则</span>
        <Input v-model:value="form.rule_name" class="w-56" placeholder="新规则名称" />
        <span class="text-sm text-gray-500">FINAL</span>
        <Radio.Group v-model:value="form.final_direction">
          <Radio value="PROXY">PROXY</Radio>
          <Radio value="DIRECT">DIRECT</Radio>
        </Radio.Group>
      </template>
    </Space>
  </Card>
  ```

  ```ts
  const form = reactive({
    // ...
    rule_name: '',
  })
  function targetReady(): boolean {
    return isSrConf.value
      ? Boolean(form.rule_id) || form.rule_name.trim().length > 0
      : Boolean(form.platform_id)
  }
  function buildInput(): GenerateInput {
    return {
      // ...
      rule_id: isSrConf.value ? form.rule_id : undefined,
      rule_name: isSrConf.value ? form.rule_name.trim() : undefined,
    }
  }
  ```

  6. `frontend/src/views/admin/assembly/NodesGroupsStep.vue`：
     - R22-06：删除未勾选预设组的 `preset` Tag；`AssemblyView.vue` 传 `show-xray`（`useSystemStore().status?.advanced_mode === true`），组件 `v-if="showXray"` 包住 xray 节点板块；`availableNodes` 在 showXray=false 时仅 manual。
     - R22-07：Modal 内“可选节点（勾选）”在上、“已选节点（有序）”在下；删除已选行“移除”按钮，取消下方勾选即可移除；保留上移/下移与拖拽。

  ```vue
  <div v-if="showXray">
    <div class="text-sm font-medium mb-1">xray 节点</div>
    <!-- 原 checkbox 列表 -->
  </div>
  <!-- 代理组 checkbox：删除 <Tag v-if="!form.group_names.includes(g.name)">preset</Tag> -->
  ```

  ```ts
  const availableNodes = computed(() => {
    const manual = props.manualNodes
    if (!props.showXray) return manual
    return [...manual, ...props.xrayNodes].filter((n) => n.allocatable && n.enabled !== false)
  })
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/version ./internal/assembly ./internal/server
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - `ListVersions` 返回的每个 `Version.ID > 0`，且等于 DB `versions.id`；
  - sr-conf 在无预建规则时 Preview/Generate 成功，自动创建规则且首个版本自动激活；版本创建失败时新规则被清理；
  - 从构建 Tab 进入四类装配器不再出现“类型与目标”步骤，顶部目标区可见；
  - 高级模式关闭时不显示 xray 节点板块与 xray 排序候选项；
  - 节点选择弹窗为“先勾选、后排序”，已选列表无移除按钮；
  - 前端 build + 既有测试全绿。


### Step 2：Clash 产物静态自检 + YAML Emoji/UTF-8 输出修复

- **目标：** 对齐 CVR 两层校验中可静态表达的部分：顶层结构、节点必填字段、组/规则引用、空 select 组；并修复 yaml.v3 把 emoji 转义为 `\U...` 的可读性/兼容性问题。
- **前置条件：** Step 1。
- **产出文件与操作：**

  1. 新增 `backend/internal/assembly/yaml_text.go`：安全还原 YAML 双引号标量里的 Unicode 转义，只还原 `\Uxxxxxxxx` / `\uxxxx`，不触碰 `\\U...` 字面文本、单引号标量与裸标量。

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

  2. 新增 `backend/internal/assembly/selfcheck.go`：纯函数静态自检。

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
  4. `backend/internal/assembly/render_clash.go`：`renderClash` 在 `yaml.Marshal` 后调用还原与自检；`clash_plan.go::RenderClashPlan` 只还原（下载侧不因自检阻断，仅日志观察）。

  ```go
  content, err := yaml.Marshal(root)
  if err != nil { return nil, fmt.Errorf("序列化 Clash YAML 失败: %w", err) }
  content = restoreYAMLUnicodeEscapes(content)
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
  - `TestClashEmojiRaw`：生成含 🚀/🌎/🛟/😀节点 的 Clash YAML，输出包含真实 emoji 原文；`yaml.Unmarshal` 回读名称一致；字面 `\\U0001F680` 不被误还原；
  - `TestSelfCheckRejectsDangling`：组引用不存在节点/规则目标不存在/空 select 组返回 error 级 issue；Preview 返回 warnings；Generate 返回 400；
  - `TestSelfCheckPassesGenerated`：合法最小 Clash 产物 0 个 error；
  - 既有 `TestClashHeaderOrder` 继续通过（Step 7 引入 use_sort 后按新预期更新）。

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
          // 【源码事实】CVR 按 u64 解析，单位小时，乘以 60 转分钟；非正整数小时无意义。
          n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
          if err != nil || n < 1 {
              return errors.New("profile-update-interval 必须是正整数小时")
          }
      case "profile-web-page-url":
          u, err := url.Parse(strings.TrimSpace(v))
          if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
              return errors.New("profile-web-page-url 必须是带 host 的 http/https 地址")
          }
      case "subscription-userinfo":
          // upload=..; download=..; total=..; expire=..，值均为整数
          for _, part := range strings.Split(v, ";") {
              kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
              if len(kv) != 2 { return errors.New("subscription-userinfo 格式须为 key=value; ...") }
              if _, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64); err != nil {
                  return fmt.Errorf("subscription-userinfo 的 %s 必须是整数", kv[0])
              }
          }
      }
      return nil
  }
  ```

  4. `backend/internal/setup/setup.go`：默认 Clash Verge 平台的 `extra_headers` 去掉手写 `Content-Disposition`（文件名由下载端点动态生成，避免旧模板写死 `subscription.yaml`）；保留 `profile-update-interval` 与 `profile-web-page-url`。
  5. `frontend/src/views/admin/PlatformEditView.vue`：附加响应头区增加「Clash 生态预设」快捷行：`profile-update-interval`（小时）、`profile-web-page-url`、`subscription-userinfo` 三键的可视化开关/输入；手工键值编辑器保留。保存时仍以键值对提交，前端按行内校验即时提示。
  6. `backend/internal/download/download.go`：高级模式系统注入的 `subscription-userinfo` 改用 `strings.Join` 已有逻辑不变；仅补 `expire` 可由 `users.expire_at` 推导的 TODO 注释，不改变本步语义。

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/download ./internal/server ./internal/platform
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 下载响应同时含 `filename` 与 `filename*=UTF-8''...`，中文/emoji 文件名被百分号编码；
  - 平台手写 `Content-Disposition` 不覆盖系统生成值（系统值最后写入）；
  - `profile-update-interval: 0`、`profile-web-page-url: ftp://x` 等被 400 拒绝；
  - 默认 Clash Verge 平台仍下发 `profile-update-interval: 6` 与合法主页头。


### Step 4：节点协议注册表 / 字段补全 + Clash 渲染与 URI 参数对齐

- **目标：** 以 CVR `src/types/global.d.ts` 的字段定义为准，把 manual 协议注册表从“能跑的最小字段集”补到“与 CVR 编辑/渲染语义对齐”；同时修正当前 Clash YAML 中扁平 `path/host` 等无法被现代 mihomo 识别的字段形态（【推断】）。
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
  | vmess | `packet-addr`、`xudp`、`packet-encoding`、`skip-cert-verify`、`fingerprint`、`reality-opts`、`http-opts`、`h2-opts`、`grpc-opts`、`ws-opts`、`global-padding`、`authenticated-length`、`smux`；废弃扁平 `path/host`（迁移到 `ws-opts`/`http-opts` 等） |
  | vless | 见上方代码注释；另将 `reality-opts` 与既有顶层 `public-key/private-key/short-id` 双形态兼容 |
  | trojan | `alpn` 改 text-list、`fingerprint`、`network`、`reality-opts`、`grpc-opts`、`ws-opts`、`ss-opts`、`client-fingerprint` |
  | hysteria / hysteria2 | `ports`、`hop-interval`、`up`/`down`、`obfs-protocol`、`skip-cert-verify`、`fingerprint`、`ca`/`ca-str`、`recv-window*`、`fast-open`、`cwnd`、`udp-mtu` 等按协议子集补齐 |
  | tuic | `token`、`ip`、`heartbeat-interval`、`reduce-rtt`、`request-timeout`、`udp-relay-mode`、`congestion-controller`、`disable-sni`、`max-open-streams`、`cwnd`、`ca`/`ca-str`、`recv-window*`、`disable-mtu-discovery`、`udp-over-stream*` |
  | wireguard | `ip`、`ipv6`、`workers`、`persistent-keepalive`、`peers`（object）、`remote-dns-resolve`、`refresh-server-ip-interval`；`allowed-ips` 改 text-list（逗号分隔），渲染时输出数组 |
  | http / socks5 | `sni`、`fingerprint`；http 增加 `headers`（object） |
  | snell | `udp` |
  | anytls | `certificate`、`ech-opts`、`idle-session-check-interval`、`idle-session-timeout`、`min-idle-session`；`alpn` 改 text-list |
  | mieru / masque | 按 Reference 2 全量补齐（transport/multiplexing/handshake-mode；ip/ipv6/mtu/remote-dns-resolve/dns 等） |
  | ssh | `password`、`private-key-passphrase`、`host-key`、`host-key-algorithms` |
  | openvpn / shadowquic / trusttunnel / tailscale | 维持现状（CVR 无对应 interface，本项目独有） |

  2. `backend/internal/node/node.go`：敏感字段支持**点路径**（如 `plugin-opts.password`、`private-key-passphrase`），加密/合并/解密函数统一走 `getPath/setPath`；`validateProtocolFields` 支持 `text-list/int-list/object` 的 JSON 类型校验。

  ```go
  // getPath/setPath 支持 "a.b" 路径；中间值不存在时创建 map[string]any。
  func getPath(m map[string]any, path string) (any, bool) { /* 按 strings.Split(path, ".") 逐层 */ }
  func setPath(m map[string]any, path string, v any) { /* 同上，中间层自动建 map */ }

  // encryptProtocolJSON：遍历 proto.SensitiveFields，对点路径加密。
  // mergeSensitive：同一路径语义；edit 留空 = 保留旧密文。
  ```

  3. `backend/internal/assembly/render_clash.go`：`clashProxy` 与 `dynamicClashProxy` 先做字段归一化，保证输出给 mihomo 的是数组/对象而不是逗号字符串或扁平旧字段。

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

  4. `backend/internal/assembly/links/links.go`：链接生成补齐 transport/Reality/插件参数；`genericLink` 的 vmess 补 `udp`，vless 读取 `ws-opts/h2-opts/grpc-opts/http-opts/xhttp-opts` 输出 `type/path/host/serviceName/mode` 等标准 query；`srLink` 的 vmess/vless 在 SR 形态参数基础上补 `tfo`（若存在）等 CVR URI parser 能回读的键。

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
  6. 兼容策略：旧库 `protocol_json` 中已有扁平 `path/host` 的 vmess/vless 节点，在 Clash 渲染前由 `normalizeLegacyTransport` 尝试迁移到 `ws-opts`（仅内存迁移，不写库）；SR/generic 链接读取时同时兼容新旧两种形态。【假设 A1：允许改写语义，但保留一次轻量兼容映射】

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/node ./internal/assembly ./internal/assembly/links
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 每个协议 `ManualProtocols()` 返回的字段与 CVR interface 的映射表核对通过（新增字段齐全、敏感路径正确）；
  - Clash YAML 中 `allowed-ips/alpn` 等以数组形态输出；`ws-opts`/`grpc-opts` 结构体输出正确；
  - `plugin-opts.password` 等嵌套敏感字段加密存储、编辑留空保留旧密文；
  - URI 生成的 vless WS/gRPC 链接可被本地 Go 端到端反解析（Step 8 的 uriparse 复用作回读器）；
  - 旧扁平 `path/host` 节点渲染不 panic，产物自检通过。

---

### Step 5：规则类型与素材池解析扩展（mihomo 规则全集）

- **目标：** 把规则白名单从 8 类扩展到 CVR `rules-editor-viewer.tsx` 所列的 mihomo 全集，并为每条规则维护 `no-resolve` 与取值校验元数据。
- **前置条件：** Step 2；Step 7 覆盖层会复用本步元数据。
- **产出文件与操作：**

  1. 新增 `backend/internal/rulespec/spec.go`（无 DB 依赖的纯元数据包，供 `pool` 与 `assembly` 共用）：

  ```go
  package rulespec

  type RuleDef struct {
      SR            bool   // Shadowrocket 是否支持
      NoResolve     bool   // 渲染时是否可追加 no-resolve
      ValueRequired bool   // false 仅 MATCH
      ValueLabel    string
  }

  // 类型全集取自 CVR rules-editor-viewer.tsx（【源码事实】），并按本项目判断标 SR 支持面。
  var Definitions = map[string]RuleDef{
      "DOMAIN":             {SR: true},
      "DOMAIN-SUFFIX":      {SR: true},
      "DOMAIN-KEYWORD":     {SR: true},
      "DOMAIN-REGEX":       {SR: false},               // 【推断】SR 不支持，Clash 渲染保留
      "GEOSITE":            {SR: false},
      "GEOIP":              {NoResolve: true},
      "SRC-GEOIP":          {SR: false},
      "IP-ASN":             {NoResolve: true},
      "SRC-IP-ASN":         {SR: false},
      "IP-CIDR":            {SR: true, NoResolve: true},
      "IP-CIDR6":           {SR: true, NoResolve: true},
      "SRC-IP-CIDR":        {SR: false},
      "IP-SUFFIX":          {NoResolve: true},
      "SRC-IP-SUFFIX":      {SR: false},
      "SRC-PORT":           {SR: false},
      "DST-PORT":           {SR: false},
      "IN-PORT":            {SR: false},
      "DSCP":               {SR: false},
      "PROCESS-NAME":       {SR: true},
      "PROCESS-PATH":       {SR: false},
      "PROCESS-NAME-REGEX": {SR: true},
      "PROCESS-PATH-REGEX": {SR: false},
      "NETWORK":            {SR: false},
      "UID":                {SR: false},
      "IN-TYPE":            {SR: false},
      "IN-USER":            {SR: false},
      "IN-NAME":            {SR: false},
      "SUB-RULE":           {SR: false},
      "RULE-SET":           {SR: false, NoResolve: true},
      "AND":                {SR: false},
      "OR":                 {SR: false},
      "NOT":                {SR: false},
      "MATCH":              {SR: true, ValueRequired: false},
      "USER-AGENT":         {SR: true}, // Clash 渲染跳过并提示，沿用现有口径
  }

  // ValidateValue 按类型校验：IP-ASN/SRC-IP-ASN/UID 数值；端口 1-65535；
  // NETWORK ∈ tcp/udp；CIDR 类用 net.ParseCIDR；DOMAIN-REGEX 用 regexp.Compile（RE2）；
  // AND/OR/NOT 至少校验括号配对；MATCH 值必须为空。
  ```

  2. `backend/internal/pool/parser.go`：`ParseLine` 增加逻辑规则与 MATCH 的处理；`ValidateEntry` 改为调 `rulespec`。

  ```go
  // 【源码事实】CVR 规则编辑器对 AND/OR/NOT 保存的是含逗号的完整表达式；
  // 现有 SplitN(line, ",", 3) 会截断这些类型，必须特殊处理。
  func ParseLine(raw string) (string, string, string, bool) {
      // 去注释/空行逻辑不变...
      typ := strings.ToUpper(strings.TrimSpace(parts[0]))
      if typ == "AND" || typ == "OR" || typ == "NOT" {
          rest := strings.TrimSpace(line[len(parts[0])+1:])
          if rest == "" { return "", "", "逻辑规则缺少表达式", false }
          // URL 来源行可能带 target；仅当 rest 不以 "(" 开头时尝试去掉最后一逗号段。
          // 本项目手动规则行有独立 target 字段，素材池 URL 行不带 target 时不受影响。【推断】
          return typ, rest, "", true
      }
      if typ == "MATCH" {
          return "MATCH", "", "", true
      }
      // 其余沿用现有两段解析
  }
  ```

  3. `backend/internal/assembly/validate.go`：删除 `validRuleTypes`，使用 `rulespec.Definitions`；Clash 允许全部 34 类（CVR 33 类 + 本项目保留的 USER-AGENT；USER-AGENT 在渲染层跳过并提示），SrConf 只允许 `SR==true` 的类型，对不支持类型在 Preview/Generate 回执中列出（跳过语义与 USER-AGENT 同口径）。
  4. `backend/internal/assembly/render_clash.go` / `render_sr.go`：`appendRule/formatRuleLine` 使用 `RuleDef.NoResolve`；MATCH 输出 `MATCH,<target>`（无匹配值）；逻辑规则输出 `TYPE,<表达式>,<target>`。
  5. `frontend/src/views/admin/AssemblyView.vue`：`RULE_TYPES` 扩展为 34 类；Clash 下拉排除 `USER-AGENT`（既有逻辑保持），SR 下拉由 `rulespec` 的 SR 集合映射（后端 context 可增加 `rule_type_options` 以避免前端重复维护，二者取其一，建议后端下发）。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/rulespec ./internal/pool ./internal/assembly
  ```

- **验收标准：**
  - `GEOSITE`、`RULE-SET`、`SRC-*`、端口类、逻辑类规则可入库/渲染；MATCH 输出无空逗号；
  - IP-CIDR/IP-CIDR6 保持 `no-resolve`；新增 `GEOIP/IP-ASN/IP-SUFFIX/SRC-IP-CIDR/RULE-SET` 的 `no-resolve` 行为符合 CVR 元数据；
  - AND/OR/NOT URL 源解析不再被截断；SrConf 对不支持类型跳过并提示；
  - 既有 8 类规则行为不回归。


### Step 6：代理组类型与字段扩展（load-balance / relay / use / 健康检查等）

- **目标：** 把全局代理组定义从 `select/url-test/fallback + groups[]` 扩展到 CVR `IProxyGroupConfig` 的 5 类型与常用渲染字段。
- **前置条件：** Step 2；Step 7 依赖本步的 `Definition` 与 `ClashPlanGroup` 结构。
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
  - `Use` 不校验具体 provider 存在性（provider 可能由 Step 7 的 Merge 覆盖层注入），生成/下载自检时校验。

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
  5. `frontend/src/views/admin/ProxyGroupsView.vue`：组类型下拉增加 `load-balance/relay`；编辑弹窗增加“高级字段”折叠区：`use`（逗号分隔标签输入）、健康检查四件套、`filter/exclude-filter/exclude-type/include-all*/disable-udp/hidden/icon`；预设组与自建组共用同一表单，name/preset_key 仍不可改。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/proxygroup ./internal/assembly
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 5 种组类型可创建/编辑；非法 `url`、未知 `exclude-type` 被 400 拒绝；
  - Clash YAML 输出完整 `url-test/fallback/load-balance/relay` 字段，且 `select` 组不再强制要求 `groups` 非空（有 `use` 或 include-all 即合法）；
  - `RenderClashPlan` 不会删除仅有 `use` 可达的组；
  - 历史 `definition_json`（只有 type/groups）加载与渲染不回归。

---

### Step 7：分层装配 — Merge + Rules/Proxies/Groups 覆盖层（核心借鉴）

- **目标：** 引入 CVR 的扩展覆盖模型。生成时与下载动态渲染时都按同一管线应用覆盖层；覆盖层随蓝图快照保存，重新编辑可完整恢复。
- **前置条件：** Step 4/5/6（字段与元数据齐备）。
- **关键设计决策（已在 §4.4 标注假设 A1/A4/A7）：**
  - 覆盖层输入为**YAML 文本**（与 CVR 编辑形态一致），后端用 `yaml.v3` 解析；
  - 本轮只服务 `clash-yaml`；覆盖层保存在 `selection_json.overlay` 与 `render_plan_json.overlay` 两个位置，避免新增数据库列（旧蓝图缺省即空覆盖层）；
  - 应用顺序：【假设】基础蓝图 + 动态 Xray 注入 → Rules seq → Proxies seq → Groups seq → Merge 深合并 → 控制面恢复 → 悬空清理 → 顶层排序。

- **产出文件与操作：**

  1. `backend/internal/assembly/models.go`：新增输入/快照结构。

  ```go
  type OverlayInput struct {
      MergeYAML    string `json:"merge_yaml,omitempty"`
      RulesYAML    string `json:"rules_yaml,omitempty"`
      ProxiesYAML  string `json:"proxies_yaml,omitempty"`
      GroupsYAML   string `json:"groups_yaml,omitempty"`
  }

  type SeqMap struct {
      Prepend []any    `yaml:"prepend" json:"prepend"`
      Append  []any    `yaml:"append" json:"append"`
      Delete  []string `yaml:"delete" json:"delete"`
  }
  ```

  `GenerateInput` 增加 `Overlay OverlayInput`；`ClashPlan` 增加 `Overlay OverlayInput \`json:"overlay,omitempty"\``。

  2. 新增 `backend/internal/assembly/overlay.go`：核心纯函数。

  ```go
  // parseSeq 把 YAML 文本解析为 SeqMap；空文本返回零值（等价 CVR 空模板）。
  func parseSeq(raw string) (*SeqMap, error) {
      raw = strings.TrimSpace(raw)
      if raw == "" { return &SeqMap{}, nil }
      var m SeqMap
      if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
          return nil, fmt.Errorf("覆盖层 YAML 解析失败: %w", err)
      }
      return &m, nil
  }

  // applySeq 实现 CVR use_seq 语义：prepend + (original - delete) + append。
  func applySeq(root *yaml.Node, field string, seq *SeqMap) error { /* 见下 */ }

  // deepMergeNode 实现 CVR use_merge 语义：Mapping 递归合并，其余以 patch 覆盖。
  func deepMergeNode(base, patch *yaml.Node) { /* 见下 */ }

  // snapshotControlPlane / enforceControlPlane 对应 CVR AuthoritativeFields：
  // 保存 CONTROL_PLANE_KEYS 的现值，merge 后恢复，缺失键删除。
  var controlPlaneKeys = []string{
      "external-controller", "secret", "mixed-port", "socks-port", "port",
      "tun", "mode", "allow-lan", "log-level", "ipv6", "unified-delay",
      // 平台键按部署目标选择性纳入；本项目生成的是客户端订阅，
      // 保护这些键可防止 Merge 误改影响 Clash Verge 的控制面接管。【推断】
  }

  // cleanupProxyGroups：与 CVR cleanup_proxy_groups 同口径——
  // 合法名 = proxies 名 ∪ proxy-groups 名 ∪ proxy-providers 名 ∪ 内置策略；
  // 清理 use 中不存在的 provider、proxies 中不存在的节点/组/provider。
  func cleanupProxyGroups(root *yaml.Node) { /* 见下 */ }

  // sortTopLevel：对齐 CVR use_sort——控制面键 → 其他键 → proxies/proxy-providers/
  // proxy-groups/rule-providers/rules 固定收尾。
  func sortTopLevel(root *yaml.Node) { /* 见下 */ }

  // applyClashOverlay 单入口：
  func applyClashOverlay(root *yaml.Node, ov OverlayInput) error {
      rules, err := parseSeq(ov.RulesYAML); if err != nil { return err }
      proxies, err := parseSeq(ov.ProxiesYAML); if err != nil { return err }
      groups, err := parseSeq(ov.GroupsYAML); if err != nil { return err }
      if err := applySeq(root, "rules", rules); err != nil { return err }
      if err := applySeq(root, "proxies", proxies); err != nil { return err }
      if err := applySeq(root, "proxy-groups", groups); err != nil { return err }

      control := snapshotControlPlane(root)
      if strings.TrimSpace(ov.MergeYAML) != "" {
          var mergeRoot yaml.Node
          if err := yaml.Unmarshal([]byte(ov.MergeYAML), &mergeRoot); err != nil {
              return fmt.Errorf("Merge YAML 解析失败: %w", err)
          }
          if mergeRoot.Kind != yaml.MappingNode { return errors.New("Merge 必须是 YAML 映射") }
          deepMergeNode(root, &mergeRoot)
      }
      enforceControlPlane(root, control)
      cleanupProxyGroups(root)
      sortTopLevel(root)
      return nil
  }
  ```

  `applySeq` 的参考实现要点：

  ```go
  func applySeq(root *yaml.Node, field string, seq *SeqMap) error {
      if seq == nil { return nil }
      seqNode := mappingValue(root, field)
      if seqNode == nil || seqNode.Kind != yaml.SequenceNode {
          seqNode = &yaml.Node{Kind: yaml.SequenceNode}
          mappingSet(root, field, seqNode)
      }
      deleteSet := map[string]bool{}
      for _, d := range seq.Delete { deleteSet[d] = true }

      kept := &yaml.Node{Kind: yaml.SequenceNode}
      for _, item := range seqNode.Content {
          name := yamlNameOf(item)
          if name != "" && deleteSet[name] { continue }
          kept.Content = append(kept.Content, cloneNode(item))
      }
      out := &yaml.Node{Kind: yaml.SequenceNode}
      for _, item := range seq.Prepend { n, err := toYAMLNode(item); if err != nil { return err }; out.Content = append(out.Content, n) }
      out.Content = append(out.Content, kept.Content...)
      for _, item := range seq.Append { n, err := toYAMLNode(item); if err != nil { return err }; out.Content = append(out.Content, n) }
      mappingSet(root, field, out)

      // 【源码事实】CVR use_seq：proxies 场景收集新增节点名，插入第一个 selector/select 组最前，
      // 且源码不要求该组 proxies 非空；同时从所有组删除 delete 命中的节点。
      if field == "proxies" {
          added := seqNames(seq.Prepend, seq.Append)
          if len(added) > 0 || len(deleteSet) > 0 { applyProxyGroupSideEffects(root, added, deleteSet) }
      }
      return nil
  }
  ```

  3. `backend/internal/assembly/render_clash.go`：基础 `root` 构建完成后调用 `applyClashOverlay(root, in.Overlay)` 再 marshal；`plan.Overlay = in.Overlay` 写入 render_plan_json。
  4. `backend/internal/assembly/clash_plan.go`：`RenderClashPlan` 解出 `plan.Overlay`，在动态节点注入与基础组收敛后调用 `applyClashOverlay(root, plan.Overlay)`，随后执行现有规则目标降级/fallback 改写与输出。历史蓝图无 `overlay` 时是零值，行为不变。
  5. `backend/internal/assembly/service.go` / `blueprint.go`：`SaveBlueprintTx` 把 `in.Overlay` 写入 selection_json 的 `"overlay"` 键；`GetBlueprint` 解析并返回 `overlay`；`loadEditIfAny` 前端恢复四个 YAML 文本。
  6. 新增 `frontend/src/views/admin/assembly/OverlayStep.vue`：仅 Clash YAML 装配器显示；四个 `Input.TextArea`（Merge/Rules/Proxies/Groups），提供 CVR 同款空模板填充按钮与字段说明。按项目简单轻量化原则先不引入 Monaco，文本域 + 预览 diff 即可覆盖首轮需求；预览前把四个文本并入 `buildInput().overlay`。
  7. `frontend/src/api/assembly.ts`：`GenerateInput.overlay`、`BlueprintResponse.selection.overlay` 类型同步。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/assembly ./internal/server
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - `TestOverlaySeqSemantics`：rules/proxies/groups 的 prepend/delete/append 顺序与 CVR `use_seq` 一致；delete 会从所有组移除节点；新增节点自动进入第一个 selector 组最前；
  - `TestOverlayMergeAndControlPlane`：嵌套 Mapping 深合并、非 Mapping 覆盖；merge 不能覆盖被保护的 `port/mode/allow-lan` 等控制面值；
  - `TestOverlayCleanupAndSort`：不存在的节点/组/provider 引用被清理；顶层键序符合 use_sort；
  - `TestBlueprintOverlayRoundtrip`：生成 → GetBlueprint 返回四个 YAML 文本 → 重新编辑加载一致；
  - `TestDownloadRendersOverlay`：激活蓝图的下载重渲染结果包含 prepend 节点/规则与 merge 字段，且动态 Xray 节点仍正确注入；
  - 无覆盖层的既有蓝图渲染结果与当前行为一致。


### Step 8：节点批量 URI 导入（借鉴 ProxiesEditor 粘贴解析与去重）

- **目标：** 为 manual 节点提供多行 URI / Base64 文本批量导入；解析失败跳过、按 name 去重、给出逐行回执，对齐 CVR `proxies-editor-viewer.tsx` 的交互模型。
- **前置条件：** Step 4（registry 字段完成）。
- **产出文件与操作：**

  1. 新增 `backend/internal/uriparse/uriparse.go`（独立小包，避免 `node ↔ assembly/links` 循环依赖）。

  ```go
  package uriparse

  type Result struct {
      Protocol string
      Name     string
      Host     string
      Port     int
      Params   map[string]any
  }

  // Parse 支持：ss / vmess（V2rayN JSON 与 Shadowrocket base64）/ vless（标准与
  // SR base64 userinfo）/ trojan / anytls / hysteria2 / hysteria / tuic / wireguard /
  // http(s) / socks5。解析规则与 CVR src/utils/uri-parser/* 保持一致。【源码事实】
  func Parse(raw string) (*Result, error) {
      s := strings.TrimSpace(raw)
      if s == "" { return nil, errors.New("空行") }
      // 若整体是 Base64 文本块（多行节点订阅形态），先尝试标准解码并按行展开；
      // 单行 Base64 由各 scheme 自己处理，避免误判普通 URI。
      scheme := strings.ToLower(strings.SplitN(s, "://", 2)[0])
      switch scheme {
      case "ss":     return parseSS(s)
      case "vmess":  return parseVMess(s)
      case "vless":  return parseVLESS(s)
      case "trojan": return parseTrojan(s)
      case "anytls": return parseAnyTLS(s)
      case "hysteria2", "hy2": return parseHysteria2(s)
      case "hysteria", "hy": return parseHysteria(s)
      case "tuic": return parseTUIC(s)
      case "wireguard", "wg": return parseWireGuard(s)
      case "http", "https": return parseHTTP(s)
      case "socks5", "socks": return parseSocks5(s)
      default: return nil, fmt.Errorf("暂不支持导入的 scheme: %s", scheme)
      }
  }

  // ParseBlock 处理多行输入：支持整块 Base64 解码；返回逐行结果与 skip 原因。
  func ParseBlock(text string) ([]Result, []Skip) { /* 50 行一批在调用方控制，不必在此并发 */ }
  ```

  VLESS Shadowrocket 形态的参考实现（对照 CVR `uri-parser/vless.ts`）：

  ```go
  func parseVLESS(s string) (*Result, error) {
      rest := strings.TrimPrefix(s, "vless://")
      // 先按标准 URI 解析；失败时尝试 SR 形态：base64(:uuid@host:port) + query
      if parsed, err := parseURLLike(rest); err == nil {
          return normalizeVLESS(parsed, false), nil
      }
      if idx := strings.Index(rest, "?"); idx > 0 {
          decoded := decodeBase64OrOriginal(rest[:idx])
          if parsed, err := parseURLLike(decoded + rest[idx:]); err == nil {
              return normalizeVLESS(parsed, true), nil
          }
      }
      return nil, errors.New("invalid vless uri")
  }
  // normalizeVLESS：uuid 在 SR 形态下去掉 "cipher:" 前缀；query 键归一化
  // security/tls→tls，pbk/sid→public-key/short-id，type/headerType→network，
  // host/path→ws-opts 或对应 transport opts（与 CVR 回读字段一致）。
  ```

  2. 新增 `backend/internal/node/uri_import.go`：批量导入业务逻辑。

  ```go
  type ImportLineResult struct {
      Line    int    `json:"line"`
      Raw     string `json:"raw"`
      OK      bool   `json:"ok"`
      Name    string `json:"name,omitempty"`
      Reason  string `json:"reason,omitempty"`
  }

  // ImportURIs：解析 → 全局 name 去重（保留第一条）→ 逐条加密敏感字段 →
  // 单事务内批量 INSERT；任何一行违反唯一索引/跨命名空间校验只跳过该行，
  // 不中断整批（与 CVR parseUri 失败跳过、按 name 去重同口径）。
  func (s *Service) ImportURIs(ctx context.Context, text string) ([]ImportLineResult, error) { /* ... */ }
  ```

  3. `backend/internal/server/node.go`：新增 `POST /api/admin/nodes/import`（会话 + 管理员中间件），请求体 `{ "text": "..." }`，响应 `{ list: ImportLineResult[], total: n }`。
  4. `frontend/src/api/node.ts`：新增 `importNodes(text)`。
  5. `frontend/src/views/admin/NodesView.vue`：PageHeader 增加「批量导入」按钮 → Modal 内 textarea（placeholder 说明支持 ss/vmess/vless/trojan/hy2/tuic/wireguard/http/socks5）→ 提交后展示成功/跳过行表格；成功后刷新列表。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/uriparse ./internal/node ./internal/server
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 标准 VLESS、SR base64 VLESS、V2rayN VMess、SR VMess、SS SIP002、Trojan、Hysteria2 URI 均能解析为 Step 4 的 protocol_json；
  - 与 CVR `uri-parser` 对同一批 URI 的解析结果字段一致（关键字段逐项比对）；
  - 重复 name 只导入第一条，第二条进入 skipped 回执；snell/mieru 等无 URI 协议返回“暂不支持”；
  - 批量导入中的非法行不阻断其余合法行；前端回执可读。

---

### Step 9：素材池同步长事务拆分（Issue7 R22-01）+ 启动补跑

- **目标：** 消除数万行规则源同步时 SQLite 单写者长事务锁死；并在服务启动时补跑“今日应跑但停机错过”的每日同步。
- **前置条件：** 无（可与 Step 8 并行）；实施前确认 Issue7 R22-01 用户已确认“短事务分批 + 临时表索引”方向。
- **产出文件与操作：**

  1. `backend/internal/pool/sync.go`：重构 `runSyncTask` 的落库阶段，禁止在单一 `TxImmediate` 内完成“插入 + keep 表 + 删除 + 状态回写”。

  ```go
  // 阶段 A：成功 URL 的条目分批插入，每批独立短事务。
  // 批次大小 500（沿用现有常量）；全部成功才进入删除阶段。
  for i := range results {
      if !results[i].OK { continue }
      for start := 0; start < len(results[i].entries); start += batchSize {
          end := min(start+batchSize, len(results[i].entries))
          err := bgStore.TxImmediate(ctx, func(tx *sql.Tx) error {
              return insertEntriesBatch(tx, poolID, results[i].URL, nextOrder, results[i].entries[start:end], &results[i])
          })
          if err != nil { /* 保留旧数据，直接进入失败终态 */ }
          nextOrder += end - start
      }
  }

  // 阶段 B：仅全部 URL 成功时，创建 keep 表并在同连接上分批插入 + 建索引。
  // 【源码事实】SQLite TEMP 表生命周期绑定连接；bgStore 为单连接，分事务安全。
  if !partial {
      keepTable := fmt.Sprintf("_pool_sync_keep_%d", taskID)
      // 每批一个短事务插入 keep；全部插入后一个短事务：
      // CREATE INDEX idx_keep ON keepTable(rule_type, match_value);
      // 然后先按 pool_entries.id 分批执行 DELETE ... NOT EXISTS (...)，每批 1000 行。
      // 删除统计另用只读查询完成，不再与 DELETE 同事务。
  }

  // 阶段 C：任务终态 + 池快照 + 清理 7 天历史，单独最后一个短事务。
  ```

  > 若删除阶段中途失败：任务落 `failed`，不执行历史清理；已插入条目保留（下轮同步幂等 `INSERT OR IGNORE` 会修正 added 统计），旧数据不丢。【推断】

  2. `backend/migrations/1014_pool_sync_index.sql`（新增）：`CREATE INDEX IF NOT EXISTS idx_pool_entries_pool_source_type_value ON pool_entries(pool_id, source, rule_type, match_value);`（Issue7 R22-01 推荐的第 3 项）。
  3. `backend/internal/cron/pool.go`：启动补跑。`StartPoolAutoSync` 首轮除“当前分钟匹配”外，增加：

  ```go
  // 【源码事实】CVR Timer.init 会补跑 overdue 订阅；本项目按 A6 只补跑“今日错过”的池。
  rows, err := st.DB().QueryContext(ctx, `
      SELECT p.id FROM rule_pools p
      WHERE p.auto_sync = 1
        AND p.sync_time <= ?
        AND COALESCE(date(p.last_synced_at), '') < date('now')
        AND NOT EXISTS (
            SELECT 1 FROM pool_sync_tasks t
            WHERE t.pool_id = p.id AND t.status = 'running'
        )`, now.UTC().Format("15:04"))
  ```

  对结果逐池 `SubmitSync`，已有 `ErrSyncRunning` 跳过即可。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/pool ./internal/cron
  ```

- **验收标准：**
  - 用 3 万行合成规则源同步时，`POST /api/admin/pools/:id/sync` 不再 5 秒内报 `database is locked`；插入阶段单个事务平均耗时明显低于 5 秒（测试可用短 busy_timeout 压测）；
  - 部分 URL 失败时仍不删除旧数据；全部成功时删除统计与旧数据正确；
  - keep 表索引存在且删除走索引（测试断言 schema 或执行计划）；
  - 启动首轮补跑测试：`last_synced_at` 昨天、当前时间晚于 `sync_time` 的池被提交一次；运行中任务不重复提交。

---

### Step 10：版本文件原子写入 + 下载渲染覆盖层接线

- **目标：** 借鉴 CVR `help::save_yaml` 的 temp + rename 原子替换，降低版本文件半写风险；并把 Step 7 的覆盖层接到用户下载重渲染路径。
- **前置条件：** Step 7；Step 9 完成前本步可与 9 并行开发（覆盖层部分不依赖 9）。
- **产出文件与操作：**

  1. `backend/internal/version/version.go`：新增 `writeFileAtomic`，`CreateVersion` 写版本文件时使用；失败清理路径相应调整。

  ```go
  // 【源码事实】CVR save_yaml：唯一临时文件（O_EXCL）→ write/flush → 保留权限 → rename 原子替换。
  func writeFileAtomic(full string, data []byte, perm fs.FileMode) (tmpPath string, err error) {
      dir := filepath.Dir(full)
      tmp := filepath.Join(dir, fmt.Sprintf(".tmp-%d-%s", time.Now().UnixNano(), randomSuffix()))
      f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
      if err != nil { return "", err }
      defer func() { if err != nil { _ = os.Remove(tmp) } }()
      if _, err = f.Write(data); err != nil { _ = f.Close(); return "", err }
      if err = f.Sync(); err != nil { _ = f.Close(); return "", err }
      if err = f.Close(); err != nil { return "", err }
      if err = os.Rename(tmp, full); err != nil { return "", err }
      return "", nil
  }
  ```

  `CreateVersion` 内：`writeFileAtomic(full, content, 0o644)`；DB 插入或 AfterCreate 失败时删除 `full`（事务回滚语义与现逻辑一致）；`evictOldest` 删除文件逻辑不变。

  2. `backend/internal/server/render.go`：`renderUserSubscription` 查询从

  ```go
  SELECT b.target_syntax, COALESCE(b.render_plan_json, '{}')
  ```

  改为在 Clash 分支解出 `plan.Overlay` 即可（Step 7 已把 overlay 写进 render_plan_json）。同时把 `RenderClashPlan` 的输出统一 `restoreYAMLUnicodeEscapes`；输出后若 `CheckClashContent` 出现 error，只 `slog.Warn` 不阻断下载（系统生成版本已在 Step 2 阻断过，这里仅防历史脏蓝图）。
  3. `backend/internal/download/download.go`：确认直接上传内容（无 blueprint）仍原样返回，不经过覆盖层/自检。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/version ./internal/assembly ./internal/server ./internal/download
  ```

- **验收标准：**
  - 写入过程中人为注入失败不会留下 `.tmp-*` 垃圾文件；并发创建版本不产生半文件；
  - 含覆盖层的 Clash 蓝图下载内容 = 预览内容（动态节点注入后同管线输出）；直接上传内容原样返回；
  - 下载渲染输出包含真实 emoji，无 `\U` 转义；
  - 历史无 overlay 蓝图下载行为与 Build8 一致。

---

### Step 11：前端 / 测试 / 文档 / smoke 收口

- **目标：** 把 Step 1~10 的 UI 与接口改动做整体回归，补齐专项测试与 smoke，并按文档体系同步。
- **前置条件：** Step 1~10 全部验收通过。
- **产出文件与操作：**

  1. `frontend/src/views/admin/assembly/`：确保四类装配器在 Step 1 目标区改造后，Steps 计数、`AssemblerShell` 的 prev/next 索引、路由 `platform_id/rule_id` 预填、重新编辑（含 Step 7 overlay 恢复）全链路正确；Clash 装配器增加 Overlay 步骤。
  2. `frontend/tests/*.spec.ts`：新增/扩展用例：
     - 版本列表 `id>0` 且“重新编辑”请求真实版本 ID；
     - 目标选择区与副 Tab 联动、SR-conf 规则名直建；
     - 高级模式关闭时 Xray 板块隐藏；
     - 节点选择弹窗先勾选后排序且无移除按钮；
     - Overlay 四个 YAML 文本随 Preview/Generate 提交、重新编辑回填；
     - 平台 Clash 生态预设头提交与错误提示；
     - 节点批量导入 Modal 成功/跳过回执。
  3. `backend` 单测：Step 1~10 各包新增用例全部保留；运行 `go test ./...`。
  4. `.smoke-test.sh` / `.smoke-test-prod.sh`：增加 Build9 冒烟路径：
     - 创建 yaml 平台 + 手动节点 + 组 + 覆盖层（prepend 规则/节点）→ Preview 含覆盖层 → Generate → 激活 → 下载响应含 `filename*`/`profile-update-interval`/真实 emoji；
     - URI 批量导入两条合法 + 一条非法，回执为 2 ok / 1 skip；
     - 重新编辑从版本管理进入并回填 overlay。
  5. 文档同步（本步执行时进行，本文档创建阶段不改）：
     - `Design2.md`：§3.3 代理组 5 类型与字段、§3.5 规则全集、§4.1 自检、新增“覆盖层”小节；
     - `Design2-UI.md`：§5 装配流程（目标区上移、Overlay 步骤、节点弹窗、批量导入）、§7 代理组高级字段；
     - `AGENTS.md`：文档清单增加 Build9（当前构建方案）；
     - `Issue7.md`：R22-01/02/03/06/07/08 标记 ✅ 并注明由 Build9 Step X 实施；R22-04/05 保持开放；
     - `ProdTestList.md`：增加 Emoji 输出、RFC5987 文件名、覆盖层下载、大规则源同步。
- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm test -- --run
  bash .smoke-test.sh
  bash .smoke-test-prod.sh
  ```

- **验收标准：** 后端/前端全绿；smoke 全绿；文档状态与代码一致；Issue7 相关条目状态闭环。

---

## 七、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-27 | 基于 clash-verge-rev 本地源码 `3503a2d`、三份 Reference 文档与 Issue7 开放项研究生成 Build9；含事实基线、假设清单、候选构建项与 Step 1~11 分步计划；未改动任何代码与既有文档。 |

