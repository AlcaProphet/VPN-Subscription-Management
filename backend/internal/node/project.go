package node

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

// ProjectActive 按当前状态投影实际活动的协议参数。
// security 是表单层的统一状态字段，VLESS/VMess 输出时转换回既有 tls 语义；
// 未激活的已知分支不会进入结果，未知对象键在其所属活动对象内保留。
func ProjectActive(proto Protocol, state CurrentState, params map[string]any) map[string]any {
	params = normalizeProtocolParameters(proto, params)
	params = cleanDisabledFeatures(proto.FormSchema, params)
	// 旧状态没有嵌套功能标识时，从实际控制值派生；关闭父功能不会复活子功能。
	state.Features = activeFeatures(proto.FormSchema, params)
	out := make(map[string]any, len(params))
	for _, field := range proto.FormSchema {
		if field.StateOnly || !field.Matches(state, "") {
			continue
		}
		value, ok := params[field.Name]
		if !ok {
			continue
		}
		projected, ok := projectFieldValue(field, value, state)
		if ok {
			out[field.Name] = projected
		}
	}
	if proto.Protocol == "vless" || proto.Protocol == "vmess" {
		delete(out, "security")
		if state.Security == "tls" || state.Security == "reality" {
			out["tls"] = true
		}
	}
	return out
}

func projectFieldValue(field FieldSchema, value any, state CurrentState) (any, bool) {
	if !hasEffectiveValue(value) {
		return nil, false
	}
	if field.Type != "object" {
		return cloneProjectValue(value), true
	}

	switch field.ObjectKind {
	case "fields":
		object, ok := value.(map[string]any)
		if !ok {
			return cloneProjectValue(value), true
		}
		return projectObjectFields(field, object, state)
	case "map":
		object, ok := value.(map[string]any)
		if !ok {
			return cloneProjectValue(value), true
		}
		out := make(map[string]any, len(object))
		for key, item := range object {
			if field.MapValueType == "string" {
				if _, ok := item.(string); ok {
					out[key] = cloneProjectValue(item)
				}
				continue
			}
			if hasEffectiveValue(item) {
				out[key] = cloneProjectValue(item)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	case "list":
		items, ok := value.([]any)
		if !ok {
			return cloneProjectValue(value), true
		}
		out := make([]any, 0, len(items))
		for _, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				if hasEffectiveValue(item) {
					out = append(out, cloneProjectValue(item))
				}
				continue
			}
			projected, ok := projectObjectFields(field, object, state)
			if ok {
				out = append(out, projected)
			}
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	default:
		return cloneProjectValue(value), true
	}
}

func projectObjectFields(field FieldSchema, object map[string]any, state CurrentState) (map[string]any, bool) {
	out := make(map[string]any, len(object))
	known := make(map[string]bool, len(field.Properties)+1)
	// 稳定条目身份是内部编辑元数据，需要保留到脱敏步骤之后再剥离，不能作为未知业务键丢弃。
	if field.ItemIDField != "" {
		known[field.ItemIDField] = true
		if value, exists := object[field.ItemIDField]; exists {
			out[field.ItemIDField] = cloneJSONValue(value)
		}
	}
	for _, property := range field.Properties {
		known[property.Name] = true
		if !property.Matches(state, "") {
			continue
		}
		value, ok := object[property.Name]
		if !ok {
			continue
		}
		projected, ok := projectFieldValue(property, value, state)
		if ok {
			out[property.Name] = projected
		}
	}
	if field.AllowUnknown {
		for key, value := range object {
			if known[key] || !hasEffectiveValue(value) {
				continue
			}
			out[key] = cloneProjectValue(value)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// ValidateCurrentState 校验当前状态、活动参数和首批明确非法组合。
// 目标限定的 required_when 不在保存节点时执行，具体目标门槛由检查/输出阶段处理。
func ValidateCurrentState(proto Protocol, state CurrentState, params map[string]any) error {
	return ValidateCurrentStateForTarget(proto, state, params, "")
}

// ValidateCurrentStateForTarget 在节点检查阶段额外执行目标限定的条件必填。
// target 为空表示保存节点本身，不要求用户预先选择某个输出目标。
func ValidateCurrentStateForTarget(proto Protocol, state CurrentState, params map[string]any, target string) error {
	if err := validateStateOnlyParams(proto, params); err != nil {
		return err
	}
	params = normalizeProtocolParameters(proto, params)
	derived := DeriveCurrentState(proto, params)
	if stateEmpty(state) {
		state = derived
	}
	hasNetwork := hasSchemaField(proto.FormSchema, "network") || hasSchemaField(proto.FormSchema, "transport")
	if hasNetwork && state.Network != derived.Network {
		return fmt.Errorf("current_state.network 与 protocol_json.network 不一致")
	}
	hasSecurity := hasSchemaField(proto.FormSchema, "security") || hasSchemaField(proto.FormSchema, "tls") || proto.Protocol == "trojan"
	if hasSecurity && state.Security != derived.Security {
		return fmt.Errorf("current_state.security 与 protocol_json 安全参数不一致")
	}
	if !samePlugin(state.Plugin, derived.Plugin) {
		return fmt.Errorf("current_state.plugin 与 protocol_json.plugin 不一致")
	}
	if !sameStringSet(state.Features, derived.Features) {
		return fmt.Errorf("current_state.features 与 protocol_json 功能开关不一致")
	}
	if err := validateStateOptions(proto, state); err != nil {
		return err
	}
	if err := validateSelectors(proto, state, params); err != nil {
		return err
	}
	if err := validateActiveFields(proto.FormSchema, state, params, "", target); err != nil {
		return err
	}
	if err := validateProtocolCombination(proto, state, params); err != nil {
		return err
	}
	return nil
}

func stateEmpty(state CurrentState) bool {
	return state.Network == "" && state.Security == "" && state.Plugin == nil && len(state.Features) == 0 && len(state.Selectors) == 0
}

func samePlugin(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validateStateOptions(proto Protocol, state CurrentState) error {
	if state.Network != "" {
		if field, ok := findSchemaField(proto.FormSchema, "network"); ok {
			if err := validateOption(field, state.Network, "network"); err != nil {
				return err
			}
		}
	}
	security, ok := findSchemaField(proto.FormSchema, "security")
	if ok && state.Security != "" {
		if err := validateOption(security, state.Security, "security"); err != nil {
			return err
		}
	}
	return nil
}

func findSchemaField(fields []FieldSchema, name string) (FieldSchema, bool) {
	for _, field := range fields {
		if field.Name == name {
			return field, true
		}
	}
	return FieldSchema{}, false
}

func validateActiveFields(fields []FieldSchema, state CurrentState, params map[string]any, prefix, target string) error {
	for _, field := range fields {
		if field.StateOnly || !field.Matches(state, target) {
			continue
		}
		path := field.Name
		if prefix != "" {
			path = prefix + "." + path
		}
		value, exists := params[field.Name]
		if field.RequiredFor(state, target) && (!exists || !hasEffectiveValue(value)) {
			return fmt.Errorf("字段 %s 必填", path)
		}
		if !exists || !hasEffectiveValue(value) {
			continue
		}
		if err := validateActiveFieldValue(field, value, state, path, target); err != nil {
			return err
		}
	}
	return nil
}

func validateActiveFieldValue(field FieldSchema, value any, state CurrentState, path, target string) error {
	if field.Type != "object" {
		if err := validateFieldValue(field, value, path); err != nil {
			return err
		}
		return validateOption(field, value, path)
	}
	switch field.ObjectKind {
	case "fields":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("字段 %s 类型应为 object", path)
		}
		return validateActiveObjectFields(field, object, state, path, target)
	case "map":
		return validateFieldValue(field, value, path)
	case "list":
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("字段 %s 类型应为 object list", path)
		}
		for i, item := range items {
			object, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("字段 %s[%d] 类型应为 object", path, i)
			}
			if err := validateActiveObjectFields(field, object, state, fmt.Sprintf("%s[%d]", path, i), target); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func validateActiveObjectFields(field FieldSchema, object map[string]any, state CurrentState, path, target string) error {
	known := make(map[string]bool, len(field.Properties)+1)
	if field.ItemIDField != "" {
		known[field.ItemIDField] = true
	}
	for _, property := range field.Properties {
		known[property.Name] = true
		if !property.Matches(state, target) {
			continue
		}
		value, exists := object[property.Name]
		propertyPath := path + "." + property.Name
		if property.RequiredFor(state, target) && (!exists || !hasEffectiveValue(value)) {
			return fmt.Errorf("字段 %s 必填", propertyPath)
		}
		if !exists || !hasEffectiveValue(value) {
			continue
		}
		if err := validateActiveFieldValue(property, value, state, propertyPath, target); err != nil {
			return err
		}
	}
	if !field.AllowUnknown {
		for key := range object {
			if !known[key] {
				return fmt.Errorf("字段 %s.%s 未在协议注册表中声明", path, key)
			}
		}
	}
	return nil
}

func validateOption(field FieldSchema, value any, path string) error {
	if len(field.OptionItems) == 0 && len(field.Options) == 0 {
		return nil
	}
	text, ok := value.(string)
	if !ok {
		return nil
	}
	allowed := false
	if len(field.OptionItems) > 0 {
		for _, item := range field.OptionItems {
			if item.Value == text {
				allowed = true
				break
			}
		}
	} else {
		for _, item := range field.Options {
			if item == text {
				allowed = true
				break
			}
		}
	}
	if !allowed && !allowsCustom(field) {
		return fmt.Errorf("字段 %s 不允许值 %q", path, text)
	}
	return nil
}

func allowsCustom(field FieldSchema) bool {
	return field.AllowCustom != nil && *field.AllowCustom
}

func validateProtocolCombination(proto Protocol, state CurrentState, params map[string]any) error {
	switch proto.Protocol {
	case "http":
		if err := validateTLSKeyPair(params); err != nil {
			return err
		}
		if err := validateHTTPHeaders(params); err != nil {
			return err
		}
	case "socks5":
		if err := validateTLSKeyPair(params); err != nil {
			return err
		}
	case "ssh":
		if err := validateSSHPrivateKey(params); err != nil {
			return err
		}
		if err := validateSSHHostKeys(params); err != nil {
			return err
		}
	case "snell":
		if err := validateSnellCombination(params); err != nil {
			return err
		}
	case "hysteria":
		if err := validateHysteriaCombination(params); err != nil {
			return err
		}
	case "hysteria2":
		if err := validateHysteria2Combination(state, params); err != nil {
			return err
		}
	case "tuic":
		if err := validateTUICCombination(state, params); err != nil {
			return err
		}
	case "vless":
		if state.Network == "xhttp" {
			xhttp := objectValue(params, "xhttp-opts")
			if mode, ok := xhttp["mode"].(string); ok && mode == "none" {
				return errorsForField("xhttp-opts.mode", "XHTTP mode 不能使用 none 作为未指定")
			}
		}
		if state.Security == "reality" && !configuredObject(params["reality-opts"]) {
			return errorsForField("reality-opts", "REALITY 安全模式缺少 reality-opts")
		}
		if security, ok := params["security"].(string); ok {
			if security == "none" && (boolValue(params["tls"]) || configuredObject(params["reality-opts"])) {
				return errorsForField("security", "security=none 不能同时启用 TLS 或 REALITY")
			}
			if security == "tls" && configuredObject(params["reality-opts"]) {
				return errorsForField("security", "security=tls 不能同时配置 REALITY 参数")
			}
		}
	case "vmess":
		if state.Security == "reality" || configuredObject(params["reality-opts"]) {
			return errorsForField("reality-opts", "VMess 首批不开放 REALITY 表单")
		}
	case "ss":
		if cipher, _ := params["cipher"].(string); cipher == "auto" {
			return errorsForField("cipher", "Shadowsocks 不支持 auto 算法")
		}
	case "trojan":
		// h2/http/xhttp 等自定义传输由目标检查/装配阶段诊断，不在此阻止保存。
		if opts := objectValue(params, "ss-opts"); boolValue(opts["enabled"]) {
			if strings.TrimSpace(stringValue(opts["method"])) == "" {
				return errorsForField("ss-opts.method", "启用内层 SS 时必须填写 method")
			}
			if strings.TrimSpace(stringValue(opts["password"])) == "" {
				return errorsForField("ss-opts.password", "启用内层 SS 时必须填写密码")
			}
		}
	}
	return nil
}

func errorsForField(path, message string) error {
	return fmt.Errorf("字段 %s: %s", path, message)
}

// bandwidthPattern 与固定 tag 的 utils.StringToBps 保持一致；纯整数按 Mbps 处理。
var bandwidthPattern = regexp.MustCompile(`^(\d+)\s*([KMGT]?)([Bb])ps$`)

// validBandwidth 判断 up／down 是否为内核可接受的非零带宽字符串。
func validBandwidth(value string) bool {
	text := strings.TrimSpace(value)
	if text == "" {
		return false
	}
	if amount, err := strconv.Atoi(text); err == nil {
		return amount > 0
	}
	matched := bandwidthPattern.FindStringSubmatch(text)
	if matched == nil {
		return false
	}
	amount, err := strconv.Atoi(matched[1])
	return err == nil && amount > 0
}

// validHysteriaPorts 校验端口跳跃语法：逗号分隔的单端口或 begin-end 范围，均为 1-65535。
func validHysteriaPorts(value string) bool {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return false
		}
		if bounds := strings.Split(part, "-"); len(bounds) == 2 {
			start, errStart := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, errEnd := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if errStart != nil || errEnd != nil || start < 1 || end > 65535 || start > end {
				return false
			}
			continue
		} else if len(bounds) > 2 {
			return false
		}
		port, err := strconv.Atoi(part)
		if err != nil || port < 1 || port > 65535 {
			return false
		}
	}
	return true
}

// validateIntegerMinimum 校验已声明为 number 的活动字段必须是整数且不低于下限。
func validateIntegerMinimum(params map[string]any, name string, minimum float64, message string) error {
	value, ok := numberParam(params[name])
	if !ok {
		return nil
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value || value < minimum {
		return errorsForField(name, message)
	}
	return nil
}

// validateHysteriaCombination 校验 Hysteria v1 的带宽、认证、端口跳跃、mTLS 与接收窗口关系。
func validateHysteriaCombination(params map[string]any) error {
	for _, name := range []string{"up", "down"} {
		if !validBandwidth(stringValue(params[name])) {
			return errorsForField(name, "带宽必须是非零的速率字符串（例如 100 Mbps）")
		}
	}
	if auth := strings.TrimSpace(stringValue(params["auth"])); auth != "" {
		if _, err := base64.StdEncoding.DecodeString(auth); err != nil {
			return errorsForField("auth", "认证必须是合法的 Base64 字符串")
		}
	}
	if ports := strings.TrimSpace(stringValue(params["ports"])); ports != "" {
		if !validHysteriaPorts(ports) {
			return errorsForField("ports", "端口跳跃只接受 1-65535 的单端口或 begin-end 范围")
		}
	}
	if err := validateTLSKeyPair(params); err != nil {
		return err
	}
	if err := validateIntegerMinimum(params, "hop-interval", 0, "Hop 间隔必须是非负整数"); err != nil {
		return err
	}
	return validateReceiveWindows(params)
}

// validateReceiveWindows 校验接收窗口非负且连接窗口不小于流窗口。
func validateReceiveWindows(params map[string]any) error {
	for _, name := range []string{"recv-window-conn", "recv-window"} {
		if err := validateIntegerMinimum(params, name, 0, "接收窗口必须是非负整数"); err != nil {
			return err
		}
	}
	stream, hasStream := numberParam(params["recv-window-conn"])
	connection, hasConnection := numberParam(params["recv-window"])
	if hasStream && hasConnection && stream > connection {
		return errorsForField("recv-window-conn", "连接接收窗口不能小于流接收窗口 recv-window")
	}
	return nil
}

// tuicDatagramFrameLimit 是固定 tag 对 max-datagram-frame-size 的硬上限。
const tuicDatagramFrameLimit = 1400

// validateTUICCombination 校验 TUIC 的 v5 UUID、直连 IP、非负数值、mTLS 与数据报／中继包联动。
func validateTUICCombination(state CurrentState, params map[string]any) error {
	if state.Selectors["auth_mode"] == "v5" {
		if _, err := uuid.Parse(strings.TrimSpace(stringValue(params["uuid"]))); err != nil {
			return errorsForField("uuid", "UUID 必须是合法的 UUID")
		}
	}
	if ip := strings.TrimSpace(stringValue(params["ip"])); ip != "" && net.ParseIP(ip) == nil {
		return errorsForField("ip", "IP 必须是合法地址")
	}
	for _, name := range []string{"heartbeat-interval", "request-timeout", "max-udp-relay-packet-size",
		"max-open-streams", "cwnd", "recv-window-conn", "recv-window", "max-datagram-frame-size"} {
		if value, ok := numberParam(params[name]); ok && value < 0 {
			return errorsForField(name, "该字段不能为负数")
		}
	}
	if err := validateTLSKeyPair(params); err != nil {
		return err
	}
	datagram, hasDatagram := numberParam(params["max-datagram-frame-size"])
	if hasDatagram && datagram > tuicDatagramFrameLimit {
		return errorsForField("max-datagram-frame-size", "最大数据报帧不能超过内核上限 1400")
	}
	if relay, hasRelay := numberParam(params["max-udp-relay-packet-size"]); hasDatagram && hasRelay && relay > datagram {
		return errorsForField("max-udp-relay-packet-size", "最大 UDP 中继包不能超过最大数据报帧")
	}
	return nil
}

// validateHysteria2Combination 校验 Hysteria2 的 mTLS、混淆、QUIC 窗口、高级整数与 Realm 子树。
func validateHysteria2Combination(state CurrentState, params map[string]any) error {
	if err := validateTLSKeyPair(params); err != nil {
		return err
	}
	for _, name := range []string{"up", "down"} {
		if value := strings.TrimSpace(stringValue(params[name])); value != "" && !validBandwidth(value) {
			return errorsForField(name, "带宽必须是非零的速率字符串（例如 100 Mbps）")
		}
	}
	for _, name := range []string{"cwnd", "udp-mtu", "handshake-timeout"} {
		if err := validateIntegerMinimum(params, name, 1, "该字段必须是正整数或未设置"); err != nil {
			return err
		}
	}
	for _, name := range []string{
		"initial-stream-receive-window", "max-stream-receive-window",
		"initial-connection-receive-window", "max-connection-receive-window"} {
		if err := validateIntegerMinimum(params, name, 0, "该字段必须是非负整数"); err != nil {
			return err
		}
	}
	if ports := strings.TrimSpace(stringValue(params["ports"])); ports != "" && !validHysteriaPorts(ports) {
		return errorsForField("ports", "端口组只接受 1-65535 的单端口或 begin-end 范围")
	}
	if interval := strings.TrimSpace(stringValue(params["hop-interval"])); interval != "" && !validHopInterval(interval) {
		return errorsForField("hop-interval", "Hop 间隔只接受单值或单范围，且最小不低于 5 秒")
	}
	if state.Selectors["obfs_mode"] == "gecko" {
		minSize, hasMin := numberParam(params["obfs-min-packet-size"])
		maxSize, hasMax := numberParam(params["obfs-max-packet-size"])
		if hasMin && minSize <= 0 {
			return errorsForField("obfs-min-packet-size", "混淆包大小必须为正数")
		}
		if hasMax && maxSize <= 0 {
			return errorsForField("obfs-max-packet-size", "混淆包大小必须为正数")
		}
		if hasMin && hasMax && minSize > maxSize {
			return errorsForField("obfs-min-packet-size", "混淆最小包不能大于混淆最大包")
		}
	}
	if initial, ok := numberParam(params["initial-stream-receive-window"]); ok {
		if maximum, ok := numberParam(params["max-stream-receive-window"]); ok && initial > maximum {
			return errorsForField("initial-stream-receive-window", "初始流接收窗口不能大于最大流接收窗口")
		}
	}
	if initial, ok := numberParam(params["initial-connection-receive-window"]); ok {
		if maximum, ok := numberParam(params["max-connection-receive-window"]); ok && initial > maximum {
			return errorsForField("initial-connection-receive-window", "初始连接接收窗口不能大于最大连接接收窗口")
		}
	}
	return validateHysteria2Realm(params)
}

// validHopInterval 校验 Hop 间隔单值或单范围，且最小值不低于固定 tag 的 5 秒下限。
func validHopInterval(value string) bool {
	bounds := strings.Split(strings.TrimSpace(value), "-")
	if len(bounds) > 2 {
		return false
	}
	start, err := strconv.Atoi(strings.TrimSpace(bounds[0]))
	if err != nil {
		return false
	}
	end := start
	if len(bounds) == 2 {
		end, err = strconv.Atoi(strings.TrimSpace(bounds[1]))
		if err != nil {
			return false
		}
	}
	return start >= 5 && end >= start
}

// validateHysteria2Realm 校验启用的 Realm 子树：绝对服务地址、STUN 主机端口与证书成对。
func validateHysteria2Realm(params map[string]any) error {
	options, ok := params["realm-opts"].(map[string]any)
	if !ok || !boolValue(options["enable"]) {
		return nil
	}
	serverURL := strings.TrimSpace(stringValue(options["server-url"]))
	if serverURL == "" {
		return errorsForField("realm-opts.server-url", "启用 Realm 时必须填写 Realm 服务地址")
	}
	parsed, err := url.Parse(serverURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return errorsForField("realm-opts.server-url", "Realm 服务地址必须是绝对的 HTTP(S) URL")
	}
	if servers, ok := stringListItems(options["stun-servers"]); ok {
		for i, item := range servers {
			host, port, err := net.SplitHostPort(strings.TrimSpace(item))
			if err != nil || host == "" || port == "" {
				return errorsForField(fmt.Sprintf("realm-opts.stun-servers[%d]", i), "STUN 服务器必须是 host:port")
			}
		}
	}
	hasCertificate := strings.TrimSpace(stringValue(options["certificate"])) != ""
	hasPrivateKey := strings.TrimSpace(stringValue(options["private-key"])) != ""
	if hasCertificate == hasPrivateKey {
		return nil
	}
	if hasCertificate {
		return errorsForField("realm-opts.private-key", "Realm 使用客户端证书时必须同时提供私钥")
	}
	return errorsForField("realm-opts.certificate", "Realm 使用客户端私钥时必须同时提供证书")
}

// validateSnellCombination 校验 Snell 版本与 UDP 的互斥关系，以及 shadow-tls 证书／私钥成对。
// 条件必填（host／password／version-hint／username）由 schema 的 required_when 保证。
func validateSnellCombination(params map[string]any) error {
	version := strings.TrimSpace(stringValue(params["version"]))
	if (version == "1" || version == "2") && boolValue(params["udp"]) {
		return errorsForField("udp", "Snell v"+version+" 不支持 UDP")
	}
	options, ok := params["obfs-opts"].(map[string]any)
	if !ok {
		return nil
	}
	hasCertificate := strings.TrimSpace(stringValue(options["certificate"])) != ""
	hasPrivateKey := strings.TrimSpace(stringValue(options["private-key"])) != ""
	if hasCertificate == hasPrivateKey {
		return nil
	}
	if hasCertificate {
		return errorsForField("obfs-opts.private-key", "使用客户端证书时必须同时提供私钥")
	}
	return errorsForField("obfs-opts.certificate", "使用客户端私钥时必须同时提供证书")
}

// validateSSHPrivateKey 要求 private-key 只接受可解析的 PEM 内容。
// 使用与固定内核一致的 ssh 解析语义，避免把普通文本当作主机文件路径交给 Mihomo 读取。
func validateSSHPrivateKey(params map[string]any) error {
	key := strings.TrimSpace(stringValue(params["private-key"]))
	if key == "" {
		return nil
	}
	if !strings.Contains(key, "PRIVATE KEY") {
		return errorsForField("private-key", "私钥必须是 PEM 内容，不能是主机文件路径或普通文本")
	}
	passphrase := strings.TrimSpace(stringValue(params["private-key-passphrase"]))
	if passphrase != "" {
		if _, err := ssh.ParsePrivateKeyWithPassphrase([]byte(key), []byte(passphrase)); err != nil {
			return errorsForField("private-key-passphrase", "私钥口令无法解开该私钥")
		}
		return nil
	}
	if _, err := ssh.ParsePrivateKey([]byte(key)); err != nil {
		var missing *ssh.PassphraseMissingError
		if errors.As(err, &missing) {
			return errorsForField("private-key-passphrase", "私钥已加密，必须提供私钥口令")
		}
		return errorsForField("private-key", "私钥不是合法的 PEM 私钥内容")
	}
	return nil
}

// validateSSHHostKeys 按 authorized-key 语法逐项校验 Host Key，并拒绝空算法名。
// 列表的去空白与去重已在 NormalizeProtocolJSON 完成，此处只做语义校验。
func validateSSHHostKeys(params map[string]any) error {
	if hostKeys, ok := stringListItems(params["host-key"]); ok {
		for i, item := range hostKeys {
			key := strings.TrimSpace(item)
			if key == "" {
				return errorsForField(fmt.Sprintf("host-key[%d]", i), "Host Key 不能为空")
			}
			if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key)); err != nil {
				return errorsForField(fmt.Sprintf("host-key[%d]", i), "Host Key 必须符合 authorized-key 语法")
			}
		}
	}
	if algorithms, ok := stringListItems(params["host-key-algorithms"]); ok {
		for i, item := range algorithms {
			if strings.TrimSpace(item) == "" {
				return errorsForField(fmt.Sprintf("host-key-algorithms[%d]", i), "Host Key 算法不能为空")
			}
		}
	}
	return nil
}

// validateTLSKeyPair 要求客户端证书与私钥同时提供或同时为空（HTTP／SOCKS5 mTLS 成对）。
func validateTLSKeyPair(params map[string]any) error {
	certificate := hasTextParam(params, "certificate")
	privateKey := hasTextParam(params, "private-key")
	if certificate == privateKey {
		return nil
	}
	if certificate {
		return errorsForField("private-key", "使用客户端证书时必须同时提供私钥")
	}
	return errorsForField("certificate", "使用客户端私钥时必须同时提供证书")
}

// validateHTTPHeaders 执行请求头键值合同，并禁止覆盖 adapter 固定的代理认证内部处理。
func validateHTTPHeaders(params map[string]any) error {
	headers, ok := params["headers"].(map[string]any)
	if !ok {
		return nil
	}
	seen := make(map[string]string, len(headers))
	for key, value := range headers {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			return errorsForField("headers", "请求头键不能为空")
		}
		if _, ok := value.(string); !ok {
			return errorsForField("headers."+trimmed, "请求头值必须为字符串")
		}
		lower := strings.ToLower(trimmed)
		if lower == "proxy-authorization" {
			return errorsForField("headers."+trimmed, "不得覆盖代理认证头 Proxy-Authorization")
		}
		if previous, exists := seen[lower]; exists {
			return errorsForField("headers."+trimmed, "请求头键 "+previous+" 与 "+trimmed+" 大小写不敏感重复")
		}
		seen[lower] = trimmed
	}
	return nil
}

// clearSelectorScopedFields 删除与已解析 selector 状态不匹配的声明字段（含子对象）。
// 只处理声明了 selector 条件的字段，用于分支切换后旧分支参数不落库、不进入输出。
func clearSelectorScopedFields(proto Protocol, state CurrentState, params map[string]any) map[string]any {
	if len(proto.Selectors) == 0 || len(params) == 0 {
		return params
	}
	out := cloneJSONMap(params)
	var walk func([]FieldSchema, map[string]any)
	walk = func(fields []FieldSchema, object map[string]any) {
		for _, field := range fields {
			if field.StateOnly {
				continue
			}
			if field.When != nil && len(field.When.Selectors) > 0 && !field.When.Matches(state, "") {
				delete(object, field.Name)
				continue
			}
			if field.Type != "object" {
				continue
			}
			if value, ok := object[field.Name].(map[string]any); ok {
				walk(field.Properties, value)
			}
		}
	}
	walk(proto.FormSchema, out)
	return out
}

func objectValue(params map[string]any, key string) map[string]any {
	value, _ := params[key].(map[string]any)
	return value
}

func boolValue(value any) bool {
	result, _ := value.(bool)
	return result
}

func stringValue(value any) string {
	result, _ := value.(string)
	return result
}

func normalizeProtocolParameters(proto Protocol, params map[string]any) map[string]any {
	out := cloneJSONMap(params)
	if proto.Protocol == "trojan" {
		if opts, ok := out["ss-opts"].(map[string]any); ok {
			if _, hasMethod := opts["method"]; !hasMethod {
				if cipher, hasCipher := opts["cipher"]; hasCipher {
					opts["method"] = cloneJSONValue(cipher)
				}
			}
			delete(opts, "cipher")
		}
	}
	canonicalizeTransportAliases(proto, out)
	// 保存、读取、检查和输出共用 WS 别名归一化，规范值优先，旧值只补缺失项。
	switch proto.Protocol {
	case "vless", "vmess", "trojan":
		canonicalizeWsAliases(out)
	}
	if proto.Protocol == "ss" {
		canonicalizeSSPluginOpts(out)
	}
	return out
}

// canonicalizeTransportAliases 将 URI 导入或历史请求中的顶层传输别名
// 收敛到协议注册表的规范对象路径，避免同一参数以两套表达落库。
func canonicalizeTransportAliases(proto Protocol, params map[string]any) {
	network, _ := params["network"].(string)
	if network == "" {
		if transport, ok := params["transport"].(string); ok {
			network = transport
		}
	}
	switch proto.Protocol {
	case "vless", "vmess":
		switch network {
		case "ws":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "ws-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["headers"]; !ok {
					if host, exists := params["host"]; exists {
						opts["headers"] = map[string]any{"Host": cloneJSONValue(host)}
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "grpc":
			if hasTransportAlias(params, "path", "serviceName") {
				opts := ensureObjectParam(params, "grpc-opts")
				if _, ok := opts["grpc-service-name"]; !ok {
					if path, exists := params["path"]; exists {
						opts["grpc-service-name"] = cloneJSONValue(path)
					} else if serviceName, exists := params["serviceName"]; exists {
						opts["grpc-service-name"] = cloneJSONValue(serviceName)
					}
				}
			}
			delete(params, "path")
			delete(params, "serviceName")
		case "h2":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "h2-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["host"]; !ok {
					if host, exists := params["host"]; exists {
						opts["host"] = cloneJSONValue(host)
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "http":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "http-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = []any{cloneJSONValue(path)}
					}
				}
				if _, ok := opts["headers"]; !ok {
					if host, exists := params["host"]; exists {
						opts["headers"] = map[string]any{"Host": []any{cloneJSONValue(host)}}
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "xhttp":
			if hasTransportAlias(params, "path", "host", "mode") {
				opts := ensureObjectParam(params, "xhttp-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["host"]; !ok {
					if host, exists := params["host"]; exists {
						opts["host"] = cloneJSONValue(host)
					}
				}
				if _, ok := opts["mode"]; !ok {
					if mode, exists := params["mode"]; exists {
						opts["mode"] = cloneJSONValue(mode)
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
			delete(params, "mode")
		}
	case "trojan":
		switch network {
		case "ws":
			if hasTransportAlias(params, "path", "host") {
				opts := ensureObjectParam(params, "ws-opts")
				if _, ok := opts["path"]; !ok {
					if path, exists := params["path"]; exists {
						opts["path"] = cloneJSONValue(path)
					}
				}
				if _, ok := opts["headers"]; !ok {
					if host, exists := params["host"]; exists {
						opts["headers"] = map[string]any{"Host": cloneJSONValue(host)}
					}
				}
			}
			delete(params, "path")
			delete(params, "host")
		case "grpc":
			if hasTransportAlias(params, "path", "serviceName") {
				opts := ensureObjectParam(params, "grpc-opts")
				if _, ok := opts["grpc-service-name"]; !ok {
					if path, exists := params["path"]; exists {
						opts["grpc-service-name"] = cloneJSONValue(path)
					}
				}
			}
			delete(params, "path")
			delete(params, "serviceName")
		}
	}
}

func hasTransportAlias(params map[string]any, keys ...string) bool {
	for _, key := range keys {
		if _, ok := params[key]; ok {
			return true
		}
	}
	return false
}

func ensureObjectParam(m map[string]any, key string) map[string]any {
	if existing, ok := m[key].(map[string]any); ok {
		return existing
	}
	created := map[string]any{}
	m[key] = created
	return created
}

func protocolParamsForStorage(proto Protocol, params map[string]any) map[string]any {
	out := normalizeProtocolParameters(proto, params)
	out = cleanDisabledFeatures(proto.FormSchema, out)
	if proto.Protocol != "vless" && proto.Protocol != "vmess" {
		return out
	}
	security, ok := out["security"].(string)
	if !ok || security == "" {
		return out
	}
	switch security {
	case "tls", "reality":
		out["tls"] = true
	case "none":
		if _, exists := out["tls"]; exists {
			out["tls"] = false
		}
	}
	delete(out, "security")
	return out
}

// validateKnownTopLevel 拒绝协议注册表未声明的顶层字段，避免更新/导入时静默丢弃。
// 未知内容应作为扩展显式声明 scope/targets 后保存。
func validateKnownTopLevel(proto Protocol, params map[string]any) error {
	if err := validateStateOnlyParams(proto, params); err != nil {
		return err
	}
	allowed := make(map[string]bool, len(proto.FormSchema))
	for _, field := range proto.FormSchema {
		if field.StateOnly {
			continue
		}
		allowed[field.Name] = true
	}
	for key := range params {
		if !allowed[key] {
			return fmt.Errorf("字段 %s 未在协议注册表中声明，请将其归入 extensions", key)
		}
	}
	return nil
}

func cloneProjectValue(value any) any {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []int:
		return append([]int(nil), v...)
	case []int64:
		return append([]int64(nil), v...)
	default:
		return cloneJSONValue(value)
	}
}

func hasEffectiveValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case map[string]any:
		return len(v) > 0
	case []any:
		return len(v) > 0
	case []string:
		return len(v) > 0
	case []int:
		return len(v) > 0
	case []int64:
		return len(v) > 0
	default:
		return true
	}
}
