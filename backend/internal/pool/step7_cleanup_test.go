package pool

import (
	"context"
	"testing"
)

func TestCleanupOldTasksPreservesPointersAndCleansOnlyUnreferencedFailed(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "清理保护池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	applyBody(t, st, svc, p.ID, sourceID, "DOMAIN,a.com\n")
	activeID := activeSnapshotID(t, st, p.ID)
	applyBody(t, st, svc, p.ID, sourceID, "b.com\n")
	pendingID := pendingSnapshotID(t, st, p.ID)
	if activeID == 0 || pendingID == 0 {
		t.Fatalf("测试前提失败: active=%d pending=%d", activeID, pendingID)
	}
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE pool_source_snapshots SET created_at=datetime('now','-8 days') WHERE id IN (?,?)`, activeID, pendingID); err != nil {
		t.Fatalf("老化 active/pending 失败: %v", err)
	}

	insertFailed := func(sourceID int64, ageModifier string) int64 {
		t.Helper()
		res, err := st.DB().ExecContext(ctx,
			`INSERT INTO pool_source_snapshots
			   (source_id, status, format, profile, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json, created_at)
			 VALUES (?, 'failed', '', '', 0, 0, 0, 0, 0, 0, '[]', '{}', datetime('now', ?))`, sourceID, ageModifier)
		if err != nil {
			t.Fatalf("插入 failed 快照失败: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	expiredFailedID := insertFailed(sourceID, "-8 days")
	recentFailedID := insertFailed(sourceID, "-1 days")

	// 过期 failed 若仍被来源指针引用，不得删除。
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO rule_pool_sources (pool_id, kind, url, source_mode, sort_order) VALUES (?, 'url', 'https://referenced.example/rules', 'auto', 1)`, p.ID)
	if err != nil {
		t.Fatalf("插入被引用来源失败: %v", err)
	}
	referencedSourceID, _ := res.LastInsertId()
	referencedFailedID := insertFailed(referencedSourceID, "-8 days")
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE rule_pool_sources SET active_snapshot_id=? WHERE id=?`, referencedFailedID, referencedSourceID); err != nil {
		t.Fatalf("设置被引用指针失败: %v", err)
	}

	// 过期未引用 failed + 仅由它承载的 Canonical/origin：应被清理。
	res, err = st.DB().ExecContext(ctx,
		`INSERT INTO pool_canonical_rules (pool_id, semantic_key, family, matcher, value) VALUES (?, 'domain|exact|c.com', 'domain', 'exact', 'c.com')`, p.ID)
	if err != nil {
		t.Fatalf("插入待清理 Canonical 失败: %v", err)
	}
	expiredCanonicalID, _ := res.LastInsertId()
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_rule_origins (pool_id, canonical_rule_id, source_id, snapshot_id, sort_order, raw_line, line_no)
		 VALUES (?, ?, ?, ?, 0, 'c.com', 0)`, p.ID, expiredCanonicalID, sourceID, expiredFailedID); err != nil {
		t.Fatalf("插入待清理 origin 失败: %v", err)
	}

	if _, err := svc.CleanupOldTasks(ctx); err != nil {
		t.Fatalf("清理失败: %v", err)
	}

	for name, snapshotID := range map[string]int64{
		"active": activeID, "pending": pendingID, "recent-failed": recentFailedID, "referenced-failed": referencedFailedID,
	} {
		var n int
		if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pool_source_snapshots WHERE id=?`, snapshotID).Scan(&n); err != nil {
			t.Fatalf("查询快照 %s 失败: %v", name, err)
		}
		if n != 1 {
			t.Fatalf("清理误删 %s 快照 %d", name, snapshotID)
		}
	}
	var n int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pool_source_snapshots WHERE id=?`, expiredFailedID).Scan(&n); err != nil {
		t.Fatalf("查询过期 failed 失败: %v", err)
	}
	if n != 0 {
		t.Fatal("过期未引用 failed 快照应删除")
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pool_canonical_rules WHERE id=?`, expiredCanonicalID).Scan(&n); err != nil {
		t.Fatalf("查询 orphan Canonical 失败: %v", err)
	}
	if n != 0 {
		t.Fatal("过期 failed 清理后应回收 orphan Canonical")
	}
	var originCount int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pool_rule_origins WHERE snapshot_id=?`, expiredFailedID).Scan(&originCount); err != nil {
		t.Fatalf("查询 origin 失败: %v", err)
	}
	if originCount != 0 {
		t.Fatal("过期 failed 清理应级联删除 origin")
	}

	var gotActive, gotPending int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COALESCE(active_snapshot_id,0), COALESCE(pending_snapshot_id,0) FROM rule_pool_sources WHERE id=?`, sourceID).
		Scan(&gotActive, &gotPending); err != nil {
		t.Fatalf("读取主来源指针失败: %v", err)
	}
	if gotActive != activeID || gotPending != pendingID {
		t.Fatalf("清理不得改变 active/pending 指针: active=%d/%d pending=%d/%d", gotActive, activeID, gotPending, pendingID)
	}
}
