package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"vpn-sub/internal/rulespec"
	"vpn-sub/internal/store"
)

func TestRuleCountsInvariantsAndStableSort(t *testing.T) {
	uniqueDomains := func(n int) string {
		var b strings.Builder
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "DOMAIN,a%02d.com\n", i)
		}
		return b.String()
	}
	cases := []struct {
		name        string
		body        string
		mode        SourceMode
		wantAccept  int
		wantExclude int
		wantReject  int
		wantDup     int
		wantUnclass int
	}{
		{
			name:        "duplicates-and-mode-excluded",
			body:        "DOMAIN,a.com\nDOMAIN,a.com\nDOMAIN-REGEX,^example\\.com$\n",
			mode:        SourceModeShadowrocket,
			wantAccept:  1,
			wantExclude: 1,
			wantDup:     1,
		},
		{
			name:        "adapter-reject",
			body:        uniqueDomains(10) + "BOGUS,xxx\n",
			mode:        SourceModeAuto,
			wantAccept:  10,
			wantReject:  1,
			wantUnclass: 1,
		},
		{
			name:        "src-reject-via-diagnostic",
			body:        uniqueDomains(10) + "SRC-IP-CIDR,10.0.0.0/8\n",
			mode:        SourceModeAuto,
			wantAccept:  10,
			wantReject:  1,
			wantUnclass: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := ParseSource([]byte(tc.body), tc.mode)
			if err != nil {
				t.Fatalf("解析失败: %v", err)
			}
			if res.Accepted != tc.wantAccept || res.Excluded != tc.wantExclude ||
				res.Rejected != tc.wantReject || res.Duplicates != tc.wantDup ||
				res.UnclassifiedRejected != tc.wantUnclass {
				t.Fatalf("顶层计数错误: %+v", res)
			}
			var sumAccepted, sumExcluded, sumRejected, sumDuplicates int
			for i, rc := range res.RuleCounts {
				sumAccepted += rc.Accepted
				sumExcluded += rc.Excluded
				sumRejected += rc.Rejected
				sumDuplicates += rc.Duplicates
				if i > 0 {
					prev := res.RuleCounts[i-1]
					prevKey := prev.Family + "\x00" + prev.Matcher + "\x00" + prev.Scope
					curKey := rc.Family + "\x00" + rc.Matcher + "\x00" + rc.Scope
					if curKey < prevKey {
						t.Fatalf("rule_counts 未稳定排序: %+v", res.RuleCounts)
					}
				}
			}
			if sumAccepted != res.Accepted || sumExcluded != res.Excluded ||
				sumDuplicates != res.Duplicates || sumRejected+res.UnclassifiedRejected != res.Rejected {
				t.Fatalf("分项合计不变量失败: top accepted=%d excluded=%d rejected=%d dup=%d unclassified=%d; sums=%d/%d/%d/%d rule_counts=%+v",
					res.Accepted, res.Excluded, res.Rejected, res.Duplicates, res.UnclassifiedRejected,
					sumAccepted, sumExcluded, sumRejected, sumDuplicates, res.RuleCounts)
			}
		})
	}
}

func TestRuleCountsMaterialPoolCapabilityRejectInvariant(t *testing.T) {
	items := make([]ParsedRule, 0, 11)
	for i := 0; i < 10; i++ {
		items = append(items, ParsedRule{
			Rule: rulespec.CanonicalRule{
				Family:  rulespec.FamilyDomain,
				Matcher: rulespec.MatcherExact,
				Value:   fmt.Sprintf("a%02d.com", i),
			},
			Origin: RuleOriginMeta{Order: i},
		})
	}
	items = append(items, ParsedRule{
		Rule:   rulespec.CanonicalRule{Family: rulespec.FamilyProcess, Matcher: rulespec.MatcherEquals, Value: "cn"},
		Origin: RuleOriginMeta{Order: 10, Raw: "RULE-SET,cn"},
	})
	res, err := finalizeParseResult(FormatTypedRuleText, items, nil, SourceModeAuto, nil)
	if err != nil {
		t.Fatalf("finalizeParseResult 失败: %v", err)
	}
	if res.Accepted != 10 || res.Rejected != 1 || res.UnclassifiedRejected != 0 {
		t.Fatalf("素材池白名单拒绝统计错误: %+v", res)
	}
	var sumAccepted, sumRejected int
	for _, rc := range res.RuleCounts {
		sumAccepted += rc.Accepted
		sumRejected += rc.Rejected
	}
	if sumAccepted != res.Accepted || sumRejected+res.UnclassifiedRejected != res.Rejected {
		t.Fatalf("素材池能力拒绝分项不变量失败: top=%+v sums=%d/%d", res, sumAccepted, sumRejected)
	}
	if len(res.Diagnostics) != 1 || res.Diagnostics[0].Kind != "reject" {
		t.Fatalf("素材池能力拒绝应保留诊断: %+v", res.Diagnostics)
	}
}

func TestApplyParseResultPreviousActiveComparison(t *testing.T) {
	t.Run("format-change", func(t *testing.T) {
		st, svc := newTestService(t)
		ctx := context.Background()
		p, err := svc.Create(ctx, "对比格式池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
		if err != nil {
			t.Fatalf("创建素材池失败: %v", err)
		}
		sourceID := p.Sources[0].ID
		applyBody(t, st, svc, p.ID, sourceID, "DOMAIN,a.com\n")
		oldID := activeSnapshotID(t, st, p.ID)
		applyBody(t, st, svc, p.ID, sourceID, "b.com\n")
		stats := readLatestSnapshotStats(t, st, sourceID)
		if stats.Comparison == nil || stats.Decision == nil || stats.Decision.InitialStatus != "pending" {
			t.Fatalf("格式变化应形成 pending 对比: %+v", stats)
		}
		pa := stats.Comparison.PreviousActive
		if pa == nil || pa.SnapshotID != oldID || pa.Format != "typed-rule-text" || pa.Profile != "common" || pa.Accepted != 1 {
			t.Fatalf("previous_active 摘要错误: %+v", pa)
		}
		if !stats.Comparison.FormatChanged || stats.Comparison.ProfileChanged || stats.Comparison.AcceptedDropThresholdPercent != 70 {
			t.Fatalf("格式变化标记错误: %+v", stats.Comparison)
		}
		if len(stats.Decision.ReasonCodes) != 1 || stats.Decision.ReasonCodes[0] != "format_changed" {
			t.Fatalf("格式变化 reason_codes 错误: %+v", stats.Decision)
		}
		assertNoTopLevelSnapshotCounts(t, st, sourceID)
	})

	t.Run("accepted-drop", func(t *testing.T) {
		st, svc := newTestService(t)
		ctx := context.Background()
		p, err := svc.Create(ctx, "对比缩量池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
		if err != nil {
			t.Fatalf("创建素材池失败: %v", err)
		}
		sourceID := p.Sources[0].ID
		var first, second strings.Builder
		for i := 0; i < 20; i++ {
			first.WriteString(fmt.Sprintf("DOMAIN,a%02d.com\n", i))
			if i < 10 {
				second.WriteString(fmt.Sprintf("DOMAIN,a%02d.com\n", i))
			}
		}
		applyBody(t, st, svc, p.ID, sourceID, first.String())
		oldID := activeSnapshotID(t, st, p.ID)
		applyBody(t, st, svc, p.ID, sourceID, second.String())
		stats := readLatestSnapshotStats(t, st, sourceID)
		if stats.Comparison == nil || stats.Decision == nil || stats.Decision.InitialStatus != "pending" {
			t.Fatalf("accepted 缩量应形成 pending: %+v", stats)
		}
		pa := stats.Comparison.PreviousActive
		if pa == nil || pa.SnapshotID != oldID || pa.Accepted != 20 {
			t.Fatalf("previous_active accepted 错误: %+v", pa)
		}
		if stats.Comparison.FormatChanged || stats.Comparison.ProfileChanged || !stats.Comparison.AcceptedDropTriggered {
			t.Fatalf("accepted 缩量比较标记错误: %+v", stats.Comparison)
		}
		found := false
		for _, code := range stats.Decision.ReasonCodes {
			if code == "accepted_below_threshold" {
				found = true
			}
		}
		if !found {
			t.Fatalf("缺少 accepted_below_threshold reason: %+v", stats.Decision)
		}
	})
}

func TestSnapshotStatsVersionZeroCompatibility(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "version0 池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID

	insertSnapshot := func(diag, stats string) int64 {
		t.Helper()
		res, err := st.DB().ExecContext(ctx,
			`INSERT INTO pool_source_snapshots
			   (source_id, status, format, profile, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json)
			 VALUES (?, 'active', '', '', 0, 0, 0, 0, 0, 0, ?, ?)`, sourceID, diag, stats)
		if err != nil {
			t.Fatalf("插入快照失败: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	emptyID := insertSnapshot("null", "{}")
	oldCountsID := insertSnapshot("[]", `{"input":3,"recognized":3,"accepted":2,"excluded":0,"rejected":1,"duplicates":0}`)
	malformedID := insertSnapshot("[]", "not-json")
	threshold := 90
	valid := SnapshotStats{
		SchemaVersion: 1,
		SourceMode:    string(SourceModeAuto),
		Detection: &DetectionStats{
			EvidenceCodes:              []string{"typed_rule_marker"},
			RecognitionRequiredPercent: &threshold,
		},
		RuleCounts: []RuleCountStat{{
			Family: "domain", Matcher: "exact", Scope: "common", Accepted: 1,
		}},
		UnclassifiedRejected: 0,
		Comparison:           &ComparisonStats{AcceptedDropThresholdPercent: 70},
		Decision:             &DecisionStats{InitialStatus: "active", ReasonCodes: []string{"first_success"}},
	}
	raw, _ := json.Marshal(valid)
	validID := insertSnapshot("[]", string(raw))

	list, total, err := svc.ListSourceSnapshots(ctx, p.ID, sourceID, 1, 20)
	if err != nil || total != 4 {
		t.Fatalf("读取快照历史失败: total=%d err=%v", total, err)
	}
	byID := map[int64]SourceSnapshot{}
	for _, s := range list {
		byID[s.ID] = s
	}
	for _, id := range []int64{emptyID, oldCountsID, malformedID} {
		s := byID[id]
		if s.Stats.SchemaVersion != 0 || s.Stats.SourceMode != "" || s.Stats.Detection != nil ||
			s.Stats.Comparison != nil || s.Stats.Decision != nil || s.Stats.UnclassifiedRejected != 0 {
			t.Fatalf("旧 stats 应规范化为 version 0: id=%d stats=%+v", id, s.Stats)
		}
		if s.Stats.RuleCounts == nil || len(s.Stats.RuleCounts) != 0 {
			t.Fatalf("旧 stats rule_counts 应为非 nil 空切片: id=%d %#v", id, s.Stats.RuleCounts)
		}
		if s.Diagnostics == nil || len(s.Diagnostics) != 0 {
			t.Fatalf("空诊断应为非 nil 空切片: id=%d %#v", id, s.Diagnostics)
		}
	}
	if got := byID[validID].Stats; !reflect.DeepEqual(got, valid) {
		t.Fatalf("合法 v1 stats 不应被旧数据影响: got=%+v want=%+v", got, valid)
	}
}

func assertNoTopLevelSnapshotCounts(t *testing.T, st *store.Store, sourceID int64) {
	t.Helper()
	var raw string
	if err := st.DB().QueryRow(
		`SELECT stats_json FROM pool_source_snapshots WHERE source_id=? ORDER BY id DESC LIMIT 1`, sourceID).Scan(&raw); err != nil {
		t.Fatalf("读取 stats_json 失败: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("解析 stats_json 失败: %v raw=%s", err, raw)
	}
	for _, key := range []string{"input", "recognized", "accepted", "excluded", "rejected", "duplicates", "confidence_score"} {
		if _, ok := m[key]; ok {
			t.Fatalf("stats_json 不应含顶层计数/confidence 字段 %q: %s", key, raw)
		}
	}
	if _, ok := m["rule_counts"]; !ok {
		t.Fatalf("stats_json 应固定包含 rule_counts: %s", raw)
	}
}
