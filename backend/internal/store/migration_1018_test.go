package store

import (
	"context"
	"testing"

	"vpn-sub/migrations"
)

func TestMigration1018ActivatedAt(t *testing.T) {
	t.Run("1017 to 1018", func(t *testing.T) {
		st, err := Open(t.TempDir(), "test.db")
		if err != nil {
			t.Fatalf("打开失败: %v", err)
		}
		defer st.Close()
		ctx := context.Background()
		if err := st.Migrate(ctx, migrationsThrough(1017)); err != nil {
			t.Fatalf("迁移到 1017 失败: %v", err)
		}
		var before int
		if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('pool_source_snapshots') WHERE name='activated_at'`).Scan(&before); err != nil {
			t.Fatalf("查询 1017 列失败: %v", err)
		}
		if before != 0 {
			t.Fatalf("1017 不应有 activated_at 列: %d", before)
		}
		if err := st.Migrate(ctx, migrationsThrough(1018)); err != nil {
			t.Fatalf("迁移到 1018 失败: %v", err)
		}
		assertActivatedAtColumn(t, st)
		if err := st.Migrate(ctx, migrationsThrough(1018)); err != nil {
			t.Fatalf("重复迁移 1018 应幂等: %v", err)
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
		assertActivatedAtColumn(t, st)
	})
}

func assertActivatedAtColumn(t *testing.T, st *Store) {
	t.Helper()
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pragma_table_info('pool_source_snapshots') WHERE name='activated_at'`).Scan(&n); err != nil {
		t.Fatalf("查询 activated_at 列失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("pool_source_snapshots 应存在 activated_at 列: %d", n)
	}
}
