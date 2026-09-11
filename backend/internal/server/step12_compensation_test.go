package server

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func step12GenerateBody() map[string]any {
	return map[string]any{
		"target_syntax": "sr-conf",
		"rule_name":     "Step12自动规则",
		"fixed_params":  map[string]any{"loglevel": "warning"},
		"pools":         []any{},
		"custom_rules": []map[string]any{
			{"rule_type": "DOMAIN-SUFFIX", "match_value": "example.com", "target": "PROXY"},
		},
		"final_direction": "DIRECT",
	}
}

// TestAssemblyAutoRuleCompensationFailureIsVisible 自动建规则后版本创建失败时，
// 补偿删除失败必须写结构化日志并保留可定位的 rule_id，不能静默当作已回滚。
func TestAssemblyAutoRuleCompensationFailureIsVisible(t *testing.T) {
	var logBuf bytes.Buffer
	lg := slog.New(slog.NewTextHandler(&logBuf, nil))
	engine, st, _, _ := newAssemblyTestEnvWithLogger(t, lg)
	ctx := context.Background()

	if _, err := st.DB().ExecContext(ctx, `
		CREATE TRIGGER step12_fail_version BEFORE INSERT ON versions
		BEGIN
			SELECT RAISE(ABORT, 'step12 version fail');
		END;`); err != nil {
		t.Fatalf("创建版本失败触发器失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, `
		CREATE TRIGGER step12_fail_rule_delete BEFORE DELETE ON rules
		BEGIN
			SELECT RAISE(ABORT, 'step12 rule delete fail');
		END;`); err != nil {
		t.Fatalf("创建规则删除失败触发器失败: %v", err)
	}

	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", step12GenerateBody())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("版本创建失败应返回 500: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(logBuf.String(), "回滚自动创建的分流规则失败") ||
		!strings.Contains(logBuf.String(), "rule_id") {
		t.Fatalf("补偿删除失败日志缺少定位上下文: %q", logBuf.String())
	}
	var ruleCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rules WHERE name = 'Step12自动规则'`).Scan(&ruleCount); err != nil {
		t.Fatalf("统计自动规则失败: %v", err)
	}
	if ruleCount != 1 {
		t.Fatalf("补偿删除失败时规则应保留供管理员处理，实际 %d", ruleCount)
	}
}

// TestAssemblyAutoRuleCompensationSuccessRemovesRule 版本创建失败但补偿删除成功时不得留下孤儿规则。
func TestAssemblyAutoRuleCompensationSuccessRemovesRule(t *testing.T) {
	engine, st, _, _ := newAssemblyTestEnvWithLogger(t, slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)))
	ctx := context.Background()
	if _, err := st.DB().ExecContext(ctx, `
		CREATE TRIGGER step12_fail_version2 BEFORE INSERT ON versions
		BEGIN
			SELECT RAISE(ABORT, 'step12 version fail');
		END;`); err != nil {
		t.Fatalf("创建版本失败触发器失败: %v", err)
	}
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", step12GenerateBody())
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("版本创建失败应返回 500: %d %s", w.Code, w.Body.String())
	}
	var ruleCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM rules WHERE name = 'Step12自动规则'`).Scan(&ruleCount); err != nil {
		t.Fatalf("统计自动规则失败: %v", err)
	}
	if ruleCount != 0 {
		t.Fatalf("补偿删除成功时不应残留自动规则，实际 %d", ruleCount)
	}
}
