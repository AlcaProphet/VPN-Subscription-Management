# Build36 — 节点对象字段高级 JSON 开关

基线为 `beta` 的 `7abfd48`。设计见 [Design4 §3.3](../../../Design4.md#33-独立开关区)，问题、研究与已确认方向见 [Issue21 R35-05](../Issue/Issue21.md#r35-05-节点管理的高级-json-切换样式与订阅装配不一致)。

| Step | 目标 | 状态 |
|---|---|---|
| 1 | 将对象字段双按钮替换为固定「高级 JSON」开关并更新测试契约 | 已验收 |
| 2 | 联合门禁、隔离浏览器验证与文档同步 | 已验收 |

## Step 1 — 对象字段编辑模式开关

- **前置与目标** 用户已确认只替换外观：保留「应用」「放弃」、草稿提示、父子草稿阻断、保存／检查拦截和无效 JSON 退出限制；不采用订阅装配的即时写入语义。范围为对象、嵌套对象、map 与 list。
- **产出** `frontend/src/components/ProtocolFieldEditor.vue` 的对象工具栏由两个 `Button` 改为受控 `Switch` 加固定 `<span>高级 JSON</span>`。开关保留 `data-field-json-toggle`，文本域保留 `data-field-json-editor`；`advanced`、`jsonText`、`jsonError`、`jsonDirty`、`applyJSON()`、`discardJSON()`、`setAdvanced()` 的 guard 与事件流未改。无效 JSON 时 `jsonError` 使开关禁用，等价于原「结构化编辑」按钮的 disabled 行为；正常状态可切回结构化并丢弃未应用草稿。
- **无障碍与布局** 每个开关的 `aria-label` 为 `<对象 label> 高级 JSON`，嵌套、map、list 的同名对象可通过对象名区分；开关不进入运行参数，不写入 `protocol_json`。桌面保持与装配一致的紧凑尺寸，手机端外层标签最小 44px，现有 `.form-overlay-drawer` 规则同时把开关本体提升到 44×44px。另以 scoped 样式覆盖 Ant Design `.ant-form label` 的 14px 固定字号，使文案与装配分区一致为 12px。
- **测试** `frontend/tests/protocol-field-editor.spec.ts` 改写按钮契约并新增开关状态、固定文案、对象／嵌套对象／map／list 可区分名称、无效 JSON 禁用与修正后退出用例；`nodes-view.spec.ts` 统一通过 `data-field-json-toggle` 触发并更新定位断言；`node-validation.spec.ts` 的定位 fixture 改为 `role="switch"`。定向 4 个文件 182 项通过；前端全量 51 个文件 471 项通过；`npm run build` 通过。
- **验收命令** `cd frontend && npm run test -- tests/protocol-field-editor.spec.ts tests/nodes-view.spec.ts tests/node-validation.spec.ts tests/header-step.spec.ts`、`npm run test`、`npm run build`。

## Step 2 — 联合门禁、隔离浏览器与文档

- **前置** Step 1 的代码、测试和构建通过。未改后端、API、数据库、持久化格式、依赖或锁文件。
- **命令** 后端 `go build ./...`、`go vet ./...`、`go test ./... -count=1` 通过；前端全量测试与构建通过；`git diff --check` 通过。
- **隔离环境** 使用 CGO 关闭的本地生产构建，监听地址在 `/tmp` 副本中限制为 `127.0.0.1:18098`；数据来自既有合成隔离库 `/tmp/r35-05/data` 的副本，未触碰仓库 `backend/data`、本机 8080、Docker、生产或外部服务。浏览器为 Google Chrome 157.0.8103.79，驱动为 Playwright-core 1.57.0。
- **新建与编辑** 在 1440×1000 浅色下新建 `R35-05-IMPL`：进入「多路复用」高级 JSON，验证固定文案、`role="switch"`、`aria-checked` 与「JSON 草稿未应用」；保存被拦截，目标检查显示「存在未应用的 JSON 草稿，请先应用或放弃后再检查」且「检查当前节点」禁用；点击「定位草稿」聚焦到对应文本域；应用后保存成功，重新打开可见持久化值。编辑 `HK` 时将 `smux.max-connections` 改为 9，应用并保存后重新打开仍为 9。
- **对象形态与阻断** HTTP `headers` map、WireGuard `peers` list、REALITY／SMux／Brutal 嵌套对象均显示各自的开关；`HK` 实际出现「REALITY 高级 JSON」「多路复用 高级 JSON」「Brutal 参数 高级 JSON」三个可区分名称。子对象存在未应用草稿时点击父对象开关会提示「存在未应用的后代 JSON 草稿，请先应用或放弃后再切换高级 JSON」，父对象保持结构化并聚焦子对象文本域；应用子草稿后父对象可进入。
- **有效／无效 JSON 与键盘** 有效 JSON 可应用、放弃、切回结构化；输入 `{` 后开关 `aria-checked="true"` 且 disabled，错误显示「请输入 JSON 对象」，点击可见文案不会退出；修正为合法对象后开关恢复可用并可切回结构化。键盘 `Enter` 和 `Space` 均可切换，Tab 只经过一个开关停靠点。
- **双视口与主题** 375×785 手机视口下外层标签高度 44px、开关本体 44×44px，页面与抽屉均无横向溢出；桌面开关 28×16px、标签 95.66×16px。浅色关闭轨道 `rgba(0,0,0,0.25)`、开启 `rgb(37,99,235)`；深色关闭 `rgba(255,255,255,0.25)`、开启 `rgb(147,197,253)`；两主题下文案均为 12px 且可读。截图与日志位于 `/tmp/r35-05-impl.PfGj5a/`，未加入仓库。
- **文档** 已同步 [Design4 §3.3](../../../Design4.md#33-独立开关区)、[Issue21 R35-05](../Issue/Issue21.md#r35-05-节点管理的高级-json-切换样式与订阅装配不一致) 与 AGENTS 索引；已归档的 `Design2-UI`／`Issue13`／`Build21` 保持原样。
- **验证边界** 未使用真实读屏软件或真机；49 个对象字段中实际交互验证了 REALITY、SMUX、Brutal、HTTP headers 与 WireGuard peers，另确认 WireGuard `ip-stack` 开关存在，其余字段依据同一递归组件和 schema 核对；未覆盖订阅装配 SR／generic 目标、Docker 镜像、远端 CI 或外部客户端。订阅装配的即时写入语义未改动。

## 变更记录

| 日期 | 记录 |
|---|---|
| 2026-11-17 | 完成 Step 1～2 并本地验收，R35-05 通过自动化与隔离浏览器验证。 |
