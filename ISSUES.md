# Issues.md — 后端代码审查问题追踪

## 待修复

- [ ] `AuthCallback` 中 `isSecure` 在 error/success 两条路径重复计算 → 函数开头算一次，两处复用

## 已验证，不修复

- [x] **`GetUpdateTime()` 全表遍历** (`subscription_service.go`): 加载全部订阅后在 Go 层双层循环遍历版本 JSON 找最大 `updated_at`。≤10 平台 × 2 类型 × 5 版本 = 最多 100 条记录，每首页加载 1 次（非轮询），循环耗时微秒级。SQL 聚合方案依赖 `json_each` 扩展且增加复杂度，当前规模下无收益。
- [x] **`SubDownloadToken` Token 无效时日志 `download_type` 硬编码** (`handlers.go`): Token 查找失败时固定写 `"subscription"`，无法区分 regular/custom。但 `access_logs` 表有 `CHECK(download_type IN ('subscription','share','custom','rule'))` 约束，改为 `"unknown"` 需改 schema 并处理已有数据库迁移。token_invalid 时 `status=failed, error_reason=token_invalid` 已足够排查，修复的收益远小于 schema 迁移的风险。

## 已修复

- [x] `download_tokens` 表缺少 UNIQUE 约束，可能插入重复 Token → 添加两条 partial unique index：`(user_id, platform, type) WHERE custom_sub_id IS NULL` 和 `(user_id, platform, custom_sub_id) WHERE custom_sub_id IS NOT NULL`
- [x] `UserService.Update` 中 `target.Role` 被强制赋值后又检查 `target.Role != existing.Role`，永远为 false 的死代码 → 移除该检查
- [x] `SubscriptionService.Delete`、`RuleService.Delete`、`ShareSubscriptionService.Delete`、`UserService.Delete`、`CustomSubscriptionService.Delete` 无事务保护 → 与 `PlatformService.Delete` 一致，DB 操作包裹事务，commit 后删文件
- [x] `readUploadContent` 的 JSON 路径无 body 大小限制（multipart 有 50MB） → 已在 JSON 路径中添加 `MaxBytesReader(50MB)`
- [x] `UploadVersion` 中 `NextVersion` 被外部和 `CreateVersion` 内部各算一次 → 移除外部 `NextVersion` 调用，改为从 `CreateVersion` 返回的 `newVersions` 末尾元素提取版本号
