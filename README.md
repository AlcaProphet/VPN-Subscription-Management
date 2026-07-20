# VPN 订阅管理

一个轻量级的自托管 VPN 订阅管理系统，专为小团队设计。管理员通过网页上传订阅配置，团队成员一键导入到 VPN 客户端，配置更新全员自动生效。

---

## 适用场景

你的团队共用 VPN 节点，每次更新配置都要逐个通知成员手动替换文件？这个工具帮你解决：

- 📤 **一次上传，全员同步** — 管理员更新订阅版本，所有成员自动获取最新配置
- 🔗 **一条链接，长期有效** — 成员拿到的订阅链接不会过期，VPN 客户端定时自动更新
- 👥 **分级管理** — 支持普通订阅和高级订阅，不同成员拿到不同节点
- 🛡️ **无需登录即可使用** — 拿到订阅链接后，VPN 客户端无需任何认证即可拉取配置

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

### 第一步：准备 docker-compose.yml

在服务器上创建一个目录，放入以下 `docker-compose.yml`：

```yaml
services:
  backend:
    image: ghcr.io/alcaprophet/vpn-sub-backend:latest
    container_name: vpn-backend
    ports:
      - "127.0.0.1:8080:8080"
    volumes:
      - vpn-data:/app/data
    restart: unless-stopped

  frontend:
    image: ghcr.io/alcaprophet/vpn-sub-frontend:latest
    container_name: vpn-frontend
    ports:
      - "127.0.0.1:8081:80"
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  vpn-data:
```

### 第二步：启动服务

```bash
docker compose up -d
```

两个容器启动后：
- 后端 API 监听 `127.0.0.1:8080`（仅本机可访问）
- 前端页面监听 `127.0.0.1:8081`（仅本机可访问）

### 第三步：配置反向代理

你需要一个外部 NGINX 将域名流量转发到这两个容器。参考配置：

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

> 💡 完整配置模板见 `deploy/nginx-example.conf`

配置好 NGINX 后重载：

```bash
sudo nginx -t && sudo nginx -s reload
```

### 第四步：首次配置

1. 浏览器打开 `https://你的域名`
2. 自动跳转到 **Setup 配置页**
3. 选择你的 OIDC 提供商（Keycloak / Auth0 / 通用 OIDC）
4. 填写连接参数 → 点击「测试连接」→ 点击「完成配置」
5. 自动跳转到登录页，使用 OIDC 账号登录
6. **第一个登录的用户自动成为管理员**

---

## 使用流程

### 管理员

登录后进入「管理面板」：

1. **创建订阅** — 为每个平台（Clash Verge、v2rayNG、Shadowrocket）上传订阅配置文件
2. **管理用户** — 查看已登录的团队成员，设置订阅级别（普通/高级）
3. **上传规则** —（可选）上传分流规则文件
4. **创建分享链接** —（可选）创建无需登录的公开订阅链接

### 普通用户

1. 浏览器打开网站，使用 OIDC 账号登录
2. 首页看到自己可用的订阅卡片
3. 点击「一键导入」唤起 VPN 客户端，或「复制链接」手动添加
4. **之后无需再登录**，VPN 客户端通过订阅链接自动获取最新配置

---

## 架构一览

```
浏览器 ──HTTPS──▶ 你的 NGINX (vpn.example.com)
                     ├─ /api/*  → 127.0.0.1:8080 (后端 API)
                     └─ /*      → 127.0.0.1:8081 (前端页面)
                                      │
                                 docker compose
                              ┌───────┴───────┐
                           backend         frontend
                          (Go + Gin)    (Vue + Nginx)
                              │
                          vpn-data (SQLite + 配置文件)
```

- **零外部依赖** — 数据库用 SQLite，无需单独安装 MySQL/Redis
- **单数据卷** — 所有数据（数据库、订阅文件、规则）都在一个 Docker volume 里
- **端口隔离** — 容器端口只绑 `127.0.0.1`，不直接暴露到公网

---

## 支持的 VPN 客户端

| 平台 | 客户端 |
|------|--------|
| Windows / macOS / Linux | Clash Verge |
| Android | v2rayNG |
| iOS | Shadowrocket |

系统初始化时自动创建以上三个平台，管理员也可自行添加更多。

---

## 日常维护

```bash
# 查看运行状态
docker compose ps

# 查看后端日志
docker compose logs -f backend

# 重启服务
docker compose restart

# 备份数据（备份整个 vpn-data 卷）
docker run --rm -v vpn-sub-management_vpn-data:/data -v $(pwd):/backup alpine tar czf /backup/vpn-backup.tar.gz -C /data .

# 恢复数据
docker run --rm -v vpn-sub-management_vpn-data:/data -v $(pwd):/backup alpine tar xzf /backup/vpn-backup.tar.gz -C /data
```

---

## 常见问题

**Q: 能否不用自己的域名？**
可以，但强烈建议使用域名 + HTTPS。OIDC 登录要求回调地址为 HTTPS。

**Q: 能否不用 OIDC 登录？**
目前仅支持 OIDC 认证。如果你没有 OIDC 服务，可以自行搭建 Keycloak（也是 Docker 一键部署）。

**Q: 订阅链接安全吗？**
每个用户有独立的下载 Token。Token 泄露后用户可以自己在首页刷新，管理员也可以在后台吊销。

**Q: 文件上传大小限制？**
上传文件限制 50MB，前端和后端双重校验。

**Q: 忘记管理员密码怎么办？**
没有"密码"的概念——登录完全通过 OIDC 提供商。如果 OIDC 不可用，检查 OIDC 服务状态。

---

## 项目信息

- **许可证**: MIT
- **技术栈**: Go + Gin（后端）/ Vue 3 + Element Plus（前端）/ SQLite（数据库）
- **完全自托管** — 不依赖任何云服务，所有数据在你自己的服务器上
