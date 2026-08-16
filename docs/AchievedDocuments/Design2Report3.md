# Design2Report3.md — Design2.md 增量能力设计深度研究验证完成报告（第二轮独立复审）

> **报告定位：** 本文档是对 [Design2.md](../../Design2.md)（增量能力：规则素材池 / 装配拼接 / 配置生成与分发 / Xray 对接）的多轮、多角度深度研究验证的完成报告，由完成报告画布（`design2-research-completion-report.canvas.tsx`）转档归档。
> **研究依据**：Design2.md（411 行）× [Design1.md](./Design1.md) 基线（926 行）× `backend/` 与 `frontend/` 现有实现 × `docs/` 全量资料（Reference 研究文档、DocTemplates 配置样例、AchievedDocuments 历史文档）。
> **约束遵守**：全程只读取证，未修改任何代码文件、配置文件与 Design 文档（`git status` 验证工作区零改动）。

---

## 一、完成概览

| 指标 | 结果 |
|------|------|
| 递进式研究轮次 | 8 轮（第 0–7 轮，每轮聚焦不同维度） |
| 历史收敛项复核 | **17/17 通过**（B1 阻断级 / G1–G4 缺口 / C1–C8 边界 / D 级运维项全部落文无残留） |
| 代码扩展点验证 | **11/11 吻合** |
| Design1×Design2 兼容性 | **12 项核心机制全部兼容** |
| 全旅程模拟路径 | **6 条全部闭环**，无逻辑断裂 |
| 本轮新发现 | **10 项**（F1–F10：A 级 3 项 / B 级 5 项 / C 级 2 项） |

对 Design2.md 完成了多轮、多角度独立复审：前两轮审核的 17 项问题全部确认落文无残留；与 Design1 基线的 12 项核心机制逐项比对全部兼容；6 条全旅程模拟路径（素材池 → 装配 → 入池 → 分发 → 用户下载 → Xray 同步 → 配额管控 → 换组 → 超限 → 重置 → 开关清空 → 重新编辑）全部闭环。

**总体结论**：设计整体成熟度高，具备进入 Build 阶段的条件；唯一遗留为构建动作项（Go 1.26 升级 + xray-core 依赖引入，B1 决策已定）。建议构建前先将 3 项 A 级口径问题（F1/F2/F3）提交决策并落文。

---

## 二、关键研究步骤

| 轮次 | 维度 | 内容 |
|------|------|------|
| 第 0 轮 | 全景摸底 | Design2 全文 + 历史研究记忆 + 迁移 SQL（0001–1008）schema 事实建立 |
| 第 1 轮 | 功能完整性 | 逐章审读；subs/conf/DailyData 样例 base64 实测比对（vless 空 cipher、STATUS/REMARKS 形态吻合） |
| 第 2 轮 | 兼容性矩阵 | Design1×Design2 十二项核心机制逐项对照：版本管理 / Token 三态 / 级联 / 权限 / 路径分级 / 错误码 / 导入导出等 |
| 第 3 轮 | 代码扩展点 | version.go / download.go / group.go / admin.go / resolve.go / dataclear.go / export.go / config.go / 前端 views 取证 |
| 第 4 轮 | 情景模拟 | 基础模式全链、高级模式全链、换组→超限→重置、开关 OFF 清空、重新编辑容错、并发场景共 6 条路径 |
| 第 5 轮 | 边界排查 | 并发（BEGIN IMMEDIATE / gRPC 串行化 / reset=true 原子 swap）、级联一致性、开关切换清理完整性 |
| 第 6 轮 | 缺口探索 | 产出 F1–F10 分级发现清单与修复建议，汇总研究报告 |
| 第 7 轮 | 证据强化 | Token 首建 / 订阅删除级联 / 导出载荷 / Xray §11 细则交叉补证；git status 验证零修改 |

---

## 三、变更文件

**无任何文件变更**。任务约束为严格只读：`backend/`、`frontend/`、配置文件、Design2.md、Design1.md 均未修改。

- 验证证据：`git status --porcelain` 输出为空（工作区零改动）；
- 全部取证仅使用 Read / Grep / Bash 只读命令。

---

## 四、验证证据摘要

| 验证项 | 证据 | 结论 |
|--------|------|------|
| 版本组件改造点 | version.go L164 事务内强制 `setCurrentLocked`；`ContentProvider` L72-75 预留第三种实现 | activate opt-in 改造点精确吻合 |
| 下载解析改造点 | download.go 无标识分支 JOIN `group_selections`（L96-100）；no-store 头（server/download.go L34） | 5.6 删表改写与 5.10 改造清单吻合 |
| Xray 同步触发器 | admin.go / user.go / approval.go / resolve.go 全部触发函数位置属实；`sendWelcomeIf` 提供「事务后钩子」同构先例 | 5.5 触发器表无遗漏（含 OIDC 合并分支核验） |
| 密钥派生复用 | config.go L220-233 HKDF-SHA256 `deriveKey/Encrypt/Decrypt` | 决策 #16 可直接复用 |
| SR 链接形态 | Shadowrocket.subs.template.md base64 解码 = `:uuid@host:port`（空 cipher）+ `tls=1/xtls=2/peer/pbk/sid` | 与 4.5/5.7 定稿完全吻合 |
| Xray API 细则 | Reference/Xray-Core-API.md §11.1–11.3 源码取证（错误子串匹配 / 空 pattern 禁用 / counter 残留 / alter_id 移除） | 与 5.5/5.8 实现细则逐项对应 |
| Go 版本冲突（B1） | Xray-core/go.mod = go 1.26；项目 go.mod = 1.25.0；Dockerfile = golang:1.25-alpine | 决策已定（升级 1.26），属 Build 动作项 |
| Token 并发首建 | token.go `GetOrCreateUserToken`：BEGIN IMMEDIATE 先查后建 + 3 次冲突重试 | 为 F5 建议提供同构实现参照 |
| 订阅删除级联 | subscription.go `Delete`：版本文件 → Token 回调 → 组关联回调 → 订阅行 | 新模型改造点精确，删全局模板后走空池口径自洽 |
| 配置导出载荷 | export.go `ExportPayload.Config` 仅承载 system_config 键值 | 决策 #24 需扩展独立字段承载 xray_instances，5.10 口径准确；高级开关随 system_config 天然导出 |
| 约束遵守 | `git status --porcelain` 空输出 | 全程零文件修改 |

---

## 五、本轮新发现清单（F1–F10）

> 以下为前两轮审核未覆盖的新发现。无一构成设计推翻；A 级建议构建前落文，B 级可在 Build 文档中明确口径，C 级为低风险备忘。

### 5.1 A 级 · 建议 Design2 落文澄清（3 项）

| 编号 | 问题 | 影响 | 建议方向 |
|------|------|------|----------|
| **F1** | 首次入池自动激活按「平台」判定，与「平台 + 产物类型」模型失配（subs 先激活后 yaml 首入池将永不激活） | Clash/SR 平台首次分发空窗 | 口径细化为「该订阅行 `current_version=0` 时自动激活」 |
| **F2** | 配额 0/NULL 语义未定义（不限流量 or 立即超限？） | 超限判定与 Subscription-Userinfo `total` 输出 | 明确「0/NULL = 不限流量」或「建组必填正整数」 |
| **F3** | 渲染时点用户注入节点集为空 → 强制组「国外流量」空组 → YAML 无效（3.3 硬校验仅覆盖装配时点） | 极端分支下客户端加载失败 | 补渲染层兜底：空强制组降级为仅含 DIRECT |

### 5.2 B 级 · 建议 Build 文档明确口径（5 项）

| 编号 | 问题 | 建议方向 |
|------|------|----------|
| **F4** | dataclear `ClearTablesTx` 硬编码表清单未纳入 9 张新表（5.10 改造清单未列） | 5.10 补 dataclear 表清单扩展 |
| **F5** | 凭据（uuid / proxy_secret）并发首建守卫口径未定义 | 参照 Token 首建：条件更新 + 影响行数判定 |
| **F6** | 存量库新增 `UNIQUE(platform, 产物类型)` 约束的迁移路径未定义（「存量可放弃」缺执行方式） | 迁移脚本先清 subscriptions，或声明升级前先一键清空 |
| **F7** | 决策 #23 注入头与平台附加头同名键（`profile-update-interval`）覆盖优先级未定义 | 明确覆盖优先级，防重复响应头 |
| **F8** | 素材池定时同步错过时刻（容器停机）无补偿语义 | 采用「当日已过 sync_time 且 last_synced_at 早于当日」补偿判定 |

### 5.3 C 级 · 低风险备忘（2 项）

| 编号 | 问题 | 处置 |
|------|------|------|
| **F9** | OIDC 自动合并现行实现仅作用于已激活账号，触发器表无遗漏；若未来恢复「合并激活待审批」语义需补触发点 | Build 文档留备注防回归 |
| **F10** | SR conf 装配目标规则实体下拉为空时的引导文案未描述 | 随 Design1-UI 增量规格补齐 |

---

## 六、最终结果

- ✅ **设计闭环**：全流程模拟（素材池 → 装配 → 入池 → 分发 → 下载 → Xray 同步 → 配额管控 → 重置 → OFF 清空）完全可用、无逻辑断裂；
- ✅ **兼容性**：Design1 基线 12 项核心机制全部兼容；17 项历史审核问题全部闭环无残留；
- ⚠️ **待决策**：F1/F2/F3 三项 A 级口径建议构建前提交用户决策并落文 Design2.md；
- ℹ️ **Build 动作项**：Go 1.26 + golang:1.26-alpine + xray-core 模块引入（B1 决策已定）。

---

## 附录：证据标注约定

| 标注 | 含义 |
|------|------|
| 【文档事实】 | Design 文档原文 |
| 【代码事实】 | 现有代码取证 |
| 【实测】 | 本次研究实际验证（含 base64 解码比对、git 状态核验等） |
| 【推断】/【设想】 | 研究推测，与源码/文档事实区分 |

> 报告生成于第 7 轮证据强化完成后；转档自完成报告画布，归档于 2026-08-16。
