# Issue21.md — 本地调试问题记录

> **文档定位：** 记录本地调试中发现的问题及其研究、修复和验证结果。已归档的问题记录见 [docs/reports/Issue/](./)，项目指令见 [AGENTS.md](../../../AGENTS.md)。
>
> **归档状态（2026-11-24）**　R35-01～R35-26 共 26 项均已修复并通过各自的本地验证，含 R35-01 的首页余量卡片间距补充修复。服务器剩余流量的真实平台验收已由用户确认通过，见 [Build41 Step 7](../Build/Build41.md#step-7--真实平台与出网验收) 与 [Build42 Step 2](../Build/Build42.md#step-2--两家真实平台与目标容器)。R35-08／R35-09 的 Docker 验收范围及环境边界见 [Build39](../Build/Build39.md) 与 [Build40](../Build/Build40.md)。
>
> **当前状态（2026-11-17）：** R35-01～R35-04 已通过本地验证。R35-02 补充 Clash 装配前的空素材提示，见第 13 节。R35-03 原始反馈来自本机 8080 的旧前端，新构建已替换并复验，素材池快照、列表乱序和预览失效的相邻问题已修复。R35-04 已修复手动 VLESS／VMess 的 Clash UDP 缺省输出，验收见第 13 节。R35-05 已按确认方向实施并完成本地自动化与隔离浏览器验收（见 R35-05 第 12 节及 [Build36.md](../Build/Build36.md)）。R35-06 已修复节点推荐选项显示 Mihomo 版本及其他验证来源的问题，通过本地自动化与隔离浏览器验收（见 R35-06 第 13 节及 [Build37.md](../Build/Build37.md)）。R35-07 已完成本地修复与验收。目标检查位于独立内联区域，结果默认折叠为汇总，重新检查会清空上一轮结果（见 R35-07 第 12 节及 [Build38.md](../Build/Build38.md)）。

> **Build41 Step 3（2026-11-24）** 采集结果发布、设置重新开启触发及 V.PS 缓存到期三项并发问题已修复，见 R35-13～R35-15 和 [Build41 Step 3](../Build/Build41.md#step-3--自动采集与轮换一致性)。
>
> **Build41 Step 4（2026-11-24）** 独立核验发现的设置写入、结果匹配、请求体和在途关停问题已修复，见 R35-16～R35-19 和 [Build41 Step 4](../Build/Build41.md#step-4--站内接口与生命周期)。
>
> **Build41 Step 5（2026-11-24）** 独立核验发现并修复脏草稿冒用新修订、旧确认解绑新实例以及旧页面采集新绑定三项问题，见 R35-20～R35-22 和 [Build41 Step 5](../Build/Build41.md#step-5--管理设置与首页展示)。
>
> **Build42 Step 1（2026-11-24）** 独立核验复现旧接口修订、手动采集时序、共享账号令牌及导入确认回归，已按确认方案修复，见 R35-23～R35-26 和 [Build42 Step 1](../Build/Build42.md#step-1--多主机绑定与折半展示)。

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

工作区已有产物的哈希与 [Issue20.md](Issue20.md) R34-01 表格记录的“最终候选”哈希不同，因此不能把该目录直接视为 Issue20 记录的同一份产物；它和临时重建产物都复现了问题。两者均使用 Tailwind 4.3.3 和同一套 `node_modules`。

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

#### 12. 首页服务器剩余流量卡片粘连（2026-11-24）

- 原容器在 1112 px 视口下，顶部摘要区到服务器剩余流量卡的间距为 16 px，余量卡到底下的平台列表为 0 px。Ant Design Vue 的 `margin: 0` 覆盖了卡片根节点上的 Tailwind `mb-4`，与 R35-01 同源。
- 首页改由纵向容器的 `gap-4` 统一设置相邻区块的间距。修复后用合成站内响应在 1112 px 和 375 px 检查生产构建，余量卡上下均为 16 px，隐藏时摘要区与平台列表之间仍为 16 px。读取失败和平台列表为空时的间距也通过复测。前端构建和全量测试结果见 [Build41.md](../Build/Build41.md#step-5--管理设置与首页展示)。

### R35-02 无素材池时 Clash 构建预检未阻断

- **现象（原始记录，保留）：** 用户反馈不存在规则素材池时，Clash 构建预检没有阻断；截图显示已进入「构建订阅/规则 → Clash - V2Ray/Mihomo（新版）→ 规则素材」步骤，素材池选择项显示为 `0`。
- **预期（原始记录，保留）：** 不存在素材池时，在预检阶段阻断 Clash 构建。
- **复现入口（原始记录，保留）：** 本地 Production 模式，进入 `/admin/assembly?tab=clash-yaml` 的 Clash 构建流程，在不存在素材池的情况下查看预检及规则素材步骤。
- **待确认（原始记录，保留）：** 截图显示能够进入规则素材步骤；后续预览、最终构建及其他条件下的具体行为尚未验证。
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本次仅记录用户反馈和预期，未研究原因或修复。
- **当前状态（2026-11-16）：** 已修复 Clash 与 SR 分流装配，按实际最终产物计算非兜底规则。用户确认 Clash 可只用覆盖层供规则，处在 `MATCH` 后面的合格规则提示顺序风险。下文第 1～11 节保留修复前的研究证据，第 12 节记录最终口径与修复。

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

- 合成管理员 `admin@test.com`，密码为 [TestPasswordList.md](../../Reference/TestPasswordList.md) 规定的统一测试密码；
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

- [Design2.md](../Design/Design2.md) 第 160 行仍写「规则为空允许生成（见 3.6 兜底规则仍有效）并在预览中提示」；[Design2-UI.md](../Design/Design2-UI.md) 第 338、357 行同样记录空规则允许生成。
- 较新的 [Design3.md](../Design/Design3.md) §7.2（第 434 行）明确规定「最终输出只统计素材池规则 + 自定义规则，不包含内置兜底；最终输出为 0 时禁止生成」；[Build22.md](../Build/Build22.md) 的 Step 6 已按该口径实现并补了 HTTP 回归测试。
- 当前代码实现的是 Design3 的后端生成门槛，但前端预检、预览警告和 `canGenerate` 没有同步 Design3 的「有效输出」概念，仍停留在「进入装配器后由用户自行发现」的状态。Design2／Design2-UI 的空规则文案与当前实现冲突，属于历史文档滞后，本轮未改写。

**初次研究时确认的产品口径（2026-11-16，「按有效规则源阻断」）：** Clash 对覆盖层独立供规则的补充决策见第 12 节。

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

#### 12. 实施范围与复测（2026-11-16）

- **修复前扩查。** 订阅管理、规则管理、版本管理和蓝图重新编辑都进入同一装配视图。Clash 与 SR 分流有相同的预览和生成控制缺口。两种纯节点订阅仍有自己的节点门槛，但复用表单时可能把隐藏的规则或组提交给后端。节点目标检查、历史 Clash 下载会调用 Clash 静态自检，版本上传和在线编辑使用独立入口。
- **最终口径。** Clash 无池时仍可走手动规则或覆盖层。SR 分流没有规则输入时在规则步骤提示。素材池和手动规则的转换统计取自实际渲染，Clash 在覆盖层后统计通过静态检查的顶层规则，SR 统计实际写出的分流规则。系统兜底排除。位于 `MATCH` 之后的合格规则计入并提示匹配顺序风险。缺失 `RULE-SET` 声明、无效目标等自检错误阻止生成。
- **关联修复。** 无池时禁用添加并提供素材池入口，恢复旧草稿时清除非正数池 ID。手工池的 `urls` 返回数组，列表和详情可显示零 URL。SR 添加池默认使用现有界面规格的 `PROXY`，后端限制两种 SR 策略。各目标只发送本次装配使用的选择，保留切换前的本地草稿。
- **状态一致性。** 正文、摘要、回执和自检随同一次预览更新。生成前重新核对本次产物与摘要，最后一条活动素材在预览后被删除时，旧摘要请求返回 409。
- **验证与构建记录。** 见 [Build33.md](../Build/Build33.md)。最终设计见 [Design3 §7.2](../Design/Design3.md#72-渲染和回执)。

#### 13. Clash 装配前的空素材提示（2026-11-17）

- 节点、平台和订阅前置条件就绪后，无池或各池均无已生效条目时显示「暂无规则素材」，等待用户选择「前往添加素材」或「继续装配」。前者打开规则素材池，返回时刷新条目状态。后者直接进入当前布局，可在手动规则或覆盖层补充规则。
- 提示使用素材池 API 的已生效条目计数。条目能否用于本次 Clash 装配由实际预览和生成校验决定。界面规格见 [Design2-UI §5.3.1](../Design/Design2-UI.md#531-clash-yaml-装配器)，构建和验收见 [Build33 Step 4](../Build/Build33.md#step-4--clash-装配前的空素材提示2026-11-17)。

### R35-03 创建素材池后返回 Clash 构建页面需要整页刷新

> **实施补记（2026-11-16）：** 用户确认原始反馈来自本机 8080。该实例曾内嵌缺少 R24-21 刷新逻辑的旧前端，现已替换并复验。R35-02 已修复 `pool_id=0` 和无 URL 空池显示。第 1～12 节保留研究阶段的记录，第 13 节记录后续复查与实施。

- **现象：** 用户反馈，在 Clash 构建页面切换到「规则素材池」创建素材后，再返回构建页面，无法立即获取新建素材；需要整页刷新后才能获得。
- **复现步骤：** 进入 `/admin/assembly?tab=clash-yaml` 的规则素材步骤；切换到「规则素材池」并创建素材；返回 Clash 构建页面，查看素材池选择项，再整页刷新查看。
- **预期：** 创建素材后返回构建页面，无需整页刷新即可获取新建素材。
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本次仅记录用户反馈，未独立复现、研究原因或修复。
- **初次研究阶段状态（2026-11-16）：** 完成分路径复现与根因研究。以下是实施前观察，按环境与操作前提区分：
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

- 合成管理员 `admin@test.com`，密码为 [TestPasswordList.md](../../Reference/TestPasswordList.md) 规定的统一测试密码；
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

#### 13. 后续复查、实施与本地验收

- **8080 原始现象。** 用户确认操作发生在本机 8080，服务返回旧分块 `AssemblyView-9uejEL3P.js`。隔离复现显示，素材池创建成功后切回构建页时没有池列表或上下文请求，整页刷新才通过 `assembly/context` 取得同一个池 ID。重新嵌入最新产物并替换服务后，同一流程可原步返回并选中新池。
- **池元数据修改。** 切换定时同步或仅改池名曾因全量替换来源而删除活动快照。改为行内开关仅更新 `auto_sync`，池编辑在事务内保留 URL 和模式均未变化的来源。改序使用新顺序读取活动规则。用户确认保留手动或定时同步。
- **列表返回顺序。** 复现晚到的旧空列表覆盖新候选，以及首次装配上下文就绪前丢弃新列表。现由装配页维护共享池列表，按请求顺序吸收新结果，保留草稿、步骤、已有选择和有效预览。
- **预览失效。** 手工条目提交后若在列表刷新期间关闭详情，旧预览仍可生成，后端返回 409。写入成功即通知父页，使旧预览立即失效。活动快照与顺序变化、已选池删除也触发失效，后端继续校验生成摘要。
- **验收。** 后端编译、vet、全量测试、定向 race、errgate，前端 51 个文件 469 项与生产构建均通过。隔离浏览器通过复现路径回归；8080 实际加载 `AssemblyView-DtNxS4Yj.js`，新建无 URL 池并返回选择成功，测试池已移除。分步构建与运行证据见 [Build34.md](../Build/Build34.md)。

### R35-04 VLESS 节点开启 UDP 后生成的 Clash 配置缺少 UDP 字段

- **现象（原始记录，保留）：** 用户截图中，Clash YAML 生成版本的 VLESS 节点没有 `udp` 和 `network` 字段；用户确认生成该版本时节点编辑器的「UDP」开关已开启。
- **预期（原始记录，保留）：** 节点启用 UDP 时，生成的 Clash/Mihomo 配置应体现该设置，使该节点允许转发 UDP 流量。
- **文档核对（原始记录，保留）：** 截图是 Clash/Mihomo YAML，而非 Xray JSON。[Mihomo VLESS 文档](https://wiki.metacubex.one/config/proxies/vless/)说明不配置 `network` 时使用 TCP，因此未显式输出 `network: tcp` 本身不代表传输方式遗漏；[Mihomo 出站通用字段文档](https://wiki.metacubex.one/config/proxies/)说明 `udp` 默认关闭，所以截图中省略 `udp` 与用户确认的开启状态不一致。项目固定目标 [Mihomo v1.19.31 的 VLESS 实现](https://github.com/MetaCubeX/mihomo/blob/v1.19.31/adapter/outbound/vless.go)也将 `udp` 用作独立布尔选项并以 TCP 为默认传输。Xray 的 [VLESS 出站](https://xtls.github.io/en/config/outbounds/vless.html)和[传输配置](https://xtls.github.io/en/config/transport.html)使用自身配置结构，不能将其字段名直接套用到 Mihomo YAML。
- **代码核对（原始记录，保留）：** VLESS 注册表将 UDP 表单默认值设为 `true`、传输默认值设为 `tcp`；Clash VLESS 适配器会从输入参数复制 `udp` 和 `network`，且只复制已存在的非零值。尚未核实截图对应节点从编辑、保存到生成的各阶段数据，因此不能据此确定字段在哪一环丢失。
- **复现入口（原始记录，保留）：** 本地 Production 模式，生成并查看启用 UDP 的 VLESS 节点对应 Clash 订阅版本内容。
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本次核对文档与相关代码，未独立复现完整生成链路或实际连接，也未修复。
- **修复前研究状态（2026-11-16）：** 已完成从节点新建／编辑、保存、持久化、重新打开到 Clash/Mihomo 订阅版本生成的全链路复现，并用固定版本 Mihomo v1.19.31 验证了缺失字段的实际语义。根因已确认，**尚未修复**。结论为：VLESS 表单把 `udp` 的 schema 默认值 `true` 仅用于开关显示，未写入 `protocol_json`；后端保存不物化表单默认值；输出投影与 Clash VLESS adapter 只复制已存在的 `udp: true`，不会按 schema 默认补齐；Mihomo v1.19.31 对缺失的 `udp` 按 `false` 处理。因此「界面开关显示开启」与「最终配置允许 UDP」不一致。`network` 缺失是另一条独立语义：Mihomo 默认 TCP，省略 `network` 与显式 `tcp` 等价；截图中两者同时缺失与「新建后未再编辑保存」的默认负载一致，但不能据此认定传输方式也丢失。详见下文。
- **当前状态（2026-11-16 实施后）** 手动 VLESS／VMess 的 Clash 适配器已应用表单的 UDP 缺省值，隔离 Production、固定 Mihomo 和本地回环 UDP 验证均通过。修复与版本边界见第 13 节。

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

- 合成管理员 `admin@test.com`，密码为 [TestPasswordList.md](../../Reference/TestPasswordList.md) 规定的统一测试密码；
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
2. **保存链路按现有设计不物化 schema 默认值。** 后端 `backend/internal/node/node.go:342-380` 的 `CreateManual` 走 `NormalizeProtocolJSON`；`backend/internal/node/normalize.go:31-55` 只做别名、列表、字节序列和凭据身份等归一化，不应用 schema 默认；`backend/internal/node/project.go:1390-1411` 的 `normalizeProtocolParameters` 也不补默认。`backend/internal/node/project.go:1561-1582` 的 `protocolParamsForStorage` 只处理显式 `security`／`tls` 兼容。因此请求里没有 `udp` 时，数据库也不会出现 `udp`。这本身与 [Design4.md](../Design/Design4.md) 第 156 行“合法缺省仍可显示，但不把展示默认值批量写入数据”的设计一致；缺陷在输出层没有把“合法缺省”映射为 Mihomo 的实际语义，而不是保存层擅自丢字段。
3. **输出投影会跳过缺失键。** `backend/internal/assembly/links.go:9-20` 的 `activeProtocolJSON` 调用 `node.ProjectActive`；`backend/internal/node/project.go:25-51` 只遍历 schema 中实际存在且状态匹配的键，`projectFieldValue` 在键缺失时不会被调用。因此 `udp` 缺失时不会在投影阶段变成 `true`，`network` 缺失时也不会变成 `tcp`。
4. **Clash VLESS adapter 只复制“活动且为 true”的字段。** `backend/internal/assembly/clash_protocols.go:185-201` 的 `vlessClashAdapter` 通过 `copyClashActiveFields` 复制 `udp`；`backend/internal/assembly/clash_protocols.go:797-806` 的实现要求键存在且 `clashValueActive` 为真，`backend/internal/assembly/clash_protocols.go:808-836` 对 bool 只在 `true` 时返回真。因此数据库没有 `udp` 时不会输出 `udp`，显式 `false` 也会被省略。该“省略 false”本身符合 Mihomo 的零值语义，但前提是该字段的 schema 默认值与 wire 零值相同；`udp` 恰好不满足。
5. **Mihomo 的 wire 零值是 `false`，与 schema 默认 `true` 冲突。** 固定 v1.19.31 的 `VlessOption.UDP` 是 `bool` 且带 `proxy:"udp,omitempty"`；缺失时取零值 `false`。`NewVless` 把这个值传入 `Base`，`Base.SupportUDP()` 返回 `b.udp`；`tunnel/tunnel.go` 在 UDP 连接匹配到代理后执行 `if metadata.NetWork == C.UDP && !adapter.SupportUDP() { continue }`，因此缺失 `udp` 会导致该 VLESS 节点不转发 UDP。`network` 则不同：`VlessOption.Network` 缺失时走 `switch` 的 `default` 分支，按 TCP 处理；所以省略 `network` 是安全的，省略 `udp` 不是。
6. **缺少与 VMess 默认补齐同类的输出层处理。** 项目已经在 `backend/internal/assembly/clash_protocols.go:264-283`（`clashAlterID`／`clashVMessCipher`）和 `backend/internal/assembly/render_clash.go:254-262` 中为 VMess 的 `alterId`／`cipher` 做了“schema 默认与 wire 零值不一致时在输出副本补齐”的处理，但 `udp` 没有对应的补齐。`backend/internal/node/registry.go:922-924` 明确声明 VLESS `udp` 默认 `true`、`network` 默认 `tcp`；这说明默认值是有意声明的，不是测试数据偶然产生。[Build32.md](../Build/Build32.md) Step 20 已记录“显式 `udp:false` 省略，与固定 tag 的 `omitempty` 等价”的合同，但该合同没有覆盖“schema 默认 `true` 且键缺失”的情况；本次缺口正在这里。
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

#### 10. 修复前的候选方向

以下保留修复前的方案对照。用户确认的实施口径见第 13 节。

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

#### 11. 修复前研究的边界

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

#### 13. 实施、影响范围与验收（2026-11-16）

- 用户确认在手动 VLESS 和 VMess 的 Clash 适配器处理 UDP。缺省与显式 `true` 输出 `udp: true`。显式 `false` 在活动投影后仍保留，由适配器省略，Mihomo 按关闭处理。本节更正研究记录第 4.1 节“显式 false 被活动值过滤”的位置描述。节点保存与表单沿用既有行为。
- 新生成版本应用该规则，已有手动节点无需重新保存或迁移。历史版本正文和手动节点渲染计划沿用生成快照。URI 导入的两个协议同属手动节点，装配遵循同一缺省规则。VLESS URI 查询参数仍由现有解析器处理。动态 Xray 注入、其他协议及其他目标沿用各自的路径。
- 隔离 Production 浏览器验证 VLESS 和 VMess 的缺省、显式开启、显式关闭六组用例。六份预览和生成正文逐字一致，库中分别仍是缺省、`true`、`false`。另验证无 UDP 参数的 VLESS URI 导入后预览输出 `udp: true`。六份原始 YAML 均通过固定 Mihomo v1.19.31 的 `-t`。本地回环流量试验验证两种协议的默认组和开启组均收到 UDP 回显，关闭组被内核跳过。
- 自动化覆盖新计划下载重渲染、历史计划快照及动态 Xray 注入；固定版内核门禁和全量测试通过。详细步骤见 [Build35.md](../Build/Build35.md)，设计口径见 [Design4 §7.3](../Design/Design4.md#73-历史版本与-xray-来源)。

### R35-05 节点管理的高级 JSON 切换样式与订阅装配不一致

- **现象：** 节点管理的对象字段编辑器使用「结构化编辑／高级 JSON」双按钮切换；订阅装配的 Clash 头部参数分区使用「高级 JSON」开关。用户提供的截图以节点的「多路复用」区域和装配的「端口配置」区域为例。
- **预期：** 节点管理中的高级 JSON 切换样式与订阅装配一致，使用开关类按钮。
- **检查范围：** 对照用户截图与前端组件：节点管理的对象字段由 `ProtocolFieldEditor.vue` 渲染双按钮，订阅装配的 `HeaderStep.vue` 对相关分区使用开关；对象字段编辑器也支持嵌套，后续处理时需检查节点管理内适用的对象字段及切换状态。
- **原始状态（2026-11-16 记录时，原文）：** 待后续处理；本次仅检查并记录样式差异与用户预期，未修改代码或验证切换行为。
- **当前状态（2026-11-17）：** 已完成研究、实施与本地验收。对象字段编辑模式已改为与订阅装配一致的「高级 JSON」开关；用户确认的“只替换外观、保留应用／放弃与草稿、覆盖所有对象字段、固定「高级 JSON」文案”方向已落地，第 12 节记录实施、测试与隔离浏览器证据。第 1～11 节保留实施前的研究记录与未实施结论。

#### 1. 结论摘要

- **两处不是同一组件的两套皮肤，而是两个独立实现。** 节点管理由 `ProtocolFieldEditor.vue` 渲染对象级编辑模式，订阅装配的 Clash 头部由 `HeaderStep.vue` 渲染分区级编辑模式；两者的 DOM 结构、ARIA 角色、尺寸、状态变量和提交时机都不同。
- **节点侧的双按钮是有意设计，不是漏改。** 2026-09-03 的 R27-05（提交 `3416fda`）明确把对象编辑模式从 `Switch` 改为按钮／分段控件，理由是避免与运行参数开关混淆，并写入 `Design4` §3.3。2026-09-02 的提交 `781b1f7` 则为装配 Clash 分区引入 `Switch`；两条实现从那时起分叉。更早的 `Design2-UI` §6.2 和 `Build15` 仍写“每个对象保留独立高级 JSON Switch”，与当前 `Design4` 和代码存在新旧文档冲突。
- **外观实测差异明确。** 节点为两个并排 Ant Design 按钮（桌面各约 32px 高，手机 44px），以 `role="group"`、`aria-pressed` 和主色／描边表达当前模式；装配为一个 28×16 的小号 `Switch` 加固定文案「高级 JSON」，以 `role="switch"`、`aria-checked` 和轨道颜色表达状态，桌面与手机的标签命中区均约 16px 高。
- **操作语义也不同。** 节点是“草稿 → 校验 → 应用／放弃”，未应用草稿会阻止保存和检查，退出高级 JSON 会静默丢弃草稿；装配是“输入即写入”，没有草稿、应用或放弃，但无效 JSON 时开关无法关闭，也没有显式放弃入口。两者在“无效 JSON 时不能退回结构化／关闭开关”这一点上一致。
- **已发现三类既有行为差异，其中两类是潜在 UX 风险。** 节点退出高级 JSON 不确认就丢弃草稿；装配在无效 JSON 时没有放弃路径；装配中修改其他分区会触发 `watch` 刷新当前分区的 JSON 文本，从而静默覆盖未应用的无效草稿并清除错误。本轮只记录，不判断是否为缺陷，也不修复。
- **适用范围已从 schema 侧确认。** 19 个 manual 协议的 `form_schema` 递归统计共有 49 个对象字段（39 个 `fields`、9 个 `map`、1 个 `list`；39 个顶层、10 个嵌套）。浏览器实际打开验证了 6 个字段（含嵌套 `smux.brutal-opts`、`headers` map、WireGuard `peers` list），其余字段仅通过 schema／源码核对，未逐个打开。
- **用户已确认后续调整方向（2026-11-17）：** 只替换外观；保留「应用／放弃」、草稿和父子阻断；覆盖所有对象字段（含嵌套、map、list）；开关旁固定显示「高级 JSON」。该方向尚未实施，见第 8 节。
- 本轮**未修改任何项目代码、配置、依赖或锁文件，未实施 UI 调整**；唯一持久化变更为本 `Issue21.md`。

#### 2. 代码版本、运行环境与测试前提

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `869c0c1`（`git describe` 为 `v4.1.4-11-g869c0c1`）；工作区研究开始时干净 |
| 代码与前端产物关系 | `frontend/dist` 时间戳为 2026-11-17 11:13，与提交 `4637cff` 的时间一致；`4637cff..HEAD` 在 `frontend/`、`backend/` 下无差异，因此按同源产物使用；本轮未重新构建前端 |
| 前端依赖 | Vue 3.5.44、Vite 7.3.7、Tailwind CSS 4.3.3、`@tailwindcss/vite` 4.3.3、`ant-design-vue` 4.2.6 |
| 后端／运行时 | Go 1.26.4（darwin/arm64）；Node.js 26.7.0；macOS 27.2 |
| 浏览器／驱动 | Google Chrome 157.0.8103.79；Playwright-core 1.57.0（临时目录，未加入项目依赖） |
| 隔离实例 | `/tmp/r35-05/server`；`APP_MODE=prod`、临时 `DATA_DIR=/tmp/r35-05/data`、`PORT=18095`，仅监听 `127.0.0.1` |
| 视口／主题 | 1440×1000、375×785；浅色与深色 |
| 证据目录 | `/tmp/r35-05/evidence/`（截图与 JSON 文件，未加入仓库） |

**构建产物。** 为避免改动工作区，后端源码和当前 `frontend/dist` 都复制到 `/tmp/r35-05` 后再构建：

| 产物 | 说明 | 关键哈希 |
|------|------|----------|
| 前端 `index.html` | 工作区 `frontend/dist` 原样使用，未重新构建 | `3085dda5789ce99491824ed8dcba71a818b812c9cc5664a8459e065b48315a3e` |
| 主 JS `index-BAQWrEVN.js` | 同一产物 | `f84620a7da4c039bbbd0930d8bffee8df56d3a2bb893cfffe406b894a33ec962` |
| 主 CSS `index-DLg-fzOL.css` | 同一产物 | `8dad59dcc6b012723843bd18210ecce25d471d50323776a909a19ca6b77b73e2` |
| Go 服务 `/tmp/r35-05/server` | 当前 HEAD 后端源码 + 上述前端产物嵌入，`CGO_ENABLED=0 go build ./cmd/server` | `73cca1eeb4b568865d3e25ac2987938d61e9b2aef7a12b3c01ef06450fd75ec9`；约 56 MB |

**测试数据前提。** 测试库来自既有隔离合成库 `/tmp/r3502-fix.GxIgJu/data` 的副本，不是当前仓库 `backend/data`，也没有连接或写入本机 8080 服务：

- 合成管理员 `admin@test.com`，密码为 [TestPasswordList.md](../../Reference/TestPasswordList.md) 规定的统一测试密码；
- 1 个手动 VLESS 节点 `HK`（`1.1.1.1:443`，TCP + REALITY，`smux` 初始关闭）；
- 本轮通过管理员 API 新建的合成节点 `R35-05-HTTP`（HTTP，`headers` map）和 `R35-05-WG`（WireGuard，两个 Peer 的 `peers` list），主机名均为 `*.example.com`，密钥为合成 Base64 值；
- 所有节点、草稿和保存结果只写入 `/tmp/r35-05/data`；没有操作生产、预发布、外部服务、真实账号或真实数据。

**构建产物与源码一致性。** 本轮没有在项目工作区执行 `npm run build`、`go build`、测试或格式化命令；隔离服务是在 `/tmp` 副本中构建的。前端产物与当前 HEAD 的一致性依据是时间戳和空 diff，而不是重新构建，已在第 9 节列为证据边界。

#### 3. 复现步骤

1. 阅读 `ProtocolFieldEditor.vue`、`HeaderStep.vue`、`PlatformHeadersEditor.vue`、`NodesView.vue` 及相关测试和设计文档，确认两处控件分别由哪个组件、哪个状态变量控制。
2. 在 `/tmp/r35-05/backend` 复制当前后端源码和当前 `frontend/dist`，构建隔离服务；使用临时 `DATA_DIR` 和 `127.0.0.1:18095` 启动，确保不接触 8080、Docker、外部服务或真实数据。
3. 使用合成管理员登录，进入 `/admin/nodes`，打开 `HK` 节点的编辑弹窗，展开「更多功能 / 高级参数」等折叠区，定位「多路复用」对象；记录按钮、ARIA、尺寸、主题与手机布局。
4. 依次执行：进入高级 JSON → 修改为有效 JSON → 保存被拦截 → 切回结构化 → 输入无效 JSON → 保存被拦截 → 放弃 → 应用有效 JSON → 子对象 `smux.brutal-opts` 输入草稿 → 父对象尝试进入高级 JSON → 取消并关闭弹窗 → 重新打开 → 应用并保存 → 重新打开确认持久化；同时检查目标检查的阻断与定位。
5. 新建或读取 `R35-05-HTTP`、`R35-05-WG`，重复查看 `headers` map、`peers` list 和 `ip-stack` 对象的同一个双按钮控件。
6. 进入 `/admin/assembly?tab=clash-yaml`，在空素材提示中点击「继续装配」，展开 Clash 头部「端口配置」「Geo 数据」「DNS 配置」「更多参数」四个分区；记录分区级「高级 JSON」开关、尺寸、ARIA、主题与手机布局。
7. 对端口分区执行：开启开关 → 修改有效 JSON → 关闭开关并查看结构化值 → 重新开启并输入无效 JSON → 尝试关闭开关和点击标签 → 修正后关闭 → 键盘 `Enter`／`Space`；另测未知键、跨分区修改后的草稿刷新。
8. 在 375×785 视口和深色主题下重复关键步骤；用 Playwright 读取 `getComputedStyle` 和 `getBoundingClientRect`，用 `getByRole('switch', { name: '高级 JSON' })` 检查可访问名称。所有临时脚本、截图和日志只写入 `/tmp`。

**实际检查边界。** 桌面节点侧实际打开的是 VLESS `reality-opts`、`smux`、`smux.brutal-opts`，HTTP `headers`，WireGuard `peers` 和 `ip-stack`；装配侧实际打开的是 Clash 目标的分区开关。其余 43 个对象字段、其他 16 个协议、装配的 SR／generic 目标和真实读屏软件没有逐一实际验证，见第 6、9 节。

#### 4. 两处控件的实际外观与状态表达

##### 4.1 节点对象字段编辑器

「多路复用」位于节点编辑弹窗的「更多功能 / 高级参数」折叠区内，默认折叠；展开后该对象区域顶部左侧是标题「多路复用」，右侧是编辑模式控件。简化后的实际 DOM 结构（省略构建哈希与 `data-v-*` 属性）：

```html
<div class="protocol-editor-mode" role="group" aria-label="多路复用编辑模式">
  <button class="ant-btn ant-btn-primary" aria-pressed="true">结构化编辑</button>
  <button class="ant-btn ant-btn-default" aria-pressed="false" data-field-json-toggle>高级 JSON</button>
</div>
```

- **桌面（1440×1000，浅色）。** 两个按钮高 32px，宽分别约 102px 和 100.84px；字体 14px／22px。当前模式使用 Ant Design 主色按钮（背景 `rgb(37, 99, 235)`、白字），另一侧为白底、`rgb(203, 213, 225)` 描边；两按钮通过 `border-radius` 拼成左右分段，左侧 `6px 0 0 6px`、右侧 `0 6px 6px 0`。两个按钮都在 Tab 顺序中，各带 `aria-pressed`；外层 group 的 `aria-label` 包含对象名。
- **高级态。** 点击「高级 JSON」后，结构化控件被替换为 6 行等宽字体 `TextArea`，下方固定出现「应用」「放弃」两个小按钮；输入后另有「JSON 草稿未应用」提示。当前模式由右侧按钮的主色和 `aria-pressed="true"` 表达，文案仍同时显示「结构化编辑」「高级 JSON」两个选项。
- **手机（375×785，浅色）。** 对象工具栏由横排变为纵向（`flex-direction: column`），工具栏高约 76px；两个按钮仍并排，宽度约 102px＋100.84px，高度由桌面 32px 提升到 44px，满足项目对移动端命中区的要求。
- **深色。** 当前模式按钮背景为 `rgb(85, 143, 216)`、白字；非当前模式为 `rgb(17, 24, 39)` 背景、`rgb(248, 250, 252)` 文字、`rgb(51, 65, 85)` 描边；区域和标题随主题变化。
- **嵌套。** `smux.brutal-opts`（「Brutal 参数」）拥有同一套独立双按钮；`headers` map 和 `peers` list 的对象级工具栏也使用同一套按钮。按钮的 `aria-label` 会随对象标题变化，例如「Brutal 参数编辑模式」。
- **适用条件。** 对象字段只有在当前协议、传输、安全、插件等条件命中时才渲染；`smux` 的 `enabled` 为 false 时，其子字段可能被条件隐藏，但对象级工具栏和双按钮仍存在，并提示「运行开关位于“独立开关”区域」。

##### 4.2 订阅装配 Clash 头部参数

Clash 头部由 `HeaderStep.vue` 按「端口配置 / Geo 数据 / DNS 配置 / 更多参数」四个可折叠分区渲染。每个分区的工具栏结构相同（简化，省略构建哈希与 `data-v-*` 属性）：

```html
<div class="section-editor-toolbar">
  <span>HTTP、SOCKS 与透明代理监听端口</span>
  <label class="flex items-center gap-2 text-xs text-text-secondary whitespace-nowrap">
    <button role="switch" aria-checked="false" class="ant-switch ant-switch-small">…</button>
    <span>高级 JSON</span>
  </label>
</div>
```

- **桌面（1440×1000，浅色）。** `Switch` 本体为 28×16px，外层 `<label>` 约 95.66×16px，文案 12px／16px、颜色 `rgb(71, 85, 105)`。关闭时轨道为 `rgba(0, 0, 0, 0.25)`，开启时轨道为 `rgb(29, 78, 216)`；开关角色为 `role="switch"`，状态为 `aria-checked`，没有 `aria-pressed`。点击文案、`Enter` 和 `Space` 都能切换；聚焦时出现约 2px 的浅蓝描边。
- **状态表达。** 文案固定为「高级 JSON」，不随状态变化；开启表示当前分区显示 JSON 文本域，关闭表示显示结构化字段。四个分区各自维护独立开关，互不影响（这一点来自源码 `advancedSections` 的四个键，未逐一做四分区交叉切换测试）。
- **可访问名称。** 在 Clash 头部，`getByRole('switch', { name: '高级 JSON' })` 命中 4 个元素；四个开关的可访问名称完全相同，分区上下文只由相邻的说明文字提供。这一点是节点侧改用开关时需要特别避免的。
- **手机（375×785，浅色）。** 分区工具栏同样变为纵向；开关本体仍为 28×16px，外层标签命中区约 95.66×16px，明显小于节点按钮 44px 的高度。
- **深色。** 关闭时轨道为 `rgba(255, 255, 255, 0.25)`，开启时为 `rgb(147, 197, 253)`；文案颜色 `rgb(203, 213, 225)`。开关本体和轨道尺寸不变。
- **另一种装配开关。** 非 Clash 目标（如 SR 订阅组）的「头部参数」右上角还有一个整体级「高级 JSON」开关；它通过 `v-model:checked` 直接绑定 `form.fixed_params_text`，没有分区校验、错误提示或草稿。本轮只确认该开关存在并计数为 1，未把它与 Clash 分区开关混为一谈。

##### 4.3 外观对照

| 维度 | 节点对象字段 | 装配 Clash 分区 |
|------|--------------|-----------------|
| 控件 | 两个并排 Ant Design `Button` | 一个 Ant Design `Switch` 加固定标签 |
| 角色／状态 | `role="group"`、两个 `aria-pressed` | `role="switch"`、一个 `aria-checked` |
| 可见文案 | 「结构化编辑」「高级 JSON」同时显示 | 固定「高级 JSON」 |
| 桌面尺寸 | 约 102×32px＋100.84×32px | 28×16px，标签约 95.66×16px |
| 手机命中区 | 两个按钮各 44px 高 | 标签整体约 16px 高 |
| 键盘停靠点 | 两个 Tab 停靠点 | 一个 Tab 停靠点；`Enter`／`Space` 切换 |
| 主题表达 | 主色实心按钮／描边按钮 | 轨道填充色 |
| 上下文命名 | group 的 `aria-label` 含对象名 | 四个开关同名「高级 JSON」 |
| 位置 | 对象区域标题右侧 | 分区工具栏右侧 |

#### 5. 切换行为对照

##### 5.1 节点对象字段编辑器实测

| 操作 | 实际结果 |
|------|----------|
| 进入高级 JSON | 显示 `TextArea`（初始为当前对象值的 2 空格缩进 JSON）；显示「应用」「放弃」；结构化控件隐藏；未修改父模型 |
| 输入有效 JSON | 显示「JSON 草稿未应用」；父模型不变；`aria-pressed` 仍表示高级态 |
| 有草稿时保存 | Toast「存在未应用的 JSON 草稿，请先应用或放弃后再保存」；没有发出 `POST`／`PUT` 请求 |
| 切回结构化（有草稿） | 不弹确认，直接丢弃草稿并恢复当前有效值；`TextArea` 消失；父模型不变 |
| 输入无效 JSON | 显示「请输入 JSON 对象」（根节点语法／形状错误）或带路径的固定对象错误；「结构化编辑」和「应用」变为 disabled；「放弃」仍可用 |
| 无效 JSON 时保存 | Toast「请先修正协议对象参数中的格式错误」；没有发出请求 |
| 点击「放弃」 | 文本框恢复当前有效 JSON，草稿提示消失，仍停留在高级态 |
| 点击「应用」 | 父模型更新（本例 `max-connections` 由空变为 4，`protocol` 为 `yamux`），草稿提示消失；未自动保存到后端 |
| 嵌套子对象草稿 | 在 `smux.brutal-opts` 输入草稿后，点击父对象「高级 JSON」：Toast「存在未应用的后代 JSON 草稿，请先应用或放弃后再切换高级 JSON」；父对象保持结构化，焦点移动到子对象文本框（`data-field-path="smux.brutal-opts"`） |
| 取消编辑 | 弹窗关闭；重新打开后回到数据库中的值，未应用草稿不持久化 |
| 应用后保存 | `PUT /api/admin/nodes/1` 的 `protocol_json` 含 `smux: {enabled: true, protocol: "yamux", max-connections: 4}`；Toast「节点已更新」；重新打开可见持久化值 |
| 目标检查 | 有未应用草稿时，检查面板显示「存在未应用的 JSON 草稿，请先应用或放弃后再检查」，「检查当前节点」disabled，「定位草稿」可聚焦到对应文本框 |
| 未知键 | `{"enabled":true,"foo":1}` 报「字段 smux.foo 未在协议注册表中声明」；「应用」和「结构化编辑」均 disabled |
| 嵌套形状错误 | `{"enabled":true,"brutal-opts":"x"}` 报「请输入 JSON 对象：smux.brutal-opts」 |
| 结构化字段回写 | 先在结构化态将 `max-connections` 改为 6，再进入高级 JSON，文本显示 6；说明两种模式共用同一有效值 |
| map／list 对象 | HTTP `headers` 的高级 JSON 为 `{"User-Agent":"r35-05","X-Test":"1"}`；WireGuard `peers` 的高级 JSON 为包含 `_credential_id` 的两个 Peer 数组；工具栏与普通对象相同 |

##### 5.2 订阅装配 Clash 分区实测

| 操作 | 实际结果 |
|------|----------|
| 开启「高级 JSON」 | 显示分区 JSON 文本域（端口分区为 4 个端口键的 2 空格缩进 JSON）；结构化字段隐藏；无「应用」「放弃」按钮 |
| 输入有效 JSON | 输入即写入 `form.fixed_params_text`；没有草稿提示；关闭开关后结构化字段立即显示新值（`port` 由 7890 变为 7891） |
| 关闭开关 | 结构化和 JSON 共享同一份值；关闭不做额外确认；没有未应用草稿 |
| 输入无效 JSON | 显示原始 JS 解析错误，例如「Expected property name or '}' in JSON at position 1 (line 1 column 2)」；开关仍为 `aria-checked="true"` |
| 无效时尝试关闭 | 点击开关无效，仍保持开启；错误文本保留；没有「放弃」入口 |
| 无效时点击标签 | 同样无法关闭；行为与点击开关一致 |
| 未知键 | 输入 `{"port":7890,"foo":1}` 报「参数“foo”不属于此分区」；开关仍保持开启 |
| 跨分区修改 | 端口分区保持无效 JSON `{` 时，修改 DNS 分区的「监听地址」输入；`watch(form.fixed_params_text)` 会把端口 JSON 刷新为当前有效默认值并清除错误，开关仍为开启；未应用的无效文本被静默覆盖 |
| 键盘 | 开关聚焦后 `Enter` 和 `Space` 均可切换；Tab 只经过一个开关停靠点 |
| 整体级开关（SR） | 非 Clash 目标的「高级 JSON」直接绑定原始文本，不做 JSON 解析、错误提示或分区块校验；本轮仅确认存在，未展开行为测试 |

##### 5.3 差异归纳与潜在行为问题

- **共同点。** 两处都按对象／分区独立维护模式；都禁止在无效 JSON 状态下退回结构化／关闭开关；都只影响前端编辑态，不改变后端协议字段、保存负载结构或输出语义。
- **核心差异是提交时机。** 节点采用显式草稿和「应用／放弃」，装配采用输入即写入。节点因此可以保留未应用草稿、阻止保存／检查、并做父子草稿阻断；装配没有这些状态。
- **退出语义不同。** 节点切回结构化会静默丢弃未应用草稿（源码 `setAdvanced(false)` 直接重置 `jsonDirty` 和文本）；装配关闭开关不会“丢弃”内容，因为有效输入已经写入，但在无效状态下根本没有放弃入口。
- **错误呈现不同。** 节点把解析和递归形状错误统一成中文提示（根节点为「请输入 JSON 对象」，字段路径为「字段 …」）；装配直接显示浏览器 `JSON.parse` 的原始英文错误，只有分区键校验是中文。
- **外部刷新行为不同。** 装配的 `watch` 会在其他分区或父级修改时刷新当前分区的 JSON 文本；节点的草稿在无关父级变化时尽量保留，并有 `jsonResetVersions` 处理重叠范围。装配的跨分区覆盖在本轮实测中可复现，是否为预期行为尚未确认。
- **是否是缺陷需分开判断。** “节点退出静默丢草稿”“装配无效 JSON 无放弃入口”“装配跨分区覆盖无效草稿”都是观察到的既有行为，本轮不把它们认定为已确认缺陷，也不在 R35-05 范围内修复；如果后续把节点改为开关，这三项会直接影响交互设计和回归范围。

#### 6. 适用对象字段与嵌套场景

从 `GET /api/admin/nodes/protocols` 的 19 个协议 `form_schema` 递归统计：

| 统计项 | 数量 | 说明 |
|--------|------|------|
| 对象字段总数 | 49 | 含同一字段在多个协议中的重复声明 |
| `object_kind=fields` | 39 | 固定结构对象，最常见 |
| `object_kind=map` | 9 | 开放键值映射，如请求头、插件参数、Peer Info |
| `object_kind=list` | 1 | WireGuard `peers` |
| 顶层对象 | 39 | 对象字段位于协议 schema 第一层 |
| 嵌套对象 | 10 | 第二层对象，集中在插件的 `headers`／`ech-opts` 和 `smux.brutal-opts` |

代表性嵌套路径：`smux.brutal-opts`（VLESS／VMess／Shadowsocks）、`ws-opts.headers`（VMess／VLESS／Trojan）、`http-opts.headers`（VMess／VLESS）、`v2ray-plugin-opts.headers`、`v2ray-plugin-opts.ech-opts`（Shadowsocks）。本轮在 schema 中观察到的最深对象层级为 2 层；没有发现第 3 层对象。

**实际打开验证的字段（浏览器）：**

| 字段 | 协议／前提 | 形态 | 验证内容 |
|------|-----------|------|----------|
| `reality-opts` | VLESS，TCP + REALITY | fields，顶层 | 工具栏、双按钮、`aria-pressed`、桌面尺寸、浅／深色 |
| `smux` | VLESS，用户截图对应区域 | fields，顶层 | 默认折叠、双按钮、进入高级 JSON、草稿、保存／检查拦截、应用、持久化 |
| `smux.brutal-opts` | VLESS，`smux` 已启用 | fields，第二层 | 嵌套独立工具栏、子草稿阻止父对象进入 JSON、焦点定位 |
| `headers` | HTTP，无认证 | map，顶层 | 结构化键值行、高级 JSON 对象、开放键保留 |
| `peers` | WireGuard，`peer_mode=peers` | list，顶层 | 两个 Peer 条目、上移／下移／删除、对象数组高级 JSON、`_credential_id` 保留 |
| `ip-stack` | WireGuard | fields，顶层 | 对象级双按钮与结构化字段 |

**仅通过 schema／源码核对、未逐个打开的字段：** 其余 43 个对象声明，包括 `plugin-opts`、`obfs-opts`、`v2ray-plugin-opts`、`shadow-tls-opts`、`restls-opts`、`ss-opts`、`ws-opts`、`grpc-opts`、`h2-opts`、`http-opts`、`xhttp-opts`、`ech-opts`、`realm-opts`、`jls-opts`、`peer-info`、各协议的 `ip-stack` 等。这些字段与已打开字段使用同一个 `ProtocolFieldEditor` 递归组件和同一套双按钮，但本轮没有逐一验证其条件可见性、字段路径或极端嵌套。不能把上述 6 个字段的结论直接等同于全部 49 个字段均已人工验证。

#### 7. 样式差异形成的实现原因

| 时间 | 证据 | 结论 |
|------|------|------|
| 2026-08-30 | `Build15` Step 2、`Design2-UI` §6.2（第 410 行） | 节点对象编辑器最初设计为“每个对象保留独立「高级 JSON」Switch” |
| 2026-09-02 | 提交 `781b1f7`（`HeaderStep.vue`） | 装配 Clash 头部引入分区级 `advancedSections` Switch；节点当时仍使用 Switch |
| 2026-09-03 | 提交 `3416fda`（R27-05）、`Issue13.md` 的 R27-05 修复方向与实施记录 | 节点对象编辑器以“编辑模式按钮替代 Switch”；理由是“用按钮或分段控件表达结构化编辑／高级 JSON，避免与运行开关混用” |
| 当前 | `Design4` §3.3（第 142 行） | 明确要求“结构化／JSON 属于编辑模式操作，使用明确的按钮或分段控件；不能混成节点运行参数” |
| 当前 | `ProtocolFieldEditor.vue:657-660`、`tests/protocol-field-editor.spec.ts:165-169` | 代码和测试都把双按钮作为当前契约；测试显式断言 `role="group"` 且不存在 `.ant-switch` |

因此，**样式差异不是 Tailwind／主题／浏览器问题，也不是遗漏同步**：它是 2026-09-03 在节点侧做出的、用户当时确认过的控件类型决策；装配侧保留了 2026-09-02 引入的 Switch。更早的 `Design2-UI` 仍写 Switch，与当前 `Design4`、代码和测试冲突，属于新旧文档冲突，而不是同一份设计内的实现漂移。R35-05 是用户对 R27-05 该决策的再次调整，用户已在 2026-11-17 确认以 R35-05 为准，后续实施时需要同步修订 `Design4` §3.3 和相关测试口径。

组件职责也解释了为什么不能简单“共用同一个开关组件”：

- `ProtocolFieldEditor.vue`：本地维护 `advanced`、`jsonText`、`jsonError`、`jsonDirty`；只在「应用」时 `emit('update:modelValue')`；通过 `json-dirty-change` 把草稿状态交给 `NodesView` 的 `unappliedJsonPaths`，由页面统一阻断保存／检查、处理父子草稿和定位。`NodesView.vue:715-731` 是保存拦截，`NodesView.vue:298-315` 是草稿与父子阻断处理。
- `HeaderStep.vue`：本地维护 `advancedSections`、`sectionJSON`、`sectionErrors`；`updateSectionJSON()` 每次输入都直接改写 `form.fixed_params_text`；没有草稿集合、没有页面级阻断、没有父子关系。`HeaderStep.vue:281-308` 是即时写入与校验。
- `PlatformHeadersEditor.vue`：第三种实现，使用 Switch 加动态文案（「结构化编辑」／「高级 JSON」），JSON 文本在切换回结构化或取值时才校验／应用；它与节点的双按钮、装配的分区开关都不同。该组件不在 R35-05 范围内，本轮未改动，也不建议把它作为本次一致性的唯一参照。

#### 8. 改为开关的实现影响

以下内容是基于现有源码和本轮实测的影响分析；除明确标注的“用户已确认”外，均为尚未实施的候选建议，不代表已实现或已验证。

##### 8.1 用户已确认的调整方向（2026-11-17，未实施）

- **语义：** 只替换外观；保留「应用」「放弃」、草稿提示、父子草稿阻断、保存／检查拦截和无效 JSON 禁用逻辑。不采用装配的即时写入语义。
- **范围：** 所有对象字段，包括嵌套对象、map 和 list；不只在「多路复用」区域替换。
- **文案：** 开关旁固定显示「高级 JSON」，不随状态切换为「结构化编辑」。
- **设计冲突处理：** 以 R35-05 的最新期望为准，允许重新使用 Switch；实施时同步修订 `Design4` §3.3，并更新把按钮当作契约的测试。

该方向目前只是已确认的候选实施口径，**没有写代码、没有修改设计文档、没有更新测试**。

##### 8.2 代码与状态影响

- **改动点集中在一个组件。** `ProtocolFieldEditor.vue:657-660` 的两个 `Button` 可替换为一个 `Switch` 加 `<span>高级 JSON</span>`；`setAdvanced()`、`updateJSON()`、`applyJSON()`、`discardJSON()` 和 `jsonDirty`／`jsonError` 逻辑可以保留，不需要改父组件或后端。
- **状态命名。** 现有 `advanced` 是布尔值，可直接作为 `:checked`；不需要改名为 `jsonMode`。`jsonText`、`jsonError`、`jsonDirty`、`jsonInvalidPath` 仍分别承担文本、错误、草稿和无效路径。`data-field-json-toggle` 建议保留在开关上，因为 `nodeValidation.ts` 和保存定位依赖该标记。
- **切换处理。** `setAdvanced(true)` 仍应阻止有后代草稿时进入；`setAdvanced(false)` 仍应在 `jsonError` 时拒绝退出。受控 Switch 在拒绝时会自动回到原状态，但不会自动解释原因；现有错误文本可保留，是否额外提示见 8.3。
- **样式。** 需要移除或调整 `.protocol-editor-mode .ant-btn` 的三条圆角规则（`:850-852`），并新增开关与文案的对齐、命中区和深色适配。现有工具栏 `flex items-center justify-between` 布局可继续使用。
- **不影响后端。** 保存负载、`protocol_json`、`current_state`、`reset_scopes`、`credential_ops`、目标检查和输出投影都不需要改；不涉及数据库、API、依赖或锁文件。

##### 8.3 无障碍、键盘与状态表达

- `role="group"` ＋ 两个 `aria-pressed` 会变成一个 `role="switch"` ＋ `aria-checked`。视觉上“当前模式”从主色按钮变为开关轨道，状态表达更接近装配。
- 节点对象可能同时出现多个开关（例如 `smux`、`smux.brutal-opts`、`ws-opts.headers` 等）。如果可见文案都只是「高级 JSON」，屏幕阅读器会得到多个同名开关；装配当前就有 4 个同名「高级 JSON」。建议在开关上加包含对象名的 `aria-label`，例如「多路复用 高级 JSON」；可见文案仍固定为「高级 JSON」。
- 键盘从两个 Tab 停靠点变为一个；`Space` 和 `Enter` 在 Ant Design Switch 上均可切换（装配实测通过）。聚焦描边、按 `Esc` 关闭弹窗、保存按钮的回焦等既有行为不应改变。
- 无效 JSON 时，按钮方案会把「结构化编辑」明确禁用；开关方案只能让开关“弹回”开启态。建议在错误文本之外保留简短的不可退出原因，或至少保证错误文本在视口内可见；这是候选交互细节，尚未确认。
- 节点现有移动端规则只给 `.ant-btn` 加 44px 最小高度（`:853-855`）；改成 28×16 的 `Switch` 后，若不做外层包裹，命中区会降到约 16px，低于 `Design4` 要求的 44px。建议用 44px 高的 `<label>` 包裹开关和文案，或复用对象工具栏的整行高度。
- 主题方面，节点按钮当前使用主色／描边，装配开关使用轨道填充；改用开关后应沿用 Ant Design 主题 Token，避免在深色主题出现低对比轨道。浅色实测开关开启为 `rgb(29, 78, 216)`、关闭为 `rgba(0, 0, 0, 0.25)`；深色开启为 `rgb(147, 197, 253)`、关闭为 `rgba(255, 255, 255, 0.25)`，可作为视觉回归基线。

##### 8.4 草稿、校验与嵌套行为

- **草稿保护不变。** 用户选择保留「应用／放弃」，因此 `jsonDirty`、`unappliedJsonPaths`、保存／检查阻断、父子草稿阻断都应保持现状。
- **退出时的静默丢弃仍是现状。** 当前 `setAdvanced(false)` 在无错误时会直接丢弃草稿且不确认。只换控件不会改变这一点，但开关比按钮更容易误触；如果后续认为风险不可接受，可新增“有草稿时关闭先确认”的独立决策。本轮用户选择的是“只换外观”，所以该风险按现状记录，不列为本次实施内容。
- **无效 JSON 仍不能退出。** 开关应受 `jsonError` 控制（`setAdvanced(false)` 的既有 guard）；不能用 `v-model` 直接绑定绕过。否则会出现“开关已关闭但错误草稿仍存在”或“错误草稿被静默丢弃”的不一致。
- **嵌套对象逐个独立。** 每个对象有自己的 `advanced`，父对象进入 JSON 会卸载子编辑器；因此父子草稿阻断和“首个后代草稿”定位必须保留。改成单一全局开关会破坏该语义，不应采用。
- **保存／检查负载不变。** `applyJSON()` 仍只在用户点击「应用」后 emit；`save()` 仍先检查 `invalidProtocolPaths`、`unappliedJsonPaths`、`unappliedControlPaths`。改为开关不应改变这些顺序和文案。

##### 8.5 测试与文档影响

- **测试改造成本较高。** `tests/protocol-field-editor.spec.ts`、`tests/nodes-view.spec.ts`、`tests/header-step.spec.ts` 中检索到 31 处通过 `button.text() === '高级 JSON'` 触发控件；`protocol-field-editor.spec.ts:165-169` 还显式断言“使用按钮且不存在 Switch”。改为 Switch 后这些用例需要改成按 `role="switch"` 或 `data-field-json-toggle` 触发，且该断言必须重写。
- `nodes-view.spec.ts:3005` 通过 `document.activeElement?.textContent` 包含「高级 JSON」来判断定位；Switch 的按钮本体文本为空，标签文本是兄弟节点，改为开关后该断言可能失效，需要改成检查 `aria-label` 或外层标签。
- `node-validation.ts` 的定位 fixture 使用 `[data-field-json-toggle]`；建议保留该属性，避免无谓地扩大 `node-validation.spec.ts` 的改动。
- `Design4` §3.3 需要从“按钮或分段控件”改为允许 Switch，并说明“编辑模式开关”与“运行参数开关”的区分方式；`Design2-UI` §6.2 是归档文档，仍写 Switch，不需要回写成按钮，但应在实施记录中说明其已被 R27-05 后续设计取代、现又被 R35-05 调整。`Issue13` R27-05 和 `Build21` 是历史记录，不应改写，只在新记录中引用。
- **不需要改后端测试。** 本轮没有发现协议 schema、保存、检查或输出层参与控件类型；改为开关属于纯前端编辑模式调整。

##### 8.6 不受影响或影响很小的部分

- 订阅装配、`HeaderStep.vue`、`PlatformHeadersEditor.vue` 不需要改；R35-05 只改节点管理。
- 后端、数据库、API、依赖、锁文件、部署配置不需要改。
- 节点的运行参数开关（「独立开关」区域）仍由 `NodesView` 的 `switchFields` 和集中开关逻辑管理；对象编辑模式开关不进入 `protocol_json`，不会与运行参数混写。
- 保存后的协议 JSON、目标检查、Clash／SR／generic 输出和版本生成不受控件类型影响；已有节点不需要迁移。

#### 9. 尚未确认内容与测试限制

- **没有实施验证。** 本轮没有改代码，所以“换成 Switch 后的实际外观、命中区、焦点顺序、读屏体验和测试通过情况”都未验证；第 8 节是基于现有代码和实测的推断与建议。
- **没有真实读屏软件。** 只通过 DOM、`role`、`aria-checked`／`aria-pressed` 和 Playwright 的 `getByRole` 检查；没有使用 VoiceOver、NVDA 等实际朗读验证。
- **没有真实手机。** 手机结论来自 375×785 的 headless Chrome 和触摸模拟，不是真机；虚拟键盘、地址栏收放和实际手指命中未测。
- **没有覆盖全部字段。** 只实际打开 6 个对象字段、3 个协议；其余 43 个对象声明和其他 16 个协议是 schema／源码核对，未验证条件可见性、字段路径和极端数据。
- **没有覆盖装配的全部目标。** 只实测 Clash 分区的四个开关中的一个（端口）和 SR 整体开关的存在；Geo／DNS／more 分区只确认了相同结构和独立状态，未逐一执行完整 JSON 编辑。
- **装配跨分区刷新是否是缺陷未确认。** 已复现“修改 DNS 后端口无效 JSON 被刷新覆盖”，但没有找到对应设计条款或用户反馈；本轮只作为行为差异记录。
- **节点静默丢弃草稿是否是缺陷未确认。** 当前代码和 Build19 的“取消则放弃该草稿”合同可以解释该行为；但“切换模式”是否应等同于“取消”没有单独产品结论。
- **前端产物未重建。** 使用工作区 `frontend/dist`（时间戳与 `4637cff` 一致，且 `4637cff..HEAD` 无前后端差异）；没有在隔离副本中重新执行 `npm run build`。如果产物与源码存在未发现的偏差，外观测量可能受影响，但控件源码与 DOM 结构已相互印证。
- **未在项目工作区运行测试或构建，也未运行 Docker、外部服务或真实客户端。** 隔离服务只在 `/tmp` 副本中构建；没有操作 8080 的数据，没有访问生产／预发布／外部站点，没有使用真实账号或真实数据。
- **未确认 R35-05 的实施排期。** 用户已确认方向，但本轮不实施；后续是否单独开构建步骤、是否同时处理平台响应头编辑器的文案不一致，均待定。

#### 10. 后续验证建议与候选回归范围

如果后续按已确认方向实施，建议至少覆盖：

- **组件级：** 双按钮消失、Switch 出现；`aria-checked`、`role="switch"`、带对象名的无障碍标签；`data-field-json-toggle` 仍可用于保存定位；「应用」「放弃」「JSON 草稿未应用」和错误文本仍存在。
- **行为级：** 进入／退出高级 JSON；有效 JSON 应用；无效 JSON 阻止退出和应用；保存／检查阻断；父子草稿阻断与定位；取消／重开不持久化；应用后保存并重开。
- **对象形态：** 固定对象（`smux`、`reality-opts`）、嵌套对象（`smux.brutal-opts`）、map（HTTP `headers`）、list（WireGuard `peers`）；至少覆盖一个禁用子字段条件和一个开放未知键对象。
- **响应式与主题：** 1440×1000、375×785、浅色与深色；按钮改为开关后移动端命中区不小于 44px，且无横向溢出。
- **键盘与读屏：** Tab 只经过一个开关停靠点；`Space`／`Enter` 切换；无效 JSON 时开关弹回且原因可见；实际读屏至少抽查一个嵌套对象，确认不会只朗读四个同名「高级 JSON」。
- **自动化：** 更新 `protocol-field-editor.spec.ts` 中“按钮且无 Switch”的断言及检索到的 31 处按钮点击；检查 `nodes-view.spec.ts:3005` 的定位断言；保留 `node-validation.spec.ts` 的 `data-field-json-toggle` fixture 兼容。
- **文档：** 实施时同步修订 `Design4` §3.3，新增构建／变更记录，并在 `Issue21` 将 R35-05 从未实施更新为实施后状态；不要把本轮研究记录改写成已修复。

候选替代方向（本轮未采纳，仅保留）：

- **保持双按钮。** 维持 R27-05 的原始决策，把 R35-05 记录为“不采纳”；与用户本次明确期望不一致，不作为推荐。
- **改用分段控件但视觉接近开关。** 可保留两态文案和较大命中区，但不是用户要求的“与订阅装配一致”的 Switch；需要重新定义视觉规格。
- **完全对齐装配的即时写入。** 会移除草稿和「应用／放弃」，违反 `Design4` §6.4 的草稿合同，并影响父子阻断、保存／检查拦截和定位；不建议在 R35-05 中采用。

#### 11. 记录边界

本轮只做测试、研究与文档记录。唯一持久化修改是 `Issue21.md`；没有修改任何项目代码、配置、依赖或锁文件，没有实施 UI 调整，没有操作生产、预发布、外部服务、真实账号或真实数据。本节记录的研究轮次中 R35-05 为**未实施**，不能表述为已修复；实施结果见第 12 节。

#### 12. 实施结果（2026-11-17）

- **状态：** 已按第 8 节方向实施并完成前端自动化、生产构建、后端回归及隔离浏览器验收；R35-05 在本次本地范围内标记完成。第 1～11 节保留实施前的研究与未实施结论；真实读屏、真机或全部 49 个对象字段的后续验证不在本轮结论内。
- **代码变更（唯一生产文件）：** `frontend/src/components/ProtocolFieldEditor.vue` 的对象工具栏由两个 `Button` 改为受控 `Switch` 加固定 `<span>高级 JSON</span>`；桌面开关 28×16px、文案 12px，手机外层标签至少 44px，现有 `.form-overlay-drawer` 规则使开关本体为 44×44px。开关保留 `data-field-json-toggle`，文本域保留 `data-field-json-editor`；无障碍名称为 `<对象 label> 高级 JSON`。没有修改后端、API、数据库、持久化格式、依赖或锁文件。
- **既有语义保护：** 仅替换控件与展示／无障碍适配。`advanced`、`jsonText`、`jsonError`、`jsonDirty`、`applyJSON()`、`discardJSON()`、`setAdvanced()` 的父子草稿 guard、`json-dirty-change`／`advanced-json-blocked` 事件、`NodesView` 的保存／检查阻断和定位均未改。无效 JSON 时 `jsonError` 使开关禁用，等价于原「结构化编辑」按钮的 disabled；修正后恢复。没有采用订阅装配的即时写入语义。
- **自动化验证：** 定向 4 个文件 182 项通过；前端全量 51 个文件 471 项通过；`npm run build` 通过；后端 `go build ./...`、`go vet ./...`、`go test ./... -count=1` 通过；`git diff --check` 通过。
- **隔离浏览器验证：** Chrome 157.0.8103.79 + Playwright-core 1.57.0，隔离生产构建绑定 `127.0.0.1:18098`，合成库副本。1440×1000 与 375×785、浅色／深色下验证新建 `R35-05-IMPL`、编辑 `HK`、HTTP `headers` map、WireGuard `peers` list，实际交互 REALITY／SMUX／Brutal 嵌套开关，并确认 WireGuard `ip-stack` 开关存在；固定文案、`role="switch"`、`aria-checked`、可区分名称、Enter／Space、有效／无效 JSON、应用／放弃、保存／检查阻断、定位、父子阻断与持久化均通过，页面与抽屉无横向溢出。编辑 `HK` 的 `smux.max-connections=9` 保存后重开仍为 9。
- **布局与主题实测：** 桌面开关 28×16px、标签 95.66×16px；手机标签 44×291px、开关 44×44px；浅色关闭／开启轨道为 `rgba(0,0,0,0.25)`／`rgb(37,99,235)`，深色为 `rgba(255,255,255,0.25)`／`rgb(147,197,253)`；两种主题下文案均为 12px。截图与日志位于 `/tmp/r35-05-impl.PfGj5a/`，未加入仓库。
- **文档：** `Design4` §3.3 已改为开关口径并记录 v1.33；新增 [Build36.md](../Build/Build36.md)；`AGENTS` 索引同步。`Design2-UI`、`Issue13` R27-05、`Build21` 等历史记录保持原样。
- **未覆盖与边界：** 未使用真实读屏软件或真机；49 个对象字段中实际交互 5 类代表字段，其余依据同一递归组件与 schema 核对；未覆盖订阅装配 SR／generic 目标、其他编辑器、Docker 镜像、远端 CI、外部客户端或生产环境。第 1～11 节记录的“节点退出静默丢草稿”“装配无效 JSON 无放弃入口”“装配跨分区覆盖”仍是既有行为，未在本轮修改。

### R35-06 节点创建和编辑界面显示 Mihomo 版本号

- **入口：** 节点管理 `/admin/nodes`，新建或编辑节点。
- **现象（原始反馈，2026-11-17 记录时保留）：** 用户反馈节点创建和编辑界面显示了 Mihomo 版本号。截图中新建 VLESS 节点时，「传输」和「安全」选择框显示了 `Mihomo 1.19.31 · 常用` 等内容。
- **期望（原始反馈，保留）：** 节点创建和编辑界面无需显示 Mihomo 版本号。
- **原始状态（2026-11-17 记录时）：** 待处理；本轮仅记录，未研究原因和修复方案，也未实施修复。
- **当前状态（2026-11-17）：** 已修复并完成本地验收。新建和编辑节点的已选值、展开选项和无障碍文本均省略 `verified`，保留「常用／扩展／旧版兼容／待验证」分组及协议 API 中的证据元数据。第 1～12 节保留修复前研究和独立复核，第 13 节记录实施与验收。

#### 1. 结论摘要

- **显示来源已定位。** 版本号不是前端硬编码，也不是从数据库节点记录读取。后端协议注册表的 `option()` 把 `mihomo-1.19.31` 写入推荐项的 `verified` 字段；`GET /api/admin/nodes/protocols` 随 `form_schema[].option_items[]` 下发；`EditableCombobox.vue` 的 `verifiedLabel()` 把 `mihomo-` 替换成 `Mihomo ` 并渲染为选项副标题。
- **不是只在下拉展开时出现。** 有当前值的字段会把同一副标题渲染在已选值上，因此新建 VLESS 默认的「传输」「安全」在未展开时也直接显示 `TCP Mihomo 1.19.31 · 常用`、`无 Mihomo 1.19.31 · 常用`；编辑 VLESS 时「Flow」「客户端指纹」等已有值也会显示。用户截图与这一实际表现一致。
- **实际范围是 4 个协议、21 个字段、83 个推荐项。** 只有 Shadowsocks、VMess、VLESS、Trojan 的 `option_items` 带 `verified`；其中 80 个为 `mihomo-1.19.31`，3 个 SS 2022 cipher 为 `project-unknown`。其余 15 个协议的当前表单无此文案。
- **组件层面实际显示版本的是 18 个下拉字段。** 21 个字段中，3 个 `ALPN` 是 `text-list`，其「添加推荐条目」只显示纯标签，不显示版本；其余 18 个字段使用 `EditableCombobox`，下拉副标题显示版本，共 77 个推荐项（74 个 Mihomo、3 个 `project-unknown`）。SS 2022 cipher 的 `project-unknown` 也出现在同一副标题位置。
- **版本元数据不参与兼容性判断或选项校验。** 后端 `validateOption()` 只按 `OptionItem.Value`／`Options` 和 `allow_custom` 判断；`OptionItem.Verified` 只在 `option()` 赋值和前端的 `verifiedLabel()` 读取。`TargetEvidence.Version`、诊断 `evidence` 是另一组证据元数据，`NodeCheckPanel.vue` 不渲染 `evidence`，当前目标检查面板也未出现版本号。
- **与历史结论存在明确冲突，已由用户确认新方向。** Issue15 的 R29-05 曾把 `Mihomo 1.19.29 · 常用` 作为“可读名称和明确分隔”的预期；本次期望相反。历史 Issue15 不重写，只在本文记录新决策及其覆盖关系；`Design4` §12.2 仍把 `verified` 定义为“验证来源/级别”，Build32 Step 1 仍把它作为前端 verified label 更新对象，后续实施时需要同步测试和文档口径。
- **调整可以是纯前端展示调整。** 用户确认保留后端/API metadata 和固定 Mihomo 版本合同；因此候选方向不需要删除 `OptionItem.Verified`、`TargetEvidence`、诊断 evidence 或 `.mihomo-test.sh`／固定版本测试。只调整 `EditableCombobox` 的视觉副标题和选项 `label`，并更新锁定旧文案的测试即可。
- 本轮**未实施任何界面调整**，也未把问题标记为已修复。

#### 2. 代码版本、运行环境与测试前提

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `869c0c1c48e3098a04a8817cabaa3ab84845189f`（`git describe` 为 `v4.1.4-11-g869c0c1`）；工作区起始已有未提交的 `Issue21.md`（R35-05 研究内容），已完整保留，未覆盖或清理 |
| 前端产物 | 使用工作区当前 `frontend/dist`，时间戳为 2026-11-17 11:13；`index.html` SHA-256 `3085dda5789ce99491824ed8dcba71a818b812c9cc5664a8459e065b48315a3e`，主 JS `index-BAQWrEVN.js` SHA-256 `f84620a7da4c039bbbd0930d8bffee8df56d3a2bb893cfffe406b894a33ec962`；本轮未在仓库内重新构建前端 |
| 后端构建 | 当前 HEAD 的 `backend/` 与上述 `frontend/dist` 复制到 `/tmp/r35-06` 后执行 `CGO_ENABLED=0 go build ./cmd/server`；服务二进制 SHA-256 `a3ef6882de6155bd60dfa99f109a21aafade23b26eae30f9a1af51845d9419b2`；仓库内未执行构建 |
| 运行环境 | Go 1.26.4（darwin/arm64）；Node.js 26.7.0；npm 11.19.0；macOS 27.2；Google Chrome 157.0.8103.79；Playwright-core 1.57.0（`/tmp/r35-02-tools`，未加入项目依赖） |
| 隔离实例 | `/tmp/r35-06`；`APP_MODE=prod`、`DATA_DIR=/tmp/r35-06/data`、`PORT=18096`，仅监听 `127.0.0.1`；未连接 8080、Docker、外部服务或真实账号 |
| 测试数据 | 从既有合成隔离库 `/tmp/r35-05/data` 复制；管理员 `admin@test.com`（[TestPasswordList.md](../../Reference/TestPasswordList.md) 的统一测试密码）；既有节点 `HK`（VLESS/TCP/REALITY）、`R35-05-HTTP`、`R35-05-WG`；为覆盖编辑路径临时创建 `R35-06-VMESS`、`R35-06-SS`、`R35-06-TROJAN`，检查后通过隔离 API 删除，未留在库中 |
| 视口 | 桌面 1440×1000；手机 375×785 |
| 证据目录 | `/tmp/r35-06/`（截图、DOM 文本、API JSON、日志，未加入仓库） |

**边界。** 本轮没有在项目工作区执行 `npm run build` 或 `go build`；隔离服务是在 `/tmp` 副本中构建的。前端产物与当前 HEAD 的一致性依据是工作区 `frontend/` 无代码差异、产物时间戳为 2026-11-17 11:13，且本轮未修改前端源码，而不是本轮重新构建。实际执行的是定向前端和后端测试（见第 10 节），没有执行全量测试、前端 build 或后端全量 build。测试库为既有合成库副本，临时节点检查后已删除；没有操作生产、预发布、外部服务、真实账号或真实数据。

#### 3. 复现步骤

1. 阅读 `registry.go`、`schema.go`、`node.go`、`server/node.go`、`api/node.ts`、`NodesView.vue`、`ProtocolFieldEditor.vue`、`EditableCombobox.vue`、`NodeCheckPanel.vue` 和相关测试，确认协议元数据、API 与前端渲染关系。
2. 把当前 HEAD 的 `backend/` 和当前 `frontend/dist` 复制到 `/tmp/r35-06`，构建并启动隔离服务；用合成管理员登录，进入 `/admin/nodes`。
3. 点击「新建节点」（默认 VLESS），不展开高级折叠区，记录「传输」「安全」的已选值；分别展开两个下拉，记录全部选项。
4. 选择安全 `TLS`，检查 `Flow`、`客户端指纹`、`ALPN`；把传输切到 `XHTTP`，检查 `xhttp-opts.mode`。用干净的「新建节点」分别选择 VMess、Trojan、Shadowsocks，打开全部折叠区和对应条件分支，逐个打开推荐下拉。
5. 用列表页「编辑」打开 `HK`，记录已选值及下拉；再临时创建 VMess／SS／Trojan 节点并按同样方式编辑检查，完成后删除。
6. 对 `HK` 执行「目标检查」，记录检查面板和响应；确认版本是否出现在面板、诊断或产物中。
7. 用 375×785 视口新建 VLESS，检查「传输」「安全」的文本和宽度；另用脚本对 19 个协议逐个新建并统计表单内可见的 `Mihomo` 文本。
8. 运行定向测试：前端 `npm test -- --run editable-combobox`（8 项）与后端 `go test ./internal/node -count=1`。

#### 4. 显示来源与数据流

- **注册表构造。** `backend/internal/node/registry.go` 的 `option(value, label, group, verified)`（约 1422 行）构造 `OptionItem`；`setOptionItems()` 同时写入 `OptionItems` 与旧的 `Options`。`enrichVLESS`／`enrichVMess`／`enrichTrojan`／`enrichSS`（约 1502～1831 行）为推荐项写入 `common`／`extended`／`legacy`／`pending` 分组和 `mihomo-1.19.31`；SS 2022 的三个 cipher 写入 `project-unknown`。
- **API 下发。** `backend/internal/node/node.go` 的 `GetProtocols()`（约 332 行）返回可编辑表单协议目录（`editorFormSchema` 移除新连接模型已取代的 VLESS/VMess 顶层兼容字段）；`GET /api/admin/nodes/protocols`（`backend/internal/server/node.go` 约 80、98 行）以 `{data:{list:[...]}}` 返回 `form_schema[].option_items[]`。
- **前端接收。** `frontend/src/api/node.ts` 的 `OptionItem`（约 48～53 行）保留 `value/label/group/verified`；`NodesView.vue` 的 `load()` 通过 `getProtocols()` 取目录（约 99 行）；`ProtocolFieldEditor.vue` 在字段类型为 `select` 或 `text` 且存在 `option_items` 时渲染 `EditableCombobox`（约 811 行）。
- **格式化与渲染。** `EditableCombobox.vue` 的 `verifiedLabel()`（约 54～57 行）执行 `mihomo-` → `Mihomo `；每组 `Select.Option` 的 `label` 由 `[label, value, verifiedLabel, groupLabel]` 拼接（约 146 行），可见插槽再分别输出值、`·` 和副标题（约 149～151 行）。已选值复用同一插槽，因此选中状态也带版本。
- **实际 DOM 证据。** 新建 VLESS 的 `[data-field-path="network"] .ant-select-selection-item` 为 `TCP` + `Mihomo 1.19.31` + ` · ` + `常用`；`security` 为 `无` + `Mihomo 1.19.31 · 常用`。同一浏览器会话中 `/api/admin/nodes/protocols` 响应的 `network`/`security` `option_items` 也带 `verified: "mihomo-1.19.31"`；API 响应与 DOM 文本已对照保存（`display-chain.json`）。
- **范围收口。** `getProtocols()` 只被 `NodesView.vue` 调用；`EditableCombobox` 只被 `ProtocolFieldEditor.vue` 使用。因此该显示路径只影响节点管理的新建/编辑表单，不扩散到其他页面。`NodesView.optionLabel()`（约 198 行）只用 `label` 生成「当前组合」，所以「当前组合」没有版本。
- **另一组版本数据。** `TargetEvidence.Version`、`backend/internal/assembly/node_check.go`／`node/check.go` 的 `evidence: mihomo-1.19.31-yaml`、`ssplugin` 合同和固定 Mihomo 测试脚本也包含版本；它们不经过 `EditableCombobox`，也不由 `NodeCheckPanel` 的 `evidence` 渲染。当前 `HK` 检查的面板和响应未见 `Mihomo`／`1.19.31`（响应中只有 `project-uri-capability` 等 evidence）。

#### 5. 新建与编辑、协议和字段的实际结果

| 协议 | 带 `verified` 的字段 | 推荐项数 | 实际 UI 结果 |
|------|------------------------|----------|----------------|
| Shadowsocks | `cipher`、`plugin`、`obfs-opts.mode`、`v2ray-plugin-opts.mode`、`client-fingerprint` | 23 | 5 个字段均使用 `EditableCombobox`，下拉均显示版本；新建默认 `plugin` 已选，显示 `不使用插件 Mihomo 1.19.31 · 常用`，`cipher` 为空，只在展开下拉时显示；`obfs`、`v2ray-plugin`、`shadow-tls` 三个条件分支均已实际打开检查；`cipher` 下拉中 3 个 SS 2022 项显示 `project-unknown · 待验证` |
| VMess | `network`、`security`、`cipher`、`client-fingerprint`、`alpn` | 21 | `network`、`security`、`cipher`、`client-fingerprint` 使用 `EditableCombobox`，下拉和已选值显示版本；`alpn` 是 `text-list`，「添加推荐条目」只显示 `h2`、`http/1.1`、`其他（自定义）`，不显示版本 |
| VLESS | `network`、`security`、`flow`、`xhttp-opts.mode`、`client-fingerprint`、`alpn`、`encryption` | 24 | 6 个 `EditableCombobox` 字段显示版本，`alpn` 不显示；新建默认「传输」「安全」直接显示版本，`encryption` 在默认折叠的「更多功能 / 高级参数」中，展开后显示；选择 TLS 后 `flow`／`client-fingerprint` 出现，切到 XHTTP 后 `xhttp-opts.mode` 出现，均已在真实表单中打开 |
| Trojan | `network`、`client-fingerprint`、`alpn`、`ss-opts.method` | 15 | `network`、`client-fingerprint`、`ss-opts.method` 使用 `EditableCombobox` 并显示版本；`alpn` 不显示；新建和编辑的 `network` 均显示；`client-fingerprint` 与高级区 `ss-opts.method` 在对应状态下已实际检查 |
| **合计** | **21 个字段** | **83** | **18 个字段、77 个下拉副标题显示版本（74 个 Mihomo、3 个 `project-unknown`）；3 个 `alpn` 列表字段不显示版本** |

**19 个协议的实际检查。** 在干净的「新建节点」中逐个切换协议并打开全部折叠区，统计表单内可见 `Mihomo` 文本：只有 Shadowsocks、VMess、VLESS、Trojan 的结果非零（分别为 1、3、3、1 处）；不展开高级区时，VLESS 默认只看到「传输」「安全」2 处，`encryption` 位于折叠的「更多功能 / 高级参数」内。Hysteria、Hysteria2、TUIC、WireGuard、HTTP、SOCKS5、Snell、AnyTLS、Mieru、MASQUE、OpenVPN、SSH、ShadowQUIC、TrustTunnel、Tailscale 均为 0。该结果与 API 侧只有这 4 个协议存在带 `verified` 的 `option_items` 一致。

**新建与编辑的实际差异。** 新建时只有已有默认值/已选值的字段在未展开时显示版本；空字段要展开下拉才看到。编辑已有值时，选中值直接显示版本：

| 场景 | 选中值直接显示版本的位置 | 备注 |
|------|---------------------------|------|
| 新建 VLESS（默认） | `network`、`security` | `encryption` 在默认折叠的「更多功能 / 高级参数」内 |
| 编辑 `HK`（VLESS/TCP/REALITY） | `network`、`security`、`flow`、`client-fingerprint` | 4 处；`encryption` 展开高级区后再显示 |
| 新建 VMess（默认/TLS）；编辑临时 VMess（WS+TLS） | `network`、`security`、`cipher` | 3 处 |
| 新建/编辑 Shadowsocks（临时节点） | `plugin`；编辑时还有 `cipher` | 新建默认 1 处，编辑 2 处 |
| 新建/编辑 Trojan（临时节点） | `network` | 1 处 |

**手机与目标检查。** 375×785 下新建 VLESS 的「传输」「安全」仍显示完整版本副标题；两个 `ant-select` 宽度约 343px、高度 44px，未观察到版本文字被截断。对 `HK` 执行「目标检查」后，面板显示 `clash-yaml`／`generic-subs`／`sr-subs` 的状态、诊断和脱敏产物，但未显示 `Mihomo` 或 `1.19.31`；当前检查响应也没有这两个字符串。

#### 6. 版本信息与兼容性判断、目标检查的关系

- **选项校验与版本无关。** `backend/internal/node/project.go` 的 `validateOption()`（约 329～357 行）在存在 `OptionItems` 时只比较 `item.Value`；`allow_custom` 决定清单外值是否允许。`Verified` 不参与允许值、必填、条件显示、清空或保存合并。
- **兼容性判断走独立链路。** 目标检查由 `backend/internal/node/check.go` 与 `backend/internal/assembly/node_check.go` 按协议适配器、字段合同和固定内核证据诊断；`TargetEvidence.Version` 和诊断 `Evidence` 只是证据标签，未被任何条件分支读取（`TargetEvidence` 在活动代码中仅被赋值，未被读取）。`OptionItem.Verified` 同样只被赋值和前端展示读取。
- **目标检查面板不显示版本。** `NodeCheckPanel.vue` 只渲染 target 状态、`code`、`message`、`field_path` 和 `preview`，不渲染 `diagnostic.evidence`；因此即使某些诊断响应携带 `mihomo-1.19.31-yaml`，节点表单也不显示该字符串。
- **固定版本合同仍在。** `.mihomo-test.sh` 校验二进制为 `Mihomo Meta v1.19.31`；后端多个 `mihomo_*_test.go` 和适配器注释、诊断 evidence 也使用 `v1.19.31`。这些是测试/证据合同，与 `OptionItem.Verified` 的界面展示无行为耦合；隐藏界面副标题不会改变生成结果或兼容性判断。
- **数据不持久化版本。** 测试库节点行的 `protocol_json` 中没有 `Mihomo` 或 `1.19.31`（查询计数为 0）；版本只存在于协议目录响应和构建/测试元数据中，保存节点不会写入它。
- **文档版本存在漂移。** 活动代码和 Build32 Step 1 使用 `1.19.31`，而 `Design4` 的 D16 与 §12.2 示例仍使用 `1.19.29`；这不影响当前显示链，但说明界面上的版本号是随证据元数据更新的展示值，不是前端稳定合同。

#### 7. 影响范围与候选调整方向

**已确认的影响范围（静态与实测一致）：**

- 只涉及节点管理的新建/编辑表单；`NodesView` 是唯一 `getProtocols()` 调用方，`EditableCombobox` 是唯一显示组件。
- 4 个协议、21 个字段、83 个推荐项；18 个字段、77 个下拉副标题显示版本，其中含 3 个 `project-unknown`；3 个 `alpn` 列表字段不显示版本。
- 版本同时出现在已选值和展开下拉中；只改下拉控件或只改 VLESS 的「传输」「安全」会留下其他字段/协议的不一致。
- `OptionItem.Value`、`Options`、`allow_custom`、条件显示、保存校验、目标检查、Clash／SR／generic 输出均不依赖 `Verified`；用户确认保留后端 metadata 时，显示调整不需要改 API 或数据。
- `project-unknown` 与 Mihomo 版本共用同一副标题；用户已确认“移除全部 `verified` 文案、保留分组”，因此 SS 2022 项应变为只显示值名称和「待验证」，不再出现 `project-unknown`。
- 视觉与无障碍文本都要去掉版本；如果只删可见 `<span>` 而保留 `Select.Option` 的 `label`，屏幕阅读器仍会读出 `Mihomo 1.19.31`，不满足用户确认的方向。
- 需要更新锁定旧文案的测试：`frontend/tests/editable-combobox.spec.ts` 当前明确断言 `Mihomo 1.19.31 · 常用`（约 91～92 行），本轮定向运行 8 项通过；仅改组件而不改测试会导致该测试失败。

**候选方向（仅记录，未实施）：**

- **A. 纯前端隐藏 `verified` 副标题，保留分组与后端 metadata（推荐，且与用户已确认方向一致）。** 修改 `EditableCombobox` 的可见副标题和 `Select.Option` 的 `label`，去掉 `verifiedLabel(item.verified)` 的渲染；保留 `groupLabel(item.group)`，并确保分组在无版本时仍有清晰分隔；更新测试断言；不改后端/API/数据。影响面最小，行为风险最低。
- **B. 只隐藏 `Mihomo ...`，保留 `project-unknown`。** 与用户“移除全部 verified 文案”的选择不一致；SS 2022 项仍会出现 `project-unknown`，不采纳。
- **C. 去掉 `verified` 和分组，只留值名称。** 更简洁，但会丢失“常用／扩展／旧版兼容／待验证”的辨识信息，与用户选择不一致，不采纳。
- **D. 从 API/后端删除 `OptionItem.Verified`。** 会破坏 Build32 的字段证据合同、未来版本核对和现有测试的可追溯性，且与用户“保留后端 metadata”的选择冲突，不推荐。
- **E. 把版本移到 tooltip／帮助文本。** 用户明确要求节点表单无需显示版本号；tooltip 仍可能在交互中显示，且会扩大改动面，不推荐。
- **F. 只处理 VLESS「传输」「安全」。** 与用户确认的“所有受影响协议与字段”不一致；其他字段/协议仍显示版本，不采纳。

**后续回归验证范围（实施时）：**

- 新建和编辑 SS／VMess／VLESS／Trojan，检查已选值与展开下拉：版本和 `project-unknown` 均消失，分组仍显示；`alpn` 列表保持现状。
- 空值、已有值、自定义值、分支切换后的选项重建、移动端 375px、浅色/深色主题和键盘/读屏文本。
- 节点保存、读取、检查、装配预览和生成仍正常；确认 `OptionItems`、`TargetEvidence`、诊断 evidence 未被改动；固定 Mihomo 门禁继续通过。
- 更新并运行 `editable-combobox.spec.ts`，补充 `nodes-view.spec.ts`／`protocol-field-editor.spec.ts` 的相关断言；执行前端全量测试和 build、后端 build/vet/test。上述回归尚未执行，因为本轮未实施。

#### 8. 用户确认的后续方向与待决策边界

用户于 2026-11-17 明确选择：

1. **范围：** 覆盖所有受影响协议与字段（Shadowsocks、VMess、VLESS、Trojan 的 21 个 `option_items` 字段），不只处理截图中的 VLESS「传输」「安全」。
2. **保留文字：** 移除全部 `verified` 文案（`Mihomo 1.19.31` 和 `project-unknown`），保留「常用／扩展／旧版兼容／待验证」分组。
3. **无障碍与内部数据：** 视觉和无障碍文本都去掉版本；后端/API 的 `verified` metadata、目标证据和固定 Mihomo 版本合同保留。

该方向**尚未实施**。以下事项仍需在实施或后续文档同步时决定，不能把候选方案当作已确认方案：

- 分组在无版本时的精确视觉：例如 `TCP · 常用` 还是 `TCP 常用`；分隔符和副标题字号需按现有控件规格确定。
- 无障碍 `label` 的精确内容：去掉版本后是否保留分组与规范值；用户已确认去掉版本，但未逐字确认 `label` 的最终格式。
- `Design4` §12.2、Build32 Step 1 与 Issue15 R29-05 的历史文案/预期如何同步。当前任务只授权修改 `Issue21.md`，本轮没有改写这些文档；Issue15 的历史结论保持原样。
- 是否在实施时补充“版本只作为内部证据、不在节点表单展示”的设计说明；属于后续 Design/Build 同步范围，不在本轮决定。

#### 9. 尚未确认内容与未覆盖范围

- 没有实施或验证“隐藏版本后”的实际界面；没有修改 `EditableCombobox`、测试或任何前端产物，不能宣称问题已修复。
- 没有用真实读屏软件验证；只读取了 DOM 和无障碍快照，确认当前 `Mihomo 1.19.31` 会进入选中值文本与 combobox 可访问文本。手机只验证了 375×785 的布局与文本，没有手机读屏、深色主题或横屏复测。
- 没有运行 Docker 镜像、远端 CI、真实客户端或真实代理连接；没有验证实施后的装配、订阅分发和客户端使用。
- 没有逐项测试所有 83 个推荐项在界面中的可见像素；对 21 个字段的下拉文本、默认/编辑选中值做了实测，对 15 个无 `option_items` 的协议做了整表检查。`ALPN` 的 `text-list` 只验证了“添加推荐条目”下拉，没有验证已添加标签的展示，因为当前编辑节点没有 ALPN 值。
- 没有搜索第三方前端消费者或未来插件对 `OptionItem.Verified` 的依赖；当前仓库内只有 `EditableCombobox` 读取该字段。API 仍保留字段，未来消费者可能使用，因此不能据“当前无行为依赖”断言外部无影响。
- 没有修改 `Design4`、Build32、Issue15 或任何历史记录；本轮唯一纳入版本管理的持久化变更是 `Issue21.md`。

#### 10. 支持证据摘要

| 证据 | 内容 |
|------|------|
| `protocols.json`／`display-chain.json` | 19 个协议、21 个带 `option_items` 的字段、83 个推荐项（80 个 `mihomo-1.19.31`、3 个 `project-unknown`）；VLESS `network`/`security` 的 API 元数据与 DOM 选中值逐项对照 |
| `02-new-vless.png`、`03-network-dropdown.png` | 新建 VLESS 的已选值与「传输」下拉显示 `Mihomo 1.19.31 · 常用`／`扩展` |
| `edit-hk-form.png`、`edit-vmess.png`、`edit-R35-06-SS.png`、`edit-R35-06-TROJAN.png` | 四种协议编辑态选中值；VLESS `HK` 同时显示 `network`／`security`／`flow`／`client-fingerprint` |
| `check-all-19.log`、`all-protocols.json` | 逐协议新建检查；只有 4 个协议非零，15 个协议为 0 |
| `field-results.json`／`inspect-all-v2.log` | 21 个字段的下拉文本；`alpn` 列表只显示纯标签；SS 2022 项显示 `project-unknown · 待验证` |
| `edit-check-response.json`／`inspect-edit-and-check.log` | `HK` 的目标检查响应与面板文本；面板/响应无 `Mihomo`／`1.19.31` |
| `mobile-new-vless.png`／`inspect-mobile.log` | 375×785 下版本文字仍显示，字段宽度约 343px、高 44px |
| `npm test -- --run editable-combobox` | 8 项通过；当前测试锁定 `Mihomo 1.19.31 · 常用` |
| `go test ./internal/node -count=1` | 通过；用于确认当前后端节点/协议测试状态 |

#### 11. 本轮未做

本轮只做测试、研究与文档记录。唯一纳入版本管理的持久化变更是 `Issue21.md`；没有修改任何项目代码、配置、依赖或锁文件，没有修改 `frontend/dist` 或实施 UI 调整，没有将问题标记为已修复，没有操作生产、预发布、外部服务、真实账号或真实数据。隔离库中的临时 VMess／SS／Trojan 节点已在检查后删除。

#### 12. 2026-11-17 当前 HEAD 复核记录

**复核目的与边界。** 当前仓库 HEAD `70f388b` 已提交第 1～11 节记录（研究时的代码基线为 `869c0c1`）。为避免只复述既有结论，本轮从当前 HEAD 重新构建隔离实例，用实际页面、DOM、API 响应和定向测试独立复核。本节不改变第 1～11 节的原始记录，也不重新决策第 8 节记录的用户方向；**未实施任何界面调整，R35-06 仍未修复**。

**12.1 代码版本、运行环境与测试前提**

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `70f388b400b745cd67fe02c524fb8291d6d16095`（`git describe --tags --always` 为 `v4.1.4-12-g70f388b`）；复核开始时工作区干净，无未提交/未跟踪改动 |
| 前端产物 | `frontend/dist/index.html` SHA-256 `3085dda5789ce99491824ed8dcba71a818b812c9cc5664a8459e065b48315a3e`，主 JS `assets/index-BAQWrEVN.js` SHA-256 `f84620a7da4c039bbbd0930d8bffee8df56d3a2bb893cfffe406b894a33ec962`；产物时间 2026-11-17 11:13；`4637cff..HEAD` 无前后端代码差异，产物与当前 HEAD 源码一致；未在仓库内重新构建前端 |
| 后端构建 | 将当前 `backend/` 与 `frontend/dist` 复制到 `/tmp/r35-06-recheck`，用当前产物覆盖复制件的 `backend/web/dist` 后执行 `CGO_ENABLED=0 go build -o ... ./cmd/server`；服务二进制 SHA-256 `0bedaf4a6776d8985bb690dd1104c80d13165dceb141e5925f86d20499ee6744`；仓库内未执行构建 |
| 运行环境 | Go 1.26.4（darwin/arm64）；Node.js 26.7.0；npm 11.19.0；macOS 27.2；Google Chrome 157.0.8103.79；Playwright-core 1.57.0（`/tmp/r35-02-tools`，未加入项目依赖） |
| 隔离实例 | `/tmp/r35-06-recheck`；`APP_MODE=prod`、`DATA_DIR=/tmp/r35-06-recheck/data`、`PORT=18097`，仅监听 `127.0.0.1`；未连接 8080、Docker、外部服务或真实账号 |
| 测试数据 | 复制既有合成隔离库 `/tmp/r35-06/data`；管理员 `admin@test.com`（[TestPasswordList.md](../../Reference/TestPasswordList.md) 统一密码 `12345678`）；既有节点 `HK`（VLESS/TCP/REALITY）、`R35-05-HTTP`、`R35-05-WG`；临时节点 `RECHECK-VMESS`、`RECHECK-SS`、`RECHECK-TROJAN`（ID 7～9）检查后已通过隔离 API 删除 |
| 视口与证据 | 1440×1000；375×785；证据目录 `/tmp/r35-06-recheck`（截图、DOM 文本、API JSON、日志，未加入仓库）；关键文件 `protocols-api.json`、`all-protocols.json`、`field-results.json`、`edit-check-response.json`、`check-a11y.log`、`inspect-mobile.log` 与 `shots/*.png` |

**本地构建前提。** 工作区 `backend/web/dist` 是 2026-08-25 的旧嵌入产物（`assets/index-48_-me5R.js`），而 `frontend/dist` 是 2026-11-17 11:13 的当前产物（`assets/index-BAQWrEVN.js`）。首次直接构建时服务返回旧 UI，新建 VLESS 的「传输」「安全」没有版本文案；把 `frontend/dist` 覆盖到复制件的 `backend/web/dist` 后重新构建，才复现 R35-06。项目的 `Dockerfile` 第 16 行本来就把 frontend 构建产物 `COPY` 到 `backend/web/dist`；按 Dockerfile 静态核对，这是本地手工构建的前提，不是生产构建缺陷。覆盖只发生在 `/tmp` 副本，未修改工作区 `backend/web/dist` 或 `frontend/dist`。

**12.2 复核步骤**

1. 核对 `git status`、`git describe` 与源码；确认工作区干净、HEAD 与第 1～11 节记录的关系。
2. 按上述前提构建隔离服务，登录后确认 `/api/site/info` 可用；在 Chrome 中打开 `/admin/nodes`。
3. 请求 `GET /api/admin/nodes/protocols`，统计 19 个协议、带 `option_items` 的字段和推荐项数量，并保存响应。
4. 在新建节点中逐个切换 19 个协议，展开全部 `<details>`，统计表单内可见的 `Mihomo` 文本；再对 VLESS 不展开高级区复核默认的「传输」「安全」。
5. 覆盖 VLESS／VMess／Trojan／Shadowsocks 的 12 个表单场景，逐个打开 18 个使用 `EditableCombobox` 的协议字段和 3 个 `alpn` 列表，记录已选值、下拉文本和分组。
6. 编辑 `HK` 及临时 VMess／SS／Trojan 节点，记录已选值；执行「目标检查」，对照面板文本和响应；用 375×785 视口检查移动端；读取选项的 `label` 属性确认无障碍文本。
7. 运行定向测试：前端 `npm test -- --run editable-combobox`（8 项）与后端 `go test ./internal/node -count=1`。

**12.3 复核结果**

- **显示链路与第 1～11 节一致。** `backend/internal/node/registry.go:1422` 的 `option()` 把 `mihomo-1.19.31` 写入 `OptionItem.Verified`；`schema.go:38-44` 定义该字段；`node.go:333` 的 `GetProtocols()` 与 `server/node.go:80,98` 的 `GET /api/admin/nodes/protocols` 下发；`frontend/src/api/node.ts:48-53` 接收；`NodesView.vue:99` 调用；`ProtocolFieldEditor.vue:811` 渲染 `EditableCombobox`；`EditableCombobox.vue:54-57` 的 `verifiedLabel()` 把 `mihomo-` 替换为 `Mihomo `，`:146` 把它拼入 `Select.Option` 的 `label`，`:149-151` 把它渲染为可见副标题。已选值与展开下拉复用同一段文本。
- **数量与第 1～11 节一致。** API 返回 19 个协议、21 个带 `option_items` 的协议字段、83 个推荐项：80 个 `mihomo-1.19.31`、3 个 SS 2022 cipher 的 `project-unknown`。按字段类型分为 18 个 `EditableCombobox` 字段（77 个推荐项，74 个 Mihomo、3 个 `project-unknown`）与 3 个 `alpn` 的 `text-list` 字段（各 2 个推荐项，只显示纯标签）。
- **逐协议实测。** 展开全部折叠区后，表单内可见 `Mihomo` 文本数：Shadowsocks 1、VMess 3、VLESS 3、Trojan 1；其余 Hysteria、Hysteria2、TUIC、WireGuard、HTTP、SOCKS5、Snell、AnyTLS、Mieru、MASQUE、OpenVPN、SSH、ShadowQUIC、TrustTunnel、Tailscale 均为 0。VLESS 不展开「更多功能 / 高级参数」时只有「传输」「安全」2 处，`encryption` 在折叠区内。
- **新建与编辑。** 新建 VLESS 的已选值显示 `TCP` + `Mihomo 1.19.31 · 常用`、`无` + `Mihomo 1.19.31 · 常用`；「当前组合」只使用 `label`（`NodesView.vue:195-201`），显示 `VLESS · TCP · 无`，不含版本。编辑 `HK` 的 `network`、`security`、`flow`、`client-fingerprint` 4 处直接显示版本。临时编辑 VMess（WS+TLS）显示 `network`、`security`、`cipher` 3 处；SS 显示 `cipher`、`plugin` 2 处；Trojan 显示 `network` 1 处。选择 SS 2022 cipher 后已选值显示 `SS 2022 AES-128` + `project-unknown · 待验证`；`alpn` 的「添加推荐条目」只有 `h2`、`http/1.1`、`其他（自定义）`。
- **无障碍文本。** 展开 VLESS「传输」下拉时，选项 DOM 的 `label` 属性为 `TCP tcp Mihomo 1.19.31 常用`；可见文本为 `TCP` + `Mihomo 1.19.31 · 常用`。因此只删除可见 `<span>`、不改 `Select.Option` 的 `label`，屏幕阅读器仍可能读出 `Mihomo 1.19.31`。
- **行为关联。** `backend/internal/node/project.go:329-357` 的 `validateOption()` 只比较 `OptionItem.Value`（或 `Options`）和 `allow_custom`，不读取 `Verified`；活动代码中 `Verified` 只在 `registry.go:1423` 赋值、由前端展示；`TargetEvidence` 仅在注册表赋值，未发现读取其 `Version` 的业务分支；`NodeCheckPanel.vue` 只渲染 status、code、message、field_path 与 preview，不渲染 diagnostic `evidence` 或 `target_evidence`。对 `HK` 执行目标检查后面板与响应未出现 `Mihomo`／`1.19.31`；对 VMess REALITY 草稿只核对了 API 响应，亦未出现这两个字符串。`GET /api/admin/nodes` 响应与测试库中 `HK`、`R35-05-HTTP`、`R35-05-WG` 的 `protocol_json` 均未出现该字符串。
- **目标检查与兼容性判断保持独立。** 检查使用协议适配器、字段合同和固定内核证据诊断；固定 `Mihomo v1.19.31` 的测试合同、`.mihomo-test.sh`、`evidence: mihomo-1.19.31-yaml` 和 `TargetEvidence.Version` 都不是由 `OptionItem.Verified` 驱动，也不因隐藏副标题而改变。由于当前未实施，上述是“只改显示”的静态与实测影响判断，不是修改后的回归结果。
- **现有测试依赖旧文案。** `frontend/tests/editable-combobox.spec.ts:91-92` 明确断言 `Mihomo 1.19.31 · 常用`；本轮定向运行 8 项通过。若按第 8 节方向实施，必须同步修改该断言；`nodes-view.spec.ts` 与 `protocol-field-editor.spec.ts` 的 fixture 未设置 `verified`，当前检索未发现其他前端测试依赖该文案。
- **只改显示的影响范围。** 若仅删除 `verified` 副标题并保留后端 metadata：不影响 `OptionItem` 数据、`validateOption`、保存/读取、目标检查、Clash／SR／generic 输出或固定 Mihomo 测试；需同步修改 `editable-combobox.spec.ts` 的文案断言，并同时去掉可见 `<span>` 与 `Select.Option` 的 `label` 才满足无障碍要求；「常用／扩展／旧版兼容／待验证」分组必须保留。除节点管理表单外，未发现其他页面渲染该版本。
- **文档口径冲突仍待后续同步。** `Design4.md` §12.2 示例仍用 `mihomo-1.19.29` 且把 `verified` 定义为“验证来源/级别”；`Build32.md` Step 1 仍把“前端 verified label”作为版本升级对象；`Issue15.md` R29-05 的预期是 `Mihomo 1.19.29 · 常用`。本轮只按用户已确认的新方向记录，未改写这些历史文档，后续实施需决定同步方式。
- **移动端。** 375×785 下新建 VLESS 的「传输」「安全」仍显示完整副标题；`ant-select` 宽度 343px、高 44px，未发现截断。
- **候选方向不变。** 第 7、8 节记录的 A（纯前端隐藏 `verified` 副标题、保留分组与后端 metadata）及用户已确认的范围继续有效；本次复核没有产生新的方案决策，也没有实施。

**12.4 尚未确认与未覆盖**

- 没有实施，也没有验证“隐藏版本后”的实际界面；不能宣称问题已修复。
- 没有使用 VoiceOver／NVDA 等真实读屏软件，只检查了 DOM 与 `label` 属性；没有 iPhone/Android 真机、深色主题或屏幕阅读器实测。
- 没有逐个像素查看全部 83 个推荐项，只核对了 18 个下拉字段的完整选项文本、3 个 `alpn` 列表和 4 个协议的编辑态；未覆盖所有自定义值、分支切换后的极端状态和保存后重开。
- 未运行前端全量测试、`npm run build`、后端全量 `go build/vet/test`、Docker 镜像、远端 CI、真实客户端或真实代理连接；只运行了上述定向测试。
- 未检查仓库外的第三方 API 消费者是否使用 `OptionItem.Verified`；API 字段保留，外部影响不能据当前仓库断言。
- 未修改 `Design4`、`Build32`、`Issue15` 或任何历史记录；本轮唯一纳入版本管理的持久化变更为 `Issue21.md`。

**12.5 本轮未做**

本轮只做测试、研究与文档记录。没有修改任何项目代码、配置、依赖或锁文件，没有修改工作区 `frontend/dist` 或 `backend/web/dist`，没有实施 UI 调整，没有将 R35-06 标记为已修复，没有操作生产、预发布、外部服务、真实账号或真实数据。隔离库中的临时节点已删除。

#### 13. 2026-11-17 实施与验收

- `EditableCombobox.vue` 的已选值与展开选项使用「名称 · 分组」，如「TCP · 常用」。无分组选项只显示名称。选项 `aria-label` 包含显示名、规范值和分组。两处文案省略 Mihomo 版本与 `project-unknown`，SS 2022 项继续显示「待验证」，ALPN 推荐条目使用原有名称。
- 隔离 Chrome 对照修复前的新建表单，覆盖 Shadowsocks／VMess／VLESS／Trojan 的 18 个下拉和 3 个 ALPN 字段，检查选项文字、顺序、DOM 与无障碍快照。四协议编辑态、键盘选中 WebSocket 后保存并重开、375×785 明暗主题均通过。协议目录 JSON 与修复前一致，VLESS `HK` 除每次生成的检查 ID 外，目标检查结果也与基线一致。
- 前端全量 51 文件 471 项与生产构建、后端全量编译／静态检查／测试、固定 Mihomo v1.19.31 门禁通过。实际读屏软件、手机真机、Docker 镜像与远端 CI 未执行。实施与验证命令见 [Build37.md](../Build/Build37.md)。
- [Design4](../Design/Design4.md) v1.34 记录现行展示规则和协议证据用途，AGENTS 已更新构建入口。Build32 Step 1 和 Issue15 R29-05 记录当时的展示行为，当前规则以 Design4 §3.2 为准。

### R35-07 目标检查位于高级数据区域内

- **入口：** 节点管理 `/admin/nodes` 的节点表单。
- **现象（原始反馈，保留）：** 截图中「目标检查」功能位于「高级数据与目标检查」折叠区域内部。
- **期望（原始反馈，保留）：** 将「目标检查」功能移动到「高级数据与目标检查」区域外部。
- **原始状态（2026-11-17 记录时）：** 待处理；本轮仅记录，未研究原因和修复方案，也未实施修复。
- **当前状态（2026-11-17）：** 已按用户确认的方向实施并通过本地验收。「目标检查」位于「更多功能 / 高级参数」之后、「高级数据」之前。检查结果默认折叠为一条汇总，重新检查立即移除旧结果。检查请求所用草稿变更时结束旧请求的加载提示，迟到响应仍被忽略。第 1～11 节为修复前研究记录，第 12 节为实施及验收结果。

#### 1. 结论摘要

- **当前位置已定位。** `frontend/src/views/admin/NodesView.vue` 的 `<details class="node-advanced-data">`（约 1249～1317 行）的标题是「高级数据与目标检查」，内容先渲染「未知扩展」，再在约 1315 行渲染 `<NodeCheckPanel>`。因此目标检查与未知扩展共用同一个默认折叠容器，而不是同级独立区域。
- **不是条件区域。** 该 `<details>` 没有 `v-if`，所有 19 个协议的新建和编辑表单都会渲染；`NodeCheckPanel` 也没有 `v-if`，在表单浮层打开期间始终挂载。折叠只影响可见性，不卸载组件。桌面 1440×1000 下，新建 VLESS 的高级数据摘要顶部约在 1184px，已在首屏下方；编辑节点的表单更长。用户必须滚动到表单底部并再展开一次，才能看到目标检查。
- **显示条件。** 浮层打开（新建或编辑）后，折叠区摘要始终存在；展开后显示未知扩展和检查面板。检查按钮仅在存在未应用 JSON 草稿或未应用自定义值／列表项草稿时禁用；表单必填、类型错误和未填写完整不会在客户端阻止检查，而是由服务端返回目标诊断。检查面板自身不依赖协议、编辑模式或高级数据展开状态。
- **数据来源与流程。** `NodesView.vue` 的 `checkRequest`（约 253～273 行）从当前响应式草稿生成 `node_id`／`base_revision`（仅编辑）、`protocol`、`host`、`port`、`protocol_json`、`current_state`、`reset_scopes`、`credential_ops`、扩展操作和三个默认目标；`checkNode()`（`frontend/src/api/node.ts` 约 253～254 行）调用 `POST /api/admin/nodes/check`。后端 `Service.Check`（`backend/internal/node/check.go` 约 100 行起）合并凭据、校验草稿并调用实际适配器，返回 `targets`、诊断和脱敏预览，不写节点、版本或扩展。检查不产生真实连接。
- **与高级数据区域的关系是 DOM 嵌套和共享标题，不是组件或状态耦合。** `NodeCheckPanel` 只接收 `request` 和 `blockedReason`，发出 `locate`、`conflict`；未知扩展由父组件 `extensionOps` 管理，检查请求只是读取该状态。关闭折叠区不会清空检查结果，重新展开后结果仍在（已实测）。因此仅移动模板位置不会改变请求、结果、失效、409 或保存流程。
- **移动本身影响以布局和可见性为主，但用户选择的结果摘要会带来组件行为改动。** 独立内联区域会去掉折叠区的双层边框和一次展开步骤；但目标检查仍位于长表单下部，不会自动变成首屏入口。在新建 VLESS 且三个目标均有结果的场景下，检查面板当前为 338px（桌面）／418px（移动端）常驻高度；用户已选择默认折叠成摘要，因此实施时需要新增摘要／展开状态，不能只移动模板。
- **与现行设计存在明确冲突。** `Design4.md` D9（第 65 行）记录「高级区按需打开脱敏检查」且注明用户确认；§3.1（约 114 行）把「高级数据与目标检查」列为默认折叠区；§7.2（约 403 行）写「节点高级区按需检查当前草稿」。`docs/reports/Build/Build19.md` Step 3／Step 5 也按高级区记录。用户已确认在本文记录本次要求覆盖这些方向，其他文档留待后续同步；本轮未修改 `Design4`、Build19 或任何历史记录。
- 本轮**未实施任何界面调整**，未修改代码、配置、依赖或锁文件，未将问题标记为已修复。

#### 2. 代码版本、运行环境与测试前提

| 项目 | 实际值 |
|------|--------|
| 代码基线 | 分支 `beta`，HEAD `d2b705ed225e495412f2592c670bc85bb726ee59`（`git describe` 为 `v4.1.4-13-gd2b705e`）；最后一个代码提交为 `4637cff`（2026-11-17 11:13） |
| 工作区状态 | 研究开始时工作区干净（无未提交或未跟踪改动）；本轮唯一持久化修改为 `Issue21.md` |
| 前端产物 | 工作区 `frontend/dist` 时间戳 2026-11-17 11:13；`index.html` SHA-256 `3085dda5789ce99491824ed8dcba71a818b812c9cc5664a8459e065b48315a3e`，主 JS `index-BAQWrEVN.js` SHA-256 `f84620a7da4c039bbbd0930d8bffee8df56d3a2bb893cfffe406b894a33ec962`，主 CSS `index-DLg-fzOL.css` SHA-256 `8dad59dcc6b012723843bd18210ecce25d471d50323776a909a19ca6b77b73e2`；与 R35-06 复核记录一致，本轮未在仓库内重新构建 |
| 后端构建 | 将当前 HEAD 的 `backend/` 与上述 `frontend/dist` 复制到 `/tmp/r35-07` 后执行 `CGO_ENABLED=0 go build ./cmd/server`；服务二进制 SHA-256 `d81ea73a9b30c652d3ad7b02d47a55351c77abff6585f50d7f0de2c949201434`；仓库内未执行构建 |
| 运行环境 | Go 1.26.4（darwin/arm64）；Node.js 26.7.0；npm 11.19.0；macOS 27.2（26B123）；Google Chrome 157.0.8103.79；Playwright-core 1.57.0（`/tmp/r35-02-tools`，未加入项目依赖） |
| 隔离实例 | `/tmp/r35-07`；`APP_MODE=prod`、`DATA_DIR=/tmp/r35-07/data`、`PORT=18097`；通过 `127.0.0.1:18097` 访问；未使用本机 8080 的既有进程、Docker、外部服务或真实账号 |
| 测试数据 | 从既有合成隔离库 `/tmp/r35-06/data` 通过 `sqlite3 .backup` 复制；管理员 `admin@test.com`（[TestPasswordList.md](../../Reference/TestPasswordList.md) 的统一测试密码）；既有节点 `HK`（VLESS/TCP/REALITY）、`R35-05-HTTP`、`R35-05-WG`。为验证检查后保存临时创建 `R35-07-SAVE`，检查后通过隔离 API 删除；为验证 409 将隔离库中 `HK` 的编辑修订号推进到 3。以上均只发生在 `/tmp` 隔离库，未触碰仓库 `backend/data` |
| 视口与主题 | 桌面 1440×1000、1440×1400；手机 375×785，移动端 Drawer；仅浅色主题。未覆盖深色、横屏、200% 缩放、Safari／Firefox |
| 证据目录 | `/tmp/r35-07/`（截图、DOM 文本、API JSON、日志、测量结果）；均未加入仓库 |

**边界。** 测试库为合成数据副本；目标检查只使用本地代码和适配器，未访问外部服务、真实客户端或真实代理连接。前端定向测试与后端定向测试在仓库内执行（见第 10 节），但没有执行前端全量测试、`npm run build`、后端全量 `go build`／`go vet`／`go test`、Docker 镜像或远端 CI。测试执行后（文档编辑前）工作区无跟踪文件改动。

#### 3. 复现步骤

1. 阅读 `NodesView.vue`、`NodeCheckPanel.vue`、`api/node.ts`、`backend/internal/server/node.go`、`backend/internal/node/check.go` 及相关测试，确认目标检查的模板位置、显示条件、请求构造和后端落库边界。
2. 将当前 HEAD 的 `backend/` 复制到 `/tmp/r35-07`，将工作区 `frontend/dist` 复制为 `backend/web/dist`，用 `CGO_ENABLED=0 go build ./cmd/server` 构建隔离服务；用 `sqlite3 .backup` 复制 `/tmp/r35-06/data/app-prod.db` 到 `/tmp/r35-07/data`。
3. 以 `APP_MODE=prod DATA_DIR=/tmp/r35-07/data PORT=18097` 启动服务，通过 `127.0.0.1:18097/api/auth/login` 使用 `admin@test.com` 登录，把返回 Token 注入浏览器 `localStorage.token`。
4. 打开 `/admin/nodes`，点击「新建节点」，记录「高级数据与目标检查」折叠区的默认关闭状态、目标检查是否可见、摘要位置；再点击摘要展开，记录未知扩展与目标检查的先后顺序。
5. 用列表页「编辑」打开 `HK`，重复记录；分别测试新建与编辑、检查请求中的 `node_id`／`base_revision` 差异。
6. 在新建和编辑中点击「检查当前节点」，记录进行中、成功结果、错误诊断、折叠后再展开、草稿修改后的失效，以及 409 冲突表现。
7. 通过修改真实表单字段复现未应用 JSON 草稿和未应用自定义值草稿，确认检查按钮禁用、「定位草稿」聚焦对应控件；用延迟响应模拟检查进行中及迟到响应。
8. 对代表性协议（Shadowsocks、VMess、VLESS、Trojan、Hysteria2、WireGuard、HTTP、OpenVPN、Tailscale）重复新建检查；用 375×785 视口重复布局测量。
9. 检查后保存一个临时节点，确认检查不阻断保存、不落库；检查后删除临时节点。执行前端 `npm test -- --run node-check-panel nodes-view` 与后端 `go test ./internal/node -count=1`。

#### 4. 当前所在位置、显示条件与相关组件

**模板位置与顺序（当前实现）：**

| 顺序 | 元素 | 位置／说明 |
|------|------|-----------|
| 1 | 基本信息、认证与密钥、连接方式与当前参数、独立开关 | `NodesView.vue` 中的 `FormSection`；默认展开 |
| 2 | 更多功能 / 高级参数 | `<details class="node-advanced-fields">`，默认折叠 |
| 3 | 高级数据与目标检查 | `<details class="node-advanced-data">`，默认折叠；约 1249 行 |
| 3.1 | 未知扩展 | 折叠区内约 1253～1312 行 |
| 3.2 | 目标检查 | 同一折叠区内约 1314～1316 行，`<NodeCheckPanel>` |

**相关文件：**

- `frontend/src/views/admin/NodesView.vue:1130`：`FormOverlay` 使用 `width=920`、`destroy-on-close`，桌面 Modal、手机底部 Drawer。
- `frontend/src/views/admin/NodesView.vue:1249-1317`：高级数据折叠区、未知扩展和 `NodeCheckPanel` 的模板位置；`summary` 为「高级数据与目标检查」，说明为「未知扩展摘要、局部 JSON 草稿与脱敏目标检查；默认折叠。」
- `frontend/src/views/admin/NodesView.vue:253-273`：`checkRequest` 由当前草稿构造；`node_id`／`base_revision` 仅编辑时存在。
- `frontend/src/views/admin/NodesView.vue:441-454`：`locateBlockedDraft()` 与 `checkBlockedReason`；阻断原因只覆盖未应用 JSON 和未应用自定义值／列表项草稿。
- `frontend/src/components/NodeCheckPanel.vue:21-52`：`checking`、`result`、`error`、`seq` 本地状态；`watch(request)` 在请求变化时清空结果；迟到响应按序号丢弃；409 发 `conflict`。
- `frontend/src/components/NodeCheckPanel.vue:76-115`：面板 DOM；按钮「检查当前节点」、阻断／错误 Alert、进行中文本、逐目标状态／诊断／脱敏产物 `<details>`。
- `frontend/src/api/node.ts:177-211`、`:253-254`：`NodeCheckRequest`／`NodeCheckResponse` 类型及 `POST /admin/nodes/check` 调用。
- `backend/internal/server/node.go:75`、`:157-190`：路由与处理函数；`backend/internal/node/check.go` 的 `Service.Check` 负责草稿合并、校验、适配器调用和响应，不写节点表。

**显示条件：**

- 浮层打开（`creating=true`）时，`<details>` 和 `NodeCheckPanel` 均被挂载；关闭浮层时 `destroy-on-close` 销毁，结果不持久化。
- `<details>` 没有 `v-if`，不依赖当前协议是否存在高级字段，也不依赖 `groupFields('advanced').length`；摘要对 19 个协议都显示。
- `<details>` 关闭时内容不可见；浏览器仍保留 DOM。`NodeCheckPanel` 的组件状态（包括结果）在折叠／展开之间保留，已实测。
- 检查按钮可见但可能 `disabled`：当 `checkBlockedReason` 非空时禁用；此时显示「定位草稿」按钮和警告 Alert。
- 未填写必填字段、host 为空或 UUID 为空不会禁用检查；点击后由后端返回 `invalid_node_draft` 等诊断。

**当前布局测量（隔离实例、浅色、动画完成后）：**

| 场景 | 视口 | 表单内容高度 | 目标检查面板高度 | 备注 |
|------|------|--------------|------------------|------|
| 新建 VLESS，折叠区关闭 | 1440×1000 | 1060px | 不可见 | 高级数据摘要顶部约 1184px，已在首屏下方 |
| 新建 VLESS，展开高级数据 | 1440×1000 | 1272px | 70px | 面板仍位于页面下部，需要继续滚动 |
| 新建 VLESS，完成检查 | 1440×1000 | 1540px | 338px | 三个目标结果各约 72px，预览默认折叠 |
| 新建 VLESS，折叠区关闭 | 375×785 | 1636px | 不可见 | Drawer 内容区高 651px；高级数据摘要位于内容约 1685px 处 |
| 新建 VLESS，展开高级数据 | 375×785 | 1912px | 102px | 检查按钮 100×44px；桌面同按钮约 24px 高 |
| 新建 VLESS，完成检查 | 375×785 | 2228px | 418px | Drawer 底部操作栏固定，正文内容高度约 2268px |

以上为内容高度测量值；浏览器缩放、系统字号和长诊断会改变结果。编辑 `HK` 的表单比新建 VLESS 更长，在 1440×1400 下最后一个折叠区底部已超过 1600px，目标检查仍在首屏之外。

#### 5. 与高级数据区域、节点草稿和保存流程的实际关系

- **组件依赖：** `NodeCheckPanel` 不接收未知扩展列表、折叠区状态或父组件实例；只接收 `request`（完整草稿）和 `blockedReason`，并发出 `locate`、`conflict`。未知扩展的 `extensionOps` 由 `NodesView` 管理，`checkRequest` 读取它；应用／删除扩展会改变请求，从而让旧检查结果失效。这是父级状态依赖，不是对折叠区 DOM 的生命周期依赖。
- **草稿依赖：** 检查使用当前表单草稿，不使用数据库中的旧值作为当前分区兜底。新建请求不含 `node_id`；编辑请求含 `node_id`、`base_revision`、`reset_scopes`、`credential_ops`。`protocolPayload()` 与保存共用同一活动参数构造；未应用的 JSON／自定义值／列表项草稿不会进入请求，并会阻止检查。检查不读取或修改未应用草稿。
- **保存依赖：** `save()`（约 715 行）与 `checkRequest` 共用 `protocolPayload()`、`current_state`、`reset_scopes`、`credential_ops` 和扩展操作，但保存走 `POST /api/admin/nodes` 或 `PUT /api/admin/nodes/:id`，检查走 `/api/admin/nodes/check`。保存不要求先检查，检查结果也不参与保存判定；检查不写库。实测先检查再保存，节点创建成功、浮层关闭，检查结果随浮层销毁。
- **折叠依赖：** 折叠 `<details>` 不会取消检查请求、清空结果或重置 `NodeCheckPanel`；只改变可见性。实测检查后关闭折叠区再打开，结果文本保持一致。关闭整个浮层会销毁组件，结果丢失；再次打开显示数据库中的已保存状态。
- **未知扩展关系：** 未知扩展仍只作为加密存档和诊断输入，不进入 Clash／SR／generic 产物；检查请求通过 `extensions`（新建）或 `extension_ops`（编辑）携带它们。检查面板不直接显示扩展；移动面板不会改变扩展的保存或输出语义，但会让“应用扩展后检查结果立即清空”这一反馈更直接可见。
- **后端与外部边界：** `/check` 由管理员会话和角色中间件保护；`Service.Check` 在编辑时校验修订号，返回 409 时不写库；新建时只做草稿检查。检查调用项目内适配器生成脱敏预览和诊断，没有外部服务调用、真实连接或数据库写入。

#### 6. 新建、编辑及其他状态的实际结果

| 状态 | 操作 | 实际结果 | 覆盖 |
|------|------|----------|------|
| 新建、默认折叠 | 新建 VLESS，打开 `/admin/nodes` 后不展开高级数据 | 摘要「高级数据与目标检查」存在；目标检查不可见；表单底部在首屏外 | 已测 |
| 新建、展开 | 点击高级数据摘要 | 先显示未知扩展，再显示目标检查；检查按钮可见 | 已测 |
| 新建、空草稿 | 不填 host 直接检查；另一次保留 host 但清空 UUID 后检查 | 按钮可用；均返回 3 个目标的 `invalid_node_draft`：空 host 报「服务器地址不能为空」，缺 UUID 报「字段 uuid 必填」 | 已测 |
| 新建、有效 VLESS | 填 host `1.1.1.1`、UUID，检查 | `clash-yaml`／`generic-subs`／`sr-subs` 均为 `ok`，显示脱敏产物入口 | 已测 |
| 编辑、有效 VLESS | 编辑 `HK` 后检查 | `clash-yaml=ok`；`generic-subs`／`sr-subs=warn`，诊断 `uri_partial_fields [smux]` | 已测 |
| 不同协议 | 新建并检查 SS、VMess、VLESS、Trojan、Hysteria2、WireGuard、HTTP、OpenVPN、Tailscale | 面板均正常出现并返回目标结果；未填必填字段时返回对应错误（如 SS password、Trojan password、OpenVPN ca、WireGuard private-key）；HTTP 三目标 `ok`；Tailscale 提示 `tailscale_auth_key_interactive_login`，generic／SR 为 `skip` | 已测 9 个协议；其余 10 个未做 UI 检查 |
| 未保存修改 | 检查成功后修改 host | 旧结果立即清空，面板回到只有标题和按钮 | 已测 |
| 检查进行中 | 延迟响应，点击检查 | 显示「正在检查…」，按钮 loading；折叠／展开折叠区不丢失进行中状态；响应回来后显示结果 | 已测 |
| 迟到响应 | 请求进行中修改 host | 旧响应不覆盖新草稿；但面板会停留在「正在检查…」直到下一次检查（见第 9 节） | 已测；属于现有行为，未修复 |
| 未应用 JSON 草稿 | 在 `reality-opts` 高级 JSON 中加入字段但不应用 | 检查按钮禁用，警告「存在未应用的 JSON 草稿…」；「定位草稿」聚焦 `reality-opts` 文本框；放弃后恢复可用 | 已测 |
| 未应用自定义值 | 将 `flow` 切到「其他（自定义）」并输入但不应用 | 检查按钮禁用，警告「存在未应用的自定义值或列表项草稿…」 | 已测 |
| 检查后折叠 | 检查成功后关闭高级数据 | 结果仍保留，重新展开一致；再次检查会保留旧结果直到新响应（代码层面未在请求开始时清空） | 已测折叠保留；重复检查旧结果保留由代码推断 |
| 编辑冲突 | 编辑 `HK` 后将隔离库修订号推进到 3，再检查 | HTTP 409；面板显示冲突文本，顶部同时出现冲突 Alert 与「重新加载」按钮 | 已测 |
| 检查后保存 | 有效新建节点检查通过后点击保存 | 保存成功，浮层关闭，节点出现在列表；检查没有阻断或写入检查结果 | 已测；临时节点已删除 |
| 移动端 | 375×785 新建 VLESS | 目标检查位于高级数据折叠区内；展开后按钮 100×44px，面板宽 317px；结果常驻使正文高度增加到 2228px | 已测 |

**协议抽测说明。** 上述 9 个协议是在真实 UI 中逐项选择并点击检查的结果；其中 OpenVPN／Tailscale 需要先滚动协议下拉的虚拟列表。剩余 Hysteria、TUIC、SOCKS5、Snell、AnyTLS、Mieru、MASQUE、SSH、ShadowQUIC、TrustTunnel 只通过 API 冒烟确认 `/check` 路由返回目标结构，未在 UI 中逐项检查；API 冒烟使用通用最小载荷，不能作为这些协议语义或完整表单的可视化验收。

**校验错误与检查的关系。** 空必填、host 为空、UUID 缺失等由服务端返回 `invalid_node_draft` 目标诊断，不会把检查按钮置灰；未应用的 JSON 草稿或自定义值／列表项草稿会让 `checkBlockedReason` 非空并禁用按钮。保存接口的 400 字段校验、对象 JSON 解析错误与保存校验定位属于保存流程，本轮没有把保存 400 与检查面板组合起来测试；已测的是空草稿和未应用草稿两类。检查面板不提供“校验失败”本地状态，错误以目标诊断形式呈现。

#### 7. 将目标检查移到折叠区外可能产生的影响

**7.1 功能、数据与接口**

- 仅移动模板位置时，不需要修改 `checkRequest`、`NodeCheckPanel` 的请求／响应、`POST /api/admin/nodes/check`、节点保存 DTO、凭据合并、扩展操作、数据库或锁文件。检查仍使用当前草稿，仍不写库，仍按 409 处理修订冲突。
- 移出不会改变目标检查与保存的分级：保存仍不要求检查，检查结果仍不参与保存判定；未应用 JSON／控件草稿仍同时阻止保存和检查。
- 用户选择的「结果默认折叠为摘要」不是纯模板移动，需要修改 `NodeCheckPanel` 的结果展示和本地状态；它不改变服务端响应和数据，但会改变用户看到结果的速度、表单高度、空结果／错误／进行中的呈现，以及测试。

**7.2 布局与滚动**

- 当前折叠区在表单最底部，目标检查还嵌在未知扩展之后，形成「页面底部 → 展开折叠区 → 继续向下」的两级门槛。移出后去掉展开步骤，但目标检查仍在长表单下部；独立内联位置不会自动把检查带到首屏。
- 桌面 1440×1000 下，新建 VLESS 的折叠区摘要已在 1184px；检查面板初始约 70px，三条结果约 338px。若结果常驻，表单会从 1060px 增加到约 1130px（仅面板）或 1398px（含结果）；用户选择的默认折叠摘要应显著减少结果态增量，但具体高度尚未实现和测量。
- 移动端 375×785 下，Drawer 正文高 651px、表单 1636px；检查面板初始 102px、三条结果 418px。移出后仍需要从顶部滚动到折叠区附近；结果摘要若常驻，会增加正文高度，但底部操作栏保持固定。
- 去掉 `node-advanced-data` 内层容器后，目标检查不再与未知扩展共享外边框；当前面板自带 `rounded-lg border p-3`，结果卡片又有内边框，移出后应检查是否出现重复标题／重复边框。若用 `FormSection` 包裹，需避免与 `NodeCheckPanel` 内部标题「目标检查」重复。
- 桌面 Modal 的底部操作栏不是固定悬浮，而是跟随内容；结果常驻会把「保存」推得更远。手机 Drawer 的底部操作栏在 Drawer 层固定，正文滚动增加但按钮始终可见。窄屏摘要应使用单行或可换行的状态文本，避免横向溢出。

**7.3 折叠状态与检查结果生命周期**

- 当前结果在折叠时隐藏但保留；移出后，结果默认可见或显示摘要。用户选择 B 后，需要明确：新检查开始时是否保留旧摘要、草稿变化时是否同时关闭展开状态、重新检查时是否自动展开。当前实现是「请求变化立即清空结果」、「迟到响应丢弃」，但没有「结果折叠」状态。
- 阻断、错误和 409 不属于结果详情，不能藏进折叠摘要；否则用户只看到「检查」按钮而不知道为何不可用。建议这些 Alert 与「定位草稿」保持在摘要外始终可见。
- 若摘要默认折叠，建议摘要至少显示目标数量与最差状态（例如三个目标的状态计数），并保留明确的「展开详情」入口；具体格式、是否自动展开错误、是否逐目标展开仍需实施时确认。
- 检查结果不持久化；移动位置不应改变这一点。关闭浮层、切换节点、保存成功都会销毁或重建面板，旧结果不能跨会话恢复。

**7.4 无障碍与键盘**

- 当前折叠区关闭时，目标检查按钮不在可见／可访问流程中；移出后少一层展开门槛，键盘用户可直接到达检查区域。
- 若结果改为默认折叠，需要使用原生 `<details>` 或带 `aria-expanded`／`aria-controls` 的按钮，并保证键盘可展开、焦点可见、状态摘要可被读屏读出。不要把状态摘要只做成颜色或纯视觉图标。
- 当前目标卡片是 `div`，没有独立标题层级；移动后若作为独立区域，建议补充可导航的区域标题，但这不是本轮已实现内容。
- 未执行真实读屏、键盘全流程或移动端辅助技术测试。

**7.5 测试与文档**

- 现有 `frontend/tests/nodes-view.spec.ts` 通过 `findComponent(NodeCheckPanel).props('blockedReason')` 检查阻断，不依赖 DOM 嵌套；`node-check-panel.spec.ts` 测试结果文本、迟到响应和预览背景。本轮定向运行 2 个文件 120 项通过。
- 没有测试锁定「高级数据与目标检查」标题或 `<details>` 嵌套；移动模板不会直接造成现有测试失败。但结果默认折叠是行为变化，`node-check-panel.spec.ts` 需要新增／调整断言：初始只显示摘要、展开后显示目标详情、草稿变化后摘要和展开状态复位、阻断／错误仍可见、移动端摘要不溢出。
- 现行 `Design4` D9／§3.1／§7.2、`docs/reports/Build/Build19.md` Step 3／Step 5 与 `docs/reports/Design/Design2-UI.md` §6.2 的历史分区描述需要后续同步；本轮按用户选择只在本文记录覆盖方向，不修改这些文档。

**7.6 未知扩展和说明文案**

- 移出后，原折叠区只剩未知扩展，标题「高级数据与目标检查」和说明「未知扩展摘要、局部 JSON 草稿与脱敏目标检查；默认折叠。」不再准确。尤其「局部 JSON」实际是各对象字段的编辑模式，不在该折叠区内；用户已确认标题改为「高级数据」、说明改为「未知扩展摘要；默认折叠。」
- 未知扩展的「新增／替换／清除」操作仍会使检查结果失效；目标检查移出后，用户更容易看到结果被清空，但面板没有说明是哪一个扩展操作触发失效。是否需要提示属于后续体验细节，本轮未决策。
- 未发现移动目标检查会改变扩展的加密保存、脱敏、诊断或输出排除逻辑。

#### 8. 用户确认的后续方向与待决策边界

用户于 2026-11-17 明确选择：

1. **位置：** 目标检查作为独立内联区域，放在「更多功能 / 高级参数」之后、「高级数据」折叠区之前；不使用顶部／底部操作栏加独立浮层的方案。
2. **原折叠区标题与说明：** 标题改为「高级数据」；说明改为「未知扩展摘要；默认折叠。」；目标检查不再属于该折叠区。
3. **结果展示：** 检查结果默认折叠为摘要，需要时展开；不保持当前三条结果直接常驻展开的状态。
4. **文档冲突处理：** 在 `Issue21.md` 记录本次要求覆盖 `Design4` D9／§3.1／§7.2 的方向；`Design4`、Build19、Design2-UI 等其他文档待后续同步，本轮不修改。

**按该方向实施时的候选步骤（仅记录，未编写代码）：**

- 在 `NodesView.vue` 中将 `<NodeCheckPanel>` 移出 `node-advanced-data`，放到「更多功能 / 高级参数」`</details>` 与「高级数据」`<details>` 之间；保持 `:request`、`:blocked-reason`、`@locate`、`@conflict` 绑定不变。
- 将原折叠区 `summary` 改为「高级数据」，说明改为「未知扩展摘要；默认折叠。」；未知扩展内容保持原样。
- 在 `NodeCheckPanel` 中增加结果摘要／展开状态：初始无结果时只显示按钮和进行中／阻断／错误；有结果时默认显示摘要，展开后显示现有逐目标详情与脱敏产物；请求变化时清空结果和展开状态；阻断／错误／409 始终在摘要之外可见。
- 不修改检查请求、后端检查、保存、凭据、扩展或数据库；保留「检查不写库、不保存节点」说明。
- 补充组件测试（默认折叠、展开、失效复位、阻断可见、移动端）和浏览器回归；同步 Design4／Build19／Design2-UI 的口径。

**仍需在实施或后续文档同步时确认：**

- 摘要的精确文案和布局：例如「3 个目标 · 1 ok · 2 warn」还是逐目标状态；是否显示最差状态颜色；长诊断时如何截断。
- 摘要展开是全局一个开关，还是逐目标展开；错误是否自动展开（用户只确认默认折叠，未确认错误自动展开）。
- 重新检查时旧摘要是否保留到新响应；草稿变化时是否关闭展开；这些状态需要与现有迟到响应行为一起定义。
- 目标检查独立区域是否使用 `FormSection` 包裹；若使用，需要处理与 `NodeCheckPanel` 内部「目标检查」标题的重复。
- Design4 D9 后续是改写为「独立区域」还是保留历史记录并在新版本变更记录中说明覆盖；Build19 为已归档历史文档，是否只加指向 Issue21 的说明。
- 摘要折叠是否影响 409 冲突时「重新加载」按钮的可见性；当前顶部冲突 Alert 已有「重新加载」，应避免重复。

#### 9. 尚未确认内容、未覆盖范围与测试限制

- 没有实施移动或结果摘要，未验证移动后的实际间距、边框、标题层级、滚动位置、焦点顺序、移动端摘要换行和结果展开高度；不能把本文候选方向表述为已实现。
- 没有 UI 覆盖全部 19 个协议：实际逐项打开检查的是 9 个协议；其余 10 个仅通过 API 冒烟确认 `/check` 路由，未验证各自条件字段、长表单和诊断展示。
- 未测试深色主题、横屏、平板、200% 缩放、Safari／Firefox、真实读屏软件、键盘完整 Tab 顺序和触控手势。
- 未测试网络失败、服务端 500、请求取消／AbortController、重复快速点击检查、检查进行中关闭浮层、保存失败后继续检查、409 后「重新加载」完整流程。
- 未在浏览器中逐项验证未知扩展「新增／替换／清除」使结果失效；该结论来自 `extensionOps` 参与 `checkRequest` 的代码关系，未做完整交互回归。
- 未测试所有目标状态组合、超长诊断、超长脱敏产物、预览展开后的高度，以及三个目标全部 `skip`／全部 `error` 的可视化。
- 未验证目标检查与真实客户端连接、外部服务、Docker 镜像、远端 CI、生产或预发布环境；检查本身不产生真实连接，但本文不能作为连接级验收。
- 发现一个与本问题位置无关的现有行为：检查请求进行中修改草稿时，`NodeCheckPanel` 会清空结果并递增序号，但 `checking` 不会复位，迟到响应被丢弃后面板会停留在「正在检查…」直到下一次检查。该行为在本轮通过延迟响应复现，未修复，也未纳入 R35-07 的移动范围；后续改动结果摘要／折叠时应一并回归，是否另立问题由用户决定。
- 未执行前端全量测试、`npm run build`、后端全量 `go build`／`go vet`／`go test`、Docker 或远端 CI；只执行了第 10 节列出的定向测试。
- 未修改 `Design4`、Build19、Design2-UI 或任何历史记录；本轮唯一纳入版本管理的持久化变更是 `Issue21.md`。

#### 10. 支持证据摘要

| 证据 | 内容 |
|------|------|
| `01-new-node-top.png`、`02-new-node-advanced-open.png` | 新建 VLESS 的默认折叠状态与展开后的未知扩展／目标检查顺序；目标检查在长表单底部 |
| `03-new-check-result.png` | 空 host 新建检查返回三个 `invalid_node_draft` 目标；结果卡片与脱敏产物入口 |
| `check-edit.log`、`edit-check` 证据 | 编辑 `HK` 检查：`clash-yaml=ok`，`generic-subs`／`sr-subs` 为 `uri_partial_fields [smux]`；折叠后结果保留 |
| `new-valid.log` | 有效新建 VLESS 检查三个目标均为 `ok` |
| `check-save.log`、`nodes.json`／数据库查询 | 检查后保存创建 `R35-07-SAVE` 成功；检查前后 `HK` 的 `edit_revision` 未被检查改变（3→3）；临时节点随后删除 |
| `05-blocked-json.png`、`14-custom-block.png` | 未应用 JSON 草稿和未应用自定义值草稿均禁用检查；「定位草稿」聚焦对应输入 |
| `check-flight.log` | 延迟响应下「正在检查…」与 loading；折叠／展开不丢失进行中状态 |
| `06-late-discard.png`、`check-late.log` | 请求进行中修改 host 后旧响应被丢弃；面板停留在「正在检查…」的现有行为 |
| `07-conflict.png`、`check-conflict.log` | 修订号冲突返回 409；面板与顶部 Alert 同时显示冲突文本 |
| `measure.json` | 桌面／移动端表单高度、检查面板高度（70／338、102／418）和 Drawer 尺寸 |
| `08-mobile-closed.png`、`09-mobile-open.png`、`10-mobile-panel.png` | 375×785 下折叠区、目标检查按钮 44px 高、面板宽度与滚动位置 |
| `protocol-results.json`、`13-openvpn.png`、`13-tailscale.png` | 9 个协议的实际 UI 检查结果；OpenVPN 缺 ca 错误、Tailscale 无 host 时的 warn／skip |
| `api19.log` | 19 个协议的 API 冒烟；通用最小载荷下均返回目标结构，仅证明路由与响应形态，不代表协议语义覆盖 |
| `npm test -- --run node-check-panel nodes-view` | 2 个测试文件、120 项通过（2026-11-17） |
| `go test ./internal/node -count=1` | 通过（2026-11-17） |
| `git status --porcelain` | 研究与测试前后工作区干净；文档编辑后仅 `Issue21.md` 有改动（本轮未提交） |

#### 11. 本轮未做

本轮只做测试、研究与文档记录。没有修改任何项目代码、配置、依赖或锁文件，没有修改工作区 `frontend/dist` 或 `backend/web/dist`，没有实施 UI 调整，没有将 R35-07 标记为已修复，没有操作生产、预发布、外部服务、真实账号或真实数据。隔离实例与临时数据均在 `/tmp/r35-07`；临时节点已删除；仓库 `backend/data` 未被触碰。

#### 12. 实施与验收（2026-11-17）

- **本轮决策。** 用户选择一个全局汇总，显示目标总数及通过、警告、错误、跳过的数量。重新检查时立即清空旧摘要和详情。用户同时确认本轮修复检查期间变更草稿后加载状态未复位的问题。
- **区域与可操作性。** 在新建和编辑表单中将 `NodeCheckPanel` 移到「更多功能 / 高级参数」（如有）之后、「高级数据」之前。面板本身提供可导航的标题。「高级数据」默认折叠，说明为「未知扩展摘要；默认折叠。」；未知扩展操作留在该区域。
- **结果与请求状态。** 成功响应默认只显示结果汇总。键盘或触控均可展开全部目标状态、诊断和脱敏产物入口；产物仍可单独展开。重新检查时结果区显示「正在检查…」。检查请求所用草稿变化后立即清空结果并结束旧加载，迟到响应不会覆盖新结果，也不会结束最新请求的加载。阻断、定位、请求失败和 409 提示均在结果折叠区外，409 继续复用表单顶部的「重新加载」。
- **本地自动化。** 定向测试 125 项通过，前端全量 51 个文件／476 项通过，前端生产构建及后端 `go build`、`go vet`、`go test` 通过。
- **隔离浏览器。** Chrome 1440×1000 验证新建和编辑、Tab 焦点与 Enter／Space 展开和收起、全通过和全错误摘要、编辑 `HK` 的混合状态、脱敏预览、重查与迟到响应、JSON 阻断与定位、修订冲突及重新加载。检查后保存可正常创建节点；检查本身未创建记录，且未推进 `HK` 的修订号。375×785 模拟手机的明暗主题下，检查按钮与汇总达到 44px 命中高度，摘要换行、展开内容与保存区均可操作。复现步骤与验证边界见 [Build38.md](../Build/Build38.md)。
- **文档。** Design4 v1.35 同步 D9、§3.1 和 §7.2。Build19 与 Design2-UI 在原章节保留历史记载并链接现行设计，AGENTS 同步构建索引。

### R35-08 Docker 挂载目录不可写导致误入应急模式

- 现象为删除数据库后重启容器进入应急模式。2026-11-18 在 Termius 的 KCS Aliyun 会话只读检查到 `/data` 为 root 持有的 `0755` bind mount，容器 `app` 的 UID/GID 为 `100:101`，数据库和 sidecar 均缺失，目录写权限检查失败。日志记录 `unable to open database file (14)` 和 `reason=db_corrupt`。
- 根因为镜像构建时对 `/data` 的属主设置被宿主机挂载覆盖，固定 `USER app` 无法创建数据库。后端把 SQLite 打开失败统一归入损坏救援。
- 2026-11-21 用户确认采用镜像内部入口准备目录、随后业务进程以非 root 身份运行。设计见 [Design6](../Design/Design6.md)，构建记录见 [Build39](../Build/Build39.md)。
- 影响包括 Docker 入口、实际进程身份、挂载目录自身属主和 Open／迁移的存储失败分支。普通用户目录采用其 UID/GID，root 目录调整目录自身属主。现有 SQLite 文件逐项检查，实际损坏仍进入应急恢复。
- 当前状态为已修复并通过本地验收。独立 Docker 回归覆盖 root／普通用户目录、旧库、重启、仅删除主库后初始化、显式用户、只读挂载、主库／WAL 权限冲突、实际损坏、手动应急、含空格 DATA_DIR、dev 模式和命名卷。后端全量测试／build／vet、前端 build 通过。PID 1 已核对为非 root，CapEff 为零。完整记录见 Build39。2026-11-21 用户授权以 `v4.1.5` 提交并推送构建，远端结果以对应 GitHub Actions 记录为准，KCS Aliyun 尚待使用新镜像验证。

### R35-09 Docker 默认身份在宿主机显示为 usbmux:lxd

- 现象为 v4.1.5 自动准备 root 拥有的目录后，宿主机显示属主为 `usbmux:lxd`。
- 根因为 Alpine 自动创建的镜像用户 `app` 数值身份为 `100:101`，宿主机按自己的账号表把同一组编号显示为系统账号。绑定挂载共享目录属主。
- 用户授权修复，并明确暂无数据，省去旧数据迁移验证。默认 app 固定为 `10001:10001`，空的旧 `100:101` 目录会在入口阶段纠正目录自身属主。普通用户目录与显式非 root 身份继续按实际指定身份运行。
- 当前状态为已修复并通过本地验收。镜像内 app、root 空目录及空的旧 `100:101` 目录均采用 `10001:10001`，普通用户目录和显式身份测试通过。Docker 场景回归、后端全量测试／build／vet、前端 build 通过。按用户确认省去旧数据迁移验证。2026-11-21 用户授权以 `v4.1.6` 提交并推送构建，远端结果以对应 GitHub Actions 记录为准，远端部署尚待执行。设计见 [Design6](../Design/Design6.md)，构建和验证见 [Build40](../Build/Build40.md)。

### R35-10 配置导入查表失败后仍提交覆盖

- **现象与复现。** Build41 Step 1 独立核验在临时 SQLite 库中模拟功能表查询失败。v1 同步导入返回成功，v2 异步任务进入成功终态。两条路径均覆盖了站点名称并留下原服务商绑定，违反配置覆盖事务的清理合同。
- **根因与影响。** v1/v2 事务清理 `provider_traffic_settings` 时借用了只返回布尔值的表检查。读取 `sqlite_master` 报错与查询确认缺表走同一分支，均跳过删除。RESET 和应急清库原已逐项处理查表错误。
- **修复与验证。** 导入与清库复用明确返回查询错误的事务查表函数。真实缺表的旧库继续兼容。新增回归通过 SQLite 连接级 SQL 长度限制使目录查询失败，修复前复现两种导入均误报成功。修复后 v1 返回错误，v2 任务标记失败，两者都保留原站点名称、服务商绑定密文、修订和开关。本地复验通过，命令与范围见 [Build41 Step 1](../Build/Build41.md#step-1--存储配置覆盖和旧库兼容)。

### R35-11 上游数值校验与错误分类偏离约定

- **现象与复现。** Build41 Step 2 独立核验中，KiwiVM 的近似倍率和下溢到零的非零错误码通过了成功判断，两家都接受了下溢到零的负流量。V.PS 读取停滞的 `404` 正文约 8 秒后给出通用服务商错误。两家拦截重定向时也给出通用服务商错误。
- **根因与影响。** 浮点转换发生在精确比较前，舍入改变了业务判断。V.PS 的错误正文读取层吞掉了读取错误。`CheckRedirect` 返回 `ErrUseLastResponse` 时，客户端收到首跳响应，旧分类器将该重定向归为普通非成功响应。影响 KiwiVM 查询、V.PS 续期与资源查询。
- **修复与验证。** 使用规范化十进制值校验业务码和倍率，拒绝会下溢成零或溢出的流量字段。V.PS 直接处理 `401` 与 `429`，并在其余 `4xx` 的限读过程中保留失败原因。两家识别首跳重定向并归为连接服务商失败。新增回归修复前复现、修复后通过，本地 TLS 复验确认正文超时和禁止跳转的行为。状态为已修复并通过本地复验，验证命令和真实平台验收边界见 [Build41 Step 2](../Build/Build41.md#step-2--kiwivm-与-vps-上游客户端)。

### R35-12 手动取消后仍可能发起续期后的资源查询

- **现象与复现。** Build41 Step 3 实施中的并发测试在临时 SQLite、合成凭据与阻塞模拟响应下复现：手动刷新已收到并解析新 Token 对后，调用方取消上下文；若取消仅通过 `context.AfterFunc` 异步传播到采集链，续期提交完成后的条件复核仍可能通过，随后发起一次资源查询。修复前先在取消后释放阻塞续期响应，资源请求计数为 1；修复后同类场景为 0，且新 Refresh Token 仍完成入库。
- **根因与影响。** 采集链只检查派生的链上下文，派生上下文对父请求取消的传播不是同步的；续期提交后的复核使用该派生上下文，没有直接读取原始手动请求上下文。影响手动刷新取消后的资源出站与结果发布，不影响已收到的新 Refresh Token 入库。
- **修复与验证。** 采集链增加手动请求上下文参数，在每次出站前、续期提交后、资源查询与结果发布前同步检查调用方请求上下文；续期提交继续使用独立的 5 秒 detached 上下文，取消不会丢失已解析的新 Token 对。回归 `TestCollectorVPSCommitSurvivesCancellation` 覆盖取消后资源请求为 0、新 Refresh Token 仍落库、结果不发布；`go test -race ./internal/providertraffic -count=3` 与重复运行通过。状态为已修复并通过本地复验；站内 HTTP 映射已由 Step 4 路由测试覆盖，真实服务商、Docker 与浏览器验收分别留给 Step 5～7，见 [Build41 Step 3](../Build/Build41.md#step-3--自动采集与轮换一致性)。

### R35-13 手动取消或设置变更后仍发布在途结果

- **现象与复现。** 独立核验使用临时 SQLite 和模拟上游，在结果通过资格复核后阻塞结果锁。此时取消手动请求仍写入 `ok`。关闭总开关同样写入 `ok` 并返回成功。改绑时新绑定读取不到旧结果，但本轮手动采集仍收到旧绑定的成功结果。
- **根因与影响。** SQLite 资格查询和内存写入之间允许其他配置事务提交，结果锁内也没有再次检查原始手动请求取消。手动入口确认的绑定修订没有传入采集链。
- **修复与验证。** 固定手动入口的绑定修订。短事务复核实时资格，持结果锁完成发布并检查取消，事务失败保留原结果。配置变更已提交时返回 `ErrDiscarded`。定向回归覆盖晚到取消、关停写入与发布顺序、改绑后零出站，并保留 R35-12 的续期入库验证。状态为已修复并通过本地验证，站内 `409` 映射已由 Step 4 路由测试核验。 [Build41 Step 3](../Build/Build41.md#step-3--自动采集与轮换一致性) · [Build41 Step 4](../Build/Build41.md#step-4--站内接口与生命周期)

### R35-14 总开关关闭后重新开启可能漏掉首轮

- **现象与复现。** 两笔开关写入均已提交，再执行各自的提交后通知。两次通知均观察到最终的开启状态，立即采集次数为零，直到下一固定周期才采集。
- **根因与影响。** 调度依据通知时的最新快照与上次快照，关闭发生在两次观察之间时看不到真实的开启变化。
- **修复与验证。** 设置业务入口串行完成本地提交和调度交接。使用同一写事务内的前后状态安排采集，失败或同值保存无触发，提交后请求取消仍完成交接。定向回归验证重开在周期前触发一次，已有回归覆盖同值和冲突、单家触发、两家独立及忙碌时合并。状态为已修复并通过本地验证。 [Build41 Step 3](../Build/Build41.md#step-3--自动采集与轮换一致性)

### R35-15 V.PS 出站前复核期间 Access Token 到期

- **现象与复现。** V.PS 选择了当时尚有效的缓存 Token，后续资格复核等待 SQLite 连接时 Token 到期。连接释放后仍以旧 Token 查询资源，本轮报告上游鉴权失败。
- **根因与影响。** 缓存有效性在数据库复核之前判定，数据库等待期间经过的时间没有计入。
- **修复与验证。** 先复核出站资格，再在缓存锁内判断到期。到期则本轮续期并提交新 Refresh Token 后查询资源。定向回归将时钟推进到到期时刻，断言各发出一次续期与资源查询，资源端收到新 Access Token 前新 Refresh Token 已入库。状态为已修复并通过本地验证。 [Build41 Step 3](../Build/Build41.md#step-3--自动采集与轮换一致性)

### R35-16 模式切换先提交后设置写入仍成功

- **现象与根因。** 独立核验在 HTTP 入口检查通过后、请求体读完前切入高级模式。原请求返回 `200` 并落库。设置事务缺少 Setup 和基础模式复核。补充实验确认修改绑定和解绑也受影响，Setup 状态变化可复现同样的问题。
- **修复与验证。** 共用设置写事务复核 Setup 和模式，拒绝时返回 `403` 且不交接调度。三种写入分别与两类模式变化交错，均保持数据库设置原值。已修复并通过本地复验。 [Build41 Step 4](../Build/Build41.md#step-4--站内接口与生命周期)

### R35-17 设置响应将新实例的采集结果挂在旧修订

- **现象与根因。** 并发换绑时，设置响应先拿到旧实例与修订，单独查询结果时重新读取了新绑定。模拟上游先完成新实例采集后，响应错误地在旧绑定中显示新实例余量。
- **修复与验证。** 按同一响应内的绑定修订读取内存结果，无匹配时返回 `null`。真实采集器在模拟上游环境验证新实例成功后旧修订无结果，路由回归覆盖设置读取与写入响应。已修复并通过本地复验。 [Build41 Step 4](../Build/Build41.md#step-4--站内接口与生命周期)

### R35-18 设置接口接受了含多余内容或超限的请求体

- **现象与根因。** JSON 解码读到首个对象便开始写入。含第二个对象的请求、合法对象后带超限尾部的未知长度请求，均返回 `200` 并执行写入。
- **修复与验证。** 以固定缓冲区校验完整对象后只剩 JSON 空白，并沿用全局体积限制。回归确认恰好到上限的合法请求通过，多余内容返回 `400`，超限返回 `413`，失败响应带 `no-store` 且写入次数为零。已修复并通过本地复验。 [Build41 Step 4](../Build/Build41.md#step-4--站内接口与生命周期)

### R35-19 服务正常关停时在途手动采集返回 500

- **现象与根因。** 模拟上游正在处理时停止正常服务，HTTP 请求收到 `500`。停机信号和采集链结果有两个出口，分别可能交付「采集服务不可用」或上下文取消。
- **修复与验证。** 已受理的手动采集使用工作链最终结果。服务退出导致丢弃时映射为 `409`，保持客户端取消与本地故障的独立处理。直接停止采集器与取消服务上下文均通过。回环 HTTP 复验连续三次返回 `409`，无结果发布且服务正常退出。已修复并通过本地复验。 [Build41 Step 4](../Build/Build41.md#step-4--站内接口与生命周期)

### R35-20 未保存草稿使用了新绑定修订

- **现象与根因。** 页面 A 为旧绑定输入草稿，页面 B 改绑后，页面 A 保存总开关拿到新修订，同时仍保留旧草稿。修复前的模拟用例捕获到旧草稿携带新修订提交。草稿合并仅保留输入，提交却读取最新已保存修订，写入事务无法识别该草稿来自旧绑定。
- **修复与验证。** 草稿记录自身基准，修订不一致时阻止本家保存及立即采集，直至管理员显式采用最新绑定并清除本家草稿。前端回归覆盖不同实例改绑、同实例换证及其他页面先完成首次绑定，确认旧草稿保留、无写请求、显式重置后才提交新修订。 [Build41 Step 5](../Build/Build41.md#step-5--管理设置与首页展示)

### R35-21 旧解绑确认可能删除新绑定

- **现象与根因。** 切回基础模式时重读尚未完成，旧表单可以打开解绑确认。重读返回新绑定后，确认回调临时读取最新修订，修复前的模拟用例捕获到 DELETE 携带新修订。
- **修复与验证。** 确认框显示实例并固定打开时的修订。模式变化或读到新修订时使旧确认失效，切回基础模式先完成重读。确认后仍由后端按提交修订校验。前端回归验证重读期间无解绑入口、旧确认无请求、重新确认为新实例时才提交新修订。 [Build41 Step 5](../Build/Build41.md#step-5--管理设置与首页展示)

### R35-22 旧页面手动采集了当前新绑定

- **现象与根因。** 原 POST 只提交平台，服务端采用请求进入时的当前绑定。另一页先改绑时，旧页面可收到新实例结果。修复前的模拟用例证实旧实例行会显示新实例的数值。对 V.PS，该请求还可能为新实例续期。
- **修复与验证。** 用户授权立即采集改为提交所见的 `revision`。采集器在启动请求链前比对；旧修订返回 `409`。回归确认两家均零出站、无结果发布，合法修订仍支持原有发布和 V.PS 同修订轮换，前端也拦截与当前绑定不符的迟到结果。HTTP 回归覆盖完整请求体、大小限制、`no-store` 及旧修订映射。 [Build41 Step 5](../Build/Build41.md#step-5--管理设置与首页展示)

### R35-23 旧版接口接受主机的过期修订

- **现象与根因。** 旧版保存和解绑校验绑定修订。用新版接口停用或改名后，旧版请求仍可重新启用或删除主机，临时用例复现了两种写入。旧版立即采集也使用同一绑定修订。
- **修复与验证。** `providers[].revision` 使用编辑修订，保存、解绑和立即采集统一按编辑修订校验。站内 HTTP 回归验证过期保存与解绑返回 `409`，重读后可以保存和解绑。采集回归验证旧修订被拒绝，当前修订可采集。

### R35-24 手动采集排队后遗漏编辑修订复核

- **现象与根因。** 同平台的槽位用满时，旧页面请求在修改主机名称和折半开关后仍能出站。采集入口校验编辑修订，进入工作链后只携带绑定修订。
- **修复与验证。** 手动链在平台槽位和账号续期锁等待后复核两项修订，发布事务也复核。回归验证排队期间编辑时，本轮丢弃且无新增上游请求。资源查询已经发出时，本轮结果丢弃，最新设置仍可按原采集时间展示上一份样本。

### R35-25 V.PS 单台主机停用后同账号重复续期

- **现象与根因。** 第一台主机在续期途中停用，Refresh Token 已落库，Access Token 却因该主机的资格检查未进入账号缓存。第二台主机随后重新续期。
- **修复与验证。** 在账号锁内按账号修订落库并缓存令牌，资源查询由调用方复核主机与请求状态。并发回归验证停用第一台后，两台合计只执行一次续期及一次资源查询，第二台使用已持久化的轮换结果。

### R35-26 配置导入确认文案与前端回归断言不一致

- **现象与根因。** Build42 的管理端提示已改成「所有主机绑定和凭据会被清除」。旧测试仍断言「平台绑定与凭据会被清除」，独立复验前端全量 525 通过、1 失败。
- **修复与验证。** 更新现有断言，继续覆盖清除全部主机绑定和凭据以及重新绑定的提示。前端全量 526 项测试和生产构建通过。

## 二、记录格式

每项记录已观察到的现象、操作步骤和必要的环境信息；根因结论必须附可复现证据和对照实验，无法确认的内容标明待确认。修复方案如未实施，明确标注为候选／待决策，不把候选方案当作已确认方案。

## 三、变更记录

| 日期 | 说明 |
|------|------|
| 2026-11-24 | 核对 R35-01～R35-26 的修复和验证记录，全部问题闭环，本文归档。 |
| 2026-11-24 | 完成 R35-23～R35-26，修复管理接口修订、采集复核、V.PS 共享轮换与导入确认测试，见 Design7 v1.9 和 Build42 Step 1。 |
| 2026-11-24 | 完成 R35-20～R35-22，修正草稿、确认和手动采集的绑定修订，见 Design7 v1.7 与 Build41 Step 5。 |
| 2026-11-24 | 完成 R35-16～R35-19 设置写事务、结果修订、完整请求体和手动采集关停的修复与本地复验，见 Build41 Step 4。 |
| 2026-11-24 | 完成 R35-11 上游数值校验、V.PS 正文超时和拒绝重定向分类的修复与本地复验，见 Build41 Step 2。 |
| 2026-11-24 | 完成 R35-12 手动取消后的同步上下文检查修复与本地复验，见 Build41 Step 3。 |
| 2026-11-24 | 完成 R35-13～R35-15 结果发布、设置调度和缓存到期三处时序修复与本地验证，见 Build41 Step 3。 |
| 2026-11-23 | 完成 R35-10 配置导入查表失败时整体回滚的修复和本地复验，见 Build41 Step 1。 |
| 2026-11-21 | 完成 R35-09 默认 UID/GID 固定为 `10001:10001`，空的旧默认身份目录可自动纠正。本地 Docker 回归与联合门禁通过，见 Design6 v1.1／Build40。 |
| 2026-11-21 | 完成 R35-08 镜像内部目录准备、降权启动与存储错误分类修复，Docker 场景回归和联合门禁通过，设计见 Design6，验收见 Build39。 |
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
| 2026-11-16 | R35-02 按确认口径修复 Clash／SR 最终规则门槛与预览状态，并修复 R35-03 关联的无效池 ID 和空池 URL 数组。影响排查、验收和剩余边界见 R35-02 第 12 节及 Build33。 |
| 2026-11-16 | R35-03 确认原始 8080 旧构建并完成本地替换与浏览器复验。修复来源快照丢失、池列表竞态和旧预览失效时序，见 R35-03 第 13 节及 Build34。 |
| 2026-11-16 | R35-04 修复 VLESS／VMess 手动节点的 Clash UDP 缺省输出，通过本地验收，见第 13 节及 Build35。 |
| 2026-11-17 | R35-02 补充 Clash 装配前的空素材提示，提供添加素材和继续装配两条路径。 |
| 2026-11-17 | 记录 R35-06：节点创建和编辑界面无需显示 Mihomo 版本号。 |
| 2026-11-17 | 记录 R35-07：目标检查应移至高级数据区域外部。 |
| 2026-11-17 | 按授权完成 R35-05 研究：确认节点对象字段使用双按钮、装配 Clash 分区使用 Switch，两者由独立组件和不同提交时机实现；确认双按钮来自 2026-09-03 R27-05 的明确改动，并记录 `Design2-UI`／`Design4`／`Issue13`／`Build21` 的新旧口径差异。实测节点有效／无效 JSON、草稿、应用／放弃、保存／检查拦截、父子草稿阻断、取消／重开、持久化，以及装配开关的即时写入、无效 JSON 无法关闭、跨分区刷新覆盖和键盘／主题／手机表现；从 schema 统计 19 协议 49 个对象字段并打开 6 个代表性字段。记录用户确认的后续方向（只换外观、保留应用／放弃与草稿、覆盖所有对象字段、固定「高级 JSON」文案）及实现影响、未确认边界和回归建议；本轮仅测试、研究与文档记录，未修改 UI、项目代码、配置、依赖或锁文件，未将问题标记为已修复。 |
| 2026-11-17 | 按授权实施 R35-05：节点对象字段的「结构化编辑／高级 JSON」双按钮改为固定「高级 JSON」开关，覆盖对象、嵌套对象、map 与 list；保留应用／放弃、草稿、父子阻断、保存／检查拦截和无效 JSON 退出限制；同步定向/全量测试、隔离 Chrome 验证、`Design4` §3.3 v1.33 与 `Build36`。 |
| 2026-11-17 | 按授权完成 R35-06 研究：确认 Mihomo 版本号来自协议注册表 `OptionItem.Verified`，经 `/api/admin/nodes/protocols` 下发并由 `EditableCombobox` 的 `verifiedLabel()` 渲染；实测新建/编辑下 Shadowsocks、VMess、VLESS、Trojan 四个协议、21 个字段、83 个推荐项，逐协议检查 19 个协议，区分选中值、下拉、`alpn` 列表和 `project-unknown` 的实际表现；确认 `verified` 不参与兼容性判断或选项校验，目标检查面板不显示版本；记录用户确认的后续方向（全部受影响字段、移除全部 `verified` 文案并保留分组、视觉与无障碍文本均去掉版本、保留后端 metadata）及候选影响、回归范围和未确认边界；本轮仅测试、研究与文档记录，未实施界面调整，未修改代码、配置、依赖或锁文件。 |
| 2026-11-17 | 在当前 HEAD `70f388b` 上独立复核 R35-06：重新构建隔离实例，实测 19 个协议、21 个协议字段、83 个推荐项，新建/编辑、SS 2022 `project-unknown`、移动端、无障碍 `label` 与目标检查；确认版本号来源及“不参与选项校验、兼容性判断或目标检查”的结论与第 1～11 节一致；补充本地手工构建需先用 `frontend/dist` 覆盖 `backend/web/dist` 的测试前提。本轮仍未实施界面调整，仅更新本文档。 |
| 2026-11-17 | 完成 R35-06 修复与本地验收。已选值、展开选项和无障碍文本省略验证来源并保留分组，协议目录继续下发证据元数据。同步 Design4 v1.34 与 Build37，实施结果见 R35-06 第 13 节。 |
| 2026-11-17 | 按授权完成 R35-07 研究：确认目标检查位于 `NodesView.vue` 的「高级数据与目标检查」`<details>` 内、未知扩展之后，`NodeCheckPanel` 与折叠区无组件／生命周期依赖；实测新建与编辑、9 个协议 UI、未应用 JSON／自定义值阻断、检查进行中与迟到响应、折叠后结果保留、409 冲突、检查后保存、桌面与 375px 布局；确认检查不写库、不阻断保存，移动仅需模板位置与标题／说明调整，结果摘要会新增组件状态。记录用户确认方向（独立内联区位于「更多功能 / 高级参数」之后、「高级数据」之前；标题改「高级数据」、说明改「未知扩展摘要；默认折叠。」；结果默认折叠摘要、需要时展开；覆盖 `Design4` D9／§3.1／§7.2 的方向，其他文档待同步）及影响、未确认边界和回归范围；本轮仅测试、研究与文档记录，未修改代码、配置、依赖或锁文件，未实施界面调整，未将问题标记为已修复。 |
| 2026-11-17 | 完成 R35-07 实施与本地验收，目标检查移为独立内联区域，成功结果默认显示单条汇总。重新检查立即清空旧结果，草稿变更时复位旧加载状态。同步 Design4 v1.35、Build38、AGENTS 及历史文档入口。 |
