package xray

import (
	"context"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func TestEnsureCredentialsAndDecrypt(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(ctx, config.KeySigningKey, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}
	res, err := st.DB().ExecContext(ctx, `INSERT INTO users (username,email,password_hash,user_source) VALUES ('u','u@x.com','h','local')`)
	if err != nil {
		t.Fatal(err)
	}
	uid, _ := res.LastInsertId()
	svc := NewCredentialService(st, cfg)
	if err := svc.EnsureCredentials(ctx, uid); err != nil {
		t.Fatalf("EnsureCredentials 失败: %v", err)
	}
	uuid, secret, err := svc.Credentials(ctx, uid)
	if err != nil {
		t.Fatalf("Credentials 失败: %v", err)
	}
	if uuid == "" || secret == "" {
		t.Fatalf("凭据不应为空: uuid=%q secret=%q", uuid, secret)
	}
	var ue, se string
	if err := st.DB().QueryRowContext(ctx, `SELECT uuid_encrypted, proxy_secret_encrypted FROM users WHERE id=?`, uid).Scan(&ue, &se); err != nil {
		t.Fatal(err)
	}
	if ue == "" || se == "" {
		t.Fatalf("凭据密文不应为空: %q %q", ue, se)
	}
}
