package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// seedPlatformWithVersion 播种平台 + 平台唯一订阅 + 1 个激活版本，返回 (platformID, subID)
func seedPlatformWithVersion(t *testing.T, srv *Server, productType string) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	res, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO platforms (slug, name, product_type) VALUES (?, '测试平台', ?)`, "platform-"+productType, productType)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	plat, _ := res.LastInsertId()
	res, err = srv.store.DB().ExecContext(ctx,
		`INSERT INTO subscriptions (slug, name, platform_id) VALUES (?, '订阅条目', ?)`, "sub-"+productType, plat)
	if err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}
	sub, _ := res.LastInsertId()
	dir := filepath.Join(srv.store.DBPath(), "..", "contents", "subscription", pathInt(sub))
	_ = dir
	// 版本文件写入内容根目录（dataDir 为 DBPath 的父目录）
	root := filepath.Dir(srv.store.DBPath())
	relDir := filepath.Join(root, "contents", "subscription", pathInt(sub))
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		t.Fatalf("建版本目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(relDir, "v1"), []byte("proxies: []\n"), 0o644); err != nil {
		t.Fatalf("写版本文件失败: %v", err)
	}
	rel := filepath.Join("subscription", pathInt(sub), "v1")
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path, file_name) VALUES ('subscription', ?, 1, ?, 'sub.yaml')`,
		sub, rel); err != nil {
		t.Fatalf("插入版本失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`UPDATE subscriptions SET current_version = 1 WHERE id = ?`, sub); err != nil {
		t.Fatalf("设置当前版本失败: %v", err)
	}
	return plat, sub
}

func pathInt(v int64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

// TestHomePlatformsNormalNewModel 普通用户平台卡片：ready 携带无标识 Token；管理员字段不出现
func TestHomePlatformsNormalNewModel(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	if _, err := srv.store.DB().Exec(`UPDATE users SET role = 'user' WHERE id = 1`); err != nil {
		t.Fatalf("降级用户失败: %v", err)
	}
	seedPlatformWithVersion(t, srv, "yaml")

	w := profileReq(t, srv, http.MethodGet, "/api/home/platforms", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("平台列表失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			List  []map[string]any `json:"list"`
			Total int              `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 || len(resp.Data.List) != 1 {
		t.Fatalf("平台列表异常: %+v", resp)
	}
	card := resp.Data.List[0]
	if card["status"] != "ready" {
		t.Errorf("普通用户有激活版本应 ready: %v", card["status"])
	}
	if card["download_token"] == nil || card["download_token"] == "" {
		t.Error("ready 状态应携带无标识下载 Token")
	}
	if card["subscription_name"] != "订阅条目" {
		t.Errorf("订阅名异常: %v", card["subscription_name"])
	}
	if _, ok := card["subscriptions"]; ok {
		t.Error("普通用户不应携带管理员池字段")
	}
	if _, ok := card["subscription"]; ok {
		t.Error("普通用户不应携带管理员预览对象")
	}
}

// TestHomePlatformsNoActiveVersion 平台有订阅行但无激活版本：不生成 Token
func TestHomePlatformsNoActiveVersion(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	if _, err := srv.store.DB().Exec(`UPDATE users SET role = 'user' WHERE id = 1`); err != nil {
		t.Fatalf("降级用户失败: %v", err)
	}
	ctx := context.Background()
	res, err := srv.store.DB().ExecContext(ctx, `INSERT INTO platforms (slug, name) VALUES ('platform-empty', '空平台')`)
	if err != nil {
		t.Fatalf("创建平台失败: %v", err)
	}
	plat, _ := res.LastInsertId()
	if _, err := srv.store.DB().ExecContext(ctx, `INSERT INTO subscriptions (slug, name, platform_id) VALUES ('sub-empty','空订阅',?)`, plat); err != nil {
		t.Fatalf("创建订阅失败: %v", err)
	}

	w := profileReq(t, srv, http.MethodGet, "/api/home/platforms", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("平台列表失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if len(resp.Data.List) != 1 {
		t.Fatalf("平台数量异常: %d", len(resp.Data.List))
	}
	card := resp.Data.List[0]
	if card["status"] != "unassigned" {
		t.Errorf("无激活版本应 unassigned: %v", card["status"])
	}
	if card["download_token"] != nil && card["download_token"] != "" {
		t.Error("无激活版本不应生成 Token")
	}
}

// TestHomePlatformsAdminPreview 管理员平台卡片：仅预览形态，不生成 Token
func TestHomePlatformsAdminPreview(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "admin", "admin@x.com", "password123")
	seedPlatformWithVersion(t, srv, "yaml")

	w := profileReq(t, srv, http.MethodGet, "/api/home/platforms", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("平台列表失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			List []map[string]any `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if len(resp.Data.List) != 1 {
		t.Fatalf("平台数量异常: %d", len(resp.Data.List))
	}
	card := resp.Data.List[0]
	if card["status"] != "admin_preview" {
		t.Errorf("管理员应 admin_preview: %v", card["status"])
	}
	if card["preview_available"] != true {
		t.Errorf("preview_available 应为 true: %v", card["preview_available"])
	}
	sub, ok := card["subscription"].(map[string]any)
	if !ok {
		t.Fatalf("管理员预览应携带 subscription 对象: %+v", card)
	}
	if sub["name"] != "订阅条目" || sub["product_type"] != "yaml" || sub["current_version"] != float64(1) {
		t.Errorf("订阅预览字段异常: %+v", sub)
	}
	if sub["content_kind"] != "upload" {
		t.Errorf("直接上传内容形态应 upload: %v", sub["content_kind"])
	}
	if card["download_token"] != nil && card["download_token"] != "" {
		t.Error("管理员预览不应生成显式 Token")
	}
}

// TestHomeSummaryBaseMode /api/home/summary 基础模式：traffic 不限流量；未设置默认规则时 home_rule=null
func TestHomeSummaryBaseMode(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	w := profileReq(t, srv, http.MethodGet, "/api/home/summary", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("summary 失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Traffic struct {
				Unlimited bool `json:"unlimited"`
			} `json:"traffic"`
			HomeRule any `json:"home_rule"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 || !resp.Data.Traffic.Unlimited {
		t.Errorf("基础模式 traffic 异常: %+v", resp)
	}
	if resp.Data.HomeRule != nil {
		t.Errorf("未设置默认规则 home_rule 应为 null: %+v", resp.Data.HomeRule)
	}
}

// TestHomeSummaryDefaultRule 设置首页默认规则后 summary 返回规则卡字段；删除默认规则后回 null
func TestHomeSummaryDefaultRule(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	ctx := context.Background()
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO rules (id, slug, name, current_version, is_home_default) VALUES (1, 'rule-x', '默认规则', 0, 1)`); err != nil {
		t.Fatalf("创建规则失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO rule_tokens (token, rule_id) VALUES ('rt-1', 1)`); err != nil {
		t.Fatalf("创建规则 Token 失败: %v", err)
	}
	// 无激活版本 → home_rule 仍为 null
	w := profileReq(t, srv, http.MethodGet, "/api/home/summary", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("summary 失败: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			HomeRule any `json:"home_rule"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Data.HomeRule != nil {
		t.Fatalf("默认规则无激活版本 home_rule 应为 null: %+v", resp.Data.HomeRule)
	}

	// 补建激活版本后 home_rule 返回卡片字段
	root := filepath.Dir(srv.store.DBPath())
	dir := filepath.Join(root, "contents", "rule", "1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建版本目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v1"), []byte("[Rule]\n"), 0o644); err != nil {
		t.Fatalf("写版本文件失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO versions (owner_type, owner_id, version_no, file_path, file_name) VALUES ('rule',1,1,'rule/1/v1','rule.conf')`); err != nil {
		t.Fatalf("插入版本失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `UPDATE rules SET current_version = 1 WHERE id = 1`); err != nil {
		t.Fatalf("设置当前版本失败: %v", err)
	}
	w2 := profileReq(t, srv, http.MethodGet, "/api/home/summary", token, nil)
	if w2.Code != http.StatusOK {
		t.Fatalf("summary 失败: %d %s", w2.Code, w2.Body.String())
	}
	var resp2 struct {
		Data struct {
			HomeRule map[string]any `json:"home_rule"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp2.Data.HomeRule == nil || resp2.Data.HomeRule["name"] != "默认规则" || resp2.Data.HomeRule["token"] != "rt-1" {
		t.Fatalf("home_rule 字段异常: %+v", resp2.Data.HomeRule)
	}

	// 删除默认规则后 home_rule 回 null
	if _, err := srv.store.DB().ExecContext(ctx, `DELETE FROM rules WHERE id = 1`); err != nil {
		t.Fatalf("删除规则失败: %v", err)
	}
	w3 := profileReq(t, srv, http.MethodGet, "/api/home/summary", token, nil)
	var resp3 struct {
		Data struct {
			HomeRule any `json:"home_rule"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w3.Body.Bytes(), &resp3); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp3.Data.HomeRule != nil {
		t.Errorf("删除默认规则后 home_rule 应为 null: %+v", resp3.Data.HomeRule)
	}
}
