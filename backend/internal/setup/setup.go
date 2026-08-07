// Package setup 提供 Setup 引导业务层：标识自动生成器与快速开始事务。
package setup

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

var ErrAlreadyConfigured = errors.New("系统已完成配置")

// Service Setup 服务
type Service struct {
	store      *store.Store
	cfg        *config.Service
	log        *slog.Logger
	trustProxy string // TRUST_PROXY 策略（auto/on/off）：frontend_url 推导时判定转发头可信性
}

func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger, trustProxy string) *Service {
	return &Service{store: st, cfg: cfg, log: lg, trustProxy: trustProxy}
}

func (s *Service) IsConfigured(ctx context.Context) (bool, error) {
	return s.cfg.GetBool(ctx, config.KeyConfigured, false), nil
}

// --- 标识自动生成器（Design1 §2.2）---

// slug 短码字符集：小写字母数字，去除易混淆字符（与密码字符集规则一致）
const slugCharset = "abcdefghjkmnpqrstuvwxyz23456789"

// GenerateSlug 类型前缀 + 8 位加密安全随机短码；冲突自动重试最多 3 次，仍冲突报错并记日志
func (s *Service) GenerateSlug(ctx context.Context, tx *sql.Tx, prefix string, exists func(slug string) (bool, error)) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		code, err := randomCode(8) // crypto/rand 从 slugCharset 取 8 字符；失败返回 err
		if err != nil {
			return "", err
		}
		slug := prefix + code
		dup, err := exists(slug)
		if err != nil {
			return "", err
		}
		if !dup {
			return slug, nil
		}
	}
	s.log.Error("标识生成冲突超过重试上限", "prefix", prefix)
	return "", errors.New("标识生成失败：连续冲突，请重试")
}

func randomCode(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成随机短码失败: %w", err)
	}
	for i := range b {
		b[i] = slugCharset[int(b[i])%len(slugCharset)]
	}
	return string(b), nil
}

// --- 快速开始（关键约束：单个 BEGIN IMMEDIATE 事务，任一步失败整体回滚）---

// CompleteQuickStart 确保签名密钥 → 预置默认组 → 3 个默认平台 → configured 置位 → frontend_url 推导初始值
func (s *Service) CompleteQuickStart(ctx context.Context, r *http.Request) error {
	configured, err := s.IsConfigured(ctx)
	if err != nil {
		return err
	}
	if configured {
		return ErrAlreadyConfigured // 接入层映射 409
	}
	frontendURL := DeriveFrontendURL(r, s.trustedForwarded(r)) // 事务前推导（只读请求头）
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		// 1) 确保签名密钥存在（复用 auth 阶段的 EnsureSigningKey 逻辑，不重复生成）
		if _, err := s.cfg.EnsureSigningKeyTx(ctx, tx); err != nil {
			return err
		}
		// 2) 预置默认组 + 3 个默认平台（抽取复用，OIDC Setup 分支共用）
		if err := s.seedPresets(ctx, tx, frontendURL); err != nil {
			return err
		}
		// 3) configured 置位 + frontend_url 初始值（手动覆盖优先的缓存语义在 Build3 面板实现）
		if err := s.cfg.SetTx(ctx, tx, config.KeyConfigured, "true"); err != nil {
			return err
		}
		if err := s.cfg.SetTx(ctx, tx, config.KeyFrontendURL, frontendURL); err != nil {
			return err
		}
		return nil
	})
}

// seedPresets 预置默认组（is_default=1，不可删除）与 3 个默认平台（Design1 §2.2/3.4.4）；事务内调用
func (s *Service) seedPresets(ctx context.Context, tx *sql.Tx, frontendURL string) error {
	groupSlug, err := s.GenerateSlug(ctx, tx, "group-", func(slug string) (bool, error) {
		return tableHasSlug(tx, "groups", slug)
	})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO groups (slug, name, is_default) VALUES (?, '默认组', 1)`, groupSlug); err != nil {
		return fmt.Errorf("创建预置默认组失败: %w", err)
	}
	for _, p := range defaultPlatforms(frontendURL) {
		slug, err := s.GenerateSlug(ctx, tx, "platform-", func(slug string) (bool, error) {
			return tableHasSlug(tx, "platforms", slug)
		})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO platforms (slug, name, description, schemes, extra_headers) VALUES (?,?,?,?,?)`,
			slug, p.Name, p.Description, p.Schemes, p.ExtraHeaders); err != nil {
			return fmt.Errorf("创建默认平台 %s 失败: %w", p.Name, err)
		}
	}
	return nil
}

// CompleteOidcSetup 高级配置分支（Step 6）：与快速开始同一事务语义：
// 保存 OIDC 参数（Secret 加密）→ 预置默认组/平台 → configured 置位
func (s *Service) CompleteOidcSetup(ctx context.Context, r *http.Request, providerType string, saveParams func(tx *sql.Tx) error) error {
	configured, err := s.IsConfigured(ctx)
	if err != nil {
		return err
	}
	if configured {
		return ErrAlreadyConfigured
	}
	// 事务前：推导 frontend_url 与 callback_url 初始值（frontend_url + "/api/auth/oidc/callback"）
	frontendURL := DeriveFrontendURL(r, s.trustedForwarded(r))
	callbackURL := frontendURL + "/api/auth/oidc/callback"
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := s.cfg.EnsureSigningKeyTx(ctx, tx); err != nil {
			return err // 复用不重复生成
		}
		if err := saveParams(tx); err != nil {
			return err // OIDC 参数写入（Secret 加密）
		}
		if err := s.cfg.SetTx(ctx, tx, "oidc_"+config.KeyConfigured, "true"); err != nil {
			return err
		}
		if err := s.seedPresets(ctx, tx, frontendURL); err != nil {
			return err // 预置默认组/平台
		}
		if err := s.cfg.SetTx(ctx, tx, config.KeyConfigured, "true"); err != nil {
			return err
		}
		if err := s.cfg.SetTx(ctx, tx, config.KeyFrontendURL, frontendURL); err != nil {
			return err
		}
		if err := s.cfg.SetTx(ctx, tx, config.KeyCallbackURL, callbackURL); err != nil {
			return err
		}
		return nil
	})
}

// defaultPlatforms 预置平台的 scheme 与附加头（Design1 §3.4.4/4.3）；
// v2rayNG 与 Shadowrocket 取各自客户端常用导入 scheme
func defaultPlatforms(frontendURL string) []struct{ Name, Description, Schemes, ExtraHeaders string } {
	return []struct{ Name, Description, Schemes, ExtraHeaders string }{
		{"Clash Verge", "桌面端 Clash 内核客户端",
			`["clash://install-config?url={url}"]`,
			// 三条兼容附加头；Content-Disposition 文件名在下载时按订阅名动态生成，此处存模板
			`{"Content-Disposition":"attachment; filename*=UTF-8''subscription.yaml","profile-update-interval":"300","profile-web-page-url":"{frontend_url}"}`},
		{"v2rayNG", "Android 端 V2Ray 客户端",
			`["v2rayng://install-config?url={url}"]`, `{}`},
		{"Shadowrocket", "iOS 端代理客户端",
			`["shadowrocket://add/{url}"]`, `{}`},
	}
}

// tableHasSlug 检查表内是否已存在该 slug（供标识生成器冲突检测）。
// 表名仅允许白名单内的固定值（防动态 SQL 注入）
func tableHasSlug(tx *sql.Tx, table, slug string) (bool, error) {
	switch table {
	case "groups", "platforms":
	default:
		return false, fmt.Errorf("非法表名: %s", table)
	}
	var n int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE slug = ?`, slug).Scan(&n); err != nil {
		return false, fmt.Errorf("查询 %s 标识失败: %w", table, err)
	}
	return n > 0, nil
}

// --- 前端地址推导（Design1 §3.1/6.4）---
// DeriveFrontendURL TRUST_PROXY 信任来源时优先取 X-Forwarded-Host，否则取 Host 头；
// scheme 按 X-Forwarded-Proto / TLS 状态推导
func DeriveFrontendURL(r *http.Request, trusted bool) string {
	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" && trusted {
		host = strings.TrimSpace(strings.Split(xfh, ",")[0])
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + host
}

// trustedForwarded 按 TRUST_PROXY 策略判定远端是否可信（Design1 §6.4）：
// on=始终信任转发头；off=从不信任；auto=仅回环+私有网段来源信任（与 gin SetTrustedProxies 的 auto 档口径一致）
func (s *Service) trustedForwarded(r *http.Request) bool {
	switch s.trustProxy {
	case "on":
		return true
	case "off":
		return false
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		return true
	}
	return false
}
