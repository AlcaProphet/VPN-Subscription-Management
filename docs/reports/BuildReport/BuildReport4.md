# BuildReport4.md — Design1～Design4 / Build1～Build21 / Issue1～Issue13 全量核验报告

> 核验日期：2026-09-04（以当前工作区 HEAD `982086f` 为准）。
> 核验对象：`~/Desktop/Repo/VPN-Subscription-Management` 当前整个仓库，依据 `AGENTS.md`、`Design1.md`～`Design4.md`、`Build1.md`～`Build21.md`、`Issue1.md`～`Issue13.md` 及项目内已有验收/安全/UI 报告。
> 核验方式：不信任文档中的“已完成/已修复”标记；对源码、迁移、路由、前端页面、测试、Docker 与 smoke 脚本进行复核。本次未修改任何业务代码或其它文档。
> 用户口径：Issue13 中仍在处理的 R27-08、R27-09 暂时不计入本次“遗漏”结论，但在本报告中仍如实记录其当前未实施状态。

---

## 一、结论摘要

**不能无条件宣布“项目已全部完成并完全达到 Design1～Design4 预期”。**

已确认成立的部分：

- `Build1～Build8` 的主要功能与修复均已真实落地，后端/前端自动验证通过。
- `Build9～Build15` 的主要实现与 Issue9～Issue11 修复均已落地，自动验证通过。
- `Build17～Build21` 的核心节点编辑器能力已落地，`R27-01～R27-07` 均有代码与自动化回归证据。
- 后端 `go build ./... && go vet ./... && go test ./...` 全部通过；前端 `npm run build` 与 41 个测试文件 / 195 个用例全部通过；Docker 多阶段镜像可成功构建。

尚不能视为完全验收闭环的部分：

| # | 未闭环项 | 性质 |
|---|---------|------|
| 1 | `Build16 / Design3` 并非“全部完成”：per-URL 快照状态/诊断 API 与前端展示缺失、pending 激活/丢弃操作无 UI、装配回执未展示、`no_resolve` 语义丢失、来源证据/排序未正确落库、零输出门槛存在漏网、后端素材池能力白名单未强制等 | 实质功能缺口 |
| 2 | `Issue13 R27-08/R27-09`：目标检查 `diagnostics:null` 会崩溃；Clash/Mihomo SS 插件仍输出 URI 风格字符串并删除结构化 `plugin-opts` | 已知未实施（按用户要求不作为本次遗漏项，但仍是未完成事实） |
| 3 | 节点输出与诊断仍有未登记缺口：未知 SS 插件 `plugin-opts` 被静默丢弃、`target_evidence` 未被检查链路消费、v2ray-plugin/shadow-tls/restls 的 URI 检查可能误报 `ok`、VMess/VLESS 的 SR 输出缺少若干 TLS/ALPN/指纹参数 | 新增实质缺口 |
| 4 | `.smoke-test.sh` / `.smoke-test-prod.sh` 当前原样执行不通过：先因 Python 布尔输出大小写比较失败，修正后因 Clash 装配请求缺少 `fallback_group_members` 失败 | 交付脚本与当前模型不一致 |
| 5 | `SecurityReport2` 的 N01～N07 仍为“建议未落地”：依赖漏洞/CI 门禁、备份未加密、验证码 fail-open、重置端点无限流、JWT 存 localStorage、安全头/CSP、密码设置无邮件通知 | 安全未闭环 |
| 6 | `ProdTestList.md` 中大量 Production、真机客户端导入/连接、R26/R27 人工回归仍未执行 | 人工验收待办 |

**定性：** “构建与自动化主体完成”成立；“Design 预期完全达成、可以交付验收闭环”尚不成立。

---

## 二、实测验证记录

```text
后端：
  cd backend && go build ./...                PASS
  cd backend && go vet ./...                  PASS
  cd backend && go test ./... -count=1 -timeout 180s
                                               PASS，全部包 ok
前端：
  cd frontend && npm run build                PASS（仅既有 >600 kB chunk 体积提示）
  cd frontend && npm test -- --run            PASS，41 files / 195 tests
Docker / smoke：
  bash .smoke-test-prod.sh                    镜像构建成功；脚本原样执行 FAIL
    - 10d) emergency 实际输出 "False"，脚本比较 "false"，停止
    - 修正该比较后：Clash 装配返回 400
        『🛟无法归属的流量』组未包含任何成员
    - 再补上 smoke 请求缺失的 fallback_group_members 后：SMOKE ALL DONE
  说明：当前交付脚本不能直接作为“全绿”证据，失败均属于脚本陈旧，不是后端业务逻辑错误。
```

另：`npm audit --audit-level=high` 在 smoke 中通过（0 vulnerabilities）。

---

## 三、Build1～Build8 / Design1 / Issue1～Issue8

### 3.1 逐 Build 核验

| Build | 文档声称 | 代码/测试证据 | 结论 |
|------|---------|--------------|------|
| Build1 工程骨架与认证 | 7/7 完成 | Go 1.26、迁移 0001～0005、auth/setup/oidc/captcha/ratelimit、前端登录/注册/Setup/OIDC 路由 | ✅ 已落地 |
| Build2 订阅核心与用户端 | 7/7 完成 | platform/version/subscription/group/token/download/custom/share/rule/home 及迁移 1001～1008；Build4 后采用“每平台一份订阅”的演进模型 | ✅ 已落地（按后续演进模型） |
| Build3 管理面与运维 | 6/6 完成 | 用户/审批/邮件/配置/导入导出/备份/日志/应急、SSE、危险操作等 | ✅ 已落地 |
| Build4 Go1.26+1009+基础模式 | 完成 | go.mod 1.26.0，1009 迁移拆除旧模型，规则素材池/高级模式门禁 | ✅ 已落地 |
| Build5 装配/manual 节点/代理组 | 完成 | 19 协议注册表、代理组、四类装配器、链接渲染、蓝图、前端页面 | ✅ 已落地 |
| Build6 Xray 后端 | 完成 | xray client/instance/credentials/sync/reconcile/quota/cron、任务注册表、fake gRPC 测试 | ✅ 已落地 |
| Build7 Xray 管理面/收口 | 完成 | Xray UI、高级组/用户/设置、独立账号、OFF 清空、v2 导出；Production 人工验收保留在 ProdTestList | ✅ 已落地（人工验收未完成） |
| Build8 Issue5 R20 修复 | 完成 | 池同步超时/取消、导入保护、候选 fail-closed、错误处理与测试 | ✅ 已落地 |

### 3.2 Issue1～Issue8 抽样结论

文档标记的修复大多有对应代码与测试，未发现整段“只有文档没有实现”的 Step。但存在以下非文档标记的质量/一致性问题：

| 编号 | 问题 | 证据/影响 |
|------|------|-----------|
| N-core-1 | 首管理员初始化标记只写不读 | `KeyAdminInitialized` 仅在 `user.go`、`oidc.go` 写入，无读取；设计中的“已有用户后防止再次首管理员”实际仅依赖 users 表为空 |
| N-core-2 | 自定义订阅用户每次首页加载会重新生成隐藏的组解析 Token | `home.go` 先创建无标识 Token，再覆盖为 custom Token；`custom.go` 曾删除组 Token，但下次首页又重新生成，存在隐藏 Token 残留 |
| N-core-3 | 仍存在忽略 error | `pool/sync.go` 状态回写 `_ , _`；`server/assembly.go` 回滚删除自动规则 `_ = Delete`；`server/oidc.go` 删除过期 ticket `_ , _` |
| N-core-4 | 接入层直接访问存储 | `server/render.go`、`server/traffic.go`、`server/oidc.go` 存在直接 `st.DB()` 查询/写入 |
| N-core-5 | 少量包级全局状态 | `response.go` 的 `debugProvider`、`log` 包默认 logger 与 level 变量；log 是注释声明的例外，但仍是 AGENTS 全局变量约束的边界情况 |
| N-core-6 | 验证码 Secret 明文存储/明文返回 | 与 AGENTS §4.2 的“敏感配置加密落库”存在冲突；Build3 文档将其标记为故意非敏感，属于文档与强要求的矛盾点 |
| N-core-7 | Issue1 R07-05 已超集 | 无激活版本的下载/预览当前是 HTTP 200 + 注释块，不是 Issue1 早期要求的 404；与 Build4/Design2 现行口径一致，不算当前缺陷 |

---

## 四、Build9～Build16 / Design2 / Design3 / Issue9～Issue12

### 4.1 Build9～Build15

| Build | 结论 |
|------|------|
| Build9 goccy YAML/自检/RFC5987/规则/代理组扩展 | ✅ 核心已落地，测试通过 |
| Build10 覆盖层/URI 批量导入/池补跑/原子写/收口 | ✅ 核心已落地，测试通过 |
| Build11 UI/UX 与管理员概览 | ✅ 已落地；R24-01 的 SQLite 单连接死锁已修复，全量测试通过 |
| Build12 Token 化/全局浮层/焦点 | ✅ 已落地 |
| Build13 R24-02/12/15 | ✅ 已落地 |
| Build14 R24 第二批 | ✅ 已落地 |
| Build15 R24-19/20 节点表单结构化 | ✅ 已落地 |

### 4.2 Build16 / Design3：**未能确认全部达标**

后端 Canonical Rule、能力注册表、探测/适配器、1016 迁移、快照状态机和装配回执都有真实代码，但按 Design3 验收仍有以下实质缺口：

| # | 缺口 | 证据/影响 |
|---|------|-----------|
| D3-1 | `no_resolve` 语义丢失 | `assembly/load.go` 读取 Canonical Rule 时丢弃 `Options.NoResolve`；`render_clash.go`/`render_sr.go` 按“类型支持”无条件追加 `no-resolve`，违反“只有源实际设置且目标支持时才输出” |
| D3-2 | 被来源模式排除的数量重复计算 | `pool/pipeline.go` 在循环中已 `Excluded++`，之后又用 `len(rules)-accepted-rejected` 再次累加，导致回执计数错误 |
| D3-3 | 手工规则更新会污染共享 URL Canonical | `pool/pool.go` 直接更新 `pool_canonical_rules`，若同一语义键同时存在 URL 来源，会改变 URL 派生规则 |
| D3-4 | 后端未强制素材池能力白名单 | `pool/pool.go` 仅做值校验，未校验 `MaterialPool`；API 可创建 RULE-SET/AND/OR/NOT/MATCH 等高级规则，前端过滤不是后端约束 |
| D3-5 | 来源原始证据/顺序未正确落库 | `pool/sync.go` 将 `sort_order` 全部写为同值，`line_no=0`、`raw_line` 用语义键代替原始行；查询也未按来源内原始顺序返回 |
| D3-6 | 零输出门槛不完整 | `server/assembly.go` 仅在 pools/custom_rules 非空且 final=0 时拒绝；用户不选任何素材池/规则时，可仅以内置 GEOIP/MATCH 生成，违反 Design3“最终输出必须至少 1 条非系统规则” |
| D3-7 | per-URL 快照/状态/诊断 API 缺失 | 后端没有按 URL 展示 snapshot 状态、格式、profile、统计与诊断的完整列表 API；`PoolSource` 只回传 active/pending ID |
| D3-8 | pending 激活/丢弃无前端 UI | 前端 API 有 `activatePending/discardPending`，但没有任何页面调用 |
| D3-9 | 装配回执未展示 | 后端返回 `receipt`，前端类型也有定义，但 `AssemblyView`/`PreviewStep` 未渲染 |
| D3-10 | 1015→1016 迁移缺少 store 级测试 | 虽迁移 SQL 存在，但未找到验证旧数据移除、ID 防复用与历史快照保留的迁移测试 |

### 4.3 Issue9～Issue12

- Issue9～Issue11 的 R24/R25 系列修复均在当前代码中找到实现和回归测试。
- Issue12 的 R26 系列代码已修复，但文档明确为“暂时完成”，人工回归仍留在 `ProdTestList.md` §五，不能视为完整人工验收。
- 另有陈旧文案：`PoolTab.vue` 仍显示“停机错过不补跑”，而 `cron/pool.go` 已实现启动补跑。

---

## 五、Build17～Build21 / Design4 / Issue13

### 5.1 R27-01～R27-09 实际状态

| 编号 | 文档状态 | 实际证据 | 结论 |
|------|---------|---------|------|
| R27-01 可编辑下拉搜索态 | 已修复 | `EditableCombobox` 打开清空搜索、空值可点；前端测试覆盖 | ✅ 已实现 |
| R27-02 递归条件/SS 插件拆分 | 已修复 | `ProtocolFieldEditor` 递归传递 currentState；obfs/v2ray/shadow-tls/restls 拆为独立对象；测试通过 | ✅ 已实现 |
| R27-03 SMux/Brutal 关闭清空 | 已修复 | `features.go`/`nodeFeatures.ts` 按元数据清理；前端/后端回归通过 | ✅ 已实现 |
| R27-04 移除 legacy 可编辑入口 | 已修复 | `editorFormSchema()` 排除旧 TLS/WS；前端 openEdit 清理草稿；测试通过 | ✅ 已实现 |
| R27-05 表单顺序/层级/集中开关 | 已修复 | `organizeFirstBatchForm()`、`nodeFormLayout.ts`、NodesView 重构；测试与本地浏览器记录 | ✅ 已实现 |
| R27-06 allow_custom 三态 | 已修复 | 后端 `*bool`、前端仅 `=== true` 放行；测试覆盖 | ✅ 已实现 |
| R27-07 saved_sensitive_paths/数组凭据 | 已修复 | 后端响应非空数组、WireGuard Peer UUID、历史升级与测试 | ✅ 已实现 |
| R27-08 diagnostics null 崩溃 | 方案已确认待实施 | `check.go:161` 仍 `nil`；`NodeCheckPanel.vue` 仍无空值保护；现有测试只覆盖 `[]` | ⚠️ 已知待实施（按用户口径不计入本次遗漏） |
| R27-09 SS 插件 Clash 输出回归 | 方案已确认待实施 | `render_clash.go` 仍转 URI 字符串并删除 plugin-opts；`RenderPluginForTarget` 忽略 target 参数；检查仍可 `ok` | ⚠️ 已知待实施（按用户口径不计入本次遗漏） |

### 5.2 Build17～Build21 结论

| Build | 结论 |
|------|------|
| Build17 统一保存契约 | ✅ 1017 迁移、current_state/edit_revision、409、凭据/扩展加密、URI 初始化等均在 |
| Build18 FieldSchema/检查接口 | ✅ 条件/选项/目标证据、活动投影、/check 与夹具均在；受 R27-08 的 diagnostics 契约影响 |
| Build19 前端动态表单 | ✅ 可编辑下拉、递归条件、分支清空、局部 JSON、目标检查 UI、409 均在；受 R27-08 与若干输出诊断缺口影响 |
| Build20 全协议过渡/输出门槛 | ✅ 统一归一化、19 协议保存、输出门槛均在；SS 插件目标映射与诊断未完整（R27-09 及新增缺口） |
| Build21 BuildReport3 对齐修复 | ✅ 分组/回显/折叠/列表/SS 映射等大部分落地；R27-04/05 已含；但 Issue13 R27-08/09 仍未实施 |

### 5.3 新增节点/输出缺口（Issue13 未登记）

| # | 缺口 | 证据/影响 |
|---|------|-----------|
| N-node-1 | 未知 SS 插件 `plugin-opts` 被静默删除 | `normalize.go` 只把已知插件复制到独立对象后删除 `plugin-opts`；`uriparse.go` 对未知插件也不保留 opts；违反“未知/不可识别数据不得静默丢失” |
| N-node-2 | `target_evidence` 未用于检查诊断 | 字段带 `partial/unverified` 证据，但 check/diagnostic 链路没有消费；导致 v2ray-plugin/shadow-tls/restls 等未验证映射仍可显示 `ok` |
| N-node-3 | VMess SR URI 缺 TLS 相关参数 | `links.go` SR vmess 分支未输出 `tls/peer/fp/allowInsecure`；Build21 已列为待独立核验边界，但当前仍为实际缺口 |
| N-node-4 | VLESS SR URI 缺 ALPN/client-fingerprint/flow/skip-cert-verify | `links.go` SR vless 分支未输出这些参数，Shadowrocket 兼容性未验证 |
| N-node-5 | SS v2ray-plugin 等 SR/generic 检查未给出未验证/警告 | `linkTargetDiagnostics` 仅对 obfs/未知插件/SS2022 告警；相关 fixture 还把 v2ray-plugin 期望为 ok |
| N-node-6 | 未知扩展/局部 JSON 仍有部分边界 | 与 R27-08/09 相关的面板崩溃和输出语义问题，见上 |

---

## 六、项目级交付、部署、文档与安全核验

### 6.1 容器与部署

多阶段构建、非 root、单端口、单数据卷、compose restart/healthcheck 均符合 AGENTS；但有观察项：

- Dockerfile 本身无 `HEALTHCHECK`（仅 compose 提供）。
- 运行镜像未显式安装 `ca-certificates`，OIDC/SMTP/素材 URL 等出站 HTTPS 需验证。
- 基础镜像与 GHCR `:latest` 未固定 digest（安全报告中已记录为边界项）。

### 6.2 Smoke 脚本（新发现问题）

- `.smoke-test.sh` 当前不是全绿：
  1. 第 97 行用 `"$EMERGENCY" != "false"` 比较，但 Python `J()` 输出布尔为 `False`，直接失败。
  2. 修正后 Clash 装配请求缺少 `fallback_group_members`，违反当前 `validate.go` 对强制组的非空校验，返回 400。
- 两个都只是脚本陈旧；在临时副本中补上 `False` 比较和 `fallback_group_members` 后，`SMOKE ALL DONE`。
- `.smoke-test-prod.sh` 无执行位，但用户用 `bash` 调用，非阻塞；脚本还依赖全新数据库，重复运行不幂等。

### 6.3 AGENTS 强要求符合度

| 约束 | 结论 |
|------|------|
| 路径穿越/静态分级/download no-store/Token 脱敏/错误码 | ✅ 基本符合 |
| 管理员端点双层中间件 | ✅ 基本符合；但 `/api/admin/logs/stream` 使用一次性短期 Token，是明确的 SSE 例外 |
| 前端路由级代码分割、AntD 按需引入、Token 化 | ✅ 主体符合；仍有 `NodeCheckPanel.vue`/`EditableCombobox.vue` 等使用 `bg-gray-50`/`border-gray-300` 等旧 Tailwind 类 |
| 接入层不直接操作存储 | ❌ 仍有 `server/render.go`、`server/traffic.go`、`server/oidc.go` 直接 SQL |
| 所有 error 必须处理 | ❌ 存在多处 `_ =`/`_, _ =` 忽略错误 |
| 禁止包级全局服务 | ⚠️ `response.debugProvider` 与 `log` 默认 logger/levelVar 例外 |
| 文件上传大小/流式 | ❌ 导入端点豁免体积上限，且 `settings_ops.go` 整体读入内存 |
| 敏感配置加密 | ⚠️ SMTP/OIDC 等已加密；captcha secret 明文存储/返回，属文档化冲突 |

### 6.4 文档一致性

- `AGENTS.md` 多处仍写 Design4 `v1.5`，实际为 `v1.12`。
- `AGENTS.md` 文档清单未列 `BuildReport2/3`、`SecurityReport2` 等已存在报告。
- `Build21.md` 标记 SS 插件目标映射完成，但 Issue13 R27-09 和当前代码仍显示未完成，存在文档矛盾。
- README 提到“LICENSE 文件”，仓库未发现 LICENSE。
- `docs/Reference/xray-server-side.md` 存在指向仓库外 `Xray-examples` 的失效链接。
- Build17～Build21 按归档规则仍未归档（AGENTS 已注明待归档，但状态已在“已完成”）。

### 6.5 安全历史未落地项（SecurityReport2 / SecurityReport3）

| 编号 | 当前状态 |
|------|---------|
| N01 | 依赖升级与 govulncheck CI 门禁未落地（go.mod 仍 go1.26.0/grpc 1.79.3/quic 0.59.0；Dockerfile 浮动 Go 镜像） |
| N02 | 备份下载仍为未加密 tar.gz |
| N03 | 验证码 secret 缺失时仍 fail-open |
| N04 | `/api/auth/reset` 仍无限流，重置令牌仍明文落库 |
| N05 | 前端 JWT 仍存 localStorage，未迁移 HttpOnly Cookie/CSRF |
| N06 | 安全头只有 nosniff；CSP 缺 object-src/base-uri，生产仍放行 ws |
| N07 | OIDC 用户首次设置密码未发邮件通知 |

### 6.6 其他项目级观察

- 素材池 URL 同步的 SSRF/内网地址拦截仍按历史“部署者边界”维持，未在代码中做内网阻断；导入接口的整体读入内存与体积豁免也已在 AGENTS 约束中标记为不符合。
- 前端存在未引用文件：`views/admin/assembly/GenerateStep.vue`、`components/PreviewState.vue`、`components/ResponsiveCollection.vue`、`components/CopyField.vue`，属于清理候选。
- `npm audit` 当前为 0 漏洞；但 Go 依赖与 govulncheck 门禁仍未落地（见 N01）。

---

## 七、最终判定

1. **Build 代码主体完成度：高。** Build1～Build21 的主要实现、迁移、接口、前端页面、自动化测试均真实存在，后端全量测试、前端全量测试/构建、Docker 镜像构建全部通过。
2. **Design 预期完成度：不能判定为全部达成。**
   - Design3 对应的 Build16 存在多处未完成/与设计不符的实质缺口（per-URL UI/API、pending 操作、回执展示、no_resolve、来源证据、零输出门槛等）。
   - Design4 对应的节点编辑器核心已实现，但 R27-08/R27-09 已知未实施，且新增发现未知 SS 插件参数丢失、target_evidence 未消费、SR/VLESS/VMess 输出不完整等问题。
   - Design1/Design2 的历史功能基本落地，但仍有分层、忽略错误、Token 残留、安全与人工验收未闭环。
3. **交付与验收状态：未完全闭环。**
   - Production smoke 脚本原样失败（脚本陈旧，修正后可通过）。
   - `ProdTestList.md` 的真实 Production、真机客户端导入/连接、R26/R27 人工回归均未执行。
   - `SecurityReport2` 的 N01～N07 仍未实施。

**建议后续优先级：**

1. 修复 Build16/Design3 的实质缺口（尤其 per-URL UI/API、`no_resolve`、来源证据、零输出门槛、后端素材池白名单）。
2. 实施 Issue13 R27-08/R27-09，并同时处理新增的未知 SS 插件参数丢失、target_evidence 诊断、SR 输出缺失参数。
3. 修复 `.smoke-test.sh` 的布尔比较与 `fallback_group_members`，使 Production smoke 恢复为可直接全绿。
4. 按 AGENTS/安全报告落地 N01～N07 与工程约束整改（分层、错误处理、上传限制等）。
5. 同步文档：AGENTS 的 Design4 版本、报告清单、Build21/R27 状态、README LICENSE、Reference 失效链接。

---

## 八、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-04 | 首次创建：对 Design1～Design4、Build1～Build21、Issue1～Issue13 的全量核验；记录自动验证、Build16/Design3 缺口、节点编辑器 R27 状态与新增缺口、smoke 脚本问题、安全未落地项和文档不一致。未修改任何代码或其它文档。 |
