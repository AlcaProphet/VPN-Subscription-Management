package slug

import (
	"context"
	"regexp"
	"testing"
)

// TestGenerate 格式匹配与冲突重试（tx 传 nil：exists 由注入 mock 提供，不实际访问库）
func TestGenerate(t *testing.T) {
	ctx := context.Background()
	// 注入 exists 冲突两次后成功
	attempts := 0
	value, err := Generate(ctx, nil, "group-", func(s string) (bool, error) {
		attempts++
		return attempts <= 2, nil // 前两次冲突，第三次成功
	})
	if err != nil {
		t.Fatalf("Generate 失败: %v", err)
	}
	if !regexp.MustCompile(`^group-[a-z0-9]{8}$`).MatchString(value) {
		t.Errorf("slug 格式异常: %s", value)
	}
	// 一直冲突 → 超限报错
	if _, err = Generate(ctx, nil, "platform-", func(s string) (bool, error) {
		return true, nil
	}); err == nil {
		t.Error("连续冲突应报错")
	}
}

// TestTableHasSlug 非法表名拒绝（防动态 SQL 注入白名单）
func TestTableHasSlug(t *testing.T) {
	if _, err := TableHasSlug(context.Background(), nil, "users; DROP TABLE groups", "x"); err == nil {
		t.Error("非法表名应报错")
	}
}
