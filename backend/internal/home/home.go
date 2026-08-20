// Package home 提供首页用户端数据业务层：平台卡片、汇总、Token 刷新与更新时间。
// R14-16：将原 server/home.go 的接入层直连 SQL 下沉到本包，server 仅做协议解析与响应。
package home

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
)

// Service 首页数据服务
type Service struct {
	store    *store.Store
	tokenSvc *token.Service
	cfg      *config.Service
}

func NewService(st *store.Store, tokenSvc *token.Service, cfg *config.Service) *Service {
	return &Service{store: st, tokenSvc: tokenSvc, cfg: cfg}
}

// InstallerFileCard 首页本地安装包条目（已拼接公开可缓存路径）
type InstallerFileCard struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// InstallerURLCard 首页外部下载链接条目
type InstallerURLCard struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// AdminPreviewSubscription 管理员平台卡片的订阅条目预览信息（每平台唯一）
type AdminPreviewSubscription struct {
	Name             string     `json:"name"`
	ProductType      string     `json:"product_type"`
	ContentKind      string     `json:"content_kind"` // blueprint / upload；无激活版本为空
	CurrentVersion   int64      `json:"current_version"`
	VersionUpdatedAt *time.Time `json:"version_updated_at,omitempty"`
}

// PlatformCard 平台卡片（普通用户三态 / 管理员预览形态）
type PlatformCard struct {
	PlatformID              int64                     `json:"platform_id"`
	Slug                    string                    `json:"slug"`
	Name                    string                    `json:"name"`
	Description             string                    `json:"description"`
	Schemes                 []string                  `json:"schemes"`
	InstallerFiles          []InstallerFileCard       `json:"installer_files"`
	InstallerURLs           []InstallerURLCard        `json:"installer_urls"`
	Status                  string                    `json:"status"` // custom / unassigned / ready / admin_preview
	DownloadToken           string                    `json:"download_token,omitempty"`
	DownloadURL             string                    `json:"download_url,omitempty"`
	SubscriptionName        string                    `json:"subscription_name,omitempty"`
	SubscriptionProductType string                    `json:"subscription_product_type,omitempty"`
	VersionUpdatedAt        *time.Time                `json:"version_updated_at,omitempty"`
	Subscription            *AdminPreviewSubscription `json:"subscription,omitempty"`
	PreviewAvailable        bool                      `json:"preview_available"`
}

// TrafficInfo 首页流量汇总（基础模式恒不限量）
type TrafficInfo struct {
	Unlimited bool `json:"unlimited"`
}

// HomeRule 首页分流规则卡
type HomeRule struct {
	RuleID         int64  `json:"rule_id"`
	Name           string `json:"name"`
	CurrentVersion int64  `json:"current_version"`
	Token          string `json:"token"`
	DownloadURL    string `json:"download_url"`
}

// Summary 首页汇总响应
type Summary struct {
	Traffic  TrafficInfo `json:"traffic"`
	HomeRule *HomeRule   `json:"home_rule"`
}

// frontendBase 下载链接前缀：frontend_url（Setup 推导初始值/面板可覆盖，修改需重启生效，Design1 §3.4.8）；
// 为空时保持相对路径（异常场景——Setup 完成时必写，正常不触发）
// 非关键展示类配置，维持 fail-safe 并记录口径（R14-25）。
func (s *Service) frontendBase(ctx context.Context) string {
	f, _ := s.cfg.Get(ctx, config.KeyFrontendURL)
	return strings.TrimSuffix(f, "/")
}

// ListPlatforms 当前用户可见平台卡片数据，直接携带可用下载 Token，无 Token 时按需生成。
func (s *Service) ListPlatforms(ctx context.Context, userID int64, role string) ([]PlatformCard, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, slug, name, COALESCE(description,''), schemes, COALESCE(installer_files,'[]'), COALESCE(installer_urls,'[]')
		 FROM platforms ORDER BY id`)
	if err != nil {
		return nil, err
	}
	// 先将平台行全部读入内存（游标关闭释放唯一连接，避免循环内开事务死锁，MaxOpenConns=1）
	type platRow struct {
		id             int64
		slug, name     string
		desc           string
		schemesRaw     string
		installerFiles string
		installerURLs  string
	}
	var plats []platRow
	for rows.Next() {
		var p platRow
		if err := rows.Scan(&p.id, &p.slug, &p.name, &p.desc, &p.schemesRaw, &p.installerFiles, &p.installerURLs); err != nil {
			_ = rows.Close()
			return nil, err
		}
		plats = append(plats, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]PlatformCard, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for _, p := range plats {
		card := PlatformCard{
			PlatformID:     p.id,
			Slug:           p.slug,
			Name:           p.name,
			Description:    p.desc,
			Schemes:        parseSchemes(p.schemesRaw),
			InstallerFiles: installerFilesOf(p.installerFiles),
			InstallerURLs:  installerURLsOf(p.installerURLs),
		}
		sub, err := s.platformSubscription(ctx, p.id)
		if err != nil {
			return nil, err
		}
		if role == "admin" {
			// 管理员：预览形态，仅模板信息 + 「按平台预览当前版本」；不再生成显式 Token
			card.Status = "admin_preview"
			card.Subscription = sub
			card.PreviewAvailable = sub != nil && sub.CurrentVersion > 0
		} else if sub == nil || sub.CurrentVersion <= 0 {
			// 平台无订阅行 / 无激活版本：不生成 Token（下载端点返回 200 注释块）
			card.Status = "unassigned"
		} else {
			card.Status = "ready"
			card.SubscriptionName = sub.Name
			card.SubscriptionProductType = sub.ProductType
			card.VersionUpdatedAt = sub.VersionUpdatedAt
			// 无标识 Token（按平台解析）
			t, err := s.tokenSvc.GetOrCreateUserToken(ctx, userID, p.id, 0, 0)
			if err != nil {
				return nil, err
			}
			card.DownloadToken = t.Token
			card.DownloadURL = s.frontendBase(ctx) + "/subscriptions/" + p.slug + "/download?token=" + t.Token
		}
		// 普通用户自定义订阅优先（优先级最高）
		if role != "admin" {
			var customID int64
			err := s.store.DB().QueryRowContext(ctx,
				`SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, p.id).Scan(&customID)
			if err == nil {
				card.Status = "custom"
				card.SubscriptionName = "自定义订阅"
				card.SubscriptionProductType = ""
				card.VersionUpdatedAt = nil
				t, err := s.tokenSvc.GetOrCreateUserToken(ctx, userID, p.id, customID, 0)
				if err != nil {
					return nil, err
				}
				card.DownloadToken = t.Token
				card.DownloadURL = s.frontendBase(ctx) + "/subscriptions/" + p.slug + "/download?token=" + t.Token
			} else if !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
		out = append(out, card)
	}
	return out, nil
}

// platformSubscription 平台唯一订阅条目及当前激活版本信息（无订阅行返回 nil）
func (s *Service) platformSubscription(ctx context.Context, platformID int64) (*AdminPreviewSubscription, error) {
	var sub AdminPreviewSubscription
	var subID int64
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, name, product_type, COALESCE(current_version,0) FROM subscriptions WHERE platform_id = ?`,
		platformID).Scan(&subID, &sub.Name, &sub.ProductType, &sub.CurrentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if sub.CurrentVersion <= 0 {
		return &sub, nil
	}
	// 激活版本时间戳（切换动作会刷新 updated_at）
	var raw sql.NullString
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT updated_at FROM versions WHERE owner_type = 'subscription' AND owner_id = ? AND version_no = ?`,
		subID, sub.CurrentVersion).Scan(&raw); err != nil {
		return nil, err
	}
	if raw.Valid {
		ts, err := parseVersionTime(raw.String)
		if err != nil {
			return nil, err
		}
		sub.VersionUpdatedAt = &ts
	}
	// 内容形态：装配蓝图 → blueprint，否则直接上传 → upload
	var blueprint int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 WHERE v.owner_type = 'subscription' AND v.owner_id = ? AND v.version_no = ?`,
		subID, sub.CurrentVersion).Scan(&blueprint); err != nil {
		return nil, err
	}
	if blueprint > 0 {
		sub.ContentKind = "blueprint"
	} else {
		sub.ContentKind = "upload"
	}
	return &sub, nil
}

// Summary 首页独立汇总端点（Design2Report11 决策）：traffic + home_rule。
// 本 Build 阶段 traffic 恒为 {unlimited:true}（高级模式数值由 Build6 Step5 补入）。
func (s *Service) Summary(ctx context.Context) (*Summary, error) {
	resp := &Summary{Traffic: TrafficInfo{Unlimited: true}}
	var ruleID, currentVersion int64
	var name, slug, ruleToken string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT r.id, r.name, r.slug, COALESCE(r.current_version,0),
		        COALESCE((SELECT rt.token FROM rule_tokens rt WHERE rt.rule_id = r.id LIMIT 1), '')
		 FROM rules r WHERE r.is_home_default = 1 LIMIT 1`).
		Scan(&ruleID, &name, &slug, &currentVersion, &ruleToken)
	if errors.Is(err, sql.ErrNoRows) || currentVersion <= 0 || ruleToken == "" {
		return resp, nil
	}
	if err != nil {
		return nil, err
	}
	resp.HomeRule = &HomeRule{
		RuleID:         ruleID,
		Name:           name,
		CurrentVersion: currentVersion,
		Token:          ruleToken,
		DownloadURL:    s.frontendBase(ctx) + "/rules/" + slug + "/download?token=" + ruleToken,
	}
	return resp, nil
}

// RefreshToken 刷新指定平台下载 Token（旧失效）——先查该用户该平台当前有效 Token（自定义优先）再轮替
func (s *Service) RefreshToken(ctx context.Context, userID, platformID int64) (string, error) {
	// 自定义优先
	var current string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT token FROM download_tokens WHERE user_id = ? AND platform_id = ? AND custom_sub_id IS NOT NULL LIMIT 1`,
		userID, platformID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		// 无自定义 → 无标识 Token
		err = s.store.DB().QueryRowContext(ctx,
			`SELECT token FROM download_tokens WHERE user_id = ? AND platform_id = ? AND custom_sub_id IS NULL AND subscription_id IS NULL LIMIT 1`,
			userID, platformID).Scan(&current)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return "", token.ErrTokenNotFound
	}
	if err != nil {
		return "", err
	}
	t, err := s.tokenSvc.RefreshUserToken(ctx, current)
	if err != nil {
		return "", err
	}
	return t.Token, nil
}

// UpdatedAt 订阅更新时间戳：普通用户=自定义订阅 + 平台唯一订阅的最大版本更新时间；管理员=全池最大值
func (s *Service) UpdatedAt(ctx context.Context, userID int64, role string) (*time.Time, error) {
	var subIDs []int64
	if role == "admin" {
		rows, err := s.store.DB().QueryContext(ctx, `SELECT id FROM subscriptions`)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return nil, err
			}
			subIDs = append(subIDs, id)
		}
		_ = rows.Close()
		ts, err := s.maxVersionUpdatedAt(ctx, "subscription", subIDs)
		if err != nil {
			return nil, err
		}
		return ts, nil
	}

	// 普通用户可见集合：自定义订阅（owner_type=custom）+ 全部平台唯一订阅（owner_type=subscription）
	customRows, err := s.store.DB().QueryContext(ctx, `SELECT id FROM custom_subscriptions WHERE user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	var customIDs []int64
	for customRows.Next() {
		var id int64
		if err := customRows.Scan(&id); err != nil {
			_ = customRows.Close()
			return nil, err
		}
		customIDs = append(customIDs, id)
	}
	_ = customRows.Close()
	subRows, err := s.store.DB().QueryContext(ctx, `SELECT id FROM subscriptions`)
	if err != nil {
		return nil, err
	}
	for subRows.Next() {
		var id int64
		if err := subRows.Scan(&id); err != nil {
			_ = subRows.Close()
			return nil, err
		}
		subIDs = append(subIDs, id)
	}
	_ = subRows.Close()

	var latest *time.Time
	for _, pair := range []struct {
		ot  string
		ids []int64
	}{{"custom", customIDs}, {"subscription", subIDs}} {
		ts, err := s.maxVersionUpdatedAt(ctx, pair.ot, pair.ids)
		if err != nil {
			return nil, err
		}
		if ts != nil && (latest == nil || ts.After(*latest)) {
			latest = ts
		}
	}
	return latest, nil
}

// maxVersionUpdatedAt 一组 owner 的版本最大更新时间
func (s *Service) maxVersionUpdatedAt(ctx context.Context, ot string, ids []int64) (*time.Time, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT MAX(updated_at) FROM versions WHERE owner_type = ? AND owner_id IN (` +
		strings.Join(placeholders, ",") + `)`
	var raw sql.NullString
	if err := s.store.DB().QueryRowContext(ctx, query, append([]any{ot}, args...)...).Scan(&raw); err != nil {
		return nil, err
	}
	if !raw.Valid {
		return nil, nil
	}
	ts, err := parseVersionTime(raw.String)
	if err != nil {
		return nil, err
	}
	return &ts, nil
}

// parseVersionTime 兼容 SQLite 返回的两种时间形态：`2006-01-02 15:04:05` 与 RFC3339
func parseVersionTime(raw string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	return time.Parse("2006-01-02 15:04:05", raw)
}

// parseSchemes 解析平台 schemes JSON 数组
func parseSchemes(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// installerFilesOf 解析本地安装包列表并拼接公开可缓存路径（/public/installers/<file>）
func installerFilesOf(raw string) []InstallerFileCard {
	var items []struct {
		Name string `json:"name"`
		File string `json:"file"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	out := make([]InstallerFileCard, 0, len(items))
	for _, it := range items {
		if it.File == "" {
			continue
		}
		out = append(out, InstallerFileCard{Name: it.Name, URL: "/public/installers/" + it.File})
	}
	return out
}

// installerURLsOf 解析外部下载链接列表（非法 JSON 容错返回空列表）
func installerURLsOf(raw string) []InstallerURLCard {
	var items []InstallerURLCard
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	out := make([]InstallerURLCard, 0, len(items))
	for _, it := range items {
		if it.URL != "" {
			out = append(out, it)
		}
	}
	return out
}
