# VPN 订阅管理系统 功能构建计划（Build23：BuildReport4 未闭环项 3）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前构建方案**，承接 [Build22.md](Build22.md) 的“先研究、再构建”方法，针对 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 未闭环项 3 形成根因研究与可执行构建计划。
> - 设计记录：[Design4.md](Design4.md)（当前设计记录；与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - 问题来源：[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md)（全量核验报告，未闭环项 3）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 问题追踪：[Issue13.md](Issue13.md)（当前问题记录，含 R27-09）
> - 历史构建与问题记录：[Build21.md](Build21.md)（R27-09 Step 7～10 已完成）、[Build17.md](Build17.md)～[Build22.md](Build22.md)、[docs/reports/Build/](docs/reports/Build)（均已存档，仅核查）
>
> **用户已确认的决策：**
> 1. Build23.md 放仓库根目录，作为当前构建方案。
> 2. 研究/构建范围覆盖 BuildReport4 未闭环项 3，并纳入 Build21 曾排除的 N-node-3/N-node-4。
> 3. `target_evidence` 仅按 SS 插件合同派生诊断，不全局启用所有字段级证据。
> 4. 未知 SS 插件在 Clash 输出中保留结构化 `plugin` + `plugin-opts`，并给出未验证 warning，不阻断。
> 5. SR VMess 输出包含 `alpn`/`fp`，但这些字段没有固定版本解析器证据，必须标注 Shadowrocket 真机待验证。
> 6. generic VMess 本次不补充 `skip-cert-verify`，保持现状并在文档/测试中记录该边界。
>
> **执行原则（与 Build22 一致）：**
> - 每一步完成后均可编译、可测试。不跳步、不并行多步。
> - AI 执行指令：每次仅执行一个 Step，完成后运行验收命令，确认通过后再进入下一步。
> - **排序原则：先修复后构建、先安全后优化、先依赖后独立**。
> - 每步的新增逻辑必须配套单元测试（用户决策）。
> - 所有输出投影必须克隆数据，不得改动 `protocol_json`、历史版本快照或调用方传入 map。
> - 本文档只描述根因、修复方向、可选方案与后续可执行 Step；实施前仍应逐 Step 执行、验收，未完成前不得把 R27-09 / N-node 项标记为“已全部闭环”。
>
> **研究结论摘要：**
> - N-node-1：未知插件存储/URI 导入已由 Build21 修复，但 Clash 输出仍拍平并删除结构化 `plugin-opts`，自检也不识别旧 URI 字符串格式。
> - N-node-2/N-node-5：`target_evidence` 仅是元数据且未被检查链路消费；SS 诊断硬编码，导致 v2ray-plugin/shadow-tls/restls 的 URI 检查可能误报 `ok`。
> - N-node-3/N-node-4：SR VMess/VLESS 缺少 TLS/ALPN/指纹/Flow/Skip 参数，`uriparse` 的 SR VMess 回读也不完整。
> - 修复方向：Clash 结构化投影 + 产物自检、SS 插件统一目标诊断、未知插件前端编辑、SR URI TLS 参数补全、全链路回归与文档收口。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| — | 当前无进行中的构建 Step（Build21 Step 7～10 已验收，Step 11～14 未实施） | — | — |
| 1 | Clash/Mihomo 结构化 SS 插件投影与产物自检 | Build21 §7.8；Design4 §7.4 | ☐ 未开始 |
| 2 | SS 插件统一目标诊断与正式装配门槛（含 N-node-2/5） | Build21 §7.9；BuildReport4 §5.3 | ☐ 未开始 |
| 3 | 未知插件参数前端编辑、校验与分支清空 | Build21 §7.10 | ☐ 未开始 |
| 4 | VMess/VLESS SR URI TLS/ALPN/指纹/Flow/Skip 输出补全与解析同步（N-node-3/4） | BuildReport4 §5.3；Design4 §12.4 | ☐ 未开始 |
| 5 | 全链路回归、固定版本证据、浏览器与文档收口 | Build21 §7.11；AGENTS §3.4 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/assembly/render_clash.go`、`backend/internal/assembly/links/links.go`、`backend/internal/assembly/selfcheck.go` 及对应测试 | Clash YAML 输出改为 `plugin` + 结构化 `plugin-opts`；删除旧 URI 字符串形态；自检识别旧格式 |
| 2 | `backend/internal/assembly/node_check.go`、`diagnose.go`、`render_clash.go`、`render_sr.go`、`backend/internal/ssplugin/contract.go` 及 fixtures | 建立共享 SS 插件诊断器；SR/generic 按 partial/unverified/unsupported 产生 warn/skip；Clash 消费同一诊断 |
| 3 | `frontend/src/components/ProtocolFieldEditor.vue`、`frontend/src/utils/nodeFormLayout.ts`、`frontend/src/utils/nodeFeatures.ts`、`frontend/src/views/admin/NodesView.vue` 及前端测试 | 未知插件字符串 map 编辑、JSON 校验、`plugin_not` 条件、`reset_on=plugin` 清空、凭据隔离 |
| 4 | `backend/internal/assembly/links/links.go`、`backend/internal/uriparse/uriparse.go` 及对应测试 | SR VMess/VLESS 补 `tls/peer/alpn/fp/flow/allowInsecure`；`uriparse` 同步回读 |
| 5 | `Issue13.md`、`Design4.md`、`ProdTestList.md`、`AGENTS.md`、`Build21.md`、`Build23.md` 与全量自动化 | 全量回归、固定版本证据、浏览器验证、文档状态同步 |

---

## 三、构建顺序依赖图

```text
Build21 Step 7～10（统一合同/存储/URI 分流）
  ├─→ Step 1（Clash 结构化投影与自检）
  │        ↓
  ├─→ Step 2（SS 插件统一目标诊断；依赖 Step 1 的 Clash 结构可判定）
  │        ↓
  ├─→ Step 3（未知插件前端编辑；依赖 Step 2 的诊断与真实 API 契约）
  │        ↓
  └─→ Step 4（VMess/VLESS SR TLS 补全；与 SS 插件链路相对独立，但放在后端输出稳定后执行）
          ↓
Step 1～4 全部通过 ──→ Step 5（全量回归与文档收口）
```

> 说明：Step 4 与 Step 1～3 没有强耦合，但为了减少输出层同时改动，建议在前三步完成后再实施；若需缩短周期，可在 Step 2 后并行开展，但必须保持同一验收基线。

---

## 四、分步构建计划

### Step 1：Clash/Mihomo 结构化 SS 插件投影与产物自检

- **目标：** 所有 SS 插件在 Clash YAML 中使用纯插件名 + 结构化 `plugin-opts`；项目自检能拒绝旧的 URI 字符串插件格式，消除“可解析但语义错误”的假阳性。
- **前置条件：** Build21 Step 7～10 已通过；`ssplugin` 合同、四个独立插件对象、未知字符串 map 已稳定。
- **研究证据与根因：**
  - `render_clash.go:259-267` 的 `normalizeClashFields()` 对任何非空插件调用 `RenderPluginForClashLegacy`，随后删除 `plugin-opts`、`obfs-opts`、`v2ray-plugin-opts`、`shadow-tls-opts`、`restls-opts`。
  - `links/links.go:603-626` 的 `RenderPluginForClashLegacy` 仍是 Build21 Step 11 前的 legacy 行为：`obfs`→URI 风格 `obfs-local;obfs=...`，`v2ray-plugin`→`v2ray-plugin;mode=...`，未知插件默认走 `pluginString(name, opts)`。
  - 固定版本 Mihomo 模板证据（`docs/DocTemplates/ClashOfficial.yaml.template.md`）显示正确形态是：
    ```yaml
    plugin: obfs
    plugin-opts:
      mode: http
    ```
    以及 `plugin: v2ray-plugin`、`plugin: shadow-tls`、`plugin: restls` 的结构化 `plugin-opts`。
  - `selfcheck.go:39-90` 只检查 `name/type/server/port` 和协议 `Required` 字段，没有检查 SS `plugin` 是否为纯名、`plugin-opts` 是否为 mapping，因此旧字符串形态可通过自检。
- **产出文件与操作：**
  - `backend/internal/assembly/render_clash.go`：新增 `projectSSPluginForClash`（或等价 helper），在 `normalizeClashFields` 的 SS 分支调用；只读取当前插件唯一对象，克隆并输出 `plugin` + `plugin-opts`，补输出默认值后删除内部对象。
  - `backend/internal/assembly/links/links.go`：正式 Clash 路径不再调用 `RenderPluginForClashLegacy`；该函数仅保留为历史兼容参考或后续清理。
  - `backend/internal/assembly/selfcheck.go`：增加 SS 插件结构检查：
    - `plugin` 必须是纯插件名，不得包含 `;`、`=` 或 URI 参数串；
    - 有参数时 `plugin-opts` 必须是 mapping；
    - 已知插件必要字段/枚举由 `ssplugin` 合同检查；
    - 旧字符串形态直接报 error。
- **参考伪代码：**
  ```go
  func projectSSPluginForClash(params map[string]any, plugin string) (map[string]any, error) {
      out := cloneJSONMap(params)
      opts := assemblylinks.PluginOpts(out, plugin)
      if def, ok := ssplugin.Lookup(plugin); ok {
          contract, _ := def.Target(ssplugin.TargetClash)
          for key, value := range contract.Defaults {
              if _, exists := opts[key]; !exists {
                  opts[key] = value
              }
          }
      }
      out["plugin"] = plugin
      if len(opts) > 0 {
          out["plugin-opts"] = opts
      }
      for _, key := range []string{"plugin-opts", "obfs-opts", "v2ray-plugin-opts", "shadow-tls-opts", "restls-opts"} {
          delete(out, key)
      }
      return out, nil
  }
  ```
- **必须新增回归：**
  - 四已知插件 + 未知插件 YAML 解码后精确 map 断言，不用字符串 `contains`；
  - 旧格式 `plugin: obfs-local;obfs=http` 即使可被 YAML / `mihomo -t` 接受，也被 `CheckClashContent` 报 error；
  - 输入 map 深比较保持不变；重复渲染产物稳定；
  - 内部元数据、凭据 ID、`saved_sensitive_paths` 不进入产物。
- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/assembly -count=1 -run 'ClashProxy|SSPlugin|CheckClashContent|NodeCheckFixtures'
  cd backend && go test -race ./internal/assembly -count=1
  cd backend && go build ./... && go vet ./...
  ```
- **验收标准：** 不再出现 `plugin: obfs-local;obfs=http`；正确 `plugin/plugin-opts` 能通过项目自检与固定 Mihomo 正例；缺失必需字段和错误 mode 有精确字段路径诊断。

---

### Step 2：SS 插件统一目标诊断与正式装配门槛

- **目标：** 节点不落库检查与正式 Clash/SR/generic 装配共享同一 SS 插件诊断，消除“检查 `ok`、实际部分支持/未验证/不支持”的分叉。
- **前置条件：** Step 1 已通过；`ssplugin` 合同可判断目标支持级别。
- **研究证据与根因：**
  - `schema.go:22-29` 定义 `TargetEvidence`；`registry.go:461-505` 给四个插件对象写入目标证据；但没有任何后端运行时读取它。
  - `node_check.go:101-152` 的 `linkTargetDiagnostics` 是硬编码：SS 分支只对 `obfs`、未知插件、SS 2022 告警；`v2ray-plugin`、`shadow-tls`、`restls` 没有对应 warn/error 逻辑。
  - `node_check_test.go:180-181` 的 `ss-v2ray-plugin.json` 仍期望 SR/generic `ok`，正是 N-node-5 的实验证据。
  - `ssplugin` 合同事实：
    - `obfs`：Clash complete，SR/generic partial；
    - `v2ray-plugin`：Clash complete，SR/generic partial；
    - `shadow-tls`：Clash complete，SR unverified，generic unsupported；
    - `restls`：Clash complete，SR unverified，generic unsupported；
    - 未知插件：无合同，SR 按未验证透传，generic 不支持。
  - CVR 2.5.2 `src/utils/uri-parser/ss.ts` 只接受 `obfs-local/simple-obfs` 与 `v2ray-plugin`，对 `shadow-tls`、`restls`、未知插件在 generic URI 导入路径就是不可表达。
- **产出文件与操作：**
  - `backend/internal/assembly/node_check.go`：抽出 `diagnoseSSPluginForTarget(target, plugin, params)`，供 `checkClashNodeTarget` 与 `checkLinkNodeTarget` 共用；替换当前 SS 分支硬编码。
  - `backend/internal/assembly/diagnose.go`、`render_clash.go`、`render_sr.go`：继续通过 `CheckNodeTarget` 单一入口消费诊断，不新增第二套规则。
  - `backend/internal/ssplugin`：若需要，补充合同查询辅助方法用于诊断，但保持叶子包不依赖 `node`/`assembly`。
  - 仅消费 SS 插件合同派生的证据；不遍历 VLESS/VMess/Trojan 等非 SS 字段的 `target_evidence`，避免无关注降级。
- **推荐诊断码：** `plugin_partial_mapping`、`plugin_no_verified_mapping`、`plugin_option_unexpressible`、`ss_plugin_shape_invalid`、`ss_plugin_required_field_missing`、`core_semantic_unexpressible`。
- **必须新增回归：**
  - `ss-v2ray-plugin.json`：SR/generic 从 `ok` 改为 `warn`，诊断码 `plugin_partial_mapping` 或等价；
  - 新增 `shadow-tls`、`restls` SR fixture：`warn`；generic fixture：`skip` + `core_semantic_unexpressible`；
  - 新增未知插件 Clash/SR/generic fixture：Clash/SR `warn`，generic `skip`；
  - 同一草稿在节点检查和正式装配中得到相同 severity/code/field_path；
  - 所有 warning 进入装配回执/用户提示，所有核心语义 error 阻断或跳过；
  - 无插件 SS、普通 SS cipher、VLESS/VMess/Trojan 的非 SS 证据不因本 Step 被降级。
- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/assembly ./internal/node ./internal/server -count=1 -run 'NodeCheck|SSPlugin|Diagnostics|Render|ZeroOutput'
  cd backend && go test -race ./internal/assembly ./internal/node ./internal/server -count=1
  cd backend && go build ./... && go vet ./...
  ```
- **验收标准：** 未验证/部分支持不得显示 `ok`；不支持/不可表达进入跳过或阻断；`diagnostics` 保持数组语义，不回归 R27-08 的 null。

---

### Step 3：未知插件参数前端编辑、校验与分支清空

- **目标：** 用户能通过服务端 schema 完整编辑未知插件字符串参数，并保证结构化/JSON 两种模式、保存重开、错误定位、插件切换清空、凭据隔离一致。
- **前置条件：** Step 2 已通过；真实协议 API 已下发 `plugin_not`、`map_value_type=string`。
- **研究证据与根因：**
  - Build21 Step 7 已在前端类型与条件匹配中支持 `plugin_not`、`map_value_type=string`（`api/node.ts`、`nodeFormLayout.ts`）。
  - `ProtocolFieldEditor.vue:132-149` 已校验字符串 map 的 JSON 叶子类型；`:330-344` 已有结构化字符串 map 编辑入口。
  - `NodesView.vue:630-632` 对 `plugin` 切换触发 `applyResetScope('plugin')`；`nodeFeatures.ts` 的 `resetProtocolScope` 可按 `reset_on` 清空路径。
  - 但 Build21 Step 13 尚未正式验收，需补充完整回归：已知→未知、未知→已知、未知 A→未知 B→未知 A、未知→无插件、保存重开、特殊字符、凭据隔离。
- **产出文件与操作：**
  - `frontend/src/components/ProtocolFieldEditor.vue`：确认 `map_value_type=string` 时只渲染字符串 Input；高级 JSON 只接受普通 object 且所有直接值均为 string；错误精确到键。
  - `frontend/src/utils/nodeFormLayout.ts`：确认 `plugin_not` 与后端同语义；不硬编码插件名单。
  - `frontend/src/utils/nodeFeatures.ts`、`frontend/src/views/admin/NodesView.vue`：确认 `reset_on=["plugin"]` 清除通用+四个已知对象、错误状态、未应用 JSON 草稿；A→B→A 不恢复。
  - 未知 `plugin-opts` 不显示“已保存/待替换/已清除”等凭据状态；键名碰巧为 `password/token/secret` 也保持普通字段行为。
- **必须新增回归：**
  - 已知→未知、未知→已知、未知 A→未知 B→未知 A、未知→无插件的可见性与清空；
  - 结构化新增/改名/删除、空字符串 flag、特殊字符、保存重开回显；
  - 高级 JSON 非字符串、数组、对象、数字、布尔反例；错误自动展开所属区域并禁止保存；
  - 真实 `saved_sensitive_paths` 只影响已知固定敏感字段，不影响未知参数。
- **测试与验收命令：**
  ```bash
  cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/node-form-layout.spec.ts tests/node-features.spec.ts tests/nodes-view.spec.ts
  cd frontend && npm run build
  ```
- **验收标准：** 未知插件字符串参数可完整创建、编辑、保存、重开；前后端对非法类型、条件和清空结果一致；现有四协议布局、开关、JSON 草稿与凭据 UI 无回归。

---

### Step 4：VMess/VLESS SR URI TLS/ALPN/指纹/Flow/Skip 输出补全与解析同步

- **目标：** 补齐 SR VMess/VLESS 的外层 TLS 身份、ALPN、客户端指纹、Flow、skip-cert-verify 映射；同步 URI 导入解析，使生成/导入可互相往返。
- **前置条件：** Step 1～3 已通过（避免输出层同时存在插件和 TLS 两套改动导致回归难定位）。
- **研究证据与根因：**
  - `links/links.go:60-72`：VMess SR 只输出 `remarks/udp/alterId/tfo/type/path/host/mode`，没有 `tls/peer/alpn/fp/allowInsecure`。
  - `links/links.go:73-102`：VLESS SR 只在 REALITY 时输出 `tls/xtls/peer/pbk/sid`，在 TLS 时输出 `tls/peer`；没有 `alpn/fp/flow/allowInsecure`。
  - `links/links.go:247-311`：generic VLESS 已输出 `alpn/fp/flow`，但未输出 `allowInsecure`；generic VMess JSON 已输出 `sni/alpn/fp`，但不含 `skip-cert-verify`。
  - `uriparse/uriparse.go:334-391`：`parseVMessSR` 只读取 `tls/peer/type/path/host/mode/fp`，不读取 `alpn` 与 `allowInsecure/skip-cert-verify`；`parseVLESS` 已读取这些字段。
  - 外部源码证据（`urlclash-converter`/CVR 2.5.2）：
    - vless 标准 URI 生成应包含 `flow`、`security`、`sni`、`fp`、`allowInsecure=1`、`alpn`；
    - vmess V2rayN JSON 生成包含 `tls/sni/alpn/fp`；
    - CVR 2.5.2 的 Shadowrocket VMess 解析器只读取 `tls/sni/verify_cert` 等，未读取 `alpn/fp`；用户已确认本次仍输出 `alpn/fp`，但必须作为“生态常见形态 + 真机待验”处理，不能仅凭生成成功宣称完整兼容。
- **产出文件与操作：**
  - `backend/internal/assembly/links/links.go`：
    - 新增 `addSRTLSQuery(params, q)` 或等价 helper，供 VMess/VLESS 分支复用；
    - VMess SR：`tls=1`、`peer=<servername/sni>`、`alpn=<csv>`、`fp=<client-fingerprint>`、`allowInsecure=1`（当 `skip-cert-verify=true`）；
    - VLESS SR：在 TLS/REALITY 分支补 `alpn`、`fp`、`flow`；TLS 分支补 `allowInsecure=1`；REALITY 分支按允许矩阵补 `fp`/`alpn`/`flow`；
    - generic VLESS：补 `allowInsecure=1`；保持现有 `alpn/fp/flow`；
    - generic VMess 按用户确认**不补充** `skip-cert-verify`：保持现状，并在测试/文档中明确记录“该字段未被 generic VMess 输出”，避免后续误加或误宣称完整映射。
  - `backend/internal/uriparse/uriparse.go`：
    - `parseVMessSR` 补读 `alpn`、`allowInsecure`/`skip-cert-verify`；
    - 核对 `parseVLESS` 与生成端字段名一致（当前已读 `alpn/fp/flow/allowInsecure`）。
  - 更新 `TestCanonicalEditorSecurityCheckSaveAndOutput`：VMess 检查目标加入 `sr-subs` 并断言 TLS 语义。
- **必须新增回归：**
  - SR VMess TLS：`tls=1`、`peer`、`alpn`、`fp`、`allowInsecure` 均输出；无 TLS 时不输出；
  - SR VLESS TLS/REALITY：`alpn/fp/flow` 均输出；`skip-cert-verify=true` 时输出 `allowInsecure=1`；
  - generic VLESS：`allowInsecure=1` 输出，既有 `alpn/fp/flow` 不回归；
  - generic VMess 负向断言：当前输出 JSON 不含 `skip-cert-verify`，保持用户确认边界；
  - `uriparse.Parse` 对上述 SR URI 可回读关键字段（至少 alpn/allowInsecure/flow）；
  - 检查状态不与输出字段冲突：添加字段后，已有 fixture 不应把“能生成”误认为“SR 真机已验证”，保留 `ProdTestList` 人工边界。
- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/assembly/links ./internal/uriparse ./internal/assembly -count=1 -run 'SR|Vless|Vmess|Link|Uri'
  cd backend && go test -race ./internal/assembly/links ./internal/uriparse -count=1
  cd backend && go build ./... && go vet ./...
  ```
- **验收标准：** SR/generic 输出覆盖 Design4 §12.4 中已声明为 C 的 TLS 字段；生成→解析可往返；Shadowrocket 真机仍按人工待办处理，不写入“已验证”。

---

### Step 5：全链路回归、固定版本证据、浏览器与文档收口

- **目标：** 以自动化、固定版本离线证据和隔离本地浏览器验证收口本 Build 范围，并同步文档实际状态与人工边界。
- **前置条件：** Step 1～4 全部通过；若任一目标仍出现无诊断参数丢失，不得进入文档“已修复”。
- **自动化矩阵：**
  - 四已知插件+未知插件：创建/更新/数据库/详情/列表/重载、当前状态、插件切换、敏感字段、URI 导入；
  - Clash：精确结构、默认值、不泄漏内部元数据、缺失字段/非法 mode/旧错误字符串反例；
  - SR/generic：SIP002 特殊字符往返、目标支持矩阵、不可表达参数跳过、零输出门槛；
  - VMess/VLESS SR：TLS/ALPN/指纹/Flow/Skip 输出与导入往返；
  - 节点检查与正式装配：相同诊断、`diagnostics: []`、预览脱敏；
  - 固定 Mihomo 1.19.29：正确四插件正例、缺少必需字段反例；保留“旧拼接字符串可能通过内核 `-t`，但项目自检仍拒绝”的证据。
- **本地浏览器：** 最新生产前端构建与隔离临时 Dev 数据库；走真实 API 验证已知/未知插件切换、参数保存重开、目标检查面板、桌面与 375px 手机视口；不连接真实 OIDC/SMTP/Xray 或复用真实凭据。
- **文档同步：**
  - `Issue13.md`：R27-09 状态推进；记录 N-node-1～N-node-5 已处理，N-node-3/4 真机边界仍保留；
  - `Design4.md`：写入最终 SS 插件输出、SR TLS 字段映射与 Shadowrocket 人工验证边界；
  - `ProdTestList.md`：保留 Shadowrocket 真机导入/连接，增加四插件、未知插件、VMess/VLESS SR TLS 组合清单；未执行项不得勾选；
  - `AGENTS.md`、`Build21.md`、`Build23.md`：更新实际状态、文件、命令/结果和版本记录；保留历史验收数据。
- **最终验证命令：**
  ```bash
  cd backend && go test ./... -count=1
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test -race ./internal/node ./internal/uriparse ./internal/assembly/links ./internal/assembly ./internal/server -count=1
  cd frontend && npm test -- --run
  cd frontend && npm run build
  git diff --check
  ```
- **验收标准：** 全部命令通过；固定版本正反例、真实 API 与浏览器核心流程有可复查记录；工作树只包含本轮精确范围内变更；Shadowrocket 真机未执行时明确标记待办，不宣称交付闭环。

---

## 五、候选构建项（待用户决策，逐项转 Step）

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| 1 | Shadowrocket 真机导入/连接验收 | 当前只有生态规范与版本公告证据，无真机连接结果；不属于本 Build 自动化可闭环项 | Design4 §8.5；ProdTestList |
| 2 | 非 SS 字段级 `target_evidence` 全局诊断或前端逐字段证据展示 | 本 Build 已确认仅按 SS 插件合同消费；全局启用会扩大影响面，建议作为后续独立优化 | Build21 §7.2 排除说明 |
| 3 | BuildReport4 未闭环项 1、2、4～6 | Build16/Design3、smoke、安全报告、人工验收等，均不属于本 Build 范围 | BuildReport4 结论摘要 |

> 候选转 Step 流程：用户确认 → 在本文件追加 Step（含目标/前置/参考代码/验收命令）→ 按序执行。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-05 | 首次创建：根据 BuildReport4 未闭环项 3 完成未知 SS 插件输出丢失、target_evidence 未消费、v2ray/shadow/restls URI 诊断误报、VMess/VLESS SR TLS 参数缺失的根因研究、修复方向与分步构建计划；未修改任何业务代码。 |
| v2.0 | 2026-09-05 | 按 `docs/DocTemplates/Build.template.md` 重新排版：补充分步构建的模板结构（进度追踪、文件总览、依赖图、分步计划、候选项、变更记录）；补充 Build21 已落地步骤、CVR/Mihomo 源码证据、fixed-version 正例与真机边界。 |
| v2.1 | 2026-09-05 | 根据进一步源码研究与用户确认，明确 SR VMess 输出包含 `alpn`/`fp` 但保留 Shadowrocket 真机待验证；明确 generic VMess 不补充 `skip-cert-verify` 并加入负向回归。 |
