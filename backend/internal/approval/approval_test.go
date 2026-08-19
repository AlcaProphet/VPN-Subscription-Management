package approval

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// mockMail 记录调用并以可配置错误返回（SMTP 失败不阻断验证）；
// scopes 模拟 mail.Service.ScopeEnabled——nil/未含项表示该类型邮件未启用（不记录调用）
type mockMail struct {
	welcomeBodies []string
	notifyCalls   []bool // approved 值
	failSend      bool
	scopes        map[string]bool
}

func (m *mockMail) SendWelcome(ctx context.Context, to, siteName, loginURL, source string) error {
	if !m.scopes["welcome"] {
		return nil // scope 未启用不发送
	}
	if source == "oidc" {
		m.welcomeBodies = append(m.welcomeBodies, "单点登录")
	} else {
		m.welcomeBodies = append(m.welcomeBodies, "邮箱与密码")
	}
	if m.failSend {
		return errors.New("模拟发送失败")
	}
	return nil
}

func (m *mockMail) SendApprovalNotify(ctx context.Context, to, siteName string, approved bool) error {
	if !m.scopes["approval_notify"] {
		return nil // scope 未启用不发送
	}
	m.notifyCalls = append(m.notifyCalls, approved)
	if m.failSend {
		return errors.New("模拟发送失败")
	}
	return nil
}

// newTestApproval 创建临时库 + 审批服务（mock mail）
func newTestApproval(t *testing.T, failSend bool) (*store.Store, *Service, *mockMail) {
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
			oidc_subject TEXT UNIQUE, username TEXT NOT NULL, email TEXT UNIQUE,
			role TEXT NOT NULL DEFAULT 'user', group_id INTEGER, password_hash TEXT,
			user_source TEXT NOT NULL CHECK (user_source IN ('oidc','local','selfreg')),
			status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','active','disabled')),
			credential_version INTEGER NOT NULL DEFAULT 0, oidc_claims TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	_ = cfg.Set(context.Background(), "site_name", "测试站点")
	_ = cfg.Set(context.Background(), "frontend_url", "https://vpn.example.com")
	mm := &mockMail{failSend: failSend}
	svc := NewService(st, mm, cfg, log.New("error", "console"))
	return st, svc, mm
}

// seedPending 插入待审批用户（oidc/selfreg 两种来源）
func seedPending(t *testing.T, st *store.Store, username, email, source string, claims string) int64 {
	t.Helper()
	res, err := st.DB().Exec(`INSERT INTO users (username, email, role, user_source, status, oidc_claims)
		VALUES (?,?,?,?,?,?)`, username, email, "user", source, "pending", claims)
	if err != nil {
		t.Fatalf("插入待审批用户失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestApproveActivatesAndClearsClaims 通过：激活 + 清 claims；欢迎邮件按来源区分文案
func TestApproveActivatesAndClearsClaims(t *testing.T) {
	st, svc, mm := newTestApproval(t, false)
	mm.scopes = map[string]bool{"welcome": true}
	ctx := context.Background()
	oidcID := seedPending(t, st, "alice", "alice@example.com", "oidc", `{"sub":"s1"}`)
	selfID := seedPending(t, st, "bob", "bob@example.com", "selfreg", "")

	if err := svc.Approve(ctx, oidcID); err != nil {
		t.Fatalf("通过失败: %v", err)
	}
	if err := svc.Approve(ctx, selfID); err != nil {
		t.Fatalf("通过失败: %v", err)
	}
	var status, claims string
	if err := st.DB().QueryRow(`SELECT status, COALESCE(oidc_claims,'') FROM users WHERE id = ?`, oidcID).Scan(&status, &claims); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if status != "active" || claims != "" {
		t.Errorf("通过后应激活且清空 claims: status=%s claims=%q", status, claims)
	}
	// 欢迎邮件按来源区分文案（oidc → 单点登录；selfreg → 邮箱与密码）
	if len(mm.welcomeBodies) != 2 || mm.welcomeBodies[0] != "单点登录" || mm.welcomeBodies[1] != "邮箱与密码" {
		t.Errorf("欢迎邮件按来源区分文案异常: %v", mm.welcomeBodies)
	}
}

// TestRejectDeletesAndReleasesEmail 拒绝：账号删除、邮箱释放；claims 随账号删除
func TestRejectDeletesAndReleasesEmail(t *testing.T) {
	st, svc, mm := newTestApproval(t, false)
	mm.scopes = map[string]bool{"approval_notify": true}
	ctx := context.Background()
	id := seedPending(t, st, "carol", "carol@example.com", "oidc", `{"sub":"s2"}`)

	if err := svc.Reject(ctx, id); err != nil {
		t.Fatalf("拒绝失败: %v", err)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM users WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Errorf("拒绝后账号应删除: %d", n)
	}
	if len(mm.notifyCalls) != 1 || mm.notifyCalls[0] != false {
		t.Errorf("拒绝应触发拒绝通知: %v", mm.notifyCalls)
	}
	// 邮箱释放：同邮箱可重新插入（唯一约束不冲突）
	if _, err := st.DB().Exec(`INSERT INTO users (username, email, role, user_source, status)
		VALUES ('carol2', 'carol@example.com', 'user', 'selfreg', 'pending')`); err != nil {
		t.Errorf("邮箱应释放可重新注册: %v", err)
	}
}

// TestBatchApproveCounts 批量通过：部分失败回执计数正确（不存在的 id 计失败）
func TestBatchApproveCounts(t *testing.T) {
	st, svc, _ := newTestApproval(t, false)
	ctx := context.Background()
	id1 := seedPending(t, st, "d1", "d1@example.com", "selfreg", "")
	id2 := seedPending(t, st, "d2", "d2@example.com", "selfreg", "")

	succeeded, failed, err := svc.BatchApprove(ctx, []int64{id1, 99999, id2})
	if err != nil {
		t.Fatalf("批量通过失败: %v", err)
	}
	if succeeded != 2 || failed != 1 {
		t.Errorf("批量通过计数异常: succeeded=%d failed=%d", succeeded, failed)
	}
}

// TestSMTPFailureDoesNotBlock SMTP 失败不阻断：注入发送失败 → Approve/Reject 仍成功
func TestSMTPFailureDoesNotBlock(t *testing.T) {
	st, svc, mm := newTestApproval(t, true)
	mm.scopes = map[string]bool{"welcome": true, "approval_notify": true}
	ctx := context.Background()
	id := seedPending(t, st, "e1", "e1@example.com", "selfreg", "")

	if err := svc.Approve(ctx, id); err != nil {
		t.Errorf("邮件失败不应阻断通过: %v", err)
	}
	id2 := seedPending(t, st, "e2", "e2@example.com", "selfreg", "")
	if err := svc.Reject(ctx, id2); err != nil {
		t.Errorf("邮件失败不应阻断拒绝: %v", err)
	}
	if len(mm.welcomeBodies) != 1 || len(mm.notifyCalls) != 1 {
		t.Errorf("mock 调用异常: %v %v", mm.welcomeBodies, mm.notifyCalls)
	}
}

// TestScopeEnabled 通知邮件按 scope：approval_notify 未启用时不发送（mock 断言未调用）
func TestScopeEnabled(t *testing.T) {
	st, svc, mm := newTestApproval(t, false)
	ctx := context.Background()
	// 未配置任何 scope：welcome/approval_notify 均未启用
	id := seedPending(t, st, "f1", "f1@example.com", "selfreg", "")
	if err := svc.Approve(ctx, id); err != nil {
		t.Fatalf("通过失败: %v", err)
	}
	if len(mm.welcomeBodies) != 0 {
		t.Errorf("welcome scope 未启用不应发送欢迎邮件: %v", mm.welcomeBodies)
	}
	id2 := seedPending(t, st, "f2", "f2@example.com", "selfreg", "")
	if err := svc.Reject(ctx, id2); err != nil {
		t.Fatalf("拒绝失败: %v", err)
	}
	if len(mm.notifyCalls) != 0 {
		t.Errorf("approval_notify scope 未启用不应发送通知邮件: %v", mm.notifyCalls)
	}
}

// TestListPagination 待审批列表分页 + 字段
func TestListPagination(t *testing.T) {
	st, svc, _ := newTestApproval(t, false)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		seedPending(t, st, "g"+strings.Repeat("x", i), "g"+strings.Repeat("x", i)+"@example.com", "selfreg", "")
	}
	list, total, err := svc.List(ctx, 1, 2)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total != 3 || len(list) != 2 {
		t.Errorf("列表分页异常: total=%d len=%d", total, len(list))
	}
	// 已激活/已禁用不计入待审批
	if _, err := st.DB().Exec(`UPDATE users SET status = 'active' WHERE id = 1`); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	list, total, err = svc.List(ctx, 1, 20)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total != 2 {
		t.Errorf("激活后待审批数应为 2: %d", total)
	}
}
