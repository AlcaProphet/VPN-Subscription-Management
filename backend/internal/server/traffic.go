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
func trafficPayload(ctx context.Context, st *store.Store, cfg *config.Service, syncSvc *xray.SyncService, userID int64) (gin.H, error) {
	if !cfg.GetBool(ctx, config.KeyAdvancedMode, false) {
		return gin.H{"unlimited": true, "used_bytes": 0, "quota_bytes": nil, "exceeded": false}, nil
	}
	var used int64
	if err := st.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(uplink+downlink),0) FROM traffic_records WHERE user_id = ? AND ym = ?`, userID, currentYM()).Scan(&used); err != nil {
		return nil, err
	}
	quota, err := syncSvc.EffectiveQuota(ctx, userID)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return gin.H{"unlimited": true, "used_bytes": used, "quota_bytes": nil, "exceeded": false}, nil
	}
	quotaBytes := int64(*quota * 1024 * 1024 * 1024)
	var exceeded int
	if err := st.DB().QueryRowContext(ctx, `SELECT quota_exceeded FROM users WHERE id = ?`, userID).Scan(&exceeded); err != nil {
		return nil, err
	}
	return gin.H{"unlimited": false, "used_bytes": used, "quota_bytes": quotaBytes, "exceeded": exceeded == 1}, nil
}
