// Package xray 提供 Xray-core gRPC 客户端封装与账号构造（高级模式）。
package xray

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	handlercmd "github.com/xtls/xray-core/app/proxyman/command"
	statscmd "github.com/xtls/xray-core/app/stats/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
)

const (
	// DialTimeout 首个 RPC 探测使用的快速失败超时（grpc.NewClient 为懒建连，不阻塞拨号）。
	DialTimeout = 10 * time.Second
	// RPCTimeout 普通 gRPC 调用的单次超时。
	RPCTimeout = 30 * time.Second
)

// Client 是单个 Xray 实例的 gRPC 客户端。所有方法均持互斥锁串行执行，
// 满足 Xray-Core-API.md §11.4 的保守并发结论。
type Client struct {
	conn    *grpc.ClientConn
	handler handlercmd.HandlerServiceClient
	stats   statscmd.StatsServiceClient
	mu      sync.Mutex
}

// Dial 校验 TCP 地址并创建懒连接客户端。grpc.NewClient 不实际拨号，
// 快速失败由调用方在首个 RPC 上携带短 deadline 达成。
func Dial(apiAddr string) (*Client, error) {
	if err := ValidateAddr(apiAddr); err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(apiAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("创建 xray gRPC 客户端失败: %w", err)
	}
	return &Client{
		conn:    conn,
		handler: handlercmd.NewHandlerServiceClient(conn),
		stats:   statscmd.NewStatsServiceClient(conn),
	}, nil
}

// Close 关闭底层连接。
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// ValidateAddr 校验 api_addr 为非空 TCP 地址（host:port）。
func ValidateAddr(apiAddr string) error {
	if apiAddr == "" {
		return errors.New("api_addr 不能为空")
	}
	host, port, err := net.SplitHostPort(apiAddr)
	if err != nil {
		return fmt.Errorf("api_addr 必须是 host:port 格式: %w", err)
	}
	if host == "" || port == "" {
		return errors.New("api_addr 缺少 host 或 port")
	}
	return nil
}

// AddUser 向指定 inbound 添加用户；`already exists.` 视为幂等成功。
func (c *Client) AddUser(ctx context.Context, tag string, u *protocol.User) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	op := serial.ToTypedMessage(&handlercmd.AddUserOperation{User: u})
	_, err := c.handler.AlterInbound(ctx, &handlercmd.AlterInboundRequest{Tag: tag, Operation: op})
	if err == nil || strings.Contains(err.Error(), "already exists.") {
		return nil
	}
	return &OpError{Op: "AddUser", Instance: c.addr(), Tag: tag, Err: err}
}

// RemoveUser 从指定 inbound 移除用户；`not found.` 视为幂等成功。
func (c *Client) RemoveUser(ctx context.Context, tag, email string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	op := serial.ToTypedMessage(&handlercmd.RemoveUserOperation{Email: email})
	_, err := c.handler.AlterInbound(ctx, &handlercmd.AlterInboundRequest{Tag: tag, Operation: op})
	if err == nil || strings.Contains(err.Error(), "not found.") {
		return nil
	}
	return &OpError{Op: "RemoveUser", Instance: c.addr(), Tag: tag, Err: err}
}

// ListInbounds 透传入站列表。
func (c *Client) ListInbounds(ctx context.Context) (*handlercmd.ListInboundsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, err := c.handler.ListInbounds(ctx, &handlercmd.ListInboundsRequest{})
	if err != nil {
		return nil, &OpError{Op: "ListInbounds", Instance: c.addr(), Err: err}
	}
	return resp, nil
}

// GetInboundUsers 查询指定 inbound 的用户列表。
func (c *Client) GetInboundUsers(ctx context.Context, tag, email string) (*handlercmd.GetInboundUserResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, err := c.handler.GetInboundUsers(ctx, &handlercmd.GetInboundUserRequest{Tag: tag, Email: email})
	if err != nil {
		return nil, &OpError{Op: "GetInboundUsers", Instance: c.addr(), Tag: tag, Err: err}
	}
	return resp, nil
}

// QueryStats 查询并（可选）重置统计计数器。
func (c *Client) QueryStats(ctx context.Context, pattern string, reset bool) (*statscmd.QueryStatsResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resp, err := c.stats.QueryStats(ctx, &statscmd.QueryStatsRequest{Pattern: pattern, Reset_: reset})
	if err != nil {
		return nil, &OpError{Op: "QueryStats", Instance: c.addr(), Err: err}
	}
	return resp, nil
}

func (c *Client) addr() string {
	if c == nil || c.conn == nil {
		return ""
	}
	return c.conn.Target()
}
