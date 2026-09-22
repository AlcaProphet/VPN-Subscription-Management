package node

import (
	"errors"
	"fmt"
	"strings"
)

// defaultEndpointPolicy 在协议未声明 EndpointPolicies 时保持既有普通 endpoint 语义。
var defaultEndpointPolicy = EndpointPolicy{
	HostMode: "required",
	PortMode: "required",
	EmitHost: true,
	EmitPort: true,
}

func effectiveEndpointPolicies(proto Protocol) []EndpointPolicy {
	if len(proto.EndpointPolicies) == 0 {
		return []EndpointPolicy{defaultEndpointPolicy}
	}
	return proto.EndpointPolicies
}

// validateProtocolEndpointPolicies 校验注册表 endpoint policy 的形状和 selector 条件引用。
func validateProtocolEndpointPolicies(proto Protocol) error {
	for _, policy := range proto.EndpointPolicies {
		if err := validateEndpointPolicy(proto, policy); err != nil {
			return err
		}
	}
	return nil
}

func validateEndpointPolicy(proto Protocol, policy EndpointPolicy) error {
	switch policy.HostMode {
	case "required", "optional", "hidden":
	default:
		return fmt.Errorf("host_mode 非法: %s", policy.HostMode)
	}
	switch policy.PortMode {
	case "required", "optional", "hidden":
	default:
		return fmt.Errorf("port_mode 非法: %s", policy.PortMode)
	}
	if policy.HostMode == "hidden" && policy.EmitHost {
		return errors.New("hidden host 不得输出")
	}
	if policy.PortMode == "hidden" && policy.EmitPort {
		return errors.New("hidden port 不得输出")
	}
	if policy.HostMode != "hidden" && !policy.EmitHost {
		return errors.New("非 hidden host 必须输出")
	}
	if policy.PortMode != "hidden" && !policy.EmitPort {
		return errors.New("非 hidden port 必须输出")
	}
	if policy.When == nil {
		return nil
	}
	for name, values := range policy.When.Selectors {
		selector, ok := selectorSchemaByName(proto, name)
		if !ok {
			return fmt.Errorf("endpoint policy 引用未声明 selector: %s", name)
		}
		for _, value := range values {
			if !selectorValueAllowed(selector, value) {
				return fmt.Errorf("endpoint policy selector %s 含未允许值: %s", name, value)
			}
		}
	}
	return nil
}

// MatchEndpointPolicy 按协议＋当前 selector 状态匹配唯一 endpoint policy。
func MatchEndpointPolicy(proto Protocol, state CurrentState) (EndpointPolicy, error) {
	policies := effectiveEndpointPolicies(proto)
	var matched []EndpointPolicy
	for _, policy := range policies {
		if policy.When != nil && !policy.When.Matches(state, "") {
			continue
		}
		matched = append(matched, policy)
	}
	if len(matched) != 1 {
		return EndpointPolicy{}, fmt.Errorf("协议 %s 的 endpoint policy 命中 %d 条，必须且只能命中一条", proto.Protocol, len(matched))
	}
	return matched[0], nil
}

// NormalizeEndpoint 按 endpoint policy 规范化 host/port：
// hidden 统一为 ”/0；required 要求 host 非空、port 1-65535；optional 允许空。
func NormalizeEndpoint(proto Protocol, state CurrentState, host string, port int) (string, int, EndpointPolicy, error) {
	policy, err := MatchEndpointPolicy(proto, state)
	if err != nil {
		return "", 0, EndpointPolicy{}, err
	}
	if policy.HostMode == "hidden" {
		host = ""
	} else if policy.HostMode == "required" && strings.TrimSpace(host) == "" {
		return "", 0, EndpointPolicy{}, errors.New("服务器地址不能为空")
	}
	if policy.PortMode == "hidden" {
		port = 0
	} else if policy.PortMode == "required" {
		if port < 1 || port > 65535 {
			return "", 0, EndpointPolicy{}, errors.New("端口须在 1-65535 之间")
		}
	} else if policy.PortMode == "optional" && port != 0 && (port < 1 || port > 65535) {
		return "", 0, EndpointPolicy{}, errors.New("端口须在 0 或 1-65535 之间")
	}
	return host, port, policy, nil
}

// ValidateEndpoint 只校验 endpoint policy，不返回规范化结果。
func ValidateEndpoint(proto Protocol, state CurrentState, host string, port int) error {
	_, _, _, err := NormalizeEndpoint(proto, state, host, port)
	return err
}
