# 构建计划 — Phase 2（Tailwind CSS v3 UI 迁移）

> **前置条件**: Phase 1 全部 42 个子块已完成（见 `BUILD_PLAN.md`），项目可正常编译、Docker 部署、OIDC 认证全链路跑通。
> **目标**: 用 Tailwind CSS v3 重写所有页面的视觉层，保留 Element Plus 处理复杂交互组件。静态资源（JS/CSS/字体/图片）通过 `/api/v1/public/` 由 Go 后端托管，支持后续 CDN 配置。
> **设计原则**: 性能优先、简约优先。本地打包，零 CDN 运行时依赖。响应式（移动端优先），暗色模式。

---

## 变更概览

| 层 | 新增 | 修改 | 不变 |
|----|------|------|------|
| **前端** | `tailwind.config.js`、`postcss.config.js`、`src/assets/tailwind.css` | `package.json`、`vite.config.js`、`main.js`、`App.vue`、`useTheme.js`、全部 15 个 `.vue` + 3 个组件 | `api.js`、`user.js`、`router/index.js`（业务逻辑完全不改） |
| **后端** | 无新文件 | `router/router.go`（新增 `r.Static`）、`Dockerfile`（多阶段 COPY 前端产物） | 所有 handler/service/repository（API 逻辑完全不改） |
| **部署** | 无新文件 | `docker-compose.yml`（backend build context 改为 repo 根目录）、`deploy/nginx-example.conf`（补充注释） | 外部 NGINX 配置（`/api/*` → backend 已涵盖 `/api/v1/public/*`） |

---

## 块划分

| 块 | 内容 | 依赖 | 预计工作量 |
|----|------|------|-----------|
| 块 10A | Tailwind CSS 环境搭建 | 无 | 小 |
| 块 10B | 全局样式与暗色模式对接 | 块 10A | 小 |
| 块 10C | 公共组件 Tailwind 化 | 块 10B | 中 |
| 块 10D | 用户端页面（Home + Rules） | 块 10C | 大 |
| 块 10E | 认证页面（Setup + Login） | 块 10B | 中 |
| 块 10F | 管理布局（Manage.vue） | 块 10B | 中 |
| 块 10G | 管理页面批次 1（SubList/SubVersions/ShareList/ShareVersions/PlatformManage） | 块 10F | 大 |
| 块 10H | 管理页面批次 2（UserManage/RulesManage/RuleVersions/OIDCConfig/Logs） | 块 10F | 大 |
| 块 10I | 后端静态资源托管 + Docker 更新 | 块 10D~10H 任一完成即可 | 中 |
| 块 10J | 部署验证 | 块 10I | 小 |

---

## 块 10A：Tailwind CSS 环境搭建

**目标**: 安装 Tailwind CSS v3 及相关依赖，创建配置文件，确保 Vite 构建正常。

**任务**:

- [ ] 安装依赖：
  ```bash
  cd frontend
  npm install -D tailwindcss@3 postcss autoprefixer
  ```
- [ ] 创建 `frontend/tailwind.config.js`：
  ```js
  /** @type {import('tailwindcss').Config} */
  export default {
    content: [
      "./index.html",
      "./src/**/*.{vue,js,ts,jsx,tsx}",
    ],
    darkMode: 'class',  // 与 useTheme.js 的 .dark class 联动
    theme: {
      extend: {},
    },
    plugins: [],
  }
  ```
- [ ] 创建 `frontend/postcss.config.js`：
  ```js
  export default {
    plugins: {
      tailwindcss: {},
      autoprefixer: {},
    },
  }
  ```
- [ ] 创建 `frontend/src/assets/tailwind.css`：
  ```css
  @tailwind base;
  @tailwind components;
  @tailwind utilities;
  ```
- [ ] 修改 `frontend/src/main.js`：在 Element Plus 样式之前引入 Tailwind：
  ```js
  import '@/assets/tailwind.css'
  import 'element-plus/dist/index.css'
  import 'element-plus/theme-chalk/dark/css-vars.css'
  ```
  （Tailwind 在前，Element Plus 在后，确保 Element Plus 的组件样式可覆盖 Tailwind base 的 reset）
- [ ] 修改 `frontend/vite.config.js`：生产构建时设置 `base` 为 `/api/v1/public/`：
  ```js
  export default defineConfig(({ mode }) => ({
    base: mode === 'production' ? '/api/v1/public/' : '/',
    plugins: [vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      }
    },
    server: {
      proxy: {
        '/api': 'http://localhost:8080'
      }
    }
  }))
  ```

**验证**: `npm run build` 通过，生成的 `dist/index.html` 中资源引用前缀为 `/api/v1/public/`。

**涉及文件**: `frontend/package.json`, `frontend/tailwind.config.js` (新), `frontend/postcss.config.js` (新), `frontend/src/assets/tailwind.css` (新), `frontend/src/main.js`, `frontend/vite.config.js`

---

## 块 10B：全局样式与暗色模式对接

**目标**: 更新 `App.vue`、`useTheme.js`、`index.html`，确保 Tailwind 暗色模式与现有机制无缝协作。

**任务**:

- [ ] 更新 `frontend/src/App.vue`：
  - 将 `<el-config-provider>` 包裹在 Tailwind 的暗色 class 上下文中（实际上 `<html class="dark">` 已经由 useTheme.js 设置，无需额外包裹）
  - 确保 `<router-view />` 正常渲染
  - 无需大改，主要是验证 Tailwind class 是否生效
- [ ] 更新 `frontend/src/composables/useTheme.js`：
  - 确认 `document.documentElement.classList.toggle('dark', isDark.value)` 与 Tailwind `darkMode: 'class'` 一致 ✅（当前已正确）
  - 无需修改逻辑，仅添加注释说明与 Tailwind 的协作关系
- [ ] 更新 `frontend/index.html`：
  - 添加 `<meta name="theme-color">` 支持（可选，用于浏览器主题色）
  - 无需其他修改

**验证**: `npm run dev` → 切换暗色模式 → 检查 `<html>` 是否有 `class="dark"` → Tailwind `dark:` class 生效。

**涉及文件**: `frontend/src/App.vue`, `frontend/src/composables/useTheme.js`, `frontend/index.html`

---

## 块 10C：公共组件 Tailwind 化

**目标**: 用 Tailwind 样式重写 3 个公共组件（ConfirmDialog / OIDCSwitchDialog / UploadModal），保留其 Element Plus 交互逻辑。

**任务**:

- [ ] `ConfirmDialog.vue`：
  - 保留 `el-dialog` + `el-button`（交互逻辑）
  - 用 Tailwind class 覆盖弹窗内边距、按钮间距、文字排版
  - 使用 `dark:` 前缀处理暗色模式下的文字/背景色
- [ ] `OIDCSwitchDialog.vue`：
  - 保留 `el-dialog` + `el-radio-group`（交互逻辑）
  - 用 Tailwind class 美化提供商选项布局
- [ ] `UploadModal.vue`：
  - 保留 `el-upload` + `el-input` textarea（交互逻辑）
  - 用 Tailwind class 美化 tabs 切换、上传区域边框、文本编辑区

**设计原则**（适用于所有后续页面）:
- 布局：`flex`/`grid` + `gap-*` 替代 Element Plus 的内联布局
- 间距：`p-*`/`m-*` 统一间距体系
- 颜色：Tailwind 色板替代硬编码颜色
- 响应式：`sm:`/`md:`/`lg:` 前缀处理断点
- 暗色：`dark:` 前缀处理所有颜色反转
- 自定义 CSS 仅用于 Element Plus 组件深层样式覆盖（`<style scoped>` 中 `:deep()` 选择器）

**验证**: `npm run build` 通过；3 个组件在任意页面中使用时显示正常。

**涉及文件**: `frontend/src/components/ConfirmDialog.vue`, `frontend/src/components/OIDCSwitchDialog.vue`, `frontend/src/components/UploadModal.vue`

---

## 块 10D：用户端页面（Home.vue + Rules.vue）

**目标**: 用 Tailwind 重写首页和规则浏览页的视觉层。**Home.vue 是最复杂的页面**，包含响应式卡片网格、多种订阅状态的条件渲染、操作按钮组。

**10D-1: Home.vue**

- [ ] **顶部栏**: 用 Tailwind `flex items-center justify-between` + 响应式 padding 替代现有布局
- [ ] **平台卡片网格**: 用 Tailwind `grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6` 替代 `el-row/el-col`
- [ ] **卡片样式**: 用 Tailwind `bg-white dark:bg-gray-800 rounded-lg shadow-md` 替代 `el-card` 的默认样式
- [ ] **订阅区段**: 用 Tailwind `border-t dark:border-gray-700 pt-4` 区分订阅区段，标签用 Tailwind badge 样式替代 `el-tag`
- [ ] **操作按钮**: 保留 `el-button`（交互逻辑：loading、disabled 状态），用 Tailwind 覆盖其颜色和尺寸
- [ ] **加载/空状态**: 用 Tailwind 条件渲染 + 自定义 spinner 替代 `v-loading` + `el-empty`
- [ ] **响应式**: 所有间距、字号使用 Tailwind 响应式前缀

**10D-2: Rules.vue**

- [ ] **表格**: 保留 `el-table`（排序、数据绑定逻辑），用 Tailwind 覆盖表格容器样式
- [ ] **空状态**: 同 Home，用 Tailwind 替代 `el-empty`
- [ ] **下载按钮**: 用 Tailwind 样式化的 `<a>` 标签

**验证**: `npm run build` 通过；首页在桌面/平板/手机三种宽度下卡片布局正确；暗色模式切换正常。

**涉及文件**: `frontend/src/views/Home.vue`, `frontend/src/views/Rules.vue`

---

## 块 10E：认证页面（Setup.vue + Login.vue）

**目标**: 用 Tailwind 重写首次配置页和登录页。这两个页面结构简单，主要是表单布局。

**10E-1: Setup.vue**

- [ ] **表单布局**: 用 Tailwind `max-w-lg mx-auto` 居中 + `space-y-*` 间距
- [ ] **输入框**: 保留 `el-input`，用 Tailwind 控制其容器样式
- [ ] **按钮**: 保留 `el-button`，用 Tailwind 覆盖颜色

**10E-2: Login.vue**

- [ ] **居中布局**: 用 Tailwind `flex items-center justify-center min-h-screen`
- [ ] **卡片**: 用 Tailwind 白底圆角阴影卡片替代 Element Plus 默认样式

**验证**: `npm run build` 通过；Setup 表单在移动端宽度下不溢出。

**涉及文件**: `frontend/src/views/Setup.vue`, `frontend/src/views/Login.vue`

---

## 块 10F：管理布局（Manage.vue）

**目标**: 用 Tailwind 重写管理后台的侧边栏布局，**修复移动端响应式**。

**任务**:

- [ ] **整体布局**: 用 Tailwind `flex h-screen` 替代 `el-container/el-aside/el-main`
- [ ] **侧边栏**: 用 Tailwind `w-[200px]` 固定宽度 + `bg-gray-50 dark:bg-gray-900` 背景色
  - 菜单项：保留 `el-menu`（router 模式、高亮逻辑），用 Tailwind 覆盖其样式（背景、文字颜色、激活态渐变紫色）
  - 移动端：用 Tailwind `translate-x-[-200px]` + `transition-transform` 实现滑入滑出，替代现有的 CSS 方案
- [ ] **汉堡按钮**: 用 Tailwind 定位和图标样式
- [ ] **内容区**: 用 Tailwind `flex-1 overflow-auto p-6`

**验证**: `npm run build` 通过；侧边栏在桌面端固定显示、移动端滑出；菜单高亮正确。

**涉及文件**: `frontend/src/views/Manage.vue`

---

## 块 10G：管理页面批次 1

**目标**: 用 Tailwind 重写 5 个管理页面（SubList / SubVersions / ShareList / ShareVersions / PlatformManage）。**ShareList 的创建按钮 bug 在本块彻底修复**。

**10G-1: SubList.vue + SubVersions.vue**

- [ ] **SubList**: 保留 `el-table` + `el-dialog`，用 Tailwind 覆盖容器样式、表格边框、对话框内边距
- [ ] **SubVersions**: 版本列表表格 + 上传区域用 Tailwind 美化
- [ ] **当前版本标识**: 用 Tailwind `bg-green-100 text-green-800` 标签替代 `el-tag type="success"`

**10G-2: ShareList.vue（重点 — 修复创建按钮 bug）**

- [ ] **根因修复**: 移除 `v-loading` 指令，改用 Tailwind 条件渲染：
  ```html
  <!-- 加载中 -->
  <div v-if="loading" class="flex items-center justify-center py-12 text-gray-400">
    <svg class="animate-spin h-5 w-5 mr-2">...</svg>
    加载中...
  </div>
  <!-- 空状态 -->
  <div v-else-if="shares.length === 0" class="text-center py-12 text-gray-400">
    暂无分享订阅，请创建
  </div>
  <!-- 列表 -->
  <el-table v-else ...>
  ```
  Element Plus `v-loading` + `el-empty` 组合不再使用，从根本上绕过渲染 bug。
- [ ] **表格**: 保留 `el-table`，用 Tailwind 覆盖容器
- [ ] **创建对话框**: 保留 `el-dialog` + `el-upload` + `el-input`，用 Tailwind 控制布局

**10G-3: ShareVersions.vue**: 同 SubVersions 模式

**10G-4: PlatformManage.vue**

- [ ] **表格**: 保留 `el-table`
- [ ] **Client Schemes 展示**: 用 Tailwind `flex flex-wrap gap-1` 标签列表

**验证**: `npm run build` 通过；**ShareList 创建按钮可见且可交互**；所有页面的表格、对话框、上传区域在暗色模式下正常显示。

**涉及文件**: `frontend/src/views/SubList.vue`, `frontend/src/views/SubVersions.vue`, `frontend/src/views/ShareList.vue`, `frontend/src/views/ShareVersions.vue`, `frontend/src/views/PlatformManage.vue`

---

## 块 10H：管理页面批次 2

**目标**: 用 Tailwind 重写剩下 5 个管理页面（UserManage / RulesManage / RuleVersions / OIDCConfig / Logs）。

**10H-1: UserManage.vue**

- [ ] **表格**: 保留 `el-table`，用 Tailwind 美化标签（角色/级别/自定义订阅）
- [ ] **编辑对话框**: 保留 `el-dialog` + `el-switch`（is_advanced 开关）
- [ ] **上传自定义订阅对话框**: 保留 `el-select` + `el-upload`

**10H-2: RulesManage.vue + RuleVersions.vue**: 同 SubList/SubVersions 模式

**10H-3: OIDCConfig.vue**

- [ ] **双卡片布局**: 用 Tailwind `grid grid-cols-1 lg:grid-cols-2 gap-6` 替代 `el-card` 布局
- [ ] **表单**: 保留 `el-input` + `el-select`

**10H-4: Logs.vue**

- [ ] **日期选择器**: 保留 `el-date-picker`
- [ ] **表格**: 保留 `el-table`，用 Tailwind 美化状态标签

**验证**: `npm run build` 通过；所有管理页面的表格和对话框正常；暗色模式统一生效。

**涉及文件**: `frontend/src/views/UserManage.vue`, `frontend/src/views/RulesManage.vue`, `frontend/src/views/RuleVersions.vue`, `frontend/src/views/OIDCConfig.vue`, `frontend/src/views/Logs.vue`

---

## 块 10I：后端静态资源托管 + Docker 更新

**目标**: Go 后端新增 `/api/v1/public/` 静态文件服务，更新 Docker 构建流程，使后端镜像自包含前端构建产物。

**10I-1: 后端路由**

- [ ] 修改 `backend/internal/router/router.go`：
  在 `SetupRouter()` 中，`api := r.Group("/api/v1")` 之前添加：
  ```go
  // Serve frontend static assets (JS/CSS/fonts/images) for CDN caching
  // These files are copied from frontend build output during Docker multi-stage build
  r.Static("/api/v1/public", "/app/public")
  ```
- [ ] 确保 `/app/public` 目录不存在时不会导致 panic（Gin 的 `Static()` 在目录不存在时会静默失败，仅不匹配该路由）

**10I-2: 后端 Dockerfile**

- [ ] 修改 `backend/Dockerfile`，增加前端构建阶段，将 `dist/` 复制到 `/app/public/`：
  ```dockerfile
  # Stage 1: Build frontend assets
  FROM node:22-alpine AS frontend-builder
  WORKDIR /app
  COPY frontend/package.json frontend/package-lock.json ./
  RUN npm ci
  COPY frontend/ .
  RUN npm run build
  # Output: /app/dist/

  # Stage 2: Build Go binary
  FROM golang:alpine AS backend-builder
  WORKDIR /app
  COPY backend/go.mod backend/go.sum ./
  RUN go mod download
  COPY backend/ .
  RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

  # Stage 3: Runtime
  FROM gcr.io/distroless/static-debian12
  COPY --from=backend-builder /app/server /server
  COPY --from=frontend-builder /app/dist /app/public
  ENV DATA_DIR=/app/data
  ENV PORT=8080
  EXPOSE 8080
  ENTRYPOINT ["/server"]
  ```

**10I-3: Docker Compose**

- [ ] 修改 `docker-compose.yml` 中 backend 服务的 build context：
  ```yaml
  backend:
    build:
      context: .              # 改为 repo 根目录，使 Dockerfile 能访问 frontend/
      dockerfile: backend/Dockerfile
    container_name: vpn-backend
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - vpn-data:/app/data
    restart: unless-stopped
  ```
  frontend 服务 `context: ./frontend` 保持不变。

**10I-4: 外部 NGINX 参考配置**

- [ ] 更新 `deploy/nginx-example.conf` 注释，说明 `/api/v1/public/*` 已由后端处理，CDN 可配置缓存：
  ```nginx
  # /api/v1/public/* → 静态资源（JS/CSS/字体/图片），可配置 CDN 长期缓存
  # /api/v1/*        → Gin API
  location /api/ {
      proxy_pass http://127.0.0.1:8080;
      ...
  }
  ```
  location 块本身**无需修改**（`/api/` 已覆盖 `/api/v1/public/`）。

**验证**: 
- `docker compose build --no-cache` 两个镜像构建成功
- `docker compose up -d` 启动后：
  - `curl http://127.0.0.1:8081/` → 返回 `index.html`
  - `index.html` 中的 `<script src="/api/v1/public/assets/...">` → 浏览器请求 → 外部 NGINX → backend:8080 → 返回 JS 文件
- 前端页面在浏览器中正常加载

**涉及文件**: `backend/internal/router/router.go`, `backend/Dockerfile`, `docker-compose.yml`, `deploy/nginx-example.conf`

---

## 块 10J：部署验证

**目标**: 端到端验证 Phase 2 全部变更。

**任务**:

- [ ] `npm run build` 通过（开发环境验证 Tailwind 编译 + Vite 构建）
- [ ] `go build ./...` 通过（后端编译，含新增 `r.Static` 路由）
- [ ] `docker compose build --no-cache` 两个镜像构建成功（验证多阶段构建 + 静态资源复制）
- [ ] `docker compose up -d` 启动后：
  - [ ] `/health` → 200
  - [ ] `/api/v1/system/status` → `{"configured":false}`
  - [ ] 前端 `index.html` 正常返回
  - [ ] 浏览器访问首页 → JS/CSS 正常加载（通过 `/api/v1/public/` 路径）
  - [ ] 暗色模式切换正常
  - [ ] 手机宽度下页面布局正常
- [ ] **回归验证**（业务逻辑不受影响）：
  - [ ] ShareList 创建按钮可见且可交互
  - [ ] 所有 Element Plus 交互组件（表格排序、表单校验、文件上传、弹窗开关）正常工作
  - [ ] 所有 API 调用正常（登录、下载、管理操作）

**涉及文件**: 无新增，使用块 10A~10I 产物

---

## 验收标准

Phase 2 完成后应满足：
1. `go build ./...` 和 `npm run build` 均通过
2. `docker compose build` 两个镜像构建成功
3. `docker compose up -d` 启动正常
4. 前端 JS/CSS 通过 `/api/v1/public/` 路径加载（浏览器 Network 面板确认）
5. ShareList 创建按钮可见且可交互（**Phase 1 bug 彻底修复**）
6. 所有 15 个页面在桌面端和移动端布局正常
7. 暗色模式在所有页面生效
8. 所有现有业务逻辑不受影响（API 调用、Element Plus 交互组件、文件上传、Token 刷新等）
9. 外部 NGINX 配置无需修改（`/api/` location 已涵盖 `/api/v1/public/`）
10. 静态资源文件路径带 content hash（`index-abc123.js`），天然支持 CDN 长期缓存

---

## 与 Phase 1 的关键差异

| 项目 | Phase 1 | Phase 2 |
|------|---------|---------|
| Vite `base` | `/`（默认） | 开发 `/`，生产 `/api/v1/public/` |
| 静态资源托管 | 前端 nginx 容器 | Go 后端 `/api/v1/public/` |
| 前端 nginx | serve 所有静态文件 | 仅 serve `index.html` + SPA 回退 |
| 后端 Dockerfile | 仅编译 Go | 多阶段：前端构建 + Go 编译 + 复制产物 |
| 后端镜像内容 | 仅 Go 二进制 | Go 二进制 + 前端 JS/CSS/字体/图片 |
| 外部 NGINX | `/api/*` → backend, `/*` → frontend | **不变**（`/api/v1/public/*` 已涵盖在 `/api/*` 中）|
