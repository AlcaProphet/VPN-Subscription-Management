# Issues.md — 后端代码审查问题追踪

## 待修复

（无）

## 已验证，不修复

- [x] **`GetUpdateTime()` 全表遍历** (`subscription_service.go`): 加载全部订阅后在 Go 层双层循环遍历版本 JSON 找最大 `updated_at`。≤10 平台 × 2 类型 × 5 版本 = 最多 100 条记录，每首页加载 1 次（非轮询），循环耗时微秒级。SQL 聚合方案依赖 `json_each` 扩展且增加复杂度，当前规模下无收益。
- [x] **`SubDownloadToken` Token 无效时日志 `download_type` 硬编码** (`handlers.go`): Token 查找失败时固定写 `"subscription"`，无法区分 regular/custom。但 `access_logs` 表有 `CHECK(download_type IN ('subscription','share','custom','rule'))` 约束，改为 `"unknown"` 需改 schema 并处理已有数据库迁移。token_invalid 时 `status=failed, error_reason=token_invalid` 已足够排查，修复的收益远小于 schema 迁移的风险。
- [x] **OIDC state 查询未校验 TTL** (`oidc_state_repo.go:FindByState`): 查询仅 `WHERE state = ?`，无 `AND created_at > datetime('now', '-10 minutes')`。过期清理每小时执行一次，state 最长可存活 ~70 分钟。但 state 是一次性使用（回调后立即 `DELETE`），重放风险有限；且 CSRF 防护已有 HttpOnly Cookie + DB 三重校验兜底。修复需加时间条件但收益不大，暂不修复。
- [x] **`enforceMaxVersions` 在 DB 事务提交前删除旧版本文件** (`version_service.go`): `CreateVersion` 调用 `enforceMaxVersions`（`os.Remove` 旧文件）→ 回到 `UploadVersion` 执行 `UPDATE` + `Commit`。若 `UPDATE`/`Commit` 失败，defer 只清理新版本文件不恢复旧文件。触发条件极罕见（版本已满 5 个 + DB 写入失败），且 SQLite WAL 模式下 commit 失败概率极低。重构成本（拆分 enforceMaxVersions 为计算+删除两步，涉及 4 个 service 的 UploadVersion）高于风险。当前存留，后续如有类似场景再统一处理。
- [x] **Cookie 未显式设置 SameSite** (`handlers.go:AuthLogin`): Gin 的 `SetCookie` 不支持 `SameSite` 参数，生成的 `Set-Cookie` 头不含 `SameSite=...`。现代浏览器（Chrome/Firefox/Safari）对无 `SameSite` 的 cookie 默认视为 `SameSite=Lax`，OIDC 回调是顶层 GET 跳转，`Lax` 恰好允许携带 cookie。功能完全正常，CSRF 防护已有 Cookie + DB + query 三重校验。显式设置需手动拼接 header，收益仅为"声明一个与默认值一致的值"，暂不处理。
- [x] **api.js 401 拦截器未排除公开端点** (`frontend/src/services/api.js`): 拦截器对所有 401 响应无条件清除 JWT 并跳转 `/login`。但 `/auth/login` 使用 `window.location.href` 直接跳转（不走 axios），`/system/status` 后端无 AuthRequired 中间件永不返回 401。当前所有通过 axios 调用且可能返回 401 的端点（`/auth/me`、`/user/*`、`/admin/*`）均应当触发登出，拦截器行为正确。后续如有新公开端点可能返回 401，可添加排除列表作为防御性改进。
- [x] **后端不可达时 `checkSystemStatus` 失败导致路由守卫用户体验差** (`frontend/src/router/index.js`): 已通过与"`checkSystemStatus` catch 不缓存 false"（下方已修复）联动缓解。修复后网络错误时 `isConfigured` 保持 `null`，守卫不会强制跳转 `/setup`，用户最终落脚 `/login` 页面（而非无法操作的 Setup 页）。
- [x] **`Manage.vue` `activeMenu` fallback 无注释** (`frontend/src/views/Manage.vue`): `/admin`（无子路由）经 `startsWith` 全部不匹配后 fallback 到 `/admin/subscriptions`，与路由重定向一致。`/admin/rules/:id/versions` 被 `/admin/rules` 的 `startsWith` 匹配到父级菜单项，恰好是期望行为（版本管理页面高亮父级）。逻辑正确，无需修改。
- [x] **版本管理页 current 判定用 `updated_at` 排序理论不稳** (`SubVersions.vue` / `ShareVersions.vue` / `RuleVersions.vue`): 当前版本通过 `max(updated_at)` 判定。后端版本切换在事务内完成（行级锁），`updated_at` 以 `time.Now().UTC()` 写入。并发上传时事务串行执行，`updated_at` 必然不同。仅在两个请求在同一纳秒完成时可能相同，实际不会触发。用服务端返回的 current 标记（如有）会更可靠，但当前方案在管理场景下足够。暂不修复。
- [x] **分享订阅创建后 Token 显示时间过短** (`ShareList.vue`): 创建成功后通过 `ElMessage.info` 显示 Token（默认 3 秒消失）。但列表 API 已返回 `token` 字段（块 7 构建时增强），用户随时可点击「复制分享链接」按钮复制。Token 不会丢失，影响极小。暂不修复。
- [x] **`getLoginRateLimit()` / `getDownloadRateLimit()` 每次请求创建新 `SystemConfigRepo`** (`middleware/rate_limit.go`): 每次请求调用 `repository.NewSystemConfigRepo()` 创建新的 repo 实例。repo 内部使用全局 `repository.DB`，功能上正确但违背 service 单例复用模式。性能影响微乎其微（零分配 struct），当前不处理。
- [x] **`VersionService.NextVersion()` 公开方法未被外部使用** (`version_service.go`): 块 3B 设计时用于外部计算版本号。实际实现中所有 service 的 `UploadVersion` 都通过 `CreateVersion` 内部调用 `nextVersion`，`NextVersion()` 目前是未使用的公开 API。保留备用，暂不删除。
- [x] **前端列表页 `currentVersion()` 通过 `updated_at` 排序推断** (`SubList.vue` / `ShareList.vue` / `RulesManage.vue`): 使用 `versions.sort((a,b) => new Date(b.updated_at) - new Date(a.updated_at))[0]` 推断当前版本。后端 API 未返回 `current_version` 标记，语义上不严谨。未来可加强：后端列表 API 增加 `current_version` 字段。当前版本管理页（SubVersions 等）同样使用此模式。暂不修复。

## 已修复

- [x] `download_tokens` 表缺少 UNIQUE 约束，可能插入重复 Token → 添加两条 partial unique index：`(user_id, platform, type) WHERE custom_sub_id IS NULL` 和 `(user_id, platform, custom_sub_id) WHERE custom_sub_id IS NOT NULL`
- [x] `UserService.Update` 中 `target.Role` 被强制赋值后又检查 `target.Role != existing.Role`，永远为 false 的死代码 → 移除该检查
- [x] `SubscriptionService.Delete`、`RuleService.Delete`、`ShareSubscriptionService.Delete`、`UserService.Delete`、`CustomSubscriptionService.Delete` 无事务保护 → 与 `PlatformService.Delete` 一致，DB 操作包裹事务，commit 后删文件
- [x] `readUploadContent` 的 JSON 路径无 body 大小限制（multipart 有 50MB） → 已在 JSON 路径中添加 `MaxBytesReader(50MB)`
- [x] `UploadVersion` 中 `NextVersion` 被外部和 `CreateVersion` 内部各算一次 → 移除外部 `NextVersion` 调用，改为从 `CreateVersion` 返回的 `newVersions` 末尾元素提取版本号
- [x] **自定义订阅版本管理端点传错 ID** — 5 个 handler（`UploadCustomSubscriptionVersion`、`DeleteCustomSubscription`、`GetCustomVersion`、`SwitchCustomVersion`、`DeleteCustomVersion`）将路由 `:id`（用户 ID）当作自定义订阅 ID 使用。修复：统一改为 `userID + ?platform=` → `GetByUserAndPlatform` → `cs.ID` 调 service 方法。
- [x] **`ConfigureSystem` 每次调用重新生成 JWT_SECRET** — Normal 模式下管理员在 OIDC 配置页保存 → 全员 JWT 失效 + 其他提供商加密 secret 无法解密。修复：先检查 `JWT_SECRET` 是否存在，存在则复用；仅首次配置时生成新密钥。
- [x] **规则下载端点缺少速率限制** — `GET /rules/:id/download` 未挂 `RateLimitDownload` 中间件。修复：router.go 补充 `middleware.RateLimitDownload()`。
- [x] **Auth0 域名 `TrimLeft` 误用为 cutset** — `strings.TrimLeft(domain, "https://")` 第二参数是字符集，会将域名中含 h/t/p/s 的字符错误截断。修复：两处（handlers.go、oidc_service.go）改用 `strings.TrimPrefix`。
- [x] **速率限制触发的下载未记录 access_logs** — 限流中间件 `writeRateLimitResponse` 不写日志。修复：新增 `repository.InsertAccessLog` 包级函数 + middleware 中新增 `logRateLimitedDownload`，从 URL 路径推断 download_type 及相关 ID 后写入日志。
- [x] **下载失败未区分 `version_not_found`** — 4 个下载 handler 在内容读取失败时统一写 `file_not_found`。修复：新增 `errorReasonFromErr` 辅助函数，匹配 `"no versions"` 返回 `version_not_found`，其余返回 `file_not_found`。
- [x] **规则创建不支持上传首个版本文件** — `CreateRule` 只接受 JSON 创建空记录，不与 `CreateShare` 一致支持一步创建+首版本上传。修复：`CreateRule` 同时支持 JSON（含 `content` 字段）和 multipart（form 字段 + file）两种方式，创建后立即调用 `UploadVersion` 写入首版本，失败则清理 DB 记录。
- [x] **`checkSystemStatus()` 网络错误后永久缓存 `false`** (`frontend/src/stores/user.js`): catch 分支将 `isConfigured` 设为 `false` 并永久缓存。后端未启动时首次请求失败后，即使后端恢复正常也会被永久卡在 `/setup` 页面。修复：catch 中不设值（保持 `null`），下次路由守卫运行时自动重试。
- [x] **`createRuleWithFirstVersion` RefreshToken 失败未清理 DB + 文件** (`handlers.go`): 函数流程为 `Create` → `UploadVersion` → `RefreshToken`。`UploadVersion` 成功后若 `RefreshToken` 失败，DB 中留下无 token 的规则记录+版本文件。修复：`RefreshToken` 失败时调用 `RuleSvc.Delete()` 级联清理。
- [x] **`ShareSubscriptionService.Create` Token 创建失败未清理 DB + 文件** (`share_subscription_service.go`): 与上面规则创建对称。`repo.Create` + `CreateVersion` + `UpdateVersions` 全部成功后，若 `tokenRepo.Create` 失败，留下无 token 的分享订阅。修复：`tokenRepo.Create` 失败时调用 `repo.Delete()` + `RemoveVersionDir()` 清理。
- [x] **`Home.vue` 自定义订阅刷新发送了错误的 type 参数** (`Home.vue`): `handleRefresh` 对自定义订阅发送 `platform.sub_type`（如 'advanced'）而非 'custom'，虽然后端检测 custom_sub 自动兜底，但语义不清。修复：显式传 `type: 'custom'`，并添加注释说明后端检测逻辑。
- [x] **`CacheControlMiddleware` 死代码** (`middleware/cache_control.go`): 该中间件定义了但从未在 router 中注册，且若全局使用会与 `NoCacheForDownloads` 产生重复/冲突的 `Cache-Control` 头。修复：删除该文件。
- [x] **速率限制器 `periodicCleanup` 间隔过长** (`middleware/rate_limit.go`): 清理间隔 5 分钟 + 2 分钟 cutoff，导致过期 IP 记录可在内存中残留最多 ~7 分钟。修复：改为 2 分钟间隔 + 1 分钟 cutoff，与限流窗口 (1 分钟) 对齐。
