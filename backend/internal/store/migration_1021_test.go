package store

import (
	"context"
	"testing"

	"vpn-sub/migrations"
)

// TestMigration1021OidcTicketFlowHash 1021 为 OIDC ticket 增加流程指纹列；
// 迁移前已存在的旧 ticket 保留，但 flow_hash 为空，消费侧按历史无效票据处理。
func TestMigration1021OidcTicketFlowHash(t *testing.T) {
	t.Run("1020 to 1021", func(t *testing.T) {
		st, err := Open(t.TempDir(), "test.db")
		if err != nil {
			t.Fatalf("打开失败: %v", err)
		}
		defer st.Close()
		ctx := context.Background()
		if err := st.Migrate(ctx, migrationsThrough(1020)); err != nil {
			t.Fatalf("迁移到 1020 失败: %v", err)
		}
		if _, err := st.DB().ExecContext(ctx,
			`INSERT INTO oidc_login_tickets (ticket, session_token, expires_at) VALUES ('legacy-ticket','session',datetime('now','+1 minute'))`); err != nil {
			t.Fatalf("写入旧 ticket 失败: %v", err)
		}
		if err := st.Migrate(ctx, migrationsThrough(1021)); err != nil {
			t.Fatalf("迁移到 1021 失败: %v", err)
		}
		assertOidcTicketFlowHashColumn(t, st)
		var flowHash string
		if err := st.DB().QueryRow(
			`SELECT flow_hash FROM oidc_login_tickets WHERE ticket='legacy-ticket'`).Scan(&flowHash); err != nil {
			t.Fatalf("旧 ticket 行应保留: %v", err)
		}
		if flowHash != "" {
			t.Fatalf("旧 ticket 的 flow_hash 应为空: %q", flowHash)
		}
	})
	t.Run("fresh full", func(t *testing.T) {
		st, err := Open(t.TempDir(), "test.db")
		if err != nil {
			t.Fatalf("打开失败: %v", err)
		}
		defer st.Close()
		if err := st.Migrate(context.Background(), migrations.FS); err != nil {
			t.Fatalf("全新全量迁移失败: %v", err)
		}
		assertOidcTicketFlowHashColumn(t, st)
	})
}

func assertOidcTicketFlowHashColumn(t *testing.T, st *Store) {
	t.Helper()
	var n int
	if err := st.DB().QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('oidc_login_tickets') WHERE name='flow_hash'`).Scan(&n); err != nil {
		t.Fatalf("查询 flow_hash 列失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("oidc_login_tickets 应存在 flow_hash 列: %d", n)
	}
}
