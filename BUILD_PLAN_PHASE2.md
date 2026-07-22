# 构建计划 — Phase 2（Tailwind CSS v3 UI 迁移 + 单容器架构）

> **前置条件**: Phase 1 全部 42 个子块已完成（见 `BUILD_PLAN.md`），项目可正常编译、Docker 部署、OIDC 认证全链路跑通。
> **设计原则**: 性能优先、简约优先。本地打包，零 CDN 运行时依赖。响应式（移动端优先），暗色模式。
> **关键决策**（已与维护者确认，构建时必须遵守，勿擅自更改）:
> 1. **单容器架构** — Go 后端 serve 一切（API + 静态文件 + SPA 回退）。
> 2. **Element Plus 仅保留 5 个复杂组件**: `el-table`、`el-dialog`、`el-form`、`el-upload`、`el-menu`。其余组件全部用 Tailwind 原生替代。
> 3. **不新增后端 API 路由**，完全复用现有路径（仅后端 `router.go` 调整根路由 + 新增 SPA 静态服务）。
> 4. **根路由冲突处理**: 删除 `r.GET("/")` 的 JSON handler（WAF/LB 健康探测改用已存在的 `/health`），根路径 `/` 交给 `r.NoRoute()` 返回 `index.html`。
> 5. **EP CSS 缩减时机**: 延后到末块（10J）。10B–10H 期间保留全量 `element-plus/dist/index.css`，保证中间态可增量验证，避免无样式破损。
> 6. **内联 el-tabs 复用**: 抽取可复用 `UploadTabs.vue` 组件，`UploadModal` + `ShareList`/`RulesManage` 创建对话框共用。
> 7. **Tailwind preflight**: 保留 preflight + 调整引入顺序（Tailwind base 先于 EP CSS）。10B 实测 `el-table`/`el-form`/`el-dialog` 视觉，发现冲突再 scoped 覆盖。
> 8. **ShareList bug 处理**: bug 极其隐蔽无法定位修复，迁移时彻底摒弃 `el-empty` + 条件渲染组合，改用纯 Tailwind div 空状态从根本上绕过。

---

## 一、当前代码事实清单（构建前必须核对）

下列事实基于 Phase 1 完成后的实际代码状态，构建时若发现与代码不符，**先停下核实**，勿基于错误假设改代码。

### 1.1 后端

- `backend/internal/router/router.go` — `SetupRouter()` 同时注册所有路由（Setup/Normal 双模式无重启切换）。当前已注册:
  - `r.GET("/health")` → 返回 `{"status":"ok"}`（容器健康检查，**保留**）
  - `r.GET("/")` → 返回 `{"status":"ok"}`（WAF/LB 探测，**Phase 2 删除**，改由 NoRoute 返回 index.html）
  - `api := r.Group("/api/v1")` — 所有业务路由
- `backend/cmd/server/main.go` — 读取 `PORT`（默认 8080）、`DATA_DIR`（默认 `/app/data`），`SetTrustedProxies(["0.0.0.0/0"])`（Docker 网关 IP 非 127.0.0.1，须信任全部 IPv4 代理以正确解析 X-Forwarded-For）。**Phase 2 无需修改**。
- `backend/go.sum` 存在 ✅（Dockerfile `go mod download` 可用）。

### 1.2 前端（迁移范围实测）

- `frontend/src/main.js` — 当前全量引入 `element-plus/dist/index.css` + `element-plus/theme-chalk/dark/css-vars.css` + `app.use(ElementPlus, { locale: zhCn })`。**注意**: 图标 `@element-plus/icons-vue` **未在 main.js 全局注册**，而是各 `.vue` 文件按需 `import { Plus, UploadFilled } from '@element-plus/icons-vue'`（grep 确认 12 文件）。迁移时按文件删除按需 import 并改内联 SVG，10J 再从 `package.json` 移除依赖；main.js 本身无 icons 注册代码需删。
- `frontend/src/App.vue` — 当前用 `<el-config-provider>`（无 prop）包裹 `<router-view />`。**Phase 2 移除该包裹**（无全局配置需求）。
- `frontend/vite.config.js` — `base: '/'`，`/api` proxy 到 `localhost:8080`。**不修改**。
- **待移除组件实测分布**（grep 确认，16 文件 / 107 处）:
  - `v-loading` 指令: Home, Logs, OIDCConfig, PlatformManage, Rules, RulesManage, RuleVersions, ShareVersions, SubList, SubVersions, UserManage（共 11 文件）
  - `el-empty`: Home, Logs, PlatformManage, Rules, RulesManage, RuleVersions, ShareList, ShareVersions, SubList, SubVersions, UserManage
  - `el-card`: Home, OIDCConfig, Setup
  - `el-row`/`el-col`: Home, OIDCConfig
  - `el-icon` + `@element-plus/icons-vue`: Home, Login, Manage, PlatformManage, RulesManage, RuleVersions, ShareList, ShareVersions, SubList, SubVersions, UserManage, UploadModal
  - `el-tabs`/`el-tab-pane`: UploadModal, ShareList（创建对话框）, RulesManage（创建对话框）
  - `el-select`: SubList, RulesManage, UserManage
  - `el-switch`: UserManage
  - `el-date-picker`: Logs
  - `el-input-number`: OIDCConfig
  - `el-aside`/`el-main`: Manage
  - `el-config-provider`: App.vue
  - `el-tooltip`: Rules（禁用下载按钮的提示，2 处）
- **ShareList.vue 现状**（重要）: 当前**不使用 `v-loading`**，用 `v-if="!loading && shares.length === 0"`/`v-else` + `el-table` + 手动 `loading` ref。但据维护者确认 bug 仍存在且极隐蔽。迁移时**不保留任何 el-empty**，全部改纯 Tailwind div。

### 1.3 部署

- `docker-compose.yml` — 当前 2 服务: `backend`（127.0.0.1:8080）+ `frontend`（nginx, 127.0.0.1:8081）。**Phase 2 合并为单服务 `app`**。
- `frontend/Dockerfile` + `frontend/nginx.conf` — 前端独立容器构建。**Phase 2 不再需要**（保留文件备用，不删除，避免破坏 git 历史；docker-compose 不再引用）。
- `backend/Dockerfile` — 后端独立构建。**Phase 2 不再用于部署**（保留备用），改用 repo 根目录新 Dockerfile。
- `deploy/nginx-example.conf` + `deploy/README.md` — 当前双 location 分流。**Phase 2 简化为单端口反代**。

---

## 二、架构变更

### Phase 1（当前）
```
外部 NGINX
  ├─ /api/* → backend:8080 (Go)
  └─ /*     → frontend:8081 (nginx 静态文件)
```
两个容器: `vpn-backend` + `vpn-frontend`

### Phase 2（目标）
```
外部 NGINX
  └─ /* → app:8080 (Go 单容器)
        ├─ /health            → Gin handler (JSON, 保留)
        ├─ /api/v1/*          → Gin API
        ├─ /assets/*          → Vite 构建静态资源 (r.Static)
        └─ /* (含 /)          → index.html (r.NoRoute, SPA 回退)
```
单个容器: `vpn-app`

### Element Plus 组件策略

| 状态 | 组件 | 原因 |
|------|------|------|
| ✅ **保留** | `el-table`、`el-dialog`、`el-form`、`el-upload`、`el-menu` | 交互逻辑复杂（排序、焦点管理、校验、拖拽、router 高亮），重建成本高 |
| ❌ **移除并用 Tailwind 替代** | `el-button`、`el-input`、`el-tag`、`el-card`、`el-empty`、`el-switch`、`el-select`、`el-date-picker`、`el-tabs`/`el-tab-pane`、`el-row`/`el-col`、`el-input-number`、`el-aside`/`el-main`、`el-config-provider`、`v-loading`、`el-icon`、`el-tooltip`、`ElMessage`、`@element-plus/icons-vue` | 视觉层组件，Tailwind 原生可完全替代 |

---

## 三、块划分（修订版）

> **关键调整 vs 旧计划**:
> - 旧计划 10A 早期缩减 EP CSS → **修订为延后到 10J**（避免中间态破损）。
> - 新增 **块 10C 抽取 `UploadTabs.vue`**（解决三处 el-tabs 重复）。
> - **块 10E 扩展**: 除 Setup/Login 外，纳入 Home.vue + Rules.vue 两个用户端页面（纠正初版遗漏，Home.vue 为全站最复杂页面）。
> - 块 10I 后端改动明确包含**删除 `r.GET("/")`**。
> - 新增 **块 10J 收尾**: EP CSS 缩减 + 全量验证（原 10J 部署验证拆分合并）。

| 块 | 内容 | 依赖 | 预计工作量 |
|----|------|------|-----------|
| 10A | Tailwind CSS 环境搭建（不缩减 EP CSS） | 无 | 小 |
| 10B | 全局样式、暗色模式、Toast 系统、preflight 冲突实测 | 10A | 中 |
| 10C | 抽取 `UploadTabs.vue` 可复用组件 | 10B | 中 |
| 10D | 公共组件重写（ConfirmDialog/OIDCSwitchDialog/UploadModal） | 10C | 中 |
| 10E | 用户端页面（Setup.vue + Login.vue + Home.vue + Rules.vue） | 10B | 大 |
| 10F | 管理布局（Manage.vue） | 10B | 中 |
| 10G | 管理页面批次 1（SubList/SubVersions/ShareList/ShareVersions/PlatformManage） | 10D、10F、10C | 大 |
| 10H | 管理页面批次 2（UserManage/RulesManage/RuleVersions/OIDCConfig/Logs） | 10D、10F、10C | 大 |
| 10I | 单容器化（根 Dockerfile + docker-compose + 后端 SPA fallback + 删除根路由 JSON） | 10D~10H 任一完成即可 | 中 |
| 10J | 收尾: EP CSS 缩减 + 全量回归验证 | 10D~10I 全部完成 | 中 |

**块间依赖关系**:
- 10A → 10B（Tailwind 环境）
- 10B → 10E、10F（基础样式设施；10E 含 Home.vue，为全站最复杂页面）
- 10B → 10C → 10D（UploadTabs 先于用它的 UploadModal）
- 10C、10D、10F → 10G、10H（管理页面依赖公共组件 + 布局 + UploadTabs）
- 10I 可在任一页面迁移完成后并行启动（后端改动独立）
- 10J 必须最后（CSS 缩减需所有页面已不依赖被移除的组件 CSS）

---

## 块 10A：Tailwind CSS 环境搭建

**目标**: 安装 Tailwind v3，创建配置文件，引入 Tailwind 基础样式。**本块不缩减 Element Plus CSS**（延后到 10J）。

**任务**:

- [ ] 安装 Tailwind 依赖（在 `frontend/` 下）:
  ```bash
  cd frontend
  npm install -D tailwindcss@3 postcss autoprefixer
  ```
- [ ] 创建 `frontend/tailwind.config.js`:
  ```js
  /** @type {import('tailwindcss').Config} */
  export default {
    content: ['./index.html', './src/**/*.{vue,js}'],
    darkMode: 'class',
    theme: {
      extend: {
        colors: {
          // 与 Element Plus primary 对齐，便于过渡期混用
          primary: { DEFAULT: '#409eff', dark: '#409eff' },
        },
      },
    },
    plugins: [],
  }
  ```
  - `darkMode: 'class'` 与 `useTheme.js` 的 `.dark` class 联动（AGENTS.md 编码约束）。
  - `content` 必须覆盖 `index.html` + `src/**`，否则 Purge 会误删使用中的 class。
- [ ] 创建 `frontend/postcss.config.js`:
  ```js
  export default {
    plugins: { tailwindcss: {}, autoprefixer: {} },
  }
  ```
- [ ] 创建 `frontend/src/assets/tailwind.css`:
  ```css
  @tailwind base;
  @tailwind components;
  @tailwind utilities;
  ```
- [ ] 修改 `frontend/src/main.js` — **在现有 EP CSS 引入之前**引入 Tailwind（引入顺序决定 preflight 与 EP 基础样式的层叠，见 10B 实测）:
  ```js
  import { createApp } from 'vue'
  import { createPinia } from 'pinia'
  import ElementPlus from 'element-plus'
  import zhCn from 'element-plus/es/locale/lang/zh-cn'

  // 引入顺序: Tailwind base 先于 EP CSS（preflight 在底层，EP 组件样式作用域隔离覆盖）
  import '@/assets/tailwind.css'
  import 'element-plus/dist/index.css'
  import 'element-plus/theme-chalk/dark/css-vars.css'

  import App from './App.vue'
  import router from './router'

  const app = createApp(App)
  app.use(createPinia())
  app.use(router)
  app.use(ElementPlus, { locale: zhCn })
  app.mount('#app')
  ```
  - **仅新增 `import '@/assets/tailwind.css'`**（置于 EP CSS 引入之前），其余保持现状。**切勿引入 `@element-plus/icons-vue` 全局注册**（当前 main.js 无此代码，引入反而与目标相反）。
  - **本块保留全量 EP CSS**（`element-plus/dist/index.css`），10B–10H 期间页面正常工作。
  - **本块不改动 `@element-plus/icons-vue` 按需导入**（图标在各 `.vue` 中按需 import）。10D–10H 迁移各页面到内联 SVG 时逐文件删除 import；10J 从 `package.json` 移除依赖。
- [ ] `vite.config.js` **不修改**（`base: '/'` 保持）。

**验证**:
- `npm run build` 通过。
- 产物中含 Tailwind 生成 CSS（`dist/assets/index-*.css` 中可见 `--tw-` 变量或 utility class）。
- 现有页面视觉无回归（EP CSS 仍在，Tailwind base 仅影响底层重置）。

**涉及文件**: `frontend/package.json`, `frontend/package-lock.json`(自动), `frontend/tailwind.config.js`(新), `frontend/postcss.config.js`(新), `frontend/src/assets/tailwind.css`(新), `frontend/src/main.js`

---

## 块 10B：全局样式、暗色模式、Toast 系统

**目标**: 搭建 Tailwind 暗色模式基础设施，自建 Toast 通知系统替代 `ElMessage`，**实测 preflight 与 EP 组件冲突**。

**任务**:

- [ ] 验证 `useTheme.js` 与 Tailwind `darkMode: 'class'` 联动 — `.dark` class 当前已设置在 `<html>` 上，与 Tailwind `dark:` 前缀匹配 ✅。**`useTheme.js` 本身无需修改**（已切换 `.dark` class + localStorage 持久化）。
- [ ] **preflight 冲突实测**（关键）: 启动 `npm run dev`，逐一检查保留的 5 个 EP 组件在亮/暗模式下的视觉:
  - `el-table` — 表头背景、边框、hover 行是否正常
  - `el-dialog` — 弹窗阴影、遮罩、圆角是否正常
  - `el-form` — label 对齐、input 边框（若仍用 el-input）是否正常
  - `el-upload` — 拖拽区虚线框、文件列表是否正常
  - `el-menu` — 菜单项背景、激活态是否正常
  - 若发现冲突（如 `el-table` 表头背景被 preflight 重置为透明），在对应页面 `<style scoped>` 中用 `:deep()` 针对性恢复，**勿全局禁用 preflight**。
  - 将实测结果与任何必要的 scoped 覆盖记录在 10B 产出中。
- [ ] 创建 `frontend/src/composables/useToast.js` — 自建 Toast 通知系统替代 `ElMessage`:
  ```js
  import { ref } from 'vue'

  const toasts = ref([])
  let id = 0

  export function useToast() {
    function show(message, type = 'info') {
      const toast = { id: ++id, message, type }
      toasts.value.push(toast)
      setTimeout(() => {
        toasts.value = toasts.value.filter(t => t.id !== toast.id)
      }, 3000)
    }
    return {
      toasts,
      success: m => show(m, 'success'),
      error: m => show(m, 'error'),
      info: m => show(m, 'info'),
      warning: m => show(m, 'warning'),
    }
  }
  ```
  - 通知位于页面右下角，3 秒自动消失（AGENTS.md 4.1: "页面底部显示 3 秒自动消失"）。
  - `toasts` 是模块级单例 ref，跨组件共享同一队列。
- [ ] 更新 `frontend/src/App.vue` — 移除 `<el-config-provider>` 包裹（无全局配置需求），添加 Toast 容器:
  ```html
  <template>
    <router-view />
    <!-- Toast 通知容器 (右下角) -->
    <div class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
      <div v-for="t in toasts" :key="t.id"
           class="px-4 py-2 rounded-lg shadow-lg text-white text-sm transition-all"
           :class="toastClass(t.type)">
        {{ t.message }}
      </div>
    </div>
  </template>

  <script setup>
  import { useTheme } from '@/composables/useTheme'
  import { useToast } from '@/composables/useToast'
  const { toasts } = useToast()
  const toastClass = (type) => ({
    success: 'bg-green-600',
    error: 'bg-red-600',
    info: 'bg-gray-700',
    warning: 'bg-yellow-600',
  })[type] || 'bg-gray-700'
  useTheme()
  </script>
  ```
- [ ] 更新 `frontend/index.html` — 添加 `<meta name="theme-color">` 支持浏览器主题色。

**验证**:
- `npm run build` 通过。
- 切换暗色模式 → Tailwind `dark:` class 生效（可临时在某元素加 `dark:bg-gray-900` 验证）。
- 调用 `useToast().success('test')` → 页面右下角出现绿色 Toast，3 秒后消失。
- 5 个保留 EP 组件视觉无回归（preflight 实测通过）。

**涉及文件**: `frontend/src/composables/useToast.js`(新), `frontend/src/App.vue`, `frontend/index.html`, 可能的 `<style scoped>` 覆盖（视实测）

---

## 块 10C：抽取 `UploadTabs.vue` 可复用组件

**目标**: 抽取可复用的"文件上传 / 文本编辑"Tab 切换组件，供 `UploadModal`、`ShareList` 创建对话框、`RulesManage` 创建对话框共用，消除三处 `el-tabs` 重复。

**背景**: 当前 `UploadModal.vue`、`ShareList.vue` 创建对话框、`RulesManage.vue` 创建对话框各自内联 `el-tabs`（文件上传 + 文本编辑两个 tab），逻辑高度重复。ShareList/RulesManage 因需额外字段（名称/客户端类型）未直接复用 UploadModal，但 Tab 部分可统一。

**任务**:

- [ ] 创建 `frontend/src/components/UploadTabs.vue` — 自定义 Tailwind tabs（不使用 `el-tabs`）:
  - **Props**: `modelValue`（当前 tab: 'file' | 'text'）、`accept`（文件类型，默认 `*`）、`maxSize`（MB，默认 50）、`textContent`（文本 tab 的内容，v-model）
  - **Emits**: `update:modelValue`、`update:textContent`、`file-change`（选择文件时触发，传出 File 对象）、`upload`（点击上传按钮，传出 File 或 {content}）、`clear-file`
  - **UI 结构**:
    - Tab 头: 两个按钮 `<button>`，激活态 `bg-primary text-white`，非激活 `bg-gray-100 dark:bg-gray-700`
    - 文件 tab: 拖拽上传区（原生 `<div>` + `@dragover.prevent` + `@drop.prevent`）+ 文件选择 `<input type="file" class="hidden">` + 已选文件名展示 + 清除按钮。**保留 `el-upload`**（AGENTS.md 4.4: 版本上传用 el-upload），或用原生 input + 手动 drag/drop。**决策**: 本组件内部**保留 `el-upload`**（拖拽、文件列表、进度由 EP 处理），仅替换外层 tabs 为 Tailwind。
    - 文本 tab: 原生 `<textarea>` + Tailwind 样式（`w-full h-48 p-3 rounded border ...`）
  - **不包含**业务字段（名称、平台、客户端类型）— 这些由父组件提供，UploadTabs 只负责"文件/文本获取"。
  - **操作按钮归属**（关键）: **`UploadTabs` 不包含任何按钮**（无取消、无上传、无保存）。所有操作按钮统一由**父组件**放在 `el-dialog` 的 `#footer` slot 中。`UploadTabs` 仅通过 `upload` 事件传出 File 或 `{content}`，由父组件决定调 API + loading 态 + 是否关闭对话框。三处调用方的结构统一为：
    ```
    el-dialog
      ├── [父组件的业务字段: 名称/平台/客户端类型]
      ├── UploadTabs（文件/文本切换，无按钮）
      └── #footer（取消 + 上传/保存按钮，由父组件提供）
    ```
    注意当前 [`UploadModal.vue`](frontend/src/components/UploadModal.vue) 的按钮在各自 tab-pane 内部，抽取时须将按钮上移到父组件对话框 `#footer`。
- [ ] 修改 `frontend/src/components/UploadModal.vue` — 内部改用 `UploadTabs`:
  - 移除 `el-tabs`/`el-tab-pane`，改 `<UploadTabs v-model:tab="..." v-model:text="..." @upload="handleUpload" />`
  - 保留外层 `el-dialog`（块 10D 处理）
  - 上传逻辑不变（调用父组件传入的 upload API）

**验证**:
- `npm run build` 通过。
- `UploadTabs` 单独渲染: tab 切换正常，文件选择 + 文本编辑均可用。
- `UploadModal` 调用 `UploadTabs` 后功能与之前一致（上传版本成功）。

**涉及文件**: `frontend/src/components/UploadTabs.vue`(新), `frontend/src/components/UploadModal.vue`

> **注**: `ShareList`/`RulesManage` 的创建对话框改用 `UploadTabs` 在块 10G/10H 完成（依赖此块产出）。

---

## 块 10D：公共组件重写

**目标**: 用 Tailwind 重写 3 个公共组件。`ConfirmDialog`/`OIDCSwitchDialog` 保留 `el-dialog`，`UploadModal` 保留 `el-dialog` + 内部 `el-upload`（经 10C 已用 `UploadTabs`）。

**通用替换规则**（适用于所有块）:
| 移除 | Tailwind 替代 |
|------|---------------|
| `el-button` | `<button class="...">` + 变体 class（见下方"按钮变体清单"） |
| `el-input` | `<input class="...">` |
| `el-tag` | `<span class="rounded-full px-2 py-0.5 text-xs font-medium ...">` |
| `el-empty` | `<div class="text-center py-12 text-gray-400 dark:text-gray-500">...</div>` |
| `v-loading` | `<div v-if="loading" class="flex items-center justify-center py-12"><svg class="animate-spin ..."></svg></div>` |
| `el-icon` + `@element-plus/icons-vue` | 内联 SVG |
| `el-tooltip` | 原生 `title` 属性（仅 Rules.vue 2 处，按钮已 disabled，无需复杂 tooltip） |
| `ElMessage` | `useToast()` |

> **🔴 关键: `el-form` + 原生 input 校验兼容性**（影响 Setup/SubList/PlatformManage/OIDCConfig）:
> Element Plus 的 `el-form` 校验依赖 `el-form-item` 监听子组件的 `blur`/`change` 事件。原生 `<input>` 不会触发这些监听器，导致**实时校验（blur/change）失效**。但 `ref.validate()` 手动全量校验仍正常工作。
> 
> **解决方案**: 在原生 `<input>` 上绑定 `@blur="formRef.validateField('fieldName')"` 手动触发单字段校验。示例:
> ```html
> <el-form ref="formRef" :model="form" :rules="rules">
>   <el-form-item label="名称" prop="name">
>     <input v-model="form.name" @blur="formRef.validateField('name')" class="..." />
>   </el-form-item>
> </el-form>
> ```
> 每个使用 `el-form` + 原生 input 的页面均需添加此 `@blur` 绑定。

**按钮变体清单**（统一全项目）:
- **primary**: `bg-blue-600 hover:bg-blue-700 text-white rounded-md px-4 py-2 text-sm disabled:opacity-50 disabled:cursor-not-allowed`
- **default**: `bg-white dark:bg-gray-700 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-gray-600 rounded-md px-4 py-2 text-sm`
- **warning text (small)**: `text-yellow-600 hover:text-yellow-700 text-xs`
- **danger**: `bg-red-600 hover:bg-red-700 text-white rounded-md px-4 py-2 text-sm`
- **loading 态**: 加 `disabled` + 内联 spinner SVG（`animate-spin`）

**任务**:

- [ ] `ConfirmDialog.vue`:
  - 保留 `el-dialog`（焦点管理、ESC 关闭、遮罩点击）
  - 按钮从 `el-button` 改为 Tailwind 原生 `<button>`（确认用 primary/danger 变体，取消用 default 变体）
  - 弹窗内容区用 Tailwind 排版（标题、提示文字、按钮区右对齐 `flex justify-end gap-2`）
- [ ] `OIDCSwitchDialog.vue`:
  - 保留 `el-dialog`
  - 单选按钮从 `el-radio-group`/`el-radio` 改为原生 `<input type="radio">` + `<label>` + Tailwind 样式
  - 三个提供商选项用 Tailwind 布局（`space-y-3`，每项 `flex items-center gap-2`）
- [ ] `UploadModal.vue`:
  - 保留 `el-dialog`（外层）+ `el-upload`（经 `UploadTabs` 内部）
  - Tab 切换已由 `UploadTabs`（10C）处理
  - 文本编辑区已由 `UploadTabs` 的 `<textarea>` 处理
  - 底部按钮用 Tailwind 原生

**验证**:
- `npm run build` 通过。
- 三个组件在父页面中交互正常: ConfirmDialog 弹出/确认/取消；OIDCSwitchDialog 切换提供商；UploadModal 上传文件 + 文本编辑。

**涉及文件**: `frontend/src/components/ConfirmDialog.vue`, `frontend/src/components/OIDCSwitchDialog.vue`, `frontend/src/components/UploadModal.vue`

---

## 块 10E：用户端页面（Setup.vue + Login.vue + Home.vue + Rules.vue）

**目标**: Tailwind 重写所有非管理端页面。Setup/Login 为认证页面，Home 为首页（最复杂页面，涉及 5 种订阅显示分支），Rules 为规则浏览页。

**10E-1: Setup.vue**

- [ ] **表单布局**: `max-w-lg mx-auto py-8 px-4` 居中 + `space-y-4`
- [ ] **外层卡片**: `el-card` → `<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6">`
- [ ] **输入框**: `el-input` → 原生 `<input>` + Tailwind 样式:
  `class="w-full rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700 px-3 py-2 text-sm focus:ring-2 focus:ring-blue-500 focus:border-blue-500 outline-none"`
- [ ] **下拉选择**: 提供商选择 `el-select` → 原生 `<select>` + Tailwind 样式
- [ ] **按钮**: 原生 `<button>`（primary 变体 + loading 态: `disabled` + spinner SVG）
- [ ] **表单校验**: 保留 `el-form`（校验规则和错误提示）。`el-form-item` 内的 `el-input` 改为原生 `<input>`，**每个原生 input 需加 `@blur="formRef.validateField('fieldName')"`** 以保持实时校验（详见 10D 关键提示）。`el-form` 的 `:model` + `:rules` + `ref.validate()` 机制保留。
- [ ] **切换提供商对话框**: 复用 `OIDCSwitchDialog`（10D 已重写）

**10E-2: Login.vue**

- [ ] **居中布局**: `flex items-center justify-center min-h-screen bg-gray-50 dark:bg-gray-900`
- [ ] **登录卡片**: `bg-white dark:bg-gray-800 rounded-lg shadow-md p-8 max-w-sm w-full`
- [ ] **按钮**: 原生 `<button>`（primary 变体，点击 → `window.location.href = '/api/v1/auth/login'`，逻辑不变）
- [ ] **暗色模式按钮**: 内联 SVG（Sun/Moon）替代 `el-icon`

**10E-3: Home.vue（🔴 全站最复杂页面）**

首页是项目中逻辑最复杂的页面（AGENTS.md 4.2），涉及订阅分级体系（默认/高级/自定义三种订阅的显示逻辑）、管理员预览模式、平台卡片响应式网格、下载客户端按钮等。实码共 **8 个 `<template v-if>` 条件块**，嵌套 `has_custom_sub`、`sub_type`、`isAdmin`、`*_configured` 四层判断。**迁移时每个分支逐一核对，不可遗漏**。

- [ ] **顶部栏**: 水平 flex 布局（`flex items-center justify-between`），内含：
  - 左侧：标题「VPN 订阅」+ 更新于时间戳
  - 右侧：管理面板按钮（仅管理员可见，`v-if="userStore.isAdmin"`）、用户名、角色标签（`el-tag` → Tailwind span：admin 紫色 / user 灰色）、退出按钮、暗色模式切换（`el-icon` Sunny/Moon → 内联 SVG）
- [ ] **加载与空状态**: `v-loading` → 自定义 spinner（`v-if="loading"`）；`el-empty` → 纯 Tailwind div 空状态
- [ ] **平台卡片响应式网格**: `el-row`/`el-col` → Tailwind `grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5`
- [ ] **平台卡片**: `el-card` → `<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden">`
  - 卡片头部（原 `#header` slot）→ `<div class="px-4 py-3 border-b border-gray-200 dark:border-gray-700">`
  - 卡片主体 → `<div class="p-4">`
- [ ] **订阅区段 — 8 个条件分支逐条迁移**（实码顺序）:

  | # | 条件 | 显示内容 | EP → Tailwind |
  |---|------|---------|---------------|
  | 1 | `has_custom_sub` | 「已被分配自定义订阅」标签 + 一键导入/复制链接/刷新链接 三个按钮 | `el-tag type="warning"` → orange span；三个 `el-button` → primary/default/warning-text |
  | 2 | `!has_custom_sub && sub_type === 'default'` | 「默认订阅」标签 + 三个按钮（普通用户主视图）。若 `!default_configured` 改为显示「默认订阅未配置，请联系管理员」| `el-tag type="info"` → gray span；按钮同上 |
  | 3 | `!has_custom_sub && sub_type === 'advanced'` | 「高级订阅」标签 + 三个按钮（高级用户主视图）。若 `!advanced_configured` 改为显示「高级订阅未配置，请联系管理员」| `el-tag type="warning"` → blue span；按钮同上 |
  | 4 | `isAdmin && !has_custom_sub && sub_type === 'advanced' && default_configured` | 「默认订阅（预览）」标签 + 三个按钮（管理员用 `p.preview_token` 预览默认订阅）| 标签 gray + 按钮组 |
  | 5 | `isAdmin && !has_custom_sub && sub_type === 'default' && advanced_configured` | 「高级订阅（预览）」标签 + 三个按钮（管理员用 `p.preview_token` 预览高级订阅）| 标签 blue + 按钮组 |
  | 6 | `isAdmin && !has_custom_sub && sub_type === 'advanced' && !default_configured` | 「默认订阅未配置」纯文本提示（无按钮）| `el-tag` → 无，纯 `<p>` |
  | 7 | `isAdmin && !has_custom_sub && sub_type === 'default' && !advanced_configured` | 「高级订阅未配置」纯文本提示（无按钮）| `el-tag` → 无，纯 `<p>` |
  | 8 | `isAdmin && has_custom_sub` | 管理员已有自定义订阅时，额外显示两组预览：① `v-if="p.preview_token && p.preview_sub_type === 'default'"` → 「默认订阅（预览）」；② `v-if="p.preview_token2 && p.preview_sub_type2 === 'advanced'"` → 「高级订阅（预览）」| 每组标签 + 两个按钮（一键导入/复制链接，无刷新链接） |

  - **注意**: 分支 8 的预览按钮仅含「一键导入」和「复制链接」两个按钮（实码无刷新链接），与分支 4/5 不同。迁移时须保留 `preview_sub_type` / `preview_sub_type2` 的条件判断，不可仅检查 `preview_token` 是否存在。
  - **三个按钮统一变体**: 一键导入 `primary`、复制链接 `default`、刷新链接 `warning text small`
  - **订阅标签统一替换**: `el-tag` → Tailwind `<span>`（默认灰 `bg-gray-100 text-gray-700` / 高级蓝 `bg-blue-100 text-blue-700` / 自定义橙 `bg-orange-100 text-orange-700`）
- [ ] **下载客户端按钮**: 仅当 `p.download_url` 非空时显示，Tailwind 链接按钮（`text-blue-600 hover:text-blue-700`）
- [ ] **复制链接对话框**: 保留 `el-dialog`，内部的 `el-input`（readonly）→ 原生 `<input readonly>` + Tailwind 样式，关闭按钮用 Tailwind 原生。点击输入框自动复制到剪贴板逻辑不变。**注意**: 改为原生 input 后 `copyInputRef.value` 直接是 DOM 元素，clipboard fallback 代码需从 `copyInputRef.value?.$el?.querySelector('input')` 改为 `copyInputRef.value`
- [ ] **所有 `ElMessage`** → `useToast()`（包括复制成功、刷新成功、导入失败等提示）

**10E-4: Rules.vue（用户规则浏览页）**

- [ ] 容器 `v-loading` → 自定义 spinner
- [ ] `el-empty` → 纯 Tailwind div 空状态
- [ ] 表格: 保留 `el-table`（用户只读浏览，无需排序/编辑功能）
- [ ] 客户端类型标签: `el-tag` → Tailwind span
- [ ] **`el-tooltip`**（2 处，禁用下载按钮的提示）→ 原生 `title` 属性（`<button disabled title="请联系管理员获取下载链接">`），无需保留 el-tooltip CSS
- [ ] 版本切换: 经实码核查，Rules.vue 当前无不含版本选择器（仅显示当前版本），此项跳过
- [ ] 下载按钮: Tailwind 原生

**验证**:
- `npm run build` 通过。
- Setup 表单在移动端不溢出（`max-w-lg` + 响应式 padding）。
- 输入框在暗色模式下背景/边框正常。
- `el-form` 校验错误提示正常显示。
- Login 页面居中，暗色模式切换正常。
- **Home 页面**: 8 个条件分支全部正确（逐条核对上述 1–8 分支）。响应式网格在 375px/768px/1024px 下均正常。一键导入/复制链接/刷新链接功能正常。下载客户端按钮仅在有 download_url 时显示。
- **Rules 页面**: 规则列表正确渲染，版本下载功能正常。

**涉及文件**: `frontend/src/views/Setup.vue`, `frontend/src/views/Login.vue`, `frontend/src/views/Home.vue`, `frontend/src/views/Rules.vue`

---

## 块 10F：管理布局（Manage.vue）

**目标**: 重写管理后台侧边栏布局。

**任务**:

- [ ] **整体布局**: `el-container`/`el-aside`/`el-main` → `flex h-screen`:
  ```html
  <div class="flex h-screen bg-gray-50 dark:bg-gray-900">
    <!-- 侧边栏 -->
    <aside class="...">...</aside>
    <!-- 内容区 -->
    <main class="flex-1 overflow-auto p-6">...</main>
  </div>
  ```
- [ ] **侧边栏**: 保留 `el-menu`（router 模式、高亮跟踪）。用 Tailwind + `:deep()` 覆盖其容器样式:
  - 桌面: `w-[200px] shrink-0 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700`
  - 移动端: `fixed inset-y-0 left-0 z-40 transition-transform duration-200` + `translate-x-[-200px]`（隐藏）/ `translate-x-0`（显示）
- [ ] **菜单项样式**: 用 `:deep()` 覆盖 Element Plus 菜单项颜色、激活态渐变紫色:
  ```css
  :deep(.sidebar-menu .el-menu-item.is-active) {
    background: linear-gradient(to right, #7c3aed, #6366f1);
    color: white;
  }
  ```
- [ ] **移动端汉堡按钮**: 顶部栏 `el-icon` → 内联 SVG（hamburger icon），`@click="toggleSidebar"`
- [ ] **菜单图标**: 各菜单项的 `el-icon` → 内联 SVG（Document/Share/Monitor/User/List/Setting/Tickets）
- [ ] **遮罩**: 移动端侧边栏展开时，主区添加半透明遮罩 `<div class="fixed inset-0 bg-black/50 z-30" @click="closeSidebar">`
- [ ] **顶部栏**: 暗色模式切换按钮 + 返回首页按钮（内联 SVG）

**验证**:
- `npm run build` 通过。
- 桌面端: 侧边栏固定 200px，菜单项点击路由跳转，当前路由项高亮（渐变紫色）。
- 移动端: 侧边栏默认隐藏，汉堡按钮点击滑入，遮罩点击关闭。
- 暗色模式: 侧边栏背景、菜单项颜色正常。

**涉及文件**: `frontend/src/views/Manage.vue`

---

## 块 10G：管理页面批次 1

**目标**: 重写 SubList、SubVersions、ShareList、ShareVersions、PlatformManage。

**通用替换清单**（5 个页面均适用，参见 10D 通用规则）:
- `v-loading` → 自定义 spinner（`v-if="loading"`）
- `el-empty` → 自定义空状态 div
- `el-button`/`el-input`/`el-tag`/`el-icon`/`el-select` → Tailwind 原生 / 内联 SVG
- `ElMessage` → `useToast()`

**保留**: `el-table`、`el-dialog`、`el-form`、`el-upload`（经 `UploadTabs`）

**10G-1: SubList.vue**

- [ ] 容器 `v-loading` → 自定义 spinner
- [ ] `el-empty` → 自定义空状态
- [ ] 表格: 保留 `el-table`
- [ ] 创建/编辑对话框: 保留 `el-dialog` + `el-form`。表单内 `el-input` → 原生 input，`el-select`（类型/平台）→ 原生 select
- [ ] 类型标签（default/advanced）、平台标签: Tailwind `el-tag` 替代
- [ ] 按钮组（版本管理、编辑、删除）: Tailwind 原生
- [ ] 头部 `el-icon`（Plus）→ 内联 SVG

**10G-2: ShareList.vue（🔴 关键: 彻底绕过 bug）**

> **维护者确认**: ShareList 创建按钮 bug 极其隐蔽无法定位修复。迁移时**彻底摒弃 `el-empty` + 条件渲染组合**，改用纯 Tailwind div 从根本上绕过。

- [ ] **根因绕过**: 移除所有 `el-empty`，改用纯 Tailwind div 三态分支:
  ```html
  <!-- 加载态 -->
  <div v-if="loading" class="flex items-center justify-center py-12">
    <svg class="animate-spin h-5 w-5 mr-2 text-blue-600" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
    </svg>
    <span class="text-gray-500 dark:text-gray-400">加载中...</span>
  </div>
  <!-- 空态 -->
  <div v-else-if="shares.length === 0" class="text-center py-12 text-gray-400 dark:text-gray-500">
    暂无分享订阅，请创建
  </div>
  <!-- 列表态 -->
  <el-table v-else :data="shares" ...>...</el-table>
  ```
  - **严禁**使用 `el-empty`。
  - **严禁**在 `el-table` 上叠加 `v-loading`。
- [ ] **创建按钮可见性**: 创建按钮位于 `.page-header`（顶部），**始终可见**，不依赖列表加载状态。确认按钮无 `v-if` 条件包裹导致隐藏。
- [ ] 创建对话框: 保留 `el-dialog`。对话框结构（当前代码名称输入在每个 tab-pane 内重复，迁移后只保留一份在顶部）:
    ```
    el-dialog
      ├── 名称 input（原生，el-form 内，对话框顶部，仅一份）
      ├── UploadTabs（文件/文本切换，无按钮，10C 产出）
      └── #footer（取消 + 创建按钮，Tailwind 原生）
    ```
    内联 `el-tabs` → 改用 `UploadTabs`（10C 产出）。名称输入 `el-input` → 原生 input。
- [ ] Token 状态标签（有效/已吊销）、按钮组（版本管理、复制链接、刷新 Token、吊销 Token、删除）: Tailwind 原生
- [ ] 头部 `el-icon`（Plus）→ 内联 SVG

**10G-3: SubVersions.vue / ShareVersions.vue**

- [ ] 容器 `v-loading` → 自定义 spinner
- [ ] `el-empty` → 自定义空状态
- [ ] 版本列表表格: 保留 `el-table`
- [ ] 当前版本标识: Tailwind 绿色标签 `<span class="rounded-full px-2 py-0.5 text-xs font-medium bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300">当前</span>`
- [ ] 上传/编辑: `UploadModal`（10D 已重写，内部用 `UploadTabs`）
- [ ] 预览对话框: 保留 `el-dialog`，`<pre class="...">` 展示内容
- [ ] 头部 `el-icon`（ArrowLeft/Plus）→ 内联 SVG
- [ ] 返回按钮、上传按钮: Tailwind 原生

**10G-4: PlatformManage.vue**

- [ ] 容器 `v-loading` → 自定义 spinner
- [ ] `el-empty` → 自定义空状态
- [ ] 表格: 保留 `el-table`
- [ ] Client Schemes: Tailwind `flex flex-wrap gap-1` 标签列表（每个 scheme 一个 `el-tag` 替代 span）
- [ ] 创建/编辑对话框: 保留 `el-dialog` + `el-form`。表单内 `el-input` → 原生 input，download_url 字段原生 input。
- [ ] 头部 `el-icon`（Plus）→ 内联 SVG

**验证**:
- `npm run build` 通过。
- **ShareList 创建按钮可见且可交互**（bug 绕过确认）: 页面加载后创建按钮立即可点击，空态/列表态均正常显示。
- 各管理页面 CRUD 功能正常（创建、编辑、删除、版本管理）。
- 暗色模式下标签、按钮、表格视觉正常。

**涉及文件**: `frontend/src/views/SubList.vue`, `frontend/src/views/SubVersions.vue`, `frontend/src/views/ShareList.vue`, `frontend/src/views/ShareVersions.vue`, `frontend/src/views/PlatformManage.vue`

---

## 块 10H：管理页面批次 2

**目标**: 重写 UserManage、RulesManage、RuleVersions、OIDCConfig、Logs。通用替换规则同 10G。

**10H-1: UserManage.vue**

- [ ] 容器 `v-loading` → 自定义 spinner
- [ ] `el-empty` → 自定义空状态
- [ ] 表格: 保留 `el-table`
- [ ] **is_advanced 开关**: `el-switch` → Tailwind 自定义 toggle:
  ```html
  <button @click="toggleAdvanced(row)"
    class="relative inline-flex h-6 w-11 items-center rounded-full transition-colors"
    :class="row.is_advanced ? 'bg-blue-600' : 'bg-gray-300 dark:bg-gray-600'">
    <span class="inline-block h-4 w-4 transform rounded-full bg-white transition-transform"
      :class="row.is_advanced ? 'translate-x-6' : 'translate-x-1'"/>
  </button>
  ```
  - **管理员行禁用**（is_advanced 不可修改，AGENTS.md 5.x）: 加 `disabled:opacity-50 disabled:cursor-not-allowed` + `@click` 守卫
- [ ] 角色/级别标签: Tailwind 原生（role: admin 紫色 / user 灰色；is_advanced: 高级 蓝色 / 普通 灰色）
- [ ] 编辑对话框: 保留 `el-dialog` + `el-form`
- [ ] 上传自定义订阅对话框: 保留 `el-dialog` + `el-upload`（经 `UploadTabs` 或直接 el-upload）。平台选择 `el-select` → 原生 select
- [ ] groups 字段: 仅当用户有 groups 时显示（AGENTS.md 4.5）
- [ ] 按钮组（编辑、上传自定义订阅、删除自定义订阅、吊销 Token、删除用户）: Tailwind 原生
- [ ] 头部 `el-icon` → 内联 SVG

**10H-2: RulesManage.vue**

- [ ] 容器 `v-loading` → 自定义 spinner
- [ ] `el-empty` → 自定义空状态
- [ ] 表格: 保留 `el-table`
- [ ] 创建对话框: 保留 `el-dialog`。内联 `el-tabs` → 改用 `UploadTabs`（10C 产出）。三个表单字段（均用原生元素，位于对话框顶部）: **ID**（`el-input` → 原生 input，格式 `[a-z0-9-]+`）、**名称**（`el-input` → 原生 input）、**客户端类型**（`el-select` → 原生 select，当前仅 Shadowrocket 可选）。对话框结构同 ShareList（10G-2）: 表单字段在顶部 → UploadTabs 居中 → #footer 按钮
- [ ] Token 状态标签、按钮组（版本管理、复制链接、轮替 Token、删除）: Tailwind 原生
- [ ] 头部 `el-icon` → 内联 SVG

**10H-3: RuleVersions.vue**

- [ ] 同 10G-3 模式（SubVersions 结构）

**10H-4: OIDCConfig.vue**

- [ ] 容器 `v-loading` → 自定义 spinner
- [ ] **双栏布局**: `el-card` → Tailwind `grid grid-cols-1 lg:grid-cols-2 gap-6`，每栏 `<div class="bg-white dark:bg-gray-800 rounded-lg shadow-md p-6">`
- [ ] 输入框: `el-input` → 原生 input（密码框 `type="password"`，Client Secret 脱敏回显）
- [ ] 切换提供商: `OIDCSwitchDialog`（10D 已重写）
- [ ] **速率限制**: `el-input-number` → 原生 `<input type="number">`。`el-row`/`el-col` → Tailwind `grid grid-cols-2 gap-4`
- [ ] 测试连接、保存按钮: Tailwind 原生（loading 态）

**10H-5: Logs.vue**

- [ ] `v-loading` → 自定义 spinner
- [ ] `el-empty` → 自定义空状态
- [ ] **日期选择**: `el-date-picker` → 原生 `<input type="date">` + Tailwind 样式
- [ ] 表格: 保留 `el-table`
- [ ] 状态标签（success 绿色 / failed 红色）: Tailwind 原生
- [ ] 筛选按钮: Tailwind 原生

**验证**:
- `npm run build` 通过。
- UserManage: is_advanced toggle 切换正常，管理员行禁用，上传自定义订阅成功。
- RulesManage: 创建规则（文件 + 文本两种方式）成功，轮替 Token 正常。
- OIDCConfig: 双栏布局，速率限制 input 可修改保存，切换提供商对话框正常。
- Logs: 日期筛选正常，状态标签颜色正确。
- 暗色模式下所有页面视觉正常。

**涉及文件**: `frontend/src/views/UserManage.vue`, `frontend/src/views/RulesManage.vue`, `frontend/src/views/RuleVersions.vue`, `frontend/src/views/OIDCConfig.vue`, `frontend/src/views/Logs.vue`

---

## 块 10I：单容器化

**目标**: 合并 backend + frontend 为单一容器，Go 后端 serve API + 静态文件 + SPA 回退。**本块不新增任何 API 路由**，仅调整后端静态服务 + 删除根路由 JSON handler。

**10I-1: 根目录 Dockerfile（多阶段构建）**

- [ ] 创建 repo 根目录 `Dockerfile`:
  ```dockerfile
  # Stage 1: Build frontend
  FROM node:22-alpine AS frontend-builder
  WORKDIR /app
  COPY frontend/package.json frontend/package-lock.json ./
  RUN npm ci
  COPY frontend/ .
  RUN npm run build

  # Stage 2: Build Go binary
  FROM golang:alpine AS backend-builder
  WORKDIR /app
  COPY backend/go.mod backend/go.sum ./
  RUN go mod download
  COPY backend/ .
  RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o server ./cmd/server

  # Stage 3: Runtime (distroless, 零 CGO)
  FROM gcr.io/distroless/static-debian12
  COPY --from=backend-builder /app/server /server
  COPY --from=frontend-builder /app/dist /app/public
  ENV DATA_DIR=/app/data
  ENV PORT=8080
  EXPOSE 8080
  ENTRYPOINT ["/server"]
  ```
  - 前端构建产物 `dist/` 复制到 `/app/public`（后端读取静态文件路径）。
  - `CGO_ENABLED=0` + distroless 满足 AGENTS.md 6.1"纯静态编译，不依赖 CGO"。

**10I-2: 后端 SPA fallback + 删除根路由 JSON**

- [ ] 修改 `backend/internal/router/router.go` — **删除根路由 JSON handler**（WAF/LB 探测改用 `/health`）:
  ```go
  // 删除以下代码（约第 24-26 行）:
  // // Root endpoint for WAF / load-balancer health probes
  // r.GET("/", func(c *gin.Context) {
  //     c.JSON(200, gin.H{"status":"ok"})
  // })
  ```
  `/health` 保留不变（容器健康检查）。
- [ ] 在 `SetupRouter()` 末尾（`return r` 之前）添加 SPA 静态服务:
  ```go
  // 在现有 import 块中新增: "os", "path/filepath", "strings"

  // ... 现有 API 路由注册 ...

  // Serve frontend static files (JS/CSS/fonts/images from Vite build)
  r.Static("/assets", "/app/public/assets")

  // SPA fallback: 非 API 请求先尝试服务 /app/public 下的静态文件（vite.svg 等），
  // 不存在再回落 index.html（Vue Router 在客户端接管 /admin/subscriptions 等前端路由）
  r.NoRoute(func(c *gin.Context) {
      path := c.Request.URL.Path
      if strings.HasPrefix(path, "/api/") {
          c.JSON(404, gin.H{"error": "not found"})
          return
      }
      // 尝试服务 /app/public 下的静态文件（防路径穿越）
      cleaned := filepath.Clean(filepath.Join("/app/public", path))
      if strings.HasPrefix(cleaned, "/app/public/") {
          if info, err := os.Stat(cleaned); err == nil && !info.IsDir() {
              c.File(cleaned)
              return
          }
      }
      // 静态文件不存在 → 回退 index.html
      c.File("/app/public/index.html")
  })

  return r
  ```
  - `r.Static("/assets", "/app/public/assets")` — 匹配 Vite 构建产物的默认输出路径（`dist/assets/`），匹配该前缀的请求由 Static 直接服务（文件不存在时 Static 自身返回 404，不触发 NoRoute）。
  - `r.NoRoute(...)` — 非 API 请求**先尝试服务 `/app/public` 下的静态文件**（如 `vite.svg` favicon、`robots.txt` 等非 `/assets` 的根级静态资源），**不存在再回落 `index.html`**。Vue Router 在客户端接管 `/admin/subscriptions` 等前端路由。
  - **路径穿越防护**: `filepath.Clean(filepath.Join("/app/public", path))` + `strings.HasPrefix(cleaned, "/app/public/")` 双重校验，防止 `../` 逃逸（符合 AGENTS.md 安全约束）。`/` 会被 Clean 为目录、`os.Stat` 命中目录时 `!info.IsDir()` 为 false → 跳过 → 回落 index.html。
  - API 路径上的 404 仍返回 JSON 错误（保持 Phase 1 行为）。
  - **不新增任何 API 路由前缀**，完全复用 Gin 内置的 `Static` 和 `NoRoute`。
- [ ] **开发环境兼容**: 本地开发时 `go run` 无 `/app/public` 目录，`r.Static`/`c.File` 会 404，但本地前端用 Vite dev server (5173) 直接访问，不受影响。可选: 用 `os.Stat` 判断目录存在再注册 Static（非必须，但更健壮）。**决策**: 不加判断，保持简单；开发时前端走 Vite，后端 `/` 返回 404 由前端 dev server 代理兜底（`vite.config.js` 的 `/api` proxy 已处理 API，根路径由 Vite serve）。
- [ ] 确认 `main.go` 无需修改（`PORT`、`DATA_DIR`、`SetTrustedProxies` 逻辑不变）。

**10I-3: docker-compose.yml（单服务）**

- [ ] 合并为单服务:
  ```yaml
  services:
    app:
      build:
        context: .
        dockerfile: Dockerfile
      container_name: vpn-app
      ports:
        - "127.0.0.1:8080:8080"
      volumes:
        - vpn-data:/app/data
      restart: unless-stopped

  volumes:
    vpn-data:
  ```
  - 端口绑定 `127.0.0.1`（AGENTS.md 8.1: 禁止直接映射到宿主机公网接口）。
  - 单 volume `vpn-data` 挂载 `/app/data`（AGENTS.md 8.7）。
  - **不配置 compose healthcheck**（决策）: distroless 镜像无 shell/wget/curl，容器内 healthcheck 难以实现。健康探测由外部 NGINX 指向 `/health`（已显式注册为 Gin handler）完成，与 10I-4 NGINX 配置一致。当前 frontend 容器的 `wget --spider` healthcheck 在单容器架构下不再适用。

**10I-4: 外部 NGINX 参考配置**

- [ ] 更新 `deploy/nginx-example.conf` — 简化为单端口反代:
  ```nginx
  server {
      listen 443 ssl;
      server_name vpn.example.com;
      # ssl_certificate ... (已有的 TLS 配置)

      location / {
          proxy_pass http://127.0.0.1:8080;
          proxy_set_header Host              $host;
          proxy_set_header X-Real-IP         $remote_addr;
          proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
          proxy_set_header X-Forwarded-Proto $scheme;

          client_max_body_size 55m;  # 50MB 上传限制 + 余量
      }
  }
  ```
- [ ] 更新 `deploy/README.md` — 单容器部署说明（WAF/LB 健康探测指向 `/health`，不再指向 `/`）。

**10I-5: 清理**

- [ ] `frontend/Dockerfile`、`frontend/nginx.conf` — 不再需要，**保留文件不删除**（避免破坏 git 历史；docker-compose 不再引用）。
- [ ] `backend/Dockerfile` — 不再用于部署，**保留备用**。
- [ ] 在 `deploy/README.md` 中注明旧的双容器 Dockerfile 已废弃。

**验证**:
- `docker build -t vpn-app .` 构建成功（三阶段）。
- `docker compose up -d` 启动 → 单容器运行。
- `curl http://127.0.0.1:8080/health` → `{"status":"ok"}` ✅
- `curl http://127.0.0.1:8080/` → 返回 `index.html`（SPA 回退，根路由 JSON 已删除）✅
- `curl http://127.0.0.1:8080/admin/subscriptions` → 返回 `index.html`（SPA 回退）✅
- `curl http://127.0.0.1:8080/api/v1/nonexistent` → 404 JSON ✅
- `curl http://127.0.0.1:8080/assets/index-*.js` → 返回 JS 文件 ✅
- `curl http://127.0.0.1:8080/vite.svg` → 返回 SVG 文件（favicon 静态文件服务，Content-Type: image/svg+xml）✅
- `curl http://127.0.0.1:8080/../etc/passwd` → 返回 `index.html`（路径穿越被拦截，未泄露系统文件）✅

**涉及文件**: `Dockerfile`(新，repo 根), `backend/internal/router/router.go`, `docker-compose.yml`, `deploy/nginx-example.conf`, `deploy/README.md`

---

## 块 10J：收尾 — EP CSS 缩减 + 全量回归验证

**目标**: 所有页面迁移完成后，缩减 Element Plus CSS 为仅 5 个保留组件，并执行全量回归验证。

> **前置条件**: 10D–10H 全部完成（含 10E 的 Home.vue + Rules.vue 迁移），所有页面已不依赖被移除组件（el-button/el-input/el-tag/el-card/el-empty/el-switch/el-select/el-date-picker/el-tooltip/el-tabs/el-row/el-col/el-icon/el-input-number/el-aside/el-main/el-config-provider/v-loading）的 CSS。

**10J-1: EP CSS 缩减**

- [ ] **路径验证**（实施前先确认）: 在 `frontend/node_modules/` 下验证以下路径存在:
  ```bash
  ls node_modules/element-plus/es/components/table/style/css.js
  ls node_modules/element-plus/es/components/dialog/style/css.js
  ls node_modules/element-plus/es/components/form/style/css.js
  ls node_modules/element-plus/es/components/upload/style/css.js
  ls node_modules/element-plus/es/components/menu/style/css.js
  ls node_modules/element-plus/es/components/menu-item/style/css.js
  ```
  Element Plus v2.9.6 应支持此路径。若个别文件不存在，改用 `element-plus/theme-chalk/el-table.css` 等替代路径。
- [ ] 修改 `frontend/src/main.js` — 全量 EP CSS 引入缩减为仅 5 个组件 + 暗色变量:
  ```js
  import { createApp } from 'vue'
  import { createPinia } from 'pinia'
  import ElementPlus from 'element-plus'
  import zhCn from 'element-plus/es/locale/lang/zh-cn'

  // 仅保留 5 个复杂组件的样式
  import 'element-plus/es/components/table/style/css'
  import 'element-plus/es/components/dialog/style/css'
  import 'element-plus/es/components/form/style/css'
  import 'element-plus/es/components/upload/style/css'
  import 'element-plus/es/components/menu/style/css'
  import 'element-plus/es/components/menu-item/style/css'
  // 暗色模式变量仍需要
  import 'element-plus/theme-chalk/dark/css-vars.css'

  import '@/assets/tailwind.css'
  import App from './App.vue'
  import router from './router'

  const app = createApp(App)
  app.use(createPinia())
  app.use(router)
  app.use(ElementPlus, { locale: zhCn })
  app.mount('#app')
  ```
  - `app.use(ElementPlus)` 仍注册全部组件 JS（降低变更范围），但仅引入 5 个组件 CSS。若后续要进一步 tree-shake JS 可用 `unplugin-element-plus`，**本期不做**。
- [ ] **移除 `@element-plus/icons-vue` 依赖**（10D–10H 已将各 `.vue` 的按需 import 改为内联 SVG）:
  - 确认 grep 无残留 `from '@element-plus/icons-vue'` 导入（10D–10H 应已逐文件删除）。
  - 从 `frontend/package.json` 移除 `@element-plus/icons-vue` 依赖: `npm uninstall @element-plus/icons-vue`。
  - **注**: main.js 无全局注册代码（图标一直按需导入），故 10J **无需改 main.js 删 icons**；main.js 仅按 10J-1 缩减 EP CSS 引入。

**10J-2: 编译验证**

- [ ] `cd frontend && npm run build` 通过（Tailwind 编译 + Vite 构建）。
- [ ] `cd backend && go build ./...` 通过（含 SPA fallback + `strings` import + 删除根路由）。
- [ ] 检查前端构建产物 CSS 体积显著减小（EP CSS 从全量 ~1.1MB 缩减为 5 组件）。

**10J-3: 全量回归验证**

- [ ] `docker compose build --no-cache` 构建成功（三阶段多步构建）。
- [ ] `docker compose up -d` 启动 → 单容器运行 → 端口 8080。
- [ ] 浏览器访问 `http://127.0.0.1:8080/` → 页面正常加载。
- [ ] **ShareList 创建按钮可见且可交互**（bug 绕过确认）。
- [ ] 暗色模式切换正常（所有页面）。
- [ ] 手机宽度下页面布局正常（响应式，Chrome DevTools 模拟 375px/768px/1024px/1280px）。
- [ ] Toast 通知正常（替代 ElMessage，右下角 3 秒消失）。
- [ ] **5 个保留 EP 组件功能回归**:
  - [ ] `el-table` 排序、列宽正常（SubList/ShareList/UserManage/RulesManage/Logs/SubVersions/ShareVersions/RuleVersions/PlatformManage）
  - [ ] `el-dialog` 弹窗开关正常（所有创建/编辑/预览对话框）
  - [ ] `el-form` 校验正常（Setup/SubList/PlatformManage/UserManage 创建编辑表单）
  - [ ] `el-upload` 文件上传正常（UploadTabs 内部，所有版本上传场景）
  - [ ] `el-menu` 路由高亮正常（Manage 侧边栏）
- [ ] **业务 API 回归**:
  - [ ] `/health` 返回 `{"status":"ok"}`
  - [ ] `/api/v1/system/status` 返回 `{ configured: bool }`
  - [ ] 登录流程（OIDC 跳转 → 回调 → JWT 存 localStorage）
  - [ ] 订阅下载（JWT + Token + Share Token 三种途径）
  - [ ] 管理操作（用户/订阅/分享/平台/规则/OIDC/日志 CRUD）
- [ ] **SPA 路由回退回归**:
  - [ ] 直接访问 `/admin/subscriptions` → 加载 Manage 页面（非 404）
  - [ ] 直接访问 `/rules` → 加载 Rules 页面
  - [ ] 刷新任意前端路由页面 → 正常加载（非白屏）

**涉及文件**: `frontend/src/main.js`, `frontend/package.json`(可选移除 icons-vue)

---

## 四、验收标准

1. `go build ./...` 和 `npm run build` 均通过。
2. `docker compose up -d` 单容器启动正常，端口 `127.0.0.1:8080`。
3. 前端页面通过 Go 后端 serve 正常加载（`/` → index.html，`/assets/*` → 静态资源）。
4. ShareList 创建按钮可见且可交互 ✅（bug 绕过确认）。
5. 所有 15 个页面在桌面端和移动端布局正常。
6. 暗色模式在所有页面生效（Tailwind `dark:` 前缀 + useTheme.js 联动）。
7. Toast 通知正常（替代 ElMessage，右下角 3 秒消失）。
8. 5 个保留的 Element Plus 组件（table/dialog/form/upload/menu）功能正常。
9. 所有业务 API 不受影响（无新增路由，无路由变更）。
10. 外部 NGINX 配置简化为单端口反代。
11. 无 CDN 运行时依赖，所有资源本地打包。
12. 根路由 `/` 返回 index.html（不再返回 JSON），`/health` 保留 JSON 健康检查。
13. EP CSS 缩减为仅 5 个保留组件（产物体积显著减小）。
14. `@element-plus/icons-vue` 已移除，全部改为内联 SVG。

---

## 五、与 Phase 1 的关键差异

| 项目 | Phase 1 | Phase 2 |
|------|---------|---------|
| 容器数 | 2（backend + frontend nginx） | **1**（Go serve 一切） |
| 端口映射 | `127.0.0.1:8080` + `127.0.0.1:8081` | **仅** `127.0.0.1:8080` |
| 根路由 `/` | 返回 JSON `{"status":"ok"}` | **返回 index.html**（SPA 回退） |
| 健康探测 | `/` 和 `/health` 均可用 | **仅 `/health`**（WAF/LB 探测改指此处） |
| 外部 NGINX | `/api/*` → backend, `/*` → frontend | **`/*` → 单容器** |
| Element Plus | 全量引入（~1.1MB CSS chunk） | **仅保留 5 个组件 CSS**（CSS 大幅减小） |
| 组件库 | Element Plus 全部 | **Tailwind 原生** + 5 个 Element Plus 组件 |
| 通知系统 | ElMessage | **自建 useToast()** |
| 图标 | @element-plus/icons-vue | **内联 SVG** |
| 加载状态 | v-loading 指令 | **自定义 spinner** |
| Tabs | el-tabs（UploadModal + ShareList + RulesManage 各一份） | **UploadTabs.vue 可复用组件** |
| 空状态 | el-empty | **纯 Tailwind div**（ShareList 关键绕过） |
| 前端路由 | Vite base `/` | **不变**（`/`） |
| 后端路由 | API 路由 + 根 JSON | API 路由 + **Static + NoRoute（SPA fallback）**，删除根 JSON |
| Dockerfile | 2 个（backend/ + frontend/） | **1 个**（repo 根目录，多阶段） |
| NGINX 配置 | 双 location | **单 location** |

---

## 六、风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Tailwind preflight 与 EP 组件样式冲突 | el-table/el-form/el-dialog 视觉异常 | 10B 实测，scoped `:deep()` 针对性覆盖，勿全局禁用 preflight |
| Home.vue 订阅分支逻辑复杂（8 个条件分支，嵌套 4 层判断），迁移时易遗漏 | 部分用户看到错误的订阅类型或按钮缺失 | 10E-3 逐分支列出全部 8 个 `<template v-if>` 块及对应 EP→Tailwind 替换；迁移后按用户角色逐分支回归测试 |
| EP CSS 缩减后遗漏组件 CSS | 个别页面元素无样式 | 10J 前确认所有页面已迁移；缩减后全量回归（10J-3） |
| ShareList bug 未彻底绕过 | 创建按钮不可见/不可交互 | 10G-2 严禁 el-empty + v-loading，纯 Tailwind div 三态分支；10J 验证 |
| 根路由删除影响 WAF 探测 | 健康检查失败，容器被标记不健康 | 部署文档明确 WAF/LB 探测改指 `/health`；`/health` 保留 |
| distroless 无 shell，调试困难 | 容器内无法 exec sh | 构建阶段充分测试；运行时仅静态二进制 + 静态文件，无需调试 |
| 开发环境后端无 `/app/public` | `r.Static`/`c.File` 本地 404 | 开发时前端走 Vite dev server (5173)，`/api` proxy 兜底；根路径由 Vite serve |

---

## 七、编码约束提醒（构建时必须遵守，源自 AGENTS.md 第五章）

- **Vue 模板**: 属性中不可使用双引号转义 `\"`，用「」或计算属性；`v-model` 中不可用可选链 `?.`，用 `v-if` 守卫。
- **删除确认**: 必须用 `ConfirmDialog.vue` 组件，不用 `ElMessageBox.confirm`。
- **登出**: 调用 `userStore.logout(router)`，传入 router 实例。
- **文件上传**: 手动设置 `Content-Type: multipart/form-data`。
- **Tailwind CSS**: 优先 utility class，禁止内联 `style` 属性；暗色用 `dark:` 前缀；自定义 CSS 仅在 Tailwind 无法覆盖时写 `<style scoped>`；响应式用 Tailwind 预设断点；`darkMode: 'class'`。
- **Go 工程**: 修改 `go.mod` 后运行 `go mod tidy`；修改代码后运行 `go build ./...` 验证；`isValidID()` 在 utils 包。
- **安全**: 所有含用户输入的路径操作经 `sanitizePath()`；所有 `/api/v1/admin/*` 路由有 AdminRequired 中间件；Logger 脱敏 `?token=`；下载端点返回 `Cache-Control: no-store, no-cache, must-revalidate` + `Pragma: no-cache`。
- **不新增 API 路由**: Phase 2 仅调整后端静态服务（Static + NoRoute）+ 删除根路由 JSON，**不新增任何 `/api/v1/*` 路由**。

---

## 八、Phase 2 完成状态确认（2026-07-22）

全部 10 个块（10A–10J）已完成。详见 `BUILD_PLAN_PHASE2_MOBILE.md` 中「Phase 2 完成状态」章节。

后续改进计划（Phase 2.1 移动端表格 UX）也已记录在 `BUILD_PLAN_PHASE2_MOBILE.md`。
