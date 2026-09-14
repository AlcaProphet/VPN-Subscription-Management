package server

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"vpn-sub/internal/config"
)

type trafficPayloadShape struct {
	Unlimited  bool   `json:"unlimited"`
	UsedBytes  int64  `json:"used_bytes"`
	QuotaBytes *int64 `json:"quota_bytes"`
	Exceeded   bool   `json:"exceeded"`
}

func decodeTrafficData(t *testing.T, body []byte) trafficPayloadShape {
	t.Helper()
	var resp struct {
		Code int                 `json:"code"`
		Data trafficPayloadShape `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("解析流量响应失败: %v", err)
	}
	return resp.Data
}

func decodeSummaryTrafficData(t *testing.T, body []byte) trafficPayloadShape {
	t.Helper()
	var resp struct {
		Code int `json:"code"`
		Data struct {
			Traffic trafficPayloadShape `json:"traffic"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("解析首页汇总响应失败: %v", err)
	}
	return resp.Data.Traffic
}

// TestHomeSummaryAndProfileTrafficShape 固定首页/个人中心流量 JSON 形状：
// basic unlimited/quota_bytes=null；advanced 配额字节与超限标记均由 Xray 业务层返回。
func TestHomeSummaryAndProfileTrafficShape(t *testing.T) {
	srv := newDownloadTestServer(t)
	token := regUser(t, srv, "u1", "u1@x.com", "password123")
	ctx := context.Background()

	for _, path := range []string{"/api/home/summary", "/api/profile/traffic"} {
		w := profileReq(t, srv, http.MethodGet, path, token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应 200: %d %s", path, w.Code, w.Body.String())
		}
		decode := decodeTrafficData
		if path == "/api/home/summary" {
			decode = decodeSummaryTrafficData
		}
		got := decode(t, w.Body.Bytes())
		if !got.Unlimited || got.UsedBytes != 0 || got.QuotaBytes != nil || got.Exceeded {
			t.Fatalf("%s 基础模式形状异常: %+v", path, got)
		}
	}

	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO groups (slug, name, is_default, default_quota) VALUES ('group-test','测试组',1,5)`); err != nil {
		t.Fatalf("创建配额组失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx, `UPDATE users SET group_id = 1 WHERE id = 1`); err != nil {
		t.Fatalf("绑定用户组失败: %v", err)
	}
	if err := srv.cfg.Set(ctx, config.KeyAdvancedMode, "true"); err != nil {
		t.Fatalf("开启高级模式失败: %v", err)
	}
	if _, err := srv.store.DB().ExecContext(ctx,
		`INSERT INTO traffic_records (user_id, ym, uplink, downlink) VALUES (1, ?, ?, ?)`,
		time.Now().UTC().Format("2006-01"), int64(1)<<30, int64(1)<<30); err != nil {
		t.Fatalf("插入流量失败: %v", err)
	}
	w := profileReq(t, srv, http.MethodGet, "/api/profile/traffic", token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("高级模式流量应 200: %d %s", w.Code, w.Body.String())
	}
	got := decodeTrafficData(t, w.Body.Bytes())
	fiveGiB := int64(5) << 30
	if got.Unlimited || got.UsedBytes != 2<<30 || got.QuotaBytes == nil || *got.QuotaBytes != fiveGiB || got.Exceeded {
		t.Fatalf("高级模式流量形状异常: %+v", got)
	}

	if _, err := srv.store.DB().ExecContext(ctx, `UPDATE users SET quota_exceeded = 1 WHERE id = 1`); err != nil {
		t.Fatalf("设置超限失败: %v", err)
	}
	w = profileReq(t, srv, http.MethodGet, "/api/profile/traffic", token, nil)
	got = decodeTrafficData(t, w.Body.Bytes())
	if !got.Exceeded {
		t.Fatalf("超限标记应透出: %+v", got)
	}
}
