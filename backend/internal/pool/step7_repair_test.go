package pool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"vpn-sub/internal/redact"
	"vpn-sub/internal/store"
)

func readLatestSnapshotStats(t *testing.T, st *store.Store, sourceID int64) SnapshotStats {
	t.Helper()
	var raw string
	if err := st.DB().QueryRow(
		`SELECT stats_json FROM pool_source_snapshots WHERE source_id=? ORDER BY id DESC LIMIT 1`, sourceID).Scan(&raw); err != nil {
		t.Fatalf("读取快照 stats_json 失败: %v", err)
	}
	var stats SnapshotStats
	if err := json.Unmarshal([]byte(raw), &stats); err != nil {
		t.Fatalf("解析快照 stats_json 失败: %v raw=%s", err, raw)
	}
	return stats
}

func TestFailedSnapshotStrictStatsShapeAndReasonCodes(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantReason string
	}{
		{name: "http", status: http.StatusInternalServerError, body: "boom", wantReason: "http_status_error"},
		{name: "conflicting", status: http.StatusOK, body: "payload:\n  - 'a.com'\n  - '1.2.3.0/24'\n", wantReason: "conflicting_format"},
		{name: "mixed", status: http.StatusOK, body: "DOMAIN-REGEX,^a$\nUSER-AGENT,curl\n", wantReason: "mixed_platform"},
		{name: "html", status: http.StatusOK, body: "<html><body>login</body></html>", wantReason: "html_source"},
		{name: "parse", status: http.StatusOK, body: `{"version":1,"rules":"bad"}`, wantReason: "parse_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, svc := newTestService(t)
			ctx := context.Background()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			p, err := svc.Create(ctx, "失败形状池-"+tc.name, []SourceInput{{URL: srv.URL, SourceMode: SourceModeAuto}}, false, "04:00")
			if err != nil {
				t.Fatalf("创建失败: %v", err)
			}
			if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
				t.Fatalf("提交同步失败: %v", err)
			}
			waitSync(t, svc, p.ID, "failed")
			stats := readLatestSnapshotStats(t, st, p.Sources[0].ID)
			if stats.SchemaVersion != 1 || stats.Detection == nil || stats.Comparison == nil || stats.Decision == nil {
				t.Fatalf("failed v1 stats 固定形状不完整: %+v", stats)
			}
			if len(stats.Detection.EvidenceCodes) != 0 || stats.Detection.RecognitionRequiredPercent != nil {
				t.Fatalf("failed detection 应为空 evidence + null 识别门槛: %+v", stats.Detection)
			}
			if stats.Comparison.PreviousActive != nil || stats.Comparison.AcceptedDropThresholdPercent != 70 || stats.Comparison.AcceptedDropTriggered {
				t.Fatalf("首次 failed comparison 形状错误: %+v", stats.Comparison)
			}
			if len(stats.RuleCounts) != 0 || stats.UnclassifiedRejected != 0 {
				t.Fatalf("failed rule_counts/unclassified_rejected 应为空: %+v", stats)
			}
			if len(stats.Decision.ReasonCodes) != 1 || stats.Decision.ReasonCodes[0] != tc.wantReason {
				t.Fatalf("reason_code=%v want %s", stats.Decision.ReasonCodes, tc.wantReason)
			}
		})
	}
}

func TestSuccessfulFirstSnapshotStrictStatsShape(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "首次成功形状池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	applyBody(t, st, svc, p.ID, p.Sources[0].ID, "DOMAIN,a.com\n")
	stats := readLatestSnapshotStats(t, st, p.Sources[0].ID)
	if stats.SchemaVersion != 1 || stats.Detection == nil || stats.Comparison == nil || stats.Decision == nil {
		t.Fatalf("首次成功 v1 stats 固定形状不完整: %+v", stats)
	}
	if len(stats.Detection.EvidenceCodes) != 1 || stats.Detection.EvidenceCodes[0] != "typed_rule_marker" {
		t.Fatalf("首次成功 evidence 应来自 detector 实际分支: %+v", stats.Detection)
	}
	if stats.Comparison.PreviousActive != nil || stats.Comparison.AcceptedDropThresholdPercent != 70 {
		t.Fatalf("首次成功 comparison 形状错误: %+v", stats.Comparison)
	}
	if len(stats.Decision.ReasonCodes) != 1 || stats.Decision.ReasonCodes[0] != "first_success" {
		t.Fatalf("首次成功 reason_code 错误: %+v", stats.Decision)
	}
}

func TestFailureSnapshotWriteSuffixReserved(t *testing.T) {
	base := strings.Repeat("错", redact.MaxFieldRunes*2)
	got := taskErrorWithFailureSnapshotSuffix(base)
	if utf8.RuneCountInString(got) > redact.MaxFieldRunes {
		t.Fatalf("错误文本超过 %d rune: %d", redact.MaxFieldRunes, utf8.RuneCountInString(got))
	}
	if !strings.HasSuffix(got, "；失败快照写入失败") {
		t.Fatalf("缺失失败快照写入提示: %q", got)
	}
}

func TestSourceSnapshotWireShapeExplicitNulls(t *testing.T) {
	snap := SourceSnapshot{
		ID: 1, SourceID: 1, Status: "active",
		Diagnostics: []ParseDiagnostic{},
		Stats: SnapshotStats{
			SchemaVersion: 1,
			Detection:     &DetectionStats{EvidenceCodes: []string{}},
			RuleCounts:    []RuleCountStat{},
			Comparison:    &ComparisonStats{AcceptedDropThresholdPercent: 70},
			Decision:      &DecisionStats{InitialStatus: "active", ReasonCodes: []string{"first_success"}},
		},
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("序列化快照失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析快照 JSON 失败: %v", err)
	}
	if v, ok := m["error"]; !ok || v != "" {
		t.Fatalf("error 应显式输出空串: %s", raw)
	}
	for _, key := range []string{"activated_at", "created_at"} {
		if v, ok := m[key]; !ok || v != nil {
			t.Fatalf("%s 应显式输出 null: %s", key, raw)
		}
	}

	status := SourceStatus{SourceID: 1, DisplayURL: "https://example.com/rules", SourceMode: SourceModeAuto}
	raw, err = json.Marshal(status)
	if err != nil {
		t.Fatalf("序列化来源状态失败: %v", err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("解析来源状态 JSON 失败: %v", err)
	}
	for _, key := range []string{"latest_attempt", "active", "pending", "latest_failed"} {
		if v, ok := m[key]; !ok || v != nil {
			t.Fatalf("%s 应显式输出 null: %s", key, raw)
		}
	}
}

func TestSanitizePerURLResultTruncatesAndRedacts(t *testing.T) {
	longURL := "https://example.com/?token=SECRET&" + strings.Repeat("a", 300)
	got := SanitizePerURLResult(PerURLResult{URL: longURL})
	if utf8.RuneCountInString(got.URL) > redact.MaxFieldRunes {
		t.Fatalf("URL 未按 %d rune 限长: %d", redact.MaxFieldRunes, utf8.RuneCountInString(got.URL))
	}
	if strings.Contains(got.URL, "SECRET") {
		t.Fatalf("URL 泄漏敏感参数值: %s", got.URL)
	}
	if !strings.Contains(got.URL, "token=***") {
		t.Fatalf("URL 应保留脱敏后的前置敏感参数: %s", got.URL)
	}
}

func TestSourceStatusDisplayURLTruncatesAndKeepsRawURL(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	longURL := "https://example.com/?token=SECRET&" + strings.Repeat("a", 300)
	p, err := svc.Create(ctx, "长URL状态池", []SourceInput{{URL: longURL, SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	statuses, err := svc.ListSourceStatuses(ctx, p.ID)
	if err != nil {
		t.Fatalf("查询来源状态失败: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("来源状态数量错误: %d", len(statuses))
	}
	if utf8.RuneCountInString(statuses[0].DisplayURL) > redact.MaxFieldRunes {
		t.Fatalf("display_url 未限长: %d", utf8.RuneCountInString(statuses[0].DisplayURL))
	}
	if strings.Contains(statuses[0].DisplayURL, "SECRET") {
		t.Fatalf("display_url 泄漏敏感值: %s", statuses[0].DisplayURL)
	}
	var raw string
	if err := st.DB().QueryRowContext(ctx, `SELECT COALESCE(url,'') FROM rule_pool_sources WHERE id=?`, p.Sources[0].ID).Scan(&raw); err != nil {
		t.Fatalf("读取原始 URL 失败: %v", err)
	}
	if raw != longURL {
		t.Fatalf("rule_pool_sources.url 不应被截断或改写")
	}
}
