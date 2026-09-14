# docs/Reference — 研究参考资料索引

> **文档定位：** 本目录存放从外部项目/生态/源码提取的研究参考资料，以及本地隔离测试资料。本目录不是 Design/Build/Issue 文档，不直接定义实现；内容用于后续设计、构建、核验与排查。
> **维护约定：** 文档交叉审核后已按主题归类并规范化命名。正式文档引用本目录时应使用下方当前文件名；`.qoder/repowiki` 等生成式快照不在本仓库手工链接同步范围内。

## 一、分类总览

| 类别 | 文档 | 内容与用途 |
|---|---|---|
| Clash Verge Rev / Clash 客户端 | [Clash-Subscription-Validation-Emoji-API.md](Clash-Subscription-Validation-Emoji-API.md) | Clash Verge Rev 订阅导入校验、格式/编码、Emoji/文件名处理、响应头与对外下载 API |
| Clash Verge Rev / Clash 客户端 | [Clash-Verge-Rev-Node-Parameters.md](Clash-Verge-Rev-Node-Parameters.md) | Clash 客户端节点/代理字段定义，供 manual 节点表单、协议注册表与 YAML 渲染参考 |
| Clash Verge Rev / Clash 客户端 | [Clash-Verge-Rev-Subscription-Assembly.md](Clash-Verge-Rev-Subscription-Assembly.md) | Clash Verge Rev 订阅装配、扩展覆盖、规则/代理组机制 |
| 节点链接 / URI | [Node-Link-Standards.md](Node-Link-Standards.md) | 节点分享链接标准、URI 生成/解析、往返陷阱与 Shadowrocket 兼容性 |
| Xray / Xray-examples | [Xray-Core-API.md](Xray-Core-API.md) | Xray-core gRPC API、Stats/Handler 行为、Account 结构与订阅规范 |
| Xray / Xray-examples | [Xray-Client-Config-Research.md](Xray-Client-Config-Research.md) | Xray 客户端 `client.jsonc` 配置字段取证与组合矩阵 |
| Xray / Xray-examples | [Xray-Server-Config-Research.md](Xray-Server-Config-Research.md) | Xray 服务端配置到客户端节点表单的转换边界与字段建议 |
| SSPanel-UIM | [SSPanel-Research.md](SSPanel-Research.md) | SSPanel-UIM 订阅分发、管理链路、方案差距与 Design2 对照 |
| SSPanel-UIM | [SSPanel-Node-Editor-Research.md](SSPanel-Node-Editor-Research.md) | SSPanel-UIM 后台节点管理/custom_config 字典，与 manual 节点编辑器边界对照 |
| 节点编辑器研究 | [Node-Editor-Research.md](Node-Editor-Research.md) | 节点编辑器分层、多目标适配、当前代码审计与改进方向（原 Design-Research + Improvement-Directions 合并） |
| 节点编辑器研究 | [Node-Editor-3xui-Xray-Research.md](Node-Editor-3xui-Xray-Research.md) | 3x-ui / Xray 客户端样例/项目节点处理对照，含 Build17-21 后 3x-ui 深度研究 |
| 节点编辑器研究 | [Node-Editor-Ecosystem-Research.md](Node-Editor-Ecosystem-Research.md) | Mihomo/CVR、sing-box、v2rayN/NekoBox、Sub-Store、Hiddify/Marzban 等生态补充研究 |
| 本地测试资料 | [TestPasswordList.md](TestPasswordList.md) | 仅供本地隔离环境使用的合成测试账号与密码清单 |

## 二、命名规范

- 文件名使用 `主题-来源/用途-类型.md` 形式，主题优先：`Clash-*`、`Xray-*`、`SSPanel-*`、`Node-Editor-*`、`Node-Link-*`。
- `TestPasswordList.md` 为本地测试资料，不属于外部生态研究。
- 文件内部首行标题与文件名保持一致，避免出现“文件名与标题不一致”的历史问题。
- 历史文件名（如 `Xray-Client-Config-Research.md`、`SSPanel-Research.md`、`Node-Editor-...-2.md`）已在正式文档链接中更新；如需追溯历史可查看 `git log`。

## 三、与其他目录的关系

- 设计文档：[Design2.md](../reports/Design/Design2.md)、[Design3.md](../reports/Design/Design3.md)、[Design4.md](../../Design4.md)
- 构建/问题/验收记录：[docs/reports/](../reports)、根目录 Build/Issue/ProdTestList
- 文档模板与样例：[docs/DocTemplates/](../DocTemplates)
