package log

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动（与 store 包同源，测试避免循环依赖直接使用）
)

// newTestLogDB 创建临时库（含 access_logs 与 users 表）；直接使用 sql.DB 避免 store→log 循环依赖
func newTestLogDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "test.db")+"?_txlock=immediate")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`); err != nil {
		t.Fatalf("初始化 PRAGMA 失败: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT, username TEXT NOT NULL, email TEXT UNIQUE,
		role TEXT NOT NULL DEFAULT 'user', user_source TEXT NOT NULL DEFAULT 'local',
		status TEXT NOT NULL DEFAULT 'active');
		CREATE TABLE IF NOT EXISTS access_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, user_id INTEGER, ip TEXT NOT NULL,
		download_type TEXT NOT NULL, platform TEXT, resource_slug TEXT NOT NULL,
		status TEXT NOT NULL, fail_reason TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	return db
}

// TestAccessQuery 访问日志分页/日期筛选/清空/联查用户名
func TestAccessQuery(t *testing.T) {
	db := newTestLogDB(t)
	ctx := context.Background()
	svc := NewAccessService(db, slog.New(slog.NewTextHandler(io.Discard, nil)))

	// 用户 + 两条日志（不同日期）
	if _, err := db.Exec(`INSERT INTO users (username, email) VALUES ('alice','alice@example.com')`); err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO access_logs (user_id, ip, download_type, resource_slug, status, created_at)
		VALUES (1, '1.1.1.1', 'subscription', 'sub-x', 'success', '2026-08-01 10:00:00')`); err != nil {
		t.Fatalf("插入日志失败: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO access_logs (user_id, ip, download_type, resource_slug, status, fail_reason, created_at)
		VALUES (NULL, '2.2.2.2', 'share', 'share-y', 'fail', 'token_invalid', '2026-08-05 12:00:00')`); err != nil {
		t.Fatalf("插入日志失败: %v", err)
	}

	// 日期筛选（8 月 2 日无记录）
	list, total, err := svc.Query(ctx, "2026-08-02", "2026-08-03", 1, 20)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Errorf("日期筛选异常: total=%d", total)
	}
	// 全量 + 联查用户名 + 空 user 展示空串
	list, total, err = svc.Query(ctx, "", "", 1, 20)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 2 || len(list) != 2 {
		t.Errorf("全量查询异常: total=%d", total)
	}
	// ORDER BY created_at DESC：最新（share，无用户）在前，较早（subscription，alice）在后
	if list[0].Username != "" || list[1].Username != "alice" {
		t.Errorf("用户名联查异常: %q %q", list[0].Username, list[1].Username)
	}
	// 分页
	list, total, err = svc.Query(ctx, "", "", 1, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 2 || len(list) != 1 {
		t.Errorf("分页异常: total=%d len=%d", total, len(list))
	}
	// 非法日期
	if _, _, err := svc.Query(ctx, "bad-date", "", 1, 20); err == nil {
		t.Error("非法日期应报错")
	}
	// 清空
	if err := svc.Clear(ctx); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	list, total, err = svc.Query(ctx, "", "", 1, 20)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if total != 0 || len(list) != 0 {
		t.Errorf("清空后应为空: total=%d", total)
	}
}

// TestRingBufferCapacity 环形缓冲：写入 600 条 → 仅存最近 500 条
func TestRingBufferCapacity(t *testing.T) {
	buf := NewRingBuffer()
	for i := 0; i < 600; i++ {
		buf.Append(Entry{Time: time.Now(), Level: "info", Message: "m", Attrs: ""})
	}
	h := buf.History()
	if len(h) != RingBufferSize {
		t.Errorf("缓冲应保留最近 %d 条: %d", RingBufferSize, len(h))
	}
	// 订阅者收到广播
	ch := make(chan Entry, 64)
	buf.mu.Lock()
	buf.subs[ch] = struct{}{}
	buf.mu.Unlock()
	buf.Append(Entry{Message: "broadcast"})
	select {
	case e := <-ch:
		if e.Message != "broadcast" {
			t.Errorf("广播内容异常: %q", e.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("订阅者未收到广播")
	}
}

// TestStreamTokenOneTime 短期 Token 一次性：ConsumeToken 后再用失败；过期失效
func TestStreamTokenOneTime(t *testing.T) {
	buf := NewRingBuffer()
	svc := NewStreamService(buf, slog.New(slog.NewTextHandler(io.Discard, nil)))
	token, err := svc.IssueToken()
	if err != nil {
		t.Fatalf("换取 Token 失败: %v", err)
	}
	if len(token) < 32 {
		t.Errorf("Token 熵不足: %d", len(token))
	}
	if !svc.ConsumeToken(token) {
		t.Fatal("首次消费应成功")
	}
	if svc.ConsumeToken(token) {
		t.Error("用后即删：二次消费应失败")
	}
	// 过期 Token 失效（直接注入过期时间）
	token2, _ := svc.IssueToken()
	svc.mu.Lock()
	svc.tokens[token2] = time.Now().Add(-time.Minute)
	svc.mu.Unlock()
	if svc.ConsumeToken(token2) {
		t.Error("过期 Token 应失效")
	}
}

// TestStreamConnectionLimit 8 连接上限：第 9 个 Subscribe 返回 false
func TestStreamConnectionLimit(t *testing.T) {
	buf := NewRingBuffer()
	svc := NewStreamService(buf, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var chans []chan Entry
	for i := 0; i < MaxSSEConnections; i++ {
		ch, _, ok := svc.Subscribe()
		if !ok {
			t.Fatalf("第 %d 个连接应成功", i+1)
		}
		chans = append(chans, ch)
	}
	if _, _, ok := svc.Subscribe(); ok {
		t.Error("第 9 个连接应被拒绝")
	}
	// 断开后释放
	svc.Unsubscribe(chans[0])
	if _, _, ok := svc.Subscribe(); !ok {
		t.Error("断开后应可再订阅")
	}
	svc.Unsubscribe(chans[1])
}

// TestStreamReset Reset 后 tokens/缓冲/连接全复位
func TestStreamReset(t *testing.T) {
	buf := NewRingBuffer()
	svc := NewStreamService(buf, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, _ = svc.IssueToken()
	_, _, ok := svc.Subscribe()
	if !ok {
		t.Fatal("订阅失败")
	}
	buf.Append(Entry{Message: "x"})
	svc.Reset()
	if len(svc.tokens) != 0 || svc.connCount != 0 {
		t.Errorf("Reset 后内存态应复位: tokens=%d conns=%d", len(svc.tokens), svc.connCount)
	}
	if len(buf.History()) != 0 {
		t.Error("Reset 后缓冲应清空")
	}
	// Reset 已关闭全部订阅通道（含 ch）；断开后不重复 Unsubscribe（通道已关闭，幂等由调用方保证）
	if _, _, ok := svc.Subscribe(); !ok {
		t.Error("Reset 后应可重新订阅")
	}
}

// TestRingHandlerTokenRedact 环形缓冲内容经 token 脱敏（路径属性含 ?token= 值）
func TestRingHandlerTokenRedact(t *testing.T) {
	buf := NewRingBuffer()
	// 与 log.New 同构：Redact 外层包裹 Ring
	logger := New("debug", "console", buf)
	logger.Info("http_request", "path", "/subscriptions/x/download?token=SECRET123")
	time.Sleep(10 * time.Millisecond)
	h := buf.History()
	if len(h) == 0 {
		t.Fatal("缓冲应为空")
	}
	if contains(h[0].Attrs, "SECRET123") {
		t.Errorf("缓冲内容应脱敏: %q", h[0].Attrs)
	}
	if !contains(h[0].Attrs, "token=***") {
		t.Errorf("缓冲应含脱敏标记: %q", h[0].Attrs)
	}
}

// contains 子串判定
func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

// indexOf 子串位置（避免 strings import 的极简实现）
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// TestRedact 覆盖验收项：路径内 token、多参数间 token、消息体内嵌 token 均脱敏（Build1 既有测试，保留）
func TestRedact(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/x?token=abc", "/x?token=***"},
		{"a=1&token=abc&b=2", "a=1&token=***&b=2"},
		{"/sub?token=xyz&platform=clash", "/sub?token=***&platform=clash"},
		{"no token here", "no token here"},
		{"token=without_prefix", "token=without_prefix"}, // 无 ? 或 & 前缀不匹配
	}
	for _, c := range cases {
		if got := Redact(c.in); got != c.want {
			t.Errorf("Redact(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNewFormats 验证双格式输出与分级（Build1 既有测试，保留）
func TestNewFormats(t *testing.T) {
	// console 格式
	l1 := New("info", "console")
	if l1 == nil {
		t.Fatal("console logger 构建失败")
	}
	// json 格式
	l2 := New("debug", "json")
	if l2 == nil {
		t.Fatal("json logger 构建失败")
	}
	// 级别过滤：error 级别下 info 不输出
	l3 := New("error", "console")
	ctx := context.Background()
	if l3.Enabled(ctx, slog.LevelInfo) {
		t.Error("error 级别下 info 不应启用")
	}
	if !l3.Enabled(ctx, slog.LevelError) {
		t.Error("error 级别下 error 应启用")
	}
	// SetLevel 运行时切换生效
	SetLevel("debug")
	if !l3.Enabled(ctx, slog.LevelDebug) {
		t.Error("切换 debug 后 debug 应启用")
	}
}

