package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
)

func TestNodeUpdateRevisionConflictResponse(t *testing.T) {
	engine, st, cfg := newAssemblyTestEnv(t)
	lg := log.New("error", "console")
	nodeSvc := node.NewService(st, cfg, lg)
	noop := func(c *gin.Context) { c.Next() }
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, noop, noop)

	created, err := nodeSvc.CreateManual(context.Background(), node.CreateManualInput{
		Name: "API修订节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp",
		},
	})
	if err != nil {
		t.Fatalf("创建测试节点失败: %v", err)
	}
	body, err := json.Marshal(node.UpdateManualInput{
		Protocol: "vless", Host: "stale.example.com", Port: 443, BaseRevision: 0,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
	})
	if err != nil {
		t.Fatalf("序列化更新请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/nodes/%d", created.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("旧修订 API 状态码异常: %d, body=%s", w.Code, w.Body.String())
	}
	var conflict struct {
		Error           string `json:"error"`
		Code            string `json:"code"`
		CurrentRevision int64  `json:"current_revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &conflict); err != nil {
		t.Fatalf("解析修订冲突响应失败: %v", err)
	}
	if conflict.Code != "revision_conflict" || conflict.CurrentRevision != 1 || conflict.Error == "" {
		t.Fatalf("修订冲突响应异常: %+v", conflict)
	}

	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/nodes/%d", created.ID), nil)
	getW := httptest.NewRecorder()
	engine.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("节点详情 API 状态码异常: %d, body=%s", getW.Code, getW.Body.String())
	}
	var detail struct {
		Code int       `json:"code"`
		Data node.Node `json:"data"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &detail); err != nil {
		t.Fatalf("解析节点详情响应失败: %v", err)
	}
	if detail.Code != 0 || detail.Data.EditRevision != 1 || detail.Data.ID != created.ID {
		t.Fatalf("节点详情响应未返回当前修订: %+v", detail)
	}
}

func TestNodeCheckRouteUsesAdapterAndDoesNotWrite(t *testing.T) {
	engine, st, cfg := newAssemblyTestEnv(t)
	lg := log.New("error", "console")
	nodeSvc := node.NewService(st, cfg, lg)
	assemblySvc := assembly.NewService(st, cfg, lg)
	nodeSvc.SetCheckRenderer(assemblySvc.CheckNodeTarget)
	noop := func(c *gin.Context) { c.Next() }
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, noop, noop)

	request := node.CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "route-check-secret", "network": "ws", "tls": true,
			"ws-opts": map[string]any{"path": "/check"},
		},
		Targets: []string{"clash-yaml", "generic-subs"},
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("序列化节点检查请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("节点检查路由状态码异常: %d, body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Code int                `json:"code"`
		Data node.CheckResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析节点检查响应失败: %v", err)
	}
	if response.Code != 0 || response.Data.CheckID == "" || response.Data.CheckVersion != 1 {
		t.Fatalf("节点检查响应基本结构异常: %+v", response)
	}
	for _, target := range []string{"clash-yaml", "generic-subs"} {
		result, ok := response.Data.Targets[target]
		if !ok || result.Status != "ok" || result.Preview == nil {
			t.Fatalf("节点检查目标结果异常: target=%s result=%+v", target, result)
		}
	}
	if strings.Contains(w.Body.String(), "route-check-secret") {
		t.Fatalf("节点检查 HTTP 响应泄漏凭据: %s", w.Body.String())
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes`).Scan(&count); err != nil {
		t.Fatalf("读取检查后节点数量失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("节点检查不应写入数据库: count=%d", count)
	}
}

func TestNodeUpdateForbiddenReturns403(t *testing.T) {
	engine, st, cfg := newAssemblyTestEnv(t)
	lg := log.New("error", "console")
	nodeSvc := node.NewService(st, cfg, lg)
	noop := func(c *gin.Context) { c.Next() }
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, noop, noop)

	ctx := context.Background()
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO xray_instances (name, slug, api_addr) VALUES ('API实例','api-xray','https://example.com')`)
	if err != nil {
		t.Fatalf("插入 Xray 实例失败: %v", err)
	}
	instID, _ := res.LastInsertId()
	res, err = st.DB().ExecContext(ctx,
		`INSERT INTO nodes (source,name,instance_id,tag,protocol,host,port,protocol_json,allocatable,missing)
		 VALUES ('xray','api-xray-vless',?,'inbound','vless','example.com',443,'{}',1,0)`, instID)
	if err != nil {
		t.Fatalf("插入 Xray 节点失败: %v", err)
	}
	nodeID, _ := res.LastInsertId()

	body, err := json.Marshal(node.UpdateManualInput{
		Protocol: "vless", Host: "new.example.com", Port: 443, BaseRevision: 0,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
	})
	if err != nil {
		t.Fatalf("序列化更新请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/nodes/%d", nodeID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("禁止编辑 Xray 节点应返回 403，实际 %d, body=%s", w.Code, w.Body.String())
	}
}
