package config

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newTestExport 创建临时库 + 导出服务（mode 可指定）
func newTestExport(t *testing.T, mode string) (*store.Store, *ExportService) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := NewService(st, log.New("error", "console"))
	svc := NewExportService(st, cfg, t.TempDir(), mode, log.New("error", "console"))
	// Setup 预置注入（测试 mock：仅验证被调用）
	svc.SetSeedPresets(func(ctx context.Context, tx *sql.Tx, frontendURL string) error {
		if _, err := tx.ExecContext(ctx, `INSERT INTO system_config (key, value) VALUES ('seed_marker','1')`); err != nil {
			return err
		}
		return nil
	})
	return st, svc
}

// seedExportConfig 写入测试配置（含签名密钥与唯一键）
func seedExportConfig(t *testing.T, st *store.Store, extra map[string]string) {
	t.Helper()
	cfg := NewService(st, log.New("error", "console"))
	ctx := context.Background()
	for k, v := range map[string]string{
		"configured":  "true",
		"signing_key": "test-signing-key-32bytes!!",
		"site_name":   "测试站点",
	} {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置失败: %v", err)
		}
	}
	for k, v := range extra {
		if err := cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置失败: %v", err)
		}
	}
}

// TestExportImportRoundTrip 导出/导入往返：解密后配置一致；错误密码解密失败
func TestExportImportRoundTrip(t *testing.T) {
	st, svc := newTestExport(t, "prod")
	ctx := context.Background()
	seedExportConfig(t, st, nil)

	data, err := svc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("导出内容为空")
	}
	// 导入到新库（整体覆盖）：先清目标键再写
	st2, svc2 := newTestExport(t, "prod")
	if err := svc2.Import(ctx, data, "export-pass-123", ConfirmWordImport, false); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	cfg2 := NewService(st2, log.New("error", "console"))
	siteName, _ := cfg2.Get(ctx, "site_name")
	if siteName != "测试站点" || !cfg2.GetBool(ctx, "configured", false) {
		t.Error("导入后配置不一致")
	}
	// 错误密码解密失败
	if err := svc2.Import(ctx, data, "wrong-password", ConfirmWordImport, false); err == nil {
		t.Error("错误密码应解密失败")
	}
	// 确认词错误
	if err := svc2.Import(ctx, data, "export-pass-123", "WRONG", false); err == nil {
		t.Error("确认词错误应拒绝")
	}
}

// TestImportStrictOverwrite 导入严格整体覆盖：目标实例独有键被清除
func TestImportStrictOverwrite(t *testing.T) {
	st, svc := newTestExport(t, "prod")
	ctx := context.Background()
	seedExportConfig(t, st, nil)

	data, err := svc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	// 目标库有独有键 unique_local（导出文件中不存在）
	st2, svc2 := newTestExport(t, "prod")
	seedExportConfig(t, st2, nil)
	if err := NewService(st2, log.New("error", "console")).Set(ctx, "unique_local", "only-here"); err != nil {
		t.Fatalf("写入独有键失败: %v", err)
	}
	if err := svc2.Import(ctx, data, "export-pass-123", ConfirmWordImport, false); err != nil {
		t.Fatalf("导入失败: %v", err)
	}
	cfg2 := NewService(st2, log.New("error", "console"))
	if v, _ := cfg2.Get(ctx, "unique_local"); v != "" {
		t.Errorf("独有键应被整体覆盖清除: %q", v)
	}
}

// TestImportFailureRollback 导入失败回滚：库内配置无变更
func TestImportFailureRollback(t *testing.T) {
	st, svc := newTestExport(t, "prod")
	ctx := context.Background()
	seedExportConfig(t, st, nil)
	data, err := svc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	// 篡改数据（模拟损坏文件）→ 解密失败 → 无任何变更
	st2, svc2 := newTestExport(t, "prod")
	seedExportConfig(t, st2, nil)
	corrupted := append([]byte{}, data...)
	corrupted[len(corrupted)-1] ^= 0xFF
	if err := svc2.Import(ctx, corrupted, "export-pass-123", ConfirmWordImport, false); err == nil {
		t.Fatal("损坏文件应解密失败")
	}
	cfg2 := NewService(st2, log.New("error", "console"))
	if v, _ := cfg2.Get(ctx, "site_name"); v != "测试站点" {
		t.Errorf("导入失败后配置应无变更: %q", v)
	}
}

// TestImportSetupModeSeedsPresets Setup 导入分支：同事务预置默认组与默认平台（seed 回调被调用）
func TestImportSetupModeSeedsPresets(t *testing.T) {
	st, svc := newTestExport(t, "prod")
	ctx := context.Background()
	seedExportConfig(t, st, nil)
	data, err := svc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	st2, svc2 := newTestExport(t, "prod")
	if err := svc2.Import(ctx, data, "export-pass-123", ConfirmWordImport, true); err != nil {
		t.Fatalf("Setup 导入失败: %v", err)
	}
	var marker string
	if err := st2.DB().QueryRow(`SELECT value FROM system_config WHERE key = 'seed_marker'`).Scan(&marker); err != nil {
		t.Fatalf("seed 回调应被调用: %v", err)
	}
}

// TestExportDevModeDenied Dev 模式：导出/导入返回 ErrModeRestricted 哨兵（接入层映射 403，R07-06）
func TestExportDevModeDenied(t *testing.T) {
	st, svc := newTestExport(t, "dev")
	ctx := context.Background()
	if _, err := svc.Export(ctx, "export-pass-123"); !errors.Is(err, ErrModeRestricted) {
		t.Errorf("Dev 模式导出应拒绝并返回哨兵错误: %v", err)
	}
	if err := svc.Import(ctx, []byte("x"), "export-pass-123", ConfirmWordImport, false); !errors.Is(err, ErrModeRestricted) {
		t.Errorf("Dev 模式导入应拒绝并返回哨兵错误: %v", err)
	}
	if err := svc.Import(ctx, []byte("x"), "export-pass-123", ConfirmWordImport, true); !errors.Is(err, ErrModeRestricted) {
		t.Errorf("Dev 模式 Setup 导入应拒绝并返回哨兵错误: %v", err)
	}
	_ = st
}

// TestExportPasswordTooShort 导出密码 <8 拒绝
func TestExportPasswordTooShort(t *testing.T) {
	_, svc := newTestExport(t, "prod")
	ctx := context.Background()
	if _, err := svc.Export(ctx, "short"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("短密码应拒绝: %v", err)
	}
}

// TestImportProtectionCoversExtensions 导入保护同时覆盖节点扩展密文。
func TestImportProtectionCoversExtensions(t *testing.T) {
	ctx := context.Background()
	source, sourceSvc := newTestExport(t, "prod")
	seedExportConfig(t, source, nil)
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	target, targetSvc := newTestExport(t, "prod")
	seedExportConfig(t, target, nil)
	if _, err := target.DB().ExecContext(ctx, `CREATE TABLE nodes (
		protocol_json TEXT NOT NULL,
		extensions_json TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("创建节点测试表失败: %v", err)
	}
	if _, err := target.DB().ExecContext(ctx,
		`INSERT INTO nodes (protocol_json, extensions_json) VALUES ('{}', '{"entries":[{"id":"ext-1","scope":"node","status":"encrypted","payload_encrypted":"enc:ext:v1:encrypted"}]}')`); err != nil {
		t.Fatalf("写入扩展密文失败: %v", err)
	}
	targetCfg := NewService(target, log.New("error", "console"))
	if err := targetCfg.Set(ctx, KeySigningKey, "different-signing-key-0123456789"); err != nil {
		t.Fatalf("修改目标签名密钥失败: %v", err)
	}
	if err := targetSvc.Import(ctx, data, "export-pass-123", ConfirmWordImport, false); err == nil {
		t.Fatal("目标存在扩展密文且签名密钥变化时应拒绝导入")
	}

	sameKeyTarget, sameKeySvc := newTestExport(t, "prod")
	seedExportConfig(t, sameKeyTarget, nil)
	if _, err := sameKeyTarget.DB().ExecContext(ctx, `CREATE TABLE nodes (
		protocol_json TEXT NOT NULL,
		extensions_json TEXT NOT NULL
	)`); err != nil {
		t.Fatalf("创建同密钥节点测试表失败: %v", err)
	}
	if _, err := sameKeyTarget.DB().ExecContext(ctx,
		`INSERT INTO nodes (protocol_json, extensions_json) VALUES ('{}', '{"entries":[{"id":"ext-1","scope":"node","status":"encrypted","payload_encrypted":"enc:ext:v1:encrypted"}]}')`); err != nil {
		t.Fatalf("写入同密钥扩展密文失败: %v", err)
	}
	if err := sameKeySvc.Import(ctx, data, "export-pass-123", ConfirmWordImport, false); err != nil {
		t.Fatalf("同签名密钥导入不应被扩展保护拦截: %v", err)
	}
}
