# 构建计划 — Phase 3（Low 优先级 ISSUES 修复）

> **状态**: 计划已确认，所有 8 项决策已定案（详见第八章）。等待实施。
> **前置条件**: Phase 2 全部完成（UI 迁移到 Tailwind + 单容器架构）。
> **目标**: 修复 `ISSUES.md` 中所有 🟡 Low 优先级项（共 12 项），为 Future_Plan_01（本地用户管理）清理技术债。
> **总预估工作量**: 2–3 天。

---

## 〇、与 AGENTS.md 的关系

本阶段不新增功能、不改变产品行为、不修改 API 契约。所有改动均属于代码质量优化与 bug 修复，AGENTS.md 无需更新（除非 L1 拆分后 handler 文件命名规范需要补充）。

---

## 一、当前代码事实清单（构建前必须核对）

### 1.1 关键文件行数与函数分布

| 文件 | 行数 | 函数数 | 说明 |
|------|------|--------|------|
| `backend/internal/handler/handlers.go` | ~1806 | 81 | 所有 HTTP handler 集中在单文件 |
| `backend/internal/handler/services.go` | ~60 | 2 (`InitServices`, `SetDebugMode`) | 服务实例初始化 + 包级全局变量 |
| `backend/internal/service/version_service.go` | ~312 | 15 | 共享版本管理逻辑 |
| `backend/internal/service/subscription_service.go` | ~400 | ~18 | 含 UploadVersion/SwitchVersion/DeleteVersion |
| `backend/internal/service/rule_service.go` | ~300 | ~14 | 含 UploadVersion/SwitchVersion/DeleteVersion |
| `backend/internal/service/share_subscription_service.go` | ~345 | ~16 | 含 UploadVersion/SwitchVersion/DeleteVersion |
| `backend/internal/service/custom_subscription_service.go` | ~310 | ~14 | 含 UploadVersion/SwitchVersion/DeleteVersion + RefreshToken |
| `backend/internal/service/user_service.go` | ~145 | 7 | 含 Update（Token 删除逻辑） |
| `backend/internal/service/system_service.go` | ~90 | 8 | GetDebugMode/SetDebugMode |
| `backend/internal/service/platform_service.go` | ~130 | ~9 | |
| `frontend/src/views/UserManage.vue` | ~520 | ~20 | 含 is_advanced 编辑逻辑 |
| `frontend/src/assets/tailwind.css` | ~8 | — | 仅 Tailwind 指令 + toast 动画 |
| `frontend/src/App.vue` | ~40 | — | `useTheme()` 调用但未使用返回值 |
| `frontend/tailwind.config.js` | ~13 | — | 定义 `colors.primary` 但未被任何组件引用 |

### 1.2 已验证的环境信息

- 本地 Go 版本: `go1.26.4 darwin/arm64`
- Docker 构建镜像: `golang:alpine` → `go1.26.5`
- 当前 `go.mod` 声明: `go 1.25.0`（格式错误，Go 版本号不含 `.0` 后缀）
- 前端构建: `npm run build` ✅ 通过（零错误）

### 1.3 Go 同包多文件规则

Go 允许同一 `package` 的代码分散在多个 `.go` 文件中，所有文件共享包级变量和函数。**L1 拆分时无需修改任何函数签名或 import**，仅将函数按域移动到新文件即可。当前 `services.go` 中定义的包级变量（`PlatformSvc`, `UserSvc` 等）在新文件中直接可用。

---

## 二、阶段 1：秒修项（≤ 1 小时）

### 2.1 L6 — `go.mod` 版本号修正

**问题**: `go.mod` 第 3 行声明 `go 1.25.0`，此版本号格式不正确（Go 版本号不含 `.0` 后缀）。实际使用的 Go 版本：本地 `go1.26.4`，Docker `golang:alpine` → `go1.26.5`。

**方案**: 将 `go 1.25.0` 改为 `go 1.26`（与本地和 Docker 构建环境保持一致）。

**改动**:
```
文件: backend/go.mod, 第 3 行
旧: go 1.25.0
新: go 1.26
```

改动后运行 `go mod tidy`（遵守 AGENTS.md 编码约束：「修改 go.mod 后必须运行 go mod tidy」）。

**验证**: `go build ./...` 通过。

---

### 2.2 L3 — `logAccess` 写入失败加日志

**问题**: `handlers.go` 第 1767 行，`repo.Insert()` 的返回值被丢弃（`_ = repo.Insert(...)`），日志写入失败时无任何记录，问题排查困难。

**当前代码** (`handlers.go:1746-1768`):
```go
func logAccess(userID, ip, downloadType, platform, shareSubID, ruleID, status, errorReason string) {
    identifier := userID
    if userID != "" {
        userRepo := repository.NewUserRepo()
        if u, err := userRepo.FindByID(userID); err == nil && u.Email != "" {
            identifier = u.Email
        } else if err != nil {
            log.Debug().Str("user_id", userID).Err(err).Msg("logAccess: FindByID failed")
        } else {
            log.Debug().Str("user_id", userID).Str("username", u.Username).Msg("logAccess: FindByID OK but email empty")
        }
    }
    repo := repository.NewAccessLogRepo()
    _ = repo.Insert(&repository.AccessLogRecord{...})  // ← 错误被忽略
}
```

**方案**: 在 `Insert` 失败时使用 `log.Error()` 记录错误。

**改动**:
```go
// 将 _ = repo.Insert(...) 改为:
if err := repo.Insert(&repository.AccessLogRecord{...}); err != nil {
    log.Error().Err(err).
        Str("user_id", userID).
        Str("ip", ip).
        Str("type", downloadType).
        Str("platform", platform).
        Str("status", status).
        Msg("logAccess: Insert failed")
}
```

**验证**: 编译通过。可选：手动触发下载后在日志中确认 error 级别日志正常输出。

---

### 2.3 L7 — Tailwind CSS 未使用变量清理

**问题**: `tailwind.config.js` 第 9 行定义了 `colors.primary`，但 grep 确认 `frontend/src/` 下**没有任何组件**使用 `primary` 这个 color name。此配置为过渡期 Element Plus 颜色对齐用，Phase 2 完成后已无引用。

**方案**: 从 `tailwind.config.js` 的 `theme.extend.colors` 中删除 `primary` 定义。

**改动**:
```
文件: frontend/tailwind.config.js
删除:
      colors: {
        // 与 Element Plus primary 对齐，便于过渡期混用
        primary: { DEFAULT: '#409eff', dark: '#409eff' },
      },
保留空的 theme.extend（或整个 extend 块移除）:
    extend: {},
```

**验证**: `npm run build` 通过。

---

### 2.4 L8 — `App.vue` 中 `useTheme()` 调用位置调整

**问题**: `App.vue` 第 22 行 `useTheme()` 被调用但返回值未被使用。主题初始化是副作用（设置 `document.documentElement.classList` 和监听 `prefers-color-scheme`），但放在 `App.vue` 中每次路由切换时不会重新执行（因为 `App.vue` 是根组件只挂载一次），所以实际上正常工作。但语义上这个初始化更适合放在 `main.js` 中（应用启动时执行一次）。

**方案**: 在 `main.js` 中调用 `useTheme()` 进行初始化，从 `App.vue` 中移除。

**改动**:

`frontend/src/main.js`:
```js
// 在 import App from './App.vue' 之前添加:
import { useTheme } from '@/composables/useTheme'

// 在 app.mount('#app') 之前添加:
useTheme() // 初始化暗色模式（读取 localStorage + 系统偏好）
```

`frontend/src/App.vue`:
```vue
<!-- 删除 <script setup> 中的这行: -->
useTheme()
<!-- 删除对应的 import: -->
import { useTheme } from '@/composables/useTheme'
```

**验证**: `npm run build` 通过。浏览器中验证暗色模式切换正常。

---

## 三、阶段 2：结构优化（~半天）

### 3.1 L1 — `handlers.go` 按业务域拆分

**问题**: `handlers.go` 约 1806 行、81 个函数，全部挤在单个文件中。Future_Plan_01 将新增 ~12 个端点，不拆分会导致文件膨胀到 2500+ 行。

**方案**: 按业务域拆分为以下文件（同一 `package handler`，共享 `services.go` 中的变量）:

| 新文件 | 包含函数 | 大约行数 |
|--------|---------|----------|
| `handlers.go` (保留) | `GetSystemStatus`, `GetPlatforms`, `GetRules`, `GetRuleDownloadLink`, `GetRuleDownload` + 所有 helper 函数 (`readUploadContent`, `parseVersionParam`, `setDownloadHeaders`, `logAccess`, `errorReasonFromErr`) | ~200 |
| `auth_handlers.go` | `AuthLogin`, `AuthCallback`, `AuthMe` | ~180 |
| `user_handlers.go` | `UserPlatforms`, `UserUpdateTime`, `UserRefreshToken` | ~200 |
| `subscription_handlers.go` | `handleJWTDownload`, `SubDownload`, `SubDownloadPreview`, `SubDownloadToken`, `ShareDownload` | ~200 |
| `admin_setup_handlers.go` | `PostConfigure`, `PostSwitchProvider`, `PostTestOIDC`, `GetOIDCConfig` | ~280 |
| `admin_user_handlers.go` | `ListUsers`, `GetUser`, `UpdateUser`, `DeleteUser`, `RevokeUserTokens`, `UploadCustomSubscription`, `UploadCustomSubscriptionVersion`, `DeleteCustomSubscription`, `GetCustomVersion`, `SwitchCustomVersion`, `DeleteCustomVersion`, `RefreshCustomSubToken` | ~330 |
| `admin_subscription_handlers.go` | `ListSubscriptions`, `CreateSubscription`, `GetSubscription`, `UpdateSubscription`, `DeleteSubscription`, `UploadSubscriptionVersion`, `SwitchSubscriptionVersion`, `GetSubscriptionVersion`, `DeleteSubscriptionVersion` | ~190 |
| `admin_share_handlers.go` | `ListShares`, `CreateShare`, `GetShare`, `UpdateShare`, `DeleteShare`, `UploadShareVersion`, `SwitchShareVersion`, `GetShareVersion`, `DeleteShareVersion`, `RefreshShareToken`, `RevokeShareToken` | ~230 |
| `admin_platform_handlers.go` | `ListPlatforms`, `CreatePlatform`, `GetPlatform`, `UpdatePlatform`, `DeletePlatform` | ~70 |
| `admin_rule_handlers.go` | `ListAdminRules`, `CreateRule`, `GetAdminRule`, `UpdateAdminRule`, `DeleteAdminRule`, `UploadRuleVersion`, `SwitchRuleVersion`, `GetRuleVersion`, `DeleteRuleVersion`, `RefreshRuleToken`, `createRuleWithFirstVersion` | ~200 |
| `admin_system_handlers.go` | `GetRateLimit`, `UpdateRateLimit`, `GetAnnouncement`, `UpdateAnnouncement`, `PublicAnnouncement`, `GetDebugMode`, `UpdateDebugMode` | ~130 |
| `admin_log_handlers.go` | `GetLogs` | ~40 |

**操作方式**: 纯代码移动（cut & paste），不做任何逻辑修改。每个新文件保留相同的 `package handler` 声明和必要的 import。`services.go` 保持不变。

**Import 处理**: 每个新文件独立管理 import。拆分时对每个文件——复制原 `handlers.go` 的 import block → 编译 → 根据 `unused import` 编译错误逐个删除未使用的 import → 编译通过 → 下一个文件。

**拆分顺序**（按依赖关系，从叶子到根）:
1. `admin_log_handlers.go` — 只依赖 `SystemSvc`，最独立
2. `admin_platform_handlers.go` — 只依赖 `PlatformSvc`
3. `admin_rule_handlers.go` — 依赖 `RuleSvc`
4. `admin_share_handlers.go` — 依赖 `ShareSvc`
5. `admin_subscription_handlers.go` — 依赖 `SubSvc`
6. `admin_system_handlers.go` — 依赖 `SystemSvc`
7. `admin_user_handlers.go` — 依赖 `UserSvc`, `CustomSubSvc`
8. `admin_setup_handlers.go` — 依赖 `auth.DefaultService`, `middleware`
9. `subscription_handlers.go` — 依赖 `SubSvc`, `CustomSubSvc`, `ShareSvc`
10. `auth_handlers.go` — 依赖 `auth.DefaultService`
11. `user_handlers.go` — 依赖 `PlatformSvc`, `SubSvc`, `CustomSubSvc`
12. `handlers.go` — 保留公共 handler + 所有 helper

每完成一个文件拆分，立即运行 `go build ./...` 验证。

**潜在风险**:
- `handlers.go` 中的 import block 被所有函数共享，拆分后每个新文件需独立管理 import。逐个文件编译 → 删未使用 import → 再编译的方式虽慢但精确。
- 无其他实际风险 — Go 同包不同文件共享所有符号，无需修改函数签名或调用方式。

**验证**: `go build ./...` + `go vet ./...` 通过。

---

## 四、阶段 3：独立质量修复（~半天）

### 4.1 L5 — `setDownloadHeaders` 缓存 `frontend_url`

**问题**: `setDownloadHeaders` 每次调用都执行 `repository.NewSystemConfigRepo().Get("frontend_url")` 查数据库。下载端点调用频繁（每个客户端请求都触发），属于不必要的 DB 查询。

**当前代码** (`handlers.go:1730-1743`):
```go
func setDownloadHeaders(c *gin.Context, platform string) {
    if platform != "clash-verge" {
        return
    }
    c.Header("Content-Disposition", `attachment; filename*=UTF-8''Luneflare%20VPN%20Clash.yaml`)
    c.Header("profile-update-interval", "300")
    cfgRepo := repository.NewSystemConfigRepo()
    if frontendURL, err := cfgRepo.Get("frontend_url"); err == nil && frontendURL != "" {
        c.Header("profile-web-page-url", frontendURL)
    }
}
```

**方案**: 在 `services.go` 中添加一个包级变量缓存 `frontend_url`，在 `InitServices` 中读取一次并缓存整个服务生命周期。管理员修改 `frontend_url` 后需重启服务生效。这是可接受的权衡——该字段极少变更，简单缓存避免每次下载请求查 DB。

**改动**:

`services.go`:
```go
// CachedFrontendURL is initialized at startup and may be empty if not configured.
// Read once at startup; an admin must restart the service after changing
// frontend_url via the admin panel for it to take effect in download headers.
var CachedFrontendURL string

func InitServices() {
    // ... 现有代码 ...
    // 缓存 frontend_url（启动时读取一次，全生命周期有效）
    cfgRepo := repository.NewSystemConfigRepo()
    if url, err := cfgRepo.Get("frontend_url"); err == nil {
        CachedFrontendURL = url
    }
}
```

`handlers.go` 中 `setDownloadHeaders`:
```go
func setDownloadHeaders(c *gin.Context, platform string) {
    if platform != "clash-verge" {
        return
    }
    c.Header("Content-Disposition", `attachment; filename*=UTF-8''Luneflare%20VPN%20Clash.yaml`)
    c.Header("profile-update-interval", "300")
    if CachedFrontendURL != "" {
        c.Header("profile-web-page-url", CachedFrontendURL)
    }
}
```

**验证**: `go build ./...` 通过。检查下载响应头中 `profile-web-page-url` 是否正确。

**验证**: `go build ./...` 通过。检查下载响应头中 `profile-web-page-url` 是否正确。

---

### 4.2 L4 — `AuthLogin` prompt 参数校验

**问题**: `AuthLogin` handler 直接将从 query string 读取的 `prompt` 参数传给 OIDC provider。OIDC spec 规定 `prompt` 仅允许 `none`、`login`、`consent`、`select_account`（可空格分隔组合）。当前无任何校验。

**方案**: 添加长度限制（≤128 字符）和值格式校验。对于不符合 OIDC spec 的值返回 400。

**改动** (`auth_handlers.go` 中 `AuthLogin` 函数):
```go
func AuthLogin(c *gin.Context) {
    // ...
    prompt := c.Query("prompt")
    
    // 校验 prompt 参数
    if prompt != "" {
        if len(prompt) > 128 {
            c.JSON(http.StatusBadRequest, gin.H{"error": "prompt parameter too long"})
            return
        }
        // 校验 prompt 值（可选严格模式，仅允许 OIDC spec 规定的值）
        // 当前实现: 仅长度限制，不做值白名单校验
        // 如需严格校验，启用以下代码:
        // validPrompts := map[string]bool{"none": true, "login": true, "consent": true, "select_account": true}
        // for _, p := range strings.Fields(prompt) {
        //     if !validPrompts[p] {
        //         c.JSON(http.StatusBadRequest, gin.H{"error": "invalid prompt value"})
        //         return
        //     }
        // }
    }
    }
}
```

> 已将白名单校验逻辑以注释形式保留在代码中，供未来需要严格校验时启用。

**验证**: `go build ./...` 通过。

---

### 4.3 LR2 — 用户升降级时 Token 删除 UI 警告

**问题**: `UserService.Update` (第 70 行) 在 `is_advanced` 变更时静默删除用户所有下载 Token。管理员在 `UserManage.vue` 编辑用户、切换 is_advanced 开关后保存时，**没有任何提示**告知 Token 将被删除。用户下次访问首页时 Token 静默丢失（重新生成），但管理员不知道自己操作的影响。

**方案**: 在 `UserManage.vue` 编辑对话框中，当 `is_advanced` 开关被切换（与原始值不同）时，显示一条警告文字。`handleEditSave` 调用前加 `ConfirmDialog` 提醒。

**改动**:

`UserManage.vue` 编辑对话框的订阅级别区域:
```vue
<el-form-item label="订阅级别">
  <!-- ...现有 toggle 按钮... -->
  <div v-if="editIsAdvanced !== editUser.is_advanced" 
       class="mt-2 text-sm text-orange-600 dark:text-orange-400 bg-orange-50 dark:bg-orange-900/20 rounded-md px-3 py-2">
    ⚠️ 变更订阅级别将立即删除该用户的所有下载链接。用户下次访问首页时将自动获取新链接。
  </div>
</el-form-item>
```

> **说明**: 无需额外 `ConfirmDialog` 弹窗 — 警告文字在切换时实时显示在表单中，管理员在点击保存前即可看到。这比弹窗更友好（不打断操作流），且信息充分。

**验证**: 前端构建通过。浏览器中验证：编辑用户 → 切换 is_advanced → 警告文字出现 → 切回原值 → 警告消失。

---

## 五、阶段 4：Service 层重构（~1.5 天）

### 5.1 LR1 — 自定义订阅 Token 刷新竞争条件

**问题**: `CustomSubscriptionService.RefreshToken` (第 294 行) 使用"先读后写"模式：
```go
func (s *CustomSubscriptionService) RefreshToken(customSubID string) error {
    oldToken, err := s.tokenRepo.FindTokenByCustomSubID(customSubID) // 1. 读
    if err != nil {
        return nil
    }
    newToken, err := utils.GenerateToken()                           // 2. 生成
    if err != nil {
        return err
    }
    return s.tokenRepo.ReplaceTokenValue(oldToken, newToken)         // 3. 用旧值定位 UPDATE
}
```

竞争场景：两次快速刷新请求同时读到同一个 `oldToken`，第一个 UPDATE 成功，第二个 UPDATE 用已被替换的旧值作为 WHERE 条件 → 影响 0 行 → Token 未刷新但返回成功。

**对比**: `SubscriptionService.RefreshToken` 使用 `ReplaceTokenForSub(userID, platform, subType, newToken)`，WHERE 条件是业务键 `(user_id, platform, type)` 而非 token 值本身，天然免疫此竞争。

**方案**: 将 `ReplaceTokenValue(oldToken, newToken)` 改为基于业务键的 UPDATE：新增 `ReplaceTokenByCustomSubID(customSubID, newToken)` 方法。

**改动**:

`download_token_repo.go` 新增方法:
```go
// ReplaceTokenByCustomSubID atomically replaces any token for a custom subscription
// using the business key (custom_sub_id), avoiding the read-then-write race.
func (r *DownloadTokenRepo) ReplaceTokenByCustomSubID(customSubID, newToken string) (bool, error) {
    result, err := DB.Exec(
        `UPDATE download_tokens SET token = ? WHERE custom_sub_id = ?`,
        newToken, customSubID,
    )
    if err != nil {
        return false, err
    }
    n, _ := result.RowsAffected()
    return n > 0, nil
}
```

`custom_subscription_service.go` 中 `RefreshToken`:
```go
func (s *CustomSubscriptionService) RefreshToken(customSubID string) error {
    newToken, err := utils.GenerateToken()
    if err != nil {
        return err
    }
    updated, err := s.tokenRepo.ReplaceTokenByCustomSubID(customSubID, newToken)
    if err != nil {
        return err
    }
    if !updated {
        // No existing token — nothing to refresh
        return nil
    }
    return nil
}
```

**同时清理**: 旧方法 `ReplaceTokenValue(oldToken, newToken)` 和 `FindTokenByCustomSubID` 如果无其他调用方可删除。

**验证**: `go build ./...` + `go vet ./...` 通过。逻辑上与 `SubscriptionService.RefreshToken` 的模式一致。

---

### 5.2 LR3 — `CreateVersion` 文件/DB 操作清理不一致

**问题**: 4 个 service 的 `UploadVersion` 方法和 2 个 `Create` 方法在文件写入失败时的清理逻辑不一致：

| 方法 | 清理模式 | 问题 |
|------|---------|------|
| `SubscriptionService.UploadVersion` | `defer` + `committed` flag | ✅ 正确 |
| `RuleService.UploadVersion` | `defer` + `committed` flag | ✅ 正确 |
| `ShareSubscriptionService.UploadVersion` | `defer` + `committed` flag | ✅ 正确 |
| `CustomSubscriptionService.UploadVersion` | `defer` + `committed` flag | ✅ 正确 |
| `ShareSubscriptionService.Create` | 手动 if/else 清理，无事务包装 | ⚠️ 创建记录 → 写版本文件 → 更新 DB，中间任一步失败清理不完整 |
| `CustomSubscriptionService.Upload` (create 分支) | `s.repo.Delete(id)` 清理 DB | ⚠️ 未清理可能已写入的版本文件 |

**方案**: 在 L2（版本管理去重）中统一解决。通用事务方法将覆盖所有 4 个 service 的 `UploadVersion` 以及 2 个 `Create` 方法，确保失败清理路径一致。当前阶段 LR3 作为 L2 的验收标准——"所有 Create/UploadVersion 方法的失败清理路径是否一致"。

**验证**: L2 完成后，审查以下方法的 defer cleanup 一致性：
- `SubscriptionService.UploadVersion`
- `RuleService.UploadVersion`
- `ShareSubscriptionService.UploadVersion` + `ShareSubscriptionService.Create`
- `CustomSubscriptionService.UploadVersion` + `CustomSubscriptionService.Upload` (create 分支)

---

### 5.3 L2 — 版本管理代码去重

**问题**: 4 个 service（`SubscriptionService`, `RuleService`, `ShareSubscriptionService`, `CustomSubscriptionService`）中的 `UploadVersion`、`SwitchVersion`、`DeleteVersion` 三个方法代码重复率 >80%。每种方法的核心逻辑相同：

```
UploadVersion:    开启事务 → SELECT versions (行锁) → 解析 JSON → versionSvc.CreateVersion → defer 清理 → UPDATE versions → 提交
SwitchVersion:    开启事务 → SELECT versions (行锁) → 解析 JSON → versionSvc.SwitchVersion → UPDATE versions → 提交
DeleteVersion:    开启事务 → SELECT versions (行锁) → 解析 JSON → versionSvc.DeleteVersion → UPDATE versions → 提交
```

唯一不同的是表名、资源 ID、subDir 前缀。

**方案**: 利用现有的 `VersionService` 已有文件操作抽象，进一步将 DB 事务逻辑也提取为通用函数。

**设计**: 在 `version_service.go` 中新增三个方法，接受函数参数来处理 DB 读写：

```go
// VersionedResource 是拥有 versions JSON 字段的任何资源的抽象。
type VersionedResource interface {
    GetID() string
    GetSubDir() string       // e.g., "subscriptions/abc", "rules/xyz"
    GetVersions() []models.Version
    SetVersions(v []models.Version)
}

// 或者使用更简单的函数式方案:
// TxVersionOp 封装事务中的版本操作。
type TxVersionOp struct {
    TableName     string                           // e.g., "subscriptions", "rules"
    IDColumn      string                           // e.g., "id"
    ResourceID    string
    SubDir        string                           // e.g., "subscriptions/abc"
    CurrentVersions *[]models.Version              // 会被原地更新
}

func (s *VersionService) UploadVersionTx(op TxVersionOp, content string) error
func (s *VersionService) SwitchVersionTx(op TxVersionOp, versionNum int) error
func (s *VersionService) DeleteVersionTx(op TxVersionOp, versionNum int) error
```

每个方法内部处理：`BEGIN → SELECT ... (通过 QueryRow 行锁) → JSON 解析 → 文件操作 → defer cleanup → UPDATE → COMMIT`。

同时将 `ShareSubscriptionService.Create` 和 `CustomSubscriptionService.Upload` (创建新记录分支) 也迁移到此通用事务模板中（解决 LR3），确保所有版本创建路径的清理逻辑一致。

**改动范围**:

**改动范围**:
1. `version_service.go` 新增 ~150 行（3 个通用方法 + TxVersionOp struct）
2. 4 个 service 各删除 ~100 行（UploadVersion/SwitchVersion/DeleteVersion 简化为 5 行调用）
3. 净减少 ~250 行重复代码

**简化后的 service 方法示例**:
```go
func (s *SubscriptionService) UploadVersion(id, content string) (*models.Subscription, error) {
    sub, err := s.repo.FindByID(id)
    if err != nil {
        return nil, fmt.Errorf("subscription not found")
    }
    err = s.versionSvc.UploadVersionTx(service.TxVersionOp{
        TableName:       "subscriptions",
        IDColumn:        "id",
        ResourceID:      id,
        SubDir:          "subscriptions/" + id,
        CurrentVersions: &sub.Versions,
    }, content)
    if err != nil {
        return nil, err
    }
    return sub, nil
}
```

这是本阶段风险最大的改动。通过以下措施控制风险：
- 4 个 service 的版本操作逻辑在 Phase 1/2 期间已高度稳定
- 每个 migration step 单独 commit（通用方法 → 逐个 service 迁移 → 删除旧代码），每步可独立 revert
- `go build ./...` + `go vet ./...` 保证编译安全
- 浏览器实测 4 种资源类型的完整版本管理流程

**验证**: `go build ./...` + `go vet ./...` 通过。在浏览器中实际测试：上传版本 → 切换当前版本 → 删除旧版本，覆盖 4 种资源类型（订阅/规则/分享订阅/自定义订阅）。

---

## 六、阶段 5：架构决策（设计阶段，无需写代码）

### 6.1 A11 — Service 层依赖注入规划

**问题**: `services.go` 使用包级全局变量存储服务实例（`PlatformSvc`, `UserSvc`, ...），handler 函数直接引用这些全局变量。这导致：
- 单元测试困难（无法 mock 服务）
- 服务生命周期与包绑定
- Future_Plan_01 新增服务会继续"传染"这个模式

**方案**: 设计一个 DI 容器方案，写入 `AGENTS.md` 作为 Future_Plan_01 及以后的新服务规范，**当前阶段不强制重构现有代码**。

**推荐方案**: 构造函数注入 + 结构体 Handler

```go
// 新架构（供 Future_Plan_01 使用）
type UserHandler struct {
    userSvc    *service.UserService
    customSvc  *service.CustomSubscriptionService
}

func NewUserHandler(userSvc *service.UserService, customSvc *service.CustomSubscriptionService) *UserHandler {
    return &UserHandler{userSvc: userSvc, customSvc: customSvc}
}

func (h *UserHandler) ListUsers(c *gin.Context) {
    users, err := h.userSvc.List()
    // ...
}
```

在 `router.go` 中组装:
```go
userHandler := handler.NewUserHandler(userSvc, customSvc)
admin.GET("/users", userHandler.ListUsers)
```

**迁移策略**:
1. Future_Plan_01 的所有新 handler **必须**使用结构体+构造函数注入
2. 现有 handler 函数**不强制重构**（保持全局变量可用）
3. 当某个现有 handler 需要修改时（如 L1 拆分后），**顺手**将其重构为结构体注入
4. 逐步淘汰 `services.go` 中的全局变量

**写入 AGENTS.md**:
在 `AGENTS.md` 的「编码约束」→「后端 Handler」节末尾添加：
```
- 新增 Handler 必须使用结构体 + 构造函数注入服务依赖（禁止新增包级全局 service 变量）
- 修改现有 Handler 时鼓励顺手重构为结构体注入，但不强制
```

---

## 七、执行顺序总览

```
阶段 1: L6 → L3 → L7 → L8  (并行独立, ≤1h)
    ↓
阶段 2: L1  (拆分 handlers.go, ~3h)
    ↓
阶段 3: L5 → L4 → LR2  (独立质量修复, ~3h)
    ↓
阶段 4: LR1 → L2 → LR3  (LR1 独立, L2 包含 LR3, ~6h)
    ↓
阶段 5: A11  (设计文档, 0.5h)
```

**每阶段完成后的验证门禁**:
- 阶段 1/3: `go build ./...` + `npm run build` 通过
- 阶段 2: 同上 + 每个拆分文件 commit 一次，可独立 revert
- 阶段 4: 同上 + 浏览器实测版本管理操作

---

## 八、决策确认（2026-07-23）

所有待决策项已确认，实施时以此为最终方案。

| # | 项 | 决策 | 选定的方案 |
|---|-----|------|------|
| 1 | L6: `go.mod` 目标版本 | **`go 1.26`** | 与本地 `go1.26.4` 和 Docker `golang:alpine` → `go1.26.5` 保持一致。改动后运行 `go mod tidy` |
| 2 | L5: `frontend_url` 运行时更新 | **仅启动时缓存**（选项 A） | 管理员修改后需重启服务生效。简单可靠，无额外复杂度 |
| 3 | L4: `prompt` 参数校验严格度 | **仅长度限制**（推荐方案） | 兼容所有 OIDC provider。白名单逻辑以注释保留供未来启用 |
| 4 | L2: 版本管理去重方案 | **通用事务方法**（选项 A） | 一劳永逸；同时解决 LR3。每个 migration step 独立 commit |
| 5 | LR3: Create 方法事务改造 | **在 L2 中统一解决**（选项 A） | 通用事务方法覆盖所有 6 个版本的创建/上传路径，LR3 作为验收标准 |
| 6 | A11: DI 规范 | **强制新代码用 DI**（选项 A） | 「必须」使用结构体+构造函数注入，写入 AGENTS.md |
| 7 | L1: 拆分后 import 整理 | **手动逐个文件整理**（选项 A） | 复制 import block → 编译 → 删未使用 → 编译通过，精确可控 |
| 8 | L6: `go mod tidy` | **运行**（选项 A） | 遵守 AGENTS.md 编码约束 |

---

## 九、不可并行事项

- **L1 拆分与 L2 去重不可并行** — L2 需要在拆分后的文件结构上进行，否则改动位置不明确
- **L2 必须在 LR3 之前** — LR3 的清理一致性由 L2 统一解决
- **L5 必须在 L1 之后** — L5 的缓存变量在 `services.go`，L1 拆分后的 handler 文件结构确定后再动 `services.go`
- **阶段 5（A11）可与阶段 3/4 并行** — 纯设计文档，无代码依赖

---

## 十、回滚策略

- **阶段 1**: 每个改动独立 commit，可单独 revert
- **阶段 2 (L1)**: 拆分前在 `handlers.go` 上打 tag，拆分后若出现问题可整体 revert 到 tag
- **阶段 3**: 每个改动独立 commit
- **阶段 4 (L2)**: 风险最高，拆分为多个小 commit（通用事务方法 → 逐个 service 迁移 → 删除旧代码），每个步骤可 revert
- **阶段 5**: 纯文档，无需回滚
