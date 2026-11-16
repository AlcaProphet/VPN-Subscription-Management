package node

// ConditionRule 是协议字段的声明式活动条件。
// 不引入脚本求值；同一维度的多个值表示“或”，不同维度之间表示“且”。
type ConditionRule struct {
	Network   []string            `json:"network,omitempty"`
	Security  []string            `json:"security,omitempty"`
	Plugin    []string            `json:"plugin,omitempty"`
	PluginNot []string            `json:"plugin_not,omitempty"`
	Features  []string            `json:"features,omitempty"`
	Selectors map[string][]string `json:"selectors,omitempty"`
	// NonEmpty 声明依赖的兄弟字段在 protocol_json 中必须存在有效值（Build32 Step 14 引入的第八个维度）。
	// 路径使用与 protocol_json 一致的规范点路径；任一列出的字段缺失或为空即不匹配。
	NonEmpty []string `json:"non_empty,omitempty"`
	Targets  []string `json:"targets,omitempty"`
}

// SelectorSchema 声明协议级 selector 的名称、允许值、默认值与来源字段。
// SourceField 为空表示 state_only selector，只保存在 current_state_json；
// 非空时使用点路径从 protocol_json 的普通 wire 字段派生选择值。
type SelectorSchema struct {
	Name        string   `json:"name"`
	Values      []string `json:"values"`
	Default     string   `json:"default"`
	SourceField string   `json:"source_field,omitempty"`
}

// EndpointPolicy 声明当前协议／selector 组合下 host/port 的显示、必填与 wire 输出策略。
// When 为空表示默认策略；每个状态必须且只能命中一条策略。
type EndpointPolicy struct {
	When     *ConditionRule `json:"when,omitempty"`
	HostMode string         `json:"host_mode"` // required|optional|hidden
	PortMode string         `json:"port_mode"` // required|optional|hidden
	EmitHost bool           `json:"emit_host"`
	EmitPort bool           `json:"emit_port"`
}

// OptionItem 是下拉/可搜索输入使用的推荐项。
type OptionItem struct {
	Value    string `json:"value"`
	Label    string `json:"label,omitempty"`
	Group    string `json:"group,omitempty"`
	Verified string `json:"verified,omitempty"`
}

// TargetEvidence 描述字段在特定输出目标上的证据范围。
type TargetEvidence struct {
	Target  string `json:"target"`
	Client  string `json:"client,omitempty"`
	Version string `json:"version,omitempty"`
	Entry   string `json:"entry,omitempty"`
	Status  string `json:"status"` // complete|equivalent|partial|unsupported|unverified
}

// Matches 判断当前字段是否属于给定状态。
// target 为空时只判断节点公共活动状态，忽略目标限定，供活动参数投影使用。
// root 是完整的 protocol_json，用于评估 non_empty 兄弟字段条件。
func (f FieldSchema) Matches(state CurrentState, root map[string]any, target string) bool {
	return f.When == nil || f.When.Matches(state, root, target)
}

// RequiredFor 判断字段是否在给定状态/目标下必填。
// Required 是活动字段的无条件必填；RequiredWhen 是额外的条件必填。
func (f FieldSchema) RequiredFor(state CurrentState, root map[string]any, target string) bool {
	if f.Required && f.Matches(state, root, target) {
		return true
	}
	if f.RequiredWhen == nil {
		return false
	}
	// 目标限定的必填规则只在具体目标校验时生效；保存节点本身不要求
	// 用户先选定全部输出目标。
	if target == "" && len(f.RequiredWhen.Targets) > 0 {
		return false
	}
	return f.RequiredWhen.Matches(state, root, target)
}

// ShouldReset 判断字段是否声明了指定的清空作用域。
func (f FieldSchema) ShouldReset(scope string) bool {
	for _, item := range f.ResetOn {
		if item == scope {
			return true
		}
	}
	return false
}

// Matches 判断状态是否满足条件规则。
func (r ConditionRule) Matches(state CurrentState, root map[string]any, target string) bool {
	if len(r.Network) > 0 && !containsAny(r.Network, []string{state.Network}) {
		return false
	}
	if len(r.Security) > 0 && !containsAny(r.Security, []string{state.Security}) {
		return false
	}
	plugin := ""
	if state.Plugin != nil {
		plugin = *state.Plugin
	}
	if len(r.Plugin) > 0 && !containsAny(r.Plugin, []string{plugin}) {
		return false
	}
	if len(r.PluginNot) > 0 && containsAny(r.PluginNot, []string{plugin}) {
		return false
	}
	if len(r.Features) > 0 && !containsAny(r.Features, state.Features) {
		return false
	}
	for name, allowed := range r.Selectors {
		if len(allowed) == 0 {
			continue
		}
		current := ""
		if state.Selectors != nil {
			current = state.Selectors[name]
		}
		if !containsAny(allowed, []string{current}) {
			return false
		}
	}
	if len(r.NonEmpty) > 0 {
		for _, path := range r.NonEmpty {
			value, ok := GetPath(root, path)
			if !ok || !hasEffectiveValue(value) {
				return false
			}
		}
	}
	if target != "" && len(r.Targets) > 0 && !containsAny(r.Targets, []string{target}) {
		return false
	}
	return true
}

func containsAny(needles, values []string) bool {
	for _, needle := range needles {
		for _, value := range values {
			if needle == value {
				return true
			}
		}
	}
	return false
}
