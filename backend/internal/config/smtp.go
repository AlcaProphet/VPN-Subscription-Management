package config

import (
	"context"
	"net"
	"strconv"
)

const (
	SMTPSecurityStartTLS    = "starttls"
	SMTPSecurityImplicitTLS = "implicit_tls"
	SMTPSecurityPlain       = "plain"
	SMTPAuthRequiredKey     = "smtp_auth_required"
)

// SMTPLoopbackHost 明文中继仅允许同容器的回环地址。
func SMTPLoopbackHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// SMTPConfigured 统一面板、邮件服务和用户管理的可用性判定。
func SMTPConfigured(ctx context.Context, cfg *Service) bool {
	if cfg == nil {
		return false
	}
	ok, err := smtpConfiguredWith(ctx, func(ctx context.Context, key string) (string, error) {
		return cfg.GetOr(ctx, key), nil
	})
	if err != nil && cfg.log != nil {
		cfg.log.Warn("读取 SMTP 配置失败，按未配置降级", "err", err)
	}
	return ok
}

// SMTPConfiguredStrict 严格判定 SMTP 配置：配置读取失败直接返回错误，供派发前可用性查询使用。
// 与 SMTPConfigured 共享同一判定规则，避免面板、用户管理和邮件派发口径漂移。
func SMTPConfiguredStrict(ctx context.Context, cfg *Service) (bool, error) {
	if cfg == nil {
		return false, nil
	}
	return smtpConfiguredWith(ctx, cfg.Get)
}

// smtpGet 是严格/宽松配置读取的共用函数签名。
type smtpGet func(ctx context.Context, key string) (string, error)

// smtpConfiguredWith 实现 SMTP 完整配置判定；getter 决定读取失败是返回错误还是按空值降级。
func smtpConfiguredWith(ctx context.Context, get smtpGet) (bool, error) {
	host, err := get(ctx, "smtp_host")
	if err != nil {
		return false, err
	}
	portRaw, err := get(ctx, "smtp_port")
	if err != nil {
		return false, err
	}
	from, err := get(ctx, "smtp_from")
	if err != nil {
		return false, err
	}
	port, err := strconv.Atoi(portRaw)
	if host == "" || err != nil || port < 1 || port > 65535 || from == "" {
		return false, nil
	}
	security, err := get(ctx, "smtp_security")
	if err != nil {
		return false, err
	}
	auth, err := get(ctx, SMTPAuthRequiredKey)
	if err != nil {
		return false, err
	}
	if auth != "true" && auth != "false" {
		return false, nil
	}
	switch security {
	case SMTPSecurityStartTLS, SMTPSecurityImplicitTLS:
		if auth == "true" {
			user, err := get(ctx, "smtp_user")
			if err != nil {
				return false, err
			}
			password, err := get(ctx, "smtp_password")
			if err != nil {
				return false, err
			}
			return user != "" && password != "" && password != "***", nil
		}
		return true, nil
	case SMTPSecurityPlain:
		return auth == "false" && SMTPLoopbackHost(host), nil
	default:
		return false, nil
	}
}
