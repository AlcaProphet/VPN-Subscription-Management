package store

import (
	"context"
	"testing"

	"vpn-sub/migrations"
)

// TestMigration1020OidcStateRedirectURI 1020 增加 state 固定的 redirect_uri 列并保留旧行（旧行为空）。
func TestMigration1020OidcStateRedirectURI(t *testing.T) {
	t.Run("1019 to 1020", func(t *testing.T) {
		st, err := Open(t.TempDir(), "test.db")
		if err != nil {
			t.Fatalf("打开失败: %v", err)
		}
		defer st.Close()
		ctx := context.Background()
		if err := st.Migrate(ctx, migrationsThrough(1019)); err != nil {
			t.Fatalf("迁移到 1019 失败: %v", err)
		}
		if _, err := st.DB().ExecContext(ctx,
			`INSERT INTO oidc_states (state, code_verifier, nonce, intent, provider_type, config_hash) VALUES ('legacy-1020','v','n','login','generic','hash')`); err != nil {
			t.Fatalf("写入旧 state 失败: %v", err)
		}
		if err := st.Migrate(ctx, migrationsThrough(1020)); err != nil {
			t.Fatalf("迁移到 1020 失败: %v", err)
		}
		assertOidcStateRedirectURIColumn(t, st)
		var redirectURI string
		if err := st.DB().QueryRow(
			`SELECT redirect_uri FROM oidc_states WHERE state='legacy-1020'`).Scan(&redirectURI); err != nil {
			t.Fatalf("旧 state 行应保留: %v", err)
		}
		if redirectURI != "" {
			t.Fatalf("旧 state 的 redirect_uri 应为空: %q", redirectURI)
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
		assertOidcStateRedirectURIColumn(t, st)
	})
}

func assertOidcStateRedirectURIColumn(t *testing.T, st *Store) {
	t.Helper()
	var n int
	if err := st.DB().QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('oidc_states') WHERE name='redirect_uri'`).Scan(&n); err != nil {
		t.Fatalf("查询 redirect_uri 列失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("oidc_states 应存在 redirect_uri 列: %d", n)
	}
}
