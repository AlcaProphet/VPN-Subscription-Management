# ProdTestList.md — 当前人工核验清单

> 本文件跟踪基础模式待执行或待复验的 Production、浏览器和真实客户端人工项目。详细过程与证据见对应 Build／Issue。

## 当前人工核验项目

**截至 2026-11-24，本清单无待执行项。**

2026-11-24 用户确认基础模式服务器剩余流量的首版与多主机扩展均通过真实平台验收，搬瓦工和 V.PS 运行正常，无异常。[Build41 Step 7](docs/reports/Build/Build41.md#step-7--真实平台与出网验收) · [Build42 Step 2](docs/reports/Build/Build42.md#step-2--两家真实平台与目标容器)

Build32 Step 21 正式浏览器与 API 验收、Step 22 隔离候选 smoke 已通过。R34 双产物视觉复核已完成。[Build32.md](docs/reports/Build/Build32.md) · [Issue19.md](docs/reports/Issue/Issue19.md) · [Issue20.md](docs/reports/Issue/Issue20.md)

2026-10-24 用户豁免了 **PT-B32-17-02（TrustTunnel HTTP/2、QUIC 与复用模式）** 和 **PT-B32-18-02（OpenVPN 用户名密码、证书与组合认证）**。两项真实连接均未执行。[Build32 Step 22](docs/reports/Build/Build32.md#step-22联合门禁隔离-smoke-与归档)

Xray 人工核验见 [XrayRelated1.md](XrayRelated1.md)。当前发布记录见 [TODOLIST.md](TODOLIST.md#当前基线)。

## 变更记录

- 2026-11-24 登记用户确认的 Build41／Build42 真实平台验收结果。
- 2026-11-12 清理已完成项目及重复执行记录，保留验收和用户豁免的证据入口。
