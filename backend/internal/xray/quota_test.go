package xray

import (
	"context"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func TestEffectiveQuotaAndResetOnlyActive(t *testing.T) {
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
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO groups (slug,name,is_default,default_quota) VALUES ('group-a','g',1,5)`); err != nil {
		t.Fatal(err)
	}
	res, err := st.DB().ExecContext(ctx, `INSERT INTO users (username,email,password_hash,user_source,status,group_id) VALUES ('u','u@x.com','h','local','active',1)`)
	if err != nil {
		t.Fatal(err)
	}
	uid, _ := res.LastInsertId()
	syncSvc := NewSyncService(st, cfg, NewCredentialService(st, cfg), NewInstanceService(st, log.New("error", "console"), nil), nil, log.New("error", "console"))
	q, err := syncSvc.EffectiveQuota(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if q == nil || *q != 5 {
		t.Fatalf("默认组配额应为 5，实际 %v", q)
	}
	if _, err := st.DB().ExecContext(ctx, `UPDATE users SET quota_override=10 WHERE id=?`, uid); err != nil {
		t.Fatal(err)
	}
	q, err = syncSvc.EffectiveQuota(ctx, uid)
	if err != nil {
		t.Fatal(err)
	}
	if q == nil || *q != 10 {
		t.Fatalf("覆盖配额应为 10，实际 %v", q)
	}
	if _, err := st.DB().ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=?`, uid); err != nil {
		t.Fatal(err)
	}
	if err := syncSvc.ResetQuota(ctx, uid); err == nil {
		t.Fatal("非 active 用户重置应报错")
	}
}
