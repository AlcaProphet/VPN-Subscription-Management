---
kind: dependency_management
name: Go + Node.js 双栈依赖管理与单容器打包
category: dependency_management
scope:
    - '**'
source_files:
    - backend/go.mod
    - backend/go.sum
    - frontend/package.json
    - frontend/package-lock.json
    - Dockerfile
    - .github/workflows/docker-build.yml
    - docker-compose.yml
---

## 1. 使用的系统与工具

本项目采用 **Go + Node.js 双栈** 的依赖管理方式，并通过多阶段 Dockerfile 将前后端依赖统一打包进单一可执行镜像：

- **后端（Go）**：使用 `go mod` 进行依赖声明与版本锁定，依赖清单位于 `backend/go.mod` 与 `backend/go.sum`。
- **前端（Vue 3 + Vite）**：使用 npm 包管理器，依赖声明在 `frontend/package.json`，锁定文件为 `frontend/package-lock.json`。
- **构建与发布**：通过 GitHub Actions（`.github/workflows/docker-build.yml`）触发基于 Docker Buildx 的多阶段构建，并将产物推送到 `ghcr.io`。

## 2. 关键文件

- `backend/go.mod` / `backend/go.sum` — Go 模块定义、直接/间接依赖及精确版本锁定。
- `frontend/package.json` / `frontend/package-lock.json` — 前端运行时与开发依赖声明及锁定。
- `Dockerfile` — 三阶段构建：Node 构建前端静态资源 → Go 编译后端并 embed 前端产物 → Alpine 最小运行时。
- `.github/workflows/docker-build.yml` — 标签触发构建并推送至 GitHub Container Registry。
- `docker-compose.yml` — 本地编排，挂载 `vpn-data` 卷持久化数据。

## 3. 架构与约定

### 3.1 Go 依赖管理
- 模块名为 `vpn-sub`，Go 版本固定为 `1.25.0`。
- 仅引入少量核心依赖：Gin 框架、JWT、SQLite（modernc.org/sqlite）、golang.org/x/crypto，其余均为间接依赖。
- 未使用 vendor 目录，依赖由 `go.sum` 锁定；Docker 构建中先 `COPY go.mod go.sum` 再 `go mod download`，利用 Docker 层缓存加速重复构建。
- 编译时启用 `CGO_ENABLED=0`、`-trimpath` 及 `-ldflags="-s -w"`，生成无 CGO、去符号、去调试信息的最小二进制。

### 3.2 Node.js 依赖管理
- 生产依赖（dependencies）包含 Vue 3、Ant Design Vue、Pinia、Axios、Dayjs、Markdown-it 等。
- 开发依赖（devDependencies）包含 Vite、TypeScript、Vitest、TailwindCSS、PostCSS、vue-tsc 等。
- 使用 `npm ci` 安装（Dockerfile 阶段一），确保与 `package-lock.json` 完全一致，禁止增量变更。
- 前端构建产物输出到 `frontend/dist`，被第二阶段 COPY 进 Go 源码的 `web/dist` 目录，最终通过 Go embed 机制嵌入二进制。

### 3.3 多阶段构建与镜像分层
```
阶段一 (node:22-alpine)：npm ci + npm run build → 产出 frontend/dist
阶段二 (golang:1.25-alpine)：go mod download + go build -o server → 产物 /out/server
阶段三 (alpine:3.21)：仅拷贝 /server，创建非 root 用户 app，暴露 8080
```

### 3.4 CI/CD 集成
- 仅在 push `v*` 标签或手动触发 workflow_dispatch 时运行。
- 使用 `docker/metadata-action` 生成 semver、latest、sha 多标签。
- 通过 GITHUB_TOKEN 登录 ghcr.io 并推送镜像。

## 4. 约定与约束

- **依赖版本锁定**：Go 使用 `go.sum`，Node 使用 `package-lock.json`，均提交到仓库，保证构建可重现。
- **私有依赖**：当前未发现 GOPRIVATE、私有 Go module proxy 或 npm registry 配置；所有依赖均来自公开源（pkg.go.dev、registry.npmjs.org、GitHub）。如需引入私有模块，需在 CI 环境变量中配置 `GOPRIVATE` 或使用 `~/.netrc`/`GONOSUMDB`。
- **安全加固**：Go 编译禁用 CGO、strip 符号与调试信息；容器以非 root 用户 `app` 运行。
- **数据持久化**：通过 docker-compose 的 `vpn-data` 卷持久化 SQLite 数据库与应用数据，不随镜像重建丢失。
- **前端资源内嵌**：前端静态资源在构建期被编译并嵌入 Go 二进制，运行时无需独立 Web 服务器。
- **镜像最小化**：最终镜像仅基于 alpine:3.21，体积尽可能小，仅包含运行时所需系统命令（wget 用于 healthcheck）。

## 5. 注意事项

- 若未来引入私有 Go 模块或私有 npm 包，需同步更新 Dockerfile 构建上下文与 GitHub Actions 的认证配置。
- 当前未使用 Go workspace 或多模块结构，所有 Go 代码集中在 `backend/` 下单一模块。
- 前端构建依赖 Node 22，Go 构建依赖 Go 1.25，CI 与本地环境需保持版本一致以避免依赖解析差异。