// Package ssplugin 提供 Shadowsocks 插件的固定合同，供节点 schema、输出与诊断共用。
package ssplugin

// 固定输出目标名称与平台 target_syntax 保持一致。
const (
	TargetClash        = "clash-yaml"
	TargetShadowrocket = "sr-subs"
	TargetGeneric      = "generic-subs"
)

// SupportLevel 描述插件参数在目标上的固定证据等级。
type SupportLevel string

const (
	SupportComplete    SupportLevel = "complete"
	SupportPartial     SupportLevel = "partial"
	SupportUnverified  SupportLevel = "unverified"
	SupportUnsupported SupportLevel = "unsupported"
)

// TargetContract 描述一个插件在单一输出目标上的参数合同。
type TargetContract struct {
	Support           SupportLevel
	Defaults          map[string]string
	RequiredFields    []string
	ExpressibleFields []string
}

// Definition 描述一个已知插件的内部存储键与目标合同。
type Definition struct {
	Name       string
	StorageKey string
	Targets    map[string]TargetContract
}

// Target 返回目标合同的副本，避免调用方修改全局定义。
func (d Definition) Target(target string) (TargetContract, bool) {
	contract, ok := d.Targets[target]
	if !ok {
		return TargetContract{}, false
	}
	return cloneTarget(contract), true
}

var definitions = []Definition{
	{
		Name: "obfs", StorageKey: "obfs-opts",
		Targets: map[string]TargetContract{
			TargetClash:        target(SupportComplete, []string{"mode"}, map[string]string{"mode": "http"}, "mode", "host"),
			TargetShadowrocket: target(SupportPartial, nil, nil, "mode", "host"),
			TargetGeneric:      target(SupportPartial, nil, nil, "mode", "host"),
		},
	},
	{
		Name: "v2ray-plugin", StorageKey: "v2ray-plugin-opts",
		Targets: map[string]TargetContract{
			TargetClash: target(SupportComplete, []string{"mode"}, map[string]string{"mode": "websocket"},
				"mode", "host", "path", "headers", "tls", "mux", "v2ray-http-upgrade", "v2ray-http-upgrade-fast-open", "fingerprint", "certificate", "private-key", "name-cert-verify"),
			TargetShadowrocket: target(SupportPartial, nil, nil, "mode", "host", "path", "tls"),
			TargetGeneric:      target(SupportPartial, nil, nil, "mode", "host", "path", "tls"),
		},
	},
	{
		Name: "shadow-tls", StorageKey: "shadow-tls-opts",
		Targets: map[string]TargetContract{
			TargetClash: target(SupportComplete, []string{"host"}, nil,
				"host", "password", "version", "alpn", "fingerprint", "certificate", "private-key", "skip-cert-verify"),
			TargetShadowrocket: target(SupportUnverified, nil, nil,
				"host", "password", "version", "alpn", "fingerprint", "certificate", "private-key", "skip-cert-verify"),
			TargetGeneric: target(SupportUnsupported, nil, nil),
		},
	},
	{
		Name: "restls", StorageKey: "restls-opts",
		Targets: map[string]TargetContract{
			TargetClash: target(SupportComplete, []string{"password", "host", "version-hint"}, nil,
				"password", "host", "version-hint", "restls-script", "fingerprint", "skip-cert-verify"),
			TargetShadowrocket: target(SupportUnverified, nil, nil,
				"password", "host", "version-hint", "restls-script", "fingerprint", "skip-cert-verify"),
			TargetGeneric: target(SupportUnsupported, nil, nil),
		},
	},
}

func target(support SupportLevel, required []string, defaults map[string]string, expressible ...string) TargetContract {
	return TargetContract{
		Support: support, RequiredFields: required, Defaults: defaults, ExpressibleFields: expressible,
	}
}

// KnownNames 按稳定的表单顺序返回四个已知插件名。
func KnownNames() []string {
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	return names
}

// Lookup 返回已知插件合同的副本；未知插件由调用方使用普通字符串映射合同处理。
func Lookup(name string) (Definition, bool) {
	for _, definition := range definitions {
		if definition.Name == name {
			return cloneDefinition(definition), true
		}
	}
	return Definition{}, false
}

func cloneDefinition(definition Definition) Definition {
	out := Definition{Name: definition.Name, StorageKey: definition.StorageKey, Targets: make(map[string]TargetContract, len(definition.Targets))}
	for name, contract := range definition.Targets {
		out.Targets[name] = cloneTarget(contract)
	}
	return out
}

func cloneTarget(contract TargetContract) TargetContract {
	out := TargetContract{
		Support:           contract.Support,
		RequiredFields:    append([]string(nil), contract.RequiredFields...),
		ExpressibleFields: append([]string(nil), contract.ExpressibleFields...),
	}
	if contract.Defaults != nil {
		out.Defaults = make(map[string]string, len(contract.Defaults))
		for key, value := range contract.Defaults {
			out.Defaults[key] = value
		}
	}
	return out
}
