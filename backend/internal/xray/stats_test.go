package xray

import (
	"context"
	"strings"
	"testing"

	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func TestUpsertTrafficIncrements(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatal(err)
	}
	res, err := st.DB().ExecContext(ctx, `INSERT INTO users (username,email,password_hash,user_source) VALUES ('u','u@x.com','h','local')`)
	if err != nil {
		t.Fatal(err)
	}
	uid, _ := res.LastInsertId()
	ym := currentYM()
	if err := upsertTraffic(ctx, st, uid, ym, 100, 200); err != nil {
		t.Fatal(err)
	}
	if err := upsertTraffic(ctx, st, uid, ym, 50, 25); err != nil {
		t.Fatal(err)
	}
	var up, down int64
	if err := st.DB().QueryRowContext(ctx, `SELECT uplink, downlink FROM traffic_records WHERE user_id=? AND ym=?`, uid, ym).Scan(&up, &down); err != nil {
		t.Fatal(err)
	}
	if up != 150 || down != 225 {
		t.Fatalf("流量累加错误: up=%d down=%d", up, down)
	}
	if !strings.HasPrefix(ym, "20") || len(ym) != 7 {
		t.Fatalf("currentYM 格式异常: %q", ym)
	}
}
