package emergency

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// emergencyTestFS 应急测试所需全部表（ClearTablesTx 依赖）
var emergencyTestFS = fstest.MapFS{
	"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
		CREATE TABLE IF NOT EXISTS system_config (
		key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL, email TEXT UNIQUE,
		role TEXT NOT NULL DEFAULT 'user', user_source TEXT NOT NULL DEFAULT 'local',
		status TEXT NOT NULL DEFAULT 'active', password_hash TEXT,
		credential_version INTEGER NOT NULL DEFAULT 0, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL UNIQUE,
		is_default INTEGER NOT NULL DEFAULT 0); CREATE TABLE IF NOT EXISTS platforms (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);`)},
	"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
		platform_id INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS versions (
		id INTEGER PRIMARY KEY AUTOINCREMENT, owner_type TEXT NOT NULL, owner_id INTEGER NOT NULL,
		version_no INTEGER NOT NULL, file_path TEXT NOT NULL);`)},
	"1009_xray.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS rule_pools (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
		CREATE TABLE IF NOT EXISTS rule_pool_sources (
		id INTEGER PRIMARY KEY AUTOINCREMENT, pool_id INTEGER NOT NULL, kind TEXT NOT NULL,
		url TEXT, source_mode TEXT NOT NULL DEFAULT 'auto', sort_order INTEGER NOT NULL,
		active_snapshot_id INTEGER, pending_snapshot_id INTEGER);
		CREATE TABLE IF NOT EXISTS pool_source_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT, source_id INTEGER NOT NULL, format TEXT NOT NULL DEFAULT '',
		profile TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'staging');
		CREATE TABLE IF NOT EXISTS pool_canonical_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT, pool_id INTEGER NOT NULL, semantic_key TEXT NOT NULL,
		family TEXT NOT NULL, matcher TEXT NOT NULL, value TEXT NOT NULL, options_json TEXT NOT NULL DEFAULT '{}');
		CREATE TABLE IF NOT EXISTS pool_rule_origins (
		id INTEGER PRIMARY KEY AUTOINCREMENT, pool_id INTEGER NOT NULL, canonical_rule_id INTEGER NOT NULL,
		source_id INTEGER NOT NULL, snapshot_id INTEGER, sort_order INTEGER NOT NULL, raw_line TEXT NOT NULL DEFAULT '', line_no INTEGER NOT NULL DEFAULT 0);
		CREATE TABLE IF NOT EXISTS pool_sync_tasks (
		id INTEGER PRIMARY KEY AUTOINCREMENT, pool_id INTEGER NOT NULL);
		CREATE TABLE IF NOT EXISTS xray_instances (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
		CREATE TABLE IF NOT EXISTS nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT NOT NULL, name TEXT NOT NULL UNIQUE,
		instance_id INTEGER);
		CREATE TABLE IF NOT EXISTS proxy_groups (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
		CREATE TABLE IF NOT EXISTS group_nodes (
		group_id INTEGER NOT NULL, node_id INTEGER NOT NULL, PRIMARY KEY (group_id, node_id));
		CREATE TABLE IF NOT EXISTS xray_users (
		user_id INTEGER NOT NULL, instance_id INTEGER NOT NULL, inbound_tag TEXT NOT NULL,
		node_id INTEGER NOT NULL, PRIMARY KEY (user_id, instance_id, inbound_tag));
		CREATE TABLE IF NOT EXISTS traffic_records (
		user_id INTEGER NOT NULL, ym TEXT NOT NULL, PRIMARY KEY (user_id, ym));
		CREATE TABLE IF NOT EXISTS assembly_blueprints (
		id INTEGER PRIMARY KEY AUTOINCREMENT, version_id INTEGER NOT NULL UNIQUE);
		CREATE TABLE IF NOT EXISTS xray_ext_accounts (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE);
		CREATE TABLE IF NOT EXISTS xray_ext_users (
		ext_account_id INTEGER NOT NULL, instance_id INTEGER NOT NULL, inbound_tag TEXT NOT NULL,
		node_id INTEGER, PRIMARY KEY (ext_account_id, instance_id, inbound_tag));
		CREATE TABLE IF NOT EXISTS xray_ext_traffic (
		ext_account_id INTEGER NOT NULL, ym TEXT NOT NULL, PRIMARY KEY (ext_account_id, ym));`)},
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
		platform_id INTEGER NOT NULL); CREATE TABLE IF NOT EXISTS share_subscriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);`)},
	"1006_rules.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL);`)},
	"0005_reset_tokens.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS password_reset_tokens (
		token TEXT PRIMARY KEY, user_id INTEGER NOT NULL, expires_at TIMESTAMP NOT NULL);`)},
	"0004_oidc.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS oidc_states (
		state TEXT PRIMARY KEY, code_verifier TEXT NOT NULL, intent TEXT NOT NULL DEFAULT '',
		bind_user_id INTEGER, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	"1013_oidc_login_tickets.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS oidc_login_tickets (
		ticket TEXT PRIMARY KEY, session_token TEXT NOT NULL, expires_at TIMESTAMP NOT NULL);`)},
}

// newTestEmergency 创建临时库 + 应急服务（reason/dbReadable 可指定）
func newTestEmergency(t *testing.T, reason TriggerReason, dbReadable bool) (*store.Store, *Service, string) {
	t.Helper()
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), emergencyTestFS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	clearSvc := dataclear.NewService(st, dataDir, log.New("error", "console"))
	svc := NewService(reason, dbReadable, st, cfg, clearSvc, dataDir, "test.db", log.New("error", "console"))
	return st, svc, dataDir
}

// TestDetectManual 手动触发：RESET_ADMIN_PASSWORD 已设置 → manual
func TestDetectManual(t *testing.T) {
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	_ = st.Migrate(context.Background(), emergencyTestFS)
	cfg := config.NewService(st, log.New("error", "console"))
	t.Setenv("RESET_ADMIN_PASSWORD", "1")
	reason, dbReadable := Detect(context.Background(), st, cfg, log.New("error", "console"))
	if reason != TriggerManual || !dbReadable {
		t.Errorf("手动触发异常: reason=%s readable=%v", reason, dbReadable)
	}
}

// TestDetectKeyMissing 自动触发：configured=true 但签名密钥缺失 → key_missing
func TestDetectKeyMissing(t *testing.T) {
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	_ = st.Migrate(context.Background(), emergencyTestFS)
	cfg := config.NewService(st, log.New("error", "console"))
	_ = cfg.Set(context.Background(), config.KeyConfigured, "true") // 签名密钥缺失
	t.Setenv("RESET_ADMIN_PASSWORD", "")
	reason, _ := Detect(context.Background(), st, cfg, log.New("error", "console"))
	if reason != TriggerKeyMissing {
		t.Errorf("关键配置损坏应触发 key_missing: %s", reason)
	}
	// 正常：configured=false 且库完好 → 无触发
	_ = cfg.Set(context.Background(), config.KeyConfigured, "false")
	reason, _ = Detect(context.Background(), st, cfg, log.New("error", "console"))
	if reason != TriggerNone {
		t.Errorf("正常状态不应触发: %s", reason)
	}
}

// TestDetectDBCorrupt 自动触发：数据库不可用（探测失败）→ db_corrupt + 不可读；
// 覆盖 main 的 Open/Migrate 失败分支调用路径（Design1 §3.8）
func TestDetectDBCorrupt(t *testing.T) {
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	_ = st.Migrate(context.Background(), emergencyTestFS)
	_ = st.Close() // 连接关闭：探测失败即视为数据库损坏
	cfg := config.NewService(st, log.New("error", "console"))
	t.Setenv("RESET_ADMIN_PASSWORD", "")
	reason, dbReadable := Detect(context.Background(), st, cfg, log.New("error", "console"))
	if reason != TriggerDBCorrupt || dbReadable {
		t.Errorf("损坏库应触发 db_corrupt 且不可读: reason=%s readable=%v", reason, dbReadable)
	}
}

// TestOpCodeOneTime 操作码一次性：提交（无论成败）即消耗重新生成；8 位字符集断言
func TestOpCodeOneTime(t *testing.T) {
	_, svc, _ := newTestEmergency(t, TriggerManual, true)
	first := svc.opCode
	if len(first) != 8 {
		t.Fatalf("操作码应为 8 位: %q", first)
	}
	charset := "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	for _, r := range first {
		if !strings.ContainsRune(charset, r) {
			t.Errorf("操作码含非法字符 %q", r)
		}
	}
	// 正确提交 → 成功且消耗
	if !svc.VerifyOpCode(first) {
		t.Fatal("正确操作码应通过")
	}
	if svc.opCode == first {
		t.Error("提交后应重新生成操作码")
	}
	// 旧码再用失败
	if svc.VerifyOpCode(first) {
		t.Error("旧操作码应已失效")
	}
	// 错误提交同样消耗并重新生成
	second := svc.opCode
	if svc.VerifyOpCode("WRONGCODE") {
		t.Error("错误操作码应失败")
	}
	if svc.opCode == second {
		t.Error("失败提交后也应重新生成操作码")
	}
}

// TestCapability 能力分级：自动触发仅重初始化；库不可读降级；用户表为空不可重置
func TestCapability(t *testing.T) {
	ctx := context.Background()
	// 自动触发（db_corrupt）→ 不可重置
	_, svc, _ := newTestEmergency(t, TriggerDBCorrupt, false)
	if svc.CanResetPassword(ctx) {
		t.Error("自动触发不应提供重置密码")
	}
	// manual + 库不可读 → 降级（不可重置）
	_, svc2, _ := newTestEmergency(t, TriggerManual, false)
	if svc2.CanResetPassword(ctx) {
		t.Error("库不可读应降级为仅重新初始化")
	}
	// manual + 可读 + 用户表为空 → 不可重置
	_, svc3, _ := newTestEmergency(t, TriggerManual, true)
	if svc3.CanResetPassword(ctx) {
		t.Error("用户表为空不可重置")
	}
}

// TestResetAdminPassword 重置密码：递增 credential_version；非 admin 目标拒绝
func TestResetAdminPassword(t *testing.T) {
	st, svc, _ := newTestEmergency(t, TriggerManual, true)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO users (username, email, role, password_hash) VALUES ('admin-a','a@example.com','admin','x')`); err != nil {
		t.Fatalf("插入管理员失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO users (username, email, role, password_hash) VALUES ('user-a','u@example.com','user','x')`); err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	if err := svc.ResetAdminPassword(ctx, 2, "newpass123"); err == nil {
		t.Error("非 admin 目标应拒绝")
	}
	if err := svc.ResetAdminPassword(ctx, 1, "newpass123"); err != nil {
		t.Fatalf("重置失败: %v", err)
	}
	var cv int
	if err := st.DB().QueryRow(`SELECT credential_version FROM users WHERE id = 1`).Scan(&cv); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if cv != 1 {
		t.Errorf("重置后凭据版本号应递增: %d", cv)
	}
	// 密码已更新（可登录校验经 auth 包，此处断言哈希已替换）
	var hash string
	if err := st.DB().QueryRow(`SELECT password_hash FROM users WHERE id = 1`).Scan(&hash); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if hash == "x" {
		t.Error("密码哈希应已更新")
	}
	// 管理员名单（验码后）
	admins, err := svc.ListAdmins(ctx)
	if err != nil || len(admins) != 1 || !admins[0].HasPassword {
		t.Errorf("管理员名单异常: %+v err=%v", admins, err)
	}
}

// TestReinitializeSQLPath 重初始化 SQL 清空路径（库可读）：数据清空
func TestReinitializeSQLPath(t *testing.T) {
	st, svc, dataDir := newTestEmergency(t, TriggerManual, true)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO users (username, email) VALUES ('u1','u1@example.com')`); err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "contents"), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := svc.Reinitialize(ctx); err != nil {
		t.Fatalf("重新初始化失败: %v", err)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Errorf("SQL 清空后用户应为 0: %d", n)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "contents")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("数据目录应删除: %v", err)
	}
}

// TestReinitializeFilePath 重初始化删文件路径（库损坏）：数据库文件与数据目录删除
func TestReinitializeFilePath(t *testing.T) {
	_, svc, dataDir := newTestEmergency(t, TriggerDBCorrupt, false)
	ctx := context.Background()
	dbPath := filepath.Join(dataDir, "test.db")
	if err := os.WriteFile(dbPath, []byte("corrupted"), 0o644); err != nil {
		t.Fatalf("写损坏库文件失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "public"), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := svc.Reinitialize(ctx); err != nil {
		t.Fatalf("重新初始化失败: %v", err)
	}
	if _, err := os.Stat(dbPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("数据库文件应删除: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "public")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("public 目录应删除: %v", err)
	}
}
