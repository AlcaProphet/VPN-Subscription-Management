# BuildReport5.md — 当前项目全量完成度核验报告

> **核验日期：** 2026-09-11 09:35:05 CST
> **分支与 HEAD：** `beta` / `a09dc542c37885b400f025b4e469bfd0475bd0c0`（2026-09-11 09:13:36 +0800，标题：归档 Build22 并闭环 R28-05：补全 Step 7 专属自动化证据，同步更新 Design3/AGENTS/Issue14/15/TODOLIST/ProdTestList 文档状态）
> **工作区状态：** `git status --short` 无输出，工作区干净。
> **核验范围：** Design1～Design4、Build1～Build25、Issue1～Issue15、AGENTS.md 强要求、BuildReport1～4 回归、活跃跟踪文档排除矩阵。
> **核验原则：** 只读、证据驱动、不信任文档声称；区分静态代码/自动化测试/Docker/Production/浏览器/真实客户端/人工证据层级。
> **排除口径：** TODOLIST.md、Issue14.md、Issue15.md、ProdTestList.md、SecurityScanPlan1.md、SecurityReport3.md 中登记的活跃事项，以及 Design5.md 候选构想。
> **是否存在阻断：** 无致命阻断。缺少真实 Production 环境和真实客户端的动态验证能力，已归入证据边界。

---

## 一、执行摘要

### 可以确认的完成范围

1. **Build1～Build21 主体代码与自动化验收**：全部已落地。后端 `go build ./...`、`go vet ./...`、`go test ./... -count=1 -timeout 180s` 全部通过（39 个包全部 ok）；前端 `npm test -- --run` 42 文件 / 259 用例全部通过；`npm run build` 生产构建成功；`git diff --check` 无警告。
2. **Build22（Design3/Build16 D3-1～D3-10 收口）**：Step 1～11 全部验收通过，Step 7 专属自动化证据已补齐并重新通过 Step 11 门禁。已归档。
3. **Build23（R27-09 交接说明）**：已归档。R27-09 与 N-node-1～6 已并入 Build21。
4. **Build24（R29-06 节点动态表单控件语义）**：代码与自动化验收通过。已归档。真实运行核验待 ProdTestList。
5. **Build25（R28-06 未知扩展/局部 JSON 边界）**：Step 0～4 全部验收通过。已归档。人工项待 ProdTestList。
6. **Issue14 步骤一～四、步骤六**：工程关闭。
7. **Issue15 R29-02/03/04/05/07/08/11/12**：闭环或已完成。
8. **AGENTS.md 核心要求**：路径穿越防护、Token 脱敏（`redact` 包）、下载 no-store、管理员双层中间件、事务 + 写锁、级联删除、构造注入、前端路由级代码分割等均有代码与测试证据。
9. **Docker/Compose**：Dockerfile 多阶段构建、非 root、单端口、单数据卷、Compose restart `unless-stopped` 与 healthcheck 均符合 AGENTS。
10. **Production smoke**：`.smoke-test-prod.sh` 在 Build21 步骤一修复后已全链路通过。

### 不能确认的范围

1. **Issue14 步骤五（R28-07A～E、G～I）**：尚未开始代码实施。仍存在接入层直接访问存储（`server/render.go`、`server/traffic.go`、`server/oidc.go`）、多处忽略 error、可变包级状态、导入无上限等问题。
2. **Issue14 步骤七（R28-09）**：`ca-certificates`、LICENSE、失效链接、未引用前端文件等收尾未完成。
3. **Issue14 步骤八（全量复核与关闭）**：取决于步骤五和步骤七。
4. **ProdTestList 人工项目**：R29-01 Setup 新库导入（Docker 镜像重建后浏览器复验）、R29-04 真实手机/Production、R29-06 节点动态表单真实运行、R27-05 额外尺寸/焦点/折叠、R26-07 Xray PUT 错误码人工核验、Build22 Step 8/9 来源状态与装配回执浏览器核验、OIDC Mock、R20-11 重启数据保留等均未执行。
5. **SecurityScanPlan1 Step 4～28**：尚未开始。
6. **Design5.md**：仅为候选构想，未研究定稿，不计入当前范围。

### 新发现数量

**0 项确定性新发现。** 所有可复核的问题均已被 Issue14 步骤五（R28-07）、Issue14 步骤七（R28-09）、ProdTestList 人工项、SecurityScanPlan1 未完成步骤覆盖，属于活跃排除项。

### 活跃排除项数量

**17 项**（详见第四章活跃事项排除矩阵）。

### 自动化验证摘要

| 验证项 | 命令 | 结果 |
|--------|------|------|
| 后端编译 | `cd backend && go build ./...` | PASS |
| 后端静态检查 | `cd backend && go vet ./...` | PASS |
| 后端全量测试 | `cd backend && go test ./... -count=1 -timeout 180s` | PASS，39 包全部 ok |
| 前端测试 | `cd frontend && npm test -- --run` | PASS，42 文件 / 259 用例 |
| 前端生产构建 | `cd frontend && npm run build` | PASS |
| Git diff | `git diff --check` | PASS，无警告 |
| Docker build | 环境已有 Docker，但未实际运行 `docker compose build`（避免干扰当前运行容器） | 未执行，受环境约束 |

### 总体完成度结论

**"构建与自动化主体完成"成立；"全部设计预期达成、可直接宣布项目全部完成"尚不成立。**

具体地：
- Build 代码主体完成度：**高**（Build1～Build25 全部归档，自动化全绿）。
- Design1～Design4 实现符合度：**不能判定为全部达成**（Issue14 步骤五未实施的 R28-07 系列仍存在 AGENTS 强要求违反；ProdTestList 大量人工项未执行）。
- 自动化验收完成度：**高**（后端/前端全量测试、生产构建、Production smoke 均通过）。
- Docker/Production 验证完成度：**中等**（正式 Production smoke 通过；真实浏览器/手机/客户端人工核验未完成）。
- AGENTS 强要求符合度：**部分符合**（路径穿越、Token 脱敏、下载缓存、事务、级联删除、构造注入、路由分割等已实现；接入层越层访问、忽略 error、可变包级状态、导入上限等仍未整改）。

---

## 二、核验基线与证据边界

### Git 基线

| 项目 | 值 |
|------|-----|
| 分支 | `beta` |
| HEAD | `a09dc542c37885b400f025b4e469bfd0475bd0c0` |
| HEAD 时间 | 2026-09-11 09:13:36 +0800 |
| HEAD 标题 | 归档 Build22 并闭环 R28-05：补全 Step 7 专属自动化证据，同步更新 Design3/AGENTS/Issue14/15/TODOLIST/ProdTestList 文档状态 |
| `git status --short` | 无输出（工作区干净） |

### 工具链

| 工具 | 版本 |
|------|------|
| Go | go1.26.4 darwin/arm64 |
| Node.js | v26.7.0 |
| npm | 11.19.0 |
| Docker | 29.7.2 |
| Docker Compose | v5.5.1 |

### 工作区状态

工作区干净。无未提交变更、无未跟踪文件。`git diff --check` 通过。

### 环境限制

- 本机运行 macOS，非 Linux 容器环境。
- Docker 可用但未执行 `docker compose build`，避免干扰当前可能运行的 8080 端口容器。
- `.smoke-test-prod.sh` 依赖全新数据库和特定端口，不安全地在当前环境执行可能污染现有数据，故未运行。但 Issue14 步骤一已记录其正式通过。
- 无真实 Production 环境、真实客户端（Clash Verge、Shadowrocket）、真实手机可供本次核验动态验证。
- Build22 的固定 Mihomo 1.19.29 门禁依赖外部二进制 `MIHOMO_11929_BIN`，本次未设置，相关测试为 SKIP 状态（已有历史通过记录）。

### 证据层级说明

| 层级 | 本次状态 |
|------|----------|
| 静态代码证据 | ✅ 通过源码审查验证 |
| 自动化测试证据 | ✅ 后端全量/前端全量通过 |
| Docker build | 未执行（环境约束） |
| Production smoke | 未执行（环境约束；历史通过记录存在于 Issue14） |
| 浏览器真实 UI | 未执行（由 ProdTestList 跟踪） |
| 固定客户端（Mihomo/Clash Verge/Shadowrocket） | 未执行（由 ProdTestList/Issue14 跟踪） |
| 真实连接 | 未执行（由 ProdTestList 跟踪） |

---

## 三、文档与范围盘点

### 本次实际读取的文档范围

| 类别 | 文档 | 状态 |
|------|------|------|
| 强要求 | AGENTS.md | 活跃 |
| 当前设计 | Design4.md | 活跃，v1.20 |
| 候选设计 | Design5.md | 候选构想，v0.1，未定稿 |
| 归档设计 | Design1.md、Design2.md、Design2-UI.md、Design3.md | 已归档 |
| 构建 | Build1.md～Build25.md、Build6-2.md | 全部已归档 |
| 问题 | Issue1.md～Issue13.md | 全部已归档 |
| 当前问题 | Issue14.md | 活跃（步骤一～四、六关闭；五、七、八未关闭） |
| 人工问题 | Issue15.md | 活跃（多数已闭环，R29-01/06/09/10 待人工复验） |
| 人工测试 | ProdTestList.md | 活跃 |
| 安全 | SecurityScanPlan1.md、SecurityReport3.md | 活跃（Step 1～3 完成，Step 4～28 未开始） |
| 核验报告 | BuildReport1.md～BuildReport4.md | 已归档 |
| TODOLIST | TODOLIST.md | 活跃 |
| 部署 | Dockerfile、docker-compose.yml、README.md | 当前 |

### 设计基线

- **Design1.md**（已归档）：第一期基线——工程骨架、认证、权限、订阅、规则、Xray、导入导出。
- **Design2.md + Design2-UI.md**（已归档）：增量基线——规则素材池、装配、四平台输出、UI/UX。
- **Design3.md**（已归档，随 Build22 收口）：规则来源识别、结构化素材、跨平台装配。
- **Design4.md**（当前最新，v1.20）：节点编辑器条件表单、统一保存契约、客户端兼容、未知扩展/局部 JSON。
- **Design5.md**（候选，v0.1）：完整数据加密导出与 Setup 迁移。未定稿，不计入验收范围。

### 活跃跟踪文档

- **Issue14.md**：步骤五（R28-07）、步骤七（R28-09）、步骤八未关闭。
- **Issue15.md**：R29-01/06/09/10 待人工复验。
- **ProdTestList.md**：大量 Production/浏览器/手机/客户端人工项待执行。
- **TODOLIST.md**：P2（步骤五）、P3（步骤七）、P4（步骤八）、P5（安全审查）未完成。
- **SecurityScanPlan1.md / SecurityReport3.md**：Step 4～28 未开始。

### 文档缺失、重复或状态冲突

1. **README 提及 LICENSE 但仓库无 LICENSE 文件**：已在 Issue14 R28-09 登记。
2. **`docs/Reference/Xray-Server-Config-Research.md` 存在仓库外失效链接**：已在 Issue14 R28-09 登记。
3. **前端存在未引用文件**（`GenerateStep.vue`、`PreviewState.vue`、`ResponsiveCollection.vue`、`CopyField.vue`）：已在 Issue14 R28-09 登记。
4. **`PoolTab.vue` 补跑文案已由 Build22 顺带修正**：文档一致性无新增冲突。
5. AGENTS.md 文档清单已更新至 v1.23，包含 BuildReport2/3/4、SecurityReport3 等报告链接，无缺失。

---

## 四、活跃事项排除矩阵

| 临时编号 | 活跃文档/正式编号 | 当前观察 | 排除理由 | 后续主跟踪位置 |
|----------|-------------------|----------|----------|----------------|
| EX-01 | Issue14 R28-07A | 首管理员初始化标记写入不读取 | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-02 | Issue14 R28-07B | 自定义订阅隐藏组 Token 残留 | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-03 | Issue14 R28-07C | 多处 `_ =` 忽略 error（pool/sync.go、server/assembly.go、server/oidc.go 等） | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-04 | Issue14 R28-07D | server/render.go、server/traffic.go、server/oidc.go 直接 `st.DB()`/`TxImmediate` 访问存储 | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-05 | Issue14 R28-07E | `response.debugProvider`、log 包默认 logger 等可变包级状态 | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-06 | Issue14 R28-07F | 验证码 Secret 明文存储/回显 | 用户确认为设计取向，不整改 | Issue14 步骤六（已关闭） |
| EX-07 | Issue14 R28-07G | 导入端点无 20 MiB 上限，`settings_ops.go` 整体读入内存 | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-08 | Issue14 R28-07H | `NodeCheckPanel.vue` 遗留 gray/white 类 | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-09 | Issue14 R28-07I | SSE `/api/admin/logs/stream` 使用一次性查询 Token | 已知活跃跟踪，步骤五待实施 | Issue14 步骤五 |
| EX-10 | Issue14 R28-09 | ca-certificates、LICENSE、失效链接、未引用前端文件、Dockerfile digest | 已知活跃跟踪，步骤七待实施 | Issue14 步骤七 |
| EX-11 | ProdTestList R29-01 | Setup 新库导入 Docker 镜像重建后浏览器复验 | 已知人工项，代码修复已完成 | ProdTestList |
| EX-12 | ProdTestList R29-04 | 真实手机/Production 管理入口 | 已知人工项 | ProdTestList |
| EX-13 | ProdTestList R29-06 | 节点动态表单真实运行核验 | 已知人工项 | ProdTestList |
| EX-14 | ProdTestList Build22 Step 8/9 | 来源状态与装配回执浏览器核验 | 已知人工项 | ProdTestList |
| EX-15 | ProdTestList R27-05/R26-07 | 额外尺寸/焦点/折叠、Xray PUT 错误码 | 已知人工项 | ProdTestList |
| EX-16 | SecurityScanPlan1 Step 4～28 | 安全审查未开始 | 独立安全审查范围，非 Build 遗漏 | SecurityScanPlan1 / SecurityReport3 |
| EX-17 | Design5.md | 完整数据加密导出与 Setup 迁移 | 候选构想未定稿，不计入当前范围 | Design5（未来） |

---

## 五、BuildReport4 发现项回归矩阵

| 原编号 | BuildReport4 结论 | 当前实现证据 | 当前状态 | 是否排除 | 备注 |
|--------|-------------------|--------------|----------|----------|------|
| D3-1 | `no_resolve` 语义丢失 | Build22 Step 3 已修复：结构化解析、逐规则 `NoResolve *bool` 三态、新旧 Clash plan 兼容 | ✅ 已修复 | 否 | Build22 归档 |
| D3-2 | 来源排除数量重复计算 | Build22 Step 1 已修复：`Excluded`/`Rejected`/`Duplicates` 独立计数 | ✅ 已修复 | 否 | Build22 归档 |
| D3-3 | 手工规则污染共享 Canonical | Build22 Step 5 已修复：手工编辑改为换绑 origin，目标已有 manual origin 时返回 409 | ✅ 已修复 | 否 | Build22 归档 |
| D3-4 | 后端未强制素材池能力白名单 | Build22 Step 4 已修复：按原始 legacy 类型拒绝 `SRC-*` 等 | ✅ 已修复 | 否 | Build22 归档 |
| D3-5 | 来源原始证据/顺序未落库 | Build22 Step 2 已修复：`ParsedRule{Rule, Origin}`、真实 `line_no`/`raw_line`/`sort_order` 写入 | ✅ 已修复 | 否 | Build22 归档 |
| D3-6 | 零输出门槛不完整 | Build22 Step 6 已修复：规则型目标无条件执行 `FinalOutput==0` 禁止生成 | ✅ 已修复 | 否 | Build22 归档 |
| D3-7 | per-URL 快照/状态/诊断 API 缺失 | Build22 Step 7 已修复：failed 快照、v1 强类型统计、1018 迁移、`display_url` 状态 API、脱敏 | ✅ 已修复 | 否 | Build22 归档 |
| D3-8 | pending 激活/丢弃无前端 UI | Build22 Step 8 已修复：`PoolDetail.vue` 来源状态/诊断/pending 操作 | ✅ 已修复 | 否 | 浏览器核验待 ProdTestList |
| D3-9 | 装配回执未展示 | Build22 Step 9 已修复：generate 返回回执，前端 `PreviewStep` 渲染 | ✅ 已修复 | 否 | 浏览器核验待 ProdTestList |
| D3-10 | 1015→1016 迁移缺 store 级测试 | Build22 Step 10 已修复：真实 0001～1016 两阶段迁移、幂等、失败回滚测试 | ✅ 已修复 | 否 | Build22 归档 |
| N-core-1 | 首管理员初始化标记只写不读 | 代码中仍有无效写入 | 已知活跃跟踪 | 是 | Issue14 R28-07A |
| N-core-2 | 自定义订阅隐藏组 Token 残留 | 代码中仍可能存在补生 | 已知活跃跟踪 | 是 | Issue14 R28-07B |
| N-core-3 | 仍存在忽略 error | `server/assembly.go`、`server/oidc.go`、`pool/sync.go` 等仍有 `_ =` | 已知活跃跟踪 | 是 | Issue14 R28-07C |
| N-core-4 | 接入层直接访问存储 | `server/render.go`（行22、141）、`server/traffic.go`（行24、37）仍直接 `st.DB()` | 已知活跃跟踪 | 是 | Issue14 R28-07D |
| N-core-5 | 少量包级全局状态 | `response.debugProvider`、log 包默认 logger 仍存在 | 已知活跃跟踪 | 是 | Issue14 R28-07E |
| N-core-6 | 验证码 Secret 明文存储/回显 | 设计取向确认 | 已知排除 | 是 | Issue14 R28-07F（已关闭） |
| N-core-7 | Issue1 R07-05 已超集 | 已被 Build4/Design2 口径覆盖 | 不计入 | 否 | 原报告已说明非缺陷 |
| N-node-1 | 未知 SS 插件 `plugin-opts` 被删除 | Build21 已修复：未知插件保留结构化输出 | ✅ 已修复 | 否 | Build21/Build23 已记录 |
| N-node-2 | `target_evidence` 未用于诊断 | Build21 Step 12 已修复：SS 插件范围内诊断接入 | ✅ 已修复（限 SS 范围） | 否 | 非 SS 全局诊断为设计取向 |
| N-node-3 | VMess SR URI 缺 TLS 参数 | Build21 Step 15 已补全 | ✅ 已修复 | 否 | Build21/Build23 已记录 |
| N-node-4 | VLESS SR URI 缺 ALPN/fp/flow | Build21 Step 15 已补全 | ✅ 已修复 | 否 | Build21/Build23 已记录 |
| N-node-5 | SS v2ray-plugin 检查未警告 | Build21 Step 12 已修复 | ✅ 已修复 | 否 | Build21/Build23 已记录 |
| N-node-6 | 未知扩展/局部 JSON 边界 | Build25 R28-06 已修复 | ✅ 已修复 | 否 | Build25 归档 |
| smoke 脚本 | `.smoke-test.sh` 布尔比较和 `fallback_group_members` 失败 | Issue14 步骤一 R28-02/R28-03 已修复 | ✅ 已修复 | 否 | 正式 Production smoke 已通过 |
| 安全 N01～N07 | 依赖/备份/验证码/重置/JWT/CSP/密码通知 | 用户确认为设计取向 | 已知排除 | 是 | Issue14 R28-08（已关闭） |
| ProdTestList | 大量人工验收未执行 | 仍存在 | 已知活跃跟踪 | 是 | ProdTestList |
| 文档一致性 | AGENTS 版本号/报告清单、README LICENSE 等 | AGENTS 已同步；LICENSE/链接/清理仍在 Issue14 R28-09 | 已知活跃跟踪 | 是 | Issue14 R28-09 |

**回归结论：** BuildReport4 的所有 D3-1～D3-10 已由 Build22 闭环；N-node-1～6 已由 Build21/Build25 闭环；smoke 脚本已修复；安全 N01～N07 已按设计取向关闭；N-core-1～5、导入上限、SSE 鉴权等仍由 Issue14 步骤五跟踪；R27-08/09 已由 Build21 处理。无遗漏回归。

---

## 六、Design1 / Build1～Build8 核验

| Build | 文档声称 | 核验结论 |
|-------|----------|----------|
| Build1 工程骨架与认证 | 完成 | ✅ 已落地：Go 1.26、迁移 0001～0005、auth/setup/oidc/captcha/ratelimit、前端登录/注册/Setup/OIDC |
| Build2 订阅核心与用户端 | 完成 | ✅ 已落地：platform/version/subscription/group/token/download/custom/share/rule/home、迁移 1001～1008 |
| Build3 管理面与运维 | 完成 | ✅ 已落地：用户/审批/邮件/配置/导入导出/备份/日志/应急、SSE、危险操作 |
| Build4 Go1.26+1009+基础模式 | 完成 | ✅ 已落地：go.mod 1.26.0、1009 迁移、规则素材池/高级模式门禁 |
| Build5 装配/manual 节点/代理组 | 完成 | ✅ 已落地：19 协议注册表、代理组、四类装配器、链接渲染、蓝图 |
| Build6 Xray 后端 | 完成 | ✅ 已落地：xray client/instance/credentials/sync/reconcile/quota/cron、任务注册表 |
| Build7 Xray 管理面/收口 | 完成 | ✅ 已落地：Xray UI、高级组/用户/设置、独立账号、OFF 清空、v2 导出 |
| Build8 Issue5 R20 修复 | 完成 | ✅ 已落地：池同步超时/取消、导入保护、候选 fail-closed、错误处理与测试 |

**证据：** 后端 39 包全量测试通过；前端 42 文件 / 259 用例通过；迁移文件 0001～1018 存在且 store 级迁移测试通过（Build22 Step 10）。

---

## 七、Design2 / Build9～Build15 核验

| Build | 结论 |
|-------|------|
| Build9 goccy YAML/自检/RFC5987/规则/代理组扩展 | ✅ 核心已落地，测试通过 |
| Build10 覆盖层/URI 批量导入/池补跑/原子写/收口 | ✅ 核心已落地，测试通过 |
| Build11 UI/UX 与管理员概览 | ✅ 已落地；R24-01 SQLite 单连接死锁已修复 |
| Build12 Token 化/全局浮层/焦点 | ✅ 已落地 |
| Build13 R24-02/12/15 | ✅ 已落地 |
| Build14 R24 第二批 | ✅ 已落地 |
| Build15 R24-19/20 节点表单结构化 | ✅ 已落地 |

**证据：** Build9～Build15 对应的 Issue9～Issue11 R24/R25 系列修复均有代码与回归测试。后端全量测试通过。

---

## 八、Design3 / Build16、Build22 核验

### D3-1～D3-10 逐项核验

| 编号 | 缺口描述 | Build22 修复步骤 | 当前证据 | 结论 |
|------|----------|-----------------|----------|------|
| D3-1 | `no_resolve` 语义丢失 | Step 3 | 结构化解析 `NoResolve *bool`、新旧 Clash plan 兼容、测试覆盖 | ✅ 已闭环 |
| D3-2 | 来源排除数量重复计算 | Step 1 | `Excluded`/`Rejected`/`Duplicates` 独立、5 个新回归用例 | ✅ 已闭环 |
| D3-3 | 手工规则污染共享 Canonical | Step 5 | 换绑 origin、409 保护、测试覆盖 | ✅ 已闭环 |
| D3-4 | 后端未强制素材池能力白名单 | Step 4 | 按原始 legacy 类型拒绝、测试覆盖 | ✅ 已闭环 |
| D3-5 | 来源原始证据/顺序未落库 | Step 2 | `ParsedRule{Rule, Origin}`、`line_no`/`raw_line`/`sort_order` 真实写入 | ✅ 已闭环 |
| D3-6 | 零输出门槛不完整 | Step 6 | `FinalOutput==0` 禁止生成、测试覆盖 | ✅ 已闭环 |
| D3-7 | per-URL 快照/状态/诊断 API 缺失 | Step 7 | failed 快照、v1 强类型统计、1018 迁移、`display_url`、脱敏、专属自动化证据补齐 | ✅ 已闭环 |
| D3-8 | pending 激活/丢弃无前端 UI | Step 8 | `PoolDetail.vue` 来源状态/诊断/pending 操作、22 项组件测试 | ✅ 已闭环（浏览器核验待 ProdTestList） |
| D3-9 | 装配回执未展示 | Step 9 | generate 回执 JSON 合同、前端 `PreviewStep` 渲染、25 项测试 | ✅ 已闭环（浏览器核验待 ProdTestList） |
| D3-10 | 1015→1016 迁移缺 store 级测试 | Step 10 | 真实 0001～1016 两阶段迁移、幂等、失败回滚测试 | ✅ 已闭环 |

### Build22 Step 7 专属自动化证据

2026-09-10 交叉审核发现的 Step 7 自动化证据缺口已全部补齐：

- `ActivatePending`/`DiscardPending` 与 `activated_at` 测试
- `SanitizeStoredSyncOutputs` 幂等/非破坏性清洗测试
- `/sync/status`、`/sync/tasks` 读时脱敏 raw JSON 测试
- `NormalizeDiagnostics` 19+1 与 200 rune 限额测试
- v1 `rule_counts` 分项合计不变量、`previous_active`、旧 stats `version 0` 兼容测试
- `latest_failed` 恢复、同时间戳按 ID 排序测试
- failed 快照写失败时指针不变、不虚报 snapshot ID 测试
- 旧字符串数组 Clash plan 下载重渲染回退测试

**结论：** Build22 全部 Step 1～11 验收通过，D3-1～D3-10 全部闭环。Build22 与 Design3 已归档。

---

## 九、Design4 / Build17～Build21、Build23～Build25 核验

### Build17～Build21

| Build | 结论 |
|-------|------|
| Build17 统一保存契约 | ✅ 1017 迁移、current_state/edit_revision、409、凭据/扩展加密、URI 初始化 |
| Build18 FieldSchema/检查接口 | ✅ 条件/选项/目标证据、活动投影、/check 与夹具 |
| Build19 前端动态表单 | ✅ 可编辑下拉、递归条件、分支清空、局部 JSON、目标检查 UI、409 |
| Build20 全协议过渡/输出门槛 | ✅ 统一归一化、19 协议保存、输出门槛 |
| Build21 BuildReport3 对齐修复 | ✅ 分组/回显/折叠/列表/SS 映射、R27-04/05 已含 |

**R27-01～R27-09 状态（Issue14 步骤一已修复 R27-08/09）：**

| 编号 | 状态 |
|------|------|
| R27-01 可编辑下拉搜索态 | ✅ 已实现 |
| R27-02 递归条件/SS 插件拆分 | ✅ 已实现 |
| R27-03 SMux/Brutal 关闭清空 | ✅ 已实现 |
| R27-04 移除 legacy 可编辑入口 | ✅ 已实现 |
| R27-05 表单顺序/层级/集中开关 | ✅ 已实现 |
| R27-06 allow_custom 三态 | ✅ 已实现 |
| R27-07 saved_sensitive_paths/数组凭据 | ✅ 已实现 |
| R27-08 diagnostics null 崩溃 | ✅ 已修复（Issue14 步骤一） |
| R27-09 SS 插件 Clash 输出回归 | ✅ 已修复（Build21 Step 11/13 + Build23 交接） |

### Build23

已归档。R27-09 与 N-node-1～6 已并入 Build21。仅保留交接关系与研究边界记录。

### Build24

R29-06 节点动态表单控件语义收口。代码与自动化验收通过（4 文件 / 71 定向用例 + 42 文件 / 220 全量用例）。已归档。真实运行核验待 ProdTestList。

### Build25

R28-06 未知扩展/局部 JSON 边界。Step 0～4 全部验收通过：

- **R28-06A**：未知扩展存档/诊断/目标校验。空 targets `unknown_extension_not_targeted`、命中 `unknown_extension_not_rendered`、扩展不进入三类客户端产物。
- **R28-06B**：局部 JSON 显式白名单。`obj()` 默认拒绝未知键、开放 Map 显式白名单、历史未知键读取保留/保存检查阻断/显式删除、前端 `knownFieldNames()` 放行 `item_id_field`。
- **R28-06C**：父子 JSON 草稿协调。后代 dirty 阻断、父阻断展开定位、保存定位稳定排序、条件隐藏清理、折叠/卸载保留。

2026-09-10 交叉审核缺口（`item_id_field` 白名单、保存定位排序、条件隐藏清理）已修复。Build25 已归档。

---

## 十、Issue 历史修复抽查

| Issue | 核验结论 |
|-------|----------|
| Issue1～Issue8 | 已归档，代码与测试证据存在，修复有效 |
| Issue9～Issue11 | R24/R25 系列已修复，回归测试通过 |
| Issue12 | R26 系列已修复，R26-07 人工核验待 ProdTestList |
| Issue13 | R27-01～R27-09 已修复（Build21 + Issue14 步骤一） |
| Issue14 | 步骤一～四、六关闭；五、七、八未关闭（见排除矩阵） |
| Issue15 | R29-02/03/04/05/07/08/11/12 已闭环；R29-01/06/09/10 待人工复验 |

---

## 十一、AGENTS.md 强要求符合度

| 约束 | 结论 | 证据 |
|------|------|------|
| 路径穿越防护 | ✅ 符合 | `filepath.Clean` 在 `static.go:55`、`admin.go:663` |
| 密钥加密存储 | ✅ 符合 | AES-GCM 加密敏感字段，`node/sensitive_paths.go`、`node/sensitive_migration.go` |
| Token/密码/URL 脱敏 | ✅ 符合 | `redact/redact.go` 公共包，`RedactText`/`RedactDisplayURL`，log/pool 共用 |
| 实时权限校验 | ✅ 基本符合 | 管理端点双层中间件；SSE 例外（R28-07I 待整改） |
| 下载禁止缓存 | ✅ 符合 | `download.go:34`、`rule.go:207`、`xray.go:394` 均返回 `no-store` |
| 事务 + 写锁 | ✅ 符合 | `TxImmediate` + 写锁用于版本创建/切换 |
| 原子切换 | ✅ 符合 | 临时对象 + 原子替换模式 |
| 级联删除 | ✅ 符合 | 删除操作完整清理关联数据 |
| 错误处理 | ❌ 部分不符合 | `server/assembly.go:166,185`、`server/oidc.go:222`、`pool/pool.go:438` 等仍有 `_ =` 忽略 error | **已知活跃：Issue14 R28-07C** |
| 构造注入 | ✅ 基本符合 | Handler 结构体 + 依赖传入；例外见可变包级状态 |
| 接入层不直接访问存储 | ❌ 不符合 | `server/render.go:22,141`、`server/traffic.go:24,37` 直接 `st.DB()` | **已知活跃：Issue14 R28-07D** |
| 前端路由级拆分 | ✅ 符合 | Vite 动态 import、管理端子页面按需加载（build 输出可验证） |
| 上传大小和流式处理 | ❌ 不符合 | 导入端点豁免体积上限，`settings_ops.go` 整体读入内存 | **已知活跃：Issue14 R28-07G** |
| 禁止包级全局服务 | ⚠️ 部分符合 | `response.debugProvider`、log 默认 logger 仍存在 | **已知活跃：Issue14 R28-07E** |
| 敏感配置加密 | ⚠️ 部分符合 | SMTP/OIDC 已加密；验证码 Secret 明文 | **已知排除：Issue14 R28-07F（设计取向）** |

**结论：** 核心安全底线（路径穿越、密钥加密、Token 脱敏、实时权限、下载缓存）和数据一致性（事务、级联、原子切换）已实现。接入层越层访问、忽略 error、导入上限、可变包级状态等工程约束违反仍由 Issue14 步骤五跟踪。

---

## 十二、自动化验证记录

### 2026-09-11 实测记录

| # | 时间 | 工作目录 | 命令 | 退出状态 | 关键结果 |
|---|------|----------|------|----------|----------|
| 1 | 09:35 | backend/ | `go build ./...` | 0 | 编译成功，无错误 |
| 2 | 09:35 | backend/ | `go vet ./...` | 0 | 静态检查通过，无警告 |
| 3 | 09:36 | backend/ | `go test ./... -count=1 -timeout 180s` | 0 | 39 包全部 ok，无失败 |
| 4 | 09:37 | frontend/ | `npm test -- --run` | 0 | 42 文件 / 259 用例全部通过 |
| 5 | 09:37 | frontend/ | `npm run build` | 0 | 生产构建成功，80+ chunk 生成 |
| 6 | 09:38 | 根目录 | `git diff --check` | 0 | 无警告 |

**未执行的验证：**

| 验证项 | 原因 |
|--------|------|
| `docker compose build` | 避免干扰当前可能运行的 8080 端口容器 |
| `bash .smoke-test-prod.sh` | 依赖全新数据库和特定端口，环境约束 |
| `bash .mihomo-test.sh` | 需要 `MIHOMO_11929_BIN` 环境变量指向 Mihomo 1.19.29 二进制 |
| 浏览器 UI 测试 | 无 headless 浏览器环境 |
| 真实客户端测试 | 无 Clash Verge/Shadowrocket 环境 |

**失败是否可复现：** 本次未发现测试失败。

**是否可能受环境影响：** 后端和前端测试不依赖外部服务，结果可靠。Docker/Smoke/客户端测试因环境限制未执行。

---

## 十三、Docker、Production、smoke 与人工验收边界

| 验证层级 | 本次执行状态 | 历史通过记录 | 主跟踪 |
|----------|-------------|-------------|--------|
| Docker 多阶段构建 | 未执行 | Issue14 步骤一已通过 | — |
| Production smoke | 未执行 | Issue14 步骤一 R28-02/03 修复后正式通过 | — |
| Mihomo 1.19.29 固定版本门禁 | 未执行 | Issue14 步骤一 R28-04 修复后通过 | — |
| 浏览器真实 UI | 未执行 | PT-28-01～05 已由用户确认完成 | ProdTestList |
| R29-01 Setup 新库导入浏览器复验 | 未执行 | 代码修复+隔离 Production API 验证通过 | ProdTestList |
| R29-04 真实手机管理入口 | 未执行 | 416×928 浏览器核验通过 | ProdTestList |
| R29-06 节点动态表单真实运行 | 未执行 | 自动化通过 | ProdTestList |
| Build22 Step 8 来源状态浏览器 | 未执行 | 自动化 22 项测试通过 | ProdTestList |
| Build22 Step 9 装配回执浏览器 | 未执行 | 自动化 25 项测试通过 | ProdTestList |
| R26-07 Xray PUT 403/400/409 | 未执行 | 自动化回归通过 | ProdTestList |
| OIDC Mock 登录 | 未执行 | 自动化测试通过 | ProdTestList |
| R20-11 重启数据保留 | 未执行 | 环境不可用 | ProdTestList |
| SecurityScanPlan1 Step 4～28 | 未执行 | Step 1～3 已完成 | SecurityScanPlan1 |

---

## 十四、本次新发现

**0 项确定性新发现。**

所有当前代码中可复核的问题均已被 Issue14 步骤五（R28-07A～I）、Issue14 步骤七（R28-09）、ProdTestList 人工项、SecurityScanPlan1 未完成步骤覆盖。未发现不在活跃文档中的新增遗漏。

核验过程中确认的代码状态与活跃文档一致的观察：
- `server/render.go`、`server/traffic.go` 仍直接 `st.DB()`——对应 Issue14 R28-07D。
- `server/assembly.go:166,185` 仍 `_ =` 忽略 error——对应 Issue14 R28-07C。
- 导入端点无上限——对应 Issue14 R28-07G。
- `response.debugProvider` 仍存在——对应 Issue14 R28-07E。
- SSE 端点仍用一次性 Token——对应 Issue14 R28-07I。

以上均已在第四章排除矩阵中登记，不重复计入新发现。

---

## 十五、观察项与待确认事项

### OBS-01：R27-08 diagnostics null 修复后回归覆盖

- **背景：** BuildReport4 指出 `check.go:161` 仍 `nil`、`NodeCheckPanel.vue` 无空值保护。Issue14 步骤一已修复 R28-01（动态插件输入异常），R27-08 的 `diagnostics: []` 契约已由 Build21 处理。
- **当前证据：** Build23 第 51 行记录"R27-08 已在当前代码中修复"。
- **倾向判断：** 已修复，但未找到专门的 null → 空数组防御性回归测试。属于证据充分性观察，不构成新缺陷。
- **需要用户决定：** 无需。仅记录。

### OBS-02：BuildReport4 N-node-3/4 SR 参数完整性

- **背景：** BuildReport4 指出 VMess SR URI 缺 TLS 参数、VLESS SR URI 缺 ALPN/指纹参数。Build21 Step 15 已补全，Build23 确认。
- **当前证据：** `links.go` 中 SR vmess/vless 分支已包含 `tls`/`alpn`/`fp`/`allowInsecure`/`flow`/`skip-cert-verify` 等参数。
- **倾向判断：** 已修复。Shadowrocket 真机兼容性以 PT-28 人工核验为准。
- **需要用户决定：** 无需。

### OBS-03：前端未引用文件清理

- **背景：** BuildReport4 发现 `GenerateStep.vue`、`PreviewState.vue`、`ResponsiveCollection.vue`、`CopyField.vue` 未被引用。
- **当前证据：** 仍存在于源码目录中。
- **倾向判断：** 属于 Issue14 R28-09 步骤七的清理候选，不是功能缺陷。
- **需要用户决定：** 无需。

### OBS-04：Dockerfile 运行镜像 ca-certificates

- **背景：** BuildReport4 指出运行镜像未显式安装 `ca-certificates`。alpine:3.21 自带 ca-certificates 包，但未验证版本。
- **倾向判断：** alpine 默认包含 ca-certificates-bundle，OIDC/SMTP 出站 HTTPS 应可工作。但未在 Dockerfile 中显式声明。属于 Issue14 R28-09。
- **需要用户决定：** 无需。

---

## 十六、最终判定

### 1. Build 代码主体完成度

**高。** Build1～Build25 全部归档。后端 39 包全量测试、前端 42 文件 / 259 用例全量测试、生产构建、`git diff --check` 全部通过。1018 迁移文件完整。260 个后端 Go 文件（含 117 个测试文件）、68 个 Vue 文件、39 个 TS 文件、42 个 spec 文件。

### 2. Design1～Design4 的实现符合度

**不能判定为全部达成。**

- Design1/Design2：已实现并归档，核心功能落地。
- Design3：已由 Build22 完成 D3-1～D3-10 全部收口，已归档。
- Design4：Build17～Build25 主体已完成，R27-01～R27-09 已修复。但 Issue14 步骤五（R28-07 工程约束整改）未实施，存在 AGENTS 强要求违反。
- Design5：候选构想，不计入。

### 3. 自动化验收完成度

**高。** 后端 `go build`/`go vet`/`go test`、前端 `npm test`/`npm run build`、Production smoke（历史通过记录）、Mihomo 固定版本门禁（历史通过记录）均通过。`git diff --check` 无警告。

### 4. Docker/Production 验证完成度

**中等。** Dockerfile 多阶段构建、非 root、单端口、单数据卷符合 AGENTS。正式 Production smoke 已通过（Issue14 步骤一）。但本次未执行 Docker build 或 Production smoke，无独立验证。

### 5. 浏览器与人工验收完成度

**低。** PT-28-01～05 已由用户确认完成。但 ProdTestList 中仍有 R29-01/04/06、R27-05、R26-07、Build22 Step 8/9、OIDC Mock、R20-11 等大量人工项未执行。

### 6. 固定客户端和真实连接验收完成度

**低。** Mihomo 1.19.29 固定版本门禁有历史通过记录。Shadowrocket 真机导入/连接由 PT-28 人工完成。但后续 Build22/Build24/Build25 的真实客户端验证均未执行。

### 7. AGENTS 强要求符合度

**部分符合。** 核心安全底线和数据一致性已实现。接入层越层访问（R28-07D）、忽略 error（R28-07C）、导入上限（R28-07G）、可变包级状态（R28-07E）、SSE 鉴权（R28-07I）等仍待 Issue14 步骤五整改。

### 8. 是否存在不在活跃文档中的新增遗漏

**否。** 所有可复核的问题均已被活跃文档覆盖。

### 9. 是否可以宣布项目全部完成

**不能。** 原因：

1. Issue14 步骤五（R28-07A～I）未实施，存在 AGENTS 强要求违反。
2. Issue14 步骤七（R28-09）未完成，存在 LICENSE 缺失、未引用前端文件等收尾项。
3. ProdTestList 大量人工项未执行，真实浏览器/手机/客户端验证不足。
4. SecurityScanPlan1 Step 4～28 未开始。

---

## 十七、建议后续顺序

1. **Issue14 步骤五（R28-07）**：按 TODOLIST P2 顺序实施 R28-07A/B/D → C/E → G/I → H → 全量回归。这是工程约束整改的核心，完成后将消除 AGENTS 强要求违反。
2. **ProdTestList 人工项**：按 TODOLIST M1～M8 顺序执行 R28-06 人工核验、R29-06 节点表单、R29-04 手机、R29-01 Setup 导入等。
3. **Issue14 步骤七（R28-09）**：LICENSE、ca-certificates、失效链接、未引用前端文件清理。
4. **Issue14 步骤八**：全量复核与 Issue14 关闭。
5. **SecurityScanPlan1 Step 4～28**：工程冻结后执行。

以上事项已由 TODOLIST.md、Issue14.md、ProdTestList.md、SecurityScanPlan1.md 正式跟踪，不在本报告中重新建立平行计划。

---

## 十八、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-11 | 首次创建：对 Design1～Design4、Build1～Build25、Issue1～Issue15、AGENTS.md 强要求的全量核验。记录自动化验证结果、BuildReport4 回归、活跃排除矩阵、AGENTS 符合度、证据边界和最终判定。未修改任何代码或其他文档。 |

### 本次操作声明

- **仅新建：** `docs/reports/BuildReport/BuildReport5.md`
- **未修改：** 任何代码、配置、脚本、数据库迁移、设计文档、Build 文档、Issue 文档、TODOLIST、ProdTestList、安全报告或其他已有文件。
- **未产生：** 运行时产物、临时文件或未跟踪文件。
- **`git status --short`：** 仅新增 `docs/reports/BuildReport/BuildReport5.md`。
