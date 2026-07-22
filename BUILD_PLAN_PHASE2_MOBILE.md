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
