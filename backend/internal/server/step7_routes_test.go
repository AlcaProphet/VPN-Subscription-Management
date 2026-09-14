package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"vpn-sub/internal/pool"
)

func TestSourceStatusAndSnapshotAPIRawJSONContract(t *testing.T) {
	engine, st, svc := newPoolRoutesEnv(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "来源状态 API 池", []pool.SourceInput{{URL: "https://example.com/rules?token=RAW", SourceMode: pool.SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	sourceID := p.Sources[0].ID
	insertSnapshot := func(status, diag, stats, createdAt string) int64 {
		t.Helper()
		res, err := st.DB().ExecContext(ctx,
			`INSERT INTO pool_source_snapshots
			   (source_id, status, format, profile, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json, created_at)
			 VALUES (?, ?, 'typed-rule-text', 'common', 1, 1, 1, 0, 0, 0, ?, ?, ?)`,
			sourceID, status, diag, stats, createdAt)
		if err != nil {
			t.Fatalf("插入快照失败: %v", err)
		}
		id, _ := res.LastInsertId()
		return id
	}
	activeID := insertSnapshot("active", `[]`,
		`{"schema_version":1,"source_mode":"auto","detection":{"evidence_codes":["typed_rule_marker"],"recognition_required_percent":90},"rule_counts":[],"unclassified_rejected":0,"comparison":null,"decision":{"initial_status":"active","reason_codes":["first_success"]}}`,
		"2026-09-10 11:00:00")
	failedID := insertSnapshot("failed", `[{"line":0,"kind":"error","message":"HTTP 500","raw":""}]`,
		`{"schema_version":1,"source_mode":"auto","detection":{"evidence_codes":[],"recognition_required_percent":null},"rule_counts":[],"unclassified_rejected":0,"comparison":{"previous_active":null,"format_changed":false,"profile_changed":false,"accepted_drop_threshold_percent":70,"accepted_drop_triggered":false},"decision":{"initial_status":"failed","reason_codes":["http_status_error"]}}`,
		"2026-09-10 12:00:00")
	pendingID := insertSnapshot("pending", `[]`,
		`{"schema_version":1,"source_mode":"auto","detection":{"evidence_codes":["typed_rule_marker"],"recognition_required_percent":90},"rule_counts":[],"unclassified_rejected":0,"comparison":{"previous_active":{"snapshot_id":`+strconv.FormatInt(activeID, 10)+`,"format":"typed-rule-text","profile":"common","accepted":1},"format_changed":true,"profile_changed":false,"accepted_drop_threshold_percent":70,"accepted_drop_triggered":false},"decision":{"initial_status":"pending","reason_codes":["format_changed"]}}`,
		"2026-09-10 13:00:00")
	if _, err := st.DB().ExecContext(ctx,
		`UPDATE rule_pool_sources SET active_snapshot_id=?, pending_snapshot_id=? WHERE id=?`, activeID, pendingID, sourceID); err != nil {
		t.Fatalf("更新来源指针失败: %v", err)
	}

	statusPath := "/api/admin/pools/" + strconv.FormatInt(p.ID, 10) + "/sources/status"
	req := httptest.NewRequest(http.MethodGet, statusPath, nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sources/status 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	statusData := decodeDataObject(t, w.Body.Bytes())
	list, _ := statusData["list"].([]any)
	if len(list) != 1 {
		t.Fatalf("来源状态列表数量错误: %+v", list)
	}
	status, _ := list[0].(map[string]any)
	if _, ok := status["url"]; ok {
		t.Fatalf("SourceStatus 不得回传原始 url: %+v", status)
	}
	if got, _ := status["display_url"].(string); got != "https://example.com/rules?token=***" {
		t.Fatalf("display_url 错误: %q", got)
	}
	if status["never_synced"] != false || status["source_mode"] != "auto" {
		t.Fatalf("来源状态顶层字段错误: %+v", status)
	}
	for key, wantID := range map[string]int64{"latest_attempt": pendingID, "active": activeID, "pending": pendingID, "latest_failed": failedID} {
		snap, _ := status[key].(map[string]any)
		if snap == nil || int64(snap["id"].(float64)) != wantID {
			t.Fatalf("%s 字段/摘要形状错误: %+v", key, snap)
		}
		if snap["diagnostics"] == nil {
			t.Fatalf("%s.diagnostics 不得为 null: %+v", key, snap)
		}
	}
	if active, _ := status["active"].(map[string]any); active["error"] != "" {
		t.Fatalf("active.error 应显式为空串: %+v", active)
	}
	if failed, _ := status["latest_failed"].(map[string]any); failed["error"] != "HTTP 500" {
		t.Fatalf("latest_failed.error 应从诊断还原: %+v", failed)
	}

	base := "/api/admin/pools/" + strconv.FormatInt(p.ID, 10) + "/sources/" + strconv.FormatInt(sourceID, 10) + "/snapshots"
	req = httptest.NewRequest(http.MethodGet, base+"?page=1&page_size=2", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("snapshots 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	snapshotData := decodeDataObject(t, w.Body.Bytes())
	if total, _ := snapshotData["total"].(float64); int64(total) != 3 {
		t.Fatalf("snapshots total 错误: %+v", snapshotData)
	}
	snapList, _ := snapshotData["list"].([]any)
	if len(snapList) != 2 {
		t.Fatalf("snapshots 分页长度错误: %+v", snapList)
	}
	first, _ := snapList[0].(map[string]any)
	if int64(first["id"].(float64)) != pendingID {
		t.Fatalf("snapshots 应按 created_at DESC, id DESC: %+v", first)
	}
	if first["created_at"] == nil || first["activated_at"] != nil || first["error"] != "" {
		t.Fatalf("snapshot wire null/空串合同错误: %+v", first)
	}

	// pool/source 归属与非法 ID。
	other, err := svc.Create(ctx, "其他池", nil, false, "04:00")
	if err != nil {
		t.Fatalf("创建其他池失败: %v", err)
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/pools/"+strconv.FormatInt(other.ID, 10)+"/sources/"+strconv.FormatInt(sourceID, 10)+"/snapshots", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("跨池来源历史应 404，实际 %d body=%s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/admin/pools/"+strconv.FormatInt(p.ID, 10)+"/sources/abc/snapshots", nil)
	w = httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("非法 sourceId 应 400，实际 %d body=%s", w.Code, w.Body.String())
	}
}

func decodeDataObject(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var resp struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("解析响应外层失败: %v body=%s", err, body)
	}
	var data map[string]any
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析响应 data 失败: %v body=%s", err, body)
	}
	return data
}
