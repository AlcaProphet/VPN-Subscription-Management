---
kind: frontend_style
name: 前端样式体系：Tailwind + Ant Design Vue 主题与暗色模式
category: frontend_style
scope:
    - '**'
source_files:
    - frontend/tailwind.config.js
    - frontend/postcss.config.js
    - frontend/src/style.css
    - frontend/src/theme.ts
    - frontend/src/App.vue
    - frontend/src/main.ts
    - frontend/package.json
    - frontend/src/layouts/AdminLayout.vue
    - frontend/src/layouts/HomeLayout.vue
    - frontend/src/layouts/BlankLayout.vue
    - frontend/src/components/AppHeader.vue
---

## 1. 采用的样式系统

前端采用 **Tailwind CSS v3** 作为原子化样式框架，配合 **PostCSS + Autoprefixer** 在构建时生成 CSS；UI 组件层统一使用 **Ant Design Vue 4.x**（含 `@ant-design/icons-vue`），并通过 `ConfigProvider` 在全局注入语言与主题。样式入口为 `frontend/src/style.css`，仅引入 Tailwind 的 base/components/utilities 三个层级并补充全局 html/body/#app 基础重置。

## 2. 关键文件与包

- `frontend/tailwind.config.js`：启用 `darkMode: 'class'`，扫描 `./index.html` 与 `./src/**/*.{vue,ts}` 中的类名。
- `frontend/postcss.config.js`：注册 `tailwindcss` 与 `autoprefixer` 两个插件。
- `frontend/src/style.css`：Tailwind 指令入口 + 全局基础样式。
- `frontend/src/theme.ts`：集中定义 AntD 主题 token（主色 `#1677FF`）、中文 locale（`zh_CN`）以及基于 `localStorage('theme')` 的明/暗切换 composable `useTheme()`，通过 `document.documentElement.classList.toggle('dark', ...)` 驱动 Tailwind 的 class 模式暗色。
- `frontend/src/App.vue`：根组件用 `<ConfigProvider :locale="antdLocale" :theme="antdTheme">` 包裹整个应用，使所有 AntD 组件共享主题与语言。
- `frontend/package.json`：声明依赖 `ant-design-vue ^4.2.6`、`tailwindcss ^3.4.17`、`postcss ^8`、`autoprefixer ^10`、`vite ^5` 等。

## 3. 架构与约定

- **布局分层**：`layouts/AdminLayout.vue`、`HomeLayout.vue`、`BlankLayout.vue` 提供三种页面骨架，内部通过 Tailwind 类组合背景色、间距与阴影。
- **组件组织**：`components/` 存放跨视图复用的 UI 片段（如 `AppHeader`、`ConfirmModal`、`CaptchaWidget`、`TriStateList`、`MarkdownView`），每个组件按需从 `ant-design-vue` 导入所需子组件，避免全量引入。
- **视图划分**：`views/admin/` 下按功能域（Users、Subscriptions、Settings、Rules、Groups、Platforms、Shares、Approvals、Logs、Assembly、VersionManage 等）拆分单文件组件；用户端视图（Login、Register、Forgot、Reset、Profile、Home、Rules、Emergency、Setup 等）位于 `views/` 根目录。
- **状态与样式解耦**：主题状态由 Pinia store（`stores/system.ts`）或 `useTheme()` composable 管理，不混入业务逻辑；样式完全通过 Tailwind 类名与 AntD 主题 token 表达。
- **构建产物**：Vite 构建后静态资源被 Go 后端通过 `backend/web/web.go` 嵌入并随单一二进制分发，因此前端无独立部署形态。

## 4. 约定与约束

- **暗色模式开关**：通过给 `<html>` 添加/移除 `dark` 类实现，所有组件内使用 `dark:` 前缀的 Tailwind 类（如 `bg-white dark:bg-gray-800`、`text-gray-100`、`border-gray-200 dark:border-gray-600`）响应主题切换；该策略在 `theme.ts` 中由 `watch(dark, ...)` 统一维护到 `localStorage` 与 DOM。
- **品牌主色**：AntD 主题 token 固定为 `colorPrimary: '#1677FF'`，未做额外扩展，保持默认科技蓝。
- **国际化**：AntD 全局使用 `zh_CN` locale，所有表单提示、按钮文案等中文由组件库内置。
- **样式粒度**：不使用 SCSS/Less，也不编写自定义 CSS 规则（除 `style.css` 中的基础重置外），全部样式以 Tailwind 原子类直接写在模板中。
- **组件库使用规范**：各组件按需 import AntD 子组件（例如 `Form, Input, Button, Alert`），未见全局注册，便于 tree-shaking。
- **测试覆盖**：`frontend/tests/theme.spec.ts` 验证了 `useTheme` 的 localStorage 持久化与 `document.documentElement.classList` 行为，确保暗色模式切换契约稳定。

总体而言，本项目的前端风格体系以 Tailwind 原子类 + Ant Design Vue 组件库为核心，通过单一的 `theme.ts` 暴露主题与暗色模式能力，并以 `ConfigProvider` 在根节点统一注入，形成一致、可切换且无需额外 CSS 文件的视觉风格。