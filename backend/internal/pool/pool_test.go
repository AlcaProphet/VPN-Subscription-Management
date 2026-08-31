package pool

import (
	"context"
	"errors"
	"testing"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newTestService(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return st, NewService(st, log.New("error", "console"))
}

func TestCRUDAndSort(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试池", []SourceInput{{URL: "https://a.example/rules.txt", SourceMode: SourceModeAuto}}, true, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if len(p.Sources) != 1 || p.Sources[0].SourceMode != SourceModeAuto {
		t.Fatalf("创建来源异常: %+v", p.Sources)
	}
	if _, err := svc.Create(ctx, "测试池", nil, false, "04:00"); !errors.Is(err, ErrNameConflict) {
		t.Errorf("重名应 409: %v", err)
	}
	if _, err := svc.Create(ctx, "坏池", []SourceInput{{URL: "ftp://x"}}, false, "04:00"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("非法 URL 应拒绝: %v", err)
	}

	m1, err := svc.CreateEntry(ctx, p.ID, "DOMAIN-SUFFIX", "manual.com")
	if err != nil {
		t.Fatalf("创建 manual1 失败: %v", err)
	}
	m2, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "manual2.com")
	if err != nil {
		t.Fatalf("创建 manual2 失败: %v", err)
	}
	if m1.SortOrder != 0 || m2.SortOrder != 1 {
		t.Errorf("manual 排序异常: %d %d", m1.SortOrder, m2.SortOrder)
	}
	if _, err := svc.CreateEntry(ctx, p.ID, "DOMAIN-SUFFIX", "manual.com"); !errors.Is(err, ErrEntryConflict) {
		t.Errorf("同类型同值应冲突: %v", err)
	}

	list, total, err := svc.ListEntries(ctx, p.ID, 1, 20, "")
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Fatalf("条目数应为 2: total=%d len=%d", total, len(list))
	}
	manual, manualTotal, err := svc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if err != nil || manualTotal != 2 || len(manual) != 2 {
		t.Fatalf("manual 来源筛选异常: total=%d len=%d err=%v", manualTotal, len(manual), err)
	}
	url, urlTotal, err := svc.ListEntries(ctx, p.ID, 1, 20, "url")
	if err != nil || urlTotal != 0 || len(url) != 0 {
		t.Fatalf("无 url 条目应返回空: %+v", url)
	}
	if _, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "invalid"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("非法来源应返回 ErrBadRequest，实际 %v", err)
	}

	if err := svc.DeleteEntry(ctx, m1.ID); err != nil {
		t.Fatalf("删除 manual 失败: %v", err)
	}
	list, total, _ = svc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if total != 1 || len(list) != 1 || list[0].MatchValue != "manual2.com" {
		t.Fatalf("删除后列表异常: %+v", list)
	}

	// Update 全量替换 URL 来源
	if err := svc.Update(ctx, p.ID, "测试池", []SourceInput{{URL: "https://b.example/rules.txt", SourceMode: SourceModeClash}}, false, "05:00"); err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	pools, err := svc.List(ctx)
	if err != nil || len(pools) != 1 {
		t.Fatalf("列表读取失败: %v", err)
	}
	if len(pools[0].Sources) != 2 {
		t.Fatalf("更新后应保留 manual + 1 个 url: %+v", pools[0].Sources)
	}
	if pools[0].URLs[0] != "https://b.example/rules.txt" {
		t.Fatalf("URL 未更新: %+v", pools[0].URLs)
	}

	// 迁移后新库中不存在旧表
	for _, table := range []string{"pool_entries"} {
		var n int
		if err := st.DB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&n); err != nil {
			t.Fatalf("查询表失败: %v", err)
		}
		if n != 0 {
			t.Fatalf("旧表 %s 应不存在", table)
		}
	}
}
