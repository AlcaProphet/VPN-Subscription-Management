---
kind: external_dependency
name: Xray-core 代理服务端（gRPC 管理面）
slug: xray-core
category: external_dependency
category_hints:
    - vendor_identity
    - sdk_real_api
    - client_constraint
scope:
    - '**'
source_files:
    - DesignOnHold.md
    - Reference/Xray-Core-API.md
---

### 角色与定位
本项目当前仅做订阅内容管理与分发；Xray-core 作为**独立进程/容器**运行在另一台服务器或同机回环地址上，通过 gRPC `commander` 暴露管理 API。本系统扮演「管理面」：用户生命周期同步、流量采集、配额超限摘除、在线状态查询。

### 集成方式（路径 A，已确认决策）
- 安全约束：Xray gRPC 无认证无 TLS，必须依赖 IP 白名单 + 回环/unix socket 部署。
- 并发限制：官方约第 10 个并发线程后丢弃请求，客户端需串行化/限并发。
- 统计前提：需在 Xray 侧开启 `policy.statsUserUplink/Downlink/Online`，否则统计接口返回空。

### 能力边界（源码核验结论）
- 支持：用户增删（`AddUserOperation` / `RemoveUserOperation`）、流量统计（`QueryStats` / `GetUsersStats`）、在线用户查询、系统状态。
- 不支持：带宽限速（per-user rate limit）、流量配额、到期时间——这些必须由面板侧实现为软限（采集差值 → 超限 → `RemoveUser` 摘除）。
- 幂等性：同 email 重复 Add 报 `already exists`、Remove 不存在报 `not found`，需按幂等成功处理。

### 数据模型衔接
新增 `xray_instances`（实例）、`xray_inbounds`（入站清单）、`xray_users`（用户↔Xray 账号映射，UUID 加密存储）、`traffic_records`（周期采集快照）四张表；装配器节点层可直接引用 `xray_inbounds` 渲染 Clash 节点行，下载时按 Token 对应用户替换 `{user_uuid}` 占位符。

### 待验证
具体 gRPC proto 方法名、参数结构以本地 `/Desktop/Repo/Xray-core` 源码为准。