# ExportRelated1.md — 配置导出与迁移问题跟踪

> **文档定位**　跟踪配置导出和导入问题，以及从 Design5 迁入的整站加密迁移候选。Design5 保留历史设计与验收记录。R31-01 自已归档的 Issue17 分出，继续独立处理。
> **当前状态**　R31-01 待确认修复方案。整站加密导出与 Setup 一步迁移待研究和用户决策。Design5 §七的邮件内容定制已验收，其配置导入导出行为作为 R31-01 回归基线。
> **关联文档**　[AGENTS.md](AGENTS.md)、[Design5.md](docs/reports/Design/Design5.md)、[TODOLIST.md](TODOLIST.md)、[Build28.md](docs/reports/Build/Build28.md)、[Issue17.md](docs/reports/Issue/Issue17.md)。

---

## 一、进行中问题

### R31-01 signing_key 经 JSON 导出损坏（高）

- **现象与根因：** `EnsureSigningKey` / `EnsureSigningKeyTx` 生成 32 字节随机密钥后以原始 `string` 落库；配置导出把整个 `system_config` map 直接 `json.Marshal`。Go 的 `encoding/json` 会把非法 UTF-8 字节替换为 `U+FFFD`，导致随机二进制 `signing_key` 在导出/导入往返后损坏。
- **影响范围：** 配置导出/导入后的 OIDC Client Secret、SMTP 密码、节点/独立账号密文等无法解密；导入时的签名密钥保护与 `ValidateImportedAuthUsable` 也会因此失效。
- **核验证据：** 隔离测试用包含非法 UTF-8 的随机 key 做 JSON 往返，输入 32 字节与输出不一致，已确认损坏。
- **待确认方案**　比较 Base64 与 hex 编码，明确落库及导出编码、旧库读取兼容和已损坏文件的处理方式。[TODOLIST.md](TODOLIST.md#p0-配置导出完整性独立专项)
- **回归边界**　覆盖邮件模板配置往返及已有加密渲染计划的同密钥导入限制。Build28 记录模板键往返结果，随机密钥和各类密文解密须另行核验。[Design5 §7.6](docs/reports/Design/Design5.md#76-实施状态2026-09-15) · [Design4 §7.3](docs/reports/Design/Design4.md#73-历史版本与-xray-来源)
- **状态**　待用户确认修复方案，后续独立实施和验收。

## 二、整站加密迁移候选

- **目标与状态**　把完整站点数据封装成单个加密文件，在未配置的 Production 目标站通过 Setup 一步恢复。现有 `.enc` 覆盖配置，`tar.gz` 整库备份需手工恢复。候选仍待研究及决策，未建立对应 Build。[Design5 §一](docs/reports/Design/Design5.md#一背景与现状差距)
- **已确认边界**　导出与恢复仅限 Production。目标站在替换数据前拒绝非 Production 来源及任何 mock 配置，含存在 `oidc_params_mock` 键的文件。来源模式以新格式元数据表达，验证方式待随格式确定。[Design5 §1.3](docs/reports/Design/Design5.md#13-已确认目标与非结论)
- **待研究与决策**　先审计 schema、快照、内容目录和外部依赖。比较独立加密备份与扩展 `.enc` 的路线，再决定格式和版本兼容、恢复入口是否允许覆盖已配置站点、加密与完整性验证、数据库与文件的一致性及原子回滚、旧站切换和 OIDC／SMTP／Xray 等外部依赖的恢复方式。[Design5 §三](docs/reports/Design/Design5.md#三候选方案待研究与用户决策) · [Design5 §四](docs/reports/Design/Design5.md#四设计前必须回答的问题)
- **候选验收**　验证异常文件在写入前被拒绝、失败完整回滚。成功后核对原账号登录和角色归属、外键、内容哈希、当前版本指针及换域后的外部配置。[Design5 §五](docs/reports/Design/Design5.md#五候选验收边界)

### 现有配置导入的后续人工复验

按 Design5 §1.2 登记，随整站迁移研究复核现有 Setup 与管理端的确认词、20 MiB 文件边界和错误提示。Setup 新库使用 `IMPORT`，管理端对应的覆盖导入保留 `IMPORT → DISABLE`。R29-01 已完成人工核验并闭环，文件上限亦有 Build26 工程证据。保留这一后续复验范围，沿用现有结案状态。[Design5 §1.2](docs/reports/Design/Design5.md#12-当前能力边界) · [Issue15 R29-01](docs/reports/Issue/Issue15.md#r29-01-setup-新库导入被错误要求同时输入-import-和-disable) · [Build26](docs/reports/Build/Build26.md)

## 三、后续处理边界

1. R31-01 的工程验收须包含真实随机密钥的 `Export → Import → 解密` 隔离端到端往返，并核对不同密文类型及既有损坏导出文件的处理方式。
2. 工程自动化、隔离环境核验与用户人工验收分别记录，不以其中一类证据代替其他验收层级。
3. Design5 §一～§六保留候选状态；若后续实施整站迁移，应先完成设计决策并建立对应构建计划。

## 四、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-14 | 按用户更正从 XrayRelated1 撤回 R31-01，建立配置导出专项跟踪，并关联 Design5。 |
| v1.1 | 2026-11-24 | 登记 Design5 的整站加密迁移候选和现有配置导入人工复验范围，补充 R31-01 的兼容及回归边界。 |
