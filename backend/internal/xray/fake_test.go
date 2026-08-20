package xray

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"

	"google.golang.org/grpc"

	proxyman "github.com/xtls/xray-core/app/proxyman"
	handlercmd "github.com/xtls/xray-core/app/proxyman/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/core"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	vlessinbound "github.com/xtls/xray-core/proxy/vless/inbound"

	"vpn-sub/internal/config"
	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
	"vpn-sub/migrations"
)

type fakeHandlerServer struct {
	handlercmd.UnimplementedHandlerServiceServer
	mu    sync.Mutex
	users map[string]map[string]bool
}

func newFakeHandlerServer() *fakeHandlerServer {
	return &fakeHandlerServer{users: map[string]map[string]bool{}}
}

func (f *fakeHandlerServer) AlterInbound(ctx context.Context, req *handlercmd.AlterInboundRequest) (*handlercmd.AlterInboundResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	inst, err := req.GetOperation().GetInstance()
	if err != nil {
		return nil, err
	}
	switch op := inst.(type) {
	case *handlercmd.AddUserOperation:
		email := op.GetUser().GetEmail()
		if f.users[req.GetTag()] == nil {
			f.users[req.GetTag()] = map[string]bool{}
		}
		if f.users[req.GetTag()][email] {
			return nil, errAlreadyExists
		}
		f.users[req.GetTag()][email] = true
	case *handlercmd.RemoveUserOperation:
		if f.users[req.GetTag()] == nil || !f.users[req.GetTag()][op.GetEmail()] {
			return nil, errNotFound
		}
		delete(f.users[req.GetTag()], op.GetEmail())
	}
	return &handlercmd.AlterInboundResponse{}, nil
}

func (f *fakeHandlerServer) ListInbounds(ctx context.Context, req *handlercmd.ListInboundsRequest) (*handlercmd.ListInboundsResponse, error) {
	rc := &proxyman.ReceiverConfig{
		PortList: &xnet.PortList{Range: []*xnet.PortRange{{From: 443, To: 443}}},
	}
	rc2 := &proxyman.ReceiverConfig{
		PortList: &xnet.PortList{Range: []*xnet.PortRange{{From: 8388, To: 8388}}},
	}
	vlessCfg := &vlessinbound.Config{}
	ssCfg := &shadowsocks.ServerConfig{}
	return &handlercmd.ListInboundsResponse{
		Inbounds: []*core.InboundHandlerConfig{
			{
				Tag:              "in-vless",
				ReceiverSettings: serial.ToTypedMessage(rc),
				ProxySettings:    serial.ToTypedMessage(vlessCfg),
			},
			{
				Tag:              "in-ss",
				ReceiverSettings: serial.ToTypedMessage(rc2),
				ProxySettings:    serial.ToTypedMessage(ssCfg),
			},
		},
	}, nil
}

type fakeStatsServer struct {
	statscmd.UnimplementedStatsServiceServer
	mu       sync.Mutex
	counters map[string]int64
}

func newFakeStatsServer() *fakeStatsServer {
	return &fakeStatsServer{counters: map[string]int64{}}
}

func (f *fakeStatsServer) QueryStats(ctx context.Context, req *statscmd.QueryStatsRequest) (*statscmd.QueryStatsResponse, error) {
	if req.GetPattern() == "" {
		return nil, errEmptyPattern
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	resp := &statscmd.QueryStatsResponse{}
	for name, value := range f.counters {
		if strings.HasPrefix(name, req.GetPattern()) {
			resp.Stat = append(resp.Stat, &statscmd.Stat{Name: name, Value: value})
			if req.GetReset_() {
				f.counters[name] = 0
			}
		}
	}
	return resp, nil
}

var (
	errAlreadyExists = &grpcError{msg: "user already exists."}
	errNotFound      = &grpcError{msg: "user not found."}
	errEmptyPattern  = &grpcError{msg: "pattern is empty"}
)

type grpcError struct{ msg string }

func (e *grpcError) Error() string { return e.msg }

func startFakeXray(t *testing.T) (addr string, handler *fakeHandlerServer, stats *fakeStatsServer) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	handler = newFakeHandlerServer()
	stats = newFakeStatsServer()
	handlercmd.RegisterHandlerServiceServer(srv, handler)
	statscmd.RegisterStatsServiceServer(srv, stats)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), handler, stats
}

func TestClientFakeAddRemove(t *testing.T) {
	addr, handler, _ := startFakeXray(t)
	client, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx := context.Background()
	u, err := BuildUser(7, "uuid", "secret", NodeView{Protocol: "vless"})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.AddUser(ctx, "in-vless", u); err != nil {
		t.Fatalf("AddUser 失败: %v", err)
	}
	// 幂等：already exists 视为成功
	if err := client.AddUser(ctx, "in-vless", u); err != nil {
		t.Fatalf("AddUser 幂等失败: %v", err)
	}
	if err := client.RemoveUser(ctx, "in-vless", UserEmail(7)); err != nil {
		t.Fatalf("RemoveUser 失败: %v", err)
	}
	// 幂等：not found 视为成功
	if err := client.RemoveUser(ctx, "in-vless", UserEmail(7)); err != nil {
		t.Fatalf("RemoveUser 幂等失败: %v", err)
	}
	if _, err := client.ListInbounds(ctx); err != nil {
		t.Fatalf("ListInbounds 失败: %v", err)
	}
	if _, err := client.QueryStats(ctx, "user>>>user-7@vpn.local>>>traffic", true); err != nil {
		t.Fatalf("QueryStats 失败: %v", err)
	}
	_ = handler
}

func TestClientFakeListInboundsProtocol(t *testing.T) {
	addr, _, _ := startFakeXray(t)
	client, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	resp, err := client.ListInbounds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetInbounds()) != 2 || resp.GetInbounds()[0].GetTag() != "in-vless" || resp.GetInbounds()[1].GetTag() != "in-ss" {
		t.Fatalf("ListInbounds 响应异常: %+v", resp)
	}
	if !strings.Contains(resp.GetInbounds()[0].GetProxySettings().GetType(), "vless") {
		t.Fatalf("协议类型异常: %s", resp.GetInbounds()[0].GetProxySettings().GetType())
	}
	if !strings.Contains(resp.GetInbounds()[1].GetProxySettings().GetType(), "shadowsocks") {
		t.Fatalf("协议类型异常: %s", resp.GetInbounds()[1].GetProxySettings().GetType())
	}
}

func TestBuild6EndToEndFakeXray(t *testing.T) {
	addr, handler, stats := startFakeXray(t)
	ctx := context.Background()

	st, err := store.Open(t.TempDir(), "test.db")
	if err != nil {
		t.Fatalf("打开测试库失败: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx, migrations.FS); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	cfg := config.NewService(st, log.New("error", "console"))
	if err := cfg.Set(ctx, config.KeySigningKey, "0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatal(err)
	}

	// 用户与默认组
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO groups (slug, name, is_default) VALUES ('group-default', '默认组', 1)`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO users (username, email, role, user_source, status, group_id) VALUES ('u1','u1@x.com','user','local','active',(SELECT id FROM groups WHERE slug='group-default'))`); err != nil {
		t.Fatal(err)
	}
	var userID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM users WHERE email='u1@x.com'`).Scan(&userID); err != nil {
		t.Fatal(err)
	}

	taskReg := tasks.NewRegistry()
	instSvc := NewInstanceService(st, log.New("error", "console"), taskReg)
	inst, err := instSvc.Create(ctx, "fake", addr, "", true)
	if err != nil {
		t.Fatalf("创建实例失败: %v", err)
	}
	detect, err := instSvc.DetectNodes(ctx, inst.ID)
	if err != nil {
		t.Fatalf("检测节点失败: %v", err)
	}
	if detect.Added != 2 || len(detect.AddedNodes) != 2 {
		t.Fatalf("检测应新增 2 个节点: %+v", detect)
	}

	var vlessID, ssID int64
	var vlessName, ssName string
	if err := st.DB().QueryRowContext(ctx, `SELECT id, name FROM nodes WHERE tag='in-vless'`).Scan(&vlessID, &vlessName); err != nil {
		t.Fatal(err)
	}
	if err := st.DB().QueryRowContext(ctx, `SELECT id, name FROM nodes WHERE tag='in-ss'`).Scan(&ssID, &ssName); err != nil {
		t.Fatal(err)
	}

	// 平台/订阅/版本/蓝图（候选集包含两个节点）
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO platforms (slug, name) VALUES ('plat','平台')`); err != nil {
		t.Fatal(err)
	}
	var platformID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM platforms WHERE slug='plat'`).Scan(&platformID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO subscriptions (slug, name, platform_id, current_version) VALUES ('sub','订阅',?,1)`, platformID); err != nil {
		t.Fatal(err)
	}
	var subID int64
	if err := st.DB().QueryRowContext(ctx, `SELECT id FROM subscriptions WHERE slug='sub'`).Scan(&subID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO versions (owner_type, owner_id, version_no, file_path) VALUES ('subscription',?,1,'x.yaml')`, subID); err != nil {
		t.Fatal(err)
	}
	sel := fmt.Sprintf(`{"xray_candidates":["%s","%s"]}`, vlessName, ssName)
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO assembly_blueprints (version_id, target_syntax, selection_json)
		 VALUES ((SELECT id FROM versions WHERE owner_type='subscription' AND owner_id=? AND version_no=1), 'clash-yaml', ?)`, subID, sel); err != nil {
		t.Fatal(err)
	}

	// 组分配两个节点
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO group_nodes (group_id, node_id, sort_order) VALUES ((SELECT id FROM groups WHERE slug='group-default'), ?, 0)`, vlessID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.DB().ExecContext(ctx, `INSERT INTO group_nodes (group_id, node_id, sort_order) VALUES ((SELECT id FROM groups WHERE slug='group-default'), ?, 1)`, ssID); err != nil {
		t.Fatal(err)
	}

	creds := NewCredentialService(st, cfg)
	syncSvc := NewSyncService(st, cfg, creds, instSvc, taskReg, log.New("error", "console"))
	synced, failed, err := syncSvc.PushUser(ctx, userID)
	if err != nil {
		t.Fatalf("PushUser 失败: %v", err)
	}
	if synced != 2 || failed != 0 {
		t.Fatalf("PushUser 计数异常 synced=%d failed=%d", synced, failed)
	}
	handler.mu.Lock()
	if len(handler.users["in-vless"]) != 1 || len(handler.users["in-ss"]) != 1 {
		handler.mu.Unlock()
		t.Fatalf("fake 应收到两个 AddUser: %+v", handler.users)
	}
	handler.mu.Unlock()

	// 移除 ss 节点后 ReconcileUser 应精确 RemoveUser
	if _, err := st.DB().ExecContext(ctx, `DELETE FROM group_nodes WHERE node_id = ?`, ssID); err != nil {
		t.Fatal(err)
	}
	if err := syncSvc.ReconcileUser(ctx, userID); err != nil {
		t.Fatalf("ReconcileUser 失败: %v", err)
	}
	handler.mu.Lock()
	if len(handler.users["in-ss"]) != 0 || len(handler.users["in-vless"]) != 1 {
		handler.mu.Unlock()
		t.Fatalf("移除后 fake 状态异常: %+v", handler.users)
	}
	handler.mu.Unlock()

	// 流量采集：设置 fake 计数器并执行 CollectInstance
	stats.mu.Lock()
	stats.counters["user>>>user-1@vpn.local>>>traffic>>>uplink"] = 100
	stats.counters["user>>>user-1@vpn.local>>>traffic>>>downlink"] = 200
	stats.mu.Unlock()
	cur, err := instSvc.Get(ctx, inst.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := instSvc.CollectInstance(ctx, *cur); err != nil {
		t.Fatalf("CollectInstance 失败: %v", err)
	}
	var up, down int64
	if err := st.DB().QueryRowContext(ctx, `SELECT uplink, downlink FROM traffic_records WHERE user_id = ? AND ym = ?`, userID, currentYM()).Scan(&up, &down); err != nil {
		t.Fatalf("查询流量失败: %v", err)
	}
	if up != 100 || down != 200 {
		t.Fatalf("流量累加异常 up=%d down=%d", up, down)
	}
}
