package user

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
	"vpn-sub/internal/version"
)

// newTestAdminService 创建临时库 + 管理服务（含用户管理所需的全部表）
func newTestAdminService(t *testing.T) (*store.Store, *AdminService, *token.Service, *auth.ResetService, *version.Service) {
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
		"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			oidc_subject TEXT UNIQUE,
			username TEXT NOT NULL,
			email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			group_id INTEGER,
			password_hash TEXT,
			user_source TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
			credential_version INTEGER NOT NULL DEFAULT 0,
			oidc_claims TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL UNIQUE,
			is_default INTEGER NOT NULL DEFAULT 0, needs_reselect INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
			id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '', schemes TEXT NOT NULL DEFAULT '[]',
			extra_headers TEXT NOT NULL DEFAULT '{}', installer_files TEXT NOT NULL DEFAULT '[]', installer_urls TEXT NOT NULL DEFAULT '[]',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1002_subscriptions_versions.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			owner_type TEXT NOT NULL, owner_id INTEGER NOT NULL, version_no INTEGER NOT NULL,
			file_path TEXT NOT NULL, file_name TEXT NOT NULL DEFAULT '', created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (owner_type, owner_id, version_no));`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS download_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token TEXT NOT NULL UNIQUE, user_id INTEGER NOT NULL, platform_id INTEGER NOT NULL,
			custom_sub_id INTEGER, subscription_id INTEGER,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1005_custom_share.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS custom_subscriptions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE, user_id INTEGER NOT NULL, platform_id INTEGER NOT NULL,
			current_version INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, platform_id));`)},
		"0005_reset_tokens.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS password_reset_tokens (
			token TEXT PRIMARY KEY, user_id INTEGER NOT NULL,
			expires_at TIMESTAMP NOT NULL, used INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	lg := log.New("error", "console")
	users := NewService(st, cfg, lg)
	tokenSvc := token.NewService(st, lg)
	resetSvc := auth.NewResetService(st, users, lg)
	dataDir := t.TempDir()
	verSvc := version.NewService(st, dataDir, lg)
	adminSvc := NewAdminService(st, users, tokenSvc, resetSvc, cfg, verSvc, lg)
	return st, adminSvc, tokenSvc, resetSvc, verSvc
}

// seedUser 直接插入用户并返回 ID（role/status 可指定；pwdHash 空串存 NULL = 无密码）
func seedUser(t *testing.T, db *sql.DB, username, email, role, status string, pwdHash string) int64 {
	t.Helper()
	var pwd any
	if pwdHash != "" {
		pwd = pwdHash
	}
	res, err := db.Exec(`INSERT INTO users (username, email, password_hash, role, user_source, status)
		VALUES (?,?,?,?,?,?)`, username, email, pwd, role, "local", status)
	if err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// seedAdminPair 管理员 + 普通用户各一（管理员保护测试基底）
func seedAdminPair(t *testing.T, db *sql.DB) (adminID, userID int64) {
	t.Helper()
	adminID = seedUser(t, db, "admin-a", "admin-a@example.com", "admin", "active", "x")
	userID = seedUser(t, db, "user-a", "user-a@example.com", "user", "active", "x")
	return
}

// TestAdminSelfOperation 五重保护：删自己/改自己角色/禁用自己/重置自己密码 → ErrSelfOperation
func TestAdminSelfOperation(t *testing.T) {
	st, adminSvc, _, _, _ := newTestAdminService(t)
	ctx := context.Background()
	adminID, _ := seedAdminPair(t, st.DB())

	if err := adminSvc.Delete(ctx, adminID, adminID); !errors.Is(err, ErrSelfOperation) {
		t.Errorf("删自己应拒绝: %v", err)
	}
	if err := adminSvc.ChangeRole(ctx, adminID, adminID, "user"); !errors.Is(err, ErrSelfOperation) {
		t.Errorf("改自己角色应拒绝: %v", err)
	}
	if err := adminSvc.SetStatus(ctx, adminID, adminID, true); !errors.Is(err, ErrSelfOperation) {
		t.Errorf("禁用自己应拒绝: %v", err)
	}
	if _, err := adminSvc.ResetPasswordDirect(ctx, adminID, adminID); !errors.Is(err, ErrSelfOperation) {
		t.Errorf("重置自己密码应拒绝: %v", err)
	}
	if err := adminSvc.ResetPasswordByEmail(ctx, adminID, adminID); !errors.Is(err, ErrSelfOperation) {
		t.Errorf("邮件重置自己密码应拒绝: %v", err)
	}
}

// TestAdminLastAdmin 五重保护：删最后管理员/降级最后活跃管理员/禁用最后活跃管理员 → ErrLastAdmin
func TestAdminLastAdmin(t *testing.T) {
	st, adminSvc, _, _, _ := newTestAdminService(t)
	ctx := context.Background()
	adminID, userID := seedAdminPair(t, st.DB())

	if err := adminSvc.Delete(ctx, userID, adminID); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("删最后管理员应拒绝: %v", err)
	}
	if err := adminSvc.ChangeRole(ctx, userID, adminID, "user"); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("降级最后活跃管理员应拒绝: %v", err)
	}
	if err := adminSvc.SetStatus(ctx, userID, adminID, true); !errors.Is(err, ErrLastAdmin) {
		t.Errorf("禁用最后活跃管理员应拒绝: %v", err)
	}
}

// TestAdminChangeRoleDowngradeClearsExplicitTokens 降级（admin→user）清显式 Token、保留无标识 Token
func TestAdminChangeRoleDowngradeClearsExplicitTokens(t *testing.T) {
	st, adminSvc, tokenSvc, _, _ := newTestAdminService(t)
	ctx := context.Background()
	adminID, _ := seedAdminPair(t, st.DB())
	targetID := seedUser(t, st.DB(), "admin-b", "admin-b@example.com", "admin", "active", "x")

	// 目标用户显式 Token（subscription_id 非空）+ 无标识 Token（复用键 user+platform）
	if _, err := tokenSvc.GetOrCreateUserToken(ctx, targetID, 1, 0, 99); err != nil {
		t.Fatalf("创建显式 Token 失败: %v", err)
	}
	if _, err := tokenSvc.GetOrCreateUserToken(ctx, targetID, 2, 0, 0); err != nil {
		t.Fatalf("创建无标识 Token 失败: %v", err)
	}

	if err := adminSvc.ChangeRole(ctx, adminID, targetID, "user"); err != nil {
		t.Fatalf("降级失败: %v", err)
	}
	var explicit, plain int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id = ? AND subscription_id IS NOT NULL`, targetID).Scan(&explicit); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id = ? AND subscription_id IS NULL`, targetID).Scan(&plain); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if explicit != 0 || plain != 1 {
		t.Errorf("降级后显式 Token 应全删、无标识保留: explicit=%d plain=%d", explicit, plain)
	}
	// 角色已变更
	var role string
	if err := st.DB().QueryRow(`SELECT role FROM users WHERE id = ?`, targetID).Scan(&role); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if role != "user" {
		t.Errorf("角色应为 user: %s", role)
	}
}

// TestAdminDisableBumpsCredentialAndClearsTokens 禁用 = 同事务递增 credential_version + Token 全删；启用不恢复
func TestAdminDisableBumpsCredentialAndClearsTokens(t *testing.T) {
	st, adminSvc, tokenSvc, _, _ := newTestAdminService(t)
	ctx := context.Background()
	adminID, userID := seedAdminPair(t, st.DB())

	if _, err := tokenSvc.GetOrCreateUserToken(ctx, userID, 1, 0, 0); err != nil {
		t.Fatalf("创建 Token 失败: %v", err)
	}
	if err := adminSvc.SetStatus(ctx, adminID, userID, true); err != nil {
		t.Fatalf("禁用失败: %v", err)
	}
	var cv, tokens int
	if err := st.DB().QueryRow(`SELECT credential_version FROM users WHERE id = ?`, userID).Scan(&cv); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id = ?`, userID).Scan(&tokens); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if cv != 1 || tokens != 0 {
		t.Errorf("禁用应递增凭据版本号并清 Token: cv=%d tokens=%d", cv, tokens)
	}
	// 启用：状态恢复但 Token 不恢复
	if err := adminSvc.SetStatus(ctx, adminID, userID, false); err != nil {
		t.Fatalf("启用失败: %v", err)
	}
	var status string
	if err := st.DB().QueryRow(`SELECT status FROM users WHERE id = ?`, userID).Scan(&status); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if status != "active" {
		t.Errorf("启用后应为 active: %s", status)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id = ?`, userID).Scan(&tokens); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if tokens != 0 {
		t.Errorf("启用后不应恢复 Token: %d", tokens)
	}
}

// TestAdminResetPasswordDirect 直接重置：8 位字符集断言 + 凭据版本号递增（旧会话失效）
func TestAdminResetPasswordDirect(t *testing.T) {
	st, adminSvc, _, _, _ := newTestAdminService(t)
	ctx := context.Background()
	adminID, userID := seedAdminPair(t, st.DB())

	pwd, err := adminSvc.ResetPasswordDirect(ctx, adminID, userID)
	if err != nil {
		t.Fatalf("直接重置失败: %v", err)
	}
	if len(pwd) != 8 {
		t.Fatalf("密码应为 8 位: %q", pwd)
	}
	charset := "abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ23456789"
	for _, r := range pwd {
		if !strings.ContainsRune(charset, r) {
			t.Errorf("密码含非法字符 %q", r)
		}
	}
	for _, bad := range "iIoO0lL" {
		if strings.ContainsRune(pwd, bad) {
			t.Errorf("密码不应含易混淆字符 %q", bad)
		}
	}
	var cv int
	if err := st.DB().QueryRow(`SELECT credential_version FROM users WHERE id = ?`, userID).Scan(&cv); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if cv != 1 {
		t.Errorf("重置后凭据版本号应递增: %d", cv)
	}
}

// seedSMTP 配置 SMTP 三键（经 config.Set 加密落库，与生产同路径）
func seedSMTP(t *testing.T, st *store.Store) {
	t.Helper()
	cfg := config.NewService(st, log.New("error", "console"))
	ctx := context.Background()
	for k, v := range map[string]string{"smtp_host": "smtp.example.com", "smtp_user": "user@example.com", "smtp_password": "secret"} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置 SMTP 失败: %v", err)
		}
	}
}

// TestAdminResetPendingRejected 待审批账号拒绝重置（直接与邮件两种方式）
func TestAdminResetPendingRejected(t *testing.T) {
	st, adminSvc, _, _, _ := newTestAdminService(t)
	ctx := context.Background()
	seedSMTP(t, st)
	adminID := seedUser(t, st.DB(), "admin-a", "admin-a@example.com", "admin", "active", "x")
	pendingID := seedUser(t, st.DB(), "pending-a", "pending-a@example.com", "user", "pending", "x")

	if _, err := adminSvc.ResetPasswordDirect(ctx, adminID, pendingID); !errors.Is(err, ErrPendingNotAllowed) {
		t.Errorf("待审批直接重置应拒绝: %v", err)
	}
	if err := adminSvc.ResetPasswordByEmail(ctx, adminID, pendingID); !errors.Is(err, ErrPendingNotAllowed) {
		t.Errorf("待审批邮件重置应拒绝: %v", err)
	}
}

// TestAdminBatchSendLinks 批量发链接筛选：合格/待审批/禁用/无邮箱四类计数；SMTP 未配置拒绝
func TestAdminBatchSendLinks(t *testing.T) {
	st, adminSvc, _, _, _ := newTestAdminService(t)
	ctx := context.Background()
	seedUser(t, st.DB(), "ok", "ok@example.com", "user", "active", "")
	seedUser(t, st.DB(), "pending", "pending@example.com", "user", "pending", "")
	seedUser(t, st.DB(), "disabled", "disabled@example.com", "user", "disabled", "")
	if _, err := st.DB().Exec(`INSERT INTO users (username, role, user_source, status) VALUES (?,?,?,?)`,
		"no-email", "user", "oidc", "active"); err != nil {
		t.Fatalf("插入无邮箱用户失败: %v", err)
	}

	// SMTP 未配置：返回错误（前端置灰依据）
	if _, _, _, _, err := adminSvc.BatchSendPasswordLinks(ctx); !errors.Is(err, ErrSMTPNotConfigured) {
		t.Errorf("SMTP 未配置应拒绝: %v", err)
	}
	seedSMTP(t, st)

	sent, skippedPending, skippedDisabled, skippedNoEmail, err := adminSvc.BatchSendPasswordLinks(ctx)
	if err != nil {
		t.Fatalf("批量发送失败: %v", err)
	}
	if sent != 1 || skippedPending != 1 || skippedDisabled != 1 || skippedNoEmail != 1 {
		t.Errorf("筛选计数异常: sent=%d pending=%d disabled=%d noemail=%d",
			sent, skippedPending, skippedDisabled, skippedNoEmail)
	}
}

// TestAdminDeleteUserCascade 删除用户级联：Token/自定义订阅/版本文件无残留；邮箱释放后可重新注册
func TestAdminDeleteUserCascade(t *testing.T) {
	st, adminSvc, tokenSvc, _, verSvc := newTestAdminService(t)
	ctx := context.Background()
	adminID, userID := seedAdminPair(t, st.DB())

	// 给目标用户创建 Token + 自定义订阅（含版本文件）
	if _, err := tokenSvc.GetOrCreateUserToken(ctx, userID, 1, 0, 0); err != nil {
		t.Fatalf("创建 Token 失败: %v", err)
	}
	customID := seedCustom(t, st, userID)
	if _, err := verSvc.CreateVersion(ctx, "custom", customID, version.BytesContent([]byte("proxies: []"))); err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}
	// 版本文件已落盘
	verFile := filepath.Join(verSvc.ContentsRoot(), "custom", strconv.FormatInt(customID, 10), "v1")
	if _, err := os.Stat(verFile); err != nil {
		t.Fatalf("版本文件应存在: %v", err)
	}

	if err := adminSvc.Delete(ctx, adminID, userID); err != nil {
		t.Fatalf("删除用户失败: %v", err)
	}
	var tokens, customs int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id = ?`, userID).Scan(&tokens); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM custom_subscriptions WHERE user_id = ?`, userID).Scan(&customs); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if tokens != 0 || customs != 0 {
		t.Errorf("级联删除不彻底: tokens=%d customs=%d", tokens, customs)
	}
	// 邮箱释放：可重新注册
	if _, err := st.DB().Exec(`SELECT COUNT(*) FROM users WHERE email = 'user-a@example.com'`); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE email = 'user-a@example.com'`).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Errorf("邮箱应释放: %d", n)
	}
	// 版本文件级联删除
	if _, err := os.Stat(verFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("版本文件应级联删除: %v", err)
	}
}

// seedCustom 插入自定义订阅记录（不创建版本文件）
func seedCustom(t *testing.T, st *store.Store, userID int64) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO custom_subscriptions (slug, user_id, platform_id) VALUES (?,?,?)`,
		"custom-"+strings.Repeat("a", 8)+"1", userID, 1)
	if err != nil {
		t.Fatalf("插入自定义订阅失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestAdminCreateAndFillEmail 新建用户 + 无邮箱补填
func TestAdminCreateAndFillEmail(t *testing.T) {
	st, adminSvc, _, _, _ := newTestAdminService(t)
	ctx := context.Background()
	adminID := seedUser(t, st.DB(), "admin-a", "admin-a@example.com", "admin", "active", "x")

	u, err := adminSvc.Create(ctx, "newbie", "newbie@example.com", "password123")
	if err != nil {
		t.Fatalf("新建用户失败: %v", err)
	}
	if u.Role != "user" || u.Status != "active" || u.Source != "local" {
		t.Errorf("新建用户属性异常: %+v", u)
	}
	// 邮箱冲突 409
	if _, err := adminSvc.Create(ctx, "dup", "newbie@example.com", "password123"); !errors.Is(err, ErrEmailConflict) {
		t.Errorf("邮箱冲突应 409: %v", err)
	}
	// 无邮箱用户补填：先冲突（对另一无邮箱用户填已占用邮箱），再正常补填，再重复补填拒绝
	noEmailID := seedUser(t, st.DB(), "no-email", "", "user", "active", "")
	if err := adminSvc.FillEmail(ctx, noEmailID, "newbie@example.com"); !errors.Is(err, ErrEmailConflict) {
		t.Errorf("补填冲突邮箱应拒绝: %v", err)
	}
	if err := adminSvc.FillEmail(ctx, noEmailID, "FILLED@example.com"); err != nil {
		t.Fatalf("补填邮箱失败: %v", err)
	}
	if err := adminSvc.FillEmail(ctx, noEmailID, "another@example.com"); err == nil {
		t.Error("已有邮箱的用户补填应拒绝")
	}
	// 删除用户级联保护：删自己拒绝（复用 adminID）
	if err := adminSvc.Delete(ctx, adminID, adminID); !errors.Is(err, ErrSelfOperation) {
		t.Errorf("删自己应拒绝: %v", err)
	}
}

// TestAdminList 分页 + 模糊搜索 + 字段完整性
func TestAdminList(t *testing.T) {
	st, adminSvc, _, _, _ := newTestAdminService(t)
	ctx := context.Background()
	seedUser(t, st.DB(), "kyle", "kyle@example.com", "admin", "active", "x")
	seedUser(t, st.DB(), "bob", "bob@example.com", "user", "active", "")
	seedUser(t, st.DB(), "carol", "carol@example.com", "user", "disabled", "")

	list, total, err := adminSvc.List(ctx, ListQuery{Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Errorf("列表总数异常: total=%d len=%d", total, len(list))
	}
	// 模糊搜索邮箱
	list, total, err = adminSvc.List(ctx, ListQuery{Page: 1, Size: 20, Keyword: "kyle"})
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}
	if total != 1 || list[0].Username != "kyle" {
		t.Errorf("关键词搜索异常: total=%d", total)
	}
	// 分页
	list, total, err = adminSvc.List(ctx, ListQuery{Page: 1, Size: 2})
	if err != nil {
		t.Fatalf("分页查询失败: %v", err)
	}
	if len(list) != 2 || total != 3 {
		t.Errorf("分页异常: len=%d total=%d", len(list), total)
	}
	// 字段：has_password / custom_platforms 空切片
	var hasPwd bool
	for _, u := range list {
		if u.Username == "kyle" {
			hasPwd = u.HasPassword
		}
	}
	if !hasPwd {
		t.Error("kyle 应有密码标记")
	}
	// 无自定义订阅的用户 custom_subs 必须为 [] 而非 null（前端 .map 安全守护）
	for _, u := range list {
		if u.CustomSubs == nil {
			t.Errorf("用户 %s 的 custom_subs 应为空切片而非 nil（序列化后为 null）", u.Username)
		}
	}
}
