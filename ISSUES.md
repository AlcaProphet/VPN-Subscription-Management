# Issues.md — 问题追踪

> **状态**: 🟡 Low 级别待修复。Critical/High/Medium 已于 2026-07-23 修复。

---

## 待修复

- [ ] **L1. `handlers.go` ~1690 行** — 建议按业务域拆分为多个文件
- [ ] **L2. 版本管理代码重复 >80%** — 4 个 service 中 UploadVersion/SwitchVersion/DeleteVersion 高度相似
- [ ] **L3. `logAccess` 写入失败静默忽略** — `repo.Insert()` 错误未记录
- [ ] **L4. `AuthLogin` prompt 参数无校验** — 建议限制长度和白名单值
- [ ] **L5. `setDownloadHeaders` 每次查 DB** — frontend_url 可缓存在 Service 初始化时
- [ ] **L6. `go.mod` 版本号 `go 1.25.0`** — 应改为实际 Go 版本
- [ ] **L7. `tailwind.css` 中未使用的 CSS 变量** — `--color-primary` 等需检查
- [ ] **L8. `App.vue` 中 `useTheme()` 返回值未使用** — 建议在 `main.js` 中调用
- [ ] **LR1. 自定义订阅 Token 刷新竞争条件** — `RefreshToken` 先读后写
- [ ] **LR2. 用户升降级时 Token 被删无通知** — Admin UI 应加警告
- [ ] **LR3. `CreateVersion` 文件/DB 操作不同步** — 各 service defer 清理不一致
- [ ] **A11. Service 层包级全局变量** — 长期可引入依赖注入

---

## 已完成（2026-07-23 会话）

<details>
<summary><b>第一轮代码审查</b>（7 项，点击展开）</summary>

- [x] **C1. OIDC Cookie SameSite** — `http.SetCookie` + `SameSite: http.SameSiteLaxMode`
- [x] **C2. Clash Verge Content-Disposition** — AGENTS.md §3.4 已更新
- [x] **C3. AES 密钥派生熵损失** — `AESKeyFromSecret` 改为 base64-decode
- [x] **H1. NoRoute 路径穿越** — 改用 `utils.SanitizePath`
- [x] **H2. `GET /rules` Token 泄露** — 方案 A：AuthRequired + `/download-link` + Rules.vue 弹窗
- [x] **H3. ConditionalSetupAuth 重复** — `ValidateJWTAndSetContext` + sentinel error
- [x] **H4. json.Unmarshal 错误忽略** — 12 处全部加 error handling

</details>

<details>
<summary><b>第二轮审计</b>（10 项，点击展开）</summary>

- [x] **A1. DeleteVersion wasCurrent** — `os.Readlink` 精确比对
- [x] **A2. Create TOCTOU** — UNIQUE constraint → 409
- [x] **A3. `:key="u.id"`** — `UserManage.vue` → `u.user_id`
- [x] **A4. OIDC 回调 error** — `AuthCallback` 检查 `?error=`
- [x] **A5. 自定义订阅刷新** — `type:custom` + `code:custom_sub_removed`
- [x] **A6. Logger Token 脱敏** — `net/url.Parse` + `url.ParseQuery`
- [x] **A7. GetUserIsAdvanced 断言** — `ok` 检查
- [x] **A8. 混合日志统一** — zerolog + ConsoleWriter + `LOG_FORMAT=json`
- [x] **A9. Setup.vue 死 CSS** — 删除 6 个未用 scoped 类
- [x] **A10. 5xx 脱敏 + Debug Mode** — `internalError()` + 管理面板开关

</details>

<details>
<summary><b>Setup / OIDC 交互修复</b>（5 项，点击展开）</summary>

- [x] **B1. 表单验证失败无 toast** — `Setup.vue` + `SystemSettings.vue`
- [x] **B2. Setup 状态检测加固** — `setupConfirmed` 标记
- [x] ~~B3. Setup 切换提供商调 API~~ — 已回退
- [x] **B4. OIDCSwitchDialog 弹窗中断** — `nextTick` 延迟关闭
- [x] **B5. `<el-form>` 内 `<button>` 触发原生提交** — 9 个按钮加 `type="button"` + 401 拦截器路径检测。**已由浏览器实测验证修复。**
- [x] **B6. OIDCSwitchDialog 改为确认式交互** — 选择后需点「确认」才应用，避免误操作
- [x] **B7. `<select>` 替换为 `<el-select>`** — 原生 select 在 append-to-body dialog 内下拉层定位错误。3 个文件 4 处全部替换
- [x] **B8. 版本页空状态 + 弹窗重叠** (`SubVersions`/`ShareVersions`/`RuleVersions` + `UploadModal`):
  - 空状态：`v-if="versions.length===0"` → `v-else`，有数据才渲染 el-table，消除三重空状态（"暂无版本" + 空表格 + "暂无数据"）
  - 弹窗重叠：`UploadModal` 加 `:modal-append-to-body="true"`，遮罩层同步 teleport 到 body，消除 el-table 固定列穿透
  - **待浏览器实测验证**

</details>

<details>
<summary><b>DEBUG 日志增强</b>（11 项，点击展开）</summary>

- [x] **D11. `LOG_LEVEL` 环境变量** — `zerolog.SetGlobalLevel()`
- [x] **D1-D4.** OIDC/Auth 后端 Debug — `InitiateLogin`, `HandleCallback`, `ValidateJWTAndSetContext`, `ConfigureSystem`
- [x] **D5+D10.** 前端 API 层 — 401 拦截器 + 请求拦截器 `console.debug`
- [x] **D6.** 版本操作 — `version_service.go` Create/Switch/Delete
- [x] **D7.** 下载端点 — `handlers.go` SubDownloadToken/ShareDownload/GetRuleDownload
- [x] **D8.** 速率限制 — `rate_limit.go` allow() 被拒绝分支
- [x] **D9.** 路由守卫 — `router/index.js` beforeEach

</details>

---

## 历史参考（Phase 1 / Phase 2）

<details>
<summary>30+ 项，点击展开</summary>

- Phase 2: 分享订阅「创建」按钮 → Tailwind
- Phase 1: CreatedAt time.Time→string, 订阅ID自动生成, 预览编辑, Clash响应头, X-Forwarded-For
- 批量: download_tokens UNIQUE, 事务保护, body限制, NextVersion, 自定义订阅端点ID, JWT_SECRET复用, 规则限流, Auth0 TrimLeft, 限流日志, version_not_found, 规则首版本, checkSystemStatus缓存, 清理逻辑, Home.vue type, CacheControl死代码, rate_limit间隔, Client Secret回显, platform参数, UploadModal uploadRef, 日志时区, AuthRequired nil

</details>
