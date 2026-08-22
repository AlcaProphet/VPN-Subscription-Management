package pool

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

func testMigrateFS() fstest.MapFS {
	return fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"1009_xray.sql": &fstest.MapFile{Data: []byte(`
			CREATE TABLE IF NOT EXISTS rule_pools (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL UNIQUE,
				urls_json TEXT NOT NULL DEFAULT '[]',
				last_synced_at TIMESTAMP,
				sync_status TEXT NOT NULL DEFAULT '',
				sync_error TEXT NOT NULL DEFAULT '',
				auto_sync INTEGER NOT NULL DEFAULT 0,
				sync_time TEXT NOT NULL DEFAULT '04:00',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS pool_entries (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				pool_id INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
				rule_type TEXT NOT NULL,
				match_value TEXT NOT NULL,
				source TEXT NOT NULL CHECK (source IN ('url','manual')),
				sort_order INTEGER NOT NULL,
				source_url TEXT NOT NULL DEFAULT '',
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (pool_id, rule_type, match_value));
			CREATE TABLE IF NOT EXISTS pool_sync_tasks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				pool_id INTEGER NOT NULL REFERENCES rule_pools(id) ON DELETE CASCADE,
				status TEXT NOT NULL CHECK (status IN ('running','succeeded','failed','partial')),
				per_url_json TEXT NOT NULL DEFAULT '[]',
				error TEXT NOT NULL DEFAULT '',
				started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				finished_at TIMESTAMP,
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
}

func newTestService(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), testMigrateFS()); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return st, NewService(st, log.New("error", "console"))
}

func waitTask(t *testing.T, svc *Service, poolID int64, terminal string) *SyncTask {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		task, err := svc.GetStatus(ctx, poolID)
		if err != nil && !errors.Is(err, ErrNotFound) {
			t.Fatalf("查询状态失败: %v", err)
		}
		if task != nil && task.Status == terminal {
			return task
		}
		if task != nil && (task.Status == "failed" || task.Status == "partial" || task.Status == "succeeded") && terminal == "done" {
			return task
		}
		time.Sleep(20 * time.Millisecond)
	}
	task, _ := svc.GetStatus(ctx, poolID)
	t.Fatalf("等待任务终态超时: %+v", task)
	return nil
}

// TestParser 解析规则：full/裸域名/标准行/IP-CIDR/注释与多余段
func TestParser(t *testing.T) {
	cases := []struct {
		line     string
		typ, val string
		ok       bool
	}{
		{"full:Apple.com", "DOMAIN", "Apple.com", true},
		{"www.apple.com", "DOMAIN-SUFFIX", "www.apple.com", true},
		{"IP-CIDR,1.2.3.0/24,no-resolve", "IP-CIDR", "1.2.3.0/24", true},
		{"# 注释", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		typ, val, _, ok := ParseLine(c.line)
		if ok != c.ok || (ok && (typ != c.typ || val != c.val)) {
			t.Errorf("ParseLine(%q) = (%q,%q,%v), want (%q,%q,%v)", c.line, typ, val, ok, c.typ, c.val, c.ok)
		}
	}
	// 白名单校验：类型统一大写、域名小写 / CIDR 归一 / 非法拒绝
	if typ, v, err := ValidateEntry("domain-suffix", "APPLE.COM"); err != nil || typ != "DOMAIN-SUFFIX" || v != "apple.com" {
		t.Errorf("类型/域名应规范化: %q %q %v", typ, v, err)
	}
	if typ, v, err := ValidateEntry("ip-cidr", "1.2.3.0/24"); err != nil || typ != "IP-CIDR" || v != "1.2.3.0/24" {
		t.Errorf("类型/CIDR 应规范化: %q %q %v", typ, v, err)
	}
	if _, _, err := ValidateEntry("DOMAIN", "bad,value"); err == nil {
		t.Error("含逗号应拒绝")
	}
	if _, _, err := ValidateEntry("UNKNOWN", "x"); err == nil {
		t.Error("未知类型应拒绝")
	}
}

// TestCRUDAndSort 池 CRUD + manual/URL 两段排序（manual 段恒在 url 段之前）
func TestCRUDAndSort(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "测试池", []string{"https://a.example/rules.txt"}, true, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Create(ctx, "测试池", nil, false, "04:00"); !errors.Is(err, ErrNameConflict) {
		t.Errorf("重名应 409: %v", err)
	}
	if _, err := svc.Create(ctx, "坏池", []string{"ftp://x"}, false, "04:00"); !errors.Is(err, ErrBadRequest) {
		t.Errorf("非法 URL 应拒绝: %v", err)
	}
	// manual 两条
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
	// 直接插入一条 url 条目（模拟同步已发生），sort_order 从 URLBase 起
	if _, err := st.DB().Exec(`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order)
		VALUES (?, 'DOMAIN-SUFFIX', 'url.com', 'url', ?)`, p.ID, URLBase); err != nil {
		t.Fatalf("插入 url 条目失败: %v", err)
	}
	list, total, err := svc.ListEntries(ctx, p.ID, 1, 20)
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	if total != 3 {
		t.Fatalf("条目数应为 3: %d", total)
	}
	if list[0].Source != "manual" || list[1].Source != "manual" || list[2].Source != "url" {
		t.Errorf("渲染顺序应 manual 段在前、url 段在后: %+v", list)
	}
}

// TestSyncAllSuccess 全成功：url 条目 upsert + 差量删除（临时表 JOIN）
func TestSyncAllSuccess(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("full:a.com\nb.com\n# comment\nIP-CIDR,10.0.0.0/8\n"))
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "同步池", []string{srv.URL}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// 既有 url 条目 c.com（本次响应中消失）→ 全成功后应被差量删除
	if _, err := st.DB().Exec(`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order)
		VALUES (?, 'DOMAIN-SUFFIX', 'c.com', 'url', ?)`, p.ID, URLBase); err != nil {
		t.Fatalf("插入旧条目失败: %v", err)
	}
	taskID, err := svc.SubmitSync(ctx, p.ID)
	if err != nil || taskID <= 0 {
		t.Fatalf("提交同步失败: %v", err)
	}
	task := waitTask(t, svc, p.ID, "succeeded")
	if len(task.PerURL) != 1 || !task.PerURL[0].OK {
		t.Fatalf("单 URL 应成功: %+v", task.PerURL)
	}
	var urlCount, cCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_entries WHERE pool_id=? AND source='url'`, p.ID).Scan(&urlCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if urlCount != 3 {
		t.Errorf("url 段应为 3 条: %d", urlCount)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_entries WHERE pool_id=? AND match_value='c.com'`, p.ID).Scan(&cCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if cCount != 0 {
		t.Error("消失的 url 条目应被差量删除")
	}
}

// TestSyncPartialNoDelete 单 URL 失败：成功项仍 upsert，但不执行差量删除
func TestSyncPartialNoDelete(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("new.com\n"))
	}))
	defer good.Close()
	p, err := svc.Create(ctx, "部分失败池", []string{good.URL, bad.URL}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order)
		VALUES (?, 'DOMAIN-SUFFIX', 'old.com', 'url', ?)`, p.ID, URLBase); err != nil {
		t.Fatalf("插入旧条目失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	task := waitTask(t, svc, p.ID, "partial")
	if task.Status != "partial" {
		t.Fatalf("状态应为 partial: %s", task.Status)
	}
	var oldCount, newCount int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_entries WHERE pool_id=? AND match_value='old.com'`, p.ID).Scan(&oldCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_entries WHERE pool_id=? AND match_value='new.com'`, p.ID).Scan(&newCount); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if oldCount != 1 {
		t.Error("部分失败不应删除旧条目")
	}
	if newCount != 1 {
		t.Error("成功 URL 的新条目应照常 upsert")
	}
}

// TestSyncEmptyResponseAndZeroEntries 空响应与零有效条目均视为失败并保留旧数据
func TestSyncEmptyResponseAndZeroEntries(t *testing.T) {
	for name, body := range map[string]string{"空响应": "", "零条目": "# only comment\n"} {
		t.Run(name, func(t *testing.T) {
			st, svc := newTestService(t)
			ctx := context.Background()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			p, err := svc.Create(ctx, name, []string{srv.URL}, false, "04:00")
			if err != nil {
				t.Fatalf("创建失败: %v", err)
			}
			if _, err := st.DB().Exec(`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order)
				VALUES (?, 'DOMAIN-SUFFIX', 'keep.com', 'url', ?)`, p.ID, URLBase); err != nil {
				t.Fatalf("插入旧条目失败: %v", err)
			}
			if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
				t.Fatalf("提交失败: %v", err)
			}
			task := waitTask(t, svc, p.ID, "partial")
			if !strings.Contains(task.PerURL[0].Error, "保留旧数据") {
				t.Errorf("失败原因异常: %s", task.PerURL[0].Error)
			}
			var n int
			_ = st.DB().QueryRow(`SELECT COUNT(*) FROM pool_entries WHERE pool_id=? AND match_value='keep.com'`, p.ID).Scan(&n)
			if n != 1 {
				t.Error("失败应保留旧数据")
			}
		})
	}
}

// TestSyncRunningConflict 同一池已有 running 时再次提交返回 ErrSyncRunning
func TestSyncRunningConflict(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		_, _ = w.Write([]byte("x.com\n"))
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "并发池", []string{srv.URL}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}
	// 等待任务真正开始（running 行已由首次提交写入，第二次会直接冲突）
	if _, err := svc.SubmitSync(ctx, p.ID); !errors.Is(err, ErrSyncRunning) {
		t.Fatalf("运行中应拒绝: %v", err)
	}
	close(release)
	waitTask(t, svc, p.ID, "succeeded")
}

// TestSyncCleanupOldTasks 终态写回时清理 7 天前历史任务，7 天内任务保留
func TestSyncCleanupOldTasks(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("z.com\n"))
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "清理池", []string{srv.URL}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	// 预置 8 天前与 1 天前两条终态任务
	if _, err := st.DB().Exec(`INSERT INTO pool_sync_tasks (pool_id, status, per_url_json, started_at, finished_at)
		VALUES (?, 'succeeded', '[]', datetime('now','-8 days'), datetime('now','-8 days'))`, p.ID); err != nil {
		t.Fatalf("插入旧任务失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO pool_sync_tasks (pool_id, status, per_url_json, started_at, finished_at)
		VALUES (?, 'succeeded', '[]', datetime('now','-1 day'), datetime('now','-1 day'))`, p.ID); err != nil {
		t.Fatalf("插入新任务失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	waitTask(t, svc, p.ID, "succeeded")
	var total int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_sync_tasks WHERE pool_id=?`, p.ID).Scan(&total); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	// 本次任务 + 1 天前任务 = 2；8 天前任务被清理
	if total != 2 {
		t.Errorf("历史任务清理异常，应保留 2 条，实际 %d", total)
	}
}

// TestGetStatusEmptyAndListEntriesNotFound 无任务状态返回空，不存在池列表返回 404
func TestGetStatusEmptyAndListEntriesNotFound(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "空状态池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	task, err := svc.GetStatus(ctx, p.ID)
	if err != nil || task != nil {
		t.Fatalf("无任务应返回 nil,nil，实际 task=%+v err=%v", task, err)
	}
	if _, _, err := svc.ListEntries(ctx, 99999, 1, 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在池列表应 ErrNotFound，实际 %v", err)
	}
	if _, _, err := svc.ListTasks(ctx, 99999, 1, 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("不存在池任务列表应 ErrNotFound，实际 %v", err)
	}
}

// TestSyncEmptyURLFails 无 URL 池同步返回 failed 且不清空旧数据
func TestSyncEmptyURLFails(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "无URL池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order, source_url)
		VALUES (?, 'DOMAIN-SUFFIX', 'keep.com', 'url', ?, 'https://old')`, p.ID, URLBase); err != nil {
		t.Fatalf("插入旧条目失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	task := waitTask(t, svc, p.ID, "failed")
	if !strings.Contains(task.Error, "未配置 URL") {
		t.Fatalf("错误信息异常: %s", task.Error)
	}
	var n int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_entries WHERE pool_id=?`, p.ID).Scan(&n); err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if n != 1 {
		t.Fatalf("空 URL 同步不应清空旧数据，实际 %d", n)
	}
}

// TestSyncRemovedAndSourceURL 差量删除回写 removed 且新条目记录 source_url
func TestSyncRemovedAndSourceURL(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("a.com\n"))
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "删除统计池", []string{srv.URL}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := st.DB().Exec(`INSERT INTO pool_entries (pool_id, rule_type, match_value, source, sort_order, source_url)
		VALUES (?, 'DOMAIN-SUFFIX', 'old.com', 'url', ?, ?)`, p.ID, URLBase, srv.URL); err != nil {
		t.Fatalf("插入旧条目失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("提交失败: %v", err)
	}
	task := waitTask(t, svc, p.ID, "succeeded")
	if len(task.PerURL) != 1 || task.PerURL[0].Removed != 1 {
		t.Fatalf("removed 应统计为 1，实际 %+v", task.PerURL)
	}
	var src string
	if err := st.DB().QueryRow(`SELECT source_url FROM pool_entries WHERE pool_id=? AND match_value='a.com'`, p.ID).Scan(&src); err != nil {
		t.Fatalf("查询 source_url 失败: %v", err)
	}
	if src != srv.URL {
		t.Fatalf("source_url 应为 %s，实际 %s", srv.URL, src)
	}
}

// TestSyncAddedOnlyNew 同一 URL 第二次同步 added 应为 0
func TestSyncAddedOnlyNew(t *testing.T) {
	_, svc := newTestService(t)
	ctx := context.Background()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("a.com\n"))
	}))
	defer srv.Close()
	p, err := svc.Create(ctx, "新增统计池", []string{srv.URL}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("首次提交失败: %v", err)
	}
	task1 := waitTask(t, svc, p.ID, "succeeded")
	if len(task1.PerURL) != 1 || task1.PerURL[0].Added != 1 {
		t.Fatalf("首次 added 应为 1，实际 %+v", task1.PerURL)
	}
	if _, err := svc.SubmitSync(ctx, p.ID); err != nil {
		t.Fatalf("二次提交失败: %v", err)
	}
	task2 := waitTask(t, svc, p.ID, "succeeded")
	if len(task2.PerURL) != 1 || task2.PerURL[0].Added != 0 {
		t.Fatalf("二次 added 应为 0，实际 %+v", task2.PerURL)
	}
}

// TestParseURLBodySkipReasons 解析返回跳过原因
func TestParseURLBodySkipReasons(t *testing.T) {
	entries, skipped, reasons, err := parseURLBody([]byte("DOMAIN-SUFFIX,a.com,extra\n#comment\nbad line\n"))
	if err != nil {
		t.Fatalf("解析不应失败: %v", err)
	}
	if len(entries) != 1 || entries[0].RuleType != "DOMAIN-SUFFIX" || entries[0].MatchValue != "a.com" {
		t.Fatalf("有效条目异常: %+v", entries)
	}
	if skipped < 2 || len(reasons) == 0 {
		t.Fatalf("应记录跳过原因，skipped=%d reasons=%v", skipped, reasons)
	}
}

// TestParseURLBodyScannerError 超长行必须返回错误，防止调用方把不完整解析结果当成功。
func TestParseURLBodyScannerError(t *testing.T) {
	longLine := strings.Repeat("a", 1024*1024+1)
	_, _, _, err := parseURLBody([]byte("DOMAIN-SUFFIX,ok.example\n" + longLine + "\n"))
	if err == nil {
		t.Fatal("超长行应返回 Scanner 错误")
	}
}

