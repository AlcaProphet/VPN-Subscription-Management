# Issues.md — 问题追踪

> **状态**: 🟡 仅 Low 级别待修复。本次会话所有 Critical/High/Medium 问题已于 2026-07-23 修复。
> **待检查**: Setup/SystemSettings OIDC 表单交互修复需在浏览器中实际验证。

---

## 本次会话修复（2026-07-23）

### 第一轮代码审查

- [x] **C1. OIDC Cookie 缺少 SameSite** — `http.SetCookie` + `SameSite: http.SameSiteLaxMode`
- [x] **C2. Clash Verge Content-Disposition 文档化** — AGENTS.md §3.4 已更新
- [x] **C3. AES 密钥派生熵损失** — `AESKeyFromSecret` 改为 base64-decode 恢复原始字节
- [x] **H1. NoRoute 路径穿越** — 改用 `utils.SanitizePath`
- [x] **H2. `GET /rules` Token 泄露** — 方案 A：加 AuthRequired + 新增 `/download-link` 端点 + Rules.vue 弹窗交互
- [x] **H3. ConditionalSetupAuth 代码重复** — 抽取 `ValidateJWTAndSetContext` + sentinel error
- [x] **H4. 12 处 json.Unmarshal 错误忽略** — 全部加 error handling

### 第二轮审计（外部报告核对）

- [x] **A1. `DeleteVersion` wasCurrent 语义错误** — `os.Readlink` 精确比对
- [x] **A2. `Create` TOCTOU 竞态** — 捕获 UNIQUE constraint 返回 409
- [x] **A3. `:key="u.id"` 字段名错误** — `UserManage.vue` 改为 `u.user_id`
- [x] **A4. OIDC 回调 error 参数** — `AuthCallback` 检查 `?error=` 返回友好提示
- [x] **A5. 自定义订阅刷新边缘情况** — 后端 `type:custom` + `code:custom_sub_removed`；前端静默刷新列表
- [x] **A6. Logger Token 脱敏** — `net/url.Parse` + `url.ParseQuery` 替换字符串匹配
- [x] **A7. GetUserIsAdvanced 防御性断言** — `.(bool)` 改为 `ok` 检查
- [x] **A8. 混合日志统一** — 方案 C：zerolog + ConsoleWriter + `LOG_FORMAT=json`
- [x] **A9. Setup.vue 死 CSS** — 删除 6 个未用 scoped 类
- [x] **A10. 5xx 脱敏 + Debug Mode** — `internalError()` 辅助函数 + 管理面板开关

### Setup / OIDC 配置交互修复

- [x] **B1. 表单验证失败无 toast** — `Setup.vue` + `SystemSettings.vue` 的 `handleTest`/`handleSubmit`/`handleSave` 验证失败时加 `toastError`
- [x] **B2. Setup 状态检测加固** — 新增 `setupConfirmed` 标记，操作前重新验证 `configured` 状态
- [x] ~~B3. Setup 切换提供商调 API~~ — **已回退**，该调用引入了不必要的 401 风险，根因见 B5
- [x] **B4. OIDCSwitchDialog 弹窗中断** — `onSelect` 中用 `nextTick` 延迟关闭弹窗
- [x] **B5. `<el-form>` 内 `<button>` 缺少 `type="button"` 导致原生表单提交** (`Setup.vue` + `SystemSettings.vue` + `UserManage.vue`):
  - **根因**: `<el-form>` 渲染为 HTML `<form>`，内部 `<button>` 默认 `type="submit"`。点击任意按钮触发浏览器原生 GET 提交 → URL 变为 `/setup?`（空 query string）→ 页面完全刷新 → 表单清空、toast 消失、弹窗中断
  - **修复**: 3 个文件共 9 个 `<el-form>` 内按钮全部加 `type="button"`

---

## 待修复（Low 优先级，暂不处理）

### DEBUG 日志增强（2026-07-23 已完成 D1-D11）

- [x] **D11. `LOG_LEVEL` 环境变量** (`main.go`): `LOG_LEVEL=debug|info|warn` → `zerolog.SetGlobalLevel()`，默认 info
- [x] **D1. OIDC `InitiateLogin`** (`oidc_service.go`): state + PKCE + prompt
- [x] **D2. OIDC `HandleCallback`** (`oidc_service.go`): code exchange、首次管理员、JWT 签发
- [x] **D3. `ValidateJWTAndSetContext`** (`middleware/auth.go`): JWT 失败原因、用户查找
- [x] **D4. `ConfigureSystem`** (`oidc_service.go`): JWT_SECRET 新建 vs 复用
- [x] **D5. 前端 401 拦截器** (`api.js`): 端点 + 页面路径
- [x] **D6. 版本操作** (`version_service.go`): 创建/删除/切换 + 是否 current
- [x] **D7. 下载端点** (`handlers.go`): 订阅/分享/规则下载成功
- [x] **D8. 速率限制** (`rate_limit.go`): IP + 计数 + 限制值 + Retry-After
- [x] **D9. 路由守卫** (`router/index.js`): from→to + 状态快照
- [x] **D10. API 请求拦截器** (`api.js`): METHOD + URL + JWT

### 代码结构

- [ ] **L1. `handlers.go` ~1690 行** — 建议按业务域拆分为多个文件
- [ ] **L2. 版本管理代码重复 >80%** — 4 个 service 中 UploadVersion/SwitchVersion/DeleteVersion 高度相似
- [ ] **L3. `logAccess` 写入失败静默忽略** — `repo.Insert()` 错误未记录
- [ ] **L4. `AuthLogin` prompt 参数无校验** — 建议限制长度和白名单值
- [ ] **L5. `setDownloadHeaders` 每次查 DB** — frontend_url 可缓存在 Service 初始化时
- [ ] **L6. `go.mod` 版本号 `go 1.25.0`** — 应改为实际 Go 版本
- [ ] **L7. `tailwind.css` 中未使用的 CSS 变量** — `--color-primary` 等需检查
- [ ] **L8. `App.vue` 中 `useTheme()` 返回值未使用** — 建议在 `main.js` 中调用

### 逻辑风险

- [ ] **LR1. 自定义订阅 Token 刷新竞争条件** — `RefreshToken` 先读后写
- [ ] **LR2. 用户升降级时 Token 被删无通知** — Admin UI 应加警告
- [ ] **LR3. `CreateVersion` 文件/DB 操作不同步** — 各 service defer 清理不一致
- [ ] **A11. Service 层包级全局变量** — 长期可引入依赖注入

---

## 历史参考

<details>
<summary>Phase 1 / Phase 2 修复记录（30+ 项，点击展开）</summary>

- Phase 2: 分享订阅「创建」按钮 bug → Tailwind 替代 v-loading + el-empty
- Phase 1: 自定义订阅 500 错误 → CreatedAt time.Time → string
- Phase 1: 订阅 ID 自动生成、预览版本编辑、Clash Verge 响应头、X-Forwarded-For
- 批量修复: download_tokens UNIQUE 约束、事务保护、body 大小限制、NextVersion 重复计算
- 批量修复: 自定义订阅端点 ID 错误、JWT_SECRET 复用、规则下载速率限制、Auth0 TrimLeft
- 批量修复: 限流下载日志、version_not_found 区分、规则创建首版本上传、checkSystemStatus 缓存
- 批量修复: createRuleWithFirstVersion 清理、ShareSubscriptionService.Create 清理
- 批量修复: Home.vue type 参数、CacheControlMiddleware 死代码、rate_limit 间隔
- 批量修复: Client Secret 回显、自定义订阅 platform 参数、UploadModal uploadRef、日志时区
- 批量修复: AuthRequired nil 防御

</details>
