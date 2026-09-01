---
kind: external_dependency
name: Docker 镜像制品仓库 ghcr.io
slug: ghcr
category: external_dependency
category_hints:
    - vendor_identity
    - client_constraint
scope:
    - '**'
source_files:
    - .github/workflows/docker-build.yml
    - README.md
---

### 角色与定位
CI 构建产物发布到 GitHub Container Registry，镜像名为 `ghcr.io/<owner>/vpnmanagement`，TAG 格式为 `vX.X.X`（semver raw）+ `latest`（默认分支）+ `sha`。用户通过 `docker compose` 引用该镜像一键部署。

### 触发条件
- push `v*` 标签自动触发；也可手动 `workflow_dispatch` 触发。
- 使用 `GITHUB_TOKEN` 鉴权（packages: write），无需额外 PAT。

### 已知约束
- 当前 workflow 未配置 `platforms`，仅产出 `linux/amd64` 单平台镜像；Apple Silicon（arm64）无法直接拉取，需 QEMU + Buildx 多平台构建。
- README 中快速开始示例使用 `ghcr.io/alcaprophet/vpnmanagement:latest`，需与仓库 owner 一致。

### 建议
如需 arm64 原生支持，workflow 的 `build-push-action` 需追加 `platforms: linux/amd64,linux/arm64`。