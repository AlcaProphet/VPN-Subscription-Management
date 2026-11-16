# Issue21.md — 本地调试问题记录

> **文档定位：** 记录本地调试中发现的问题及其研究、修复和验证结果。已归档的问题记录见 [docs/reports/Issue/](docs/reports/Issue/)，项目指令见 [AGENTS.md](AGENTS.md)。
>
> **当前状态（2026-11-16）：** R35-01 已实施前端修复并通过本地复测，见本项第 11 节。R35-02 的阻断口径已由用户确认，尚未修复。R35-03 已完成分路径研究，`pool_id=0` 前置状态和旧内嵌构建可复现，用户原始环境待确认，尚未修复。R35-04 已完成从节点新建／编辑到 Clash/Mihomo 版本生成的全链路复现，尚未修复。R35-05 待处理。

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
- **当前状态（2026-11-16）：** 已修复已确认的同源间距回归。用户确认的新建平台和创建规则两处「最后一行 scheme → 添加按钮」均采用 12px。第 1～10 节保留修复前的研究和候选方案，第 11 节记录实际修复与复测。

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
- 即使把工具类移出 `@layer`，只要仍保留 `:where()`，零特异性仍会输给 `.ant-card` 的 0,1,0 特异性。父容器的 `gap` 直接定义子项间隔，绕过这两层外边距竞争。

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

#### 9. 修复前比较的候选方向

以下保留根因研究阶段的候选方向和当时的风险判断。实际实施方案见第 11 节。

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

#### 10. 根因研究阶段未做

- 未修改任何项目代码、配置、依赖或锁文件；
- 未实施任何候选修复，也未把问题标记为已修复；
- 未操作生产、预发布、外部服务、真实账号或真实数据；
- 未运行 Docker 镜像、远端 CI 或真实客户端连接；
- 唯一持久化变更为本 `Issue21.md` 文档更新。

#### 11. 实施与本地复测（2026-11-16）

**修复前扩查。** 从 `beta` 的 `0e74c41` 出发，解析前端 Vue 模板，检查 `space-y-*` 容器的直接子项（含条件模板）、组件根上的间距类，以及 Ant Design Vue 4.2.6 对应组件的重置样式。用隔离 Production 构建在 Chrome 157 中扫描 38 个路由或交互状态，记录 50 次组件根间距不符和 26 次相邻元素间距缩小。不同状态可能重复命中同一位置。

| 扩查范围 | 修复前发现 |
|---|---|
| 登录与用户侧规则 | 首次管理员提示、错误提示和公开下载警告的外边距为 0。 |
| 订阅装配与创建分享 | 构建前置条件提示、SR 分步步骤条、目标选择及分享名称输入框的间距被覆盖。 |
| 设置与日志 | 验证码提示、运行安全提示、SMTP 字段说明、邮件模板正文变量区和邮件历史提示受到影响。速率限制的数字输入还被组件默认宽度覆盖。 |
| Xray、应急及条件分支 | 复现 Xray 空态提示、损坏库应急页提示的间距丢失，并按相同重置规则核对高级表单、素材池、节点检查、OpenVPN 导入和共享确认弹窗的条件分支。 |

**实施。** 由纵向容器使用 `gap` 承担卡片、字段和提示之间的间隔。单点间距由包装层承载，或在明确被覆盖的组件根上用 Tailwind 4 的局部重要修饰符恢复原声明。保留按钮按内容定宽、组件内部样式、拖拽与表单交互。节点表单的警告到「基本信息」恢复到 20px，与修复前在同一 DOM 上模拟旧选择器测得的间距一致。速率限制的标签和输入框分行，标签下留 4px，输入框恢复占满所在列。

| 用户确认的额外调整 | 修复前 | 修复后目标 | 多条输入行之间 |
|---|---:|---:|---:|
| 新建／编辑平台，最后一行 scheme → 添加按钮 | 8px | 12px | 8px |
| 创建规则，最后一行 scheme → 添加按钮 | 4px | 12px | 4px |

**验证。** 同一组 38 个路由和状态在重建后的 Production 候选上复跑，未再检出这两类间距冲突。前端 `npm run build` 与 `npm run test` 通过（51 个文件、460 项）；后端 `go build ./...`、`go vet ./...`、`go test ./...` 通过。隔离副本首次运行 Go 测试时遗漏测试使用的两个 `docs/DocTemplates` 文件，补齐后全量通过。浏览器服务使用临时回环实例和合成数据。

- 1280px、920px、375px 分别覆盖浅色和深色，共通过 246 项几何或计算样式断言。实际点击装配前置条件的「前往节点管理」，横幅到标题为 16px。新建节点警告下方为 20px，创建规则在桌面弹窗和手机抽屉中都恢复为 12px 字段间距。订阅装配分步步骤条与下一段、单页相邻卡片均为 16px。
- 新建平台和创建规则在单行、两行时均实测到添加按钮前 12px。新增和删除交互通过，桌面新建平台的拖拽排序通过；另用已保存的临时平台验证编辑入口为 12px。
- 在真实临时损坏库中，应急原因到操作码区域从 0px 恢复为 16px，验码后两级危险提示从 0px 恢复为 8px。补测共用确认词输入为 12px，邮件模板加载错误到重试按钮为 8px，并验证重试恢复模板编辑。

**覆盖边界。** 本地结果以候选构建为准。已发布镜像、远端 CI、跨浏览器和外部 Xray/OIDC/SMTP 连接不在本轮验证范围。未进入的业务状态只按模板和组件样式核对。

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
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本次仅记录用户反馈，未独立复现、研究原因或修复。
- **当前状态（2026-11-16 研究后）：** 已完成分路径复现与根因研究，尚未修复。结论不是单一代码根因，需按环境与操作前提区分：
  - 当前 HEAD 的隔离构建产物在干净流程下**未复现**“必须整页刷新”：从池页签返回构建页时会触发 `refreshPools()`，新素材池已进入候选；
  - 若先按 R35-02 的方式在无池时点击「添加素材池」，形成 `pool_id=0` 行，再创建素材池，返回后候选虽然已更新，但选择值仍显示 `0`；整页刷新重置表单后才显示新池。该路径与 R35-02 的 `addPool()` 回退值直接相关；
  - 工作区 `backend/web/dist`（2026-08-25，8080 当前内嵌）早于 R24-21 修复，缺少 `pools-changed`／`refreshPools`，可精确复现“候选列表不更新、整页刷新后才出现”；用户实际测试实例未确认；
  - 另发现创建无 URL 的素材池时 `urls:null` 导致池列表渲染空白，与本问题的刷新根因不同，详见第 4 节 H 与第 5 节。
  - 本轮只做测试、研究、文档记录，**未修改任何项目代码、配置、依赖或锁文件，未实施修复，也未将问题标记为已修复**。

#### 1. 结论摘要

- **当前 HEAD 的干净流程不复现原始描述。** 在页面加载后先切换到「规则素材池」创建带 URL 来源的素材池，再切回「构建订阅/规则」，`watch(mainTab)` 会调用 `refreshPools()`；`GET /api/admin/pools` 返回新池，`onPoolsChanged()` 替换 `context.pools`，规则步骤的池下拉立即出现新池并可正常选中。该结论来自当前 HEAD 的隔离 Production 构建（路径 A）。
- **当前 HEAD 存在一条“看起来仍需整页刷新”的残留路径。** 若在无素材池时先点击「添加素材池」，`addPool()` 会把 `pool_id` 写成 `0`；之后创建素材池并返回，候选列表其实已经刷新，但已存在的 `0` 行不会被重新绑定。页面表现为下拉仍显示 `0`，整页刷新清空表单后再添加才会显示新池（路径 B）。这与 R35-02 的无效 `0` 缺陷共用状态，但不是候选列表没有刷新。
- **工作区的旧内嵌构建可精确复现原始描述。** `backend/web/dist` 的时间戳为 2026-08-25，早于 R24-21 修复提交 `097cb58`（2026-08-30）；该构建没有 `pools-changed`／`pool-content-changed` 事件，也没有返回构建页时的 `refreshPools()`。从池页签返回后，池下拉仍为 `0` 且展开为「暂无数据」；整页刷新重新请求 `assembly/context` 后才出现新池（路径 Old）。本地 8080 当前服务的正是这一旧构建。
- **应用内切换页面再返回可以刷新。** 离开 `/admin/assembly` 到 `/admin/nodes` 再返回会重新挂载 `AssemblyView` 并重新请求 `assembly/context`，外部创建的素材池会出现（路径 G）。这说明刷新机制依赖组件挂载或页签切换，而不是全局缓存失效。
- **已有素材池中新增素材与“新建素材池”表现不同。** 在已选素材池中新增手动条目后返回，预览会按既有 R24-22 逻辑转为过期并提示“配置已变化，请重新预览”；点击「刷新预览」后后端实时读取到新条目，不需要整页刷新（路径 F）。这不是 R35-03 的候选列表问题。
- **附带发现一个独立的空池渲染缺陷。** 创建不带任何 URL 来源的素材池时，列表 API 返回 `urls:null`；`PoolTab.vue:241` 直接读取 `record.urls.length` 会抛 `TypeError`，池列表区域渲染为空白。整页刷新仍空白，但构建页的池下拉仍能拿到该池（路径 H）。它与“返回构建页需要刷新”不是同一根因，建议另立问题跟踪。
- **用户原始环境未确认。** 原始反馈没有记录访问的是 8080 旧构建、Vite 开发服务器、工作区 `frontend/dist` 还是其他 Production 构建，也没有说明创建前是否已有 `pool_id=0` 行。因此不能把上述某一条路径直接认定为用户遇到的唯一根因；本轮分别记录并保留三条已验证路径。
- 本轮**未修改任何项目代码、配置、依赖或锁文件**，未实施修复。

#### 2. 代码版本、运行环境与测试数据前提

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `1239e3c`（`git describe` 为 `v4.1.4-3-g1239e3c`）；最后一个代码提交为 `1ceea52`；与 R35-02 构建使用的 `7660292` 相比，`frontend/`、`backend/` 的 diff 为空 |
| 工作区状态 | 研究开始时工作区干净；本轮结束时仅 `Issue21.md` 有持久化修改 |
| 前端依赖 | Vue 3.5.44、Vite 7.3.7、Tailwind CSS 4.3.3、`@tailwindcss/vite` 4.3.3、`ant-design-vue` 4.2.6 |
| 后端/运行时 | Go 1.26.4（darwin/arm64）；Node.js 26.7.0；macOS 27.2 |
| 浏览器/驱动 | Google Chrome 157.0.8103.52；Playwright-core 1.57.0（安装在临时目录，未加入项目依赖） |
| 当前 HEAD 隔离实例 | `/tmp/r35-03-pool-refresh`；`APP_MODE=prod`、临时 `DATA_DIR`、`PORT=18110`，仅监听 `127.0.0.1` |
| 旧构建隔离实例 | `/tmp/r35-03-olddist`；当前后端源码 + 工作区 `backend/web/dist`（2026-08-25）重新嵌入构建，`PORT=18111`，仅监听 `127.0.0.1` |

**构建产物。** 本轮未重新构建前端，沿用 R35-02 研究时从同一代码基线构建的隔离产物；`7660292..HEAD` 的前后端 diff 为空，因此代码一致。为避免改动工作区，旧构建实例是在 `/tmp` 复制当前后端源码、替换为工作区旧 `backend/web/dist` 后重新执行 `CGO_ENABLED=0 go build` 得到。

| 产物 | 说明 | 关键哈希 |
|------|------|----------|
| 当前 HEAD 服务 `/tmp/r35-03-pool-refresh/server` | 当前 HEAD 后端 + R35-02 同源前端构建 | `06163d61f040b8316eefffe5ea0688d1a3617b076d5fb385cd959f177c38f0c4` |
| 当前 HEAD 前端 `index.html` | R35-02 临时构建 | `330ac0fb45326003e263c6397faebd1e96f012f8a572b90c3f1a6272eba85edd` |
| 当前 HEAD 主 CSS `index-CFZtmKWH.css` | 同一构建 | `3b4626a471f35d7de8e6764bb1566c3ab34697a0ac8c60ea2e7a9c83213c516b` |
| 当前 HEAD 装配页分块 `AssemblyView-kkF8QGCH.js` | 同一构建；包含 `pools-changed`／`pool-content-changed` | `4b63a7fd37b68604d7adb62ac25191c506e8dfe0b31a062894e3281b43a47bac` |
| 旧构建 `index.html` | 工作区 `backend/web/dist` 原样嵌入 | `d3612456c680a5dba5b4e9e854fca033cec01fac7425053ae91b1b79db146aa3` |
| 旧构建 `AssemblyView-9uejEL3P.js` | 2026-08-25 产物；不含池刷新事件 | `febd1518f4f5f37f884e0589e03d4e1665d8cab969e7b2fb2a72fbbcc214ec51` |
| 旧构建服务 `/tmp/r35-03-olddist/server` | 当前后端 + 旧前端产物 | 54 MB；`CGO_ENABLED=0 go build` |

**本地 8080 的证据。** `GET http://127.0.0.1:8080/` 与 `/admin/assembly` 均引用 `assets/index-48_-me5R.js`，且 `assets/AssemblyView-9uejEL3P.js` 返回 200；这与 `backend/web/dist/index.html` 的旧构建一致。8080 的数据未被本轮操作，旧构建复现使用独立端口与临时数据库。

**测试数据前提。** 测试库由仓库内本地测试库 `backend/data/app-prod.db` 通过 `sqlite3 -readonly ... ".backup"` 复制到临时目录；源库未写入或清空。复制时库内已有：

- 合成管理员 `admin@test.com`，密码为 [TestPasswordList.md](docs/Reference/TestPasswordList.md) 规定的统一测试密码；
- 3 个平台、1 个 Clash 订阅（平台 1，订阅 id 1）、1 个手动 VLESS 节点 `HK`、9 个启用预设代理组；
- **0 个规则素材池**，用于观察“创建后返回”和“首次进入前”两种前提。

每条路径开始前都从同一基线备份恢复到临时库；所有素材池、URL 来源和条目只写入 `/tmp` 临时库。代表性测试数据包括：`R35-03 D 池`（URL 来源 + 手动条目 `r35-03-e2.example`／`r35-03-f3.example`）、`R35-03 F 池`（URL 来源）、`R35-03 G 池（外部创建）`、`R35-03 H 池（其他标签页）`、`R35-03 H 空池`（无 URL 来源）和 `R35-03 OldDist 池`（旧构建，URL 来源）。没有操作生产、预发布、外部服务、真实账号或真实数据。

#### 3. 复现步骤

1. 将当前 HEAD 的隔离服务启动在 `127.0.0.1:18110`；登录 `admin@test.com`。
2. 打开 `/admin/assembly?tab=clash-yaml`，在第 1 步保持默认头部，点击「下一步」。
3. 在「节点与代理组」勾选唯一手动节点 `HK`（界面显示 `HKvless`），打开「🌎国外流量」的「选择与排序」，勾选 `HK` 并保存；点击「下一步」进入「规则素材」。
4. **当前 HEAD 干净流程（A）**：在「规则素材」不点击「添加素材池」，直接切换到「规则素材池」，点击「新建素材池」，填写名称 `R35-03 D 池` 和 URL `https://example.com/rules.txt`，保存；切回「构建订阅/规则」，点击「添加素材池」，观察选择值与下拉选项。
5. **当前 HEAD 的 `0` 前置路径（B）**：在步骤 3 的规则步骤先点击「添加素材池」（此时无池，行显示 `0`），再按步骤 4 创建 `R35-03 F 池`（池名，不代表路径编号）；返回构建页后观察该行，再整页刷新并重走步骤 2～3，重新添加素材池对照。
6. **旧内嵌构建（Old）**：在 `127.0.0.1:18111` 打开同样的 Clash 构建流程；旧构建第 1 步需先选择平台，节点步骤的国外流量成员直接以复选框列出 `HK`。在规则步骤切换到「规则素材池」，创建 `R35-03 OldDist 池`（URL `https://example.com/rules.txt`），返回后点击「添加素材池」，展开下拉并观察；随后整页刷新重走流程对照。
7. **已有池新增素材（F）**：先选择 `R35-03 D 池` 并进入预览，再切到「规则素材池」打开该池详情，新增条目 `r35-03-f3.example`，返回构建页观察预览是否过期；点击「刷新预览」后检查预览正文是否包含新条目。
8. **应用内页面切换（D）**：在构建页加载后，通过已登录会话的 API 创建 `R35-03 G 池（外部创建）`；先确认当前页候选未更新，再导航到 `/admin/nodes` 后返回 `/admin/assembly?tab=clash-yaml`，检查候选。
9. **其他浏览器标签页（E）**：在第二个浏览器页签创建 `R35-03 H 池（其他标签页）`，回到第一个页签检查候选；随后整页刷新对照。
10. **无 URL 空池（H）**：创建一个只填名称、不填 URL 的 `R35-03 H 空池`，观察池列表、浏览器 console、整页刷新后的列表，以及构建页下拉是否仍可获得该池。
11. 用 Playwright 记录页面文本、截图和 `/api/admin/assembly/context`、`/api/admin/pools`、`/api/admin/assembly/preview` 的请求／响应；截图仅保存在 `/tmp/r35-03-pool-refresh/evidence/`，未加入仓库。

#### 4. 不同创建、切换和返回路径的实际结果

| # | 路径 | 环境与前置 | 返回构建页后的候选 | 返回后当前选择值 | 整页刷新后 | 复现原始描述？ |
|---|------|------------|--------------------|------------------|------------|----------------|
| A | 干净流程：创建带 URL 的池后切回构建页 | 当前 HEAD；页面加载时 0 池；未预点「添加素材池」 | 下拉含新池 | 新池（`addPool` 默认选中） | 与新池一致 | 否 |
| B | 先点「添加素材池」再创建池 | 当前 HEAD；无池时已写入 `pool_id=0` | 下拉含新池 | 仍为 `0` | 新池；表单已清空 | **部分**（可见值仍为 `0`；候选实际已刷新） |
| C | 首次进入构建页之前创建池 | 当前 HEAD；先停在 `?tab=pool` 创建，再进入构建页 | 下拉含该池 | 该池 | 一致 | 否 |
| D | 应用内离开再返回 | 当前 HEAD；构建页加载后外部创建 G 池，导航到 `/admin/nodes` 再返回 | 下拉含 G 池 | G 池 | 一致 | 否（组件重挂载，重新请求 context） |
| E | 另一个浏览器标签页创建 | 当前 HEAD；第一个页签保持构建页不重挂载 | 不含 H 池 | 仍为旧池 G | 含 G、H 池 | 仅在该边界下需要刷新；不属于应用内页签返回 |
| F | 已有池新增手动条目 | 当前 HEAD；已选 D 池且已有预览 | 候选池不变；预览转为过期 | 已选 D 池不变 | 未测试整页刷新；点击「刷新预览」即可获得新条目 | 否；按 R24-22 设计需要刷新预览，不需整页刷新 |
| G | 旧内嵌构建创建池后返回 | 2026-08-25 `backend/web/dist`；8080 同源旧构建 | 下拉仍为「暂无数据」 | `0` | 出现 OldDist 池并被默认选中 | **是**（精确复现候选列表未刷新） |
| H | 创建无 URL 的空池 | 当前 HEAD；只填名称 | 构建页下拉含该池 | 该池 | 仍含该池 | 否（池列表空白是另一缺陷，刷新不修复） |

**A. 当前 HEAD 干净流程。** 创建 `R35-03 D 池` 后返回，`GET /api/admin/pools` 返回 `list:[{id:2,name:"R35-03 D 池",urls:["https://example.com/rules.txt"]}]`；`context.pools` 被替换，规则步骤的池下拉显示该池，展开选项也为该池。另一次从规则步骤直接切换池页签、未预点「添加素材池」的干净流程（A2）结果相同：添加素材池后当前值为新池，选项也只有新池。没有出现“需要整页刷新”的现象。

**B. 当前 HEAD 的 `0` 前置路径。** 在无池时点击「添加素材池」，行内池选择显示 `0`；随后创建 `R35-03 F 池`，返回构建页后下拉展开已有 `R35-03 F 池`，但当前输入框仍显示 `0`（截图 `F3-return-zero.png`）。整页刷新后重走步骤并重新添加素材池，输入框显示 `R35-03 F 池`（截图 `F4-after-reload.png`）。这条路径可以解释用户看到的“必须刷新才能拿到新素材”，但根因是**旧的无效选择值没有被清掉／替换**，不是新池没有进入候选。

**C. 首次进入构建页之前创建。** 先停在 `?tab=pool` 创建空池 `R35-03 H 空池`，再进入构建页并走到规则步骤；池下拉包含该池并可选中。说明 mount 时的 `assembly/context` 能带回已存在的池。

**D. 应用内页面切换。** 构建页保持打开时通过 API 创建 `R35-03 G 池（外部创建）`；当前页候选没有立即变化（符合无轮询设计）。导航到 `/admin/nodes` 再返回装配页后，`AssemblyView` 重新挂载并重新请求 `assembly/context`，下拉出现 G 池；这是“应用内切页返回”路径，不需要整页刷新。

**E. 另一个浏览器标签页。** 在第二个页签创建 `R35-03 H 池（其他标签页）` 后回到第一个页签，第一个页签的候选仍只有 G 池；整页刷新后才出现 G、H 池。该行为说明当前没有跨标签页通知或窗口聚焦刷新；它不属于“应用内规则素材池页签 → 构建页签”路径，列为边界。

**F. 已有池新增素材。** 已选 D 池并生成预览后，在池详情新增 `r35-03-f3.example` 并返回，页面显示“配置已变化，请重新预览”，预览正文仍是旧内容；点击「刷新预览」后正文出现新条目，未整页刷新。候选池列表不需要变化。

**G. 旧内嵌构建。** 创建 `R35-03 OldDist 池` 后返回构建页，池行仍显示 `0`，展开下拉为「暂无数据」（截图 `OLD4-dropdown-before-reload.png`）；整页刷新并重走流程后，池行显示 `R35-03 OldDist 池` 且选项包含该池（截图 `OLD3-after-reload.png`）。这条路径与原始反馈的现象完全一致。

**H. 无 URL 空池。** 创建 `R35-03 H 空池` 后，池页签列表空白，console 报 `TypeError: Cannot read properties of null (reading 'length')`；整页刷新后仍空白；但构建页的池下拉可以选择该池。该缺陷与刷新时机无关。

#### 5. 请求、响应与前端生命周期证据

**当前 HEAD（干净流程 A／`0` 残留路径 B）的实际请求顺序：**

```text
GET  /api/admin/assembly/context   → pools: []
GET  /api/admin/pools             → {list: [], total: 0}        （PoolTab 加载）
POST /api/admin/pools             → 200, id=2, urls=[...]      （新建素材池）
GET  /api/admin/pools             → {list: [{id:2,...}]}       （PoolTab 保存后 load）
GET  /api/admin/pools             → {list: [{id:2,...}]}       （切回构建页触发 refreshPools）
```

- 切回构建页时没有重新请求 `assembly/context`；只请求了 `/api/admin/pools`，由 `onPoolsChanged()` 定向替换 `context.pools`。
- 在 `0` 前置路径中，第二次 `GET /api/admin/pools` 同样返回新池，但 `form.pools` 中已存在的 `{pool_id:0}` 没有被改动；因此“候选已更新”和“当前选择仍为 0”同时成立。
- 整页刷新后重新请求 `assembly/context`，`context.pools` 直接包含新池；`form` 重新初始化为空，再点击「添加素材池」时 `addPool()` 取到新池的 id。

**旧内嵌构建（Old）的实际请求顺序：**

```text
GET  /api/admin/assembly/context   → pools: []                  （页面加载）
GET  /api/admin/pools             → {list: [], total: 0}        （PoolTab 加载）
POST /api/admin/pools             → 200, id=2, urls=[...]      （新建素材池）
GET  /api/admin/pools             → {list: [{id:2,...}]}       （PoolTab 保存后本地 load）
切回构建页：无 /api/admin/pools、无 /api/admin/assembly/context 请求
整页刷新：GET /api/admin/assembly/context → pools: [{id:2,...}]
```

旧构建的 `AssemblyView-9uejEL3P.js` 中检索不到 `pools-changed` 或 `pool-content-changed` 字符串；当前构建分块中分别有 2 个 `pools-changed` 和 3 个 `pool-content-changed`。旧构建没有返回构建页时的 `refreshPools()`，也没有父页面回写事件，因此 `context.pools` 一直是加载时的空数组。

**关键文件与源码位置：**

| 文件位置 | 作用与已验证事实 |
|----------|------------------|
| `frontend/src/views/admin/AssemblyView.vue:71-73` | `watch(mainTab)`：从其他一级页签进入 `build` 时调用 `refreshPools()` |
| `frontend/src/views/admin/AssemblyView.vue:336-347` | `onPoolsChanged()` 替换 `context.pools`；仅当旧上下文中曾存在、且新列表已不存在的已选池才提示；`pool_id=0` 不在旧 ID 集合中，不会触发修复 |
| `frontend/src/views/admin/AssemblyView.vue:356-362` | `refreshPools()` 只请求 `listPools()`，失败仅提示，不回退到重新加载整个 `context` |
| `frontend/src/views/admin/AssemblyView.vue:364-368` | `onMounted` 才调用 `loadContext()`；页签切换不重挂载，因此不会重新请求 `assembly/context` |
| `frontend/src/views/admin/AssemblyView.vue:584-586` | `addPool()` 使用 `context.pools[0]?.id ?? 0`；无池时写入 `pool_id=0` |
| `frontend/src/views/admin/AssemblyView.vue:349-354` | 已选池内容变化时递增 `selectedPoolContentRevision`，使旧预览过期；不负责修复池候选或选择值 |
| `frontend/src/views/admin/AssemblyView.vue:403-428` | `restoreAssemblyDraft()` 只恢复由预检跳转保存的草稿；普通整页刷新不会恢复当前步骤与表单 |
| `frontend/src/views/admin/assembly/RulesStep.vue:42-60` | 池下拉选项来自 `context.pools`；「添加素材池」不检查候选是否为空 |
| `frontend/src/views/admin/assembly/PoolTab.vue:25-40` | `load()` 请求池列表、更新局部 `pools`，并向父组件发出 `pools-changed` |
| `frontend/src/views/admin/assembly/PoolTab.vue:72-98` | 新建／编辑成功后关闭弹窗并 `await load()`，因此当前 HEAD 会立即回写父上下文 |
| `frontend/src/views/admin/assembly/PoolTab.vue:183-186` | 池详情变更后发出 `pool-content-changed`，并刷新池摘要 |
| `frontend/src/views/admin/assembly/PoolTab.vue:241` | 表格读取 `record.urls.length`；API 返回 `urls:null` 时抛错，导致空池列表空白 |
| `frontend/src/views/admin/assembly/PoolDetail.vue:133-156` | 手动条目新增成功后 `loadManualEntries()` 并发出 `changed`（父层再发 `pool-content-changed`） |
| `frontend/src/api/pool.ts:157-158` | `listPools()` 解包 `/api/admin/pools` 的 `data.list` |
| `backend/internal/server/assembly.go:64-88` | `GET /api/admin/assembly/context` 通过 `poolSvc.List` 返回池摘要；无缓存 |
| `backend/internal/server/pool.go:21-30` | `/api/admin/pools` 列表与 CRUD 路由；列表每次查库 |
| `backend/internal/pool/pool.go:283-320` | `List()` 按 `rule_pools` 查询并加载来源；没有内存缓存 |

**测试执行。** 本轮运行了 `assembly-view.spec.ts`、`pool-tab.spec.ts`、`pool-detail.spec.ts`、`rules-step.spec.ts` 共 4 个测试文件、50 项测试，全部通过。现有 `assembly-view.spec.ts` 覆盖了 `PoolTab` 发出 `pools-changed` 后候选更新，以及 `pool-content-changed` 后预览过期；未覆盖“从池页签返回时 `mainTab` 触发 `refreshPools`”和“`pool_id=0` 行在候选更新后仍保留”的断言。

#### 6. 根因分析

1. **当前 HEAD 干净流程的预期行为由 R24-21 修复提供。** `PoolTab` 在每次成功取得列表后发出 `pools-changed`，`AssemblyView` 定向替换 `context.pools`；从池页签返回构建页时，`watch(mainTab)` 再调用一次 `refreshPools()` 兜底。因此新建素材池会进入候选，不需要重挂载整个页面或重新请求 `assembly/context`。
2. **当前 HEAD 的残留路径是“无效选择值未修复”，不是“候选未刷新”。** `addPool()` 在无池时写入 `0`；候选更新后 `onPoolsChanged()` 只做“旧上下文中存在、且新列表已不存在”的失效判断，`0` 从未存在于旧池 ID 集合中，所以既不会告警也不会替换。页面保留 `{pool_id:0}`，视觉上继续显示 `0`；只有整页刷新清空表单后，重新添加才会选中新池。
3. **旧内嵌构建的精确复现是构建版本落后。** `backend/web/dist` 早于 R24-21 修复提交 `097cb58`，既没有 `pools-changed`／`pool-content-changed` 事件，也没有 `refreshPools()`。`AssemblyView` 只在 `onMounted` 请求一次 `assembly/context`，`PoolTab` 的局部 `load()` 不会回写父页面，所以创建池后返回构建页仍使用旧候选；整页刷新重新挂载并请求 context 后才出现。
4. **空池列表空白是独立的空值渲染缺陷。** 后端无 URL 来源的池返回 `urls:null`，`PoolTab.vue:241` 对 `null` 取 `.length` 抛错，Vue 将该列表子树渲染为空。整页刷新仍会拿到 `urls:null`，所以刷新不能修复；构建页下拉直接使用池列表，不受该表格渲染错误影响。该缺陷不应与 R35-03 的刷新路径合并。
5. **不是以下原因：** 不是后端没有返回新池（`GET /api/admin/pools` 与 `assembly/context` 均实测返回新池）；不是数据库写入失败（POST 200，列表可查）；不是权限或会话问题；不是 HTTP 缓存导致（返回后请求确实发生，旧构建则没有请求）；不是 `PoolTab` 没有保存（新建池在池页签列表中可见）。
6. **证据边界。** 当前 HEAD 的可复现路径依赖 R35-02 的 `0` 前置状态；旧构建路径能独立复现，但用户实际访问的是哪个构建未确认。原始反馈没有提供网络请求或截图中的下拉展开状态，因此不能排除用户只观察到 `0` 而未展开下拉的情况。

#### 7. 与 R35-02 的关系及证据边界

- **存在直接状态关联。** R35-02 已确认：无素材池时点击「添加素材池」，`addPool()` 写入 `pool_id=0`，下拉显示 `0`，预览返回 400「素材池不存在: 0」。R35-03 的当前 HEAD 路径 B 复用了同一个 `0` 行：创建池后候选已刷新，但该行仍是 `0`，整页刷新清空表单后才恢复。该关联有实际页面、请求和状态证据。
- **修复 R35-02 的 `0` 回退会移除路径 B 的前置条件。** 如果后续按 R35-02 已确认口径修复“无池时禁止添加或给出引导”，当前 HEAD 的路径 B 就不会形成；但这是对候选修复方向的推断，本轮未实施、未验证。R35-02 的其他口径（规则步骤阻断、`final_output=0` 禁用生成）不直接解决 R35-03 的候选刷新或旧构建问题。
- **不能把两个问题整体认定为同一根因。** 旧构建路径在没有 `0` 前置的情况下也能复现，原因是缺少 R24-21 的刷新逻辑；当前 HEAD 干净流程则不复现。本轮围绕 R35-03 的复现至少涉及“旧构建未刷新”“当前 HEAD 无效 0 残留”和“空池渲染错误”三个不同层面的问题，不能用一个根因覆盖。
- **同源修复历史。** R24-21（Issue9，2026-08-30，提交 `097cb58`）已经修过“新建并同步规则素材池后构建页候选必须刷新页面才更新”；旧构建时间戳 2026-08-25 早于该修复。因此旧构建现象属于该修复未部署到 8080 的环境问题，而不是当前源码中再次出现的回归。

#### 8. 对现有草稿、步骤状态和已选素材的影响

- **应用内页签返回：** `refreshPools()` 只替换 `context.pools`，不会重置 `currentStep`、`layoutMode`、`form.pools`、手动规则、节点选择或头部内容；这是当前 HEAD 的预期行为。已选且仍存在的池会保留，池内容变化由 `pool-content-changed` 标记预览过期。
- **整页刷新：** `AssemblyView` 重新挂载，`currentStep` 回到第 1 步，`form` 回到默认值；普通刷新不会恢复未保存的节点、代理组、已选池或手动规则。只有通过预检跳转保存到 localStorage 的 `assembly_ctx_v1` 草稿会在返回装配页时恢复（`restoreAssemblyDraft()`），而“规则素材池”页签本身不会保存该草稿。路径 B 的整页刷新对照中，确实需要重新勾选节点并重新添加素材池。
- **已选素材池：** 候选刷新时，如果已选池仍在列表中，选择值和目标组保持不变；如果已选池被删除，`onPoolsChanged()` 会使预览过期并提示“已选素材池已不存在，请移除后重新预览”。`pool_id=0` 不满足“旧上下文中存在”的条件，因此不会被这条保护逻辑修复。
- **预览：** 仅切换页签或刷新池候选不会自动重新预览；新增／同步已选池内容后，预览保持旧正文并显示过期提示，需要点击「刷新预览」。后端预览和生成会实时读取池条目，不使用进入页面时的快照；这与 R24-22 的服务端摘要校验一致。
- **影响范围：** `refreshPools()` 只刷新池，不刷新节点、代理组、平台或订阅；如果用户在其他页面修改了这些数据，返回装配页时应依赖重新挂载或各自页面的刷新机制，本轮未验证这些跨页面组合。

#### 9. 尚未确认的内容、未覆盖范围与限制

- **用户原始环境未确认。** 无法确定用户访问的是 8080 的 2026-08-25 旧构建、Vite 5173、工作区 `frontend/dist` 还是其他 Production 构建；也无法确定创建前是否已有 `pool_id=0` 行。两条主要路径都能产生“显示 0、刷新后出现新池”的可见结果，需要用户补充环境或操作序列才能唯一归因。
- **工作区 `frontend/dist` 未实际加载。** 该 2026-11-11 产物的 `AssemblyView-DBETbe5u.js` 静态检索包含 `pools-changed`／`pool-content-changed`；若用户使用该产物，候选列表整体不刷新的旧构建解释不成立，更可能是 `pool_id=0` 残留。但本轮没有用该目录启动服务，不能据此排除其他差异。
- **未运行已发布的 `v4.1.3`／`v4.1.4` Docker 镜像**，也未运行 Docker Compose；结论基于当前 HEAD 的本地隔离 Production 构建和旧 `backend/web/dist` 构建。
- **未覆盖 URL 来源实际同步。** 本轮只配置 URL、未触发同步；手动条目新增已覆盖。URL 同步后的 `entry_count`、pending/active 快照和同步回执未在浏览器逐一验证。
- **未覆盖其他装配目标。** 只验证了 Clash YAML；SR 订阅、通用订阅和 SR 分流规则的预检、池候选和预览流程不同，不能直接外推。
- **未覆盖所有视口、主题和浏览器。** 本轮使用 Chrome 157、1440×1000 的 headless 会话；该问题是功能与状态问题，未做移动端和明暗主题矩阵。本轮使用默认“分步”布局，未单独测试“单页”布局下切换池页签后的候选刷新；两者共用 `form` 与 `mainTab`，但未在单页模式实测。
- **未做并发/延迟注入。** 没有模拟 `PoolTab` 初始 `load()` 与新池 `load()` 响应乱序；理论上同一页面存在旧响应覆盖新列表的窗口，但本轮没有证据，不能列为已确认根因。
- **未运行全量测试套件。** 已运行 4 个前端定向测试文件、50 项测试并全部通过；未运行前端全量 Vitest、后端 `go test ./...` 或发布门禁。现有测试没有覆盖 `mainTab` 返回刷新和 `0` 残留选择。
- **空池 `urls:null` 缺陷未单独编号。** 本轮只在本节记录，未创建新问题、未修改后端返回契约或前端渲染；是否另立问题由后续决策。
- **未验证修复后的行为。** 本文的候选方向均为后续输入，不代表已实施或已验证的修复。

#### 10. 候选修复方向（待决策，未实施）

以下仅记录方向，均未编写或应用修复代码，也不代表已确认方案。

| 方向 | 思路 | 主要风险／待验证点 |
|------|------|------------------|
| A. 修复无池时 `addPool()` 写入 `0` | 无可用池时禁用「添加素材池」并给出「请先在规则素材池创建素材」引导，或让 `addPool()` 在候选为空时直接提示并返回；与 R35-02 已确认口径一致 | 需与 R35-02 的“保留手动规则路径”一起设计；不能用页面级无池硬阻断误伤“无池 + 手动规则” |
| B. 在 `onPoolsChanged()` 中修复无效已选值 | 新池列表返回后，若 `form.pools` 中存在不在 `validIDs` 内的 `pool_id`（包括 `0`），清空该行或提示用户重新选择；不要静默替换有效选择 | 需要决定“自动选中新池”还是“要求用户手动选择”；必须保留目标组、顺序和预览失效语义；会影响已删除池的现有提示逻辑 |
| C. 为返回刷新和 `0` 残留补测试 | 在 `assembly-view.spec.ts` 中覆盖 `mainTab` 从 `pool` 到 `build` 触发 `listPools`、候选更新、`0` 行保留与整页重挂载对照；在 `pool-tab.spec.ts` 覆盖 `urls:null` | 组件测试需要 mock `listPools` 与 `PoolTab` 事件；浏览器回归应断言下拉选项和当前值，而不只断言文本 |
| D. 重新构建并替换 8080 的前端产物 | 用当前前端重新构建并更新 `backend/web/dist`／部署镜像，使 8080 不再服务 2026-08-25 的旧构建；验证 `AssemblyView` 分块包含 `pools-changed` | 属于环境/部署修复，不是代码修复；需确认 8080 的用途、构建来源和是否允许替换；本轮未执行 |
| E. 修复空池 `urls:null` 渲染 | 后端列表对无来源池返回空数组，或前端表格使用 `(record.urls ?? []).length`、`record.urls?.length ?? 0` | 与本问题不同根因；需要决定后端契约还是前端容错；应单独验证空池、URL 池和同步池三种列表状态 |
| F. 跨浏览器标签页同步 | 在 `visibilitychange`／focus 时刷新池列表，或引入跨标签页通知 | 超出 R35-03 的“应用内页签”范围；可能增加请求和状态冲突，暂不建议作为默认方向 |

**后续验证建议：**

- 先确认用户实际访问的构建（8080 旧构建／当前 HEAD 构建）以及创建前是否点击过「添加素材池」；这决定优先修 A/B 还是 D。
- 若采用 A/B，增加“无池 + 添加素材池”“创建后返回”“整页刷新后重走”的浏览器回归，断言当前选择值和下拉选项，而不只是页面是否出现池名。
- 修复后重跑前端 `npm test` 和 `npm run build`；后端无改动时至少保持 `go build ./...`、`go vet ./...` 通过。若修改后端 `urls` 契约，补 `go test ./...` 与列表响应测试。
- 若采用 D，记录替换前后的 `index.html`／`AssemblyView` 哈希，并在 8080 上复验“创建 → 返回 → 下拉出现新池”，避免只更新静态文件但未重启或未替换嵌入产物。

#### 11. 支持证据摘要

| 场景 | 实际请求／响应或页面观察 |
|------|--------------------------|
| 当前 HEAD 干净流程 | 创建池后返回，`GET /api/admin/pools` 返回新池；下拉选项含新池，当前值即为新池 |
| 当前 HEAD `0` 前置路径 | 返回后 `GET /api/admin/pools` 返回新池；下拉选项含新池，但当前值仍为 `0`；整页刷新后当前值为新池 |
| 当前 HEAD 应用内切页 | 导航到 `/admin/nodes` 再返回，重新 `GET /api/admin/assembly/context`，外部 G 池出现在下拉 |
| 当前 HEAD 其他浏览器页签 | 第一页签不重挂载，候选不含 H 池；整页刷新后含 G、H 池 |
| 当前 HEAD 已选池新增条目 | 返回后提示“配置已变化，请重新预览”；点击「刷新预览」后正文包含新条目 |
| 旧构建创建池后返回 | 池行显示 `0`，展开为「暂无数据」；返回构建页时没有 `/api/admin/pools` 或 `assembly/context` 请求 |
| 旧构建整页刷新 | 重新请求 `assembly/context`，返回 OldDist 池；池行显示该池并被默认选中 |
| 旧构建代码证据 | `AssemblyView-9uejEL3P.js` 无 `pools-changed`／`pool-content-changed`；当前构建分块有 2／3 个对应字符串 |
| 无 URL 空池 | 列表 `urls:null`；console `TypeError: Cannot read properties of null (reading 'length')`；池列表空白，整页刷新仍空白；构建页下拉仍可选择该池 |
| 当前 HEAD 定向测试 | 4 个测试文件、50 项通过（assembly-view 20、pool-tab 7、pool-detail 22、rules-step 1） |

以上浏览器、API 与网络观察均来自 2026-11-16 的临时隔离实例；请求体中的节点、平台、规则和账号均为本地测试数据。关键截图保存在 `/tmp/r35-03-pool-refresh/evidence/`（如 `D2-return-dropdown.png`、`F3-return-zero.png`、`F4-after-reload.png`、`OLD3-after-reload.png`、`OLD4-dropdown-before-reload.png`、`H1-empty-pool-blank.png`），未加入仓库。

#### 12. 本轮未做

- 未修改任何项目代码、配置、依赖或锁文件；
- 未实施任何候选修复，也未将问题标记为已修复；
- 未操作生产、预发布、外部服务、真实账号或真实数据；
- 未运行 Docker 镜像、远端 CI 或真实客户端连接；
- 未重新构建当前 HEAD 前端（沿用 R35-02 同源构建；`7660292..HEAD` 前后端 diff 为空）；
- 唯一持久化变更为本 `Issue21.md` 文档更新。

### R35-04 VLESS 节点开启 UDP 后生成的 Clash 配置缺少 UDP 字段

- **现象（原始记录，保留）：** 用户截图中，Clash YAML 生成版本的 VLESS 节点没有 `udp` 和 `network` 字段；用户确认生成该版本时节点编辑器的「UDP」开关已开启。
- **预期（原始记录，保留）：** 节点启用 UDP 时，生成的 Clash/Mihomo 配置应体现该设置，使该节点允许转发 UDP 流量。
- **文档核对（原始记录，保留）：** 截图是 Clash/Mihomo YAML，而非 Xray JSON。[Mihomo VLESS 文档](https://wiki.metacubex.one/config/proxies/vless/)说明不配置 `network` 时使用 TCP，因此未显式输出 `network: tcp` 本身不代表传输方式遗漏；[Mihomo 出站通用字段文档](https://wiki.metacubex.one/config/proxies/)说明 `udp` 默认关闭，所以截图中省略 `udp` 与用户确认的开启状态不一致。项目固定目标 [Mihomo v1.19.31 的 VLESS 实现](https://github.com/MetaCubeX/mihomo/blob/v1.19.31/adapter/outbound/vless.go)也将 `udp` 用作独立布尔选项并以 TCP 为默认传输。Xray 的 [VLESS 出站](https://xtls.github.io/en/config/outbounds/vless.html)和[传输配置](https://xtls.github.io/en/config/transport.html)使用自身配置结构，不能将其字段名直接套用到 Mihomo YAML。
- **代码核对（原始记录，保留）：** VLESS 注册表将 UDP 表单默认值设为 `true`、传输默认值设为 `tcp`；Clash VLESS 适配器会从输入参数复制 `udp` 和 `network`，且只复制已存在的非零值。尚未核实截图对应节点从编辑、保存到生成的各阶段数据，因此不能据此确定字段在哪一环丢失。
- **复现入口（原始记录，保留）：** 本地 Production 模式，生成并查看启用 UDP 的 VLESS 节点对应 Clash 订阅版本内容。
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本次核对文档与相关代码，未独立复现完整生成链路或实际连接，也未修复。
- **当前状态（2026-11-16 研究后）：** 已完成从节点新建／编辑、保存、持久化、重新打开到 Clash/Mihomo 订阅版本生成的全链路复现，并用固定版本 Mihomo v1.19.31 验证了缺失字段的实际语义。根因已确认，**尚未修复**。结论为：VLESS 表单把 `udp` 的 schema 默认值 `true` 仅用于开关显示，未写入 `protocol_json`；后端保存不物化表单默认值；输出投影与 Clash VLESS adapter 只复制已存在的 `udp: true`，不会按 schema 默认补齐；Mihomo v1.19.31 对缺失的 `udp` 按 `false` 处理。因此「界面开关显示开启」与「最终配置允许 UDP」不一致。`network` 缺失是另一条独立语义：Mihomo 默认 TCP，省略 `network` 与显式 `tcp` 等价；截图中两者同时缺失与「新建后未再编辑保存」的默认负载一致，但不能据此认定传输方式也丢失。详见下文。

#### 1. 结论摘要

- **已实际复现，且不是保存失败。** 在当前 HEAD 的隔离 Production 构建中新建 VLESS 节点，只填写必填字段，不改动「UDP」和「传输」的显示默认值，保存请求的 `protocol_json` 只有 `uuid`；数据库、重新读取、装配输入、adapter 输出和最终版本 YAML 都没有 `udp` 和 `network`。表单中「UDP」开关的 `aria-checked` 为 `true`，但其值从未进入保存请求。
- **显式开启 UDP 可以正常输出。** 在同一新建表单中，将「UDP」从显示开启点成关闭、再点成开启后保存，请求、数据库、adapter 输出和最终 YAML 都包含 `udp: true`；固定 Mihomo 解析为 `udp: true`。这说明 adapter 的字段复制逻辑本身没有坏，问题只在「未显式写入」这条路径。
- **显式关闭与未写入在最终 YAML 中都无法区分，但语义相同。** 点一次关闭后保存，数据库保存 `udp: false`，adapter 按既有「只输出非零值」规则省略该字段；最终 YAML 同样没有 `udp`，Mihomo 也解析为 `false`。与之不同，schema 默认 `true` 的「未写入」同样被 Mihomo 解析为 `false`，这正是缺陷所在。
- **根因是“展示默认值”与“wire 默认值”不一致，而不是字段改名、传输方式或数据库问题。** schema 声明 `udp` 默认 `true`，UI 据此把开关显示为开启；保存链路按现有设计不批量物化展示默认值；输出层又没有像 VMess 的 `alterId`／`cipher` 那样为 schema 默认值与 Mihomo 零值不一致的字段补齐默认，最终落到 Mihomo 的 `udp` 默认 `false`。
- **`network` 和 `udp` 必须分开判断。** `network` 是传输方式（TCP／WS／gRPC／HTTP/2／HTTP／XHTTP），VLESS 省略时 Mihomo 按 TCP 处理；`udp` 是“是否允许 UDP 通过该出站代理”的独立开关，Mihomo 默认 `false`。本次实测 WS 节点同样在未显式开启时不输出 `udp`，显式开启后输出 `udp: true`，说明两者互不替代。
- **编辑路径会物化 `network`，但仍不会物化 `udp`。** 打开已有节点再保存时，前端 `openEdit()` 会从 `current_state` 回填 `network: tcp`（以及表单层 `security`），因此之后的 YAML 会出现 `network: tcp`，但 `udp` 仍保持缺失；再次打开时开关仍因 schema 默认显示为开启。这个不对称可以解释为什么有些截图缺少 `network`，有些编辑后的版本只缺少 `udp`。
- **同一模式已在 VMess 复现。** VMess 新建时只填 `uuid`，保存请求与最终投影都没有 `udp`；显式开启后预览出现 `udp: true`。源码层面还有 10 个 manual 协议也把 `udp` schema 默认声明为 `true`，但本次没有逐协议生成验证，不能把 VLESS／VMess 的实测结论直接等同于全部协议已确认受影响。
- **固定 Mihomo v1.19.31 已验证解析语义，但未做真实 UDP 连接。** 用固定二进制对隔离生成的默认、显式开启、显式关闭版本执行 `-t` 均成功；启动隔离 Mihomo 并通过本地 `/proxies` API 读取，得到缺失/显式关闭为 `udp: false`、显式开启为 `udp: true`。这证明配置解析后的开关值，不等同于真实 UDP 流量可转发；本次没有真实 VLESS 服务端、没有外网连接，也没做端到端 UDP 测试。
- 本轮只做测试、研究与文档记录，**未修改任何项目代码、配置、依赖或锁文件，未实施修复，也未将问题标记为已修复**。

#### 2. 代码版本、运行环境与测试数据前提

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `7cca913`（`git describe` 为 `v4.1.4-4-g7cca913`）；最后一个代码提交为 `1ceea52`；`1ceea52..HEAD` 与 `7660292..HEAD` 在 `backend/`、`frontend/` 下均无代码差异 |
| 工作区状态 | 研究开始时工作区干净；本轮结束前唯一持久化改动为本文档 |
| 前端依赖 | Vue 3.5.44、Vite 7.3.7、Tailwind CSS 4.3.3、`@tailwindcss/vite` 4.3.3、`ant-design-vue` 4.2.6 |
| 后端/运行时 | Go 1.26.4（darwin/arm64）；Node.js 26.7.0；macOS 27.2 |
| 浏览器/驱动 | Google Chrome 157.0.8103.52；Playwright-core 1.57.0（安装在临时目录，未加入项目依赖） |
| 隔离实例 | 当前 HEAD 源码在 `/tmp/r35-04-vless-udp` 构建；`APP_MODE=prod`、临时 `DATA_DIR`、`PORT=18120`，仅监听 `127.0.0.1` |
| 固定 Mihomo | `/Users/kyle/Desktop/Repo/Temp/mihomo-darwin-arm64-go124-v1.19.31`；`Mihomo Meta v1.19.31 darwin arm64 with go1.24.13`；SHA-256 `3ed36cabe89783acc24dbb3b411a9a0a6fb6353e97ea9d129a707c9bb9b147c5` |

**构建产物。** 为避免改动工作区，前端与后端都在 `/tmp` 临时副本中构建；前端通过软链接使用工作区现有 `node_modules`，未修改其内容：

| 产物 | 说明 | 关键哈希 |
|------|------|----------|
| 前端 `index.html` | 当前 HEAD 在 `/tmp/r35-04-vless-udp/frontend` 执行 `npm run build` | `330ac0fb45326003e263c6397faebd1e96f012f8a572b90c3f1a6272eba85edd` |
| 主 CSS `index-CFZtmKWH.css` | 同一构建产物 | `3b4626a471f35d7de8e6764bb1566c3ab34697a0ac8c60ea2e7a9c83213c516b` |
| Go 服务 `/tmp/r35-04-vless-udp/server` | 当前 HEAD 后端源码 + 上述前端产物嵌入，`CGO_ENABLED=0 go build` | `34c4327ea3a7251e241c6a418917c487f6427ca4d252580343f600bdfae53e19`；约 54 MB |

**测试数据前提。** 测试库由仓库内本地测试库 `backend/data/app-prod.db` 通过 `sqlite3 -readonly ... ".backup"` 复制到临时目录；源库未写入或清空。复制时库内已有：

- 合成管理员 `admin@test.com`，密码为 [TestPasswordList.md](docs/Reference/TestPasswordList.md) 规定的统一测试密码；
- 3 个平台、1 个 Clash 订阅（平台 1、订阅 1、当前版本 1）、1 个手动 VLESS 节点 `HK`、9 个启用预设代理组、0 个规则素材池。

本次新建的节点均为合成数据：`R35-04-A-default`、`R35-04-B-explicit-on`、`R35-04-C-explicit-off`、`R35-04-D-ws-default`、`R35-04-E-vmess-default`、`R35-04-F-default-2`、`R35-04-G-import`；主机名使用 `*.example.com`，UUID 使用公开的合成值。所有节点、订阅版本、生成配置都只写入 `/tmp` 临时库和临时内容目录；没有操作生产、预发布、外部服务、真实账号或真实数据。

**版本保留边界。** 该订阅已有版本 1；本轮陆续生成版本 2～7。版本服务按资源保留最多 5 版，生成 v7 时淘汰了最旧的 v2／v3，因此下文对默认路径引用仍可复核的 v7，对显式开启引用 v6，对显式关闭引用 v4；v2／v3 的预览响应与生成结果记录仍保留在证据 JSON 中，不影响根因结论。

#### 3. 复现步骤

1. 将当前 HEAD 的 `frontend/`、`backend/` 复制到 `/tmp/r35-04-vless-udp`，前端执行 `npm run build`，把 `dist` 放入临时后端 `web/dist`，再执行 `CGO_ENABLED=0 go build -o /tmp/r35-04-vless-udp/server ./cmd/server`。
2. 复制本地测试数据库到临时 `DATA_DIR`，启动隔离服务：`PORT=18120 APP_MODE=prod DATA_DIR=/tmp/r35-04-vless-udp/data ./server`；只通过 `127.0.0.1:18120` 访问。
3. 浏览器登录 `admin@test.com`，进入 `/admin/nodes`。新建 VLESS 节点，只填写名称、服务器、端口和 UUID，**不点击**「UDP」或「传输」；确认「UDP」开关显示为开启后保存。
4. 再新建两个 VLESS 节点：一个把「UDP」从显示开启点成关闭、再点成开启（显式开启）后保存；另一个只点一次关闭（显式关闭）后保存。记录每步的 POST 请求、响应和数据库 `protocol_json`。
5. 重新打开默认节点，先只修改服务器地址并保存，观察请求是否带 `udp`／`network`；再重新打开，显式关闭再开启 UDP 后保存，观察请求和后续 YAML 的变化。
6. 新建一个 VLESS 节点，把「传输」从默认 TCP 改为 WebSocket，保持 UDP 显示开启但不点击，保存并预览；随后编辑该节点，显式开启 UDP 后再次预览，作为传输方式对照。
7. 新建一个 VMess 节点，只填 `uuid` 等必填项，保持 UDP 默认显示开启，保存并预览；再显式开启 UDP 后预览，作为协议对照。
8. 用 `/api/admin/nodes/import` 导入一条含 `udp=1` 查询参数的合成 VLESS URI，检查导入后的节点 JSON 与预览，作为 URI 导入路径对照。
9. 通过 `POST /api/admin/assembly/preview` 和 `/generate` 对上述代表节点生成 Clash YAML；`preview` 请求体使用 `target_syntax=clash-yaml`、`platform_id=1`、单节点 `node_names`、同名 `overseas_members`／`fallback_group_members`，并附带一条 `DOMAIN-SUFFIX,example.net,🚀直接连接` 手动规则以满足生成门槛。
10. 保存 `generate` 返回的 `preview_hash` 与版本号；从 `DATA_DIR/contents/subscription/1/vN` 读取最终 YAML，与预览正文比对。
11. 用固定 Mihomo v1.19.31 对最终 YAML 执行 `-t`；再在隔离目录启动 Mihomo，配置仅绑定回环地址和本地 `external-controller`，通过 `/proxies` API 读取解析后的 `udp`／`xudp` 值。未发起真实代理连接。

#### 4. 各阶段实际 UDP 值

##### 4.1 新建表单的三种状态

| 阶段 | F 默认未改写 | B 显式开启 | C 显式关闭 |
|------|--------------|------------|------------|
| 表单「UDP」开关 `aria-checked` | `true`（仅 schema 默认，未写入） | `true`（点两次后） | `false`（点一次后） |
| POST `/api/admin/nodes` 的 `protocol_json` | `{"uuid":"<synthetic>"}` | `{"uuid":"<synthetic>","udp":true}` | `{"uuid":"<synthetic>","udp":false}` |
| `current_state` | `{"network":"tcp","security":"none","plugin":null,"features":[]}` | 同左 | 同左 |
| 数据库 `nodes.protocol_json` | `{"reality-opts":{},"uuid":"enc:v1:..."}`，无 `udp` | `{"reality-opts":{},"udp":true,"uuid":"enc:v1:..."}` | `{"reality-opts":{},"udp":false,"uuid":"enc:v1:..."}` |
| `ProjectActive`／adapter 输出 | 无 `udp` | `udp: true` | 无 `udp`（显式 `false` 被活动值过滤） |
| 最终 YAML | 无 `udp`（v7） | B 的 v3 预览为 `udp: true`，v3 生成后的版本正文在淘汰前读取到 `udp: true`；另用 A 的 v6 作为可复核的显式开启 YAML 样本（含编辑回填的 `network`） | 无 `udp`（v4） |
| 固定 Mihomo `/proxies` | `udp: false` | `udp: true` | `udp: false` |

**F 的最终 YAML（v7，默认未改写）：**

```yaml
proxies:
- name: R35-04-F-default-2
  type: vless
  server: f.example.com
  port: 443
  uuid: 99999999-8888-7777-6666-555555555555
```

**A 显式开启后的最终 YAML（v6，含一次编辑回填的 `network`）：**

```yaml
proxies:
- name: R35-04-A-default
  type: vless
  server: a-edited.example.com
  port: 443
  network: tcp
  udp: true
  uuid: 11111111-2222-3333-4444-555555555555
```

**C 显式关闭的最终 YAML（v4）：**

```yaml
proxies:
- name: R35-04-C-explicit-off
  type: vless
  server: c.example.com
  port: 443
  uuid: 11111111-2222-3333-4444-555555555555
```

以上三份文件都包含完整的 `proxy-groups` 与 `rules`；这里只摘录节点部分。v4／v6／v7 均通过固定 Mihomo `-t`。

##### 4.2 编辑、传输、协议与导入路径

| 路径 | 保存请求关键字段 | 数据库／重新打开 | 输出与验证 |
|------|------------------|------------------|------------|
| F 新建后直接生成 | `protocol_json` 只有 `uuid` | 无 `udp`；按 schema 默认仍会显示 `true` | v7 无 `udp`；Mihomo `udp:false` |
| A 只改 host 后保存 | `protocol_json` 含 `network:"tcp"`、`security:"none"`，**无 `udp`** | 无 `udp`；开关仍显示 `true` | v5 有 `network: tcp`、无 `udp`；Mihomo `udp:false` |
| A 显式开启 UDP 后保存 | `protocol_json` 含 `network:"tcp"`、`udp:true` | `udp:true`；重新打开显示开启 | v6 有 `network: tcp` 与 `udp:true`；Mihomo `udp:true` |
| D 新建 VLESS+WebSocket，不碰 UDP | `protocol_json` 含 `network:"ws"`，无 `udp` | 无 `udp`；开关显示 `true` | 预览有 `network: ws`、无 `udp` |
| D 显式开启 UDP 后保存 | `protocol_json` 含 `network:"ws"`、`udp:true` | `udp:true`；重新打开显示开启 | 预览有 `network: ws` 与 `udp:true` |
| E 新建 VMess，不碰 UDP | `protocol_json` 只有 `uuid` | 无 `udp`；开关显示 `true` | 预览有 `alterId: 0`、`cipher: auto`，无 `udp` |
| E 显式开启 UDP 后保存 | `protocol_json` 含 `network:"tcp"`、`udp:true` | `udp:true` | 预览有 `udp:true` |
| G 导入 VLESS URI（含 `udp=1`） | — | 导入结果 `protocol_json` 含 `network:"tcp"`、`tls:false`、`encryption:"none"`，无 `udp` | 预览无 `udp` |

**D 的 WebSocket 对照。** 新建时只改变传输方式、保持 UDP 显示开启但不点击，预览中出现 `network: ws`，没有 `udp`；编辑后显式开启 UDP，预览同时出现 `network: ws` 与 `udp: true`。这直接证明 `udp` 与 `network` 是独立字段，不能用 `network` 的有无推断 UDP 转发能力。

**E 的 VMess 对照。** 新建时请求只有 `uuid`，预览由 VMess adapter 补齐了固定 tag 必须存在的 `alterId: 0` 和 `cipher: auto`，但没有补 `udp`；显式开启后预览出现 `udp: true`。这与 VLESS 的失败模式一致，说明它不是 VLESS adapter 独有的字段复制错误，而是“schema 默认 true 未在输出层补齐”的通用问题。

**G 的导入路径。** 导入 URI 为 `vless://<synthetic-uuid>@g.example.com:443?type=tcp&security=none&udp=1#R35-04-G-import`，接口返回 200；导入后的节点 JSON 没有 `udp`，预览也没有。源码中 `parseVLESS` 只处理 `type`／`security`／`sni`／`flow`／`fp`／`alpn`／`path`／`host`／`serviceName`／`mode` 等查询参数，没有读取 `udp`。该 URI 的 `udp` 查询参数是否属于项目应支持的 VLESS URI 扩展，尚无产品约定，因此这里只记录为“同一条字段缺失路径的实测边界”，不把它单独认定为已确认缺陷。

#### 5. 根因分析

1. **表单把 `udp: true` 当作显示默认值，而不是待保存值。** `frontend/src/components/ProtocolFieldEditor.vue:780` 的普通 bool Switch 使用 `Boolean(modelValue ?? field.default ?? false)`，所以 VLESS 的 `udp` 在没有存储值时显示为开启；但 `frontend/src/views/admin/NodesView.vue:918-920` 的 `fieldModelValue()` 只返回实际 `protocol_json` 值，`frontend/src/utils/nodeProtocolPayload.ts:26-47` 的 `buildProtocolPayload()` 也只克隆已存在的参数，不会把带默认值的普通 bool 批量写入请求。`frontend/src/utils/nodeFormLayout.ts:57-60` 的 `isTriStateBool` 只判定“无 default 的 bool”，`udp` 有 default，因此既不会显示“未设置”，也不会被编码为 `null` 或默认值。`frontend/src/views/admin/NodesView.vue:650-660` 的新建表单初始 `protocol_json` 为 `{}`。
2. **保存链路按现有设计不物化 schema 默认值。** 后端 `backend/internal/node/node.go:342-380` 的 `CreateManual` 走 `NormalizeProtocolJSON`；`backend/internal/node/normalize.go:31-55` 只做别名、列表、字节序列和凭据身份等归一化，不应用 schema 默认；`backend/internal/node/project.go:1390-1411` 的 `normalizeProtocolParameters` 也不补默认。`backend/internal/node/project.go:1561-1582` 的 `protocolParamsForStorage` 只处理显式 `security`／`tls` 兼容。因此请求里没有 `udp` 时，数据库也不会出现 `udp`。这本身与 [Design4.md](Design4.md) 第 156 行“合法缺省仍可显示，但不把展示默认值批量写入数据”的设计一致；缺陷在输出层没有把“合法缺省”映射为 Mihomo 的实际语义，而不是保存层擅自丢字段。
3. **输出投影会跳过缺失键。** `backend/internal/assembly/links.go:9-20` 的 `activeProtocolJSON` 调用 `node.ProjectActive`；`backend/internal/node/project.go:25-51` 只遍历 schema 中实际存在且状态匹配的键，`projectFieldValue` 在键缺失时不会被调用。因此 `udp` 缺失时不会在投影阶段变成 `true`，`network` 缺失时也不会变成 `tcp`。
4. **Clash VLESS adapter 只复制“活动且为 true”的字段。** `backend/internal/assembly/clash_protocols.go:185-201` 的 `vlessClashAdapter` 通过 `copyClashActiveFields` 复制 `udp`；`backend/internal/assembly/clash_protocols.go:797-806` 的实现要求键存在且 `clashValueActive` 为真，`backend/internal/assembly/clash_protocols.go:808-836` 对 bool 只在 `true` 时返回真。因此数据库没有 `udp` 时不会输出 `udp`，显式 `false` 也会被省略。该“省略 false”本身符合 Mihomo 的零值语义，但前提是该字段的 schema 默认值与 wire 零值相同；`udp` 恰好不满足。
5. **Mihomo 的 wire 零值是 `false`，与 schema 默认 `true` 冲突。** 固定 v1.19.31 的 `VlessOption.UDP` 是 `bool` 且带 `proxy:"udp,omitempty"`；缺失时取零值 `false`。`NewVless` 把这个值传入 `Base`，`Base.SupportUDP()` 返回 `b.udp`；`tunnel/tunnel.go` 在 UDP 连接匹配到代理后执行 `if metadata.NetWork == C.UDP && !adapter.SupportUDP() { continue }`，因此缺失 `udp` 会导致该 VLESS 节点不转发 UDP。`network` 则不同：`VlessOption.Network` 缺失时走 `switch` 的 `default` 分支，按 TCP 处理；所以省略 `network` 是安全的，省略 `udp` 不是。
6. **缺少与 VMess 默认补齐同类的输出层处理。** 项目已经在 `backend/internal/assembly/clash_protocols.go:264-283`（`clashAlterID`／`clashVMessCipher`）和 `backend/internal/assembly/render_clash.go:254-262` 中为 VMess 的 `alterId`／`cipher` 做了“schema 默认与 wire 零值不一致时在输出副本补齐”的处理，但 `udp` 没有对应的补齐。`backend/internal/node/registry.go:922-924` 明确声明 VLESS `udp` 默认 `true`、`network` 默认 `tcp`；这说明默认值是有意声明的，不是测试数据偶然产生。[Build32.md](docs/reports/Build/Build32.md) Step 20 已记录“显式 `udp:false` 省略，与固定 tag 的 `omitempty` 等价”的合同，但该合同没有覆盖“schema 默认 `true` 且键缺失”的情况；本次缺口正在这里。
7. **编辑路径的 `network` 回填加深了表象差异。** `frontend/src/views/admin/NodesView.vue:685-714` 的 `openEdit()` 会从 `current_state` 回填 `security` 和 `network`，所以编辑并保存过的节点通常会在 YAML 中出现 `network: tcp`；但同函数没有回填 `udp`，开关继续依赖 schema 默认显示。结果就是：新建后直接生成可能同时缺少 `udp` 和 `network`；编辑一次后 `network` 出现，`udp` 仍缺失。用户截图中的“两者都缺失”与前者吻合，本地编辑复现则表现为“只缺 `udp`”；两者是同一根因的不同表象。
8. **不是以下原因：** 不是节点保存失败（POST 200，数据库可读）；不是凭据解密或读取失败（`uuid` 正常进入预览）；不是版本生成没有使用最新节点（每次生成前都重新保存并核对版本正文）；不是 `network: tcp` 被误删（显式保存 `network` 后会输出）；不是 Clash YAML 序列化问题（同一 adapter 在显式 `udp:true` 时能输出该字段）；也不是 Xray JSON 字段名被套用（截图和复现产物都是 Mihomo YAML）。

**交互后果：** 因为新建表单的「UDP」初始显示为开启，用户只点击一次实际会写入 `udp:false`；要在界面得到显式 `true`，需要先关闭再开启（本次 B 即两次点击），或通过高级 JSON 写入。也就是说，“看到开关开着”本身不能证明请求里写入了 `udp:true`。这不是根因，但会放大“界面已开启、产物却没有”的感知差异。

#### 6. `udp` 与 `network` 的语义和默认行为

| 字段 | 所属层 | 默认／缺失语义 | 与另一个字段的关系 |
|------|--------|----------------|--------------------|
| `udp` | Mihomo 出站代理的通用字段 | 默认 `false`；缺失与显式 `false` 在 Mihomo 中都表示不允许 UDP 通过该代理 | 与传输方式无关；TCP／WS／gRPC 等出站都可以在 `udp: true` 时承载 UDP |
| `network` | VLESS 协议的传输层字段 | 默认 TCP；缺失或非 `ws`／`http`／`h2`／`grpc`／`xhttp` 的值都按 TCP 处理 | 不能替代 `udp`；`network: ws` 只改变传输层，不会自动允许 UDP |
| `xudp`／`packet-addr`／`packet-encoding` | VLESS 的 UDP 包编码相关字段 | 由固定 tag 按输入推导；`xudp` 可在 `udp` 为 `false` 时仍为 `true` | 不是 UDP 转发开关；本次 Mihomo API 读取的 VLESS 节点都显示 `xudp: true`，同时 `udp` 分别为 `false`／`true` |
| 项目 schema 的 `udp` | 表单／内部活动模型 | `def("udp", "bool", "UDP", true)`，默认开启 | 与 `network` 的 `tcp` 默认独立声明 |
| 项目 schema 的 `network` | 表单／内部活动模型 | `sel("network", "传输", "tcp", ...)`，默认 `tcp` | 只决定传输方式；未显式保存时前端 `currentState` 仍补 `network: tcp` |

**结论。** 用户截图同时缺少 `udp` 和 `network`，其中 `network` 缺失本身是安全的（Mihomo 按 TCP），`udp` 缺失才是实质问题。不能因为 `network` 没有输出就推断传输方式被改坏，也不能把 `udp` 当成“UDP 传输方式”而与 TCP／WS 的选择混为一谈。

#### 7. 固定版本 Mihomo v1.19.31 验证

- **配置文件测试。** 对 v4（显式关闭）、v6（显式开启，含 `network: tcp`）、v7（默认未改写）执行固定二进制 `-t -f <version> -d <temp>`，三者均返回 `configuration file ... test is successful`，退出码 0。解析成功只证明结构可被固定 tag 接受，不代表真实 UDP 流量可转发。
- **解析后的实际开关值。** 将最终 YAML 前置回环 `mixed-port` 与本地 `external-controller`，启动固定 Mihomo 后读取 `/proxies`：
  - v7（默认未改写）：`{"type":"Vless","udp":false,"xudp":true}`；
  - v6（显式开启）：`{"type":"Vless","udp":true,"xudp":true}`；
  - v4（显式关闭）：`{"type":"Vless","udp":false,"xudp":true}`；
  - v5（只改 host、未写 UDP）：`{"type":"Vless","udp":false,"xudp":true}`。
- **源码依据。** 固定 tag 的 `VlessOption.UDP` 为 `bool`＋`omitempty`；`Base.SupportUDP()` 返回 `b.udp`；`tunnel` 对不支持的 UDP 代理会跳过。Mihomo 文档也写明通用字段 `udp` 默认 `false`，VLESS 文档写明 `network` 不配置或其他值时按 TCP。
- **验证边界。** 没有启动真实 VLESS 服务端、没有向外部网络发送 UDP、没有验证真实客户端的 UDP 转发链路；因此不能把“配置解析为 `udp:true`”写成“真实 UDP 已可转发”。本次也没有运行已发布镜像或 Docker Compose。

#### 8. 影响范围

**已实际复现：**

- manual VLESS 新建后不改「UDP」默认值，保存并生成 Clash YAML；
- manual VLESS 新建后显式开启／显式关闭 UDP；
- manual VLESS 编辑（只改 host、显式开启 UDP）后重新保存并生成；
- manual VLESS + WebSocket 传输的默认／显式开启对照；
- manual VMess 新建默认／显式开启对照；
- VLESS URI 导入（含 `udp=1` 查询参数）；
- 固定 Mihomo v1.19.31 的配置解析与 `/proxies` 语义读取。

**源码层面的潜在影响（未逐项实测）：**

- `backend/internal/node/registry.go` 中还有 10 个 manual 协议声明 `udp` 默认 `true`：`ss`、`trojan`、`socks5`、`wireguard`、`mieru`、`masque`、`tailscale`、`anytls`、`trusttunnel`、`openvpn`。它们各自使用显式 adapter，但本次没有逐个生成 YAML 验证其 `udp` 是否同样依赖 schema 默认；不能据此宣称全部已复现。
- Snell 的 `udp` schema 默认是 `false`，且 v1／v2 明确禁止；它与本次“默认 true 但 wire 默认 false”的问题不同，不应混入同一结论。
- Xray 来源节点的 `buildProtocolJSON`（`backend/internal/xray/detect.go:268-298`）没有生成 `udp`；下载重渲染的 `normalizeClashFields`（`backend/internal/assembly/render_clash.go:220-268`）只补 VMess 的 `alterId`／`cipher`，不补 `udp`。因此 Xray 来源的 VLESS／VMess 节点在动态重渲染路径上也可能出现同样缺失；本轮没有 Xray 实例或真实入站，未实测。
- VLESS URI 导入路径（`backend/internal/uriparse/uriparse.go:447-547`）没有处理 `udp` 查询参数；本次导入 `udp=1` 后节点 JSON 无 `udp`。该 URI 扩展的项目约定未确认，只能记录为潜在路径。
- SR 订阅、通用订阅、SR 分流规则、分享下载、已发布镜像和 Docker Compose 未覆盖；不能把 Clash YAML 的结论直接外推到这些目标。

**时序边界：** 版本内容是生成时的快照。若在生成版本后才编辑节点或开启 UDP，旧版本不会自动变化；本次每次生成前都先保存并核对最终文件，因此可以排除“先生成、后编辑”的时序因素。编辑节点不会回溯修改历史版本。

#### 9. 未确认内容与限制

- **用户原始环境和操作序列未提供。** 无法确定用户看到的是新建后未重新保存的默认状态、编辑后保存的状态、导入路径，还是某个旧构建产物。已知可解释路径包括：默认未写入；或生成发生在保存／开启之前（版本是生成时的快照，后者未作为独立实验重走）。用户截图中 `network` 也缺失，与“新建后未重新保存”的默认负载最一致；但这只是与已复现状态的吻合，不是对用户点击序列的证明。
- **用户是否实际点击过 UDP 开关未确认。** 本次证明：只要请求里写入 `udp: true`，后续 YAML 和 Mihomo 都会保留。若用户当时的保存请求确实写入了 `udp: true` 而最终仍缺失，则说明还存在本次未覆盖的额外路径；需要用户补充当时的请求、数据库或操作序列才能判断。
- **未获取用户截图对应的构建版本。** 不能确认是本地 Vite、工作区 `frontend/dist`、8080 旧内嵌构建还是已发布镜像；结论基于当前 HEAD 的本地隔离 Production 构建。
- **未做真实 UDP 连接测试。** 固定 Mihomo 只验证配置解析和 `/proxies` 开关值；没有真实 VLESS 服务端、没有 UDP echo／DNS 查询、没有验证真实客户端流量。配置能通过解析不等于真实 UDP 可转发。
- **未逐协议、逐来源验证。** 除 VLESS 和 VMess 外的 10 个 `udp` 默认 `true` 协议、Xray 来源、SR／generic、分享下载、已发布镜像均未实测；只记录了源码层面的相同条件。
- **URI 导入是否应支持 `udp` 未定。** 本次只观察到导入 `udp=1` 被忽略，未确认这是产品约定的不支持，还是解析器缺口；需要单独决策。
- **未验证编辑保存时 `udp` 与 `network` 的其他组合。** 例如在 TLS／REALITY、gRPC、HTTP/2、XHTTP 等分支下同时切换安全方式或传输；本次只覆盖了 VLESS TCP／WS 和 VMess TCP。
- **未做并发、缓存或版本保留专项测试。** 版本保留上限会淘汰旧版本，但不是本问题根因；未验证并发编辑、旧版本下载或外部缓存。
- **未实施或验证修复。** 下文候选方向只是后续工作输入，不代表已确认方案。

#### 10. 候选修复方向（待决策，未实施）

以下仅记录方向，均未编写或应用修复代码，也不代表已确认方案。

| 方向 | 思路 | 主要风险／待验证点 |
|------|------|--------------------|
| A. 在 Clash 输出层补齐 schema 默认值 | 对 `udp` schema 默认 `true` 的协议，在 adapter 构造 wire 副本时：缺失 → 输出 `udp: true`；显式 `true` → 输出；显式 `false` → 省略（等价于 Mihomo 默认 false）。可先只在 VLESS 实现，或提取共享 helper | 必须逐协议确认 `udp` 在目标 tag 中的实际语义、selector 分支（如 MASQUE h3-l4proxy 禁 UDP、Snell v1/v2 禁 UDP）和固定 tag 的 `omitempty` 行为；不能误把 Snell 等默认 false 的协议改成默认 true；会影响所有使用这些 adapter 的预览、生成、下载重渲染和节点检查，需要全量回归 |
| B. 保存时物化 `udp` 默认值 | 新建／编辑保存时把 schema 默认 `true` 写入 `protocol_json`，让输出层继续按现有“只复制 true”逻辑工作 | 与 Design4 的“不把展示默认值批量写入数据”冲突；会改变存储语义、合并与清空行为，并影响旧节点；只能修复未来保存，不能修复已有节点；需要确认是否接受 `udp:true` 被显式持久化 |
| C. 调整表单默认或显示 | 把 `udp` 默认改为 `false`，或让开关显示“未设置／关闭”，要求用户显式开启 | 会改变用户当前看到的产品语义和默认体验；已有节点仍可能缺少字段；需要用户决定是否接受“默认不转发 UDP”；不适合作为无产品决策的默认修复 |
| D. 仅增加提示／检查门槛 | 在节点检查或装配前提示“UDP 显示开启但未持久化，生成后实际为 false” | 不修复配置结果，只提前暴露；会增加检查项与文案维护；不能替代 A 或 B |

**兼容性初步评估（未决策）：** 从现有设计看，A 最符合“展示默认值不批量落库”的约束，也有 VMess `alterId`／`cipher` 在输出层补齐先例；影响面主要在 adapter 与产物，但必须逐协议确认固定 tag 零值和 selector 分支。B 会改变数据库存储与更新合并语义，且只覆盖未来保存；C 会改变用户当前看到的默认行为；D 只能提示、不能修正产物。以上均未实施、未验证，不能视为已确认方案。

**后续验证建议（若用户决定修复）：**

- 为 VLESS 增加“默认未写／显式 true／显式 false”三组 adapter wire shape 测试，断言最终 YAML 分别为 `udp:true`／`udp:true`／无 `udp`，并断言 `network` 仍按原样输出。
- 在固定 Mihomo v1.19.31 上重新执行 `-t` 与 `/proxies` 读取；如条件允许，再用本地 VLESS 服务端做一次真实 UDP 转发验证，明确区分“解析成功”与“流量可转发”。
- 覆盖 VMess、SS、Trojan、SOCKS5、WireGuard、Mieru、MASQUE、Tailscale、AnyTLS、TrustTunnel、OpenVPN 的 `udp` 默认语义，先核对每个固定 tag option 的零值，再决定是否使用共享 helper。
- 覆盖 Xray 动态重渲染、VLESS URI 导入、SR／generic、分享下载和已发布镜像；对“缺失字段”和“显式 false”分别断言。
- 若采用 A，回归历史版本下载与 `RenderClashPlan` 重渲染，确认旧版本不会因重渲染突然增加 `udp`；若采用 B，补迁移或兼容策略，避免旧节点在新旧保存路径下行为不一致。

#### 11. 本轮未做

- 未修改任何项目代码、配置、依赖或锁文件；
- 未实施任何候选修复，也未将问题标记为已修复；
- 未操作生产、预发布、外部服务、真实账号或真实数据；
- 未运行 Docker 镜像、Docker Compose 或远端 CI；
- 未进行真实 VLESS 连接、真实 UDP 流量、真实客户端或人工真机验收；
- 唯一持久化变更为本 `Issue21.md` 文档更新。

#### 12. 支持证据摘要

| 场景 | 实际请求／响应或页面观察 |
|------|--------------------------|
| F 新建默认未改写 | 表单「UDP」`aria-checked=true`；POST `protocol_json={"uuid":"..."}`；DB 无 `udp`；v7 无 `udp`；Mihomo `udp:false` |
| B 显式开启 | 两次点击后 `aria-checked=true`；POST 含 `udp:true`；DB 含 `udp:true`；预览含 `udp:true`；v3 生成后的版本正文在淘汰前读取到 `udp:true` |
| C 显式关闭 | 一次点击后 `aria-checked=false`；POST 含 `udp:false`；DB 含 `udp:false`；v4 无 `udp`；Mihomo `udp:false` |
| A 只改 host 后保存 | PUT `protocol_json` 含 `network:"tcp"`、`security:"none"`，无 `udp`；v5 有 `network: tcp`、无 `udp`；Mihomo `udp:false` |
| A 显式开启后保存 | PUT 含 `network:"tcp"`、`udp:true`；v6 两者都有；Mihomo `udp:true`；重新打开开关为 true |
| D WebSocket 对照 | 新建预览 `network: ws`、无 `udp`；显式开启后预览 `network: ws`＋`udp:true` |
| E VMess 对照 | 新建 POST 仅 `uuid`；预览 `alterId:0`／`cipher:auto`、无 `udp`；显式开启后预览 `udp:true` |
| G URI 导入对照 | `POST /api/admin/nodes/import` 200；导入节点 `network:"tcp"`、`tls:false`、`encryption:"none"`、无 `udp`；预览无 `udp` |
| 版本正文 | v7（F）无 `udp`；v6（A）`network: tcp`＋`udp: true`；v4（C）无 `udp`；v5（A host-only）`network: tcp`、无 `udp` |
| 固定 Mihomo 配置测试 | v4／v6／v7 `-t` 均 successful，退出码 0 |
| 固定 Mihomo `/proxies` | v7 `udp:false`；v6 `udp:true`；v4 `udp:false`；v5 `udp:false`；四者均 `xudp:true` |
| 源码证据 | `registry.go:922-924` 默认 `udp:true`／`network:tcp`；`ProtocolFieldEditor.vue:780` 显示默认；`nodeProtocolPayload.ts:26-47` 不物化默认；`project.go:25-51` 跳过缺失键；`clash_protocols.go:797-836` 只复制 true |

浏览器与 API 观察均来自 2026-11-16 的临时隔离实例；关键截图与请求 JSON 保存在 `/tmp/r35-04-vless-udp/evidence/`（如 `F-form-before-save.png`、`B-form-before-save.png`、`C-form-before-save.png`、`A-edit-explicit-on.png`、`D-ws-form-before-save.png`、`E-vmess-form-before-save.png`、`nodes-create-evidence.json`、`edit-node-evidence.json`、`preview-generate-evidence.json`、`F-create-evidence.json`、`vmess-node-evidence.json`、`ws-node-evidence.json`、`import-evidence.json`），未加入仓库。配置中的 UUID、主机名和账号均为合成测试数据；未写入真实凭据。

### R35-05 节点管理的高级 JSON 切换样式与订阅装配不一致

- **现象：** 节点管理的对象字段编辑器使用「结构化编辑／高级 JSON」双按钮切换；订阅装配的 Clash 头部参数分区使用「高级 JSON」开关。用户提供的截图以节点的「多路复用」区域和装配的「端口配置」区域为例。
- **预期：** 节点管理中的高级 JSON 切换样式与订阅装配一致，使用开关类按钮。
- **检查范围：** 对照用户截图与前端组件：节点管理的对象字段由 `ProtocolFieldEditor.vue` 渲染双按钮，订阅装配的 `HeaderStep.vue` 对相关分区使用开关；对象字段编辑器也支持嵌套，后续处理时需检查节点管理内适用的对象字段及切换状态。
- **状态：** 待后续处理；本次仅检查并记录样式差异与用户预期，未修改代码或验证切换行为。

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
| 2026-11-16 | 按授权完成 R35-03 分路径研究：确认当前 HEAD 干净流程不复现（`watch(mainTab)` → `refreshPools()` 更新候选）；复现无池时先点「添加素材池」形成 `pool_id=0`、创建池后返回仍显示 `0`、整页刷新后才显示新池的残留路径；确认工作区 2026-08-25 旧内嵌构建早于 R24-21 修复，缺少 `pools-changed`／`refreshPools`，可精确复现候选列表不刷新；补充应用内切页、其他浏览器页签、已有池新增条目等对照，以及无 URL 空池 `urls:null` 导致列表空白的附带发现。 |
| 2026-11-16 | 更新文档定位与 R35-03 状态：研究完成、尚未修复；记录当前 HEAD 干净流程、`pool_id=0` 前置路径与旧构建路径的根因边界，明确用户原始环境待确认；本轮仅测试、研究与文档记录，唯一持久化变更为本 `Issue21.md`。 |
| 2026-11-16 | 记录 R35-04：用户确认 VLESS 节点已开启 UDP，生成的 Clash YAML 未显示该字段；核对 Xray 与 Mihomo 文档及相关代码，未确认丢失环节或实施修复。 |
| 2026-11-16 | 记录 R35-05：节点管理高级 JSON 双按钮与订阅装配开关样式不一致。 |
| 2026-11-16 | 按授权完成 R35-04 全链路研究：在当前 HEAD 的隔离 Production 构建中复现“新建 VLESS 默认未改写 → 保存请求仅含 UUID → 数据库/读取/投影/adapter/最终 YAML 均无 `udp`”，确认根因是 schema `udp:true` 的展示默认值与 Mihomo `udp` wire 默认 `false` 不一致，且保存与输出层都没有物化该默认；对照显式开启（`udp:true` 正常输出）、显式关闭（`udp:false` 被省略、语义仍为 false）、编辑回填 `network:tcp` 但 `udp` 仍缺失、WebSocket 传输、VMess 同模式、VLESS URI 导入 `udp=1` 被忽略等路径；用固定 Mihomo v1.19.31 `-t` 与 `/proxies` 读取验证 `udp:false/true` 语义及 `xudp:true` 的独立性，并明确未做真实 UDP 连接；记录影响范围、未确认边界与 A～D 候选方向。本轮仅测试、研究与文档记录，未修改代码、配置、依赖或锁文件，未实施修复。 |
| 2026-11-16 | 更新文档定位与 R35-04 当前状态：根因已确认，尚未修复；`udp` 与 `network` 的语义、固定版本验证结果及候选修复方向已记录，原始反馈与历史记录保留。 |
| 2026-11-16 | 完成 R35-01 修复前扩查、前端布局修复和本地复测。依据用户选择，将两处最后一行 scheme 到添加按钮的间距统一为 12px。 |
