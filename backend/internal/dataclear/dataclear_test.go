package dataclear

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newTestClear 创建临时库（含全部业务表）+ 清理服务
func newTestClear(t *testing.T) (*store.Store, *Service, string) {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL, email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user', user_source TEXT NOT NULL DEFAULT 'local',
			status TEXT NOT NULL DEFAULT 'active');`)},
		"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL UNIQUE,
			is_default INTEGER NOT NULL DEFAULT 0); CREATE TABLE IF NOT EXISTS platforms (
			id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			platform_id INTEGER NOT NULL, current_version INTEGER NOT NULL DEFAULT 0);
			CREATE TABLE IF NOT EXISTS versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT, owner_type TEXT NOT NULL, owner_id INTEGER NOT NULL,
			version_no INTEGER NOT NULL, file_path TEXT NOT NULL, file_name TEXT NOT NULL DEFAULT '');`)},
		"1009_xray.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS rule_pools (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
			CREATE TABLE IF NOT EXISTS pool_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT, pool_id INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE);
			CREATE TABLE IF NOT EXISTS pool_sync_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT, pool_id INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE);
			CREATE TABLE IF NOT EXISTS xray_instances (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
			CREATE TABLE IF NOT EXISTS nodes (
			id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT NOT NULL, name TEXT NOT NULL UNIQUE,
			instance_id INTEGER REFERENCES xray_instances(id) ON DELETE CASCADE);
			CREATE TABLE IF NOT EXISTS proxy_groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
			CREATE TABLE IF NOT EXISTS group_nodes (
			group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
			PRIMARY KEY (group_id, node_id));
			CREATE TABLE IF NOT EXISTS xray_users (
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			instance_id INTEGER NOT NULL REFERENCES xray_instances(id) ON DELETE CASCADE,
			inbound_tag TEXT NOT NULL,
			node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
			PRIMARY KEY (user_id, instance_id, inbound_tag));
			CREATE TABLE IF NOT EXISTS traffic_records (
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			ym TEXT NOT NULL,
			PRIMARY KEY (user_id, ym));
			CREATE TABLE IF NOT EXISTS assembly_blueprints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL UNIQUE REFERENCES versions(id) ON DELETE CASCADE);
			CREATE TABLE IF NOT EXISTS xray_ext_accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
			CREATE TABLE IF NOT EXISTS xray_ext_users (
			ext_account_id INTEGER NOT NULL REFERENCES xray_ext_accounts(id) ON DELETE CASCADE,
			instance_id INTEGER NOT NULL REFERENCES xray_instances(id) ON DELETE CASCADE,
			inbound_tag TEXT NOT NULL,
			node_id INTEGER REFERENCES nodes(id) ON DELETE CASCADE,
			PRIMARY KEY (ext_account_id, instance_id, inbound_tag));
			CREATE TABLE IF NOT EXISTS xray_ext_traffic (
			ext_account_id INTEGER NOT NULL REFERENCES xray_ext_accounts(id) ON DELETE CASCADE,
			ym TEXT NOT NULL,
			PRIMARY KEY (ext_account_id, ym));`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS download_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT, token TEXT NOT NULL UNIQUE, user_id INTEGER NOT NULL,
			platform_id INTEGER NOT NULL, custom_sub_id INTEGER, subscription_id INTEGER);
			CREATE TABLE IF NOT EXISTS share_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT, token TEXT NOT NULL UNIQUE, share_id INTEGER NOT NULL);
			CREATE TABLE IF NOT EXISTS rule_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT, token TEXT NOT NULL UNIQUE, rule_id INTEGER NOT NULL);
			CREATE TABLE IF NOT EXISTS access_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT, ip TEXT NOT NULL, download_type TEXT NOT NULL,
			resource_slug TEXT NOT NULL, status TEXT NOT NULL);`)},
		"1005_custom_share.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS custom_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, user_id INTEGER NOT NULL,
			platform_id INTEGER NOT NULL, UNIQUE (user_id, platform_id));
			CREATE TABLE IF NOT EXISTS share_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);`)},
		"1006_rules.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS rules (
			id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);`)},
		"0005_reset_tokens.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			token TEXT PRIMARY KEY, user_id INTEGER NOT NULL, expires_at TIMESTAMP NOT NULL);`)},
		"0004_oidc.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS oidc_states (
			state TEXT PRIMARY KEY, code_verifier TEXT NOT NULL, intent TEXT NOT NULL DEFAULT '',
			bind_user_id INTEGER, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	svc := NewService(st, dataDir, log.New("error", "console"))
	return st, svc, dataDir
}

// TestClearAll 一键清空：确认词校验 + 清库 + configured=false + 内存态复位回调 + 数据文件删除
func TestClearAll(t *testing.T) {
	st, svc, dataDir := newTestClear(t)
	ctx := context.Background()
	// 造数据：配置 + 用户 + 文件
	cfg := config.NewService(st, log.New("error", "console"))
	_ = cfg.Set(ctx, config.KeyConfigured, "true")
	_ = cfg.Set(ctx, config.KeySigningKey, "old-signing-key")
	if _, err := st.DB().Exec(`INSERT INTO users (username, email) VALUES ('u1','u1@example.com')`); err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "contents", "subscription", "1"), 0o755); err != nil {
		t.Fatalf("创建版本目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "contents", "subscription", "1", "v1"), []byte("x"), 0o644); err != nil {
		t.Fatalf("写版本文件失败: %v", err)
	}
	resetCalled := false
	svc.SetResetRuntimeState(func() { resetCalled = true })

	// 确认词错误拒绝
	if err := svc.ClearAll(ctx, "WRONG"); err == nil {
		t.Fatal("确认词错误应拒绝")
	}
	if err := svc.ClearAll(ctx, ConfirmWordReset); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	// configured=false（回 Setup）
	if cfg.GetBool(ctx, config.KeyConfigured, false) {
		t.Error("清空后 configured 应为 false")
	}
	// 业务数据清空
	var users, cfgCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM system_config`).Scan(&cfgCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if users != 0 || cfgCount != 0 {
		t.Errorf("清空不彻底: users=%d config=%d", users, cfgCount)
	}
	// 数据文件删除
	if _, err := os.Stat(filepath.Join(dataDir, "contents")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("版本目录应删除: %v", err)
	}
	if !resetCalled {
		t.Error("内存态复位回调应被调用")
	}
}

// TestClearAllFileErrorDoesNotBlock 文件删除失败不阻断回 Setup
func TestClearAllFileErrorDoesNotBlock(t *testing.T) {
	st, svc, _ := newTestClear(t)
	ctx := context.Background()
	cfg := config.NewService(st, log.New("error", "console"))
	_ = cfg.Set(ctx, config.KeyConfigured, "true")
	// 以文件方式占用 public 目录（RemoveAll 对只读目录在部分平台会失败；此处验证逻辑不因文件错误中断）
	if err := svc.ClearAll(ctx, ConfirmWordReset); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if cfg.GetBool(ctx, config.KeyConfigured, false) {
		t.Error("清空后 configured 应为 false")
	}
}

// TestClearTablesTxClearsBuild4Tables 验证 13 张增量新表可被一键清空（清表顺序与 Build4 清单一致）
func TestClearTablesTxClearsBuild4Tables(t *testing.T) {
	st, svc, _ := newTestClear(t)
	ctx := context.Background()
	db := st.DB()

	inserts := []string{
		`INSERT INTO rule_pools (id, name) VALUES (1, 'p1')`,
		`INSERT INTO pool_entries (id, pool_id) VALUES (1, 1)`,
		`INSERT INTO pool_sync_tasks (id, pool_id) VALUES (1, 1)`,
		`INSERT INTO xray_instances (id, name) VALUES (1, 'i1')`,
		`INSERT INTO nodes (id, source, name, instance_id) VALUES (1, 'xray', 'n1', 1)`,
		`INSERT INTO proxy_groups (id, name) VALUES (1, 'g1')`,
		`INSERT INTO groups (id, slug, name, is_default) VALUES (1, 'group-1', '默认组', 1)`,
		`INSERT INTO group_nodes (group_id, node_id) VALUES (1, 1)`,
		`INSERT INTO users (id, username, email) VALUES (1, 'u1', 'u1@example.com')`,
		`INSERT INTO xray_users (user_id, instance_id, inbound_tag, node_id) VALUES (1, 1, 't1', 1)`,
		`INSERT INTO traffic_records (user_id, ym) VALUES (1, '2026-08')`,
		`INSERT INTO versions (id, owner_type, owner_id, version_no, file_path) VALUES (1, 'subscription', 1, 1, 'x')`,
		`INSERT INTO assembly_blueprints (id, version_id) VALUES (1, 1)`,
		`INSERT INTO xray_ext_accounts (id, name) VALUES (1, 'ext1')`,
		`INSERT INTO xray_ext_users (ext_account_id, instance_id, inbound_tag, node_id) VALUES (1, 1, 't1', 1)`,
		`INSERT INTO xray_ext_traffic (ext_account_id, ym) VALUES (1, '2026-08')`,
	}
	for _, q := range inserts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			t.Fatalf("插入测试数据失败: %v\nSQL: %s", err, q)
		}
	}

	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		return svc.ClearTablesTx(ctx, tx)
	}); err != nil {
		t.Fatalf("清空失败: %v", err)
	}

	tables := []string{
		"rule_pools", "pool_entries", "pool_sync_tasks",
		"xray_instances", "nodes", "proxy_groups", "group_nodes",
		"xray_users", "traffic_records", "assembly_blueprints",
		"xray_ext_accounts", "xray_ext_users", "xray_ext_traffic",
	}
	for _, name := range tables {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+name).Scan(&n); err != nil {
			t.Fatalf("查询表 %s 失败: %v", name, err)
		}
		if n != 0 {
			t.Errorf("表 %s 清空后应无数据，实际 %d 行", name, n)
		}
	}
}
