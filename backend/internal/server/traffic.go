package server

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/xray"
)

func currentYM() string {
	return time.Now().UTC().Format("2006-01")
}

// trafficPayload 返回首页/个人中心统一的流量负载。
func trafficPayload(ctx context.Context, st *store.Store, cfg *config.Service, syncSvc *xray.SyncService, userID int64) gin.H {
	if !cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		return gin.H{"unlimited": true, "used_bytes": 0, "quota_bytes": nil, "exceeded": false}
	}
	var used int64
	_ = st.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(uplink+downlink),0) FROM traffic_records WHERE user_id = ? AND ym = ?`, userID, currentYM()).Scan(&used)
	quota, err := syncSvc.EffectiveQuota(ctx, userID)
	if err != nil || quota == nil {
		return gin.H{"unlimited": true, "used_bytes": used, "quota_bytes": nil, "exceeded": false}
	}
	quotaBytes := int64(*quota * 1024 * 1024 * 1024)
	var exceeded int
	_ = st.DB().QueryRowContext(ctx, `SELECT quota_exceeded FROM users WHERE id = ?`, userID).Scan(&exceeded)
	return gin.H{"unlimited": false, "used_bytes": used, "quota_bytes": quotaBytes, "exceeded": exceeded == 1}
}
