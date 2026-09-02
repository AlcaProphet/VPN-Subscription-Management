# Build20.md — 全量手动协议过渡、URI/来源适配、输出门槛与回归收口构建计划

> **文档定位：** 本文是 VPN 订阅管理系统第二十轮当前构建方案及实施记录，将 [Design4.md](Design4.md) §6.5、§7.4、§9、§10.3、§12.5 中的过渡期、来源适配、目标生成门槛和全量验收转化为实现手册。本 Build 已按 Step 顺序开始实施并同步记录验收结果。
> - 设计依据：[Design4.md](Design4.md)（当前最新设计，已确认作为 Build 依据）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序构建：[Build17.md](Build17.md)（保存契约）、[Build18.md](Build18.md)（FieldSchema/检查）、[Build19.md](Build19.md)（前端动态表单）
> - 用户已确认：4 份 Build 文档存入根目录；Trojan 内层 SS 目标对齐；VMess REALITY 首批不开放；客户端验证只含固定版本正反例/夹具与手工验收待办。
>
> **执行原则：**
> - 不把首批四协议之外的 15 个协议做成完整条件表单；保留原表单能力，但统一接入最小当前状态与保存契约。
> - Xray 来源节点不开放 manual 编辑，只在进入共享输出层前做来源适配。
> - 目标生成门槛按 Design4 §7.4 执行：核心语义不可表达时 SR/generic 跳过、Clash 阻止；可选损失与未验证分别诊断。
> - 历史 manual 快照不被静默重写；重新装配才形成新产物。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|----------|------|
| 1 | 手动表单与 URI 导入统一归一化/当前状态初始化 | Design4 §6.5、§12.5 | ✅ 验收通过 |
| 2 | 其余 15 个 manual 协议接入统一保存契约 | Design4 §D21、§9.1 | ✅ 验收通过 |
| 3 | Xray 来源适配与 SS/插件 URI 映射收敛 | Design4 §12.5、§8.4 | ✅ 验收通过 |
| 4 | 输出生成门槛与装配诊断接入 | Design4 §7.4、§12.6 | ✅ 验收通过 |
| 5 | 全量回归、历史版本边界与文档收口 | Design4 §10.3、AGENTS §3.4～§3.6 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/node/uri_import.go`、新增 `backend/internal/node/normalize.go`、`backend/internal/node/node.go` | 规范路径归一化、当前状态初始化、导入逐行回执 |
| 2 | `backend/internal/node/registry.go`、`backend/internal/node/node.go` | 全部 19 个协议保存/读取带 current_state/edit_revision；非首批保持原 schema |
| 3 | `backend/internal/xray/detect.go`、`backend/internal/assembly/links/links.go` | `ws-path/ws-host` 来源适配、SS `obfs`/插件目标映射 |
| 4 | `backend/internal/assembly/render_sr.go`、`render_clash.go`、`validate.go`、新增 `backend/internal/assembly/diagnose.go` | 节点级目标检查接入生成、跳过/阻止、可选/未验证诊断 |
| 5 | `backend/internal/node/*_test.go`、`backend/internal/assembly/*_test.go`、`frontend/tests/*` | 全量回归、历史边界、文档同步 |

---

## 三、构建顺序依赖图

```text
Step 1 统一入口/初始化
  → Step 2 全协议过渡
    → Step 3 来源适配/URI 映射
      → Step 4 输出门槛
        → Step 5 回归与收口
```

依赖理由：

- 先统一保存入口，后续所有 manual 协议才有共同当前状态。
- 全协议过渡是输出门槛的数据基础。
- 来源适配必须在校验/输出前完成，避免旧别名穿透。
- 输出门槛在保存/检查之后接入实际装配，避免半套诊断。
- 最后做全量验证与文档同步。

---

## 四、分步构建计划

### Step 1：手动表单与 URI 导入统一归一化/当前状态初始化

- **目标：** manual 表单创建与 URI 导入共用同一归一化与当前状态初始化入口；导入保留逐行回执，不静默改写。
- **前置条件：** Build17 的保存契约、Build18 的 FieldSchema/派生函数可用。
- **产出文件与操作：**
  - 新增 `backend/internal/node/normalize.go`：
    - `NormalizeProtocolJSON(proto Protocol, params map[string]any) (map[string]any, error)`
      - 将兼容输入别名规约到规范路径：
        - `ws-path` → `ws-opts.path`
        - `ws-host`/`ws-headers` → `ws-opts.headers`
        - `grpc-service-name` → `grpc-opts.grpc-service-name`
        - `public-key`/`short-id` 顶层旧值 → `reality-opts.public-key`/`reality-opts.short-id`
        - `cipher`（Trojan 内层旧表单）→ `ss-opts.method`（仅在 Trojan 内层 SS 场景，且由来源/协议明确判断）
      - 不在 URI 导入中自动删除无法归属的字段；无法归属的顶层未知值进入高级区/扩展块提示。
    - `InitCurrentState(proto Protocol, params map[string]any) CurrentState`
      - 复用 Build18 `DeriveCurrentState`；对非首批协议仅提取稳定标识（network/security/plugin/features 中当前存在的项），不生成不存在于该协议的字段。
  - `backend/internal/node/uri_import.go`：
    - 每行解析后调用 `NormalizeProtocolJSON` + `InitCurrentState` + `encryptProtocolJSON`。
    - 创建写入包含：
      ```sql
      INSERT INTO nodes (... protocol_json, current_state_json, extensions_json, edit_revision, state_format_version ...)
      ```
    - 每行回执继续保留 `line/raw/ok/name/reason`；失败仅跳过该行，不中断整批。
  - `backend/internal/node/node.go` `CreateManual` 调用 `NormalizeProtocolJSON` 前置处理。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go
  cd backend && go test ./internal/node -run 'TestNormalize|TestImport|TestInitState' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - URI 导入的 `ws-path` 等旧别名落入规范 `ws-opts`，不再生成冲突顶层键。
  - 导入成功节点带有 `current_state` 与 `edit_revision=1`。
  - 每行失败原因明确；不会因单行失败整批回滚。
  - Xray 检测路径暂不调用此函数（由 Step 3 单独适配）。

---

### Step 2：其余 15 个 manual 协议接入统一保存契约

- **目标：** 全部 19 个 manual 协议统一使用 `current_state_json`、`edit_revision`、`state_format_version` 和扩展存储；非首批 15 个保留原 schema 与编辑能力，不建立分支恢复数据，不隐藏入口。
- **前置条件：** Step 1 统一入口可用。
- **产出文件与操作：**
  - `backend/internal/node/registry.go`：
    - 保持 19 个协议注册表数量与入口不变。
    - 对非首批协议，`FieldSchema` 可继续使用当前静态 schema；只需在 `current_state` 中保存可解析的当前选择（如 `network`、`tls`、`plugin`、`udp` 等中真实存在的稳定项），不要求完整 `when` 矩阵。
    - 不把 VMess REALITY、SS 2022 等未完成组合移到“已完成”。
  - `backend/internal/node/node.go`：
    - 创建/更新对所有 manual 协议走统一 DTO。
    - 非首批协议的 `current_state` 可为最小/空；服务端仍写入 `{}` 或仅含可识别字段。
    - 更新时 `base_revision` 校验统一适用；非首批不实现复杂 reset 分支，但至少支持协议切换整体清空 + 同协议普通编辑保留凭据。
  - 前端（Build19 已完成四个协议动态表单）：
    - 其他 15 个协议继续使用原有 `ProtocolFieldEditor` 能力；
    - 新建/编辑仍可保存，列表能显示 `edit_revision`，保存能携带 `base_revision`。
  - 测试：
    - 枚举 19 个协议，逐一创建最小合法节点、更新、读取。
    - 非首批协议保存后 `edit_revision` 递增，current_state 可序列化。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go
  cd backend && go test ./internal/node -run 'TestAllManual|TestTransition' -count=1
  cd backend && go build ./...
  cd backend && go vet ./internal/node
  git diff --check
  ```
- **验收标准：**
  - 19 个协议均可新建/编辑/保存/读取。
  - 非首批协议的原表单与输出入口不丢失。
  - 保存契约不会因首批/非首批产生两套不兼容 API。
  - 不创建分支恢复数据，不隐藏/删除非首批入口。

---

### Step 3：Xray 来源适配与 SS/插件 URI 映射收敛

- **目标：** Xray 检测字段在进入共享输出层前完成来源归一化；SS 插件 URI 输出按目标客户端映射，避免把内部名直接当作客户端名。
- **前置条件：** Step 2 全协议过渡可用。
- **产出文件与操作：**
  - `backend/internal/xray/detect.go`（或新增 `backend/internal/xray/normalize.go`）：
    - 检测结果中的 `ws-path` → `ws-opts.path`、`ws-host` → `ws-opts.headers.Host`、`grpc-service-name` → `grpc-opts.grpc-service-name`。
    - Xray 来源不建立人工可编辑分支状态；适配只发生在输出层/共享读取层，不开放 manual 清空逻辑。
    - 保持 Xray 来源所有权和现有检测/同步路径不变。
  - `backend/internal/assembly/links/links.go`：
    - 新增插件目标映射函数，例如：
      - 内部 `obfs` → CVR/SIP003 偏好 `obfs-local;obfs=tls|http;obfs-host=...`；
      - `v2ray-plugin` → 按目标参数名输出；
      - `shadow-tls` / `restls` → 保留当前格式并标记映射状态。
    - `pluginString` 不再直接用存储插件名串联客户端参数；改为 `renderPluginForTarget(plugin, opts, target)`。
    - 对 SS 2022 保持“后续/待验证”，不交付完整 URI 编码修复；如用户输入 2022 算法，输出不得宣称完整支持。
  - 测试：
    - Xray 规范化前后字段断言；
    - SS `obfs` 在 SR/generic/Clash 的目标映射正反例；
    - 未知插件不做静默透传，至少产生诊断。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/xray/*.go internal/assembly/links/*.go
  cd backend && go test ./internal/xray ./internal/assembly/links -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - Xray 检测产生的旧顶层字段不进入共享输出。
  - SS 插件输出与 CVR 2.5.2 观察到的可解析形态一致（或明确诊断差异）。
  - 未知/未验证插件不会伪装成完整支持。

---

### Step 4：输出生成门槛与装配诊断接入

- **目标：** 把 Design4 §7.4 的目标生成门槛接入实际装配生成：SR/generic 跳过核心不可表达节点、零有效链接拒绝；Clash 阻止核心错误且不自动改组成员；可选损失/未验证分别诊断。
- **前置条件：** Step 3 来源适配与 URI 映射完成；Build18 的检查/诊断函数可用。
- **产出文件与操作：**
  - 新增 `backend/internal/assembly/diagnose.go`：
    ```go
    type NodeDiagnostic struct {
        Severity string `json:"severity"`
        Code     string `json:"code"`
        Target   string `json:"target"`
        FieldPath string `json:"field_path,omitempty"`
        Message  string `json:"message"`
    }
    ```
  - `backend/internal/assembly/render_sr.go`：
    - 在生成每个节点链接前，调用 `node.CheckNodeForTarget` 或等价诊断：
      - `error`/`skip` 核心不可表达 → 追加 `SkipItem`，不输出该链接；
      - `warn` 可选损失/算法改写/插件丢失 → 仍输出，但在回执/诊断中列出；
      - `unverified` → 输出但标注未验证。
    - 最终 `len(skipped core) + len(output)==0` 时，返回“节点订阅至少需要 1 个可转换链接的节点”已有错误；保留 Design2 §4.5 语义。
  - `backend/internal/assembly/render_clash.go`：
    - 在生成 Clash 前对已选 manual 节点执行诊断：
      - 若目标为 `clash-yaml` 且存在 `error` 级核心语义不可表达 → 返回 `ErrBadRequest` 并包含节点/字段，阻止生成；
      - 不允许自动删除节点或改写代理组成员；
      - `warn`/`unverified` 可在回执/提示中展示，不影响生成。
  - `backend/internal/assembly/validate.go`：
    - 保持现有空产物、组引用、节点可用性校验；新增从 `SkipItem`/诊断聚合到 `ConversionReceipt` 或 `PreviewResult.Warnings`。
  - 注意：
    - 覆盖层应用后的最终检查仍需执行，不能因局部检查通过而跳过。
    - 历史 manual 快照不重写；只有重新装配才使用新的投影/检查。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/assembly/*.go
  cd backend && go test ./internal/assembly -run 'Test.*Diagnose|Test.*Skip|Test.*Clash.*Block|Test.*Zero' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - SR/generic 中 Trojan WS/gRPC、VMess 风险算法、SS obfs 差异产生明确跳过或警告。
  - Clash 对核心不可表达节点阻止生成且不自动改组成员。
  - 零有效链接时拒绝生成。
  - 可选调优损失与未验证不是相同状态，不能冒充完整支持。
  - 已生成快照没有被编辑/能力表更新静默重写。

---

### Step 5：全量回归、历史版本边界与文档收口

- **目标：** 完成前后端全量验证、历史版本边界检查，并按文档体系登记 Build17～Build20 与 Design4 的实现状态。
- **前置条件：** Step 1～4 验收通过。
- **产出文件与操作：**
  - 后端测试：
    - 全量 `go test ./...`；
    - 重点验证：
      - 19 协议 create/update/read；
      - revision 冲突；
      - reset/credential/extensions；
      - 项目/URI 导入；
      - Xray 来源适配；
      - Clash/SR/generic 输出门槛；
      - 历史快照边界；
      - 备份恢复：确认 `nodes` 新列（`current_state_json`、`extensions_json`、`edit_revision`、`state_format_version`）随备份完整恢复；
      - 配置导入保护：不同签名密钥导入在存在扩展密文时被拒绝，同密钥往返通过。
  - 前端测试：
    - `npm test -- --run`；
    - 重点验证动态表单、清空、检查、409、移动端。
  - 人工验收待办清单（写入文档或 `ProdTestList.md`，不作为自动门禁）：
    - 在 Mihomo 1.19.29 / Clash Verge Rev 2.5.2 上导入代表组合；
    - 真实连接后另行记录连接级结论；
    - Shadowrocket 保持“待真机验证”。
  - 文档同步：
    - 更新 `Design4.md` 状态：从“待构建”改为“已构建/验收中”仅在实际完成构建并验收后；
    - 更新 `AGENTS.md` 文档清单：登记 Build17～Build20；
    - 将本次构建计划按归档规则移入 `docs/reports/Build/`（用户已指示本次文档暂存根目录，实际构建完成后按项目惯例归档）或经用户确认后处理；
    - Build17～Build20 若后续被实施完成，补充实际结果与验收记录。
- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test -timeout 180s ./...
  cd frontend && npm test -- --run
  cd frontend && npm run build
  git diff --check
  ```
- **验收标准：**
  - 后端构建/vet/全量测试通过。
  - 前端测试与生产构建通过。
  - 无真机连接证据时，相关状态保持 `unverified`。
  - 文档登记与 Design4 状态一致，不提前标记客户端连接验收完成。

---

## 五、边界与禁止事项

- 不实现 SS 2022 完整密钥/URI/客户端兼容；只保留后续/待验证。
- 不把 VMess REALITY 做成首批可编辑表单。
- 不把 TUIC 认证互斥、WireGuard 多 Peer 扩展模型当作首批完成的 UI；仅作为后续扩展验例保留。
- 不创建 `node_edit_states` 表，不保存非激活分支/恢复副本。
- 不开放 Xray 来源节点的人工分支编辑。
- 不在读取时旧值静默迁移/改写；只做明确归一化并保留诊断。
- 不把配置解析成功或单元测试通过当作客户端连接验收完成。
- 不自动删除 Clash 已选节点或改写代理组成员来规避诊断。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-02 | 初始构建方案：全 19 协议过渡、URI/Xray 来源适配、输出生成门槛、全量回归与文档收口。仅创建 Build 文档，未构建代码。 |
| v1.1 | 2026-09-02 | 完成 Step 1：新增 `normalize.go` 的 `NormalizeProtocolJSON`/`InitCurrentState`，在手动创建与 URI 导入接入统一归一化；补充 `ws-path/ws-host/ws-headers/grpc-service-name/reality 旧别名/Trojan 内层 SS cipher` 映射；非首批当前状态校验允许省略不存在维度；新增归一化测试，`go test ./internal/node` 与 `go build ./...` 通过。 |
| v1.2 | 2026-09-02 | 完成 Step 2：所有 manual 协议创建/更新/读取统一走 `current_state_json`/`edit_revision`/`state_format_version`；非首批无 nil 请求时使用最小当前状态初始化；新增 19 协议枚举 create/update/read 测试；修复 ws 归一化不应凭空生成 `ws-opts` 的问题；`go test ./internal/node`、`go build`、`go vet ./internal/node` 通过。 |
| v1.3 | 2026-09-02 | 完成 Step 3：Xray 检测字段在输出前归一化 `ws-path/ws-host/serviceName` 到规范对象；SS obfs 输出映射为 `obfs-local;obfs=...;obfs-host=...`；未知插件诊断、SS obfs 诊断代码更新；新增 Xray/链接/未知插件测试，`go test ./internal/xray ./internal/assembly/links ./internal/assembly` 与 `go build` 通过。 |
| v1.4 | 2026-09-02 | 完成 Step 4：新增 `diagnose.go`，SR/generic 在生成前按目标诊断跳过核心不可表达节点且全跳过时拒绝；Clash 对节点级 error 诊断阻止生成且不自动改组成员；warn/info 诊断进入回执警告；新增跳过/全部跳过/Clash 阻止测试，`go test ./internal/assembly`、`go build`、`go vet` 通过。 |
| v1.5 | 2026-09-02 | 完成 Step 5：全量回归通过（后端 `go test -timeout 180s ./...`、`go vet ./...`；前端 39 文件 / 158 用例、`npm run build`）；同步更新 AGENTS/Design4/Build19/Build20 文档状态；客户端真机连接仍保留为人工验收待办，不视为自动验收。 |
