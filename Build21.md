# Build21.md — BuildReport3 节点编辑器对齐修复构建计划与实施记录

> **文档定位：** 本文档是依据 [BuildReport3.md](docs/reports/BuildReport/BuildReport3.md) 开展的第二十一轮构建记录，将“Build17~Build20 表单结构与 Design4 对齐”发现的问题按用户确认的推荐方案落地。
> - 设计依据：[Design4.md](Design4.md)（当前最新节点编辑器设计）、[BuildReport3.md](docs/reports/BuildReport/BuildReport3.md)
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 前序构建：[Build17.md](Build17.md)～[Build20.md](Build20.md)、历史构建存档于 [docs/reports/Build/](docs/reports/Build)
>
> **本文件状态：** 已完成核心修复并验证构建；文档同步待后续归档或用户确认。

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

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-03 | 初次创建：根据 BuildReport3 完成节点编辑器字段分组、编辑回显、UI 分区、列表控件、SS 插件映射与输出投影修复；前后端构建与测试通过。 |
| v1.1 | 2026-09-03 | 补充 R27-04 修复 Step：表单 schema 投影、编辑草稿 TLS 清理与共用 WS 归一化；接口/服务/输出/前端回归及最新构建浏览器验证通过，记录客户端与 VMess SR 的验证边界。 |
