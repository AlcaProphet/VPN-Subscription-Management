package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/user"
	"vpn-sub/migrations"
)

func newOverviewTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dataDir := t.TempDir()
	st, err := store.Open(dataDir, "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background(), migrations.FS); err != nil {
		t.Fatalf("执行完整迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	users := user.NewService(st, cfg, log.New("error", "console"))
	streamSvc := log.NewStreamService(log.NewRingBuffer(), log.New("error", "console"))
	srv, err := New(st, cfg, users, log.New("error", "console"), "dev", mustPolicy(t, "off"), "0", dataDir, streamSvc)
	if err != nil {
		t.Fatalf("装配 server 失败: %v", err)
	}
	return srv
}

type overviewResponse struct {
	Code int `json:"code"`
	Data struct {
		Status struct {
			AppMode      string `json:"app_mode"`
			AdvancedMode bool   `json:"advanced_mode"`
			Emergency    bool   `json:"emergency"`
		} `json:"status"`
		Counts    overviewCounts `json:"counts"`
		Checklist []struct {
			Key    string `json:"key"`
			Done   bool   `json:"done"`
			Manual bool   `json:"manual"`
		} `json:"checklist"`
		Recent struct {
			PendingUsers []struct {
				Username string `json:"username"`
			} `json:"pending_users"`
			AccessLogs []struct {
				ID int64 `json:"id"`
			} `json:"access_logs"`
		} `json:"recent"`
	} `json:"data"`
}

func getOverview(t *testing.T, srv *Server, token string) overviewResponse {
	t.Helper()
	w := profileReq(t, srv, http.MethodGet, "/api/admin/overview", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("获取概览失败: %d %s", w.Code, w.Body.String())
	}
	var resp overviewResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析概览响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("概览业务码异常: %d", resp.Code)
	}
	return resp
}

func TestOverviewEndpointAggregatesStableData(t *testing.T) {
	srv := newOverviewTestServer(t)
	token := regUser(t, srv, "admin", "admin@example.com", "password123")
	if w := profileReq(t, srv, http.MethodGet, "/api/admin/overview", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("无会话访问概览应 401: %d", w.Code)
	}

	empty := getOverview(t, srv, token)
	if empty.Data.Status.AppMode != "dev" || empty.Data.Status.AdvancedMode || empty.Data.Status.Emergency {
		t.Errorf("空数据状态异常: %+v", empty.Data.Status)
	}
	if empty.Data.Counts.Users != 1 || empty.Data.Counts.Platforms != 0 || empty.Data.Counts.UsableNodes != 0 {
		t.Errorf("空数据计数异常: %+v", empty.Data.Counts)
	}
	if len(empty.Data.Checklist) != 5 || empty.Data.Checklist[4].Key != "member_check" || !empty.Data.Checklist[4].Manual || empty.Data.Checklist[4].Done {
		t.Errorf("空数据 checklist 异常: %+v", empty.Data.Checklist)
	}
	baseProxyGroupCount := empty.Data.Counts.ProxyGroups

	ctx := context.Background()
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO platforms (slug, name, product_type) VALUES ('platform-test', '测试平台', 'yaml')`); err != nil {
		t.Fatalf("插入平台失败: %v", err)
	}
	var platformID int64
	if err := srv.store.DB().QueryRowContext(ctx, `SELECT id FROM platforms WHERE slug = 'platform-test'`).Scan(&platformID); err != nil {
		t.Fatalf("查询平台 ID 失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO subscriptions (slug, name, platform_id, product_type, current_version) VALUES ('subscription-test', '测试订阅', ?, 'yaml', 1)`, platformID); err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO xray_instances (name, slug, api_addr) VALUES ('测试实例', 'xray-test', '127.0.0.1:10085')`); err != nil {
		t.Fatalf("插入 Xray 实例失败: %v", err)
	}
	var instanceID int64
	if err := srv.store.DB().QueryRowContext(ctx, `SELECT id FROM xray_instances WHERE slug = 'xray-test'`).Scan(&instanceID); err != nil {
		t.Fatalf("查询 Xray 实例 ID 失败: %v", err)
	}
	nodeRows := []struct {
		Source      string
		Name        string
		InstanceID  any
		Tag         any
		Enabled     int
		Allocatable int
		Missing     int
	}{
		{"manual", "manual-usable", nil, nil, 1, 0, 0},
		{"manual", "manual-disabled", nil, nil, 0, 1, 0},
		{"xray", "xray-unallocatable", instanceID, "inbound-a", 1, 0, 0},
		{"xray", "xray-missing", instanceID, "inbound-b", 1, 1, 1},
		{"xray", "xray-usable", instanceID, "inbound-c", 1, 1, 0},
	}
	for _, n := range nodeRows {
		if _, err := srv.store.DB().ExecContext(ctx,
			`INSERT INTO nodes (source, name, instance_id, tag, protocol, host, port, protocol_json, enabled, allocatable, missing)
			 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			n.Source, n.Name, n.InstanceID, n.Tag, "shadowsocks", "127.0.0.1", 8388, "{}", n.Enabled, n.Allocatable, n.Missing); err != nil {
			t.Fatalf("插入节点 %s 失败: %v", n.Name, err)
		}
	}
	if _, err := srv.store.DB().ExecContext(ctx, `INSERT INTO rules (slug, name) VALUES ('rule-test', '测试规则')`); err != nil {
		t.Fatalf("插入规则失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `INSERT INTO share_subscriptions (slug, name) VALUES ('share-test', '测试分享')`); err != nil {
		t.Fatalf("插入分享失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `INSERT INTO rule_pools (name) VALUES ('测试素材池')`); err != nil {
		t.Fatalf("插入素材池失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO proxy_groups (name, type, definition_json) VALUES ('测试代理组', 'custom', '{}')`); err != nil {
		t.Fatalf("插入代理组失败: %v", err)
	}
	createdAt := []time.Time{
		time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.January, 3, 10, 0, 0, 0, time.UTC),
	}
	for i, u := range []struct {
		Username string
		Status   string
	}{{"active-user", "active"}, {"pending-old", "pending"}, {"pending-new", "pending"}} {
		if _, err := srv.store.DB().ExecContext(ctx,
			`INSERT INTO users (username, email, role, user_source, status, created_at) VALUES (?,?,?,?,?,?)`,
			u.Username, u.Username+"@example.com", "user", "selfreg", u.Status, createdAt[i]); err != nil {
			t.Fatalf("插入用户 %s 失败: %v", u.Username, err)
		}
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO xray_ext_accounts (name, email) VALUES ('测试独立账号', 'ext@example.com')`); err != nil {
		t.Fatalf("插入独立账号失败: %v", err)
	}
	accessRows := []string{"2026-01-01 10:00:00", "2026-01-02 10:00:00"}
	for _, created := range accessRows {
		if _, err := srv.store.DB().ExecContext(ctx,
			`INSERT INTO access_logs (ip, download_type, resource_slug, status, created_at) VALUES (?,?,?,?,?)`,
			"127.0.0.1", "subscription", "subscription-test", "success", created); err != nil {
			t.Fatalf("插入访问日志失败: %v", err)
		}
	}

	resp := getOverview(t, srv, token)
	counts := resp.Data.Counts
	if counts.Platforms != 1 || counts.Subscriptions != 1 || counts.Nodes != 5 || counts.UsableNodes != 2 ||
		counts.ManualNodes != 2 || counts.XrayNodes != 3 || counts.Rules != 1 || counts.Shares != 1 ||
		counts.Users != 4 || counts.PendingUsers != 2 || counts.Pools != 1 || counts.ProxyGroups != baseProxyGroupCount+1 {
		t.Errorf("概览计数异常: %+v", counts)
	}
	if counts.XrayInstances != 0 || counts.ExtAccounts != 0 {
		t.Errorf("高级模式关闭时 Xray 计数应为 0: %+v", counts)
	}
	if !resp.Data.Checklist[0].Done || !resp.Data.Checklist[1].Done || !resp.Data.Checklist[2].Done || !resp.Data.Checklist[3].Done {
		t.Errorf("基础 checklist 应已完成: %+v", resp.Data.Checklist)
	}
	if len(resp.Data.Recent.PendingUsers) != 2 || resp.Data.Recent.PendingUsers[0].Username != "pending-new" ||
		len(resp.Data.Recent.AccessLogs) != 2 || resp.Data.Recent.AccessLogs[0].ID <= resp.Data.Recent.AccessLogs[1].ID {
		t.Errorf("动态摘要排序异常: %+v", resp.Data.Recent)
	}
	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatalf("开启高级模式失败: %v", err)
	}
	advanced := getOverview(t, srv, token)
	if !advanced.Data.Status.AdvancedMode || advanced.Data.Counts.XrayInstances != 1 || advanced.Data.Counts.ExtAccounts != 1 {
		t.Errorf("高级模式概览计数异常: status=%+v counts=%+v", advanced.Data.Status, advanced.Data.Counts)
	}
}
