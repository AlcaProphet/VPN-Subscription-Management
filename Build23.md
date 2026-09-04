# Build23.md — BuildReport4 未闭环项 3：节点输出与诊断缺口研究与构建计划

> **文档定位：** 本文档是依据 [BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) 中“未闭环项 3”开展的研究与下一轮构建计划。当前只完成研究与文档撰写，未修改任何业务代码。
> - 设计依据：[Design4.md](Design4.md)（首批四协议目标检查/输出门槛与组合矩阵）、[Build21.md](Build21.md)（R27-09 Step 7～10 已完成及 Step 11～14 未实施）、[Issue13.md](Issue13.md)（R27-09 当前状态）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序构建：[Build17.md](Build17.md)～[Build22.md](Build22.md)、历史构建存档于 [docs/reports/Build/](docs/reports/Build)
> - 用户已确认（见文末决策记录）：Build23 放仓库根目录；纳入 BuildReport4 N-node-3/N-node-4；`target_evidence` 仅按 SS 插件合同消费；未知 SS 插件在 Clash 输出中保留结构化 `plugin-opts` 并给出未验证警告。

> **执行原则：**
> - 本文档只描述根因、修复方向、可选方案与后续可执行 Step；实施前仍应逐 Step 执行、验收，未完成前不得把 R27-09 / N-node 项标记为“已全部闭环”。
> - 每步新增逻辑必须配套单元测试；测试应先复现缺口，再修改实现。
> - 不处理 BuildReport4 中未闭环项 1、2、4～6（Build16/Design3、R27-08/R27-09 之外的已知待办、smoke 脚本、安全报告、人工验收），也不扩张到非 SS 协议的全局 `target_evidence` 诊断。
> - 所有输出投影必须克隆数据，不得改动 `protocol_json`、历史版本快照或调用方传入 map。

---

## 一、研究结论摘要

BuildReport4 未闭环项 3 的实质是：**节点数据已经有较完整的内部结构化模型（四个 SS 插件独立对象、未知插件字符串 map、目标证据元数据），但“输出适配器”与“目标诊断”仍然没有统一消费这套模型，导致部分输出仍使用旧的 URI 字符串格式，且检查结果会漏报或误报。**

| # | 缺口 | 当前实际状态 | 根因定位 | 影响 |
|---|------|-------------|---------|------|
| N-node-1 | 未知 SS 插件 `plugin-opts` 被静默丢弃 | 存储/URI 导入已由 Build21 Step 8/10 修复；**Clash 输出仍把 `plugin-opts` 拍平成 URI 插件字符串后删除** | `render_clash.go` 的 `normalizeClashFields()` 调用 legacy helper，再删除结构化对象 | 未知插件参数在 Clash YAML 中仍无法以结构化 `plugin-opts` 保留；旧错误格式可能被内核接受且项目自检不报 |
| N-node-2 | `target_evidence` 未被检查链路消费 | 仅作为 `FieldSchema` 元数据下发；后端 check/diagnostic 未读取 | `schema.go`/`registry.go` 只有定义与赋值；`node_check.go` 用硬编码插件名判断 | 无法把 `partial/unverified/unsupported` 证据自动转化为诊断；前端也无证据展示 |
| N-node-3 | VMess SR URI 缺 TLS 相关参数 | 仍缺 `tls`、`peer`、`alpn`、`fp`、`allowInsecure`/`skip-cert-verify` | `links.go` SR vmess 分支只输出 remarks/udp/alterId/type/path/host/mode | Shadowrocket 导入/使用 VMess TLS 节点时丢失外层 TLS 身份与安全参数；与 Design4 §12.4 矩阵 C 不符 |
| N-node-4 | VLESS SR URI 缺 ALPN/client-fingerprint/flow/skip-cert-verify | SR vless 分支只输出 `tls`/`peer`/`xtls`/`pbk`/`sid`，未输出 `alpn`、`fp`、`flow`、`allowInsecure` | `links.go` SR vless 分支未接入完整 TLS/REALITY 查询参数；generic 分支已输出大多数字段但同样缺 `allowInsecure` | SR 端 VLESS TLS/REALITY 连接语义不完整；与 Design4 §12.4 矩阵 C 不符 |
| N-node-5 | v2ray-plugin/shadow-tls/restls 的 URI 检查可能误报 `ok` | `linkTargetDiagnostics()` 只对 obfs、未知插件、SS 2022 告警；`ss-v2ray-plugin` fixture 仍期望 `ok` | 诊断逻辑硬编码，未消费 `ssplugin` 合同的 `SupportPartial/SupportUnverified/SupportUnsupported` | SR 上 v2ray/shadow-tls/restls 可能显示 `ok`，而实际只是部分支持或未验证；检查与正式装配状态不一致 |

**当前已确认事实补充：**

- Build21 Step 7～10 已落地统一 `ssplugin` 合同、SIP002 转义、未知插件存储/URI 导入保留、四个已知插件独立对象与目标证据。因此 N-node-1 的“落库/导入丢失”已经不再成立，剩余问题集中在 **Clash 输出层与自检层**。
- Build21 Step 11～14 是 R27-09 的既定后续：Step 11 修 Clash 结构化投影与自检，Step 12 修 SS 插件目标诊断/装配门槛，Step 13 修未知插件前端编辑，Step 14 全量回归与文档收口。本 Build23 覆盖这些 Steps，并将之前被 Build21 明确排除的 N-node-3/N-node-4 作为独立 Step 纳入。
- BuildReport4 记录的 `RenderPluginForTarget 忽略 target` 在当前 HEAD 已修复：`renderPluginForTarget` 已按 SR/generic 分支消费 `target`。但仍不能说明 Clash 输出已正确。

---

## 二、现状核验与根因分析

### 2.1 N-node-1：Clash 输出仍拍平/删除结构化插件参数

当前数据链路：

```text
protocol_json 内部形态
  ├─ 已知插件：plugin=obfs / v2ray-plugin / shadow-tls / restls
  │            + obfs-opts / v2ray-plugin-opts / shadow-tls-opts / restls-opts
  └─ 未知插件：plugin=custom
              + plugin-opts: map[string]string

→ render_clash.go normalizeClashFields()
→ 若 plugin 非空：
    opts = assemblylinks.PluginOpts(out, plugin)
    out["plugin"] = assemblylinks.RenderPluginForClashLegacy(plugin, opts)   // 变成字符串
    删除 plugin-opts / obfs-opts / v2ray-plugin-opts / shadow-tls-opts / restls-opts
→ marshalClashYAML()
```

关键代码事实：

| 位置 | 事实 |
|------|------|
| `backend/internal/assembly/render_clash.go:259-267` | SS 分支无条件调用 `RenderPluginForClashLegacy`，并把所有内部插件对象删除。 |
| `backend/internal/assembly/links/links.go:603-626` | `RenderPluginForClashLegacy`：`obfs`→`obfs-local;obfs=...`；`v2ray-plugin`→`v2ray-plugin;mode=...`；`default`→`pluginString(name, opts)`，即 `name;key=value` URI 形态。 |
| `backend/internal/assembly/links/links.go:476-490` | `pluginString` 用 `fmt.Sprint` 拼接，不是结构化的 `plugin-opts` map。 |
| `backend/internal/assembly/selfcheck.go:39-90` | `CheckClashContent` 只检查 `name/type/server/port` 与协议注册表 `Required` 字段；没有检查 SS `plugin` 是纯插件名、`plugin-opts` 是 mapping。 |

影响：

1. 未知插件的字符串 map 在 Clash 产物中被拆成 `plugin: custom;key=value` 并删除 `plugin-opts`，客户端无法按结构化插件参数消费。
2. 已知插件也输出 Mihomo 不期望的旧 URI 字符串；Build21 已证明该形态可能被 `mihomo -t` 接受，形成“看起来可解析但语义错误”的假阳性。
3. `CheckClashContent` 不识别这种旧格式，因此节点检查仍可能返回 `ok`。

### 2.2 N-node-2 / N-node-5：目标证据与 SS 插件诊断没有统一入口

当前 `target_evidence` 事实：

- `backend/internal/node/schema.go:22-29` 定义 `TargetEvidence`，状态为 `complete|equivalent|partial|unsupported|unverified`。
- `backend/internal/node/registry.go:461-505` 的 `applySSPluginContract()` 给四个插件对象写入目标证据：Clash/Mihomo 1.19.29、Shadowrocket 未验证、generic/CVR 2.5.2 支持级别。
- `backend/internal/node/registry.go:602-604, 624-626, 662, 690, 709-711, 777-779` 还在 VLESS/VMess/Trojan/SS 的部分字段上标注了证据。
- `frontend/src/api/node.ts:44-50, 76` 只定义了类型；前端没有读取或展示 `target_evidence`。

当前诊断事实：

- `backend/internal/assembly/node_check.go:101-152` 的 `linkTargetDiagnostics()` 是 URI 检查诊断的唯一入口，但只按 `protocol` 和少数硬编码条件生成诊断。
- SS 分支只对 `obfs`、未知插件、SS 2022 告警；`v2ray-plugin`、`shadow-tls`、`restls` 不在告警列表。
- `checkClashNodeTarget()` 只调用 `CheckClashContent`，没有调用 SS 插件专属诊断，因此 Clash 对未知/未验证插件也可能没有提示。
- 正式装配 `diagnose.go` 通过 `CheckNodeTarget()` 复用同一路径，所以只要检查端漏报，正式装配也会漏报。

现有测试证据：

- `backend/internal/assembly/testdata/node_check/ss-v2ray-plugin.json` 只有 `mode/host/tls/path`，`TestNodeCheckFixtures` 将其期望为 SR/generic `ok`（`node_check_test.go:180-181`）。
- 没有 `shadow-tls`、`restls`、未知插件在 SR/generic 上的系统化 fixture。
- `backend/internal/assembly/links/links_test.go` 已覆盖 SR/generic 对不支持/不可表达参数的渲染错误（第二道防线），但目标检查状态仍可能先返回 `ok`。

### 2.3 N-node-3 / N-node-4：VMess/VLESS SR URI 输出不完整

关键代码事实：

| 位置 | 事实 |
|------|------|
| `backend/internal/assembly/links/links.go:60-72` | VMess SR 分支只输出 `remarks/udp/alterId/tfo/type/path/host/mode`，没有 `tls/peer/alpn/fp/allowInsecure`。 |
| `backend/internal/assembly/links/links.go:73-102` | VLESS SR 分支只在 REALITY 时输出 `tls/xtls/peer/pbk/sid`，在 TLS 时输出 `tls/peer`；没有 `alpn/fp/flow/allowInsecure`。 |
| `backend/internal/assembly/links/links.go:247-311` | generic VLESS 已输出 `alpn/fp/flow`，但未输出 `allowInsecure`；generic VMess JSON 已输出 `sni/alpn/fp`，但不含 `skip-cert-verify`。 |
| `backend/internal/uriparse/uriparse.go:334-391` | `parseVMessSR` 只读取 `tls/peer/type/path/host/mode/fp`，不读取 `alpn` 与 `allowInsecure/skip-cert-verify`；`parseVLESS` 已读取这些字段。 |

这与 Design4 的矩阵不一致：

- Design4 §12.4 VLESS `TCP+TLS` 标注 SR/generic URI `C`，表单字段含 SNI、ALPN、client-fingerprint、skip-cert-verify。
- Design4 §12.4 VMess `TCP+TLS` 标注 SR/generic URI `C`，表单字段含 SNI、ALPN、client-fingerprint。
- 当前 SR 输出无法把这些字段完整带到 Shadowrocket，属于“适配器声明完整但实际缺字段”的实质缺口。

### 2.4 关联缺口：Clash 结构自检与未知插件前端

- Clash 结构自检缺失是 N-node-1 的“假阳性”放大器；即使 `mihomo -t` 接受，项目自身 `CheckClashContent` 也应识别旧字符串插件形态。
- 未知插件前端编辑在 Build21 Step 7 已有基础（`map_value_type=string`、`plugin_not`、字符串 map 结构化/JSON 校验），但 Step 13 尚未正式验收；Build23 应把它作为独立 Step，避免“后端能保存、前端不能完整编辑”的分叉。

---

## 三、修复方向与方案对比

### 3.1 方向 A：Clash/Mihomo 结构化插件投影 + 产物自检（对应 R27-09 Step 11）

**推荐。** 在 `render_clash.go` 新增 `projectSSPluginForClash`（或等价函数）：

- 只读取当前活动插件对应的唯一对象（已知插件读 `obfs-opts/v2ray-plugin-opts/shadow-tls-opts/restls-opts`，未知插件读 `plugin-opts`）。
- 将 `out["plugin"]` 设为原始插件名，`out["plugin-opts"]` 设为结构化 map。
- 对 `obfs.mode=http`、`v2ray-plugin.mode=websocket` 等默认值仅在克隆后的输出对象补缺，不写回数据库。
- 删除内部编辑键，保持输出键序稳定。
- 在 `selfcheck.go` 增加 SS 结构检查：`plugin` 是纯插件名（不含 `;`/`=`/URI 分隔符）；有参数时 `plugin-opts` 必须是 mapping；已知插件必要字段/枚举由 `ssplugin` 合同检查；旧字符串形态报 error。

**优点：** 直接修复 N-node-1 与 Clash 假阳性；与 Build21 既定方案一致；输出可被固定 Mihomo 正例验证。
**风险/代价：** 需要精确维护 `ssplugin` 合同与 YAML 字段映射；曾把 `plugin-opts` 作为普通 map 的旧数据应保留，不能因修复而改写历史。

### 3.2 方向 B：SS 插件专属目标诊断，消费合同派生证据（对应 R27-09 Step 12 / N-node-2/5）

**推荐（用户已确认：仅 SS 插件合同证据）。** 新增 `diagnoseSSPluginForTarget(target, plugin, params)`：

- `plugin==""`：不诊断。
- 未知插件：
  - Clash：warning `plugin_no_verified_mapping`，仍允许输出结构化参数。
  - SR：warning `plugin_no_verified_mapping`，仍允许输出（SIP002 可无损时）。
  - generic：error `core_semantic_unexpressible`，复用现有跳过/阻断门槛。
- 已知插件：
  - 目标合同不存在或 `SupportUnsupported`：error。
  - `SupportUnverified`：warning，如 `plugin_no_verified_mapping` / `unverified_compatibility`。
  - `SupportPartial`：warning，如 `plugin_partial_mapping`。
  - `SupportComplete`：无 warning；缺失必需字段继续由现有 `ValidateCurrentStateForTarget`/`required_when` 处理。
  - SR/generic 中出现合同未声明可无损表达的字段时，可提前给出带 `field_path` 的 error，避免只依赖渲染器返回的通用错误。
- 在 `checkClashNodeTarget()`、`checkLinkNodeTarget()`、正式装配 `diagnoseNodeForTarget()` 共用同一诊断器。

**优点：** 状态与 Design4 §7.4 一致（未验证/部分支持不再是 `ok`）；只影响 SS 插件，不把 VLESS/VMess/Trojan 普通字段降级；与 Build21 的“不全局启用 target_evidence”决策吻合。
**风险/代价：** 需要同步调整现有 fixture（`ss-v2ray-plugin` 从 `ok` 改为 `warn`），否则测试会与“检查不误报”冲突。

### 3.3 方向 C：VMess/VLESS SR URI 完整 TLS/ALPN/指纹/Flow/Skip 映射 + 解析同步（N-node-3/4）

**推荐。** 在 `links.go` 的 SR 分支补全查询参数，并同步 `uriparse` 的 SR 解析器：

- VMess SR：`tls=1`、`peer=<servername/sni>`、`alpn=<csv>`、`fp=<client-fingerprint>`、`allowInsecure=1`（当 `skip-cert-verify=true`）。
- VLESS SR：在现有 `tls/xtls/peer/pbk/sid` 基础上增加 `alpn`、`fp`、`flow`、`allowInsecure=1`；REALITY 与 TLS 分支都补 `alpn/fp/flow`（REALITY 的 skip-cert-verify 按目标矩阵允许时再处理）。
- generic VLESS：补 `allowInsecure=1`，使标准 URI 与 `Node-Link-Standards.md` 的 vless 生成规则一致；generic VMess 是否增加 `skip-cert-verify` 需按 `urlclash-converter`/CVR JSON 规范确认，不能仅凭“有字段就应该输出”添加。
- `uriparse.parseVMessSR` 增加 `alpn`、`allowInsecure`/`skip-cert-verify` 读取；`uriparse.parseVLESS` 已具备读取能力，只需与生成端互相验证。

**优点：** 补齐 Design4 已声明为 C 的 SR 输出；使导入/导出往返更完整；通用 VLESS 也补齐 `allowInsecure` 标准参数。
**风险/代价：** Shadowrocket 真机仍是未验证证据；必须在文档和 `ProdTestList` 中保留“SR 仅按生态规范输出，真机导入/连接待人工验收”，不能因 URI 可生成而宣称全部完成。

### 3.4 方向 D：未知插件前端编辑与全链路回归（对应 R27-09 Step 13/14）

**推荐。** 完善并验收 `ProtocolFieldEditor` 的字符串 map 控件、`nodeFormLayout` 的 `plugin_not` 条件、`reset_on=["plugin"]` 清空逻辑；用真实 API 验证未知插件保存→重开→检查→输出全链路。

**优点：** 闭合“后端能保存、前端不能编辑”的体验缺口。
**风险/代价：** 前端改动需要回归四协议布局；应保持未知插件参数为普通字符串，不进入凭据/敏感路径。

---

## 四、构建计划（候选 Step，需逐项实施）

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。下列 Step 为后续实施时的推荐拆分；实际执行仍按“一次一个 Step、先失败回归、后实现、再全量验证”的规则。

### Step 1：Clash/Mihomo 结构化 SS 插件投影与产物自检（R27-09 Step 11 / N-node-1）

- **目标：** 所有 SS 插件在 Clash YAML 中使用纯插件名 + 结构化 `plugin-opts`；项目自检能拒绝旧的 URI 字符串插件格式。
- **前置条件：** Build21 Step 7～10 已通过；`ssplugin` 合同稳定。
- **产出文件与操作：**
  - `backend/internal/assembly/render_clash.go`：新增 `projectSSPluginForClash`（等同 Build21 Step 11 参考），在 `normalizeClashFields` 的 SS 分支调用；只读当前插件对象，克隆并输出 `plugin` + `plugin-opts`，补输出默认值后删除内部对象。
  - `backend/internal/assembly/links/links.go`：`RenderPluginForClashLegacy` 不再被正式 Clash 路径调用；可保留为兼容辅助或移除（由后续清理决定），但正式路径不得使用。
  - `backend/internal/assembly/selfcheck.go`：增加 SS 插件结构检查（纯插件名、`plugin-opts` 为 map、已知插件必要字段/枚举、旧字符串报 error）。
  - 删除/更新 `TestNormalizeClashListFields` 附近或新增结构化断言测试。
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
  - 四个已知插件+未知插件 YAML 解码后精确 map 断言，不用字符串 `contains`。
  - 旧格式 `plugin: obfs-local;obfs=http` 即使可被 YAML/`mihomo -t` 接受，也被 `CheckClashContent` 报 error。
  - 输入 map 深比较保持不变；重复渲染产物稳定；内部元数据/凭据 ID/`saved_sensitive_paths` 不进入产物。
- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/assembly -count=1 -run 'ClashProxy|SSPlugin|CheckClashContent|NodeCheckFixtures'
  cd backend && go test -race ./internal/assembly -count=1
  cd backend && go build ./... && go vet ./...
  ```
- **验收标准：** 不再出现 `plugin: obfs-local;obfs=http`；正确 `plugin/plugin-opts` 可通过项目自检与固定 Mihomo 正例；缺少必需字段/错误枚举有可定位诊断。

### Step 2：SS 插件通用目标诊断与正式装配门槛（R27-09 Step 12 / N-node-2/5）

- **目标：** 节点不落库检查与正式 Clash/SR/generic 装配共享同一 SS 插件诊断，消除“检查 `ok`、实际部分支持/未验证/不支持”的分叉。
- **前置条件：** Step 1 已通过；`ssplugin` 合同可判断目标支持级别。
- **产出文件与操作：**
  - `backend/internal/assembly/node_check.go`：抽出 `diagnoseSSPluginForTarget`，在 `checkClashNodeTarget` 与 `checkLinkNodeTarget` 中同时调用；替换当前 `linkTargetDiagnostics` 中 SS 分支的硬编码判断。
  - `backend/internal/assembly/diagnose.go`、`render_clash.go`、`render_sr.go`：继续保持 `CheckNodeTarget` 单一入口，不新增第二套规则。
  - `backend/internal/node/registry.go`：保留/核对 `TargetEvidence` 仅作 schema 展示；本 Step 只消费 `ssplugin` 合同，不遍历所有字段证据。
- **推荐诊断码：** `plugin_partial_mapping`、`plugin_no_verified_mapping`、`plugin_option_unexpressible`、`ss_plugin_shape_invalid`、`ss_plugin_required_field_missing`、`core_semantic_unexpressible`。
- **必须新增回归：**
  - `ss-v2ray-plugin.json`：SR/generic 从 `ok` 改为 `warn`，诊断码 `plugin_partial_mapping` 或等价。
  - 新增 `shadow-tls`、`restls` SR fixture：`warn`；generic fixture：`skip` + `core_semantic_unexpressible`。
  - 新增未知插件 Clash/SR/generic fixture：Clash/SR `warn`，generic `skip`。
  - 同一草稿在节点检查和正式装配中得到相同 severity/code/field_path；所有 warning 进入装配回执；所有核心语义 error 阻断或跳过。
  - 无插件 SS、普通 SS cipher、VLESS/VMess/Trojan 的非 SS 证据不因本 Step 被降级。
- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/assembly ./internal/node ./internal/server -count=1 -run 'NodeCheck|SSPlugin|Diagnostics|Render|ZeroOutput'
  cd backend && go test -race ./internal/assembly ./internal/node ./internal/server -count=1
  cd backend && go build ./... && go vet ./...
  ```
- **验收标准：** 未验证/部分支持不得显示 `ok`；不支持/不可表达进入跳过或阻断；`diagnostics` 保持数组语义，不回归 R27-08 的 null。

### Step 3：未知插件参数前端编辑、校验与分支清空（R27-09 Step 13）

- **目标：** 用户能通过服务端 schema 完整编辑未知插件字符串参数，并保证结构化/JSON 两种模式、保存重开、错误定位、插件切换清空、凭据隔离一致。
- **前置条件：** Step 2 已通过；真实协议 API 已下发 `plugin_not` 与 `map_value_type=string`。
- **产出文件与操作：**
  - `frontend/src/components/ProtocolFieldEditor.vue`：确认 `map_value_type=string` 时只渲染字符串 Input；高级 JSON 只接受普通 object 且所有直接值均为 string；错误精确到键。
  - `frontend/src/utils/nodeFormLayout.ts`：确认 `plugin_not` 与后端同语义；不硬编码插件名单。
  - `frontend/src/views/admin/NodesView.vue`、`frontend/src/utils/nodeFeatures.ts`：确认 `reset_on=["plugin"]` 清除通用+四个已知对象、错误状态、未应用 JSON 草稿；A→B→A 不恢复。
  - 未知 `plugin-opts` 不显示“已保存/待替换/已清除”等凭据状态；键名碰巧为 `password/token/secret` 也保持普通字段行为。
- **必须新增回归：**
  - 已知→未知、未知→已知、未知 A→未知 B→未知 A、未知→无插件的可见性与清空。
  - 结构化新增/改名/删除、空字符串 flag、特殊字符、保存重开回显。
  - 高级 JSON 非字符串、数组、对象、数字、布尔反例；错误自动展开所属区域并禁止保存。
  - 真实 `saved_sensitive_paths` 只影响已知固定敏感字段，不影响未知参数。
- **测试与验收命令：**
  ```bash
  cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/node-form-layout.spec.ts tests/node-features.spec.ts tests/nodes-view.spec.ts
  cd frontend && npm run build
  ```
- **验收标准：** 未知插件字符串参数可完整创建、编辑、保存、重开；前后端对非法类型、条件和清空结果一致；现有四协议布局、开关、JSON 草稿与凭据 UI 无回归。

### Step 4：VMess/VLESS SR URI TLS/ALPN/指纹/Flow/Skip-cert 输出补全与解析同步（N-node-3/4）

- **目标：** 补齐 SR VMess/VLESS 的外层 TLS 身份、ALPN、客户端指纹、Flow、skip-cert-verify 映射；同步 URI 导入解析，使生成/导入可互相往返。
- **前置条件：** Step 1～3 已通过（避免在输出层同时存在插件和 TLS 两套改动导致回归难定位）。
- **产出文件与操作：**
  - `backend/internal/assembly/links/links.go`：
    - 新增 `addSRTLSQuery(params, q)` 或等价 helper，供 VMess/VLESS 分支复用。
    - VMess SR：`tls=1`、`peer`、`alpn`、`fp`、`allowInsecure=1`。
    - VLESS SR：在 TLS/REALITY 分支补 `alpn`、`fp`、`flow`；TLS 分支补 `allowInsecure=1`；REALITY 分支按允许矩阵补 `fp`/`alpn`/`flow`。
    - generic VLESS：补 `allowInsecure=1`；保持现有 `alpn/fp/flow`。
  - `backend/internal/uriparse/uriparse.go`：
    - `parseVMessSR` 补读 `alpn`、`allowInsecure`/`skip-cert-verify`。
    - 核对 `parseVLESS` 与生成端字段名一致（当前已读 `alpn/fp/flow/allowInsecure`）。
  - 更新 `TestCanonicalEditorSecurityCheckSaveAndOutput`：VMess 检查目标加入 `sr-subs` 并断言 TLS 语义。
- **必须新增回归：**
  - SR VMess TLS：`tls=1`、`peer`、`alpn`、`fp`、`allowInsecure` 均输出；无 TLS 时不输出。
  - SR VLESS TLS/REALITY：`alpn/fp/flow` 均输出；`skip-cert-verify=true` 时输出 `allowInsecure=1`。
  - generic VLESS：`allowInsecure=1` 输出，既有 `alpn/fp/flow` 不回归。
  - `uriparse.Parse` 对上述 SR URI 可回读关键字段（至少 alpn/allowInsecure/flow）。
  - 检查状态不与输出字段冲突：添加字段后，已有 fixture 不应把“能生成”误认为“SR 真机已验证”，保留 `ProdTestList` 人工边界。
- **测试与验收命令：**
  ```bash
  cd backend && go test ./internal/assembly/links ./internal/uriparse ./internal/assembly -count=1 -run 'SR|Vless|Vmess|Link|Uri'
  cd backend && go test -race ./internal/assembly/links ./internal/uriparse -count=1
  cd backend && go build ./... && go vet ./...
  ```
- **验收标准：** SR/generic 输出覆盖 Design4 §12.4 中已声明为 C 的 TLS 字段；生成→解析可往返；Shadowrocket 真机仍按人工待办处理，不写入“已验证”。

### Step 5：全链路回归、固定版本证据、浏览器与文档收口（R27-09 Step 14）

- **目标：** 以自动化、固定版本离线证据和隔离本地浏览器验证收口本 Build 范围，并同步文档实际状态与人工边界。
- **前置条件：** Step 1～4 全部通过；若任一目标仍出现无诊断参数丢失，不得进入文档“已修复”。
- **自动化矩阵：**
  - 四已知插件+未知插件：创建/更新/数据库/详情/列表/重载、当前状态、插件切换、敏感字段、URI 导入。
  - Clash：精确结构、默认值、不泄漏内部元数据、缺失字段/非法 mode/旧错误字符串反例。
  - SR/generic：SIP002 特殊字符往返、目标支持矩阵、不可表达参数跳过、零输出门槛。
  - VMess/VLESS SR：TLS/ALPN/指纹/Flow/Skip 输出与导入往返。
  - 节点检查与正式装配：相同诊断、`diagnostics: []`、预览脱敏。
  - 固定 Mihomo 1.19.29：正确四插件正例、缺少必需字段反例；保留“旧拼接字符串可能通过内核 `-t`，但项目自检仍拒绝”的证据。
- **本地浏览器：** 最新生产前端构建与隔离临时 Dev 数据库；走真实 API 验证已知/未知插件切换、参数保存重开、目标检查面板、桌面与 375px 手机视口；不连接真实 OIDC/SMTP/Xray 或复用真实凭据。
- **文档同步：**
  - `Issue13.md`：R27-09 状态推进；记录 N-node-1～N-node-5 已处理，N-node-3/4 真机边界仍保留。
  - `Design4.md`：写入最终 SS 插件输出、SR TLS 字段映射与 Shadowrocket 人工验证边界。
  - `ProdTestList.md`：保留 Shadowrocket 真机导入/连接，增加四插件、未知插件、VMess/VLESS SR TLS 组合清单；未执行项不得勾选。
  - `AGENTS.md` 与 `Build21.md`：更新实际状态、文件、命令/结果和版本记录；保留历史验收数据。
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

## 五、测试矩阵与预期变化

| 缺口 | 测试资产 | 修复前 | 修复后 |
|------|---------|--------|--------|
| N-node-1 | Clash SS plugin 结构化断言 | 旧字符串 + 删除 plugin-opts | `plugin` 纯名 + `plugin-opts` map；自检拒旧格式 |
| N-node-2 | `TargetEvidence` 合同/状态测试 | 只有 schema 元数据 | SS 插件诊断消费合同证据；非 SS 证据不进入状态 |
| N-node-3 | SR VMess TLS URI + 导入 | 无 TLS 参数 | 输出 `tls/peer/alpn/fp/allowInsecure`，解析可回读 |
| N-node-4 | SR VLESS TLS/REALITY URI + 导入 | 缺 `alpn/fp/flow/allowInsecure` | 输出完整，解析可回读 |
| N-node-5 | `ss-v2ray-plugin` / shadow-tls / restls / unknown fixture | v2ray 误报 ok | 按合同得到 warn/skip；正式装配同状态 |

---

## 六、已确认决策与待办边界

| 决策点 | 用户确认 |
|--------|---------|
| Build23 位置 | 仓库根目录 `Build23.md` |
| 研究/计划范围 | 覆盖 BuildReport4 未闭环项 3（N-node-1～N-node-5），并纳入 Build21 曾排除的 N-node-3/N-node-4 |
| `target_evidence` 消费范围 | 仅按 SS 插件合同派生诊断，不全局启用所有字段级 `target_evidence` |
| 未知插件 Clash 输出 | 保留结构化 `plugin` + `plugin-opts`，并给出未验证 warning，不阻断 |

**不在本 Build 范围：**

- BuildReport4 未闭环项 1（Build16/Design3 实质缺口）、2（R27-08/R27-09 之外的已知待办）、4～6（smoke、安全、人工验收）。
- 非 SS 协议的字段级 `target_evidence` 全局诊断或前端逐字段证据展示（可作为后续独立优化）。
- 新增 SQL migration、重写历史节点、升级 `state_format_version`、未知插件动态敏感路径。
- Shadowrocket 真机导入/连接验收；它继续保留在 `ProdTestList.md` 人工待办。
- Build22 所述的 Design3/素材池相关修复。

---

## 七、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-05 | 根据 BuildReport4 未闭环项 3，完成未知 SS 插件输出丢失、target_evidence 未消费、v2ray/shadow/restls URI 诊断误报、VMess/VLESS SR TLS 参数缺失的根因研究、修复方向与分步构建计划；未修改任何业务代码。 |
