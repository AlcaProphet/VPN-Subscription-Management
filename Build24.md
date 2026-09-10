# VPN 订阅管理系统 增量构建记录（Build24：R29-06 节点动态表单控件语义收口）

> **文档定位：** 本文档记录用户根据“研究选择框样式优化”结论明确授权实施的 R29-06 独立增量步骤。该修复已由提交 `f3258ee` 实施；本文补齐中断后的构建交接、自动化验收与人工边界，不取代仍在执行的 [Build22.md](Build22.md)，也不改写 [Build23.md](Build23.md) 的历史交接关系。
> - 设计记录：[Design4.md](Design4.md) v1.15 §4.1
> - 问题追踪：[Issue15.md](Issue15.md) R29-06；[Issue14.md](Issue14.md) R28-07H 仅记录随改造删除的遗留颜色类
> - 人工核验：[ProdTestList.md](ProdTestList.md) R29-06
> - 编码指令：[AGENTS.md](AGENTS.md)（唯一强要求）
>
> **用户已确认的决策：** 推荐字段采用标准单值下拉；显式允许自定义时增加“其他（自定义）”，并在下方展示独立内联输入；自定义输入在应用前不写入节点字段，真实运行核验由用户执行。同步检查发现的可选数字、整数列表、列表推荐入口和页面级未应用草稿阻断按推荐方案同批收口。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|---|---|---|---|
| 1 | R29-06 节点动态表单控件语义收口 | Design4 v1.15 §4.1；Issue15 R29-06 | ✅ 代码与自动化验收通过；真实运行待用户核验 |

> 本 Build 仅含一个已经用户明确授权的独立 Step；没有后续候选项，不与 Build22 Step 1～11 合并或交叉扩张。

---

## 二、影响评估与文件清单

| 范围 | 文件 | 处理方式 |
|---|---|---|
| 标准推荐下拉 | `frontend/src/components/EditableCombobox.vue` | 复用 `AppSelect`；有限候选不启用搜索，保留候选元数据、空值候选、`allow_custom` 三态与旧自定义值回显 |
| 递归字段语义 | `frontend/src/components/ProtocolFieldEditor.vue` | 自定义值和新增列表项使用显式草稿；可选数字不再隐式回退到 0；推荐列表可直接追加 |
| 页面级草稿门槛 | `frontend/src/views/admin/NodesView.vue`、`frontend/src/components/NodeCheckPanel.vue` | 未应用控件草稿阻止保存和目标检查；分支切换、作用域清空与取消编辑丢弃草稿 |
| 自动化回归 | `frontend/tests/editable-combobox.spec.ts`、`protocol-field-editor.spec.ts`、`nodes-view.spec.ts`、`node-check-panel.spec.ts` | 覆盖选择、自定义、旧值、空值、数字、列表、保存/检查阻断和输入法合成态 |
| 文档同步 | `Design4.md`、`Issue14.md`、`Issue15.md`、`ProdTestList.md`、`AGENTS.md`、本文 | 记录设计结论、问题状态、工程边界和待用户执行的真实运行清单 |

不修改后端 `FieldSchema`、节点 API、数据库 schema、当前状态、凭据/扩展或目标输出合同。R28-07H 剩余的 `NodeCheckPanel` 预览背景与静态扫描/双主题断言继续由 Issue14 跟踪，不在本 Step 扩张。

---

## 三、Step 1：R29-06 节点动态表单控件语义收口

- **目标：** 让推荐字段具有与协议字段一致的标准选择框外观和交互，同时在不破坏自定义值兼容的前提下，消除可选数字和列表新增的隐式 0，并阻止未应用草稿进入保存或检查。
- **前置条件：** 用户已经确认“标准下拉 + 其他（自定义）+ 独立内联表单”；工作区原始状态无未提交变更；后端 schema 和持久化合同保持不变。
- **核心实现：**
  ```text
  已知候选 -> Select 一次性写入规范值
  其他（自定义） -> 本地草稿 -> 应用后一次性写入 / 取消恢复实际值
  未应用自定义或列表草稿 -> 阻止保存与目标检查
  可选数字空值 -> undefined；显式 0 -> 0
  新增整数列表项 -> 合法整数确认后才追加
  ```
- **验收标准：**
  1. 标准单值 Select 具有箭头和整框展开能力，不启用搜索输入；`allow_custom=false` 不出现“其他”。
  2. “其他”内部值不进入 API/数据库/当前状态/输出；旧自定义值原样回填；输入法合成态 Enter 不误应用。
  3. 空值候选与“其他”不混淆；未设置数字为空，显式 0 保留；整数列表新增不自动产生 0。
  4. ALPN 等列表可从推荐项追加，也可通过独立草稿追加自定义项并保持顺序。
  5. 未应用草稿阻止保存和目标检查；作用域清空与取消编辑不会留下幽灵草稿。
- **自动化验收命令：**
  ```bash
  cd frontend && npm test -- --run tests/editable-combobox.spec.ts tests/protocol-field-editor.spec.ts tests/node-check-panel.spec.ts tests/nodes-view.spec.ts
  cd frontend && npm test
  cd frontend && npm run build
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test ./...
  git diff --check
  ```
- **自动化结果：** 定向前端 4 文件/71 用例通过；前端全量 42 文件/220 用例通过；前端生产构建、后端 build/vet/test 与 `git diff --check` 通过。
- **人工边界：** 浏览器、375px、浅色/深色主题、真实保存/重开和客户端运行结果由用户按 `ProdTestList.md` R29-06 执行；在用户回报前不得写成人工验收通过。

---

## 四、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-10 | 补齐 R29-06 独立增量构建记录：记录用户决策、实施文件、自动化验收和真实运行待用户核验边界；不取代 Build22 当前执行入口。 |
| v1.1 | 2026-09-10 | 根据用户真实运行反馈进一步统一推荐字段与协议入口：候选数量有限，不再启用搜索输入层，关闭及展开状态均保持按钮式 Select；候选、自定义草稿与数据合同不变。 |
