# ProdTestList.md — 当前人工核验清单

> 本文件只记录仍需用户亲自执行或修复后重新执行的 Production、浏览器和真实客户端人工核验。已完成且未发现问题的项目从清单移除；工程问题和自动化证据写入对应的 Issue/Build 文档。

## 当前人工核验项目

### Build32 人工核验执行前置

> **正式人工验收尚未开始。** 当前 Build32 Step 21 仍有两项工程合同需要先收口：①协议切换提示必须按 endpoint policy 说明，不再统一声称保留 server/port；②切换协议、selector、endpoint 模式及应用 OpenVPN 解析结果前，必须统一检查未应用的自定义值／列表项／JSON／parser 草稿，并只展示将清除的字段名称和凭据数量，不展示凭据值。上述修复、失败优先测试及 Step 21 三条前端门禁写入 Build32 后，才开始下列正式人工核验，避免在仍会变化的前端产物上重复验收。
>
> 每轮必须使用最新 `frontend/dist` 与全新隔离 `DATA_DIR`／测试账号，先记录 HEAD、构建时间、浏览器版本、视口和主题，禁止复用无法确认版本的 `backend/web/dist`。发现问题时停止当前项目及其后续依赖项目，保存不含凭据的最小复现步骤，并登记到当前 Issue；修复后从失败项目重新开始。自动化通过、固定 Mihomo `-t`、浏览器可见结果和真实连接分别记录，不互相替代。

### 第一阶段：Build32 Step 21 全协议前端回归（按编号顺序执行）

- [ ] **PT-B32-21-01 切换预检、清理摘要与 endpoint policy：** 在 920px 浅色主题先建立基线，分别覆盖普通 endpoint → Tailscale 无 endpoint、WireGuard `single → peers`、Hysteria2 `single → ports`、Mieru `single → range`、协议切换及 OpenVPN 解析结果应用。每条路径都先制造未应用的自定义值、列表项、局部 JSON 或 parser 草稿，并准备至少一个已保存或待替换凭据；确认切换前出现统一预检，摘要只列将清除的字段名称和凭据数量、不显示任何值，取消后草稿与凭据状态完整保留，确认后只清除命中范围且 A→B→A 不恢复。协议切换说明必须按目标 endpoint policy 区分保留、隐藏或清空 host/port，不得再统一声称保留 server/port。
- [ ] **PT-B32-21-02 19 协议响应式与主题快速矩阵：** 对 `ss`、`vmess`、`vless`、`trojan`、`http`、`socks5`、`ssh`、`snell`、`hysteria`、`hysteria2`、`tuic`、`wireguard`、`mieru`、`masque`、`tailscale`、`anytls`、`shadowquic`、`trusttunnel`、`openvpn` 逐一检查 `920 × 785` 与 `375 × 785`、浅色与深色四种组合。每个组合至少确认协议可选、字段由 schema 正常渲染、当前组合与 endpoint 显隐正确、独立开关／高级折叠可操作、无横向溢出、无重复或缺失控件；375px 必须使用 Drawer 单列，920px 使用 Modal 并允许双列。此项是界面快速矩阵，不代替后续分支生命周期和真实连接测试。
- [ ] **PT-B32-21-03 字段类型、长内容、列表与错误定位：** 依次核对 `text/password/number/bool/select/object/text-list/int-list/multiline/secret-multiline/byte-sequence` 的单一 schema 渲染；确认 number 区分未设置与 0、三态 bool 区分 unset/false/true、`state_only` 只进入 `current_state.selectors`、隐藏字段同步离开提交草稿和 credential ops。使用不含生产秘密的长 CA／证书／SSH 或 OpenVPN 私钥文本检查换行、滚动和脱敏；复核 WireGuard Peer 新增／删除／上移／下移、首末禁用、稳定身份和 PSK 状态。主动制造折叠区字段错误、Peer 条目错误及 OpenVPN 行级 diagnostic，确认页面能展开、滚动并聚焦到精确位置。
- [ ] **PT-B32-21-04 保存、重开、检查与清空闭环：** 在 PT-B32-21-01～03 通过后，选取每个协议至少一个合法分支执行创建或编辑、检查目标、保存、读取和重开；对具有 selector/feature 的协议再执行一次分支切换与清空。确认当前组合、endpoint、列表顺序、configured／keep／replace／clear 状态及诊断在重开后符合合同，列表／详情／检查预览／错误中均无凭据明文。任何协议只能完成渲染但无法保存、重开或定位错误时，本项不得通过。

### 第二阶段：TrustTunnel、OpenVPN 与 `.ovpn` 深度人工核验

> 工程实现与自动化证据见 [Build32.md](Build32.md) Step 17～19。以下项目仅在隔离测试环境执行；使用测试凭据与测试节点，截图、日志和验收记录不得包含密码、私钥、静态 key 或完整 `.ovpn` 原文。

- [ ] **PT-B32-17-01 TrustTunnel 浏览器字段生命周期：** 分别在桌面宽度与 375px、明暗主题下验证 `none/connections/streams` 复用分支、HTTP/2／QUIC 分支、username/password、ECH 与证书/私钥成对规则；确认切换分支会立即隐藏并清除已失活字段，A→B→A 不恢复旧值，保存、重开与“检查目标”保持一致，密码和私钥只显示 configured／keep／replace／clear 状态且不出现在预览、响应或页面错误中。
- [ ] **PT-B32-18-01 OpenVPN 结构化编辑与分支清空：** 在真实浏览器依次验证 `userpass`、`cert`、`cert_userpass` 三种认证模式和 `none/tls-auth/tls-crypt/tls-crypt-v2` 四种 TLS key 模式；确认模式切换只清除新分支不再活动的凭据、仍活动的凭据保持、A→B→A 不恢复旧值，并复核 DNS 开关、列表去重、`tran-window` 未设置/显式 0、保存、重开和“检查目标”。密码、客户端私钥及 TLS key 不得在列表、详情、检查预览、日志或错误中明文出现。
- [ ] **PT-B32-19-01 真实 `.ovpn` 解析、应用与保存：** 使用不含生产凭据的真实或脱敏测试 `.ovpn`，覆盖 inline CA、cert/key、`auth-user-pass` 以及测试文件实际采用的 TLS key；确认字段来源行号和 diagnostics 可定位、敏感结构只显示“已解析”、组合认证推导正确。应用后只覆盖 parser 明确产出的字段，既有未产出草稿保持；保存、重开、检查与最终装配成功，原文、行号和 diagnostics 不进入节点数据、localStorage、响应或日志。
- [ ] **PT-B32-19-02 `.ovpn` API、安全边界与草稿阻断：** 使用浏览器开发者工具或隔离 API 客户端核对匿名 401、普通用户 403、成功 200、危险指令/外部文件引用/冲突配置 400、超限 413 均带 `Cache-Control: no-store`；确认未应用原文会阻断保存与检查，应用、取消、关闭面板或切换协议后解除阻断并丢弃原文。另在桌面宽度与 375px、明暗主题下验证长证书/私钥、逐行诊断和错误定位无溢出或凭据回显。

### 第三阶段：最终候选产物的真实服务／客户端连接

> 仅在第一、第二阶段通过，且 Build32 Step 22 联合门禁与隔离 smoke 已形成最终候选产物后执行，避免代码继续变化导致真实连接结果失效。若暂时没有可控服务端，保留未执行状态并记录环境缺口，不以固定内核、Docker、API 或浏览器结果代替。

- [ ] **PT-B32-17-02 TrustTunnel 真实连接：** 使用 Mihomo v1.19.31 与可控 TrustTunnel 测试服务，至少分别验证一个 HTTP/2 配置和一个 QUIC 配置可导入并建立真实连接；覆盖实际使用的复用模式，核对生成配置不含 `reuse_mode` 等 state-only 字段，关闭 QUIC 后不残留拥塞参数。若测试服务不支持某一分支，记录未覆盖分支，不得以固定内核 `-t` 结果代替连接成功。
- [ ] **PT-B32-18-02 OpenVPN 真实客户端连接：** 使用真实测试 OpenVPN 服务与 Mihomo v1.19.31，对测试服务实际支持的认证模式完成配置生成、导入和连接；至少覆盖一条用户名密码路径及一条证书路径，服务支持组合认证时再覆盖 `cert_userpass`。确认实际连接可用、最终配置不含 `client-config`、selector、导入行号或 `.ovpn` 原文；未具备服务端条件的模式明确记录为未执行。

Xray 相关人工核验继续由 [XrayRelated1.md](XrayRelated1.md) 独立跟踪；当前聚焦基础模式，高级模式暂不跟进。

## 变更记录

| 日期 | 说明 |
|---|---|
| 2026-09-27 | 建立 Build32 后续人工测试顺序：先等待 Step 21 的切换预检与 endpoint policy 提示两项工程合同收口，再按 PT-B32-21-01～04 完成全协议前端回归；随后执行 TrustTunnel／OpenVPN／`.ovpn` 深度浏览器与 API 核验，最后在 Step 22 最终候选产物上执行真实服务连接。补充最新产物、隔离数据、失败即停、证据分层和凭据保护要求。 |
| 2026-09-23 | Build32 Step 17～19 工程与自动化门禁完成后，新增 PT-B32-17-01～PT-B32-19-02，跟踪 TrustTunnel/OpenVPN 浏览器字段生命周期、真实 Mihomo/服务端连接、真实 `.ovpn` 解析应用、API 安全边界及响应式人工核验；不把尚未执行的真实连接表述为固定内核证据。 |
| 2026-09-20 | 用户确认 R32-02/R32-03 业务邮件异步派发、历史结果与当前发送队列已完成人工真机核验，暂未发现问题；依清单规则移除全部当前人工核验项目。此前历史记录已压缩。 |
