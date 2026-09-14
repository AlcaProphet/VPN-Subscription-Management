# VPN 订阅管理系统 功能构建计划（Build26：Issue14 步骤五 R28-07 核心工程约束整改，已归档）

> **文档定位：** 本文档是 Issue14 步骤五、R28-07A～R28-07I 的**唯一详细构建记录**。承接已归档 [Build25.md](Build25.md)，只处理 R28-07A～E、R28-07G～I；R28-07F 是用户已确认的设计取向，只记录、不实施。本文件不进入 R28-08、R28-09、Issue15 其他问题、`SecurityScanPlan1.md` 或 `Design5.md` 范围。Step 1～20 已验收通过；**本文件已按归档规则移入 `docs/reports/Build/`**，仅作核查，不再作为执行入口。
>
> **关联文档：**
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 当前设计：[Design4.md](../../../Design4.md)（已增加 §12.7“核心工程约束现行合同补充”；不修改归档 Design1）
> - 问题追踪：[Issue14.md](../../../Issue14.md)（步骤五、R28-07A～I）
> - 人工测试：[ProdTestList.md](../../../ProdTestList.md)（未新增人工通过结论，真实浏览器/Production 项目保持未执行）
> - 历史核验：[BuildReport4.md](../BuildReport/BuildReport4.md) §3.2/§6.3、[BuildReport5.md](../BuildReport/BuildReport5.md)
> - 构建模板：[Build.template.md](../../DocTemplates/Build.template.md)
>
> **授权方式（2026-09-11 用户最新确认）：** 用户已一次性授权 Step 1～20 串行执行。允许从 Step 1 开始严格按既定顺序连续实施到 Step 20；单个 Step 通过其全部验收后，无需再次等待单独授权即可进入下一 Step。每一步仍须单独完成失败优先测试、实施、验收和证据记录；只有当前 Step 验收通过才可进入下一 Step。遇到阻断、重大设计选择或文档冲突仍必须停止。不得跳步、并行、同时启动多个代理/工作流处理不同 Step，也不得借一次性授权扩大范围。
>
> **旧口径覆盖说明：** 本节及 TODOLIST 中“每完成一个 Step 后等待下一 Step 单独授权”的旧执行口径由上述最新授权覆盖；范围、顺序、排除项、停止条件和技术决策不变。AGENTS.md 仍是唯一强要求文档，优先级 AGENTS > Design > Build > Issue。

---

## 一、用户已确认决策（2026-09-11）

1. **Build 文档：** 根目录 `Build26.md`，完成后按归档规则移入 `docs/reports/Build/`。
2. **E/C 顺序：** 接受报告建议，E 先于 C；其余按本文构建顺序。
3. **G multipart 余量：** 完整请求体上限固定为 `21 MiB = 20 MiB 文件 + 1 MiB 余量`；文件字段硬上限固定为 `20 × 1024 × 1024` 字节。
4. **D1 渲染服务位置：** 新建 `backend/internal/userrender` 独立包承载用户动态下载渲染业务；`internal/server` 不再包含该业务与直接 SQL。
5. **E 注入结构：** 采用 `log.Runtime`（Logger + 级别控制器）与请求上下文注入；保留 `log.New` 兼容入口，但不再共享可变包级 Logger/LevelVar；Store 使用 `Open` Option 注入 Logger，`config.AdminService` 接收级别控制器。
6. **Error 静态门禁：** 采用类型化 `errgate`（使用 `golang.org/x/tools` 作为工具依赖）识别真正被丢弃的 `error`；合法忽略使用显式 `// errgate:allow <reason>`，comma-ok bool、测试代码和明确清理语义不误报。
7. **I 权限变化：** SSE 流内每 15 秒重查 `active/admin`，权限变化即关闭流；断线后前端重新以会话凭据建立连接。
8. **现行设计落点：** 实施完成后在 `Design4.md` 增加“核心工程约束现行合同补充”，明确 A/F/G/I 的现行口径；不回写归档 Design1，不称历史设计“当时错误”。
9. **C 非关键配置读取：** 对 `mail`、`approval`、`status` 等既有 fail-safe 读取保持对外行为，增加结构化 warn 或 `errgate:allow <reason>`；不得机械改成所有配置读取都返回错误。
10. **最终 Production smoke：** 纳入最终联合门禁，并在隔离环境执行；Production smoke 与自动化、浏览器人工、真实部署证据分别记录。
11. **R28-07F：** 继续记录“设计取向、不实施”；数据库、导入导出、API、管理页原值回显全部不变，不得写成“已修复”。

---

## 二、范围与边界

### 2.1 纳入范围

- R28-07A：删除密码注册和 OIDC 首次建号中的首管理员冗余标记写入。
- R28-07B：自定义订阅优先与隐藏组 Token 的业务键原子协调、历史双 Token 清理、刷新链接纠正。
- R28-07C：生产 Go 代码中被忽略 error 的有边界全量审计、分类处理、失败注入和类型化静态门禁。
- R28-07D：用户动态下载渲染、月流量/配额汇总、OIDC ticket 生命周期移出 `internal/server`；增加架构门禁。
- R28-07E：消除 `response.debugProvider`、默认 Logger/LevelVar、运行期敏感键注册等可变包级状态；统一实例 Logger 与请求上下文。
- R28-07G：`/api/setup/import` 与 `/api/admin/settings/import` 的 20 MiB 文件硬上限、21 MiB 请求体上限、前端提前拒绝、读取错误区分。
- R28-07H：清理 `NodeCheckPanel.vue` 遗留 gray/white 类，增加前端颜色静态门禁和双主题 DOM class 断言。
- R28-07I：SSE 移入管理员路由，前端改 `fetch + ReadableStream`，删除一次性查询 Token，保留历史/增量/重连/清理/连接限制，权限变化 15 秒内关闭流。

### 2.2 明确排除

- R28-07F 的任何代码、数据库、导入导出、API、管理页行为变更。
- R28-08 N01～N07、R28-09 项目级收尾、Issue15 其他问题。
- `SecurityScanPlan1.md` / `SecurityReport3.md` 的步骤、证据、状态。
- 数据库 schema 变更、导入文件格式变更、AES-GCM 整体导入格式改造。
- 归档文档回写；历史绿色结果替代本轮重新验证。
- 未经确认的节点/装配/客户端输出合同变化。

### 2.3 通用执行规则

- 每个 Step 开始前确认前置 Step 已验收；只执行获准的单个 Step。
- 每个 Step 先补失败优先测试或静态门禁，再实施最小修复。
- 每个 Step 完成后执行定向测试、编译、静态检查；汇报真实结果，等待下一 Step 授权。
- 每个 Step 的自动化、Production smoke、浏览器人工、真实部署证据必须分开陈述。
- 不得把未执行的人工项目标记为通过；不得引用历史构建结果代替步骤五修改后的重新验证。
- 任何新设计取舍、范围扩大或文档冲突，立即停止并询问用户。

---

## 三、构建进度追踪

| Step | 内容 | 设计/问题依据 | 状态 |
|---|---|---|---|
| 0 | 创建 Build26 文档、冻结范围、决策和验收边界 | Issue14 步骤五；用户 2026-09-11 决策 | ✅ 本文档创建即验收 |
| 1 | 失败优先回归基座与架构/error/颜色静态门禁 | AGENTS §3.3/§3.4；Issue14 R28-07C/D/H | ✅ 验收通过 |
| 2 | R28-07A 首管理员冗余标记 | Issue14 R28-07A；Design1 §2.5（历史） | ✅ 验收通过 |
| 3 | R28-07B 自定义订阅与隐藏组 Token 协调 | Issue14 R28-07B；Design1 §2.3/§4.2 | ✅ 验收通过 |
| 4 | R28-07D1 用户下载渲染服务化 | Issue14 R28-07D；Design2 §5.7 | ✅ 验收通过 |
| 5 | R28-07D2 流量汇总 Xray 业务化 | Issue14 R28-07D；Design2 §5.8/§5.10 | ✅ 验收通过 |
| 6 | R28-07D3 OIDC ticket 服务化 | Issue14 R28-07D；Design1 §3.2/§5.4 | ✅ 验收通过 |
| 7 | R28-07D4 架构门禁清零 | AGENTS §五；Issue14 R28-07D | ✅ 验收通过 |
| 8 | R28-07E1 debug 请求上下文化 | Issue14 R28-07E；AGENTS §4.3 | ✅ 验收通过 |
| 9 | R28-07E2 Logger/级别控制器实例注入 | Issue14 R28-07E；AGENTS §五 | ✅ 验收通过 |
| 10 | R28-07E3 敏感键固定集合 | Issue14 R28-07E；AGENTS §4.2 | ✅ 验收通过 |
| 11 | R28-07C1 素材池同步错误语义与终态清理 | Issue14 R28-07C | ✅ 验收通过 |
| 12 | R28-07C2 补偿删除与 OIDC ticket 清理错误 | Issue14 R28-07C；R28-07D3 产出 | ✅ 验收通过 |
| 13 | R28-07C3 全量错误审计与 errgate 清零 | Issue14 R28-07C；用户决策 6/9 | ✅ 验收通过 |
| 14 | R28-07G 后端 20 MiB/21 MiB 上限 | Issue14 R28-07G；用户决策 3 | ✅ 验收通过 |
| 15 | R28-07G 前端文件提前拒绝 | Issue14 R28-07G；用户决策 3 | ✅ 验收通过 |
| 16 | R28-07I 后端 SSE 管理员路由与删除 token | Issue14 R28-07I；用户决策 7 | ✅ 验收通过 |
| 17 | R28-07I 前端 fetch/ReadableStream/SSE 解析 | Issue14 R28-07I；用户决策 7 | ✅ 验收通过 |
| 18 | R28-07I 流内权限 15 秒重查 | Issue14 R28-07I；用户决策 7 | ✅ 验收通过 |
| 19 | R28-07H 颜色 Token 与前端静态门禁 | Issue14 R28-07H；用户决策 10 | ✅ 验收通过 |
| 20 | 联合回归、文档同步、关闭条件核验 | Issue14 步骤五关闭条件；用户决策 10/11 | ✅ 验收通过；文档已归档 |

状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过 / ⛔ 阻断。

---

## 四、构建概要（文件清单总览）

| Step | 主要涉及文件 | 要点 |
|---|---|---|
| 0 | `Build26.md` | 唯一构建记录、决策冻结、边界和命令冻结 |
| 1 | `backend/internal/server/architecture_test.go`、`backend/cmd/errgate` 或工具包、`frontend/tests/style-tokens.spec.ts`、测试基座 | 建立允许清单/基线；删项先失败 |
| 2 | `backend/internal/user/{user,oidc}.go`、`internal/config/config.go`、测试 | 删除两处冗余标记写入，回归角色与并发 |
| 3 | `backend/internal/token/token.go`、`internal/home/home.go`、`internal/custom/custom.go`、测试 | 业务键解析/创建/轮替/历史双 token 清理 |
| 4 | 新增 `backend/internal/userrender`；迁移 `server/render.go`、相关测试；`server/server.go` | 动态下载渲染整体迁移，产物不变 |
| 5 | `backend/internal/xray`、`server/traffic.go`、`server/{home,profile,server}.go`、测试 | 流量/配额业务结构体，HTTP 形状不变 |
| 6 | `backend/internal/oidc`、`server/oidc.go`、`server/server.go`、测试 | ticket issue/consume/expired 收回 OIDC 服务 |
| 7 | `backend/internal/server/server.go`、`internal/log/access.go`、架构测试 | `internal/server` 零 `DB()`/`TxImmediate()` |
| 8 | `backend/internal/response/response.go`、`server/server.go`、测试 | debug 经请求上下文和中间件 |
| 9 | `backend/internal/log/log.go`、`store/store.go`、`config/admin.go`、`cmd/server/main.go`、`server/server.go`、测试 | `log.Runtime`、实例 LevelVar、Store Logger、请求上下文 Logger |
| 10 | `backend/internal/config/config.go`、`internal/mail/mail.go`、测试 | 固定敏感键集合，删除运行期注册 |
| 11 | `backend/internal/pool/{sync,snapshot,pool}.go`、定向测试 | Marshal/查询/终态清理错误语义 |
| 12 | `backend/internal/server/assembly.go`、`internal/oidc`、定向测试 | 补偿删除可观测；ticket 删除不虚报 |
| 13 | 第 11/12 步之外的生产错误点、`errgate`、测试 | 全量审计分类、allow 理由、门禁清零 |
| 14 | `backend/internal/server/{hardening,settings_ops}.go`、可能的 `import_upload.go`、测试 | 21 MiB 请求体、20 MiB 文件、EOF/真实错误 |
| 15 | `frontend/src/views/SetupView.vue`、`SettingsView.vue`、可能的 fileLimits 工具、测试 | 前端 20 MiB+1 提前拒绝 |
| 16 | `backend/internal/server/log.go`、`internal/log/stream.go`、`server/server.go`、测试 | 管理员路由 + 删除 token 状态 |
| 17 | `frontend/src/views/admin/LogsView.vue`、`api/log.ts`、`utils/sse.ts`、测试 | fetch/ReadableStream/SSE 分帧/重连/abort |
| 18 | `LogsView`、`LogHandler`、`auth.UserSource` 注入、测试 | 15 秒权限重查关闭流 |
| 19 | `frontend/src/components/NodeCheckPanel.vue`、颜色门禁、组件测试 | `bg-surface-subtle`，双主题断言 |
| 20 | 全仓库受影响文件、`Build26.md`、`Design4.md`、`Issue14.md`、`AGENTS.md`、`ProdTestList.md` | 联合门禁、真实验证、文档同步、关闭条件 |

---

## 五、构建顺序依赖图

```text
Step 0 文档/合同冻结
   ↓
Step 1 门禁与失败优先基座
   ↓
Step 2 A ──→ Step 3 B ──→ Step 4 D1 ──→ Step 5 D2 ──→ Step 6 D3 ──→ Step 7 D4
                                            ↓
                              Step 8 E1 → Step 9 E2 → Step 10 E3
                                            ↓
                              Step 11 C1 → Step 12 C2 → Step 13 C3
                                            ↓
                              Step 14 G后端 → Step 15 G前端
                                            ↓
                              Step 16 I后端 → Step 17 I前端 → Step 18 I权限
                                            ↓
                                        Step 19 H
                                            ↓
                              Step 20 联合回归/文档/关闭核验
```

> 说明：D3 必须先于 C2；E2 必须先于 C1/C2/C3（结构化错误日志和实例 Logger）；I 后端和前端属于同一协议变更，Step 16/17 必须连续实施并在 Step 17 结束后做联合 HTTP 验收；I 完成后再做 H 的最终前端扫描。

---

## 六、逐 Step 构建计划

> 每一 Step 均默认包含：前置条件为本 Step 前一 Step 已验收；影响评估限定在“本 Step 产出文件 + 定向测试”；回滚边界为本 Step 文件级回退且不得影响已验收 Step；范围外为本轮排除项和未授权扩围。以下逐项补充具体内容。

### Step 0：创建 Build26 文档、冻结范围与决策

- **目标：** 创建唯一 Build26 记录，冻结用户决策、Step 顺序、验收口径、排除项。
- **前置条件：** 用户已明确授权“先创建完整 Build26，再执行 Step 1～20”；最新授权为一次性串行执行，每 Step 仍独立验收。
- **产出文件：** 根目录 `Build26.md`。
- **失败优先检查：** 无代码改动；检查本文件是否完整包含 R28-07A～I、F 排除、用户决策、每 Step 验收和联合门禁。
- **验收命令：**

```bash
cd /Users/kylechen/Desktop/Repo/VPN-Subscription-Management
test -f Build26.md
git diff --check
```

> 路径说明（2026-09-11 预检）：原示例路径 `/Users/kyle/Desktop/Repo/VPN-Subscription-Management` 与当前仓库实际路径 `/Users/kylechen/Desktop/Repo/VPN-Subscription-Management` 不一致；本次执行已按 `pwd` 确认的实际路径修正文档示例，未改变构建范围、代码合同或验收标准。

- **验收标准：** Build26.md 存在；`git diff --check` 通过；无其他文件改动；`Issue14.md` 陈旧文字未擅自修改。
- **范围外：** 不执行 Step 1，不修改 Design4/AGENTS/Issue14/ProdTestList，不运行测试/构建/Production smoke。

### Step 1：失败优先回归基座与三类静态门禁

- **目标：** 建立架构门禁、类型化 error 门禁、前端颜色门禁的基座；基座在当前已知违规允许清单/基线下通过，删除任一允许项即失败。
- **前置条件：** Step 0 验收通过。
- **影响评估：** 新增测试/工具代码，不改变生产行为；可能将 `golang.org/x/tools` 转直接工具依赖（用户已确认）。
- **产出文件：**
  - `backend/internal/server/architecture_test.go`
  - `backend/cmd/errgate/` 或等价工具包与基线文件
  - `frontend/tests/style-tokens.spec.ts`
  - 必要的测试辅助/允许清单文件
- **失败优先测试：** 三条门禁均先证明“去掉允许项/引入样例违规即失败”。
- **验收命令：**

```bash
cd backend
go test ./internal/server -run 'ArchitectureNoDirectStore' -count=1
go run ./cmd/errgate ./...              # 命令名以实际产出为准
cd ../frontend
npx vitest run tests/style-tokens.spec.ts
cd ..
git diff --check
```

- **验收标准：** 基座通过；允许清单/基线只含当前已确认内容；error 门禁排除测试文件、comma-ok bool；颜色门禁不误报 `whitespace`/`whitelist`。
- **范围外：** 不修复 A～I 的具体缺陷；不删除当前允许项；不改变生产代码。

### Step 2：R28-07A 删除首管理员初始化冗余标记

- **目标：** 删除 `user.Register` 和 `user.CreateFromOidc` 中的 `KeyAdminInitialized` 写入；继续以同一事务 users 表计数作为唯一事实来源；历史键保留不迁移。
- **前置条件：** Step 1 验收通过。
- **影响评估：** 只删除无读取方的元数据写入；角色、审批、Setup 行为不变；无 schema/API 变化。
- **产出文件：** `backend/internal/user/user.go`、`backend/internal/user/oidc.go`、必要时 `internal/config/config.go` 常量保留/注释、`internal/user/user_test.go`、`internal/oidc/oidc_test.go`。
- **失败优先测试：**
  - 修复前新增测试断言首管理员写入后不存在 `admin_initialized` 新行，旧代码失败；
  - 预置历史 `admin_initialized=true`、users 为空时首注册仍 admin；
  - 密码首注册、OIDC 首建号、后续用户、并发首建、OIDC 并发、待审批用户占表场景。
- **验收命令：**

```bash
cd backend
go test ./internal/user ./internal/oidc -run 'FirstAdmin|ConcurrentFirst|CreateFromOidc|AdminInitialized' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 新首建不再写入标记；历史键不参与任何判断；角色结果与修复前一致；并发只产生一个 admin。
- **范围外：** 不删除历史键、不做迁移、不改 Setup 或审批逻辑、不处理 F。

### Step 3：R28-07B 自定义订阅与隐藏组 Token 协调

- **目标：** 自定义订阅查询前移；存在自定义订阅时只创建/复用自定义 Token 并清理无标识组 Token；不存在自定义且平台有激活版本时才创建/复用组 Token；历史双 Token 幂等清理；刷新链接按业务键原子轮替。
- **前置条件：** Step 2 验收通过。
- **影响评估：** `home`、`token`、`custom` 行为调整；Token 数量/返回值可能变化，但卡片 JSON、下载地址、下载状态码、自定义覆盖语义不变。
- **产出文件：** `backend/internal/token/token.go`、`backend/internal/home/home.go`、`backend/internal/custom/custom.go`、`backend/internal/server/home.go`（仅错误映射需要时）、相关测试。
- **失败优先测试：**
  - 自定义上传后首次首页只存在一个自定义 Token，旧实现产生隐藏组 Token 时失败；
  - 重复首页复用同一 Token；
  - 删除自定义后恢复组 Token；
  - 历史“自定义 Token + 组 Token”并存时协调清理组 Token；
  - 并发首页/并发 resolve 不产生第二个适用 Token；
  - 无自定义且无激活版本不产生 Token；
  - `RefreshToken` 自定义优先、组回退；
  - 卡片 `download_token` 实际解析内容与展示类型一致。
- **验收命令：**

```bash
cd backend
go test ./internal/token ./internal/home ./internal/custom ./internal/server -run 'ResolveUserToken|UserToken|HomePlatforms|RefreshToken|UpsertDeletesGroupToken|HiddenGroup' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 同用户/平台不存在“自定义 Token + 隐藏组 Token”残留；历史双 Token 在业务路径自动修复；页面卡片和下载地址不回归。
- **范围外：** 不改变下载 404/200 合同、不增加数据库唯一索引迁移、不删除历史数据表、不把协调逻辑放在 Handler。

### Step 4：R28-07D1 用户动态下载渲染服务化

- **目标：** 新建 `backend/internal/userrender` 独立业务服务，完整承载 `server/render.go` 的蓝图查询、manual 名称、凭据、目标、Clash/SR/generic 重渲染逻辑；`download.Service` 通过接口/服务调用；保持产物不变。
- **前置条件：** Step 3 验收通过。
- **影响评估：** 移动业务代码和测试；`server/server.go` 的 wiring 变化；下载 HTTP 状态、响应头、文件名、内容不变。
- **产出文件：** 新增 `backend/internal/userrender/*.go`；迁移/删除 `backend/internal/server/render.go`；`internal/download`、`internal/server/server.go`；迁移 `r28_06_download_test.go`、`step7_legacy_plan_test.go`、`render_bench_test.go`。
- **失败优先测试：** 先让架构门禁删去 `render.go` 允许项并失败；迁移后通过；原有下载产物、旧 Clash plan 回退、R28-06 sentinel、benchmark 全部迁移通过。`userrender` 包不得 import Gin/HTTP Handler。
- **验收命令：**

```bash
cd backend
go test ./internal/userrender ./internal/download -run 'Render|R28_06|LegacyPlan|Benchmark' -count=1 -race
go test ./internal/server -run 'Download|ArchitectureNoDirectStore' -count=1
go build ./...
go vet ./...
```

- **验收标准：** `internal/server` 不再含渲染 SQL/全局 slog；下载输出与迁移前一致；架构允许项删除一项。
- **范围外：** 不改变渲染算法、响应头、下载端点、蓝图 schema。

### Step 5：R28-07D2 流量汇总 Xray 业务化

- **目标：** 将 `server/traffic.go` 的月用量、有效配额、`quota_exceeded` 汇总迁入 Xray 业务层，返回不感知 Gin/HTTP 的业务结构体。
- **前置条件：** Step 4 验收通过。
- **影响评估：** `home.summary`、`profile.traffic` 的 JSON 字段名/类型保持；`HomeHandler`/`ProfileHandler` 删除 traffic 相关 `st/cfg/syncSvc` 依赖。
- **产出文件：** `backend/internal/xray/quota.go` 或新增 traffic 文件；删除/收缩 `server/traffic.go`；`server/{home,profile,server}.go`；相关测试。
- **失败优先测试：** 基础模式 `unlimited=true`；高级模式不限/有配额/超限/未超限矩阵；`quota_bytes` null 语义；HTTP JSON 形状与迁移前一致。
- **验收命令：**

```bash
cd backend
go test ./internal/xray ./internal/server -run 'Traffic|Summary|Profile' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** Handler 不再直接查 traffic 表；基础/高级接口形状不变；架构允许项继续减少。
- **范围外：** 不改变流量采集、配额计算、超限行为、响应头 `subscription-userinfo`。

### Step 6：R28-07D3 OIDC ticket 服务化

- **目标：** 将 OIDC 登录 ticket 的创建、消费、过期记录清理收回到 OIDC 服务；Handler 只取 cookie、调用服务、映射响应。
- **前置条件：** Step 5 验收通过。
- **影响评估：** `OidcHandler` 删除 `store` 字段；`issueLoginTicket`/`exchange` 方法迁移；HTTP 状态和 cookie 行为不变；为 C2 提供可测试的错误返回。
- **产出文件：** `backend/internal/oidc` 新增 ticket 服务方法；`backend/internal/server/oidc.go`、`server/server.go`；测试。
- **失败优先测试：** ticket issue/一次性 consume；二次消费失败；过期 ticket 清理；非过期删除错误返回；HTTP exchange 成功/失败状态不变。
- **验收命令：**

```bash
cd backend
go test ./internal/oidc ./internal/server -run 'LoginTicket|Ticket|Exchange|ArchitectureNoDirectStore' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** `server/oidc.go` 无 `TxImmediate`；ticket 生命周期单一服务负责；架构允许项删除一项。
- **范围外：** 不改变 OIDC state/PKCE/回调/绑定逻辑、不改变 `oidc_login_tickets` schema。

### Step 7：R28-07D4 架构门禁清零

- **目标：** `internal/server` 非测试生产文件不再直接调用 `DB()` / `TxImmediate()`；完成 access log 服务装配方式调整。
- **前置条件：** Step 6 验收通过。
- **影响评估：** 仅 wiring/构造函数调整；access log、日志查询、清空功能不变。
- **产出文件：** `backend/internal/server/server.go`、可能的 `internal/log/access.go`、架构测试允许清单清零。
- **失败优先测试：** 架构门禁允许清单清空后先失败并逐项修复；最终零命中。
- **验收命令：**

```bash
cd backend
go test ./internal/server -run 'ArchitectureNoDirectStore' -count=1
go test ./internal/log ./internal/server -run 'AccessLog' -count=1
go build ./...
go vet ./...
```

- **验收标准：** `internal/server` 非测试文件零 `DB()`/`TxImmediate()`；管理端日志查询/清空不回归。
- **范围外：** 不改变日志表结构、不处理 R28-07I。

### Step 8：R28-07E1 debug 请求上下文化

- **目标：** 删除 `response.debugProvider` 全局回调，改为请求上下文中的 debug 标志 + 请求中间件实时读取 `debug_mode`。
- **前置条件：** Step 7 验收通过。
- **影响评估：** `response.Fail` 行为保持；多 Server 实例隔离；无 HTTP 合同变化。
- **产出文件：** `backend/internal/response/response.go`、`backend/internal/server/server.go`、`server/NewEmergency` 及测试。
- **失败优先测试：** 两个 Server 实例 debug 配置不同且互不污染；5xx 详情/脱敏；缺失上下文默认脱敏；400/401/403 不受影响。
- **验收命令：**

```bash
cd backend
go test ./internal/response ./internal/server -run 'Debug|Fail|Isolation|ArchitectureNoDirectStore' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 无包级 debug 回调；每个请求按自身 Server/cfg 决定 5xx 详情。
- **范围外：** 不改变调试开关持久化、错误码、5xx 日志内容语义。

### Step 9：R28-07E2 Logger/级别控制器实例注入

- **目标：** 引入 `log.Runtime{Logger, Level}` 与 `NewRuntime`；保留 `log.New` 兼容但不共享可变 LevelVar；请求上下文注入实例 Logger；Store 通过 `Open` Option 注入 Logger；去掉默认 Logger/全局 `Info/Error/SetLevel` 依赖。
- **前置条件：** Step 8 验收通过。
- **影响评估：** `cmd/server/main.go`、`server/server.go`、`store`、`config.AdminService` 构造/装配调整；日志输出内容保持；所有服务继续使用注入 Logger。
- **产出文件：** `internal/log/log.go`、`internal/store/store.go`、`internal/config/admin.go`、`cmd/server/main.go`、`server/server.go`、标准库 `slog.*` 直接调用点、测试。
- **失败优先测试：** 两个 Runtime 级别隔离；`log.New` 不再改变全局；并发切换级别与日志 `-race`；请求日志/panic/5xx 使用实例 Logger；store 迁移日志进入注入 Logger；直接 `slog.*` 扫描清零或显式允许。
- **验收命令：**

```bash
cd backend
go test ./internal/log ./internal/store ./internal/config ./internal/server -run 'Runtime|LogLevel|Default|Migration|RequestLogger|Race' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 无可变包级 Logger/LevelVar；多 Server 日志级别隔离；标准库 `slog.*` 不再绕过统一 Logger/Redact。
- **范围外：** 不改变日志格式、环境变量语义、日志级别持久化、SSE 缓冲。

### Step 10：R28-07E3 敏感键固定集合

- **目标：** 删除 `config.RegisterSensitive` 与运行期 map 注册；改为编译期固定敏感键判定（至少 `smtp_password`）；验证码双密钥继续明文、不纳入集合。
- **前置条件：** Step 9 验收通过。
- **影响评估：** `smtp_password` 加密/解密行为不变；未知键仍明文；R28-07F 行为不变。
- **产出文件：** `backend/internal/config/config.go`、`internal/mail/mail.go`、相关测试。
- **失败优先测试：** `smtp_password` 加密；未知键明文；无 `RegisterSensitive`；验证码 Site/Secret 明文与原值回显；导入导出不变。
- **验收命令：**

```bash
cd backend
go test ./internal/config ./internal/mail -run 'Sensitive|Captcha|SMTP|Plaintext' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 不再有运行期敏感键注册入口；SMTP 和验证码的既有边界完全保持。
- **范围外：** 不实施 R28-07F、不加密验证码 Secret、不改导入导出格式。

### Step 11：R28-07C1 素材池同步错误语义与终态清理

- **目标：** 处理 `pool/sync.go` 的 JSON 序列化、failed 快照旧 active 查询、终态任务清理错误；按错误语义返回、回滚或结构化记录。
- **前置条件：** Step 10 验收通过；E2 的实例 Logger 已可用。
- **影响评估：** 失败路径可见性和终态 JSON 可靠性提升；成功路径行为、快照 schema、任务 API 不变。
- **产出文件：** `backend/internal/pool/sync.go`、`internal/pool/snapshot.go`、`internal/pool/pool.go` 相关行、定向测试。
- **失败优先测试：** 序列化错误注入；旧 active 查询失败注入；终态清理 Exec 失败注入并断言 pool_id/task_id 日志；失败快照不虚报成功。
- **验收命令：**

```bash
cd backend
go test ./internal/pool -run 'Sync|Snapshot|Terminal|Marshal|Cleanup' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 无非 EOF/真实语义错误被静默吞；终态写入失败有定位上下文；快照统计不再静默失真。
- **范围外：** 不改变池同步算法、URL 拉取、阈值/快照状态机。

### Step 12：R28-07C2 补偿删除与 OIDC ticket 清理错误

- **目标：** 处理 `server/assembly.go` 自动建规则补偿删除错误和 D3 迁移后的 OIDC ticket 过期删除错误；必要时用 `errors.Join` 与主错误共同返回。
- **前置条件：** Step 11 验收通过；Step 6 已完成 ticket 迁移。
- **影响评估：** 补偿失败可观测；主 HTTP 状态码不变；不泄漏内部详情（500 脱敏）。
- **产出文件：** `backend/internal/server/assembly.go`、`internal/oidc` ticket 服务、定向测试。
- **失败优先测试：** 自动建规则成功后补偿删除失败；ticket 过期删除失败；非过期删除失败回滚；断言日志/错误链。
- **验收命令：**

```bash
cd backend
go test ./internal/server ./internal/oidc -run 'Compensation|AutoRule|Ticket' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 补偿/清理失败有结构化记录；不会把补偿失败静默当作成功，也不会把 400 内部细节泄漏到客户端。
- **范围外：** 不改变装配生成合同、ticket cookie 合同、OIDC 登录流程。

### Step 13：R28-07C3 全量错误审计与 errgate 清零

- **目标：** 完成生产 Go 代码剩余错误审计；对 correctness、补偿/清理、异步状态写入分类处理；清理 error 门禁基线/允许项；无理由忽略为零。
- **前置条件：** Step 12 验收通过。
- **影响评估：** 涉及多个包的错误路径；按用户决策保持 fail-safe 路径的外部行为并增加 warn/allow 理由；不改变成功路径和 HTTP 合同。
- **产出文件：** 第 11/12 步之外的生产代码、`errgate` 允许清单、所有定向测试。
- **失败优先测试：** 逐点失败注入；类型化门禁删除允许项先失败；完整门禁通过。
- **验收命令：**

```bash
cd backend
go test ./internal/pool ./internal/server ./internal/share ./internal/rule ./internal/user ./internal/xray ./internal/config ./internal/assembly ./internal/uriparse ./internal/node -run 'Error|Ignore|Fail|Allow' -count=1 -race
go run ./cmd/errgate ./...
go test ./... -count=1
go vet ./...
```

- **验收标准：** 所有 `_`/`, _` 均有真实修复、显式允许理由或是门禁可识别的无错误语义；`errgate` 无新违规。
- **范围外：** 不处理 R28-07I 前端 SSE 文件、不改变 R28-08 安全设计取向、不混入 SecurityScanPlan1。

### Step 14：R28-07G 后端 20 MiB/21 MiB 上限

- **目标：** 两个导入端点文件硬上限 20 MiB、完整请求体 21 MiB；无 Content-Length/分块/伪造长度全部受限；超限 413；区分 EOF 与真实读取错误；真实错误不创建任务。
- **前置条件：** Step 13 验收通过。
- **影响评估：** 导入请求处理错误码变化仅限超限/读取失败；导入格式、Argon2id/AES-GCM 流程、任务返回不变；R29-01 的 Setup 仅 IMPORT / 管理端 IMPORT+DISABLE 语义保持。
- **产出文件：** `backend/internal/server/hardening.go`、`backend/internal/server/settings_ops.go`、可能的 `import_upload.go`、`internal/server/*_test.go`、`internal/config/export_v2_test.go` 保护性回归。
- **失败优先测试：** 恰好 20 MiB 通过大小检查；20 MiB+1 413；请求体 >21 MiB 413；无 Content-Length；chunked；伪造 Content-Length；截断/中途 reader error；Setup 与管理双入口；超限/读取失败零任务创建；R29-01 三分支回归。
- **验收命令：**

```bash
cd backend
go test ./internal/server ./internal/config -run 'Import.*(Limit|Body|Chunk|Truncate|Read|Setup|Admin|ZeroTask|Disable)' -count=1
go build ./...
go vet ./...
```

- **验收标准：** 文件 20 MiB 边界正确；完整请求体 21 MiB；所有读取错误不创建任务；R29-01 行为不变。
- **范围外：** 不改变导入格式、不流式化 AES-GCM、不处理 R29-01 的人工复验状态。

### Step 15：R28-07G 前端文件提前拒绝

- **目标：** Setup 和管理端设置两个导入入口在选择文件时拒绝 >20 MiB，恰好 20 MiB 允许。
- **前置条件：** Step 14 验收通过。
- **影响评估：** 仅上传前校验和提示；合法文件流程不变。
- **产出文件：** `frontend/src/views/SetupView.vue`、`frontend/src/views/admin/SettingsView.vue`、可能的 `frontend/src/utils/fileLimits.ts`、测试。
- **失败优先测试：** 20 MiB 接受；20 MiB+1 拒绝；错误提示；不影响 R29-01 Setup 导入。
- **验收命令：**

```bash
cd frontend
npm test -- --run tests/settings-view.spec.ts tests/import-limit.spec.ts
npm run build
```

- **验收标准：** 两个界面都在文件选择阶段拒绝超限；不发送超限请求。
- **范围外：** 不修改 `<input>` 上传协议、不改变密码/确认词交互。

### Step 16：R28-07I 后端 SSE 管理员路由与删除 token

- **目标：** `/api/admin/logs/stream` 移入会话+管理员路由组；删除 `/stream/token`、`IssueToken`、`ConsumeToken`、Token map/TTL 和相关复位逻辑；保留历史缓冲、增量、连接限制。
- **前置条件：** Step 15 验收通过。
- **影响评估：** SSE 建立方式改变；HTTP 401/403 行为符合 AGENTS；查询 token 不再存在；日志查询/清空不变。
- **产出文件：** `backend/internal/server/log.go`、`backend/internal/log/stream.go`、`backend/internal/server/server.go`、测试。
- **失败优先测试：** 未登录 401；普通用户 403；管理员 200；`/stream/token` 404；历史先推、增量推送；8 连接上限；`Reset` 只清缓冲/连接，不再清 token。
- **验收命令：**

```bash
cd backend
go test ./internal/log ./internal/server -run 'SSE|Stream|Token|ConnectionLimit|Reset' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 无一次性查询 Token 状态；流端点有会话+管理员双层中间件；历史/增量/连接限制保留。
- **范围外：** 不改前端（Step 17）、不改日志缓冲大小、不处理权限重查（Step 18）。

### Step 17：R28-07I 前端 fetch/ReadableStream/SSE 解析

- **目标：** 前端改用 `fetch` 携带 Authorization，解析 `ReadableStream` SSE 帧；保留暂停/清屏/级别过滤/最多 3 次重连提示/卸载 abort；删除 `issueStreamToken`。
- **前置条件：** Step 16 验收通过。
- **影响评估：** 日志实时页连接方式变化；401/403 停止重连并提示；不再有 query token。
- **产出文件：** `frontend/src/views/admin/LogsView.vue`、`frontend/src/api/log.ts`、新增 `frontend/src/utils/sse.ts`、`frontend/tests/sse-parser.spec.ts`、`frontend/tests/logs-view.spec.ts`。
- **失败优先测试：** SSE 分帧、跨 chunk、多行 `data:`、空行、注释、CRLF；Authorization 头；无 query token；401/403 行为；断线重连；组件卸载 abort；暂停/清屏/过滤不回归。
- **验收命令：**

```bash
cd frontend
npm test -- --run tests/sse-parser.spec.ts tests/logs-view.spec.ts
npm run build
```

- **验收标准：** fetch 携带现有会话凭据；SSE 解析正确；无 EventSource 查询 token；清理和重连行为完整。
- **范围外：** 不改访问日志查询/清空、不改后端路由（Step 16）。

### Step 18：R28-07I 流内权限 15 秒重查

- **目标：** SSE 流建立后每 15 秒通过 `auth.UserSource` 实时查库；若用户缺失、非 active 或非 admin，关闭流并清理订阅。
- **前置条件：** Step 17 验收通过。
- **影响评估：** 每条约 15 秒一次轻量查询，最多 8 条连接；权限变化 15 秒内生效；前端感知为断流后按既有重连策略尝试，若 401/403 停止重连。
- **产出文件：** `backend/internal/server/log.go`、`server/server.go`、`auth.UserSource` 适配、测试。
- **失败优先测试：** 流中 admin→user、active→disabled、用户删除时流关闭；权限不变时不误断；并发/卸载清理；`-race`。
- **验收命令：**

```bash
cd backend
go test ./internal/server -run 'StreamPermission|PermissionChange|SSE' -count=1 -race
go build ./...
go vet ./...
```

- **验收标准：** 权限变化最长约 15 秒内关闭；无 goroutine/连接泄漏；日志查询/清空不回归。
- **范围外：** 不引入事件总线、不改下载 Token 实时权限、不改前端重连上限。

### Step 19：R28-07H 颜色 Token 与前端静态门禁

- **目标：** `NodeCheckPanel.vue` 预览背景改用设计 Token；颜色静态门禁零违规；补浅色/深色 DOM class 断言。
- **前置条件：** Step 18 验收通过；I 的前端文件已冻结。
- **影响评估：** 仅视觉背景类和前端测试；不改变检查行为/数据合同。
- **产出文件：** `frontend/src/components/NodeCheckPanel.vue`、`frontend/tests/style-tokens.spec.ts`、`frontend/tests/node-check-panel.spec.ts`、必要时 `theme.spec.ts`。
- **失败优先测试：** 旧代码颜色门禁失败；修复后通过；预览 `<pre>` 含 `bg-surface-subtle` 且不含 `bg-gray-50`；light/dark 断言一致。
- **验收命令：**

```bash
cd frontend
npx vitest run tests/style-tokens.spec.ts tests/node-check-panel.spec.ts tests/theme.spec.ts
npm test -- --run
npm run build
```

- **验收标准：** `frontend/src` 无 gray/white Tailwind 颜色工具类违规；两种主题类名下 DOM class 符合 Token；不误报 `whitespace`/`whitelist`。
- **范围外：** 不重做 R29-06、不改终端固定深色样式、不扫描第三方内容。

### Step 20：联合回归、文档同步与关闭条件核验

- **目标：** 对步骤五全部修改重新执行后端定向/全量/race/build/vet、前端定向/全量/build、接口级 401/403/413、Docker build、隔离 Production smoke、`git diff --check`；同步文档；仅在所有条件满足后评估步骤五关闭。
- **前置条件：** Step 1～19 全部验收通过。
- **影响评估：** 无新增业务逻辑；只做真实门禁、文档同步和状态核验。
- **产出文件：** `Build26.md`、`Design4.md`、`Issue14.md`、`AGENTS.md`、`ProdTestList.md`，必要时 `Issue15.md` 交叉引用。
- **失败优先/最终验收命令：**

```bash
cd backend
go test ./... -count=1 -timeout 300s
go test -race ./internal/log ./internal/response ./internal/server ./internal/home ./internal/token ./internal/pool ./internal/xray ./internal/oidc ./internal/config ./internal/custom ./internal/user
go run ./cmd/errgate ./...
go test ./internal/server -run 'ArchitectureNoDirectStore|SSE|Import|401|403|413' -count=1
go build ./...
go vet ./...

cd ../frontend
npm test -- --run
npm run build

cd ..
git diff --check
docker compose build
```

- **接口级回归清单：** SSE 401/403/管理员可建立流；旧 `/stream/token` 不存在；导入 Setup/管理双入口 20 MiB 边界/超 1 字节/分块/伪造长度/截断；超限或读取失败零任务创建；权限变化行为。
- **Production smoke：** 在隔离环境执行正式脚本；单独记录结果，不与自动化混写。
- **人工边界：** 若新增 SSE 真实浏览器、导入体验、主题视觉等人工项，登记到 `ProdTestList.md` 并保持未执行；不得提前标记通过。
- **文档同步：**
  - `Issue14.md`：修正 `Issue14.md:80` 的“步骤三仍有未关闭缺口”陈旧文字；更新 R28-07 状态、证据类型、步骤五关闭条件和变更记录；
  - `Design4.md`：增加“核心工程约束现行合同补充”，覆盖 A/B/D/E/G/I 的现行口径，并明确 F 不实施；
  - `AGENTS.md`：同步 Build26 状态、当前构建记录、Design4 现行合同入口；不写具体设计细节；
  - `ProdTestList.md`：仅登记真实未执行人工项；
  - 不修改任何归档文档，不把后续变化写成“原报告错误”。
- **验收标准：** 全部门禁真实通过；R28-07A～E、G～I 有代码、定向回归、全量门禁和文档同步证据；R28-07F 仍记录为设计取向；步骤五方可评估关闭。
- **范围外：** 不处理 R28-08/R28-09、Issue15 其他问题、SecurityScanPlan1；不把自动化或 Production smoke 替代浏览器人工结论。

---

## 七、失败优先与证据层级规则

- **修复前必须失败：** 每个 Step 新增的行为测试、错误注入或门禁删除允许项，必须先证明旧代码/旧文档状态失败。
- **自动化证据：** 定向测试、race、build、vet、前端测试、接口级 httptest 回归。
- **Production smoke：** 正式仓库脚本，隔离环境；单独记录，不替代浏览器人工。
- **浏览器人工：** 由用户在真实 Production/浏览器执行；本轮登记后不得提前通过。
- **真实部署/客户端证据：** 单独记录；单元测试、离线解析、Docker build 不替代。
- **历史结果：** Build22/Build24/Build25 或 BuildReport5 的绿色结果不能替代步骤五修改后的重新验证。

---

## 八、风险、回滚与停止条件

- **A/B：** 行为风险低；数据库无 schema 变化，回滚仅回退 Go 文件。
- **D1～D3：** 业务迁移风险中；每步先保留原有测试并迁移，产物/HTTP 合同回归失败则回退该 Step 文件与 wiring。
- **E：** 构造 API 变化风险中；`log.New` 兼容入口保留，先迁移 main/server/store/config 再清理全局。
- **C：** 错误路径行为变化风险中；失败注入必须先证明预期，补偿错误只记录/按需 Join，不改变成功路径。
- **G：** 兼容风险低；20 MiB 是新硬边界，R29-01 行为必须保持。
- **I：** 前后端协议联动风险中；Step 16/17 必须连续实施并在 Step 17 后做联合 HTTP 验收，不能只通过单端测试就宣告完成。
- **H：** 视觉风险低；静态扫描 + 双主题断言。
- **停止条件：** 发现需求疑点、文档冲突、多种合理方案、影响 Build17～Build25 既有行为、范围外问题或需要改变用户已确认决策时，立即停止并询问，不自行扩围。

---

## 九、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-11 | 初始版本：按用户授权创建 Build26，冻结 R28-07A～I 范围、用户 2026-09-11 决策、Step 1～20 逐 Step 计划、静态门禁、失败优先测试、最终联合门禁和文档同步边界；R28-07F 明确不实施。 |
| v1.1 | 2026-09-11 | Step 20 联合回归、文档同步与关闭条件核验通过：后端全量/race/build/vet、errgate/架构/SSE/导入接口回归、前端全量/build、Docker build、隔离 Production smoke 与 `git diff --check` 均通过；A～E、G～I 关闭、F 保持设计取向；同步 Design4/Issue14/AGENTS/TODOLIST/ProdTestList，ProdTestList 新增真实浏览器/Production 人工项且未标记通过；本记录按归档规则移入 `docs/reports/Build/Build26.md`。 |


---

## 十、实施日志（预检与逐 Step 记录）

### 预检记录（2026-09-11 22:37 CST，Step 0 前置检查）

- **规范化绝对路径：** `/Users/kylechen/Desktop/Repo/VPN-Subscription-Management`（`pwd` 确认）。原示例路径修正见 Step 0；该修正有事实依据，不改变构建范围、代码合同或验收标准。
- **Git 基线：** 分支 `beta`；HEAD `ea65ef41521dc32efa0507c5637e47779052e01c`（2026-09-11T15:15:58+08:00，标题为“新增 Build27 构建计划：整合 R28-09 用户决策与执行范围，包含 Step 0 完成状态与后续步骤定义”）；`git status --short`、`git diff --stat`、`git diff --name-only` 及暂存区差异均为空；工作树无未提交修改，本次执行基线为干净工作树。
- **Build26 状态核对：** Build26 Step 0 的文档、范围、用户决策、Step 1～20 顺序和验收边界均已存在；Step 1～20 未执行；仓库中未发现 `internal/userrender`、`cmd/errgate`、`architecture_test.go`、`frontend/tests/style-tokens.spec.ts`、`frontend/src/utils/sse.ts` 等本轮新文件，也未发现与 R28-07A～I 对应的提前实现。
- **文档阅读记录：** 已完整阅读 `AGENTS.md`、`Build26.md`、`TODOLIST.md`、`Issue14.md` 步骤五与 R28-07A～I、`Issue15.md` 中 R29-01/R29-12 及交叉引用内容、`Design4.md` 中 Build26 依赖的现行节点/未知扩展/JSON/API 契约、`ProdTestList.md` 中导入/SSE/主题/人工边界；并核验 `BuildReport4.md` §3.2/§6.3、`BuildReport5.md`、历史 `Design1.md` §2.3/§2.5/§3.2/§4.2/§5.4、`Design2.md` §5.7/§5.8/§5.10 等引用段落。归档文档仅用于核查，未回写。
- **状态一致性：** `AGENTS.md` 与 `TODOLIST.md` 仍保留“Step 1～20 等待逐 Step 授权”的旧口径；该陈旧的授权状态已在本预检后按用户最新一次性串行授权修正。`Issue14.md` 步骤五状态与 Build26 Step 0 描述一致；步骤三已关闭、步骤四已工程闭环，不存在与 Build26 前置条件冲突的记录。
- **工具链与命令入口：** Go `go1.26.6 darwin/arm64`、Node `v26.7.0`、npm `11.19.0`、Docker `29.7.2`、Docker Compose `v5.5.1` 可用；`backend/go.mod` 位于 `backend/`，Go 命令须从 `backend/` 执行；`golang.org/x/tools v0.47.0` 当前为间接依赖，与用户已确认的 errgate 工具依赖方向一致。
- **已有工作树修改归属：** 无未提交修改、无暂存修改、无未跟踪任务实现。仓库 HEAD 中存在 `Build27.md`，属于另一构建计划的已提交文档；它不是 Build26 执行入口，本次不修改、不执行、不将其状态混入 Build26。
- **路径与数据隔离：** 本轮不要求清空测试数据；`backend/data` 未发现需处理的既有数据库文件。后续如需测试数据，按 AGENTS.md §6.1 仅操作经确认的仓库本地 `backend/data`。
- **开工疑点审计结论：** 未发现 AGENTS/Design/Build/Issue 之间阻断 Step 1 的实质冲突；未发现 Build26 内部互相矛盾；未发现需要数据库 schema、HTTP/JSON/下载合同、R29-01 IMPORT/DISABLE 语义或 R28-07F 变更才能开工的事项；未发现会不可避免改变 Build17～Build25 已验收行为的事项；未发现需要真实凭据、真实外部服务或隔离测试环境以外数据的事项。Step 1 所需的 `golang.org/x/tools` 已在依赖树中，无需引入新的重型依赖。
- **最新授权生效：** 用户已一次性授权 Step 1～20 串行执行；每 Step 仍按失败优先测试、最小实施、定向验收、证据记录后自动进入下一步；遇阻断、重大设计选择或文档冲突停止。不得并行、跳步或扩围。
- **预检结论：** 无阻断 Step 1 的未决问题；下一步为 **Step 1：失败优先回归基座与三类静态门禁**。本节仅完成预检记录，尚未修改业务代码；Step 1 开始时将把 Build26 进度表中的 Step 1 标记为 ◧ 进行中。


### Step 1：失败优先回归基座与三类静态门禁

- **Step 编号和标题：** Step 1：失败优先回归基座与三类静态门禁。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 22:38～22:43 CST（预检完成后立即串行开始）。
- **前置条件检查：** Step 0 已完成；预检无阻断；一次性串行授权已记录；工作树基线干净。
- **影响评估：** 仅新增测试、静态门禁工具与测试依赖；未修改生产 Go/前端行为。`golang.org/x/tools` 从间接依赖提升为直接工具依赖（用户已确认）；前端测试配置为读取 `node:fs` 补充 `node` 类型，不改变构建产物行为。
- **工作树基线重叠情况：** 基线无未提交修改；本 Step 新增文件未与用户修改重叠。

**失败优先测试：**
- 架构门禁：先以空允许清单运行 `go test ./internal/server -run 'TestArchitectureNoDirectStore' -count=1`，稳定失败并列出 `oidc.go:189/214`、`render.go:22/141`、`server.go:439`、`traffic.go:24/37` 共 7 个 `DB()/TxImmediate()` 直访点；证明门禁能真实识别当前违规。
- error 门禁：先运行 `go run ./cmd/errgate ./...`，无基线时稳定非零退出，列出 86 条生产代码被丢弃 error；证明类型化工具在旧实现上识别真实缺口。
- 颜色门禁：先以空允许清单运行 `npx vitest run tests/style-tokens.spec.ts`，稳定失败并报 `src/components/NodeCheckPanel.vue:110 bg-gray-50`；证明扫描器真实命中遗留颜色类。

**实施方案：**
- `backend/internal/server/architecture_test.go`：AST 扫描 `internal/server` 非测试生产文件中的 `.DB()`/`.TxImmediate()`；当前以文件级允许清单记录 4 处待服务化文件（Step 4～7 每步拆除对应条目），并防止允许多余/过期条目。
- `backend/cmd/errgate/main.go`：基于 `golang.org/x/tools/go/packages` 加载真实类型信息，识别显式空白标识符丢弃的 error 结果及返回 error 的表达式调用；排除测试文件、comma-ok bool、Close/Rollback 清理、`fmt` 诊断输出、内存 writer 等明确无错误语义，支持 `// errgate:allow <reason>`。默认读取 `backend/errgate_baseline.json`。
- `backend/errgate_baseline.json`：由工具 `-write-baseline` 生成的 86 项当前基线，reason 统一记录“Build26 Step 1 当前已确认忽略点；待 R28-07C3 分类修复/允许”；Step 11～13 将逐项拆除或补真实处理/allow 理由。
- `frontend/tests/style-tokens.spec.ts`：扫描 `frontend/src` 中 gray/white/black Tailwind 颜色工具类；当前仅登记 `NodeCheckPanel.vue` 的 `bg-gray-50`，并自测不误报 `whitespace`/`whitelist`。
- `frontend/tsconfig.json`：仅在 tests 使用 `node:fs` 的静态扫描场景增加 `node` types；不改业务 TS 目标或运行时行为。

**修改文件：** `backend/internal/server/architecture_test.go`、`backend/cmd/errgate/main.go`、`backend/errgate_baseline.json`、`frontend/tests/style-tokens.spec.ts`、`backend/go.mod`、`frontend/tsconfig.json`。

**自主决策及依据：**
- errgate 将显式 Clear/Rollback、`fmt` 诊断输出、`strings.Builder`/`bytes.Buffer` 内存写、`os.Remove` 失败清理视为“明确无业务错误语义”不误报；依据用户决策 6/9 和 Build26“comma-ok bool、测试代码和明确清理语义不误报”的边界。baseline 仍保留其余 86 条真实忽略点作为 R28-07C 审计入口。
- 架构允许项按文件而非行号登记，避免后续服务化移动代码时行号漂移导致门禁噪声；文件级允许项必须最终在 Step 7 清空。
- 前端静态扫描使用 `node:fs`，因此补充 `node` types；只影响测试类型检查。

**验收命令与真实结果：**
- `cd backend && go test ./internal/server -run 'ArchitectureNoDirectStore' -count=1`：通过。
- 空 allowlist 反向验证：Step 1 首次运行同一命令失败，7 项违规；证明删除允许项即失败。
- `cd backend && go run ./cmd/errgate ./...`：通过，输出 `OK (86 production packages; 86 allowed ignored errors; 0 unexpected)`。
- `cd backend && go run ./cmd/errgate -baseline= ./...`：非零退出，列出全部 86 项未基线违规；证明删除基线即失败。
- `cd frontend && npx vitest run tests/style-tokens.spec.ts`：通过，1 文件/2 用例。
- 空 allowlist 反向验证：首次运行同一命令失败，命中 `src/components/NodeCheckPanel.vue:110 bg-gray-50`；证明删除允许项即失败。
- `cd backend && go build ./...`：通过；`go vet ./...`：通过；`cd frontend && npm run build`：通过（仅既有 chunk >600kB 提示）。
- `git diff --check`：通过。

**验收标准逐项结论：** 基座通过；三类允许清单/基线只含当前已确认内容；error 门禁排除测试文件与 comma-ok bool；颜色门禁不误报 `whitespace`/`whitelist`；删除允许项/基线均已有失败证据；未修复 A～I；未删除当前允许项；未改变生产行为。全部满足。
- **自动化证据边界：** 本 Step 只证明门禁能识别当前代码；不证明后续缺陷已修复，不替代 Step 2～20 的定向与失败注入。
- **Production smoke 边界：** 未执行，本 Step 无服务端行为变更。
- **浏览器人工边界：** 未执行；颜色视觉断言留给 Step 19 自动化与 ProdTestList 人工项。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** errgate 基线 86 项尚未分类；架构 4 个文件允许项和颜色 1 项允许项必须在后续 Step 拆除；若后续 Step 改动表达式文本，工具会明确报“基线过期”，需随对应 Step 同步更新。
- **下一步：** 进入 Step 2：R28-07A 删除首管理员初始化冗余标记（当前 Step 已验收通过）。

### Step 2：R28-07A 删除首管理员初始化冗余标记

- **Step 编号和标题：** Step 2：R28-07A 首管理员初始化冗余标记。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 22:43～22:45 CST。
- **前置条件检查：** Step 1 已 ✅ 验收通过；一次串行授权有效；工作树仅有 Step 1 新增/修改。
- **影响评估：** 删除 `Register`/`CreateFromOidc` 中无读取方的 `admin_initialized` 写入；首管理员仍由同一 `BEGIN IMMEDIATE` 事务内 users 空表计数决定。无 schema、Setup、审批、角色或 API 变化；历史键保留但不读取、不迁移。
- **工作树基线重叠情况：** 与 Step 1 文件不重叠；`config.go` 仅在 Step 10 还会再次修改，本 Step 只更新历史键注释。

**失败优先测试：**
- 新增 `TestFirstAdminDoesNotWriteAdminInitialized`、`TestFirstOidcAdminDoesNotWriteAdminInitialized`、`TestConcurrentFirstOidcAdmin`：旧实现分别稳定失败，实际报告 `admin_initialized` 1 行。
- 新增 `TestHistoricalAdminInitializedIgnored`、`TestPendingUserOccupiesTableBlocksFirstAdmin`，锁定历史标记不参与判断、待审批占表阻断首管理员；旧实现通过，作为保护性回归。
- 新增 `TestResolveLoginFirstAdminDoesNotWriteAdminInitialized` 走完整 OIDC `ResolveLogin` 路径，旧实现失败：实际 `admin_initialized` 1 行。

**实施方案：** 删除 `backend/internal/user/user.go` 与 `backend/internal/user/oidc.go` 中的两处 `SetTx(config.KeyAdminInitialized, "true")`；更新函数注释为 users 表计数唯一事实来源；`config.KeyAdminInitialized` 保留为历史兼容常量并在注释中明确不写入/不判断/不迁移。

**修改文件：** `backend/internal/user/user.go`、`backend/internal/user/oidc.go`、`backend/internal/config/config.go`、`backend/internal/user/user_test.go`、`backend/internal/oidc/oidc_test.go`。

**自主决策及依据：** 保留 `KeyAdminInitialized` 常量而非删除，是为历史库导出、兼容读取和测试定位保留名称，符合 Build26“历史键保留不迁移”；不删除历史 DB 行，避免 schema/数据变更。

**验收命令与真实结果：**
- 修复前：`go test ./internal/user -run 'TestFirstAdminDoesNotWriteAdminInitialized|TestFirstOidcAdminDoesNotWriteAdminInitialized|TestConcurrentFirstOidcAdmin'`：失败（3 处实际 1 行）。
- 修复前：`go test ./internal/oidc -run 'TestResolveLoginFirstAdminDoesNotWriteAdminInitialized'`：失败（实际 1 行）。
- 修复后：`go test ./internal/user ./internal/oidc -run 'FirstAdmin|ConcurrentFirst|CreateFromOidc|AdminInitialized|ResolveLoginFirstAdmin' -count=1 -race`：通过（user 4.98s、oidc 1.70s）。
- `go build ./...`：通过。
- `go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线仍 86 项/0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 新首建不再写入标记；历史键不参与判断；密码/OIDC 首建角色与并发唯一 admin 均保持；无 schema/API/Setup 变更。全部通过。
- **自动化证据边界：** 单元测试和 `-race` 证明角色与事务语义；不替代真实 OIDC 浏览器流程，该流程仍属 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；本 Step 无 HTTP 合同变化，后续 Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 历史库中已有 `admin_initialized` 行仍保留但无读取方，符合设计；无其他遗留。
- **下一步：** 进入 Step 3：R28-07B 自定义订阅与隐藏组 Token 协调。

### Step 3：R28-07B 自定义订阅与隐藏组 Token 协调

- **Step 编号和标题：** Step 3：R28-07B 自定义订阅与隐藏组 Token 协调。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 22:45～22:49 CST。
- **前置条件检查：** Step 2 已 ✅ 验收通过；Step 1 门禁可用；无工作树冲突。
- **影响评估：** Token 解析从旧“先建组 Token、后查自定义覆盖”改为 `token.Service` 单事务业务键解析；首页卡片 JSON 字段、下载 URL、下载端点状态码、自定义优先级和显式 Token 语义保持。无 schema/API/导入导出变化。
- **工作树基线重叠情况：** 本 Step 新增/修改 token/home/custom 测试，与 Step 1/2 改动无冲突。

**失败优先测试：**
- 新增 `TestResolveUserTokenCustomPriorityCleansGroupToken` 与 `TestListPlatformsCustomPriorityCleansHiddenGroupToken`：模拟历史“自定义 Token + 隐藏组 Token”，对照旧清理行为时稳定失败（断言残留组 Token）。
- 新增 `TestRefreshTokenCustomPriority`：刷新链接时旧行为只轮替自定义 Token，历史组 Token 残留；对照失败。
- 新增 `TestResolveUserTokenGroupFallback`、`TestResolveUserTokenNoContentDoesNotCreate`、`TestResolveUserTokenConcurrent`、`TestRefreshUserTokenByBusinessKey`、`TestListPlatformsGroupAndUnassigned` 作为正向/边界/并发回归。

**实施方案：**
- `token.go`：抽出 `getOrCreateUserTokenTx`；新增 `resolveApplicableTokenKeyTx`（自定义优先，否则按平台 current_version 判断组可用）、`cleanupNonApplicableUserTokensTx`（保留显式 subscription Token）、`ResolveUserToken`（单事务解析并清理历史双 Token）、`RefreshUserTokenByBusinessKey`（业务键原子轮替，自定义优先/组回退）。
- `home.go`：普通用户平台卡片改为调用 `ResolveUserToken` 后再判定 `custom/ready/unassigned`；`RefreshToken` 改为调用 `RefreshUserTokenByBusinessKey`；删除接入层先查自定义、再后补组 Token 的顺序。
- `custom.go` 既有上传/覆盖后删无标识组 Token 行为保留；不再缓存/生成隐藏组 Token。
- 新增 `token_test.go` 业务键矩阵与 `home_test.go` 卡片/刷新集成回归。

**修改文件：** `backend/internal/token/token.go`、`backend/internal/home/home.go`、`backend/internal/token/token_test.go`、`backend/internal/home/home_test.go`。

**自主决策及依据：**
- 将适用 Token 判定放入 `token.Service` 单事务（而非 home 层先读后写），依据 AGENTS §4.6 业务键原子操作和 Build26“避免接入层直接 SQL 和先读后写竞态”。
- `RefreshUserTokenByBusinessKey` 在适用业务内容存在但 Token 缺失时创建新 Token；只有业务内容不存在才返回 `ErrTokenNotFound`。理由：刷新链接语义是获得可用新链接，且能修复历史双 Token 下只剩不可用组 Token 的边界；不改变有 Token 的正常轮替路径。
- 保留 `GetOrCreateUserToken` 兼容入口与显式订阅 Token，避免影响订阅删除/降级/管理员预览既有生命周期。

**验收命令与真实结果：**
- 修复前对照：将自定义分支清理临时关闭后，`go test ./internal/token ./internal/home -run 'CustomPriority|RefreshToken' -count=1` 失败，分别报历史隐藏组 Token 仍存在。
- 修复后：`go test ./internal/token ./internal/home ./internal/custom ./internal/server -run 'ResolveUserToken|UserToken|HomePlatforms|RefreshToken|UpsertDeletesGroupToken|HiddenGroup' -count=1 -race`：通过（token/home/custom/server 全 ok）。
- `go build ./...`：通过。
- `go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 86 项、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 同用户/平台不再残留“自定义 Token + 隐藏组 Token”；历史双 Token 在首页/刷新业务路径自动清理；自定义优先、无自定义组回退、无激活版本不创建 Token、刷新链接原子轮替、并发只产生一个适用 Token、显式 Token 保留均通过测试；卡片 JSON/下载地址/状态码未改。全部满足。
- **自动化证据边界：** 单元测试覆盖业务键矩阵与并发；真实浏览器卡片显示和下载链接点击仍属 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；本 Step 未改变 HTTP contract，Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 未来若引入每用户/平台多自定义订阅，需要新的业务键设计；当前符合每用户每平台一份的既有合同。
- **下一步：** 进入 Step 4：R28-07D1 用户动态下载渲染服务化。

### Step 4：R28-07D1 用户动态下载渲染服务化

- **Step 编号和标题：** Step 4：R28-07D1 用户动态下载渲染服务化。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 22:49～22:54 CST。
- **前置条件检查：** Step 3 已 ✅ 验收通过；架构门禁已有 `render.go` 允许项，可先删项验证红。
- **影响评估：** 将原 `internal/server/render.go` 的整段用户下载业务（蓝图 SQL、manual 名称、凭据/目标读取、Clash 全量重渲染、SR/generic 占位与 base64）迁入独立业务包 `internal/userrender`；`download.Service` 通过注入的 `Render` 方法调用。HTTP 状态码、响应头、文件名、下载内容不变；`internal/server` 不再包含该业务与直接 SQL。
- **工作树基线重叠情况：** 删除 `server/render.go` 及其三个原测试文件；新增 userrender 包和迁移测试。无其他任务修改重叠。

**失败优先测试：**
- 先删除架构允许清单中的 `render.go` 条目后运行 `go test ./internal/server -run 'TestArchitectureNoDirectStore'`，在旧文件存在时稳定失败，列出 `render.go:22`、`render.go:141` 两处直访 `st.DB()`；证明服务化迁移必要且门禁可识别。
- 迁移后的下载产物测试保留原断言：R28-06 sentinel 不泄漏、旧字符串数组 Clash plan 回退、1 万规则渲染基准。

**实施方案：**
- 新增 `backend/internal/userrender/userrender.go`：构造注入 `store/config/xray.SyncService/xray.CredentialService/logger`，提供 `Render(ctx, subID,userID,content,fileName)`；完整承载原渲染算法、蓝图查询、manual 名称、凭据注入、Clash 重渲染、SR/generic base64、旧 plan 回退和自检告警。
- `server/server.go` 改为 `userrender.NewService(...).Render` 注入 `download.Service`；删除 `server/render.go`。
- 迁移 `r28_06_download_test.go`、`step7_legacy_plan_test.go`、`render_bench_test.go` 到 `internal/userrender`，并新增 `userrender_test.go` 提供完整迁移测试环境。
- `architecture_test.go` 删除 `render.go` 允许项；保留 `oidc.go`/`server.go`/`traffic.go` 待后续 Step。
- `userrender` 不导入 Gin/HTTP Handler；使用注入 `slog.Logger` 取代原 `slog` 全局调用。

**修改文件：** `backend/internal/userrender/userrender.go`、`userrender_test.go`、`r28_06_download_test.go`、`step7_legacy_plan_test.go`、`render_bench_test.go`；`backend/internal/server/server.go`、`backend/internal/server/architecture_test.go`；删除 `backend/internal/server/render.go`、`r28_06_download_test.go`、`step7_legacy_plan_test.go`、`render_bench_test.go`。

**自主决策及依据：**
- 服务方法名使用 `Render`，保持与 `download.SetRenderUser` 现函数签名兼容；未引入新接口层，符合最小迁移。
- `userrender` 内部复制 `firstOutputError` 小助手而不是导出 `server` 未导出函数，避免扩大 `assembly` API。
- 迁移测试使用完整 `migrations.FS`，确保渲染 SQL 与真实 schema 一致；删除原 server 测试文件避免重复/失效引用。

**验收命令与真实结果：**
- 红：临时保留旧 `render.go` 且允许清单无 render 条目时，`go test ./internal/server -run 'TestArchitectureNoDirectStore' -count=1` 失败，列出 render.go 两处直访。
- `go test ./internal/userrender ./internal/download -run 'Render|R28_06|LegacyPlan|Benchmark' -count=1 -race`：通过（userrender 2.29s；download 无匹配测试但编译通过）。
- `go test ./internal/userrender -count=1`：通过。
- `go test ./internal/server -run 'Download|ArchitectureNoDirectStore' -count=1`：通过。
- `go test ./internal/server -count=1`：通过（1.71s）。
- `go build ./...`：通过。
- `go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，86 项基线、0 unexpected。
- `git diff --check`：通过（本轮记录追加后再次检查）。

**验收标准逐项结论：** `internal/server` 不再含渲染 SQL/业务；下载产物、旧 plan 回退、R28-06 sentinel、1 万规则性能均通过迁移测试；架构允许项已删除一项；`userrender` 不依赖 Gin/HTTP。全部满足。
- **自动化证据边界：** 迁移测试使用真实迁移 schema 和注入服务；不替代真实浏览器下载与真实客户端导入，仍归 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** `download` 仍通过函数注入而非接口抽象；当前无第二渲染实现，保持最小改动。
- **下一步：** 进入 Step 5：R28-07D2 流量汇总 Xray 业务化。

### Step 5：R28-07D2 流量汇总 Xray 业务化

- **Step 编号和标题：** Step 5：R28-07D2 流量汇总 Xray 业务化。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 22:54～22:57 CST。
- **前置条件检查：** Step 4 已 ✅ 验收通过；架构允许清单仍含 `traffic.go` 待拆。
- **影响评估：** 将原 `server/traffic.go` 的月用量 SUM、有效配额换算、`quota_exceeded` 读取迁入 `internal/xray`，返回 `TrafficSummary` 业务结构体；`HomeHandler`/`ProfileHandler` 不再持有 traffic 相关 `st/cfg/syncSvc`，仅依赖窄接口。`/api/home/summary` 的 `traffic` 子对象与 `/api/profile/traffic` 的 JSON 字段名/类型/quota_bytes null 语义保持不变。无 schema/API 合同变化。
- **工作树基线重叠情况：** 删除 `server/traffic.go`；新增 xray 业务测试与 server 接口级流量形状测试；与 Step 4 改动的 server.go/home.go 有重叠但均为本任务连续修改，无用户修改冲突。

**失败优先测试：**
- 新增 `TestTrafficSummaryBasicModeUnlimited`、`TestTrafficSummaryAdvancedUnlimited`、`TestTrafficSummaryAdvancedQuotaMatrix` 与 `TestHomeSummaryAndProfileTrafficShape`。
- 对照旧 handler 无高级分支的行为，临时将业务方法改为恒基础模式后运行：xray 测试失败于“配额内汇总异常”；server 接口测试失败于“高级模式流量形状异常”。证明测试真实覆盖旧实现缺口。
- 通过修复后，基础模式 unlimited、高级无配额 `quota_bytes=null`、配额内字节换算、超限标记、首页/个人中心接口形状全部转绿。

**实施方案：**
- `internal/xray/quota.go`：新增 `TrafficSummary` 业务结构体与 `SyncService.TrafficSummaryForUser`；基础模式直接返回 unlimited，高级模式查询当月 `traffic_records`、复用 `EffectiveQuota`、读取 `quota_exceeded`。
- `server/home.go`：定义 `trafficSummaryProvider` 窄接口，`HomeHandler` 删除 `st/cfg/syncSvc`，summary 只调用接口并映射原响应。
- `server/profile.go`：同样删除 traffic 相关字段，`traffic` 端点调用业务结构体。
- `server/server.go`：装配时注入 `syncSvc` 到窄接口字段。
- 删除 `backend/internal/server/traffic.go`；新增 xray 与 server 流量回归测试。

**修改文件：** `backend/internal/xray/quota.go`、`backend/internal/xray/traffic_summary_test.go`、`backend/internal/server/home.go`、`backend/internal/server/profile.go`、`backend/internal/server/server.go`、`backend/internal/server/traffic_summary_test.go`、`backend/internal/server/download_test.go`（测试 schema 补 `traffic_records` / 用户配额字段）；删除 `backend/internal/server/traffic.go`。

**自主决策及依据：**
- Handler 使用本地 `trafficSummaryProvider` 接口而非直接持有 `*xray.SyncService`，依据 Build26“删除 traffic 相关 st/cfg/syncSvc 依赖”，同时保持构造注入与接入层窄依赖。
- 保留 `TrafficSummary` 的 JSON 标签为已有 snake_case 字段，避免前端合同变化。
- server 测试夹具只补测试所需列/表，不改生产 schema。

**验收命令与真实结果：**
- 红：临时恒返回基础模式后，`go test ./internal/xray -run 'TrafficSummary'` 失败（高级配额矩阵）；`go test ./internal/server -run 'TestHomeSummaryAndProfileTrafficShape'` 失败（高级流量形状）。
- `go test ./internal/xray ./internal/server -run 'Traffic|Summary|Profile' -count=1 -race`：通过（xray 2.71s、server 8.10s）。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，86 项基线、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** Handler 不再直接查 traffic 表；基础/高级接口 JSON 形状与 null 语义保持；配额字节、超限标记、未超限矩阵均通过；架构允许项少一项（traffic.go）。全部满足。
- **自动化证据边界：** 业务单元测试与 httptest 接口回归证明合同；真实浏览器首页/个人中心显示仍归 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** `TrafficSummaryForUser` 依赖 `SyncService` 现有有效性配额查询；若未来扩展独立账号流量展示，需另立业务接口，不在本 Step 扩围。
- **下一步：** 进入 Step 6：R28-07D3 OIDC ticket 服务化。

### Step 6：R28-07D3 OIDC ticket 服务化

- **Step 编号和标题：** Step 6：R28-07D3 OIDC ticket 服务化。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 22:57～23:00 CST。
- **前置条件检查：** Step 5 已 ✅ 验收通过；架构允许清单仍含 `oidc.go`；Step 12 后续依赖本 Step ticket 服务错误返回。
- **影响评估：** OIDC 登录 ticket 的创建、一次性消费、过期清理从 `server/oidc.go` 的 Handler 事务迁入 `internal/oidc` 服务；Handler 删除 `store` 字段，只读取 Cookie、调用服务、映射响应。OIDC state/PKCE/回调/绑定逻辑不变；`oidc_login_tickets` schema 不变；正常换票成功/无效/过期 401 行为不变；数据库清理失败从旧“静默忽略/误报 401”变为显式 500。
- **工作树基线重叠情况：** `server/oidc.go`、`server.go` 与 Step 4/5 的 server 装配连续修改；无用户修改冲突。

**失败优先测试：**
- 新增 `TestLoginTicketIssueConsumeOneTime`；对照旧 handler 未删除已消费 ticket 的行为时稳定失败：消费后记录仍为 1。
- 新增 `TestLoginTicketExpiredCleanup`、`TestLoginTicketDeleteErrors`：覆盖过期记录清理、有效消费删除失败、过期清理删除失败、Issue 过期清理失败，均要求返回真实错误而非静默成功。
- 新增 server `TestOidcExchangeTicketHTTP`：HTTP 成功 200、重复换票 401、过期 401、删除失败 500。
- 架构门禁红证据继承 Step 1：旧 `server/oidc.go` 两处 `h.store.TxImmediate` 被门禁识别；本 Step 删除该允许项。

**实施方案：**
- 新增 `backend/internal/oidc/ticket.go`：`ErrLoginTicketInvalid`、`IssueLoginTicket`（生成 256 位 ticket、事务内清过期、插入 60 秒 TTL）、`ConsumeLoginTicket`（查询、过期清理、一次性删除；过期清理提交后返回无效；删除失败返回真实错误）。
- `server/oidc.go`：删除 `store` 字段和 `issueLoginTicket` 方法；callback 调用 `oidcSvc.IssueLoginTicket`；exchange 调用 `oidcSvc.ConsumeLoginTicket`，`ErrLoginTicketInvalid` → 401，其他错误 → 500；保留 Cookie 清除与响应形状。
- `server/server.go`：构造 `OidcHandler` 不再传 `store`。
- `architecture_test.go`：删除 `oidc.go` 允许项，仅剩 Step 7 的 `server.go`。
- `errgate_baseline.json`：删除旧 `server/oidc.go` 中已修复的 `tx.ExecContext` 忽略项（86→85），证明允许项随修复同步删除。

**修改文件：** `backend/internal/oidc/ticket.go`、`backend/internal/oidc/ticket_test.go`、`backend/internal/oidc/oidc_test.go`、`backend/internal/server/oidc.go`、`backend/internal/server/oidc_exchange_test.go`、`backend/internal/server/server.go`、`backend/internal/server/architecture_test.go`、`backend/internal/server/download_test.go`、`backend/errgate_baseline.json`。

**自主决策及依据：**
- 过期 ticket 的删除在同一事务内完成后以普通 `nil` 返回、由事务外转换为 `ErrLoginTicketInvalid`，避免 `TxImmediate` 因返回 error 回滚清理结果；有效删除失败仍返回真实错误并回滚，保持安全语义。
- exchange 对非无效类数据库错误返回 500，而不是旧实现的 401；这是把“清理失败”从误报中显式区分的必要修正，不改变正常无效/过期 ticket 的 401 合同。
- ticket TTL 保持 60 秒，Cookie 路径/HttpOnly/SameSite/Secure 逻辑不变。

**验收命令与真实结果：**
- 红：临时让消费不删除记录后，`go test ./internal/oidc -run 'TestLoginTicketIssueConsumeOneTime'` 失败：“消费后 ticket 应删除，实际 1”。
- `go test ./internal/oidc ./internal/server -run 'LoginTicket|Ticket|Exchange|ArchitectureNoDirectStore' -count=1 -race`：通过（oidc 1.44s、server 1.51s）。
- `go test ./internal/oidc -run 'LoginTicket|Ticket' -count=1`：通过。
- `go test ./internal/server -run 'TestOidcExchangeTicketHTTP' -count=1`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 85 项、0 unexpected。
- `go test ./internal/server -run 'ArchitectureNoDirectStore' -count=1`：通过。
- `go build ./...`：通过；`go vet ./...`：通过；`git diff --check`：通过。

**验收标准逐项结论：** `server/oidc.go` 无 `TxImmediate`；ticket 生命周期单一服务负责；一次性消费、过期清理、删除失败可观测、HTTP 成功/401/500 映射、Cookie 行为均通过；架构允许项删除一项；无 schema/OIDC state/绑定合同变化。全部满足。
- **自动化证据边界：** 业务测试与 httptest 覆盖 ticket 生命周期和接入层映射；真实 OIDC 提供商浏览器回调仍属 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** ticket 表无 schema 变更，旧数据兼容保持；若未来需要多实例共享 ticket，需另立设计，不在本轮范围。
- **下一步：** 进入 Step 7：R28-07D4 架构门禁清零。

### Step 7：R28-07D4 架构门禁清零

- **Step 编号和标题：** Step 7：R28-07D4 架构门禁清零。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:00～23:02 CST。
- **前置条件检查：** Step 6 已 ✅ 验收通过；架构允许清单仅剩 `server.go` 的 access log 装配 `st.DB()`。
- **影响评估：** 仅调整 access log 服务装配方式和构造入口；查询/清空 SQL、HTTP 合同、日志表 schema、90 天清理任务均不变。`internal/server` 非测试生产文件将不再出现 `DB()`/`TxImmediate()`。
- **工作树基线重叠情况：** `server.go`、`architecture_test.go`、`internal/log/access.go`、`internal/log/log_test.go` 与前述 Step 连续修改；无用户修改冲突。

**失败优先测试：**
- 先将 `architectureAllowlist` 清空，运行 `go test ./internal/server -run 'TestArchitectureNoDirectStore' -count=1`，稳定失败并报 `server.go:439 st.DB(...)`；证明清零后门禁确实抓取旧装配。
- 新增 `TestAccessServiceFromProvider`，验证通过存储适配接口注入后的查询/清空行为与直接 `*sql.DB` 构造一致。

**实施方案：**
- `internal/log/access.go`：新增 `DBProvider` 接口与 `NewAccessServiceFromProvider(provider, lg)`；原 `NewAccessService(*sql.DB, lg)` 保留给已有测试/低层调用。
- `server/server.go`：access log 装配改为 `log.NewAccessServiceFromProvider(st, lg)`，server 生产代码不再直接调用 `st.DB()`。
- `architecture_test.go`：允许清单清空为空 map；门禁保留“发现未允许违规”和“允许项过期”双重失败能力。

**修改文件：** `backend/internal/log/access.go`、`backend/internal/log/log_test.go`、`backend/internal/server/server.go`、`backend/internal/server/architecture_test.go`。

**自主决策及依据：** 使用 log 包内 `DBProvider` 窄接口而非让 log 依赖 store，避免既有 `store→log` 循环；`server` 只把 `*store.Store` 作为满足接口的业务存储传入，符合“接入层不直接操作存储”并保持既有 SQL 在日志服务内封装。

**验收命令与真实结果：**
- 红：清空允许清单后 `go test ./internal/server -run 'TestArchitectureNoDirectStore'` 失败，列出 `server.go:439 st.DB(...)`。
- `go test ./internal/server -run 'ArchitectureNoDirectStore' -count=1`：通过（空允许清单）。
- `go test ./internal/log -run 'Access' -count=1`：通过（含 `TestAccessQuery`、`TestAccessServiceFromProvider`）。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 85 项、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** `internal/server` 非测试生产文件零 `DB()`/`TxImmediate()`；架构允许清单清零且过期条目有自检；管理端日志查询/清空装配与行为不回归。全部满足。
- **自动化证据边界：** 真实浏览器日志页查询/清空仍由 ProdTestList 跟踪；Step 20 统一接口回归。
- **Production smoke 边界：** 未执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** `AccessService` 仍持有 `*sql.DB` 并直接执行日志 SQL，属于日志服务内部数据访问，不属于接入层越层；后续如需要 Store 事务协调再另立设计。
- **下一步：** 进入 Step 8：R28-07E1 debug 请求上下文化。

### Step 8：R28-07E1 debug 请求上下文化

- **Step 编号和标题：** Step 8：R28-07E1 debug 请求上下文化。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:02～23:04 CST。
- **前置条件检查：** Step 7 已 ✅ 验收通过；`response.debugProvider` 仍为包级可变回调；无工作树冲突。
- **影响评估：** 删除 `response` 包级 `debugProvider`/`SetDebugProvider`，改为请求上下文标志；`server` 增加 `debugContextMiddleware`，每个请求按所属 Server 的 `debug_mode` 写入 context；`New` 与 `NewEmergency` 都注册该中间件。5xx 详情/脱敏行为、错误码、日志语义不变。
- **工作树基线重叠情况：** `response.go`、`server.go` 与 Step 4～7 连续修改；新增 response/server 定向测试。

**失败优先测试：**
- 新增 `TestDebugFlagFromRequestContext`：调试 context 返回详情、默认 context 脱敏。
- 新增 `TestDebugContextMiddlewareServerIsolation`：两个 Server 实例 `debug_mode` 不同，调试实例返回详情、普通实例脱敏，互不污染。
- 对照旧全局回调无法按请求隔离的行为，临时让 `DebugEnabled` 恒 false 后运行：response 与 server 测试均稳定失败（调试请求被错误脱敏），再恢复转绿。

**实施方案：**
- `internal/response/response.go`：新增类型化 `debugContextKey`、`WithDebug(ctx,bool)`、`DebugEnabled(ctx)`；`Fail` 只读取当前请求 context；删除包级回调与 setter。
- `server/server.go`：新增 `debugContextMiddleware(cfg)`，在 `New` 和 `NewEmergency` 的中间件链中注册；删除装配时的 `response.SetDebugProvider`。
- 新增 `response_test.go`、`debug_context_test.go`。

**修改文件：** `backend/internal/response/response.go`、`backend/internal/response/response_test.go`、`backend/internal/server/server.go`、`backend/internal/server/debug_context_test.go`。

**自主决策及依据：** 使用私有类型 context key，避免与其他包 context 值冲突；middleware 放在 body limit 之后、业务路由之前，保证所有请求（含 emergency）都能按各自 cfg 注入；未改变 `debug_mode` 的 DB 读取失败 fail-safe 口径（`GetBool` 默认 false）。

**验收命令与真实结果：**
- 红：临时 `DebugEnabled` 恒 false 后，`go test ./internal/response -run 'Debug'` 失败；“调试请求应返回内部详情”。
- 红：同 mutation 后，`go test ./internal/server -run 'Debug|Isolation'` 失败；“调试实例应返回详情”。
- 绿：`go test ./internal/response ./internal/server -run 'Debug|Fail|Isolation|ArchitectureNoDirectStore' -count=1 -race`：通过。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，85 项基线、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 无包级 debug 回调；每个请求按自身 Server/cfg 决定 5xx 详情；缺失 context 默认脱敏；多 Server 隔离；400/401/403 不受影响（`Fail` 只在 >=500 分支读 debug）；`NewEmergency` 也已接入。全部满足。
- **自动化证据边界：** 使用测试 Server 与 httptest 验证行为；真实浏览器调试模式检查仍属人工项。
- **Production smoke 边界：** 未执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** `debug_mode` 的配置读取仍 fail-safe 默认 false，与用户决策 9 一致；Step 9 将处理 Logger/全局状态。
- **下一步：** 进入 Step 9：R28-07E2 Logger/级别控制器实例注入。

### Step 9：R28-07E2 Logger/级别控制器实例注入

- **Step 编号和标题：** Step 9：R28-07E2 Logger/级别控制器实例注入。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:04～23:09 CST。
- **前置条件检查：** Step 8 已 ✅ 验收通过；`response.debugProvider` 已移除；`log` 包仍有包级 defaultLogger/levelVar、全局 Info/Error/SetLevel，Store 迁移日志与 server 请求/panic 日志仍绕过实例注入。
- **影响评估：** 日志格式、环境变量语义、日志级别持久化键、5xx 脱敏和日志文本保持不变；构造 API 扩展为 `log.Runtime`、`store.Open(..., WithLogger)`、`config.NewAdminService(..., level)`；多实例级别隔离，不再有可变包级 Logger/LevelVar。
- **工作树基线重叠情况：** `log.go`、`store.go`、`config/admin.go`、`cmd/server/main.go`、`server.go`、slug 调用点及多个测试与已验收 Step 连续修改；无用户修改冲突。

**失败优先测试：**
- `TestNewFormats`/`TestLogLevelSwitch`：临时让 `NewRuntime` 复用包级 LevelVar 模拟旧行为后稳定失败，分别报“New 兼容入口共享可变 LevelVar”“切换日志级别污染其他 Runtime”。
- `TestOpenInjectsMigrationLogger`：临时让 `WithLogger` 不注入后稳定失败，报“迁移日志未进入注入 Logger”。
- 新增 `TestFailUsesContextLogger`、`TestRequestAndPanicLoggerUseInstance`，锁定 5xx/请求/panic 使用实例 Logger；`TestAccessServiceFromProvider` 保持 Step 7 装配回归。

**实施方案：**
- `internal/log/log.go`：删除包级 `defaultLogger`/`levelVar`、`SetDefault/SetLevel/Info/Error/Default`；新增 `Runtime{Logger,Level}` + `NewRuntime`，`New` 保留为每次独立 LevelVar 的兼容入口；新增 `WithLogger/FromContext` 请求上下文注入，缺失时使用丢弃 logger 而非可变全局。
- `cmd/server/main.go`：`log.NewRuntime` 创建运行实例；Store 通过 `store.WithLogger(runtime.Logger)` 注入；`server.New(..., runtime, ...)`。
- `internal/store/store.go`：`Store.logger` + `Open(..., opts ...Option)`/`WithLogger`；迁移日志写 `s.logger.Info`。
- `config/admin.go`：`AdminService` 持有 `*slog.LevelVar`，`SetLogLevel` 只切换当前运行实例。
- `server/server.go`：`New` 接收 `log.Runtime`；请求中间件先注入 `loggerContextMiddleware(lg)` 再注入 debug 标志；`requestLogger`/`panicRecovery` 接收实例 Logger；`NewEmergency` 同步接入。
- `slug.Generate`/`subscription.GenerateSlugTx` 增加实例 Logger 参数并更新调用点，移除标准库 `slog.Error` 直接全局调用。

**修改文件：** `backend/internal/log/log.go`、`backend/internal/log/log_test.go`、`backend/internal/store/store.go`、`backend/internal/store/store_test.go`、`backend/internal/config/admin.go`、`backend/internal/config/admin_test.go`、`backend/cmd/server/main.go`、`backend/internal/server/server.go`、`backend/internal/server/*_test.go`、`backend/internal/response/response.go`、`backend/internal/response/response_test.go`、`backend/internal/slug/slug.go`、`backend/internal/slug/slug_test.go` 及 slug 调用方（group/platform/setup/subscription/custom/share/xray/rule）。

**自主决策及依据：**
- `server.New` 参数由 `*slog.Logger` 改为 `log.Runtime`，以同一构造点携带 Logger+LevelVar；所有调用方（main 与测试）同步使用 `log.NewRuntime`。未改变实际日志格式或级别语义。
- `store.Open` 使用函数式 Option，保持现有无参调用兼容；未注入时创建独立 stdout logger，不读写包级状态。
- `slug.Generate` 增加显式 Logger 参数，属于内部辅助 API 的机械扩展；所有调用点传入所属服务实例 logger，避免标准库全局 logger。
- `log.FromContext` 缺失时返回丢弃 logger，不回退到任何包级可变 logger；业务路由均由中间件注入，panic/请求日志使用显式传入实例。

**验收命令与真实结果：**
- 红：共享 LevelVar mutation 后 `go test ./internal/log -run 'TestNewFormats'` 失败；`go test ./internal/config -run 'TestLogLevelSwitch'` 失败。
- 红：WithLogger no-op mutation 后 `go test ./internal/store -run 'TestOpenInjectsMigrationLogger'` 失败。
- 绿：`go test ./internal/log ./internal/store ./internal/config ./internal/response ./internal/server -count=1 -race`：通过（log 1.31s、store 2.81s、config 4.89s、response 1.79s、server 18.87s）。
- 绿：`go test ./... -count=1 -timeout 300s`：全部 39 包 ok。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，85 项基线、0 unexpected。
- `go test ./internal/server -run 'ArchitectureNoDirectStore'`：通过；`git diff --check`：通过。

**验收标准逐项结论：** 无可变包级 Logger/LevelVar；多 Runtime 级别隔离；Store 迁移日志进入注入 Logger；请求/panic/5xx 使用实例 Logger；标准库 slog 直接调用点清零；日志格式、环境变量、持久化键与文本保持。全部满足。
- **自动化证据边界：** 真实日志采集、容器 stdout、SSE 实时流仍由 Step 16～18 与 Production smoke/人工验证覆盖。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** slug/subscription 内部 API 增加参数已由全量测试覆盖；后续新增调用方必须显式传 logger。
- **下一步：** 进入 Step 10：R28-07E3 敏感键固定集合。

### Step 10：R28-07E3 敏感键固定集合

- **Step 编号和标题：** Step 10：R28-07E3 敏感键固定集合。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:09～23:11 CST。
- **前置条件检查：** Step 9 已 ✅ 验收通过；`config.RegisterSensitive` 与运行期 `sensitiveKeys` map 仍存在，`mail.init` 运行期注册 smtp_password。
- **影响评估：** smtp_password 的 AES-256-GCM 加密/解密行为不变；未知键（至少验证码 Site Key/Secret Key）继续明文存储与原值回显；R28-07F 不变；不再提供运行期敏感键注册入口。无 schema/导入导出/API 变化。
- **工作树基线重叠情况：** `config.go`、`mail.go` 与 Step 2/9 的 config/log 改动连续；测试同步更新。

**失败优先测试：**
- 扩展 `TestSensitiveSetGet`：固定 smtp_password 应密文落库并解密，未知 captcha_secret_key 应明文落库。
- `TestSensitiveMasked` 保持 SMTP 面板敏感字段回归，并删除测试内 RegisterSensitive 调用。
- 对照旧“运行期注册”缺失/未登记行为，临时令 `isSensitiveKey` 恒 false 后运行，`TestSensitiveSetGet` 与 `TestSensitiveMasked` 均稳定失败于“敏感值应以密文落库”。

**实施方案：**
- `internal/config/config.go`：删除 `sensitiveKeys` map 与 `RegisterSensitive`，改为编译期 `isSensitiveKey(key) bool { return key == "smtp_password" }`；Get/Set/GetTx/SetTx 全部改走固定判定。
- `internal/mail/mail.go`：删除 `init()` 与运行期注册，保留敏感键常量与注释口径。
- `internal/config/config_test.go`、`admin_test.go`：删除 RegisterSensitive/delete(sensitiveKeys) 操作，改用固定键矩阵。

**修改文件：** `backend/internal/config/config.go`、`backend/internal/mail/mail.go`、`backend/internal/config/config_test.go`、`backend/internal/config/admin_test.go`。

**自主决策及依据：** 使用无状态比较函数而不是固定 map，彻底消除运行期可变注册表；验证码双密钥按用户已确认的 R28-07F 保持明文，不纳入集合，不改管理页回显。

**验收命令与真实结果：**
- 红：`isSensitiveKey` 恒 false 后，`go test ./internal/config -run 'TestSensitiveSetGet|TestSensitiveMasked'` 失败，均报敏感值未密文落库。
- `go test ./internal/config ./internal/mail -run 'Sensitive|Captcha|SMTP|Plaintext' -count=1 -race`：通过（config 1.35s、mail 1.57s）。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，85 项基线、0 unexpected。
- `go test ./... -run '^$'`：全部测试包编译通过；`git diff --check`：通过。

**验收标准逐项结论：** 无运行期敏感键注册入口；`smtp_password` 加解密行为不变；验证码 Site/Secret 明文与原值回显不变；导入导出格式不变；R28-07F 未实施。全部满足。
- **自动化证据边界：** 单元测试覆盖加密落库/解密/未知键明文；面板真实回显与导入导出体验仍属 ProdTestList/R28-07F 边界。
- **Production smoke 边界：** 未执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 未来新增敏感配置键需修改 `isSensitiveKey` 编译期判定并补测试，不能运行期注册。
- **下一步：** 进入 Step 11：R28-07C1 素材池同步错误语义与终态清理。

### Step 11：R28-07C1 素材池同步错误语义与终态清理

- **Step 编号和标题：** Step 11：R28-07C1 素材池同步错误语义与终态清理。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:11～23:15 CST。
- **前置条件检查：** Step 10 已 ✅ 验收通过；`pool/sync.go` 仍有 `json.Marshal` 忽略、failed 快照旧 active 查询忽略、终态清理 `_, _ =`，`pool/pool.go` 有 `json.Unmarshal`/`json.Marshal` 忽略与无效 `_ = semanticKey`。
- **影响评估：** 成功路径、快照 schema、任务 API、池同步算法、URL 拉取和状态机不变；错误路径改为返回错误/回滚或结构化日志，失败快照不再虚报成功，终态清理失败可见。
- **工作树基线重叠情况：** `pool/pool.go`、`pool/sync.go` 与 Step 4/5 的 xray/server 改动无文件冲突；新增定向测试。

**失败优先测试：**
- 新增 `TestApplyParseResultMarshalFailureRollsBack`：注入失败 marshal 后旧实现会写入半成品快照；临时禁用错误处理后稳定失败于“序列化失败应原样返回”。
- 新增 `TestRecordFailedSnapshotMarshalFailure`：failed 快照序列化失败必须返回错误且不写假成功快照。
- 新增 `TestFailedSnapshotOldActiveQueryFailure`：旧 active 查询失败（rolled-back tx）必须显式返回；临时忽略查询错误后稳定失败。
- 新增 `TestFinishTaskSerializationErrorIsLogged`、`TestFinishTaskCleanupErrorIsLogged`：分别注入 marshal 失败和 DELETE 触发器失败，断言 `pool_id`/`task_id` 日志；忽略清理错误时稳定失败于日志缺失。

**实施方案：**
- `Service` 增加实例级 `marshal func(any)([]byte,error)` 与 `marshalJSON` helper（默认 `json.Marshal`），测试可注入失败；不引入包级可变 seam。
- `applyParseResultTxWithMarshal`：诊断/统计序列化失败返回 `fmt.Errorf`，由事务回滚；生产路径传入 `s.marshalJSON`；移除 `_ = id`。
- `recordFailedSnapshotTxWithReasonAndMarshal`：旧 active 查询错误与 ErrNoRows 均显式返回定位错误；诊断/统计序列化失败返回错误；原签名保留兼容。
- `finishTask`/`failTask`：结果序列化失败写结构化日志（`pool_id`/`task_id`）并以 `[]` 安全终态继续；终态回写失败日志补 `pool_id`；终态清理 DELETE 失败写结构化日志，不影响已提交终态。
- `pool/pool.go`：`ensureCanonicalTx` 的 options JSON 序列化失败返回；`ListEntries` 解析 options 失败记 warn 并保持空选项显示；移除 `_ = semanticKey`。

**修改文件：** `backend/internal/pool/pool.go`、`backend/internal/pool/sync.go`、`backend/internal/pool/step11_error_test.go`、`backend/errgate_baseline.json`（从 85 项删减到 75 项，删除本 Step 已修复的 10 项）。

**自主决策及依据：** 使用 `Service.marshal` 实例字段作为最小测试 seam，符合用户允许“为实现已明确要求的失败注入增加最小测试 seam”，且没有恢复包级可变状态；failed 快照旧 active 查询 `ErrNoRows` 视为来源缺失错误，因为失败快照必须绑定到存在来源，避免写入不可归属数据。

**验收命令与真实结果：**
- 红：禁用序列化错误处理后 `TestApplyParseResultMarshalFailureRollsBack` 失败于“序列化失败应原样返回: <nil>”。
- 红：忽略终态清理错误后 `TestFinishTaskCleanupErrorIsLogged` 失败于“清理失败日志缺少定位上下文”。
- 红：忽略旧 active 查询错误后 `TestFailedSnapshotOldActiveQueryFailure` 失败于返回裸 `sql.TxDone` 而非定位错误。
- 绿：`go test ./internal/pool -run 'Sync|Snapshot|Terminal|Marshal|Cleanup' -count=1 -race`：通过（10.58s）。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 75 项、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 无非 EOF/真实语义错误被静默吞；终态写入/清理失败带 `pool_id`/`task_id` 定位；失败快照不虚报成功；快照统计不再静默失真；成功路径/状态机/schema 不变。全部满足。
- **自动化证据边界：** 失败注入使用 SQLite 触发器/rolled-back tx/实例 marshal，不替代真实网络中断、磁盘满等生产故障注入。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** `ListEntries` 对历史非法 options 改为 warn+空选项展示，保持可读性；若需严格失败需另立设计。
- **下一步：** 进入 Step 12：R28-07C2 补偿删除与 OIDC ticket 清理错误。

### Step 12：R28-07C2 补偿删除与 OIDC ticket 清理错误

- **Step 编号和标题：** Step 12：R28-07C2 补偿删除与 OIDC ticket 清理错误。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:15～23:19 CST。
- **前置条件检查：** Step 11 已 ✅ 验收通过；Step 6 OIDC ticket 服务已提供可观察删除错误；`server/assembly.go` 仍有两处 `_ = h.ruleSvc.Delete`。
- **影响评估：** 装配成功路径、生成/版本/规则合同、OIDC ticket schema 与正常换票行为不变；补偿删除失败新增结构化日志并可与主错误 `errors.Join`；ticket 过期/已消费删除失败继续返回真实错误，HTTP 非无效错误按 500 脱敏处理。
- **工作树基线重叠情况：** `server/assembly.go`、`server/server.go`、`internal/oidc/ticket.go` 与 Step 6 及 server 装配改动连续；新增补偿测试并扩展 assembly 测试环境 logger。

**失败优先测试：**
- 新增 `TestAssemblyAutoRuleCompensationFailureIsVisible`：自动建规则后版本写入失败，规则删除触发器再失败；旧实现静默 `_ =` 不写日志时稳定失败于“补偿删除失败日志缺少定位上下文”。
- 新增 `TestAssemblyAutoRuleCompensationSuccessRemovesRule`：补偿删除成功时不得残留孤儿规则。
- OIDC 复用 Step 6 `TestLoginTicketDeleteErrors`、`TestOidcExchangeTicketHTTP`：过期清理删除失败、有效消费删除失败、HTTP 清理失败 500 均可观察。

**实施方案：**
- `AssemblyHandler` 增加注入 `logger *slog.Logger`；新增 `rollbackAutoRule`：删除失败写 `回滚自动创建的分流规则失败`（`rule_id`/`err`）并返回带 `rule_id` 的包装错误。
- 两处自动建规则补偿改为调用 `rollbackAutoRule`；`resolveOwner` 错误时用 `errors.Join` 合并主错误与补偿错误；若主错误是 400 且补偿失败，返回“装配参数错误（自动创建规则回滚失败）”，不把删除 SQL 细节带到 400 响应。
- `versionSvc.CreateVersion` 失败补偿同样 Join；500 响应仍走统一脱敏。
- OIDC ticket 已在 Step 6 实现：过期清理/有效消费删除失败返回真实错误，Handler 对 `ErrLoginTicketInvalid` 返回 401，对清理/删除 DB 错误返回 500。
- `errgate_baseline.json` 删除两处已修复的 `h.ruleSvc.Delete` 忽略项（75→73）。

**修改文件：** `backend/internal/server/assembly.go`、`backend/internal/server/server.go`、`backend/internal/server/assembly_test.go`、`backend/internal/server/step12_compensation_test.go`、`backend/errgate_baseline.json`；OIDC 相关沿用 `internal/oidc/ticket.go` 与测试。

**自主决策及依据：** 未改变 `resolveOwner` 或版本创建合同；只在补偿路径增加 `errors.Join` 与日志。400 响应在补偿失败时使用固定用户可读文案，防止 DB/路径细节泄漏；正常校验错误仍保留详细消息。

**验收命令与真实结果：**
- 红：让 `rollbackAutoRule` 吞掉删除错误后，`go test ./internal/server -run 'TestAssemblyAutoRuleCompensationFailureIsVisible'` 失败于日志缺失。
- 绿：`go test ./internal/server ./internal/oidc -run 'Compensation|AutoRule|Ticket' -count=1 -race`：通过（server 2.47s、oidc 1.47s）。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 73 项、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 补偿/清理失败有结构化记录；不会静默当作成功；不会把 400 内部细节泄漏；ticket 过期/非过期删除错误可观察；成功路径、生成合同、cookie 合同和 OIDC 流程不变。全部满足。
- **自动化证据边界：** 使用 SQLite 触发器制造版本/规则删除失败；不替代真实数据库磁盘 I/O 故障。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 自动创建规则补偿失败时规则保留，需管理员手动清理，这是安全可观察语义而非静默孤儿。
- **下一步：** 进入 Step 13：R28-07C3 全量错误审计与 errgate 清零。

### Step 13：R28-07C3 全量错误审计与 errgate 清零

- **Step 编号和标题：** Step 13：R28-07C3 全量错误审计与 errgate 清零。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:19～23:32 CST。
- **前置条件检查：** Step 12 已 ✅ 验收通过；`errgate_baseline.json` 尚有 73 项已确认忽略点。
- **影响评估：** 对生产 Go 代码的 73 项剩余忽略进行有边界分类：correctness 错误修复并返回；失败清理/解析错误改为显式回退或警告；非关键配置读取改用 `GetOr` 记录结构化 warn 并保持原外部 fail-safe 行为。不改变成功路径、HTTP 合同、DB schema 或导入格式。
- **工作树基线重叠情况：** 涉及 config/approval/captcha/download/emergency/home/mail/oidc/assembly/node/rule/share/pool/xray/uriparse/server 等已在前述 Step 修改过的文件；无用户冲突。

**失败优先测试：**
- 保留 Step11/Step12 的池同步、终态清理、补偿删除失败注入。
- 新增运行期最小违规探针：在 `internal/errgateprobe` 写入 `_ = errors.New("probe")` 后运行 `go run ./cmd/errgate ./...` 稳定失败并输出该定位；删除探针后门禁通过。证明清零后门禁仍能抓新违规。
- 修复过程中 `errgate -list` 从 73 项逐步降到 0；删除基线允许项不是靠扩充 allowlist，而是逐项修复/分类。

**实施方案：**
- **配置 fail-safe 读取：** `config.Service.GetOr` 统一记录 `读取配置失败，按未设置降级` 结构化 warn 并返回空串；config/approval/captcha/download/emergency/home/mail/oidc/server/user 等 41 处原 `v, _ := cfg.Get(...)` 改为 `v := cfg.GetOr(...)`，保持对外行为。
- **RowsAffected/LastInsertId：** rule/share/user/emergency 结果读取错误改为显式返回；捕获不到影响行数不再被当作 nil。
- **JSON/序列化/解析：** assembly options JSON、pool snapshot diagnostics、pool canonical options 等改为错误返回或结构化 warn；node check ID 序列化异常降级为空对象但不再忽略。
- **URI 解析 fallback：** uriparse 的 10 处 `url.QueryUnescape` 统一改用已有 `urlQueryUnescapeOrRaw`；`Atoi`/`ParseQuery`/SS plugin JSON 解析错误显式分支返回空值/空集合，保持解析回退语义。
- **清理/响应助手：** server hardening 的 deadline 设置改为错误显式分支并注释语义；xray RetryUser 的 RemoveUserFromTargets 错误改为返回。
- `errgate_baseline.json` 清零为空数组 `{"version":1,"allowed":[]}`；门禁输出改为 `0 ignored errors matched baseline; baseline entries 0; 0 unexpected`。

**修改文件（按域）：** 配置与邮件 `config/{config,admin,_test}.go`、`mail.go`；认证/用户 `approval.go`、`captcha.go`、`user/{user,oidc,admin}`；装配/节点 `assembly/{clash_plan,load}.go`、`node/check.go`；素材池 `pool/{pool,sync,snapshot}.go`；下载/渲染 `download.go`、`userrender`；OIDC `oidc/{flow,mock,oidc}.go`；服务接入 `server/{pool,hardening,oidc,server,status}.go`；分享/规则/xray/uriparse 对应文件；`cmd/errgate/main.go`、`errgate_baseline.json`。

**自主决策及依据：**
- 非关键配置读取统一落到 `GetOr`，而不是机械地让所有读取返回错误；符合用户决策 9 的“保留 fail-safe 外部行为，增加 warn”边界。
- uriparse 的 10 行 `decodedName, _ := url.QueryUnescape(name)` 为完全重复的机械回退，使用一次精确 `perl -pi` 替换为已有 helper，随后 gofmt、全量测试和 errgate 验证；`str_replace_editor` 对同文件重复同文本无法唯一匹配，故记录此机械例外。
- 对畸形快照 diagnostics JSON 由“静默空诊断”改为显式错误返回，因为该字段参与展示与状态判断，继续静默会失真；成功数据格式不变。
- 未引入任何 `// errgate:allow` 豁免来规避修复；最终基线为空。

**验收命令与真实结果：**
- 探针红色：`go run ./cmd/errgate ./...` 非零，列出 `internal/errgateprobe/probe.go Bad [assign] errors.New("probe")`；删除后通过。
- `go run ./cmd/errgate ./...`：`OK (0 ignored errors matched baseline; baseline entries 0; 0 unexpected)`。
- `go test ./... -count=1 -timeout 300s`：全部 39 个包 ok。
- `go test ./internal/pool ./internal/server ./internal/share ./internal/rule ./internal/user ./internal/xray ./internal/config ./internal/assembly ./internal/uriparse ./internal/node -run 'Error|Ignore|Fail|Allow' -count=1 -race`：通过（pool 7.81s、server 2.06s、user 3.97s、xray 2.84s、config 2.71s、assembly 4.49s、node 2.78s；无匹配测试的包也编译通过）。
- `go build ./...`：通过；`go vet ./...`：通过；`git diff --check`：通过。

**验收标准逐项结论：** 所有剩余忽略均有真实修复、显式 fallback 分支或带 warn 的 fail-safe 处理；errgate 基线/允许项清零；新违规探针可触发失败；无无理由忽略；成功路径、HTTP/DB/导入合同不变。全部满足。
- **自动化证据边界：** 静态门禁证明“忽略”清零，不证明所有运行期故障；生产故障注入留待 Step 20 Production smoke/人工边界。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 少数 fallback（URI 非法编码、畸形历史 options/快照 JSON）改为显式空值/错误分支；如生产存量存在异常数据，需要在 Step 20 隔离 smoke 中关注。
- **下一步：** 进入 Step 14：R28-07G 后端 20 MiB/21 MiB 上限。

### Step 14：R28-07G 后端 20 MiB/21 MiB 上限

- **Step 编号和标题：** Step 14：R28-07G 后端 20 MiB/21 MiB 上限。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:32～23:37 CST。
- **前置条件检查：** Step 13 已 ✅ 验收通过；用户决策 3 的固定边界为文件 20×1024×1024、完整请求体 21×1024×1024；双导入入口和 R29-01 的 Setup `IMPORT` / 管理端 `IMPORT→DISABLE` 语义不变。
- **影响评估：** 导入端点从“完全豁免 body limit + 无界读入内存 + 非 EOF 当结束”改为固定请求体/文件上限、分块与伪造长度防护、真实读取错误 500 且不创建任务；导入格式、AES-GCM/Argon2id 流程、成功任务返回和 Setup/管理端确认词语义不变。
- **工作树基线重叠情况：** `settings_ops.go/hardening.go` 与 Step 7/16 后续 SSE 路由改动会重叠；本 Step 先完成导入边界，新增独立测试文件，无用户修改冲突。

**失败优先测试：**
- 新增 `TestImportFileAndRequestBodyLimits`：双入口恰好 20 MiB 允许、20 MiB+1 413、完整请求体 >21 MiB 413。
- 新增 `TestImportChunkedAndForgedContentLength`：`ContentLength=-1` + chunked、伪造小 Content-Length 仍按 21 MiB 上限拒绝。
- 新增 `TestImportReadErrorReturns500`：multipart 中途读取真实错误返回 500（debug 测试实例可看到通用提示），不得被当作 EOF 正常结束。
- 对照旧实现临时关闭大小检查和“非 EOF 即结束”处理后，边界与读取错误测试稳定失败；恢复后转绿。

**实施方案：**
- `hardening.go`：新增 `MaxImportFileBytes=20<<20`、`MaxImportRequestBodyBytes=21<<20`；body middleware 对 `/api/setup/import` 与 `/api/admin/settings/import` 不再豁免，而是设置 21 MiB 上限，`ContentLength` 超限直接 413，其余用 `http.MaxBytesReader` 包裹。
- 新增 `import_upload.go`：`readImportFile` 有界读取，超过 20 MiB 返回 `errImportFileTooLarge`；正常 EOF 结束，真实读取错误包装 `errImportReadFailed`；`isRequestBodyTooLarge` 识别 `*http.MaxBytesError`。
- `settings_ops.go`：`FormFile` 解析错误区分 413 / 500 / 缺失文件 400；读取循环改用有界 helper；超限 413、读取错误 500，且都发生在 `ImportV2` 之前，因此零任务创建；保留密码/确认词校验与既有错误映射。
- 新增 `import_limit_test.go`，双入口覆盖边界。

**修改文件：** `backend/internal/server/hardening.go`、`backend/internal/server/import_upload.go`、`backend/internal/server/settings_ops.go`、`backend/internal/server/import_limit_test.go`。

**自主决策及依据：**
- 文件字段 20 MiB 判定在 `readImportFile` 内按累计字节数执行，保证无 Content-Length、chunked、伪造长度都受同一上限约束；不改变 multipart 字段名或上传协议。
- 非 `http.ErrMissingFile` 的 multipart/读取错误统一返回 500 通用脱敏信息，真实原因只写实例日志；普通缺文件仍 400。
- 测试实例开启 `debug_mode` 仅用于在 httptest 中检查通用错误文本，生产行为仍由 `response.Fail` 5xx 脱敏控制。

**验收命令与真实结果：**
- 红：旧行为 mutation 后 `go test ./internal/server -run 'TestImportFileAndRequestBodyLimits|TestImportReadErrorReturns500'` 失败；恢复后通过。
- `go test ./internal/server ./internal/config -run 'Import.*(Limit|Body|Chunk|Truncate|Read|Setup|Admin|ZeroTask|Disable)' -count=1`：通过（server 0.67s、config 0.33s）。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 0 项、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 恰好 20 MiB 允许；20 MiB+1 413；完整请求体 21 MiB；无 Content-Length/chunked/伪造长度均受限；EOF 与真实读取错误区分；读取失败不进入 `ImportV2`、零任务创建；双入口与 R29-01 语义保留；导入格式/AES-GCM 流程不变。全部满足。
- **自动化证据边界：** httptest 已验证接口级边界和读取中断；真实反向代理缓冲、真实大文件浏览器上传仍属 Step 15/20 与 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 21 MiB 完整请求体对正常导出文件（历史 ≤3MB）留有充足余量；`readImportFile` 仍在内存持有最多 20 MiB，符合用户决策 3 的“有界缓冲兼容”要求。
- **下一步：** 进入 Step 15：R28-07G 前端文件提前拒绝。

### Step 15：R28-07G 前端文件提前拒绝

- **Step 编号和标题：** Step 15：R28-07G 前端文件提前拒绝。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:37～23:40 CST。
- **前置条件检查：** Step 14 已 ✅ 验收通过；后端固定 20 MiB 文件 / 21 MiB 请求体上限；两个导入入口均使用 Ant Upload `before-upload`。
- **影响评估：** 仅增加选择文件阶段的本地校验和错误提示；合法文件（含恰好 20 MiB）流程、FormData 字段、密码/确认词语义、Setup `IMPORT` 和管理端 `IMPORT→DISABLE` 流程不变。
- **工作树基线重叠情况：** `SetupView.vue`、`SettingsView.vue` 与 Step 18 前端 SSE 不同文件；无用户修改冲突。

**失败优先测试：**
- 新增 `frontend/tests/import-limit.spec.ts`：恰好 20 MiB 通过，20 MiB+1 返回错误；静态确认 SetupView 与 SettingsView 均接入 `importFileError`。
- 临时让 `importFileError` 恒返回 null 以对照未提前拒绝的旧行为，测试稳定失败于超限文件未被拒绝；恢复后通过。

**实施方案：**
- 新增 `frontend/src/utils/fileLimits.ts`：`MAX_IMPORT_FILE_BYTES = 20 × 1024 × 1024` 与 `importFileError(file)`；恰好 20 MiB 允许，超出返回带当前 MiB 数值的中文提示。
- `SetupView.vue` 和 `SettingsView.vue` 的 `onImportFile` 统一先调用 `importFileError`；超限时清空已选文件、`Notify.error` 且 `before-upload` 返回 false，不发起上传。
- 新增 `import-limit.spec.ts` 覆盖边界和两个入口接线。

**修改文件：** `frontend/src/utils/fileLimits.ts`、`frontend/src/views/SetupView.vue`、`frontend/src/views/admin/SettingsView.vue`、`frontend/tests/import-limit.spec.ts`。

**自主决策及依据：** 前端阈值工具与后端 `MaxImportFileBytes` 数值语义一致（20×1024×1024）；仅校验文件大小，不读取文件内容，不改变上传协议；超限时清空 state 防止用户误以为仍可提交。

**验收命令与真实结果：**
- 红：`importFileError` 恒返回 null 后 `npx vitest run tests/import-limit.spec.ts` 失败；“恰好 20 MiB 允许，20 MiB+1 拒绝”。
- `cd frontend && npx vitest run tests/settings-view.spec.ts tests/import-limit.spec.ts`：通过，2 文件/6 用例。
- `npm run build`：通过，仅既有 chunk >600kB 提示。
- `git diff --check`：通过。

**验收标准逐项结论：** 两个界面均在文件选择阶段拒绝 >20 MiB；恰好 20 MiB 允许；超限不发送请求；未修改上传协议和密码/确认词交互；不影响 R29-01 Setup 导入。全部满足。
- **自动化证据边界：** 组件未做真实浏览器点击 Ant Upload 的人工走查；已由静态接线断言和 vitest 边界测试覆盖，真实体验仍可登记 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行；建议用户在 Step 20 后按 ProdTestList 复验导入体验。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 前端拒绝只改善体验，安全边界仍由 Step 14 后端 21 MiB/20 MiB 强制；两者一致。
- **下一步：** 进入 Step 16：R28-07I 后端 SSE 管理员路由与删除 token。

### Step 16：R28-07I 后端 SSE 管理员路由与删除 token

- **Step 编号和标题：** Step 16：R28-07I 后端 SSE 管理员路由与删除 token。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:40～23:44 CST。
- **前置条件检查：** Step 15 已 ✅ 验收通过；`/api/admin/logs/stream` 仍独立注册并依赖一次性查询 Token；`StreamService` 有 tokens map / IssueToken / ConsumeToken / TTL / Reset 复位。
- **影响评估：** SSE 建立方式从“独立路由 + query token”改为“管理员路由组 + 会话/管理员双中间件”；删除 `/stream/token`、IssueToken、ConsumeToken、tokens map/TTL 与 Reset 中的 token 复位。历史缓冲、增量推送、8 连接上限、断开清理和访问日志查询/清空不变；HTTP 401/403/200 合同按管理员路由。
- **工作树基线重叠情况：** `server/log.go`、`log/stream.go`、`log/log_test.go` 尚未被后续 Step 修改；新增 SSE 路由测试。

**失败优先测试：**
- 新增 `TestLogStreamAdminRoute`：未登录 401、普通用户 403、管理员建立 200 SSE。
- 新增 `TestLogStreamTokenEndpointRemoved`：旧 `/api/admin/logs/stream/token` 必须 404。
- 对照旧路由临时恢复“独立无鉴权流 + query token 换取路由”后，上述测试稳定失败（未登录流返回 200、旧 token 路由存在）；恢复后转绿。
- `TestStreamConnectionLimit`、`TestStreamReset` 继续覆盖 8 连接与内存复位；删除 token 状态断言。

**实施方案：**
- `server/log.go`：`/stream` 移入 `g := engine.Group("/api/admin/logs", sessionMW, adminMW)`；删除 `/stream/token` 路由与 `issueStreamToken` handler；`stream` 删除 ConsumeToken 分支，保留 history、flush、增量、断开清理。
- `log/stream.go`：删除 `StreamTokenTTL`、`tokens`、`IssueToken`、`ConsumeToken`、`gcLocked`；`NewStreamService`、`Reset` 不再管理 token；保留 `MaxSSEConnections`、`Subscribe`、`Unsubscribe`、缓冲。
- 测试同步删除 Token 一次性测试，保留并更新连接上限/Reset 测试；新增 SSE 路由测试。

**修改文件：** `backend/internal/server/log.go`、`backend/internal/log/stream.go`、`backend/internal/log/log_test.go`、`backend/internal/server/sse_route_test.go`。

**自主决策及依据：** 管理员 SSE 连接由现有会话中间件解析 `Authorization: Bearer`，前端 Step 17 改用 fetch 后可携带；不新增流内权限重查（Step 18 单独实现）；不改变缓冲大小和连接上限。

**验收命令与真实结果：**
- 红：临时恢复旧无鉴权流 + token 路由后，`TestLogStreamAdminRoute` 失败于“未登录流端点应 401: 200”；旧 token 测试同样失败。
- 绿：`go test ./internal/log ./internal/server -run 'SSE|Stream|Token|ConnectionLimit|Reset' -count=1 -race`：通过（log 1.22s、server 3.05s）。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 0 项、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 无一次性查询 Token 状态；流端点位于会话+管理员双层中间件；历史/增量/连接限制保留；旧 `/stream/token` 不存在；日志查询/清空不回归。全部满足。
- **自动化证据边界：** 后端 httptest 使用真实 HTTP client 建立 SSE 后取消；前端 fetch/ReadableStream 与真实浏览器分帧由 Step 17 和 ProdTestList 验证。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 前端仍在 Step 17 前保留旧 EventSource 实现，当前前后端协议处于中间态；必须连续完成 Step 17 联合验收。
- **下一步：** 进入 Step 17：R28-07I 前端 fetch/ReadableStream/SSE 解析。

### Step 17：R28-07I 前端 fetch/ReadableStream/SSE 解析

- **Step 编号和标题：** Step 17：R28-07I 前端 fetch/ReadableStream/SSE 解析。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:44～23:48 CST。
- **前置条件检查：** Step 16 已 ✅ 验收通过；后端流端点已为管理员路由，旧 token 路由已删除；前端仍使用 `issueStreamToken` + EventSource query token。
- **影响评估：** 前端连接方式改为 `fetch` + `ReadableStream` + 自研 SSE 帧解析，使用现有 Authorization Bearer 凭据；保留暂停/清屏/级别过滤/最多 3 次重连/卸载 abort；401/403 停止重连并提示；不再有 query token。后端合同不变。
- **工作树基线重叠情况：** `api/log.ts`、`LogsView.vue` 与 Step 16 后端接口联动；新增 `utils/sse.ts` 和定向测试。

**失败优先测试：**
- 新增 `frontend/tests/sse-parser.spec.ts`：标准帧、跨 chunk、多行 data、空行、注释、CRLF、flush 尾帧、无冒号 data；`openLogStream` 使用 Bearer 头且 URL 不含 query token。
- 新增 `frontend/tests/logs-view.spec.ts`：源码不含 EventSource/issueStreamToken/`?token=`；401/403 停止重连并提示；ReadableStream 分帧渲染；组件卸载 abort 当前流。
- 对照旧 EventSource 实现的静态与协议断言稳定失败（查询 Token、无 Authorization fetch）；新实现转绿。
- Step 16/17 联合 HTTP 验收：Go `TestLogStreamAdminRoute`（真实 HTTP client 以 Bearer 建立 SSE、401/403）与前端 fetch 协议测试在同一串行命令中通过。

**实施方案：**
- 新增 `frontend/src/utils/sse.ts`：`createSSEParser` 支持跨 chunk 缓冲、CRLF、多行 `data:` 合并、空行分帧、注释忽略、`flush()` 尾帧。
- `frontend/src/api/log.ts`：删除 `issueStreamToken`，新增 `openLogStream(signal)`；从 localStorage 取会话凭据并放在 `Authorization` 头，URL 固定 `/api/admin/logs/stream`，`cache:'no-store'`。
- `frontend/src/views/admin/LogsView.vue`：替换 EventSource 为 `AbortController` + fetch + `ReadableStream` reader + parser；401/403 设置停止重连标志并提示；流结束/错误按 3 次上限重连；卸载 abort；保留 paused/levelFilter/lines/scroll。
- 新增 parser 与 LogsView 定向测试。

**修改文件：** `frontend/src/utils/sse.ts`、`frontend/src/api/log.ts`、`frontend/src/views/admin/LogsView.vue`、`frontend/tests/sse-parser.spec.ts`、`frontend/tests/logs-view.spec.ts`。

**自主决策及依据：**
- SSE 解析器只产生 `{event,data}`，JSON 解码仍留在 LogsView，保持协议解析与业务日志展示分离。
- 401/403 视为不可自动恢复，停止重连；网络错误/正常断流仍按最多 3 次重连，符合用户决策 7 与旧行为。
- `openLogStream` 直接读取 localStorage，与 `api/request.ts` 的现有凭据来源一致，不引入新状态层。

**验收命令与真实结果：**
- `cd frontend && npx vitest run tests/sse-parser.spec.ts tests/logs-view.spec.ts`：通过，2 文件/10 用例。
- 联合 HTTP：`cd backend && go test ./internal/server -run 'TestLogStreamAdminRoute|TestLogStreamTokenEndpointRemoved' -count=1 -race` 通过（2.73s），随后前端同组测试通过。
- `npm run build`：通过，仅既有 chunk >600kB 提示。
- Step 16 后端 race/build/vet/errgate 已通过；`git diff --check`：通过。

**验收标准逐项结论：** fetch 携带现有会话凭据；SSE 解析正确；无 EventSource 查询 Token；401/403 停止并提示；断线重连、暂停/清屏/过滤和卸载清理完整；后端管理员路由配合通过。全部满足。
- **自动化证据边界：** 真实浏览器网络面板、真实 SSE 长连接与反向代理缓冲行为未验证；登记 ProdTestList 人工项（如需要）。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 浏览器/反代对流式响应缓冲差异可能影响首帧；Step 20 接口级和 Production smoke 会继续验证。
- **下一步：** 进入 Step 18：R28-07I 流内权限 15 秒重查。

### Step 18：R28-07I 流内权限 15 秒重查

- **Step 编号和标题：** Step 18：R28-07I 流内权限 15 秒重查。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:48～23:52 CST。
- **前置条件检查：** Step 17 已 ✅ 验收通过；流端点为管理员路由，Handler 已注入 `user.Service` 作为 `auth.UserSource` 的基础可用。
- **影响评估：** 流建立后每 15 秒轻量查库；用户删除、status 非 active、role 非 admin 时关闭流并 `Unsubscribe` 清理。权限不变不误断；最多 8 连接，每次查询很轻；不引入事件总线，不改变前端 3 次重连上限和下载 Token 实时权限逻辑。
- **工作树基线重叠情况：** `server/log.go`、`server/server.go` 与 Step 16/17 连续修改；新增 `sse_permission_test.go`。

**失败优先测试：**
- 新增 `TestStreamPermissionChangeCloses` 子用例：admin→user、active→disabled、删除用户三种权限变化均在约 20ms 测试间隔内关闭流，且连接数释放可再次订阅。
- 新增 `TestStreamPermissionStableAdminConnection`：权限持续为 active/admin 时流保持打开，至少完成 3 次重查，不误断。
- 对照旧实现把 `streamUserAllowed` 临时改为恒 true 后，`TestStreamPermissionChangeCloses/admin-to-user` 稳定失败于客户端 2s 超时；恢复后转绿。

**实施方案：**
- `LogHandler` 增加 `users auth.UserSource` 与测试可注入的 `permissionInterval`（生产零值默认 15 秒）。
- `stream` 在历史/增量 select 循环增加 `permissionTicker` 分支；每 tick 调用 `streamUserAllowed`，查库失败/快照为空/非 active/非 admin 均关闭并写结构化 warn（`user_id`），随后 defer Unsubscribe。
- `server.New` 构造 `LogHandler` 时传入 `users`。
- 新增 SSE 权限测试使用真实 httptest.Server 和客户端取消，避免 ResponseRecorder 并发读写竞态。

**修改文件：** `backend/internal/server/log.go`、`backend/internal/server/server.go`、`backend/internal/server/sse_permission_test.go`。

**自主决策及依据：**
- 权限查询失败按“不可确认权限”处理为关闭流，符合实时权限安全边界；不把 DB 错误当作仍为管理员。
- 生产间隔固定 15 秒；测试通过 Handler 私有字段注入更短间隔，不改变生产常量/环境变量语义。
- 仅查 `SnapshotByID` 的 `role/status`，不引入缓存，符合 AGENTS §4.4“权限每次实时查库”。

**验收命令与真实结果：**
- 红：`streamUserAllowed` 恒 true 后 `go test ./internal/server -run 'TestStreamPermissionChangeCloses/admin-to-user' -count=1` 失败于“context deadline exceeded”。
- 绿：`go test ./internal/server -run 'StreamPermission|PermissionChange|SSE' -count=1 -race`：通过（1.65s）。
- `TestStreamPermissionChangeCloses` 三种变化均通过；`TestStreamPermissionStableAdminConnection` 通过。
- `go build ./...`：通过；`go vet ./...`：通过。
- `go run ./cmd/errgate ./...`：通过，基线 0 项、0 unexpected。
- `git diff --check`：通过。

**验收标准逐项结论：** 权限变化最长约 15 秒内关闭；无 goroutine/连接泄漏（连接数释放、可重新订阅）；权限不变不误断；日志查询/清空不回归；不改变前端重连上限和下载 Token 逻辑。全部满足。
- **自动化证据边界：** 使用真实 httptest HTTP 连接和注入短间隔；不替代真实浏览器等待 15 秒场景，Step 20 接口级清单会再次核验权限变化行为。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 每 8 条连接每 15 秒一次查库，属轻量；无需额外连接池调整。
- **下一步：** 进入 Step 19：R28-07H 颜色 Token 与前端静态门禁。

### Step 19：R28-07H 颜色 Token 与前端静态门禁

- **Step 编号和标题：** Step 19：R28-07H 颜色 Token 与前端静态门禁。
- **状态：** ✅ 验收通过。
- **开始/完成时间：** 2026-09-11 23:52～23:55 CST。
- **前置条件检查：** Step 18 已 ✅ 验收通过；SSE 前端文件已冻结；`style-tokens.spec.ts` 仍登记 `NodeCheckPanel.vue` 的 `bg-gray-50`。
- **影响评估：** 仅替换 NodeCheckPanel 预览背景类为设计 Token `bg-surface-subtle`，浅色/深色主题随 `--ui-surface-subtle` 变量生效；检查行为、数据合同、终端固定深色样式和第三方内容不变。
- **工作树基线重叠情况：** `NodeCheckPanel.vue` 与 Step 17 前端文件无冲突；`style-tokens.spec.ts`、`node-check-panel.spec.ts` 为 Step 1 建立的静态门禁和既有组件测试扩展。

**失败优先测试：**
- 先清空 `styleTokenAllowlist` 并保留旧 `bg-gray-50`，运行 `npx vitest run tests/style-tokens.spec.ts tests/node-check-panel.spec.ts`：静态门禁稳定失败并定位 `src/components/NodeCheckPanel.vue:110 bg-gray-50`；新增的双主题 DOM 断言同时失败于 `<pre>` 不含 `bg-surface-subtle`。
- 修复后同一组测试通过：静态扫描零违规；light/dark 两种根类名下 `<pre>` 均含 `bg-surface-subtle` 且不含 gray/white/black 遗留类。

**实施方案：**
- `frontend/src/components/NodeCheckPanel.vue`：预览 `<pre>` 背景 `bg-gray-50` → `bg-surface-subtle`。
- `frontend/tests/style-tokens.spec.ts`：删除 NodeCheckPanel 允许项，允许清单清零；保留 `whitespace`/`whitelist` 不误报断言。
- `frontend/tests/node-check-panel.spec.ts`：新增 `it.each([false,true])` 双主题断言，检查 Token 类存在和遗留类不存在，并在测试后清理 dark 根类。
- 未改 `theme.spec.ts` 既有 Token/CSS 变量断言；按 Build26 命令一并执行。

**修改文件：** `frontend/src/components/NodeCheckPanel.vue`、`frontend/tests/style-tokens.spec.ts`、`frontend/tests/node-check-panel.spec.ts`。

**自主决策及依据：** 使用既有 Tailwind 语义色 `bg-surface-subtle`（映射 `--ui-surface-subtle`）而不是新增颜色 Token，保持主题变量体系不变；双主题断言使用根 `.dark` 类切换，和项目 darkMode: 'class' 一致。

**验收命令与真实结果：**
- 红：清空允许项后 `npx vitest run tests/style-tokens.spec.ts tests/node-check-panel.spec.ts` 失败，3 个用例失败（样式门禁 + 两个主题断言）。
- 绿：`npx vitest run tests/style-tokens.spec.ts tests/node-check-panel.spec.ts tests/theme.spec.ts`：通过，3 文件/12 用例。
- `npm test -- --run`：通过，46 文件/275 用例。
- `npm run build`：通过，仅既有 chunk >600kB 提示。
- `git diff --check`：通过。

**验收标准逐项结论：** `frontend/src` 无 gray/white/black Tailwind 颜色工具类违规；两个主题类名下 DOM class 符合设计 Token；`whitespace`/`whitelist` 不误报；终端固定深色、第三方内容和 R29-06 行为未改。全部满足。
- **自动化证据边界：** 组件测试不替代真实浏览器视觉走查；浅色/深色真实渲染仍登记 ProdTestList 人工项。
- **Production smoke 边界：** 未执行；Step 20 统一执行。
- **浏览器人工边界：** 未执行。
- **真实部署/真实客户端边界：** 未执行。
- **遗留风险：** 无功能性遗留；视觉变量最终效果由 Step 20 前端构建与人工项覆盖。
- **下一步：** 进入 Step 20：联合回归、文档同步与关闭条件核验。

### Step 20：联合回归、文档同步与关闭条件核验

- **Step 编号和标题：** Step 20：联合回归、文档同步与关闭条件核验。
- **状态：** ✅ 验收通过；本记录已按归档规则移入 `docs/reports/Build/Build26.md`。
- **开始/完成时间：** 2026-09-11（Step 19 完成后连续执行；本 Step 为联合回归与文档收口）。
- **前置条件检查：** Step 1～19 全部 ✅ 验收通过；Step 19 结束时前端全量 46 文件/275 用例、build 和 `git diff --check` 通过；后端和 Docker/Production smoke 在本 Step 统一重新执行。
- **影响评估：** 本 Step 无新增业务逻辑或 schema/合同变化，只执行真实联合门禁、接口级回归、隔离 Production smoke 和文档同步；不修改 R28-07F、R28-08/R28-09、Issue15 其他问题、SecurityScanPlan1 或 Design5。
- **失败优先检查：** 本 Step 不新增行为测试；任何最终门禁失败都必须记录真实失败输出并回到对应 Step 修复，不能用文档说明或历史绿色结果替代。实际执行未出现失败，故最终验收阶段为全绿；各 Step 失败优先红测证据见对应 Step 记录。

**联合门禁命令与真实结果（2026-09-11，仓库路径按预检规范化）：**

```bash
cd /Users/kylechen/Desktop/Repo/VPN-Subscription-Management/backend
go test ./... -count=1 -timeout 300s
go test -race ./internal/log ./internal/response ./internal/server ./internal/home ./internal/token ./internal/pool ./internal/xray ./internal/oidc ./internal/config ./internal/custom ./internal/user
go run ./cmd/errgate ./...
go test ./internal/server -run 'ArchitectureNoDirectStore|SSE|Stream|Import|401|403|413' -count=1
go build ./...
go vet ./...
```

- `go test ./... -count=1 -timeout 300s`：通过；`go list ./...` 共 45 个包，其中 40 个有测试的包全部 `ok`，另 5 个为 `no test files`，无失败、无 panic、无超时。
- `go test -race ...`：通过，`log/response/server/home/token/pool/xray/oidc/config/custom/user` 指定包竞态测试全部通过。
- `go run ./cmd/errgate ./...`：通过，输出 `OK (0 ignored errors matched baseline; baseline entries 0; 0 unexpected)`；基线文件为空、无未预期忽略 error。
- `go test ./internal/server -run 'ArchitectureNoDirectStore|SSE|Stream|Import|401|403|413' -count=1`：通过；覆盖 `internal/server` 非测试生产文件零 `DB()`/`TxImmediate()`、SSE 管理员路由 401/403/可建立流、旧 token 路由删除、权限变化关闭、导入 20 MiB/21 MiB 边界与超限 413、读取错误 500 零任务。
- `go build ./...`：通过。
- `go vet ./...`：通过。

```bash
cd /Users/kylechen/Desktop/Repo/VPN-Subscription-Management/frontend
npm test -- --run
npm run build
```

- `npm test -- --run`：通过，46 个测试文件、275 个用例全部通过。
- `npm run build`：通过；仅保留仓库既有 main chunk >600 kB 提示，无新增错误或合同变化。

```bash
cd /Users/kylechen/Desktop/Repo/VPN-Subscription-Management
git diff --check
docker compose build
bash .smoke-test-prod.sh
```

- `docker compose build`：通过，镜像构建成功。
- `bash .smoke-test-prod.sh`：通过；在隔离临时容器 `127.0.0.1:18081` 和临时数据卷上执行，未使用真实凭据/生产数据；正式脚本完整结束于 `=== PROD SMOKE ALL DONE ===`。
- `git diff --check`：通过；最终文档同步后已再次执行（见本 Step 收口结论）。

**接口级回归清单结果：**

- SSE：未登录 401、非管理员 403、管理员可建立流；旧 `/api/admin/logs/stream/token` 不存在；流内权限变化关闭、稳定管理员连接不误断、连接释放与重订阅通过。
- 导入：Setup/管理双入口精确 20 MiB 文件通过；20 MiB+1、无 Content-Length/chunked、伪造长度和总请求体超过 21 MiB 返回 413；真实读取错误返回 500 且零任务创建；前端 20 MiB+1 提前拒绝测试通过；R29-01 的 `IMPORT`/`DISABLE` 语义未回归。
- 前端：SSE 分帧/CRLF/多行 data/注释/`flush`、401/403 停止重连、组件卸载 abort；颜色静态门禁零违规和双主题 DOM 断言通过。
- 静态门禁：errgate 基线 0 项、`internal/server` 架构允许清单 0 项、颜色允许清单 0 项。

**文档同步结果：**

- `Design4.md`：已增加 §12.7“核心工程约束现行合同补充”，覆盖 R28-07A/B/D/E/G/I 的现行口径并明确 F 不实施，版本记录 v1.22。
- `Issue14.md`：已更新 R28-07 状态、步骤五关闭条件与 v1.25 变更记录；R28-07F 保持“设计取向、不实施”，未写成已修复。
- `AGENTS.md`：已同步 Build26 完成状态、当前构建记录/归档入口、Design4 §12.7 入口，未加入具体设计细节。
- `TODOLIST.md`：P2-1～P2-10 已按真实验收结果勾选，活跃文档快照、P2 进展和变更记录已同步；P3/P4/P5 仍保持未启动。
- `ProdTestList.md`：新增 Build26 导入体积、SSE 管理端日志流、主题 Token 人工核验章节和记录表，v2.17；所有新增项目均保持未执行、未标记人工通过。
- `Issue15.md`：本轮未新增、未关闭任何 Issue15 问题，也无必需交叉引用项，因此未修改；Build26 不处理 Issue15 其他问题。
- 未修改任何归档文档为“原报告错误”，未回写 Build27.md（属另一构建计划，超出 Build26 范围）。

**验收标准逐项结论：**

- 全部门禁真实通过：后端全量/race/build/vet、errgate、架构/SSE/导入接口回归、前端全量/build、Docker build、隔离 Production smoke、`git diff --check` 均满足。
- R28-07A～E、G～I 均有代码、失败优先/定向回归、全量门禁和文档同步证据；R28-07F 仍记录为设计取向、未实施。
- Issue14 步骤五关闭条件满足；未进入步骤七/步骤八、R28-08/R28-09、SecurityScanPlan1 或 Design5 范围。

**证据边界与遗留风险：**

- Production smoke 是正式脚本驱动的隔离容器/API 级证据，不是真实浏览器、真实手机、真实客户端或真实部署人工结论。
- ProdTestList 新增的人工项目全部未执行；自动化与 smoke 不得替代人工通过。
- 日志流每 8 条连接每 15 秒一次轻量查库属于设计边界；真实反代/浏览器流式缓冲仍需人工核验。导入真实读取异常、主题视觉观感同样留给人工项。
- 无数据库 schema、导入文件格式、AES-GCM 整体格式、下载产物、HTTP/JSON 合同变化。

**归档决定：** 按 2026-09-11 用户决策第 1 条与 AGENTS 归档规则，Step 20 验收通过后本记录从根目录移入 `docs/reports/Build/Build26.md`，标题标记“已归档”，外部参考同步指向归档路径；归档后仅作核查，不再作为执行入口。Build27.md 保持原样，未授权 Step 1～6，不在本轮启动。

**下一步：** Build26 范围内无后续 Step；不开始 Issue14 步骤七/八、R28-08、R28-09、SecurityScanPlan1 或 Design5，等待用户另行授权。
