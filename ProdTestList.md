# ProdTestList.md — 待用户自行执行的 Production 测试清单

> **定位：** 记录当前无法在本环境自动完成的 Production 模式测试项，由用户后续自行在真实/临时 Production 实例中执行。
> 关联：[R16-09](./Issue3.md)、[R17-07](./Issue3.md)、[R19-05](./Issue4.md)。

---

## 一、.smoke-test.sh Production 模式验证

- **背景：** `.smoke-test.sh` 中的 v2 导出/导入必须运行在 Production 模式（`app_mode=prod`），否则会因 Dev 模式 403 导致假绿或直接失败。
- **当前状态：** 已新增 [.smoke-test-prod.sh](./.smoke-test-prod.sh) 可自动拉起临时 Production 容器并执行四类装配器 + v2 导出/导入往返；仍需要用户在有 Docker 的环境中执行。
- **执行方式：**
  ```bash
  bash .smoke-test-prod.sh
  ```
- 也可以继续使用手动方式（见下）。

### 待用户手动执行

1. 准备一个 Production 模式实例（可通过环境变量 `APP_MODE=prod` 启动，或临时部署）。
2. 执行：
   ```bash
   BASE=http://127.0.0.1:18080 .smoke-test.sh
   ```
3. 确认脚本完整跑完并输出：
   ```text
   === SMOKE ALL DONE ===
   ```
4. 重点检查第 16 步：
   - HTTP 必须为 `200`；
   - 导出文件不能是 JSON 错误；
   - 文件大小 > 0。
5. 可选导入回环：
   ```bash
   SMOKE_IMPORT=1 BASE=http://127.0.0.1:18080 .smoke-test.sh
   ```

---

## 二、Production 专项人工核查

- [ ] v2 导出文件可在新库上正常导入；
- [ ] 导入后实例/节点/独立账号/装配蓝图引用正确；
- [ ] 素材池大数据量同步期间其他 API 仍可响应；
- [ ] 装配生成后“去版本管理激活”不再出现 15s `context canceled`；
- [ ] 重启后平台数据不丢失、平台列表正常（对应 R20-11；原环境已不可用，未能复现，需在真实环境人工验证）；
- [ ] OIDC Mock 登录邮箱可通过；
- [ ] 代理组管理在“订阅装配”Tab 内可用；
- [ ] REALITY 新字段可保存并在链接中正确输出。

---

## 三、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-21 | 初始版本：记录 Production 冒烟与人工核查清单，待用户后续自行执行。 |
| v1.1 | 2026-08-22 | 新增 `.smoke-test-prod.sh` 自动拉起临时 Production 容器；R20-11 标记为“原环境不可用，未能复现，转人工验证”。 |
