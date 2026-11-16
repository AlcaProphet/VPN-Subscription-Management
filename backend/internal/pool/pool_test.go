package pool

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"vpn-sub/internal/log"
	"vpn-sub/internal/rulespec"
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

func TestLegacyTypeForCanonicalSelectsCIDRAddressFamily(t *testing.T) {
	ipv4 := rulespec.CanonicalRule{Family: rulespec.FamilyIP, Matcher: rulespec.MatcherCIDR, Value: "1.0.1.0/24"}
	if got := legacyTypeForCanonical(ipv4); got != "IP-CIDR" {
		t.Fatalf("IPv4 CIDR 类型异常: %s", got)
	}
	ipv6 := rulespec.CanonicalRule{Family: rulespec.FamilyIP, Matcher: rulespec.MatcherCIDR, Value: "2001:db8::/32"}
	if got := legacyTypeForCanonical(ipv6); got != "IP-CIDR6" {
		t.Fatalf("IPv6 CIDR 类型异常: %s", got)
	}
}

func TestPoolWithoutURLReturnsEmptyArray(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "手工素材池", nil, false, "04:00")
	if err != nil || p.URLs == nil || len(p.URLs) != 0 {
		t.Fatalf("创建纯手工池时应返回空 URL 数组: %+v %v", p, err)
	}
	pools, err := svc.List(ctx)
	if err != nil || len(pools) != 1 || pools[0].URLs == nil || len(pools[0].URLs) != 0 {
		t.Fatalf("读取纯手工池时应返回空 URL 数组: %+v %v", pools, err)
	}
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

	// Update 移除旧 URL 并追加新的 URL。
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

func TestUpdatePreservesUnchangedPoolSources(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "rules.txt", time.Time{}, strings.NewReader(r.URL.Path[1:]+".example\n"))
	}))
	defer source.Close()
	a := SourceInput{URL: source.URL + "/a", SourceMode: SourceModeAuto}
	b := SourceInput{URL: source.URL + "/b", SourceMode: SourceModeAuto}
	p, err := svc.Create(ctx, "已同步池", []SourceInput{a, b}, false, "05:00")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	waitSync(t, svc, p.ID, "succeeded")
	get := func() Pool {
		t.Helper()
		list, err := svc.List(ctx)
		if err != nil || len(list) != 1 {
			t.Fatalf("查询池失败: %v", err)
		}
		return list[0]
	}
	before := get()
	if before.EntryCount != 2 || before.Sources[0].ActiveSnapshotID == nil || before.Sources[1].ActiveSnapshotID == nil {
		t.Fatalf("预期两个活动来源: %+v", before)
	}
	if err := svc.SetAutoSync(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	toggled := get()
	if !toggled.AutoSync || toggled.Name != before.Name || toggled.SyncTime != before.SyncTime ||
		!reflect.DeepEqual(toggled.Sources, before.Sources) || toggled.EntryCount != 2 {
		t.Fatalf("切换开关改变了其他设置或活动规则: %+v", toggled)
	}
	if err := svc.Update(ctx, p.ID, "改名池", []SourceInput{b, a}, true, "06:00"); err != nil {
		t.Fatal(err)
	}
	edited := get()
	if edited.EntryCount != 2 || edited.Sources[0].ID != before.Sources[1].ID || edited.Sources[1].ID != before.Sources[0].ID ||
		!reflect.DeepEqual(edited.Sources[0].ActiveSnapshotID, before.Sources[1].ActiveSnapshotID) ||
		!reflect.DeepEqual(edited.Sources[1].ActiveSnapshotID, before.Sources[0].ActiveSnapshotID) {
		t.Fatalf("改名和排序应保留活动快照: %+v", edited.Sources)
	}
	entries, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "url")
	if err != nil || len(entries) != 2 || entries[0].MatchValue != "b.example" || entries[1].MatchValue != "a.example" {
		t.Fatalf("重排后应按新顺序读取活动规则: %+v, %v", entries, err)
	}
	a.SourceMode = SourceModeClash
	if err := svc.Update(ctx, p.ID, "改名池", []SourceInput{a}, true, "06:00"); err != nil {
		t.Fatal(err)
	}
	replaced := get()
	if len(replaced.Sources) != 1 || replaced.Sources[0].ID == before.Sources[0].ID ||
		replaced.Sources[0].ActiveSnapshotID != nil || replaced.EntryCount != 0 {
		t.Fatalf("移除来源及更改模式应清理旧规则: %+v", replaced)
	}
	var snapshots, rules int
	if err := st.DB().QueryRowContext(ctx, `SELECT (SELECT count(*) FROM pool_source_snapshots),
		(SELECT count(*) FROM pool_canonical_rules WHERE pool_id = ?)`, p.ID).Scan(&snapshots, &rules); err != nil {
		t.Fatal(err)
	}
	if snapshots != 0 || rules != 0 {
		t.Fatalf("旧快照或规则未清理: %d, %d", snapshots, rules)
	}
}

func TestCRUDRejectsNonMaterialPoolByOriginalLegacyType(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "白名单池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	for _, typ := range []string{"RULE-SET", "AND", "OR", "NOT", "MATCH", "GEOSITE", "SRC-GEOIP", "SRC-IP-ASN", "SRC-IP-CIDR", "IP-SUFFIX", "DST-PORT"} {
		value := "example"
		switch typ {
		case "MATCH":
			value = ""
		case "RULE-SET", "GEOSITE", "SRC-GEOIP":
			value = "cn"
		case "SRC-IP-ASN":
			value = "13335"
		case "SRC-IP-CIDR", "IP-SUFFIX":
			value = "10.0.0.0/8"
		case "AND":
			value = "((DOMAIN,a.com),(NETWORK,tcp))"
		case "OR":
			value = "((DOMAIN,a.com),(DOMAIN,b.com))"
		case "NOT":
			value = "((DOMAIN,a.com))"
		case "DST-PORT":
			value = "443"
		}
		if _, err := svc.CreateEntry(ctx, p.ID, typ, value); !errors.Is(err, ErrBadRequest) {
			t.Errorf("%s 应被白名单拒绝为 ErrBadRequest，实际 %v", typ, err)
		}
	}
	var canonicalCount, originCount int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pool_canonical_rules WHERE pool_id=?`, p.ID).Scan(&canonicalCount); err != nil {
		t.Fatalf("查询 canonical 失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM pool_rule_origins WHERE pool_id=?`, p.ID).Scan(&originCount); err != nil {
		t.Fatalf("查询 origin 失败: %v", err)
	}
	if canonicalCount != 0 || originCount != 0 {
		t.Fatalf("拒绝创建不得写入 canonical/origin: canonical=%d origin=%d", canonicalCount, originCount)
	}

	valid, err := svc.CreateEntry(ctx, p.ID, "DOMAIN", "ok.example")
	if err != nil {
		t.Fatalf("创建正例失败: %v", err)
	}
	if err := svc.UpdateEntry(ctx, valid.ID, "SRC-GEOIP", "CN"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("UpdateEntry 也应拒绝非素材池类型，实际 %v", err)
	}
	list, _, err := svc.ListEntries(ctx, p.ID, 1, 20, "manual")
	if err != nil || len(list) != 1 || list[0].MatchValue != "ok.example" {
		t.Fatalf("拒绝更新不得改变原记录: %+v err=%v", list, err)
	}
}
