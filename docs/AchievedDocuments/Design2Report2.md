# Design2 深度研究验证完成报告（Design2Report2）

> 研究对象：Design2.md 增量能力设计（规则素材池 / 装配拼接 / 配置生成与分发 / Xray 对接）与 Design1.md 基线的兼容性。
> 研究方式：十轮递进式研究（文档通读 → 代码取证 → 旅程模拟 → 兼容矩阵 → 边界排查 → 资料取证 → 外部证据强化）。
> 任务性质：纯研究分析任务（未修改任何代码文件与 Design1/Design2 文档）· 2026-08-16。

## 一、概览

| 指标 | 值 |
|------|-----|
| 研究轮次 | 10 |
| G 级缺口（G-1~G-4） | 4 |
| C 级边界问题 | 7 |
| D 级建议 | 3 |
| 兼容性验证通过项 | 24 |
| 文件修改 | 0 |

## 二、成就摘要

以十轮递进式研究对 Design2.md（411 行）与 Design1.md（926 行）进行全维度交叉验证：9 项代码扩展点全部吻合（ContentProvider 预留 / SwitchVersion 复用 / versions 独立主键 / 三态 Token 表兼容 / 生命周期触发点一一对应 / cron ticker 模式等），历史遗留问题（OIDC 触发点、迁移编号、Go 版本冲突等）全部复核闭合；同时新发现 4 处 G 级功能缺口、7 处 C 级边界问题，每项均给出问题描述、影响范围与修复方向。全链路（素材池→装配→入池→分发→用户下载→Xray 同步→配额管控）经 10 组场景模拟验证，除已识别的 G-1~G-4 外完全闭环。

## 三、关键步骤（研究轮次）

1. **第 1~2 轮：Design2 全文 + Design1 基线通读** — 建立基线与增量设计的对照框架：版本管理（5 版上限 / BEGIN IMMEDIATE / 不可删当前激活）、三态 Token 体系、级联删除规则、路径分级。
2. **第 3 轮：代码扩展点取证（14 文件 + 6 迁移 SQL）** — 确认 9 项扩展点全部吻合：ContentProvider 第三实现预留、SwitchVersion 复用、versions.id 独立主键支撑蓝图外键、users AUTOINCREMENT 无 email 碰撞、Register/CreateFromOidc/Approve/SetStatus/UpdateGroup 触发点均存在。
3. **第 4~7 轮：旅程模拟 / 兼容矩阵 / 边界排查** — 10 组场景模拟闭环验证；识别 G-1（动态重建空强制组）、G-2（OFF 后 proxy-groups 悬空引用）、G-3（平台产物类型歧义）与 C 级流程矛盾（规则空实体不可达等）。
4. **第 8 轮：数据清理 / 导出 / 下载渲染 / 前端取证** — 发现 G-4：dataclear 硬编码表清单未覆盖 9 张增量新表，全清后孤儿数据残留；确认 export.go FormatVersion=1、AssemblyView 占位页、下载端点渲染插入点。
5. **第 9 轮：Reference 4 份 + DocTemplates 5 份亲自取证** — Xray-Core-API §11.1~11.4 / Node-Link-Standards 第二四章引用全部有效；SR subs 样例实测空 cipher vless 形态一致；SSPanel「空节点 YAML 客户端导入失败」佐证 G-1；预置组库（🛟无法归属 / 🍻哔哩哔哩 / 🧩Steam下载）全部存在。
6. **第 10 轮：外部证据强化 + 增补报告** — mihomo 实机日志「proxy group: use or proxies missing」fatal + MetaCubeX Issue #2443 佐证 G-1/G-2 严重性；输出最终问题清单总表与研究完成审计。

## 四、G 级功能缺口（4 项，建议构建前落文）

| 编号 | 问题 | 影响范围 | 修复方向 |
|------|------|---------|---------|
| G-1 | 动态重建「强制组按约束保留」在注入集为空时产生空 select 强制组 → Clash 加载失败（与 3.3/4.1 空组硬校验自相矛盾） | 高级模式下组分配为空的所有用户下载 | 空强制组降级注入内置 DIRECT（与 rules 降级 DIRECT 口径统一） |
| G-2 | 开关 OFF 清空后激活模板 proxy-groups 悬空引用已删 xray 节点，「占位替换为注释」只保 YAML 语法、不保 Clash 语义 | 高级模式关闭后全部用户下载 | 动态重建统一应用于所有下载渲染（含基础模式/OFF） |
| G-3 | 平台产物类型归属歧义：「每平台一份全局模板」与「平台+产物类型唯一约束」矛盾，下载端点无产物类型维度 | 同平台双产物类型模板时的下载解析与装配目标选择 | platforms 增加 product_type 属性（推荐），唯一约束退化为 platform 唯一 |
| G-4 | 一键清空（dataclear）硬编码表清单未覆盖 9 张增量新表 → 全清后 rule_pools/nodes/xray_instances/proxy_groups 等孤儿数据残留 | 面板 RESET 与应急重新初始化共用 ClearTablesTx | ClearTablesTx 清单扩展全部增量表 + Design2 落文 |

## 五、C 级边界问题（7 项）与 D 级建议（3 项）

| 编号 | 类别 | 问题 | 修复方向 |
|------|------|------|---------|
| C-1 | 流程矛盾 | 规则创建必带首版 vs「空规则实体首个 conf 自动激活」条款不可达 | 允许空规则实体创建，或删除该自动激活条款 |
| C-2 | 边界 | reset-quota 对 disabled 用户的 AddUser 行为未定义（违背禁用语义） | 校验 status=active 才执行 AddUser |
| C-3 | 边界 | 开关 OFF 状态下业务钩子（换组/注册/审批）静默跳过未落文 | 钩子执行前统一检查高级模式开关 |
| C-4 | 边界 | 动态用量头与平台附加头同键冲突优先级未定义 | 动态头优先或保存时拒绝该键 |
| C-5 | 边界 | SR subs 产物默认文件名为 .yaml（defaultFileName 未按产物类型区分） | subs 默认 .txt 等按产物类型补扩展名 |
| C-6 | 措辞 | 「规则 Token 一键导入链接」与 Design1 3.5「移除一键导入 UI」矛盾 | 改为「复制链接」或明确恢复 scheme 唤起 |
| C-7 | 边界 | manual 与 xray 节点重名无约束 → Clash proxies 同名冲突 | 节点名唯一约束或装配勾选校验 |
| D-1 | UX | 高级模式首次配置顺序依赖（装配→激活→组分配）无引导，空候选集不可操作 | 组管理页空状态引导 |
| D-2 | 细节 | 5.10 改造清单未列 home.go updatedAt/groupSelected 的 group_selections 适配 | BuildN.md 列出 |
| D-3 | 细节 | 首页卡片状态枚举（group_selected/unassigned/admin_pool）变化未落文 | 并入 UI 规格补充 |

## 六、验证证据（24 项通过矩阵抽样）

| 关键论断 | 证据来源 | 状态 |
|---------|---------|------|
| ContentProvider 预留「装配生成」第三实现 | backend/internal/version/version.go L67-75 | 已核实 |
| CreateVersion 无 activate 参数（事务内强制 setCurrentLocked）→ 改造点准确 | backend/internal/version/version.go L123-176 | 已核实 |
| versions.id 独立主键 → assembly_blueprints 外键可行 | migrations/1002 L16 | 已核实 |
| users.id AUTOINCREMENT → email=user-{id}@vpn.local 无复用碰撞 | migrations/0002 L2 | 已核实 |
| 三态 Token 表 subscription_id 列保留、不再新发 → 落地处置一致 | migrations/1004 L10 | 已核实 |
| download.go 两处 JOIN group_selections → 渲染注入替换位明确 | backend/internal/download/download.go L96-100/L184-188 | 已核实 |
| 生命周期触发点方法全部存在（Register/CreateFromOidc/Approve/SetStatus/UpdateGroup） | user.go L84、user/oidc.go L70、approval.go L93/125、admin.go L239/411 | 已核实 |
| cron ticker 模式可复用素材池定时同步 | backend/internal/cron/cleanup.go L12-29 | 已核实 |
| AssemblyView 占位页 + 侧边栏「订阅装配」入口 + 路由存在 | frontend/src/views/admin/AssemblyView.vue、AdminLayout.vue L53、router L36 | 已核实 |
| export.go FormatVersion=1 → 决策 #24 升级 format_version=2 准确 | backend/internal/config/export.go L29 | 已核实 |
| dataclear 硬编码表清单（16 表）不含增量新表 → G-4 | backend/internal/dataclear/dataclear.go L40-45 | 已核实 |
| Xray-Core-API §11.1~11.4 引用有效且结论一致 | docs/Reference/Xray-Core-API.md L107-153 | 已核实 |
| SR subs 样例实测空 cipher vless（base64 首字符 :） | docs/DocTemplates/Shadowrocket.subs.template.md L7 | 已核实 |
| 预置组库 🛟无法归属 / 🍻哔哩哔哩 / 🧩Steam下载 等全部存在 | docs/DocTemplates/Clash.yaml.template.md L142/168/230 | 已核实 |
| SSPanel「空节点 clash YAML 客户端导入会失败」佐证 G-1 | docs/Reference/SSpanel-Subscribe.md L58 | 已核实 |
| mihomo 对空 proxy-group 直接 fatal（use or proxies missing） | 外部实机日志（right.com.cn）+ MetaCubeX Issue #2443 | 已核实 |

约束遵守：全程零文件写操作（仅 Read/Grep/Bash 取证），未修改任何代码文件与 Design1/Design2 文档；推断均以【推断】标注。

## 七、最终结论

全链路（素材池→装配→入池→分发→用户下载→Xray 同步→配额管控）在模拟场景中除 G-1~G-4 四处状态化缺口外完全可用、无逻辑断裂。G-1+G-2 可合并一处修复关闭（空强制组降级 DIRECT + 动态重建全场景适用）；G-3 建议 platforms 增加 product_type 属性；G-4 扩展 ClearTablesTx 表清单。C-1 需二选一决策，其余 C/D 级可随 BuildN.md 细化落文。全部断裂点已识别、证据充分、修复路径清晰。

> 任务标注：纯研究任务 · 无文件变更 · 无截图产物 · 报告基于 worktree 当前状态取证。
