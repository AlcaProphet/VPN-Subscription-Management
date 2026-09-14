package config

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/migrations"
)

// newFullExportTest 使用完整迁移构造导出服务测试环境。
func newFullExportTest(t *testing.T) (*store.Store, *ExportService) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := NewService(st, log.New("error", "console"))
	svc := NewExportService(st, cfg, t.TempDir(), "prod", log.New("error", "console"))
	return st, svc
}

// TestImportV2ExtRebindUnmatchedMarksFailed 导入后 ext 推送目标未匹配节点时置 failed，避免 NULL+pending 悬挂。
func TestImportV2ExtRebindUnmatchedMarksFailed(t *testing.T) {
	st, svc := newFullExportTest(t)
	ctx := context.Background()
	payload := &ExportPayload{
		FormatVersion: FormatVersion,
		Config:        map[string]string{"site_name": "测试"},
		Instances: []ExportedInstance{{
			Name: "inst", Slug: "instance-abc", APIAddr: "127.0.0.1:10086", Enabled: true,
		}},
		Accounts: []ExportedExtAccount{{
			Name: "ext", Email: "ext-1@vpn.local", UUIDEncrypted: "", ProxySecretEncrypted: "",
			PushTargets: []ExportedExtPushTarget{{InstanceSlug: "instance-abc", InboundTag: "in-missing"}},
		}},
	}
	if _, err := svc.importV2(ctx, payload, ConfirmWordImport, false); err != nil {
		t.Fatalf("importV2 失败: %v", err)
	}
	var accID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM xray_ext_accounts WHERE email='ext-1@vpn.local'`).Scan(&accID); err != nil {
		t.Fatalf("读取导入账号失败: %v", err)
	}
	var status, lastErr string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT sync_status, last_error FROM xray_ext_users WHERE ext_account_id=?`, accID).Scan(&status, &lastErr); err != nil {
		t.Fatalf("读取推送目标状态失败: %v", err)
	}
	if status != "failed" || !strings.Contains(lastErr, "导入重绑未匹配节点") {
		t.Fatalf("未匹配节点应置 failed+last_error，实际 status=%s last_error=%q", status, lastErr)
	}
}

// TestImportV2ReturnsHints importV2 应将后处理提示作为返回值上抛，供任务终态写入。
func TestImportV2ReturnsHints(t *testing.T) {
	_, svc := newFullExportTest(t)
	ctx := context.Background()
	svc.SetDetectImportedInstances(func(context.Context, *ExportPayload) []string {
		return []string{"检测提示"}
	})
	svc.SetPostImportRebindReconcile(func(context.Context, *ExportPayload) []string {
		return []string{"对账提示"}
	})
	payload := &ExportPayload{
		FormatVersion: FormatVersion,
		Config:        map[string]string{"site_name": "测试"},
	}
	hints, err := svc.importV2(ctx, payload, ConfirmWordImport, false)
	if err != nil {
		t.Fatalf("importV2 失败: %v", err)
	}
	found := map[string]bool{}
	for _, h := range hints {
		found[h] = true
	}
	if !found["检测提示"] || !found["对账提示"] {
		t.Fatalf("hints 应包含注入的后处理提示，实际 %v", hints)
	}
}

// TestImportV2DisableConfirmationByEntry 确认 DISABLE 只保护已有系统的覆盖导入，Setup 新库不得被阻断。
func TestImportV2DisableConfirmationByEntry(t *testing.T) {
	ctx := context.Background()
	sourceStore, sourceSvc := newFullExportTest(t)
	sourceCfg := NewService(sourceStore, log.New("error", "console"))
	for key, value := range map[string]string{
		KeyConfigured:      "true",
		KeyAllowLocalLogin: "true",
	} {
		if err := sourceCfg.Set(ctx, key, value); err != nil {
			t.Fatalf("写入导出配置 %s 失败: %v", key, err)
		}
	}
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("生成 v2 导出文件失败: %v", err)
	}

	t.Run("setup import only requires IMPORT", func(t *testing.T) {
		_, svc := newFullExportTest(t)
		registry := tasks.NewRegistry()
		svc.SetTaskRegistry(registry)
		svc.SetSeedPresets(func(context.Context, *sql.Tx, string) error { return nil })
		taskID, err := svc.ImportV2(ctx, data, "export-pass-123", ConfirmWordImport, "", true)
		if err != nil {
			t.Fatalf("Setup 新库导入不应要求 DISABLE: %v", err)
		}
		waitImportTask(t, registry, taskID)
	})

	t.Run("admin import still requires DISABLE", func(t *testing.T) {
		_, svc := newFullExportTest(t)
		if _, err := svc.ImportV2(ctx, data, "export-pass-123", ConfirmWordImport, "", false); err == nil || !strings.Contains(err.Error(), ConfirmWordDisable) {
			t.Fatalf("管理面板导入缺少 DISABLE 应被拒绝，实际 err=%v", err)
		}
	})

	t.Run("admin import accepts DISABLE", func(t *testing.T) {
		_, svc := newFullExportTest(t)
		registry := tasks.NewRegistry()
		svc.SetTaskRegistry(registry)
		taskID, err := svc.ImportV2(ctx, data, "export-pass-123", ConfirmWordImport, ConfirmWordDisable, false)
		if err != nil {
			t.Fatalf("管理面板导入提供 DISABLE 后应提交成功: %v", err)
		}
		waitImportTask(t, registry, taskID)
	})
}

func waitImportTask(t *testing.T, registry *tasks.Registry, taskID string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		task := registry.Get(taskID)
		if task.Status == tasks.StatusSucceeded {
			return
		}
		if task.Status == tasks.StatusFailed {
			t.Fatalf("导入任务失败: %s", task.Error)
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("等待导入任务完成超时")
}
