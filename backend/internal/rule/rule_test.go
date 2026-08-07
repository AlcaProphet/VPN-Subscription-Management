package rule

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/subscription"
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
)

// testMigrateFS 构造含 rules/subscriptions/versions/rule_tokens 表的迁移集
func testMigrateFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL CHECK (owner_type IN ('subscription','rule','custom','share')),
				owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL, file_path TEXT NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (owner_type, owner_id, version_no));
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS rule_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				rule_id INTEGER NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				refreshed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1006_rules.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS rules (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
				client_type TEXT NOT NULL DEFAULT 'shadowrocket',
				schemes TEXT NOT NULL DEFAULT '[]',
				current_version INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
}

// newTestRuleService 临时库 + 规则服务（含版本/Token/订阅服务）
func newTestRuleService(t *testing.T) (*store.Store, *Service, *version.Service, *token.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), testMigrateFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	ver := version.NewService(st, t.TempDir(), log.New("error", "console"))
	tk := token.NewService(st, log.New("error", "console"))
	subs := subscription.NewService(st, ver, log.New("error", "console"))
	svc := NewService(st, ver, tk, subs, log.New("error", "console"))
	return st, svc, ver, tk
}

// TestCreate 创建规则：手填标识 + 自动生成全局 Token + 首版本
func TestCreate(t *testing.T) {
	_, svc, _, _ := newTestRuleService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, "默认规则", "my-rules", "shadowrocket", []string{"shadowrocket://add/{url}"}, version.BytesContent([]byte("rules: []")))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if r.Token == "" {
		t.Error("创建后应自动生成规则 Token")
	}
	if r.CurrentVersion != 1 {
		t.Errorf("首版本应切换为当前: %d", r.CurrentVersion)
	}
	if r.Slug != "my-rules" {
		t.Errorf("手填标识应保留: %s", r.Slug)
	}
}

// TestSlugCrossConflict 规则标识手填跨四类校验（与既有订阅冲突 → 409；至此四表查重全部生效）
func TestSlugCrossConflict(t *testing.T) {
	st, svc, _, _ := newTestRuleService(t)
	ctx := context.Background()
	// 预置订阅占用 slug
	if _, err := st.DB().Exec(`INSERT INTO subscriptions (slug, name, platform_id) VALUES ('shared-slug', '订阅', 1)`); err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}
	_, err := svc.Create(ctx, "规则", "shared-slug", "shadowrocket", nil, version.BytesContent([]byte("v1")))
	if !errors.Is(err, subscription.ErrSlugConflict) {
		t.Errorf("跨四类冲突应返回 ErrSlugConflict: %v", err)
	}
	// 可用标识创建成功
	if _, err := svc.Create(ctx, "规则", "free-slug", "shadowrocket", nil, version.BytesContent([]byte("v1"))); err != nil {
		t.Errorf("可用标识应创建成功: %v", err)
	}
}

// TestTokenGlobalShare 规则 Token 全局共享：不绑定用户，刷新轮替旧 Token 失效
func TestTokenGlobalShare(t *testing.T) {
	st, svc, _, _ := newTestRuleService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, "规则", "global-rule", "shadowrocket", nil, version.BytesContent([]byte("v1")))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// 轮替：旧 Token 删除、新 Token 生成
	newToken, err := svc.RefreshToken(ctx, r.ID)
	if err != nil {
		t.Fatalf("刷新失败: %v", err)
	}
	if newToken == r.Token {
		t.Error("刷新后 Token 应变化")
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rule_tokens WHERE rule_id = ?`, r.ID).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Errorf("轮替后应仅一条 Token: %d", count)
	}
}

// TestRenameOnly 创建后仅可改名（客户端类型与 scheme 不可修改）
func TestRenameOnly(t *testing.T) {
	st, svc, _, _ := newTestRuleService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, "原名", "rename-rule", "shadowrocket", []string{"shadowrocket://add/{url}"}, version.BytesContent([]byte("v1")))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := svc.Rename(ctx, r.ID, "新名"); err != nil {
		t.Fatalf("改名失败: %v", err)
	}
	var name, clientType string
	if err := st.DB().QueryRow(`SELECT name, client_type FROM rules WHERE id = ?`, r.ID).Scan(&name, &clientType); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if name != "新名" || clientType != "shadowrocket" {
		t.Errorf("仅名称应变化: name=%s client_type=%s", name, clientType)
	}
}

// TestInvalidClientType 客户端类型校验（当前仅 shadowrocket）
func TestInvalidClientType(t *testing.T) {
	_, svc, _, _ := newTestRuleService(t)
	ctx := context.Background()
	if _, err := svc.Create(ctx, "规则", "bad-type", "clash", nil, version.BytesContent([]byte("v1"))); !errors.Is(err, ErrBadRequest) {
		t.Errorf("非法客户端类型应报参数错误: %v", err)
	}
}
