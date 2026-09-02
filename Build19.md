# Build19.md — 节点编辑器前端动态表单、可编辑选项与目标检查 UI 构建计划

> **文档定位：** 本文是 VPN 订阅管理系统第十九轮当前构建方案及实施记录，将 [Design4.md](Design4.md) §三～§七的界面与交互结论转化为可执行的前端实现手册。本 Build 已按 Step 顺序开始实施并同步记录验收结果。
> - 设计依据：[Design4.md](Design4.md)（当前最新设计，已确认作为 Build 依据）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序构建：[Build17.md](Build17.md)（保存契约）、[Build18.md](Build18.md)（FieldSchema/检查接口）；本 Build 假设后端接口已按前两份文档提供。
> - 用户已确认：Trojan 内层 SS 使用 `enabled/method/password`；VMess REALITY 首批不开放；客户端验证只含离线夹具与手工验收待办。
>
> **执行原则：**
> - 复用现有 `FormOverlay`、`FormSection`、`ProtocolFieldEditor`、`AppSelect` 与全局浮层/焦点机制。
> - 后端 `FieldSchema` 是唯一字段来源，不在前端复制协议字段全集。
> - 分支切换只改草稿，保存成功才落库；取消/保存失败不修改数据库。
> - 折叠、切换结构化/JSON 模式不清空有效数据；切换分支/关闭功能按 Design4 §3.4 清空。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|----------|------|
| 1 | 前端类型与 API 客户端扩展 | Design4 §12.2、§12.3 | ✅ 验收通过 |
| 2 | 可编辑下拉/推荐选项组件 | Design4 §4.1 | ✅ 验收通过 |
| 3 | 动态表单分区、条件显示与当前组合摘要 | Design4 §3.1、§3.2 | ✅ 验收通过 |
| 4 | 分支切换清空、凭据状态、局部 JSON 草稿 | Design4 §3.4、§6.3、§6.4 | ✅ 验收通过 |
| 5 | 目标检查 UI 与迟到响应保护 | Design4 §7.2 | ✅ 验收通过 |
| 6 | 保存集成、409 处理与前端测试 | Design4 §12.3 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `frontend/src/api/node.ts` | `FieldSchema` 新字段、`NodeForm`、检查请求/响应类型 |
| 2 | `frontend/src/components/EditableCombobox.vue`（新增）、`AppSelect.vue` 或组件内封装 | 候选/自定义/键盘/失焦不改值 |
| 3 | `frontend/src/views/admin/NodesView.vue` | 协议优先+当前组合摘要、动态分区、独立开关区、高级区 |
| 4 | `frontend/src/components/ProtocolFieldEditor.vue`、`NodesView.vue` | 条件渲染、清空、凭据状态、局部 JSON 草稿 |
| 5 | `frontend/src/views/admin/NodesView.vue`、新增 `NodeCheckPanel.vue` | 检查触发、状态/诊断展示、失效与旧响应丢弃 |
| 6 | `frontend/src/api/node.ts`、`NodesView.vue`、`frontend/tests/*.spec.ts` | 保存 payload、409、动态表单与检查测试 |

---

## 三、构建顺序依赖图

```text
Step 1 类型/API
  → Step 2 可编辑下拉
    → Step 3 动态分区
      → Step 4 清空/凭据/JSON
        → Step 5 检查 UI
          → Step 6 保存/测试
```

依赖理由：

- 前端类型先行，组件才能消费新 schema。
- 可编辑下拉是动态表单的基础交互。
- 清空和 JSON 草稿依赖组件与表单状态模型。
- 检查 UI 依赖后端 `/check` 与当前草稿可见。
- 最后统一保存与回归测试。

---

## 四、分步构建计划

### Step 1：前端类型与 API 客户端扩展

- **目标：** 使前端可以消费 Build18 下发的条件/选项元数据，以及 Build17 的新保存/读取/检查契约。
- **前置条件：** 后端 DTO/字段已按 Build17/18 提供。
- **产出文件与操作：**
  - `frontend/src/api/node.ts`：
    - `FieldSchema` 增加：
      ```ts
      export interface ConditionRule {
        network?: string[]
        security?: string[]
        plugin?: string[]
        features?: string[]
        targets?: string[]
      }
      export interface OptionItem {
        value: string
        label?: string
        group?: string
        verified?: string
      }
      export interface TargetEvidence {
        target: string
        client?: string
        version?: string
        entry?: string
        status: 'complete' | 'equivalent' | 'partial' | 'unsupported' | 'unverified'
      }
      export interface FieldSchema {
        // 既有字段不变...
        group?: 'basic' | 'auth' | 'connection' | 'switches' | 'advanced'
        when?: ConditionRule
        required_when?: ConditionRule
        reset_on?: string[]
        option_items?: OptionItem[]
        allow_custom?: boolean
        canonical_path?: string
        aliases?: string[]
        target_evidence?: TargetEvidence[]
      }
      ```
    - `NodeItem` 增加 `edit_revision`、`state_format_version`、`current_state`、`extensions`。
    - `NodeForm` 增加：
      ```ts
      export interface NodeForm {
        name: string
        protocol: string
        host: string
        port: number
        protocol_json: Record<string, unknown>
        current_state?: CurrentState
        base_revision?: number
        reset_scopes?: string[]
        credential_ops?: CredentialOp[]
        extension_ops?: ExtensionOp[]
      }
      export interface CurrentState {
        network?: string
        security?: string
        plugin?: string | null
        features?: string[]
      }
      export interface CredentialOp { path: string; op: 'keep' | 'clear' }
      export interface ExtensionOp { op: 'keep' | 'replace' | 'clear' | 'add'; id?: string; scope?: string; targets?: string[]; label?: string; payload?: string }
      ```
    - 新增检查方法：
      ```ts
      export const checkNode = (data: NodeCheckRequest) =>
        http.post<any, NodeCheckResponse>('/admin/nodes/check', data)
      ```
    - 列表/详情类型带上扩展摘要。
- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run src 2>/dev/null || true
  git diff --check
  ```
  实际前端单测在 Step 6 统一补充；本步以 TypeScript 编译通过为准。
- **验收标准：**
  - TS 类型可编译。
  - 现有调用未被破坏（字段均为可选兼容）。

---

### Step 2：可编辑下拉/推荐选项组件

- **目标：** 为单值候选字段提供“推荐选项优先 + 可搜索 + 支持自定义 + 不自动改写旧值”的 Combobox。
- **前置条件：** Step 1 类型可用。
- **产出文件与操作：**
  - 新增 `frontend/src/components/EditableCombobox.vue`：
    - Props：`value`、`items: OptionItem[]`、`allowCustom`、`placeholder`、`disabled`。
    - 行为：
      - 空状态可打开浏览选项，输入过滤（大小写不敏感、显示名与规范值均可匹配）。
      - 没有匹配时展示“使用自定义值：<输入>”按钮。
      - 只有选择候选或明确点击“使用自定义值”才 `update:modelValue`。
      - 失焦时不自动改成第一项、不自动改成 `auto`/空值。
      - 支持 Escape 关闭、方向键选择、焦点回归；遵循 W3C 可编辑 Combobox 的可访问性要求，并接入现有单浮层管理。
      - 自定义值保留原字符串，不允许变成多选标签。
    - 对旧值回显：若 `value` 不在候选列表，仍显示原值并允许用户选择后覆盖；若用户未操作，保存原值。
  - `ProtocolFieldEditor.vue`：
    - 当 `field.type === 'select'` 且 `field.option_items` 存在时，使用 `EditableCombobox`；否则兼容旧 `AppSelect`。
    - 当 `field.allow_custom` 为 false 时，只允许候选选择。
    - 当 `field.type === 'text-list'`/`int-list` 仍保留逗号输入，但若有 `option_items`，可在高级/提示中展示推荐条目（可后置）。
- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run editable-combobox 2>/dev/null || true
  ```
- **验收标准：**
  - 可搜索、选择、自定义、回显、清空正常。
  - 输入法/键盘/Escape 可用。
  - 未确认输入不会写入 `protocol_json`。
  - 旧值不会因失焦/刷新被替换。

---

### Step 3：动态表单分区、条件显示与当前组合摘要

- **目标：** 按协议+当前连接组合动态展示字段，保持 ProtocolFieldEditor 递归能力，形成“基础/认证/当前连接/开关/高级/检查”的信息架构。
- **前置条件：** Step 2 可编辑下拉可用。
- **产出文件与操作：**
  - `frontend/src/views/admin/NodesView.vue`：
    - 顶部显示当前组合摘要：
      ```text
      当前组合：VLESS · WebSocket · TLS
      ```
      由 `currentSchema` + 表单当前值实时计算。
    - 分区：
      1. 基本信息：协议、名称、服务器、端口（固定）。
      2. 认证与密钥：`section === 'auth'` 或 `group === 'auth'`。
      3. 连接方式与当前参数：`group === 'connection'` 或 `section === 'transport'`/`security` 且满足 `when`。
      4. 独立开关区：`group === 'switches'` 或 `section === 'switches'`，只显示当前协议与组合适用的 bool；常用直接展示，更多开关放在区内折叠。
      5. 更多功能/兼容/性能：`group === 'advanced'`，默认折叠；已配置项显示摘要。
      6. 高级数据与目标检查：未知扩展摘要、局部 JSON、脱敏检查。
    - 新增 `fieldVisible(field)`：根据 `field.when` 与当前 `currentState` 判断；`targets` 存在时仅目标检查上下文内过滤，不影响编辑显示。
    - 新增 `sectionFieldsByGroup(group)`/`visibleFields`，替代旧的固定六区 `sectionFields`。
    - 传输/安全/插件选择使用 `Select` 或 `EditableCombobox`；多选一不并用多个 bool。
  - 移动端：维持 `FormOverlay` 全屏 Drawer；单列、标签说明上下排列；复杂对象跨列仅在桌面。
- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run nodes-view 2>/dev/null || true
  ```
- **验收标准：**
  - 切换协议后表单显示对应协议字段，不残留上一协议参数。
  - 切换传输后显示对应传输字段，隐藏被切离字段。
  - 独立开关区只包含当前适用的 bool。
  - 高级区默认折叠但有配置摘要。
  - 桌面双列/移动单列无横向溢出。

---

### Step 4：分支切换清空、凭据状态、局部 JSON 草稿

- **目标：** 实现 Design4 §3.4 的清空范围与 §6.3/§6.4 的凭据/JSON 草稿行为。
- **前置条件：** Step 3 动态分区。
- **产出文件与操作：**
  - `NodesView.vue`：
    - 维护 `resetScopes` 集合（`Set<string>`），记录本次编辑已发生切换/关闭的作用域。
    - 切换协议：清空 `protocol_json` 中全部协议参数与凭据、扩展、局部 JSON 草稿；保留 name/host/port；添加 `reset_scopes: ['protocol']` 或按服务端约定覆盖全部。
    - 切换 network：清空 `reset_on` 含 `network` 的字段、子对象、扩展；添加 `network` 到 `resetScopes`；不恢复旧值。
    - 切换 security：清空安全分区；添加 `security`。
    - 切换/取消 SS plugin：清空插件参数与插件扩展；添加 `plugin`；SS 主密码不清空。
    - 关闭有子配置功能：清空该 feature 子参数；添加 `feature.<name>`。
    - 每次清空同时：
      - 移除对应路径的 `invalidProtocolPaths`；
      - 使目标检查结果失效；
      - 放弃对应范围的未应用 JSON 草稿。
    - 提示确认：切换前显示 Toast/Confirm，例如“切换将清空该分区参数，切回需重新填写”。
  - `ProtocolFieldEditor.vue`：
    - 支持 `credentialState`（`'unset' | 'saved' | 'replacing'`）或 emit 凭据状态：
      - 普通编辑未重置的敏感字段显示“已保存（留空保留）”。
      - 被重置后显示“未配置”，不允许留空隐式保留。
      - 用户输入非空视为“替换”，提交时作为明文新值。
    - 局部 JSON：
      - 每个对象维持“结构化 / 高级 JSON”切换；切换模式不清空已应用有效数据。
      - JSON 编辑为草稿：修改后未应用时，保存节点前需明确“应用”或“放弃”；放弃后恢复原结构化值。
      - 分支切换/清空时丢弃该范围的 JSON 草稿；禁止旧草稿重新应用。
  - 保存 payload：
    - `current_state` 由当前表单计算；
    - `reset_scopes` 从 `resetScopes` 生成数组；
    - `credential_ops` 对“清除”的敏感路径显式 `clear`；未操作敏感路径不发送，服务端默认保留；
    - `extension_ops` 对未知扩展的增删改显式发送。
- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run protocol-field-editor nodes-view 2>/dev/null || true
  ```
- **验收标准：**
  - WS → gRPC → WS 不恢复 WS 参数/凭据。
  - 插件 A → B → A 不恢复插件参数/密码。
  - 取消编辑或保存失败不写库，数据库中无草稿。
  - 折叠/结构化/JSON 切换不清空有效值。
  - 被重置凭据不显示“已保存”，输入空值不能回填旧密文。

---

### Step 5：目标检查 UI 与迟到响应保护

- **目标：** 在高级区提供按需脱敏目标检查，展示目标状态与诊断；旧响应不得覆盖新草稿。
- **前置条件：** Step 3/4 表单与清空/凭据状态可用；后端 `/check` 已提供。
- **产出文件与操作：**
  - 新增 `frontend/src/components/NodeCheckPanel.vue`（或内嵌在 `NodesView.vue`）：
    - 按钮“检查当前节点”进入高级区。
    - 请求体使用当前草稿 + `node_id`/`base_revision` + `reset_scopes` + `credential_ops` + `current_state` + `targets`（默认 clash-yaml、sr-subs、generic-subs）。
    - 展示每个目标的状态徽标（`ok/warn/skip/error`）与诊断列表；诊断按 `error`、`warn`、`info` 排序。
    - 脱敏产物用 `<details>` 展示，明确“脱敏片段，不能直接连接”。
    - 检查结果只用于提示，不自动保存节点。
  - `NodesView.vue`：
    - 维护 `checkSeq` 或 `checkId`。
    - 当 form 中影响输出的字段、协议、current_state、reset_scopes、credential_ops、extension_ops 变化时，立即将已有检查结果标记为失效/隐藏。
    - 响应回来时，若 `checkSeq` 不是最新，则丢弃；不使用后端 `check_id` 覆盖新结果。
    - 保存成功后清除检查面板（若重新编辑再检查）。
  - 若检查返回 409（编辑 revision 冲突），跳转到保存冲突处理（Step 6）。
- **测试与验收命令：**
  ```bash
  cd frontend && npm run build
  cd frontend && npm test -- --run node-check 2>/dev/null || true
  ```
- **验收标准：**
  - 新建草稿无需节点 ID 即可检查。
  - 编辑草稿携带正确 base_revision；检查不写库。
  - 迟到响应不覆盖新草稿的检查结果。
  - 脱敏产物不显示凭据明文或扩展密文。
  - 用户不保存节点也能看到差异诊断。

---

### Step 6：保存集成、409 处理与前端测试

- **目标：** 将新保存契约接入前端保存流程，并补齐关键自动化测试。
- **前置条件：** Step 1～5 完成。
- **产出文件与操作：**
  - `frontend/src/api/node.ts`：`createNode`/`updateNode` 入参使用扩展 `NodeForm`。
  - `NodesView.vue`：
    - 保存：
      - 新建：提交 `current_state`、`extensions`、`protocol_json`；
      - 编辑：提交 `base_revision: node.edit_revision`、`current_state`、`reset_scopes`、`credential_ops`、`extension_ops`。
    - 409 处理：
      - 捕获 `ApiError.status === 409`；
      - 保留当前草稿，弹出提示“节点已被其他编辑更新，请重新加载后重试”；
      - 提供“重新加载”按钮：重新拉取节点详情并覆盖表单（或丢弃草稿），不自动合并。
    - 保存成功：
      - 重新加载列表；
      - 清空 `resetScopes`、检查结果、JSON 草稿；
      - 显示成功提示。
  - 更新 `frontend/tests/nodes-view.spec.ts`：
    - 动态协议切换清空参数且保留基础信息。
    - 敏感字段被重置后不再显示“留空保留”。
    - 保存调用包含 `base_revision`、`reset_scopes`、`current_state`。
    - 409 时保留草稿并显示冲突提示。
  - 更新 `frontend/tests/protocol-field-editor.spec.ts`：
    - 可选下拉自定义值不失焦改写。
    - JSON 未应用草稿在分支切换后丢弃。
  - 新增 `frontend/tests/node-check-panel.spec.ts`：
    - 检查请求携带正确草稿；
    - 修改字段后旧结果失效；
    - 迟到响应不更新 UI。
- **测试与验收命令：**
  ```bash
  cd frontend && npm test -- --run
  cd frontend && npm run build
  git diff --check
  ```
- **验收标准：**
  - 全部前端测试与生产构建通过。
  - 保存 payload 符合 Build17/18 的 API 契约。
  - 409 不丢草稿，不自动合并。
  - 动态表单、清空、JSON、检查与响应式均有所覆盖。

---

## 五、关键状态模型

```text
form.current_state        → 当前协议/network/security/plugin/features
resetScopes               → 本次编辑已发生切换/关闭的 scope
credentialStateByPath     → unset / saved / replacing / cleared
localJsonDrafts           → 按 fieldPath 保存未应用 JSON 文本
checkSeq + checkResult    → 检查请求序号和当前有效结果
```

所有状态仅存在当前编辑会话；保存成功后才进入数据库，取消/失败不持久化。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-02 | 初始构建方案：前端类型、可编辑下拉、动态分区、分支清空/凭据/JSON、目标检查 UI 与保存/409 集成。仅创建 Build 文档，未构建代码。 |
| v1.1 | 2026-09-02 | 完成 Step 1：前端 `FieldSchema`/`NodeItem`/`NodeForm`/检查类型与 `checkNode` API 扩展；`npm run build` 通过。 |
| v1.2 | 2026-09-02 | 完成 Step 2：新增 `EditableCombobox.vue`，支持候选/自定义/过滤/失焦不改写/浮层管理，并在 `ProtocolFieldEditor.vue` 的 `select+option_items` 场景接入；新增 `editable-combobox.spec.ts`，5 项单测与构建通过。 |
| v1.3 | 2026-09-02 | 完成 Step 3：`NodesView.vue` 改为按 group/when 动态分区，新增当前组合摘要、连接方式区、独立开关区与默认折叠的高级区；条件显示与 section 回退兼容；现有 nodes 测试更新并通过，前后端构建通过。 |
| v1.4 | 2026-09-02 | 完成 Step 4：节点编辑维护 `resetScopes`/`clearedSensitivePaths`，分支切换清空所属字段并随保存生成 `reset_scopes`/`credential_ops`；`ProtocolFieldEditor` 支持凭据“已保存/未配置”状态与高级 JSON“应用/放弃”草稿；新增分支清空、草稿、凭据状态测试，构建与定向测试通过。 |
| v1.5 | 2026-09-02 | 完成 Step 5：新增 `NodeCheckPanel.vue`，接入当前草稿目标检查、状态/诊断/脱敏预览展示，草稿变化即失效、迟到响应丢弃；新增 `node-check-panel.spec.ts` 3 项测试，构建与定向测试通过。 |
| v1.6 | 2026-09-02 | 完成 Step 6：保存 payload 接入 `current_state`/`reset_scopes`/`credential_ops`/`base_revision`，409 保留草稿并支持重新加载；补充保存与 409 测试；前端全量 `npm test -- --run` 39 文件 / 158 用例通过，`npm run build` 通过。 |
