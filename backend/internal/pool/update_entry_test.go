package pool

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"vpn-sub/internal/store"
)

func applyBody(t *testing.T, st *store.Store, svc *Service, poolID, sourceID int64, body string) {
	t.Helper()
	ctx := context.Background()
	parsed, err := ParseSource([]byte(body), SourceModeAuto)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, _, err := applyParseResultTx(ctx, tx, poolID, sourceID, parsed)
		return err
	}); err != nil {
		t.Fatalf("apply 失败: %v", err)
	}
}

func TestUpdateEntryRebindsWithoutPollutingSharedCanonical(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "共享池", []SourceInput{{URL: "https://one.example/rules.txt", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	applyBody(t, st, svc, p.ID, p.Sources[0].ID, "DOMAIN,shared.com\n")
	m, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "shared.com")
	if err != nil {
		t.Fatalf("创建 manual 失败: %v", err)
	}
	if err := svc.UpdateEntry(ctx, m.ID, "DOMAIN", "changed.com"); err != nil {
		t.Fatalf("换绑失败: %v", err)
	}
	manual, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if err != nil || len(manual) != 1 || manual[0].MatchValue != "changed.com" {
		t.Fatalf("manual 应已换绑为 changed.com: %+v err=%v", manual, err)
	}
	url, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "url")
	if err != nil || len(url) != 1 || url[0].MatchValue != "shared.com" {
		t.Fatalf("URL 派生规则应保持 shared.com: %+v err=%v", url, err)
	}
	var sharedCanonicalCount int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_canonical_rules WHERE pool_id=? AND family='domain' AND value='shared.com'`, p.ID).Scan(&sharedCanonicalCount); err != nil {
		t.Fatalf("查询 shared canonical 失败: %v", err)
	}
	if sharedCanonicalCount != 1 {
		t.Fatalf("URL 仍在引用 shared canonical，不应被清理: %d", sharedCanonicalCount)
	}
}

func TestUpdateEntryAllowsShareWhenTargetHasOnlyURLOrigin(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "仅URL池", []SourceInput{{URL: "https://one.example/rules.txt", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	applyBody(t, st, svc, p.ID, p.Sources[0].ID, "DOMAIN,target.com\n")
	m, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "manual.com")
	if err != nil {
		t.Fatalf("创建 manual 失败: %v", err)
	}
	if err := svc.UpdateEntry(ctx, m.ID, "DOMAIN", "target.com"); err != nil {
		t.Fatalf("目标仅有 URL origin 时允许共享: %v", err)
	}
	manual, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if err != nil || len(manual) != 1 || manual[0].MatchValue != "target.com" {
		t.Fatalf("manual 应指向 target.com: %+v err=%v", manual, err)
	}
	url, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "url")
	if err != nil || len(url) != 1 || url[0].MatchValue != "target.com" {
		t.Fatalf("URL 不应被手工编辑污染: %+v err=%v", url, err)
	}
}

func TestUpdateEntryManualConflictReturnsErrAndPreservesBoth(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "冲突池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	m1, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "a.com")
	if err != nil {
		t.Fatalf("创建 m1 失败: %v", err)
	}
	m2, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "b.com")
	if err != nil {
		t.Fatalf("创建 m2 失败: %v", err)
	}
	if err := svc.UpdateEntry(ctx, m1.ID, "DOMAIN", "b.com"); !errors.Is(err, ErrEntryConflict) {
		t.Fatalf("manual→manual 冲突应返回 ErrEntryConflict: %v", err)
	}
	list, total, err := svc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if err != nil || total != 2 || len(list) != 2 {
		t.Fatalf("冲突后两条 manual 应保留: total=%d len=%d err=%v", total, len(list), err)
	}
	if list[0].MatchValue != "a.com" || list[1].MatchValue != "b.com" {
		t.Fatalf("冲突后内容异常: %+v", list)
	}
	_ = m2
}

func TestUpdateEntryPreservesSortAndCleansOrphan(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "排序清理池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	m1, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "a.com")
	if err != nil {
		t.Fatalf("创建 m1 失败: %v", err)
	}
	if _, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "b.com"); err != nil {
		t.Fatalf("创建 m2 失败: %v", err)
	}
	if err := svc.UpdateEntry(ctx, m1.ID, "DOMAIN", "c.com"); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	list, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if err != nil || len(list) != 2 {
		t.Fatalf("列表异常: %+v err=%v", list, err)
	}
	if list[0].MatchValue != "c.com" || list[1].MatchValue != "b.com" {
		t.Fatalf("编辑后 manual 顺序应保持 c,b: %+v", list)
	}
	var orphan int
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_canonical_rules WHERE pool_id=? AND family='domain' AND value='a.com'`, p.ID).Scan(&orphan); err != nil {
		t.Fatalf("查询孤儿失败: %v", err)
	}
	if orphan != 0 {
		t.Fatalf("旧 canonical 无有效 origin 应被清理: %d", orphan)
	}
}
