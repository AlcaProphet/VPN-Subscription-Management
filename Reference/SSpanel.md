# SSpanel.md — SSPanel-UIM 设计研究参考资料

> **文档定位：** 本文档是对成熟面板 SSPanel-UIM（PHP）的源码研究结论，作为 vpn-sub 项目 Xray 对接（高级模式）设计的参照资料。设计决策与方案见 [DesignOnHold.md](../DesignOnHold.md) 第三章；Xray-core API 研究见 [Xray-Core-API.md](./Xray-Core-API.md)。
> **核验来源：** 本地仓库 `/Users/kyle/Desktop/Repo/SSPanel-UIM` 源码（核心文件：`src/Services/Subscribe/*`、`src/Controllers/SubController.php`、`src/Models/Node.php`、`src/Models/User.php`、`src/Models/Link.php`）。

---

## 一、核心机制核验

1. **订阅生成模型**：SSPanel 为 9 种订阅格式（clash/json/sip008/singbox/v2rayjson/sip002/ss/v2ray/trojan）提供生成器，全部遵循同一模式：**全局基础模板 + 按用户过滤节点 + 注入用户凭据（uuid/port/passwd/method）**——与 vpn-sub「平台全局模板 + 组节点注入 + 用户 UUID」方案完全同构，验证了设计方向。
2. **Clash 生成细节**（`Subscribe/Clash.php`）：`Clash_Config` 为全局 YAML 基础配置（规则/策略组），`Clash_Group_Indexes` 指定需追加节点名的策略组索引，生成时按节点名追加 `proxies[]` 并 `yaml_emit` 结构合并——**采用「结构化合并」而非文本占位替换**（vpn-sub 采用 `# {{xray_nodes}}` 文本占位标记，更简单；结构合并更健壮，列为后续优化候选项）。
3. **用户节点可见性**（`Services/Subscribe.php getUserNodes`）：`type=1`（启用）+ `node_class <= user.class`（等级）+ `node_group in [0, user.node_group]`（分组，0=公共节点）+ 节点带宽未超限——**双维度（等级+分组）过滤**；vpn-sub「组→节点多对多 + is_public 公共标记」为单维度简化。
4. **用户凭据模型**（`Models/User.php`）：每用户固定 `uuid/port/passwd/method`（SS 按端口区分用户、vmess/trojan 按 UUID）；另有 `node_speedlimit`（用户限速，由后端实现）、`node_iplimit`（IP 数限制）、`class_expire`（到期）、`transfer_enable`（可用流量）。
5. **订阅 Token**（`Models/Link.php`）：每用户一条 token 记录——与 vpn-sub `download_tokens` 同构。
6. **流量模型**：`u/d` 字段 + `HourlyUsage`（小时粒度 JSON 数组）——SSPanel 为「节点主动上报」模式（push）；vpn-sub 为「面板定时拉取」模式（pull，由 Xray API 能力决定），存储可参考 HourlyUsage 聚合思想。
7. **Subscription-Userinfo 标准头**（`SubController.php`）：响应头返回 `upload=; download=; total=; expire=`——**客户端（Clash/SingBox/v2rayNG）原生展示流量信息的标准做法**，vpn-sub 已采纳（下载响应注入四字段）。

## 二、方案差距分析（SSPanel 对照 vpn-sub）

| 维度 | SSPanel | vpn-sub 方案 | 差距/启示 |
|------|---------|--------------|----------|
| 订阅内容 | 纯动态生成（模板来自环境变量，无版本管理） | 平台模板 + 版本管理（更可控） | vpn-sub 占优；模板来源（装配/上传）更灵活 |
| 用户凭据 | uuid/port/passwd/method | UUID（AES 加密落库） | 一致；vless/vmess 均以 UUID 为凭据 |
| 节点可见性 | 等级+分组双维度 | 组→节点多对多 + is_public 公共标记 | is_public 标记对所有组自动可见（已定稿） |
| 流量获取 | 节点上报（push） | 面板拉取（pull） | Xray API 仅支持 pull，方向已定 |
| 限额 | 总量制 + 自定义重置日 | 自然月制 + 手动重置 | 自然月已确认；SSPanel 重置日设计可作远期参考 |
| 到期 | class_expire（标配） | 不纳入本期（users 表预留 expire_at） | 已确认不纳入 |
| 限速 | node_speedlimit（后端实现） | 不做（官方 Xray 无此能力） | 一致；Xray 侧仅 Reality LimitFallback 预留字段（未生效） |
| IP 数限制 | node_iplimit | 不做 | 已确认不做（Xray API 无此能力） |
| 用量展示 | Subscription-Userinfo 头 | UI 展示 + 响应头四字段 | 已采纳标准头 |
| 订阅限流 | IP+Token 双限流 | 已有下载限流 | 一致 |
| 流量倍率 | traffic_rate | 不需要 | 已确认不需要（1-5 台小规模，配额按实际流量计；traffic_rate 服务于商业计费） |

## 三、对 vpn-sub 设计的验证结论

- **每用户专属订阅**（全局模板 + 按用户过滤节点 + 注入凭据）：与 SSPanel 订阅生成模型同构
- **公共节点概念**：SSPanel `node_group=0` 为公共节点 → vpn-sub `is_public` 标记
- **流量聚合存储**：SSPanel HourlyUsage 小时粒度聚合 → vpn-sub `traffic_records` 按月聚合
- **订阅 Token**：两者同构（每用户/每资源一条 token 记录）

---

## 变更记录

| 日期 | 说明 |
|------|------|
| 2026-08-13 | 从 DesignOnHold.md（v1.2~v1.4）提取整理：SSPanel-UIM 源码研究结论（4.2）、方案差距分析（4.4） |
