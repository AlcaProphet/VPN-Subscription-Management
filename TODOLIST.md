# TODOLIST.md — 短期待办跟踪（2026-09-14）

> 本文件只保留当前仍需处理的短期事项。已完成的构建、修复、自动化验证和人工验收不在此重复记录。

## 当前待办

### 1. Issue16 R30-01：Build11 邮件相关人工测试

- [ ] 补充重置链接四态（`valid` / `missing` / `used` / `expired`）的具体失败现象、复现步骤和证据。
- [ ] 根据 [Issue16.md](Issue16.md) 的记录继续处理；修复后重新执行对应人工核验。

### 2. R26-07：Xray 节点禁止编辑的错误码

- [ ] 在隔离 Production 或等价真实 API 环境完成一次人工核验，并将结果回写 [ProdTestList.md](ProdTestList.md)。
- [ ] 覆盖 Xray 来源节点禁止编辑的 `403`、manual 节点普通字段校验的 `400`、过期 `base_revision` 的 `409`，以及前端对三类错误的区分；确认均不写入数据。

### 3. Issue17：OIDC 与配置导出遗留问题（核验 R30-05 后确认）

- [ ] R31-01：修复 `signing_key` 原始二进制经 JSON 导出损坏的问题，并补真实随机 key 的 Export→Import→解密往返测试。
- [ ] R31-02：为真实 OIDC 的 discovery/token/JWKS endpoint 增加统一 HTTPS/SSRF 校验；真实 OIDC 登录仅在隔离环境核验。
- [ ] R31-03：确认提供商切换字段语义后，处理“源地址/Client ID + 目标 Secret”混合风险。
- [ ] R31-04：对字面值/不可解密密文增加损坏识别与强制重填路径。
- [ ] R31-06：统一限制 OIDC mock 仅 Dev 可用（Setup/SaveOidc/oidcAvailable/Exchange），并补 prod+mock 回归测试。
- [ ] R31-07：明确「暂未启用」是本地 UI 状态还是持久化停用语义；若为后者，设计停用保留参数与重新启用合同。

## 跟踪规则

1. 未执行项目不得标为通过；人工发现的问题登记到 [Issue16.md](Issue16.md) 或新的当前问题记录。
2. 工程实现、自动化门禁和正式构建结果写入对应的 Issue/Build/Design 文档，本文件只保留待处理入口。
3. 完成事项从本文件移除，不保留逐项历史快照。

## 变更记录

| 日期 | 说明 |
|---|---|
| 2026-09-14 | 清理已完成事项和历史展开，仅保留 Issue16 R30-01、R26-07 两项短期待办。 |
| 2026-09-14 | 核验 R30-05 后新增 Issue17：登记 signing_key 导出损坏、真实 OIDC endpoint HTTPS 缺失及提供商切换/损坏占位符残余边界。 |
| 2026-09-14 | 按用户指令将 Issue16 原 R30-07「暂未启用」未落库问题迁入 Issue17，重编号为 R31-07。 |
| 2026-09-14 | 二轮核验补充 Issue17 R31-06：mock 模式校验不完整，Setup/SaveOidc/Exchange 需统一限制 Dev。 |
