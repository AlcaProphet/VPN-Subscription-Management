# TODOLIST.md — 短期待办跟踪（2026-11-13）

> 本文件跟踪当前待办与待确认范围。已完成工作的详细证据见对应 Design／Build／Issue。

## 当前基线

- **开发与验收** 基础模式既定开发范围已完成。Build32 Step 0.5～22 已验收并归档，Issue19 R33-01～09 与 Issue20 R34-01～04 均已关闭并归档。[Design4.md](docs/reports/Design/Design4.md) · [Build32.md](docs/reports/Build/Build32.md) · [Issue19.md](docs/reports/Issue/Issue19.md) · [Issue20.md](docs/reports/Issue/Issue20.md)
- **已发布镜像** `v4.1.4` 指向 `e401d57`，包含 R33-09 和 R34-02～04。远端全依赖 high 审计、镜像构建和推送均已通过。GHCR 的 `v4.1.4`、`latest` 和 `sha-e401d57` 共同指向 `sha256:8f87e208ee90dda990bf92fa5fde61a1f0bc0aa785d840c700827775bc7c9ff6`。[发布复核](docs/reports/Issue/Issue20.md#二v414-发布复核) · [GitHub Actions #54](https://github.com/AlcaProphet/VPN-Subscription-Management/actions/runs/43255471928)

## 当前待办

### 待确认与文档维护

- [ ] 确认局部 JSON 删除列表键、WireGuard／MASQUE IP 文本清空的预期，再决定是否实施。显式清空协议文本列表的既定修复已由 R33-06 验收。[Issue19.md](docs/reports/Issue/Issue19.md)

## 独立跟踪与后续范围

### P0 配置导出完整性（独立专项）

- [ ] 以 R31-01 为入口，确认 `signing_key` 编码、旧库读取兼容和已损坏导出文件的处理方案。方案确认后，以真实随机密钥在隔离环境核验 `Export → Import → 解密`、不同密文类型及失败路径。[ExportRelated1.md](ExportRelated1.md)

### 其他独立跟踪与后续范围

- [ ] 跟踪 `@vue/test-utils → js-beautify` 开发依赖链的 4 项 moderate，待上游兼容版本发布后重查。[Issue20.md R34-02](docs/reports/Issue/Issue20.md#r34-02-follow-redirects-新公告阻断候选发布)
- Xray R26-07 人工核验与独立 Xray outbound 见 Xray 专项。整站加密迁移候选及现有配置导入后续复验见导出专项。[XrayRelated1.md](XrayRelated1.md) · [ExportRelated1.md](ExportRelated1.md)
- 通用客户端兼容候选保留 SS 2022 完整密钥／URI／客户端兼容、手动 VMess REALITY 及 AmneziaWG，待分别确认是否启动。[Design4 §8.4](docs/reports/Design/Design4.md#84-ss-2022-与插件编码的设计含义) · [Design4 §12.4 VMess](docs/reports/Design/Design4.md#vmess) · [Build32 §3.3](docs/reports/Build/Build32.md#33-已确认边界amneziawg-不纳入)
- 目标检查结果的逐条诊断定位保留为独立交互候选。保存校验的字段定位已由 R33-07 验收。[Design4 §3.3](docs/reports/Design/Design4.md#33-独立开关区) · [Issue19.md](docs/reports/Issue/Issue19.md)
- 数据库外历史兼容清理待另行确认启动。第三期安全审查由独立计划跟踪。[Build31.md](docs/reports/Build/Build31.md) · [SecurityScanPlan1.md](SecurityScanPlan1.md)

## 变更记录

- 2026-11-24 归档 Design4 和 Design5。节点编辑器的通用候选由本清单保留，Xray 和整站迁移后续工作见对应专项。
- 2026-11-24 将 2026-09-17 用户确认通过的 PT-B28-01～PT-B28-06 同步至 [Design5 §7.6](docs/reports/Design/Design5.md#76-实施状态2026-09-15)，并修复与 [Issue18.md](docs/reports/Issue/Issue18.md) 的归档链接。
- 2026-11-13 完成 v4.1.4 发布复核并归档 Issue19／Issue20，移除已完成的发布待办，将配置导出 P0 和中危开发依赖移入独立跟踪。
