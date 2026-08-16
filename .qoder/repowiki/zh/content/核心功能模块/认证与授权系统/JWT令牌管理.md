# JWT令牌管理

<cite>
**本文引用的文件**
- [backend/internal/auth/auth.go](file://backend/internal/auth/auth.go)
- [backend/internal/config/config.go](file://backend/internal/config/config.go)
- [backend/internal/server/auth.go](file://backend/internal/server/auth.go)
- [backend/cmd/server/main.go](file://backend/cmd/server/main.go)
- [backend/migrations/1004_tokens.sql](file://backend/migrations/1004_tokens.sql)
- [backend/internal/token/token.go](file://backend/internal/token/token.go)
</cite>

## 目录
1. [简介](#简介)
2. [项目结构](#项目结构)
3. [核心组件](#核心组件)
4. [架构总览](#架构总览)
5. [详细组件分析](#详细组件分析)
6. [依赖关系分析](#依赖关系分析)
7. [性能考虑](#性能考虑)
8. [故障排查指南](#故障排查指南)
9. [结论](#结论)
10. [附录：API与数据模型](#附录api与数据模型)

## 简介
本文件围绕JWT令牌管理系统，系统性说明令牌的签发、解析、验证流程，签名密钥管理、令牌载荷设计（用户ID、凭证版本等）、有效期与刷新机制；并解释会话中间件如何从请求头提取Bearer令牌、校验有效性并注入用户上下文。同时给出安全与性能建议，以及常见问题的定位方法。

## 项目结构
后端采用分层架构：
- 接入层（server）：注册路由、参数校验、统一响应
- 业务层（auth、config、token、user等）：认证、配置、令牌生命周期
- 数据层（store + migrations）：SQLite持久化、迁移、事务

```mermaid
graph TB
Client["客户端"] --> API["Gin 路由 /api/auth/*"]
API --> AuthHandler["AuthHandler<br/>登录/注册/登出/当前用户"]
AuthHandler --> AuthService["auth.Service<br/>Issue/Parse/SessionMiddleware"]
AuthService --> ConfigService["config.Service<br/>EnsureSigningKey/GetSigningKey"]
AuthService --> UserSource["user.UserSource<br/>SnapshotByID(实时查库)"]
API --> TokenService["token.Service<br/>下载/分享/规则Token"]
ConfigService --> DB[("SQLite 数据库")]
UserSource --> DB
TokenService --> DB
```

图表来源
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)
- [backend/internal/auth/auth.go:87-135](file://backend/internal/auth/auth.go#L87-L135)
- [backend/internal/config/config.go:282-313](file://backend/internal/config/config.go#L282-L313)
- [backend/internal/token/token.go:19-88](file://backend/internal/token/token.go#L19-L88)

章节来源
- [backend/cmd/server/main.go:24-98](file://backend/cmd/server/main.go#L24-L98)
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)

## 核心组件
- 认证服务（auth.Service）
  - Issue：使用HS256签发JWT，载荷包含用户ID和凭证版本，附带标准iat/exp
  - Parse：验签并解析载荷，强制算法为HMAC
  - SessionMiddleware：从Authorization头提取Bearer令牌，解析后实时查库比对credential_version与状态，注入用户上下文
- 配置服务（config.Service）
  - EnsureSigningKey/GetSigningKey：确保并读取签名密钥（system_config.signing_key），缺失时自动生成32字节随机密钥
  - 敏感配置加解密：基于签名密钥派生AES-GCM密钥
- 令牌服务（token.Service）
  - 下载/分享/规则三类Token的创建、轮替、吊销与生命周期联动
- 服务器路由（server.AuthHandler）
  - 注册/登录/登出/当前用户端点，叠加限流与验证码中间件

章节来源
- [backend/internal/auth/auth.go:22-135](file://backend/internal/auth/auth.go#L22-L135)
- [backend/internal/config/config.go:24-313](file://backend/internal/config/config.go#L24-L313)
- [backend/internal/token/token.go:19-235](file://backend/internal/token/token.go#L19-L235)
- [backend/internal/server/auth.go:16-157](file://backend/internal/server/auth.go#L16-L157)

## 架构总览
JWT令牌在系统中的位置与流转如下：

```mermaid
sequenceDiagram
participant C as "客户端"
participant S as "Gin 路由"
participant A as "AuthHandler"
participant AS as "auth.Service"
participant U as "user.UserSource"
participant CFG as "config.Service"
participant DB as "SQLite"
C->>S : POST /api/auth/login {email,password,remember}
S->>A : login()
A->>U : Login(email,password)
U-->>A : 用户信息(含credential_version)
A->>AS : Issue(userID, credentialVersion, duration)
AS->>CFG : EnsureSigningKey()
CFG->>DB : 读取/生成 signing_key
AS-->>A : token, expires_at
A-->>C : {token, expires_at, user}
Note over C,S : 后续受保护请求携带 Authorization : Bearer <token>
C->>S : GET /api/auth/me (带Bearer)
S->>A : me()
A->>AS : SessionMiddleware()
AS->>AS : Parse(token)
AS->>CFG : GetSigningKey()
CFG->>DB : 读取 signing_key
AS->>U : SnapshotByID(userID)
U-->>AS : {role,status,credential_version}
AS-->>A : 通过(设置CtxUserID/CtxUserRole)
A-->>C : 当前用户信息
```

图表来源
- [backend/internal/server/auth.go:99-134](file://backend/internal/server/auth.go#L99-L134)
- [backend/internal/auth/auth.go:98-135](file://backend/internal/auth/auth.go#L98-L135)
- [backend/internal/config/config.go:282-313](file://backend/internal/config/config.go#L282-L313)

## 详细组件分析

### JWT签发与解析（auth.Service）
- 签发（Issue）
  - 获取或生成签名密钥（EnsureSigningKey）
  - 构造Claims：uid、cv（credential_version）、iat、exp
  - 使用HS256签名生成JWT
- 解析（Parse）
  - 获取签名密钥（GetSigningKey）
  - 强制要求算法为HMAC，否则拒绝
  - 返回Claims供后续校验

```mermaid
flowchart TD
Start(["调用 Issue(userID, cv, dur)"]) --> Key["EnsureSigningKey()"]
Key --> Claims["构建 Claims{uid,cv,iat,exp}"]
Claims --> Sign["jwt.NewWithClaims(HS256).SignedString(key)"]
Sign --> Return["返回 token, expires_at"]
```

图表来源
- [backend/internal/auth/auth.go:98-116](file://backend/internal/auth/auth.go#L98-L116)
- [backend/internal/config/config.go:294-313](file://backend/internal/config/config.go#L294-L313)

章节来源
- [backend/internal/auth/auth.go:66-135](file://backend/internal/auth/auth.go#L66-L135)
- [backend/internal/config/config.go:282-313](file://backend/internal/config/config.go#L282-L313)

### 会话中间件（SessionMiddleware）
- 从Authorization头提取Bearer令牌
- 解析并验签
- 实时查库取用户快照，比对credential_version与status
- 将用户ID与角色注入到上下文（CtxUserID、CtxUserRole）

```mermaid
flowchart TD
MStart["进入 SessionMiddleware"] --> Extract["提取 Authorization 头并去除 'Bearer '"]
Extract --> CheckEmpty{"是否为空?"}
CheckEmpty -- 是 --> Fail401["返回 401 凭据缺失"]
CheckEmpty -- 否 --> Parse["Parse(token)"]
Parse --> POK{"解析成功?"}
POK -- 否 --> FailExp["返回 401 无效或过期"]
POK -- 是 --> Snap["SnapshotByID(uid)"]
Snap --> Ver{"credential_version 匹配?"}
Ver -- 否 --> FailReLogin["返回 401 请重新登录"]
Ver -- 是 --> Status{"status == active?"}
Status -- 否 --> FailInactive["返回 401 未激活或禁用"]
Status -- 是 --> Inject["设置 CtxUserID/CtxUserRole"]
Inject --> Next["继续处理请求"]
```

图表来源
- [backend/internal/auth/auth.go:144-179](file://backend/internal/auth/auth.go#L144-L179)

章节来源
- [backend/internal/auth/auth.go:137-179](file://backend/internal/auth/auth.go#L137-L179)

### 登录/注册流程（server.AuthHandler）
- 注册：校验参数与开关，创建用户（首管理员逻辑由user包实现），若直接激活则签发会话
- 登录：校验邮箱密码，根据remember选择时长（7天/24小时），签发会话
- 当前用户：经SessionMiddleware鉴权后返回用户信息
- 登出：客户端语义，服务端无状态

```mermaid
sequenceDiagram
participant C as "客户端"
participant H as "AuthHandler"
participant U as "user.Service"
participant A as "auth.Service"
participant CFG as "config.Service"
C->>H : POST /api/auth/register
H->>U : Register(...)
U-->>H : 用户(可能active/pending)
alt 直接激活
H->>A : Issue(uid, cv, 24h)
A->>CFG : EnsureSigningKey()
A-->>H : token, exp
H-->>C : {token, expires_at, status, is_admin}
else 待审批
H-->>C : {status : "pending", message}
end
C->>H : POST /api/auth/login
H->>U : Login(...)
U-->>H : 用户
H->>A : Issue(uid, cv, remember?7d : 24h)
A-->>H : token, exp
H-->>C : {token, expires_at, user}
```

图表来源
- [backend/internal/server/auth.go:44-134](file://backend/internal/server/auth.go#L44-L134)
- [backend/internal/auth/auth.go:98-116](file://backend/internal/auth/auth.go#L98-L116)
- [backend/internal/config/config.go:294-313](file://backend/internal/config/config.go#L294-L313)

章节来源
- [backend/internal/server/auth.go:44-157](file://backend/internal/server/auth.go#L44-L157)

### 令牌服务（token.Service）
- 下载Token：三态复用键（user+platform±custom_sub_id/subscription_id），并发安全先查后建，冲突重试
- 分享/规则Token：创建、轮替（旧删新写）、记录刷新时间
- 生命周期联动：删除订阅/自定义/显式Token，批量清理用户Token

```mermaid
classDiagram
class Service {
-store *store.Store
-log *slog.Logger
+GetOrCreateUserToken(ctx, userID, platformID, customSubID, subscriptionID) (*UserToken, error)
+RefreshUserToken(ctx, tokenValue) (*UserToken, error)
+FindByToken(ctx, tokenValue) (*UserToken, error)
+CreateShareTokenTx(ctx, tx, shareID) (string, error)
+RotateShareTokenTx(ctx, tx, shareID) (string, error)
+CreateRuleTokenTx(ctx, tx, ruleID) (string, error)
+RotateRuleTokenTx(ctx, tx, ruleID) (string, error)
+RotateRuleToken(ctx, ruleID) (string, error)
+DeleteGroupTokens(ctx, userID, platformID) error
+DeleteBySubscriptionTx(ctx, tx, subscriptionID) error
+DeleteByCustomTx(ctx, tx, customID) error
+DeleteExplicit(ctx, userID) error
+DeleteAllForUserTx(ctx, tx, userID) error
}
```

图表来源
- [backend/internal/token/token.go:19-235](file://backend/internal/token/token.go#L19-L235)

章节来源
- [backend/internal/token/token.go:19-235](file://backend/internal/token/token.go#L19-L235)

### 签名密钥管理（config.Service）
- 存储位置：system_config表 key=signing_key
- 生成策略：首次EnsureSigningKey时生成32字节加密安全随机值并落库
- 读取策略：GetSigningKey仅读取不生成，缺失返回错误（用于验签）
- 事务内版本：EnsureSigningKeyTx/GetSigningKeyTx保证事务原子性

```mermaid
flowchart TD
KStart["调用 EnsureSigningKey(ctx)"] --> Read["Get(KeySigningKey)"]
Read --> Exists{"存在?"}
Exists -- 是 --> ReturnKey["返回已有密钥"]
Exists -- 否 --> Gen["rand.Read(32 bytes)"]
Gen --> Set["Set(KeySigningKey, value)"]
Set --> Log["记录日志"]
Log --> ReturnNew["返回新密钥"]
```

图表来源
- [backend/internal/config/config.go:294-313](file://backend/internal/config/config.go#L294-L313)

章节来源
- [backend/internal/config/config.go:24-313](file://backend/internal/config/config.go#L24-L313)

## 依赖关系分析
- auth.Service依赖config.Service获取签名密钥，依赖user.UserSource进行实时用户快照查询
- server.AuthHandler组合auth.Service与user.Service，提供HTTP端点
- token.Service依赖store.Store进行数据库操作，独立于JWT体系但共享同一数据库
- main负责装配各服务并启动HTTP服务

```mermaid
graph LR
Main["main.go"] --> Server["server.New(...)"]
Server --> AuthRoutes["RegisterAuthRoutes(...)"]
AuthRoutes --> AuthSvc["auth.Service"]
AuthSvc --> ConfigSvc["config.Service"]
AuthSvc --> UserSrc["user.UserSource"]
Server --> TokenSvc["token.Service"]
ConfigSvc --> Store["store.Store"]
UserSrc --> Store
TokenSvc --> Store
```

图表来源
- [backend/cmd/server/main.go:77-84](file://backend/cmd/server/main.go#L77-L84)
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)

章节来源
- [backend/cmd/server/main.go:77-84](file://backend/cmd/server/main.go#L77-L84)
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)

## 性能考虑
- 会话校验每次请求实时查库，避免缓存权限信息导致越权
- SQLite单写者模型（MaxOpenConns=1）配合BEGIN IMMEDIATE事务，降低并发写冲突
- 下载Token采用“先查后建”并发安全模式，减少重复写入
- 访问日志按created_at索引，支持定期清理

[本节为通用指导，无需特定文件引用]

## 故障排查指南
- 401 凭据缺失：检查Authorization头是否包含Bearer前缀
- 401 解析失败：确认签名密钥已配置且一致；算法必须为HMAC
- 401 凭据失效：检查credential_version是否变更（如重置密码、禁用账号）
- 401 账号未激活/禁用：检查用户status字段
- 下载Token不存在：确认复用键是否正确，或Token是否已被轮替/删除

章节来源
- [backend/internal/auth/auth.go:144-179](file://backend/internal/auth/auth.go#L144-L179)
- [backend/internal/token/token.go:125-172](file://backend/internal/token/token.go#L125-L172)

## 结论
本系统采用无状态JWT会话机制，结合凭证版本号实现细粒度失效控制；签名密钥集中管理，支持自动初始化与事务内一致性；会话中间件严格校验用户状态与权限，保障接口安全。下载/分享/规则Token提供灵活的资源访问控制，并通过数据库事务保证并发安全。整体设计兼顾安全性、可维护性与性能。

[本节为总结，无需特定文件引用]

## 附录：API与数据模型

### 认证相关API
- POST /api/auth/register：注册并可能签发会话
- POST /api/auth/login：登录并签发会话
- GET /api/auth/me：获取当前用户信息（需鉴权）
- POST /api/auth/logout：客户端登出（服务端无状态）

章节来源
- [backend/internal/server/auth.go:24-34](file://backend/internal/server/auth.go#L24-L34)
- [backend/internal/server/auth.go:44-157](file://backend/internal/server/auth.go#L44-L157)

### 数据模型（节选）
- users：用户基本信息、角色、状态、凭证版本
- system_config：系统配置（含signing_key）
- download_tokens/share_tokens/rule_tokens：各类资源访问Token

章节来源
- [backend/migrations/1004_tokens.sql:1-47](file://backend/migrations/1004_tokens.sql#L1-L47)