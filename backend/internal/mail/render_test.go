package mail

import (
	"context"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	"vpn-sub/internal/log"
)

// parseHTMLFragment 提取文本与锚点，供安全渲染测试验证实际 DOM 语义。
func parseHTMLFragment(t *testing.T, raw string) (string, []struct{ href, text string }) {
	t.Helper()
	root, err := xhtml.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("解析 HTML 失败: %v", err)
	}
	var text strings.Builder
	var anchors []struct{ href, text string }
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && n.Data == "a" {
			var href string
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href = attr.Val
				}
			}
			var linkText strings.Builder
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == xhtml.TextNode {
					linkText.WriteString(child.Data)
				}
			}
			anchors = append(anchors, struct{ href, text string }{href, linkText.String()})
		}
		if n.Type == xhtml.TextNode {
			text.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return text.String(), anchors
}

func TestRenderFiveBranchesDefaults(t *testing.T) {
	values := RenderValues{
		SiteName: "站点示例",
		LoginURL: "https://app.example.com/login?source=test&x=1#frag",
		ResetURL: "https://app.example.com/reset#token=abc",
	}
	for _, def := range Definitions() {
		t.Run(string(def.ID), func(t *testing.T) {
			got, err := Render(def.ID, def.Default, values)
			if err != nil {
				t.Fatalf("Render 失败: %v", err)
			}
			if got.Subject == "" || got.TextBody == "" || got.HTMLBody == "" {
				t.Fatalf("渲染结果不应为空: %+v", got)
			}
			if def.ID == TemplateApprovalRejected && strings.Contains(got.TextBody, "{{") {
				t.Fatalf("审批拒绝不应残留占位符: %q", got.TextBody)
			}
			if def.ID == TemplatePasswordReset {
				if !strings.Contains(got.TextBody, values.ResetURL) {
					t.Fatalf("密码重置纯文本应含完整 reset_url: %q", got.TextBody)
				}
			}
			if def.ID == TemplateApprovalApproved || def.ID == TemplateWelcomeLocal || def.ID == TemplateWelcomeOIDC {
				if !strings.Contains(got.TextBody, values.LoginURL) {
					t.Fatalf("%s 纯文本应含完整 login_url: %q", def.ID, got.TextBody)
				}
			}
			if def.ID == TemplateApprovalRejected {
				// 不使用的 URL 值即使为空也不应触发校验失败。
				if _, err := Render(def.ID, def.Default, RenderValues{SiteName: "站点"}); err != nil {
					t.Fatalf("审批拒绝不应强制要求 URL: %v", err)
				}
			}
		})
	}
}

func TestRenderRepeatLongURLAndLineBreaks(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("segment/", 40) + "end?q=" + strings.Repeat("x", 200) + "#frag"
	got, err := Render(TemplatePasswordReset, Template{
		Subject: "x",
		Body:    "{{reset_url}}\n\t{{reset_url}}",
	}, RenderValues{ResetURL: long})
	if err != nil {
		t.Fatalf("Render 失败: %v", err)
	}
	if strings.Count(got.TextBody, long) != 2 {
		t.Fatalf("重复变量应各展开一次: %q", got.TextBody)
	}
	if !strings.Contains(got.TextBody, "\n\t") {
		t.Fatalf("纯文本应保留 LF/Tab: %q", got.TextBody)
	}
	if !strings.Contains(got.HTMLBody, "<br>") {
		t.Fatalf("HTML 应把 LF 转为 br: %q", got.HTMLBody)
	}
	_, anchors := parseHTMLFragment(t, got.HTMLBody)
	if len(anchors) != 2 {
		t.Fatalf("重复 URL 变量应生成两个锚点: %+v", anchors)
	}
	for _, a := range anchors {
		if a.href != long || a.text != long {
			t.Fatalf("长 URL 锚点不一致: %+v", a)
		}
	}
}

func TestRenderNoRecursiveInterpretationAndEscaping(t *testing.T) {
	// 变量值含模板样式文本只作为普通文本，不递归替换。
	got, err := Render(TemplateApprovalRejected, Template{Subject: "主题", Body: "{{site_name}}"}, RenderValues{
		SiteName: "{{login_url}} & <b>\"'",
	})
	if err != nil {
		t.Fatalf("Render 失败: %v", err)
	}
	if !strings.Contains(got.TextBody, "{{login_url}}") {
		t.Fatalf("变量值不应递归解释: %q", got.TextBody)
	}
	if strings.Contains(got.HTMLBody, "<b>") {
		t.Fatalf("HTML 应转义普通文本: %q", got.HTMLBody)
	}
	if !strings.Contains(got.HTMLBody, "{{login_url}}") {
		t.Fatalf("变量值中的占位符样式文本应原样保留、不递归: %q", got.HTMLBody)
	}
	if _, anchors := parseHTMLFragment(t, got.HTMLBody); len(anchors) != 0 {
		t.Fatalf("变量值中的占位符文本不应生成锚点: anchors=%+v", anchors)
	}
	text, _ := parseHTMLFragment(t, got.HTMLBody)
	if !strings.Contains(text, "{{login_url}}") || !strings.Contains(text, `& <b>"'`) {
		t.Fatalf("DOM 解码后文本不符: %q", text)
	}
	if strings.Contains(got.HTMLBody, "<script") {
		t.Fatalf("不得生成脚本节点: %q", got.HTMLBody)
	}
}

func TestRenderActualURLValidation(t *testing.T) {
	base := Template{Subject: "x", Body: "{{reset_url}}"}
	invalid := []string{
		"",
		"/relative",
		"ftp://example.com/reset",
		"https://",
		"https://user:pass@example.com/reset",
		"https://example.com/reset\ninjected",
	}
	for _, raw := range invalid {
		t.Run(raw, func(t *testing.T) {
			_, err := Render(TemplatePasswordReset, base, RenderValues{ResetURL: raw})
			if err == nil {
				t.Fatalf("非法 reset_url 应失败: %q", raw)
			}
			if raw != "" && strings.Contains(err.Error(), raw) {
				t.Fatalf("错误不得回显 URL: %v", err)
			}
		})
	}
	valid := []string{
		"http://example.com/reset",
		"https://example.com/reset?x=1&y=2#frag",
		"https://example.com/reset%20token",
	}
	for _, raw := range valid {
		got, err := Render(TemplatePasswordReset, base, RenderValues{ResetURL: raw})
		if err != nil {
			t.Fatalf("合法 URL 应通过: %q err=%v", raw, err)
		}
		if !strings.Contains(got.TextBody, raw) {
			t.Fatalf("纯文本应保留完整 URL: %q", got.TextBody)
		}
		_, anchors := parseHTMLFragment(t, got.HTMLBody)
		if len(anchors) != 1 || anchors[0].href != raw || anchors[0].text != raw {
			t.Fatalf("锚点 href/文字应与纯文本 URL 完全一致: anchors=%+v", anchors)
		}
	}
}

func TestRenderStaticURLConservativeBoundary(t *testing.T) {
	renderBody := func(t *testing.T, body string) (Rendered, error) {
		t.Helper()
		return Render(TemplateApprovalRejected, Template{Subject: "主题", Body: body}, RenderValues{})
	}

	linked := []string{
		"https://example.com/path",
		"https://example.com/path?x=1&y=2#frag",
		"https://example.com/path%20x",
	}
	for _, body := range linked {
		t.Run("链接/"+body, func(t *testing.T) {
			got, err := renderBody(t, body)
			if err != nil {
				t.Fatalf("应成功: %v", err)
			}
			if !strings.Contains(got.TextBody, body) {
				t.Fatalf("纯文本应保留原始 URL: %q", got.TextBody)
			}
			_, anchors := parseHTMLFragment(t, got.HTMLBody)
			if len(anchors) != 1 || anchors[0].href != body || anchors[0].text != body {
				t.Fatalf("静态 URL 锚点不一致: anchors=%+v", anchors)
			}
		})
	}

	plain := []string{
		"https://example.com/path,",
		"https://example.com/path.",
		"https://example.com/path。",
		"(https://example.com/path)",
		"请访问https://example.com/path",
		"https://example.com/path（x）",
	}
	for _, body := range plain {
		t.Run("普通文本/"+body, func(t *testing.T) {
			got, err := renderBody(t, body)
			if err != nil {
				t.Fatalf("保守边界不应报错: %v", err)
			}
			if _, anchors := parseHTMLFragment(t, got.HTMLBody); len(anchors) != 0 {
				t.Fatalf("紧邻标点/文字不应自动链接: anchors=%+v", anchors)
			}
			if !strings.Contains(got.TextBody, body) {
				t.Fatalf("普通文本应原样保留: %q", got.TextBody)
			}
		})
	}

	// 以 http(s):// 开头的明确候选若非法，必须返回 400 类错误而不是降级为普通文本。
	for _, body := range []string{"https://", "https://user:pass@example.com"} {
		t.Run("非法候选/"+body, func(t *testing.T) {
			_, err := renderBody(t, body)
			if err == nil {
				t.Fatalf("非法候选应失败: %q", body)
			}
		})
	}
}

func TestRenderLinkHrefVisibleAndPlainTextExact(t *testing.T) {
	raw := "https://example.invalid/a%20b?q=1&r=%2F#frag"
	got, err := Render(TemplateWelcomeLocal, Template{
		Subject: "欢迎",
		Body:    "请登录：{{login_url}}\n结束",
	}, RenderValues{SiteName: "站点", LoginURL: raw})
	if err != nil {
		t.Fatalf("Render 失败: %v", err)
	}
	if !strings.Contains(got.TextBody, raw) {
		t.Fatalf("纯文本应含完整 URL: %q", got.TextBody)
	}
	_, anchors := parseHTMLFragment(t, got.HTMLBody)
	if len(anchors) != 1 || anchors[0].href != raw || anchors[0].text != raw {
		t.Fatalf("href/可见文字/纯文本必须完全一致: anchors=%+v", anchors)
	}
}

func TestPreviewUsesEffectiveSiteNameAndSyntheticURLs(t *testing.T) {
	def, err := Definition(TemplateWelcomeLocal)
	if err != nil {
		t.Fatal(err)
	}
	svc := NewService(nil, log.New("error", "console"))
	got, err := svc.PreviewTemplate(context.Background(), def.ID, def.Default)
	if err != nil {
		t.Fatalf("预览失败: %v", err)
	}
	values := PreviewValues("VPN 订阅管理")
	if values.SiteName != "VPN 订阅管理" ||
		values.ResetURL != "https://example.invalid/reset/example-token?source=preview" ||
		values.LoginURL != "https://example.invalid/login?source=preview" {
		t.Fatalf("预览值不符: %+v", values)
	}
	want, err := Render(def.ID, def.Default, values)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("预览必须与 Render(kind, template, 有效站点名+合成 URL) 完全一致:\ngot=%+v\nwant=%+v", got, want)
	}
}
