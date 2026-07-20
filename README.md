# VPN 订阅管理

一个轻量级的自托管 VPN 订阅管理系统，专为小团队设计。管理员通过网页上传订阅配置，团队成员一键导入到 VPN 客户端，配置更新全员自动生效。

---

## 功能特性

- 📤 **一次上传，全员同步** — 管理员更新订阅版本，所有成员自动获取最新配置
- 🔗 **一条链接，长期有效** — 成员拿到的订阅链接不会过期，VPN 客户端定时自动更新
- 👥 **分级管理** — 支持默认订阅和高级订阅，不同成员拿到不同节点
- 🛡️ **无需登录即可使用** — 拿到订阅链接后，VPN 客户端无需任何认证即可拉取配置
- 🔧 **自定义订阅** — 管理员可为特定用户分配独立配置，覆盖默认/高级自动分配
- 🔗 **分享订阅** — 创建无需登录的公开分享链接，适合外部人员或团队公共配置
- 📋 **分流规则管理** — 支持 Shadowrocket 规则文件，独立版本管理与下载 Token
- 📊 **访问日志** — 记录所有下载请求，按日期筛选，区分成功/失败及原因
- ⚡ **速率限制** — 登录和下载 API 可配置限流，防止滥用
- 🌓 **暗色模式** — 支持深色/浅色主题切换，自动跟随系统偏好
- 📱 **移动端适配** — 响应式布局，手机和桌面均可流畅使用
- 🐳 **Docker 一键部署** — 零外部依赖，SQLite 嵌入式存储，`docker compose up -d` 即可启动

---

## 前置要求

开始之前，确保你具备：

| 条件 | 说明 |
|------|------|
| 一台服务器 | 能运行 Docker，有公网 IP 或域名 |
| Docker 环境 | 安装 [Docker](https://docs.docker.com/get-docker/) 和 Docker Compose |
| 域名 | 如 `vpn.example.com`，配置了 HTTPS（推荐） |
| OIDC 提供商 | 用于用户登录。支持 Keycloak、Auth0、或任何标准 OIDC 服务 |

---

## 快速部署

### 第一步：获取项目

```bash
git clone https://github.com/alcaprophet/vpn-subscription-management.git
cd vpn-subscription-management
```

### 第二步：选择部署方式

**方式 A：使用预构建镜像（推荐）**

直接启动，镜像自动从 GitHub Container Registry 拉取：

```bash
docker compose up -d
```

**方式 B：从源码构建**

如果你修改了代码或想使用最新提交：

```bash
# 编辑 docker-compose.yml，将 image 行替换为 build 段（文件中已有注释说明）
docker compose build --no-cache
docker compose up -d
```

两个容器启动后：
- 后端 API 监听 `127.0.0.1:8080`（仅本机可访问）
- 前端页面监听 `127.0.0.1:8081`（仅本机可访问）

### 第三步：配置反向代理

你需要一个外部 NGINX 将域名流量转发到这两个容器。参考配置见 `deploy/nginx-example.conf`：

```nginx
server {
    listen 443 ssl;
    server_name vpn.example.com;   # 替换为你的域名

    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        client_max_body_size 55m;
    }

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

> 💡 完整模板（含 HTTP→HTTPS 跳转、TLS 配置注释）见 `deploy/nginx-example.conf`
>
> **关于 `location /api/`**：这是 NGINX 前缀匹配，会自动匹配所有以 `/api/` 开头的请求，包括 `/api/v1/auth/login` 等。不需要写成 `/api/v1/`，系统所有 API 都在 `/api/v1/` 路径下。

配置好 NGINX 后重载：

```bash
sudo nginx -t && sudo nginx -s reload
```

### 第四步：首次配置

1. 浏览器打开 `https://你的域名`
2. 自动跳转到 **Setup 配置页**
3. 选择你的 OIDC 提供商（Keycloak / Auth0 / 通用 OIDC）
4. 填写连接参数。**回调地址和前端地址已自动填入**当前浏览器域名，通常无需修改
   - 回调地址格式：`https://你的域名/api/v1/auth/callback`
   - 前端地址格式：`https://你的域名`
5. 在你的 OIDC 提供商后台，将回调地址（Redirect URI）配置为 `https://你的域名/api/v1/auth/callback`
6. 点击「测试连接」验证 OIDC 配置正确 → 点击「完成配置」
7. 自动跳转到登录页，使用 OIDC 账号登录
8. **第一个登录的用户自动成为管理员**

---

## 使用流程

### 管理员

登录后点击顶部「管理面板」进入后台：

| 功能 | 说明 |
|------|------|
| **订阅管理** | 为每个平台（Clash Verge、v2rayNG、Shadowrocket）创建默认和高级两种订阅，上传配置文件，支持版本管理（最多保留 5 个历史版本，可切换/预览/回滚） |
| **分享订阅** | 创建不绑定用户的公开分享链接，适合外部人员或团队公共配置。支持 Token 刷新和吊销 |
| **平台管理** | 管理 VPN 客户端平台，配置一键导入的 scheme 和客户端下载链接 |
| **用户管理** | 查看已登录用户，设置订阅级别（普通↔高级），上传自定义订阅（覆盖默认/高级分配），吊销 Token，删除用户 |
| **规则管理** | 上传 Shadowrocket 分流规则文件，独立版本管理和下载 Token，支持 Token 轮替 |
| **OIDC 配置** | 查看和修改 OIDC 提供商参数，切换提供商类型，测试连接。Client Secret 加密存储 |
| **速率限制** | 配置登录和下载 API 的速率限制（默认登录 10 次/分钟，下载 20 次/分钟） |
| **日志查看** | 按日期筛选访问日志，查看下载类型、用户、平台、成功/失败状态及原因 |

### 普通用户

1. 浏览器打开网站，使用 OIDC 账号登录
2. 首页看到自己可用的订阅卡片（根据级别显示默认订阅或高级订阅）
3. 点击「一键导入」唤起 VPN 客户端，或「复制链接」手动添加
4. 可点击「刷新链接」轮替自己的下载 Token
5. **之后无需再登录**，VPN 客户端通过订阅链接自动获取最新配置
6. 可访问「规则浏览」页面查看和下载分流规则

### 自定义订阅（管理员功能）

管理员可为特定用户分配独立的订阅配置：
- 在用户管理页选择用户 → 上传自定义订阅 → 指定适用平台
- 该用户在该平台的默认/高级订阅被替换为自定义内容
- 删除自定义订阅后，用户自动恢复默认/高级自动分配

---

## 架构一览

```
浏览器 ──HTTPS──▶ 你的 NGINX (vpn.example.com)
                     ├─ /api/*  → 127.0.0.1:8080 (后端 API, 路径 /api/v1/*)
                     └─ /*      → 127.0.0.1:8081 (前端页面)
                                      │
                                 docker compose
                              ┌───────┴───────┐
                           backend         frontend
                          (Go + Gin)    (Vue + Nginx)
                              │
                          vpn-data (SQLite + 配置文件)
```

- **API 路径**: 所有接口在 `/api/v1/` 下（如 `/api/v1/auth/login`），NGINX 用 `/api/` 前缀匹配覆盖全部
- **零外部依赖** — 数据库用 SQLite，无需单独安装 MySQL/Redis
- **单数据卷** — 所有数据（数据库、订阅文件、规则、自定义订阅、分享订阅）都在一个 Docker volume 里
- **端口隔离** — 容器端口只绑 `127.0.0.1`，不直接暴露到公网
- **无需重启** — OIDC 配置完成后服务自动切换为正常模式，无需手动重启

---

## 支持的 VPN 客户端

| 平台 | 客户端 |
|------|--------|
| Windows / macOS / Linux | Clash Verge |
| Android | v2rayNG |
| iOS | Shadowrocket |

系统初始化时自动创建以上三个平台。管理员可在「平台管理」中添加更多平台、修改一键导入 scheme 或设置客户端下载链接。

---

## 日常维护

```bash
# 查看运行状态
docker compose ps

# 查看后端日志
docker compose logs -f backend

# 重启服务
docker compose restart

# 更新到最新镜像
docker compose pull && docker compose up -d

# 备份数据（备份整个 vpn-data 卷）
docker run --rm -v vpn-subscription-management_vpn-data:/data -v $(pwd):/backup alpine tar czf /backup/vpn-backup.tar.gz -C /data .

# 恢复数据
docker run --rm -v vpn-subscription-management_vpn-data:/data -v $(pwd):/backup alpine tar xzf /backup/vpn-backup.tar.gz -C /data
```

> ⚠️ 备份和恢复命令中的 volume 名称 `vpn-subscription-management_vpn-data` 取决于项目目录名。使用 `docker volume ls` 查看实际名称。

---

## 常见问题

**Q: 能否不用自己的域名？**
可以，但强烈建议使用域名 + HTTPS。OIDC 登录要求回调地址为 HTTPS（localhost 除外）。

**Q: 能否不用 OIDC 登录？**
目前仅支持 OIDC 认证。如果你没有 OIDC 服务，可以自行搭建 Keycloak（也是 Docker 一键部署）。

**Q: 订阅链接安全吗？**
每个用户有独立的下载 Token。Token 泄露后用户可以自己在首页点击「刷新链接」，管理员也可以在后台吊销该用户所有 Token。

**Q: 如何从源码构建而非使用预构建镜像？**
编辑 `docker-compose.yml`，将 `image:` 行注释掉，取消 `build:` 段的注释，然后运行 `docker compose build --no-cache && docker compose up -d`。

**Q: 文件上传大小限制？**
上传文件限制 50MB，前端和后端双重校验。

**Q: 忘记管理员密码怎么办？**
没有"密码"的概念——登录完全通过 OIDC 提供商。如果 OIDC 不可用，检查 OIDC 服务状态。如果所有管理员账号都不可用，需要操作 OIDC 提供商恢复账号访问。

**Q: 如何切换 OIDC 提供商？**
管理员在「管理面板 → OIDC 配置」中可以切换提供商类型、修改参数、测试连接。切换时已填的字段会被保留。

**Q: 普通用户和高级用户有什么区别？**
管理员在「用户管理」中设置用户的订阅级别。普通用户获得默认订阅，高级用户获得高级订阅。管理员还可以为特定用户上传自定义订阅（覆盖默认/高级分配）。

**Q: 日志保留多久？**
访问日志默认保留 90 天，系统每小时自动清理过期记录。

---

## 项目信息

- **许可证**: MIT
- **技术栈**: Go + Gin（后端）/ Vue 3 + Element Plus（前端）/ SQLite（数据库）
- **完全自托管** — 不依赖任何云服务，所有数据在你自己的服务器上
