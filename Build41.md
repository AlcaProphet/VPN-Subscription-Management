# Build41 — 基础模式服务器剩余流量

> v1.1，2026-11-23。基线为 `beta` / `af20f8a545d9b7471ade4783c5f16265dda5c71b`，设计依据为已确认的 [Design7 v1.1](Design7.md)，工程约束见 [AGENTS.md](AGENTS.md)，构建格式参照 [Build 模板](docs/DocTemplates/Build.template.md)。第三节冻结实施合同，第四节规定串行步骤。Step 1～7 尚未实施。

## 一、进度与依赖

| Step | 产出 | 依据 | 状态 |
| --- | --- | --- | --- |
| 0 | 基线验证、环境准备和影响复核 | AGENTS §3.3、§3.5 | 已完成 |
| 1 | 功能表、加密绑定、迁移与清理 | Design7 §5.1、§六 | 未开始 |
| 2 | 两个固定上游客户端 | Design7 §二、§5.3 | 未开始 |
| 3 | 自动采集、内存结果与 V.PS 轮换 | Design7 §一、§5.3 | 未开始 |
| 4 | 首页及管理员 API、正常服务装配 | Design7 §5.2 | 未开始 |
| 5 | 设置和首页组件 | Design7 §三、§四 | 未开始 |
| 6 | 本地联合回归、隔离容器、浏览器与移除验证 | Design7 §六、§七 | 未开始 |
| 7 | 从目标容器按平台真实验收 | Design7 §七 | 待本地验收 |

依赖顺序为 Step 0 → 1 → 2 → 3 → 4 → 5 → 6 → 7。每一步记录所用命令、退出状态和测试范围，验收通过后进入下一步。真实验证分别记录搬瓦工与 V.PS 的结果。

## 二、Step 0 — 开工前基线

- **目标与前置。** 以已确认的 Design7 v1.1 为入口，检查当前工作区、工具链、增量迁移、会话权限、手动采集超时、配置导入和清表路径。基线提交为 `af20f8a`，Design7 为当前工作区新增文档。
- **准备结果。** 本机 Go `1.26.6`、Node `26.7.0`、npm `11.19.0` 可用，前端已安装依赖。Docker 客户端和服务端均为 `29.9.3`，Compose 为 `5.7.1`，Compose 配置校验通过。候选镜像继续使用当前 Dockerfile 的 Node 24 和 Go 1.26 构建阶段。
- **验收命令。** 仓库根目录执行以下检查，表中结论记录当前基线。

```bash
git status --short --branch
git rev-parse HEAD
(cd backend && go build ./... && go vet ./... && go test ./... -count=1)
(cd backend && go test -race ./internal/store ./internal/config ./internal/dataclear ./internal/server -count=1)
(cd backend && go run ./cmd/errgate ./...)
(cd frontend && npm test && npm run build)
docker version --format 'Client={{.Client.Version}} Server={{.Server.Version}}'
docker compose config --quiet
node scripts/check-md-links.mjs Design7.md
git diff --check
```

| 检查项 | 基线结果 |
| --- | --- |
| 后端 build、vet、全量测试 | 通过 |
| 存储、配置、清库及 HTTP 定向 race | 通过，4 个包 |
| errgate | 通过，0 项违规 |
| 前端全量测试 | 通过，51 文件、476 项 |
| 前端生产构建 | 通过，仍有既有 Vite 大分块提示 |
| Docker / Compose | 引擎可用，配置解析通过 |
| Design7 仓库链接与空白检查 | 通过 |

- **关键准备。** `0001` 的完整 schema 合同要保留，真实 `0001 → 0002` 单独验收。前端全局请求超时为 15 秒，V.PS 手动采集可连续发出两次各 8 秒的上游请求，按 Step 5 给该调用单独留出时间。整库备份可保留「已开启」，恢复到第二套站点时须在它首次启动前停用源站对应采集。 [首版 schema 合同](backend/internal/store/baseline_schema_test.go) · [Axios 默认超时](frontend/src/api/request.ts) · [备份实现](backend/internal/backup/backup.go)
- **验收标准。** 当前源码基线通过，测试工具和隔离容器环境可用。后续测试以临时 SQLite、模拟服务商响应及独立 Docker 数据卷为主。站内账号走正常 Setup 流程和仓库现有测试约定。实际 Key、Refresh Token 仅在测试站管理员界面输入。

## 三、冻结的实施合同

以下条款落实已确认的 Design7 v1.1，作为 Step 1～7 的验收依据。若发现真实上游与合同不一致，记录实际响应形态及受影响平台，按现有合同返回错误。变更功能范围、启用条件、凭据流转、数据迁移或接口合同须由用户确认后再实施，现有项目约束仍以 AGENTS.md 为准。

### 3.1 范围、常量与启用条件

| 项目 | 固定值 |
| --- | --- |
| 上游平台及展示顺序 | `kiwivm`（搬瓦工）→ `vps`（V.PS），每家最多一条绑定 |
| 启用默认值 | 总开关及每家开关均为关，缺少功能表记录等价于未绑定且关闭 |
| 采集及首页刷新 | 后端固定周期 `15 分钟`，首次及指定变更另触发首轮。首页每 `60 秒` 只读本站结果 |
| 超时及响应上限 | 上游单次 `8 秒`，单次上游响应体最大 `256 KiB`。前端只给「立即采集」指定 `25 秒` |
| 存储 | 一张独立 SQLite 功能表。每家的最近结果、V.PS Access Token 仅存进程内存 |

采集及首页展示的共同前置条件为正常服务、Setup 已完成、基础模式、总开关开、该平台开且绑定存在。服务端读取实时有效状态。高级模式以 `advanced_mode` 判定，Dev/Production 不影响此项的启用条件。Setup 与应急态不发起上游请求。关闭时下次首页读取返回相应空列表或移除该行，已打开的首页在下一次读取同步。总开关关闭时，基础模式管理员仍可保存或解除绑定。

固定常量可写在功能模块中。功能范围限于余量、采集时间和本次错误。禁止硬编码实例 ID 或真实凭据，禁止增加任意上游地址、配置环境变量、历史表、任务中心联动或服务商管理操作。原首页汇总、用户配额及 `traffic_card_enabled`、订阅、规则、Xray 与现有公开接口沿用现行行为。

### 3.2 数据、写入与清理

- `0002_provider_traffic.sql` 只新增 `provider_traffic_settings`。单行主键 `id=1`，总开关列 `enabled`。两家分别使用 `kiwivm_`、`vps_` 前缀，各存 `enabled`、`instance_id`、`credential_ciphertext`、`revision`。开关为 `0/1`。未绑定时三项绑定数据为空，单家开关为关。仅保存总开关可创建默认行。
- 实例 ID 以字符串入参传递，接受正数的规范十进制表示（首位 `1–9`，其余为 `0–9`）。首次绑定要求 `revision:null` 和凭据，修改或解除绑定要求匹配当前非空修订。更换实例必须提交新凭据。同一实例省略凭据字段表示保留，界面空输入按省略提交。接口显式传入空串、纯空白或 `null` 视为无效输入。开关值必须显式传布尔值，`false` 是有效输入。
- 保存新凭据生成从未使用的修订号，首次绑定亦如此。只切换开关或自动续期沿用现有修订。利用 `TxImmediate` 完成状态与修订校验、加密和写入。自动续期只允许按修订更新已有绑定，影响零行直接丢弃。解除绑定清除该家资料、开关和内存结果。V.PS 解除绑定同时清除其 Access Token。总开关关闭保留两家绑定与开关。
- 加密复用 `config.EncryptWithTx`。凭据不能进入配置文件导出、读取响应、错误正文、日志或浏览器持久存储。v1/v2 配置导入在覆盖事务内清除功能表数据，成功后总开关为关且未绑定，失败回滚时保留。全量清空用共用清表事务，旧库仅有 `0001` 时允许本表不存在。整库备份包含功能表、密文及原签名密钥。恢复到另一站点前，须在恢复容器首次启动前停用源站对应采集。
- `0001_initial_schema.sql` 不修改。原基线合同对仅含 `0001` 的集合执行，另外测试真实 `0001 → 0002`。后续卸载移除本功能的组件、路由、服务与清理入口，追加正向迁移删除本表，保留迁移记录。

### 3.3 上游协议与错误

| 平台和动作 | 固定请求 | 成功及计量 |
| --- | --- | --- |
| KiwiVM 读取 | `POST https://api.64clouds.com/v1/getServiceInfo`，`application/x-www-form-urlencoded`，请求体仅含 `veid`、`api_key` | HTTP `200` 且 `error=0`，`plan_monthly_data > 0`、`data_counter ≥ 0`。`monthly_data_multiplier=1` 时按 `max(0, plan_monthly_data - data_counter) / 2^30` 返回 `GiB` |
| V.PS 续期 | `POST https://vps.hosting/api/token`，JSON 请求体仅含 `refresh_token`，无需 Bearer | HTTP `200`，响应须有非空 `token` 与 `refresh` |
| V.PS 读取 | `GET https://vps.hosting/api/service/{id}/resources`，`Authorization: Bearer <access_token>` | HTTP `200`，取有限且非负的 `limit.data_combined` 原值，单位 `GB` |

数值字段区分缺失与零。允许 JSON 数值或只包含十进制数的字符串，拒绝单位、分隔符和非有限值。API 返回数值，界面统一保留两位；合法零显示 `0.00`。倍率缺失、异常或非 1 单独报告，既不假定为 1，也不采用推测公式。出站拒绝重定向，禁止改用查询参数承载凭据。每轮最多进行一次续期和一次资源请求，应用层不追加重试。上游原始正文留在客户端解析范围内，日志和 API 仅使用受控错误。

| 事件 | 首页及管理端通用短文案 | 管理端处理 |
| --- | --- | --- |
| Cloudflare `403`、连接失败或超时 | 无法连接服务商 | 核对部署出网 |
| 上游 `429` | 服务商限流 | 等待下次周期或稍后手动采集 |
| KiwiVM 倍率缺失、异常或非 1 | 计费倍率待核对 | 核对面板与官方口径 |
| 缺字段、非法值或无法解析 | 服务商返回的流量数据无法读取 | 核对服务商响应 |
| 上游明确拒绝 API Key 或 Refresh Token | 服务商凭据被拒绝 | 提示重新填写对应凭据 |
| V.PS 资源请求明确拒绝 Access Token | 服务商鉴权失败 | 作废 Access Token，下轮续期 |
| 上游成功签发，刷新凭据未能入库 | 服务器流量采集失败 | 续期结果未保存，可能需要重新绑定 |
| 其他非成功 HTTP 或业务结果 | 服务商返回错误 | 显示可安全归类的原因 |

上游 403 未明确属于凭据失效时按上游访问失败处理。管理员的操作提示不携带服务商原始正文或错误链。

### 3.4 采集与结果发布

1. 正常服务启动时采集当前符合条件的平台。总开关由关变开、已启用的绑定新建/更换实例/换证、单家由关变开，在写入提交后安排符合条件的平台采集。实例与开关均未变化且未提交新凭据时不重复触发。总开关只触发两家中已就绪者，修改一家只触发这一家。高级模式切回基础模式后最迟下一个固定周期恢复。页面读接口从不触发采集。
2. 自动和手动进入同一采集入口，每家同时最多一个上游请求链。固定周期触发遇同家忙碌时跳过本次，手动返回 `409`「正在采集，请稍后再试」。配置变更触发遇同家忙碌时，合并为一次待执行信号，当前请求结束后读取最新绑定及开关再决定是否采集。一家失败不阻止另一家。数据库事务仅覆盖本地读写，网络请求不持有写事务。
3. V.PS Access Token 只在同一绑定修订、未过期时缓存复用。以响应 JWT 的到期时间判断可复用性，无法取得有效到期时间时只用本轮。取得新的一对 Token 后，以该绑定修订写入新 Refresh Token 并提交，随后才可使用或缓存新 Access Token。已收到并解析有效的新 Token 对后，为这次本地入库保留独立的最长 `5 秒` 提交机会，浏览器取消请求不抹掉已收到的续期结果。
4. 每次出站前复核当前启用条件与绑定修订。已开始的请求允许结束。若收到续期响应时仅开关或模式已变化，仍可为相同绑定提交新 Refresh Token，当前不符合条件时停止后续请求并禁止发布采集结果。改绑、解绑、导入或清空后，旧修订不得写回凭据或发布结果。提交失败直接提示，不开始资源请求。Access Token 被拒绝后只清缓存，本轮直接失败。
5. 每家只有一个内存结果槽，绑定修订改变或进程重启后视为从未采集。相同绑定只切换开关时，最近一次结果可在重新启用后显示，成功结果始终带原采集时间。新失败覆盖旧数值，严禁从上一次成功回填。出现失败无需增加退避队列、历史或额外任务状态。

### 3.5 站内接口合同

所有成功响应使用现有 `{ "code": 0, "data": … }` 包裹。下表路径以 `/api` 为前缀，`:provider` 只允许 `kiwivm` 与 `vps`。配置写入与解除绑定采用 JSON 请求体。除手动采集外，设置返回只反映已提交本地状态，不等待上游。

| 路径 | 方法与请求体 | `data` |
| --- | --- | --- |
| `/home/provider-traffic` | `GET`，需要启用的登录会话 | `{ list: Result[], total: number }`。列表为空时返回 `[]`，`total=0` |
| `/admin/settings/provider-traffic` | `GET`，需管理员 | `Settings` |
| 同上 | `PUT`，`{ enabled: boolean }` | 保存后的 `Settings` |
| `/admin/settings/provider-traffic/:provider` | `PUT`，`{ enabled: boolean, instance_id: string, revision: string or null, credential?: string }` | 保存后的 `Settings` |
| 同上 | `DELETE`，`{ revision: string }` | 解除绑定后的 `Settings` |
| `/admin/settings/provider-traffic/:provider/refresh` | `POST`，无请求体，仅读当前已保存绑定 | 本轮 `Result`，包括上游返回错误时的 `error` 结果 |

- `Result` 的公共字段为 `provider`、`status`。`waiting` 无附加字段；`ok` 带 `remaining`（数字）、`unit`（`GiB` 或 `GB`）、`sampled_at`（UTC RFC3339）；`error` 带 `message`，省略 `remaining`、`unit` 和 `sampled_at`。
- `Settings` 含 `available`、`enabled`、`providers`。基础模式中 `providers` 固定按两家顺序返回，每项含 `provider`、`enabled`、`instance_id`、`revision`、`credential_configured`、`result`。未绑定的 `instance_id` 和 `revision` 为 `null`，`credential_configured=false`。无与当前绑定匹配的结果时 `result=null`。模式不可用时返回 `{ "available": false, "enabled": false, "providers": [] }`。管理端读取设置和立即采集返回的错误结果可附安全操作提示 `hint`。
- 站内 `401` 表示会话无效，`403` 表示无管理员权限或当前模式不允许设置写操作，`400` 用于输入错误或未知平台枚举，`409` 用于绑定修订冲突、采集忙碌或当前绑定未启用，`413` 沿用现有请求体限制，`500` 对本地读写故障返回通用正文。手动采集途中绑定被替换或因关停而丢弃结果时返回 `409`。上游故障走 HTTP `200` 加 `status:"error"`，不得透传上游 `401` 触发站内退出。
- 两组路由响应带 `Cache-Control: no-store`，权限中间件前设置。全局大小限制先行短路的 `413` 也要覆盖，仅在本功能路径补齐响应头。应急服务不注册本模块路由。首页接口不含实例 ID、绑定修订、凭据或管理操作提示。

### 3.6 界面与验收约束

- 设置只挂在现有「运行与安全」，总开关单独保存。单家未绑定、已停用、总开关关闭、当前平台有未保存草稿或采集忙碌时，禁用该家「立即采集」。同一家正在处理的请求不重复发送。凭据控件掩码显示且保存成功后清空。解除绑定复用现有确认弹窗。高级模式只显示不可用提示。
- 首页卡固定在顶部摘要区下方及订阅平台列表上方。`list=[]` 隐藏整个卡片。等待只显示「等待采集」，成功显示余量和本地化采集时间，失败显示「采集失败」和固定短文案。首页接口本身失败，仅在该区域显示「服务器余量读取失败」。页面离开清掉定时器，迟到响应不得影响新挂载的页面。
- Step 1～5 必须覆盖各自新增分支。Step 6 以完整候选验证权限、关停与并发、导入回滚、两个平台独立、浏览器与临时卷。Step 7 必须分别记录模拟通过、本地容器通过及真实上游通过的证据。V.PS 目标容器不可达时保留未验收并停用该家。KiwiVM 非 1 倍率保持明确提示。两家任一项未通过时，不把该平台标为已验收。

| 必验场景 | 通过判定 | 步骤 |
| --- | --- | --- |
| 新库、迁移库、导入成功 | 总开关为关，首页返回 `list=[]`，上游调用计数为 0 | 1、4、6 |
| 两家已启用，只改绑一家 | 仅该家安排采集，旧修订无权写回凭据或结果。若旧采集在途，结束后只为最新绑定采集一次 | 1、3 |
| V.PS 续期返回后关闭功能 | 同一绑定的新 Refresh Token 已提交，后续资源请求与结果发布均为 0 | 3 |
| V.PS 续期返回后入库失败 | 资源请求为 0，设置显示入库失败提示 | 3、5 |
| KiwiVM 成功，V.PS 曾成功但本轮失败 | KiwiVM 数值仍在。V.PS 行为 `error` 且无余量字段，下轮成功才恢复显示 | 3、4、5 |
| 用户会话有效，上游返回鉴权错误 | 站内 HTTP `200`、平台结果 `error`，浏览器仍保持登录 | 4、5 |
| 目标容器的 V.PS 出网被拦截 | V.PS 记录真实验收待解决且保持停用。KiwiVM 记录自己的独立结论 | 7 |

## 四、分步构建

### Step 1 — 存储、配置覆盖和旧库兼容

- **目标与前置。** Step 0 通过。建立独立、可清空的本地存储，保持首版数据库的结构与种子合同。
- **产出。** 新增 `backend/migrations/0002_provider_traffic.sql` 和 `backend/internal/providertraffic` 的存储代码。复核 [baseline_schema_test.go](backend/internal/store/baseline_schema_test.go)，在 [export.go](backend/internal/config/export.go) 的 v1/v2 覆盖事务和 [dataclear.go](backend/internal/dataclear/dataclear.go) 的共用清表入口接入功能表删除。正常导出沿用现有打包结构，整库备份沿用 SQLite 快照。
- **参考实现。** 单行表存总开关与两个固定平台的绑定。无记录视为关闭。用 `TxImmediate` 在同一事务读取、校验和更新，总开关只更新自身字段。实例 ID 使用正十进制数字字符串。首次绑定或换实例必填凭据，留空时只允许同一实例保留当前密文。改绑、换证产生不可复用的新修订号。自动续期按当前修订执行条件更新，影响零行表示原绑定已失效。加密复用 `EncryptWithTx`，解密沿用现有 AES-GCM 辅助方法。导入和清库成功后删掉功能行，回滚时保留原行。应急读取到仅有 `0001` 的库时，允许该功能表尚未建立。
- **验收命令。**

```bash
cd backend
go test ./internal/store ./internal/providertraffic ./internal/config ./internal/dataclear ./internal/emergency ./internal/backup -count=1
go build ./... && go vet ./...
```

- **验收标准。** 以仅含 `0001` 的迁移集合运行原首版表、种子和 manifest 断言。另测真实新库与已配置 `0001` 库迁移到 `0002`、重开幂等和总开关默认关闭。检查原业务数据保留、凭据加密、旧修订拒绝写回、导出包排除绑定、v1/v2 覆盖及回滚、RESET、应急旧库和整库快照是否包含加密绑定。

### Step 2 — KiwiVM 与 V.PS 上游客户端

- **目标与前置。** Step 1 通过。实现两家固定协议及直接错误提示，测试通过 mock transport 或本地 TLS 测试服务器运行。
- **产出。** `backend/internal/providertraffic` 内的 KiwiVM 和 V.PS 客户端、合成响应样本及定向测试。
- **参考实现。** KiwiVM `POST /v1/getServiceInfo` 使用表单 `veid`、`api_key`，要求 HTTP 200 且业务 `error=0`。倍率 1 时计算 `max(0, plan_monthly_data - data_counter) / 2^30`，其余倍率提示待核对。V.PS `POST /api/token` 的 JSON 只传 `refresh_token`，资源查询只走带 Bearer 的 `GET /api/service/{id}/resources`，从 `limit.data_combined` 取 `GB`。固定 HTTPS 主机和路径，拒绝重定向，单次上游请求上限 8 秒，响应体上限 256 KiB。只将固定错误类别和安全信息向上返回。
- **验收命令。**

```bash
cd backend
go test ./internal/providertraffic -count=1
go build ./... && go vet ./...
```

- **验收标准。** 捕获方法、路径及凭据位置的 mock 请求断言通过。覆盖成功真实零、数值缺失或非法、KiwiVM 业务失败与非 1 倍率、V.PS 续期字段、网关 403、上游 429、超时和错误响应。使用合成凭据断言响应和日志无明文。

### Step 3 — 自动采集与轮换一致性

- **目标与前置。** Step 2 通过。以同一入口执行定时和手动采集，每家最多一条在途请求链。
- **产出。** `backend/internal/providertraffic` 内的采集服务、固定周期循环、按绑定修订缓存的最近结果和 V.PS Access Token，以及并发测试。
- **参考流程。**

```text
启动 / 15 分钟定时 / 设置生效 / 手动触发
  → 校验 Setup、模式、总开关、单家开关与绑定修订
  → 每家占用一个采集机会
  → V.PS 若需续期，先以绑定修订加密提交新 Refresh Token
  → 再查当前模式与绑定状态，读取上游余量
  → 仅为仍启用且匹配绑定的记录发布本次结果
```

  总开关开启、新启用绑定和重新启用平台安排首轮。回到基础模式最迟在下一固定周期继续。一次失败仅更新该平台的最新错误，另一家继续执行。Access Token 被拒绝则清除内存缓存，下次再续期。已收到新 Refresh Token 时，为同一绑定保留有时限的本地提交机会，避免浏览器取消后丢掉已轮换的凭据。若功能已关闭，提交后结束本轮；若绑定已删除或替换，则丢弃旧结果。提交失败直接提示可能需要重新绑定。
- **验收命令。**

```bash
cd backend
go test -race ./internal/providertraffic -count=1
go build ./... && go vet ./...
```

- **验收标准。** 用可控触发和阻塞测试响应验证默认关闭时无上游请求、开启后首轮、固定周期、单家与双家、同家并发拒绝、成功转失败及恢复、已轮换后入库顺序、保存失败、关闭或模式变化、改绑、清空和重启。无需在用例中等待真实 15 分钟。

### Step 4 — 站内接口与生命周期

- **目标与前置。** Step 3 通过。正常服务装配独立 handler，让管理员配置并让首页只读本地结果。
- **产出。** `backend/internal/server/provider_traffic.go`、其路由测试和 [server.go](backend/internal/server/server.go) 的注册及运行收尾。复用现有会话与管理员中间件，以及现有 `no-store` 中间件。
- **参考实现。** 首页 `GET /api/home/provider-traffic` 返回 `list/total` 和 `waiting/ok/error`。管理员在 `/api/admin/settings/provider-traffic` 读状态或写总开关，在 `/:provider` 上保存或解除绑定，通过 `/:provider/refresh` 同步取得一次结果。修改与解除绑定提交当前修订，服务端只接受 `kiwivm`、`vps` 两个平台。未启用或模式不适用时首页返回空列表。管理员写操作检查实时模式，忙碌和修订冲突按现有 409 口径处理。上游拒绝凭据仍作为平台采集结果返回，站内 401 只代表会话问题。
- **验收命令。**

```bash
cd backend
go test ./internal/server ./internal/auth ./internal/setup -count=1
go test -race ./internal/server ./internal/providertraffic -count=1
go build ./... && go vet ./...
```

- **验收标准。** 管理员和普通用户看到各自权限范围内的数据，游客及停用账号无访问权限。高级模式、Setup、应急和二级开关组合按 §3.1 执行。首页读取过程中上游调用次数为零。按 §3.5 验证 JSON 字段、站内状态码与 `no-store`，包括全局体积限制直接返回的 `413`。验证安全的错误响应及调试日志，监听失败和收到退出信号时采集退出。

### Step 5 — 管理设置与首页展示

- **目标与前置。** Step 4 通过。新增两个独立前端组件，设置页可操作，普通用户首页只读。
- **产出。** `frontend/src/api/providerTraffic.ts`，设置与首页的独立组件，挂接 [SettingsView.vue](frontend/src/views/admin/SettingsView.vue) 的「运行与安全」和 [HomeView.vue](frontend/src/views/HomeView.vue) 顶部摘要下方。复用现有设置草稿提示及解除绑定确认，补充配置导入确认中的重新绑定文案。
- **参考实现。** 总开关单独保存，关停时仍可持久化各家凭据。密码框只提交用户输入的新值，提交成功清空。立即采集使用已保存绑定，按钮有忙碌反馈；仅该调用将 Axios 上限设为 25 秒，覆盖最慢两次上游请求和本地处理。首页进入时独立加载本地结果，之后每分钟读一次，离页停止。失败直接替换该行的数值为错误，订阅、规则和公告仍走各自加载逻辑。
- **验收命令。**

```bash
cd frontend
npm test -- home-view settings-view provider-traffic
npm run build
```

- **验收标准。** 模拟默认关闭、单家、双家、高级模式、首轮等待、真实零、单家失败、恢复、接口读取失败和手动慢响应。验证凭据输入与留空语义、手动采集只用已保存的绑定、旧修订提示、解除绑定后隐藏、计时器在离页后停止。测试定向组件及现有首页、设置页回归。

### Step 6 — 联合回归、隔离容器与卸载核查

- **目标与前置。** Step 1～5 通过。对完整候选验证原功能、运行环境和可拆卸边界。
- **产出。** 联合测试记录、隔离镜像及临时卷证据、浏览器核验结果和移除核查清单。验收完成后按仓库文档分工同步入口和验收状态。
- **参考操作。** 以独立 Docker 卷和回环端口启动候选镜像，完成空库 Setup 及默认关闭检查；用仅有 `0001` 的独立测试库验证升级后保留数据。浏览器验证管理员设置和普通用户首页，桌面及窄屏至少各一次。成功与失败场景用 mock API 验证，单独记录容器真实后端 smoke。检查卸载仅涉及独立组件、API、Go 模块、装配与导入/清库入口，后期删除表使用前向迁移。
- **验收命令。** 从仓库根目录执行。

```bash
(cd backend && go build ./... && go vet ./... && go test ./... -count=1)
(cd backend && go test -race ./internal/providertraffic ./internal/server ./internal/store ./internal/config ./internal/dataclear -count=1)
(cd backend && go run ./cmd/errgate ./...)
(cd frontend && npm test && npm run build)
docker compose config --quiet
docker build -t vpn-sub:build41-local .
node scripts/check-md-links.mjs Design7.md Build41.md
git diff --check
```

  镜像验收使用新命名的临时卷。以下起停命令可在同一 shell 中依次运行，中间保留容器供 API 与浏览器核验。

```bash
build41_id="vpn-build41-$(date +%s)-$$"
docker volume create "${build41_id}-data"
docker run -d --name "$build41_id" -p 127.0.0.1::8080 -e APP_MODE=prod \
  -v "${build41_id}-data:/data" vpn-sub:build41-local
docker port "$build41_id" 8080/tcp
# 完成 Setup、状态接口及浏览器核验后清理本轮容器和卷
docker rm -f "$build41_id"
docker volume rm "${build41_id}-data"
```

- **验收标准。** 全量和定向门禁通过，容器健康并由非 root 运行，干净库及迁移库均通过，原首页/设置行为可用。核对后续移除不修改既有业务表结构，保留迁移历史。浏览器截图和日志留存仅包含合成数据或脱敏结果。

### Step 7 — 真实平台与出网验收

- **目标与前置。** Step 6 通过。使用计划部署的容器出网条件，分别验证两家。两家独立启用，记录各自结论。
- **产出。** 平台、测试容器及出网位置、采集成功或失败原因、面板对账和轮换落库证据。使用测试绑定。备份恢复到另一台机器时，先停止源站对应采集，再首次启动目标容器。
- **参考操作和验收命令。** 在目标出网环境启动一份隔离的候选容器，将其容器名记为 `build41_target`。用 `docker port "$build41_target" 8080/tcp` 确认本轮容器映射。通过设置页输入测试凭据、保存并启动采集，用设置页及首页结果与官方控制面板对账。V.PS 先核验「续期 → 入库 → 查询」，再用 `docker restart "$build41_target"` 验证同一数据卷中的轮换凭据可在重启后继续采集。日志只以安全的状态、阶段和时间作为证据。
- **验收标准。** 搬瓦工与面板在显示精度内一致，遇非 1 倍率保留待核对结果。V.PS 必须从目标容器完成实际续期与资源查询并核对余量。研究中的浏览器成功和 mock 仅作为各自范围的证据。若目标容器仍收到 Cloudflare 403，记录 V.PS 外部验收待解决，保持该平台停用，搬瓦工按自己的真实验收结果记录和启用。

## 五、测试状态与变更记录

| 测试范围 | 当前状态 | 完成位置 |
| --- | --- | --- |
| 现有源码、本机工具链、Docker 引擎和配置解析 | 基线已通过 | Step 0 |
| 新表与升级、凭据与清理、上游 mock、采集并发和 API 权限 | 待实现后执行 | Step 1～4 |
| 页面定向回归、全量门禁、隔离容器与桌面/窄屏 | 待候选后执行 | Step 5～6 |
| 两家真实出网、面板对账、V.PS 轮换后的重启 | 待真实验收 | Step 7 |

| 版本 | 日期 | 记录 |
| --- | --- | --- |
| v1.0 | 2026-11-23 | 基于已确认的 Design7 v1.1 编制串行计划，记录开工基线、回归范围与分平台验收。 |
| v1.1 | 2026-11-23 | 冻结开关矩阵、表与修订语义、上游和站内接口、轮换时序、错误展示及验收断言。 |
