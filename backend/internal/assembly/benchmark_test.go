package assembly

import (
	"fmt"
	"testing"
	"time"
)

func benchmarkInput(syntax TargetSyntax) GenerateInput {
	rules := make([]RuleLine, 10000)
	for i := range rules {
		rules[i] = RuleLine{RuleType: "DOMAIN-SUFFIX", MatchValue: fmt.Sprintf("example%d.com", i), Target: "PROXY"}
	}
	in := GenerateInput{TargetSyntax: syntax, CustomRules: rules}
	if syntax == ClashYAML {
		in.OverseasMembers = []string{"🚀直接连接"}
		in.FallbackGroupMembers = []string{"🚀直接连接", "🌎国外流量"}
	}
	return in
}

func BenchmarkRenderClash10kRules(b *testing.B) {
	svc, _, _ := newTestService(b)
	in := benchmarkInput(ClashYAML)
	ld := &loadedData{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := svc.render(in, ld); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRenderSrConf10kRules(b *testing.B) {
	svc, _, _ := newTestService(b)
	in := benchmarkInput(SrConf)
	ld := &loadedData{}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := svc.render(in, ld); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRenderClash10kRulesThreshold(t *testing.T) {
	svc, _, _ := newTestService(t)
	in := benchmarkInput(ClashYAML)
	ld := &loadedData{}
	start := time.Now()
	if _, err := svc.render(in, ld); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("Clash 1 万规则渲染超过 500ms: %v", d)
	}
}

func TestRenderSrConf10kRulesThreshold(t *testing.T) {
	svc, _, _ := newTestService(t)
	in := benchmarkInput(SrConf)
	ld := &loadedData{}
	start := time.Now()
	if _, err := svc.render(in, ld); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("SR conf 1 万规则渲染超过 500ms: %v", d)
	}
}
