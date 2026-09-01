# SecurityReport1.md — VPN 订阅管理系统 网络安全静态审计报告（第一期）

> **文档定位：** 本文档记录对本项目的**静态网络安全审计**结果、逐项用户确认结论、处置状态与后续建议。属于存档核查文档，不作为当前强要求；编码强要求仍见 [AGENTS.md](../../../AGENTS.md)。
> **审计方式：** 仅静态代码/文档/依赖清单审查，**未构建、未运行项目**。
> **审计基线：** `beta` 分支 HEAD `bad1750`，工作区干净。
> **归档说明：** 本文档于 2026-08-25 新建于 `docs/reports/`，供后续 Build/Issue 承接安全修复时引用。

---

## 一、审计范围与方法

### 1.1 范围

- 后端：`backend/` 全部 Go 源码、迁移 SQL、Dockerfile。
- 前端：`frontend/` 的 Vue/TS 源码、CSP、路由与静态资源策略。
- 部署：`docker-compose.yml`、`docker-compose.yml.example`、README 部署路径。
- 设计：`AGENTS.md`、`docs/reports/Design/Design1.md`、`Design2.md`。
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
| F01 | OIDC id_token 未验签，未校验 iss/aud/exp/nonce | 确认纳入，按推荐方案修复 | ✅ 已修复（见 §8.18） |
| F02 | Setup 导入无请求体上限、Setup 端点无速率限制 | 标注为部署边界；README 需提示部署完成前不开放公网访问 | ✅ README 已补充部署边界（见 §8.13） |
| F03 | `/public/installers` 同源输出可执行文件，缺少 nosniff/附件头 | 确认纳入，按推荐方案修复 | ✅ 已修复（见 §8.18） |
| F04 | 根目录 compose 默认 Dev + 全接口暴露 | 不纳入；属个人测试环境，README 需提示公网/真实数据不得使用 Dev | ✅ README 已补充部署边界（见 §8.13） |
| F05 | OIDC 正常流程未强制 HTTPS | 不纳入；属部署者自行处理的安全边界 | ✅ 已记录，维持部署边界（README 已提示） |
| F06 | `/api/oidc/test` 在系统已配置但 OIDC 未配置时仍匿名可达，且存在 DNS rebinding 残余 | 确认纳入，按推荐方案修复 | ✅ 已修复（见 §8.18） |
| F07 | 请求日志泄漏 `/reset/{token}` 与 OIDC `code/state`，脱敏仅覆盖 `?token=` | 确认纳入，按推荐方案修复 | ✅ 已修复（见 §8.18） |
| F08 | HTTP Server 无读写/空闲超时，无全局请求体限制 | 确认纳入，按推荐方案修复；同时将相关配置纳入系统设置中的速率控制区域，提供默认值与手动调整能力 | ✅ 已修复（见 §8.18） |
| F09 | `TRUST_PROXY=auto` 与 Cloudflare/EdgeOne 不匹配，`on` 可被直连伪造 | 确认纳入，按推荐方案修复 | ✅ 已修复（见 §8.18） |
| F10 | 平台 scheme 未做协议白名单，可配置 `javascript:` | 不纳入；属部署者自行处理的安全边界，README 提示 | ✅ README 已补充部署边界（见 §8.13） |
| F11 | 规则素材池 URL 同步 SSRF 无内网拦截/重定向限制 | 不纳入；内网部署默认为可信，由部署者自行决策 | ✅ 已记录，维持部署边界（README 已提示） |
| L01 | OIDC 回调中转页可接受任意 `?token=`，登录 CSRF/账号互换 | 确认纳入，标记为低风险硬化项 | ✅ 已修复（见 §8.18） |
| L02 | 配置导入绕过“本地/OIDC 认证死锁”校验 | 确认纳入，标记为低风险硬化项 | ✅ 已修复（见 §8.18） |
| L03 | 装配手动规则/头部参数未校验控制字符，可注入 SR conf 行 | 不纳入；属部署者自行处理的范围 | ✅ 已记录，维持部署边界 |
| L04 | 反代 TLS 终止时 OIDC state Cookie 未设 `Secure` | 确认纳入，标记为低风险硬化项 | ✅ 已修复（见 §8.18） |
| L05 | npm 依赖审计：dev/build 链存在 vitest/vite/nanoid/esbuild 告警 | 确认纳入，标记为低风险/工具链项 | ✅ 已修复（见 §8.18） |
| L06 | GitHub Actions 与镜像使用可变标签/未固定 digest | 后续再考虑，只记录，不处理 | ✅ 已记录，暂不处理 |
| L07 | 缺少管理操作审计日志 | 纳入，但不处理；作为长期方向 | ✅ 已记录，长期方向 |
| L08 | 过期 `password_reset_tokens` 无自动清理 | 确认纳入，标记为低风险硬化项 | ✅ 已修复（见 §8.18） |
| L09 | 请求日志未记录客户端 IP，且脱敏规则可被大小写/编码绕过 | 确认纳入，标记为低风险硬化项 | ✅ 已修复（见 §8.18） |

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
- **状态：** ✅ 已修复（见 §8.18）。

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
- **状态：** ✅ 已修复（见 §8.18）。

### F06【中 · A01/A05】`/api/oidc/test` 匿名可达与 DNS rebinding 残余

- **现象：**
  - `server/oidc.go:29-40` 以 `oidcSvc.IsConfigured()` 判断是否需登录；该函数只检查 `oidc_configured`，不是系统级 `configured`。
  - 快速开始 Setup 后、OIDC 未配置时，`/api/oidc/test` 仍对公网匿名开放。
  - `validateOIDCURL` 与 `DialContext` 存在两次 DNS 解析，仍可能被 DNS rebinding 绕过。
- **修复建议：**
  1. 已配置系统一律要求会话 + 管理员，只有系统未配置时允许 Setup 匿名测试。
  2. 对匿名测试端点预解析并 pin 住 IP，禁止重定向。
  3. 收敛为仅保留 `/api/admin/settings/oidc/test`。
- **状态：** ✅ 已修复（见 §8.18）。

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
- **状态：** ✅ 已修复（见 §8.18）。

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
- **状态：** ✅ 已修复（见 §8.18）。

### F09【中 · A05】`TRUST_PROXY` 与 Cloudflare/EdgeOne 不匹配

- **现象：**
  - `server.go:481-490`：`auto` 只信任回环 + RFC1918；`on` 信任所有来源。
  - Cloudflare/EdgeOne 回源 IP 通常是公网地址，`auto` 下 `X-Forwarded-For` 不被信任，所有用户共享 Edge IP 的限流桶与日志 IP。
  - `setup.go:165-174` 的 `DeriveFrontendURL` 无条件信任 `X-Forwarded-Proto`。
- **修复建议：**
  1. 增加显式可信代理 CIDR 配置（如 `TRUST_PROXY_CIDRS`），支持 Cloudflare/EdgeOne 回源段。
  2. `X-Forwarded-Proto/Host` 的信任必须与 `trustedForwarded` 同口径。
  3. 生产建议仅绑定 `127.0.0.1`，由反代/WAF 接入。
- **状态：** ✅ 已修复（见 §8.18）。

---

## 四、已确认低风险硬化项

### L01【低 · A07】OIDC 回调中转页可接受任意 `?token=`，登录 CSRF/账号互换

- **位置：** `frontend/src/views/OidcCallbackView.vue:12-14`、`server/oidc.go:103`。
- **建议：** 改为一次性 ticket + HttpOnly Cookie 换取会话，或增加 nonce 绑定。
- **状态：** ✅ 已修复（见 §8.18）。

### L02【低 · A08】配置导入绕过“本地/OIDC 认证死锁”校验

- **位置：** `config/export.go:330-336` 直接覆盖全部配置；`config/admin.go:134-180` 的 `ErrAuthDeadlock` 仅覆盖 UI 保存路径。
- **建议：** 导入事务提交前执行同样的认证可用性校验，防止导入文件造成登录锁死。
- **状态：** ✅ 已修复（见 §8.18）。

### L04【低 · A05】反代 TLS 终止时 OIDC state Cookie 未设 `Secure`

- **位置：** `server/oidc.go:53,152` 仅以 `c.Request.TLS != nil` 判断。
- **建议：** 依据可信 `X-Forwarded-Proto` 或 `frontend_url` 是否为 HTTPS 来决定 `Secure`。
- **状态：** ✅ 已修复（见 §8.18）。

### L05【低 · A06】npm 依赖审计告警

- **结果：** `npm audit --package-lock-only` 报告：
  - `vitest` critical（dev/test 链）
  - `vite` high（dev/build 链）
  - `nanoid` high（经 postcss 链）
  - `esbuild` moderate（dev/build 链）
  - `@vitest/mocker`、`vite-node` moderate（dev/test 链）
- **说明：** 以上主要属于构建/测试工具链，不进入生产运行时 bundle；但 CI/供应链风险仍建议处理。
- **状态：** ✅ 已修复（见 §8.18）。

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
- **状态：** ✅ 已修复（见 §8.18）。

### L09【低 · A09】请求日志无 IP、脱敏规则可被绕过

- **位置：** `server/server.go:501`、`log/log.go:62-66`。
- **建议：** 请求日志补充客户端 IP（注意隐私与脱敏）；脱敏改为大小写不敏感并覆盖编码形态。
- **状态：** ✅ 已修复（见 §8.18）。

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
| Token 日志脱敏 | 达标：`/reset/{token}`、`code/state`、大小写/编码变体均已脱敏；重置链接已改为 fragment |
| 实时权限校验 | 达标：会话中间件实时查库 + `credential_version` 比对，管理端点双中间件 |
| 下载防缓存 | 达标：订阅/规则/分享均 `no-store` |
| 上传大小双重校验 | 按部署边界处理：配置导入由反代/部署层控制（F02），代码侧未新增全局上限；已在 README 明确说明 |
| 5xx 脱敏 | 达标：默认通用错误，debug_mode 才回显详情 |
| 无效 Token 统一 404 | 达标 |
| OIDC 认证安全 | 达标：JWKS 验签 + iss/aud/exp/nonce/azp + 非对称算法白名单 |

---

## 七、后续动作建议（已闭环）

1. **README 补充**：✅ 已完成。公网部署前完成 Setup 并注册管理员再开放公网访问、生产禁止使用根目录 dev compose / Dev 模式、平台 scheme 只配置可信客户端等提示均已写入 README。
2. **Build/Issue 承接**：✅ 已完成。F01/F03/F06/F07/F08/F09 与 L01/L02/L04/L05/L08/L09 均已按 §8.18 落地并验证。
3. **后续复审**：✅ 已执行后端 build/vet/test、前端 build/test、`npm audit --audit-level=high` 与 Docker 构建。后续可继续关注 Go 依赖漏洞扫描（如 `govulncheck`）与依赖更新策略。

---

## 八、修复方案研究（SecurityReport1 承接计划，已与用户逐项确认）

> **研究基线：** 当前 `beta` HEAD `7ae7b34`（相对审计基线 `bad1750` 仅新增本报告文档，代码未再改动；下文引用均已按当前工作区重定位）。
> **本部分性质：** 只形成修复方案与影响评估，**未修改任何代码**。每项包含「推荐方案 / 架构与副作用 / 测试点 / 已确认结论」；全部决策已与用户逐项确认并记录于 §8.15。
> **执行顺序建议：** P0 = F01、F03、F06、F07、F08、F09；P1 = L01、L02、L04、L08、L09；P2 = L05 与 README 补充。F03 的持久型 XSS 修复应先于或与 L01 同批完成（L01 仍把会话凭据存于 `localStorage`，同源脚本执行风险必须先关闭）。

### 8.0 全局影响评估（所有方案共同遵守）

1. **数据库迁移只做加法**：新增迁移文件，不改写历史迁移；`ALTER TABLE ... ADD COLUMN` 必须给出兼容旧数据的默认值；服务启动仍由既有迁移框架自动执行。
2. **不改变部署形态与零配置启动**：新增内容属于安全默认值或运维环境变量（`TRUST_PROXY_CIDRS`），不引入 .env 业务配置；面板可调参数一律存 `system_config`。
3. **不改变认证模型**：会话凭据仍为无服务端状态的 JWT + 实时查库；不引入服务端会话表（L01 只增加一次性“换票”记录，凭据本体不落库）。
4. **前端产物同步**：凡改 `frontend/`，必须重新执行 `npm run build`，并把产物同步至 `backend/web/dist`（本地 `go run`/测试嵌入的是后者），容器构建路径本身会重新生成。
5. **验证命令**（每个 Build Step 验收时执行）：
   - `cd backend && go build ./... && go vet ./... && go test ./...`
   - `cd frontend && npm run build && npm test`
6. **文档同步**：修复落地后在当前 Build/Issue 文档闭环；SecurityReport1 仅作安全项追踪。若修复结论与已归档 Design1 冲突，按 AGENTS.md 规则提请用户决策，不擅自改写归档设计。

---

### 8.1 F01【高】OIDC id_token 验签 + iss/aud/exp/nonce/azp 校验

**当前代码：** `backend/internal/oidc/flow.go:182-183` 仍不验签；`helpers.go:24-32` 仅解码 payload；`oidc.go:43-50` 的 `Discovery` 已预留 `jwks_uri`/`issuer` 字段但未用于校验。

**推荐方案 A（推荐）：接入标准 OIDC verifier，保留现有发现文档与 token 交换流程**

1. 新增依赖 `github.com/coreos/go-oidc/v3`（若拉取受限则可用仓库现有 `go-oidc` 兼容版本，但优先最新稳定 v3；实现时以 `go mod tidy` 实际解析为准）。
2. 继续沿用当前 `fetchDiscovery` 取得的 `issuer` 与 `jwks_uri`，用 `oidc.NewRemoteKeySet` + `oidc.NewVerifier(issuer, keySet, cfg)` 构造 verifier；`RemoteKeySet` 会自行处理 JWKS 缓存与轮换。**推荐用服务级 context 创建并按 issuer 缓存 verifier**，避免每次登录都拉取 JWKS；若实现时不缓存，则每次 `Exchange` 用请求 context 创建并随请求结束丢弃。
3. `Config` 必须设置：
   - `ClientID: p.ClientID`（启用 aud 校验）；
   - `SkipIssuerCheck: false`、`SkipExpiryCheck: false`（默认即可，显式写出防误改）；
   - `SupportedSigningAlgs` 显式限定为非对称算法白名单（至少 RS256/384/512、ES256/384/512，视 go-oidc 版本补充 PS/EdDSA），**不包含 HS 系列**，从而拒绝 `alg=none` 与 HS 混淆攻击。
4. 在 `Verify` 通过后追加两项业务校验：
   - `idToken.Nonce == rec.Nonce`（go-oidc 只解析 nonce，不负责比对）；
   - 从 raw claims 读取 `azp`，存在则必须等于 `p.ClientID`。
5. **nonce 落库与回传**：
   - 新增迁移 `1012_oidc_nonce.sql`：`ALTER TABLE oidc_states ADD COLUMN nonce TEXT NOT NULL DEFAULT ''`；
   - `StartFlow` 生成 32 字节随机 nonce（与 state 同强度），`saveState` 一并写入，授权 URL 增加 `nonce=<base64url>`；
   - `StateRecord` 增加 `Nonce`；`ConsumeState` 一并读回。
6. `Exchange` 中：`providerType == "mock"` 继续走 `mockExchange`，不经过真实验签；真实流程用 `idToken.Claims(&claims)` 或继续解析已验签 payload 提取 `sub/email/email_verified/preferred_username/name/realm_access.roles/groups`，**删除** `decodeJWTPayload` 直接信任逻辑。
7. 错误语义保持现有回调映射：验签失败 / nonce 不匹配 / iss、aud、azp、exp 失败统一映射到 `/login?oidc_error=exchange_failed`，不向回调方回显细节（细节记 warn 日志）。

**架构与副作用：**

- 迁移为加法，不影响既有业务数据；**升级瞬间已发出的旧授权流**（库内 `nonce=''`）会在回调时被拒，用户重新点击登录即可。若不能接受一次性中断，可给 10 分钟兼容窗口，但会保留无 nonce 旧记录可被接受的口子，不推荐。
- 新增依赖会增大二进制体积与 `go.sum`；纯 Go、无 CGO，符合静态编译约束。
- 现有 `discCache` 与 verifier 缓存需共用同一把锁/同一生命周期，`ClearDiscCache` 时同步失效，避免配置切换后仍用旧 JWKS。
- 不影响用户查建/自动合并/白名单逻辑；`Identity.RawClaims` 改为“验签后的 payload 原文”，审批页展示语义不变。

**测试点：**

- 用固定 JWKS（RS256/ES256）生成合法 id_token 的单测：合法通过、错误签名拒绝、`alg=none` 拒绝、HS256 拒绝、iss/aud/exp/nonce/azp 逐项不匹配拒绝、JWKS kid 轮换后新 token 通过。
- mock 模式现有测试保持全绿；补 `StartFlow` 授权 URL 含 nonce、state 记录含 nonce 的断言。

**已确认：** D-F01-1=只引入 go-oidc `RemoteKeySet + NewVerifier`，保留现有发现/交换流程；D-F01-2=严格校验 nonce，不设兼容窗口；D-F01-3=非对称算法白名单 + azp 严格等于 client_id。

---

### 8.2 F03【高】`/public/installers` 附件下载 + 全局 nosniff + 前端 noopener

**当前代码：** `backend/internal/server/static.go:46-58` 对 `/public/*` 统一 `c.File`；`frontend/src/views/HomeView.vue:160` 为 `window.open(url, '_blank')`。

**推荐方案：**

1. `registerStatic` 中增加 `/public/installers/*` 分支：
   - 强制 `Content-Disposition: attachment`；文件名取磁盘基本名（当前生成规则为 `installer-<unixnano>.<sanitized ext>`，本就是安全 ASCII），建议用 `mime.FormatMediaType` 生成 `attachment; filename=...`，避免手工拼接头注入；
   - 保留 `Cache-Control: public, max-age=86400`；
   - `/public/site/icon.*` 仍 inline，不受影响。
2. 新增全局安全响应头中间件 `securityHeaders()`（正常模式与应急模式都注册，放在 `requestLogger` 附近）：
   - 至少 `X-Content-Type-Options: nosniff`；
   - 本项只加 nosniff，不夹带 CSP/frame 策略，避免扩大行为变化面（后续如需可另立安全项）。
3. `HomeView.openInstaller` 改为 `window.open(url, '_blank', 'noopener,noreferrer')`；如担心旧浏览器对 feature 参数兼容性，改用临时 `<a rel="noopener noreferrer" target="_blank">` 点击。
4. **上传时拒绝危险扩展名（已确认 D-F03/D-F03-1）**：在 `platform.UploadInstaller` 增加扩展名黑名单，至少拒绝 `html/htm/xhtml/svg/svgz/js/mjs/xml`（实施时统一小写化、去点后比较）；拒绝时返回 400 并提示仅允许可下载安装包格式。`attachment + nosniff` 仍作为纵深防御保留。
5. **独立无 Cookie 下载域名列为后续项（已确认 D-F03-1）**：本次不引入独立域名，当前附件下载已切断同源脚本执行；后续需要 CDN/完全隔离时再在 Design 中立项。

**架构与副作用：**

- 上传策略从“不限制扩展名”（Design1 §6.3）变更为“拒绝危险可执行/标记类型”，需要在实施时同步 Design/UI 文案；这属于用户确认后的设计收紧，不改变 300MB 大小限制。
- 升级前已经上传的危险扩展名文件不会被自动删除，但会以 `attachment + nosniff` 下载、不再同源执行；如需清理，由管理员在平台安装包列表中逐项删除。
- 安装包从“同源打开”变为“下载”，是预期行为变化；exe/dmg/apk/zip 等正常文件不受影响。
- 站点 ICON 仍可 inline 展示；若未来 `/public` 新增其他可预览资源，需按资源类型单独决定是否 attachment。
- `noopener` 后 `window.open` 返回值为 `null`，当前函数未使用返回值，不受影响。

**测试点：**

- 上传 `.html/.svg/.js` 被 400 拒绝且不落盘；`.zip/.dmg/.exe` 仍成功；
- GET 断言 `X-Content-Type-Options: nosniff`、`Content-Disposition: attachment`；
- 站点 ICON GET 断言无 attachment 且可 inline；路径穿越用例保持 404；应急模式静态路径同样有 nosniff。

**已确认：** D-F03=增加危险扩展名拒绝；D-F03-1=拒绝危险扩展名，独立下载域名列为后续。

---

### 8.3 F06【中】OIDC 测试端点收敛 + 消除 DNS rebinding TOCTOU

**当前代码：** `backend/internal/server/oidc.go:29-40` 以 `oidc_configured` 决定匿名可达；`backend/internal/oidc/ssrf.go` 先 `net.LookupIP` 校验，`oidc.go` 的 `DialContext` 再解析一次，形成两次解析。

**推荐方案：**

1. **权限条件改为系统级 `configured`，并做严格读取（已确认 D-F06-1）**：
   - OidcHandler 注入 `config.Service`（或一个 `SystemConfigured(ctx) (bool,error)` 小接口），读取 `config.KeyConfigured`；DB 读取失败必须 500，不能静默按未配置放行。
   - Setup 匿名端点迁移为 `/api/setup/oidc/test`，语义与 `/api/setup/*` 一致：**仅系统未配置时可用，系统已配置返回 404 并不再匿名开放**；管理员测试只保留既有 `/api/admin/settings/oidc/test`。Setup 前端 `frontend/src/api/oidc.ts` 同步改路径。
   - 保留 `ratelimit_oidc_test`（10/min）与 HTTPS+公网 IP 防护、上游响应体不回显。
2. **DNS 只解析一次并 pin 住实际拨号 IP**：
   - 将 `validateOIDCURL` 拆为两层：预检只校验 URL 语法（scheme=https、hostname 非空），**不再做 DNS 预解析**；
   - `http.Transport.DialContext` 在拨号时刻执行唯一一次 `LookupIPAddr`，拒绝任一非公网 IP，并直接向本次解析得到的 IP 拨号（TCP 连接天然 pin 住该 IP）；TLS SNI 仍由 `http.Transport` 使用 URL 中的原始主机名，证书校验不受影响；
   - `CheckRedirect` 只做语法校验 + 次数限制；重定向后的最终拨号同样被 DialContext 兜底。这样不存在“校验解析”与“拨号解析”之间可被 rebinding 利用的窗口。
   - 测试连接错误信息改为通用“发现文档不可达/地址校验失败”，不暴露内部解析细节。

**架构与副作用：**

- 迁移 Setup 测试端点路径是一次小的前端+后端协同变更，不动 Setup 业务事务语义。
- DNS 失败/私网拒绝的报错时机从“发请求前”变为“拨号时”，单测断言需同步调整。
- 若部署者配置了环境代理，`http.Transport` 的 `DialContext` 只能看到代理地址而看不到最终目标 IP；**已确认保留 `ProxyFromEnvironment`（D-F06-2）**，因此经代理访问 OIDC 时存在“代理地址替代目标 IP 校验”的边界，由部署者保证代理可信；不使用代理的直连场景仍由 DialContext 完整校验目标 IP。

**测试点：**

- 系统 `configured=true` 时匿名调用旧/新端点均拒绝；`configured=false` 时匿名可用且限流生效；管理员端点始终要求会话+管理员。
- 解析返回混合公网/私网 IP 时请求必须失败；重定向到私网/HTTP 必须失败；无重定向回显。

**已确认：** D-F06-1=迁移为 `/api/setup/oidc/test`；D-F06-2=保留 `ProxyFromEnvironment`，代理可信作为部署边界。

---

### 8.4 F07【中】日志脱敏补全 + 重置链接改 fragment + 日志补 IP（L09 的日志部分一并处理）

**当前代码：** `backend/internal/server/server.go:496-506` 记录 `RequestURI`；`backend/internal/log/log.go:62-66` 仅正则脱敏 `?token=`；`frontend/src/router/index.ts:64` 使用 `/reset/:token`；`auth/reset.go:110-112` 生成路径链接。

**推荐方案：**

1. **重写 `log.Redact` 为“结构化解析 + 正则兜底”**：
   - 路径段：`/reset/<token>` 一律替换为 `/reset/***`（大小写不敏感）；
   - 查询串：按 `&`/`;` 分段，对每段取第一个 `=` 前的内容做 `url.QueryUnescape`，键名 `token`/`code`/`state`（大小写不敏感）命中时保留原键、值替换为 `***`；未命中保留原始段。此方式可覆盖 `%3D`、`%74oken`、`STATE` 等大小写/编码绕过，且不改动正常参数的记录形态；
   - 保留现有正则作为字符串非 URL 场景的兜底。
   - 该脱敏在 `RedactHandler` 内生效，因此 stdout、环形缓冲/实时日志流、`panicRecovery` 中的 `path` 属性都会被统一覆盖。
2. **请求日志补客户端 IP**：`requestLogger` 增加 `"ip", c.ClientIP()`。IP 与 `TRUST_PROXY` 策略同口径（F09 修复后自动一致）；在文档中注明该字段属于运维日志，不进访问日志 UI 新增项。
3. **重置链接改 fragment，并移除旧路径（已确认 D-F07）**：
   - `auth/reset.go:resetLink` 改为 `/reset#token=<url.QueryEscape(token)>`；
   - 前端路由只保留 `/reset`，删除 `/reset/:token`；`ResetView` 从 `route.hash` 读取 `token`，读取后 `history.replaceState` 清除 hash；提交 API 不变。
   - 新链接的 token 不再进入 Web 服务器/反代访问日志、Referer；旧邮件中的 `/reset/:token` 在本次上线后失效（Token 最长 1 小时，过渡期风险可控），无需保留旧路由。

**架构与副作用：**

- 邮件链接形态变化；升级前已发送的旧链接会失效，需在变更记录/发布说明中提示，不强制重发。
- 日志中所有 `code=`/`state=` 参数都会被脱敏，即使个别非 OIDC 场景使用同名参数，也不影响正确性，只影响排障细节，符合“宁多脱敏不漏脱敏”。
- `url.QueryUnescape` 只用于日志侧判定，不回写请求对象，不影响业务解析。

**测试点：**

- `log.Redact` 表驱动新增：`/reset/TOKEN`、`?token=SECRET`、`?TOKEN=SECRET`、`?code=ABC&state=XYZ`、`?code%3DABC`、`?%74oken%3dSECRET`、`;state=XYZ`；确认值不出现、非敏感参数不丢失。
- `requestLogger` 输出含 `ip`；`ResetView` 单测覆盖 fragment 读取与 hash 清理。

**已确认：** D-F07=立即移除旧 `/reset/:token` 路径。

---

### 8.5 F08【中】HTTP Server 超时 + 请求体限制（配置化，纳入“速率限制”面板区）

**当前代码：** `backend/internal/server/server.go:84-86` 与 `451-452` 的 `http.Server` 未设置任何超时；无 `MaxBytesReader` 全局/路由限制。

**推荐方案：**

1. **新增一个共享的 server hardening 构造器**（普通模式与应急模式共用，避免两份 `http.Server` 配置漂移），从 `system_config` 读取以下键并设置到 `http.Server`：

| 配置键 | 默认 | 建议可调范围 | 说明 |
|---|---|---|---|
| `http_read_header_timeout_sec` | 5 | 1–60 | 防慢头攻击 |
| `http_read_timeout_sec` | 60 | 1–3600 | 整个请求体读取期限；大文件上传路由在 handler 内用 `ResponseController.SetReadDeadline(time.Time{})` 清空，再交业务层 `LimitReader` 管控大小 |
| `http_write_timeout_sec` | 300 | 1–3600 | 响应写出期限；SSE、备份下载、`/public` 大文件等长流响应在 handler 内清空 write deadline |
| `http_idle_timeout_sec` | 120 | 1–3600 | keep-alive 空闲回收 |
| `http_max_body_mb` | 4 | 1–320 | 通用 API 请求体上限 |

2. **请求体限制按路由分级**（中间件在路由匹配后、handler 前执行，按 `c.FullPath()` 查表）：

| 路由 | 上限 |
|---|---|
| 默认 API | `http_max_body_mb`（默认 4 MiB，面板可调，改后即时生效） |
| 订阅/规则/自定义/分享版本上传（含 text 模式） | 55 MiB（50 MiB 内容 + multipart/编码余量，固定） |
| `/api/admin/platforms/:id/installers` | 320 MiB（300 MiB 安装包 + multipart 余量，固定） |
| `/api/setup/import` 与 `/api/admin/settings/import` | **豁免全局请求体限制（已确认 D-F08-2，维持 F02 部署边界口径）**，继续由反代 `client_max_body_size` 与部署边界控制 |

   - 超限统一映射为 **413「请求体过大」（已确认 D-F08-3）**；实施时同步更新 AGENTS §4.8 错误码表与前端 `request.ts` 的 413 文案；`http.MaxBytesReader` 的错误要在中间件统一识别，避免落入 500。
   - `http.MaxBytesReader` 只限制总请求体字节数，不能替代业务层大小校验；现有 50MB/300MB/2MB 业务校验全部保留。
3. **面板配置**：
   - 复用现有“速率限制”卡片（可改名“速率限制与连接防护”），GET/PUT `/api/admin/settings/ratelimit` 扩展返回/接收上述 5 个字段；四个速率值继续即时生效；**Server 超时字段在 `http.Server` 启动时读取，保存后提示“重启容器后生效”**；`http_max_body_mb` 因走中间件每次请求读配置，可即时生效。
   - 旧前端只提交四个速率字段时，后端对缺失的防护字段取默认值并保持兼容。
4. **长连接/长传输豁免清单**（与超时同时落地，避免回归）：
   - `/api/admin/logs/stream`（SSE）：进入 handler 后清除 write deadline；
   - `/api/admin/settings/backup`、配置导出下载、`/public/*` 安装包下载：清除 write deadline；
   - 安装包/版本文件上传 handler：清除 read deadline（但 body 总大小仍受 MaxBytesReader 上限）。
   - 不豁免普通 API、登录/注册/找回密码等小请求，保持慢体攻击防护。

**架构与副作用：**

- `ReadTimeout`/`WriteTimeout` 是对“整段读取/写出”的硬截止，直接设置会切断 SSE 与 300MB 上传/下载，所以必须与逐端点 deadline 清理一起落地；这是本项最主要的回归风险。
- 应急模式同样套用默认值，但不含 SSE/业务上传，风险更低。
- 配置全部存 `system_config`，不违反“业务配置走 Web UI”；默认值在无配置时生效，与现有设计一致。

**测试点：**

- 慢头：`httptest` 连接延迟发送 header，超过 5s 应被断开（或读取失败）。
- 超 body：4 MiB JSON 请求返回 413；安装包 multipart ≤300 MiB 仍成功、>300 MiB 仍由业务层 400（业务大小校验，非连接层限制）；版本上传 50 MiB 边界不回归。
- SSE：建立连接后超过 WriteTimeout 时间仍保持，或验证 handler 清空 deadline 后不断开。
- 面板 GET/PUT 新字段持久化；旧字段兼容。

**已确认：** D-F08-1=推荐默认值 + 长传输豁免；D-F08-2=导入路由豁免全局 body 限制；D-F08-3=新增 413；D-F08-4=按推荐清单豁免长传输端点。

---

### 8.6 F09【中】TRUST_PROXY 显式 CIDR 与 X-Forwarded-Proto 同口径信任

**当前代码：** `backend/internal/server/server.go:481-490` 仅三档硬编码；`backend/internal/setup/setup.go:163-195` 的 `DeriveFrontendURL` 无论是否可信都接受 `X-Forwarded-Proto`。

**推荐方案：**

1. 新增环境变量 `TRUST_PROXY_CIDRS`（逗号分隔 CIDR，仅运维参数，不走 Web UI；**已确认 D-F09=新增独立 `cidr` 档位，D-F09-1=cidr 自动包含回环地址**）：
   - `TRUST_PROXY=cidr`：可信集合 = 显式 CIDR + `127.0.0.1/8`、`::1/128`；用于 Cloudflare/EdgeOne 回源段，且本机反代无需重复配置回环；
   - `TRUST_PROXY=auto`：维持既有回环/RFC1918 默认值，不使用 `TRUST_PROXY_CIDRS`；
   - `TRUST_PROXY=off`：从不信任，忽略 `TRUST_PROXY_CIDRS`；`TRUST_PROXY=on`：仍全信任并保留 gin 不安全警告；
   - `TRUST_PROXY` 取值扩展为 `auto/on/off/cidr` 四档，非法值启动时直接退出；
   - 启动时解析失败（非法 CIDR）直接退出并给出明确错误，与 `APP_MODE` 非法同策略。
2. 抽取共享信任策略，消除 server 与 setup 两处口径漂移：
   - 新建轻量 `internal/proxytrust`：持有 `mode + explicitCIDRs`（支持 auto/on/off/cidr 四档），提供 `Trusted(remoteAddr string) bool` 与 `GinTrustedProxies() []string`；
   - `server.applyTrustProxy` 与 `setup.Service.trustedForwarded` 都改用它。
3. **修复 `DeriveFrontendURL`**：
   - `scheme = https` 仅当 `r.TLS != nil` **或** `trusted && X-Forwarded-Proto=https`；不可信时忽略伪造的 `X-Forwarded-Proto`。
4. 面板“速率限制”区展示当前策略时，增加 `TRUST_PROXY_CIDRS` 生效摘要（如条目数/原始列表），便于排障；README/compose example 在实施时同步给出 Cloudflare/EdgeOne 示例与“请以厂商最新回源段为准”的提示。

**架构与副作用：**

- 新增一个很小内部包，不构成架构变动；`main` 解析 env 后注入 server/setup。
- Cloudflare/EdgeOne 回源 IP 段会变化，部署者需自行维护该 env 并重启；文档须写明。
- 应用若绑定 `0.0.0.0` 且把 WAF 段加入信任，仍可能被同一网段内其他来源伪造头；生产推荐继续 `127.0.0.1:8080` 反代接入，README 必须同步强调。
- 既有测试中“不可信但带 XFP=https 仍推导 https”的用例需按新口径修正。

**测试点：**

- `applyTrustProxy(cidr)` 下，显式段内来源与回环来源的 XFF 生效、段外来源忽略；`off` 忽略显式 CIDR；`on` 保持全信任。
- `DeriveFrontendURL`：不可信 + XFP=https → http；可信 + XFP=https → https；TLS 直连仍 https。
- 非法 CIDR 启动失败。

**已确认：** D-F09=新增 `cidr` 档位；D-F09-1=`cidr = 显式 CIDR + 回环地址`。

---

### 8.7 L01【低】OIDC 回调改“一次性 ticket + HttpOnly Cookie 换票”

**当前代码：** `backend/internal/server/oidc.go:93-103` 把会话 token 放入 302 Location；`frontend/src/views/OidcCallbackView.vue:12-14` 接受任意 `?token=`。

**推荐方案：**

1. 新增迁移 `1013_oidc_login_tickets.sql`：

```sql
CREATE TABLE oidc_login_tickets (
    ticket       TEXT PRIMARY KEY,          -- 256 位随机
    session_token TEXT NOT NULL,            -- 已签发的会话凭据（仅短时暂存，换票后删除）
    expires_at   TIMESTAMP NOT NULL,        -- 建议 60 秒
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_oidc_login_tickets_exp ON oidc_login_tickets(expires_at);
```

2. 回调成功并签发会话后：
   - 生成 ticket，事务内写入上表（写入前顺带 `DELETE FROM oidc_login_tickets WHERE expires_at < now`，防未换票记录积累），设置 `HttpOnly + SameSite=Lax` Cookie（名 `oidc_login_ticket`，Path 限定 `/api/auth/oidc/exchange`，TTL 与 Secure 规则见 L04）；
   - 302 到 `/login/callback`，**不再携带任何查询参数**。
3. 新增 `POST /api/auth/oidc/exchange`：
   - 读取 HttpOnly Cookie → 事务内 `SELECT ... WHERE ticket=? AND expires_at > now` → 命中即 `DELETE`（严格一次性）→ 返回 `{token, expires_at}`；
   - 未命中/过期返回 401 通用错误；处理完成后 `SetCookie` 清除 ticket Cookie。
4. `OidcCallbackView` 改为 `onMounted` 调 exchange 接口 → 成功才写 store → 跳首页；失败跳 `/login?oidc_error=exchange_failed`。前端不再读取任何 `?token=`。
5. mock 登录仍走 JSON 响应，不经过回调中转页，不受影响。

**架构与副作用：**

- 会话 token 不再出现在 URL/Referer/浏览器历史/反代日志；登录 CSRF/账号互换需攻击者先种入 HttpOnly Cookie，而该 Cookie 只能由通过 state 三重校验的回调签发。
- `session_token` 短时落库是可接受的暂存；表被清理（全清）时未换票登录失败，用户重新登录即可。
- 前端多一次同源 POST；回调页体验增加一次网络往返，不影响视觉流程。
- 若先实施 F03，localStorage 同源 XSS 面已显著收窄；如需彻底消除 token 盗取面，需改全站 HttpOnly 会话架构，超出本项范围。

**测试点：**

- 回调后 Location 不含 token；exchange 首次成功、二次 401；过期 ticket 401；Cookie HttpOnly/SameSite/Secure 断言；前端回调页单测覆盖成功/失败分支。

**已确认：** D-L01=新增 `oidc_login_tickets` 表 + HttpOnly Cookie 换票。

---

### 8.8 L02【低】配置导入复用“认证死锁”校验

**当前代码：** `backend/internal/config/export.go:243-257`（v1 同步导入）与 `326-340`（v2 导入事务）直接 `DELETE FROM system_config` 后全量写入，未执行 `admin.go` 中 `SaveOidc`/`ClearOidc`/`SaveLocalAuth` 同款 `ErrAuthDeadlock` 校验（当前为 `admin.go:170-207` 与 `277-288`）。

**推荐方案：**

1. 在 `config` 包新增纯函数 `ValidateImportedAuthUsable(cfgMap map[string]string) error`，按“导入后视角”判定：
   - `configured != "true"` → 跳过（系统仍回 Setup，不会死锁）；
   - `allow_local_login` 缺失按默认 `true` 处理；存在则严格解析布尔，非法值拒绝导入（避免写入后状态接口 500）；
   - `allow_local_login == false` 时，要求 `oidc_provider_type` 非空且 `oidc_params_<provider>` 的 JSON 中 `base_url`、`client_id`、`client_secret` 均非空；否则返回 `ErrAuthDeadlock`。
2. `Import`（v1 同步）与 `importV2`（v2 异步）在解密成功、进入事务前调用该函数；`ErrAuthDeadlock` 映射为 400，不注册异步任务、不做任何写入。
3. `checkImportProtection` 保持先执行（signing_key 保护），本校验在其后；两者顺序可固定为“先完整性/密钥保护，再认证可用性”。

**架构与副作用：**

- 会拒绝“本地登录关且 OIDC 不可用”的配置文件，这正是防锁死目的；正常导出文件均能通过。
- 判定口径比 UI 保存路径更完整（UI 仅看 `oidc_configured` 标记），属于正向收紧；若用户希望与 UI 完全一致，可降级为只查 `oidc_configured=true`（见 D-L02）。
- 不影响导出、全清、备份等其它运维路径。

**测试点：**

- 导入本地关 + OIDC 可用 → 成功；本地关 + OIDC 参数缺失/secret 空 → 400 且原库配置不变；本地开 → 成功；`configured=false` 模板 → 跳过；非法布尔 → 400。

**已确认：** D-L02=按实际可用性严格校验（local 关闭时要求 OIDC 参数完整）。

---

### 8.9 L04【低】反代 TLS 终止下 OIDC Cookie 的 Secure 判定

**当前代码：** `backend/internal/server/oidc.go:53,152` 仅用 `c.Request.TLS != nil`。

**推荐方案：**

1. 与 F09 共享 `proxytrust` 策略，新增 `requestIsSecure(c, policy)`：
   - `true` 当且仅当任一成立：`c.Request.TLS != nil`；远端经 `proxytrust.Trusted` 判定可信且 `X-Forwarded-Proto` 首个值为 `https`；`frontend_url` 配置解析为 `https` 且解析成功。
   - 优先级按上述顺序；`frontend_url` 仅作最后兜底，防止配置错误导致 HTTP 环境误设 Secure Cookie。
2. OIDC state Cookie（login/bind）与 L01 ticket Cookie 统一改用该判定，不再直接看 `c.Request.TLS`。
3. 日志记录 cookie secure 决策时避免记录完整 frontend_url（无敏感参数，可记录 scheme）。

**架构与副作用：**

- 与 F09 的信任口径完全一致，修复“auto 下反代 TLS 终止但 Cookie 不 Secure”的问题。
- `TRUST_PROXY=on` 时仍可能被直连伪造 XFP=https 导致 HTTP 下 Cookie 带 Secure 而登录失败；这是 `on` 档既有语义，README 需提示 `on` 仅用于完全受控前置链路。

**测试点：**

- TLS 直连 → Secure；可信反代 + XFP=https → Secure；不可信 + XFP=https → 不 Secure；frontend_url=https 且无转发头 → Secure；HTTP frontend_url → 不 Secure。

**已确认：** D-L04=Secure 判定顺序 TLS > 可信 XFP > frontend_url https。

---

### 8.10 L05【低】前端工具链漏洞升级

**当前情况：** 本次复核 `npm audit --package-lock-only` 结果为 23 项（1 critical / 19 high / 3 moderate）；除报告所列 `vitest/vite/esbuild/nanoid` 外，还有经 `postcss→@vue/compiler-sfc` 传播到 `vue`/`ant-design-vue`/`pinia` 等的高危告警。这些包主要处于 dev/build/test 链，不进入生产运行时 bundle，但 CI/供应链风险应处理。

**推荐方案：**

1. **已确认 D-L05=直接升级 Vite 7 最新版**（实施时用 `npm view vite version` / `npm audit` 复核最新稳定版本）：
   - `vite` 升级到 Vite 7 最新稳定版（Vite 7 需 Node 20.19+/22.12+，Dockerfile 使用 `node:22-alpine` 满足）；
   - `vitest` 同步升级到与 Vite 7 兼容的最新稳定版（且必须 `>=3.2.6`，修复 GHSA-5xrq-8626-4rwp）；
   - `@vitejs/plugin-vue`、`vue-tsc`、`@vue/test-utils` 等适配 Vite 7 主版本；
   - 通过升级让 `esbuild >=0.25.0`、`nanoid >=3.3.18` 由依赖树自然解析；若仍有旧版本残留，用 `npm ls esbuild nanoid` 定位后再决定是否使用 `overrides`。
2. 升级后执行 `npm audit --package-lock-only` 归零或对剩余项逐条评估；执行 `npm run build && npm test`，并人工回归 `vite.config.ts`/`vitest.config.ts` 相关构建行为。
3. **已确认 D-L05-CI=加入 CI 门禁**：在 GitHub Actions 构建流程中增加 `npm audit --audit-level=high`，高危即失败；本地 smoke 脚本同步保留该检查。
4. 不升级与本次安全无关的运行时大版本（Vue/AntDV 等），除非 audit 明确指向且修复可用；避免为了“清零”制造大规模 UI 回归。

**架构与副作用：**

- Vite 7 + Vitest 大版本升级可能影响 dev server 行为与测试环境 jsdom 配置，但生产构建产物理论不变；需要跑完整测试集与手工 smoke。
- `package-lock.json` 会发生较大变化，属于工具链变更，不改变运行时依赖树（需用 `npm ls --omit=dev` 复核）。
- CI 增加 audit 门禁后，未来新依赖若触发高危告警会阻断合并，需要在升级流程文档中说明豁免机制（仅可由用户显式批准）。

**测试点：** 前端 65+ 单测全绿；`npm run build` 成功；Docker 多阶段构建成功；`npm audit --audit-level=high` 通过或剩余项用户签字。

**已确认：** D-L05=直接升级 Vite 7 最新版；D-L05-CI=加入 CI 门禁，阻断高危。

---

### 8.11 L08【低】过期密码重置令牌每日清理

**当前代码：** `backend/internal/auth/reset.go` 仅用后即删；无后台清理。

**推荐方案：**

1. `cron` 包新增 `StartResetTokenCleanup(db, lg)`：
   - 启动时立即执行一次，随后每 24 小时执行；
   - `DELETE FROM password_reset_tokens WHERE expires_at < ? OR used = 1`；
   - 返回 stop 函数，main 与 `StartAccessLogCleanup` 并列启动、退出时关闭。
2. 不改动令牌生成/消费事务语义；删除动作与 `Complete` 的用后即删并发安全（SQLite 写串行）。
3. 可选：在 `Request`/`IssueForUser` 写入前顺带清理该用户过期令牌（与 OIDC state 写入前清理同模式），进一步防积累；非必需。

**架构与副作用：** 纯后台清理，无接口/表结构变化；失败只记 error 日志，不影响主流程。

**测试点：** 插入过期/已用/未过期记录，调用清理函数后只保留未过期未用；stop 后不再执行。

**已确认：** D-L08=仅每日清理任务，不做写入前顺带清理。

---

### 8.12 L09【低】请求日志补客户端 IP（与 F07 同批完成）

- 在 F07 中已覆盖：`requestLogger` 增加 `ip=c.ClientIP()`，脱敏解析覆盖大小写/编码形态。
- 隐私口径：IP 仅出现在 stdout/实时日志流（管理员可见）与既有 `access_logs.ip`，不新增公开接口；README 部署说明中补充“日志含客户端 IP，按当地合规要求管理日志留存”。

---

### 8.13 不纳入项的 README 补充方案（随修复批次一并落地）

以下不写代码，只在 README / compose 注释中补齐部署边界：

1. **F02**：快速开始前增加显著提示——“首次 Setup 完成并注册管理员之前，不要把服务暴露到公网；公网部署请先仅绑定 `127.0.0.1`，完成 Setup 后再开放反代接入”。Setup 导入端点仍依赖导出密码 + 5/min 限流，不额外加代码。
2. **F04**：根目录 `docker-compose.yml` 顶部注释标明“Dev 模式 + 全接口暴露，仅限个人本机/可信内网测试；公网或真实数据必须使用 `docker-compose.yml.example` 的 Production 配置”。README 快速开始示例改为显式 `APP_MODE: prod` 并给出回环/反代二选一说明。
3. **F05**：README 明确“OIDC 授权回调承载登录凭据，公网部署必须由 HTTPS 反代接入；不得公网 HTTP 直连启用 OIDC”。同时保留局域网直连仅限可信内网且不建议 OIDC 的既有口径。
4. **F10**：README 面板配置/平台说明增加“平台 scheme 会唤起本机客户端，只应配置可信客户端 scheme；不要配置来源不明的 scheme”。代码不做 scheme 协议白名单。
5. **F11**：README 规则素材池说明增加“素材池 URL 同步为管理员配置的服务端拉取；内网部署默认可信；在共享/云环境部署时应自行限制出网目标，系统当前不默认拦截私网地址”。

---

### 8.14 批次与文档落地建议

| 批次 | 内容 | 验收 |
|---|---|---|
| P0 | F01、F03、F06、F07、F08、F09 | 后端 build/vet/test、前端 build/test、Docker build、手工 smoke（OIDC 真实提供商 / 安装包下载 / 反代 Secure Cookie / SSE） |
| P1 | L01、L02、L04、L08、L09 | 同上；重点回归 OIDC 登录/绑定、配置导入导出、密码重置 |
| P2 | L05、README/compose 补充 | `npm audit` 复核、前端全量测试、README 走查 |

每个批次完成后在 Issue/Build 文档闭环；SecurityReport1 的总表与本节状态同步更新。全部待决策项已在 §8.15 确认；实施中若因真实环境兼容性需要变更，先回写本节并经用户确认再进入编码。

---

### 8.15 已确认决策记录（2026-08-25 逐项确认，已同步回写各小节）

| 编号 | 决策点 | 确认结果 | 落地要点 |
|---|---|---|---|
| D-F01-1 | OIDC 验签集成方式 | 只引入 go-oidc `RemoteKeySet + NewVerifier` | 保留现有发现文档与 token 交换流程，verifier 按 issuer 缓存 |
| D-F01-2 | 旧授权流升级中断 | 严格校验 nonce，不设兼容窗口 | 升级瞬间旧的 in-flight 回调失败，用户重新登录 |
| D-F01-3 | 算法白名单与 azp | 非对称白名单 + azp 严格等于 client_id | 拒绝 none/HS；azp 存在时必须匹配 |
| D-F03 | 安装包扩展名 | 增加危险扩展名拒绝 | 上传时拒绝 `html/htm/xhtml/svg/svgz/js/mjs/xml` |
| D-F03-1 | 独立下载域名 | 拒绝危险扩展名，独立域名列为后续 | 本次只落上传黑名单 + attachment/nosniff |
| D-F06-1 | Setup OIDC 测试端点 | 迁移为 `/api/setup/oidc/test` | 已配置后不再匿名开放，管理员端点不变 |
| D-F06-2 | OIDC 出站代理 | 保留 `ProxyFromEnvironment` | 代理可信作为部署边界；直连仍做目标 IP 校验 |
| D-F07 | 重置链接旧路径 | 立即移除旧 `/reset/:token` | 新链接只用 fragment；发布说明提示旧邮件失效 |
| D-F08-1 | 超时默认值 | ReadHeader 5s / Read 60s / Write 300s / Idle 120s + 长传输豁免 | 逐端点清 deadline，避免 SSE/大文件回归 |
| D-F08-2 | Setup/面板导入体限 | 导入路由豁免全局请求体限制 | 维持 F02 部署边界口径 |
| D-F08-3 | 超限状态码 | 新增 413 | 同步更新 AGENTS §4.8 错误码表与前端文案 |
| D-F08-4 | 长传输豁免口径 | 按推荐清单豁免 SSE/备份/导出/公开安装包/大文件上传 | 其余 API 保持超时防护 |
| D-F09 | `TRUST_PROXY_CIDRS` 档位语义 | 新增独立 `cidr` 档位 | auto/on/off 语义不变；cidr 只信任显式列表 |
| D-F09-1 | cidr 是否含回环 | `cidr = 显式 CIDR + 回环地址` | 自动含 `127.0.0.1/8`、`::1/128` |
| D-L01 | 登录换票实现 | 新表 `oidc_login_tickets` + HttpOnly Cookie | 严格一次性，写入前清理过期 ticket |
| D-L02 | 导入死锁判定 | 按实际可用性严格校验 | local 关闭时要求 OIDC 参数完整 |
| D-L04 | Secure Cookie 兜底 | TLS > 可信 XFP > frontend_url https | state/ticket Cookie 统一使用 |
| D-L05 | 工具链升级幅度 | 直接升级 Vite 7 最新版 | Vitest 同步最新兼容版，且 ≥3.2.6 |
| D-L05-CI | npm audit 门禁 | 加入 CI `--audit-level=high`，阻断高危 | 本地 smoke 同步检查 |
| D-L08 | 重置令牌清理 | 仅每日清理任务 | 不做写入前顺带清理 |

> 上述结论已逐项回写 §8.1～§8.13 与 §8.16；后续 Build 执行时直接按本表落地，不再重复确认。若实施中因真实提供商兼容性需调整算法白名单或危险扩展名清单，先提出变更并回写本文档。

---

### 8.16 修复后验证矩阵（建议在 Build 验收时逐项打勾）

| 项 | 验证内容 |
|---|---|
| F01 | 伪造 id_token（无签名/none/HS/错 aud/错 iss/过期/错 nonce/错 azp）全部拒绝；真实 OIDC 登录成功 |
| F03 | 危险扩展名上传被 400 拒绝且不落盘；`/public/installers/*` 下载附 attachment + nosniff；ICON inline 正常 |
| F06 | 未配置 Setup 经 `/api/setup/oidc/test` 可测；已配置匿名不可测；管理员专用可测；私网/DNS rebinding 用例拒绝 |
| F07 | 日志/缓冲中 reset token、code、state 不可见（含大小写/编码形态）；新 fragment 重置邮件可完成重置，旧路径已移除 |
| F08 | 慢头被限、超体返回 413；300MB 安装包上传、SSE、备份下载不回归；面板新配置持久化并提示重启 |
| F09 | `cidr` 档位下显式段与回环来源 XFF 生效、段外忽略；不可信 XFP 不再改变推导 scheme；限流与日志 IP 一致 |
| L01 | OIDC 回调 URL 无 token；换票一次有效；跨站不可换票；登录/绑定流程正常 |
| L02 | 坏配置导入被 400 拒绝且零写入；正常导出文件往返通过 |
| L04 | 反代 HTTPS 下 state/ticket Cookie 带 Secure；直连 HTTP 不带 |
| L05 | Vite 7 + Vitest 最新兼容版升级完成；`npm audit --audit-level=high` 通过或剩余项用户签字；build/test/Docker build 通过 |
| L08 | 过期/已用重置令牌被每日清理 |
| L09 | 请求日志含 IP；脱敏覆盖编码绕过用例 |

---

### 8.17 P0 → P1 → P2 文件级实施计划（已结合当前代码研究，供后续 Build 承接）

> 本节约 2026-08-25 按用户确认写入：只更新 SecurityReport1.md，不改动代码。实施顺序严格按 §8.14；各项决策以 §8.15 为准，不再重复确认。

#### 8.17.1 共同前置约定

1. 数据库迁移只新增文件，不改老迁移；新增列必须带默认值。
2. 所有 `frontend/` 改动完成后必须重新 `npm run build`，如有需要把产物同步到 `backend/web/dist`。
3. 通用验证命令：
   - `cd backend && go build ./... && go vet ./... && go test ./...`
   - `cd frontend && npm run build && npm test`
4. 若实施中发现与本计划/§8.15 冲突，先暂停并回写本报告向用户确认，再进入编码。
5. 本计划中的“新增文件”允许在 Build 阶段创建；当前工作区仍未修改代码。

#### 8.17.2 P0 批：F01、F03、F06、F07、F08、F09

##### P0-F01 OIDC id_token 验签 + nonce/iss/aud/exp/azp

- `backend/go.mod` / `go.sum`：新增 `github.com/coreos/go-oidc/v3`（本地模块缓存已含 v3.10.0/v3.19.0/v3.20.0，建议 `go mod tidy` 解析到 v3.20.0；若网络受限使用模块缓存）。
- 新增 `backend/migrations/1012_oidc_nonce.sql`：
  `ALTER TABLE oidc_states ADD COLUMN nonce TEXT NOT NULL DEFAULT '';`
- `backend/internal/oidc/flow.go`：
  - `StateRecord` 增加 `Nonce string`；
  - `StartFlow` 生成 32 字节随机 nonce，`saveState` 写入；授权 URL 增加 `nonce=<base64url>`；
  - `saveState` 的 INSERT 列增加 `nonce`；
  - `ConsumeState` 的 SELECT 增加 `nonce` 并回填。
- `backend/internal/oidc/oidc.go`：
  - 在 `Service` 中增加按 issuer/JWKS 缓存的 verifier（建议 `verifierCache map[string]*goidc.IDTokenVerifier` 与 `verifierMu`）；
  - 用 `goidc.NewRemoteKeySet(goidc.ClientContext(ctx, s.httpCli), disc.JWKSURI)` + `goidc.NewVerifier(issuer, keySet, &goidc.Config{...})` 构造；
  - `Config`：`ClientID`、`SkipIssuerCheck:false`、`SkipExpiryCheck:false`、`SupportedSigningAlgs` 只含 RS/ES/PS/EdDSA 等非对称算法，禁止 HS/none；
  - `ClearDiscCache` 同步清空 verifier 缓存。
- `backend/internal/oidc/flow.go` `Exchange`：
  - 真实模式删除直接 `decodeJWTPayload` 信任路径，改为 `verifier.Verify`；
  - 校验 `idToken.Nonce == rec.Nonce`；
  - 通过 `idToken.Claims(&claims)` 读取 `azp`，若存在必须等于 `p.ClientID`；
  - `Identity.RawClaims` 用验签后 claims 序列化保存，保持审批/白名单语义不变；
  - mock 模式继续走 `mockExchange`。
- 测试：`backend/internal/oidc/oidc_test.go` 增加固定 JWKS 单测：合法 RS/ES 通过；错误签名/none/HS/错 iss/aud/exp/nonce/azp 拒绝；JWKS 轮换通过；StartFlow 授权 URL 与 state 记录含 nonce。

##### P0-F03 安装包附件下载 + 危险扩展名 + nosniff + noopener

- `backend/internal/platform/platform.go`：
  - 新增危险扩展名集合：`html/htm/xhtml/svg/svgz/js/mjs/xml`；
  - `UploadInstaller` 在 `sanitizeExt` 后校验，命中返回可识别错误（建议新增 `ErrUnsafeInstallerExt` 或复用 `ErrBadRequest`）；
  - 保持 300MB 与路径穿越防护不变。
- `backend/internal/server/platform.go`：将新错误映射为 400，并给出“仅允许可下载安装包格式”提示。
- `backend/internal/server/static.go`：
  - 新增安全响应头中间件（`X-Content-Type-Options: nosniff`），普通与应急模式都注册；
  - `/public/installers/*` 分支设置 `Content-Disposition: attachment`，文件名用 `mime.FormatMediaType` 安全构造；
  - `/public/site/*` 保持 inline。
- `frontend/src/views/HomeView.vue`：`openInstaller` 改为 `window.open(url, '_blank', 'noopener,noreferrer')`，或使用带 `rel` 的临时 `<a>`。
- 测试：平台上传 `.html/.svg/.js` 被拒且不落盘；`.zip/.dmg/.exe` 成功；GET 安装包含 nosniff + attachment；ICON 无 attachment；路径穿越保持 404。

##### P0-F06 OIDC 测试端点收敛 + DNS rebinding 消除

- `backend/internal/server/setup.go` / `RegisterSetupRoutes`：
  - 新增 `POST /api/setup/oidc/test`，仅系统未配置时匿名可用；已配置返回 404/403 不再匿名；
  - 保留 `ratelimit_oidc_test` 10/min；管理员专用 `/api/admin/settings/oidc/test` 不变。
- `backend/internal/server/oidc.go`：删除原 `/api/oidc/test` 匿名入口，或仅保留内部测试函数供 Setup/Admin 复用。
- `backend/internal/oidc/ssrf.go`：
  - `validateOIDCURL` 改为只做 URL 语法/HTTPS/host 校验，不再 DNS 预解析；
  - `Service.httpCli.Transport.DialContext` 作为唯一拨号点：`LookupIPAddr` 后拒绝任一非公网 IP，并直接向解析出的 IP 拨号；
  - `CheckRedirect` 只做语法与次数限制，实际目标由 DialContext 兜底。
- `backend/internal/oidc/oidc.go` 中 `fetchDiscoveryWithParams`、`verifyClientCredentials` 改调用语法校验版。
- `frontend/src/api/oidc.ts`：`oidcTest` 路径改为 `/setup/oidc/test`。
- 测试：系统已配置时匿名调用拒绝；未配置时匿名可测且限流；DNS 混合公网/私网、重定向到私网/HTTP 均拒绝。

##### P0-F07 日志脱敏补全 + 重置链接 fragment + L09 IP

- `backend/internal/log/log.go`：
  - 重写 `Redact`：路径段 `/reset/<token>` 大小写不敏感替换；查询串按分段解析，`token/code/state` 键名经 `url.QueryUnescape` 后命中即替换值；
  - 保留字符串正则兜底；`RedactHandler` 继续统一覆盖 stdout/缓冲/panic 日志。
- `backend/internal/server/server.go`：
  - `requestLogger` 增加 `ip` 字段：`c.ClientIP()`（与 F09 信任策略一致）；
  - `panicRecovery` 继续记录 `path`，确保经脱敏。
- `backend/internal/auth/reset.go`：`resetLink` 改为 `/reset#token=<url.QueryEscape(token)>`。
- `frontend/src/router/index.ts`：移除 `/reset/:token`，新增 `/reset`（公开）。
- `frontend/src/views/ResetView.vue`：从 `route.hash` 解析 token，读取后用 `history.replaceState` 清除；API 调用不变。
- `backend/internal/server/emergency_gate.go`：`isSPAPath` 增加 `path == "/reset"`，并按 D-F07 移除旧 `/reset/` 前缀白名单。
- 测试：`log.Redact` 覆盖 `/reset/TOKEN`、`?token=`、`?TOKEN=`、`?code=`、`?state=`、编码/大小写变体；前端 ResetView 覆盖 fragment 读取与清理。

##### P0-F08 HTTP 超时 + 请求体限制（配置化）

- `backend/internal/config/admin.go`：
  - 扩展 `RateLimitSettings` 增加 5 个字段（`http_read_header_timeout_sec`、`http_read_timeout_sec`、`http_write_timeout_sec`、`http_idle_timeout_sec`、`http_max_body_mb`）；
  - 新增持久化键与默认值；`GetRateLimit`/`SaveRateLimit` 支持新字段，旧前端缺省时取默认值（建议新字段用指针或“0=未填写”兼容）。
- `backend/internal/server/`（建议新增 `server/hardening.go`）：
  - 构造共享 `http.Server` 配置，普通与应急模式共用；
  - 从 `system_config` 读取超时值设置 `ReadHeaderTimeout/ReadTimeout/WriteTimeout/IdleTimeout`；
  - 新增请求体限制中间件，按 `c.FullPath()` 分级：
    - 默认 API：`http_max_body_mb`（默认 4 MiB）；
    - 订阅/规则/自定义/分享版本上传：55 MiB；
    - 安装包上传：320 MiB；
    - `/api/setup/import` 与 `/api/admin/settings/import`：豁免（维持部署边界）。
  - 超限统一 413，识别 `http.MaxBytesReader` 错误避免落 500。
- 长传输豁免：
  - SSE `/api/admin/logs/stream`、备份下载、配置导出、`/public/*` 下载：清 write deadline；
  - 安装包/版本文件上传：清 read deadline；
  - 普通 API 不豁免。
- `frontend/src/api/settings.ts` / `frontend/src/views/admin/SettingsView.vue`：
  - 速率限制卡片增加连接防护字段；
  - 保存时提示超时字段需重启容器生效，`http_max_body_mb` 即时生效。
- `frontend/src/api/request.ts`：`defaultMsg` 增加 413 文案。
- 测试：慢头断开；4 MiB JSON 413；安装包/50MB 上传不回归；SSE/备份/导出不断开；面板新字段持久化与旧字段兼容。

##### P0-F09 TRUST_PROXY 显式 CIDR + X-Forwarded-Proto 同口径

- 新增 `backend/internal/proxytrust/proxytrust.go`：
  - 支持 `auto/on/off/cidr`；
  - `cidr` = 显式 `TRUST_PROXY_CIDRS` + `127.0.0.1/8`、`::1/128`；
  - 提供 `Trusted(remoteAddr)`、`TrustedProxies()`、`Parse` 等。
- `backend/cmd/server/main.go`：
  - 读取 `TRUST_PROXY_CIDRS` 并随 `TRUST_PROXY` 一起传给 `server.New`、`server.NewEmergency`、`setup.NewService`；
  - 非法值启动失败。
- `backend/internal/server/server.go`：
  - `applyTrustProxy` 改用 `proxytrust` 策略；
  - 支持 `cidr` 档位，`off` 忽略 CIDR，`on` 保持全信任。
- `backend/internal/setup/setup.go`：
  - `trustedForwarded` 改用 `proxytrust`；
  - `DeriveFrontendURL` 只有在可信来源时才接受 `X-Forwarded-Proto/Host`。
- `backend/internal/server/settings.go`：`getRateLimit` 返回 `TRUST_PROXY_CIDRS` 生效摘要（条目数/原始列表）。
- 测试：cidr 内/段外/回环 XFF 生效；不可信 XFP 不改变 scheme；非法 CIDR 启动失败。

#### 8.17.3 P1 批：L01、L02、L04、L08、L09

##### P1-L01 OIDC 一次性 ticket + HttpOnly Cookie 换票

- 新增 `backend/migrations/1013_oidc_login_tickets.sql`：建表 `oidc_login_tickets`（ticket 主键、session_token、expires_at、created_at），并建过期索引。
- `backend/internal/server/oidc.go`：
  - 回调登录成功后不再 302 携带 `?token=`，改为生成 ticket、写入上表、设置 `HttpOnly + SameSite=Lax` Cookie（名 `oidc_login_ticket`，Path 限定 `/api/auth/oidc/exchange`），302 到 `/login/callback`；
  - 新增 `POST /api/auth/oidc/exchange`：读取 Cookie → 事务内查/删 ticket（严格一次性）→ 返回 `{token, expires_at}`；失败 401。
  - 写入前顺带删除过期 ticket。
- `frontend/src/api/oidc.ts`：新增 `exchangeOidc`。
- `frontend/src/views/OidcCallbackView.vue`：`onMounted` 调 exchange，成功写 store 跳首页；失败跳 `/login?oidc_error=exchange_failed`；不再读取 `?token=`。
- 测试：回调 Location 无 token；exchange 首次成功、二次 401、过期 401；Cookie 属性断言；前端成功/失败分支。

##### P1-L02 配置导入“认证死锁”校验

- `backend/internal/config/export.go`：
  - 新增纯函数 `ValidateImportedAuthUsable(cfgMap map[string]string) error`：
    - `configured != "true"` 跳过；
    - `allow_local_login` 缺省按 true；非法布尔拒绝；
    - local 关闭时要求 `oidc_provider_type` 非空且对应 `oidc_params_<type>` JSON 的 `base_url`、`client_id`、`client_secret` 非空，否则返回 `ErrAuthDeadlock`。
  - 在 `Import`（v1）和 `importV2`（v2）解密成功、事务前调用；正常导出文件应通过。
- `backend/internal/server/settings_ops.go`：导入路径把 `ErrAuthDeadlock` 映射为 400。
- 测试：本地关+OIDC 可用成功；本地关+OIDC 参数缺/secret 空 400 且零写入；本地开成功；`configured=false` 跳过；非法布尔 400。

##### P1-L04 反代 TLS 下 Secure Cookie

- `backend/internal/server/`（建议与 F09 共用 `proxytrust`）新增 `requestIsSecure(c, policy)`：
  - 判定顺序：`TLS != nil` → 可信 `X-Forwarded-Proto=https` → `frontend_url` 为 https。
- `backend/internal/server/oidc.go`：
  - login/bind 的 state Cookie 改用该判定；
  - L01 ticket Cookie 同样使用该判定。
- 测试：TLS 直连 Secure；可信反代+XFP=https Secure；不可信+XFP=https 不 Secure；frontend_url=https 兜底 Secure。

##### P1-L08 过期重置令牌每日清理

- `backend/internal/cron/`（新增或扩展 cleanup.go）：`StartResetTokenCleanup(db, lg)`，启动立即执行一次，随后每 24 小时执行；删除 `expires_at < now OR used = 1`；返回 stop。
- `backend/cmd/server/main.go`：与访问日志清理并列启动，defer stop。
- 测试：插入过期/已用/未过期记录，清理后仅保留未过期未用；stop 后不再执行。

##### P1-L09 请求日志 IP（已在 P0-F07 覆盖）

- 按 §8.12：`requestLogger` 增 `ip`；README 补充日志含客户端 IP 的合规提示（P2 文档步落地）。

#### 8.17.4 P2 批：L05、README/compose 补充

##### P2-L05 前端工具链升级 + npm audit CI 门禁

- `frontend/package.json` / `frontend/package-lock.json`：
  - Vite 7 最新稳定版，Vitest 同步升级且 `>=3.2.6`；
  - `@vitejs/plugin-vue`、`vue-tsc`、`@vue/test-utils` 等适配 Vite 7；
  - 让 `esbuild >=0.25.0`、`nanoid >=3.3.18` 由依赖树自然解析；必要时用 `npm ls`/`overrides` 定位残留。
- `.github/workflows/docker-build.yml`：增加 `npm audit --audit-level=high` 门禁（或在前端构建 job 中执行）。
- `.smoke-test.sh`：同步加入 `npm audit --audit-level=high` 检查。
- 验证：`npm audit --package-lock-only`、`npm run build`、`npm test`、Docker build；剩余高危若无法消除需用户签字。

##### P2-README/compose 与文档同步

- `README.md`、`docker-compose.yml`、`docker-compose.yml.example`：
  - 补充 F02/F04/F05/F10/F11 部署边界提示；
  - 环境变量表增加 `TRUST_PROXY_CIDRS`（`cidr` 档语义、Cloudflare/EdgeOne 示例）；
  - 快速开始示例建议显式 `APP_MODE: prod` 与回环/反代二选一；
  - 补充“日志含客户端 IP，按当地合规要求管理日志留存”；
  - 补充 413 与连接防护配置说明。
- `AGENTS.md`：按 §8.5 同步 §4.8 错误码表（新增 413）与安全相关默认值说明。
- 文档闭环：当前 Issue/Build 文档按批次更新；SecurityReport1 总表与本节状态同步。

#### 8.17.5 建议落地顺序与批次验收

- **P0 建议顺序**：F03 → F01/F06（OIDC 相关）→ F07/L09 → F09 → F08；或按依赖分为“静态/文件安全”与“OIDC/服务加固”，但必须在进入 P1 前完成 F03 与 F07，降低同源 XSS 与日志泄露面。
- **P1 建议顺序**：L01（依赖 P0 的 F01/secure 判定）→ L04 → L02 → L08 → 回归 L09。
- **P2 最后**：L05 工具链 + README/compose/AGENTS 同步。
- 每个批次完成后执行 §8.0 通用验证，并按 §8.16 矩阵勾选；全部通过后再进入下一批。



---

### 8.18 修复落地与验证记录（2026-08-25）

| 项 | 状态 | 验证说明 |
|---|---|---|
| F01 OIDC 验签 | ✅ 已修复 | 使用现有 `golang-jwt/jwt/v5` + JWKS 实现验签，校验 iss/aud/exp/nonce/azp；新增迁移 `1012_oidc_nonce.sql`；`go test ./...` 通过 |
| F03 安装包附件下载 | ✅ 已修复 | 危险扩展名拒绝、`securityHeaders` nosniff、attachment、前端 noopener；`go test`、前端 build/test 通过 |
| F06 Setup 测试端点收敛 | ✅ 已修复 | 新增 `/api/setup/oidc/test`，删除匿名 `/api/oidc/test`；DNS 解析收敛到 DialContext 单次 pin |
| F07 日志脱敏/重置 fragment | ✅ 已修复 | `log.Redact` 覆盖 path/query/code/state/大小写/编码；重置链接改 `/reset#token=`；`ResetView` 读 fragment |
| F08 HTTP 超时/请求体限制 | ✅ 已修复 | 新增 `server/hardening.go`，连接超时配置化 + 分级 body 限制 + 长传输豁免；面板配置同步 |
| F09 TRUST_PROXY CIDR | ✅ 已修复 | 新增 `internal/proxytrust`，支持 `auto/on/off/cidr`；server/setup/settings 共用同口径 |
| L01 ticket 换票 | ✅ 已修复 | 新增迁移 `1013_oidc_login_tickets.sql`；回调不再携带 token，HttpOnly Cookie + POST exchange |
| L02 导入认证死锁校验 | ✅ 已修复 | `ValidateImportedAuthUsable` 在 v1/v2 导入事务前执行，返回 400 |
| L04 Secure Cookie | ✅ 已修复 | `requestIsSecure` 按 TLS > 可信 XFP > frontend_url 判定，state/ticket Cookie 统一使用 |
| L08 重置令牌每日清理 | ✅ 已修复 | `cron.StartResetTokenCleanup` 启动即清 + 每日清理，main 已接线 |
| L09 请求日志 IP | ✅ 已修复 | `requestLogger` 增加 `ip=c.ClientIP()`，与 F09 信任策略一致 |
| L05 前端工具链升级 | ✅ 已修复 | Vite 7.3.6 + Vitest 4.1.11 + `@vitejs/plugin-vue` 6.0.8 + `vue-tsc` 3.3.11 升级完成；`nanoid` 经 overrides 提升至 3.3.18；`npm audit --audit-level=high` 为 0 漏洞；本地 smoke 脚本已加入审计门禁；前端 build/test 通过 |
| README/compose/AGENTS 同步 | ✅ 已修复 | 补充部署边界、`TRUST_PROXY_CIDRS`、日志合规、413 错误码等 |

**验证记录：**

- `cd backend && go test ./...`：全部通过（37 个包 ok）。
- `cd backend && go vet ./...`、`go build ./...`：通过。
- `cd frontend && npm run build`：通过（Vite 7.3.6，3398 模块正常转译）。
- `cd frontend && npm test -- --run --no-cache`：通过（19 个测试文件，65 个测试）。
- `cd frontend && npm audit --audit-level=high`：0 漏洞（含 `nanoid` 修复后）。
- 已将新构建的 `frontend/dist` 同步到 `backend/web/dist`，本地 `go run`/测试嵌入的即为最新前端产物。
- Docker 多阶段构建已执行验证（见构建日志）。

---

## 九、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-25 | 初始版本：完成第一期静态网络安全审计，逐项与用户确认，记录已确认待修复项、低风险硬化项、不纳入/部署边界项。 |
| v1.1 | 2026-08-25 | 新增第八章修复方案研究：逐项形成 F01/F03/F06/F07/F08/F09 与 L01/L02/L04/L05/L08/L09 的推荐方案、副作用评估、测试点与待决策清单；本次仅更新文档，未修改代码。 |
| v1.2 | 2026-08-25 | 使用提问工具与用户逐项确认 D-F01～D-L08 及 D-F03-1/D-F09-1，将确认结果回写 §8.15 与各小节方案、§8.16 验证矩阵；新增 413 错误码、cidr 档位、危险扩展名拒绝、Vite 7 升级等确认口径。 |
| v1.3 | 2026-08-25 | 新增 §8.17：按 §8.14 的 P0→P1→P2 顺序形成文件级实施计划，包含涉及文件、迁移、改动点、测试与验收；仅更新文档，未修改代码。 |
| v1.4 | 2026-08-25 | 落地 P0/P1/P2（L05 当时仅完成 CI/文档部分，依赖升级留待 v1.5 在可联网环境补做），新增 §8.18 修复与验证记录；后端 build/vet/test 全绿，前端 build/test 在临时镜像工作区全绿。 |
| v1.5 | 2026-08-25 | 在可联网环境完成 L05 剩余项：升级 Vite 7.3.6 / Vitest 4.1.11 / @vitejs/plugin-vue 6.0.8 / vue-tsc 3.3.11，并通过 overrides 修复 nanoid 至 3.3.18；npm audit 0 漏洞；同步 frontend/dist 到 backend/web/dist；补充本地 smoke 审计门禁。 |
