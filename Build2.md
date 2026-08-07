# Build2.md — 订阅核心与用户端价值链（当前构建方案）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第二轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则），承接已归档的 [Build1.md](./Build1.md)（第一轮：工程骨架与认证闭环，须全部验收通过后本轮方可启动）。
> - 设计基线：[Design1.md](./Design1.md)（与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design1-UI.md](./Design1-UI.md)
> - 编码指令：[AGENTS.md](./AGENTS.md)（**唯一强要求**）
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
| 手填标识 | 小写字母数字连字符，3~64 字符，四类资源全局唯一命名空间交叉校验 | Design1 §2.2 |
| 自动生成标识 | 类型前缀 + 8 位随机短码（share-/custom-），冲突重试最多 3 次 | Design1 §2.2 |
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

- ☐ Step 1：平台管理（含安装包分发）
- ☐ Step 2：通用版本管理事务组件与订阅池
- ☐ Step 3：用户组与订阅分发机制
- ☐ Step 4：下载 Token 体系与统一下载端点
- ☐ Step 5：自定义订阅与分享订阅
- ☐ Step 6：规则管理与用户端页面
- ☐ Step 7：管理面板布局、订阅装配预留与全局收尾

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | 平台管理（含安装包分发） | Design1 §3.4.4/4.7/6.3 | ☐ 未开始 |
| 2 | 通用版本管理事务组件与订阅池 | Design1 §4.1/3.4.1/2.2 | ☐ 未开始 |
| 3 | 用户组与订阅分发机制 | Design1 §2.2/3.4.2/4.4 | ☐ 未开始 |
| 4 | 下载 Token 体系与统一下载端点 | Design1 §4.2/4.3/6.4 | ☐ 未开始 |
| 5 | 自定义订阅与分享订阅 | Design1 §2.3/2.4/3.4.3 | ☐ 未开始 |
| 6 | 规则管理与用户端页面 | Design1 §3.4.7/3.3/3.5/3.6 | ☐ 未开始 |
| 7 | 管理面板布局、订阅装配预留与全局收尾 | Design1 §3.4/3.9/3.7，UI §五/七 | ☐ 未开始 |

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

- **参考代码/伪代码：** 待补充。

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
     - `subscriptions` 表：`id`、`slug`（UNIQUE，手填，四类全局命名空间）、`name`（不强制唯一）、`platform_id`（外键）、`current_version`（INTEGER）、`created_at`、`updated_at`。
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
     - CRUD：指定平台 + 名称（不强制唯一）+ **标识（手填，小写字母数字连字符 3~64，与分享/规则/自定义共用全局唯一命名空间，创建时跨四类资源交叉校验冲突）** + 关联用户组多选（可为空）。
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

- **参考代码/伪代码：** 待补充。

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

- **参考代码/伪代码：** 待补充。

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

- **参考代码/伪代码：** 待补充。

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

- **参考代码/伪代码：** 待补充。

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

  1. **创建 `backend/migrations/1006_rules.sql`**：`rules` 表：`id`、`slug`（UNIQUE，**手填**，四类命名空间交叉校验）、`name`（不强制唯一）、`client_type`（当前仅 `shadowrocket`）、`schemes`（JSON 数组，含 `{url}` 占位符）、`current_version`、`created_at`、`updated_at`。

  2. **创建 `backend/internal/rule/`（业务层）**：规则服务。必须实现：
     - CRUD：名称 + 手填标识 + 客户端类型 + scheme + 首版本上传；**创建后仅可改名（客户端类型与 scheme 不可修改）**。
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

- **参考代码/伪代码：** 待补充。

- **测试与验收命令：**
  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build && npm run test
  ```
- **验收标准：**
  - 全部命令通过。
  - 后端单测覆盖：规则标识手填跨四类校验、规则 Token 全局共享（不随用户禁用失效）、改邮箱/密码递增凭据版本号、改邮箱冲突 409。
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

- **参考代码/伪代码：** 待补充。

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
