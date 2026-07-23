# Phase 2 — 管理面板移动端适配

> 前置: Phase 2 全部 10 个块（10A-10J）已完成。

## 总览

| Phase | 内容 | 涉及文件数 | 状态 |
|-------|------|-----------|------|
| 2.1 | 移动端表格 UX 改进（ActionMenu + 列隐藏） | 12 | ✅ 已完成 |
| 2.2 | 卡片化重构 + Dialog z-index 修复 | 14 | ✅ 已完成 |
| 2.3 | Dialog 移动端宽度自适应 | 13 | ✅ 已完成 |
| 2.4 | 全项目 UI/UX 体验修复（反馈/动画/边界） | 10 + 1 新 | ✅ 已完成 |
| 2.5 | 导航入口 + 表单 ID 清理 + 规则页风格 + 公告栏 | 11（含后端 4） | ✅ 已完成 |
| 2.6 | 全局 UI 比例调整（字号/标题） | 15 | ✅ 已完成 |

---

# ✅ 已完成

## Phase 2.1 — 移动端表格 UX 改进

> **问题**: 管理面板表格在手机上超出页面空间（操作列 260-340px）。
> **方案**: ActionMenu 下拉菜单 + 非关键列移动端隐藏 + 表格容器 overflow-x-auto。

### 完成状态（2026-07-22）

| 块 | 内容 | 状态 |
|----|------|------|
| 11A | ActionMenu.vue + useIsMobile.js + Manage.vue min-w-0 | ✅ |
| 11B | SubList / ShareList / UserManage / RulesManage / PlatformManage 接入 | ✅ |
| 11C | SubVersions / ShareVersions / RuleVersions（ActionMenu+列隐藏）/ Logs（仅列隐藏，无操作按钮） | ✅ |
| 11D | 编译验证 + 残留 CSS 清理 | ✅ |

### 改动汇总

| 类型 | 文件 |
|------|------|
| 新增 | `ActionMenu.vue`, `useIsMobile.js` |
| 修改 | SubList, ShareList, UserManage, RulesManage, PlatformManage, SubVersions, ShareVersions, RuleVersions, Logs, Manage |
| 清理 | 6 个文件残留 scoped CSS |

### 关键设计决策

- **ActionMenu.vue**: 桌面端(md+) 显示全部按钮，移动端(<md) 收起为「...」下拉菜单，clickoutside 自动关闭；操作列从 260-340px → ~80px
- **列隐藏**: 用 `v-if="!isMobile"` 条件渲染（非 CSS 隐藏），确保 el-table 列宽计算正确
- **容器约束**: Manage.vue main 加 `min-w-0` 防 flex 溢出，表格外包 `overflow-x-auto`
- **断点**: 统一 768px（`useIsMobile`）

> **审阅发现（2026-07-22）**: 4 个表格保留页（Logs/SubVersions/ShareVersions/RuleVersions）的 `el-table` 实际**未包裹 `overflow-x-auto` 容器**。由于 Element Plus 内置横向滚动 + Manage.vue `min-w-0` 约束，溢出问题未实际暴露。在 Phase 2.4 实施时顺手补上 4 处 `<div class="w-full overflow-x-auto">` 包裹。

---

## Phase 2.2 — 管理面板卡片化重构 + Dialog z-index 修复

> **问题 A**: el-dialog 弹出层被 el-table `fixed="right"` 列遮挡（z-index 层叠上下文冲突）。
> **问题 B**: 5 个列表页 el-table 在移动端即使有 ActionMenu 仍显拥挤。
> **方案**: 全部 el-dialog 加 `append-to-body`；5 个列表页改为 Home 风格卡片 grid。

### 完成状态（2026-07-22）

| 块 | 内容 | 状态 |
|----|------|------|
| 12A | ConfirmDialog / UploadModal / OIDCSwitchDialog 加 append-to-body | ✅ |
| 12B | SubList 卡片化 | ✅ |
| 12C | ShareList 卡片化 | ✅ |
| 12D | UserManage 卡片化 | ✅ |
| 12E | RulesManage 卡片化 | ✅ |
| 12F | PlatformManage 卡片化 | ✅ |
| 12G | SubVersions / ShareVersions / RuleVersions / Logs 加 append-to-body | ✅ |
| 12H | OIDCConfig 清理残留 CSS + Home.vue 加 append-to-body | ✅ |
| 12I | 全量编译验证 | ✅ |

### 代码审查确认（grep 验证）

| 检查项 | 结果 |
|--------|------|
| `append-to-body` | ✅ 14 处全部就位（3 组件 + 11 views） |
| `el-table` 残留 | ✅ 仅 4 个表格保留页（SubVersions / ShareVersions / RuleVersions / Logs） |
| 卡片布局 | ✅ 5 个列表页均采用 `grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5` |

### 改动汇总

| 类型 | 文件 |
|------|------|
| 组件修改 | `ConfirmDialog.vue`, `UploadModal.vue`, `OIDCSwitchDialog.vue`（加 append-to-body） |
| 列表页 → 卡片 | SubList, ShareList, UserManage, RulesManage, PlatformManage（去 el-table/ActionMenu/useIsMobile，改卡片 grid + 内联按钮） |
| 表格保留页 | SubVersions, ShareVersions, RuleVersions（append-to-body + el-table + ActionMenu）；Logs（append-to-body + el-table + 列隐藏，无操作按钮故无 ActionMenu） |
| 其他 | Home.vue（加 append-to-body）, OIDCConfig.vue（清理残留 CSS） |

### 页面布局现状

| 分类 | 页面 | 布局方式 |
|------|------|---------|
| 卡片 grid | SubList, ShareList, UserManage, RulesManage, PlatformManage | `grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3`，内联 `flex flex-wrap gap-1` 按钮 |
| 表格 | Logs, SubVersions, ShareVersions, RuleVersions | el-table + ActionMenu（版本页）/ 列隐藏（Logs） |
| 已有卡片 | OIDCConfig | 仅清理残留 CSS |
| 不受影响 | Home, Login, Setup, Rules(用户), Manage(布局壳) | — |

### 关键设计决策

- **append-to-body**: dialog 挂载到 `<body>` 脱离表格层叠上下文，解决被 sticky 列遮挡问题
- **卡片布局**: 遵循 Home.vue 模式，卡片内操作按钮 flex-wrap 自然换行，无需 ActionMenu
- **ActionMenu 保留**: 三个版本页（SubVersions/ShareVersions/RuleVersions）仍用 el-table + ActionMenu；Logs 无操作按钮仅用 useIsMobile 列隐藏
- **z-index 统一**: 所有 el-dialog 均 append-to-body，z-index 由 Element Plus 统一管理

### 残留问题

所有 el-dialog 仍使用固定 px 宽度（400px–640px），手机端（375px 视口基准）全部溢出。→ 由 Phase 2.3 解决。

---

# ❌ 待实施

## Phase 2.3 — 管理面板 Dialog 移动端宽度自适应

> **触发**: Phase 2.2 完成后，所有 el-dialog 已正确挂载到 body（z-index 修复 + 卡片化），但弹窗宽度仍为固定 px 值，在手机端溢出渲染区域。
> **目标**: 全部 15 处 el-dialog 移动端宽度自适应；预览弹窗全屏处理。

### 前置检查

| 检查项 | 结果 |
|--------|------|
| `append-to-body` | ✅ 14 处全部就位 |
| 列表页卡片化 | ✅ 5 个列表页已无 el-table |
| 表格保留页 | ✅ 4 个保留页 el-table + ActionMenu 正常 |
| `useDialogWidth.js` | ❌ 不存在，需新建（13A） |

### 问题: Dialog 固定 px 宽度溢出

以 iPhone SE（375px 视口）为基准，全部 15 处 el-dialog 均溢出：

| # | 文件 | 用途 | 当前 width | 溢出比 | 严重度 |
|---|------|------|-----------|--------|--------|
| 1 | `ConfirmDialog.vue` | 确认操作 | 420px | 112% | ⚠️ 中 |
| 2 | `OIDCSwitchDialog.vue` | 切换 OIDC 提供商 | 400px | 107% | ⚠️ 中 |
| 3 | `UploadModal.vue` | 上传版本（文件/文本） | 520px | 139% | 🔴 高 |
| 4 | `SubList.vue` | 创建/编辑订阅 | 480px | 128% | 🔴 高 |
| 5 | `ShareList.vue` | 创建分享订阅 | 520px | 139% | 🔴 高 |
| 6 | `UserManage.vue` | 编辑用户 | 460px | 123% | 🔴 高 |
| 7 | `UserManage.vue` | 上传自定义订阅 | 480px | 128% | 🔴 高 |
| 8 | `UserManage.vue` | 删除自定义订阅 | 440px | 117% | ⚠️ 中 |
| 9 | `RulesManage.vue` | 创建规则 | 520px | 139% | 🔴 高 |
| 10 | `PlatformManage.vue` | 创建/编辑平台 | 540px | 144% | 🔴 高 |
| 11 | `SubVersions.vue` | 版本预览（代码） | 640px | 171% | 🔴 严重 |
| 12 | `ShareVersions.vue` | 版本预览（代码） | 640px | 171% | 🔴 严重 |
| 13 | `RuleVersions.vue` | 版本预览（代码） | 640px | 171% | 🔴 严重 |
| 14 | `Home.vue` | 复制订阅链接 | 500px | 133% | 🔴 高 |

（#15 Setup.vue 使用 OIDCSwitchDialog 组件，已在 #2 覆盖）

### 设计决策（已确认，2026-07-22）

| 决策 | 选项 | 结论 |
|------|------|------|
| UploadModal 移动端处理 | A: 90%宽度 / **B: fullscreen 全屏** | **B** — 内含 el-upload 拖拽区+textarea，338px 偏窄 |
| PlatformManage 纵向溢出 | A: 等实测 / **B: 主动加滚动** | **B** — 7 个表单项~700px，iPhone SE 667px 必溢出 |
| 预览 `<pre>` 全屏高度 | A: 保持 max-h-96 / **B: 适配全屏高度** | **B** — 全屏下 384px 仅用 ~57% 高度，浪费空间 |

### 修复策略

#### 策略 A: useDialogWidth.js Composable（10 处弹窗）

新建 `@/composables/useDialogWidth.js`，复用已有 `useIsMobile`（断点 768px）：

- 移动端 (<768px): `"90%"`
- 桌面端 (≥768px): 传入的 `desktopWidth`（保持原 px 值）

**优点**: 单一入口、响应式（窗口 resize 自动切换）、无需 `!important`、每个弹窗保持独立桌面端宽度。

**适用**: ConfirmDialog, OIDCSwitchDialog, SubList, ShareList, UserManage(×3), RulesManage, PlatformManage, Home — 共 10 处。

#### 策略 B: 弹窗全屏（4 处弹窗，UploadModal + 3 个预览弹窗）

| 文件 | Dialog | 原因 |
|------|--------|------|
| `UploadModal.vue` | 上传版本（文件/文本） | el-upload 拖拽区 + textarea，520px→90%(~338px) 偏窄 |
| `SubVersions.vue` | 版本预览 | 640px 代码展示，移动端 90% 仅 ~338px |
| `ShareVersions.vue` | 版本预览 | 同上 |
| `RuleVersions.vue` | 版本预览 | 同上 |

这三个页面和 UploadModal 采用**:fullscreen="isMobile"** 全屏模式。UploadModal 需新增 `useIsMobile` 引入；三个版本页已有。

```vue
:width="isMobile ? '90%' : '640px'"
:fullscreen="isMobile"
```

#### 策略 C: PlatformManage 弹窗纵向滚动（已确认）

PlatformManage 有 7 个 el-form-item（含多行 textarea 和提示文字），估算总高度 ~700px，iPhone SE 视口 667px 可能溢出。主动在弹窗 body 外套 `max-h-[calc(100vh-200px)] overflow-y-auto`（仅移动端，通过 `isMobile` 条件判断）。`100vh-200px` 扣除 dialog header+footer+padding，最大化利用弹窗空间。

#### 策略 D: 预览弹窗 `<pre>` 高度适配（已确认）

三个版本预览弹窗（SubVersions / ShareVersions / RuleVersions）当前 `<pre>` 使用 `max-h-96`（384px）。全屏后可用高度约 550px（扣除 header/footer），384px 浪费空间。移动端全屏时使用 `max-h-[calc(100vh-120px)]`（扣除 dialog header+footer），桌面端保持 `max-h-96`。

#### 策略 E: 不采用全局 CSS

全局 CSS 需要 `!important` 覆盖 Element Plus 内联样式，且无法区分不同弹窗类型。采用策略 A+B+C+D 逐文件精确控制。

### 实施计划

| 块 | 内容 | 涉及文件 | 依赖 |
|----|------|---------|------|
| 13A | 新建 `useDialogWidth.js` composable | 1 新文件 | — |
| 13B | 共享组件接入（ConfirmDialog / OIDCSwitchDialog 用 useDialogWidth，UploadModal 用 fullscreen） | 3 组件 | 13A |
| 13C | 卡片列表页弹窗接入（SubList / ShareList / UserManage / RulesManage / PlatformManage） | 5 views | 13A |
| 13D | 版本预览弹窗全屏（SubVersions / ShareVersions / RuleVersions） | 3 views | — |
| 13E | Home.vue 弹窗接入 | 1 view | 13A |
| 13F | 编译验证 `npm run build` | 全部 | 13B–13E |

依赖链: 13A → 13B/13C/13E（可并行）；13D 独立（复用已有 useIsMobile）→ 13F

### 文件级改动明细

#### 13A — useDialogWidth.js（新建 1 文件）

`frontend/src/composables/useDialogWidth.js`:
```js
import { computed } from 'vue'
import { useIsMobile } from './useIsMobile'

export function useDialogWidth(desktopWidth = '520px') {
  const isMobile = useIsMobile()
  return computed(() => isMobile.value ? '90%' : desktopWidth)
}
```

#### 13B — 共享组件（3 组件）

| 文件 | 策略 | 改动 |
|------|------|------|
| `ConfirmDialog.vue` | A (useDialogWidth) | `width="420px"` → `:width="dialogWidth"`（desktopWidth=`'420px'`） |
| `OIDCSwitchDialog.vue` | A (useDialogWidth) | `width="400px"` → `:width="dialogWidth"`（desktopWidth=`'400px'`） |
| `UploadModal.vue` | B (fullscreen) | `width="520px"` → `:width="isMobile ? '90%' : '520px'" :fullscreen="isMobile"`，新增 `import { useIsMobile }` |

改动模式（策略 A）：`<script setup>` 加 `import { useDialogWidth } from '@/composables/useDialogWidth'` + `const dialogWidth = useDialogWidth('xxxpx')`

改动模式（策略 B）：`<script setup>` 加 `import { useIsMobile } from '@/composables/useIsMobile'` + `const isMobile = useIsMobile()`

#### 13C — 卡片列表页弹窗（5 views，7 处 el-dialog）

| 文件 | Dialog | 当前 width | 变量名 | desktopWidth |
|------|--------|-----------|--------|-------------|
| `SubList.vue` | 创建/编辑 | 480px | `dialogWidth` | `'480px'` |
| `ShareList.vue` | 创建 | 520px | `dialogWidth` | `'520px'` |
| `UserManage.vue` | 编辑用户 | 460px | `editDialogWidth` | `'460px'` |
| `UserManage.vue` | 上传自定义 | 480px | `uploadDialogWidth` | `'480px'` |
| `UserManage.vue` | 删除自定义 | 440px | `deleteDialogWidth` | `'440px'` |
| `RulesManage.vue` | 创建规则 | 520px | `dialogWidth` | `'520px'` |
| `PlatformManage.vue` | 创建/编辑 | 540px | `dialogWidth` | `'540px'` |

#### 13D — 弹窗全屏（4 处：UploadModal + 3 个版本预览）

| 文件 | 当前 | 改后 | isMobile 来源 |
|------|------|------|-------------|
| `UploadModal.vue` | `width="520px"` | `:width="isMobile ? '90%' : '520px'" :fullscreen="isMobile"` | 新增 `useIsMobile` |
| `SubVersions.vue` | `width="640px"` | `:width="isMobile ? '90%' : '640px'" :fullscreen="isMobile"` | 已有 |
| `ShareVersions.vue` | `width="640px"` | 同上 | 已有 |
| `RuleVersions.vue` | `width="640px"` | 同上 | 已有 |

> SubVersions / ShareVersions / RuleVersions 已有 `const isMobile = useIsMobile()`（用于表格列隐藏 + ActionMenu），无需额外引入。UploadModal 需新增。

#### 13D-extra — PlatformManage 弹窗 body 纵向滚动 + 预览 `<pre>` 高度

**PlatformManage**（7 个 el-form-item，移动端可能溢出）:
- 在弹窗的 `<el-form>` 外套一层 `<div :class="{ 'max-h-[calc(100vh-200px)] overflow-y-auto': isMobile }">`
- PlatformManage 需新增 `import { useIsMobile }` + `const isMobile = useIsMobile()`

**三个版本预览页** `<pre>` 高度适配:
```vue
<pre class="... overflow-auto max-h-96 max-md:max-h-[calc(100vh-120px)] ...">{{ previewContent }}</pre>
```
- `max-md:` 仅在 <768px（全屏触发断点）时生效，扣除 header+footer(~120px)
- 桌面端保持 `max-h-96`（384px），弹窗非全屏，高度合理
- 纯 template class 替换，不改 JS

#### 13E — Home.vue（1 处）

| 文件 | 当前 | 改后 | desktopWidth |
|------|------|------|-------------|
| `Home.vue` | `width="500px"` | `:width="dialogWidth"` | `'500px'` |

#### 13F — 编译验证

```bash
cd frontend && npm run build
```

验证清单:
- [ ] 编译无报错
- [ ] 桌面端（≥768px）所有弹窗宽度 = 原固定 px 值（零回归）
- [ ] 移动端（<768px）普通弹窗（useDialogWidth）宽度 = 视口 90%
- [ ] 移动端 UploadModal + 版本预览弹窗 = fullscreen 全屏，代码可滚动
- [ ] 移动端 PlatformManage 弹窗 body 纵向可滚动（max-h-[calc(100vh-200px)]）
- [ ] 预览弹窗 `<pre>` 桌面端 max-h-96，全屏模式充分利用高度
- [ ] 暗色模式弹窗样式正常

### 不影响的部分

- **后端**: 零改动
- **API 调用 / 路由 / 状态管理**: 不变
- **表单逻辑 / 校验 / 提交**: 不变
- **弹窗 open/close 生命周期**: 不变
- **ConfirmDialog / UploadModal / OIDCSwitchDialog 的 props/emits**: 不变
- **ActionMenu.vue / useIsMobile.js**: 保留不变
- **卡片布局 / 表格布局**: 不变

### 约束

- 不使用 `!important`，不写全局 CSS 覆盖 Element Plus
- 桌面端所有弹窗宽度与原值完全一致（零回归）
- 统一断点 768px（`useIsMobile`），与 Phase 2.1/2.2 一致
- 预览弹窗 fullscreen 仅移动端启用（`:fullscreen="isMobile"`）
- 不修改 Element Plus 内部样式
- 不新增后端 API

---

## Phase 2.4 — 全项目 UI/UX 体验修复

> **触发**: Phase 2.3 方案研究完成后的全项目 UI/UX 审计。
> **范围**: 13 项初始检查发现 → 3 项代码验证后排除（复制反馈 ✅、favicon ✅、OIDCConfig spinner 部分已有）→ **12 项待修复**（含审阅新增 #11 加载态、#12 暗色验证）。
> **与 Phase 2.3 关系**: 无直接代码冲突。Phase 2.3 改 dialog 宽度，Phase 2.4 修交互反馈和边界场景。可先后或并行实施。

### 代码验证排除项

| 原编号 | 问题 | 验证结果 |
|--------|------|---------|
| #3 | 复制操作无反馈提示 | ❌ 误报 — 3 处均已实现 `toastSuccess('已复制到剪贴板')` |
| #13 | 缺少 favicon | ❌ 误报 — `public/vite.svg` 存在 |

### 总览

| # | 优先级 | 问题 | 文件 | 可合并 |
|---|--------|------|------|--------|
| 1 | 🔴 | Rules.vue 移动端表格溢出 | Rules.vue | 与 #8 合并 |
| 2 | 🔴 | Toast 无过渡动画 | useToast.js + App.vue | 与 #9 合并 |
| 3 | 🔴 | Login.vue 登录按钮无 loading | Login.vue | — |
| 4 | 🟡 | OIDCConfig 保存/测试按钮无 spinner | OIDCConfig.vue | — |
| 5 | 🟡 | UserManage toggle 开关无 ARIA | UserManage.vue | — |
| 6 | 🟡 | 路由切换无全局 loading 指示 | router/index.js + App.vue | — |
| 7 | 🟡 | 无 404 页面 | router/index.js + 新文件 | — |
| 8 | 🟡 | Rules.vue 加载失败静默忽略 | Rules.vue | 与 #1 合并 |
| 9 | 🟢 | Toast 容器无上限 | useToast.js | 与 #2 合并 |
| 10 | 🟢 | Home.vue 8 v-if 分支可维护性差 | Home.vue | — |
| 11 | 🟡 | ShareList.vue 缺少加载态 | ShareList.vue | — |
| 12 | 🟡 | EP 暗色模式实测验证 | 全部（测试） | — |

---

### #1 + #8: Rules.vue 卡片化 + 错误提示（🔴+🟡，合并修复）

**决策**: Q1 → B。Rules 是选择和下载页，不需要展示细节，卡片更合适。

**现状**:
```
文件: frontend/src/views/Rules.vue
路由: /rules（用户规则浏览页，非管理页）
问题 A: el-table + fixed="right" 操作列，移动端溢出
问题 B: onMounted catch 块无 toast 提示，用户无法区分「无规则」和「加载失败」
```

**修复方案 — 卡片化**:
- 替换 el-table 为卡片 grid（与 RulesManage.vue 卡片化模式一致）
- 移除 `fixed="right"`、无需 ActionMenu、无需 useIsMobile
- 每张卡片显示：规则名称 + 客户端类型标签 + 当前版本号 + 下载按钮
- `catch` 块加 `toastError('加载规则失败')`

**卡片设计**:
```
┌──────────────────────────┐
│ 规则名称     [Shadowrocket]│
│ 当前版本: v3             │
│ 更新于: 2026-07-22       │
│         [下载当前版本]    │
└──────────────────────────┘
```

**layout 改动**:
```vue
<!-- 当前: el-table -->
<el-table :data="rules" stripe>...</el-table>

<!-- 改后: 卡片 grid -->
<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
  <div v-for="rule in rules" :key="rule.id"
    class="bg-white dark:bg-gray-800 rounded-lg shadow-md overflow-hidden">
    <!-- 卡片头: 名称 + 客户端类型 -->
    <div class="px-4 py-3 border-b ...">
      <span>{{ rule.name }}</span>
      <span class="...">{{ rule.client_type || 'Shadowrocket' }}</span>
    </div>
    <!-- 卡片体: 版本 + 下载 -->
    <div class="p-4">
      <div>当前版本: v{{ currentVersion(rule) }}</div>
      <div v-if="currentUpdatedAt(rule)">更新于: {{ formatTime(...) }}</div>
      <a :href="getRuleDownloadUrl(rule.id, rule.token)" ...>
        <button>下载当前版本</button>
      </a>
    </div>
  </div>
</div>
```

**影响面**: 仅改 Rules.vue 一个文件。`publicApi.getRules()` 调用方不变，API 契约不变。

---

### #2 + #9: Toast 动画 + 上限（🔴+🟢，合并修复）

**现状**:
```
文件: frontend/src/composables/useToast.js + App.vue
问题 A: toasts 数组直接 push/splice，无 CSS transition
问题 B: 无 max 条数限制，批量 toast 会堆叠溢出
```

**修复方案 — useToast.js 改动**:
```js
const MAX_TOASTS = 5

function show(message, type = 'info') {
  const toast = { id: ++id, message, type }
  toasts.value.push(toast)
  // 超出上限移除最旧的
  if (toasts.value.length > MAX_TOASTS) {
    toasts.value = toasts.value.slice(-MAX_TOASTS)
  }
  setTimeout(() => {
    toasts.value = toasts.value.filter(t => t.id !== toast.id)
  }, 3000)
}
```

**修复方案 — App.vue 改动**:
```vue
<!-- 当前: 直接 v-for -->
<div v-for="t in toasts" :key="t.id" ...>{{ t.message }}</div>

<!-- 改后: TransitionGroup + 动画 -->
<TransitionGroup name="toast" tag="div" class="fixed bottom-4 right-4 z-50 flex flex-col gap-2">
  <div v-for="t in toasts" :key="t.id" ...>{{ t.message }}</div>
</TransitionGroup>
```

**CSS 动画**（写在 `tailwind.css` 或 App.vue `<style>`）:
```css
.toast-enter-active { transition: all 0.3s ease-out; }
.toast-leave-active { transition: all 0.2s ease-in; }
.toast-enter-from { opacity: 0; transform: translateY(20px); }
.toast-leave-to { opacity: 0; transform: translateX(30px); }
```

**影响面**:
- `useToast.js`: 仅改内部逻辑，对外 API (`success/error/info/warning`) 不变
- `App.vue`: 仅改 template 的 toast 渲染部分，不影响 `<router-view />`
- 所有 10+ 个调用 `useToast()` 的组件无需改动
- 新增 CSS 约 6 行，不与 Tailwind 或 Element Plus 冲突

**与 Phase 2.3 关系**: 无冲突。

---

### #3: Login.vue 登录按钮防重复点击（🔴）

**现状**:
```
文件: frontend/src/views/Login.vue
问题: handleLogin() 和 handleSwitchAccount() 均直接 window.location.href 跳转
     两个函数都无 loading 状态，快速双击可能触发多次 OIDC redirect
```

**修复方案**（两个函数共用一个 `loggingIn` ref）:
```js
const loggingIn = ref(false)

function handleLogin() {
  if (loggingIn.value) return
  loggingIn.value = true
  window.location.href = '/api/v1/auth/login'
}

function handleSwitchAccount() {
  if (loggingIn.value) return
  loggingIn.value = true
  window.location.href = '/api/v1/auth/login?prompt=login'
}
```

Template 改动: 两个按钮均加 `:disabled="loggingIn"` + spinner SVG。

**影响面**: 仅 Login.vue 内部。无 API 调用，无路由影响。

**与 Phase 2.3 关系**: 无冲突。Login.vue 不在 Phase 2.3 范围内。

---

### #4: OIDCConfig.vue 保存/测试按钮加 spinner（🟡）

**现状**:
```
文件: frontend/src/views/OIDCConfig.vue
对比: Setup.vue 的测试连接/完成配置按钮有 <svg> spinner
问题: OIDCConfig 的同名按钮只有 :disabled 属性，无 spinner
```

**修复方案**: 复制 Setup.vue 的 spinner SVG 到 OIDCConfig 的测试连接和保存按钮。同时顺手给速率限制的两个 `<input type="number">` 加 `max="1000"` 属性（前端辅助校验，后端已有保护）。

```vue
<!-- 当前 -->
<button ... :disabled="saving" @click="handleTest">测试连接</button>
<!-- 改后 -->
<button ... :disabled="saving" @click="handleTest">
  <svg v-if="testing" class="animate-spin ..." ...>...</svg>
  测试连接
</button>
```

**影响面**: 仅 OIDCConfig.vue template，不改 script 逻辑。

> **备注**: Setup.vue 的「完成配置」按钮已有 `:disabled="saving"` 保护，功能上可接受。如需进一步体验优化，可同样加 spinner SVG（低优先级，可选）。

**与 Phase 2.3 关系**: 无冲突。OIDCConfig 使用 OIDCSwitchDialog（13B 处理），自身无需要宽度自适应的弹窗。

---

### #5: UserManage toggle 开关 ARIA 无障碍（🟡）

**现状**:
```
文件: frontend/src/views/UserManage.vue（编辑对话框内的 is_advanced 切换）
问题: 自定义 <button> + <span> 模拟 toggle，无 role/aria/keyboard 支持
```

**修复方案**: 加 ARIA 属性 + 键盘事件，无需改动视觉样式：
```vue
<button
  role="switch"
  :aria-checked="editIsAdvanced"
  @click="editIsAdvanced = !editIsAdvanced"
  @keydown.space.prevent="editIsAdvanced = !editIsAdvanced"
  @keydown.enter.prevent="editIsAdvanced = !editIsAdvanced"
  ...
>
```

**影响面**: 仅 UserManage.vue 编辑对话框内的 1 个按钮。不改样式、不改逻辑。

**与 Phase 2.3 关系**: 无冲突。Phase 2.3 改 UserManage 的 3 个 el-dialog width 属性，不涉及弹窗内容。

---

### #6: 路由切换全局 loading 指示（🟡，方案已确认）

**决策**: Q2 → A。纯 CSS 顶部进度条，零依赖。

**修复方案**:
- 在 `App.vue` 中监听 `router.beforeResolve` / `router.afterEach`
- 显示一条 2px 高的顶部蓝色渐变条（类似 YouTube/GitHub）
- 纯 CSS + 少量 JS，无第三方依赖

**影响面**: 需改 `router/index.js`（加 afterEach hooks）+ `App.vue`（加进度条元素 + CSS）。不影响任何页面组件。

**与 Phase 2.3 关系**: 无冲突。

---

### #7: 404 页面（🟡）

**现状**:
```
文件: frontend/src/router/index.js
问题: 访问未定义路由时空白页，无提示
```

**修复方案**: 
1. 新建 `frontend/src/views/NotFound.vue`（约 20 行，居中显示"页面不存在"+ 返回首页按钮）
2. `router/index.js` 加 catch-all 路由: `{ path: '/:pathMatch(.*)*', name: 'NotFound', component: ... }`

**影响面**: 1 新文件 + 1 行路由配置。不影响现有页面。

**与 Phase 2.3 关系**: 无冲突。

---

### #10: Home.vue 8 v-if 分支可维护性（🟢，已确认暂缓）

**决策**: Q3 → A。当前功能正确，重构风险 > 收益。等下次 Home.vue 有功能性改动时一并重构。本次不实施。

---

### #11: ShareList.vue 缺少加载态（🟡，审阅发现）

**现状**:
```
文件: frontend/src/views/ShareList.vue
问题: template 中无 v-if="loading" 分支，数据加载期间显示空卡片 grid
对比: SubList.vue 使用标准三态（loading → empty → content），ShareList 缺失 loading 态
```

**修复方案**: 改为与 SubList 一致的三态分支：
```vue
<!-- 当前 -->
<div v-if="!loading && shares.length === 0">暂无分享订阅，请创建</div>
<div v-else class="grid ...">

<!-- 改后 -->
<div v-if="loading" class="flex items-center justify-center py-12">
  <svg class="animate-spin h-5 w-5 mr-2 text-blue-600">...</svg>
  <span>加载中...</span>
</div>
<div v-else-if="shares.length === 0" class="text-center py-12 text-gray-400">暂无分享订阅，请创建</div>
<div v-else class="grid ...">
```

**影响面**: 仅 ShareList.vue template。不改 script 逻辑。

---

### #12: EP 暗色模式实测验证（🟡，测试项）

**说明**: Phase 2.3 实施完成后，在浏览器暗色模式下逐一验证 5 个保留 EP 组件的显示效果：
- `el-table`: 表头背景、边框、hover 行
- `el-dialog`: 弹窗背景、遮罩
- `el-form`: label 颜色
- `el-upload`: 拖拽区边框
- `el-menu`: 菜单项背景、激活态

如发现问题，在对应页面 `<style scoped>` 中用 `:deep()` 添加暗色覆盖。**不改 useTheme.js**。若一切正常则无需额外处理。

---

## 实施计划（建议顺序）

由于与 Phase 2.3 无冲突，可与 Phase 2.3 先后或并行实施。以下按优先级 + 合并机会排序：

| 块 | 内容 | 涉及文件 | 预计时间 | 依赖 |
|----|------|---------|---------|------|
| 14A | #2+#9 Toast 动画+上限 | useToast.js, App.vue, tailwind.css | 20min | — |
| 14B | #3 Login 防重复点击 | Login.vue | 10min | — |
| 14C | #1+#8 Rules.vue 卡片化+错误提示 | Rules.vue | 25min | — |
| 14D | #4 OIDCConfig spinner | OIDCConfig.vue | 10min | — |
| 14E | #5 ARIA toggle | UserManage.vue | 5min | — |
| 14F | #6 路由 loading（方案 A 纯 CSS） | router/index.js, App.vue | 20min | — |
| 14G | #7 404 页面 | NotFound.vue(新), router/index.js | 10min | — |
| 14H | #11 ShareList 加载态修复 | ShareList.vue | 5min | — |
| 14I | #12 EP 暗色模式验证 | 全部（测试） | 15min | Phase 2.3 完成后 |
| 14J | 编译验证 | 全部 | 5min | 14A-14I |

依赖: 无内部依赖，14A-14G 可完全并行。
总预计时间: ~100min

---

> 决策已确定，详见文档末尾「全部 Q&A 决策汇总」。

| 块 | 改动文件 | 改 template | 改 script | 新增文件 | 后端影响 |
|----|---------|------------|-----------|---------|---------|
| 14A | 2 | ✅ | ✅ | — | — |
| 14B | 1 | ✅ | ✅ | — | — |
| 14C | 1 | ✅ | ✅ | — | — |
| 14D | 1 | ✅ | — | — | — |
| 14E | 1 | ✅ | — | — | — |
| 14F | 2 | ✅ | ✅ | — | — |
| 14G | 2 | ✅ | — | 1 (NotFound.vue) | — |
| 14H | 1 | ✅ | — | — | — |
| 14I | 0 | — | — | — | — |
| **合计** | **10 文件** | — | — | **1 新文件** | **零** |

---

## Phase 2.5 — 导航入口 + 表单 ID 清理 + 规则页风格统一

> **触发**: 全项目 UI/UX 审计后续发现。
> **范围**: 3 个独立问题，涉及前后端共 9 个文件。
> **与 Phase 2.3 / 2.4 关系**: 无代码冲突。Phase 2.5 的 #2（ID 清理）涉及后端改动，这是唯一跨越前后端边界的 Phase。

### 总览

| # | 问题 | 涉及文件 | 后端改动 | 决策 |
|---|------|---------|---------|------|
| 1 | Home 页面分流规则入口 + 公告栏 | Home.vue + 后端新增 API | ✅ 需后端支持 | ✅ Q4: 卡片形式 |
| 2 | RulesManage / PlatformManage ID 输入 | RulesManage.vue, PlatformManage.vue + 后端 | ✅ 需后端支持 | ✅ Q5: 保留只读 |
| 3 | Rules.vue 风格统一 | Rules.vue | — | ✅ Q6: 简洁返回按钮 |

---

### #1: Home 页面「分流规则」卡片 + 「公告栏」卡片

**决策**: Q4 → 将「分流规则」入口设计为独立卡片（与平台订阅卡片同级）；同时新增公告栏功能。

#### 1A — 分流规则卡片

在 Home 页面的平台卡片 grid 上方添加一张独立的「分流规则」卡片：

```
┌──────────────────────────────┐
│ 🔧 分流规则                   │
│ 浏览和下载可用的分流规则配置    │
│              [查看规则 →]     │
└──────────────────────────────┘
```

- 放在 `<main class="p-6">` 内、平台卡片 grid 之前
- 使用与平台卡片相同的 `bg-white dark:bg-gray-800 rounded-lg shadow-md` 样式
- 但横向撑满（不放在 grid 内），内部 `flex justify-between items-center`
- 点击跳转 `/rules`

**影响面**: 仅 Home.vue template，加 ~15 行。

#### 1B — 公告栏系统

**需求**: 管理员在后台输入公告内容，首页展示公告卡片；无内容时不显示。

**后端设计**:

| 组件 | 内容 |
|------|------|
| 存储 | `system_config` 表新 key: `announcement_content`（TEXT，空字符串 = 无公告） |
| Admin API | `PUT /api/v1/admin/system/announcement` — 保存公告内容 |
| Admin API | `GET /api/v1/admin/system/announcement` — 获取当前公告 |
| Public API | `GET /api/v1/system/announcement` — 获取公告（无需认证，供首页调用） |

**后端改动文件**:
- `handler/handlers.go` — 新增 3 个 handler（Get/PUT announcement）  
- `router/router.go` — 新增 3 条路由（admin GET+PUT + public GET）
- `service/system_service.go` — 新增 `GetAnnouncement()` / `SetAnnouncement()` 方法（参照现有 `GetRateLimit`/`UpdateRateLimit` 模式）
- `repository/system_config_repo.go` — 复用现有 `Get(key)` / `Set(key, value)` 方法

**管理端 UI**: 在 `OIDCConfig.vue` 页面添加第三个卡片「公告栏」，含 textarea + 保存按钮。与 OIDC 配置、速率限制并列。

**首页 UI**: 在 Home 页面分流规则卡片下方，公告栏卡片：
```
┌──────────────────────────────┐
│ 📢 公告栏                     │
│ 这是一条公告内容...            │
│                         [✕]  │
└──────────────────────────────┘
```
- `v-if="announcement"` 控制显隐
- 右侧 [✕] 关闭按钮，`localStorage` 记录关闭状态（session 级别）
- 支持多行文本，`whitespace-pre-wrap`

**localStorage 关闭逻辑**: 
- 键名: `dismissed-announcement-{content前20字符}`
- 用户点击关闭后，同一内容不再显示；管理员更新内容后，哈希变化，重新显示

**决策**: Q8 → A。取内容前 20 字符做哈希键名，简单有效。管理员编辑公告不需要额外区分，编辑入口在管理面板 OIDCConfig 页。

---

### #2: RulesManage / PlatformManage 创建弹窗移除 ID 输入

**决策**: Q5 → PlatformManage 编辑时保留 ID 为只读文本。

**后端改动**（增加 `GenerateUUID()` fallback）:
- `rule_service.go`: `Create()` 方法加 `if rule.ID == "" { id := GenerateUUID()[:12] }`
- `platform_service.go`: `Create()` 方法同上
- `handlers.go`: `CreateRule()` 移除 JSON 和 multipart 两处的 `id` 必填校验

> ⚠️ **实施顺序**: 必须先改 service 层（加 auto-generate），再改 handler 层（移除必填校验）。如果先移除 handler 校验，空 ID 传入 service 会触发 `IsValidID("")` 报错。

**前端改动**:

| 文件 | 创建模式 | 编辑模式 |
|------|---------|---------|
| `RulesManage.vue` | 删除 ID 输入框 + 校验规则 | N/A（无编辑模式） |
| `PlatformManage.vue` | 删除 ID 输入框 | ID 改为只读灰色文本 `<span>ID: clash-verge</span>` |

---

### #3: Rules.vue 风格统一

**决策**: Q6 → 简洁「← 首页」返回按钮。

**合并实施**: 与 Phase 2.4 的 14C（Rules.vue 卡片化）一次性完成：
- Top bar: `← 首页` 返回按钮 + 标题「分流规则」
- 内容: 卡片 grid 替换 el-table
- 样式: 与 Home.vue 卡片一致
- 错误处理: catch 块加 toast 提示

> **注意**: 当前 Rules.vue 未引入 `useRouter`，实施时需新增 `import { useRouter } from 'vue-router'` + `const router = useRouter()` 以支持「← 首页」按钮的 `router.push('/')`。

---

## Phase 2.6 — 全局 UI 比例调整

> **触发**: 用户反馈「适度增大字号，合理利用空间，不需要过度留白」。
> **范围**: 全局替换，约 15 个文件，~100 处改动。
> **原则**: 内容尺寸增大但间距不变（让页面更充实而非更稀疏）。

### 当前比例审计

| 元素 | 当前值 | 问题 |
|------|--------|------|
| 页面标题 | `text-xl` (20px) | 适度，管理后台标准 |
| 卡片标题（Admin） | `text-sm font-semibold` (14px) | **偏小** — Home 用 `text-base`，不一致 |
| 按钮文字（卡片内） | `text-xs` (12px) | **太小** — 桌面勉强可读，移动端难点击 |
| 按钮文字（对话框） | `text-sm` (14px) | 合理 |
| 创建按钮 | `px-3 py-1.5 text-sm` | 合理 |
| 输入框文字 | `text-sm` (14px) | 偏小，可提升到 `text-base` |
| 描述/元信息 | `text-sm text-gray-500` (14px) | 合理，辅助信息 |
| 标签/徽章 | `text-xs` (12px) | 合理，装饰性元素 |
| 卡片内边距 | `p-4` (16px) | 合理 |
| 卡片间距 | `gap-5` (20px) | 合理 |
| 页面外边距 | `p-6` (24px) | 合理 |

### 调整方案

**核心思路**: 只增大字号，不增大间距。内容变大自然填充空间，无需额外留白。

#### 层级 1: 按钮字号统一（影响最广，收益最大）

| 位置 | 当前 | 改动后 |
|------|------|--------|
| 卡片内操作按钮 | `px-3 py-1.5 text-xs` | `px-3 py-1.5 text-sm` |
| 版本页表格操作按钮 | `px-3 py-1.5 text-xs` | `px-3 py-1.5 text-sm` |
| Home 页订阅操作按钮 | `px-3 py-1.5 text-xs` | `px-3 py-1.5 text-sm` |
| 「刷新链接」文字按钮 | `text-xs` | `text-sm` |

**涉及文件**: Home.vue, SubList.vue, ShareList.vue, UserManage.vue, RulesManage.vue, PlatformManage.vue, SubVersions.vue, ShareVersions.vue, RuleVersions.vue, Logs.vue — 共 **10 个 views**。

#### 层级 2: 卡片标题统一

| 位置 | 当前 | 改动后 |
|------|------|--------|
| Admin 卡片标题 | `text-sm font-semibold` | `text-base font-semibold` |

**涉及文件**: SubList.vue, ShareList.vue, UserManage.vue, RulesManage.vue, PlatformManage.vue — 共 **5 个 views**（Home.vue 已使用 `text-base`，无需改）。

#### 层级 3: 输入框字号（已确认）

**决策**: Q7 → A。在层级 1+2 验证通过后实施。

| 位置 | 当前 | 改动后 |
|------|------|--------|
| 所有 `<input>` / `<textarea>` / `<select>` | `text-sm` | `text-base` |

这会影响所有表单（Setup, OIDCConfig, SubList, ShareList, UserManage, RulesManage, PlatformManage 等弹窗内的输入框）。输入框变大后对话框可能需要在桌面端略微加宽，届时视实际情况调整。

### 不变的部分

| 元素 | 保持 | 理由 |
|------|------|------|
| 页面标题 | `text-xl` | 已足够，管理后台标准 |
| 卡片内边距 | `p-4` | 不增大留白 |
| 卡片间距 | `gap-5` | 不增大留白 |
| 描述文字 | `text-sm` | 辅助信息，不需突出 |
| 标签徽章 | `text-xs` | 装饰性，保持小巧 |
| 圆角 | `rounded-md` / `rounded-lg` | 不变 |

### 实施计划

| 块 | 内容 | 涉及文件数 | 改动处 | 风险 |
|----|------|----------|--------|------|
| 16A | 层级1: 按钮 `text-xs` → `text-sm` | 10 views | ~60 处 | 低 — 纯字号替换 |
| 16B | 层级2: 卡片标题 `text-sm` → `text-base` | 5 views | ~10 处 | 低 — 纯字号替换 |
| 16C | 层级3: 输入框 `text-sm` → `text-base` | ~12 文件 | ~50 处 | 中 — 可能需调对话框宽度 |
| 16D | 编译验证 | 全部 | — | — |

---

## 全局实施排序（按依赖合并后的推荐顺序）

> 排序原则: 后端改动优先（减少前端等待）→ 基础组件先行（dialog 宽度 / Toast 系统）→ 页面改动批处理。

### 第一梯队：后端 + 基础前端组件（可并行）

| 顺序 | 块 | Phase | 内容 | 文件 | 预计 |
|------|----|-------|------|------|------|
| 1 | 15D | 2.5 | 后端 ID 自动生成 | rule_service.go, platform_service.go | 10min |
| 2 | 15B | 2.5 | 公告栏后端 API | handlers.go, router.go | 20min |
| 3 | 13A | 2.3 | 新建 useDialogWidth.js | useDialogWidth.js（新） | 3min |
| — | — | — | 后端编译验证 | 全部后端 | 5min |

### 第二梯队：dialog 宽度 + Toast 系统（可并行）

| 顺序 | 块 | Phase | 内容 | 文件 | 预计 |
|------|----|-------|------|------|------|
| 4 | 13B | 2.3 | 共享组件 dialog 接入 | ConfirmDialog, OIDCSwitchDialog, UploadModal | 5min |
| 5 | 13C | 2.3 | 卡片列表页 dialog 接入 | SubList, ShareList, UserManage, RulesManage, PlatformManage | 10min |
| 6 | 13D | 2.3 | 版本预览弹窗全屏 + pre 高度 | SubVersions, ShareVersions, RuleVersions | 5min |
| 7 | 13E | 2.3 | Home.vue dialog 接入 | Home.vue | 2min |
| 8 | 14A | 2.4 | Toast 动画 + 上限 | useToast.js, App.vue, tailwind.css | 20min |
| — | — | — | 前端编译验证 | 全部前端 | 5min |

### 第三梯队：页面体验修复 + 公告栏前端（可并行）

| 顺序 | 块 | Phase | 内容 | 文件 | 预计 |
|------|----|-------|------|------|------|
| 9 | 15C | 2.5 | 公告栏管理端 UI | OIDCConfig.vue | 15min |
| 10 | 15E | 2.5 | 前端移除 ID 输入 | RulesManage.vue, PlatformManage.vue | 15min |
| 11 | 14C+15F | 2.4+2.5 | Rules.vue 卡片化 + top bar + 错误提示 | Rules.vue | 25min |
| 12 | 14B | 2.4 | Login 防重复点击 | Login.vue | 10min |
| 13 | 14D | 2.4 | OIDCConfig spinner + max | OIDCConfig.vue | 10min |
| 14 | 14E | 2.4 | ARIA toggle | UserManage.vue | 5min |
| 15 | 14F | 2.4 | 路由 loading 进度条 | router/index.js, App.vue | 20min |
| 16 | 14G | 2.4 | 404 页面 | NotFound.vue（新）, router/index.js | 10min |
| 17 | 14H | 2.4 | ShareList 加载态 | ShareList.vue | 5min |
| 18 | 15A | 2.5 | Home 分流规则卡片 + 公告栏卡片 | Home.vue | 15min |

### 第四梯队：UI 比例 + 暗色验证

| 顺序 | 块 | Phase | 内容 | 文件 | 预计 |
|------|----|-------|------|------|------|
| 19 | 16A | 2.6 | 按钮字号统一 | 10 views | 30min |
| 20 | 16B | 2.6 | 卡片标题统一 | 5 views | 10min |
| 21 | 14I | 2.4 | EP 暗色模式验证 | 全部（浏览器测试） | 15min |
| — | — | — | 最终编译验证 | 全部 | 5min |

### 总计

| 梯队 | 块数 | 预计总时间 |
|------|------|-----------|
| 第一梯队（后端） | 3 块 | ~35min |
| 第二梯队（dialog+toast） | 5 块 | ~45min |
| 第三梯队（页面修复） | 10 块 | ~130min |
| 第四梯队（比例+验证） | 3 块 | ~60min |
| **合计** | **21 块** | **~4.5h** |

### 文件改动频次（同一文件被多个块修改，合并实施）

| 文件 | 涉及块 | 建议 |
|------|--------|------|
| Home.vue | 13E + 15A + 16A | 一次性改完 |
| Rules.vue | 14C + 15F + 16A | 一次性改完 |
| OIDCConfig.vue | 14D + 15C + 16A | 一次性改完 |
| UserManage.vue | 13C + 14E + 15E + 16A + 16B | 一次性改完 |
| RulesManage.vue | 13C + 15E + 16A + 16B | 一次性改完 |
| PlatformManage.vue | 13C + 15E + 16A + 16B | 一次性改完 |
| App.vue | 14A + 14F + 16A | 一次性改完 |
| router/index.js | 14F + 14G | 一次性改完 |

> **实施建议**: 按文件分组实施，不按块逐个实施。例如，打开 Home.vue 后一次性完成 13E + 15A + 16A 三个块的所有改动。

## 全部 Q&A 决策汇总

| Q | 问题 | 决策 | 状态 |
|---|------|------|------|
| Q1 | Rules.vue 移动端 | B: 卡片化 | ✅ |
| Q2 | 路由 loading | A: 纯 CSS 进度条 | ✅ |
| Q3 | Home.vue 分支 | A: 暂不改 | ✅ |
| Q4 | Home rules 入口 | 卡片形式 + 公告栏 | ✅ |
| Q5 | PlatformManage ID | 编辑时保留只读 | ✅ |
| Q6 | Rules.vue top bar | 简洁「← 首页」 | ✅ |
| Q7 | 输入框字号 | A: 层级 1+2 验证后实施 | ✅ |
| Q8 | 公告栏哈希 | A: 内容前 20 字符 | ✅ |

> 无待确认项，全部决策已确定。

---

## 实施完成记录（2026-07-23）

### 实施汇总

全部 4 个 Phase（2.3/2.4/2.5/2.6）均已实施完成，前后端编译均通过。

| Phase | 实施文件数 | 关键产出 |
|-------|----------|---------|
| 2.3 | 13 | useDialogWidth.js, 14 处 dialog 宽度自适应, 4 处 fullscreen, PlatformManage body 滚动, pre 高度适配 |
| 2.4 | 10 + NotFound.vue | Toast 动画+上限, Login 防重复, OIDC spinner, ARIA toggle, 路由进度条, 404 页面, ShareList 加载态, Rules.vue 卡片化, overflow-x-auto 补齐 |
| 2.5 | 9 (含后端 4) | 后端 ID 自动生成, 公告栏前后端完整系统, Home 分流规则+公告栏卡片, Rules/PlatformManage 去 ID 输入, Rules.vue top bar |
| 2.6 | 15 | 按钮字号 text-xs→text-sm (10 views), 卡片标题 text-sm→text-base (5 views) |

### 实施中发现的 Bug 及修复

| Bug | 位置 | 描述 | 修复 |
|-----|------|------|------|
| 🔴 严重 | Home.vue | 平台卡片 grid 被错误绑定为公告栏卡片的 `v-else`，导致有公告时平台卡片不显示 | `v-else` → `v-if="!loading && platforms.length > 0"` |
| 🔴 严重 | Home.vue | `fetchAnnouncement()` 调用 `publicApi.getAnnouncement()` 但未导入 `publicApi` | 在 import 中新增 `publicApi` |
| 🟡 中等 | App.vue | 路由进度条使用 `beforeResolve()` 钩子触发太晚（仅在组件解析时） | 改为 `beforeEach()` 钩子，在整个导航周期显示 |
| 🟡 中等 | Rules.vue | 卡片化替换后遗留旧代码片段导致编译失败 | 清理残留的 `})` 和 `</script>` 标签 |

### 跳过项

| 块 | 内容 | 原因 |
|----|------|------|
| 14I | EP 暗色模式实测验证 | 纯测试项，需浏览器实际操作 |
| 16C | 层级 3: 输入框字号 `text-sm`→`text-base` | 按 Q7 决定，待层级 1+2 验证后再做 |

### 验证状态

| 检查项 | 结果 |
|--------|------|
| `go build ./...` | ✅ 通过 |
| `go vet ./...` | ✅ 通过 |
| `npm run build` | ✅ 通过 |
| `el-table` 仅限 4 个表格保留页 | ✅ Logs, SubVersions, ShareVersions, RuleVersions |
| `text-xs` 仅限标签/徽章/辅助文字 | ✅ 按钮已全部改为 `text-sm` |
