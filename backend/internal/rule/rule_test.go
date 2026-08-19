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
								file_name TEXT NOT NULL DEFAULT '',
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
				is_home_default INTEGER NOT NULL DEFAULT 0,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE UNIQUE INDEX IF NOT EXISTS idx_rules_home_default ON rules(is_home_default) WHERE is_home_default = 1;`)},
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

// TestCreateEmptyRule 空规则实体：src=nil 时仅创建规则行 + Token，无版本（供 SR 分流规则装配目标）
func TestCreateEmptyRule(t *testing.T) {
	st, svc, _, _ := newTestRuleService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, "空规则", "empty-rule", "shadowrocket", nil, nil)
	if err != nil {
		t.Fatalf("创建空规则失败: %v", err)
	}
	if r.CurrentVersion != 0 {
		t.Errorf("空规则 current_version 应为 0: %d", r.CurrentVersion)
	}
	if r.Token == "" {
		t.Error("空规则仍应自动生成规则 Token")
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM versions WHERE owner_type='rule' AND owner_id=?`, r.ID).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Errorf("空规则不应创建版本: %d", n)
	}
}

// TestSetHomeDefaultUnique 首页默认规则唯一：切换时清旧置新；取消后回空态；删除默认规则后无默认行
func TestSetHomeDefaultUnique(t *testing.T) {
	st, svc, _, _ := newTestRuleService(t)
	ctx := context.Background()
	a, err := svc.Create(ctx, "规则A", "home-a", "shadowrocket", nil, nil)
	if err != nil {
		t.Fatalf("创建 A 失败: %v", err)
	}
	b, err := svc.Create(ctx, "规则B", "home-b", "shadowrocket", nil, nil)
	if err != nil {
		t.Fatalf("创建 B 失败: %v", err)
	}
	if err := svc.SetHomeDefault(ctx, a.ID, true); err != nil {
		t.Fatalf("设置 A 为默认失败: %v", err)
	}
	if err := svc.SetHomeDefault(ctx, b.ID, true); err != nil {
		t.Fatalf("切换默认到 B 失败: %v", err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rules WHERE is_home_default = 1`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("默认规则应至多一条: %d", count)
	}
	var currentID int64
	if err := st.DB().QueryRow(`SELECT id FROM rules WHERE is_home_default = 1`).Scan(&currentID); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if currentID != b.ID {
		t.Errorf("默认规则应为 B: %d", currentID)
	}
	// 取消默认
	if err := svc.SetHomeDefault(ctx, b.ID, false); err != nil {
		t.Fatalf("取消默认失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rules WHERE is_home_default = 1`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 0 {
		t.Errorf("取消后应无默认规则: %d", count)
	}
	// 删除默认规则后同样无默认行
	if err := svc.SetHomeDefault(ctx, a.ID, true); err != nil {
		t.Fatalf("设置 A 为默认失败: %v", err)
	}
	if err := svc.Delete(ctx, a.ID); err != nil {
		t.Fatalf("删除默认规则失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rules WHERE is_home_default = 1`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 0 {
		t.Errorf("删除默认规则后应无默认行: %d", count)
	}
}
