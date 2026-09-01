---
kind: build_system
name: Docker 多阶段构建与 CI/CD 发布流水线
category: build_system
scope:
    - '**'
source_files:
    - Dockerfile
    - .github/workflows/docker-build.yml
    - docker-compose.yml
    - backend/go.mod
    - frontend/package.json
    - backend/migrations/embed.go
    - .smoke-test.sh
---

## 1. 构建系统总览

本项目采用 **Docker 多阶段构建** 作为统一的编译、打包与分发入口，将前端（Vue 3 + Vite）、后端（Go/Gin）和静态资源合并为单一可执行镜像，并通过 GitHub Actions 在打 tag 时自动推送到 ghcr.io。

- 构建工具链：`node:22-alpine` 负责前端 `npm ci` + `vue-tsc -b && vite build`；`golang:1.25-alpine` 负责 Go 静态编译（`CGO_ENABLED=0`，`GOOS=linux`，`-trimpath -ldflags="-s -w"`）；最终运行时基于 `alpine:3.21`。
- 产物形态：单个 `/server` 二进制，内嵌前端 `web/dist` 与 SQL 迁移文件（通过 `//go:embed *.sql`），数据目录挂载到 `/data`，对外暴露 `8080` 端口。
- 编排与运行：根级 `docker-compose.yml` 提供一键启动（默认绑定 `8080:8080`，数据卷 `vpn-data:/data`），支持通过环境变量 `APP_MODE`、`LOG_LEVEL`、`TRUST_PROXY`、`RESET_ADMIN_PASSWORD` 控制行为。

## 2. 关键文件

| 文件 | 作用 |
|---|---|
| `Dockerfile` | 三阶段构建：前端构建 → 后端静态编译（嵌入前端 dist）→ 最小 Alpine 运行时 |
| `.github/workflows/docker-build.yml` | GitHub Actions 流水线：push `v*` 标签或手动触发，生成 semver/sha/latest 标签并推送至 `ghcr.io/<owner>/vpnmanagement` |
| `docker-compose.yml` | 开发/本地编排，含 healthcheck（`wget http://127.0.0.1:8080/health`）和数据卷持久化 |
| `backend/go.mod` | Go module 定义，依赖 Gin、JWT、modernc/sqlite（纯 Go SQLite，配合 CGO_ENABLED=0） |
| `frontend/package.json` | 前端脚本：`dev`/`build`/`test`/`test:watch`，使用 Vite + Vue-TSC + Vitest |
| `backend/migrations/embed.go` | 用 `//go:embed *.sql` 将所有迁移 SQL 内嵌进二进制，保证单文件分发 |
| `.smoke-test.sh` | 冒烟测试脚本，按顺序调用 setup、注册、创建订阅/版本/组、下载、分享、规则等核心 API，验证端到端流程 |

## 3. 架构与约定

### 3.1 多阶段构建流程
```
stage 1 (frontend) : node:22-alpine → npm ci → npm run build → /build/dist
stage 2 (backend)  : golang:1.25-alpine → go mod download → COPY --from=frontend /build/dist → CGO_ENABLED=0 go build -o /out/server
stage 3 (runtime)  : alpine:3.21 → adduser app → COPY /out/server → VOLUME /data → EXPOSE 8080 → ENTRYPOINT ["/server"]
```
- 前端产物通过 `COPY --from=frontend /build/dist ./web/dist` 注入后端，由后端 `web/web.go` 的 embed 机制在服务端直接提供静态页面。
- Go 二进制以 `CGO_ENABLED=0` 静态编译，仅依赖纯 Go 库（sqlite 使用 modernc.org/sqlite），确保跨平台可移植性。

### 3.2 版本与标签策略
- 触发条件：`push v*` 标签（如 `v1.2.3`）或 `workflow_dispatch` 手动触发。
- 标签生成（`docker/metadata-action`）：
  - semver raw：`vX.X.X`
  - sha：完整 commit sha
  - latest：仅在默认分支（main）上手动触发时生成
- 镜像仓库：`ghcr.io/${{ github.repository_owner }}/vpnmanagement`，权限 `packages: write`，认证使用 `GITHUB_TOKEN`。

### 3.3 数据与配置持久化
- 所有状态（SQLite 数据库、安装包、站点资源、`/public`）统一挂载到 `/data` 卷。
- 环境变量驱动行为：`APP_MODE`、`LOG_LEVEL`、`TRUST_PROXY`、`RESET_ADMIN_PASSWORD`（应急恢复管理员密码）。
- Healthcheck 通过 `wget` 探测 `/health` 接口，间隔 30s，超时 5s，重试 3 次。

### 3.4 测试与验证
- 前端单元测试：`npm test`（Vitest），位于 `frontend/tests/`。
- 冒烟测试：`.smoke-test.sh` 通过 curl 依次调用 setup、auth、subscription、version、group、download、share、rule 等 API，覆盖核心业务流程。
- 数据库迁移：SQL 文件位于 `backend/migrations/`，命名规范 `NNNN_xxx.sql`，通过 `embed.FS` 内嵌并在运行时按序应用。

## 4. 约束与约定

- **禁止 CGO**：构建强制 `CGO_ENABLED=0`，所有依赖必须为纯 Go（sqlite 使用 modernc.org/sqlite）。
- **非 root 运行**：容器内创建 `app` 用户并切换 `USER app`，数据目录 `/data` 预赋权给该用户。
- **端口固定**：服务固定监听 `8080`，compose 中通过端口映射暴露；生产环境建议仅绑定回环地址并由外部反代处理 TLS。
- **标签命名**：镜像标签遵循 OCI 小写规范，semver 标签来自 git tag，sha 标签用于精确追溯，`latest` 仅在默认分支手动触发时更新。
- **单二进制分发**：前端静态资源、SQL 迁移均通过 `embed` 内嵌，最终产物仅为一个 `/server` 可执行文件。
- **数据卷唯一挂载点**：所有持久化数据集中在 `/data`，不分散挂载多个路径。
- **健康检查接口**：必须实现 `/health` 端点供 compose healthcheck 探测。