# DesignReport1.md — Design2.md 增量能力设计深度研究验证报告

> **文档定位：** 本文档为 Design2.md（增量能力：规则素材池 / 装配拼接 / 配置生成与分发 / Xray 对接）的多轮多角度深度研究验证报告，由完成报告画布（design2-research-report.canvas.tsx）转为 Markdown 存档。研究针对 Design2.md 修订版（2026-08-16，410 行）与 Design1.md 基线（926 行）、现有代码及 docs/Reference 参考资料进行交叉验证。
> **研究约束**：全程只读——未修改任何代码文件（backend/、frontend/、配置文件）与 Design2.md/Design1.md；结论标注来源（【文档事实】/【代码事实】/【推断】）。
> **研究时间**：2026-08-16

---

## 一、执行摘要

**总体结论：设计成熟度高，无阻断级问题，但存在 3 个实质性缺口需在构建前处置。**

- 修订后的 Design2.md 上一轮 17 项问题已全部闭环落文（含 B1 Go 版本冲突的决策保持正确），代码扩展点 16/16 吻合，SR 双产物编码口径经样例实测与参考资料三源一致
- 本轮在更深层渲染语义、安全边界与配置迁移交叉场景发现：
  - 🔴 **G 级实质性缺口 3 项**：G-新1（proxy-groups 悬空引用）、G-新2（开关 OFF 不清理 Xray 侧账号）、G-新3（配置导入×业务密文）
  - 🟡 **C 级边界/一致性项 18 项**（Build 期需明确口径）
  - 🟢 **D 级运维/文档建议 9 项**
- 全流程模拟（素材池→装配→入池→分发→用户下载→Xray 同步→配额管控）在修复 G-新1/2/3 后完整闭环、无逻辑断裂

---

## 二、关键步骤（七轮递进研究）

| 轮次 | 维度 | 内容与产出 |
|------|------|-----------|
| 一 | 文档通读 | 精读 Design2.md（410 行修订版）与 Design1.md（926 行基线），核对上一轮 17 项问题落文状态——全部闭环，B1 Go 1.26 决策保持正确 |
| 二 | 代码取证 | 精读 10+ 核心文件（version/download/group/token/user/approval/oidc/config/cron/dataclear/export/server）+ grep 批量检索，扩展点吻合度核验 16 项 |
| 三 | 功能完整性与情景模拟 | 模拟管理员与用户 10 段旅程（部署→素材池→装配→分发→下载→换组→超限→重置），发现 G-新1（proxy-groups 悬空）与 G-新2（OFF 不清理 Xray 侧） |
| 四 | Design1 × Design2 兼容性 | 版本管理、Token 体系、级联删除、权限模型、路径分级五大核心机制逐项对照，识别 activate opt-in 改造、显式 Token 处置、home.go 改造等衔接点 |
| 五 | 冲突与边界排查 | 数据模型 × 现有代码匹配度、并发场景（BEGIN IMMEDIATE 串行化）、级联一致性（双侧 CASCADE）、开关切换数据清理完整性 |
| 六 | 补充设计探索 | UI 交互（Design1-UI 待补）、错误恢复（推送重试/采集告警/对账）、运维场景、配置迁移（format_version=2）完善建议 |
| 深化 | 交叉场景深化 | 聚焦配置迁移×密文、对账×配额拦截、装配×平台映射等交叉场景，新增发现 G-新3（配置导入密钥替换后业务密文不可解）与 C-新13~18 |

---

## 三、变更文件

**0 个文件变更**——严格禁止修改任何代码文件（backend/、frontend/、配置文件）与 Design2.md、Design1.md，已全程遵守。全部研究结论经 Read/Grep/Bash 只读取证，交付物仅为研究报告。

---

## 四、验证证据（16 项通过，精选 10 项）

| 类别 | 验证项 | 结果 |
|------|--------|------|
| 代码扩展点 | ContentProvider 接口预留装配生成第三种实现（[version.go](../../../backend/internal/version/version.go) L72-75） | ✅ 吻合 |
| 代码扩展点 | CreateVersion 无 activate 参数，改造点清晰有界（Design2 4.4） | ✅ 吻合 |
| 代码扩展点 | 下载解析两处 JOIN group_selections（[download.go](../../../backend/internal/download/download.go) L96-100/L185-188；5.6 删表后改造点吻合） | ✅ 吻合 |
| 代码扩展点 | withPlatformHeaders 附加头注入机制（[download.go](../../../backend/internal/download/download.go) L120-135，Subscription-Userinfo 融合点） | ✅ 吻合 |
| 代码扩展点 | 9 个生命周期钩子点全部存在（Register/CreateFromOidc/Approve/Reject/AdminService.Create/SetStatus/Delete/UpdateGroup/ChangeRole） | ✅ 吻合 |
| 代码扩展点 | HKDF-SHA256 派生 AES-256-GCM（[config.go](../../../backend/internal/config/config.go) L222-230，UUID 加密复用机制） | ✅ 吻合 |
| 代码扩展点 | cron ticker 模式（定时同步/流量采集复用）；迁移编号 1009 起（1001~1008 已占用）；export.go format_version=1→2 改造点 | ✅ 吻合 |
| 协议验证 | SR subs 样例实测：vless 空 cipher base64 userinfo（`Shadowrocket.subs.template.md` 解码验证）与 4.5/5.7 口径一致 | ✅ 一致 |
| 协议验证 | ss=SIP002 / vmess=V2rayN JSON / trojan 映射与 [Node-Link-Standards.md](../../Reference/Node-Link-Standards.md) 一致 | ✅ 一致 |
| 设计自洽 | 首次入池自动激活/候选集清理时机/组删除迁移链/审批拒绝幂等（RemoveUser not found 幂等成功），均闭环 | ✅ 自洽 |

---

## 五、发现清单

### 5.1 G 级：实质性功能缺口（3 项，修复后全流程闭环）

| 编号 | 问题描述 | 影响范围 | 建议修复方向 |
|------|---------|---------|-------------|
| **G-新1** | Clash YAML 模板含占位时 proxy-groups 引用候选集节点而 proxies 区为空；无凭据用户下载「模板其余部分原样」产生悬空引用（5.7 动态重建仅覆盖注入场景） | 无凭据用户 Clash/mihomo 加载失败；4.3「模板可独立校验」表述不成立 | 渲染层无条件重建 proxy-groups（空注入剔除节点组，rules 引用行降级 DIRECT）；修订 4.3 措辞 |
| **G-新2** | 高级开关 OFF 清空清单仅清面板数据（实例/节点/组分配/xray_users/traffic_records/UUID/代理密码/配额），未对可达 Xray 实例批量 RemoveUser | Xray 侧账号残留（旧凭据仍可连接）；重新 ON 批量初始化生成新 UUID 导致同实例新旧账号并存，配额管控从 OFF 起失效 | OFF 清空事务提交后按 5.7 实例删除同口径批量 RemoveUser（不可达跳过记因）；清空确认清单补充提示 |
| **G-新3** | 配置导入（已配置系统）签名密钥替换后，业务表密文（users.uuid_encrypted / users.proxy_secret_encrypted / nodes.protocol_json 凭据字段）不可解——Design1 3.4.8 导入整体覆盖 system_config 但不重加密业务密文，Design2 引入业务密文后成为新语义断裂点 | 向在用实例导入配置后：Xray 推送失败、下载注入失败、manual 节点渲染失败 | 三选一（需用户决策）：a) 导入时检测密钥变化+存在业务密文→拒绝并提示；b) 导入事务内清除业务凭据+置标记，引导重新生成/录入；c) 文档声明「配置导入仅适用全新部署或同机往返」，在用实例迁移走备份恢复路径 |

### 5.2 C 级：边界/一致性项（18 项，Build 期需明确口径）

| 编号 | 问题 |
|------|------|
| C-新1 | SR subs 模板存储形态与上传输入格式未显式定义（5.7 措辞倾向 base64 存储【推断】，需 Build 期显式统一） |
| C-新2 | subscriptions「平台+产物类型」唯一约束与「不做任何迁移」的张力（旧每平台多份数据破坏建约束，需在迁移中清空重建） |
| C-新3 | xray_users 表主键未定义（建议复合 PK user_id+instance_id+inbound_tag，支撑按实例级联删） |
| C-新4 | groups.needs_reselect 列处理未在 5.9 落文（5.6 删除项 vs 5.9 仅 +default_quota） |
| C-新5 | 公共节点与装配候选集交互：is_public 节点未勾选进候选集时静默不注入且组管理页无标注 |
| C-新6 | 5.10 实现清单遗漏 4 处：download 无标识 Token 解析改造 / home.go adminPool 显式 Token 不再新发 / dataclear.ClearTablesTx 表清单扩展新表 / 高级模式开关与流量卡片开关的配置键及端点归属 |
| C-新7 | 首页 platformCard「unassigned」状态语义未随单模板模型更新（应改为平台无激活版本口径） |
| C-新8 | default_quota / quota_override 为 NULL 或 0 的配额判定语义未定义（建议 NULL/0=不限流量跳过检查） |
| C-新9 | proxy-groups 重建的「已注入节点」集合构成未定义（须含 manual 静态节点，防纯 manual 组误剔除） |
| C-新10 | 换组/组删除迁移时新旧组公共节点先删后加（同凭据下短暂断连，可 diff 优化） |
| C-新11 | 开关 ON 批量初始化时组无节点分配的处置未定义（建议跳过不记失败） |
| C-新12 | 多平台候选集不一致（Clash 模板候选集 vs SR subs 模板候选集）时组管理页标注口径未定义 |
| C-新13 | xray_instances.name 唯一性未声明（节点名 {实例标识}-{入站tag} 冲突风险） |
| C-新14 | SR conf 规则实体与「分流规则」卡片映射未定义（多规则实体时展示哪个/哪些） |
| C-新15 | 装配器目标平台选择流程未落文（生成产物入哪个平台的订阅池） |
| C-新16 | 实例对账「面板有/实例无→补推」未复用超限前置拦截（quota_exceeded 用户被补推，绕过 G2 管控） |
| C-新17 | traffic_records 流量单位未定义（Subscription-Userinfo 头按字节、UI 按 GB） |
| C-新18 | 「Xray 侧已删除 inbound」的标记存储未定义（nodes 表无状态列；enabled 又被「检测不覆盖」保护） |

### 5.3 D 级：运维/文档建议（9 项，核心 6 项）

| 编号 | 建议 |
|------|------|
| D-新1 | 配置导入完成后引导：重新刷新节点检测 + 重配组分配 + 复用实例级对账端点 |
| D-新2 | 流量采集周期「可配置」的配置键位置随高级模式开关一并落文 |
| D-新3 | Design1 4.1/4.5「上传即激活」旧语义待构建完成后同步入基线（Design1 3.9 已声明，属已知待办） |
| D-新4 | 每平台一份全局模板的订阅名称命名规则未定义（影响下载文件名与 UI 展示） |
| D-新5 | traffic_records.ym 月界时区口径（建议显式声明按 UTC 计算） |
| D-新6 | Subscription-Userinfo 与平台自定义附加头的键冲突优先级（建议系统头覆盖或禁止同名键） |

---

## 六、最终结论与处置建议

### 6.1 构建前处置优先级

1. **G-新1 / G-新2 / G-新3**：建议在进入 Build 前落文修复——
   - G-新1 涉及渲染层核心语义（影响全部高级模式用户下载正确性）
   - G-新2 涉及安全边界（OFF 后 Xray 侧账号失控）
   - G-新3 涉及配置迁移正确性（密钥替换后业务密文不可解）
2. **C-新2 / C-新3 / C-新6**：影响迁移与实现清单完整性，随 Build 方案细化时明确
3. 其余 **C/D 级**：实现口径类，随对应 Build Step 落地时决策

### 6.2 闭环结论

模拟场景（素材池→装配→入池→分发→用户下载→Xray 同步→配额管控）在修复 G-新1/2/3 后**完整闭环、无逻辑断裂**。缺口修复需用户逐项决策后落文 Design2.md，属后续处置任务（不在本次只读研究范围内）。研究结论已存档记忆，可追溯。

---

## 七、研究约束与来源标注

- 本文档所有结论均标注来源：【文档事实】（Design1/Design2 原文）、【代码事实】（backend/frontend 源码行号）、【推断】（基于文档措辞的合理推断）、【实测】（模板样例解码验证）
- 与历史研究的关系：本文档基于上一轮《Design2.md 增量能力设计深度研究与验证报告》（17 项问题已闭环）与《Design2.md 审核报告问题全量处置与决策落地》的结论继续深化，聚焦修订后版本的渲染语义、安全边界与配置迁移交叉场景
