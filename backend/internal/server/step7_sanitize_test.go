package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"vpn-sub/internal/pool"
	"vpn-sub/internal/redact"
)

func TestExistingSyncAPIsRedactRawJSONAndKeepEditableURLs(t *testing.T) {
	engine, st, svc := newPoolRoutesEnv(t)
	ctx := context.Background()
	rawURL := "https://example.com/rules?token=RAW_SECRET"
	p, err := svc.Create(ctx, "raw JSON 清洗池", []pool.SourceInput{{URL: rawURL, SourceMode: pool.SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建素材池失败: %v", err)
	}
	if _, err := st.DB().ExecContext(ctx, `UPDATE rule_pools SET sync_error=? WHERE id=?`, "pool token=SECRET", p.ID); err != nil {
		t.Fatalf("写入池错误失败: %v", err)
	}
	rawPer := `[{"url":"https://example.com/rules?token=SECRET","source_id":` + strconv.FormatInt(p.Sources[0].ID, 10) + `,"ok":false,"accepted":0,"error":"download token=SECRET"}]`
	longTaskErr := "task token=SECRET " + strings.Repeat("x", 300)
	if _, err := st.DB().ExecContext(ctx,
		`INSERT INTO pool_sync_tasks (pool_id,status,per_url_json,error,started_at,finished_at)
		 VALUES (?, 'failed', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, p.ID, rawPer, longTaskErr); err != nil {
		t.Fatalf("插入同步任务失败: %v", err)
	}

	for _, path := range []string{
		"/api/admin/pools/" + strconv.FormatInt(p.ID, 10) + "/sync/status",
		"/api/admin/pools/" + strconv.FormatInt(p.ID, 10) + "/sync/tasks",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 状态码异常: %d body=%s", path, w.Code, w.Body.String())
		}
		body := w.Body.String()
		if strings.Contains(body, "SECRET") {
			t.Fatalf("%s 读时脱敏泄漏敏感值: %s", path, body)
		}
		if !strings.Contains(body, "token=***") {
			t.Fatalf("%s 应保留脱敏标记: %s", path, body)
		}
		task := decodeFirstSyncTask(t, w.Body.Bytes())
		perURL, _ := task["per_url"].([]any)
		if len(perURL) != 1 {
			t.Fatalf("%s per_url 数量异常: %+v", path, task)
		}
		per, _ := perURL[0].(map[string]any)
		urlText, _ := per["url"].(string)
		errText, _ := per["error"].(string)
		if !strings.Contains(urlText, "token=***") || strings.Contains(errText, "SECRET") {
			t.Fatalf("%s per_url 清洗异常: %+v", path, per)
		}
		taskErr, _ := task["error"].(string)
		if strings.Contains(taskErr, "SECRET") || utf8.RuneCountInString(taskErr) > redact.MaxFieldRunes {
			t.Fatalf("%s task.error 清洗/限额失败: %q (%d rune)", path, taskErr, utf8.RuneCountInString(taskErr))
		}
	}

	// 池列表：sync_error 清洗，但编辑用 sources[].url / urls[] 必须保留原始值。
	req := httptest.NewRequest(http.MethodGet, "/api/admin/pools", nil)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("/api/admin/pools 状态码异常: %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int             `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析池列表响应失败: %v", err)
	}
	var data struct {
		List []struct {
			ID      int64    `json:"id"`
			URLs    []string `json:"urls"`
			Sources []struct {
				URL string `json:"url"`
			} `json:"sources"`
			SyncError string `json:"sync_error"`
		} `json:"list"`
		Total int64 `json:"total"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("解析池列表 data 失败: %v", err)
	}
	if len(data.List) != 1 || data.List[0].ID != p.ID {
		t.Fatalf("池列表异常: %+v", data.List)
	}
	got := data.List[0]
	if strings.Contains(got.SyncError, "SECRET") {
		t.Fatalf("Pool.sync_error 读时清洗失败: %q", got.SyncError)
	}
	if len(got.Sources) != 1 || got.Sources[0].URL != rawURL {
		t.Fatalf("sources[].url 应保留原始编辑值: %+v", got.Sources)
	}
	if len(got.URLs) != 1 || got.URLs[0] != rawURL {
		t.Fatalf("urls[] 应保留原始编辑值: %+v", got.URLs)
	}
}

func decodeFirstSyncTask(t *testing.T, body []byte) map[string]any {
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
	if list, ok := data["list"].([]any); ok {
		if len(list) == 0 {
			t.Fatal("同步任务列表为空")
		}
		task, _ := list[0].(map[string]any)
		return task
	}
	return data
}
