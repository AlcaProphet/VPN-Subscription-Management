package assembly

import (
	assemblylinks "vpn-sub/internal/assembly/links"
	"vpn-sub/internal/node"
)

// activeProtocolJSON 输出前按节点当前状态投影，避免 legacy/非激活分支进入实际产物。
func activeProtocolJSON(nd *nodeData) map[string]any {
	proto, err := node.GetProtocol(nd.Protocol)
	if err != nil {
		return nd.ProtocolJSON
	}
	state := nd.CurrentState
	if state.Network == "" && state.Security == "" && state.Plugin == nil && len(state.Features) == 0 {
		state = node.DeriveCurrentState(proto, nd.ProtocolJSON)
	}
	return node.ProjectActive(proto, state, nd.ProtocolJSON)
}

// srLink 生成 Shadowrocket 原生参数风格节点链接（委托共享子包）。
func srLink(nd *nodeData) (string, error) {
	return assemblylinks.Render(nd.Protocol, nd.RenderName, nd.Host, nd.Port, activeProtocolJSON(nd), false)
}

// genericLink 生成通用标准节点链接（委托共享子包）。
func genericLink(nd *nodeData) (string, error) {
	return assemblylinks.Render(nd.Protocol, nd.RenderName, nd.Host, nd.Port, activeProtocolJSON(nd), true)
}

// RenderLink 导出给下载渲染复用：按协议生成 SR 或通用标准链接。
func RenderLink(protocol, renderName, host string, port int, params map[string]any, generic bool) (string, error) {
	return assemblylinks.Render(protocol, renderName, host, port, params, generic)
}
