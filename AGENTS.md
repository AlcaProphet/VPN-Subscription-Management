# AGENTS.md — VPN 订阅管理系统 AI 编码指令

> 本文档是给 AI 编码助手的指令集，也是项目**唯一的强要求文档**（详见「八、文档体系与优先级」）。
> 当前设计：[Design2.md](Design2.md) 与 [Design2-UI.md](Design2-UI.md) 为**当前最新的设计文档**（增量能力：订阅装配与 Xray 对接，已定稿并已构建验收）；[Design1.md](docs/reports/Design/Design1.md) 为第一期基线（已构建完成，存档）。当前构建与修复记录：[Build12.md](Build12.md)（UI/浮层，已完成未归档）、[Build13.md](Build13.md)（R24-02/12/15，已完成）、[Build14.md](Build14.md)（R24 第二批 UI 修复，已完成）、[Build15.md](Build15.md)（R24-19/20 节点表单结构化修复，已完成）；[Build11.md](Build11.md) 主体已完成，原后端死锁已由 R24-01 修复，尚未归档；已归档构建：[Build8.md](docs/reports/Build/Build8.md)～[Build10.md](docs/reports/Build/Build10.md)（均已验收存档）；当前问题记录：[Issue9.md](Issue9.md)（进行中）；历史问题记录已归档：[Issue5.md](docs/reports/Issue/Issue5.md)～[Issue8.md](docs/reports/Issue/Issue8.md)（均已闭环存档）；历史文档（Design0/Design1/Design1-UI/DesignOnHold/Build1~7/Build6-2/Issue1~4 等）统一存档于 [docs/reports/](docs/reports) 下按类型归档，仅用于核查，不再用于构建。

---

## 一、项目基本信息

- **项目**：自托管 VPN 订阅管理系统（单容器 + SQLite，面向小团队）
- **后端**：Go 1.26，module `vpn-sub`，目录 `backend/`（Go 版本升级见 Build4 Step 0；xray-core 依赖引入见 Build6 Step 0，均为 Design2 §5.3 决策的构建落点）
- **前端**：Vue 3 + Vite + Tailwind CSS，目录 `frontend/`
- **部署**：Docker Compose 单服务，多阶段构建单镜像
- **文档定位与优先级**：编码前先阅读本文件（强要求）。当前设计见 [Design2.md](Design2.md) 与 [Design2-UI.md](Design2-UI.md)（增量能力，非强制），第一期基线见存档的 [Design1.md](docs/reports/Design/Design1.md)；当前主体构建与后续修复记录见 [Build11.md](Build11.md)、[Build12.md](Build12.md)、[Build13.md](Build13.md)、[Build14.md](Build14.md)、[Build15.md](Build15.md)（均已完成后尚未归档；Build11 原后端死锁已由 R24-01 修复），已归档构建见 [docs/reports/Build/](docs/reports/Build)；当前进行中问题见 [Issue9.md](Issue9.md)，历史问题记录已闭环存档（Issue1~8 见 [docs/reports/Issue/](docs/reports/Issue)）

---

## 二、核心编码原则

### 简单轻量化

- 功能做减法，不引入不必要的抽象和重型框架
- 后端单二进制 + 嵌入式存储，前端按需打包，项目自身资源零 CDN 运行时依赖
- 所有依赖可纯静态编译/打包，不依赖 CGO、不依赖外部数据库进程

### 不过度防御

- 合理假设输入有效，不堆叠冗余的 nil/空值检查链与 fallback
- 错误处理到位即可，不添加无意义的兜底分支
- 网络安全边界由部署者控制，不为极端边界堆叠防御逻辑

### 零配置启动

- `docker compose up -d` 即可启动，首次启动即进入 Setup 引导
- 业务配置一律通过 Web UI 完成并存入数据库，不使用 .env 文件
- 仅监听端口、日志级别、日志格式等运维参数可通过环境变量覆盖

### 安全底线不妥协

- 路径穿越防护、密钥加密存储、Token 日志脱敏、实时权限校验、下载防缓存，五项缺一不可（详见「四、工程约束」）

### 数据一致性

- 版本创建/切换必须并发安全（事务 + 写锁）
- 级联删除保证无孤儿数据、无悬空引用

### 不确定时主动提问

- 遇到模糊需求、多种可行方案或技术取舍时，优先使用 `ask_user_question` 内置工具询问用户
- 提问时**必须附上推荐选项**并简要说明理由
- 不要在假设下自行决定关键设计（详见「三、AI 行为准则」）

---

## 三、AI 行为准则

### 3.1 不确定时必须提问

- 遇到模糊需求、多种可行方案、技术取舍或可能影响现有功能的情况时，**禁止自行决策**
- 必须优先使用 `ask_user_question` 内置工具与用户确认，并附上详细说明、各方案对比及推荐方案（含推荐理由）

### 3.2 无明确指令不得自行改动

- 没有收到用户明确指令时，不得自行修改文档或编写/改动代码

### 3.3 构建前必须检查疑点

- 开始代码构建前，必须执行文档或设计方案检查，确认没有需要用户决策的内容后才能开始构建
- 若存在疑点，禁止自行决策；必须按 3.1 与用户确认后再继续

### 3.4 构建后必须测试

- 完成代码构建后，必须执行编译验证和相关测试，确认通过后才能标记完成：
  - 后端：`cd backend && go build ./... && go vet ./...`
  - 前端：`cd frontend && npm run build`
  - 有单元测试时补充执行 `go test ./...`

### 3.5 变更前影响评估

- 进行设计内容调整或编写代码之前，必须先检查该变更是否会影响其他模块、文档或已有功能
- 若有影响，须明确列出受影响项并说明处理方式，再开始实施

### 3.6 文档同步

- 功能构建完成后，按文档体系分工同步更新对应文档（设计结论入 Design、构建记录入 Build、问题入 Issue）
- 文档间出现冲突时**提示用户并让用户决策**，不擅自选择遵守哪一份

### 3.7 内置工具使用优先级

#### 3.7.1 文件编辑：优先使用 str_replace_editor

- 在开始编码或修改任何文件/文档前，**先检查当前环境是否提供 `str_replace_editor` 工具**。
- 如果该工具可用，**必须优先使用 `str_replace_editor` 进行文件查看与编辑**，包括：
  - `view`：先查看目标文件内容与行号，确认精确上下文；
  - `create`：新建文件；
  - `str_replace`：用唯一、精确的 `old_str` 替换；
  - `insert`：在指定行后插入内容。
- **禁止优先使用 Python 脚本、sed/perl 等替代工具直接改文档或代码**，除非 `str_replace_editor` 不可用、无法处理该场景，或用户明确要求使用其他方式。
- 使用 `str_replace` 前必须先用 `view` 读取目标文件，确保 `old_str` 与文件内容完全一致；替换失败时应重新查看并调整上下文，不要盲目批量替换。
- 如果 `str_replace_editor` 不存在，再考虑其他可用工具，并尽量保持最小化改动。

#### 3.7.2 需要用户决策：优先使用 ask_user_question

- 遇到以下情况时，**先检查当前环境是否提供 `ask_user_question` 工具**：
  - 需求模糊、存在多种可行方案或技术取舍；
  - 偏离设计预期、与现有文档冲突，或涉及大方向决策；
  - 任何需要用户拍板才能继续的情况。
- 如果该工具可用，**必须使用 `ask_user_question` 与用户确认**，不要自行假设或擅自决定。
- 提问时**必须附上推荐方案**，并给出详细说明：
  - 说明背景、影响范围和可选方案；
  - 明确列出每个方案的优缺点/风险；
  - 将推荐选项放在首位并标注“(Recommended)/推荐”；
  - 等待用户回答后再继续实施。
- 如果 `ask_user_question` 不存在，再采用其他可用的提问/沟通方式，但不得在未确认关键决策前直接改动。

---

## 四、工程约束

### 4.1 输入与路径安全

- 所有含用户输入的路径操作必须经过路径穿越防护（禁止 `..` 与绝对路径逃逸）
- 资源标识统一格式校验（小写字母数字连字符）
- 文件上传大小限制前端 + 后端双重校验；大文件上传流式落盘，不整读内存

### 4.2 密钥与凭据

- 不可硬编码密钥；密钥自动生成后存数据库，敏感配置加密落库
- 业务配置（认证参数/密钥/限流等）一律通过 Web UI 存数据库，不用 .env

### 4.3 日志

- Logger 必须将 `?token=` 查询参数值脱敏为 `***`
- 日志输出到 stdout，支持分级（debug/info/warn/error）与 JSON/Console 双格式
- 5xx 内部错误默认脱敏返回通用信息，仅调试模式开启时返回详情

### 4.4 权限与认证

- 权限信息不入会话凭据，每次请求实时查库，不做缓存
- 所有管理端点必须叠加管理员校验中间件（会话校验 + 角色校验双层）
- 下载类公开端点仅凭 Token 校验，无效 Token 不得泄露资源存在性

### 4.5 下载与缓存

- 订阅/规则/分享等配置内容下载端点必须返回禁止缓存头（`no-store` 等）
- 静态可缓存资源（图片/安装包）走独立公开路径并返回可缓存头，两类路径严格分级

### 4.6 并发与事务

- 版本号计算与资源列表更新必须在单个事务 + 写锁内完成
- 指针类切换采用临时对象 + 原子替换，避免读到半切换状态
- Token/记录类更新基于业务键原子操作，禁止「先读后写」竞态模式

### 4.7 级联与清理

- 删除操作必须按设计文档的级联规则完整清理关联数据与文件，不留孤儿
- 文件写入类操作统一失败清理模式（任一步失败完整回滚清理）

### 4.8 错误码

- 400=校验错误，401=会话凭据缺失/无效/过期，403=权限不足，409=重复冲突，413=请求体过大，429=速率限制，500=服务器内部错误
- 成功操作用统一的成功响应结构；列表响应用统一包裹结构
- 下载类端点的业务错误（如组未分配）返回 HTTP 200 + 纯文本注释块（如 `# error: unassigned`），不返回 JSON/HTML；无效/过期 Token 仍统一返回 404（见 Design1.md 4.3）

---

## 五、代码规范

- **所有 error 必须处理**，不可忽略返回值
- 注释使用**中文**
- 日志使用结构化日志库，禁止散落的 `fmt.Println` 调试输出
- 职责分层：接入层只做协议解析与响应，业务层承载规则，数据层封装 SQL 与文件读写；接入层不直接操作存储，业务层不感知 HTTP
- 服务实例一律构造注入（结构体 Handler + 依赖传入），禁止包级全局变量持有服务实例
- HTTP 处理器按业务域拆分文件，每个文件独立管理依赖，禁止单文件集中全部处理器
- 多资源共用的机制（如版本管理）提取为共享事务组件，禁止复制粘贴重复实现
- 前端路由级代码分割，管理端子页面按需加载；重复页面逻辑抽取为通用组件复用

---

## 六、构建与验证命令

| 场景 | 命令 |
|------|------|
| 后端编译 | `cd backend && go build ./...` |
| 后端静态检查 | `cd backend && go vet ./...` |
| 后端测试 | `cd backend && go test ./...` |
| 前端构建 | `cd frontend && npm run build` |
| 前端开发 | `cd frontend && npm run dev` |
| 容器构建 | `docker compose build` |

- 每次代码改动后至少执行对应端的编译验证；涉及逻辑变更的必须补充相关测试执行

---

## 七、部署约束（容器化）

- 多阶段构建：前端打包 → 后端静态编译（CGO_ENABLED=0）→ 最小运行时，单镜像分发
- 单容器同时服务 API、静态资源与 SPA 回退，对外只暴露一个端口
- 非 root 用户运行；日志输出到 stdout
- 单一数据卷承载全部持久化数据（数据库 + 内容文件 + 静态资源）
- 数据库 schema 采用版本化迁移框架，启动时按版本号迁移
- compose 模板 restart 策略用 `unless-stopped`；healthcheck 仅作状态展示，不触发重启动作

---

## 八、文档体系与优先级

### 8.1 文档类型与定位

| 文档类型 | 文件 | 定位 | 约束力 |
|---------|------|------|--------|
| **强要求** | **AGENTS.md（本文件）** | AI 编码助手的强制指令集：编码原则、工程约束、操作规范、行为准则 | **唯一强要求，尽量不违背** |
| Design 文档 | [Design2.md](Design2.md)（增量能力，当前最新设计） | 面向人类的可读性描述文档，阐述设计思路、方案选型、产品功能与架构决策；第一期基线见存档的 Design1.md | 非强制，供参考 |
| Design GUI 规格 | [Design2-UI.md](Design2-UI.md)（当前最新 GUI 规格，承载 Design2 全部界面部件） | 受影响界面的 GUI 样式规格（布局/组件映射/状态分支/响应式）；功能行为以 Design2.md 为准 | 非强制，供参考 |
| Build 文档 | [Build1.md](docs/reports/Build/Build1.md)～[Build10.md](docs/reports/Build/Build10.md)、[Build6-2.md](docs/reports/Build/Build6-2.md)（已验收归档）；[Build11.md](Build11.md)（主体完成、后端死锁已由 R24-01 修复）、[Build12.md](Build12.md)、[Build13.md](Build13.md)、[Build14.md](Build14.md)、[Build15.md](Build15.md)（已完成，尚未归档） | 将 Design2 的设计转化为指导 AI 构建的构建手册，必须包含：分步 TODO LIST、构建参考代码/伪代码、每步的验收规范与验证命令 | 非强制，执行建议 |
| Issue 文档 | [Issue9.md](Issue9.md)（进行中）；[Issue1.md](docs/reports/Issue/Issue1.md)～[Issue8.md](docs/reports/Issue/Issue8.md)（全部已闭环归档） | 记录 bug 或改进项，含现象、根因、影响范围、修复方案、状态追踪 | 非强制，经验参考 |

**优先级**：AGENTS.md > Design > Build > Issue。

### 8.2 文档分工规则

- **AGENTS.md 不含具体设计内容**：产品定义、功能流程、数据模型、架构细节等设计信息全部留在 Design 文档中，本文件仅通过引用指向
- **Design 文档**面向人类阅读，描述「为什么这样设计」与最终设计结论；新设计构想经用户确认后落入 Design；当前设计为 Design2.md + Design2-UI.md（增量能力已定稿，构建时不得偏离），Design1.md 为第一期已建成功能基线（存档参考）
- **Build 文档**面向 AI 执行，每个 Step 必须含：目标、前置条件、产出文件与参考代码/伪代码、验收规范与验证命令；每次仅执行一个 Step，验收通过后再进入下一步
- **Issue 文档**只记录 bug 与修复闭环；非问题的优化候选归 Design 文档记录
- **归档规则**：构建验收完成或被后续文档取代的 Design/Build/Issue 文档移入 [docs/reports/](docs/reports)，仅作核查，不用于构建；研究参考资料存于 [docs/Reference/](docs/Reference)
- 新建文档参照 [docs/DocTemplates/](docs/DocTemplates) 中对应模板的结构与样式

### 8.3 执行规则

- 只有 **AGENTS.md** 是强要求文档，其他类型文档均为设计取向，不是强规则
- 若 Design / Build / Issue 文档之间存在冲突，或与 AGENTS.md 冲突：**提示用户并让用户做决策**，不擅自选择遵守哪一份
- 若设计内容本身存在冲突，同样**提示用户**，由用户决策

### 8.4 文档清单

| 文档 | 目标读者 | 内容 | 状态 |
|------|---------|------|------|
| AGENTS.md（本文件） | AI 编码助手 | 编码指令与约束（**唯一强要求**） | 活跃 |
| [Design2.md](Design2.md) | 人类（开发者/用户）与 AI 编码助手 | 当前最新增量设计：模式分层 / 规则素材池 / 装配拼接 / 配置生成与分发 / Xray 对接，已定稿并已构建 | 活跃 |
| [Design2-UI.md](Design2-UI.md) | 人类（开发者/用户）与 AI 编码助手 | 当前最新 GUI 规格：Design2 受影响界面的布局/组件/状态分支/交互契约（全量重写式自包含） | 活跃 |
| [Build4.md](docs/reports/Build/Build4.md) | AI 编码助手 | 第四轮构建：Go 1.26 + 1009 迁移 + 旧分发模型拆除 + 规则素材池（含节点表 display_name 列与有效渲染名唯一索引） | 已存档 |
| [Build5.md](docs/reports/Build/Build5.md) | AI 编码助手 | 第五轮构建：协议注册表/manual 节点与 display-name 服务（有效渲染名唯一：节点间 + 代理组/强制组/Clash-mihomo 内建保留代理名）、代理组、四类装配器渲染与分发 | 已存档 |
| [Build6.md](docs/reports/Build/Build6.md) | AI 编码助手 | 第六轮构建：xray-core 客户端、实例与节点检测（保留/校验 display_name）、组分配与候选集、用户同步、下载动态渲染（按有效渲染名） | 已存档 |
| [Build7.md](docs/reports/Build/Build7.md) | AI 编码助手 | 第七轮构建：对账/独立账号/导入导出 v2（含节点命名映射）/OFF 清空/高级 UI 收口 | 已存档 |
| [Build8.md](docs/reports/Build/Build8.md) | AI 编码助手 | 第八轮构建：Issue5 R20 系列修复（数据安全/错误处理/同步超时/前端与文档收口） | 已存档 |
| [Build9.md](docs/reports/Build/Build9.md) | AI 编码助手 | 第九轮构建（第一阶段）：R22 收口 / goccy YAML / 响应头语义 / 节点协议 / 规则全集 / 代理组扩展 | 已存档 |
| [Build10.md](docs/reports/Build/Build10.md) | AI 编码助手 | 第十轮构建（第二阶段）：分层覆盖层 / URI 批量导入 / 池启动补跑 / 原子写入 / 收口 | 已存档 |
| [Build11.md](Build11.md) | AI 编码助手 | 第十一轮构建：UI/UX 改进（可信状态/手机可操作/结构统一/视觉 Token/管理员概览） | 主体完成，后端死锁已由 R24-01 修复，尚未归档 |
| [Build12.md](Build12.md) | AI 编码助手 | 第十二轮构建：全量 gray/white Token 化、全局单浮层管理与统一焦点管理 | 已完成，尚未归档 |
| [Build13.md](Build13.md) | AI 编码助手 | 第十三轮构建：R24-02/12/15 已决策问题修复 | 已完成，尚未归档 |
| [Build14.md](Build14.md) | AI 编码助手 | 第十四轮构建：R24-09/10/11/13/14/16/17 已决策问题修复 | 已完成，尚未归档 |
| [Build15.md](Build15.md) | AI 编码助手 | 第十五轮构建：R24-19/20 节点表单分区、结构化对象编辑与递归校验 | 已完成，尚未归档 |

| [Issue1.md](docs/reports/Issue/Issue1.md) | AI 编码助手 / 开发者 | 问题记录：R1~R11 系列（首轮基础问题与修复） | 已存档 |

| [Issue5.md](docs/reports/Issue/Issue5.md) | AI 编码助手 / 开发者 | 问题记录：R20/R21 系列（Build1~7 + Issue1~4 双重核验新增） | 已存档 |
| [Issue6.md](docs/reports/Issue/Issue6.md) | AI 编码助手 / 开发者 | 问题记录：R21 UI/交互系列（素材池弹窗/节点表单/装配流程/死锁/Dev登录） | 已存档 |
| [Issue7.md](docs/reports/Issue/Issue7.md) | AI 编码助手 / 开发者 | 问题记录：R22 系列（Build9/Build10 前置修复与状态追踪） | 已存档 |
| [Issue8.md](docs/reports/Issue/Issue8.md) | AI 编码助手 / 开发者 | 问题记录：R23 系列（版本管理样式/首页分流规则卡片） | 已存档 |
| [Issue9.md](Issue9.md) | AI 编码助手 / 开发者 | 问题记录：R24 系列（Build11/12 全量核验与 UI/后端修复） | 进行中，R24-19/20 已修复 |

| [ProdTestList.md](ProdTestList.md) | 用户 / 测试者 | Production 模式待人工验证清单（含 .smoke-test.sh 实机执行） | 活跃 |
| [docs/DocTemplates/](docs/DocTemplates) | 开发者 | 四类文档的模板（AGENTS/Design/Build/Issue）与 Clash/Shadowrocket 配置参考样例 | 活跃 |
| [docs/Reference/](docs/Reference) | 开发者 | 研究参考资料：Xray-core API 研究、SSpanel 订阅输出逻辑 | 活跃 |
| [docs/reports/Design/Design1.md](docs/reports/Design/Design1.md) | 人类（开发者/用户） | 第一期设计基线：产品定义、角色权限、功能全景、核心机制、架构、安全、部署运维（已构建完成） | 已存档 |
| [docs/reports/Design/Design1-UI.md](docs/reports/Design/Design1-UI.md) | 人类（开发者/用户）与 AI 编码助手 | 已建界面 GUI 样式规格：13 个页面/部件；增量界面规格已由 Design2-UI.md 取代 | 已存档 |
| [docs/reports/Design/DesignOnHold.md](docs/reports/Design/DesignOnHold.md) | 开发者 | 增量设计源稿（含修订过程记录），内容已全量转入 Design2.md | 已存档 |
| [docs/reports/Build/](docs/reports/Build) | AI 编码助手 / 开发者 | 历史构建方案：Build1~10、Build6-2（均已验收/归档） | 已存档 |
| [docs/reports/Design/](docs/reports/Design) | 人类（开发者/用户）与 AI 编码助手 | 历史设计文档：Design0、Design1、Design1-UI、DesignOnHold | 已存档 |
| [docs/reports/Issue/](docs/reports/Issue) | AI 编码助手 / 开发者 | 历史问题追踪：Issue1~8（均已闭环归档） | 已存档 |
| [docs/reports/DesignReport/](docs/reports/DesignReport) | 人类（开发者/用户）与 AI 编码助手 | Design2 核验/研究报告：DesignReport1~10（原 Design2Report1~11，缺 6 已重新连续编号） | 已存档 |
| [docs/reports/BuildReport/](docs/reports/BuildReport) | AI 编码助手 / 开发者 | 构建验收/修复报告：BuildReport1 | 已存档 |
| [docs/reports/SecurityReport/](docs/reports/SecurityReport) | AI 编码助手 / 开发者 | 安全审计/修复报告：SecurityReport1 | 已存档 |
