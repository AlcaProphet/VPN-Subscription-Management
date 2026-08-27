# SecurityReport2.md — VPN 订阅管理系统 网络安全静态审计报告（第二期）

> **文档定位：** 本文档记录对本项目的**第二期静态网络安全审计**结果、逐项用户确认结论与修复方案研究。编码强要求仍见 [AGENTS.md](../../../AGENTS.md)；第一期审计与修复记录见 [SecurityReport1.md](SecurityReport1.md)（已存档核查）。
> **审计方式：** 静态代码/文档/依赖清单审查 + `govulncheck` 可达性漏洞扫描 + `npm audit`；**未构建、未运行项目，未修改任何代码**。
> **审计基线：** 2026-08-27 当前工作区（含 SecurityReport1 全部修复落地后的代码状态）。
> **威胁模型假设（用户指定）：** 项目运行于 NGINX 反代之后，前置 WAF/Cloudflare/EdgeOne 类服务；自部署小规模（5-50 人）、非商业、需公网访问。审查**不考虑**部署者自行管控的安全边界（白名单、本地/内网访问情景）；**不考虑** Xray gRPC 公网明文传输（Xray 设计决定，已由部署者白名单保证）。审查视角以 **OWASP Top 10** 为主。

---

## 一、审计范围与方法

### 1.1 范围

- 后端：`backend/` 全部 Go 源码（约 120 个文件）、迁移 SQL、Dockerfile。
- 前端：`frontend/` 的 Vue 3/TS 源码、CSP、会话存储与渲染面。
- 部署：`docker-compose.yml.example`、Dockerfile、GitHub Actions 流水线。
- 设计：`AGENTS.md`、`Design2.md`、存档的 `Design1.md`。
- 依赖：`govulncheck ./...`（Go 模块 + 标准库可达性分析）、`npm audit --package-lock-only`。

### 1.2 方法

- 人工静态走读：认证/会话/权限、密码重置、验证码、限流、下载链路、Setup/OIDC、应急模式、备份/导入导出、加密存储、日志脱敏、静态资源与路径穿越。
- 并行专项审查：装配渲染注入面（Clash/SR/素材池）、前端 XSS 与会话存储面。
- 与 SecurityReport1 的 12 项修复逐项回归核验。
- 已知排除：Xray gRPC 明文（设计决定）；部署者自管的白名单/本地访问边界。

### 1.3 总体结论

**项目安全基线良好且在第一期基础上持续收敛。** 第一期 12 项修复全部核验到位（OIDC 验签链完整、一次性换票、路径穿越防护、`no-store`、双层中间件、实时查库 + `credential_version`、HKDF+AES-256-GCM、Argon2id 导出、重置链接 fragment、请求体分级限制、`TRUST_PROXY` 四档、日志脱敏完整）；SQL 全参数化，无模板/命令注入面；`npm audit` 0 漏洞；前端 XSS 面收敛（唯一 `v-html` 已加固，用户可控数据全插值转义）。

本轮新发现 **1 项中高危（依赖组件漏洞，govulncheck 9 项可达）** 与 **6 项中低风险硬化项**，已逐项与用户确认处置方式（见 §三 与 §九）。无高危逻辑漏洞。

---

## 二、第一期修复回归核验（12/12 到位）

| 编号 | 第一期问题 | 本次核验结论 |
|------|-----------|--------------|
| F01 | OIDC id_token 未验签 | ✅ `oidc/verify.go`：JWKS 验签 + RS/ES/PS/EdDSA 算法白名单（拒 none/HS）+ iss/aud/exp/nonce/azp 全校验 + JWKS 轮换重试；`resolve.go` 邮箱自动合并要求 `email_verified` 且 `oidc_subject IS NULL` 条件更新，接管链已关闭 |
| F03 | `/public/installers` 持久 XSS | ✅ `static.go`：安装包强制 `attachment`（`mime.FormatMediaType` 构造）+ 全局 `nosniff`；路径穿越双重校验（`..`/绝对路径 + 前缀比对） |
| F06 | OIDC 测试端点匿名可达 | ✅ 仅 `/api/setup/oidc/test`（未配置时，10/min 限流）与管理员专用端点；SSRF 防护收敛至单次拨号 |
| F07 | 日志泄漏敏感参数 | ✅ `log/log.go` `Redact`：`/reset/<token>`、`token/code/state` 键名大小写不敏感 + URL 解码覆盖；重置链接改 `/reset#token=` 且前端读后清除 |
| F08 | HTTP 无超时/请求体限制 | ✅ `hardening.go`：四项超时配置化 + 按路由分级请求体限制（4/55/320 MiB，导入豁免）+ 413 + 长传输逐端点豁免 |
| F09 | TRUST_PROXY 口径 | ✅ `internal/proxytrust` 四档（auto/on/off/cidr）+ `X-Forwarded-Proto` 同口径信任 |
| L01 | OIDC 回调 token 入 URL | ✅ 一次性 ticket + HttpOnly Cookie 换票（迁移 1013） |
| L02 | 导入绕过认证死锁校验 | ✅ `ValidateImportedAuthUsable` 于 v1/v2 导入事务前执行，映射 400 |
| L04 | 反代下 Secure Cookie | ✅ `requestIsSecure`：TLS > 可信 XFP > frontend_url 三级判定 |
| L05 | 前端工具链漏洞 | ✅ `npm audit --package-lock-only` 复核 **0 漏洞**（2026-08-27） |
| L08 | 过期重置令牌无清理 | ✅ `cron.StartResetTokenCleanup` 启动即清 + 每日清理 |
| L09 | 请求日志无 IP | ✅ `requestLogger` 含 `ip = c.ClientIP()`，与信任策略同口径 |

---

## 三、本轮新确认待修复项

### N01【中高 · A06】Go 标准库与第三方依赖存在 9 项可达漏洞

- **证据：** `govulncheck ./...`（2026-08-27）报告“代码受影响 9 项”，且存在真实调用链（Example traces）：

| 漏洞编号 | 组件 | 现状 | 修复版本 | 可达路径摘要 |
|---------|------|------|---------|--------------|
| GO-2026-6218 | net/url（stdlib） | go1.26.4 | go1.26.6 | OIDC 交换流程 `http.Client.Do → url.Parse`，resolvePath 二次复杂度 DoS |
| GO-2026-6091 | html/template（stdlib） | go1.26.4 | go1.26.6 | `http.Server.ListenAndServe` 错误页渲染链 |
| GO-2026-6090 | crypto/tls（stdlib） | go1.26.4 | go1.26.6 | HTTP 服务/邮件/OIDC 出站全部 TLS 握手链 |
| GO-2026-6089 | net/http（stdlib） | go1.26.4 | go1.26.6 | 未加密 HTTP/2 探测时 ReadHeaderTimeout 不生效（削弱 F08 慢头防护） |
| GO-2026-5972 | encoding/asn1（stdlib） | go1.26.4 | go1.26.6 | 邮件 `tls.Dial → asn1.Unmarshal` 递归深度 |
| GO-2026-5856 | crypto/tls（stdlib） | go1.26.4 | go1.26.5 | ECH 隐私泄漏 |
| GO-2026-5026 | golang.org/x/net/idna（经 net/http） | go1.26.4 | go1.26.6 | OIDC 出站 Punycode 标签 |
| GO-2026-6061 | google.golang.org/grpc | v1.79.3 | v1.82.1 | **xray gRPC 客户端** HTTP/2 传输实现（`client.go` 实际调用） |
| GO-2026-5676 | github.com/quic-go/quic-go | v0.59.0 | v0.59.1 | HTTP/3 QPACK Trailer 内存耗尽（间接依赖） |

- **影响：** 以本项目“公网可达 + 前置 WAF”的部署形态，net/http 与 crypto/tls 项直接暴露于公网入站流量；grpc 项暴露于后端到 Xray 的出站链路。多数为 DoS/拒绝服务类，单个利用价值有限，但组合面真实存在。
- **用户确认：** 纳入修复——Dockerfile/CI 升级 Go ≥ 1.26.6，`go.mod` 升级 grpc ≥ v1.82.1 与 quic-go ≥ v0.59.1，并将 `govulncheck` 纳入 CI 门禁（修复方案见 §8.1）。

### N02【中 · A02/A04】备份下载输出未加密的完整数据库

- **位置：** `backend/internal/backup/backup.go:31`（`CreateBackup`）、`backend/internal/server/settings_ops.go:154`（`backup`）。
- **现象：** `GET /api/admin/settings/backup` 流式输出未加密 `tar.gz`，内含 `VACUUM INTO` 一致性快照（完整 `app.db`：密码哈希、会话签名密钥、OIDC client_secret 密文、用户下载 Token、Xray 凭据密文、系统配置）与全部内容文件。
- **对比：** 配置导出（`/export`）已采用“导出密码 → Argon2id → AES-256-GCM”整体加密，备份却完全无保护；备份文件一旦离开受控环境（误存网盘/转发邮件/备份介质失窃）即全量泄露。
- **缓解现状：** 端点受会话 + 管理员双中间件保护；下载链路走反代 HTTPS。属于“管理员自持文件”的纵深防御缺口。
- **用户确认：** 纳入加密——复用导出路径的 Argon2id + AES-256-GCM 方案，下载时要求输入密码（修复方案见 §8.2）。

### N03【中 · A04】验证码 fail-open（第一期遗留未处置项）

- **位置：** `backend/internal/captcha/captcha.go:51-56`（`Enforced`）。
- **现象：** 验证码提供商已启用且页面已勾选时，若 `captcha_secret_key` 缺失，仅记 warn 并**跳过校验**放行请求。管理员配置失误（清空密钥、导入缺字段）会静默关闭验证码，登录/注册/找回密码退回无验证码状态，削弱防暴力破解纵深。
- **用户确认：** 改 fail-closed——密钥缺失视为配置错误，强制校验的页面直接拒绝请求（修复方案见 §8.3）。

### N04【低 · A04】密码重置端点无限流 + 重置令牌明文落库

- **位置：** `backend/internal/server/auth.go:31`（`/api/auth/reset` 路由无限流）、`backend/internal/auth/reset.go:71-73`（令牌明文 INSERT）。
- **现象：** 重置端点仅受一次性令牌保护，无速率限制；令牌以明文存储（虽有 1 小时 TTL + 用后即删 + 每日清理）。令牌 256 位熵下暴力破解不可行，但缺少“限流 + 哈希存储”的常规纵深。
- **用户确认：** 两项都纳入——加按 IP 限流；令牌改哈希（SHA-256）落库（修复方案见 §8.4）。

### N05【中 · A02/A07】前端会话 JWT 存于 localStorage

- **位置：** `frontend/src/stores/auth.ts`、`frontend/src/api/request.ts:12-13`（`localStorage.getItem('token')` → `Authorization: Bearer`）。
- **现象：** 会话凭据持久化于 `localStorage`：同源任何 XSS（含未来第三方依赖引入的漏洞）可直接读取并跨标签长期窃取；7 天“记住我”窗口放大暴露面。当前同源 XSS 面已收敛（见 §五 正面清单），该项属纵深防御。
- **用户确认：** 改 HttpOnly Cookie，**双通道并存**——保留 `Authorization: Bearer` 认证（非浏览器调用方不受影响），浏览器前端改用 `HttpOnly + SameSite=Lax` Cookie，并新增 CSRF 防护（修复方案见 §8.5）。

### N06【低 · A05】安全响应头缺口：点击劫持防护与 CSP 细节

- **位置：** `backend/internal/server/static.go:18`（`securityHeaders` 仅含 `nosniff`）、`frontend/index.html:9`（CSP meta）。
- **现象：**
  1. 无 `X-Frame-Options`/`frame-ancestors`（meta CSP 无法表达 `frame-ancestors`），页面可被第三方嵌入实施点击劫持；
  2. CSP 缺 `object-src 'none'` 与 `base-uri 'none'`；
  3. `connect-src` 放行任意主机 `ws: wss:`（仅为 Vite dev HMR，生产无 WebSocket 请求）。
- **用户确认：** 全部纳入——后端全局补 `X-Frame-Options: DENY`；CSP 补 `object-src 'none'`、`base-uri 'none'`，生产构建移除 `ws:`（修复方案见 §8.6）。

### N07【低 · A09】纯 OIDC 用户首次设置本地密码无通知

- **位置：** `backend/internal/server/profile.go:78`（`updatePassword`）、`user.Service.UpdatePassword`。
- **现象：** 无本地密码的 OIDC 用户首次设密码免旧密码（无密码可验，既定设计）。若会话被窃，攻击者可设密码并借 `credential_version` 递增锁掉原主。缺少事后发现能力。
- **用户确认：** 增加邮件通知——密码（新）设置成功后向账号邮箱发送通知（修复方案见 §8.7）。

---

## 四、维持原决策的部署边界项（本期复核确认）

以下项在第一期经用户确认“不纳入代码修复”，本期复核确认代码状态未变、**全部维持原决策**：

| 编号 | 项目 | 复核结论 |
|------|------|---------|
| 边界-1 | 素材池 URL 同步 SSRF 无内网拦截 | `pool/sync.go:240` 仍为普通 `http.Client`（仅 Timeout）。**仅管理员可配置**、50MB 内容上限、响应不回显公网；内网默认可信由部署者决策。维持 |
| 边界-2 | 装配 SR conf 头部参数/手动规则控制字符注入 | `render_sr.go` 拼接未校验换行（已知）；装配端点受会话+管理员双中间件，攻击者须已是管理员。维持 |
| 边界-3 | 导入文件无应用层大小上限 | `settings_ops.go:110` 读入内存无上限，由反代 `client_max_body_size` 控制（D-F08-2 口径）。维持 |
| 边界-4 | Xray gRPC 明文传输 | `xray/client.go:46` `insecure.NewCredentials()`。Xray 设计决定 + 部署者白名单保证，用户明确排除。维持 |
| 边界-5 | Setup 阶段资源限制 / Dev compose / HTTPS 强制 / 平台 scheme | 第一期 F02/F04/F05/F10 部署边界口径，README 已提示。维持 |
| 边界-6 | CI/镜像未固定 digest（L06） | 第一期“后续再考虑”。维持 |

---

## 五、OWASP Top 10 逐项对照

| 类别 | 结论 |
|------|------|
| A01 访问控制失效 | ✅ 达标。管理端点一律会话+管理员双中间件（高级功能再叠加 `AdvancedMode` 实时查库）；权限实时查库不缓存；`credential_version` 比对使密码变更/禁用即时踢出会话；无效下载 Token 统一 404 不泄露存在性；首管理员空表判定全程 `BEGIN IMMEDIATE` 事务串行化，无竞态；`/public` 与版本文件路径穿越防护完整 |
| A02 加密机制失效 | ⚠️ 基本达标，2 项纳入修复。敏感配置 HKDF(signing_key)+AES-256-GCM；导出/导入 Argon2id+GCM；bcrypt 密码；令牌均 256 位熵。**缺口：备份未加密（N02）、前端凭据存 localStorage（N05）** |
| A03 注入 | ✅ 达标。SQL 全参数化（含动态表名白名单）；无 `os/exec`、无模板引擎执行用户数据；Clash YAML 走 `yaml.Node` 序列化；素材池条目/节点名/邮箱均拒控制字符；唯一遗留为管理员自管范围的 SR conf 拼接（边界-2） |
| A04 不安全设计 | ⚠️ 基本达标，2 项纳入修复。验证码 fail-open（N03）、重置端点无限流（N04）；防枚举响应（登录/忘记统一措辞）、应急操作码一次性+恒定时间比较、死锁防护（本地登录关闭需 OIDC 可用）等设计到位 |
| A05 安全配置错误 | ⚠️ 基本达标，1 项纳入修复。HTTP 超时/请求体分级限制、`TRUST_PROXY` 四档、panic 统一 500、5xx 脱敏、非 root 容器、单一数据卷；**缺口：点击劫持防护与 CSP 细节（N06）** |
| A06 过时易受攻击组件 | ❌ 本期主要新发现。govulncheck 9 项可达漏洞（N01），npm audit 0 漏洞 |
| A07 身份识别与认证失效 | ✅ 达标。OIDC JWKS 验签 + nonce/azp + 一次性 ticket 换票；密码重置一次性 + 递增凭据版本；应急模式操作码严格一次性 |
| A08 软件与数据完整性失效 | ✅ 达标。导入文件密码+确认词+认证死锁校验+签名密钥保护；前端产物哈希命名 + immutable；CI 镜像 digest 固定为已记录的后续项（边界-6） |
| A09 日志与监控失效 | ⚠️ 基本达标，1 项纳入修复。请求日志含 IP、全链路脱敏（`token/code/state` 含编码变体）、500 详情仅 debug 回显、备份/清空/导入记审计日志；**缺口：密码变更无邮件通知（N07）**。管理操作全量审计日志仍为长期方向（第一期 L07） |
| A10 SSRF | ✅ 按边界口径。OIDC 测试端点已做公网 IP 校验 + 单次拨号防 DNS rebinding；素材池同步为管理员自管的部署边界（边界-1） |

---

## 六、正面清单（本期确认保持的安全资产）

1. SQL 全参数化 + 动态表名白名单（`slug.TableHasSlug`）。
2. 管理端点双层中间件 + 高级功能三重闸（会话/管理员/高级模式实时查库）。
3. 权限实时查库不缓存；`credential_version` 使密码/邮箱变更、禁用、角色降级即时失效全部会话。
4. 下载链路：无效/过期 Token 统一 404，业务错误返回 200 纯文本注释块，`no-store` 禁缓存。
5. 令牌体系全部 256 位加密随机熵（下载/分享/规则/重置/操作码）；应急操作码一次性 + `subtle.ConstantTimeCompare`。
6. 首管理员机制单事务串行化（本地注册与 OIDC 首登同口径），无竞态窗口。
7. 并发安全：版本切换/令牌复用键/令牌轮替全部 `BEGIN IMMEDIATE` 事务。
8. 前端：唯一 `v-html`（MarkdownView）`html:false` + linkify 关闭；订阅预览、Diff、SSE 日志、访问日志全插值转义；`window.open` 全 `noopener,noreferrer`；CSP 无 `unsafe-inline` 脚本。
9. 容器：多阶段构建、非 root（`USER app`）、单数据卷、healthcheck 不触发重启。
10. `npm audit` 0 漏洞；前端已升 Vite 7 + audit CI 门禁。

---

## 七、低风险观察项（已缓解，仅记录）

| 编号 | 观察 | 缓解状态 |
|------|------|---------|
| O1 | SSE 实时日志流短期 Token 置于 URL query（`LogsView.vue`），可能落入反代访问日志 | 严格一次性 + 短期有效；EventSource 无法设请求头属架构约束。**N05 改 Cookie 认证后此面可一并消除**（EventSource 自动携带 Cookie） |
| O2 | `log.Redact` 非查询串兜底正则 `[?&]token=` 大小写敏感 | 主路径（查询串分段解析）已覆盖大小写/编码；兜底仅作用于无 `?` 字符串，实际无敏感场景 |
| O3 | `govulncheck` 另报告 3 项导入包 + 2 项模块级漏洞，但代码不可达 | 随 N01 的 Go/grpc 升级自然消除或持续观察 |

---

## 八、修复方案研究（供后续 Build 承接）

> 本节只形成修复方案与影响评估，**未修改任何代码**。优先级建议：P0 = N01（依赖，影响面最广且改动机械）；P1 = N02、N03、N04、N06、N07（小改动批）；P2 = N05（架构级，单独立项单独验收）。

### 8.1 N01 依赖升级 + govulncheck 门禁

1. **Dockerfile/CI**：`golang:1.26-alpine` 确认点版本 ≥ 1.26.6（实施时以 `docker pull golang:1.26-alpine && go version` 复核；若 alpine 标签尚未带 1.26.6，临时固定带补丁号的完整标签）。
2. **go.mod**：`go get google.golang.org/grpc@latest`（≥ v1.82.1）；quic-go 为间接依赖，执行 `go get github.com/quic-go/quic-go@v0.59.1`（或升级直接依赖使依赖树自然解析）后 `go mod tidy`。
3. **CI 门禁**：`.github/workflows/docker-build.yml` 增加 `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` 步骤，存在代码可达漏洞即失败；`.smoke-test.sh` 同步。
4. **验收**：`govulncheck` 代码可达项归零；`go build ./... && go vet ./... && go test ./...` 全绿（grpc 大版本内升级需重点回归 `internal/xray` 全部测试与实机对账）；Docker 构建成功。

### 8.2 N02 备份下载加密

1. **交互**：备份下载由 `GET` 改为 `POST /api/admin/settings/backup`（body 含 `password` ≥8，与导出同规则），响应 `application/octet-stream` 加密文件；前端备份按钮弹出密码输入框。
2. **算法**：复用 `config/export.go` 的 Argon2id 参数与 `salt ‖ nonce ‖ 密文` 格式，将 tar.gz 流整体加密后输出。建议抽取导出/备份共享的加密助手函数，避免两处漂移。
3. **实现要点**：先在临时区完成 tar.gz 打包（备份含 `VACUUM INTO` 快照，已有临时文件），再整体加密输出；失败清理沿用既有模式。恢复侧：备份恢复目前为部署者手工解包还原数据卷（无应用内恢复入口），加密后需在 README 补充“解密后再还原”的操作说明。
4. **验收**：无密码/错误密码 400；正确密码输出文件可解密还原出与现有备份一致的 tar.gz；备份失败清理路径测试。

### 8.3 N03 验证码 fail-closed

1. `captcha.Enforced` 中 `secret == ""` 分支：由“跳过并放行”改为返回需要校验但密钥缺失的语义；`Verify` 在该状态下返回明确错误（如“验证码服务配置不完整，请联系管理员”），接入层 400。
2. 未启用（`off`/空）或页面未勾选的语义不变；`/api/system/status` 的验证码状态展示需区分“已启用但密钥缺失”（前端可给出配置警示）。
3. **验收**：启用+页面勾选+密钥缺失 → 注册/登录/找回密码全部 400；密钥补齐后恢复；未启用场景不受影响；更新 `captcha_test` 用例。

### 8.4 N04 重置端点限流 + 令牌哈希存储

1. **限流**：`/api/auth/reset` 叠加 `limiter.Middleware("reset", "ratelimit_reset", 5)`，新配置键默认 5/min，面板速率控制区同步暴露。
2. **哈希存储**：写入与校验均用 `SHA-256(token)` 十六进制；令牌 256 位熵下 SHA-256 足够（无需慢哈希——暴力面不存在，哈希仅为库文件/备份泄露时的纵深）。注意迁移：存量未过期令牌（≤1 小时）升级后失效，用户重新申请即可，无需数据迁移兼容层。
3. **验收**：超频 429；明文令牌不再出现在库中；既有重置流程（申请→邮件→重置→递增凭据版本）端到端通过。

### 8.5 N05 会话凭据迁移 HttpOnly Cookie（双通道）

1. **后端双通道**：`SessionMiddleware` 认证来源扩展为——优先 `Authorization: Bearer`（语义完全不变，非浏览器调用方零影响），无 Bearer 时读会话 Cookie。
2. **Cookie 属性**：`HttpOnly`、`SameSite=Lax`、`Secure` 按 `requestIsSecure`（L04 既有判定）设置；`Path=/`；过期时间随签发时长（24h/7d）；登出端点从“客户端语义”升级为同时下发过期 Cookie。
3. **CSRF 防护**：Cookie 通道下所有**状态变更请求**（非 GET/HEAD）要求携带自定义请求头（如 `X-App-Client: 1`），缺失即 403——本项目未开放任何跨域 CORS，自定义头足以阻断表单类 CSRF；`SameSite=Lax` 为第二层。Bearer 通道不检查该头（保持兼容）。
4. **签发点改造**：本地登录/注册/OIDC 换票（`/api/auth/oidc/exchange`）成功后 `Set-Cookie` 下发；响应体仍返回 `expires_at` 但不再返回 `token`（前端改为只依赖 Cookie）；`localStorage` token 读写全部移除，`/api/auth/me` 判断登录态。
5. **收益联动**：SSE 日志流（O1）可改为依赖 Cookie 认证，移除 URL 短期 Token 面。
6. **回归重点**：登录/登出/记住我、OIDC 登录与绑定、401 拦截跳登录、多标签页、反代 HTTPS 下 Secure 判定、Bearer 调用方（smoke 脚本）不受影响；前端 65+ 单测同步改造。
7. **文档**：Design/README 同步认证模型描述（双通道 + CSRF 头约定）。

### 8.6 N06 安全头与 CSP

1. `securityHeaders()` 增加 `X-Frame-Options: DENY`（正常与应急模式均已注册该中间件，一处生效）；同时评估追加 `Referrer-Policy: same-origin`（可选，实施时一并决定）。
2. `index.html` CSP：追加 `object-src 'none'; base-uri 'none'`；`connect-src` 仅在开发构建保留 `ws: wss:`——实施方式为构建期注入（如 vite `define`/模板变量）或生产 meta 直接收紧为 `connect-src 'self'`（Vite dev 不读取打包产物，可直接改）。
3. **验收**：响应头断言；CSP 收紧后前端 build + 全量测试 + 验证码（reCAPTCHA/Turnstile frame-src 不受影响）回归。

### 8.7 N07 密码设置邮件通知

1. `updatePassword` 成功路径（含管理员代改密码 `user.AdminService` 路径，实施时一并评估）在事务提交后异步发送邮件：“你的账号密码已被设置/修改，若非本人操作请立即联系管理员”。
2. 复用 `mail.Service` 与既有 `SetWelcomeSender` 注入模式；SMTP 未配置/失败不阻断主流程（与重置邮件同口径），仅记日志。
3. 邮件内容只含站点名与操作事实，不含任何凭据/链接。
4. **验收**：设置/修改密码触发邮件（mock SMTP）；SMTP 失败不影响改密成功；文案走查。

---

## 九、已确认决策记录（2026-08-27 三轮提问逐项确认）

| 编号 | 决策点 | 确认结果 |
|------|--------|---------|
| D-N01 | 依赖漏洞处置 | 纳入修复：Go ≥1.26.6 + grpc ≥v1.82.1 + quic-go ≥v0.59.1，govulncheck 纳入 CI 门禁 |
| D-N02 | 备份加密 | 纳入：复用导出路径 Argon2id + AES-256-GCM，下载时要求密码 |
| D-N03 | 验证码策略 | 改 fail-closed：密钥缺失时强制校验页面直接拒绝 |
| D-N04 | 重置端点加固 | 两项都纳入：按 IP 限流 + 令牌 SHA-256 哈希落库 |
| D-N05 | 会话凭据存储 | 改 HttpOnly Cookie，**双通道并存**：保留 Bearer（非浏览器调用方），前端改 Cookie + CSRF 自定义头防护 |
| D-N06 | 安全头加固 | 全部纳入：X-Frame-Options: DENY + CSP `object-src 'none'`/`base-uri 'none'` + 生产移除 `ws:` |
| D-N07 | OIDC 首设密码 | 增加邮件通知（事后发现能力） |
| D-边界 | 第一期部署边界遗留项 | 全部维持原决策（素材池 SSRF / SR conf 注入 / 导入大小 / Xray 明文 / Setup 边界 / CI digest） |
| D-报告 | 报告落位 | 新建 SecurityReport2.md；SecurityReport1 保持存档 |
| D-其他 | 下载限流可用性（非安全问题） | 用户明确不处理、不记录，不属本次扫描重点 |

---

## 十、后续动作建议

1. **Build 承接**：按 §八 优先级（P0 = N01 → P1 = N02/N03/N04/N06/N07 → P2 = N05）分步实施，每步执行 AGENTS §3.4 验证命令；N05 建议单独成 Build Step 并附完整回归清单。
2. **CI 常态化**：`govulncheck`（本期新增）与 `npm audit --audit-level=high`（第一期已有）共同构成依赖防线。
3. **长期方向（不排期）**：管理操作全量审计日志（第一期 L07）；CI/镜像 digest 固定（边界-6）。
4. **复审建议**：N05 落地后对本报告 §二/§五 做一次快速回归；依赖扫描建议随每次 Go 点版本发布例行执行。

---

## 十一、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-27 | 初始版本：完成第二期静态审计（OWASP Top 10 视角 + govulncheck + npm audit + 第一期修复回归核验），三轮提问与用户逐项确认，形成 7 项纳入修复项（N01-N07）、6 项维持部署边界项与修复方案研究。本次仅新增文档，未修改代码。 |
