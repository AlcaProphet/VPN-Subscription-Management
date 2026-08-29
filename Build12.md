# Build12.md — VPN 订阅管理系统 UI/UX 后续修复当前构建方案

> **文档定位：** 本文档是承接 Build11 验收后的**后续修复构建方案**，针对核验发现的 2 个非阻断项：旧灰度类全量 Token 化、全局单浮层与统一焦点管理。本文档按 `docs/DocTemplates/Build.template.md` 样式编写。
> - 当前设计：`Design2.md`、`Design2-UI.md`
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 历史构建：`docs/reports/Build/`（Build1~Build10 已存档）、`Build11.md`（已收口，待归档）
> - 执行原则：每次仅执行一个 Step，完成后运行验收命令，确认通过后再进入下一步。
> - 用户已确认方向：①全量类名迁移；②全局所有浮层统一管理；③直接 Modal 纳入焦点管理。

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 0 | 创建 Build12 文档 | ✅ 已完成（本文档） |
| 1 | 全量灰色/白色类迁移到 Token 类 | ✅ 验收通过 |
| 2 | 全局单浮层管理器 + 统一焦点管理 | ✅ 验收通过 |
| 3 | 测试补强、文档与收口 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `frontend/tailwind.config.js`、`frontend/src/style.css`、约 43 个 `frontend/src/**/*.vue`、`frontend/src/**/*.ts` | 将 `gray-*`/`dark:bg-gray-*`/`bg-white` 替换为 `text-text`、`text-text-secondary`、`text-text-tertiary`、`bg-surface`、`bg-page`、`bg-surface-subtle`、`border-subtle`、`border` 等 Token 类，删除临时 CSS 旧类映射 |
| 2 | 新增 `frontend/src/utils/overlayManager.ts`、`frontend/src/components/AppDropdown.vue`、`frontend/src/components/AppModal.vue`、`frontend/src/components/AppPopover.vue`、`frontend/src/components/AppSelect.vue`、`frontend/src/components/AppRangePicker.vue`、`frontend/src/components/AppTimePicker.vue`；改造 `FormOverlay.vue`、`ConfirmModal.vue`、`AppHeader.vue`、`HomeView.vue`、`UsersView.vue`、`VersionManageView.vue`、`XrayInstancesView.vue`、`RulesView.vue`、`assembly/PoolTab.vue`、`assembly/NodesGroupsStep.vue` 及全部 Select/RangePicker/TimePicker 使用点 | 全局浮层注册表、唯一浮层、Esc 关闭、焦点进入/陷阱/回焦 |
| 3 | `frontend/tests/`、`Build12.md`、`Design2-UI.md`、`AGENTS.md`、`ProdTestList.md` | 新增 overlay 单测、更新受影响组件测试、文档同步 |

---

## 三、构建顺序依赖图

```
Build11 收口
  → Step 1（全量 Token 化，为浮层/弹层视觉统一提供稳定基底）
  → Step 2（浮层管理器与焦点管理，依赖 Step 1 的视觉组件完备）
  → Step 3（测试、文档与收口）
```

---

## 四、分步构建计划

### Step 1：全量灰色/白色类迁移到 Token 类

- **目标：** 清除源码中的 `gray-*`、`dark:bg-gray-*`、`dark:text-gray-*`、`dark:border-gray-*` 与 `bg-white` 硬编码，全部使用 `uiTokens` 对应的 Tailwind Token 类。
- **前置条件：** Build11 Step 6 的 `uiTokens` 与 Tailwind 变量已就绪。

#### 1.1 映射规则

| 旧类名 | 新类名 |
|--------|--------|
| `text-gray-900/800/700` | `text-text` |
| `text-gray-600/500` | `text-text-secondary` |
| `text-gray-400` | `text-text-tertiary` |
| `bg-gray-50` | 页面级背景 → `bg-page`；容器/卡片/代码/空态 → `bg-surface-subtle` |
| `bg-gray-100` | `bg-surface-subtle` |
| `bg-gray-200` | `bg-surface-subtle` |
| `bg-gray-800` / `dark:bg-gray-800` | `bg-surface` |
| `bg-gray-900` / `dark:bg-gray-900` | 代码/pre → `bg-surface`；整页深色 → `bg-page` |
| `bg-white` | `bg-surface` |
| `border-gray-200` | `border-subtle` |
| `border-gray-300` | `border` |
| `dark:border-gray-600` | `border` 或 `border-subtle`，删除 `dark:` |
| `hover:bg-gray-100` | `hover:bg-surface-subtle` |
| `dark:hover:bg-gray-700` | 删除 `dark:`，使用 Token 自动适配 |
| `dark:text-gray-*` | 删除 `dark:`，使用对应 `text-*` Token |

#### 1.2 处理步骤

1. 生成全量清单：
   ```bash
   grep -RIn "gray-[0-9]\|dark:bg-gray\|dark:text-gray\|dark:border-gray\|bg-white" frontend/src
   ```
2. 按高频文件优先处理：
   - `HomeView.vue`
   - `views/admin/assembly/PoolDetail.vue`
   - `views/admin/XrayInstancesView.vue`
   - `views/admin/PlatformEditView.vue`
   - `views/admin/NodesView.vue`
   - `views/admin/AdminOverviewView.vue`
   - `views/SetupView.vue`
   - `views/admin/SettingsView.vue`
   - `components/AppHeader.vue`
   - 其余文件
3. 对 `bg-gray-50` / `bg-gray-900` 按上下文判断页面底、容器底、代码底。
4. 删除 `style.css` 中临时添加的旧类 CSS 映射块。
5. 验证：
   ```bash
   grep -RIn "gray-[0-9]\|dark:bg-gray\|dark:text-gray\|dark:border-gray" frontend/src || echo clean
   cd frontend && npm run build && npm test -- --run
   ```

- **验收标准：**
  - `frontend/src` 不再出现 `gray-[0-9]`、`dark:bg-gray-*`、`dark:text-gray-*`、`dark:border-gray-*`；
  - 浅色/暗色下页面底色、卡片、代码块、空态无视觉断层；
  - 构建与全部前端单测通过。

---

### Step 2：全局单浮层管理器 + 统一焦点管理

- **目标：** 同一时刻只允许一个交互浮层打开；Esc 关闭当前浮层并回焦；所有 Modal/Drawer/Dropdown/Select/DatePicker/TimePicker/Popover 统一焦点进入、焦点陷阱与关闭回焦。
- **前置条件：** Step 1 完成。
- **用户确认范围：** 全局所有浮层；直接 Modal 纳入焦点管理；Tooltip 不纳入单浮层强制关闭。

#### 2.1 新增 `frontend/src/utils/overlayManager.ts`

核心接口：

```ts
export type OverlayType = 'dropdown' | 'popover' | 'select' | 'picker' | 'modal' | 'drawer'

export interface OverlayHandle {
  id: string
  type: OverlayType
  close: () => void
  focusTrigger?: () => void
}

export function registerOverlay(handle: OverlayHandle): () => void
export function closeTopOverlay(): boolean
export function closeNonModalOverlays(): void
export function getActiveOverlay(): OverlayHandle | null
export function installGlobalEscapeHandler(): void
export function saveFocus(id: string): void
export function restoreFocus(id: string): void
```

规则：
- 非模态浮层（dropdown/popover/select/picker）全局唯一：新浮层打开时自动关闭上一个非模态浮层；
- 模态浮层（modal/drawer）允许合法层叠，例如“确认框叠在表单 Modal 上”；
- Esc 只关闭栈顶浮层，并调用其 `focusTrigger` 回焦；
- Modal/Drawer 打开时保存当前焦点，关闭时恢复。

#### 2.2 新增 `AppDropdown.vue`

包装 AntD `Dropdown`：
- 传入 `overlayId`、`onOpenChange`；
- 打开时调用 `registerOverlay`；
- 关闭时调用 `close` 与 `restoreFocus`；
- 自动把触发器元素记录为回焦目标。

#### 2.3 改造 `FormOverlay.vue`

- 通过 `overlayManager` 注册 `modal` / `drawer`；
- 打开时 `saveFocus` 并聚焦第一个可聚焦元素；
- 关闭时 `restoreFocus`；
- 保持现有桌面 Modal / 手机 Drawer 结构。

#### 2.4 改造 `ConfirmModal.vue`

- 与 FormOverlay 同一套焦点注册/回焦；
- 支持确认框叠在已打开 Modal 上时，不误关上层表单 Modal。

#### 2.5 直接 Modal 接入

改造以下页面中的直接 AntD `Modal`：

- `HomeView.vue`：按平台预览
- `RulesView.vue`：规则内容预览
- `assembly/PoolTab.vue`：池详情
- `assembly/NodesGroupsStep.vue`：节点选择与排序

统一使用 `AppModal.vue` 或直接调用 `overlayManager` 注册，保证焦点进入/陷阱/回焦。

#### 2.6 覆盖 AntD 内部弹层

- 已新增：
  - `AppSelect.vue`（全局注册，覆盖全部 Select 使用点）
  - `AppRangePicker.vue`
  - `AppTimePicker.vue`
- `AppPopover.vue` 已按同一模式接入；
- 通过 `openChange` / `dropdownVisibleChange` 事件注册到 `overlayManager`；
- 由于 AntD 内部 API 可能会随版本变化，先以当前 `ant-design-vue@4.2.6` 可用事件为准。

- **测试与验收命令：**
  ```bash
  cd frontend && npm run build && npm test -- --run
  ```
- **验收标准：**
  - 打开第二个非模态浮层时，第一个自动关闭；
  - Esc 关闭当前浮层并回焦到触发器；
  - Modal/Drawer 打开后 Tab 不逃出容器；
  - 确认框叠在表单 Modal 上仍可正常操作；
  - 手机端 Drawer 行为不回归。

---

### Step 3：测试补强、文档与收口

- **目标：** 为 Step 1/2 补齐自动化测试，同步设计/构建/问题文档。
- **前置条件：** Step 1、Step 2 完成。
- **产出：**
  - 新增 `frontend/tests/overlay-manager.spec.ts`；
  - 更新 `Design2-UI.md` 全局浮层与焦点约定；
  - 更新 `AGENTS.md` 当前构建方案指向；
  - 回写本文件进度与变更记录。

- **验收标准：**
  - 新增逻辑均有对应单测；
  - 既有 97+ tests 不回归；
  - 文档与代码状态一致。

---

## 五、候选/待细化项

| # | 候选 | 结论 | 状态 |
|---|------|------|------|
| 1 | 是否将红色/绿色/琥珀等状态色也统一到 Token | 本次仅迁移灰色/白色；状态色后续视需要另立 Step | 待定 |
| 2 | Tooltip 是否纳入单浮层强制关闭 | 不纳入，保持 hover 提示 | 已确认 |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-28 | 初始版本：承接 Build11 核验出的 2 个非阻断项，制定全量 Token 化与全局浮层/焦点管理修复方案。 |
| v1.1 | 2026-08-28 | 完成 Step 1：全量清理 `gray-*`/`dark:bg-gray-*`/`bg-white`，替换为 Token 类并删除旧 CSS 映射；完成 Step 2：新增 overlayManager、AppDropdown、AppModal、AppSelect、AppRangePicker、AppTimePicker，FormOverlay/ConfirmModal/直接 Modal 接入统一焦点与浮层管理，新增 overlay-manager 单测。 |
| v1.2 | 2026-08-28 | 完成 Step 3：补充 overlay-manager 单测，同步 Design2-UI §10.6、AGENTS 与 Build12 收口；前端 `npm run build` 与 `npm test -- --run`（104 tests）通过。 |
| v1.3 | 2026-08-30 | 完成 Issue9 R24-07：新增 AppPopover 并接入全局非模态浮层管理；订阅列表移除状态列与常驻 Alert，入池行改为可悬停、聚焦和点击的「待激活」浮层入口。 |
