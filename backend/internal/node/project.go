package node

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	mierutp "github.com/enfein/mieru/v3/apis/trafficpattern"
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
		if field.StateOnly || !field.Matches(state, params, "") {
			continue
		}
		value, ok := params[field.Name]
		if !ok {
			continue
		}
		projected, ok := projectFieldValue(field, value, state, params)
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

func projectFieldValue(field FieldSchema, value any, state CurrentState, root map[string]any) (any, bool) {
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
		return projectObjectFields(field, object, state, root)
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
			projected, ok := projectObjectFields(field, object, state, root)
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

func projectObjectFields(field FieldSchema, object map[string]any, state CurrentState, root map[string]any) (map[string]any, bool) {
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
		if !property.Matches(state, root, "") {
			continue
		}
		value, ok := object[property.Name]
		if !ok {
			continue
		}
		projected, ok := projectFieldValue(property, value, state, root)
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
	if err := validateActiveFields(proto.FormSchema, state, params, "", target, params); err != nil {
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

func validateActiveFields(fields []FieldSchema, state CurrentState, params map[string]any, prefix, target string, root map[string]any) error {
	for _, field := range fields {
		if field.StateOnly || !field.Matches(state, root, target) {
			continue
		}
		path := field.Name
		if prefix != "" {
			path = prefix + "." + path
		}
		value, exists := params[field.Name]
		if field.RequiredFor(state, root, target) && (!exists || !hasEffectiveValue(value)) {
			return fmt.Errorf("字段 %s 必填", path)
		}
		if !exists || !hasEffectiveValue(value) {
			continue
		}
		if err := validateActiveFieldValue(field, value, state, path, target, root); err != nil {
			return err
		}
	}
	return nil
}

func validateActiveFieldValue(field FieldSchema, value any, state CurrentState, path, target string, root map[string]any) error {
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
		return validateActiveObjectFields(field, object, state, path, target, root)
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
			if err := validateActiveObjectFields(field, object, state, fmt.Sprintf("%s[%d]", path, i), target, root); err != nil {
				return err
			}
		}
		return nil
	default:
		return nil
	}
}

func validateActiveObjectFields(field FieldSchema, object map[string]any, state CurrentState, path, target string, root map[string]any) error {
	known := make(map[string]bool, len(field.Properties)+1)
	if field.ItemIDField != "" {
		known[field.ItemIDField] = true
	}
	for _, property := range field.Properties {
		known[property.Name] = true
		if !property.Matches(state, root, target) {
			continue
		}
		value, exists := object[property.Name]
		propertyPath := path + "." + property.Name
		if property.RequiredFor(state, root, target) && (!exists || !hasEffectiveValue(value)) {
			return fmt.Errorf("字段 %s 必填", propertyPath)
		}
		if !exists || !hasEffectiveValue(value) {
			continue
		}
		if err := validateActiveFieldValue(property, value, state, propertyPath, target, root); err != nil {
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
	case "shadowquic":
		if err := validateShadowQUICCombination(params); err != nil {
			return err
		}
	case "anytls":
		if err := validateAnyTLSCombination(params); err != nil {
			return err
		}
	case "trusttunnel":
		if err := validateTrustTunnelCombination(params); err != nil {
			return err
		}
	case "openvpn":
		if err := validateOpenVPNCombination(params); err != nil {
			return err
		}
	case "tailscale":
		if err := validateTailscaleCombination(params); err != nil {
			return err
		}
	case "masque":
		if err := validateMASQUECombination(state, params); err != nil {
			return err
		}
	case "mieru":
		if err := validateMieruCombination(state, params); err != nil {
			return err
		}
	case "wireguard":
		if err := validateWireGuardCombination(state, params); err != nil {
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

// isKeptCredentialCiphertext 判断字段值是否为更新／检查路径中保留的项目密文。
// 未改动的敏感字段以密文参与合并校验；密文在首次保存时已通过同一语义校验，
// 重新按明文解析既无意义也必然失败（Step 11 暴露的共享基线缺陷）。
func isKeptCredentialCiphertext(value any) bool {
	text, ok := value.(string)
	return ok && strings.HasPrefix(strings.TrimSpace(text), encPrefix)
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
	if auth := strings.TrimSpace(stringValue(params["auth"])); auth != "" && !isKeptCredentialCiphertext(params["auth"]) {
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
	if state.Selectors["auth_mode"] == "v5" && !isKeptCredentialCiphertext(params["uuid"]) {
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

// shadowQUICVersionAllowed 判断单个 QUIC 版本表达是否属于固定 tag parser 的 v1／v2。
// 固定 tag 的 ParseQUICVersion 还接受 1／2／rfc9000／rfc9369，Build32 只要求规范 v1／v2 表达。
func shadowQUICVersionAllowed(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "v1", "v2":
		return true
	}
	return false
}

// validateShadowQUICCombination 校验 QUIC 版本列表、非负整数与可选速率字符串。
// TLS 只有 sni／alpn（固定 tag 的 ShadowQuicOption 没有证书／ECH／skip-cert-verify）；
// zero-rtt 的重放风险由 adapter 以 warn 提示，不改变保存结果。
func validateShadowQUICCombination(params map[string]any) error {
	if items, ok := stringListItems(params["quic-versions"]); ok {
		for i, item := range items {
			if !shadowQUICVersionAllowed(item) {
				return errorsForField(fmt.Sprintf("quic-versions[%d]", i), "只接受固定 tag parser 支持的 v1 或 v2")
			}
		}
	}
	for _, name := range []string{"keep-alive-interval", "cwnd", "recv-window-conn", "recv-window",
		"max-datagram-frame-size", "max-open-streams"} {
		if err := validateIntegerMinimum(params, name, 0, "该字段必须是非负整数"); err != nil {
			return err
		}
	}
	for _, name := range []string{"up", "down"} {
		if value := strings.TrimSpace(stringValue(params[name])); value != "" && !validBandwidth(value) {
			return errorsForField(name, "带宽必须是非零的速率字符串（例如 100 Mbps）")
		}
	}
	return nil
}

// anytlsIdleSessionMinimum 是固定 tag 静默替换阈值：≤5 秒会被内核改成 30 秒。
// 项目要求显式取值只能为 0（未设置）或不低于该下限，避免保存值与内核生效值不一致。
const anytlsIdleSessionMinimum = 6

// validateAnyTLSCombination 校验 mTLS 成对、会话参数范围与固定 tag 的 idle-session 语义。
// 三种伪装对象的互斥、条件必填与切换清空由 schema 的 when／required_when／reset_on 结构性保证；
// 主 password 不声明 selector 清空域，因此不随 security_mode 切换清除。
func validateAnyTLSCombination(params map[string]any) error {
	if err := validateTLSKeyPair(params); err != nil {
		return err
	}
	for _, name := range []string{"idle-session-check-interval", "idle-session-timeout", "min-idle-session"} {
		if err := validateIntegerMinimum(params, name, 0, "该字段必须是非负整数"); err != nil {
			return err
		}
	}
	for _, name := range []string{"idle-session-check-interval", "idle-session-timeout"} {
		if value, ok := numberParam(params[name]); ok && value != 0 && value < anytlsIdleSessionMinimum {
			return errorsForField(name, fmt.Sprintf("空闲会话取值只能是 0（未设置）或不小于 %d 秒", anytlsIdleSessionMinimum))
		}
	}
	if interval, ok := numberParam(params["idle-session-check-interval"]); ok && interval > 0 {
		if timeout, ok := numberParam(params["idle-session-timeout"]); ok && timeout > 0 && timeout < interval {
			return errorsForField("idle-session-timeout", "空闲超时不能小于空闲检查间隔")
		}
	}
	return nil
}

// validateTrustTunnelCombination 校验 TrustTunnel 的凭据成对、mTLS 成对、复用分支与 QUIC 调优字段。
// reuse_mode 分支的活动、条件必填与切换清空由 schema 的 when／required_when／reset_on 结构性保证；
// 本函数补齐字段级语义：用户名／密码必须同时为空或同时非空，复用数字必须是正整数且两组不得混合。
func validateTrustTunnelCombination(params map[string]any) error {
	username := hasTextParam(params, "username")
	password := hasTextParam(params, "password")
	if username != password {
		if username {
			return errorsForField("password", "提供用户名时必须同时提供密码")
		}
		return errorsForField("username", "提供密码时必须同时提供用户名")
	}
	if err := validateTLSKeyPair(params); err != nil {
		return err
	}
	connections := hasNumberParam(params, "max-connections") || hasNumberParam(params, "min-streams")
	streams := hasNumberParam(params, "max-streams")
	if connections && streams {
		return errorsForField("max-streams", "max-connections／min-streams 与 max-streams 属于互斥复用模式，不得同时配置")
	}
	for _, name := range []string{"max-connections", "min-streams", "max-streams"} {
		if value, ok := numberParam(params[name]); ok && (value < 1 || value != math.Trunc(value)) {
			return errorsForField(name, "复用参数必须是正整数")
		}
	}
	if err := validateIntegerMinimum(params, "cwnd", 0, "该字段必须是非负整数"); err != nil {
		return err
	}
	return nil
}

// tailscaleHostnamePattern 是 Tailscale 设备名的 DNS label 约束（固定 tag 本身不做校验）。
var tailscaleHostnamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// openvpnCompLZOValues 是项目侧收紧后允许的 comp-lzo 取值（固定 tag 只归一化 yes／adaptive）。
var openvpnCompLZOValues = []string{"yes", "no", "adaptive"}

// validateOpenVPNCombination 校验 OpenVPN 的固定枚举、非负整数与 peer-info 键值合同。
// 认证组完整性与三种 TLS key 的互斥由 schema 的 when／required_when／reset_on／clear_when_inactive
// 结构性保证；本函数补齐固定 tag 会拒绝、但声明式元数据无法表达的取值约束。
func validateOpenVPNCombination(params map[string]any) error {
	if proto := strings.ToLower(strings.TrimSpace(stringValue(params["proto"]))); proto != "" && proto != "udp" && proto != "tcp" {
		return errorsForField("proto", "固定 tag 只支持 udp 与 tcp")
	}
	if dev := strings.ToLower(strings.TrimSpace(stringValue(params["dev"]))); dev != "" && dev != "tun" {
		return errorsForField("dev", "固定 tag 只支持 tun")
	}
	for _, name := range []string{"cipher", "data-ciphers-fallback"} {
		value := strings.TrimSpace(stringValue(params[name]))
		if value != "" && !containsString(openvpnCipherValues, value) {
			return errorsForField(name, "只支持固定 tag 的 AES-GCM／AES-CBC 与 CHACHA20-POLY1305")
		}
	}
	if items, ok := stringListItems(params["data-ciphers"]); ok {
		for i, item := range items {
			if !containsString(openvpnCipherValues, item) {
				return errorsForField(fmt.Sprintf("data-ciphers[%d]", i), "只支持固定 tag 的 AES-GCM／AES-CBC 与 CHACHA20-POLY1305")
			}
		}
	}
	if auth := strings.TrimSpace(stringValue(params["auth"])); auth != "" && !openvpnAuthAllowed(auth) {
		return errorsForField("auth", "只支持固定 tag 的 MD5／SHA1／SHA256／SHA384／SHA512")
	}
	if lzo := strings.ToLower(strings.TrimSpace(stringValue(params["comp-lzo"]))); lzo != "" && !containsString(openvpnCompLZOValues, lzo) {
		return errorsForField("comp-lzo", "只支持 yes／no／adaptive 或留空")
	}
	if direction := strings.TrimSpace(stringValue(params["key-direction"])); direction != "" && direction != "0" && direction != "1" {
		return errorsForField("key-direction", "只允许 0、1 或留空")
	}
	if err := validateOpenVPNPeerInfo(params); err != nil {
		return err
	}
	for _, name := range []string{"ping", "ping-restart", "tran-window", "handshake-timeout", "mtu"} {
		if err := validateIntegerMinimum(params, name, 0, "该字段必须是非负整数"); err != nil {
			return err
		}
	}
	return nil
}

// openvpnAuthAllowed 判断 auth 摘要是否属于固定 tag 的 normalizeAuth 集合（大小写不敏感）。
func openvpnAuthAllowed(value string) bool {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "MD5", "SHA1", "SHA256", "SHA384", "SHA512":
		return true
	}
	return false
}

// validateOpenVPNPeerInfo 执行 peer-info 的键值合同，与 HTTP headers 保持同一套规则。
func validateOpenVPNPeerInfo(params map[string]any) error {
	peerInfo, ok := params["peer-info"].(map[string]any)
	if !ok {
		return nil
	}
	seen := make(map[string]string, len(peerInfo))
	for key, value := range peerInfo {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			return errorsForField("peer-info", "Peer Info 键不能为空")
		}
		if _, ok := value.(string); !ok {
			return errorsForField("peer-info."+trimmed, "Peer Info 值必须为字符串")
		}
		lower := strings.ToLower(trimmed)
		if previous, exists := seen[lower]; exists {
			return errorsForField("peer-info."+trimmed, "Peer Info 键 "+previous+" 与 "+trimmed+" 大小写不敏感重复")
		}
		seen[lower] = trimmed
	}
	return nil
}

// validateTailscaleCombination 校验设备名、控制面地址与出口节点。
// auth-key 允许为空（由 adapter 返回“首次真实连接需要交互登录”的 warn）；LAN access 的空值清空
// 由 canonicalizeTailscaleGuards 与 non_empty 条件共同保证。
func validateTailscaleCombination(params map[string]any) error {
	if hostname := strings.TrimSpace(stringValue(params["hostname"])); hostname != "" && !tailscaleHostnamePattern.MatchString(hostname) {
		return errorsForField("hostname", "必须是合法设备名（小写字母、数字与连字符，1-63 位，首尾不能为连字符）")
	}
	if controlURL := strings.TrimSpace(stringValue(params["control-url"])); controlURL != "" {
		parsed, err := url.Parse(controlURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			return errorsForField("control-url", "必须是绝对的 HTTP(S) URL")
		}
	}
	if exitNode := strings.TrimSpace(stringValue(params["exit-node"])); exitNode != "" {
		if net.ParseIP(exitNode) == nil {
			// 固定 tag 只把 "auto:" + 非空后缀识别为自动出口节点。
			expr, ok := strings.CutPrefix(exitNode, "auto:")
			if !ok || strings.TrimSpace(expr) == "" {
				return errorsForField("exit-node", "必须是合法 IP 或 auto:* 形式")
			}
		}
	}
	return nil
}

// validateMASQUEKeys 按固定 tag 校验 MASQUE 的两类 EC 密钥：
// private-key 必须是 Base64＋SEC1 EC 私钥，public-key 必须是 Base64＋PKIX 且为 ECDSA 公钥。
// 只判断「能 Base64 解码」不足以表达固定 tag 的结构要求。
func validateMASQUEKeys(params map[string]any) error {
	privateKey := strings.TrimSpace(stringValue(params["private-key"]))
	if privateKey != "" && !isKeptCredentialCiphertext(privateKey) {
		decoded, err := base64.StdEncoding.DecodeString(privateKey)
		if err != nil {
			return errorsForField("private-key", "必须是合法的 Base64 编码")
		}
		if _, err := x509.ParseECPrivateKey(decoded); err != nil {
			return errorsForField("private-key", "必须是 SEC1 编码的 EC 私钥")
		}
	}
	publicKey := strings.TrimSpace(stringValue(params["public-key"]))
	if publicKey != "" {
		decoded, err := base64.StdEncoding.DecodeString(publicKey)
		if err != nil {
			return errorsForField("public-key", "必须是合法的 Base64 编码")
		}
		parsed, err := x509.ParsePKIXPublicKey(decoded)
		if err != nil {
			return errorsForField("public-key", "必须是 PKIX 编码的公钥")
		}
		if _, ok := parsed.(*ecdsa.PublicKey); !ok {
			return errorsForField("public-key", "必须是 ECDSA 公钥")
		}
	}
	return nil
}

// validateMASQUEURI 校验连接 URI 是带 scheme 与 host 的绝对 URL。
// 错误只返回固定文案，不回显可能含 userinfo／query 凭据的原值。
func validateMASQUEURI(params map[string]any) error {
	raw := strings.TrimSpace(stringValue(params["uri"]))
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errorsForField("uri", "必须是带 scheme 与 host 的绝对 URL")
	}
	return nil
}

// validateMASQUECombination 校验 MASQUE 的 EC 密钥结构、本地地址、URI、非负数值与 DNS 列表。
// network_mode 分支清空（h3_l4proxy 强制关闭 UDP、h2／h3_l4proxy 清空 QUIC 调优）由 schema 的
// when／reset_on 与 clearSelectorScopedFields 结构性保证。
func validateMASQUECombination(_ CurrentState, params map[string]any) error {
	if err := validateMASQUEKeys(params); err != nil {
		return err
	}
	if err := validateLocalAddresses(params, "MASQUE"); err != nil {
		return err
	}
	if err := validateMASQUEURI(params); err != nil {
		return err
	}
	for _, name := range []string{"mtu", "cwnd", "handshake-timeout"} {
		if err := validateIntegerMinimum(params, name, 0, "该字段必须是非负整数"); err != nil {
			return err
		}
	}
	return validateDNSList(params)
}

// mieruPortRangeBounds 解析并校验固定 tag 的单段 begin-end 端口段（两端 1-65535 且 begin ≤ end）。
// 固定 tag 使用 Sscanf，会静默接受 "1-2-3" 这类尾随输入；项目按 Build32 收紧为严格单段。
func mieruPortRangeBounds(value string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	begin, errBegin := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, errEnd := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errBegin != nil || errEnd != nil {
		return 0, 0, false
	}
	if begin < 1 || begin > 65535 || end < 1 || end > 65535 || begin > end {
		return 0, 0, false
	}
	return begin, end, true
}

// validateMieruTrafficPattern 先 Base64 解码再执行固定 tag 语义校验。
// 固定 tag 的错误文本包含原串，因此项目只返回不含原值的字段级错误。
func validateMieruTrafficPattern(params map[string]any) error {
	raw := strings.TrimSpace(stringValue(params["traffic-pattern"]))
	if raw == "" {
		return nil
	}
	pattern, err := mierutp.Decode(raw)
	if err != nil {
		return errorsForField("traffic-pattern", "必须是固定 tag 可解析的 Base64 TrafficPattern")
	}
	if err := mierutp.Validate(pattern); err != nil {
		return errorsForField("traffic-pattern", "TrafficPattern 未通过固定 tag 语义校验")
	}
	return nil
}

// validateMieruCombination 校验 Mieru 的端口段、流量特征与传输枚举。
// port 与 port-range 的二选一由 endpoint policy＋selector 清空域结构性保证：
// range 模式顶层 port 规范化为 0，single 模式清空 port-range。
func validateMieruCombination(state CurrentState, params map[string]any) error {
	if state.Selectors["endpoint_mode"] == "range" {
		value := strings.TrimSpace(stringValue(params["port-range"]))
		if value == "" {
			return errorsForField("port-range", "range 模式必须填写单个 begin-end 端口段")
		}
		if _, _, ok := mieruPortRangeBounds(value); !ok {
			return errorsForField("port-range", "端口段只接受 1-65535 的单个 begin-end 且 begin ≤ end")
		}
	}
	return validateMieruTrafficPattern(params)
}

// wireGuardKeyLength 是 curve25519 私钥／公钥与预共享密钥的固定长度（字节）。
// 固定 tag 只校验 Base64 可解码，长度是 WireGuard 协议自身的形状要求，由项目在保存前收紧。
const wireGuardKeyLength = 32

// wireGuardDNSSchemes 是固定 tag parseNameServer 明确接受的 DNS scheme 集合。
var wireGuardDNSSchemes = map[string]bool{
	"udp": true, "tcp": true, "tls": true, "http": true, "https": true, "quic": true,
	"system": true, "ts": true, "tailscale": true, "et": true, "easytier": true, "dhcp": true,
}

// validateWireGuardCombination 校验标准 WireGuard 的密钥、本地地址、非负整数、DNS 与 Peer 分支合同。
// AmneziaWG 已由 Build32 第三章明确排除，不在此校验任何 amnezia-wg-option 字段。
func validateWireGuardCombination(state CurrentState, params map[string]any) error {
	if err := validateWireGuardKeys(params); err != nil {
		return err
	}
	if err := validateLocalAddresses(params, "WireGuard"); err != nil {
		return err
	}
	for _, name := range []string{"workers", "mtu", "persistent-keepalive", "refresh-server-ip-interval"} {
		if err := validateIntegerMinimum(params, name, 0, "该字段必须是非负整数"); err != nil {
			return err
		}
	}
	if err := validateDNSList(params); err != nil {
		return err
	}
	if state.Selectors["peer_mode"] == "peers" {
		return validateWireGuardPeers(params)
	}
	_, err := validateWireGuardAllowedIPs(stringListValues(params["allowed-ips"]), "allowed-ips")
	return err
}

// validateWireGuardBase64Key 校验单个 WireGuard 密钥为合法 Base64 且恰为 32 字节。
// 更新／检查路径中未改动的敏感字段以项目密文参与合并，密文不重新解析（保留语义）。
func validateWireGuardBase64Key(source map[string]any, name, path string) error {
	text := strings.TrimSpace(stringValue(source[name]))
	if text == "" || isKeptCredentialCiphertext(text) {
		return nil
	}
	decoded, err := base64.StdEncoding.DecodeString(text)
	if err != nil {
		return errorsForField(path, "必须是合法的 Base64 编码")
	}
	if len(decoded) != wireGuardKeyLength {
		return errorsForField(path, fmt.Sprintf("Base64 解码后必须为 %d 字节，实际 %d", wireGuardKeyLength, len(decoded)))
	}
	return nil
}

// validateWireGuardKeys 覆盖顶层 private-key／public-key／pre-shared-key 与每个 Peer 的公钥、PSK。
func validateWireGuardKeys(params map[string]any) error {
	for _, name := range []string{"private-key", "public-key", "pre-shared-key"} {
		if err := validateWireGuardBase64Key(params, name, name); err != nil {
			return err
		}
	}
	peers, ok := params["peers"].([]any)
	if !ok {
		return nil
	}
	for i, value := range peers {
		peer, ok := value.(map[string]any)
		if !ok {
			continue
		}
		for _, name := range []string{"public-key", "pre-shared-key"} {
			if err := validateWireGuardBase64Key(peer, name, fmt.Sprintf("peers[%d].%s", i, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateLocalAddresses 要求 ip／ipv6 至少一项且均为合法前缀（缺省前缀已在归一化补齐）；
// WireGuard 与 MASQUE 的固定 tag 都使用同一 Prefixes() 语义。
func validateLocalAddresses(params map[string]any, label string) error {
	count := 0
	for _, name := range []string{"ip", "ipv6"} {
		text := strings.TrimSpace(stringValue(params[name]))
		if text == "" {
			continue
		}
		if _, err := netip.ParsePrefix(text); err != nil {
			return errorsForField(name, "必须是合法的 IP 地址或 CIDR 前缀")
		}
		count++
	}
	if count == 0 {
		return errorsForField("ip", label+" 至少需要一个本地地址（ip 或 ipv6）")
	}
	return nil
}

// wireGuardNetworkKey 把 CIDR 归一化为掩码后的网段字符串，用于跨 Peer 冲突比较。
func wireGuardNetworkKey(cidr string) (string, bool) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return "", false
	}
	return prefix.Masked().String(), true
}

// validateWireGuardAllowedIPs 逐项校验 CIDR，返回出现的网段集合（值无实际用途）。
func validateWireGuardAllowedIPs(items []string, path string) (map[string]bool, error) {
	seen := make(map[string]bool, len(items))
	for i, item := range items {
		key, ok := wireGuardNetworkKey(item)
		if !ok {
			return nil, errorsForField(fmt.Sprintf("%s[%d]", path, i), "Allowed IPs 必须是合法 CIDR")
		}
		seen[key] = true
	}
	return seen, nil
}

// validateWireGuardPeers 校验多 Peer 分支：至少两项、每项稳定身份与必填字段、allowed-ips 不跨 Peer 冲突。
func validateWireGuardPeers(params map[string]any) error {
	peers, ok := params["peers"].([]any)
	if !ok || len(peers) == 0 {
		return errorsForField("peers", "多 Peer 模式必须提供 peers 列表")
	}
	if len(peers) < 2 {
		return errorsForField("peers", "多 Peer 模式至少需要两个 Peer 条目")
	}
	owner := map[string]int{}
	for i, value := range peers {
		peer, ok := value.(map[string]any)
		if !ok {
			return errorsForField(fmt.Sprintf("peers[%d]", i), "Peer 必须是结构化对象")
		}
		if strings.TrimSpace(stringValue(peer[sensitiveItemIDField])) == "" {
			return errorsForField(fmt.Sprintf("peers[%d].%s", i, sensitiveItemIDField), "Peer 必须具有稳定凭据身份")
		}
		if strings.TrimSpace(stringValue(peer["server"])) == "" {
			return errorsForField(fmt.Sprintf("peers[%d].server", i), "Peer 服务器不能为空")
		}
		if port, ok := numberParam(peer["port"]); !ok || math.Trunc(port) != port || port < 1 || port > 65535 {
			return errorsForField(fmt.Sprintf("peers[%d].port", i), "Peer 端口必须是 1-65535 的整数")
		}
		items := stringListValues(peer["allowed-ips"])
		if len(items) == 0 {
			return errorsForField(fmt.Sprintf("peers[%d].allowed-ips", i), "多 Peer 模式每项必须填写 Allowed IPs")
		}
		for j, item := range items {
			key, ok := wireGuardNetworkKey(item)
			if !ok {
				return errorsForField(fmt.Sprintf("peers[%d].allowed-ips[%d]", i, j), "Allowed IPs 必须是合法 CIDR")
			}
			if previous, exists := owner[key]; exists && previous != i {
				return errorsForField(fmt.Sprintf("peers[%d].allowed-ips", i),
					fmt.Sprintf("网段 %s 已被第 %d 个 Peer 使用，不同 Peer 不允许相同网段", key, previous+1))
			}
			owner[key] = i
		}
	}
	return nil
}

// validateDNSList 校验远端解析开启时的 DNS 列表；关闭时的清空由 feature 清空域保证（共用）。
func validateDNSList(params map[string]any) error {
	for i, item := range stringListValues(params["dns"]) {
		text := strings.TrimSpace(item)
		if text == "" {
			return errorsForField(fmt.Sprintf("dns[%d]", i), "DNS 条目不能为空")
		}
		if strings.ContainsAny(text, " \t") {
			return errorsForField(fmt.Sprintf("dns[%d]", i), "DNS 条目不能包含空白字符")
		}
		scheme, rest, found := strings.Cut(text, "://")
		if !found {
			continue
		}
		if !wireGuardDNSSchemes[strings.ToLower(scheme)] {
			return errorsForField(fmt.Sprintf("dns[%d]", i), "不支持的 DNS scheme: "+scheme)
		}
		if strings.TrimSpace(rest) == "" && !strings.EqualFold(scheme, "system") {
			return errorsForField(fmt.Sprintf("dns[%d]", i), "DNS 条目缺少地址")
		}
	}
	return nil
}

// stringListValues 把列表字段读取为字符串切片；非文本类型返回 nil（类型错误由 schema 校验报告）。
func stringListValues(value any) []string {
	items, ok := stringListItems(value)
	if !ok {
		return nil
	}
	return items
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
	if key == "" || isKeptCredentialCiphertext(params["private-key"]) {
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
			if field.When != nil && len(field.When.Selectors) > 0 && !field.When.Matches(state, params, "") {
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
