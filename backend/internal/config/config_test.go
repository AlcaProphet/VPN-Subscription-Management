package config

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"testing/fstest"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
)

// newTestStore 创建临时库并迁移
func newTestStore(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	// 用最小内联迁移（避免依赖 embed FS 中的其他迁移）
	fsys := fstest.MapFS{
		"0001_test.sql": &fstest.MapFile{Data: []byte(`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY, applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);
			CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY, value TEXT, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);`)},
	}
	if err := st.Migrate(context.Background(), fsys); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := NewService(st, log.New("error", "console"))
	return st, cfg
}

// TestEncryptDecryptRoundTrip 加密/解密往返一致
func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef") // 32 字节
	plain := "smtp-password-secret"
	enc, err := Encrypt([]byte(plain), key)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	// 密文应为 base64url 且不含明文
	if strings.Contains(enc, plain) {
		t.Error("密文不应包含明文")
	}
	if _, err := base64.RawURLEncoding.DecodeString(enc); err != nil {
		t.Errorf("密文不是合法 base64url: %v", err)
	}
	dec, err := Decrypt(enc, key)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}
	if string(dec) != plain {
		t.Errorf("往返不一致: got %q want %q", dec, plain)
	}
}

// TestDecryptTampered 篡改密文返回错误（防篡改）
func TestDecryptTampered(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := Encrypt([]byte("secret"), key)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(enc)
	raw[len(raw)-1] ^= 0x01 // 篡改最后一个字节
	if _, err := Decrypt(base64.RawURLEncoding.EncodeToString(raw), key); err == nil {
		t.Error("篡改后的密文应解密失败")
	}
	// 错误密钥解密失败
	if _, err := Decrypt(enc, []byte("another-key-another-key-1234")); err == nil {
		t.Error("错误密钥应解密失败")
	}
}

// TestSensitiveSetGet 敏感键 Set/Get 自动加解密
func TestSensitiveSetGet(t *testing.T) {
	_, cfg := newTestStore(t)
	ctx := context.Background()
	RegisterSensitive("test_secret")
	t.Cleanup(func() { delete(sensitiveKeys, "test_secret") })

	if err := cfg.Set(ctx, "test_secret", "plain-value"); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	// 库内应为密文
	raw, err := cfg.GetRaw(ctx, "test_secret")
	if err != nil {
		t.Fatalf("GetRaw 失败: %v", err)
	}
	if raw == "plain-value" {
		t.Error("敏感值应以密文落库")
	}
	// 读取自动解密
	got, err := cfg.Get(ctx, "test_secret")
	if err != nil {
		t.Fatalf("Get 失败: %v", err)
	}
	if got != "plain-value" {
		t.Errorf("Get 解密结果不一致: got %q", got)
	}
}

// TestGetSetBasic 普通键读写与类型化读取
func TestGetSetBasic(t *testing.T) {
	_, cfg := newTestStore(t)
	ctx := context.Background()
	if err := cfg.Set(ctx, "some_key", "some_value"); err != nil {
		t.Fatalf("Set 失败: %v", err)
	}
	v, err := cfg.Get(ctx, "some_key")
	if err != nil || v != "some_value" {
		t.Errorf("Get 结果异常: v=%q err=%v", v, err)
	}
	// 覆盖写入
	if err := cfg.Set(ctx, "some_key", "new_value"); err != nil {
		t.Fatalf("Set 覆盖失败: %v", err)
	}
	v, _ = cfg.Get(ctx, "some_key")
	if v != "new_value" {
		t.Errorf("覆盖写入失败: got %q", v)
	}
	// 未设置返回空串
	v, err = cfg.Get(ctx, "missing")
	if err != nil || v != "" {
		t.Errorf("未设置键应返回空串: v=%q err=%v", v, err)
	}
	// 类型化读取
	if err := cfg.Set(ctx, "flag", "true"); err != nil {
		t.Fatal(err)
	}
	if !cfg.GetBool(ctx, "flag", false) {
		t.Error("GetBool(true) 应返回 true")
	}
	if !cfg.GetBool(ctx, "missing", true) {
		t.Error("GetBool 默认值应生效")
	}
	if err := cfg.Set(ctx, "num", "42"); err != nil {
		t.Fatal(err)
	}
	if n := cfg.GetInt(ctx, "num", 0); n != 42 {
		t.Errorf("GetInt 异常: got %d", n)
	}
}

// TestEnsureSigningKey 签名密钥确保：生成后复用不重复生成
func TestEnsureSigningKey(t *testing.T) {
	_, cfg := newTestStore(t)
	ctx := context.Background()
	k1, err := cfg.EnsureSigningKey(ctx)
	if err != nil {
		t.Fatalf("EnsureSigningKey 失败: %v", err)
	}
	if len(k1) != 32 {
		t.Errorf("签名密钥长度应为 32: got %d", len(k1))
	}
	k2, err := cfg.EnsureSigningKey(ctx)
	if err != nil {
		t.Fatalf("再次 EnsureSigningKey 失败: %v", err)
	}
	if string(k1) != string(k2) {
		t.Error("签名密钥应复用不重复生成")
	}
}
