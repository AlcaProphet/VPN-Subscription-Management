package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/migrations"
)

type schemaManifest struct {
	Tables  []tableManifest  `json:"tables"`
	Indexes []indexManifest  `json:"indexes"`
	Seeds   []proxyGroupSeed `json:"proxy_group_seeds"`
	Rows    []tableRowCount  `json:"non_seed_rows"`
}

type tableManifest struct {
	Name        string           `json:"name"`
	Columns     []columnManifest `json:"columns"`
	ForeignKeys []foreignKey     `json:"foreign_keys"`
}

type columnManifest struct {
	CID        int     `json:"cid"`
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	NotNull    int     `json:"not_null"`
	Default    *string `json:"default,omitempty"`
	PrimaryKey int     `json:"primary_key"`
	Hidden     int     `json:"hidden"`
}

type foreignKey struct {
	ID       int    `json:"id"`
	Sequence int    `json:"sequence"`
	Table    string `json:"table"`
	From     string `json:"from"`
	To       string `json:"to"`
	OnUpdate string `json:"on_update"`
	OnDelete string `json:"on_delete"`
	Match    string `json:"match"`
}

type indexManifest struct {
	Table   string        `json:"table"`
	Name    string        `json:"name"`
	Unique  int           `json:"unique"`
	Origin  string        `json:"origin"`
	Partial int           `json:"partial"`
	Columns []indexColumn `json:"columns"`
}

type indexColumn struct {
	Sequence int     `json:"sequence"`
	CID      int     `json:"cid"`
	Name     *string `json:"name,omitempty"`
	Desc     int     `json:"desc"`
	Coll     string  `json:"collation"`
	Key      int     `json:"key"`
}

type proxyGroupSeed struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	PresetKey      string `json:"preset_key"`
	Enabled        int    `json:"enabled"`
	DefinitionJSON string `json:"definition_json"`
}

type tableRowCount struct {
	Table string `json:"table"`
	Count int    `json:"count"`
}

const schemaV1ManifestSHA256 = "9c35b92c8243f9ada77a8396a16d6430cf8e794ff510831580dfe48654196459"

// TestBaselineSchemaContract 固定首版最终结构、索引、外键、种子和空表合同。
func TestBaselineSchemaContract(t *testing.T) {
	ctx := context.Background()
	st := openMigratedStore(t, "baseline.db", migrations.FS)
	manifest := collectSchemaManifest(t, ctx, st.DB())
	if got := len(manifest.Tables); got != 34 {
		t.Fatalf("表数量 = %d，期望 34", got)
	}
	explicitIndexes := 0
	for _, index := range manifest.Indexes {
		if index.Origin == "c" {
			explicitIndexes++
		}
	}
	if explicitIndexes != 25 {
		t.Fatalf("显式索引数量 = %d，期望 25", explicitIndexes)
	}
	if got := len(manifest.Seeds); got != 9 {
		t.Fatalf("代理组种子数量 = %d，期望 9", got)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("序列化 schema manifest 失败: %v", err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(manifestJSON)); got != schemaV1ManifestSHA256 {
		t.Fatalf("schema manifest 漂移: got %s want %s\n%s", got, schemaV1ManifestSHA256, formatManifest(manifest))
	}
	var versions, maxVersion int
	if err := st.DB().QueryRow(`SELECT COUNT(*), COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&versions, &maxVersion); err != nil {
		t.Fatalf("查询迁移版本失败: %v", err)
	}
	if versions != 1 || maxVersion != 1 {
		t.Fatalf("迁移版本记录 = count %d, max %d；期望 count 1, max 1", versions, maxVersion)
	}
	assertDatabaseIntegrity(t, st.DB())
	assertConstraintProbes(t, st.DB())
}

// TestBaselineMigrationLifecycle 覆盖同实例、重开后的幂等以及首版种子不重复。
func TestBaselineMigrationLifecycle(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := Open(dir, "lifecycle.db")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("同实例重复迁移失败: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("关闭数据库失败: %v", err)
	}

	st, err = Open(dir, "lifecycle.db")
	if err != nil {
		t.Fatalf("重新打开数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("重开后重复迁移失败: %v", err)
	}
	var versions, seeds int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("查询版本记录失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM proxy_groups`).Scan(&seeds); err != nil {
		t.Fatalf("查询种子失败: %v", err)
	}
	if versions != 1 || seeds != 9 {
		t.Fatalf("重复迁移后 version=%d seeds=%d，期望 1/9", versions, seeds)
	}
}

// TestBaselineMigrationRollback 验证首版 SQL 任一步失败时，业务结构、种子和版本记录整体回滚。
func TestBaselineMigrationRollback(t *testing.T) {
	baseline, err := fs.ReadFile(migrations.FS, "0001_initial_schema.sql")
	if err != nil {
		t.Fatalf("读取首版基线失败: %v", err)
	}
	broken := append(append([]byte(nil), baseline...), []byte("\nINSERT INTO __missing_baseline_probe__(id) VALUES (1);\n")...)
	fsys := fstest.MapFS{"0001_initial_schema.sql": &fstest.MapFile{Data: broken}}
	st, err := Open(t.TempDir(), "rollback.db")
	if err != nil {
		t.Fatalf("打开数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), fsys); err == nil {
		t.Fatal("损坏基线应迁移失败")
	}
	var businessTables, versions int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name <> 'schema_migrations'`).Scan(&businessTables); err != nil {
		t.Fatalf("查询回滚后的表失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&versions); err != nil {
		t.Fatalf("查询回滚后的版本失败: %v", err)
	}
	if businessTables != 0 || versions != 0 {
		t.Fatalf("迁移失败残留 business_tables=%d versions=%d", businessTables, versions)
	}
}

// TestBaselineSupportsFutureMigration 验证未来可从首版版本 1 正常升级到测试版本 2。
func TestBaselineSupportsFutureMigration(t *testing.T) {
	baseline, err := fs.ReadFile(migrations.FS, "0001_initial_schema.sql")
	if err != nil {
		t.Fatalf("读取首版基线失败: %v", err)
	}
	fsys := fstest.MapFS{
		"0001_initial_schema.sql": &fstest.MapFile{Data: baseline},
		"0002_probe.sql":          &fstest.MapFile{Data: []byte(`CREATE TABLE migration_v2_probe (id INTEGER PRIMARY KEY);`)},
	}
	st := openMigratedStore(t, "future.db", fsys)
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("版本 2 重复迁移失败: %v", err)
	}
	var versions, maxVersion, probeTables int
	if err := st.DB().QueryRow(`SELECT COUNT(*), MAX(version) FROM schema_migrations`).Scan(&versions, &maxVersion); err != nil {
		t.Fatalf("查询升级版本失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name='migration_v2_probe'`).Scan(&probeTables); err != nil {
		t.Fatalf("查询版本 2 探针表失败: %v", err)
	}
	if versions != 2 || maxVersion != 2 || probeTables != 1 {
		t.Fatalf("未来升级结果 versions=%d max=%d probe=%d，期望 2/2/1", versions, maxVersion, probeTables)
	}
}

// TestBaselineSequenceStarts 验证首版不携带开发期素材池 ID 保留历史。
func TestBaselineSequenceStarts(t *testing.T) {
	st := openMigratedStore(t, "sequence.db", migrations.FS)
	result, err := st.DB().Exec(`INSERT INTO proxy_groups(name, type) VALUES ('custom-sequence', 'custom')`)
	if err != nil {
		t.Fatalf("新增代理组失败: %v", err)
	}
	proxyGroupID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("读取代理组 ID 失败: %v", err)
	}
	result, err = st.DB().Exec(`INSERT INTO rule_pools(name) VALUES ('pool-sequence')`)
	if err != nil {
		t.Fatalf("新增素材池失败: %v", err)
	}
	rulePoolID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("读取素材池 ID 失败: %v", err)
	}
	if proxyGroupID != 10 || rulePoolID != 1 {
		t.Fatalf("首版序列 proxy_group=%d rule_pool=%d，期望 10/1", proxyGroupID, rulePoolID)
	}
}

type constraintProbe struct {
	name      string
	setup     []string
	statement string
}

func assertConstraintProbes(t *testing.T, db *sql.DB) {
	t.Helper()
	probes := []constraintProbe{
		{name: "users role", statement: `INSERT INTO users(username, role, user_source, status) VALUES ('u', 'owner', 'local', 'active')`},
		{name: "users source", statement: `INSERT INTO users(username, role, user_source, status) VALUES ('u', 'user', 'imported', 'active')`},
		{name: "users status", statement: `INSERT INTO users(username, role, user_source, status) VALUES ('u', 'user', 'local', 'deleted')`},
		{name: "oidc state intent", statement: `INSERT INTO oidc_states(state, code_verifier, intent) VALUES ('s', 'v', 'approve')`},
		{name: "versions owner type", statement: `INSERT INTO versions(owner_type, owner_id, version_no, file_path) VALUES ('pool', 1, 1, 'x')`},
		{name: "share token status", statement: `INSERT INTO share_subscriptions(slug, name, token_status) VALUES ('share-a', 'a', 'expired')`},
		{name: "nodes source instance", statement: `INSERT INTO nodes(source, name, protocol, host, port) VALUES ('xray', 'n', 'vmess', '127.0.0.1', 1)`},
		{name: "proxy group type", statement: `INSERT INTO proxy_groups(name, type) VALUES ('custom-invalid', 'fallback')`},
		{
			name: "xray user sync status",
			setup: []string{
				`INSERT INTO users(id, username, role, user_source, status) VALUES (101, 'u', 'user', 'local', 'active')`,
				`INSERT INTO xray_instances(id, name, slug, api_addr) VALUES (101, 'i', 'i', '127.0.0.1:1')`,
				`INSERT INTO nodes(id, source, name, instance_id, tag, protocol, host, port) VALUES (101, 'xray', 'n', 101, 'tag', 'vmess', '127.0.0.1', 1)`,
			},
			statement: `INSERT INTO xray_users(user_id, instance_id, inbound_tag, node_id, email, sync_status) VALUES (101, 101, 'in', 101, 'u@example.invalid', 'unknown')`,
		},
		{
			name:      "xray ext action",
			setup:     []string{`INSERT INTO xray_ext_accounts(id, name, email) VALUES (101, 'a', 'a@example.invalid')`, `INSERT INTO xray_instances(id, name, slug, api_addr) VALUES (101, 'i', 'i', '127.0.0.1:1')`},
			statement: `INSERT INTO xray_ext_users(ext_account_id, instance_id, inbound_tag, sync_status, action) VALUES (101, 101, 'in', 'pending', 'replace')`,
		},
		{
			name:      "assembly target syntax",
			setup:     []string{`INSERT INTO versions(id, owner_type, owner_id, version_no, file_path) VALUES (101, 'rule', 1, 1, 'x')`},
			statement: `INSERT INTO assembly_blueprints(version_id, target_syntax) VALUES (101, 'sing-box')`,
		},
		{
			name:      "pool source kind",
			setup:     []string{`INSERT INTO rule_pools(id, name) VALUES (101, 'p')`},
			statement: `INSERT INTO rule_pool_sources(pool_id, kind, source_mode, sort_order) VALUES (101, 'file', 'auto', 1)`,
		},
		{
			name:      "pool source mode",
			setup:     []string{`INSERT INTO rule_pools(id, name) VALUES (101, 'p')`},
			statement: `INSERT INTO rule_pool_sources(pool_id, kind, source_mode, sort_order) VALUES (101, 'manual', 'surge', 1)`,
		},
		{
			name: "pool snapshot status",
			setup: []string{
				`INSERT INTO rule_pools(id, name) VALUES (101, 'p')`,
				`INSERT INTO rule_pool_sources(id, pool_id, kind, source_mode, sort_order) VALUES (101, 101, 'manual', 'auto', 1)`,
			},
			statement: `INSERT INTO pool_source_snapshots(source_id, status) VALUES (101, 'archived')`,
		},
		{
			name:      "pool sync task status",
			setup:     []string{`INSERT INTO rule_pools(id, name) VALUES (101, 'p')`},
			statement: `INSERT INTO pool_sync_tasks(pool_id, status) VALUES (101, 'cancelled')`,
		},
		{name: "mail result", statement: `INSERT INTO mail_result_logs(kind, source, recipient_masked, result, recorded_at) VALUES ('welcome', 'register', 'a***', 'queued', CURRENT_TIMESTAMP)`},
		{name: "mail accepted failure stage", statement: `INSERT INTO mail_result_logs(kind, source, recipient_masked, result, failure_stage, recorded_at) VALUES ('welcome', 'register', 'a***', 'accepted', 'smtp', CURRENT_TIMESTAMP)`},
		{
			name: "node render name unique",
			setup: []string{
				`INSERT INTO nodes(source, name, display_name, protocol, host, port) VALUES ('manual', 'n1', 'same', 'vmess', '127.0.0.1', 1)`,
			},
			statement: `INSERT INTO nodes(source, name, display_name, protocol, host, port) VALUES ('manual', 'n2', 'same', 'vmess', '127.0.0.1', 2)`,
		},
		{
			name:      "home default unique",
			setup:     []string{`INSERT INTO rules(slug, name, is_home_default) VALUES ('rule-a', 'a', 1)`},
			statement: `INSERT INTO rules(slug, name, is_home_default) VALUES ('rule-b', 'b', 1)`,
		},
		{
			name:      "subscription platform unique",
			setup:     []string{`INSERT INTO platforms(id, slug, name) VALUES (101, 'platform-a', 'a')`, `INSERT INTO subscriptions(slug, name, platform_id) VALUES ('sub-a', 'a', 101)`},
			statement: `INSERT INTO subscriptions(slug, name, platform_id) VALUES ('sub-b', 'b', 101)`,
		},
		{
			name: "custom subscription user platform unique",
			setup: []string{
				`INSERT INTO users(id, username, role, user_source, status) VALUES (101, 'u', 'user', 'local', 'active')`,
				`INSERT INTO platforms(id, slug, name) VALUES (101, 'platform-a', 'a')`,
				`INSERT INTO custom_subscriptions(slug, user_id, platform_id) VALUES ('custom-a', 101, 101)`,
			},
			statement: `INSERT INTO custom_subscriptions(slug, user_id, platform_id) VALUES ('custom-b', 101, 101)`,
		},
		{
			name: "pool source URL unique",
			setup: []string{
				`INSERT INTO rule_pools(id, name) VALUES (101, 'p')`,
				`INSERT INTO rule_pool_sources(pool_id, kind, url, source_mode, sort_order) VALUES (101, 'url', 'https://example.invalid/a', 'auto', 1)`,
			},
			statement: `INSERT INTO rule_pool_sources(pool_id, kind, url, source_mode, sort_order) VALUES (101, 'url', 'https://example.invalid/a', 'auto', 2)`,
		},
		{
			name: "pool manual source unique",
			setup: []string{
				`INSERT INTO rule_pools(id, name) VALUES (101, 'p')`,
				`INSERT INTO rule_pool_sources(pool_id, kind, source_mode, sort_order) VALUES (101, 'manual', 'auto', 1)`,
			},
			statement: `INSERT INTO rule_pool_sources(pool_id, kind, source_mode, sort_order) VALUES (101, 'manual', 'auto', 2)`,
		},
	}
	for _, probe := range probes {
		t.Run(probe.name, func(t *testing.T) {
			tx, err := db.Begin()
			if err != nil {
				t.Fatalf("开启探针事务失败: %v", err)
			}
			defer tx.Rollback()
			for _, statement := range probe.setup {
				if _, err := tx.Exec(statement); err != nil {
					t.Fatalf("探针前置失败: %v", err)
				}
			}
			if _, err := tx.Exec(probe.statement); err == nil {
				t.Fatal("无效数据未被约束拒绝")
			}
		})
	}
}

func openMigratedStore(t *testing.T, name string, fsys fs.FS) *Store {
	t.Helper()
	st, err := Open(t.TempDir(), name)
	if err != nil {
		t.Fatalf("打开临时数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移临时数据库失败: %v", err)
	}
	return st
}

func collectSchemaManifest(t *testing.T, ctx context.Context, db *sql.DB) schemaManifest {
	t.Helper()
	manifest := schemaManifest{}
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("查询表清单失败: %v", err)
	}
	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatalf("读取表名失败: %v", err)
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("关闭表清单结果失败: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历表清单失败: %v", err)
	}

	for _, table := range tableNames {
		manifest.Tables = append(manifest.Tables, tableManifest{
			Name:        table,
			Columns:     collectColumns(t, ctx, db, table),
			ForeignKeys: collectForeignKeys(t, ctx, db, table),
		})
		manifest.Indexes = append(manifest.Indexes, collectIndexes(t, ctx, db, table)...)
		if table != "schema_migrations" && table != "proxy_groups" {
			var count int
			if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quoteIdentifier(table)).Scan(&count); err != nil {
				t.Fatalf("统计表 %s 失败: %v", table, err)
			}
			manifest.Rows = append(manifest.Rows, tableRowCount{Table: table, Count: count})
		}
	}
	sort.Slice(manifest.Indexes, func(i, j int) bool {
		if manifest.Indexes[i].Table != manifest.Indexes[j].Table {
			return manifest.Indexes[i].Table < manifest.Indexes[j].Table
		}
		return manifest.Indexes[i].Name < manifest.Indexes[j].Name
	})

	seedRows, err := db.QueryContext(ctx, `SELECT id, name, type, preset_key, enabled, definition_json FROM proxy_groups ORDER BY id`)
	if err != nil {
		t.Fatalf("查询代理组种子失败: %v", err)
	}
	defer seedRows.Close()
	for seedRows.Next() {
		var seed proxyGroupSeed
		if err := seedRows.Scan(&seed.ID, &seed.Name, &seed.Type, &seed.PresetKey, &seed.Enabled, &seed.DefinitionJSON); err != nil {
			t.Fatalf("读取代理组种子失败: %v", err)
		}
		manifest.Seeds = append(manifest.Seeds, seed)
	}
	if err := seedRows.Err(); err != nil {
		t.Fatalf("遍历代理组种子失败: %v", err)
	}
	return manifest
}

func collectColumns(t *testing.T, ctx context.Context, db *sql.DB, table string) []columnManifest {
	t.Helper()
	rows, err := db.QueryContext(ctx, `PRAGMA table_xinfo(`+quoteIdentifier(table)+`)`)
	if err != nil {
		t.Fatalf("查询表 %s 列失败: %v", table, err)
	}
	defer rows.Close()
	var columns []columnManifest
	for rows.Next() {
		var column columnManifest
		var defaultValue sql.NullString
		if err := rows.Scan(&column.CID, &column.Name, &column.Type, &column.NotNull, &defaultValue, &column.PrimaryKey, &column.Hidden); err != nil {
			t.Fatalf("读取表 %s 列失败: %v", table, err)
		}
		if defaultValue.Valid {
			value := defaultValue.String
			column.Default = &value
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历表 %s 列失败: %v", table, err)
	}
	return columns
}

func collectForeignKeys(t *testing.T, ctx context.Context, db *sql.DB, table string) []foreignKey {
	t.Helper()
	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_list(`+quoteIdentifier(table)+`)`)
	if err != nil {
		t.Fatalf("查询表 %s 外键失败: %v", table, err)
	}
	defer rows.Close()
	var keys []foreignKey
	for rows.Next() {
		var key foreignKey
		if err := rows.Scan(&key.ID, &key.Sequence, &key.Table, &key.From, &key.To, &key.OnUpdate, &key.OnDelete, &key.Match); err != nil {
			t.Fatalf("读取表 %s 外键失败: %v", table, err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历表 %s 外键失败: %v", table, err)
	}
	return keys
}

func collectIndexes(t *testing.T, ctx context.Context, db *sql.DB, table string) []indexManifest {
	t.Helper()
	rows, err := db.QueryContext(ctx, `PRAGMA index_list(`+quoteIdentifier(table)+`)`)
	if err != nil {
		t.Fatalf("查询表 %s 索引失败: %v", table, err)
	}
	var indexes []indexManifest
	for rows.Next() {
		var sequence int
		var index indexManifest
		index.Table = table
		if err := rows.Scan(&sequence, &index.Name, &index.Unique, &index.Origin, &index.Partial); err != nil {
			t.Fatalf("读取表 %s 索引失败: %v", table, err)
		}
		indexes = append(indexes, index)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("关闭表 %s 索引结果失败: %v", table, err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历表 %s 索引失败: %v", table, err)
	}
	// Store 将连接数限制为 1；必须先关闭外层 rows，再执行嵌套 PRAGMA。
	for i := range indexes {
		indexes[i].Columns = collectIndexColumns(t, ctx, db, indexes[i].Name)
	}
	return indexes
}

func collectIndexColumns(t *testing.T, ctx context.Context, db *sql.DB, indexName string) []indexColumn {
	t.Helper()
	rows, err := db.QueryContext(ctx, `PRAGMA index_xinfo(`+quoteIdentifier(indexName)+`)`)
	if err != nil {
		t.Fatalf("查询索引 %s 列失败: %v", indexName, err)
	}
	defer rows.Close()
	var columns []indexColumn
	for rows.Next() {
		var column indexColumn
		var name sql.NullString
		if err := rows.Scan(&column.Sequence, &column.CID, &name, &column.Desc, &column.Coll, &column.Key); err != nil {
			t.Fatalf("读取索引 %s 列失败: %v", indexName, err)
		}
		if name.Valid {
			value := name.String
			column.Name = &value
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("遍历索引 %s 列失败: %v", indexName, err)
	}
	return columns
}

func assertDatabaseIntegrity(t *testing.T, db *sql.DB) {
	t.Helper()
	rows, err := db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatalf("执行 foreign_key_check 失败: %v", err)
	}
	if rows.Next() {
		rows.Close()
		t.Fatal("foreign_key_check 返回异常记录")
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("关闭 foreign_key_check 结果失败: %v", err)
	}
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		t.Fatalf("执行 integrity_check 失败: %v", err)
	}
	if result != "ok" {
		t.Fatalf("integrity_check = %q，期望 ok", result)
	}
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func formatManifest(value schemaManifest) string {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("<manifest marshal failed: %v>", err)
	}
	return string(data)
}
