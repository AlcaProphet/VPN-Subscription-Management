# Issue20.md — GitHub Actions 前端依赖审计失败

> **文档定位：** 独立记录 `v4.1.2` 镜像发布工作流的依赖审计失败、修复与验证边界。Build32 Step 21 和第三期安全审查仍由各自文档跟踪；本文件不替代其验收。

---

## 一、问题处理记录

### R34-01 `v4.1.2` 的 `npm audit (high gate)` 阻断镜像构建

- **现象：** [GitHub Actions 运行 38020693218](https://github.com/AlcaProphet/VPN-Subscription-Management/actions/runs/38020693218) 在 2026-10-10 的 `npm ci` 成功后，于 `npm audit --audit-level=high` 以退出码 1 失败；当时报告 10 high、3 moderate。Docker Buildx、GHCR 登录、镜像构建和推送均未执行。
- **根因：** 标签与当前工作区锁文件固定了存在已知公告的 Vue 3.5.41、Axios 1.19.0、markdown-it 15.0.0 等版本。Tailwind CSS 3.4.19 还经 `chokidar`／`micromatch` 引入尚无修复发布版的 `braces`，并经旧 `postcss-selector-parser` 引入中危项。2026-10-22 在原锁文件上重新运行完整审计时，漏洞库结果已变为 13 high、4 moderate；两次数字是不同时间点的报告，不代表本次代码新增漏洞。
- **影响范围：** `frontend/package.json`、锁文件、Tailwind／Vite 接入、共用样式与少量已更名工具类；前端全部页面消费构建后的 CSS。最终镜像只含静态 Go 二进制和嵌入式前端产物，不携带构建阶段的 `node_modules`。不改后端、API、数据库、协议逻辑或工作流的审计阈值。
- **用户决策：** 2026-10-22 选择方案 A：保留对全部依赖的 high 审计门禁，升级 Tailwind 4；当前项目没有旧浏览器兼容需求。
- **修复方案与实施：** 更新 Vue 3.5.44、Axios 1.20.0、markdown-it 15.0.3、Vite 7.3.7、Vitest 4.1.12；间接依赖更新至 `brace-expansion` 2.1.7、`follow-redirects` 1.16.1。升级 Tailwind CSS 4.3.3 并采用 `@tailwindcss/vite`，移除旧 PostCSS 插件及 JS 配置；用 CSS `@theme inline` 保留现有主题 Token、`.dark` 类切换和原默认边框色；更名已使用的 `shadow-sm` 与 `flex-shrink-0` 工具类。锁文件中已无 `braces` 或 `postcss-selector-parser`。
- **本地验证：** `npm ci` 成功且报告 0 vulnerabilities；`npm audit --audit-level=high` 为 0；`npm test -- --run` 为 48 文件／346 项通过；`npm run build` 通过，另用 Node 24.22.1 执行同一构建通过；`git diff --check` 通过。产物包含 `bg-page`、`text-text-tertiary`、`border-border-subtle`、`shadow-xs`、`shrink-0` 和暗色变体。用本地静态预览检查登录页明／暗主题可见；未启动后端，预览中的 API 代理拒绝连接属于预期，不能据此声明 API 或其他页面验收。
- **未完成边界：** 本机 Docker 守护进程未运行，未执行容器构建；未推送分支或标签，未触发新的远端 GitHub Actions，也未确认 GHCR 新镜像。完整页面视觉回归和 Build32 正式人工验收仍未执行。后续发布必须以新标签／运行的审计、构建和推送结果作为远端证据。
- **状态：** ◧ 本地工程修复与自动化验证完成；容器、远端 CI/GHCR 和完整浏览器验收未完成。

---

## 二、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-10-22 | 登记 R34-01 的失败根因、方案 A 的本地修复、验证结果与远端验收边界。 |
