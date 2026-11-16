// openvpn_import.go：Build32 Step 19 的 `.ovpn` 只读解析器。
// 边界：有界输入、纯内存、不读取任何外部文件、不执行脚本／hook／include、不落库。
// 解析结果只是结构化草稿与来源行号，字段级诊断使用稳定 code，前端不得解析中文 message 决定行为。
package node

import (
	"fmt"
	"strconv"
	"strings"
)

// MaxOpenVPNParseBytes 是 `.ovpn` 文本的硬上限（256 KiB），与前端选择阶段限制一致。
const MaxOpenVPNParseBytes = 256 << 10

// OpenVPN 解析诊断的稳定 code。
const (
	// 冻结集合（Build32 §4.4）
	OpenVPNDiagUnsupportedDirective  = "ovpn_unsupported_directive"
	OpenVPNDiagExternalFileForbidden = "ovpn_external_file_forbidden"
	OpenVPNDiagMultipleRemotes       = "ovpn_multiple_remotes"
	OpenVPNDiagUnclosedInlineBlock   = "ovpn_unclosed_inline_block"
	OpenVPNDiagConflictingAuth       = "ovpn_conflicting_auth"
	OpenVPNDiagConflictingTLSKey     = "ovpn_conflicting_tls_key"
	// 本步补充的稳定 code
	OpenVPNDiagDangerousDirective = "ovpn_dangerous_directive"
	OpenVPNDiagConflictingValue   = "ovpn_conflicting_directive"
	OpenVPNDiagUnsupportedValue   = "ovpn_unsupported_value"
	OpenVPNDiagInlineBlockMissing = "ovpn_inline_block_missing"
	OpenVPNDiagAuthUserPassFile   = "ovpn_auth_user_pass_file_ignored"
	OpenVPNDiagRemotePortMissing  = "ovpn_remote_port_missing"
	OpenVPNDiagNoAuthDirective    = "ovpn_no_auth_directive"
	OpenVPNDiagSizeExceeded       = "ovpn_size_exceeded"
)

// OpenVPNParseDiagnostic 是单个诊断；severity 固定为 error／warn／info。
type OpenVPNParseDiagnostic struct {
	Severity  string `json:"severity"`
	Code      string `json:"code"`
	Line      int    `json:"line,omitempty"`
	FieldPath string `json:"field_path,omitempty"`
	Message   string `json:"message"`
}

// OpenVPNParseResult 是解析后的结构化草稿、来源行号与诊断，永不包含原文。
// 行号、诊断与原文都不得进入最终保存请求。
type OpenVPNParseResult struct {
	Host         string                   `json:"host"`
	Port         int                      `json:"port"`
	ProtocolJSON map[string]any           `json:"protocol_json"`
	Selectors    map[string]string        `json:"selectors,omitempty"`
	FieldSources map[string]int           `json:"field_sources,omitempty"`
	Diagnostics  []OpenVPNParseDiagnostic `json:"diagnostics"`
}

// OpenVPNParseError 表示解析被阻断（HTTP 400）；Code 是稳定的诊断 code。
type OpenVPNParseError struct {
	Code    string
	Message string
}

func (e *OpenVPNParseError) Error() string { return e.Message }

// openvpnInlineBlocks 是受支持的内嵌块指令；标签去除后内容映射到同名字段。
var openvpnInlineBlocks = map[string]bool{
	"ca": true, "cert": true, "key": true,
	"tls-auth": true, "tls-crypt": true, "tls-crypt-v2": true,
}

// openvpnDangerousDirectives 是脚本、hook、权限变更与 include 类指令：一律 400，且绝不执行。
var openvpnDangerousDirectives = map[string]bool{
	"up": true, "down": true, "route-up": true, "route-pre-down": true, "ipchange": true,
	"tls-verify": true, "auth-user-pass-verify": true, "client-connect": true,
	"client-disconnect": true, "learn-address": true, "script-security": true,
	"plugin": true, "include": true, "chroot": true, "user": true, "group": true,
	"daemon": true, "askpass": true, "pkcs12": true, "engine": true, "crl-verify": true,
	"writepid": true, "status": true, "log": true, "log-append": true,
}

// openvpnSingleValueDirectives 是必须唯一且取值确定的结构化指令。
var openvpnSingleValueDirectives = map[string]bool{
	"proto": true, "dev": true, "cipher": true, "data-ciphers": true,
	"data-ciphers-fallback": true, "auth": true, "comp-lzo": true, "ping": true,
	"ping-restart": true, "tran-window": true, "handshake-timeout": true, "mtu": true,
	"key-direction": true,
}

// openvpnParseState 承载解析过程的中间事实；只保存行号与规范值，不保存原文行。
type openvpnParseState struct {
	result       *OpenVPNParseResult
	singleValues map[string]string
	remotes      []string
	certSeen     bool
	keySeen      bool
	authUserPass bool
	tlsKeys      map[string]bool
	firstCode    string
}

func (s *openvpnParseState) source(path string, line int) {
	if _, exists := s.result.FieldSources[path]; !exists {
		s.result.FieldSources[path] = line
	}
}

// errorf 记录阻断诊断并保留第一个错误 code 作为 HTTP 400 的稳定标识。
func (s *openvpnParseState) errorf(code, path string, line int, message string) {
	if s.firstCode == "" {
		s.firstCode = code
	}
	s.result.Diagnostics = append(s.result.Diagnostics, OpenVPNParseDiagnostic{
		Severity: "error", Code: code, Line: line, FieldPath: path, Message: message,
	})
}

func (s *openvpnParseState) warnf(code, path string, line int, message string) {
	s.result.Diagnostics = append(s.result.Diagnostics, OpenVPNParseDiagnostic{
		Severity: "warn", Code: code, Line: line, FieldPath: path, Message: message,
	})
}

// setSingle 记录单值指令；重复出现且规范值不同即视为冲突。
func (s *openvpnParseState) setSingle(key, value, path string, line int, message string) {
	if previous, exists := s.singleValues[key]; exists {
		if previous != value {
			s.errorf(OpenVPNDiagConflictingValue, path, line, message)
		}
		return
	}
	s.singleValues[key] = value
	s.result.ProtocolJSON[path] = value
	s.source(path, line)
}

// ParseOpenVPN 解析 `.ovpn` 文本为结构化草稿。
// 阻断时清空可应用草稿、保留诊断，并返回 *OpenVPNParseError 供调用方映射 400。
func ParseOpenVPN(text string) (*OpenVPNParseResult, error) {
	if len(text) > MaxOpenVPNParseBytes {
		result := &OpenVPNParseResult{
			ProtocolJSON: map[string]any{},
			Diagnostics: []OpenVPNParseDiagnostic{{
				Severity: "error", Code: OpenVPNDiagSizeExceeded,
				Message: fmt.Sprintf("`.ovpn` 文本超过 %d 字节上限", MaxOpenVPNParseBytes),
			}},
		}
		return result, &OpenVPNParseError{Code: OpenVPNDiagSizeExceeded, Message: "`.ovpn` 文本超过大小上限"}
	}

	state := &openvpnParseState{
		result: &OpenVPNParseResult{
			ProtocolJSON: map[string]any{},
			FieldSources: map[string]int{},
		},
		singleValues: map[string]string{},
		tlsKeys:      map[string]bool{},
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for index := 0; index < len(lines); index++ {
		lineNumber := index + 1
		raw := strings.TrimSpace(lines[index])
		if raw == "" || strings.HasPrefix(raw, "#") || strings.HasPrefix(raw, ";") {
			continue
		}
		if block, ok := openvpnBlockName(raw); ok {
			index = state.consumeInlineBlock(lines, index, block)
			continue
		}
		state.consumeDirective(raw, lineNumber)
	}

	state.finishAuth()
	return state.finish()
}

// openvpnBlockName 判断当前行是否为内嵌块开始标签，例如 `<ca>`。
func openvpnBlockName(line string) (string, bool) {
	if !strings.HasPrefix(line, "<") || !strings.HasSuffix(line, ">") || strings.HasPrefix(line, "</") {
		return "", false
	}
	name := strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
	if !openvpnInlineBlocks[name] {
		return "", false
	}
	return name, true
}

// consumeInlineBlock 收集内嵌块内容并返回结束标签所在的下标；未闭合时记录阻断并消费到结尾。
func (s *openvpnParseState) consumeInlineBlock(lines []string, openIndex int, name string) int {
	openLine := openIndex + 1
	var body []string
	closeTag := "</" + name + ">"
	for index := openIndex + 1; index < len(lines); index++ {
		if strings.EqualFold(strings.TrimSpace(lines[index]), closeTag) {
			s.applyInlineBlock(name, strings.Join(body, "\n"), openLine)
			return index
		}
		body = append(body, lines[index])
	}
	s.errorf(OpenVPNDiagUnclosedInlineBlock, name, openLine, fmt.Sprintf("内嵌块 <%s> 未闭合", name))
	return len(lines)
}

// applyInlineBlock 把内嵌块内容映射到同名字段，并处理重复块冲突。
func (s *openvpnParseState) applyInlineBlock(name, body string, line int) {
	content := strings.TrimSpace(body)
	if content == "" {
		s.warnf(OpenVPNDiagInlineBlockMissing, name, line, fmt.Sprintf("内嵌块 <%s> 为空", name))
		return
	}
	if previous, exists := s.result.ProtocolJSON[name]; exists {
		if previous != content {
			s.errorf(OpenVPNDiagConflictingValue, name, line, fmt.Sprintf("内嵌块 <%s> 出现多个不同内容", name))
		}
		return
	}
	s.result.ProtocolJSON[name] = content
	s.source(name, line)
	switch name {
	case "cert":
		s.certSeen = true
	case "key":
		s.keySeen = true
	case "tls-auth", "tls-crypt", "tls-crypt-v2":
		s.tlsKeys[name] = true
	}
}

// consumeDirective 处理单个非内嵌块指令行。
func (s *openvpnParseState) consumeDirective(line string, lineNumber int) {
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return
	}
	name := strings.ToLower(tokens[0])
	args := tokens[1:]

	if openvpnDangerousDirectives[name] {
		s.errorf(OpenVPNDiagDangerousDirective, name, lineNumber,
			fmt.Sprintf("指令 %s 属于脚本／hook／include 或权限变更，解析器不会执行或读取", name))
		return
	}
	switch name {
	case "remote":
		s.consumeRemote(args, lineNumber)
	case "proto":
		s.consumeProto(args, lineNumber)
	case "dev":
		s.consumeDev(args, lineNumber)
	case "cipher":
		s.consumeCipher(args, lineNumber)
	case "data-ciphers":
		s.consumeDataCiphers(args, lineNumber)
	case "data-ciphers-fallback":
		s.consumeCipherAlias("data-ciphers-fallback", args, lineNumber)
	case "auth":
		s.consumeAuth(args, lineNumber)
	case "comp-lzo":
		s.consumeCompLZO(args, lineNumber)
	case "key-direction":
		s.consumeKeyDirection(args, lineNumber)
	case "ping", "ping-restart", "tran-window", "handshake-timeout", "mtu":
		s.consumeInteger(name, args, lineNumber)
	case "peer-info":
		s.consumePeerInfo(args, lineNumber)
	case "auth-user-pass":
		s.consumeAuthUserPass(args, lineNumber)
	default:
		if openvpnInlineBlocks[name] {
			// `ca inline`／`tls-auth [inline]` 只是声明内容由后续内嵌块提供，不读取任何文件；
			// 其它参数形式是 inline 之外的文件引用，一律拒绝且绝不读取服务器文件。
			if len(args) == 0 || strings.EqualFold(args[0], "inline") || args[0] == "[inline]" {
				return
			}
			s.errorf(OpenVPNDiagExternalFileForbidden, name, lineNumber,
				fmt.Sprintf("指令 %s 引用外部文件，解析器不会读取服务器文件", name))
			return
		}
		s.warnf(OpenVPNDiagUnsupportedDirective, name, lineNumber,
			fmt.Sprintf("指令 %s 不在固定 tag 的结构化字段内，已忽略", name))
	}
}

func (s *openvpnParseState) consumeRemote(args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, "remote", line, "remote 缺少主机地址")
		return
	}
	host := strings.TrimSpace(args[0])
	port := 0
	if len(args) > 1 {
		parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil || parsed < 1 || parsed > 65535 {
			s.errorf(OpenVPNDiagUnsupportedValue, "remote", line, "remote 端口必须是 1-65535 的整数")
			return
		}
		port = parsed
	} else {
		s.warnf(OpenVPNDiagRemotePortMissing, "remote", line, "remote 未提供端口，需要在表单中补充")
	}
	spec := fmt.Sprintf("%s:%d", host, port)
	for _, existing := range s.remotes {
		if existing == spec {
			return
		}
	}
	s.remotes = append(s.remotes, spec)
	if len(s.remotes) > 1 {
		s.errorf(OpenVPNDiagMultipleRemotes, "remote", line, "存在多个不同 remote，解析器不会静默选择其中一项")
		return
	}
	s.result.Host = host
	s.result.Port = port
	s.source("remote", line)
}

func (s *openvpnParseState) consumeProto(args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, "proto", line, "proto 缺少取值")
		return
	}
	var value string
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "udp", "udp4":
		value = "udp"
	case "tcp", "tcp-client", "tcp4", "tcp4-client":
		value = "tcp"
	default:
		s.errorf(OpenVPNDiagUnsupportedValue, "proto", line, "固定 tag 只支持 udp 与 tcp")
		return
	}
	s.setSingle("proto", value, "proto", line, "proto 出现互相冲突的取值")
}

func (s *openvpnParseState) consumeDev(args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, "dev", line, "dev 缺少取值")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(args[0]), "tun") {
		s.errorf(OpenVPNDiagUnsupportedValue, "dev", line, "固定 tag 只支持 dev tun")
		return
	}
	s.setSingle("dev", "tun", "dev", line, "dev 出现互相冲突的取值")
}

func (s *openvpnParseState) consumeCipher(args []string, line int) {
	s.consumeCipherAlias("cipher", args, line)
}

// consumeCipherAlias 处理 cipher 与 data-ciphers-fallback：大写化并把固定 tag 的 AES-CBC 别名归一化。
func (s *openvpnParseState) consumeCipherAlias(field string, args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, field, line, fmt.Sprintf("%s 缺少取值", field))
		return
	}
	value := strings.ToUpper(strings.TrimSpace(args[0]))
	if value == "AES-CBC" {
		value = "AES-128-CBC"
	}
	if !containsString(openvpnCipherValues, value) {
		s.errorf(OpenVPNDiagUnsupportedValue, field, line, "只支持固定 tag 的 AES-GCM／AES-CBC 与 CHACHA20-POLY1305")
		return
	}
	s.setSingle(field, value, field, line, fmt.Sprintf("%s 出现互相冲突的取值", field))
}

func (s *openvpnParseState) consumeDataCiphers(args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, "data-ciphers", line, "data-ciphers 缺少取值")
		return
	}
	raw := strings.Join(args, "")
	items := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ":") {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		value := strings.ToUpper(trimmed)
		if value == "AES-CBC" {
			value = "AES-128-CBC"
		}
		if !containsString(openvpnCipherValues, value) {
			s.errorf(OpenVPNDiagUnsupportedValue, "data-ciphers", line, "只支持固定 tag 的 AES-GCM／AES-CBC 与 CHACHA20-POLY1305")
			return
		}
		items = append(items, value)
	}
	if len(items) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, "data-ciphers", line, "data-ciphers 缺少有效取值")
		return
	}
	joined := strings.Join(items, ",")
	if previous, exists := s.singleValues["data-ciphers"]; exists {
		if previous != joined {
			s.errorf(OpenVPNDiagConflictingValue, "data-ciphers", line, "data-ciphers 出现互相冲突的取值")
		}
		return
	}
	s.singleValues["data-ciphers"] = joined
	s.result.ProtocolJSON["data-ciphers"] = items
	s.source("data-ciphers", line)
}

func (s *openvpnParseState) consumeAuth(args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, "auth", line, "auth 缺少取值")
		return
	}
	value := strings.ToUpper(strings.TrimSpace(args[0]))
	if value == "SHA-1" {
		value = "SHA1"
	}
	if !openvpnAuthAllowed(value) {
		s.errorf(OpenVPNDiagUnsupportedValue, "auth", line, "只支持固定 tag 的 MD5／SHA1／SHA256／SHA384／SHA512")
		return
	}
	s.setSingle("auth", value, "auth", line, "auth 出现互相冲突的取值")
}

func (s *openvpnParseState) consumeCompLZO(args []string, line int) {
	value := "yes"
	if len(args) > 0 {
		value = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if !containsString(openvpnCompLZOValues, value) {
		s.errorf(OpenVPNDiagUnsupportedValue, "comp-lzo", line, "只支持 yes／no／adaptive")
		return
	}
	s.setSingle("comp-lzo", value, "comp-lzo", line, "comp-lzo 出现互相冲突的取值")
}

func (s *openvpnParseState) consumeKeyDirection(args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, "key-direction", line, "key-direction 缺少取值")
		return
	}
	value := strings.TrimSpace(args[0])
	if value != "0" && value != "1" {
		s.errorf(OpenVPNDiagUnsupportedValue, "key-direction", line, "固定 tag 只支持 key-direction 0 或 1")
		return
	}
	s.setSingle("key-direction", value, "key-direction", line, "key-direction 出现互相冲突的取值")
}

func (s *openvpnParseState) consumeInteger(field string, args []string, line int) {
	if len(args) == 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, field, line, fmt.Sprintf("%s 缺少取值", field))
		return
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || parsed < 0 {
		s.errorf(OpenVPNDiagUnsupportedValue, field, line, fmt.Sprintf("%s 必须是非负整数", field))
		return
	}
	if previous, exists := s.singleValues[field]; exists {
		if previous != strconv.Itoa(parsed) {
			s.errorf(OpenVPNDiagConflictingValue, field, line, fmt.Sprintf("%s 出现互相冲突的取值", field))
		}
		return
	}
	s.singleValues[field] = strconv.Itoa(parsed)
	s.result.ProtocolJSON[field] = parsed
	s.source(field, line)
}

func (s *openvpnParseState) consumePeerInfo(args []string, line int) {
	if len(args) < 2 {
		s.errorf(OpenVPNDiagUnsupportedValue, "peer-info", line, "peer-info 需要键与值")
		return
	}
	key := strings.TrimSpace(args[0])
	value := strings.Join(args[1:], " ")
	if key == "" {
		s.errorf(OpenVPNDiagUnsupportedValue, "peer-info", line, "peer-info 键不能为空")
		return
	}
	peerInfo, ok := s.result.ProtocolJSON["peer-info"].(map[string]any)
	if !ok {
		peerInfo = map[string]any{}
		s.result.ProtocolJSON["peer-info"] = peerInfo
		s.source("peer-info", line)
	}
	peerInfo[key] = value
}

// consumeAuthUserPass 只标记认证能力：既不读取引用文件，也不制造用户名与密码。
func (s *openvpnParseState) consumeAuthUserPass(args []string, line int) {
	s.authUserPass = true
	s.source("auth-user-pass", line)
	if len(args) > 0 {
		s.warnf(OpenVPNDiagAuthUserPassFile, "auth-user-pass", line,
			"auth-user-pass 引用的文件不会被读取，用户名与密码需要在表单中填写")
	}
}

// finishAuth 推导认证与 TLS key 模式，并对真实冲突给出阻断诊断。
func (s *openvpnParseState) finishAuth() {
	if s.certSeen != s.keySeen {
		s.errorf(OpenVPNDiagConflictingAuth, "cert", 0, "cert 与 key 必须成对出现；只有一半时无法确定认证模式")
	}
	if len(s.tlsKeys) > 1 {
		s.errorf(OpenVPNDiagConflictingTLSKey, "tls-key-mode", 0,
			"tls-auth／tls-crypt／tls-crypt-v2 互斥，不能同时出现")
	}
	s.result.Selectors = map[string]string{}
	switch {
	case s.authUserPass && s.certSeen && s.keySeen:
		s.result.Selectors["auth_mode"] = "cert_userpass"
	case s.certSeen && s.keySeen:
		s.result.Selectors["auth_mode"] = "cert"
	case s.authUserPass:
		s.result.Selectors["auth_mode"] = "userpass"
	default:
		s.result.Selectors["auth_mode"] = "userpass"
		s.warnf(OpenVPNDiagNoAuthDirective, "auth-mode", 0,
			"未发现 auth-user-pass 或完整的 cert/key，保存前必须补充认证信息")
	}
	switch {
	case s.tlsKeys["tls-crypt-v2"]:
		s.result.Selectors["tls_key_mode"] = "tls_crypt_v2"
	case s.tlsKeys["tls-crypt"]:
		s.result.Selectors["tls_key_mode"] = "tls_crypt"
	case s.tlsKeys["tls-auth"]:
		s.result.Selectors["tls_key_mode"] = "tls_auth"
	default:
		s.result.Selectors["tls_key_mode"] = "none"
	}
}

// finish 在存在阻断诊断时清空可应用草稿，只保留诊断与稳定 code。
func (s *openvpnParseState) finish() (*OpenVPNParseResult, error) {
	if s.firstCode == "" {
		return s.result, nil
	}
	result := &OpenVPNParseResult{
		ProtocolJSON: map[string]any{},
		Diagnostics:  s.result.Diagnostics,
	}
	return result, &OpenVPNParseError{Code: s.firstCode, Message: "`.ovpn` 解析被阻断"}
}
