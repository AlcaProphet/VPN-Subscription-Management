# Build37 — 节点推荐选项省略验证来源

以 `beta` 提交 `2959a68` 为基线。用户确认的范围见 [Issue21 R35-06](../Issue/Issue21.md#r35-06-节点创建和编辑界面显示-mihomo-版本号)。现行展示与协议目录的分工见 [Design4 §3.2](../../../Design4.md#32-动态显示与字段优先级) 和 §12.2。

## 构建进度

| Step | 目标 | 状态 |
|---|---|---|
| 1 | 更新节点推荐单选框与回归测试 | 已验收 |
| 2 | 联合门禁、隔离浏览器验证与文档同步 | 已验收 |

## Step 1 — 推荐项展示

- **前置与目标** Issue21 §8 已确认新建和编辑的范围，覆盖 Shadowsocks／VMess／VLESS／Trojan 的推荐字段。保留分组与证据元数据，界面省略全部 `verified` 文案。
- **产出** `frontend/src/components/EditableCombobox.vue` 删除 `verifiedLabel()`。已选值和展开选项共用 `显示名 +（有分组时）「 · 」+ 分组`，例如「TCP · 常用」和「SS 2022 AES-128 · 待验证」。选项的 `aria-label` 由显示名、规范值和分组组成。`frontend/tests/editable-combobox.spec.ts` 覆盖上述两处可见文字、实际 `role="option"` 节点的 `aria-label`、四种分组及无分组选项。
- **验收标准与命令** 先运行 `cd frontend && npm test -- tests/editable-combobox.spec.ts`，确认新断言能复现旧版 `project-unknown` 残留，修复后 8 项通过。选项规范值、自定义草稿、空值选择等既有用例通过。

## Step 2 — 联合门禁与隔离验证

- **前置与目标** Step 1 定向回归通过。用生产构建核对已选值、展开选项、键盘可访问名称、选值回写、检查及协议目录。
- **产出与复现** 在临时副本中将 `frontend/dist` 放入 `backend/web/dist`，以 `CGO_ENABLED=0 go build -o <临时二进制> ./cmd/server` 生成测试服务。使用合成测试库的 SQLite 备份，`APP_MODE=prod`，服务在副本中绑定 `127.0.0.1:18106`。通过隔离 Chrome 登录并打开 `/admin/nodes`，用修复前记录对照选中值、展开选项、DOM 和浏览器无障碍快照。
- **新建与编辑验收** 新建覆盖 4 个协议的 21 个受影响字段，其中 18 个推荐下拉及 3 个 ALPN 推荐列表。选项顺序和分组与基线一致，Mihomo 版本与 `project-unknown` 从选中值、选项 DOM 与无障碍快照消失。编辑覆盖四协议，VLESS 的 TCP 默认值可用键盘改为 WebSocket，保存回写 `network=ws`，重新编辑显示「WebSocket · 常用」。375×785 手机视口的明暗主题均通过，抽屉及所测已选项无横向溢出。
- **API 与内核验收** 19 个协议的目录响应 JSON 与基线一致。VLESS `HK` 的目标检查结果与基线一致，每次请求生成的 `check_id` 单独处理。后端全量包含装配预览、生成与零产出门槛回归。固定 Mihomo v1.19.31 的 17 组顶层测试通过。
- **验收命令** `cd frontend && npm test && npm run build` 通过，51 个测试文件、471 项。`cd backend && go build ./... && go vet ./... && go test ./... -count=1` 通过。在仓库根目录设置 `MIHOMO_11931_BIN` 为固定版本二进制路径后执行 `./.mihomo-test.sh` 通过。`git diff --check` 通过。
- **文档与验证边界** 已同步 [Design4](../../../Design4.md)、[Issue21](../Issue/Issue21.md) 和 [AGENTS](../../../AGENTS.md)。Build32 和 Issue15 保存当时的验收口径。浏览器证据在 `/tmp/r35-06-impl.xBaPZK/`。本次验收为本地自动化和隔离 Chrome，实际读屏软件、手机真机、Docker 镜像与远端 CI 未执行。

## 变更记录

| 日期 | 记录 |
|---|---|
| 2026-11-17 | 完成 Step 1～2，R35-06 通过本地验收。 |
