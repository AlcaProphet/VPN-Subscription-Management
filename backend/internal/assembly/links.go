package assembly

import (
	assemblylinks "vpn-sub/internal/assembly/links"
)

// srLink 生成 Shadowrocket 原生参数风格节点链接（委托共享子包）。
func srLink(nd *nodeData) (string, error) {
	return assemblylinks.Render(nd.Protocol, nd.RenderName, nd.Host, nd.Port, nd.ProtocolJSON, false)
}

// genericLink 生成通用标准节点链接（委托共享子包）。
func genericLink(nd *nodeData) (string, error) {
	return assemblylinks.Render(nd.Protocol, nd.RenderName, nd.Host, nd.Port, nd.ProtocolJSON, true)
}

// RenderLink 导出给下载渲染复用：按协议生成 SR 或通用标准链接。
func RenderLink(protocol, renderName, host string, port int, params map[string]any, generic bool) (string, error) {
	return assemblylinks.Render(protocol, renderName, host, port, params, generic)
}
