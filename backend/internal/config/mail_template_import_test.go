package config_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/mail"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/migrations"
)

func newMailTemplateExportHarness(t *testing.T) (*store.Store, *config.Service, *config.ExportService) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	lg := log.New("error", "console")
	cfg := config.NewService(st, lg)
	svc := config.NewExportService(st, cfg, t.TempDir(), "prod", lg)
	return st, cfg, svc
}

func snapshotConfig(t *testing.T, st *store.Store) map[string]string {
	t.Helper()
	rows, err := st.DB().Query(`SELECT key, value FROM system_config`)
	if err != nil {
		t.Fatalf("读取配置快照失败: %v", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			t.Fatal(err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func seedValidMailTemplates(t *testing.T, cfg *config.Service) {
	t.Helper()
	ctx := context.Background()
	for _, def := range mail.Definitions() {
		raw, err := json.Marshal(def.Default)
		if err != nil {
			t.Fatal(err)
		}
		if err := cfg.Set(ctx, def.ConfigKey, string(raw)); err != nil {
			t.Fatalf("写入模板 %s 失败: %v", def.ID, err)
		}
	}
}

func TestExportImportPreservesMailTemplateOverrides(t *testing.T) {
	ctx := context.Background()
	_, sourceCfg, sourceSvc := newMailTemplateExportHarness(t)
	seedValidMailTemplates(t, sourceCfg)
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}

	_, targetCfg, targetSvc := newMailTemplateExportHarness(t)
	targetSvc.SetValidateConfig(mail.ValidateTemplateOverrides)
	if err := targetSvc.Import(ctx, data, "export-pass-123", config.ConfirmWordImport, false); err != nil {
		t.Fatalf("合法五键导入失败: %v", err)
	}
	mailSvc := mail.NewService(targetCfg, log.New("error", "console"))
	for _, def := range mail.Definitions() {
		tpl, state, err := mailSvc.LoadTemplate(ctx, def.ID)
		if err != nil || state != mail.TemplateStateCustomized || tpl != def.Default {
			t.Fatalf("导入后 %s 应为 customized 且值往返: tpl=%+v state=%s err=%v", def.ID, tpl, state, err)
		}
	}
}

// TestUnknownMailTemplatePrefixSurvivesImport 邮件回调不得拒绝或改写未知 mail_template_ 前缀键；
// 缺键站点导入后仍使用内置默认模板。
func TestUnknownMailTemplatePrefixSurvivesImport(t *testing.T) {
	ctx := context.Background()
	_, sourceCfg, sourceSvc := newMailTemplateExportHarness(t)
	const unknownKey = "mail_template_future_branch"
	const unknownRaw = `{not-json`
	if err := sourceCfg.Set(ctx, unknownKey, unknownRaw); err != nil {
		t.Fatal(err)
	}
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatal(err)
	}

	targetSt, targetCfg, targetSvc := newMailTemplateExportHarness(t)
	targetSvc.SetValidateConfig(mail.ValidateTemplateOverrides)
	if err := targetSvc.Import(ctx, data, "export-pass-123", config.ConfirmWordImport, false); err != nil {
		t.Fatalf("未知前缀键不应被邮件回调拒绝: %v", err)
	}
	var got string
	if err := targetSt.DB().QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, unknownKey).Scan(&got); err != nil {
		t.Fatalf("未知前缀键应原样保留: %v", err)
	}
	if got != unknownRaw {
		t.Fatalf("未知前缀键不得被改写: got=%q want=%q", got, unknownRaw)
	}
	mailSvc := mail.NewService(targetCfg, log.New("error", "console"))
	if _, state, err := mailSvc.LoadTemplate(ctx, mail.TemplatePasswordReset); err != nil || state != mail.TemplateStateDefault {
		t.Fatalf("缺键站点应使用内置默认: state=%s err=%v", state, err)
	}
}

// TestImportV2InvalidKnownTemplateTaskFailedWithoutOverwrite v2 先建任务，模板非法必须 task failed，
// 且不得开始覆盖事务或后处理。
func TestImportV2InvalidKnownTemplateTaskFailedWithoutOverwrite(t *testing.T) {
	ctx := context.Background()
	_, sourceCfg, sourceSvc := newMailTemplateExportHarness(t)
	if err := sourceCfg.Set(ctx, mail.ConfigKeyPasswordReset, `{"subject":"x","body":"缺少必需变量"}`); err != nil {
		t.Fatal(err)
	}
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatal(err)
	}

	targetSt, targetCfg, targetSvc := newMailTemplateExportHarness(t)
	if err := targetCfg.Set(ctx, "keep_key", "keep-value"); err != nil {
		t.Fatal(err)
	}
	before := snapshotConfig(t, targetSt)
	registry := tasks.NewRegistry()
	targetSvc.SetTaskRegistry(registry)
	targetSvc.SetValidateConfig(mail.ValidateTemplateOverrides)
	detectCalled, postCalled := 0, 0
	targetSvc.SetDetectImportedInstances(func(context.Context, *config.ExportPayload) []string {
		detectCalled++
		return nil
	})
	targetSvc.SetPostImportRebindReconcile(func(context.Context, *config.ExportPayload) []string {
		postCalled++
		return nil
	})

	taskID, err := targetSvc.ImportV2(ctx, data, "export-pass-123", config.ConfirmWordImport, config.ConfirmWordDisable, false)
	if err != nil {
		t.Fatalf("v2 入口应注册任务后异步失败: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		task := registry.Get(taskID)
		if task.Status == tasks.StatusFailed {
			if !strings.Contains(task.Error, "邮件模板") {
				t.Fatalf("任务失败原因应为安全模板错误: %q", task.Error)
			}
			break
		}
		if task.Status == tasks.StatusSucceeded {
			t.Fatalf("非法模板 v2 任务不得成功: %+v", task)
		}
		if time.Now().After(deadline) {
			t.Fatalf("等待 v2 任务 failed 超时: %+v", task)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if after := snapshotConfig(t, targetSt); !reflect.DeepEqual(before, after) {
		t.Fatalf("v2 非法模板不得覆盖配置:\nbefore=%v\nafter=%v", before, after)
	}
	if detectCalled != 0 || postCalled != 0 {
		t.Fatalf("v2 非法模板不得调用后处理: detect=%d post=%d", detectCalled, postCalled)
	}
}

func TestImportRejectsInvalidKnownTemplateBeforeOverwrite(t *testing.T) {
	ctx := context.Background()
	_, sourceCfg, sourceSvc := newMailTemplateExportHarness(t)
	if err := sourceCfg.Set(ctx, mail.ConfigKeyPasswordReset, `{"subject":"x","body":"缺少必需变量"}`); err != nil {
		t.Fatal(err)
	}
	data, err := sourceSvc.Export(ctx, "export-pass-123")
	if err != nil {
		t.Fatal(err)
	}

	targetSt, targetCfg, targetSvc := newMailTemplateExportHarness(t)
	if err := targetCfg.Set(ctx, "keep_key", "keep-value"); err != nil {
		t.Fatal(err)
	}
	before := snapshotConfig(t, targetSt)
	targetSvc.SetValidateConfig(mail.ValidateTemplateOverrides)
	err = targetSvc.Import(ctx, data, "export-pass-123", config.ConfirmWordImport, false)
	if !errors.Is(err, mail.ErrInvalidTemplate) {
		t.Fatalf("v1 非法已知模板应返回 ErrInvalidTemplate: %v", err)
	}
	after := snapshotConfig(t, targetSt)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("拒绝导入后配置不得变化:\nbefore=%v\nafter=%v", before, after)
	}
}
