# VPN 订阅管理系统 功能构建计划（Build25：R28-06 未知扩展与局部 JSON 边界，已归档）

> **文档定位：** 本文档是 Issue14 步骤四、R28-06（N-node-6）的**唯一详细构建记录**。承接已归档 [Build22.md](../Build/Build22.md)（D3 已收口）与已归档 [Build24.md](Build24.md)，只处理 R28-06A/B/C，不进入 R28-07、R28-08、R28-09。Step 0～4 已完成，2026-09-10 文档交叉审核发现的前端 `item_id_field` 白名单缺口、保存定位排序和条件隐藏清理证据缺口已修复/补齐，并重新通过全部自动化门禁；**本文件已按归档规则移入 `docs/reports/Build/`**。浏览器、手机和真实客户端人工项仍以 [ProdTestList.md](../../../ProdTestList.md) 为准，未标记为通过。
> - 设计记录：[Design4.md](../../../Design4.md)（R28-06 文档同步目标；与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - 问题追踪：[Issue14.md](../../../Issue14.md)（步骤四 R28-06）
> - 历史来源：[BuildReport4.md](../BuildReport/BuildReport4.md) §5.3 N-node-6、[BuildReport3.md](../BuildReport/BuildReport3.md) §6.4、[Build23.md](Build23.md) §二.7
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 人工核验：[ProdTestList.md](../../../ProdTestList.md)（R28-06 人工项目统一迁入此处）
> - 构建模板：[Build.template.md](../../DocTemplates/Build.template.md)

> **用户已确认决策（2026-09-09）：**
> 1. **R28-06A：** 未知扩展定位为受保护的加密存档块，当前只保存、回显摘要和诊断，不进入 Clash、Shadowrocket 或 generic 客户端产物；`targets` 仅表示期望/关联目标，不代表输出支持。
> 2. **R28-06B：** 局部 JSON 未知键采用显式白名单；固定结构对象默认拒绝未知键，只有业务明确开放的 Map/对象继续允许；Headers 等开放 Map 与未知 SS `plugin-opts` 保持普通参数合同，不按键名猜测敏感性；顶层未知字段继续拒绝且零写入；不自动迁入 extensions。
> 3. **R28-06C：** 父子草稿冲突采用阻止覆盖；发现后代未应用草稿时阻止父对象切换到高级 JSON，展开并定位首个后代草稿，要求用户先应用或放弃，不静默清除，也不自动合并。
> 4. **空 targets 诊断码（2026-09-10 补充确认）：** 空 targets 使用 `unknown_extension_not_targeted`；命中当前 target 时保留 `unknown_extension_not_rendered`。空 targets 表示“未关联目标，因此未参与输出”，命中表示“已关联但当前适配器不渲染”。
> 5. **历史固定对象未知键（2026-09-10 补充确认）：** 读取时保留并在高级 JSON 中显示；检查/保存时定位为需要用户处理的未知字段；用户必须通过对应高级 JSON 显式删除后才能保存；不静默保留为活动输出参数、不自动删除、不自动迁移 extensions。

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|---|---|---|---|
| 0 | 构建前合同检查、对象白名单盘点与 Design4 冲突同步 | Issue14 R28-06；Design4 §6.4/§12 | ✅ 验收通过 |
| 1 | R28-06A 未知扩展存档/诊断/目标校验 | Issue14 R28-06A；Design4 §6.4/§12.3 | ✅ 验收通过 |
| 2 | R28-06B 局部 JSON 显式白名单与存量边界 | Issue14 R28-06B；Design4 §6.4/§12.2 | ✅ 验收通过；前端 `item_id_field` 已放行并由高级 JSON 回归覆盖 |
| 3 | R28-06C 父子 JSON 草稿协调 | Issue14 R28-06C；Design4 §6.4 | ✅ 验收通过；保存定位统一稳定排序，条件隐藏清理与折叠/卸载边界专项回归已补齐 |
| 4 | 前后端联合回归、人工边界迁移和文档收口 | Issue14 步骤四关闭条件 | ✅ 缺口修复后全部定向/全量/build/vet/Docker/Production smoke 门禁通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过 / ⛔ 阻断
>
> **交叉审核闭环（2026-09-10）：** 交叉审核确认的前端固定对象高级 JSON 未放行 `field.item_id_field` 缺口已修复：[ProtocolFieldEditor.vue](../../../frontend/src/components/ProtocolFieldEditor.vue) 新增 `knownFieldNames()`，统一纳入 `field.properties` 与 `field.item_id_field`；WireGuard `peers._credential_id` 可原样应用/保存/检查，输出剥离合同不变。保存定位改为复用 `sortedUnappliedJsonPaths()`，并新增多草稿顺序、条件隐藏清理、折叠/组件卸载保留等专项回归。**R28-06B/C 自动化工程缺口已闭环，Build25 已归档；人工/真机项目仍由 ProdTestList 跟踪。**

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件 | 要点 |
|---|---|---|
| 0 | `Build25.md`、`Design4.md` | 冻结合同、完成 object 白名单盘点、修正 Design4 冲突表述 |
| 1 | `backend/internal/node/{registry,check,node}.go`、`backend/internal/ssplugin/contract.go` 及测试；`frontend/src/api/node.ts`、`frontend/src/views/admin/NodesView.vue` 及测试 | targets 合法集合、空/命中/非命中诊断、加密存档不回显、前端受控多选与文案、三类产物 sentinel 负向边界 |
| 2 | `backend/internal/node/{registry,project,node}.go` 及测试；`frontend/src/components/ProtocolFieldEditor.vue` 及测试 | `obj()` 默认拒绝未知键、开放 Map 显式白名单、schema `allow_unknown` 固定下发、历史未知键读取保留/保存检查阻断/显式删除 |
| 3 | `frontend/src/components/ProtocolFieldEditor.vue`、`frontend/src/views/admin/NodesView.vue` 及测试 | 页面级父/子路径草稿协调、阻断、展开定位、失效清理、折叠保留、保存/检查阻断一致性 |
| 4 | 后端/前端、`Dockerfile`/compose 相关验证、`Build25.md`、`Design4.md`、`Issue14.md`、`AGENTS.md`、`Build23.md`、`ProdTestList.md` | 定向+全量回归、build/vet、Docker build、Production smoke、文档收口与人工项目迁移 |

---

## 三、构建顺序依赖图

```text
Step 0 合同冻结/白名单盘点/Design4 同步
   ↓
Step 1 R28-06A 扩展存档与诊断边界（依赖 Step 0 的目标集合和诊断码）
   ↓
Step 2 R28-06B 局部 JSON 显式白名单（依赖 Step 0 的 object 盘点）
   ↓
Step 3 R28-06C 父子 JSON 草稿协调（依赖 Step 2 的 JSON 校验/错误路径）
   ↓
Step 4 联合回归、人工边界迁移与文档收口
```

> 每个 Step 完成并验收通过后才能进入下一步；不得并行实施多个 Step。

---

## 四、Step 0：构建前合同检查、对象白名单盘点与 Design4 冲突同步

### 4.1 背景与根因

- `extensions_json` 已实现整体加密、摘要回显、替换/清除和按 scope 清空，但没有任何输出适配器解密或消费扩展负载；`targets` 命中时检查固定报告 `unknown_extension_not_rendered`，空 `targets` 没有扩展诊断。
- Design4 §6.4 当前仍写“扩展仅在明确指定的目标中参与输出”，与用户 2026-09-09 确认的“仅加密存档/诊断、不进入任何产物”冲突；Design4 §12.1/§12.3/§12.6 与验收矩阵也需要同步。
- 注册表通用 `obj()` 对所有对象默认设置 `allow_unknown=true`，固定对象内的未知键会留在活动 `protocol_json` 并可能进入后续适配器。
- 子对象存在未应用 JSON 草稿时，父对象仍可切换高级 JSON 并卸载子编辑器；页面级 `unappliedJsonPaths` 缺少组件卸载清理。

### 4.2 目标

1. 在编码前冻结 R28-06A/B/C 的可执行合同；
2. 列出全部当前 object 字段的路径、kind、properties、AllowUnknown、开放 Map、Map 值类型、敏感性、是否输出和 R28-06 后边界；
3. 把 Design4 §6.4、§10.3、§12.1～§12.3、§12.6 的冲突表述改成“加密存档/诊断、不输出”；
4. 不允许在 Step 0 修改 `extensions_json` 存储结构或提前宣称 R28-06 完成。

### 4.3 对象白名单盘点（编码前冻结）

> 说明：当前 `obj()` 对下列所有 object 默认 `AllowUnknown=true`。R28-06 后固定对象改为 `false`，仅开放 Map 显式 `true`；“是否输出”指现有 R28-06 前的活动输出链路。未知扩展负载不属于活动 `protocol_json`，不进入任何客户端产物。

| # | 字段路径 | object_kind | 已声明 properties | 当前 AllowUnknown | 开放 Map | Map 值类型 | 敏感数据 | 当前进入输出 | R28-06 后允许未知键 | 依据 |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | `ss.plugin-opts` | map | 无 | true | 是 | string | 否：用户确认未知插件键值均为普通字符串，即使键名为 password/token/secret 也不按凭据处理 | 是：未知 SS 插件的普通参数输出合同 | **是**（唯一开放业务 Map） | Build21 Step 8/10/13；Issue14 R28-06B 用户决策 |
| 2 | `vless.ws-headers` | map | 无 | true | 是 | 未声明（现有任意值合同） | 未声明敏感路径；普通请求头参数 | 是：归一化为 `ws-opts.headers` 后输出 | **是**（显式开放 Map） | Build19 WS 别名兼容；Design4 §12.5 |
| 3 | `http.headers` | map | 无 | true | 是 | 未声明（现有任意值合同） | 未声明敏感路径；普通请求头参数 | 是 | **是**（显式开放 Map） | HTTP 请求头 Map 既有合同 |
| 4 | `vless.http-opts.headers` | map | 无 | true | 是 | 未声明 | 未声明敏感路径 | 是 | **是**（显式开放 Map） | `httpOpts()` 既有开放请求头 Map |
| 5 | `vmess.http-opts.headers` | map | 无 | true | 是 | 未声明 | 未声明敏感路径 | 是 | **是**（显式开放 Map） | 同上 |
| 6 | `vless.ws-opts.headers` | map | 无 | true | 是 | 未声明 | 未声明敏感路径 | 是 | **是**（显式开放 Map） | `wsOpts()` 既有开放请求头 Map |
| 7 | `vmess.ws-opts.headers` | map | 无 | true | 是 | 未声明 | 未声明敏感路径 | 是 | **是**（显式开放 Map） | 同上 |
| 8 | `trojan.ws-opts.headers` | map | 无 | true | 是 | 未声明 | 未声明敏感路径 | 是 | **是**（显式开放 Map） | 同上 |
| 9 | `ss.v2ray-plugin-opts.headers` | map | 无 | true | 是 | 未声明 | 未声明敏感路径（`private-key` 在父对象固定路径） | 是 | **是**（显式开放 Map） | Build21 Step 9/11 固定合同 |
| 10 | `vless.reality-opts` | fields | `public-key`、`short-id` | true | 否 | — | `public-key` 为公开参数，不是项目敏感路径 | 是 | **否**（固定结构） | REALITY 固定结构 |
| 11 | `vmess.reality-opts` | fields | `public-key`、`short-id` | true | 否 | — | 同上 | 仅活动且目标可表达时 | **否**（固定结构） | VMess REALITY 候选结构 |
| 12 | `trojan.reality-opts` | fields | `public-key`、`short-id` | true | 否 | — | 同上 | 仅活动且目标可表达时 | **否**（固定结构） | Trojan REALITY 候选结构 |
| 13 | `vless.grpc-opts` | fields | `grpc-service-name` | true | 否 | — | 否 | 是 | **否**（固定结构） | gRPC 固定合同 |
| 14 | `vmess.grpc-opts` | fields | `grpc-service-name` | true | 否 | — | 否 | 是 | **否**（固定结构） | 同上 |
| 15 | `trojan.grpc-opts` | fields | `grpc-service-name` | true | 否 | — | 否 | 是 | **否**（固定结构） | 同上 |
| 16 | `vless.ws-opts` | fields | `path`、`headers`、`max-early-data`、`early-data-header-name`、`v2ray-http-upgrade`、`v2ray-http-upgrade-fast-open` | true | 否 | — | 否 | 是 | **否**（固定结构；`headers` 为显式开放子 Map） | WS 固定合同 |
| 17 | `vmess.ws-opts` | fields | 同上 | true | 否 | — | 否 | 是 | **否**（固定结构；`headers` 为开放子 Map） | 同上 |
| 18 | `trojan.ws-opts` | fields | 同上 | true | 否 | — | 否 | 是 | **否**（固定结构；`headers` 为开放子 Map） | 同上 |
| 19 | `vless.http-opts` | fields | `method`、`path`、`headers` | true | 否 | — | 否 | 是 | **否**（固定结构；`headers` 为开放子 Map） | HTTP 固定合同 |
| 20 | `vmess.http-opts` | fields | 同上 | true | 否 | — | 否 | 是 | **否**（固定结构；`headers` 为开放子 Map） | 同上 |
| 21 | `vless.h2-opts` | fields | `path`、`host` | true | 否 | — | 否 | 是 | **否**（固定结构） | H2 固定合同 |
| 22 | `vmess.h2-opts` | fields | `path`、`host` | true | 否 | — | 否 | 是 | **否**（固定结构） | 同上 |
| 23 | `vless.xhttp-opts` | fields | `path`、`host`、`mode` | true | 否 | — | 否 | 是 | **否**（固定结构） | XHTTP 固定合同 |
| 24 | `vless.smux` | fields | `enabled`、`protocol`、`max-connections`、`min-streams`、`max-streams`、`padding`、`statistic`、`only-tcp`、`brutal-opts` | true | 否 | — | 否 | 是 | **否**（固定结构） | R27-03 功能合同 |
| 25 | `vmess.smux` | fields | 同上 | true | 否 | — | 否 | 是 | **否**（固定结构） | R27-03 功能合同 |
| 26 | `ss.smux` | fields | 同上 | true | 否 | — | 否 | 是 | **否**（固定结构） | R27-03 功能合同 |
| 27 | `vless.smux.brutal-opts`、`vmess.smux.brutal-opts`、`ss.smux.brutal-opts` | fields | `enabled`、`up`、`down` | true | 否 | — | 否 | 是 | **否**（固定结构） | R27-03 功能合同 |
| 28 | `ss.obfs-opts` | fields | `mode`、`host` | true | 否 | — | 否 | 是 | **否**（固定结构） | Build21 Step 9 固定合同 |
| 29 | `ss.v2ray-plugin-opts` | fields | `mode`、`host`、`tls`、`path`、`headers`、`ech-opts`、`mux`、`v2ray-http-upgrade`、`v2ray-http-upgrade-fast-open`、`fingerprint`、`certificate`、`private-key`、`skip-cert-verify`、`name-cert-verify` | true | 否 | — | `private-key` 为声明敏感路径 | 是 | **否**（固定结构；`headers` 为开放子 Map） | Build21 Step 9/11/13 |
| 30 | `ss.v2ray-plugin-opts.ech-opts` | fields | `enable`、`config`、`query-server-name` | true | 否 | — | 未声明敏感路径；当前按普通配置处理 | 是 | **否**（固定结构） | Build21 Step 11 |
| 31 | `ss.shadow-tls-opts` | fields | `password`、`host`、`version`、`alpn`、`fingerprint`、`certificate`、`private-key`、`skip-cert-verify`、`name-cert-verify` | true | 否 | — | `password`、`private-key` 为声明敏感路径 | 是 | **否**（固定结构） | Build21 Step 9/11 |
| 32 | `ss.restls-opts` | fields | `password`、`host`、`version-hint`、`restls-script`、`fingerprint`、`skip-cert-verify`、`name-cert-verify` | true | 否 | — | `password` 为声明敏感路径 | 是 | **否**（固定结构） | Build21 Step 9/11 |
| 33 | `trojan.ss-opts` | fields | `enabled`、`method`、`password` | true | 否 | — | `password` 为声明敏感路径 | 是 | **否**（固定结构） | Trojan 内层 SS 固定合同 |
| 34 | `wireguard.peers` | list | `server`、`port`、`public-key`、`pre-shared-key`、`reserved`、`allowed-ips` | true | 否 | — | `pre-shared-key` 为声明敏感路径 | 是 | **否**（固定列表项结构） | R27-07 WireGuard Peer 合同 |
| 35 | `anytls.ech-opts` | fields | `enable`、`config` | true | 否 | — | 未声明敏感路径 | 现有输出链按协议合同处理 | **否**（固定结构） | AnyTLS ECH 固定合同 |

> 结论：R28-06 后只允许 `ss.plugin-opts`、`vless.ws-headers`、`http.headers`、`vless/vmess/trojan.ws-opts.headers`、`vless/vmess.http-opts.headers`、`ss.v2ray-plugin-opts.headers` 保留未知普通键；其余 `fields`/`list` 对象全部拒绝未知键。开放 Map 不放宽值类型校验：`plugin-opts` 继续要求字符串键值，Headers 继续沿用现有值合同。

### 4.4 合同冻结

- 空 `targets`：每个本次请求目标返回 `unknown_extension_not_targeted` warn；
- `targets` 命中当前目标：保留 `unknown_extension_not_rendered` warn；
- `targets` 不包含当前目标：不产生该目标的诊断；
- 合法 target 集合从节点检查权威集合派生：`clash-yaml`、`sr-subs`、`generic-subs`；
- 非法 target：create/update/check 草稿返回明确校验错误，create/update 零写入、update 不递增 `edit_revision`；
- 历史固定对象未知键：读取保留并显示；检查/保存定位并阻断；用户在对应高级 JSON 中删除后允许保存；不自动删除、不自动迁移、不进入输出投影。

### 4.5 产出文件与操作

- `Build25.md`：本文件，创建并维护 Step 0～4 的唯一记录；
- `Design4.md`：修改 §6.4 的未知扩展语义；同步 §10.3 验收矩阵、§12.1/§12.2/§12.3 示例与说明、§12.6 诊断分级、第十三章变更记录；
- 不修改数据库 schema、`extensions_json` 存储结构、已有 API 字段名称或输出适配器。

### 4.6 失败优先检查

```bash
grep -n "扩展仅在明确指定的目标中参与输出" Design4.md
grep -n "AllowUnknown = true" backend/internal/node/registry.go
grep -n "containsString(record.Targets, target)" backend/internal/node/check.go
```

验收时上述旧事实必须被同步修正或记录为后续 Step 处理。

### 4.7 验收命令

```bash
git diff --check
grep -n "扩展仅在明确指定的目标中参与输出" Design4.md || true
grep -n "unknown_extension_not_targeted" Build25.md
grep -n "允许未知键" Build25.md
```

### 4.8 验收标准

1. Build25 包含 Step 0～4 的目标、前置、影响、产出、测试、验收命令、标准和范围外边界；
2. Design4 不再声称指定 target 后扩展参与输出；
3. 35 行对象盘点有代码依据且开放 Map/固定对象边界唯一；
4. 诊断码和历史未知键行为已冻结；
5. `git diff --check` 通过。

### 4.9 范围外边界

- 不实施 R28-07/R28-08/R28-09；
- 不新增未知扩展输出适配器、不自动迁移未知字段到 extensions、不解密回显扩展 payload；
- 不重构节点表单或 UI 改版；
- 不改变 `extensions_json` schema、数据库迁移或公开 API 字段名称。

### 4.10 执行记录

- **2026-09-10 构建前只读预检：** `pwd`、`git branch --show-current`、`git status --short`、`git diff --check`、`git log -5 --oneline` 已执行；分支 `beta`，工作树干净，无既有未提交改动。
- **工具与合同核对：** `str_replace_editor` 可用并已按 AGENTS.md 优先使用；`rg` 在当前环境不可用，改用 `grep -RIn`/`grep -n` 搜索；已阅读 AGENTS.md、Issue14.md、Design4.md §6.3～§6.5/§7.2～§7.4/§10/§12.1～§12.3/§12.6、BuildReport4 §5.3、BuildReport3 §6.4、Build23 §二.7、Build21 相关记录、Build24、ProdTestList、Build 模板。
- **新增确认：** 空 targets 采用 `unknown_extension_not_targeted`；历史固定对象未知键采用“读取保留、检查/保存阻断、显式删除后才可保存”。
- **Design4 同步：** §6.4 已改为“加密存档/诊断、不进入任何产物”，补充 `targets` 关联语义、显式 `allow_unknown` 白名单和父子草稿阻止覆盖；§10.2/§10.3 验收矩阵、§12.1 扩展结构、§12.2 FieldSchema、§12.3 API 语义、§12.6 扩展诊断表已同步；变更记录追加 v1.17。
- **验收命令结果：** `git diff --check` 退出码 0；旧冲突句 `扩展仅在明确指定的目标中参与输出` 已无匹配；Build25 已包含诊断码、白名单和 35 行对象盘点。
- **范围外：** 未实施 R28-07/R28-08/R28-09；未改数据库结构、`extensions_json` schema 或输出适配器；未提前标记 Issue14 R28-06 完成。

---

## 五、Step 1：R28-06A 未知扩展存档/诊断/目标校验

### 5.1 背景与根因

`targets` 未校验、空 targets 无诊断、检查语义未区分“未关联”与“已关联未渲染”；前端 targets 仍是任意逗号字符串且文案没有明确“不进入产物”。

### 5.2 目标

保持 extensions_json 整体加密、摘要 API、keep/replace/clear/add 与 scope reset 不变；补全 target 白名单、空 targets 诊断、前端受控多选和三类产物负向 sentinel 证据。

### 5.3 前置条件

Step 0 验收通过；目标集合和诊断码已冻结。

### 5.4 影响评估

- 后端：`node.CreateManual`、`node.UpdateManual`、`node.Check`、`extensionDiagnostics`、target 校验 helper；
- 前端：`NodesView.vue` 扩展表单、`api/node.ts` 类型与常量；
- 输出：不改适配器，只用 sentinel 测试固定扩展负载/密文不进入 Clash/SR/generic；
- 兼容：已有空 targets 扩展不再被忽略诊断，命中 targets 扩展的 code 不变。

### 5.5 产出文件与参考实现

- `backend/internal/ssplugin/contract.go`：新增稳定 target 列表 helper（如 `TargetNames()`），节点检查与目标校验共用；
- `backend/internal/node/check.go`：`defaultCheckTargets` 从 helper 派生；`extensionDiagnostics` 区分空/命中/非命中；
- `backend/internal/node/node.go`：`validateExtensionTargets()`，在 `prepareExtensionInputs`、`prepareExtensionOps` 的 add/replace 中调用；
- `frontend/src/api/node.ts`：`NODE_CHECK_TARGETS` 常量或等价单一前端常量；
- `frontend/src/views/admin/NodesView.vue`：扩展 targets 改为受控多选、空值说明、文案明确“不进入任何输出产物”；
- 对应 Go/Vitest 测试：`backend/internal/node/r28_06_test.go`、`backend/internal/assembly/r28_06_extension_output_test.go`、`backend/internal/node/r28_06_log_test.go`、`backend/internal/server/r28_06_download_test.go`；前端 `nodes-view.spec.ts`/`node-check-panel.spec.ts`。

### 5.6 参考流程

```text
扩展输入/操作 targets
  → 逐项 trim/去重
  → 只允许节点检查权威目标集合
  → 空数组保留（表示未关联目标）
  → 加密落库/内存检查合并
  → check: 空 → not_targeted warn；命中 → not_rendered warn；非命中 → 不影响
```

### 5.7 失败优先测试

- create 空 targets 成功、合法 targets 成功、非法 target 报错且零写入；
- update add/replace 合法 targets、非法 target 报错且 revision 不变；
- check 新建/编辑草稿空 targets 对每个请求目标返回诊断；
- check 命中 target 保留 `unknown_extension_not_rendered`；
- check 非命中 target 不受影响；
- extensions 密文保存且 API 无 payload/密文；
- reset scope 清除；
- Clash/SR/generic/preview/generate/下载重渲染/诊断/日志 sentinel 负向断言；
- 前端空/合法/非法 targets、文案无输出支持暗示、保存/检查阻断。

### 5.8 验收命令

```bash
cd backend && go test ./internal/node ./internal/assembly ./internal/assembly/links ./internal/server -count=1
cd backend && go build ./...
cd frontend && npm test -- --run tests/nodes-view.spec.ts tests/node-check-panel.spec.ts
cd frontend && npm run build
git diff --check
```

### 5.9 验收标准

- `targets` 空数组允许；仅 `clash-yaml`/`sr-subs`/`generic-subs` 允许；重复去重；非法报错且零写入/revision 不变；
- 空 targets 每个目标有 `unknown_extension_not_targeted`；命中有 `unknown_extension_not_rendered`；不命中不影响；
- 扩展 payload/密文不出现在三类产物、预览、生成、下载重渲染、诊断或日志；
- 前端不再静默提交任意逗号字符串，文案明确“不进入任何输出产物”；
- diagnostics 为数组且不泄漏秘密。

### 5.10 范围外边界

不新增 payload 回显、目标专属 payload schema、输出消费、数据库迁移；不实施 R28-07。

### 5.11 执行记录

- **失败优先测试：** 先新增 `backend/internal/node/r28_06_test.go` 与 `backend/internal/assembly/r28_06_extension_output_test.go`。旧代码下 `TestExtensionTargetsExplicitWhitelist`（非法 create/update/replace）、`TestExtensionDiagnosticsTargetMatrix`（空 targets 无诊断、命中扩展仍 status=ok）、`TestExtensionCheckDraftRejectsIllegalTargets` 均先失败；产物 sentinel 测试属于负向边界守卫，当前代码本来未消费扩展故先通过，不作为“已自动防护”的唯一依据。
- **后端实现：**
  - `backend/internal/ssplugin/contract.go` 新增 `TargetNames()`，授权目标集合从 SS 插件合同包统一派生；
  - `backend/internal/node/check.go` 的 `defaultCheckTargets`、`normalizeCheckTargets` 改用该集合；`extensionDiagnostics()` 区分空 targets（`unknown_extension_not_targeted`）与命中 targets（保留 `unknown_extension_not_rendered`），非命中不产生诊断；扩展 warn 后不再保留虚假 `ok`；
  - `backend/internal/node/node.go` 新增 `validateExtensionTargets()`，在 create 的 `ExtensionInput`、update 的 `add`/`replace` 中校验 targets，空数组允许、trim/去重、非法值在事务前返回错误。
- **前端实现：** `api/node.ts` 新增 `NODE_CHECK_TARGETS`/`NODE_CHECK_TARGET_LABELS`；`NodesView.vue` 扩展 targets 改为受控多选、空值说明、非法值前端阻断；文案改为“当前仅加密保存并参与诊断，不会进入任何输出产物”；payload 占位文案不再暗示输出。
- **实际命令与结果：**
  - `cd backend && go test ./internal/node ./internal/assembly ./internal/assembly/links ./internal/server -count=1`：4 个包全部 `ok`；其中 `TestR28_06DownloadRerenderDoesNotExposeExtensionSentinel`、`TestExtensionPayloadAndCiphertextNotWrittenToLogs` 通过；
  - `cd backend && go build ./...`：通过；
  - `cd frontend && npm test -- --run tests/nodes-view.spec.ts tests/node-check-panel.spec.ts`：2 文件 / 38 用例通过；
  - `cd frontend && npm run build`：通过（仅既有大 chunk 提示）；
  - `git diff --check`：通过。
- **审计补强（2026-09-10）：** 针对核对发现的下载重渲染/日志直接证据缺口，新增：
  - `backend/internal/server/r28_06_download_test.go`：用带 sentinel 扩展的 manual 节点生成 Clash 蓝图，再经 `renderUserSubscription` 走用户下载重渲染路径；断言 render plan 与下载产物均无扩展明文、密文和 label；
  - `backend/internal/node/r28_06_log_test.go`：用 `log.RingBuffer` 捕获节点服务创建、读取、更新、检查、删除日志，断言扩展明文、密文和 label 不进入日志。
  - 补回历史未知子键清理回归：`TestFeatureCloseClearsHistoricalUnknownKeys` 与前端 `关闭父功能或子功能时清除历史未知子键且不修改原对象`。
- **验收对照：** 空 targets、合法 targets、非法 create/update 零写入、replace 失败 revision 不变、空/命中/非命中诊断矩阵、reset scope 清除、API 仅摘要、三类产物 preview/generate、用户下载重渲染、日志和诊断 sentinel 负向断言均已覆盖；未新增 payload 明文/密文回显或输出适配器。

---

## 六、Step 2：R28-06B 局部 JSON 显式白名单与存量边界

### 6.1 背景与根因

`obj()` 全局 `AllowUnknown=true`，固定对象未知键会进入活动 `protocol_json` 和后续输出；前端高级 JSON 不校验固定对象未知键。

### 6.2 目标

固定对象默认拒绝未知键；只有 Step 0 盘点列出的开放 Map 显式允许；历史未知键按已冻结合同读取保留、检查/保存阻断、显式删除后保存；顶层未知字段和 URI 导入继续拒绝。

### 6.3 前置条件

Step 0、Step 1 验收通过；对象白名单冻结。

### 6.4 影响评估

- `obj()` 默认值变更会影响所有协议对象校验、ProjectActive、mergeProtocolJSON、检查/输出投影；
- 开放 Map 必须显式设置 `AllowUnknown=true`，否则 Headers/未知 SS plugin-opts 会回归；
- 历史未知键需要 schema-aware merge 支持显式删除，且不能静默保留到输出；
- 前端高级 JSON 需要递归校验并返回字段路径。

### 6.5 产出文件与参考实现

- `backend/internal/node/registry.go`：`obj()` 不再默认设置 `AllowUnknown=true`；新增开放 Map helper 或对 `plugin-opts`/Headers 显式设置 `AllowUnknown=true`；`FieldSchema.AllowUnknown` JSON tag 固定为 `allow_unknown`（无 `omitempty` 歧义）；
- `backend/internal/node/project.go`：`projectObjectFields` 只在 `field.AllowUnknown` 时保留未知键；固定对象未知键不进入输出投影；
- `backend/internal/node/node.go`：`validateObjectProperties`/`validateActiveObjectFields` 使用同一 `AllowUnknown` 合同；`mergeProtocolJSON` 对固定 `fields` 使用 schema-aware merge，允许显式删除历史未知键，但不自动删除未被用户提交处理的旧对象；
- `frontend/src/api/node.ts`：`allow_unknown` 固定布尔类型；
- `frontend/src/components/ProtocolFieldEditor.vue`：高级 JSON 解析后递归校验固定对象未知键并定位字段路径；开放 Map 仍允许合法普通键。

### 6.6 参考流程

```text
obj() 默认 false
  → 开放 Map helper 显式 true（plugin-opts/ws-headers/headers）
  → 保存/检查：validateObjectProperties 拒绝固定对象未知键
  → ProjectActive：固定对象未知键不进入输出
  → 历史节点：Get 保留未知键；Update/Check 合并后校验并阻断
  → 用户高级 JSON 删除：schema-aware merge 不再复活
  → 前端 parseJSON 递归检查 allow_unknown !== true
```

### 6.7 失败优先测试

- 每类开放 Map 正例：Headers 普通键/任意值、未知 SS plugin-opts 字符串键值；
- 固定对象反例：reality-opts/grpc-opts/ws-opts/http-opts/h2-opts/xhttp-opts/smux/brutal/已知插件 opts/ech-opts/ss-opts/peers 未知键拒绝；
- schema 原始 JSON `allow_unknown` 为 true/false 固定；
- 顶层未知字段仍拒绝且零写入；
- URI 导入逐行失败；
- 历史固定对象未知键读取保留、更新阻断、revision 不变；高级 JSON 显式删除后可保存；
- 保存失败零写入、check 不写库；
- 前端固定对象高级 JSON 错误路径；Headers 新键；应用/放弃 dirty 正确。

### 6.8 验收命令

```bash
cd backend && go test ./internal/node ./internal/server -count=1
cd backend && go build ./...
cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/nodes-view.spec.ts
cd frontend && npm run build
git diff --check
```

### 6.9 验收标准

- 固定对象默认拒绝未知键；开放 Map 白名单与 Step 0 盘点和依据一致；
- `allow_unknown` API JSON 无歧义；
- 未知 SS plugin-opts 完整保留，已知插件未声明键拒绝；
- 历史未知键按冻结合同处理，不静默删除、不自动迁移、不进入输出；
- 顶层未知字段和 URI 导入边界不回归；
- 前端可定位固定对象未知键且不覆盖有效模型。

### 6.10 范围外边界

不将未知键自动迁移到 extensions；不按字段名猜测敏感性；不改变开放 Map 值类型合同；不修改数据库 schema。

### 6.11 执行记录

- **失败证据：** 修改 `obj()` 默认值后，原有“固定对象保留未知键”的测试先暴露回归：`TestObjectSchemaKeepsExtensionsAndSmuxShape`、`TestProjectActiveDropsInactiveBranchesAndPreservesUnknown`、SS 插件私钥生命周期中的 `legacy-field`、WireGuard 内部 `_credential_id` 与前端 smux 历史 `future` 夹具均按旧合同断言，已按 R28-06B 新合同更新测试夹具，并补历史未知键读取/阻断/显式删除回归。
- **后端实现：**
  - `registry.go` 的 `obj()` 不再默认 `AllowUnknown=true`；新增 `openMap()` 显式开放 `plugin-opts`、`headers`、`ws-headers` 等 Map；`FieldSchema.AllowUnknown` JSON tag 去掉 `omitempty`，固定对象固定下发 `false`；
  - `project.go` 固定对象投影只保留声明字段，同时保留 `ItemIDField` 内部身份到脱敏步骤后再剥离；
  - `node.go` 的 `mergeProtocolJSON` 对固定 `fields` 对象执行 schema-aware 合并：旧未知键未在本次显式提交中继续出现时不再复活，新显式未知键保留给校验层精确定位；`validateObjectProperties`/`validateActiveObjectFields` 对 `ItemIDField` 放行。
- **前端实现：** `ProtocolFieldEditor.vue` 高级 JSON 增加递归 `allow_unknown !== true` 固定对象未知键校验与字段路径错误；开放 Map 仍允许普通键、字符串 Map 仍拒绝非字符串值；固定对象未声明参数提示改为“需显式删除后才能保存或检查”。
- **缺口修复补记（2026-09-10）：** 新增 `knownFieldNames()`，将 `field.properties` 与 `field.item_id_field` 统一纳入已知字段集合；`validateFixedObjectProperties()` 复用该集合，WireGuard `peers._credential_id` 可原样通过高级 JSON 应用、保存和检查，其他固定对象未知键仍拒绝。新增组件级多 Peer 重排保留 ID、固定对象其他未知键拒绝，以及 `NodesView` WireGuard 高级 JSON 保存集成回归。
- **实际命令与结果：**
  - `cd backend && go test ./internal/node ./internal/server -count=1`：2 包 `ok`；
  - `cd backend && go build ./...`：通过；
  - `cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/nodes-view.spec.ts`：2 文件 / 64 用例通过；
  - `cd frontend && npm run build`：通过（仅既有大 chunk 提示）；
  - `git diff --check`：通过。
- **补充回归：** 后端新增 `TestObjectAllowUnknownExplicitWhitelistSchema`、`TestOpenMapsAcceptUnknownOrdinaryKeys`、`TestFixedObjectUnknownKeysRejectedAndZeroWrite`、`TestFixedObjectUnknownKeyUpdateAndHistoricalDeleteBoundary`、`TestProjectActiveDropsFixedUnknownButKeepsOpenMap`；顶层未知字段、URI 逐行拒绝、失败零写入、check 不写库、未知 SS plugin-opts 完整保留等现有回归全部通过。
- **边界说明：** 未将历史未知键迁移到 extensions，未按字段名猜测敏感性，未改变开放 Map 值类型合同，未修改数据库 schema。

---

## 七、Step 3：R28-06C 父子 JSON 草稿协调

### 7.1 背景与根因

父子编辑器各自维护 `advanced` 与本地 JSON 草稿，父对象切换会卸载子编辑器；页面级 `unappliedJsonPaths` 缺少路径层次协调和卸载清理，可能覆盖子草稿或留下幽灵阻断。

### 7.2 目标

页面级路径层次协调：子草稿存在时阻止父对象进入高级 JSON，展开并定位首个后代草稿；条件失效/reset 清理状态；折叠不清理；保存/检查阻断无幽灵路径；409 保留草稿。

### 7.3 前置条件

Step 2 验收通过；路径段比较 helper `pathContains` 已有。

### 7.4 影响评估

- `ProtocolFieldEditor` 增加/转发事件时不能改变已有草稿应用/放弃语义；
- `NodesView` 继续作为页面级路径状态唯一协调者；需要维护后代 dirty 路径和定位目标；
- 条件隐藏/组件卸载需要区分业务失效与显示卸载。

### 7.5 产出文件与参考实现

- `frontend/src/components/ProtocolFieldEditor.vue`：新增/扩展事件表达请求进入高级 JSON、当前路径、后代 dirty 路径、阻断后定位路径；
- `frontend/src/views/admin/NodesView.vue`：页面级 `unappliedJsonPaths` 层次协调、阻止父切换、展开祖先、聚焦首个后代草稿、清理失效路径；
- `frontend/tests/protocol-field-editor.spec.ts`、`frontend/tests/nodes-view.spec.ts` 及必要布局测试。

### 7.6 参考流程

```text
子 ProtocolFieldEditor JSON 输入 dirty
  → emit json-dirty-change 到页面级集合
父点击高级 JSON
  → 查找 fieldPath 的严格后代路径（按段落）
  → 若有 → 阻止切换 emit/reveal 首个后代
  → reveal：展开 details、滚动、聚焦可见编辑器
  → 用户应用/放弃后从集合移除，重新可切换
业务条件/reset 清理 → 同步失效 dirty/validity
单纯折叠/视觉隐藏 → 保留
```

### 7.7 失败优先测试

- 两层父/子、三层父子定位；子应用/放弃后父可进入；
- 多个后代稳定顺序、逐个处理、不清其他有效草稿；
- 条件隐藏失效清理；reset scope 清理；折叠保留；
- 页面切换/取消编辑清空页面级集合；409 保留草稿；
- 保存和检查错误定位一致，无幽灵阻断；
- `foo` 与 `foo-bar` 不误匹配。

### 7.8 验收命令

```bash
cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/nodes-view.spec.ts tests/node-features.spec.ts tests/node-form-layout.spec.ts
cd frontend && npm run build
git diff --check
```

### 7.9 验收标准

- 父/子草稿不能并存覆盖同一区域；
- 阻止切换时不自动应用/丢弃，子草稿保持；
- 稳定定位、逐个处理；
- 条件失效/reset 无幽灵阻断；折叠不清草稿；保存/检查/409 无回归。

### 7.10 范围外边界

不自动合并父子草稿；不做全局状态框架；不恢复已失效分支草稿。

### 7.11 执行记录

- **实现方式：** `ProtocolFieldEditor` 新增 `jsonDirtyPaths` prop 与 `advanced-json-blocked` 事件；点击“高级 JSON”时先按字段段落筛选严格后代 dirty 路径，存在则阻止自身 `advanced` 切换并上报 `{ path, blockedBy }`。`NodesView` 继续作为页面级唯一协调者，持有 `unappliedJsonPaths` 集合并按“长度 + 字典序”稳定排序；收到阻断事件后显示明确提示，展开祖先 `details`、滚动并聚焦真实可见的后代编辑器。递归组件只转发事件，不维护页面级副本。
- **保存/检查：** 保存要求 `unappliedJsonPaths` 为空，并复用 `sortedUnappliedJsonPaths()[0]` 定位草稿，与父阻断/检查定位统一为“长度+字典序”稳定排序。目标检查的 `blockedReason` 在控件草稿之前优先报告未应用 JSON 草稿；`NodeCheckPanel` 的“定位草稿”入口与父阻断定位使用同一页面级路径解析。
- **失效清理：** 既有 `clearScopedFields`、`jsonResetVersions` 与 `resetAllEditScopes` 负责分支/reset/取消编辑时清理 dirty/validity；新增 `ProtocolFieldEditor` 模型值快照保护，父级因无关参数变化而替换对象但内容未变时不清理仍有效的 JSON 草稿，不添加组件卸载无条件清理。`foo` 与 `foo-bar` 通过严格字段段落判断避免误父子匹配。
- **失败优先测试：** 新增 `ProtocolFieldEditor` 的两层/三层后代阻断、稳定首个路径、`foo` vs `foo-bar` 不误匹配、开放 Map 与固定对象 JSON 校验；新增 `NodesView` 真实父/子编辑器端到端回归：子草稿阻止父切换、提示后代草稿、展开/聚焦正确编辑器、子草稿应用后父可继续；新增多 JSON 草稿保存稳定排序、条件隐藏清理与保留无关有效草稿、折叠/组件卸载保留边界回归。原“集中开关修改使重叠 JSON 草稿失效”“保存展开定位”“折叠不丢草稿”回归保持。
- **实际命令与结果：**
  - `cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/nodes-view.spec.ts tests/node-features.spec.ts tests/node-form-layout.spec.ts tests/node-check-panel.spec.ts`：5 文件 / 89 用例通过；
  - `cd frontend && npm run build`：通过（仅既有大 chunk 提示）；全量 `npm test -- --run`：42 文件 / 259 用例通过；
  - `git diff --check`：通过。
- **边界说明：** 未自动应用、自动丢弃或自动合并父子草稿；未引入新的全局状态框架；未改变后端节点 API 或数据库结构。

---

## 八、Step 4：前后端联合回归、人工边界迁移和文档收口

### 8.1 目标

执行后端/前端定向和全量测试、build、vet、Docker build、Production smoke；同步 Build25、Design4、Issue14、AGENTS、Build23、ProdTestList；区分自动化证据与人工/真机证据。

### 8.2 前置条件

Step 0～3 全部 ✅ 验收通过。

### 8.3 必须命令

```bash
cd backend && go test ./internal/node ./internal/assembly ./internal/assembly/links ./internal/server -count=1 -timeout 180s
cd backend && go build ./...
cd backend && go vet ./...
cd frontend && npm test -- --run tests/protocol-field-editor.spec.ts tests/nodes-view.spec.ts tests/node-check-panel.spec.ts tests/node-features.spec.ts tests/node-form-layout.spec.ts
cd frontend && npm run build
cd backend && go test ./... -count=1 -timeout 180s
cd frontend && npm test -- --run
docker compose build
bash .smoke-test-prod.sh
git diff --check
```

### 8.4 失败/跳过边界

- 不得引用 Build22/Build24 的历史结果代替本次执行；
- 不得将关键 skip 视为 pass；
- 不得修改 smoke 脚本隐藏产品回归；
- 不得放宽扩展不进入产物的负向断言；
- sentinel 必须同时检查明文和密文不进入产物/API/诊断。

### 8.5 人工项目

浏览器、真实设备、真实客户端项目统一登记到 `ProdTestList.md` 的 `## R28-06 未知扩展与局部 JSON 人工核验`，不混入 Build22/R29-06 条目；未执行不得标记通过。

### 8.6 执行记录

- **后端定向回归（2026-09-10 缺口修复后）：** `go test ./internal/node ./internal/assembly ./internal/assembly/links ./internal/server -count=1 -timeout 180s`：4 包全部 `ok`；`go build ./...` 通过；`go vet ./...` 通过。
- **前端定向回归：** `npm test -- --run tests/protocol-field-editor.spec.ts tests/nodes-view.spec.ts tests/node-check-panel.spec.ts tests/node-features.spec.ts tests/node-form-layout.spec.ts`：5 文件 / 89 用例通过；`npm run build` 通过（仅既有大 chunk 提示）。
- **后端全量回归：** `go test ./... -count=1 -timeout 180s`：全部包 `ok`。
- **前端全量回归：** `npm test -- --run`：42 个测试文件 / 259 个用例全部通过。
- **容器与 Production smoke：** `docker compose build` 成功产出 `vpn-subscription-management-vpn-sub` 镜像；`bash .smoke-test-prod.sh` 完成启动、`SMOKE ALL DONE`、`PROD SMOKE ALL DONE`。
- **工作区检查：** `git diff --check` 通过。
- **文档同步：** `Design4.md` 追加 v1.20 并澄清 `item_id_field` 白名单合同；`Issue14.md` 更新步骤四、R28-06 证据、关闭条件与 v1.22；`AGENTS.md` 将 Build25 移入归档构建；`Build23.md` 更新 N-node-6 交接闭环状态；`ProdTestList.md` 保留 R28-06 人工待核验并追加 v2.15；`Issue15.md` 同步 R29-09/R29-10 自动化闭环状态。未标记任何人工项目为通过。
- **人工边界：** 浏览器、手机、真实客户端项目未执行/未虚标通过，统一登记 ProdTestList；自动化结果只证明合同、存储、API、诊断和生成结果。
- **范围外：** 未进入 R28-07（步骤五），未进入 R28-08/R28-09，未新增未知扩展输出适配器，未自动迁移未知字段，未解密回显 payload，未改数据库 schema。

---

## 九、候选与范围外事项（待决策，不实施）

| # | 候选 | 说明 | 来源 |
|---|---|---|---|
| 1 | 未知扩展目标专属 payload schema 与输出适配 | 未来如要进入客户端产物，必须另立 Design/Build，定义注入位置、冲突策略、脱敏、自检与固定客户端证据 | Issue14 R28-06A |
| 2 | 未知字段自动迁移到 extensions | 明确排除在本轮之外，不得自行实现 | Issue14 R28-06A/B |
| 3 | R28-07～R28-09 | 未授权，不在本轮范围 | Issue14 步骤五～七 |

---

## 十、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-10 | 初次创建 Build25：冻结 R28-06A/B/C 与两个补充确认的细节，完成 object 白名单盘点模板和 Step 0～4 串行计划；Step 0 进行中。 |
| v1.1 | 2026-09-10 | 完成 Step 0：Design4 冲突表述、§10/§12 与 v1.17 变更记录同步；完成 35 行对象白名单盘点；`git diff --check` 通过。 |
| v1.2 | 2026-09-10 | 完成 Step 1 R28-06A：新增扩展 targets 权威集合与校验、空/命中/非命中诊断矩阵、status 不虚假 ok、前端受控多选和产物 sentinel 负向测试；后端 4 包测试、build、前端 2 文件/38 用例、build 与 `git diff --check` 通过。 |
| v1.3 | 2026-09-10 | 完成 Step 2 R28-06B：`obj()` 默认拒绝未知键，Headers/未知 SS plugin-opts 等开放 Map 显式 `allow_unknown=true`，schema JSON 固定下发布尔值；固定对象历史未知键读取保留、检查/保存阻断、高级 JSON 显式删除后保存；前端高级 JSON 递归校验字段路径。后端 node/server 测试、build、前端 2 文件/64 用例、build 与 `git diff --check` 通过。 |
| v1.4 | 2026-09-10 | 完成 Step 3 R28-06C 主体：页面级后代 JSON dirty 路径协调，父对象切换高级 JSON 前阻断并展开/定位真实后代编辑器；多层后代、父阻断稳定排序、保存/检查阻断、父阻断定位入口、reset 与折叠回归通过。保存定位仍使用 Set 插入序且“条件隐藏清理”缺少专项测试，已在交叉审核中登记为证据缺口；前端 5 文件/83 用例、build 与 `git diff --check` 通过。 |
| v1.5 | 2026-09-10 | 完成 Step 4：后端定向 4 包、全量测试、build、vet，前端定向 5 文件/83 用例、全量 42 文件/253 用例、build，Docker Compose build、正式 Production smoke 与 `git diff --check` 全部通过；Build25、Design4、Issue14、AGENTS、Build23、ProdTestList 已同步；人工/真机项目迁移 ProdTestList 并保留未执行状态；R28-06 达到代码与自动化工程闭环，未进入步骤五。 |
| v1.6 | 2026-09-10 | 审计证据补强：新增服务端用户下载重渲染 sentinel 测试、节点服务日志 sentinel 测试，并补回后端/前端历史未知子键关闭清理回归；后端 node/server 定向测试与 build/vet、前端 42 文件/253 用例、build 与 `git diff --check` 重新通过；Build25 Step 1 执行记录、Issue14 R28-06 证据同步更新。 |
| v1.7 | 2026-09-10 | 文档交叉审核修正：确认 R28-06B 前端仍未把 `item_id_field` 纳入高级 JSON 固定对象白名单，WireGuard `peers._credential_id` 会阻断 JSON 应用/保存/检查，故 Step 2 标记为存在缺口、Build25 保持根目录活跃且不归档；同时修正执行/变更记录日期、修正“保存复用稳定排序”和“条件隐藏清理已有专项回归”两处超出实际证据的表述。本次只改文档，未修改任何代码。 |
| v1.8 | 2026-09-10 | 缺口修复闭环：新增 `knownFieldNames()` 放行 `item_id_field`、保存定位统一稳定排序、补条件隐藏清理与折叠/组件卸载边界回归；前端定向 5 文件/89 用例、全量 42 文件/259 用例、build，后端定向 4 包/全量测试/build/vet、Docker Compose build、正式 Production smoke 与 `git diff --check` 全部通过；同步 Design4 v1.20、Issue14 v1.22、AGENTS、Build23、ProdTestList v2.15、Issue15，并将 Build25 移入 `docs/reports/Build/`。人工/真机项目仍未标记通过。 |

---

## 附录：Build25 执行自主决策与阻断记录

| ID | 日期时间 | Step | 类型 | 发现与证据 | 处理决定或阻断点 | 影响范围 | 状态 |
|----|----------|------|------|------------|------------------|----------|------|
| AD-01 | 2026-09-10 | Step 1 | 自主决策 | 前端 targets 需要受控多选，后端 `ssplugin.TargetNames()` 是权威校验源；无现成跨端共享接口 | 在后端使用 `TargetNames()` 作为检查与扩展校验唯一事实源；前端新增 `NODE_CHECK_TARGETS` 仅用于受控选项和展示，非法值仍由后端拒绝；不新增公开 API 字段 | `api/node.ts`、`NodesView.vue`、`check.go`、`ssplugin/contract.go` | 已应用 |
| AD-02 | 2026-09-10 | Step 2 | 自主决策 | 固定对象默认拒绝后，WireGuard `_credential_id` 内部身份会在脱敏前被 ProjectActive 丢弃 | 在 `projectObjectFields` 保留 `ItemIDField` 到脱敏步骤，再由 `StripInternalProtocolMetadata` 在检查/输出前剥离；不把内部 ID 暴露给客户端产物 | `project.go`、`sensitive_paths.go` 既有剥离链 | 已应用 |
| AD-03 | 2026-09-10 | Step 3 | 自主决策/审核修正 | 多个后代 dirty 路径需要稳定首个定位 | 父组件阻断、后代定位和保存定位统一采用“路径长度 + 字典序”确定性排序并补多草稿回归 | `NodesView.vue`、`ProtocolFieldEditor.vue`、前端测试 | 已应用 |
| AD-04 | 2026-09-10 | Step 4 | 范围控制 | Issue14 将扩展 payload 类型/大小摘要列为可读性收口，但非核心合同 | 不新增 API 字段、数据库元数据或解密读取，只用现有安全摘要字段完成 R28-06 | 无 API/schema 变更 | 已应用 |
| AD-05 | 2026-09-10 | Step 3 | 缺陷修复/审核补充 | 父级因无关参数变化而替换整个 `protocol_json` 时，子编辑器的模型值内容未变但引用已变；旧逻辑会无条件清除子级 JSON dirty，导致仍有效的无关草稿失去保存阻断 | 在 `ProtocolFieldEditor` 增加模型值快照保护：`jsonDirty` 为真且内容快照未变时不清理、不 emit 假清除；由 reset scope 继续负责真正失效分支清理 | `ProtocolFieldEditor.vue`、`nodes-view.spec.ts` | 已应用 |
| BL-01 | 2026-09-10 | 全 Step | 阻断记录 | 无 | 未触发停止上报边界；未实施未经确认的输出适配器、迁移、解密回显或 API/schema 变更；人工项按授权迁移 ProdTestList | R28-06 | 无需阻断 |
