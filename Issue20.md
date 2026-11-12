# Issue20.md — 前端依赖发布审计与 Tailwind 4 视觉回归

> **文档定位：** 记录前端依赖发布审计、R34-01 的 Tailwind 4 视觉回归及验证结果。Build32 和第三期安全审查由各自文档跟踪。

---

## 一、问题处理记录

### R34-01 `v4.1.2` 的 `npm audit (high gate)` 阻断镜像构建

- **现象：** [GitHub Actions 运行 38020693218](https://github.com/AlcaProphet/VPN-Subscription-Management/actions/runs/38020693218) 在 2026-10-10 的 `npm ci` 成功后，于 `npm audit --audit-level=high` 以退出码 1 失败；当时报告 10 high、3 moderate。Docker Buildx、GHCR 登录、镜像构建和推送均未执行。
- **根因：** 标签与当前工作区锁文件固定了存在已知公告的 Vue 3.5.41、Axios 1.19.0、markdown-it 15.0.0 等版本。Tailwind CSS 3.4.19 还经 `chokidar`／`micromatch` 引入尚无修复发布版的 `braces`，并经旧 `postcss-selector-parser` 引入中危项。2026-10-22 在原锁文件上重新运行完整审计时，漏洞库结果已变为 13 high、4 moderate；两次数字是不同时间点的报告，不代表本次代码新增漏洞。
- **影响范围：** `frontend/package.json`、锁文件、Tailwind／Vite 接入、共用样式与少量已更名工具类；前端全部页面消费构建后的 CSS。最终镜像只含静态 Go 二进制和嵌入式前端产物，不携带构建阶段的 `node_modules`。不改后端、API、数据库、协议逻辑或工作流的审计阈值。
- **用户决策：** 2026-10-22 选择方案 A：保留对全部依赖的 high 审计门禁，升级 Tailwind 4；当前项目没有旧浏览器兼容需求。
- **修复方案与实施：** 更新 Vue 3.5.44、Axios 1.20.0、markdown-it 15.0.3、Vite 7.3.7、Vitest 4.1.12；间接依赖更新至 `brace-expansion` 2.1.7、`follow-redirects` 1.16.1。升级 Tailwind CSS 4.3.3 并采用 `@tailwindcss/vite`，移除旧 PostCSS 插件及 JS 配置；用 CSS `@theme inline` 保留现有主题 Token、`.dark` 类切换和原默认边框色；更名已使用的 `shadow-sm` 与 `flex-shrink-0` 工具类。锁文件中已无 `braces` 或 `postcss-selector-parser`。
- **本地验证：** `npm ci` 成功且报告 0 vulnerabilities；`npm audit --audit-level=high` 为 0；`npm test -- --run` 为 48 文件／346 项通过；`npm run build` 通过，另用 Node 24.22.1 执行同一构建通过；`git diff --check` 通过。产物包含 `bg-page`、`text-text-tertiary`、`border-border-subtle`、`shadow-xs`、`shrink-0` 和暗色变体。用本地静态预览检查登录页明／暗主题可见；未启动后端，预览中的 API 代理拒绝连接属于预期，不能据此声明 API 或其他页面验收。
- **远端发布：** `v4.1.3`（`45cb62a`）通过当时的审计、构建和推送。2026-11-12 复核 OCI 索引摘要为 `sha256:3862b98b1b0db8f10dcc5c8c63d61270528aa04191be30b8c52689b13bc11a61`，应用镜像平台为 `linux/amd64`。[GitHub Actions #39727958557](https://github.com/AlcaProphet/VPN-Subscription-Management/actions/runs/39727958557)
- **视觉验收范围（2026-11-12 已确认）：** 已发布的 `v4.1.3` 镜像与审计修复后的候选分别覆盖基础模式 29 条路由。每条路由检查 `1280×800`、`920×785`、`375×785` 的浅色和深色，共 348 个基础组合。覆盖公开入口 9 条、普通用户 3 条、管理页面 13 条和四类版本详情。组管理与 Xray 两条高级专属路由在本轮范围之外。
- **验收环境：** 2026-11-12 使用 Chrome 157.0.8103.39、Playwright 1.65.0。两套镜像均以回环 Production 实例、独立临时卷和合成账号运行。Setup 用未配置实例，应急用独立实例。OIDC 失败与联系管理员、密码重置有效与过期采用浏览器内的本地响应夹具。

| 产物 | 构建标识 | 镜像与平台 | 前端入口 SHA-256 | 主 CSS SHA-256 |
|---|---|---|---|---|
| 已发布 `v4.1.3` | `45cb62ae576dae49588535ad6cd947e25a1eab78` | `sha256:3862b98b1b0db8f10dcc5c8c63d61270528aa04191be30b8c52689b13bc11a61` · `linux/amd64` | `81f90716eb8be95fa8f09c20fe9ca4d074a70e5a6d4372657668ff1884c487af` | `f2ce3790a3eadd590cb22a3ba066d5f8bb77cdd0a171a14c512a05ed635465ee` |
| 本地最终候选 | `baa73070f1046ba127e4eb35d12e57bd31bf699d` 加 R34-02～04，`git diff --binary -- frontend` 的 SHA-256 为 `d3a6fd718a4785e61136f6fef860bf73e15525820ba8dab24c48c949d4424caa` | `sha256:b69f28b8f11e5e38d93792e98d71059e4a30a0f3d59846860ab9c98349ccb83e` · `linux/arm64` | `330ac0fb45326003e263c6397faebd1e96f012f8a572b90c3f1a6272eba85edd` | `3b4626a471f35d7de8e6764bb1566c3ab34697a0ac8c60ea2e7a9c83213c516b` |

| 页面覆盖 | 路由 |
|---|---|
| 公开入口（9） | `/setup`、`/emergency`、`/login`、`/register`、`/forgot`、`/reset`、`/pending`、`/login/callback`、404 |
| 普通用户（3） | `/`、`/rules`、`/profile` |
| 管理端（13） | `/admin`、订阅、分享、平台列表、新建平台、编辑平台、规则、装配、节点、用户、审批、设置、日志 |
| 版本详情（4） | 订阅、分享、规则、自定义订阅 |

- **基础矩阵结果：** 两套均覆盖 29 路由 × 三视口 × 明暗主题。最终候选 **174/174 通过**。发布镜像 **170/174 通过**，920px 浅色与深色的节点页宽 955px（[R33-08](Issue19.md)），375px 浅色与深色的用户管理操作按钮裁切（R34-04）。发布镜像用真实手动应急实例覆盖应急页的视觉组合，真实损坏库另有六个组合无法进入应急页（R34-03）。最终候选的应急页六个组合均使用真实损坏库。
- **补充状态：** 两套各 **48/48 通过**，在 1280px 浅色和 375px 深色覆盖 Setup 确认、节点创建／编辑和浮层 Tab 焦点、装配的代理组及四类目标、Shadowrocket 规则真实预览和与激活版本差异、邮件／实时日志、密码管理、设置其余五组、四类版本内容预览、OIDC 联系提示与重置过期。Shadowrocket 订阅和通用 V2Ray 采用前置条件提示状态。
- **本地证据：** `.ui-check/r34-20261112/` 下保存 `coverage-summary.json`、每套产物的 `environment.json`（含 11 个实际加载 CSS 的 SHA-256）、逐视口 `coverage.json`、`states.json`、`states-supplement.json` 和截图。另有 `audit-high.json`、损坏库前后记录和窄屏按钮测量。

### R34-02 `follow-redirects` 新公告阻断候选发布

- **复现：** 2026-11-12，`beta` 基线 `baa7307` 的锁文件与 `v4.1.3` 相同。完整 `npm audit --audit-level=high` 退出 1，报告 1 项 high 和 4 项 moderate。high 来源为 `axios@1.20.0 → follow-redirects@1.16.1`，公告最低修复版本为 `1.16.2`。[GHSA-mrwp-f3q8-3g9p](https://github.com/advisories/GHSA-mrwp-f3q8-3g9p)
- **用户决策：** 定向更新锁文件中 Axios 的间接依赖 `follow-redirects`，沿用全依赖 high 门禁。4 项 moderate 来自开发依赖 `@vue/test-utils → js-beautify → glob／editorconfig → minimatch`。继续跟踪中危，待上游兼容版本发布后重查。[glob 公告](https://github.com/advisories/GHSA-3g3m-2fjr-6pw4) · [minimatch 公告](https://github.com/advisories/GHSA-g3hv-75wg-m249)
- **研究证据：** 临时锁文件定向解析到 `follow-redirects@1.16.3`，high 门禁通过，报告仍列出 4 项 moderate。单独更新 `@vue/test-utils` 到 2.5.1 后两条中危公告仍在。跨主版本局部覆盖可使临时副本审计归零，用户选择等待上游兼容版本，正式方案仅更新 `follow-redirects`。
- **本地实施：** 锁文件中的 `follow-redirects` 已更新至 1.16.3，差异仅含该条目的版本、下载 URL 和完整性摘要。锁文件 SHA-256 为 `2356f71aa6f7a7b35cac7d5a215493607edaea155f1026bcf52ec20ece620358`。
- **本地验收：** Node 24.22.3／npm 11.19.1 下干净安装通过，`npm ls axios follow-redirects` 显示 Axios 1.20.0 使用 1.16.3，`npm audit --audit-level=high --json` 退出 0（0 critical、0 high、4 moderate）。最终前端 51 文件／460 项与生产构建通过，Go 1.26.6 下后端 build、vet、全量 test 通过。仓库 Dockerfile 构建 `linux/arm64` 最终候选镜像通过。
- **状态：** high 阻塞已在本地修复并验收。4 项 moderate 等待上游兼容版本。下次发布以实际标签提交的远端审计、构建、推送结果与 GHCR 摘要补充发布证据。

### R34-03 数据库损坏时应急页与 Setup 之间循环跳转

- **现象：** R34-01 双产物复核在独立损坏库上复现。实际 `/api/system/status` 返回 `emergency=true`、`configured=false` 和 `emergency_reason=db_corrupt`；`v4.1.3` 与仅更新锁文件的候选均无法显示应急恢复表单。
- **根因：** 前端守卫先把其他路径重定向到 `/emergency`。抵达 `/emergency` 后，守卫继续执行未配置分支，返回 `/setup`，随后再次进入应急重定向。
- **修复范围：** 在前端守卫的应急分支内完成决策，应急状态下直接放行 `/emergency`，其他路径重定向到该页。通过真实路由测试覆盖数据库损坏状态的直接访问和重定向，以同类损坏库复测最终候选。发布镜像的复现结果保留在双版本证据中。
- **验收：** 四项真实 Vue Router 定向用例在旧守卫下均失败，修复后全部通过。最终候选在 375px 和 920px 分别访问 `/emergency`、`/setup`、`/login`、`/admin/settings`，八组均停留在 `/emergency` 并显示操作码表单。最终候选本地已关闭，发布状态见下次标签记录。

### R34-04 窄屏用户管理页头的操作按钮被裁切

- **现象：** `v4.1.3` 与 R34-03 修复候选的窄屏截图均显示「新建用户」按钮超出页头。候选在 375px 时按钮右沿到达 376px，而页头内容区右沿为 351px。320px 时按钮仍到达 376px，页面根节点宽度受裁切样式限制，无法用根节点滚动宽度发现。
- **根因与修复范围：** 共用 `PageHeader` 的操作区按固有宽度排列，用户页内部 `Space` 虽启用换行但未获得页头可用宽度约束。给共用操作区补齐最大宽度约束，保留用户页的既有按钮顺序和换行设置，复核共用页头的 29 路由矩阵及 320／375px 按钮几何边界。
- **验收：** 最终候选在 320px 和 375px 下，两个按钮换行且右沿分别为 280px 和 112px，均位于页头内容区内。另核对 767／768px、920px 和 1280px。375px 明暗主题截图与完整矩阵通过。最终候选本地已关闭，发布状态见下次标签记录。

---

## 二、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.3 | 2026-11-12 | 完成双产物 29 路由视觉矩阵，记录构建摘要、发布镜像的已知失败及 R34-03／04 最终候选复验。 |
| v1.2 | 2026-11-12 | 记录 R34-02 定向锁文件修复与门禁，登记双产物复核发现的 R34-03。 |
| v1.1 | 2026-11-12 | 记录 v4.1.3 发布证据、双产物基础模式视觉验收范围、R34-02 审计复现和用户决策。 |
| v1.0 | 2026-10-22 | 登记 R34-01 的失败根因、方案 A 的本地修复、验证结果与远端验收边界。 |
