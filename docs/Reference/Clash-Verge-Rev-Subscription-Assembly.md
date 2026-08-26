# Clash Verge Rev 订阅装配参考 — 订阅编辑/规则/代理组/扩展机制

> **文档定位：** 本文是 `clash-verge-rev` 客户端中“订阅（Profile）”大模块的源码级研究资料，供本项目的装配模块（Design2.md 第三~四章）借鉴。  
> **研究来源：** 本地仓库 `~/Desktop/Repo/clash-verge-rev` 的前端 TypeScript 与 Rust 源码，核心文件：
> - `src-tauri/src/config/prfitem.rs`：订阅条目模型、远程订阅下载与头部信息解析
> - `src-tauri/src/config/profiles.rs`：订阅列表持久化、更新、删除、选中节点记录
> - `src-tauri/src/enhance/merge.rs` / `seq.rs` / `mod.rs` / `script.rs`：最终配置装配管线
> - `src/components/profile/profile-viewer.tsx` / `groups-editor-viewer.tsx` / `rules-editor-viewer.tsx`：可视化编辑 UI
> - `src/types/global.d.ts`：节点、代理组、规则的类型定义
> - `src-tauri/src/core/timer.rs`：订阅定时更新
>
> **标注约定：** 【源码事实】=直接从源码得到；【推断】=由源码逻辑推出；【建议】=对当前 VPN 订阅管理项目的改进建议。

> **姊妹篇：** 节点各协议的完整字段定义见 [Clash-Verge-Rev-Node-Parameters.md](Clash-Verge-Rev-Node-Parameters.md)；订阅校验/格式/Emoji/API 见 [Clash-Subscription-Validation-Emoji-API.md](Clash-Subscription-Validation-Emoji-API.md)。

---

## 一、Clash Verge Rev 的“订阅”是什么

在 Clash Verge Rev 中，**订阅（Profile）** 是一份可以被“激活”的 Clash 配置来源，通常来自远程 URL 或本地 YAML 文件。

- 每条订阅是一个 `PrfItem`，持久化在 `profiles.yaml` 中。
- 订阅类型 `type` 不只是 `remote` / `local`，还有 `merge` / `script` / `rules` / `proxies` / `groups` 这些“扩展子项”。
- 每个远程/本地订阅可以挂载 5 个扩展子项，用来在原始订阅内容之上追加、删除或改写规则、节点、代理组、顶层配置，甚至执行 JS 脚本。
- 这一点就是本项目装配模块最值得借鉴的“分层装配”思路：**原始订阅内容 + 管理员可编辑的覆盖层，而不是只能从零生成静态产物。**

### 1.1 PrfItem 字段定义（`src-tauri/src/config/prfitem.rs`）

【源码事实】`PrfItem` 核心字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `uid` | `Option<String>` | 唯一 ID，远程自动生成 `R...`，本地 `L...`，merge/script/rules/proxies/groups 分别用 `m/s/r/p/g` 前缀 |
| `type` | `Option<String>` | `remote` / `local` / `merge` / `script` |
| `name` | `Option<String>` | 显示名 |
| `desc` | `Option<String>` | 描述 |
| `file` | `Option<String>` | 对应文件，如 `Rxxxx.yaml`、`mxxxx.yaml`、`pxxxx.yaml` |
| `url` | `Option<String>` | 远程订阅 URL |
| `selected` | `Option<Vec<PrfSelected>>` | 代理组当前选中的节点记录（`name`=组名，`now`=当前节点） |
| `extra` | `Option<PrfExtra>` | 订阅流量信息：`upload` / `download` / `total` / `expire` |
| `option` | `Option<PrfOption>` | 订阅更新参数与 5 个扩展子项 UID |
| `home` | `Option<String>` | 订阅主页（来自 `profile-web-page-url`） |
| `updated` | `Option<usize>` | 上次更新时间戳 |

`PrfExtra` 字段：`upload`、`download`、`total`、`expire`，全部为 `u64`。这些数据来自订阅响应头，可用于展示流量、到期时间。

### 1.2 PrfOption 字段定义

【源码事实】`PrfOption` 核心字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `user_agent` | `Option<String>` | 请求订阅时使用的 User-Agent |
| `with_proxy` | `Option<bool>` | 使用系统代理更新 |
| `self_proxy` | `Option<bool>` | 使用 Clash 内核代理更新 |
| `update_interval` | `Option<u64>` | 自动更新间隔（分钟） |
| `timeout_seconds` | `Option<u64>` | HTTP 请求超时（秒） |
| `danger_accept_invalid_certs` | `Option<bool>` | 允许无效证书（危险） |
| `allow_auto_update` | `Option<bool>` | 是否允许自动更新 |
| `merge` | `Option<String>` | 关联的 merge 子项 UID |
| `script` | `Option<String>` | 关联的 script 子项 UID |
| `rules` | `Option<String>` | 关联的 rules 子项 UID |
| `proxies` | `Option<String>` | 关联的 proxies 子项 UID |
| `groups` | `Option<String>` | 关联的 groups 子项 UID |

前端 `src/types/global.d.ts` 中的 `IProfileItem` / `IProfileOption` 与之一致。

### 1.3 订阅编辑表单字段（`src/components/profile/profile-viewer.tsx`）

【源码事实】新建/编辑订阅弹窗包含：

- 类型：`remote` / `local`
- 名称、描述
- 远程：订阅链接、User-Agent、HTTP 请求超时（秒）、更新间隔（分钟）
- 远程开关：使用系统代理、使用内核代理、允许无效证书（危险）、允许自动更新
- 本地：选择本地 YAML 文件

【源码事实】更新间隔下限为 `1440` 分钟（24 小时），见 `src-tauri/src/constants.rs` `MIN_UPDATE_INTERVAL`。前端也会对小于该值的间隔弹出警告。

### 1.4 远程订阅下载与校验（`prfitem.rs::from_url`）

【源码事实】下载远程订阅并入库的关键逻辑：

1. 解析并“修复脏 URL”：若 URL 无 query 但 path 中含 `&`，把 path 中的 `&param=value` 移入 query（`fix_dirty_url`）。
2. 按 `self_proxy` / `with_proxy` 选择网络出口：`self_proxy` 优先本地内核，`with_proxy` 使用系统代理，否则直连。
3. 读取 HTTP 响应头：
   - 解析带 `subscription-userinfo` 后缀的自定义头（兼容 `x-amz-meta-subscription-userinfo` 等）到 `PrfExtra`。
   - 读取 `Content-Disposition` 中的 `filename*` / `filename`，用于命名。
   - 读取 `profile-update-interval`，单位为小时，乘以 60 转为分钟。
   - 读取 `profile-web-page-url` 作为订阅主页。
4. 去除 UTF-8 BOM 后按 YAML 解析。
5. **硬校验：YAML 必须包含 `proxies` 或 `proxy-providers`，否则判定远程数据无效。**
6. 若未指定扩展子项，自动为每项创建独立的 `merge` / `script` / `rules` / `proxies` / `groups` 文件。

【建议】本项目目前是“管理员上传/编辑一份配置”，可以增加“远程订阅 URL 拉取 + 自动校验是否含 proxies/proxy-providers/规则集合”的能力，并把校验结果作为装配预览中的警告。

### 1.5 订阅更新重试策略（`src-tauri/src/feat/profile.rs`）

【源码事实】订阅更新失败时按以下顺序自动重试：

1. 按订阅配置的正常方式（直连 / 系统代理 / 内核代理）更新。
2. 失败后改用“Clash 内核代理” (`self_proxy=true`, `with_proxy=false`)。
3. 再失败后改用“系统代理” (`self_proxy=false`, `with_proxy=true`)。

【建议】当前项目的素材池同步、订阅拉取可参考该“代理退避”策略，尤其在服务器需要通过代理访问订阅源时。

### 1.6 定时更新（`src-tauri/src/core/timer.rs`）

【源码事实】定时更新由 `Timer` 管理：

- 仅远程订阅、`allow_auto_update=true`、`update_interval>0` 会被加入调度。
- 调度基于 `updated` 时间戳，到期立即执行。
- 更新间隔下限 1440 分钟。
- 初始化时会补跑“已经到期”的订阅。

---

## 二、扩展覆盖机制：Merge / Script / Rules / Proxies / Groups

这是 Clash Verge Rev 对“订阅装配”最重要的设计。原始订阅内容作为基线，通过 5 个扩展层做非破坏性增强。

### 2.1 五类扩展子项

| 扩展类型 | 文件 | 作用 | 编辑形态 |
|---------|------|------|---------|
| `merge` | `mxxxx.yaml` | 对最终 YAML 做深度合并，可覆盖任意顶层/嵌套配置 | 文本 YAML |
| `script` | `sxxxx.js` | 执行 JS，接收 `config` 和 `profileName`，返回新 config | JS 代码编辑器 + 运行日志 |
| `rules` | `rxxxx.yaml` | 对 `rules` 做 prepend / append / delete | 可视化规则编辑器 / 文本 |
| `proxies` | `pxxxx.yaml` | 对 `proxies` 做 prepend / append / delete，并可自动加入首个 selector 组 | 可视化节点编辑器 / 文本 |
| `groups` | `gxxxx.yaml` | 对 `proxy-groups` 做 prepend / append / delete | 可视化代理组编辑器 / 文本 |

【源码事实】模板见 `src-tauri/src/utils/tmpl.rs`：

```yaml
# rules / proxies / groups 共用结构
prepend: []
append: []
delete: []
```

### 2.2 Merge 深度合并（`src-tauri/src/enhance/merge.rs`）

【源码事实】`use_merge` 对配置做递归合并：

- 两边都是 Mapping 时递归合并；
- 否则以 merge 侧的值为准（覆盖）。
- 合并前会把 merge 配置的 key 转为小写，以兼容大小写写法。

【建议】本项目装配器可以增加“覆盖层”入口：管理员可以粘贴任意 Clash/Shadowrocket 顶层键（例如 `dns`、`tun`、`geox-url`、`profile`、`rule-providers`、`proxy-providers`），在最终生成产物上做深合并。

### 2.3 Rules / Proxies / Groups 的序列结构（`src-tauri/src/enhance/seq.rs`）

【源码事实】`SeqMap`：

```rust
pub struct SeqMap {
    pub prepend: Sequence,
    pub append: Sequence,
    pub delete: Vec<String>,
}
```

`use_seq` 的处理方式因字段而异：

- **rules**：最终 `rules` = `prepend` + 原 `rules`（删除 `delete` 命中的行）+ `append`。
- **proxy-groups**：最终 `proxy-groups` = `prepend` + 原 `proxy-groups`（删除命中的组）+ `append`。
- **proxies**：
  1. 先构造 `added_proxy_names`：从 prepend 和 append 的节点中收集 `name`。
  2. 最终 `proxies` = `prepend` + 原 `proxies`（删除 `delete` 命中的节点）+ `append`。
  3. **特殊规则**：如果存在 selector/select 类型的代理组，且其 `proxies` 列表不是空，则会把新添加的节点名称自动插入到第一个 selector 组的 `proxies` 最前面；其他组不自动插入。
  4. 删除节点时，也会从所有代理组的 `proxies` 中同步移除该节点名。

【建议】当前项目装配器目前只有“勾选节点/组/规则素材”，没有“prepend/append/delete”这种可叠加的差异层。可以实现一个类似的 `差异编辑` 数据结构，让管理员在生成的版本上追加节点、规则、代理组，并保留原始订阅内容作为基线。

### 2.4 Script 扩展（`src-tauri/src/enhance/script.rs`）

【源码事实】`script` 是一个 JS 主函数：

```js
function main(config, profileName) {
  return config;
}
```

- 运行环境是 `boa_engine`（Rust 嵌入式 JS）。
- `config` 会被序列化为 lower-case-key 的 JSON 传入。
- 支持 `console.log/info/error/debug/warn/table`，日志会在界面展示。
- 限制：最多 1000 条日志、日志总量 1MB、JSON 大小 10MB、循环 1000 万次、超时 5 秒。

【建议】本项目可以不引入 JS 引擎，但可以预留“自定义模板/脚本化后处理”的扩展点；若未来需要高度自定义，可参考这些安全限制（超时、输出上限、返回结构校验）。

### 2.5 最终装配管线（`src-tauri/src/enhance/mod.rs::enhance`）

【源码事实】最终配置按固定顺序组装：

1. **先应用序列扩展**：`rules` → `proxies` → `proxy-groups` 的 prepend/append/delete。
2. 合并 App 默认配置（监听端口、TUN、DNS、控制面等）。
3. 应用内建增强脚本（按内核类型选择）。
4. 应用 TUN 设置。
5. 应用 DNS 设置。
6. 捕获 App 权威控制面字段（例如 external-controller、端口、tun、mode 等）。
7. 应用 **全局 Merge/Script**。
8. 应用 **当前订阅的 Merge/Script**。
9. 恢复 App 权威控制面字段，避免被用户覆盖。
10. `cleanup_proxy_groups`：删除代理组中引用不存在节点/组/不存在的 provider 的条目。
11. `use_sort`：按固定顺序输出顶层字段（优先控制面，再默认字段，最后其余字段）。

【建议】当前项目装配模块“一次生成”后即固定，可以在蓝图里保存类似的“分层模板”：固定头部 + 节点/组/规则 + 覆盖层；下载重渲染时，先按覆盖层处理，再执行空引用清理。这样能避免用户在下载后手动修补失效引用。

---

## 三、规则（Rules）定义与编辑

### 3.1 规则编辑器的类型清单（`src/components/profile/rules-editor-viewer.tsx`）

【源码事实】Clash Verge Rev 支持的规则类型远超当前项目的 8 类：

| 规则类型 | 示例 | 说明 |
|---------|------|------|
| `DOMAIN` | `example.com` | 完整域名 |
| `DOMAIN-SUFFIX` | `example.com` | 域名后缀 |
| `DOMAIN-KEYWORD` | `example` | 域名关键字 |
| `DOMAIN-REGEX` | `example.*` | 域名正则 |
| `GEOSITE` | `youtube` | GeoSite 规则 |
| `GEOIP` | `CN` | IP 国家代码（通常 no-resolve） |
| `SRC-GEOIP` | `CN` | 来源 IP 国家 |
| `IP-ASN` | `13335` | IP 所属 ASN（通常 no-resolve） |
| `SRC-IP-ASN` | `9808` | 来源 IP ASN |
| `IP-CIDR` | `127.0.0.0/8` | IP 段（no-resolve） |
| `IP-CIDR6` | `2620:0:2d0:200::7/32` | IPv6 段（no-resolve） |
| `SRC-IP-CIDR` | `192.168.1.201/32` | 来源 IP 段 |
| `IP-SUFFIX` | `8.8.8.8/24` | IP 后缀 |
| `SRC-IP-SUFFIX` | `192.168.1.201/8` | 来源 IP 后缀 |
| `SRC-PORT` | `7777` | 来源端口 |
| `DST-PORT` | `80` | 目标端口 |
| `IN-PORT` | `7897` | 入站端口 |
| `DSCP` | `4` | DSCP 标记 |
| `PROCESS-NAME` | `chrome.exe` / `curl` | 进程名 |
| `PROCESS-PATH` | `/usr/bin/wget` | 完整进程路径 |
| `PROCESS-NAME-REGEX` | `.*telegram.*` | 进程名正则 |
| `PROCESS-PATH-REGEX` | `(?i).*Application\\chrome.*` | 进程路径正则 |
| `NETWORK` | `udp` / `tcp` | 网络类型 |
| `UID` | `1001` | Linux UID |
| `IN-TYPE` | `SOCKS/HTTP` | 入站类型 |
| `IN-USER` | `mihomo` | 入站用户名 |
| `IN-NAME` | `ss` | 入站名称 |
| `SUB-RULE` | `(NETWORK,tcp)` | 子规则 |
| `RULE-SET` | `providername` | 规则集引用（no-resolve） |
| `AND` | `((DOMAIN,baidu.com),(NETWORK,UDP))` | 逻辑与 |
| `OR` | `((NETWORK,UDP),(DOMAIN,baidu.com))` | 逻辑或 |
| `NOT` | `((DOMAIN,baidu.com))` | 逻辑非 |
| `MATCH` | 无 | 兜底匹配 |

【源码事实】前端校验器包括：
- `IP-CIDR` / `IP-CIDR6` / `SRC-IP-CIDR` / `IP-SUFFIX` / `SRC-IP-SUFFIX` 使用 `isValidIpCidr`。
- 端口类型使用 `portValidator`（1~65535）。
- `NETWORK` 必须是 `tcp` / `udp`。
- 数值类型（ASN、UID）使用 `+value` 真值校验。

### 3.2 规则行序列化格式

【源码事实】规则编辑器的输出格式为：

```text
TYPE,content,policy[,no-resolve]
```

其中：
- `policy` 可以是内置代理策略 `DIRECT` / `REJECT` / `REJECT-DROP` / `PASS`，也可以是当前配置中存在的代理组名。
- `no-resolve` 仅对声明了 `noResolve` 的类型在开关打开时追加。
- `MATCH` 不需要 content，输出形如 `MATCH,policy`。

【建议】当前项目的 `RuleLine` 和素材池只支持 `DOMAIN`、`DOMAIN-SUFFIX`、`DOMAIN-KEYWORD`、`IP-CIDR`、`IP-CIDR6`、`PROCESS-NAME`、`PROCESS-NAME-REGEX`、`USER-AGENT`。可考虑按 mihomo 白名单扩充规则类型，尤其是 `DOMAIN-REGEX`、`GEOIP`、`GEOSITE`、`RULE-SET`、`SRC-*`、端口类、`NETWORK`、逻辑组合类。至少要保证数据库/前端白名单与最终渲染一致。

### 3.3 规则扩展文件格式

【源码事实】`rules` 扩展文件保存为：

```yaml
prepend:
  - "DOMAIN,example.com,GroupName"
append:
  - "DOMAIN-SUFFIX,example.net,PROXY"
delete:
  - "DOMAIN,old.example.com,GroupName"
```

可视化编辑器右侧显示原始订阅中的规则，左侧可构建前置/后置规则并拖拽排序。这与当前项目“规则素材池 + 手动规则行”的形态不同：Clash Verge 是“在已有订阅规则上打补丁”，本项目是“从零拼接规则”。两种思路可以结合。

### 3.4 Proxy Provider / Rule Provider / Sub-Rule

【源码事实】Clash Verge Rev 并不单独管理 provider 的 CRUD，而是把 `proxy-providers`、`rule-providers`、`sub-rules` 作为原始订阅、当前订阅 Merge、全局 Merge 三处配置的合并结果：

- 代理组编辑时，读取原始订阅、当前 proxies/merge、全局 Merge 中的 `proxy-providers`，合并后作为“代理集合”下拉项，可填入代理组的 `use` 字段。
- 规则编辑时，读取原始订阅、当前 merge、全局 Merge 中的 `rule-providers` 和 `sub-rules`，用于 `RULE-SET` 和 `SUB-RULE` 类型的内容选择。
- `RULE-SET,providername` 可引用远程/本地规则集；`SUB-RULE,(NETWORK,tcp)` 可引用子规则。

【建议】当前项目的规则素材池可以视为“本地规则集”的简化版；将来若要覆盖更多 Clash 生态，可在装配器中增加 `proxy-providers` / `rule-providers` / `sub-rules` 的合并展示，并让代理组支持 `use` provider、规则支持 `RULE-SET` / `SUB-RULE`。

---

## 四、代理组（Proxy Groups）定义与参数

### 4.1 代理组类型

【源码事实】`src/types/global.d.ts` 的 `IProxyGroupConfig.type` 以及 `groups-editor-viewer.tsx` 支持：

| 类型 | 说明（前端中文文案） |
|------|------|
| `select` | 手动选择代理 |
| `url-test` | 根据 URL 测试延迟选择代理 |
| `fallback` | 不可用时切换到另一个代理 |
| `load-balance` | 根据负载均衡分配代理 |
| `relay` | 根据定义的代理链传递 |

当前项目仅支持 `select` / `url-test` / `fallback`，可考虑增加 `load-balance` 与 `relay`。

### 4.2 代理组完整字段（`IProxyGroupConfig` 与编辑表单）

【源码事实】支持字段：

| 字段 | 类型 | 默认/示例 | 说明 |
|------|------|-----------|------|
| `name` | string | 必填 | 组名，必填且不能重复 |
| `type` | enum | `select` | 组类型 |
| `icon` | string | - | 代理组图标 |
| `proxies` | string[] | - | 引入的节点 |
| `use` | string[] | - | 引入的代理集合（provider） |
| `url` | string | `http://cp.cloudflare.com/generate_204` | 健康检查测试地址 |
| `expected-status` | string | `*` | 期望状态码 |
| `interval` | number | `300` 秒 | 检查间隔 |
| `timeout` | number | `5000` 毫秒 | 超时 |
| `max-failed-times` | number | `5` | 最大失败次数 |
| `interface-name` | string | - | 出站接口 |
| `routing-mark` | number | - | 路由标记 |
| `filter` | string | - | 过滤节点（正则/表达式） |
| `exclude-filter` | string | - | 排除节点 |
| `exclude-type` | string | - | 排除节点类型，多个用 `|` 分隔 |
| `include-all` | bool | - | 引入所有出站代理、代理集合 |
| `include-all-proxies` | bool | - | 引入所有出站代理 |
| `include-all-providers` | bool | - | 引入所有代理集合 |
| `lazy` | bool | `true` | 懒惰状态 |
| `disable-udp` | bool | - | 禁用 UDP |
| `hidden` | bool | - | 隐藏代理组 |

【源码事实】`exclude-type` 可选值来自 `groups-editor-viewer.tsx`：

```text
Direct, Reject, RejectDrop, Compatible, Pass, Dns,
Shadowsocks, ShadowsocksR, Snell, Socks5, Http,
Vmess, Vless, Trojan, Hysteria, Hysteria2, WireGuard,
Tuic, Mieru, Masque, AnyTLS, Sudoku, Relay,
Selector, Fallback, URLTest, LoadBalance, Ssh
```

【建议】当前项目的 `proxy_groups` 只存 `type` 和子组引用，渲染时仅输出 `name/type/proxies`。可以扩展全局代理组定义，加入健康检查、负载均衡/relay、过滤、UD 开关、隐藏/图标等字段。这样装配器不仅能生成“能跑”的配置，还能生成更贴近实际客户端使用习惯的配置。

### 4.3 代理组扩展机制

【源码事实】`groups` 扩展文件与 rules/proxies 一样是 `prepend/append/delete` 结构：

```yaml
prepend:
  - name: MyGroup
    type: select
    proxies: [...]
append:
  - name: AnotherGroup
    type: url-test
    url: http://cp.cloudflare.com/generate_204
    interval: 300
delete:
  - OldGroupName
```

应用时：prepend 组在前，原配置中未删除的组居中，append 组在后。

【建议】当前项目可以增加“代理组覆盖层”，让管理员在不改原始订阅/基础装配的情况下，追加或删除代理组。

### 4.4 清理悬空引用

【源码事实】`enhance/mod.rs::cleanup_proxy_groups` 会：

- 收集所有合法代理名（`proxies[*].name`）、合法组名（`proxy-groups[*].name`）、合法 provider 名。
- 内置策略 `DIRECT` / `REJECT` / `REJECT-DROP` / `PASS` 永远合法。
- 从每个组的 `use` 中删除不存在的 provider。
- 从每个组的 `proxies` 中删除不存在的节点、组、provider 名。

当前项目的下载重渲染已经有类似的可达性收敛（`RenderClashPlan`），可以对照参考，把“不存在 provider”的清理也纳入。

---

## 五、最终产出建议（面向当前 VPN-Subscription-Management）

1. **增加“差异/覆盖层”模型**：在装配蓝图中保存 `prepend_rules`、`append_rules`、`delete_rules`、`prepend_proxies`、`append_proxies`、`delete_proxies`、`prepend_groups`、`append_groups`、`delete_groups`。生成时按 Clash Verge 的 `SeqMap` 语义合并，而不是只从空结构拼接。
2. **增加深合并覆盖层**：允许管理员粘贴 `merge` YAML，对最终产物做顶层/嵌套覆盖；对关键控制面字段（端口、mode、TUN 等）可由系统强制保护，避免订阅覆盖导致客户端不可用。
3. **扩充规则类型**：至少把 mihomo 常用规则类型纳入白名单；若保留 Shadowrocket 独有类型，继续在 Clash 渲染时跳过并提示。
4. **扩充代理组类型与字段**：支持 `load-balance`、`relay`，并为 `url-test` / `fallback` 渲染健康检查字段（`url`、`interval`、`timeout`、`max-failed-times`、`expected-status`），为所有组支持 `use`、`filter`、`include-all`、`disable-udp`、`hidden`、`icon` 等。
5. **保留“自动清理悬空引用”作为下载重渲染的最终步骤**：与现有 `RenderClashPlan` 结合，确保任何被删除的节点/组/provider 都不会留在最终产物中。
6. **订阅拉取增强**：可借鉴 `subscription-userinfo`、`profile-update-interval`、`profile-web-page-url`、`Content-Disposition` 的头部解析；用于远端订阅页或面板展示流量/到期/主页。
