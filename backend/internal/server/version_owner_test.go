package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

type versionOwnerAPIResponse struct {
	Code int                  `json:"code"`
	Data versionOwnerResponse `json:"data"`
}

func TestVersionOwnerEndpointResolvesAllOwnerTypes(t *testing.T) {
	srv := newOverviewTestServer(t)
	token := regUser(t, srv, "admin", "admin@example.com", "password123")
	ctx := context.Background()
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO platforms (slug, name, product_type) VALUES ('platform-owner', '归属平台', 'yaml')`); err != nil {
		t.Fatalf("插入平台失败: %v", err)
	}
	var platformID int64
	if err := srv.store.DB().QueryRowContext(ctx, `SELECT id FROM platforms WHERE slug = 'platform-owner'`).Scan(&platformID); err != nil {
		t.Fatalf("查询平台 ID 失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO subscriptions (slug, name, platform_id, product_type) VALUES ('subscription-owner', '归属订阅', ?, 'yaml')`, platformID); err != nil {
		t.Fatalf("插入订阅失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `INSERT INTO rules (slug, name) VALUES ('rule-owner', '归属规则')`); err != nil {
		t.Fatalf("插入规则失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `INSERT INTO share_subscriptions (slug, name) VALUES ('share-owner', '归属分享')`); err != nil {
		t.Fatalf("插入分享失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO users (username, email, role, user_source, status) VALUES ('custom-user', 'custom-user@example.com', 'user', 'selfreg', 'active')`); err != nil {
		t.Fatalf("插入自定义订阅用户失败: %v", err)
	}
	var subscriptionID, ruleID, shareID, customUserID int64
	for _, query := range []struct {
		SQL  string
		Dest *int64
	}{
		{`SELECT id FROM subscriptions WHERE slug = 'subscription-owner'`, &subscriptionID},
		{`SELECT id FROM rules WHERE slug = 'rule-owner'`, &ruleID},
		{`SELECT id FROM share_subscriptions WHERE slug = 'share-owner'`, &shareID},
		{`SELECT id FROM users WHERE username = 'custom-user'`, &customUserID},
	} {
		if err := srv.store.DB().QueryRowContext(ctx, query.SQL).Scan(query.Dest); err != nil {
			t.Fatalf("查询归属资源 ID 失败: %v", err)
		}
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO custom_subscriptions (slug, user_id, platform_id) VALUES ('custom-owner', ?, ?)`, customUserID, platformID); err != nil {
		t.Fatalf("插入自定义订阅失败: %v", err)
	}
	var customID int64
	if err := srv.store.DB().QueryRowContext(ctx, `SELECT id FROM custom_subscriptions WHERE slug = 'custom-owner'`).Scan(&customID); err != nil {
		t.Fatalf("查询自定义订阅 ID 失败: %v", err)
	}

	insertVersion := func(ownerType string, ownerID int64) int64 {
		t.Helper()
		res, err := srv.store.DB().ExecContext(ctx,
			`INSERT INTO versions (owner_type, owner_id, version_no, file_path, file_name) VALUES (?,?,?,?,?)`,
			ownerType, ownerID, 1, ownerType+"/v1", "test.conf")
		if err != nil {
			t.Fatalf("插入 %s 版本失败: %v", ownerType, err)
		}
		versionID, err := res.LastInsertId()
		if err != nil {
			t.Fatalf("读取 %s 版本 ID 失败: %v", ownerType, err)
		}
		return versionID
	}
	cases := []struct {
		VersionID int64
		OwnerType string
		OwnerID   int64
		Name      string
		TypeLabel string
		BackPath  string
	}{
		{insertVersion("subscription", subscriptionID), "subscription", subscriptionID, "归属订阅", "订阅", "/admin/subscriptions"},
		{insertVersion("rule", ruleID), "rule", ruleID, "归属规则", "规则", "/admin/rules"},
		{insertVersion("share", shareID), "share", shareID, "归属分享", "分享", "/admin/shares"},
		{insertVersion("custom", customID), "custom", customID, "custom-user / 归属平台", "自定义订阅", "/admin/users"},
	}
	for _, tc := range cases {
		path := "/api/admin/versions/" + strconv.FormatInt(tc.VersionID, 10) + "/owner"
		w := profileReq(t, srv, http.MethodGet, path, token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 归属状态码异常: %d %s", tc.OwnerType, w.Code, w.Body.String())
		}
		var resp versionOwnerAPIResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析 %s 归属响应失败: %v", tc.OwnerType, err)
		}
		if resp.Code != 0 || resp.Data.OwnerType != tc.OwnerType || resp.Data.OwnerID != tc.OwnerID ||
			resp.Data.Name != tc.Name || resp.Data.TypeLabel != tc.TypeLabel || resp.Data.BackPath != tc.BackPath {
			t.Errorf("%s 归属响应异常: %+v", tc.OwnerType, resp)
		}
	}
	if w := profileReq(t, srv, http.MethodGet, "/api/admin/versions/999999/owner", token, nil); w.Code != http.StatusNotFound {
		t.Errorf("不存在版本应 404: %d %s", w.Code, w.Body.String())
	}
}
