// Package mail 提供 SMTP 邮件服务（Build3 Step 2）：配置读 system_config、发送测试邮件与三类业务邮件模板。
// 设计约束：SMTP 未配置或发送失败不阻断主流程——返回 error 供业务层记录并携带标记（Design1 §4.6）。
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"slices"
	"strings"

	"vpn-sub/internal/config"
)

// 配置键（存 system_config；smtp_password 为敏感键，登记入 config.sensitiveKeys 加密落库）
const (
	KeyHost     = "smtp_host"
	KeyPort     = "smtp_port"
	KeyUser     = "smtp_user"
	KeyPassword = "smtp_password"        // 敏感加密
	KeyFrom     = "smtp_from"
	KeyTLS      = "smtp_tls"             // "true"/"false"
	KeyScopes   = "smtp_enabled_scopes"  // JSON 数组：password_reset/approval_notify/welcome，默认全不启用
)

// 邮件启用范围（smtp_enabled_scopes 取值）
const (
	ScopePasswordReset = "password_reset"
	ScopeApprovalNotify = "approval_notify"
	ScopeWelcome        = "welcome"
)

// init 登记敏感配置键：smtp_password 以 AES-256-GCM 加密落库（AGENTS §4.2，Build3 面板配置接通）
func init() {
	config.RegisterSensitive(KeyPassword)
}

// Service SMTP 邮件服务
type Service struct {
	cfg *config.Service
	log *slog.Logger
}

func NewService(cfg *config.Service, lg *slog.Logger) *Service {
	return &Service{cfg: cfg, log: lg}
}

// Configured SMTP 是否已配置（host+user+password 三键非空，与用户管理 smtpConfigured 同口径）
func (s *Service) Configured(ctx context.Context) bool {
	host, _ := s.cfg.Get(ctx, KeyHost)
	user, _ := s.cfg.Get(ctx, KeyUser)
	pass, _ := s.cfg.Get(ctx, KeyPassword)
	return host != "" && user != "" && pass != ""
}

// Send SMTP 发送（TLS 直连 / STARTTLS 升级 / 明文三路径）；未配置或发送失败返回 error（不 panic、不阻断调用方）
func (s *Service) Send(ctx context.Context, to, subject, body string) error {
	if !s.Configured(ctx) {
		return errors.New("SMTP 未配置")
	}
	host, _ := s.cfg.Get(ctx, KeyHost)
	port, _ := s.cfg.Get(ctx, KeyPort)
	if port == "" {
		port = "587"
	}
	user, _ := s.cfg.Get(ctx, KeyUser)
	pass, _ := s.cfg.Get(ctx, KeyPassword)
	from, _ := s.cfg.Get(ctx, KeyFrom)
	if from == "" {
		from = user // 发件人缺省取账号
	}
	useTLS := s.cfg.GetBool(ctx, KeyTLS, true)
	addr := net.JoinHostPort(host, port)

	var conn net.Conn
	var err error
	if useTLS {
		conn, err = tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	} else {
		conn, err = net.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("初始化 SMTP 会话失败: %w", err)
	}
	defer client.Close()
	// 明文路径尝试 STARTTLS 升级（服务器支持时）
	if !useTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: host}); err != nil {
				return fmt.Errorf("STARTTLS 升级失败: %w", err)
			}
		}
	}
	if user != "" {
		if err := client.Auth(smtp.PlainAuth("", user, pass, host)); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("发件人被拒: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("收件人被拒: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("开始写入邮件失败: %w", err)
	}
	// 组装 RFC822 报文（Subject 清洗换行防头注入；站点名等外部输入可能含 \r\n）
	msg := "From: " + sanitizeHeader(from) + "\r\n" +
		"To: " + sanitizeHeader(to) + "\r\n" +
		"Subject: " + sanitizeHeader(subject) + "\r\n" +
		"Content-Type: text/plain; charset=utf-8\r\n" +
		"MIME-Version: 1.0\r\n\r\n" + body
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("结束邮件写入失败: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("SMTP 退出失败: %w", err)
	}
	return nil
}

// sanitizeHeader 清洗邮件头中的换行（防 SMTP 头注入）
func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}

// SendTest 发送测试邮件到当前操作管理员邮箱，失败返回具体错误（供面板展示）
func (s *Service) SendTest(ctx context.Context, adminEmail string) error {
	return s.Send(ctx, adminEmail, "SMTP 配置测试", "这是一封测试邮件，收到即表示 SMTP 配置正确。")
}

// ScopeEnabled 某邮件类型是否在启用范围内（smtp_enabled_scopes JSON 数组）
func (s *Service) ScopeEnabled(ctx context.Context, scope string) bool {
	scopes := s.cfg.GetJSONStringSlice(ctx, KeyScopes)
	return slices.Contains(scopes, scope)
}

// --- 业务邮件模板（纯文本，最小模板，Design1 §3.4.6）---

// SendWelcome 欢迎邮件——所有新用户首次激活时发送（审批通过/直接激活/白名单命中/管理员创建均发送）；
// 按来源区分文案：本地创建/自注册 → 邮箱与密码登录；OIDC → 单点登录（不携带凭据）
func (s *Service) SendWelcome(ctx context.Context, to, siteName, loginURL, source string) error {
	if !s.ScopeEnabled(ctx, ScopeWelcome) {
		return nil
	}
	body := siteName + "\n\n"
	if source == "oidc" {
		body += "您的账号已激活，请使用单点登录（OIDC）登录：" + loginURL
	} else {
		body += "您的账号已激活，请使用邮箱与密码登录：" + loginURL
	}
	return s.Send(ctx, to, siteName+" 账号已激活", body)
}

// SendApprovalNotify 通过/拒绝通知（按 approval_notify scope）；拒绝通知在点击「拒绝」动作时触发发送
func (s *Service) SendApprovalNotify(ctx context.Context, to, siteName string, approved bool) error {
	if !s.ScopeEnabled(ctx, ScopeApprovalNotify) {
		return nil
	}
	var body string
	if approved {
		body = "您在 " + siteName + " 的账号已通过审批，现在可以登录。"
	} else {
		body = "您在 " + siteName + " 的账号申请未通过审批。"
	}
	return s.Send(ctx, to, siteName+" 审批通知", body)
}

// SendPasswordReset 密码重置邮件（接通 Build1 Step 7 预留的 sendMail 注入点）
func (s *Service) SendPasswordReset(ctx context.Context, to, resetURL string) error {
	if !s.ScopeEnabled(ctx, ScopePasswordReset) {
		return nil
	}
	return s.Send(ctx, to, "密码重置", "请在 1 小时内使用以下链接重置密码（一次性）：\n"+resetURL)
}
