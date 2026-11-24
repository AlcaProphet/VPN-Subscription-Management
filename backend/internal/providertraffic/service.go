// Package providertraffic 提供基础模式服务器剩余流量的绑定管理与采集。
package providertraffic

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

// Provider 是固定的服务商枚举；顺序固定为 kiwivm → vps。
type Provider string

const (
	ProviderKiwiVM Provider = "kiwivm"
	ProviderVPS    Provider = "vps"
)

var (
	// ErrUnknownProvider 表示服务商枚举不在固定范围内。
	ErrUnknownProvider = errors.New("未知的服务商")
	// ErrNotBound 表示当前服务商尚未绑定实例。
	ErrNotBound = errors.New("服务商未绑定")
	// ErrInvalidInstanceID 表示实例 ID 不是规范的正十进制数字字符串。
	ErrInvalidInstanceID = errors.New("实例 ID 必须为正十进制数字")
	// ErrInvalidCredential 表示显式提交的凭据为空串、纯空白或 null。
	ErrInvalidCredential = errors.New("凭据必须为非空字符串")
	// ErrCredentialRequired 表示首次绑定或更换实例时未提交凭据。
	ErrCredentialRequired = errors.New("首次绑定或更换实例必须填写凭据")
	// ErrRevisionConflict 表示绑定修订缺失、为空或与当前绑定不匹配。
	ErrRevisionConflict    = errors.New("绑定修订冲突")
	ErrInvalidHostName     = errors.New("主机名称必须为 1 到 64 个字符")
	ErrDuplicateInstance   = errors.New("该实例已添加")
	ErrVPSAccountRequired  = errors.New("请先保存 V.PS 账号凭据")
	ErrVPSHostsExist       = errors.New("请先解除所有 V.PS 主机绑定")
	ErrVPSSharedCredential = errors.New("请在 V.PS 账号设置中管理共享凭据")
)

// CredentialInput 表示一次绑定保存请求中的凭据字段。
// Omitted 为 true 表示请求省略该字段（同一实例保留现有密文）；
// 未省略时 Null 或纯空白 Value 均视为无效输入。
type CredentialInput struct {
	Omitted bool
	Null    bool
	Value   string
}

// SaveBindingInput 是保存单家绑定的入参。
// Revision 为 nil 表示 JSON null；首次绑定必须为 nil，修改或解除绑定必须提交当前非空修订。
type SaveBindingInput struct {
	Provider   Provider
	Enabled    bool
	InstanceID string
	Revision   *string
	Credential CredentialInput
}

// ProviderBinding 是单家已提交的本地状态；不包含凭据密文。
type ProviderBinding struct {
	ID                   string
	Name                 string
	HalfRemaining        bool
	EditRevision         string
	Provider             Provider
	Enabled              bool
	InstanceID           string
	Revision             string
	CredentialConfigured bool
}

// Settings 是两家固定顺序的本地设置；无功能表记录时返回默认关闭且未绑定。
type Settings struct {
	Enabled     bool
	Providers   []ProviderBinding
	Hosts       []ProviderBinding
	VPSRevision string
}

type hostKey struct {
	provider Provider
	id       string
}

func legacyKey(provider Provider) hostKey {
	return hostKey{provider: provider, id: string(provider)}
}

func (b ProviderBinding) key() hostKey {
	return hostKey{provider: b.Provider, id: b.ID}
}

// SaveHostInput 保存一个主机。ID 为空时新建，修改时校验编辑修订。
type SaveHostInput struct {
	Provider      Provider
	ID            string
	Name          string
	Enabled       bool
	HalfRemaining bool
	InstanceID    string
	Revision      *string
	Credential    CredentialInput
}

type settingsChange struct {
	before Settings
	after  Settings
}

// Service 是服务器剩余流量的本地存储服务。
type Service struct {
	store *store.Store
	cfg   *config.Service
	log   *slog.Logger
	// newRevision 生成不可复用的绑定修订号；测试可替换。
	newRevision func() (string, error)
}

// NewService 构造本地存储服务。
func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{store: st, cfg: cfg, log: lg, newRevision: generateRevision}
}

// GetSettings 读取已提交的本地设置；无记录视为总开关关闭且两家未绑定。
func (s *Service) GetSettings(ctx context.Context) (Settings, error) {
	return readSettings(ctx, s.store.DB())
}

// SetEnabled 只保存总开关；正常服务的设置入口使用 Collector.SetEnabled 交接调度。
func (s *Service) SetEnabled(ctx context.Context, enabled bool) (Settings, error) {
	change, err := s.setEnabled(ctx, enabled)
	return change.after, err
}

func (s *Service) setEnabled(ctx context.Context, enabled bool) (settingsChange, error) {
	return s.changeSettings(ctx, func(tx *sql.Tx) error {
		if err := ensureSettingsRow(ctx, tx); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE provider_traffic_settings SET enabled = ? WHERE id = 1`, boolToInt(enabled)); err != nil {
			return fmt.Errorf("保存服务器剩余流量总开关失败: %w", err)
		}
		return nil
	})
}

// SaveBinding 在单个 TxImmediate 事务内完成当前绑定读取、修订校验、凭据加密和写入。
// 首次绑定要求 revision=null 且提交凭据；已有绑定按编辑修订校验。
// 正常服务的设置入口使用 Collector.SaveBinding 交接调度。
func (s *Service) SaveBinding(ctx context.Context, in SaveBindingInput) (Settings, error) {
	change, err := s.saveBinding(ctx, in)
	return change.after, err
}

func (s *Service) saveBinding(ctx context.Context, in SaveBindingInput) (settingsChange, error) {
	return s.saveHost(ctx, SaveHostInput{
		Provider: in.Provider, ID: string(in.Provider), Enabled: in.Enabled,
		InstanceID: in.InstanceID, Revision: in.Revision, Credential: in.Credential,
	}, true)
}

func (s *Service) saveHost(ctx context.Context, in SaveHostInput, legacy bool) (settingsChange, error) {
	if providerIndex(in.Provider) < 0 {
		return settingsChange{}, ErrUnknownProvider
	}
	if !validInstanceID(in.InstanceID) {
		return settingsChange{}, ErrInvalidInstanceID
	}
	in.Name = strings.TrimSpace(in.Name)
	if !legacy && (in.Name == "" || utf8.RuneCountInString(in.Name) > 64 || strings.ContainsRune(in.Name, 0)) {
		return settingsChange{}, ErrInvalidHostName
	}
	if err := validateCredentialInput(in.Credential); err != nil {
		return settingsChange{}, err
	}
	if !legacy && in.Provider == ProviderVPS && !in.Credential.Omitted {
		return settingsChange{}, ErrInvalidCredential
	}
	return s.changeSettings(ctx, func(tx *sql.Tx) error {
		if err := ensureSettingsRow(ctx, tx); err != nil {
			return err
		}
		key := hostKey{provider: in.Provider, id: in.ID}
		current, err := readHost(ctx, tx, key)
		if err != nil {
			return err
		}
		creating := current.ID == ""
		if legacy {
			in.Name = current.Name
			in.HalfRemaining = current.HalfRemaining
			if creating {
				in.Name = providerLabel(in.Provider)
			}
		}
		if creating {
			if in.Revision != nil || (!legacy && in.ID != "") {
				return ErrRevisionConflict
			}
			if !legacy {
				key.id, err = generateRevision()
				if err != nil {
					return fmt.Errorf("生成主机标识失败: %w", err)
				}
			}
		} else if in.Revision == nil || *in.Revision != current.EditRevision {
			return ErrRevisionConflict
		}
		targetChanged := creating || current.InstanceID != in.InstanceID
		if (in.Provider == ProviderKiwiVM || legacy) && targetChanged && in.Credential.Omitted {
			return ErrCredentialRequired
		}

		if in.Provider == ProviderVPS {
			var account sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT vps_revision FROM provider_traffic_settings WHERE id = 1`).Scan(&account); err != nil {
				return fmt.Errorf("读取 V.PS 账号修订失败: %w", err)
			}
			if legacy && !in.Credential.Omitted {
				var otherHosts bool
				if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_traffic_hosts
					WHERE provider = 'vps' AND id <> ?)`, key.id).Scan(&otherHosts); err != nil {
					return fmt.Errorf("检查 V.PS 主机失败: %w", err)
				}
				if otherHosts {
					return ErrVPSSharedCredential
				}
			} else if !account.Valid {
				return ErrVPSAccountRequired
			}
		}

		revision := current.Revision
		if targetChanged || !in.Credential.Omitted {
			revision, err = s.newRevision()
			if err != nil {
				return fmt.Errorf("生成绑定修订失败: %w", err)
			}
		}
		editRevision := current.EditRevision
		if revision != current.Revision {
			editRevision = revision
		} else if in.Name != current.Name || in.Enabled != current.Enabled || in.HalfRemaining != current.HalfRemaining {
			editRevision, err = s.newRevision()
			if err != nil {
				return fmt.Errorf("生成设置修订失败: %w", err)
			}
		}
		ciphertext := current.Ciphertext
		if !in.Credential.Omitted {
			encrypted, err := s.cfg.EncryptWithTx(ctx, tx, in.Credential.Value)
			if err != nil {
				return fmt.Errorf("加密服务商凭据失败: %w", err)
			}
			if in.Provider == ProviderVPS {
				if err := writeVPSAccount(ctx, tx, encrypted, revision); err != nil {
					return err
				}
			} else {
				ciphertext = sql.NullString{String: encrypted, Valid: true}
			}
		}
		query := `INSERT INTO provider_traffic_hosts
			(provider, instance_id, name, enabled, half_remaining, credential_ciphertext, revision, edit_revision, id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
		if !creating {
			query = `UPDATE provider_traffic_hosts SET provider = ?, instance_id = ?, name = ?, enabled = ?,
				half_remaining = ?, credential_ciphertext = ?, revision = ?, edit_revision = ? WHERE id = ?`
		}
		_, err = tx.ExecContext(ctx, query, key.provider, in.InstanceID, in.Name, boolToInt(in.Enabled),
			boolToInt(in.HalfRemaining), ciphertext, revision, editRevision, key.id)
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
			return ErrDuplicateInstance
		}
		if err != nil {
			return fmt.Errorf("保存服务商主机失败: %w", err)
		}
		return nil
	})
}

func providerLabel(provider Provider) string {
	if provider == ProviderKiwiVM {
		return "搬瓦工"
	}
	return "V.PS"
}

// ClearBinding 解除旧版接口对应的单台主机。
func (s *Service) ClearBinding(ctx context.Context, provider Provider, revision string) (Settings, error) {
	change, err := s.clearHost(ctx, legacyKey(provider), revision, true)
	return change.after, err
}

func (s *Service) clearHost(ctx context.Context, key hostKey, revision string, legacy bool) (settingsChange, error) {
	if providerIndex(key.provider) < 0 {
		return settingsChange{}, ErrUnknownProvider
	}
	if revision == "" {
		return settingsChange{}, ErrRevisionConflict
	}
	return s.changeSettings(ctx, func(tx *sql.Tx) error {
		current, err := readHost(ctx, tx, key)
		if err != nil {
			return err
		}
		if current.EditRevision != revision {
			return ErrRevisionConflict
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM provider_traffic_hosts WHERE id = ?`, key.id); err != nil {
			return fmt.Errorf("解除服务商绑定失败: %w", err)
		}
		if legacy && key.provider == ProviderVPS {
			if _, err := tx.ExecContext(ctx, `UPDATE provider_traffic_settings
				SET vps_credential_ciphertext = NULL, vps_revision = NULL WHERE id = 1
				AND NOT EXISTS (SELECT 1 FROM provider_traffic_hosts WHERE provider = 'vps')`); err != nil {
				return fmt.Errorf("清除 V.PS 账号失败: %w", err)
			}
		}
		return nil
	})
}

func (s *Service) saveVPSAccount(ctx context.Context, revision, credential string) (settingsChange, error) {
	return s.changeSettings(ctx, func(tx *sql.Tx) error {
		if err := ensureSettingsRow(ctx, tx); err != nil {
			return err
		}
		var current sql.NullString
		if err := tx.QueryRowContext(ctx, `SELECT vps_revision FROM provider_traffic_settings WHERE id = 1`).Scan(&current); err != nil {
			return fmt.Errorf("读取 V.PS 账号修订失败: %w", err)
		}
		if current.String != revision {
			return ErrRevisionConflict
		}
		if credential == "" {
			var hasHosts bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM provider_traffic_hosts WHERE provider = 'vps')`).Scan(&hasHosts); err != nil {
				return fmt.Errorf("检查 V.PS 主机失败: %w", err)
			}
			if hasHosts {
				return ErrVPSHostsExist
			}
			return writeVPSAccount(ctx, tx, nil, nil)
		}
		ciphertext, err := s.cfg.EncryptWithTx(ctx, tx, credential)
		if err != nil {
			return fmt.Errorf("加密 V.PS 账号凭据失败: %w", err)
		}
		next, err := s.newRevision()
		if err != nil {
			return fmt.Errorf("生成 V.PS 账号修订失败: %w", err)
		}
		if err := writeVPSAccount(ctx, tx, ciphertext, next); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE provider_traffic_hosts SET revision = lower(hex(randomblob(16))),
			edit_revision = lower(hex(randomblob(16))) WHERE provider = 'vps'`); err != nil {
			return fmt.Errorf("更新 V.PS 主机修订失败: %w", err)
		}
		return nil
	})
}

func writeVPSAccount(ctx context.Context, tx *sql.Tx, ciphertext, revision any) error {
	if _, err := tx.ExecContext(ctx, `UPDATE provider_traffic_settings
		SET vps_credential_ciphertext = ?, vps_revision = ? WHERE id = 1`, ciphertext, revision); err != nil {
		return fmt.Errorf("保存 V.PS 账号凭据失败: %w", err)
	}
	return nil
}

// changeSettings 在同一写事务内验证模式并记录变更前后状态，只交付成功提交的变化。
func (s *Service) changeSettings(ctx context.Context, write func(*sql.Tx) error) (settingsChange, error) {
	var change settingsChange
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		available, err := s.basicModeAvailableTx(ctx, tx)
		if err != nil {
			return err
		}
		if !available {
			return ErrModeUnavailable
		}
		before, err := readSettings(ctx, tx)
		if err != nil {
			return err
		}
		if err := write(tx); err != nil {
			return err
		}
		after, err := readSettings(ctx, tx)
		if err != nil {
			return err
		}
		change = settingsChange{before: before, after: after}
		return nil
	})
	if err != nil {
		return settingsChange{}, err
	}
	return change, nil
}

// withPublicationSettings 让最终复核及结果发布与设置、模式、导入和清空写入串行。
// 回调仅处理内存，不发网络请求，也不经连接池读取数据库。
func (s *Service) withPublicationSettings(ctx context.Context, publish func(Settings) error) error {
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		available, err := s.basicModeAvailableTx(ctx, tx)
		if err != nil {
			return err
		}
		if !available {
			return ErrDiscarded
		}
		settings, err := readSettings(ctx, tx)
		if err != nil {
			return err
		}
		return publish(settings)
	})
	// 上下文取消时 database/sql 可能已先回滚，使提交返回 ErrTxDone。
	if ctx.Err() != nil && errors.Is(err, sql.ErrTxDone) {
		return ctx.Err()
	}
	return err
}

func (s *Service) basicModeAvailableTx(ctx context.Context, tx *sql.Tx) (bool, error) {
	configured, err := s.readBoolTx(ctx, tx, config.KeyConfigured)
	if err != nil || !configured {
		return false, err
	}
	advanced, err := s.readBoolTx(ctx, tx, config.KeyAdvancedMode)
	return !advanced, err
}

func (s *Service) readBoolTx(ctx context.Context, tx *sql.Tx, key string) (bool, error) {
	value, err := s.cfg.GetTx(ctx, tx, key)
	if err != nil || value == "" {
		return false, err
	}
	v, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("解析配置 %s 失败: %w", key, err)
	}
	return v, nil
}

// UpdateCredentialByRevision 提交当前账号的续期结果，不改变账号修订。
func (s *Service) UpdateCredentialByRevision(ctx context.Context, provider Provider, revision, credential string) (bool, error) {
	var check, update string
	switch provider {
	case ProviderVPS:
		check = `SELECT 1 FROM provider_traffic_settings WHERE id = 1 AND vps_revision = ?`
		update = `UPDATE provider_traffic_settings SET vps_credential_ciphertext = ? WHERE id = 1 AND vps_revision = ?`
	case ProviderKiwiVM:
		check = `SELECT 1 FROM provider_traffic_hosts WHERE id = 'kiwivm' AND revision = ?`
		update = `UPDATE provider_traffic_hosts SET credential_ciphertext = ? WHERE id = 'kiwivm' AND revision = ?`
	default:
		return false, ErrUnknownProvider
	}
	if revision == "" {
		return false, nil
	}
	if strings.TrimSpace(credential) == "" {
		return false, ErrInvalidCredential
	}
	var updated bool
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var one int
		err := tx.QueryRowContext(ctx, check, revision).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("校验服务商凭据修订失败: %w", err)
		}
		ciphertext, err := s.cfg.EncryptWithTx(ctx, tx, credential)
		if err != nil {
			return fmt.Errorf("加密服务商凭据失败: %w", err)
		}
		if _, err := tx.ExecContext(ctx, update, ciphertext, revision); err != nil {
			return fmt.Errorf("更新服务商凭据失败: %w", err)
		}
		updated = true
		return nil
	})
	return updated, err
}

// LoadCredential 读取旧版绑定，仅供采集模块和绑定校验使用。
func (s *Service) LoadCredential(ctx context.Context, provider Provider) (instanceID, revision, credential string, err error) {
	if providerIndex(provider) < 0 {
		return "", "", "", ErrUnknownProvider
	}
	binding, err := readHost(ctx, s.store.DB(), legacyKey(provider))
	if err != nil {
		return "", "", "", err
	}
	if binding.ID == "" {
		return "", "", "", ErrNotBound
	}
	credentialRevision := binding.Revision
	if provider == ProviderVPS {
		credentialRevision = binding.VPSRevision
	}
	credential, err = s.loadCredential(ctx, binding.key(), credentialRevision)
	if err != nil {
		return "", "", "", err
	}
	return binding.InstanceID, binding.Revision, credential, nil
}

func (s *Service) loadCredential(ctx context.Context, key hostKey, revision string) (string, error) {
	var ciphertext string
	var err error
	if key.provider == ProviderVPS {
		err = s.store.DB().QueryRowContext(ctx, `SELECT vps_credential_ciphertext
			FROM provider_traffic_settings WHERE id = 1 AND vps_revision = ?`, revision).Scan(&ciphertext)
	} else {
		err = s.store.DB().QueryRowContext(ctx, `SELECT credential_ciphertext
			FROM provider_traffic_hosts WHERE id = ? AND provider = ? AND revision = ?`,
			key.id, key.provider, revision).Scan(&ciphertext)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotBound
	}
	if err != nil {
		return "", fmt.Errorf("读取服务商凭据失败: %w", err)
	}
	signingKey, err := s.cfg.GetSigningKey(ctx)
	if err != nil {
		return "", fmt.Errorf("读取签名密钥失败: %w", err)
	}
	plain, err := config.Decrypt(ciphertext, signingKey)
	if err != nil {
		return "", fmt.Errorf("解密服务商凭据失败: %w", err)
	}
	return string(plain), nil
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func readSettings(ctx context.Context, q rowQuerier) (Settings, error) {
	settings := Settings{
		Providers: []ProviderBinding{{Provider: ProviderKiwiVM}, {Provider: ProviderVPS}},
		Hosts:     []ProviderBinding{},
	}
	rows, err := q.QueryContext(ctx, `SELECT s.enabled, s.vps_revision, h.id, h.provider,
		h.instance_id, h.name, h.enabled, h.half_remaining, h.revision, h.edit_revision
		FROM provider_traffic_settings s LEFT JOIN provider_traffic_hosts h ON h.settings_id = s.id
		WHERE s.id = 1 ORDER BY h.provider, h.rowid`)
	if err != nil {
		return Settings{}, fmt.Errorf("读取服务器剩余流量设置失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var enabled bool
		var account, id, provider, instance, name, revision, editRevision sql.NullString
		var hostEnabled, half sql.NullBool
		if err := rows.Scan(&enabled, &account, &id, &provider, &instance, &name,
			&hostEnabled, &half, &revision, &editRevision); err != nil {
			return Settings{}, fmt.Errorf("读取服务器剩余流量主机失败: %w", err)
		}
		settings.Enabled = enabled
		settings.VPSRevision = account.String
		if !id.Valid {
			continue
		}
		binding := ProviderBinding{
			ID: id.String, Provider: Provider(provider.String), InstanceID: instance.String,
			Name: name.String, Enabled: hostEnabled.Bool, HalfRemaining: half.Bool,
			Revision: revision.String, EditRevision: editRevision.String,
			CredentialConfigured: Provider(provider.String) == ProviderKiwiVM || account.Valid,
		}
		settings.Hosts = append(settings.Hosts, binding)
		if binding.ID == string(binding.Provider) {
			settings.Providers[providerIndex(binding.Provider)] = binding
		}
	}
	if err := rows.Err(); err != nil {
		return Settings{}, fmt.Errorf("读取服务器剩余流量设置失败: %w", err)
	}
	return settings, nil
}

type hostRow struct {
	ProviderBinding
	Ciphertext  sql.NullString
	VPSRevision string
}

func readHost(ctx context.Context, q rowQuerier, key hostKey) (hostRow, error) {
	host := hostRow{ProviderBinding: ProviderBinding{Provider: key.provider}}
	var account sql.NullString
	err := q.QueryRowContext(ctx, `SELECT h.id, h.instance_id, h.name, h.enabled, h.half_remaining,
		h.credential_ciphertext, h.revision, h.edit_revision, s.vps_revision
		FROM provider_traffic_hosts h JOIN provider_traffic_settings s ON s.id = h.settings_id
		WHERE h.id = ? AND h.provider = ?`, key.id, key.provider).Scan(
		&host.ID, &host.InstanceID, &host.Name, &host.Enabled, &host.HalfRemaining,
		&host.Ciphertext, &host.Revision, &host.EditRevision, &account)
	if errors.Is(err, sql.ErrNoRows) {
		return hostRow{}, nil
	}
	if err != nil {
		return hostRow{}, fmt.Errorf("读取服务商主机失败: %w", err)
	}
	host.VPSRevision = account.String
	return host, nil
}

func ensureSettingsRow(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO provider_traffic_settings (id) VALUES (1) ON CONFLICT(id) DO NOTHING`); err != nil {
		return fmt.Errorf("初始化服务器剩余流量设置失败: %w", err)
	}
	return nil
}

func validateCredentialInput(input CredentialInput) error {
	if input.Omitted {
		return nil
	}
	if input.Null || strings.TrimSpace(input.Value) == "" {
		return ErrInvalidCredential
	}
	return nil
}

func validInstanceID(value string) bool {
	if value == "" || value[0] < '1' || value[0] > '9' {
		return false
	}
	for i := 1; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func generateRevision() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
