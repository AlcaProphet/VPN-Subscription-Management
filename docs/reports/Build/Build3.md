# Build3.md — 管理面补全与运维能力（当前构建方案）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第三轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则），承接已归档的 [Build1.md](Build1.md)（骨架与认证）、[Build2.md](Build2.md)（订阅核心与用户端）（前两轮须全部验收通过后本轮方可启动）。
> - 设计基线：[Design1.md](../Design/Design1.md)（与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design1-UI.md](../Design/Design1-UI.md)
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 前置轮次：Build1.md、Build2.md（均已归档）
>
> **里程碑：本 Build 全部 Step 完成后，系统具备完整的管理面（用户管理/审批中心/面板配置/日志）与运维能力（配置导入导出/备份下载/应急恢复），达到可交付状态。**

---

## 执行约束（执行 AI 必须严格遵守）

1. **严格按 Step 顺序执行**，完成一个 Step 并验收通过后，方可进入下一个 Step。**禁止跳步、禁止并行执行多个 Step、禁止自行合并步骤**。
2. **每个 Step 完成后必须运行该 Step 的「验证命令」**，全部通过才算完成；任一命令失败必须修复后重验，禁止带错进入下一 Step。
3. **遇到模糊、歧义或设计文档未覆盖的细节，必须停止并向用户询问，禁止自行假设或自由发挥**。本文件未明确的技术选型，以 Design1.md §5.1 为准。
4. **禁止引入设计文档未提及的框架、库或架构模式**（技术栈同 Build1 执行约束第 4 条）。配置导出加密使用 Argon2id + AES-256-GCM（Design1 §3.4.8/6.2），Argon2id 使用 `golang.org/x/crypto/argon2`。
5. **Build1/Build2 已建立的机制必须复用，禁止重复实现**：构造注入、配置存储（含敏感加密）、日志（token 脱敏）、统一响应、会话/角色中间件、标识生成器、迁移框架、`BEGIN IMMEDIATE` 事务助手、版本管理组件、Token 服务、限流中间件、验证码服务、前端路由守卫/拦截器/通用组件。**列表接口必须沿用统一 ListData 包裹（`{list,total}`）+ 前端 api 层统一解包约定：全量列表 api 用 `.then(d => d.list)` 解包为数组，分页列表保留包裹结构由调用方取 list/total（R02-01 确立，见 Issue1）**。
6. **关键设计参数必须严格按下表取值**，与 Design1.md 保持一致，禁止修改：

| 参数 | 取值 | 出处 |
|------|------|------|
| 管理员保护 | 禁止删自己/删最后管理员/改自己角色/禁用最后活跃管理员/禁用自己（五重） | Design1 §2.5 |
| 直接重置密码 | 随机 8 位，大小写字母+数字，去除易混淆字符 i I o O 0 l L，无特殊符号 | Design1 §3.4.5 |
| 密码重置令牌 | 一次性、1 小时 TTL、用后即删、≥128 位 | Design1 §4.6 |
| 一键清空确认词 | 固定 `RESET` + 二次确认 | Design1 §3.4.8 |
| 配置导入确认词 | 固定 `IMPORT` + 二次确认 | Design1 §3.4.8 |
| 导出密码 | ≥8 字符，Argon2id 派生密钥 + AES-256-GCM | Design1 §3.4.8 |
| 配置导入/导出 | 仅 Production 模式提供（Dev 不提供） | Design1 §3.4.8 |
| 站点名称 | ≤50 字符 | Design1 §3.4.8 |
| 站点 ICON | ≤2MB，仅 png/jpeg/webp/ico（排除 SVG/GIF） | Design1 §3.4.8 |
| 系统公告 | 纯文本 ≤2000 字符，前端转义渲染禁 HTML | Design1 §3.4.8 |
| 应急操作码 | 8 位大写字母+数字，去易混淆字符；严格一次性（每次提交即消耗）；仅存内存 | Design1 §3.8 |
| SSE 并发上限 | 全局 8 连接（不按管理员计） | Design1 §4.8 |
| SSE 短期 Token | ≥128 位，单次连接建立后即删，未使用 5 分钟 TTL | Design1 §4.8 |
| 实时日志环形缓冲 | 最近 500 条 | Design1 §4.8 |
| 访问日志保留 | 90 天自动清理 | Design1 §3.4.9 |
| 日志级别 | debug/info/warn/error，运行时切换并持久化 | Design1 §3.4.8 |
| 用户管理分页 | 后端分页默认 20 条/页 + 用户名/邮箱模糊搜索 | UI §5.5 |
| 审批中心分页 | 后端分页 | UI §5.6 |

7. **注释使用中文**；所有 error 必须处理；禁止 `fmt.Println` 调试输出。
8. **职责分层**与**按业务域拆分处理器文件**的约束同 Build1。
9. **所有管理端点必须叠加会话校验 + 角色校验双层中间件**；权限信息每次请求实时查库，禁止缓存。
10. **错误码约定**同 Build1 执行约束第 11 条。
11. **敏感配置（OIDC Client Secret、SMTP 密码）必须以 AES-256-GCM 加密落库，面板回显一律脱敏（`a-input-password`），禁止返回明文；验证码站点/服务端密钥为明文存储（面板回显真实值，非敏感配置，R11-01）**。

---

## TODOLIST CheckList（构建进度核对）

> 执行 AI 必须逐个完成并勾选。状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

- ✅ Step 1：用户管理（含全部操作项与批量操作）
- ✅ Step 2：审批中心与邮件服务
- ✅ Step 3：面板配置（认证/验证码/SMTP/站点/限流/日志/公告/调试）
- ✅ Step 4：危险操作区与配置导入导出、备份下载
- ✅ Step 5：日志查看（访问日志 + 实时日志流 SSE）
- ✅ Step 6：应急恢复模式

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 用户管理（含全部操作项与批量操作） | Design1 §3.4.5/2.5/4.6 | ✅ 验收通过 |
| 2 | 审批中心与邮件服务 | Design1 §3.4.6/4.6/3.4.8 | ✅ 验收通过 |
| 3 | 面板配置（认证/验证码/SMTP/站点/限流/日志/公告/调试） | Design1 §3.4.8 | ✅ 验收通过 |
| 4 | 危险操作区与配置导入导出、备份下载 | Design1 §3.4.8/4.8/6.2/7.2 | ✅ 验收通过 |
| 5 | 日志查看（访问日志 + 实时日志流 SSE） | Design1 §3.4.9/4.8 | ✅ 验收通过 |
| 6 | 应急恢复模式 | Design1 §3.8 | ✅ 验收通过 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件（核心） | 要点 |
|------|----------------|------|
| 1 | `backend/internal/user/admin.go`、`backend/internal/server/user.go`、`frontend/src/views/admin/UsersView.vue` | 用户 CRUD + 角色/禁用/重置密码/清绑定/吊销 Token + 批量发链接 |
| 2 | `backend/internal/{approval,mail}/...`、`backend/internal/server/approval.go`、`frontend/src/views/admin/ApprovalsView.vue` | 审批通过/拒绝/批量 + SMTP + 欢迎/通知邮件 |
| 3 | `backend/internal/config/admin.go`、`backend/internal/server/settings.go`、`frontend/src/views/admin/SettingsView.vue` | 面板配置全分区 |
| 4 | `backend/internal/{config/export.go,backup,dataclear}/...`、`backend/internal/server/settings_ops.go` | 全清 + 导入导出 + 备份下载 |
| 5 | `backend/internal/log/{access,stream}.go`、`backend/internal/server/log.go`、`frontend/src/views/admin/LogsView.vue` | 访问日志查询/清空 + SSE 实时流 |
| 6 | `backend/internal/emergency/...`、`backend/internal/server/emergency.go`、`frontend/src/views/EmergencyView.vue` | 应急模式 + 操作码 + 重置/重初始化 |

---

## 三、构建顺序依赖图

```
Step 1（用户管理）──▶ Step 2（审批中心操作用户；邮件服务供用户管理批量发链接）
Step 2（邮件服务）──▶ Step 3（面板配置 SMTP 分区配置邮件服务参数）
Step 3（面板配置）──▶ Step 4（导入导出/备份/全清操作配置与全量数据）
Step 1/2/3 ──▶ Step 5（日志查看依赖完整业务产生访问/运行日志）
Step 3/4 ──▶ Step 6（应急恢复依赖配置存储与用户体系；全清与应急重初始化机制独立）
```

> 线性执行序：Step 1 → 2 → 3 → 4 → 5 → 6。

---

## 四、分步构建计划

---

### Step 1：用户管理（含全部操作项与批量操作）

**本 Step 完成后，系统应具备：用户列表（后端分页+搜索）、编辑/角色变更/自定义订阅管理/吊销 Token/设置重置密码/清除 OIDC 绑定/禁用启用/删除用户、批量发送密码设置链接的完整用户管理能力。**

- **目标：** 实现管理员对用户的全生命周期管理，落地五重管理员保护。
- **前置条件：** Build1（认证/用户体系）、Build2（自定义订阅机制、Token 服务）已验收。
- **产出文件与操作：**

  1. **创建 `backend/internal/user/admin.go`（业务层）**：用户管理服务。必须实现：
     - **列表**：后端分页（默认 20 条/页）+ 用户名/邮箱模糊搜索；返回字段：用户名、邮箱（无邮箱标注）、角色、所属组、来源、状态、自定义订阅平台标记。
     - **编辑**：调整所属组（换组无需清 Token，实时解析跟随）。
     - **角色变更**（admin↔user）：**仅可由其他管理员执行**（禁止改自己角色）；**admin→user 降级时级联清除其全部显式订阅 Token**。
     - **吊销所有下载 Token**：物理删除该用户全部 Token 记录（无标记态），用户下次访问首页重新生成。
     - **设置/重置密码**（关键约束，Design1 §3.4.5）：
       - 可对任意用户执行（含其他管理员）；**管理员不能通过本入口操作自己的密码**。
       - **两种方式二选一**：**触发重置邮件**（已配置 SMTP 时可选，发送一次性重置链接 1h TTL，用户自设密码）；**直接重置**（二次确认后系统随机生成 8 位密码——大小写字母+数字，去除易混淆字符 i I o O 0 l L，无特殊符号——展示供复制，即时生效）。
       - **重置后递增该用户 credential_version（全部现有会话立即失效）**。
       - **对「待审批」账号不提供本操作**（拒绝并提示「请先在审批中心处理」）。
       - **无邮箱用户可在此手动补填邮箱**（补填后获得设置密码/重置能力）。
     - **清除 OIDC 绑定**：清空 `oidc_subject`；**该用户无本地密码时显著警告「清除后该用户将无法登录，建议先为其设置本地密码」**（警告由前端展示，后端返回该用户是否有密码标记）。
     - **禁用/启用**：**禁止管理员禁用自己**；**禁止禁用最后一个活跃（未禁用）管理员**；**禁用 = 同一事务内递增 credential_version + 物理删除其全部 Token**；启用后不恢复原 Token。
     - **删除用户**：级联删全部 Token + 自定义订阅（含版本文件）；**禁止删除自己**；**禁止删除最后一个管理员**；**待审批账号删除与审批中心「拒绝」同效果（账号删除、邮箱释放）**。
     - **批量操作**：「为所有无密码用户发送密码设置链接」——**仅面向已激活的无密码用户，待审批与已禁用自动排除并提示；无邮箱用户自动排除并提示**；依赖 SMTP（未配置时前端置灰）。本 Step 实现筛选与令牌生成，邮件发送在 Step 2 接通（本 Step 可先以日志记录代替并标注）。
     - **新建用户**：用户名 + 邮箱 + 密码（邮箱唯一冲突 409；密码复杂度 ≥8）。

  2. **创建用户端点（接入层，`backend/internal/server/user.go`）**：会话 + 管理员：
     - `GET /api/admin/users`（分页+搜索）、`POST /api/admin/users`（新建）、`PUT /api/admin/users/:id`（编辑/换组）、`PUT /api/admin/users/:id/role`（角色变更）、`POST /api/admin/users/:id/tokens/revoke`（吊销 Token）、`POST /api/admin/users/:id/password/reset`（重置密码，body 区分 `send_email`/`direct`）、`DELETE /api/admin/users/:id/oidc`（清绑定）、`PUT /api/admin/users/:id/status`（禁用/启用）、`DELETE /api/admin/users/:id`（删除）、`POST /api/admin/users/send_password_links`（批量发链接）。
     - 全部操作执行前校验五重管理员保护。

  3. **创建前端 `frontend/src/views/admin/UsersView.vue`**（UI §5.5）：
     - 双态列表（后端分页 20 条/页 + 搜索框用户名/邮箱模糊；卡片态展示前 4 字段）：用户名、邮箱（无邮箱灰 tag）、角色标签、所属组标签、来源标注、状态 `a-badge`（待审批橙/已激活绿/已禁用灰）、自定义订阅平台角标、操作 Dropdown。
     - 头部批量操作「为所有无密码用户发送密码设置链接」（未配置 SMTP 置灰 + 提示；执行后回执提示排除范围）。
     - **操作 Dropdown**：编辑（分组 Select）、角色变更（ConfirmModal，降级 提示级联清显式 Token）、上传自定义订阅（平台 Select + 文件/文本，成功展示 custom- 标识）、**自定义订阅版本管理**（存在自定义订阅时可点，跳 `/admin/customs/:id/versions`；**该路由返回按钮 backPath 当前指向 `/admin/subscriptions`（R04-01 暂定），本 Step 建立 `/admin/users` 后同步改为 `/admin/users`**）、删除自定义订阅、吊销所有 Token（ConfirmModal 危险）、**设置/重置密码**（专属弹窗二选一：触发重置邮件/直接重置；直接重置确认后展示随机 8 位密码供复制 + 提示会话已失效；待审批账号拒绝并提示去审批中心；管理员本人入口禁用）、清除 OIDC 绑定（无密码用户先弹显著警告）、禁用/启用（禁用自己置灰）、删除（ConfirmModal，待审批账号与「拒绝」同效果说明）。
     - 新建用户弹窗（用户名 + 邮箱 + 密码，邮箱冲突即时提示）。
     - `frontend/src/api/user.ts`。

- **参考代码/伪代码：**

  > 编写顺序：internal/user/admin.go（五重保护 + 各操作）→ server/user.go → UsersView.vue。复用：user 包既有服务（扩展 admin.go 文件）、Token 服务、重置令牌服务（Build1 Step 7）、双中间件。

  **1. `backend/internal/user/admin.go`（业务层：用户管理服务）**

  ```go
  // 五重管理员保护错误（Design1 §2.5，接入层统一映射 403/400）
  var (
      ErrSelfOperation     = errors.New("不能对自己执行此操作")
      ErrLastAdmin         = errors.New("不能删除/降级/禁用最后一个活跃管理员")
      ErrPendingNotAllowed = errors.New("请先在审批中心处理待审批账号")
  )

  type AdminService struct {
      store    *store.Store
      users    *Service          // 复用基础用户服务
      tokens   *token.Service
      resetSvc *auth.ResetService
      log      *slog.Logger
  }

  // --- 五重保护校验辅助（均在事务内实时查库，不缓存）---

  // checkNotSelf：操作者不能操作自己（删自己/改自己角色/禁用自己）
  func (s *AdminService) checkNotSelf(operatorID, targetID int64) error {
      if operatorID == targetID {
          return ErrSelfOperation
      }
      return nil
  }

  // countActiveAdmins：活跃（未禁用）管理员数，排除指定用户
  func (s *AdminService) countActiveAdmins(ctx context.Context, tx *sql.Tx, excludeID int64) (int, error) {
      var n int
      err := tx.QueryRowContext(ctx,
          `SELECT COUNT(*) FROM users WHERE role = 'admin' AND status = 'active' AND id != ?`, excludeID).Scan(&n)
      return n, err
  }

  // --- 列表：后端分页（默认 20 条/页）+ 用户名/邮箱模糊搜索 ---

  type ListQuery struct {
      Page, Size int
      Keyword    string // 用户名/邮箱模糊
  }

  func (s *AdminService) List(ctx context.Context, q ListQuery) ([]AdminUser, int64, error) {
      if q.Size <= 0 {
          q.Size = 20
      }
      // SELECT 含：用户名/邮箱（无邮箱前端标注）/角色/所属组名/来源/状态/自定义订阅平台标记（子查询聚合 custom_subscriptions.platform_id）
      // WHERE (username LIKE ? OR email LIKE ?)，LIMIT/OFFSET + COUNT(*) 总数
      ...
  }

  // --- 角色变更（admin↔user）：仅可由其他管理员执行；降级级联清显式 Token ---

  func (s *AdminService) ChangeRole(ctx context.Context, operatorID, targetID int64, newRole string) error {
      if err := s.checkNotSelf(operatorID, targetID); err != nil { // 禁止改自己角色
          return err
      }
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var curRole string
          if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, targetID).Scan(&curRole); err != nil {
              return err
          }
          if curRole == "admin" && newRole == "user" {
              remaining, err := s.countActiveAdmins(ctx, tx, targetID)
              if err != nil {
                  return err
              }
              if remaining == 0 {
                  return ErrLastAdmin // 降级最后一个活跃管理员
              }
          }
          if _, err := tx.ExecContext(ctx, `UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newRole, targetID); err != nil {
              return err
          }
          // 降级（admin→user）同事务级联清全部显式订阅 Token（Design1 §2.5）
          if curRole == "admin" && newRole == "user" {
              if _, err := tx.ExecContext(ctx,
                  `DELETE FROM download_tokens WHERE user_id = ? AND subscription_id IS NOT NULL`, targetID); err != nil {
                  return err
              }
          }
          return nil
      })
  }

  // --- 设置/重置密码（关键约束，Design1 §3.4.5）---

  // 直接重置字符集：大小写字母 + 数字，去除易混淆 i I o O 0 l L，无特殊符号，8 位
  const directResetCharset = "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"

  // genDirectResetPassword：crypto/rand 取 8 字符
  func genDirectResetPassword() (string, error) {
      b := make([]byte, 8)
      if _, err := rand.Read(b); err != nil {
          return "", fmt.Errorf("生成随机密码失败: %w", err)
      }
      for i := range b {
          b[i] = directResetCharset[int(b[i])%len(directResetCharset)]
      }
      return string(b), nil
  }

  // ResetPasswordDirect：二次确认由前端负责；待审批账号拒绝；重置后递增 credential_version（同事务）
  func (s *AdminService) ResetPasswordDirect(ctx context.Context, operatorID, targetID int64) (string, error) {
      if operatorID == targetID {
          return "", ErrSelfOperation // 管理员不能通过本入口操作自己的密码
      }
      pwd, err := genDirectResetPassword()
      if err != nil {
          return "", err
      }
      hash, err := auth.HashPassword(pwd)
      if err != nil {
          return "", err
      }
      err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var status string
          if err := tx.QueryRowContext(ctx, `SELECT status FROM users WHERE id = ?`, targetID).Scan(&status); err != nil {
              return err
          }
          if status == "pending" {
              return ErrPendingNotAllowed
          }
          _, err := tx.ExecContext(ctx,
              `UPDATE users SET password_hash = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
              hash, targetID) // 重置后全部现有会话立即失效
          return err
      })
      if err != nil {
          return "", err
      }
      return pwd, nil // 接入层返回明文密码供复制（仅此一次）
  }

  // ResetPasswordByEmail：已配置 SMTP 时可选；生成一次性重置令牌（复用 Build1 Step 7 ResetService）并发送
  func (s *AdminService) ResetPasswordByEmail(ctx context.Context, operatorID, targetID int64) error {
      // 待审批拒绝；无邮箱拒绝（提示先补填）；调 resetSvc 生成令牌 + mail 发送（Step 2 接通）
      ...
  }

  // FillEmail：无邮箱用户补填邮箱（唯一校验 409；补填后获得设置密码/重置能力）
  func (s *AdminService) FillEmail(ctx context.Context, targetID int64, emailRaw string) error { /* NormalizeEmail + 唯一预查 + UPDATE */ }

  // --- 禁用/启用（禁用 = 同事务递增 credential_version + 物理删全部 Token）---

  func (s *AdminService) SetStatus(ctx context.Context, operatorID, targetID int64, disable bool) error {
      if err := s.checkNotSelf(operatorID, targetID); err != nil { // 禁止禁用自己
          return err
      }
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if disable {
              var role string
              if err := tx.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, targetID).Scan(&role); err != nil {
                  return err
              }
              if role == "admin" {
                  remaining, err := s.countActiveAdmins(ctx, tx, targetID)
                  if err != nil {
                      return err
                  }
                  if remaining == 0 {
                      return ErrLastAdmin // 禁止禁用最后一个活跃管理员
                  }
              }
              // 同一事务：递增 credential_version（会话立即失效）+ 物理删除全部 Token（防竞态窗口）
              if _, err := tx.ExecContext(ctx,
                  `UPDATE users SET status = 'disabled', credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, targetID); err != nil {
                  return err
              }
              if _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, targetID); err != nil {
                  return err
              }
              return nil
          }
          // 启用：不恢复原 Token（用户下次访问首页重新生成）
          _, err := tx.ExecContext(ctx, `UPDATE users SET status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, targetID)
          return err
      })
  }

  // --- 删除用户（级联 + 保护）---

  func (s *AdminService) Delete(ctx context.Context, operatorID, targetID int64) error {
      if err := s.checkNotSelf(operatorID, targetID); err != nil { // 禁止删自己
          return err
      }
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var role, status string
          if err := tx.QueryRowContext(ctx, `SELECT role, status FROM users WHERE id = ?`, targetID).Scan(&role, &status); err != nil {
              return err
          }
          if role == "admin" && status == "active" {
              remaining, err := s.countActiveAdmins(ctx, tx, targetID)
              if err != nil {
                  return err
              }
              if remaining == 0 {
                  return ErrLastAdmin // 禁止删除最后一个管理员
              }
          }
          // 级联：全部 Token（download_tokens 外键已 CASCADE，显式 DELETE 保语义清晰）+ 自定义订阅（含版本文件）
          if _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, targetID); err != nil {
              return err
          }
          // 收集自定义订阅 ID → DELETE versions（owner=custom）→ DELETE custom_subscriptions
          ...
          // 待审批账号删除与审批中心「拒绝」同效果：账号删除、邮箱释放（DELETE 行即释放唯一约束）
          if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, targetID); err != nil {
              return err
          }
          return nil
      })
      // 事务提交后删自定义版本文件（失败记日志）
  }

  // --- 其他操作 ---

  // UpdateGroup：调整所属组（换组无需清 Token，实时解析跟随）
  func (s *AdminService) UpdateGroup(ctx context.Context, targetID, groupID int64) error { /* UPDATE users SET group_id */ }

  // RevokeAllTokens：吊销所有下载 Token（物理删除，无标记态）
  func (s *AdminService) RevokeAllTokens(ctx context.Context, targetID int64) error {
      _, err := s.store.DB().ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, targetID)
      return err
  }

  // ClearOidcBinding：清空 oidc_subject；返回 has_password 标记供前端警告（无密码时清除后无法登录）
  func (s *AdminService) ClearOidcBinding(ctx context.Context, targetID int64) (hasPassword bool, err error) {
      // SELECT password_hash IS NOT NULL → UPDATE users SET oidc_subject = NULL
      ...
  }

  // BatchSendPasswordLinks：仅面向已激活的无密码用户；待审批/已禁用/无邮箱自动排除并回执计数
  func (s *AdminService) BatchSendPasswordLinks(ctx context.Context) (sent, skippedPending, skippedDisabled, skippedNoEmail int, err error) {
      // SELECT id, email, status FROM users WHERE password_hash IS NULL
      // 逐类统计排除；对合格者生成重置令牌（resetSvc）+ 发邮件（Step 2 接通前日志记录）
      ...
  }

  // Create：新建用户（邮箱唯一冲突 409；密码复杂度 ≥8；来源 local，默认直接激活）
  func (s *AdminService) Create(ctx context.Context, username, emailRaw, password string) (*User, error) {
      // NormalizeEmail + ValidatePassword + HashPassword + 唯一预查事务 + INSERT（status=active, source=local）
      ...
  }
  ```

  **2. `backend/internal/server/user.go`（用户端点；会话 + 管理员）**

  ```go
  type UserAdminHandler struct{ adminSvc *user.AdminService }

  func RegisterUserAdminRoutes(engine *gin.Engine, h *UserAdminHandler, sessionMW, adminMW gin.HandlerFunc) {
      g := engine.Group("/api/admin/users", sessionMW, adminMW)
      g.GET("", h.list)                                  // ?page=&size=&keyword=
      g.POST("", h.create)
      g.PUT("/:id", h.update)                            // 编辑/换组（body: group_id；可附 email 补填）
      g.PUT("/:id/role", h.changeRole)                   // body: { role: admin|user }
      g.POST("/:id/tokens/revoke", h.revokeTokens)
      g.POST("/:id/password/reset", h.resetPassword)     // body: { mode: "send_email"|"direct" }
      g.DELETE("/:id/oidc", h.clearOidc)
      g.PUT("/:id/status", h.setStatus)                  // body: { disabled: bool }
      g.DELETE("/:id", h.delete)
      g.POST("/send_password_links", h.batchSendLinks)
  }

  // 统一保护错误映射：ErrSelfOperation/ErrPendingNotAllowed → 400；ErrLastAdmin → 403
  func mapProtectErr(c *gin.Context, err error) bool {
      switch {
      case errors.Is(err, user.ErrSelfOperation), errors.Is(err, user.ErrPendingNotAllowed):
          server.Fail(c, http.StatusBadRequest, err.Error())
      case errors.Is(err, user.ErrLastAdmin):
          server.Fail(c, http.StatusForbidden, err.Error())
      case errors.Is(err, user.ErrEmailConflict):
          server.Fail(c, http.StatusConflict, "该邮箱已被使用")
      default:
          return false
      }
      return true
  }

  // resetPassword：mode=direct → 返回 { password: "随机 8 位" }（仅一次展示）；mode=send_email → SMTP 未配置返 400 提示
  ```

  **3. 前端 `UsersView.vue`（UI §5.5）**

  ```vue
  <script setup lang="ts">
  // 双态列表（后端分页 20 条/页 + 搜索框用户名/邮箱模糊；卡片态展示前 4 字段）：
  // 用户名、邮箱（无邮箱灰 tag）、角色标签、所属组标签、来源标注、状态 a-badge（待审批橙/已激活绿/已禁用灰）、
  // 自定义订阅平台角标、操作 Dropdown
  const page = ref(1); const size = ref(20); const keyword = ref('')
  async function load() {
    // GET /admin/users?page=&size=&keyword= → { list, total }（统一列表包裹结构）
  }
  watch([page, keyword], () => load())

  // 头部批量操作「为所有无密码用户发送密码设置链接」：未配置 SMTP 置灰 + 提示；执行后回执提示排除范围
  async function batchSendLinks() {
    const res = await sendPasswordLinks()
    Notify.success(`已发送 ${res.sent} 封；排除：待审批 ${res.skipped_pending}、已禁用 ${res.skipped_disabled}、无邮箱 ${res.skipped_no_email}`)
  }

  // 操作 Dropdown（a-dropdown + a-menu）：
  //   编辑（分组 Select）/角色变更（ConfirmModal，降级提示级联清显式 Token）/
  //   上传自定义订阅（平台 Select + 文件/文本，成功展示 custom- 标识）/
  //   自定义订阅版本管理（存在时可点，跳 /admin/customs/:id/versions）/删除自定义订阅/
  //   吊销所有 Token（ConfirmModal 危险）/设置重置密码（专属弹窗）/清除 OIDC 绑定（无密码先弹显著警告）/
  //   禁用启用（禁用自己置灰）/删除（ConfirmModal，待审批账号与「拒绝」同效果说明）

  // 设置/重置密码弹窗（二选一）：
  //   触发重置邮件（SMTP 未配置置灰）/ 直接重置（二次确认 → 成功后展示随机 8 位密码供复制 + 提示会话已失效）
  //   待审批账号：入口拒绝并提示去审批中心；管理员本人（当前登录者）入口禁用
  const resetOpen = ref(false); const resetMode = ref<'send_email' | 'direct'>('direct')
  const directResult = ref('') // 直接重置成功后的随机密码（仅一次展示）
  async function doReset() {
    const res = await resetPassword(target.value!.id, { mode: resetMode.value })
    if (resetMode.value === 'direct') {
      directResult.value = res.password
      Notify.info('该用户全部现有会话已失效')
    } else {
      Notify.success('重置邮件已发送')
    }
  }
  </script>
  ```

  ```ts
  // api/user.ts（要点）
  // 分页列表保留 {list,total} 包裹（调用方取 list/total），与全量列表 api 的 .then(d => d.list) 解包约定区分（R02-01）
  export const listUsers = (q: { page: number; size: number; keyword: string }) =>
    http.get<any, { list: AdminUser[]; total: number }>('/admin/users', { params: q })
  export const changeRole = (id: number, role: string) => http.put(`/admin/users/${id}/role`, { role })
  export const resetPassword = (id: number, data: { mode: 'send_email' | 'direct' }) =>
    http.post<any, { password?: string }>(`/admin/users/${id}/password/reset`, data)
  export const setStatus = (id: number, disabled: boolean) => http.put(`/admin/users/${id}/status`, { disabled })
  export const revokeTokens = (id: number) => http.post(`/admin/users/${id}/tokens/revoke`)
  export const clearOidc = (id: number) => http.delete<any, { has_password: boolean }>(`/admin/users/${id}/oidc`)
  export const sendPasswordLinks = () => http.post<any, { sent: number; skipped_pending: number; skipped_disabled: number; skipped_no_email: number }>('/admin/users/send_password_links')
  ```

  **4. 单元测试要点（验收要求）**

  - 五重保护逐条：删自己/改自己角色/禁用自己 → ErrSelfOperation；删最后管理员/降级最后活跃管理员/禁用最后活跃管理员 → ErrLastAdmin；待审批拒绝重置。
  - 降级清显式 Token：admin→user 后显式 Token 全删、无标识 Token 保留。
  - 禁用：同事务 credential_version + 1 且 Token 全删（旧会话 401、旧 Token 404）；启用后不恢复。
  - 直接重置：8 位密码字符集断言（逐字符属于字符集、不含 iIoO0lL）；重置后旧会话 401。
  - 批量发链接：构造 4 类用户（合格/待审批/禁用/无邮箱）验证筛选计数。
  - 删除用户级联：Token/自定义订阅/版本文件无残留；邮箱释放后可重新注册。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：五重保护逐条、降级清显式 Token、禁用同事务清 Token + 递增凭据版本号、直接重置 8 位密码字符集（无易混淆）、重置后会话失效、待审批拒绝重置、批量发链接筛选（排除待审批/禁用/无邮箱）、删除用户级联。
  - 手动验证：管理员对另一用户直接重置密码 → 该用户旧会话失效、新密码可登录；禁用某用户 → 其 Token 全部 404；尝试禁用自己/删自己/删最后管理员被拒。

---

### Step 2：审批中心与邮件服务

**本 Step 完成后，系统应具备：待审批用户列表（批量通过/拒绝）、OIDC claims 快照展示、SMTP 邮件服务（密码重置/审批通知/欢迎邮件）的能力。**

- **目标：** 实现审批流与 SMTP 邮件发送。
- **前置条件：** Step 1（用户体系操作）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/internal/mail/`（业务层）**：SMTP 邮件服务。必须实现：
     - 配置存 `system_config`（键：`smtp_host`、`smtp_port`、`smtp_user`、`smtp_password`（敏感加密）、`smtp_from`、`smtp_tls`、`smtp_enabled_scopes`（JSON 数组：password_reset/approval_notify/welcome，默认全不启用））。
     - 发送函数（支持 TLS）；**SMTP 未配置或发送失败不阻断主流程**——业务操作仍完成，记录错误日志并返回「邮件发送失败」标记供前端提示。
     - **发送测试邮件**：发送到当前操作管理员邮箱，验证配置正确性，失败返回具体错误。
     - **接通 Build1/Step1 预留的邮件发送点**：密码重置邮件、批量密码设置链接邮件。

  2. **创建 `backend/internal/approval/`（业务层）**：审批服务。必须实现：
     - 待审批列表（后端分页）：来源、用户名、邮箱、申请时间（UTC 本地化）、OIDC claims 快照（如有）。
     - **通过**：激活账号（status→active）；**审批通过后清空 `oidc_claims` 列**。
     - **拒绝**：删除账号（邮箱释放可重新注册）；claims 随账号删除。
     - **批量通过**（勾选后头部按钮）。
     - **通知邮件**：通过/拒绝均按 `smtp_enabled_scopes.approval_notify` 发送；**拒绝通知在点击「拒绝」动作时触发发送**。
     - **欢迎邮件**：所有新用户首次激活时发送（审批通过、直接激活、白名单命中、管理员创建均发送；自注册提交时仅创建待审批不发）；**最小模板（纯文本）：站点名称 + 登录链接（前端地址）+ 按来源区分文案——本地创建/自注册：「您的账号已激活，请使用邮箱与密码登录」；OIDC：「您的账号已激活，请使用单点登录（OIDC）登录」（不携带凭据）**。

  3. **创建审批端点（接入层，`backend/internal/server/approval.go`）**：会话 + 管理员：
     - `GET /api/admin/approvals`（分页）、`POST /api/admin/approvals/:id/approve`、`POST /api/admin/approvals/:id/reject`、`POST /api/admin/approvals/batch_approve`。
     - `POST /api/admin/settings/smtp/test`（发送测试邮件，Step 3 面板复用，本 Step 建立）。

  4. **创建前端 `frontend/src/views/admin/ApprovalsView.vue`**（UI §5.6）：双态列表（后端分页）——来源标签、用户名、邮箱、申请时间（UTC 本地化）、OIDC claims 展开（`a-typography-paragraph` JSON 只读）；行首 Checkbox 批量选择；行操作**通过**/**拒绝**（ConfirmModal：账号将删除、邮箱释放，拒绝通知按配置发送）；头部「批量通过」；空状态 Empty「暂无待审批用户」。`frontend/src/api/approval.ts`。

- **参考代码/伪代码：**

  > 编写顺序：internal/mail（SMTP 发送 + 测试邮件）→ internal/approval（通过/拒绝/批量 + 通知邮件）→ server/approval.go → ApprovalsView.vue。复用：config 敏感加密（smtp_password 登记入 sensitiveKeys）、重置令牌服务（Build1 Step 7 的 sendMail 注入点在本 Step 接通）。

  **1. `backend/internal/mail/`（业务层：SMTP 邮件服务）**

  ```go
  const (
      KeyHost     = "smtp_host"
      KeyPort     = "smtp_port"
      KeyUser     = "smtp_user"
      KeyPassword = "smtp_password"        // 敏感加密（登记入 config.sensitiveKeys）
      KeyFrom     = "smtp_from"
      KeyTLS      = "smtp_tls"             // "true"/"false"
      KeyScopes   = "smtp_enabled_scopes"  // JSON 数组：password_reset/approval_notify/welcome，默认全不启用
  )

  type Service struct {
      cfg *config.Service
      log *slog.Logger
  }

  // Configured：SMTP 是否已配置（host+user+password 非空）
  func (s *Service) Configured(ctx context.Context) bool { /* 读三键判定 */ }

  // Send：SMTP 发送（支持 TLS）；未配置或发送失败不阻断主流程——返回 error 供业务层记录并携带标记
  func (s *Service) Send(ctx context.Context, to, subject, body string) error {
      if !s.Configured(ctx) {
          return errors.New("SMTP 未配置")
      }
      host, _ := s.cfg.Get(ctx, KeyHost)
      port, _ := s.cfg.Get(ctx, KeyPort)
      user, _ := s.cfg.Get(ctx, KeyUser)
      pass, _ := s.cfg.Get(ctx, KeyPassword) // 已自动解密
      from, _ := s.cfg.Get(ctx, KeyFrom)
      useTLS := s.cfg.GetBool(ctx, KeyTLS, true)
      // net/smtp：auth = smtp.PlainAuth("", user, pass, host)
      // useTLS → tls.Dial + smtp.NewClient；否则 smtp.Dial
      // 组装 RFC822 报文（From/To/Subject/Content-Type: text/plain; charset=utf-8）
      // 任一失败返回 error（不 panic、不阻断调用方）
      ...
  }

  // SendTest：发送测试邮件到当前操作管理员邮箱，失败返回具体错误（供面板展示）
  func (s *Service) SendTest(ctx context.Context, adminEmail string) error {
      return s.Send(ctx, adminEmail, "SMTP 配置测试", "这是一封测试邮件，收到即表示 SMTP 配置正确。")
  }

  // ScopeEnabled：某邮件类型是否在启用范围内（smtp_enabled_scopes JSON 数组）
  func (s *Service) ScopeEnabled(ctx context.Context, scope string) bool {
      scopes := s.cfg.GetJSONStringSlice(ctx, KeyScopes)
      return slices.Contains(scopes, scope)
  }

  // --- 业务邮件模板（纯文本，最小模板）---

  // SendWelcome：欢迎邮件——所有新用户首次激活时发送（审批通过/直接激活/白名单命中/管理员创建均发送）
  // 按来源区分文案：本地创建/自注册 → 「您的账号已激活，请使用邮箱与密码登录」；OIDC → 「您的账号已激活，请使用单点登录（OIDC）登录」（不携带凭据）
  func (s *Service) SendWelcome(ctx context.Context, to, siteName, loginURL, source string) error {
      if !s.ScopeEnabled(ctx, "welcome") {
          return nil
      }
      body := siteName + "\n\n"
      if source == "oidc" {
          body += "您的账号已激活，请使用单点登录（OIDC）登录：" + loginURL
      } else {
          body += "您的账号已激活，请使用邮箱与密码登录：" + loginURL
      }
      return s.Send(ctx, to, siteName+" 账号已激活", body)
  }

  // SendApprovalNotify：通过/拒绝通知（按 approval_notify scope）；拒绝通知在点击「拒绝」动作时触发
  func (s *Service) SendApprovalNotify(ctx context.Context, to, siteName string, approved bool) error {
      if !s.ScopeEnabled(ctx, "approval_notify") {
          return nil
      }
      var body string
      if approved {
          body = "您在 " + siteName + " 的账号已通过审批，现在可以登录。"
      } else {
          body = "您在 " + siteName + " 的账号申请未通过审批。"
      }
      return s.Send(ctx, to, siteName+" 审批通知", body)
  }

  // SendPasswordReset：密码重置邮件（接通 Build1 Step 7 预留的 sendMail 注入点）
  func (s *Service) SendPasswordReset(ctx context.Context, to, resetURL string) error {
      if !s.ScopeEnabled(ctx, "password_reset") {
          return nil
      }
      return s.Send(ctx, to, "密码重置", "请在 1 小时内使用以下链接重置密码（一次性）：\n"+resetURL)
  }
  ```

  **2. `backend/internal/approval/`（业务层：审批服务）**

  ```go
  type Service struct {
      store *store.Store
      mail  *mail.Service
      cfg   *config.Service
      log   *slog.Logger
  }

  type PendingUser struct {
      ID         int64
      Username   string
      Email      string
      Source     string // oidc/selfreg
      OidcClaims string // JSON 快照（可空）
      CreatedAt  time.Time
  }

  // List：待审批列表（后端分页，默认 20 条/页）
  func (s *Service) List(ctx context.Context, page, size int) ([]PendingUser, int64, error) {
      // SELECT ... FROM users WHERE status = 'pending' ORDER BY created_at LIMIT/OFFSET + COUNT
      ...
  }

  // Approve：激活账号（status→active）；审批通过后清空 oidc_claims 列；发欢迎邮件（按来源区分文案）
  func (s *Service) Approve(ctx context.Context, id int64) error {
      var email, source string
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := tx.QueryRowContext(ctx,
              `SELECT COALESCE(email,''), user_source FROM users WHERE id = ? AND status = 'pending'`, id).
              Scan(&email, &source); err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx,
              `UPDATE users SET status = 'active', oidc_claims = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
              return err
          }
          return nil
      })
      if err != nil {
          return err
      }
      // 欢迎邮件（事务提交后发送，失败不阻断——记日志）
      if email != "" {
          siteName, _ := s.cfg.Get(ctx, "site_name")
          loginURL, _ := s.cfg.Get(ctx, "frontend_url")
          if err := s.mail.SendWelcome(ctx, email, siteName, loginURL, source); err != nil {
              s.log.Warn("欢迎邮件发送失败", "user_id", id, "err", err)
          }
      }
      return nil
  }

  // Reject：删除账号（邮箱释放可重新注册）；claims 随账号删除；拒绝通知在动作时触发发送
  func (s *Service) Reject(ctx context.Context, id int64) error {
      var email string
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := tx.QueryRowContext(ctx,
              `SELECT COALESCE(email,'') FROM users WHERE id = ? AND status = 'pending'`, id).Scan(&email); err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil { // 账号删除、邮箱释放
              return err
          }
          return nil
      })
      if err != nil {
          return err
      }
      if email != "" {
          siteName, _ := s.cfg.Get(ctx, "site_name")
          if err := s.mail.SendApprovalNotify(ctx, email, siteName, false); err != nil {
              s.log.Warn("拒绝通知邮件发送失败", "user_id", id, "err", err)
          }
      }
      return nil
  }

  // BatchApprove：批量通过（逐个走 Approve 语义；单个失败不阻断其余，回执成功/失败计数）
  func (s *Service) BatchApprove(ctx context.Context, ids []int64) (succeeded, failed int, err error) {
      for _, id := range ids {
          if err := s.Approve(ctx, id); err != nil {
              failed++
              s.log.Warn("批量审批单项失败", "user_id", id, "err", err)
          } else {
              succeeded++
          }
      }
      return succeeded, failed, nil
  }
  ```

  **3. `backend/internal/server/approval.go`（审批端点；会话 + 管理员）**

  ```go
  type ApprovalHandler struct {
      approvalSvc *approval.Service
      mailSvc     *mail.Service
      users       *user.Service
  }

  func RegisterApprovalRoutes(engine *gin.Engine, h *ApprovalHandler, sessionMW, adminMW gin.HandlerFunc) {
      g := engine.Group("/api/admin/approvals", sessionMW, adminMW)
      g.GET("", h.list)                        // ?page=&size=
      g.POST("/:id/approve", h.approve)
      g.POST("/:id/reject", h.reject)
      g.POST("/batch_approve", h.batchApprove) // body: { ids: [] }
      // SMTP 测试邮件（Step 3 面板复用，本 Step 建立）
      engine.POST("/api/admin/settings/smtp/test", sessionMW, adminMW, h.smtpTest)
  }

  // smtpTest：发送到当前操作管理员邮箱，失败返回具体错误
  func (h *ApprovalHandler) smtpTest(c *gin.Context) {
      userID := c.GetInt64(auth.CtxUserID)
      u, err := h.users.GetByID(c.Request.Context(), userID)
      if err != nil || u == nil || u.Email == "" {
          server.Fail(c, http.StatusBadRequest, "当前账号无邮箱，无法发送测试邮件")
          return
      }
      if err := h.mailSvc.SendTest(c.Request.Context(), u.Email); err != nil {
          server.Fail(c, http.StatusBadRequest, "发送失败："+err.Error()) // 具体错误供面板展示
          return
      }
      server.OK(c, gin.H{"message": "测试邮件已发送"})
  }
  ```

  **4. 前端 `ApprovalsView.vue`（UI §5.6）**

  ```vue
  <script setup lang="ts">
  // 双态列表（后端分页）：来源标签（oidc/selfreg）、用户名、邮箱、申请时间（dayjs UTC→本地）、
  // OIDC claims 展开（a-typography-paragraph JSON 只读，无 claims 显示 —）
  // 行首 Checkbox 批量选择 → 头部「批量通过」（ConfirmModal 确认 N 条）
  // 行操作：通过 / 拒绝（ConfirmModal：账号将删除、邮箱释放，拒绝通知按配置发送）
  // 空状态：Empty「暂无待审批用户」
  const selected = ref<number[]>([])
  async function approveOne(u: PendingUser) { await approve(u.id); Notify.success('已通过'); await load() }
  async function rejectOne(u: PendingUser) { await reject(u.id); Notify.success('已拒绝'); await load() }
  async function batchApprove() {
    const res = await batchApproveApi(selected.value)
    Notify.success(`通过 ${res.succeeded} 条${res.failed ? `，失败 ${res.failed} 条` : ''}`)
    selected.value = []
    await load()
  }
  </script>
  ```

  ```ts
  // api/approval.ts
  export const listApprovals = (q: { page: number; size: number }) =>
    http.get<any, { list: PendingUser[]; total: number }>('/admin/approvals', { params: q })
  export const approve = (id: number) => http.post(`/admin/approvals/${id}/approve`)
  export const reject = (id: number) => http.post(`/admin/approvals/${id}/reject`)
  export const batchApproveApi = (ids: number[]) =>
    http.post<any, { succeeded: number; failed: number }>('/admin/approvals/batch_approve', { ids })
  ```

  **5. 单元测试要点（验收要求）**

  - 通过：status→active 且 oidc_claims 清空；欢迎邮件按来源区分文案（oidc vs selfreg 断言不同正文）。
  - 拒绝：账号删除、邮箱释放（同邮箱可重新注册）；claims 随账号删除。
  - 批量通过：部分失败回执计数正确。
  - SMTP 失败不阻断：注入发送失败 → Approve/Reject 仍成功，记 warn 日志。
  - 通知邮件按 scope：approval_notify 未启用时不发送（mock mail 断言未调用）。
  - ScopeEnabled：JSON 数组解析与包含判定。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：通过激活 + 清 claims、拒绝删号释放邮箱、批量通过、欢迎邮件按来源区分文案、SMTP 失败不阻断主流程、通知邮件按 scope 发送。
  - 手动验证：开启自注册审批（Step 3 配置，本 Step 可直接改库验证）→ 注册用户进待审批 → 审批通过可登录 / 拒绝后邮箱可再注册；配置 SMTP 后测试邮件送达。

---

### Step 3：面板配置（认证/验证码/SMTP/站点/限流/日志/公告/调试）

**本 Step 完成后，系统应具备：面板配置全分区（OIDC/白名单/本地认证/验证码/SMTP/站点信息/速率限制/日志级别/公告/调试模式/运行模式信息）的查看与修改能力。**

- **目标：** 实现面板配置各分区的读写与生效逻辑。
- **前置条件：** Step 2（SMTP 邮件服务）已完成并验收；Build1（OIDC/验证码/限流机制）已验收。
- **产出文件与操作：**

  1. **创建 `backend/internal/config/admin.go`（业务层）**：面板配置服务。必须实现各分区配置的读取与保存（敏感字段加密落库、回显脱敏；**验证码双密钥明文存储例外**）：
     - **OIDC 配置**：查看/修改提供商参数（Client Secret 脱敏回显）、测试连接（复用 Build1 OIDC 测试，本 Step 加管理员校验的专用端点）、**清空 OIDC 配置**（二次确认；受「本地登录与 OIDC 均不可用禁止保存」约束）、切换提供商类型（保留已填字段；切换前提示旧绑定失效）。**前端地址与回调地址启动时缓存（推导值为初始值，手动覆盖优先、重启后沿用不再被推导覆盖）、修改需重启生效，保存时提示「需重启容器生效」**。
     - **OIDC 启用规则**：审批开关（默认关闭）+ Role/Group 白名单（值列表 + 可配置声明路径，默认常见值下拉）+ **白名单为空时跳过校验直接激活，保存时显著警告「白名单为空，新用户将全部直接激活」**。
     - **本地认证**：允许本地登录（默认开）/允许自注册（默认关）/自注册审批（默认关）三开关；**本地登录关且 OIDC 不可用 → 禁止保存 + 显著警告**（防认证死锁）。
     - **验证码**：提供商 Radio + 双密钥 + 启用页面 Checkbox；**勾选未配密钥 → 校验拦截**；**双密钥明文落库、回显真实值（切换提供商/停用后保留可复用，R11-01）**。
     - **SMTP**：服务器/端口/账号/密码（加密）/发件人/TLS + 启用范围 Checkbox + 发送测试邮件（复用 Step 2）。
     - **站点信息**：名称（≤50 字符）+ ICON 上传（≤2MB，png/jpeg/webp/ico，排除 SVG/GIF；存 `/public/site/` 固定路径覆盖即更新；引用带版本参数 `?v=更新序号`）+ 删除恢复默认。
     - **速率限制**：登录/注册/找回/下载四个数字输入 + 当前 IP 解析策略展示（TRUST_PROXY 生效值）+ auto 模式伪造风险警示；**修改后立即生效**。
     - **日志级别**：debug/info/warn/error 单选；**运行时切换立即生效并持久化**（运行日志与实时日志流同步生效）。
     - **系统公告**：纯文本 ≤2000 字符（前端转义禁 HTML）；**醒目警告「公告内容接口公开可见，请勿写入内部信息」**。
     - **调试模式开关**：开启后 5xx 返回详细内部信息（生产默认关闭，状态持久化）。
     - **运行模式信息**：只读展示当前模式（Dev/Production）+ 「由启动环境变量决定，修改需重启容器」说明。

  2. **创建面板配置端点（接入层，`backend/internal/server/settings.go`）**：会话 + 管理员。按分区分端点（`GET/PUT /api/admin/settings/oidc`、`/oidc-rules`、`/local-auth`、`/captcha`、`/smtp`、`/site`、`/ratelimit`、`/log-level`、`/announcement`、`/debug`），或聚合 `GET /api/admin/settings` + 分区 `PUT`（执行时择一并全程统一）。**敏感字段 GET 返回脱敏值（验证码双密钥例外：明文回显），PUT 接受新值（空表示不修改）**。
     - **站点信息公开端点**：`GET /api/site/info`（站点名称 + ICON URL，**无需鉴权**，供登录页/Setup/首页渲染）。

  3. **创建前端 `frontend/src/views/admin/SettingsView.vue`**（UI §5.8）：左侧 `a-anchor` 锚点导航 + 右侧分区 `a-card` 堆叠（<768 锚点转顶部 Select 定位）。分区顺序与组件按 UI §5.8 表格：OIDC 配置 / OIDC 启用规则 / 本地认证 / 验证码 / SMTP / 站点信息 / **运行模式信息** / 速率限制 / 日志级别 / 系统公告 / 调试模式（配置导入导出/备份下载/危险操作区在 Step 4 补入本页）。`frontend/src/api/settings.ts`。

- **参考代码/伪代码：**

  > 端点风格已确认：按分区独立 `GET/PUT`（不聚合）。编写顺序：config/admin.go（分区读写 + 死锁防护 + 加密脱敏；验证码双密钥明文）→ server/settings.go（分区端点 + 公开站点信息）→ SettingsView.vue。复用：config 敏感加密、OIDC 测试连接（Build1 Step 6）、限流键（Build1 Step 7）。

  **1. `backend/internal/config/admin.go`（业务层：面板配置服务）**

  ```go
  // 敏感配置键登记（Build1 Step 1 预留的 sensitiveKeys 集合在本 Step 补全）
  //   oidc_client_secret / smtp_password（验证码双密钥明文存储，不登记，R11-01）
  // GET 回显一律脱敏（前端 a-input-password 占位显示「已配置」；验证码双密钥明文回显）；PUT 空串表示不修改
  //
  // 本文件 import 必须包含："unicode/utf8"（字符数校验用，站点名称/公告/导出密码长度按字符数计）；
  // 禁止使用第三方 utf8 包，禁止以 len(string)（字节数）代替字符数校验。

  type AdminService struct {
      cfg     *config.Service
      store   *store.Store
      oidcSvc *oidc.Service
      dataDir string // 数据卷根目录（站点 ICON 落盘用）
      log     *slog.Logger
  }

  // --- 通用读写辅助 ---

  // getMasked：敏感字段 GET 返回脱敏值（已配置 → "***"，未配置 → ""），禁止返回明文
  func (s *AdminService) getMasked(ctx context.Context, key string) string {
      v, _ := s.cfg.Get(ctx, key)
      if v == "" {
          return ""
      }
      return "***"
  }

  // setSensitive：PUT 接受新值（空串表示不修改）；非空时经 config.Set 自动加密落库
  func (s *AdminService) setSensitive(ctx context.Context, key, value string) error {
      if value == "" {
          return nil // 空 = 不修改
      }
      return s.cfg.Set(ctx, key, value)
  }

  // --- OIDC 配置分区 ---

  type OidcSettings struct {
      ProviderType string `json:"provider_type"`
      BaseURL      string `json:"base_url"`
      Realm        string `json:"realm"`
      ClientID     string `json:"client_id"`
      ClientSecret string `json:"client_secret"` // GET 脱敏；PUT 空=不修改
      FrontendURL  string `json:"frontend_url"`  // 启动时缓存，修改需重启生效
      CallbackURL  string `json:"callback_url"`  // 同上
  }

  // oidcUsable：判定 OIDC 是否「可用」（防认证死锁的核心判定）。
  // 可用条件：base_url 非空 且 client_id 非空 且（client_secret 非空 或 库内已有对应密文）。
  // 注意：in.ClientSecret 为 PUT 入参（空=不修改），故需同时检查库内已有密文（sensitiveExists）。
  func oidcUsable(in OidcSettings) bool {
      if in.BaseURL == "" || in.ClientID == "" {
          return false
      }
      if in.ClientSecret != "" {
          return true
      }
      return sensitiveExists(config.KeyOidcClientSecret) // 库内已有密文视为可用
  }

  // SaveOidc：保存 OIDC 参数；受「本地登录与 OIDC 均不可用禁止保存」约束（防认证死锁）
  func (s *AdminService) SaveOidc(ctx context.Context, in OidcSettings) error {
      // 死锁防护：本地登录关 + OIDC 不可用 → 禁止保存
      allowLocal := s.cfg.GetBool(ctx, config.KeyAllowLocalLogin, true)
      if !allowLocal && !oidcUsable(in) {
          return ErrAuthDeadlock // 「本地登录与 OIDC 均不可用，禁止保存」，接入层 400
      }
      // 各提供商参数独立存储（切换类型保留已填字段）；Secret 经 setSensitive 加密
      ...
      // frontend_url/callback_url：手动覆盖优先、重启后沿用不再被推导覆盖（启动时缓存语义）
      // 保存时返回 need_restart 标记供前端提示「需重启容器生效」
  }

  // ClearOidc：清空 OIDC 配置（二次确认由前端负责）；同样受死锁防护约束
  func (s *AdminService) ClearOidc(ctx context.Context) error {
      allowLocal := s.cfg.GetBool(ctx, config.KeyAllowLocalLogin, true)
      if !allowLocal {
          return ErrAuthDeadlock // 清空后 OIDC 不可用，若本地登录也关则死锁
      }
      // 清 oidc_configured / provider_type / 各提供商参数键（保留结构置空）
      ...
  }

  // --- OIDC 启用规则分区 ---

  // SaveOidcRules：审批开关 + Role/Group 白名单（值列表 + 可配置声明路径）
  // 白名单为空时跳过校验直接激活——保存时返回 warning 标记供前端显著警告「白名单为空，新用户将全部直接激活」
  func (s *AdminService) SaveOidcRules(ctx context.Context, approvalOn bool, whitelist WhitelistConfig) (warning string, err error) {
      // 存 oidc_approval / oidc_whitelist（JSON：{ role_claim_path, role_values, group_claim_path, group_values }）
      if approvalOn && whitelist.Empty() {
          warning = "白名单为空，新用户将全部直接激活"
      }
      ...
  }

  // --- 本地认证分区 ---

  // SaveLocalAuth：三开关（allow_local_login 默认开 / allow_selfreg 默认关 / selfreg_approval 默认关）
  // 本地登录关且 OIDC 不可用 → 禁止保存 + 显著警告（防认证死锁）
  func (s *AdminService) SaveLocalAuth(ctx context.Context, allowLocal, allowSelfReg, selfRegApproval bool) error {
      if !allowLocal && !s.oidcSvc.IsConfigured(ctx) {
          return ErrAuthDeadlock
      }
      // 存三键
      ...
  }

  // --- 验证码分区 ---

  // SaveCaptcha：提供商 + 双密钥 + 启用页面；勾选未配密钥 → 校验拦截
  func (s *AdminService) SaveCaptcha(ctx context.Context, provider, siteKey, secretKey string, pages []string) error {
      if provider != "off" && len(pages) > 0 {
          // 勾选了启用页面但密钥未配置（siteKey 或 secretKey 均空且库内也无）→ 拦截
          existingSite, _ := s.cfg.Get(ctx, captcha.KeySiteKey)
          existingSecret, _ := s.cfg.Get(ctx, captcha.KeySecretKey)
          if (siteKey == "" && existingSite == "") || (secretKey == "" && existingSecret == "") {
              return ErrCaptchaKeyMissing // 「启用验证码页面需先配置密钥」，接入层 400
          }
      }
      // 存 provider/site_key/secret_key(敏感)/pages(JSON)
      ...
  }

  // --- SMTP 分区 ---

  // SaveSMTP：host/port/user/password(敏感)/from/tls + 启用范围（JSON 数组）
  func (s *AdminService) SaveSMTP(ctx context.Context, in SMTPSettings) error { /* 逐键存；password 经 setSensitive */ }

  // --- 站点信息分区 ---

  const (
      MaxSiteNameLen = 50
      MaxIconSize    = 2 << 20 // 2MB
      siteIconDir    = "public/site"
  )

  var allowedIconExts = map[string]bool{"png": true, "jpeg": true, "jpg": true, "webp": true, "ico": true} // 排除 SVG/GIF

  // SaveSiteInfo：名称 ≤50 字符；ICON 上传 ≤2MB + 扩展名白名单；存 /public/site/ 固定路径覆盖即更新；
  // 引用带版本参数 ?v=更新序号（存 site_icon_version 配置键，每次上传递增）
  func (s *AdminService) SaveSiteInfo(ctx context.Context, name string, icon io.Reader, iconFilename string) error {
      if utf8.RuneCountInString(name) > MaxSiteNameLen {
          return fmt.Errorf("%w: 站点名称不超过 50 字符", ErrBadRequest)
      }
      if err := s.cfg.Set(ctx, "site_name", name); err != nil {
          return err
      }
      if icon != nil {
          ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filepath.Base(iconFilename))), ".")
          if !allowedIconExts[ext] {
              return fmt.Errorf("%w: ICON 仅支持 png/jpeg/webp/ico", ErrBadRequest)
          }
          data, err := io.ReadAll(io.LimitReader(icon, MaxIconSize+1))
          if err != nil {
              return fmt.Errorf("读取 ICON 失败: %w", err)
          }
          if len(data) > MaxIconSize {
              return fmt.Errorf("%w: ICON 超过 2MB 限制", ErrBadRequest)
          }
          full := filepath.Join(s.dataDir(), siteIconDir, "icon."+ext) // 固定路径覆盖即更新
          if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
              return err
          }
          if err := os.WriteFile(full, data, 0o644); err != nil {
              return fmt.Errorf("写入 ICON 失败: %w", err)
          }
          // 版本参数递增（前端引用 ?v=N 避免缓存旧图）
          ver := s.cfg.GetInt(ctx, "site_icon_version", 0) + 1
          if err := s.cfg.Set(ctx, "site_icon_version", strconv.Itoa(ver)); err != nil {
              return err
          }
          if err := s.cfg.Set(ctx, "site_icon_url", "/public/site/icon."+ext+"?v="+strconv.Itoa(ver)); err != nil {
              return err
          }
      }
      return nil
  }

  // DeleteSiteIcon：删除恢复默认（清 site_icon_url，前端回退默认 ICON）
  func (s *AdminService) DeleteSiteIcon(ctx context.Context) error { /* 删文件 + 清键 */ }

  // --- 速率限制分区 ---

  // SaveRateLimit：登录/注册/找回/下载四个数字输入；修改后立即生效（限流中间件每次请求读配置，Build1/2 已实现）
  // 返回当前 TRUST_PROXY 生效值供前端展示 + auto 模式伪造风险警示
  func (s *AdminService) SaveRateLimit(ctx context.Context, login, register, forgot, download int) error {
      for _, v := range []int{login, register, forgot, download} {
          if v <= 0 {
              return fmt.Errorf("%w: 限流值必须为正整数", ErrBadRequest)
          }
      }
      // 存 ratelimit_login / ratelimit_register / ratelimit_forgot / ratelimit_download
      ...
  }

  // --- 日志级别分区 ---

  // SetLogLevel：debug/info/warn/error 单选；运行时切换立即生效并持久化（运行日志与实时日志流同步生效）
  func (s *AdminService) SetLogLevel(ctx context.Context, level string) error {
      if !slices.Contains([]string{"debug", "info", "warn", "error"}, level) {
          return fmt.Errorf("%w: 日志级别无效", ErrBadRequest)
      }
      if err := s.cfg.Set(ctx, config.KeyLogLevel, level); err != nil {
          return err
      }
      // 运行时切换：slog.LevelVar 全局可调（log 包持有 *slog.LevelVar，SetLevel 立即生效）
      log.SetLevel(level)
      return nil
  }

  // --- 系统公告分区 ---

  const MaxAnnouncementLen = 2000

  // SaveAnnouncement：纯文本 ≤2000 字符（前端转义禁 HTML——Vue 默认转义，禁 v-html）
  func (s *AdminService) SaveAnnouncement(ctx context.Context, content string) error {
      if utf8.RuneCountInString(content) > MaxAnnouncementLen {
          return fmt.Errorf("%w: 公告不超过 2000 字符", ErrBadRequest)
      }
      return s.cfg.Set(ctx, "announcement", content)
  }

  // --- 调试模式分区 ---

  // SetDebug：开启后 5xx 返回详细内部信息（生产默认关闭，状态持久化）
  func (s *AdminService) SetDebug(ctx context.Context, on bool) error {
      return s.cfg.Set(ctx, "debug_mode", strconv.FormatBool(on))
  }
  // server.Fail 的 5xx 脱敏分支读取 debug_mode：开启时返回真实错误详情（Build1 Step 1 的 Fail 在此接通）
  ```

  **2. `backend/internal/server/settings.go`（面板配置端点；会话 + 管理员）**

  ```go
  type SettingsHandler struct{ adminCfg *config.AdminService }

  func RegisterSettingsRoutes(engine *gin.Engine, h *SettingsHandler, sessionMW, adminMW gin.HandlerFunc) {
      g := engine.Group("/api/admin/settings", sessionMW, adminMW)
      g.GET("/oidc", h.getOidc);        g.PUT("/oidc", h.saveOidc)
      g.DELETE("/oidc", h.clearOidc)    // 清空 OIDC 配置（二次确认前端负责）
      g.GET("/oidc-rules", h.getOidcRules); g.PUT("/oidc-rules", h.saveOidcRules)
      g.GET("/local-auth", h.getLocalAuth);  g.PUT("/local-auth", h.saveLocalAuth)
      g.GET("/captcha", h.getCaptcha);  g.PUT("/captcha", h.saveCaptcha)
      g.GET("/smtp", h.getSMTP);        g.PUT("/smtp", h.saveSMTP)
      g.GET("/site", h.getSite);        g.PUT("/site", h.saveSite)   // multipart（名称 + ICON 文件可选）
      g.DELETE("/site/icon", h.deleteSiteIcon)
      g.GET("/ratelimit", h.getRateLimit); g.PUT("/ratelimit", h.saveRateLimit)
      g.GET("/log-level", h.getLogLevel);  g.PUT("/log-level", h.saveLogLevel)
      g.GET("/announcement", h.getAnnouncement); g.PUT("/announcement", h.saveAnnouncement)
      g.GET("/debug", h.getDebug);      g.PUT("/debug", h.saveDebug)
      // OIDC 测试连接（管理员专用，复用 Build1 Step 6 TestConnection，加管理员校验）
      g.POST("/oidc/test", h.testOidc)

      // 站点信息公开端点（无需鉴权，供登录页/Setup/首页渲染）
      engine.GET("/api/site/info", h.siteInfo)
  }

  // GET 各分区：敏感字段返回脱敏值（"***"/""）；PUT 各分区：空串字段不修改
  // saveOidc/saveOidcRules 返回 need_restart / warning 标记供前端提示
  // siteInfo：返回 { site_name, icon_url }（公开，无敏感信息）
  ```

  **3. 前端 `SettingsView.vue`（UI §5.8）**

  ```vue
  <script setup lang="ts">
  // 左侧 a-anchor 锚点导航 + 右侧分区 a-card 堆叠；<768 锚点转顶部 Select 定位
  // 分区顺序（UI §5.8 表格）：OIDC 配置 / OIDC 启用规则 / 本地认证 / 验证码 / SMTP / 站点信息 /
  //   运行模式信息（只读）/ 速率限制 / 日志级别 / 系统公告 / 调试模式
  // （配置导入导出/备份下载/危险操作区在 Step 4 补入本页）

  // OIDC 配置：提供商 Radio（切换保留已填字段，切换前提示旧绑定失效）+ 参数表单（Secret a-input-password 脱敏回显）
  //   + 测试连接按钮（结果 a-alert）+ 清空配置（ConfirmModal 危险）
  //   + 前端地址/回调地址（保存后提示「需重启容器生效」，need_restart 标记驱动）
  // OIDC 启用规则：审批开关 + 白名单值列表（Tag 输入）+ 声明路径（默认常见值下拉）
  //   白名单为空保存 → 后端 warning 标记 → a-alert warning 显著提示
  // 本地认证：三开关；本地登录关且 OIDC 不可用 → 保存被 400 拦截展示死锁警告
  // 验证码：提供商 Radio + 双密钥 + 启用页面 Checkbox；勾选未配密钥被 400 拦截
  // SMTP：表单 + 启用范围 Checkbox + 发送测试邮件（复用 Step 2 端点）
  // 站点信息：名称输入（≤50 计数）+ ICON a-upload（≤2MB 前端预校验 + 格式白名单）+ 删除恢复默认
  // 速率限制：四个数字输入 + TRUST_PROXY 生效值展示 + auto 模式伪造风险 a-alert warning
  // 日志级别：a-radio-group 四档，保存即生效（无需重启提示）
  // 系统公告：a-textarea（≤2000 计数）+ a-alert warning「公告内容接口公开可见，请勿写入内部信息」
  // 调试模式：a-switch + 说明「开启后 5xx 返回详细内部信息，生产环境请保持关闭」
  // 运行模式信息：只读展示 Dev/Production + 「由启动环境变量决定，修改需重启容器」
  </script>
  ```

  ```ts
  // api/settings.ts（要点：按分区 GET/PUT）
  export const getOidc = () => http.get<any, OidcSettings>('/admin/settings/oidc')
  export const saveOidc = (data: OidcSettings) => http.put<any, { need_restart?: boolean }>('/admin/settings/oidc', data)
  export const clearOidc = () => http.delete('/admin/settings/oidc')
  export const saveOidcRules = (data: OidcRules) => http.put<any, { warning?: string }>('/admin/settings/oidc-rules', data)
  export const saveLocalAuth = (data: LocalAuthSettings) => http.put('/admin/settings/local-auth', data)
  export const saveCaptcha = (data: CaptchaSettings) => http.put('/admin/settings/captcha', data)
  export const saveSMTP = (data: SMTPSettings) => http.put('/admin/settings/smtp', data)
  export const saveSite = (form: FormData) => http.put('/admin/settings/site', form)
  export const deleteSiteIcon = () => http.delete('/admin/settings/site/icon')
  export const saveRateLimit = (data: RateLimitSettings) => http.put('/admin/settings/ratelimit', data)
  export const saveLogLevel = (level: string) => http.put('/admin/settings/log-level', { level })
  export const saveAnnouncement = (content: string) => http.put('/admin/settings/announcement', { content })
  export const saveDebug = (on: boolean) => http.put('/admin/settings/debug', { on })
  export const siteInfo = () => axios.get<any, { site_name: string; icon_url: string }>('/api/site/info') // 公开端点
  ```

  **4. 单元测试要点（验收要求）**

  - 敏感字段：Set 后库内为密文（非明文）；GET 返回 "***"；PUT 空串不修改。
  - 死锁防护：本地登录关 + OIDC 未配置 → SaveLocalAuth/SaveOidc/ClearOidc 均返回 ErrAuthDeadlock。
  - 白名单空警告：approvalOn=true 且白名单空 → 返回 warning 标记。
  - 验证码拦截：勾选页面但密钥缺失 → ErrCaptchaKeyMissing。
  - 日志级别：SetLogLevel 后 log 包 LevelVar 立即生效（debug 日志可输出）且配置持久化。
  - ICON：>2MB 拒绝、svg/gif 拒绝、png 通过且版本号递增。
  - 前端地址/回调地址：手动值保存后重启不被推导覆盖（启动缓存语义）。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：敏感字段加密落库 + 脱敏回显、本地登录与 OIDC 均不可用禁止保存、白名单空警告、验证码勾选未配密钥拦截、日志级别运行时切换持久化、ICON 格式/大小校验、前端地址/回调地址缓存语义。
  - 手动验证：修改各分区配置保存生效；改日志级别立即生效；上传 ICON 后 `/public/site/` 可访问且登录页显示；清空 OIDC 配置受约束。

---

### Step 4：危险操作区与配置导入导出、备份下载

**本 Step 完成后，系统应具备：一键清空所有数据（RESET 确认词）、配置导出（密码加密）、配置导入（整体覆盖 + IMPORT 确认词）、一键备份下载（tar.gz）的能力。**

- **目标：** 实现数据清理、配置迁移与备份三大运维能力。
- **前置条件：** Step 3（面板配置）已完成并验收；Build1/2 全部业务数据已存在。
- **产出文件与操作：**

  1. **创建 `backend/internal/dataclear/`（业务层）**：数据清理服务。必须实现：
     - **一键清空所有数据**（确认词 `RESET` + 二次确认）：清空全部业务数据（用户/用户组/订阅/平台/规则/分享/自定义订阅/各类 Token/访问日志/认证状态/密码重置令牌）+ 系统配置（含签名密钥、configured 标记）+ 数据文件（版本文件、`/public` 资源）→ 系统回到未配置状态，**无需重启**（前端路由守卫检测 configured=false 自动进 Setup）。
     - **执行顺序：先清库（事务）再删数据文件；文件删除失败记录错误日志并提示，不阻断回到 Setup 状态**。
     - **内存态复位**：SSE 连接与短期 Token、限流计数、实时日志缓冲同步重置；**地址启动缓存为全清特例——全清后回 Setup 重新推导写入新值，无需重启**。
     - 旧会话凭据因签名密钥轮换验签失败自然失效。

  2. **创建 `backend/internal/config/export.go`（业务层）**：配置导入导出服务（**仅 Production 模式提供**）。必须实现：
     - **导出**：设置导出密码（≥8 字符）→ Argon2id 派生密钥 + AES-256-GCM 加密整个配置文件 → 下载。内容：全部系统配置（含签名密钥与全部敏感密文）、站点信息（名称 + ICON base64 内嵌）、`format_version`、导出时间、来源运行模式；**不含业务数据与日志**。
     - **导入**（双入口：面板 + Setup）：上传文件 + 导出密码解密 → **校验格式与版本（`format_version` 不匹配仅警告不阻断；未知键忽略并警告），校验失败不做任何变更** → 确认词 `IMPORT` + 二次确认 → **事务内整体覆盖：先清空全部现有配置键再写入导出内容（严格整体覆盖，导出文件中不存在的键一并清除）**（含签名密钥替换与 ICON 写入 `/public/site/`；未配置状态导入时同事务创建预置默认组与默认平台）。**任一步失败整体回滚**。**导入上传文件不设大小上限**。
     - **导入后效果**：签名密钥替换 → 全部现有会话立即失效（含执行导入的管理员，前端清凭据跳登录）；含前端地址/回调地址时需重启生效；**明确操作顺序提示：导入完成 → 立即重启容器 → 再重新登录**；迁移核对警告（前端地址/回调地址已覆盖 + 整体覆盖已清除独有键）。
     - **Setup 导入端点仅未配置状态暴露，无会话保护——依赖导出密码（Argon2id 高成本）+ 按 IP 限流（同注册口径 5/min）防在线爆破**；导出/导入执行记 warn 级日志（不记录密码与文件内容）。

  3. **创建 `backend/internal/backup/`（业务层）**：备份服务。必须实现：
     - **备份下载**：先以 **SQLite backup API** 将当前数据库落为一致性快照（避免 WAL 未 checkpoint 数据遗漏）→ 将快照 + 全部版本文件 + `/public` 资源打包为 tar.gz 供下载；**备份过程不阻断正常服务**；备份文件含符号链接（保留「当前」指针，恢复后启动自检以 DB 为准重建）；**仅覆盖备份侧，恢复走手动解包**；仅管理员；执行记 warn 级日志。

  4. **创建运维端点（接入层，`backend/internal/server/settings_ops.go`）**：会话 + 管理员（Setup 导入端点除外）：
     - `POST /api/admin/settings/clear_all`（一键清空，body 含确认词 `RESET`）。
     - `POST /api/admin/settings/export`（导出，body 含导出密码）、`POST /api/admin/settings/import`（面板导入，含密码 + 确认词 `IMPORT`）、`POST /api/setup/import`（Setup 导入，未配置状态暴露，限流）。
     - `GET /api/admin/settings/backup`（备份下载 tar.gz）。

  5. **扩展前端 `SettingsView.vue`**：补入三个分区（UI §5.8）：
     - **配置导入/导出**（仅 Production 渲染）：导出（设置导出密码 ≥8 → 下载）/ 导入（上传 + 密码 + `IMPORT` 确认词 ConfirmModal；完成提示含整体覆盖与迁移核对双重警告 + 「请立即重启容器后再重新登录」）；Dev 模式显示说明文案。
     - **备份下载**：「下载备份」按钮（ConfirmModal 二次确认）→ 下载 tar.gz（进度反馈）；说明文案「含数据库 + 全部内容文件，恢复方式见部署文档」。
     - **危险操作区**（红色边框卡片）：「一键清空所有数据」→ ConfirmModal 内嵌 `RESET` 确认词输入 + 二次确认。

- **参考代码/伪代码：**

  > 编写顺序：internal/dataclear → config/export.go（Argon2id + AES-GCM）→ internal/backup（SQLite backup API + tar.gz）→ server/settings_ops.go → SettingsView 三分区。复用：BEGIN IMMEDIATE 事务助手、限流（Setup 导入端点）、slug 生成器（导入时预置数据）。

  **1. `backend/internal/dataclear/`（业务层：数据清理服务）**

  ```go
  const ConfirmWordReset = "RESET" // 一键清空确认词（固定，二次确认由前端负责）

  type Service struct {
      store   *store.Store
      dataDir string
      log     *slog.Logger
      // 内存态复位回调（由 main 装配注入）：SSE 连接与短期 Token、限流计数、实时日志缓冲
      resetRuntimeState func()
  }

  // ClearAll：一键清空所有数据——先清库（事务）再删数据文件；文件删除失败记日志并提示，不阻断回 Setup
  func (s *Service) ClearAll(ctx context.Context, confirmWord string) error {
      if confirmWord != ConfirmWordReset {
          return errors.New("确认词不正确") // 接入层 400
      }
      // 1) 清库：单事务删除全部业务数据 + 系统配置（含签名密钥、configured 标记）
      if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          tables := []string{
              "download_tokens", "share_tokens", "rule_tokens", "password_reset_tokens", "oidc_states",
              "access_logs", "versions", "subscription_group_rel", "group_selections",
              "custom_subscriptions", "share_subscriptions", "rules", "subscriptions",
              "users", "groups", "platforms", "system_config",
          }
          for _, t := range tables {
              if _, err := tx.ExecContext(ctx, "DELETE FROM "+t); err != nil {
                  return fmt.Errorf("清空表 %s 失败: %w", t, err)
              }
          }
          return nil
      }); err != nil {
          return err
      }
      // 2) 删数据文件（版本文件目录 + /public 资源）；失败记错误日志并提示，不阻断
      var fileErrs []string
      for _, dir := range []string{"contents", "public"} {
          if err := os.RemoveAll(filepath.Join(s.dataDir, dir)); err != nil {
              fileErrs = append(fileErrs, dir)
              s.log.Error("删除数据文件目录失败", "dir", dir, "err", err)
          }
      }
      // 3) 内存态复位：SSE 连接与短期 Token、限流计数、实时日志缓冲同步重置；
      //    地址启动缓存为全清特例——回 Setup 重新推导写入新值，无需重启（Design1 §3.4.8）
      if s.resetRuntimeState != nil {
          s.resetRuntimeState()
      }
      // 旧会话凭据因签名密钥轮换（configured 清除后重新 Setup 生成新密钥）验签失败自然失效
      s.log.Warn("一键清空所有数据已执行", "file_errors", fileErrs)
      return nil // 系统回到未配置状态，无需重启（前端守卫检测 configured=false 自动进 Setup）
  }
  ```

  **2. `backend/internal/config/export.go`（业务层：配置导入导出，仅 Production 模式）**

  ```go
  const (
      ConfirmWordImport = "IMPORT" // 配置导入确认词（固定）
      FormatVersion     = 1        // 导出格式版本
      MinExportPassword = 8        // 导出密码 ≥8 字符
  )

  type ExportService struct {
      store   *store.Store
      cfg     *config.Service
      setupSvc *setup.Service // 注入 Setup 服务，复用预置默认组/平台逻辑（Setup 导入分支）
      dataDir string
      mode    string // APP_MODE：导入导出仅 Production 提供
      log     *slog.Logger
  }

  // ExportPayload：导出内容（不含业务数据与日志）
  type ExportPayload struct {
      FormatVersion int               `json:"format_version"`
      ExportedAt    time.Time         `json:"exported_at"`
      SourceMode    string            `json:"source_mode"`
      Config        map[string]string `json:"config"`          // 全部系统配置（含签名密钥与全部敏感密文，原样导出）
      SiteName      string            `json:"site_name"`
      SiteIconB64   string            `json:"site_icon_base64"` // ICON base64 内嵌
  }

  // Export：导出密码（≥8）→ Argon2id 派生密钥 + AES-256-GCM 加密整个配置文件 → 返回密文供下载
  func (s *ExportService) Export(ctx context.Context, password string) ([]byte, error) {
      if s.mode != "prod" {
          return nil, errors.New("配置导出仅 Production 模式提供") // 接入层 403
      }
      if utf8.RuneCountInString(password) < MinExportPassword {
          return nil, fmt.Errorf("%w: 导出密码至少 8 字符", ErrBadRequest)
      }
      // 收集全部配置（含签名密钥与敏感密文——密文原样导出，导入侧原样落库）
      rows, err := s.store.DB().QueryContext(ctx, `SELECT key, value FROM system_config`)
      if err != nil {
          return nil, err
      }
      defer rows.Close()
      cfgMap := map[string]string{}
      for rows.Next() {
          var k, v string
          if err := rows.Scan(&k, &v); err != nil {
              return nil, err
          }
          cfgMap[k] = v
      }
      // ICON base64 内嵌（读 /public/site/icon.*）
      payload := ExportPayload{FormatVersion: FormatVersion, ExportedAt: time.Now(), SourceMode: s.mode, Config: cfgMap}
      // ... 读 ICON 文件 → base64 → payload.SiteIconB64
      plain, err := json.Marshal(payload)
      if err != nil {
          return nil, fmt.Errorf("序列化导出内容失败: %w", err)
      }
      // Argon2id 派生密钥 + AES-256-GCM 加密（salt 随机，输出格式：salt ‖ nonce ‖ 密文，base64）
      salt := make([]byte, 16)
      if _, err := rand.Read(salt); err != nil {
          return nil, fmt.Errorf("生成 salt 失败: %w", err)
      }
      key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32) // Argon2id 参数：time=1, memory=64MB, threads=4
      block, err := aes.NewCipher(key)
      if err != nil {
          return nil, err
      }
      gcm, err := cipher.NewGCM(block)
      if err != nil {
          return nil, err
      }
      nonce := make([]byte, gcm.NonceSize())
      if _, err := rand.Read(nonce); err != nil {
          return nil, err
      }
      out := append(salt, nonce...)
      out = gcm.Seal(out, nonce, plain, nil)
      s.log.Warn("配置导出已执行") // 记 warn 级日志（不记录密码与文件内容）
      return out, nil
  }

  // Import：上传文件 + 导出密码解密 → 校验格式与版本 → 事务内整体覆盖（严格整体覆盖语义）
  func (s *ExportService) Import(ctx context.Context, data []byte, password, confirmWord string, setupMode bool) error {
      if s.mode != "prod" {
          return errors.New("配置导入仅 Production 模式提供")
      }
      if confirmWord != ConfirmWordImport {
          return errors.New("确认词不正确")
      }
      // 解密（Argon2id + AES-GCM 逆过程）；失败返回「密码错误或文件损坏」
      payload, err := s.decrypt(data, password)
      if err != nil {
          return err
      }
      // 校验格式与版本：format_version 不匹配仅警告不阻断；未知键忽略并警告；校验失败不做任何变更
      if payload.FormatVersion != FormatVersion {
          s.log.Warn("导入配置 format_version 不匹配", "got", payload.FormatVersion, "want", FormatVersion)
      }
      // 事务内整体覆盖：先清空全部现有配置键再写入导出内容（导出文件中不存在的键一并清除）
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if _, err := tx.ExecContext(ctx, `DELETE FROM system_config`); err != nil { // 严格整体覆盖
              return err
          }
          for k, v := range payload.Config {
              if _, err := tx.ExecContext(ctx,
                  `INSERT INTO system_config (key, value) VALUES (?, ?)`, k, v); err != nil {
                  return fmt.Errorf("写入配置键 %s 失败: %w", k, err)
              }
          }
          // 未配置状态导入（Setup 分支）：同事务创建预置默认组与默认平台（预置数据为导入流程固定动作）
          if setupMode {
              if err := s.setupSvc.SeedPresetsTx(ctx, tx); err != nil { // 复用 setup 包预置逻辑（Build1 Step 5 抽取）
                  return err
              }
          }
          return nil
      })
      // 事务提交后：ICON 写入 /public/site/（base64 解码落盘）
      // 导入后效果：签名密钥替换 → 全部现有会话立即失效（含执行导入的管理员，前端清凭据跳登录）
      // 含前端地址/回调地址时需重启生效——返回提示「导入完成 → 立即重启容器 → 再重新登录」
  }
  ```

  **3. `backend/internal/backup/`（业务层：备份服务）**

  ```go
  type Service struct {
      store   *store.Store
      dataDir string
      dbFile  string
      log     *slog.Logger
  }

  // CreateBackup：SQLite backup API 一致性快照 + 全部版本文件 + /public 资源打包 tar.gz；不阻断正常服务
  func (s *Service) CreateBackup(ctx context.Context, w io.Writer) error {
      // 1) SQLite backup API 落一致性快照（避免 WAL 未 checkpoint 数据遗漏）
      snapshot := filepath.Join(os.TempDir(), fmt.Sprintf("backup-%d.db", time.Now().UnixMilli()))
      defer os.Remove(snapshot)
      if err := s.snapshotTo(snapshot); err != nil { // modernc.org/sqlite 的 backup 或 VACUUM INTO
          return fmt.Errorf("创建数据库快照失败: %w", err)
      }
      // 2) tar.gz 打包：快照 + contents/（版本文件，含符号链接保留「当前」指针）+ public/（站点资源/安装包）
      gz := gzip.NewWriter(w)
      tw := tar.NewWriter(gz)
      defer func() { _ = tw.Close(); _ = gz.Close() }()
      if err := addFileToTar(tw, snapshot, "app.db"); err != nil {
          return err
      }
      for _, dir := range []string{"contents", "public"} {
          root := filepath.Join(s.dataDir, dir)
          if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
              if err != nil {
                  return err
              }
              rel, _ := filepath.Rel(s.dataDir, path)
              return addToTar(tw, path, rel, info) // 符号链接以链接形式写入（恢复后启动自检以 DB 为准重建）
          }); err != nil {
              return fmt.Errorf("打包 %s 失败: %w", dir, err)
          }
      }
      s.log.Warn("备份下载已执行") // 记 warn 级日志
      return nil
  }

  // snapshotTo：一致性快照——首选 SQLite backup API（驱动层备份，Design1 §7.2）；
  // 驱动未直接暴露 backup 能力时以 VACUUM INTO 等价实现（同样产生一致性快照，避免 WAL 未 checkpoint 数据遗漏）
  //
  // 版本要求：VACUUM INTO 需 SQLite ≥ 3.27.0（2019-02 引入）；modernc.org/sqlite 须 ≥ v1.30.0（已验证支持）。
  // 若目标驱动版本不支持 VACUUM INTO（运行时报错），降级路径：先执行 PRAGMA wal_checkpoint(FULL) 将 WAL 落盘，
  // 再直接拷贝数据库主文件（此时无未 checkpoint 数据，拷贝即为一致快照）。
  func (s *Service) snapshotTo(dest string) error {
      // 优先尝试 modernc 驱动的 backup 能力（若暴露）；否则走 VACUUM INTO：
      if _, err := s.store.DB().Exec(`VACUUM INTO ?`, dest); err != nil {
          // 降级：WAL checkpoint 后拷贝主文件（保证一致性）
          if _, cerr := s.store.DB().Exec(`PRAGMA wal_checkpoint(FULL)`); cerr != nil {
              return fmt.Errorf("快照失败且 WAL checkpoint 降级失败: %w", cerr)
          }
          return copyFile(s.store.DBPath(), dest) // 拷贝数据库主文件
      }
      return nil
  }
  // 注：copyFile 为本包文件拷贝辅助函数；s.store.DBPath() 为数据层需提供的数据库主文件路径访问器（Build1 store 包补充）。
  ```

  **4. `backend/internal/server/settings_ops.go`（运维端点）**

  ```go
  type SettingsOpsHandler struct {
      clearSvc  *dataclear.Service
      exportSvc *config.ExportService
      backupSvc *backup.Service
      limiter   *ratelimit.Limiter
  }

  func RegisterSettingsOpsRoutes(engine *gin.Engine, h *SettingsOpsHandler, sessionMW, adminMW gin.HandlerFunc) {
      g := engine.Group("/api/admin/settings", sessionMW, adminMW)
      g.POST("/clear_all", h.clearAll)              // body: { confirm_word: "RESET" }
      g.POST("/export", h.export)                   // body: { password } → 返回加密文件下载
      g.POST("/import", h.importPanel)              // multipart 文件 + password + confirm_word=IMPORT
      g.GET("/backup", h.backup)                    // tar.gz 流式下载
      // Setup 导入端点：未配置状态暴露，无会话保护——依赖导出密码（Argon2id 高成本）+ 按 IP 限流（同注册口径 5/min）
      engine.POST("/api/setup/import",
          h.limiter.Middleware("setup_import", ratelimit.KeyRegister, 5), h.importSetup)
  }

  func (h *SettingsOpsHandler) clearAll(c *gin.Context) {
      var req struct{ ConfirmWord string `json:"confirm_word" binding:"required"` }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      if err := h.clearSvc.ClearAll(c.Request.Context(), req.ConfirmWord); err != nil {
          server.Fail(c, http.StatusBadRequest, err.Error())
          return
      }
      server.OK(c, gin.H{"message": "系统已重置，即将进入首次配置"})
  }

  func (h *SettingsOpsHandler) export(c *gin.Context) {
      var req struct{ Password string `json:"password" binding:"required"` }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      data, err := h.exportSvc.Export(c.Request.Context(), req.Password)
      if err != nil {
          server.Fail(c, http.StatusBadRequest, err.Error())
          return
      }
      c.Header("Content-Disposition", `attachment; filename="vpn-sub-config-`+time.Now().Format("20060102")+`.enc"`)
      c.Data(http.StatusOK, "application/octet-stream", data)
  }

  func (h *SettingsOpsHandler) backup(c *gin.Context) {
      c.Header("Content-Disposition", `attachment; filename="vpn-sub-backup-`+time.Now().Format("20060102-150405")+`.tar.gz"`)
      c.Header("Content-Type", "application/gzip")
      if err := h.backupSvc.CreateBackup(c.Request.Context(), c.Writer); err != nil {
          // 流式写出后无法再改状态码；打包前预检（快照）失败时此处仍返回 500
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
  }

  // importPanel / importSetup：multipart 文件（不设大小上限）+ password + confirm_word；
  // importSetup 额外校验未配置状态（已配置 409）；成功后返回「请立即重启容器后再重新登录」
  ```

  **5. 前端 SettingsView.vue 三分区扩展**

  ```vue
  <script setup lang="ts">
  // 配置导入/导出（仅 Production 渲染；Dev 显示说明文案「Dev 模式不提供配置导入导出」）
  const isProd = computed(() => system.status?.app_mode === 'prod')

  // 导出：设置导出密码（≥8）→ 下载（a-input-password + 确认密码一致性校验）
  async function doExport() {
    const blob = await exportConfig({ password: exportPwd.value }) // responseType: 'blob'
    downloadBlob(blob, `vpn-sub-config-${dayjs().format('YYYYMMDD')}.enc`)
  }

  // 导入：上传文件 + 密码 + IMPORT 确认词 ConfirmModal；
  // 完成提示：整体覆盖警告 + 迁移核对警告 + 「请立即重启容器后再重新登录」→ 清凭据跳登录
  async function doImport() {
    await importConfig(importForm)
    Modal.warning({
      title: '导入完成',
      content: '配置已整体覆盖（独有键已清除）。请立即重启容器后再重新登录。',
      onOk: () => { auth.logout(); router.push('/login') },
    })
  }

  // 备份下载：ConfirmModal 二次确认 → 下载 tar.gz（进度反馈）
  async function doBackup() {
    const blob = await downloadBackup() // responseType: 'blob'
    downloadBlob(blob, `vpn-sub-backup-${dayjs().format('YYYYMMDD-HHmmss')}.tar.gz`)
  }

  // 危险操作区（红色边框卡片）：「一键清空所有数据」→ ConfirmModal 内嵌 RESET 确认词输入 + 二次确认
  // ConfirmModal 组件已支持 confirmWord prop（Build1 Step 2 建立）
  async function doClearAll() {
    await clearAll({ confirm_word: 'RESET' })
    Notify.success('系统已重置')
    await system.fetchStatus(true) // configured=false → 守卫自动跳 /setup
    router.push('/setup')
  }
  </script>
  ```

  **6. 单元测试要点（验收要求）**

  - 一键清空：先清库后删文件；内存态复位回调被调用；configured=false；文件删除失败不阻断（注入只读目录验证）。
  - 导出/导入往返：Export → Import 解密后配置一致；错误密码解密失败。
  - 导入严格整体覆盖：库内独有键（导出文件中不存在）导入后被清除。
  - 导入失败回滚：注入中途失败 → 库内配置无变更。
  - 导入后会话失效：签名密钥替换后旧凭据验签失败 401。
  - 备份：tar.gz 解包含 app.db 快照 + contents/ + public/；符号链接保留。
  - Setup 导入限流：第 6 次/分钟 429。
  - 仅 Production：Dev 模式导出/导入返回 403。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：一键清空（先清库后删文件 + 内存态复位 + 回 Setup）、导出加密/导入解密往返、导入严格整体覆盖（清独有键）、导入失败回滚、导入后会话失效、备份 tar.gz 含快照 + 版本文件 + /public、Setup 导入限流。
  - 手动验证：导出配置 → 一键清空 → Setup 导入恢复配置；备份下载解包内容完整；一键清空后系统回 Setup。

---

### Step 5：日志查看（访问日志 + 实时日志流 SSE）

**本 Step 完成后，系统应具备：访问日志按日期查询/清空、实时日志流 SSE 推送（环形缓冲 + 短期 Token + 8 连接上限 + 级别过滤/暂停）的能力。**

- **目标：** 实现访问日志视图与实时日志流视图。
- **前置条件：** Build2（下载产生访问日志）、Build1（结构化日志管道）已验收。
- **产出文件与操作：**

  1. **创建 `backend/internal/log/access.go`（业务层）**：访问日志服务。必须实现：
     - 按日期范围查询（后端分页，默认 20 条/页）；展示下载类型、用户（可空）、平台、IP、状态、失败原因。
     - **资源标识记录口径**：显式/自定义 Token 下载记录订阅标识；无标识 Token 记录解析出的订阅标识，解析失败（unassigned）记录平台标识。
     - 90 天定时清理（Build2 已建后台任务，本 Step 确认接通）。
     - **清空日志**：删除全部访问日志记录（二次确认）。

  2. **创建 `backend/internal/log/stream.go`（业务层）**：实时日志流服务。必须实现：
     - 运行日志经统一管道同时输出 stdout 与**内存环形缓冲（最近 500 条）**。
     - **SSE 端点**：先推送缓冲历史，再实时推送增量；连接断开自动清理订阅。
     - **认证方式（关键约束）**：前端先通过会话凭据（Authorization Header）向 `POST /api/admin/logs/stream/token` 换取**一次性短期 Token（≥128 位）**，再将该 Token 作为 `?token=` 用于 EventSource URL `GET /api/admin/logs/stream?token=`；**Token 单次连接建立后即删除（严格一次性）**；断开重连重新换取；已换取未使用的 Token 按短 TTL（5 分钟）过期清理。响应 `no-cache`。
     - **并发订阅上限**：全局 8 连接（不按管理员计）；超出拒绝新连接并提示「连接数已达上限，请关闭其他日志页后重试」。
     - 仅管理员可订阅。

  3. **创建日志端点（接入层，`backend/internal/server/log.go`）**：会话 + 管理员：
     - `GET /api/admin/logs/access?from=&to=&page=&size=`（访问日志查询）、`POST /api/admin/logs/access/clear`（清空）。
     - `POST /api/admin/logs/stream/token`（换短期 Token）、`GET /api/admin/logs/stream?token=`（SSE 流）。

  4. **创建前端 `frontend/src/views/admin/LogsView.vue`**（UI §5.9）：`a-tabs` 双页签：
     - **访问日志**：日期 `a-range-picker` 筛选 + 双态列表（后端分页 20 条/页；下载类型、用户（可空 —）、平台、IP、状态 Badge、失败原因）+ 右上角「清空日志」（ConfirmModal）。
     - **实时日志流**：工具栏（级别过滤 Select + 暂停/继续 + 清屏——**暂停语义：前端停止渲染新日志，后端缓冲继续滚动；继续时从最新恢复渲染，暂停期间错过的条目不补推**）+ 日志容器（**终端风深色底，不随主题变化**；等宽字体；级别色块 info 白/warn 黄/error 红/debug 灰；滚动跟随；断线自动重连提示；连接数达上限提示）；SSE 短期 Token 换取过程对 UI 透明（前端自动先换 Token 再建 EventSource）。
     - `frontend/src/api/log.ts`。

- **参考代码/伪代码：**

  > 编写顺序：log/access.go（查询/清空）→ log/stream.go（环形缓冲 + SSE + 短期 Token + 8 连接上限）→ server/log.go → LogsView.vue。复用：Build1 log 包（slog Handler 链上追加环形缓冲 Handler）、统一列表包裹结构。

  **1. `backend/internal/log/access.go`（业务层：访问日志服务）**

  ```go
  type AccessService struct {
      store *store.Store
      log   *slog.Logger
  }

  type AccessLog struct {
      ID          int64
      UserID      int64  // 0 = 空（分享/规则下载）
      Username    string // 联查 users 填充（可空显示 —）
      IP          string
      DownloadType string
      Platform    string
      ResourceSlug string
      Status      string // success/fail
      FailReason  string
      CreatedAt   time.Time
  }

  // Query：按日期范围查询（后端分页，默认 20 条/页）
  func (s *AccessService) Query(ctx context.Context, from, to time.Time, page, size int) ([]AccessLog, int64, error) {
      if size <= 0 {
          size = 20
      }
      // SELECT a.*, u.username FROM access_logs a LEFT JOIN users u ON u.id = a.user_id
      // WHERE a.created_at BETWEEN ? AND ? ORDER BY a.created_at DESC LIMIT ? OFFSET ?
      // + COUNT(*) 总数
      ...
  }

  // Clear：清空全部访问日志记录（二次确认由前端 ConfirmModal 负责）
  func (s *AccessService) Clear(ctx context.Context) error {
      _, err := s.store.DB().ExecContext(ctx, `DELETE FROM access_logs`)
      return err
  }

  // 资源标识记录口径（Design1 §3.4.9，Build2 Step 4 写入时遵循）：
  //   显式/自定义 Token 下载 → 记录订阅标识；无标识 Token → 记录解析出的订阅标识；
  //   解析失败（unassigned）→ 记录平台标识
  // 90 天定时清理：Build2 Step 4 已建后台任务（cron.StartAccessLogCleanup），本 Step 确认接通
  ```

  **2. `backend/internal/log/stream.go`（业务层：实时日志流服务）**

  ```go
  const (
      RingBufferSize    = 500              // 环形缓冲最近 500 条
      MaxSSEConnections = 8                // 全局 8 连接（不按管理员计）
      StreamTokenTTL    = 5 * time.Minute  // 未使用短期 Token 5 分钟过期
  )

  // --- 环形缓冲 Handler（接入 slog 管道，与 stdout 输出并存）---

  // RingHandler：slog Handler 包装——记录同时输出 inner（stdout）与内存环形缓冲
  type RingHandler struct {
      inner slog.Handler
      buf   *RingBuffer
  }

  type RingBuffer struct {
      mu     sync.RWMutex
      entries []Entry // 环形：满后覆盖最旧
      subs   map[chan Entry]struct{} // 活跃订阅者
  }

  type Entry struct {
      Time    time.Time `json:"time"`
      Level   string    `json:"level"`
      Message string    `json:"message"`
      Attrs   string    `json:"attrs"` // 预格式化键值对串
  }

  func (b *RingBuffer) Append(e Entry) {
      b.mu.Lock()
      defer b.mu.Unlock()
      if len(b.entries) >= RingBufferSize {
          b.entries = b.entries[1:] // 覆盖最旧
          // 定期紧凑化：底层数组因切片左移无限增长，容量超过 4 倍缓冲大小时拷贝紧凑（防内存缓慢膨胀）
          if cap(b.entries) > RingBufferSize*4 {
              b.entries = append([]Entry(nil), b.entries...)
          }
      }
      b.entries = append(b.entries, e)
      for ch := range b.subs { // 广播给活跃订阅者（非阻塞）
          select {
          case ch <- e:
          default: // 订阅者消费慢则丢弃，防阻塞日志管道
          }
      }
  }

  // --- 短期 Token（一次性，≥128 位）---

  type StreamService struct {
      buf     *RingBuffer
      log     *slog.Logger
      mu      sync.Mutex
      tokens  map[string]time.Time // token → 过期时间（仅存内存）
      connCount int                 // 当前活跃 SSE 连接数
  }

  // IssueToken：换取一次性短期 Token（≥128 位；单次连接建立后即删；未使用 5 分钟 TTL）
  func (s *StreamService) IssueToken() (string, error) {
      b := make([]byte, 32) // 256 位
      if _, err := rand.Read(b); err != nil {
          return "", fmt.Errorf("生成短期 Token 失败: %w", err)
      }
      token := base64.RawURLEncoding.EncodeToString(b)
      s.mu.Lock()
      defer s.mu.Unlock()
      s.gcLocked() // 顺带清理过期 Token
      s.tokens[token] = time.Now().Add(StreamTokenTTL)
      return token, nil
  }

  // ConsumeToken：校验并用后即删（严格一次性）；过期视同无效
  func (s *StreamService) ConsumeToken(token string) bool {
      s.mu.Lock()
      defer s.mu.Unlock()
      exp, ok := s.tokens[token]
      if !ok || time.Now().After(exp) {
          return false
      }
      delete(s.tokens, token) // 用后即删
      return true
  }

  // --- SSE 连接管理（全局 8 连接上限）---

  // Subscribe：注册订阅者；超上限返回 false（接入层拒绝并提示）
  func (s *StreamService) Subscribe() (chan Entry, []Entry, bool) {
      s.mu.Lock()
      defer s.mu.Unlock()
      if s.connCount >= MaxSSEConnections {
          return nil, nil, false // 「连接数已达上限，请关闭其他日志页后重试」
      }
      s.connCount++
      ch := make(chan Entry, 64)
      s.buf.mu.Lock()
      s.buf.subs[ch] = struct{}{}
      history := append([]Entry(nil), s.buf.entries...) // 先推送缓冲历史
      s.buf.mu.Unlock()
      return ch, history, true
  }

  // Unsubscribe：连接断开自动清理订阅
  func (s *StreamService) Unsubscribe(ch chan Entry) {
      s.mu.Lock()
      defer s.mu.Unlock()
      s.buf.mu.Lock()
      delete(s.buf.subs, ch)
      s.buf.mu.Unlock()
      close(ch)
      s.connCount--
  }

  // Reset：一键清空数据时内存态复位（Build3 Step 4 的 resetRuntimeState 回调调用）
  func (s *StreamService) Reset() {
      s.mu.Lock()
      defer s.mu.Unlock()
      s.tokens = map[string]time.Time{}
      s.buf.mu.Lock()
      s.buf.entries = nil
      for ch := range s.buf.subs { // 断开全部活跃连接
          close(ch)
          delete(s.buf.subs, ch)
      }
      s.buf.mu.Unlock()
      s.connCount = 0
  }
  ```

  **3. `backend/internal/server/log.go`（日志端点；会话 + 管理员）**

  ```go
  type LogHandler struct {
      accessSvc *log.AccessService
      streamSvc *log.StreamService
  }

  func RegisterLogRoutes(engine *gin.Engine, h *LogHandler, sessionMW, adminMW gin.HandlerFunc) {
      g := engine.Group("/api/admin/logs", sessionMW, adminMW)
      g.GET("/access", h.queryAccess)              // ?from=&to=&page=&size=
      g.POST("/access/clear", h.clearAccess)
      g.POST("/stream/token", h.issueStreamToken)  // 换一次性短期 Token（会话凭据鉴权）
      g.GET("/stream", h.stream)                   // SSE：?token= 短期 Token（EventSource 无法带 Header）
  }

  // issueStreamToken：仅管理员（双中间件已保证）；返回 { token }
  func (h *LogHandler) issueStreamToken(c *gin.Context) {
      token, err := h.streamSvc.IssueToken()
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, gin.H{"token": token})
  }

  // stream：SSE 端点——先推缓冲历史，再实时推增量；连接断开自动清理
  func (h *LogHandler) stream(c *gin.Context) {
      if !h.streamSvc.ConsumeToken(c.Query("token")) { // 一次性校验
          server.Fail(c, http.StatusUnauthorized, "短期 Token 无效或已过期")
          return
      }
      ch, history, ok := h.streamSvc.Subscribe()
      if !ok {
          server.Fail(c, http.StatusTooManyRequests, "连接数已达上限，请关闭其他日志页后重试")
          return
      }
      defer h.streamSvc.Unsubscribe(ch)

      c.Header("Content-Type", "text/event-stream")
      c.Header("Cache-Control", "no-cache")
      c.Header("Connection", "keep-alive")
      // 先推历史
      for _, e := range history {
          writeSSE(c, e)
      }
      // 再推增量（断开检测：c.Writer.CloseNotify 或写入失败）
      flusher, _ := c.Writer.(http.Flusher)
      for {
          select {
          case e, ok := <-ch:
              if !ok {
                  return
              }
              writeSSE(c, e)
              if flusher != nil {
                  flusher.Flush()
              }
          case <-c.Request.Context().Done(): // 客户端断开
              return
          }
      }
  }

  func writeSSE(c *gin.Context, e log.Entry) {
      data, _ := json.Marshal(e)
      fmt.Fprintf(c.Writer, "data: %s\n\n", data) // SSE 协议：data: <json>\n\n
  }
  ```

  **4. 前端 `LogsView.vue`（UI §5.9）**

  ```vue
  <script setup lang="ts">
  // a-tabs 双页签：访问日志 / 实时日志流

  // --- 访问日志页签 ---
  // 日期 a-range-picker 筛选 + 双态列表（后端分页 20 条/页）：
  //   下载类型、用户（可空 —）、平台、IP、状态 Badge（成功绿/失败红）、失败原因
  // 右上角「清空日志」（ConfirmModal 危险）
  const range = ref<[Dayjs, Dayjs] | null>(null)
  async function loadAccess() {
    // GET /admin/logs/access?from=&to=&page=&size=
  }

  // --- 实时日志流页签 ---
  // 工具栏：级别过滤 Select（全部/debug/info/warn/error）+ 暂停/继续 + 清屏
  // 暂停语义：前端停止渲染新日志，后端缓冲继续滚动；继续时从最新恢复渲染，暂停期间错过的条目不补推
  const paused = ref(false)
  const levelFilter = ref('')
  const lines = ref<LogEntry[]>([])
  let eventSource: EventSource | null = null

  // SSE 短期 Token 换取过程对 UI 透明：前端自动先换 Token 再建 EventSource
  async function connect() {
    const { token } = await issueStreamToken() // POST /admin/logs/stream/token（Bearer 会话）
    eventSource = new EventSource(`/api/admin/logs/stream?token=${token}`)
    eventSource.onmessage = (ev) => {
      if (paused.value) return // 暂停：停止渲染（后端缓冲继续）
      const entry = JSON.parse(ev.data) as LogEntry
      if (levelFilter.value && entry.level !== levelFilter.value) return
      lines.value.push(entry)
      if (lines.value.length > 1000) lines.value = lines.value.slice(-1000) // 前端渲染上限
      nextTick(() => scrollToBottom()) // 滚动跟随
    }
    eventSource.onerror = () => {
      Notify.warning('日志流连接断开，正在重连…') // 断线自动重连提示
      eventSource?.close()
      setTimeout(connect, 3000) // 重连重新换取 Token
    }
  }
  onMounted(connect)
  onUnmounted(() => eventSource?.close())
  </script>

  <template>
    <!-- 实时日志流页签日志容器：终端风深色底，不随主题变化；等宽字体；级别色块 info 白/warn 黄/error 红/debug 灰 -->
    <div class="log-terminal bg-gray-900 text-gray-100 font-mono text-xs p-4 rounded h-[60vh] overflow-auto">
      <div v-for="(line, i) in lines" :key="i" :class="levelClass(line.level)">
        [{{ line.time }}] [{{ line.level.toUpperCase() }}] {{ line.message }} {{ line.attrs }}
      </div>
    </div>
  </template>

  <style scoped>
  /* 终端风深色底固定，不随暗色模式切换（UI §5.9） */
  .log-terminal { background-color: #1a1a1a !important; }
  .level-info { color: #e5e5e5; }
  .level-warn { color: #facc15; }
  .level-error { color: #f87171; }
  .level-debug { color: #6b7280; }
  </style>
  ```

  ```ts
  // api/log.ts
  export const queryAccessLogs = (q: { from: string; to: string; page: number; size: number }) =>
    http.get<any, { list: AccessLog[]; total: number }>('/admin/logs/access', { params: q })
  export const clearAccessLogs = () => http.post('/admin/logs/access/clear')
  export const issueStreamToken = () => http.post<any, { token: string }>('/admin/logs/stream/token')
  ```

  **5. 单元测试要点（验收要求）**

  - 访问日志：分页/日期筛选/清空；资源标识记录口径（显式记订阅标识、unassigned 记平台标识）。
  - 环形缓冲：写入 600 条 → 仅存最近 500 条。
  - SSE 短期 Token：一次性（ConsumeToken 后再用失败）；未使用 5 分钟 TTL 过期。
  - 8 连接上限：第 9 个 Subscribe 返回 false（429 + 提示文案）。
  - 断开清理：Unsubscribe 后 connCount 递减、订阅移除。
  - Reset：清空后 tokens/缓冲/连接全复位。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：访问日志分页/日期筛选/清空、环形缓冲 500 条、SSE 短期 Token 一次性（用后即删）、未使用 Token 5 分钟 TTL、8 连接上限拒绝、资源标识记录口径。
  - 手动验证：产生下载后访问日志可查；打开实时日志流先收历史再收增量；暂停后继续不补推；开 9 个连接第 9 个被拒；清空日志后列表为空。

---

### Step 6：应急恢复模式

**本 Step 完成后，系统应具备：手动（环境变量）/自动（数据库损坏/关键配置损坏）触发应急模式、一次性操作码防护、重置管理员密码、重新初始化（应急全清）的能力。**

- **目标：** 实现管理员密码救援与系统灾难恢复的最后手段。
- **前置条件：** Build1（配置存储/用户体系/日志）、Step 4（全清机制参考）已验收。
- **产出文件与操作：**

  1. **创建 `backend/internal/emergency/`（业务层）**：应急服务。必须实现：
     - **触发判定**（启动时）：
       - **手动触发**：环境变量 `RESET_ADMIN_PASSWORD` 已设置（值非空）→ 进入应急模式。
       - **自动触发**（仅两类）：数据库无法连接/损坏（含 `PRAGMA integrity_check` 不通过）；关键配置损坏（configured=true 但签名密钥缺失）。
       - 运行中检测到关键查询失败（如 `SQLITE_CORRUPT`）→ 进程退出（exit），由 compose restart 拉起后进入应急模式。
     - **一次性操作码（关键约束）**：应急模式启动时生成 **8 位大写字母+数字（去易混淆字符）** 操作码，**输出到运行日志（docker compose logs 可见）**；**仅存进程内存（不落库）**；应急页提交时与内存值比对；**严格一次性——每次提交（无论成功或失败）即消耗失效，系统立即重新生成新码并输出日志**；容器重启后操作码变更。操作码兼承担确认词职能，页面不展示任何词。
     - **能力分级（安全收紧）**：
       - **重置管理员密码**（仅环境变量触发且数据库可读时提供）：先输入操作码，校验通过后才展示管理员账号选择（名单不经验码不暴露）→ 选账号 → 设新密码（≥8 字符）→ 确认更新；**完成后进程退出（exit）**，由 compose restart 拉起；**成功后递增该管理员 credential_version**。**用户表为空时此能力不可用**；**仅对设有本地密码的账号有效（纯 OIDC 管理员重置后仍无法本地登录）**。
       - **重新初始化（应急全清）**：操作码 + 二次确认 → 一键清空所有数据回 Setup；**数据库可连接时以 SQL 清空；无法打开/损坏时降级为删除数据库文件 + 版本文件目录 + /public 资源后重建空库**；**完成后进程退出（exit）**。**自动触发的应急页仅保留此按钮**；**环境变量已设置但数据库不可读时，页面自动降级为仅重新初始化**。
     - **应急模式期间**：业务 API 与下载端点返回 503；系统状态、站点信息、`/assets` 静态资源、应急页路由正常服务；`/health` 返回 503。

  2. **创建应急端点（接入层，`backend/internal/server/emergency.go`）**：**仅在应急模式下暴露（正常运行时不注册路由）**：
     - `POST /api/emergency/verify`（校验操作码，通过返回管理员名单（若可重置））。
     - `POST /api/emergency/reset_password`（操作码 + user_id + 新密码）。
     - `POST /api/emergency/reinitialize`（操作码 + 二次确认）。
     - 应急模式为极低频救援场景且正常服务已暂停，不额外加限流/验证码。

  3. **扩展系统状态端点**：`emergency` 标记在应急模式返回 true（替换 Build1 的恒 false 占位），并返回触发原因与可用能力（是否可重置密码）。

  4. **创建前端 `frontend/src/views/EmergencyView.vue`**（UI §三）：
     - 独立全屏路由（前端据系统状态 `emergency` 标记强制路由至此）；居中卡片，顶部红色警示图标 + 「应急恢复模式」+ 触发原因说明（`a-alert error`）。
     - **交互序列**：① 操作码输入（8 位大字号等宽输入框 + 校验按钮；失败提示「操作码已失效，请重新从运行日志获取」）→ ② 校验通过后按能力分级渲染——**重置管理员密码**（`a-select` 管理员账号（验码前不暴露名单）→ 新密码 + 确认 → 确认；完成提示「进程即将退出重启，请移除 RESET_ADMIN_PASSWORD 环境变量后重启容器」）/ **重新初始化**（ConfirmModal + 二次确认 → 执行全清；完成提示同上）。
     - 本页不依赖业务 API（仅调应急端点与系统状态/站点信息端点）。
     - 接通路由守卫 `emergency`（Build1 已建框架，本 Step 验证：emergency=true 全站强制跳 `/emergency`，自身与静态资源除外）。

- **参考代码/伪代码：**

  > 编写顺序：internal/emergency（触发判定 + 一次性操作码 + 能力分级）→ server/emergency.go → status 扩展 → EmergencyView.vue。复用：dataclear 全清机制（Step 4）、user 包凭据版本号递增。

  **1. `backend/internal/emergency/`（业务层：应急服务）**

  ```go
  // 操作码字符集：8 位大写字母+数字，去易混淆字符（无 0/O/1/I/L 等）
  const opCodeCharset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

  type TriggerReason string

  const (
      TriggerNone      TriggerReason = ""              // 正常模式
      TriggerManual    TriggerReason = "manual"        // 环境变量手动触发
      TriggerDBCorrupt TriggerReason = "db_corrupt"    // 数据库无法连接/损坏
      TriggerKeyMissing TriggerReason = "key_missing"  // 关键配置损坏（configured=true 但签名密钥缺失）
  )

  type Service struct {
      store   *store.Store
      cfg     *config.Service
      dataDir string
      dbFile  string
      log     *slog.Logger
      mu      sync.Mutex
      opCode  string        // 一次性操作码（仅存进程内存，不落库）
      reason  TriggerReason
      dbReadable bool       // 数据库可读（决定能力分级）
  }

  // DetectTrigger：启动时触发判定（main 装配时调用，先于路由注册）
  func Detect(ctx context.Context, store *store.Store, cfg *config.Service, log *slog.Logger) (TriggerReason, bool) {
      // 1) 手动触发：环境变量 RESET_ADMIN_PASSWORD 已设置（值非空）——手动优先，但仍探测数据库可读性
      //    （决定能力分级：环境变量已设置但数据库不可读时，页面自动降级为仅重新初始化，Design1 §3.8）
      if os.Getenv("RESET_ADMIN_PASSWORD") != "" {
          return TriggerManual, probeDBReadable(ctx, store)
      }
      // 2) 自动触发（仅两类）：
      //    a) 数据库无法连接/损坏（含 PRAGMA integrity_check 不通过）
      if !probeDBReadable(ctx, store) {
          return TriggerDBCorrupt, false
      }
      //    b) 关键配置损坏：configured=true 但签名密钥缺失
      configured := cfg.GetBool(ctx, config.KeyConfigured, false)
      signingKey, _ := cfg.Get(ctx, config.KeySigningKey)
      if configured && signingKey == "" {
          return TriggerKeyMissing, true
      }
      return TriggerNone, true
  }

  // probeDBReadable：探测数据库可读性（连接可用 + PRAGMA integrity_check 通过）
  func probeDBReadable(ctx context.Context, store *store.Store) bool {
      var integrity string
      if err := store.DB().QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
          return false
      }
      return integrity == "ok"
  }

  // NewService：进入应急模式时构造——生成操作码并输出到运行日志（docker compose logs 可见）
  func NewService(reason TriggerReason, dbReadable bool, deps ...) *Service {
      s := &Service{reason: reason, dbReadable: dbReadable, ...}
      s.regenerateOpCode() // 初始生成
      return s
  }

  // regenerateOpCode：生成 8 位操作码并输出日志；每次提交（无论成败）即消耗失效并重新生成
  func (s *Service) regenerateOpCode() {
      b := make([]byte, 8)
      if _, err := rand.Read(b); err != nil {
          s.log.Error("生成应急操作码失败", "err", err)
          return
      }
      for i := range b {
          b[i] = opCodeCharset[int(b[i])%len(opCodeCharset)]
      }
      s.mu.Lock()
      s.opCode = string(b)
      s.mu.Unlock()
      // 输出到运行日志（docker compose logs 可见）；页面不展示任何词
      s.log.Warn("应急操作码已生成", "opcode", s.opCode, "reason", string(s.reason))
  }

  // VerifyOpCode：校验操作码——严格一次性：每次提交（无论成功或失败）即消耗失效，立即重新生成新码并输出日志
  func (s *Service) VerifyOpCode(input string) bool {
      s.mu.Lock()
      current := s.opCode
      s.mu.Unlock()
      ok := subtle.ConstantTimeCompare([]byte(input), []byte(current)) == 1 // 恒定时间比较防时序侧信道
      s.regenerateOpCode() // 无论成败均消耗并重新生成
      return ok
  }

  // --- 能力分级（安全收紧）---

  // CanResetPassword：重置管理员密码能力——仅环境变量触发且数据库可读时提供；用户表为空时不可用
  func (s *Service) CanResetPassword(ctx context.Context) bool {
      if s.reason != TriggerManual || !s.dbReadable {
          return false
      }
      var n int
      if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n == 0 {
          return false
      }
      return true
  }

  // ListAdmins：验码通过后才返回管理员名单（不经验码不暴露）
  func (s *Service) ListAdmins(ctx context.Context) ([]AdminOption, error) {
      // SELECT id, username, email FROM users WHERE role = 'admin' AND status = 'active'
      // 仅对设有本地密码的账号有效（纯 OIDC 管理员重置后仍无法本地登录）——返回 has_password 标记供前端标注
      ...
  }

  // ResetAdminPassword：选账号 → 设新密码（≥8）→ 确认更新；成功后递增 credential_version；完成后进程退出
  func (s *Service) ResetAdminPassword(ctx context.Context, userID int64, newPassword string) error {
      if err := auth.ValidatePassword(newPassword); err != nil {
          return err
      }
      hash, err := auth.HashPassword(newPassword)
      if err != nil {
          return err
      }
      if _, err := s.store.DB().ExecContext(ctx,
          `UPDATE users SET password_hash = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND role = 'admin'`,
          hash, userID); err != nil {
          return fmt.Errorf("重置管理员密码失败: %w", err)
      }
      s.log.Warn("应急重置管理员密码已执行", "user_id", userID)
      return nil
      // 接入层成功响应后进程退出（exit），由 compose restart 拉起
  }

  // Reinitialize：重新初始化（应急全清）——数据库可连接时以 SQL 清空；无法打开/损坏时降级为删文件重建
  func (s *Service) Reinitialize(ctx context.Context) error {
      if s.dbReadable {
          // 路径 A：SQL 清空（复用 dataclear 清库逻辑）+ 删数据文件
          if err := s.clearBySQL(ctx); err != nil {
              return err
          }
      } else {
          // 路径 B：数据库无法打开/损坏 → 删除数据库文件 + 版本文件目录 + /public 资源后重建空库
          if err := os.Remove(filepath.Join(s.dataDir, s.dbFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
              return fmt.Errorf("删除数据库文件失败: %w", err)
          }
          for _, dir := range []string{"contents", "public"} {
              if err := os.RemoveAll(filepath.Join(s.dataDir, dir)); err != nil {
                  s.log.Error("删除数据目录失败", "dir", dir, "err", err)
              }
          }
      }
      s.log.Warn("应急重新初始化已执行")
      return nil
      // 接入层成功响应后进程退出（exit），由 compose restart 拉起后进入正常 Setup
  }
  ```

  **2. `backend/internal/server/emergency.go`（应急端点；仅应急模式下注册路由）**

  ```go
  type EmergencyHandler struct{ emSvc *emergency.Service }

  // RegisterEmergencyRoutes：仅在应急模式下调用（正常运行时不注册——main 按 DetectTrigger 结果分支装配）
  func RegisterEmergencyRoutes(engine *gin.Engine, h *EmergencyHandler) {
      g := engine.Group("/api/emergency")
      g.POST("/verify", h.verify)              // 校验操作码，通过返回管理员名单（若可重置）
      g.POST("/reset_password", h.resetPassword) // 操作码 + user_id + 新密码
      g.POST("/reinitialize", h.reinitialize)    // 操作码 + 二次确认
      // 应急模式为极低频救援场景且正常服务已暂停，不额外加限流/验证码
  }

  func (h *EmergencyHandler) verify(c *gin.Context) {
      var req struct{ OpCode string `json:"op_code" binding:"required,len=8"` }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      if !h.emSvc.VerifyOpCode(req.OpCode) {
          server.Fail(c, http.StatusUnauthorized, "操作码已失效，请重新从运行日志获取")
          return
      }
      resp := gin.H{"can_reset_password": h.emSvc.CanResetPassword(c.Request.Context())}
      if resp["can_reset_password"] == true {
          admins, err := h.emSvc.ListAdmins(c.Request.Context()) // 验码通过后才返回名单
          if err != nil {
              server.Fail(c, http.StatusInternalServerError, err.Error())
              return
          }
          resp["admins"] = admins
      }
      server.OK(c, resp)
  }

  func (h *EmergencyHandler) resetPassword(c *gin.Context) {
      var req struct {
          OpCode      string `json:"op_code" binding:"required,len=8"`
          UserID      int64  `json:"user_id" binding:"required"`
          NewPassword string `json:"new_password" binding:"required,max=128"`
      }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      if !h.emSvc.VerifyOpCode(req.OpCode) {
          server.Fail(c, http.StatusUnauthorized, "操作码已失效，请重新从运行日志获取")
          return
      }
      if !h.emSvc.CanResetPassword(c.Request.Context()) {
          server.Fail(c, http.StatusForbidden, "当前环境不支持重置密码")
          return
      }
      if err := h.emSvc.ResetAdminPassword(c.Request.Context(), req.UserID, req.NewPassword); err != nil {
          server.Fail(c, http.StatusBadRequest, err.Error())
          return
      }
      server.OK(c, gin.H{"message": "密码已重置，进程即将退出重启，请移除 RESET_ADMIN_PASSWORD 环境变量后重启容器"})
      go func() { time.Sleep(500 * time.Millisecond); os.Exit(0) }() // 响应发出后进程退出，compose restart 拉起
  }

  func (h *EmergencyHandler) reinitialize(c *gin.Context) {
      var req struct {
          OpCode      string `json:"op_code" binding:"required,len=8"`
          ConfirmText string `json:"confirm" binding:"required"` // 二次确认非空校验（操作码已兼确认词职能，本字段仅防误触，不校验具体内容）
      }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      if !h.emSvc.VerifyOpCode(req.OpCode) {
          server.Fail(c, http.StatusUnauthorized, "操作码已失效，请重新从运行日志获取")
          return
      }
      if err := h.emSvc.Reinitialize(c.Request.Context()); err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, gin.H{"message": "系统已重新初始化，进程即将退出重启"})
      go func() { time.Sleep(500 * time.Millisecond); os.Exit(0) }()
  }
  ```

  **3. 应急模式装配与 503 拦截（main.go 分支 + server 中间件）**

  ```go
  // main.go：启动时 DetectTrigger → 应急模式分支装配（不注册业务路由）
  reason, dbReadable := emergency.Detect(ctx, st, cfg, logger)
  if reason != emergency.TriggerNone {
      emSvc := emergency.NewService(reason, dbReadable, ...)
      // 应急模式 server：仅注册 系统状态/站点信息/应急端点/静态资源（/assets、/public、SPA 回退）
      srv := server.NewEmergency(emSvc, ...)
      // 业务 API 与下载端点返回 503（中间件拦截非白名单路径）
      // /health 返回 503（Build1 预留注释在本 Step 接通）
  }

  // server/emergency_gate.go：应急模式 503 拦截中间件
  func emergencyGate() gin.HandlerFunc {
      return func(c *gin.Context) {
          path := c.Request.URL.Path
          // 白名单：/api/system/status、/api/site/info、/api/emergency/*、/assets/*、/public/*、/health（返 503）、SPA 回退
          if isEmergencyAllowed(path) {
              c.Next()
              return
          }
          c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "系统处于应急恢复模式"})
          c.Abort()
      }
  }
  ```

  **4. status 端点扩展**：`emergency` 标记在应急模式返回 true（替换 Build1 恒 false 占位），并返回 `emergency_reason`（manual/db_corrupt/key_missing）与 `can_reset_password`（可用能力）。

  **5. 前端 `EmergencyView.vue`（UI §三）**

  ```vue
  <script setup lang="ts">
  // 独立全屏路由（前端据系统状态 emergency 标记强制路由至此——Build1 Step 2 守卫已建框架，本 Step 接通）
  // 居中卡片：顶部红色警示图标 + 「应急恢复模式」+ 触发原因说明（a-alert error）
  const reason = computed(() => system.status?.emergency_reason)
  const reasonText = computed(() => ({
    manual: '管理员手动触发（RESET_ADMIN_PASSWORD 环境变量）',
    db_corrupt: '数据库无法连接或已损坏',
    key_missing: '关键配置损坏（签名密钥缺失）',
  }[reason.value ?? ''] ?? '未知原因'))

  // 交互序列：
  // ① 操作码输入（8 位大字号等宽输入框 + 校验按钮；失败提示「操作码已失效，请重新从运行日志获取」）
  const opCode = ref('')
  const verified = ref(false)
  const canReset = ref(false)
  const admins = ref<AdminOption[]>([])
  async function verify() {
    try {
      const res = await emergencyVerify({ op_code: opCode.value })
      verified.value = true
      canReset.value = res.can_reset_password
      admins.value = res.admins ?? []
    } catch (err) {
      Notify.error((err as Error).message) // 「操作码已失效…」
      opCode.value = '' // 操作码已消耗，需重新从日志获取
    }
  }

  // ② 校验通过后按能力分级渲染：
  //   重置管理员密码（a-select 管理员账号（验码前不暴露名单）→ 新密码 + 确认 → 确认；
  //     完成提示「进程即将退出重启，请移除 RESET_ADMIN_PASSWORD 环境变量后重启容器」）
  //   重新初始化（ConfirmModal + 二次确认 → 执行全清；完成提示同上）
  //   自动触发（db_corrupt/key_missing）或库不可读：仅保留「重新初始化」按钮
  async function doResetPassword() {
    await emergencyResetPassword({ op_code: opCode.value, user_id: selectedAdmin.value, new_password: newPwd.value })
    Modal.success({ title: '密码已重置', content: '进程即将退出重启，请移除 RESET_ADMIN_PASSWORD 环境变量后重启容器' })
  }
  async function doReinitialize() {
    await emergencyReinitialize({ op_code: opCode.value, confirm: '确认重新初始化' })
    Modal.success({ title: '系统已重新初始化', content: '进程即将退出重启，重启后将进入首次配置' })
  }
  // 本页不依赖业务 API（仅调应急端点与系统状态/站点信息端点）
  </script>
  ```

  ```ts
  // api/emergency.ts（独立 axios 实例，不走 401 拦截器——应急模式无会话）
  import axios from 'axios'
  const emergencyHttp = axios.create({ baseURL: '/api/emergency' })
  export const emergencyVerify = (data: { op_code: string }) =>
    emergencyHttp.post<any, { can_reset_password: boolean; admins?: AdminOption[] }>('/verify', data).then(r => r.data.data)
  export const emergencyResetPassword = (data: { op_code: string; user_id: number; new_password: string }) =>
    emergencyHttp.post('/reset_password', data)
  export const emergencyReinitialize = (data: { op_code: string; confirm: string }) =>
    emergencyHttp.post('/reinitialize', data)
  ```

  **6. 单元测试要点（验收要求）**

  - 触发判定：手动（环境变量）/自动（integrity_check 失败 / configured=true 但签名密钥缺失）三分支。
  - 操作码一次性：提交（无论成败）即消耗重新生成（旧码再用失败）；8 位字符集断言。
  - 能力分级：自动触发仅重初始化（CanResetPassword=false）；库不可读降级；用户表为空不可重置。
  - 重置密码：credential_version 递增（旧会话失效）；非 admin 目标拒绝。
  - 重初始化两路径：SQL 清空（库可读）/删文件重建（库损坏）。
  - 应急期间业务 API 503：emergencyGate 拦截非白名单路径；/health 返 503。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  RESET_ADMIN_PASSWORD=1 go run ./cmd/server   # 手动验证应急模式
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：手动/自动触发判定、操作码一次性（提交即消耗重新生成）、能力分级（自动触发仅重初始化；库不可读降级）、重置密码递增凭据版本号、重初始化两路径（SQL 清空/删文件重建）、应急期间业务 API 503。
  - 手动验证：设 `RESET_ADMIN_PASSWORD` 重启 → 进应急页 → 从 logs 取操作码 → 重置管理员密码 → 进程退出重启 → 移除变量恢复；测试重初始化回 Setup。

---

## 五、候选构建项（待用户决策，逐项转 Step）

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| 1 | 模块化订阅装配（完整功能） | 管理员勾选模块/子模块动态拼接双语法订阅入订阅池；当前仅预留入口（Build2 Step 7），详细设计见 DesignOnHold.md | Design1 §3.9/九 |
| 2 | 记住我设备管理 | 个人中心查看/撤销「记住我」会话，远期实现 | Design1 §九 |
| 3 | 多版本下载支持 | 规则页允许用户选择历史版本下载 | Design1 §九 |
| 4 | 管理操作审计 | 记录管理操作的操作用户与时间，可追溯 | Design1 §九 |

> 候选转 Step 流程：用户确认 → 新建 BuildN 文档追加 Step（含目标/前置/参考代码/验收命令）→ 按序执行。

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-07 | 初始版本：管理面补全与运维能力（6 Step），承接 Build1/Build2 |
| v1.1 | 2026-08-09 | 同步 Build2 修复：① 执行约束第 5 条补充列表统一 ListData 包裹 + 前端 api 层解包约定（R02-01）；② Step 1 注明 customs 版本路由返回按钮 backPath 待改 /admin/users（R04-01）；③ Step 1 列表 api 分页包裹与全量解包区分注释 |
| v1.2 | 2026-08-09 | 全部 6 个 Step 验收通过（✅ 2026-08-09）：后端 go build/vet/test 全绿（25 包）；前端 npm run build/test 通过（20 用例）；应急模式手动验证通过（RESET_ADMIN_PASSWORD 触发、操作码一次性、业务 API 503、SPA 回退正常） |
| v1.3 | 2026-08-09 | 全量审查补漏（R05-01/02）：① Setup 页补「导入已有配置」卡片（仅 Production 渲染，配置导入双入口闭环，`api/settings.ts` 新增 `setupImportConfig`）；② server_test 补 `TestSetupImportRateLimit` 端点级限流测试（前 5 次放行、第 6 次 429）；复验后端 25 包全绿 + 前端构建/20 用例全绿 |
| v1.4 | 2026-08-11 | 验证码缺陷修复（R11-01/02，静态验证通过，运行测试待部署后手动）：① R11-01 验证码双密钥改明文存储——移除 `captcha_secret_key` 敏感登记，`GetCaptcha` 明文回显、`SaveCaptcha` 明文落库，前端 placeholder 同步（切换提供商/停用后密钥保留可复用，不再被「***」回显覆盖损坏）；② R11-02 验证码启用后登录/注册/找回密码永远 400——中间件与处理器统一改用 gin `ShouldBindBodyWithJSON`（body 缓存进 context 复用，`ShouldBindJSON` 直接消费 body 不缓存）；③ 附带修复 Turnstile 组件永不渲染（Turnstile api.js 不支持 `onload` 参数，改在 script.onload 渲染）；④ 补充回归测试：`TestMiddlewareBodyReuse`（mock siteverify，无真实网络）、`TestCaptchaPlainStorage`（明文回显 + 停用重开复用） |
