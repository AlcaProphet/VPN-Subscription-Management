package setup

import (
	"context"
	"errors"
	"net/http/httptest"
	"regexp"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newTestSetupService 创建临时库 + setup 服务（含全部 Build1 表迁移）
func newTestSetupService(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		"0003_groups_platforms.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS groups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL UNIQUE,
			is_default INTEGER NOT NULL DEFAULT 0,
			needs_reselect INTEGER NOT NULL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS platforms (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			schemes TEXT NOT NULL DEFAULT '[]',
			extra_headers TEXT NOT NULL DEFAULT '{}',
			installer_file TEXT,
			installer_url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	svc := NewService(st, cfg, log.New("error", "console"), "auto")
	return st, svc
}

// TestGenerateSlug 格式匹配与冲突重试
func TestGenerateSlug(t *testing.T) {
	_, svc := newTestSetupService(t)
	ctx := context.Background()
	// 注入 exists 冲突两次后成功
	attempts := 0
	slug, err := svc.GenerateSlug(ctx, nil, "group-", func(s string) (bool, error) {
		attempts++
		return attempts <= 2, nil // 前两次冲突，第三次成功
	})
	if err != nil {
		t.Fatalf("GenerateSlug 失败: %v", err)
	}
	if !regexp.MustCompile(`^group-[a-z0-9]{8}$`).MatchString(slug) {
		t.Errorf("slug 格式异常: %s", slug)
	}
	// 一直冲突 → 超限报错
	_, err = svc.GenerateSlug(ctx, nil, "platform-", func(s string) (bool, error) {
		return true, nil
	})
	if err == nil {
		t.Error("连续冲突应报错")
	}
}

// TestCompleteQuickStart 快速开始事务：默认组 + 3 平台 + configured + frontend_url
func TestCompleteQuickStart(t *testing.T) {
	st, svc := newTestSetupService(t)
	ctx := context.Background()
	req := httptest.NewRequest("POST", "http://vpn.example.com/api/setup/quickstart", nil)
	if err := svc.CompleteQuickStart(ctx, req); err != nil {
		t.Fatalf("快速开始失败: %v", err)
	}
	// 默认组
	var defaultGroup int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM groups WHERE is_default = 1 AND name = '默认组'`).Scan(&defaultGroup); err != nil {
		t.Fatalf("查询默认组失败: %v", err)
	}
	if defaultGroup != 1 {
		t.Error("预置默认组缺失")
	}
	// 3 个平台
	var platforms int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM platforms`).Scan(&platforms); err != nil {
		t.Fatalf("查询平台失败: %v", err)
	}
	if platforms != 3 {
		t.Errorf("默认平台数量异常: %d", platforms)
	}
	// clash-verge 三条附加头
	var headers string
	if err := st.DB().QueryRow(`SELECT extra_headers FROM platforms WHERE name = 'Clash Verge'`).Scan(&headers); err != nil {
		t.Fatalf("查询附加头失败: %v", err)
	}
	if headers != `{"Content-Disposition":"attachment; filename*=UTF-8''subscription.yaml","profile-update-interval":"300","profile-web-page-url":"{frontend_url}"}` {
		t.Errorf("Clash Verge 附加头异常: %s", headers)
	}
	// configured 与 frontend_url
	cfgVal, _ := svc.cfg.Get(ctx, config.KeyConfigured)
	if cfgVal != "true" {
		t.Error("configured 未置位")
	}
	furl, _ := svc.cfg.Get(ctx, config.KeyFrontendURL)
	if furl != "http://vpn.example.com" {
		t.Errorf("frontend_url 推导异常: %s", furl)
	}
	// 签名密钥已生成
	if _, err := svc.cfg.GetSigningKey(ctx); err != nil {
		t.Errorf("签名密钥未生成: %v", err)
	}
	// 重复调用返回 ErrAlreadyConfigured
	if err := svc.CompleteQuickStart(ctx, req); !errors.Is(err, ErrAlreadyConfigured) {
		t.Errorf("重复调用应返回 ErrAlreadyConfigured: %v", err)
	}
}

// TestQuickStartRollback 注入中途失败 → 整体回滚（表内无残留）
func TestQuickStartRollback(t *testing.T) {
	// 构造缺失表（无 groups 表）的库 → 快速开始失败 → 无残留配置
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开失败: %v", err)
	}
	defer st.Close()
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
		// 故意不建 groups/platforms 表 → INSERT 失败
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	svc := NewService(st, cfg, log.New("error", "console"), "auto")
	req := httptest.NewRequest("POST", "http://vpn.example.com/api/setup/quickstart", nil)
	if err := svc.CompleteQuickStart(context.Background(), req); err == nil {
		t.Fatal("缺表场景应失败")
	}
	// 配置应整体回滚：configured 未置位、签名密钥未写入
	cfgVal, _ := cfg.Get(context.Background(), config.KeyConfigured)
	if cfgVal != "" {
		t.Error("事务应回滚，configured 不应存在")
	}
	if _, err := cfg.GetSigningKey(context.Background()); err == nil {
		t.Error("事务应回滚，签名密钥不应写入")
	}
}

// TestDeriveFrontendURL X-Forwarded-Host 优先（可信时）、Host 兜底、https 推导
func TestDeriveFrontendURL(t *testing.T) {
	// 可信 + X-Forwarded-Host 优先
	req := httptest.NewRequest("POST", "http://inner/api", nil)
	req.Header.Set("X-Forwarded-Host", "vpn.example.com, other.com")
	req.Header.Set("X-Forwarded-Proto", "https")
	if got := DeriveFrontendURL(req, true); got != "https://vpn.example.com" {
		t.Errorf("XFH 优先推导异常: %s", got)
	}
	// 不可信 → 忽略 X-Forwarded-Host 用 Host
	req2 := httptest.NewRequest("POST", "http://inner/api", nil)
	req2.Header.Set("X-Forwarded-Host", "evil.example.com")
	if got := DeriveFrontendURL(req2, false); got != "http://inner" {
		t.Errorf("不可信时应用 Host: %s", got)
	}
	// https 推导（X-Forwarded-Proto）
	req3 := httptest.NewRequest("POST", "http://inner/api", nil)
	req3.Header.Set("X-Forwarded-Proto", "https")
	if got := DeriveFrontendURL(req3, true); got != "https://inner" {
		t.Errorf("https 推导异常: %s", got)
	}
}

// TestTrustProxyTiers TRUST_PROXY 三档对 X-Forwarded-Host 信任的影响（Design1 §6.4）
func TestTrustProxyTiers(t *testing.T) {
	// on：公网来源也信任转发头
	onSvc := NewService(nil, nil, log.New("error", "console"), "on")
	req := httptest.NewRequest("POST", "http://inner/api", nil)
	req.RemoteAddr = "203.0.113.5:12345" // 公网 IP
	req.Header.Set("X-Forwarded-Host", "vpn.example.com")
	if got := DeriveFrontendURL(req, onSvc.trustedForwarded(req)); got != "http://vpn.example.com" {
		t.Errorf("on 档应信任转发头: %s", got)
	}

	// off：即使回环来源也不信任
	offSvc := NewService(nil, nil, log.New("error", "console"), "off")
	req2 := httptest.NewRequest("POST", "http://inner/api", nil)
	req2.RemoteAddr = "127.0.0.1:12345" // 回环
	req2.Header.Set("X-Forwarded-Host", "vpn.example.com")
	if got := DeriveFrontendURL(req2, offSvc.trustedForwarded(req2)); got != "http://inner" {
		t.Errorf("off 档应忽略转发头: %s", got)
	}

	// auto：回环来源信任
	autoSvc := NewService(nil, nil, log.New("error", "console"), "auto")
	req3 := httptest.NewRequest("POST", "http://inner/api", nil)
	req3.RemoteAddr = "127.0.0.1:12345"
	req3.Header.Set("X-Forwarded-Host", "vpn.example.com")
	if got := DeriveFrontendURL(req3, autoSvc.trustedForwarded(req3)); got != "http://vpn.example.com" {
		t.Errorf("auto 档回环来源应信任转发头: %s", got)
	}
	// auto：公网来源不信任
	req4 := httptest.NewRequest("POST", "http://inner/api", nil)
	req4.RemoteAddr = "203.0.113.5:12345"
	req4.Header.Set("X-Forwarded-Host", "vpn.example.com")
	if got := DeriveFrontendURL(req4, autoSvc.trustedForwarded(req4)); got != "http://inner" {
		t.Errorf("auto 档公网来源应忽略转发头: %s", got)
	}
}
