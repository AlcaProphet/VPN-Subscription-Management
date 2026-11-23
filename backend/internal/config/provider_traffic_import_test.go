package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
)

// insertProviderTrafficBinding 在完整迁移库中写入一条合成功能绑定，供导入/导出边界测试使用。
func insertProviderTrafficBinding(t *testing.T, st *store.Store, instanceID, ciphertext string) {
	t.Helper()
	if _, err := st.DB().ExecContext(context.Background(),
		`INSERT INTO provider_traffic_settings
		 (id, enabled, kiwivm_enabled, kiwivm_instance_id, kiwivm_credential_ciphertext, kiwivm_revision)
		 VALUES (1, 1, 1, ?, ?, 'synthetic-revision')`, instanceID, ciphertext); err != nil {
		t.Fatalf("写入功能绑定失败: %v", err)
	}
}

func providerTrafficRows(t *testing.T, st *store.Store) int {
	t.Helper()
	var rows int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM provider_traffic_settings`).Scan(&rows); err != nil {
		t.Fatalf("查询功能绑定失败: %v", err)
	}
	return rows
}

// TestExportExcludesProviderTrafficBinding 配置导出不包含功能表绑定、实例 ID 或凭据密文。
func TestExportExcludesProviderTrafficBinding(t *testing.T) {
	ctx := context.Background()
	st, svc := newFullExportTest(t)
	seedExportConfig(t, st, nil)
	insertProviderTrafficBinding(t, st, "900000001", "synthetic-ciphertext-900000001")

	data, err := svc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	payload, err := svc.decrypt(data, "export-pass-123")
	if err != nil {
		t.Fatalf("解密导出内容失败: %v", err)
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("序列化导出内容失败: %v", err)
	}
	for _, forbidden := range []string{"900000001", "synthetic-ciphertext-900000001", "synthetic-revision", "provider_traffic"} {
		if strings.Contains(string(plain), forbidden) {
			t.Fatalf("导出包不应包含功能绑定信息 %q: %s", forbidden, plain)
		}
	}
	for key := range payload.Config {
		if strings.HasPrefix(key, "provider_traffic") || strings.HasPrefix(key, "kiwivm_") || strings.HasPrefix(key, "vps_") {
			t.Fatalf("导出配置不应包含功能键 %q", key)
		}
	}
}

// TestImportV1ClearsProviderTrafficBinding v1 同步覆盖成功后，功能表回到关闭且未绑定。
func TestImportV1ClearsProviderTrafficBinding(t *testing.T) {
	ctx := context.Background()
	source, sourceSvc := newFullExportTest(t)
	seedExportConfig(t, source, nil)
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	target, targetSvc := newFullExportTest(t)
	seedExportConfig(t, target, nil)
	insertProviderTrafficBinding(t, target, "900000002", "synthetic-ciphertext-900000002")

	if err := targetSvc.Import(ctx, data, "export-pass-123", ConfirmWordImport, false); err != nil {
		t.Fatalf("v1 导入失败: %v", err)
	}
	if rows := providerTrafficRows(t, target); rows != 0 {
		t.Fatalf("v1 导入后功能表应为空，实际 %d 行", rows)
	}
	if siteName := mustStr(NewService(target, log.New("error", "console")).Get(ctx, "site_name")); siteName != "测试站点" {
		t.Fatalf("v1 导入后站点名称不一致: %q", siteName)
	}
}

// TestImportV2ClearsProviderTrafficBinding v2 覆盖成功后，功能表同样回到关闭且未绑定。
func TestImportV2ClearsProviderTrafficBinding(t *testing.T) {
	ctx := context.Background()
	source, sourceSvc := newFullExportTest(t)
	seedExportConfig(t, source, nil)
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	payload, err := sourceSvc.decrypt(data, "export-pass-123")
	if err != nil {
		t.Fatalf("解密导出内容失败: %v", err)
	}

	target, targetSvc := newFullExportTest(t)
	seedExportConfig(t, target, nil)
	insertProviderTrafficBinding(t, target, "900000003", "synthetic-ciphertext-900000003")

	if _, err := targetSvc.importV2(ctx, payload, ConfirmWordImport, false); err != nil {
		t.Fatalf("v2 导入失败: %v", err)
	}
	if rows := providerTrafficRows(t, target); rows != 0 {
		t.Fatalf("v2 导入后功能表应为空，实际 %d 行", rows)
	}
}

// TestImportV1RollbackPreservesProviderTrafficBinding v1 覆盖事务失败时保留原功能绑定。
func TestImportV1RollbackPreservesProviderTrafficBinding(t *testing.T) {
	ctx := context.Background()
	source, sourceSvc := newFullExportTest(t)
	seedExportConfig(t, source, nil)
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	target, targetSvc := newFullExportTest(t)
	seedExportConfig(t, target, nil)
	insertProviderTrafficBinding(t, target, "900000004", "synthetic-ciphertext-900000004")
	targetSvc.SetSeedPresets(func(context.Context, *sql.Tx, string) error {
		return errors.New("forced rollback")
	})

	if err := targetSvc.Import(ctx, data, "export-pass-123", ConfirmWordImport, true); err == nil {
		t.Fatal("强制失败应返回错误")
	}
	var instanceID, ciphertext, revision string
	if err := target.DB().QueryRow(
		`SELECT kiwivm_instance_id, kiwivm_credential_ciphertext, kiwivm_revision
		 FROM provider_traffic_settings WHERE id = 1`).Scan(&instanceID, &ciphertext, &revision); err != nil {
		t.Fatalf("回滚后读取功能绑定失败: %v", err)
	}
	if instanceID != "900000004" || ciphertext != "synthetic-ciphertext-900000004" || revision != "synthetic-revision" {
		t.Fatalf("v1 回滚后绑定被改变: instance=%q ciphertext=%q revision=%q", instanceID, ciphertext, revision)
	}
}

// TestImportV2RollbackPreservesProviderTrafficBinding v2 覆盖事务失败时保留原功能绑定。
func TestImportV2RollbackPreservesProviderTrafficBinding(t *testing.T) {
	ctx := context.Background()
	source, sourceSvc := newFullExportTest(t)
	seedExportConfig(t, source, nil)
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	payload, err := sourceSvc.decrypt(data, "export-pass-123")
	if err != nil {
		t.Fatalf("解密导出内容失败: %v", err)
	}

	target, targetSvc := newFullExportTest(t)
	seedExportConfig(t, target, nil)
	insertProviderTrafficBinding(t, target, "900000005", "synthetic-ciphertext-900000005")
	targetSvc.SetSeedPresets(func(context.Context, *sql.Tx, string) error {
		return errors.New("forced rollback")
	})

	if _, err := targetSvc.importV2(ctx, payload, ConfirmWordImport, true); err == nil {
		t.Fatal("强制失败应返回错误")
	}
	var instanceID, ciphertext, revision string
	if err := target.DB().QueryRow(
		`SELECT kiwivm_instance_id, kiwivm_credential_ciphertext, kiwivm_revision
		 FROM provider_traffic_settings WHERE id = 1`).Scan(&instanceID, &ciphertext, &revision); err != nil {
		t.Fatalf("回滚后读取功能绑定失败: %v", err)
	}
	if instanceID != "900000005" || ciphertext != "synthetic-ciphertext-900000005" || revision != "synthetic-revision" {
		t.Fatalf("v2 回滚后绑定被改变: instance=%q ciphertext=%q revision=%q", instanceID, ciphertext, revision)
	}
}

// TestImportProviderTrafficSchemaReadFailure 验证 v1 同步和 v2 异步导入在查表失败时整体回滚。
func TestImportProviderTrafficSchemaReadFailure(t *testing.T) {
	ctx := context.Background()
	source, sourceSvc := newFullExportTest(t)
	seedExportConfig(t, source, map[string]string{"site_name": "导入的新名称"})
	const password = "export-pass-123"
	data, err := sourceSvc.Export(ctx, password)
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			target, svc := newFullExportTest(t)
			const originalName = "原站点名称"
			const originalInstance = "900000006"
			const originalCiphertext = "synthetic-ciphertext-900000006"
			seedExportConfig(t, target, map[string]string{"site_name": originalName})
			insertProviderTrafficBinding(t, target, originalInstance, originalCiphertext)

			// 单连接测试库中，60 字节限制允许覆盖 system_config，但会拒绝更长的 sqlite_master 查表语句。
			previousLimit := setImportTestSQLLengthLimit(t, target, 60)
			var failure string
			switch version {
			case "v1":
				if err := svc.Import(ctx, data, password, ConfirmWordImport, false); err != nil {
					failure = err.Error()
				}
			case "v2":
				registry := tasks.NewRegistry()
				svc.SetTaskRegistry(registry)
				taskID, err := svc.ImportV2(ctx, data, password, ConfirmWordImport, ConfirmWordDisable, false)
				if err != nil {
					t.Fatalf("提交导入任务失败: %v", err)
				}
				deadline := time.Now().Add(2 * time.Second)
				task := registry.Get(taskID)
				for task.Status == tasks.StatusRunning && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
					task = registry.Get(taskID)
				}
				if task.Status != tasks.StatusFailed {
					t.Errorf("导入任务应以失败结束，实际为 %s", task.Status)
				}
				failure = task.Error
			}
			setImportTestSQLLengthLimit(t, target, previousLimit)

			if !strings.Contains(failure, "检查服务器剩余流量功能表失败") {
				t.Errorf("应返回功能表查询错误，实际为 %q", failure)
			}
			if name, err := svc.cfg.Get(ctx, "site_name"); err != nil || name != originalName {
				t.Errorf("回滚后应保留原站点名称，实际为 %q，错误为 %v", name, err)
			}
			var enabled, providerEnabled int
			var instanceID, ciphertext, revision string
			if err := target.DB().QueryRowContext(ctx,
				`SELECT enabled, kiwivm_enabled, kiwivm_instance_id, kiwivm_credential_ciphertext, kiwivm_revision
				 FROM provider_traffic_settings WHERE id = 1`).Scan(&enabled, &providerEnabled, &instanceID, &ciphertext, &revision); err != nil {
				t.Fatalf("回滚后读取原绑定失败: %v", err)
			}
			if enabled != 1 || providerEnabled != 1 || instanceID != originalInstance ||
				ciphertext != originalCiphertext || revision != "synthetic-revision" {
				t.Error("回滚后应保留原绑定与开关")
			}
		})
	}
}

func setImportTestSQLLengthLimit(t *testing.T, st *store.Store, limit int) int {
	t.Helper()
	conn, err := st.DB().Conn(context.Background())
	if err != nil {
		t.Fatalf("获取 SQLite 连接失败: %v", err)
	}
	previous, limitErr := sqlite.Limit(conn, sqlite3.SQLITE_LIMIT_SQL_LENGTH, limit)
	if err := conn.Close(); err != nil {
		t.Fatalf("归还 SQLite 连接失败: %v", err)
	}
	if limitErr != nil {
		t.Fatalf("设置 SQLite SQL 长度限制失败: %v", limitErr)
	}
	return previous
}

// TestImportWithoutProviderTrafficTable 仅含 0001 的最小库允许导入时功能表不存在。
func TestImportWithoutProviderTrafficTable(t *testing.T) {
	ctx := context.Background()
	source, sourceSvc := newTestExport(t, "prod")
	seedExportConfig(t, source, nil)
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	t.Run("v1", func(t *testing.T) {
		target, targetSvc := newTestExport(t, "prod")
		if err := targetSvc.Import(ctx, data, "export-pass-123", ConfirmWordImport, false); err != nil {
			t.Fatalf("v1 导入应跳过缺失功能表: %v", err)
		}
		if siteName := mustStr(NewService(target, log.New("error", "console")).Get(ctx, "site_name")); siteName != "测试站点" {
			t.Fatalf("v1 导入后站点名称不一致: %q", siteName)
		}
	})

	t.Run("v2", func(t *testing.T) {
		_, targetSvc := newTestExport(t, "prod")
		payload := &ExportPayload{
			FormatVersion: FormatVersion,
			Config:        map[string]string{"site_name": "无功能表库"},
		}
		if _, err := targetSvc.importV2(ctx, payload, ConfirmWordImport, false); err != nil {
			t.Fatalf("v2 导入应跳过缺失功能表: %v", err)
		}
	})
}
