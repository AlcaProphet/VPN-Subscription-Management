// normalize.go：manual 表单创建与 URI 导入共用的协议参数归一化和当前状态初始化（Build20 Step 1）。
package node

// NormalizeProtocolJSON 将兼容输入别名规约到规范路径，供手动创建与 URI 导入共用。
// 无法归属的未知顶层字段不会被自动删除，仍交由后续 validateKnownTopLevel 显式处理。
func NormalizeProtocolJSON(proto Protocol, params map[string]any) (map[string]any, error) {
	if params == nil {
		params = map[string]any{}
	}
	out := normalizeProtocolParameters(proto, params)
	canonicalizeLegacyAliases(proto, out)
	return out, nil
}

func canonicalizeLegacyAliases(proto Protocol, params map[string]any) {
	switch proto.Protocol {
	case "vless", "vmess":
		canonicalizeWsAliases(params)
		canonicalizeGrpcServiceName(params)
	case "trojan":
		canonicalizeWsAliases(params)
		canonicalizeGrpcServiceName(params)
		canonicalizeTrojanInnerSS(params)
	case "ss":
		canonicalizeSSPluginOpts(params)
	}
	if proto.Protocol == "vless" {
		canonicalizeRealityAliases(params)
	}
}

// canonicalizeSSPluginOpts 把旧版/URI 导入的 plugin-opts 按插件收敛到独立对象。
func canonicalizeSSPluginOpts(params map[string]any) {
	plugin, _ := params["plugin"].(string)
	opts, ok := params["plugin-opts"].(map[string]any)
	if !ok {
		return
	}
	switch plugin {
	case "obfs":
		params["obfs-opts"] = cloneJSONValue(opts)
	case "v2ray-plugin":
		params["v2ray-plugin-opts"] = cloneJSONValue(opts)
	case "shadow-tls":
		params["shadow-tls-opts"] = cloneJSONValue(opts)
	case "restls":
		params["restls-opts"] = cloneJSONValue(opts)
	}
	delete(params, "plugin-opts")
}

func canonicalizeWsAliases(params map[string]any) {
	_, hasPath := params["ws-path"]
	_, hasHost := params["ws-host"]
	_, hasHeaders := params["ws-headers"]
	if !hasPath && !hasHost && !hasHeaders {
		return
	}
	opts := ensureObjectParam(params, "ws-opts")
	if _, exists := opts["path"]; !exists {
		if value, ok := params["ws-path"]; ok {
			opts["path"] = cloneJSONValue(value)
		}
	}
	if headers, ok := opts["headers"].(map[string]any); ok {
		if value, ok := params["ws-host"]; ok {
			if _, exists := headers["Host"]; !exists {
				headers["Host"] = cloneJSONValue(value)
			}
		}
		if value, ok := params["ws-headers"]; ok {
			if source, ok := value.(map[string]any); ok {
				for key, item := range source {
					if _, exists := headers[key]; !exists {
						headers[key] = cloneJSONValue(item)
					}
				}
			}
		}
	} else {
		headers := map[string]any{}
		if value, ok := params["ws-host"]; ok {
			headers["Host"] = cloneJSONValue(value)
		}
		if value, ok := params["ws-headers"]; ok {
			if source, ok := value.(map[string]any); ok {
				for key, item := range source {
					if _, exists := headers[key]; !exists {
						headers[key] = cloneJSONValue(item)
					}
				}
			}
		}
		if len(headers) > 0 {
			opts["headers"] = headers
		}
	}
	delete(params, "ws-path")
	delete(params, "ws-host")
	delete(params, "ws-headers")
}

func canonicalizeGrpcServiceName(params map[string]any) {
	if value, ok := params["grpc-service-name"]; ok {
		opts := ensureObjectParam(params, "grpc-opts")
		if _, exists := opts["grpc-service-name"]; !exists {
			opts["grpc-service-name"] = cloneJSONValue(value)
		}
		delete(params, "grpc-service-name")
	}
}

func canonicalizeRealityAliases(params map[string]any) {
	opts := ensureObjectParam(params, "reality-opts")
	if value, ok := params["public-key"]; ok {
		if _, exists := opts["public-key"]; !exists {
			opts["public-key"] = cloneJSONValue(value)
		}
		delete(params, "public-key")
	}
	if value, ok := params["short-id"]; ok {
		if _, exists := opts["short-id"]; !exists {
			opts["short-id"] = cloneJSONValue(value)
		}
		delete(params, "short-id")
	}
}

// canonicalizeTrojanInnerSS 把 Trojan 旧表单顶层 cipher 收敛到 ss-opts.method。
func canonicalizeTrojanInnerSS(params map[string]any) {
	if value, ok := params["cipher"]; ok {
		opts := ensureObjectParam(params, "ss-opts")
		if _, exists := opts["method"]; !exists {
			opts["method"] = cloneJSONValue(value)
		}
		delete(params, "cipher")
	}
}

// InitCurrentState 对协议参数初始化最小当前状态。
// 非首批协议只保留其 schema 中真实存在的稳定标识，不生成不存在于该协议的字段。
func InitCurrentState(proto Protocol, params map[string]any) CurrentState {
	state := DeriveCurrentState(proto, params)
	if !hasSchemaField(proto.FormSchema, "network") && !hasSchemaField(proto.FormSchema, "transport") {
		state.Network = ""
	}
	if !hasSchemaField(proto.FormSchema, "security") && !hasSchemaField(proto.FormSchema, "tls") && proto.Protocol != "trojan" {
		state.Security = ""
	}
	return state
}
