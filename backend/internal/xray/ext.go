package xray

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	statscmd "github.com/xtls/xray-core/app/stats/command"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

// ExtPushTarget 独立账号推送目标（请求/响应形状为 instance_id + inbound_tag）。
type ExtPushTarget struct {
	InstanceID  int64   `json:"instance_id"`
	InboundTag  string  `json:"inbound_tag"`
	NodeID      int64   `json:"node_id,omitempty"`
	Name        string  `json:"name,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	RenderName  string  `json:"render_name,omitempty"`
	APIAddr     string  `json:"api_addr,omitempty"`
	Protocol    string  `json:"protocol,omitempty"`
}

// ExtCredentials 独立账号明文凭据（仅创建/查询端点一次性返回）。
type ExtCredentials struct {
	UUID        string `json:"uuid"`
	ProxySecret string `json:"proxy_secret"`
}

// ExtAccount 独立账号数据模型。
type ExtAccount struct {
	ID            int64           `json:"id"`
	Name          string          `json:"name"`
	Email         string          `json:"email"`
	Quota         *float64        `json:"quota,omitempty"`
	QuotaExceeded bool            `json:"quota_exceeded"`
	PushTargets   []ExtPushTarget `json:"push_targets,omitempty"`
	UsedBytes     int64           `json:"used_bytes,omitempty"`
}

// ExtService 独立账号业务服务。
type ExtService struct {
	store     *store.Store
	cfg       *config.Service
	instances *InstanceService
	log       *slog.Logger
	apiFor    func(ctx context.Context, instanceID int64) (API, error)
}

// NewExtService 构造独立账号服务。
func NewExtService(st *store.Store, cfg *config.Service, instances *InstanceService, lg *slog.Logger) *ExtService {
	return &ExtService{store: st, cfg: cfg, instances: instances, log: lg}
}

// SetAPIFactory 供测试注入 fake。
func (s *ExtService) SetAPIFactory(fn func(ctx context.Context, instanceID int64) (API, error)) {
	s.apiFor = fn
}

func (s *ExtService) api(ctx context.Context, instanceID int64) (API, error) {
	if s.apiFor != nil {
		return s.apiFor(ctx, instanceID)
	}
	return s.instances.ClientFor(ctx, instanceID)
}

// ExtEmail 返回独立账号 Xray email。
func ExtEmail(id int64) string {
	return fmt.Sprintf("ext-%d@vpn.local", id)
}

// CreateExt 创建独立账号。credentialMode 为 generate/manual。
func (s *ExtService) CreateExt(ctx context.Context, name, credentialMode, uuidVal, proxySecret string, quota *float64, targets []ExtPushTarget) (*ExtAccount, *ExtCredentials, error) {
	if name == "" {
		return nil, nil, fmt.Errorf("%w: 名称必填", ErrBadRequest)
	}
	if quota != nil && *quota < 0 {
		return nil, nil, fmt.Errorf("%w: 配额不能为负数", ErrBadRequest)
	}
	if credentialMode != "generate" && credentialMode != "manual" {
		return nil, nil, fmt.Errorf("%w: 凭据模式无效", ErrBadRequest)
	}
	if credentialMode == "manual" && (uuidVal == "" || proxySecret == "") {
		return nil, nil, fmt.Errorf("%w: 手填接管需要 UUID 与代理密码", ErrBadRequest)
	}
	if credentialMode == "generate" {
		uuidVal = uuid.NewString()
		proxySecret = randomSecret()
	}

	var created *ExtAccount
	var creds *ExtCredentials
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		advanced, err := s.cfg.GetTx(ctx, tx, config.KeyAdvancedMode)
		if err != nil {
			return err
		}
		if advanced != "true" {
			return ErrAdvancedOff
		}
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_ext_accounts WHERE name = ?`, name).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrExtConflict
		}
		valid, err := s.validateTargetsTx(ctx, tx, targets)
		if err != nil {
			return err
		}
		signingKey, err := s.cfg.GetSigningKeyTx(ctx, tx)
		if err != nil {
			return err
		}
		uuidEnc, err := config.Encrypt([]byte(uuidVal), signingKey)
		if err != nil {
			return fmt.Errorf("加密 UUID 失败: %w", err)
		}
		secretEnc, err := config.Encrypt([]byte(proxySecret), signingKey)
		if err != nil {
			return fmt.Errorf("加密代理密码失败: %w", err)
		}
		pending := "ext-pending-" + randomHex(8) + "@vpn.local"
		var quotaVal any
		if quota != nil {
			quotaVal = *quota
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO xray_ext_accounts (name, email, uuid_encrypted, proxy_secret_encrypted, quota) VALUES (?,?,?,?,?)`,
			name, pending, uuidEnc, secretEnc, quotaVal)
		if err != nil {
			return fmt.Errorf("创建独立账号失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		email := ExtEmail(id)
		if _, err := tx.ExecContext(ctx,
			`UPDATE xray_ext_accounts SET email = ? WHERE id = ?`, email, id); err != nil {
			return fmt.Errorf("回填独立账号 email 失败: %w", err)
		}
		for _, t := range valid {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO xray_ext_users (ext_account_id, instance_id, inbound_tag, node_id, sync_status)
				 VALUES (?,?,?,?,'pending')`, id, t.InstanceID, t.InboundTag, t.NodeID); err != nil {
				return fmt.Errorf("写入独立账号推送目标失败: %w", err)
			}
		}
		created = &ExtAccount{ID: id, Name: name, Email: email, Quota: quota, PushTargets: valid}
		if credentialMode == "generate" {
			creds = &ExtCredentials{UUID: uuidVal, ProxySecret: proxySecret}
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	// 提交后逐个 AddUser；generate 模式同 email 已存在先 Remove 再 Add。
	for _, t := range created.PushTargets {
		if err := s.pushOne(ctx, created.ID, t, credentialMode == "generate"); err != nil {
			s.log.Warn("独立账号创建后推送失败", "ext_id", created.ID, "tag", t.InboundTag, "err", err)
		}
	}
	return created, creds, nil
}

// UpdateExt 更新独立账号：名称/配额/凭据留空保留/推送目标全量 diff。
func (s *ExtService) UpdateExt(ctx context.Context, id int64, name, uuidVal, proxySecret string, quota *float64, targets []ExtPushTarget) (*ExtAccount, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: 名称必填", ErrBadRequest)
	}
	if quota != nil && *quota < 0 {
		return nil, fmt.Errorf("%w: 配额不能为负数", ErrBadRequest)
	}
	var oldTargets []ExtPushTarget
	var newTargets []ExtPushTarget
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_ext_accounts WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrExtNotFound
		}
		var dup int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_ext_accounts WHERE name = ? AND id != ?`, name, id).Scan(&dup); err != nil {
			return err
		}
		if dup > 0 {
			return ErrExtConflict
		}
		valid, err := s.validateTargetsTx(ctx, tx, targets)
		if err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx,
			`SELECT instance_id, inbound_tag, node_id FROM xray_ext_users WHERE ext_account_id = ?`, id)
		if err != nil {
			return err
		}
		oldTargets = nil
		for rows.Next() {
			var t ExtPushTarget
			if err := rows.Scan(&t.InstanceID, &t.InboundTag, &t.NodeID); err != nil {
				_ = rows.Close()
				return err
			}
			oldTargets = append(oldTargets, t)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		signingKey, err := s.cfg.GetSigningKeyTx(ctx, tx)
		if err != nil {
			return err
		}
		if uuidVal != "" {
			enc, err := config.Encrypt([]byte(uuidVal), signingKey)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE xray_ext_accounts SET uuid_encrypted = ? WHERE id = ?`, enc, id); err != nil {
				return err
			}
		}
		if proxySecret != "" {
			enc, err := config.Encrypt([]byte(proxySecret), signingKey)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE xray_ext_accounts SET proxy_secret_encrypted = ? WHERE id = ?`, enc, id); err != nil {
				return err
			}
		}
		var quotaVal any
		if quota != nil {
			quotaVal = *quota
		} else {
			quotaVal = nil
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE xray_ext_accounts SET name = ?, quota = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, quotaVal, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM xray_ext_users WHERE ext_account_id = ?`, id); err != nil {
			return err
		}
		for _, t := range valid {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO xray_ext_users (ext_account_id, instance_id, inbound_tag, node_id, sync_status)
				 VALUES (?,?,?,?,'pending')`, id, t.InstanceID, t.InboundTag, t.NodeID); err != nil {
				return err
			}
		}
		newTargets = valid
		return nil
	})
	if err != nil {
		return nil, err
	}
	oldMap := map[string]ExtPushTarget{}
	for _, t := range oldTargets {
		oldMap[fmt.Sprintf("%d/%s", t.InstanceID, t.InboundTag)] = t
	}
	newMap := map[string]ExtPushTarget{}
	for _, t := range newTargets {
		newMap[fmt.Sprintf("%d/%s", t.InstanceID, t.InboundTag)] = t
	}
	var exceeded bool
	_ = s.store.DB().QueryRowContext(ctx, `SELECT quota_exceeded FROM xray_ext_accounts WHERE id = ?`, id).Scan(&exceeded)
	for key, t := range oldMap {
		if _, ok := newMap[key]; !ok {
			_ = s.removeOne(ctx, id, t)
		}
	}
	if !exceeded {
		for key, t := range newMap {
			if _, ok := oldMap[key]; !ok {
				_ = s.pushOne(ctx, id, t, false)
			}
		}
	}
	return s.GetExt(ctx, id)
}

// DeleteExt 删除独立账号并清理 Xray 侧账号。
func (s *ExtService) DeleteExt(ctx context.Context, id int64) error {
	var targets []ExtPushTarget
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			`SELECT instance_id, inbound_tag, node_id FROM xray_ext_users WHERE ext_account_id = ?`, id)
		if err != nil {
			return err
		}
		for rows.Next() {
			var t ExtPushTarget
			if err := rows.Scan(&t.InstanceID, &t.InboundTag, &t.NodeID); err != nil {
				_ = rows.Close()
				return err
			}
			targets = append(targets, t)
		}
		if err := rows.Close(); err != nil {
			return err
		}
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_ext_accounts WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrExtNotFound
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM xray_ext_accounts WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除独立账号失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, t := range targets {
		_ = s.removeOne(ctx, id, t)
	}
	return nil
}

// RetryExt 重试 failed 推送记录。
func (s *ExtService) RetryExt(ctx context.Context, id int64) (map[string]any, error) {
	var exists int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_ext_accounts WHERE id = ?`, id).Scan(&exists); err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrExtNotFound
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT instance_id, inbound_tag, node_id, action FROM xray_ext_users WHERE ext_account_id = ? AND sync_status = 'failed'`, id)
	if err != nil {
		return nil, err
	}
	type failedTarget struct {
		ExtPushTarget
		Action string
	}
	var failed []failedTarget
	for rows.Next() {
		var t ExtPushTarget
		var action string
		if err := rows.Scan(&t.InstanceID, &t.InboundTag, &t.NodeID, &action); err != nil {
			_ = rows.Close()
			return nil, err
		}
		failed = append(failed, failedTarget{ExtPushTarget: t, Action: action})
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var exceeded bool
	_ = s.store.DB().QueryRowContext(ctx, `SELECT quota_exceeded FROM xray_ext_accounts WHERE id = ?`, id).Scan(&exceeded)
	added, addFailed := 0, 0
	removed, removeFailed := 0, 0
	for _, ft := range failed {
		if ft.Action == "remove" {
			if err := s.removeOne(ctx, id, ft.ExtPushTarget); err != nil {
				removeFailed++
			} else {
				removed++
			}
			continue
		}
		if exceeded {
			addFailed++
			continue
		}
		if err := s.pushOne(ctx, id, ft.ExtPushTarget, false); err != nil {
			addFailed++
		} else {
			added++
		}
	}
	return map[string]any{"added": added, "add_failed": addFailed, "removed": removed, "remove_failed": removeFailed}, nil
}

// GetExtCredentials 解密返回独立账号凭据。
func (s *ExtService) GetExtCredentials(ctx context.Context, id int64) (*ExtCredentials, error) {
	var ue, se string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(uuid_encrypted,''), COALESCE(proxy_secret_encrypted,'') FROM xray_ext_accounts WHERE id = ?`, id).Scan(&ue, &se)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExtNotFound
		}
		return nil, err
	}
	if ue == "" || se == "" {
		return nil, ErrIncompleteCredentials
	}
	key, err := s.cfg.GetSigningKey(ctx)
	if err != nil {
		return nil, err
	}
	u, err := config.Decrypt(ue, key)
	if err != nil {
		return nil, fmt.Errorf("解密 UUID 失败: %w", err)
	}
	p, err := config.Decrypt(se, key)
	if err != nil {
		return nil, fmt.Errorf("解密代理密码失败: %w", err)
	}
	return &ExtCredentials{UUID: string(u), ProxySecret: string(p)}, nil
}

// GetExt 获取单个独立账号（含推送目标与本月用量）。
func (s *ExtService) GetExt(ctx context.Context, id int64) (*ExtAccount, error) {
	var acc ExtAccount
	var quota sql.NullFloat64
	var exceeded int
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, name, email, quota, quota_exceeded FROM xray_ext_accounts WHERE id = ?`, id).
		Scan(&acc.ID, &acc.Name, &acc.Email, &quota, &exceeded)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExtNotFound
		}
		return nil, err
	}
	if quota.Valid {
		acc.Quota = &quota.Float64
	}
	acc.QuotaExceeded = exceeded == 1
	acc.PushTargets, err = s.pushTargetsFor(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(uplink+downlink),0) FROM xray_ext_traffic WHERE ext_account_id = ? AND ym = ?`, id, currentYM()).Scan(&acc.UsedBytes)
	return &acc, nil
}

// ListExt 返回全部独立账号（含用量与推送摘要）。
func (s *ExtService) ListExt(ctx context.Context) ([]ExtAccount, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, name, email, quota, quota_exceeded FROM xray_ext_accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ExtAccount, 0)
	for rows.Next() {
		var acc ExtAccount
		var quota sql.NullFloat64
		var exceeded int
		if err := rows.Scan(&acc.ID, &acc.Name, &acc.Email, &quota, &exceeded); err != nil {
			return nil, err
		}
		if quota.Valid {
			acc.Quota = &quota.Float64
		}
		acc.QuotaExceeded = exceeded == 1
		acc.PushTargets, err = s.pushTargetsFor(ctx, acc.ID)
		if err != nil {
			return nil, err
		}
		_ = s.store.DB().QueryRowContext(ctx,
			`SELECT COALESCE(SUM(uplink+downlink),0) FROM xray_ext_traffic WHERE ext_account_id = ? AND ym = ?`, acc.ID, currentYM()).Scan(&acc.UsedBytes)
		out = append(out, acc)
	}
	return out, rows.Err()
}

// ResetExtQuota 清当月流量并重新推送。
func (s *ExtService) ResetExtQuota(ctx context.Context, id int64) error {
	var exists int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM xray_ext_accounts WHERE id = ?`, id).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return ErrExtNotFound
	}
	if _, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM xray_ext_traffic WHERE ext_account_id = ? AND ym = ?`, id, currentYM()); err != nil {
		return err
	}
	if _, err := s.store.DB().ExecContext(ctx,
		`UPDATE xray_ext_accounts SET quota_exceeded = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id); err != nil {
		return err
	}
	targets, err := s.pushTargetsFor(ctx, id)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if err := s.pushOne(ctx, id, t, false); err != nil {
			s.log.Warn("重置配额后推送独立账号失败", "ext_id", id, "tag", t.InboundTag, "err", err)
		}
	}
	return nil
}

// CollectExtTraffic 对单个实例采集全部独立账号流量。
func (s *ExtService) CollectExtTraffic(ctx context.Context, inst Instance) error {
	client, err := s.api(ctx, inst.ID)
	if err != nil {
		return err
	}
	statsClient, ok := client.(interface {
		QueryStats(ctx context.Context, pattern string, reset bool) (*statscmd.QueryStatsResponse, error)
	})
	if !ok {
		return errors.New("当前 Xray API 客户端不支持流量统计查询")
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT DISTINCT e.id FROM xray_ext_accounts e
		 JOIN xray_ext_users xu ON xu.ext_account_id = e.id
		 WHERE xu.instance_id = ? ORDER BY e.id`, inst.ID)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		email := ExtEmail(id)
		rctx, cancel := context.WithTimeout(ctx, RPCTimeout)
		resp, err := statsClient.QueryStats(rctx, "user>>>"+email+">>>traffic", true)
		cancel()
		if err != nil {
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
		if _, err := s.store.DB().ExecContext(ctx,
			`INSERT INTO xray_ext_traffic (ext_account_id, ym, uplink, downlink) VALUES (?,?,?,?)
			 ON CONFLICT(ext_account_id, ym) DO UPDATE SET
			   uplink = uplink + excluded.uplink,
			   downlink = downlink + excluded.downlink,
			   updated_at = CURRENT_TIMESTAMP`, id, currentYM(), up, down); err != nil {
			return err
		}
	}
	return nil
}

// CheckExtQuota 检查单个独立账号是否超限；超限则移除全部已推账号。
func (s *ExtService) CheckExtQuota(ctx context.Context, id int64) error {
	var quota sql.NullFloat64
	var exceeded int
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT quota, quota_exceeded FROM xray_ext_accounts WHERE id = ?`, id).Scan(&quota, &exceeded)
	if err != nil {
		return err
	}
	if !quota.Valid || quota.Float64 <= 0 {
		return nil
	}
	var used int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(uplink+downlink),0) FROM xray_ext_traffic WHERE ext_account_id = ? AND ym = ?`, id, currentYM()).Scan(&used); err != nil {
		return err
	}
	if float64(used) <= quota.Float64*1024*1024*1024 {
		return nil
	}
	targets, err := s.pushTargetsFor(ctx, id)
	if err != nil {
		return err
	}
	for _, t := range targets {
		// 超限摘除只从 Xray 侧移除，不删除本地期望集记录；保留目标行以便重置后重推。
		if err := s.removeQuotaExceededTarget(ctx, id, t); err != nil {
			s.log.Warn("超限摘除独立账号 Xray 用户失败", "ext_id", id, "tag", t.InboundTag, "err", err)
		}
	}
	_, err = s.store.DB().ExecContext(ctx,
		`UPDATE xray_ext_accounts SET quota_exceeded = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// CheckAllExtQuota 检查全部独立账号配额。
func (s *ExtService) CheckAllExtQuota(ctx context.Context) error {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id FROM xray_ext_accounts ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return err
		}
		if err := s.CheckExtQuota(ctx, id); err != nil {
			s.log.Warn("独立账号配额检查失败", "ext_id", id, "err", err)
		}
	}
	return nil
}

// SyncAllExt 对全部独立账号执行一次全量推送（导入/初始化后使用；超限账号跳过并记录提示）。
func (s *ExtService) SyncAllExt(ctx context.Context) []string {
	rows, err := s.store.DB().QueryContext(ctx, `SELECT id FROM xray_ext_accounts ORDER BY id`)
	if err != nil {
		return []string{"读取独立账号列表失败: " + err.Error()}
	}
	defer rows.Close()
	var hints []string
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			hints = append(hints, fmt.Sprintf("读取独立账号 ID 失败: %v", err))
			continue
		}
		var exceeded int
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT quota_exceeded FROM xray_ext_accounts WHERE id = ?`, id).Scan(&exceeded); err != nil {
			hints = append(hints, fmt.Sprintf("读取独立账号配额状态失败: %v", err))
			continue
		}
		if exceeded == 1 {
			hints = append(hints, fmt.Sprintf("独立账号 %d 已超限，跳过推送", id))
			continue
		}
		targets, err := s.pushTargetsFor(ctx, id)
		if err != nil {
			hints = append(hints, fmt.Sprintf("读取独立账号 %d 推送目标失败: %v", id, err))
			continue
		}
		for _, t := range targets {
			if err := s.pushOne(ctx, id, t, false); err != nil {
				hints = append(hints, fmt.Sprintf("独立账号 %d 推送 %d/%s 失败: %v", id, t.InstanceID, t.InboundTag, err))
			}
		}
	}
	return hints
}

// --- 内部辅助 ---

var (
	ErrExtNotFound = errors.New("独立账号不存在")
	ErrExtConflict = errors.New("独立账号名称已存在")
)

func (s *ExtService) validateTargetsTx(ctx context.Context, tx *sql.Tx, targets []ExtPushTarget) ([]ExtPushTarget, error) {
	if len(targets) == 0 {
		return []ExtPushTarget{}, nil
	}
	valid := make([]ExtPushTarget, 0, len(targets))
	for _, t := range targets {
		var id int64
		var name, tag, protocol, apiAddr string
		var display sql.NullString
		var nEnabled, allocatable, missing, iEnabled int
		err := tx.QueryRowContext(ctx,
			`SELECT n.id, n.name, n.tag, n.protocol, i.api_addr, n.display_name,
			        n.enabled, n.allocatable, n.missing, i.enabled
			 FROM nodes n JOIN xray_instances i ON i.id = n.instance_id
			 WHERE n.instance_id = ? AND n.tag = ? AND n.source = 'xray'`,
			t.InstanceID, t.InboundTag).
			Scan(&id, &name, &tag, &protocol, &apiAddr, &display, &nEnabled, &allocatable, &missing, &iEnabled)
		if err != nil {
			return nil, fmt.Errorf("%w: 推送目标不存在或不可用: %d/%s", ErrBadRequest, t.InstanceID, t.InboundTag)
		}
		if !isSupportedExtProtocol(protocol) || nEnabled != 1 || allocatable != 1 || missing != 0 || iEnabled != 1 {
			return nil, fmt.Errorf("%w: 推送目标不满足独立账号条件: %d/%s", ErrBadRequest, t.InstanceID, t.InboundTag)
		}
		valid = append(valid, ExtPushTarget{
			InstanceID: t.InstanceID, InboundTag: t.InboundTag, NodeID: id,
			Name: name, APIAddr: apiAddr, Protocol: protocol,
		})
		if display.Valid && display.String != "" {
			v := display.String
			valid[len(valid)-1].DisplayName = &v
			valid[len(valid)-1].RenderName = v
		} else {
			valid[len(valid)-1].RenderName = name
		}
	}
	return valid, nil
}

func isSupportedExtProtocol(p string) bool {
	switch p {
	case "vless", "vmess", "trojan", "shadowsocks", "ss":
		return true
	}
	return false
}

func (s *ExtService) pushTargetsFor(ctx context.Context, id int64) ([]ExtPushTarget, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT xu.instance_id, xu.inbound_tag, xu.node_id, n.name, n.display_name, n.protocol, i.api_addr
		 FROM xray_ext_users xu
		 JOIN nodes n ON n.id = xu.node_id
		 JOIN xray_instances i ON i.id = xu.instance_id
		 WHERE xu.ext_account_id = ? ORDER BY xu.instance_id, xu.inbound_tag`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ExtPushTarget, 0)
	for rows.Next() {
		var t ExtPushTarget
		var display sql.NullString
		if err := rows.Scan(&t.InstanceID, &t.InboundTag, &t.NodeID, &t.Name, &display, &t.Protocol, &t.APIAddr); err != nil {
			return nil, err
		}
		if display.Valid && display.String != "" {
			v := display.String
			t.DisplayName = &v
			t.RenderName = v
		} else {
			t.RenderName = t.Name
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *ExtService) pushOne(ctx context.Context, extID int64, t ExtPushTarget, overwrite bool) error {
	client, err := s.api(ctx, t.InstanceID)
	if err != nil {
		s.markExtFailed(ctx, extID, t, err)
		return err
	}
	creds, err := s.GetExtCredentials(ctx, extID)
	if err != nil {
		s.markExtFailed(ctx, extID, t, err)
		return err
	}
	nv, err := extNodeView(ctx, s.store, t.NodeID)
	if err != nil {
		s.markExtFailed(ctx, extID, t, err)
		return err
	}
	nv.Tag = t.InboundTag
	user, err := BuildExtUser(extID, creds.UUID, creds.ProxySecret, nv)
	if err != nil {
		s.markExtFailed(ctx, extID, t, err)
		return err
	}
	if overwrite {
		// generate 模式：若 Xray 侧已存在同 email 账号，先移除旧账号再以新凭据覆盖。
		if lister, ok := client.(interface {
			GetInboundUsers(ctx context.Context, tag, email string) (*command.GetInboundUserResponse, error)
		}); ok {
			if resp, lerr := lister.GetInboundUsers(ctx, t.InboundTag, ExtEmail(extID)); lerr == nil && len(resp.GetUsers()) > 0 {
				if rerr := client.RemoveUser(ctx, t.InboundTag, ExtEmail(extID)); rerr != nil && !IsNotFound(rerr) {
					s.log.Warn("generate 覆盖前移除旧 Xray 账号失败", "ext_id", extID, "tag", t.InboundTag, "err", rerr)
				}
			}
		}
	}
	err = client.AddUser(ctx, t.InboundTag, user)
	if err != nil {
		if IsAlreadyExists(err) {
			if overwrite {
				// generate 模式：已存在时先移除旧账号，再以新凭据覆盖。
				if rerr := client.RemoveUser(ctx, t.InboundTag, ExtEmail(extID)); rerr != nil && !IsNotFound(rerr) {
					s.log.Warn("generate 覆盖移除旧 Xray 账号失败", "ext_id", extID, "tag", t.InboundTag, "err", rerr)
				}
				err = client.AddUser(ctx, t.InboundTag, user)
			} else {
				// manual 接管模式：已存在视为接管成功（Build7 Step1 口径）。
				err = nil
			}
		}
		if err != nil {
			s.markExtFailed(ctx, extID, t, err)
			return err
		}
	}
	s.markExtSynced(ctx, extID, t)
	return nil
}

func (s *ExtService) removeOne(ctx context.Context, extID int64, t ExtPushTarget) error {
	if err := s.removeFromXray(ctx, extID, t); err != nil {
		return err
	}
	_, err := s.store.DB().ExecContext(ctx,
		`DELETE FROM xray_ext_users WHERE ext_account_id = ? AND instance_id = ? AND inbound_tag = ?`,
		extID, t.InstanceID, t.InboundTag)
	return err
}

// removeFromXray 仅从 Xray 侧移除账号，不删除本地推送目标行；失败时记录 remove 状态。
func (s *ExtService) removeFromXray(ctx context.Context, extID int64, t ExtPushTarget) error {
	client, err := s.api(ctx, t.InstanceID)
	if err != nil {
		return err
	}
	err = client.RemoveUser(ctx, t.InboundTag, ExtEmail(extID))
	if err != nil && !IsNotFound(err) {
		s.markExtRemoveFailed(ctx, extID, t, err)
		return err
	}
	return nil
}

// removeQuotaExceededTarget 超限摘除专用：仅 RemoveUser，保留本地目标行并标记为 failed/add 以便重置后重推。
func (s *ExtService) removeQuotaExceededTarget(ctx context.Context, extID int64, t ExtPushTarget) error {
	client, err := s.api(ctx, t.InstanceID)
	if err != nil {
		return err
	}
	err = client.RemoveUser(ctx, t.InboundTag, ExtEmail(extID))
	if err != nil && !IsNotFound(err) {
		return err
	}
	s.markExtFailed(ctx, extID, t, errors.New("已超限，已从 Xray 移除（保留目标）"))
	return nil
}

func (s *ExtService) markExtSynced(ctx context.Context, extID int64, t ExtPushTarget) {
	_, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO xray_ext_users (ext_account_id, instance_id, inbound_tag, node_id, sync_status, action)
		 VALUES (?,?,?,?,'synced','add')
		 ON CONFLICT(ext_account_id, instance_id, inbound_tag) DO UPDATE SET
		   node_id = excluded.node_id, sync_status = 'synced', action = 'add', last_error = '', updated_at = CURRENT_TIMESTAMP`,
		extID, t.InstanceID, t.InboundTag, t.NodeID)
	if err != nil {
		s.log.Warn("更新独立账号 synced 失败", "ext_id", extID, "tag", t.InboundTag, "err", err)
	}
}

func (s *ExtService) markExtFailed(ctx context.Context, extID int64, t ExtPushTarget, err error) {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	_, dbErr := s.store.DB().ExecContext(ctx,
		`INSERT INTO xray_ext_users (ext_account_id, instance_id, inbound_tag, node_id, sync_status, action, last_error)
		 VALUES (?,?,?,?,'failed','add',?)
		 ON CONFLICT(ext_account_id, instance_id, inbound_tag) DO UPDATE SET
		   node_id = excluded.node_id, sync_status = 'failed', action = 'add', last_error = excluded.last_error, updated_at = CURRENT_TIMESTAMP`,
		extID, t.InstanceID, t.InboundTag, t.NodeID, msg)
	if dbErr != nil {
		s.log.Warn("更新独立账号 failed 失败", "ext_id", extID, "tag", t.InboundTag, "err", dbErr)
	}
}

// markExtRemoveFailed 记录“移除失败”的独立账号推送目标，供 RetryExt 按 action=remove 重试移除。
func (s *ExtService) markExtRemoveFailed(ctx context.Context, extID int64, t ExtPushTarget, err error) {
	msg := err.Error()
	if len(msg) > 200 {
		msg = msg[:200]
	}
	_, dbErr := s.store.DB().ExecContext(ctx,
		`INSERT INTO xray_ext_users (ext_account_id, instance_id, inbound_tag, node_id, sync_status, action, last_error)
		 VALUES (?,?,?,?,'failed','remove',?)
		 ON CONFLICT(ext_account_id, instance_id, inbound_tag) DO UPDATE SET
		   node_id = excluded.node_id, sync_status = 'failed', action = 'remove', last_error = excluded.last_error, updated_at = CURRENT_TIMESTAMP`,
		extID, t.InstanceID, t.InboundTag, t.NodeID, msg)
	if dbErr != nil {
		s.log.Warn("更新独立账号移除失败状态失败", "ext_id", extID, "tag", t.InboundTag, "err", dbErr)
	}
}

func extNodeView(ctx context.Context, st *store.Store, nodeID int64) (NodeView, error) {
	var protocol, rawJSON string
	err := st.DB().QueryRowContext(ctx,
		`SELECT protocol, protocol_json FROM nodes WHERE id = ?`, nodeID).Scan(&protocol, &rawJSON)
	if err != nil {
		return NodeView{}, err
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(rawJSON), &m); err != nil {
		return NodeView{}, err
	}
	nv := NodeView{Protocol: protocol}
	if v, ok := m["cipher"].(string); ok {
		nv.Cipher = v
	}
	if v, ok := m["flow"].(string); ok {
		nv.Flow = v
	}
	return nv, nil
}

// BuildExtUser 构造独立账号 Xray User。
func BuildExtUser(extID int64, uuid, proxySecret string, node NodeView) (*protocol.User, error) {
	account, err := accountOf(uuid, proxySecret, node)
	if err != nil {
		return nil, err
	}
	return &protocol.User{Level: 0, Email: ExtEmail(extID), Account: serial.ToTypedMessage(account)}, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("生成随机串失败: " + err.Error())
	}
	return hex.EncodeToString(b)
}
