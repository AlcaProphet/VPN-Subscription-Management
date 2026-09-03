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

## 五、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-03 | 初次创建：根据 BuildReport3 完成节点编辑器字段分组、编辑回显、UI 分区、列表控件、SS 插件映射与输出投影修复；前后端构建与测试通过。 |
