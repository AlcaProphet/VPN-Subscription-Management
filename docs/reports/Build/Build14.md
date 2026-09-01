# Build14.md — R24 第二批已决策问题修复构建记录

> **文档定位：** 本文记录 `Issue9.md` 中已由用户确认并实施的 R24-09、R24-10、R24-11、R24-13、R24-14、R24-16、R24-17 修复及验收结果；R24-19、R24-20 继续暂缓。
> - 当前设计：`Design2.md`、`Design2-UI.md`
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 问题追踪：[Issue9.md](../../../Issue9.md)
> - 前一轮构建：[Build13.md](Build13.md)
> - 实施日期：2026-08-30

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 1 | R24-09/R24-10：规则列表精简与首页默认操作收口 | ✅ 验收通过 |
| 2 | R24-11：订阅版本页隐藏类型 Tag | ✅ 验收通过 |
| 3 | R24-13：代理组高级开关单行 | ✅ 验收通过 |
| 4 | R24-14：装配目标 Tab 面向用户文案 | ✅ 验收通过 |
| 5 | R24-16：头部高级 JSON Switch 与默认值警示 | ✅ 验收通过 |
| 6 | R24-17：节点/代理组选区卡片化与中文术语 | ✅ 验收通过 |
| 7 | 前端全量测试、构建与文档同步 | ✅ 已完成 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、范围与未处理项

本构建实施以下 7 项：

- R24-09：管理员规则列表移除「客户端类型」列及移动端重复 Tag
- R24-10：移除「首页默认」单选列，操作区统一提供「设为首页默认 / 取消默认」
- R24-11：订阅版本页隐藏页头类型 Tag，其它资源类型保留
- R24-13：代理组高级开关统一单行并响应式降列
- R24-14：装配四个目标 Tab 改为面向用户文案，内部枚举不变
- R24-16：高级 JSON 改为 Switch，默认值入口改为「使用默认值」警示样式并保留覆盖确认
- R24-17：节点/代理组选区改为分区卡片、状态摘要与中文术语

不在本次范围内：

- R24-19：节点弹窗排版，继续暂缓。
- R24-20：协议对象参数结构化编辑，继续暂缓。

---

## 三、实施内容

### Step 1：规则列表精简与首页默认操作收口

- **改动文件：**
  - `frontend/src/views/admin/RulesView.vue`
  - `frontend/tests/rules-view.spec.ts`（新增）
- **实现：**
  - 移除桌面端「客户端类型」列和移动端重复 `client_type` Tag，保留创建表单唯一类型约束、后端字段和用户端展示。
  - 移除桌面端「首页默认」Radio 列及移动端名称旁 Radio；操作区对非默认行显示「设为首页默认」、默认行显示「取消默认」，继续沿用原有 ConfirmModal 与 `setHomeDefault` 后端语义。

### Step 2：订阅版本页隐藏类型 Tag

- **改动文件：**
  - `frontend/src/views/admin/VersionManageView.vue`
  - `frontend/tests/version-manage-view.spec.ts`
- **实现：** 仅当 `ownerType === 'subscription'` 时隐藏页头 `owner.type_label` Tag；规则/分享/自定义订阅版本页仍保留。

### Step 3：代理组高级开关单行

- **改动文件：**
  - `frontend/src/views/admin/ProxyGroupsView.vue`
- **实现：** 高级开关项统一为 `flex items-center` + `whitespace-nowrap` 单行布局，网格从四列改为响应式 `1/2/3` 列，避免「引入全部 Provider」等长文案折行或横向溢出。

### Step 4：装配目标 Tab 面向用户文案

- **改动文件：**
  - `frontend/src/views/admin/AssemblyView.vue`
  - `frontend/tests/assembly-view.spec.ts`
- **实现：** 新增 `SUB_TAB_LABELS` 单向展示映射，Tab 显示为 `Clash - V2Ray/Mihomo（新版）`、`Shadowrocket 订阅组`、`通用V2Ray格式`、`Shadowrocket规则组`；路由、TargetSyntax、API 与后端枚举不变。

### Step 5：头部高级 JSON Switch 与默认值警示

- **改动文件：**
  - `frontend/src/views/admin/assembly/HeaderStep.vue`
  - `frontend/src/views/admin/AssemblyView.vue`
  - `frontend/tests/header-step.spec.ts`
- **实现：** 「高级 JSON / 结构化表单」改为带双态标签的 Switch；「一键采用默认值」改为「使用默认值」警示按钮，确认弹窗保留并明确为整体覆盖当前头部参数。

### Step 6：节点/代理组选区卡片化与中文术语

- **改动文件：**
  - `frontend/src/views/admin/assembly/NodesGroupsStep.vue`
  - `frontend/tests/nodes-groups-step.spec.ts`
- **实现：** “手动添加的节点”“Xray 节点”“代理组”改为 Section/Card 容器，显示已选/总数摘要；选项行统一边框、已选/不可用/强制/自建状态区分；`manual 节点`及空态改为中文；移动端降为单/自适应列。

---

## 四、验证记录

| 范围 | 命令 | 结果 |
|------|------|------|
| 前端构建 | `cd frontend && npm run build` | 通过 |
| 前端全量测试 | `cd frontend && npm test -- --run` | 通过，34 个测试文件、122 项测试 |
| 定向测试 | `rules-view / header-step / assembly-view / nodes-groups-step / version-manage-view / proxy-groups-view` | 通过 |

---

## 五、文档同步

- `Design2-UI.md` 已同步：
  - §4.2 订阅版本页类型 Tag 隐藏
  - §4.6 规则列表客户端类型列移除、首页默认操作收口
  - §5.3.0 目标类型 Tab 文案
  - §5.3.1～5.3.3 高级 JSON Switch、使用默认值、节点/代理组选区视觉
  - §7.2 高级开关统一单行
- `Issue9.md` 已将 R24-09/10/11/13/14/16/17 更新为已实施并补充分变更记录。
- `AGENTS.md` 已同步当前构建/问题修复记录指向。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-30 | 实施并验收 R24-09/10/11/13/14/16/17；记录范围、实现、验证与文档同步；R24-19/20 继续暂缓。 |
