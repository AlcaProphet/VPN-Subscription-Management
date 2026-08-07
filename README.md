# VPN 订阅管理系统 — 部署向导

自托管 VPN 订阅管理系统：单容器 + SQLite，面向小团队。

## 快速开始

```bash
docker compose up -d
```

浏览器访问 `http://<主机>:8080` → 首次启动自动进入 **Setup 引导**（快速开始一键完成配置）。

## 接入方式 A：外部反代（推荐，公网部署）

`docker-compose.yml` 默认仅将端口绑定到回环地址（`127.0.0.1:8080:8080`），由外部反代承担 TLS 与域名接入：

```nginx
# Nginx 示例
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

> ⚠️ **OIDC 回调与验证码依赖公网可达的 HTTPS 域名**：请确保域名解析与证书配置完成后，再进行 OIDC 高级配置。

## 接入方式 B：局域网直连

注释掉 `ports` 中的 `127.0.0.1:8080:8080`，改用 `8080:8080` 直接暴露端口。

> ⚠️ **HTTP 明文风险提示**：直连模式下凭据与订阅链接均明文传输，**仅限可信内网使用**，公网部署务必使用方式 A。

## 抢注窗口提示

**Setup 完成后请立即注册/登录成为管理员**：首个完成认证的用户自动成为管理员。公网部署下存在被他人抢注的风险，请部署者本人尽快完成注册。

## 应急恢复

管理员密码救援：在 `docker-compose.yml` 中取消 `RESET_ADMIN_PASSWORD` 注释并重启容器，进入应急恢复模式（逻辑在 Build3 实现）。操作码输出在容器运行日志中：

```bash
docker compose logs vpn-sub | grep 操作码
```

完成后请移除该环境变量并重启容器。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `APP_MODE` | `prod` | `dev` / `prod`（决定数据库文件与功能差异） |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `LOG_FORMAT` | `console` | `console` / `json` |
| `PORT` | `8080` | 监听端口 |
| `TRUST_PROXY` | `auto` | `auto` / `on` / `off` |
| `DATA_DIR` | `./data` | 数据目录（容器内固定 `/data`） |
| `RESET_ADMIN_PASSWORD` | — | 应急恢复触发（Build3） |

## 开发联调

- 后端：`cd backend && go run ./cmd/server`（内嵌占位前端，仅 API）
- 前端：`cd frontend && npm run dev`（Vite dev server 代理 `/api` 到 `127.0.0.1:8080`）
