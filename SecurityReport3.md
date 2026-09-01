# SecurityReport3.md — VPN 订阅管理系统第三期完整网络安全审查报告

> **文档定位：** 本报告是 [SecurityScanPlan1.md](SecurityScanPlan1.md) 的单一结果载体，按 Step 追加可复核证据。报告只记录审查结论，不授权在审查 Step 内修改业务代码。
>
> **固定基线：** `beta` / `4958db3d900019eff3f6ffb2373b2b66a5b9a730`，2026-08-30 23:16:56 +0800。固定基线时工作区干净；本报告与计划状态更新属于审查产物，不改变受审业务代码基线。
>
> **主口径：** OWASP Top 10:2025；兼容映射 OWASP Top 10:2021。威胁模型、授权和排除项以 [SecurityScanPlan1.md](SecurityScanPlan1.md) §一为准。

---

## 一、范围、边界与方法

### 1.1 纳入范围

- 后端：`backend/cmd/`、`backend/internal/`、`backend/migrations/`、`backend/web/`。
- 前端：`frontend/src/`、`frontend/tests/`、前端构建/测试配置、`package.json` 与锁文件。
- 数据与交付：SQLite schema、`/data`、`/public`、导入导出、备份、版本文件、Docker、Compose、GitHub Actions 与内嵌前端产物。
- 网络与外部依赖：公开 HTTP/API、OIDC、SMTP、素材 URL、Xray gRPC、可信代理头。
- 设计与历史：AGENTS、当前 Design/Build/Issue、SecurityReport1/2 的全部编号项与边界项。

### 1.2 明确边界

- 不审宿主机、SSH、内核、防火墙、安全组、WAF/CDN 厂商规则、TLS 证书运维或真实第三方服务本身。
- WAF/反向代理仅作为背景条件，不替代应用的认证、授权、输入校验、SSRF、数据保护和日志控制。
- 不访问生产环境，不使用真实数据/凭据，不向第三方发送测试载荷，不执行压力测试、持久化利用或破坏性验证。
- 审查与修复分离：发现只记录、分级和提出建议，不在本报告对应的审查 Step 内修改业务代码。

### 1.3 方法

1. 固定 Git、工具链、文件数量和关键配置哈希。
2. 从入口追踪至 handler/service/store/输出或副作用，并同时记录缺陷与正面控制。
3. 自动化搜索只用于枚举；任何结论均需人工核对反证、既有中间件、事务、编码器、限制器和测试。
4. 动态验证仅使用后续 Step 5 建立的本地隔离实例与合成数据。
5. 依赖扫描结论必须记录扫描器版本和漏洞库时间，并区分代码可达、仅模块存在、dev/build 链与运行时可达。

### 1.4 当前状态词汇

| 状态 | 含义 |
|------|------|
| 已修复 | 当前源码仍存在对应控制，且未发现与历史问题相同的回归证据 |
| 回归 | 历史已修复控制在当前实现中失效或被绕过 |
| 建议未落地 | 历史报告已确认的处置方案尚未进入当前实现 |
| 仍属已确认边界 | 当前实现与历史边界决策一致；不表示本期自动排除，仍须在对应领域 Step 重新评估 |
| 被后续设计取代 | 原问题的前提或行为已被后续已确认设计替换 |
| 证据不足 | 当前 Step 未获得足以确认或排除的静态/动态/时效性证据 |

---

## 二、固定审查基线

### 2.1 Git 与时间

| 项目 | 固定值 |
|------|--------|
| 开始时间 | 2026-08-30 23:16:56 +0800 |
| 分支 | `beta`（跟踪 `origin/beta`） |
| HEAD | `4958db3d900019eff3f6ffb2373b2b66a5b9a730` |
| `git status --short --branch` | `## beta...origin/beta`，无 dirty paths |
| 未暂存/已暂存 diff | 均为空 |
| 临时证据目录 | `/tmp/vpn-security-scan-step1.0HvGdK`；位于仓库外，不入库 |

### 2.2 工具链

| 工具 | 当前版本 | 说明 |
|------|----------|------|
| Go | `go1.26.6 darwin/arm64` | 本机工具链；不代表 Docker 构建镜像的实际补丁版本 |
| Node.js | `v26.7.0` | 本机工具链；CI 与 Dockerfile 当前指定 Node 22 |
| npm | `11.19.0` | 本机工具链 |
| Docker Client / Server | `29.7.2 / 29.7.2` | 本机 Docker 环境 |

### 2.3 跟踪文件计数

计数均基于 `git ls-files`，只描述固定 Git 基线，不把忽略文件或运行时数据计入：

| 类别 | 数量 | 计数口径 |
|------|-----:|----------|
| 全部跟踪文件 | 637 | `git ls-files` |
| 后端 Go 文件 | 192 | `backend/**/*.go`，包含测试 |
| 后端 Go 测试 | 74 | `backend/**/*_test.go` |
| 前端 `src` 文件 | 101 | `frontend/src/**` |
| 前端测试目录文件 | 36 | `frontend/tests/**` |
| SQL 迁移 | 20 | `backend/migrations/*.sql` |
| GitHub Actions workflow | 1 | `.github/workflows/*` |

### 2.4 关键配置 SHA-256

| 文件 | SHA-256 |
|------|---------|
| `AGENTS.md` | `2268065d9da56069f48429a0fe78564d6fcd0deebe8e12039dc8149b0ef5daa5` |
| `Design2.md` | `b2f2caae8678cf267982cc56f1391fd4c14c71ffd6ff8f585c576f09c33839ca` |
| `Design2-UI.md` | `6be86c436cba120f86b5681ff49bb8aa1c7aece5cee937bcb9ebdbb4b6edf31f` |
| `backend/go.mod` | `fb0f48528f6deffe8cecd0c308c6ad5868b8f05577f2ede6839b57f0ef1891ae` |
| `backend/go.sum` | `604995df37c3af2c946bf0bfefe5544b42ab4df3db00630e7644dece760652b4` |
| `frontend/package.json` | `6cde1b80a49ac2cca6561b9bd34abf179b10f7d47974894f434967ab93aeedf5` |
| `frontend/package-lock.json` | `b5418310f326ee40d2a451f482ced3d5448ee1136ab1fe29fd62ea1621b30126` |
| `Dockerfile` | `4aeef2dcdd62295827ca670e5d62afdbce7e3f026574f9bbd206f80a55fc113a` |
| `docker-compose.yml` | `ce6b2b3102df0679601ce2d98a32ccc04c74790ae7718bd43c5dec3cfd4201ca` |
| `docker-compose.yml.example` | `d2334e64f574b376d68b93f19e17d23c56813144ea00550d2dc6a39008622137` |
| `frontend/vite.config.ts` | `fcf28d08901d23e55dc5dfc16759d829b09e4c946df8556db92c1a9d50eaaf64` |
| `frontend/vitest.config.ts` | `cf5fd4dc0915d06ff1cc9a34299f8218b5e3107a9f3e77908251dc0cc735eb48` |
| `.github/workflows/docker-build.yml` | `2bbba72abb9a2f48a63ed5b4e476ac034f8c3a2a6e206c7b7095510430b05b11` |

### 2.5 仓库文件、生成物与运行时数据

- `backend/web/dist/index.html` 是唯一跟踪的内嵌前端占位文件；Docker 构建时由 `frontend/dist` 真实产物覆盖。
- `frontend/dist/` 与 `backend/web/dist/` 的真实构建产物被 `.gitignore` 排除；不能把本机残留产物当作固定代码基线。
- `backend/data/` 被 `.gitignore` 排除，固定基线时未发现其中有文件；本 Step 未清理、创建或读取数据库。
- `frontend/node_modules/` 是忽略的本机依赖环境，不属于固定 Git 基线。Step 2 发现其安装版本落后于锁文件，相关测试结论已单独注明。
- `govulncheck`、`npm audit` 和镜像漏洞库均具有时间敏感性；Step 1–2 未刷新漏洞库或执行新扫描，统一留待 Step 6 记录扫描器版本、数据库时间和可达性判断。

---

## 三、发现编号与严重度

### 3.1 编号规则

- 本期新发现按确认顺序使用 `S3-01`、`S3-02`……，不得复用或重排。
- 历史 `Fxx`、`Lxx`、`Nxx`、`O1–O3` 与“边界-1–6”保持原编号，只记录回归状态，不改编为新发现。
- 同一根因跨多个 Step 出现时只保留一个 S3 编号，其余检查点引用该编号并追加证据。

### 3.2 严重度与置信度

| 严重度 | 本项目口径 |
|--------|------------|
| Critical | 无需高权限即可造成系统级接管、全量敏感数据泄漏或同等级不可逆影响，且现实利用链清晰 |
| High | 可造成管理员/大量账号接管、关键控制绕过或大范围敏感数据泄漏，前置条件有限 |
| Medium | 可造成受限数据/权限影响、可靠拒绝服务或需要特定配置/角色的实质风险 |
| Low | 纵深防御缺口、较强前置条件下的有限影响，或小规模部署下可校准降低的风险 |
| Informational | 正面控制、观察项、运维提醒或未形成漏洞的改进建议 |

置信度使用 High / Medium / Low，分别表示证据链完整、仍需一项关键验证、或主要依赖静态推断。适用时给出 CVSS 向量，但严重度最终以本项目 10–50 人非商业部署语境校准。

### 3.3 当前发现总表

Step 1–2 未扩展新漏洞搜索，暂无 `S3-xx` 新发现。历史未落地项保留原编号，不重复编号。

---

## 四、前两期历史项当前回归矩阵

### 4.1 SecurityReport1 — F01–F11

| 编号 | 当前状态 | 当前实现与证据 | 后续复核 |
|------|----------|----------------|----------|
| F01 | 已修复 | `backend/internal/oidc/verify.go:36-100` 保留非对称算法白名单、JWKS 验签、iss/aud/exp/nonce/azp 校验；`flow.go:189-193` 真实交换链调用 verifier；`resolve.go:36-67` 仅在 `email_verified` 且目标未绑定时条件合并。OIDC 包测试通过，但当前没有伪造签名/none/HS/错声明的直接回归测试。 | Step 9 补恶意 token 与本地 mock 动态链 |
| F02 | 仍属已确认边界（部分缓解） | `server/settings_ops.go:35-37` 已为 Setup 导入增加 5/min 限流，`server_test.go:441-457` 覆盖第 6 次 429；但 `hardening.go:32-35` 继续豁免导入体限，`settings_ops.go:102-119` 仍无上限读入内存。历史“无速率限制”前提已被后续实现部分取代，应用层体限边界仍在。 | Step 8、17、18、21 重新评估，不能仅引用反代 |
| F03 | 已修复 | `platform/platform.go:46-52,416-424` 拒危险扩展名；`server/static.go:17-22,51-72` 全局 `nosniff`、安装包强制附件、路径前缀校验；`HomeView.vue:172` 使用 `noopener,noreferrer`。相关 platform/server 包测试通过，但缺少公开下载响应头与危险扩展名的专门负向测试。 | Step 15、17 动态核对响应头与同源行为 |
| F04 | 仍属已确认边界 | 根 `docker-compose.yml:1-17` 仍为 Dev + `8080:8080`，但文件头有显著警告；生产模板 `docker-compose.yml.example:12-29` 默认为回环绑定 + prod。当前本期边界不自动排除错误使用 Dev 模板。 | Step 7 复核默认值、文档与失败模式 |
| F05 | 仍属已确认边界 | 应用未全局强制 HTTPS；README `169-191,264-265` 要求公网 OIDC 由 HTTPS 反代接入。`server/oidc.go:59-76` 仅负责可信请求的 Secure Cookie 判定，不等于 HTTPS 强制。 | Step 7、9、16 重新校准 |
| F06 | 已修复 | `server/oidc.go:42-54` 仅保留未配置系统可用的 `/api/setup/oidc/test`，10/min 限流且 configured 后 404；管理员测试端点位于受双中间件保护的 settings 路由。OIDC 出站保护的 DNS pin/重定向细节留待专项。 | Step 9、20 动态验证匿名状态与 rebinding |
| F07 | 已修复 | `log/log.go:62-111` 脱敏 reset 路径及大小写/URL 解码后的 token/code/state；`server.go:496-507` 请求日志经过统一 logger；前端 `ResetView` 读取 fragment，相关 reset/log 测试通过。测试当前未直接覆盖大小写、编码 code/state 的完整历史矩阵。 | Step 13、24、27 补全恶意输入回归 |
| F08 | 已修复 | `server/hardening.go:14-53` 配置四项 HTTP 超时与 4/55/320 MiB 分级体限，导入按历史决策豁免；`server.go:85-87,461-463` 正常/应急服务器均接线；413 映射存在。相关 server 测试通过，但慢头和实际连接 deadline 未在本 Step 动态验证。 | Step 7、21 动态验证超时与豁免 |
| F09 | 已修复 | `proxytrust/proxytrust.go:11-109` 保留 auto/on/off/cidr 四档，cidr 自动含回环；`server.go:491-504` Gin 客户端 IP 与日志使用同一策略；`oidc.go:59-76` XFP Secure 判定复用策略。`TestTrustProxyClientIPTiers` 通过，尚缺 cidr + XFP 完整组合测试。 | Step 7、16、21 补可信/不可信头矩阵 |
| F10 | 仍属已确认边界 | `platform/platform.go:314-324` 只拒空值和控制字符，不做协议白名单；README `266` 要求只配可信客户端 scheme。历史部署者边界与本期“管理员功能仍纳入攻击面”不完全一致。 | Step 15 复核浏览器 URL/scheme 注入 |
| F11 | 仍属已确认边界 | `pool/sync.go:231-245` 仍使用普通 `http.Client{Timeout: ...}` 拉取管理员 URL，未见私网地址、重定向或 DNS rebinding 控制。50MB 响应限制与仅管理员入口属于缓解，不替代 SSRF 控制。 | Step 20 专项重新评估 |

### 4.2 SecurityReport1 — L01–L09

| 编号 | 当前状态 | 当前实现与证据 | 后续复核 |
|------|----------|----------------|----------|
| L01 | 已修复 | `server/oidc.go:90-143,182-233` 回调只下发 Path 限定的 HttpOnly 一次性 ticket，重定向不带 token，换票在 `BEGIN IMMEDIATE` 中消费后删除；OIDC 回调前端测试通过。当前无重复换票/跨站 Cookie 的路由级测试。 | Step 9、10 动态验证一次性与 Cookie 属性 |
| L02 | 已修复 | `config/export.go:223-255,324-345,606-642` 在 v1/v2 写事务前执行 `ValidateImportedAuthUsable`，接入层 `settings_ops.go:127-141` 映射为 400；config 包测试通过。现有测试主要覆盖导入往返/回滚，未直接构造 v1/v2 认证死锁文件。 | Step 18、27 补负向导入样例 |
| L03 | 仍属已确认边界 | `assembly/render_sr.go:64-74` 将 `FixedParams` 键值直接拼到 `[General]`；SR 订阅 `status/remarks` 也直接写行。规则值已有 `rulespec.ValidateValue`，但头部固定参数的 CR/LF 注入前提仍存在。 | Step 14 重新评估所有配置/CRLF 注入面 |
| L04 | 已修复 | `server/oidc.go:59-76,85-86,141-142,245-246` 按 TLS > 可信 XFP > frontend_url 设置 state/ticket Cookie 的 Secure 属性。当前缺少三分支直接测试。 | Step 9、10、16 动态验证 |
| L05 | 已修复（时效证据待刷新） | 锁文件声明 Vite 7.3.6、Vitest 4.1.11、plugin-vue 6.0.8、vue-tsc 3.3.11、nanoid 3.3.18；`package.json:23-40` 与 `.github/workflows/docker-build.yml:32-38` 保留升级和 high audit 门禁。本 Step 未运行联网 audit。 | Step 6 用 `npm ci` + 当前漏洞库复核 |
| L06 | 仍属已确认边界 | `.github/workflows/docker-build.yml:25-61` 仍使用 `actions/*@vN`，Dockerfile base image 与生产 `:latest` 未锁 digest。 | Step 6 供应链专项 |
| L07 | 仍属已确认边界 | 当前仅对备份、导入等少数敏感操作写结构化日志，未见覆盖全部管理操作的统一审计设施。 | Step 24 建立管理操作覆盖表 |
| L08 | 已修复 | `cron/cleanup.go:39-62` 启动即清 + 每日清理过期/已使用 reset token；`cmd/server/main.go:118` 接线；`TestResetCleanup` 通过。 | Step 13 回归生命周期 |
| L09 | 已修复 | `server/server.go:496-507` 请求日志记录 `c.ClientIP()`，`log/log.go:62-111` 处理敏感参数；log/server 相关测试通过。历史大小写/编码全矩阵仍未被直接测试覆盖。 | Step 24、27 补恶意编码回归 |

### 4.3 SecurityReport2 — N01–N07

| 编号 | 当前状态 | 当前实现与证据 | 后续复核 |
|------|----------|----------------|----------|
| N01 | 建议未落地 | 本机 Go 已为 1.26.6，但 `backend/go.mod:3,13,46` 仍是 Go 1.26.0、grpc 1.79.3、quic-go 0.59.0；`Dockerfile:10` 仍用浮动 `golang:1.26-alpine`；CI 无 `govulncheck` 门禁。不能用本机版本替代可复现交付修复。 | Step 6 刷新 govulncheck、依赖可达性和镜像版本 |
| N02 | 建议未落地 | `backup/backup.go:30-64` 仍直接写 `tar.gz`，其中包含 `app.db`、`contents/`、`public/`；`server/settings_ops.go:153-162` GET 下载无密码/加密封装。管理员双中间件与 HTTPS 是现有缓解。 | Step 18、19 动态检查备份内容与密码学方案 |
| N03 | 建议未落地 | `captcha/captcha.go:41-57` 在页面已启用但 secret 缺失时 warn 后返回 false；`captcha_test.go:69-78` 还明确断言“密钥缺失跳过校验”。这是可复现的 fail-open 当前行为。 | Step 8、13 动态验证配置错误状态 |
| N04 | 建议未落地 | `server/auth.go:31-32` 只给 `/reset/validate` 限流，真正 `/reset` 仍无限流；`auth/reset.go:82,107,131,162-171` 仍以明文 token 主键查询/更新。256 位熵、1 小时 TTL、用后标记和每日清理是缓解。 | Step 13、19、21 |
| N05 | 建议未落地 | `frontend/src/stores/auth.ts:1-20` 仍将 JWT 放 `localStorage`；`api/request.ts:8-15` 仍注入 Bearer；后端登录/注册/OIDC exchange 仍返回 token，未见浏览器会话 Cookie 与 CSRF 头协议。相关前端测试也断言 localStorage 行为。 | Step 10 单独审查双通道与 CSRF |
| N06 | 建议未落地 | `server/static.go:17-22` 只有 `X-Content-Type-Options`；无 X-Frame-Options/header CSP。`frontend/index.html:7-9` CSP 仍缺 `object-src 'none'`、`base-uri 'none'`，且生产 meta 仍含 `ws: wss:`。 | Step 16 动态核对全部响应类型 |
| N07 | 建议未落地 | `server/profile.go:17-23,77-103` 的 ProfileHandler 未注入邮件服务，OIDC 首设/普通改密成功后直接返回；`profile_test.go:126-140` 只验证免旧密码成功，不验证通知。 | Step 13、24 评估事后发现与失败语义 |

### 4.4 SecurityReport2 — 历史部署边界项

> 第二期报告的边界假设曾排除部分管理员功能和 Xray 明文链；本期计划明确“管理员功能仍纳入攻击面”且 WAF 不能替代应用控制。因此下表的“仍属已确认边界”只表示实现未漂移，不表示本期接受其风险结论。

| 编号 | 当前状态 | 当前证据 | 本期重新评估入口 |
|------|----------|----------|------------------|
| 边界-1 | 仍属已确认边界 | 素材池 URL 同步仍为普通 `http.Client`，未做私网/重定向/DNS 防护；见 `pool/sync.go:231-245`。 | Step 20 |
| 边界-2 | 仍属已确认边界 | SR 固定参数仍可直接拼接配置行；见 `assembly/render_sr.go:64-74`。 | Step 14 |
| 边界-3 | 仍属已确认边界 | 两个导入路由继续豁免全局体限并无上限读入内存；见 `hardening.go:32-35`、`settings_ops.go:102-119`。 | Step 17、18、21 |
| 边界-4 | 仍属已确认边界 | Xray 客户端仍使用 `insecure.NewCredentials()`；见 `xray/client.go:39-47`。 | Step 19、23 |
| 边界-5 | 仍属已确认边界（拆分复核） | Setup/Dev/HTTPS/scheme 的实现与第一期边界大体一致；Setup 导入限流已后续补充。 | Step 7、8、9、15、16、21 |
| 边界-6 | 仍属已确认边界 | Actions、Docker base 和生产 `latest` 仍未固定 digest。 | Step 6 |

### 4.5 SecurityReport2 — O1–O3

| 编号 | 当前状态 | 当前证据 | 后续复核 |
|------|----------|----------|----------|
| O1 | 仍属观察项，缓解未变化 | `LogsView.vue:69-88` 仍先签发 5 分钟一次性 token，再放入 EventSource query；N05 Cookie 迁移未落地，因此 URL 凭据面未消除。 | Step 10、12、24 |
| O2 | 仍属观察项，缓解未变化 | `log/log.go:65-77` 的无 `?` 兜底 `tokenValueRe` 仍大小写敏感；正常查询串路径仍经大小写不敏感的 key 解析。 | Step 24、27 |
| O3 | 证据不足 | 历史不可达漏洞是否仍存在依赖当前漏洞库和可达性分析；grpc/quic 版本仍未按 N01 升级，本 Step 未运行 govulncheck。 | Step 6 |

### 4.6 漂移与测试环境说明

- 受审业务代码相对 Step 1 固定基线没有漂移；Step 2 只新增报告内容。
- 仓库锁文件目标版本为 Vite 7.3.6 / Vitest 4.1.11 / plugin-vue 6.0.8 / vue-tsc 3.3.11 / nanoid 3.3.18。
- 本机忽略目录 `frontend/node_modules` 实际为 Vite 5.4.21 / Vitest 2.1.9 / plugin-vue 5.2.4 / vue-tsc 2.2.12，`npm ls` 返回 `ELSPROBLEMS`。因此本 Step 的前端测试虽通过，但只作为源码回归证据；不能证明锁文件工具链已经在当前本机安装，也不能代替 Step 6 的 `npm ci`、audit 和构建验证。

### 4.7 后续 Steps 必须重新验证的高风险假设

1. F01 的验签实现静态完整，但缺少伪造签名、none/HS、错 iss/aud/exp/nonce/azp 与 JWKS 轮换的直接回归测试。
2. N01 的本机 Go 修复不能代表 Docker/CI 交付；grpc/quic 和 govulncheck 门禁明确未落地。
3. F02/边界-3 的“反代限制导入大小”与 F11/边界-1 的“管理员 URL 可信”不符合本期直接继承条件，必须重评。
4. N05 未落地，浏览器 JWT、SSE query token、CSRF 与 Cookie/Bearer 双通道须作为一条完整凭据链审查。
5. N02–N04、N06–N07 均保留第二期原始行为，后续不能引用“已确认纳入修复”文字作为实际修复证据。
6. 历史定向测试存在覆盖缺口：OIDC 对抗 token、安装包响应头、Secure Cookie 三分支、导入认证死锁文件、日志编码变体均需要对应领域 Step 补齐。

---

## 五、Step 检查点

### Step 1 检查点 — 固定基线、创建报告骨架与证据目录

- 基线：`beta` / `4958db3d900019eff3f6ffb2373b2b66a5b9a730` / 无 dirty paths / 2026-08-30 23:16:56 +0800。
- 实际范围：Git/工具链/跟踪文件数量/关键配置哈希/生成物与运行时数据边界；未读取运行时数据库，未运行漏洞扫描，未修改业务代码。
- 执行命令：`git status --short --branch`、`git rev-parse HEAD`、`git diff --stat`、`git ls-files` 计数、`go version`、`node --version`、`npm --version`、`docker version`、`shasum -a 256 ...`、`mktemp -d /tmp/vpn-security-scan-step1.XXXXXX`。
- 结论：通过。
- 新发现：无。
- 历史项：本 Step 不判定；交由 Step 2。
- 正面控制：Git 工作区干净；审查证据目录位于仓库外；`backend/data` 和真实构建产物均不进入固定基线。
- 未验证项：依赖漏洞库、镜像实际 base digest、运行时数据库/端口、生产或第三方服务；分别留待 Step 5–7 及领域 Steps。
- 证据索引：本报告 §2.1–2.5；`SecurityScanPlan1.md` §一、§五 Step 1；`.gitignore:35,49-52,62`。
- 下一步交接：固定代码基线是 commit `4958db3d...` 且无初始 dirty paths。后续审查产生的 `SecurityReport3.md` 与计划状态变更不算业务代码漂移。所有时效性扫描必须重新记录工具/漏洞库日期，不能沿用 2026-08-27 的“0 漏洞”或“9 项可达”数字。

### Step 2 检查点 — 前两期报告与当前实现逐项回归矩阵

- 基线：同 Step 1；进入 Step 2 时业务代码未变化，审查产物后写入。
- 实际范围：SecurityReport1 的 F01–F11/L01–L09，SecurityReport2 的 N01–N07、边界-1–6、O1–O3；只追踪历史项点名的源码、测试、依赖版本与 CI，不扩展新漏洞搜索。
- 执行命令：历史编号与路径 `rg -n`；相关文件 `nl -ba` 取证；`go test ./internal/oidc ./internal/platform ./internal/log ./internal/server ./internal/config ./internal/cron ./internal/captcha ./internal/backup ./internal/auth`；`npm test -- --run --no-cache tests/auth-store.spec.ts tests/request.spec.ts tests/oidc-callback-view.spec.ts tests/reset-view.spec.ts tests/import-url.spec.ts tests/home-view.spec.ts`；`npm ls vite vitest @vitejs/plugin-vue vue-tsc nanoid --depth=0`；锁文件版本查询。
- 结论：有历史未落地项，无历史已修复项回归。12 个第一期代码修复项仍在；第二期 N01–N07 均未落地；历史部署边界实现未明显漂移，但因本期威胁模型变化必须重评。
- 新发现：无；本 Step 不把历史未落地项重复编号为 S3。
- 历史项：逐项状态见 §4.1–4.5；F01/F03/F06/F07/F08/F09、L01/L02/L04/L05/L08/L09 为已修复；N01–N07 为建议未落地；其余 F/L/边界项仍属历史边界或长期项；O1/O2 仍为观察项，O3 证据不足。
- 正面控制：OIDC 验签链、安装包附件/nosniff/路径校验、Setup OIDC 端点收敛、日志脱敏、HTTP 超时/体限、可信代理策略、一次性 OIDC ticket、导入认证死锁校验、Secure Cookie 判定、reset token 清理、请求日志 IP 均仍存在。
- 未验证项：未运行 `govulncheck`、`npm audit`、`npm ci`、Docker 构建或动态 HTTP/OIDC/SMTP/Xray 场景；前端本机 node_modules 与锁文件不一致；若无后续专项证据，不得把这些边界写成最终“安全”。
- 证据索引：本报告 §4.1–4.7；后端定向测试 9 个包全部 `ok`；前端 6 个文件/21 个测试通过；`npm ls` 因本机安装版本与 package.json 不符返回 `ELSPROBLEMS`。
- 下一步交接：Step 3 建模时应把管理员 URL 拉取、导入无体限、Xray 明文 gRPC、平台 scheme、SR 固定参数、浏览器 localStorage JWT 与未加密备份全部视为当前攻击面，而不是沿用前两期排除。Step 6 必须用锁文件重建前端依赖后再扫描；Step 9/10/13/16/20/24/27 需补本节列出的直接测试缺口。

### Step 3 检查点 — 架构、资产、数据流与信任边界建模

- 基线：固定业务代码基线仍为 `beta` / `4958db3d900019eff3f6ffb2373b2b66a5b9a730`；进入 Step 3 时当前 HEAD 为 `e08fd8b6b545a271a96fe31c524773d4c1a01d63`，`4958db3d..e08fd8b` 仅新增/修改 `SecurityReport3.md` 与 `SecurityScanPlan1.md`，无业务代码漂移；2026-08-30 23:30:12 +0800。
- 实际范围：从 `backend/cmd/server/main.go` 启动装配追踪正常/应急 Gin 服务器、全局中间件、Handler/Service/Store、后台任务与内存态；对照全部迁移和 Design2 §5.7–5.10；覆盖浏览器/订阅客户端、WAF/反代、Go、SQLite/数据卷、OIDC、验证码、SMTP、素材 URL、Xray、外部安装链接与宿主机日志/备份接收面。未执行 Step 4 的逐路由保护链矩阵，也未启动服务或连接任何第三方。
- 执行命令：`git log/diff 4958db3d..HEAD`、`rg --files backend/cmd backend/internal backend/migrations`、`rg -n` 枚举构造函数/路由/外部 URL/网络拨号/文件读写/加解密、`sed`/`nl -ba` 人工追踪 `main.go`、`server.go`、认证/OIDC/验证码/SMTP/素材池/Xray/下载/版本/备份/导入导出/迁移与前端路由/凭据存储。
- 结论：通过。本节建立了实际组件图、AS-01–AS-12 资产表、角色/运行状态表、TB-01–TB-12 信任边界和 DF-01–DF-15 文字化数据流；每个已识别外部系统、公开入口类别与敏感资产均至少被一条数据流引用。存在 4 项设计/实现差异或口径张力，均只登记为后续专项输入。
- 新发现：无；Step 3 只建立攻击面模型，表 5.7 的差异不在本步判定严重度或分配 `S3-xx`。
- 历史项：F02/F04/F05/F09/F10/F11、L01/L03/L04/L06/L07、N02–N07、边界-1–6、O1–O2 已进入对应资产/边界/数据流；当前状态沿用 §4，不在本步重判。
- 正面控制：依赖集中构造注入；正常与应急路由树分离；会话后实时查用户状态/角色且高级开关实时查库；SQLite 启用 WAL/外键/单写连接并提供 `BEGIN IMMEDIATE`；OIDC 出站使用 HTTPS、公网解析后按解析 IP 拨号及重定向复检；敏感下载禁缓存；版本文件原子写；应急模式以全局 gate 收缩业务面。
- 未验证项：真实 WAF/反代/TLS 与可信头拓扑、容器/卷权限、真实 OIDC/验证码/SMTP/素材源/Xray、SQLite 运行数据、浏览器第三方脚本行为、所有路由的精确保护链、动态重定向/DNS/代理环境、备份和导入实际字节内容。分别留待 Step 4–7、9–27；本节图中的协议与信任结论均为当前源码静态事实或明确部署假设。
- 证据索引：本节 §5.1–5.8；`backend/cmd/server/main.go:26-137`；`backend/internal/server/server.go:68-450,453-484`；`backend/internal/store/store.go:24-163`；`backend/internal/auth/auth.go:63-177`；`frontend/src/router/index.ts:82-166`；`frontend/src/stores/auth.ts:1-20`；`backend/migrations/0001_init.sql`–`1015_platform_builtin_default.sql`。
- 下一步交接：Step 4 应以 DF-02/03/04/06/09/13/14 的入口类别为枚举种子，逐条核对正常与应急两棵路由树，特别区分匿名、Bearer、管理员、advanced、查询 Token、HttpOnly 一次性 Cookie 和应急操作码。后续专项不得漏掉额外识别的验证码提供商、外部安装 URL/客户端 scheme、宿主机 stdout/备份接收方；Xray 地址限制、验证码密钥存储、配置地址缓存语义和 `users.group_id` 关系仅为待复核差异，不能从本步直接写成漏洞结论。

#### 5.1 当前实现架构与组件所有权

| 层次 | 当前实现 | 主要数据/副作用 | 关键证据 |
|------|----------|-----------------|----------|
| 部署与进程入口 | 单个 Go 进程读取 `APP_MODE/PORT/DATA_DIR/TRUST_PROXY/LOG_*`，选择 dev/prod SQLite；数据库打开、迁移或关键配置异常时切换独立应急服务器 | 环境变量、stdout、进程信号、数据目录、正常/应急状态 | `main.go:26-137` |
| 浏览器与订阅客户端 | Vue SPA 由 Go 内嵌分发；浏览器将会话 JWT 放 `localStorage` 并用 Bearer 调 API；Clash/v2rayNG/Shadowrocket 等客户端以查询 Token 拉取订阅/规则/分享 | JWT、一次性 OIDC Cookie、下载 Token、第三方验证码脚本、客户端 scheme | `frontend/src/stores/auth.ts:1-20`；`frontend/src/api/request.ts:8-15`；`frontend/index.html:6-10` |
| HTTP 接入层 | `gin.New()` 后统一挂请求日志、panic recovery、安全头、分级体限；按域注册结构体 Handler；正常服务器最终注册静态资源/SPA fallback | JSON、multipart、path/query/header/cookie、SSE、文件流与重定向 | `server/server.go:68-450`；`server/static.go:15-91` |
| 认证与授权 | `auth.Service` 签发/验证 HS256 JWT；SessionMiddleware 实时读取用户、凭据版本和状态；AdminMiddleware 读实时快照写入的角色；AdvancedMode 每次读 DB 配置 | 用户身份、角色、状态、credential_version、signing key | `auth/auth.go:63-177`；`server/middleware.go:10-24` |
| 业务服务层 | setup/auth/OIDC、用户/审批、平台/订阅/规则/分享/自定义、版本/装配/素材池、节点/组/Xray、配置/导入/备份/日志等服务均在 `server.New` 构造并用回调连接跨域副作用 | DB 事务、版本文件、异步 Xray/邮件/同步、运行日志 | `server/server.go:78-447` |
| 数据层 | `store.Store` 封装 modernc SQLite，启用 WAL、外键、5s busy timeout、单连接与 `_txlock=immediate`；迁移嵌入二进制 | `app-dev.db`/`app-prod.db`、WAL、全部业务表 | `store/store.go:24-163`；`migrations/embed.go` |
| 文件数据层 | `DATA_DIR/contents/{subscription,rule,custom,share}/...` 保存版本正文；`DATA_DIR/public` 保存站点图标和安装包；DB 保存文件相对路径与当前版本 | 订阅/规则正文、装配模板、安装包、图标、symlink/current 指针 | `version/version.go:77-230`；`server/static.go:51-75` |
| 后台与内存态 | 素材池自动同步、访问日志/reset token 清理、Xray 采集；长任务 registry、限流计数、SSE token/连接与日志环形缓冲、OIDC discovery/JWKS cache、Xray client cache 位于内存 | 定时出站、异步数据库写、进程重启即失的状态 | `main.go:82-120`；`server/server.go:145-165,344-436` |
| 应急服务器 | 使用同一全局硬化中间件再叠加 `emergencyGate`，只放行 health/status/site/emergency/static/SPA；其余业务 API 与下载返回 503 | 应急操作码校验、管理员改密或全量重初始化、可能退出进程 | `server/server.go:453-484`；`server/emergency_gate.go:13-48` |

构造关系的核心结论是：接入层不直接持有全局业务单例，绝大多数跨域副作用由 `server.New` 注入回调；但 SQLite、`DATA_DIR`、stdout 和外部网络仍由同一进程共享，故任一高权限导入/清空/Xray/文件操作都可能跨多个业务域，后续不能只按单个 Handler 局部审查。

#### 5.2 资产清单

| ID | 资产与敏感性 | 位置/生命周期 | 主要读写者 | 关联流 |
|----|--------------|---------------|------------|--------|
| AS-01 | 根信任材料：`signing_key`；用于 JWT HMAC 并派生 AES-256-GCM 配置/节点/Xray 凭据密钥，属于最高敏感级的机密与完整性资产 | `system_config` 明文值；Setup 生成，导入/清空可替换 | config/auth/OIDC/node/xray/export | DF-01/03/04/06/08/10/12/14 |
| AS-02 | 浏览器会话与短期能力：Bearer JWT、OIDC state/PKCE verifier/nonce、HttpOnly login ticket、SSE token | JWT 在浏览器 `localStorage`；OIDC state/ticket 在 Cookie+SQLite；SSE token 在内存 | auth/OIDC/前端/SSE | DF-03/04/13 |
| AS-03 | 公开能力 Token：用户下载、分享、规则、密码重置、应急操作码 | 前四类在 SQLite；应急码由进程随机生成、仅存内存并写 stdout，`RESET_ADMIN_PASSWORD` 只负责触发手动应急模式 | download/auth/emergency | DF-03/09/14 |
| AS-04 | 身份与个人信息：用户名、邮箱、OIDC subject/claims、角色、状态、组、IP、审批记录语义 | `users`、`access_logs`、运行日志；OIDC claims 可持久化 | user/auth/OIDC/approval/log | DF-03/04/06/10/11/13/14 |
| AS-05 | 本地认证材料：bcrypt 密码哈希、credential_version | `users`；改密/禁用/角色与导入操作影响会话有效性 | auth/user/emergency | DF-03/06/12/14 |
| AS-06 | 外部服务配置与秘密：OIDC client secret、SMTP password、CAPTCHA secret/site key、回调/前端 URL、可信代理与限流配置 | `system_config`；OIDC secret 嵌套密文、SMTP password 敏感键密文、CAPTCHA secret 当前为明文 | config/OIDC/mail/captcha/setup | DF-01/03/04/05/06/11/12 |
| AS-07 | 订阅与规则业务正文：subscription/rule/share/custom 版本、原始文件名、当前版本、装配 blueprint/render plan | 元数据在 SQLite；正文在 `DATA_DIR/contents`；最多 5 版并有 current 指针 | version/assembly/download/admin | DF-06/08/09/12/14 |
| AS-08 | VPN 节点与代理凭据：manual `protocol_json` 密文字段、用户/独立账号 UUID 与代理密码、节点地址/协议、Xray 推送状态 | `nodes/users/xray_ext_accounts/xray_users/xray_ext_users`；渲染/推送时解密 | node/assembly/download/xray | DF-06/08/09/10/12/14 |
| AS-09 | 素材池与外部内容：管理员 URL、同步响应、解析后的规则条目、任务错误/逐 URL 结果 | `rule_pools/pool_entries/pool_sync_tasks`；响应只在同步进程内短暂存在 | pool/cron/assembly | DF-07/08/13 |
| AS-10 | 流量、配额与授权派生状态：组节点分配、公共节点、月流量、超限、候选集 | `groups/group_nodes/traffic_records/xray_ext_traffic` 及相关用户字段 | group/xray/download/home | DF-06/09/10 |
| AS-11 | 文件与恢复资产：dev/prod DB、WAL、`contents/`、`public/`、未加密 `tar.gz` 备份、加密配置导出、导入文件与临时 DB 快照 | 数据卷、HTTP 下载接收方、OS 临时目录；配置导出不含全部业务正文，备份包含全部持久化数据 | store/version/platform/config/backup/dataclear | DF-01/02/06/08/12/14/15 |
| AS-12 | 可观测与运行时资产：stdout 日志、SQLite 访问日志、500 条环形缓冲、SSE token/连接、限流桶、长任务状态、OIDC/JWKS 与 Xray client cache | stdout、SQLite、进程内存；重启后部分丢失 | log/ratelimit/tasks/OIDC/Xray | DF-01/04/06/07/10/11/12/13/14 |

#### 5.3 角色、身份与运行状态

| 角色/状态 | 认证材料与实际能力边界 | 关键限制/转换 |
|-----------|------------------------|---------------|
| 匿名访问者 | 可访问 health、system status、site info、公告、静态资源/SPA、Setup/认证/OIDC 公开链和 Token 下载入口 | 是否真正可用受 configured、APP_MODE、OIDC/captcha 配置、限流和 emergency gate 影响；Step 4 建精确矩阵 |
| 能力 Token 持有者 | 不一定有会话；凭下载/reset/OIDC ticket/SSE/emergency 操作码进入单用途链 | 各 Token 的有效期、绑定对象、一次性/撤销语义不同，不得统称“匿名” |
| 未配置/Setup 状态 | `configured=false` 时 Setup quickstart/OIDC/import 可用；完成后预置默认组/平台并生成/导入签名密钥 | Setup 完成不等于已有管理员；用户表为空时下一位注册者成为首管理员 |
| 待审批用户 | `users.status=pending`，有持久化身份/claims 但 SessionMiddleware 不允许进入会话保护面 | 审批通过转 active；拒绝可能删除用户并触发 Xray 清理/通知 |
| 普通用户 | `status=active, role=user`；Bearer 会话可访问 home/rules/profile/预览，并持有个人平台下载 Token | 角色与状态每请求实时查库；换邮箱/密码、禁用等递增 credential_version 或清 Token |
| 管理员 | `status=active, role=admin`；拥有全部普通用户能力和管理/运维入口 | 管理员能力仍在本期威胁模型内；凭据被滥用时其 URL 拉取、导入、备份、Xray 等副作用不可排除 |
| 高级模式管理员 | 管理员且 `advanced_mode=true`；额外进入 Xray、组高级字段和部分用户管理操作 | 这是配置状态而非独立角色；后端 `AdvancedMode` 每次读 DB，前端隐藏不是授权边界 |
| 禁用用户 | `status=disabled`，现有 Bearer 会话被 SessionMiddleware 拒绝；管理动作会清下载 Token并尝试移除 Xray 用户 | 历史分享/规则等全局能力不等同于该用户会话；Step 11/12 细分对象所有权 |
| Dev 模拟身份 | 仅 `APP_MODE=dev` 且 provider=mock 时使用模拟 OIDC；仍落入相同用户/角色/状态模型 | 根 Compose 默认 Dev 是历史边界；Step 7/8/9 动态核对条件注册与失败模式 |
| 应急模式操作者 | 无常规会话；从宿主机日志取得进程随机生成的一次性应急操作码后执行管理员改密或重新初始化 | `RESET_ADMIN_PASSWORD` 仅触发手动模式；正常业务路由树不注册；gate 只保留最小面，重初始化可能删除 DB/contents/public 并退出 |

#### 5.4 信任边界

| ID | 边界与跨越数据 | 当前实现中的边界控制/假设 | 后续专项 |
|----|----------------|---------------------------|----------|
| TB-01 | 互联网中的浏览器/订阅客户端 ↔ WAF/CDN/反向代理 | WAF/TLS/证书均为部署外部条件；应用不把其视为认证或输入校验替代 | Step 7/16/25/26 |
| TB-02 | WAF/反代 ↔ Go HTTP 监听端口 | `TRUST_PROXY` 决定 XFF/X-Real-IP/XFP/XFH 信任；应用本身不全局强制 HTTPS，内部链是否加密未知 | Step 7/16/21 |
| TB-03 | 浏览器运行时 ↔ Go API/静态站点 | Bearer JWT 在 localStorage；OIDC 临时 Cookie 为 HttpOnly/Lax；公开下载用 query Token；CSP 为 meta | Step 4/10/12/15/16 |
| TB-04 | 普通身份 ↔ 管理员/高级模式逻辑边界（Go 进程内） | Session → Admin → Advanced 中间件组合，角色/状态/开关均以 DB 当前值为准；前端守卫仅辅助 | Step 4/11/26 |
| TB-05 | Go 进程 ↔ SQLite 与本地数据卷/OS 临时目录 | OS 文件权限是部署边界；应用启用 WAL/FK/单写连接、原子版本写，DB 与正文文件并非同一事务介质 | Step 17/18/19/22 |
| TB-06 | Go ↔ OIDC 提供商；浏览器 ↔ OIDC 授权页 | Go 以 HTTPS、自定义 DNS/拨号、重定向校验访问 discovery/token/JWKS；浏览器携 state/PKCE/nonce 走重定向；真实 IdP 未验证 | Step 9/20/25 |
| TB-07 | 浏览器/Go ↔ Google reCAPTCHA 或 Cloudflare Turnstile | 浏览器加载第三方脚本/frame并取得 token；Go 将 secret+token POST 到固定验证端点；缺 secret 当前 fail-open | Step 13/15/19/20 |
| TB-08 | Go ↔ 管理员配置的 SMTP | 支持 TLS 直连、STARTTLS 或明文；发送邮箱、重置链接、站点名与 SMTP 凭据，服务失败多为 best-effort | Step 13/19/20/24 |
| TB-09 | Go ↔ 管理员配置的规则素材 URL | 普通 `http.Client` 允许 URL 响应进入解析器/SQLite；50MB 限制，未在本步确认地址/重定向限制 | Step 14/20/21 |
| TB-10 | Go ↔ 管理员配置的 Xray gRPC 实例 | `host:port` 语法校验后使用 `insecure.NewCredentials()`；传送节点元数据、用户 UUID/密码、email、控制动作和流量统计 | Step 19/20/23 |
| TB-11 | Go 输出 ↔ 订阅客户端、外部安装站点与本机客户端 scheme | Token 下载输出用户定制正文/响应头；管理员配置安装 URL 与 scheme 由浏览器导航或唤起本地客户端 | Step 12/14/15/16/20 |
| TB-12 | Go ↔ 宿主机/容器运行者、stdout 收集器与备份/导出接收方 | 环境变量决定运行模式/数据目录/应急；日志离开进程进入外部平台；管理员浏览器接收完整备份或加密配置导出 | Step 7/18/19/24 |

#### 5.5 文字化数据流图

以下箭头中的 `TB-*` 表示跨越信任边界，`AS-*` 表示所携带或影响的敏感资产。

1. **DF-01 启动、迁移与运行态：** 宿主机环境/数据卷 `→ TB-12 → main` 解析模式、端口、可信代理和应急变量 `→ TB-05 → store.Open/Migrate` 打开对应 SQLite、应用嵌入迁移；成功后构造正常服务器与定时任务，失败或检测异常则构造应急服务器；日志同时写 stdout 和内存环形缓冲（AS-01/06/11/12）。
2. **DF-02 SPA 与公开元数据：** 匿名浏览器/客户端 `→ TB-01/TB-02 → Go` 获取 `/health`、系统状态、站点信息、公告、内嵌 `/assets`、数据卷 `/public` 或 SPA fallback；Go 从 AS-06/11 读取配置或文件后返回，公开安装包可作为附件，站点图片可缓存；应急模式下同类入口受 gate 白名单约束（AS-06/11）。
3. **DF-03 Setup、本地注册/登录与密码重置：** 匿名请求 `→ TB-03 → JSON/multipart/query 解析、验证码/限流`；Setup 在 `BEGIN IMMEDIATE` 中生成/导入 AS-01、写配置/默认组/平台；注册写 AS-04/05 并决定首管理员/pending；登录以 bcrypt 校验后签发 AS-02 JWT 给浏览器 localStorage；forgot 生成 AS-03 reset token并交 DF-11 外发，reset 消费 token、更新密码哈希与 credential_version（AS-01–06）。
4. **DF-04 OIDC 登录/绑定：** 浏览器 `→ TB-03 → Go` 生成并持久化 state/PKCE/nonce，Go `→ TB-06 → IdP` 拉 discovery/JWKS、用 code+verifier 换 token并验签；身份/claims 回到 Go 解析、合并或创建 AS-04 用户；pending 跳公开页，active 会话写入 SQLite 一次性 ticket并以 HttpOnly Cookie返回，浏览器换取 AS-02 JWT；绑定链在既有会话下复用该流程（AS-01/02/04/06/12）。
5. **DF-05 验证码：** 登录/注册/forgot 页面加载 Google/Cloudflare 脚本与 frame `→ TB-07` 得到 token；浏览器把 token 发 Go，Go 读取 AS-06 secret 后 `→ TB-07` POST 固定验证端点并解析 JSON success，再决定是否继续 DF-03（AS-06）。
6. **DF-06 受保护业务与管理操作：** 浏览器 Bearer `→ TB-03 → SessionMiddleware` 验 JWT并实时读 AS-04/05；管理员/高级入口再跨 TB-04；Handler 解析 JSON/multipart/path/query，Service 在 TB-05 内读写平台、用户、组、Token、版本元数据、配置、节点、配额与任务，并可能触发 DF-07/08/10/11/12/13 的异步或外部副作用（AS-01/03–12）。
7. **DF-07 素材 URL 同步：** 管理员保存 URL并提交/定时器启动任务（DF-06）`→ TB-09 → 外部 URL`；Go 限量读取响应、逐行解析/规范化规则，使用独立 SQLite 连接和分批短事务 upsert/差量删除，任务与错误写 AS-09/12，随后成为 DF-08 的装配输入（AS-09/12）。
8. **DF-08 装配、版本与文件写入：** 管理员选择 manual/xray 节点、proxy groups、素材池和自定义规则 `→ TB-04`；Go 解析 JSON/YAML/URI/协议字段并解密所需节点秘密，以当前数据库快照渲染内容；`version.Service` 在事务中计算版本号并通过同目录临时文件+rename写 `DATA_DIR/contents`，再写 versions/assembly_blueprints 和激活指针（AS-01/07–10/11）。
9. **DF-09 公开 Token 下载与会话预览：** 订阅客户端或浏览器 `→ TB-11/TB-02 → query Token/path 或 Bearer preview`；Go 实时解析 AS-03 绑定和当前版本，读取 AS-07 文件；装配模板按用户组/候选节点、AS-08 凭据与 AS-10 配额动态重渲染并编码，写访问日志后以 `text/plain`、`no-store` 和平台响应头返回；分享/规则/直接上传内容走原样分支（AS-03/04/07/08/10/12）。
10. **DF-10 Xray 控制与流量：** 高级管理员操作、用户生命周期、节点变化、导入后处理或 cron `→ TB-04 → Xray services` 读取/解密 AS-08、计算期望集；Go `→ TB-10 → gRPC` 执行节点检测、Add/Remove/GetUsers/QueryStats；返回 protobuf 被解析为节点/流量，SQLite 更新 AS-08/10/12，失败进入同步状态/日志/重试任务（AS-04/08/10/12）。
11. **DF-11 SMTP 邮件：** reset/欢迎/审批/测试动作读取 AS-04 邮箱与解密后的 AS-06 SMTP 密码，组装清洗过 header 的纯文本邮件 `→ TB-08 → SMTP`；可携 AS-03 reset URL，发送错误通常写 AS-12 而不回滚主业务（AS-03/04/06/12）。
12. **DF-12 配置导入导出与完整备份：** 管理员 Bearer 或未配置 Setup import `→ TB-03/TB-04 → multipart/JSON`；导入把整个文件读内存、用 Argon2id/AES-GCM 解密并解析配置/Xray JSON，在事务中覆盖并可能跨 TB-10 清理/检测/对账，写 AS-01/06/08/11/12；配置导出将配置/Xray实例/独立账号/图标加密后返回；备份在 OS 临时目录生成 SQLite 一致性快照并把 DB+contents+public 以未加密 tar.gz 流向管理员接收方 `→ TB-12`（AS-01/04–08/10–12）。
13. **DF-13 日志、访问记录与 SSE：** 所有 HTTP 请求/业务告警/外部错误流向统一 logger `→ TB-12 → stdout` 并进入 AS-12 环形缓冲；下载事件另写 SQLite `access_logs`；管理员用 Bearer换一次性短期 token，再以 `/api/admin/logs/stream?token=` 跨 TB-03 建 SSE，消费运行日志；清空操作复位 DB/内存态（AS-02/04/09/12）。
14. **DF-14 应急与全量清空：** 启动检测或 `RESET_ADMIN_PASSWORD` 选择独立应急路由；操作者以 AS-03 应急码 `→ TB-03` 验证后改指定管理员密码，或确认重新初始化；服务经 TB-05 删除/重建 DB、contents/public并可能退出。正常管理员 `clear_all` 也清 DB/文件并重置签名密钥、限流/SSE/日志内存态（AS-01–12）。
15. **DF-15 安装包、外部下载与客户端唤起：** 管理员上传安装包写 `DATA_DIR/public/installers`，或保存外部 installer URL/客户端 scheme（DF-06）；普通浏览器从 Go 取公开附件，或 `→ TB-11` 导航外部 URL/以带订阅 URL 的 scheme 唤起本机 VPN 客户端；由外部站点/本地客户端继续处理数据（AS-03/07/11）。

#### 5.6 跨边界危险操作索引

| 操作类型 | 当前入口点/解析器 | 影响资产与边界 | 后续检查 |
|----------|-------------------|----------------|----------|
| 解析不可信输入 | Gin JSON/multipart/path/query/header/Cookie；OIDC discovery/JWKS/JWT；验证码 JSON；素材规则文本；node URI/protocol JSON；装配 YAML/JSON；导入密文+JSON；Xray protobuf | TB-03/06/07/09/10，AS-02–10 | Step 8–18/20/23/27 |
| 解密/派生 | signing key HKDF → AES-GCM；OIDC/SMTP/manual node/用户与独立账号凭据；Argon2id 导入文件 | AS-01/06/08/11，TB-05/12 | Step 18/19/23 |
| 网络拨号 | OIDC HTTPS、验证码 HTTPS、SMTP TCP/TLS、素材 HTTP(S)、Xray gRPC | TB-06–10 | Step 9/13/20/21/23 |
| 文件写入/删除 | SQLite/WAL、版本临时文件+rename、站点图标/安装包、导入图标、备份临时 DB、清空/应急重初始化 | TB-05/12，AS-07/11 | Step 17/18/22 |
| 对外内容/凭据输出 | JWT/一次性 Cookie、Token 下载、动态订阅、SSE、配置导出、完整备份、SMTP reset URL、外部 URL/scheme | TB-03/08/11/12，AS-02/03/07/08/11/12 | Step 10/12–19/24 |

#### 5.7 设计、计划与当前实现差异

| 编号 | 差异/口径张力 | 当前静态证据 | 本步处置 |
|------|---------------|--------------|----------|
| D3-01 | Design2 §5.9 描述 `xray_instances.api_addr` 有“IP 白名单防护”，当前 `ValidateAddr` 只校验 `host:port` 与端口范围，随后使用明文 gRPC；未见 CIDR/主机白名单接线 | `backend/internal/xray/client.go:41-76`；`instance.go:92-99,137-144` | 仅作为 TB-10 建模事实；Step 20/23 判定成立条件与风险，不在本步选设计或实现口径 |
| D3-02 | AGENTS §4.2 要求敏感配置加密，当前 CAPTCHA `secret_key` 的代码注释和注册逻辑明确为明文存储/真实值回显；OIDC 与 SMTP secret 则走密文 | `captcha/captcha.go:21-27`；`config/config.go:37-52`；`mail/mail.go:37-40` | 记录 AS-06；Step 13/19 核对产品意图、暴露面与是否形成新发现 |
| D3-03 | Design1 多处写“frontend_url/callback_url 启动时缓存、修改后需重启”，当前 OIDC 实际 callback 直接由实时读取的 `frontend_url + /api/auth/oidc/callback` 生成；`callback_url` 仅见配置读写，未进入真实授权/换票链。Cookie secure 判定、下载响应头和邮件 URL 也实时读配置，但保存端点/UI 仍提示重启 | `backend/internal/oidc/oidc.go:193-197`；`backend/internal/config/admin.go:153-205`；`backend/internal/server/oidc.go:59-76`；`backend/internal/server/settings.go:103-110`；`backend/internal/download/download.go:163-190` | Step 7/9/16 核对真实运行语义和 UI/文档一致性，本步不自行决定哪一口径应保留 |
| D3-04 | `users.group_id` 是核心授权/渲染关系，但迁移只建普通 INTEGER+索引，没有声明 `REFERENCES groups(id)`；组删除依赖业务服务先迁用户到默认组，而其他新增关系大量依赖 FK CASCADE | `migrations/0002_users.sql:1-16`；`migrations/1009_xray.sql:106-137`；`internal/group/group.go` | 作为 TB-05 的逻辑关系记录；Step 11/22 审查绕过业务层、导入/清空和并发下的一致性 |

此外，Step 3 原计划点名的外部系统未显式列出验证码提供商、外部安装链接/本机客户端 scheme 与宿主机日志/备份接收方；当前实现确有这些跨边界通道，已纳入 TB-07/TB-11/TB-12 和 DF-05/13/15。这是范围补全，不是漏洞结论，也不改变计划的明确排除项。

#### 5.8 覆盖闭环与后续引用规则

- 外部系统闭环：WAF/反代（DF-01/02）、OIDC（DF-04）、验证码（DF-05）、SMTP（DF-11）、素材 URL（DF-07）、Xray（DF-10）、外部安装/本机客户端（DF-15）、宿主机日志与备份接收方（DF-01/12/13/14）均已入流。
- 公开入口闭环：health/status/site/announcement/static/SPA（DF-02）、Setup/本地认证/reset（DF-03）、OIDC 登录/回调/换票（DF-04）、Token 下载/会话预览（DF-09）、SSE query token（DF-13）、应急端点（DF-14）均已入流；逐 method/path 和中间件顺序留给 Step 4。
- 敏感资产闭环：AS-01–AS-12 均至少关联两条流；后续发现优先引用资产 ID、TB ID 与 DF ID，避免每个 Step 重新定义信任关系。
- 本图不表达“边界安全”：例如标注有 WAF、HTTPS、管理员或高级模式只说明前置条件，不能替代后续对认证、授权、SSRF、加密、解析、文件与审计控制的专项判断。

### Step 4–28 检查点目录

以下目录为后续追加位置。每个 Step 完成后必须写入完整证据胶囊，不得只更新状态：

| Step | 标题 | 状态 |
|-----:|------|------|
| 4 | 后端 API、前端路由与保护链全量清点 | 未开始 |
| 5 | 本地隔离动态环境与合成数据基线 | 未开始 |
| 6 | Go/npm/容器/CI 软件供应链审查 | 未开始 |
| 7 | 应用交付与运行时安全配置审查 | 未开始 |
| 8 | Setup、本地认证、注册与应急模式审查 | 未开始 |
| 9 | OIDC 发现、登录、绑定与换票链审查 | 未开始 |
| 10 | 会话、Bearer/Cookie、CSRF 与凭据失效审查 | 未开始 |
| 11 | RBAC、对象所有权、管理员与高级模式授权审查 | 未开始 |
| 12 | 下载 Token、分享、公开内容与 SSE 凭据审查 | 未开始 |
| 13 | 密码重置、验证码、审批与邮件安全审查 | 未开始 |
| 14 | SQL、命令、配置、CRLF 与解析器注入审查 | 未开始 |
| 15 | 前端 XSS、DOM、Markdown、URL 与第三方组件审查 | 未开始 |
| 16 | CSP、安全响应头、CORS、跳转与缓存策略审查 | 未开始 |
| 17 | 路径穿越、文件上传/下载、静态资源与临时文件审查 | 未开始 |
| 18 | 配置导入导出、备份、清空与数据完整性审查 | 未开始 |
| 19 | 密码学、密钥派生、凭据存储与敏感数据生命周期审查 | 未开始 |
| 20 | SSRF、DNS rebinding、重定向与全部出站网络审查 | 未开始 |
| 21 | 请求限制、超时、限流、任务与小规模 DoS 审查 | 未开始 |
| 22 | 业务一致性、并发事务、级联、预览/生成竞态审查 | 未开始 |
| 23 | Xray 集成、协议参数、凭据与对账副作用审查 | 未开始 |
| 24 | 日志脱敏、错误处理、调试、审计与告警能力审查 | 未开始 |
| 25 | 匿名、公开、Setup 与异常状态黑盒动态验证 | 未开始 |
| 26 | 普通用户、管理员与跨角色黑盒动态验证 | 未开始 |
| 27 | 恶意输入回归、历史修复回归与全量验证 | 未开始 |
| 28 | 发现去重、风险校准、OWASP 总结与最终交付 | 未开始 |

---

## 六、变更记录

| 版本 | 日期 | 内容 |
|------|------|------|
| v0.1 | 2026-08-30 | Step 1：固定 Git/工具链/文件数量/配置哈希，建立范围、方法、编号、严重度和检查点骨架。 |
| v0.2 | 2026-08-30 | Step 2：完成 SecurityReport1/2 全部历史编号、部署边界与观察项的当前实现回归矩阵；运行历史相关定向测试并记录本机 node_modules 漂移。 |
| v0.3 | 2026-08-30 | Step 3：从当前实现建立组件所有权、AS-01–AS-12 资产、角色/运行状态、TB-01–TB-12 信任边界、DF-01–DF-15 数据流与跨边界操作索引；单列设计/实现差异并完成覆盖闭环。 |
