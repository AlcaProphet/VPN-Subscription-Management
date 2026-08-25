# SecurityReport1.md — VPN 订阅管理系统 网络安全静态审计报告（第一期）

> **文档定位：** 本文档记录对本项目的**静态网络安全审计**结果、逐项用户确认结论、处置状态与后续建议。属于存档核查文档，不作为当前强要求；编码强要求仍见 [AGENTS.md](../../AGENTS.md)。
> **审计方式：** 仅静态代码/文档/依赖清单审查，**未构建、未运行项目**。
> **审计基线：** `beta` 分支 HEAD `bad1750`，工作区干净。
> **归档说明：** 本文档于 2026-08-25 新建于 `docs/AchievedDocuments/`，供后续 Build/Issue 承接安全修复时引用。

---

## 一、审计范围与方法

### 1.1 范围

- 后端：`backend/` 全部 Go 源码、迁移 SQL、Dockerfile。
- 前端：`frontend/` 的 Vue/TS 源码、CSP、路由与静态资源策略。
- 部署：`docker-compose.yml`、`docker-compose.yml.example`、README 部署路径。
- 设计：`AGENTS.md`、`docs/AchievedDocuments/Design1.md`、`Design2.md`。
- 已知排除：Xray gRPC 无 TLS/无鉴权问题（按项目设计，由部署者 IP 白名单控制）；部署者自行管理的白名单、内网/回环访问边界。

### 1.2 方法

- 人工静态代码走读：认证、授权、路径处理、文件上传、日志脱敏、下载缓存、并发事务、OIDC、SSRF、装配渲染。
- `npm audit --package-lock-only` 依赖清单静态审计。
- 未执行 `go build`、`go test`、`npm run build`、`npm test` 或启动服务。

### 1.3 总体结论

项目安全基线较好：SQL 基本全参数化、Token 熵达标、敏感配置 AES-256-GCM、下载端点 `no-store`、管理端点双层中间件、路径穿越防护较完整。主要风险集中在 OIDC 身份验证、Setup 阶段资源限制、日志敏感信息、`/public` 同源静态文件与反代信任模型。

---

## 二、逐项确认结论总表

| 编号 | 问题摘要 | 用户确认结论 | 处置状态 |
|------|----------|--------------|----------|
| F01 | OIDC id_token 未验签，未校验 iss/aud/exp/nonce | 确认纳入，按推荐方案修复 | ☐ 待修复（高优先级） |
| F02 | Setup 导入无请求体上限、Setup 端点无速率限制 | 标注为部署边界；README 需提示部署完成前不开放公网访问 | 不纳入代码修复；README 待补充提示 |
| F03 | `/public/installers` 同源输出可执行文件，缺少 nosniff/附件头 | 确认纳入，按推荐方案修复 | ☐ 待修复（高优先级） |
| F04 | 根目录 compose 默认 Dev + 全接口暴露 | 不纳入；属个人测试环境，README 需提示公网/真实数据不得使用 Dev | 不纳入；README 待补充提示 |
| F05 | OIDC 正常流程未强制 HTTPS | 不纳入；属部署者自行处理的安全边界 | 不纳入 |
| F06 | `/api/oidc/test` 在系统已配置但 OIDC 未配置时仍匿名可达，且存在 DNS rebinding 残余 | 确认纳入，按推荐方案修复 | ☐ 待修复（中优先级） |
| F07 | 请求日志泄漏 `/reset/{token}` 与 OIDC `code/state`，脱敏仅覆盖 `?token=` | 确认纳入，按推荐方案修复 | ☐ 待修复（中优先级） |
| F08 | HTTP Server 无读写/空闲超时，无全局请求体限制 | 确认纳入，按推荐方案修复；同时将相关配置纳入系统设置中的速率控制区域，提供默认值与手动调整能力 | ☐ 待修复（含配置化需求） |
| F09 | `TRUST_PROXY=auto` 与 Cloudflare/EdgeOne 不匹配，`on` 可被直连伪造 | 确认纳入，按推荐方案修复 | ☐ 待修复（中优先级） |
| F10 | 平台 scheme 未做协议白名单，可配置 `javascript:` | 不纳入；属部署者自行处理的安全边界，README 提示 | 不纳入；README 待补充提示 |
| F11 | 规则素材池 URL 同步 SSRF 无内网拦截/重定向限制 | 不纳入；内网部署默认为可信，由部署者自行决策 | 不纳入 |
| L01 | OIDC 回调中转页可接受任意 `?token=`，登录 CSRF/账号互换 | 确认纳入，标记为低风险硬化项 | ☐ 待规划 |
| L02 | 配置导入绕过“本地/OIDC 认证死锁”校验 | 确认纳入，标记为低风险硬化项 | ☐ 待规划 |
| L03 | 装配手动规则/头部参数未校验控制字符，可注入 SR conf 行 | 不纳入；属部署者自行处理的范围 | 不纳入 |
| L04 | 反代 TLS 终止时 OIDC state Cookie 未设 `Secure` | 确认纳入，标记为低风险硬化项 | ☐ 待规划 |
| L05 | npm 依赖审计：dev/build 链存在 vitest/vite/nanoid/esbuild 告警 | 确认纳入，标记为低风险/工具链项 | ☐ 待规划 |
| L06 | GitHub Actions 与镜像使用可变标签/未固定 digest | 后续再考虑，只记录，不处理 | 已记录，暂不处理 |
| L07 | 缺少管理操作审计日志 | 纳入，但不处理；作为长期方向 | 已记录，长期方向 |
| L08 | 过期 `password_reset_tokens` 无自动清理 | 确认纳入，标记为低风险硬化项 | ☐ 待规划 |
| L09 | 请求日志未记录客户端 IP，且脱敏规则可被大小写/编码绕过 | 确认纳入，标记为低风险硬化项 | ☐ 待规划 |

---

## 三、已确认待修复问题

### F01【高 · A07/A02】OIDC id_token 未验签，且未校验 iss/aud/exp/nonce

- **现象：**
  - `backend/internal/oidc/flow.go:182-183` 明确注释“不验签”。
  - `backend/internal/oidc/helpers.go:24-32` 的 `decodeJWTPayload` 只 base64 解码 JWT payload，不验证签名。
  - 后续用户查建/自动合并完全信任 payload 中的 `sub`、`email`、`email_verified`。
- **影响：** 一旦 OIDC 启用，任何能控制或中间人 token endpoint 的一方（结合 HTTP 端点问题）可伪造身份，存在管理员账号被接管的前置条件。
- **修复建议：**
  1. 接入标准 OIDC verifier，通过 JWKS 验签。
  2. 校验 `iss`、`aud`、`exp`、`azp`。
  3. 请求携带 `nonce`，与 state 绑定并回验。
  4. 拒绝 `alg=none` 及非对称提供商下的 HS 算法。
- **状态：** ☐ 待修复（高优先级）。

### F03【中高 · A03/A05】`/public/installers` 同源输出可执行文件，形成持久型 XSS 载体

- **现象：**
  - `backend/internal/server/static.go:46-58` 对 `/public/*` 直接 `c.File`，无 `X-Content-Type-Options: nosniff`，无 `Content-Disposition: attachment`。
  - 安装包扩展名不做白名单（Design1 既有决策）。
  - `frontend/src/views/HomeView.vue:160` 用 `window.open(url, '_blank')` 打开。
- **影响：** 管理员上传 HTML/SVG 等文件后，普通用户点击“下载客户端”会在同源新标签页执行脚本，可读取 `localStorage` 中的会话 Token。
- **修复建议：**
  1. `/public/installers/*` 强制 `Content-Disposition: attachment`。
  2. 全局加 `X-Content-Type-Options: nosniff`。
  3. 拒绝危险扩展名（html/svg/js/mjs/xml 等）或改用独立无 Cookie 下载域名。
  4. 前端 `window.open` 加 `noopener`。
- **状态：** ☐ 待修复（高优先级）。

### F06【中 · A01/A05】`/api/oidc/test` 匿名可达与 DNS rebinding 残余

- **现象：**
  - `server/oidc.go:29-40` 以 `oidcSvc.IsConfigured()` 判断是否需登录；该函数只检查 `oidc_configured`，不是系统级 `configured`。
  - 快速开始 Setup 后、OIDC 未配置时，`/api/oidc/test` 仍对公网匿名开放。
  - `validateOIDCURL` 与 `DialContext` 存在两次 DNS 解析，仍可能被 DNS rebinding 绕过。
- **修复建议：**
  1. 已配置系统一律要求会话 + 管理员，只有系统未配置时允许 Setup 匿名测试。
  2. 对匿名测试端点预解析并 pin 住 IP，禁止重定向。
  3. 收敛为仅保留 `/api/admin/settings/oidc/test`。
- **状态：** ☐ 待修复（中优先级）。

### F07【中 · A09】日志泄漏密码重置 Token 与 OIDC `code/state`

- **现象：**
  - `server/server.go:501` 记录完整 `RequestURI`。
  - `log/log.go:62-66` 只脱敏 `?token=`。
  - 前端 `/reset/:token`（`router/index.ts:64`）把密码重置 Token 放入 URL 路径。
  - OIDC callback 的 `?code=...&state=...` 会进入日志。
- **影响：** 有日志读取权限者可拿到重置 Token 并重置他人密码；OIDC code 虽受 PKCE 保护，但仍属敏感凭据。
- **修复建议：**
  1. 日志脱敏增加 `/reset/{token}`、`code=`、`state=`。
  2. 重置链接改用 URL fragment（`/reset#token=...`）。
  3. 脱敏正则大小写不敏感，并覆盖 `%3D` 等编码。
- **状态：** ☐ 待修复（中优先级）。

### F08【中 · A05】HTTP Server 无超时/全局请求体限制（需配置化）

- **现象：**
  - `server/server.go:86` 与 `452` 的 `http.Server` 未设置 `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`。
  - 无全局 `MaxBytesReader` / `MaxBytesHandler`。
- **影响：** 慢头/慢体攻击可长期占用连接；超大请求可被完整读入内存。
- **用户确认：** 确认按推荐方法修复；同时希望将相关配置加入系统设置中的“速率控制”区域，自带默认值并允许手动调整。
- **修复建议：**
  1. 设置 `ReadHeaderTimeout`（建议 5s）、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`。
  2. 对 API 加 `http.MaxBytesReader` 或全局请求体限制。
  3. 在面板“速率限制/安全配置”区域暴露这些值，后端配置键持久化到 `system_config`，默认值与现有设计一致。
- **状态：** ☐ 待修复（含配置化需求）。

### F09【中 · A05】`TRUST_PROXY` 与 Cloudflare/EdgeOne 不匹配

- **现象：**
  - `server.go:481-490`：`auto` 只信任回环 + RFC1918；`on` 信任所有来源。
  - Cloudflare/EdgeOne 回源 IP 通常是公网地址，`auto` 下 `X-Forwarded-For` 不被信任，所有用户共享 Edge IP 的限流桶与日志 IP。
  - `setup.go:165-174` 的 `DeriveFrontendURL` 无条件信任 `X-Forwarded-Proto`。
- **修复建议：**
  1. 增加显式可信代理 CIDR 配置（如 `TRUST_PROXY_CIDRS`），支持 Cloudflare/EdgeOne 回源段。
  2. `X-Forwarded-Proto/Host` 的信任必须与 `trustedForwarded` 同口径。
  3. 生产建议仅绑定 `127.0.0.1`，由反代/WAF 接入。
- **状态：** ☐ 待修复（中优先级）。

---

## 四、已确认低风险硬化项

### L01【低 · A07】OIDC 回调中转页可接受任意 `?token=`，登录 CSRF/账号互换

- **位置：** `frontend/src/views/OidcCallbackView.vue:12-14`、`server/oidc.go:103`。
- **建议：** 改为一次性 ticket + HttpOnly Cookie 换取会话，或增加 nonce 绑定。
- **状态：** ☐ 待规划。

### L02【低 · A08】配置导入绕过“本地/OIDC 认证死锁”校验

- **位置：** `config/export.go:330-336` 直接覆盖全部配置；`config/admin.go:134-180` 的 `ErrAuthDeadlock` 仅覆盖 UI 保存路径。
- **建议：** 导入事务提交前执行同样的认证可用性校验，防止导入文件造成登录锁死。
- **状态：** ☐ 待规划。

### L04【低 · A05】反代 TLS 终止时 OIDC state Cookie 未设 `Secure`

- **位置：** `server/oidc.go:53,152` 仅以 `c.Request.TLS != nil` 判断。
- **建议：** 依据可信 `X-Forwarded-Proto` 或 `frontend_url` 是否为 HTTPS 来决定 `Secure`。
- **状态：** ☐ 待规划。

### L05【低 · A06】npm 依赖审计告警

- **结果：** `npm audit --package-lock-only` 报告：
  - `vitest` critical（dev/test 链）
  - `vite` high（dev/build 链）
  - `nanoid` high（经 postcss 链）
  - `esbuild` moderate（dev/build 链）
  - `@vitest/mocker`、`vite-node` moderate（dev/test 链）
- **说明：** 以上主要属于构建/测试工具链，不进入生产运行时 bundle；但 CI/供应链风险仍建议处理。
- **状态：** ☐ 待规划（升级前端工具链）。

### L06【低 · A08】CI 与镜像未锁定

- **位置：** `.github/workflows/docker-build.yml` 使用 `@v4/v5/v6` 可变标签；README/compose 使用 `:latest`。
- **用户确认：** 后续再考虑，只记录，不处理。
- **状态：** 已记录，暂不处理。

### L07【低 · A09】缺少管理操作审计日志

- **说明：** Design1 已将其列为远期能力；当前仅有下载访问日志。
- **用户确认：** 纳入，但不处理，作为长期方向。
- **状态：** 已记录，长期方向。

### L08【低 · A04】过期 `password_reset_tokens` 无自动清理

- **位置：** `auth/reset.go` 仅用后即删。
- **建议：** 增加每日清理任务，删除过期记录。
- **状态：** ☐ 待规划。

### L09【低 · A09】请求日志无 IP、脱敏规则可被绕过

- **位置：** `server/server.go:501`、`log/log.go:62-66`。
- **建议：** 请求日志补充客户端 IP（注意隐私与脱敏）；脱敏改为大小写不敏感并覆盖编码形态。
- **状态：** ☐ 待规划。

---

## 五、不纳入项与部署边界说明

以下问题经用户确认**不纳入代码修复**，但其中部分需要 README 补充部署提示：

| 编号 | 问题 | 不纳入原因/处置 |
|------|------|------------------|
| F02 | Setup 导入无请求体上限、Setup 端点无速率限制 | 标注为部署边界：Setup 阶段本身设计为不对外直接访问；README 需提示用户必须在部署完成、管理员注册后才开放公网访问。 |
| F04 | 根目录 compose 默认 Dev + 全接口暴露 | 属个人测试环境；README 需提示公网/真实数据部署不得使用 Dev 模式。 |
| F05 | OIDC 正常流程未强制 HTTPS | 属部署者自行处理的安全边界，由反代/HTTPS 配置保证。 |
| F10 | 平台 scheme 未做协议白名单 | 属部署者自行处理的安全边界；README 提示管理员只配置可信客户端 scheme。 |
| F11 | 规则素材池 URL 同步 SSRF | 内网部署默认为可信，由部署者自行决策；当前不安排默认私网拦截。 |
| L03 | 装配手动规则/头部参数未校验控制字符 | 属部署者自行处理的范围，暂不安排校验。 |

---

## 六、AGENTS.md / Design 符合性对照

| AGENTS 强制项 | 结论 |
|---------------|------|
| 路径穿越防护 | 基本达标：`/public`、安装包文件名、版本文件均有校验 |
| 密钥加密存储 | 达标：AES-256-GCM；签名密钥明文落库为设计取舍 |
| Token 日志脱敏 | 部分不达标：`?token=` 已脱敏，但 `/reset/:token`、`code/state` 未处理 |
| 实时权限校验 | 达标：会话中间件实时查库 + `credential_version` 比对，管理端点双中间件 |
| 下载防缓存 | 达标：订阅/规则/分享均 `no-store` |
| 上传大小双重校验 | 不达标：配置导入后端无上限、前端无大小校验（F02 已按部署边界记录） |
| 5xx 脱敏 | 达标：默认通用错误，debug_mode 才回显详情 |
| 无效 Token 统一 404 | 达标 |
| OIDC 认证安全 | 不达标：id_token 未验签、无 nonce/iss/aud/exp（F01） |

---

## 七、后续动作建议

1. **README 补充**：
   - 公网部署前完成 Setup 并注册管理员，再开放公网访问。
   - 生产禁止使用根目录 dev compose / Dev 模式承载真实数据。
   - 提示管理员只配置可信客户端 scheme。
2. **Build/Issue 承接**：
   - 将 F01、F03、F06、F07、F08、F09 按优先级转入后续 Build 步骤。
   - 将 L01、L02、L04、L05、L08、L09 作为低风险硬化候选记录到 Issue 或 Design 后续候选。
3. **后续复审**：
   - 修复完成后应补做编译、单测、前端构建与回归。
   - 建议后续引入 Go 依赖漏洞扫描（如 `govulncheck`）与前端 `npm audit` 纳入 CI。

---

## 八、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-25 | 初始版本：完成第一期静态网络安全审计，逐项与用户确认，记录已确认待修复项、低风险硬化项、不纳入/部署边界项。 |
