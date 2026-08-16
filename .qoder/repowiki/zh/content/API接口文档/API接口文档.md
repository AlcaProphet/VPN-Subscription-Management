# API接口文档

<cite>
**本文引用的文件**
- [backend/internal/server/server.go](file://backend/internal/server/server.go)
- [backend/internal/server/auth.go](file://backend/internal/server/auth.go)
- [backend/internal/server/oidc.go](file://backend/internal/server/oidc.go)
- [backend/internal/server/log.go](file://backend/internal/server/log.go)
- [backend/internal/server/user.go](file://backend/internal/server/user.go)
- [backend/internal/server/subscription.go](file://backend/internal/server/subscription.go)
- [backend/internal/server/platform.go](file://backend/internal/server/platform.go)
- [backend/internal/server/group.go](file://backend/internal/server/group.go)
- [backend/internal/server/rule.go](file://backend/internal/server/rule.go)
- [backend/internal/server/download.go](file://backend/internal/server/download.go)
- [backend/internal/server/settings.go](file://backend/internal/server/settings.go)
- [backend/internal/server/custom.go](file://backend/internal/server/custom.go)
- [backend/internal/server/share.go](file://backend/internal/server/share.go)
- [backend/internal/server/home.go](file://backend/internal/server/home.go)
- [backend/internal/server/status.go](file://backend/internal/server/status.go)
- [backend/internal/user/user.go](file://backend/internal/user/user.go)
</cite>

## 更新摘要
**变更内容**
- 新增Xray实例管理API端点，支持动态节点检测与用户生命周期同步
- 增强订阅组装端点，支持异步激活模式与蓝图版本管理
- 改进用户生命周期同步API，增加并发首建守卫与配额超限拦截机制
- 完善高级模式开关控制，支持Xray集成功能的条件启用

## 目录
1. [简介](#简介)
2. [项目结构](#项目结构)
3. [核心组件](#核心组件)
4. [架构总览](#架构总览)
5. [详细端点说明](#详细端点说明)
6. [依赖关系分析](#依赖关系分析)
7. [性能与限流](#性能与限流)
8. [故障排查指南](#故障排查指南)
9. [结论](#结论)
10. [附录](#附录)

## 简介
本文件为VPN订阅管理系统的完整RESTful API参考，覆盖认证、OIDC、用户管理、平台与订阅、规则、分享与自定义订阅、下载、设置、日志、系统状态等全部对外接口。文档包含：
- HTTP方法与URL路径
- 请求参数（JSON表单/查询参数/文件上传）
- 响应格式与统一包装
- 状态码与错误处理
- 认证方式（会话Cookie、短期Token、OIDC回调）
- WebSocket/SSE实时日志连接协议
- 版本管理、速率限制与安全注意事项
- **新增**：Xray实例管理与用户生命周期同步API

## 项目结构
后端采用Gin路由装配，按业务域拆分处理器并集中注册：
- 认证与OIDC：/api/auth、/api/auth/oidc
- 管理员面板：/api/admin/*
- 用户端数据：/api/home、/api/rules
- 下载与预览：/subscriptions/*、/share/*、/rules/*、/api/subscriptions/preview
- 系统能力：/api/system/status、/api/public/announcement、/api/site/info
- 日志SSE：/api/admin/logs/stream
- **新增**：Xray实例管理：/api/admin/xray-instances/*

```mermaid
graph TB
A["HTTP入口<br/>server.New()"] --> B["认证路由<br/>/api/auth/*"]
A --> C["OIDC路由<br/>/api/auth/oidc/*"]
A --> D["管理员路由<br/>/api/admin/*"]
A --> E["用户端路由<br/>/api/home, /api/rules"]
A --> F["下载路由<br/>/subscriptions/*, /share/*, /rules/*"]
A --> G["系统状态<br/>/api/system/status, /api/public/*"]
A --> H["日志SSE<br/>/api/admin/logs/stream"]
A --> I["Xray实例管理<br/>/api/admin/xray-instances/*"]
```

图表来源
- [backend/internal/server/server.go:63-157](file://backend/internal/server/server.go#L63-L157)

章节来源
- [backend/internal/server/server.go:63-157](file://backend/internal/server/server.go#L63-L157)

## 核心组件
- 统一响应封装：OK/Fail/ListData
- 中间件链：请求日志、panic恢复、信任代理、限流、验证码、会话/管理员鉴权
- 服务装配：auth、setup、oidc、captcha、ratelimit、version、platform、subscription、group、token、download、custom、share、rule、mail、approval、config、log、backup、dataclear、emergency
- **新增**：Xray实例管理服务，支持节点检测与用户生命周期同步

章节来源
- [backend/internal/server/server.go:40-157](file://backend/internal/server/server.go#L40-L157)

## 架构总览
```mermaid
sequenceDiagram
participant Client as "客户端"
participant Gin as "Gin引擎"
participant MW as "中间件(限流/验证码/会话/管理员)"
participant Handler as "业务Handler"
participant Svc as "领域Service"
participant Xray as "Xray实例服务"
participant DB as "数据库/存储"
Client->>Gin : HTTP请求
Gin->>MW : 校验(限流/验证码/会话/管理员)
MW-->>Gin : 通过/拒绝
Gin->>Handler : 调用处理器
Handler->>Svc : 执行业务逻辑
Svc->>Xray : Xray实例操作(高级模式)
Xray-->>Svc : 节点检测结果
Svc->>DB : 读写数据
DB-->>Svc : 结果
Svc-->>Handler : 结果
Handler-->>Client : 统一响应(OK/Fail/ListData)
```

图表来源
- [backend/internal/server/server.go:63-157](file://backend/internal/server/server.go#L63-L157)
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)
- [backend/internal/server/settings.go:51-81](file://backend/internal/server/settings.go#L51-L81)

## 详细端点说明

### 通用约定
- 内容类型：默认application/json；文件上传使用multipart/form-data
- 统一响应：
  - 成功：{ data }
  - 列表：{ list, total }
  - 失败：{ error }
- 鉴权：
  - 会话Cookie：由Session中间件注入用户ID与角色
  - 短期Token：用于SSE连接（一次性）
  - OIDC：授权码流程+state Cookie保护
- 限流：按IP或Key计数，超限返回429
- 缓存控制：下载类端点强制no-store/no-cache
- **新增**：高级模式开关检查，Xray相关功能需advanced_mode配置开启

章节来源
- [backend/internal/server/server.go:40-50](file://backend/internal/server/server.go#L40-L50)
- [backend/internal/server/download.go:23-36](file://backend/internal/server/download.go#L23-L36)

### 认证接口
- POST /api/auth/register
  - 描述：本地注册（受本地登录开关与自注册策略控制）
  - 请求体：username、email、password
  - 响应：激活用户返回token、expires_at、status、is_admin；否则返回pending提示
  - 状态码：200/400/403/409/500
- POST /api/auth/login
  - 描述：本地登录
  - 请求体：email、password、remember
  - 响应：token、expires_at、user信息
  - 状态码：200/400/401/500
- GET /api/auth/me
  - 描述：当前用户信息（含group_name）
  - 鉴权：会话
  - 状态码：200/401
- POST /api/auth/logout
  - 描述：客户端清除本地会话
  - 鉴权：会话
  - 状态码：200
- POST /api/auth/forgot
  - 描述：发送密码重置邮件（防枚举）
  - 请求体：email
  - 状态码：200/400/500
- POST /api/auth/reset
  - 描述：重置密码（令牌校验）
  - 请求体：token、password
  - 状态码：200/400/500

章节来源
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)
- [backend/internal/server/auth.go:44-91](file://backend/internal/server/auth.go#L44-L91)
- [backend/internal/server/auth.go:99-134](file://backend/internal/server/auth.go#L99-L134)
- [backend/internal/server/auth.go:137-157](file://backend/internal/server/auth.go#L137-L157)
- [backend/internal/server/auth.go:159-196](file://backend/internal/server/auth.go#L159-L196)

### OIDC接口
- GET /api/auth/oidc/login
  - 描述：发起OIDC授权（302跳转），写入state Cookie
  - 状态码：302/500
- GET /api/auth/oidc/callback
  - 描述：OIDC回调（state三重校验、用后即删）
  - 成功：重定向至前端携带token或pending页
  - 状态码：302/500
- POST /api/auth/oidc/mock/login
  - 描述：开发模拟登录（Dev+mock）
  - 请求体：email、username、email_verified、roles、groups
  - 响应：token、expires_at、status或pending
  - 状态码：200/400/409/500
- POST /api/auth/oidc/bind
  - 描述：绑定OIDC到当前会话（需会话）
  - 响应：auth_url（供前端跳转）
  - 状态码：200/500
- POST /api/oidc/test
  - 描述：测试OIDC连接（不落库）
  - 请求体：provider_type、base_url、realm、client_id、client_secret
  - 状态码：200/400/500

章节来源
- [backend/internal/server/oidc.go:19-27](file://backend/internal/server/oidc.go#L19-L27)
- [backend/internal/server/oidc.go:31-40](file://backend/internal/server/oidc.go#L31-L40)
- [backend/internal/server/oidc.go:42-90](file://backend/internal/server/oidc.go#L42-L90)
- [backend/internal/server/oidc.go:92-125](file://backend/internal/server/oidc.go#L92-L125)
- [backend/internal/server/oidc.go:127-139](file://backend/internal/server/oidc.go#L127-L139)
- [backend/internal/server/oidc.go:141-163](file://backend/internal/server/oidc.go#L141-L163)

### 管理员-用户管理
- GET /api/admin/users
  - 描述：用户列表（分页、关键词搜索）
  - 查询：page、size、keyword
  - 响应：list、total
  - 状态码：200/500
- POST /api/admin/users
  - 描述：新建用户
  - 请求体：username、email、password
  - 状态码：200/400/409/500
- PUT /api/admin/users/:id
  - 描述：编辑/换组；可选补填email
  - 请求体：group_id、email
  - 状态码：200/400/404/500
- PUT /api/admin/users/:id/role
  - 描述：角色变更（admin↔user）
  - 请求体：role
  - 状态码：200/400/403/500
- POST /api/admin/users/:id/tokens/revoke
  - 描述：吊销所有下载Token
  - 状态码：200/500
- POST /api/admin/users/:id/password/reset
  - 描述：重置密码（send_email/direct）
  - 请求体：mode
  - 状态码：200/400/500
- DELETE /api/admin/users/:id/oidc
  - 描述：清除OIDC绑定（返回has_password）
  - 状态码：200/400/404/500
- PUT /api/admin/users/:id/status
  - 描述：禁用/启用（disabled）
  - 状态码：200/400/500
- DELETE /api/admin/users/:id
  - 描述：删除用户（级联）
  - 状态码：200/400/404/500
- POST /api/admin/users/send_password_links
  - 描述：批量发送密码设置链接（回执计数）
  - 状态码：200/400/500

**新增**：用户生命周期同步机制
- 自动触发：用户激活时生成UUID和代理密码，AES-256-GCM加密存储
- 并发守卫：BEGIN IMMEDIATE事务内条件更新，防止重复生成凭据
- 配额拦截：超限用户不推送Xray节点，需管理员重置配额后恢复
- 推送范围：所属组节点 ∪ 公共节点（is_public=1）

章节来源
- [backend/internal/server/user.go:19-32](file://backend/internal/server/user.go#L19-L32)
- [backend/internal/server/user.go:54-67](file://backend/internal/server/user.go#L54-L67)
- [backend/internal/server/user.go:69-89](file://backend/internal/server/user.go#L69-L89)
- [backend/internal/server/user.go:91-121](file://backend/internal/server/user.go#L91-L121)
- [backend/internal/server/user.go:123-146](file://backend/internal/server/user.go#L123-L146)
- [backend/internal/server/user.go:148-159](file://backend/internal/server/user.go#L148-L159)
- [backend/internal/server/user.go:161-196](file://backend/internal/server/user.go#L161-L196)
- [backend/internal/server/user.go:198-213](file://backend/internal/server/user.go#L198-L213)
- [backend/internal/server/user.go:215-238](file://backend/internal/server/user.go#L215-L238)
- [backend/internal/server/user.go:240-256](file://backend/internal/server/user.go#L240-L256)
- [backend/internal/server/user.go:258-274](file://backend/internal/server/user.go#L258-L274)

### 管理员-平台
- GET /api/admin/platforms
  - 描述：平台列表
  - 状态码：200/500
- POST /api/admin/platforms
  - 描述：创建平台
  - 请求体：name、description、schemes、extra_headers
  - 状态码：200/400/500
- GET /api/admin/platforms/:id
  - 描述：获取平台详情
  - 状态码：200/404/500
- PUT /api/admin/platforms/:id
  - 描述：更新平台（slug只读）
  - 状态码：200/400/404/500
- DELETE /api/admin/platforms/:id
  - 描述：删除平台（级联）
  - 状态码：200/404/500
- POST /api/admin/platforms/:id/installer
  - 描述：上传安装包（≤300MB）
  - 请求体：multipart file
  - 状态码：200/400/404/500
- DELETE /api/admin/platforms/:id/installer
  - 描述：删除安装包
  - 状态码：200/404/500

章节来源
- [backend/internal/server/platform.go:19-30](file://backend/internal/server/platform.go#L19-L30)
- [backend/internal/server/platform.go:50-57](file://backend/internal/server/platform.go#L50-L57)
- [backend/internal/server/platform.go:59-74](file://backend/internal/server/platform.go#L59-L74)
- [backend/internal/server/platform.go:76-92](file://backend/internal/server/platform.go#L76-L92)
- [backend/internal/server/platform.go:94-118](file://backend/internal/server/platform.go#L94-L118)
- [backend/internal/server/platform.go:120-135](file://backend/internal/server/platform.go#L120-L135)
- [backend/internal/server/platform.go:137-163](file://backend/internal/server/platform.go#L137-L163)
- [backend/internal/server/platform.go:165-180](file://backend/internal/server/platform.go#L165-L180)

### 管理员-订阅与版本
- GET /api/admin/subscriptions
  - 描述：订阅列表（含关联组与被选定标记）
  - 状态码：200/500
- POST /api/admin/subscriptions
  - 描述：创建订阅
  - 请求体：platform_id、name、slug（可选）、group_ids
  - 状态码：200/400/409/500
- PUT /api/admin/subscriptions/:id
  - 描述：更新订阅（名称、组）
  - 状态码：200/400/404/500
- DELETE /api/admin/subscriptions/:id
  - 描述：删除订阅
  - 状态码：200/404/500
- GET /api/admin/subscriptions/:id/versions
  - 描述：版本列表（含当前激活标记）
  - 状态码：200/500
- POST /api/admin/subscriptions/:id/versions
  - 描述：创建版本（文件上传或文本模式）
  - 查询：mode=upload|text
  - 请求体：文件或多行文本
  - 状态码：200/400/500
- PUT /api/admin/subscriptions/:id/versions/current
  - 描述：切换当前版本（原子）
  - 请求体：version_no
  - 状态码：200/400/404/500
- GET /api/admin/subscriptions/:id/versions/:ver/preview
  - 描述：预览版本内容（text/plain，no-store）
  - 状态码：200/404/500
- DELETE /api/admin/subscriptions/:id/versions/:ver
  - 描述：删除版本（不可删最后/当前）
  - 状态码：200/400/404/500
- GET /api/admin/slug/check
  - 描述：标识唯一性即时校验
  - 查询：slug、type、id
  - 状态码：200/500

**增强**：订阅组装端点升级
- 异步激活：支持opt-in激活模式，避免自动切换影响生产环境
- 蓝图版本：保存结构化渲染计划，支持重新编辑与悬空引用容错
- 首次激活：新订阅首个版本自动激活，避免空窗期
- 候选集管理：基于已激活蓝图构建全局节点候选集

章节来源
- [backend/internal/server/subscription.go:21-38](file://backend/internal/server/subscription.go#L21-L38)
- [backend/internal/server/subscription.go:56-63](file://backend/internal/server/subscription.go#L56-L63)
- [backend/internal/server/subscription.go:65-90](file://backend/internal/server/subscription.go#L65-L90)
- [backend/internal/server/subscription.go:92-115](file://backend/internal/server/subscription.go#L92-L115)
- [backend/internal/server/subscription.go:117-132](file://backend/internal/server/subscription.go#L117-L132)
- [backend/internal/server/subscription.go:134-148](file://backend/internal/server/subscription.go#L134-L148)
- [backend/internal/server/subscription.go:152-170](file://backend/internal/server/subscription.go#L152-L170)
- [backend/internal/server/subscription.go:174-192](file://backend/internal/server/subscription.go#L174-L192)
- [backend/internal/server/subscription.go:194-235](file://backend/internal/server/subscription.go#L194-L235)
- [backend/internal/server/subscription.go:237-260](file://backend/internal/server/subscription.go#L237-L260)
- [backend/internal/server/subscription.go:262-283](file://backend/internal/server/subscription.go#L262-L283)
- [backend/internal/server/subscription.go:285-309](file://backend/internal/server/subscription.go#L285-L309)

### 管理员-用户组
- GET /api/admin/groups
  - 描述：组列表（含关联订阅数、用户数、needs_reselect）
  - 状态码：200/500
- GET /api/admin/groups/:id
  - 描述：组详情（基础信息+每平台选定）
  - 状态码：200/404/500
- POST /api/admin/groups
  - 描述：创建组
  - 请求体：name
  - 状态码：200/400/409/500
- PUT /api/admin/groups/:id
  - 描述：改名+关联订阅+每平台选定（整体提交）
  - 请求体：name、sub_ids、selections
  - 状态码：200/400/404/500
- DELETE /api/admin/groups/:id
  - 描述：删除组（迁入默认组）
  - 状态码：200/400/404/500
- PUT /api/admin/groups/:id/selections
  - 描述：每平台选定（subscription_id=0取消）
  - 请求体：selections
  - 状态码：200/400/404/500

章节来源
- [backend/internal/server/group.go:18-27](file://backend/internal/server/group.go#L18-L27)
- [backend/internal/server/group.go:29-36](file://backend/internal/server/group.go#L29-L36)
- [backend/internal/server/group.go:38-60](file://backend/internal/server/group.go#L38-L60)
- [backend/internal/server/group.go:62-80](file://backend/internal/server/group.go#L62-L80)
- [backend/internal/server/group.go:82-117](file://backend/internal/server/group.go#L82-L117)
- [backend/internal/server/group.go:119-138](file://backend/internal/server/group.go#L119-L138)
- [backend/internal/server/group.go:140-167](file://backend/internal/server/group.go#L140-L167)

### 管理员-规则
- GET /api/admin/rules
  - 描述：规则列表
  - 状态码：200/500
- POST /api/admin/rules
  - 描述：创建规则（文件/文本双模式）
  - 查询：mode=upload|text
  - 请求体：name、slug（可选）、client_type、schemes、text或文件
  - 状态码：200/400/409/500
- PUT /api/admin/rules/:id
  - 描述：重命名
  - 请求体：name
  - 状态码：200/400/404/500
- DELETE /api/admin/rules/:id
  - 描述：删除
  - 状态码：200/404/500
- POST /api/admin/rules/:id/token/refresh
  - 描述：刷新全局Token
  - 状态码：200/500
- 版本端点：/api/admin/rules/:id/versions/*（同订阅通用模式）

章节来源
- [backend/internal/server/rule.go:22-40](file://backend/internal/server/rule.go#L22-L40)
- [backend/internal/server/rule.go:42-49](file://backend/internal/server/rule.go#L42-L49)
- [backend/internal/server/rule.go:51-105](file://backend/internal/server/rule.go#L51-L105)
- [backend/internal/server/rule.go:107-129](file://backend/internal/server/rule.go#L107-L129)
- [backend/internal/server/rule.go:131-146](file://backend/internal/server/rule.go#L131-L146)
- [backend/internal/server/rule.go:148-159](file://backend/internal/server/rule.go#L148-L159)
- [backend/internal/server/rule.go:186-206](file://backend/internal/server/rule.go#L186-L206)

### 用户端-规则
- GET /api/rules
  - 描述：规则卡片列表（含全局Token）
  - 鉴权：会话
  - 状态码：200/500
- GET /api/rules/:id/preview
  - 描述：预览当前版本（text/plain，no-store）
  - 鉴权：会话
  - 状态码：200/404/500

章节来源
- [backend/internal/server/rule.go:22-40](file://backend/internal/server/rule.go#L22-L40)
- [backend/internal/server/rule.go:161-169](file://backend/internal/server/rule.go#L161-L169)
- [backend/internal/server/rule.go:171-184](file://backend/internal/server/rule.go#L171-L184)

### 下载与预览
- GET /subscriptions/:platform/download?token=...
  - 描述：用户订阅下载（限流20/min，禁缓存）
  - 成功：返回配置内容（可带Content-Disposition）
  - 未分配：返回纯文本注释块（HTTP 200）
  - 无效Token/无版本：404
  - 状态码：200/404/500
- GET /share/:slug/download?token=...
  - 描述：分享订阅下载（公开，限流）
  - 状态码：200/404/500
- GET /rules/:slug/download?token=...
  - 描述：规则下载（公开，限流）
  - 状态码：200/404/500
- GET /api/subscriptions/preview
  - 描述：会话凭据预览（管理员可指定subscription_id）
  - 鉴权：会话
  - 状态码：200/404/500

章节来源
- [backend/internal/server/download.go:23-30](file://backend/internal/server/download.go#L23-L30)
- [backend/internal/server/download.go:38-69](file://backend/internal/server/download.go#L38-L69)
- [backend/internal/server/download.go:71-98](file://backend/internal/server/download.go#L71-L98)
- [backend/internal/server/download.go:100-127](file://backend/internal/server/download.go#L100-L127)
- [backend/internal/server/download.go:129-156](file://backend/internal/server/download.go#L129-L156)

### 用户端-主页数据
- GET /api/home/platforms
  - 描述：平台卡片（含下载Token/URL；管理员显示池内订阅）
  - 鉴权：会话
  - 状态码：200/500
- POST /api/home/token/refresh
  - 描述：刷新指定平台下载Token（旧失效）
  - 请求体：platform_id
  - 状态码：200/400/404/500
- GET /api/home/updated_at
  - 描述：可见订阅更新时间戳（管理员全池）
  - 鉴权：会话
  - 状态码：200/500

章节来源
- [backend/internal/server/home.go:37-43](file://backend/internal/server/home.go#L37-L43)
- [backend/internal/server/home.go:69-164](file://backend/internal/server/home.go#L69-L164)
- [backend/internal/server/home.go:166-194](file://backend/internal/server/home.go#L166-L194)
- [backend/internal/server/home.go:214-254](file://backend/internal/server/home.go#L214-L254)
- [backend/internal/server/home.go:256-343](file://backend/internal/server/home.go#L256-L343)

### 管理员-设置
- GET/PUT/DELETE /api/admin/settings/oidc
  - 描述：OIDC配置读取/保存/清空
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/oidc-rules
  - 描述：OIDC启用规则（审批开关、白名单）
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/local-auth
  - 描述：本地认证开关
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/captcha
  - 描述：验证码配置
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/smtp
  - 描述：SMTP配置
  - 状态码：200/400/500
- GET/PUT/DELETE /api/admin/settings/site
  - 描述：站点信息（名称+ICON文件可选）
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/ratelimit
  - 描述：速率限制配置（返回trust_proxy生效值）
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/log-level
  - 描述：日志级别
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/announcement
  - 描述：公告与页脚（首页/登录页/页脚）
  - 状态码：200/400/500
- GET/PUT /api/admin/settings/debug
  - 描述：调试模式开关
  - 状态码：200/400/500
- POST /api/admin/settings/oidc/test
  - 描述：管理员专用OIDC连接测试
  - 状态码：200/400/500
- GET /api/site/info
  - 描述：站点信息公开（无需鉴权）
  - 状态码：200

**新增**：高级模式配置
- advanced_mode：控制Xray实例管理功能开关
- 配置项：xray_api_addr、xray_api_tag等实例连接参数
- 安全考虑：仅管理员可访问，支持配置导入导出时的签名验证

章节来源
- [backend/internal/server/settings.go:51-81](file://backend/internal/server/settings.go#L51-L81)
- [backend/internal/server/settings.go:83-137](file://backend/internal/server/settings.go#L83-L137)
- [backend/internal/server/settings.go:139-165](file://backend/internal/server/settings.go#L139-L165)
- [backend/internal/server/settings.go:167-203](file://backend/internal/server/settings.go#L167-L203)
- [backend/internal/server/settings.go:205-222](file://backend/internal/server/settings.go#L205-L222)
- [backend/internal/server/settings.go:224-262](file://backend/internal/server/settings.go#L224-L262)
- [backend/internal/server/settings.go:264-282](file://backend/internal/server/settings.go#L264-L282)
- [backend/internal/server/settings.go:284-303](file://backend/internal/server/settings.go#L284-L303)
- [backend/internal/server/settings.go:305-338](file://backend/internal/server/settings.go#L305-L338)
- [backend/internal/server/settings.go:340-359](file://backend/internal/server/settings.go#L340-L359)

### 管理员-自定义订阅
- POST /api/admin/users/:id/custom?mode=upload|text
  - 描述：上传/覆盖自定义订阅（文件/文本）
  - 请求体：platform_id、text或文件
  - 状态码：200/400/500
- DELETE /api/admin/users/:id/custom/:platform
  - 描述：删除自定义订阅
  - 状态码：200/404/500
- 版本端点：/api/admin/customs/:id/versions/*（同通用模式）

章节来源
- [backend/internal/server/custom.go:21-32](file://backend/internal/server/custom.go#L21-L32)
- [backend/internal/server/custom.go:34-82](file://backend/internal/server/custom.go#L34-82)
- [backend/internal/server/custom.go:84-103](file://backend/internal/server/custom.go#L84-L103)
- [backend/internal/server/custom.go:105-126](file://backend/internal/server/custom.go#L105-L126)

### 管理员-分享订阅
- GET /api/admin/shares
  - 描述：分享列表（含token_status）
  - 状态码：200/500
- POST /api/admin/shares
  - 描述：创建分享（名称+首版本）
  - 状态码：200/400/500
- PUT /api/admin/shares/:id
  - 描述：重命名
  - 状态码：200/400/404/500
- DELETE /api/admin/shares/:id
  - 描述：删除
  - 状态码：200/404/500
- POST /api/admin/shares/:id/token/refresh
  - 描述：轮替Token（支持恢复）
  - 状态码：200/404/500
- POST /api/admin/shares/:id/token/revoke
  - 描述：吊销Token（立即失效）
  - 状态码：200/404/500
- 版本端点：/api/admin/shares/:id/versions/*（同通用模式）

章节来源
- [backend/internal/server/share.go:20-36](file://backend/internal/server/share.go#L20-L36)
- [backend/internal/server/share.go:38-45](file://backend/internal/server/share.go#L38-L45)
- [backend/internal/server/share.go:47-91](file://backend/internal/server/share.go#L47-L91)
- [backend/internal/server/share.go:93-115](file://backend/internal/server/share.go#L93-L115)
- [backend/internal/server/share.go:117-132](file://backend/internal/server/share.go#L117-L132)
- [backend/internal/server/share.go:134-168](file://backend/internal/server/share.go#L134-L168)
- [backend/internal/server/share.go:170-191](file://backend/internal/server/share.go#L170-L191)

### 系统状态与公告
- GET /api/system/status
  - 描述：系统状态（configured/app_mode/emergency/本地认证/注册/OIDC/验证码）
  - 状态码：200/500
- GET /api/public/announcement
  - 描述：公告/页脚公开端点
  - 状态码：200

**新增**：高级模式状态检测
- 系统状态包含advanced_mode标志位
- 支持Xray实例连接状态检测
- 提供实例管理能力可用性指示

章节来源
- [backend/internal/server/status.go:22-37](file://backend/internal/server/status.go#L22-L37)
- [backend/internal/server/status.go:39-79](file://backend/internal/server/status.go#L39-L79)

### 日志与SSE实时流
- GET /api/admin/logs/access
  - 描述：访问日志查询（日期范围+分页）
  - 查询：from、to、page、size
  - 鉴权：会话+管理员
  - 状态码：200/400/500
- POST /api/admin/logs/access/clear
  - 描述：清空访问日志
  - 鉴权：会话+管理员
  - 状态码：200/500
- POST /api/admin/logs/stream/token
  - 描述：换取一次性短期Token（用于SSE）
  - 鉴权：会话+管理员
  - 状态码：200/500
- GET /api/admin/logs/stream?token=...
  - 描述：SSE实时日志流（先推历史，再推增量；连接断开自动清理）
  - 鉴权：一次性短期Token（EventSource无法带Header）
  - 协议：text/event-stream；消息格式：data: <json>\n\n
  - 限制：8连接上限；连接建立即消费Token
  - 状态码：200/401/429

章节来源
- [backend/internal/server/log.go:21-30](file://backend/internal/server/log.go#L21-L30)
- [backend/internal/server/log.go:32-51](file://backend/internal/server/log.go#L32-L51)
- [backend/internal/server/log.go:53-61](file://backend/internal/server/log.go#L53-L61)
- [backend/internal/server/log.go:63-113](file://backend/internal/server/log.go#L63-L113)

## 依赖关系分析
- 路由注册集中在server.New中，按顺序注入各域服务与中间件
- 鉴权中间件复用：SessionMiddleware与AdminMiddleware
- 限流中间件：按Key（register/login/forgot/download）限制频率
- 验证码中间件：在注册/登录/忘记密码上叠加
- 版本模块：订阅/规则/自定义/分享共用版本CRUD与预览
- **新增**：Xray实例服务依赖高级模式配置，支持节点检测与用户同步

```mermaid
graph LR
R["路由注册(server.go)"] --> A["认证(auth.go)"]
R --> O["OIDC(oidc.go)"]
R --> U["用户管理(user.go)"]
R --> P["平台(platform.go)"]
R --> S["订阅(subscription.go)"]
R --> G["组(group.go)"]
R --> RU["规则(rule.go)"]
R --> D["下载(download.go)"]
R --> ST["设置(settings.go)"]
R --> C["自定义(custom.go)"]
R --> SH["分享(share.go)"]
R --> H["主页(home.go)"]
R --> L["日志(log.go)"]
R --> SS["状态(status.go)"]
R --> X["Xray实例(xray_instance.go)"]
```

图表来源
- [backend/internal/server/server.go:63-157](file://backend/internal/server/server.go#L63-L157)

章节来源
- [backend/internal/server/server.go:63-157](file://backend/internal/server/server.go#L63-L157)

## 性能与限流
- 限流策略：
  - 注册/登录/忘记密码：分别限制（KeyRegister/Login/Forgot）
  - 下载：按IP限制20次/分钟
- 缓存控制：下载类端点强制no-store/no-cache
- 内存安全：大文件流式处理，避免整读内存
- 并发与连接：SSE连接上限8；事件源一次性Token防重放
- 优雅退出：HTTP服务支持上下文关闭与超时
- **新增**：Xray实例操作限流，避免频繁API调用导致Xray服务压力

章节来源
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)
- [backend/internal/server/download.go:23-36](file://backend/internal/server/download.go#L23-L36)
- [backend/internal/server/log.go:63-113](file://backend/internal/server/log.go#L63-L113)
- [backend/internal/server/server.go:239-258](file://backend/internal/server/server.go#L239-L258)

## 故障排查指南
- 常见状态码：
  - 400：参数校验失败/业务参数错误
  - 401：会话无效/短期Token无效
  - 403：权限不足/最后管理员保护
  - 404：资源不存在/版本缺失
  - 409：冲突（邮箱/标识重复）
  - 429：限流触发
  - 500：服务器内部错误
- 调试建议：
  - 开启debug_mode以在5xx时返回更多详情
  - 检查限流Key与阈值
  - 确认OIDC state Cookie与回调参数一致
  - SSE连接失败时检查短期Token是否已消费
- **新增**：Xray集成问题排查
  - 检查advanced_mode配置是否开启
  - 验证Xray实例连接参数正确性
  - 查看用户同步状态（pending/synced/failed）
  - 检查配额超限标记（quota_exceeded）

章节来源
- [backend/internal/server/server.go:225-237](file://backend/internal/server/server.go#L225-L237)
- [backend/internal/server/settings.go:340-359](file://backend/internal/server/settings.go#L340-L359)
- [backend/internal/server/log.go:63-113](file://backend/internal/server/log.go#L63-L113)

## 结论
本API文档覆盖了系统的全部对外接口，明确了认证、鉴权、限流、版本管理与SSE实时日志等关键机制。**新增的Xray实例管理功能**提供了完整的用户生命周期同步能力，包括并发安全的凭据生成、配额超限拦截、高级模式开关控制等特性。建议在集成时严格遵循统一响应格式、错误码约定与缓存控制策略，并结合限流与调试模式进行稳定性保障。

## 附录

### 认证流程图（OIDC）
```mermaid
sequenceDiagram
participant FE as "前端"
participant API as "OIDC登录"
participant IDP as "身份提供商"
participant AUTH as "会话签发"
FE->>API : GET /api/auth/oidc/login
API-->>FE : 302 + state Cookie
FE->>IDP : 跳转授权
IDP-->>API : GET /api/auth/oidc/callback?code&state
API->>AUTH : 解析并签发会话
AUTH-->>API : token
API-->>FE : 302 重定向至前端携带token
```

图表来源
- [backend/internal/server/oidc.go:31-40](file://backend/internal/server/oidc.go#L31-L40)
- [backend/internal/server/oidc.go:42-90](file://backend/internal/server/oidc.go#L42-L90)

### SSE日志流时序
```mermaid
sequenceDiagram
participant Admin as "管理员"
participant API as "日志服务端"
participant SSE as "SSE通道"
Admin->>API : POST /api/admin/logs/stream/token
API-->>Admin : { token }
Admin->>API : GET /api/admin/logs/stream?token=...
API->>SSE : 推送历史日志
API->>SSE : 推送增量日志
Note over Admin,SSE : 连接断开自动清理；Token一次性
```

图表来源
- [backend/internal/server/log.go:53-113](file://backend/internal/server/log.go#L53-L113)

### Xray用户生命周期同步流程
```mermaid
sequenceDiagram
participant User as "用户"
participant API as "用户管理API"
participant Sync as "同步服务"
participant Xray as "Xray实例"
User->>API : 用户激活/启用/换组
API->>Sync : 触发用户同步
Sync->>Sync : 检查高级模式开关
alt 高级模式开启
Sync->>Sync : 检查配额超限
alt 未超限
Sync->>Xray : AddUser/RemoveUser
Xray-->>Sync : 同步结果
Sync->>API : 更新同步状态
else 超限
Sync->>API : 记录跳过原因
end
else 高级模式关闭
Sync->>API : 静默跳过
end
```

图表来源
- [backend/internal/user/user.go:82-154](file://backend/internal/user/user.go#L82-L154)
- [Design2.md:260-290](file://Design2.md#L260-L290)