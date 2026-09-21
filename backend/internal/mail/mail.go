// Package mail 提供 SMTP 邮件服务（Build3 Step 2）：配置读 system_config、发送测试邮件与三类业务邮件模板。
// 设计约束：SMTP 未配置或发送失败不阻断主流程——返回 error 供业务层记录并携带标记（Design1 §4.6）。
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/smtp"
	"slices"
	"strings"
	"time"

	"vpn-sub/internal/config"
)

// 配置键（存 system_config；smtp_password 为编译期固定敏感键，由 config 包自动加解密）
const (
	KeyHost     = "smtp_host"
	KeyPort     = "smtp_port"
	KeyUser     = "smtp_user"
	KeyPassword = "smtp_password" // 敏感加密
	KeyFrom     = "smtp_from"
	KeySecurity = "smtp_security" // starttls/implicit_tls/plain
	KeyAuth     = config.SMTPAuthRequiredKey
	KeyScopes   = "smtp_enabled_scopes" // JSON 数组：password_reset/approval_notify/welcome，默认全不启用
)

// 邮件启用范围（smtp_enabled_scopes 取值）
const (
	ScopePasswordReset  = "password_reset"
	ScopeApprovalNotify = "approval_notify"
	ScopeWelcome        = "welcome"
)

// Service SMTP 邮件服务
type Service struct {
	cfg         *config.Service
	log         *slog.Logger
	dialContext func(ctx context.Context, network, address string) (net.Conn, error)
	// tlsConfig 仅供测试注入自签证书/信任配置；生产默认 nil，行为与内建默认一致。
	tlsConfig *tls.Config
}

// sendError 保留内部原因供诊断，但对页面与普通日志只暴露阶段化提示。
type sendError struct {
	stage FailureStage
	err   error
}

func (e *sendError) Error() string {
	if errors.Is(e.err, errStartTLSNotSupported) {
		return errStartTLSNotSupported.Error()
	}
	return e.stage.message()
}

func (e *sendError) Unwrap() error { return e.err }

func NewService(cfg *config.Service, lg *slog.Logger) *Service {
	return newServiceWithDialContext(cfg, lg, (&net.Dialer{}).DialContext)
}

// newServiceWithDialContext 构造注入网络拨号（测试可替换为本地 stub，不引入包级可变状态）。
func newServiceWithDialContext(cfg *config.Service, lg *slog.Logger, dialContext func(ctx context.Context, network, address string) (net.Conn, error)) *Service {
	return &Service{cfg: cfg, log: lg, dialContext: dialContext}
}

// tlsConfigForHost 返回本次 TLS 握手配置：默认使用内建配置，仅测试注入覆盖敏感/证书校验参数。
func (s *Service) tlsConfigForHost(host string) *tls.Config {
	cfg := &tls.Config{}
	if s.tlsConfig != nil {
		cfg = s.tlsConfig.Clone()
	}
	if cfg.ServerName == "" {
		cfg.ServerName = host
	}
	return cfg
}

// Configured SMTP 是否按当前连接方式与认证设置完整配置。
func (s *Service) Configured(ctx context.Context) bool {
	return config.SMTPConfigured(ctx, s.cfg)
}

// Send 发送固定单一 text/plain 报文（SMTP 测试邮件沿用此入口）；未配置或发送失败返回 error。
func (s *Service) Send(ctx context.Context, to, subject, body string) error {
	if err := ctx.Err(); err != nil {
		return &sendError{stage: FailureCanceled, err: err}
	}
	from := s.cfg.GetOr(ctx, KeyFrom)
	msg, err := buildPlainMessage(from, to, subject, body)
	if err != nil {
		return &sendError{stage: FailureRender, err: err}
	}
	return s.sendMessage(ctx, from, to, msg)
}

// sendMessage 只负责 SMTP 会话：接收已构造的完整报文字节，不参与正文/模板拼接。
func (s *Service) sendMessage(ctx context.Context, from, to string, msg []byte) error {
	if err := ctx.Err(); err != nil {
		return &sendError{stage: FailureCanceled, err: err}
	}
	if !s.Configured(ctx) {
		return &sendError{stage: FailureConfig, err: errSMTPNotConfigured}
	}
	host := s.cfg.GetOr(ctx, KeyHost)
	port := s.cfg.GetOr(ctx, KeyPort)
	user := s.cfg.GetOr(ctx, KeyUser)
	pass := s.cfg.GetOr(ctx, KeyPassword)
	security := s.cfg.GetOr(ctx, KeySecurity)
	authRequired := s.cfg.GetOr(ctx, KeyAuth) == "true"
	addr := net.JoinHostPort(host, port)
	// 包括 TCP、TLS、SMTP 会话与 DATA 的总期限；取消请求时关闭连接以解除阻塞读取。
	sendCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var conn net.Conn
	var err error
	if security == "implicit_tls" {
		conn, err = (&tls.Dialer{NetDialer: &net.Dialer{}, Config: s.tlsConfigForHost(host)}).DialContext(sendCtx, "tcp", addr)
	} else {
		conn, err = s.dialContext(sendCtx, "tcp", addr)
	}
	if err != nil {
		return &sendError{stage: FailureConnect, err: err}
	}
	stopClose := context.AfterFunc(sendCtx, func() { _ = conn.Close() })
	defer stopClose()
	if deadline, ok := sendCtx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return &sendError{stage: FailureInternal, err: err}
		}
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return &sendError{stage: FailureHandshake, err: err}
	}
	defer client.Close()
	// STARTTLS 必须升级成功；本地无认证中继保持明文且不尝试升级。
	if security == config.SMTPSecurityStartTLS {
		// Go 1.26 的 net/smtp.Client.Extension 返回 (bool, string)，内部 hello() 的错误会被吞掉；
		// 先显式执行 Hello 以暴露 EHLO/HELO 阶段错误，再读取缓存的扩展能力。
		if err := client.Hello(""); err != nil {
			return &sendError{stage: FailureStartTLS, err: err}
		}
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			return &sendError{stage: FailureStartTLS, err: errStartTLSNotSupported}
		}
		if err := client.StartTLS(s.tlsConfigForHost(host)); err != nil {
			return &sendError{stage: FailureStartTLS, err: err}
		}
	}
	if authRequired {
		if err := client.Auth(smtp.PlainAuth("", user, pass, host)); err != nil {
			return &sendError{stage: FailureAuth, err: err}
		}
	}
	if err := client.Mail(from); err != nil {
		return &sendError{stage: FailureMailFrom, err: err}
	}
	if err := client.Rcpt(to); err != nil {
		return &sendError{stage: FailureRcptTo, err: err}
	}
	w, err := client.Data()
	if err != nil {
		return &sendError{stage: FailureData, err: err}
	}
	if _, err := w.Write(msg); err != nil {
		return &sendError{stage: FailureData, err: err}
	}
	if err := w.Close(); err != nil {
		return &sendError{stage: FailureData, err: err}
	}
	if err := client.Quit(); err != nil {
		return &sendError{stage: FailureQuit, err: err}
	}
	return nil
}

// sendMultipart 构造业务邮件 multipart/alternative 报文后交给同一 SMTP 会话函数。
func (s *Service) sendMultipart(ctx context.Context, to, subject, textBody, htmlBody string) error {
	from := s.cfg.GetOr(ctx, KeyFrom)
	msg, err := buildMultipartAlternativeMessage(from, to, subject, textBody, htmlBody)
	if err != nil {
		return &sendError{stage: FailureRender, err: err}
	}
	return s.sendMessage(ctx, from, to, msg)
}

// sanitizeHeader 清洗邮件头中的换行（防 SMTP 头注入）
func sanitizeHeader(s string) string {
	return strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ").Replace(s)
}

// SendTest 发送测试邮件到当前操作管理员邮箱，失败返回安全阶段错误（供面板展示）
func (s *Service) SendTest(ctx context.Context, adminEmail string) error {
	return s.Send(ctx, adminEmail, "SMTP 配置测试", "这是一封测试邮件，收到即表示 SMTP 配置正确。")
}

// testSMTP 供 Dispatcher 同步诊断路径调用，保持测试邮件不进入业务队列。
func (s *Service) testSMTP(ctx context.Context, adminEmail string) error {
	return s.SendTest(ctx, adminEmail)
}

// ScopeEnabled 某邮件类型是否在启用范围内（smtp_enabled_scopes JSON 数组）
func (s *Service) ScopeEnabled(ctx context.Context, scope string) bool {
	scopes := s.cfg.GetJSONStringSlice(ctx, KeyScopes)
	return slices.Contains(scopes, scope)
}

// availability 严格判定某业务 scope 是否可用：scope 启用且 SMTP 配置完整。
// 配置读取/JSON 解析失败返回错误；scope 未启用或 SMTP 不完整返回 Available=false 及稳定 reason。
func (s *Service) availability(ctx context.Context, scope string) (Availability, error) {
	scopes, err := s.cfg.GetJSONStringSliceStrict(ctx, KeyScopes)
	if err != nil {
		return Availability{}, err
	}
	if !slices.Contains(scopes, scope) {
		return Availability{Reason: ReasonScopeDisabled}, nil
	}
	configured, err := config.SMTPConfiguredStrict(ctx, s.cfg)
	if err != nil {
		return Availability{}, err
	}
	if !configured {
		return Availability{Reason: ReasonConfigUnavailable}, nil
	}
	return Availability{Available: true}, nil
}

// siteContext 严格读取有效站点名与前端登录地址；供派发器在入队时快照。
// login_url 允许为空或非法，由 worker 在发送阶段按 render 失败记录；但读取错误必须向上返回。
func (s *Service) siteContext(ctx context.Context) (siteName, loginURL string, err error) {
	siteName, err = s.cfg.EffectiveSiteNameStrict(ctx)
	if err != nil {
		return "", "", err
	}
	loginURL, err = s.cfg.Get(ctx, config.KeyFrontendURL)
	if err != nil {
		return "", "", err
	}
	return siteName, loginURL, nil
}

// siteName 严格读取有效站点名；审批拒绝只使用站点名时避免为 frontend_url 读取引入失败。
func (s *Service) siteName(ctx context.Context) (string, error) {
	return s.cfg.EffectiveSiteNameStrict(ctx)
}

// frontendURL 严格读取前端地址，供密码重置相对路径构造绝对 URL。
func (s *Service) frontendURL(ctx context.Context) (string, error) {
	return s.cfg.Get(ctx, config.KeyFrontendURL)
}

// sendJob 发送已入队任务：worker 使用；不检查 scope，模板与 SMTP 配置按发送时读取。
func (s *Service) sendJob(ctx context.Context, kind TemplateKind, to string, values RenderValues) error {
	return s.renderAndSendTemplate(ctx, kind, to, values)
}

// --- 业务邮件（Design5 §七 / Build28 §3.8）---

// renderAndSendTemplate 读取三态模板并复用 Render；读取损坏或数据库错误时使用内置默认值继续发送。
func (s *Service) renderAndSendTemplate(ctx context.Context, kind TemplateKind, to string, values RenderValues) error {
	// LoadTemplate 已记录不含模板内容的读取类别，并在读取错误/损坏时返回内置默认值；本封邮件继续发送。
	t, _, loadErr := s.LoadTemplate(ctx, kind)
	if loadErr != nil {
		s.log.Debug("邮件模板读取失败，继续使用内置默认值", "template", kind)
	}
	rendered, err := Render(kind, t, values)
	if err != nil {
		return &sendError{stage: FailureRender, err: err}
	}
	return s.sendMultipart(ctx, to, rendered.Subject, rendered.TextBody, rendered.HTMLBody)
}

// SendWelcome 非审批路径首次激活的欢迎邮件；仅 source=="oidc" 走 OIDC 分支，其余既有本地来源走 local。
func (s *Service) SendWelcome(ctx context.Context, to, siteName, loginURL, source string) error {
	if !s.ScopeEnabled(ctx, ScopeWelcome) {
		return nil
	}
	kind := TemplateWelcomeLocal
	if source == "oidc" {
		kind = TemplateWelcomeOIDC
	}
	return s.renderAndSendTemplate(ctx, kind, to, RenderValues{SiteName: siteName, LoginURL: loginURL})
}

// SendApprovalApproved 审批通过通知（approval_notify scope）；每个成功审批账号至多一封。
func (s *Service) SendApprovalApproved(ctx context.Context, to, siteName, loginURL string) error {
	if !s.ScopeEnabled(ctx, ScopeApprovalNotify) {
		return nil
	}
	return s.renderAndSendTemplate(ctx, TemplateApprovalApproved, to, RenderValues{SiteName: siteName, LoginURL: loginURL})
}

// SendApprovalRejected 审批拒绝通知（approval_notify scope）；每个成功拒绝账号至多一封。
func (s *Service) SendApprovalRejected(ctx context.Context, to, siteName string) error {
	if !s.ScopeEnabled(ctx, ScopeApprovalNotify) {
		return nil
	}
	return s.renderAndSendTemplate(ctx, TemplateApprovalRejected, to, RenderValues{SiteName: siteName})
}

// SendPasswordReset 密码重置邮件（password_reset scope；reset_url 必填且为绝对 http/https URL）。
func (s *Service) SendPasswordReset(ctx context.Context, to, resetURL string) error {
	if !s.ScopeEnabled(ctx, ScopePasswordReset) {
		return nil
	}
	return s.renderAndSendTemplate(ctx, TemplatePasswordReset, to, RenderValues{ResetURL: resetURL})
}
