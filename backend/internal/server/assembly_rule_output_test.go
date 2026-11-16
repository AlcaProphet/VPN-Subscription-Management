package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/log"
)

type ruleOutputPreview struct {
	Content string                      `json:"content"`
	Hash    string                      `json:"preview_hash"`
	Receipt *assembly.ConversionReceipt `json:"receipt"`
	Issues  []assembly.OutputIssue      `json:"issues"`
}

func assemblyVersionCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM versions").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAssemblyClashFinalRuleOutput(t *testing.T) {
	engine, st, _ := newAssemblyTestEnv(t)
	pid := insertAssemblyBase(t, st)
	for _, tc := range []struct {
		name, rules, merge, expectedBody, expectedIssue string
		base, blocked                                   bool
		output                                          int
	}{
		{name: "prepend only", rules: `prepend: ["DOMAIN,added.test,DIRECT"]`, expectedBody: "DOMAIN,added.test,DIRECT", output: 1},
		{name: "append after MATCH warns", rules: `append: ["DOMAIN,added.test,DIRECT"]`, expectedBody: "DOMAIN,added.test,DIRECT", expectedIssue: "可能被前面的 MATCH 覆盖", output: 1},
		{name: "delete last rule", base: true, rules: `delete: ["DOMAIN-SUFFIX,example.com,组A"]`, blocked: true},
		{name: "merge clears the list", base: true, merge: "rules: []", blocked: true},
		{name: "merge replaces seq and system fallback", rules: `prepend: ["DOMAIN,removed.test,DIRECT"]`, merge: `rules: ["GEOIP,cn,DIRECT","MATCH,DIRECT"]`, blocked: true},
		{name: "merge only", merge: `rules: ["DOMAIN,replaced.test,DIRECT","MATCH,DIRECT"]`, expectedBody: "DOMAIN,replaced.test,DIRECT", output: 1},
		{name: "valid rule provider", merge: "rule-providers:\n  local:\n    type: inline\n    behavior: classical\n    payload: [\"DOMAIN,example.test\"]\nrules: [\"RULE-SET,local,DIRECT\",\"MATCH,DIRECT\"]", expectedBody: "RULE-SET,local,DIRECT", output: 1},
		{name: "bad provider alongside valid rule", base: true, rules: `prepend: ["RULE-SET,missing,DIRECT"]`, expectedIssue: "规则集未声明", blocked: true, output: 1},
		{name: "unsupported overlay rule", rules: `prepend: ["USER-AGENT,Telegram,DIRECT"]`, expectedIssue: "不支持的 Clash 规则类型", blocked: true},
		{name: "invalid overlay rule target", rules: `prepend: ["DOMAIN,added.test,missing"]`, expectedIssue: "规则目标不存在", blocked: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := assemblyBody(pid)
			if !tc.base {
				in["custom_rules"] = []any{}
			}
			in["overlay"] = assembly.OverlayInput{RulesYAML: tc.rules, MergeYAML: tc.merge}
			w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", in)
			var preview struct {
				Data ruleOutputPreview
			}
			if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil || w.Code != http.StatusOK {
				t.Fatalf("预览失败: %d %s %v", w.Code, w.Body.String(), err)
			}
			result := preview.Data
			if result.Receipt == nil || result.Receipt.FinalOutput != tc.output ||
				(tc.base && result.Receipt.DirectOutput != 1) || (!tc.base && result.Receipt.Input != 0) {
				t.Fatalf("回执不符: %+v", result.Receipt)
			}
			if tc.expectedBody != "" && !strings.Contains(result.Content, tc.expectedBody) {
				t.Fatalf("最终正文缺少规则: %s", tc.expectedBody)
			}
			if tc.expectedIssue != "" {
				found := false
				for _, issue := range result.Issues {
					found = found || strings.Contains(issue.Message, tc.expectedIssue)
				}
				if !found {
					t.Fatalf("未返回自检诊断 %q: %+v", tc.expectedIssue, result.Issues)
				}
			}
			in["preview_hash"] = result.Hash
			before := assemblyVersionCount(t, st.DB())
			w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", in)
			after := assemblyVersionCount(t, st.DB())
			if tc.blocked {
				if w.Code != http.StatusBadRequest || after != before {
					t.Fatalf("应拒绝创建版本: %d %s，版本 %d → %d", w.Code, w.Body.String(), before, after)
				}
			} else if w.Code != http.StatusOK || after != before+1 {
				t.Fatalf("应成功创建版本: %d %s，版本 %d → %d", w.Code, w.Body.String(), before, after)
			}
		})
	}
}

func TestAssemblyReceiptMatchesTargetRenderer(t *testing.T) {
	engine, st, _ := newAssemblyTestEnv(t)
	clash := assemblyBody(insertAssemblyBase(t, st))
	clash["custom_rules"] = []assembly.RuleLine{{RuleType: "IP-SUFFIX", MatchValue: "192.0.2.1/32", Target: "组A"}}
	sr := map[string]any{
		"target_syntax": "sr-conf", "rule_name": "独立分流", "custom_rules": []assembly.RuleLine{
			{RuleType: "PROCESS-PATH", MatchValue: "/test/path", Target: "DIRECT"},
		},
	}
	for _, tc := range []struct {
		name, match     string
		in              map[string]any
		output, skipped int
		status          int
	}{
		{"Clash IP-SUFFIX", "IP-SUFFIX", clash, 1, 0, http.StatusOK},
		{"SR PROCESS-PATH", "PROCESS-PATH", sr, 0, 1, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", tc.in)
			var preview struct {
				Data ruleOutputPreview
			}
			if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil || w.Code != http.StatusOK {
				t.Fatalf("预览失败: %d %s %v", w.Code, w.Body.String(), err)
			}
			result := preview.Data
			if result.Receipt == nil || result.Receipt.FinalOutput != tc.output || result.Receipt.SkippedUnsupported != tc.skipped ||
				strings.Contains(result.Content, tc.match) != (tc.output > 0) {
				t.Fatalf("目标规则与回执不一致: %+v\n%s", result.Receipt, result.Content)
			}
			tc.in["preview_hash"] = result.Hash
			w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", tc.in)
			if w.Code != tc.status {
				t.Fatalf("生成状态不符: %d %s", w.Code, w.Body.String())
			}
		})
	}
	sr["custom_rules"] = []assembly.RuleLine{{RuleType: "DOMAIN", MatchValue: "valid.test", Target: "🚀直接连接"}}
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", sr)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "必须是 PROXY 或 DIRECT") {
		t.Fatalf("SR 应拒绝 Clash 策略组: %d %s", w.Code, w.Body.String())
	}
}

func TestAssemblyStalePreviewWhenLastRuleDisappears(t *testing.T) {
	engine, st, _, h := newAssemblyTestEnvWithLogger(t, log.New("error", "console"))
	ctx := context.Background()
	in := assemblyBody(insertAssemblyBase(t, st))
	p, err := h.poolSvc.Create(ctx, "R35-02", nil, false, "04:00")
	if err != nil {
		t.Fatal(err)
	}
	entry, err := h.poolSvc.CreateEntry(ctx, p.ID, "DOMAIN", "valid.test")
	if err != nil {
		t.Fatal(err)
	}
	in["custom_rules"] = []any{}
	in["pools"] = []assembly.PoolSelection{{PoolID: p.ID, Target: "组A"}}
	w := doJSON(t, engine, http.MethodPost, "/api/admin/assembly/preview", in)
	var preview struct {
		Data ruleOutputPreview
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil || w.Code != http.StatusOK || preview.Data.Hash == "" {
		t.Fatalf("预览失败: %d %s %v", w.Code, w.Body.String(), err)
	}
	if err := h.poolSvc.DeleteEntry(ctx, entry.ID); err != nil {
		t.Fatal(err)
	}
	in["preview_hash"] = preview.Data.Hash
	w = doJSON(t, engine, http.MethodPost, "/api/admin/assembly/generate", in)
	if w.Code != http.StatusConflict || assemblyVersionCount(t, st.DB()) != 0 {
		t.Fatalf("素材删除后应要求重新预览: %d %s", w.Code, w.Body.String())
	}
}
