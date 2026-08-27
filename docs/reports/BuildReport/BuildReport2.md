# BuildReport2.md — Build8/Build9/Build10 事后验收报告

> 核验日期：2026-08-27。核验对象：当前工作区代码（HEAD `cb2d355`，分支 `beta`）、[Build8.md](../../../Build8.md)、[Build9.md](../../../Build9.md)、[Build10.md](../../../Build10.md)、相关测试与 Production 容器 smoke。
> 结论前置：**Build8、Build9、Build10 的功能代码已在当前工作区真实落地，后端编译/静态检查/单测、前端类型检查/构建/单测全部通过；Production 镜像可成功构建；仓库内 `.smoke-test-prod.sh` 与 `.smoke-test.sh` 已修复并复测通过，现在可以直接一键跑通完整 Production smoke。**  
> 本次修复内容包括：smoke-prod 的 Bash 3.2 变量解析兼容、smoke 样本代理组改到当前模型、overlay 样本补齐协议必填字段，并为关键 API 响应增加成功校验，避免 smoke 假绿。

---

## 一、总览

| 维度 | 结果 |
|------|------|
| 后端编译 | ✅ `go build ./...` 通过 |
| 后端静态检查 | ✅ `go vet ./...` 通过 |
| 后端单测 | ✅ `go test ./...` 全绿（70 个 `_test.go` 文件，全部包 ok） |
| 前端类型检查/构建 | ✅ `npm run build`（`vue-tsc -b && vite build`）通过 |
| 前端单测 | ✅ 19 个 spec 文件、66 个用例全部通过 |
| 容器镜像构建 | ✅ `docker build` 多阶段构建成功，生成 `vpn-sub:smoke-prod` |
| Production smoke | ✅ `bash .smoke-test-prod.sh` 原样执行通过（四类装配器、URI 导入、overlay、v2 导出/导入） |
| Build8 文档进度 | ✅ 文档标记 Step 0~5 验收通过，代码/测试证据与文档一致 |
| Build9 文档进度 | ✅ 文档标记 Step 1~6 验收通过，关键文件与专项测试均存在 |
| Build10 文档进度 | ✅ 文档标记 Step 1~5 验收通过，关键文件与专项测试均存在 |
| 交付 smoke 脚本合规性 | ✅ 已修复 3 个问题并复测通过（见第五节） |

---

## 二、验收命令实测记录

```text
backend:
  go version                                    go1.26.6 darwin/arm64
  go build ./...                                PASS
  go vet ./...                                  PASS
  go test ./...                                 PASS（所有包 ok，含 assembly/uriparse/node/cron/version/server/xray/pool 等）

frontend:
  node                                          v26.7.0
  npm version                                   11.19.0
  npm run build                                 PASS（vue-tsc + vite build）
  npm test -- --run                             19 files / 66 tests PASS（jsdom 的 getComputedStyle 等非致命 stderr 不影响结果）

container:
  bash .smoke-test-prod.sh（修复后原样执行）     PASS：四类装配器 + URI 导入 + overlay + v2 导出/导入全部成功
  修复前复现问题                                 见第五节（Bash 3.2 变量解析、旧代理组样本、overlay 节点缺必填字段）
```

---

## 三、Build8 事后验收

### 3.1 结论

Build8（Issue5 R20 修复）五个 Step 的代码与测试均已落地，自动验证通过。未发现“只有文档、没有实现”的 Step。唯一需要保留的是：R20-11 本身是“环境已不可用，未能复现”的文档闭环，无法在当前环境做实证，符合其既定处理方式。

### 3.2 分 Step 证据

| Step | 代码/测试证据 | 结论 |
|------|--------------|------|
| Step 0 创建文档与决策回写 | `Build8.md` 存在，包含用户决策记录 | ✅ |
| Step 1 数据安全/一致性 | `pool/sync.go` Scanner 错误返回、`config/export.go` 导入保护、`assembly/validate.go` GroupNodeOrders 校验、`xray/instance.go` 删除校验等均存在；相关单测通过 | ✅ |
| Step 2 Xray/高级模式语义 | `xray/reconcile.go` CredentialsOne/pushUserTarget、`xray/ext.go` 凭据重推、`xray/offclear.go` 确认词、`server/server.go` CloseAll 等均存在；专项测试通过 | ✅ |
| Step 3 同步超时/取消 | `pool/sync.go` CancelSync、`server/pool.go` cancel 路由、`PoolTab.vue` 取消按钮均存在；pool/server 测试通过 | ✅ |
| Step 4 前端/测试/文档/smoke | 前端 build/test 通过；README/文档已更新；`.smoke-test-prod.sh` 已修复并复测通过（见第五节） | ✅ |
| Step 5 R20-11 闭环 | `Issue5.md`/`ProdTestList.md` 有闭环记录 | ✅（文档闭环） |

---

## 四、Build9 / Build10 事后验收

### 4.1 Build9（第一阶段 Step 1~6）

| Step | 关键落点 | 测试证据 | 结论 |
|------|---------|---------|------|
| Step 1 R22-03 收口 | `AssemblyView.vue` 条件化目标选择区；`assembly-view.spec.ts`/`home-view.spec.ts` 更新 | 前端 66 测试通过 | ✅ |
| Step 2 goccy YAML + 静态自检 | `assembly/goccy_yaml.go`、`selfcheck.go`、`render_clash.go`、`clash_plan.go` 迁移 | `TestClashEmojiRaw`、`TestGoccyAutoInt`、`TestGoccyCommentMap`、`TestSelfCheck*` 通过 | ✅ |
| Step 3 响应头语义/RFC 5987 | `download/download.go`、`server/download.go`、`platform/platform.go` | `TestDownloadContentDispositionUsesRFC5987AndOverridesPlatform` 等通过 | ✅ |
| Step 4 节点协议注册表/链接 | `node/registry.go`、`node/node.go`、`assembly/links/links.go`、`NodesView.vue` | 注册表/链接/敏感字段/归一化测试通过 | ✅ |
| Step 5 规则全集/素材池解析 | `rulespec/spec.go`、`pool/parser.go`、`selfcheck.go`、`AssemblyView.vue` | rulespec/pool/selfcheck/assembly 测试通过 | ✅ |
| Step 6 代理组类型/字段扩展 | `proxygroup/proxygroup.go`、`assembly/{models,load,clash_plan,render_clash,selfcheck}.go`、`ProxyGroupsView.vue` | proxygroup/assembly/前端测试通过 | ✅ |

### 4.2 Build10（第二阶段 Step 1~5）

| Step | 关键落点 | 测试证据 | 结论 |
|------|---------|---------|------|
| Step 1 分层装配覆盖层 | `assembly/overlay.go`、`overlay_test.go`、`OverlayStep.vue`、蓝图/下载重渲染接线 | `TestOverlaySeqSemantics`、`TestOverlayMergeAndControlPlane`、`TestOverlayCleanupAndSort`、`TestRenderClashPlanOverlay`、`TestBlueprintOverlayRoundtrip` 通过 | ✅ |
| Step 2 URI 批量导入 | `uriparse/uriparse.go`、`node/uri_import.go`、`server/node.go`、`NodesView.vue` | `TestImportURIsBatch`、`TestImportURIsDuplicateSkipped`、uriparse 多协议测试通过 | ✅ |
| Step 3 池启动补跑 | `cron/pool.go`、`cron/pool_test.go` | `TestRunPoolAutoSyncTriggerDueOnce`、`SameMinuteDedup`、`SkipsRunning`、`CatchUpMissed`、`DueAndMissedDedup` 通过 | ✅ |
| Step 4 版本原子写 + 下载接线 | `version/version.go`、`server/render.go`、`download/download.go` | `TestWriteFileAtomicNoTempLeft`、版本并发/回滚测试、assembly 覆盖层重渲染测试通过 | ✅ |
| Step 5 前端/测试/文档/smoke 收口 | 前端 Overlay/批量导入 UI、spec 扩展、文档同步 | 前端 build/test 通过；仓库 smoke 修复后全链路通过 | ✅ |

---

## 五、已修复问题与验证记录

### 5.1 已修复：`.smoke-test-prod.sh` 在 macOS 自带 Bash 3.2 下不能启动

- **原现象**：原样执行 `bash .smoke-test-prod.sh`，Docker 镜像构建成功后立即报：
  `PORT�: unbound variable`
- **根因**：第 27 行 `echo "==> 启动临时 Production 容器（端口 $PORT）"` 中，`$PORT` 后紧跟全角右括号 `）`。在当前 Bash 3.2 + `set -u` 环境下，多字节字符被并入变量名解析，导致变量名不是 `PORT`。
- **修复**：改为 `${PORT}）`，明确变量边界。
- **验证**：修复后 `.smoke-test-prod.sh` 可直接启动容器并进入业务 smoke。

### 5.2 已修复：`.smoke-test.sh` 仍按旧代理组模型创建样本

- **原现象**：第一个 Clash 装配生成返回 400：`参数错误: select 组至少需要 groups/use/include-all 之一`，随后 `代理组不存在: smoke-group`。
- **根因**：Build9/Build10 已取消“代理组全局保存节点引用”，改为装配时通过 `group_node_orders` 传节点；同时自建 select 组必须至少包含 `groups/use/include-all` 之一，否则创建即被拒绝。仓库 smoke 脚本仍使用旧字段 `nodes` + 空 `groups`。
- **修复**：
  1. 创建样本组时改为 `"groups":["🚀直接连接"]`，使自建组可合法创建；
  2. Clash 与 overlay 装配请求补充 `"group_node_orders":{"smoke-group":["smoke-node"]}`；
  3. 对创建代理组、四个装配器、overlay 生成等关键响应增加 `require_success` 校验，失败立即退出。
- **验证**：修复后四类装配器全部通过。

### 5.3 已修复：`.smoke-test.sh` 的 overlay 样本节点缺少必填字段

- **原现象**：修正问题 2 后，overlay 生成仍返回 400：`产物自检未通过：ss 缺少必填字段 cipher`。
- **根因**：Build9 Step 4 协议注册表要求 `ss` 节点必填 `cipher`、`password`，而 smoke 的 overlay prepend 节点只写了 `name/type/server/port`。这说明自检在工作，但 smoke 样本未同步到新注册表。
- **修复**：overlay prepend 的 `ss` 节点补充 `cipher: aes-256-gcm` 与 `password: test`。
- **验证**：修复后 overlay 装配生成通过，得到真实 `version_id`。

### 5.4 其他遗留/可选改进

- 已增加 `require_success` 与 URI 导入回执数量断言，避免关键 API 失败时仍打印 `SMOKE ALL DONE` 的假绿。
- smoke 脚本仍未覆盖 Build10 Step 5 所列全部验收项（如下载响应 `filename*`、`profile-update-interval`、真实 emoji、重新编辑回填 overlay），这些目前主要由后端单测覆盖；后续可继续补强为可选增强项。
- 当前 HEAD 提交信息为 `feat: 完成 Build 8 的构建`，但该提交实际包含 Build10 Step 1~5 的大量代码（overlay、uriparse、cron 等），提交命名与内容存在错位，建议后续整理 git 历史/提交说明，避免验收追溯混淆。

## 六、核验口径说明

- 本次 v1.1 已实际修改仓库内 `.smoke-test.sh` 与 `.smoke-test-prod.sh`，并运行 `bash .smoke-test-prod.sh` 复测通过。
- “真实完成构建”的判定基于：代码文件存在、关键接口/算法可从源码核对、后端与前端自动验证全绿、Docker 镜像可构建、仓库 smoke 脚本可直接端到端跑通。
- 主验收链路现已达标；剩余 Build10 Step 5 中少数“下载头/emoji/重新编辑回填”等断言属于 smoke 覆盖可选增强项，目前由后端单测覆盖。

---

## 七、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-27 | 首次创建：Build8/Build9/Build10 事后验收；后端/前端全绿；Production 镜像构建成功；发现 3 个 smoke 脚本可复现问题及提交信息错位观察。 |
| v1.1 | 2026-08-27 | 修复 smoke：`.smoke-test-prod.sh` 兼容 Bash 3.2；`.smoke-test.sh` 改为当前代理组模型并补齐 `group_node_orders`、overlay `ss` 必填字段；增加关键 API `require_success` 和 URI 回执断言；复测 Production smoke 全链路通过。 |
