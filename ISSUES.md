# Issues.md — 问题追踪

> **状态**: 🟡 Low 级别待修复。Critical/High/Medium 已于 2026-07-23 修复。
> **待验证**: B5 表单提交问题需在浏览器中实测确认。

---

## 待修复

### 🔴 待验证（可能是环境问题，浏览器实测后确认）

- [ ] **B5. `<el-form>` 内 `<button>` 触发原生表单提交** (`Setup.vue` + `SystemSettings.vue` + `UserManage.vue`):
  - **现象**: 点击「切换提供商」/「测试连接」/「完成配置」→ 页面重定向至 `/setup?` → 表单清空、toast 消失、弹窗中断
  - **根因推测**: `<el-form>` 渲染为 HTML `<form>`，内部 `<button>` 默认 `type="submit"`，触发浏览器原生 GET 提交
  - **已应用修复**: 3 个文件共 9 个按钮全部加 `type="button"` + 401 拦截器加 `/setup` 路径检测
  - **待确认**: 是否还有其他触发页面重载的因素

### 🟡 Low 优先级

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
