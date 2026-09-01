# Build11.md — VPN 订阅管理系统 UI/UX 改进当前构建方案（v2.1）

> **文档定位：** 本文档是承接 UI/UX 研究报告（[UIReport1.md](../UI/UIReport1.md)、[UIReport2.md](../UI/UIReport2.md)）后的**当前构建方案**。v1.2 已从头重新核验前后端源码与报告，修正可执行性、后端契约与测试落点；v1.3 同步用户构建前决策并落地 Step 1。目标是让 Build11 具备稳定、可逐步执行的能力。
> - 研究依据：`docs/reports/UI/UIReport1.md`、`docs/reports/UI/UIReport2.md`
> - 当前设计：`Design2.md`、`Design2-UI.md`（本卷已获用户确认，Build11 相关章节已同步）
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（唯一强要求）
> - 历史构建：`docs/reports/Build/`（Build1~Build10 均已存档，仅核查）
>
> **执行原则：**
> - 每次仅执行一个 Step，完成后运行验收命令，确认通过后再进入下一步。
> - 排序原则：先修复后构建、先依赖后独立、先安全后优化。
> - 本卷允许**最小后端调整**，但不改变产品功能、权限模型和既有业务语义。
>
> **本版状态：** v2.1 已完成 Step 1~7 及 Issue9 R24-01 后端死锁修复；前端 `npm run build` 与 `npm test -- --run`（97 tests）保持既有通过记录；后端 `go build ./...`、`go vet ./...`、`go test -timeout 180s ./...` 已于 2026-08-28 全部通过。Build11 整体已收口，后续仅剩 Production 人工核查与归档。
>
> ✅ 2026-08-28 R24-01 验收补充：`ExtService.ListExt`、`ExtService.CheckAllExtQuota` 已改为先读完并关闭外层游标再执行补充数据库操作，`SyncService.writeQuotaExceededError` 已改为单条批量更新；三个非空数据定向回归与 `TestOverviewEndpointAggregatesStableData` 均通过，解除 [ProjectWideReport2.md](../ProjectWideReport/ProjectWideReport2.md) §4.1 记录的后端阻塞。

---

## 〇、已确认决策（用户拍板）

| ID | 决策项 | 用户选择 | 说明 |
|----|--------|----------|------|
| D1 | 改进优先级 | 按 UIReport2：可信状态 → 手机可操作 → 结构统一 → 视觉系统 | — |
| D2 | 管理导航 | 分组但不折叠 | 需修改 `Design2-UI.md §2.1` |
| D3 | 视觉 Token | `#2563EB` + UIReport2 §6.2 双主题 Token | 需修改 `Design2-UI.md §1.1` |
| D4 | 设置页 | 同一路由 + 桌面分组导航 + 手机 Select | — |
| D5 | 手机复杂表单 | 全部 Modal 表单统一为手机全屏 Drawer；已是独立页的表单只做响应式优化 | — |
| D6 | 首次发布引导 | 新增 `/admin` 概览页，内容含状态 + 发布清单 + 计数 + 快捷入口 + **动态摘要** | — |
| D7 | 页面覆盖 | 包含高级模式页面（Xray、用户组） | — |
| D8 | 后端边界 | 允许最小后端调整 | — |
| D9 | 重置链接状态 | 新增后端校验接口，支持缺失/过期/已使用三态 | 需保留 `used` 标记 |
| D10 | 概览页数据源 | 新增 `GET /api/admin/overview` 汇总接口 | — |
| D11 | 版本页真实资源名 | 新增 `GET /api/admin/versions/:id/owner` 通用归属接口 | 以版本 ID 反查 owner |
| D12 | 装配警告过滤 | 后端 `Warnings()` 为主，前端按 `targetSyntax` 兜底过滤 | — |
| D13 | 概览页入口 | 管理菜单顶部加“概览”，AppHeader“管理面板”跳 `/admin` | — |
| D14 | 重置校验限流 | 新增设置页可配置的“重置校验”限流，默认 10 次/分钟 | 偏离 v1.2“不新增限流配置键”，用户拍板 |
| D15 | 概览动态摘要 | 静态最近 5 条待审批 + 最近 5 条访问日志 | 不做实时日志流 |
| D16 | 手机 Drawer 方向 | 底部全屏 Drawer（`placement="bottom"`, `100dvh`） | 仅改 `FormOverlay` 配置可调整 |
| D17 | 高级模式 UI 审计 | 现在先做完整审计（Xray/用户组），Step 5 前完成 | 已按源码/设计做静态走查 |
| D18 | 概览 checklist | 现在定稿：5 步基线 + `member_check` 恒为人工步骤 | Step 5 按此实现 |
| D19 | Token 对比度 | 现在预校验并定稿：当前候选值均达 WCAG AA | Step 6 按定稿 Token 实施 |
| D20 | 构建节奏 | 逐个完成全部 Step，每完成一个 Step 后执行对应验收 | — |

### 0.1 构建前补充定稿与审计记录

#### 0.1.1 概览页 Checklist 定稿

| key | done 条件 | label | action_path | action_label |
|-----|-----------|-------|-------------|-------------|
| `platforms` | 平台数 > 0 | 创建至少一个平台 | `/admin/platforms` | 创建平台 |
| `subscriptions` | 订阅数 > 0 | 为平台创建订阅条目 | `/admin/subscriptions` | 新建订阅 |
| `nodes` | `usable_nodes > 0` | 添加至少一个可用节点 | `/admin/nodes` | 新建节点 |
| `version_active` | 任一订阅 `current_version > 0` | 生成并激活首个版本 | `/admin/assembly` | 前往装配 |
| `member_check` | 恒为人工步骤，初始 `done=false`、`manual=true` | 以普通用户身份检查 | `/` | 查看用户首页 |

> `nodes.done` 必须基于 `usable_nodes`，不以原始节点总数判断，避免停用/缺失节点误判完成。

#### 0.1.2 Token 对比度预校验结果

| 组合 | 对比度 | 结论 |
|------|--------|------|
| 浅色正文 `#0F172A` / 表面 `#FFFFFF` | 17.85:1 | AA |
| 浅色辅助文字 `#64748B` / 表面 `#FFFFFF` | 4.76:1 | AA |
| 浅色主色 `#2563EB` / 表面 `#FFFFFF` | 5.17:1 | AA |
| 深色正文 `#F8FAFC` / 表面 `#111827` | 16.96:1 | AA |
| 深色辅助文字 `#94A3B8` / 表面 `#111827` | 6.92:1 | AA |
| 深色主色 `#60A5FA` / 表面 `#111827` | 6.98:1 | AA |
| 浅色 primarySoft `#EFF6FF` / primary `#2563EB` | 4.75:1 | AA |
| 深色 primarySoft `#172554` / primary `#60A5FA` | 5.78:1 | AA |

> Step 6 实施时按此结论使用 UIReport2 §6.2 / Build11 Step 6 中的 Token 值，无需再调整主色。

#### 0.1.3 高级模式页面静态审计结论（Step 5 输入）

- **XrayInstancesView**
  - 页头同时存在“开始初始化”和“新增实例”两个主操作，不符合“每页一个主操作”约束。
  - 实例 Tab 内“实例列表 / 刷新节点 / 对账”仍平铺，未按三层结构重组。
  - 新增/编辑实例、独立账号、检测回执、对账均使用普通 Modal，未迁移 FormOverlay。
  - 桌面/手机操作均为 `size="small"`，危险项与普通项并排；手机卡片缺少“更多”收口。
  - 列表直接暴露内部 `slug`，手机端信息密度偏高。
- **GroupsView**
  - 编辑组为单一大 Modal，未按“基础信息 / 流量 / 可用节点”分段。
  - 页头缺少“用户组影响说明”。
  - 删除/编辑操作均为 `size="small"`，手机触控与误触风险较高。
  - 未使用 FormOverlay / 手机 Drawer。
- **处理建议**：上述问题在 Step 4 表单载体迁移与 Step 5 高级页重组中统一处理，无需新增后端能力。


---

## 一、当前代码事实与后端影响总表

> 以下事实经源码核对，是各 Step 的前置依据；路径均为项目内实际文件。

### 1.1 可信状态问题（代码事实）

| 问题 | 当前事实 | 修复层 |
|------|----------|--------|
| 装配旧预览 | `AssemblyView.vue` 的 `previewText` 在切换类型后不清空；`doGenerate()` 不校验预览新旧 | 纯前端（Step 2） |
| 无关空规则警告 | `backend/internal/assembly/service.go` `Warnings()` 对四种 targetSyntax 无条件输出“未选择任何规则素材池或手动规则，将生成空规则” | 后端为主，前端兜底（Step 1/2） |
| `/emergency` 非应急误显示 | `EmergencyView.vue` 无条件渲染应急 UI；`GET /api/system/status` 已返回 `emergency`/`emergency_reason`/`can_reset_password` | 纯前端（Step 2） |
| `/reset` 无 token 仍可输入 | `ResetView.vue` 未在初始化阶段判断 token；后端只有 `POST /api/auth/reset` 提交即消费 | 后端新增校验 + 前端三态（Step 1/2） |
| OIDC 回调错误 | 后端已生成 `state_mismatch`、`state_expired`、`exchange_failed` 等 query key；`OidcCallbackView.vue` 丢弃接口错误并统一跳 `exchange_failed`；`LoginView.vue` 直接展示内部 key；`request.ts` 的 401 拦截器会把 `/login/callback` 的交换失败抢先跳转登录页 | 前端映射 + 拦截器例外（Step 2） |
| 装配跨页上下文丢失 | 装配前置条件跳 `/admin/nodes`、`/admin/subscriptions` 前不保存草稿；无 ContextBar | 纯前端（Step 2） |

### 1.2 手机/结构与视觉问题（代码事实）

| 问题 | 当前事实 |
|------|----------|
| 管理导航 | `AdminLayout.vue` 为 10~12 项单层 `Menu`，无分组 |
| 页面标题 | `PageHeader.vue` 输出 `<h2 class="text-lg">`；管理页均无页面级 `h1` |
| 设置页 | `SettingsView.vue` 15 个锚点分区连续堆叠，每区独立保存但无未保存聚合 |
| 手机控件 | `AppHeader.vue` 同时放汉堡/站点名/管理入口/用户名/主题开关；大量 `size="small"` 与 `size="small"` Switch |
| 复杂表单 | `NodesView.vue` 新建/编辑节点用 720px Modal；Xray、用户组、代理组、素材池、订阅、分享、规则、用户等同样存在 Modal 表单 |
| 视觉 Token | `theme.ts` 仅配置 `colorPrimary`；Tailwind `theme.extend` 为空；暗色由 AntD 算法与 `gray-*` 各自决定 |
| 版本标题 | `VersionManageView.vue` 显示 `resourceName`（路由传的是 ownerType 英文） |
| `/admin` 路由 | 前端无 `/admin` 精确路由；后端 `emergency_gate.go` 的 `isSPAPath` 只覆盖 `/admin/` 前缀，不含精确 `/admin` |

### 1.3 后端影响矩阵

| 变更 | 后端文件 | 影响与约束 |
|------|----------|------------|
| 重置 token 校验 + used 保留 + 限流 | `internal/auth/reset.go`、`internal/server/auth.go`、`internal/ratelimit/ratelimit.go`、`internal/config/admin.go`、`frontend/src/api/settings.ts`、`frontend/src/views/admin/SettingsView.vue`、测试 | 改 `Complete` 从 DELETE 改为 `UPDATE used=1`；新增 `POST /api/auth/reset/validate`；新增设置页可配置限流（默认 10/min）；不新增数据库列 |
| 装配警告按目标过滤 | `internal/assembly/service.go`、测试 | 仅 `clash-yaml` 与 `sr-conf` 输出规则空警告；`sr-subs`/`generic-subs` 不输出 |
| `/admin` 概览汇总 | 新增 `internal/server/overview.go` 及测试；`internal/approval/approval.go` 新增最近待审批只读方法 | 新增 `GET /api/admin/overview`；只读聚合，不改变任何业务数据 |
| 版本归属反查 | `internal/version/version.go`、`internal/server/version_owner.go`、`internal/rule/rule.go`、`internal/custom/custom.go` 及测试 | 新增 `GET /api/admin/versions/:id/owner`；只读查询 |
| 应急网关 `/admin` 精确路径 | `internal/server/emergency_gate.go` | `isSPAPath` 增加 `path == "/admin"` |
| 其余 UI 步骤 | 无后端改动 | 仅前端组件、路由、样式、测试 |

---

## 二、构建进度追踪

| Step | 内容 | 依赖 | 状态 |
|------|------|------|------|
| 0 | 创建并完善 Build11 文档 | — | ✅ 已完成（本文档 v1.3） |
| 1 | 后端：重置 token 三态 + 装配警告语义修正 + 设置页重置校验限流 | Step 0 | ✅ 验收通过（用户已确认） |
| 2 | 前端：可信状态修复（预览指纹、应急/重置/OIDC、跨页上下文） | Step 1 | ✅ 验收通过 |
| 3 | 后端：`/admin/overview` + 版本归属接口 + 应急网关 `/admin` | Step 0 | ✅ 验收通过 |
| 4 | 前端：手机任务可完成性（顶栏、触控、FormOverlay） | Step 2 | ✅ 验收通过 |
| 5 | 前端：信息架构与结构统一（PageShell、菜单分组、设置分组、概览页、版本标题、高级模式页） | Step 3/4 | ✅ 验收通过 |
| 6 | 前端：视觉 Token 与主题统一 | Step 5 | ✅ 验收通过 |
| 7 | 效率润色、测试补强与文档收口 | Step 1~6 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 三、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/auth/reset.go`、`backend/internal/server/auth.go`、`backend/internal/assembly/service.go`、`backend/internal/auth/reset_test.go`、新增 `backend/internal/cron/cleanup_test.go`、`backend/internal/assembly/assembly_test.go`、`backend/internal/ratelimit/ratelimit.go`、`backend/internal/config/admin.go`、新增 `backend/internal/server/reset_validate_test.go`、`frontend/src/api/settings.ts`、`frontend/src/views/admin/SettingsView.vue` | 重置三态、used 保留与清理测试、按 target 过滤警告、重置校验限流（设置页可配置，默认 10/min） |
| 2 | `frontend/src/views/admin/AssemblyView.vue`、`frontend/src/views/admin/assembly/{PreviewStep,AssemblerShell}.vue`、`frontend/src/views/{EmergencyView,ResetView,OidcCallbackView,LoginView}.vue`、`frontend/src/api/auth.ts`、`frontend/src/api/request.ts`、`frontend/src/layouts/AdminLayout.vue`、新增 `frontend/src/components/ContextBar.vue` 及测试 | 预览指纹与过期态、状态页恢复、401 拦截例外、错误映射、跨页返回 |
| 3 | 新增 `backend/internal/server/overview.go`、`backend/internal/server/version_owner.go`、`backend/internal/server/overview_test.go`；改造 `backend/internal/version/version.go`、`backend/internal/approval/approval.go`、`backend/internal/approval/approval_test.go`、`backend/internal/rule/rule.go`、`backend/internal/custom/custom.go`、`backend/internal/server/emergency_gate.go`、`backend/internal/server/server.go` 及测试 | 概览聚合接口、最近待审批只读方法、版本归属反查、应急网关 `/admin` |
| 4 | 新增 `frontend/src/components/FormOverlay.vue`；`frontend/src/components/AppHeader.vue`、`frontend/src/layouts/AdminLayout.vue`、`frontend/src/views/admin/{NodesView,XrayInstancesView,GroupsView,ProxyGroupsView,assembly/PoolTab,assembly/PoolDetail,SubscriptionsView,SharesView,RulesView,UsersView,VersionManageView,AssemblyView}.vue`；全局样式 | 手机顶栏精简、44px 命中区、全部 Modal 表单迁移 Drawer |
| 5 | 新增 `frontend/src/components/{PageShell,EmptyState,StateContainer,ResponsiveCollection,FormSection,PreviewState}.vue`、`frontend/src/views/admin/AdminOverviewView.vue`、`frontend/src/api/overview.ts`；改造 `PageHeader.vue`、`TriStateList.vue`、`AdminLayout.vue`、`SettingsView.vue`、`VersionManageView.vue`、`XrayInstancesView.vue`、`GroupsView.vue`、`router/index.ts`、`AppHeader.vue` | 页面骨架、菜单分组、设置六分组、概览页、真实版本标题、高级页重组 |
| 6 | `frontend/src/theme.ts`、`frontend/src/style.css`、`frontend/tailwind.config.js`、`frontend/src/router/index.ts`、`frontend/src/components/MarkdownView.vue`、`frontend/src/views/HomeView.vue`、全部含 `gray-*`/`dark:` 硬编码类名的 40 个 Vue 文件 | 双主题 Token；AntD 用具体色值、Tailwind 用 CSS 变量；暗色表面分层 |
| 7 | `frontend/src/views/admin/assembly/PreviewStep.vue`、`frontend/src/components/DiffView.vue`、`frontend/tests/`、`.smoke-test.sh`、`.smoke-test-prod.sh`、`Design2-UI.md`、`AGENTS.md` | 预览/差异增强、统一反馈、测试与文档收口 |

---

## 四、构建顺序依赖图

```
Step 1（后端：重置三态 + 警告语义）
  → Step 2（前端：可信状态，依赖 reset/validate 契约）
Step 2 → Step 4（手机任务，依赖状态可信稳定后改造表单）
Step 3（后端：概览/版本归属，可与 Step 1/2 并行观察）
  → Step 5（前端：结构统一与概览页，依赖 Step 3 契约与 Step 4 表单载体）
Step 5 → Step 6（视觉 Token，依赖页面骨架稳定）
Step 1~6 → Step 7（润色/测试/文档收口）
```

> 后端最小调整统一在 Step 1 与 Step 3 完成，并在本文件变更记录中注明。

---

## 五、分步构建计划

### Step 1：后端 — 重置 token 三态 + 装配警告语义修正

- **目标：** 让前端能预先判定重置链接状态；让装配 API 不再对节点订阅返回无关规则警告。
- **前置条件：** 无。

#### 1.1 重置 token 三态

1. `backend/internal/auth/reset.go`：
   - 新增状态类型与只读校验方法（不消费、不删除 token）：

  ```go
  type ResetTokenStatus string

  const (
      ResetTokenMissing ResetTokenStatus = "missing"
      ResetTokenUsed    ResetTokenStatus = "used"
      ResetTokenExpired ResetTokenStatus = "expired"
      ResetTokenValid   ResetTokenStatus = "valid"
  )

  // Validate 只读校验，不消费、不删除；供 /reset 页面初始化判定状态。
  func (s *ResetService) Validate(ctx context.Context, token string) (ResetTokenStatus, error) {
      var expiresAt time.Time
      var used int
      err := s.store.DB().QueryRowContext(ctx,
          `SELECT expires_at, used FROM password_reset_tokens WHERE token = ?`, token).
          Scan(&expiresAt, &used)
      if errors.Is(err, sql.ErrNoRows) { return ResetTokenMissing, nil }
      if err != nil { return "", err }
      if used == 1 { return ResetTokenUsed, nil }
      if time.Now().After(expiresAt) { return ResetTokenExpired, nil }
      return ResetTokenValid, nil
  }
  ```

   - `Complete` 中把“用后即删”改为**标记已使用**：
     `UPDATE password_reset_tokens SET used = 1 WHERE token = ?`。
   - 保留 `IMMEDIATE` 事务，二次使用因 `used=1` 失败；已用记录由既有 `cron.StartResetTokenCleanup` 每日清理。
2. `backend/internal/server/auth.go`：
   - 注册 `g.POST("/reset/validate", limiter.Middleware("reset_validate", ratelimit.KeyResetValidate, 10), h.validateReset)`，token 走请求体，不进 URL/访问日志。
   - handler 返回 `OK(c, gin.H{"status": status})`。
   - **用户已确认：新增限流，且进入设置页可配置。** 新增 `ratelimit.KeyResetValidate`（默认 10/min）、`config.RateLimitSettings.ResetValidate`，并在面板“速率限制与连接防护”增加“重置校验（次/分钟）”输入框；旧前端未提交时按默认 10 保存。
3. 测试：
   - 更新 `backend/internal/auth/reset_test.go`：
     - “用后即删”断言改为“Complete 后 token 行 `used=1`，二次 Complete 失败”；
     - 新增 `TestResetValidateStatuses` 覆盖 valid/expired/used/missing。
   - 新增 `backend/internal/cron/cleanup_test.go`：`TestResetCleanup` 直接调用 `cleanupResetTokensOnce`，验证 used=1/过期行被删除、有效行保留。
   - 新增 `backend/internal/server/reset_validate_test.go`：覆盖 `/api/auth/reset/validate` 四态与默认 10/min 限流（第 11 次 429）。

#### 1.2 装配警告语义修正

1. `backend/internal/assembly/service.go` `Warnings()`：

  ```go
  if (in.TargetSyntax == ClashYAML || in.TargetSyntax == SrConf) &&
      len(in.Pools) == 0 && len(in.CustomRules) == 0 {
      warnings = append(warnings, "未选择任何规则素材池或手动规则，将生成空规则")
  }
  ```

2. 测试：在 `backend/internal/assembly/assembly_test.go` 增加 `TestWarningsSkipRulesForNodeSubscriptions`（`SrSubs`/`GenericSubs` 不输出该警告）与 `TestWarningsKeepRulesForClashAndSrConf`。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test ./...
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - `POST /api/auth/reset/validate` 对 valid/missing/used/expired 返回统一成功包裹 `{status}`；默认限流 10/min，第 11 次 429。
  - Complete 成功后 token 行保留且 `used=1`，二次 Complete 失败。
  - `sr-subs`/`generic-subs` 预览与生成 warnings 不含“未选择任何规则素材池或手动规则”。
  - 面板“速率限制与连接防护”可配置“重置校验（次/分钟）”，默认 10；旧前端未提交该字段时保存不报错。

---

### Step 2：前端 — 可信状态修复

- **目标：** 让装配预览与确认生成强一致；让应急/重置/OIDC 页面呈现正确、可恢复的状态；跨页补前置条件后能回到原任务。
- **前置条件：** Step 1 完成。

#### 2.1 装配预览指纹与过期态

1. `frontend/src/views/admin/AssemblyView.vue`：
   - 新增 `previewFingerprint`（覆盖 targetSyntax、platform/rule、fixed_params、nodes/groups/group orders、overseas、pools、custom_rules、final_direction、overlay；数组与嵌套对象用 stable stringify 规范化）。
   - 新增 `previewedAt` 与 `previewedTargetSyntax` 状态；`doPreview()` 成功后记录 `lastPreviewFingerprint`、`previewedAt`、`previewedTargetSyntax`，并清空 Diff。
   - `doGenerate()` 前置守卫：`previewStale.value || !previewText.value` 时禁止生成并提示“配置已变化，请重新预览”。
   - 本地兜底过滤规则素材警告：`sr-subs`/`generic-subs` 不展示。
2. `frontend/src/views/admin/assembly/PreviewStep.vue`：
   - 新增 `previewStale`、`previewedAt`、`previewedTargetSyntax` props；过期时显示 Alert“配置已变化，请重新预览”，旧正文折叠/半透明展示；最新态显示生成时间与目标类型。
3. `frontend/src/views/admin/assembly/AssemblerShell.vue`：
   - 新增 `canGenerate` prop；“确认生成” `:disabled="!canGenerate"`，禁用时 Tooltip“请先刷新预览”。
4. 测试：更新 `frontend/tests/assembly-view.spec.ts`：
   - preview 成功后生成可用；修改字段/切换类型后 stale 且禁用；`sr-subs` 下后端仍返回规则警告时页面不渲染该条。

#### 2.2 应急/重置/OIDC 状态页

1. `frontend/src/views/EmergencyView.vue`：
   - `onMounted` 调 `system.fetchStatus(true)`；非应急状态显示 `Result status="info"`“当前未处于应急恢复模式”，动作“返回登录/首页”，不渲染操作码表单。
2. `frontend/src/api/auth.ts` 与 `frontend/src/views/ResetView.vue`：
   - 新增 `validateResetToken(token)`；页面按 valid/missing/used/expired 渲染表单或 Result；非 valid 不渲染密码输入；网络失败显示重试。
3. `frontend/src/api/request.ts` 与 `frontend/src/views/OidcCallbackView.vue`：
   - 在 401 拦截器中对 `/auth/oidc/exchange` 或当前路由 `/login/callback` 增加例外：不要抢先 `auth.logout()`/跳转，由回调页自行展示错误；
   - `OidcCallbackView.vue` 失败时在页面内显示 `Result`，读取 `ApiError.message`；动作“重新使用 OIDC 登录 / 使用本地账号 / 联系管理员”；成功路径不变。
4. `frontend/src/views/LoginView.vue`：
   - 建立 `oidc_error` 映射表：`state_mismatch`/`state_expired`/`exchange_failed`/`resolve_failed`/`issue_failed` → 中文；其他后端文本加“OIDC 登录失败：”前缀。
5. 测试：
   - 新增 `reset-view.spec.ts`、`oidc-callback-view.spec.ts`、`emergency-view.spec.ts`；
   - 更新 `request.spec.ts`：`/login/callback` 的 OIDC 交换 401 不再被拦截器抢先跳转登录页。

#### 2.3 装配跨页上下文（ContextBar）

1. 新增 `frontend/src/components/ContextBar.vue`：从 `sessionStorage` 读取 `assembly_ctx_v1`，有值且未过期时在管理内容区顶部渲染来源任务与“返回装配”。
2. `AssemblyView.vue`：preflight 跳转前 `saveAssemblyDraft()`；返回时恢复 tab/subTab/step 与表单草稿，并清除上下文。
3. `AdminLayout.vue`：在 `<RouterView />` 上方统一挂载 `<ContextBar />`。
4. 测试：新增 `frontend/tests/context-bar.spec.ts`。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - 切换装配类型或修改任一产物字段后，旧预览不可作为当前预览，确认生成禁用。
  - 非应急状态 `/emergency` 不出现“正常服务已暂停”和操作码输入。
  - `/reset` 四态正确，非 valid 不渲染密码表单。
  - OIDC 回调失败页面不出现内部 key，恢复动作可用。
  - 从装配跳转补节点/订阅后，一次操作返回原装配类型与步骤。

---

### Step 3：后端 — 概览汇总、版本归属与 `/admin` 应急网关

- **目标：** 为 `/admin` 概览页与版本标题提供稳定的只读后端契约；保证应急模式对 `/admin` 精确路径的前端回退不误拦。
- **前置条件：** Step 0；可与 Step 1/2 并行观察。

#### 3.1 `GET /api/admin/overview`

1. 新增 `backend/internal/server/overview.go`：
   - `OverviewHandler` 依赖现有只读业务服务：`platformSvc`、`subSvc`、`nodeSvc`、`ruleSvc`、`shareSvc`、`poolSvc`、`proxyGroupSvc`、`approvalSvc`、`adminUserSvc`、`accessSvc`、`xraySvc`、`extSvc`，并接收 `mode`。
   - 在 `server.go` 中 **`RegisterLogRoutes` 之后、`registerStatic` 之前** 注册 `GET /api/admin/overview`（会话 + 管理员双中间件）。
2. 新增 `internal/approval/approval.go` 只读方法：

  ```go
  // RecentPending 返回最近 limit 条待审批（created_at 倒序），供概览页动态摘要使用；
  // 不改变审批中心现有列表的正序语义。
  func (s *Service) RecentPending(ctx context.Context, limit int) ([]PendingUser, error)
  ```

3. 响应契约：

  ```json
  {
    "status": { "app_mode": "prod", "advanced_mode": false, "emergency": false },
    "counts": {
      "platforms": 3, "subscriptions": 1, "nodes": 2, "usable_nodes": 2,
      "manual_nodes": 1, "xray_nodes": 0, "rules": 1, "shares": 0, "users": 2,
      "pending_users": 1, "pools": 1, "proxy_groups": 1,
      "xray_instances": 0, "ext_accounts": 0
    },
    "checklist": [
      { "key": "platforms", "done": true, "label": "创建至少一个平台", "action_path": "/admin/platforms", "action_label": "创建平台" },
      { "key": "subscriptions", "done": true, "label": "为平台创建订阅条目", "action_path": "/admin/subscriptions", "action_label": "新建订阅" },
      { "key": "nodes", "done": true, "label": "添加至少一个可用节点", "action_path": "/admin/nodes", "action_label": "新建节点" },
      { "key": "version_active", "done": true, "label": "生成并激活首个版本", "action_path": "/admin/assembly", "action_label": "前往装配" },
      { "key": "member_check", "done": false, "manual": true, "label": "以普通用户身份检查", "action_path": "/", "action_label": "查看用户首页" }
    ],
    "recent": { "pending_users": [], "access_logs": [] }
  }
  ```

   > Checklist 的 key、label、action_path、done 判定以 §0.1.1 定稿为准；后端仅返回结构化数据，最终文案由前端按定稿渲染。


4. 计数与摘要口径：
   - 列表类使用现有服务 `List(ctx)` 的长度（小团队规模可接受）；
   - `users`/`pending_users` 使用 `adminUserSvc.List(ctx, ListQuery{Page:1, Size:1})` 与 `approvalSvc.List(ctx, 1, 1)` 的 `total`；
   - `usable_nodes`：`enabled && !missing && (source == "manual" || allocatable)` 的节点数，checklist `nodes.done` 以 `usable_nodes > 0` 为准，避免停用/缺失节点误判完成；
   - `recent.pending_users` 调用新增 `RecentPending(ctx, 5)`；`recent.access_logs` 调用 `accessSvc.Query(ctx, "", "", 1, 5)`（该方法已按 `created_at DESC` 返回）；
   - `version_active`：任一订阅 `current_version > 0` 即 done；`member_check` 恒为人工步骤；
   - 高级模式关闭时 `xray_instances`/`ext_accounts` 返回 0，保持字段形状稳定。
5. `frontend/src/api/overview.ts` 类型与接口封装放 Step 5；本步只写后端。
6. 测试：
   - `backend/internal/approval/approval_test.go` 新增 `TestRecentPendingDesc`；
   - 新增 `backend/internal/server/overview_test.go`：空数据、有数据、`usable_nodes` 过滤、高级模式开关、`recent` 长度与倒序。

#### 3.2 `GET /api/admin/versions/:id/owner`

1. `backend/internal/version/version.go` 新增 `OwnerByVersionID(ctx, versionID)`：查 `versions` 表返回 `(OwnerType, ownerID)`；不存在返回 `ErrVersionNotFound`。
2. 只读查询方法补齐（均不改变既有业务语义）：
   - `backend/internal/rule/rule.go`：新增 `Get(ctx, id) (*Rule, error)`；
   - `backend/internal/custom/custom.go`：新增 `Get(ctx, id) (*CustomOwnerInfo, error)`，join `users`/`platforms`，返回 `user_id`、`username`、`platform_id`、`platform_name`；
   - subscription 使用既有 `subSvc.Get`；share 使用既有 `shareSvc.Get`。
3. 新增 `backend/internal/server/version_owner.go`：
   - **独立注册** `GET /api/admin/versions/:id/owner`（会话 + 管理员）；不要并入 `AssemblyHandler`，避免把 share/custom 依赖塞进装配 handler。
   - 反查后解析真实名称：`type_label` 分别为“订阅/规则/分享/自定义订阅”；`back_path` 分别为 `/admin/subscriptions`、`/admin/rules`、`/admin/shares`、`/admin/users`；custom 的 `name` 用 `{username} / {platform_name}`。
   - 响应：`{ owner_type, owner_id, name, type_label, back_path }`。
4. 测试：
   - `backend/internal/version/version_test.go` 覆盖 `OwnerByVersionID` 存在/不存在；
   - server 级测试覆盖四类 owner 解析与 404。

#### 3.3 应急网关补 `/admin` 精确路径

1. `backend/internal/server/emergency_gate.go` `isSPAPath` 增加 `case path == "/admin": return true`。
2. 测试：补 `GET /admin` 白名单断言。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test ./...
  ```
- **验收标准：**
  - `/api/admin/overview` 字段稳定，空数据与有数据均符合契约。
  - `/api/admin/versions/:id/owner` 四类资源反查正确。
  - 应急模式 `GET /admin` 不返回 503。

---

### Step 4：前端 — 手机任务可完成性

- **目标：** 390px 下可靠完成主要操作，所有 Modal 表单统一为全屏 Drawer，可交互命中区 ≥44px。
- **前置条件：** Step 2 完成。

1. 新增 `frontend/src/components/FormOverlay.vue`：
   - `matchMedia('(max-width: 767px)')` 判定；桌面渲染 AntD `Modal`，手机渲染全宽全高 `Drawer`（`placement="bottom"`，`height="100dvh"`）。
   - 顶部固定标题 + 关闭按钮（accessible name“关闭{标题}”），底部固定“取消/保存”，padding-bottom 使用 `env(safe-area-inset-bottom)`。
   - 保留 slots：`default`、`footer`；`open`、`title`、`width`、`loading`、`destroy-on-close`。
2. 迁移全部 Modal 表单（ConfirmModal 维持小尺寸居中确认框，不迁移）：
   - `NodesView.vue`：新建/编辑节点、节点显示名；
   - `XrayInstancesView.vue`：实例新增/编辑、独立账号新增/编辑、检测回执如需编辑；
   - `GroupsView.vue`：新建组、编辑组；
   - `ProxyGroupsView.vue`：新建/编辑代理组；
   - `assembly/PoolTab.vue`：新建/编辑素材池；
   - `assembly/PoolDetail.vue`：新增/编辑条目；
   - `SubscriptionsView.vue`：新建/编辑订阅；
   - `SharesView.vue`：创建分享；
   - `RulesView.vue`：创建/编辑规则；
   - `UsersView.vue`：全部表单类 Modal（新建用户、编辑、自定义订阅、重置密码等）；
   - `VersionManageView.vue`：创建版本、编辑版本、预览 Modal 均按 `FormOverlay` 迁移：桌面宽 Modal，手机全屏 Drawer（只读预览也走手机 Drawer）。
   - `AssemblyView.vue`：头部默认确认、节点选择排序等编辑型 Modal。
3. `frontend/src/components/AppHeader.vue`：
   - 手机只保留汉堡、站点名、账户入口；主题切换进账户菜单；
   - 账户按钮命中区 ≥44px；站点 ICON/站点名区域不挤压账户按钮。
4. `frontend/src/layouts/AdminLayout.vue`：
   - 手机 Drawer 顶部显示当前页面名与关闭按钮；底部固定“返回主界面”；全部纯图标项加 Tooltip 与 `aria-label`。
5. 触控与溢出：
   - 全局样式增加 `.touch-target`（`min-width:44px; min-height:44px`）并应用于手机端主操作与列表操作；
   - 320/360/390/430 视口无页面级横向溢出；修复 `UsersView` 当前 400px 溢出问题。
6. 测试：
   - 新增 `frontend/tests/form-overlay.spec.ts`（`matchMedia` 切换桌面/手机渲染 Modal/Drawer）；
   - 更新受影响页面的既有测试，确保 FormOverlay 下表单交互仍可触发。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - 390×844 下节点必填区、账户菜单、装配类型选择、版本查看可完成。
  - 所有 Modal 表单在 <768 走全屏 Drawer，底部操作始终可见。
  - 320–430px 无页面横向溢出；主操作命中区 ≥44px。

---

### Step 5：前端 — 信息架构与结构统一

- **目标：** 统一页面骨架、状态容器、菜单与设置分组，落地 `/admin` 概览页、真实版本标题和高级模式页重组。
- **前置条件：** Step 3 完成（后端契约），Step 4 完成（表单载体稳定）；高级模式静态审计已按 §0.1.3 完成，Step 5 前如有浏览器环境再补 DOM/截图走查作为非阻断增强。

1. 新增通用组件：
   - `PageShell.vue`：唯一 `h1`、描述、ContextBar、StateContainer；
   - `EmptyState.vue`：标题/描述/最多两个动作，限制最小高度；
   - `StateContainer.vue`：加载/错误/空/内容四态；
   - `ResponsiveCollection.vue`：桌面表格 + 手机卡片统一操作收口；
   - `FormSection.vue`：字段分组与帮助文本；
   - `PreviewState.vue`：未生成/生成中/最新/已过期四态。
2. 改造 `PageHeader.vue`：标题改 `h1` 24px；每个页面只允许一个主操作；危险操作不进入主操作位。
3. 改造 `TriStateList.vue` 与空态页面：
   - `TriStateList.vue` 作为 `StateContainer` 的兼容实现，增加 `empty` 插槽；
   - `ApprovalsView.vue` 与 `LogsView.vue` 在 `total === 0` 时不渲染分页；无数据时不渲染批量/清空等动作；
   - 所有管理页逐步替换为 `PageShell` + `StateContainer`，避免只改组件而页面仍把分页/批量写在容器外。
4. `AdminLayout.vue` 菜单分组：
   - 顶部“概览” `/admin`；
   - 分发：订阅、分享、平台、规则；
   - 装配：订阅装配、节点、Xray 实例（高级）；
   - 成员：用户、审批中心、用户组（高级）；
   - 系统：面板配置、日志。
   - 收起侧栏保留分组分隔线，所有图标 Tooltip；`selectedKeys` 兼容 `/admin`。
5. 路由与入口：
   - `frontend/src/router/index.ts` 增加 `{ path:'/admin', component: AdminOverviewView, meta:{layout:'admin', requiresAdmin:true} }`；
   - `AppHeader.vue` 的“管理面板”按钮目标改为 `/admin`。
6. 新增 `frontend/src/api/overview.ts` 与 `frontend/src/views/admin/AdminOverviewView.vue`：
   - 计数网格、快捷入口；`status` 与 `checklist` 保持接口兼容但不在概览页展示；
   - 动态摘要：最近 5 条待审批与最近 5 条访问日志；
   - 每张卡片提供失败重试；空态保留明确下一步。
   - **Checklist 文案与判定按 §0.1.1 定稿执行。**
7. `SettingsView.vue` 重组为六分组：
   - 身份与访问（OIDC/OIDC 规则/本地认证/验证码）；
   - 通知（SMTP）；
   - 外观与内容（站点信息、公告页脚）；
   - 运行与安全（运行模式、高级模式、速率限制、日志级别、调试）；
   - 数据管理（导入导出、备份）；
   - 危险操作（红色独立区，默认折叠）。
   - 每组独立保存，顶部显示“N 个分区有未保存更改”；离开页面仅在确有修改时提示；手机顶部 Select 切换分组。
8. `VersionManageView.vue`：
   - `load()` 后若有版本，调用 `GET /api/admin/versions/:id/owner`（使用第一个版本 ID）获取 `{name,type_label,back_path}`；
   - 标题显示“{name} · 版本管理”与中文类型 Badge；无版本时按 ownerType 回退，如“订阅 #id · 版本管理”，不得把 ownerType 英文当真实名称。
9. 高级模式页面（改造依据 §0.1.3 审计结论）：
   - `XrayInstancesView.vue`：顶部实例状态摘要；实例 Tab 内按“实例列表/节点检测/对账”分层；危险操作进更多菜单。
   - `GroupsView.vue`：页头增加用户组影响说明；编辑按“基础信息/流量/可用节点”分段，手机 Drawer。
10. 测试：
   - 新增 `frontend/tests/admin-overview-view.spec.ts`、`frontend/tests/page-shell.spec.ts`、`frontend/tests/empty-state.spec.ts`；
   - 更新 `settings-view.spec.ts`、`version-manage` 相关测试（当前无独立 spec 则新增）、`xray-instances-view.spec.ts`、`groups-view.spec.ts`。

- **测试与验收命令：**
  ```bash
  cd backend && go test ./...
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - 所有管理页唯一 `h1`；空态均有下一步。
  - 管理菜单分组清晰，`/admin` 概览入口可访问。
  - 概览页只用 `/api/admin/overview` 一个汇总请求；动态摘要渲染正确。
  - 设置六分组可在 10 秒内定位；未保存提示正确。
  - 版本页显示真实资源名和中文类型。

---

### Step 6：前端 — 视觉 Token 与主题统一

- **目标：** AntD 与 Tailwind 由同一 `uiTokens` 驱动，Tailwind 走 CSS 变量、AntD 走具体色值；消除暗色多套表面断层。
- **前置条件：** Step 5 完成。

1. `frontend/src/theme.ts` 定义唯一 Token 源（采用 UIReport2 §6.2 候选值，对比度已按 §0.1.2 预校验通过）：

  ```ts
  export const uiTokens = {
    light: {
      page: '#F1F5F9', surface: '#FFFFFF', surfaceSubtle: '#F8FAFC',
      surfaceRaised: '#FFFFFF', border: '#CBD5E1', borderSubtle: '#E2E8F0',
      text: '#0F172A', textSecondary: '#475569', textTertiary: '#64748B',
      primary: '#2563EB', primaryHover: '#1D4ED8', primarySoft: '#EFF6FF',
      success: '#15803D', successSoft: '#F0FDF4', warning: '#B45309',
      warningSoft: '#FFFBEB', danger: '#B91C1C', dangerSoft: '#FEF2F2',
      info: '#0369A1', infoSoft: '#F0F9FF',
    },
    dark: {
      page: '#0B1120', surface: '#111827', surfaceSubtle: '#172033',
      surfaceRaised: '#1E293B', border: '#334155', borderSubtle: '#273449',
      text: '#F8FAFC', textSecondary: '#CBD5E1', textTertiary: '#94A3B8',
      primary: '#60A5FA', primaryHover: '#93C5FD', primarySoft: '#172554',
      success: '#4ADE80', successSoft: '#052E16', warning: '#FBBF24',
      warningSoft: '#451A03', danger: '#F87171', dangerSoft: '#450A0A',
      info: '#38BDF8', infoSoft: '#082F49',
    },
    radius: { control: 8, card: 12, modal: 14 },
    spacing: { pageDesktop: 24, pageMobile: 16, section: 24, card: 20 },
    type: {
      pageTitle: { size: 24, lineHeight: 32, weight: 650 },
      sectionTitle: { size: 18, lineHeight: 26, weight: 600 },
      body: { size: 14, lineHeight: 22, weight: 400 },
      helper: { size: 13, lineHeight: 20, weight: 400 },
    },
  } as const
  ```

2. `frontend/src/theme.ts` 中 `useTheme()` 改为双通道输出：
   - **AntD 通道：** `antdTheme` computed 使用当前主题的**具体颜色字符串**（`uiTokens[dark ? 'dark' : 'light']`），再喂给 `theme.darkAlgorithm/defaultAlgorithm`；禁止把 `var(--ui-*)` 作为 `colorPrimary` 传入 AntD 算法（TinyColor 会把 CSS 变量解析为黑色并生成错误调色板）。
   - **CSS 变量通道：** watch `dark` 时给 `document.documentElement` 写入 `--ui-page`、`--ui-surface`、`--ui-primary` 等变量，并维持 `dark` class。
   - `style.css` 只保留 `:root` 初始回退值；运行时由 `useTheme` 覆盖为当前主题值，避免 JS 与 CSS 两处维护同一份色值。
3. `frontend/src/style.css` 与 `frontend/tailwind.config.js`：
   - `style.css` 保留 `:root` 初始回退变量与 `.dark` 回退变量；
   - `tailwind.config.js` 在 `theme.extend.colors` 中把 `page/surface/.../primary/danger` 等映射到 `var(--ui-*)`；保留 `darkMode:'class'`。
4. 清理硬编码：
   - `frontend/src/router/index.ts` 顶部进度条 `#1677FF` → `var(--ui-primary)`（inline style 使用 CSS 变量安全）；
   - `frontend/src/components/MarkdownView.vue` `#1677ff` → `var(--ui-primary)`；
   - `frontend/src/views/HomeView.vue` 流量色不要传 CSS 变量给 AntD Progress `strokeColor`，改为从 `uiTokens` 当前主题取具体色值；
   - 全量 `gray-*`/`dark:bg-gray-*` 类名迁移到 token 类。当前实际命中 **40 个 Vue 文件**（以执行时 `grep -Rl "gray-[0-9]" frontend/src | sort` 为准，不手工枚举）。

  | 文件（数量） | 迁移要点 |
  |---|---|
  | `HomeView.vue`（21）、`assembly/PoolDetail.vue`（16）、`XrayInstancesView.vue`（15）等 | 文本/边框/背景统一改为 `text-secondary`、`border-subtle`、`bg-surface` 等 token 类 |
  | `SettingsView.vue`、`NodesView.vue`、`SetupView.vue` 等中低频文件 | 仅替换类名，不改变布局 |
  | 所有 `dark:` 变体 | 由根节点 CSS 变量自动适配，删除重复的 `dark:bg-gray-*`/`dark:text-gray-*` |

5. 排版/圆角/阴影：
   - 页面标题 24/32/650、区块 18/26/600、正文 14/22、辅助 13/20；
   - control 8px、card 12px、modal 14px；
   - 卡片 `0 1px 2px rgb(15 23 42 / 6%)`，浮层 `0 12px 32px rgb(15 23 42 / 12%)`。
6. 焦点与动效：
   - 全局 2px 主色 focus-visible ring；
   - `prefers-reduced-motion` 关闭非必要动画。
7. 测试：
   - 更新 `frontend/tests/theme.spec.ts`：断言 token 值与 dark 类名；
   - 构建后浏览器快照核验 1440/1024/390 浅暗主题核心页面。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run
  ```
- **验收标准：**
  - AntD 与 Tailwind 由同一 `uiTokens` 驱动：Tailwind 走 CSS 变量，AntD 走具体色值。
  - 暗色无纯黑断层，至少两个稳定表面层级。
  - 主色/正文/辅助文字达到 WCAG AA；状态不只靠颜色。
  - 既有页面视觉回归无布局破坏。

---

### Step 7：效率润色、测试补强与文档收口

- **目标：** 完成预览/Diff 增强、反馈统一，补齐自动化测试和文档同步。
- **前置条件：** Step 1~6 完成。

1. `frontend/src/views/admin/assembly/PreviewStep.vue`：
   - 增加行号、搜索、复制、换行切换；
   - 预览与 Diff 采用 Tabs 切换，避免上下重复全文。
2. `frontend/src/components/DiffView.vue`：
   - 新增/删除计数与定位按钮；错误行定位。
3. 状态反馈：
   - 相同 Toast 2 秒去重；
   - 同一时刻只允许一个 Dropdown/浮层，Esc 关闭并回焦触发器；
   - Modal/Drawer 焦点进入与回焦；
   - 中文按钮关闭 AntD auto-insert-space。
4. 测试补强：
   - 更新 `.smoke-test.sh` 与 `.smoke-test-prod.sh`：覆盖 `/admin` 概览、重置三态、应急正常态、装配 stale preview、设置分组、手机 Drawer；
   - 补充/更新上述各 Step 列出的 spec。
5. 文档同步：
   - `Design2-UI.md §1.1`：主色改为 `#2563EB` + 双主题 Token；
   - `Design2-UI.md §2.1`：菜单改为“概览 + 四分组，分组但不折叠”，并增加 `/admin` 路由；
   - `Design2-UI.md §4.7`：设置页六分组规格；
   - `AGENTS.md` 若涉及新的 UI 约束则同步；
   - 更新 `ProdTestList.md`。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm test -- --run
  ```
- **验收标准：**
  - 所有新增逻辑有测试；关键 UI 流程无回归。
  - smoke 用例覆盖本卷 P0/P1 变更。
  - 文档与代码状态一致。

---

## 六、候选/待细化项

| # | 候选 | 结论 | 状态 |
|---|------|------|------|
| 1 | 概览页 checklist 文案与判定 | 定稿为 5 步：平台 → 订阅 → 可用节点 → 生成并激活首个版本 → 人工成员检查；`member_check` 恒为人工步骤 | ✅ 已确认（2026-08-28） |
| 2 | 概览页动态摘要是否加实时日志 | 不加实时日志流，采用静态最近 5 条待审批 + 最近 5 条访问日志 | ✅ 已确认（2026-08-28） |
| 3 | 重置校验接口是否叠加限流 | 新增设置页可配置限流，默认 10/min，旧前端缺省按 10 保存 | ✅ 已确认并实现（2026-08-28，Step 1） |
| 4 | 高级模式页面详细 UI 审计 | 现在先做完整审计；已完成源码/设计静态走查，Step 5 前如有浏览器环境再补 DOM/截图核验 | ✅ 已确认（2026-08-28） |
| 5 | Token 最终对比度 | 候选值已预校验，关键文本/背景组合均达 WCAG AA；按 UIReport2 §6.2 定稿 | ✅ 已确认（2026-08-28） |
| 6 | 手机 Drawer 的动画方向 | 采用底部全屏 Drawer（`placement="bottom"`, `height=100dvh`） | ✅ 已确认（2026-08-28） |

---

## 七、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-28 | 初始版本：汇总 UIReport1/UIReport2、源码审计与用户决策，作为 Build11 当前构建方案 |
| v1.1 | 2026-08-28 | 深度复核前后端源码：补充代码事实、后端影响矩阵、Step 1~7 可执行计划；确认重置三态后端接口、`/admin/overview` 后端接口、版本归属接口、后端警告过滤、全部 Modal 表单 Drawer、UIReport2 Token 与概览页动态摘要 |
| v1.2 | 2026-08-28 | 从头重新核验：修正 OIDC 401 拦截器例外、AntD Token 不得传入 CSS 变量、overview 最近待审批需新增倒序方法、`usable_nodes` 口径、版本归属只读方法补齐、清理测试归属 cron、灰度类实际 40 个文件、空态分页/批量需在页面层条件渲染 |
| v1.3 | 2026-08-28 | 同步用户构建前决策：重置校验限流进入设置页（默认 10/min）、概览动态摘要静态化、Drawer 底部全屏、高级模式审计先做、概览 checklist 与 Token 对比度定稿；完成 Step 1 代码与测试（重置三态、used 保留、装配警告过滤、重置校验限流），用户已确认 Step 1 通过 |
| v1.4 | 2026-08-28 | 完成 Step 2：装配预览稳定指纹与过期态、节点订阅规则警告前端兜底、应急 / 重置 / OIDC 可恢复状态页、OIDC 401 例外、30 分钟跨页装配草稿与 ContextBar；`npm run build` 与 `npm test -- --run`（81 tests）均通过。 |
| v1.5 | 2026-08-28 | 完成 Step 3：新增管理员概览汇总与四类版本归属只读接口、最近待审批倒序查询、`/admin` 应急模式 SPA 白名单；补充空/有数据、可用节点过滤、高级模式、动态摘要、四类归属和白名单测试；`go build ./...`、`go vet ./...`、`go test ./...` 均通过。 |
| v1.6 | 2026-08-28 | 完成 Step 4：新增 FormOverlay，桌面保持 Modal、<768 改为底部全屏 Drawer；迁移节点、Xray、用户组、代理组、素材池、订阅、分享、规则、用户、版本与装配表单；移动顶栏/管理导航、44px 命中区与窄视口溢出收口；`npm run build` 与 `npm test -- --run`（85 tests）均通过，320×844 本地入口走查无横向溢出。 |
| v1.7 | 2026-08-28 | 完成 Step 5：新增 PageShell/StateContainer/EmptyState/ResponsiveCollection/FormSection/PreviewState 等通用组件，管理菜单按“概览+分发/装配/成员/系统”分组，设置页重组为六分组并支持未保存提示，落地 `/admin` 概览页与 `/api/admin/overview` 前端契约，版本页改为真实资源名与中文类型，高级模式页（Xray/用户组）做摘要、分段与危险操作收口；`npm run build` 与 `npm test -- --run`（92 tests）均通过。 |
| v1.8 | 2026-08-28 | 完成 Step 6：新增 `uiTokens` 双主题唯一色值源，AntD 使用具体色值、Tailwind 映射 `--ui-*` CSS 变量；更新 `style.css` 初始回退变量、焦点 ring、reduced-motion、卡片/浮层阴影；进度条与 Markdown 主色改为变量；HomeView 流量色取当前主题具体色；补充 Token 与 CSS 变量单测。 |
| v1.9 | 2026-08-28 | 完成 Step 7：PreviewStep 增加行号/搜索/复制/换行/Tabs 切换，DiffView 增加新增/删除计数与定位、错误行定位；Toast 2 秒去重；FormOverlay 焦点管理；关闭 AntD 中文按钮自动空格；smoke 覆盖概览/重置校验/应急正常态；新增 notify 去重单测；同步 Design2-UI、AGENTS、ProdTestList 与 Build11 文档。 |
| v2.0 | 2026-08-28 | 全量核验记录：发现 `/api/admin/overview` 在高级模式+独立账号场景下因 `ExtService.ListExt` 的 SQLite 单连接嵌套查询导致死锁；同步发现 `CheckAllExtQuota`、`writeQuotaExceededError` 同类风险。本次仅记录与研究，未修改任何代码。 |
| v2.1 | 2026-08-28 | 完成 Issue9 R24-01：`ListExt`、`CheckAllExtQuota` 落地 rows-close-first，`writeQuotaExceededError` 改为单条批量更新；新增三个非空数据回归测试，既有管理员概览高级模式回归恢复通过；后端 `go build ./...`、`go vet ./...`、`go test -timeout 180s ./...` 全部通过。 |
| v2.2 | 2026-08-30 | 完成 Issue9 R24-05 第一阶段：管理员概览移除「服务状态」与「首次发布清单」面板，保留 `/api/admin/overview` 的 `status`/`checklist` 契约及后端聚合；概览聚焦资源计数与近期管理活动。 |
| v2.3 | 2026-08-30 | 完成 Issue9 R24-06：管理员概览刷新按钮采用局部 flex 垂直居中，修正 Reload 图标与文字的基线偏移；不改变其他页面的 AntD 按钮布局。 |
