# Build40 — Docker 默认 UID/GID 固定

> 基线 `beta` / `8a6d03a`。设计见 [Design6](../Design/Design6.md)，问题见 [Issue21](../Issue/Issue21.md) R35-09，约束见 [AGENTS](../../../AGENTS.md)。用户确认暂无数据，本轮按无数据场景验收。

## 一、构建进度

| Step | 内容 | 状态 |
|---|---|---|
| 1 | 固定默认 UID/GID，修正旧身份空目录 | 已验收 |
| 2 | Docker 回归、构建和文档同步 | 已验收 |

## 二、分步构建

### Step 1 — 固定身份与空目录修正

- 目标为消除 Alpine 自动分配 `100:101` 与宿主机系统账号的编号碰撞。
- 前置为用户授权修复，确认暂无数据并省去旧数据迁移验证。
- 产出为 `Dockerfile` 的显式 `10001:10001`、`docker-entrypoint.sh` 对空 `100:101` 目录的识别，以及 Docker 场景脚本的固定身份断言和空目录回归。
- 流程为读取目录属主 → root 目录或空的旧默认身份目录选择 app → 修改目录自身属主 → 降权验证 → exec 服务。普通用户目录和显式非 root 身份沿用已有选择流程。
- 验收命令为 `sh -n docker-entrypoint.sh`、`docker build -t vpn-sub:fixed-runtime-identity .` 和 `python3 -u scripts/test-docker-permissions.py vpn-sub:fixed-runtime-identity`。
- 验收规范为默认目录及 PID 1 使用 `10001:10001`，空 `100:101` 目录纠正后进入 Setup，普通用户目录可运行。

### Step 2 — 联合验收与文档

- 前置为 Step 1 通过。
- 产出为 Design6、Issue21、README、Compose 示例及 AGENTS 的现行行为说明与本记录。
- 验收命令为后端 `go build ./...`、`go vet ./...`、`go test ./... -count=1`，前端 `npm run build`，Compose 示例 `config --quiet`，文档内链检查和 `git diff --check`。
- 验收规范为现有启动、初始化、退出和权限失败测试通过，并记录本轮验证范围。

## 三、验收结果

- `sh -n docker-entrypoint.sh` 与镜像构建通过。本地候选镜像为 `vpn-sub:fixed-runtime-identity`，镜像内 `app` 的 UID/GID 明确为 `10001:10001`。
- `python3 -u scripts/test-docker-permissions.py vpn-sub:fixed-runtime-identity` 通过。使用独立临时卷中的 Linux 目录作为 bind mount 源，覆盖 root `0755` 空目录、空的旧 `100:101` 目录、普通用户 `17001:17002` 的 `0700` 目录及默认命名卷。默认路径中的 PID 1 和新数据库属主采用 `10001:10001`，普通用户目录按原身份运行。
- 现有 Setup／重启／信号退出／删除主库后初始化、显式非 root 身份、只读挂载、主库与 WAL 访问冲突、损坏与手动应急、含空格 DATA_DIR、dev 模式回归通过。每种成功身份继续核对 PID 1 的 UID/GID 和 CapEff 为零。
- 后端 `go build ./...`、`go vet ./...`、`go test ./... -count=1`，前端 `npm run build`，Compose 示例 `config --quiet`、文档内链检查和 `git diff --check` 通过。
- 验证范围为当前源码、本地 Docker Desktop 与临时测试数据。用户确认的旧数据迁移验证已省去。2026-11-21 用户授权提交并再次触发 GitHub 构建，本轮版本采用 `v4.1.6`，远端结果以对应 GitHub Actions 记录为准。用户要求触发后即可结束本轮，远端部署尚待执行。

## 四、变更记录

| 日期 | 记录 |
|---|---|
| 2026-11-21 | 实施固定 UID/GID 与旧默认身份空目录纠正。 |
| 2026-11-21 | Step 1～2 验收通过，补齐设计、问题与部署文档。 |
