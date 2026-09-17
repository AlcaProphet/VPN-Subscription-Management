package approval

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"vpn-sub/internal/log"
	"vpn-sub/internal/mail"
	"vpn-sub/internal/store"
	"vpn-sub/internal/xray"
)

// approvalMailCall 记录一次审批邮件派发调用。
type approvalMailCall struct {
	userID int64
	to     string
}

// mockMail 记录审批通过/拒绝派发调用并返回可配置结果；
// scopes 模拟派发器 scope 判定——nil/未含项返回 skipped 且不记录调用。
type mockMail struct {
	approvedCalls []approvalMailCall
	rejectedCalls []approvalMailCall
	failSend      bool
	scopes        map[string]bool
}

func (m *mockMail) DispatchApprovalApproved(_ context.Context, userID int64, to string) mail.DispatchResult {
	if !m.scopes["approval_notify"] {
		return mail.DispatchResult{Status: mail.DispatchSkipped, Reason: mail.ReasonScopeDisabled}
	}
	m.approvedCalls = append(m.approvedCalls, approvalMailCall{userID: userID, to: to})
	if m.failSend {
		return mail.DispatchResult{Status: mail.DispatchRejected, Reason: mail.ReasonQueueFull, LogID: 1}
	}
	return mail.DispatchResult{Status: mail.DispatchQueued, LogID: 1}
}

func (m *mockMail) DispatchApprovalRejected(_ context.Context, userID int64, to string) mail.DispatchResult {
	if !m.scopes["approval_notify"] {
		return mail.DispatchResult{Status: mail.DispatchSkipped, Reason: mail.ReasonScopeDisabled}
	}
	m.rejectedCalls = append(m.rejectedCalls, approvalMailCall{userID: userID, to: to})
	if m.failSend {
		return mail.DispatchResult{Status: mail.DispatchRejected, Reason: mail.ReasonQueueFull, LogID: 1}
	}
	return mail.DispatchResult{Status: mail.DispatchQueued, LogID: 1}
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
	mm := &mockMail{failSend: failSend}
	svc := NewService(st, mm, log.New("error", "console"))
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

func TestRecentPendingDesc(t *testing.T) {
	st, svc, _ := newTestApproval(t, false)
	ctx := context.Background()
	createdAt := []time.Time{
		time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.January, 3, 10, 0, 0, 0, time.UTC),
	}
	for i, username := range []string{"oldest", "middle", "newest"} {
		if _, err := st.DB().ExecContext(ctx,
			`INSERT INTO users (username, email, role, user_source, status, created_at) VALUES (?,?,?,?,?,?)`,
			username, username+"@example.com", "user", "selfreg", "pending", createdAt[i]); err != nil {
			t.Fatalf("插入待审批用户失败: %v", err)
		}
	}

	list, err := svc.RecentPending(ctx, 2)
	if err != nil {
		t.Fatalf("读取最近待审批用户失败: %v", err)
	}
	if len(list) != 2 || list[0].Username != "newest" || list[1].Username != "middle" {
		t.Errorf("最近待审批用户排序异常: %+v", list)
	}
	if empty, err := svc.RecentPending(ctx, 0); err != nil || len(empty) != 0 {
		t.Errorf("limit=0 应返回空列表: list=%+v err=%v", empty, err)
	}
}

// TestApproveActivatesAndClearsClaims 通过：激活 + 清 claims；欢迎邮件按来源区分文案
func TestApproveActivatesAndClearsClaims(t *testing.T) {
	st, svc, mm := newTestApproval(t, false)
	mm.scopes = map[string]bool{"approval_notify": true}
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
	// 审批通过只走 approval_notify 的审批通过通知，不得再叠发 welcome；每个成功账号一封。
	if len(mm.approvedCalls) != 2 {
		t.Fatalf("审批通过应每个账号发送一封审批通过通知: %+v", mm.approvedCalls)
	}
	if mm.approvedCalls[0].to != "alice@example.com" || mm.approvedCalls[1].to != "bob@example.com" {
		t.Errorf("审批通过通知收件人异常: %+v", mm.approvedCalls)
	}
	if mm.approvedCalls[0].userID != oidcID || mm.approvedCalls[1].userID != selfID {
		t.Errorf("审批通过通知用户 ID 异常: %+v", mm.approvedCalls)
	}
	if len(mm.rejectedCalls) != 0 {
		t.Errorf("审批通过不得触发拒绝通知: %+v", mm.rejectedCalls)
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
	if len(mm.rejectedCalls) != 1 || mm.rejectedCalls[0].to != "carol@example.com" || mm.rejectedCalls[0].userID != id {
		t.Errorf("拒绝应触发审批拒绝通知: %+v", mm.rejectedCalls)
	}
	if len(mm.approvedCalls) != 0 {
		t.Errorf("拒绝不得触发审批通过通知: %+v", mm.approvedCalls)
	}
	// 邮箱释放：同邮箱可重新插入（唯一约束不冲突）
	if _, err := st.DB().Exec(`INSERT INTO users (username, email, role, user_source, status)
		VALUES ('carol2', 'carol@example.com', 'user', 'selfreg', 'pending')`); err != nil {
		t.Errorf("邮箱应释放可重新注册: %v", err)
	}
}

// TestRejectCallsXrayCleanupHooks 拒绝删除前收集目标，删除后执行清理回调。
func TestRejectCallsXrayCleanupHooks(t *testing.T) {
	st, svc, _ := newTestApproval(t, false)
	ctx := context.Background()
	id := seedPending(t, st, "dave", "dave@example.com", "selfreg", "")
	want := []xray.Target{{NodeID: 1, InstanceID: 2, Tag: "in-a", APIAddr: "127.0.0.1:10086"}}
	var gotDeletedUser int64
	var gotDeletedTargets []xray.Target
	svc.SetOnUserDeleting(func(_ context.Context, uid int64) ([]xray.Target, error) {
		return want, nil
	})
	svc.SetOnUserDeleted(func(_ context.Context, uid int64, targets []xray.Target) {
		gotDeletedUser = uid
		gotDeletedTargets = append(gotDeletedTargets, targets...)
	})
	if err := svc.Reject(ctx, id); err != nil {
		t.Fatalf("拒绝失败: %v", err)
	}
	if gotDeletedUser != id || len(gotDeletedTargets) != 1 {
		t.Fatalf("拒绝清理回调异常 user=%d targets=%+v", gotDeletedUser, gotDeletedTargets)
	}
}

// TestBatchApproveCounts 批量通过：部分失败回执计数正确（不存在的 id 计失败）
func TestBatchApproveCounts(t *testing.T) {
	st, svc, mm := newTestApproval(t, false)
	mm.scopes = map[string]bool{"approval_notify": true}
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
	if len(mm.approvedCalls) != 2 {
		t.Fatalf("批量通过应每个成功账号复用单次审批发送一封：%+v", mm.approvedCalls)
	}
	for _, call := range mm.approvedCalls {
		if call.userID == 0 || call.to == "" {
			t.Fatalf("批量通过派发参数异常: %+v", call)
		}
	}
}

// TestSMTPFailureDoesNotBlock SMTP 失败不阻断：注入发送失败 → Approve/Reject 仍成功
func TestSMTPFailureDoesNotBlock(t *testing.T) {
	st, svc, mm := newTestApproval(t, true)
	mm.scopes = map[string]bool{"approval_notify": true}
	ctx := context.Background()
	id := seedPending(t, st, "e1", "e1@example.com", "selfreg", "")

	if err := svc.Approve(ctx, id); err != nil {
		t.Errorf("邮件失败不应阻断通过: %v", err)
	}
	id2 := seedPending(t, st, "e2", "e2@example.com", "selfreg", "")
	if err := svc.Reject(ctx, id2); err != nil {
		t.Errorf("邮件失败不应阻断拒绝: %v", err)
	}
	if len(mm.approvedCalls) != 1 || len(mm.rejectedCalls) != 1 {
		t.Errorf("mock 调用异常: approved=%+v rejected=%+v", mm.approvedCalls, mm.rejectedCalls)
	}
}

// TestScopeEnabled 通知邮件按 scope：approval_notify 未启用时不发送（mock 断言未调用）
func TestScopeEnabled(t *testing.T) {
	st, svc, mm := newTestApproval(t, false)
	ctx := context.Background()
	// 未配置任何 scope：审批通过/拒绝通知均不应发送。
	id := seedPending(t, st, "f1", "f1@example.com", "selfreg", "")
	if err := svc.Approve(ctx, id); err != nil {
		t.Fatalf("通过失败: %v", err)
	}
	if len(mm.approvedCalls) != 0 {
		t.Errorf("approval_notify scope 未启用不应发送审批通过通知: %+v", mm.approvedCalls)
	}
	id2 := seedPending(t, st, "f2", "f2@example.com", "selfreg", "")
	if err := svc.Reject(ctx, id2); err != nil {
		t.Fatalf("拒绝失败: %v", err)
	}
	if len(mm.rejectedCalls) != 0 {
		t.Errorf("approval_notify scope 未启用不应发送审批拒绝通知: %+v", mm.rejectedCalls)
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

// TestApproveRejectSkipEmptyEmail 无邮箱账号仍可完成审批主流程，但不得调用任何邮件发送。
func TestApproveRejectSkipEmptyEmail(t *testing.T) {
	st, svc, mm := newTestApproval(t, false)
	mm.scopes = map[string]bool{"approval_notify": true}
	ctx := context.Background()
	insert := func(username, status string) int64 {
		res, err := st.DB().ExecContext(ctx,
			`INSERT INTO users (username, email, role, user_source, status) VALUES (?, NULL, 'user', 'selfreg', ?)`,
			username, status)
		if err != nil {
			t.Fatalf("插入无邮箱用户失败: %v", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	approveID := insert("no-email-approve", "pending")
	rejectID := insert("no-email-reject", "pending")
	if err := svc.Approve(ctx, approveID); err != nil {
		t.Fatalf("无邮箱审批通过失败: %v", err)
	}
	if err := svc.Reject(ctx, rejectID); err != nil {
		t.Fatalf("无邮箱审批拒绝失败: %v", err)
	}
	if len(mm.approvedCalls) != 0 || len(mm.rejectedCalls) != 0 {
		t.Fatalf("无邮箱账号不得发送邮件: approved=%+v rejected=%+v", mm.approvedCalls, mm.rejectedCalls)
	}
}
