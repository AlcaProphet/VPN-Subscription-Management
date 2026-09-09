package pool

import (
	"context"
	"database/sql"
	"testing"
)

func TestParseItemsCarryOriginEvidence(t *testing.T) {
	body := []byte("DOMAIN,a.com\nDOMAIN,a.com\nIP-CIDR,1.2.3.0/24,no-resolve\n")
	res, err := ParseSource(body, SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if res.Accepted != 2 || res.Duplicates != 1 || len(res.Items) != 3 {
		t.Fatalf("Items 应包含唯一与重复全部 origin: accepted=%d duplicates=%d items=%d", res.Accepted, res.Duplicates, len(res.Items))
	}
	if len(res.Rules) != 2 {
		t.Fatalf("Rules 投影应只含唯一 Canonical: %d", len(res.Rules))
	}
	wantOrders := []int{0, 1, 2}
	wantLines := []int{1, 2, 3}
	for i, item := range res.Items {
		if item.Origin.Order != wantOrders[i] || item.Origin.Line != wantLines[i] {
			t.Fatalf("第 %d 条 origin 元数据错误: %+v", i, item.Origin)
		}
	}
	if res.Items[0].Origin.Raw != "DOMAIN,a.com" || res.Items[1].Origin.Raw != "DOMAIN,a.com" || res.Items[2].Origin.Raw != "IP-CIDR,1.2.3.0/24,no-resolve" {
		t.Fatalf("raw_line 未保存原始行: %+v", res.Items)
	}
}

func TestApplyParseResultPersistsOriginEvidenceAndDuplicates(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "证据池", []SourceInput{{URL: "https://example.invalid/rules.txt", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	parsed, err := ParseSource([]byte("DOMAIN,a.com\nDOMAIN,a.com\nIP-CIDR,1.2.3.0/24\n"), SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	var snapshotID int64
	err = st.TxImmediate(ctx, func(tx *sql.Tx) error {
		var pending bool
		snapshotID, pending, err = applyParseResultTx(ctx, tx, p.ID, sourceID, parsed)
		_ = pending
		return err
	})
	if err != nil {
		t.Fatalf("apply 失败: %v", err)
	}
	if snapshotID <= 0 {
		t.Fatal("snapshotID 应存在")
	}
	var originCount int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_rule_origins WHERE snapshot_id = ?`, snapshotID).Scan(&originCount); err != nil {
		t.Fatalf("查询 origin 失败: %v", err)
	}
	if originCount != 3 {
		t.Fatalf("重复语义也应保留 origin，期望 3 实际 %d", originCount)
	}
	var canonicalCount int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT canonical_rule_id) FROM pool_rule_origins WHERE snapshot_id = ?`, snapshotID).Scan(&canonicalCount); err != nil {
		t.Fatalf("查询 canonical 失败: %v", err)
	}
	if canonicalCount != 2 {
		t.Fatalf("唯一 Canonical 数应为 2，实际 %d", canonicalCount)
	}
	rows, err := st.DB().QueryContext(ctx,
		`SELECT sort_order, line_no, raw_line FROM pool_rule_origins WHERE snapshot_id=? ORDER BY id`, snapshotID)
	if err != nil {
		t.Fatalf("查询证据失败: %v", err)
	}
	defer rows.Close()
	type evidence struct {
		order, line int
		raw         string
	}
	got := []evidence{}
	for rows.Next() {
		var e evidence
		if err := rows.Scan(&e.order, &e.line, &e.raw); err != nil {
			t.Fatalf("扫描证据失败: %v", err)
		}
		got = append(got, e)
	}
	if len(got) != 3 {
		t.Fatalf("证据行数应为 3: %d", len(got))
	}
	wantRaw := []string{"DOMAIN,a.com", "DOMAIN,a.com", "IP-CIDR,1.2.3.0/24"}
	for i, e := range got {
		if e.order != i || e.line != i+1 || e.raw != wantRaw[i] {
			t.Fatalf("第 %d 条证据错误: %+v", i, e)
		}
	}
}

func TestListEntriesDedupsBeforePaginationAndCount(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "去重池", []SourceInput{{URL: "https://example.invalid/rules.txt", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	parsed, err := ParseSource([]byte("DOMAIN,a.com\nDOMAIN,a.com\nIP-CIDR,1.2.3.0/24\n"), SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, _, err := applyParseResultTx(ctx, tx, p.ID, sourceID, parsed)
		return err
	}); err != nil {
		t.Fatalf("apply 失败: %v", err)
	}
	all, total, err := svc.ListEntries(ctx, p.ID, 1, 20, "")
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total != 2 || len(all) != 2 {
		t.Fatalf("去重后应为 2: total=%d len=%d", total, len(all))
	}
	_, urlTotal, err := svc.ListEntries(ctx, p.ID, 1, 20, "url")
	if err != nil || urlTotal != 2 {
		t.Fatalf("URL 来源筛选 total 应为 2: total=%d err=%v", urlTotal, err)
	}
	for page := int64(1); page <= 2; page++ {
		pageList, pageTotal, err := svc.ListEntries(ctx, p.ID, page, 1, "")
		if err != nil {
			t.Fatalf("第 %d 页失败: %v", page, err)
		}
		if len(pageList) != 1 || pageTotal != 2 {
			t.Fatalf("第 %d 页应恰好 1 条且 total=2: len=%d total=%d", page, len(pageList), pageTotal)
		}
	}
}

func TestListEntriesSortsManualThenURLSourceThenOriginOrder(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "排序池", []SourceInput{
		{URL: "https://one.example/rules.txt", SourceMode: SourceModeAuto},
		{URL: "https://two.example/rules.txt", SourceMode: SourceModeAuto},
	}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "manual.com"); err != nil {
		t.Fatalf("创建 manual 失败: %v", err)
	}
	body1 := []byte("DOMAIN,url1.com\nDOMAIN,shared.com\n")
	parsed1, err := ParseSource(body1, SourceModeAuto)
	if err != nil {
		t.Fatalf("解析 URL1 失败: %v", err)
	}
	body2 := []byte("DOMAIN,url2.com\nDOMAIN,shared.com\n")
	parsed2, err := ParseSource(body2, SourceModeAuto)
	if err != nil {
		t.Fatalf("解析 URL2 失败: %v", err)
	}
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, _, err := applyParseResultTx(ctx, tx, p.ID, p.Sources[0].ID, parsed1); err != nil {
			return err
		}
		if _, _, err := applyParseResultTx(ctx, tx, p.ID, p.Sources[1].ID, parsed2); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("apply 失败: %v", err)
	}
	list, total, err := svc.ListEntries(ctx, p.ID, 1, 20, "")
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total != 4 || len(list) != 4 {
		t.Fatalf("排序池条目数应为 4: total=%d len=%d", total, len(list))
	}
	want := []string{"manual.com", "url1.com", "shared.com", "url2.com"}
	for i, e := range list {
		if e.MatchValue != want[i] {
			t.Fatalf("第 %d 条顺序错误: got %s want %s", i, e.MatchValue, want[i])
		}
	}
}

func TestListEntriesFallsBackAfterDeletingEarliestOrigin(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "回退池", []SourceInput{
		{URL: "https://one.example/rules.txt", SourceMode: SourceModeAuto},
		{URL: "https://two.example/rules.txt", SourceMode: SourceModeAuto},
	}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	parsed1, err := ParseSource([]byte("DOMAIN,url1.com\nDOMAIN,shared.com\n"), SourceModeAuto)
	if err != nil {
		t.Fatalf("解析 URL1 失败: %v", err)
	}
	parsed2, err := ParseSource([]byte("DOMAIN,url2.com\nDOMAIN,shared.com\n"), SourceModeAuto)
	if err != nil {
		t.Fatalf("解析 URL2 失败: %v", err)
	}
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, _, err := applyParseResultTx(ctx, tx, p.ID, p.Sources[0].ID, parsed1); err != nil {
			return err
		}
		if _, _, err := applyParseResultTx(ctx, tx, p.ID, p.Sources[1].ID, parsed2); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("apply 失败: %v", err)
	}
	// 删除 shared.com 在 URL1（最早来源）中的 origin，应回退到 URL2 的 shared.com。
	var url1SharedOriginID int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT o.id FROM pool_rule_origins o
		 JOIN pool_canonical_rules cr ON cr.id = o.canonical_rule_id
		 WHERE o.source_id=? AND cr.family='domain' AND cr.value='shared.com'
		 ORDER BY o.id LIMIT 1`, p.Sources[0].ID).Scan(&url1SharedOriginID); err != nil {
		t.Fatalf("查询 URL1 shared origin 失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`DELETE FROM pool_rule_origins WHERE id=?`, url1SharedOriginID); err != nil {
		t.Fatalf("删除最早 origin 失败: %v", err)
	}
	list, total, err := svc.ListEntries(ctx, p.ID, 1, 20, "")
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Fatalf("删除最早 origin 后仍应有 3 个 Canonical: total=%d len=%d", total, len(list))
	}
	want := []string{"url1.com", "url2.com", "shared.com"}
	gotValues := []string{}
	for _, e := range list {
		gotValues = append(gotValues, e.MatchValue)
	}
	for i, w := range want {
		if gotValues[i] != w {
			t.Fatalf("第 %d 条顺序错误: got %s want %s", i, gotValues[i], w)
		}
	}
}
