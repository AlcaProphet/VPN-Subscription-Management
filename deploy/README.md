# VPN Subscription Management — 部署说明

## 快速开始

### 1. 克隆项目

```bash
git clone <your-repo-url> vpn-sub
cd vpn-sub
```

### 2. 启动服务

```bash
docker compose up -d
```

首次启动后：
- 单容器监听 `127.0.0.1:8080`（Go 后端 serve API + 静态文件 + SPA）

### 3. 配置外部 NGINX（TLS 终止）

在部署机已有的 NGINX 配置中添加 `vpn.example.com` 的 server block，参考 `deploy/nginx-example.conf`。

```bash
# 1. 复制配置文件
sudo cp deploy/nginx-example.conf /etc/nginx/sites-available/vpn-sub

# 2. 修改 server_name 为你的域名
sudo vim /etc/nginx/sites-available/vpn-sub

# 3. 启用站点
sudo ln -s /etc/nginx/sites-available/vpn-sub /etc/nginx/sites-enabled/

# 4. 测试并重载
sudo nginx -t && sudo nginx -s reload
```

**健康检查**: 外部 WAF/LB 健康探测请指向 `/health`（返回 `{"status":"ok"}`），不要指向 `/`（该路径返回 SPA 的 index.html）。

### 4. 访问系统

打开浏览器访问 `https://your-domain.com`，自动进入 Setup 首次配置流程。

---

## 架构说明

```
用户浏览器
   ↓ HTTPS (443)
外部 NGINX (部署机, vpn.example.com)
   └─ /* → http://127.0.0.1:8080  (单容器, Go serve 一切)
         ├─ /api/v1/*  → Gin API
         ├─ /assets/*  → Vite 静态资源
         └─ /*         → index.html (SPA 回退)
```

- 对外只暴露外部 NGINX 的 443 端口
- 容器端口仅绑定 `127.0.0.1`，外部网络不可达
- Go 后端统一 serve API + 静态文件 + SPA，外部 NGINX 不做路径分流

---

## 数据持久化

所有数据存储在 Docker Volume `vpn-data` 中，挂载到容器的 `/app/data`：
- SQLite 数据库 (`vpn.db`)
- 订阅/规则/自定义订阅/分享订阅版本文件

### 备份与恢复

```bash
# 备份
docker run --rm -v vpn-sub_vpn-data:/data \
  -v $(pwd):/backup alpine tar czf /backup/vpn-backup.tar.gz -C /data .

# 恢复
docker run --rm -v vpn-sub_vpn-data:/data \
  -v $(pwd):/backup alpine tar xzf /backup/vpn-backup.tar.gz -C /data
```

> ⚠️ Volume 名称取决于项目目录名。使用 `docker volume ls` 查看实际名称。

---

> **架构变更 (v2.0)**: 从双容器（backend + frontend nginx）合并为单容器架构。旧的 `backend/Dockerfile` 和 `frontend/Dockerfile` 已废弃，仅保留备用。

---

## 常用运维命令

```bash
# 查看容器状态
docker compose ps

# 查看日志
docker compose logs -f backend
docker compose logs -f frontend

# 重启服务
docker compose restart

# 停止服务
docker compose down

# 更新镜像并重启
docker compose pull
docker compose up -d

# 清理所有数据（包括数据库和配置文件）
docker compose down -v
```

## 数据备份

所有持久化数据存储在 Docker volume `vpn-data` 中：

```bash
# 备份数据库
docker compose exec backend cat /app/data/vpn.db > vpn.db.backup
# 注意: distroless 镜像无 shell，需从 volume 直接复制
docker run --rm -v vpn-sub_vpn-data:/data alpine cp /data/vpn.db /data/vpn.db.backup

# 或直接访问 volume 路径（macOS Docker Desktop）
# /var/lib/docker/volumes/vpn-sub_vpn-data/_data/
```
