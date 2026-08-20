package xray

import (
	"context"
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
	vlessinbound "github.com/xtls/xray-core/proxy/vless/inbound"
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
	vlessCfg := &vlessinbound.Config{}
	return &handlercmd.ListInboundsResponse{
		Inbounds: []*core.InboundHandlerConfig{
			{
				Tag:              "in-vless",
				ReceiverSettings: serial.ToTypedMessage(rc),
				ProxySettings:    serial.ToTypedMessage(vlessCfg),
			},
		},
	}, nil
}

type fakeStatsServer struct {
	statscmd.UnimplementedStatsServiceServer
}

func (f *fakeStatsServer) QueryStats(ctx context.Context, req *statscmd.QueryStatsRequest) (*statscmd.QueryStatsResponse, error) {
	if req.GetPattern() == "" {
		return nil, errEmptyPattern
	}
	return &statscmd.QueryStatsResponse{}, nil
}

var (
	errAlreadyExists = &grpcError{msg: "user already exists."}
	errNotFound      = &grpcError{msg: "user not found."}
	errEmptyPattern  = &grpcError{msg: "pattern is empty"}
)

type grpcError struct{ msg string }

func (e *grpcError) Error() string { return e.msg }

func startFakeXray(t *testing.T) (addr string, handler *fakeHandlerServer) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	handler = newFakeHandlerServer()
	handlercmd.RegisterHandlerServiceServer(srv, handler)
	statscmd.RegisterStatsServiceServer(srv, &fakeStatsServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), handler
}

func TestClientFakeAddRemove(t *testing.T) {
	addr, handler := startFakeXray(t)
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
	addr, _ := startFakeXray(t)
	client, err := Dial(addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	resp, err := client.ListInbounds(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetInbounds()) != 1 || resp.GetInbounds()[0].GetTag() != "in-vless" {
		t.Fatalf("ListInbounds 响应异常: %+v", resp)
	}
	if !strings.Contains(resp.GetInbounds()[0].GetProxySettings().GetType(), "vless") {
		t.Fatalf("协议类型异常: %s", resp.GetInbounds()[0].GetProxySettings().GetType())
	}
}
