# Build34 — 素材池来源、装配候选与预览一致性

基线为 `beta` 的 `157cd31`。用户确认按四步实施，本地 8080 已更新。问题证据见 [Issue21 R35-03](../Issue/Issue21.md#r35-03-创建素材池后返回-clash-构建页面需要整页刷新)，现行口径见 [Design3 §2.4](../Design/Design3.md#24-来源配置变化) 与 [Design2-UI §5.1](../Design/Design2-UI.md#51-页面骨架单菜单入口页内-tabs)。

| Step | 目标 | 状态 |
|---|---|---|
| 1 | 保留未变更的 URL 来源与快照，缩小定时开关的写入范围 | 通过 |
| 2 | 装配与素材池管理共享按请求排序的池列表 | 通过 |
| 3 | 在条目写入成功、活动来源变化或池删除时使旧预览过期 | 通过 |
| 4 | 全量门禁、浏览器回归、本地重建与文档同步 | 通过 |

## Step 1 — 保留未变更来源

- **前置**是用本地 HTTP 规则源复现仅改名或切换定时后活动规则归零。影响覆盖现有 PUT 池编辑、行内开关、来源快照和活动规则顺序。
- **实现**位于 `backend/internal/pool/pool.go`、`backend/internal/server/pool.go`、`frontend/src/api/pool.ts` 和 `PoolTab.vue`。按 `URL + source_mode` 匹配存量来源并更新顺序。同一写事务内删除被移除或实质变化的旧来源、插入新来源并清理无 origin 的 canonical。行内开关走 `PATCH /api/admin/pools/:id/auto-sync`，只更新开关。用户确认保存来源后由管理员手动同步，或由已启用的定时同步处理。
- **验收**覆盖活动规则、来源和快照 ID 保留，来源重排后的读取顺序，显式删除／改模式的级联清理，以及开关的 `false`、缺字段和未知池。通过 `go test ./internal/pool ./internal/server -run 'Test(UpdatePreservesUnchangedPoolSources|CRUDAndSort|PoolAutoSyncPatchOnlyUpdatesSwitch)'`、后端 build/vet、前端 `npm run test -- pool-tab.spec.ts` 和 build。

## Step 2 — 统一池列表响应顺序

- **前置**是 Step 1 通过。前端原有多个池列表写入点，同一装配视图服务 Clash 和 SR 分流，且来自订阅、规则、版本和蓝图入口。
- **实现**位于 `AssemblyView.vue` 和 `PoolTab.vue`。发起 `assembly/context` 或 `listPools` 时分配递增序号，成功结果仅在序号不早于最近应用的结果时替换共享列表。池列表可先于装配上下文保存。构建页签返回时仍定向查询池列表。管理页处理同步轮询、操作反馈和详情。
- **验收**要求两个已复现的乱序场景中，新池可选，旧响应既不改变有效选择，也不误报删除。通过 `npm run test -- assembly-view.spec.ts pool-tab.spec.ts` 与 build。

## Step 3 — 通知与预览失效

- **前置**是共享列表能稳定反映次序。后端已用生成正文摘要拒绝过期请求。
- **实现**位于 `PoolDetail.vue`、`PoolTab.vue` 和 `AssemblyView.vue`。手工条目写入完成后，通过详情打开时接收的回调通知装配，随后刷新条目。删除已选池、刷新发现已选池消失或活动快照序列变化时，递增已有的预览修订号。纯元数据编辑保留有效预览。
- **验收**覆盖写入响应前关闭详情、写入成功但条目查询延迟、活动快照顺序变化且计数相同，以及改名和定时开关保留旧预览。通过 `npm run test -- assembly-view.spec.ts pool-tab.spec.ts pool-detail.spec.ts rules-step.spec.ts` 的 59 项和 build。

## Step 4 — 联合编译与本地运行验证

- **前置**是前 3 步全部通过。8080 原服务运行在项目 `backend/`，使用 `backend/data/app-prod.db`，切换前无运行中同步任务。
- **命令**为后端 `go build ./...`、`go vet ./...`、`go test ./...`、素材池与装配 HTTP 定向 `go test -race` 和 `go run ./cmd/errgate ./...`，前端 `npm run test` 和 `npm run build`，以及 `git diff --check`。前端全量 51 个文件、469 项通过，errgate 无未处理错误。Vite 报出既有的大分块体积提示。
- **浏览器回归**使用回环监听的临时 Production 实例、合成数据库、Chrome 和本地规则源。验证活动快照与规则保留、明确移除活动来源使预览过期、两条列表乱序路径、同数量手工条目更新后立即禁用旧预览，以及正常删除已选池。
- **本地重建**先生成 `frontend/dist`，在临时后端构建目录以该产物填充 `web/dist`，再执行 `CGO_ENABLED=0 go build -trimpath -o <本地二进制> ./cmd/server`。切换时沿用原工作目录和本地数据目录，保留旧二进制。8080 的 `/health` 返回 OK，装配分块为 `AssemblyView-DtNxS4Yj.js`，SHA-256 为 `18c7fb34cfd18c5882ed7b081e7b399c8b729760f476995633e15535c16cc43f`。
- **8080 验收**在 Clash 规则素材步骤新建无 URL 素材池，切回后原步骤可直接选中新池。测试池已删除并确认从列表移除。此次从临时后端目录嵌入生成文件，仓库差异涵盖源码、回归测试和文档。
- **验证边界**为 macOS 宿主构建、隔离浏览器和实际 8080 实例。Docker、远端 CI 和真实客户端连接不在本次验收范围。
