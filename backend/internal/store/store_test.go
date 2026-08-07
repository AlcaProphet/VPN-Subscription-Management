package store

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"sync"
	"testing"
	"testing/fstest"
)

// testFS 构造最小迁移集
func testFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
}

// TestMigrateSuccess 临时库迁移成功且 system_config 存在
func TestMigrateSuccess(t *testing.T) {
	st, err := Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background(), testFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	// 验证表存在
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='system_config'`).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 1 {
		t.Error("system_config 表未创建")
	}
	// 验证迁移记录
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n); err != nil {
		t.Fatalf("查询迁移记录失败: %v", err)
	}
	if n != 1 {
		t.Errorf("迁移记录数异常: %d", n)
	}
	// 幂等：重复迁移不报错
	if err := st.Migrate(context.Background(), testFS()); err != nil {
		t.Errorf("重复迁移应成功: %v", err)
	}
}

// TestMigrateRejectHigherVersion 注入伪造更高版本记录 → 拒绝启动（回滚边界）
func TestMigrateRejectHigherVersion(t *testing.T) {
	st, err := Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background(), testFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	// 注入伪造更高版本（9999）
	if _, err := st.DB().Exec(`INSERT INTO schema_migrations (version) VALUES (9999)`); err != nil {
		t.Fatalf("注入伪造版本失败: %v", err)
	}
	if err := st.Migrate(context.Background(), testFS()); err == nil {
		t.Error("数据库版本高于代码支持版本应拒绝启动")
	}
}

// TestMigrateInvalidFile 非法迁移文件名应报错
func TestMigrateInvalidFile(t *testing.T) {
	st, err := Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer st.Close()
	fsys := fstest.MapFS{
		"abc_bad.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE t (id INTEGER)`)},
	}
	if err := st.Migrate(context.Background(), fsys); err == nil {
		t.Error("非法迁移文件名应报错")
	}
}

// TestTxImmediateConcurrent 并发两个 TxImmediate 串行完成不报 busy
func TestTxImmediateConcurrent(t *testing.T) {
	st, err := Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background(), testFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	ctx := context.Background()
	// 预置初始值
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, err := tx.Exec(`INSERT INTO system_config (key, value) VALUES ('counter', '0')`)
		return err
	}); err != nil {
		t.Fatalf("预置失败: %v", err)
	}
	// 并发 N 个事务：每个读-加-写
	const N = 8
	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- st.TxImmediate(ctx, func(tx *sql.Tx) error {
				var v int
				if err := tx.QueryRow(`SELECT CAST(value AS INTEGER) FROM system_config WHERE key = 'counter'`).Scan(&v); err != nil {
					return err
				}
				_, err := tx.Exec(`UPDATE system_config SET value = ? WHERE key = 'counter'`, strconv.Itoa(v+1))
				return err
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("并发事务失败: %v", err)
		}
	}
	// 最终值应为 N（串行化无丢失更新）
	var final int
	if err := st.DB().QueryRow(`SELECT CAST(value AS INTEGER) FROM system_config WHERE key = 'counter'`).Scan(&final); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if final != N {
		t.Errorf("并发事务丢失更新: final=%d want=%d", final, N)
	}
}

// TestTxImmediateRollback 事务内返回错误自动回滚
func TestTxImmediateRollback(t *testing.T) {
	st, err := Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(context.Background(), testFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	ctx := context.Background()
	err = st.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO system_config (key, value) VALUES ('k', 'v')`); err != nil {
			return err
		}
		return errors.New("故意失败")
	})
	if err == nil {
		t.Fatal("事务应返回错误")
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM system_config WHERE key = 'k'`).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 0 {
		t.Error("事务应回滚，无残留数据")
	}
}

// TestOpenPathTraversal 数据库文件名含路径应拒绝
func TestOpenPathTraversal(t *testing.T) {
	if _, err := Open(t.TempDir(), "../evil.db"); err == nil {
		t.Error("含路径分隔符的数据库文件名应拒绝")
	}
}
