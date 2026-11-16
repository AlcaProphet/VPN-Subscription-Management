# Build33 — Clash／SR 装配规则门槛修复

> 设计见 [Design3 §7.2](../Design/Design3.md#72-渲染和回执)，问题和历史复现见 [Issue21 R35-02](../../../Issue21.md#r35-02-无素材池时-clash-构建预检未阻断)。工作基线为 `beta` 的 `5d53d2c`。编码约束见 [AGENTS.md](../../../AGENTS.md)。

| Step | 目标 | 状态 |
|---|---|---|
| 1 | 核对共用入口、渲染和自检边界 | 通过 |
| 2 | 落地最终规则计数与装配交互 | 通过 |
| 3 | 整体验收与文档同步 | 本地验收通过 |

## Step 1 — 影响排查

- **前置**：用户确认同时修复 Clash、SR 分流和覆盖层最终门槛，允许覆盖层独立供规则，`MATCH` 顺序风险仅提示。
- **检查和产出**：用 `rg -n 'previewAssembly|generateAssembly|CheckClashContent|/admin/assembly' frontend/src backend/internal` 追踪页面入口和共用自检。订阅管理、规则管理、版本管理和蓝图编辑均进入同一个装配页。相同缺口影响 Clash 与 SR 分流的分步和单页布局。两个纯节点目标各自有链接数门槛，但共用草稿会向后端携带当前目标用不到的字段。
- **验收**：区分装配、版本上传／在线编辑、节点目标检查和历史下载。管理员生成走新门槛。节点检查继续读取自检诊断，历史 Clash 下载的静态自检仍只作日志诊断。手工池返回 `urls: null` 会影响素材池列表及详情，作为新增素材池导引的关联修复。

## Step 2 — 修复实现

- **前置**：Step 1 确认调用边界。保留规则回执六个字段和历史蓝图的读取格式。
- **产出和参考路径**：`backend/internal/assembly/` 由一次目标映射生成转换回执、正文输入和 Clash 计划规则。Clash 自检读取应用覆盖层后的正文并计算最终条数，SR 在写出规则时记数。`backend/internal/server/assembly.go` 返回预览自检问题，生成时按新摘要和自检结果控制写入。`frontend/src/views/admin/AssemblyView.vue` 按次保存预览结果，校验规则来源、显示阻断原因，并按目标提交字段。`backend/internal/pool/pool.go` 为无 URL 素材池返回空数组。
- **验收**：覆盖层独立规则能生成，覆盖层删除唯一规则应拒绝。后置 `MATCH` 提示不阻断有效输出。SR 跳过的规则不得计数，没规则输入时不发预览请求。规则集引用缺失、无效策略、并发预览与旧预览摘要须有负例。运行 `cd backend && go test ./internal/assembly ./internal/server ./internal/pool`，及 `cd frontend && npm run test -- --run tests/assembly-view.spec.ts tests/rules-step.spec.ts tests/preview-step.spec.ts`。

## Step 3 — 本地验证

- **前置**：后端装配和前端组件定向回归通过。
- **验收命令**：后端 `go build ./...`、`go vet ./...`、`go test ./...`、`go run ./cmd/errgate ./...`，前端 `npm run test`、`npm run build`，仓库 `git diff --check`。后端装配及 HTTP 定向 race 回归通过，前端全量 51 个文件、465 项通过。固定 Mihomo v1.19.31 的本地 `-t` 接受含 `IP-SUFFIX`、有效 inline `RULE-SET` 与 `MATCH` 后补充规则的配置。
- **隔离浏览器**：将最新前端产物嵌入 CGO 关闭的本地二进制，临时副本绑定回环，使用独立合成测试数据库与 Chrome 验证。经订阅管理进入 Clash 分步装配，验证空池入口、零输出禁用、仅覆盖层的 `append` 规则带顺序提示并生成。经规则管理验证 SR 分流空输入在分步和单页均被拦截，手动规则可生成。两个纯节点目标均完成预览和生成。另验证由装配新建无 URL 空池、打开详情、返回装配立即选中新池，且空池本身仍为零输出。
- **发布边界**：本记录覆盖宿主构建、自动化、固定 Mihomo 配置检查与隔离浏览器。Docker、远端发布和真实客户端连通性尚未验收。
