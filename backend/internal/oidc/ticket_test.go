package oidc

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vpn-sub/internal/store"
)

func countLoginTickets(t *testing.T, st *store.Store) int {
	t.Helper()
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM oidc_login_tickets`).Scan(&n); err != nil {
		t.Fatalf("统计 ticket 失败: %v", err)
	}
	return n
}

func failTicketDeleteTrigger(t *testing.T, st *store.Store) {
	t.Helper()
	if _, err := st.DB().Exec(`
		CREATE TRIGGER fail_ticket_delete BEFORE DELETE ON oidc_login_tickets
		BEGIN
			SELECT RAISE(ABORT, 'fail ticket delete');
		END;`); err != nil {
		t.Fatalf("创建删除失败触发器失败: %v", err)
	}
}

// TestLoginTicketIssueConsumeOneTime 正常创建、一次性消费；重复消费必须失败。
func TestLoginTicketIssueConsumeOneTime(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	ctx := context.Background()
	ticket, err := svc.IssueLoginTicket(ctx, "session-token-1")
	if err != nil {
		t.Fatalf("IssueLoginTicket 失败: %v", err)
	}
	session, err := svc.ConsumeLoginTicket(ctx, ticket)
	if err != nil || session != "session-token-1" {
		t.Fatalf("ConsumeLoginTicket 失败: %q %v", session, err)
	}
	if got := countLoginTickets(t, st); got != 0 {
		t.Fatalf("消费后 ticket 应删除，实际 %d", got)
	}
	if _, err := svc.ConsumeLoginTicket(ctx, ticket); !errors.Is(err, ErrLoginTicketInvalid) {
		t.Fatalf("二次消费应 ErrLoginTicketInvalid: %v", err)
	}
	if _, err := svc.ConsumeLoginTicket(ctx, "unknown-ticket"); !errors.Is(err, ErrLoginTicketInvalid) {
		t.Fatalf("未知 ticket 应 ErrLoginTicketInvalid: %v", err)
	}
}

// TestLoginTicketExpiredCleanup 过期 ticket 返回无效且记录被清理。
func TestLoginTicketExpiredCleanup(t *testing.T) {
	st, svc, _ := newTestOidcService(t)
	ctx := context.Background()
	if _, err := st.DB().Exec(`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('expired','s',?)`, time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("插入过期 ticket 失败: %v", err)
	}
	if _, err := svc.ConsumeLoginTicket(ctx, "expired"); !errors.Is(err, ErrLoginTicketInvalid) {
		t.Fatalf("过期 ticket 应 ErrLoginTicketInvalid: %v", err)
	}
	if got := countLoginTickets(t, st); got != 0 {
		t.Fatalf("过期 ticket 应被清理，实际 %d", got)
	}
}

// TestLoginTicketDeleteErrors 删除失败不能被静默当作 ticket 无效：有效消费与过期清理都必须返回真实错误。
func TestLoginTicketDeleteErrors(t *testing.T) {
	ctx := context.Background()
	t.Run("valid-consume-delete", func(t *testing.T) {
		st, svc, _ := newTestOidcService(t)
		if _, err := st.DB().Exec(`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('valid','s',?)`, time.Now().Add(time.Minute)); err != nil {
			t.Fatalf("插入 ticket 失败: %v", err)
		}
		failTicketDeleteTrigger(t, st)
		_, err := svc.ConsumeLoginTicket(ctx, "valid")
		if err == nil || errors.Is(err, ErrLoginTicketInvalid) {
			t.Fatalf("有效 ticket 删除失败应返回真实错误，实际 %v", err)
		}
		if got := countLoginTickets(t, st); got != 1 {
			t.Fatalf("删除失败应回滚保留 ticket，实际 %d", got)
		}
	})
	t.Run("expired-cleanup-delete", func(t *testing.T) {
		st, svc, _ := newTestOidcService(t)
		if _, err := st.DB().Exec(`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('expired','s',?)`, time.Now().Add(-time.Minute)); err != nil {
			t.Fatalf("插入过期 ticket 失败: %v", err)
		}
		failTicketDeleteTrigger(t, st)
		_, err := svc.ConsumeLoginTicket(ctx, "expired")
		if err == nil || errors.Is(err, ErrLoginTicketInvalid) {
			t.Fatalf("过期清理删除失败应返回真实错误，实际 %v", err)
		}
	})
	t.Run("issue-expired-cleanup-delete", func(t *testing.T) {
		st, svc, _ := newTestOidcService(t)
		if _, err := st.DB().Exec(`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('expired','s',?)`, time.Now().Add(-time.Minute)); err != nil {
			t.Fatalf("插入过期 ticket 失败: %v", err)
		}
		failTicketDeleteTrigger(t, st)
		_, err := svc.IssueLoginTicket(ctx, "new-session")
		if err == nil || !strings.Contains(err.Error(), "清理过期 OIDC ticket 失败") {
			t.Fatalf("Issue 清理过期记录失败应返回真实错误，实际 %v", err)
		}
	})
}
