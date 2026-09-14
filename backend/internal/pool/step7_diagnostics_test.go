package pool

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"vpn-sub/internal/redact"
)

func TestNormalizeDiagnosticsCapsAt19PlusTruncatedSummary(t *testing.T) {
	diags := make([]ParseDiagnostic, 0, 25)
	for i := 0; i < 25; i++ {
		diags = append(diags, ParseDiagnostic{
			Line:    i + 1,
			Kind:    "warn",
			Message: fmt.Sprintf("第%d条 token=SECRET ", i+1) + strings.Repeat("你", 300),
			Raw:     "raw code=SECRET " + strings.Repeat("x", 300),
		})
	}
	got := NormalizeDiagnostics(diags)
	if len(got) != redact.MaxDiagnostics {
		t.Fatalf("诊断数量应为 %d，实际 %d", redact.MaxDiagnostics, len(got))
	}
	for i := 0; i < redact.MaxDiagnostics-1; i++ {
		if got[i].Line != i+1 || got[i].Kind != "warn" {
			t.Fatalf("第 %d 条真实诊断元数据被改写: %+v", i+1, got[i])
		}
		if strings.Contains(got[i].Message, "SECRET") || strings.Contains(got[i].Raw, "SECRET") {
			t.Fatalf("第 %d 条诊断泄漏敏感值: %+v", i+1, got[i])
		}
		if utf8.RuneCountInString(got[i].Message) > redact.MaxFieldRunes ||
			utf8.RuneCountInString(got[i].Raw) > redact.MaxFieldRunes {
			t.Fatalf("第 %d 条诊断未按 %d rune 限额: %+v", i+1, redact.MaxFieldRunes, got[i])
		}
	}
	last := got[redact.MaxDiagnostics-1]
	if last.Line != 0 || last.Kind != "truncated" || last.Message != "另有 6 条诊断未展示" || last.Raw != "" {
		t.Fatalf("第 20 条应为截断摘要: %+v", last)
	}
}

func TestNormalizeDiagnosticsBoundaryAndEmptyJSON(t *testing.T) {
	empty := NormalizeDiagnostics(nil)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("空诊断应返回非 nil 空切片: %#v", empty)
	}
	raw, err := json.Marshal(empty)
	if err != nil || string(raw) != "[]" {
		t.Fatalf("空诊断必须序列化为 []: %s err=%v", raw, err)
	}

	exact := make([]ParseDiagnostic, 20)
	for i := range exact {
		exact[i] = ParseDiagnostic{Line: i + 1, Kind: "warn", Message: "ok", Raw: ""}
	}
	if got := NormalizeDiagnostics(exact); len(got) != 20 || got[19].Kind == "truncated" {
		t.Fatalf("恰好 20 条不应截断: %+v", got)
	}

	over := make([]ParseDiagnostic, 21)
	for i := range over {
		over[i] = ParseDiagnostic{Line: i + 1, Kind: "warn", Message: "ok", Raw: ""}
	}
	got := NormalizeDiagnostics(over)
	if len(got) != 20 || got[19].Kind != "truncated" || got[19].Message != "另有 2 条诊断未展示" {
		t.Fatalf("21 条应保留 19+1: %+v", got)
	}
}
