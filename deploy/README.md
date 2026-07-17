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
- 后端容器监听 `127.0.0.1:8080`（Gin API）
- 前端容器监听 `127.0.0.1:8081`（Nginx 静态文件）
- 数据持久化在 Docker volume `vpn-data`（`/app/data/`）

### 3. 配置外部 NGINX 反向代理

将 `deploy/nginx-example.conf` 的内容合并到部署机已有的 NGINX 配置中：

```bash
# 1. 复制配置文件
sudo cp deploy/nginx-example.conf /etc/nginx/sites-available/vpn-sub

# 2. 修改 server_name 为你的域名
sudo vim /etc/nginx/sites-available/vpn-sub

# 3. 启用站点
sudo ln -s /etc/nginx/sites-available/vpn-sub /etc/nginx/sites-enabled/

# 4. 测试配置
sudo nginx -t

# 5. 重载 NGINX
sudo nginx -s reload
```

### 4. 访问系统

打开浏览器访问 `https://your-domain.com`，自动进入 Setup 首次配置流程。

---

## 架构说明

```
用户浏览器
   ↓ HTTPS (443)
外部 NGINX (部署机, vpn.example.com)
   ├─ /api/* → http://127.0.0.1:8080  (backend 容器)
   └─ /*     → http://127.0.0.1:8081  (frontend 容器)
   ↓
docker-compose (两个容器, 端口均绑 127.0.0.1)
   ├─ backend  :8080 → Gin API (不对外)
   └─ frontend :8081 → Nginx 静态文件 (无 proxy_pass)
```

- 对外只暴露外部 NGINX 的 443 端口
- 容器端口仅绑定 `127.0.0.1`，外部网络不可达
- `/api/` 分流完全由外部 NGINX 承担，容器内不做反代

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
