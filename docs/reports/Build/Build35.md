# Build35 — 手动 VLESS／VMess 的 Clash UDP 缺省输出

基线为 `beta` 的 `857f446`。设计见 [Design4 §7.3](../Design/Design4.md#73-历史版本与-xray-来源)，问题及复现见 [Issue21 R35-04](../Issue/Issue21.md#r35-04-vless-节点开启-udp-后生成的-clash-配置缺少-udp-字段)。

| Step | 目标 | 状态 |
|---|---|---|
| 1 | 在两种手动协议的 Clash 适配器补齐 UDP 缺省值 | 已验收 |
| 2 | 验证生产模式生成链路、固定内核与本地流量，同步文档 | 已验收 |

## Step 1 — 适配器与历史版本边界

- **前置与目标** 用户确认缺省按表单的开启语义处理，范围为手动 VLESS 和 VMess。显式关闭、历史版本正文与冻结的手动计划，以及动态 Xray 下载均沿用既有语义。
- **产出** `backend/internal/assembly/clash_protocols.go` 的两个协议适配器沿用显式字段复制，并复用同一缺省补齐函数。参考逻辑为 `活动参数中缺少 udp 键 → 在输出 map 写入 udp: true`。显式 `true` 随字段复制，显式 `false` 省略并由 Mihomo 解析为关闭。活动参数仍保留原值，装配覆盖层在节点输出后应用。
- **回归测试** `clash_udp_test.go` 对两种协议分别经过节点检查、预览、生成和新计划重渲染验证三个 UDP 状态，同时验证历史手动条目及两种 Xray 动态条目保留缺省。修复前两种协议只在缺省组失败，修复后全部通过。`links_test.go` 将新增输出的 `udp` 纳入已有稳定键序断言。
- **验收命令** 在 `backend/` 执行 `go test ./internal/assembly -run '^TestClash(ManualVLESSAndVMessUDPDefault|UDPDefaultDoesNotApplyToHistoricalPlanOrXray|ProxyKeyOrder)$'`。同一范围的 `go test -race` 验证 UDP 默认与边界用例。

## Step 2 — 联合门禁和本地复测

- **前置与产出** Step 1 已通过。使用临时目录、只读复制的本地测试数据库和回环监听的 Production 实例，嵌入当次前端生产构建。文档同步至 Design4 §7.3、Issue21 R35-04 与 AGENTS 构建索引。
- **验收命令** 后端通过 `go build ./...`、`go vet ./...`、`go test ./... -count=1`、UDP 定向 race 和 `go run ./cmd/errgate ./...`。固定内核通过 `MIHOMO_11931_BIN=<Mihomo v1.19.31 二进制> ./.mihomo-test.sh` 的 17 组顶层测试。前端 `npm run test` 通过 51 个文件、469 项，`npm run build` 通过并报出既有的大分块提示。`git diff --check` 通过。
- **浏览器与版本文件** 用 Chrome 验证 VLESS 和 VMess 的缺省、显式开启与显式关闭，共生成六份实际版本文件。每组装配预览与版本正文逐字一致，库中分别仍是缺省、`true` 和 `false`，无显式传输字段时保留 Mihomo 的默认 TCP 行为。另以未携带 UDP 参数的 VLESS URI 验证导入后的节点仍缺少存储值，Clash 预览输出 `udp: true`。
- **固定内核与流量** 六份原始 YAML 均通过 Mihomo v1.19.31 的 `-t`。提取各版本的代理条目，通过回环 SOCKS5 UDP 入口将报文经本机 VLESS／VMess 入站转发到本机 UDP 回显服务。缺省和显式开启组均由对应节点成功转发，显式关闭组由内核跳过，测试使用 `REJECT` 兜底以排除直连。配置检查与流量试验均使用合成节点和回环地址。
- **验证范围** 本地宿主构建、隔离 Production、固定内核和回环连接。仓库本地 8080 实例、Docker 镜像、远端 CI 与外部代理服务未纳入验收。

## 变更记录

| 日期 | 记录 |
|---|---|
| 2026-11-16 | 完成 Step 1～2 并归档，R35-04 通过本地验收。 |
