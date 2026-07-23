# Achieved Build Plan 1 — VPN Subscription Management

> **生成日期**: 2026-07-23
> **状态**: 三阶段全部完成，文档整合归档。

---

## 目录

1. [项目概览](#一项目概览)
2. [Phase 1 — 核心功能构建](#二phase-1--核心功能构建)
3. [Phase 2 — Tailwind CSS v3 UI 迁移 + 单容器架构](#三phase-2--tailwind-css-v3-ui-迁移--单容器架构)
4. [Phase 2 Mobile — 管理面板移动端适配](#四phase-2-mobile--管理面板移动端适配)
5. [技术架构总结](#五技术架构总结)
6. [待办/延期项](#六待办延期项)
7. [关键设计决策汇总](#七关键设计决策汇总)

---

## 一、项目概览

自托管的 VPN 订阅管理系统，面向 ≤10 人小团队。管理员通过 Web UI 配置 OIDC 认证、管理用户、上传订阅配置文件（Clash/V2Ray/Shadowrocket 格式）和分流规则。

**技术栈**:
| 层级 | 技术 |
|------|------|
| 后端 | Go + Gin + zerolog + SQLite (`modernc.org/sqlite`) |
| 前端 | Vue 3 (Composition API) + Vite + Tailwind CSS v3 + Element Plus (仅 5 组件) |
| 认证 | OIDC PKCE + JWT (HS256, 7 天) |
| 部署 | Docker 单容器 (Go serve 一切) + 外部 NGINX TLS 终止 |

**架构**:
```
用户 → HTTPS → 外部 NGINX (TLS) → http://127.0.0.1:8080 → Go 单容器
                                              ├─ /api/v1/*  → Gin API
                                              ├─ /assets/*  → Vite 静态资源
                                              └─ /*         → index.html (SPA)
```

---

## 二、Phase 1 — 核心功能构建

> **状态**: ✅ 全部 42 个子块完成 (2026-07)
> **来源**: `BUILD_PLAN.md`

### 2.1 构建范围

| 块 | 内容 | 子块数 | 状态 |
|----|------|--------|------|
| 块 1 | 后端骨架 (工程初始化 + 数据模型 + 数据访问层 + HTTP 基础设施) | 4 | ✅ |
| 块 2 | OIDC 认证 + Setup 流程 + 首位管理员 | 2 | ✅ |
| 块 3 | 后端核心业务 (平台/用户/订阅/规则/自定义订阅/分享订阅/系统配置) | 6 | ✅ |
| 块 4 | 下载端点 + 日志 + 速率限制 | 4 | ✅ |
| 块 5 | 前端脚手架 + 核心基础设施 + 公共组件 | 3 | ✅ |
| 块 6 | 前端认证入口页 + 管理面板布局 + 首页 + 规则浏览页 | 4 | ✅ |
| 块 7 | 前端管理页面 (10 个功能页面) | 7 | ✅ |
| 块 8 | Docker 化 + 联调验证 | 5 | ✅ |
| 块 9 | 测试验证 | 7 | 5/7 ✅ |

### 2.2 数据库 (12 张表)

| 表名 | 用途 |
|------|------|
| `system_config` | key-value 存储 (OIDC 配置、JWT_SECRET、速率限制、公告等) |
| `users` | 用户 (username, email, role, is_advanced, groups) |
| `platforms` | VPN 客户端平台 (client_schemes, download_url) |
| `subscriptions` | 订阅 (platform + type 唯一, versions JSON) |
| `rules` | 分流规则 (client_type, versions JSON) |
| `access_logs` | 访问日志 (90 天自动清理) |
| `oidc_state` | OIDC PKCE state (10 分钟 TTL) |
| `download_tokens` | 用户下载 Token (user + platform + type 复用) |
| `custom_subscriptions` | 用户自定义订阅 (user + platform 唯一) |
| `share_subscriptions` | 独立分享订阅 |
| `share_tokens` | 分享订阅下载 Token |
| `rule_tokens` | 规则下载 Token |

### 2.3 API 端点 (共 50+)

**公开** (无认证): `/health`, `/system/status`, `/platforms`, `/rules`, `/rules/:id/download`, `/system/announcement`

**OIDC 认证**: `/auth/login`, `/auth/callback`, `/auth/me`

**用户**: `/user/platforms`, `/user/update-time`, `/user/refresh-token`

**订阅下载**: `/subscriptions/:platform/download`, `/subscriptions/:platform/download-token`

**分享订阅下载**: `/share/:id/download`

**管理员** (需 AdminRequired):
- 用户管理: `/admin/users/*` + 自定义订阅 + Token 吊销
- 订阅管理: `/admin/subscriptions/*` + 版本管理
- 分享订阅: `/admin/share/*` + Token 刷新/吊销
- 平台管理: `/admin/platforms/*`
- 规则管理: `/admin/rules/*` + Token 轮替
- 系统配置: `/admin/oidc-config`, `/admin/system/*`
- 日志: `/admin/logs`

### 2.4 版本文件存储

```
data/
├── vpn.db                          SQLite 数据库
├── subscriptions/{id}/             v1.conf, v2.conf, ... + current.conf (软链接)
├── rules/{id}/                     同上
├── custom/{user_id}/{platform}/    同上
└── shares/{id}/                    同上
```

- 每个业务域最多保留 5 个历史版本
- `current` 软链接原子切换 (`current.new` → `rename`)
- 不可删除最后一个版本

### 2.5 未完成的测试项

| 编号 | 内容 | 原因 |
|------|------|------|
| 9F | 联机功能测试 (需 OIDC) | 需要真实的 OIDC 提供商配置 |
| 9G | 端到端完整场景 | 依赖 9F |

---

## 三、Phase 2 — Tailwind CSS v3 UI 迁移 + 单容器架构

> **状态**: ✅ 全部 10 个块完成 (2026-07-22)
> **来源**: `BUILD_PLAN_PHASE2.md`

### 3.1 构建范围

| 块 | 内容 | 状态 |
|----|------|------|
| 10A | Tailwind CSS 环境搭建 | ✅ |
| 10B | 全局样式、暗色模式、Toast 系统、preflight 冲突实测 | ✅ |
| 10C | 抽取 `UploadTabs.vue` 可复用组件 | ✅ |
| 10D | 公共组件重写 (ConfirmDialog/OIDCSwitchDialog/UploadModal) | ✅ |
| 10E | 用户端页面 (Setup/Login/Home/Rules) | ✅ |
| 10F | 管理布局 (Manage.vue) | ✅ |
| 10G | 管理页面批次 1 (SubList/SubVersions/ShareList/ShareVersions/PlatformManage) | ✅ |
| 10H | 管理页面批次 2 (UserManage/RulesManage/RuleVersions/OIDCConfig/Logs) | ✅ |
| 10I | 单容器化 (根 Dockerfile + docker-compose + 后端 SPA fallback) | ✅ |
| 10J | 收尾: EP CSS 缩减 + 全量回归验证 | ✅ |

### 3.2 关键架构变更

| 项目 | Phase 1 | Phase 2 |
|------|---------|---------|
| 容器数 | 2 (backend + frontend nginx) | **1** (Go serve 一切) |
| 端口 | `127.0.0.1:8080` + `127.0.0.1:8081` | **仅** `127.0.0.1:8080` |
| 根路由 `/` | 返回 JSON | **返回 index.html** (SPA) |
| Element Plus | 全量 (~1.1MB CSS) | **仅 5 组件** (table/dialog/form/upload/menu) |
| 组件库 | EP 全部 | **Tailwind 原生** + 5 EP 组件 |
| 通知 | ElMessage | **自建 useToast()** |
| 图标 | @element-plus/icons-vue | **内联 SVG** |
| Dockerfile | 2 个 (backend/+frontend/) | **1 个** (repo 根, 多阶段) |

### 3.3 Element Plus 组件策略

| 状态 | 组件 | 原因 |
|------|------|------|
| ✅ 保留 | `el-table`, `el-dialog`, `el-form`, `el-upload`, `el-menu` | 交互逻辑复杂 |
| ❌ 移除 | `el-button`, `el-input`, `el-tag`, `el-card`, `el-empty`, `el-switch`, `el-select`, `el-date-picker`, `el-tabs`, `el-row/col`, `el-input-number`, `el-aside/main`, `el-config-provider`, `v-loading`, `el-icon`, `el-tooltip`, `ElMessage`, `@element-plus/icons-vue` | Tailwind 原生替代 |

### 3.4 新增/修改的关键文件

**新增**:
- `frontend/src/assets/tailwind.css`
- `frontend/tailwind.config.js`
- `frontend/postcss.config.js`
- `frontend/src/composables/useToast.js`
- `frontend/src/components/UploadTabs.vue`
- `Dockerfile` (repo 根, 多阶段构建)

**修改**:
- `frontend/src/main.js` — EP CSS 缩减为仅 5 组件
- `backend/internal/router/router.go` — SPA fallback + 删除根路由 JSON handler
- `docker-compose.yml` — 双服务 → 单服务
- `deploy/nginx-example.conf` — 双 location → 单 location

**保留备用** (不再用于部署):
- `backend/Dockerfile`
- `frontend/Dockerfile`
- `frontend/nginx.conf`

---

## 四、Phase 2 Mobile — 管理面板移动端适配

> **状态**: ✅ 全部 4 个 Phase 完成 (2026-07-23)
> **来源**: `BUILD_PLAN_PHASE2_MOBILE.md`

### 4.1 构建范围

| Phase | 内容 | 涉及文件 | 状态 |
|-------|------|---------|------|
| 2.1 | 移动端表格 UX 改进 (ActionMenu + 列隐藏) | 12 | ✅ |
| 2.2 | 卡片化重构 + Dialog z-index 修复 | 14 | ✅ |
| 2.3 | Dialog 移动端宽度自适应 | 13 | ✅ |
| 2.4 | 全项目 UI/UX 体验修复 | 10 + NotFound.vue | ✅ |
| 2.5 | 导航入口 + 表单 ID 清理 + 规则页风格 + 公告栏 | 9 (含后端 4) | ✅ |
| 2.6 | 全局 UI 比例调整 | 15 | ✅ |

### 4.2 Phase 2.1 — 移动端表格 UX 改进

**新增组件**:
- `ActionMenu.vue` — 桌面端显示全部按钮，移动端折叠为三点下拉菜单
- `useIsMobile.js` — 响应式断点检测 (768px)

**改动**: 11 个页面接入 ActionMenu/列隐藏

### 4.3 Phase 2.2 — 卡片化重构

**改动**:
- 5 个列表页 (SubList/ShareList/UserManage/RulesManage/PlatformManage) 从 `el-table` 改为卡片 grid
- 全部 `el-dialog` 加 `append-to-body` (解决 z-index 层叠冲突)
- 仅 4 个表格保留页 (Logs/SubVersions/ShareVersions/RuleVersions) 保留 `el-table`

**页面布局现状**:
| 布局 | 页面 |
|------|------|
| 卡片 grid | SubList, ShareList, UserManage, RulesManage, PlatformManage |
| el-table | Logs, SubVersions, ShareVersions, RuleVersions |
| 已有卡片 | OIDCConfig |
| 不受影响 | Home, Login, Setup, Rules(用户), Manage(布局壳) |

### 4.4 Phase 2.3 — Dialog 移动端宽度自适应

**策略 A — useDialogWidth.js**: 10 处普通弹窗，桌面端保持原 px 宽度，移动端 90% 视口宽度

**策略 B — fullscreen 全屏**: 4 处 (UploadModal + 3 个版本预览页)，移动端全屏展示

**策略 C — PlatformManage 纵向滚动**: 移动端弹窗 body 加 `max-h-[calc(100vh-200px)] overflow-y-auto`

**策略 D — 预览 `<pre>` 高度适配**: 移动端全屏时 `max-h-[calc(100vh-120px)]`，桌面端 `max-h-96`

### 4.5 Phase 2.4 — UI/UX 体验修复

| # | 修复项 | 文件 |
|---|--------|------|
| 1+8 | Rules.vue 卡片化 + 错误提示 | Rules.vue |
| 2+9 | Toast 动画 (TransitionGroup) + 上限 (MAX_TOASTS=5) | useToast.js, App.vue |
| 3 | Login 防重复点击 (loggingIn ref) | Login.vue |
| 4 | OIDCConfig 保存/测试按钮 spinner | OIDCConfig.vue |
| 5 | UserManage toggle ARIA 无障碍 | UserManage.vue |
| 6 | 路由切换全局 loading 进度条 | router/index.js, App.vue |
| 7 | 404 页面 | NotFound.vue (新), router/index.js |
| 11 | ShareList 加载态修复 | ShareList.vue |
| — | 4 个表格页补 `overflow-x-auto` 包裹 | Logs, SubVersions, ShareVersions, RuleVersions |

### 4.6 Phase 2.5 — 导航入口 + 表单 ID 清理 + 规则页 + 公告栏

| # | 内容 | 涉及文件 |
|---|------|---------|
| 1A | Home 页「分流规则」卡片 | Home.vue |
| 1B | 公告栏系统 (前后端完整) | handlers.go, router.go, system_service.go, OIDCConfig.vue, Home.vue |
| 2 | 后端 ID 自动生成 + 前端移除 ID 输入 | rule_service.go, platform_service.go, handlers.go, RulesManage.vue, PlatformManage.vue |
| 3 | Rules.vue 风格统一 (← 首页 返回按钮) | Rules.vue |

### 4.7 Phase 2.6 — 全局 UI 比例调整

| 层级 | 改动 | 涉及文件 | 状态 |
|------|------|---------|------|
| 16A | 按钮 `text-xs` → `text-sm` | 10 views | ✅ |
| 16B | 卡片标题 `text-sm` → `text-base` | 5 views | ✅ |
| 16C | 输入框 `text-sm` → `text-base` | 9 views (35 处) | ✅ |

### 4.8 实施中修复的 Bug

| 严重度 | 位置 | 描述 | 修复 |
|--------|------|------|------|
| 🔴 | Home.vue | 平台卡片 grid 被错误绑定为公告栏卡片的 `v-else` | `v-else` → 独立 `v-if` |
| 🔴 | Home.vue | `fetchAnnouncement()` 未导入 `publicApi` | import 新增 `publicApi` |
| 🟡 | App.vue | 路由进度条 `beforeResolve()` 触发太晚 | 改为 `beforeEach()` |
| 🟡 | Rules.vue | 卡片化后遗留旧代码导致编译失败 | 清理残留代码 |
| 🟡 | 版本表格 | 操作列 `width="80"` 太窄导致按钮截断 | 改为动态 `:width="isMobile ? 60 : 220"` |

### 4.9 后续增量修复 (2026-07-23)

| 修复 | 描述 |
|------|------|
| 公告不可关闭 | 移除 Home.vue 公告卡片的 ✕ 按钮和 localStorage 关闭逻辑 |
| 规则管理隐藏 Token | RulesManage.vue 移除 Token 脱敏展示行 |
| 版本表格修复 | 3 个版本页操作列动态宽度 |
| 侧边栏重命名 | "OIDC 配置" → "面板配置" |

---

## 五、技术架构总结

### 5.1 后端

```
backend/
├── cmd/server/main.go             入口 (Setup/Normal 双模式)
├── internal/
│   ├── auth/oidc_service.go       OIDC PKCE + JWT
│   ├── handler/                   HTTP handler (按业务域拆分)
│   ├── service/                   业务逻辑层
│   ├── repository/                数据访问层 (12 个 repo)
│   ├── middleware/                8 个中间件
│   ├── models/types.go           全部结构体
│   ├── router/router.go          双模式路由 + SPA fallback
│   └── utils/                    env, crypto, sanitizePath, isValidID
```

### 5.2 前端

```
frontend/src/
├── router/index.js               16 条路由 + 三重守卫
├── services/api.js               Axios 封装 + 分组 API
├── stores/user.js                Pinia 用户状态
├── composables/
│   ├── useTheme.js               暗色模式
│   ├── useToast.js               Toast 通知
│   ├── useIsMobile.js            响应式断点
│   └── useDialogWidth.js         Dialog 宽度自适应
├── components/
│   ├── ConfirmDialog.vue         确认对话框
│   ├── OIDCSwitchDialog.vue      切换 OIDC 提供商
│   ├── UploadModal.vue           版本上传
│   ├── UploadTabs.vue            文件/文本 Tab 切换
│   └── ActionMenu.vue            响应式操作菜单
└── views/
    ├── Setup.vue                 首次配置
    ├── Login.vue                 登录
    ├── Home.vue                  首页 (最复杂页面, 8 个条件分支)
    ├── Rules.vue                 规则浏览 (卡片 grid)
    ├── Manage.vue                管理面板布局
    ├── SubList.vue               订阅管理 (卡片 grid)
    ├── SubVersions.vue           订阅版本管理 (el-table)
    ├── ShareList.vue             分享订阅 (卡片 grid)
    ├── ShareVersions.vue         分享版本管理 (el-table)
    ├── PlatformManage.vue        平台管理 (卡片 grid)
    ├── UserManage.vue            用户管理 (卡片 grid)
    ├── RulesManage.vue           规则管理 (卡片 grid)
    ├── RuleVersions.vue          规则版本管理 (el-table)
    ├── OIDCConfig.vue            OIDC 配置 + 速率限制 + 公告栏
    ├── Logs.vue                  日志查看 (el-table)
    └── NotFound.vue              404 页面
```

### 5.3 部署

```yaml
# docker-compose.yml (单容器)
services:
  app:
    build: .               # 根目录 Dockerfile (多阶段)
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - vpn-data:/app/data
    restart: unless-stopped
```

**Dockerfile 多阶段构建**: Node (前端构建) → Go (后端编译) → distroless (运行时)

---

## 六、待办/延期项

| 编号 | 内容 | 来源 | 原因 | 优先级 |
|------|------|------|------|--------|
| 9F | 联机功能测试 | Phase 1 | 需要真实 OIDC 提供商 | 🟡 中 |
| 9G | 端到端完整场景 | Phase 1 | 依赖 9F | 🟡 中 |
| 14I | EP 暗色模式实测验证 | Phase 2 Mobile | 纯浏览器手工测试项 | 🟢 低 |

> 注: 16C (输入框字号) 已于 2026-07-23 实施完成，从延期列表中移除。

---

## 七、关键设计决策汇总

| # | 决策 | 结论 |
|---|------|------|
| 1 | 容器架构 | 单容器 (Go serve 一切: API + 静态文件 + SPA) |
| 2 | Element Plus | 仅保留 5 个复杂组件 (table/dialog/form/upload/menu) |
| 3 | 样式框架 | Tailwind CSS v3, 本地打包, 零 CDN |
| 4 | 暗色模式 | Tailwind `dark:` 前缀 + useTheme composable (`.dark` class) |
| 5 | 通知系统 | 自建 useToast() 替代 ElMessage |
| 6 | 图标 | 全部内联 SVG, 已移除 @element-plus/icons-vue |
| 7 | 响应式断点 | 统一 768px (useIsMobile) |
| 8 | 管理面板布局 | 卡片 grid 为主 (5 个列表页), el-table 仅 4 个保留页 |
| 9 | 表单校验 | el-form 保留, 原生 input 通过 @blur 手动触发校验 |
| 10 | 公告栏 | 基于 system_config 的简单 key-value, 不可被用户关闭 |
| 11 | ID 生成 | 后端自动生成 UUID fallback, 前端创建弹窗无需输入 ID |
| 12 | 版本管理 | 最多 5 个版本, current 软链接原子切换 |
| 13 | 根路由 `/` | 返回 index.html (SPA), `/health` 保留 JSON 健康检查 |
| 14 | 数据库 | SQLite (`modernc.org/sqlite`), WAL 模式, 单连接 |
| 15 | JWT | HS256, 7 天有效期, claims 仅 user_id + exp/iat |
| 16 | 权限 | 实时查库, 不缓存, role/is_advanced 不入 JWT claims |

---

> **归档说明**: 本文档整合了 Phase 1 (核心功能)、Phase 2 (Tailwind UI 迁移 + 单容器) 和 Phase 2 Mobile (移动端适配) 三份构建计划的最终成果。原始文档 (`BUILD_PLAN.md`, `BUILD_PLAN_PHASE2.md`, `BUILD_PLAN_PHASE2_MOBILE.md`) 保留备查。
