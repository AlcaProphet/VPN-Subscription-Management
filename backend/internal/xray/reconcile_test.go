package xray

import (
	"context"
	"errors"
	"testing"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/tasks"
)

func TestCredentialsOneExtSkipsExceeded(t *testing.T) {
	st, extSvc, fake, instID, nodeID := newExtTestEnv(t)
	ctx := context.Background()
	quota := float64(1)
	acc, _, err := extSvc.CreateExt(ctx, "ext-quota", "manual", "uuid-1", "secret-1", &quota, []ExtPushTarget{{InstanceID: instID, InboundTag: "in-a"}})
	if err != nil {
		t.Fatalf("创建独立账号失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, `UPDATE xray_ext_accounts SET quota_exceeded=1 WHERE id=?`, acc.ID); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	instSvc := NewInstanceService(st, log.New("error", "console"), tasks.NewRegistry())
	syncSvc := NewSyncService(st, cfg, NewCredentialService(st, cfg), instSvc, tasks.NewRegistry(), log.New("error", "console"))
	syncSvc.SetExtService(extSvc)
	item := ReconcileItem{
		Source: "ext", ExtAccountID: &acc.ID,
		InstanceID: instID, InboundTag: "in-a", NodeID: nodeID,
	}
	err = syncSvc.CredentialsOne(ctx, item)
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("超限凭据修复应返回 ErrQuotaExceeded，实际 %v", err)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.removed) != 0 || len(fake.added) != 1 {
		t.Fatalf("超限账号不应执行 Remove/Add，当前 removed=%d added=%d", len(fake.removed), len(fake.added))
	}
}

func TestReconcileExtRemoveCandidateNotInToPush(t *testing.T) {
	st, extSvc, fake, instID, _ := newExtTestEnv(t)
	ctx := context.Background()
	acc, _, err := extSvc.CreateExt(ctx, "ext-remove", "manual", "uuid-2", "secret-2", nil, []ExtPushTarget{{InstanceID: instID, InboundTag: "in-a"}})
	if err != nil {
		t.Fatalf("创建独立账号失败: %v", err)
	}
	// 模拟移除失败：保留本地行，标记 action=remove。
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE xray_ext_users SET sync_status='failed', action='remove', last_error='remove failed' WHERE ext_account_id=?`, acc.ID); err != nil {
		t.Fatal(err)
	}
	// 独立账号的期望集不应再包含该目标。
	targets, err := extSvc.pushTargetsFor(ctx, acc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 0 {
		t.Fatalf("action=remove 的目标不应出现在 add 目标中: %+v", targets)
	}
	// fake 仍保留 Xray 侧账号，用于对账发现待移除。
	fake.mu.Lock()
	fake.added["in-a|"+ExtEmail(acc.ID)] = true
	fake.mu.Unlock()
	_ = st
	_ = fake
}
