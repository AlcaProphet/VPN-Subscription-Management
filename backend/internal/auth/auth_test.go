package auth

import (
	"context"
	"testing"
	"testing/fstest"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newTestAuthConfig 创建临时库 + 配置服务
func newTestAuthConfig(t *testing.T) *config.Service {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	fsys := fstest.MapFS{
		"0001_test.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return config.NewService(st, log.New("error", "console"))
}

// TestHashCheckPassword 哈希/校验往返
func TestHashCheckPassword(t *testing.T) {
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("哈希失败: %v", err)
	}
	if hash == "password123" {
		t.Error("哈希不应等于明文")
	}
	if !CheckPassword(hash, "password123") {
		t.Error("正确密码应校验通过")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("错误密码应校验失败")
	}
}

// TestValidatePassword 拒绝 <8 字符
func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("short123"); err != nil {
		t.Error("8 字符密码应通过")
	}
	if err := ValidatePassword("1234567"); err == nil {
		t.Error("7 字符密码应拒绝")
	}
	if err := ValidatePassword(""); err == nil {
		t.Error("空密码应拒绝")
	}
	// 中文按字符数计（rune）
	if err := ValidatePassword("密码密码密码密码"); err != nil {
		t.Error("8 个中文字符应通过（按字符数）")
	}
}

// TestNormalizeEmail trim/小写/控制字符拒绝
func TestNormalizeEmail(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"  Test@Example.COM  ", "test@example.com", false},
		{"user@example.com", "user@example.com", false},
		{"no-at-sign", "", true},
		{"", "", true},
		{"user@exa\nmple.com", "", true}, // 控制字符拒绝（防 SMTP 头注入）
		{"user@exa\x7fmple.com", "", true},
	}
	for _, c := range cases {
		got, err := NormalizeEmail(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizeEmail(%q) 应报错", c.in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("NormalizeEmail(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

// TestParseRejectForgedSignature 伪造签名拒绝
func TestParseRejectForgedSignature(t *testing.T) {
	cfg := newTestAuthConfig(t)
	svc := NewService(cfg, nil, nil)
	ctx := context.Background()
	token, _, err := svc.Issue(ctx, 1, 0, time.Hour)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	claims, err := svc.Parse(ctx, token)
	if err != nil || claims == nil || claims.UserID != 1 {
		t.Fatalf("正常解析失败: %v", err)
	}
	// 篡改 token（伪造签名）
	if _, err := svc.Parse(ctx, token[:len(token)-2]+"xx"); err == nil {
		t.Error("篡改的凭据应解析失败")
	}
}
