# Phase 2.1 — 管理面板移动端表格 UX 改进

> 前置: Phase 2 全部 10 个块（10A-10J）已完成。
> 问题: 管理面板表格在手机上超出页面空间。

## Phase 2 完成状态

- [x] 10A-10J 全部完成（Tailwind CSS / Toast / UploadTabs / 公共组件 / 所有页面 / 单容器 / EP CSS 缩减）
- 残留: SubList/UserManage/ShareList 底部 scoped CSS 有未使用旧类，可择机清理。

## 问题分析

| 页面 | 操作按钮数 | 操作列宽度 | 表格最小宽度 |
|------|-----------|-----------|------------|
| UserManage | 5 | 340px | ~870px |
| ShareList | 5 | 320px | ~870px |
| RulesManage | 4 | 300px | ~910px |
| SubList | 3 | 260px | ~840px |
| PlatformManage | 2 | 160px | ~800px |
| Logs | 0 | - | ~820px |
| SubVersions等 | 3 | 240px | ~800px |

## 改进方案（三层组合策略）

### 策略 A: ActionMenu.vue 可复用组件

- 桌面端(md+): 正常显示所有操作按钮
- 移动端(<md): 显示「...」按钮，点击展开下拉菜单
- 操作列从 260-340px 缩减至 ~80px

### 策略 B: 非关键列移动端隐藏

| 页面 | 可隐藏列 | 节省宽度 |
|------|---------|---------|
| SubList | 更新时间 | 180px |
| RulesManage | 更新时间、Token预览 | 320px |
| Logs | IP、用户ID | 290px |
| PlatformManage | Client Schemes、下载链接 | 360px |
| SubVersions等 | 创建时间 | 180px |

实现: `v-if="!isMobile"` 条件渲染（非CSS隐藏，确保el-table列宽计算正确）

### 策略 C: 表格容器约束

- Manage.vue main 加 `min-w-0` 防止flex溢出
- 表格外包 `<div class="w-full overflow-x-auto">`

## 实施计划

| 块 | 内容 | 涉及文件 |
|----|------|---------|
| 11A | ActionMenu.vue + useIsMobile.js + Manage.vue main容器 | 3个文件 |
| 11B | 管理列表页接入(SubList/ShareList/UserManage/RulesManage/PlatformManage) | 5个views |
| 11C | 版本管理页+日志页(SubVersions/ShareVersions/RuleVersions/Logs) | 4个views |
| 11D | 移动端全量验证+清理残留CSS | 全部 |

依赖: 11A -> 11B -> 11C -> 11D

## 约束

- 不替换el-table / 不新增后端API / 桌面端零回归 / 暗色模式兼容
- ActionMenu需clickoutside自动关闭 / 移动端菜单项先关闭下拉再触发ConfirmDialog

---

## 实施完成状态（2026-07-22）

全部 4 个块（11A–11D）已完成。

| 块 | 内容 | 状态 |
|----|------|------|
| 11A | ActionMenu.vue + useIsMobile.js + Manage.vue min-w-0 | ✅ |
| 11B | SubList / ShareList / UserManage / RulesManage / PlatformManage | ✅ |
| 11C | SubVersions / ShareVersions / RuleVersions / Logs | ✅ |
| 11D | 编译验证 + 残留 CSS 清理 | ✅ |

**改动汇总**:
- 新增 2 文件: `ActionMenu.vue`, `useIsMobile.js`
- 修改 10 个 views: SubList, ShareList, UserManage, RulesManage, PlatformManage, SubVersions, ShareVersions, RuleVersions, Logs, Manage
- 清理 6 个文件残留 scoped CSS
- 桌面端零回归，移动端操作列从 260-340px 缩减至 80px（ActionMenu 下拉）

---

# Phase 2.2 — 管理面板卡片化重构 + Dialog z-index 修复

> 触发: Phase 2.1 移动端适配完成，但表格在手机上仍显拥挤；el-dialog 弹出层被 el-table 固定列遮挡。
> 目标: 5 个管理列表页改为 Home 风格卡片布局；全部 el-dialog 加 append-to-body 修复 z-index。

## 问题分析

### 问题 A: 表格被对话框遮挡

**根因**: 所有管理页面的 `el-table` 操作列使用 `fixed="right"`（sticky 定位创建独立层叠上下文），而全局所有 `el-dialog` 均未设置 `:append-to-body="true"`，导致 dialog 在表格的层叠上下文中渲染，被 sticky 列遮挡。

**影响范围**（12 处 el-dialog）:
- 3 个共享组件: `ConfirmDialog.vue`, `UploadModal.vue`, `OIDCSwitchDialog.vue`
- 9 个 views 中的直接 el-dialog: SubList, ShareList, UserManage(×3), RulesManage, PlatformManage, SubVersions, ShareVersions, RuleVersions, Home

**修复**: 全部加 `:append-to-body="true"` → dialog 挂载到 `<body>`，脱离表格层叠上下文。

### 问题 B: 列表页表格体验差

5 个管理列表页（SubList / ShareList / UserManage / RulesManage / PlatformManage）用 `el-table` 展示，在小屏上即使有 ActionMenu 仍显拥挤。改为 Home 风格卡片式布局可彻底解决空间问题。

## 页面分类

| 分类 | 页面 | 操作 |
|------|------|------|
| 列表页 → 卡片 | SubList, ShareList, UserManage, RulesManage, PlatformManage | 替换 el-table 为卡片 grid |
| 表格保留 | Logs, SubVersions, ShareVersions, RuleVersions | 仅加 append-to-body |
| 已是卡片 | OIDCConfig | 清理残留 CSS |
| 不受影响 | Home, Login, Setup, Rules(用户), Manage(布局壳) | 无改动 |

## 卡片布局设计

遵循 Home.vue 模式: `grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5`，每卡片 `bg-white dark:bg-gray-800 rounded-lg shadow-md`。

操作按钮改为**内联 flex-wrap**（卡片内按钮自然换行，无需 ActionMenu）。

### SubList 订阅卡片
```
┌──────────────────────────┐
│ 订阅名称           [默认] │
│ 平台: clash-verge   v3   │
│ 更新于: 2026-07-22       │
│ [版本管理] [编辑] [删除] │
└──────────────────────────┘
```

### ShareList 分享卡片
```
┌──────────────────────────┐
│ 分享名称           [有效] │
│ 创建于: 2026-07-22  v2   │
│ [版本管理]   [复制链接]   │
│ [刷新Token] [吊销] [删除] │
└──────────────────────────┘
```

### UserManage 用户卡片
```
┌──────────────────────────┐
│ 用户名           [管理员] │
│ user@example.com         │
│ 级别: [高级]  自定义: —  │
│ [编辑] [上传自定义]       │
│ [删除自定义] [吊销Token]  │
│ [删除用户]               │
└──────────────────────────┘
```

### RulesManage 规则卡片
```
┌──────────────────────────┐
│ 规则名称                 │
│ 客户端: Shadowrocket  v1 │
│ 更新于: 2026-07-22       │
│ [版本管理] [复制链接]     │
│ [轮替Token]       [删除] │
└──────────────────────────┘
```

### PlatformManage 平台卡片
```
┌──────────────────────────┐
│ 平台名称                 │
│ ID: clash-verge          │
│ Schemes: clash://...     │
│ 下载: https://...        │
│         [编辑] [删除]    │
└──────────────────────────┘
```

## 实施计划

| 块 | 内容 | 涉及文件 |
|----|------|---------|
| 12A | 共享组件加 append-to-body（ConfirmDialog / UploadModal / OIDCSwitchDialog） | 3 个组件 |
| 12B | SubList 卡片化（去 el-table / ActionMenu / useIsMobile，加卡片 grid + append-to-body） | 1 个 view |
| 12C | ShareList 卡片化 | 1 个 view |
| 12D | UserManage 卡片化 | 1 个 view |
| 12E | RulesManage 卡片化 | 1 个 view |
| 12F | PlatformManage 卡片化 | 1 个 view |
| 12G | 表格保留页修 z-index（Logs + 三个版本页） | 4 个 views |
| 12H | OIDCConfig 清理残留 CSS + Home.vue 加 append-to-body | 2 个 views |
| 12I | 全量编译验证 | 全部 |

依赖: 12A → 12B/12C/12D/12E/12F（可并行）→ 12G/12H → 12I

## 文件级改动清单

### 共享组件（append-to-body）
| 文件 | 改动 |
|------|------|
| `ConfirmDialog.vue` | el-dialog 加 `:append-to-body="true"` |
| `UploadModal.vue` | el-dialog 加 `:append-to-body="true"` |
| `OIDCSwitchDialog.vue` | el-dialog 加 `:append-to-body="true"` |

### 卡片化页面（el-table → card grid）
| 文件 | 移除 | 新增/改动 |
|------|------|----------|
| `SubList.vue` | el-table, ActionMenu, useIsMobile | 卡片 grid + 内联按钮 + append-to-body |
| `ShareList.vue` | el-table, ActionMenu, useIsMobile | 卡片 grid + 内联按钮 + append-to-body |
| `UserManage.vue` | el-table, ActionMenu, useIsMobile | 卡片 grid + 内联按钮 + 3×append-to-body |
| `RulesManage.vue` | el-table, ActionMenu, useIsMobile | 卡片 grid + 内联按钮 + append-to-body |
| `PlatformManage.vue` | el-table, ActionMenu, useIsMobile | 卡片 grid + 内联按钮 + append-to-body |

### 表格保留页（仅 append-to-body）
| 文件 | 改动 |
|------|------|
| `Logs.vue` | el-dialog 无（该页无弹窗），无需改动 |
| `SubVersions.vue` | el-dialog + preview dialog 加 `:append-to-body="true"` |
| `ShareVersions.vue` | el-dialog + preview dialog 加 `:append-to-body="true"` |
| `RuleVersions.vue` | el-dialog + preview dialog 加 `:append-to-body="true"` |

### 其他
| 文件 | 改动 |
|------|------|
| `Home.vue` | 复制链接 el-dialog 加 `:append-to-body="true"` |
| `OIDCConfig.vue` | 清理残留 scoped CSS（.oidc-container, .config-card, .card-header-row, .form-tip） |

## 不影响的部分

- **后端**: 零改动（纯前端 UI 重构）
- **API 调用**: 不变（只是展示层从 el-table 改为卡片）
- **路由 / 状态管理**: 不变
- **Manage.vue 布局壳**: 不变（min-w-0 保留）
- **ActionMenu.vue / useIsMobile.js**: 保留不移除（Logs + 版本页仍需要）
- **Rules.vue（用户规则页）**: 不变（非管理页）
- **Setup.vue / Login.vue**: 不变

## 约束

- 卡片的创建/编辑弹窗逻辑不变，仅加 append-to-body
- 桌面端卡片 grid 3 列，平板 2 列，手机 1 列
- 暗色模式自动跟随（复用 Home.vue 的 dark: 类模式）
- 操作按钮用 `flex flex-wrap gap-1` 自然换行，不用 ActionMenu
- 不新增后端 API / 不修改路由
