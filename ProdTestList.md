# ProdTestList.md — 待用户自行执行的 Production 测试清单

> **定位：** 记录当前无法在本环境自动完成的 Production 模式测试项，由用户后续自行在真实/临时 Production 实例中执行。
> 关联：[R16-09](docs/reports/Issue/Issue3.md)、[R17-07](docs/reports/Issue/Issue3.md)、[R19-05](docs/reports/Issue/Issue4.md)。

---

## 一、.smoke-test.sh Production 模式验证

- **背景：** `.smoke-test.sh` 中的 v2 导出/导入必须运行在 Production 模式（`app_mode=prod`），否则会因 Dev 模式 403 导致假绿或直接失败。
- **当前状态：** 已新增 [.smoke-test-prod.sh](.smoke-test-prod.sh) 可自动拉起临时 Production 容器并执行四类装配器 + v2 导出/导入往返；仍需要用户在有 Docker 的环境中执行。
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
- [ ] Clash 下载响应头含 `filename*`/RFC5987 与真实 emoji（无 `\U` 转义）；
- [ ] 覆盖层装配（Merge + Rules/Proxies/Groups Seq）预览、生成、下载内容一致；
- [ ] 节点 URI 批量导入回执正确：合法 2 条 / 非法 1 条，刷新后节点列表可见。

---

## 三、Build11 专项人工核查

- [ ] Production 实例中 `/admin` 概览可访问，服务状态/Checklist/计数/最近待审批/最近访问日志正常；
- [ ] 重置链接四态（valid / missing / used / expired）页面表现正确，已使用/过期链接不渲染密码表单；
- [ ] 非应急状态下 `/emergency` 显示正常提示，不显示操作码输入；
- [ ] 装配预览修改配置后出现“配置已变化，请重新预览”，确认生成被禁用；
- [ ] 设置页六分组可快速定位，未保存提示与离开拦截正确；
- [ ] 手机宽度（≤430px）下复杂表单使用全屏 Drawer，底部操作可见且命中区 ≥44px；
- [ ] 浅色/暗色主题下 AntD 与 Tailwind 颜色一致，无纯黑断层与明显对比度问题。

---

## 四、变更记录

| 版本 | 日期 | 说明 |
|------|------|------|
| v1.0 | 2026-08-21 | 初始版本：记录 Production 冒烟与人工核查清单，待用户后续自行执行。 |
| v1.1 | 2026-08-22 | 新增 `.smoke-test-prod.sh` 自动拉起临时 Production 容器；R20-11 标记为“原环境不可用，未能复现，转人工验证”。 |
| v1.2 | 2026-08-28 | 新增 Build11 专项人工核查：管理员概览、重置四态、应急正常态、装配 stale 预览、设置六分组、手机 Drawer、双主题一致性。 |
