# Build10.md — VPN 订阅管理系统 当前构建方案（第二阶段：核心装配与收口）

> **文档定位：** 本文档是 Build9 之后的**第二阶段当前构建方案（v1.4）**，承接 [Build9.md](Build9.md) 已完成的 Step 1~6（R22-03 收口、goccy YAML 迁移、响应头语义、节点协议注册表、规则全集、代理组扩展）。  
> 本卷只规划 Step 1~5（对应原 Build9 Step 7~11）：分层装配覆盖层、节点批量 URI 导入、素材池启动补跑、版本文件原子写入与下载渲染接线、全量收口。  
> 事实基线、研究结论、候选项与假设沿用 Build9 §四~§五；本卷不重复搬运，建议执行前先阅读 Build9 全文与 [AGENTS.md](AGENTS.md)。  
> 本卷只写文档，不构建、不改任何代码与既有文档。

---

## 〇、本卷构建进度追踪

| Step | 内容 | 状态 |
|------|------|------|
| 0 | 创建 Build10 文档与承接说明 | ✅ 已完成（本文档） |
| 1 | 分层装配：Merge + Rules/Proxies/Groups 覆盖层（原 Build9 Step7） | ☐ 未开始 |
| 2 | 节点批量 URI 导入（原 Build9 Step8） | ☐ 未开始 |
| 3 | 素材池同步启动补跑（原 Build9 Step9） | ☐ 未开始 |
| 4 | 版本文件原子写入 + 下载渲染覆盖层接线（原 Build9 Step10） | ☐ 未开始 |
| 5 | 前端/测试/文档/smoke 收口 | ☐ 未开始 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。执行者仍应遵守 AGENTS：每次仅执行一个 Step，验收后再下一步。

---

## 一、构建概要（文件清单总览）

| Step | 对应原 Build9 | 涉及文件 | 要点 |
|------|--------------|---------|------|
| 1 | Step 7 | `backend/internal/assembly/{overlay.go,models.go,service.go,render_clash.go,clash_plan.go,blueprint.go}`、`frontend/src/views/admin/assembly/OverlayStep.vue`（新增）、`AssemblyView.vue` 及测试 | 四类覆盖层、控制面保护、清理悬空、排序、蓝图持久化与下载重渲染 |
| 2 | Step 8 | `backend/internal/uriparse/uriparse.go`（新增）、`backend/internal/node/uri_import.go`（新增）、`backend/internal/server/node.go`、`frontend/src/views/admin/NodesView.vue` 及测试 | URI 批量导入、逐行跳过与去重、事务批量创建 |
| 3 | Step 9 | `backend/internal/cron/pool.go` 及测试（`pool/sync.go` 与 `migrations/1014` 已实施，见 Build9 §4.5） | 启动补跑今日错过任务 |
| 4 | Step 10 | `backend/internal/version/version.go`、`backend/internal/server/render.go`、`backend/internal/download/download.go` 及测试 | 版本文件 temp+rename 原子写、Clash 下载渲染应用覆盖层 |
| 5 | Step 11 | `frontend/src/views/admin/assembly/`、`frontend/tests/`、`.smoke-test.sh`、`.smoke-test-prod.sh`、`Design2.md`、`Design2-UI.md`、`AGENTS.md`、`Issue7.md`、`ProdTestList.md` | UI 集成、专项测试、smoke、文档状态回写 |

---

## 二、构建顺序依赖图

```
Build9 Step 1~6 全部验收通过
  → Build10 Step 1（分层装配覆盖层，核心）
  → Build10 Step 2（节点批量 URI 导入，依赖 Build9 Step4；可与 Step1 观察）
  → Build10 Step 3（素材池启动补跑，独立，可与 Step2 并行）
  → Build10 Step 4（版本原子写 + 下载覆盖层接线，依赖 Step1/Step3）
  → Build10 Step 5（前端/测试/文档/smoke 收口）
```

---

## 三、分步构建计划

### Step 1：分层装配 — Merge + Rules/Proxies/Groups 覆盖层（核心借鉴）

- **目标：** 引入 CVR 的扩展覆盖模型。生成时与下载动态渲染时都按同一管线应用覆盖层；覆盖层随蓝图快照保存，重新编辑可完整恢复。
- **前置条件：** Build9 Step 4/5/6（字段与元数据齐备）。
- **关键设计决策（已在 Build9 §4.4 标注假设 A1/A4/A7）：**
  - 覆盖层输入为**YAML 文本**（与 CVR 编辑形态一致），后端用 `goccy/go-yaml` 的 `MapSlice` + `UseOrderedMap` 解析；
  - 本轮只服务 `clash-yaml`；覆盖层保存在 `selection_json.overlay` 与 `render_plan_json.overlay` 两个位置，避免新增数据库列（旧蓝图缺省即空覆盖层）；
  - 应用顺序（沿袭 Build9 v1.3 定稿）：**完整基础文档组装（蓝图 + 动态 Xray 注入 + 头部/组/规则） → Rules seq → Proxies seq → Groups seq → Merge 深合并 → 控制面恢复 → 悬空清理 → 顶层排序 → 基于最终文档做组可达性收敛与规则降级**。可达性收敛必须发生在覆盖层之后，否则覆盖层 prepend 的节点/组与 Merge 注入的 provider 无法救活依赖组。

- **产出文件与操作：**

  1. `backend/internal/assembly/models.go`：新增输入/快照结构。

  ```go
  type OverlayInput struct {
      MergeYAML    string `json:"merge_yaml,omitempty"`
      RulesYAML    string `json:"rules_yaml,omitempty"`
      ProxiesYAML  string `json:"proxies_yaml,omitempty"`
      GroupsYAML   string `json:"groups_yaml,omitempty"`
  }

  type SeqMap struct {
      Prepend []any    `yaml:"prepend" json:"prepend"`
      Append  []any    `yaml:"append" json:"append"`
      Delete  []string `yaml:"delete" json:"delete"`
  }
  ```

  `GenerateInput` 增加 `Overlay OverlayInput`；`ClashPlan` 增加 `Overlay OverlayInput \`json:"overlay,omitempty"\``。

  2. 新增 `backend/internal/assembly/overlay.go`：核心纯函数。

  ```go
  // parseSeq 把 YAML 文本解析为 SeqMap；空文本返回零值（等价 CVR 空模板）。
  func parseSeq(raw string) (*SeqMap, error) {
      raw = strings.TrimSpace(raw)
      if raw == "" { return &SeqMap{}, nil }
      var m SeqMap
      if err := gyaml.UnmarshalWithOptions([]byte(raw), &m, gyaml.UseOrderedMap()); err != nil {
          return nil, fmt.Errorf("覆盖层 YAML 解析失败: %w", err)
      }
      return &m, nil
  }

  // applySeq 实现 CVR use_seq 语义：prepend + (original - delete) + append。
  func applySeq(root *gyaml.MapSlice, field string, seq *SeqMap) error { /* 见下 */ }

  // deepMerge 实现 CVR use_merge 语义：MapSlice 递归合并，其余以 patch 覆盖。
  // 调用前必须先对 patch 顶层 key 小写化（lowercaseTopLevelKeys），与 CVR 一致。
  func deepMerge(base, patch *gyaml.MapSlice) { /* 见下 */ }

  // snapshotControlPlane / enforceControlPlane 对应 CVR AuthoritativeFields：
  // 保存 CONTROL_PLANE_KEYS 的现值，merge 后恢复，缺失键删除。
  // 【用户已确认】完整采纳 CVR 控制面保护（v1.3）。
  var controlPlaneKeys = []string{
      "external-controller", "external-controller-cors",
      // 平台通道键按部署目标门控纳入（本项目生成客户端订阅，通常不存在，
      // 但快照/恢复逻辑保留，防止 Merge 注入后被写入下载内容）：
      "external-controller-unix", "external-controller-pipe",
      "secret", "mixed-port", "socks-port", "port",
      "redir-port", "tproxy-port",
      "tun", "mode", "allow-lan", "log-level", "ipv6", "unified-delay",
  }

  // dns.ipv6 按 CVR AuthoritativeFields 单独快照/恢复：
  // snapshot 记录基础文档 dns.ipv6（若存在）；enforce 时仅当最终文档仍有 dns 映射时恢复。

  // cleanupProxyGroups：与 CVR cleanup_proxy_groups 同口径——
  // 合法名 = proxies 名 ∪ proxy-groups 名 ∪ proxy-providers 名 ∪ 内置策略；
  // 清理 use 中不存在的 provider、proxies 中不存在的节点/组/provider；
  // 【核验提醒】需保留 CVR 的 has_valid_provider 语义：组存在合法 use provider 时，
  // 其 proxies 中未命中合法名的字符串成员仍应保留（可能来自 provider）。
  // 本项目额外保留 COMPATIBLE 作为合法内置策略。
  func cleanupProxyGroups(root *gyaml.MapSlice) { /* 见下 */ }

  // sortTopLevel：对齐 CVR use_sort——控制面键 → 其他键 → proxies/proxy-providers/
  // proxy-groups/rule-providers/rules 固定收尾。
  func sortTopLevel(root *gyaml.MapSlice) { /* 见下 */ }

  // applyClashOverlay 单入口：
  func applyClashOverlay(root *gyaml.MapSlice, ov OverlayInput) error {
      rules, err := parseSeq(ov.RulesYAML); if err != nil { return err }
      proxies, err := parseSeq(ov.ProxiesYAML); if err != nil { return err }
      groups, err := parseSeq(ov.GroupsYAML); if err != nil { return err }
      if err := applySeq(root, "rules", rules); err != nil { return err }
      if err := applySeq(root, "proxies", proxies); err != nil { return err }
      if err := applySeq(root, "proxy-groups", groups); err != nil { return err }

      control := snapshotControlPlane(root)
      dnsIPv6 := snapshotDNSIPv6(root)
      if strings.TrimSpace(ov.MergeYAML) != "" {
          var mergeRoot gyaml.MapSlice
          if err := gyaml.UnmarshalWithOptions([]byte(ov.MergeYAML), &mergeRoot, gyaml.UseOrderedMap()); err != nil {
              return fmt.Errorf("Merge YAML 解析失败: %w", err)
          }
          lowercaseTopLevelKeys(&mergeRoot) // 【源码事实】CVR use_merge 先小写化 merge 顶层 key
          deepMerge(root, &mergeRoot)
      }
      enforceControlPlane(root, control)
      enforceDNSIPv6(root, dnsIPv6)
      cleanupProxyGroups(root)
      sortTopLevel(root)
      return nil
  }
  ```

  `applySeq` 的参考实现要点：

  ```go
  func applySeq(root *gyaml.MapSlice, field string, seq *SeqMap) error {
      if seq == nil { return nil }
      seqItems := mapSeqList(root, field) // 不存在时返回空 []any

      deleteSet := map[string]bool{}
      for _, d := range seq.Delete { deleteSet[d] = true }

      kept := make([]any, 0, len(seqItems))
      for _, item := range seqItems {
          name := goccyNameOf(item)
          if name != "" && deleteSet[name] { continue }
          kept = append(kept, item)
      }
      out := make([]any, 0, len(seq.Prepend)+len(kept)+len(seq.Append))
      out = append(out, seq.Prepend...)
      out = append(out, kept...)
      out = append(out, seq.Append...)
      mapSetList(root, field, out)

      // 【源码事实】CVR use_seq：proxies 场景收集新增节点名，插入第一个 selector/select 组最前，
      // 且源码不要求该组 proxies 非空；同时从所有组删除 delete 命中的节点。
      if field == "proxies" {
          added := seqNames(seq.Prepend, seq.Append)
          if len(added) > 0 || len(deleteSet) > 0 { applyProxyGroupSideEffects(root, added, deleteSet) }
      }
      return nil
  }
  ```

  3. `backend/internal/assembly/render_clash.go`（生成/预览路径）：基础 `gyaml.MapSlice` 构建完成后调用 `applyClashOverlay(doc, in.Overlay)`，再通过 `marshalClashYAML` 输出；`plan.Overlay = in.Overlay` 写入 render_plan_json。
  4. `backend/internal/assembly/clash_plan.go`（下载重渲染路径）：`RenderClashPlan` 解出 `plan.Overlay`，**重构为以下顺序**：
     - 先组装完整基础文档（计划头部 + 计划 manual proxies + 动态 Xray 节点 + 计划全量 proxy-groups + 计划 rules/fallback，**暂不做可达性删组**）；
     - 调用 `applyClashOverlay(doc, plan.Overlay)`（内部完成 seq → merge → 控制面恢复 → 清理 → 排序）；
     - 解析最终文档的 proxies / proxy-groups / proxy-providers，执行组可达性收敛（强制组保留、普通组按最终引用与 provider `use` 判定）；
     - 基于最终保留组执行规则目标降级与 fallback 改写；
     - 重新清理/排序后经 `marshalClashYAML` 输出。历史蓝图无 `overlay` 时是零值，行为不变。
  5. `backend/internal/assembly/service.go` / `blueprint.go`：`SaveBlueprintTx` 把 `in.Overlay` 写入 selection_json 的 `"overlay"` 键；`GetBlueprint` 解析并返回 `overlay`；`loadEditIfAny` 前端恢复四个 YAML 文本。
  6. 新增 `frontend/src/views/admin/assembly/OverlayStep.vue`：仅 Clash YAML 装配器显示；四个 `Input.TextArea`（Merge/Rules/Proxies/Groups），提供 CVR 同款空模板填充按钮与字段说明。按项目简单轻量化原则先不引入 Monaco，文本域 + 预览 diff 即可覆盖首轮需求；预览前把四个文本并入 `buildInput().overlay`。
  7. `frontend/src/api/assembly.ts`：`GenerateInput.overlay`、`BlueprintResponse.selection.overlay` 类型同步。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/assembly ./internal/server
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - `TestOverlaySeqSemantics`：rules/proxies/groups 的 prepend/delete/append 顺序与 CVR `use_seq` 一致；delete 会从所有组移除节点；新增节点自动进入第一个 selector 组最前；
  - `TestOverlayMergeAndControlPlane`：嵌套 Mapping 深合并、非 Mapping 覆盖；merge 顶层 key 大小写归一；merge 不能覆盖完整 `CONTROL_PLANE_KEYS` 与 `dns.ipv6` 快照值；
  - `TestOverlayCleanupAndSort`：不存在的节点/组/provider 引用被清理；顶层键序符合 use_sort；
  - `TestBlueprintOverlayRoundtrip`：生成 → GetBlueprint 返回四个 YAML 文本 → 重新编辑加载一致；
  - `TestDownloadRendersOverlay`：激活蓝图的下载重渲染结果包含 prepend 节点/规则与 merge 字段，且动态 Xray 节点仍正确注入；**覆盖层 prepend 的节点或 Merge 注入的 provider 能救活依赖它们的普通组，不被提前删除**；
  - 无覆盖层的既有蓝图渲染结果与当前行为一致。


### Step 2：节点批量 URI 导入（借鉴 ProxiesEditor 粘贴解析与去重）

- **目标：** 为 manual 节点提供多行 URI / Base64 文本批量导入；解析失败跳过、按 name 去重、给出逐行回执，对齐 CVR `proxies-editor-viewer.tsx` 的交互模型。
- **前置条件：** Build9 Step 4（registry 字段完成）。
- **产出文件与操作：**

  1. 新增 `backend/internal/uriparse/uriparse.go`（独立小包，避免 `node ↔ assembly/links` 循环依赖）。

  ```go
  package uriparse

  type Result struct {
      Protocol string
      Name     string
      Host     string
      Port     int
      Params   map[string]any
  }

  // Parse 支持：ss / vmess（V2rayN JSON 与 Shadowrocket base64）/ vless（标准与
  // SR base64 userinfo）/ trojan / anytls / hysteria2 / hysteria / tuic / wireguard /
  // http(s) / socks5。解析规则与 CVR src/utils/uri-parser/* 保持一致。【源码事实】
  // 注意：CVR 也支持 ssr，但本项目按 Design2 §4.5 排除 ssr，归入候选 C2；此处不实现。
  func Parse(raw string) (*Result, error) {
      s := strings.TrimSpace(raw)
      if s == "" { return nil, errors.New("空行") }
      // 若整体是 Base64 文本块（多行节点订阅形态），先尝试标准解码并按行展开；
      // 单行 Base64 由各 scheme 自己处理，避免误判普通 URI。
      scheme := strings.ToLower(strings.SplitN(s, "://", 2)[0])
      switch scheme {
      case "ss":     return parseSS(s)
      case "vmess":  return parseVMess(s)
      case "vless":  return parseVLESS(s)
      case "trojan": return parseTrojan(s)
      case "anytls": return parseAnyTLS(s)
      case "hysteria2", "hy2": return parseHysteria2(s)
      case "hysteria", "hy": return parseHysteria(s)
      case "tuic": return parseTUIC(s)
      case "wireguard", "wg": return parseWireGuard(s)
      case "http", "https": return parseHTTP(s)
      case "socks5", "socks": return parseSocks5(s)
      default: return nil, fmt.Errorf("暂不支持导入的 scheme: %s", scheme)
      }
  }

  // ParseBlock 处理多行输入：支持整块 Base64 解码；返回逐行结果与 skip 原因。
  func ParseBlock(text string) ([]Result, []Skip) { /* 50 行一批在调用方控制，不必在此并发 */ }
  ```

  VLESS Shadowrocket 形态的参考实现（对照 CVR `uri-parser/vless.ts`）：

  ```go
  func parseVLESS(s string) (*Result, error) {
      rest := strings.TrimPrefix(s, "vless://")
      // 先按标准 URI 解析；失败时尝试 SR 形态：base64(:uuid@host:port) + query
      if parsed, err := parseURLLike(rest); err == nil {
          return normalizeVLESS(parsed, false), nil
      }
      if idx := strings.Index(rest, "?"); idx > 0 {
          decoded := decodeBase64OrOriginal(rest[:idx])
          if parsed, err := parseURLLike(decoded + rest[idx:]); err == nil {
              return normalizeVLESS(parsed, true), nil
          }
      }
      return nil, errors.New("invalid vless uri")
  }
  // normalizeVLESS：uuid 在 SR 形态下去掉 "cipher:" 前缀；query 键归一化
  // security/tls→tls，pbk/sid→public-key/short-id，type/headerType→network，
  // host/path→ws-opts 或对应 transport opts（与 CVR 回读字段一致）。
  ```

  2. 新增 `backend/internal/node/uri_import.go`：批量导入业务逻辑。

  ```go
  type ImportLineResult struct {
      Line    int    `json:"line"`
      Raw     string `json:"raw"`
      OK      bool   `json:"ok"`
      Name    string `json:"name,omitempty"`
      Reason  string `json:"reason,omitempty"`
  }

  // ImportURIs：解析 → 全局 name 去重（保留第一条）→ 逐条加密敏感字段 →
  // 单事务内批量 INSERT；任何一行违反唯一索引/跨命名空间校验只跳过该行，
  // 不中断整批（与 CVR parseUri 失败跳过、按 name 去重同口径）。
  // 【用户已确认】与已有节点同名时同样跳过并回执，不覆盖既有节点。
  func (s *Service) ImportURIs(ctx context.Context, text string) ([]ImportLineResult, error) { /* ... */ }
  ```

  3. `backend/internal/server/node.go`：新增 `POST /api/admin/nodes/import`（会话 + 管理员中间件），请求体 `{ "text": "..." }`，响应 `{ list: ImportLineResult[], total: n }`。
  4. `frontend/src/api/node.ts`：新增 `importNodes(text)`。
  5. `frontend/src/views/admin/NodesView.vue`：PageHeader 增加「批量导入」按钮 → Modal 内 textarea（placeholder 说明支持 ss/vmess/vless/trojan/hy2/tuic/wireguard/http/socks5）→ 提交后展示成功/跳过行表格；成功后刷新列表。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/uriparse ./internal/node ./internal/server
  cd frontend && npm run build && npm test -- --run
  ```

- **验收标准：**
  - 标准 VLESS、SR base64 VLESS、V2rayN VMess、SR VMess、SS SIP002、Trojan、Hysteria2 URI 均能解析为 Build9 Step 4 的 protocol_json；
  - 与 CVR `uri-parser` 对同一批 URI 的解析结果字段一致（关键字段逐项比对）；
  - 重复 name 只导入第一条，第二条进入 skipped 回执；与已有节点同名时跳过并回执，不覆盖；snell/mieru 等无 URI 协议返回“暂不支持”；
  - 批量导入中的非法行不阻断其余合法行；前端回执可读。

---

### Step 3：素材池同步启动补跑（R22-01 主体已实施，核验后修订）

- **目标：** R22-01 的“短事务分批 + keep 表索引 + 联合索引”已在 `a0cd819` 落地；本步只剩服务启动时补跑“今日应跑但停机错过”的每日同步。
- **前置条件：** 无（可与 Step 2 并行）；当前 `pool/sync.go` 已通过相关测试。
- **现状（核验确认，不再重复实现）：**
  - `backend/internal/pool/sync.go`：插入阶段每批 500 行独立短事务；全部成功才建临时 keep 表；删除前统计与差量删除分批；终态独立回写；
  - `backend/migrations/1014_pool_sync_optimize.sql`：联合索引已存在；
  - 失败时保留已插入条目、跳过删除并落 `failed` 的语义已实现。
- **本步产出文件与操作：**

  1. `backend/internal/cron/pool.go`：在 `StartPoolAutoSync` 首轮中，除“当前分钟匹配”外，增加“今日错过”查询：

  ```go
  // 【源码事实】CVR Timer.init 会补跑 overdue 订阅；本项目按 A6 只补跑“今日错过”的池。
  rows, err := st.DB().QueryContext(ctx, `
      SELECT p.id FROM rule_pools p
      WHERE p.auto_sync = 1
        AND p.sync_time <= ?
        AND COALESCE(date(p.last_synced_at), '') < date('now')
        AND NOT EXISTS (
            SELECT 1 FROM pool_sync_tasks t
            WHERE t.pool_id = p.id AND t.status = 'running'
        )`, now.UTC().Format("15:04"))
  ```

  2. 对结果逐池 `SubmitSync`，已有 `ErrSyncRunning` 跳过即可。
  3. 补充防重复：若当前分钟命中与补跑查询命中同一池，应使用同一 `poolAutoSyncState` 去重或合并为一次提交，避免同分钟重复同步。
  4. 补测试：
     - `last_synced_at` 为昨天、当前时间晚于 `sync_time` 的池被提交一次；
     - 当前分钟命中的池不与补跑重复提交；
     - 运行中任务不重复提交。

- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/pool ./internal/cron
  ```

- **验收标准：**
  - 启动首轮补跑：昨日已同步、今日已到点的池被提交一次；
  - 当前分钟命中与补跑去重，同一池不会被重复提交；
  - 运行中任务会被跳过；
  - 既有短事务/索引相关测试保持全绿。

---

### Step 4：版本文件原子写入 + 下载渲染覆盖层接线

- **目标：** 借鉴 CVR `help::save_yaml` 的 temp + rename 原子替换，降低版本文件半写风险；并把 Step 1 的覆盖层接到用户下载重渲染路径。
- **前置条件：** Step 1；Step 3 完成前本步可与 Step 3 并行开发（覆盖层部分不依赖 Step 3）。
- **产出文件与操作：**

  1. `backend/internal/version/version.go`：新增 `writeFileAtomic`，`CreateVersion` 写版本文件时使用；失败清理路径相应调整。

  ```go
  // 【源码事实】CVR save_yaml：唯一临时文件（O_EXCL）→ write/flush → 保留权限 → rename 原子替换。
  func writeFileAtomic(full string, data []byte, perm fs.FileMode) (tmpPath string, err error) {
      dir := filepath.Dir(full)
      tmp := filepath.Join(dir, fmt.Sprintf(".tmp-%d-%s", time.Now().UnixNano(), randomSuffix()))
      f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
      if err != nil { return "", err }
      defer func() { if err != nil { _ = os.Remove(tmp) } }()
      if _, err = f.Write(data); err != nil { _ = f.Close(); return "", err }
      if err = f.Sync(); err != nil { _ = f.Close(); return "", err }
      if err = f.Close(); err != nil { return "", err }
      if err = os.Rename(tmp, full); err != nil { return "", err }
      return "", nil
  }
  ```

  `CreateVersion` 内：`writeFileAtomic(full, content, 0o644)`；DB 插入或 AfterCreate 失败时删除 `full`（事务回滚语义与现逻辑一致）；`evictOldest` 删除文件逻辑不变。

  2. `backend/internal/server/render.go`：`renderUserSubscription` 现有查询已读取 `COALESCE(b.render_plan_json, '{}')`，无需改 SQL；Step 1 已把 overlay 写入 `render_plan_json`，`RenderClashPlan` 内部按 Step 1 新顺序（先应用覆盖层、后可达性收敛）处理。`RenderClashPlan` 已通过 `marshalClashYAML` 输出真实 UTF-8，不再需要 `restoreYAMLUnicodeEscapes`；输出后若 `CheckClashContent` 出现 error，只 `slog.Warn` 不阻断下载（系统生成版本已在 Build9 Step 2 阻断过，这里仅防历史脏蓝图）。
  3. `backend/internal/download/download.go`：确认直接上传内容（无 blueprint）仍原样返回，不经过覆盖层/自检。
- **测试与验收命令：**

  ```bash
  cd backend && go test ./internal/version ./internal/assembly ./internal/server ./internal/download
  ```

- **验收标准：**
  - 写入过程中人为注入失败不会留下 `.tmp-*` 垃圾文件；并发创建版本不产生半文件；
  - 含覆盖层的 Clash 蓝图下载内容 = 预览内容（动态节点注入后同管线输出）；直接上传内容原样返回；
  - 下载渲染输出包含真实 emoji，无 `\U` 转义；
  - 历史无 overlay 蓝图下载行为与 Build8 一致。

---

### Step 5：前端 / 测试 / 文档 / smoke 收口

- **目标：** 把 Build9 Step 1~6 与 Build10 Step 1~4 的 UI 与接口改动做整体回归，补齐专项测试与 smoke，并按文档体系同步。
- **前置条件：** Build9 Step 1~6 与 Build10 Step 1~4 全部验收通过。
- **产出文件与操作：**

  1. `frontend/src/views/admin/assembly/`：确保四类装配器在 Build9 Step 1 目标区改造后，Steps 计数、`AssemblerShell` 的 prev/next 索引、路由 `platform_id/rule_id` 预填、重新编辑（含 Step 1 overlay 恢复）全链路正确；Clash 装配器增加 Overlay 步骤。
  2. `frontend/tests/*.spec.ts`：新增/扩展用例：
     - 版本列表 `id>0` 且“重新编辑”请求真实版本 ID；
     - 目标选择区与副 Tab 联动、SR-conf 规则名直建；
     - 高级模式关闭时 Xray 板块隐藏；
     - 节点选择弹窗先勾选后排序且无移除按钮；
     - Overlay 四个 YAML 文本随 Preview/Generate 提交、重新编辑回填；
     - 平台 Clash 生态预设头提交与错误提示；
     - 节点批量导入 Modal 成功/跳过回执。
  3. `backend` 单测：Build9 Step 1~6 与 Build10 Step 1~4 各包新增用例全部保留；运行 `go test ./...`。
  4. `.smoke-test.sh` / `.smoke-test-prod.sh`：增加 Build9/Build10 冒烟路径：
     - 创建 yaml 平台 + 手动节点 + 组 + 覆盖层（prepend 规则/节点）→ Preview 含覆盖层 → Generate → 激活 → 下载响应含 `filename*`/`profile-update-interval`/真实 emoji；
     - URI 批量导入两条合法 + 一条非法，回执为 2 ok / 1 skip；
     - 重新编辑从版本管理进入并回填 overlay。
  5. 文档同步（本步执行时进行，本文档创建阶段不改）：
     - `Design2.md`：§3.3 代理组 5 类型与字段、§3.5 规则全集、§4.1 自检、新增“覆盖层”小节；
     - `Design2-UI.md`：§5 装配流程（目标区上移、Overlay 步骤、节点弹窗、批量导入）、§7 代理组高级字段；
     - `AGENTS.md`：文档清单增加 Build9/Build10（当前两阶段构建方案）；
     - `Issue7.md`：R22-01/02/03/06/07/08 按实际状态回写；R22-03 标注由 Build9 Step 1 实施；R22-04/05 已在 `a0cd819` 闭环，不再列入开放项；
     - `ProdTestList.md`：增加 Emoji 输出、RFC5987 文件名、覆盖层下载、大规则源同步。
- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm test -- --run
  bash .smoke-test.sh
  bash .smoke-test-prod.sh
  ```

- **验收标准：** 后端/前端全绿；smoke 全绿；文档状态与代码一致；Issue7 相关条目状态闭环。

---

## 四、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-27 | 由 Build9 v1.3 拆分生成：承接原 Step 7~11，重编号为本卷 Step 1~5；仅调整文档结构、交叉引用与构建顺序，未改动任何代码。 |
| v1.1 | 2026-08-27 | 拆分后的文档收口：修正本卷内 Step 编号引用、前置条件明确指向 Build9 Step 1~6、变更记录独立为本卷记录。未改动任何代码。 |

