# VPN 订阅管理系统 功能构建计划（Build27：Issue14 步骤七 R28-09 项目级工程与文档收尾）

> **文档定位：** 本文档是 Issue14 步骤七、R28-09 项目级工程与文档收尾的**唯一详细构建计划**。承接已归档的 Build1～Build25 与当前活跃的 [Build26.md](Build26.md)（Issue14 步骤五/R28-07）。本文件只处理 R28-09 范围，不进入步骤五、步骤八/P4、Issue15 其他问题、`SecurityScanPlan1.md`、`SecurityReport3.md`、`Design5.md` 或任何发布动作。
>
> **创建状态（2026-09-11）：** 用户明确要求将 R28-09 已确认决策和操作内容整合为 Build27 并创建本文档。**创建行为只授权文档创建与计划冻结，不授权 Step 1～6 的代码、Dockerfile、工作流、README、LICENSE、参考文档、Issue、TODOLIST、AGENTS 或其他工作区文件修改，也不构成步骤七正式实施授权。**
>
> **当前活跃构建关系：**
> - [Build26.md](Build26.md) 仍是 Issue14 步骤五/R28-07 的当前活跃唯一构建记录；
> - Build26 的 Step 0 已完成，Step 1～20 尚未执行；
> - Build27 在 Step 1 正式实施前必须先满足：Issue14 步骤五按 Build26 真实证据关闭、Build26 归档、Git 工作区重新核对、用户逐 Step 授权；
> - Build27 与 Build26 不并行实施，不把 R28-09 加入 Build26，也不把 R28-07 加入 Build27。
>
> **关联文档：**
> - 编码指令：[AGENTS.md](AGENTS.md)（**唯一强要求**）
> - 当前设计：[Design4.md](Design4.md)（R28-09 原则上不改设计合同；仅在用户确认需要时同步）
> - 问题追踪：[Issue14.md](Issue14.md)（步骤七、R28-09、关闭条件）
> - 人工测试：[ProdTestList.md](ProdTestList.md)（R28-09 不新增人工通过结论；如新增真实浏览器/升级项目，只登记、不标通过）
> - 历史核验：[BuildReport4.md](docs/reports/BuildReport/BuildReport4.md) §6.1/§6.4/§6.6、[BuildReport5.md](docs/reports/BuildReport/BuildReport5.md) OBS-03/OBS-04
> - 构建模板：[Build.template.md](docs/DocTemplates/Build.template.md)

---

## 一、用户已确认决策（2026-09-11，ask_user_question）

1. **CA 证书：** 运行阶段**显式安装 `ca-certificates`**。在 `USER app` 之前执行 `apk add --no-cache ca-certificates`，并增加证书文件断言；默认证书场景不额外手写 `update-ca-certificates`，安装脚本/trigger 已负责更新。
2. **Dockerfile digest：** **不固定 Dockerfile 中的基础镜像 digest**。R28-09 按“评估后决定不固定，CI 记录应用镜像最终 digest，并保留人工月度/发布检查流程”的关闭口径处理；不得声称已固定 Dockerfile digest。
3. **运行阶段 Alpine：** 从 `alpine:3.21` 升级到 **`alpine:3.24`**（当前解析为 Alpine 3.24.1），与当前 Node/Golang 构建阶段的 Alpine 3.24 基线一致；不固定该 tag 的 digest。
4. **GHCR 用户引用：** README 与 Compose 模板**保持 `latest`**，只补充升级说明、可变 tag 风险说明和需要可复现时的版本 tag 使用方式；不在本轮发布新镜像。
5. **GHCR 多架构：** 本轮只记录“当前 GHCR 仅有 linux/amd64 + attestation”的事实，**不扩展多架构构建**。
6. **CI Node 版本：** `.github/workflows/docker-build.yml` 的 `setup-node` 从 Node 22 **对齐到 Node 24**；本轮不新增 `engines` 或 `.nvmrc`，除非后续另行授权。
7. **LICENSE：** 用户确认恢复历史 MIT 许可证，沿用历史版权声明：

   ```text
   MIT License

   Copyright (c) 2026 AlcaProphet
   ```

   该决策视为用户对开源意图和历史版权声明的确认；正式实施时再创建 `LICENSE` 并同步 README。
8. **Xray 外链：** 只修 `docs/Reference/Xray-Server-Config-Research.md` 中登记的外链，改为官方 `XTLS/Xray-examples` 固定 commit 链接；固定 commit 使用：

   ```text
   a64a519ab7f49f55a464a34596533e26666f75f8
   ```

9. **四个前端文件：** 逐项删除：
   - `frontend/src/views/admin/assembly/GenerateStep.vue`
   - `frontend/src/components/PreviewState.vue`
   - `frontend/src/components/ResponsiveCollection.vue`
   - `frontend/src/components/CopyField.vue`
10. **链接门禁：** 新增最小仓库内 Markdown 链接检查脚本；外部链接只做本次涉及 URL 的有边界核验，不把网络波动变成全仓强制 CI。
11. **远端 CI 证据边界：** 接受“静态校验 + 本地门禁”作为本轮证据；远端 CI 实际运行结果待后续发布/触发时补记，不写成 CI 已通过。
12. **安全审查边界：** R28-09 只做项目级整改；`SecurityScanPlan1.md` / `SecurityReport3.md` 保持独立，不引用其 Step、证据或状态。
13. **发布边界：** 本轮接受只改引用策略/文档，**不发布新镜像、不推送 GHCR、不打 Git tag、不创建 GitHub Release**。
14. **digest 更新机制：** 不固定 Dockerfile digest；由人工按发布或月度检查官方基础镜像/GHCR 状态并记录，不新增 Renovate/Dependabot。
15. **Build27 创建时机：** 用户明确要求现在创建 Build27 并整合上述操作内容；Build27 作为计划/决策载体存在，但 Step 1～6 仍须等待 Build26/步骤五关闭并逐 Step 授权。

---

## 二、范围与边界

### 2.1 纳入范围

- R28-09-1：运行阶段 `ca-certificates` 的显式安装和最终镜像验证。
- R28-09-2：基础镜像 digest 的评估、决策记录和 R28-09 关闭口径。
- R28-09-3：GHCR 引用、digest 记录、升级说明、Node CI 对齐和多架构事实记录。
- R28-09-4：LICENSE 恢复与 README 许可证描述同步。
- R28-09-5：`Xray-Server-Config-Research.md` 失效外链修复和链接门禁。
- R28-09-6：四个前端未引用文件的逐项删除和回归。
- R28-09-7：PoolTab 补跑文案的已完成证据核对，不重复实施。
- R28-09-8：AGENTS 文档清单缺报告问题的已完成证据核对，不重复实施。
- R28-09-9：受影响构建、镜像、链接、静态扫描和最终联合门禁。
- R28-09-10：每项关闭或迁移到专项并保留可追踪链接。

### 2.2 明确排除

- Issue14 步骤五/R28-07 的任何代码或测试实施；Build26 Step 1～20。
- Issue14 步骤八/P4 的全量复核与最终关闭；不得借 Build27 提前实施步骤八。
- Issue14 R28-08/N01～N07；R28-07F 的设计取向记录。
- `SecurityScanPlan1.md`、`SecurityReport3.md` 的任何 Step、证据胶囊或状态。
- `Design5.md`、Issue15 其他问题、与 R28-09 无关的依赖升级或代码清理。
- 新版本发布、Git tag、GitHub Release、GHCR 推送、CI 密钥或仓库设置变更。
- 修改 `Build26.md`、归档 Build/Design/Issue/Report 文档；修改 `docs/reference` 中本文件未纳入的其他文档。
- 把自动化、Docker 构建、Production smoke、浏览器人工或真实部署结果互相替代。

### 2.3 通用执行规则

- 每个 Step 开始前确认前一步已验收；只执行获准的单个 Step。
- 每个 Step 先做影响评估，再补失败优先检查或回归证据，最后实施最小变更。
- 每步完成后运行定向门禁，记录真实结果，等待下一 Step 授权。
- 任何决策变更、文档冲突、范围外问题或需要改变用户已确认决策的情况，立即停止并询问。
- 历史 Build 的绿色结果不能替代 Build27 修改后的重新验证。
- 人工项目未执行不得标记通过；Build27 当前没有新增人工通过结论。

---

## 三、当前事实快照（2026-09-11）

- 当前分支：`beta`；HEAD：`8df4364b890d9d638d9642ed07ce32764304331c`；`git status --short` 在本 Build27 创建前为空。
- 当前活跃构建：`Build26.md`（Issue14 步骤五/R28-07），Step 0 完成，Step 1～20 未开始。
- Dockerfile：
  - 前端构建阶段：`node:24-alpine`，本机解析 Node v24.21.0 / Alpine 3.24.1；
  - 后端构建阶段：`golang:1.26-alpine`，本机解析 Go 1.26.8 / Alpine 3.24.1；
  - 运行阶段：`alpine:3.21`，本机解析 Alpine 3.21.7；未显式安装 `ca-certificates`。
- 运行镜像需要出站 HTTPS：OIDC discovery/JWKS/token、验证码校验、素材池 URL 同步、SMTP TLS/STARTTLS；均依赖系统根证书。
- 当前 `alpine:3.21` 基础镜像自带 `ca-certificates-bundle`，证书文件为 `/etc/ssl/certs/ca-certificates.crt`，但没有 `ca-certificates` 元包，也没有 `update-ca-certificates`。
- GHCR：`latest` 与 `v3.0.2` 当前同一 index digest，当前 GHCR 只有 linux/amd64 和 attestation manifest，没有 linux/arm64；当前 `latest` 不包含当前 HEAD。
- LICENSE：仓库当前不存在 LICENSE；README 第 284～286 行仍声称“开源项目，详情见仓库 LICENSE 文件”；Git 历史曾有 MIT `LICENSE`，版权行为 `Copyright (c) 2026 AlcaProphet`。
- Xray 文档：`docs/Reference/Xray-Server-Config-Research.md` 存在 13 个指向仓库外 `../../../Xray-examples/...` 的 Markdown 链接，并有第 24、373 行的本机绝对路径说明。
- 四个前端候选文件：当前静态检查均无运行时/测试/自动注册/路由/barrel 引用；替代实现已存在。
- PoolTab：当前文案已为“服务启动时补跑今日错过”，由 Build22 Step 11 修正。
- AGENTS 文档清单：当前已包含 BuildReport1～5、SecurityReport1～2、根目录 SecurityReport3。
- 远端 CI：当前 workflow 仅定义 `setup-node` Node 22；实际工作流未在本次触发。
- 当前工作区在 Build27 创建前未发现用户未提交改动；Build27 创建后只应新增该文件，其他文件保持不变。

---

## 四、构建进度追踪

| Step | 内容 | 依据 | 状态 |
|---|---|---|---|
| 0 | 创建 Build27、冻结用户决策与范围 | 用户 2026-09-11 明确要求；Issue14 R28-09 | ✅ 本文档创建并记录决策 |
| 0.5 | 前置条件核验：Build26 关闭/归档、步骤五关闭、Git 复核 | Issue14 步骤七前置；TODOLIST P3 | ☐ 未执行；等待步骤五关闭 |
| 1 | 运行阶段 Alpine 3.24、显式安装/验证 `ca-certificates` | R28-09-1/2；用户决策 1/2/3 | ☐ 未开始 |
| 2 | GHCR `latest` 升级说明、CI digest 记录、workflow Node 24 对齐 | R28-09-3；用户决策 4/5/6/11/13/14 | ☐ 未开始 |
| 3 | 恢复 MIT LICENSE 并同步 README | R28-09-4；用户决策 7 | ☐ 未开始 |
| 4 | 修复 Xray 外链并加入最小内链检查 | R28-09-5；用户决策 8/10 | ☐ 未开始 |
| 5 | 逐文件删除四个未引用前端文件并回归 | R28-09-6；用户决策 9 | ☐ 未开始 |
| 6 | 联合构建、镜像、链接、静态扫描、Production smoke、文档收口 | R28-09-7～10；用户决策 11/12/13 | ☐ 未开始 |

状态标记：☐ 未开始 / ◧ 进行中 / ✅ 已验收 / ⛔ 阻断。

> Step 0 的“已完成”仅表示本文档已按用户要求创建并记录决策；**不代表步骤七已具备实施条件，也不代表 Step 1～6 中任何工程动作已获授权。**

---

## 五、构建概要（文件清单总览）

| Step | 主要涉及文件 | 要点 |
|---|---|---|
| 0 | `Build27.md` | 创建计划、冻结用户决策、范围与排除项 |
| 0.5 | `Issue14.md`、`Build26.md`、`docs/reports/Build/Build26.md`、Git 状态 | 只读核验步骤五关闭证据与 Build26 归档状态；不修改 Build26 |
| 1 | `Dockerfile` | `alpine:3.24`、`apk add --no-cache ca-certificates`、证书文件断言；不固定 digest |
| 2 | `.github/workflows/docker-build.yml`、`README.md`、`docker-compose.yml.example` | Node 24 对齐、digest 记录、`latest` 升级说明；不推送/不发布 |
| 3 | `LICENSE`（新增）、`README.md` | 恢复历史 MIT 文本，修正 README 许可证描述 |
| 4 | `docs/Reference/Xray-Server-Config-Research.md`、`scripts/check-md-links.mjs`（候选新增） | 13 个外链改为官方固定 commit 链接；新增最小内链检查 |
| 5 | 四个精确 Vue 文件、相关前端测试 | 逐文件删除，删除前后引用扫描，前端定向/全量/build |
| 6 | 全仓库受影响文件、`Build27.md`、`Issue14.md`、`TODOLIST.md`、`AGENTS.md`、README/Compose（按 Step 2/3） | 联合门禁、真实证据、文档收口、R28-09 关闭/迁移核验 |

---

## 六、构建顺序依赖图

```text
Step 0：Build27 创建与决策冻结（当前已完成）
   ↓
Step 0.5：等待 Build26 关闭并归档、Issue14 步骤五关闭、Git 复核
   ↓
Step 1（Dockerfile/Apline/CA）┐
Step 3（LICENSE）               ├─→ Step 6 联合门禁与文档收口
Step 4（Xray 链接）             │
Step 5（前端文件删除）          │
Step 2（GHCR/CI/README）───────┘
```

说明：

- Step 1～5 之间没有强绑定，但按 AGENTS 与本次授权规则**必须串行执行**，每次只获准一个 Step。
- Step 2 涉及 workflow，必须先获得用户对 CI 静态验证边界的再次确认；不推送/不发布。
- Step 6 依赖 Step 1～5 全部验收通过；不得提前执行步骤八/P4。

---

## 七、逐 Step 构建计划

### Step 0：创建 Build27、冻结用户决策与范围

- **目标：** 创建 Build27 文档，记录 2026-09-11 用户通过 ask_user_question 确认的全部决策、范围边界、排除项和后续 Step 依赖。
- **前置条件：** 用户明确要求“将当前所有需要操作的内容，整合为 build27，并创建”。
- **产出：** 根目录 `Build27.md`。
- **失败优先检查：** 无代码改动；检查本文件是否包含 R28-09 全部 10 项、用户决策、Step 1～6、证据边界和排除项。
- **验收标准：** 本文档创建成功；`git status --short` 只新增 `Build27.md`；其他文件未修改。
- **状态：** ✅ 本文档创建并验收（2026-09-11）。
- **范围外：** 不执行 Step 0.5/1～6；不修改 Issue14/TODOLIST/AGENTS/Dockerfile/README/工作流；不运行构建或测试。

### Step 0.5：前置条件核验

- **目标：** 确认步骤七正式实施的前置条件全部满足，避免 Build26 尚未关闭时并行实施 R28-09。
- **前置条件：** 用户明确授权对 Step 0.5 进行只读核验或正式构建收口。
- **失败优先检查（只读）：**

```bash
git status --short
git branch --show-current
git rev-parse HEAD
grep -n '步骤五关闭' Issue14.md
grep -n 'Build26' AGENTS.md TODOLIST.md Issue14.md
test -f docs/reports/Build/Build26.md && echo 'Build26 archived' || echo 'Build26 still active'
```

- **通过条件：**
  - Issue14 步骤五已依据 Build26 的真实证据关闭；
  - 步骤三、四、六仍保持关闭；
  - Build26 已按归档规则完成/归档；
  - Git 工作区状态已重新核对；
  - 没有未解决的文档或设计冲突。
- **失败处理：** 任何一条不满足立即停止，报告证据，不进入 Step 1。
- **范围外：** 不修改 Build26；不代替步骤五验收；不进入步骤八。

### Step 1：运行阶段 Alpine 3.24 与 `ca-certificates`

- **目标：** 将运行阶段基础镜像从 `alpine:3.21` 升级为 `alpine:3.24`，显式安装并验证 `ca-certificates`，保证最终运行镜像具备明确、可验证的 CA 信任链。
- **前置条件：** Step 0.5 通过；用户单独授权 Step 1；Git 工作区已重新核对。
- **影响评估：**
  - 修改 Dockerfile 运行阶段；
  - 变更基础 OS，可能影响 musl、BusyBox、APK 仓库、证书包版本和 healthcheck；
  - 不改变业务 Go 代码、数据库、HTTP 合同、前端行为；
  - 不固定 digest，仍会跟随 `alpine:3.24` tag 的补丁更新。
- **实现内容：**

```dockerfile
FROM alpine:3.24

# 显式安装 CA 证书；安装脚本/trigger 会更新证书存储
RUN apk add --no-cache ca-certificates \
 && test -s /etc/ssl/certs/ca-certificates.crt
```

  放置位置：`FROM alpine:3.24` 之后、`addgroup`/`adduser`/`USER app` 之前。不手动调用 `update-ca-certificates`，不额外添加自定义 CA。
- **失败优先/回归：**
  - 修改前：`grep -q 'apk add --no-cache ca-certificates' Dockerfile` 应失败；
  - 修改前：Dockerfile 第 20 行仍为 `alpine:3.21`；
  - 修改后：`grep -n 'FROM alpine:3.24' Dockerfile` 与证书安装断言应通过。
- **定向验收命令：**

```bash
docker build -t vpn-sub:r28-09-step1 .
docker run --rm --entrypoint sh vpn-sub:r28-09-step1 -c \
  'apk info -e ca-certificates && test -s /etc/ssl/certs/ca-certificates.crt'
docker run --rm --entrypoint sh vpn-sub:r28-09-step1 -c \
  'test "$(id -u)" -ne 0'
docker run --rm --network bridge --entrypoint sh vpn-sub:r28-09-step1 -c \
  'wget -q -T 10 --spider https://example.com'
docker run --rm --entrypoint sh vpn-sub:r28-09-step1 -c \
  'for c in go node npm gcc cc make; do command -v "$c" && exit 1; done; true'
docker image inspect vpn-sub:r28-09-step1 --format '{{.Os}}/{{.Architecture}}'
git diff --check
```

- **验收标准：**
  - 运行阶段为 `alpine:3.24`；
  - `apk info -e ca-certificates` 通过；
  - `/etc/ssl/certs/ca-certificates.crt` 非空；
  - 容器 HTTPS 探测通过；
  - 非 root 运行、无 Go/Node/npm/gcc 等构建工具；
  - 该步骤的真实 Docker 构建结果记录在 Build27；
  - 基础镜像只读探针不得替代最终镜像验证。
- **范围外：** 不固定 digest；不改 Node/Golang 构建阶段；不改业务代码；不发布镜像。

### Step 2：GHCR `latest` 升级说明、CI digest 记录、workflow Node 24 对齐

- **目标：** 保持用户部署引用为 `latest`，补充可变 tag 风险/升级说明；让 CI 记录应用镜像最终 digest；将 GitHub Actions 的 Node 版本对齐到 24。
- **前置条件：** Step 1 通过；用户单独授权 Step 2；明确本轮不推送/不发布。
- **影响评估：**
  - README 与 `docker-compose.yml.example` 增加升级/版本 tag 说明；
  - `.github/workflows/docker-build.yml` 增加 digest 输出并将 `setup-node` 对齐 Node 24；
  - 不改变 GHCR 现有 tag 生成、不推送、不发布、不触发远端 CI。
- **实现候选：**

  1. README/Compose 保持 `image: ghcr.io/alcaprophet/vpnmanagement:latest`，补充：
     - `latest` 可能随默认分支更新；
     - 生产升级前建议阅读变更记录；
     - 需要可复现时可改用发布版本 tag，例如 `:v3.0.2`；当前版本 tag 仍可能被覆盖，如需更严格验证可记录并使用 digest；
     - 当前 GHCR 仅有 linux/amd64。
  2. workflow：
     - `node-version: 22` → `node-version: 24`；
     - 给 Build and push 步骤加 `id: build`；
     - 增加 digest summary 步骤，例如向 `$GITHUB_STEP_SUMMARY` 写入 `${{ steps.build.outputs.digest }}`；
     - 不新增 `platforms:` 多架构矩阵。
- **失败优先/回归：**

```bash
# 当前 workflow 仍为 Node 22
grep -n 'node-version:' .github/workflows/docker-build.yml
# 当前无 digest 输出
grep -n 'outputs.digest' .github/workflows/docker-build.yml && exit 1 || true
# 当前 README/Compose 仍使用 latest
git grep -n ':latest' -- README.md docker-compose.yml.example
```

- **定向验收命令：**

```bash
docker compose -f docker-compose.yml.example config --quiet
git grep -n 'ghcr.io/alcaprophet/vpnmanagement' -- README.md docker-compose.yml.example
grep -n 'node-version: 24' .github/workflows/docker-build.yml
grep -n 'outputs.digest' .github/workflows/docker-build.yml
git diff --check
```

- **验收标准：**
  - 用户引用仍为 `latest`，并已有明确的升级/可复现说明；
  - workflow 的 Node 版本已对齐 24；
  - workflow 已包含应用镜像最终 digest 记录逻辑；
  - 本地 Compose 配置校验通过；
  - 明确记录“远端 CI 未执行/未推送，实际 workflow 结果待后续补记”，不得写成 CI 已通过；
  - 不修改 GHCR 上的任何 tag/digest，不推送。
- **范围外：** 不发布新版本；不加多架构；不新增 APK/Go 依赖；不把 Actions 供应链审查混入。

### Step 3：恢复 MIT LICENSE 并同步 README

- **目标：** 恢复历史 MIT 许可证，修正 README 的许可证描述，使“开源项目/License 文件”表述与仓库事实一致。
- **前置条件：** Step 2 通过；用户确认历史 MIT 版权声明 `Copyright (c) 2026 AlcaProphet`；用户单独授权 Step 3。
- **实现内容：**
  - 新增 `LICENSE`，内容使用历史 Git 文件 `36ce532:LICENSE` 的 MIT 文本与 `Copyright (c) 2026 AlcaProphet`；
  - README 第 284～286 行同步为明确许可证描述，例如：
    - `本项目基于 MIT License 开源，详情见仓库 LICENSE 文件。`
  - 不改 git 历史、不改 GitHub 仓库设置；不代替用户做其他许可证选择。
- **失败优先/回归：**

```bash
test -e LICENSE && exit 1        # 修改前无 LICENSE
grep -n '详情见仓库 LICENSE' README.md
```

- **定向验收命令：**

```bash
test -s LICENSE
grep -n 'MIT License' LICENSE
grep -n 'Copyright (c) 2026 AlcaProphet' LICENSE
grep -n 'MIT' README.md
git diff --check
```

- **验收标准：**
  - `LICENSE` 存在且为确认的 MIT 文本；
  - README 不再声称 LICENSE 缺失或描述矛盾；
  - `git status --short` 只含本 Step 预期文件；
  - 不修改依赖许可证，不修改仓库元数据。
- **范围外：** 不选其他许可证；不追写历史归档报告；不推动 GitHub license 自动识别之外的设置变更。

### Step 4：修复 Xray 外链并加入最小内链检查

- **目标：** 修复 `docs/Reference/Xray-Server-Config-Research.md` 中 13 个仓库外失效链接，改为官方 `XTLS/Xray-examples` 固定 commit 链接；新增最小仓库内 Markdown 链接检查脚本，并对外部链接做有边界核验。
- **前置条件：** Step 3 通过；用户单独授权 Step 4；官方 Xray-examples 固定 commit `a64a519ab7f49f55a464a34596533e26666f75f8` 已核验仍可访问。
- **实现内容：**
  - 保留第 24、373 行的来源说明，但改成与固定 commit 一致的可复核外部来源说明；
  - 替换以下位置的外链：
    - 第 92 行：VLESS 4 个链接；
    - 第 106 行：VMess 3 个链接；
    - 第 118 行：Trojan 2 个链接；
    - 第 131 行：Shadowsocks 2 个链接；
    - 第 151 行：Hysteria2 1 个链接；
    - 第 164 行：SOCKS5 1 个链接；
  - 官方固定链接形式：

```text
https://github.com/XTLS/Xray-examples/blob/a64a519ab7f49f55a464a34596533e26666f75f8/<百分号编码路径>
```

  - 新增最小内链检查脚本（建议 `scripts/check-md-links.mjs`）：
    - 支持解析 Markdown 链接；
    - 只检查仓库内相对路径；
    - 跳过外部 URL、`#` 锚点、`mailto:`、localhost、示例占位符和代码块；
    - 对 `%20`、全角字符等百分号编码做解码后再解析；
    - 仅对当前活跃/指定文档运行，默认不把全仓归档占位符纳入强制检查。
- **失败优先/回归：**

```bash
# 修改前：13 个链接解析到仓库外 ../Xray-examples
git grep -n '../../../Xray-examples/' -- docs/Reference/Xray-Server-Config-Research.md
# 修改前：最小内链脚本尚不存在
test ! -e scripts/check-md-links.mjs && echo 'link checker absent'
```

- **定向验收命令：**

```bash
node scripts/check-md-links.mjs docs/Reference/Xray-Server-Config-Research.md
for url in <13 个官方固定 commit URL>; do
  curl -sS -L -o /dev/null -w '%{http_code} %{url_effective}\n' --max-time 15 "$url"
done
git grep -n 'Xray-examples' -- docs/Reference/Xray-Server-Config-Research.md
git diff --check
```

- **验收标准：**
  - 文档不再存在 `../../../Xray-examples/` 仓库外相对链接；
  - 13 个官方固定 commit URL 有本次核验记录，HTTP 状态为 2xx/3xx；
  - 仓库内链接检查通过；
  - 外部链接核验记录查询时间、URL、状态码和重定向目标；
  - 网络波动不导致全仓强制失败；如需 CI 化只纳入内链检查。
- **范围外：** 不扩展修改其他 Reference 文档；不复制 Xray-examples 文件进仓库；不改外部仓库。

### Step 5：逐文件删除四个未引用前端文件并回归

- **目标：** 逐文件、可审计地删除四个确认未引用的前端文件，并证明删除不影响前端测试、类型检查和生产构建。
- **前置条件：** Step 4 通过；用户确认四个都删除；用户单独授权 Step 5。
- **实现内容：** 只删除以下精确路径，不使用 glob、不批量清理：

```text
frontend/src/views/admin/assembly/GenerateStep.vue
frontend/src/components/PreviewState.vue
frontend/src/components/ResponsiveCollection.vue
frontend/src/components/CopyField.vue
```

- **删除前引用证据：**

```bash
for f in \
  frontend/src/views/admin/assembly/GenerateStep.vue \
  frontend/src/components/PreviewState.vue \
  frontend/src/components/ResponsiveCollection.vue \
  frontend/src/components/CopyField.vue; do
  test -f "$f"
done

git grep -n -E 'GenerateStep|PreviewState|ResponsiveCollection|CopyField' -- frontend/src frontend/tests
# 接受的结果：仅候选文件自身注释命中；不得出现 import、模板、路由、barrel、动态注册
```

- **删除后回归：**

```bash
for f in \
  frontend/src/views/admin/assembly/GenerateStep.vue \
  frontend/src/components/PreviewState.vue \
  frontend/src/components/ResponsiveCollection.vue \
  frontend/src/components/CopyField.vue; do
  test ! -e "$f"
done

! git grep -n -E 'GenerateStep|PreviewState|ResponsiveCollection|CopyField' -- frontend/src frontend/tests

cd frontend
npm test -- --run tests/assembly-view.spec.ts tests/preview-step.spec.ts tests/home-view.spec.ts tests/pool-tab.spec.ts
npm test -- --run
npm run build
cd ..

git diff --check
```

- **验收标准：**
  - 每个文件删除前有引用扫描结果，删除后有存在性/零引用证据；
  - 前端定向测试、全量测试、生产构建通过；
  - 当前 Build/Issue 记录说明归档 Design2-UI/Build11 等历史引用不回写为“原报告错误”；
  - 不修改 `frontend/dist`、`node_modules`、构建产物。
- **范围外：** 不清理其他前端文件；不修改业务组件行为；不新增人工浏览器通过结论。

### Step 6：联合构建、镜像、链接、静态扫描和文档收口

- **目标：** 对 Step 1～5 的全部真实修改重新执行受影响门禁；同步 Build27 与受影响文档；仅在所有 R28-09 项目关闭或迁移后评估步骤七关闭。
- **前置条件：** Step 1～5 全部验收通过；用户单独授权 Step 6。
- **失败优先/最终验收命令：**

```bash
cd /Users/kyle/Desktop/Repo/VPN-Subscription-Management

# 工作区与差异
git status --short
git diff --check

# Compose/Docker 构建
docker compose config --quiet
docker compose build

# 最终运行镜像：CA、非 root、HTTPS、无构建工具
IMG=vpn-sub:r28-09-final
docker build -t "$IMG" .
docker run --rm --entrypoint sh "$IMG" -c \
  'apk info -e ca-certificates && test -s /etc/ssl/certs/ca-certificates.crt'
docker run --rm --entrypoint sh "$IMG" -c 'test "$(id -u)" -ne 0'
docker run --rm --network bridge --entrypoint sh "$IMG" -c \
  'wget -q -T 10 --spider https://example.com'
docker run --rm --entrypoint sh "$IMG" -c \
  'for c in go node npm gcc cc make; do command -v "$c" && exit 1; done; true'
docker image inspect "$IMG" --format '{{.Os}}/{{.Architecture}}'

# 前端
cd frontend
npm test -- --run
npm run build
cd ..

# 未引用文件静态扫描（删除后应为 0）
! git grep -n -E 'GenerateStep|PreviewState|ResponsiveCollection|CopyField' -- frontend/src frontend/tests

# 仓库内 Markdown 链接检查
node scripts/check-md-links.mjs README.md docs/Reference/*.md

# 本次涉及外链的有边界核验（记录时间与状态码）
for url in <本次 Xray 固定链接>; do
  curl -sS -L -o /dev/null -w '%{http_code} %{url_effective}\n' --max-time 15 "$url"
done

# 正式 Production smoke（隔离环境；单独记录）
bash .smoke-test-prod.sh
```

- **文档同步：**
  - `Build27.md`：Step 状态、真实命令、结果、证据类型、未执行项；
  - `Issue14.md`：R28-09 逐项关闭/迁移状态、步骤七关闭条件、变更记录；
  - `TODOLIST.md`：P3 勾选、活跃快照；
  - `AGENTS.md`：Build27 当前/归档状态（Build26 归档后再切换）；
  - `README.md`、`docs/Reference/Xray-Server-Config-Research.md`：按 Step 2/3/4 实际修改同步；
  - 不修改 Build26、归档报告、SecurityScanPlan1/Report3、Design5、Issue15 其他问题。
- **验收标准：**
  - Step 1～5 的真实修改均有重新验证结果；
  - R28-09 每项关闭或迁移到专项并保留链接；
  - 自动化、Docker、Production smoke、浏览器人工、真实部署证据分开记录；
  - 远端 CI 未触发时明确记录“未执行”，不写成通过；
  - 步骤七关闭后仍不自动进入 P4/步骤八。

---

## 八、失败优先与证据层级规则

- **修复前必须失败：** 每个 Step 新增的行为、门禁、删除前检查或文档断言，必须证明修改前处于预期旧状态。
- **自动化证据：** 定向测试、全量测试、build、vet、脚本检查、本地 Compose 校验。
- **Docker/镜像证据：** `docker build`、最终镜像内证书、非 root、HTTPS、构建工具残留、平台/manifest 检查；基础镜像只读探针与最终镜像证据分开。
- **Production smoke：** 使用正式 `.smoke-test-prod.sh`，隔离环境执行；单独记录，不替代浏览器人工。
- **浏览器人工/真实部署：** 未执行时保持未执行；如新增项目只登记 `ProdTestList.md`，不标通过。
- **远端 CI：** 不推送、不触发时只能做静态校验；真实 workflow 结果待发布时补记。
- **历史结果：** Build21～Build26 或 BuildReport4/5 的绿色结果不能替代 Build27 修改后的重新验证。

---

## 九、风险、回滚与停止条件

- **Alpine 3.24 升级：** 运行 OS 变化风险中；如 Docker 构建、证书、healthcheck、非 root 或 Production smoke 失败，回退 Step 1 的 Dockerfile 运行阶段到 `alpine:3.21` 与该 Step 文件级改动。
- **CA 安装：** 低风险；若 APK 安装/证书断言失败，回退本 Step 的证书安装 layer，同时保留失败证据，不标记为通过。
- **GHCR/CI：** workflow 静态改动无法在未推送情况下真实运行；不得以静态通过替代 CI 结果。
- **LICENSE：** 法律/授权变更；如有版权或贡献者问题，立即停止并回到用户决策。
- **Xray 外链：** 外部链接可能变化；如固定 commit URL 不可达或上游策略变化，停止并询问，不替换为主题相似但证据不同的页面。
- **前端删除：** 如删除后发现间接引用、测试失败或构建失败，逐文件回退并重新评估。
- **停止条件：** 发现 Build26/步骤五未关闭、Git 工作区有未预期改动、用户决策冲突、范围外问题、需要发布/推送、需要修改 SecurityScanPlan1 或需要提前进入步骤八时，立即停止并询问。

---

## 十、文档同步与关闭条件

- **Step 0：** 仅创建 `Build27.md`。
- **Step 0.5：** 只读核验 Issue14/Build26/AGENTS/TODOLIST 的步骤五关闭证据；不修改 Build26。
- **Step 1～5：** 仅同步受实际修改影响的文件与本 Build27 记录。
- **Step 6：** 统一回写 Issue14/TODOLIST/AGENTS；Build27 完成后按归档规则移入 `docs/reports/Build/Build27.md`。
- **R28-09 关闭条件：**
  - `ca-certificates`：显式安装并在最终镜像中验证；
  - digest：按用户决策记录“不固定 Dockerfile digest，CI 记录应用镜像 digest，人工月度/发布检查”；
  - GHCR：`latest` 升级说明和 digest 记录策略落实；
  - LICENSE：MIT 文件与 README 一致；
  - Xray：13 个链接改为固定官方 commit，内链门禁通过；
  - 前端四个文件：逐项删除并回归通过；
  - PoolTab/AGENTS 历史项：保留证据，不重复实施；
  - 联合门禁：Docker/Compose、镜像证书、HTTPS、平台、前端测试/build、静态扫描、链接核验、Production smoke、`git diff --check` 全部真实记录。
- **步骤七关闭不等于 Issue14 关闭：** P4/步骤八仍需另行授权执行全量复核与最终关闭。

---

## 十一、变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-11 | 用户要求整合 R28-09 决策与操作内容并创建 Build27；记录 CA、digest、Alpine、GHCR、CI Node、LICENSE、Xray、前端清理、链接门禁、安全审查边界和发布边界；Build27 仅作为计划/决策载体，Step 1～6 未开始，等待步骤五关闭与逐 Step 授权。 |
