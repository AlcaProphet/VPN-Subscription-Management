package group

import (
	"context"
	"testing"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/migrations"
)

func newFullTestGroupService(t *testing.T) (*store.Store, *Service) {
	t.Helper()
	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return st, NewService(st, log.New("error", "console"))
}

func seedCandidateSet(t *testing.T, st *store.Store) (groupID int64, nodeA, nodeB int64) {
	t.Helper()
	ctx := context.Background()
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO groups (slug, name, is_default) VALUES ('group-adv', '高级组', 0)`); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM groups WHERE slug='group-adv'`).Scan(&groupID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO platforms (slug, name) VALUES ('plat', '平台')`); err != nil {
		t.Fatal(err)
	}
	var platformID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM platforms WHERE slug='plat'`).Scan(&platformID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO subscriptions (slug, name, platform_id, current_version) VALUES ('sub', '订阅', ?, 1)`, platformID); err != nil {
		t.Fatal(err)
	}
	var subID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM subscriptions WHERE slug='sub'`).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription', ?, 1, 'x.yaml')`, subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO assembly_blueprints (version_id, target_syntax, selection_json) VALUES ((SELECT id FROM versions WHERE owner_type='subscription' AND owner_id=? AND version_no=1), 'clash-yaml', '{"xray_candidates":["node-a","node-b"]}')`, subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO xray_instances (name, slug, api_addr) VALUES ('inst', 'instance-abc', '127.0.0.1:10086')`); err != nil {
		t.Fatal(err)
	}
	var instID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM xray_instances WHERE slug='instance-abc'`).Scan(&instID); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"node-a", "node-b"} {
		if _, err := st.DB().ExecContext(ctx,
			`INSERT INTO nodes (source, name, instance_id, tag, protocol, host, port, enabled, allocatable, missing)
			 VALUES ('xray', ?, ?, ?, 'vless', 'h', 443, 1, 1, 0)`, name, instID, name); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM nodes WHERE name='node-a'`).Scan(&nodeA); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM nodes WHERE name='node-b'`).Scan(&nodeB); err != nil {
		t.Fatal(err)
	}
	return groupID, nodeA, nodeB
}

func TestCandidateSet(t *testing.T) {
	st, svc := newFullTestGroupService(t)
	ctx := context.Background()
	seedCandidateSet(t, st)
	candidates, err := svc.CandidateSet(ctx)
	if err != nil {
		t.Fatalf("读取候选集失败: %v", err)
	}
	if len(candidates) != 2 || candidates[0].Name != "node-a" || candidates[1].Name != "node-b" {
		t.Fatalf("候选集异常: %+v", candidates)
	}
}

func TestSetNodesAndRecompute(t *testing.T) {
	st, svc := newFullTestGroupService(t)
	ctx := context.Background()
	groupID, nodeA, nodeB := seedCandidateSet(t, st)
	if err := svc.SetNodes(ctx, groupID, []int64{nodeA, nodeB}); err != nil {
		t.Fatalf("设置节点失败: %v", err)
	}
	var cnt int
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM group_nodes WHERE group_id = ?`, groupID).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 2 {
		t.Fatalf("分配数应为 2: %d", cnt)
	}
	// 将 node-b 置为 missing=1，重算应删除该分配
	if _, err := st.DB().ExecContext(ctx, `UPDATE nodes SET missing=1 WHERE id = ?`, nodeB); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RecomputeCandidateSet(ctx); err != nil {
		t.Fatalf("重算失败: %v", err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM group_nodes WHERE group_id = ? AND node_id = ?`, groupID, nodeB).Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 0 {
		t.Fatalf("不可用节点分配应被删除: %d", cnt)
	}
}

func TestSetNodesRejectsNonCandidate(t *testing.T) {
	st, svc := newFullTestGroupService(t)
	ctx := context.Background()
	groupID, _, _ := seedCandidateSet(t, st)
	// 插入不在候选集的节点
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO nodes (source, name, instance_id, tag, protocol, host, port, enabled, allocatable, missing)
		 VALUES ('xray', 'node-other', (SELECT id FROM xray_instances LIMIT 1), 'other', 'vless', 'h', 443, 1, 1, 0)`)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if err := svc.SetNodes(ctx, groupID, []int64{id}); err == nil {
		t.Fatal("非候选集节点应拒绝")
	}
}

func TestSetDefaultQuota(t *testing.T) {
	st, svc := newFullTestGroupService(t)
	ctx := context.Background()
	groupID, _, _ := seedCandidateSet(t, st)
	q := 10.5
	if err := svc.SetDefaultQuota(ctx, groupID, &q); err != nil {
		t.Fatalf("设置配额失败: %v", err)
	}
	g, err := svc.Get(ctx, groupID)
	if err != nil {
		t.Fatal(err)
	}
	if g.DefaultQuota == nil || *g.DefaultQuota != 10.5 {
		t.Fatalf("配额未生效: %+v", g.DefaultQuota)
	}
	neg := -1.0
	if err := svc.SetDefaultQuota(ctx, groupID, &neg); err == nil {
		t.Fatal("负数配额应拒绝")
	}
}

func TestGroupDeleteNotifiesAffectedUsers(t *testing.T) {
	st, svc := newFullTestGroupService(t)
	ctx := context.Background()
	groupID, _, _ := seedCandidateSet(t, st)
	// 补默认组（seedCandidateSet 未创建）
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO groups (slug, name, is_default) VALUES ('group-default', '默认组', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO users (username, email, group_id, user_source, status) VALUES ('u1','u1@x.com',?, 'local', 'active')`, groupID); err != nil {
		t.Fatal(err)
	}
	var uid int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM users WHERE email='u1@x.com'`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	var got []int64
	svc.SetOnNodesChanged(func(_ context.Context, _ int64, userIDs []int64) { got = append(got, userIDs...) })
	if err := svc.Delete(ctx, groupID); err != nil {
		t.Fatalf("删除组失败: %v", err)
	}
	if len(got) != 1 || got[0] != uid {
		t.Fatalf("删除组应通知受影响用户: %+v", got)
	}
}

func TestSetNodesNotifiesAffectedUsers(t *testing.T) {
	st, svc := newFullTestGroupService(t)
	ctx := context.Background()
	groupID, nodeA, _ := seedCandidateSet(t, st)
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO groups (slug, name, is_default) VALUES ('group-default', '默认组', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO users (username, email, group_id, user_source, status) VALUES ('u1','u1@x.com',?, 'local', 'active')`, groupID); err != nil {
		t.Fatal(err)
	}
	var uid int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM users WHERE email='u1@x.com'`).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	var got []int64
	svc.SetOnNodesChanged(func(_ context.Context, _ int64, userIDs []int64) { got = append(got, userIDs...) })
	if err := svc.SetNodes(ctx, groupID, []int64{nodeA}); err != nil {
		t.Fatalf("设置节点失败: %v", err)
	}
	if len(got) != 1 || got[0] != uid {
		t.Fatalf("设置节点应通知受影响用户: %+v", got)
	}
}
