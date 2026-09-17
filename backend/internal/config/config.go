// Package config 提供基于 system_config 表的系统配置服务（业务层）。
package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"golang.org/x/crypto/hkdf"

	"vpn-sub/internal/store"
)

// 配置键常量（本 Step 起步集合，后续 Step 按需补充）
const (
	KeyConfigured = "configured"  // 系统是否已完成 Setup（"true"/"false"）
	KeySigningKey = "signing_key" // 签名密钥（Setup 时生成，明文落库，Design1 §6.2）
	KeyLogLevel   = "log_level"
	KeyAppMode    = "app_mode"
	KeySiteName   = "site_name"
	// Step 4 新增：本地认证与首管理员相关
	KeyAllowLocalLogin  = "allow_local_login" // 允许本地登录（默认 true）
	KeyAllowSelfreg     = "allow_selfreg"     // 允许自注册（默认 false）
	KeySelfRegApproval  = "selfreg_approval"  // 自注册审批开关（默认 false）
	KeyAdminInitialized = "admin_initialized" // 历史键：保留读取/常量兼容，不写入、不参与首管理员判断，不做迁移
	KeyFrontendURL      = "frontend_url"      // 前端地址（Setup 推导初始值，Build3 面板可手动覆盖）
	KeyCallbackURL      = "callback_url"      // OIDC 回调地址（OIDC Setup 推导初始值）
	KeyOidcFlowEpoch    = "oidc_flow_epoch"   // R31-07：OIDC 流程代际（停用/清空时轮换，为空按 initial 处理）
	KeyAdvancedMode     = "advanced_mode"     // 高级模式开关（"true"/"false"，未设置视为 false；Build4 只读暴露）
)

// MaskedSecret 敏感配置在接口回显中的固定占位符；读写两侧统一识别，禁止作为新值保存。
const MaskedSecret = "***"

// ErrSigningKeyUnavailable 表示全站签名密钥缺失或读取失败。
// 管理端 OIDC 保存必须使用该错误阻止任何重填写入，不得在该路径自动生成新密钥。
var ErrSigningKeyUnavailable = errors.New("签名密钥不可用")

// SecretUsable 判定敏感值是否已配置且不是回显占位符（损坏的历史占位符按未配置处理）。
func SecretUsable(secret string) bool {
	return secret != "" && secret != MaskedSecret
}

// isSensitiveKey 编译期固定敏感配置键判定（值以 AES-256-GCM 密文落库）：
// 当前仅 smtp_password；OIDC Client Secret 由 oidc 包手动加密，验证码双密钥明文存储不入集合。
// 不再提供运行期注册入口，避免可变包级注册表。
func isSensitiveKey(key string) bool {
	return key == "smtp_password"
}

type Service struct {
	store *store.Store
	log   *slog.Logger
}

func NewService(st *store.Store, lg *slog.Logger) *Service {
	return &Service{store: st, log: lg}
}

// Get 读取配置；未设置返回空串，调用方自行判定
func (s *Service) Get(ctx context.Context, key string) (string, error) {
	if s.store == nil { // 数据库无法打开的应急模式（main Open 失败分支），配置不可读按未设置处理
		return "", nil
	}
	var v string
	err := s.store.DB().QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取配置 %s 失败: %w", key, err)
	}
	// 敏感键解密返回（密文无法解密时返回错误，防止静默使用损坏数据）
	if isSensitiveKey(key) && v != "" {
		plain, err := s.DecryptWithKey(ctx, v)
		if err != nil {
			return "", fmt.Errorf("解密配置 %s 失败: %w", key, err)
		}
		return string(plain), nil
	}
	return v, nil
}

// Exists 判断配置键是否存在；供需要区分“缺键”和“键存在但值为空/损坏”的读取路径使用。
func (s *Service) Exists(ctx context.Context, key string) (bool, error) {
	if s.store == nil { // 应急模式无持久化配置，统一按缺键处理
		return false, nil
	}
	var one int
	err := s.store.DB().QueryRowContext(ctx, `SELECT 1 FROM system_config WHERE key = ?`, key).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查配置 %s 失败: %w", key, err)
	}
	return true, nil
}

// GetOr 读取非关键配置：读取失败时记录结构化 warn 并返回空串，保持既有 fail-safe 外部行为。
// 供展示/公告/邮件启用判断等非 correctness 配置使用；关键配置仍需调用 Get 并显式处理错误。
func (s *Service) GetOr(ctx context.Context, key string) string {
	v, err := s.Get(ctx, key)
	if err != nil && s.log != nil {
		s.log.Warn("读取配置失败，按未设置降级", "key", key, "err", err)
	}
	return v
}

// DefaultSiteName 是站点名称未配置时的统一回退文案。
const DefaultSiteName = "VPN 订阅管理"

// effectiveSiteName 统一有效站点名称回退规则：已保存值去空白后非空则使用，否则回退默认名称。
func effectiveSiteName(raw string) string {
	if name := strings.TrimSpace(raw); name != "" {
		return name
	}
	return DefaultSiteName
}

// EffectiveSiteName 返回已保存的非空站点名称；未设置或仅含空白时返回默认名称。
// nil Service 仅用于不依赖持久化的渲染单测，同样按未设置处理。
func (s *Service) EffectiveSiteName(ctx context.Context) string {
	if s == nil {
		return DefaultSiteName
	}
	return effectiveSiteName(s.GetOr(ctx, KeySiteName))
}

// EffectiveSiteNameStrict 与 EffectiveSiteName 使用同一回退规则，但配置读取失败时返回错误；
// 供派发器在入队前快照有效站点名，避免把数据库读取故障误当成未配置。
func (s *Service) EffectiveSiteNameStrict(ctx context.Context) (string, error) {
	if s == nil {
		return DefaultSiteName, nil
	}
	raw, err := s.Get(ctx, KeySiteName)
	if err != nil {
		return "", err
	}
	return effectiveSiteName(raw), nil
}

// GetRaw 读取配置原始值（不解密；供导出等需要密文原样的场景）
func (s *Service) GetRaw(ctx context.Context, key string) (string, error) {
	var v string
	err := s.store.DB().QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取配置 %s 失败: %w", key, err)
	}
	return v, nil
}

// Set 写入配置；敏感键自动加密落库
func (s *Service) Set(ctx context.Context, key, value string) error {
	v := value
	if isSensitiveKey(key) {
		enc, err := s.EncryptSensitive(ctx, value) // 失败即中断，禁止明文落库
		if err != nil {
			return err
		}
		v = enc
	}
	_, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO system_config (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, key, v)
	if err != nil {
		return fmt.Errorf("写入配置 %s 失败: %w", key, err)
	}
	return nil
}

// Delete 删除单个配置键；键不存在时仍成功（幂等），供邮件模板恢复默认等场景复用。
func (s *Service) Delete(ctx context.Context, key string) error {
	if s.store == nil { // 应急模式下无持久化配置可删除，按幂等成功处理
		return nil
	}
	if _, err := s.store.DB().ExecContext(ctx, `DELETE FROM system_config WHERE key = ?`, key); err != nil {
		return fmt.Errorf("删除配置 %s 失败: %w", key, err)
	}
	return nil
}

// GetTx 事务内读取配置（供事务闭包内使用；事务内禁止经 store.DB() 二次取连接，防连接池死锁）
func (s *Service) GetTx(ctx context.Context, tx *sql.Tx, key string) (string, error) {
	var v string
	err := tx.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("读取配置 %s 失败: %w", key, err)
	}
	if isSensitiveKey(key) && v != "" {
		// 事务内敏感键读取：先取同事务签名密钥解密
		keyBytes, err := s.GetSigningKeyTx(ctx, tx)
		if err != nil {
			return "", fmt.Errorf("解密配置 %s 失败: %w", key, err)
		}
		plain, err := Decrypt(v, keyBytes)
		if err != nil {
			return "", fmt.Errorf("解密配置 %s 失败: %w", key, err)
		}
		return string(plain), nil
	}
	return v, nil
}

// GetSigningKeyTx 事务内读取签名密钥（缺失或读取失败返回 ErrSigningKeyUnavailable，不生成）
func (s *Service) GetSigningKeyTx(ctx context.Context, tx *sql.Tx) ([]byte, error) {
	var v string
	err := tx.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, KeySigningKey).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: 未配置", ErrSigningKeyUnavailable)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningKeyUnavailable, err)
	}
	if v == "" {
		return nil, fmt.Errorf("%w: 未配置", ErrSigningKeyUnavailable)
	}
	return []byte(v), nil
}

// SetTx 事务内写入（供 Setup 快速开始等多键原子写入场景复用）
func (s *Service) SetTx(ctx context.Context, tx *sql.Tx, key, value string) error {
	v := value
	if isSensitiveKey(key) {
		enc, err := s.EncryptWithTx(ctx, tx, value)
		if err != nil {
			return err
		}
		v = enc
	}
	_, err := tx.ExecContext(ctx,
		`INSERT INTO system_config (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = CURRENT_TIMESTAMP`, key, v)
	if err != nil {
		return fmt.Errorf("写入配置 %s 失败: %w", key, err)
	}
	return nil
}

// GetBool 类型化读取：解析失败按默认值并记 warn 日志
func (s *Service) GetBool(ctx context.Context, key string, def bool) bool {
	v, err := s.Get(ctx, key)
	if err != nil {
		s.log.Warn("读取布尔配置失败，使用默认值", "key", key, "err", err)
		return def
	}
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		s.log.Warn("解析布尔配置失败，使用默认值", "key", key, "value", v)
		return def
	}
	return b
}

// GetBoolStrict 关键路径布尔读取：DB 错误直接返回，不做 fail-safe（R14-25 用户决策）。
func (s *Service) GetBoolStrict(ctx context.Context, key string, def bool) (bool, error) {
	v, err := s.Get(ctx, key)
	if err != nil {
		return false, err
	}
	if v == "" {
		return def, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def, fmt.Errorf("解析配置 %s 失败: %w", key, err)
	}
	return b, nil
}

// GetInt 类型化读取：解析失败按默认值并记 warn 日志
func (s *Service) GetInt(ctx context.Context, key string, def int) int {
	v, err := s.Get(ctx, key)
	if err != nil {
		s.log.Warn("读取整数配置失败，使用默认值", "key", key, "err", err)
		return def
	}
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		s.log.Warn("解析整数配置失败，使用默认值", "key", key, "value", v)
		return def
	}
	return n
}

// GetJSONStringSlice 解析 JSON 字符串数组配置（读取/解析失败返回空切片并记 warn）。
func (s *Service) GetJSONStringSlice(ctx context.Context, key string) []string {
	out, err := s.GetJSONStringSliceStrict(ctx, key)
	if err != nil && s.log != nil {
		s.log.Warn("解析 JSON 数组配置失败，按未设置降级", "key", key, "err", err)
	}
	return out
}

// GetJSONStringSliceStrict 严格解析 JSON 字符串数组；缺键返回空切片，读取/解析失败返回错误。
// 供邮件 scope 等 correctness 路径使用，避免损坏配置被静默当成“未启用”。
func (s *Service) GetJSONStringSliceStrict(ctx context.Context, key string) ([]string, error) {
	v, err := s.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if v == "" {
		return nil, nil
	}
	var out []string
	if err := json.Unmarshal([]byte(v), &out); err != nil {
		return nil, fmt.Errorf("解析 JSON 数组配置 %s 失败: %w", key, err)
	}
	return out, nil
}

// --- 敏感配置加解密：AES-256-GCM，密钥由签名密钥经 HKDF-SHA256 派生（用户已确认选型）---

// deriveKey 由签名密钥派生 32 字节 AES-256 密钥（info 固定，全程统一）
func deriveKey(signingKey []byte) ([]byte, error) {
	r := hkdf.New(sha256.New, signingKey, nil, []byte("vpn-sub/config-encryption"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("派生配置加密密钥失败: %w", err)
	}
	return key, nil
}

// Encrypt 输出格式：base64url(nonce ‖ 密文)
func Encrypt(plain, signingKey []byte) (string, error) {
	key, err := deriveKey(signingKey)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("初始化 AES 失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(gcm.Seal(nonce, nonce, plain, nil)), nil
}

// Decrypt Encrypt 的逆过程；base64 解码失败或 GCM 校验失败均返回明确错误（防篡改）
func Decrypt(encoded string, signingKey []byte) ([]byte, error) {
	key, err := deriveKey(signingKey)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("初始化 AES 失败: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("初始化 GCM 失败: %w", err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("密文解码失败: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("密文长度非法")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("密文校验失败（可能被篡改）: %w", err)
	}
	return plain, nil
}

// GetSigningKey 读取签名密钥（明文落库）；缺失或读取失败返回 ErrSigningKeyUnavailable，不生成。
func (s *Service) GetSigningKey(ctx context.Context) ([]byte, error) {
	v, err := s.Get(ctx, KeySigningKey)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSigningKeyUnavailable, err)
	}
	if v == "" {
		return nil, fmt.Errorf("%w: 未配置", ErrSigningKeyUnavailable)
	}
	return []byte(v), nil
}

// EnsureSigningKey 确保签名密钥存在：为空则生成 32 字节加密安全随机值写入（明文，256 位熵）。
// 供会话签发前置调用；Setup 完成事务复用同一密钥，不重复生成（Design1 §3.1/6.2）
func (s *Service) EnsureSigningKey(ctx context.Context) ([]byte, error) {
	existing, err := s.Get(ctx, KeySigningKey)
	if err != nil {
		return nil, err
	}
	if existing != "" {
		return []byte(existing), nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成签名密钥失败: %w", err)
	}
	if err := s.Set(ctx, KeySigningKey, string(key)); err != nil {
		return nil, err
	}
	s.log.Info("签名密钥已生成")
	return key, nil
}

// EnsureSigningKeyTx 事务内确保签名密钥存在（Setup 完成事务 / OIDC Setup 事务复用，不重复生成）
func (s *Service) EnsureSigningKeyTx(ctx context.Context, tx *sql.Tx) ([]byte, error) {
	var existing string
	err := tx.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = ?`, KeySigningKey).Scan(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("读取签名密钥失败: %w", err)
	}
	if existing != "" {
		return []byte(existing), nil
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("生成签名密钥失败: %w", err)
	}
	if err := s.SetTx(ctx, tx, KeySigningKey, string(key)); err != nil {
		return nil, err
	}
	return key, nil
}

// EncryptSensitive 用当前签名密钥加密敏感值（供 Set 使用）
func (s *Service) EncryptSensitive(ctx context.Context, plain string) (string, error) {
	key, err := s.EnsureSigningKey(ctx)
	if err != nil {
		return "", fmt.Errorf("获取签名密钥失败: %w", err)
	}
	return Encrypt([]byte(plain), key)
}

// DecryptWithKey 用当前签名密钥解密（供 Get 使用）
func (s *Service) DecryptWithKey(ctx context.Context, encoded string) ([]byte, error) {
	key, err := s.GetSigningKey(ctx)
	if err != nil {
		return nil, err
	}
	return Decrypt(encoded, key)
}

// EncryptWithTx 事务内加密（读同一 tx 中的签名密钥；Setup/OIDC Setup 事务内使用）
func (s *Service) EncryptWithTx(ctx context.Context, tx *sql.Tx, plain string) (string, error) {
	key, err := s.EnsureSigningKeyTx(ctx, tx)
	if err != nil {
		return "", err
	}
	return Encrypt([]byte(plain), key)
}
