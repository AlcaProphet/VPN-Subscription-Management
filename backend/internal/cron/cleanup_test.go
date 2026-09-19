package cron

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func TestResetCleanup(t *testing.T) {
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	db := st.DB()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `INSERT INTO users (username, email, role, user_source, status) VALUES ('cleanup','cleanup@example.com','user','local','active')`); err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	now := time.Now()
	cases := []struct {
		token string
		exp   time.Time
		used  int
		keep  bool
	}{
		{"expired-clean", now.Add(-time.Hour), 0, false},
		{"used-clean", now.Add(time.Hour), 1, false},
		{"valid-keep", now.Add(time.Hour), 0, true},
	}
	for _, tc := range cases {
		if _, err := db.ExecContext(ctx, `INSERT INTO password_reset_tokens (token, user_id, expires_at, used) VALUES (?,1,?,?)`, tc.token, tc.exp, tc.used); err != nil {
			t.Fatalf("插入令牌失败: %v", err)
		}
	}
	cleanupResetTokensOnce(db, slog.New(slog.NewTextHandler(io.Discard, nil)))
	for _, tc := range cases {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM password_reset_tokens WHERE token = ?`, tc.token).Scan(&n); err != nil {
			t.Fatalf("查询令牌失败: %v", err)
		}
		if tc.keep && n != 1 {
			t.Errorf("有效令牌 %s 应保留，实际 %d", tc.token, n)
		}
		if !tc.keep && n != 0 {
			t.Errorf("过期/已用令牌 %s 应清除，实际 %d", tc.token, n)
		}
	}
}

func TestMailResultCleanupKeepsNinetyDayBoundary(t *testing.T) {
	st, err := store.Open(t.TempDir(), "mail-cleanup.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	now := time.Now()
	for _, tc := range []struct {
		masked string
		at     time.Time
	}{
		{masked: "o***@example.com", at: now.AddDate(0, 0, -91)},
		{masked: "k***@example.com", at: now.AddDate(0, 0, -89)},
	} {
		if _, err := st.DB().Exec(`INSERT INTO mail_result_logs
			(kind, source, recipient_masked, result, recorded_at) VALUES ('smtp_test','smtp_test',?,'accepted',?)`, tc.masked, tc.at); err != nil {
			t.Fatalf("插入邮件结果失败: %v", err)
		}
	}
	cleanupMailResultsOnce(st.DB(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	var count int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM mail_result_logs WHERE recipient_masked = 'o***@example.com'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("90 天前结果应删除: count=%d err=%v", count, err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM mail_result_logs WHERE recipient_masked = 'k***@example.com'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("90 天内结果应保留: count=%d err=%v", count, err)
	}
}
