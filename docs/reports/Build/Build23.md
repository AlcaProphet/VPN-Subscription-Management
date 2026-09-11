# VPN 订阅管理系统 增量交接说明（Build23：BuildReport4 未闭环项 3，已归档）

> **文档定位：** 本文档最初是 R27-09 剩余步骤的独立构建方案。经文档归属整理，R27-09 主体步骤已并入 [Build21.md](Build21.md) §7，作为唯一详细执行记录；Build23 不再重复这些 Step，仅保留交接关系、研究背景、用户确认边界与后续候选。**本文件已归档至 `docs/reports/Build/Build23.md`，仅保留交接与边界记录，不再作为当前执行入口。**
> - 设计记录：[Design4.md](../../../Design4.md)（当前设计记录；与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - 问题来源：[BuildReport4.md](../BuildReport/BuildReport4.md)（全量核验报告，未闭环项 3）
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 问题追踪：[Issue14.md](../../../Issue14.md)（Build21 Step 14 工程问题、D3/N-core/N-node-6/安全等遗留工程问题）；R27 历史记录见 [Issue13.md](../Issue/Issue13.md)
> - 用户人工验收：[ProdTestList.md](../../../ProdTestList.md)（Production、浏览器和真实客户端结果以此为准）
> - 历史构建与问题记录：见 [docs/reports/](..)（均已存档，仅核查）
>
> **用户已确认的决策：**
> 1. Build23 作为 R27-09 交接说明，执行历史以 Build21 为准；现按文档归档规则存入 `docs/reports/Build/`。
> 2. 研究/构建范围覆盖 BuildReport4 未闭环项 3，并纳入 Build21 曾排除的 N-node-3/N-node-4。
> 3. `target_evidence` 仅按 SS 插件合同派生诊断，不全局启用所有字段级证据。
> 4. 未知 SS 插件在 Clash 输出中保留结构化 `plugin` + `plugin-opts`，并给出未验证 warning，不阻断。
> 5. SR VMess 输出包含 `alpn`/`fp`，这些字段没有固定版本解析器证据；在得到真机证据前必须标注待验证，本轮 PT-28-01～PT-28-05 已由用户完成相关人工核验。
> 6. generic VMess 本次不补充 `skip-cert-verify`，保持现状并在文档/测试中记录该边界。
>
> **执行入口：** R27-09 与 N-node-3/N-node-4 已实施完成，历史执行记录以 [Build21.md](Build21.md) §7 的 Step 7～15 为准；Build23 不再独立维护 Step 1～5。
>
> **研究结论摘要（当前状态）：**
> - N-node-1：未知插件存储/URI 导入与 Clash 结构化 `plugin`/`plugin-opts` 输出已由 [Build21.md](Build21.md) Step 8/10/11/13 修复；项目自检已能识别并拒绝旧 URI 字符串格式。
> - N-node-2/N-node-5：SS 插件范围内的目标诊断已由 [Build21.md](Build21.md) Step 12 接入节点检查与正式装配，v2ray-plugin/shadow-tls/restls 不再无条件误报 `ok`；非 SS 字段级 `target_evidence` 仍按用户确认不全局启用。
> - N-node-3/N-node-4：SR VMess/VLESS 的 TLS/ALPN/指纹/Flow/Skip 输出与解析已由 [Build21.md](Build21.md) Step 15 补全。
> - N-node-6：未知扩展/局部 JSON 边界主体与交叉审核缺口已由 Issue14 步骤四 R28-06 / [Build25.md](Build25.md) 完成；前端 `item_id_field` 白名单、保存定位稳定排序和条件隐藏清理证据已闭环，Build25 已归档，详见 [Issue14.md](../../../Issue14.md) R28-06。
> - 遗留：Build21 Step 14 的工程问题与安全/D3 等其他 BuildReport4 遗留工程问题统一见 [Issue14.md](../../../Issue14.md)。

---

## 一、与 Build21 的归属关系

| 原 Build23 内容 | 现归属 |
|---|---|
| Step 1：Clash/Mihomo 结构化 SS 插件投影与产物自检 | 已并入 [Build21.md](Build21.md) §7.8 Step 11 |
| Step 2：SS 插件统一目标诊断与正式装配门槛 | 已并入 [Build21.md](Build21.md) §7.9 Step 12 |
| Step 3：未知插件参数前端编辑、校验与分支清空 | 已并入 [Build21.md](Build21.md) §7.10 Step 13 |
| Step 4：VMess/VLESS SR URI TLS/ALPN/指纹/Flow/Skip 输出补全与解析同步 | 已并入 [Build21.md](Build21.md) §7.12 Step 15 |
| Step 5：全链路回归、固定版本证据、浏览器与文档收口 | 已并入 [Build21.md](Build21.md) §7.11 Step 14，并纳入 Step 15 的收口范围 |

后续若需调整 R27-09 或 N-node-3/4 的相关行为，应通过新的 Design/Build 流程处理；本文档只作背景与边界记录。

---

## 二、保留的研究边界

1. 本轮 **Shadowrocket 真机导入/连接**人工项目已由用户确认完成；后续新增客户端矩阵仍统一记录在 [ProdTestList.md](../../../ProdTestList.md)，不能因 URI 可生成或内部往返通过就替代相应人工结论。
2. **未知 SS 插件参数**按用户确认作为普通字符串参数处理，不进入敏感字段/凭据模型；即使键名为 `password`、`token`、`secret` 也不按凭据处理。
3. **generic VMess**本轮明确不补充 `skip-cert-verify`，保持现状并在测试中作为负向边界。
4. **非 SS 字段级 `target_evidence`**不全局消费，仅按 SS 插件合同派生诊断，避免无关降级。
5. **固定版本证据**主要指 Mihomo 1.19.29 与 CVR 2.5.2 的离线/源码证据；Shadowrocket 兼容性仍以人工导入/连接证据为准，本轮 PT-28-01～PT-28-05 已完成相关核验。
6. **R27-08**（`diagnostics: []` 契约）已在当前代码中修复，后续回归需继续保持非空数组语义。
7. **N-node-6**（未知扩展/局部 JSON 边界）主体与交叉审核缺口已由 [Build25.md](Build25.md) Step 0～4 及后续修复完成；前端固定对象高级 JSON 已放行 `item_id_field`，保存定位稳定排序和条件隐藏清理专项回归补齐，Build25 已归档。后续以 [Issue14.md](../../../Issue14.md) R28-06 为唯一跟踪入口。

> **交接更新（2026-09-10）：** N-node-6 已完成：未知扩展仅加密存档/诊断、不进入任何客户端产物；局部 JSON 固定对象默认拒绝未知键、开放 Map 显式白名单；父子 JSON 草稿阻止覆盖并定位；WireGuard `peers._credential_id` 高级 JSON 白名单、保存定位稳定排序和条件隐藏清理证据均已在缺口修复后重新执行全部自动化门禁。Build25 已归档；Build23 继续保留为历史交接与边界说明，不重新成为执行入口。浏览器、手机和真实客户端人工项见 [ProdTestList.md](../../../ProdTestList.md)，未标记为通过。

---

## 三、候选构建项（待用户决策，逐项转 Step）

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| 1 | 后续 Shadowrocket/客户端矩阵验收 | 本轮 PT-28 人工项目已完成；后续新增矩阵仍迁移至 [ProdTestList.md](../../../ProdTestList.md)，不属于自动化可闭环项 | Design4 §8.5；ProdTestList |
| 2 | 非 SS 字段级 `target_evidence` 全局诊断或前端逐字段证据展示 | 当前已确认仅按 SS 插件合同消费；全局启用会扩大影响面，建议作为后续独立优化 | Build21 §7.2 排除说明 |
| 3 | BuildReport4 未闭环项 1、2、4～6 | Build16/Design3、smoke、安全报告、人工验收等，均不属于本 Build 范围；工程项已登记至 [Issue14.md](../../../Issue14.md) R28-05/R28-09，R28-08 已记录为设计取向关闭，人工项见 [ProdTestList.md](../../../ProdTestList.md) | BuildReport4 结论摘要 |

> 候选转 Step 流程：用户确认后，直接在 Build21 或后续对应 Build 文档中追加 Step，不在本文件重复展开。

---

## 四、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-05 | 首次创建：根据 BuildReport4 未闭环项 3 完成未知 SS 插件输出丢失、target_evidence 未消费、v2ray/shadow/restls URI 诊断误报、VMess/VLESS SR TLS 参数缺失的根因研究、修复方向与分步构建计划；未修改任何业务代码。 |
| v2.0 | 2026-09-05 | 按 `docs/DocTemplates/Build.template.md` 重新排版：补充分步构建的模板结构（进度追踪、文件总览、依赖图、分步计划、候选项、变更记录）；补充 Build21 已落地步骤、CVR/Mihomo 源码证据、fixed-version 正例与真机边界。 |
| v2.1 | 2026-09-05 | 根据进一步源码研究与用户确认，明确 SR VMess 输出包含 `alpn`/`fp` 但保留 Shadowrocket 真机待验证；明确 generic VMess 不补充 `skip-cert-verify` 并加入负向回归。 |
| v3.0 | 2026-09-05 | 文档归属整理：按用户确认将 R27-09 主体步骤全部并入 Build21 §7，Build23 不再重复 Step 1～5；本文档改为交接说明与边界记录。 |
| v3.1 | 2026-09-08 | 文档交叉审核：更新研究结论为当前已落地状态（N-node-1/2/3/4/5 已由 Build21 处理），并将 N-node-6 及 BuildReport4 其余遗留工程项登记至 Issue14。 |
| v3.2 | 2026-09-10 | 追加交接状态：N-node-6 主体已由 Issue14 步骤四 R28-06 / Build25 实施，但前端固定对象高级 JSON 仍未放行 `item_id_field`，R28-06B 未闭环，Build25 保持根目录活跃；Build23 继续作为历史交接说明，不重新成为执行入口。 |
| v3.3 | 2026-09-10 | 文档交叉审核：修正此前“N-node-6 已完整闭环”的过时表述，保留候选与研究边界，明确 Build21、Build23、Build24 已归档；Build22 与 Build25 因仍有缺口保持根目录活跃。 |
| v3.4 | 2026-09-10 | N-node-6 缺口修复闭环：前端 `item_id_field` 白名单、保存定位稳定排序、条件隐藏清理与折叠/卸载边界回归完成；Build25 重新通过后端定向/全量/build/vet、前端定向/全量/build、Docker build、Production smoke 与 `git diff --check` 并归档。人工/真机项仍在 ProdTestList，未标记通过。 |
