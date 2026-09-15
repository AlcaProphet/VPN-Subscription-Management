package store

import (
	"context"
	"testing"

	"vpn-sub/migrations"
)

// TestMigration1019OidcStateSignature 1019 增加 state 固定列且保留旧行（旧行两列为空）。
func TestMigration1019OidcStateSignature(t *testing.T) {
	t.Run("1018 to 1019", func(t *testing.T) {
		st, err := Open(t.TempDir(), "test.db")
		if err != nil {
			t.Fatalf("打开失败: %v", err)
		}
		defer st.Close()
		ctx := context.Background()
		if err := st.Migrate(ctx, migrationsThrough(1018)); err != nil {
			t.Fatalf("迁移到 1018 失败: %v", err)
		}
		if _, err := st.DB().ExecContext(ctx,
			`INSERT INTO oidc_states (state, code_verifier, nonce, intent) VALUES ('legacy-migration','v','n','login')`); err != nil {
			t.Fatalf("写入旧 state 失败: %v", err)
		}
		if err := st.Migrate(ctx, migrationsThrough(1019)); err != nil {
			t.Fatalf("迁移到 1019 失败: %v", err)
		}
		assertOidcStateSignatureColumns(t, st)
		var providerType, configHash string
		if err := st.DB().QueryRow(
			`SELECT provider_type, config_hash FROM oidc_states WHERE state='legacy-migration'`).Scan(&providerType, &configHash); err != nil {
			t.Fatalf("旧 state 行应保留: %v", err)
		}
		if providerType != "" || configHash != "" {
			t.Fatalf("旧 state 固定列应为空: provider=%q hash=%q", providerType, configHash)
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
		assertOidcStateSignatureColumns(t, st)
	})
}

func assertOidcStateSignatureColumns(t *testing.T, st *Store) {
	t.Helper()
	for _, column := range []string{"provider_type", "config_hash"} {
		var n int
		if err := st.DB().QueryRow(
			`SELECT COUNT(*) FROM pragma_table_info('oidc_states') WHERE name=?`, column).Scan(&n); err != nil {
			t.Fatalf("查询 %s 列失败: %v", column, err)
		}
		if n != 1 {
			t.Fatalf("oidc_states 应存在 %s 列: %d", column, n)
		}
	}
}
