// Package server 用户端数据端点（接入层）：平台卡片（携带下载 Token）、Token 刷新、更新时间戳。
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
	"vpn-sub/internal/download"
	"vpn-sub/internal/store"
	"vpn-sub/internal/token"
)

// HomeHandler 用户端数据处理器（结构体 Handler + 依赖注入）
type HomeHandler struct {
	store    *store.Store
	tokenSvc *token.Service
	dlSvc    *download.Service
	cfg      *config.Service // frontend_url 前缀拼接（R10-10）
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
	g.POST("/token/refresh", h.refreshToken)
	g.GET("/updated_at", h.updatedAt)
}

// platformCard 平台卡片（普通用户三态 / 管理员池）
type platformCard struct {
	PlatformID       int64             `json:"platform_id"`
	Name             string            `json:"name"`
	Description      string            `json:"description"`
	Schemes          []string          `json:"schemes"`
	InstallerFileURL string            `json:"installer_file_url"`
	InstallerURL     string            `json:"installer_url"`
	Status           string            `json:"status"` // group_selected/custom/unassigned/admin_pool
	DownloadToken    string            `json:"download_token"`
	DownloadURL      string            `json:"download_url"`
	SubscriptionName string            `json:"subscription_name,omitempty"`
	Subscriptions    []adminPoolSub    `json:"subscriptions,omitempty"` // 管理员池内订阅列表
}

type adminPoolSub struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Slug           string `json:"slug"`
	CurrentVersion int64  `json:"current_version"`
	Token          string `json:"token"`
	DownloadURL    string `json:"download_url"`
}

// platforms 当前用户可见平台卡片数据，直接携带可用下载 Token，无 Token 时按需生成（Design1 §5.2）
func (h *HomeHandler) platforms(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	role := c.GetString(auth.CtxUserRole)
	rows, err := h.store.DB().QueryContext(ctx,
		`SELECT id, slug, name, COALESCE(description,''), schemes, COALESCE(installer_file,''), COALESCE(installer_url,'')
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
		installerFile  string
		installerURL   string
	}
	var plats []platRow
	for rows.Next() {
		var p platRow
		if err := rows.Scan(&p.id, &p.slug, &p.name, &p.desc, &p.schemesRaw, &p.installerFile, &p.installerURL); err != nil {
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
			PlatformID:       p.id,
			Name:             p.name,
			Description:      p.desc,
			Schemes:          parseSchemes(p.schemesRaw),
			InstallerFileURL: installerURLOf(p.installerFile),
			InstallerURL:     p.installerURL,
		}
		if role == "admin" {
			// 管理员：池内全部订阅（预览用显式 Token）
			card.Status = "admin_pool"
			subs, err := h.adminPool(ctx, userID, p.id, p.slug)
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			card.Subscriptions = subs
		} else {
			// 普通用户：自定义 → 组选定 → 未分配
			var customID int64
			hasCustom := false
			err := h.store.DB().QueryRowContext(ctx,
				`SELECT id FROM custom_subscriptions WHERE user_id = ? AND platform_id = ?`, userID, p.id).Scan(&customID)
			if err == nil {
				hasCustom = true
			} else if !errors.Is(err, sql.ErrNoRows) {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			selected, subName, err := h.groupSelected(ctx, userID, p.id)
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			var t *token.UserToken
			if hasCustom {
				card.Status = "custom"
				card.SubscriptionName = "自定义订阅"
				t, err = h.tokenSvc.GetOrCreateUserToken(ctx, userID, p.id, customID, 0)
			} else if selected {
				card.Status = "group_selected"
				card.SubscriptionName = subName
				t, err = h.tokenSvc.GetOrCreateUserToken(ctx, userID, p.id, 0, 0)
			} else {
				card.Status = "unassigned" // 仍返无标识 Token（下载时返回 unassigned 注释块）
				t, err = h.tokenSvc.GetOrCreateUserToken(ctx, userID, p.id, 0, 0)
			}
			if err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			card.DownloadToken = t.Token
			card.DownloadURL = h.frontendBase(ctx) + "/subscriptions/" + p.slug + "/download?token=" + t.Token // R10-10：完整 URL（含 frontend_url 前缀）
		}
		out = append(out, card)
	}
	OK(c, ListData{List: out, Total: int64(len(out))}) // 列表统一包裹结构（AGENTS §4.8）
}

// adminPool 管理员池内订阅（每份生成/复用显式 Token）；游标先读完再逐个生成 Token（防单连接死锁）
func (h *HomeHandler) adminPool(ctx context.Context, userID, platformID int64, platformSlug string) ([]adminPoolSub, error) {
	rows, err := h.store.DB().QueryContext(ctx,
		`SELECT id, name, slug, COALESCE(current_version,0) FROM subscriptions WHERE platform_id = ? ORDER BY id`, platformID)
	if err != nil {
		return nil, err
	}
	var subs []adminPoolSub
	for rows.Next() {
		var sub adminPoolSub
		if err := rows.Scan(&sub.ID, &sub.Name, &sub.Slug, &sub.CurrentVersion); err != nil {
			_ = rows.Close()
			return nil, err
		}
		subs = append(subs, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range subs {
		t, err := h.tokenSvc.GetOrCreateUserToken(ctx, userID, platformID, 0, subs[i].ID) // 显式 Token（复用键 user+platform+subscription）
		if err != nil {
			return nil, err
		}
		subs[i].Token = t.Token
		subs[i].DownloadURL = h.frontendBase(ctx) + "/subscriptions/" + platformSlug + "/download?token=" + t.Token // R10-10：完整 URL（含 frontend_url 前缀）
	}
	return subs, nil
}

// groupSelected 查询组在该平台是否有选定订阅，返回是否选定与订阅名
func (h *HomeHandler) groupSelected(ctx context.Context, userID, platformID int64) (bool, string, error) {
	var name string
	err := h.store.DB().QueryRowContext(ctx,
		`SELECT s.name FROM users u
		 JOIN group_selections gs ON gs.group_id = u.group_id AND gs.platform_id = ?
		 JOIN subscriptions s ON s.id = gs.subscription_id
		 WHERE u.id = ? AND u.group_id IS NOT NULL AND gs.subscription_id IS NOT NULL`,
		platformID, userID).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	return true, name, nil
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

// updatedAt 订阅更新时间戳：普通用户=可见订阅最大值；管理员=全池最大值；无可见订阅返回空
func (h *HomeHandler) updatedAt(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.GetInt64(auth.CtxUserID)
	role := c.GetString(auth.CtxUserRole)
	var ids []int64
	if role == "admin" {
		rows, err := h.store.DB().QueryContext(ctx, `SELECT id FROM subscriptions`)
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				Fail(c, http.StatusInternalServerError, err.Error())
				return
			}
			ids = append(ids, id)
		}
	} else {
		// 可见集合：自定义订阅 + 组选定订阅
		rows, err := h.store.DB().QueryContext(ctx,
			`SELECT id FROM custom_subscriptions WHERE user_id = ?`, userID)
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
			ids = append(ids, id)
		}
		_ = rows.Close()
		rows, err = h.store.DB().QueryContext(ctx,
			`SELECT gs.subscription_id FROM users u
			 JOIN group_selections gs ON gs.group_id = u.group_id
			 WHERE u.id = ? AND gs.subscription_id IS NOT NULL`, userID)
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
			ids = append(ids, id)
		}
		_ = rows.Close()
	}
	if len(ids) == 0 {
		OK(c, gin.H{"updated_at": nil})
		return
	}
	// MAX(updated_at) 取可见订阅的版本时间戳
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := `SELECT MAX(updated_at) FROM versions WHERE owner_type = 'subscription' AND owner_id IN (` +
		strings.Join(placeholders, ",") + `)`
	var ts sql.NullString
	if err := h.store.DB().QueryRowContext(ctx, query, args...).Scan(&ts); err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	if !ts.Valid {
		OK(c, gin.H{"updated_at": nil})
		return
	}
	// SQLite CURRENT_TIMESTAMP 为 UTC 无时区字符串 → time.Time 输出 RFC3339（前端按本地时区展示，R07-04）
	t, err := time.Parse("2006-01-02 15:04:05", ts.String)
	if err != nil {
		Fail(c, http.StatusInternalServerError, err.Error())
		return
	}
	OK(c, gin.H{"updated_at": t})
}

// parseSchemes 解析平台 schemes JSON 数组
func parseSchemes(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// installerURLOf 安装包公开路径（/public 可缓存路径）
func installerURLOf(file string) string {
	if file == "" {
		return ""
	}
	return "/public/installers/" + file
}
