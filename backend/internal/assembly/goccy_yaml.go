package assembly

import (
	"sort"
	"strings"

	gyaml "github.com/goccy/go-yaml"
)

// orderedMapToMapSlice 将项目内有序映射转换为 goccy 有序映射。
func orderedMapToMapSlice(m *OrderedMap) gyaml.MapSlice {
	if m == nil {
		return gyaml.MapSlice{}
	}
	out := make(gyaml.MapSlice, 0, m.Len())
	for _, key := range m.Keys() {
		value, _ := m.Get(key)
		out = append(out, gyaml.MapItem{Key: key, Value: toGoccyValue(value)})
	}
	return out
}

// toGoccyValue 递归转换 JSON/装配模型中的集合，避免无序 map 破坏稳定输出。
func toGoccyValue(value any) any {
	switch v := value.(type) {
	case *OrderedMap:
		return orderedMapToMapSlice(v)
	case gyaml.MapSlice:
		out := make(gyaml.MapSlice, 0, len(v))
		for _, item := range v {
			out = append(out, gyaml.MapItem{Key: item.Key, Value: toGoccyValue(item.Value)})
		}
		return out
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out := make(gyaml.MapSlice, 0, len(keys))
		for _, key := range keys {
			out = append(out, gyaml.MapItem{Key: key, Value: toGoccyValue(v[key])})
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = toGoccyValue(v[i])
		}
		return out
	case []string:
		return append([]string(nil), v...)
	default:
		return value
	}
}

// marshalClashYAML 统一 Clash YAML 序列化选项。
func marshalClashYAML(doc gyaml.MapSlice, comments gyaml.CommentMap) ([]byte, error) {
	opts := []gyaml.EncodeOption{gyaml.AutoInt()}
	if len(comments) > 0 {
		opts = append(opts, gyaml.WithComment(comments))
	}
	return gyaml.MarshalWithOptions(doc, opts...)
}

// proxyCommentMap 将节点区注释放在首个节点前；空列表时放在 proxies 键前。
func proxyCommentMap(comment string, hasProxies bool) gyaml.CommentMap {
	lines := goccyHeadComment(comment)
	if len(lines) == 0 {
		return nil
	}
	path := "$.proxies"
	if hasProxies {
		path = "$.proxies[0]"
	}
	return gyaml.CommentMap{path: {gyaml.HeadComment(lines...)}}
}

func goccyHeadComment(comment string) []string {
	var out []string
	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimPrefix(line, "#"))
		if line != "" {
			out = append(out, " "+line)
		}
	}
	return out
}
