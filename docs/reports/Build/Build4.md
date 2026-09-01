# Build4.md — 基础模式地基：Go 1.26 + 1009 迁移 + 旧分发模型拆除 + 规则素材池（当前构建方案·第四轮）

> **文档定位：** 本文档是 VPN 订阅管理系统的**第四轮构建方案**（依据 AGENTS.md §8.1：Build 文档为详细构建方案，非强规则），承接已归档的 [Build1.md](Build1.md)、[Build2.md](Build2.md)、[Build3.md](Build3.md)（前三轮均已验收归档）。
> - 设计基线：[Design2.md](../Design/Design2.md)（增量能力：订阅装配与 Xray 对接；与 AGENTS.md 或用户决策冲突时以用户确认为准）
> - GUI 规格：[Design2-UI.md](../Design/Design2-UI.md)（已归档，承载 Design2 全部界面部件）
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 后续轮次：[Build5.md](Build5.md)（节点/代理组/四类装配器与分发）、[Build6.md](Build6.md)（高级模式 Xray 对接后端）、[Build7.md](Build7.md)（高级模式管理面与交付收口），本轮验收后按序启动
>
> **里程碑：本 Build 全部 Step 完成后，系统必须在 Go 1.26 上编译通过、已应用 1009 增量迁移、代码中不再存在 group_selections / subscription_group_rel 旧分发模型、订阅地址池为「每平台一份 + product_type」模型、空池下载返回 200 注释块，并具备完整的规则素材池管理（CRUD / URL 同步 / 异步轮询 / 可选定时同步）与基础模式前端骨架。**
>
> **拆分说明（用户已授权按实际体量决定文档数）：** Design2 增量被拆为 4 个 Build 文档，避免单个 Build 过长超出构建上下文：Build4（本文件）只做地基与规则素材池；Build5 做基础模式装配与分发；Build6 做高级模式后端核心；Build7 做高级模式管理面与全量收口。**执行 AI 按 Build4 → Build7 顺序执行，禁止跨文档并行或跳步。**

---

## 执行约束（执行 AI 必须严格遵守）

1. **严格按 Step 顺序执行**，完成一个 Step 并验收通过后，方可进入下一个 Step。**禁止跳步、禁止并行执行多个 Step、禁止自行合并步骤、禁止跨 Build 提前实现后续功能**。
2. **每个 Step 完成后必须运行该 Step 的「验证命令」**，全部通过才算完成；任一命令失败必须修复后重验，禁止带错进入下一 Step。
3. **遇到模糊、歧义或设计文档未覆盖的细节，必须停止并使用提问工具向用户询问，禁止自行假设或自由发挥**。本文未列出的技术选型以 Design2.md 为准；Design2.md 未覆盖的实现细节先查 `docs/Reference/`，仍无答案则提问。
4. **禁止引入设计文档未提及的框架、库或架构模式**。本 Build 不新增第三方 Go 依赖（xray-core 在 Build6 Step 0 才引入）；前端仅可新增 Design2-UI 明示的 `diff`（jsdiff）依赖（Build5 引入，本 Build 不安装）。
5. **关键设计参数必须严格按下表取值**，与 Design2.md 保持一致，禁止修改：

| 参数 | 取值 | 出处 |
|------|------|------|
| Go 版本 | **go 1.26.0**；Docker 后端基础镜像 `golang:1.26-alpine` | Design2 §5.3，AGENTS.md §一 |
| 增量迁移编号 | **1009_xray.sql**（唯一一个增量迁移，一次包含全部增量 DDL；1008 已占用） | Design2 §5.9 |
| 新部署口径 | 仅纯增量 DDL + 种子写入；**不迁移旧业务数据、不重建 subscriptions、不清理 download_tokens/versions、不引入迁移后钩子** | Design2 §一/§5.9 |
| product_type 枚举 | `yaml` / `subs` / `generic-subs`（platforms 与 subscriptions 同枚举；默认 `yaml`） | Design2 §4.4/§5.9 |
| 订阅条目唯一性 | `subscriptions.platform_id` 唯一（UNIQUE 索引）；每平台一份订阅条目 | Design2 §4.4 |
| 规则素材池拉取 | 单 URL 超时 60s；内容上限 50MB；**空响应视为失败、响应非空但有效条目为 0 视为失败、部分失败时不执行差量删除** | Design2 §2.4 |
| 条目排序口径 | manual 段恒在 url 段之前；manual 段按创建序、url 段按同步首次出现序；**系统维护，禁止条目级手动调序** | Design2 §2.2 |
| 定时同步 | 每池每日执行；`sync_time` 默认 `04:00` 按 **UTC**；停机错过不补跑 | Design2 §2.4 |
| 池同步任务持久化 | `pool_sync_tasks` 状态：running/succeeded/failed/partial；服务启动时把 running 置 failed（原因「服务重启，任务中断」） | Design2 §2.4/§5.9 |
| 同步任务历史保留 | **保留 7 天，超期动态清理**（任务终态写回同事务内顺手清理该池超期旧行） | Design2 §5.9 |
| 空池下载口径 | 订阅/分享/规则下载端点与 `/api/subscriptions/preview` 在无激活版本时返回 **HTTP 200 + text/plain**，内容 `# error: no active version\n`；无效/过期 Token 仍统一 404 | Design2 §4.4/§5.10，AGENTS §4.8 |
| 高级模式配置键 | `advanced_mode`（"true"/"false"，未设置视为 false）；本 Build 只读取并暴露 status，**开关写入口在 Build7** | Design2 §5.10 |
| 首页默认规则 | `rules.is_home_default`，至多一条 =1（partial unique index）；切换时事务内清旧置新 | Design2 §5.9 |
| 显式 Token | 不再新发（管理员首页卡片改预览形态）；`download_tokens.subscription_id` 列与订阅删除级联逻辑保留兼容，**不新增写入** | Design2 §一/§4.4 |

6. **注释使用中文**；所有 error 必须处理，禁止忽略返回值；禁止散落的 `fmt.Println` 调试输出，统一使用结构化日志库。
7. **服务实例一律构造注入**（结构体 Handler + 依赖传入），禁止包级全局变量持有服务实例；HTTP 处理器按业务域拆分文件；职责分层（接入层 / 业务层 / 数据层）。
8. **日志必须将 `?token=` 查询参数值脱敏为 `***`**；5xx 内部错误默认脱敏返回通用信息（沿用现有机制，不破坏）。
9. **错误码约定**：400=校验错误，401=会话凭据缺失/无效/过期，403=权限不足，409=重复冲突，429=速率限制，500=服务器内部错误；下载类业务错误返回 HTTP 200 + 纯文本注释块。
10. **本 Build 不得实现 Xray 运行时行为**：1009 迁移中的 xray 表/字段只建列、不做业务读写（dataclear 清表除外）；`internal/xray` 包、高级端点、节点/代理组/装配器业务代码在后续 Build 实现。

---

## TODOLIST CheckList（构建进度核对）

> 执行 AI 必须逐个完成并勾选，便于核对构建进度。状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

- ✅ Step 0：Go 1.26 升级与 Dockerfile 同步（编译核验）
- ✅ Step 1：1009 增量迁移（全量 DDL + proxy_groups 种子）与 dataclear 清表清单适配
- ✅ Step 2：拆除 group_selections / subscription_group_rel 旧分发链路（后端）
- ✅ Step 3：CreateVersion activate 语义、订阅新模型、平台 product_type、空规则实体与首页默认规则
- ✅ Step 4：前端基线适配（订阅/平台/组/规则/首页/版本管理）+ advanced_mode 状态与路由骨架（R14 修复收口已完成）
- ✅ Step 5：规则素材池后端（CRUD / 解析白名单 / URL 同步任务 / 定时同步）（R14-19 已关闭）
- ✅ Step 6：规则素材池前端（AssemblyView 页签壳 + 池列表/详情/同步轮询）（R14-10/R14-23/R14-15⑧ 已关闭）
- ✅ Step 7：Build4 端到端验收与文档核对（已随修复收口完成）

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 0 | Go 1.26 升级与 Dockerfile 同步 | Design2 §5.3 | ✅ 验收通过 |
| 1 | 1009 增量迁移与 dataclear 清表清单 | Design2 §5.9，AGENTS §4.7 | ✅ 验收通过 |
| 2 | 拆除旧分发链路（后端） | Design2 §一/§4.4/§5.10 | ✅ 验收通过 |
| 3 | CreateVersion activate 语义与平台/规则改造 | Design2 §4.4/§5.9 | ✅ 验收通过 |
| 4 | 前端基线适配与 advanced_mode 骨架 | Design2-UI §2~4 | ✅ 验收通过 |
| 5 | 规则素材池后端 | Design2 §二 | ✅ 验收通过 |
| 6 | 规则素材池前端 | Design2-UI §5.1/§5.2/§9.1/§9.2 | ✅ 验收通过 |
| 7 | Build4 端到端验收 | Design2 §一/§二 | ✅ 验收通过 |

---

## 二、构建概要（文件清单总览）

| Step | 涉及文件（核心） | 要点 |
|------|----------------|------|
| 0 | `backend/go.mod`、`Dockerfile`、`README.md`、`.github/workflows/docker-build.yml` | go 1.26.0 + golang:1.26-alpine；本 Step 不引入 xray-core |
| 1 | `backend/migrations/1009_xray.sql`、`backend/internal/dataclear/dataclear.go`、`backend/internal/dataclear/dataclear_test.go` | 全量增量 DDL + 预设组种子 + 清表清单替换 |
| 2 | `backend/internal/group/`、`backend/internal/subscription/`、`backend/internal/download/`、`backend/internal/server/{server,group,subscription,download,home}.go` 及对应测试 | 移除组选定/关联；订阅唯一平台化；下载按平台解析；无激活版本 200 注释块 |
| 3 | `backend/internal/version/`、`backend/internal/platform/`、`backend/internal/setup/`、`backend/internal/rule/`、`backend/internal/server/{subscription,platform,rule}.go` 及对应测试 | CreateOptions.activate；product_type 全链路；空规则实体 + 首页默认规则 |
| 4 | `frontend/src/api/{home,subscription,group,platform,rule,version,system}.ts`、`frontend/src/views/{HomeView,ProfileView}.vue`、`frontend/src/views/admin/{SubscriptionsView,VersionManageView,PlatformsView,PlatformEditView,RulesView,GroupsView}.vue`、`frontend/src/layouts/AdminLayout.vue`、`frontend/src/router/index.ts`、`frontend/src/stores/system.ts`、`frontend/src/components/{PageHeader.vue,CopyField.vue}` | 新 API 形状落地；菜单/路由 advanced_mode 驱动；PageHeader/CopyField 通用组件 |
| 5 | `backend/internal/pool/`、`backend/internal/server/pool.go`、`backend/internal/cron/`、`backend/cmd/server/main.go`、`backend/internal/server/server.go`、测试 | 素材池 + 条目 + 解析白名单 + 同步任务 + 轮询端点 + 定时同步 + 启动重置 running |
| 6 | `frontend/src/api/pool.ts`、`frontend/src/api/request.ts`、`frontend/src/views/admin/AssemblyView.vue`、相关子组件 | 五页签壳（池页签实现，四个装配器页签为后续 Build 占位）、pollTask 轮询封装 |
| 7 | 全部上述文件 + `docs/` 核对 | 全新库迁移 + 编译/测试 + 手工冒烟清单 |

---

## 三、构建顺序依赖图

```
Step 0（Go 1.26）──▶ Step 1（1009 迁移，依赖编译通过）
Step 1 ──▶ Step 2（拆除旧分发：新 schema 上后端可运行）
Step 2 ──▶ Step 3（版本激活语义与平台/规则改造）
Step 3 ──▶ Step 4（前端基线适配，依赖新后端契约）
Step 3 ──▶ Step 5（素材池后端，依赖 1009 表与既有服务骨架）
Step 4 + Step 5 ──▶ Step 6（素材池前端，依赖 pollTask 与后端端点）
Step 6 ──▶ Step 7（端到端验收）
```

> 线性执行序：Step 0 → 1 → 2 → 3 → 4 → 5 → 6 → 7。Step 4 与 Step 5 无相互依赖，但仍按序号执行，不并行。

---

## 四、分步构建计划

---

### Step 0：Go 1.26 升级与 Dockerfile 同步

**本 Step 完成后，系统应能在 Go 1.26 工具链上完成 `go build ./...`、`go vet ./...`、`go test ./...`，且 Dockerfile 后端阶段使用 `golang:1.26-alpine`。**

- **目标：** 按 Design2 §5.3 决策升级 Go 工具链声明与镜像，不引入任何业务代码变更。
- **前置条件：** 执行机已安装 Go 1.26（`go version` 输出 `go1.26.x`）。若本机仍为旧版本，先安装 Go 1.26 再继续；**不得在 Go 1.25 上跳过本 Step**。
- **产出文件与操作：**

  1. **`backend/go.mod`**：把 `go 1.25.0` 改为 `go 1.26.0`。**本 Step 不添加 xray-core 依赖**（Build6 Step 0 才引入）；不要运行会引入无关依赖的 `go get`。
  2. **`Dockerfile`**：后端阶段基础镜像 `FROM golang:1.25-alpine AS backend` 改为 `FROM golang:1.26-alpine AS backend`。
  3. **`README.md`**：两处 Go 版本标识同步为 1.26（徽章 `Go-1.26`、技术栈文案「Go 1.26」）。
  4. **`.github/workflows/docker-build.yml`**：若 CI 使用 Go 镜像或 setup-go 版本，同步为 1.26（无相关显式声明则不改，但必须检查）。
  5. **全仓检查**：`grep -R "1\.25\|golang:1.25" --include='*.go' --include='*.md' --include='Dockerfile' --include='*.yml' .` 不应再有遗留（历史存档文档中的 1.25 记录允许保留，但活跃文件必须清零）。

- **参考代码/伪代码：**

  ```bash
  # 检查执行机工具链（必须 1.26）
  go version
  cd backend
  go mod edit -go=1.26.0
  go mod tidy        # 只应整理 go 行，不新增依赖；若新增依赖，停止检查原因
  go build ./...
  ```

  > 若 `go mod tidy` 因工具链版本/模块缓存权限失败：先修复 Go 环境（安装 1.26、清理只读缓存或设置可写的 GOMODCACHE/GOCACHE），禁止通过删依赖来「绕过」失败。

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd frontend && npm run build
  cd .. && grep -n "golang:1.25\|go 1.25" Dockerfile backend/go.mod README.md .github/workflows/docker-build.yml 2>/dev/null || true
  ```

- **验收标准：** 后端编译/静态检查/单测全部通过；前端构建通过；活跃文件无 1.25 遗留；`git diff` 仅含工具链与文档版本修改（无业务代码变更）。

---

### Step 1：1009 增量迁移（全量 DDL + proxy_groups 种子）与 dataclear 清表清单适配

**本 Step 完成后，全新数据库执行迁移将得到 Design2 §5.9 定义的全部增量表/列；`dataclear.ClearTablesTx` 覆盖全部 13 张增量新表并停止引用已删除的两张旧表；单测通过。**

- **目标：** 一次落盘全部增量 schema（含高级模式表，本 Build 只建不用）与代理组预设种子；同步修复一键清空。
- **前置条件：** Step 0 验收通过。**数据口径：本期按全新部署实施，本 Step 禁止为旧业务数据写迁移脚本**（Design2 §5.9）。
- **产出文件与操作：**

  1. **创建 `backend/migrations/1009_xray.sql`**：内容按下述参考 SQL 完整写入，禁止增删表/列/索引或改名。SQL 内注释使用中文。
     - 先做 5 张既有表 ALTER；再做全部新表（按外键依赖顺序）；最后 DROP 两张旧表。
     - 预设组种子写入 `proxy_groups`：9 个预设组，`enabled=1`，`type='preset'`，`definition_json` 含组类型与默认成员「🚀直接连接」（**空节点数组 + 子组数组 `["🚀直接连接"]`）；**组名与 `Clash.yaml.template.md` 作者配置逐字一致（含 emoji 前缀）**。
  2. **修改 `backend/internal/dataclear/dataclear.go`**：`ClearTablesTx` 的表清单改为下方顺序；**必须移除 `subscription_group_rel`、`group_selections`**（迁移后这两张表已不存在，继续 DELETE 会报错）；新增 13 张增量表。
  3. **修改 `backend/internal/dataclear/dataclear_test.go`**：其 fstest.MapFS 增补 1009 简表定义（新表名与列可简化，但必须包含清表清单引用的所有表名）；断言保持不变，新增「13 张增量表可被清空」的用例（向其中若干表插 1 行，清空后计数为 0）。
  4. **启动验证口径**：本 Step 允许启动旧业务服务做迁移与 `/health` 验证；若旧代码因已 DROP 的两张表在业务端点报错，属预期过渡态，按全新部署口径清空已有数据重新开始（不要用旧业务库做验收）。

- **参考代码/伪代码：**

  **1. `backend/migrations/1009_xray.sql`（全量，可整段复制；注意 SQLite 语法）**

  ```sql
  -- 1009_xray.sql — Design2 增量 DDL（基础模式 + 高级模式全量表结构一次落盘；全新部署口径，不迁移旧数据）
  -- 0) 既有表增量列
  ALTER TABLE platforms ADD COLUMN product_type TEXT NOT NULL DEFAULT 'yaml';
  ALTER TABLE rules ADD COLUMN is_home_default INTEGER NOT NULL DEFAULT 0;
  CREATE UNIQUE INDEX idx_rules_home_default ON rules(is_home_default) WHERE is_home_default = 1;
  ALTER TABLE groups ADD COLUMN default_quota REAL;
  ALTER TABLE users ADD COLUMN quota_override REAL;
  ALTER TABLE users ADD COLUMN uuid_encrypted TEXT;
  ALTER TABLE users ADD COLUMN expire_at TEXT;
  ALTER TABLE users ADD COLUMN quota_exceeded INTEGER NOT NULL DEFAULT 0;
  ALTER TABLE users ADD COLUMN proxy_secret_encrypted TEXT;
  ALTER TABLE subscriptions ADD COLUMN product_type TEXT NOT NULL DEFAULT 'yaml';
  DROP INDEX idx_subscriptions_platform;
  CREATE UNIQUE INDEX idx_subscriptions_platform_uniq ON subscriptions(platform_id);

  -- f) 规则素材池（基础模式，第二章）
  CREATE TABLE rule_pools (
      id             INTEGER PRIMARY KEY AUTOINCREMENT,
      name           TEXT NOT NULL UNIQUE,
      urls_json      TEXT NOT NULL DEFAULT '[]',
      last_synced_at TIMESTAMP,
      sync_status    TEXT NOT NULL DEFAULT '',
      sync_error     TEXT NOT NULL DEFAULT '',
      auto_sync      INTEGER NOT NULL DEFAULT 0,
      sync_time      TEXT NOT NULL DEFAULT '04:00',
      created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE pool_entries (
      id          INTEGER PRIMARY KEY AUTOINCREMENT,
      pool_id     INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
      rule_type   TEXT NOT NULL,
      match_value TEXT NOT NULL,
      source      TEXT NOT NULL CHECK (source IN ('url','manual')),
      sort_order  INTEGER NOT NULL,
      created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      UNIQUE (pool_id, rule_type, match_value)
  );
  CREATE INDEX idx_pool_entries_pool_order ON pool_entries(pool_id, sort_order);

  CREATE TABLE pool_sync_tasks (
      id           INTEGER PRIMARY KEY AUTOINCREMENT,
      pool_id      INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
      status       TEXT NOT NULL CHECK (status IN ('running','succeeded','failed','partial')),
      per_url_json TEXT NOT NULL DEFAULT '[]',
      error        TEXT NOT NULL DEFAULT '',
      started_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      finished_at  TIMESTAMP,
      created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );
  CREATE INDEX idx_pool_sync_tasks_pool ON pool_sync_tasks(pool_id, id DESC);

  -- Xray 实例与统一节点表（高级模式，第三章/第五章；本 Build 仅建表）
  CREATE TABLE xray_instances (
      id               INTEGER PRIMARY KEY AUTOINCREMENT,
      name             TEXT NOT NULL UNIQUE,
      slug             TEXT NOT NULL UNIQUE,
      api_addr         TEXT NOT NULL,
      api_tag          TEXT NOT NULL DEFAULT '',
      enabled          INTEGER NOT NULL DEFAULT 1,
      last_collect_at  TIMESTAMP,
      collect_status   TEXT NOT NULL DEFAULT '',
      collect_error    TEXT NOT NULL DEFAULT '',
      created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE nodes (
      id            INTEGER PRIMARY KEY AUTOINCREMENT,
      source        TEXT NOT NULL CHECK (source IN ('manual','xray')),
      name          TEXT NOT NULL UNIQUE,
      display_name  TEXT,
      instance_id   INTEGER REFERENCES xray_instances(id) ON DELETE CASCADE,
      tag           TEXT,
      protocol      TEXT NOT NULL,
      host          TEXT NOT NULL,
      port          INTEGER NOT NULL,
      protocol_json TEXT NOT NULL DEFAULT '{}',
      is_public     INTEGER NOT NULL DEFAULT 0,
      enabled       INTEGER NOT NULL DEFAULT 1,
      allocatable   INTEGER NOT NULL DEFAULT 1,
      last_seen_at  TIMESTAMP,
      missing       INTEGER NOT NULL DEFAULT 0,
      created_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at    TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      UNIQUE (instance_id, tag),
      CHECK ((source = 'xray' AND instance_id IS NOT NULL) OR (source = 'manual' AND instance_id IS NULL))
  );
  CREATE INDEX idx_nodes_instance ON nodes(instance_id);
  -- 有效渲染名全局唯一兜底（display_name 非空则用之，否则 name）；跨表（代理组/强制组/Clash-mihomo 内建保留代理名）冲突由应用层校验
  CREATE UNIQUE INDEX idx_nodes_render_name ON nodes(COALESCE(NULLIF(display_name,''), name));

  CREATE TABLE proxy_groups (
      id              INTEGER PRIMARY KEY AUTOINCREMENT,
      name            TEXT NOT NULL UNIQUE,
      type            TEXT NOT NULL CHECK (type IN ('preset','custom')),
      preset_key      TEXT,
      enabled         INTEGER NOT NULL DEFAULT 1,
      definition_json TEXT NOT NULL DEFAULT '{}',
      created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE group_nodes (
      group_id   INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
      node_id    INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
      sort_order INTEGER NOT NULL DEFAULT 0,
      PRIMARY KEY (group_id, node_id)
  );
  CREATE INDEX idx_group_nodes_node ON group_nodes(node_id);

  CREATE TABLE xray_users (
      user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      instance_id  INTEGER NOT NULL REFERENCES xray_instances(id) ON DELETE CASCADE,
      inbound_tag  TEXT NOT NULL,
      node_id      INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
      email        TEXT NOT NULL,
      sync_status  TEXT NOT NULL CHECK (sync_status IN ('pending','synced','failed')),
      last_error   TEXT NOT NULL DEFAULT '',
      created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      PRIMARY KEY (user_id, instance_id, inbound_tag)
  );
  CREATE INDEX idx_xray_users_node ON xray_users(node_id);

  CREATE TABLE traffic_records (
      user_id  INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
      ym       TEXT NOT NULL,
      uplink   INTEGER NOT NULL DEFAULT 0,
      downlink INTEGER NOT NULL DEFAULT 0,
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      PRIMARY KEY (user_id, ym)
  );

  -- 装配快照（基础模式，第四章；version_id 与 versions 1:1）
  CREATE TABLE assembly_blueprints (
      id                INTEGER PRIMARY KEY AUTOINCREMENT,
      version_id        INTEGER NOT NULL UNIQUE REFERENCES versions(id) ON DELETE CASCADE,
      target_syntax     TEXT NOT NULL CHECK (target_syntax IN ('clash-yaml','sr-subs','generic-subs','sr-conf')),
      fixed_params_json TEXT NOT NULL DEFAULT '{}',
      selection_json    TEXT NOT NULL DEFAULT '{}',
      custom_rules_json TEXT NOT NULL DEFAULT '[]',
      render_plan_json  TEXT NOT NULL DEFAULT '{}',
      created_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at        TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  -- 独立 Xray 账号（高级模式，§5.11；本 Build 仅建表）
  CREATE TABLE xray_ext_accounts (
      id                       INTEGER PRIMARY KEY AUTOINCREMENT,
      name                     TEXT NOT NULL UNIQUE,
      email                    TEXT NOT NULL UNIQUE,
      uuid_encrypted           TEXT,
      proxy_secret_encrypted   TEXT,
      quota                    REAL,
      quota_exceeded           INTEGER NOT NULL DEFAULT 0,
      created_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at               TIMESTAMP DEFAULT CURRENT_TIMESTAMP
  );

  CREATE TABLE xray_ext_users (
      ext_account_id INTEGER NOT NULL REFERENCES xray_ext_accounts(id) ON DELETE CASCADE,
      instance_id    INTEGER NOT NULL REFERENCES xray_instances(id) ON DELETE CASCADE,
      inbound_tag    TEXT NOT NULL,
      node_id        INTEGER REFERENCES nodes(id) ON DELETE CASCADE,
      sync_status    TEXT NOT NULL CHECK (sync_status IN ('pending','synced','failed')),
      last_error     TEXT NOT NULL DEFAULT '',
      created_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      PRIMARY KEY (ext_account_id, instance_id, inbound_tag)
  );
  CREATE INDEX idx_xray_ext_users_node ON xray_ext_users(node_id);

  CREATE TABLE xray_ext_traffic (
      ext_account_id INTEGER NOT NULL REFERENCES xray_ext_accounts(id) ON DELETE CASCADE,
      ym             TEXT NOT NULL,
      uplink         INTEGER NOT NULL DEFAULT 0,
      downlink       INTEGER NOT NULL DEFAULT 0,
      updated_at     TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
      PRIMARY KEY (ext_account_id, ym)
  );

  -- 预设代理组种子（Design2 §3.3）：名称 + 组类型 + 默认成员「🚀直接连接」（强制组名与模板逐字一致）；管理员后续可编辑成员
  INSERT INTO proxy_groups (name, type, preset_key, enabled, definition_json) VALUES
    ('🎬YouTube',     'preset', 'youtube',         1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('🍿Netflix',     'preset', 'netflix',         1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('🍻哔哩哔哩',    'preset', 'bilibili',        1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('📽️国外流媒体',  'preset', 'global-streaming', 1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('🍎苹果海外服务','preset', 'apple-overseas',  1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('🍏苹果国内服务','preset', 'apple-cn',        1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('🤖AI',          'preset', 'ai',              1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('🎮Steam',       'preset', 'steam',           1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}'),
    ('🧩Steam下载',   'preset', 'steam-download',  1, '{"type":"select","nodes":[],"groups":["🚀直接连接"]}');

  -- g) 旧分发模型下线（全新部署口径；业务代码在 Build4 Step 2 全部停止引用）
  DROP TABLE group_selections;
  DROP TABLE subscription_group_rel;
  ```

  > 预设组名称与 `Clash.yaml.template.md` 作者配置的可选组名**逐字一致（含 emoji 前缀）**（渲染时名称即代理组名，Design2 §3.3；组名允许 emoji，见 Design2 §3.3 字符集口径）。全部组类型与模板一致取 `select`。若 SQLite 版本对 partial index 或 CHECK 报错，先确认 SQLite ≥3.35（本项目基线已满足）。

  **2. `backend/internal/dataclear/dataclear.go` 的 `ClearTablesTx` 表清单（严格按此顺序）**

  ```go
  tables := []string{
      // Build4 新增（先子后父，避免外键扫描差异）
      "pool_sync_tasks", "pool_entries", "rule_pools",
      "xray_ext_traffic", "xray_ext_users", "xray_ext_accounts",
      "traffic_records", "xray_users", "group_nodes", "assembly_blueprints",
      "nodes", "xray_instances", "proxy_groups",
      // 既有表（保留）
      "download_tokens", "share_tokens", "rule_tokens", "password_reset_tokens", "oidc_states",
      "access_logs", "versions",
      "custom_subscriptions", "share_subscriptions", "rules", "subscriptions",
      "users", "groups", "platforms", "system_config",
  }
  // 注意：subscription_group_rel、group_selections 必须从清单移除（1009 已 DROP）
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/dataclear/... ./internal/store/...
  # 全新库迁移验证（禁止用旧业务库验证）
  rm -rf /tmp/vpn-sub-b4-mig && mkdir -p /tmp/vpn-sub-b4-mig && APP_MODE=dev DATA_DIR=/tmp/vpn-sub-b4-mig go run ./cmd/server &
  # 等日志出现「迁移已应用 ... 1009_xray.sql」后：
  curl -s http://127.0.0.1:8080/health
  # 检查新表与旧表消失（另确认 nodes 的 display_name 列与 idx_nodes_render_name 唯一索引存在）：
  sqlite3 /tmp/vpn-sub-b4-mig/app-dev.db ".tables"
  sqlite3 /tmp/vpn-sub-b4-mig/app-dev.db "PRAGMA table_info(nodes); PRAGMA index_list(nodes);"
  # 停掉进程后清理 /tmp/vpn-sub-b4-mig
  ```

  > 本机若未安装 `sqlite3` CLI，用 `go test` 内新写的表存在性断言替代；不要跳过表清单核验。

- **验收标准：** 编译/静态检查通过；dataclear 单测通过；全新库迁移成功且 `.tables` 可见全部新表、无 `group_selections`/`subscription_group_rel`；`nodes` 表含 `display_name` 列与 `idx_nodes_render_name` 唯一索引；旧库不做迁移验证（按全新部署口径）。

---

### Step 2：拆除 group_selections / subscription_group_rel 旧分发链路（后端）

**本 Step 完成后，后端不再有任何对 `group_selections`、`subscription_group_rel` 的引用；用户下载按「平台 → 该平台唯一订阅条目 → 当前激活版本」解析；无激活版本时订阅/分享/规则下载与预览返回 HTTP 200 注释块；`/api/home/platforms` 输出新卡片模型。本 Step 不写前端（Step 4 适配）。**

- **目标：** 在后端完成旧模型拆除与新分发链路，恢复 1009 迁移后的可运行状态。
- **前置条件：** Step 1 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/group/group.go`**：
     - 删除 `ErrSubInSelection`、`ErrSubNotLinked`、`Selection` 类型、`SetSelections`、`Selections`、`rebuildSelections`、`diffRemovedSubs`、`OnSubscriptionDeleted`；`Group` 结构删除 `NeedsReselect`、`SubCount`，新增 `DefaultQuota *float64`（JSON 名 `default_quota`，**序列化用 `json:"default_quota,omitempty"`**；扫描 `groups.default_quota` 可空列用 `sql.NullFloat64`）。**advanced_mode=off 时（本 Build 恒 off）`groups.default_quota` 恒为 NULL，列表与详情响应均自然省略该高级字段**，对齐 Design2 §5.10 / UI §9.1「off 时仅返回基础组信息，省略 default_quota」。
     - `Update(ctx, id, name)` 仅改名；不再接收订阅关联与选定。
     - `List` 返回组名、是否默认组、默认配额、组内用户数、`NodeCount`（json `node_count`；直接 `COUNT(*) FROM group_nodes WHERE group_id=?`，本 Build 恒 0，Build6 起有分配数据）；`Get` 返回基础信息。
     - `Delete` 保留「默认组不可删 + 组内用户迁默认组」逻辑，删除关联/选定的代码。
     - 单元测试改为新模型（删除所有 sub/selections 用例，新增 default_quota 回显与迁组断言）。
  2. **`backend/internal/server/group.go`**：
     - 删除 `/api/admin/groups/:id/selections` 路由与 `setSelections` handler；`updateReq` 仅 `{name}`；GET 详情直接返回 `{group}`（不再嵌 selections）；**advanced_mode=off 时 `{group}` 省略 `default_quota`**（omitempty 或条件序列化，仅含基础组信息，Design2 §5.10 / UI §9.1）。
     - 错误映射保留 `ErrDefaultGroup`/`ErrNameConflict`/`ErrNotFound`。
  3. **`backend/internal/subscription/subscription.go`**：
     - 删除 `GroupBrief`、`Groups`、`SelectedBy`、`PlatformGroup`、`SetOnSubscriptionDeleted`、`groupRel`、`selectedByCount`、`CreateInput.GroupIDs/FirstContent`。
     - `Subscription` 新增 `ProductType string`（`json:"product_type"`）。
     - 新增错误 `ErrPlatformOccupied = errors.New("该平台已有订阅条目")`。
     - `Create(ctx, in CreateInput{PlatformID, Name, Slug})`：事务内校验平台存在、`SELECT product_type FROM platforms WHERE id=?` 写入 subscriptions.product_type；先查 `COUNT(*) FROM subscriptions WHERE platform_id=?`，>0 返回 `ErrPlatformOccupied`（事务内查重，防并发绕过 UNIQUE；INSERT 若仍报 UNIQUE 约束也映射 409）。**不再创建首版本**。
     - `Update(ctx, id, name)` 仅改名；`Get`/`List` 改为平铺列表 `[]Subscription`（`List` 内 JOIN platforms 取 platform_name 可选）；`Subscription` 增加 `ContentKind`（json `content_kind`：`blueprint`/`upload`/空，由当前激活版本的 `assembly_blueprints` EXISTS 判定，无激活版本为空——Build5 生成蓝图后自然回填）；移除所有关联/选定代码与回调。
     - `Delete` 保留版本收集/Token 级联/订阅行删除；删除 `onSubDeleted` 调用。
  4. **`backend/internal/server/subscription.go`**：`create` 请求仅 `{platform_id, name, slug?}`；错误映射新增 `ErrPlatformOccupied → 409`；`update` 仅 `{name}`；`list` 直接 `OK(ListData{List: list, Total: ...})`（不再平台分组）。
  5. **`backend/internal/download/download.go`**：
     - `ResolveUserDownload` 默认分支（无 custom、无显式 subscription_id）改为：
       ```sql
        SELECT s.id FROM subscriptions s
        WHERE s.platform_id = ?
       ```
        按平台唯一条目解析（**不要在 SQL 层过滤 current_version**，让 `ReadCurrentWithName` 对 current=0 返回 `version.ErrVersionNotFound`，从而区分两种业务错误）；`ErrNoRows` 返回 `ErrUnassigned`。
        > 注意区分：平台无任何订阅行 → `ErrUnassigned` → HTTP 200 `# error: unassigned`；平台存在订阅行但无激活版本 → `version.ErrVersionNotFound` → 接入层 HTTP 200 `# error: no active version`。二者都必须是 200 纯文本，无效 Token 仍是 404。（两态拆分经 DesignReport10 确认）
     - 显式 Token 分支保留只读兼容（不再新发；用于老库残留 Token 与 Build4 前数据），不新增写入。
     - `PreviewForUser`：移除 `subIDParam` 与管理员指定订阅语义；管理员预览也走「平台 → 唯一订阅 → 当前版本」；普通用户优先自定义、否则平台订阅；无订阅行返回 `ErrUnassigned`，无版本返回 `ErrVersionNotFound`。
     - `withPlatformHeaders` 保持平台附加头；`subscription` 文件名按 subscription.name + 版本原始扩展名（product_type 扩展名规则在 Build5 精细化，本 Step 不阻断）。
  6. **`backend/internal/server/download.go`**：
     - `userDownload`/`shareDownload`/`ruleDownload`/`preview` 的 `errors.Is(err, version.ErrVersionNotFound)` 分支从 `404 Fail` 改为：
       ```go
       setNoCache(c)
       c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: no active version\n"))
       ```
       `preview` 同样改。无效 Token 仍 404 不变。
  6b. **`backend/internal/server/rule.go` 的 `preview`（`/api/rules/:id/preview`）同步改造**：`ReadCurrent` 返回 `version.ErrVersionNotFound` 时改为 `Cache-Control: no-store` + HTTP 200 + `# error: no active version\n`（空规则实体/无激活版本口径，Design2 §5.10「规则端点必改」）。
   7. **`backend/internal/server/home.go`**：
     - `platformCard` 结构重写为 Design2-UI §3.1.3 / §9.3 `api/home.ts` 目标形状：
       - 普通用户：`status: 'custom' | 'unassigned' | 'ready'`（ready=平台唯一订阅有激活版本），字段 `subscription_name`、`subscription_product_type`、`version_updated_at`；无激活版本时不生成下载 Token。
       - 管理员：`status: 'admin_preview'`，字段 `subscription`（`{name, product_type, content_kind: 'blueprint'|'upload', current_version, version_updated_at}` 或 null），`preview_available: bool`。
       - **管理员不再拿 download_token / download_url，不再生成显式 Token**；字段名以前端 Step 4 与 UI §9.3 对齐（后端/前端同一次改动，不保留旧枚举兼容）。
     - 普通用户无标识 Token 仅在平台订阅有激活版本时生成；无激活版本不生成 Token。
     - `groupSelected` 函数删除；`updatedAt` 普通用户可见集合改为「自定义订阅（owner_type='custom'）+ 平台唯一订阅」（修复 Design2 §5.10 指明的问题）；管理员仍为全部订阅。
  8. **`backend/internal/server/server.go`**：
     - 删除 `subSvc.SetOnSubscriptionDeleted(groupSvc.OnSubscriptionDeleted)`；`GroupHandler` 构造不变。
     - `tokenSvc` 照旧；`dlSvc` 构造不变（本 Step 无新依赖）。
  9. **新增/更新单测**：`group_test.go`、`subscription_test.go`、`download_test.go`、`server/download_test.go` 的旧表 fstest 与断言按新模型改写；**新建** `server/home_test.go`（platformCard 新形状）与 `server/rule_test.go`（preview 200 注释块，对应 item 6b）；**`backend/internal/emergency/emergency_test.go` 与 `backend/internal/platform/platform_test.go` 的 fstest 夹具含 `group_selections` / `subscription_group_rel` CREATE TABLE，必须一并按新模型改写**（否则本 Step 验收 grep=0 命中）；**代码目录（backend/ 与 frontend/）`grep -R "group_selections\|subscription_group_rel"` 必须为 0 命中**（与验收命令范围一致，DesignReport6 P2-6）。

- **参考代码/伪代码：**

  **`download.Service.ResolveUserDownload` 默认分支**

  ```go
  default: // 无标识：按平台读唯一订阅条目（Design2 §4.4/§5.10）
      var subID int64
      err := s.store.DB().QueryRowContext(ctx,
          `SELECT id FROM subscriptions WHERE platform_id = ?`, rec.PlatformID).Scan(&subID)
      if errors.Is(err, sql.ErrNoRows) {
          return nil, &AccessEntry{UserID: rec.UserID, Platform: platformSlug, FailReason: "unassigned"}, ErrUnassigned
      }
      if err != nil {
          return nil, nil, err
      }
      content, fileName, err := s.versions.ReadCurrentWithName(ctx, version.OwnerSubscription, subID)
      if errors.Is(err, version.ErrVersionNotFound) {
          return nil, &AccessEntry{UserID: rec.UserID, Platform: platformSlug, Type: "subscription", ResourceID: subID, FailReason: "no_active_version"}, err
      }
      if err != nil {
          return nil, nil, err
      }
      return s.withPlatformHeaders(ctx, content, fileName, rec.PlatformID, "subscription", subID, rec.UserID)
  ```

  **接入层无激活版本分支（四个函数统一）**

  ```go
  case errors.Is(err, version.ErrVersionNotFound):
      h.dlSvc.WriteAccessLog(ctx, ip, entry, false)
      setNoCache(c)
      c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte("# error: no active version\n"))
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  # 限定代码目录，避免命中本 Build 文档与存档文档（DesignReport6 P2-6）
  grep -R "group_selections\|subscription_group_rel" -n backend frontend || true
  ```

- **验收标准：** 编译/静态检查/全部单测通过；grep 无业务代码引用旧表；新下载解析与新 home 形状有单测覆盖（含「平台无订阅行→unassigned」「平台有订阅无版本→no active version」「无效 token→404」三态）。

---

### Step 3：CreateVersion activate 语义、平台 product_type、空规则实体与首页默认规则

**本 Step 完成后，版本组件支持 activate opt-in 与首次入池自动激活；订阅池上传/装配（Build5 接入）不自动激活、其余资源保持创建即激活；平台与订阅全链路携带 product_type；规则允许空实体并支持首页默认规则唯一设置。**

- **目标：** 落地 Design2 §4.4 版本语义与 §5.9 数据模型改造。
- **前置条件：** Step 2 验收通过。
- **产出文件与操作：**

  1. **`backend/internal/version/version.go`**：
     - 新增 `type CreateOptions struct { Activate bool; AfterCreate func(tx *sql.Tx, versionID int64, content []byte) error }`。**AfterCreate 传入的是新插入 `versions.id`（不是 version_no）**，供 assembly_blueprints.version_id 直接使用（DesignReport9 Q11）。
     - `CreateVersion` 签名改为 `CreateVersion(ctx, ot, ownerID, src, opts CreateOptions) (*Version, bool, error)`，第二个返回值为「是否激活」。
     - 事务内逻辑改为：写文件 → 插入版本行（取 `LastInsertId` 作为 versionID）→ `opts.AfterCreate(tx, versionID, content)`（如需；**先于 setCurrent 执行，失败时 current symlink 尚未被改动，只需删刚写文件即可**）→ 读取 owner 当前版本并计算 `effectiveCurrent`（`current==0` 或 `opts.Activate` 则 setCurrentLocked 并激活新版本；否则保持 owner 当前版本）→ `evictOldest(..., effectiveCurrent)`。**`current==0` 判定与 setCurrent 在同一 `BEGIN IMMEDIATE` 事务内完成**（防双首版并发）。
     - `opts.AfterCreate` 出错时删除刚写文件并返回错误（事务回滚，不涉及 symlink 恢复）；文件写失败/记录写失败清理逻辑沿用。
     - 更新所有调用点：`server/versionCreate`、`rule.Create`、`share.Create`、`custom.Upsert`、`subscription`（已无首版调用）；订阅 owner 与后续 assembly 传 `Activate:false`，rule/share/custom 手动上传/文本编辑传 `Activate:true`。
     - 为订阅文本模式补默认文件名：新增 `type TextContent struct { Text []byte; Name string }` 实现 ContentProvider（Build5 装配也会复用）；`versionCreate` 对订阅按 product_type 选名：yaml→`subscription.yaml`，subs/generic-subs→`subscription.txt`；rule→`rule.conf`；share/custom→默认 `subscription.yaml`。
  2. **`backend/internal/server/subscription.go`**：`versionCreate` 调用 `verSvc.CreateVersion(..., version.CreateOptions{Activate:false})`；响应增加 `auto_activated` 布尔（返回第二个值），供前端展示「首个版本已自动激活」。
  3. **`backend/internal/platform/platform.go`**：
     - 常量 `ProductYAML="yaml"`、`ProductSubs="subs"`、`ProductGenericSubs="generic-subs"`；`ProductType` 结构字段加入 `Platform`。
     - `Create`/`Update` 入参增加 productType，校验枚举；`Update` 事务内校验：若存在 `subscriptions.platform_id=? AND product_type != ?`，返回 `ErrProductTypeInUse`（接入层 400），**文案含既有订阅条目的 product_type 插值：「该平台已有 {yaml|subs|generic-subs} 订阅条目，请先处理后再变更产物格式」**（与 Design2-UI §4.4 定稿逐字一致）。
     - `Get`/`List` SELECT 增加 `product_type`；cascadeCounts 不变。
  4. **`backend/internal/server/platform.go`**：create/update 请求增加 `product_type`（默认 `yaml`），错误映射。
  5. **`backend/internal/setup/setup.go`**：`defaultPlatforms` 返回结构增加 `ProductType`；插入 SQL 增加 `product_type` 列；种子口径 **Clash Verge→yaml、v2rayNG→generic-subs、Shadowrocket→subs**。
  6. **`backend/internal/rule/rule.go`**：
     - `Rule` 增加 `IsHomeDefault bool`（json `is_home_default`）。
     - `Create` 允许 `src == nil`（空规则实体）：创建规则行与 Token 后跳过版本创建，`CurrentVersion=0`；`src != nil` 时 CreateVersion 传 `Activate:true`。
     - 新增 `SetHomeDefault(ctx, id, on bool) error`：`BEGIN IMMEDIATE` 事务内存在性校验；`on=true` 时 `UPDATE rules SET is_home_default=0 WHERE is_home_default=1` 再置目标为 1；`on=false` 置 0。
     - `List`/`Get` 查询增加 `is_home_default`。
  7. **`backend/internal/server/rule.go`**：
     - `create`：文本模式 `text` 与文件模式 `file` 均改为可选；两者都缺省时 `src=nil`（空实体）。
     - 新增 `PUT /api/admin/rules/:id/home-default`，请求 `{is_default: bool}`，返回统一成功结构。
  8. **`backend/internal/server/home.go`**：**新增独立汇总端点 `GET /api/home/summary`（会话凭据，DesignReport10 决策）**，返回 `{traffic, home_rule}` 两字段：
     - `traffic`：本 Build 阶段恒 `{unlimited:true}`（高级模式用量/配额字段由 Build6 Step5 补入）；
     - `home_rule`：`{rule_id, name, current_version, token, download_url}` 或 `null`（未设置默认规则/无激活版本）；读取 `rules.is_home_default=1` 规则及其当前激活版本与规则 Token，供首页分流规则卡片展示与复制链接（Design2-UI §3.1.2/§9.3）。
     - **`/api/home/platforms` 恢复纯列表包裹（`{list, total}` 不变）**，不再承载 home_rule/traffic 顶层字段。
     > DesignReport10 决策：home_rule/traffic 不与平台列表混用响应结构，独立端点承载；`traffic_card_enabled` 对流量卡显隐的控制待 Build6 Step5 暴露字段后接入。
  9. **同步更新单测**：`version_test.go`（activate=false 不切当前、首版自动激活、双首版事务）、`platform_test.go`、`setup_test.go`、`rule_test.go`；**`server/home_test.go`（Step 2 新建）补 `/api/home/summary` 用例**：`traffic={unlimited:true}`、未设置默认规则时 `home_rule=null`、**删除 `is_home_default=1` 规则后 `home_rule` 返回 null（首页分流规则卡片回未设置空态）**。

- **参考代码/伪代码：**

  **`version.CreateVersion` 核心事务改动**

  ```go
  type CreateOptions struct {
      Activate    bool
      AfterCreate func(tx *sql.Tx, versionID int64, content []byte) error
  }

  func (s *Service) CreateVersion(ctx context.Context, ot OwnerType, ownerID int64, src ContentProvider, opts CreateOptions) (*Version, bool, error) {
      // 读内容/大小校验不变 ...
      var created *Version
      activated := false
      err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
          // 写文件、INSERT versions 后取得 versionID（LastInsertId） ...
          if opts.AfterCreate != nil {
              // 先于 setCurrent：失败时 current symlink 尚未改动，只需删刚写文件
              if err := opts.AfterCreate(tx, versionID, content); err != nil { _ = os.Remove(full); return err }
          }
          current, err := ownerCurrent(ctx, tx, ot, ownerID)
          if err != nil { _ = os.Remove(full); return err }
          if current == 0 || opts.Activate {
              if err := s.setCurrentLocked(ctx, tx, ot, ownerID, newNo); err != nil { _ = os.Remove(full); return err }
              activated = true
              current = newNo
          }
          if err := s.evictOldest(ctx, tx, ot, ownerID, current); err != nil { return err }
          created = &Version{No: newNo, FilePath: rel, FileName: fileName, Current: activated}
          return nil
      })
      return created, activated, err
  }
  ```

  **平台种子改动**

  ```go
  return []struct{ Name, Description, Schemes, ExtraHeaders, ProductType string }{
      {"Clash Verge", "...", `["clash://install-config?url={url}"]`, `{...}`, "yaml"},
      {"v2rayNG", "...", `["v2rayng://install-config?url={url}"]`, `{}`, "generic-subs"},
      {"Shadowrocket", "...", `["shadowrocket://add/{url}"]`, `{}`, "subs"},
  }
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  ```

- **验收标准：** 全部单测通过；新单测覆盖：activate=false 不激活（已有当前版本）、首版自动激活、rule/share/custom 手动版本仍激活、平台 product_type 种子三值、产品格式变更冲突、空规则实体与首页默认唯一；`/api/home/summary` 单测覆盖 `traffic={unlimited:true}`、`home_rule` 未设置为 null、**删除默认规则后 `home_rule` 回 null**。

---

### Step 4：前端基线适配 + advanced_mode 状态与路由骨架

**本 Step 完成后，前端在 TypeScript 层面适配新后端契约并可构建；侧边栏「用户组」菜单与 `/admin/groups`、`/admin/xray` 路由由 `advanced_mode` 驱动（当前为 off，隐藏）；订阅/平台/规则/首页/版本页呈现 Design2-UI §3.1/§4.1/§4.2/§4.4/§4.6 的基础形态（装配入口、蓝图标签、重新编辑留 Build5）。**

- **目标：** 前端基线适配，保证 Build4 结束时系统可运行、可点击、无旧模型残留。
- **前置条件：** Step 3 验收通过。
- **产出文件与操作：**

  1. **后端小改 `backend/internal/server/status.go`**：`/api/system/status` 响应增加 `advanced_mode` 布尔（`cfg.GetBool(ctx, "advanced_mode", false)`；应急模式恒 false）。配置文件新增常量 `config.KeyAdvancedMode = "advanced_mode"`（在 `internal/config/config.go`）。**`traffic_card_enabled` 键不在本 Build 暴露，由 Build6 Step5 补入 status 响应**（DesignReport6 Q3）。
  2. **`frontend/src/api/system.ts`**：`SystemStatus` 类型增加 `advanced_mode: boolean`（该类型在 `vite-env.d.ts` 或全局声明处，同步更新）。
  3. **`frontend/src/api/subscription.ts`**：类型与函数按新模型重写——`SubscriptionItem { id, slug, name, platform_id, product_type: 'yaml'|'subs'|'generic-subs', current_version, content_kind: 'blueprint'|'upload'|null, platform_name? }`；`listSubscriptions` 返回平铺列表；`createSubscription({platform_id, name, slug?})`；`updateSubscription(id,{name})`。
  4. **`frontend/src/api/home.ts`**：`PlatformCard` 与响应类型按 Design2-UI §3.1/§9.3 更新（普通用户 `status: 'custom'|'ready'|'unassigned'`，管理员 `status:'admin_preview'`）；**新增 `getHomeSummary()` 调 `GET /api/home/summary`，返回 `{traffic, home_rule}`**（`traffic` 本 Build 恒 `{unlimited:true}`；`home_rule` 形状见 Step 3 item 8）；`/api/home/platforms` 保持纯列表包裹。`refreshHomeToken` 逻辑不变但仅 ready/custom 时可用。
  5. **`frontend/src/api/group.ts`**：`GroupItem` 更新为 `{id, slug, name, is_default, default_quota?: number|null, node_count, user_count}`（**advanced_mode=off 时后端省略 `default_quota`，字段可缺省**）；`updateGroup(id,{name})`；删除 `setSelections` 与 `SelectionItem`。
  6. **`frontend/src/api/platform.ts`**：类型与 create/update 请求增加 `product_type`。
  7. **`frontend/src/api/rule.ts`**：`RuleItem` 增加 `is_home_default`；新增 `setHomeDefault(id,is_default)`；`createRule` 类型允许无首版本（FormData 无 file 或 JSON 无 text）。
  8. **`frontend/src/api/version.ts`**：`VersionItem` 增加 `blueprint: boolean`（后端 Build5 才回传，现在可缺省）；`versionApi` 类型不变。
  9. **`frontend/src/stores/system.ts` / `router/index.ts` / `layouts/AdminLayout.vue`**：
     - 菜单增加「节点 `/admin/nodes`」「代理组 `/admin/proxy-groups`」「Xray 实例 `/admin/xray`」三项（Build5/7 页面落地前可暂用占位组件，或路由懒加载到 Build5 组件——**本 Step 只加路由与占位**）；「用户组」与「Xray 实例」菜单 `v-if="system.status?.advanced_mode"` 渲染；节点/代理组/订阅装配始终显示。
     - 路由守卫：`to.path` 为 `/admin/groups` 或 `/admin/xray` 且 `system.status?.advanced_mode !== true` → `return '/admin/subscriptions'` + `message.warning('高级功能未开启，请在面板配置中开启高级模式')`（**用 `!== true` 而非 `=== false`：系统状态未加载（status 为 null）时同样视为 off 并重定向**，按 Design2-UI §2.4）。
  10. **`frontend/src/views/admin/SubscriptionsView.vue`**：按 Design2-UI §4.1 改造为平铺双态列表：平台名、订阅名、product_type 标签（yaml 蓝 / subs 青 / generic-subs 紫）、当前版本与「未激活」灰字、操作（版本管理/编辑/删除）；新建弹窗仅平台+名称，占用平台禁用并标「（已有订阅）」；新建成功轻提示「**可上传内容或前往订阅装配生成模板**」（UI §4.1）；删除 ConfirmModal 新影响清单；PageHeader 右侧「前往装配」按钮（跳 `/admin/assembly`，本 Build 目标页存在但装配器为占位）。**移除组关联多选与「加入组可用范围」引导**。**「内容形态标签」列（装配模板紫 / 直接上传灰）由 Build5 补齐**（后端 `content_kind` 字段本 Build 已提供，当前全量为上传内容）。**「已入池未生效」引导（Design2 §4.4 / UI §4.1）：上传或装配生成完成后，对应行临时高亮 + 行内 `a-alert info` 风格标签「已入池未生效，请激活」+「去激活」快捷链接（直达该订阅版本管理页）**。
  11. **`frontend/src/views/admin/VersionManageView.vue`**：订阅/规则页的「设为当前」文案改「激活/分发」，确认文案「激活后对全体用户生效」；分享/自定义保持「设为当前」。**蓝图标签与重新编辑按钮本 Build 不实现（Build5）**；创建成功订阅版本时若响应 `auto_activated=true` 提示「首个版本已自动激活」；**`auto_activated=false`（非首个版本）时同口径提示「已入池未生效，请激活」**（Design2 §4.4 / UI §4.1 入池未生效引导）。
  12. **`frontend/src/views/admin/PlatformsView.vue` / `PlatformEditView.vue`**：列表新增 product_type 列；新建/编辑表单 `a-radio-group` 三选一，默认 yaml；编辑提交 400 时表单级展示后端文案（Design2-UI §4.4）。
  13. **`frontend/src/views/admin/RulesView.vue`**：创建弹窗首版本改为可选（「暂不创建版本」）；列表无激活版本实体展示灰字「无激活版本」；新增「首页默认展示」单选列，切换走 ConfirmModal 与 `setHomeDefault`；**默认行专设「取消默认」操作（仅默认行显示，ConfirmModal 确认后调 `setHomeDefault(id,false)`，Design2-UI §4.6 定稿口径）**。
  14. **`frontend/src/views/admin/GroupsView.vue`**：按当前后端契约最小化改造（Build7 再做节点分配等高级 UI）：列表显示组名、默认组标签、默认配额（**off 时后端省略 `default_quota`，前端缺省即显示「不限流量」**）、用户数；编辑仅改名；删除文案保留迁默认组。因 advanced_mode=false 菜单隐藏，本页仅兜底编译与深链重定向。
  15. **`frontend/src/views/HomeView.vue`**：按 Design2-UI §3.1 改造卡片顺序与形态（**流量卡与分流规则卡数据源为 `getHomeSummary()`**）：流量卡（基础模式仅「不限流量」= `traffic.unlimited`，受 `traffic_card_enabled` 配置待 Build7——本 Build 恒显示）→ 分流规则卡（读 `home_rule`，空态文案「管理员暂未设置分流规则」，SR 双内容引导，点击跳 `/rules`）→ 平台卡（普通用户 ready/unassigned/custom 三态；**无激活版本态显示灰色占位「暂无可用版本，请联系管理员」+ 一键导入/复制链接/刷新链接三按钮隐藏（UI §3.1.3）**；管理员 admin_preview 仅「按平台预览当前版本」按钮，无激活禁用）。**管理员平台卡不再生成/展示 Token 与复制链接**。
  16. **`frontend/src/views/ProfileView.vue`**：基本信息新增「本月流量」行（基础模式「不限流量」）；「所属组」行基础模式隐藏（读 advanced_mode）。
  17. **`frontend/src/components/PageHeader.vue`、`frontend/src/components/CopyField.vue`**：按 Design2-UI §1.3 新建通用组件（标题 + 副标题 + 右侧操作区；复制字段按钮 + Toast）。后续页面统一使用，禁止各页继续复制实现。
  18. **`frontend/src/AppHeader.vue`**：所属组名标签基础模式隐藏（读 `advanced_mode`）。

- **参考代码/伪代码：**

  **菜单可见性（AdminLayout.vue 核心；最终菜单顺序必须严格按 UI §2.1 的 13 项，示例只表达显隐逻辑）**

  ```ts
  const system = useSystemStore()
  const adv = computed(() => system.status?.advanced_mode === true)
  const menuItems = computed(() => {
    const items = [
      { key: '/admin/subscriptions', icon: ..., label: '订阅' },
      { key: '/admin/shares', ... },
      { key: '/admin/platforms', ... },
      { key: '/admin/users', ... },
      { key: '/admin/approvals', ... },
      { key: '/admin/rules', ... },
      { key: '/admin/settings', ... },
      { key: '/admin/logs', ... },
      { key: '/admin/assembly', ... },
      { key: '/admin/nodes', ... },
      { key: '/admin/proxy-groups', ... },
    ]
    if (adv.value) {
      items.splice(1, 0, { key: '/admin/groups', ... }) // 用户组仅高级模式
      items.push({ key: '/admin/xray', ... })           // Xray 实例仅高级模式
    }
    return items
  })
  ```

  **`api/request.ts` 后续需支持 403 高级未开启的特殊提示（Build7 使用），本 Step 先不动。**

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test
  cd ../backend && go build ./... && go vet ./... && go test ./...
  ```

- **验收标准：** 前后端构建与测试通过；前端无旧 `group_ids`/`selections`/`admin_pool`/`subscriptions[]` 类型引用（grep 核对）；`/api/system/status` 返回 `advanced_mode:false`；`/api/home/summary` 返回 `{traffic:{unlimited:true}, home_rule:null}`（未设置默认规则时）；手动访问 `/admin/groups` 被守卫重定向并提示（**含系统状态未加载场景，`!== true` 判定，前端路由单测覆盖**）；订阅上传后行内出现「已入池未生效，请激活」引导（含「去激活」链接）；普通用户平台卡无激活版本态显示占位文案「暂无可用版本，请联系管理员」且三按钮隐藏。

---

### Step 5：规则素材池后端（CRUD / 解析白名单 / URL 同步任务 / 定时同步）

**本 Step 完成后，后端具备 Design2 第二章全部素材池能力：池/条目 CRUD、逐行解析与白名单校验、手动+URL 双来源隔离、URL 差量同步、异步任务持久化与轮询端点、可选每日定时同步。**

- **目标：** 实现 Design2 §2.1~§2.5 的 `internal/pool` 业务包与 `server/pool.go` 接入层，以及 cron 定时检查与启动重置。
- **前置条件：** Step 4 验收通过（先有前端骨架再补后端也可，但按序号先做本 Step）。
- **产出文件与操作：**

  1. **`backend/internal/pool/pool.go`**：模型与 CRUD。
     - `Pool { ID, Name, URLs []string, EntryCount, LastSyncedAt, SyncStatus, SyncError, AutoSync, SyncTime }`。
     - `Entry { ID, PoolID, RuleType, MatchValue, Source, SortOrder }`。
     - `Create/Update`：name 唯一（409）、URL 列表校验（http/https、去重、禁止控制字符）、auto_sync/sync_time 校验（HH:MM）。（**URL http/https scheme 校验经 DesignReport10 确认**：Design2 §2.4「目标地址不设限制」指不设白名单/域名限制，scheme 合法性校验保留。）
     - `ListPools` 带 entry_count 聚合；`ListEntries(pool,page,size)` 按 `sort_order,id` 排序分页。
     - `CreateEntry/UpdateEntry/DeleteEntry`：仅 manual；类型白名单 + 匹配值白名单；唯一冲突返回 `ErrEntryConflict`（409）。
  2. **`backend/internal/pool/parser.go`**：逐行解析。
     - 支持 `full:<domain>` → DOMAIN；裸域名 → DOMAIN-SUFFIX；标准规则行 `TYPE,VALUE`（逗号多于两段只取前两段并记录 skip 原因）；空行/`#` 注释行忽略；无法识别 skip 不阻断。
     - 全部入库值过 `ValidateEntry(ruleType, matchValue)`：域名 lowercase、无 scheme/路径/空格/逗号/控制字符；IP-CIDR/6 用 `net.ParseCIDR` 并回写规范串；PROCESS-NAME/PROCESS-NAME-REGEX/USER-AGENT 禁止逗号/换行/控制字符且长度上限（建议 512）。**规则类型白名单：DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / IP-CIDR / IP-CIDR6 / PROCESS-NAME / PROCESS-NAME-REGEX / USER-AGENT**。
     - 去重：同一 URL 结果内先按（类型,值）合并；同值冲突时保留首个并记 skip。
  3. **`backend/internal/pool/sync.go`**：同步任务。
     - `SubmitSync(ctx, poolID)`：**在同一个 `BEGIN IMMEDIATE` 事务内**完成「池存在性检查 + 是否已有 running 任务检查 + 插入 `pool_sync_tasks(status='running')`」（DesignReport9 Q12-8），已有 running 任务则返回 `ErrSyncRunning`（409）；事务提交后启动 goroutine `runSyncTask`，返回 task id。
     - `runSyncTask`：串行拉取全部 URL（`http.Client{Timeout: 60s}`，`io.LimitReader(50MB+1)`）；每个 URL 结果 `{url, ok, added, removed, skipped, error}`；**任一 URL 失败、空响应或零有效条目，则该 URL ok=false；只有全部 URL 成功才执行 url 来源差量删除**；成功 URL 的条目照常 upsert。
     - **任务边界**（DesignReport9 Q12-9）：删除池或编辑池 URL 不取消已启动任务；池已被删除时，任务终态写回失败仅记日志、不崩溃；URL 编辑只影响下一次同步；任务历史按保留策略继续展示。
     - 入库（单个事务）：新 url 条目 sort_order 从 `MAX(当前 URL 段最大序号, urlBase-1)+1` 起追加；**既有条目 sort_order 一律不改写**；删除仅删 `source='url'` 且不在本次成功结果并集中的行，且只在无任何失败时执行。manual 条目不触碰。**差量删除实现不得用单条 NOT IN 大列表**（数万行规模会超 SQLite 参数上限：默认 999 / 编译上限 32766，Design2 §2.4 要求支持数万行规模）——采用临时表 JOIN 删除或分批（chunk）删除，见参考代码。
     - 终态写回任务行（succeeded/failed/partial）并更新 rule_pools.last_synced_at/sync_status/sync_error；**同事务内顺手清理该池超期历史：`DELETE FROM pool_sync_tasks WHERE pool_id=? AND finished_at < datetime('now','-7 days')`（保留 7 天口径，Design2 §5.9）**。
     - `GetStatus(ctx, poolID)`：读最近一次任务，返回 `{task_id,status,per_url,started_at,finished_at,error}`。
       - `ListTasks(ctx, poolID, page, pageSize)`：按 id DESC 分页读历史任务（供 UI §5.2.2 历史列表）。
  4. **`backend/internal/pool/sort.go`（或并入 pool.go）**：排序口径实现。
     - `sortOrderManual` 从 0 起；`sortOrderURLBase = 1 << 30`（manual 段恒小于 url 段）。**manual 新增取 manual 段内 `MAX(sort_order WHERE sort_order < urlBase)+1`；url 新增取 URL 段内 `MAX(sort_order, urlBase-1)+1`**；两段各自维护，互不穿越。注释说明「两段拼接、系统维护」。
  5. **`backend/internal/server/pool.go`**：路由 `/api/admin/pools`（session+admin）：
     - `GET /api/admin/pools`；`POST /api/admin/pools`；`PUT/DELETE /api/admin/pools/:id`；
     - `GET /api/admin/pools/:id/entries?page=&page_size=`（默认 20，上限 100）；
     - `POST/PUT/DELETE /api/admin/pools/:id/entries(/:entryId)`；
     - `POST /api/admin/pools/:id/sync` → `{task_id}`；`GET /api/admin/pools/:id/sync/status` → 状态形状按 Design2-UI §9.1；`GET /api/admin/pools/:id/sync/tasks?page=&page_size=` → 历史任务分页列表（同 UI §9.1 listSyncTasks）。
  6. **`backend/internal/server/server.go`**：构造 `poolSvc := pool.NewService(st, log)`，注册 `RegisterPoolRoutes`。
  7. **`backend/internal/cron/pool.go` 或并入现有 cron 包**：`StartPoolAutoSync(db, poolSvc, lg)` 每 1 分钟 tick：查 `auto_sync=1 AND sync_time=?`（当前 UTC `15:04`）的池，逐个 `SubmitSync`；已有 running 或 ErrSyncRunning 跳过；返回 stop 函数。**同一分钟内只触发一次**（用 `lastRun map[string]bool` 或 DB running 判重）。
  8. **`backend/cmd/server/main.go`**：启动时执行 `UPDATE pool_sync_tasks SET status='failed', error='服务重启，任务中断', finished_at=CURRENT_TIMESTAMP WHERE status='running'`，并**同步执行 `UPDATE rule_pools SET sync_status='failed', sync_error='服务重启，任务中断' WHERE id IN (SELECT DISTINCT pool_id FROM pool_sync_tasks WHERE status='failed' AND error='服务重启，任务中断')`**（快照与任务行一致，DesignReport7 P2-3）；`StartPoolAutoSync` 启动并注册 stop。
  9. **单测**：parser 全规则类型/白名单/零条目；sort 两段；sync 用 `httptest.Server` 覆盖「全成功差量删除」「单 URL 失败不删除」「空响应失败」「零条目保护」；CRUD 唯一冲突与分页；**终态写回清理超 7 天历史任务（超期行被删、7 天内行保留）**。

- **参考代码/伪代码：**

  **解析核心（parser.go）**

  ```go
  var supportedTypes = map[string]bool{
      "DOMAIN": true, "DOMAIN-SUFFIX": true, "DOMAIN-KEYWORD": true,
      "IP-CIDR": true, "IP-CIDR6": true,
      "PROCESS-NAME": true, "PROCESS-NAME-REGEX": true, "USER-AGENT": true,
  }

  // ParseLine 返回 (ruleType, matchValue, skipReason)；ok=false 表示跳过并记录原因
  func ParseLine(raw string) (string, string, string, bool) {
      line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
      if line == "" || strings.HasPrefix(line, "#") {
          return "", "", "空行或注释", false
      }
      if strings.HasPrefix(line, "full:") {
          v := strings.TrimSpace(strings.TrimPrefix(line, "full:"))
          return "DOMAIN", v, "", v != ""
      }
      if !strings.Contains(line, ",") {
          return "DOMAIN-SUFFIX", line, "", line != "" // 裸域名 → DOMAIN-SUFFIX
      }
      parts := strings.SplitN(line, ",", 3)
      typ, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
      if len(parts) > 2 {
          // 仅取前两段，多余段忽略并记录
      }
      return typ, val, "标准规则行", true
  }

  // ValidateEntry 入库白名单（DOMAIN/DOMAIN-SUFFIX 转小写并拒绝 , \r \n 与首尾空白等）
  func ValidateEntry(typ, val string) (string, error) { ... }
  ```

  **同步差量事务（sync.go 关键逻辑）**

  ```go
  err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
      for _, r := range results {
          if !r.OK { continue }
          for _, e := range r.Entries {
              // INSERT ... ON CONFLICT(pool_id, rule_type, match_value) DO UPDATE SET
              //   match_value=excluded.match_value, updated_at=CURRENT_TIMESTAMP
              // 仅当 source='url' 时才允许本 UPSERT；若冲突行 source='manual' 则跳过（manual 优先保留）
          }
      }
      if !partial {
          // 差量删除（数万行规模，单条 NOT IN 大列表会超 SQLite 参数上限：默认 999 / 编译上限 32766）
          // 方案 A（推荐）临时表 JOIN：
          //   CREATE TEMP TABLE _sync_keep(pool_id INTEGER, rule_type TEXT, match_value TEXT);
          //   分批 INSERT 本次全部成功 URL 的并集条目；
          //   DELETE FROM pool_entries WHERE pool_id=? AND source='url'
          //     AND (rule_type, match_value) NOT IN
          //       (SELECT rule_type, match_value FROM _sync_keep WHERE pool_id=?);
          //   成功结果为空也要清空 url 段（全失败已在 partial 分支拦截，此处仅全成功路径）。
          // 方案 B：按 (rule_type, match_value) 分批 NOT IN（每批 <500 对）循环删除。
      }
      return nil
  })
  ```

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./internal/pool/... ./internal/cron/... ./...
  ```

- **验收标准：** 全部测试通过；`grep -R "ParseLine\|ValidateEntry" backend/internal/pool` 有实现与测试；同步状态端点形状与 UI §9.1 一致；任务持久化与启动置 failed 有测试覆盖；**差量删除走临时表 JOIN / 分批实现（不依赖单条 NOT IN 大列表）并有「全成功差量删除」单测覆盖**。

---

### Step 6：规则素材池前端（AssemblyView 页签壳 + 池列表/详情/同步轮询）

**本 Step 完成后，`/admin/assembly` 为五页签页面：规则素材池完整可用；其余四个装配器页签为「后续 Build」占位（Build5 实现）。同步采用 pollTask 轮询并可卸载取消。**

- **目标：** 落地 Design2-UI §5.1/§5.2/§9.1/§9.2 的素材池部分。
- **前置条件：** Step 5 验收通过。
- **产出文件与操作：**

  1. **`frontend/src/api/request.ts`**：新增 `pollTask` 与单请求 timeout 覆写能力（契约严格按 Design2-UI §9.2）：
     - `pollTask({submit, query, interval=1500, timeout=5*60*1000})`；终态 `succeeded/failed/partial`；超时抛特定错误；返回 `{result, cancel}`；连续 3 次网络失败才终止。
     - `http` 调用支持 `config.timeout` 覆写（axios 原生即可，封装一个便捷函数 `httpWithTimeout` 或文档说明直接传 config；禁止改全局 15s 默认值）。
  2. **`frontend/src/api/pool.ts`**：函数与类型完全按 Design2-UI §9.1 `api/pool.ts` 表实现（`listPools/createPool/updatePool/deletePool/listEntries/createEntry/updateEntry/deleteEntry/submitSync/getSyncStatus/listSyncTasks`）。
  3. **`frontend/src/views/admin/AssemblyView.vue`**：重写为 `PageHeader` + `a-tabs` 五页签（key：`pool/clash-yaml/sr-subs/generic-subs/sr-conf`）；URL query `?tab=` 驱动，无效回退 `pool`；每个页签懒加载/独立挂载；`pool` 页签实现，其余四个渲染 `a-result`「将在下一轮构建实现」占位（**不要**沿用旧的「即将推出」整页占位）。
  4. **新建池页签子组件**（建议 `frontend/src/views/admin/assembly/PoolTab.vue`、`PoolDetail.vue`、`PoolFormModal.vue`，目录不存在则创建）：
     - 池列表双态列表（≥768 表格 / <768 卡片）：池名、URL 数、条目数、上次同步（本地时区）、同步状态 Badge（色系：pending 橙 / running 蓝 processing / succeeded 绿 / failed 红 / partial 橙；失败附原因 Tooltip）、定时同步行内 `a-switch` + 「每日 04:00 UTC」、操作（详情/同步/编辑/删除）。
     - 新建/编辑弹窗 480px：名称 + URL 动态列表（http/https 校验）+ 定时开关与 `a-time-picker`（副说明 UTC）。
     - 详情面包屑「素材池 / {池名}」：顶部信息条 + 条目分页表（默认 20/页；来源 manual/url 段分隔标题行；规则类型 Tag、匹配值 code、手动条目增删改；不提供条目级排序控件）。
     - **同步历史列表**：详情页内分区/弹窗分页展示最近 N 条任务（后端保留 7 天，超期行已在终态写回时清理；状态 Badge、开始/结束时间、逐 URL 明细摘要、错误 Tooltip），调用 `listSyncTasks`（Design2-UI §5.2.2）。
     - 同步流：点「同步」→ `submitSync` 得 task_id → `pollTask` 轮询 `getSyncStatus` → 按钮 loading、池行/详情「同步中…」；终态展示逐 URL 回执（成功/失败/部分失败文案按 Design2-UI §5.2.3）；**进行中再点：后端返回 409（ErrSyncRunning）→ 前端按后端错误串匹配或专用错误码特判为 `message.warning`「同步进行中，请等待完成」**（不落入 §9.4 通用 409 `Notify.error`，UI §5.2.3）；组件卸载调用 cancel（后端任务不中断）。
     - 删除池 ConfirmModal：「池内 N 条条目将级联删除；已装配版本为快照不受影响」。
  5. **`frontend/src/views/admin/AssemblyView.vue` 之外的复用**：`PageHeader`/`CopyField`（Step 4 已建）在池页使用；`TriStateList`/`ConfirmModal`/`Notify` 沿用。
  6. **前端单测**：新增 `frontend/tests/pool-tab.spec.ts` 与 `request-poll.spec.ts`（mock axios），覆盖列表加载、同步轮询终态与卸载取消、进行中重复触发提示、**同步历史列表分页**、空态文案。

- **参考代码/伪代码：**

  **`pollTask` 实现要点**

  ```ts
  export async function pollTask<T>(opts: {
    submit: () => Promise<unknown>
    query: () => Promise<T>
    isDone: (r: T) => boolean
    interval?: number
    timeoutMs?: number
    signal?: AbortSignal
  }): Promise<T> {
    await opts.submit()
    const deadline = Date.now() + (opts.timeoutMs ?? 300_000)
    let failures = 0
    while (Date.now() < deadline) {
      if (opts.signal?.aborted) throw new PollAbortedError()
      try {
        const r = await opts.query()
        failures = 0
        if (opts.isDone(r)) return r
      } catch (e) {
        if (++failures >= 3) throw new PollNetworkError()
      }
      await sleep(opts.interval ?? 1500, opts.signal)
    }
    throw new PollTimeoutError('任务仍在后台执行，请稍后刷新查看结果')
  }
  // 池同步调用：isDone = r => ['succeeded','failed','partial'].includes(r.status)
  ```

- **测试与验收命令：**

  ```bash
  cd frontend && npm run build && npm test
  cd ../backend && go test ./...
  ```

- **验收标准：** 前端构建与测试通过；`/admin/assembly` 五页签可切换且 query 生效；素材池全流程手工走查通过（新建→同步→详情→手动条目→删除）；轮询卸载取消与超时兜底文案有单测。

---

### Step 7：Build4 端到端验收与文档核对

**本 Step 完成后，Build4 里程碑达成，可归档进入 Build5。**

- **目标：** 全新部署验证基础模式主链路，并核对文档/设计覆盖。
- **前置条件：** Step 0~6 全部验收通过。
- **产出文件与操作：**

  1. 全新库启动验证（不要用旧业务库）：`rm -rf /tmp/vpn-sub-b4-final`，`APP_MODE=dev DATA_DIR=/tmp/vpn-sub-b4-final go run ./cmd/server`。
  2. 手工冒烟清单：
     - `/health` 200；`/api/system/status` 返回 `configured=false, advanced_mode=false`。
     - 走 Setup 快速开始 → 自动登录首管理员；侧边栏无「用户组」，有「订阅装配/节点/代理组」占位。
     - 平台列表三个默认平台 product_type 为 yaml/generic-subs/subs。
     - 订阅管理：同平台重复创建被 409 提示；创建成功后无「首版本」，订阅列表「未激活」；上传内容版本后「首个版本已自动激活」；用户端平台卡可见复制链接；管理端平台卡仅「按平台预览当前版本」。
     - 规则：创建空规则实体成功，列表显示「无激活版本」；设置首页默认后首页分流规则卡空态切换；规则下载无版本返回 `# error: no active version`。
     - 素材池：新建池 → 添加 `https://...` 测试 URL（或本地 `python3 -m http.server` 提供 txt）→ 同步成功/失败回执 → 手动条目 → 定时开关保存。
  3. 更新本文件「TODOLIST CheckList」与「一、构建进度追踪」全部勾选为 ✅，变更记录追加新版本行。
  4. 核对：
     - `grep -R "group_selections\|subscription_group_rel" backend frontend`（本文件与存档除外）应为空。
     - `grep -R "TODO(Build4\|FIXME(Build4" backend frontend` 应为空。
     - Build4 覆盖 Design2 第一~二章及 §4.4/§5.9/§5.10 中属于本 Build 的条目；未覆盖项必须在本文档「五、候选构建项」列出并指向 Build5/6/7。

- **测试与验收命令：**

  ```bash
  cd backend && go build ./... && go vet ./... && go test ./...
  cd ../frontend && npm run build && npm test
  ```

- **验收标准：** 全部命令通过；手工冒烟清单无未关闭问题；核对无遗漏（遗漏项进入候选清单并明确归属后续 Build）。

---

## 五、候选构建项（已确认归属后续 Build，不在本 Build 实现）

| # | 候选 | 说明 | 归属 |
|---|------|------|------|
| 1 | manual 节点与协议注册表 | Design2 §3.2/§5.10 `GET /api/admin/nodes/protocols` | Build5 |
| 2 | 代理组管理 | Design2 §3.3 | Build5 |
| 3 | 四类装配器与装配快照/预览/diff | Design2 §3~4、UI §5 | Build5 |
| 4 | 装配入口与重新编辑 UI | Design2 §4.4、UI §4.2/§5.4 | Build5 |
| 5 | 每用户下载动态渲染与 Subscription-Userinfo 头 | Design2 §5.7 | Build6 |
| 6 | Xray 实例/节点检测/组分配/用户同步/流量配额 | Design2 §5.1~5.11 | Build6/7 |
| 7 | 高级模式开关、实例/独立账号/对账管理 UI | Design2 §5.10/§5.11、UI §八 | Build7 |

---

## 六、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-19 | 初始版本：Build4 构建方案（Go 1.26 + 1009 迁移 + 旧分发拆除 + 规则素材池），7 个 Step；后续 Build5/6/7 范围说明 |
| v1.1 | 2026-08-19 | DesignReport5 核验修订：预设组种子默认成员改为 `groups:["直接连接"]`（空节点数组）；首页 home_rule 字段统一为 `{rule_id,name,current_version,token,download_url}` |
| v1.2 | 2026-08-19 | DesignReport6 核验修订：Step2 grep 验收命令限定 backend/frontend 目录（P2-6）；Step4 status.go 注记 `traffic_card_enabled` 由 Build6 Step5 补入（Q3） |
| v1.3 | 2026-08-19 | DesignReport7 修订：Step1 允许启动旧服务做迁移/健康验证并可清空数据重新开始（P2-1）；YouTube 种子改 select（P2-16）；两段排序 manual/URL 各自维护（Q6）；启动同步刷新 rule_pools 快照（P2-3）；池历史任务端点与 UI（Q12） |
| v1.4 | 2026-08-19 | 构建前核验修订（用户确认）：预设组种子改用 `Clash.yaml.template.md` 模板 emoji 名（Step1 种子与注记）；同步任务历史保留 7 天超期动态清理（约束表 + Step5 终态清理与单测 + Step6 历史列表注记）；RulesView 取消默认固定为专设操作（Step4 item 13） |
| v1.5 | 2026-08-19 | DesignReport8 修订：预设组种子默认成员同步改 `groups:["🚀直接连接"]`（M4 强制组 emoji 连锁）；Step2 单测清单补 server/home_test.go 与 server/rule_test.go |
| v1.6 | 2026-08-19 | DesignReport9 修订：CreateVersion 事务顺序改为「AfterCreate 先于 setCurrent」且 AfterCreate 传 versions.id（Q3/Q11）；池同步 SubmitSync 查+插同事务（Q12-8）；补同步期间删池/改 URL 边界（Q12-9） |
| v1.7 | 2026-08-19 | DesignReport10 核验修订：Step2 测试清单补 emergency/platform 两测试文件与组详情 off 省略 default_quota；Step3 ErrProductTypeInUse 文案含类型插值、home_rule/traffic 改独立端点 /api/home/summary；Step4 入池未生效引导/平台卡空态三按钮隐藏/新建订阅轻提示/路由守卫 !== true；Step5 差量删除改临时表或分批；Step6 同步进行中 409 前端特判 warning |
| v1.8 | 2026-08-19 | Build4 全部 Step 执行验收通过（2026-08-19）。执行口径记录：Step2 旧表 grep 按用户决策限定 `backend/internal`、`backend/cmd`、`frontend`（历史迁移 1002/1003/1009 保留迁移链、不作为业务代码命中）；Go 1.26 经下载工具链执行，前端测试环境补齐 localStorage 内存实现（Node 26 + jsdom 组合）；Step7 全新库自动冒烟覆盖 Setup/平台种子/订阅唯一与首版自动激活/普通用户与管理员首页形态/空规则与默认规则/规则下载 200 注释块/素材池同步与 manual 段排序 |
| v1.9 | 2026-08-20 | 第三轮事后验收修订：Build4 Step4~7 因 Issue2 R14 未关闭项未完全达标，状态回退为 ◧ 修复收口中（Step0~3 保持 ✅）；用户决策先执行 Build4/5 修复收口，详细清单见 Issue2 附件优先级 0 与 docs/reports/BuildReport/BuildReport1.md |
| v1.10 | 2026-08-20 | Build4/5 修复收口完成：Issue2 R14 相关未关闭项已全部按状态关闭；Build4 Step4~7 恢复为 ✅ 验收通过。验证：后端 `go build/vet/test ./...`、前端 `npm run build` + `npm test`（45 passed）通过。 |
