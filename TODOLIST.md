# TODOLIST.md — 短期待办跟踪（2026-11-12）

> 本文件跟踪当前待办与待确认范围。已完成工作的详细证据见对应 Design／Build／Issue。

## 当前基线

- **开发与本地验收** `beta` 与远端同为 `1ceea52`。基础模式的既定开发范围已验收，Build32 Step 0.5～22 已归档，Issue19 R33-01～09 已关闭，Issue20 R34-01～04 的本地候选验收已完成。[Design4.md](Design4.md) · [Build32.md](docs/reports/Build/Build32.md) · [Issue19.md](Issue19.md) · [Issue20.md](Issue20.md)
- **已发布镜像** GHCR 的 `v4.1.3` 和 `latest` 均指向 `45cb62a` 对应的镜像，索引摘要为 `sha256:3862b98b1b0db8f10dcc5c8c63d61270528aa04191be30b8c52689b13bc11a61`。该标签通过了当时的发布工作流，当前 `beta` 的修复仍待发布。[GitHub Actions #39727958557](https://github.com/AlcaProphet/VPN-Subscription-Management/actions/runs/39727958557)

## 当前待办

### P0 配置导出完整性（独立专项）

- [ ] 以 R31-01 为入口，确认 `signing_key` 编码、旧库读取兼容和已损坏导出文件的处理方案。方案确认后，以真实随机密钥在隔离环境核验 `Export → Import → 解密`、不同密文类型及失败路径。[ExportRelated1.md](ExportRelated1.md)

### 发布复核

- [ ] 下次发布核对实际标签提交是否包含 R33-09 和 R34-02～04，并核验远端全依赖 high 审计、镜像构建和推送结果，记录 GHCR 摘要。当前审计复查为 `beta` 0 high／4 moderate，`v4.1.3` 锁文件仍报 `follow-redirects` 1 high／4 moderate。[Issue20.md](Issue20.md)
- [ ] 跟踪 `@vue/test-utils → js-beautify` 开发依赖链的 4 项 moderate，待上游兼容版本发布后重查。[Issue20.md R34-02](Issue20.md#r34-02-follow-redirects-新公告阻断候选发布)

### 待确认与文档维护

- [ ] 确认局部 JSON 删除列表键、WireGuard／MASQUE IP 文本清空的预期，再决定是否实施。显式清空协议文本列表的既定修复已由 R33-06 验收。[Issue19.md](Issue19.md)
- [ ] 同步 Design5 §7.6 的邮件人工验收状态，修正 Design5 与归档 Issue18 的失效相对链接。PT-B28-01～06 已于 2026-09-17 获用户确认通过。[Design5.md](Design5.md#76-实施状态2026-09-15) · [Issue18.md](docs/reports/Issue/Issue18.md)

## 独立跟踪与后续范围

- Xray R26-07 隔离 API／UI 人工核验及第三期安全审查 Step 4 起的工作，分别由对应专项跟踪。[XrayRelated1.md](XrayRelated1.md) · [SecurityScanPlan1.md](SecurityScanPlan1.md)
- SS 2022 完整密钥／URI／客户端兼容和独立 Xray outbound 属于 Design4 后续专项。AmneziaWG、整站迁移、数据库外历史兼容清理属于候选范围，待另行确认启动。[Design4.md](Design4.md) · [Build32.md](docs/reports/Build/Build32.md) · [Design5.md](Design5.md) · [Build31.md](docs/reports/Build/Build31.md)

## 变更记录

- 2026-11-12 核对 `beta`、已发布镜像和本地验收状态，压缩已完成工作的历史记录，补记待确认范围及文档同步项。
