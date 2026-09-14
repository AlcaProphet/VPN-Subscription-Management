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
	host := cfg.GetOr(ctx, "smtp_host")
	port, err := strconv.Atoi(cfg.GetOr(ctx, "smtp_port"))
	if host == "" || err != nil || port < 1 || port > 65535 || cfg.GetOr(ctx, "smtp_from") == "" {
		return false
	}
	security := cfg.GetOr(ctx, "smtp_security")
	auth := cfg.GetOr(ctx, SMTPAuthRequiredKey)
	if auth != "true" && auth != "false" {
		return false
	}
	switch security {
	case SMTPSecurityStartTLS, SMTPSecurityImplicitTLS:
		if auth == "true" {
			password := cfg.GetOr(ctx, "smtp_password")
			return cfg.GetOr(ctx, "smtp_user") != "" && password != "" && password != "***"
		}
		return true
	case SMTPSecurityPlain:
		return auth == "false" && SMTPLoopbackHost(host)
	default:
		return false
	}
}
