---
kind: build_system
name: 多阶段 Docker 构建与单容器一体化编排
category: build_system
scope:
    - '**'
source_files:
    - Dockerfile
    - .github/workflows/docker-build.yml
    - docker-compose.yml
    - backend/go.mod
    - backend/web/web.go
    - frontend/package.json
    - .smoke-test.sh
---

## 1. 构建系统总览

本项目采用 **Docker 多阶段构建** 将 Go 后端与 Vue 前端静态资源编译进单一可执行文件，并通过 `docker-compose` 统一编排数据卷、端口与环境变量实现一键部署。整个流程由 GitHub Actions 在打 tag 时触发，产物推送至 GitHub Container Registry（ghcr.io）。

## 2. 关键文件与工具链

- **Dockerfile**：三阶段构建
  - 阶段一 `node:22-alpine`：安装依赖并执行 `npm run build`，产出 `frontend/dist`
  - 阶段二 `golang:1.25-alpine`：`go mod download` 后复制前端产物到 `backend/web/dist`，再执行 `CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server`，生成静态链接的 Linux 二进制
  - 阶段三 `alpine:3.21`：最小运行时镜像，以非 root 用户 `app` 运行 `/server`，暴露 8080 端口，挂载 `/data` 数据卷
- **backend/web/web.go**：使用 `//go:embed all:dist` 将前端静态资源嵌入 Go 二进制；构建时被真实 dist 覆盖占位目录
- **.github/workflows/docker-build.yml**：GitHub Actions CI，监听 `push v*` 标签和 `workflow_dispatch`，通过 `docker/metadata-action` 生成 semver、latest、sha 标签，使用 `docker/build-push-action@v6` 构建并推送到 `ghcr.io/<owner>/vpnmanagement`
- **docker-compose.yml**：定义 `vpn-sub` 服务，默认绑定 `8080:8080`，挂载 `vpn-data:/data` 持久化数据库与内容文件，提供 healthcheck 探测 `/health`
- **backend/go.mod**：Go 模块 `vpn-sub`，基于 Go 1.25.0，核心依赖 Gin、JWT、modernc.org/sqlite（纯 Go SQLite，配合 CGO_ENABLED=0 静态编译）
- **frontend/package.json**：Vue 3 + Vite 项目，`build` 脚本为 `vue-tsc -b && vite build`，测试使用 Vitest
- **.smoke-test.sh**：冒烟测试脚本，启动后依次调用 setup、注册管理员、创建订阅/版本/组、下载、分享、规则等 API，验证端到端功能

## 3. 架构与设计约定

- **前后端一体化打包**：前端构建产物通过 Docker 多阶段复制到后端源码的 `web/dist`，再由 Go embed 机制打入最终二进制，运行时无需外部 Web 服务器
- **静态编译与最小镜像**：后端使用 `CGO_ENABLED=0` 编译，依赖 modernc.org/sqlite 避免 C 依赖，最终镜像仅包含 alpine 基础镜像 + 单个二进制
- **无状态应用 + 数据卷持久化**：所有持久化数据（SQLite 数据库、安装包、站点资源、/public）统一放在 `/data` 卷，容器本身不可变
- **环境变量驱动配置**：通过 `APP_MODE`、`LOG_LEVEL`、`TRUST_PROXY`、`RESET_ADMIN_PASSWORD` 等环境变量控制行为，支持开发/生产模式切换
- **CI/CD 触发策略**：仅对 `v*` 标签自动构建并发布 latest 镜像；手动触发 workflow_dispatch 可用于排查或重跑
- **安全基线**：容器内以非 root 用户 `app` 运行，健康检查使用 wget 探测 `/health` 端点

## 4. 约束与规范

- 镜像名必须小写（OCI 规范），通过 ghcr.io 发布
- 版本号遵循 semver 格式（如 `v1.2.3`），由 git tag 驱动
- 构建环境固定：Node 22、Go 1.25、Alpine 3.21
- 前端构建必须在后端编译之前完成，产物路径固定为 `backend/web/dist`
- 数据卷 `/data` 是唯一的持久化边界，迁移脚本位于 `backend/migrations/` 并以 SQL 文件形式管理
- 本地开发可通过 `docker-compose up` 直接构建镜像，也可分别运行前端 dev server 与后端 serve 进行调试