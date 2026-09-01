# Build15.md — R24-19/R24-20 节点表单结构化修复构建记录

> **文档定位：** 本文记录 `Issue9.md` R24-19、R24-20 经用户确认后的实施与验收结果。
> - 当前设计：[Design2.md](../Design/Design2.md)、[Design2-UI.md](../Design/Design2-UI.md)
> - 编码指令：[AGENTS.md](../../../AGENTS.md)（**唯一强要求**）
> - 问题追踪：[Issue9.md](../../../Issue9.md)
> - 前一轮构建：[Build14.md](Build14.md)
> - 实施日期：2026-08-30

---

## 一、构建进度追踪

| Step | 内容 | 设计依据 | 状态 |
|------|------|---------|------|
| 1 | R24-19：协议字段分区与节点弹窗重排 | Design2-UI §6.2 | ✅ 验收通过 |
| 2 | R24-20：递归对象 schema 与结构化编辑器 | Design2-UI §6.2 | ✅ 验收通过 |
| 3 | 服务端递归校验与兼容测试 | Design2-UI §6.2 | ✅ 验收通过 |
| 4 | 全量验证与文档同步 | AGENTS.md §3.4/§3.6 | ✅ 验收通过 |

> 状态标记：☐ 未开始 / ◧ 进行中 / ✅ 验收通过。

---

## 二、构建概要

| Step | 涉及文件 | 要点 |
|------|---------|------|
| 1 | `backend/internal/node/registry.go`、`frontend/src/api/node.ts`、`frontend/src/views/admin/NodesView.vue` | `FieldSchema.section`、六区布局、独立开关区、920px Modal / 移动端 Drawer |
| 2 | `backend/internal/node/registry.go`、`frontend/src/components/ProtocolFieldEditor.vue` | `fields/map/list` 三种对象形态、递归属性、未知键保留、对象级高级 JSON |
| 3 | `backend/internal/node/node.go`、前后端测试 | 嵌套类型路径校验、smux 对象口径、嵌套敏感字段与无效 JSON 保存拦截 |
| 4 | `Design2-UI.md`、`Issue9.md`、`AGENTS.md`、节点参数参考 | 设计、构建、问题状态与参考口径同步 |

依赖顺序：

```text
FieldSchema 增量契约 → 节点表单分区 → 结构化对象编辑 → 递归服务端校验 → 全量验证与文档同步
```

本构建不新增数据库迁移，不改变 `protocol_json` 的字段名、JSON 结构或敏感字段加密存储方式。

---

## 三、分步实施与验收

### Step 1：R24-19 字段分区与弹窗重排

- **目标：** 消除协议字段、开关和高级参数连续混排，建立跨 19 个协议一致的信息架构。
- **前置条件：** 保持协议注册表为动态表单唯一字段来源，不在前端复制协议字段表。
- **产出：**
  - `FieldSchema` 增加 `section=auth/transport/security/switches/advanced`。
  - 节点弹窗改为基本信息、认证与密钥、协议与传输、安全与证书、开关参数、高级参数六区。
  - 顶层 bool 字段统一进入独立开关区；高级参数默认折叠；协议切换清空当前协议参数。
- **参考结构：**
  ```text
  basic（固定字段）
  ├─ auth
  ├─ transport
  ├─ security
  ├─ switches（全部顶层 bool）
  └─ advanced（默认折叠）
  ```
- **验收标准：** 注册表全部字段有合法分区；bool 不进入普通输入区；桌面双列、移动端单列及全屏 Drawer 保持可操作。

### Step 2：R24-20 递归对象 schema 与编辑器

- **目标：** 让管理员无需直接编写 JSON 即可编辑常用复杂协议参数，同时保持未来字段兼容。
- **前置条件：** Step 1 的增量 schema 契约可被前后端共同读取。
- **产出：**
  - `object_kind=fields`：REALITY、传输对象、插件、内层 SS、ECH、Sing-Mux 等固定属性对象。
  - `object_kind=map`：headers / ws-headers 等任意键值映射。
  - `object_kind=list`：WireGuard peers 对象数组。
  - `ProtocolFieldEditor.vue` 递归渲染子字段，更新时基于原对象合并以保留未知键。
  - 每个对象保留独立高级 JSON Switch；未知键提示数量，可在高级入口编辑。
- **验收标准：** 固定对象、映射和数组均可结构化编辑；复杂/未知数据不静默丢失；无效 JSON 明确报错并阻止保存。

### Step 3：服务端递归校验与安全兼容

- **目标：** 前端结构化控件不能成为唯一校验边界，嵌套类型和敏感字段仍由服务端兜底。
- **产出：**
  - 对象按 `fields/map/list` 形态校验；固定对象和数组项递归验证已知子字段，错误返回完整路径。
  - `allow_unknown=true` 的扩展键保持兼容。
  - `plugin-opts.password`、`ss-opts.password` 继续走点路径加密、脱敏和编辑留空保留旧密文。
  - `smux` 统一为嵌套对象，并覆盖 `brutal-opts` 子对象。
- **验收标准：** 错误类型能定位 `peers[0].port` 等完整路径；未知扩展键合法；smux boolean 被拒绝而对象通过；嵌套凭据测试继续通过。

### Step 4：全量验证与文档同步

- **验证命令：**
  ```bash
  cd backend && go build ./...
  cd backend && go vet ./...
  cd backend && go test -timeout 180s ./...
  cd frontend && npm test -- --run
  cd frontend && npm run build
  ```
- **验收结果：**
  - 后端 build、vet、全量 test 全部通过。
  - 前端 35 个测试文件、126 项测试全部通过。
  - 前端 TypeScript 检查与生产构建通过。
  - `git diff --check` 通过。

---

## 四、文档同步

- `Design2-UI.md` §6.2：更新弹窗尺寸、六分区布局、对象 schema、结构化编辑、高级 JSON、未知键与递归校验契约。
- `Issue9.md`：R24-19/R24-20 补充实际修复内容、验证结果并标记完成。
- `docs/Reference/Clash-Verge-Rev-Node-Parameters.md`：修正 VLESS/VMess/SS 字段表中 `smux` 的 boolean 摘要错误，统一指向 3.19 嵌套对象。
- `AGENTS.md`：登记 Build15 并更新 Issue9 状态摘要。

---

## 五、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-30 | 实施并验收 R24-19/R24-20：节点表单分区、独立开关区、递归对象 schema、结构化编辑器、服务端递归校验、兼容测试与文档同步。 |
