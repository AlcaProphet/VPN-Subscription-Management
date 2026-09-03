package node

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// sensitiveItemIDField 是对象数组条目的内部稳定身份，仅用于凭据合并与编辑回显。
const sensitiveItemIDField = "_credential_id"

type pathSegment struct {
	name     string
	selector string
	list     bool
}

func parsePath(path string) ([]pathSegment, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	segments := make([]pathSegment, 0, len(parts))
	for _, part := range parts {
		segment := pathSegment{name: part}
		if open := strings.IndexByte(part, '['); open >= 0 {
			if open == 0 || !strings.HasSuffix(part, "]") || strings.Count(part, "[") != 1 {
				return nil, false
			}
			segment.name = part[:open]
			segment.selector = part[open+1 : len(part)-1]
			segment.list = true
		}
		if segment.name == "" || (segment.list && strings.ContainsAny(segment.selector, "[].")) {
			return nil, false
		}
		segments = append(segments, segment)
	}
	return segments, true
}

func listItem(items []any, selector string) (map[string]any, bool) {
	if index, err := strconv.Atoi(selector); err == nil {
		if index < 0 || index >= len(items) {
			return nil, false
		}
		item, ok := items[index].(map[string]any)
		return item, ok
	}
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		if id, _ := item[sensitiveItemIDField].(string); id == selector {
			return item, true
		}
	}
	return nil, false
}

func getPathValue(m map[string]any, path string) (any, bool) {
	segments, ok := parsePath(path)
	if !ok {
		return nil, false
	}
	var current any = m
	for _, segment := range segments {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[segment.name]
		if !ok {
			return nil, false
		}
		if segment.list {
			items, ok := current.([]any)
			if !ok {
				return nil, false
			}
			current, ok = listItem(items, segment.selector)
			if !ok {
				return nil, false
			}
		}
	}
	return current, true
}

func setPathValue(m map[string]any, path string, value any) bool {
	segments, ok := parsePath(path)
	if !ok || len(segments) == 0 {
		return false
	}
	current := m
	for i, segment := range segments {
		last := i == len(segments)-1
		if segment.list {
			items, ok := current[segment.name].([]any)
			if !ok {
				return false
			}
			item, ok := listItem(items, segment.selector)
			if !ok {
				return false
			}
			if last {
				return false
			}
			current = item
			continue
		}
		if last {
			current[segment.name] = value
			return true
		}
		next, ok := current[segment.name].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment.name] = next
		}
		current = next
	}
	return false
}

func deletePathValue(m map[string]any, path string) bool {
	segments, ok := parsePath(path)
	if !ok || len(segments) == 0 {
		return false
	}
	current := m
	for i, segment := range segments {
		last := i == len(segments)-1
		if segment.list {
			items, ok := current[segment.name].([]any)
			if !ok {
				return false
			}
			item, ok := listItem(items, segment.selector)
			if !ok || last {
				return false
			}
			current = item
			continue
		}
		if last {
			delete(current, segment.name)
			return true
		}
		next, ok := current[segment.name].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	return false
}

func sensitivePathMatches(pattern, path string) bool {
	patternSegments, ok := parsePath(pattern)
	if !ok {
		return false
	}
	pathSegments, ok := parsePath(path)
	if !ok || len(patternSegments) != len(pathSegments) {
		return false
	}
	for i := range patternSegments {
		want, got := patternSegments[i], pathSegments[i]
		if want.name != got.name || want.list != got.list {
			return false
		}
		if want.list {
			if want.selector != "" && want.selector != got.selector {
				return false
			}
			if want.selector == "" {
				parsed, err := uuid.Parse(got.selector)
				if err != nil || parsed.String() != got.selector {
					return false
				}
			}
		}
	}
	return true
}

func isSensitivePath(patterns []string, path string) bool {
	for _, pattern := range patterns {
		if sensitivePathMatches(pattern, path) {
			return true
		}
	}
	return false
}

// ensureSensitiveItemIDs 为含敏感子字段的对象数组补齐稳定身份，并拒绝重复身份。
func ensureSensitiveItemIDs(params map[string]any, patterns []string) error {
	for _, pattern := range patterns {
		segments, ok := parsePath(pattern)
		if !ok {
			return fmt.Errorf("非法敏感字段路径: %s", pattern)
		}
		if err := ensurePathItemIDs(params, segments); err != nil {
			return fmt.Errorf("敏感字段路径 %s: %w", pattern, err)
		}
	}
	return nil
}

func ensurePathItemIDs(current any, segments []pathSegment) error {
	if len(segments) == 0 {
		return nil
	}
	object, ok := current.(map[string]any)
	if !ok {
		return nil
	}
	segment := segments[0]
	value, exists := object[segment.name]
	if !exists {
		return nil
	}
	if !segment.list {
		return ensurePathItemIDs(value, segments[1:])
	}
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	seen := make(map[string]bool, len(items))
	for index, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id, _ := item[sensitiveItemIDField].(string)
		parsed, err := uuid.Parse(id)
		if err != nil {
			id = uuid.NewString()
		} else {
			id = parsed.String()
		}
		item[sensitiveItemIDField] = id
		if seen[id] {
			return fmt.Errorf("数组条目稳定身份重复: %s[%d]", segment.name, index)
		}
		seen[id] = true
		if err := ensurePathItemIDs(item, segments[1:]); err != nil {
			return err
		}
	}
	return nil
}

// ConcreteSensitivePaths 将注册表数组模式展开为带稳定条目 ID 的具体敏感路径。
func ConcreteSensitivePaths(params map[string]any, patterns []string) []string {
	paths := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		segments, ok := parsePath(pattern)
		if !ok {
			continue
		}
		expandSensitivePath(params, segments, "", &paths)
	}
	return uniqueSortedStrings(paths)
}

func expandSensitivePath(current any, segments []pathSegment, prefix string, paths *[]string) {
	if len(segments) == 0 {
		*paths = append(*paths, prefix)
		return
	}
	object, ok := current.(map[string]any)
	if !ok {
		return
	}
	segment := segments[0]
	value, exists := object[segment.name]
	if !exists {
		return
	}
	name := segment.name
	if prefix != "" {
		name = prefix + "." + name
	}
	if !segment.list {
		expandSensitivePath(value, segments[1:], name, paths)
		return
	}
	items, ok := value.([]any)
	if !ok {
		return
	}
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			continue
		}
		id, _ := item[sensitiveItemIDField].(string)
		if id == "" {
			continue
		}
		expandSensitivePath(item, segments[1:], name+"["+id+"]", paths)
	}
}

func concreteSensitivePathUnion(patterns []string, values ...map[string]any) []string {
	seen := map[string]bool{}
	for _, value := range values {
		for _, path := range ConcreteSensitivePaths(value, patterns) {
			seen[path] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

// StripInternalProtocolMetadata 从目标检查与客户端输出副本移除 schema 声明的编辑内部字段。
func StripInternalProtocolMetadata(proto Protocol, params map[string]any) map[string]any {
	out := cloneJSONMap(params)
	stripInternalFields(out, proto.FormSchema)
	return out
}

func stripInternalFields(object map[string]any, fields []FieldSchema) {
	for _, field := range fields {
		value, ok := object[field.Name]
		if !ok || field.Type != "object" {
			continue
		}
		switch field.ObjectKind {
		case "fields":
			if child, ok := value.(map[string]any); ok {
				stripInternalFields(child, field.Properties)
			}
		case "list":
			items, ok := value.([]any)
			if !ok {
				continue
			}
			for _, value := range items {
				item, ok := value.(map[string]any)
				if !ok {
					continue
				}
				if field.ItemIDField != "" {
					delete(item, field.ItemIDField)
				}
				stripInternalFields(item, field.Properties)
			}
		}
	}
}
