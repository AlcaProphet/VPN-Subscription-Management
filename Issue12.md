# Issue12.md — VPN 订阅管理系统问题追踪（当前）

> **文档定位：** 本文记录当前发现的 UI 问题与修复闭环，承接已归档的 [Issue1～Issue9](docs/reports/Issue/) 以及当前的 [Issue10.md](Issue10.md)、[Issue11.md](Issue11.md)。设计结论见 [Design3.md](Design3.md) §8.4；编码约束以 [AGENTS.md](AGENTS.md) 为准。

---

## 一、问题记录与修复状态

### R26-01 Clash YAML 头部表单字段缺失、层级扁平且默认值偏离个人配置

- **来源：** 装配页 Clash - V2Ray/Mihomo（新版）头部表单的浏览器评论与用户确认的 UI 调整范围。
- **现象：** 页面仅平铺 `port`、`socks-port`、`mode`、`log-level` 和 `allow-lan`。`redir-port`、`tproxy-port`、完整 Geo 数据、DNS、NTP 及其开关没有结构化入口；当前默认值还写入个人模板之外的 `external-controller`，并把日志级别预填为 `info` 而非个人配置中的 `warning`。长配置需要切换到整个头部 JSON，难以安全维护嵌套数组和对象。
- **根因：** `HeaderStep.vue` 的 Clash 字段 schema 仅覆盖早期最小头部集合，且全局“高级 JSON”模型不能对复杂子配置提供清晰分区；默认值没有与 `Clash.yaml.template.md` 的头部同步。
- **影响范围：** 影响 Clash YAML 装配的编辑效率、默认产物语义和开关可发现性；不影响后端的 `fixed_params`、生成 API、蓝图快照、节点/代理组/规则步骤和既有历史版本。
- **修复方案：**
  1. 头部改为端口配置、Geo 数据、DNS 配置、更多参数四个默认折叠分区；端口/Geo/DNS/更多全部支持结构化编辑和分区级高级 JSON。
  2. 默认头部改为个人模板中的端口、Geo、NTP、DNS 和其他顶层参数；`mixed-port` 保持端口分区可选字段，不默认写入。
  3. 每个分区内的 bool 统一置于本分区“开关参数”区；DNS 的数组改为可增删结构化行，更多参数保留未知顶层键以兼容旧蓝图和扩展字段。
  4. 维持现有 `fixed_params_text`、预览过期和“使用默认值”确认语义，不新增后端接口或数据库迁移；设计结论同步至 Design3 §8.4。
- **验收范围：** 分区默认折叠、模板默认值、分区开关位置、分区 JSON、未知顶层键保留、SR 头部行为回归；前端 Vitest、`npm run build`、`git diff --check`，以及本地页面桌面/移动断点布局核验。
- **状态：** ✅ 已修复（2026-09-02：`npm test` 37 文件 / 141 用例通过，`npm run build` 通过，`git diff --check` 通过；分区默认折叠、结构化/JSON 切换、模板默认值与响应式网格由前端回归和生产构建覆盖）。

---

## 二、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-09-02 | 新建当前问题记录，登记 R26-01 Clash YAML 头部表单分区、默认值和结构化/JSON 编辑修复。 |
