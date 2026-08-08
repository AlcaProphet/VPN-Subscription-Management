# Issue1.md — VPN 订阅管理系统 问题追踪（当前）

> **文档定位：** 本文档是 VPN 订阅管理系统的**当前问题记录**（记录错误与修复方案，非强制，经验参考）。
> 设计记录见 [Design1.md](./Design1.md)；构建方案见 [Build1.md](./Build1.md)、[Build2.md](./Build2.md)（已归档）、[Build3.md](./Build3.md)；编码指令见 [AGENTS.md](./AGENTS.md)（**唯一强要求**）。

---

## 一、进行中问题

当前无进行中问题。已闭环问题见下：

### R02-01 订阅面板报错「a.map is not a function」：列表接口契约不一致 + 空列表返回 null

- **现象：** 进入管理端「订阅」面板提示 `a.map is not a function`，列表无法渲染（未创建任何订阅时必现）；用户组/分享/平台/规则面板同样受影响。
- **根因（两个叠加问题）：**
  1. **前后端契约不一致**：`/api/admin/groups`、`/api/admin/shares`、`/api/admin/rules`、`/api/admin/platforms` 返回 `{code,data:{list,total}}`（ListData 包裹），而前端 `group.ts`/`share.ts`/`rule.ts`/`platform.ts` 按裸数组解包，对 `{list,total}` 对象调用 `.map()` 崩溃；`subscription/home/rules-user/versions` 则返回裸数组，与 AGENTS §4.8「列表统一包裹结构」不一致。
  2. **nil slice 序列化为 null**：各 List 方法 `var out []T` 空库时返回 nil slice，`response.OK`（`Data any json:"data,omitempty"`）序列化为 `null`，前端 `.map()` 同样崩溃。
- **影响范围：** 全部列表类接口与页面（订阅/用户组/分享/平台/规则/版本/用户端首页），空数据场景必现。
- **修复方案：**
  1. 后端 4 处裸数组列表改 ListData 包裹（subscription list、rule userList、home platforms、版本 list 共享组件）；
  2. 后端 10 处列表构建点 `var out []T` → `make([]T, 0)`（subscription×2、group×2、share/custom/version/rule/platform/home 各×1）；
  3. 前端 8 处列表封装统一解包 `.list`（subscription/group/platform/share/rule×2/home/version）；
  4. 新增守护测试 `TestListEmpty`（空库 List 返回非 nil 空数组）；home-view.spec mock 保持调用方视角（数组）。
- **状态：** ✅ 已修复（2026-08-09）
  - 验收命令与实际结果：`go build/vet/test` 全绿（20 包 + TestListEmpty）；`npm run build` + `npm run test` 全绿（20/20）；`docker compose build` + `up -d` 后 browser-use 真实浏览器遍历订阅/用户组/平台/分享/规则/首页全部正常渲染、无错误提示。

### R01-01 生产构建白屏：antd/vendor manualChunks 拆包触发跨 chunk 循环引用

- **现象：** Docker 部署（`docker compose up`）后浏览器访问首页一片空白；DevTools Console 报 `Cannot access 'Q' before initialization`；Network 面板 index.html 与全部 `/assets/*` 均 200 加载成功，仅 JS 执行中断导致 Vue 未挂载。Vite dev server（`npm run dev`）模式下无此问题。
- **根因：** [vite.config.ts](./frontend/vite.config.ts) 中 `manualChunks` 将 `ant-design-vue` 独立拆为 antd chunk、`vue` 等拆为 vendor chunk。ant-design-vue 4.2.x 内部存在模块循环依赖，rollup 拆包后形成**跨 chunk 循环引用**，模块初始化顺序触发 TDZ（暂时性死区）错误。Build1 Step 2 验收时仅执行构建命令、未做真实浏览器验证，导致漏检。
- **影响范围：** 所有生产构建产物（`npm run build` / `docker compose build`）均白屏，覆盖 Build1+Build2 全部前端功能；开发模式不受影响。
- **修复方案：** 移除 `manualChunks` 自定义拆分，交由 rollup 自动分割（antd 4.x 循环依赖在单 chunk 内可安全处理）。同步更新 Build1.md Step 2 的 manualChunks 描述。
- **状态：** ✅ 已修复（2026-08-09）
  - 验收命令与实际结果：`npm run build` 通过（vue-tsc + vite 无错误）；`npm run test` 20/20 通过；`docker compose build` + `up -d` 后真实浏览器验证 `http://127.0.0.1:8080/` 正常跳转 `/setup`，Setup 页完整渲染，Console 零错误。

---

## 二、格式说明（新问题记录模板）

发现问题时，按以下结构追加到「进行中问题」：

```
### RXX-01 问题标题

- **现象：** ...
- **根因：** ...
- **影响范围：** ...
- **修复方案：** ...（决策后同步更新至 BuildN 对应 Step）
- **状态：** ☐ 待修复 / ◧ 修复中 / ✅ 已修复（日期 + 验收命令）
```

**流程约定：**

1. 问题发现 → 记录现象/根因/影响范围；
2. 存在方案取舍时，使用提问工具附推荐选项与用户确认；
3. 修复方案确定后，由 [BuildN.md](./BuildN.md) 承接为构建 Step；
4. 修复完成并验收通过后，更新状态为 ✅ 并记录验收命令与实际结果；
5. 非问题的优化候选 / 已知遗留事项归 [Design1.md](./Design1.md) 记录，不记录在本文件。

---

## 三、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-09 | 初始版本：记录 R01-01 生产构建白屏（manualChunks 跨 chunk 循环依赖）修复闭环 |
| v1.1 | 2026-08-09 | 追加 R02-01 空列表接口返回 null 导致前端 .map 崩溃修复闭环（10 处列表构建点 + 守护测试） |
