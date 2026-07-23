# VPN Subscription Management

一个轻量级、自托管的 VPN 订阅管理系统，面向 ≤10 人的小团队。管理员通过 Web UI 配置 OIDC 认证、管理用户与订阅配置文件；普通用户登录后一键导入到 VPN 客户端。配置更新全员自动生效，用户获取一次订阅链接后无需登录即可持续获得最新配置。

---

## 功能特性

- 📤 **一次上传，全员同步** — 管理员更新订阅版本，所有成员自动获取最新配置
- 🔗 **长期有效链接** — 用户拿到的下载 Token 无过期时间，VPN 客户端可定时自动拉取
- 👥 **订阅分级** — 默认订阅与高级订阅，由管理员在用户管理中控制 is_advanced 标记
- 🛡️ **无需登录下载** — 拿到订阅链接后，VPN 客户端通过 `?token=` 即可拉取配置
- 🔧 **自定义订阅** — 管理员可为特定用户+平台分配独立配置，覆盖默认/高级自动分配
- 🔗 **独立分享订阅** — 创建不绑定用户的公开分享链接，适合外部人员或团队公共配置
- 📋 **分流规则管理** — 支持 Shadowrocket (.conf) 规则文件，独立版本管理与下载 Token
- 🔄 **版本管理** — 所有订阅/规则/自定义/分享均支持最多 5 个历史版本，可预览、切换、回滚
- 📊 **访问日志** — 记录所有下载请求（subscription/share/custom/rule），区分成功/失败及原因，保留 90 天
- ⚡ **速率限制** — 登录与下载 API 可配置限流（默认 10/分钟、20/分钟），管理员可在后台修改
- 🌓 **暗色模式** — 深色/浅色主题切换，localStorage 持久化偏好
- 📱 **移动端适配** — 响应式布局，手机与桌面均可流畅使用
- 🐳 **Docker 一键部署** — 零外部依赖，SQLite 嵌入式存储，`docker compose up -d` 即可启动

---

## 前置要求

| 条件 | 说明 |
|------|------|
| 服务器 | 能运行 Docker，建议有公网 IP 或域名 |
| Docker 环境 | [Docker](https://docs.docker.com/get-docker/) + Docker Compose |
| 域名 | 如 `vpn.example.com`，配置 HTTPS（推荐） |
| OIDC 提供商 | 用于用户登录。支持 **Keycloak**、**Auth0** 或任何标准 OIDC 服务 |

---

## 快速部署

### 第一步：获取项目

```bash
git clone https://github.com/alcaprophet/vpn-subscription-management.git
cd vpn-subscription-management
```

### 第二步：启动容器

**方式 A：使用预构建镜像（推荐）**

镜像自动从 GitHub Container Registry 拉取：

```bash
docker compose up -d
```

**方式 B：从源码构建**

编辑 `docker-compose.yml`，将 `image:` 行注释掉，取消 `build:` 段的注释：

```bash
docker compose build --no-cache
docker compose up -d
```

启动后：
- 单容器监听 `127.0.0.1:8080`（Go 后端 serve API + 静态文件 + SPA，仅本机可访问）

### 第三步：配置外部反向代理

容器端口仅绑定 `127.0.0.1`，需通过外部 NGINX 将域名流量转发。参考配置见 `deploy/nginx-example.conf`：

```nginx
server {
    listen 443 ssl;
    server_name vpn.example.com;

    # 所有请求 → 单容器（Go 后端 serve 一切）
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        client_max_body_size 55m;
    }
}
```

> 完整模板（含 HTTP→HTTPS 跳转、TLS 配置注释）见 `deploy/nginx-example.conf`。

重载 NGINX：

```bash
sudo nginx -t && sudo nginx -s reload
```

### 第四步：首次配置

1. 浏览器打开 `https://你的域名`
2. 自动跳转到 **Setup 配置页**（`system_config.configured=false`）
3. 选择 OIDC 提供商类型（Keycloak / Auth0 / 通用 OIDC）
4. 填写连接参数。**回调地址和前端地址已自动填入**当前浏览器域名
   - 回调地址：`https://你的域名/api/v1/auth/callback`
   - 前端地址：`https://你的域名`
5. 在 OIDC 提供商后台，将 Redirect URI 配置为上述回调地址
6. 点击「测试连接」验证 → 点击「完成配置」→ 系统自动生成 JWT_SECRET
7. 跳转到登录页，使用 OIDC 账号登录
8. **第一个登录的用户自动成为管理员**（`role=admin`, `is_advanced=true`）

---

## 使用流程

### 管理员

登录后点击顶部「管理面板」进入后台，左侧边栏导航：

| 模块 | 说明 |
|------|------|
| **订阅管理** | 为每个平台（Clash Verge、v2rayNG、Shadowrocket）创建默认和高级两种订阅。支持文件上传与文本编辑两种方式创建新版本，最多保留 5 个历史版本，可预览、切换当前版本、删除旧版本 |
| **分享订阅** | 创建不绑定用户的公开分享链接。每个分享订阅自动生成独立下载 Token，支持版本管理、Token 刷新和吊销。下载端点无需认证 |
| **平台管理** | 管理 VPN 客户端平台，配置一键导入的 `client_schemes` 和客户端下载链接 `download_url` |
| **用户管理** | 查看已登录用户（OIDC 自动创建），设置 is_advanced（普通↔高级），上传自定义订阅（指定平台，覆盖默认/高级分配），吊销所有 Token，删除用户（含管理员自我保护） |
| **规则管理** | 上传 Shadowrocket 分流规则，独立版本管理与下载 Token，支持 Token 轮替 |
| **面板配置** | 查看和修改 OIDC 提供商参数（含测试连接、切换提供商类型）、速率限制参数（登录/下载 API 限流）、系统公告栏 |
| **日志查看** | 按日期筛选访问日志，查看下载类型、用户、平台、成功/失败状态及失败原因 |

### 普通用户

1. 浏览器打开网站，使用 OIDC 账号登录
2. 首页根据 `is_advanced` 显示对应订阅卡片（默认订阅或高级订阅）
3. 若管理员分配了自定义订阅，对应平台卡片显示"已被分配自定义订阅"提示 + 自定义订阅操作按钮
4. 操作按钮：
   - **一键导入** — 拼接 `client_scheme` URL 并跳转，唤起 VPN 客户端
   - **复制链接** — 弹窗显示订阅 URL，用户手动复制到客户端
   - **刷新链接** — 轮替下载 Token（旧 Token 立即失效）
5. 获取链接后**无需再登录**，VPN 客户端通过下载 Token 自动拉取最新配置
6. 可访问「规则浏览」页面（`/rules`）查看和下载分流规则

### 自定义订阅（管理员功能）

- 在「用户管理」中为特定用户上传自定义订阅文件（需指定适用平台）
- 该用户在该平台的默认/高级订阅被替换为自定义内容
- 自定义订阅同样支持版本管理（最多 5 个版本）
- 删除自定义订阅后，用户自动恢复 `is_advanced` 对应的默认/高级订阅

---

## 架构

```
用户浏览器 ──HTTPS──▶ 外部 NGINX (vpn.example.com)
                       └─ /* → http://127.0.0.1:8080  (Go 后端, 单容器)
                             ├─ /api/v1/*  → Gin API
                             ├─ /assets/*  → Vite 构建的静态资源 (JS/CSS)
                             └─ /*         → index.html (SPA history 模式回退)
                                      │
                               vpn-data Volume
                         ┌───────────┼───────────┐
                      vpn.db    subscriptions/  rules/
                                custom/         shares/
```

- **单容器架构** — Go 后端 serve 一切：API + 静态文件 + SPA 回退。外部 NGINX 仅作 TLS 终止
- **零外部依赖** — SQLite 嵌入式数据库（`modernc.org/sqlite`，纯 Go，零 CGO）
- **单数据卷** — 所有持久化数据（数据库 + 配置文件 + 规则 + 自定义订阅 + 分享订阅）统一在 `vpn-data` volume 中
- **端口隔离** — 容器端口只绑 `127.0.0.1`，不直接暴露到公网
- **无需重启** — Setup 完成后自动切换为 Normal 模式，OIDC 配置修改即时生效
- **前端同源** — 生产部署使用相对路径 `/api/v1/`，开发/生产一致，无跨域问题

---

## 支持的 VPN 客户端

| 平台 | 客户端 | Scheme 示例 |
|------|--------|-------------|
| Windows / macOS / Linux | Clash Verge | `clash://install-config?url=` |
| Android | v2rayNG | `v2rayng://install-config?url=` |
| iOS | Shadowrocket | `shadowrocket://install-config?url=` |

系统初始化时自动创建以上三个平台。管理员可在「平台管理」中添加自定义平台、修改 scheme 或设置客户端下载链接（首页"下载客户端"按钮）。

---

## 技术栈

| 层 | 技术 |
|----|------|
| 后端 | Go 1.25 + Gin + zerolog |
| 数据库 | SQLite (`modernc.org/sqlite`，纯 Go 驱动) |
| 认证 | OIDC (PKCE) + JWT (`golang-jwt/jwt/v5`) |
| 加密 | AES-256-GCM（JWT_SECRET 前 32 字节作为加密密钥） |
| 前端 | Vue 3 (Composition API + `<script setup>`) + Vite |
| UI 库 | Element Plus + Pinia + Vue Router |
| HTTP | Axios（统一 baseURL `/api/v1`，401 自动登出） |
| 容器化 | Docker 多阶段构建（单镜像）+ distroless 运行时 |
| CI/CD | GitHub Actions（自动构建并推送到 GHCR） |

---

## 日常维护

```bash
# 查看运行状态
docker compose ps

# 查看日志
docker compose logs -f app

# 重启服务
docker compose restart

# 更新到最新镜像
docker compose pull && docker compose up -d

# 备份数据
docker run --rm -v vpn-subscription-management_vpn-data:/data \
  -v $(pwd):/backup alpine tar czf /backup/vpn-backup.tar.gz -C /data .

# 恢复数据
docker run --rm -v vpn-subscription-management_vpn-data:/data \
  -v $(pwd):/backup alpine tar xzf /backup/vpn-backup.tar.gz -C /data
```

> ⚠️ Volume 名称 `vpn-subscription-management_vpn-data` 取决于项目目录名。使用 `docker volume ls` 查看实际名称。

---

## 常见问题

**Q: 能否不用域名？**
可以，但强烈建议使用域名 + HTTPS。OIDC 登录要求回调地址为 HTTPS（localhost 除外）。

**Q: 能否不用 OIDC 登录？**
目前仅支持 OIDC 认证。如果没有 OIDC 服务，可自行搭建 Keycloak（同样是 Docker 一键部署）。

**Q: 订阅链接安全吗？**
每个用户有独立的下载 Token。Token 泄露后可点击「刷新链接」轮替（旧 Token 立即失效），管理员也可在后台吊销用户所有 Token。

**Q: 如何从源码构建？**
编辑 `docker-compose.yml`，注释 `image:` 行，取消 `build:` 段注释，然后 `docker compose build --no-cache && docker compose up -d`。

**Q: 文件上传大小限制？**
50MB，前端（el-upload）和后端（multipart + JSON MaxBytesReader）双重校验。

**Q: 忘记管理员密码怎么办？**
没有"密码"的概念 — 登录完全通过 OIDC 提供商。如果 OIDC 不可用，需检查 OIDC 服务状态。

**Q: 如何切换 OIDC 提供商？**
管理员在「管理面板 → 面板配置」中切换提供商类型、修改参数、测试连接。切换时已填字段会被保留。

**Q: 普通用户和高级用户有什么区别？**
管理员在「用户管理」中设置 `is_advanced`。普通用户获得默认订阅，高级用户获得高级订阅。管理员始终为高级（后端强制），且可为特定用户上传自定义订阅覆盖自动分配。

**Q: 日志保留多久？**
默认 90 天，系统每小时自动清理过期记录。

**Q: 管理员能删除自己吗？**
不能。系统有多重保护：禁止删除自己、禁止删除最后一个管理员、禁止修改自己的 role。

---

## 项目信息

- **许可证**: [MIT](LICENSE)
- **仓库**: [github.com/alcaprophet/vpn-subscription-management](https://github.com/alcaprophet/vpn-subscription-management)
- **容器镜像**: `ghcr.io/alcaprophet/vpn-subscription-manager`
- **完全自托管** — 不依赖任何云服务，所有数据在你自己的服务器上
