package mail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

// 邮件模板固定 ID（同时用于路由参数与前端值）。
type TemplateKind string

const (
	TemplatePasswordReset    TemplateKind = "password_reset"
	TemplateApprovalApproved TemplateKind = "approval_approved"
	TemplateApprovalRejected TemplateKind = "approval_rejected"
	TemplateWelcomeLocal     TemplateKind = "welcome_local"
	TemplateWelcomeOIDC      TemplateKind = "welcome_oidc"
)

// 模板配置键（仅后端持久化使用，不暴露为可编辑字段）。
const (
	ConfigKeyPasswordReset    = "mail_template_password_reset"
	ConfigKeyApprovalApproved = "mail_template_approval_approved"
	ConfigKeyApprovalRejected = "mail_template_approval_rejected"
	ConfigKeyWelcomeLocal     = "mail_template_welcome_local"
	ConfigKeyWelcomeOIDC      = "mail_template_welcome_oidc"
)

// 模板长度上限均按 Unicode code point 计算。
const (
	MaxSubjectRunes = 200
	MaxBodyRunes    = 10000
)

// Template 单个模板的持久化与编辑对象。
type Template struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// TemplateDefinition 固定模板定义：ID、默认值、变量、scope 与显示顺序的单一来源。
type TemplateDefinition struct {
	ID                    TemplateKind
	ConfigKey             string
	Label                 string
	Scope                 string
	Default               Template
	SubjectVariables      []string
	BodyVariables         []string
	RequiredBodyVariables []string
}

// TemplateState 模板读取状态。
type TemplateState string

const (
	TemplateStateDefault    TemplateState = "default"
	TemplateStateCustomized TemplateState = "customized"
	TemplateStateDamaged    TemplateState = "damaged"
)

// TemplateView 管理端读取返回的有效模板视图。
type TemplateView struct {
	ID                    TemplateKind  `json:"id"`
	Label                 string        `json:"label"`
	Scope                 string        `json:"scope"`
	Subject               string        `json:"subject"`
	Body                  string        `json:"body"`
	State                 TemplateState `json:"state"`
	Warning               string        `json:"warning"`
	SubjectVariables      []string      `json:"subject_variables"`
	BodyVariables         []string      `json:"body_variables"`
	RequiredBodyVariables []string      `json:"required_body_variables"`
}

var (
	// ErrUnknownTemplate 未知模板 ID。
	ErrUnknownTemplate = errors.New("未知邮件模板")
	// ErrInvalidTemplate 模板 JSON 或领域校验失败。
	ErrInvalidTemplate = errors.New("邮件模板无效")
)

const damagedTemplateWarning = "模板配置损坏，已回退内置默认文案；可重新保存或恢复默认。"

// templateDefinitions 固定顺序是管理 API 与前端展示的稳定顺序，禁止动态注册或新增。
var templateDefinitions = []TemplateDefinition{
	{
		ID:                    TemplatePasswordReset,
		ConfigKey:             ConfigKeyPasswordReset,
		Label:                 "密码重置",
		Scope:                 ScopePasswordReset,
		Default:               Template{Subject: "密码重置", Body: "请在 1 小时内使用以下链接重置密码（一次性）：\n{{reset_url}}"},
		SubjectVariables:      []string{},
		BodyVariables:         []string{"reset_url"},
		RequiredBodyVariables: []string{"reset_url"},
	},
	{
		ID:                    TemplateApprovalApproved,
		ConfigKey:             ConfigKeyApprovalApproved,
		Label:                 "审批通过",
		Scope:                 ScopeApprovalNotify,
		Default:               Template{Subject: "{{site_name}} 审批通知", Body: "您在 {{site_name}} 的账号已通过审批，现在可以登录：\n{{login_url}}"},
		SubjectVariables:      []string{"site_name"},
		BodyVariables:         []string{"site_name", "login_url"},
		RequiredBodyVariables: []string{"login_url"},
	},
	{
		ID:                    TemplateApprovalRejected,
		ConfigKey:             ConfigKeyApprovalRejected,
		Label:                 "审批拒绝",
		Scope:                 ScopeApprovalNotify,
		Default:               Template{Subject: "{{site_name}} 审批通知", Body: "您在 {{site_name}} 的账号申请未通过审批。"},
		SubjectVariables:      []string{"site_name"},
		BodyVariables:         []string{"site_name"},
		RequiredBodyVariables: []string{},
	},
	{
		ID:                    TemplateWelcomeLocal,
		ConfigKey:             ConfigKeyWelcomeLocal,
		Label:                 "本地欢迎",
		Scope:                 ScopeWelcome,
		Default:               Template{Subject: "{{site_name}} 账号已激活", Body: "{{site_name}}\n\n您的账号已激活，请使用邮箱与密码登录：{{login_url}}"},
		SubjectVariables:      []string{"site_name"},
		BodyVariables:         []string{"site_name", "login_url"},
		RequiredBodyVariables: []string{"login_url"},
	},
	{
		ID:                    TemplateWelcomeOIDC,
		ConfigKey:             ConfigKeyWelcomeOIDC,
		Label:                 "OIDC 欢迎",
		Scope:                 ScopeWelcome,
		Default:               Template{Subject: "{{site_name}} 账号已激活", Body: "{{site_name}}\n\n您的账号已激活，请使用单点登录（OIDC）登录：{{login_url}}"},
		SubjectVariables:      []string{"site_name"},
		BodyVariables:         []string{"site_name", "login_url"},
		RequiredBodyVariables: []string{"login_url"},
	},
}

// Definitions 返回固定模板定义的深拷贝，调用方修改不会污染包内单一来源。
func Definitions() []TemplateDefinition {
	out := make([]TemplateDefinition, 0, len(templateDefinitions))
	for _, def := range templateDefinitions {
		out = append(out, cloneDefinition(def))
	}
	return out
}

// Definition 按 ID 返回固定定义；未知 ID 返回 ErrUnknownTemplate。
func Definition(kind TemplateKind) (TemplateDefinition, error) {
	for _, def := range templateDefinitions {
		if def.ID == kind {
			return cloneDefinition(def), nil
		}
	}
	return TemplateDefinition{}, ErrUnknownTemplate
}

func cloneDefinition(def TemplateDefinition) TemplateDefinition {
	def.SubjectVariables = append([]string{}, def.SubjectVariables...)
	def.BodyVariables = append([]string{}, def.BodyVariables...)
	def.RequiredBodyVariables = append([]string{}, def.RequiredBodyVariables...)
	return def
}

// NormalizeTemplate 只做正文 CRLF/CR → LF 规范化；主题保持原值供长度与单行校验。
func NormalizeTemplate(t Template) Template {
	t.Body = normalizeBody(t.Body)
	return t
}

func normalizeBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// ValidateTemplate 统一领域校验：保存、预览、导入和实际发送必须复用本函数。
func ValidateTemplate(kind TemplateKind, t Template) error {
	def, err := Definition(kind)
	if err != nil {
		return err
	}
	if err := validateSubject(def, t.Subject); err != nil {
		return err
	}
	body := normalizeBody(t.Body)
	if err := validateBody(def, body); err != nil {
		return err
	}
	return nil
}

func validateSubject(def TemplateDefinition, subject string) error {
	n := utf8.RuneCountInString(subject)
	if n < 1 || n > MaxSubjectRunes {
		return fmt.Errorf("%w：主题长度必须为 1～%d 字符", ErrInvalidTemplate, MaxSubjectRunes)
	}
	if strings.TrimSpace(subject) == "" {
		return fmt.Errorf("%w：主题不能为空", ErrInvalidTemplate)
	}
	for _, r := range subject {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w：主题不能包含换行或控制字符", ErrInvalidTemplate)
		}
	}
	return validatePlaceholders(subject, def.SubjectVariables, "主题")
}

func validateBody(def TemplateDefinition, body string) error {
	n := utf8.RuneCountInString(body)
	if n < 1 || n > MaxBodyRunes {
		return fmt.Errorf("%w：正文长度必须为 1～%d 字符", ErrInvalidTemplate, MaxBodyRunes)
	}
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("%w：正文不能为空", ErrInvalidTemplate)
	}
	for _, r := range body {
		if r == '\n' || r == '\t' {
			continue
		}
		if unicode.IsControl(r) {
			return fmt.Errorf("%w：正文包含不允许的控制字符", ErrInvalidTemplate)
		}
	}
	if err := validatePlaceholders(body, def.BodyVariables, "正文"); err != nil {
		return err
	}
	for _, v := range def.RequiredBodyVariables {
		if !strings.Contains(body, "{{"+v+"}}") {
			return fmt.Errorf("%w：正文必须包含 {{%s}}", ErrInvalidTemplate, v)
		}
	}
	return nil
}

// validatePlaceholders 只允许精确的 {{[a-z_]+}} 白名单字面量；任何未闭合/越界占位符均拒绝。
func validatePlaceholders(text string, allowed []string, field string) error {
	allowedSet := make(map[string]bool, len(allowed))
	for _, v := range allowed {
		allowedSet[v] = true
	}
	for i := 0; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "{{"):
			rest := text[i+2:]
			end := strings.Index(rest, "}}")
			if end < 0 {
				return fmt.Errorf("%w：%s中的占位符未闭合", ErrInvalidTemplate, field)
			}
			name := rest[:end]
			if !isPlaceholderName(name) || !allowedSet[name] {
				return fmt.Errorf("%w：%s包含不允许的占位符", ErrInvalidTemplate, field)
			}
			i += 2 + end + 2
		case strings.HasPrefix(text[i:], "}}"):
			return fmt.Errorf("%w：%s中的占位符未闭合", ErrInvalidTemplate, field)
		default:
			i++
		}
	}
	return nil
}

func isPlaceholderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		b := name[i]
		if (b < 'a' || b > 'z') && b != '_' {
			return false
		}
	}
	return true
}

type templateJSON struct {
	Subject *string `json:"subject"`
	Body    *string `json:"body"`
}

// ParseTemplateJSON 严格解析持久化 JSON：只接受 subject/body 两个必需字符串字段。
func ParseTemplateJSON(raw string) (Template, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var in templateJSON
	if err := dec.Decode(&in); err != nil {
		return Template{}, fmt.Errorf("%w：模板 JSON 非法", ErrInvalidTemplate)
	}
	if in.Subject == nil || in.Body == nil {
		return Template{}, fmt.Errorf("%w：模板 JSON 缺少 subject 或 body", ErrInvalidTemplate)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Template{}, fmt.Errorf("%w：模板 JSON 存在尾随内容", ErrInvalidTemplate)
	}
	return Template{Subject: *in.Subject, Body: *in.Body}, nil
}

// LoadTemplate 读取并校验单分支覆盖：
// 缺键 → default；合法覆盖 → customized；JSON/领域校验失败 → damaged + 默认值；
// 存储读取错误 → 默认值 + damaged + 非 nil 安全错误，供发送降级与管理 API 区分 500。
func (s *Service) LoadTemplate(ctx context.Context, kind TemplateKind) (Template, TemplateState, error) {
	def, err := Definition(kind)
	if err != nil {
		return Template{}, TemplateStateDefault, err
	}
	raw, err := s.cfg.Get(ctx, def.ConfigKey)
	if err != nil {
		s.warnTemplate(kind, def.ConfigKey, "read_error")
		return def.Default, TemplateStateDamaged, fmt.Errorf("读取邮件模板失败: %w", err)
	}
	if raw == "" {
		exists, existsErr := s.cfg.Exists(ctx, def.ConfigKey)
		if existsErr != nil {
			s.warnTemplate(kind, def.ConfigKey, "read_error")
			return def.Default, TemplateStateDamaged, fmt.Errorf("读取邮件模板失败: %w", existsErr)
		}
		if !exists {
			return def.Default, TemplateStateDefault, nil
		}
	}
	t, err := ParseTemplateJSON(raw)
	if err != nil {
		s.warnTemplate(kind, def.ConfigKey, "invalid_json")
		return def.Default, TemplateStateDamaged, nil
	}
	if err := ValidateTemplate(kind, t); err != nil {
		s.warnTemplate(kind, def.ConfigKey, "invalid_template")
		return def.Default, TemplateStateDamaged, nil
	}
	return NormalizeTemplate(t), TemplateStateCustomized, nil
}

// SaveTemplate 先领域校验再只 UPSERT 目标配置键，不触碰其他模板、scope 或 SMTP 配置。
func (s *Service) SaveTemplate(ctx context.Context, kind TemplateKind, t Template) (TemplateView, error) {
	def, err := Definition(kind)
	if err != nil {
		return TemplateView{}, err
	}
	normalized := NormalizeTemplate(t)
	if err := ValidateTemplate(kind, normalized); err != nil {
		return TemplateView{}, err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return TemplateView{}, fmt.Errorf("序列化邮件模板失败: %w", err)
	}
	if err := s.cfg.Set(ctx, def.ConfigKey, string(raw)); err != nil {
		return TemplateView{}, fmt.Errorf("保存邮件模板失败: %w", err)
	}
	return templateView(def, normalized, TemplateStateCustomized), nil
}

// RestoreTemplate 只删除目标键，操作幂等；恢复后返回内置默认视图。
func (s *Service) RestoreTemplate(ctx context.Context, kind TemplateKind) (TemplateView, error) {
	def, err := Definition(kind)
	if err != nil {
		return TemplateView{}, err
	}
	if err := s.cfg.Delete(ctx, def.ConfigKey); err != nil {
		return TemplateView{}, fmt.Errorf("恢复默认邮件模板失败: %w", err)
	}
	return templateView(def, def.Default, TemplateStateDefault), nil
}

// ListTemplates 按固定顺序返回五个有效模板；任一存储读取错误导致整体失败（管理 API 500）。
func (s *Service) ListTemplates(ctx context.Context) ([]TemplateView, error) {
	out := make([]TemplateView, 0, len(templateDefinitions))
	for _, def := range templateDefinitions {
		t, state, err := s.LoadTemplate(ctx, def.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, templateView(def, t, state))
	}
	return out, nil
}

// ValidateTemplateOverrides 只校验五个已知模板配置键；未知键（包括未知 mail_template_ 前缀）不处理。
func ValidateTemplateOverrides(cfg map[string]string) error {
	for _, def := range templateDefinitions {
		raw, ok := cfg[def.ConfigKey]
		if !ok {
			continue
		}
		t, err := ParseTemplateJSON(raw)
		if err != nil {
			return fmt.Errorf("%w：模板 %s 的 JSON 非法", ErrInvalidTemplate, def.ID)
		}
		if err := ValidateTemplate(def.ID, t); err != nil {
			return fmt.Errorf("%w：模板 %s 校验失败", ErrInvalidTemplate, def.ID)
		}
	}
	return nil
}

func templateView(def TemplateDefinition, t Template, state TemplateState) TemplateView {
	if state == TemplateStateDefault || state == TemplateStateDamaged {
		t = def.Default
	}
	warning := ""
	if state == TemplateStateDamaged {
		warning = damagedTemplateWarning
	}
	return TemplateView{
		ID:                    def.ID,
		Label:                 def.Label,
		Scope:                 def.Scope,
		Subject:               t.Subject,
		Body:                  t.Body,
		State:                 state,
		Warning:               warning,
		SubjectVariables:      append([]string{}, def.SubjectVariables...),
		BodyVariables:         append([]string{}, def.BodyVariables...),
		RequiredBodyVariables: append([]string{}, def.RequiredBodyVariables...),
	}
}

func (s *Service) warnTemplate(kind TemplateKind, configKey, category string) {
	if s.log == nil {
		return
	}
	// 只记录模板 ID、配置键与错误类别，不记录主题、正文、URL 或配置原始值。
	s.log.Warn("邮件模板读取异常", "kind", string(kind), "config_key", configKey, "category", category)
}
