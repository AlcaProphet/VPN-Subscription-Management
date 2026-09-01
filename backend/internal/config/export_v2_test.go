package config

import (
	"context"
	"strings"
	"testing"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
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
