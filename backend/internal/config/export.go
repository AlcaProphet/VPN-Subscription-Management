// config/export.go：配置导入导出服务（Build3 Step 4）——仅 Production 模式提供（Design1 §3.4.8/6.2）。
// 导出：Argon2id 派生密钥 + AES-256-GCM 加密整个配置文件；导入：事务内严格整体覆盖（先清空再写入）。
package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"vpn-sub/internal/store"
	"vpn-sub/internal/tasks"
)

const (
	ConfirmWordImport  = "IMPORT"  // 配置导入确认词（固定，二次确认由前端负责）
	ConfirmWordDisable = "DISABLE" // 导入清空高级模式数据分支的第二确认词
	FormatVersion      = 2         // 导出格式版本
	MinExportPassword  = 8         // 导出密码 ≥8 字符
)

// ErrModeRestricted 配置导入导出仅 Production 模式提供（接入层映射 403，R07-06）
var ErrModeRestricted = errors.New("配置导入导出仅 Production 模式提供")

// ExportedNodeName 导出实例的节点命名映射。
type ExportedNodeName struct {
	Tag         string `json:"tag"`
	DisplayName string `json:"display_name,omitempty"`
}

// ExportedInstance 导出实例。
type ExportedInstance struct {
	Name     string             `json:"name"`
	Slug     string             `json:"slug"`
	APIAddr  string             `json:"api_addr"`
	APITag   string             `json:"api_tag"`
	Enabled  bool               `json:"enabled"`
	Nodes    []ExportedNodeName `json:"nodes,omitempty"`
}

// ExportedExtPushTarget 导出独立账号推送目标。
type ExportedExtPushTarget struct {
	InstanceSlug string `json:"instance_slug"`
	InboundTag   string `json:"inbound_tag"`
}

// ExportedExtAccount 导出独立账号。
type ExportedExtAccount struct {
	Name                 string                  `json:"name"`
	Email                string                  `json:"email"`
	UUIDEncrypted        string                  `json:"uuid_encrypted"`
	ProxySecretEncrypted string                  `json:"proxy_secret_encrypted"`
	Quota                *float64                `json:"quota,omitempty"`
	QuotaExceeded        bool                    `json:"quota_exceeded"`
	PushTargets          []ExportedExtPushTarget `json:"push_targets,omitempty"`
}

// ExportPayload 导出内容（不含业务数据与日志）
type ExportPayload struct {
	FormatVersion int                  `json:"format_version"`
	ExportedAt    time.Time            `json:"exported_at"`
	SourceMode    string               `json:"source_mode"`
	Config        map[string]string    `json:"config"`           // 全部系统配置（含签名密钥与全部敏感密文，原样导出）
	SiteName      string               `json:"site_name"`        // 站点名称
	SiteIconB64   string               `json:"site_icon_base64"` // ICON base64 内嵌（可空）
	Instances     []ExportedInstance   `json:"instances,omitempty"`
	Accounts      []ExportedExtAccount `json:"accounts,omitempty"`
}

// ExportService 配置导入导出服务
type ExportService struct {
	store    *store.Store
	cfg      *Service
	dataDir  string
	mode     string // APP_MODE：导入导出仅 Production 提供
	log      *slog.Logger
	registry *tasks.Registry
	// seedPresets Setup 导入分支预置默认组/平台（由 server.New 注入 setup.SeedPresetsTx；避免 config↔setup 循环依赖）
	seedPresets func(ctx context.Context, tx *sql.Tx, frontendURL string) error

	// v2 导入后处理钩子（均由 server.New 注入，避免 config 包反向依赖 xray/assembly/server）
	cleanupXrayTargets      func(ctx context.Context, targets []ImportCleanupTarget)
	detectImportedInstances func(ctx context.Context, payload *ExportPayload) []string
	postImportRebindReconcile func(ctx context.Context, payload *ExportPayload) []string
}

// ImportCleanupTarget 是导入覆盖前需要从 Xray 侧清理的旧账号快照。
type ImportCleanupTarget struct {
	Email   string
	Tag     string
	APIAddr string
}

func NewExportService(st *store.Store, cfg *Service, dataDir, mode string, lg *slog.Logger) *ExportService {
	return &ExportService{store: st, cfg: cfg, dataDir: dataDir, mode: mode, log: lg}
}

// SetTaskRegistry 注入全局长任务 registry（v2 导入异步任务使用）。
func (s *ExportService) SetTaskRegistry(reg *tasks.Registry) {
	s.registry = reg
}

// SetSeedPresets 注入 Setup 预置逻辑（Setup 导入分支使用）
func (s *ExportService) SetSeedPresets(fn func(ctx context.Context, tx *sql.Tx, frontendURL string) error) {
	s.seedPresets = fn
}

// SetCleanupXrayTargets 注入 v2 导入覆盖后对旧 Xray 账号的 best-effort 清理函数。
func (s *ExportService) SetCleanupXrayTargets(fn func(ctx context.Context, targets []ImportCleanupTarget)) {
	s.cleanupXrayTargets = fn
}

// SetDetectImportedInstances 注入 v2 导入提交后的节点自动检测函数，返回完成提示（如跳过实例）。
func (s *ExportService) SetDetectImportedInstances(fn func(ctx context.Context, payload *ExportPayload) []string) {
	s.detectImportedInstances = fn
}

// SetPostImportRebindReconcile 注入 v2 导入提交后的装配快照重绑与对账函数，返回完成提示/错误说明。
func (s *ExportService) SetPostImportRebindReconcile(fn func(ctx context.Context, payload *ExportPayload) []string) {
	s.postImportRebindReconcile = fn
}

// Export 导出：导出密码（≥8）→ Argon2id 派生密钥 + AES-256-GCM 加密整个配置文件 → 返回密文供下载。
// 内容含全部系统配置（含签名密钥与敏感密文——密文原样导出，导入侧原样落库）+ 站点信息（ICON base64）
func (s *ExportService) Export(ctx context.Context, password string) ([]byte, error) {
	if s.mode != "prod" {
		return nil, ErrModeRestricted
	}
	if utf8.RuneCountInString(password) < MinExportPassword {
		return nil, fmt.Errorf("%w: 导出密码至少 8 字符", ErrBadRequest)
	}
	rows, err := s.store.DB().QueryContext(ctx, `SELECT key, value FROM system_config`)
	if err != nil {
		return nil, fmt.Errorf("读取系统配置失败: %w", err)
	}
	cfgMap := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("解析配置行失败: %w", err)
		}
		cfgMap[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	payload := ExportPayload{
		FormatVersion: FormatVersion,
		ExportedAt:    time.Now(),
		SourceMode:    s.mode,
		Config:        cfgMap,
		SiteName:      mustStr(s.cfg.Get(ctx, "site_name")),
	}
	// v2：导出 Xray 实例与独立账号（表不存在时跳过，兼容最小测试库）。
	if s.hasTable(ctx, "xray_instances") {
		payload.Instances, _ = s.exportInstances(ctx)
	}
	if s.hasTable(ctx, "xray_ext_accounts") {
		payload.Accounts, _ = s.exportAccounts(ctx)
	}
	// ICON base64 内嵌（/public/site/icon.*）
	if iconData, err := readSiteIcon(s.dataDir); err == nil {
		payload.SiteIconB64 = base64.StdEncoding.EncodeToString(iconData)
	} else {
		s.log.Warn("读取站点 ICON 失败（跳过内嵌）", "err", err)
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化导出内容失败: %w", err)
	}
	// Argon2id 派生密钥 + AES-256-GCM 加密（salt 随机，输出格式：salt ‖ nonce ‖ 密文）
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成 salt 失败: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32) // Argon2id：time=1, memory=64MB, threads=4
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}
	out := append(salt, nonce...)
	out = gcm.Seal(out, nonce, plain, nil)
	s.log.Warn("配置导出已执行") // 记 warn 级日志（不记录密码与文件内容）
	return out, nil
}

// Import 导入：上传文件 + 导出密码解密 → 校验格式与版本 → 事务内整体覆盖（严格整体覆盖语义，Design1 §3.4.8）。
// setupMode=true 时（Setup 入口）同事务创建预置默认组与默认平台
func (s *ExportService) Import(ctx context.Context, data []byte, password, confirmWord string, setupMode bool) error {
	if s.mode != "prod" {
		return ErrModeRestricted
	}
	if confirmWord != ConfirmWordImport {
		return errors.New("确认词不正确")
	}
	payload, err := s.decrypt(data, password)
	if err != nil {
		return err // 「密码错误或文件损坏」
	}
	// 校验格式与版本：format_version 不匹配仅警告不阻断；未知键忽略并警告；校验失败不做任何变更
	if payload.FormatVersion != FormatVersion {
		s.log.Warn("导入配置 format_version 不匹配", "got", payload.FormatVersion, "want", FormatVersion)
	}
	// 事务内整体覆盖：先清空全部现有配置键再写入导出内容（导出文件中不存在的键一并清除——严格整体覆盖）
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM system_config`); err != nil {
			return err
		}
		for k, v := range payload.Config {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO system_config (key, value) VALUES (?, ?)`, k, v); err != nil {
				return fmt.Errorf("写入配置键 %s 失败: %w", k, err)
			}
		}
		// 未配置状态导入（Setup 分支）：同事务创建预置默认组与默认平台（预置数据为导入流程固定动作）
		if setupMode {
			if s.seedPresets == nil {
				return errors.New("Setup 预置逻辑未注入")
			}
			if err := s.seedPresets(ctx, tx, ""); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务提交后：ICON 写入 /public/site/（base64 解码落盘；失败记日志不阻断）
	if payload.SiteIconB64 != "" {
		if err := s.writeSiteIcon(payload.SiteIconB64); err != nil {
			s.log.Warn("导入站点 ICON 失败", "err", err)
		}
	}
	// 导入后效果：签名密钥替换 → 全部现有会话立即失效（含执行导入的管理员，前端清凭据跳登录）；
	// 含前端地址/回调地址时需重启生效——UI 提示「导入完成 → 立即重启容器 → 再重新登录」
	s.log.Warn("配置导入已执行", "setup_mode", setupMode)
	return nil
}

// ImportV2 导入入口：v1 保持同步兼容；v2 返回异步任务 ID。
// disableConfirmWord 用于“无实例/账号且 advanced_mode=false”分支的 DISABLE 第二确认词。
func (s *ExportService) ImportV2(ctx context.Context, data []byte, password, confirmWord, disableConfirmWord string, setupMode bool) (string, error) {
	if s.mode != "prod" {
		return "", ErrModeRestricted
	}
	payload, err := s.decrypt(data, password)
	if err != nil {
		return "", err
	}
	if payload.FormatVersion != FormatVersion {
		// v1 兼容：保持旧同步语义。
		if err := s.Import(ctx, data, password, confirmWord, setupMode); err != nil {
			return "", err
		}
		return "", nil
	}
	if confirmWord != ConfirmWordImport {
		return "", errors.New("确认词不正确")
	}
	// 无实例/账号且高级模式关闭：按 OFF 清空口径清理旧高级数据，需要额外的 DISABLE 确认。
	hasAdvancedData := len(payload.Instances) > 0 || len(payload.Accounts) > 0
	if !hasAdvancedData && payload.Config[KeyAdvancedMode] != "true" && disableConfirmWord != ConfirmWordDisable {
		return "", errors.New("该导入会清空高级模式数据，请输入 DISABLE 确认")
	}
	if s.registry == nil {
		return "", errors.New("任务注册表未注入")
	}
	taskID := s.registry.Register(tasks.KindImport)
	bg := context.WithoutCancel(ctx)
	go func() {
		if err := s.importV2(bg, payload, confirmWord, setupMode); err != nil {
			s.registry.Fail(taskID, err.Error())
			return
		}
		s.registry.Succeed(taskID, map[string]any{"imported": true})
	}()
	return taskID, nil
}

// importV2 在任务体内执行 v2 导入。
func (s *ExportService) importV2(ctx context.Context, payload *ExportPayload, confirmWord string, setupMode bool) error {
	if confirmWord != ConfirmWordImport {
		return errors.New("确认词不正确")
	}
	// 导入保护：signing_key 变化且存在业务密文时拒绝。
	if err := s.checkImportProtection(ctx, payload); err != nil {
		return err
	}
	var oldTargets []ImportCleanupTarget
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM system_config`); err != nil {
			return err
		}
		for k, v := range payload.Config {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO system_config (key, value) VALUES (?, ?)`, k, v); err != nil {
				return fmt.Errorf("写入配置键 %s 失败: %w", k, err)
			}
		}
		// 收集旧 Xray 推送快照（在删除实例/账号前）
		if s.hasTableTx(ctx, tx, "xray_instances") {
			rows, err := tx.QueryContext(ctx,
				`SELECT xu.email, xu.inbound_tag, i.api_addr
				 FROM xray_users xu JOIN xray_instances i ON i.id = xu.instance_id
				 UNION ALL
				 SELECT a.email, xu.inbound_tag, i.api_addr
				 FROM xray_ext_users xu
				 JOIN xray_ext_accounts a ON a.id = xu.ext_account_id
				 JOIN xray_instances i ON i.id = xu.instance_id`)
			if err != nil {
				return err
			}
			for rows.Next() {
				var t ImportCleanupTarget
				if err := rows.Scan(&t.Email, &t.Tag, &t.APIAddr); err != nil {
					_ = rows.Close()
					return err
				}
				oldTargets = append(oldTargets, t)
			}
			if err := rows.Close(); err != nil {
				return err
			}
		}
		// 高级模式一致性
		advanced := payload.Config[KeyAdvancedMode]
		hasAdvancedData := len(payload.Instances) > 0 || len(payload.Accounts) > 0
		if hasAdvancedData && advanced != "true" {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO system_config (key, value) VALUES (?, 'true') ON CONFLICT(key) DO UPDATE SET value = 'true'`, KeyAdvancedMode); err != nil {
				return err
			}
		}
		if !hasAdvancedData && advanced != "true" && s.hasTableTx(ctx, tx, "xray_instances") {
			// 无实例/账号且高级关闭：按 OFF 清空口径清理高级数据。
			if _, err := tx.ExecContext(ctx, `DELETE FROM xray_instances`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM xray_ext_accounts`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM traffic_records`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM xray_ext_traffic`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE users SET uuid_encrypted = NULL, proxy_secret_encrypted = NULL, quota_override = NULL, quota_exceeded = 0`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE groups SET default_quota = NULL`); err != nil {
				return err
			}
		}
		// 重建实例
		if s.hasTableTx(ctx, tx, "xray_instances") {
			if _, err := tx.ExecContext(ctx, `DELETE FROM xray_instances`); err != nil {
				return err
			}
			for _, inst := range payload.Instances {
				if _, err := tx.ExecContext(ctx,
					`INSERT INTO xray_instances (name, slug, api_addr, api_tag, enabled) VALUES (?,?,?,?,?)`,
					inst.Name, inst.Slug, inst.APIAddr, inst.APITag, boolToInt(inst.Enabled)); err != nil {
					return fmt.Errorf("导入实例失败: %w", err)
				}
			}
		}
		// 重建独立账号
		if s.hasTableTx(ctx, tx, "xray_ext_accounts") {
			if _, err := tx.ExecContext(ctx, `DELETE FROM xray_ext_accounts`); err != nil {
				return err
			}
			for _, acc := range payload.Accounts {
				res, err := tx.ExecContext(ctx,
					`INSERT INTO xray_ext_accounts (name, email, uuid_encrypted, proxy_secret_encrypted, quota, quota_exceeded)
					 VALUES (?,?,?,?,?,?)`,
					acc.Name, acc.Email, acc.UUIDEncrypted, acc.ProxySecretEncrypted, acc.Quota, boolToInt(acc.QuotaExceeded))
				if err != nil {
					return fmt.Errorf("导入独立账号失败: %w", err)
				}
				id, err := res.LastInsertId()
				if err != nil {
					return err
				}
				for _, pt := range acc.PushTargets {
					var instID int64
					if err := tx.QueryRowContext(ctx,
						`SELECT id FROM xray_instances WHERE slug = ?`, pt.InstanceSlug).Scan(&instID); err != nil {
						if err == sql.ErrNoRows {
							continue
						}
						return err
					}
					if _, err := tx.ExecContext(ctx,
						`INSERT INTO xray_ext_users (ext_account_id, instance_id, inbound_tag, node_id, sync_status)
						 VALUES (?,?,?,NULL,'pending')`, id, instID, pt.InboundTag); err != nil {
						return fmt.Errorf("导入独立账号推送目标失败: %w", err)
					}
				}
			}
		}
		if setupMode {
			if s.seedPresets == nil {
				return errors.New("Setup 预置逻辑未注入")
			}
			if err := s.seedPresets(ctx, tx, ""); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if payload.SiteIconB64 != "" {
		if err := s.writeSiteIcon(payload.SiteIconB64); err != nil {
			s.log.Warn("导入站点 ICON 失败", "err", err)
		}
	}
	hints := []string{}
	if s.cleanupXrayTargets != nil && len(oldTargets) > 0 {
		s.cleanupXrayTargets(ctx, oldTargets)
		hints = append(hints, fmt.Sprintf("已执行旧 Xray 账号清理，共 %d 个目标", len(oldTargets)))
	}
	if s.detectImportedInstances != nil {
		hints = append(hints, s.detectImportedInstances(ctx, payload)...)
	}
	if err := s.rebindImportedDisplayNames(ctx, payload); err != nil {
		hints = append(hints, "节点显示名回填失败: "+err.Error())
	}
	if err := s.rebindImportedExtNodes(ctx, payload); err != nil {
		hints = append(hints, "独立账号推送目标重绑失败: "+err.Error())
	}
	if s.postImportRebindReconcile != nil {
		hints = append(hints, s.postImportRebindReconcile(ctx, payload)...)
	}
	s.log.Warn("配置导入已执行", "setup_mode", setupMode, "format_version", payload.FormatVersion, "hints", hints)
	return nil
}

// checkImportProtection 导入保护：signing_key 将变化且存在业务密文时拒绝。

// rebindImportedDisplayNames 在自动检测后，将导出文件中的节点显示名映射回填到新建节点。
func (s *ExportService) rebindImportedDisplayNames(ctx context.Context, payload *ExportPayload) error {
	if !s.hasTable(ctx, "nodes") {
		return nil
	}
	for _, inst := range payload.Instances {
		for _, nd := range inst.Nodes {
			if nd.DisplayName == "" {
				continue
			}
			if _, err := s.store.DB().ExecContext(ctx,
				`UPDATE nodes SET display_name = ?, updated_at = CURRENT_TIMESTAMP
				 WHERE instance_id = (SELECT id FROM xray_instances WHERE slug = ?) AND tag = ? AND source = 'xray'`,
				nd.DisplayName, inst.Slug, nd.Tag); err != nil {
				return err
			}
		}
	}
	return nil
}

// rebindImportedExtNodes 将导入的独立账号推送目标按 (instance slug, inbound tag) 重绑到检测后的 node_id。
func (s *ExportService) rebindImportedExtNodes(ctx context.Context, payload *ExportPayload) error {
	if !s.hasTable(ctx, "xray_ext_users") || !s.hasTable(ctx, "xray_ext_accounts") {
		return nil
	}
	for _, acc := range payload.Accounts {
		var accID int64
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT id FROM xray_ext_accounts WHERE email = ?`, acc.Email).Scan(&accID); err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return err
		}
		for _, pt := range acc.PushTargets {
			var instID int64
			if err := s.store.DB().QueryRowContext(ctx,
				`SELECT id FROM xray_instances WHERE slug = ?`, pt.InstanceSlug).Scan(&instID); err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				return err
			}
			if _, err := s.store.DB().ExecContext(ctx,
				`UPDATE xray_ext_users SET node_id = (
				   SELECT id FROM nodes WHERE instance_id = ? AND tag = ? AND source = 'xray' LIMIT 1
				 )
				 WHERE ext_account_id = ? AND instance_id = ? AND inbound_tag = ?`,
				instID, pt.InboundTag, accID, instID, pt.InboundTag); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ExportService) checkImportProtection(ctx context.Context, payload *ExportPayload) error {
	newKey, _ := payload.Config[KeySigningKey]
	currentKey, _ := s.cfg.GetRaw(ctx, KeySigningKey)
	if newKey == "" || newKey == currentKey {
		return nil
	}
	var count int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE (uuid_encrypted IS NOT NULL AND uuid_encrypted != '') OR (proxy_secret_encrypted IS NOT NULL AND proxy_secret_encrypted != '')`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return errors.New("配置导入仅适用全新部署/同密钥往返，在用实例请使用备份恢复")
	}
	if s.hasTable(ctx, "nodes") {
		var nodeCount int
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM nodes WHERE protocol_json LIKE '%enc:v1:%'`).Scan(&nodeCount); err != nil {
			return err
		}
		count += nodeCount
	}
	if s.hasTable(ctx, "xray_ext_accounts") {
		var extCount int
		if err := s.store.DB().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM xray_ext_accounts WHERE (uuid_encrypted IS NOT NULL AND uuid_encrypted != '') OR (proxy_secret_encrypted IS NOT NULL AND proxy_secret_encrypted != '')`).Scan(&extCount); err != nil {
			return err
		}
		count += extCount
	}
	if count > 0 {
		return errors.New("配置导入仅适用全新部署/同密钥往返，在用实例请使用备份恢复")
	}
	return nil
}

// decrypt 解密导入文件（Argon2id + AES-GCM 逆过程）；失败返回「密码错误或文件损坏」
func (s *ExportService) decrypt(data []byte, password string) (*ExportPayload, error) {
	if len(data) < 16+12 {
		return nil, errors.New("密码错误或文件损坏")
	}
	salt, rest := data[:16], data[16:]
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	plain, err := gcm.Open(nil, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], nil)
	if err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	var payload ExportPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	if payload.Config == nil {
		return nil, errors.New("导入文件内容无效")
	}
	return &payload, nil
}

func (s *ExportService) hasTable(ctx context.Context, name string) bool {
	var n int
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	return err == nil && n > 0
}

func (s *ExportService) hasTableTx(ctx context.Context, tx *sql.Tx, name string) bool {
	var n int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n)
	return err == nil && n > 0
}

func (s *ExportService) exportInstances(ctx context.Context) ([]ExportedInstance, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT name, slug, api_addr, api_tag, enabled FROM xray_instances ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ExportedInstance, 0)
	for rows.Next() {
		var inst ExportedInstance
		var enabled int
		if err := rows.Scan(&inst.Name, &inst.Slug, &inst.APIAddr, &inst.APITag, &enabled); err != nil {
			return nil, err
		}
		inst.Enabled = enabled == 1
		inst.Nodes = []ExportedNodeName{}
		out = append(out, inst)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 节点命名映射
	for i := range out {
		nrows, err := s.store.DB().QueryContext(ctx,
			`SELECT tag, COALESCE(display_name,'') FROM nodes WHERE instance_id = (SELECT id FROM xray_instances WHERE slug = ?) AND source = 'xray' AND display_name IS NOT NULL AND display_name != ''`, out[i].Slug)
		if err != nil {
			return nil, err
		}
		for nrows.Next() {
			var node ExportedNodeName
			if err := nrows.Scan(&node.Tag, &node.DisplayName); err != nil {
				_ = nrows.Close()
				return nil, err
			}
			out[i].Nodes = append(out[i].Nodes, node)
		}
		if err := nrows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *ExportService) exportAccounts(ctx context.Context) ([]ExportedExtAccount, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT name, email, COALESCE(uuid_encrypted,''), COALESCE(proxy_secret_encrypted,''), quota, quota_exceeded
		 FROM xray_ext_accounts ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ExportedExtAccount, 0)
	for rows.Next() {
		var acc ExportedExtAccount
		var quota sql.NullFloat64
		var exceeded int
		if err := rows.Scan(&acc.Name, &acc.Email, &acc.UUIDEncrypted, &acc.ProxySecretEncrypted, &quota, &exceeded); err != nil {
			return nil, err
		}
		if quota.Valid {
			acc.Quota = &quota.Float64
		}
		acc.QuotaExceeded = exceeded == 1
		acc.PushTargets = []ExportedExtPushTarget{}
		out = append(out, acc)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		prows, err := s.store.DB().QueryContext(ctx,
			`SELECT i.slug, xu.inbound_tag FROM xray_ext_users xu
			 JOIN xray_instances i ON i.id = xu.instance_id
			 WHERE xu.ext_account_id = (SELECT id FROM xray_ext_accounts WHERE email = ?)`, out[i].Email)
		if err != nil {
			return nil, err
		}
		for prows.Next() {
			var pt ExportedExtPushTarget
			if err := prows.Scan(&pt.InstanceSlug, &pt.InboundTag); err != nil {
				_ = prows.Close()
				return nil, err
			}
			out[i].PushTargets = append(out[i].PushTargets, pt)
		}
		if err := prows.Close(); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// readSiteIcon 读取站点 ICON 文件（/public/site/icon.*；不存在返回错误）
func readSiteIcon(dataDir string) ([]byte, error) {
	matches, err := filepath.Glob(filepath.Join(dataDir, "public", "site", "icon.*"))
	if err != nil || len(matches) == 0 {
		return nil, errors.New("站点 ICON 不存在")
	}
	return os.ReadFile(matches[0])
}

// writeSiteIcon base64 解码落盘 /public/site/（固定路径覆盖即更新；版本号递增供前端刷新）
func (s *ExportService) writeSiteIcon(b64 string) error {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("ICON base64 解码失败: %w", err)
	}
	// 扩展名推断：仅按现有文件/默认 png（导入侧不依赖文件名，固定 icon.png 亦可）
	ext := "png"
	if matches, gerr := filepath.Glob(filepath.Join(s.dataDir, "public", "site", "icon.*")); gerr == nil && len(matches) > 0 {
		if e := filepath.Ext(matches[0]); e != "" {
			ext = strings.TrimPrefix(e, ".")
		}
	}
	full := filepath.Join(s.dataDir, "public", "site", "icon."+ext)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return fmt.Errorf("写入 ICON 失败: %w", err)
	}
	ver := s.cfg.GetInt(context.Background(), "site_icon_version", 0) + 1
	if err := s.cfg.Set(context.Background(), "site_icon_version", fmt.Sprint(ver)); err != nil {
		return err
	}
	return s.cfg.Set(context.Background(), "site_icon_url", "/public/site/icon."+ext+"?v="+fmt.Sprint(ver))
}
