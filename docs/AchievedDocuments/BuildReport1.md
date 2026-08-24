# BuildReport1.md — Build4/Build5 事后验收与 Build6/Build7 事前预检报告

> 核验日期：2026-08-20。核验对象：当前工作区代码（Build4/Build5 已落地产物）、[Issue2.md](./Issue2.md) 未修复问题、[Build4.md](./Build4.md) / [Build5.md](./Build5.md) 验收要求，以及 [Build6.md](./Build6.md) / [Build7.md](./Build7.md) 的构建前深度核验。
> 结论前置：**Build4/Build5 的自动验收命令全部通过，但对照 Build 文档与 Design2-UI 的“完全达标”不能成立**——Issue2 中 R14-01~R14-24 仍为未修复状态并已逐条与代码事实核对；本次另发现 4 项 Issue2 未覆盖问题。Build6/Build7 的设计一致性总体可用，已对确定性问题直接修订，并保留 3 项需用户决策的不确定问题。

---

## 一、总览

| 维度 | 结果 |
|------|------|
| 后端编译/静态检查/单测 | ✅ `go build ./... && go vet ./... && go test ./...` 全绿 |
| 前端类型检查/构建/单测 | ✅ `vue-tsc -b && vite build` 通过；`vitest run` 35/35 通过 |
| 渲染性能 benchmark | ✅ Clash 1 万规则 ≈ 8.0ms/op；SR conf 1 万规则 ≈ 0.88ms/op；阈值测试双通过 |
| 特定 grep 命中数 | ✅ 旧表引用 / 活跃文件 1.25 / `TODO(FIXME)(Build4` / Build5 装配占位均为 0 命中（`PlaceholderView` 仅 `/admin/xray` 保留，属 Build7 正常占位） |
| 关键包 race | ✅ `go test -race` 覆盖 pool/assembly/node/proxygroup/version/subscription/server 通过 |
| 全新库迁移 | ✅ 0001~1010 迁移正常，status `advanced_mode:false` |
| Build4/Build5 文档合规性 | ❌ **不完全达标**：R14-01~R14-24 未关闭，另新增 N1~N4（见第三节） |
| Build6/Build7 事前预检 | ⚠️ 确定性问题已修订（Build6 v2.1、Build7 v1.11、Build5 v1.10 衔接注记）；3 项不确定问题待用户决策 |

---

## 二、验收命令实测记录

```text
backend:
  go build ./...                                    PASS
  go vet ./...                                      PASS
  go test ./...                                     PASS
  go test -race ./internal/pool/... ./internal/assembly/... ./internal/node/... ./internal/proxygroup/... ./internal/version/... ./internal/subscription/... ./internal/server/...  PASS

assembly benchmark:
  BenchmarkRenderClash10kRules-14                  8002584 ns/op  (≈ 8.0ms, <500ms)
  BenchmarkRenderSrConf10kRules-14                  877541 ns/op  (≈ 0.88ms, <500ms)
  TestRenderClash10kRulesThreshold                  PASS
  TestRenderSrConf10kRulesThreshold                 PASS

frontend:
  npm run build                                     PASS
  npm test                                          PASS (11 files, 35 tests)

grep:
  group_selections|subscription_group_rel in backend/internal, backend/cmd, frontend    0 命中
  1.25 / golang:1.25 in Dockerfile, backend/go.mod, README, CI                        0 命中
  TODO(Build4 / FIXME(Build4 in backend, frontend                                      0 命中
  "将在下一轮构建实现" in frontend/src                                                0 命中
```

---

## 三、事后验收：Build4/Build5 逐 Step 结论

> 判定口径：自动命令通过 ≠ Step 完全达标；Step 若存在 Issue2 未关闭问题或本次新增问题，按“⚠️ 未完全达标”记录。

### Build4

| Step | 自动验证 | 合规性结论 |
|------|---------|-----------|
| Step 0 Go 1.26 与 Dockerfile | ✅ | ✅ 达标 |
| Step 1 1009 迁移与 dataclear | ✅ | ✅ 达标（1010 增量迁移为 R12-02/R12-13 用户确认后的追加，可接受） |
| Step 2 旧分发模型拆除 | ✅ | ✅ 达标 |
| Step 3 版本/平台/规则改造 | ✅ | ✅ 达标（`/api/home/summary` 契约与单测存在） |
| Step 4 前端基线 | ✅ | ⚠️ 未完全：R14-08/09/10/11/12/13/14/16/17/24 均落在本 Step 范围 |
| Step 5 素材池后端 | ✅ | ⚠️ 未完全：R14-19 的 `failTask` 兜底写回失败无感知；其余 R12 系列已核实关闭 |
| Step 6 素材池前端 | ✅ | ⚠️ 未完全：R14-10 断点互斥缺失、R14-23 PoolDetail 条目卡态缺失、R14-15 第⑧项 URL 无前端校验 |
| Step 7 端到端验收 | ✅ | ⚠️ 未完全：受上表问题影响，里程碑“完全达标”不成立 |

### Build5

| Step | 自动验证 | 合规性结论 |
|------|---------|-----------|
| Step 1 协议注册表/manual 节点 | ✅ | ✅ 达标（openvpn 敏感字段例外与 onXrayChanged 均已按 Issue2 用户结论落地） |
| Step 2 代理组后端 | ✅ | ⚠️ 未完全：R14-18 两函数复制实现未按 Step2 要求共享基础函数（行为等价，结构不符） |
| Step 3 渲染内核/链接编码 | ✅ benchmark 达标 | ⚠️ 未完全：R14-02 proxy-groups 键序、R14-03/04/05 链接参数与形态、R14-06 render_plan 不自包含 |
| Step 4 装配端点/蓝图集成 | ✅ | ⚠️ 未完全：R14-06/R14-07 蓝图列缺口；另见新发现 N1（assembly 接入层直连 SQL） |
| Step 5 节点/代理组前端 | ✅ | ⚠️ 未完全：R14-14 八项 UI 细节；Step5 item7 要求的节点/代理组前端单测缺失（新发现 N3） |
| Step 6 装配页前端 | ✅ | ⚠️ 未完全：R14-15 八项 UI 细节、装配页 spec 缺失；R14-09 装配流 pooled_sub 标记缺失 |
| Step 7 分发 UI/端到端 | ✅ | ⚠️ 未完全：R14-01 版本管理页每开必报错、R14-08 取消默认无确认、R14-21 429 Retry-After 分支不可达；`.smoke-test.sh` 仍为旧模型（新发现 N4） |

---

## 四、Issue2 R14 未修复项逐条核对结果

已逐条比对当前代码，**R14-01~R14-24 全部仍为未修复（☐）**，抽样确认事实：

| 编号 | 当前代码事实（核实） |
|------|----------------------|
| R14-01 | `server/subscription.go` 无 `GET /api/admin/subscriptions/:id`；`api/subscription.ts` 的 `getSubscription` 仍被 `VersionManageView` 调用；`static.go` NoRoute 对 `/api/` GET 仍返回 200+HTML。已用全新库启动服务实测复现（返回 `text/html` 而非订阅 JSON） |
| R14-02 | `render_clash.go` 三强制组与勾选组仍以 `map[string]any` 传入 `toYAMLNode`，键序不稳定 |
| R14-03 | `links.go` anytls 缺 alpn/client-fingerprint、tuic 缺 alpn、wireguard 缺 pre-shared-key |
| R14-04 | registry 缺 hysteria alpn/mport/insecure、hysteria2 alpn、vless alpn、vmess alpn/fp；trojan alpn 有 registry 无渲染 |
| R14-05 | trojan/anytls/hysteria/hysteria2/tuic/wireguard/http/socks5 空 query 仍无条件拼 `?`；http/socks5 空凭据仍输出 `:@` |
| R14-06 | `render_clash.go` plan 仍为 `manual_proxies` 名字数组、`proxy_groups` 组名数组、`rules` 池引用数组，不自包含 |
| R14-07 | `assembly/service.go:91` `xray_candidates` 仍硬编码 `[]string{}` |
| R14-08 | `RulesView.cancelDefault` 仍无 ConfirmModal 直调接口 |
| R14-09 | `AssemblyView.doGenerate` 成功后未写 `pooled_sub_${subscriptionId}` 标记 |
| R14-10 | `PoolTab.vue` 桌面 Table 无 `hidden md:block`，与 `md:hidden` 卡片同现 |
| R14-11 | 桌面订阅行只有底色+按钮，无 `a-alert` 标签；删除清单缺“不级联自定义订阅” |
| R14-12 | 移动端更多菜单无重新编辑；空态静态串未按 ownerType；装配按钮在页头 |
| R14-13 | PlatformEdit 400 仍全局 Notify；RulesView 空实体行无“可作为 SR 分流规则装配目标”副文案 |
| R14-14 | NodesView manual 标签蓝色、非 missing xray 删除无 Tooltip、无空格实时校验；ProxyGroupsView 标签色反、Checkbox 不在行首、DAG 保存时检测、拖拽无 HolderOutlined/断点门控 |
| R14-15 | TypeTarget 无平台空态/规则新建快捷；Clash 头部初始 `{}`；RulesStep 无桌面拖拽；PreviewStep 无 `# {{xray_nodes}}` Tooltip；重编辑 alert 无 vN；无装配页 spec；池 URL 无前端校验 |
| R14-16 | `server/home.go` 12 处、`server/profile.go` 5 处直连 SQL；`ProfileHandler.users` 未注入且未使用 |
| R14-17 | CopyField 0 引用；PageHeader 仅 3 个管理页使用 |
| R14-18 | `node.ValidateNodeName` 与 `ValidateProxyGroupName` 仍为两份复制实现 |
| R14-19 | `assembly/models.go:115` `kb, _`；`pool/sync.go:332` `_ = TxImmediate`；`rule/rule.go:112` 首个 DELETE `_, _` 均在 |
| R14-20 | 清单内死代码逐项仍存在 |
| R14-21 | `ApiError` 构造未提取 `retry-after` 头 |
| R14-22 | 三处旧模型注释残留 |
| R14-23 | `PoolDetail.vue` 条目区仅 a-table，无 <768 卡片分支 |
| R14-24 | `router-guard.spec.ts` 仅覆盖 `advanced_mode:false`，未覆盖 `status=null` |

> R14-25（config.Get 系统性静默）与 R14-26（TOCTOU 窗口）仍为“待确认”，本次不做代码改动。

---

## 五、本次新发现（Issue2 尚未记录）

- **N1（分层红线遗漏面）**：`backend/internal/server/assembly.go` 的 `resolveOwner` 与 `blueprint` 直接 `store.DB().QueryRowContext` 查询 subscriptions/assembly_blueprints/nodes/proxy_groups/rule_pools，属于接入层直接操作存储，违反 AGENTS.md §五；Issue2 R14-16 仅点名 home/profile，未覆盖本文件。建议纳入 R14-16 整改范围。
- **N2（忽略 error 新增面）**：`backend/internal/download/download.go` `withPlatformHeaders` 中两处 `_ = s.store.DB().QueryRowContext(...).Scan(&resName)` 忽略查询错误（custom/slug 与 subscription/name），违反“所有 error 必须处理”。
- **N3（前端单测缺口）**：Build5 Step5 item 7 要求“节点表单动态渲染/敏感字段留空；代理组环检测提示；移动端排序降级渲染”单测，`frontend/tests/` 无 NodesView/ProxyGroupsView spec；Build5 Step6 item 7 的装配页 spec 缺失已由 R14-15 第⑦项覆盖。
- **N4（交付脚本陈旧）**：`.smoke-test.sh` 仍为 Build2/3 旧模型（固定 18080、`sub_ids/selections`、旧首页 `subscriptions[]` 形状），按当前 API 执行必然失败。Build7 Step6 已在本轮修订为“必须先重写脚本至 Build4/5 新模型”。

---

## 六、事前预检：Build6/Build7 结论与已执行修订

### 6.1 设计一致性结论

- Build6 各 Step 与 Design2 §5.1~§5.8 的取值、端点、事务与并发口径总体一致；与 Xray-Core-API.md §11 的取证结论一致（幂等字符串、QueryStats pattern、串行化、vmess 无 alter_id）。
- Build7 各 Step 与 Design2 §5.4/§5.10/§5.11 及 Design2-UI §3/§4/§8/§9 的能力点和交互分支总体对齐，未发现整段能力遗漏。
- 跨 Build 衔接（task registry、ext 采集、导入后回调、OFF 复用 AfterAdvancedOff）已有明确落点；但仍存在 1 项架构级待决策问题（见第七节 Q2/Q3）。

### 6.2 本轮确定性修订

**Build6.md → v2.1**
1. Step1 文件清单补 `detect.go` 与 `node.go`（删除清理钩子落点）。
2. Step1 slug 共享命名空间改为双处扩展：`TableHasSlug` 白名单 + `subscription.slugExistsTx` / `slug.ExistsInFourTables` 四表清单。
3. Step2 文件清单补 `platform.go` / `subscription.go`；订阅删除触发不再引用已删除的 `onSubDeleted`，改为注入函数字段或 server handler 调用。
4. Step3 node 启停触发修正为复用 Build5 实际存在的 `SetOnXrayChanged` / `XrayChangedFunc`，与 detect 的 `OnNodeVisibilityChanged` 区分；事务内配置读取点名使用既有 `config.Service.GetTx` / `GetSigningKeyTx`（当前 `MaxOpenConns=1`，误用 `cfg.Get` 会死锁）。
5. Step4 链接共享抽取前必须先关闭 R14-03/04/05（按 Node-Link-Standards §2 补齐参数与 `?`/userinfo 形态）并补一致性单测。
6. Step4 render plan 冻结口径澄清：不重读 pool_entries/proxy_groups 当前定义，nodes 表仅用于动态 xray 行、可用性过滤与稳定键→renderName 映射。

**Build7.md → v1.11**
1. Step1 CreateExt 临时 email 必须唯一化（固定占位串并发会撞 UNIQUE），并明确 `xray_ext_users.node_id` 按 (instance_id, inbound_tag) 解析写入。
2. Step2 `/api/admin/settings/advanced` 明确只叠加 session+admin，不得套 advancedMode（否则 OFF 态无法开启/关闭与轮询）。
3. Step2 Import v2 的 slug 冲突/重复明确为“任务终态 failed，error 按 400 口径”，HTTP 提交阶段不能同步返回 400。
4. Step4 基础模式所属组隐藏补移动卡片态。
5. Step6 明确必须先重写 `.smoke-test.sh` 至 Build4/5 新模型（现行脚本必然失败）。

**Build5.md → v1.10（衔接注记）**
- 注明 R14-03/04/05 链接缺口须在 Build6 Step4 抽取 `assembly/links` 前于 Build5 Step3 `links.go` 补齐，与 Build6 v2.1 钉死。

---

## 七、用户决策清单（2026-08-20 已确认）

| 编号 | 问题 | 用户最终决策 | 已落盘位置 |
|------|------|--------------|-----------|
| Q1 | Build4/5 仍存在 R14-01~24 + N1~N4，修复如何排期？ | **先执行 Build4/5 修复收口，完成后方可启动 Build6**（R14-06/07 仍按既有决策随 Build6 Step2/Step4 修） | Issue2 附件新增优先级 0；BuildReport1 |
| Q2 | 全局任务 registry 包归属与分层矛盾 | **下沉为中性包 `internal/tasks`**，server 构造注入；server 只保留查询路由适配 | Build6 v2.2、Build7 v1.12 |
| Q3 | OFF 清空“状态翻转+任务登记同事务”与内存 registry 的组合 | **维持字面同事务，接受 DB 回滚残留幽灵任务边界**（重启后统一 failed 兜底） | Build6 v2.2、Build7 v1.12 |
| Q4a | R14-25 config.Get 错误静默 | **关键路径（签名密钥 / advanced_mode 及鉴权、同步钩子入口）改为显式错误**；其余展示类配置维持 fail-safe 并补口径注释；纳入 Build4/5 修复收口 | Issue2 R14-25 状态更新 |
| Q4b | R14-26 node/proxygroup TOCTOU | **维持现状**，记录“单连接 + 重读 NotFound 兜底 + 装配期 invalid_refs 二次校验”的合理解释，不改代码 | Issue2 R14-26 状态更新 |

---

## 八、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-20 | 首次创建：Build4/5 事后验收报告 + Issue2 R14 逐条核对 + 新发现 N1~N4；Build6/7 事前预检确定性问题修订（Build6 v2.1、Build7 v1.11、Build5 v1.10）；Q1~Q4 待用户决策 |
| v1.1 | 2026-08-20 | 用户决策落盘（Q1~Q4）：先执行 Build4/5 修复收口（Issue2 附件优先级 0）；registry 下沉 `internal/tasks`（Build6 v2.2/Build7 v1.12）；OFF 同事务登记并接受幽灵任务边界（Build6 v2.2/Build7 v1.12）；R14-25 关键路径显式错误、R14-26 维持现状（Issue2 v2.1） |
| v1.2 | 2026-08-20 | Build4/5 修复收口首轮执行：完成 R14-01/02/03/04/05/08/09/10/11/12/13/14/18/19/20/21/22/23/24 与 N1/N2/N4；R14-15/16/17/25 部分落地；R14-06/07 按决策随 Build6 实施；N3 仍待补。验证：后端 build/vet/test 全绿，前端 build + 36 单测通过。 |
