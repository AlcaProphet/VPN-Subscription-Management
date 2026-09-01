# Build2.md — 订阅核心与用户端价值链（当前构建方案）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第二轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则），承接已归档的 [Build1.md](Build1.md)（第一轮：工程骨架与认证闭环，须全部验收通过后本轮方可启动）。
> - 设计基线：[Design1.md](../Design/Design1.md)（与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design1-UI.md](../Design/Design1-UI.md)
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 前置轮次：Build1.md（已归档）；后续轮次：Build3.md（管理面与运维）
>
> **里程碑：本 Build 全部 Step 完成后，系统必须具备订阅上传、版本管理、用户组分发、三类下载 Token、用户端一键导入的完整价值链，以及自定义/分享/规则周边资源能力。**

---

## 执行约束（执行 AI 必须严格遵守）

1. **严格按 Step 顺序执行**，完成一个 Step 并验收通过后，方可进入下一个 Step。**禁止跳步、禁止并行执行多个 Step、禁止自行合并步骤**。
2. **每个 Step 完成后必须运行该 Step 的「验证命令」**，全部通过才算完成；任一命令失败必须修复后重验，禁止带错进入下一 Step。
3. **遇到模糊、歧义或设计文档未覆盖的细节，必须停止并向用户询问，禁止自行假设或自由发挥**。本文件未明确的技术选型，以 Design1.md §5.1 为准。
4. **禁止引入设计文档未提及的框架、库或架构模式**（技术栈同 Build1 执行约束第 4 条）。
5. **Build1 已建立的机制必须复用，禁止重复实现**：构造注入装配、配置存储、结构化日志（含 token 脱敏）、统一响应结构、会话/角色中间件、标识自动生成器、迁移框架、`BEGIN IMMEDIATE` 事务助手、前端路由守卫/Axios 拦截器/通用组件（ConfirmModal/TriStateList/Notify）。
6. **关键设计参数必须严格按下表取值**，与 Design1.md 保持一致，禁止修改：

| 参数 | 取值 | 出处 |
|------|------|------|
| 版本保留上限 | 每份资源最多 5 个版本（含当前激活版本） | Design1 §4.1 |
| 版本号规则 | 已有最大编号 + 1（禁止用列表长度 + 1） | Design1 §4.1 |
| 版本删除约束 | 不可删最后一个；不可删当前激活版本（须先切换） | Design1 §4.1 |
| 上传/编辑后行为 | 自动创建新版本号并切换为当前 | Design1 §4.1 |
| 版本事务 | 版本号计算与列表更新必须在单个 `BEGIN IMMEDIATE` 事务 + 库级写锁内 | Design1 §4.1 |
| 当前指针切换 | 原子替换（临时指针 + rename）；以 DB 记录为准，启动自检重建 symlink | Design1 §4.1 |
| 下载 Token 熵 | ≥128 位加密安全随机值 | Design1 §4.2 |
| 用户 Token 复用键 | 无标识 user+platform；自定义 user+platform+custom_sub_id；显式 user+platform+subscription_id | Design1 §4.2 |
| 自动生成标识 | 类型前缀 + 8 位随机短码（subscription-/share-/rule-/custom-，2026-08-09 变更：四类资源全部自动生成，见 Issue1 R03-02），冲突重试最多 3 次 | Design1 §2.2 |
| 下载响应 | 始终 `text/plain`；禁缓存头（no-store 等） | Design1 §4.3 |
| 无效/过期 Token | 统一 404（不泄露资源存在性），日志记 `token_invalid` | Design1 §4.3 |
| 下载业务错误 | HTTP 200 + 纯文本注释块（如 `# error: unassigned`），text/plain + 禁缓存头 | Design1 §4.3 |
| 下载限流 | 默认 20/min（按 IP，固定窗口） | Design1 §3.4.8 |
| 内容文件上传上限 | 订阅/规则/自定义/分享 ≤50MB；安装包 ≤300MB（流式落盘） | Design1 §6.3 |
| 安装包 | 公开下载、可缓存、不限流、不记访问日志；覆盖时删旧时间戳文件 | Design1 §4.7 |
| 访问日志保留 | 90 天自动清理 | Design1 §3.4.9 |

7. **注释使用中文**；所有 error 必须处理；禁止 `fmt.Println` 调试输出。
8. **职责分层**（接入层/业务层/数据层）与**按业务域拆分处理器文件**的约束同 Build1。
9. **多资源共用机制必须提取共享组件**：四类内容资源（订阅/规则/自定义/分享）的版本管理必须共用同一「版本管理事务组件」，禁止复制粘贴重复实现（AGENTS §八-2）。
10. **错误码约定**同 Build1 执行约束第 11 条；**下载类端点的业务错误返回 HTTP 200 + 纯文本注释块，不返回 JSON/HTML；无效/过期 Token 统一返回 404**。

---

## TODOLIST CheckList（构建进度核对）

> 执行 AI 必须逐个完成并勾选。状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

- ✅ Step 1：平台管理（含安装包分发）
- ✅ Step 2：通用版本管理事务组件与订阅池
- ✅ Step 3：用户组与订阅分发机制
- ✅ Step 4：下载 Token 体系与统一下载端点
- ✅ Step 5：自定义订阅与分享订阅
- ✅ Step 6：规则管理与用户端页面
- ✅ Step 7：管理面板布局、订阅装配预留与全局收尾

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 平台管理（含安装包分发） | Design1 §3.4.4/4.7/6.3 | ✅ 验收通过 |
| 2 | 通用版本管理事务组件与订阅池 | Design1 §4.1/3.4.1/2.2 | ✅ 验收通过 |
| 3 | 用户组与订阅分发机制 | Design1 §2.2/3.4.2/4.4 | ✅ 验收通过 |
| 4 | 下载 Token 体系与统一下载端点 | Design1 §4.2/4.3/6.4 | ✅ 验收通过 |
| 5 | 自定义订阅与分享订阅 | Design1 §2.3/2.4/3.4.3 | ✅ 验收通过 |
| 6 | 规则管理与用户端页面 | Design1 §3.4.7/3.3/3.5/3.6 | ✅ 验收通过 |
| 7 | 管理面板布局、订阅装配预留与全局收尾 | Design1 §3.4/3.9/3.7，UI §五/七 | ✅ 验收通过 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件（核心） | 要点 |
|------|----------------|------|
| 1 | `backend/migrations/1001_platforms.sql`、`backend/internal/platform/...`、`backend/internal/server/platform.go`、`frontend/src/views/admin/PlatformsView.vue`、`PlatformEditView.vue` | 平台 CRUD + scheme 排序 + 附加头 + 安装包双来源 |
| 2 | `backend/migrations/1002_subscriptions_versions.sql`、`backend/internal/version/...`（共享组件）、`backend/internal/subscription/...`、`frontend/src/views/admin/SubscriptionsView.vue`、`VersionManageView.vue` | 版本事务组件 + 订阅池 CRUD |
| 3 | `backend/migrations/1003_groups.sql`、`backend/internal/group/...`、`backend/internal/server/group.go`、`frontend/src/views/admin/GroupsView.vue` | 组 CRUD + 每平台选定 + 删组迁入 |
| 4 | `backend/migrations/1004_tokens.sql`、`backend/internal/token/...`、`backend/internal/download/...`、`backend/internal/server/download.go` | 三态 Token + 统一下载 + 限流 + 访问日志 |
| 5 | `backend/migrations/1005_custom_share.sql`、`backend/internal/{custom,share}/...`、`frontend/src/views/admin/SharesView.vue` | 自定义覆盖 + 分享（吊销矩阵） |
| 6 | `backend/migrations/1006_rules.sql`、`backend/internal/rule/...`、`frontend/src/views/{HomeView,RulesView,ProfileView}.vue`、`frontend/src/views/admin/RulesView.vue` | 规则 + 用户端三页 |
| 7 | `frontend/src/layouts/AdminLayout.vue`、`frontend/src/views/admin/AssemblyView.vue`、路由/菜单收尾 | 面板布局 + 装配占位 + 全局体验 |

---

## 三、构建顺序依赖图

```
Step 1（平台）──▶ Step 2（订阅池依赖平台；版本组件为本 Step 建立）
Step 2（订阅池+版本组件）──▶ Step 3（用户组关联/选定订阅）
Step 2 ──▶ Step 4（Token 绑定订阅/平台；下载读版本文件）
Step 3 ──▶ Step 4（无标识 Token 实时解析依赖组选定）
Step 4 ──▶ Step 5（自定义/分享复用 Token 与版本组件）
Step 2 ──▶ Step 5（自定义/分享复用版本组件）
Step 4/5 ──▶ Step 6（用户端首页依赖 Token 与平台数据；规则复用版本组件）
Step 1~6 ──▶ Step 7（面板布局承载各管理页；装配预留依赖版本组件扩展点）
```

> 线性执行序：Step 1 → 2 → 3 → 4 → 5 → 6 → 7。

---

## 四、分步构建计划

---

### Step 1：平台管理（含安装包分发）

**本 Step 完成后，系统应具备：平台 CRUD、scheme 列表管理（含拖拽排序）、附加响应头配置、客户端安装包本地上传/外部链接双来源分发、平台级联删除的能力。**

- **目标：** 实现平台资源的全生命周期管理与安装包公开分发。
- **前置条件：** Build1 全部 Step 已验收（骨架/认证/Setup 可用；默认 3 平台已预置）。
- **产出文件与操作：**

  1. **创建 `backend/migrations/1001_platforms.sql`**：若 Build1 已建 `platforms` 表则本迁移补充/确认列：`id`、`slug`（UNIQUE，`platform-`+8 短码）、`name`、`description`、`schemes`（JSON 数组，有序）、`extra_headers`（JSON 键值对）、`installer_file`（可空，存带时间戳的文件名）、`installer_url`（可空）、`created_at`、`updated_at`。**平台标识为独立命名空间，不参与订阅/分享/规则/自定义四类标识校验**（Design1 §2.2）。

  2. **创建 `backend/internal/platform/`（业务层）**：平台服务。必须实现：
     - CRUD：**创建后 `slug` 不可修改**；名称不强制唯一；`slug` 由 Build1 的标识生成器生成（`platform-` 前缀）。
     - **scheme 列表**：有序数组存储；**一键导入取列表首项**；scheme 值含 `{url}` 占位符。
     - **附加响应头**：键值对列表；**键与值均禁止 `\r\n` 等控制字符，键另须符合 HTTP 头名规范（不含空格等非法字符，防响应头注入）**；值支持 `{frontend_url}` 占位符。
     - **安装包上传**：≤300MB，**流式落盘**（禁止整读内存）；文件名带时间戳（URL 变化突破 CDN 缓存）；**覆盖时同步删除旧时间戳文件**；**上传/覆盖/删除走 `BEGIN IMMEDIATE` 事务 + 库级写锁**（防并发互删）。存至 `/public/installers/` 可缓存路径。
     - **安装包删除**：可单独删除本地上传安装包（级联删文件，恢复为仅外链/无来源状态）。
     - **删除平台（级联，关键约束，Design1 §4.4）**：级联删除该平台全部订阅（含版本文件）+ 相关下载 Token + 自定义订阅（含版本文件）+ 所有组在该平台的关联与选定 + 安装包文件。**平台删除后组在该平台已无订阅可重选，不置 needs_reselect 标记**。本 Step 实现平台删除时，订阅/Token/自定义/组关联的级联调用可先留接口（这些资源在后续 Step 建立），本 Step 至少完成安装包文件级联与平台行删除，并在代码中以 TODO 注释标注「Step 2/3/5 接入完整级联」。**执行时注意：若选择本 Step 仅删平台与安装包，必须在 Step 5 完成时补齐完整级联并补测试。**

  3. **创建平台端点（接入层，`backend/internal/server/platform.go`）**：全部需会话 + 管理员双中间件：
     - `GET /api/admin/platforms`（列表）、`POST /api/admin/platforms`（创建）、`GET /api/admin/platforms/:id`、`PUT /api/admin/platforms/:id`（编辑，slug 只读）、`DELETE /api/admin/platforms/:id`（级联删除，二次确认由前端负责）。
     - `POST /api/admin/platforms/:id/installer`（上传安装包，流式）、`DELETE /api/admin/platforms/:id/installer`（删除安装包）。
     - **公开下载端点**：`GET /public/installers/<file>`（静态可缓存，无需鉴权，不限流，不记访问日志）。

  4. **创建前端页面**：
     - `frontend/src/views/admin/PlatformsView.vue`（UI §5.4）：双态列表（≥768 表格 / <768 卡片）——名称、标识（只读复制）、scheme 数量、安装包状态（本地已传/外链/无）、操作（编辑/删除危险）；**删除平台 ConfirmModal 逐项列出影响清单（N 份订阅、M 个 Token、K 份自定义订阅 + 文件不可恢复提示）**（影响统计数据由后端删除预览接口或列表接口附带提供）；**新建平台成功后引导「为各用户组选定该平台的订阅」**。
     - `frontend/src/views/admin/PlatformEditView.vue`（独立页非弹窗，UI §7.4）：名称、描述、**scheme 动态列表（`a-form-list`，含 `{url}` 占位符提示，支持拖拽排序，首项即一键导入默认唤起方式）**、**附加响应头键值对编辑器**（动态行，控制字符即时校验报错，`{frontend_url}` 占位符提示）、安装包区（`a-upload` 本地上传 ≤300MB 进度条 + 删除按钮 / 外部链接输入框）、标识只读展示。
     - `frontend/src/api/platform.ts` 接口封装。

- **参考代码/伪代码：**

  > 编写顺序：1001_platforms.sql（确认 Build1 已建表，必要时补列）→ internal/platform（CRUD/校验/安装包事务）→ server/platform.go → 前端页面。复用 Build1：标识生成器（setup 包，本 Step 抽取到共享包）、BEGIN IMMEDIATE 事务助手、双中间件、路径安全。

  **1. `backend/migrations/1001_platforms.sql`**

  ```sql
  -- platforms 表已在 Build1 Step 5（0003）建立；本迁移仅做列确认/补充（幂等）：
  -- 若缺少列则 ALTER TABLE platforms ADD COLUMN ...（SQLite 支持 ADD COLUMN）
  -- 确认列：id / slug UNIQUE / name / description / schemes JSON / extra_headers JSON /
  --        installer_file / installer_url / created_at / updated_at
  -- 平台标识为独立命名空间，不参与四类标识校验（Design1 §2.2）
  ```

  **2. 标识生成器抽取为共享包（Build1 setup 包 → `internal/slug/`）**

  ```go
  // 将 Build1 setup.GenerateSlug 与字符集抽取到 internal/slug 包，供 platform/group/share/custom 共用：
  func Generate(ctx context.Context, tx *sql.Tx, prefix string, exists func(string) (bool, error)) (string, error)
  // 短码字符集：小写字母数字去易混淆；冲突重试最多 3 次，仍冲突报错并记日志（Design1 §2.2）
  ```

  **3. `backend/internal/platform/`（业务层：平台服务）**

  ```go
  const (
      MaxInstallerSize = 300 << 20 // 安装包 ≤300MB
      installerDir     = "public/installers"
  )

  type Service struct {
      store   *store.Store
      dataDir string // 安装包落盘根目录（/data）
      log     *slog.Logger
  }

  type Platform struct {
      ID            int64
      Slug          string
      Name          string
      Description   string
      Schemes       []string          // 有序数组；一键导入取首项；含 {url} 占位符
      ExtraHeaders  map[string]string // 附加响应头；值支持 {frontend_url} 占位符
      InstallerFile string            // 带时间戳文件名
      InstallerURL  string
      // CascadeCounts：删除预览用影响统计（订阅/Token/自定义数量，后续 Step 接通后回填）
  }

  // Create：slug 由生成器自动生成（platform- 前缀）；名称不强制唯一
  func (s *Service) Create(ctx context.Context, name, description string, schemes []string, headers map[string]string) (*Platform, error) {
      if err := ValidateSchemes(schemes); err != nil {
          return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
      }
      if err := ValidateExtraHeaders(headers); err != nil {
          return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
      }
      var created *Platform
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          slug, err := slug.Generate(ctx, tx, "platform-", func(s string) (bool, error) {
              return tableHasSlug(tx, "platforms", s)
          })
          if err != nil {
              return err
          }
          res, err := tx.ExecContext(ctx,
              `INSERT INTO platforms (slug, name, description, schemes, extra_headers) VALUES (?,?,?,?,?)`,
              slug, name, description, toJSON(schemes), toJSON(headers))
          if err != nil {
              return fmt.Errorf("创建平台失败: %w", err)
          }
          // ... 组装 created
          return nil
      })
      return created, err
  }

  // Update：创建后 slug 不可修改（接入层不接收 slug 字段）；可改名称/描述/scheme/附加头
  func (s *Service) Update(ctx context.Context, id int64, name, description string, schemes []string, headers map[string]string) error {
      // 校验同 Create → UPDATE（不含 slug 列）
  }

  // ValidateExtraHeaders：键与值均禁止 \r\n 等控制字符；键另须符合 HTTP 头名规范（防响应头注入）
  var headerNameRe = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`) // RFC 7230 token

  func ValidateExtraHeaders(h map[string]string) error {
      for k, v := range h {
          if !headerNameRe.MatchString(k) {
              return fmt.Errorf("附加头键 %q 不符合 HTTP 头名规范", k)
          }
          if containsControl(k) || containsControl(v) {
              return fmt.Errorf("附加头 %q 含控制字符", k)
          }
      }
      return nil
  }

  func containsControl(s string) bool {
      for _, r := range s {
          if r < 0x20 || r == 0x7f {
              return true
          }
      }
      return false
  }

  // --- 安装包：流式上传 + 覆盖删旧 + 事务（防并发互删，Design1 §4.7）---

  // UploadInstaller：≤300MB，流式落盘（禁止整读内存）；文件名带时间戳（URL 变化突破 CDN 缓存）；
  // BEGIN IMMEDIATE 事务内：读旧文件名 → 写新文件 → 更新 DB → 删旧文件（任一步失败完整清理）
  func (s *Service) UploadInstaller(ctx context.Context, id int64, body io.Reader, filename string) error {
      ext := filepath.Ext(filepath.Base(filename)) // 路径穿越防护：仅取基名扩展名，丢弃任何目录部分
      newName := fmt.Sprintf("installer-%d%s", time.Now().UnixMilli(), sanitizeExt(ext))
      full := filepath.Join(s.dataDir, installerDir, newName)
      if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
          return fmt.Errorf("创建安装包目录失败: %w", err)
      }
      var oldName string
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := tx.QueryRowContext(ctx, `SELECT COALESCE(installer_file,'') FROM platforms WHERE id = ?`, id).Scan(&oldName); err != nil {
              return err
          }
          // 流式落盘：io.Copy 限流包装，超限即中止并清理
          f, err := os.Create(full)
          if err != nil {
              return fmt.Errorf("创建安装包文件失败: %w", err)
          }
          written, err := io.Copy(f, io.LimitReader(body, MaxInstallerSize+1))
          if closeErr := f.Close(); err == nil {
              err = closeErr
          }
          if err != nil {
              _ = os.Remove(full) // 失败清理
              return fmt.Errorf("安装包写入失败: %w", err)
          }
          if written > MaxInstallerSize {
              _ = os.Remove(full)
              return ErrInstallerTooLarge // 接入层映射 400
          }
          if _, err := tx.ExecContext(ctx, `UPDATE platforms SET installer_file = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, newName, id); err != nil {
              _ = os.Remove(full)
              return err
          }
          return nil
      })
      if err != nil {
          return err
      }
      // 事务提交后删旧文件（删除失败仅记日志，不影响新包生效）
      if oldName != "" {
          if err := os.Remove(filepath.Join(s.dataDir, installerDir, oldName)); err != nil && !errors.Is(err, os.ErrNotExist) {
              s.log.Warn("删除旧安装包文件失败", "file", oldName, "err", err)
          }
      }
      return nil
  }

  // DeleteInstaller：单独删除本地安装包（级联删文件，恢复为仅外链/无来源状态）；同样事务内读→清 DB→删文件
  func (s *Service) DeleteInstaller(ctx context.Context, id int64) error { /* 同事务模式 */ }

  // sanitizeExt：清洗安装包扩展名（非白名单拦截——Design1 §6.3 明确「扩展名不做白名单限制」，仅大小校验）。
  // 职责：小写化 + 仅保留安全字符（字母/数字/点），剥除路径分隔符与控制字符，防路径穿越与危险文件名；空扩展名返回空串。
  func sanitizeExt(ext string) string {
      ext = strings.ToLower(ext)
      var b strings.Builder
      for _, r := range ext {
          if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
              b.WriteRune(r)
          }
      }
      return b.String()
  }

  // --- 删除平台（级联，Design1 §4.4，关键约束）---

  // Delete：本 Step 完成安装包文件级联 + 平台行删除；完整级联以接口占位 + TODO 标注，
  // Step 2/3/5 分别接入订阅/Token/组关联/自定义级联，Step 5 完成时补齐并补测试
  func (s *Service) Delete(ctx context.Context, id int64) error {
      var slug, installer string
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := tx.QueryRowContext(ctx, `SELECT slug, COALESCE(installer_file,'') FROM platforms WHERE id = ?`, id).Scan(&slug, &installer); err != nil {
              return err
          }
          // TODO(Build2 Step 2)：级联删该平台全部订阅（含版本文件）+ 指向它们的下载 Token
          // TODO(Build2 Step 3)：级联删所有组在该平台的关联与选定（平台已删无重选对象，不置 needs_reselect）
          // TODO(Build2 Step 5)：级联删该平台全部自定义订阅（含版本文件）与相关 Token
          if _, err := tx.ExecContext(ctx, `DELETE FROM platforms WHERE id = ?`, id); err != nil {
              return fmt.Errorf("删除平台失败: %w", err)
          }
          return nil
      })
      if err != nil {
          return err
      }
      if installer != "" { // 安装包文件级联（事务提交后删，失败仅记日志）
          if err := os.Remove(filepath.Join(s.dataDir, installerDir, installer)); err != nil && !errors.Is(err, os.ErrNotExist) {
              s.log.Warn("删除安装包文件失败", "file", installer, "err", err)
          }
      }
      return nil
  }
  ```

  **4. `backend/internal/server/platform.go`（平台端点，接入层；全部会话 + 管理员双中间件）**

  ```go
  type PlatformHandler struct{ platformSvc *platform.Service }

  func RegisterPlatformRoutes(engine *gin.Engine, h *PlatformHandler, sessionMW, adminMW gin.HandlerFunc) {
      admin := engine.Group("/api/admin/platforms", sessionMW, adminMW)
      admin.GET("", h.list)
      admin.POST("", h.create)
      admin.GET("/:id", h.get)
      admin.PUT("/:id", h.update)     // slug 只读：不接收 slug 字段
      admin.DELETE("/:id", h.delete)  // 级联删除，二次确认由前端 ConfirmModal 负责
      admin.POST("/:id/installer", h.uploadInstaller)
      admin.DELETE("/:id/installer", h.deleteInstaller)
      // 公开下载端点：GET /public/installers/<file> 已由 Build1 Step 3 静态服务承载（可缓存、无需鉴权、不限流、不记访问日志）
  }

  // uploadInstaller：不设 gin 体限制，直接流式透传 c.Request.Body（限流在业务层 LimitReader）
  func (h *PlatformHandler) uploadInstaller(c *gin.Context) {
      id, err := strconv.ParseInt(c.Param("id"), 10, 64)
      if err != nil {
          server.Fail(c, http.StatusBadRequest, "参数错误")
          return
      }
      // multipart 或原始流均可；参考实现取 multipart 文件流
      file, header, err := c.Request.FormFile("file")
      if err != nil {
          server.Fail(c, http.StatusBadRequest, "未接收到文件")
          return
      }
      defer file.Close()
      if err := h.platformSvc.UploadInstaller(c.Request.Context(), id, file, header.Filename); err != nil {
          if errors.Is(err, platform.ErrInstallerTooLarge) {
              server.Fail(c, http.StatusBadRequest, "安装包超过 300MB 限制")
              return
          }
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, nil)
  }
  // list：附删除预览影响统计（CascadeCounts，后续 Step 接通）；create/update/delete 常规绑定与错误映射（409/400/500）
  ```

  **5. 前端**

  ```ts
  // api/platform.ts
  export interface PlatformItem {
    id: number; slug: string; name: string; description: string
    schemes: string[]; extra_headers: Record<string, string>
    installer_file: string; installer_url: string
    cascade?: { subscriptions: number; tokens: number; customs: number } // 删除预览影响统计
  }
  export const listPlatforms = () => http.get<any, PlatformItem[]>('/admin/platforms')
  export const createPlatform = (data: Partial<PlatformItem>) => http.post('/admin/platforms', data)
  export const updatePlatform = (id: number, data: Partial<PlatformItem>) => http.put(`/admin/platforms/${id}`, data)
  export const deletePlatform = (id: number) => http.delete(`/admin/platforms/${id}`)
  export const uploadInstaller = (id: number, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return http.post(`/admin/platforms/${id}/installer`, form, { headers: { 'Content-Type': 'multipart/form-data' } })
  }
  export const deleteInstaller = (id: number) => http.delete(`/admin/platforms/${id}/installer`)
  ```

  ```vue
  <!-- PlatformsView.vue 骨架（UI §5.4）：双态列表（≥768 表格 / <768 卡片，逐页内联实现——2026-08-16 确认口径，本 Step 可先用条件渲染占位） -->
  <script setup lang="ts">
  // 列：名称、标识（只读复制）、scheme 数量、安装包状态（本地已传/外链/无）、操作（编辑/删除危险）
  const toDelete = ref<PlatformItem | null>(null) // ConfirmModal 目标
  // 删除确认：逐项列出影响清单（N 份订阅、M 个 Token、K 份自定义订阅 + 文件不可恢复提示），数据取自行的 cascade 字段
  async function confirmDelete() { await deletePlatform(toDelete.value!.id); Notify.success('平台已删除'); refresh() }
  // 新建成功引导：「为各用户组选定该平台的订阅」（直达组管理按钮 + 跳过，Step 3 接通）
  </script>

  <!-- PlatformEditView.vue 骨架（独立页非弹窗，UI §7.4）：路由 /admin/platforms/:id/edit（新建用 /admin/platforms/new） -->
  <script setup lang="ts">
  // 表单：名称、描述、标识只读展示（新建时展示「保存后自动生成」）
  // scheme 动态列表：a-form-list + 拖拽排序（原生 draggable 或列表上下移按钮，首项即一键导入默认唤起方式），{url} 占位符提示
  // 附加响应头键值对编辑器：动态行（键 + 值 + 删除），控制字符即时校验报错，{frontend_url} 占位符提示
  // 安装包区：a-upload 本地上传（≤300MB 前端预校验 + 进度条 + 删除按钮）/ 外部链接输入框；两者并存
  </script>
  ```

  **6. 单元测试要点（验收要求）**

  - `ValidateExtraHeaders`：含 `\r\n` 的键/值拒绝；非法头名（含空格）拒绝；`{frontend_url}` 合法值通过。
  - 安装包：模拟 >300MB 流被拒且无残留文件；覆盖上传后旧时间戳文件被删；并发两个上传（goroutine）串行完成且仅新文件存活。
  - slug：创建后调 Update 无法改 slug（接入层不接收）。
  - 删除：级联删安装包文件；TODO 级联点在 Step 5 补齐后回归本用例。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：附加头键/值控制字符与头名规范校验、安装包流式上传与覆盖删旧文件、安装包并发上传事务、slug 创建后不可改。
  - 手动验证：创建平台 → 编辑 scheme 拖拽排序保存 → 上传安装包（>300MB 被拒）→ `/public/installers/<file>` 公开可下载且有缓存头 → 删除平台级联删安装包文件。

---

### Step 2：通用版本管理事务组件与订阅池

**本 Step 完成后，系统应具备：四类资源共用的版本管理事务组件（创建/切换/预览/删除/5 版上限/原子切换）、订阅池 CRUD（按平台分组、跨四类标识校验）的能力。**

- **目标：** 建立共享版本管理组件，并基于它实现订阅池。
- **前置条件：** Step 1（平台）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/1002_subscriptions_versions.sql`**：
     - `subscriptions` 表：`id`、`slug`（UNIQUE，自动生成，四类全局命名空间；兼容手填）、`name`（不强制唯一）、`platform_id`（外键）、`current_version`（INTEGER）、`created_at`、`updated_at`。
     - `versions` 表（四类资源共用）：`id`、`owner_type`（TEXT，`subscription`/`rule`/`custom`/`share`）、`owner_id`（INTEGER）、`version_no`（INTEGER）、`file_path`（TEXT）、`created_at`、`updated_at`。联合索引 `(owner_type, owner_id, version_no)`。
     - `subscription_group_rel` 表（订阅-组多对多关联）：`subscription_id`、`group_id`（本 Step 建表，Step 3 使用）。

  2. **创建 `backend/internal/version/`（业务层，共享组件，关键约束）**：版本管理事务组件，四类资源共用。必须实现：
     - `CreateVersion(ownerType, ownerID, content)`：**单个 `BEGIN IMMEDIATE` 事务内**计算版本号（**已有最大编号 + 1，禁止列表长度 + 1**）→ 写版本文件 → 更新版本列表 → 切换当前指针。**任一步失败完整回滚清理（删文件 + 回滚记录）**。
     - **5 版上限**：超出自动删除最旧（文件 + 记录）。
     - `SwitchVersion(ownerType, ownerID, versionNo)`：**原子切换**——写新版本文件（如已存在则直接切）→ 切 symlink（临时指针 + rename）→ 事务内更新 DB「当前」。**切换时更新该版本时间戳（切回旧版本也刷新，首页反映「分发内容最近变动」）**。
     - **当前指针以 DB 记录为准**：下载时先查 DB 当前版本号再读对应版本文件（symlink 仅作文件组织）；**启动时自检：DB「当前」与 symlink 不一致时以 DB 为准重建 symlink**。
     - `DeleteVersion`：不可删最后一个；**不可删当前激活版本（须先切换）**；级联删文件。
     - `PreviewVersion(ownerType, ownerID, versionNo)`：读取指定版本内容（供预览）。
     - **YAML 语法提示**：文本编辑保存前对内容做 YAML 语法检测——仅当内容为 YAML 时提示（非 YAML 如 base64/v2ray/.conf 静默跳过），**均不阻断保存**（提示由前端展示，后端返回 `yaml_warning` 标记）。
     - 版本文件按内容域分目录存放（订阅/规则/自定义/分享），每版本独立文件（`v1`、`v2`…），「当前」为 symlink。
     - **预留第三种版本创建方式扩展点**（装配生成，Design1 §3.9）：版本创建入口以策略/接口形式抽象，当前实现「文件上传」「文本编辑」两种，预留「装配生成」扩展位（不实现逻辑）。

  3. **创建 `backend/internal/subscription/`（业务层）**：订阅池服务。必须实现：
     - CRUD：指定平台 + 名称（不强制唯一；标识**自动生成**（`subscription-` 前缀，见 Issue1 R03-02），创建后展示供复制）+ 关联用户组多选（可为空）。
     - **平台创建后不可修改**；编辑仅可改名称与关联组（取消关联受「该组正在选定此订阅则拒绝」约束，Step 3 接通校验）。
     - **删除订阅（级联，Design1 §4.4）**：级联删除全部版本文件 + 指向它的下载 Token（含管理员显式预览 Token）+ 所有组的关联与选定；**受影响组置空不回退，并在删除事务内置 `needs_reselect` 标记**（Step 3 接通组标记，本 Step 预留调用）。
     - 标识唯一性校验服务：提供 `CheckSlugAvailable(slug, excludeOwner)` 供四类资源共用（跨 subscriptions/rules/custom_subscriptions/share_subscriptions 四表查重）。

  4. **创建订阅端点（接入层，`backend/internal/server/subscription.go`）**：会话 + 管理员：
     - `GET /api/admin/subscriptions`（按平台分组列表，含关联组、「被哪些组选定中」标记）、`POST`（创建）、`PUT /:id`（编辑）、`DELETE /:id`（级联删除）。
     - 版本端点（四类资源通用模式，本 Step 先落地订阅）：`GET /api/admin/subscriptions/:id/versions`、`POST /:id/versions`（创建新版本，支持文件上传/文本编辑双模式）、`PUT /:id/versions/current`（切换当前）、`GET /:id/versions/:ver/preview`（指定版本预览，仅管理员，text/plain 禁缓存）、`DELETE /:id/versions/:ver`。
     - `GET /api/admin/slug/check?slug=&type=`：标识唯一性校验（供前端即时提示）。

  5. **创建前端页面**：
     - `frontend/src/views/admin/SubscriptionsView.vue`（UI §5.1）：`PageHeader`（标题 + 「新建订阅」）；按平台分组 `a-collapse`（默认全展开），组内订阅双态行——名称、当前版本 `a-tag`、关联组标签组、「被 N 个组选定中」蓝色提示标签、操作（版本管理/编辑/删除）。**新建订阅弹窗**（平台 Select、名称、标识输入（实时格式校验 + 唯一性提示，调 slug/check）、关联组多选）；**创建成功后引导弹窗两步「关联用户组 → 每平台选定」（直达组管理按钮 + 「跳过」）**。编辑弹窗（名称 + 关联组多选，平台只读）。
     - `frontend/src/views/admin/VersionManageView.vue`（**通用版本管理视图组件，四类资源复用**，UI §5.1/7.1）：版本表格（版本号、创建/更新时间、当前激活 `a-tag`）；操作：创建新版本（文件上传 `a-upload` / 在线文本编辑双页签，文本编辑保存前 YAML 语法提示 `a-alert warning` 不阻断）、切换当前（确认弹窗）、预览（`a-modal` 宽屏 `a-typography-paragraph code` 纯文本，禁 HTML）、删除（ConfirmModal，当前激活版本禁删提示先切换）。**本组件必须以 props 接收 ownerType/ownerId 与对应 API 前缀，供订阅/规则/自定义/分享四处复用**。
     - `frontend/src/api/subscription.ts`、`frontend/src/api/version.ts`（通用版本接口封装）。

- **参考代码/伪代码：**

  > 编写顺序：1002 迁移 → internal/version（共享组件：创建/切换/删除/预览/自检/扩展点）→ internal/subscription（含四类 slug 交叉校验）→ 端点 → 前端。版本组件是四类资源共用基础，接口设计必须与资源无关（仅 ownerType/ownerID）。

  **1. `backend/migrations/1002_subscriptions_versions.sql`**

  ```sql
  CREATE TABLE subscriptions (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      slug            TEXT NOT NULL UNIQUE,   -- 自动生成（subscription- 前缀；兼容手填），四类全局命名空间
      name            TEXT NOT NULL,          -- 不强制唯一
      platform_id     INTEGER NOT NULL REFERENCES platforms(id),
      current_version INTEGER NOT NULL DEFAULT 0,
      created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  -- 四类资源共用版本表（owner_type：subscription/rule/custom/share）
  CREATE TABLE versions (
      id         INTEGER PRIMARY KEY AUTOINCREMENT,
      owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
      owner_id   INTEGER NOT NULL,
      version_no INTEGER NOT NULL,
      file_path  TEXT NOT NULL,               -- 相对内容根的文件路径
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      UNIQUE (owner_type, owner_id, version_no)
  );
  CREATE INDEX idx_versions_owner ON versions(owner_type, owner_id, version_no);

  -- 订阅-组多对多关联（本 Step 建表，Step 3 使用）
  CREATE TABLE subscription_group_rel (
      subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
      group_id        INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
      PRIMARY KEY (subscription_id, group_id)
  );
  ```

  **2. `backend/internal/version/`（业务层：共享版本管理事务组件，关键约束）**

  ```go
  // 文件组织：{dataDir}/contents/{ownerType}/{ownerID}/v{n}，「当前」为 symlink current；
  // 下载以 DB 记录为准（先查 current_version 再读对应文件），symlink 仅作文件组织
  const (
      MaxVersions = 5 // 每份资源最多 5 个版本（含当前激活）
      currentLink = "current"
  )

  type OwnerType string

  const (
      OwnerSubscription OwnerType = "subscription"
      OwnerRule         OwnerType = "rule"
      OwnerCustom       OwnerType = "custom"
      OwnerShare        OwnerType = "share"
  )

  type Service struct {
      store   *store.Store
      dataDir string
      log     *slog.Logger
  }

  type Version struct {
      No        int64
      FilePath  string
      Current   bool // 由调用方对照 owner 的 current_version 填充
      CreatedAt time.Time
      UpdatedAt time.Time
  }

  // --- 版本创建来源策略（预留第三种扩展点，Design1 §3.9）---

  // ContentProvider：版本内容来源抽象；当前实现「文件上传」「文本编辑」两种，
  // 「装配生成」在 DesignOnHold 恢复开发时新增实现，不改本组件
  type ContentProvider interface {
      Content() ([]byte, error)
  }

  type BytesContent []byte               // 文本编辑来源
  func (b BytesContent) Content() ([]byte, error) { return b, nil }

  type ReaderContent struct{ R io.Reader; Max int64 } // 文件上传来源（流式，限大小）
  func (r ReaderContent) Content() ([]byte, error)    { return io.ReadAll(io.LimitReader(r.R, r.Max)) }

  // CreateVersion：单个 BEGIN IMMEDIATE 事务内：计算版本号（已有最大编号 + 1，禁止列表长度 + 1）
  // → 写版本文件 → 写版本记录 → 切换当前指针；任一步失败完整回滚清理（删文件 + 回滚记录）
  func (s *Service) CreateVersion(ctx context.Context, ot OwnerType, ownerID int64, src ContentProvider) (*Version, error) {
      content, err := src.Content()
      if err != nil {
          return nil, fmt.Errorf("读取版本内容失败: %w", err)
      }
      var created *Version
      err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          // 版本号 = 已有最大编号 + 1（删除后不复用，Design1 §4.1）
          var maxNo int64
          if err := tx.QueryRowContext(ctx,
              `SELECT COALESCE(MAX(version_no), 0) FROM versions WHERE owner_type = ? AND owner_id = ?`, ot, ownerID).
              Scan(&maxNo); err != nil {
              return err
          }
          newNo := maxNo + 1
          rel := versionRelPath(ot, ownerID, newNo)
          full := filepath.Join(s.dataDir, "contents", rel)
          if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
              return fmt.Errorf("创建版本目录失败: %w", err)
          }
          if err := os.WriteFile(full, content, 0o644); err != nil {
              return fmt.Errorf("写版本文件失败: %w", err)
          }
          if _, err := tx.ExecContext(ctx,
              `INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES (?,?,?,?)`,
              ot, ownerID, newNo, rel); err != nil {
              _ = os.Remove(full) // 失败清理：删文件
              return fmt.Errorf("写版本记录失败: %w", err)
          }
          // 切换当前指针（DB + symlink；事务内任一失败回滚后文件由外层清理）
          if err := s.setCurrentLocked(ctx, tx, ot, ownerID, newNo); err != nil {
              _ = os.Remove(full)
              return err
          }
          // 5 版上限：超出自动删最旧（文件 + 记录，含当前激活版本以外的最旧）
          if err := s.evictOldest(ctx, tx, ot, ownerID, newNo); err != nil {
              return err
          }
          created = &Version{No: newNo, FilePath: rel, Current: true}
          return nil
      })
      return created, err
  }

  // evictOldest：版本数 > MaxVersions 时删最旧（不删当前激活）
  func (s *Service) evictOldest(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID, currentNo int64) error {
      rows, err := tx.QueryContext(ctx,
          `SELECT version_no, file_path FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no != ? ORDER BY version_no ASC LIMIT ?`,
          ot, ownerID, currentNo, -1) // 取全部非当前版本，按升序
      if err != nil {
          return err
      }
      defer rows.Close()
      // 计数后对超出部分逐个 DELETE + os.Remove（事务内删记录，文件同步删）
      ...
  }

  // SwitchVersion：原子切换——切 symlink（临时指针 + rename）→ 事务内更新 DB「当前」+ 刷新该版本时间戳
  // （切回旧版本也刷新，首页反映「分发内容最近变动」）
  func (s *Service) SwitchVersion(ctx context.Context, ot OwnerType, ownerID, versionNo int64) error {
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var exists int
          if err := tx.QueryRowContext(ctx,
              `SELECT COUNT(*) FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo).Scan(&exists); err != nil {
              return err
          }
          if exists == 0 {
              return ErrVersionNotFound
          }
          return s.setCurrentLocked(ctx, tx, ot, ownerID, versionNo)
      })
  }

  // setCurrentLocked：DB 更新 owner 表 current_version + 版本行 updated_at；symlink 临时指针 + rename 原子替换
  func (s *Service) setCurrentLocked(ctx context.Context, tx *sql.Tx, ot OwnerType, ownerID, versionNo int64) error {
      if err := updateOwnerCurrent(ctx, tx, ot, ownerID, versionNo); err != nil { // 按 ownerType 更新对应 owner 表
          return err
      }
      if _, err := tx.ExecContext(ctx,
          `UPDATE versions SET updated_at = CURRENT_TIMESTAMP WHERE owner_type = ? AND owner_id = ? AND version_no = ?`,
          ot, ownerID, versionNo); err != nil {
          return err
      }
      return s.rebuildSymlink(ot, ownerID, versionNo) // 临时文件 current.tmp + os.Rename 原子替换
  }

  // DeleteVersion：不可删最后一个；不可删当前激活版本（须先切换）；级联删文件
  func (s *Service) DeleteVersion(ctx context.Context, ot OwnerType, ownerID, versionNo int64) error {
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var count int
          if err := tx.QueryRowContext(ctx,
              `SELECT COUNT(*) FROM versions WHERE owner_type = ? AND owner_id = ?`, ot, ownerID).Scan(&count); err != nil {
              return err
          }
          if count <= 1 {
              return ErrLastVersion // 「不可删最后一个」，接入层映射 400
          }
          current, err := ownerCurrent(ctx, tx, ot, ownerID)
          if err != nil {
              return err
          }
          if current == versionNo {
              return ErrCurrentVersion // 「不可删当前激活版本（须先切换）」
          }
          var rel string
          if err := tx.QueryRowContext(ctx,
              `SELECT file_path FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo).Scan(&rel); err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx,
              `DELETE FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo); err != nil {
              return err
          }
          if err := os.Remove(filepath.Join(s.dataDir, "contents", rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
              s.log.Warn("删除版本文件失败", "path", rel, "err", err) // 不阻断
          }
          return nil
      })
  }

  // PreviewVersion：读指定版本内容（供预览；接入层 text/plain + 禁缓存）
  func (s *Service) PreviewVersion(ctx context.Context, ot OwnerType, ownerID, versionNo int64) ([]byte, error) {
      var rel string
      if err := s.store.DB().QueryRowContext(ctx,
          `SELECT file_path FROM versions WHERE owner_type = ? AND owner_id = ? AND version_no = ?`, ot, ownerID, versionNo).Scan(&rel); err != nil {
          return nil, ErrVersionNotFound
      }
      return os.ReadFile(filepath.Join(s.dataDir, "contents", rel))
  }

  // ReadCurrent：下载分发用——先查 DB 当前版本号再读对应版本文件（以 DB 为准，Design1 §4.1）
  func (s *Service) ReadCurrent(ctx context.Context, ot OwnerType, ownerID int64) ([]byte, error) { /* ... */ }

  // StartupCheck：启动自检——DB「当前」与 symlink 不一致时以 DB 为准重建 symlink
  func (s *Service) StartupCheck(ctx context.Context) error {
      // 遍历 versions 表的 (owner_type, owner_id) 去重集合，逐个对照 owner 表 current_version 重建
      ...
  }

  // YamlWarning：文本编辑保存前 YAML 语法检测——先启发式判定「是否 YAML」，是 YAML 再做语法检测并提示，
  // 非 YAML（base64/v2ray/.conf）静默跳过；均不阻断保存（提示由前端 a-alert warning 展示）
  func YamlWarning(content []byte) string {
      if looksNonYaml(content) { // 启发式：可 base64 解码，或以 v2ray://、clash:// 等协议前缀开头 → 静默跳过
          return ""
      }
      if !looksLikeYaml(content) { // 启发式：不含「键: 值」行结构且非 --- 开头 → 不视为 YAML，静默跳过
          return ""
      }
      var probe any
      if err := yaml.Unmarshal(content, &probe); err != nil {
          return "YAML 语法问题：" + err.Error() // 判定为 YAML 但语法错误 → 返回警告标记（不阻断）
      }
      return "" // 合法 YAML → 无警告
  }
  // 注：YAML 检测库以 Design1 §5.1 允许的范围内择一并全程统一（参考实现用 gopkg.in/yaml.v3）；
  // looksLikeYaml 启发式：--- 开头，或存在「非空键 + 冒号 + 空格/换行」的行（如 proxies:、port: 8080）
  ```

  **3. `backend/internal/subscription/`（业务层：订阅池）**

  ```go
  var slugRe = regexp.MustCompile(`^[a-z0-9-]{3,64}$`) // 手填标识：小写字母数字连字符，3~64

  type Service struct {
      store    *store.Store
      versions *version.Service
      log      *slog.Logger
  }

  // CheckSlugAvailable：四类资源全局唯一命名空间交叉校验（跨四表查重，供四类资源共用）
  func (s *Service) CheckSlugAvailable(ctx context.Context, slugVal string, excludeOwner string, excludeID int64) (bool, error) {
      if !slugRe.MatchString(slugVal) {
          return false, nil // 格式不合法直接不可用
      }
      for _, table := range []string{"subscriptions", "rules", "custom_subscriptions", "share_subscriptions"} {
          // 后三表在后续 Step 建立；本 Step 对不存在的表跳过（sqlite_master 预检），Step 5/6 完成后全量生效
          var n int
          // SELECT COUNT(*) FROM <table> WHERE slug = ?（排除自身时用 AND id != excludeID）
          ...
          if n > 0 {
              return false, nil
          }
      }
      return true, nil
  }

  // Create：指定平台 + 名称 + 手填标识（交叉校验）+ 关联组多选（可空）+ 首版本内容（可选）
  func (s *Service) Create(ctx context.Context, in CreateInput) (*Subscription, error) {
      ok, err := s.CheckSlugAvailable(ctx, in.Slug, "", 0)
      if err != nil {
          return nil, err
      }
      if !ok {
          return nil, ErrSlugConflict // 接入层映射 409
      }
      var created *Subscription
      err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          // INSERT subscriptions（platform 创建后不可修改）→ 写关联组 → 有首版本内容则调 versions.CreateVersion
          ...
          return nil
      })
      return created, err
  }

  // Update：仅可改名称与关联组；平台只读；
  // 取消关联受「该组正在选定此订阅则拒绝」约束（Step 3 接通 group_selections 校验，本 Step 预留接口）
  func (s *Service) Update(ctx context.Context, id int64, name string, groupIDs []int64) error {
      // TODO(Build2 Step 3)：校验被移除的组是否在 group_selections 中选定此订阅，是则拒绝并提示先改选
      ...
  }

  // Delete（级联，Design1 §4.4）：删全部版本文件 + 指向它的下载 Token（含显式预览 Token）
  // + 所有组的关联与选定；受影响组置空不回退并在删除事务内置 needs_reselect
  func (s *Service) Delete(ctx context.Context, id int64) error {
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          // 1) 收集全部版本文件路径 → DELETE versions 行 → 事务提交后删文件
          // 2) TODO(Build2 Step 4)：DELETE download_tokens WHERE subscription_id = ?（含显式预览 Token）
          // 3) DELETE subscription_group_rel WHERE subscription_id = ?
          // 4) TODO(Build2 Step 3)：清 group_selections 选定 + 对失去选定的组 UPDATE groups SET needs_reselect = 1
          // 5) DELETE subscriptions WHERE id = ?
          ...
          return nil
      })
      // 事务提交后删版本文件（失败记日志不阻断，与平台删除同模式）
  }
  ```

  **4. `backend/internal/server/subscription.go`（订阅端点；会话 + 管理员）**

  ```go
  type SubscriptionHandler struct {
      subSvc  *subscription.Service
      verSvc  *version.Service
  }

  func RegisterSubscriptionRoutes(engine *gin.Engine, h *SubscriptionHandler, sessionMW, adminMW gin.HandlerFunc) {
      admin := engine.Group("/api/admin/subscriptions", sessionMW, adminMW)
      admin.GET("", h.list)   // 按平台分组列表，含关联组、「被哪些组选定中」标记（Step 3 接通）
      admin.POST("", h.create)
      admin.PUT("/:id", h.update)
      admin.DELETE("/:id", h.delete)

      // 版本端点（四类资源通用模式，本 Step 先落地订阅）
      admin.GET("/:id/versions", h.listVersions)
      admin.POST("/:id/versions", h.createVersion)          // 文件上传/文本编辑双模式（multipart 字段 mode=upload|text）
      admin.PUT("/:id/versions/current", h.switchVersion)   // body: { version_no }
      admin.GET("/:id/versions/:ver/preview", h.previewVersion) // text/plain + no-store，仅管理员
      admin.DELETE("/:id/versions/:ver", h.deleteVersion)

      // 标识唯一性即时校验（供前端输入时提示）
      engine.GET("/api/admin/slug/check", sessionMW, adminMW, h.checkSlug) // ?slug=&type=
  }

  // createVersion：双模式——mode=upload 取 multipart 文件流（ReaderContent，≤50MB，Design1 §6.3）；
  // mode=text 取文本体（BytesContent）；文本模式返回 yaml_warning 标记（不阻断）
  func (h *SubscriptionHandler) createVersion(c *gin.Context) {
      // ... 解析 id / mode；调用 versions.CreateVersion(ctx, version.OwnerSubscription, id, provider)
      // 返回 { "version_no": N, "yaml_warning": "..." }
  }

  // previewVersion：禁缓存 + text/plain（AGENTS §4.5 下载类分级）
  func (h *SubscriptionHandler) previewVersion(c *gin.Context) {
      content, err := h.verSvc.PreviewVersion(ctx, version.OwnerSubscription, id, ver)
      if errors.Is(err, version.ErrVersionNotFound) {
          server.Fail(c, http.StatusNotFound, "版本不存在")
          return
      }
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      c.Header("Cache-Control", "no-store")
      c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
  }
  ```

  **5. 前端**

  ```ts
  // api/version.ts：通用版本接口封装（四类资源复用，前缀参数化）
  export function versionApi(prefix: string) {
    return {
      list: (ownerId: number) => http.get<any, VersionItem[]>(`${prefix}/${ownerId}/versions`),
      create: (ownerId: number, payload: FormData | { text: string }) =>
        http.post(`${prefix}/${ownerId}/versions`, payload),
      switchCurrent: (ownerId: number, versionNo: number) =>
        http.put(`${prefix}/${ownerId}/versions/current`, { version_no: versionNo }),
      preview: (ownerId: number, ver: number) =>
        http.get(`${prefix}/${ownerId}/versions/${ver}/preview`, { responseType: 'text' }),
      remove: (ownerId: number, ver: number) => http.delete(`${prefix}/${ownerId}/versions/${ver}`),
    }
  }
  // api/subscription.ts：list/create/update/delete + checkSlug(slug) 调 /api/admin/slug/check
  ```

  ```vue
  <!-- VersionManageView.vue：通用版本管理视图组件，四类资源复用（props 驱动，UI §5.1/7.1） -->
  <script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { Table, Tag, Modal, Upload, Tabs, Input, Alert } from 'ant-design-vue'
  import { versionApi } from '@/api/version'
  import ConfirmModal from '@/components/ConfirmModal.vue'
  import TriStateList from '@/components/TriStateList.vue'
  import { Notify } from '@/components/Notify'

  const props = defineProps<{
    ownerType: 'subscription' | 'rule' | 'custom' | 'share'
    ownerId: number
    apiPrefix: string // 如 /api/admin/subscriptions
    resourceName: string // 面包屑/标题展示用
  }>()

  const api = versionApi(props.apiPrefix)
  const loading = ref(false)
  const versions = ref<VersionItem[]>([])
  const createOpen = ref(false)
  const createMode = ref<'upload' | 'text'>('upload')
  const editText = ref('')
  const yamlWarning = ref('')
  const previewContent = ref<string | null>(null)
  const toDelete = ref<number | null>(null)
  const toSwitch = ref<number | null>(null)

  async function load() { /* api.list → versions */ }

  // 文本编辑保存前 YAML 语法提示（后端返回 yaml_warning，a-alert warning 展示不阻断）
  async function saveText() {
    const res = await api.create(props.ownerId, { text: editText.value })
    if (res.yaml_warning) yamlWarning.value = res.yaml_warning
    Notify.success('新版本已创建并切换为当前')
    await load()
  }

  async function doSwitch() {
    await api.switchCurrent(props.ownerId, toSwitch.value!)
    Notify.success('当前版本已切换')
    await load()
  }

  async function doDelete() {
    try {
      await api.remove(props.ownerId, toDelete.value!)
      Notify.success('版本已删除')
    } catch (err) {
      Notify.error((err as Error).message) // 「不可删当前激活版本，请先切换」等
    }
    await load()
  }
  onMounted(load)
  </script>

  <template>
    <TriStateList :loading="loading" :empty="versions.length === 0" empty-text="暂无版本，请创建第一个版本">
      <Table :data-source="versions" :pagination="false" row-key="version_no">
        <!-- 列：版本号、创建时间、更新时间、当前激活 Tag；操作：切换（确认弹窗）/预览/删除 -->
        <!-- 当前激活版本删除按钮禁用 + tooltip「请先切换到其他版本」 -->
      </Table>
    </TriStateList>

    <!-- 创建新版本弹窗：文件上传 / 在线文本编辑双页签 -->
    <Modal v-model:open="createOpen" title="创建新版本" :footer="null">
      <Tabs v-model:activeKey="createMode">
        <Tabs.TabPane key="upload" tab="文件上传"><Upload :max-count="1" @change="onUpload">...</Upload></Tabs.TabPane>
        <Tabs.TabPane key="text" tab="在线编辑">
          <Alert v-if="yamlWarning" type="warning" :message="yamlWarning" class="mb-2" />
          <Input.TextArea v-model:value="editText" :rows="12" />
          <Button type="primary" class="mt-2" @click="saveText">保存为新版本</Button>
        </Tabs.TabPane>
      </Tabs>
    </Modal>

    <!-- 预览弹窗：宽屏纯文本（禁 HTML），a-typography-paragraph code 风格 -->
    <Modal :open="previewContent !== null" width="80%" :footer="null" @cancel="previewContent = null">
      <pre class="text-xs overflow-auto max-h-[70vh]">{{ previewContent }}</pre>
    </Modal>

    <ConfirmModal v-model:open="toDelete !== null" title="删除版本" danger
                  content="删除后不可恢复" @confirm="doDelete" />
    <ConfirmModal v-model:open="toSwitch !== null" title="切换当前版本"
                  content="切换后所有下载立即生效" @confirm="doSwitch" />
  </template>
  ```

  ```vue
  <!-- SubscriptionsView.vue 骨架（UI §5.1） -->
  <script setup lang="ts">
  // PageHeader（标题 + 「新建订阅」）；按平台分组 a-collapse（默认全展开），组内双态行：
  // 名称、当前版本 Tag、关联组标签组、「被 N 个组选定中」蓝色提示标签（Step 3 接通数据）、操作（版本管理/编辑/删除）
  // 新建弹窗：平台 Select、名称、标识输入（防抖调 checkSlug 实时提示 + 格式校验）、关联组多选
  // 创建成功引导弹窗两步「关联用户组 → 每平台选定」（直达组管理按钮 + 「跳过」，Step 3 接通）
  // 编辑弹窗：名称 + 关联组多选（平台只读）；删除 ConfirmModal（影响清单）
  </script>
  ```

  **6. 单元测试要点（验收要求）**

  - 版本号：创建 3 版删 v2 后再建 → 新号为 4（最大编号 +1，不复用）。
  - 5 版上限：连续上传 6 版 → 仅存 5 版且最旧被删（文件 + 记录）。
  - 删除约束：删最后一个拒绝；删当前激活拒绝（ErrCurrentVersion）。
  - 并发创建：N 个 goroutine 同时 CreateVersion → 版本号连续不重复（BEGIN IMMEDIATE 串行化）。
  - 原子切换与自检：手工破坏 symlink 后 StartupCheck 以 DB 为准重建。
  - 失败回滚：注入 DB 写失败 → 版本文件无残留。
  - slug 交叉校验：同 slug 分别建订阅与（Step 5/6 接通后的）规则/分享/自定义，第二个 409。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：版本号最大编号 +1（删除后不复用）、5 版上限自动删最旧、不可删最后/当前版本、并发创建版本事务串行化（`BEGIN IMMEDIATE`）、原子切换与启动自检重建 symlink、失败回滚清理、跨四类标识查重。
  - 手动验证：创建订阅（标识冲突即时提示）→ 上传 6 个版本（自动删最旧保留 5）→ 切换当前 → 预览历史版本 → 删除非当前版本 → 删除订阅级联清理。

---

### Step 3：用户组与订阅分发机制

**本 Step 完成后，系统应具备：用户组 CRUD、每平台选定一份订阅、删除组迁入默认组、删除订阅置 needs_reselect 标记、组关联/选定约束校验的能力。**

- **目标：** 实现用户组与订阅池的多对多关联及每平台选定分发机制。
- **前置条件：** Step 2（订阅池）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/1003_groups.sql`**：
     - `groups` 表若 Build1 已建则确认列：`id`、`slug`（UNIQUE，`group-`+8 短码）、`name`（UNIQUE）、`is_default`、`needs_reselect`（INTEGER 默认 0）、`created_at`、`updated_at`。
     - `group_selections` 表（组选定）：`id`、`group_id`、`platform_id`、`subscription_id`（可空），**每组每平台至多一份**（UNIQUE(group_id, platform_id)）。
     - `subscription_group_rel` 已在 Step 2 建立（订阅-组关联）。

  2. **创建 `backend/internal/group/`（业务层）**：用户组服务。必须实现：
     - CRUD：名称全局唯一校验；`slug` 自动生成（`group-` 前缀，独立命名空间）；**预置默认组不可删除**（可改名受唯一校验）。
     - **每平台选定**：每组按平台从关联订阅中选定一份（可不选）；选定存 `group_selections`。
     - **关联订阅管理**：为组多选关联订阅；**取消订阅与组的关联时若该组正在选定此订阅 → 拒绝操作，提示先去改选**（防悬空选定，Design1 §4.4）。
     - **删除组**：组内用户自动迁入默认组（**Token 无需清理，实时解析自动跟随**）；默认组不可删。
     - **needs_reselect 标记**：删除订阅事务内将受影响组置位；重新选定后清除。
     - **选定变更影响提示**：提供「影响用户数」统计（组内用户数）供前端提示。

  3. **接通 Step 2 预留的级联**：删除订阅时对所有组清除该订阅的关联与选定，并对「因此失去选定」的组置 `needs_reselect`。补充完整级联测试。

  4. **创建组端点（接入层，`backend/internal/server/group.go`）**：会话 + 管理员：
     - `GET /api/admin/groups`（列表：组名、关联订阅数、组内用户数、needs_reselect）、`POST`（创建）、`PUT /:id`（改名 + 关联订阅 + 每平台选定）、`DELETE /:id`（迁入默认组）。
     - `PUT /api/admin/groups/:id/selections`（每平台选定，入参 `[{platform_id, subscription_id}]`）。

  5. **创建前端 `frontend/src/views/admin/GroupsView.vue`**（UI §5.2）：
     - **分发引导**：首管理员首次登录一次性 `a-alert` 引导条「创建第一份订阅」（直达订阅管理，可关闭）；存在 `needs_reselect` 标记的组行高亮（警示底色）+「重新选定」快捷按钮（平台删除不触发高亮）。
     - 组列表（双态）：组名（预置默认组带 `a-tag` 且无删除操作）、关联订阅数、组内用户数、操作（编辑/重新选定/删除——默认组不可删）。
     - **组编辑弹窗**：改名（唯一校验）+ **每平台选定区**（每平台一行 `a-select`，从关联订阅中选；选定变更时提示「影响 N 名用户」）+ 关联订阅多选（取消正被选定的订阅 → 拒绝并提示先改选）。
     - **删除组**：ConfirmModal「组内 N 名用户将自动迁入默认组」。
     - `frontend/src/api/group.ts`。

- **参考代码/伪代码：**

  > 编写顺序：1003_groups.sql → internal/group（CRUD/选定/关联约束/删组迁入/标记）→ 接通 Step 2 删订阅级联 → server/group.go → GroupsView.vue。复用：slug 生成器（group- 前缀）、BEGIN IMMEDIATE 事务助手。

  **1. `backend/migrations/1003_groups.sql`**

  ```sql
  -- groups 表已在 Build1 Step 5（0003）建立；本迁移确认列（幂等）：
  -- id / slug UNIQUE（group-+8 短码）/ name UNIQUE / is_default / needs_reselect 默认 0 / created_at / updated_at

  -- 组选定：每组每平台至多一份
  CREATE TABLE group_selections (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      group_id        INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
      platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
      subscription_id INTEGER REFERENCES subscriptions(id) ON DELETE SET NULL, -- 订阅被删时置空（不回退）
      UNIQUE (group_id, platform_id)
  );
  -- subscription_group_rel 已在 Step 2（1002）建立
  ```

  **2. `backend/internal/group/`（业务层：用户组服务）**

  ```go
  type Service struct {
      store *store.Store
      log   *slog.Logger
  }

  type Group struct {
      ID            int64
      Slug          string
      Name          string
      IsDefault     bool
      NeedsReselect bool
      SubCount      int64 // 关联订阅数
      UserCount     int64 // 组内用户数
  }

  // Create：名称全局唯一校验；slug 自动生成（group- 前缀，独立命名空间）
  func (s *Service) Create(ctx context.Context, name string) (*Group, error) {
      var created *Group
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var dup int
          if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE name = ?`, name).Scan(&dup); err != nil {
              return err
          }
          if dup > 0 {
              return ErrNameConflict // 接入层映射 409
          }
          gslug, err := slug.Generate(ctx, tx, "group-", func(v string) (bool, error) {
              return tableHasSlug(tx, "groups", v)
          })
          if err != nil {
              return err
          }
          // INSERT groups → 组装 created
          ...
          return nil
      })
      return created, err
  }

  // Update：改名（唯一校验）+ 关联订阅多选 + 每平台选定，单事务完成
  func (s *Service) Update(ctx context.Context, id int64, name string, subIDs []int64, selections []Selection) error {
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := s.checkEditable(ctx, tx, id); err != nil { // 组存在性校验
              return err
          }
          // 改名唯一校验（排除自身）
          var dup int
          if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM groups WHERE name = ? AND id != ?`, name, id).Scan(&dup); err != nil {
              return err
          }
          if dup > 0 {
              return ErrNameConflict
          }
          // 关联变更约束：取消订阅与组的关联时，若该组正在选定此订阅 → 拒绝（防悬空选定，Design1 §4.4）
          removed, err := diffRemovedSubs(ctx, tx, id, subIDs)
          if err != nil {
              return err
          }
          for _, subID := range removed {
              var selected int
              if err := tx.QueryRowContext(ctx,
                  `SELECT COUNT(*) FROM group_selections WHERE group_id = ? AND subscription_id = ?`, id, subID).Scan(&selected); err != nil {
                  return err
              }
              if selected > 0 {
                  return ErrSubInSelection // 「该组正在选定此订阅，请先改选」，接入层 409/400
              }
          }
          // UPDATE 名称；重建 subscription_group_rel；重建 group_selections（校验选定必须在关联范围内）
          ...
          // 重新选定后清除 needs_reselect 标记
          if _, err := tx.ExecContext(ctx, `UPDATE groups SET needs_reselect = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
              return err
          }
          return nil
      })
  }

  type Selection struct {
      PlatformID     int64 `json:"platform_id"`
      SubscriptionID int64 `json:"subscription_id"` // 0 = 取消选定
  }

  // SetSelections：每平台选定（入参 [{platform_id, subscription_id}]）；选定变更需校验订阅在该组关联内
  func (s *Service) SetSelections(ctx context.Context, id int64, selections []Selection) error {
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          for _, sel := range selections {
              if sel.SubscriptionID != 0 {
                  var linked int
                  if err := tx.QueryRowContext(ctx,
                      `SELECT COUNT(*) FROM subscription_group_rel WHERE group_id = ? AND subscription_id = ?`,
                      id, sel.SubscriptionID).Scan(&linked); err != nil {
                      return err
                  }
                  if linked == 0 {
                      return ErrSubNotLinked // 选定必须来自关联订阅
                  }
              }
              // UPSERT：INSERT ... ON CONFLICT(group_id, platform_id) DO UPDATE；subscription_id=0 时置 NULL
              ...
          }
          // 全部平台选定完成后清除 needs_reselect
          if _, err := tx.ExecContext(ctx, `UPDATE groups SET needs_reselect = 0 WHERE id = ?`, id); err != nil {
              return err
          }
          return nil
      })
  }

  // Delete：默认组不可删；组内用户自动迁入默认组（Token 无需清理，实时解析自动跟随）
  func (s *Service) Delete(ctx context.Context, id int64) error {
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var isDefault int
          if err := tx.QueryRowContext(ctx, `SELECT is_default FROM groups WHERE id = ?`, id).Scan(&isDefault); err != nil {
              return err
          }
          if isDefault == 1 {
              return ErrDefaultGroup // 「预置默认组不可删除」，接入层 400
          }
          var defaultID int64
          if err := tx.QueryRowContext(ctx, `SELECT id FROM groups WHERE is_default = 1 LIMIT 1`).Scan(&defaultID); err != nil {
              return fmt.Errorf("预置默认组缺失: %w", err)
          }
          if _, err := tx.ExecContext(ctx, `UPDATE users SET group_id = ? WHERE group_id = ?`, defaultID, id); err != nil {
              return fmt.Errorf("迁入默认组失败: %w", err)
          }
          // 关联/选定由外键 ON DELETE CASCADE 级联清理
          if _, err := tx.ExecContext(ctx, `DELETE FROM groups WHERE id = ?`, id); err != nil {
              return err
          }
          return nil
      })
  }

  // CountAffectedUsers：选定变更影响提示（组内用户数）
  func (s *Service) CountAffectedUsers(ctx context.Context, id int64) (int64, error) {
      var n int64
      err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE group_id = ?`, id).Scan(&n)
      return n, err
  }

  // --- 接通 Step 2 预留的删订阅级联（subscription.Delete 内调用）---

  // OnSubscriptionDeleted：清该订阅的关联与选定；对「因此失去选定」的组置 needs_reselect（平台删除不触发）
  func (s *Service) OnSubscriptionDeleted(ctx context.Context, tx *sql.Tx, subscriptionID int64) error {
      // 1) 找出选定此订阅的组
      // 2) DELETE group_selections WHERE subscription_id = ?（或置 NULL，语义=失去选定）
      // 3) DELETE subscription_group_rel WHERE subscription_id = ?
      // 4) UPDATE groups SET needs_reselect = 1 WHERE id IN (失去选定的组)
      ...
      return nil
  }
  ```

  **3. `backend/internal/server/group.go`（组端点；会话 + 管理员）**

  ```go
  type GroupHandler struct{ groupSvc *group.Service }

  func RegisterGroupRoutes(engine *gin.Engine, h *GroupHandler, sessionMW, adminMW gin.HandlerFunc) {
      admin := engine.Group("/api/admin/groups", sessionMW, adminMW)
      admin.GET("", h.list) // 组名、关联订阅数、组内用户数、needs_reselect
      admin.POST("", h.create)
      admin.PUT("/:id", h.update)                    // 改名 + 关联订阅 + 每平台选定（整体提交）
      admin.DELETE("/:id", h.delete)                 // 迁入默认组
      admin.PUT("/:id/selections", h.setSelections)  // 入参 [{platform_id, subscription_id}]
  }
  // 错误映射：ErrNameConflict → 409；ErrDefaultGroup/ErrSubInSelection → 400 带提示文案
  ```

  **4. 前端 `GroupsView.vue`（UI §5.2）**

  ```vue
  <script setup lang="ts">
  import { computed, onMounted, ref } from 'vue'
  import { Alert, Badge, Modal, Select, Table, Tag } from 'ant-design-vue'
  import { listGroups, updateGroup, deleteGroup, setSelections } from '@/api/group'
  import ConfirmModal from '@/components/ConfirmModal.vue'
  import TriStateList from '@/components/TriStateList.vue'
  import { Notify } from '@/components/Notify'

  const groups = ref<GroupItem[]>([])
  const editing = ref<GroupItem | null>(null)
  const toDelete = ref<GroupItem | null>(null)

  // 首管理员首次登录一次性引导条（localStorage 键 first_admin_guide_dismissed 记录关闭）
  const showGuide = computed(() => localStorage.getItem('first_admin_guide_dismissed') !== '1')

  // 组编辑弹窗：改名（唯一校验）+ 每平台选定区 + 关联订阅多选
  // 每平台一行 a-select（从关联订阅中选）；选定变更时提示「影响 N 名用户」（后端 CountAffectedUsers 随列表返回 user_count）
  // 取消正被选定的订阅 → 后端 ErrSubInSelection → Notify.error「请先在选定区改选该订阅」
  async function saveEdit() {
    try {
      await updateGroup(editing.value!.id, editForm)
      Notify.success('组已更新')
      editing.value = null
      await load()
    } catch (err) {
      Notify.error((err as Error).message)
    }
  }

  // 删除确认：「组内 N 名用户将自动迁入默认组」
  async function confirmDelete() {
    await deleteGroup(toDelete.value!.id)
    Notify.success('组已删除，成员已迁入默认组')
    toDelete.value = null
    await load()
  }
  </script>

  <template>
    <!-- 分发引导：一次性 a-alert「创建第一份订阅」（直达订阅管理，可关闭） -->
    <Alert v-if="showGuide" type="info" closable class="mb-4" message="还没有订阅内容？"
           description="前往订阅管理创建第一份订阅，再为各用户组选定分发" @close="localStorage.setItem('first_admin_guide_dismissed','1')" />
    <TriStateList :loading="loading" :empty="groups.length === 0" empty-text="暂无用户组">
      <Table :data-source="groups" row-key="id"
             :row-class-name="(r: GroupItem) => r.needs_reselect ? 'row-warn' : ''">
        <!-- 列：组名（预置默认组带 Tag 且无删除操作）、关联订阅数、组内用户数、
             操作（编辑/重新选定/删除）；needs_reselect 行警示底色高亮 + 「重新选定」快捷按钮 -->
      </Table>
    </TriStateList>
    <!-- 编辑弹窗 / 删除 ConfirmModal（文案含用户数） -->
  </template>

  <style scoped>
  /* needs_reselect 高亮（警示底色，不随主题变化的浅色警示） */
  :deep(.row-warn) { background-color: #fff7e6; }
  :global(.dark) :deep(.row-warn) { background-color: #2b2111; }
  </style>
  ```

  **5. 单元测试要点（验收要求）**

  - 组名唯一：同名创建/改名 409。
  - 默认组不可删：ErrDefaultGroup。
  - 取消关联时选定校验：组正选定订阅 A，移除 A 的关联 → 拒绝；先在选定区改选后再移除 → 成功。
  - 删组迁入默认组：组内用户 group_id 全部指向默认组；Token 记录不受影响。
  - 删订阅置 needs_reselect：OnSubscriptionDeleted 后受影响组标记置 1、选定清空；重新 SetSelections 后清除。
  - 每组每平台唯一选定：同组同平台重复 UPSERT 不产生多行。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：组名唯一、默认组不可删、取消关联时选定校验拒绝、删组迁入默认组、删订阅置 needs_reselect、重新选定清除标记、每组每平台唯一选定。
  - 手动验证：创建组 → 关联订阅 → 每平台选定 → 删除被选定的订阅 → 组置 needs_reselect 且选定清空 → 重新选定清除标记；删除组用户迁入默认组。

---

### Step 4：下载 Token 体系与统一下载端点

**本 Step 完成后，系统应具备：三类下载 Token（用户三态/分享/规则）、Token 实时解析、统一下载端点（禁缓存/404 不泄露/业务错误注释块）、下载限流与访问日志的能力。**

- **目标：** 实现下载 Token 全生命周期与统一下载分发。
- **前置条件：** Step 2（订阅+版本）、Step 3（组选定）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/1004_tokens.sql`**：
     - `download_tokens` 表（用户下载 Token）：`id`、`token`（TEXT UNIQUE，≥128 位随机值）、`user_id`、`platform_id`、`custom_sub_id`（可空）、`subscription_id`（可空）、`created_at`、`updated_at`。**custom_sub_id 与 subscription_id 互斥且可不填（三态）**。**复用键唯一性由业务层事务内先查后建保证，不依赖数据库唯一索引（可选标识为 NULL 时索引唯一语义不生效）**。
     - `share_tokens` 表：`id`、`token`（UNIQUE）、`share_id`、`created_at`（每分享至多一份有效，本 Step 建表，Step 5 使用）。
     - `rule_tokens` 表：`id`、`token`（UNIQUE）、`rule_id`、`created_at`、`refreshed_at`（每规则一份，Step 6 使用）。
     - `access_logs` 表：`id`、`user_id`（可空）、`ip`、`download_type`、`platform`（可空）、`resource_slug`、`status`、`fail_reason`、`created_at`。90 天自动清理。

  2. **创建 `backend/internal/token/`（业务层，关键约束）**：Token 服务。必须实现：
     - **生成**：≥128 位加密安全随机值。
     - **用户 Token 三态**：
       - 无标识（组解析）：复用键 user+platform；下载时实时解析「用户所属组 → 组在该平台选定订阅 → 返回内容」。
       - 自定义：复用键 user+platform+custom_sub_id；直接返回自定义内容。
       - 显式（管理员预览）：复用键 user+platform+subscription_id；**下载端点实时校验持有人当前仍为管理员**。
     - **并发首建**：**单个 `BEGIN IMMEDIATE` 事务内先查后建**，复用键命中即复用；并发由事务串行化 + 冲突重试兜底。
     - **生命周期联动（Design1 §4.2，必须实现）**：组级切换选定/用户换组/删组迁入**无需清 Token**（实时解析）；上传/覆盖自定义订阅 → 删该用户在该平台无标识 Token；删除订阅 → 级联删指向它的 Token；删除自定义 → 删 custom_sub_id 指向的 Token；角色降级 → 删全部显式 Token；删除用户 → 删全部 Token；**禁用用户 → 同一事务内递增 credential_version + 物理删除其全部 Token**。
     - **刷新 Token**：轮替（旧失效新生效）；**吊销**：物理删除记录。
     - 分享/规则 Token：创建时自动生成；刷新=物理轮替（旧删新写同事务）。

  3. **创建 `backend/internal/download/`（业务层）**：下载解析服务。必须实现：
     - 按 Token 查记录 → 按三态解析目标内容 → 读当前版本文件返回。
     - **URL 路径中的平台/资源标识必须与 Token 绑定一致，不一致与无效 Token 同等对待（404 + `token_invalid`），不泄露差异信息**。
     - **无标识 Token 解析到空（组未选定）→ 返回 HTTP 200 + `# error: unassigned` 纯文本注释块（text/plain + 禁缓存头），访问日志记失败**。
     - 附加响应头注入（平台级，`{frontend_url}` 占位符替换为当前前端地址）。
     - **分享/规则下载默认返回 `Content-Disposition`**（文件名取资源名称或标识，去除/转义控制字符与引号），避免另存文件名退化为 `download`。

  4. **创建下载端点（接入层，`backend/internal/server/download.go`）**：
     - `GET /subscriptions/{平台标识}/download?token=`（用户三态）。
     - `GET /share/{资源标识}/download?token=`（分享，Step 5 接通）。
     - `GET /rules/{资源标识}/download?token=`（规则，Step 6 接通）。
     - **会话凭据预览端点**（独立鉴权）：`GET /api/subscriptions/preview?platform=&subscription_id=`（Bearer 会话；管理员可指定 subscription_id 预览池内任意订阅；**非管理员传 subscription_id 一律忽略**；普通用户预览跟随分发优先级：有自定义返回自定义，否则返回组选定，未分配返 `# error: unassigned`）。
     - 全部下载端点：`text/plain`、禁缓存头、**下载限流（默认 20/min 按 IP）**、记访问日志（含成功/失败与原因）。**无效/过期 Token 统一 404**。
     - 访问日志 90 天定时清理（后台任务）。

  5. **创建用户端数据端点（接入层，`backend/internal/server/home.go`）**：
     - `GET /api/home/platforms`：返回当前用户可见平台卡片数据（普通用户=所属组选定订阅/自定义订阅/未分配；管理员=池内全部订阅），**直接携带该用户可用下载 Token 返回，无 Token 时按需生成**（Design1 §5.2）。
     - `POST /api/home/token/refresh`：刷新指定平台下载 Token（旧失效）。
     - `GET /api/home/updated_at`：订阅更新时间戳（普通用户=可见订阅最大值；管理员=全池最大值；无可见订阅返回空）。

- **参考代码/伪代码：**

  > 编写顺序：1004_tokens.sql → internal/token（三态 + 先查后建事务 + 生命周期联动）→ internal/download（解析/附加头）→ server/download.go + home.go。本 Step 无前端（用户端页面在 Step 6）。

  **1. `backend/migrations/1004_tokens.sql`**

  ```sql
  -- 用户下载 Token（三态：custom_sub_id 与 subscription_id 互斥且可不填）
  CREATE TABLE download_tokens (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      token           TEXT NOT NULL UNIQUE,    -- ≥128 位加密安全随机值
      user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
      custom_sub_id   INTEGER REFERENCES custom_subscriptions(id) ON DELETE CASCADE,
      subscription_id INTEGER REFERENCES subscriptions(id) ON DELETE CASCADE,
      created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_dt_user_platform ON download_tokens(user_id, platform_id);
  -- 注：复用键唯一性由业务层事务内先查后建保证，不建唯一索引（可选标识 NULL 时索引唯一语义不生效，Design1 §4.2）

  CREATE TABLE share_tokens (
      id         INTEGER PRIMARY KEY AUTOINCREMENT,
      token      TEXT NOT NULL UNIQUE,
      share_id   INTEGER NOT NULL REFERENCES share_subscriptions(id) ON DELETE CASCADE,
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  ); -- Step 5 使用

  CREATE TABLE rule_tokens (
      id           INTEGER PRIMARY KEY AUTOINCREMENT,
      token        TEXT NOT NULL UNIQUE,
      rule_id      INTEGER NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
      created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      refreshed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  ); -- Step 6 使用

  -- 访问日志（90 天自动清理）
  CREATE TABLE access_logs (
      id            INTEGER PRIMARY KEY AUTOINCREMENT,
      user_id       INTEGER,               -- 分享/规则下载可空
      ip            TEXT NOT NULL,
      download_type TEXT NOT NULL,         -- subscription/custom/explicit/share/rule
      platform      TEXT,                  -- 平台标识
      resource_slug TEXT NOT NULL,         -- 记录口径见 Build3 Step 5
      status        TEXT NOT NULL,         -- success/fail
      fail_reason   TEXT,                  -- token_invalid/unassigned/...
      created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_access_logs_created ON access_logs(created_at);
  ```

  **2. `backend/internal/token/`（业务层：Token 服务，关键约束）**

  ```go
  type Service struct {
      store *store.Store
      log   *slog.Logger
  }

  // generate：≥128 位加密安全随机值（32 字节 = 256 位，base64url）
  func generate() (string, error) {
      b := make([]byte, 32)
      if _, err := rand.Read(b); err != nil {
          return "", fmt.Errorf("生成 Token 失败: %w", err)
      }
      return base64.RawURLEncoding.EncodeToString(b), nil
  }

  type UserToken struct {
      ID             int64
      Token          string
      UserID         int64
      PlatformID     int64
      CustomSubID    int64 // 0 = NULL
      SubscriptionID int64 // 0 = NULL
  }

  // GetOrCreateUserToken：并发首建——单个 BEGIN IMMEDIATE 事务内先查后建，复用键命中即复用（Design1 §4.2）
  // 复用键：无标识 user+platform；自定义 user+platform+custom_sub_id；显式 user+platform+subscription_id
  func (s *Service) GetOrCreateUserToken(ctx context.Context, userID, platformID, customSubID, subscriptionID int64) (*UserToken, error) {
      if customSubID != 0 && subscriptionID != 0 {
          return nil, errors.New("custom_sub_id 与 subscription_id 互斥")
      }
      var t *UserToken
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          // 先查（NULL 语义：可选标识为 0 时匹配 IS NULL）
          row := tx.QueryRowContext(ctx,
              `SELECT id, token, user_id, platform_id,
                      COALESCE(custom_sub_id,0), COALESCE(subscription_id,0)
               FROM download_tokens
               WHERE user_id = ? AND platform_id = ?
                 AND COALESCE(custom_sub_id,0) = ? AND COALESCE(subscription_id,0) = ?`,
              userID, platformID, customSubID, subscriptionID)
          var found UserToken
          if err := row.Scan(&found.ID, &found.Token, &found.UserID, &found.PlatformID, &found.CustomSubID, &found.SubscriptionID); err == nil {
              t = &found // 复用键命中 → 复用既有 Token
              return nil
          } else if !errors.Is(err, sql.ErrNoRows) {
              return err
          }
          // 后建（冲突重试兜底：UNIQUE(token) 失败时重新生成）
          for attempt := 0; attempt < 3; attempt++ {
              value, err := generate()
              if err != nil {
                  return err
              }
              _, err = tx.ExecContext(ctx,
                  `INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES (?,?,?,?,?)`,
                  value, userID, platformID, nullIf0(customSubID), nullIf0(subscriptionID))
              if err == nil {
                  t = &UserToken{Token: value, UserID: userID, PlatformID: platformID, CustomSubID: customSubID, SubscriptionID: subscriptionID}
                  return nil
              }
          }
          return errors.New("Token 创建冲突超过重试上限")
      })
      return t, err
  }

  // --- 生命周期联动（Design1 §4.2，全部物理删除，无标记态）---

  // DeleteGroupTokens：上传/覆盖自定义订阅 → 删该用户在该平台无标识 Token
  func (s *Service) DeleteGroupTokens(ctx context.Context, userID, platformID int64) error {
      _, err := s.store.DB().ExecContext(ctx,
          `DELETE FROM download_tokens WHERE user_id = ? AND platform_id = ? AND custom_sub_id IS NULL AND subscription_id IS NULL`,
          userID, platformID)
      return err
  }

  // DeleteBySubscriptionTx：删订阅 → 级联删指向它的 Token（含显式预览 Token），在删订阅事务内调用
  func (s *Service) DeleteBySubscriptionTx(ctx context.Context, tx *sql.Tx, subscriptionID int64) error {
      _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE subscription_id = ?`, subscriptionID)
      return err
  }

  // DeleteByCustomTx：删自定义 → 删 custom_sub_id 指向的 Token
  func (s *Service) DeleteByCustomTx(ctx context.Context, tx *sql.Tx, customID int64) error {
      _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE custom_sub_id = ?`, customID)
      return err
  }

  // DeleteExplicit：角色降级（admin→user）→ 清全部显式 Token
  func (s *Service) DeleteExplicit(ctx context.Context, userID int64) error {
      _, err := s.store.DB().ExecContext(ctx,
          `DELETE FROM download_tokens WHERE user_id = ? AND subscription_id IS NOT NULL`, userID)
      return err
  }

  // DeleteAllForUserTx：删除用户/禁用用户 → 物理删全部 Token（禁用时与 credential_version 递增同一事务）
  func (s *Service) DeleteAllForUserTx(ctx context.Context, tx *sql.Tx, userID int64) error {
      _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE user_id = ?`, userID)
      return err
  }

  // RefreshUserToken：轮替（旧失效新生效）——同事务删旧建新，复用键不变
  func (s *Service) RefreshUserToken(ctx context.Context, tokenValue string) (*UserToken, error) {
      var t *UserToken
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var rec UserToken
          if err := tx.QueryRowContext(ctx,
              `SELECT id, user_id, platform_id, COALESCE(custom_sub_id,0), COALESCE(subscription_id,0)
               FROM download_tokens WHERE token = ?`, tokenValue).
              Scan(&rec.ID, &rec.UserID, &rec.PlatformID, &rec.CustomSubID, &rec.SubscriptionID); err != nil {
              return ErrTokenNotFound
          }
          if _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE id = ?`, rec.ID); err != nil {
              return err
          }
          // 同复用键重建（直接 INSERT，事务内无并发）
          value, err := generate()
          if err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx,
              `INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES (?,?,?,?,?)`,
              value, rec.UserID, rec.PlatformID, nullIf0(rec.CustomSubID), nullIf0(rec.SubscriptionID)); err != nil {
              return err
          }
          t = &UserToken{Token: value, UserID: rec.UserID, PlatformID: rec.PlatformID, CustomSubID: rec.CustomSubID, SubscriptionID: rec.SubscriptionID}
          return nil
      })
      return t, err
  }

  // 分享/规则 Token（Step 5/6 使用）：创建时自动生成；刷新=物理轮替（旧删新写同事务）；吊销=物理删除
  func (s *Service) CreateShareTokenTx(ctx context.Context, tx *sql.Tx, shareID int64) (string, error) { /* generate + INSERT share_tokens */ }
  func (s *Service) RotateShareToken(ctx context.Context, shareID int64) (string, error)           { /* 事务内 DELETE + INSERT */ }
  func (s *Service) CreateRuleTokenTx(ctx context.Context, tx *sql.Tx, ruleID int64) (string, error) { /* 同上 */ }
  func (s *Service) RotateRuleToken(ctx context.Context, ruleID int64) (string, error)             { /* 同上 + refreshed_at */ }
  ```

  **3. `backend/internal/download/`（业务层：下载解析服务）**

  ```go
  type Service struct {
      store    *store.Store
      versions *version.Service
      cfg      *config.Service
      log      *slog.Logger
  }

  type Result struct {
      Content      []byte
      ExtraHeaders map[string]string // 平台级附加头（{frontend_url} 已替换）
  }

  var (
      ErrTokenInvalid = errors.New("token_invalid")  // 无效/标识不一致 → 404
      ErrUnassigned   = errors.New("unassigned")     // 组未选定 → HTTP 200 注释块
  )

  // ResolveUserDownload：按 Token 查记录 → 三态解析；
  // URL 路径中的平台标识必须与 Token 绑定一致，不一致与无效 Token 同等对待（ErrTokenInvalid，不泄露差异）
  func (s *Service) ResolveUserDownload(ctx context.Context, tokenValue, platformSlug string) (*Result, *AccessEntry, error) {
      var rec struct {
          UserID         int64
          PlatformID     int64
          PlatformSlug   string
          CustomSubID    int64
          SubscriptionID int64
      }
      err := s.store.DB().QueryRowContext(ctx,
          `SELECT dt.user_id, dt.platform_id, p.slug, COALESCE(dt.custom_sub_id,0), COALESCE(dt.subscription_id,0)
           FROM download_tokens dt JOIN platforms p ON p.id = dt.platform_id WHERE dt.token = ?`, tokenValue).
          Scan(&rec.UserID, &rec.PlatformID, &rec.PlatformSlug, &rec.CustomSubID, &rec.SubscriptionID)
      if errors.Is(err, sql.ErrNoRows) || rec.PlatformSlug != platformSlug {
          return nil, &AccessEntry{Platform: platformSlug, FailReason: "token_invalid"}, ErrTokenInvalid
      }
      if err != nil {
          return nil, nil, err
      }
      switch {
      case rec.CustomSubID != 0: // 自定义：直接返回自定义内容
          content, err := s.versions.ReadCurrent(ctx, version.OwnerCustom, rec.CustomSubID)
          if err != nil {
              return nil, nil, err
          }
          return s.withPlatformHeaders(ctx, content, rec.PlatformID, "custom", rec.CustomSubID)
      case rec.SubscriptionID != 0: // 显式：实时校验持有人当前仍为管理员
          var role string
          if err := s.store.DB().QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, rec.UserID).Scan(&role); err != nil || role != "admin" {
              return nil, &AccessEntry{FailReason: "token_invalid"}, ErrTokenInvalid
          }
          content, err := s.versions.ReadCurrent(ctx, version.OwnerSubscription, rec.SubscriptionID)
          if err != nil {
              return nil, nil, err
          }
          return s.withPlatformHeaders(ctx, content, rec.PlatformID, "explicit", rec.SubscriptionID)
      default: // 无标识：实时解析「用户所属组 → 组在该平台选定 → 内容」
          var subID int64
          err := s.store.DB().QueryRowContext(ctx,
              `SELECT gs.subscription_id FROM users u
               JOIN group_selections gs ON gs.group_id = u.group_id AND gs.platform_id = ?
               WHERE u.id = ? AND u.group_id IS NOT NULL AND gs.subscription_id IS NOT NULL`,
              rec.PlatformID, rec.UserID).Scan(&subID)
          if errors.Is(err, sql.ErrNoRows) {
              return nil, &AccessEntry{Platform: platformSlug, FailReason: "unassigned"}, ErrUnassigned
          }
          if err != nil {
              return nil, nil, err
          }
          content, err := s.versions.ReadCurrent(ctx, version.OwnerSubscription, subID)
          if err != nil {
              return nil, nil, err
          }
          return s.withPlatformHeaders(ctx, content, rec.PlatformID, "subscription", subID)
      }
  }

  // withPlatformHeaders：附加响应头注入（{frontend_url} 占位符替换为当前前端地址）
  func (s *Service) withPlatformHeaders(ctx context.Context, content []byte, platformID int64, dlType string, resID int64) (*Result, *AccessEntry, error) {
      headers := map[string]string{}
      // SELECT extra_headers FROM platforms WHERE id = ? → JSON 解析 → 逐值替换 {frontend_url}（读 frontend_url 配置）
      ...
      return &Result{Content: content, ExtraHeaders: headers}, &AccessEntry{Type: dlType, ResourceID: resID}, nil
  }

  // ResolveShare / ResolveRule（Step 5/6 接通）：token 查 share_tokens/rule_tokens → 读当前版本；
  // 默认返回 Content-Disposition（文件名取资源名称或标识，去除/转义控制字符与引号）
  func sanitizeFilename(name string) string {
      name = strings.Map(func(r rune) rune {
          if r < 0x20 || r == 0x7f || r == '"' || r == '\\' {
              return '_'
          }
          return r
      }, name)
      if name == "" {
          name = "download"
      }
      return name
  }

  // AccessEntry：访问日志写入参数；口径见 Build3 Step 5（显式/自定义记订阅标识；无标识记解析结果，unassigned 记平台标识）
  type AccessEntry struct {
      UserID      int64
      Type        string
      Platform    string
      ResourceID  int64 // 由写入时转换为 slug
      FailReason  string
  }

  // WriteAccessLog：成功/失败均记；status=success/fail
  func (s *Service) WriteAccessLog(ctx context.Context, ip string, e *AccessEntry, success bool) {
      // INSERT access_logs；写入失败仅记 warn 日志，不阻断下载响应
  }
  ```

  **4. `backend/internal/server/download.go`（下载端点，接入层）**

  ```go
  type DownloadHandler struct {
      dlSvc     *download.Service
      limiter   *ratelimit.Limiter
      sessionMW gin.HandlerFunc
  }

  func RegisterDownloadRoutes(engine *gin.Engine, h *DownloadHandler) {
      // 全部下载端点：限流（20/min 按 IP，ratelimit_download 配置）+ 禁缓存 + text/plain + 访问日志
      dl := engine.Group("", h.limiter.Middleware("download", ratelimit.KeyDownload, 20))
      dl.GET("/subscriptions/:platform/download", h.userDownload)
      dl.GET("/share/:slug/download", h.shareDownload)   // Step 5 接通解析
      dl.GET("/rules/:slug/download", h.ruleDownload)    // Step 6 接通解析
      engine.GET("/api/subscriptions/preview", h.sessionMW, h.preview) // 会话凭据预览（独立鉴权）
  }

  func setNoCache(c *gin.Context) {
      c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
      c.Header("Pragma", "no-cache")
  }

  func (h *DownloadHandler) userDownload(c *gin.Context) {
      ctx := c.Request.Context()
      res, entry, err := h.dlSvc.ResolveUserDownload(ctx, c.Query("token"), c.Param("platform"))
      ip := c.ClientIP()
      switch {
      case errors.Is(err, download.ErrTokenInvalid):
          h.dlSvc.WriteAccessLog(ctx, ip, entry, false)
          setNoCache(c)
          server.Fail(c, http.StatusNotFound, "资源不存在") // 统一 404，不泄露资源存在性
      case errors.Is(err, download.ErrUnassigned):
          h.dlSvc.WriteAccessLog(ctx, ip, entry, false)
          setNoCache(c)
          c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: unassigned\n")) // HTTP 200 纯文本注释块
      case err != nil:
          server.Fail(c, http.StatusInternalServerError, err.Error())
      default:
          h.dlSvc.WriteAccessLog(ctx, ip, entry, true)
          setNoCache(c)
          for k, v := range res.ExtraHeaders {
              c.Header(k, v)
          }
          c.Data(http.StatusOK, "text/plain; charset=utf-8", res.Content)
      }
  }

  // shareDownload/ruleDownload：同模式（Step 5/6 接通 ResolveShare/ResolveRule，附 Content-Disposition）

  // preview：会话凭据预览（Design1 §4.3）
  func (h *DownloadHandler) preview(c *gin.Context) {
      ctx := c.Request.Context()
      userID := c.GetInt64(auth.CtxUserID)
      role := c.GetString(auth.CtxUserRole)
      platformSlug := c.Query("platform")
      subIDParam := c.Query("subscription_id")
      // 管理员可指定 subscription_id 预览池内任意订阅；非管理员传 subscription_id 一律忽略
      if role == "admin" && subIDParam != "" {
          // 显式预览：读指定订阅当前版本（text/plain + no-store）
          ...
          return
      }
      // 普通用户：跟随分发优先级——有自定义返回自定义，否则组选定，未分配返 `# error: unassigned`
      ...
  }
  ```

  **5. `backend/internal/server/home.go`（用户端数据端点；会话）**

  ```go
  type HomeHandler struct {
      store    *store.Store
      tokenSvc *token.Service
      dlSvc    *download.Service
  }

  func RegisterHomeRoutes(engine *gin.Engine, h *HomeHandler, sessionMW gin.HandlerFunc) {
      g := engine.Group("/api/home", sessionMW)
      g.GET("/platforms", h.platforms)
      g.POST("/token/refresh", h.refreshToken)
      g.GET("/updated_at", h.updatedAt)
  }

  // platforms：当前用户可见平台卡片数据，直接携带可用下载 Token，无 Token 时按需生成（Design1 §5.2）
  func (h *HomeHandler) platforms(c *gin.Context) {
      ctx := c.Request.Context()
      userID := c.GetInt64(auth.CtxUserID)
      role := c.GetString(auth.CtxUserRole)
      // 管理员：池内全部订阅（预览用，生成/复用显式 Token）
      // 普通用户：逐平台判定——有自定义→自定义 Token；否则→无标识 Token（组解析）；未选定→仍返无标识 Token（下载时返 unassigned）
      // 每张卡片：平台名称/描述/schemes/安装包来源/订阅状态（group_selected/custom/unassigned/admin_pool）+ download_token
      // Token 统一经 tokenSvc.GetOrCreateUserToken（事务先查后建，并发安全）
      ...
  }

  // refreshToken：刷新指定平台下载 Token（旧失效）——先查该用户该平台当前有效 Token（自定义优先）再轮替
  func (h *HomeHandler) refreshToken(c *gin.Context) { /* body: { platform_id } → RefreshUserToken → 返回新 token */ }

  // updatedAt：普通用户=可见订阅 versions.updated_at 最大值；管理员=全池最大值；无可见订阅返回空
  func (h *HomeHandler) updatedAt(c *gin.Context) { /* SELECT MAX(updated_at) FROM versions WHERE owner_type='subscription' AND owner_id IN (可见集合) */ }
  ```

  **6. 访问日志 90 天定时清理（后台任务）**

  ```go
  // backend/internal/cron/cleanup.go（或在 main 装配处启动 goroutine）
  func StartAccessLogCleanup(db *sql.DB, lg *slog.Logger) (stop func()) {
      ticker := time.NewTicker(24 * time.Hour)
      done := make(chan struct{})
      go func() {
          for {
              select {
              case <-ticker.C:
                  cutoff := time.Now().AddDate(0, 0, -90)
                  if _, err := db.Exec(`DELETE FROM access_logs WHERE created_at < ?`, cutoff); err != nil {
                      lg.Error("清理访问日志失败", "err", err)
                  }
              case <-done:
                  ticker.Stop()
                  return
              }
          }
      }()
      return func() { close(done) }
  }
  ```

  **7. 单元测试要点（验收要求）**

  - 三态解析：无标识（组选定内容）/自定义（直接内容）/显式（校验管理员，降级后 404）。
  - 复用键先查后建：同参数二次调用返回同一 Token；并发 N 个 GetOrCreate 只产生一条记录（BEGIN IMMEDIATE 串行化）。
  - 实时解析无感生效：组切换选定/用户换组/删组迁入后，同一 Token 返回新内容（无需清 Token）。
  - 禁用用户：同事务内 credential_version 递增 + Token 全删（旧 Token 下载 404）。
  - 降级：DeleteExplicit 后显式 Token 清空、无标识 Token 保留。
  - URL 标识与 Token 不一致 → ErrTokenInvalid（404），与无效 Token 无差异。
  - unassigned：HTTP 200 + `# error: unassigned` + text/plain + no-store；访问日志记 fail/unassigned。
  - 限流：第 21 次/分钟 429 + Retry-After。
  - 访问日志：成功/失败均写入，含原因。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：三态 Token 解析、复用键先查后建并发安全、组切换/换组/删组无感生效（实时解析）、禁用用户同事务清 Token、降级清显式 Token、URL 标识与 Token 不一致 404、unassigned 返回 200 注释块、无效 Token 404、下载限流 429、访问日志记录。
  - 手动验证：用户首页数据返回 Token → 用 Token 下载当前版本内容 → 组切换选定后同 Token 返回新内容 → 刷新 Token 旧链接 404 → 未分配平台返回 `# error: unassigned`。

---

### Step 5：自定义订阅与分享订阅

**本 Step 完成后，系统应具备：管理员为用户+平台上传自定义订阅（覆盖组分配）、分享订阅 CRUD 与 Token 刷新/吊销（含吊销矩阵）的能力。**

- **目标：** 实现自定义订阅覆盖机制与独立分享订阅。
- **前置条件：** Step 2（版本组件）、Step 4（Token 体系）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/1005_custom_share.sql`**：
     - `custom_subscriptions` 表：`id`、`slug`（UNIQUE，`custom-`+8 短码自动生成）、`user_id`、`platform_id`、`current_version`、**UNIQUE(user_id, platform_id)**（每用户每平台最多一份）、`created_at`、`updated_at`。
     - `share_subscriptions` 表：`id`、`slug`（UNIQUE，`share-`+8 短码自动生成）、`name`（不强制唯一）、`current_version`、`token_status`（TEXT，`active`/`revoked`）、`created_at`、`updated_at`。

  2. **创建 `backend/internal/custom/`（业务层）**：自定义订阅服务。必须实现：
     - 上传/覆盖：指定用户 + 平台 + 内容（文件/文本）；**每用户每平台最多一份，再次上传即覆盖——复用原记录与标识，仅创建新版本（Token 复用键 user+platform+custom_sub_id 保持稳定）**。
     - **上传/覆盖时删除该用户在该平台原有的无标识（组解析）Token**（旧链接立即失效）。
     - 版本管理复用 Step 2 版本组件（owner_type=custom）。
     - **删除自定义订阅**：级联删 custom_sub_id 指向的 Token 与版本文件，用户下次访问首页重新生成无标识 Token。
     - 标识自动生成（`custom-` 前缀，参与四类命名空间校验）。

  3. **创建 `backend/internal/share/`（业务层）**：分享订阅服务。必须实现：
     - 创建：名称 + 首版本上传 → 自动生成标识（`share-` 前缀）与分享 Token。
     - **改名**（创建后仅可改名）、版本管理（复用版本组件 owner_type=share）。
     - **刷新 Token**：物理轮替（旧删新写同事务）。
     - **吊销 Token**：**物理删除 Token 记录 + 置 `token_status=revoked`**，链接立即失效，文件与版本保留；**刷新时清除 revoked 标记并新建 Token（恢复手段）**。
     - **删除**：级联删文件 + Token。
     - 分享下载走 Step 4 的 `/share/{slug}/download?token=`（公开，无需登录）。

  4. **接通 Step 1 平台删除的完整级联**：平台删除时级联删自定义订阅（含版本文件）与相关 Token。补充完整级联测试。

  5. **创建端点（接入层）**：
     - `backend/internal/server/custom.go`：`POST /api/admin/users/:id/custom`（上传/覆盖自定义订阅，管理员）、`DELETE /api/admin/users/:id/custom/:platform`（删除）、自定义订阅版本端点（复用版本路由模式 `/api/admin/customs/:id/versions/...`）。
     - `backend/internal/server/share.go`：`GET/POST /api/admin/shares`、`PUT /:id`（改名）、`DELETE /:id`、`POST /:id/token/refresh`、`POST /:id/token/revoke`、版本端点（`/api/admin/shares/:id/versions/...`）。均会话 + 管理员。

  6. **创建前端**：
     - `frontend/src/views/admin/SharesView.vue`（UI §5.3）：双态列表——名称、创建时间、当前版本、Token 状态（`a-badge` 有效绿/已吊销红）；操作：改名、版本管理（复用 VersionManageView）、复制分享链接（仅 Token 有效时可用）、刷新 Token（ConfirmModal）、吊销（ConfirmModal 危险）、删除（ConfirmModal 危险）；**已吊销状态按钮矩阵：复制置灰、刷新可用（恢复）、改名/版本/删除不受影响**。创建对话框：名称 + 首版本上传（文件/文本页签）→ 成功后显著展示自动生成标识供复制。
     - 自定义订阅的版本管理复用 `VersionManageView`（路由 `/admin/customs/:id/versions`，从用户管理入口进入，**用户管理界面在 Build3，本 Step 仅保证版本组件支持 owner_type=custom 并预留路由**）。
     - `frontend/src/api/{custom,share}.ts`。

- **参考代码/伪代码：**

  > 编写顺序：1005 迁移 → internal/custom（覆盖复用）→ internal/share（吊销矩阵）→ 接通平台级联 → 端点 → SharesView。复用：版本组件（owner_type=custom/share）、Token 服务、slug 生成器。

  **1. `backend/migrations/1005_custom_share.sql`**

  ```sql
  CREATE TABLE custom_subscriptions (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      slug            TEXT NOT NULL UNIQUE,          -- custom- + 8 短码自动生成（四类命名空间）
      user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      platform_id     INTEGER NOT NULL REFERENCES platforms(id) ON DELETE CASCADE,
      current_version INTEGER NOT NULL DEFAULT 0,
      created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      UNIQUE (user_id, platform_id)                  -- 每用户每平台最多一份
  );

  CREATE TABLE share_subscriptions (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      slug            TEXT NOT NULL UNIQUE,          -- share- + 8 短码自动生成（四类命名空间）
      name            TEXT NOT NULL,                 -- 不强制唯一；创建后仅可改名
      current_version INTEGER NOT NULL DEFAULT 0,
      token_status    TEXT NOT NULL DEFAULT 'active' CHECK (token_status IN ('active','revoked')),
      created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  ```

  **2. `backend/internal/custom/`（业务层：自定义订阅）**

  ```go
  type Service struct {
      store    *store.Store
      versions *version.Service
      tokens   *token.Service
      log      *slog.Logger
  }

  type Custom struct {
      ID        int64
      Slug      string
      UserID    int64
      PlatformID int64
      CurrentVersion int64
  }

  // Upsert：上传/覆盖——每用户每平台最多一份，再次上传即覆盖：
  // 复用原记录与标识，仅创建新版本（Token 复用键 user+platform+custom_sub_id 保持稳定，Design1 §2.3）
  func (s *Service) Upsert(ctx context.Context, userID, platformID int64, src version.ContentProvider) (*Custom, error) {
      var c *Custom
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var id int64
          err := tx.QueryRowContext(ctx,
              `SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, platformID).Scan(&id)
          if errors.Is(err, sql.ErrNoRows) {
              // 首次创建：生成标识（custom- 前缀，参与四类命名空间校验）
              cslug, err := slug.Generate(ctx, tx, "custom-", func(v string) (bool, error) {
                  // 四类查重（沿用 Step 2 语义：rules 表 Step 6 才建，缺失则跳过，Step 6 后全量生效）
                  return slug.ExistsInFourTables(ctx, tx, v)
              })
              if err != nil {
                  return err
              }
              res, err := tx.ExecContext(ctx,
                  `INSERT INTO custom_subscriptions (slug, user_id, platform_id) VALUES (?,?,?)`, cslug, userID, platformID)
              if err != nil {
                  return fmt.Errorf("创建自定义订阅失败: %w", err)
              }
              id, _ = res.LastInsertId()
          } else if err != nil {
              return err
          }
          c = &Custom{ID: id, UserID: userID, PlatformID: platformID}
          return nil
      })
      if err != nil {
          return nil, err
      }
      // 版本创建在独立事务（版本组件自带 BEGIN IMMEDIATE）；失败需回滚首次创建的空记录
      v, err := s.versions.CreateVersion(ctx, version.OwnerCustom, c.ID, src)
      if err != nil {
          s.rollbackEmptyRecord(ctx, c.ID) // 失败清理模式：首建且无版本时删记录
          return nil, err
      }
      c.CurrentVersion = v.No
      // 上传/覆盖后：删该用户在该平台原有的无标识（组解析）Token（旧链接立即失效）
      if err := s.tokens.DeleteGroupTokens(ctx, userID, platformID); err != nil {
          s.log.Error("删除无标识 Token 失败", "err", err) // 不阻断，记日志（下次下载时自定义优先级仍生效）
      }
      return c, nil
  }

  // Delete：级联删 custom_sub_id 指向的 Token + 版本文件；用户下次访问首页重新生成无标识 Token
  func (s *Service) Delete(ctx context.Context, userID, platformID int64) error {
      var id int64
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if err := tx.QueryRowContext(ctx,
              `SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, platformID).Scan(&id); err != nil {
              return err
          }
          if err := s.tokens.DeleteByCustomTx(ctx, tx, id); err != nil { // 级联删指向它的 Token
              return err
          }
          if _, err := tx.ExecContext(ctx, `DELETE FROM versions WHERE owner_type = 'custom' AND owner_id = ?`, id); err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx, `DELETE FROM custom_subscriptions WHERE id = ?`, id); err != nil {
              return err
          }
          return nil
      })
      if err != nil {
          return err
      }
      s.versions.RemoveOwnerDir(version.OwnerCustom, id) // 事务提交后删版本目录（失败记日志）
      return nil
  }
  ```

  **3. `backend/internal/share/`（业务层：分享订阅）**

  ```go
  type Service struct {
      store    *store.Store
      versions *version.Service
      tokens   *token.Service
      log      *slog.Logger
  }

  type Share struct {
      ID          int64
      Slug        string
      Name        string
      TokenStatus string // active/revoked
      Token       string // 有效时返回（吊销后为空）
      CurrentVersion int64
  }

  // Create：名称 + 首版本上传 → 自动生成标识（share- 前缀）与分享 Token（同一事务语义）
  func (s *Service) Create(ctx context.Context, name string, src version.ContentProvider) (*Share, error) {
      var sh *Share
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          sslug, err := slug.Generate(ctx, tx, "share-", func(v string) (bool, error) {
              // 四类查重（沿用 Step 2 语义：rules 表 Step 6 才建，缺失则跳过）
              return slug.ExistsInFourTables(ctx, tx, v)
          })
          if err != nil {
              return err
          }
          res, err := tx.ExecContext(ctx, `INSERT INTO share_subscriptions (slug, name) VALUES (?,?)`, sslug, name)
          if err != nil {
              return fmt.Errorf("创建分享订阅失败: %w", err)
          }
          id, _ := res.LastInsertId()
          tk, err := s.tokens.CreateShareTokenTx(ctx, tx, id) // 创建时自动生成 Token
          if err != nil {
              return err
          }
          sh = &Share{ID: id, Slug: sslug, Name: name, TokenStatus: "active", Token: tk}
          return nil
      })
      if err != nil {
          return nil, err
      }
      // 首版本创建（版本组件事务）；失败回滚分享记录与 Token（失败清理模式）
      if _, err := s.versions.CreateVersion(ctx, version.OwnerShare, sh.ID, src); err != nil {
          s.rollbackRecord(ctx, sh.ID) // DELETE share_tokens + share_subscriptions
          return nil, err
      }
      return sh, nil
  }

  // Rename：创建后仅可改名
  func (s *Service) Rename(ctx context.Context, id int64, name string) error { /* UPDATE name */ }

  // RefreshToken：物理轮替（旧删新写同事务）；同时清除 revoked 标记并新建 Token（恢复手段，Design1 §3.4.3）
  func (s *Service) RefreshToken(ctx context.Context, id int64) (string, error) {
      var newToken string
      err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          tk, err := s.tokens.RotateShareTokenTx(ctx, tx, id) // Step 4 RotateShareToken 的事务内版本（同 SQL 抽取：DELETE 旧 + INSERT 新）
          if err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx,
              `UPDATE share_subscriptions SET token_status = 'active', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
              return err
          }
          newToken = tk
          return nil
      })
      return newToken, err
  }

  // RevokeToken：物理删除 Token 记录 + 置 token_status=revoked；链接立即失效，文件与版本保留
  func (s *Service) RevokeToken(ctx context.Context, id int64) error {
      return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          if _, err := tx.ExecContext(ctx, `DELETE FROM share_tokens WHERE share_id = ?`, id); err != nil {
              return err
          }
          if _, err := tx.ExecContext(ctx,
              `UPDATE share_subscriptions SET token_status = 'revoked', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
              return err
          }
          return nil
      })
  }

  // Delete：级联删版本文件 + Token
  func (s *Service) Delete(ctx context.Context, id int64) error {
      // 事务内：DELETE share_tokens → DELETE versions 行 → DELETE share_subscriptions；提交后删版本目录
      ...
  }

  // ResolveForDownload（供 Step 4 下载服务接通）：token 查 share_tokens → token_status=active → 读当前版本 + Content-Disposition（文件名取名称/标识）
  ```

  **4. 接通 Build2 Step 1 平台删除的完整级联**

  ```go
  // internal/platform.Delete 事务内替换 TODO：
  //   级联删该平台全部自定义订阅（含版本文件）：
  //   DELETE download_tokens WHERE platform_id = ?（含自定义 Token）→ DELETE versions（owner=custom 对应行）
  //   → DELETE custom_subscriptions WHERE platform_id = ?；提交后删版本目录
  //   （订阅/组关联级联在 Step 2/3 已接通的 TODO 同步替换）
  // 补充完整级联测试：平台删除后 subscriptions/custom_subscriptions/download_tokens/versions 无残留、文件无残留
  ```

  **5. 端点（接入层；会话 + 管理员）**

  ```go
  // server/custom.go
  func RegisterCustomRoutes(engine *gin.Engine, h *CustomHandler, sessionMW, adminMW gin.HandlerFunc) {
      admin := engine.Group("/api/admin", sessionMW, adminMW)
      admin.POST("/users/:id/custom", h.upsert)               // 上传/覆盖：multipart 文件/文本双模式（同版本创建模式）
      admin.DELETE("/users/:id/custom/:platform", h.delete)   // 按用户+平台删除
      // 版本端点复用通用路由模式：
      admin.GET("/customs/:id/versions", h.listVersions)
      admin.POST("/customs/:id/versions", h.createVersion)
      admin.PUT("/customs/:id/versions/current", h.switchVersion)
      admin.GET("/customs/:id/versions/:ver/preview", h.preview)
      admin.DELETE("/customs/:id/versions/:ver", h.deleteVersion)
  }

  // server/share.go
  func RegisterShareRoutes(engine *gin.Engine, h *ShareHandler, sessionMW, adminMW gin.HandlerFunc) {
      admin := engine.Group("/api/admin/shares", sessionMW, adminMW)
      admin.GET("", h.list)                        // 含 token_status；Token 值仅 active 时返回
      admin.POST("", h.create)                     // 名称 + 首版本（文件/文本）
      admin.PUT("/:id", h.rename)                  // 仅改名
      admin.DELETE("/:id", h.delete)
      admin.POST("/:id/token/refresh", h.refresh)  // 轮替（含 revoked 恢复）
      admin.POST("/:id/token/revoke", h.revoke)
      // 版本端点：/api/admin/shares/:id/versions/... 同 custom 模式
  }
  // 分享下载走 Step 4 已注册的 GET /share/:slug/download?token=（公开，无需登录），本 Step 接通 ResolveForDownload
  ```

  **6. 前端**

  ```ts
  // api/share.ts
  export interface ShareItem {
    id: number; slug: string; name: string; token_status: 'active' | 'revoked'
    token: string; current_version: number; created_at: string
  }
  export const listShares = () => http.get<any, ShareItem[]>('/admin/shares')
  export const createShare = (payload: FormData) => http.post('/admin/shares', payload)
  export const renameShare = (id: number, name: string) => http.put(`/admin/shares/${id}`, { name })
  export const deleteShare = (id: number) => http.delete(`/admin/shares/${id}`)
  export const refreshShareToken = (id: number) => http.post<any, { token: string }>(`/admin/shares/${id}/token/refresh`)
  export const revokeShareToken = (id: number) => http.post(`/admin/shares/${id}/token/revoke`)
  // api/custom.ts：upsertCustom(userId, payload) / deleteCustom(userId, platformId)
  ```

  ```vue
  <!-- SharesView.vue 骨架（UI §5.3）：双态列表 -->
  <script setup lang="ts">
  // 列：名称、创建时间、当前版本、Token 状态（a-badge 有效绿/已吊销红）
  // 操作：改名、版本管理（跳 /admin/shares/:id/versions，复用 VersionManageView）、
  //      复制分享链接（仅 active 可用，链接 = {frontend_url}/share/{slug}/download?token=）、
  //      刷新 Token（ConfirmModal）、吊销（ConfirmModal 危险）、删除（ConfirmModal 危险）
  // 已吊销状态按钮矩阵：复制置灰、刷新可用（恢复）、改名/版本/删除不受影响
  const revoked = (s: ShareItem) => s.token_status === 'revoked'
  async function copyLink(s: ShareItem) {
    const url = `${location.origin}/share/${s.slug}/download?token=${s.token}`
    await navigator.clipboard.writeText(url)
    Notify.success('分享链接已复制')
  }
  async function doRefresh(s: ShareItem) {
    await refreshShareToken(s.id)
    Notify.success('Token 已刷新（吊销状态已恢复）')
    await load()
  }
  // 创建对话框：名称 + 首版本上传（文件/文本页签）→ 成功后显著展示自动生成标识供复制
  </script>
  ```

  **7. 单元测试要点（验收要求）**

  - 自定义覆盖：同用户同平台二次 Upsert → 记录 ID 与 slug 不变，版本号 +1；DeleteGroupTokens 被调用（旧无标识 Token 404）。
  - 删自定义级联：custom Token 被删、版本文件清理；之后首页接口重新生成无标识 Token。
  - 分享吊销：RevokeToken 后 share_tokens 无记录且 token_status=revoked，旧链接 404；Refresh 后清标记新建，链接恢复可下载。
  - 平台删除完整级联（含自定义）：平台删除后四类表与文件无残留（回归 Step 1 用例 + 本 Step 新增自定义维度）。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：自定义覆盖复用记录与标识、覆盖删无标识 Token、删自定义级联、分享吊销物理删 + 标记、刷新恢复（清标记新建）、平台删除完整级联（含自定义）。
  - 手动验证：为用户上传自定义订阅 → 该用户该平台返回自定义内容且旧组 Token 失效 → 删自定义恢复组分配；创建分享 → 复制链接公开下载 → 吊销 404 → 刷新恢复可下载。

---

### Step 6：规则管理与用户端页面

**本 Step 完成后，系统应具备：规则 CRUD 与全局 Token、用户端首页（平台卡片/一键导入/复制/刷新/下载客户端）、规则浏览页、个人中心、404 的完整能力。**

- **目标：** 实现规则资源与用户端三大页面，完成用户侧价值链。
- **前置条件：** Step 2（版本组件）、Step 4（Token/下载/首页数据）、Step 5（自定义，首页自定义展示）已完成并验收。
- **产出文件与操作：**

  1. **创建 `backend/migrations/1006_rules.sql`**：`rules` 表：`id`、`slug`（UNIQUE，**系统自动生成**（`rule-` 前缀 + 8 位随机短码），四类命名空间交叉校验；后端兼容手填）、`name`（不强制唯一）、`client_type`（当前仅 `shadowrocket`）、`schemes`（JSON 数组，含 `{url}` 占位符）、`current_version`、`created_at`、`updated_at`。

  2. **创建 `backend/internal/rule/`（业务层）**：规则服务。必须实现：
     - CRUD：名称 + 客户端类型 + scheme + 首版本上传；**标识由系统自动生成（`rule-` 前缀，后端兼容手填校验）**；**创建后仅可改名（客户端类型与 scheme 不可修改）**。
     - **规则 Token 全局共享**（每规则一份，不绑定用户）；**任何持有链接者均可下载，不随用户禁用/删除失效**。
     - 版本管理复用版本组件（owner_type=rule）；Token 刷新=物理轮替；删除级联删文件 + Token。
     - 规则下载走 Step 4 的 `/rules/{slug}/download?token=`（公开）。

  3. **创建规则端点（接入层，`backend/internal/server/rule.go`）**：管理端（会话+管理员）：`GET/POST /api/admin/rules`、`PUT /:id`（改名）、`DELETE /:id`、`POST /:id/token/refresh`、版本端点。用户端（会话）：`GET /api/rules`（规则卡片列表，登录用户）、`GET /api/rules/:id/preview`（会话凭据预览当前版本，需登录）。

  4. **创建用户端页面**：
     - `frontend/src/views/HomeView.vue`（UI §4.1，替换 Build1 占位）：顶部栏（站点名称 + 订阅更新时间戳；右侧管理面板入口（仅管理员）+ 用户名 + 角色标签 + 所属组名 + 暗色切换 + 退出）；**平台卡片网格**（大屏 3 列/中屏 2 列/小屏 1 列）——平台名称/描述、订阅区段（普通用户：组选定一份/未选定灰色占位「未分配，请联系管理员」且三按钮隐藏/有自定义显示 `a-alert info`「已被分配自定义订阅」；管理员：池内全部折叠列表）、操作按钮组（**一键导入**（主按钮，无 scheme 隐藏；多个 scheme 取首项；对下载 URL 做 URL 编码后替换 `{url}`；跳转后无响应提示「请确认已安装对应客户端」）、**复制链接**（弹窗展示 URL + 复制；用户绑定类 Token 复制时弹警示「该链接与您的账号绑定，请勿分享」）、**刷新链接**（ConfirmModal 后刷新））、底部「下载客户端」（本地下载/链接下载并存则两个都显示）。公告栏卡片（有内容才显示，纯文本转义）。分流规则入口卡片。
     - `frontend/src/views/RulesView.vue`（UI §4.2）：`PageHeader`「分流规则」+ `a-alert warning`「规则内容公开，链接请谨慎分发，请勿外发」；规则卡片网格（名称、客户端类型标签、当前版本）；每卡片「下载」（会话凭据，需登录）+「一键导入」（全局规则 Token）。
     - `frontend/src/views/ProfileView.vue`（UI §4.3）：`a-tabs` 三页签——基本信息（`a-descriptions` 展示 + 修改用户名/邮箱行内编辑，邮箱修改成功弹「所有设备会话已失效」跳登录）、密码管理（已设密码需验证当前密码；OIDC 用户首次设置免旧密码）、OIDC 绑定（仅 OIDC 已配置显示；已绑定展示 subject 脱敏，未绑定「绑定 OIDC」跳授权）。
     - `frontend/src/api/{rule,home,profile}.ts`。

  5. **个人中心后端端点（接入层，`backend/internal/server/profile.go`）**：会话：
     - `PUT /api/profile/username`（改用户名，即时生效；OIDC 用户下次 OIDC 登录会被覆盖）。
     - `PUT /api/profile/email`（改邮箱，新邮箱被占用拒绝 409；**成功递增 credential_version，所有设备会话失效**）。
     - `PUT /api/profile/password`（设置/修改密码；已设密码需验证当前密码；OIDC 用户首次设置免旧密码但须已登录；成功递增 credential_version）。

- **参考代码/伪代码：**

  > 编写顺序：1006_rules.sql → internal/rule → server/rule.go + profile.go → 前端三页（HomeView/RulesView/ProfileView）。复用：版本组件（owner_type=rule）、Token 轮替、四类 slug 校验。

  **1. `backend/migrations/1006_rules.sql`**

  ```sql
  CREATE TABLE rules (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      slug            TEXT NOT NULL UNIQUE,    -- 自动生成（rule- 前缀 + 8 位随机短码；兼容手填），四类命名空间交叉校验
      name            TEXT NOT NULL,           -- 不强制唯一
      client_type     TEXT NOT NULL DEFAULT 'shadowrocket', -- 当前仅 shadowrocket；创建后不可改
      schemes         TEXT NOT NULL DEFAULT '[]',           -- JSON 数组，含 {url} 占位符；创建后不可改
      current_version INTEGER NOT NULL DEFAULT 0,
      created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  -- rule_tokens 表已在 Build2 Step 4（1004）建立；slug 查重至此实现全四表生效（替换 Step 2 的跳过逻辑）
  ```

  **2. `backend/internal/rule/`（业务层：规则服务）**

  ```go
  type Service struct {
      store    *store.Store
      versions *version.Service
      tokens   *token.Service
      subs     *subscription.Service // 复用 CheckSlugAvailable
      log      *slog.Logger
  }

  type Rule struct {
      ID         int64
      Slug       string
      Name       string
      ClientType string
      Schemes    []string
      Token      string // 全局共享 Token（每规则一份，不绑定用户）
      CurrentVersion int64
  }

  // Create：名称 + 客户端类型 + scheme + 首版本上传；标识由系统自动生成（rule- 前缀，后端兼容手填跨四类校验）；自动生成规则 Token
  func (s *Service) Create(ctx context.Context, name, slugVal, clientType string, schemes []string, src version.ContentProvider) (*Rule, error) {
      ok, err := s.subs.CheckSlugAvailable(ctx, slugVal, "", 0)
      if err != nil {
          return nil, err
      }
      if !ok {
          return nil, subscription.ErrSlugConflict // 409
      }
      var r *Rule
      err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          res, err := tx.ExecContext(ctx,
              `INSERT INTO rules (slug, name, client_type, schemes) VALUES (?,?,?,?)`,
              slugVal, name, clientType, toJSON(schemes))
          if err != nil {
              return fmt.Errorf("创建规则失败: %w", err)
          }
          id, _ := res.LastInsertId()
          tk, err := s.tokens.CreateRuleTokenTx(ctx, tx, id) // 创建时自动生成
          if err != nil {
              return err
          }
          r = &Rule{ID: id, Slug: slugVal, Name: name, ClientType: clientType, Schemes: schemes, Token: tk}
          return nil
      })
      if err != nil {
          return nil, err
      }
      if _, err := s.versions.CreateVersion(ctx, version.OwnerRule, r.ID, src); err != nil {
          s.rollbackRecord(ctx, r.ID) // 失败清理：删 rule_tokens + rules 行
          return nil, err
      }
      return r, nil
  }

  // Rename：创建后仅可改名（客户端类型与 scheme 不可修改——接入层不接收该字段）
  func (s *Service) Rename(ctx context.Context, id int64, name string) error { /* UPDATE name */ }

  // RefreshToken：物理轮替（规则 Token 全局共享，不随用户禁用/删除失效）
  func (s *Service) RefreshToken(ctx context.Context, id int64) (string, error) {
      return s.tokens.RotateRuleToken(ctx, id)
  }

  // Delete：级联删版本文件 + Token
  func (s *Service) Delete(ctx context.Context, id int64) error {
      // 事务内：DELETE rule_tokens → DELETE versions 行 → DELETE rules；提交后删版本目录
      ...
  }
  // 规则下载走 Step 4 已注册的 GET /rules/:slug/download?token=（公开），本 Step 接通解析（同分享模式，含 Content-Disposition）
  ```

  **3. `backend/internal/server/rule.go`（规则端点）**

  ```go
  type RuleHandler struct{ ruleSvc *rule.Service; verSvc *version.Service }

  func RegisterRuleRoutes(engine *gin.Engine, h *RuleHandler, sessionMW, adminMW gin.HandlerFunc) {
      admin := engine.Group("/api/admin/rules", sessionMW, adminMW) // 管理端：会话 + 管理员
      admin.GET("", h.list)
      admin.POST("", h.create)
      admin.PUT("/:id", h.rename)
      admin.DELETE("/:id", h.delete)
      admin.POST("/:id/token/refresh", h.refresh)
      // 版本端点：/api/admin/rules/:id/versions/... 同通用模式

      user := engine.Group("/api/rules", sessionMW) // 用户端：仅会话
      user.GET("", h.userList)                    // 规则卡片列表（登录用户，含全局 Token 供一键导入）
      user.GET("/:id/preview", h.preview)         // 会话凭据预览当前版本（需登录；text/plain + no-store）
  }
  ```

  **4. `backend/internal/server/profile.go`（个人中心端点；会话）**

  ```go
  type ProfileHandler struct{ store *store.Store }

  func RegisterProfileRoutes(engine *gin.Engine, h *ProfileHandler, sessionMW gin.HandlerFunc) {
      g := engine.Group("/api/profile", sessionMW)
      g.PUT("/username", h.updateUsername)
      g.PUT("/email", h.updateEmail)
      g.PUT("/password", h.updatePassword)
  }

  // 改用户名：即时生效（OIDC 用户下次 OIDC 登录会被提供商最新值覆盖，Design1 §4.6）
  func (h *ProfileHandler) updateUsername(c *gin.Context) {
      var req struct{ Username string `json:"username" binding:"required,min=1,max=64"` }
      // UPDATE users SET username = ? WHERE id = CtxUserID
      ...
  }

  // 改邮箱：新邮箱被占用拒绝 409；成功递增 credential_version（所有设备会话立即失效）
  func (h *ProfileHandler) updateEmail(c *gin.Context) {
      var req struct{ Email string `json:"email" binding:"required,max=254"` }
      email, err := auth.NormalizeEmail(req.Email)
      if err != nil {
          server.Fail(c, http.StatusBadRequest, "邮箱格式无效")
          return
      }
      ctx := c.Request.Context()
      userID := c.GetInt64(auth.CtxUserID)
      err = h.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          var dup int
          if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ? AND id != ?`, email, userID).Scan(&dup); err != nil {
              return err
          }
          if dup > 0 {
              return user.ErrEmailConflict
          }
          // 同事务：改邮箱 + 递增凭据版本号（旧会话全部失效）
          _, err := tx.ExecContext(ctx,
              `UPDATE users SET email = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
              email, userID)
          return err
      })
      if errors.Is(err, user.ErrEmailConflict) {
          server.Fail(c, http.StatusConflict, "该邮箱已被使用")
          return
      }
      if err != nil {
          server.Fail(c, http.StatusInternalServerError, err.Error())
          return
      }
      server.OK(c, gin.H{"message": "邮箱已修改，请重新登录"}) // 前端清凭据跳登录
  }

  // 设置/修改密码：已设密码需验证当前密码；OIDC 用户首次设置免旧密码但须已登录；成功递增 credential_version
  func (h *ProfileHandler) updatePassword(c *gin.Context) {
      var req struct {
          CurrentPassword string `json:"current_password"`
          NewPassword     string `json:"new_password" binding:"required,max=128"`
      }
      if err := c.ShouldBindJSON(&req); err != nil {
          server.Fail(c, http.StatusBadRequest, "参数校验失败")
          return
      }
      if err := auth.ValidatePassword(req.NewPassword); err != nil {
          server.Fail(c, http.StatusBadRequest, err.Error())
          return
      }
      ctx := c.Request.Context()
      userID := c.GetInt64(auth.CtxUserID)
      // 查当前 password_hash：非空 → 必须验证 CurrentPassword；空（OIDC 首设）→ 免旧密码
      // 同事务：写新哈希 + credential_version + 1
      ...
  }
  ```

  **5. 前端用户端三页**

  ```ts
  // api/home.ts（Step 4 端点封装）
  export interface PlatformCard {
    platform_id: number; name: string; description: string
    schemes: string[]                       // 含 {url} 占位符；一键导入取首项
    installer_file_url: string; installer_url: string
    status: 'group_selected' | 'custom' | 'unassigned' | 'admin_pool'
    download_token: string
    download_url: string                    // /subscriptions/{平台标识}/download?token=
    subscription_name?: string
  }
  export const homePlatforms = () => http.get<any, PlatformCard[]>('/home/platforms')
  export const refreshHomeToken = (platformId: number) => http.post<any, { token: string }>('/home/token/refresh', { platform_id: platformId })
  export const homeUpdatedAt = () => http.get<any, { updated_at: string | null }>('/home/updated_at')
  // api/rule.ts：userRules / previewRule；api/profile.ts：updateUsername/updateEmail/updatePassword
  ```

  ```vue
  <!-- HomeView.vue（UI §4.1，替换 Build1 占位）骨架 -->
  <script setup lang="ts">
  import dayjs from 'dayjs'
  // 顶部栏：站点名称（/api/site/info，Build3）+ 订阅更新时间戳（dayjs 本地时区，后端 UTC）；
  //        右侧：管理面板入口（仅管理员）+ 用户名 + 角色标签 + 所属组名 + 暗色切换 + 退出
  // 平台卡片网格：大屏 3 列 / 中屏 2 列 / 小屏 1 列（Tailwind grid-cols-1 md:grid-cols-2 xl:grid-cols-3）
  // 卡片三态：
  //   普通用户：组选定一份 / 未选定灰色占位「未分配，请联系管理员」（三按钮隐藏）/ 有自定义 a-alert info「已被分配自定义订阅」
  //   管理员：池内全部折叠列表（预览用显式 Token）

  // 一键导入：无 scheme 隐藏；多个取首项；对下载 URL 做 URL 编码后替换 {url}
  function oneClickImport(card: PlatformCard) {
    const scheme = card.schemes[0]
    const url = scheme.replace('{url}', encodeURIComponent(card.download_url))
    window.location.href = url
    // 跳转后无响应提示：setTimeout 后 Notify.info「请确认已安装对应客户端」
  }

  // 复制链接：弹窗展示 URL + 复制；用户绑定类 Token（group_selected/custom）复制时警示「该链接与您的账号绑定，请勿分享」
  function isUserBound(card: PlatformCard) { return card.status !== 'admin_pool' }

  // 刷新链接：ConfirmModal 后调 refreshHomeToken，旧链接立即失效
  // 底部「下载客户端」：本地下载（installer_file_url，公开可缓存）/ 链接下载（installer_url）并存则两个都显示
  // 公告栏卡片（有内容才显示，纯文本转义——Vue 默认转义，禁 v-html）；分流规则入口卡片 → /rules
  </script>
  ```

  ```vue
  <!-- RulesView.vue（UI §4.2）骨架 -->
  <script setup lang="ts">
  // PageHeader「分流规则」+ a-alert warning「规则内容公开，链接请谨慎分发，请勿外发」
  // 规则卡片网格：名称、客户端类型标签、当前版本
  // 每卡片：「下载」（会话凭据，调 preview 接口或新窗口打开）+「一键导入」（全局规则 Token：schemes 首项 + encodeURIComponent(下载 URL)）
  </script>

  <!-- ProfileView.vue（UI §4.3）骨架：a-tabs 三页签 -->
  <script setup lang="ts">
  // 基本信息：a-descriptions 展示 + 修改用户名/邮箱行内编辑；
  //          邮箱修改成功 → 弹「所有设备会话已失效」→ auth.logout() → 跳登录
  // 密码管理：已设密码需验证当前密码；OIDC 用户首次设置免旧密码（后端按 password_hash 是否为空判定）
  // OIDC 绑定：仅 oidc_configured 显示；已绑定展示 subject 脱敏（首尾保留中间掩码），未绑定「绑定 OIDC」→ startBind 跳授权
  </script>
  ```

  **6. 单元测试要点（验收要求）**

  - 后端：规则标识自动生成（兼容手填跨四类校验；与既有订阅/分享/自定义冲突 409；至此四表查重全部生效）；规则 Token 全局共享（禁用/删除其他用户后 Token 仍可下载）；改邮箱/改密码递增 credential_version（旧会话 401）；改邮箱冲突 409；OIDC 首设密码免旧密码、已设密码必填旧密码。
  - 前端：首页平台卡片三态渲染（已分配/未分配/自定义）；一键导入 URL 编码拼接（`encodeURIComponent` 验证特殊字符）。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：规则标识自动生成与兼容手填跨四类校验、规则 Token 全局共享（不随用户禁用失效）、改邮箱/密码递增凭据版本号、改邮箱冲突 409。
  - 前端单测覆盖：首页平台卡片三态（已分配/未分配/自定义）、一键导入 URL 编码拼接。
  - 手动验证：创建规则 → 用户端规则页下载/一键导入；用户首页一键导入唤起 scheme、复制链接、刷新链接；个人中心改邮箱后会话失效跳登录。

---

### Step 7：管理面板布局、订阅装配预留与全局收尾

**本 Step 完成后，系统应具备：完整管理面板布局（侧边栏 9 模块 + 1 预留）、订阅装配占位页、全局体验（暗色/响应式/404/进度条）收尾的能力。**

- **目标：** 建立管理面板整体布局，落地订阅装配预留入口，完成全局体验收尾。
- **前置条件：** Step 1~6（各管理页与用户端页面）已完成并验收。
- **产出文件与操作：**

  1. **创建 `frontend/src/layouts/AdminLayout.vue`**（UI §5.0）：`a-layout`——左侧 `a-layout-sider` 浅色主题（展开 220px/收起 64px，当前路由高亮）；右侧内容区白底卡片容器（24px 内边距）；**<768 侧边栏默认收起，汉堡按钮唤出 Drawer 抽屉式菜单**。**侧边栏菜单（9 模块 + 1 预留，平铺不分组，图标+文字）**：订阅、用户组、分享、平台、用户、审批中心、规则、面板配置、日志、**订阅装配（预留）**。其中「用户/审批中心/面板配置/日志」在 Build3 实现，本 Step 菜单项可先渲染但点击提示「Build3 提供」或隐藏（执行时选择隐藏对应菜单项，Build3 补充显示）。

  2. **创建 `frontend/src/views/admin/AssemblyView.vue`**（订阅装配占位页，UI §5.0，**第一次构建唯一落地形态**）：`a-result info` 风格卡片——标题「订阅装配」+ 功能一句话说明（勾选模块动态拼接双语法订阅）+ `a-tag`「即将推出」+ 暂缓提示文案（详见 DesignOnHold.md）；**无任何表单与接口调用**。

  3. **接通版本管理子路由**（UI §7.1）：`/admin/subscriptions/:id/versions`、`/admin/shares/:id/versions`、`/admin/rules/:id/versions`、`/admin/customs/:id/versions` 均复用 `VersionManageView`（按 ownerType 传参）。

  4. **完善路由表**（UI §7.1）：补齐本 Build 全部管理路由（subscriptions/groups/shares/platforms/platforms/:id/edit/rules/assembly）与用户端路由（/、/rules、/profile），路由级代码分割按懒加载分组（admin-sub/admin-user/admin-settings/home）。角色守卫：`/admin/**` 实时校验管理员（与后端中间件双保险，后端为准）。

  5. **全局体验收尾**（Design1 §3.7，UI §六/七）：
     - 确认暗色模式全站跟随（实时日志区例外在 Build3）。
     - 确认响应式三档断点（≥1200 / 768~1199 / <768）在各页面生效。
     - 确认 404 页、顶部进度条、删除确认统一 ConfirmModal。
     - 确认空状态文案（UI §7.5）在各列表页落地。
     - 确认时间展示：后端 UTC → 前端 dayjs 本地时区格式化。

- **参考代码/伪代码：**

  > 本 Step 以后端无改动（仅启动自检调用点在 Step 2 已预留）、前端为主。编写顺序：AdminLayout.vue → AssemblyView.vue → 版本子路由接线 → 路由表收尾 → 全局体验确认。

  **1. `frontend/src/layouts/AdminLayout.vue`（UI §5.0）**

  ```vue
  <script setup lang="ts">
  import { computed, onMounted, onUnmounted, ref } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { Layout, Menu, Drawer, Button } from 'ant-design-vue'
  import {
    CloudUploadOutlined, TeamOutlined, ShareAltOutlined, AppstoreOutlined,
    UserOutlined, AuditOutlined, BranchesOutlined, SettingOutlined,
    FileTextOutlined, BlockOutlined,
  } from '@ant-design/icons-vue'
  import { useTheme } from '@/theme'

  const route = useRoute()
  const router = useRouter()
  const { dark, toggle } = useTheme()

  // isMobile：matchMedia('(max-width: 767px)') 响应式布尔（与三档断点 <768 对齐）；
  // 必须监听窗口缩放，窗口跨越 768px 断点时侧边栏/Drawer 响应式切换
  const isMobile = ref(false)
  const collapsed = ref(false)   // <768 默认收起（onMounted 中按 isMobile 初始化）
  const drawerOpen = ref(false)

  function checkMobile() {
    isMobile.value = window.matchMedia('(max-width: 767px)').matches
    if (isMobile.value) collapsed.value = true // 移动端默认收起
  }
  onMounted(() => {
    checkMobile()
    window.addEventListener('resize', checkMobile)
  })
  onUnmounted(() => {
    window.removeEventListener('resize', checkMobile)
  })

  // 侧边栏菜单：9 模块 + 1 预留，平铺不分组（图标+文字）；
  // 「用户/审批中心/面板配置/日志」在 Build3 实现，本 Step 隐藏（Build3 补充显示）
  const menuItems = computed(() => [
    { key: '/admin/subscriptions', icon: CloudUploadOutlined, label: '订阅' },
    { key: '/admin/groups', icon: TeamOutlined, label: '用户组' },
    { key: '/admin/shares', icon: ShareAltOutlined, label: '分享' },
    { key: '/admin/platforms', icon: AppstoreOutlined, label: '平台' },
    // { key: '/admin/users', icon: UserOutlined, label: '用户' },          // Build3 显示
    // { key: '/admin/approvals', icon: AuditOutlined, label: '审批中心' }, // Build3 显示
    { key: '/admin/rules', icon: BranchesOutlined, label: '规则' },
    // { key: '/admin/settings', icon: SettingOutlined, label: '面板配置' }, // Build3 显示
    // { key: '/admin/logs', icon: FileTextOutlined, label: '日志' },        // Build3 显示
    { key: '/admin/assembly', icon: BlockOutlined, label: '订阅装配' },    // 预留占位页
  ])

  const selectedKeys = computed(() => {
    // 版本管理子路由高亮对应父菜单（/admin/subscriptions/:id/versions → 订阅）
    const seg = route.path.split('/')[2]
    return seg ? [`/admin/${seg}`] : []
  })

  function onMenuClick(key: string) {
    drawerOpen.value = false
    router.push(key)
  }
  </script>

  <template>
    <Layout class="min-h-screen">
      <!-- ≥768：固定 Sider（展开 220px / 收起 64px，浅色主题，当前路由高亮） -->
      <Layout.Sider v-if="!isMobile" v-model:collapsed="collapsed" theme="light"
                    :width="220" :collapsed-width="64" collapsible>
        <Menu mode="inline" :selected-keys="selectedKeys" :items="menuItems"
              @click="(e: any) => onMenuClick(e.key)" />
      </Layout.Sider>
      <!-- <768：汉堡按钮唤出 Drawer 抽屉式菜单 -->
      <Drawer v-else :open="drawerOpen" placement="left" :width="220" @close="drawerOpen = false">
        <Menu mode="inline" :selected-keys="selectedKeys" :items="menuItems"
              @click="(e: any) => onMenuClick(e.key)" />
      </Drawer>

      <Layout>
        <Layout.Header v-if="isMobile" class="bg-white dark:bg-gray-800 flex items-center">
          <Button type="text" @click="drawerOpen = true">☰</Button>
        </Layout.Header>
        <!-- 右侧内容区：白底卡片容器（24px 内边距） -->
        <Layout.Content class="p-6">
          <div class="bg-white dark:bg-gray-800 rounded-lg p-6 min-h-full">
            <RouterView />
          </div>
        </Layout.Content>
      </Layout>
    </Layout>
  </template>
  ```

  **2. `frontend/src/views/admin/AssemblyView.vue`（订阅装配占位页，第一次构建唯一落地形态）**

  ```vue
  <script setup lang="ts">
  import { Result, Tag } from 'ant-design-vue'
  // 无任何表单与接口调用；暂缓项详见 DesignOnHold.md
  </script>

  <template>
    <Result status="info" title="订阅装配">
      <template #subTitle>
        <p>勾选模块动态拼接双语法订阅（Clash YAML / Shadowrocket .conf）</p>
        <Tag color="processing" class="mt-2">即将推出</Tag>
        <p class="text-gray-400 text-sm mt-3">该功能已完成设计、暂缓开发，恢复时间见项目规划</p>
      </template>
    </Result>
  </template>
  ```

  **3. 版本管理子路由接线 + 路由表收尾（`src/router/index.ts` 扩展）**

  ```ts
  import AdminLayout from '@/layouts/AdminLayout.vue'

  // 版本管理子路由：四条均复用 VersionManageView，按 ownerType 传参（UI §7.1）
  const versionRoutes = [
    { path: '/admin/subscriptions/:id/versions', ownerType: 'subscription', prefix: '/api/admin/subscriptions' },
    { path: '/admin/shares/:id/versions', ownerType: 'share', prefix: '/api/admin/shares' },
    { path: '/admin/rules/:id/versions', ownerType: 'rule', prefix: '/api/admin/rules' },
    { path: '/admin/customs/:id/versions', ownerType: 'custom', prefix: '/api/admin/customs' },
  ].map((r) => ({
    path: r.path,
    component: () => import('@/views/admin/VersionManageView.vue'),
    props: (route: any) => ({
      ownerType: r.ownerType,
      ownerId: Number(route.params.id),
      apiPrefix: r.prefix,
      resourceName: r.ownerType,
    }),
    meta: { layout: 'admin', requiresAdmin: true },
  }))

  // 管理路由（懒加载分组 admin-sub；路由级代码分割）
  const adminRoutes = [
    { path: '/admin/subscriptions', component: () => import('@/views/admin/SubscriptionsView.vue') },
    { path: '/admin/groups', component: () => import('@/views/admin/GroupsView.vue') },
    { path: '/admin/shares', component: () => import('@/views/admin/SharesView.vue') },
    { path: '/admin/platforms', component: () => import('@/views/admin/PlatformsView.vue') },
    { path: '/admin/platforms/:id/edit', component: () => import('@/views/admin/PlatformEditView.vue') },
    { path: '/admin/platforms/new', component: () => import('@/views/admin/PlatformEditView.vue') },
    { path: '/admin/rules', component: () => import('@/views/admin/RulesView.vue') },
    { path: '/admin/assembly', component: () => import('@/views/admin/AssemblyView.vue') },
    // Build3 补充：users/approvals/settings/logs
  ].map((r) => ({ ...r, meta: { layout: 'admin', requiresAdmin: true } }))

  // 用户端路由（懒加载分组 home）
  const userRoutes = [
    { path: '/', component: () => import('@/views/HomeView.vue'), meta: { layout: 'home' } },
    { path: '/rules', component: () => import('@/views/RulesView.vue'), meta: { layout: 'home' } },
    { path: '/profile', component: () => import('@/views/ProfileView.vue'), meta: { layout: 'home' } },
  ]

  // 角色守卫：/admin/** 实时校验管理员（与后端中间件双保险，后端为准）——扩展 Step 2 守卫框架
  router.beforeEach(async (to) => {
    if (to.meta.requiresAdmin) {
      const auth = useAuthStore()
      if (!auth.user) {
        try { auth.user = await me() } catch { return '/login' }
      }
      if (auth.user.role !== 'admin') return '/' // 非管理员访问管理路由 → 回首页
    }
    return true
  })
  // App.vue layouts 映射补充 admin: AdminLayout
  ```

  **4. 全局体验收尾（确认项，无新增代码则逐项走查）**

  ```text
  ☐ 暗色模式全站跟随：切换后各页组件/卡片/表格同步（实时日志区例外，Build3）
  ☐ 响应式三档断点：≥1200 / 768~1199 / <768 各页生效（双态列表/卡片网格/AdminLayout 抽屉）
  ☐ 404 页、顶部进度条（Step 2 自实现）、删除确认统一 ConfirmModal（无浏览器原生 confirm 残留）
  ☐ 空状态文案（UI §7.5）各列表页落地（TriStateList empty-text 逐项核对）
  ☐ 时间展示：后端 UTC → 前端 dayjs 本地时区（统一封装 fmtTime(ts) = dayjs(ts).format('YYYY-MM-DD HH:mm')）
  ```

  **5. 验收走查脚本（手动）**

  ```text
  1. 管理员登录 → 首页「管理面板」入口 → 侧边栏 6 项可见（Build3 4 项隐藏）+ 订阅装配预留项
  2. 浏览器宽度 <768 → 侧边栏收起，汉堡按钮唤出 Drawer
  3. 订阅装配页：无表单、无接口调用（Network 面板确认零请求）
  4. 四条版本管理子路由渲染同一 VersionManageView（ownerType 传参正确）
  5. 非管理员账号访问 /admin/subscriptions → 被守卫拦回首页
  ```

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 手动验证：管理员登录进入管理面板，侧边栏 9 模块 + 订阅装配预留项可见；移动端汉堡 Drawer 正常；订阅装配占位页无表单无接口；各版本管理子路由复用同一组件；暗色/响应式/404 全局生效。

---

## 五、候选构建项（待用户决策，逐项转 Step）

| # | 候选 | 说明 | 来源 |
|---|------|------|------|
| — | 本 Build 无候选项 | Build2 范围为已确认的固定 7 Step；管理面与运维能力见 Build3.md | — |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-07 | 初始版本：订阅核心与用户端价值链（7 Step），承接 Build1 |
