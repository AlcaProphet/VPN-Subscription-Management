package assembly

import (
	"strings"
	"testing"

	gyaml "github.com/goccy/go-yaml"
)

func TestClashEmojiRaw(t *testing.T) {
	doc := gyaml.MapSlice{
		{Key: "proxies", Value: []any{orderedMapToMapSlice(NewOrderedMap().Set("name", "🚀🌎🛟😀").Set("type", "vless"))}},
	}
	content, err := marshalClashYAML(doc, nil)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if !strings.Contains(string(content), "🚀🌎🛟😀") || strings.Contains(string(content), `\U0001F680`) {
		t.Fatalf("emoji 未以 UTF-8 原文输出:\n%s", content)
	}
	var decoded any
	if err := gyaml.UnmarshalWithOptions(content, &decoded, gyaml.UseOrderedMap()); err != nil {
		t.Fatalf("回读失败: %v", err)
	}
	root, _ := yamlMap(decoded)
	proxies, _ := seqOfValue(root, "proxies")
	proxy, _ := yamlMap(proxies[0])
	if got := mapString(proxy, "name"); got != "🚀🌎🛟😀" {
		t.Fatalf("emoji 回读不一致: %q", got)
	}
}

func TestGoccyAutoInt(t *testing.T) {
	content, err := marshalClashYAML(gyaml.MapSlice{{Key: "port", Value: float64(7890)}}, nil)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if !strings.Contains(string(content), "port: 7890\n") || strings.Contains(string(content), "7890.0") {
		t.Fatalf("AutoInt 未生效:\n%s", content)
	}
}

func TestGoccyCommentMap(t *testing.T) {
	for _, tc := range []struct {
		name       string
		proxies    []any
		comment    string
		wantBefore string
	}{
		{name: "有节点", proxies: []any{gyaml.MapSlice{{Key: "name", Value: "节点A"}}}, comment: "# {{xray_nodes}}", wantBefore: "- name: 节点A"},
		{name: "空节点", proxies: []any{}, comment: "# {{xray_nodes}}", wantBefore: "proxies: []"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := gyaml.MapSlice{{Key: "proxies", Value: tc.proxies}}
			content, err := marshalClashYAML(doc, proxyCommentMap(tc.comment, len(tc.proxies) > 0))
			if err != nil {
				t.Fatalf("序列化失败: %v", err)
			}
			text := string(content)
			commentAt := strings.Index(text, "# {{xray_nodes}}")
			valueAt := strings.Index(text, tc.wantBefore)
			if commentAt < 0 || valueAt < 0 || commentAt > valueAt || strings.Contains(text, "##") {
				t.Fatalf("注释位置或格式错误:\n%s", text)
			}
		})
	}
}
