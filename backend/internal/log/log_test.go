package log

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

// TestRedact 覆盖验收项：路径内 token、多参数间 token、消息体内嵌 token 均脱敏
func TestRedact(t *testing.T) {
	cases := []struct{ in, want string }{
		{"/x?token=abc", "/x?token=***"},
		{"a=1&token=abc&b=2", "a=1&token=***&b=2"},
		{"/sub?token=xyz&platform=clash", "/sub?token=***&platform=clash"},
		{"no token here", "no token here"},
		{"token=without_prefix", "token=without_prefix"}, // 无 ? 或 & 前缀不匹配
	}
	for _, c := range cases {
		if got := Redact(c.in); got != c.want {
			t.Errorf("Redact(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestHandlerRedact 验证 RedactHandler：消息与字符串属性统一脱敏
func TestHandlerRedact(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, nil)
	h := NewRedactHandler(inner)
	l := slog.New(h)
	l.Info("下载请求 /x?token=secret123", "path", "/sub/download?token=abc&a=1", "count", 3)

	out := buf.String()
	if strings.Contains(out, "secret123") {
		t.Errorf("消息中的 token 未脱敏: %s", out)
	}
	if strings.Contains(out, "token=abc") {
		t.Errorf("属性中的 token 未脱敏: %s", out)
	}
	if !strings.Contains(out, "token=***") {
		t.Errorf("脱敏标记缺失: %s", out)
	}
	if !strings.Contains(out, "count=3") {
		t.Errorf("非字符串属性被误处理: %s", out)
	}
}

// TestNewFormats 验证双格式输出与分级
func TestNewFormats(t *testing.T) {
	// console 格式
	l1 := New("info", "console")
	if l1 == nil {
		t.Fatal("console logger 构建失败")
	}
	// json 格式
	l2 := New("debug", "json")
	if l2 == nil {
		t.Fatal("json logger 构建失败")
	}
	// 级别过滤：error 级别下 info 不输出
	l3 := New("error", "console")
	ctx := context.Background()
	if l3.Enabled(ctx, slog.LevelInfo) {
		t.Error("error 级别下 info 不应启用")
	}
	if !l3.Enabled(ctx, slog.LevelError) {
		t.Error("error 级别下 error 应启用")
	}
	// SetLevel 运行时切换生效
	SetLevel("debug")
	if !l3.Enabled(ctx, slog.LevelDebug) {
		t.Error("切换 debug 后 debug 应启用")
	}
}
