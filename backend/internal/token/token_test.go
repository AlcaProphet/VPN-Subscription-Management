package token

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// testMigrateFS 构造含 users/platforms/download_tokens 等表的迁移集
func testMigrateFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0002_users.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT, oidc_subject TEXT UNIQUE,
			username TEXT NOT NULL, email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('admin','user')),
			group_id INTEGER, password_hash TEXT,
			user_source TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
			credential_version INTEGER NOT NULL DEFAULT 0, oidc_claims TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1004_tokens.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS download_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL,
				custom_sub_id INTEGER,
				subscription_id INTEGER,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE INDEX IF NOT EXISTS idx_dt_user_platform ON download_tokens(user_id, platform_id);
			CREATE TABLE IF NOT EXISTS share_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				share_id INTEGER NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS rule_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				token TEXT NOT NULL UNIQUE,
				rule_id INTEGER NOT NULL,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				refreshed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS access_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER, ip TEXT NOT NULL,
				download_type TEXT NOT NULL, platform TEXT,
				resource_slug TEXT NOT NULL, status TEXT NOT NULL,
				fail_reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1005_business_key.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS custom_subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id INTEGER NOT NULL,
				platform_id INTEGER NOT NULL);
			CREATE TABLE IF NOT EXISTS subscriptions (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				platform_id INTEGER NOT NULL,
				current_version INTEGER NOT NULL DEFAULT 0);`)},
	}
}

// newTestTokenService 临时库 + Token 服务
func newTestTokenService(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), testMigrateFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	svc := NewService(st, log.New("error", "console"))
	return st, svc
}

// TestGetOrCreateReuse 复用键先查后建：同参数二次调用返回同一 Token
func TestGetOrCreateReuse(t *testing.T) {
	_, svc := newTestTokenService(t)
	ctx := context.Background()
	t1, err := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("首次创建失败: %v", err)
	}
	t2, err := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("二次获取失败: %v", err)
	}
	if t1.Token != t2.Token {
		t.Errorf("复用键命中应返回同一 Token: %s != %s", t1.Token, t2.Token)
	}
	var count int
	if err := svc.store.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Errorf("仅应产生一条记录: %d", count)
	}
	// 不同复用键（显式）产生不同 Token
	t3, err := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 99)
	if err != nil {
		t.Fatalf("显式创建失败: %v", err)
	}
	if t3.Token == t1.Token {
		t.Error("不同复用键应产生不同 Token")
	}
}

// TestGetOrCreateConcurrent 并发 N 个 GetOrCreate 只产生一条记录（BEGIN IMMEDIATE 串行化）
func TestGetOrCreateConcurrent(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	const n = 8
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.GetOrCreateUserToken(ctx, 7, 3, 0, 0)
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		if e != nil {
			t.Fatalf("并发获取 %d 失败: %v", i, e)
		}
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Errorf("并发下应仅一条记录: %d", count)
	}
}

// TestRefreshUserToken 轮替：旧 Token 失效、复用键不变
func TestRefreshUserToken(t *testing.T) {
	_, svc := newTestTokenService(t)
	ctx := context.Background()
	orig, err := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	refreshed, err := svc.RefreshUserToken(ctx, orig.Token)
	if err != nil {
		t.Fatalf("刷新失败: %v", err)
	}
	if refreshed.Token == orig.Token {
		t.Error("刷新后 Token 应变化")
	}
	// 旧 Token 不再存在
	if _, err := svc.FindByToken(ctx, orig.Token); err != ErrTokenNotFound {
		t.Errorf("旧 Token 应失效: %v", err)
	}
	// 复用键不变：再取同复用键返回新 Token
	again, err := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("再取失败: %v", err)
	}
	if again.Token != refreshed.Token {
		t.Error("刷新后复用键应指向新 Token")
	}
}

// TestDeleteGroupTokens 删无标识 Token，自定义 Token 保留
func TestDeleteGroupTokens(t *testing.T) {
	_, svc := newTestTokenService(t)
	ctx := context.Background()
	groupTk, err := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("创建无标识失败: %v", err)
	}
	if _, err := svc.GetOrCreateUserToken(ctx, 1, 1, 5, 0); err != nil {
		t.Fatalf("创建自定义失败: %v", err)
	}
	if err := svc.DeleteGroupTokens(ctx, 1, 1); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	if _, err := svc.FindByToken(ctx, groupTk.Token); err != ErrTokenNotFound {
		t.Errorf("无标识 Token 应被删除: %v", err)
	}
	var custom int
	if err := svc.store.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE custom_sub_id IS NOT NULL`).Scan(&custom); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if custom != 1 {
		t.Errorf("自定义 Token 应保留: %d", custom)
	}
}

// TestLifecycleDeletes 生命周期联动：删订阅/删自定义/降级/删用户 各清对应 Token
func TestLifecycleDeletes(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	// 无标识 + 显式 + 自定义
	groupTk, _ := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	explicitTk, _ := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 100)
	customTk, _ := svc.GetOrCreateUserToken(ctx, 1, 1, 50, 0)
	// 删订阅（事务内调用）→ 显式 Token 清除
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		return svc.DeleteBySubscriptionTx(ctx, tx, 100)
	}); err != nil {
		t.Fatalf("删订阅级联失败: %v", err)
	}
	if _, err := svc.FindByToken(ctx, explicitTk.Token); err != ErrTokenNotFound {
		t.Errorf("删订阅后显式 Token 应清除: %v", err)
	}
	// 删自定义（事务内调用）→ 自定义 Token 清除
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		return svc.DeleteByCustomTx(ctx, tx, 50)
	}); err != nil {
		t.Fatalf("删自定义级联失败: %v", err)
	}
	if _, err := svc.FindByToken(ctx, customTk.Token); err != ErrTokenNotFound {
		t.Errorf("删自定义后 Token 应清除: %v", err)
	}
	// 降级（非事务）→ 清显式（当前无显式），无标识保留
	if err := svc.DeleteExplicit(ctx, 1); err != nil {
		t.Fatalf("降级清理失败: %v", err)
	}
	if _, err := svc.FindByToken(ctx, groupTk.Token); err != nil {
		t.Errorf("降级不应清无标识 Token: %v", err)
	}
	// 删用户（事务内调用）→ 全部清除
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		return svc.DeleteAllForUserTx(ctx, tx, 1)
	}); err != nil {
		t.Fatalf("删用户级联失败: %v", err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id = 1`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 0 {
		t.Errorf("删用户后全部 Token 应清除: %d", count)
	}
}

// TestShareRuleTokenRotate 分享/规则 Token：创建与轮替（旧删新写）
func TestShareRuleTokenRotate(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		v1, err := svc.CreateShareTokenTx(ctx, tx, 1)
		if err != nil {
			return err
		}
		v2, err := svc.RotateShareTokenTx(ctx, tx, 1)
		if err != nil {
			return err
		}
		if v1 == v2 {
			return errors.New("轮替后 Token 应变化")
		}
		return nil
	}); err != nil {
		t.Fatalf("分享 Token 轮替失败: %v", err)
	}
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM share_tokens WHERE share_id = 1`).Scan(&count); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 {
		t.Errorf("轮替后应仅一条有效 Token: %d", count)
	}
}

func insertCustomSub(t *testing.T, st *store.Store, userID, platformID int64) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO custom_subscriptions (user_id, platform_id) VALUES (?,?)`, userID, platformID)
	if err != nil {
		t.Fatalf("插入自定义订阅失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

func insertActiveSubscription(t *testing.T, st *store.Store, platformID int64) {
	t.Helper()
	if _, err := st.DB().Exec(`INSERT INTO subscriptions (platform_id, current_version) VALUES (?,1)`, platformID); err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
}

func tokenCount(t *testing.T, st *store.Store, where string, args ...any) int {
	t.Helper()
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE `+where, args...).Scan(&n); err != nil {
		t.Fatalf("统计 Token 失败: %v", err)
	}
	return n
}

// TestResolveUserTokenCustomPriorityCleansGroupToken 自定义订阅优先：
// 旧实现会先创建无标识组 Token 再被自定义覆盖；新业务键解析只保留自定义 Token。
func TestResolveUserTokenCustomPriorityCleansGroupToken(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	groupTk, err := svc.GetOrCreateUserToken(ctx, 1, 1, 0, 0)
	if err != nil {
		t.Fatalf("预置组 Token 失败: %v", err)
	}
	customID := insertCustomSub(t, st, 1, 1)
	tk, err := svc.ResolveUserToken(ctx, 1, 1)
	if err != nil {
		t.Fatalf("ResolveUserToken 失败: %v", err)
	}
	if tk == nil || tk.CustomSubID != customID {
		t.Fatalf("自定义订阅存在时应返回自定义 Token: %+v", tk)
	}
	if _, err := svc.FindByToken(ctx, groupTk.Token); err != ErrTokenNotFound {
		t.Fatalf("历史隐藏组 Token 应被清理: %v", err)
	}
	again, err := svc.ResolveUserToken(ctx, 1, 1)
	if err != nil || again.Token != tk.Token {
		t.Fatalf("重复首页应复用同一自定义 Token: %v %+v", err, again)
	}
	if n := tokenCount(t, st, `user_id=? AND platform_id=?`, 1, 1); n != 1 {
		t.Fatalf("同用户/平台应只保留一个适用 Token，实际 %d", n)
	}
}

// TestResolveUserTokenGroupFallback 无自定义但有激活版本时创建并复用无标识组 Token。
func TestResolveUserTokenGroupFallback(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	insertActiveSubscription(t, st, 1)
	tk, err := svc.ResolveUserToken(ctx, 2, 1)
	if err != nil || tk == nil {
		t.Fatalf("组回退失败: %v %+v", err, tk)
	}
	if tk.CustomSubID != 0 {
		t.Fatalf("无自定义时应返回组 Token: %+v", tk)
	}
	again, err := svc.ResolveUserToken(ctx, 2, 1)
	if err != nil || again.Token != tk.Token {
		t.Fatalf("重复首页应复用组 Token: %v %+v", err, again)
	}
}

// TestResolveUserTokenNoContentDoesNotCreate 无自定义且无激活版本时不得创建 Token，
// 同时清理历史孤儿自定义 Token。
func TestResolveUserTokenNoContentDoesNotCreate(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES ('orphan-custom',1,1,99,NULL)`); err != nil {
		t.Fatalf("预置孤儿自定义 Token 失败: %v", err)
	}
	tk, err := svc.ResolveUserToken(ctx, 1, 1)
	if err != nil || tk != nil {
		t.Fatalf("无适用业务内容时不应创建 Token: %v %+v", err, tk)
	}
	if n := tokenCount(t, st, `user_id=? AND platform_id=?`, 1, 1); n != 0 {
		t.Fatalf("孤儿自定义 Token 应清理，实际 %d", n)
	}
}

// TestResolveUserTokenConcurrent 并发业务键解析只产生一条适用 Token。
func TestResolveUserTokenConcurrent(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	insertCustomSub(t, st, 6, 3)
	const n = 8
	var wg sync.WaitGroup
	tokens := make([]string, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tk, err := svc.ResolveUserToken(ctx, 6, 3)
			if err != nil {
				errs[i] = err
				return
			}
			if tk == nil {
				errs[i] = errors.New("nil token")
				return
			}
			tokens[i] = tk.Token
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("并发解析 %d 失败: %v", i, err)
		}
	}
	for i := 1; i < n; i++ {
		if tokens[i] != tokens[0] {
			t.Fatalf("并发解析应返回同一 Token: %s != %s", tokens[i], tokens[0])
		}
	}
	if got := tokenCount(t, st, `user_id=6 AND platform_id=3`); got != 1 {
		t.Fatalf("并发解析应只产生一条 Token，实际 %d", got)
	}
}

// TestRefreshUserTokenByBusinessKey 轮替前清理历史双 Token：
// 自定义优先时组 Token 消失；无自定义回退到组时孤儿自定义 Token 消失。
func TestRefreshUserTokenByBusinessKey(t *testing.T) {
	st, svc := newTestTokenService(t)
	ctx := context.Background()
	// 自定义优先：预置旧自定义 Token + 历史组 Token
	customID := insertCustomSub(t, st, 7, 1)
	oldCustom, err := svc.GetOrCreateUserToken(ctx, 7, 1, customID, 0)
	if err != nil {
		t.Fatalf("预置自定义 Token 失败: %v", err)
	}
	groupTk, err := svc.GetOrCreateUserToken(ctx, 7, 1, 0, 0)
	if err != nil {
		t.Fatalf("预置组 Token 失败: %v", err)
	}
	rotated, err := svc.RefreshUserTokenByBusinessKey(ctx, 7, 1)
	if err != nil {
		t.Fatalf("自定义轮替失败: %v", err)
	}
	if rotated.CustomSubID != customID || rotated.Token == oldCustom.Token {
		t.Fatalf("自定义优先轮替结果异常: %+v", rotated)
	}
	if _, err := svc.FindByToken(ctx, groupTk.Token); err != ErrTokenNotFound {
		t.Fatalf("历史组 Token 应被清理: %v", err)
	}
	if got := tokenCount(t, st, `user_id=7 AND platform_id=1`); got != 1 {
		t.Fatalf("轮替后应只剩一条 Token，实际 %d", got)
	}
	// 组回退：无自定义 + 激活版本 + 历史孤儿自定义 Token
	st2, svc2 := newTestTokenService(t)
	insertActiveSubscription(t, st2, 2)
	if _, err := st2.DB().Exec(`INSERT INTO download_tokens (token, user_id, platform_id, custom_sub_id, subscription_id) VALUES ('orphan-custom-2',8,2,77,NULL)`); err != nil {
		t.Fatalf("预置孤儿自定义 Token 失败: %v", err)
	}
	rotated2, err := svc2.RefreshUserTokenByBusinessKey(ctx, 8, 2)
	if err != nil || rotated2 == nil || rotated2.CustomSubID != 0 {
		t.Fatalf("组回退轮替失败: %v %+v", err, rotated2)
	}
	var customCount int
	if err := st2.DB().QueryRow(`SELECT COUNT(*) FROM download_tokens WHERE user_id=8 AND platform_id=2 AND custom_sub_id IS NOT NULL`).Scan(&customCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if customCount != 0 {
		t.Fatalf("孤儿自定义 Token 应被清理，实际 %d", customCount)
	}
}
