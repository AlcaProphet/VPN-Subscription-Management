package store

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"vpn-sub/migrations"
)

const (
	oldBlueprintFilePath  = "sentinel/build22-old-blueprint.txt"
	oldBlueprintSelection = `{"pools":[{"pool_id":100,"target":"PROXY"}]}`
	oldBlueprintPlan      = `{"sentinel":"build22-1016-old-render-plan"}`
)

// brokenMigrationsThrough1016 复制真实 0001～1016，仅在真实 1016 SQL 末尾追加必然失败语句。
// 失败语句位于全部删表、建表、sqlite_sequence 更新与临时表删除之后，用于验证单迁移整体回滚。
func brokenMigrationsThrough1016() fstest.MapFS {
	out := migrationsThrough(1016)
	name := ""
	for candidate := range out {
		v, err := parseVersion(candidate)
		if err == nil && v == 1016 {
			name = candidate
			break
		}
	}
	if name == "" {
		panic("真实 1016 迁移不存在")
	}
	realSQL, err := fs.ReadFile(migrations.FS, name)
	if err != nil {
		panic(err)
	}
	failingSQL := []byte("\n-- 测试专用：位于真实 1016 全部删表/建表/sequence/临时表操作之后\nINSERT INTO __migration_1016_rollback_probe__ (id) VALUES (1);\n")
	out[name] = &fstest.MapFile{Data: append(append([]byte{}, realSQL...), failingSQL...)}
	return out
}

func openMigrationTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(dir, "test.db")
	if err != nil {
		t.Fatalf("打开迁移测试库失败: %v", err)
	}
	return st, dir
}

func migrateTo1015(t *testing.T, st *Store) {
	t.Helper()
	if err := st.Migrate(context.Background(), migrationsThrough(1015)); err != nil {
		t.Fatalf("真实 0001～1015 迁移失败: %v", err)
	}
}

func scalarInt(t *testing.T, st *Store, query string, args ...any) int {
	t.Helper()
	var value int
	if err := st.DB().QueryRowContext(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("查询失败 %q: %v", query, err)
	}
	return value
}

func queryString(t *testing.T, st *Store, query string, args ...any) string {
	t.Helper()
	var value string
	if err := st.DB().QueryRowContext(context.Background(), query, args...).Scan(&value); err != nil {
		t.Fatalf("查询失败 %q: %v", query, err)
	}
	return value
}

func tableExists(t *testing.T, st *Store, name string) bool {
	t.Helper()
	return scalarInt(t, st, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name) == 1
}

func assertMigrationThrough1015(t *testing.T, st *Store) {
	t.Helper()
	if got := scalarInt(t, st, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`); got != 1015 {
		t.Fatalf("MAX(version) = %d, 期望 1015", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM schema_migrations WHERE version > 1015`); got != 0 {
		t.Fatalf("存在高于 1015 的迁移记录: %d", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM schema_migrations`); got != 20 {
		t.Fatalf("1015 迁移记录数 = %d, 期望 20", got)
	}
	for _, name := range []string{"rule_pools", "pool_entries", "pool_sync_tasks", "versions", "assembly_blueprints"} {
		if !tableExists(t, st, name) {
			t.Fatalf("1015 旧 schema 缺少表 %s", name)
		}
	}
}

func insertOldPoolFixtures(t *testing.T, st *Store) int64 {
	t.Helper()
	ctx := context.Background()
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO rule_pools (id, name, urls_json) VALUES (10, 'Build22 旧池10', '[]')`); err != nil {
		t.Fatalf("插入旧池 10 失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO rule_pools (id, name, urls_json) VALUES (100, 'Build22 旧池100', '["https://old.example/rules.txt?token=secret"]')`); err != nil {
		t.Fatalf("插入旧池 100 失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order) VALUES (100, 'DOMAIN-SUFFIX', 'manual.example.com', 'manual', 1)`); err != nil {
		t.Fatalf("插入 manual 旧条目失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order) VALUES (100, 'DOMAIN-SUFFIX', 'url.example.com', 'url', 2)`); err != nil {
		t.Fatalf("插入 URL 旧条目失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_sync_tasks (pool_id, status, per_url_json, error) VALUES (100, 'succeeded', '[{"url":"https://old.example/rules.txt?token=secret"}]', '')`); err != nil {
		t.Fatalf("插入旧同步任务失败: %v", err)
	}
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription', 10, 7, ?)`, oldBlueprintFilePath)
	if err != nil {
		t.Fatalf("插入 sentinel 版本失败: %v", err)
	}
	versionID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("读取 sentinel 版本 ID 失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO assembly_blueprints (version_id, target_syntax, fixed_params_json, selection_json, custom_rules_json, render_plan_json)
		 VALUES (?, 'clash-yaml', '{"port":7890}', ?, '[]', ?)`,
		versionID, oldBlueprintSelection, oldBlueprintPlan); err != nil {
		t.Fatalf("插入 sentinel 蓝图失败: %v", err)
	}
	return versionID
}

func assertOldPoolFixtures(t *testing.T, st *Store, versionID int64) {
	t.Helper()
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM rule_pools`); got != 2 {
		t.Fatalf("旧池数量 = %d, 期望 2", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM pool_entries`); got != 2 {
		t.Fatalf("旧条目数量 = %d, 期望 2", got)
	}
	if got := queryString(t, st, `SELECT match_value FROM pool_entries WHERE source = 'manual'`); got != "manual.example.com" {
		t.Fatalf("manual 旧条目内容变化: %q", got)
	}
	if got := queryString(t, st, `SELECT match_value FROM pool_entries WHERE source = 'url'`); got != "url.example.com" {
		t.Fatalf("URL 旧条目内容变化: %q", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM pool_sync_tasks`); got != 1 {
		t.Fatalf("旧任务数量 = %d, 期望 1", got)
	}
	if got := queryString(t, st, `SELECT file_path FROM versions WHERE id = ?`, versionID); got != oldBlueprintFilePath {
		t.Fatalf("versions sentinel = %q", got)
	}
	if got := queryString(t, st, `SELECT selection_json FROM assembly_blueprints WHERE version_id = ?`, versionID); got != oldBlueprintSelection {
		t.Fatalf("selection_json sentinel = %q", got)
	}
	if got := queryString(t, st, `SELECT render_plan_json FROM assembly_blueprints WHERE version_id = ?`, versionID); got != oldBlueprintPlan {
		t.Fatalf("render_plan_json sentinel = %q", got)
	}
}

func assert1016SchemaAndHistory(t *testing.T, st *Store, versionID int64) {
	t.Helper()
	if tableExists(t, st, "pool_entries") {
		t.Fatal("1016 后 pool_entries 表应不存在")
	}
	if !tableExists(t, st, "pool_sync_tasks") {
		t.Fatal("1016 后 pool_sync_tasks 同名新表应存在")
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM pool_sync_tasks`); got != 0 {
		t.Fatalf("1016 后旧同步任务应清零, 实际 %d", got)
	}
	for _, name := range []string{"rule_pool_sources", "pool_source_snapshots", "pool_canonical_rules", "pool_rule_origins"} {
		if !tableExists(t, st, name) {
			t.Fatalf("1016 后缺少新表 %s", name)
		}
	}
	if got := scalarInt(t, st, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`); got != 1016 {
		t.Fatalf("1016 后 MAX(version) = %d, 期望 1016", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM schema_migrations WHERE version > 1016`); got != 0 {
		t.Fatalf("存在高于 1016 的迁移记录: %d", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM schema_migrations`); got != 21 {
		t.Fatalf("1016 后迁移记录数 = %d, 期望 21", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM schema_migrations WHERE version = 1016`); got != 1 {
		t.Fatalf("1016 记录数 = %d, 期望 1", got)
	}
	if got := queryString(t, st, `SELECT file_path FROM versions WHERE id = ?`, versionID); got != oldBlueprintFilePath {
		t.Fatalf("1016 后 versions sentinel = %q", got)
	}
	if got := queryString(t, st, `SELECT selection_json FROM assembly_blueprints WHERE version_id = ?`, versionID); got != oldBlueprintSelection {
		t.Fatalf("1016 后 selection_json sentinel = %q", got)
	}
	if got := queryString(t, st, `SELECT render_plan_json FROM assembly_blueprints WHERE version_id = ?`, versionID); got != oldBlueprintPlan {
		t.Fatalf("1016 后 render_plan_json sentinel = %q", got)
	}
}

func insertNewPoolAndAssertSequence(t *testing.T, st *Store) int64 {
	t.Helper()
	res, err := st.DB().ExecContext(context.Background(), `INSERT INTO rule_pools (name) VALUES ('Build22 新池')`)
	if err != nil {
		t.Fatalf("插入 1016 后新池失败: %v", err)
	}
	newPoolID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("读取 1016 后新池 ID 失败: %v", err)
	}
	if newPoolID != 101 {
		t.Fatalf("新旧池 ID 防复用失败: LastInsertId=%d, 期望 101", newPoolID)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM rule_pools WHERE id <= 100`); got != 0 {
		t.Fatalf("出现 ID <= 100 的新池: %d", got)
	}
	if got := scalarInt(t, st, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 101 {
		t.Fatalf("rule_pools sqlite_sequence = %d, 期望 101", got)
	}
	return newPoolID
}

func TestMigration1015To1016StoreLevel(t *testing.T) {
	st, dir := openMigrationTestStore(t)
	closed := false
	t.Cleanup(func() {
		if !closed {
			_ = st.Close()
		}
	})
	ctx := context.Background()
	migrateTo1015(t, st)
	assertMigrationThrough1015(t, st)
	versionID := insertOldPoolFixtures(t, st)
	assertOldPoolFixtures(t, st, versionID)
	if got := scalarInt(t, st, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 100 {
		t.Fatalf("1015 迁移后 rule_pools sqlite_sequence = %d, 期望 100", got)
	}

	if err := st.Migrate(ctx, migrationsThrough(1016)); err != nil {
		t.Fatalf("真实 0001～1016 迁移失败: %v", err)
	}
	assert1016SchemaAndHistory(t, st, versionID)
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM rule_pools`); got != 0 {
		t.Fatalf("1016 后旧 rule_pools 行应清除, 实际 %d", got)
	}
	if got := scalarInt(t, st, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 100 {
		t.Fatalf("1016 后插入新池前 rule_pools sqlite_sequence = %d, 期望 100", got)
	}

	// 幂等：同一 Store 再次执行 1016。
	if err := st.Migrate(ctx, migrationsThrough(1016)); err != nil {
		t.Fatalf("同一 Store 重复迁移 1016 应幂等: %v", err)
	}
	assert1016SchemaAndHistory(t, st, versionID)
	if got := scalarInt(t, st, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 100 {
		t.Fatalf("重复迁移后 rule_pools sqlite_sequence = %d, 期望 100", got)
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM rule_pools`); got != 0 {
		t.Fatalf("重复迁移后 rule_pools 仍应为空, 实际 %d", got)
	}

	// 旧 ID 不复用：新池从 101 开始，sequence 同步。
	newPoolID := insertNewPoolAndAssertSequence(t, st)

	// close/reopen 后再次迁移仍幂等，历史数据、sentinel 与 sequence 不变。
	if err := st.Close(); err != nil {
		t.Fatalf("关闭 Store 失败: %v", err)
	}
	closed = true
	st2, err := Open(dir, "test.db")
	if err != nil {
		t.Fatalf("重新打开 Store 失败: %v", err)
	}
	defer st2.Close()
	if err := st2.Migrate(ctx, migrationsThrough(1016)); err != nil {
		t.Fatalf("close/reopen 后重复迁移 1016 应幂等: %v", err)
	}
	assert1016SchemaAndHistory(t, st2, versionID)
	if got := scalarInt(t, st2, `SELECT COUNT(*) FROM rule_pools WHERE id = ?`, newPoolID); got != 1 {
		t.Fatalf("reopen 后新池 %d 丢失, 实际 %d", newPoolID, got)
	}
	if got := scalarInt(t, st2, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 101 {
		t.Fatalf("reopen 后 rule_pools sqlite_sequence = %d, 期望 101", got)
	}
	if got := scalarInt(t, st2, `SELECT COUNT(*) FROM schema_migrations`); got != 21 {
		t.Fatalf("reopen 后迁移记录数 = %d, 期望 21", got)
	}
}

func TestMigration1016FailureRollsBack(t *testing.T) {
	st, _ := openMigrationTestStore(t)
	defer st.Close()
	ctx := context.Background()
	migrateTo1015(t, st)
	versionID := insertOldPoolFixtures(t, st)
	assertOldPoolFixtures(t, st, versionID)
	if got := scalarInt(t, st, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 100 {
		t.Fatalf("失败注入前 rule_pools sqlite_sequence = %d, 期望 100", got)
	}

	if err := st.Migrate(ctx, brokenMigrationsThrough1016()); err == nil {
		t.Fatal("真实 1016 SQL 末尾注入失败语句后迁移应返回错误")
	}

	// 旧 schema、旧数据、sequence、sentinel 与 schema_migrations 全部回滚。
	assertMigrationThrough1015(t, st)
	if !tableExists(t, st, "pool_entries") {
		t.Fatal("失败回滚后 pool_entries 旧表应仍存在")
	}
	assertOldPoolFixtures(t, st, versionID)
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM pragma_table_info('rule_pools') WHERE name = 'urls_json'`); got != 1 {
		t.Fatalf("失败回滚后 rule_pools 旧 schema 应保留 urls_json 列, 实际 %d", got)
	}
	for _, name := range []string{"rule_pool_sources", "pool_source_snapshots", "pool_canonical_rules", "pool_rule_origins"} {
		if tableExists(t, st, name) {
			t.Fatalf("失败回滚后不应存在新表 %s", name)
		}
	}
	if tableExists(t, st, "_pool_1016_old_max") {
		t.Fatal("失败回滚后 1016 临时表应被回滚")
	}
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM schema_migrations WHERE version = 1016`); got != 0 {
		t.Fatalf("失败回滚后不应写入 1016 迁移记录: %d", got)
	}
	if got := scalarInt(t, st, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 100 {
		t.Fatalf("失败回滚后 rule_pools sqlite_sequence = %d, 期望 100", got)
	}

	// 使用真实 1016 重试必须成功，证明失败后的数据库可继续升级。
	if err := st.Migrate(ctx, migrationsThrough(1016)); err != nil {
		t.Fatalf("失败回滚后真实 1016 重试应成功: %v", err)
	}
	assert1016SchemaAndHistory(t, st, versionID)
	if got := scalarInt(t, st, `SELECT COUNT(*) FROM rule_pools`); got != 0 {
		t.Fatalf("重试 1016 后旧 rule_pools 行应清除, 实际 %d", got)
	}
	if got := scalarInt(t, st, `SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name = 'rule_pools'), 0)`); got != 100 {
		t.Fatalf("重试 1016 后插入新池前 sqlite_sequence = %d, 期望 100", got)
	}
}
