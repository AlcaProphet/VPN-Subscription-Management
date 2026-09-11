# VPN 订阅管理系统 功能构建计划（Build26：Issue14 步骤五 R28-07 核心工程约束整改）

> **文档定位：** 本文档是 Issue14 步骤五、R28-07A～R28-07I 的**唯一详细构建记录**。承接已归档 [Build25.md](docs/reports/Build/Build25.md)，只处理 R28-07A～E、R28-07G～I；R28-07F 是用户已确认的设计取向，只记录、不实施。本文件不进入 R28-08、R28-09、Issue15 其他问题、`SecurityScanPlan1.md` 或 `Design5.md` 范围。
>
> **关联文档：**
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 当前设计：[Design4.md](Design4.md)（实施完成后增加“核心工程约束现行合同补充”；不修改归档 Design1）
> - 问题追踪：[Issue14.md](Issue14.md)（步骤五、R28-07A～I）
> - 人工测试：[ProdTestList.md](ProdTestList.md)（本轮不新增人工通过结论）
> - 历史核验：[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §3.2/§6.3、[BuildReport5.md](docs/reports/BuildReport/BuildReport5.md)
> - 构建模板：[Build.template.md](docs/DocTemplates/Build.template.md)
>
> **授权方式（2026-09-11 用户确认）：** 先授权创建本完整 Build26 文档，再逐 Step 实施。每次只执行一个 Step，该 Step 通过验收并汇报真实结果后，必须等待下一 Step 授权；不得跨 Step、不得并行、不得提前标记后续 Step。

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
| 1 | 失败优先回归基座与架构/error/颜色静态门禁 | AGENTS §3.3/§3.4；Issue14 R28-07C/D/H | ☐ 未开始 |
| 2 | R28-07A 首管理员冗余标记 | Issue14 R28-07A；Design1 §2.5（历史） | ☐ 未开始 |
| 3 | R28-07B 自定义订阅与隐藏组 Token 协调 | Issue14 R28-07B；Design1 §2.3/§4.2 | ☐ 未开始 |
| 4 | R28-07D1 用户下载渲染服务化 | Issue14 R28-07D；Design2 §5.7 | ☐ 未开始 |
| 5 | R28-07D2 流量汇总 Xray 业务化 | Issue14 R28-07D；Design2 §5.8/§5.10 | ☐ 未开始 |
| 6 | R28-07D3 OIDC ticket 服务化 | Issue14 R28-07D；Design1 §3.2/§5.4 | ☐ 未开始 |
| 7 | R28-07D4 架构门禁清零 | AGENTS §五；Issue14 R28-07D | ☐ 未开始 |
| 8 | R28-07E1 debug 请求上下文化 | Issue14 R28-07E；AGENTS §4.3 | ☐ 未开始 |
| 9 | R28-07E2 Logger/级别控制器实例注入 | Issue14 R28-07E；AGENTS §五 | ☐ 未开始 |
| 10 | R28-07E3 敏感键固定集合 | Issue14 R28-07E；AGENTS §4.2 | ☐ 未开始 |
| 11 | R28-07C1 素材池同步错误语义与终态清理 | Issue14 R28-07C | ☐ 未开始 |
| 12 | R28-07C2 补偿删除与 OIDC ticket 清理错误 | Issue14 R28-07C；R28-07D3 产出 | ☐ 未开始 |
| 13 | R28-07C3 全量错误审计与 errgate 清零 | Issue14 R28-07C；用户决策 6/9 | ☐ 未开始 |
| 14 | R28-07G 后端 20 MiB/21 MiB 上限 | Issue14 R28-07G；用户决策 3 | ☐ 未开始 |
| 15 | R28-07G 前端文件提前拒绝 | Issue14 R28-07G；用户决策 3 | ☐ 未开始 |
| 16 | R28-07I 后端 SSE 管理员路由与删除 token | Issue14 R28-07I；用户决策 7 | ☐ 未开始 |
| 17 | R28-07I 前端 fetch/ReadableStream/SSE 解析 | Issue14 R28-07I；用户决策 7 | ☐ 未开始 |
| 18 | R28-07I 流内权限 15 秒重查 | Issue14 R28-07I；用户决策 7 | ☐ 未开始 |
| 19 | R28-07H 颜色 Token 与前端静态门禁 | Issue14 R28-07H；用户决策 10 | ☐ 未开始 |
| 20 | 联合回归、文档同步、关闭条件核验 | Issue14 步骤五关闭条件；用户决策 10/11 | ☐ 未开始 |

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
- **前置条件：** 用户已明确授权“先创建完整 Build26，再逐 Step 实施”。
- **产出文件：** 根目录 `Build26.md`。
- **失败优先检查：** 无代码改动；检查本文件是否完整包含 R28-07A～I、F 排除、用户决策、每 Step 验收和联合门禁。
- **验收命令：**

```bash
cd /Users/kyle/Desktop/Repo/VPN-Subscription-Management
test -f Build26.md
git diff --check
```

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
