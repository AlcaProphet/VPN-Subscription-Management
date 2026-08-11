# VPN 订阅管理系统

<p align="center">
  <b>自托管 · 轻量 · 一键部署</b> —— 给团队用的 VPN 订阅分发面板，一个容器搞定全部。
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go"/>
  <img src="https://img.shields.io/badge/Vue-3-42b883?logo=vuedotjs&logoColor=white" alt="Vue 3"/>
  <img src="https://img.shields.io/badge/SQLite-嵌入式-003B57" alt="SQLite"/>
  <img src="https://img.shields.io/badge/Docker-单容器-2496ED?logo=docker&logoColor=white" alt="Docker"/>
  <img src="https://img.shields.io/badge/认证-OIDC%20%2B%20本地-8b5cf6" alt="Auth"/>
</p>

> 简单说：把订阅链接「集中管理、一键分发」的网页工具。管理员上传一次订阅配置，成员打开网页点一下「一键导入」，客户端（Clash / v2rayNG / Shadowrocket 等）自动完成配置，再也不用手动复制粘贴。

---

## 目录

- [它解决什么问题](#它解决什么问题)
- [功能特性](#功能特性)
- [界面预览](#界面预览)
- [快速开始（一键部署）](#快速开始一键部署)
- [日常使用](#日常使用)
- [两种部署形态](#两种部署形态)
- [备份与升级](#备份与升级)
- [常见问题 FAQ](#常见问题-faq)
- [高级运维](#高级运维)
- [技术栈](#技术栈)
- [本地部署与开发](#本地部署与开发)

---

## 它解决什么问题

小团队（≤10 人）维护 VPN 订阅的传统痛点：

| 传统方式 | 本系统 |
|---------|--------|
| 订阅链接在群里反复发，过期了又要重新发 | 链接长期有效，客户端自动定时拉取最新配置 |
| 每个人要手动改配置、填服务器信息 | 网页「一键导入」，客户端自动完成 |
| 配置文件散落在个人电脑上 | 集中存储在服务器，管理员更新全员即时生效 |
| 无法控制谁能拿到什么配置 | 用户组 + 订阅分级 + 自定义分配，权限精细可控 |

---

## 功能特性

- **🔐 双认证体系**：本地账号（邮箱 + 密码）与 OIDC 单点登录（Keycloak / Auth0 / 通用 OIDC）可并存
- **📦 订阅集中管理**：按平台（Clash / v2rayNG / Shadowrocket…）维护订阅池，支持文件上传与文本编辑，每份订阅保留最多 5 个历史版本，可随时预览、切换、回滚
- **👥 用户组分发**：订阅与用户组多对多关联，组内成员自动获得对应订阅；还可为「特定用户 + 特定平台」单独分配自定义订阅
- **🔗 长期有效下载 Token**：链接永不超时，客户端可定时拉取最新配置；支持一键刷新（旧链接立即失效）与吊销
- **📤 独立分享订阅**：不绑定任何用户的公开分享链接，适合给临时访客
- **📜 分流规则管理**：规则独立版本管理与下载 Token，一键导入客户端
- **📋 完整访问日志**：记录全部下载请求（用户 / 平台 / IP / 成败原因），保留 90 天自动清理
- **🛡️ 安全机制**：JWT 会话 + 密钥加密存储、Token 日志脱敏、注册/登录/下载速率限制、验证码（reCAPTCHA / Turnstile）可选启用
- **✉️ SMTP 消息**：密码重置、审批通知、欢迎邮件（可选启用）
- **💾 一键备份**：面板上点一下，下载包含数据库一致性快照 + 全部文件的 tar.gz
- **🌙 多端友好**：手机 / 平板 / 桌面自适应，支持暗色模式

---

## 界面预览

登录页 | 成员主页（一键导入）
:---:|:---:
![登录页](docs/screenshots/01-login.png) | ![成员主页](docs/screenshots/02-home.jpg)

管理面板 · 订阅管理 | 管理面板 · 用户管理
:---:|:---:
![订阅管理](docs/screenshots/03-admin-subscriptions.png) | ![用户管理](docs/screenshots/04-admin-users.png)

管理面板 · 面板配置 | 首次部署 · Setup 引导
:---:|:---:
![面板配置](docs/screenshots/05-admin-settings.png) | ![Setup 引导](docs/screenshots/06-setup-step1.png)

---

## 快速开始（一键部署）

> ⏱️ 全程大约 5 分钟。前提：已安装 Docker（含 Docker Compose）。剩下的只需要会「复制粘贴命令」和「打开浏览器」。

### 第 1 步：获取项目文件

**方式一（推荐，无需 Git）**：下载本项目 zip 包并解压

1. 打开项目主页 → 绿色 `Code` 按钮 → `Download ZIP`
2. 解压到服务器任意目录，例如 `~/vpn-sub`
3. 终端进入该目录：`cd ~/vpn-sub`

**方式二（Git）**：

```bash
git clone https://github.com/AlcaProphet/VPN-Subscription-Management.git
cd VPN-Subscription-Management
```

### 第 2 步：一键启动

```bash
docker compose up -d
```

第一次启动会自动构建镜像，需要几分钟，看到 `Started` 字样即成功 ✅

> 生产部署建议改用预构建镜像（秒级启动、无需本地构建），见 [两种部署形态](#两种部署形态)。

### 第 3 步：打开网页，完成首次配置

浏览器访问：**http://服务器IP:8080**（服务器就在本机则访问 http://localhost:8080）

首次访问会自动进入 **Setup 首次配置引导**，只需 3 步：

**① 选择「快速开始」（推荐，零配置）**，点击「下一步」

![Setup 第一步](docs/screenshots/06-setup-step1.png)

**② 确认使用本地账号模式**，点击「确认完成」

![Setup 第二步](docs/screenshots/07-setup-confirm.png)

**③ 配置完成**，点击「前往登录」

![Setup 第三步](docs/screenshots/08-setup-complete.png)

### 第 4 步：注册管理员账号

> ⚠️ **重要**：系统采用「先到先得」机制——**第一个注册的用户自动成为管理员**。公网部署时请部署者本人立即注册，否则可能被别人抢注！

点击「前往登录」→ 页面底部进入注册 → 填写用户名、邮箱、密码 → 完成。

注册成功后自动成为管理员，页面右上角出现「管理面板」入口。恭喜，部署完成 🎉

---

## 日常使用

### 管理员：上传订阅

1. 登录后点击右上角「**管理面板**」→「订阅」
2. 点击「**新建订阅**」→ 选择平台（如 Clash Verge）→ 上传订阅配置文件（或粘贴文本）
3. 分配用户组（如「默认组」）→ 保存

成员登录主页后即可看到该订阅，点击「**一键导入**」或「**复制链接**」直接使用。

> 想重新上传新配置？编辑该订阅上传新文件即可生成新版本；「版本管理」里可随时回滚到任意历史版本（最多保留 5 个）。

### 成员：一键导入订阅

1. 打开系统网址 → 登录
2. 找到对应平台的订阅卡片 → 点击「**一键导入**」（自动唤起已安装的客户端）
3. 或点击「**复制链接**」，在客户端里粘贴导入

订阅链接长期有效，客户端会定时自动拉取最新配置；配置更新后成员无需任何操作。

---

## 两种部署形态

系统容器默认只监听 8080 端口，接入方式二选一：

### 方式 A：公网部署（推荐，配域名 + HTTPS）

用外部反向代理（Nginx 等）承接 HTTPS 与域名，容器只绑定本机回环地址，不直接暴露公网：

```bash
# 使用生产模板（内含预构建镜像 + 回环端口绑定）
cp docker-compose.yml.example docker-compose.yml
docker compose up -d
```

Nginx 反向代理配置示例（`/etc/nginx/sites-available/vpn`）：

```nginx
server {
    listen 443 ssl;
    server_name vpn.example.com;
    ssl_certificate     /path/to/fullchain.pem;
    ssl_certificate_key /path/to/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

> ⚠️ OIDC 单点登录与验证码功能依赖**公网可达的 HTTPS 域名**，请先完成域名解析与证书配置再启用。

### 方式 B：局域网直连

适合家庭 / 公司内网：直接暴露 8080 端口即可（修改 `docker-compose.yml` 中 `ports` 为 `"8080:8080"`）。

> ⚠️ 直连模式下凭据与订阅链接均为明文传输，**仅限可信内网**，公网务必使用方式 A。

---

## 备份与升级

### 备份（管理员面板完成）

「管理面板」→「面板配置」→ 下拉到「**备份下载**」→ 点击「下载备份（tar.gz）」——包含数据库一致性快照与全部文件，存到安全的地方即可。

### 升级

```bash
docker compose pull          # 拉取最新镜像
docker compose up -d         # 重启应用（数据全部保留在数据卷中）
```

---

## 常见问题 FAQ

**Q：忘记管理员密码了？**
未配置 SMTP 时，可在 docker-compose.yml 中设置 `RESET_ADMIN_PASSWORD: "1"` 并重启容器，进入应急恢复模式（操作码见容器日志），重置密码后移除该变量并重启。详见 [高级运维](#高级运维)。

**Q：成员看不到订阅？**
订阅需与用户所属的用户组关联；若该平台订阅「未分配」，请联系管理员在「订阅」页分配用户组。

**Q：能对外开放分享链接吗？**
可以。「管理面板」→「分享」创建独立分享订阅，链接不绑定任何用户，适合临时访客；支持随时刷新 / 吊销。

**Q：客户端导入后订阅不更新？**
订阅链接长期有效，Clash 等客户端会按自身策略定时拉取；也可在客户端手动「更新订阅」。

**Q：想换服务器 / 迁移？**
Production 模式下，「面板配置」→「配置导入/导出」可导出加密配置文件，新服务器 Setup 页选择「导入已有配置」即可整体迁移。

---

## 高级运维

### 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `APP_MODE` | `prod` | `dev` / `prod`（决定数据库文件与功能差异） |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `console` | `console` / `json` |
| `PORT` | `8080` | 监听端口 |
| `TRUST_PROXY` | `auto` | `auto` / `on` / `off`，真实客户端 IP 解析策略 |
| `DATA_DIR` | `./data` | 数据目录（容器内固定 `/data`） |
| `RESET_ADMIN_PASSWORD` | — | 应急恢复：管理员密码救援 |

### 应急恢复（管理员密码救援）

```yaml
# docker-compose.yml 中取消注释并重启容器
environment:
  RESET_ADMIN_PASSWORD: "1"
```

```bash
# 操作码输出在容器日志中
docker compose logs vpn-sub | grep 操作码
```

重置完成后**务必移除该变量并重启容器**恢复正常服务。

### 数据存储

全部持久化数据（数据库、订阅文件、资源）存放在 Docker 数据卷 `vpn-data` 中，删除容器不会丢失；备份请使用面板「备份下载」。

---

## 技术栈

- **后端**：Go 1.25 + Gin + SQLite（纯 Go 零 CGO 驱动，嵌入式存储，无需外部数据库）
- **前端**：Vue 3 + Vite + Ant Design Vue + Tailwind CSS
- **部署**：单容器（API + 前端页面 + 静态资源一体），多阶段构建，非 root 运行，数据卷持久化
- **CI/CD**：GitHub Actions 自动构建并推送 Docker 镜像（打 `v*` 标签触发）

---

## 本地部署与开发

### 本地部署（局域网直连）

适合家庭 / 公司内网快速体验：直接暴露 8080 端口（修改 `docker-compose.yml` 中 `ports` 为 `"8080:8080"`），浏览器访问 `http://服务器IP:8080` 即可。

> ⚠️ 直连模式下凭据与订阅链接均为明文传输，**仅限可信内网**，公网部署请使用「两种部署形态」中的方式 A。

### 本地开发

- 后端：`cd backend && go run ./cmd/server`（内嵌前端产物，仅 API）
- 前端：`cd frontend && npm run dev`（Vite dev server 代理 `/api` 到 `127.0.0.1:8080`）

构建验证：

| 场景 | 命令 |
|------|------|
| 后端编译 / 检查 / 测试 | `cd backend && go build ./... && go vet ./... && go test ./...` |
| 前端构建 | `cd frontend && npm run build` |

---

## 许可证

本项目为自托管开源项目，详情见仓库 LICENSE 文件。
