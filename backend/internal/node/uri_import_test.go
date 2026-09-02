package node

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestImportURIsBatch(t *testing.T) {
	svc, st, _ := newTestService(t)
	one := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:password1")) + "@a.example.com:8388#Alpha"
	two := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:password2")) + "@b.example.com:8388#Beta"
	text := "ss://" + one + "\nss://" + two + "\nnot-a-uri"
	results, err := svc.ImportURIs(context.Background(), text)
	if err != nil {
		t.Fatalf("ImportURIs 失败: %v", err)
	}
	var okCount, skipCount int
	for _, r := range results {
		if r.OK {
			okCount++
		} else {
			skipCount++
		}
	}
	if okCount != 2 || skipCount != 1 {
		t.Fatalf("导入回执异常: %+v", results)
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes WHERE source='manual'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("应导入 2 个节点，实际 %d", count)
	}
}

func TestImportURIsDuplicateSkipped(t *testing.T) {
	svc, st, _ := newTestService(t)
	one := base64.StdEncoding.EncodeToString([]byte("aes-256-gcm:password1")) + "@a.example.com:8388#Same"
	text := "ss://" + one + "\nss://" + one
	results, err := svc.ImportURIs(context.Background(), text)
	if err != nil {
		t.Fatalf("ImportURIs 失败: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("应有 2 条回执，实际 %d: %+v", len(results), results)
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes WHERE source='manual'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("重复名称应只导入 1 个，实际 %d", count)
	}
	if !strings.Contains(results[1].Reason, "重复") {
		t.Fatalf("第二条应回执重复: %+v", results[1])
	}
}

func TestImportURIsCanonicalizesTransportAliases(t *testing.T) {
	svc, st, _ := newTestService(t)
	text := "vless://11111111-2222-3333-4444-555555555555@example.com:443?type=ws&security=tls&sni=cdn.example.com&path=%2Fws&host=cdn.example.com#WSClean"
	results, err := svc.ImportURIs(context.Background(), text)
	if err != nil {
		t.Fatalf("ImportURIs 失败: %v", err)
	}
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("WS URI 应导入成功: %+v", results)
	}
	var raw string
	if err := st.DB().QueryRowContext(context.Background(),
		`SELECT protocol_json FROM nodes WHERE name='WSClean'`).Scan(&raw); err != nil {
		t.Fatalf("读取导入节点失败: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("解析导入节点失败: %v", err)
	}
	if _, ok := decoded["path"]; ok {
		t.Fatalf("导入后不应保留顶层 path: %+v", decoded)
	}
	if _, ok := decoded["host"]; ok {
		t.Fatalf("导入后不应保留顶层 host: %+v", decoded)
	}
	ws, ok := decoded["ws-opts"].(map[string]any)
	if !ok {
		t.Fatalf("导入后应保留规范 ws-opts: %+v", decoded)
	}
	if ws["path"] != "/ws" {
		t.Fatalf("ws-opts.path 未收敛: %+v", ws)
	}
	if headers, ok := ws["headers"].(map[string]any); !ok || headers["Host"] != "cdn.example.com" {
		t.Fatalf("ws-opts.headers 未收敛: %+v", ws)
	}
}

func TestImportURIsSkipsInvalidCombination(t *testing.T) {
	svc, st, _ := newTestService(t)
	text := "vless://11111111-2222-3333-4444-555555555555@example.com:443?type=xhttp&security=tls&path=%2Fx&mode=none#BadXHTTP"
	results, err := svc.ImportURIs(context.Background(), text)
	if err != nil {
		t.Fatalf("ImportURIs 失败: %v", err)
	}
	if len(results) != 1 || results[0].OK || !strings.Contains(results[0].Reason, "xhttp-opts.mode") {
		t.Fatalf("非法 XHTTP none 应逐行跳过并给出字段路径: %+v", results)
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM nodes WHERE source='manual'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("非法组合不应落库，实际 %d", count)
	}
}

func TestImportURIsRejectsUnknownTopLevelField(t *testing.T) {
	svc, st, _ := newTestService(t)
	text := "vless://11111111-2222-3333-4444-555555555555@example.com:443?type=tcp&mode=abc#UnknownField"
	results, err := svc.ImportURIs(context.Background(), text)
	if err != nil {
		t.Fatalf("ImportURIs 失败: %v", err)
	}
	if len(results) != 1 || results[0].OK || !strings.Contains(results[0].Reason, "mode") {
		t.Fatalf("未知顶层字段应逐行跳过: %+v", results)
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM nodes WHERE source='manual'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("未知顶层字段不应落库，实际 %d", count)
	}
}
