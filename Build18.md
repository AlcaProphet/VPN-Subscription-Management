# Build18.md — FieldSchema 条件/选项扩展、当前状态投影与节点检查构建计划

> **文档定位：** 本文是 VPN 订阅管理系统第十八轮构建方案及实施记录，将 [Design4.md](Design4.md) §三～§七、§12.2～§12.4、§12.6 已确认条件表单与目标检查契约转化为逐步实现手册。本轮文档所列 Step 1～5 已完成并通过自动化验收。
> - 设计依据：[Design4.md](Design4.md)（当前最新设计，已确认作为 Build 依据）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序构建：[Build17.md](Build17.md)（保存契约与 nodes 当前状态，已完成并由本 Build 复用）
> - 用户已确认：Trojan 内层 SS 采用 `ss-opts.enabled + method + password` 目标对齐规范；VMess REALITY 首批不开放表单；客户端验证只写固定版本正反例/夹具与手工验收待办，不宣称真机连接结论。
>
> **执行原则：**
> - 每次仅执行一个 Step；不并行多步。
> - 条件与选项规则以后端 `FieldSchema` 为唯一来源，不在前端复制协议字段全集。
> - 服务端投影是权威；UI 显隐不等于输出过滤。
> - 检查接口复用实际适配器，只返回脱敏产物与诊断，不落库。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|----------|------|
| 1 | `FieldSchema` 条件/选项/目标证据扩展 | Design4 §12.2 | ✅ 验收通过 |
| 2 | 首批四协议注册表条件与推荐选项 | Design4 §四、§五、§12.4 | ✅ 验收通过 |
| 3 | 当前状态派生、投影与保存校验 | Design4 §6.1、§6.5、§12.6 | ✅ 验收通过 |
| 4 | `/api/admin/nodes/check` 节点检查接口 | Design4 §7.2、§12.3 | ✅ 验收通过 |
| 5 | 固定版本正反例、诊断样例与服务端测试 | Design4 §八、§10.3、§12.4 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/node/registry.go`、新增 `backend/internal/node/schema.go` | `When/RequiredWhen/ResetOn/OptionItems/AllowCustom/CanonicalPath/Aliases/TargetEvidence` |
| 2 | `backend/internal/node/registry.go` | 首批四协议条件矩阵、SS/VMess 算法目录、Trojan 内层 SS、XHTTP mode 修正 |
| 3 | `backend/internal/node/node.go`、新增 `backend/internal/node/project.go` | 当前状态派生、活动参数投影、保存前条件校验 |
| 4 | `backend/internal/node/check.go`（新增）、`backend/internal/assembly/node_check.go`（新增）、`backend/internal/server/node.go` | `/check`、草稿/已保存编辑分支、实际适配器、诊断响应、不落库 |
| 5 | `backend/internal/node/check_test.go`、`backend/internal/assembly/node_check_test.go`、`backend/internal/assembly/links/links_test.go`、`backend/internal/assembly/testdata/node_check/` | Mihomo 1.19.29 / CVR 2.5.2 离线正反例与诊断 |

---

## 三、构建顺序依赖图

```text
Step 1 字段元数据扩展
  → Step 2 协议矩阵与选项
    → Step 3 投影/校验
      → Step 4 检查接口
        → Step 5 证据夹具与测试
```

依赖理由：

- 检查接口需要先有可查询的条件/选项元数据。
- 投影必须依赖协议矩阵才能知道字段归属与重置范围。
- 最终用固定版本样本验证诊断输出，不先做真机实验。

---

## 四、分步构建计划

### Step 1：`FieldSchema` 条件/选项/目标证据扩展

- **目标：** 扩展现有 `FieldSchema`，使其能表达显示条件、条件必填、重置依赖、推荐选项、自定义边界、规范路径、别名与目标证据；仍保持声明式小型规则，不引入脚本求值。
- **前置条件：** Build17 已提供保存契约与服务端类型；本步只改 schema 定义与 JSON 序列化，不要求前端消费。
- **产出文件与操作：**
  - `backend/internal/node/registry.go` 或新增 `backend/internal/node/schema.go`：
    ```go
    type ConditionRule struct {
        Network  []string `json:"network,omitempty"`
        Security []string `json:"security,omitempty"`
        Plugin   []string `json:"plugin,omitempty"`
        Features []string `json:"features,omitempty"`
        Targets  []string `json:"targets,omitempty"`
    }

    type OptionItem struct {
        Value    string `json:"value"`
        Label    string `json:"label,omitempty"`
        Group    string `json:"group,omitempty"`
        Verified string `json:"verified,omitempty"` // 例：mihomo-1.19.29 / cvr-2.5.2-uri / project-unknown
    }

    type TargetEvidence struct {
        Target  string `json:"target"`
        Client  string `json:"client,omitempty"`
        Version string `json:"version,omitempty"`
        Entry   string `json:"entry,omitempty"`
        Status  string `json:"status"` // complete|equivalent|partial|unsupported|unverified
    }

    type FieldSchema struct {
        // 既有字段保持不变...
        Group          string          `json:"group,omitempty"`          // basic/auth/connection/switches/advanced
        When           *ConditionRule  `json:"when,omitempty"`
        RequiredWhen   *ConditionRule  `json:"required_when,omitempty"`
        ResetOn        []string        `json:"reset_on,omitempty"`
        OptionItems    []OptionItem    `json:"option_items,omitempty"`
        AllowCustom    bool            `json:"allow_custom,omitempty"`
        CanonicalPath  string          `json:"canonical_path,omitempty"`
        Aliases        []string        `json:"aliases,omitempty"`
        TargetEvidence []TargetEvidence `json:"target_evidence,omitempty"`
    }
    ```
  - 保留现有 `Options []string` 作为兼容投影；新实现应优先使用 `OptionItems`，`Options` 仅用于旧客户端/旧测试兼容，最终可由兼容逻辑生成。
  - 新增辅助方法：
    ```go
    func (f FieldSchema) Matches(state CurrentState, target string) bool
    func (f FieldSchema) RequiredFor(state CurrentState, target string) bool
    func (f FieldSchema) ShouldReset(scope string) bool
    ```
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go
  cd backend && go test ./internal/node -run 'TestRegistry|TestSchema' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - 旧 `ProtocolInfo` 序列化仍包含既有字段，不破坏现有前端。
  - 新字段可下发且能被 Go 测试读取。
  - 条件、重置、推荐项与目标证据没有混入运行逻辑。

---

### Step 2：首批四协议注册表条件与推荐选项

- **目标：** 为 VLESS、VMess、Trojan、SS 补齐 §12.4 普通组合矩阵对应的条件表单、推荐选项和重置归属；VMess REALITY 不开放表单，仅保留为后续候选。
- **前置条件：** Step 1 元数据结构可用。
- **产出文件与操作：** `backend/internal/node/registry.go`
  - **VLESS**
    - `network` 候选：`tcp / ws / grpc / h2 / http / xhttp`；保留自定义。
    - `security`（新增/规范化字段）候选：`none / tls / reality`；不再用 `tls bool` 单一表达。
      - `tls=true` 时 `security=tls`；`reality-opts` 存在时 `security=reality`。
    - `ws-opts`、`grpc-opts`、`h2-opts`、`http-opts`、`xhttp-opts` 设置 `when.network` 对应值。
    - TLS 字段：`servername`、`alpn`、`client-fingerprint`、`skip-cert-verify` 设置 `when.security=tls|reality`（Reality 中部分字段按矩阵允许）。
    - REALITY 字段：`reality-opts` 仅在 `when.security=reality`；`flow` 仅在适用组合展示。
    - `xhttp-opts.mode` 候选改为 `auto / stream-one / stream-up / packet-up`，不包含 `none`；未选择时不写入默认 `none`。
    - `encryption` 保持高级区可查看，标注 target 证据：generic 当前固定输出 `none`，其它值按 `partial/unsupported` 诊断。
  - **VMess**
    - `cipher` 候选：`auto / aes-128-gcm / chacha20-poly1305 / none / zero`；`allow_custom=true`；标注 CVR URI 对 `chacha20-poly1305`、`zero` 的改写风险。
    - `network` 候选：`tcp / ws / grpc / h2 / http`（不含 xhttp 首批）。
    - `tls/security` 与 VLESS 分离，不把 REALITY 加入候选；VMess REALITY 仅保留 `target_evidence=unverified` 的后续说明，不出现在 `option_items` 或 `when` 中。
    - `alterId` 标注旧版/兼容；默认语义与显式 `0` 分开。
  - **Trojan**
    - `network` 候选：`tcp / ws / grpc`；`h2/http/xhttp` 允许自定义，但 `target_evidence` 标记为`unverified/partial`，不得列为普通组合。
    - TLS 字段：`password`、`sni`、`alpn`、`skip-cert-verify`、`client-fingerprint`。
    - `ss-opts` 改为目标对齐结构：
      ```json
      {
        "type": "object",
        "object_kind": "fields",
        "properties": [
          { "name": "enabled", "type": "bool", "default": false, "label": "启用内层 SS" },
          { "name": "method", "type": "text", "label": "内层加密方式", "option_items": [
              { "value": "aes-128-gcm", "label": "AES-128-GCM", "group": "common", "verified": "mihomo-1.19.29" },
              { "value": "aes-256-gcm", "label": "AES-256-GCM", "group": "common", "verified": "mihomo-1.19.29" },
              { "value": "chacha20-ietf-poly1305", "label": "ChaCha20-Poly1305", "group": "common", "verified": "mihomo-1.19.29" }
          ]},
          { "name": "password", "type": "password", "label": "内层密码" }
        ]
      }
      ```
    - `ss-opts` 的 `SensitiveFields` 改为 `ss-opts.password`；保留旧 `cipher` 仅作为兼容输入别名（`aliases: ["cipher"]`），输出以 `method` 为规范。
  - **SS**
    - `cipher` 候选：`aes-128-gcm / aes-256-gcm / chacha20-ietf-poly1305` 入常用组；旧版/其他算法入“旧版兼容/自定义”组并允许手填；`SS 2022` 只标后续/待验证，不列入普通完成项。
    - `plugin` 候选：`obfs / v2ray-plugin / shadow-tls / restls`；每个插件有自己的 `plugin-opts` 子字段与 `when.plugin` 条件。
    - `plugin-opts` 子字段按插件映射：`obfs` 显示 `mode/host`；`v2ray-plugin` 显示 `mode/host/tls/path/headers/version`；`shadow-tls`/`restls` 显示各自字段。
    - `udp-over-tcp`、`smux` 等作为功能开关/高级区，`reset_on: ["feature.udp-over-tcp"]`、`reset_on: ["feature.smux"]`。
  - 对所有首批字段补充 `canonical_path`：如 `ws-opts.path`、`grpc-opts.grpc-service-name`、`reality-opts.public-key`、`plugin-opts.mode`。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/registry.go
  cd backend && go test ./internal/node -run 'TestRegistry|TestSchema|TestFieldCondition' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - `GetProtocol("vless/vmess/trojan/ss")` 能返回条件、推荐项、重置依赖。
  - SS/VMess 算法清单独立；XHTTP mode 不含 `none` 默认；Trojan 内层 SS 使用 `enabled/method/password`。
  - VMess REALITY 没有首批表单入口。
  - 旧 `Options` 仍可通过兼容函数生成，不破坏 Build15/已有前端。

---

### Step 3：当前状态派生、投影与保存校验

- **目标：** 服务端能从不完整/旧参数中派生最小当前状态，按当前状态投影活动参数，并执行保存与检查共用的条件校验。
- **前置条件：** Step 2 注册表条件可用。
- **产出文件与操作：**
  - 新增 `backend/internal/node/project.go`：
    - `DeriveCurrentState(proto Protocol, params map[string]any) CurrentState`
      - `network`：读 `params["network"]`，无则 `"tcp"`（或协议无传输时为空）。
      - `security`：`reality-opts` 存在且非空 → `"reality"`；否则 `tls=true` → `"tls"`；否则 `"none"`（Trojan 按 TLS 常开表达）。
      - `plugin`：读 `params["plugin"]`，空值 `nil`。
      - `features`：读已知功能字段（`smux.enabled`、`udp-over-tcp` 等），只放当前启用项。
    - `ProjectActive(proto Protocol, state CurrentState, params map[string]any) map[string]any`
      - 遍历 `proto.FormSchema`，仅保留 `When.Matches(state,target)` 为 true 的字段及非空有效值；
      - 对象字段递归保留子字段，不投射被切离分支的参数；
      - 不删除 `extensions`（由独立扩展流程处理）。
    - `ValidateCurrentState(proto Protocol, state CurrentState, params map[string]any) error`
      - 按 `required_when` 检查当前目标下条件必填；
      - 对明确非法组合（如 XHTTP `none`、SS `auto`、Trojan h2 普通组合）返回字段路径错误。
  - `backend/internal/node/node.go`：
    - `CreateManual`/`UpdateManual` 保存前调用 `ValidateCurrentState`；若 `CurrentState` 为空则 `DeriveCurrentState` 后回写请求 DTO。
    - `ProjectActive` 用于检查/输出；保存仍通过同一归一化/校验流程写入 `protocol_json` 与当前状态，不另建分支恢复副本。
  - 输出层（`render_clash.go`/`links.go`）在 Build20 接入投影；本步先在 node 包提供函数与测试。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go
  cd backend && go test ./internal/node -run 'TestProject|TestDerive|TestValidateCurrent' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - WS 切到 gRPC 后，`ProjectActive` 不残留 `ws-opts`。
  - REALITY/TLS 状态能区分，不依赖 `reality-opts` 存在性猜测。
  - 条件必填与普通必填分离，错误路径可定位到字段。
  - 保存与检查共用同一套派生/校验函数。

---

### Step 4：`/api/admin/nodes/check` 节点检查接口

- **目标：** 为新建草稿与已保存编辑草稿提供脱敏目标检查；服务端复用实际适配器，不落库，返回产物片段与诊断。
- **前置条件：** Step 3 投影/校验可用；Build17 的保存流程提供 reset/credential/extension 共用处理。
- **产出文件与操作：**
  - 新增 `backend/internal/node/check.go`：
    ```go
    type CheckRequest struct {
        NodeID       int64                `json:"node_id,omitempty"`
        BaseRevision int64                `json:"base_revision,omitempty"`
        Protocol     string               `json:"protocol"`
        Host         string               `json:"host"`
        Port         int                  `json:"port"`
        ProtocolJSON map[string]any       `json:"protocol_json"`
        CurrentState *CurrentState        `json:"current_state,omitempty"`
        ResetScopes  []string             `json:"reset_scopes,omitempty"`
        CredentialOps []CredentialOp      `json:"credential_ops,omitempty"`
        Targets      []string             `json:"targets,omitempty"` // 缺省为全部首批目标
    }

    type TargetDiagnostic struct {
        Severity string `json:"severity"` // info|warn|error
        Code     string `json:"code"`
        Target   string `json:"target,omitempty"`
        FieldPath string `json:"field_path,omitempty"`
        Message  string `json:"message"`
        Evidence string `json:"evidence,omitempty"`
    }

    type TargetCheckResult struct {
        Status      string             `json:"status"` // ok|warn|skip|error
        Preview     *string            `json:"preview,omitempty"`
        Diagnostics []TargetDiagnostic `json:"diagnostics"`
    }

    type CheckResponse struct {
        CheckID      string                      `json:"check_id"`
        CheckVersion int                         `json:"check_version"`
        Targets      map[string]TargetCheckResult `json:"targets"`
    }
    ```
  - 服务方法 `Check(ctx, req) (*CheckResponse, error)`：
    1. 若 `NodeID > 0`，读取现有节点并检查 `BaseRevision`；不一致时按 409 处理（或检查响应中返回冲突，建议与保存一致返回 409）。
    2. 不使用现有节点中已被重置分区作为兜底；先按 Build17 的 reset/credential/extension 处理生成“检查草稿”，但只存在内存。
    3. 调用 `ProjectActive` 得到活动参数。
    4. 对每个目标：
       - `clash-yaml`：构造最小 Clash proxy 对象（可在 node 包内调用 assembly 现有 `clashProxy` 或复制只读投影），输出脱敏 YAML 片段与 `CheckClashContent` 诊断。
       - `sr-subs` / `generic-subs`：调用 `assembly.RenderLink(protocol, renderName, host, port, activeParams, generic)`；链接中敏感字段保持脱敏占位/或使用占位值，返回片段。
       - `sr-conf` 不参与节点检查。
    5. 状态判定：
       - 无诊断 → `ok`；
       - 仅 warn/info → `warn`；
       - 错误但可跳过/剔除 → `skip`；
       - 核心语义不可表达 → `error`。
    6. 生成 `check_id`（如 `chk-<timestamp>-<hash>`）与 `check_version=1`；不写库。
  - `backend/internal/server/node.go`：
    - 路由注册：
      ```go
      admin.POST("/check", h.check)
      ```
    - handler 绑定 `node.CheckRequest`，调用 `h.nodeSvc.Check`，返回 `OK(c, resp)` 或 409/400。
  - 注意：
    - 检查不得读取被清空分区的旧凭据。
    - 迟到响应防覆盖由前端通过 `check_id`/请求版本控制（Build19），后端不保存状态。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go internal/server/*.go
  cd backend && go test ./internal/node ./internal/server -run 'TestCheck|TestNodeCheck' -count=1
  cd backend && go build ./...
  cd backend && go vet ./internal/node ./internal/server
  git diff --check
  ```
- **验收标准：**
  - 新建无 `node_id` 可检查；编辑带 `node_id/base_revision` 可检查。
  - 检查不写 `nodes`/扩展/任何表；数据库无变化。
  - 脱敏产物不包含明文凭据或扩展 payload。
  - SR/generic 对无法承载的字段返回 `skip/error`，不生成“成功且静默遗漏”。

---

### Step 5：固定版本正反例、诊断样例与服务端测试

- **目标：** 将 Design4 第八章的 Mihomo 1.19.29 与 CVR 2.5.2 离线结论转成仓库内可复现夹具/测试；不把配置解析成功等同于连接成功，不添加真机结论。
- **前置条件：** Step 4 检查接口可用。
- **产出文件与操作：**
  - 新增节点检查 fixtures 目录 `backend/internal/assembly/testdata/node_check/`：
    - `vless-tcp-tls.json`、`vless-ws-tls.json`、`vless-reality.json`、`vless-xhttp-risk.json`
    - `vmess-tcp.json`、`vmess-ws-tls.json`、`vmess-cipher-risk.json`
    - `trojan-tcp-tls.json`、`trojan-ws-tls.json`、`trojan-grpc-tls.json`、`trojan-inner-ss.json`
    - `ss-aes-gcm.json`、`ss-obfs.json`、`ss-v2ray-plugin.json`、`ss-2022-pending.json`
  - 测试断言方向：
    - 普通组合在相应目标 `status` 为 `ok` 或明确 `warn`（不因缺少真机证据标 `complete`）。
    - 风险组合必须产生 `error/warn` 且带 `field_path`、`evidence`：
      - VMess `chacha20-poly1305`/`zero` 在 SR/generic URI 的改写风险；
      - Trojan WS/gRPC 在 SR/generic 当前无法承载 → skip/error；
      - SS obfs 内部名与 CVR 偏好 `obfs-local` 的差异 → 诊断；
      - XHTTP `none` 不作为未指定；
      - SS 2022 标记后续/待验证。
    - `links_test.go` 增加固定 URI 样例，验证生成链接与已知解析器观察一致（不运行外部客户端）。
  - 手工验收待办清单（写进文档/测试注释）：
    - 在指定版本 Clash Verge Rev / Mihomo 上做 YAML 导入；
    - 对代表组合执行真实连接；
    - 未执行前所有 `unverified` 状态保持。
- **测试与验收命令：**
  ```bash
  cd backend && gofmt -w internal/node/*.go internal/assembly/links/*.go
  cd backend && go test ./internal/node ./internal/assembly/links -count=1
  cd backend && go test ./internal/node -run 'TestCheck' -count=1
  cd backend && go build ./...
  git diff --check
  ```
- **验收标准：**
  - 固定版本正反例可作为后续 Build20 输出门槛的输入。
  - 不在无真机证据时把任意组合标为完整支持。
  - 测试失败信息包含目标、字段路径与证据标识。

---

## 五、合同/接口速查

| 项 | 契约 |
|---|---|
| `POST /api/admin/nodes/check` | 接受新建草稿与编辑草稿；不落库 |
| 编辑草稿必带 | `node_id`、`base_revision`、`reset_scopes`、`credential_ops`、`current_state` |
| 响应 | `check_id`、`check_version`、`targets{target:{status,preview,diagnostics[]}}` |
| 诊断严重级 | `info / warn / error` |
| 结果状态 | `ok / warn / skip / error` |
| 首批目标 | `clash-yaml`、`sr-subs`、`generic-subs` |
| 客户端证据状态 | `complete/equivalent/partial/unsupported/unverified`；不用“解析成功”表示“连接成功” |

---

## 六、实施与验收记录

### 6.1 已完成实现

- `FieldSchema` 新增 `group`、`when`、`required_when`、`reset_on`、`option_items`、`allow_custom`、`canonical_path`、`aliases`、`target_evidence`；保留旧 `options` 兼容投影，并提供条件匹配、条件必填和重置判断方法。
- 首批 VLESS、VMess、Trojan、SS 注册表已补齐传输/安全/插件条件、推荐选项和规范路径。VLESS/VMess 使用表单层 `security` 并在保存时兼容回既有 `tls`；VMess REALITY 只保留未验证证据，不进入首批选项；Trojan 内层 SS 使用 `enabled/method/password`，旧 `cipher` 仅作输入别名；XHTTP mode 不含 `none`。
- 节点服务新增活动状态派生、`ProjectActive` 投影及保存/目标共用校验；切换分支后不投影非活动字段，对象中的未知键仍保留，创建和更新保存前执行明确非法组合校验。
- 新增 `POST /api/admin/nodes/check`。新建草稿和带 `node_id/base_revision` 的编辑草稿均在内存中应用 reset、credential、extension 操作；节点服务通过注入调用实际装配器，返回 Clash 最小 YAML 片段或 SR/generic URI、脱敏凭据、`check_id/check_version` 和字段级诊断，不写入 `nodes`、扩展或其他业务表。
- 检查目标覆盖 `clash-yaml`、`sr-subs`、`generic-subs`。Clash 目标复用 `clashProxy` 与 `CheckClashContent`；URI 目标复用 `RenderLink`，对 VLESS encryption、VMess 算法改写、Trojan WS/gRPC/内层 SS、SS obfs/SS 2022 等已知损失或待验证项显式返回 `warn/skip` 诊断；目标限定的 REALITY 条件必填按目标分别校验。
- 新增 15 个节点检查 JSON 夹具、检查响应/不落库测试及固定 URI 观察测试，覆盖四协议普通组合与风险组合。夹具是 Mihomo 1.19.29 / CVR 2.5.2 的离线输入，不代表真实导入或连接成功。

### 6.2 自动化验收结果

以下命令均在本轮实现后执行并通过：

```text
cd backend && go test ./...       PASS
cd backend && go build ./...      PASS
cd backend && go vet ./...        PASS
cd frontend && npm run build      PASS（Vite 提示存在大于 600 kB 的 chunk）
git diff --check                  PASS
```

自动化测试已覆盖：FieldSchema/四协议矩阵、当前状态与活动投影、条件必填、草稿和编辑检查、修订冲突、数据库不变、凭据/扩展脱敏、目标诊断、固定 URI 形态。尚未执行指定版本 Clash Verge Rev / Mihomo 的实际导入和真实连接实验；相关 `unverified` 证据状态保持不变，后续由 Build20/人工验收继续。

## 七、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-02 | 初始构建方案：FieldSchema 条件/选项扩展、首批四协议矩阵、当前状态投影、节点检查接口与固定版本正反例。仅创建 Build 文档，未构建代码。 |
| v1.1 | 2026-09-02 | 完成 Step 1～5：实现条件/选项元数据、四协议注册表矩阵、活动投影与保存校验、`/api/admin/nodes/check` 实际适配器接入、脱敏/不落库验证、15 个离线夹具和固定 URI 测试；后端全量测试/构建/静态检查、前端生产构建与差异检查均通过。客户端实际导入/连接仍待人工验收。 |
