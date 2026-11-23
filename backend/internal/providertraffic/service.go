// Package providertraffic 提供基础模式服务器剩余流量的本地存储、绑定管理、固定上游客户端与采集编排。
// 本阶段包含 SQLite 功能表读写、绑定修订校验、凭据加解密、KiwiVM／V.P.S 两个固定客户端、
// 固定周期采集、同家单飞与配置触发合并、内存结果槽及 V.P.S Access Token 缓存；
// HTTP 接口与正式服务装配由后续 Step 接入。
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
	ErrRevisionConflict = errors.New("绑定修订冲突")
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
	Provider             Provider
	Enabled              bool
	InstanceID           string
	Revision             string
	CredentialConfigured bool
}

// Settings 是两家固定顺序的本地设置；无功能表记录时返回默认关闭且未绑定。
type Settings struct {
	Enabled   bool
	Providers []ProviderBinding
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
// 首次绑定要求 revision=null 且提交凭据；同一实例省略凭据表示保留当前密文与修订。
// 正常服务的设置入口使用 Collector.SaveBinding 交接调度。
func (s *Service) SaveBinding(ctx context.Context, in SaveBindingInput) (Settings, error) {
	change, err := s.saveBinding(ctx, in)
	return change.after, err
}

func (s *Service) saveBinding(ctx context.Context, in SaveBindingInput) (settingsChange, error) {
	cols, err := columnsFor(in.Provider)
	if err != nil {
		return settingsChange{}, err
	}
	if !validInstanceID(in.InstanceID) {
		return settingsChange{}, ErrInvalidInstanceID
	}
	if err := validateCredentialInput(in.Credential); err != nil {
		return settingsChange{}, err
	}
	return s.changeSettings(ctx, func(tx *sql.Tx) error {
		if err := ensureSettingsRow(ctx, tx); err != nil {
			return err
		}
		current, err := readBinding(ctx, tx, cols)
		if err != nil {
			return err
		}
		if !current.Bound {
			// 首次绑定：修订必须为 null，且必须提交凭据。
			if in.Revision != nil {
				return ErrRevisionConflict
			}
			if in.Credential.Omitted {
				return ErrCredentialRequired
			}
		} else {
			// 修改绑定：必须匹配当前非空修订；更换实例必须提交新凭据。
			if in.Revision == nil || *in.Revision == "" || *in.Revision != current.Revision {
				return ErrRevisionConflict
			}
			if in.InstanceID != current.InstanceID && in.Credential.Omitted {
				return ErrCredentialRequired
			}
		}
		credentialCiphertext := current.CredentialCiphertext
		revision := current.Revision
		if !in.Credential.Omitted {
			ciphertext, err := s.cfg.EncryptWithTx(ctx, tx, in.Credential.Value)
			if err != nil {
				return fmt.Errorf("加密服务商凭据失败: %w", err)
			}
			newRevision, err := s.newRevision()
			if err != nil {
				return fmt.Errorf("生成绑定修订失败: %w", err)
			}
			credentialCiphertext = ciphertext
			revision = newRevision
		}
		query := fmt.Sprintf(
			`UPDATE provider_traffic_settings SET %s = ?, %s = ?, %s = ?, %s = ? WHERE id = 1`,
			cols.enabled, cols.instanceID, cols.credential, cols.revision)
		if _, err := tx.ExecContext(ctx, query,
			boolToInt(in.Enabled), in.InstanceID, credentialCiphertext, revision); err != nil {
			return fmt.Errorf("保存服务商绑定失败: %w", err)
		}
		return nil
	})
}

// ClearBinding 按当前非空修订解除单家绑定：清除实例、密文、修订与单家开关，保留总开关和另一家。
// 正常服务的设置入口使用 Collector.ClearBinding 清理内存状态。
func (s *Service) ClearBinding(ctx context.Context, provider Provider, revision string) (Settings, error) {
	change, err := s.clearBinding(ctx, provider, revision)
	return change.after, err
}

func (s *Service) clearBinding(ctx context.Context, provider Provider, revision string) (settingsChange, error) {
	cols, err := columnsFor(provider)
	if err != nil {
		return settingsChange{}, err
	}
	if revision == "" {
		return settingsChange{}, ErrRevisionConflict
	}
	return s.changeSettings(ctx, func(tx *sql.Tx) error {
		current, err := readBinding(ctx, tx, cols)
		if err != nil {
			return err
		}
		if !current.Bound || current.Revision != revision {
			return ErrRevisionConflict
		}
		query := fmt.Sprintf(
			`UPDATE provider_traffic_settings SET %s = 0, %s = NULL, %s = NULL, %s = NULL WHERE id = 1`,
			cols.enabled, cols.instanceID, cols.credential, cols.revision)
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("解除服务商绑定失败: %w", err)
		}
		return nil
	})
}

// changeSettings 在同一写事务内记录变更前后状态，只交付成功提交的变化。
func (s *Service) changeSettings(ctx context.Context, write func(*sql.Tx) error) (settingsChange, error) {
	var change settingsChange
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
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
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		configured, err := s.readBoolTx(ctx, tx, config.KeyConfigured)
		if err != nil {
			return err
		}
		if !configured {
			return ErrDiscarded
		}
		advanced, err := s.readBoolTx(ctx, tx, config.KeyAdvancedMode)
		if err != nil {
			return err
		}
		if advanced {
			return ErrDiscarded
		}
		settings, err := readSettings(ctx, tx)
		if err != nil {
			return err
		}
		return publish(settings)
	})
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

// UpdateCredentialByRevision 仅按当前非空修订条件更新已有绑定的凭据密文。
// 修订不匹配时影响零行并返回 false；不改变修订号、开关和实例 ID。
func (s *Service) UpdateCredentialByRevision(ctx context.Context, provider Provider, revision, credential string) (bool, error) {
	cols, err := columnsFor(provider)
	if err != nil {
		return false, err
	}
	if revision == "" {
		return false, nil
	}
	if strings.TrimSpace(credential) == "" {
		return false, ErrInvalidCredential
	}
	var updated bool
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 先在同事务内确认修订仍匹配，避免零行时仍生成签名密钥或密文。
		var one int
		check := fmt.Sprintf(
			`SELECT 1 FROM provider_traffic_settings WHERE id = 1 AND %s = ? AND %s IS NOT NULL`,
			cols.revision, cols.instanceID)
		err := tx.QueryRowContext(ctx, check, revision).Scan(&one)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("校验服务商绑定修订失败: %w", err)
		}
		ciphertext, err := s.cfg.EncryptWithTx(ctx, tx, credential)
		if err != nil {
			return fmt.Errorf("加密服务商凭据失败: %w", err)
		}
		query := fmt.Sprintf(
			`UPDATE provider_traffic_settings SET %s = ? WHERE id = 1 AND %s = ? AND %s IS NOT NULL`,
			cols.credential, cols.revision, cols.instanceID)
		result, err := tx.ExecContext(ctx, query, ciphertext, revision)
		if err != nil {
			return fmt.Errorf("更新服务商凭据失败: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("读取服务商凭据更新结果失败: %w", err)
		}
		updated = affected > 0
		return nil
	})
	return updated, err
}

// LoadCredential 读取并解密当前绑定的凭据，仅供后续采集模块在进程内使用。
// 返回的明文不得进入响应、日志或导出。
func (s *Service) LoadCredential(ctx context.Context, provider Provider) (instanceID, revision, credential string, err error) {
	cols, err := columnsFor(provider)
	if err != nil {
		return "", "", "", err
	}
	query := fmt.Sprintf(
		`SELECT %s, %s, %s FROM provider_traffic_settings WHERE id = 1`,
		cols.instanceID, cols.credential, cols.revision)
	var instance, ciphertext, currentRevision sql.NullString
	err = s.store.DB().QueryRowContext(ctx, query).Scan(&instance, &ciphertext, &currentRevision)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", "", ErrNotBound
	}
	if err != nil {
		return "", "", "", fmt.Errorf("读取服务商绑定失败: %w", err)
	}
	if !instance.Valid || !ciphertext.Valid || !currentRevision.Valid ||
		instance.String == "" || ciphertext.String == "" || currentRevision.String == "" {
		return "", "", "", ErrNotBound
	}
	signingKey, err := s.cfg.GetSigningKey(ctx)
	if err != nil {
		return "", "", "", fmt.Errorf("读取签名密钥失败: %w", err)
	}
	plain, err := config.Decrypt(ciphertext.String, signingKey)
	if err != nil {
		return "", "", "", fmt.Errorf("解密服务商凭据失败: %w", err)
	}
	return instance.String, currentRevision.String, string(plain), nil
}

type providerColumns struct {
	enabled    string
	instanceID string
	credential string
	revision   string
}

func columnsFor(provider Provider) (providerColumns, error) {
	switch provider {
	case ProviderKiwiVM:
		return providerColumns{
			enabled:    "kiwivm_enabled",
			instanceID: "kiwivm_instance_id",
			credential: "kiwivm_credential_ciphertext",
			revision:   "kiwivm_revision",
		}, nil
	case ProviderVPS:
		return providerColumns{
			enabled:    "vps_enabled",
			instanceID: "vps_instance_id",
			credential: "vps_credential_ciphertext",
			revision:   "vps_revision",
		}, nil
	default:
		return providerColumns{}, ErrUnknownProvider
	}
}

type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func readSettings(ctx context.Context, q rowQuerier) (Settings, error) {
	var enabled, kiwivmEnabled, vpsEnabled int
	var kiwivmInstance, kiwivmCredential, kiwivmRevision sql.NullString
	var vpsInstance, vpsCredential, vpsRevision sql.NullString
	err := q.QueryRowContext(ctx,
		`SELECT enabled, kiwivm_enabled, kiwivm_instance_id, kiwivm_credential_ciphertext, kiwivm_revision,
		        vps_enabled, vps_instance_id, vps_credential_ciphertext, vps_revision
		 FROM provider_traffic_settings WHERE id = 1`).Scan(
		&enabled, &kiwivmEnabled, &kiwivmInstance, &kiwivmCredential, &kiwivmRevision,
		&vpsEnabled, &vpsInstance, &vpsCredential, &vpsRevision,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("读取服务器剩余流量设置失败: %w", err)
	}
	return Settings{
		Enabled: enabled == 1,
		Providers: []ProviderBinding{
			bindingFromRow(ProviderKiwiVM, kiwivmEnabled, kiwivmInstance, kiwivmCredential, kiwivmRevision),
			bindingFromRow(ProviderVPS, vpsEnabled, vpsInstance, vpsCredential, vpsRevision),
		},
	}, nil
}

func bindingFromRow(provider Provider, enabled int, instance, credential, revision sql.NullString) ProviderBinding {
	bound := instance.Valid && instance.String != "" &&
		credential.Valid && credential.String != "" &&
		revision.Valid && revision.String != ""
	if !bound {
		return ProviderBinding{Provider: provider}
	}
	return ProviderBinding{
		Provider:             provider,
		Enabled:              enabled == 1,
		InstanceID:           instance.String,
		Revision:             revision.String,
		CredentialConfigured: true,
	}
}

type bindingRow struct {
	Bound                bool
	InstanceID           string
	CredentialCiphertext string
	Revision             string
}

func readBinding(ctx context.Context, tx *sql.Tx, cols providerColumns) (bindingRow, error) {
	query := fmt.Sprintf(
		`SELECT %s, %s, %s FROM provider_traffic_settings WHERE id = 1`,
		cols.instanceID, cols.credential, cols.revision)
	var instance, credential, revision sql.NullString
	err := tx.QueryRowContext(ctx, query).Scan(&instance, &credential, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return bindingRow{}, nil
	}
	if err != nil {
		return bindingRow{}, fmt.Errorf("读取服务商绑定失败: %w", err)
	}
	if !instance.Valid || !credential.Valid || !revision.Valid ||
		instance.String == "" || credential.String == "" || revision.String == "" {
		return bindingRow{}, nil
	}
	return bindingRow{
		Bound:                true,
		InstanceID:           instance.String,
		CredentialCiphertext: credential.String,
		Revision:             revision.String,
	}, nil
}

func ensureSettingsRow(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO provider_traffic_settings (id) VALUES (1) ON CONFLICT(id) DO NOTHING`); err != nil {
		return fmt.Errorf("初始化服务器剩余流量设置失败: %w", err)
	}
	return nil
}

func defaultSettings() Settings {
	return Settings{Providers: []ProviderBinding{
		{Provider: ProviderKiwiVM},
		{Provider: ProviderVPS},
	}}
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
