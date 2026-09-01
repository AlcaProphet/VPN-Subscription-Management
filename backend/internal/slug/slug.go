// Package slug 提供资源标识自动生成器与查重助手（Build2 从 setup 包抽取，供 platform/group/share/custom 共用）。
package slug

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"log/slog"
)

// 短码字符集：小写字母数字，去除易混淆字符
const slugCharset = "abcdefghjkmnpqrstuvwxyz23456789"

// Generate 类型前缀 + 8 位加密安全随机短码；冲突自动重试最多 3 次，仍冲突报错并记日志（Design1 §2.2）
func Generate(ctx context.Context, tx *sql.Tx, prefix string, exists func(slug string) (bool, error)) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		code, err := randomCode(8) // crypto/rand 从 slugCharset 取 8 字符；失败返回 err
		if err != nil {
			return "", err
		}
		value := prefix + code
		dup, err := exists(value)
		if err != nil {
			return "", err
		}
		if !dup {
			return value, nil
		}
	}
	slog.Error("标识生成冲突超过重试上限", "prefix", prefix)
	return "", fmt.Errorf("标识生成失败：连续冲突，请重试")
}

// randomCode 从字符集取 n 位加密安全随机短码
func randomCode(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机短码失败: %w", err)
	}
	for i := range b {
		b[i] = slugCharset[int(b[i])%len(slugCharset)]
	}
	return string(b), nil
}

// TableHasSlug 检查表内是否已存在该 slug（供标识生成器冲突检测）。
// 表名仅允许白名单内的固定值（防动态 SQL 注入）
func TableHasSlug(ctx context.Context, tx *sql.Tx, table, value string) (bool, error) {
	switch table {
	case "groups", "platforms", "subscriptions", "rules", "custom_subscriptions", "share_subscriptions", "xray_instances":
	default:
		return false, fmt.Errorf("非法表名: %s", table)
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE slug = ?`, value).Scan(&n); err != nil {
		return false, fmt.Errorf("查询 %s 标识失败: %w", table, err)
	}
	return n > 0, nil
}

// ExistsInFourTables 资源全局唯一命名空间交叉校验（subscriptions/rules/custom_subscriptions/share_subscriptions/xray_instances）；
// 表尚未建立时跳过（sqlite_master 预检），供资源创建/查重共用
func ExistsInFourTables(ctx context.Context, tx *sql.Tx, value string) (bool, error) {
	for _, table := range []string{"subscriptions", "rules", "custom_subscriptions", "share_subscriptions", "xray_instances"} {
		ok, err := tableExists(ctx, tx, table)
		if err != nil {
			return false, err
		}
		if !ok {
			continue // 表未建立（后续 Step 才迁移）→ 跳过该表
		}
		dup, err := TableHasSlug(ctx, tx, table, value)
		if err != nil {
			return false, err
		}
		if dup {
			return true, nil
		}
	}
	return false, nil
}

// tableExists 检查表是否已存在（sqlite_master 预检；供「表缺失跳过」语义使用）
func tableExists(ctx context.Context, tx *sql.Tx, name string) (bool, error) {
	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		return false, fmt.Errorf("检查表 %s 失败: %w", name, err)
	}
	return n > 0, nil
}
