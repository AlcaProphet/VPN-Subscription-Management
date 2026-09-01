// Package captcha 提供验证码服务：reCAPTCHA/Turnstile 配置读写与服务端校验。
package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/config"
	"vpn-sub/internal/response"
)

const (
	KeyProvider  = "captcha_provider" // recaptcha/turnstile/off，默认 off
	KeySiteKey   = "captcha_site_key"
	KeySecretKey = "captcha_secret_key" // 明文存储（面板回显真实值，切换提供商/停用后可复用）
	KeyPages     = "captcha_pages"      // JSON 数组：register/login/forgot
)

// Service 验证码服务
type Service struct {
	cfg     *config.Service
	log     *slog.Logger
	httpCli *http.Client
}

func NewService(cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{cfg: cfg, log: lg, httpCli: &http.Client{Timeout: 10 * time.Second}}
}

// Enforced 某页面是否强制校验（页面在 captcha_pages 且密钥已配置）
func (s *Service) Enforced(ctx context.Context, page string) bool {
	provider, _ := s.cfg.Get(ctx, KeyProvider)
	if provider == "off" || provider == "" {
		return false
	}
	pages := s.cfg.GetJSONStringSlice(ctx, KeyPages) // 解析 JSON 数组
	if !slices.Contains(pages, page) {
		return false
	}
	secret, _ := s.cfg.Get(ctx, KeySecretKey)
	if secret == "" {
		// 运行中密钥配置缺失 → 跳过校验兜底并记 warn（Design1 §3.2）
		s.log.Warn("验证码密钥未配置，跳过校验", "page", page, "provider", provider)
		return false
	}
	return true
}

// Verify 调用提供商验证接口；入参 secret + response（前端 token），解析 success 字段；
// 网络/解析失败返回 error → 接入层 400
func (s *Service) Verify(ctx context.Context, page, captchaToken string) error {
	if !s.Enforced(ctx, page) {
		return nil
	}
	if captchaToken == "" {
		return errors.New("请完成验证码校验")
	}
	provider, _ := s.cfg.Get(ctx, KeyProvider)
	secret, _ := s.cfg.Get(ctx, KeySecretKey)
	var verifyURL string
	switch provider {
	case "recaptcha":
		verifyURL = "https://www.google.com/recaptcha/api/siteverify"
	case "turnstile":
		verifyURL = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	default:
		return errors.New("验证码提供商配置无效")
	}
	form := url.Values{"secret": {secret}, "response": {captchaToken}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, verifyURL,
		stringsNewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("构造验证请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := s.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("验证码服务不可达: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("读取验证响应失败: %w", err)
	}
	var result struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return errors.New("验证码服务响应异常")
	}
	if !result.Success {
		return errors.New("验证码校验失败")
	}
	return nil
}

// Middleware 接入层包装，按页面名强制校验（captchaToken 从请求体 captcha_token 字段取）；
// 用 ShouldBindBodyWithJSON 读取：body 缓存进 context，后续处理器仍可正常绑定（gin 多次绑定唯一安全姿势）
func (s *Service) Middleware(page string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.Enforced(c.Request.Context(), page) {
			c.Next()
			return
		}
		var body struct {
			CaptchaToken string `json:"captcha_token"`
		}
		_ = c.ShouldBindBodyWithJSON(&body) // 校验失败由 Verify 统一处理
		if err := s.Verify(c.Request.Context(), page, body.CaptchaToken); err != nil {
			response.Fail(c, http.StatusBadRequest, err.Error())
			c.Abort()
			return
		}
		c.Next()
	}
}

// stringsNewReader 字符串读取器（与 oidc 包同构，避免引入额外依赖）
func stringsNewReader(s string) *strings.Reader { return strings.NewReader(s) }
