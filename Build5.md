# Build5.md — 基础模式装配与分发：manual 节点、代理组、四类装配器（当前构建方案·第五轮）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第五轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则），承接 [Build4.md](./Build4.md)（第四轮：基础模式地基与规则素材池，须全部验收通过后本轮方可启动）。
> - 设计基线：[Design2.md](./Design2.md)（与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design2-UI.md](./Design2-UI.md)（活跃，承载 Design2 全部界面部件）
> - 编码指令：[AGENTS.md](./AGENTS.md)（**唯一强要求**）
> - 前置轮次：[Build4.md](./Build4.md)；后续轮次：[Build6.md](./Build6.md)（高级模式 Xray 后端）、[Build7.md](./Build7.md)（高级模式管理面与交付收口）
>
> **里程碑：本 Build 全部 Step 完成后，基础模式（第二~四章）能力完整可用：管理员可维护 manual 节点与代理组，通过订阅装配页以四类装配器（Clash YAML / SR 节点订阅 / 通用节点订阅 / SR 分流规则）生成带快照的版本；产物入池后按「入池 + 显式激活」分发；用户端与管理员端展示新订阅地址池模型；渲染性能满足 1 万规则 <500ms 级。**
>
> **范围红线：** 本 Build 实现 manual 节点全流程；**xray 来源节点只做数据层读取与占位兼容，不实现 Xray 实例录入/检测/注入**（Build6）。本 Build 不实现高级模式 UI（用户组高级列、Xray 实例页等），但节点/代理组管理页为基础模式能力，必须实现。

---

## 执行约束（执行 AI 必须严格遵守）

1. **严格按 Step 顺序执行**，完成一个 Step 并验收通过后，方可进入下一个 Step；**禁止跳步、并行、合并步骤、跨 Build 提前实现后续功能**。
2. **每个 Step 完成后必须运行该 Step 的「验证命令」**，全部通过才算完成；任一失败修复后重验。
3. **遇到模糊、歧义或设计未覆盖的细节，必须停止并向用户提问**；禁止自行假设。链接编码细节先查 [Node-Link-Standards.md](./docs/Reference/Node-Link-Standards.md)，Clash 结构先查 [Clash.yaml.template.md](./docs/DocTemplates/Clash.yaml.template.md) 与 [ClashOfficial.yaml.template.md](./docs/DocTemplates/ClashOfficial.yaml.template.md)，SR 结构先查 [Shadowrocket.subs.template.md](./docs/DocTemplates/Shadowrocket.subs.template.md) / [Shadowrocket.conf.template.md](./docs/DocTemplates/Shadowrocket.conf.template.md)。
4. **依赖白名单**：本 Build 新增 Go 依赖仅允许 `gopkg.in/yaml.v3`（YAML 渲染，依赖图中已存在）；前端新增 `diff`（jsdiff）。禁止引入其它库（如 monaco、拖拽库）。
5. **关键设计参数必须严格按下表取值**，与 Design2.md 保持一致，禁止修改：

| 参数 | 取值 | 出处 |
|------|------|------|
| manual 协议范围 | ss / vmess / vless / trojan / hysteria / hysteria2 / tuic / wireguard / http / socks5 / snell / anytls / mieru / masque / openvpn / ssh / shadowquic / trusttunnel / tailscale；**ssr 除外** | Design2 §3.2/§4.5 |
| 节点名称规则 | `nodes.name` 创建后不可修改（manual=管理员名，xray=`{实例slug}-{入站tag}` 系统名）；`display_name` 仅 source=xray 可编辑，空=回退 name；两者均禁止控制字符、逗号、首尾空白，允许中文/emoji；**有效渲染名（display_name 非空则用之，否则 name）全局唯一，且不得与 proxy_groups.name、强制组名或 Clash/mihomo 内建保留代理名（DIRECT / REJECT / REJECT-DROP / PASS / COMPATIBLE）重复** | Design2 §3.2 |
| 节点凭据加密 | AES-256-GCM（复用签名密钥派生机制）；密文字段编辑回显空值 = 保留原凭据 | Design2 §3.2 |
| 代理组名称规则 | name 创建后不可改；禁止控制字符、逗号、首尾空白，允许中文/emoji；**创建时不得与任一节点有效渲染名、强制组名或 Clash/mihomo 内建保留代理名重复，冲突 409** | Design2 §3.3 |
| 代理组类型 | `select` / `url-test` / `fallback`（三枚举）；名称创建后不可改，**组类型创建后允许修改**；名称字符集同节点 | Design2 §3.3 |
| 强制组 | 直接连接（DIRECT）/ 国外流量 / 无法归属的流量（MATCH 兜底目标）；系统内置渲染结构，**不入 proxy_groups 表** | Design2 §3.3 |
| 代理组内容约束 | 至少含节点 / 「直接连接」组 / 「国外流量」组三者之一；子组引用 DAG，禁止环形 | Design2 §3.3 |
| 规则类型 | DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / IP-CIDR / IP-CIDR6 / PROCESS-NAME / PROCESS-NAME-REGEX / USER-AGENT（Clash 渲染跳过 USER-AGENT 并提示） | Design2 §3.5 |
| IP 规则 | IP-CIDR / IP-CIDR6 一律附加 `no-resolve` | Design2 §3.5 |
| Clash 兜底 | `GEOIP,CN,直接连接` + `MATCH,无法归属的流量`（固定） | Design2 §3.6 |
| SR 兜底 | `GEOIP,CN,DIRECT` 固定；`FINAL` 方向表单二选一，默认 `FINAL,PROXY` | Design2 §3.6 |
| 空产物硬校验 | Clash「国外流量」至少 1 个节点否则拒绝；SR subs/generic-subs 至少 1 个且转换后有效链接 ≥1，否则拒绝；规则为空允许生成 | Design2 §4.1 |
| 生成与激活 | 生成/上传一律 `activate=false`（首版 current_version=0 自动激活除外）；后续版本由「激活/分发」显式切换 | Design2 §4.4 |
| 快照 | `assembly_blueprints.version_id` 1:1；Clash 另存 render_plan_json；直接上传版本无 blueprint | Design2 §4.4/§5.9 |
| 默认文件名 | clash-yaml→`.yaml`、sr-subs/generic-subs→`.txt`、sr-conf→`.conf`；直接上传保留原始扩展名 | Design2 §5.7 |
| 渲染性能 | 1 万条规则 Clash/SR conf 渲染 <500ms 级（Build 期 benchmark 验收） | Design2 §5.7 |
| 占位标记 | 仅当装配勾选了 xray 来源节点时输出 `# {{xray_nodes}}`；未勾选不输出 | Design2 §4.3 |

6. **注释使用中文**；所有 error 必须处理；构造注入；接入层/业务层/数据层分层；下载端点禁缓存；日志 token 脱敏。
7. **本 Build 不得调用 Xray gRPC、不得读写 users.uuid_encrypted/proxy_secret_encrypted、不得实现流量配额**。xray 节点渲染在无凭据时应输出占位注释（Build6 注入前行为），但本 Build 基础模式下装配勾选不到 xray 节点（无实例来源），仅保证代码路径与测试存在。

---

## TODOLIST CheckList（构建进度核对）

> 执行 AI 必须逐个完成并勾选。状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

- ☐ Step 1：协议注册表与 manual 节点后端
- ☐ Step 2：代理组后端（preset/custom、DAG、内容约束）
- ☐ Step 3：装配器渲染内核（四语法 + 快照 + 校验 + 链接编码）
- ☐ Step 4：装配 HTTP 端点与版本/蓝图集成
- ☐ Step 5：节点管理页与代理组管理页前端
- ☐ Step 6：订阅装配页前端（四装配器 + DiffView + 重新编辑）
- ☐ Step 7：分发 UI 收口、渲染 benchmark 与 Build5 端到端验收

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 协议注册表与 manual 节点后端 | Design2 §3.2/§5.10，UI §6/§9.1 | ☐ 未开始 |
| 2 | 代理组后端 | Design2 §3.3，UI §7/§9.1 | ☐ 未开始 |
| 3 | 装配器渲染内核 | Design2 §3/§4，Reference 文档 | ☐ 未开始 |
| 4 | 装配 HTTP 端点与版本集成 | Design2 §4.1/§4.4/§5.10，UI §9.1 | ☐ 未开始 |
| 5 | 节点/代理组前端 | Design2-UI §6/§7 | ☐ 未开始 |
| 6 | 装配页前端 | Design2-UI §5 | ☐ 未开始 |
| 7 | 分发 UI 收口与验收 | Design2 §4.4/§4.5，UI §3~4 | ☐ 未开始 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件（核心） | 要点 |
|------|----------------|------|
| 1 | `backend/internal/node/`、`backend/internal/server/node.go`、`backend/internal/server/server.go`、测试 | 协议注册表、manual 节点 CRUD、凭据加密/回显保留、xray 行 display-name 命名与只读占位 |
| 2 | `backend/internal/proxygroup/`、`backend/internal/server/proxy_group.go`、测试 | preset/custom、定义 JSON、DAG 校验、预设启用开关 |
| 3 | `backend/internal/assembly/`（models.go/render_clash.go/render_sr.go/validate.go/registry 联动）、测试 | 四语法渲染、链接编码、快照模型、跳过项提示 |
| 4 | `backend/internal/server/assembly.go`、`backend/internal/version/version.go` 小改、测试 | context/preview/generate/blueprint 端点、版本列表 blueprint 标记 |
| 5 | `frontend/src/api/{node,proxyGroup}.ts`、`frontend/src/views/admin/{NodesView,ProxyGroupsView}.vue`、router | 动态表单、只读约束、排序、DAG 前端提示 |
| 6 | `frontend/src/api/assembly.ts`、`frontend/src/components/DiffView.vue`、`frontend/src/views/admin/AssemblyView.vue` + `assembly/` 子组件、`package.json`（diff） | 五页签四装配器、六步/单页、预览 diff、重新编辑 |
| 7 | `frontend/src/views/admin/{SubscriptionsView,VersionManageView,RulesView}.vue`、`frontend/src/api/{version,subscription,rule}.ts`、`backend` benchmark 测试 | 入池未生效引导、装配版本标签、重新编辑入口、验收 |

---

## 三、构建顺序依赖图

```
Step 1（节点）──┐
                ├──▶ Step 3（渲染内核依赖节点/代理组数据模型）
Step 2（代理组）┘        │
Step 3 ──▶ Step 4（HTTP 端点与蓝图落库）
Step 4 ──▶ Step 6（装配前端，依赖端点契约）
Step 1+2 ──▶ Step 5（节点/代理组前端，可与 Step 6 共享子组件）
Step 4+5+6 ──▶ Step 7（分发 UI 与端到端验收）
```

> 线性执行序：Step 1 → 2 → 3 → 4 → 5 → 6 → 7。

---

## 四、分步构建计划


---

### Step 1：协议注册表与 manual 节点后端

**本 Step 完成后，`/api/admin/nodes` CRUD 与 `/api/admin/nodes/protocols` 可用：manual 节点按注册表动态校验，敏感字段 AES-256-GCM 加密存储、编辑留空保留；xray 节点行除 display_name 外只读（enabled/is_public 可切、display_name 可命名，Build6 接通推送副作用）。**

- **目标：** 新建 `internal/node` 业务包与接入层，实现统一节点表的 manual 来源能力与协议可扩展注册表。
- **前置条件：** Build4 全部验收通过。
- **产出文件与操作：**

  1. **`backend/internal/node/registry.go`**：静态协议注册表（应用层维护，无 DB schema 枚举）。
     - 每个协议定义：`Protocol`、`Label`、`FormSchema []FieldSchema`（字段名/类型/是否必填/默认值/说明）、`SensitiveFields []string`（uuid/password/private-key/psk 等凭据字段）、`SRLink bool`、`GenericLink bool`。
     - 协议清单严格为约束表列出的 19 个协议（ssr 除外）。无法转链接协议（snell/mieru/masque/openvpn/ssh/shadowquic/trusttunnel/tailscale）`SRLink=false && GenericLink=false`。
     - `GET /api/admin/nodes/protocols` 返回 `{list:[{protocol, label, form_schema, link_mappings, sensitive_fields}]}`（前端动态渲染表单；`link_mappings` 描述该协议的 SR/标准链接映射能力与参数名，按 Design2-UI §9.1 契约）。
  2. **`backend/internal/node/node.go`**：服务与 CRUD。
     - `Node` 结构对应表字段（含 `DisplayName *string`），`ProtocolJSON map[string]any`；`RenderName()` 返回有效渲染名（DisplayName 非空则用之，否则 Name）。
     - `CreateManual`：名称校验（禁止控制字符/逗号/首尾空白，允许中文 emoji；`name != strings.TrimSpace(name)`、`strings.Contains(name, ",")` 或含 `<0x20/0x7F` 拒绝）；`nodes.name` 全局唯一（409）；**跨命名空间校验：有效渲染名不得与任一节点有效渲染名、proxy_groups.name、强制组名「直接连接 / 国外流量 / 无法归属的流量」或 Clash/mihomo 内建保留代理名「DIRECT / REJECT / REJECT-DROP / PASS / COMPATIBLE」重复，冲突 409**；host/port 校验；protocol 在注册表；按注册表 field schema 校验 protocol_json；敏感字段值加密。
     - 敏感字段存储格式统一 `"enc:v1:" + base64.RawURLEncoding(...)`，加解密复用 `config.Encrypt/Decrypt`（签名密钥从 config 读取；测试用固定密钥）。`decryptProtocolJSON` 在渲染/读取时恢复明文；**列表接口不返回凭据明文**（敏感字段返回空串/`***`，按 UI 脱敏口径）。
     - `UpdateManual`：名称只读（请求带 name 且与库不一致 → 400）；编辑回显凭据字段空值 = 保留原密文；其余字段按新值替换；协议变更允许但必须整体校验（Build 期决策：协议变更等价重新填表，不保留不兼容旧字段）。
     - `SetDisplayName(ctx, id, displayName)`：**仅 source=xray**（manual 400）；空串 → 写 NULL（清空回退 name）；非空走名称字符集校验 + **有效渲染名唯一（排除自身，表达式唯一索引兜底）** + **跨命名空间校验（不得与 proxy_groups.name、强制组名或 Clash/mihomo 内建保留代理名重复）**；冲突 409；本 Build 仅落库，不触发任何 Xray 推送/候选集重算。
     - `SetEnabled`/`SetPublic`：`source=xray` 行才允许 is_public；`is_public=1` 仅 `allocatable=1 AND missing=0`；非法切换 400。本 Build 仅落库，副作用钩子留接口 `onXrayChanged func(ctx, node, oldEnabled, oldPublic)`（Build6 注入）。
     - `Delete`：`source=xray` 且 `missing!=1` 拒绝（400，文案「请先删除 Xray 入站并刷新节点检测」）；manual 可直接删除。
     - `List`：JOIN xray_instances 取实例 slug；支持 `?source=manual|xray` 筛选；返回 `display_name` 与 `render_name`（后端计算或前端按规则计算，契约见 UI §9.1）。
  3. **`backend/internal/server/node.go`**：路由全部 `/api/admin/nodes`（session+admin）：
     - `GET /api/admin/nodes`、`POST`、`PUT /:id`、`DELETE /:id`、`PUT /:id/toggle`、`PUT /:id/display-name`、`GET /api/admin/nodes/protocols`。`POST/PUT /:id` 仅 manual（xray 行返回 400「节点信息由实例检测维护」；xray 的 enabled/is_public 走 `/toggle`，display_name 走 `/display-name`）。
     - 错误映射：409 名称/显示名冲突 / 400 校验与非法切换 / 404 不存在。
  4. **`backend/internal/server/server.go`**：构造 `nodeSvc := node.NewService(st, cfg, lg)` 并注册。
  5. **单测**：名称校验（中文/emoji 通过，逗号/控制字符/首尾空白拒绝）、重名 409、**display_name 清空/设置/仅 xray 可改、有效渲染名唯一（含 name 与 display_name 交叉冲突）、与代理组名/强制组名/Clash-mihomo 内建保留代理名冲突 409**、凭据加密后库内不以明文出现、留空保留、xray 行只读与非法 is_public、协议注册表完整性（19 协议、ssr 缺失、每协议敏感字段非空且合法）。

- **参考代码/伪代码：**

  **名称校验**

  ```go
  func validateNodeName(name string) error {
      if name == "" || len([]rune(name)) > 128 { return errors.New("名称不能为空且不超过 128 字符") }
      if name != strings.TrimSpace(name) { return errors.New("名称禁止首尾空白") }
      if strings.Contains(name, ",") { return errors.New("名称禁止逗号") }
      for _, r := range name {
          if r < 0x20 || r == 0x7f { return errors.New("名称禁止控制字符") }
      }
      return nil
  }

  // 有效渲染名：display_name 非空则用之，否则回退稳定引用名 nodes.name
  func renderName(n Node) string {
      if n.DisplayName != nil && *n.DisplayName != "" { return *n.DisplayName }
      return n.Name
  }

  // 跨命名空间校验：有效渲染名不得与 proxy_groups.name、强制组名或 Clash/mihomo 内建保留代理名重复
  func checkRenderNameNamespace(ctx context.Context, tx *sql.Tx, name string) error {
      switch name {
      case "直接连接", "国外流量", "无法归属的流量",
          "DIRECT", "REJECT", "REJECT-DROP", "PASS", "COMPATIBLE":
          return errors.New("节点名称不得与代理组/强制组/内建保留代理名重复")
      }
      var id int64
      if err := tx.QueryRowContext(ctx, `SELECT id FROM proxy_groups WHERE name = ? LIMIT 1`, name).Scan(&id); err == nil {
          return errors.New("节点名称不得与代理组名重复")
      } else if !errors.Is(err, sql.ErrNoRows) {
          return err
      }
      return nil
  }
  ```

  **凭据加解密包装**

  ```go
  const encPrefix = "enc:v1:"
  func (s *Service) encryptSecret(ctx context.Context, plain string) (string, error) {
      key, err := s.cfg.Get(ctx, config.KeySigningKey)
      if err != nil || key == "" { return "", errors.New("签名密钥未配置，无法加密节点凭据") }
      b, err := config.Encrypt([]byte(plain), []byte(key))
      if err != nil { return "", err }
      return encPrefix + b, nil
  }
  func (s *Service) decryptSecret(ctx context.Context, v string) (string, error) {
      if !strings.HasPrefix(v, encPrefix) { return "", fmt.Errorf("非法密文格式") }
      key, err := s.cfg.Get(ctx, config.KeySigningKey)
      if err != nil || key == "" { return "", errors.New("签名密钥未配置") }
      b, err := config.Decrypt(strings.TrimPrefix(v, encPrefix), []byte(key))
      return string(b), err
  }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/node/... ./...
  ```

- **验收标准：** 全部测试通过；`go test ./internal/node` 覆盖上表约束；手工 `go run` 后登录管理员调用 `GET /api/admin/nodes/protocols` 返回 19 协议且 ssr 不存在。



---

### Step 2：代理组后端（preset/custom、DAG、内容约束）

**本 Step 完成后，`/api/admin/proxy-groups` CRUD 可用：预设组种子可启用/停用与编辑成员；自建组支持创建/编辑/删除；保存时后端校验 DAG 与组内容约束。**

- **目标：** 实现代理组全局定义服务。
- **前置条件：** Step 1 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/proxygroup/proxygroup.go`**：
     - `Group { ID, Name, Type, PresetKey, Enabled, Definition }`；`Definition { GroupType string; Nodes []string; Groups []string }`（有序）。
     - 名称校验复用 node 的同一字符集规则（提取 `internal/nodename` 或导出 `node.ValidateName`；禁止复制两份不同实现）；**跨命名空间校验使用 node 包导出的 `CheckNameNamespaceTx`：组名不得与任一节点有效渲染名、强制组名「直接连接 / 国外流量 / 无法归属的流量」或 Clash/mihomo 内建保留代理名「DIRECT / REJECT / REJECT-DROP / PASS / COMPATIBLE」重复，冲突 409**。
     - `CreateCustom(name, groupType string, def Definition)`：type=custom；name 唯一 + 跨命名空间校验；groupType ∈ select/url-test/fallback；校验定义。
     - `Update(id, groupType, def)`：preset 与 custom 均可编辑成员；**name/preset_key 不可改，groupType 允许修改（三枚举校验）**；preset 的 `enabled` 通过 `SetPresetEnabled` 单独切换。
     - `Delete`：preset 不可删；custom 删除（历史装配快照悬空容错，不做反向约束）。
     - `List`/`Get`；`SetPresetEnabled`。
     - 校验函数：
       - 节点引用必须存在于 `nodes` 表（不含 `source` 限制；xray 行也允许引用）；子组引用允许 `直接连接`、`国外流量`（强制组常量）或 `proxy_groups` 中其他组。
       - **DAG 校验**：建图 `groupName -> []subGroups`，检测自环与环（DFS 三色）；节点名是叶子。
       - **内容约束**：节点数组非空，或子组数组包含「直接连接」/「国外流量」；否则 400「代理组至少需直接包含一个节点、直接连接组或国外流量组」。
  2. **`backend/internal/server/proxy_group.go`**：`/api/admin/proxy-groups` CRUD + `PUT /:id/preset-toggle`。错误 400/404/409。
  3. **`backend/internal/server/server.go`** 注册。
  4. **单测**：DAG 环（A→B→A）、自环、引用不存在节点/组、强制组引用合法、内容为空拒绝、**只有自定义子组拒绝、只有节点/直接连接通过**、组名与节点有效渲染名/强制组名/内建保留名冲突 409、**组类型修改成功与非法类型 400**、预设不可删/名不可改、**预设种子（groups:["直接连接"]）可加载/可编辑/可渲染**、种子启用默认 1。

- **参考代码/伪代码：**

  **Definition JSON 结构（库中存字符串）**

  ```json
  { "type": "select", "nodes": ["节点A", "节点B"], "groups": ["直接连接"] }
  ```

  **DAG 校验核心**

  ```go
  func validateDAG(groups []proxygroupRow) error {
      const (white, gray, black = 0, 1, 2)
      color := map[string]int{}
      // 先建邻接表；对每个 group 做 DFS，命中 gray 即环；强制组「直接连接/国外流量」无定义视为叶子
  }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/proxygroup/... ./...
  ```

- **验收标准：** 全部测试通过；预设种子 9 组可列出且 enabled=1；单测覆盖全部校验错误路径。



---

### Step 3：装配器渲染内核（四语法 + 快照 + 校验 + 链接编码）

**本 Step 完成后，`internal/assembly` 可对给定输入渲染四种产物并返回跳过项与 Clash 渲染计划；所有校验（空产物、组内容、链接可转性）在渲染层闭环；渲染逻辑不直接写库。**

- **目标：** 实现 Design2 第三章的纯函数式渲染内核。
- **前置条件：** Step 2 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/assembly/models.go`**：定义
     - `TargetSyntax` 常量 `clash-yaml/sr-subs/generic-subs/sr-conf`。
     - `PoolSelection { PoolID int64; Target string }`（有序数组）、`RuleLine { RuleType, MatchValue, Target string }`、`GenerateInput { TargetSyntax; PlatformID; RuleID; FixedParams map[string]any; NodeNames []string; GroupNames []string; OverseasMembers []string; Pools []PoolSelection; CustomRules []RuleLine; FinalDirection string }`。
     - `RenderResult { Content []byte; Skipped []SkipItem; RenderPlan json.RawMessage }`；`SkipItem { Kind, Name, Reason string }`。
  2. **`backend/internal/assembly/load.go`**：上下文加载（只读）——按名称（`nodes.name` 稳定键）读节点（含解密后的 protocol_json 与 display_name）、代理组定义、素材池全部条目（按 sort_order）、平台/规则目标校验；节点对象提供 `renderName()`（display_name 非空则用之，否则 name）。
  3. **`backend/internal/assembly/render_clash.go`**：
     - 头部按 `FixedParams` 输出（用 `gopkg.in/yaml.v3` 序列化 `map[string]any`，保证键序按输入顺序——yaml.v3 的 MapSlice 或自定义 `yaml.MapSlice`；**必须保留管理员填写顺序**）。
     - `proxies:`：manual 选中节点输出 `{name: renderName(node), type, server, port, ...protocol_json}`，**禁止输出 name/type/server/port 之外的冲突字段覆盖**；敏感字段解密后输出。
     - 勾选 xray 节点时在 proxies 区写注释行 `# {{xray_nodes}}`（占位），否则不写。
     - `proxy-groups:`：先三个强制组（直接连接=DIRECT；国外流量=本次 overseas members；无法归属的流量=[DIRECT, 国外流量]），再按勾选顺序输出预设/自建组定义（子组名原样；节点成员按 `renderName(node)` 输出，xray 节点同样适用）。
     - `rules:`：勾选池按序、池内条目按序输出 `- TYPE,VALUE,TARGET`；IP 类加 `no-resolve`；**USER-AGENT 跳过并记录**；手动规则行追加在池后（同目标规则格式）；末尾固定 `- GEOIP,CN,直接连接`、`- MATCH,无法归属的流量`。
     - `render_plan_json`：结构化保存头部、manual proxies、proxy-groups 结构（含引用关系）、rules 与兜底；**节点引用在计划内统一存 `nodes.name` 稳定键**，Build6 下载重渲染时按节点表实时映射 `renderName`（字段可自行设计但必须自包含且能无状态重建全文）。
  4. **`backend/internal/assembly/render_sr.go`**：
     - `sr-conf`：`[General]` 按 FixedParams 输出 `key = value`；`[Rule]` 条目 + `GEOIP,CN,DIRECT` + `FINAL,{PROXY|DIRECT}`；USER-AGENT 保留；IP 加 no-resolve。
     - `sr-subs`：明文 = `STATUS={}` + `REMARKS={}` + 逐行节点链接 +（如勾选 xray 节点）`# {{xray_nodes}}`。
     - `generic-subs`：明文 = 逐行标准节点链接 +（如勾选 xray 节点）`# {{xray_nodes}}`；无头部行。
     - **链接渲染**统一在 `links.go`：每个可转协议一个函数，输入节点+凭据 map，输出 URI；无映射返回 SkipItem。
  5. **`backend/internal/assembly/links.go`**：链接编码规则严格按 [Node-Link-Standards.md](./docs/Reference/Node-Link-Standards.md) 二/四章与 Design2 §4.5 实现：
     - 公共：节点名统一取 `renderName(node)`，经 `url.QueryEscape`（或 encodeURIComponent 等价）作为 `#fragment`/remarks；域名非 ASCII 转 punycode（`golang.org/x/net/idna`，已在依赖图）；参数值用 `url.Values.Encode` 后把 `+` 替换为 `%20`（避免空格不对称）。
     - ss（SIP002）：`ss://base64(cipher:password)@host:port#name`（标准 base64，与样例一致）。
     - vmess-SR：`vmess://base64("auto:{uuid}@{host}:{port}")?remarks={name}&udp=1&alterId=0`（manual 节点无 UUID 时用 protocol_json.uuid）。
     - vmess-generic：`vmess://base64(JSON)`，JSON 必含 `v:2,ps,add,port,id,aid:0,scy:auto,net,type,host,path,tls`；不追加 query/fragment。
     - vless-SR：`vless://base64(":uuid@host:port")?remarks=...`，security=none 不附 tls；tls 附 `tls=1&peer={servername}`；reality 附 `tls=1&xtls=2&peer={sni}&pbk={public_key}&sid={short_id}`。
     - vless-generic：`vless://uuid@host:port?encryption=none&type=tcp&security=...&sni=...&fp=...&pbk=...&sid=...&flow=...#name`。
     - trojan / anytls / hysteria / hysteria2 / tuic / wireguard / http / socks5 按 Node-Link-Standards §2 的查询参数表逐项实现；缺字段不输出。
     - **不可转协议**：snell/mieru/masque/openvpn/ssh/shadowquic/trusttunnel/tailscale → `SkipItem{Kind:"node", Name:..., Reason:"协议无标准链接映射"}`。
  6. **`backend/internal/assembly/validate.go`**：
     - 输入存在性与语法目标匹配（clash-yaml→platform product_type=yaml；sr-subs→subs；generic-subs→generic-subs；sr-conf→rule）。
     - 规则类型白名单（与 pool 共用校验函数）；目标组存在性；**勾选的预设组必须 `type=preset AND enabled=1`，停用预设组拒绝生成（400「预设组已停用，请先启用或移除勾选」）**。
     - 空产物校验：Clash `overseas members` 为空拒绝（文案「『国外流量』组未包含任何节点」）；sr-subs/generic-subs 选中节点为空或转换后有效链接为 0 拒绝。
     - 规则为空允许生成（返回提示而非错误，提示由 Skipped/Warning 携带）。
  7. **单测（本 Step 重点）**：golden 测试四种产物（输入固定，比对关键行与占位标记有无；**含 xray 节点 display_name 非空与空回退两种渲染分支**）；链接编码与 `Shadowrocket.subs.template.md` 样例形态一致；中文名/emoji、punycode、空格转义；USER-AGENT Clash 跳过；IP no-resolve；兜底顺序；空产物校验；**停用预设组拒绝生成**；**1 万规则渲染 benchmark 测试（`BenchmarkRenderClash10kRules`）**。

- **参考代码/伪代码：**

  **Clash 输出骨架（伪代码）**

  ```yaml
  port: 7890
  mode: rule
  dns:
    ...
  proxies:
    - name: "节点A"
      type: vless
      server: example.com
      port: 443
      uuid: ...
    # {{xray_nodes}}
  proxy-groups:
    - name: "直接连接"
      type: select
      proxies: [DIRECT]
    - name: "国外流量"
      type: select
      proxies: ["节点A", "直接连接"]
    - name: "无法归属的流量"
      type: select
      proxies: [DIRECT, "国外流量"]
    - name: "YouTube"
      type: url-test
      proxies: ["节点A", "直接连接"]
  rules:
    - DOMAIN-SUFFIX,example.com,YouTube
    - IP-CIDR,1.2.3.0/24,YouTube,no-resolve
    - GEOIP,CN,直接连接
    - MATCH,无法归属的流量
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/assembly/... ./...
  cd backend && go test ./internal/assembly -run TestRender10kRules -v  # 示例：断言 <500ms
  ```

- **验收标准：** 全部测试通过；golden 产物满足模板语义；benchmark 达标；渲染内核无任何 HTTP/DB 写操作（仅注入只读加载器接口，便于测试）。



---

### Step 4：装配 HTTP 端点与版本/蓝图集成

**本 Step 完成后，`/api/admin/assembly/context`、`/api/admin/assembly/preview`、`/api/admin/assembly/generate`、`/api/admin/versions/:id/blueprint` 可用；generate 在同一版本事务内写入蓝图；版本列表返回 blueprint 标记。**

- **目标：** 把渲染内核接入 HTTP 与版本组件。
- **前置条件：** Step 3 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/server/assembly.go`**：
     - `GET /api/admin/assembly/context`：一次性返回 `{nodes, proxy_groups, pools, platforms, rules}`（形状按 Design2-UI §9.1；pools 为摘要含 entry_count；nodes 列表不返回凭据明文）。
     - `POST /api/admin/assembly/preview`：请求 = GenerateInput；调用 `assembly.Preview`，返回 `{content, skipped, warnings}`（纯文本内容），**不落库**；**前端调用统一 `timeout: 120_000`**。
     - `POST /api/admin/assembly/generate`：
       1. 校验 + 渲染；
       2. 定位 owner：subscription 类按 platform_id 唯一订阅；sr-conf 按 rule_id；
       3. 构造 `version.TextContent{Name: targetFileName(target), Text: content}`；
       4. 调用 `versionSvc.CreateVersion(ctx, ownerType, ownerID, src, version.CreateOptions{Activate:false, AfterCreate: func(tx, no, content) { return assemblySvc.SaveBlueprintTx(ctx, tx, no, in, renderPlan) }})`；
       5. 返回 `{version_id, auto_activated, skipped, warnings}`；**前端调用统一 `timeout: 120_000`**。
     - `GET /api/admin/versions/:id/blueprint`：读 assembly_blueprints + 校验引用，返回 `{blueprint, invalid_refs:[{kind,name}]}`（悬空项标记口径 Design2 §4.4/UI §5.4）。
  2. **`backend/internal/version/version.go` 小改**：`Version` 增加 `Blueprint bool`（json `blueprint`）；`ListVersions` SQL 增加 `EXISTS(SELECT 1 FROM assembly_blueprints b WHERE b.version_id = v.id)` 列。
  3. **`backend/internal/server/server.go`**：构造 assembly 服务（注入 store/version/node/proxygroup/pool/cfg/logger），注册路由。
  4. **错误处理**：校验类 400（message 定位到字段/组/池）；目标平台尚无订阅条目 → 400「请先在订阅管理为该平台创建订阅条目」（不自动创建）；目标不存在 404；版本组件错误按既有映射；所有响应统一结构。
  5. **单测**：preview 不产生版本与 blueprint 行；generate 首版 auto_activated=true 且 blueprint 1:1；第二版 auto_activated=false 且未激活；AfterCreate 失败时版本文件与记录回滚；blueprint 悬空引用标记。

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/assembly/... ./internal/version/... ./internal/server/... ./...
  ```

- **验收标准：** 全部测试通过；四个端点手工 curl 走查（管理员会话）符合 UI §9.1 契约。



---

### Step 5：节点管理页与代理组管理页前端

**本 Step 完成后，`/admin/nodes` 与 `/admin/proxy-groups` 两个新页面按 Design2-UI 第六章/第七章完整实现。**

- **目标：** 落地基础模式的两个管理页。
- **前置条件：** Step 4 验收通过。
- **产出文件与操作：**

  1. **`frontend/src/api/node.ts`**：按 UI §9.1 `api/node.ts` 表实现（list/create/update/delete/getProtocols/toggleNode/setNodeDisplayName）；类型含 `source/protocol/host/port/is_public/enabled/allocatable/missing/display_name/render_name`。
  2. **`frontend/src/api/proxyGroup.ts`**：list/create/update/delete/togglePreset；`definition` 含组类型与有序 nodes/groups。
  3. **`frontend/src/views/admin/NodesView.vue`**：按 UI §6.1~6.3 实现：
     - 双态列表；manual/xray 标签；节点名显示 `render_name`，xray 有自定义 display_name 时双行展示系统名；行内 enabled 开关（loading，失败回滚）；is_public 仅 source=xray 且 allocatable=1/missing=0 渲染，切换前 ConfirmModal；xray 行无整体编辑按钮，提供「命名」弹窗（display_name，留空清空，409 字段级提示），删除仅 missing=1；manual 删除影响说明。
     - manual 新建/编辑弹窗（720px）：协议下拉（注册表）、动态表单、凭据字段 `a-input-password` + 「留空 = 保留原凭据」、名称创建后只读、实时校验、409 提示。
  4. **`frontend/src/views/admin/ProxyGroupsView.vue`**：按 UI §7.1~7.2：
     - 双态列表；preset/custom 标签；组类型 Tag；成员摘要；预设启用勾选（行首 Checkbox，即时保存）；预设不可删、自建组名只读。
     - 自建组创建/编辑弹窗（720px）四区：基本信息、节点引用（有序，xray 节点显示 render_name，有自定义名时副行系统名；**提交时发送 nodes.name 稳定键**；拖拽 ≥768，<768 上移/下移）、子组引用（含强制组两项）、校验与保存（DAG 前端即时检测 + 内容约束 + 悬空引用红标剔除）。
  5. **排序交互**：新建 `useSortableList` 组合式函数（`HolderOutlined` 拖拽 + <768 上移/下移；无外部拖拽库）。
  6. **router**：`/admin/nodes`、`/admin/proxy-groups` 页面组件替换 Build4 占位。
  7. **前端单测**：节点表单动态渲染/敏感字段留空；代理组环检测提示；移动端排序降级渲染。

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test
  cd ../backend && go test ./...
  ```

- **验收标准：** 前端构建与测试通过；两页空态/加载/错误/危险确认/权限/防重复八项自检（按 UI §6.4/§7.3）走查通过。



---

### Step 6：订阅装配页前端（四装配器 + DiffView + 重新编辑）

**本 Step 完成后，`/admin/assembly` 四个装配器页签完整可用，支持分步/单页双形态、六步流程、预览 diff、生成回执与重新编辑流。**

- **目标：** 落地 Design2-UI 第五章最大新增页。
- **前置条件：** Step 5 验收通过。
- **产出文件与操作：**

  1. **`frontend/package.json`**：安装 `diff`（jsdiff）依赖（`npm install diff` + `npm install -D @types/diff`），版本用 npm 当前稳定版并在 package-lock 落锁。
  2. **`frontend/src/api/assembly.ts`**：按 UI §9.1 `api/assembly.ts` 表实现 `getAssemblyContext/getBlueprint/generate/preview`；请求类型与后端 GenerateInput 对齐（**规则素材池为有序数组**；节点/子组引用发送 `nodes.name` / `proxy_groups.name` 稳定键，不发送显示名）。
  3. **`frontend/src/components/DiffView.vue`**：jsdiff `diffLines`；三色高亮（新增绿底/删除红底/上下文默认）；等宽字体、max-height 60vh 纵向滚动；目标版本不存在时整体新增；禁止 monaco。
  4. **`frontend/src/views/admin/AssemblyView.vue`**：Build4 五页签壳保留，四个装配器页签替换为真实组件。
  5. **装配器子组件**（建议目录 `frontend/src/views/admin/assembly/`）：
     - `AssemblerShell.vue`：双形态切换（`a-segmented` 分步/单页，localStorage `assembly_layout_mode` 共享）；步骤条动态隐藏跳过步骤（Clash 六步；SR subs/generic 五步跳④；sr-conf 五步跳③）；单页纵向分区 + 底部「预览产物」。
     - `TypeTargetStep.vue`：类型只读；目标平台按 product_type 过滤（无匹配时空态引导建平台）；sr-conf 目标规则实体选择（含空实体后缀与新建空规则快捷）。
     - `HeaderStep.vue`：Clash 头部表单（默认值按 `Clash.yaml.template.md` 头部内置常量预填）+「一键采用默认值」ConfirmModal；SR subs STATUS/REMARKS；sr-conf [General]；generic 无头部。
     - `NodesGroupsStep.vue`：manual/xray 双来源分组；**xray 节点显示 render_name，有自定义 display_name 时副行系统名**；allocatable=0 置灰；missing=1 不列；代理组三区块（强制组锁定；预设组勾选；自建组勾选）；「国外流量」成员配置。
     - `RulesStep.vue`：已勾选池有序列表（拖拽/上移下移）+ 每池目标选择 + 手动规则行；Clash 与 sr-conf 共用，目标控件分别为代理组选择与 PROXY/DIRECT。
     - `PreviewStep.vue`：`preview` 请求（不落库）→ 纯文本预览；「与当前激活版本对比」开关 → `DiffView`；跳过项 `a-alert warning` 清单；占位标记旁 Tooltip。
     - `GenerateStep.vue` / 回执：生成校验前端预检 + 后端兜底；成功 `a-result`「已入池未生效，请激活」（`auto_activated=true` 时「首个版本已自动激活」）+「去版本管理激活」/「继续装配」。
  6. **重新编辑流**：路由 `?tab={target_syntax}&edit_version_id={id}` → `getBlueprint` 载入 → 顶部 alert「正在重新编辑版本 vN」→ 失效引用红标 + 一键剔除 → 正常生成新版本。
  7. **前端单测**：装配器页签/query 回退、步骤跳过、表单校验、preview 调用、diff 渲染、重新编辑失效剔除。

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test
  cd ../backend && go test ./...
  ```

- **验收标准：** 前端构建与测试通过；四种装配器均可走通「选目标→填头部→勾选→预览→生成→回执」；重新编辑从版本页入口可回填；移动端/暗色/防重复/空态自检按 UI §5.5 通过。



---

### Step 7：分发 UI 收口、渲染 benchmark 与 Build5 端到端验收

**本 Step 完成后，Build5 里程碑达成：订阅地址池/版本管理/平台/规则页面完整承载装配版本与「入池未生效」引导；用户端与管理员预览形态正确；渲染性能达标。**

- **目标：** 收口 Build4 已改造页面中留待装配的部分，并完成端到端验收。
- **前置条件：** Step 6 验收通过。
- **产出文件与操作：**

  1. **`frontend/src/api/version.ts`**：`VersionItem.blueprint: boolean` 接通；`versionApi` 增加 `getBlueprint(versionId)`（供版本页重新编辑入口，内部走 assembly API）。
  2. **`frontend/src/views/admin/SubscriptionsView.vue`**：
     - 列表新增「内容形态」标签：装配模板紫 / 直接上传灰（数据源由订阅行/版本列表的 blueprint 字段提供）。
     - 上传/装配生成后「已入池未生效，请激活」高亮 + 「去激活」快捷链接；PageHeader「前往装配」带目标平台参数。
     - 删除 ConfirmModal 影响清单按 UI §4.1 新写。
  3. **`frontend/src/views/admin/VersionManageView.vue`**（按 Design2-UI §4.2）：
     - 创建区第三入口「装配生成」→ `/admin/assembly?tab={对应装配器}&platform_id={平台}`（订阅）；规则页 `tab=sr-conf&rule_id={规则}`；分享/自定义不渲染。
     - `blueprint` 版本行显示紫色「装配」标签 + 「重新编辑」按钮；直接上传版本不显示。
     - 订阅/规则「激活/分发」文案与确认弹窗（分享/自定义保持「设为当前」）；首次自动激活提示。
  4. **`frontend/src/views/admin/RulesView.vue`**：规则版本页装配入口；空实体「可作为 SR 分流规则装配目标」引导。
  5. **`frontend/src/views/HomeView.vue`**：管理员「按平台预览当前版本」按钮接 `/api/subscriptions/preview?platform={slug}`（无 subscription_id）；弹窗纯文本，装配模板显示含 `# {{xray_nodes}}` 的原文。
  6. **后端渲染 benchmark 收口**：确保 `BenchmarkRenderClash10kRules` 与 SR conf 同量级 benchmark 可跑并记录；若超过 500ms，先做 profile 定位（预编译模板/缓冲池/减少重复序列化），禁止通过降低产物完整性优化。
  7. **端到端手工动线**（全新库，Build4 数据可复用）：
     - 新建 manual 节点 2 个（vless + 不可转协议 snell）→ 代理组新建自建组 → Clash 装配：选平台、头部默认、勾节点、配置国外流量成员、勾素材池指向组 → 预览（含跳过项）→ 生成 → 订阅行「已入池未生效」→ 激活 → 用户端下载可读。
     - SR 节点订阅装配：勾 vless 成功、勾 snell 时预览提示跳过；只勾 snell 生成被 400 拒绝。
     - SR 分流规则装配：选空规则实体 → [General] 默认 + FINAL,PROXY → 勾池 → 生成 → 首版自动激活 → 规则 Token 下载 conf。
     - generic-subs 装配 → 激活 → 下载内容整体 base64、明文为纯链接行。
     - 重新编辑：从版本页进入任一装配版本 → 删除一个被引用节点 → 重进显示失效项 → 剔除后生成新版本。
  8. 更新 Build5 进度表与变更记录；未实现项按约束归 Build6/7。

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd backend && go test ./internal/assembly -run TestRender10kRules -v
  cd ../frontend && npm run build && npm test
  ```

- **验收标准：** 全部命令通过；手工动线完整；benchmark <500ms 级；前端无旧占位页；`grep -R "即将推出" frontend/src/views/admin/AssemblyView.vue` 无结果。

---

## 五、候选构建项（已确认归属后续 Build）

| # | 候选 | 说明 | 归属 |
|---|------|------|------|
| 1 | Xray 实例录入/连通性/节点检测入库 | Design2 §5.2/§5.9 | Build6 |
| 2 | 组节点分配/候选集/公共节点与高级中间件 | Design2 §5.6/§5.10 | Build6 |
| 3 | 用户生命周期同步/凭据/批量初始化 | Design2 §5.5 | Build6 |
| 4 | 下载动态注入与 Subscription-Userinfo | Design2 §5.7 | Build6 |
| 5 | 流量采集/配额/超限 | Design2 §5.8 | Build6 |
| 6 | 实例对账/独立账号/配置导入导出 v2/OFF 清空 | Design2 §5.10/§5.11 | Build7 |
| 7 | Xray 实例页与独立账号 UI、组/用户/设置高级 UI | Design2-UI §4.3/§4.5/§4.7/§8 | Build7 |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 初始版本：Build5 构建方案（节点/代理组/四类装配器/分发收口），7 个 Step；xray 运行时能力明确划归 Build6/7 |
| v1.1 | 2026-08-19 | Design2Report5 核验修订：代理组名双向命名空间校验；组类型创建后允许修改；组内容约束收紧为三选一口径；预设组种子回归；停用预设组拒绝装配；preview/generate 统一 120s；NodesGroupsStep 显示名双行展示 |

