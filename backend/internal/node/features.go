package node

import "strings"

// FeatureSchema 声明功能的控制值；对象使用 Toggle 子字段，标量使用自身值。
// 嵌套名称以点分层，父功能关闭时同时重置子功能范围。
type FeatureSchema struct {
	Name          string `json:"name"`
	Toggle        string `json:"toggle,omitempty"`
	DisabledValue string `json:"disabled_value,omitempty"`
}

func featureObject(name string, field FieldSchema) FieldSchema {
	field.Feature = &FeatureSchema{Name: name, Toggle: "enabled"}
	field.ResetOn = []string{"feature." + name}
	for i := range field.Properties {
		if field.Properties[i].Name != "enabled" {
			field.Properties[i].When = &ConditionRule{Features: []string{name}}
		}
	}
	return field
}

// setScalarFeatures 将已有顶层功能也交给同一个关闭流程；普通布尔字段不声明功能。
func setScalarFeatures(fields []FieldSchema) {
	for i := range fields {
		field := &fields[i]
		switch field.Name {
		case "udp-over-tcp", "udp-over-stream", "xudp", "multiplexing":
			field.Feature = &FeatureSchema{Name: field.Name}
			if field.Name == "multiplexing" {
				field.Feature.DisabledValue = "MULTIPLEXING_OFF"
			}
			if !field.ShouldReset("feature." + field.Name) {
				field.ResetOn = append(field.ResetOn, "feature."+field.Name)
			}
		case "udp-over-tcp-version", "udp-over-stream-version":
			name := strings.TrimSuffix(field.Name, "-version")
			field.ResetOn = []string{"feature." + name}
			field.When = &ConditionRule{Features: []string{name}}
		}
	}
}

func featureEnabled(field FieldSchema, value any) bool {
	if field.Feature.Toggle != "" {
		object, _ := value.(map[string]any)
		value = object[field.Feature.Toggle]
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return field.Feature.DisabledValue != "" && v != "" && v != field.Feature.DisabledValue
	default:
		return false
	}
}

func activeFeatures(fields []FieldSchema, params map[string]any) []string {
	var result []string
	for _, field := range fields {
		value := params[field.Name]
		if field.Feature != nil {
			if !featureEnabled(field, value) {
				continue
			}
			result = append(result, field.Feature.Name)
		}
		if object, ok := value.(map[string]any); ok {
			result = append(result, activeFeatures(field.Properties, object)...)
		}
	}
	return uniqueSortedStrings(result)
}

// cleanDisabledFeatures 只清理已声明功能，不把输出转换形态用作存储模型。
// 未知键随关闭的所属对象一起清除；非法控制值保留给正常类型校验。
func cleanDisabledFeatures(fields []FieldSchema, params map[string]any) map[string]any {
	out := cloneJSONMap(params)
	active := activeFeatures(fields, out)
	var clean func([]FieldSchema, map[string]any)
	clean = func(schema []FieldSchema, object map[string]any) {
		for _, field := range schema {
			value, exists := object[field.Name]
			if !exists {
				continue
			}
			if field.Feature != nil && field.Feature.Toggle != "" && !featureEnabled(field, value) {
				if child, ok := value.(map[string]any); ok {
					toggle, present := child[field.Feature.Toggle]
					if !present {
						delete(object, field.Name)
						continue
					}
					if disabled, ok := toggle.(bool); ok && !disabled {
						object[field.Name] = map[string]any{field.Feature.Toggle: false}
						continue
					}
				}
			}
			if field.Feature == nil {
				remove := false
				for _, scope := range field.ResetOn {
					if strings.HasPrefix(scope, "feature.") && !containsString(active, strings.TrimPrefix(scope, "feature.")) {
						remove = true
					}
				}
				if remove {
					delete(object, field.Name)
					continue
				}
			}
			if child, ok := value.(map[string]any); ok {
				clean(field.Properties, child)
			}
		}
	}
	clean(fields, out)
	return out
}

// resetSchemaFields 按完整对象路径递归重置；对象根被命中时连同未知键删除。
func resetSchemaFields(object map[string]any, fields []FieldSchema, scope string) {
	for _, field := range fields {
		if field.ShouldReset(scope) {
			delete(object, field.Name)
			continue
		}
		if child, ok := object[field.Name].(map[string]any); ok {
			resetSchemaFields(child, field.Properties, scope)
		}
	}
}

func schemaPathResets(path, scope string, fields []FieldSchema) bool {
	for _, field := range fields {
		if field.ShouldReset(scope) && (path == field.Name || strings.HasPrefix(path, field.Name+".")) {
			return true
		}
		if strings.HasPrefix(path, field.Name+".") && schemaPathResets(strings.TrimPrefix(path, field.Name+"."), scope, field.Properties) {
			return true
		}
	}
	return false
}

func disabledFeatureScopes(fields []FieldSchema, params map[string]any) []string {
	active := activeFeatures(fields, params)
	var scopes []string
	var visit func([]FieldSchema)
	visit = func(schema []FieldSchema) {
		for _, field := range schema {
			if field.Feature != nil && !containsString(active, field.Feature.Name) {
				scopes = append(scopes, "feature."+field.Feature.Name)
			}
			visit(field.Properties)
		}
	}
	visit(fields)
	return scopes
}
