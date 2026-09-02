# Build17.md — 节点编辑统一保存契约与 nodes 行内当前状态构建计划

> **文档定位：** 本文是 VPN 订阅管理系统第十七轮当前构建方案（Build 文档，非强规则），将 [Design4.md](Design4.md) 第十二章已确认的契约转化为可逐步执行和验收的实现手册。本文**只编写构建方案，不构建代码**。
> - 设计依据：[Design4.md](Design4.md) §二、§六、§10、§12（当前最新设计，已确认作为 Build 依据）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序基线：已存档 [Build16.md](docs/reports/Build/Build16.md)（已完成）；节点编辑现状见 [Build15.md](docs/reports/Build/Build15.md)
> - 存放说明：按用户确认，Build17～Build20 存放在仓库根目录，承接 Design4 的构建拆分。
>
> **执行原则：**
> - 每次仅执行一个 Step；完成编译、测试和差异检查并确认后再进入下一步。
> - 不并行实施多步，不顺手重构节点、代理组、Xray、权限等非本轮范围。
> - 每一步必须可编译、可测试；临时兼容投影必须在后续收口 Step 删除。
> - 本 Build 不实现前端；前端动态表单在 [Build19.md](Build19.md) 承接，节点检查与目标诊断在 [Build18.md](Build18.md) 承接。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|----------|------|
| 1 | 1017 迁移：`nodes` 新增当前状态/扩展/修订列 | Design4 §12.1 | ☐ 未开始 |
| 2 | 节点领域类型与存取层扩展 | Design4 §12.1、§12.3 | ☐ 未开始 |
| 3 | 创建/更新/读取 API 契约与修订冲突 | Design4 §12.3 | ☐ 未开始 |
| 4 | 凭据操作、扩展加密与保存流程测试 | Design4 §6.3、§6.4、§12.1 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/migrations/1017_node_editor_state.sql` | `edit_revision`、`state_format_version`、`current_state_json`、`extensions_json` |
| 2 | `backend/internal/node/node.go`、`backend/internal/server/node.go` | `Node`/输入 DTO/扫描/脱敏/错误扩展 |
| 3 | `backend/internal/node/node.go`、`backend/internal/server/node.go` | create/update/read、409 修订冲突、列表附带修订号 |
| 4 | `backend/internal/node/node.go`、`backend/internal/node/node_test.go`、`backend/internal/config/export.go`、`backend/internal/config/export_test.go` | reset/credential/extensions 加密、配置导入密钥保护与事务测试 |

---

## 三、构建顺序依赖图

```text
Step 1 数据列落库
  → Step 2 领域类型与扫描
    → Step 3 创建/更新/读取 API
      → Step 4 凭据与扩展操作/测试收口
```

依赖理由：

- 数据库列必须先存在，扫描与服务读写才可引用。
- 类型与扫描稳定后，API 才能按契约返回脱敏详情。
- 凭据/扩展操作依赖新的入参与事务边界，最后统一补测试。

---

## 四、分步构建计划

### Step 1：1017 迁移：`nodes` 新增当前状态/扩展/修订列

- **目标：** 在 `nodes` 同一行补充保存最小当前状态、未知扩展、格式版本与编辑修订号所需列；不新增 `node_edit_states` 表，不保存非激活分支或恢复副本。
- **前置条件：** 当前数据库允许新迁移；项目当前无业务数据；仅需保证新库初始化、迁移版本化路径正确。
- **产出文件与操作：**
  - 新建 `backend/migrations/1017_node_editor_state.sql`，内容参照下列 DDL：

    ```sql
    -- 1017_node_editor_state.sql — Build17 节点编辑统一保存契约。
    -- 在 nodes 同一行保存最小当前状态、扩展负载、格式版本与编辑修订号。
    -- 不创建独立 node_edit_states 表，不保存非激活分支、恢复副本或折叠状态。

    ALTER TABLE nodes ADD COLUMN edit_revision INTEGER NOT NULL DEFAULT 0;
    ALTER TABLE nodes ADD COLUMN state_format_version INTEGER NOT NULL DEFAULT 1;
    ALTER TABLE nodes ADD COLUMN current_state_json TEXT NOT NULL DEFAULT '{}';
    ALTER TABLE nodes ADD COLUMN extensions_json TEXT NOT NULL DEFAULT '{}';
    ```

  - 如后续需要索引，仅允许为 `edit_revision` 建立普通索引；禁止对 JSON 列建立全文/表达式索引。默认不建索引。
- **约束说明：**
  - `edit_revision` 每次成功保存 manual 配置 +1；Xray 来源节点不使用本列作为人工编辑边界。
  - `state_format_version` 当前恒为 1，表示 `current_state_json` 的解释版本。
  - `current_state_json` 只保存当前激活组合：`network`、`security`、`plugin`、`features`。
  - `extensions_json` 保存未知扩展元数据与加密负载，读取时只返回摘要。
- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/store -run 'TestMigrate' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - 新库执行迁移后 `nodes` 四列存在，默认值正确。
  - 重复迁移幂等（迁移框架已应用后跳过）。
  - 不改变 `protocol_json` 的既有含义，不新增独立状态表。

---

### Step 2：节点领域类型与存取层扩展

- **目标：** 让后端读写代码识别新列，并定义创建/更新/读取所需的最小类型；前端类型同步在 Build19，不在本步改动前端。
- **前置条件：** Step 1 验收通过。
- **产出文件与操作：**
  - `backend/internal/node/node.go`：
    - `Node` 增加字段：
      ```go
      EditRevision       int64          `json:"edit_revision"`
      StateFormatVersion int            `json:"state_format_version"`
      CurrentState       CurrentState   `json:"current_state"`
      Extensions         []ExtensionSummary `json:"extensions"`
      ```
    - 新增类型定义（参考结构，Build 实现时可微调字段名但必须保持 JSON 语义一致）：
      ```go
      type CurrentState struct {
          Network  string   `json:"network,omitempty"`
          Security string   `json:"security,omitempty"`
          Plugin   *string  `json:"plugin"`
          Features []string `json:"features,omitempty"`
      }

      type CredentialOp struct {
          Path string `json:"path"`
          Op   string `json:"op"` // keep|clear
      }

      type ExtensionOp struct {
          Op      string `json:"op"`      // keep|replace|clear|add
          ID      string `json:"id,omitempty"`
          Scope   string `json:"scope,omitempty"`
          Targets []string `json:"targets,omitempty"`
          Label   string `json:"label,omitempty"`
          Payload string `json:"payload,omitempty"`
      }

      type ExtensionSummary struct {
          ID         string   `json:"id"`
          Scope      string   `json:"scope"`
          Targets    []string `json:"targets,omitempty"`
          Label      string   `json:"label,omitempty"`
          Configured bool     `json:"configured"`
      }

      type ExtensionRecord struct {
          ID         string   `json:"id"`
          Scope      string   `json:"scope"`
          Targets    []string `json:"targets,omitempty"`
          Label      string   `json:"label,omitempty"`
          Status     string   `json:"status"` // encrypted
          PayloadEnc string   `json:"payload_encrypted,omitempty"` // 内部存储
      }
      ```
    - `CreateManualInput` 扩展：
      ```go
      type CreateManualInput struct {
          Name         string                 `json:"name"`
          Protocol     string                 `json:"protocol"`
          Host         string                 `json:"host"`
          Port         int                    `json:"port"`
          ProtocolJSON map[string]any         `json:"protocol_json"`
          CurrentState *CurrentState          `json:"current_state,omitempty"`
          Extensions   []ExtensionInput       `json:"extensions,omitempty"`
      }

      type ExtensionInput struct {
          Scope   string   `json:"scope"`
          Targets []string `json:"targets"`
          Label   string   `json:"label"`
          Payload string   `json:"payload"`
      }
      ```
    - `UpdateManualInput` 扩展：
      ```go
      type UpdateManualInput struct {
          Name           string           `json:"name"`
          Protocol       string           `json:"protocol"`
          Host           string           `json:"host"`
          Port           int              `json:"port"`
          ProtocolJSON   map[string]any   `json:"protocol_json"`
          CurrentState   *CurrentState    `json:"current_state,omitempty"`
          BaseRevision   int64            `json:"base_revision"`
          ResetScopes    []string         `json:"reset_scopes,omitempty"`
          CredentialOps  []CredentialOp   `json:"credential_ops,omitempty"`
          ExtensionOps   []ExtensionOp    `json:"extension_ops,omitempty"`
      }
      ```
  - `scanNode`：
    - 查询 SQL 增加 `n.edit_revision, n.state_format_version, n.current_state_json, n.extensions_json`。
    - 读取 JSON 列并解析到 `CurrentState` 与内部扩展记录；对外转换为 `ExtensionSummary`（不返回 `payload_encrypted`）。
  - `List`/`Get`/`getRaw` 同步新列；`getRaw` 保留内部密文供更新合并使用。
  - `redactSensitive` 保持现有敏感字段脱敏；同时增加 `redactExtensions`：只返回摘要。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go
  cd backend && go test ./internal/node -run 'TestNode|TestCreateManual|TestUpdateManual' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - 新建/读取节点后 `CurrentState`、`Extensions`、`EditRevision`、`StateFormatVersion` 正确映射。
  - 列表接口也能返回 `edit_revision`。
  - 扩展摘要不包含 `payload_encrypted`。

---

### Step 3：创建/更新/读取 API 契约与修订冲突

- **目标：** 实现 Design4 §12.3 的创建、更新和读取契约；旧修订保存返回 409，且不写任何字段。
- **前置条件：** Step 2 类型与扫描通过。
- **产出文件与操作：**
  - `backend/internal/node/node.go`：
    - 新增错误变量：
      ```go
      var ErrRevisionConflict = errors.New("节点已被其他编辑更新，请重新加载后重试")
      ```
    - `CreateManual`：
      1. 校验名称、host/port、协议、`protocol_json`。
      2. 若 `CurrentState == nil`，由服务端按 `protocol_json` 派生最小当前状态（保留 `network/security/plugin/features` 中的当前值；不得猜测未激活分支）。
      3. 校验 `CurrentState` 与 `protocol_json` 基本一致（network/security/plugin 与当前协议合法值匹配）。
      4. 加密 `protocol_json` 敏感字段；加密扩展块（见 Step 4）。
      5. 在同一 `TxImmediate` 中写入：
         ```sql
         INSERT INTO nodes (
           source, name, display_name, instance_id, tag, protocol, host, port,
           protocol_json, current_state_json, extensions_json,
           edit_revision, state_format_version,
           is_public, enabled, allocatable, missing
         ) VALUES (
           'manual', ?, NULL, NULL, '', ?, ?, ?,
           ?, ?, ?, 1, 1,
           0, 1, 1, 0
         )
         ```
      6. 返回脱敏详情，`edit_revision=1`。
    - `UpdateManual`：
      1. 事务前读取 `existing`；非 manual 返回 `ErrForbidden`。
      2. 校验 `in.BaseRevision == existing.EditRevision`，否则返回：
         ```go
         fmt.Errorf("%w: current_revision=%d", ErrRevisionConflict, existing.EditRevision)
         ```
      3. 校验名称只读、host/port、协议、`protocol_json`。
      4. 构造“合并基底”：
         - 从 `existing.ProtocolJSON` 深拷贝；
         - 按 `reset_scopes` 清除作用域内参数、凭据、扩展；支持作用域 `protocol`、`network`、`security`、`plugin`、`feature.<name>`；
         - 按 `credential_ops` 对明确路径执行 `keep`（保留旧密文）或 `clear`（删除旧值）；没有出现在 `credential_ops` 的敏感字段按普通字段处理；
         - 将 `in.ProtocolJSON` 的新值合并到基底；
         - 再加密敏感字段，覆盖旧密文。
      5. 处理 `extension_ops`：
         - `keep` 只能保留未被重置作用域删除且 ID 存在的扩展；
         - `clear` 删除指定 ID；
         - `replace` 用同 ID 新明文替换负载；
         - `add` 新建扩展并加密。
      6. 在同一 `TxImmediate` 中执行：
         ```sql
         UPDATE nodes
         SET protocol = ?, host = ?, port = ?, protocol_json = ?,
             current_state_json = ?, extensions_json = ?,
             edit_revision = edit_revision + 1,
             state_format_version = ?,
             updated_at = CURRENT_TIMESTAMP
         WHERE id = ? AND edit_revision = ?
         ```
         若影响行数为 0，返回 `ErrRevisionConflict`。
      7. 重新读取并脱敏返回。
  - `backend/internal/server/node.go`：
    - create/update 绑定新 DTO。
    - update 捕获 `ErrRevisionConflict`，返回 409 与 `current_revision`：
      ```json
      { "error": "节点已被其他编辑更新，请重新加载后重试", "code": "revision_conflict", "current_revision": 9 }
      ```
    - 由于现有 `response.Fail` 仅返回 `message`，本步可在 `Fail` 之外增加专用冲突响应或在 `Fail` 扩展 `data`；推荐先最小实现专用 `c.JSON(409, gin.H{...})`，保持现有错误码约定。
  - `GET /api/admin/nodes/:id` 与列表接口返回新增字段。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go internal/server/*.go
  cd backend && go test ./internal/node ./internal/server -run 'Test.*Node|Test.*Revision|Test.*Conflict' -count=1
  cd backend && go build ./...
  cd backend && go vet ./internal/node ./internal/server
  git diff --check
  ```
- **验收标准：**
  - 创建返回 `edit_revision=1`，`state_format_version=1`，扩展摘要正确。
  - 更新成功后 `edit_revision` +1，`current_state_json` 保存当前选择。
  - 旧 `base_revision` 更新返回 409，数据库无任何字段变化。
  - 更新使用 `WHERE id AND edit_revision` 原子保护，杜绝旧页面覆盖。
  - 读取接口返回脱敏 `protocol_json` 与扩展摘要，不返回扩展密文。

---

### Step 4：凭据操作、扩展加密与保存流程测试

- **目标：** 补齐 `reset_scopes`、`credential_ops`、`extension_ops` 的服务端正确性与测试，使保存契约可被 Build18 检查接口复用。
- **前置条件：** Step 3 的读写路径可编译。
- **产出文件与操作：**
  - `backend/internal/node/node.go`：
    - 新增扩展加密辅助：
      ```go
      const extEncPrefix = "enc:ext:v1:"

      func (s *Service) encryptExtensionPayload(ctx context.Context, plain string) (string, error) {
          key, err := s.cfg.Get(ctx, config.KeySigningKey)
          if err != nil || key == "" {
              return "", errors.New("签名密钥未配置，无法加密节点扩展")
          }
          b, err := config.Encrypt([]byte(plain), []byte(key))
          if err != nil {
              return "", err
          }
          return extEncPrefix + b, nil
      }
      ```
    - 新增清理函数：
      ```go
      func clearScope(m map[string]any, scope string, schema []FieldSchema) map[string]any
      ```
      规则：
      - `protocol`：清空全部协议级参数（保留基础字段由调用方控制）。
      - `network`：清空 `transport`/`connection` 分组中 `reset_on` 包含 `network` 的字段及其子对象。
      - `security`：清空安全分组中 `reset_on` 包含 `security` 的字段及其子对象。
      - `plugin`：清空插件相关字段、`plugin-opts` 子对象与插件扩展。
      - `feature.<name>`：清空该功能声明的子参数。
    - `mergeSensitive` 改为基于“清空后的基底 + 本次新值 + cred ops”执行，保留 `keep` 语义，禁止 `clear` 后回填。
    - 扩展块处理：
      - 读取 `extensions_json` 到 `[]ExtensionRecord`；
      - 每次保存后，以 `id` 为稳定标识写入新列表，删除被 reset/clear 的条目；
      - 只有 `add`/`replace` 的明文负载被加密；`keep` 保留原密文；
      - 校验 `scope` 只能是 `node` / `transport.<network>` / `security.<security>` / `plugin.<plugin>` / `feature.<feature>`，不属于当前分支的扩展不得保存到公共区。
  - `backend/internal/config/export.go` 与 `backend/internal/config/export_test.go`：
    - `checkImportProtection` 当前仅扫描 `protocol_json LIKE '%enc:v1:%'`；本步必须扩展为同时检查 `extensions_json LIKE '%enc:ext:v1:%'`，否则更换签名密钥导入会破坏扩展密文且不被拦截。
    - 增加测试：新建节点带加密扩展后，导入不同签名密钥的配置应被拒绝；同密钥往返不受影响。
  - `backend/internal/node/node_test.go` 新增测试用例：
    - `TestCreateWithCurrentStateAndExtensions`：创建后库内无扩展明文，读取仅摘要。
    - `TestUpdateResetScopeDoesNotRestore`：同一次编辑内在 `reset_scopes` 中声明 network，更新后旧 WS 参数与扩展不残留；A→B→A 不恢复。
    - `TestUpdateCredentialKeepAndClear`：`keep` 保留密文，`clear` 删除；清空后必填重新填写才能保存。
    - `TestUpdateRevisionConflict`：旧修订返回 `ErrRevisionConflict`。
    - `TestExtensionOpsAddReplaceClear`：add/replace/clear 各自落库正确，摘要不泄漏 payload。
    - `TestExtensionResetScopeCannotKeep`：被重置作用域中的扩展 `keep` 无效，必须被删除。
    - `TestImportProtectionCoversExtensions`：节点扩展密文存在时，不同签名密钥配置导入被拒绝；同密钥导入通过。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go
  cd backend && go test ./internal/node -count=1
  cd backend && go build ./...
  cd backend && go vet ./internal/node
  git diff --check
  ```
- **验收标准：**
  - 所有保存/检查路径共用同一套 reset/credential/extension 处理。
  - 敏感字段及扩展负载均不以明文写入 `nodes`。
  - `extensions_json` 中的密文被签名密钥保护；配置导入的密钥变更保护覆盖新密文位置。
  - 被重置分区不复活旧参数、旧凭据、旧扩展。
  - 旧修订冲突时不产生部分写入。

---

## 五、合同/接口速查

| 项 | 契约 |
|---|---|
| POST `/api/admin/nodes` | body 含 `current_state`、`extensions[]`；返回脱敏详情 |
| PUT `/api/admin/nodes/:id` | body 含 `base_revision`、`reset_scopes[]`、`credential_ops[]`、`extension_ops[]` |
| 409 | `{ "error": "...", "code": "revision_conflict", "current_revision": N }` |
| 读取/列表 | 返回 `edit_revision`、`state_format_version`、`current_state`、`extensions[]` 摘要 |
| 扩展摘要 | `id/scope/targets/label/configured`，不返回 `payload_encrypted` |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-02 | 初始构建方案：按 Design4 §12 落地 nodes 新列、统一保存契约、修订冲突、凭据与扩展操作。仅创建 Build 文档，未构建代码。 |
