package pool

import (
	"context"
	"database/sql"
	"testing"
)

func TestActivatePendingAtomicallyUpdatesActivatedAtAndKeepsEvidence(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "激活语义池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	applyBody(t, st, svc, p.ID, sourceID, "DOMAIN,a.com\n")
	oldActiveID := activeSnapshotID(t, st, p.ID)
	if oldActiveID == 0 {
		t.Fatal("首次成功应产生 active")
	}
	var oldStats string
	if err := st.DB().QueryRowContext(ctx, `SELECT stats_json FROM pool_source_snapshots WHERE id=?`, oldActiveID).Scan(&oldStats); err != nil {
		t.Fatalf("读取旧 active stats 失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_source_snapshots
		   (source_id, status, format, profile, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json)
		 VALUES (?, 'failed', '', '', 0, 0, 0, 0, 0, 0, '[]', '{}')`, sourceID); err != nil {
		t.Fatalf("插入历史 failed 快照失败: %v", err)
	}

	// 第二次格式变化 -> pending；只改状态，不自动激活。
	applyBody(t, st, svc, p.ID, sourceID, "b.com\n")
	pendingID := pendingSnapshotID(t, st, p.ID)
	if pendingID == 0 {
		t.Fatal("格式变化应产生 pending")
	}
	var beforeStats string
	if err := st.DB().QueryRowContext(ctx, `SELECT stats_json FROM pool_source_snapshots WHERE id=?`, pendingID).Scan(&beforeStats); err != nil {
		t.Fatalf("读取 pending stats 失败: %v", err)
	}
	if err := svc.ActivatePending(ctx, p.ID, sourceID, pendingID); err != nil {
		t.Fatalf("激活 pending 失败: %v", err)
	}

	var activeID, pending sql.NullInt64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT active_snapshot_id, pending_snapshot_id FROM rule_pool_sources WHERE id=?`, sourceID).Scan(&activeID, &pending); err != nil {
		t.Fatalf("读取来源指针失败: %v", err)
	}
	if !activeID.Valid || activeID.Int64 != pendingID {
		t.Fatalf("active 指针未切到 pending: %+v", activeID)
	}
	if pending.Valid {
		t.Fatalf("pending 指针应清空: %+v", pending)
	}

	var afterStats, status string
	var activatedNotNull int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT stats_json, status, CASE WHEN activated_at IS NULL THEN 0 ELSE 1 END
		 FROM pool_source_snapshots WHERE id=?`, pendingID).Scan(&afterStats, &status, &activatedNotNull); err != nil {
		t.Fatalf("读取激活后快照失败: %v", err)
	}
	if status != "active" || activatedNotNull != 1 {
		t.Fatalf("激活后状态/时间错误: status=%s activated=%d", status, activatedNotNull)
	}
	if afterStats != beforeStats {
		t.Fatalf("激活不得改写 stats_json: before=%s after=%s", beforeStats, afterStats)
	}
	if oldStats == "" {
		t.Fatal("旧 active stats 不应为空")
	}

	var oldActivatedNotNull, failedActivatedNotNull int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT CASE WHEN activated_at IS NULL THEN 0 ELSE 1 END FROM pool_source_snapshots WHERE id=?`, oldActiveID).Scan(&oldActivatedNotNull); err != nil {
		t.Fatalf("读取历史 active 激活时间失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx,
		`SELECT CASE WHEN activated_at IS NULL THEN 0 ELSE 1 END FROM pool_source_snapshots WHERE source_id=? AND status='failed'`, sourceID).Scan(&failedActivatedNotNull); err != nil {
		t.Fatalf("读取历史 failed 激活时间失败: %v", err)
	}
	if oldActivatedNotNull != 0 || failedActivatedNotNull != 0 {
		t.Fatalf("历史 active/failed 不得补造 activated_at: old=%d failed=%d", oldActivatedNotNull, failedActivatedNotNull)
	}

	if err := svc.ActivatePending(ctx, p.ID, sourceID, oldActiveID); err == nil {
		t.Fatal("无 pending 时激活旧 active ID 应失败")
	}
	var activeAfterErr, pendingAfterErr sql.NullInt64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT active_snapshot_id, pending_snapshot_id FROM rule_pool_sources WHERE id=?`, sourceID).Scan(&activeAfterErr, &pendingAfterErr); err != nil {
		t.Fatalf("读取失败后指针失败: %v", err)
	}
	if !activeAfterErr.Valid || activeAfterErr.Int64 != pendingID || pendingAfterErr.Valid {
		t.Fatalf("失败激活不得改变指针: active=%+v pending=%+v", activeAfterErr, pendingAfterErr)
	}
}

func TestDiscardPendingDeletesSnapshotAndCleansOrphanCanonical(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "丢弃 pending 池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	applyBody(t, st, svc, p.ID, sourceID, "DOMAIN,a.com\n")
	oldActiveID := activeSnapshotID(t, st, p.ID)
	applyBody(t, st, svc, p.ID, sourceID, "b.com\n")
	pendingID := pendingSnapshotID(t, st, p.ID)
	if pendingID == 0 || oldActiveID == 0 {
		t.Fatalf("测试前提失败: active=%d pending=%d", oldActiveID, pendingID)
	}

	if err := svc.DiscardPending(ctx, p.ID, sourceID, pendingID); err != nil {
		t.Fatalf("丢弃 pending 失败: %v", err)
	}
	var activeID, pending sql.NullInt64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT active_snapshot_id, pending_snapshot_id FROM rule_pool_sources WHERE id=?`, sourceID).Scan(&activeID, &pending); err != nil {
		t.Fatalf("读取丢弃后指针失败: %v", err)
	}
	if !activeID.Valid || activeID.Int64 != oldActiveID {
		t.Fatalf("丢弃 pending 不得改变 active: %+v", activeID)
	}
	if pending.Valid {
		t.Fatalf("丢弃后 pending 指针应为空: %+v", pending)
	}
	var snapshotCount int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_source_snapshots WHERE id=?`, pendingID).Scan(&snapshotCount); err != nil {
		t.Fatalf("查询 pending 快照失败: %v", err)
	}
	if snapshotCount != 0 {
		t.Fatal("丢弃后 pending 快照应删除")
	}
	var newCanonical, oldCanonical int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_canonical_rules WHERE pool_id=? AND value='b.com'`, p.ID).Scan(&newCanonical); err != nil {
		t.Fatalf("查询新 Canonical 失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_canonical_rules WHERE pool_id=? AND value='a.com'`, p.ID).Scan(&oldCanonical); err != nil {
		t.Fatalf("查询旧 Canonical 失败: %v", err)
	}
	if newCanonical != 0 || oldCanonical != 1 {
		t.Fatalf("丢弃后 orphan Canonical 清理错误: b=%d a=%d", newCanonical, oldCanonical)
	}

	if err := svc.DiscardPending(ctx, p.ID, sourceID, oldActiveID); err == nil {
		t.Fatal("无 pending 时丢弃旧 active ID 应失败")
	}
}

func pendingSnapshotID(t *testing.T, st interface {
	DB() *sql.DB
}, poolID int64) int64 {
	t.Helper()
	var id int64
	if err := st.DB().QueryRow(
		`SELECT COALESCE(pending_snapshot_id,0) FROM rule_pool_sources WHERE pool_id=? AND kind='url'`, poolID).Scan(&id); err != nil {
		t.Fatalf("读取 pending snapshot 失败: %v", err)
	}
	return id
}
