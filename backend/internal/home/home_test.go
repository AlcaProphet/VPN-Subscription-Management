package home

import (
	"context"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
)

func newTestHomeService(t *testing.T) (*store.Store, *Service, *token.Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0002_home.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS platforms (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				slug TEXT NOT NULL,
				name TEXT NOT NULL,
				description TEXT,
				schemes TEXT NOT NULL DEFAULT '[]',
				installer_files TEXT NOT NULL DEFAULT '[]',
				installer_urls TEXT NOT NULL DEFAULT '[]');
			CREATE TABLE IF NOT EXISTS custom_subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL);
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				platform_id INTEGER NOT NULL,
				name TEXT NOT NULL,
				product_type TEXT NOT NULL DEFAULT 'yaml',
				current_version INTEGER NOT NULL DEFAULT 0);
			CREATE TABLE IF NOT EXISTS versions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				owner_type TEXT NOT NULL,
				owner_id INTEGER NOT NULL,
				version_no INTEGER NOT NULL,
				file_path TEXT NOT NULL DEFAULT '',
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS assembly_blueprints (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				version_id INTEGER NOT NULL);
			CREATE TABLE IF NOT EXISTS download_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL,
				custom_sub_id INTEGER,
				subscription_id INTEGER,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(context.Background(), config.KeyFrontendURL, "https://vpn.example.com"); err != nil {
		t.Fatalf("设置前端地址失败: %v", err)
	}
	tokenSvc := token.NewService(st, log.New("error", "console"))
	return st, NewService(st, tokenSvc, cfg), tokenSvc
}

func insertPlatform(t *testing.T, st *store.Store, slug, name string) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO platforms (slug, name, description, schemes) VALUES (?,?,?,?)`,
		slug, name, "desc", `["https"]`)
	if err != nil {
		t.Fatalf("插入平台失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertSubscription(t *testing.T, st *store.Store, platformID int64, name string, currentVersion int64) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO subscriptions (platform_id, name, product_type, current_version) VALUES (?,?,?,?)`,
		platformID, name, "yaml", currentVersion)
	if err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
	id, _ := res.LastInsertId()
	if currentVersion > 0 {
		if _, err := st.DB().Exec(`INSERT INTO versions (owner_type, owner_id, version_no, file_path, updated_at)
			VALUES ('subscription', ?, ?, 'v1.yaml', CURRENT_TIMESTAMP)`, id, currentVersion); err != nil {
			t.Fatalf("插入版本失败: %v", err)
		}
	}
	return id
}

func cardFor(cards []PlatformCard, platformID int64) *PlatformCard {
	for i := range cards {
		if cards[i].PlatformID == platformID {
			return &cards[i]
		}
	}
	return nil
}

// TestListPlatformsCustomPriorityCleansHiddenGroupToken 旧实现先为激活平台创建组 Token，
// 再发现自定义订阅并覆盖卡片，导致隐藏组 Token 残留。新业务键解析必须先自定义、后清理。
func TestListPlatformsCustomPriorityCleansHiddenGroupToken(t *testing.T) {
	st, svc, tokenSvc := newTestHomeService(t)
	ctx := context.Background()
	pid := insertPlatform(t, st, "clash", "Clash")
	insertSubscription(t, st, pid, "平台订阅", 1)
	groupTk, err := tokenSvc.GetOrCreateUserToken(ctx, 9, pid, 0, 0)
	if err != nil {
		t.Fatalf("预置历史组 Token 失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO custom_subscriptions (user_id, platform_id) VALUES (9,?)`, pid); err != nil {
		t.Fatalf("插入自定义订阅失败: %v", err)
	}
	cards, err := svc.ListPlatforms(ctx, 9, "user")
	if err != nil {
		t.Fatalf("ListPlatforms 失败: %v", err)
	}
	card := cardFor(cards, pid)
	if card == nil || card.Status != "custom" || card.DownloadToken == "" {
		t.Fatalf("自定义订阅卡片异常: %+v", card)
	}
	if _, err := tokenSvc.FindByToken(ctx, groupTk.Token); err != token.ErrTokenNotFound {
		t.Fatalf("隐藏组 Token 应被清理: %v", err)
	}
	var count, customCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id=9 AND platform_id=?`, pid).Scan(&count); err != nil {
		t.Fatalf("查询 Token 总数失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id=9 AND platform_id=? AND custom_sub_id IS NOT NULL`, pid).Scan(&customCount); err != nil {
		t.Fatalf("查询自定义 Token 失败: %v", err)
	}
	if count != 1 || customCount != 1 {
		t.Fatalf("应只保留一个自定义 Token: total=%d custom=%d", count, customCount)
	}
}

// TestListPlatformsGroupAndUnassigned 无自定义有激活版本时组 Token；无内容时不创建。
func TestListPlatformsGroupAndUnassigned(t *testing.T) {
	st, svc, tokenSvc := newTestHomeService(t)
	ctx := context.Background()
	readyID := insertPlatform(t, st, "ready", "Ready")
	insertSubscription(t, st, readyID, "平台订阅", 1)
	emptyID := insertPlatform(t, st, "empty", "Empty")
	cards, err := svc.ListPlatforms(ctx, 10, "user")
	if err != nil {
		t.Fatalf("ListPlatforms 失败: %v", err)
	}
	ready := cardFor(cards, readyID)
	if ready == nil || ready.Status != "ready" || ready.DownloadToken == "" {
		t.Fatalf("有激活版本应返回 ready 组 Token: %+v", ready)
	}
	empty := cardFor(cards, emptyID)
	if empty == nil || empty.Status != "unassigned" || empty.DownloadToken != "" {
		t.Fatalf("无激活版本应为 unassigned 且不创建 Token: %+v", empty)
	}
	tk, err := tokenSvc.ResolveUserToken(ctx, 10, readyID)
	if err != nil || tk == nil || tk.CustomSubID != 0 {
		t.Fatalf("组 Token 解析异常: %v %+v", err, tk)
	}
}

// TestRefreshTokenCustomPriority 刷新链接按业务键轮替：自定义优先，组 Token 同步清理。
func TestRefreshTokenCustomPriority(t *testing.T) {
	st, svc, tokenSvc := newTestHomeService(t)
	ctx := context.Background()
	pid := insertPlatform(t, st, "clash", "Clash")
	insertSubscription(t, st, pid, "平台订阅", 1)
	res, err := st.DB().Exec(`INSERT INTO custom_subscriptions (user_id, platform_id) VALUES (11,?)`, pid)
	if err != nil {
		t.Fatalf("插入自定义订阅失败: %v", err)
	}
	customID, _ := res.LastInsertId()
	oldCustom, err := tokenSvc.GetOrCreateUserToken(ctx, 11, pid, customID, 0)
	if err != nil {
		t.Fatalf("预置自定义 Token 失败: %v", err)
	}
	groupTk, err := tokenSvc.GetOrCreateUserToken(ctx, 11, pid, 0, 0)
	if err != nil {
		t.Fatalf("预置组 Token 失败: %v", err)
	}
	tokenValue, err := svc.RefreshToken(ctx, 11, pid)
	if err != nil {
		t.Fatalf("RefreshToken 失败: %v", err)
	}
	if tokenValue == "" || tokenValue == oldCustom.Token || tokenValue == groupTk.Token {
		t.Fatalf("刷新后应返回新的自定义 Token: %s", tokenValue)
	}
	if _, err := tokenSvc.FindByToken(ctx, groupTk.Token); err != token.ErrTokenNotFound {
		t.Fatalf("刷新后历史组 Token 应被清理: %v", err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id=11 AND platform_id=?`, pid).Scan(&count); err != nil {
		t.Fatalf("查询 Token 失败: %v", err)
	}
	if count != 1 {
		t.Fatalf("刷新后应只剩一个自定义 Token，实际 %d", count)
	}
}
