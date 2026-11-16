# Issue21.md — 本地调试问题记录

> **文档定位：** 记录本轮本地调试中发现的问题，以及 R35-01、R35-02 的根因研究与验证结果；当前均未实施修复。已归档的问题记录见 [docs/reports/Issue/](docs/reports/Issue/)，项目指令见 [AGENTS.md](AGENTS.md)。
>
> **当前状态（2026-11-16）：** R35-01 根因已确认，修复方案待决策；R35-02 根因已确认，阻断口径已由用户确认，尚未修复。本轮仅完成测试、研究与文档记录，未修改项目代码、配置、依赖或锁文件，也未将问题标记为已修复。

## 一、问题记录

### R35-01 项目多处界面 UI 粘连

- **现象（原始记录，保留）：** 多处页面的界面元素间距异常，出现粘连：
  - 「认证方式」阶段：步骤条与下方卡片紧贴，「快速开始」「高级配置」「导入已有配置」区域相互紧贴。
  - 「确认」阶段：步骤条与确认卡片紧贴，警告提示与下方「返回」「确认完成」按钮紧贴。
  - 「完成」阶段：「抢注风险提示」与下方「前往登录」按钮紧贴。
  - 节点管理：从订阅装配的构建前置条件进入节点管理时，顶部提示横幅与下方「节点管理」标题及「批量导入」「新建节点」操作区域紧贴（用户截图）。
  - 新建节点弹窗：顶部切换配置提示与下方「基本信息」标题紧贴。
  - 面板配置：「身份与访问」中的 OIDC 配置、OIDC 启用规则和本地认证卡片相互紧贴；「外观与内容」中的站点信息与公告及页脚卡片紧贴，公告提示与下方首页公告标签紧贴；「运行与安全」中的运行模式、高级模式与速率限制卡片相互紧贴；「数据管理」中的配置导入/导出与备份下载卡片紧贴。
  - 新建平台：scheme 列表说明提示与下方输入行紧贴。
  - 创建规则弹窗：名称输入框与下方平台选择框紧贴，平台选择框与 scheme 说明紧贴，scheme 输入行与「添加 scheme」按钮紧贴。
- **影响范围（原始记录，保留）：** 已发现首次配置、节点管理、面板配置、平台及规则界面受影响；项目内其他界面与状态仍需继续检查，不能据此认定已覆盖全部。
- **复现入口（原始记录，保留）：** 本地 Production 模式，访问 `/setup`、`/admin/nodes`、`/admin/settings`、`/admin/platforms/new` 和 `/admin/rules`，按上述场景查看。
- **本次检查范围与限制（原始记录，保留）：** 桌面浏览器当前视口下查看了管理概览、订阅、分享、平台、规则、订阅装配主要页签、节点、用户、审批中心、面板配置各分区、日志各页签和用户首页，以及新建节点、批量导入节点、新建用户、新建订阅、创建分享、创建规则弹窗和新建平台页面。检查基于当前本地数据与可见状态；未覆盖所有表单分支、弹窗、数据状态、屏幕尺寸、主题及需要额外数据或外部服务的流程。
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本轮仅记录，未调查根因或修复。
- **当前状态（2026-11-16 根因研究后）：** 根因已确认，尚未修复。结论为 Tailwind CSS 4 的分层工具类与 Ant Design Vue CSS-in-JS 未分层组件重置样式之间的级联优先级冲突；详见下文。本节只表示研究与验证完成，不代表问题已修复。

#### 1. 结论摘要

- 本轮在**当前 HEAD 源码的 Vite 开发模式**（管理页卡片）和**重新构建的 Production 产物**（下文列出的全部页面）上均复现了间距为零；两者在共同覆盖的页面上表现一致。
- 根因不是缺少 CSS 类、也不是组件结构错误，而是 **CSS Cascade Layers 的优先级**：Tailwind CSS 4 将 `mb-*`、`space-y-*` 等工具类放入 `@layer utilities`，而 Ant Design Vue 4 通过 CSS-in-JS 注入的组件重置样式（例如 `.ant-card`、`.ant-alert`、`.ant-steps`）**没有 layer**，并包含 `margin: 0`。未分层普通声明优先于分层普通声明，因此工具类的 `margin-bottom` 被覆盖。
- Tailwind 4 的 `space-y-*` 还额外使用零特异性的 `:where(...)`，并且只给“前一个子项”设置 `margin-bottom`；当这个子项是 Ant Design 组件、其 margin 被重置为 0 时，间隙完全消失。这是第二个叠加因素。
- 已记录的多数粘连点都能由同一根因解释；“scheme 输入行与添加按钮”两处实测为 4px／8px 的正常工具类间距，并未复现为 0，见第 6 节。
- 该问题随 Tailwind 4 迁移提交 `1903c58`（R34-01）进入 `v4.1.3`／`v4.1.4` 代码线；`v4.1.2` 源码使用 Tailwind 3，工作区中的旧 Tailwind 3 构建对照正常（未单独构建 `v4.1.2` 完整实例）。**未运行已发布的 `v4.1.3`／`v4.1.4` 镜像**，因此发布镜像是否同样受影响只按源码判断，未实测。
- 本轮**未修改任何项目代码、配置、依赖或锁文件**，也未实施修复。

#### 2. 复现环境与代码版本

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `0ac8160`（`git describe` 为 `v4.1.4-1-g0ac8160`）；最后一个代码提交为 `1ceea52` |
| 工作区状态 | 研究开始时仅有未跟踪的 `Issue21.md`；无其他未提交或未跟踪的工作区改动 |
| 前端依赖 | Vue 3.5.44、Vite 7.3.7、Tailwind CSS 4.3.3、`@tailwindcss/vite` 4.3.3、`ant-design-vue` 4.2.6 |
| 浏览器 | Google Chrome 157.0.8103.52（系统安装版本） |
| 自动化驱动 | Playwright 1.57.0（使用本机已安装 Chrome，无新增项目依赖） |
| 运行时 | Node.js 26.7.0；隔离 Go 服务以 `APP_MODE=prod` 启动，仅监听回环地址 |
| 视口与主题 | 1280×800、920×785、375×785；浅色与深色 |

**构建与产物。** 本轮使用了两套当前代码产物，并分别验证：

| 产物 | 说明 | 关键哈希 |
|------|------|----------|
| 工作区已有 `frontend/dist` | 时间戳 2026-11-11 14:44；来源未在仓库中记录 | `index.html` `ef856972…`；主 CSS `index-CjftUC95.css` `4a2baa1f…`；主 JS `index-DXsnELts.js` `513eed54…` |
| 当前 HEAD 临时重建 | 2026-11-16 在 `/tmp` 复制当前前端源码并执行 `npm run build`，构建成功 | `index.html` `b48d9977…`；主 CSS `index-1A87CTUv.css` `3e3479eb…`；主 JS `index-DWiJKb6p.js` `d1334480…` |

工作区已有产物的哈希与 [Issue20.md](docs/reports/Issue/Issue20.md) R34-01 表格记录的“最终候选”哈希不同，因此不能把该目录直接视为 Issue20 记录的同一份产物；它和临时重建产物都复现了问题。两者均使用 Tailwind 4.3.3 和同一套 `node_modules`。

**本地运行服务说明。** 研究时工作区已有以下进程：`127.0.0.1:5173` 的 Vite 开发服务器，以及 `127.0.0.1:8080` 的 Go 服务。8080 当前嵌入的是 `backend/web/dist` 中 2026-08-25 的旧产物（`index-48_-me5R.js`、`index-Cp6f_GO5.css`，Tailwind 3 时代），实测其面板配置卡片间隙为 16px，**不能复现**本问题。本轮因此没有使用这两个进程作为结论依据，而是启动隔离端口：

- 18090：复制本地测试数据库的当前 Production 构建，用于 `/admin/*` 页面；
- 18092：全新临时数据目录的 Production 构建，用于 `/setup`；
- 18094：当前 HEAD 临时重建产物的 Production 构建，用于交叉复核；
- 18093：使用旧 `backend/web/dist`（Tailwind 3）的对照实例，用于 `/setup` 对照。

所有实例仅监听回环地址，数据目录均在 `/tmp`；没有操作生产、预发布、外部服务、真实账号或真实数据。

#### 3. 复现步骤

1. 以当前 HEAD 启动 Vite 开发服务器，或在临时副本中执行 `npm run build` 后用当前源码构建 Go 服务，设置 `APP_MODE=prod`、临时 `DATA_DIR`。
2. 在全新临时数据目录访问 `/setup`，查看「认证方式」阶段；点击「下一步」查看「确认」阶段；在临时库中完成快速开始后查看「完成」阶段。完成操作只写入临时数据。
3. 使用合成管理员登录隔离实例，访问 `/admin/settings`，依次切换「身份与访问」「通知」「外观与内容」「运行与安全」「数据管理」。
4. 访问 `/admin/nodes`。为复现「从订阅装配进入」的顶部横幅，先阅读 `ContextBar.vue` 与 `AssemblyView.vue` 的 `saveDraftForPreflight()`，再在隔离浏览器写入形状相同的 `assembly_ctx_v1` 会话草稿并导航到 `/admin/nodes`；这复现的是同一个 `ContextBar` 分支，不是通过真实点击装配预检按钮重走，见第 8 节限制。
5. 在 `/admin/nodes` 点击「新建节点」；在 `/admin/platforms/new` 查看 scheme 区域；在 `/admin/rules` 点击「创建规则」；在登录页提交错误密码查看错误提示。
6. 用 DevTools/Playwright 读取 `getComputedStyle` 与 `getBoundingClientRect()`，并用临时注入样式做对照实验；临时样式只存在于浏览器会话，刷新即失效，未写入项目文件。

#### 4. 根因分析

##### 4.1 直接原因：Tailwind 4 分层工具类被未分层的 Ant Design 重置覆盖

当前 `frontend/src/style.css` 第 1 行为 `@import "tailwindcss";`。构建后的主 CSS 包含：

```css
@layer theme, base, components, utilities;
...
@layer utilities {
  .mb-6 { margin-bottom: calc(var(--spacing) * 6); }
  ...
}
```

Ant Design Vue 4 使用 CSS-in-JS，在运行时向 `<head>` 注入 `<style>`；这些样式**不带 `@layer`**，例如：

```css
:where(.css-lyt5dk).ant-steps { margin: 0; padding: 0; ... }
:where(.css-lyt5dk).ant-card   { margin: 0; padding: 0; ... }
:where(.css-lyt5dk).ant-alert  { margin: 0; padding: 0; ... }
:where(.css-lyt5dk).ant-input  { margin: 0; padding: 0; ... }
:where(.css-lyt5dk).ant-select { margin: 0; padding: 0; ... }
:where(.css-lyt5dk).ant-tabs   { margin: 0; padding: 0; ... }
```

这些 selector 中的 `css-lyt5dk` 是构建/运行时哈希，实际值随构建变化；关键是“未分层 + `margin: 0`”。

按照 CSS Cascade Layers 规则，**未分层的普通声明优先于任何分层的普通声明**。因此：

- `<Card class="mb-4">` 的 `mb-4` 在 `@layer utilities` 中，`.ant-card { margin: 0 }` 不在 layer 中，后者生效；
- `<Steps class="mb-6">`、`<Alert class="mb-4">`、`<Tabs class="mb-4">` 同理；
- `space-y-*` 设置在 Ant Design 子项上的 margin 也被同一个重置覆盖。

这与源码中“类已经写了、构建产物里类也存在”的现象一致：问题发生在**级联优先级**，不是类缺失。

##### 4.2 叠加因素：Tailwind 4 的 `space-y-*` 使用零特异性 `:where()` 且只设置前一个子项的下外边距

当前构建中 `space-y-4` 的实际规则是：

```css
:where(.space-y-4 > :not(:last-child)) {
  margin-block-start: calc(... * var(--tw-space-y-reverse));
  margin-block-end: calc(... * calc(1 - var(--tw-space-y-reverse)));
}
```

反向变量为 0 时，它只给“除最后一个外”的子项设置 `margin-bottom`，且整个 selector 被 `:where()` 包裹，特异性为 0。后果是：

- 如果前一个子项是 `.ant-card`／`.ant-alert` 等带 `margin: 0` 的组件，这个 `margin-bottom` 被覆盖，间隙变成 0；
- 后一个子项本身没有 `margin-top`，所以即使后一个是普通 `<div>`，也无法补回间隙。面板配置「公告提示 → 首页公告标签」正是这种情况；
- 即使把工具类移出 `@layer`，只要仍保留 `:where()`，零特异性仍会输给 `.ant-card` 的 0,1,0 特异性。因此修复必须同时处理“分层”与“零特异性/写法”两个层面。

##### 4.3 为什么 Tailwind 3／旧构建没有该问题

`v4.1.2` 的 `frontend/src/style.css` 使用 `@tailwind base/components/utilities`，Tailwind 3 构建中：

- `mb-4` 等工具类不在 `@layer` 中；
- `space-y-4` 生成的是 `.space-y-4 > :not([hidden]) ~ :not([hidden]) { margin-top: ... }`，特异性为 0,3,0，且作用于“后一个子项”。

旧构建中 Ant Design 注入的 `<style>` 位于 Tailwind `<link>` 之前，同特异性下后加载的样式生效；`space-y` 的较高特异性也足以覆盖组件的 `margin: 0`。因此旧构建的面板卡片间隙为 16px、步骤条下间距为 24px，表现正常。

##### 4.4 引入版本

Tailwind 4 迁移提交为 `1903c58`（“升级前端依赖修复 GitHub Actions 依赖审计失败 R34-01 并迁移 Tailwind CSS 4”）。`git merge-base --is-ancestor` 核对：

- `v4.1.2` **不包含** `1903c58`；
- `v4.1.3` 与 `v4.1.4` **包含** `1903c58`。

因此，就源码与本地构建而言，回归从 Tailwind 4 迁移开始，并进入 `v4.1.3`／`v4.1.4`。没有证据表明它由后端、API、数据库或页面模板结构变化引起。

##### 4.5 关键文件位置

| 文件 | 相关内容 |
|------|----------|
| `frontend/src/style.css:1` | `@import "tailwindcss";`，Tailwind 4 分层入口 |
| `frontend/src/style.css:31` | 项目自定义 `@layer base`（仅补边框色，不是本次冲突来源） |
| `frontend/vite.config.ts:3,7` | `@tailwindcss/vite` 插件接入 |
| `frontend/package.json` | `tailwindcss` / `@tailwindcss/vite` 4.3.3、`ant-design-vue` 4.2.6 |
| `frontend/node_modules/ant-design-vue/es/style/index.js:10-12` | `resetComponent()` 中的 `margin: 0` |
| `frontend/node_modules/ant-design-vue/es/card/style/index.js:207` | Card 组件样式合并 `resetComponent` |
| `frontend/node_modules/ant-design-vue/es/steps/style/index.js:228` | Steps 组件样式合并 `resetComponent` |
| `frontend/node_modules/ant-design-vue/es/alert/style/index.js:30` | Alert 组件样式合并 `resetComponent` |
| `frontend/src/views/SetupView.vue:180-285` | Steps `mb-6`、Card `mb-4`、Alert `mb-4` 等受影响类 |
| `frontend/src/views/admin/SettingsView.vue:921` | `.flex-1.space-y-4` 卡片容器；公告区在 `:1236` 附近 |
| `frontend/src/components/ContextBar.vue:27` | 节点管理顶部 `Alert.mb-4` |
| `frontend/src/views/admin/NodesView.vue:1132-1141` | 新建节点表单 `space-y-5` 与 `node-reset-warning mb-2` |
| `frontend/src/views/admin/PlatformEditView.vue:169-178` | scheme Alert `mb-2`、输入行 `mb-2`、添加按钮 |
| `frontend/src/views/admin/RulesView.vue:276-302` | 创建规则弹窗 `space-y-3`、scheme 行 `mb-1`、添加按钮 |

`node_modules` 下的路径对应当前本地安装版本，不是仓库跟踪文件；行号用于说明重置来源。

#### 5. 支持证据与对照实验

##### 5.1 直接测量（当前 Production 构建）

| 位置 | 测量 | 结果 |
|------|------|------|
| `/setup` 认证方式阶段 | `.ant-steps` 的 `margin-bottom`；步骤条底边到第一张内层卡片顶边 | `0px`；`0px`（两者坐标均为 329.5） |
| `/setup` 认证方式阶段 | 三张选项卡片的 `margin-bottom`／卡片底边到下一卡片顶边 | `0px`；相邻卡片均为 0 间隙 |
| `/setup` 确认阶段 | 警告 Alert 的 `margin-bottom`；Alert 底边到「返回」「确认完成」按钮顶边 | `0px`；`0px`（均为 555） |
| `/setup` 完成阶段 | 「抢注风险提示」Alert 底边到「前往登录」按钮顶边 | `0px`（均为 631.6） |
| `/admin/settings` 身份与访问 | `.flex-1.space-y-4` 第一、第二张卡片的相邻间隙 | `0px`（第一张底边与第二张顶边均为 331） |
| `/admin/settings` 外观与内容 | 公告警告 Alert 底边到下一「首页公告」区域顶边 | `0px`（均为 457） |
| `/admin/settings` 运行与安全 | 速率限制区 `<InputNumber class="w-full mt-1">` 的 `margin-top` | `0px`（期望 4px） |
| `/admin/nodes`（带 ContextBar 草稿） | 顶部提示横幅底边到页面 `h1` 顶边 | `0px`（均为 209.1） |
| 新建节点弹窗 | 顶部切换配置提示 Alert 底边到「基本信息」`section` 顶边 | `0px`（均为 234） |
| `/admin/platforms/new` | scheme 说明 Alert 底边到第一行输入行顶边 | `0px`（均为 641） |
| `/admin/platforms/new` | scheme 输入行底边到「添加 scheme」按钮顶边 | `8px`（正常 `mb-2`，**未复现为 0**） |
| `/admin/rules` 创建规则弹窗 | 名称 Input 底边到平台 Select 顶边；平台 Select 底边到 scheme 说明顶边 | 两处间隙均为 `0px`（对应坐标 204、236） |
| `/admin/rules` 创建规则弹窗 | scheme 输入行底边到「添加 scheme」按钮顶边 | `4px`（正常 `mb-1`，**未复现为 0**） |
| 登录页错误提示 | 错误 Alert 底边到「登录」按钮顶边 | `0px`（均为 595；额外发现，不在原始清单） |

##### 5.2 浏览器内临时对照实验（不改项目文件）

| 实验 | 操作 | 结果 |
|------|------|------|
| A. 同层追加 | 在页面中追加 `@layer utilities { .mb-6 { margin-bottom: 1.5rem; } }` | `.ant-steps` 的 `margin-bottom` 仍为 `0px` |
| B. 未分层追加 | 追加未分层的 `.mb-6 { margin-bottom: 1.5rem; }` | `margin-bottom` 变为 `24px`，步骤条与卡片恢复 24px 间距 |
| C. 删除 Ant Design 重置 | 在浏览器中删除命中 `.ant-steps` 且 margin 为 0 的 15 条规则 | `.mb-6` 立即变为 `24px` |
| D. `space-y` 同层追加 | 追加 `@layer utilities { .space-y-4 > :not(:last-child) { margin-bottom: 1rem; } }` | 面板卡片间隙仍为 `0px` |
| E. `space-y` 未分层追加 | 追加未分层的 `.space-y-4 > :not(:last-child) { margin-bottom: 1rem; }` | 间隙变为 `16px` |
| F. 未分层但保留 `:where()` | 追加未分层的 `:where(.space-y-4 > :not(:last-child)) { margin-bottom: 1rem; }` | 间隙仍为 `0px`，说明零特异性单独也会导致失败 |
| G. `!important` | 追加 `.space-y-4 > :not(:last-child) { margin-bottom: 1rem !important; }` | 间隙恢复为 `16px` |
| H. 普通元素对照 | 动态创建普通 `div.mb-4` 与普通 `div.space-y-4` | `margin-bottom` 与子项间隙均为 `16px`，工具类本身有效 |

所有实验均在浏览器会话中完成，样式仅临时注入；未写入 `frontend/src`、构建产物或配置文件。

##### 5.3 旧构建对照

在 18093 实例上使用 `backend/web/dist` 中 2026-08-25 的旧产物（Tailwind 3）访问同一 `/setup`：

- `.ant-steps.mb-6` 的 `margin-bottom` 为 `24px`，步骤条底边到第一张卡片的间隙为 `24px`；
- 选项卡片 `mb-4` 的 `margin-bottom` 为 `16px`，卡片之间为 `16px`；
- 确认阶段警告 Alert 的 `margin-bottom` 为 `16px`，Alert 与按钮间距为 `16px`。

这是同页面的 A/B 对照，直接说明问题来自前端构建/样式层，而不是页面模板或后端渲染。

##### 5.4 视口与主题矩阵

在 1280×800、920×785、375×785 三种视口下，浅色和深色各测一次：

- `/setup` 认证方式阶段步骤条 `margin-bottom` 均为 `0px`，步骤条到第一张卡片间隙均为 `0px`；
- `/setup` 确认阶段 Alert 到按钮间隙均为 `0px`；
- `/admin/settings` 面板配置卡片相邻间隙均为 `0px`；
- `/admin/settings` 的页面根节点水平溢出为 `0px`；其余页面未逐一测量横向溢出，不能据此认定全站无溢出。

明暗主题只改变颜色变量，不改变上述布局结果；本问题在桌面和窄屏上一致存在。

##### 5.5 当前 HEAD 临时重建与 Vite 开发模式

- 在当前 HEAD 的临时重建 Production 产物（18094）上，`/admin/settings` 卡片间隙为 `0px`，`/admin/platforms/new` Alert 到输入行间隙为 `0px`；
- 在工作区现有 Vite 开发服务器（5173，当前源码）上，`/admin/settings` 的卡片 `margin-top`／`margin-bottom` 与卡片间隙同为 `0px`。

因此问题不是仅存在于某个旧 `dist` 目录，而是当前源码与当前依赖组合的可复现结果。

#### 6. 各已记录现象与根因的关系

| 已记录现象 | 本轮实测 | 判定 |
|------------|----------|------|
| 认证方式：步骤条与卡片、三张选项卡片互相紧贴 | 步骤条 `mb-6` 被覆盖，卡片 `mb-4` 被覆盖，间隙 0 | 同一根因（`mb-*` + `.ant-*` margin 重置） |
| 确认：步骤条与确认卡片、Alert 与按钮紧贴 | 步骤条 `mb-6` 为 0，Alert `mb-4` 为 0，Alert→按钮 0 | 同一根因 |
| 完成：抢注提示与「前往登录」紧贴 | Alert `mb-4` 为 0，Alert→按钮 0 | 同一根因 |
| 节点管理：ContextBar 提示横幅与标题紧贴 | Alert `mb-4` 为 0，Alert→`h1` 0 | 同一根因 |
| 新建节点弹窗：顶部提示与「基本信息」紧贴 | Alert `mb-2` 为 0，Alert→FormSection 0 | 同一根因 |
| 面板配置：OIDC／OIDC 规则／本地认证卡片互相紧贴 | 外层 `space-y-4` 的卡片 `margin-bottom` 为 0 | 同一根因（`space-y` + `.ant-card` 重置） |
| 面板配置：站点信息／公告页脚卡片互相紧贴 | 同上，卡片间隙 0 | 同一根因 |
| 面板配置：公告提示与首页公告标签紧贴 | 外层 `space-y-3` 的前一个 Alert `margin-bottom` 为 0，下一普通 `div` 无 `margin-top` | 同一根因（`space-y` 的“前一子项 margin-bottom”写法） |
| 面板配置：运行模式／高级模式／速率限制卡片紧贴 | 卡片间隙 0；速率限制 `InputNumber.mt-1` 为 0 | 同一根因 |
| 面板配置：导入导出／备份下载卡片紧贴 | 卡片间隙 0 | 同一根因 |
| 新建平台：scheme 说明提示与输入行紧贴 | Alert `mb-2` 为 0，Alert→第一行 0 | 同一根因 |
| 新建平台：输入行与「添加 scheme」按钮紧贴 | 实测 8px，为正常 `mb-2` | **未复现为 0**；不属于同一失败，可能是视觉阈值或截图状态差异 |
| 创建规则：名称输入框与平台选择框紧贴 | Input `space-y-3` 的 margin-bottom 被重置，Input→Select 0 | 同一根因 |
| 创建规则：平台选择框与 scheme 说明紧贴 | Select 的 margin-bottom 被重置，Select→普通 div 0 | 同一根因 |
| 创建规则：scheme 输入行与「添加 scheme」按钮紧贴 | 实测 4px，为正常 `mb-1` | **未复现为 0**；不属于同一失败 |
| 额外发现：登录页错误 Alert 与登录按钮紧贴 | Alert `mb-4` 为 0，Alert→按钮 0 | 同一根因，原始清单未列出 |
| 额外发现：SMTP 表单中 Select／Checkbox.Group／Input 后的 4px 间距消失 | `space-y-1` 的前一个 Ant Design 组件 margin-bottom 为 0 | 同一根因，原始清单未列出 |

**结论：** 原始清单中除两处「输入行→添加按钮」实测为正常 4px／8px 外，其余均已复现为 0，并可由同一根因解释；目前没有证据表明存在第二个独立的间距问题。两处 4px／8px 是否仍让用户觉得“紧”，属于视觉接受度问题，本轮不据此认定缺陷。

#### 7. 影响范围与实际检查边界

**已实际检查（本轮）：**

- `/setup`：认证方式、确认两个阶段覆盖 1280／920／375 视口与浅色／深色；完成阶段在 1280×900 浅色下测量（未做完成阶段的全部视口／主题矩阵）；
- `/admin/settings` 六个分组：身份与访问、通知、外观与内容、运行与安全、数据管理、危险操作（危险操作默认为折叠空态）；
- `/admin/nodes`：普通列表、ContextBar 顶部提示、新建节点弹窗；
- `/admin/platforms/new`：scheme 区域；
- `/admin/rules`：创建规则弹窗；
- 登录页错误提示；
- 对 `/admin`、订阅、分享、平台、规则、装配、节点、用户、审批、设置、日志、用户首页、用户规则页和用户资料页做了可见 DOM 扫描；扫描结果受本地数据为空的状态限制。

**按源码静态检索（未逐项实际渲染）：**

- 约 99 处 `space-y-*` 使用；其中只要子项包含 Ant Design 组件，就可能出现“前一个组件 margin-bottom 被重置”的间隙丢失；
- 110 处把间距或布局工具类直接写在 Ant Design 组件标签上的用法，涉及 28 个 `.vue` 文件（正则静态检索，可能含少量误报）；
- 代表文件包括：`ContextBar.vue`、`SetupView.vue`、`SettingsView.vue`、`PlatformEditView.vue`、`RulesView.vue`、`NodesView.vue`、`NodeCheckPanel.vue`、`OpenVPNImportPanel.vue`、`PlatformHeadersEditor.vue`、`AssemblyView.vue`、`AssemblerShell.vue`、`GroupsView.vue`、`LogsView.vue`、`XrayInstancesView.vue`、`HomeView.vue`、`LoginView.vue`、`ProfileView.vue`、`RegisterView.vue`、`ResetView.vue`、`OidcCallbackView.vue` 等。以上仅为静态检索，不代表全部已复现。

**未覆盖或覆盖不足：**

- 未运行已发布的 `v4.1.3`／`v4.1.4` Docker 镜像，也未运行 Docker Compose；
- 未覆盖 Xray 实例、高级模式实际页面、应急页、OIDC 真实提供商／mock 登录、验证码、SMTP 实际发送、备份下载等需要额外数据或外部服务的流程；
- 未覆盖所有表单分支、所有弹窗（批量导入节点、新建用户、新建订阅、创建分享等只做了源码检查，未逐一测量）、所有数据状态、所有屏幕尺寸（含 320px 和浏览器缩放）、所有浏览器（仅 Chrome 157）、移动端抽屉形态和辅助技术；
- 未进行真实生产数据或真实账号操作；所有服务、数据库和账号均为本地隔离临时环境；
- 未验证真实点击「前往节点管理」路径：ContextBar 使用等价会话草稿注入复现，`saveDraftForPreflight()` 与 `ContextBar.vue` 的读取条件已对照检查，但未重走完整的装配预检点击流程。

#### 8. 未确认内容与阻碍

1. **Issue20 R34-01 的“174/174 通过”与本次复现的关系未确认。** R34-01 记录双产物 29 路由 × 三视口 × 明暗的视觉矩阵通过；本次在同一代码线的当前构建上测得明显 0 间隙。工作区 `.ui-check` 当前未找到 `r34-20261112` 证据目录，无法复核其断言方式。较合理的推测是：该矩阵主要检查路由可达、无报错、无横向溢出和主题可见性，未对组件间 computed gap 做断言；但这只是推测，未获证据确认。
2. **发布镜像是否同样受影响未实测。** 从源码和本地 Production 构建看可能性很高，但没有拉取／运行 `v4.1.3`／`v4.1.4` 镜像；不能把“源码已复现”等同于“发布镜像已复现”。
3. **工作区已有 `frontend/dist` 的来源未确认。** 其哈希与 Issue20 记录不同；本轮用当前 HEAD 临时重建产物交叉复核，结论一致，但无法证明用户截图对应的是哪一次构建。
4. **两处「输入行→添加按钮」的实际视觉感受未确认。** 实测 8px（新建平台）和 4px（创建规则），属于正常工具类；用户仍可能认为偏紧，但是否需要调整不在本轮研究范围。
5. **是否存在未覆盖的第二个独立问题未确认。** 本轮只覆盖了列出的和少量额外发现的状态；高级模式、Xray、应急、OIDC 等未实测区域不能据此排除其他独立问题。
6. **浏览器差异未验证。** 结论基于 Chrome 157 和 CSS Cascade Layers 标准行为；未验证 Safari、Firefox、旧浏览器或浏览器缩放下的表现。

#### 9. 候选修复方向（待决策，未实施）

以下仅记录方向，均未编写或应用修复代码，也不代表已确认方案。

| 方向 | 思路 | 主要风险／待验证点 |
|------|------|------------------|
| A. 将 Ant Design 样式纳入 layer 并调整顺序 | 让 CSS-in-JS 生成的 Ant Design 样式进入 `antd` layer，并声明 `theme, base, antd, components, utilities` 等顺序，使 Tailwind utilities 位于其后 | 当前 `ant-design-vue` 4.2.6 未看到可直接使用的 `StyleProvider layer` 入口，需要自定义注入或其他机制；会影响全部 Ant Design 组件、浮层和动态主题，必须做全量视觉回归 |
| B. 提供未分层的间距兼容覆盖 | 在 Tailwind 之后追加未分层的针对性规则，或使用 `!important` 变体恢复受影响间距 | 需要枚举组件与工具类组合，维护成本高；`!important` 会污染级联，可能影响其他状态；必须验证不破坏 Ant Design 内部布局 |
| C. 调整受影响页面结构 | 用普通 `div` 包裹 Ant Design 组件、改用 `Space`／inline style 等承载间距 | 改动面大且容易遗漏新代码；只能解决已改位置，不能阻止后续继续写出同样失效的类；需要逐页验证 |
| D. 回退 Tailwind 3／撤销 R34-01 | 回到旧工具类输出模型 | 与 R34-01 的依赖审计目标冲突，且会重新引入旧依赖风险；不建议作为默认方向 |

**后续验证建议（若用户决定修复）：**

- 用真实 Production 构建和至少 3 个视口 × 2 个主题，对 `/setup`、`/admin/settings`、`/admin/nodes`、`/admin/platforms/new`、`/admin/rules` 及登录错误态断言 computed `margin-bottom`／相邻 gap，而不是只截图或只检查 jsdom 类名；
- 覆盖 ContextBar、新建节点弹窗、创建规则弹窗、FormOverlay／Drawer、Ant Design 浮层和深色主题；
- 增加静态门禁，提示“在 Ant Design 组件根节点上直接写间距工具类”的写法，或统一要求使用安全的包装层；
- 对候选方案 A／B／C 分别做影响评估，确认不会改变 Ant Design 组件内部间距、表格、卡片、弹层和窄屏布局；
- 若只做局部修复，需明确未修复的页面清单，避免把“部分页面恢复”表述为“全局已修复”。

#### 10. 本轮未做

- 未修改任何项目代码、配置、依赖或锁文件；
- 未实施任何候选修复，也未把问题标记为已修复；
- 未操作生产、预发布、外部服务、真实账号或真实数据；
- 未运行 Docker 镜像、远端 CI 或真实客户端连接；
- 唯一持久化变更为本 `Issue21.md` 文档更新。

### R35-02 无素材池时 Clash 构建预检未阻断

- **现象（原始记录，保留）：** 用户反馈不存在规则素材池时，Clash 构建预检没有阻断；截图显示已进入「构建订阅/规则 → Clash - V2Ray/Mihomo（新版）→ 规则素材」步骤，素材池选择项显示为 `0`。
- **预期（原始记录，保留）：** 不存在素材池时，在预检阶段阻断 Clash 构建。
- **复现入口（原始记录，保留）：** 本地 Production 模式，进入 `/admin/assembly?tab=clash-yaml` 的 Clash 构建流程，在不存在素材池的情况下查看预检及规则素材步骤。
- **待确认（原始记录，保留）：** 截图显示能够进入规则素材步骤；后续预览、最终构建及其他条件下的具体行为尚未验证。
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本次仅记录用户反馈和预期，未研究原因或修复。
- **当前状态（2026-11-16 研究后）：** 根因已确认，阻断口径已由用户确认，尚未修复。结论为：页面级预检只覆盖节点、平台和订阅，未覆盖素材池或规则源；用户截图中的 `0` 是无池时「添加素材池」写入的 `pool_id=0` 回退值，不是素材池数量；后端已有零输出门槛，但只在最终生成阶段返回 400。详见下文。

#### 1. 结论摘要

- **直接原因：** Clash 构建的页面级预检是前端 `AssemblyView.vue` 中的 `buildPreflightMissing` 计算属性，只检查可用节点、匹配平台和平台订阅；素材池数量、素材池可用性或手动规则均不在预检条件内。因此仅缺素材池时预检结果为空，装配器正常显示并可进入「规则素材」步骤。
- **并非完全没有阻断：** 后端 `generate` 已有零输出门槛。无素材池且无手动规则时，预览仍可返回空规则并显示警告，但最终生成返回 `400 当前目标没有可输出的非系统规则`。问题是阻断太晚，且未在预检或预览阶段提示清楚。
- **截图中的 `0` 已复现：** 无素材池时点击「添加素材池」，`addPool()` 以 `context.pools[0]?.id ?? 0` 写入 `pool_id=0`，下拉框将原始值显示为 `0`，选项列表为「暂无数据」。进入预览时后端返回 `400 参数错误: 素材池不存在: 0`。这比用户原始描述更具体，说明该 `0` 是一个无效的素材池选择值。
- **空素材池与「无可用目标规则」同样未在预检拦截：** 存在素材池但条目数为 0 时，预检仍为空，预览可成功但回执 `最终输出 0`，`确认生成` 仍可点击，最终生成才返回 400。素材池仅有 Clash 不支持的 `USER-AGENT` 条目时，预览会提示规则已跳过、最终输出 0，生成同样 400。
- **存在可用于 Clash 的素材时正常：** 素材池含 `DOMAIN-SUFFIX,example.com` 时，预览输出该规则，生成成功并创建版本 v1（首个版本自动激活）。
- **无素材池但有一条手动规则时可以成功：** 预览输出手动规则，生成成功并创建版本 v2。现行 Design3 明确允许「素材池规则 + 自定义规则合计至少输出一条」，因此不能简单地把「无素材池」等同于「不可构建」。
- **设计冲突已确认：** 较早的 Design2／Design2-UI 记录「规则为空允许生成（依赖内置兜底）」，较新的 Design3 §7.2 与 Build22 Step 6 则要求「最终非系统规则输出为 0 时禁止生成」。当前实现遵循 Design3 的后端生成门槛，但 Design2／Design2-UI 的空规则文案未同步，属于文档间口径冲突；本轮不改写这些设计文档。
- **已确认的产品口径：** 用户在 2026-11-16 选择「按有效规则源阻断」：保留手动规则；页面级预检不因「无素材池」单独阻断；在规则步骤阻止「无素材池且无手动规则」进入预览；预览 `final_output=0` 时禁用生成并明确提示；修复无池时添加素材池产生 `0`。该口径尚未实施，也不是当前代码的实际行为。
- 本轮只做测试、研究和文档记录，**未修改任何项目代码、配置、依赖或锁文件，未实施修复，也未将问题标记为已修复**。

#### 2. 代码版本、运行环境与测试数据前提

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `7660292`（`git describe` 为 `v4.1.4-2-g7660292`）；最后一个代码提交为 `1ceea52`；研究开始时工作区干净，无未提交或未跟踪改动 |
| 前端依赖 | Vue 3.5.44、Vite 7.3.7、Tailwind CSS 4.3.3、`@tailwindcss/vite` 4.3.3、`ant-design-vue` 4.2.6 |
| 后端/运行时 | Go 1.26.4（darwin/arm64）；Node.js 26.7.0；macOS 27.2 |
| 浏览器/驱动 | Google Chrome 157.0.8103.52；Playwright-core 1.57.0（安装在临时目录，未加入项目依赖） |
| 隔离实例 | 当前 HEAD 源码在 `/tmp/r35-02-clash-preflight` 构建；`APP_MODE=prod`、临时 `DATA_DIR`、`PORT=18100`，仅通过 `127.0.0.1:18100` 访问 |

**构建产物。** 为避免改动工作区，前端与后端均在上述 `/tmp` 目录复制后构建：

| 产物 | 说明 | 关键哈希 |
|------|------|----------|
| 前端 `index.html` | 2026-11-16 12:40 在临时副本执行 `npm run build`，通过软链接引用工作区现有 `node_modules`（未修改其内容） | `330ac0fb45326003e263c6397faebd1e96f012f8a572b90c3f1a6272eba85edd` |
| 主 CSS `index-CFZtmKWH.css` | 同一构建产物 | `3b4626a471f35d7de8e6764bb1566c3ab34697a0ac8c60ea2e7a9c83213c516b` |
| `AssemblyView-kkF8QGCH.js` | 装配页代码分块 | `4b63a7fd37b68604d7adb62ac25191c506e8dfe0b31a062894e3281b43a47bac` |
| Go 服务 `/tmp/r35-02-clash-preflight/server` | 当前 HEAD 后端源码 + 上述前端产物嵌入，`CGO_ENABLED=0 go build` | 54 MB；未运行已发布的 `v4.1.3`／`v4.1.4` 镜像 |

**测试数据前提。** 测试库由仓库内本地测试库 `backend/data/app-prod.db` 通过 `sqlite3 -readonly ... ".backup"` 复制到临时目录；源库未写入或清空。复制时库内已有：

- 合成管理员 `admin@test.com`，密码为 [TestPasswordList.md](docs/Reference/TestPasswordList.md) 规定的统一测试密码；
- 3 个平台、1 个 Clash 订阅（平台 1，订阅 id 1）、1 个手动 VLESS 节点 `HK`、9 个启用预设代理组；
- **0 个规则素材池**，正好对应用户反馈的「不存在规则素材池」状态。

所有素材池创建、条目增删、节点启停、装配版本生成均只写入临时目录中的数据库；没有操作生产、预发布、外部服务、真实账号或真实数据。

#### 3. 复现步骤

1. 将当前 HEAD 的 `frontend/` 与 `backend/` 复制到 `/tmp/r35-02-clash-preflight`，前端执行 `npm run build`，将 `dist` 放入临时后端 `web/dist`，再执行 `CGO_ENABLED=0 go build -o /tmp/r35-02-clash-preflight/server ./cmd/server`。
2. 复制本地测试数据库到临时 `DATA_DIR`，启动隔离服务：`PORT=18100 APP_MODE=prod DATA_DIR=<临时目录> ./server`。
3. 浏览器打开 `http://127.0.0.1:18100/login`，使用 `admin@test.com` 和测试密码登录；进入 `/admin/assembly?tab=clash-yaml`。
4. 在「头部表单」保持默认值，点击「下一步」；勾选手动节点 `HK`；点击「🌎国外流量」的「选择与排序」，勾选 `HK` 并保存；点击「下一步」进入「规则素材」。
5. 观察页面级预检是否出现「构建前缺少以下前置条件」，以及能否进入规则素材、添加素材池和继续下一步。
6. 无素材池状态下分两条路径观察：
   - 不添加素材池，继续「覆盖层 → 预览」，等待自动预览后点击「确认生成」；
   - 在规则素材点击「添加素材池」，观察下拉框显示值，再继续到预览。
7. 在临时库通过管理 API 构造其他状态后刷新页面：创建空素材池；为空池添加手工 `USER-AGENT` 条目；替换为 `DOMAIN-SUFFIX` 条目；或删除全部素材池后改用手动规则行。
8. 对「无素材池 + 一条手动规则」执行同样流程：点击「添加规则行」，填写匹配值 `example.org`，目标选择 `🚀直接连接`，再预览、生成。
9. 对照实验：通过 `PUT /api/admin/nodes/1/toggle` 临时禁用 `HK`，刷新装配页，确认页面级预检对「无可用节点」确实会阻断并隐藏装配器；随后恢复启用。
10. 通过 Playwright 记录页面文本、截图和 `/api/admin/assembly/context`、`/preview`、`/generate` 的请求／响应；浏览器会话中的截图仅保存在临时目录，未加入仓库。

#### 4. 各状态实际结果

| # | 测试状态 | 页面级预检 | 规则素材步骤 | 预览 | 最终生成 |
|---|----------|------------|--------------|------|----------|
| A | 无素材池、无手动规则、不添加池 | 无提示，装配器可见 | 显示「尚未添加素材池」 | 200；警告「未选择任何规则素材池或手动规则，将生成空规则」；正文仅含 `GEOIP,CN,DIRECT` 与 `MATCH,🛟无法归属的流量`；回执最终输出 0 | `确认生成` 可点击；400「当前目标没有可输出的非系统规则」 |
| B | 无素材池、点击「添加素材池」 | 无提示 | 新增一行，池下拉显示 `0`，编辑目标为 `🚀直接连接`；打开下拉显示「暂无数据」 | 自动预览请求 400「参数错误: 素材池不存在: 0」；无预览正文；`确认生成` 禁用并提示「请先刷新预览」 | 未进入生成 |
| C | 存在空素材池（0 条目） | 无提示 | 可添加并选择该池，显示池名 | 200；无「空规则」警告；回执输入 0、最终输出 0；正文仅含系统兜底规则 | `确认生成` 可点击；400「当前目标没有可输出的非系统规则」 |
| D | 素材池仅含 Clash 不支持的 `USER-AGENT` 条目 | 无提示 | 可添加并选择该池 | 200；警告「规则已跳过：USER-AGENT,Telegram（Clash 不支持该规则类型）」；回执输入 1、目标不支持跳过 1、最终输出 0 | `确认生成` 可点击；400「当前目标没有可输出的非系统规则」 |
| E | 素材池含可用于 Clash 的 `DOMAIN-SUFFIX,example.com` | 无提示 | 可添加并选择该池 | 200；正文含 `DOMAIN-SUFFIX,example.com,🚀直接连接`；回执输入 1、最终输出 1 | 200；创建版本 v1，`auto_activated=true` |
| F | 无素材池 + 一条手动规则 `DOMAIN-SUFFIX,example.org` | 无提示 | 手动规则行可添加并选择目标 | 200；正文含 `DOMAIN-SUFFIX,example.org,🚀直接连接`；回执输入 1、最终输出 1 | 200；创建版本 v2，`auto_activated=false`（已有激活版本） |
| G | 对照：禁用唯一节点 `HK` | 出现 1 条「至少一个可用节点」，装配器隐藏 | 无法进入 | 未调用预览 | 未生成 |

**A. 无素材池但未添加池。** 页面没有「构建前缺少以下前置条件」提示，步骤条与装配器均正常显示；预检的 `GET /api/admin/assembly/context` 返回 `pools: []`。预览请求返回 200，`warnings` 只有空规则警告，`receipt` 为 `{input:0,direct_output:0,equivalent_conversions:0,skipped_unsupported:0,target_validation_failed:0,final_output:0}`；点击「确认生成」后 `POST /api/admin/assembly/generate` 返回 400。也就是说，此时并不是「完全没有任何阻断」，而是阻断发生在生成阶段，预览步骤仍可进入。

**B. 无素材池时点击「添加素材池」。** `RulesStep` 的池下拉没有选项，`addPool()` 将 `pool_id` 设为 `0`；`AppSelect` 把 `0` 作为已选值直接显示，因此出现用户截图中的 `0`，下拉展开内容为「暂无数据」。继续进入预览时，`loadPoolEntries(0)` 在 `rule_pools` 中查不到记录，返回 400「参数错误: 素材池不存在: 0」。这条路径同时验证了：用户截图中的 `0` 更可能是无效池选择值，而不是「0 个素材池」的计数展示。

**C. 存在素材池但没有条目。** 通过 API 创建 `R35-02 空素材池`（`entry_count=0`）后，规则素材步骤可正常添加并选择该池，不会出现 `0`。预览返回 200，但因为 `in.Pools` 非空，`Warnings()` 不会追加「未选择任何规则素材池或手动规则」警告；回执明确显示最终输出 0。`canGenerate` 只检查预览正文、预览哈希和过期状态，不检查回执最终输出，因此「确认生成」仍可点击，随后才收到 400。

**D. 素材池存在、条目对 Clash 不可用。** 向空池加入一条手工 `USER-AGENT,Telegram` 后，`entry_count=1`，但 Clash 渲染会将其跳过。预览响应包含跳过提示和 `skipped` 项，回执最终输出 0；生成仍返回 400。该状态说明「素材池存在」或 `entry_count>0` 都不等于「存在可用于 Clash 的素材」。

**E. 存在可用于 Clash 的素材。** 将条目替换为 `DOMAIN-SUFFIX,example.com` 后，预览正文出现目标规则，回执最终输出 1；生成返回 200，创建订阅版本 v1 并自动激活。该路径证明素材池与后端渲染链路本身工作正常，问题限定在「无可用规则源时的前置提示与阻断时机」。

**F. 无素材池但存在手动规则。** 删除全部素材池后，在规则素材步骤添加一条手动规则 `DOMAIN-SUFFIX,example.org`，目标选择 `🚀直接连接`。预览与生成均成功，创建版本 v2（已有激活版本，故不自动激活）。这与 Design3「素材池规则 + 自定义规则合计至少输出一条」的口径一致，也说明「无素材池即页面级阻断」会破坏一条现行可成功的路径。

**G. 对照：无可用节点。** 临时禁用 `HK` 后刷新装配页，页面出现「构建前缺少以下前置条件：至少一个可用节点」并隐藏装配器；恢复启用后不再出现。这说明预检机制本身有效，问题不是预检完全不工作，而是条件集合中没有素材池／规则源。

#### 5. 前后端调用链与相关文件

**实际调用链：**

```text
/admin/assembly?tab=clash-yaml
  → GET  /api/admin/assembly/context            返回 nodes / platforms / subscriptions / pools 等
  → 前端 buildPreflightMissing 计算            只判断节点、平台、订阅
  → Tabs + AssemblerShell                      预检为空时显示五步装配器
  → 规则素材 RulesStep                         纯前端表单；pools 为空时 addPool 写入 0
  → POST /api/admin/assembly/preview           loadData → validate → render → Warnings/Receipt
  → POST /api/admin/assembly/generate          Render → FinalOutput==0 零输出门槛 → 400
```

页面级预检没有独立的后端预检端点；它只消费 `context` 响应。在进入预览之前，没有任何服务端调用会判断素材池或规则源是否存在。真正的后端校验发生在 `preview` 的 `loadData`／`validate` 和 `generate` 的零输出门槛中；其中 `preview` 被有意允许返回 `final_output=0` 的回执（Design3 与 Build22 Step 6 均记录「预览仍可返回 receipt」）。

| 文件位置 | 作用与已验证事实 |
|----------|------------------|
| `frontend/src/views/admin/AssemblyView.vue:166-183` | `buildPreflightMissing` 仅检查可用节点、匹配平台、平台订阅；没有 `pools`、`entry_count` 或 `custom_rules` 判断 |
| `frontend/src/views/admin/AssemblyView.vue:812-831` | 预检非空时显示「构建前缺少以下前置条件」；预检为空时渲染 `AssemblerShell`，因此无池状态可进入规则素材 |
| `frontend/src/views/admin/AssemblyView.vue:584-586` | `addPool()` 使用 `context.pools[0]?.id ?? 0`；无池时写入 `pool_id=0` |
| `frontend/src/views/admin/assembly/RulesStep.vue:43-60` | 空态只显示「尚未添加素材池」；「添加素材池」按钮不检查选项是否为空；下拉选项来自 `context.pools` |
| `frontend/src/views/admin/AssemblyView.vue:281` | `canGenerate` 只要求预览正文、预览哈希存在且未过期，不检查 `receipt.final_output` |
| `frontend/src/views/admin/assembly/PreviewStep.vue:84-95` | 展示 `previewWarnings` 与六项回执；不负责禁用生成按钮 |
| `frontend/src/views/admin/assembly/AssemblerShell.vue:88-97` | 「确认生成」按钮仅受 `canGenerate` 控制；无 `final_output` 门槛 |
| `frontend/src/api/assembly.ts:68-112` | 前端 `AssemblyContext.pools` 与 `PreviewResponse.warnings/receipt` 契约 |
| `backend/internal/server/assembly.go:64-88` | `GET /api/admin/assembly/context` 通过 `poolSvc.List` 返回素材池摘要；空库时正确返回空数组 |
| `backend/internal/server/assembly.go:95-118` | `preview` 调用 `Service.Preview`，不做零输出拒绝 |
| `backend/internal/server/assembly.go:121-140` | `generate` 调用 `Render` 后执行 `FinalOutput==0` 零输出门槛，返回 400 |
| `backend/internal/assembly/load.go:78-101` | 仅对 `in.Pools` 中选中的池调用 `loadPoolEntries`；未选池时不加载，也不报错 |
| `backend/internal/assembly/load.go:198-205` | 池不存在时返回「素材池不存在: N」；`pool_id=0` 的 400 由此产生 |
| `backend/internal/assembly/validate.go:12-188` | 校验节点、平台、代理组、规则目标与强制组成员；不要求至少一个素材池或手动规则 |
| `backend/internal/assembly/service.go:148-171` | `Warnings` 仅在 `len(in.Pools)==0 && len(in.CustomRules)==0` 时追加空规则警告；空池或仅不支持条目不会触发该文案 |
| `backend/internal/assembly/receipt.go:11-42` | 回执只统计素材池规则与自定义规则，`FinalOutput=DirectOutput+EquivalentConversions`，不含系统兜底 |

#### 6. 根因分析

1. **直接原因是预检条件遗漏。** Clash 构建的页面级「预检」实际上只是 `buildPreflightMissing` 的前端计算，当前实现只拦截缺少节点、平台或平台订阅的装配；它没有把素材池存在性、素材池可用性或规则源纳入。仅缺素材池时该数组为空，装配器自然显示，规则素材步骤也能进入。
2. **「0」是次级 UI 缺陷，不是正常的池计数。** 无池时 `addPool()` 无条件回退到 `pool_id=0`，导致下拉框显示无效 ID；预览随即以 `素材池不存在: 0` 失败。该缺陷放大了用户看到的异常，但不是预检未阻断的唯一原因。
3. **后端已有阻断，但发生在最终生成。** `generate` 在 `FinalOutput==0` 时返回 400，且 Design3／Build22 Step 6 明确冻结了该门槛。问题不是后端允许空规则落库，而是前端在预检、规则步骤和预览阶段没有把「最终输出将为 0」提前暴露；用户直到点击「确认生成」才收到错误。
4. **预览警告条件不完整。** `Warnings()` 只在「未选择任何池且未选择任何手动规则」时提示空规则；当选择了空池或只含不支持规则的池时，`len(in.Pools)>0` 使该警告不出现。因此状态 C 的预览看起来正常，回执虽显示最终输出 0，但没有醒目的空规则提示。
5. **前端生成条件不完整。** `canGenerate` 不读取 `previewReceipt.final_output`，所以即使回执已经显示 0，按钮仍可点击；只有后端 400 才阻止落库。这是「预览已给出信息但界面未据此阻止」的直接表现。
6. **不是以下原因：** 不是 `GET /context` 没有返回 pools（实测返回空数组）；不是素材池列表或 `entry_count` 查询错误（空池、单条目池均符合预期）；不是权限或数据库写入失败；不是后端缺少零输出校验。问题集中在预检条件、无池 UI 回退和预览／生成按钮的联动。

#### 7. 与现行设计的关系及已确认口径

**现行设计存在跨文档冲突：**

- [Design2.md](docs/reports/Design/Design2.md) 第 160 行仍写「规则为空允许生成（见 3.6 兜底规则仍有效）并在预览中提示」；[Design2-UI.md](docs/reports/Design/Design2-UI.md) 第 338、357 行同样记录空规则允许生成。
- 较新的 [Design3.md](docs/reports/Design/Design3.md) §7.2（第 434 行）明确规定「最终输出只统计素材池规则 + 自定义规则，不包含内置兜底；最终输出为 0 时禁止生成」；[Build22.md](docs/reports/Build/Build22.md) 的 Step 6 已按该口径实现并补了 HTTP 回归测试。
- 当前代码实现的是 Design3 的后端生成门槛，但前端预检、预览警告和 `canGenerate` 没有同步 Design3 的「有效输出」概念，仍停留在「进入装配器后由用户自行发现」的状态。Design2／Design2-UI 的空规则文案与当前实现冲突，属于历史文档滞后，本轮未改写。

**用户已确认的产品口径（2026-11-16，「按有效规则源阻断」）：**

1. 保留「无素材池但存在手动规则」的构建路径；不把「无素材池」本身作为页面级预检的硬阻断条件。
2. 页面级预检继续只检查节点、平台和订阅；在 Clash 规则素材步骤，若既没有选中素材池、也没有任何手动规则，则阻止进入预览（具体交互形式待实现设计，例如禁用「下一步」并显示行内提示）。
3. 预览回执 `final_output=0` 时，必须显示明确提示并禁用「确认生成」；后端零输出门槛继续保留，不能放宽。
4. 修复无素材池时「添加素材池」写入 `0` 的行为：无池时应禁用该操作或给出「请先在规则素材池创建素材」的明确引导，而不是插入无效 ID。
5. 对「存在空池」或「池内规则均不被 Clash 支持」的状态，至少要在预览阶段通过最终输出 0 的提示阻止生成；是否在规则步骤提前做目标级可用性判断，属于后续设计问题。

该口径是后续修复的产品决策，**不是当前代码已经实现的行为**；本轮没有写入 Design/Build 文档，也没有修改代码。

#### 8. 候选修复方向（待实施，未编写代码）

以下方向与已确认口径一致，仅作为后续实施参考，不代表已确认方案。

| 方向 | 思路 | 主要风险／待验证点 |
|------|------|------------------|
| A. 规则步骤阻断 | 在 `AssemblyView` 增加类似「是否有规则源」的计算：`form.pools.length > 0 || form.custom_rules.length > 0`；Clash 规则步骤为假时禁用「下一步」并显示行内提示；手动规则行仍可正常添加 | 需要决定提示位置与是否允许先进入预览查看空产物；不能误伤无池 + 手动规则路径 |
| B. 无池时禁止添加池 | `addPool()` 在 `context.pools` 为空时直接 `Notify.warning` 并返回，或禁用「添加素材池」按钮并附「前往规则素材池」入口 | 与 R35-03 的跨页刷新问题相关；跳转返回后的上下文刷新需另行验证 |
| C. 预览阶段生成门槛 | `canGenerate` 增加对当前目标（Clash/SR-conf）`previewReceipt?.final_output > 0` 的判断；回执为 0 时显示醒目警告 | 需兼容旧响应缺少 receipt 的情况；不能把 sr-subs／generic-subs 误纳入；`final_output` 应只针对当前预览指纹有效 |
| D. 目标级素材可用性 | 由后端提供「某目标可输出的规则条数」或在前端依据能力元数据过滤条目；预检/规则步骤可据此提示空池或仅 SR 规则池 | 当前 `context` 只有 `entry_count`，没有目标级可用条数；需要新增契约或预检端点，成本较高，能否覆盖 URL 同步／快照状态待验证 |
| E. 页面级无池阻断 | 直接把 `context.pools.length === 0` 加入 `buildPreflightMissing` | **与已确认口径不符**，会隐藏装配器并切断手动规则路径；仅作为被否决方向记录 |

**后续验证建议：**

- 为无池、空池、仅 SR 规则池、可用池、无池 + 手动规则分别补前端组件测试和浏览器回归；断言预检文案、规则步骤能否进入、`确认生成` 是否禁用及后端请求状态。
- 对 `previewReceipt.final_output=0` 增加断言，确保只在当前预览指纹有效时禁用生成；预览过期或切换目标后应恢复「请先刷新预览」语义。
- 修复后重跑 `frontend` 测试与构建、`backend` 的 `go test ./...`、`go build ./...`、`go vet ./...`，并保留空规则负例与有效规则正例。
- 若采用方向 D，需要单独验证 URL 同步、active/pending 快照和空池切换；本轮未覆盖这些状态。

#### 9. 支持证据摘要

| 场景 | 实际请求／响应或页面观察 |
|------|--------------------------|
| 仅缺素材池 | `GET /api/admin/assembly/context` 返回 `pools: []`；页面无「构建前缺少以下前置条件」；规则素材显示「尚未添加素材池」 |
| 无池、无规则预览 | `POST /preview` → 200；`warnings` 含空规则文案；`receipt.final_output=0`；正文规则只有 `GEOIP,CN,DIRECT`、`MATCH,🛟无法归属的流量` |
| 无池、无规则生成 | `POST /generate` → 400；`message=当前目标没有可输出的非系统规则` |
| 无池时添加素材池 | 页面池下拉显示 `0`、展开「暂无数据」；`POST /preview` → 400；`message=参数错误: 素材池不存在: 0` |
| 空池选择 | `POST /preview` → 200；`warnings=[]`；`receipt.final_output=0`；`确认生成` 仍 enabled |
| 空池生成 | `POST /generate` → 400；同「当前目标没有可输出的非系统规则」 |
| 仅 `USER-AGENT` 池 | `POST /preview` → 200；`warnings` 含「规则已跳过：USER-AGENT,Telegram（Clash 不支持该规则类型）」；`receipt.final_output=0` |
| 可用池 | `POST /preview` → 200；正文含 `DOMAIN-SUFFIX,example.com`；`receipt.final_output=1`；`POST /generate` → 200，版本 v1 自动激活 |
| 无池 + 手动规则 | `POST /preview` → 200；正文含 `DOMAIN-SUFFIX,example.org`；`POST /generate` → 200，版本 v2 未自动激活 |
| 对照：无可用节点 | 页面出现 1 条「至少一个可用节点」前置条件，装配器隐藏；恢复节点后消失 |

以上浏览器与 API 观察均来自 2026-11-16 的临时隔离实例；请求体中的节点、平台、规则和账号均为本地测试数据。页面截图保存在临时目录，未加入仓库。

#### 10. 未确认内容、未覆盖范围与限制

- **未运行已发布的 `v4.1.3`／`v4.1.4` Docker 镜像**，也未运行 Docker Compose；结论基于当前 HEAD 的本地 Production 构建。发布镜像是否同样只是「预检未阻断」，未实测。
- **未获取用户原始截图的构建版本与完整操作序列。** 当前 HEAD 复现出的 `0` 与截图描述一致，但只能确认「无池时点击添加素材池」会产生该显示，不能证明用户当时一定执行了同一个点击序列。
- **未覆盖 URL 来源同步、active/pending 快照、pending 激活、来源失败等素材池状态。** 本轮只使用手工条目和空池；`entry_count` 与 `loadPoolEntries` 的活跃快照语义来自代码与现有测试，未在浏览器逐一构造。
- **未覆盖 SR 分流规则（sr-conf）及其他装配类型。** `Warnings()` 和零输出门槛同样适用于 sr-conf，但预检与 UI 流程不同；不能把本轮 Clash 结论直接外推到其他目标。
- **未覆盖所有视口、主题和浏览器。** 本轮浏览器固定为 Chrome 157、1440×1000 的 headless 会话；该问题属于功能与流程阻断，不是布局问题，但未做移动端和明暗主题矩阵。
- **未运行全量测试套件。** 已构建前后端并执行实时 API／浏览器验证；未重新执行 `go test ./...`、前端全量 Vitest 或发布门禁。现有 `assembly-view.spec.ts` 没有覆盖「无池预检」「无池 addPool 写入 0」「final_output=0 禁用生成」这些断言，后续修复应补齐。
- **「素材池存在但没有可用素材」存在两种已测解释：** 空池（条目数 0）和池内仅有目标不支持条目；两者都未在预检阻断，但预览表现不同（空池无警告，仅 SR 规则池有跳过警告）。其他「同步后无 active 条目」的状态未实测。
- **未验证修复后的行为。** 本文的方向与产品口径均为后续工作输入，不能视为已实现、已验证的修复。

#### 11. 本轮未做

- 未修改任何项目代码、配置、依赖或锁文件；
- 未实施任何候选修复，也未将问题标记为已修复；
- 未操作生产、预发布、外部服务、真实账号或真实数据；
- 未运行 Docker 镜像、远端 CI 或真实客户端连接；
- 唯一持久化变更为本 `Issue21.md` 文档更新。

### R35-03 创建素材池后返回 Clash 构建页面需要整页刷新

- **现象：** 用户反馈，在 Clash 构建页面切换到「规则素材池」创建素材后，再返回构建页面，无法立即获取新建素材；需要整页刷新后才能获得。
- **复现步骤：** 进入 `/admin/assembly?tab=clash-yaml` 的规则素材步骤；切换到「规则素材池」并创建素材；返回 Clash 构建页面，查看素材池选择项，再整页刷新查看。
- **预期：** 创建素材后返回构建页面，无需整页刷新即可获取新建素材。
- **状态：** 待后续处理；本次仅记录用户反馈，未独立复现、研究原因或修复。

## 二、记录格式

每项记录已观察到的现象、操作步骤和必要的环境信息；根因结论必须附可复现证据和对照实验，无法确认的内容标明待确认。修复方案如未实施，明确标注为候选／待决策，不把候选方案当作已确认方案。

## 三、变更记录

| 日期 | 说明 |
|------|------|
| 2026-11-16 | 创建本轮本地调试问题记录。 |
| 2026-11-16 | 记录首次配置三个阶段的 UI 粘连问题 R35-01。 |
| 2026-11-16 | 根据截图补充确认和完成阶段的粘连位置。 |
| 2026-11-16 | 将节点管理问题并入 R35-01，并补充本地逐页目视检查发现的位置及覆盖边界。 |
| 2026-11-16 | 按授权完成 R35-01 根因研究：确认 Tailwind CSS 4 `@layer utilities` 与 Ant Design Vue 未分层 CSS-in-JS 组件 `margin: 0` 重置之间的级联优先级冲突，并记录 `space-y-*` 零特异性 `:where()` 的叠加因素；补充复现环境、构建哈希、逐项实测、对照实验、影响范围、未覆盖边界与候选修复方向。 |
| 2026-11-16 | 更新文档定位、记录格式与 R35-01 状态：根因已确认，尚未修复；本轮仅测试、研究与文档记录，未修改项目代码、配置、依赖或锁文件。 |
| 2026-11-16 | 记录无素材池时 Clash 构建预检的用户反馈与预期，编号 R35-02。 |
| 2026-11-16 | 按授权完成 R35-02 根因研究：确认页面级预检 `buildPreflightMissing` 只检查节点／平台／订阅，未覆盖素材池或规则源；复现无池时 `addPool` 写入 `pool_id=0`、空池／仅 `USER-AGENT` 池预览最终输出 0、可用池与「无池 + 手动规则」可生成、节点禁用对照可阻断等状态；记录前后端调用链、Design2 与 Design3 的口径冲突、未覆盖边界及候选方向。 |
| 2026-11-16 | 记录用户确认的 R35-02「按有效规则源阻断」口径：保留手动规则，规则步骤阻止无池且无手动规则进入预览，预览 `final_output=0` 禁用生成，并修复无池时添加素材池产生 `0`；该口径尚未实施，本轮仍仅研究、测试与文档记录。 |
| 2026-11-16 | 记录创建素材池后返回 Clash 构建页面需整页刷新的用户反馈，编号 R35-03。 |
