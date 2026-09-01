// Package auth 提供认证业务层：密码、邮箱规范化、会话凭据签发/验证与中间件。
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"vpn-sub/internal/config"
	"vpn-sub/internal/response"
)

// 会话时长与密码规则常量（关键设计参数，禁止修改）
const (
	SessionRemember   = 7 * 24 * time.Hour // 记住我：7 天
	SessionNoRemember = 24 * time.Hour     // 不勾选：24 小时
	OidcSession       = 7 * 24 * time.Hour // OIDC 固定 7 天，无记住我
	MinPasswordLen    = 8                  // 所有本地密码入口统一
)

// HashPassword bcrypt 哈希
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("密码哈希失败: %w", err)
	}
	return string(b), nil
}

// CheckPassword bcrypt 校验
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// ValidatePassword 复杂度校验（≥8 字符，所有本地密码入口统一，Design1 §4.6）
func ValidatePassword(p string) error {
	if utf8.RuneCountInString(p) < MinPasswordLen {
		return errors.New("密码长度至少 8 个字符")
	}
	return nil
}

// NormalizeEmail 所有写入入口统一 trim + 小写化；拒绝控制字符（防 SMTP 头注入）
func NormalizeEmail(raw string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(raw))
	if e == "" || len(e) > 254 || !strings.Contains(e, "@") {
		return "", errors.New("邮箱格式无效")
	}
	for _, r := range e {
		if r < 0x20 || r == 0x7f {
			return "", errors.New("邮箱含非法控制字符")
		}
	}
	return e, nil
}

// Claims 会话凭据载荷仅含 user_id + credential_version + 标准声明；
// 角色/组等权限信息禁止入凭据，每次请求实时查库（Design1 §3.2/5.4）
type Claims struct {
	jwt.RegisteredClaims
	UserID            int64 `json:"uid"`
	CredentialVersion int   `json:"cv"`
}

// UserSnapshot 凭据校验所需的用户最小信息（由 user 包实现 UserSource 接口注入，避免循环依赖）
type UserSnapshot struct {
	ID                int64
	Role              string
	Status            string
	CredentialVersion int
}

// UserSource 用户快照来源接口（user 包实现）
type UserSource interface {
	SnapshotByID(ctx context.Context, id int64) (*UserSnapshot, error)
}

// Service 认证服务
type Service struct {
	cfg   *config.Service
	users UserSource
	log   *slog.Logger
}

func NewService(cfg *config.Service, users UserSource, lg *slog.Logger) *Service {
	return &Service{cfg: cfg, users: users, log: lg}
}

// Issue 用 signing_key 以 HS256 签名；签发前确保签名密钥存在（Setup 前兜底，不重复生成）
func (s *Service) Issue(ctx context.Context, userID int64, credVersion int, dur time.Duration) (string, time.Time, error) {
	key, err := s.cfg.EnsureSigningKey(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	now := time.Now()
	exp := now.Add(dur)
	claims := Claims{
		RegisteredClaims:  jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(exp)},
		UserID:            userID,
		CredentialVersion: credVersion,
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(key)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("签发会话凭据失败: %w", err)
	}
	return token, exp, nil
}

// Parse 解析并验签会话凭据
func (s *Service) Parse(ctx context.Context, tokenStr string) (*Claims, error) {
	key, err := s.cfg.GetSigningKey(ctx) // 未生成时验签必然失败 → 401
	if err != nil {
		return nil, err
	}
	var claims Claims
	_, err = jwt.ParseWithClaims(tokenStr, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("签名算法不匹配")
		}
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("凭据解析失败: %w", err)
	}
	return &claims, nil
}

// --- 中间件 ---

const (
	CtxUserID   = "auth_user_id"
	CtxUserRole = "auth_user_role"
)

// SessionMiddleware 会话校验层：解析凭据 → 实时查库取用户 → 比对 credential_version → 校验 status=active
func (s *Service) SessionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		tokenStr := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if tokenStr == "" || tokenStr == header {
			response.Fail(c, http.StatusUnauthorized, "会话凭据缺失")
			c.Abort()
			return
		}
		claims, err := s.Parse(c.Request.Context(), tokenStr)
		if err != nil {
			response.Fail(c, http.StatusUnauthorized, "会话凭据无效或已过期")
			c.Abort()
			return
		}
		snap, err := s.users.SnapshotByID(c.Request.Context(), claims.UserID) // 实时查库，禁止缓存
		if err != nil || snap == nil {
			response.Fail(c, http.StatusUnauthorized, "会话凭据无效或已过期")
			c.Abort()
			return
		}
		if snap.CredentialVersion != claims.CredentialVersion {
			response.Fail(c, http.StatusUnauthorized, "会话凭据已失效，请重新登录")
			c.Abort()
			return
		}
		if snap.Status != "active" {
			response.Fail(c, http.StatusUnauthorized, "账号未激活或已被禁用")
			c.Abort()
			return
		}
		c.Set(CtxUserID, snap.ID)
		c.Set(CtxUserRole, snap.Role)
		c.Next()
	}
}

// AdminMiddleware 角色校验层：叠加在会话校验之后，两中间件独立可组合（Build2/3 管理端点叠加使用）
func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if role, _ := c.Get(CtxUserRole); role != "admin" {
			response.Fail(c, http.StatusForbidden, "权限不足")
			c.Abort()
			return
		}
		c.Next()
	}
}
