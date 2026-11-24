# Build42 — 服务器剩余流量多主机绑定

> 2026-11-24。用户确认搬瓦工和 V.PS 均支持添加式追踪，并按主机配置余量折半。设计见 [Design7 §九](Design7.md#九多主机绑定与余量折半)，工程约束见 [AGENTS.md](AGENTS.md)。首版真实环境验收由 [Build41 Step 7](Build41.md#step-7--真实平台与出网验收) 跟踪。

## 一、构建进度

| Step | 内容 | 状态 |
| --- | --- | --- |
| 1 | 多主机迁移、采集、管理接口和页面 | 本地联合验收通过 |
| 2 | 从目标容器进行两家多主机实测 | 待验收 |

## 二、分步构建

### Step 1 — 多主机绑定与折半展示

- **目标**　两家均可按主机添加和操作。对已保存和新建的主机支持独立的「剩余流量折半显示」。
- **前置条件**　Build41 本地功能已验收，用户已确认两家多主机的范围。
- **产出**　[0003 迁移](backend/migrations/0003_provider_traffic_hosts.sql) 转换已有绑定，[存储](backend/internal/providertraffic/service.go) 管理主机和共享账号，[采集器](backend/internal/providertraffic/collector.go) 隔离各主机的状态，[管理接口](backend/internal/server/provider_traffic_hosts.go) 和 [管理页面](frontend/src/components/settings/ProviderTrafficSettings.vue) 提供主机操作，[首页流量卡片](frontend/src/components/ProviderTrafficCard.vue) 按主机展示。
- **参考流程**　根据主机标识和绑定修订采集，原始结果按绑定修订缓存。V.PS 先取得账号 Access Token，再按 Service ID 查询。返回成功结果时按主机开关计算 `remaining / 2`，前端保留两位小数。改名和切换折半直接读取同一份样本。
- **验收标准**　升级保留两个旧绑定的开关、密文和修订。可添加每家两台主机，拒绝同平台重复实例，关闭总开关时无新上游采集。两个 V.PS 主机共用当前 Token，只有一条续期在途，更新的 Refresh Token 先入库。逐台验证成功、失败和零余量，折半应用到设置、首页及手动采集。旧版及新版入口拒绝过期编辑修订，手动采集在排队后和发布前复核修订。V.PS 在发起续期的主机停用后仍可为同账号第二台主机复用已落库的轮换结果。配置导入、清空和整库快照覆盖新增表，导入确认提示需要重新绑定。
- **验证命令**

  ```sh
  cd backend
  go build ./...
  go vet ./...
  go test ./...
  go test -race ./internal/providertraffic ./internal/server ./internal/store -run 'TestCollector|TestProviderTraffic' -count=1
  go run ./cmd/errgate
  ```

  ```sh
  cd frontend
  npm run build
  npm test
  ```

- **独立核查与修复**　核查复现旧版保存和解绑接受过期修订、排队采集仍出站、V.PS 同账号重复续期，以及前端导入确认断言失败。已统一管理接口的编辑修订，补齐手动链的出站与发布复核，将已落库的轮换结果按账号缓存，并更新导入确认断言。问题与复现路径见 [Issue21 R35-23～R35-26](Issue21.md#r35-23-旧版接口接受主机的过期修订)。
- **本地复验**　修复后 Go 编译、vet、全量测试、服务器流量定向 race 和 errgate 通过。新增 HTTP 回归验证兼容接口的过期保存与解绑返回 `409`，当前修订可保存和解绑。采集回归覆盖排队期间修改、查询在途修改、过期和当前修订，以及第一台 V.PS 停用后第二台复用令牌，定向用例连续运行十次通过。前端生产构建及 53 个文件共 526 项测试通过。
- **隔离浏览器**　Production 进程以真实站内管理 API 保存两个搬瓦工主机和同一账号下的两个 V.PS 主机。Chrome 在 `1440×960` 和 `375×812` 下复验设置、逐台折半状态、首页模拟成功和失败并存及无横向溢出。采集与账号轮换使用合成服务商响应，实际平台仍待 Step 2 验收。

### Step 2 — 两家真实平台与目标容器

- **目标**　确认两家各有两台真实实例可独立采集，核对实际余量、折半和 V.PS 轮换。
- **前置条件**　Step 1 通过，有两台搬瓦工测试主机及同一 V.PS 账号下的两个 Service ID，目标容器可访问两家 API。
- **产出**　记录目标容器、各实例成功或失败、与面板对账及轮换后的同卷重启结果。
- **参考操作**　按 Build41 Step 7 在目标容器配置测试绑定。以 `docker port <候选容器> 8080/tcp` 核对端口，在设置页逐台采集并切换折半，同卷重启后复验 `docker restart <候选容器>`。
- **验收标准**　两个搬瓦工实例各自与面板对账。一个 V.PS 账号的 Access Token 可以读取两个 Service ID，轮换落库后重启能继续采集。开启折半的主机展示对应面板余量的 50%，保留单位，两位精度内一致。未取得的真实证据逐项记录。
