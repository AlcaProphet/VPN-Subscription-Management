# VPN 订阅管理系统 增量说明（Build23：BuildReport4 未闭环项 3 交接）

> **文档定位：** 本文档最初是 R27-09 剩余步骤的独立构建方案。经文档归属整理，R27-09 主体步骤已并入 [Build21.md](Build21.md) §7，作为唯一详细执行记录；Build23 不再重复这些 Step，仅保留交接关系、研究背景、用户确认边界与后续候选。
> - 设计记录：[Design4.md](Design4.md)（当前设计记录；与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - 问题来源：[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md)（全量核验报告，未闭环项 3）
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 问题追踪：[Issue14.md](Issue14.md)（Build21 Step 14 工程问题与证据缺口）；R27 历史记录见 [Issue13.md](Issue13.md)
> - 用户人工验收：[ProdTestList.md](ProdTestList.md)（Production、浏览器和真实客户端结果以此为准）
> - 历史构建与问题记录：见 [docs/reports/](docs/reports/)（均已存档，仅核查）
>
> **用户已确认的决策：**
> 1. Build23.md 放仓库根目录，作为当前构建方案的交接说明。
> 2. 研究/构建范围覆盖 BuildReport4 未闭环项 3，并纳入 Build21 曾排除的 N-node-3/N-node-4。
> 3. `target_evidence` 仅按 SS 插件合同派生诊断，不全局启用所有字段级证据。
> 4. 未知 SS 插件在 Clash 输出中保留结构化 `plugin` + `plugin-opts`，并给出未验证 warning，不阻断。
> 5. SR VMess 输出包含 `alpn`/`fp`，但这些字段没有固定版本解析器证据，必须标注 Shadowrocket 真机待验证。
> 6. generic VMess 本次不补充 `skip-cert-verify`，保持现状并在文档/测试中记录该边界。
>
> **执行入口：** 后续实施 R27-09 时，以 [Build21.md](Build21.md) §7 的 Step 13～15 为唯一分步计划；Build23 不再独立维护 Step 1～5。
>
> **研究结论摘要：**
> - N-node-1：未知插件存储/URI 导入已由 Build21 修复，但 Clash 输出仍拍平并删除结构化 `plugin-opts`，自检也不识别旧 URI 字符串格式。
> - N-node-2/N-node-5：`target_evidence` 仅是元数据且未被检查链路消费；SS 诊断硬编码，导致 v2ray-plugin/shadow-tls/restls 的 URI 检查可能误报 `ok`。
> - N-node-3/N-node-4：SR VMess/VLESS 缺少 TLS/ALPN/指纹/Flow/Skip 参数，`uriparse` 的 SR VMess 回读也不完整。
> - 修复方向：Clash 结构化投影 + 产物自检、SS 插件统一目标诊断、未知插件前端编辑、SR URI TLS 参数补全、全链路回归与文档收口。

---

## 一、与 Build21 的归属关系

| 原 Build23 内容 | 现归属 |
|---|---|
| Step 1：Clash/Mihomo 结构化 SS 插件投影与产物自检 | 已并入 [Build21.md](Build21.md) §7.8 Step 11 |
| Step 2：SS 插件统一目标诊断与正式装配门槛 | 已并入 [Build21.md](Build21.md) §7.9 Step 12 |
| Step 3：未知插件参数前端编辑、校验与分支清空 | 已并入 [Build21.md](Build21.md) §7.10 Step 13 |
| Step 4：VMess/VLESS SR URI TLS/ALPN/指纹/Flow/Skip 输出补全与解析同步 | 已并入 [Build21.md](Build21.md) §7.12 Step 15 |
| Step 5：全链路回归、固定版本证据、浏览器与文档收口 | 已并入 [Build21.md](Build21.md) §7.11 Step 14，并纳入 Step 15 的收口范围 |

后续若需调整 R27-09 或 N-node-3/4 的实施细节，应直接修改 Build21；本文档只作背景与边界记录。

---

## 二、保留的研究边界

1. **Shadowrocket 真机导入/连接**仍是人工项目，统一记录在 [ProdTestList.md](ProdTestList.md)；不能因 URI 可生成或内部往返通过就标记为完整兼容。
2. **未知 SS 插件参数**按用户确认作为普通字符串参数处理，不进入敏感字段/凭据模型；即使键名为 `password`、`token`、`secret` 也不按凭据处理。
3. **generic VMess**本轮明确不补充 `skip-cert-verify`，保持现状并在测试中作为负向边界。
4. **非 SS 字段级 `target_evidence`**不全局消费，仅按 SS 插件合同派生诊断，避免无关降级。
5. **固定版本证据**主要指 Mihomo 1.19.29 与 CVR 2.5.2 的离线/源码证据；Shadowrocket 仅有版本与公告证据。
6. **R27-08**（`diagnostics: []` 契约）已在当前代码中修复，后续回归需继续保持非空数组语义。
7. **N-node-6**（未知扩展/局部 JSON 边界）未在本轮单独闭环，后续应作为独立 Issue/Design 项明确处理或排除。

---

## 三、候选构建项（待用户决策，逐项转 Step）

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| 1 | Shadowrocket 真机导入/连接验收 | 已迁移至 [ProdTestList.md](ProdTestList.md)，结果以该清单为准；不属于自动化可闭环项 | Design4 §8.5；ProdTestList |
| 2 | 非 SS 字段级 `target_evidence` 全局诊断或前端逐字段证据展示 | 当前已确认仅按 SS 插件合同消费；全局启用会扩大影响面，建议作为后续独立优化 | Build21 §7.2 排除说明 |
| 3 | BuildReport4 未闭环项 1、2、4～6 | Build16/Design3、smoke、安全报告、人工验收等，均不属于本 Build 范围 | BuildReport4 结论摘要 |

> 候选转 Step 流程：用户确认后，直接在 Build21 或后续对应 Build 文档中追加 Step，不在本文件重复展开。

---

## 四、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-05 | 首次创建：根据 BuildReport4 未闭环项 3 完成未知 SS 插件输出丢失、target_evidence 未消费、v2ray/shadow/restls URI 诊断误报、VMess/VLESS SR TLS 参数缺失的根因研究、修复方向与分步构建计划；未修改任何业务代码。 |
| v2.0 | 2026-09-05 | 按 `docs/DocTemplates/Build.template.md` 重新排版：补充分步构建的模板结构（进度追踪、文件总览、依赖图、分步计划、候选项、变更记录）；补充 Build21 已落地步骤、CVR/Mihomo 源码证据、fixed-version 正例与真机边界。 |
| v2.1 | 2026-09-05 | 根据进一步源码研究与用户确认，明确 SR VMess 输出包含 `alpn`/`fp` 但保留 Shadowrocket 真机待验证；明确 generic VMess 不补充 `skip-cert-verify` 并加入负向回归。 |
| v3.0 | 2026-09-05 | 文档归属整理：按用户确认将 R27-09 主体步骤全部并入 Build21 §7，Build23 不再重复 Step 1～5；本文档改为交接说明与边界记录。 |
