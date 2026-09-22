package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"vpn-sub/internal/assembly"
	"vpn-sub/internal/log"
	"vpn-sub/internal/node"
)

func TestNodeProtocolsExposeOnlyCurrentEditorFields(t *testing.T) {
	engine, st, cfg := newAssemblyTestEnv(t)
	nodeSvc := node.NewService(st, cfg, log.New("error", "console"))
	noop := func(c *gin.Context) { c.Next() }
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, noop, noop)
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/admin/nodes/protocols", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("协议接口失败: %d", w.Code)
	}
	var response struct {
		Data struct {
			List []node.Protocol `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data.List) != 19 {
		t.Fatalf("手动协议入口数量改变: %d", len(response.Data.List))
	}
	for _, proto := range response.Data.List {
		fields := make(map[string]node.FieldSchema)
		for _, field := range proto.FormSchema {
			fields[field.Name] = field
		}
		switch proto.Protocol {
		case "vless", "vmess", "trojan":
			network := fields["network"]
			if network.AllowCustom == nil || !*network.AllowCustom {
				t.Errorf("%s 传输字段应明确允许自定义: %+v", proto.Protocol, network)
			}
			for _, name := range []string{"ws-path", "ws-headers", "tls"} {
				if _, exists := fields[name]; exists {
					t.Errorf("%s 表单不应包含旧入口 %s", proto.Protocol, name)
				}
			}
			for _, name := range []string{"ws-opts", "grpc-opts"} {
				if fields[name].When == nil || len(fields[name].ResetOn) == 0 {
					t.Errorf("%s 规范传输字段条件/清空归属丢失: %s", proto.Protocol, name)
				}
			}
			if proto.Protocol != "trojan" {
				security := fields["security"]
				if security.Type != "select" {
					t.Errorf("%s 缺少统一安全选择", proto.Protocol)
				}
				if security.AllowCustom == nil || *security.AllowCustom {
					t.Errorf("%s 安全字段应在真实接口中明确禁止自定义: %+v", proto.Protocol, security)
				}
			}
		case "http":
			if fields["tls"].Type != "bool" {
				t.Errorf("%s 有效 TLS 开关被删除", proto.Protocol)
			}
			mode := fields["auth-mode"]
			if !mode.StateOnly || mode.SelectorName != "auth_mode" || mode.Type != "select" {
				t.Errorf("HTTP 认证模式必须为 state_only select selector: %+v", mode)
			}
			for _, name := range []string{"username", "password"} {
				if fields[name].When == nil || fields[name].RequiredWhen == nil || !slices.Contains(fields[name].ResetOn, "selector.auth_mode") {
					t.Errorf("HTTP %s 缺少 basic 条件必填或 selector 清空归属: %+v", name, fields[name])
				}
			}
			for _, name := range []string{"sni", "skip-cert-verify", "name-cert-verify", "fingerprint", "certificate", "private-key"} {
				field, exists := fields[name]
				if !exists {
					t.Errorf("HTTP 缺少 TLS 子字段 %s", name)
					continue
				}
				if field.When == nil || !slices.Contains(field.When.Features, "tls") || !slices.Contains(field.ResetOn, "feature.tls") {
					t.Errorf("HTTP TLS 子字段 %s 缺少 feature 条件或清空归属: %+v", name, field)
				}
			}
			if fields["headers"].MapValueType != "string" || fields["headers"].ObjectKind != "map" {
				t.Errorf("HTTP headers 必须为字符串开放 Map: %+v", fields["headers"])
			}
		case "socks5":
			if fields["tls"].Type != "bool" {
				t.Errorf("%s 有效 TLS 开关被删除", proto.Protocol)
			}
		case "ssh":
			mode := fields["auth-mode"]
			if !mode.StateOnly || mode.SelectorName != "auth_mode" || mode.Type != "select" {
				t.Errorf("SSH 认证模式必须为 state_only select selector: %+v", mode)
			}
			if fields["username"].Required != true {
				t.Errorf("SSH 用户名为无条件必填: %+v", fields["username"])
			}
			if fields["password"].When == nil || !slices.Contains(fields["password"].When.Selectors["auth_mode"], "password") {
				t.Errorf("SSH password 必须只在 password 分支活动: %+v", fields["password"])
			}
			if fields["private-key"].When == nil || !slices.Contains(fields["private-key"].When.Selectors["auth_mode"], "private_key") {
				t.Errorf("SSH private-key 必须只在 private_key 分支活动: %+v", fields["private-key"])
			}
			for _, name := range []string{"password", "private-key", "private-key-passphrase"} {
				if !slices.Contains(fields[name].ResetOn, "selector.auth_mode") {
					t.Errorf("SSH %s 缺少 selector.auth_mode 清空归属: %+v", name, fields[name])
				}
			}
			for _, name := range []string{"host-key", "host-key-algorithms"} {
				if fields[name].Type != "text-list" {
					t.Errorf("SSH %s 必须为结构化列表: %+v", name, fields[name])
				}
			}
		case "snell":
			versionField, ok := fields["version"]
			if !ok || versionField.SelectorName != "version" || len(versionField.Options) != 5 {
				t.Errorf("Snell version 必须为 1～5 的普通 selector 来源字段: %+v", versionField)
			}
			mode := fields["obfs-mode"]
			if !mode.StateOnly || mode.SelectorName != "obfs_mode" || mode.Type != "select" {
				t.Errorf("Snell 混淆模式必须为 state_only select selector: %+v", mode)
			}
			if fields["udp"].When == nil || !slices.Contains(fields["udp"].When.Selectors["version"], "3") {
				t.Errorf("Snell udp 必须只在 v3 起活动: %+v", fields["udp"])
			}
			if fields["reuse"].When == nil || !slices.Contains(fields["reuse"].When.Selectors["version"], "4") {
				t.Errorf("Snell reuse 必须只在 v4/v5 活动: %+v", fields["reuse"])
			}
			obfs := fields["obfs-opts"]
			if obfs.Type != "object" || obfs.ObjectKind != "fields" || obfs.AllowUnknown {
				t.Errorf("Snell obfs-opts 必须为拒绝未知键的固定对象: %+v", obfs)
			}
			for _, name := range []string{"host", "password", "version-hint", "username"} {
				var found *node.FieldSchema
				for i := range obfs.Properties {
					if obfs.Properties[i].Name == name {
						found = &obfs.Properties[i]
					}
				}
				if found == nil || found.When == nil || !slices.Contains(found.ResetOn, "selector.obfs_mode") {
					t.Errorf("Snell obfs-opts.%s 缺少模式条件或清空归属: %+v", name, found)
				}
			}
			if fields["client-fingerprint"].When == nil || !slices.Contains(fields["client-fingerprint"].When.Selectors["obfs_mode"], "shadow_tls") {
				t.Errorf("Snell client-fingerprint 必须只在伪装分支活动: %+v", fields["client-fingerprint"])
			}
		case "hysteria2":
			for fieldName, want := range map[string]struct{ selector, value string }{
				"endpoint-mode": {"endpoint_mode", "single"},
				"obfs-mode":     {"obfs_mode", "none"},
			} {
				selector, ok := fields[fieldName]
				if !ok || !selector.StateOnly || selector.SelectorName != want.selector || selector.Default != want.value {
					t.Errorf("Hysteria2 %s 必须为 state_only selector: %+v", fieldName, selector)
				}
			}
			if len(proto.EndpointPolicies) != 2 {
				t.Fatalf("Hysteria2 必须声明 single／ports 两条 endpoint policy: %+v", proto.EndpointPolicies)
			}
			if field, exists := fields["ports"]; !exists || field.RequiredWhen == nil {
				t.Errorf("Hysteria2 ports 必须在 ports 模式条件必填: %+v", field)
			}
			if field, exists := fields["hop-interval"]; !exists || field.When == nil {
				t.Errorf("Hysteria2 hop-interval 必须只在 ports 模式活动: %+v", field)
			}
			if field, exists := fields["obfs-password"]; !exists || field.RequiredWhen == nil {
				t.Errorf("Hysteria2 obfs-password 必须在混淆分支条件必填: %+v", field)
			}
			for _, name := range []string{"obfs-min-packet-size", "obfs-max-packet-size"} {
				if field, exists := fields[name]; !exists || field.When == nil || !slices.Contains(field.When.Selectors["obfs_mode"], "gecko") {
					t.Errorf("Hysteria2 %s 必须只在 gecko 分支活动: %+v", name, field)
				}
			}
			realm := fields["realm-opts"]
			if realm.Type != "object" || realm.Feature == nil || realm.Feature.Toggle != "enable" {
				t.Errorf("Hysteria2 realm-opts 必须为以 enable 控制的 Realm 对象: %+v", realm)
			}
			for _, legacy := range []string{"ca", "ca-str", "protocol", "obfs-protocol", "obfs"} {
				if _, exists := fields[legacy]; exists {
					t.Errorf("Hysteria2 不应把历史字段 %s 保留为可保存入口", legacy)
				}
			}
		case "hysteria":
			mode := fields["auth-mode"]
			if !mode.StateOnly || mode.SelectorName != "auth_mode" || mode.Type != "select" {
				t.Errorf("Hysteria 认证方式必须为 state_only select selector: %+v", mode)
			}
			for _, name := range []string{"auth", "auth-str"} {
				field, exists := fields[name]
				if !exists || field.When == nil || field.RequiredWhen == nil || !slices.Contains(field.ResetOn, "selector.auth_mode") {
					t.Errorf("Hysteria %s 缺少分支条件、条件必填或清空归属: %+v", name, field)
				}
			}
			for _, name := range []string{"up", "down"} {
				if field, exists := fields[name]; !exists || !field.Required {
					t.Errorf("Hysteria %s 必须为无条件必填带宽入口: %+v", name, field)
				}
			}
			if field, exists := fields["protocol"]; !exists || len(field.Options) != 3 {
				t.Errorf("Hysteria protocol 必须为固定 tag 支持的枚举: %+v", field)
			}
			for _, legacy := range []string{"obfs-protocol", "up-speed", "down-speed", "ca", "ca-str"} {
				if _, exists := fields[legacy]; exists {
					t.Errorf("Hysteria 不应把历史字段 %s 保留为可保存入口", legacy)
				}
			}
			if ech := fields["ech-opts"]; ech.Type != "object" || ech.Feature == nil || ech.Feature.Toggle != "enable" {
				t.Errorf("Hysteria ech-opts 必须为以 enable 控制的 ECH 对象: %+v", ech)
			}
		case "tuic":
			mode := fields["auth-mode"]
			if !mode.StateOnly || mode.SelectorName != "auth_mode" || mode.Type != "select" {
				t.Errorf("TUIC 认证方式必须为 state_only select selector: %+v", mode)
			}
			for _, name := range []string{"token", "uuid", "password"} {
				field, exists := fields[name]
				if !exists || field.When == nil || field.RequiredWhen == nil || !slices.Contains(field.ResetOn, "selector.auth_mode") {
					t.Errorf("TUIC %s 缺少分支条件、条件必填或清空归属: %+v", name, field)
				}
			}
			if relay := fields["udp-relay-mode"]; len(relay.Options) != 2 {
				t.Errorf("TUIC udp-relay-mode 必须为固定 tag 支持的枚举: %+v", relay)
			}
			version := fields["udp-over-stream-version"]
			if version.Type != "select" || len(version.Options) != 2 || version.When == nil || !slices.Contains(version.When.Features, "udp-over-stream") {
				t.Errorf("TUIC UOT 版本必须为只在 UOT 开启时活动的两值枚举: %+v", version)
			}
			for _, legacy := range []string{"ca", "ca-str"} {
				if _, exists := fields[legacy]; exists {
					t.Errorf("TUIC 不应把历史字段 %s 保留为可保存入口", legacy)
				}
			}
			for _, name := range []string{"name-cert-verify", "certificate", "private-key", "ech-opts", "bbr-profile"} {
				if _, exists := fields[name]; !exists {
					t.Errorf("TUIC 缺少固定 tag 字段 %s", name)
				}
			}
		case "ss":
			for _, name := range []string{"cipher", "plugin"} {
				field := fields[name]
				if field.AllowCustom == nil || !*field.AllowCustom {
					t.Errorf("SS %s 应明确允许自定义: %+v", name, field)
				}
			}
			found := false
			for _, field := range fields["v2ray-plugin-opts"].Properties {
				found = found || field.Name == "tls" && field.Type == "bool"
			}
			if !found {
				t.Error("SS 插件自身的 TLS 开关被删除")
			}
			v2rayFields := make(map[string]node.FieldSchema)
			for _, field := range fields["v2ray-plugin-opts"].Properties {
				v2rayFields[field.Name] = field
			}
			shadowFields := make(map[string]node.FieldSchema)
			for _, field := range fields["shadow-tls-opts"].Properties {
				shadowFields[field.Name] = field
			}
			restlsFields := make(map[string]node.FieldSchema)
			for _, field := range fields["restls-opts"].Properties {
				restlsFields[field.Name] = field
			}
			if v2rayFields["private-key"].Type != "secret-multiline" || shadowFields["private-key"].Type != "secret-multiline" ||
				v2rayFields["certificate"].Type != "multiline" || shadowFields["certificate"].Type != "multiline" ||
				restlsFields["restls-script"].Type != "multiline" ||
				shadowFields["host"].Required || restlsFields["host"].Required {
				t.Errorf("SS 固定插件字段类型或目标限定必填被错误投影: v2ray=%+v shadow=%+v restls=%+v", v2rayFields, shadowFields, restlsFields)
			}
			if _, exists := v2rayFields["version"]; exists {
				t.Error("v2ray-plugin version 不应继续标记为固定版本字段")
			}
			if _, exists := restlsFields["path"]; exists {
				t.Error("restls path 不应继续标记为固定版本字段")
			}
			for _, path := range []string{"v2ray-plugin-opts.private-key", "shadow-tls-opts.private-key"} {
				if !slices.Contains(proto.SensitiveFields, path) {
					t.Errorf("协议接口缺少 SS 固定敏感路径 %s: %v", path, proto.SensitiveFields)
				}
			}
			custom := fields["plugin-opts"]
			if custom.ObjectKind != "map" || custom.MapValueType != "string" || custom.When == nil || len(custom.When.PluginNot) != 5 || !slices.Contains(custom.ResetOn, "plugin") {
				t.Errorf("SS 未知插件参数元数据缺失: %+v", custom)
			}
		case "shadowquic":
			// Build32 Step 16：凭据成对必填、TLS 只暴露 sni／alpn、QUIC 版本与流控字段。
			for _, name := range []string{"username", "password", "sni", "alpn", "quic-versions",
				"udp-over-stream", "zero-rtt", "keep-alive-interval", "congestion-controller",
				"up", "down", "cwnd", "recv-window-conn", "recv-window", "disable-mtu-discovery",
				"max-datagram-frame-size", "max-open-streams"} {
				if _, exists := fields[name]; !exists {
					t.Errorf("ShadowQUIC 缺少字段 %s", name)
				}
			}
			for _, forbidden := range []string{"skip-cert-verify", "certificate", "private-key", "ech-opts", "udp-over-stream-version"} {
				if _, exists := fields[forbidden]; exists {
					t.Errorf("ShadowQUIC 不得声明 %s", forbidden)
				}
			}
			if !fields["username"].Required || !fields["password"].Required {
				t.Error("ShadowQUIC username／password 必须必填")
			}
			if !slices.Contains(proto.SensitiveFields, "password") {
				t.Errorf("ShadowQUIC password 必须是敏感字段: %v", proto.SensitiveFields)
			}
			if proto.LinkMappings.SR || proto.LinkMappings.Generic {
				t.Errorf("ShadowQUIC 无 URI 映射: %+v", proto.LinkMappings)
			}
		case "anytls":
			// Build32 Step 15：security_mode selector、三种伪装互斥与主密码不进入 selector 清空域。
			if len(proto.Selectors) != 1 || proto.Selectors[0].Name != "security_mode" || proto.Selectors[0].Default != "plain" {
				t.Errorf("AnyTLS 缺少 security_mode selector: %+v", proto.Selectors)
			}
			if got := strings.Join(proto.Selectors[0].Values, ","); got != "plain,shadow_tls,restls,jls" {
				t.Errorf("AnyTLS security_mode 允许值异常: %s", got)
			}
			for _, name := range []string{"shadow-tls-opts", "restls-opts", "jls-opts"} {
				if fields[name].When == nil || !slices.Contains(fields[name].ResetOn, "selector.security_mode") {
					t.Errorf("AnyTLS %s 必须按 security_mode 分支清空: %+v", name, fields[name])
				}
			}
			if slices.Contains(fields["password"].ResetOn, "selector.security_mode") {
				t.Error("AnyTLS 主 password 不得随 security_mode 切换清空")
			}
			if _, exists := fields["reality-opts"]; exists {
				t.Error("AnyTLS 不得开放 Reality")
			}
		case "tailscale":
			// Build32 Step 14：无 endpoint、三态 bool、state-dir 不进入 schema。
			if len(proto.EndpointPolicies) != 1 || proto.EndpointPolicies[0].HostMode != "hidden" || proto.EndpointPolicies[0].PortMode != "hidden" {
				t.Errorf("Tailscale 必须固定隐藏 endpoint: %+v", proto.EndpointPolicies)
			}
			for _, name := range []string{"accept-routes", "exit-node-allow-lan-access"} {
				if fields[name].Type != "bool" || fields[name].Default != nil {
					t.Errorf("Tailscale %s 必须保留 unset/false/true 三态: %+v", name, fields[name])
				}
			}
			if fields["exit-node-allow-lan-access"].When == nil || len(fields["exit-node-allow-lan-access"].When.NonEmpty) != 1 {
				t.Errorf("Tailscale LAN access 必须声明 non_empty 依赖: %+v", fields["exit-node-allow-lan-access"].When)
			}
			if _, exists := fields["state-dir"]; exists {
				t.Error("state-dir 必须由 adapter 派生，不得进入 schema")
			}
			if !slices.Contains(proto.SensitiveFields, "auth-key") {
				t.Errorf("Tailscale auth-key 必须是敏感字段: %v", proto.SensitiveFields)
			}
		case "masque":
			// Build32 Step 13：network_mode selector、三种网络模式与 h3-l4proxy 强制关闭 UDP。
			if len(proto.Selectors) != 1 || proto.Selectors[0].Name != "network_mode" || proto.Selectors[0].Default != "quic" {
				t.Errorf("MASQUE 缺少 network_mode selector: %+v", proto.Selectors)
			}
			if got := strings.Join(proto.Selectors[0].Values, ","); got != "quic,h2,h3_l4proxy" {
				t.Errorf("MASQUE network_mode 允许值异常: %s", got)
			}
			for _, name := range []string{"udp", "congestion-controller", "cwnd", "bbr-profile"} {
				if fields[name].When == nil || !slices.Contains(fields[name].ResetOn, "selector.network_mode") {
					t.Errorf("MASQUE %s 必须按 network_mode 分支清空: %+v", name, fields[name])
				}
			}
			if _, exists := fields["name-cert-verify"]; exists {
				t.Error("MASQUE name-cert-verify 只是固定 tag placeholder，不得进入 schema")
			}
			if fields["ip-stack"].ObjectKind != "fields" {
				t.Errorf("MASQUE ip-stack 必须复用共享枚举合同: %+v", fields["ip-stack"])
			}
		case "mieru":
			// Build32 Step 12：endpoint_mode selector、single／range endpoint policy 与完整枚举。
			if len(proto.Selectors) != 1 || proto.Selectors[0].Name != "endpoint_mode" || proto.Selectors[0].Default != "single" {
				t.Errorf("Mieru 缺少 endpoint_mode selector: %+v", proto.Selectors)
			}
			if len(proto.EndpointPolicies) != 2 {
				t.Errorf("Mieru 必须声明 single／range 两条 endpoint policy: %+v", proto.EndpointPolicies)
			}
			if got := strings.Join(fields["multiplexing"].Options, ","); got != ",MULTIPLEXING_OFF,MULTIPLEXING_LOW,MULTIPLEXING_MIDDLE,MULTIPLEXING_HIGH" {
				t.Errorf("Mieru multiplexing 必须是完整上游常量: %s", got)
			}
			if got := strings.Join(fields["handshake-mode"].Options, ","); got != ",HANDSHAKE_STANDARD,HANDSHAKE_NO_WAIT" {
				t.Errorf("Mieru handshake-mode 必须是完整上游常量: %s", got)
			}
			if fields["port-range"].When == nil || fields["port-range"].RequiredWhen == nil {
				t.Errorf("Mieru port-range 必须只在 range 模式活动且条件必填: %+v", fields["port-range"])
			}
		case "wireguard":
			if fields["peers"].ItemIDField != "_credential_id" || !slices.Contains(proto.SensitiveFields, "peers[].pre-shared-key") {
				t.Errorf("WireGuard Peer 稳定身份/敏感路径契约缺失: %+v", fields["peers"])
			}
			// Build32 Step 11：peer_mode selector、single／peers endpoint policy、reserved 三字节与 AmneziaWG 排除。
			if len(proto.Selectors) != 1 || proto.Selectors[0].Name != "peer_mode" || proto.Selectors[0].Default != "single" {
				t.Errorf("WireGuard 缺少 peer_mode selector: %+v", proto.Selectors)
			}
			if len(proto.EndpointPolicies) != 2 {
				t.Errorf("WireGuard 必须声明 single／peers 两条 endpoint policy: %+v", proto.EndpointPolicies)
			}
			if fields["reserved"].Type != "byte-sequence" || fields["ip-stack"].ObjectKind != "fields" {
				t.Errorf("WireGuard reserved／ip-stack 元数据缺失: %+v", fields)
			}
			for name := range fields {
				if strings.Contains(strings.ToLower(name), "amnezia") {
					t.Errorf("WireGuard 不得声明 AmneziaWG 活动字段: %s", name)
				}
			}
		}
	}
	// 接口投影不得改变保存、校验和输出使用的内部注册表。
	for _, protocol := range []string{"vless", "vmess"} {
		proto, err := node.GetProtocol(protocol)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, field := range proto.FormSchema {
			found = found || field.Name == "tls"
		}
		if !found {
			t.Errorf("%s 内部 TLS 字段被表单投影删除", protocol)
		}
	}
}

func TestNodeUpdateRevisionConflictResponse(t *testing.T) {
	engine, st, cfg := newAssemblyTestEnv(t)
	lg := log.New("error", "console")
	nodeSvc := node.NewService(st, cfg, lg)
	noop := func(c *gin.Context) { c.Next() }
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, noop, noop)

	created, err := nodeSvc.CreateManual(context.Background(), node.CreateManualInput{
		Name: "API修订节点", Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "11111111-2222-3333-4444-555555555555", "network": "tcp",
		},
	})
	if err != nil {
		t.Fatalf("创建测试节点失败: %v", err)
	}
	body, err := json.Marshal(node.UpdateManualInput{
		Protocol: "vless", Host: "stale.example.com", Port: 443, BaseRevision: 0,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
	})
	if err != nil {
		t.Fatalf("序列化更新请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/nodes/%d", created.ID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("旧修订 API 状态码异常: %d, body=%s", w.Code, w.Body.String())
	}
	var conflict struct {
		Error           string `json:"error"`
		Code            string `json:"code"`
		CurrentRevision int64  `json:"current_revision"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &conflict); err != nil {
		t.Fatalf("解析修订冲突响应失败: %v", err)
	}
	if conflict.Code != "revision_conflict" || conflict.CurrentRevision != 1 || conflict.Error == "" {
		t.Fatalf("修订冲突响应异常: %+v", conflict)
	}

	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/nodes/%d", created.ID), nil)
	getW := httptest.NewRecorder()
	engine.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("节点详情 API 状态码异常: %d, body=%s", getW.Code, getW.Body.String())
	}
	var detail struct {
		Code int       `json:"code"`
		Data node.Node `json:"data"`
	}
	if err := json.Unmarshal(getW.Body.Bytes(), &detail); err != nil {
		t.Fatalf("解析节点详情响应失败: %v", err)
	}
	if detail.Code != 0 || detail.Data.EditRevision != 1 || detail.Data.ID != created.ID {
		t.Fatalf("节点详情响应未返回当前修订: %+v", detail)
	}
}

func TestNodeCheckRouteUsesAdapterAndDoesNotWrite(t *testing.T) {
	engine, st, cfg := newAssemblyTestEnv(t)
	lg := log.New("error", "console")
	nodeSvc := node.NewService(st, cfg, lg)
	assemblySvc := assembly.NewService(st, cfg, lg)
	nodeSvc.SetCheckRenderer(assemblySvc.CheckNodeTarget)
	noop := func(c *gin.Context) { c.Next() }
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, noop, noop)

	request := node.CheckRequest{
		Protocol: "vless", Host: "example.com", Port: 443,
		ProtocolJSON: map[string]any{
			"uuid": "route-check-secret", "network": "ws", "tls": true,
			"ws-opts": map[string]any{"path": "/check"},
		},
		Targets: []string{"clash-yaml", "generic-subs"},
	}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("序列化节点检查请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/admin/nodes/check", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("节点检查路由状态码异常: %d, body=%s", w.Code, w.Body.String())
	}
	var response struct {
		Code int                `json:"code"`
		Data node.CheckResponse `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("解析节点检查响应失败: %v", err)
	}
	if response.Code != 0 || response.Data.CheckID == "" || response.Data.CheckVersion != 1 {
		t.Fatalf("节点检查响应基本结构异常: %+v", response)
	}
	for _, target := range []string{"clash-yaml", "generic-subs"} {
		result, ok := response.Data.Targets[target]
		if !ok || result.Status != "ok" || result.Preview == nil {
			t.Fatalf("节点检查目标结果异常: target=%s result=%+v", target, result)
		}
		if result.Diagnostics == nil {
			t.Fatalf("节点检查 HTTP 响应的空诊断应为数组: target=%s body=%s", target, w.Body.String())
		}
	}
	if strings.Contains(w.Body.String(), `"diagnostics":null`) {
		t.Fatalf("节点检查 HTTP 响应不应包含 null 诊断: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "route-check-secret") {
		t.Fatalf("节点检查 HTTP 响应泄漏凭据: %s", w.Body.String())
	}
	var count int
	if err := st.DB().QueryRowContext(context.Background(), `SELECT COUNT(*) FROM nodes`).Scan(&count); err != nil {
		t.Fatalf("读取检查后节点数量失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("节点检查不应写入数据库: count=%d", count)
	}
}

func TestNodeUpdateForbiddenReturns403(t *testing.T) {
	engine, st, cfg := newAssemblyTestEnv(t)
	lg := log.New("error", "console")
	nodeSvc := node.NewService(st, cfg, lg)
	noop := func(c *gin.Context) { c.Next() }
	RegisterNodeRoutes(engine, &NodeHandler{nodeSvc: nodeSvc}, noop, noop)

	ctx := context.Background()
	res, err := st.DB().ExecContext(ctx,
		`INSERT INTO xray_instances (name, slug, api_addr) VALUES ('API实例','api-xray','https://example.com')`)
	if err != nil {
		t.Fatalf("插入 Xray 实例失败: %v", err)
	}
	instID, _ := res.LastInsertId()
	res, err = st.DB().ExecContext(ctx,
		`INSERT INTO nodes (source,name,instance_id,tag,protocol,host,port,protocol_json,allocatable,missing)
		 VALUES ('xray','api-xray-vless',?,'inbound','vless','example.com',443,'{}',1,0)`, instID)
	if err != nil {
		t.Fatalf("插入 Xray 节点失败: %v", err)
	}
	nodeID, _ := res.LastInsertId()

	body, err := json.Marshal(node.UpdateManualInput{
		Protocol: "vless", Host: "new.example.com", Port: 443, BaseRevision: 0,
		ProtocolJSON: map[string]any{"uuid": "", "network": "tcp"},
	})
	if err != nil {
		t.Fatalf("序列化更新请求失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/admin/nodes/%d", nodeID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("禁止编辑 Xray 节点应返回 403，实际 %d, body=%s", w.Code, w.Body.String())
	}
}
