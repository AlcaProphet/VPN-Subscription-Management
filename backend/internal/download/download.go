// Package download 提供下载解析服务：三态 Token 实时解析、附加响应头注入与访问日志。
package download

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/version"
)

// 业务错误（接入层映射：ErrTokenInvalid → 404；ErrUnassigned → HTTP 200 注释块）
var (
	ErrTokenInvalid = errors.New("token_invalid") // 无效/标识不一致 → 404，不泄露差异信息
	ErrUnassigned   = errors.New("unassigned")    // 平台无订阅条目 → HTTP 200 注释块
)

// Service 下载解析服务
type Service struct {
	store    *store.Store
	versions *version.Service
	cfg      *config.Service
	log      *slog.Logger

	// renderUser 装配生成模板的用户动态渲染器（Build6 Step4 由装配侧注入；nil 时保持旧逻辑）
	renderUser func(ctx context.Context, subID, userID int64, content []byte, fileName string) ([]byte, error)
}

func NewService(st *store.Store, versions *version.Service, cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{store: st, versions: versions, cfg: cfg, log: lg}
}

// SetRenderUser 注入用户下载动态渲染函数（Build6 Step4）。
func (s *Service) SetRenderUser(fn func(ctx context.Context, subID, userID int64, content []byte, fileName string) ([]byte, error)) {
	s.renderUser = fn
}

// Result 下载结果
type Result struct {
	Content      []byte
	ExtraHeaders map[string]string // 平台级附加头（{frontend_url} 已替换）
	Filename     string            // 分享/规则下载的文件名（资源名称；空则用标识）
}

// AccessEntry 访问日志写入参数；ResourceID 由写入时转换为 slug（无标识解析失败记平台标识）
type AccessEntry struct {
	UserID     int64
	Type       string // subscription/custom/explicit/share/rule
	Platform   string // 平台标识
	ResourceID int64  // 解析结果资源 ID（0 = 无）
	FailReason string // token_invalid/unassigned/...
}

// ResolveUserDownload 按 Token 查记录 → 三态解析；
// URL 路径中的平台标识必须与 Token 绑定一致，不一致与无效 Token 同等对待（ErrTokenInvalid，不泄露差异）
func (s *Service) ResolveUserDownload(ctx context.Context, tokenValue, platformSlug string) (*Result, *AccessEntry, error) {
	var rec struct {
		UserID         int64
		PlatformID     int64
		PlatformSlug   string
		CustomSubID    int64
		SubscriptionID int64
	}
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT dt.user_id, dt.platform_id, p.slug, COALESCE(dt.custom_sub_id,0), COALESCE(dt.subscription_id,0)
		 FROM download_tokens dt JOIN platforms p ON p.id = dt.platform_id WHERE dt.token = ?`, tokenValue).
		Scan(&rec.UserID, &rec.PlatformID, &rec.PlatformSlug, &rec.CustomSubID, &rec.SubscriptionID)
	if errors.Is(err, sql.ErrNoRows) || rec.PlatformSlug != platformSlug {
		return nil, &AccessEntry{Platform: platformSlug, FailReason: "token_invalid"}, ErrTokenInvalid
	}
	if err != nil {
		return nil, nil, err
	}
	switch {
	case rec.CustomSubID != 0: // 自定义：直接返回自定义内容
		content, fileName, err := s.versions.ReadCurrentWithName(ctx, version.OwnerCustom, rec.CustomSubID)
		if err != nil {
			return nil, nil, err
		}
		return s.withPlatformHeaders(ctx, content, fileName, rec.PlatformID, "custom", rec.CustomSubID, rec.UserID)
	case rec.SubscriptionID != 0: // 显式：实时校验持有人当前仍为管理员
		var role string
		if err := s.store.DB().QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, rec.UserID).Scan(&role); err != nil || role != "admin" {
			return nil, &AccessEntry{UserID: rec.UserID, Platform: platformSlug, FailReason: "token_invalid"}, ErrTokenInvalid
		}
		content, fileName, err := s.versions.ReadCurrentWithName(ctx, version.OwnerSubscription, rec.SubscriptionID)
		if errors.Is(err, version.ErrVersionNotFound) {
			// 无版本：带 fail_reason 的 entry 供访问日志记录（R07-05）
			return nil, &AccessEntry{UserID: rec.UserID, Platform: platformSlug, Type: "explicit", ResourceID: rec.SubscriptionID, FailReason: "version_missing"}, err
		}
		if err != nil {
			return nil, nil, err
		}
		if s.renderUser != nil {
			content, err = s.renderUser(ctx, rec.SubscriptionID, rec.UserID, content, fileName)
		} else {
			content, err = s.maybeEncodeSubscriptionContent(ctx, rec.SubscriptionID, content)
		}
		if err != nil {
			return nil, nil, err
		}
		return s.withPlatformHeaders(ctx, content, fileName, rec.PlatformID, "explicit", rec.SubscriptionID, rec.UserID)
	default: // 无标识：按平台读唯一订阅条目（Design2 §4.4/§5.10）
		var subID int64
		err := s.store.DB().QueryRowContext(ctx,
			`SELECT id FROM subscriptions WHERE platform_id = ?`, rec.PlatformID).Scan(&subID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, &AccessEntry{UserID: rec.UserID, Platform: platformSlug, FailReason: "unassigned"}, ErrUnassigned
		}
		if err != nil {
			return nil, nil, err
		}
		content, fileName, err := s.versions.ReadCurrentWithName(ctx, version.OwnerSubscription, subID)
		if errors.Is(err, version.ErrVersionNotFound) {
			// 平台有订阅条目但无激活版本：带 fail_reason 的 entry 供访问日志记录
			return nil, &AccessEntry{UserID: rec.UserID, Platform: platformSlug, Type: "subscription", ResourceID: subID, FailReason: "no_active_version"}, err
		}
		if err != nil {
			return nil, nil, err
		}
		if s.renderUser != nil {
			content, err = s.renderUser(ctx, subID, rec.UserID, content, fileName)
		} else {
			content, err = s.maybeEncodeSubscriptionContent(ctx, subID, content)
		}
		if err != nil {
			return nil, nil, err
		}
		return s.withPlatformHeaders(ctx, content, fileName, rec.PlatformID, "subscription", subID, rec.UserID)
	}
}

// maybeEncodeSubscriptionContent 订阅下载下发前整体 base64 编码：
// 仅 product_type ∈ {subs, generic-subs} 且当前激活版本为装配生成模板时编码；直接上传内容原样返回。
func (s *Service) maybeEncodeSubscriptionContent(ctx context.Context, subID int64, content []byte) ([]byte, error) {
	var productType string
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT product_type FROM subscriptions WHERE id = ?`, subID).Scan(&productType); err != nil {
		return nil, err
	}
	if productType != "subs" && productType != "generic-subs" {
		return content, nil
	}
	var n int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 WHERE v.owner_type = 'subscription' AND v.owner_id = ?
		   AND v.version_no = (SELECT current_version FROM subscriptions WHERE id = ?)`,
		subID, subID).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return content, nil
	}
	return []byte(base64.StdEncoding.EncodeToString(content)), nil
}

// withPlatformHeaders 附加响应头注入（{frontend_url} 占位符替换为当前前端地址）+ 下载文件名（资源名 + 原始扩展名）
func (s *Service) withPlatformHeaders(ctx context.Context, content []byte, fileName string, platformID int64, dlType string, resID, userID int64) (*Result, *AccessEntry, error) {
	headers := map[string]string{}
	var raw string
	var platformSlug string
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT slug, extra_headers FROM platforms WHERE id = ?`, platformID).Scan(&platformSlug, &raw); err != nil {
		return nil, nil, err
	}
	parsed := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		s.log.Warn("解析平台附加头失败", "platform_id", platformID, "err", err)
	} else {
		frontendURL, _ := s.cfg.Get(ctx, config.KeyFrontendURL)
		for k, v := range parsed {
			headers[k] = strings.ReplaceAll(v, "{frontend_url}", frontendURL)
		}
	}
	// 高级模式系统注入：profile 头覆盖平台同键，subscription-userinfo 仅用户订阅类携带
	if s.cfg.GetBool(ctx, config.KeyAdvancedMode, false) && (dlType == "subscription" || dlType == "custom" || dlType == "explicit") {
		frontendURL, _ := s.cfg.Get(ctx, config.KeyFrontendURL)
		headers["profile-update-interval"] = "6"
		headers["profile-web-page-url"] = frontendURL
		up, down, total, err := s.userUsage(ctx, userID)
		if err != nil {
			return nil, nil, err
		}
		parts := []string{fmt.Sprintf("upload=%d", up), fmt.Sprintf("download=%d", down)}
		if total > 0 {
			parts = append(parts, fmt.Sprintf("total=%d", total))
		}
		parts = append(parts, "expire=4102444800")
		headers["subscription-userinfo"] = strings.Join(parts, "; ")
	}
	// 下载文件名：资源名 + 原始文件扩展名（保留上传格式，Issue1 R03）；装配模板按 target_syntax 映射
	var resName string
	switch dlType {
	case "custom": // 自定义订阅无名称，用标识
		if err := s.store.DB().QueryRowContext(ctx, `SELECT slug FROM custom_subscriptions WHERE id = ?`, resID).Scan(&resName); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, nil, err
		}
	default: // subscription / explicit：用订阅名称
		if err := s.store.DB().QueryRowContext(ctx, `SELECT name FROM subscriptions WHERE id = ?`, resID).Scan(&resName); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, nil, err
		}
	}
	ext := ".yaml"
	if dlType == "subscription" || dlType == "explicit" {
		if t, ok, err := s.blueprintTargetSyntax(ctx, resID); err != nil {
			return nil, nil, err
		} else if ok {
			ext = map[string]string{"clash-yaml": ".yaml", "sr-subs": ".txt", "generic-subs": ".txt", "sr-conf": ".conf"}[t]
		}
	}
	return &Result{Content: content, Filename: joinDownloadName(resName, fileName, ext), ExtraHeaders: headers},
		&AccessEntry{UserID: userID, Type: dlType, Platform: platformSlug, ResourceID: resID}, nil
}

// userUsage 返回用户当月用量与有效配额（字节）；配额 NULL/0 时 total=0。
func (s *Service) userUsage(ctx context.Context, userID int64) (up, down, total int64, err error) {
	ym := currentYM()
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(SUM(uplink),0), COALESCE(SUM(downlink),0) FROM traffic_records WHERE user_id = ? AND ym = ?`,
		userID, ym).Scan(&up, &down); err != nil {
		return 0, 0, 0, err
	}
	var quota sql.NullFloat64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(u.quota_override, g.default_quota)
		 FROM users u LEFT JOIN groups g ON g.id = u.group_id WHERE u.id = ?`, userID).Scan(&quota); err != nil {
		return 0, 0, 0, err
	}
	if quota.Valid && quota.Float64 > 0 {
		total = int64(quota.Float64 * 1024 * 1024 * 1024)
	}
	return up, down, total, nil
}

// blueprintTargetSyntax 返回当前激活装配蓝图的 target_syntax；无蓝图时 ok=false。
func (s *Service) blueprintTargetSyntax(ctx context.Context, subID int64) (string, bool, error) {
	var t string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT b.target_syntax
		 FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 WHERE v.owner_type = 'subscription' AND v.owner_id = ?
		   AND v.version_no = (SELECT current_version FROM subscriptions WHERE id = ?)`,
		subID, subID).Scan(&t)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return t, true, nil
}

// currentYM 返回 UTC 当前月份 YYYY-MM。
func currentYM() string {
	return time.Now().UTC().Format("2006-01")
}

// PreviewForUser 会话凭据预览（Design2 §4.4/§5.10）：
// 管理员返回当前激活版本原文；普通用户有自定义订阅时优先返回自定义内容，
// 订阅装配模板按自身动态渲染（走注入的 renderUser），直接上传内容原样返回。
func (s *Service) PreviewForUser(ctx context.Context, isAdmin bool, userID int64, platformSlug string) ([]byte, error) {
	var platformID int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT id FROM platforms WHERE slug = ?`, platformSlug).Scan(&platformID); err != nil {
		return nil, ErrTokenInvalid // 平台不存在与无权限同等对待
	}
	// 分发优先级：自定义 → 平台唯一订阅 → 未分配
	var customID int64
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, platformID).Scan(&customID)
	if err == nil {
		return s.versions.ReadCurrent(ctx, version.OwnerCustom, customID)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var subID int64
	err = s.store.DB().QueryRowContext(ctx,
		`SELECT id FROM subscriptions WHERE platform_id = ?`, platformID).Scan(&subID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUnassigned
	}
	if err != nil {
		return nil, err
	}
	content, fileName, err := s.versions.ReadCurrentWithName(ctx, version.OwnerSubscription, subID)
	if err != nil {
		return nil, err
	}
	if isAdmin || s.renderUser == nil {
		return content, nil
	}
	return s.renderUser(ctx, subID, userID, content, fileName)
}

// ResolveShare 分享下载解析（Step 5 接通）：token 查 share_tokens → 读当前版本；
// 表未建立（分享资源未迁移）时与无效 Token 同等对待
func (s *Service) ResolveShare(ctx context.Context, tokenValue, slug string) (*Result, *AccessEntry, error) {
	if !tableExists(ctx, s.store.DB(), "share_subscriptions") || !tableExists(ctx, s.store.DB(), "share_tokens") {
		return nil, &AccessEntry{Platform: slug, FailReason: "token_invalid"}, ErrTokenInvalid
	}
	var shareID int64
	var resourceSlug, resourceName string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT st.share_id, ss.slug, ss.name FROM share_tokens st JOIN share_subscriptions ss ON ss.id = st.share_id
		 WHERE st.token = ? AND ss.slug = ?`, tokenValue, slug).Scan(&shareID, &resourceSlug, &resourceName)
	if errors.Is(err, sql.ErrNoRows) || resourceSlug != slug {
		return nil, &AccessEntry{Platform: slug, FailReason: "token_invalid"}, ErrTokenInvalid
	}
	if err != nil {
		return nil, nil, err
	}
	content, fileName, err := s.versions.ReadCurrentWithName(ctx, version.OwnerShare, shareID)
	if errors.Is(err, version.ErrVersionNotFound) {
		// 无版本：带 fail_reason 的 entry 供访问日志记录（R07-05）
		return nil, &AccessEntry{Type: "share", ResourceID: shareID, FailReason: "version_missing"}, err
	}
	if err != nil {
		return nil, nil, err
	}
	// 下载文件名：分享名 + 原始扩展名（保留上传格式，Issue1 R03）
	return &Result{Content: content, Filename: joinDownloadName(resourceName, fileName, ".yaml")}, &AccessEntry{Type: "share", ResourceID: shareID}, nil
}

// ResolveRule 规则下载解析（Step 6 接通）：token 查 rule_tokens → 读当前版本；表缺失与无效 Token 同等对待
func (s *Service) ResolveRule(ctx context.Context, tokenValue, slug string) (*Result, *AccessEntry, error) {
	if !tableExists(ctx, s.store.DB(), "rules") || !tableExists(ctx, s.store.DB(), "rule_tokens") {
		return nil, &AccessEntry{Platform: slug, FailReason: "token_invalid"}, ErrTokenInvalid
	}
	var ruleID int64
	var resourceSlug, resourceName string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT rt.rule_id, r.slug, r.name FROM rule_tokens rt JOIN rules r ON r.id = rt.rule_id
		 WHERE rt.token = ? AND r.slug = ?`, tokenValue, slug).Scan(&ruleID, &resourceSlug, &resourceName)
	if errors.Is(err, sql.ErrNoRows) || resourceSlug != slug {
		return nil, &AccessEntry{Platform: slug, FailReason: "token_invalid"}, ErrTokenInvalid
	}
	if err != nil {
		return nil, nil, err
	}
	content, fileName, err := s.versions.ReadCurrentWithName(ctx, version.OwnerRule, ruleID)
	if errors.Is(err, version.ErrVersionNotFound) {
		// 无版本：带 fail_reason 的 entry 供访问日志记录（R07-05）
		return nil, &AccessEntry{Type: "rule", ResourceID: ruleID, FailReason: "version_missing"}, err
	}
	if err != nil {
		return nil, nil, err
	}
	// 下载文件名：规则名 + 原始扩展名（保留上传格式，Issue1 R03）
	return &Result{Content: content, Filename: joinDownloadName(resourceName, fileName, ".conf")}, &AccessEntry{Type: "rule", ResourceID: ruleID}, nil
}

// joinDownloadName 下载文件名 = 资源名 + 原始文件扩展名（保留上传格式，Issue1 R03）；
// rawFileName 无扩展名（旧数据/文本模式兜底）时用 fallbackExt
func joinDownloadName(resName, rawFileName, fallbackExt string) string {
	ext := filepath.Ext(rawFileName)
	if ext == "" {
		ext = fallbackExt
	}
	return resName + ext
}

// tableExists 检查表是否已存在（sqlite_master 预检；供「表缺失跳过」语义使用）
func tableExists(ctx context.Context, db *sql.DB, name string) bool {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		return false
	}
	return n > 0
}

// WriteAccessLog 成功/失败均记；写入失败仅记 warn 日志，不阻断下载响应
func (s *Service) WriteAccessLog(ctx context.Context, ip string, e *AccessEntry, success bool) {
	if e == nil {
		return
	}
	status := "success"
	failReason := any(nil)
	if !success {
		status = "fail"
		failReason = e.FailReason
	}
	resourceSlug := e.Platform // 无解析结果时记平台标识（unassigned 口径）
	if e.ResourceID > 0 {
		if slugVal, err := s.slugOf(ctx, e.Type, e.ResourceID); err == nil && slugVal != "" {
			resourceSlug = slugVal
		}
	}
	userID := any(nil)
	if e.UserID > 0 {
		userID = e.UserID
	}
	if _, err := s.store.DB().ExecContext(ctx,
		`INSERT INTO access_logs (user_id, ip, download_type, platform, resource_slug, status, fail_reason)
		 VALUES (?,?,?,?,?,?,?)`,
		userID, ip, e.Type, e.Platform, resourceSlug, status, failReason); err != nil {
		s.log.Warn("写入访问日志失败", "err", err)
	}
}

// slugOf 按下载类型查询资源标识（记录口径 Build3 Step 5 核对）
func (s *Service) slugOf(ctx context.Context, dlType string, resourceID int64) (string, error) {
	table := map[string]string{
		"subscription": "subscriptions",
		"explicit":     "subscriptions",
		"custom":       "custom_subscriptions",
		"share":        "share_subscriptions",
		"rule":         "rules",
	}[dlType]
	if table == "" {
		return "", errors.New("非法下载类型")
	}
	if !tableExists(ctx, s.store.DB(), table) {
		return "", errors.New("资源表未建立")
	}
	var slugVal string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT slug FROM `+table+` WHERE id = ?`, resourceID).Scan(&slugVal)
	return slugVal, err
}

// sanitizeFilename 分享/规则下载文件名：去除/转义控制字符与引号（避免另存文件名退化）
func sanitizeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '"' || r == '\\' {
			return '_'
		}
		return r
	}, name)
	if name == "" {
		name = "download"
	}
	return name
}

// RFC5987Value 对 UTF-8 字节做 RFC 5987 百分号编码。
func RFC5987Value(value string) string {
	const hex = "0123456789ABCDEF"
	var out strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			strings.ContainsRune("!#$&+-.^_`|~", rune(c)) {
			out.WriteByte(c)
			continue
		}
		out.WriteByte('%')
		out.WriteByte(hex[c>>4])
		out.WriteByte(hex[c&0x0f])
	}
	return out.String()
}

// BuildContentDisposition 同时提供 ASCII fallback 与 CVR 优先读取的 UTF-8 filename*。
func BuildContentDisposition(displayName, fallback string) string {
	if fallback == "" {
		fallback = "subscription.yaml"
	}
	if displayName == "" {
		displayName = fallback
	}
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		sanitizeFilename(fallback), RFC5987Value(sanitizeFilename(displayName)))
}
