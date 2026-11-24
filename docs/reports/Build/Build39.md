# Build39 — Docker 数据目录权限与启动身份

> 基线 `beta` / `ce6d7eb`。设计见 [Design6](../Design/Design6.md)，问题见 [Issue21 R35-08](../Issue/Issue21.md#r35-08-docker-挂载目录不可写导致误入应急模式)，约束见 [AGENTS](../../../AGENTS.md)。

## 一、构建进度

| Step | 内容 | 状态 |
|---|---|---|
| 1 | 镜像入口与后端存储错误分类 | 已验收 |
| 2 | Docker 场景验收、联合门禁和文档同步 | 已验收 |

## 二、分步构建

### Step 1 — 数据目录准备与降权

- 目标为在挂载完成后选择运行身份、验证目录和 SQLite 文件访问权限，并区分存储访问失败与损坏救援。
- 前置为用户确认入口准备阶段采用 root、业务服务采用非 root。
- 产出包括 `Dockerfile`、`docker-entrypoint.sh`、`backend/internal/store/access_error.go` 及回归测试、`backend/cmd/server/main.go`。
- 流程为读取实际目录属主 → root 目录调整目录自身 → `su-exec UID:GID` → 以业务身份验证目录和数据库文件 → `exec /server`。显式指定非 root 用户直接进入验证。Open 与迁移错误根据 SQLite 主错误码分支到存储失败退出或损坏救援。
- 验收命令为 `sh -n docker-entrypoint.sh`、`cd backend && go test ./internal/store -run TestIsAccessError -count=1`、`docker build -t vpn-sub:bind-permissions-fix .`。
- 验收规范为目录权限与 SQLite CANTOPEN／READONLY 可识别，实际损坏保留救援，镜像可构建。

### Step 2 — 容器与联合验收

- 前置为 Step 1 验收通过。
- 产出为 Docker 场景回归脚本及 Design6、Issue21、README、Compose 示例、AGENTS 的对应记录。
- 验收规范覆盖 root 空目录、普通用户目录、命名卷、Setup 后重启、删除数据库后初始化、显式非 root 用户、只读挂载、已有 SQLite 权限冲突、实际损坏与手动应急。核对 PID 1 的实际 UID/GID 和退出信号。
- 验收命令为 Docker 场景脚本、后端 `go build ./...`、`go vet ./...`、`go test ./... -count=1`、前端 `npm run build` 和 `git diff --check`。

## 三、验收结果

- `sh -n docker-entrypoint.sh`、`go test ./internal/store -run TestIsAccessError -count=1` 和 Docker 镜像构建通过。错误分类覆盖原生权限、只读文件系统、空间不足、配额、I/O、真实 SQLite CANTOPEN、只读迁移及实际损坏文件。
- `python3 -u scripts/test-docker-permissions.py vpn-sub:bind-permissions-fix` 通过。测试在独立临时 Docker 卷中创建 Linux 目录，再以 bind mount 挂载到候选容器，避免 Docker Desktop 宿主机文件共享造成的权限映射差异。全部临时容器和卷在测试结束后清理。
- root 持有的 `0755` 空目录可以自动初始化，PID 1 为 UID/GID `100:101` 的 `/server`。Setup 后重启保持配置，SIGTERM 退出码为 0，停止后仅删除主数据库再次启动进入 Setup。root 目录中已有 app 持有的旧数据库也可以启动并保留配置。
- UID/GID `17001:17002` 的 `0700` 目录自动采用其身份，原目录权限和属主保持原值。显式 `--user 17001:17002` 可启动。每种成功身份均核对 `/proc/1/status` 的真实、有效、保存和文件系统 UID/GID，`CapEff` 为零，`/proc/1/exe` 指向 `/server`。
- 显式用户无法写目录、只读挂载、SQLite 主文件及 WAL 权限冲突均输出明确失败并以 1 退出。旧数据库文件的属主和模式保留。绕过入口运行后端时，Open 和迁移的存储访问失败也按存储错误退出。
- 实际损坏文件继续返回 `emergency_reason=db_corrupt`，手动变量返回 `manual`，两种状态的 `/health` 均为 503。正常状态 `/health` 为 200。自定义含空格 DATA_DIR、dev 模式独立数据库和默认命名卷均通过。
- 后端 `go build ./...`、`go vet ./...`、`go test ./... -count=1`，前端 `npm run build`，Compose 示例 `config --quiet`，文档内链检查和 `git diff --check` 均通过。前端构建仍提示既有大 chunk 警告。
- 本地验收镜像标签为 `vpn-sub:bind-permissions-fix`。验证范围为本机 Docker Desktop 的常规 Linux 权限模型。KCS Aliyun 新镜像部署、Rootless 和 userns-remap 环境尚待验证。用户于 2026-11-21 授权提交、创建版本 tag 并推送 GitHub 构建，本轮版本采用 `v4.1.5`，远端结果以对应 GitHub Actions 记录为准。

## 四、变更记录

| 日期 | 记录 |
|---|---|
| 2026-11-21 | 实施启动入口及存储错误分类，定向测试通过。 |
| 2026-11-21 | Step 1～2 本地验收通过，完成 Docker 场景回归、联合门禁、Design6 和问题记录同步。 |
