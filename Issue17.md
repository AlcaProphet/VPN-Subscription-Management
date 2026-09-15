# Issue17.md — R30-05 核验后的 OIDC 遗留问题

> **文档定位：** 本文件为核验 Issue16 R30-05 修复后新开的独立问题记录。R30-05 的三个阻塞项已在本轮修复；本文件跟踪未纳入该批次、但在核验中确认的真实 OIDC 网络边界问题与残余边界。原 R31-01 由 [ExportRelated1.md](ExportRelated1.md) 独立跟踪。
> 关联：[Issue16.md](docs/reports/Issue/Issue16.md)、[ExportRelated1.md](ExportRelated1.md)、[AGENTS.md](AGENTS.md)。

---

## 一、R31-02 真实 OIDC discovery/token/JWKS endpoint 未统一 HTTPS 校验（高）

- **现象与根因：** `fetchDiscovery` 直接请求由 `base_url` 拼接的发现文档地址，未调用 `validateOIDCURL`；发现文档返回的 `token_endpoint` 在真实 `Exchange` 中直接使用。`SaveOidc` / Setup 也不强制 `base_url` 为 HTTPS。
- **影响范围：** 即使入口地址为 HTTPS，发现文档仍可声明 `http://` token/JWKS 地址；在本次 Secret 解密修复后，token 请求会携带明文 Client Secret，存在明文传输风险。发现文档中的授权地址也直接交给浏览器，需一并校验。
- **核查补充：** 测试连接路径已对发现文档地址和 token endpoint 调用 `validateOIDCURL`，但真实 `StartFlow` / `Exchange` 共用的 `fetchDiscovery` 未校验，`getJWKS` 也未校验。`CheckRedirect` 只覆盖后续重定向，不覆盖初始请求；当前 `ProxyFromEnvironment` 还需核对代理转发时公网 IP 检查是否仍作用于目标地址。
- **拟议修复方案：** 在真实流程所有初始出站请求前复用统一的 HTTPS/语法校验；解析发现文档后在缓存前校验授权、token、JWKS 地址，且在实际请求处再次守卫；保持每次重定向的 HTTPS 校验。建议 OIDC 专用客户端禁用 `ProxyFromEnvironment`，使现有拨号时 DNS/公网 IP 检查实际作用于目标主机；如部署确需代理，应另行设计可验证的目标地址限制。对 `base_url` 在 Setup/保存时提前拒绝不合规输入。用隔离 mock/本地受控 HTTP 客户端覆盖 HTTP 初始地址、HTTPS 发现文档夹带 HTTP endpoint、重定向降级、缓存命中与代理边界；真实 OIDC 登录仅在隔离环境核验。
- **方案核验补充（2026-09-15）：** `Exchange` 的 token POST 含明文 Client Secret；现有 `CheckRedirect` 只要求跳转目标为 HTTPS，允许 HTTPS 跨主机跳转。即使完成上述 HTTPS 校验，仍可能把 Secret 转发给另一主机。token 请求必须另设不转发 Secret 的重定向边界：禁止重定向，或仅允许经明确校验的同源重定向；不能仅复用通用的 HTTPS 重定向规则。实施前需确定采用哪种策略，并分别测试同源/跨源 HTTPS 跳转、HTTP 降级及目标端未收到 Secret；测试连接的凭据请求也须同口径。
- **已实施修复（2026-09-15，用户确认后按串行顺序实施）：**
  1. 新增 `internal/urlguard.ValidateHTTPS`，统一 OIDC 地址的 HTTPS/语法校验。真实流程在初始 `.well-known` 请求前、discovery 解析后写缓存前校验 authorization/token/JWKS endpoint，并在 `StartFlow` 返回授权 URL 前、`Exchange` 发 token POST 前、`getJWKS` 发请求/命中缓存前再次守卫。
  2. `SaveOidc`、Setup、`SaveParams`/`SaveParamsTx` 对真实提供商非空 `base_url` 提前拒绝 HTTP；`oidcUsable`/`oidcAvailable` 同步要求真实提供商 base_url 为 HTTPS，防止已有 HTTP 配置被用来关闭本地登录形成死锁。
  3. 经用户确认的 token 凭据重定向策略为**禁止任何重定向**：`Exchange` 与测试连接 `client_credentials` 统一走 `doCredentialRequest`，任何 3xx 都在发出下一跳前失败；307/308 不会重放含明文 Client Secret 的 body，HTTPS 降级也不会跟随。同源重定向同样按该策略拒绝。
  4. 导入策略按用户确认采用“仅本地登录关闭时拒绝非 HTTPS base_url”；本地登录开启时仍允许导入，但真实登录/绑定/测试连接会被运行时守卫拦截。该检查由 v1/v2、Setup/管理端导入共同复用。
  5. 代理策略按用户确认维持 [SecurityReport1.md](docs/reports/SecurityReport/SecurityReport1.md) D-F06-2：继续使用 `ProxyFromEnvironment`，经代理时目标 DNS/公网 IP 校验不生效，代理可信作为部署边界；代码注释已明确残余边界，部署层可用 `NO_PROXY` 让 OIDC 目标直连并恢复目标 IP 校验。
- **残余边界：** OIDC Discovery 标准允许 authorization/token/JWKS 与 issuer 不同 HTTPS 源；本轮继续信任配置的 discovery 来源，不增加跨源 endpoint 白名单。若 discovery 自身被恶意控制或跨源跳转到恶意文档，仍可能声明另一 HTTPS token 主机；这不属于“HTTP/重定向泄露 Secret”的修复范围，如需收紧需单独立项。
- **自动化证据（2026-09-15）：** 新增 `internal/oidc/r31_02_test.go`、`internal/urlguard` 单测及 config/server 写入口回归；覆盖 HTTP 初始地址 0 网络请求、HTTPS discovery 夹带 HTTP authorization/token/JWKS 时拒绝且不缓存、合法 discovery 缓存命中、缓存中恶意 endpoint 在 `StartFlow`/`Exchange` 实际使用点被拒绝、token POST HTTPS→HTTP 降级/跨源 307/308/同源 307 均被拒绝且目标端 0 命中/未收到 Secret、测试连接 HTTP endpoint 拒绝、非公网 IP 拨号拒绝，以及 `SaveOidc`/Setup/导入/关闭本地登录锁死保护的入口与状态码。`cd backend && go build ./...`、`go vet ./...`、`go test ./... -count=1`、`go run ./cmd/errgate ./...` 均通过；改动包 `go test -race ./internal/oidc ./internal/config ./internal/server ./internal/urlguard` 通过。
- **真实 IdP 验收边界：** 上述均为隔离 HTTP/TLS 端点自动化证据，不等同于真实 IdP 登录验收。本环境没有真实 IdP，未执行真实授权码/PKCE 登录；该项保持“待隔离环境人工验收”，需在提供隔离 IdP 的 discovery/client/回调信息后单独记录。
- **状态：** ☑ R31-02 代码与自动化隔离验证完成；真实 IdP 登录待隔离环境人工验收。

## 二、R31-03 提供商切换仍可能混合非 Secret 字段（中）

- **现象与根因：** 设置页切换提供商时只清空 Secret，保留源提供商的 `base_url` / `client_id`；保存目标提供商时，后端使用表单字段覆盖目标参数、仅在 Secret 为空时保留目标原 Secret，可能形成“源地址/Client ID + 目标 Secret”的混合配置。
- **影响范围：** 切换后直接保存可能使目标提供商配置不可用；若源地址的发现文档与 token endpoint 可达，后续真实登录还可能把目标提供商的 Client Secret 发送到该端点。现有可用性判定只检查地址、Client ID 与 Secret 是否存在，不能证明三者属于同一配置；本地登录关闭时存在登录锁死风险。保存前的测试连接因字段与目标已存配置不一致，不会回退使用目标旧 Secret；保存后字段已被覆盖，再测试可能回退该 Secret，不能靠测试连接代替字段隔离。
- **已确认设计（2026-09-14）：** 用户选择“切换时加载目标提供商已保存字段”，不采用切换后全部显式清空，也不再跨提供商沿用当前表单的地址/Client ID。此结论明确覆盖归档 [Design1.md](docs/reports/Design/Design1.md) §3.1 中“切换时保留通用字段”的旧描述；实施时须同步现行设计与界面文案。`frontend_url` / `callback_url` 是站点级字段，切换提供商不清空。
- **拟议处理合同：**
  1. 切换下拉选项只改变页面草稿，不立即改变生效提供商。确认切换后读取目标提供商自己的 `base_url` / `realm` / `client_id`；目标从未配置则清空这些字段。切离有未保存参数草稿的提供商时提示丢弃该草稿，避免异步读取覆盖用户正在编辑的目标字段。Secret 输入始终为空，`client_secret_configured` 反映目标提供商的实际可用状态；不将源提供商 Secret 或“已配置”标记带入目标。
  2. 服务端以目标提供商已存的 `base_url` / `realm` / `client_id` 为一组比较：PUT 显式提供新 Secret 时加密替换；Secret 留空且三字段均与目标已存值一致时，才允许保留目标旧密文；任一字段变化或目标无旧 Secret 时，拒绝空 Secret 保存并要求输入新 Secret。比较与拒绝必须位于后端，覆盖直接调用 API 及同一提供商原地修改参数；校验失败不得写入参数或切换生效提供商。
  3. 保存成功后才切换生效提供商并重新读取其配置；目标与源提供商的参数分别保留，已绑定旧提供商的用户身份不自动迁移。目标提供商读取按本轮确认提供最小损坏标记与显式新 Secret 重填路径；完整的当前提供商损坏状态机、签名密钥故障专用交互与测试连接专门结果仍由 R31-04 处理，不得静默当作“从未配置”。“暂未启用”的持久化语义由 R31-07 处理。
- **实施方案确认（2026-09-15 第三轮核验，用户确认）：** 不采用“切换时批量失效旧 state”，改为固定发起时边界：`oidc_states` 增加 `provider_type` 与发起时该提供商参数原始 JSON 的带版本哈希；`StartFlow` 在单个 `BEGIN IMMEDIATE` 事务内读取 provider/raw 参数并写入 state，`Exchange` 在发出 discovery/token 请求前校验“当前生效提供商一致 + 当前参数哈希一致”，不一致直接拒绝。已有旧 state 行保留原样，缺失发起标识时在 `ConsumeState` 阶段拒绝，回调表现为 `state_expired`；旧行由既有 TTL 清理。切走后再切回且参数原始 JSON 完全未变时，仍属于同一发起边界；任何 provider/参数变化都会使旧 state 失效。此方案与 R31-05 的“仅修改地址时保留该次 `redirect_uri`”区分，不把 `frontend_url`/`callback_url` 纳入本指纹。
- **验收重点：** 覆盖切到未配置目标、切到已有完整配置并原样复用、切回源提供商、修改地址/Realm/Client ID 后空 Secret 被拒且配置不变、显式新 Secret 替换、未保存草稿切换、保存前后测试连接，以及本地登录关闭时的切换保护；补充授权发起后切换提供商再回调，断言不会用目标提供商的地址/Secret 处理旧流程；用隔离 mock/接口测试验证，不把自动化结果表述为真实 IdP 登录验收。
- **实施与自动化隔离证据（2026-09-15）：**
  1. 后端完成目标提供商读取：`GET /api/admin/settings/oidc?provider_type=<target>` 只返回目标自己的 `base_url` / `realm` / `client_id` 与 `client_secret_configured`，Secret 始终空回显；可解析损坏 Secret 时返回非 Secret 字段与只读 `params_damaged` / `params_warning`，整体 JSON 损坏时不猜测字段。对应覆盖见 `backend/internal/config/admin_test.go`、`backend/internal/server/settings_oidc_test.go`。
  2. 空 Secret 复用校验前置到 `AdminService.SaveOidc`，最终判定下沉到 `oidc.SaveParams` 的单个 `BEGIN IMMEDIATE` 事务：目标缺失、JSON 损坏、Secret 不可解密/占位符、`base_url`/`realm`/`client_id` 任一变化均拒绝且不写参数、不切换生效提供商；显式新 Secret 可替换，JSON 整体损坏时要求重填必要非 Secret 参数。
  3. `oidc_states` 新增 `provider_type` / `config_hash`（迁移 `1019_oidc_state_signature.sql`）并保留旧行；`StartFlow` 在同一写事务内固定发起时 provider/raw 参数指纹，`Exchange` 在 discovery/token 请求前校验当前 provider 与指纹，不一致直接拒绝；旧行在 `ConsumeState` 阶段以 `state_expired` 拒绝并保留至 TTL。覆盖见 `backend/internal/oidc/r31_03_test.go`、`backend/internal/store/migration_1019_test.go`。
  4. 隔离 HTTP/TLS 接口覆盖：源字段 + 目标空 Secret 直接 API 400 且数据库不变；切到已有目标空 Secret 保留目标密文；切回源保留源密文；保存前空 Secret 测试连接只回退目标已存 Secret，字段变化不回退/不发凭据，显式新 Secret 保存后回退新值，源/目标 Secret 互不进入对方地址；Dev mock 发起授权后切换真实提供商再回调，以 `exchange_failed` 拒绝且不签发 ticket；本地登录关闭时空 Secret 切换拒绝、显式新 Secret 可完成切换；前端未保存草稿切换、目标损坏警示、读取失败保持源字段均有测试。
  5. 门禁通过：`cd backend && go build ./...`、`go vet ./...`、`go test ./... -count=1`、`go run ./cmd/errgate ./...`、`go test -race ./internal/oidc ./internal/config ./internal/server -count=1`；`cd frontend && npm run test`（280 项）、`npm run build`。
- **真实 IdP 验收边界：** 上述证据均为隔离 mock/TLS 端点、数据库与服务/HTTP 接口测试，不等同于真实 IdP 授权码/PKCE 登录验收；真实 IdP 登录仍需在提供隔离 discovery/client/回调环境的条件下单独核验并记录。
- **状态：** ☑ 字段语义、实施方案、代码与自动化隔离验证完成；真实 IdP 登录待隔离环境人工验收。

## 三、R31-04 字面值/不可解密占位符未强制重填（中）

- **现象与根因：** 当前损坏判定依赖“能解密且等于 `***`”。如果库内 `client_secret` 是字面 `"***"` 或无法解密的密文，`LoadParams` 报错会跳过损坏检测；GET 还会丢失原 BaseURL/ClientID 回显，空值保存可能把该值原样写回。
- **影响范围：** 非典型损坏状态无法按 R30-05 的“判未配置并强制重填”处理；`GetOidc` 将参数缺失、JSON 解析失败与 Secret 解密失败混为未配置，可解析 JSON 中原有的 `base_url` / `realm` / `client_id` 也不回显。本地登录开启时，空 Secret 保存可能继续保留坏值；管理员测试连接在发现文档可达时可能只给出“未提供 Client Secret，未执行凭据校验”的警告。现有关闭本地登录检查与真实登录参数读取会拒绝不可读 Secret，但不能替代管理员修复路径；当前无真实库内损坏的现场证据。
- **已确认边界（2026-09-14）：** 用户确认：原始 OIDC 参数 JSON 非空但无法解析，以及 JSON 可解析但 Secret 为字面 `***`、无法解密或解密后为 `***`，均明确标为损坏；JSON 可解析时保留非 Secret 字段供管理员核对，空 Secret 不得保存。`signing_key` 缺失或读取失败属于全站密钥故障，须单独报错并阻止通过 OIDC 页面重填，不得在此路径自动生成新密钥；密钥可读而该提供商密文无法解密时，按 OIDC 配置损坏处理。JSON 为空按参数未配置处理；JSON 可解析但 Secret 为空按尚未配置可用 Secret 处理，不与损坏混淆。
- **拟议处理合同：**
  1. OIDC 服务提供一次结构化参数检查，区分未配置、尚缺可用 Secret、可用、JSON 损坏、Secret 损坏与签名密钥故障；供面板 GET、保存校验和测试连接复用，真实登录链路继续对损坏参数拒绝进入。GET 对损坏状态返回明确的只读状态，`client_secret_configured=false`；JSON 可解析时回显 `base_url` / `realm` / `client_id`，整个 JSON 无法解析时不猜测这些字段。GET 始终保持 `client_secret` 为空，响应与日志均不得包含 Secret 明文、原始密文或签名密钥。签名密钥故障以独立错误提示，不伪装为某个提供商“未配置”或可通过重填恢复。
  2. PUT 显式提交 `***` 继续拒绝；目标提供商已存参数损坏且 Secret 留空时，返回明确校验错误并要求输入新 Secret，在写入参数、站点地址或切换生效提供商前停止。显式输入新 Secret 时，用新密文替换坏值；整个 JSON 损坏时须同时重新填写必要的非 Secret 参数。底层 `SaveParams` 的空值保留路径也必须验证原值可用，不能只复制原始 `client_secret`；签名密钥故障时即使提交新 Secret 也不得写入。旧值读取、校验和相关配置写入应在同一写事务内完成，避免校验后旧值变化或失败后留下半套配置；不得在事务内绕回非事务配置读写接口。
  3. 管理员测试连接在 Secret 留空且目标已存参数损坏时，直接返回“已存配置损坏，须重填”的失败结果，不继续以普通“未提供 Secret”警告代替；显式新 Secret 草稿仍可测试。已有的回退限制继续有效：仅在请求的提供商与目标已存 `base_url` / `realm` / `client_id` 完全一致、且已存 Secret 可用时，才复用它。与 R31-03 合并验收其“非 Secret 字段改变则不能留空沿用旧 Secret”规则，不把源提供商字段或 Secret 混入目标。
  4. 设置页对损坏状态显示明确警示和重填动作；可解析时保留非 Secret 字段，Secret 输入框仍为空。损坏时不再显示“留空保持原值”的通用提示；切换提供商后只展示目标提供商自己的字段与状态。签名密钥故障显示独立系统错误并阻断面板重填提交，不提示管理员用新 Secret 覆盖恢复。
- **验收重点：** 以隔离数据覆盖字面 `***`、非法/不可解密密文、解密后为 `***`、损坏 JSON、空 JSON、可解析但空 Secret、正常密文与签名密钥缺失/读取失败；分别断言 GET 状态与字段、空值保存拒绝且数据库不变、显式重填恢复、底层保存入口、测试连接不误报、关闭本地登录防死锁及提供商切换边界。自动化/本地 mock 不等于真实 IdP 登录验收；真实登录仍单独核验。
- **与 R31-03 的边界衔接（2026-09-15 第三轮核验后更新）：** R31-03 已在目标提供商切换路径中落地最小损坏读取与重填语义，R31-04 不得再设计一套与之冲突的空值保存或目标读取判定：
  1. R31-03 在 OIDC 业务层提供可复用的结构化参数检查，并用于 `GET /api/admin/settings/oidc?provider_type=<target>` 的目标读取；可解析的非 Secret 字段继续回显，响应增加只读 `params_damaged` / `params_warning`，Secret 始终空回显。
  2. JSON 可解析但 Secret 不可解密、字面 `***` 或解密后为 `***` 时，目标读取返回可解析字段和损坏提示；JSON 整体不可解析时返回空字段和“须重新填写必要参数”的提示；签名密钥故障返回独立系统错误，不伪装成目标 Secret 损坏。
  3. 目标损坏时，空 Secret 保存在任何参数、站点地址或生效提供商写入前拒绝；显式新 Secret 可覆盖修复；JSON 整体损坏修复时须同时重新填写必要的非 Secret 参数。
  4. R31-03 已将空 Secret 复用的最终判定下沉到 `oidc.SaveParams` 的同一写事务内，保证原字段组和旧 Secret 可用性校验与保留密文写回不可分离。R31-04 应复用该原子守卫并扩展损坏分类，不得另建“先读后写”的空值保存判定。
  5. R31-04 单独完成的范围（本轮已实施）：当前生效提供商 GET 的完整损坏状态机、`signing_key` 缺失/读取失败的专用交互、管理员测试连接对“已存配置损坏”的专门失败结果、完整状态枚举及设置页统一展示。不得把 R31-03 的目标读取最小子集宣称为 R31-04 已完成。
- **实施与自动化隔离证据（2026-09-15，用户确认后按 T1/S1/K1/SCOPE-A/TC-A 串行实施）：**
  1. 新增 `config.OidcParamsStateCode` 六态枚举（`not_configured/missing_secret/usable/json_damaged/secret_damaged/signing_key_fault`）与固定提示；`GET` 的 `params_state` 对当前和目标提供商统一返回，Secret 始终为空，JSON 可解析时保留非 Secret 字段，JSON 损坏时不猜测字段；`signing_key` 缺失/NULL 读取失败在 JSON 可解析时也保留字段并返回 `signing_key_fault`。
  2. 管理端保存改为 T1：`SaveOidc` 在单个 `BEGIN IMMEDIATE` 内完成旧参数读取、分类、字段校验、严格读取 `signing_key`、加密/保留密文、写参数、写 `oidc_provider_type`/`oidc_configured` 与站点地址；事务内只使用 `GetTx`/`SetTx`/`GetSigningKeyTx`/`EncryptWithTx` 口径接口。空 Secret 复用继续由 `oidc.SaveParamsTx` 原子守卫裁决；显式新 Secret 不再经 `EncryptSensitive` 自动生成密钥。
  3. `SaveParams`/`SaveParamsTx` 使用同一分类器；损坏状态空 Secret 一律拒绝且 raw JSON 不变，显式新 Secret 可修复字面 `***`、非法/旧密钥密文、解密后 `***` 与坏 JSON（补齐必要 Base URL/Client ID）；`signing_key` 故障时显式新 Secret 也拒绝且不生成新密钥。
  4. 管理端测试连接对 JSON/Secret 损坏返回专门“已存配置损坏，须重填”失败且 0 网络请求；显式新 Secret 仍可测试；`signing_key` 故障按 K1 阻断保存与测试，直接 PUT 返回固定安全 503 文案，底层错误只进脱敏日志。
  5. `StartFlow`/`Exchange` 复用同一分类器，对 JSON 损坏、Secret 损坏与 `signing_key` 故障在 discovery/token 网络请求前拒绝；`missing_secret` 保持 public client/PKCE 现状语义；R31-03 的 provider/config 指纹边界不变。
  6. 设置页按六态显示警示/重填提示；`signing_key_fault` 显示独立系统错误并禁用保存、测试连接及 OIDC 凭据重填输入；损坏状态不再显示“留空保持原值”的通用提示。
  7. 隔离测试覆盖：临时库逐一写入空 JSON、`{}`、空 Secret、字面 `***`、非法密文、解密后 `***`、正常密文、坏 JSON、`signing_key` 缺失与 NULL 读取失败；HTTP 断言 `params_state`/字段/Secret 空回显且响应不泄露原始密文；T1 用 SQLite 触发器在 `oidc_configured` 更新点注入失败，断言参数、provider、configured、站点地址全部回滚；测试连接覆盖损坏专门失败 0 网络请求、密钥故障阻断显式新 Secret、正常密文字段一致回退、字段变化不回退。门禁：`go build ./...`、`go vet ./...`、`go test ./... -count=1`、`go test -race ./internal/oidc ./internal/config ./internal/server -count=1`、`go run ./cmd/errgate ./...`、`npm run test`（284 项）、`npm run build` 均通过。
- **真实 IdP 验收边界：** 上述均为隔离数据库、HTTP/TLS 端点和前端组件测试，不等同于真实 IdP 授权码/PKCE 登录验收；真实 IdP 登录仍需在隔离 IdP 环境单独核验并记录。
- **状态：** ☑ 代码与自动化隔离验证完成；真实 IdP 登录待隔离环境人工验收。

## 四、R31-05 导入登录入口与 OIDC 地址生效语义（同轮观察）

- **导入登录入口：** `ValidateImportedAuthUsable` 在 `configured=true` 且本地登录关闭时校验提供商参数与 Secret，却未校验 `oidc_configured`。系统状态接口按该标记返回 OIDC 状态，登录页据此显示入口；极端导入文件可能使参数完整、校验通过，但登录页同时隐藏本地与 OIDC 登录入口。此处尚无真实导入事故证据。
- **地址保存与回调：** OIDC 保存接口仅凭入参 `frontend_url` / `callback_url` 非空返回 `need_restart=true`，相同值重复保存也提示重启。现有配置读取按请求查库，`frontend_url` 的使用路径实际读取新值；独立 `callback_url` 虽保存和回显，真实 OIDC 的 `redirect_uri` 仍由 `frontend_url` 拼接，未使用该字段。归档 [Design1.md](docs/reports/Design/Design1.md) §3.1、§3.4.8 与第八章的“两个地址启动缓存、修改后重启生效”描述与当前实现不一致。
- **归属边界：** `GetOidc` 在 Secret 解密失败时丢失非 Secret 字段回显，已并入 R31-04 的损坏状态与恢复合同；R31-01 的导出密钥损坏和 R31-06 的 Production mock 限制分别处理。
- **已确认处理合同（2026-09-15）：**
  1. 导入后的系统已配置且本地登录关闭时，必须要求导入文件的 `oidc_configured=true`，并继续校验当前提供商参数与 Secret；标记缺失或为 false 时在覆盖写入前拒绝，不自动改为 true。该条件同时覆盖 v1/v2 与 Setup/管理端导入；本地登录开启时不因 OIDC 未启用而拒绝。参数检查不能证明真实 IdP 可登录。
  2. `frontend_url` 与 `callback_url` 改为保存后即时生效；真实 OIDC 优先使用已保存的独立 `callback_url`，未设置时沿用由 `frontend_url` 推导的现有回调路径。地址变化不再因启动缓存返回 `need_restart=true` 或提示“需重启生效”。这是用户明确选择的新合同，覆盖上述归档 Design1 的旧描述；实施时同步现行设计、设置页与导入提示中涉及地址生效的文案。导入是否因其他状态仍需重启，应独立核对，不由本条推断。
  3. 发起 OIDC 授权时将该次使用的 `redirect_uri` 随 state 保存，回调换取 token 时复用同一值；管理员修改地址后，新发起的登录立即使用新地址，进行中的登录仍使用原地址。若旧域名在进行中的授权完成前已不可访问，该次登录仍可能失败，应作为换域名操作边界说明。
- **方案核验补充（2026-09-15）：** 当前管理端保存把空 `callback_url` 视作“不修改”，因此已保存独立回调地址后，单靠提交空值无法恢复“未设置时从 `frontend_url` 推导”的回退状态。实施时须定义并提供显式清除独立回调地址的写入语义，同时校验所填回调地址可回到本站实际注册的 OIDC 回调路径；地址变化与清除均须明确是否影响进行中的 state，保持第 3 点已确认的 `redirect_uri` 固定合同。
- **实施与自动化隔离证据（2026-09-15，用户确认按串行实施）：**
  1. 导入：`ValidateImportedAuthUsable` 在“已配置且本地登录关闭”分支强制 `oidc_configured=true`，并在原有 provider/HTTPS base_url/Secret 校验后要求可解析出有效回调地址（独立优先、否则 `frontend_url` 推导）；本地登录开启时不因 OIDC 未启用/无地址拒绝。v1 `Import` 保持覆盖事务前校验；v2 `ImportV2` 在注册异步任务前同步执行同一校验，`importV2` 内保留校验覆盖直接调用。隔离测试覆盖缺失/false/true、本地登录开/关、参数损坏、地址缺失/错误，断言拒绝时 `system_config` 全表不变且未创建异步任务；HTTP 覆盖 Setup/管理端两个入口。证据见 `backend/internal/config/export_oidc_test.go`、`backend/internal/config/r3105_import_test.go`、`backend/internal/server/r31_05_test.go`。
  2. 地址：新增 `urlguard.ParseAbsoluteHTTPURL` / `ValidateOIDCCallbackURL` 与 `config.ResolveOidcCallbackURL`；`config.OidcCallbackPath` 同时用于真实 Gin 回调路由注册与校验，防止路径漂移。`SaveOidc` 在同一 `BEGIN IMMEDIATE` 内校验/写入 `frontend_url` 与 `callback_url`：空 `callback_url` 仍=不修改，`clear_callback_url=true` 显式清除并恢复推导回退；清除要求当前/提交的 `frontend_url` 能推导出有效绝对地址；独立回调允许任意 host，但路径必须精确为 `/api/auth/oidc/callback`，无 userinfo/query/fragment。本地登录关闭时，`SaveOidc` 防死锁与 `oidcAvailable`（`SaveLocalAuth` 路径）都要求可解析有效回调地址。保存响应不再返回 `need_restart`，设置页移除重启提示并增加“恢复推导”、当前生效回调和 host 不一致警示。证据见 `backend/internal/urlguard/urlguard_test.go`、`backend/internal/config/r31_05_test.go`、`backend/internal/server/r31_05_test.go`、`frontend/tests/settings-view.spec.ts`。
  3. state 固定：迁移 `1020_oidc_state_redirect_uri.sql` 增加 `redirect_uri`；`StartFlow` 在同一写事务内读取 provider/参数与地址、解析并固定 `redirect_uri` 后写 state；`ConsumeState` 拒绝缺少该值的旧行并保留原行；`Exchange` 只使用 state 中的固定值，不再读取当前地址。地址变化/清除不会改变 R31-03 的 provider/config 指纹，因此不影响进行中的 state；新发起的授权使用新地址。证据见 `backend/internal/oidc/r31_05_test.go`、`backend/internal/store/migration_1020_test.go`。
  4. Setup 默认值：`CompleteOidcSetup` 不再写独立 `callback_url`，新装默认由 `frontend_url` 推导；旧导出中已保存的独立回调仍按严格整体覆盖优先。证据见 `backend/internal/setup/setup_test.go`。
  5. 前端与文案：设置页保存时传递 `clear_callback_url`，展示有效回调与清除状态，独立回调 host 不一致时提示 state Cookie host-only 边界；管理端/Setup 导入完成文案改为“地址与 OIDC 即时生效、重新登录、启动参数重启后生效”。证据见 `frontend/src/views/admin/SettingsView.vue`、`frontend/src/views/SetupView.vue`、`frontend/src/api/settings.ts`、`frontend/tests/settings-view.spec.ts`。
  6. 门禁：`go build ./...`、`go vet ./...`、`go test ./... -count=1`、`go run ./cmd/errgate ./...`、`go test -race ./internal/oidc ./internal/config ./internal/server -count=1`、`npm run test`（287 项）、`npm run build` 均通过。上述均为隔离数据库、HTTP/TLS mock 与前端组件测试，不等于真实 IdP 授权码/PKCE 登录或真实浏览器/反代验收。
  7. 审计后修复（2026-09-15）：导入校验将 `configured` 改为与运行时一致的 `strconv.ParseBool` 语义，非法值直接拒绝；本地登录关闭分支要求 `oidc_provider_type` 属于真实提供商白名单。前端切换提供商时不再重置 `clearCallbackURL`，显式清除独立回调的未保存草稿得以保留。新增 `TestR3105ValidateImportedAuthUsableBooleanAndProviderSemantics` 与设置页“已标记清除后切换提供商”组件用例。修复后 `go test ./... -count=1`、`go test -race ./internal/oidc ./internal/config ./internal/server -count=1`、`go run ./cmd/errgate ./...`、`npm run test`（293 项）与 `npm run build` 均通过。

- **验收重点：** 隔离导入覆盖 `oidc_configured` 缺失/false/true、本地登录开/关、v1/v2 与 Setup/管理端入口，断言拒绝时配置不变；接口与服务测试覆盖地址原值重复保存、地址变化即时生效、独立回调优先、显式清除后未设置回退、显式清除草稿跨提供商保留、导入 `configured` 非规范真值与非法 `provider_type` 拒绝、回调路径校验、授权与 token 交换之间改址时 `redirect_uri` 一致，并核对设置页/导入文案。自动化或 mock 不等于真实 IdP 登录验收。
- **状态：** ☑ 代码与自动化隔离验证完成（含审计后修复）；真实 IdP 登录/浏览器/反代跨 host 验收待隔离环境人工核验。

## 五、R31-06 OIDC mock 模式校验不完整（高，既有问题）

- **现象与根因：** Setup / `SaveOidc` 接受 `provider_type=mock` 未检查运行模式；`flow.Exchange` 的 mock 分支也不检查 `s.mode`，只有 `MockLogin` 检查 Dev。生产实例若存在 mock 配置，登录发起/回调仍可能走 mock 解析路径。
- **影响范围：** 生产模式模拟登录绕过；与 `SaveLocalAuth` 的 mock 可用性判定（本轮已限制 Dev）存在配置状态不一致。现有配置导入还可能将 mock 参数写入 Production；未来整站恢复会同时携带用户及 OIDC 身份绑定，不能只凭恢复后的当前提供商判断身份来源。
- **已确认处理合同（2026-09-15）：**
  1. Production 的 Setup 与管理端保存均拒绝 `provider_type=mock`，不写入配置；测试连接不再报告 mock 通过。登录/绑定授权在写入 state 前拒绝 Production mock，回调换取身份时再次拒绝，覆盖已有库内配置及进行中的登录。公开状态与登录页不得把 Production mock 呈现为可用登录方式；管理端可只读查看历史配置并提示需切换到真实提供商，不自动删除参数。
  2. 现有 Production 配置导入在任何覆盖写入前检查 v1/v2 文件：`oidc_provider_type=mock`，或存在 `oidc_params_mock` 键（含空值），均整体拒绝；Setup 与管理端入口同口径，拒绝后配置不变。此规则独立于本地登录是否开启及 `oidc_configured` 标记。已有 Production 配置若保留 mock 历史键，其导出文件也不能直接往返导入，须先清理该键再重新导出。
  3. mock 可用性依据启动时确定的运行模式，不能信任可被配置导入整体覆盖的 `system_config.app_mode`。未来 Design5 整站导出/导入仅面向 Production；整站恢复须在替换目标数据前拒绝非 Production 来源文件及任何 mock 配置，不设计 Dev 文件的导入/导出。新格式的来源模式由文件元数据表达，不能从待恢复的配置键推断；元数据本身不等于不可伪造的来源证明，具体格式与验证机制仍由 Design5 后续设计决定。现行配置导入的安全修复不等待整站迁移实施。
- **验收重点：** 隔离 Production 回归覆盖 Setup/管理端保存与测试连接、已有 mock 配置的公开状态、登录/绑定发起与回调，断言拒绝后无配置写入、无新 state、无会话签发；v1/v2 的 Setup/管理端导入覆盖生效类型为 mock、仅保留空/非空 `oidc_params_mock`、本地登录开/关及 OIDC 启用标记，均在覆盖前整体拒绝。Dev mock 原有路径保持可用；未来整站恢复另测非 Production 来源、含 mock 配置与失败后目标数据不变，不以现有配置导入测试代替。
- **后续边界裁决（2026-09-15，用户确认）：** 当前项目按全新激活设计，不存在旧版本兼容要求；R31-06 不实现旧 `oidc_params_mock` 键清理/重新导出修复路径。含 mock 键的旧导出文件直接按上述导入规则整体拒绝，不属于本项支持范围。
- **实施与自动化隔离证据（2026-09-15，用户确认后串行实施）：**
  1. 运行模式：`config.AdminService` / `server.SetupHandler` 注入启动 `APP_MODE`；`oidcAvailable` 与 OIDC mock 分支不再读取 `system_config.app_mode`。隔离测试在 Production 库把 DB `app_mode` 写成 `dev`，断言公开状态、保存与防锁死判断仍按启动 `prod` 处理。
  2. 写入口与登录：`SaveOidc`、Setup/OIDC Setup 与底层 `SaveParams(Tx)` / `SaveRawParamsTx` / `SetProviderTx` 对 Production mock 拒绝且无配置写入；`StartFlow` 在清理过期 state/写入新 state 前拒绝；`Exchange` 在解析 mock code 前拒绝；`MockLogin` 保持 Dev 限制；`TestConnection*` 返回 `ok=false`。
  3. 公开状态与管理端：Production mock 的 `/api/system/status` 返回 `oidc_configured=false`、provider 为空；管理端 GET 只读保留历史参数并返回固定切换提示；设置页的 mock 选项在 Production 不可选，历史 mock 禁止保存/测试；登录页增加同口径隐藏防御。
  4. 导入：新增 `validateImportedNoMock`，在 `Import` / `ImportV2` / `importV2` 与 `ValidateImportedAuthUsable` 中检查 `oidc_provider_type=mock` 或 `oidc_params_mock` 键存在（含空值），均在覆盖写入/注册异步任务前整体拒绝，拒绝后 `system_config` 不变。
  5. 证据文件：`backend/internal/config/r31_06_test.go`、`backend/internal/oidc/r31_06_test.go`、`backend/internal/server/r31_06_test.go`、`frontend/tests/settings-view.spec.ts`、`frontend/tests/form-submit.spec.ts`。
  6. 门禁：`cd backend && go build ./...`、`go vet ./...`、`go test ./... -count=1`、`go run ./cmd/errgate ./...`（0 unexpected）、`go test -race ./internal/oidc ./internal/config ./internal/server -count=1` 均通过；`cd frontend && npm run test`（289 项）与 `npm run build` 均通过。
- **残余边界（R31-06 完成时点）：** 旧 Dev 已签发且未兑换的 `oidc_login_tickets` 不携带 provider 归属；该边界随后续 R31-07 的流程代际与 `flow_hash` 守卫关闭，见第六节。未来 Design5 整站恢复的非 Production 来源与 mock 配置拦截仍未实现、未验收，现有配置导入修复不替代整站恢复测试。
- **状态：** ☑ R31-06 代码与自动化隔离验证完成；真实 IdP 登录、Design5 整站恢复未验收。R31-07 在 R31-06 完成时为未实施状态，后续已由第六节的流程代际与 `flow_hash` 实现完成并关闭旧 Dev ticket 残余边界。

## 六、R31-07 OIDC「暂未启用」未真正落库停用（中）

- **现象与根因：** 设置页把「暂未启用」映射为 `provider_type=''`，但 `onProviderChange` 只改本地表单并折叠参数区，不调用任何保存接口；`SaveOidc` 的 `validProviders` 也不接受空 provider_type。因此选择「暂未启用」不会真正停用 OIDC，离开页面后后端仍按旧提供商工作，仅形成本地未保存状态。
- **影响范围：** OIDC 生命周期语义、公开登录入口、个人中心绑定入口、直接调用登录/绑定接口及“停用后重启用”的人工核验；不改变 R30-05 的 Secret 空回显、保留/替换与占位符拒绝合同。当前真实授权流程未统一检查 `oidc_configured`，单纯关闭该标记但保留提供商参数仍不足以阻断直接请求。
- **已确认设计（2026-09-15）：** 「暂未启用」是保存后落库的停用状态；停用时保留当前提供商类型和各提供商参数，供重新启用。选择下拉项仅形成页面草稿，不立即改变后端状态；“清空 OIDC 配置”仍是删除全部提供商参数的独立操作。停用阻止新的 OIDC 登录、绑定、进行中的回调和未兑换登录票据；已登录用户的现有会话保持有效，直到自然过期或按既有规则失效，不因停用而撤销。
- **拟议处理合同：**
  1. 继续以 `oidc_configured` 表示生效启用状态，停用时写为 false，但不清空 `oidc_provider_type`、`oidc_params_*`、站点级地址或已绑定用户身份；管理端 GET 分别返回启用状态与保留的提供商，Secret 始终空回显。公开状态与登录/绑定入口以启用状态为准，不将保留参数误报为正在提供登录能力。无需另增“上次提供商”配置键。
  2. 设置页在选择「暂未启用」后显示未保存状态和明确的“保存停用”操作，确认文案说明登录/绑定将关闭、参数保留以及现有会话继续有效；保存成功后重新读取配置并强制刷新系统状态。停用态展示保留的提供商，重新选择提供商时按 R31-03 读取目标提供商自己的字段；保存成功后才重新启用，并按 R31-03/R31-04 校验目标参数、旧 Secret 复用与损坏状态。切离未保存草稿仍须提示丢弃，不以选择下拉项自动保存或启用。
  3. 后端使用与清空操作区分的停用写入路径，不以空 `provider_type` 调用现有 `SaveOidc`，也不调用会清除参数的 `ClearOidc`。本地登录关闭时拒绝停用且配置不变；停用与关闭本地登录的检查、写入须在写事务内串行，防止并发请求各自通过旧状态检查后形成双登录不可用。事务内只用事务配置读写接口；相邻的 OIDC 保存/清空路径也需核对同一防死锁边界。
  4. OIDC 服务端在登录/绑定发起写入 state 前检查启用状态；回调换取身份、模拟登录、绑定落库和签发新会话前再次守卫，停用后未兑换的登录票据不得返回会话。不凭页面隐藏入口替代服务端校验；已有会话继续走既有校验，不新增按登录来源的撤销机制。与 R31-06 的 Production mock 限制合并核对这些入口，但不混同两个问题的授权和状态。
- **方案核验补充（2026-09-15）：** 仅在兑换时读取当前 `oidc_configured` 不足以满足“停用阻断进行中流程”：停用后若在旧 state/票据 TTL 内重新启用，旧记录可能重新可用。停用须使此前的 state 与未兑换票据不可逆失效（可在同一停用事务清理，或采用流程版本校验），重新启用不能恢复它们。最终绑定落库、会话签发/票据创建和票据兑换的启用检查还须与停用写入串行，避免检查通过后停用已提交却仍完成写入或返回会话；具体失效机制需在实施前确定。
- **相邻多键写入事务风险证据（2026-09-15，R31-03 风险 4；R31-07 已据此完成事务化改造）：**
  1. 现状调用链：`AdminService.SaveOidc` 依次调用 `oidcOps.SaveParams` 写参数 JSON、`cfg.Set(oidc_provider_type)`、`cfg.Set(oidc_configured)`，再按需写 `frontend_url` / `callback_url`，最后 `ClearDiscCache`；各步分散在多个自动提交语句中，未包裹在单个写事务内。
  2. `ClearOidc` 同样先写 `oidc_configured=false`，再清 `oidc_provider_type` 和各 `oidc_params_*`，也不是单事务；`SaveLocalAuth` 与这两条 OIDC 写入路径之间也没有共同的写事务或串行化。
  3. 已识别的失败/并发窗口：① 参数 JSON 已写但 `oidc_provider_type` 写失败 → 目标参数成为未生效残留；② `oidc_provider_type` 已写但 `oidc_configured` 写失败 → 生效提供商与“已启用”标记不一致；③ 参数与提供商已完成切换但站点地址写失败 → API 返回失败而配置已部分生效；④ 目标与源地址指向同一 discovery 缓存键时 `ClearDiscCache` 未执行可能残留旧发现文档；⑤ `ClearOidc` 部分失败时可能留下“已标记停用但提供商/参数未清空”的中间态。
  4. 与 R31-03 的关系：固定发起 state 快照在单个 `BEGIN IMMEDIATE` 事务内读取 provider/raw 参数并校验指纹，空 Secret 复用也下沉到 `SaveParams` 的同一写事务；因此“旧授权码由新提供商地址/Secret 处理”和“空 Secret 字段混用”已由 R31-03 封死，但 `SaveOidc` / `ClearOidc` 的多键整体原子性仍未解决。
  5. R31-07 研究/验收要求：不得只做静态阅读；需用故障注入或可观察的写失败点验证上述中间态，评估把 OIDC 保存/停用/清空及相邻本地登录检查统一纳入写事务或服务层串行化；事务内只能使用 `GetTx` / `SetTx` / `EncryptWithTx` 等事务接口，不得回退到非事务配置读写。该研究结论未确定前，R31-03 不实施跨包事务改造。
  - **R31-07 实施与自动化隔离证据（2026-09-15）：**
    1. 流程代际与指纹：新增 `oidc_flow_epoch` 配置键与迁移 `1021_oidc_ticket_flow_hash.sql`（`oidc_login_tickets.flow_hash`）；`StartFlow` 在写 state 前固定 `SHA256("v2"+mode+epoch+provider+rawParams)`，`Exchange` 在网络请求前校验同一指纹；停用/清空轮换代际，重新启用不恢复旧流程。旧格式 state hash 与空 `flow_hash` ticket 均按失效处理，关闭 R31-06 的旧 Dev ticket 残余边界。
    2. 事务边界：`AdminService.DisableOidc`、`ClearOidc`、`SaveLocalAuth` 均改为单个 `BEGIN IMMEDIATE`；停用/清空在同一事务清理 `oidc_states`/`oidc_login_tickets`、写 `oidc_configured=false` 并轮换 epoch；本地登录关闭在同一事务调用 `oidcAvailableTx`（仅 `GetTx`/`DescribeParamsTx`），与停用/清空串行，阻止双登录不可用。新增 `POST /api/admin/settings/oidc/disable`。
    3. 服务端守卫：`StartFlow`、`ConsumeState`、`Exchange`、`ResolveBind`、`ResolveLoginForFlow`、`MockLogin`、`IssueLoginSessionForFlow`、`IssueDirectSessionForFlow`、`ConsumeLoginTicket` 均在最终写入/兑换事务内校验启用状态与固定流程指纹；绑定/建号写入通过 user 包 guarded 事务方法执行，会话签发使用 `auth.IssueTx` 在同一事务内完成。已有 JWT 会话校验链不变。
    4. 管理端与前端：GET 增加只读 `enabled`，停用态保留 provider 与参数；公开状态停用时不返回保留 provider。设置页选择「暂未启用」仅形成草稿并显示「保存停用」，保存后重新读取配置并刷新系统状态；重新选择 provider 后保存成功才启用；清空 OIDC 仍是独立危险操作。
    5. 证据文件：`backend/internal/config/r31_07_test.go`、`backend/internal/oidc/r31_07_test.go`、`backend/internal/oidc/r31_07_concurrency_test.go`、`backend/internal/server/r31_07_test.go`、`backend/internal/store/migration_1021_test.go`、`frontend/tests/settings-view.spec.ts`。覆盖停用落库与参数/绑定保留、故障注入整体回滚、停用 vs 关闭本地登录并发、state 已消费后进行中回调不可恢复、旧 ticket/空指纹 ticket 拒绝、停用与绑定/签发并发交错无停用后结果、直接 API 与公开状态、已有会话仍有效。并发测试使用事件表触发器记录 `disable`/`bind`/`ticket` 写入顺序，断言不存在停用后新绑定或 ticket。
    6. 门禁：`go build ./...`、`go vet ./...`、`go test ./... -count=1`、`go run ./cmd/errgate ./...`、`go test -race ./internal/oidc ./internal/config ./internal/server -count=1`、`npm run test`、`npm run build` 均通过。以上均为隔离数据库/HTTP/前端组件测试，不等同于真实 IdP、真实浏览器、跨 host 反代及人工并发验收。
    7. 审计后修复（2026-09-15）：设置页在“暂未启用”未保存草稿上切回提供商时，`onProviderChange` 先弹出丢弃确认，不再直接加载目标字段覆盖停用草稿；未保存停用仍不会自动保存或启用。HTTP 回归同时用真实 OIDC 会话 JWT 验证停用后已有会话继续有效。新增 `frontend/tests/settings-view.spec.ts` 组件用例；修复后前端 `npm run test`（293 项）与 `npm run build` 均通过。

  - **人工验收边界：** 选择后仅草稿、保存停用后刷新仍停用、停用态展示保留 provider、重新启用原/其他提供商、清空与停用区别、本地登录关闭/并发保存人工复核、直接 API 与旧回调/旧 ticket 重放、已有会话继续有效等仍需在真实浏览器/隔离 IdP 环境逐项记录；真实 IdP 登录仍按既有人工验收边界执行。

- **验收重点：** 覆盖选择后未保存、保存停用与刷新后仍停用、保留各提供商参数和绑定、重新启用原/其他提供商、清空与停用的区别；本地登录关闭时停用拒绝且数据库不变，以及停用与关闭本地登录并发保存；公开状态、登录页、绑定页和直接 API 的一致性；发起前无新 state、停用前已发起回调不能绑定或签发、未兑换票据不能返回会话、停用后在旧记录 TTL 内重新启用仍不能兑换旧 state/票据、停用与绑定/签发/兑换并发交错时无停用后新结果、已有会话仍可用。隔离自动化与真实 IdP/浏览器人工验收分别记录。
- **状态：** ☑ 停用语义与既有会话边界已确认；☑ R31-07 代码与自动化隔离验证完成（含审计后修复）；☐ 真实浏览器/隔离 IdP 人工验收待执行。

## 七、处理建议

1. R31-01 由 [ExportRelated1.md](ExportRelated1.md) 独立跟踪；R31-02 已按用户单独授权修复并完成自动化隔离验证，真实 IdP 登录待隔离环境人工验收，不与 R30-05 混批。
2. R31-03 字段语义、实施方案、代码与自动化隔离验证已完成：增加按目标提供商读取能力，空 Secret 仅能与目标原地址/Realm/Client ID 组合复用，授权发起/回调固定 provider/config 指纹；真实 IdP 登录待隔离环境人工验收。
3. R31-04 已按用户确认的 T1/S1/K1/SCOPE-A/TC-A 完成代码与自动化隔离验证：六态状态机、管理端 GET/PUT 与 SaveOidc 单事务、底层 SaveParams 严格密钥读取、测试连接专门失败、真实登录网络前拒绝、设置页统一展示及响应/日志脱敏；真实 IdP 登录待隔离环境人工验收。R31-05 已完成代码与自动化隔离验证（含审计后修复）：导入同步校验 `oidc_configured`、`configured` 布尔语义、有效回调地址与真实 provider 白名单，地址保存即时生效、独立回调优先、显式清除回退且清除草稿跨提供商保留，OIDC state 固定 `redirect_uri`，Setup 默认不再写独立回调；真实 IdP/浏览器与跨 host 反代验收待隔离环境人工核验。R31-06 的现有 Production 导入拒绝与 Design5 整站恢复边界已确认，安全修复不等待整站迁移；R31-07 的持久化停用与既有会话边界已确认。
4. R31-07 已完成代码与自动化隔离验证（含审计后修复）：流程代际指纹、停用/清空/本地登录单事务、旧 state/票据不可恢复、直接 API 与公开状态守卫、设置页草稿/保存停用/重新启用，以及 from-off 未保存停用草稿切回提供商时的丢弃确认；真实浏览器与隔离 IdP 人工验收待执行。

---

## 变更记录

| 版本 | 日期 | 说明 |
|---|---|---|
| v1.0 | 2026-09-14 | 核验 Issue16 R30-05 后创建，登记 signing_key 导出损坏、真实 OIDC endpoint HTTPS 缺失及提供商切换/损坏占位符残余边界。 |
| v1.1 | 2026-09-14 | 二轮核验补充 R31-06：mock 模式校验不完整（Setup/SaveOidc/Exchange 未统一限制 Dev）。 |
| v1.2 | 2026-09-14 | 按用户指令将 Issue16 原 R30-07「暂未启用」未落库问题迁入本文件，重编号为 R31-07。 |
| v1.3 | 2026-09-14 | 将 R31-01 分至 ExportRelated1 独立跟踪；补充 R31-02 真实流程与测试连接差异、代理边界及拟议修复方案。 |
| v1.4 | 2026-09-14 | 按用户确认补充 R31-03：切换加载目标字段、未配置目标清空、旧 Secret 复用须匹配目标已存参数，以及实施与验收边界。 |
| v1.5 | 2026-09-14 | 按用户确认补充 R31-04：坏 JSON/Secret 统一标损坏并保留可解析非 Secret 字段；签名密钥故障独立处理；细化保存、测试连接、页面恢复及验收合同，尚未实施。 |
| v1.6 | 2026-09-15 | 按用户确认补充 R31-05：导入时校验 OIDC 启用标记；前端/回调地址即时生效并使用独立回调地址；同轮授权固定 redirect_uri，明确覆盖归档 Design1 的启动缓存旧描述；未实施代码。 |
| v1.7 | 2026-09-15 | 按用户确认 R31-06：现有 Production 配置导入遇任何 mock 配置整体拒绝；未来 Design5 整站导出/导入仅限 Production，恢复前拒绝非 Production 来源与 mock 配置；代码未实施。 |
| v1.8 | 2026-09-15 | 按用户确认 R31-07：暂未启用须保存落库、保留提供商参数、阻断新的 OIDC 登录/绑定及进行中流程；既有会话保持至自然过期或按既有规则失效。补充事务防锁死、界面与验收合同；代码未实施。 |
| v1.9 | 2026-09-15 | 核验 R31-02/03/05/07 方案并补充 token 凭据重定向、切换提供商时进行中流程、独立回调地址清除/路径、停用后旧 state/票据不可恢复及并发验收边界；未确定的实现策略保持待决，代码未实施。 |
| v2.0 | 2026-09-15 | 按用户确认实施 R31-02：统一 HTTPS 校验、discovery endpoint 缓存前校验与实际使用点守卫、token 凭据 POST 禁止重定向、Setup/管理端保存/导入/锁死保护；新增隔离端点自动化证据；代理维持 D-F06-2，真实 IdP 登录待隔离环境人工验收。 |
| v2.1 | 2026-09-15 | 按用户确认细化 R31-03 实施边界（固定发起 state 快照、旧 state 行保留并在回调拒绝、目标损坏最小读取与显式新 Secret 重填）；同步更新 R31-04 与 R31-03 的边界，避免重复设计空值保存/目标读取判定；记录 `SaveOidc` / `ClearOidc` 多键非事务写入的静态调用链、失败/并发窗口与 R31-07 故障注入要求；未实施代码。 |
| v2.2 | 2026-09-15 | 实施 R31-03：目标提供商读取与最小损坏重填、空 Secret 复用原子守卫、固定发起 state provider/config 指纹、设置页草稿/目标读取/警示与文案更新；补迁移、配置/服务/HTTP/前端隔离测试及全量门禁；真实 IdP 登录仍待隔离环境人工验收。 |
| v2.3 | 2026-09-15 | 按用户确认实施 R31-04（T1/S1/K1/SCOPE-A/TC-A）：六态 `params_state` 与 signing_key 故障优先只读状态、SaveOidc 全链单事务与严格密钥读取、管理端测试连接损坏/密钥故障专门失败、真实登录网络前拒绝、设置页统一展示及响应/日志脱敏；补隔离数据库/HTTP/前端测试与全量门禁，真实 IdP 登录待隔离环境人工验收。 |
| v2.4 | 2026-09-15 | 按用户确认实施 R31-05：`ValidateImportedAuthUsable` 校验 `oidc_configured` 与可解析回调地址，v2 注册任务前同步预检；`frontend_url`/独立 `callback_url` 保存即时生效，新增 `clear_callback_url` 显式清除回退；`oidc_states` 增加 `redirect_uri` 并固定发起值；Setup 不再写独立回调；清理地址“需重启”文案；补迁移、导入、地址、state、前端测试与全量门禁；真实 IdP/浏览器与跨 host 反代验收待隔离环境人工核验。 |
| v2.5 | 2026-09-15 | 按用户确认实施 R31-06：启动 mode 成为唯一运行模式依据；Production Setup/管理端保存与测试连接拒绝 mock；登录/绑定发起与回调再次拒绝且无新 state/会话；公开状态隐藏 mock、管理端只读警示；v1/v2 的 Setup/管理端导入遇到 mock 类型或 `oidc_params_mock` 键（含空值）整体拒绝且不写库；Dev mock 路径保持；按全新激活裁决不提供旧 mock 键清理兼容路径；补隔离测试与全量门禁，真实 IdP 与 Design5 整站恢复未验收。 |
| v2.6 | 2026-09-15 | 按用户确认实施 R31-07：新增 `oidc_flow_epoch` 与 `oidc_login_tickets.flow_hash`，流程指纹含 mode/provider/raw/代际；`DisableOidc`/`ClearOidc`/`SaveLocalAuth` 单事务，停用/清空同事务清理 state/ticket 并轮换代际；新增停用端点与 `enabled` 只读回显；登录/绑定/回调/模拟登录/会话签发/ticket 兑换统一事务守卫；设置页草稿、保存停用、保留 provider 与重新启用状态机落地；补故障注入、并发交错、迁移、HTTP、前端隔离测试与全量门禁；真实浏览器/隔离 IdP 人工验收待执行。 |
| v2.7 | 2026-09-15 | 按核验建议修复审计发现的 R31-05/07 缺口：导入认证可用性校验改用与运行时一致的 `configured` 布尔语义并校验真实 `provider_type` 白名单；设置页显式清除独立回调的草稿跨提供商切换保留；未保存停用草稿切回提供商时增加丢弃确认；补 OIDC 会话 JWT 停用后存活回归。新增 `TestR3105ValidateImportedAuthUsableBooleanAndProviderSemantics` 与前端组件回归；后端定向/race/全量/errgate、前端 `npm run test`（293 项）与 `npm run build` 重新通过；真实 IdP/浏览器/跨 host 反代人工验收仍未执行。 |
