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

### Step 3–28 检查点目录

以下目录为后续追加位置。每个 Step 完成后必须写入完整证据胶囊，不得只更新状态：

| Step | 标题 | 状态 |
|-----:|------|------|
| 3 | 架构、资产、数据流与信任边界建模 | 未开始 |
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
