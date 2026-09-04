# Build21.md — BuildReport3 节点编辑器对齐修复构建计划与实施记录

> **文档定位：** 本文档是依据 [BuildReport3.md](docs/reports/BuildReport/BuildReport3.md) 开展的第二十一轮构建记录，将“Build17~Build20 表单结构与 Design4 对齐”发现的问题按用户确认的推荐方案落地。
> - 设计依据：[Design4.md](Design4.md)（当前最新节点编辑器设计）、[BuildReport3.md](docs/reports/BuildReport/BuildReport3.md)
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序构建：[Build17.md](Build17.md)～[Build20.md](Build20.md)、历史构建存档于 [docs/reports/Build/](docs/reports/Build)
>
> **本文件状态：** 原 Step 1～6 已完成并通过验收；R27-09 全量扩展补充方案已确认，Step 7～8 已实施并通过验收，Step 9～14 尚未实施。原 Build21 验收事实继续保留，不以补充计划倒写为“未完成”。

---

## 一、构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 1 | 后端字段分组/条件/排序/选项元数据修正 | ✅ 验收通过 |
| 2 | security 编辑回显与 current_state 回填 | ✅ 验收通过 |
| 3 | ProtocolFieldEditor 可编辑下拉、列表编辑器、help/文案 | ✅ 验收通过 |
| 4 | NodesView 分区重构：开关折叠、高级数据与目标检查折叠 | ✅ 验收通过 |
| 5 | SS 插件目标映射与输出层活动投影 | ✅ 验收通过 |
| 6 | 前后端测试与构建验证 | ✅ 验收通过 |
| 7 | R27-09 SS 插件统一契约、补集条件与字符串映射 schema | ✅ 验收通过 |
| 8 | R27-09 新旧参数幂等归一化、未知参数保存与回显 | ✅ 验收通过 |
| 9 | R27-09 四个已知插件字段与固定敏感路径修正 | ☐ 未开始 |
| 10 | R27-09 SIP002 转义/解析与 SR/generic 目标分流 | ☐ 未开始 |
| 11 | R27-09 Clash/Mihomo 结构化插件投影与产物自检 | ☐ 未开始 |
| 12 | R27-09 SS 插件专属目标诊断与正式装配门槛 | ☐ 未开始 |
| 13 | R27-09 未知插件参数前端编辑、校验与分支清空 | ☐ 未开始 |
| 14 | R27-09 全链路回归、固定版本证据、浏览器与文档收口 | ☐ 未开始 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/node/registry.go` | `FieldSchema.advanced`；安全/TLS/REALITY 归入 connection；TLS/REALITY 条件分离；security 排序；选项元数据补充 |
| 2 | `frontend/src/views/admin/NodesView.vue` | `openEdit()` 从 `current_state` 回填 security/network/plugin |
| 3 | `frontend/src/components/ProtocolFieldEditor.vue`、`EditableCombobox.vue` | text+option_items 进入可编辑下拉；列表增删编辑器；长文本 TextArea；help 全控件渲染；下拉显示 label/中文分组 |
| 4 | `frontend/src/views/admin/NodesView.vue` | 独立开关“常用/更多”折叠；高级数据与目标检查合并为默认折叠区；文案修正 |
| 5 | `backend/internal/assembly/load.go`、`links.go`、`diagnose.go`、`render_clash.go`、`assembly/links/links.go` | 输出前按 current_state 投影；SS 插件按目标映射；Clash 侧插件字符串收口 |
| 6 | `backend/internal/node/project_test.go`、`frontend/tests/nodes-view.spec.ts` | 分组/排序/回填回归测试 |
| 7 | `backend/internal/ssplugin/`、`backend/internal/node/schema.go`、`registry.go`、`project.go`、`node.go`、`server/node_test.go`；`frontend/src/api/node.ts`、`utils/nodeFormLayout.ts`、`components/ProtocolFieldEditor.vue` 及测试 | 建立无循环依赖的 SS 插件契约；增加 `plugin_not` 与 `map_value_type=string`，只在未知插件分支激活 `plugin-opts`；前后端拒绝非字符串叶子 |
| 8 | `backend/internal/node/normalize.go`、`project.go`、`node.go`、`check.go`、`uri_import.go` 及测试 | 已知旧对象只补缺、新对象优先；未知 `plugin-opts` 原样保存/回显；不做数据库迁移或启动重写 |
| 9 | `backend/internal/node/registry.go`、敏感字段/投影相关测试 | 对齐 Mihomo 1.19.29 的四插件字段、默认值、目标必需项；新增私钥固定敏感路径，未知参数仍为普通明文参数 |
| 10 | `backend/internal/ssplugin/`、`backend/internal/uriparse/uriparse.go`、`backend/internal/assembly/links/links.go` 及测试 | SIP002 插件字符串无损转义与反向解析；SR 与 generic 不再共用同一成功结论 |
| 11 | `backend/internal/assembly/render_clash.go`、`selfcheck.go` 及测试 | Clash 输出独立 `plugin` 与 `plugin-opts`；删除内部对象前先投影；拒绝 URI 字符串伪装成 Clash 插件名 |
| 12 | `backend/internal/assembly/node_check.go`、`diagnose.go`、正式渲染路径及测试 | 同一 SS 插件判定服务节点检查和正式装配；只消费 SS 插件证据，不全局启用字段级 `target_evidence` |
| 13 | `frontend/src/components/ProtocolFieldEditor.vue`、`NodesView.vue`、相关工具与测试 | 未知插件字符串 map 的结构化/JSON 编辑、非字符串拒绝、A→B→A 清空与凭据状态隔离 |
| 14 | 前后端相关测试、`Design4.md`、`Issue13.md`、`ProdTestList.md`、`AGENTS.md`、本文件 | 全量验证与固定版本正反例；同步状态和人工边界，不宣称 Shadowrocket 真机兼容已完成 |

---

## 三、验证记录

```text
cd backend && go build ./... && go vet ./... && go test ./... -count=1
# 全部通过

cd frontend && npm run build
# vue-tsc + vite build 通过（仅有既有 chunk 体积提示）

cd frontend && npm test -- --run
# 39 files / 163 tests 全部通过
```

---

## 四、遗留与后续

- 本节前三项是原 Step 1～6 完成时的历史验收边界；其中 SS 插件输出与诊断的后续处理以第七节 R27-09 补充计划为准。
- VMess REALITY、Trojan REALITY 仍按“后续候选”处理，不在首批开放表单入口。
- `shadow-tls`/`restls` 插件仍保留原格式，由目标检查诊断提示未验证，未宣称完整支持。
- 非首批 15 协议的详细分区/字段分组仍有进一步按语义收敛的空间。
- 客户端真机导入/连接验证仍按 Design4 保留为人工验收。

---

## 五、R27-04 补充修复 Step（2026-09-03）

- **目标与前置：** 按 Issue13 已确认方案移除 legacy 可编辑入口，不保留只读迁移提示；内部存储仍使用 `tls`，因此表单目录投影不得改变内部字段白名单。仅实施 R27-04。
- **产出与实现：**
  - `registry.go` 的 `editorFormSchema()` 由 `Service.GetProtocols()` 调用，排除 VLESS 的顶层 WS/TLS 旧入口与 VMess 的顶层 TLS 入口；保留 19 协议目录及其他协议/插件的有效 TLS 字段。
  - `normalizeProtocolParameters()` 共用 WS 别名转换，覆盖读取、检查、保存、活动输出；读取只处理响应副本，规范值优先、别名只补缺项。
  - `NodesView.openEdit()` 从当前状态回填 `security` 后删除 VLESS/VMess 草稿中的顶层 `tls`；后端既有存储转换、重置与凭据保护继续生效。
- **参考流程：** `内部协议目录 → 可编辑 schema 投影 → 当前状态回填/移除草稿 tls → security 切换与 reset_scopes → 检查/保存 → 既有目标投影`。
- **TODO / 验收：**
  - [x] 真实 HTTP 协议响应无重复入口，内部 TLS 校验能力不变，其他协议和插件 TLS 字段保留。
  - [x] 旧 WS 值与冲突规范值的列表/详情回显、检查、保存一致；读取和检查不回写、更新保留凭据并递增修订。
  - [x] VLESS/VMess TLS 安全切换与检查/保存一致；实际 Clash YAML 与 generic URI 安全字段断言通过，VLESS 另覆盖 SR URI 与 REALITY。
  - [x] 最新构建浏览器走查：VLESS TCP/WS、TLS/REALITY 的重复入口消失；VLESS/VMess 创建、TLS 回显、切到 none 后保存/重开通过。
  - [x] 后端 `go test ./... -count=1`、`go build ./...`、`go vet ./...`；前端 `npm test`（40 文件 / 183 用例）、`npm run build`；`git diff --check`。
- **验证边界：** 浏览器使用本轮生产前端构建、真实后端接口及全新临时 Dev 数据库；临时 Go overlay 仅将测试后端监听限制到 `127.0.0.1`，未更改认证/业务行为。目标检查面板交互及固定版本客户端导入/连接仍见 ProdTestList。新增输出测试发现 VMess SR 适配器既有产物没有显式 `tls` 查询参数，其客户端语义待独立核验；本轮未改动该映射，不将其标为 TLS 输出已验证。

---

## 六、R27-05 补充修复 Step（2026-09-03）

- **目标与前置：** 用户确认按研究推荐实施四协议表单布局；高级运行开关集中进“独立开关 / 更多开关”，参数仍在原结构化区域。仅实施 R27-05，不修改 R27-06～R27-09。
- **产出：** `registry.go`、`form_layout_test.go`；`nodeFormLayout.ts`、`NodesView.vue`、`ProtocolFieldEditor.vue` 及对应前端测试。保持字段路径、凭据/修订/扩展契约与 19 协议目录，不新增数据库迁移。
- **参考流程：** `服务端 schema 顺序/group/advanced/when → 当前字段与集中开关展示投影 → 原路径更新顶层对象 → 既有 setField/清空/保存链`；SS 指纹额外声明插件条件和清空归属。折叠只改变可见性，模式按钮沿用应用/放弃语义；集中开关修改使重叠 JSON 草稿失效。
- **TODO / 验收：**
  - [x] 四协议连接顺序完整断言；WS/插件高级子字段与高级开关元数据；SS 指纹显示/投影/插件切换保存及主密码保留回归。
  - [x] 开关只显示一次、祖先条件和高级分类继承、SMux/Brutal 关闭再开、未知键保留、折叠保留 JSON 草稿、模式按钮及错误展开定位测试。
  - [x] 后端 `go test ./... -count=1`、`go build ./...`、`go vet ./...`；前端 `npm test`（41 文件 / 191 用例）、`npm run build`；`git diff --check`。
  - [x] 最新生产前端 + 真实后端本地浏览器：四协议核心布局；VLESS REALITY/WS 顺序、WS Early Data=2048 折叠后保存重开仍保留、SMux 最大连接数=7 关闭再开不恢复、SS shadow-tls→obfs→shadow-tls 指纹清空；桌面/375px 手机与明暗主题核验。
- **环境与边界：** 全新临时 Dev 数据库，合成 Admin 实际为 admin/active；Go overlay 仅将测试监听限制到 `127.0.0.1`，前端预览代理到隔离测试后端，不修改业务认证及正在运行的 Docker 服务。手机开关卡片 44px、Switch 本体 22px、无横向溢出。Production、更完整设备矩阵、目标检查及客户端实际导入/连接保留 ProdTestList 待办，不宣称 R27-06～09 已修复。

---

## 七、R27-09 全量扩展补充修复计划（2026-09-04）

### 7.1 来源、结论与执行规则

本节综合以下研究过程与最终前置核验结论，作为 R27-09 唯一实施口径：

- “研究 R27-09 修复方向”：确认错误不是单行格式问题，而是内部对象、Clash YAML、SR/generic URI 与目标诊断被错误收敛为同一插件字符串；固定 Mihomo 1.19.29 还证明错误的 `plugin: obfs-local;obfs=http` 可能通过 `mihomo -t`，形成静默假阳性。
- “规划 R27-09 全量扩展修复”：用户选择 **C—全量扩展**，除四个已知插件外，同时处理未知自定义插件参数保存、回显、清空、导入、输出和诊断。
- “修复未知插件参数处理”：用户进一步确认未知插件的任意字符串参数均为普通参数，删除原 C 方案中的动态敏感路径、未知参数加密、状态版本升级和迁移设计。
- “核验 R27-09 修复前置检查”：确认当前错误链路仍存在，最终方案没有剩余决策或研究阻断；`target_evidence` 只能在 SS 插件组合内消费，不能全局启用。

执行时必须遵守：

1. Step 7～14 严格串行；每次只执行一个 Step，完成该 Step 的测试与验收后再进入下一步。
2. 先为缺口补失败回归，再修改实现；不得通过降低期望、删除 fixture 或把 `error` 改成 `warn` 使测试变绿。
3. 保持创建、更新、详情、列表、URI 导入、不落库检查、正式装配使用同一规范化与目标语义；不得只修目标检查预览或只修正式下载。
4. 所有输出投影操作克隆数据，不修改 `protocol_json`、历史版本快照或调用方传入的 map。
5. 固定版本离线证据与项目自检是自动化验收；Shadowrocket 真机导入/连接仍是人工待办，不能因 URI 可生成就标记为“完整兼容”。

### 7.2 已锁定范围与明确排除项

#### 纳入本轮

- 四个已知 SS 插件：`obfs`、`v2ray-plugin`、`shadow-tls`、`restls`。
- 未知自定义插件及其 `plugin-opts: map[string]string`。
- BuildReport4 的 N-node-1（未知参数被删除）、SS 插件范围内的 N-node-2（目标证据未进入诊断）与 N-node-5（v2ray-plugin 等 URI 检查误报 `ok`）。
- Clash/Mihomo 结构化输出、Shadowrocket URI、generic/CVR 2.5.2 URI、URI 导入、节点检查、正式装配与前端编辑回显。
- 已知插件中新声明的 `private-key` 等固定敏感路径继续接入 R27-07 的加密、留空保留、重置、脱敏与 `saved_sensitive_paths` 契约。

#### 不纳入本轮

- 不新增 SQL migration，不启动重写历史节点，不升级 `state_format_version`，不新增 `plugin_sensitive_paths`。
- 未知插件参数不加入 `SensitiveFields` 或 `saved_sensitive_paths`，不借用 `extensions_json`，不根据键名猜测敏感性。即使键名为 `password`、`token`、`secret`，也按用户确认的普通字符串参数明文保存、API 回显并进入检查预览。
- 不处理 BuildReport4 N-node-3/N-node-4（VMess/VLESS 的 Shadowrocket TLS/ALPN/指纹映射），不顺带修改非 SS 协议的 `target_evidence`。
- 不处理 Build16、smoke 脚本、安全报告或其他 Issue；发现独立问题只记录，不扩张本 Step。
- 旧版本已经删除的未知 `plugin-opts` 无法恢复；本轮只保证修复后不再丢失，并在文档中明确不可逆历史边界。

### 7.3 最终数据与输出契约

#### 内部存储

| 插件分支 | 唯一有效参数对象 | 保存与回显 | 敏感性 |
|---|---|---|---|
| `obfs` | `obfs-opts` | 结构化对象 | 无新增敏感字段 |
| `v2ray-plugin` | `v2ray-plugin-opts` | 结构化对象 | `private-key` 等由 schema 明确声明的字段按固定敏感路径处理 |
| `shadow-tls` | `shadow-tls-opts` | 结构化对象 | `password`、`private-key` 按固定敏感路径处理 |
| `restls` | `restls-opts` | 结构化对象 | `password` 按固定敏感路径处理 |
| 未知自定义插件 | `plugin-opts` | 只允许字符串键值，明文保存并完整回显 | 全部按普通参数处理 |
| 无插件 | 无参数对象 | 五类插件参数对象全部不活动并清空 | 不适用 |

兼容归一化规则：

- 已知插件同时存在旧 `plugin-opts` 与新独立对象时，以新独立对象为准，旧对象只补充新对象中不存在的键；归一化后删除旧 `plugin-opts`。
- 未知插件保留 `plugin-opts`，不得将其转入已知对象、扩展或凭据模型。
- 无插件或插件切换时，清空 `plugin-opts` 与四个已知对象；A→B→A 不恢复旧参数。
- 归一化必须幂等：连续执行两次的值与一次相同；读取/检查不回写数据库，用户后续成功保存时才自然落成规范形态。

参考伪代码：

```go
func canonicalizeSSPluginParams(params map[string]any) error {
    plugin := stringValue(params["plugin"])
    if def, ok := ssplugin.Lookup(plugin); ok {
        legacy := stringAnyMap(params["plugin-opts"])
        current := stringAnyMap(params[def.StorageKey])
        params[def.StorageKey] = mergeMissing(legacy, current) // current 覆盖 legacy
        delete(params, "plugin-opts")
        clearOtherPluginObjects(params, def.StorageKey)
        return nil
    }
    if plugin == "" {
        clearAllPluginObjects(params)
        return nil
    }
    return validateStringMap(params["plugin-opts"])
}
```

#### 目标矩阵

| 插件 | Clash/Mihomo 1.19.29 | Shadowrocket URI | generic URI（CVR 2.5.2） |
|---|---|---|---|
| 无插件 | 正常输出，无插件字段 | 正常输出 | 正常输出 |
| `obfs` | `plugin: obfs` + 结构化 `plugin-opts`；缺省 mode 只在输出投影补 `http` | 映射 `obfs-local;obfs=...;obfs-host=...`，保留真机未验证 warning | 仅固定解析器可回读的 mode/host 组合通过 |
| `v2ray-plugin` | 独立插件名与结构化对象；缺省 mode 只在输出投影补 `websocket` | 只输出可无损序列化的参数；存在不可表达活动参数时阻断 | 仅 mode/host/path/tls 等 CVR 固定解析器可回读子集通过，其余阻断 |
| `shadow-tls` | 结构化输出；检查 `host` 等固定版本必需字段 | 可无损序列化时输出并给出未验证 warning；否则阻断 | 固定解析器不支持，诊断并跳过 |
| `restls` | 结构化输出；检查 `password`、`host`、`version-hint` | 可无损序列化时输出并给出未验证 warning；否则阻断 | 固定解析器不支持，诊断并跳过 |
| 未知插件 | 原插件名 + 原样字符串 `plugin-opts`；给出未验证 warning | SIP002 可无损序列化时输出并给出未验证 warning | 固定解析器不支持，诊断并跳过 |

Clash 正确产物必须形如：

```yaml
plugin: obfs
plugin-opts:
  mode: http
  host: cdn.example.com
```

以下旧格式必须由项目自检识别为错误，不能因 YAML 可解析或 `mihomo -t` 返回成功而通过：

```yaml
plugin: obfs-local;obfs=http
```

### 7.4 Step 7：SS 插件统一契约、补集条件与字符串映射 schema

- **目标：** 建立唯一 SS 插件定义源，使 schema、归一化、Clash/URI 投影和诊断共用插件名称、内部对象名、目标支持级别、默认值、必需字段与可表达字段；为未知插件参数增加服务端声明的补集条件和字符串 map 类型。
- **前置条件：** 原 Step 1～6 已通过；先提交能复现“未知插件没有参数入口、后端条件只能正向匹配、map 接受非字符串值”的失败测试。
- **产出文件与操作：**
  - 新建 `backend/internal/ssplugin/contract.go` 与测试：定义四个已知插件、`StorageKey`、Clash 默认/必需字段、SR/generic 支持级别与可表达字段；该包不得依赖 `node` 或 `assembly`，避免循环依赖。
  - `backend/internal/node/schema.go`：在 `ConditionRule` 增加 ``PluginNot []string `json:"plugin_not,omitempty"` ``；匹配语义为“当前插件不得位于集合中”，与 network/security/features/targets 维度保持 AND 关系。
  - `backend/internal/node/registry.go`：在 `FieldSchema` 增加 ``MapValueType string `json:"map_value_type,omitempty"` ``；SS schema 增加 `plugin-opts`、`object_kind=map`、`map_value_type=string`、`reset_on=["plugin"]`，仅当插件不属于 `""` 和四个已知插件时活动。
  - `backend/internal/node/project.go`、`node.go`：保存校验与活动投影共同消费 `PluginNot`；`map_value_type=string` 时逐键拒绝 bool、number、array、object，错误路径精确到 `plugin-opts.<key>`。
  - `frontend/src/api/node.ts`、`frontend/src/utils/nodeFormLayout.ts`：同步 `plugin_not`、`map_value_type` 类型与条件匹配；前端不得另写四插件名单决定可见性。
  - `frontend/src/components/ProtocolFieldEditor.vue`：字符串 map 的结构化入口只生成字符串；高级 JSON 出现 bool、number、array 或 object 叶子时立即标记无效并阻止应用。
- **参考伪代码：**

  ```go
  if len(rule.PluginNot) > 0 && contains(rule.PluginNot, statePlugin) {
      return false
  }
  if field.ObjectKind == "map" && field.MapValueType == "string" {
      for key, value := range object {
          if _, ok := value.(string); !ok {
              return fmt.Errorf("字段 %s.%s 类型应为 string", path, key)
          }
      }
  }
  ```

- **TODO：**
  - [x] 先加入 `PluginNot`、字符串 map 类型与未知插件可见性的失败测试。
  - [x] 落地叶子合同包、后端 schema/校验/投影与前端条件类型。
  - [x] 核对真实协议 API 响应并完成本 Step 定向测试。

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/ssplugin ./internal/node -count=1
  cd frontend && npm test -- --run tests/node-form-layout.spec.ts tests/protocol-field-editor.spec.ts
  ```

- **验收标准：** 真实 `/api/admin/nodes/protocols` 响应包含上述元数据；已知/空插件不投影 `plugin-opts`，未知插件才投影；后端与前端对相同 CurrentState 得到一致结果；未知 map 的非字符串叶子被前后端拒绝。
- **实施结果（2026-09-04）：**
  - 新增不依赖 `node` / `assembly` 的 `ssplugin` 叶子包，集中定义四个插件的内部存储键、Clash 默认值/必需字段、三目标支持等级与可表达字段；对外查询返回防御性副本。
  - `ConditionRule.PluginNot` 与 `FieldSchema.MapValueType` 已贯通 Go JSON、真实协议接口、活动条件和前端类型；`plugin-opts` 的已知插件排除集合直接由 `ssplugin.KnownNames()` 生成。
  - 基础字段校验、当前状态校验与创建/更新的归一化前输入校验共同拒绝非字符串叶子，错误路径精确到 `plugin-opts.<key>`，失败创建不写库；现有普通 map 行为不变。
  - 前端补集条件与后端保持 AND 语义；字符串 map 的高级 JSON 会拒绝非字符串叶子，结构化模式不会为该字段生成 bool/number/复杂值控件。
  - 失败先行证据已确认：实现前 Go 因合同/schema 缺失编译失败，前端分别复现补集条件未生效和非字符串 JSON 被接受；实现后全部转绿。
- **验证记录（2026-09-04）：**
  - `cd backend && go test ./internal/ssplugin ./internal/node ./internal/server -count=1`：通过；真实 `/api/admin/nodes/protocols` HTTP 测试断言 `plugin_not`、`map_value_type=string` 与 `reset_on=plugin`。
  - `cd frontend && npm test -- --run tests/node-form-layout.spec.ts tests/protocol-field-editor.spec.ts`：2 文件 / 19 用例通过。
  - `cd backend && go test ./... -count=1 && go build ./... && go vet ./...`：全部通过。
  - `cd frontend && npm test -- --run && npm run build`：41 文件 / 197 用例与生产构建通过；仅有既有 chunk 大小提示。
- **本 Step 边界：** 未知插件字符串参数的真实保存、详情/列表回显与 SS 归一化后全链路投影仍由 Step 8 处理；本 Step 不修改 `normalize.go`，不将该未完成链路宣称为已修复。数据库、状态版本、敏感路径、URI、Clash 输出和目标诊断均未改变。

### 7.5 Step 8：新旧参数幂等归一化、未知参数保存与回显

- **目标：** 修复未知插件参数被静默删除和已知新旧对象优先级错误，贯通创建、更新、详情、列表、重载、检查与 URI 导入前处理，不重写历史数据库。
- **前置条件：** Step 7 已通过；固定内部对象与未知字符串 map 契约不得再变更。
- **产出文件与操作：**
  - `backend/internal/node/normalize.go`、`project.go`：将 SS 插件兼容转换纳入读取/保存/检查共用的幂等规范化；已知插件采用“旧对象补缺、新对象优先”，未知插件保留 `plugin-opts`。
  - `backend/internal/node/node.go`、`check.go`、`uri_import.go`：确认各入口调用同一规范化函数；响应只规范化副本，读取与不落库检查不得写回数据库或递增 `edit_revision`。
  - 插件重置清单必须同时覆盖 `plugin-opts` 和四个独立对象；保留现有 A→B→A 持续失效语义。
  - 不新增 migration，不改 `current_state_json` 结构；`current_state.plugin` 仍只保存当前插件名。
- **参考流程：** `数据库/请求副本 → 共用 SS 规范化 → 已知对象补缺或未知字符串 map 保留 → 类型/活动状态校验 → 保存或只读投影`；规范化函数连续执行两次必须得到同一结果。
- **必须新增回归：**
  - 未知插件携带 `mode`、`host`、空字符串 flag、以及键名为 `password/token/secret` 的创建、数据库值、详情、列表、更新、服务重载。
  - 上述未知字段全部正常回显，且 `saved_sensitive_paths` 不包含任何 `plugin-opts.*`。
  - 已知插件旧 `plugin-opts` 与新对象冲突时新值胜出、旧对象只补缺；执行两次归一化结果相同。
  - 无插件与插件切换清除五类参数；保存失败不写库，读取/检查不改变修订号。
- **TODO：**
  - [x] 先以未知参数丢失和新旧对象覆盖错误建立失败回归。
  - [x] 统一创建/更新/读取/检查/导入的规范化入口并保持只读入口零写入。
  - [x] 完成数据库值、API 回显、修订号和敏感路径断言。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/node ./internal/server -count=1 -run 'SSPlugin|UnknownPlugin|Normalize|SavedSensitive|NodeCheck'
  cd backend && go test -race ./internal/node ./internal/server -count=1
  ```

- **验收标准：** 未知字符串参数不再丢失；已知参数只有一个活动来源；普通未知参数不会进入任何凭据路径；数据库 schema、`state_format_version` 与历史记录均未被后台改写。
- **实施结果（2026-09-04）：**
  - `canonicalizeSSPluginOpts()` 改为复用 Step 7 的 `ssplugin.Lookup()`：四个已知插件将旧 `plugin-opts` 递归补入独立对象，冲突时规范新对象优先；未知插件继续保留原字符串 map。转换连续执行两次结果一致。
  - 字符串 map 的活动投影保留空字符串值，使未知插件的 bare flag 不在检查前被丢弃；普通 map 的既有空值过滤行为不变。
  - 创建、更新、详情、列表、服务重载与检查沿用现有统一规范化入口；`uri_import.go` 已在校验和落库前调用 `NormalizeProtocolJSON()`，无需新增第二套转换。
  - 插件 reset 继续由 schema 与既有服务端清单同时覆盖 `plugin-opts`、`obfs-opts`、`v2ray-plugin-opts`、`shadow-tls-opts`、`restls-opts`；回到原插件或清空插件均不会恢复旧参数。
  - 未新增数据库 migration，未修改 `state_format_version`、`current_state_json`、敏感路径、URI 解析、Clash 输出或目标诊断。
- **回归与验证记录（2026-09-04）：**
  - 失败先行测试分别复现未知 `plugin-opts` 被删除、已知旧对象覆盖新对象；实现后均转绿。
  - 回归覆盖未知插件 `mode`、`host`、空字符串 flag、普通 `password/token/secret` 的数据库值、创建/详情/列表/更新响应、服务重载、检查投影与 `saved_sensitive_paths` 隔离；另覆盖非法更新零写入、历史新旧对象读/查只规范化副本且修订号不变。
  - `cd backend && go test ./internal/ssplugin ./internal/node ./internal/uriparse ./internal/assembly/links ./internal/assembly ./internal/server -count=1`：通过。
  - `cd backend && go test -race ./internal/node ./internal/server -count=1`：通过。
  - `cd backend && go test ./... -count=1 && go build ./... && go vet ./...`：全部通过。
- **本 Step 边界：** 未处理四个已知插件的字段补齐/新增固定敏感路径、SIP002 编解码、Clash 结构化输出、目标诊断或未知插件前端编辑器；这些仍按 Step 9～13 串行实施。Step 14 的前端全量、固定版本、浏览器与文档总收口尚未执行。

### 7.6 Step 9：四个已知插件字段与固定敏感路径修正

- **目标：** 以 Mihomo 1.19.29 固定源码契约修正已知插件 schema，同时保持“展示默认值不批量持久化”和 R27-07 凭据保护。
- **前置条件：** Step 7～8 已通过；字段增删必须先区分“固定版本明确消费”“目标不消费但历史可能已有”“仅建议候选”。
- **产出文件与操作：**
  - `obfs-opts`：`mode` 候选只明确支持 `http/tls`，输出缺省补 `http`；`host` 可选。
  - `v2ray-plugin-opts`：固定 `mode=websocket` 语义，补齐固定版本消费的 `host/path/headers/tls/mux/v2ray-http-upgrade/v2ray-http-upgrade-fast-open/fingerprint/certificate/private-key/name-cert-verify`；移除对当前错误 `version` 字段的“已验证”暗示。
  - `shadow-tls-opts`：补 `host`、`version`、`alpn`、`certificate`、`private-key` 等固定版本字段；`host` 是 Clash 目标必需项，`password` 可选。
  - `restls-opts`：补 `host`；`password/host/version-hint` 是固定 Clash 目标必需项，保留 `restls-script/fingerprint` 等固定版本消费字段；`path` 不再宣称为固定版本已支持。
  - 对旧数据中不再正式声明但已经存在的键，仍按所属已知对象的未知键保留；目标检查给出“固定版本未消费/未验证”诊断，不静默删除。
  - `SensitiveFields` 增加已声明 `private-key` 的完整固定路径；继续覆盖既有主密码、`shadow-tls-opts.password`、`restls-opts.password`。未知 `plugin-opts` 明确不纳入。
  - 目标必需项由 SS 插件目标契约校验，不把 Mihomo 专属必需项误变成所有目标都无法保存的通用必填；不完整草稿可保存，但对应目标检查/正式装配必须阻断。
- **参考流程：** `Mihomo 固定字段证据 → 服务端 FieldSchema/TargetEvidence → 固定敏感路径收集 → R27-07 合并/加密/脱敏 → 目标必需项诊断`；展示默认值仅由输出投影读取，不进入保存 map。
- **TODO：**
  - [ ] 先为字段缺口、错误 `version/path` 证据和新增私钥凭据建立失败测试。
  - [ ] 修正四插件 schema、证据元数据与固定敏感路径。
  - [ ] 验证不完整草稿可保存、目标输出会阻断，以及私钥全生命周期。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/node -count=1 -run 'SS|Sensitive|ProtocolCatalog|FormLayout'
  cd backend && go test -race ./internal/node -count=1
  ```

- **验收标准：** schema 字段、候选、目标证据和敏感路径与固定版本契约一致；默认 mode 只出现在活动输出投影，不回写节点；新私钥保存后加密、API 留空、摘要路径正确、留空更新保留、显式重置清除。

### 7.7 Step 10：SIP002 转义/解析与 SR/generic 目标分流

- **目标：** 用可往返的 SIP002 插件参数编解码替换简单 `strings.Split/Join`，并使 Shadowrocket 与 generic/CVR 2.5.2 分别判断可表达性。
- **前置条件：** Step 7～9 已通过；先固定目标矩阵和每个已知插件可表达字段集合。
- **产出文件与操作：**
  - `backend/internal/ssplugin/sip002.go`：实现无状态 `ParsePluginString` / `SerializePluginString`；插件名与参数中的 `:`、`;`、`=`、`\\` 先按 SIP002 规则反斜杠转义，再交给 URL query 编码。
  - 空值参数作为 bare flag 的内部表示保存为空字符串；序列化排序键名保证确定性，但语义往返不得改变值。
  - 解析器逐字符识别转义，不使用裸 `strings.Split`；拒绝尾部孤立反斜杠、空键、重复键与无法无损表示的输入，不能静默覆盖。
  - `backend/internal/uriparse/uriparse.go`：`parseSSPlugin` 返回 error；已知别名（`obfs-local/simple-obfs`）映射到规范插件和独立对象，未知插件参数写入 `plugin-opts`。
  - `backend/internal/assembly/links/links.go`：删除忽略 target 的 `_ string` 形参；SR 与 generic 分支分别消费合同。已知对象的 bool/list/map 等字段必须显式映射，禁止 `fmt.Sprint` 假装无损。
  - generic/CVR 2.5.2 只允许其固定解析器可回读的 `obfs`、`v2ray-plugin` 子集；`shadow-tls`、`restls`、未知插件或额外不可表达参数由上层诊断阻断，渲染函数自身仍返回错误作为第二道防线。
- **参考流程：** `URI query URL 解码 → SIP002 逐字符反转义 → 重复/坏转义检查 → 已知别名归一化或未知 map 保留`；反向为 `活动参数 → 目标可表达性检查 → SIP002 转义 → URL query 编码`。
- **测试矩阵：**
  - `: ; = \\`、空字符串 flag、Unicode、URL 百分号编码的 serialize→parse 与 parse→serialize 语义往返。
  - 重复键、坏转义、空键、非字符串未知参数反例。
  - 四个已知插件、未知插件在 SR/generic 的成功、warning、阻断分别断言；两种目标不得再共用同一条 `ok` 期望。
- **TODO：**
  - [ ] 先加入特殊字符、flag、重复键和目标分流失败 fixture。
  - [ ] 实现共用 SIP002 编解码器并改造导入、SR、generic 调用链。
  - [ ] 完成语义往返、目标白名单与渲染器第二道防线测试。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/ssplugin ./internal/uriparse ./internal/assembly/links -count=1
  cd backend && go test -race ./internal/uriparse ./internal/assembly/links -count=1
  ```

- **验收标准：** URI 导入未知参数不丢失；SIP002 特殊字符和 flag 可稳定往返；目标不支持或无法无损表达时返回可诊断错误，不输出看似成功但丢参数的 URI。

### 7.8 Step 11：Clash/Mihomo 结构化插件投影与产物自检

- **目标：** 修复 R27-09 直接回归，保证所有 SS 插件在 Clash YAML 中使用独立插件名和结构化参数对象。
- **前置条件：** Step 7～10 已通过；插件合同、已知对象和默认值已稳定。
- **产出文件与操作：**
  - `backend/internal/assembly/render_clash.go`：以 `projectSSPluginForClash`（或等价助手）克隆当前活动插件参数；设置 `out["plugin"]` 为原始插件名、`out["plugin-opts"]` 为结构化 map，再删除 `obfs-opts/v2ray-plugin-opts/shadow-tls-opts/restls-opts` 等内部键。
  - 已知插件只读取当前插件的独立对象；未知插件只读取通用 `plugin-opts`；无插件不输出两字段。
  - `obfs.mode=http`、`v2ray-plugin.mode=websocket` 仅在克隆后的 Clash 输出对象补齐，不改变输入 map、数据库或渲染计划中的节点源数据。
  - `backend/internal/assembly/selfcheck.go`：对 SS 节点增加结构检查：`plugin` 必须是纯插件名且不得含未转义 URI 参数串；有参数时 `plugin-opts` 必须是 mapping；已知插件的必要字段/枚举由统一合同检查。
- **参考流程：** `活动 protocol_json 克隆 → 读取当前插件唯一对象 → 补仅输出默认值 → 写入 plugin/plugin-opts → 删除内部编辑键 → 稳定排序并序列化 → 项目结构自检`。
- **必须新增回归：**
  - 四已知插件和未知插件的 YAML 解码后精确 map 断言，不用字符串 contains 代替结构断言。
  - 断言内部拆分键、`saved_sensitive_paths`、凭据 ID、状态元数据不进入产物。
  - 断言错误旧格式即使可被 YAML 与 Mihomo `-t` 接受，也被 `CheckClashContent` 报错。
  - 输入 map 深比较保持不变；重复渲染产物稳定。
- **TODO：**
  - [ ] 先加入旧错误字符串与四已知/未知结构精确断言。
  - [ ] 实现 Clash 专属结构投影与 SS 插件产物自检。
  - [ ] 验证输入不可变、内部元数据不泄漏和确定性输出。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/assembly -count=1 -run 'ClashProxy|SSPlugin|CheckClashContent|NodeCheckFixtures'
  cd backend && go test -race ./internal/assembly -count=1
  ```

- **验收标准：** 不再出现 `plugin: obfs-local;obfs=http`；正确 `plugin/plugin-opts` 能通过项目结构自检与固定 Mihomo 1.19.29 正例；缺失必需字段和错误 mode 有精确字段路径诊断。

### 7.9 Step 12：SS 插件专属目标诊断与正式装配门槛

- **目标：** 让节点不落库检查与正式 Clash/SR/generic 装配共享同一 SS 插件语义，消除“检查 `ok`、实际插件被忽略/参数丢失”的分叉。
- **前置条件：** Step 7～11 已通过；输出函数能够报告目标不可表达原因。
- **产出文件与操作：**
  - `backend/internal/assembly/node_check.go`：抽出 `diagnoseSSPluginForTarget`，按当前活动插件、当前活动参数和目标返回诊断；未活动对象不产生告警。
  - `backend/internal/assembly/diagnose.go`、`render_clash.go`、`render_sr.go`：复用同一诊断。确定丢失核心连接语义的 error 必须阻断或跳过节点；warning 保留输出并进入装配回执。
  - 推荐诊断码：`ss_plugin_shape_invalid`、`ss_plugin_required_field_missing`、`plugin_option_unexpressible`、`plugin_no_verified_mapping`、`unverified_compatibility`；generic 明确不支持时继续使用 `core_semantic_unexpressible` 以复用现有跳过门槛。
  - Clash 的 SS 插件 error 必须加入正式 Clash 阻断判定，不能因现有 `hasCoreBlockingNodeDiagnostic` 只识别少数 code 而漏过；变更只覆盖 SS 插件错误，不借机改变其他协议等级。
  - `target_evidence` 只消费 SS 插件合同派生的组合证据；不要遍历所有协议/字段直接告警，避免普通 SS cipher、VLESS、VMess、Trojan 被无关降级。
  - 诊断 status 规则保持：存在 error→目标失败；仅 warn/unverified→warning；无诊断→`ok` 且 `diagnostics: []`，不得回归 R27-08 的 null。
- **参考流程：** `当前插件 + 活动参数 + target → SS 插件合同判定 → TargetDiagnostic → 节点检查状态`，正式装配再消费同一诊断执行 `Clash 阻断 / URI 节点跳过 / warning 回执`，不得复制另一套规则。
- **正式装配行为：**
  - Clash：插件形状/必需字段/枚举 error 阻断生成，返回可定位 400；unknown 插件结构可表达时输出但 warning。
  - SR：可无损 SIP002 输出但缺真机证据时 warning；不可表达活动字段时跳过节点。
  - generic：`shadow-tls/restls/unknown` 以及 v2ray-plugin 不可回读参数均跳过；零可输出节点继续触发现有零输出门槛。
- **TODO：**
  - [ ] 先加入“检查误报 ok”及检查/正式装配不一致的失败回归。
  - [ ] 建立单一 SS 目标诊断器并接入三种正式输出门槛。
  - [ ] 核对诊断码、字段路径、warning 回执、零输出和 `diagnostics: []`。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/assembly ./internal/node ./internal/server -count=1 -run 'NodeCheck|SSPlugin|Diagnostics|Render|ZeroOutput'
  cd backend && go test -race ./internal/node ./internal/assembly ./internal/server -count=1
  ```

- **验收标准：** 同一草稿在检查和正式装配中得到相同 severity/code/field_path；所有警告进入回执，所有核心语义错误阻断或跳过；普通无插件 SS 与非 SS 协议状态不因本 Step 改变。

### 7.10 Step 13：未知插件参数前端编辑、校验与分支清空

- **目标：** 让用户通过服务端 schema 编辑未知插件字符串参数，并保证结构化模式、高级 JSON、保存回显、错误定位和插件切换契约一致。
- **前置条件：** Step 7～12 已通过；真实协议 API 已下发 `plugin_not` 与 `map_value_type=string`。
- **产出文件与操作：**
  - `frontend/src/components/ProtocolFieldEditor.vue`：当 map 声明 `map_value_type=string` 时只渲染字符串 Input；不再根据运行时值切换 Boolean/Number/复杂 JSON 控件。
  - 高级 JSON 只接受普通 object 且所有直接值均为 string；非字符串错误精确到参数键，阻止“应用”和保存。结构化重命名继续拒绝空键与重复键。
  - `frontend/src/utils/nodeFormLayout.ts`：`plugin_not` 与后端同语义；未知插件条件由 schema 决定，不硬编码插件名单。
  - `frontend/src/views/admin/NodesView.vue` 与既有 `nodeFeatures.ts`：`reset_on=["plugin"]` 清除通用和四个已知对象、错误状态、未应用 JSON 草稿；A→B→A 不恢复旧值，无关字段不受影响。
  - 未知 `plugin-opts` 不显示“已保存/待替换/已清除”等凭据状态；键名碰巧为 password/token/secret 也保持普通字段行为。
- **参考流程：** `真实 FieldSchema → plugin_not 决定未知参数入口 → 字符串 map 结构化/JSON 编辑 → 前端合法性 → 既有 setField/reset_on → 后端同规则校验 → 保存后真实响应回填`。
- **必须新增回归：**
  - 已知→未知、未知→已知、未知 A→未知 B→未知 A、未知→无插件的可见性和清空。
  - 结构化新增/改名/删除、空字符串 flag、特殊字符、保存后重开回显。
  - 高级 JSON 非字符串、数组、对象、数字、布尔反例；错误自动展开所属区域并禁止保存。
  - 真实 `saved_sensitive_paths` 只影响已知固定敏感字段，不影响未知参数。
- **TODO：**
  - [ ] 先加入未知插件入口、非字符串 JSON 和 A→B→A 失败回归。
  - [ ] 改造字符串 map 控件、条件消费、错误状态与重置链。
  - [ ] 完成保存重开、特殊字符、凭据隔离和响应式布局测试。
- **测试与验收命令：**

  ```bash
  cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/node-form-layout.spec.ts tests/node-features.spec.ts tests/nodes-view.spec.ts
  cd frontend && npm run build
  ```

- **验收标准：** 用户能够完整创建、编辑、保存和重开未知插件字符串参数；前后端对非法类型、条件和清空结果一致；现有四协议布局、开关、JSON 草稿与凭据 UI 无回归。

### 7.11 Step 14：全链路回归、固定版本证据、浏览器与文档收口

- **目标：** 以自动化、固定版本离线证据和本地隔离浏览器验证收口 R27-09，并同步文档中的实际状态与人工边界。
- **前置条件：** Step 7～13 全部通过；若任一目标仍出现无诊断参数丢失，不得进入文档“已修复”。
- **自动化矩阵：**
  - 四已知插件 + 未知插件：创建/更新/数据库/详情/列表/重载、当前状态、插件切换、敏感字段、URI 导入。
  - Clash：精确结构、默认值、不泄漏内部元数据、缺失字段/非法 mode/旧错误字符串反例。
  - SR/generic：SIP002 特殊字符往返、目标支持矩阵、不可表达参数跳过、零输出门槛。
  - 节点检查与正式装配：相同诊断、`diagnostics: []`、预览脱敏。未知参数属于普通参数，测试应明确其不受凭据脱敏保护，避免未来误改回动态敏感模型。
  - 固定 Mihomo 1.19.29：正确四插件正例、缺少必需字段反例；必须保留“旧拼接字符串可能通过内核 `-t`，但项目自检仍拒绝”的回归证据。
- **本地浏览器：** 使用最新生产前端构建与隔离临时 Dev 数据库，走真实 API 验证已知/未知插件切换、参数保存重开、目标检查面板、桌面与 375px 手机视口；不得连接真实 OIDC/SMTP/Xray 或复用真实凭据。
- **参考流程：** `定向失败回归全绿 → 后端全量/竞态/编译/vet → 前端全量/构建 → 固定版本正反例 → 隔离浏览器真实 API → 精确 diff 与文档状态同步`。
- **文档同步：**
  - `Issue13.md`：扩充 R27-09 为最终 C 方案并在真实验收后更新状态；记录 N-node-1、SS 范围 N-node-2/N-node-5 已处理，N-node-3/N-node-4 仍排除。
  - `Design4.md`：写入最终内部存储、未知普通字符串参数、三目标矩阵和人工验证边界。
  - `ProdTestList.md`：保留 Shadowrocket 真机导入/连接，增加四插件与未知插件清单；未执行项不得勾选。
  - `AGENTS.md` 与本文件：更新 Build21 补充 Step 状态、实际文件/命令/结果和版本记录；保留原 Step 1～6 的历史验收数据。
- **TODO：**
  - [ ] 完成全量自动化、竞态、固定版本离线与浏览器真实 API 验证。
  - [ ] 核对工作树、精确 diff、日志/预览凭据与所有人工边界。
  - [ ] 仅按实际证据更新 Design/Issue/ProdTestList/AGENTS/Build 状态。
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

### 7.12 依赖关系与停止条件

```text
Step 7 统一合同/schema
  ├─→ Step 8 归一化与持久化
  └─→ Step 9 已知字段与固定敏感路径
          ↓
Step 10 SIP002 与 URI 分流 ──→ Step 12 共享目标诊断
Step 11 Clash 结构化投影 ────→ Step 12 共享目标诊断
Step 8/9 ────────────────────→ Step 13 前端与真实 API
Step 7～13 全部通过 ─────────→ Step 14 全量收口
```

遇到以下任一情况必须停止当前 Step，记录证据并交由用户决策，不得自行扩大范围：

- 固定 Mihomo 1.19.29 或 CVR 2.5.2 源码与本节字段/目标矩阵出现新冲突。
- Shadowrocket 必须依赖真机行为才能决定 error/warn 或参数映射，且离线规范不能给出结论。
- 实施需要修改数据库 schema、启动迁移、`state_format_version`、未知参数敏感性或 `extensions_json` 边界。
- 修复 SS 插件必须同步改变 VMess/VLESS/Trojan 的目标输出或全局 `target_evidence` 语义。
- 发现用户工作树在本轮精确文件上已有重叠修改，无法安全保留。

---

## 八、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-03 | 初次创建：根据 BuildReport3 完成节点编辑器字段分组、编辑回显、UI 分区、列表控件、SS 插件映射与输出投影修复；前后端构建与测试通过。 |
| v1.1 | 2026-09-03 | 补充 R27-04 修复 Step：表单 schema 投影、编辑草稿 TLS 清理与共用 WS 归一化；接口/服务/输出/前端回归及最新构建浏览器验证通过，记录客户端与 VMess SR 的验证边界。 |
| v1.2 | 2026-09-03 | 完成 R27-05 补充修复 Step，记录字段排序/层次、集中开关、编辑模式按钮、尺寸与错误定位、SS 指纹条件；全量自动化与隔离本地浏览器核心流程通过。 |
| v1.3 | 2026-09-04 | 综合四项 R27-09 研究/规划/敏感性决策/前置核验，追加 C—全量扩展 Step 7～14：四已知插件与未知字符串参数、幂等兼容、固定敏感路径、SIP002、Clash 结构、目标诊断、前端与全量验收；当前仅完成方案写入，尚未实施。 |
| v1.4 | 2026-09-04 | 完成 R27-09 Step 7：新增 SS 插件集中合同、`plugin_not` 补集条件和 `map_value_type=string`，贯通后端投影/保存前校验、真实协议接口及前端条件/JSON 校验；定向与全量测试、编译、vet、生产构建通过，Step 8～14 保持未实施。 |
| v1.5 | 2026-09-04 | 完成 R27-09 Step 8：已知插件旧对象递归补缺且规范新对象优先，未知字符串 `plugin-opts` 贯通保存/回显/重载/检查并保留空 flag；插件重置、敏感路径隔离、只读零写入及后端全量/竞态/编译/vet 验收通过，Step 9～14 保持未实施。 |
