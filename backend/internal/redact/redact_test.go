package redact

import (
	"strings"
	"testing"
)

func TestRedactTextCoversSensitiveKeys(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://example.com/rules.txt?token=abc", "https://example.com/rules.txt?token=***"},
		{"token=abc&code=def&state=xyz", "token=***&code=***&state=***"},
		{"?Token=abc&TOKEN=def", "?Token=***&TOKEN=***"},
		{"?to%6ben=abc", "?to%6ben=***"},
		{"https://user:password@example.com/path", "https://user:***@example.com/path"},
		{"x=1&notsecret=abc&client_secret=xyz", "x=1&notsecret=abc&client_secret=***"},
		{"cannot parse this token=abc text", "cannot parse this token=*** text"},
		{"fragment #token=abc", "fragment #token=***"},
	}
	for _, c := range cases {
		if got := RedactText(c.in); got != c.want {
			t.Errorf("RedactText(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestRedactDisplayURLPreservesStructure(t *testing.T) {
	in := "https://example.com/path?a=1&token=abc&a=2&code=def#state=ghi"
	got := RedactDisplayURL(in)
	if strings.Contains(got, "abc") || strings.Contains(got, "def") || strings.Contains(got, "ghi") {
		t.Fatalf("RedactDisplayURL 泄漏敏感值: %s", got)
	}
	if !strings.Contains(got, "a=1") || !strings.Contains(got, "a=2") || strings.Count(got, "a=") != 2 {
		t.Fatalf("RedactDisplayURL 未保留非敏感重复参数: %s", got)
	}
	if !strings.Contains(got, "token=***") || !strings.Contains(got, "code=***") || !strings.Contains(got, "state=***") {
		t.Fatalf("RedactDisplayURL 未脱敏敏感参数: %s", got)
	}
}

func TestRedactDisplayURLUserinfo(t *testing.T) {
	got := RedactDisplayURL("https://user:supersecret@example.com/x")
	if strings.Contains(got, "supersecret") {
		t.Fatalf("userinfo 密码泄漏: %s", got)
	}
	if !strings.Contains(got, "user:***@example.com") {
		t.Fatalf("userinfo 结构异常: %s", got)
	}
}

func TestRedactTextDoesNotRedactMatchValueNoResolve(t *testing.T) {
	// no-resolve 不是敏感 key；确保普通 URL 非敏感参数保留。
	in := "https://example.com/rule?no-resolve.example.com=1&domain=ok"
	got := RedactText(in)
	if strings.Contains(got, "no-resolve.example.com=***") {
		t.Fatalf("不应把 no-resolve.example.com 当作敏感参数: %s", got)
	}
	if !strings.Contains(got, "domain=ok") {
		t.Fatalf("非敏感参数被误删: %s", got)
	}
}

func TestTruncateTextUnicode(t *testing.T) {
	s := "你好世界abcdef"
	if got := TruncateText(s, 4); got != "你好世界" {
		t.Fatalf("Unicode 截断异常: %q", got)
	}
	if got := TruncateText(s, 100); got != s {
		t.Fatalf("超长阈值不应截断: %q", got)
	}
	if got := TruncateText("abc", 0); got != "" {
		t.Fatalf("0 阈值应返回空: %q", got)
	}
}
