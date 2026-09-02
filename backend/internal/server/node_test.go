package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

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
