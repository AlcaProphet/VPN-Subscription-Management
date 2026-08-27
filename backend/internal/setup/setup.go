// Package setup 提供 Setup 引导业务层：标识自动生成器与快速开始事务。
package setup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"vpn-sub/internal/config"
	"vpn-sub/internal/proxytrust"
	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
)

var ErrAlreadyConfigured = errors.New("系统已完成配置")

// Service Setup 服务
type Service struct {
	store      *store.Store
	cfg        *config.Service
	log        *slog.Logger
	trustProxy *proxytrust.Policy // TRUST_PROXY 策略：frontend_url 推导时判定转发头可信性
}

func NewService(st *store.Store, cfg *config.Service, lg *slog.Logger, trustProxy *proxytrust.Policy) *Service {
	return &Service{store: st, cfg: cfg, log: lg, trustProxy: trustProxy}
}

func (s *Service) IsConfigured(ctx context.Context) (bool, error) {
	// R14-25：Setup/导入/鉴权前置依赖 configured，DB 错误不能静默为“未配置”。
	return s.cfg.GetBoolStrict(ctx, config.KeyConfigured, false)
}

// --- 标识生成器（Build2 抽取为共享包 internal/slug，Setup 复用 slug.Generate）---

// 快速开始（关键约束：单个 BEGIN IMMEDIATE 事务，任一步失败整体回滚）---

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
		if err := s.SeedPresetsTx(ctx, tx, frontendURL); err != nil {
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

// SeedPresetsTx 预置默认组（is_default=1，不可删除）与 3 个默认平台（Design1 §2.2/3.4.4）；事务内调用。
// 导出供配置导入（Build3 Step 4 Setup 分支）与 Setup/OIDC Setup 共用
func (s *Service) SeedPresetsTx(ctx context.Context, tx *sql.Tx, frontendURL string) error {
	return s.seedPresets(ctx, tx, frontendURL)
}

// seedPresets 预置默认组与 3 个默认平台（事务内调用）
func (s *Service) seedPresets(ctx context.Context, tx *sql.Tx, frontendURL string) error {
	groupSlug, err := slug.Generate(ctx, tx, "group-", func(value string) (bool, error) {
		return slug.TableHasSlug(ctx, tx, "groups", value)
	})
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO groups (slug, name, is_default) VALUES (?, '默认组', 1)`, groupSlug); err != nil {
		return fmt.Errorf("创建预置默认组失败: %w", err)
	}
	for _, p := range defaultPlatforms(frontendURL) {
		value, err := slug.Generate(ctx, tx, "platform-", func(v string) (bool, error) {
			return slug.TableHasSlug(ctx, tx, "platforms", v)
		})
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO platforms (slug, name, description, product_type, schemes, extra_headers, is_default) VALUES (?,?,?,?,?,?,1)`,
			value, p.Name, p.Description, p.ProductType, p.Schemes, p.ExtraHeaders); err != nil {
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
		if err := s.SeedPresetsTx(ctx, tx, frontendURL); err != nil {
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

// defaultPlatforms 预置平台的 scheme / 附加头 / product_type（Design2 §4.4/§5.9）：
// Clash Verge→yaml、v2rayNG→generic-subs、Shadowrocket→subs
func defaultPlatforms(frontendURL string) []struct{ Name, Description, Schemes, ExtraHeaders, ProductType string } {
	return []struct{ Name, Description, Schemes, ExtraHeaders, ProductType string }{
		{"Clash Verge", "桌面端 Clash 内核客户端",
			`["clash://install-config?url={url}"]`,
			// 三条兼容附加头；Content-Disposition 文件名在下载时按订阅名动态生成，此处存模板
			// profile-update-interval 生态单位为小时（6 = 每 6 小时自动更新；Design2 决策 #23 勘误）
			`{"Content-Disposition":"attachment; filename*=UTF-8''subscription.yaml","profile-update-interval":"6","profile-web-page-url":"{frontend_url}"}`,
			"yaml"},
		{"v2rayNG", "Android 端 V2Ray 客户端",
			`["v2rayng://install-config?url={url}"]`, `{}`, "generic-subs"},
		{"Shadowrocket", "iOS 端代理客户端",
			`["shadowrocket://add/{url}"]`, `{}`, "subs"},
	}
}

// --- 前端地址推导（Design1 §3.1/6.4）---
// DeriveFrontendURL TRUST_PROXY 信任来源时优先取 X-Forwarded-Host，否则取 Host 头；
// scheme 仅在 TLS 直连或可信代理声明 X-Forwarded-Proto=https 时使用 https。
func DeriveFrontendURL(r *http.Request, trusted bool) string {
	host := r.Host
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" && trusted {
		host = strings.TrimSpace(strings.Split(xfh, ",")[0])
	}
	scheme := "http"
	if r.TLS != nil || (trusted && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")) {
		scheme = "https"
	}
	return scheme + "://" + host
}

// trustedForwarded 按 TRUST_PROXY 策略判定远端是否可信（与 gin SetTrustedProxies 同口径）。
func (s *Service) trustedForwarded(r *http.Request) bool {
	return s.trustProxy.Trusted(r.RemoteAddr)
}
