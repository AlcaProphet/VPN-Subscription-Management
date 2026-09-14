# Issue17.md — R30-05 核验后的 OIDC 遗留问题

> **文档定位：** 本文件为核验 Issue16 R30-05 修复后新开的独立问题记录。R30-05 的三个阻塞项已在本轮修复；本文件跟踪未纳入该批次、但在核验中确认的真实 OIDC 网络边界问题与残余边界。原 R31-01 由 [ExportRelated1.md](ExportRelated1.md) 独立跟踪。
> 关联：[Issue16.md](docs/reports/Issue/Issue16.md)、[ExportRelated1.md](ExportRelated1.md)、[AGENTS.md](AGENTS.md)。

---

## 一、R31-02 真实 OIDC discovery/token/JWKS endpoint 未统一 HTTPS 校验（高）

- **现象与根因：** `fetchDiscovery` 直接请求由 `base_url` 拼接的发现文档地址，未调用 `validateOIDCURL`；发现文档返回的 `token_endpoint` 在真实 `Exchange` 中直接使用。`SaveOidc` / Setup 也不强制 `base_url` 为 HTTPS。
- **影响范围：** 即使入口地址为 HTTPS，发现文档仍可声明 `http://` token/JWKS 地址；在本次 Secret 解密修复后，token 请求会携带明文 Client Secret，存在明文传输风险。发现文档中的授权地址也直接交给浏览器，需一并校验。
- **核查补充：** 测试连接路径已对发现文档地址和 token endpoint 调用 `validateOIDCURL`，但真实 `StartFlow` / `Exchange` 共用的 `fetchDiscovery` 未校验，`getJWKS` 也未校验。`CheckRedirect` 只覆盖后续重定向，不覆盖初始请求；当前 `ProxyFromEnvironment` 还需核对代理转发时公网 IP 检查是否仍作用于目标地址。
- **拟议修复方案：** 在真实流程所有初始出站请求前复用统一的 HTTPS/语法校验；解析发现文档后在缓存前校验授权、token、JWKS 地址，且在实际请求处再次守卫；保持每次重定向的 HTTPS 校验。建议 OIDC 专用客户端禁用 `ProxyFromEnvironment`，使现有拨号时 DNS/公网 IP 检查实际作用于目标主机；如部署确需代理，应另行设计可验证的目标地址限制。对 `base_url` 在 Setup/保存时提前拒绝不合规输入。用隔离 mock/本地受控 HTTP 客户端覆盖 HTTP 初始地址、HTTPS 发现文档夹带 HTTP endpoint、重定向降级、缓存命中与代理边界；真实 OIDC 登录仅在隔离环境核验。
- **状态：** ☐ 待单独授权修复。

## 二、R31-03 提供商切换仍可能混合非 Secret 字段（中）

- **现象与根因：** 设置页切换提供商时只清空 Secret，保留源提供商的 `base_url` / `client_id`；保存目标提供商时，后端使用表单字段覆盖目标参数、仅在 Secret 为空时保留目标原 Secret，可能形成“源地址/Client ID + 目标 Secret”的混合配置。
- **影响范围：** 切换提供商后若未逐字段重填，目标提供商配置可能不可用；本地登录关闭时存在进一步锁死风险。
- **待决策：** 切换时加载目标提供商已保存字段，还是要求全部显式重填；需要产品口径确认后再实现。
- **状态：** ☐ 待确认设计后处理。

## 三、R31-04 字面值/不可解密占位符未强制重填（中）

- **现象与根因：** 当前损坏判定依赖“能解密且等于 `***`”。如果库内 `client_secret` 是字面 `"***"` 或无法解密的密文，`LoadParams` 报错会跳过损坏检测；GET 还会丢失原 BaseURL/ClientID 回显，空值保存可能把该值原样写回。
- **影响范围：** 非典型损坏状态无法按 R30-05 的“判未配置并强制重填”处理；TestConnection 只会落到“未提供 Client Secret”的警告分支。
- **修复方向：** 区分“参数不存在”和“参数不可读”；对原始参数存在但无法解密的情况返回明确损坏状态，要求管理员重新输入 Secret。
- **状态：** ☐ 待单独处理。

## 四、R31-05 同轮低风险观察

1. `ValidateImportedAuthUsable` 未校验 `oidc_configured`；极端导入文件可能在 `configured=true`、本地登录关闭的情况下让登录页隐藏 OIDC，形成 UI 登录死锁。
2. `GetOidc` 在 Secret 解密失败时直接提前返回，不保留 `base_url` / `client_id` 回显，管理员需要全部重填。
3. `frontend_url` / `callback_url` 非空时，OIDC 每次保存都会返回 `need_restart=true`，未比较字段是否实际变化。

## 五、R31-06 OIDC mock 模式校验不完整（高，既有问题）

- **现象与根因：** Setup / `SaveOidc` 接受 `provider_type=mock` 未检查运行模式；`flow.Exchange` 的 mock 分支也不检查 `s.mode`，只有 `MockLogin` 检查 Dev。生产实例若存在 mock 配置，登录发起/回调仍可能走 mock 解析路径。
- **影响范围：** 生产模式模拟登录绕过；与 `SaveLocalAuth` 的 mock 可用性判定（本轮已限制 Dev）存在配置状态不一致。
- **修复方向：** 在 Setup、`SaveOidc`、`oidcAvailable`、`flow.Exchange` 四处统一限制 mock 仅 Dev 可用；补 prod + mock 的接口与登录回归测试。
- **状态：** ☐ 待单独授权修复。

## 六、R31-07 OIDC「暂未启用」未真正落库停用（中）

- **现象与根因：** 设置页把「暂未启用」映射为 `provider_type=''`，但 `onProviderChange` 只改本地表单并折叠参数区，不调用任何保存接口；`SaveOidc` 的 `validProviders` 也不接受空 provider_type。因此选择「暂未启用」不会真正停用 OIDC，离开页面后后端仍按旧提供商工作，仅形成本地未保存状态。
- **影响范围：** OIDC 生命周期语义与“停用后重启用”的人工核验；不影响 R30-05 的 Secret 空回显、保留/替换与占位符拒绝合同。
- **修复方向：** 明确「暂未启用」是本地 UI 状态还是持久化停用语义；若为后者，需要设计停用时保留各提供商参数、重新启用后续用的合同，并同步前端保存/确认流程。
- **状态：** ☐ 待单独决策；代码未修改，留待独立授权后处理。

## 七、处理建议

1. R31-01 由 [ExportRelated1.md](ExportRelated1.md) 独立跟踪；R31-02 仍在本文件独立跟踪，修复授权不与 R30-05 混批。
2. R31-03 先与用户确认提供商切换字段语义，再决定是否需要新增按提供商读取接口。
3. R31-04/R31-05/R31-06/R31-07 可随对应 OIDC/导入专项一起处理。

---

## 变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-14 | 核验 Issue16 R30-05 后创建，登记 signing_key 导出损坏、真实 OIDC endpoint HTTPS 缺失及提供商切换/损坏占位符残余边界。 |
| v1.1 | 2026-09-14 | 二轮核验补充 R31-06：mock 模式校验不完整（Setup/SaveOidc/Exchange 未统一限制 Dev）。 |
| v1.2 | 2026-09-14 | 按用户指令将 Issue16 原 R30-07「暂未启用」未落库问题迁入本文件，重编号为 R31-07。 |
| v1.3 | 2026-09-14 | 将 R31-01 分至 ExportRelated1 独立跟踪；补充 R31-02 真实流程与测试连接差异、代理边界及拟议修复方案。 |
