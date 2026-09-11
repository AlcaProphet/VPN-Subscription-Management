package xray

import (
	"context"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newTrafficSummaryEnv(t *testing.T) (*store.Store, *SyncService, int64) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	lg := log.New("error", "console")
	cfg := config.NewService(st, lg)
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO groups (slug,name,is_default,default_quota) VALUES ('group-a','g',1,5)`); err != nil {
		t.Fatalf("插入组失败: %v", err)
	}
	res, err := st.DB().ExecContext(ctx, `INSERT INTO users (username,email,password_hash,user_source,status,group_id) VALUES ('u','u@x.com','h','local','active',1)`)
	if err != nil {
		t.Fatalf("插入用户失败: %v", err)
	}
	uid, _ := res.LastInsertId()
	syncSvc := NewSyncService(st, cfg, NewCredentialService(st, cfg), NewInstanceService(st, lg, nil), nil, lg)
	return st, syncSvc, uid
}

func setTraffic(t *testing.T, st *store.Store, userID int64, ym string, uplink, downlink int64) {
	t.Helper()
	if _, err := st.DB().Exec(`INSERT INTO traffic_records (user_id,ym,uplink,downlink) VALUES (?,?,?,?)`, userID, ym, uplink, downlink); err != nil {
		t.Fatalf("插入流量失败: %v", err)
	}
}

// TestTrafficSummaryBasicModeUnlimited 基础模式恒不限：即使库中有流量也不读取、不返回配额。
func TestTrafficSummaryBasicModeUnlimited(t *testing.T) {
	st, svc, uid := newTrafficSummaryEnv(t)
	ctx := context.Background()
	setTraffic(t, st, uid, currentYM(), 1024, 2048)
	summary, err := svc.TrafficSummaryForUser(ctx, uid)
	if err != nil {
		t.Fatalf("基础模式汇总失败: %v", err)
	}
	if !summary.Unlimited || summary.UsedBytes != 0 || summary.QuotaBytes != nil || summary.Exceeded {
		t.Fatalf("基础模式应为 unlimited=true/used=0/quota=null/exceeded=false: %+v", summary)
	}
}

// TestTrafficSummaryAdvancedUnlimited 高级模式无有效配额：返回已用流量但 quota_bytes=null。
func TestTrafficSummaryAdvancedUnlimited(t *testing.T) {
	st, svc, uid := newTrafficSummaryEnv(t)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE groups SET default_quota = NULL WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	setTraffic(t, st, uid, currentYM(), 1024, 2048)
	summary, err := svc.TrafficSummaryForUser(ctx, uid)
	if err != nil {
		t.Fatalf("高级不限汇总失败: %v", err)
	}
	if !summary.Unlimited || summary.UsedBytes != 3072 || summary.QuotaBytes != nil || summary.Exceeded {
		t.Fatalf("高级不限应为 unlimited=true/used=3072/quota=null/exceeded=false: %+v", summary)
	}
}

// TestTrafficSummaryAdvancedQuotaMatrix 高级模式配额矩阵：未超限 / 已超限 / quota_bytes 字节换算。
func TestTrafficSummaryAdvancedQuotaMatrix(t *testing.T) {
	st, svc, uid := newTrafficSummaryEnv(t)
	ctx := context.Background()
	if err := svc.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}
	setTraffic(t, st, uid, currentYM(), 1<<30, 0)
	summary, err := svc.TrafficSummaryForUser(ctx, uid)
	if err != nil {
		t.Fatalf("配额内汇总失败: %v", err)
	}
	const fiveGiB = int64(5) << 30
	if summary.Unlimited || summary.UsedBytes != 1<<30 || summary.QuotaBytes == nil || *summary.QuotaBytes != fiveGiB || summary.Exceeded {
		t.Fatalf("配额内汇总异常: %+v", summary)
	}
	if _, err := st.DB().Exec(`UPDATE users SET quota_exceeded = 1 WHERE id = ?`, uid); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().Exec(`UPDATE traffic_records SET uplink = ? WHERE user_id = ?`, 6<<30, uid); err != nil {
		t.Fatal(err)
	}
	summary, err = svc.TrafficSummaryForUser(ctx, uid)
	if err != nil {
		t.Fatalf("超限汇总失败: %v", err)
	}
	if summary.Unlimited || !summary.Exceeded || summary.UsedBytes != 6<<30 || summary.QuotaBytes == nil || *summary.QuotaBytes != fiveGiB {
		t.Fatalf("超限汇总异常: %+v", summary)
	}
}
