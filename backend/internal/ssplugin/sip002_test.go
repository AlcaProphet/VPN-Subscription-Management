package ssplugin

import (
	"reflect"
	"testing"
)

func TestPluginStringSemanticRoundTrip(t *testing.T) {
	wantName := `custom:plugin;=\中文`
	wantOpts := map[string]string{
		"flag":    "",
		`key:;=\`: `value:;=\`,
		"percent": "%2F + 空格",
	}
	raw, err := SerializePluginString(wantName, wantOpts)
	if err != nil {
		t.Fatalf("序列化 SIP002 插件字符串失败: %v", err)
	}
	name, opts, err := ParsePluginString(raw)
	if err != nil {
		t.Fatalf("解析 SIP002 插件字符串失败: %v", err)
	}
	if name != wantName || !reflect.DeepEqual(opts, wantOpts) {
		t.Fatalf("SIP002 语义往返不一致: name=%q opts=%#v raw=%q", name, opts, raw)
	}
	again, err := SerializePluginString(name, opts)
	if err != nil {
		t.Fatal(err)
	}
	if again != raw {
		t.Fatalf("SIP002 序列化不稳定: first=%q second=%q", raw, again)
	}
}

func TestParsePluginStringRejectsAmbiguousInput(t *testing.T) {
	for _, raw := range []string{
		`plugin;key=value\`,
		`plugin;=value`,
		`plugin;key=one;key=two`,
		`plugin;key=bad\q`,
	} {
		t.Run(raw, func(t *testing.T) {
			if _, _, err := ParsePluginString(raw); err == nil {
				t.Fatalf("应拒绝无法无损解析的插件字符串 %q", raw)
			}
		})
	}
}
