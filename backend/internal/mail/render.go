package mail

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RenderValues 单封业务邮件的实际变量值。
type RenderValues struct {
	SiteName string
	LoginURL string
	ResetURL string
}

// Rendered 一封业务邮件的主题、纯文本正文与最小 HTML 正文。
type Rendered struct {
	Subject  string `json:"subject"`
	TextBody string `json:"text_body"`
	HTMLBody string `json:"html_body"`
}

// 预览链接使用固定合成值，不读取真实 URL 或一次性 token。
const (
	PreviewResetURL = "https://example.invalid/reset/example-token?source=preview"
	PreviewLoginURL = "https://example.invalid/login?source=preview"
)

// PreviewValues 返回有效站点名称与固定合成链接组成的预览值。
func PreviewValues(siteName string) RenderValues {
	return RenderValues{SiteName: siteName, LoginURL: PreviewLoginURL, ResetURL: PreviewResetURL}
}

// Render 是业务发送与管理员预览的唯一渲染入口：
// 先复用模板领域校验，再校验实际 URL 值，并分别生成纯文本与最小 HTML。
func Render(kind TemplateKind, t Template, values RenderValues) (Rendered, error) {
	def, err := Definition(kind)
	if err != nil {
		return Rendered{}, err
	}
	normalized := NormalizeTemplate(t)
	if err := ValidateTemplate(kind, normalized); err != nil {
		return Rendered{}, err
	}
	subject, _, err := renderContent(def, normalized.Subject, def.SubjectVariables, values, false)
	if err != nil {
		return Rendered{}, err
	}
	textBody, htmlBody, err := renderContent(def, normalized.Body, def.BodyVariables, values, true)
	if err != nil {
		return Rendered{}, err
	}
	return Rendered{Subject: subject, TextBody: textBody, HTMLBody: htmlBody}, nil
}

// PreviewTemplate 使用当前有效站点名称与固定合成链接调用同一个 Render；不发送邮件。
func (s *Service) PreviewTemplate(ctx context.Context, kind TemplateKind, t Template) (Rendered, error) {
	return Render(kind, t, PreviewValues(s.cfg.EffectiveSiteName(ctx)))
}

// renderContent 单次扫描模板，生成纯文本与最小 HTML；变量值不会被再次解释。
func renderContent(def TemplateDefinition, text string, allowed []string, values RenderValues, allowStaticURL bool) (string, string, error) {
	var plain, htmlBody strings.Builder
	for i := 0; i < len(text); {
		next := strings.Index(text[i:], "{{")
		if next < 0 {
			if err := appendLiteral(&plain, &htmlBody, text[i:], allowStaticURL); err != nil {
				return "", "", err
			}
			break
		}
		next += i
		if err := appendLiteral(&plain, &htmlBody, text[i:next], allowStaticURL); err != nil {
			return "", "", err
		}
		end := strings.Index(text[next+2:], "}}")
		if end < 0 {
			return "", "", fmt.Errorf("%w：占位符未闭合", ErrInvalidTemplate)
		}
		name := text[next+2 : next+2+end]
		if !containsString(allowed, name) {
			return "", "", fmt.Errorf("%w：包含不允许的占位符", ErrInvalidTemplate)
		}
		raw, isURL, err := resolveVariable(name, values)
		if err != nil {
			return "", "", err
		}
		if isURL {
			if err := validateActualURL(raw); err != nil {
				return "", "", err
			}
			plain.WriteString(raw)
			htmlBody.WriteString(`<a href="`)
			htmlBody.WriteString(html.EscapeString(raw))
			htmlBody.WriteString(`">`)
			htmlBody.WriteString(html.EscapeString(raw))
			htmlBody.WriteString(`</a>`)
		} else {
			plain.WriteString(raw)
			appendHTMLText(&htmlBody, raw)
		}
		i = next + 2 + end + 2
	}
	return plain.String(), htmlBody.String(), nil
}

func appendLiteral(plain, htmlBody *strings.Builder, literal string, allowStaticURL bool) error {
	for i := 0; i < len(literal); {
		r, size := utf8.DecodeRuneInString(literal[i:])
		if unicode.IsSpace(r) {
			j := i + size
			for j < len(literal) {
				r2, size2 := utf8.DecodeRuneInString(literal[j:])
				if !unicode.IsSpace(r2) {
					break
				}
				j += size2
			}
			plain.WriteString(literal[i:j])
			appendHTMLText(htmlBody, literal[i:j])
			i = j
			continue
		}
		j := i + size
		for j < len(literal) {
			r2, size2 := utf8.DecodeRuneInString(literal[j:])
			if unicode.IsSpace(r2) {
				break
			}
			j += size2
		}
		word := literal[i:j]
		if allowStaticURL && isStaticURLCandidate(word) {
			if err := validateActualURL(word); err != nil {
				return err
			}
			plain.WriteString(word)
			htmlBody.WriteString(`<a href="`)
			htmlBody.WriteString(html.EscapeString(word))
			htmlBody.WriteString(`">`)
			htmlBody.WriteString(html.EscapeString(word))
			htmlBody.WriteString(`</a>`)
		} else {
			plain.WriteString(word)
			appendHTMLText(htmlBody, word)
		}
		i = j
	}
	return nil
}

// isStaticURLCandidate 采用保守边界：只有以 http(s):// 开头的空白 token、全 ASCII 可见、
// 不含引号/括号/大括号，且末字符不是常见标点时，才交给 URL 校验；否则整体保持普通文本。
func isStaticURLCandidate(word string) bool {
	if len(word) < len("http://") {
		return false
	}
	lower := strings.ToLower(word)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return false
	}
	for i := 0; i < len(word); i++ {
		if word[i] < 0x21 || word[i] > 0x7e {
			return false
		}
	}
	if strings.ContainsAny(word, "\"'<>()[]{}") {
		return false
	}
	switch word[len(word)-1] {
	case '.', ',', ';', ':', '!', '?', ')', ']', '}', '\'', '"', '>':
		return false
	}
	return true
}

// validateActualURL 校验实际变量值与已识别静态候选：绝对 http/https、host 非空、无 userinfo/opaque。
func validateActualURL(raw string) error {
	if raw == "" {
		return fmt.Errorf("%w：实际链接为空", ErrInvalidTemplate)
	}
	for _, r := range raw {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return fmt.Errorf("%w：实际链接格式无效", ErrInvalidTemplate)
		}
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w：实际链接格式无效", ErrInvalidTemplate)
	}
	if !strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https") {
		return fmt.Errorf("%w：实际链接仅支持 http/https", ErrInvalidTemplate)
	}
	if u.Hostname() == "" || u.User != nil || u.Opaque != "" {
		return fmt.Errorf("%w：实际链接格式无效", ErrInvalidTemplate)
	}
	return nil
}

func resolveVariable(name string, values RenderValues) (string, bool, error) {
	switch name {
	case "site_name":
		return values.SiteName, false, nil
	case "login_url":
		return values.LoginURL, true, nil
	case "reset_url":
		return values.ResetURL, true, nil
	default:
		return "", false, fmt.Errorf("%w：包含不允许的占位符", ErrInvalidTemplate)
	}
}

func appendHTMLText(b *strings.Builder, s string) {
	for _, r := range s {
		if r == '\n' {
			b.WriteString("<br>")
			continue
		}
		b.WriteString(html.EscapeString(string(r)))
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
