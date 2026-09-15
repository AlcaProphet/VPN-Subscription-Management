# ExportRelated1.md — 配置导出与迁移问题跟踪

> **文档定位：** 集中跟踪配置导出、导入与迁移相关的当前问题。R31-01 从 [Issue17.md](Issue17.md) 分出，由本文件独立跟踪。
> **设计关联：** [Design5.md](Design5.md) §一～§六是整站加密导出与 Setup 一步迁移的候选设计，尚未定稿或实施；§七的邮件模板导入导出边界也须核对 R31-01。本文件只记录已发现缺陷，不代表 Design5 §一～§六已获实施授权。
> **关联文档：** [AGENTS.md](AGENTS.md)（唯一强要求）、[TODOLIST.md](TODOLIST.md)、[Build28.md](docs/reports/Build/Build28.md)、[Issue17.md](Issue17.md)。

---

## 一、进行中问题

### R31-01 signing_key 经 JSON 导出损坏（高）

- **现象与根因：** `EnsureSigningKey` / `EnsureSigningKeyTx` 生成 32 字节随机密钥后以原始 `string` 落库；配置导出把整个 `system_config` map 直接 `json.Marshal`。Go 的 `encoding/json` 会把非法 UTF-8 字节替换为 `U+FFFD`，导致随机二进制 `signing_key` 在导出/导入往返后损坏。
- **影响范围：** 配置导出/导入后的 OIDC Client Secret、SMTP 密码、节点/独立账号密文等无法解密；导入时的签名密钥保护与 `ValidateImportedAuthUsable` 也会因此失效。
- **核验证据：** 隔离测试用包含非法 UTF-8 的随机 key 做 JSON 往返，输入 32 字节与输出不一致，已确认损坏。
- **修复方向：** 签名密钥落库/导出改为 base64 或 hex 编码；补充真实随机 key 的完整 `Export → Import → 解密` 往返测试；评估既有已损坏导出文件的处理方式。
- **设计与构建边界：** 修复时核对 [Design5.md](Design5.md) §一～§六的候选迁移语义，以及 §七邮件模板覆盖的导入导出要求；不能用 Build28 的受控模板键往返替代真实随机密钥往返证据。
- **状态：** ☐ 待后续独立修复与验收；本次仅调整跟踪文档。

## 二、后续处理边界

1. R31-01 的工程验收须包含真实随机密钥的 `Export → Import → 解密` 隔离端到端往返，并核对不同密文类型及既有损坏导出文件的处理方式。
2. 工程自动化、隔离环境核验与用户人工验收分别记录，不以其中一类证据代替其他验收层级。
3. Design5 §一～§六保留候选状态；若后续实施整站迁移，应先完成设计决策并建立对应构建计划。

## 三、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-14 | 按用户更正从 XrayRelated1 撤回 R31-01，建立配置导出专项跟踪，并关联 Design5。 |
