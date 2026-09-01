---
kind: dependency_management
name: Go + Node.js 双栈依赖管理（go.mod/go.sum 与 npm lockfile）
category: dependency_management
scope:
    - '**'
source_files:
    - backend/go.mod
    - backend/go.sum
    - frontend/package.json
    - frontend/package-lock.json
    - Dockerfile
---

## 1. 使用的系统与工具

本项目采用**双栈依赖管理**：
- **后端（Go）**：使用 Go Modules，通过 `backend/go.mod` 声明直接依赖、`backend/go.sum` 锁定所有间接依赖版本。
- **前端（Vue 3）**：使用 npm，通过 `frontend/package.json` 声明依赖、`frontend/package-lock.json` 锁定版本。
- **构建期**：Docker 多阶段构建中，前端阶段执行 `npm ci`（基于 lockfile 精确安装），后端阶段执行 `go mod download` 拉取依赖后编译为静态二进制。

## 2. 关键文件

- `backend/go.mod`：模块名为 `vpn-sub`，Go 版本要求 `1.25.0`，直接依赖仅 4 个：`gin-gonic/gin`、`golang-jwt/jwt/v5`、`golang.org/x/crypto`、`modernc.org/sqlite`。其余均为 indirect 传递依赖。
- `backend/go.sum`：由 `go mod tidy` 生成，锁定全部依赖的校验和。
- `frontend/package.json`：定义 `dependencies`（运行时：ant-design-vue、axios、pinia、vue、vue-router、dayjs、markdown-it）与 `devDependencies`（vite、vitest、typescript、tailwindcss 等）。
- `frontend/package-lock.json`：npm 锁文件，保证 CI/构建可重现。
- `Dockerfile`：三阶段构建——Node 22 构建前端产物 → Go 1.25 下载 go.mod/go.sum 并 CGO_ENABLED=0 静态编译 → Alpine 3.21 最小运行时。
- `.dockerignore`：排除无关目录，减少镜像体积。

## 3. 架构与约定

- **无 vendor 目录**：未使用 `go mod vendor`，依赖从公共 Go Proxy 拉取。
- **无私有代理或 replace 指令**：仓库中未发现 `GOPRIVATE`、`replace` 语句或自定义 `go.mod` proxy 配置，所有 Go 依赖均指向公共 GitHub/GOPROXY。
- **严格版本锁定**：前端使用 `package-lock.json` + `npm ci`；后端使用 `go.sum`，两者均在 Dockerfile 中作为缓存层基础，确保构建可重现。
- **CGO 禁用**：后端以 `CGO_ENABLED=0 GOOS=linux` 编译，避免引入系统库依赖，使最终镜像仅依赖 Alpine 基础镜像。
- **单一入口**：后端通过 `cmd/server/main.go` 启动，所有业务逻辑位于 `internal/` 下，依赖边界清晰。

## 4. 约定与约束

- Go 依赖版本由 `go.mod` 显式声明，间接依赖由 `go.sum` 锁定，升级需通过 `go get -u` 并配合 `go mod tidy`。
- 前端依赖版本在 `package.json` 中以 `^` 语义化版本范围声明，实际安装版本由 `package-lock.json` 固定，CI 必须使用 `npm ci` 而非 `npm install`。
- 构建过程禁止引入额外依赖：Dockerfile 分阶段隔离前端与后端依赖，最终镜像仅包含编译后的二进制与必要运行时。
- 数据持久化通过 `/data` 卷挂载，不将数据库文件打入镜像，依赖关系与运行态解耦。
- 未发现私有仓库、token 或企业级依赖治理流程（如 Dependabot、Renovate 等），依赖更新为手动维护。