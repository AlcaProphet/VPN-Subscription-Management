// Package server 用户端数据端点（接入层）：平台卡片、Token 刷新、更新时间戳。
package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
)

// HomeHandler 用户端数据处理器（结构体 Handler + 依赖注入）
type HomeHandler struct {
	store    *store.Store
	tokenSvc *token.Service
	cfg      *config.Service // frontend_url 前缀拼接
}

// frontendBase 下载链接前缀：frontend_url（Setup 推导初始值/面板可覆盖，修改需重启生效，Design1 §3.4.8）；
// 为空时保持相对路径（异常场景——Setup 完成时必写，正常不触发）
func (h *HomeHandler) frontendBase(ctx context.Context) string {
	f, _ := h.cfg.Get(ctx, config.KeyFrontendURL)
	return strings.TrimSuffix(f, "/")
}

// RegisterHomeRoutes 注册用户端数据端点；全部需会话
func RegisterHomeRoutes(engine *gin.Engine, h *HomeHandler, sessionMW gin.HandlerFunc) {
	g := engine.Group("/api/home", sessionMW)
	g.GET("/platforms", h.platforms)
	g.GET("/summary", h.summary)
	g.POST("/token/refresh", h.refreshToken)
	g.GET("/updated_at", h.updatedAt)
}

// installerFileCard 首页本地安装包条目（已拼接公开可缓存路径）
type installerFileCard struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// installerURLCard 首页外部下载链接条目
type installerURLCard struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// adminPreviewSubscription 管理员平台卡片的订阅条目预览信息（每平台唯一）
type adminPreviewSubscription struct {
	Name             string     `json:"name"`
	ProductType      string     `json:"product_type"`
	ContentKind      string     `json:"content_kind"` // blueprint / upload；无激活版本为空
	CurrentVersion   int64      `json:"current_version"`
	VersionUpdatedAt *time.Time `json:"version_updated_at,omitempty"`
}

// platformCard 平台卡片（普通用户三态 / 管理员预览形态）
type platformCard struct {
	PlatformID              int64                     `json:"platform_id"`
	Slug                    string                    `json:"slug"`
	Name                    string                    `json:"name"`
	Description             string                    `json:"description"`
	Schemes                 []string                  `json:"schemes"`
	InstallerFiles          []installerFileCard       `json:"installer_files"`
	InstallerURLs           []installerURLCard        `json:"installer_urls"`
	Status                  string                    `json:"status"` // custom / unassigned / ready / admin_preview
	DownloadToken           string                    `json:"download_token,omitempty"`
	DownloadURL             string                    `json:"download_url,omitempty"`
	SubscriptionName        string                    `json:"subscription_name,omitempty"`
	SubscriptionProductType string                    `json:"subscription_product_type,omitempty"`
	VersionUpdatedAt        *time.Time                `json:"version_updated_at,omitempty"`
	Subscription            *adminPreviewSubscription `json:"subscription,omitempty"`
	PreviewAvailable        bool                      `json:"preview_available"`
}

// platforms 当前用户可见平台卡片数据，直接携带可用下载 Token，无 Token 时按需生成
func (h *HomeHandler) platforms(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	role := c.GetString(auth.CtxUserRole)
	rows, err := h.store.DB().QueryContext(ctx,
		`SELECT id, slug, name, COALESCE(description,''), schemes, COALESCE(installer_files,'[]'), COALESCE(installer_urls,'[]')
		 FROM platforms ORDER BY id`)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
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
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		plats = append(plats, p)
	}
	if err := rows.Err(); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	// 逐平台处理（循环内可安全开事务：游标已关闭）
	out := make([]platformCard, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for _, p := range plats {
		card := platformCard{
			PlatformID:     p.id,
			Slug:           p.slug,
			Name:           p.name,
			Description:    p.desc,
			Schemes:        parseSchemes(p.schemesRaw),
			InstallerFiles: installerFilesOf(p.installerFiles),
			InstallerURLs:  installerURLsOf(p.installerURLs),
		}
		sub, err := h.platformSubscription(ctx, p.id)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
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
			t, err := h.tokenSvc.GetOrCreateUserToken(ctx, userID, p.id, 0, 0)
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			card.DownloadToken = t.Token
			card.DownloadURL = h.frontendBase(ctx) + "/subscriptions/" + p.slug + "/download?token=" + t.Token
		}
		// 普通用户自定义订阅优先（优先级最高）
		if role != "admin" {
			var customID int64
			err := h.store.DB().QueryRowContext(ctx,
				`SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, p.id).Scan(&customID)
			if err == nil {
				card.Status = "custom"
				card.SubscriptionName = "自定义订阅"
				card.SubscriptionProductType = ""
				card.VersionUpdatedAt = nil
				t, err := h.tokenSvc.GetOrCreateUserToken(ctx, userID, p.id, customID, 0)
				if err != nil {
					Fail(c, http.StatusInternalServerError, err.Error())
					return
				}
				card.DownloadToken = t.Token
				card.DownloadURL = h.frontendBase(ctx) + "/subscriptions/" + p.slug + "/download?token=" + t.Token
			} else if !errors.Is(err, sql.ErrNoRows) {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
		}
		out = append(out, card)
	}
	OK(c, ListData{List: out, Total: int64(len(out))}) // 列表统一包裹结构（AGENTS §4.8）
}

// platformSubscription 平台唯一订阅条目及当前激活版本信息（无订阅行返回 nil）
func (h *HomeHandler) platformSubscription(ctx context.Context, platformID int64) (*adminPreviewSubscription, error) {
	var sub adminPreviewSubscription
	var subID int64
	err := h.store.DB().QueryRowContext(ctx,
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
	if err := h.store.DB().QueryRowContext(ctx,
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
	if err := h.store.DB().QueryRowContext(ctx,
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

// summary 首页独立汇总端点（Design2Report11 决策）：traffic + home_rule。
// 本 Build 阶段 traffic 恒为 {unlimited:true}（高级模式数值由 Build6 Step5 补入）。
func (h *HomeHandler) summary(c *gin.Context) {
	ctx := c.Request.Context()
	resp := gin.H{
		"traffic":   gin.H{"unlimited": true},
		"home_rule": nil,
	}
	var ruleID, currentVersion int64
	var name, slug, ruleToken string
	err := h.store.DB().QueryRowContext(ctx,
		`SELECT r.id, r.name, r.slug, COALESCE(r.current_version,0),
		        COALESCE((SELECT rt.token FROM rule_tokens rt WHERE rt.rule_id = r.id LIMIT 1), '')
		 FROM rules r WHERE r.is_home_default = 1 LIMIT 1`).
		Scan(&ruleID, &name, &slug, &currentVersion, &ruleToken)
	if errors.Is(err, sql.ErrNoRows) || currentVersion <= 0 || ruleToken == "" {
		OK(c, resp)
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	resp["home_rule"] = gin.H{
		"rule_id":         ruleID,
		"name":            name,
		"current_version": currentVersion,
		"token":           ruleToken,
		"download_url":    h.frontendBase(ctx) + "/rules/" + slug + "/download?token=" + ruleToken,
	}
	OK(c, resp)
}

// refreshToken 刷新指定平台下载 Token（旧失效）——先查该用户该平台当前有效 Token（自定义优先）再轮替
func (h *HomeHandler) refreshToken(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	var req struct {
		PlatformID int64 `json:"platform_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, "参数校验失败")
		return
	}
	// 自定义优先
	var current string
	err := h.store.DB().QueryRowContext(ctx,
		`SELECT token FROM download_tokens WHERE user_id = ? AND platform_id = ? AND custom_sub_id IS NOT NULL LIMIT 1`,
		userID, req.PlatformID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		// 无自定义 → 无标识 Token
		err = h.store.DB().QueryRowContext(ctx,
			`SELECT token FROM download_tokens WHERE user_id = ? AND platform_id = ? AND custom_sub_id IS NULL AND subscription_id IS NULL LIMIT 1`,
			userID, req.PlatformID).Scan(&current)
	}
	if errors.Is(err, sql.ErrNoRows) {
		Fail(c, http.StatusNotFound, "该平台无可用 Token")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	t, err := h.tokenSvc.RefreshUserToken(ctx, current)
	if errors.Is(err, token.ErrTokenNotFound) {
		Fail(c, http.StatusNotFound, "该平台无可用 Token")
		return
	}
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"token": t.Token})
}

// updatedAt 订阅更新时间戳：普通用户=自定义订阅 + 平台唯一订阅的最大版本更新时间；管理员=全池最大值
func (h *HomeHandler) updatedAt(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	role := c.GetString(auth.CtxUserRole)

	var subIDs []int64
	if role == "admin" {
		rows, err := h.store.DB().QueryContext(ctx, `SELECT id FROM subscriptions`)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			subIDs = append(subIDs, id)
		}
		_ = rows.Close()
		ts, err := h.maxVersionUpdatedAt(ctx, "subscription", subIDs)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		OK(c, gin.H{"updated_at": ts})
		return
	}

	// 普通用户可见集合：自定义订阅（owner_type=custom）+ 全部平台唯一订阅（owner_type=subscription）
	customRows, err := h.store.DB().QueryContext(ctx, `SELECT id FROM custom_subscriptions WHERE user_id = ?`, userID)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	var customIDs []int64
	for customRows.Next() {
		var id int64
		if err := customRows.Scan(&id); err != nil {
			_ = customRows.Close()
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		customIDs = append(customIDs, id)
	}
	_ = customRows.Close()
	subRows, err := h.store.DB().QueryContext(ctx, `SELECT id FROM subscriptions`)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	for subRows.Next() {
		var id int64
		if err := subRows.Scan(&id); err != nil {
			_ = subRows.Close()
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		subIDs = append(subIDs, id)
	}
	_ = subRows.Close()

	var latest *time.Time
	for _, pair := range []struct {
		ot  string
		ids []int64
	}{{"custom", customIDs}, {"subscription", subIDs}} {
		ts, err := h.maxVersionUpdatedAt(ctx, pair.ot, pair.ids)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if ts != nil && (latest == nil || ts.After(*latest)) {
			latest = ts
		}
	}
	OK(c, gin.H{"updated_at": latest})
}

// maxVersionUpdatedAt 一组 owner 的版本最大更新时间
func (h *HomeHandler) maxVersionUpdatedAt(ctx context.Context, ot string, ids []int64) (*time.Time, error) {
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
	if err := h.store.DB().QueryRowContext(ctx, query, append([]any{ot}, args...)...).Scan(&raw); err != nil {
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
func installerFilesOf(raw string) []installerFileCard {
	var items []struct {
		Name string `json:"name"`
		File string `json:"file"`
	}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	out := make([]installerFileCard, 0, len(items))
	for _, it := range items {
		if it.File == "" {
			continue
		}
		out = append(out, installerFileCard{Name: it.Name, URL: "/public/installers/" + it.File})
	}
	return out
}

// installerURLsOf 解析外部下载链接列表（非法 JSON 容错返回空列表）
func installerURLsOf(raw string) []installerURLCard {
	var items []installerURLCard
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil
	}
	out := make([]installerURLCard, 0, len(items))
	for _, it := range items {
		if it.URL != "" {
			out = append(out, it)
		}
	}
	return out
}
