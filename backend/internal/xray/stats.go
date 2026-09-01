package xray

import (
	"context"
	"fmt"
	"strings"
	"time"

	"vpn-sub/internal/store"
)

// CollectInstance 对单个实例执行逐用户流量采集。
func (s *InstanceService) CollectInstance(ctx context.Context, inst Instance) error {
	client, err := s.ClientFor(ctx, inst.ID)
	if err != nil {
		s.recordCollectError(ctx, inst.ID, err)
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, DialTimeout)
	_, err = client.ListInbounds(probeCtx)
	cancel()
	if err != nil {
		s.recordCollectError(ctx, inst.ID, err)
		return err
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id FROM users WHERE status = 'active' ORDER BY id`)
	if err != nil {
		s.recordCollectError(ctx, inst.ID, err)
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			s.recordCollectError(ctx, inst.ID, err)
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		s.recordCollectError(ctx, inst.ID, err)
		return err
	}
	for _, id := range ids {
		email := UserEmail(id)
		rctx, cancel := context.WithTimeout(ctx, RPCTimeout)
		resp, err := client.QueryStats(rctx, "user>>>"+email+">>>traffic", true)
		cancel()
		if err != nil {
			s.recordCollectError(ctx, inst.ID, err)
			return err
		}
		var up, down int64
		for _, stat := range resp.GetStat() {
			name := stat.GetName()
			if !strings.HasPrefix(name, "user>>>"+email+">>>traffic>>>") {
				continue
			}
			if strings.HasSuffix(name, ">>>uplink") {
				up += stat.GetValue()
			}
			if strings.HasSuffix(name, ">>>downlink") {
				down += stat.GetValue()
			}
		}
		if err := upsertTraffic(ctx, s.store, id, currentYM(), up, down); err != nil {
			// 外键失败（用户已删）静默跳过；其余错误记录并中断。
			if !strings.Contains(err.Error(), "FOREIGN KEY") {
				s.recordCollectError(ctx, inst.ID, err)
				return err
			}
		}
	}
	_, err = s.store.DB().ExecContext(ctx,
		`UPDATE xray_instances SET last_collect_at = CURRENT_TIMESTAMP, collect_status = 'ok', collect_error = '', updated_at = CURRENT_TIMESTAMP WHERE id = ?`, inst.ID)
	return err
}

func (s *InstanceService) recordCollectError(ctx context.Context, instanceID int64, err error) {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	if _, err := s.store.DB().ExecContext(ctx,
		`UPDATE xray_instances SET collect_status = 'error', collect_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, msg, instanceID); err != nil {
		s.log.Warn("记录采集错误状态失败", "instance_id", instanceID, "err", err)
	}
}

func upsertTraffic(ctx context.Context, st *store.Store, userID int64, ym string, up, down int64) error {
	_, err := st.DB().ExecContext(ctx,
		`INSERT INTO traffic_records (user_id, ym, uplink, downlink) VALUES (?,?,?,?)
		 ON CONFLICT(user_id, ym) DO UPDATE SET
		   uplink = uplink + excluded.uplink,
		   downlink = downlink + excluded.downlink,
		   updated_at = CURRENT_TIMESTAMP`, userID, ym, up, down)
	if err != nil {
		return fmt.Errorf("UPSERT 流量失败: %w", err)
	}
	return nil
}

func currentYM() string {
	return time.Now().UTC().Format("2006-01")
}
