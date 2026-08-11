// config/admin.go：面板配置服务（Build3 Step 3）——各分区配置读写与生效逻辑（Design1 §3.4.8）。
// 敏感字段（OIDC Client Secret / SMTP 密码）加密落库、回显脱敏；验证码双密钥为明文存储（面板回显真实值）。
// 本地登录与 OIDC 均不可用禁止保存（防认证死锁）。
package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"vpn-sub/internal/log"
	"vpn-sub/internal/store"
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
	captchaKeyProvider  = "captcha_provider"
	captchaKeySiteKey   = "captcha_site_key"
	captchaKeySecretKey = "captcha_secret_key"
	captchaKeyPages     = "captcha_pages"
	ratelimitKeyLogin   = "ratelimit_login"
	ratelimitKeyReg     = "ratelimit_register"
	ratelimitKeyForgot  = "ratelimit_forgot"
	ratelimitKeyDown    = "ratelimit_download"
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

// OidcOps OIDC 能力接口（oidc.Service 经 server 适配注入；config 包避免 config↔oidc 循环依赖）
type OidcOps interface {
	// SaveParams 保存提供商参数（client_secret 加密落库；空值保留原密文）
	SaveParams(ctx context.Context, providerType, baseURL, realm, clientID, clientSecret string) error
	// LoadParams 读取提供商参数（client_secret 已解密）
	LoadParams(ctx context.Context, providerType string) (baseURL, realm, clientID, clientSecret string, err error)
	// IsConfigured OIDC 是否已配置
	IsConfigured(ctx context.Context) bool
	// ClearDiscCache 配置变更后清发现文档缓存
	ClearDiscCache()
}

// AdminService 面板配置服务
type AdminService struct {
	cfg     *Service
	store   *store.Store
	oidcOps OidcOps
	dataDir string // 数据卷根目录（站点 ICON 落盘用）
	log     *slog.Logger
}

func NewAdminService(cfg *Service, st *store.Store, oidcOps OidcOps, dataDir string, lg *slog.Logger) *AdminService {
	return &AdminService{cfg: cfg, store: st, oidcOps: oidcOps, dataDir: dataDir, log: lg}
}

// --- 通用读写辅助 ---

// getMasked 敏感字段 GET 返回脱敏值（已配置 → "***"，未配置 → ""），禁止返回明文
func (s *AdminService) getMasked(ctx context.Context, key string) string {
	v, _ := s.cfg.Get(ctx, key)
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
	ProviderType string `json:"provider_type"`
	BaseURL      string `json:"base_url"`
	Realm        string `json:"realm"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"` // GET 脱敏；PUT 空=不修改
	FrontendURL  string `json:"frontend_url"`  // 启动时缓存（库驱动），修改需重启生效
	CallbackURL  string `json:"callback_url"`  // 同上
}

// 合法提供商类型
var validProviders = []string{"keycloak", "auth0", "generic", "mock"}

// oidcUsable 判定 OIDC 是否「可用」（防认证死锁的核心判定，Design1 §3.4.8）：
// base_url 非空 且 client_id 非空 且（PUT 入参 secret 非空 或 库内已有对应密文）
func (s *AdminService) oidcUsable(ctx context.Context, in OidcSettings) bool {
	if in.BaseURL == "" || in.ClientID == "" {
		return false
	}
	if in.ClientSecret != "" {
		return true
	}
	_, _, _, secret, err := s.oidcOps.LoadParams(ctx, in.ProviderType) // 库内已有密文视为可用
	return err == nil && secret != ""
}

// GetOidc 回显当前 OIDC 配置（Secret 脱敏；frontend/callback 返回库值——启动缓存语义）
func (s *AdminService) GetOidc(ctx context.Context) (OidcSettings, error) {
	out := OidcSettings{}
	out.ProviderType, _ = s.cfg.Get(ctx, oidcKeyProviderType)
	out.FrontendURL, _ = s.cfg.Get(ctx, KeyFrontendURL)
	out.CallbackURL, _ = s.cfg.Get(ctx, KeyCallbackURL)
	if out.ProviderType != "" {
		baseURL, realm, clientID, secret, err := s.oidcOps.LoadParams(ctx, out.ProviderType)
		if err != nil {
			return out, nil // 参数缺失按未配置处理（不阻断回显）
		}
		out.BaseURL = baseURL
		out.Realm = realm
		out.ClientID = clientID
		if secret != "" {
			out.ClientSecret = "***"
		}
	}
	return out, nil
}

// SaveOidc 保存 OIDC 参数；受「本地登录与 OIDC 均不可用禁止保存」约束（防认证死锁）；
// 各提供商参数独立存储（切换类型保留已填字段）；frontend_url/callback_url 手动覆盖优先
func (s *AdminService) SaveOidc(ctx context.Context, in OidcSettings) error {
	if !slices.Contains(validProviders, in.ProviderType) {
		return fmt.Errorf("%w: 提供商类型无效", ErrBadRequest)
	}
	allowLocal := s.cfg.GetBool(ctx, KeyAllowLocalLogin, true)
	if !allowLocal && !s.oidcUsable(ctx, in) {
		return ErrAuthDeadlock // 本地登录与 OIDC 均不可用，禁止保存
	}
	// 各提供商参数独立存储（切换类型保留已填字段）；Secret 经 oidcOps 加密；空值保留原密文
	if err := s.oidcOps.SaveParams(ctx, in.ProviderType, in.BaseURL, in.Realm, in.ClientID, in.ClientSecret); err != nil {
		return err
	}
	if err := s.cfg.Set(ctx, oidcKeyProviderType, in.ProviderType); err != nil {
		return err
	}
	if err := s.cfg.Set(ctx, oidcKeyConfigured, "true"); err != nil {
		return err
	}
	// 前端地址/回调地址：手动覆盖优先（空 = 不修改）；启动缓存语义——修改需重启容器生效
	if in.FrontendURL != "" {
		if err := s.cfg.Set(ctx, KeyFrontendURL, in.FrontendURL); err != nil {
			return err
		}
	}
	if in.CallbackURL != "" {
		if err := s.cfg.Set(ctx, KeyCallbackURL, in.CallbackURL); err != nil {
			return err
		}
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

// SaveLocalAuth 三开关；本地登录关且 OIDC 不可用 → 禁止保存 + 显著警告（防认证死锁）
func (s *AdminService) SaveLocalAuth(ctx context.Context, in LocalAuthSettings) error {
	if !in.AllowLocalLogin && !s.oidcOps.IsConfigured(ctx) {
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
		existingSite, _ := s.cfg.Get(ctx, captchaKeySiteKey)
		existingSecret, _ := s.cfg.Get(ctx, captchaKeySecretKey)
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
	Host     string   `json:"host"`
	Port     string   `json:"port"`
	User     string   `json:"user"`
	Password string   `json:"password"` // GET 脱敏；PUT 空=不修改（敏感加密）
	From     string   `json:"from"`
	TLS      bool     `json:"tls"`
	Scopes   []string `json:"scopes"` // password_reset/approval_notify/welcome
}

func (s *AdminService) GetSMTP(ctx context.Context) SMTPSettings {
	return SMTPSettings{
		Host:     mustStr(s.cfg.Get(ctx, "smtp_host")),
		Port:     mustStr(s.cfg.Get(ctx, "smtp_port")),
		User:     mustStr(s.cfg.Get(ctx, "smtp_user")),
		Password: s.getMasked(ctx, "smtp_password"),
		From:     mustStr(s.cfg.Get(ctx, "smtp_from")),
		TLS:      s.cfg.GetBool(ctx, "smtp_tls", false), // 默认关闭（R10-04）
		Scopes:   s.cfg.GetJSONStringSlice(ctx, "smtp_enabled_scopes"),
	}
}

// SaveSMTP 服务器/端口/账号/密码（加密）/发件人/TLS + 启用范围（JSON 数组）
func (s *AdminService) SaveSMTP(ctx context.Context, in SMTPSettings) error {
	if in.Host == "" {
		return fmt.Errorf("%w: SMTP 服务器必填", ErrBadRequest)
	}
	for k, v := range map[string]string{
		"smtp_host": in.Host,
		"smtp_port": in.Port,
		"smtp_user": in.User,
		"smtp_from": in.From,
	} {
		if v == "" {
			continue // 空 = 不修改
		}
		if err := s.cfg.Set(ctx, k, v); err != nil {
			return err
		}
	}
	if err := s.setSensitive(ctx, "smtp_password", in.Password); err != nil {
		return err
	}
	if err := s.cfg.Set(ctx, "smtp_tls", strconv.FormatBool(in.TLS)); err != nil {
		return err
	}
	scopes, err := json.Marshal(in.Scopes)
	if err != nil {
		return err
	}
	return s.cfg.Set(ctx, "smtp_enabled_scopes", string(scopes))
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
	Login    int `json:"login"`
	Register int `json:"register"`
	Forgot   int `json:"forgot"`
	Download int `json:"download"`
}

func (s *AdminService) GetRateLimit(ctx context.Context) RateLimitSettings {
	return RateLimitSettings{
		Login:    s.cfg.GetInt(ctx, ratelimitKeyLogin, 10),
		Register: s.cfg.GetInt(ctx, ratelimitKeyReg, 5),
		Forgot:   s.cfg.GetInt(ctx, ratelimitKeyForgot, 5),
		Download: s.cfg.GetInt(ctx, ratelimitKeyDown, 20),
	}
}

// SaveRateLimit 四个数字输入；修改后立即生效（限流中间件每次请求读配置，Build1/2 已实现）
func (s *AdminService) SaveRateLimit(ctx context.Context, in RateLimitSettings) error {
	for k, v := range map[string]int{
		ratelimitKeyLogin: in.Login,
		ratelimitKeyReg:   in.Register,
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
	log.SetLevel(level) // 运行时切换：LevelVar 全局可调，立即生效
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

// --- 辅助 ---

// mustStr 单值读取（错误按空处理；面板回显场景配置键缺失属正常态）
func mustStr(v string, _ error) string { return v }
