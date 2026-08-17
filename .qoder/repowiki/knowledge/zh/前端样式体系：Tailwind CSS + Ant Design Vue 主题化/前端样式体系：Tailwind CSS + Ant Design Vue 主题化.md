---
kind: frontend_style
name: 前端样式体系：Tailwind CSS + Ant Design Vue 主题化
category: frontend_style
scope:
    - '**'
source_files:
    - frontend/tailwind.config.js
    - frontend/postcss.config.js
    - frontend/src/style.css
    - frontend/src/theme.ts
    - frontend/src/App.vue
    - frontend/src/layouts/AdminLayout.vue
    - frontend/src/layouts/HomeLayout.vue
    - frontend/src/layouts/BlankLayout.vue
    - frontend/src/components/AppHeader.vue
    - frontend/src/components/ConfirmModal.vue
---

## 1. 采用的样式系统

- **原子类框架**：使用 Tailwind CSS（`tailwind.config.js` 配置 `darkMode: 'class'`，扫描 `./src/**/*.{vue,ts}`），通过 `postcss.config.js` 启用 `tailwindcss` 与 `autoprefixer`。
- **组件库**：Ant Design Vue 作为 UI 组件来源，所有视图与布局均直接 import `ant-design-vue` 的组件（Layout、Menu、Drawer、Modal、Form、Table、Card、Steps 等）。
- **主题引擎**：通过 `theme.ts` 暴露 `useTheme()` composable，基于 `localStorage('theme')` 在 `light`/`dark` 间切换，并同步 `document.documentElement.classList.toggle('dark', v)`，驱动 Tailwind 的 `dark:` 前缀规则生效；同时把 AntD 的 `defaultAlgorithm` / `darkAlgorithm` 与统一主色 `#1677FF` 注入到全局 `ConfigProvider`。
- **全局入口**：`style.css` 仅引入 `@tailwind base/components/utilities` 及少量 reset（`html/body` margin/padding/min-height、`#app min-h-screen`）。

## 2. 关键文件

| 文件 | 作用 |
|---|---|
| `frontend/tailwind.config.js` | Tailwind 配置，开启 class 模式暗色、内容扫描路径 |
| `frontend/postcss.config.js` | PostCSS 插件链（tailwindcss + autoprefixer） |
| `frontend/src/style.css` | Tailwind 指令入口 + 全局 reset |
| `frontend/src/theme.ts` | 主题状态、AntD 算法与主色、中文 locale 导出 |
| `frontend/src/App.vue` | 根组件，用 `ConfigProvider` 包裹应用并传入 `antdLocale`、`antdTheme` |
| `frontend/src/layouts/AdminLayout.vue` | 管理后台布局，Sider 220/64 双宽、`<768px` 切 Drawer |
| `frontend/src/layouts/HomeLayout.vue`、`BlankLayout.vue` | 用户端与空白页布局 |
| `frontend/src/components/AppHeader.vue` | 顶部栏，含站点名、时间戳、暗色开关 |
| `frontend/src/components/ConfirmModal.vue` | 统一确认弹窗（支持危险按钮、确认词校验） |
| `frontend/src/views/**/*.vue` | 各页面，全部以 Tailwind 原子类 + AntD 组件组合实现 |

## 3. 架构与约定

- **布局分层**：`layouts/` 提供三种骨架——`AdminLayout`（侧边栏+顶栏+卡片内容区）、`HomeLayout`（用户端首页背景）、`BlankLayout`（居中登录/注册等全屏表单）。视图按角色分目录：`views/admin/` 为管理后台，其余为用户端页面。
- **响应式策略**：采用 Tailwind 断点（`md:`、`hidden md:inline` 等）配合 `matchMedia('(max-width: 767px)'` 在 JS 中判断移动端，从而在 `<768px` 时把 Sider 替换为左侧 Drawer，并在打开 Drawer 时锁住 `html/body` 滚动（含 iOS Safari 的 `overscrollBehavior` 处理）。
- **暗色模式**：通过 `useTheme()` 将 `dark` 类写入 `documentElement`，组件内统一用 `dark:bg-*`、`dark:text-*` 覆盖颜色；AntD 组件则依赖 `ConfigProvider` 注入的 `algorithm` 自动适配。
- **色彩规范**：全局主色固定为 `#1677FF`（AntD 默认科技蓝），未做自定义扩展；文本与背景色走 Tailwind 灰度族（`gray-50/100/.../900`）。
- **组件复用**：通用交互封装为 `components/` 下的 Vue 单文件组件（如 `ConfirmModal`、`CaptchaWidget`、`TriStateList`、`MarkdownView`），避免在各视图中重复实现。

## 4. 约定与约束

- **样式写法**：所有视觉样式通过 Tailwind 原子类完成，不编写业务级 CSS；仅在 `style.css` 中保留必要的 reset。
- **暗色模式必须显式声明**：AntD 组件在非 `ConfigProvider` 包裹区域（如 Drawer 标题）需手动添加 `dark:text-*` 类，否则不会跟随主题。
- **移动端导航**：宽度 `<768px` 时必须使用 Drawer 替代 Sider，且打开抽屉时需同时设置 `html` 与 `body` 的 `overflow:hidden` 与 `overscroll-behavior: contain`，防止滚动穿透。
- **主题持久化**：主题状态保存在 `localStorage('theme')`，首次加载即恢复上次选择。
- **组件库按需导入**：每个 `.vue` 文件从 `ant-design-vue` 单独 import 所需子组件，不使用全量引入。
- **图标来源**：统一使用 `@ant-design/icons-vue` 的 SVG 图标组件，不在模板中手写 SVG。
- **布局间距**：管理后台内容区统一使用 `p-6` 内边距 + `bg-white dark:bg-gray-800 rounded-lg` 卡片容器，保持视觉一致。

该仓库的前端风格由 Tailwind CSS 原子类 + Ant Design Vue 组件库 + 基于 class 模式的暗色主题三者协作构成，所有页面遵循同一套布局骨架、响应式断点与主题切换约定。