// config/admin.go：面板配置服务（Build3 Step 3）——各分区配置读写与生效逻辑（Design1 §3.4.8）。
// 敏感字段（OIDC Client Secret / SMTP 密码）加密落库、回显脱敏；验证码双密钥为明文存储（面板回显真实值）。
// 本地登录与 OIDC 均不可用禁止保存（防认证死锁）。
package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/mail"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"vpn-sub/internal/store"
	"vpn-sub/internal/urlguard"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrAuthDeadlock      = errors.New("本地登录与 OIDC 均不可用，禁止保存（防认证死锁）")
	ErrCaptchaKeyMissing = errors.New("启用验证码页面需先配置密钥")
	ErrBadRequest        = errors.New("参数错误")
)

// OIDC 配置键（与 oidc 包常量同值；config 包避免循环依赖以字面量引用）
const (
	oidcKeyProviderType = "oidc_provider_type"
	oidcKeyConfigured   = "oidc_configured"
	oidcKeyApproval     = "oidc_approval"
	oidcKeyWhitelist    = "oidc_whitelist"
)

// 验证码/限流配置键（与 captcha/ratelimit 包常量同值）
const (
	captchaKeyProvider        = "captcha_provider"
	captchaKeySiteKey         = "captcha_site_key"
	captchaKeySecretKey       = "captcha_secret_key"
	captchaKeyPages           = "captcha_pages"
	ratelimitKeyLogin         = "ratelimit_login"
	ratelimitKeyReg           = "ratelimit_register"
	ratelimitKeyForgot        = "ratelimit_forgot"
	ratelimitKeyDown          = "ratelimit_download"
	ratelimitKeyResetValidate = "ratelimit_reset_validate"

	httpReadHeaderTimeoutSecKey = "http_read_header_timeout_sec"
	httpReadTimeoutSecKey       = "http_read_timeout_sec"
	httpWriteTimeoutSecKey      = "http_write_timeout_sec"
	httpIdleTimeoutSecKey       = "http_idle_timeout_sec"
	httpMaxBodyMbKey            = "http_max_body_mb"
)

// WhitelistConfig OIDC Role/Group 白名单配置（Design1 §3.4.8；oidc 包 matchWhitelist 同构读取）
type WhitelistConfig struct {
	RoleClaimPath  string   `json:"role_claim_path"`
	RoleValues     []string `json:"role_values"`
	GroupClaimPath string   `json:"group_claim_path"`
	GroupValues    []string `json:"group_values"`
}

// Empty 白名单是否为空（role_values 与 group_values 均为空 → 跳过校验直接激活）
func (w WhitelistConfig) Empty() bool {
	return len(w.RoleValues) == 0 && len(w.GroupValues) == 0
}

// OidcParamsStateCode OIDC 参数状态枚举；接口只返回枚举与固定提示，不返回 Secret 明文/密文/签名密钥。
type OidcParamsStateCode string

const (
	OidcParamsNotConfigured   OidcParamsStateCode = "not_configured"
	OidcParamsMissingSecret   OidcParamsStateCode = "missing_secret"
	OidcParamsUsable          OidcParamsStateCode = "usable"
	OidcParamsJSONDamaged     OidcParamsStateCode = "json_damaged"
	OidcParamsSecretDamaged   OidcParamsStateCode = "secret_damaged"
	OidcParamsSigningKeyFault OidcParamsStateCode = "signing_key_fault"
)

// OIDC 固定提示：所有出口共用，禁止拼接 Secret/密文/签名密钥，避免日志与响应漂移。
const (
	OidcWarningNotConfigured         = "当前提供商尚未保存 OIDC 参数，请填写必要参数并输入新的 Client Secret 后保存"
	OidcWarningMissingSecret         = "已存 OIDC 参数尚未配置可用 Client Secret，请填写新的 Client Secret 后保存"
	OidcWarningJSONDamaged           = "已存 OIDC 参数 JSON 无法解析，请重新填写必要的 Base URL/Realm/Client ID 并输入新的 Client Secret 后保存"
	OidcWarningSecretDamaged         = "已存 Client Secret 损坏或为脱敏占位符，请输入新的 Client Secret 后保存"
	OidcWarningSigningKeyFault       = "系统签名密钥缺失或不可读取，当前无法校验或保存 OIDC 凭据；请通过备份恢复或应急初始化处理，不要在此重填 Secret"
	OidcTestStoredDamagedMessage     = "目标提供商已存 OIDC 配置损坏，须重新填写必要参数并输入新的 Client Secret 后再测试"
	OidcTestSigningKeyFaultMessage   = "系统签名密钥不可用，无法校验已存 OIDC 配置；请通过备份恢复或应急初始化处理"
	OidcSigningKeyFaultPublicMessage = "系统签名密钥不可用，OIDC 配置无法保存；请通过备份恢复或应急初始化处理"
)

// OidcParamsState OIDC 提供商参数只读状态（不含 Secret 明文或原始密文）。
type OidcParamsState struct {
	State         OidcParamsStateCode // 完整状态枚举；为空时由 EffectiveOidcParamsState 从旧标记推导
	Present       bool                // 是否存在非空参数 JSON
	BaseURL       string              // JSON 可解析时的非 Secret 字段
	Realm         string
	ClientID      string
	SecretUsable  bool // 已存 Secret 解密后非空且非脱敏占位符
	SecretDamaged bool // Secret 非空但无法解密，或解密后为 ***
	JSONDamaged   bool // 原始 JSON 非空但无法解析
}

// EffectiveOidcParamsState 兼容旧标记：State 已设置时直接返回，否则按 R31-03 布尔标记推导。
func EffectiveOidcParamsState(st OidcParamsState) OidcParamsStateCode {
	if st.State != "" {
		return st.State
	}
	if !st.Present {
		return OidcParamsNotConfigured
	}
	if st.JSONDamaged {
		return OidcParamsJSONDamaged
	}
	if st.SecretDamaged {
		return OidcParamsSecretDamaged
	}
	if st.SecretUsable {
		return OidcParamsUsable
	}
	return OidcParamsMissingSecret
}

// OidcStateWarning 返回状态对应的固定只读提示；调用方不得再拼接任何配置值。
func OidcStateWarning(st OidcParamsState) string {
	switch EffectiveOidcParamsState(st) {
	case OidcParamsNotConfigured:
		return OidcWarningNotConfigured
	case OidcParamsMissingSecret:
		return OidcWarningMissingSecret
	case OidcParamsJSONDamaged:
		return OidcWarningJSONDamaged
	case OidcParamsSecretDamaged:
		return OidcWarningSecretDamaged
	case OidcParamsSigningKeyFault:
		return OidcWarningSigningKeyFault
	default:
		return ""
	}
}

// OidcOps OIDC 能力接口（oidc.Service 经 server 适配注入；config 包避免 config↔oidc 循环依赖）
type OidcOps interface {
	// SaveParams 保存提供商参数（入参 client_secret 为明文，加密落库；空值保留原密文）
	SaveParams(ctx context.Context, providerType, baseURL, realm, clientID, clientSecret string) error
	// LoadParams 读取提供商参数（client_secret 已解密）
	LoadParams(ctx context.Context, providerType string) (baseURL, realm, clientID, clientSecret string, err error)
	// DescribeParams 读取提供商参数只读状态（不返回 Secret；供目标读取与保存校验）
	DescribeParams(ctx context.Context, providerType string) (OidcParamsState, error)
	// SaveParamsTx 在调用方写事务内保存参数并完成分类/密钥校验/加密或保留密文
	SaveParamsTx(ctx context.Context, tx *sql.Tx, providerType, baseURL, realm, clientID, clientSecret string) error
	// DescribeParamsTx 在调用方写事务内读取参数只读状态
	DescribeParamsTx(ctx context.Context, tx *sql.Tx, providerType string) (OidcParamsState, error)
	// IsConfigured OIDC 是否已配置
	IsConfigured(ctx context.Context) bool
	// ClearDiscCache 配置变更后清发现文档缓存
	ClearDiscCache()
}

// AdvancedModeSwitcher 高级模式开关能力接口（由 server 注入 xray.OffClear 实现，避免 config↔xray 循环依赖）。
type AdvancedModeSwitcher interface {
	SubmitAdvancedMode(ctx context.Context, on bool, confirmWord string) (string, error)
}

// AdminService 面板配置服务
type AdminService struct {
	cfg              *Service
	store            *store.Store
	oidcOps          OidcOps
	advancedSwitcher AdvancedModeSwitcher
	dataDir          string // 数据卷根目录（站点 ICON 落盘用）
	log              *slog.Logger
	level            *slog.LevelVar
}

func NewAdminService(cfg *Service, st *store.Store, oidcOps OidcOps, dataDir string, lg *slog.Logger, level *slog.LevelVar) *AdminService {
	return &AdminService{cfg: cfg, store: st, oidcOps: oidcOps, dataDir: dataDir, log: lg, level: level}
}

// SetAdvancedModeSwitcher 注入高级模式开关实现（server 装配时调用）。
func (s *AdminService) SetAdvancedModeSwitcher(fn AdvancedModeSwitcher) {
	s.advancedSwitcher = fn
}

// --- 通用读写辅助 ---

// getMasked 敏感字段 GET 返回脱敏值（已配置 → "***"，未配置 → ""），禁止返回明文
func (s *AdminService) getMasked(ctx context.Context, key string) string {
	v := s.cfg.GetOr(ctx, key)
	if v == "" {
		return ""
	}
	return "***"
}

// setSensitive PUT 接受新值（空串表示不修改）；非空时经 config.Set 自动加密落库
func (s *AdminService) setSensitive(ctx context.Context, key, value string) error {
	if value == "" {
		return nil // 空 = 不修改
	}
	return s.cfg.Set(ctx, key, value)
}

// --- OIDC 配置分区 ---

type OidcSettings struct {
	ProviderType           string              `json:"provider_type"`
	BaseURL                string              `json:"base_url"`
	Realm                  string              `json:"realm"`
	ClientID               string              `json:"client_id"`
	ClientSecret           string              `json:"client_secret"`            // GET 始终为空；PUT 空=保留当前提供商原密文
	ClientSecretConfigured bool                `json:"client_secret_configured"` // 当前提供商是否已有可用 Secret（只读）
	FrontendURL            string              `json:"frontend_url"`             // 保存后即时生效（库驱动）
	CallbackURL            string              `json:"callback_url"`             // 空=不修改；clear_callback_url=true 时显式清除并恢复推导回退
	ClearCallbackURL       bool                `json:"clear_callback_url"`       // 请求字段：显式清除独立回调地址
	ParamsState            OidcParamsStateCode `json:"params_state,omitempty"`   // 完整只读状态枚举
	ParamsDamaged          bool                `json:"params_damaged,omitempty"` // 已存参数存在 JSON/Secret 损坏（只读，兼容 R31-03）
	ParamsWarning          string              `json:"params_warning,omitempty"` // 固定损坏/重填提示（只读）
}

// 合法提供商类型
var validProviders = []string{"keycloak", "auth0", "generic", "mock"}

// oidcUsableState 判定“请求参数 + 已存状态”合并后 OIDC 是否可用；供 SaveOidc 事务内防死锁判定。
// 真实提供商 base_url 必须 HTTPS、client_id 非空；显式新 Secret 视为可用；空 Secret 仅在已存状态可用
// 且 Base URL/Realm/Client ID 完全一致时视为可用。
func (s *AdminService) oidcUsableState(in OidcSettings, st OidcParamsState) bool {
	if in.BaseURL == "" || in.ClientID == "" {
		return false
	}
	if in.ProviderType == "mock" {
		// 保持 R31-06/R31-07 前的 mock 锁死判定：mock 空 Secret 不作为关闭本地登录的依据。
		return SecretUsable(in.ClientSecret)
	}
	if err := urlguard.ValidateHTTPS(in.BaseURL); err != nil {
		return false
	}
	if SecretUsable(in.ClientSecret) {
		return true
	}
	return EffectiveOidcParamsState(st) == OidcParamsUsable &&
		st.BaseURL == in.BaseURL && st.Realm == in.Realm && st.ClientID == in.ClientID
}

// oidcAvailable 判定当前生效的 OIDC 是否可作为登录方式（防认证死锁第二层校验）：
// 已标记配置、当前提供商参数可读取/可解密、真实提供商 base_url 为 HTTPS、client_id 非空且 Secret 可用，
// 且能解析出有效回调地址；mock 仅 Dev 模式可用；空 Secret/损坏/签名密钥故障均不能作为关闭本地登录的依据。
func (s *AdminService) oidcAvailable(ctx context.Context) bool {
	if !s.oidcOps.IsConfigured(ctx) {
		return false
	}
	providerType := s.cfg.GetOr(ctx, oidcKeyProviderType)
	if providerType == "" {
		return false
	}
	st, err := s.oidcOps.DescribeParams(ctx, providerType)
	if err != nil {
		return false
	}
	state := EffectiveOidcParamsState(st)
	if providerType == "mock" { // 模拟 OIDC 仅 Dev 模式提供登录能力
		if s.cfg.GetOr(ctx, KeyAppMode) != "dev" {
			return false
		}
		return state != OidcParamsJSONDamaged && state != OidcParamsNotConfigured
	}
	if state != OidcParamsUsable || st.BaseURL == "" || st.ClientID == "" {
		return false
	}
	if err := urlguard.ValidateHTTPS(st.BaseURL); err != nil {
		return false
	}
	if _, err := ResolveOidcCallbackURL(s.cfg.GetOr(ctx, KeyCallbackURL), s.cfg.GetOr(ctx, KeyFrontendURL)); err != nil {
		return false
	}
	return true
}

// applyOidcParamsState 将只读状态写入 GET 响应：Secret 始终清空，JSON 损坏时不猜测非 Secret 字段，
// 固定提示与状态枚举由 OidcStateWarning 提供；mock 不展示 Secret/损坏提示，保持 R31-03 前的回显语义。
func applyOidcParamsState(out *OidcSettings, st OidcParamsState) {
	state := EffectiveOidcParamsState(st)
	out.ParamsState = state
	if state == OidcParamsJSONDamaged {
		out.BaseURL, out.Realm, out.ClientID = "", "", ""
	} else if out.ProviderType != "" {
		out.BaseURL, out.Realm, out.ClientID = st.BaseURL, st.Realm, st.ClientID
	}
	out.ClientSecret = ""
	out.ClientSecretConfigured = state == OidcParamsUsable
	out.ParamsDamaged = state == OidcParamsJSONDamaged || state == OidcParamsSecretDamaged
	// 未配置/缺 Secret 保持 R31-03 的最小回显（提示由前端按状态补全）；损坏与密钥故障由后端给固定提示。
	out.ParamsWarning = ""
	if state == OidcParamsJSONDamaged || state == OidcParamsSecretDamaged || state == OidcParamsSigningKeyFault {
		out.ParamsWarning = OidcStateWarning(st)
	}
	if out.ProviderType == "mock" {
		out.ClientSecretConfigured = false // mock 无 Secret 语义，保持旧回显
		out.ParamsDamaged = false
		out.ParamsWarning = ""
	}
}

// GetOidc 回显当前 OIDC 配置（Secret 输入值始终为空，完整状态由 params_state 表示）。
func (s *AdminService) GetOidc(ctx context.Context) (OidcSettings, error) {
	out := OidcSettings{}
	out.ProviderType = s.cfg.GetOr(ctx, oidcKeyProviderType)
	out.FrontendURL = s.cfg.GetOr(ctx, KeyFrontendURL)
	out.CallbackURL = s.cfg.GetOr(ctx, KeyCallbackURL)
	if out.ProviderType == "" {
		out.ParamsState = OidcParamsNotConfigured
		return out, nil
	}
	st, err := s.oidcOps.DescribeParams(ctx, out.ProviderType)
	if err != nil {
		if errors.Is(err, ErrSigningKeyUnavailable) {
			if st.State == "" {
				st.State = OidcParamsSigningKeyFault
			}
			applyOidcParamsState(&out, st)
			return out, nil // 独立只读系统错误，不伪装成 HTTP 5xx
		}
		return out, err
	}
	applyOidcParamsState(&out, st)
	return out, nil
}

// GetOidcForProvider 按指定提供商读取面板配置（目标提供商切换专用）：
// Secret 始终空回显；返回目标已存的非 Secret 字段、完整状态枚举与固定损坏/重填提示。
func (s *AdminService) GetOidcForProvider(ctx context.Context, providerType string) (OidcSettings, error) {
	if !slices.Contains(validProviders, providerType) {
		return OidcSettings{}, fmt.Errorf("%w: 提供商类型无效", ErrBadRequest)
	}
	out := OidcSettings{ProviderType: providerType}
	out.FrontendURL = s.cfg.GetOr(ctx, KeyFrontendURL)
	out.CallbackURL = s.cfg.GetOr(ctx, KeyCallbackURL)
	st, err := s.oidcOps.DescribeParams(ctx, providerType)
	if err != nil {
		if errors.Is(err, ErrSigningKeyUnavailable) {
			if st.State == "" {
				st.State = OidcParamsSigningKeyFault
			}
			applyOidcParamsState(&out, st)
			return out, nil
		}
		return out, err
	}
	applyOidcParamsState(&out, st)
	return out, nil
}

// validateOidcSecretReuseState 空 Secret 保存事务内校验：目标状态必须可用、三字段完全一致。
// SaveOidc 与底层 oidc.SaveParamsTx 共用同一判定，避免校验后配置变化。
func validateOidcSecretReuseState(in OidcSettings, st OidcParamsState) error {
	if in.ProviderType == "mock" {
		return nil // mock 无 Client Secret 复用语义
	}
	switch EffectiveOidcParamsState(st) {
	case OidcParamsSigningKeyFault:
		return ErrSigningKeyUnavailable
	case OidcParamsJSONDamaged:
		return fmt.Errorf("%w: 目标提供商已存 OIDC 参数 JSON 损坏，请重新填写必要参数并输入新的 Client Secret", ErrBadRequest)
	case OidcParamsSecretDamaged:
		return fmt.Errorf("%w: 目标提供商已存 Client Secret 损坏，请输入新的 Client Secret", ErrBadRequest)
	case OidcParamsUsable:
		if !st.SecretUsable {
			return fmt.Errorf("%w: 目标提供商尚无可用 Client Secret，请输入新的 Client Secret", ErrBadRequest)
		}
		if st.BaseURL != in.BaseURL || st.Realm != in.Realm || st.ClientID != in.ClientID {
			return fmt.Errorf("%w: 目标提供商的 Base URL/Realm/Client ID 与已存配置不一致，不能留空复用旧 Client Secret，请重新输入", ErrBadRequest)
		}
		return nil
	default: // not_configured / missing_secret
		return fmt.Errorf("%w: 目标提供商尚无可用 Client Secret，请输入新的 Client Secret", ErrBadRequest)
	}
}

// allowLocalLoginTx 在写事务内读取本地登录开关；缺失按默认 true。
func (s *AdminService) allowLocalLoginTx(ctx context.Context, tx *sql.Tx) (bool, error) {
	v, err := s.cfg.GetTx(ctx, tx, KeyAllowLocalLogin)
	if err != nil {
		return false, err
	}
	if v == "" {
		return true, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%w: allow_local_login 非法", ErrBadRequest)
	}
	return b, nil
}

// SaveOidc 保存 OIDC 参数（T1：旧值读取、校验、分类、密钥操作和相关配置写入在同一 BEGIN IMMEDIATE 内）；
// 受「本地登录与 OIDC 均不可用禁止保存」约束（防认证死锁）；真实提供商 base_url 非空时必须为 HTTPS；
// 各提供商参数独立存储；Secret 空值仅可在目标字段一致且旧 Secret 可用时保留；显式新值才替换。
// R31-05：前端地址/独立回调地址保存即时生效；空 callback_url=不修改，clear_callback_url=true 显式清除并回退推导。
func (s *AdminService) SaveOidc(ctx context.Context, in OidcSettings) error {
	if !slices.Contains(validProviders, in.ProviderType) {
		return fmt.Errorf("%w: 提供商类型无效", ErrBadRequest)
	}
	if in.ClientSecret == MaskedSecret {
		return fmt.Errorf("%w: 不能将脱敏占位符保存为 Client Secret，请留空保持原值或输入新 Secret", ErrBadRequest)
	}
	if in.ProviderType != "mock" && in.BaseURL != "" {
		if err := urlguard.ValidateHTTPS(in.BaseURL); err != nil {
			return fmt.Errorf("%w: OIDC Base URL 必须是 HTTPS 地址: %v", ErrBadRequest, err)
		}
	}
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		allowLocal, err := s.allowLocalLoginTx(ctx, tx)
		if err != nil {
			return err
		}
		st, err := s.oidcOps.DescribeParamsTx(ctx, tx, in.ProviderType)
		if err != nil {
			return err // 含 signing_key 故障的独立类型错误
		}
		if in.ProviderType != "mock" {
			if in.ClientSecret == "" {
				if err := validateOidcSecretReuseState(in, st); err != nil {
					return err
				}
			} else if EffectiveOidcParamsState(st) == OidcParamsJSONDamaged && (in.BaseURL == "" || in.ClientID == "") {
				return fmt.Errorf("%w: 目标提供商已存 OIDC 参数 JSON 损坏，须重新填写 Base URL 与 Client ID 后再保存", ErrBadRequest)
			}
		}
		// R31-05：地址校验与最终生效回调解析在同一写事务内完成，失败不产生任何参数/地址写入。
		requestedFrontend := strings.TrimSpace(in.FrontendURL)
		requestedCallback := strings.TrimSpace(in.CallbackURL)
		var normalizedFrontend string
		if requestedFrontend != "" {
			normalizedFrontend, err = normalizeAndValidateFrontendURL(requestedFrontend)
			if err != nil {
				return err
			}
		} else {
			currentFrontend, gerr := s.cfg.GetTx(ctx, tx, KeyFrontendURL)
			if gerr != nil {
				return gerr
			}
			normalizedFrontend = strings.TrimSpace(currentFrontend)
		}
		if in.ClearCallbackURL && requestedCallback != "" {
			return fmt.Errorf("%w: clear_callback_url 与非空 callback_url 不能同时提交", ErrBadRequest)
		}
		var normalizedCallback string
		if !in.ClearCallbackURL && requestedCallback != "" {
			normalizedCallback, err = normalizeAndValidateCallbackURL(requestedCallback)
			if err != nil {
				return err
			}
		}
		if in.ClearCallbackURL {
			if _, err := ResolveOidcCallbackURL("", normalizedFrontend); err != nil {
				return fmt.Errorf("%w: 清除独立回调地址需要有效前端地址用于推导: %v", ErrBadRequest, err)
			}
		}
		if !allowLocal && in.ProviderType != "mock" {
			var resolveErr error
			switch {
			case in.ClearCallbackURL:
				_, resolveErr = ResolveOidcCallbackURL("", normalizedFrontend)
			case normalizedCallback != "":
				_, resolveErr = ResolveOidcCallbackURL(normalizedCallback, "")
			default:
				currentCallback, gerr := s.cfg.GetTx(ctx, tx, KeyCallbackURL)
				if gerr != nil {
					return gerr
				}
				_, resolveErr = ResolveOidcCallbackURL(strings.TrimSpace(currentCallback), normalizedFrontend)
			}
			if resolveErr != nil {
				return ErrAuthDeadlock // 本地登录与 OIDC 回调地址均不可用，禁止保存
			}
		}
		if !allowLocal && !s.oidcUsableState(in, st) {
			return ErrAuthDeadlock // 本地登录与 OIDC 均不可用，禁止保存
		}
		// 各提供商参数独立存储；SaveParamsTx 在同一事务内完成分类/密钥校验/加密或保留密文。
		if err := s.oidcOps.SaveParamsTx(ctx, tx, in.ProviderType, in.BaseURL, in.Realm, in.ClientID, in.ClientSecret); err != nil {
			return err
		}
		if err := s.cfg.SetTx(ctx, tx, oidcKeyProviderType, in.ProviderType); err != nil {
			return err
		}
		if err := s.cfg.SetTx(ctx, tx, oidcKeyConfigured, "true"); err != nil {
			return err
		}
		// 前端地址/回调地址：保存即时生效；空 callback_url=不修改，clear_callback_url 显式清除后走推导回退。
		if requestedFrontend != "" {
			if err := s.cfg.SetTx(ctx, tx, KeyFrontendURL, normalizedFrontend); err != nil {
				return err
			}
		}
		if in.ClearCallbackURL {
			if err := s.cfg.SetTx(ctx, tx, KeyCallbackURL, ""); err != nil {
				return err
			}
		} else if normalizedCallback != "" {
			if err := s.cfg.SetTx(ctx, tx, KeyCallbackURL, normalizedCallback); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.oidcOps.ClearDiscCache() // 配置变更后清发现文档缓存
	return nil
}

// ClearOidc 清空 OIDC 配置（二次确认由前端负责）；同样受死锁防护约束
func (s *AdminService) ClearOidc(ctx context.Context) error {
	allowLocal := s.cfg.GetBool(ctx, KeyAllowLocalLogin, true)
	if !allowLocal {
		return ErrAuthDeadlock // 清空后 OIDC 不可用，若本地登录也关则死锁
	}
	if err := s.cfg.Set(ctx, oidcKeyConfigured, "false"); err != nil {
		return err
	}
	if err := s.cfg.Set(ctx, oidcKeyProviderType, ""); err != nil {
		return err
	}
	// 各提供商参数键保留结构置空（切换提供商类型保留已填字段的逆操作）
	for _, p := range validProviders {
		if err := s.cfg.Set(ctx, "oidc_params_"+p, ""); err != nil {
			return err
		}
	}
	s.oidcOps.ClearDiscCache()
	return nil
}

// --- OIDC 启用规则分区 ---

// GetOidcRules 回显审批开关与白名单
func (s *AdminService) GetOidcRules(ctx context.Context) (approvalOn bool, wl WhitelistConfig, err error) {
	approvalOn = s.cfg.GetBool(ctx, oidcKeyApproval, false)
	raw, gerr := s.cfg.Get(ctx, oidcKeyWhitelist)
	if gerr != nil {
		return false, wl, gerr
	}
	if raw != "" {
		if jerr := json.Unmarshal([]byte(raw), &wl); jerr != nil {
			s.log.Warn("解析 OIDC 白名单配置失败", "err", jerr)
		}
	}
	return approvalOn, wl, nil
}

// SaveOidcRules 审批开关 + Role/Group 白名单（值列表 + 可配置声明路径）；
// 白名单为空时跳过校验直接激活——返回 warning 标记供前端显著警告（防静默降级）
func (s *AdminService) SaveOidcRules(ctx context.Context, approvalOn bool, wl WhitelistConfig) (warning string, err error) {
	raw, err := json.Marshal(wl)
	if err != nil {
		return "", fmt.Errorf("序列化白名单失败: %w", err)
	}
	if err := s.cfg.Set(ctx, oidcKeyApproval, strconv.FormatBool(approvalOn)); err != nil {
		return "", err
	}
	if err := s.cfg.Set(ctx, oidcKeyWhitelist, string(raw)); err != nil {
		return "", err
	}
	if approvalOn && wl.Empty() {
		warning = "白名单为空，新用户将全部直接激活"
	}
	return warning, nil
}

// --- 本地认证分区 ---

type LocalAuthSettings struct {
	AllowLocalLogin bool `json:"allow_local_login"` // 默认开
	AllowSelfReg    bool `json:"allow_selfreg"`     // 默认关
	SelfRegApproval bool `json:"selfreg_approval"`  // 默认关
}

func (s *AdminService) GetLocalAuth(ctx context.Context) LocalAuthSettings {
	return LocalAuthSettings{
		AllowLocalLogin: s.cfg.GetBool(ctx, KeyAllowLocalLogin, true),
		AllowSelfReg:    s.cfg.GetBool(ctx, KeyAllowSelfreg, false),
		SelfRegApproval: s.cfg.GetBool(ctx, KeySelfRegApproval, false),
	}
}

// SaveLocalAuth 三开关；本地登录关且 OIDC 不可用 → 禁止保存 + 显著警告（防认证死锁）。
// 可用性使用 oidcAvailable 校验实际参数与 Secret 状态，不能只信 oidc_configured 标记。
func (s *AdminService) SaveLocalAuth(ctx context.Context, in LocalAuthSettings) error {
	if !in.AllowLocalLogin && !s.oidcAvailable(ctx) {
		return ErrAuthDeadlock
	}
	for k, v := range map[string]bool{
		KeyAllowLocalLogin: in.AllowLocalLogin,
		KeyAllowSelfreg:    in.AllowSelfReg,
		KeySelfRegApproval: in.SelfRegApproval,
	} {
		if err := s.cfg.Set(ctx, k, strconv.FormatBool(v)); err != nil {
			return err
		}
	}
	return nil
}

// --- 验证码分区 ---

type CaptchaSettings struct {
	Provider  string   `json:"provider"`   // recaptcha/turnstile/off
	SiteKey   string   `json:"site_key"`   // 明文存储；PUT 空=不修改
	SecretKey string   `json:"secret_key"` // 明文存储；PUT 空=不修改
	Pages     []string `json:"pages"`      // register/login/forgot
}

// GetCaptcha 回显验证码配置（双密钥返回明文：非敏感配置，切换提供商/停用后可复用，Design1 §3.4.8）
func (s *AdminService) GetCaptcha(ctx context.Context) CaptchaSettings {
	return CaptchaSettings{
		Provider:  mustStr(s.cfg.Get(ctx, captchaKeyProvider)),
		SiteKey:   mustStr(s.cfg.Get(ctx, captchaKeySiteKey)),
		SecretKey: mustStr(s.cfg.Get(ctx, captchaKeySecretKey)),
		Pages:     s.cfg.GetJSONStringSlice(ctx, captchaKeyPages),
	}
}

// SaveCaptcha 提供商 + 双密钥 + 启用页面；勾选未配密钥 → 校验拦截（防静默降级，Design1 §3.2）；
// 双密钥明文落库（非敏感配置）：空=不修改，停用/切换提供商后密钥保留可复用
func (s *AdminService) SaveCaptcha(ctx context.Context, in CaptchaSettings) error {
	if in.Provider != "off" && in.Provider != "recaptcha" && in.Provider != "turnstile" {
		return fmt.Errorf("%w: 验证码提供商无效", ErrBadRequest)
	}
	if in.Provider != "off" && len(in.Pages) > 0 {
		existingSite := s.cfg.GetOr(ctx, captchaKeySiteKey)
		existingSecret := s.cfg.GetOr(ctx, captchaKeySecretKey)
		if (in.SiteKey == "" && existingSite == "") || (in.SecretKey == "" && existingSecret == "") {
			return ErrCaptchaKeyMissing
		}
	}
	if err := s.cfg.Set(ctx, captchaKeyProvider, in.Provider); err != nil {
		return err
	}
	if in.SiteKey != "" {
		if err := s.cfg.Set(ctx, captchaKeySiteKey, in.SiteKey); err != nil {
			return err
		}
	}
	if in.SecretKey != "" {
		if err := s.cfg.Set(ctx, captchaKeySecretKey, in.SecretKey); err != nil {
			return err
		}
	}
	pages, err := json.Marshal(in.Pages)
	if err != nil {
		return err
	}
	return s.cfg.Set(ctx, captchaKeyPages, string(pages))
}

// --- SMTP 分区 ---

type SMTPSettings struct {
	Host               string   `json:"host"`
	Port               string   `json:"port"`
	User               string   `json:"user"`
	Password           string   `json:"password"` // GET 始终为空；PUT 空=保持原值
	PasswordConfigured bool     `json:"password_configured"`
	From               string   `json:"from"`
	Security           string   `json:"security"` // starttls/implicit_tls/plain
	AuthRequired       *bool    `json:"auth_required"`
	Configured         bool     `json:"configured"`
	Scopes             []string `json:"scopes"` // password_reset/approval_notify/welcome
}

func (s *AdminService) GetSMTP(ctx context.Context) SMTPSettings {
	security := mustStr(s.cfg.Get(ctx, "smtp_security"))
	if security == "" && s.cfg.GetOr(ctx, "smtp_host") == "" {
		security = SMTPSecurityStartTLS
	}
	if security != SMTPSecurityStartTLS && security != SMTPSecurityImplicitTLS && security != SMTPSecurityPlain {
		security = ""
	}
	authRequired := s.cfg.GetOr(ctx, SMTPAuthRequiredKey) != "false"
	port := mustStr(s.cfg.Get(ctx, "smtp_port"))
	if port == "" {
		port = "587"
	}
	return SMTPSettings{
		Host:               mustStr(s.cfg.Get(ctx, "smtp_host")),
		Port:               port,
		User:               mustStr(s.cfg.Get(ctx, "smtp_user")),
		Password:           "",
		PasswordConfigured: mustStr(s.cfg.Get(ctx, "smtp_password")) != "",
		From:               mustStr(s.cfg.Get(ctx, "smtp_from")),
		Security:           security,
		AuthRequired:       &authRequired,
		Configured:         SMTPConfigured(ctx, s.cfg),
		Scopes:             s.cfg.GetJSONStringSlice(ctx, "smtp_enabled_scopes"),
	}
}

// SaveSMTP 完整校验后原子保存服务器、连接方式、认证与启用范围。
func (s *AdminService) SaveSMTP(ctx context.Context, in SMTPSettings) error {
	in.Host = strings.TrimSpace(in.Host)
	in.Port = strings.TrimSpace(in.Port)
	in.From = strings.TrimSpace(in.From)
	in.User = strings.TrimSpace(in.User)
	if in.Host == "" || strings.ContainsAny(in.Host, " \t\r\n/") {
		return fmt.Errorf("%w: SMTP 服务器无效", ErrBadRequest)
	}
	port, err := strconv.Atoi(in.Port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("%w: SMTP 端口须为 1–65535", ErrBadRequest)
	}
	addr, err := mail.ParseAddress(in.From)
	if err != nil || addr.Address != in.From {
		return fmt.Errorf("%w: 发件邮箱须为单一邮箱地址", ErrBadRequest)
	}
	if in.Password == "***" {
		return fmt.Errorf("%w: 不能将脱敏占位符保存为 SMTP 密码，请留空保持原密码或输入新密码", ErrBadRequest)
	}
	if in.AuthRequired == nil {
		return fmt.Errorf("%w: 请选择是否需要 SMTP 认证", ErrBadRequest)
	}
	if in.Security != SMTPSecurityStartTLS && in.Security != SMTPSecurityImplicitTLS && in.Security != SMTPSecurityPlain {
		return fmt.Errorf("%w: SMTP 连接方式无效", ErrBadRequest)
	}
	if in.Security == SMTPSecurityPlain && (*in.AuthRequired || !SMTPLoopbackHost(in.Host)) {
		return fmt.Errorf("%w: 无加密仅支持回环地址上的无认证中继", ErrBadRequest)
	}
	if *in.AuthRequired && in.User == "" {
		return fmt.Errorf("%w: SMTP 认证账号必填", ErrBadRequest)
	}
	for _, scope := range in.Scopes {
		if scope != "password_reset" && scope != "approval_notify" && scope != "welcome" {
			return fmt.Errorf("%w: 邮件启用范围无效", ErrBadRequest)
		}
	}
	scopes, err := json.Marshal(in.Scopes)
	if err != nil {
		return err
	}
	tx, err := s.store.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if *in.AuthRequired {
		password := in.Password
		if password == "" {
			password, err = s.cfg.GetTx(ctx, tx, "smtp_password")
			if err != nil {
				return err
			}
		}
		if password == "" || password == "***" {
			return fmt.Errorf("%w: SMTP 认证密码无效，请重新填写专用密码", ErrBadRequest)
		}
		if in.Password != "" {
			if err := s.cfg.SetTx(ctx, tx, "smtp_password", in.Password); err != nil {
				return err
			}
		}
	} else {
		in.User = ""
		if _, err := tx.ExecContext(ctx, `DELETE FROM system_config WHERE key = 'smtp_password'`); err != nil {
			return err
		}
	}
	for _, item := range [][2]string{
		{"smtp_host", in.Host}, {"smtp_port", strconv.Itoa(port)}, {"smtp_user", in.User},
		{"smtp_from", in.From}, {"smtp_security", in.Security},
		{SMTPAuthRequiredKey, strconv.FormatBool(*in.AuthRequired)}, {"smtp_enabled_scopes", string(scopes)},
	} {
		if err := s.cfg.SetTx(ctx, tx, item[0], item[1]); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM system_config WHERE key = 'smtp_tls'`); err != nil {
		return err
	}
	return tx.Commit()
}

// --- 站点信息分区 ---

const (
	MaxSiteNameLen = 50
	MaxIconSize    = 2 << 20 // 2MB
	siteIconDir    = "public/site"
)

// allowedIconExts 扩展名白名单（排除 SVG/GIF：SVG 可内嵌脚本构成存储型 XSS，Design1 §3.4.8）
var allowedIconExts = map[string]bool{"png": true, "jpeg": true, "jpg": true, "webp": true, "ico": true}

type SiteInfo struct {
	Name    string `json:"site_name"`
	IconURL string `json:"icon_url"` // 空 = 默认 ICON
}

func (s *AdminService) GetSiteInfo(ctx context.Context) SiteInfo {
	return SiteInfo{
		Name:    mustStr(s.cfg.Get(ctx, "site_name")),
		IconURL: mustStr(s.cfg.Get(ctx, "site_icon_url")),
	}
}

// SaveSiteInfo 名称 ≤50 字符；ICON 上传 ≤2MB + 扩展名白名单；存 /public/site/ 固定路径覆盖即更新；
// 引用带版本参数 ?v=更新序号（避免 CDN/浏览器缓存旧图，Design1 §4.7）
func (s *AdminService) SaveSiteInfo(ctx context.Context, name string, icon io.Reader, iconFilename string) error {
	if utf8.RuneCountInString(name) > MaxSiteNameLen {
		return fmt.Errorf("%w: 站点名称不超过 50 字符", ErrBadRequest)
	}
	if err := s.cfg.Set(ctx, "site_name", name); err != nil {
		return err
	}
	if icon == nil {
		return nil // 仅改名称
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filepath.Base(iconFilename))), ".")
	if !allowedIconExts[ext] {
		return fmt.Errorf("%w: ICON 仅支持 png/jpeg/webp/ico", ErrBadRequest)
	}
	data, err := io.ReadAll(io.LimitReader(icon, MaxIconSize+1))
	if err != nil {
		return fmt.Errorf("读取 ICON 失败: %w", err)
	}
	if len(data) > MaxIconSize {
		return fmt.Errorf("%w: ICON 超过 2MB 限制", ErrBadRequest)
	}
	if len(data) == 0 {
		return fmt.Errorf("%w: ICON 文件为空", ErrBadRequest)
	}
	full := filepath.Join(s.dataDir, siteIconDir, "icon."+ext) // 固定路径覆盖即更新
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return fmt.Errorf("写入 ICON 失败: %w", err)
	}
	// 版本参数递增（前端引用 ?v=N 避免缓存旧图）
	ver := s.cfg.GetInt(ctx, "site_icon_version", 0) + 1
	if err := s.cfg.Set(ctx, "site_icon_version", strconv.Itoa(ver)); err != nil {
		return err
	}
	return s.cfg.Set(ctx, "site_icon_url", "/public/site/icon."+ext+"?v="+strconv.Itoa(ver))
}

// DeleteSiteIcon 删除恢复默认（清 site_icon_url，前端回退默认 ICON）
func (s *AdminService) DeleteSiteIcon(ctx context.Context) error {
	matches, err := filepath.Glob(filepath.Join(s.dataDir, siteIconDir, "icon.*"))
	if err != nil {
		return fmt.Errorf("扫描 ICON 文件失败: %w", err)
	}
	for _, f := range matches {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("删除站点 ICON 文件失败", "file", f, "err", err)
		}
	}
	if err := s.cfg.Set(ctx, "site_icon_url", ""); err != nil {
		return err
	}
	return s.cfg.Set(ctx, "site_icon_version", "0")
}

// --- 速率限制分区 ---

type RateLimitSettings struct {
	Login         int `json:"login"`
	Register      int `json:"register"`
	Forgot        int `json:"forgot"`
	Download      int `json:"download"`
	ResetValidate int `json:"reset_validate"`

	// HTTP 连接防护（0 = 旧前端未提交，保存时取默认值）
	HTTPReadHeaderTimeoutSec int `json:"http_read_header_timeout_sec"`
	HTTPReadTimeoutSec       int `json:"http_read_timeout_sec"`
	HTTPWriteTimeoutSec      int `json:"http_write_timeout_sec"`
	HTTPIdleTimeoutSec       int `json:"http_idle_timeout_sec"`
	HTTPMaxBodyMb            int `json:"http_max_body_mb"`
}

func (s *AdminService) GetRateLimit(ctx context.Context) RateLimitSettings {
	return RateLimitSettings{
		Login:                    s.cfg.GetInt(ctx, ratelimitKeyLogin, 10),
		Register:                 s.cfg.GetInt(ctx, ratelimitKeyReg, 5),
		Forgot:                   s.cfg.GetInt(ctx, ratelimitKeyForgot, 5),
		Download:                 s.cfg.GetInt(ctx, ratelimitKeyDown, 20),
		ResetValidate:            s.cfg.GetInt(ctx, ratelimitKeyResetValidate, 10),
		HTTPReadHeaderTimeoutSec: s.cfg.GetInt(ctx, httpReadHeaderTimeoutSecKey, 5),
		HTTPReadTimeoutSec:       s.cfg.GetInt(ctx, httpReadTimeoutSecKey, 60),
		HTTPWriteTimeoutSec:      s.cfg.GetInt(ctx, httpWriteTimeoutSecKey, 300),
		HTTPIdleTimeoutSec:       s.cfg.GetInt(ctx, httpIdleTimeoutSecKey, 120),
		HTTPMaxBodyMb:            s.cfg.GetInt(ctx, httpMaxBodyMbKey, 4),
	}
}

// SaveRateLimit 保存限流值与 HTTP 连接防护值。旧前端未提交新字段时，0 视为采用默认值。
func (s *AdminService) SaveRateLimit(ctx context.Context, in RateLimitSettings) error {
	for k, v := range map[string]int{
		ratelimitKeyLogin:  in.Login,
		ratelimitKeyReg:    in.Register,
		ratelimitKeyForgot: in.Forgot,
		ratelimitKeyDown:   in.Download,
	} {
		if v <= 0 {
			return fmt.Errorf("%w: 限流值必须为正整数", ErrBadRequest)
		}
		if err := s.cfg.Set(ctx, k, strconv.Itoa(v)); err != nil {
			return err
		}
	}
	// 兼容旧前端未提交 reset_validate：0 采用默认 10。
	resetValidate := in.ResetValidate
	if resetValidate == 0 {
		resetValidate = 10
	}
	if resetValidate < 1 {
		return fmt.Errorf("%w: 限流值必须为正整数", ErrBadRequest)
	}
	if err := s.cfg.Set(ctx, ratelimitKeyResetValidate, strconv.Itoa(resetValidate)); err != nil {
		return err
	}
	type hardening struct {
		key string
		val int
		def int
		max int
	}
	for _, h := range []hardening{
		{httpReadHeaderTimeoutSecKey, in.HTTPReadHeaderTimeoutSec, 5, 60},
		{httpReadTimeoutSecKey, in.HTTPReadTimeoutSec, 60, 3600},
		{httpWriteTimeoutSecKey, in.HTTPWriteTimeoutSec, 300, 3600},
		{httpIdleTimeoutSecKey, in.HTTPIdleTimeoutSec, 120, 3600},
		{httpMaxBodyMbKey, in.HTTPMaxBodyMb, 4, 320},
	} {
		v := h.val
		if v == 0 {
			v = h.def
		}
		if v < 1 || v > h.max {
			return fmt.Errorf("%w: HTTP 防护值超范围", ErrBadRequest)
		}
		if err := s.cfg.Set(ctx, h.key, strconv.Itoa(v)); err != nil {
			return err
		}
	}
	return nil
}

// --- 日志级别分区 ---

func (s *AdminService) GetLogLevel(ctx context.Context) string {
	v := mustStr(s.cfg.Get(ctx, KeyLogLevel))
	if v == "" {
		return "info"
	}
	return v
}

// SetLogLevel debug/info/warn/error 单选；运行时切换立即生效并持久化（运行日志与实时日志流同步生效）
func (s *AdminService) SetLogLevel(ctx context.Context, level string) error {
	if !slices.Contains([]string{"debug", "info", "warn", "error"}, level) {
		return fmt.Errorf("%w: 日志级别无效", ErrBadRequest)
	}
	if err := s.cfg.Set(ctx, KeyLogLevel, level); err != nil {
		return err
	}
	if s.level != nil { // 只切换当前运行时实例的 LevelVar，不影响其他 Runtime
		switch level {
		case "debug":
			s.level.Set(slog.LevelDebug)
		case "warn":
			s.level.Set(slog.LevelWarn)
		case "error":
			s.level.Set(slog.LevelError)
		default:
			s.level.Set(slog.LevelInfo)
		}
	}
	return nil
}

// --- 公告与页脚分区（R10-07：首页公告 / 登录页公告 / 登录页页脚三份独立配置；前端 markdown-it html:false 渲染 MD，禁原始 HTML 防存储型 XSS）---

const (
	MaxAnnouncementLen = 2000
	MaxFooterLen       = 2000
)

// GetAnnouncement 首页公告（键 announcement，R10-07 前为登录页+首页共用，拆分后语义为首页）
func (s *AdminService) GetAnnouncement(ctx context.Context) string {
	return mustStr(s.cfg.Get(ctx, "announcement"))
}

// SaveAnnouncement 首页公告（MD 源 ≤2000 字符；前端 markdown-it html:false 渲染，原始 HTML 按文本转义）
func (s *AdminService) SaveAnnouncement(ctx context.Context, content string) error {
	if utf8.RuneCountInString(content) > MaxAnnouncementLen {
		return fmt.Errorf("%w: 首页公告不超过 2000 字符", ErrBadRequest)
	}
	return s.cfg.Set(ctx, "announcement", content)
}

// GetLoginAnnouncement 登录页公告（R10-07 新增独立配置）
func (s *AdminService) GetLoginAnnouncement(ctx context.Context) string {
	return mustStr(s.cfg.Get(ctx, "login_announcement"))
}

// SaveLoginAnnouncement 登录页公告 ≤2000 字符（同首页公告：MD 渲染，禁原始 HTML）
func (s *AdminService) SaveLoginAnnouncement(ctx context.Context, content string) error {
	if utf8.RuneCountInString(content) > MaxAnnouncementLen {
		return fmt.Errorf("%w: 登录页公告不超过 2000 字符", ErrBadRequest)
	}
	return s.cfg.Set(ctx, "login_announcement", content)
}

// GetLoginFooter 登录页页脚（R10-07）
func (s *AdminService) GetLoginFooter(ctx context.Context) string {
	return mustStr(s.cfg.Get(ctx, "login_footer"))
}

// SaveLoginFooter 登录页页脚 ≤2000 字符（同公告：MD 渲染，禁原始 HTML）
func (s *AdminService) SaveLoginFooter(ctx context.Context, content string) error {
	if utf8.RuneCountInString(content) > MaxFooterLen {
		return fmt.Errorf("%w: 登录页页脚不超过 2000 字符", ErrBadRequest)
	}
	return s.cfg.Set(ctx, "login_footer", content)
}

// --- 调试模式分区 ---

func (s *AdminService) GetDebug(ctx context.Context) bool {
	return s.cfg.GetBool(ctx, "debug_mode", false)
}

// SetDebug 开启后 5xx 返回详细内部信息（生产默认关闭，状态持久化）；
// server.Fail 的 5xx 脱敏分支读取 debug_mode（Build1 Step 1 的 Fail 在此接通）
func (s *AdminService) SetDebug(ctx context.Context, on bool) error {
	return s.cfg.Set(ctx, "debug_mode", strconv.FormatBool(on))
}

// --- 高级模式分区（Build7 Step2） ---

// AdvancedSettings 高级模式相关设置。
type AdvancedSettings struct {
	AdvancedMode           bool `json:"advanced_mode"`
	CollectIntervalMinutes int  `json:"collect_interval_minutes"`
	TrafficCardEnabled     bool `json:"traffic_card_enabled"`
}

// GetAdvancedSettings 读取高级模式三键。
func (s *AdminService) GetAdvancedSettings(ctx context.Context) AdvancedSettings {
	return AdvancedSettings{
		AdvancedMode:           s.cfg.GetBool(ctx, KeyAdvancedMode, false),
		CollectIntervalMinutes: s.cfg.GetInt(ctx, "xray_collect_interval_minutes", 10),
		TrafficCardEnabled:     s.cfg.GetBool(ctx, "traffic_card_enabled", true),
	}
}

// SaveAdvancedSettings 保存高级模式设置。
// advanced_mode 变更必须经注入的 AdvancedModeSwitcher 执行 OFF/ON 分支，禁止普通 Set 绕过。
// confirmWord 仅关闭高级模式时需要（固定 DISABLE，由接入层校验）。
func (s *AdminService) SaveAdvancedSettings(ctx context.Context, in AdvancedSettings, confirmWord string) (string, error) {
	if in.CollectIntervalMinutes < 1 {
		return "", fmt.Errorf("%w: 采集间隔必须 ≥1 分钟", ErrBadRequest)
	}
	current := s.cfg.GetBool(ctx, KeyAdvancedMode, false)
	taskID := ""
	if in.AdvancedMode != current {
		if s.advancedSwitcher == nil {
			return "", errors.New("高级模式开关实现未注入")
		}
		var err error
		taskID, err = s.advancedSwitcher.SubmitAdvancedMode(ctx, in.AdvancedMode, confirmWord)
		if err != nil {
			return "", err
		}
	}
	if err := s.cfg.Set(ctx, "xray_collect_interval_minutes", strconv.Itoa(in.CollectIntervalMinutes)); err != nil {
		return "", err
	}
	if err := s.cfg.Set(ctx, "traffic_card_enabled", strconv.FormatBool(in.TrafficCardEnabled)); err != nil {
		return "", err
	}
	return taskID, nil
}

// --- 辅助 ---

// mustStr 单值读取（错误按空处理；面板回显场景配置键缺失属正常态）
func mustStr(v string, _ error) string { return v }
