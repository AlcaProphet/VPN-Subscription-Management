// shadowquic_protocol_test.go：Build32 Step 16 ShadowQUIC 协议合同测试。
// 覆盖用户名／密码成对、TLS 只暴露 sni／alpn、QUIC 版本有序去重、UOT 与 0-RTT 独立 bool、
// 非负整数窗口／速率校验，以及不误加通用 TLS helper 字段的边界。
package node

import (
	"context"
	"strings"
	"testing"
)

func shadowquicProto(t *testing.T) Protocol {
	t.Helper()
	proto, err := GetProtocol("shadowquic")
	if err != nil {
		t.Fatal(err)
	}
	return proto
}

func shadowquicField(t *testing.T, name string) FieldSchema {
	t.Helper()
	for _, field := range shadowquicProto(t).FormSchema {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("ShadowQUIC schema 缺少字段 %s", name)
	return FieldSchema{}
}

func shadowquicBaseParams() map[string]any {
	return map[string]any{"username": "squic-user", "password": "squic-secret"}
}

func createShadowQUIC(t *testing.T, svc *Service, name string, params map[string]any) (*Node, error) {
	t.Helper()
	return svc.CreateManual(context.Background(), CreateManualInput{
		Name: name, Protocol: "shadowquic", Host: "example.com", Port: 443,
		ProtocolJSON: params,
	})
}

// TestShadowQUICCredentialPair 覆盖用户名／密码成对必填与 password 敏感。
func TestShadowQUICCredentialPair(t *testing.T) {
	proto := shadowquicProto(t)
	for _, name := range []string{"username", "password"} {
		field := shadowquicField(t, name)
		if !field.Required {
			t.Fatalf("ShadowQUIC %s 必须必填: %+v", name, field)
		}
	}
	if !contains(proto.SensitiveFields, "password") {
		t.Fatalf("ShadowQUIC password 必须是敏感字段: %v", proto.SensitiveFields)
	}
	svc, _, _ := newTestService(t)
	for _, tc := range []struct {
		name   string
		params map[string]any
		want   string
	}{
		{name: "missing username", params: map[string]any{"password": "p"}, want: "username"},
		{name: "missing password", params: map[string]any{"username": "u"}, want: "password"},
		{name: "blank username", params: map[string]any{"username": "   ", "password": "p"}, want: "username"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := createShadowQUIC(t, svc, "squic-"+strings.ReplaceAll(tc.name, " ", "-"), tc.params)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%s 必须按 %s 拒绝，实际: %v", tc.name, tc.want, err)
			}
		})
	}
}

// TestShadowQUICTLSBoundary 覆盖 TLS 只暴露 sni／alpn，不误加通用 TLS helper 字段。
func TestShadowQUICTLSBoundary(t *testing.T) {
	proto := shadowquicProto(t)
	for _, forbidden := range []string{"skip-cert-verify", "certificate", "private-key", "ech-opts", "fingerprint", "name-cert-verify", "client-fingerprint"} {
		if _, ok := findSchemaField(proto.FormSchema, forbidden); ok {
			t.Fatalf("ShadowQUIC 固定 tag 没有 %s，schema 不得声明", forbidden)
		}
	}
	for _, name := range []string{"sni", "alpn"} {
		if _, ok := findSchemaField(proto.FormSchema, name); !ok {
			t.Fatalf("ShadowQUIC 必须暴露 %s", name)
		}
	}
	svc, _, _ := newTestService(t)
	params := shadowquicBaseParams()
	params["skip-cert-verify"] = true
	if _, err := createShadowQUIC(t, svc, "squic-forbidden-tls", params); err == nil {
		t.Fatal("固定 tag 之外的 TLS 字段必须被顶层白名单拒绝")
	}
}

// TestShadowQUICVersionsNormalization 覆盖 quic-versions 的有序去重与 v1／v2 限制。
func TestShadowQUICVersionsNormalization(t *testing.T) {
	if got := shadowquicField(t, "quic-versions").Type; got != "text-list" {
		t.Fatalf("quic-versions 必须是 text-list，实际 %s", got)
	}
	svc, _, _ := newTestService(t)

	params := shadowquicBaseParams()
	params["quic-versions"] = []any{"v2", " v1 ", "v2", ""}
	created, err := createShadowQUIC(t, svc, "squic-versions-dedup", params)
	if err != nil {
		t.Fatalf("合法 quic-versions 创建失败: %v", err)
	}
	got := shadowquicTextList(t, created.ProtocolJSON["quic-versions"])
	if strings.Join(got, ",") != "v2,v1" {
		t.Fatalf("quic-versions 必须去空白去重且保序，实际 %#v", got)
	}

	// 空列表表示内核默认，不得强写默认值。
	created, err = createShadowQUIC(t, svc, "squic-versions-empty", shadowquicBaseParams())
	if err != nil {
		t.Fatalf("空 quic-versions 应通过: %v", err)
	}
	if _, exists := created.ProtocolJSON["quic-versions"]; exists {
		t.Fatalf("空 quic-versions 不得落库: %+v", created.ProtocolJSON)
	}

	for _, bad := range [][]any{{"v3"}, {"1"}, {"rfc9000"}, {"v1", "v2", "v0"}, {"quic"}} {
		params := shadowquicBaseParams()
		params["quic-versions"] = bad
		if _, err := createShadowQUIC(t, svc, "squic-versions-bad", params); err == nil || !strings.Contains(err.Error(), "quic-versions") {
			t.Fatalf("非法 quic-versions %#v 必须按字段拒绝，实际: %v", bad, err)
		}
	}
}

func shadowquicTextList(t *testing.T, value any) []string {
	t.Helper()
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				t.Fatalf("列表项不是字符串: %#v", item)
			}
			out = append(out, text)
		}
		return out
	}
	t.Fatalf("列表类型异常: %#v", value)
	return nil
}

// TestShadowQUICUOTAndZeroRTT 覆盖 udp-over-stream／zero-rtt 独立 bool 与无附加版本字段。
func TestShadowQUICUOTAndZeroRTT(t *testing.T) {
	for _, name := range []string{"udp-over-stream", "zero-rtt", "disable-mtu-discovery"} {
		field := shadowquicField(t, name)
		if field.Type != "bool" {
			t.Fatalf("%s 必须是独立 bool: %+v", name, field)
		}
	}
	// 固定 tag 的 ShadowQuicOption 没有 udp-over-stream-version，schema 不得自造。
	for _, forbidden := range []string{"udp-over-stream-version", "udp-over-tcp-version"} {
		if _, ok := findSchemaField(shadowquicProto(t).FormSchema, forbidden); ok {
			t.Fatalf("ShadowQUIC 不得声明 %s", forbidden)
		}
	}
	svc, _, _ := newTestService(t)
	params := shadowquicBaseParams()
	params["udp-over-stream"] = true
	params["zero-rtt"] = true
	params["disable-mtu-discovery"] = true
	created, err := createShadowQUIC(t, svc, "squic-uot-zertt", params)
	if err != nil {
		t.Fatalf("独立 bool 创建失败: %v", err)
	}
	for _, key := range []string{"udp-over-stream", "zero-rtt", "disable-mtu-discovery"} {
		if created.ProtocolJSON[key] != true {
			t.Fatalf("%s 必须独立保存: %+v", key, created.ProtocolJSON)
		}
	}
	if _, exists := created.ProtocolJSON["udp-over-stream-version"]; exists {
		t.Fatalf("关闭／开启 UOT 都不得产生附加版本字段: %+v", created.ProtocolJSON)
	}
}

// TestShadowQUICNonNegativeIntegers 覆盖窗口／保活／流控字段的非负整数约束。
func TestShadowQUICNonNegativeIntegers(t *testing.T) {
	svc, _, _ := newTestService(t)
	names := []string{"keep-alive-interval", "cwnd", "recv-window-conn", "recv-window",
		"max-datagram-frame-size", "max-open-streams"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			params := shadowquicBaseParams()
			params[name] = -1
			if _, err := createShadowQUIC(t, svc, "squic-neg-"+name, params); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s 负数必须按字段拒绝，实际: %v", name, err)
			}
			params[name] = 1.5
			if _, err := createShadowQUIC(t, svc, "squic-frac-"+name, params); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("%s 小数必须按字段拒绝，实际: %v", name, err)
			}
			// 缺省不写库：只有显式设置才落库。
			params[name] = 0
			created, err := createShadowQUIC(t, svc, "squic-zero-"+name, params)
			if err != nil {
				t.Fatalf("%s=0 应通过: %v", name, err)
			}
			value, exists := numberParam(created.ProtocolJSON[name])
			if !exists || value != 0 {
				t.Fatalf("%s 显式 0 必须保留: %+v", name, created.ProtocolJSON)
			}
		})
	}
	// 未设置时不得强写内核默认值。
	created, err := createShadowQUIC(t, svc, "squic-defaults", shadowquicBaseParams())
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	for _, key := range append([]string{"congestion-controller", "bbr-profile", "up", "down"}, names...) {
		if _, exists := created.ProtocolJSON[key]; exists {
			t.Fatalf("未设置的 %s 不得写入数据库: %+v", key, created.ProtocolJSON)
		}
	}
}

// TestShadowQUICBandwidthAndCongestion 覆盖可选速率与拥塞控制器枚举。
func TestShadowQUICBandwidthAndCongestion(t *testing.T) {
	field := shadowquicField(t, "congestion-controller")
	if strings.Join(field.Options, ",") != ",cubic,new_reno,bbr_meta_v1,bbr_meta_v2,bbr" {
		t.Fatalf("congestion-controller 枚举必须与固定 tag 一致: %+v", field.Options)
	}
	svc, _, _ := newTestService(t)
	params := shadowquicBaseParams()
	params["up"] = "100 Mbps"
	params["down"] = "1 Gbps"
	params["congestion-controller"] = "bbr_meta_v2"
	params["bbr-profile"] = "standard"
	created, err := createShadowQUIC(t, svc, "squic-rate-ok", params)
	if err != nil {
		t.Fatalf("合法速率／拥塞创建失败: %v", err)
	}
	if created.ProtocolJSON["up"] != "100 Mbps" || created.ProtocolJSON["down"] != "1 Gbps" {
		t.Fatalf("速率必须原样保存: %+v", created.ProtocolJSON)
	}

	for _, tc := range []struct {
		name  string
		key   string
		value string
	}{
		{name: "up not a rate", key: "up", value: "fast"},
		{name: "down zero", key: "down", value: "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := shadowquicBaseParams()
			invalid[tc.key] = tc.value
			if _, err := createShadowQUIC(t, svc, "squic-rate-"+strings.ReplaceAll(tc.name, " ", "-"), invalid); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("%s=%q 必须按字段拒绝，实际: %v", tc.key, tc.value, err)
			}
		})
	}
	invalid := shadowquicBaseParams()
	invalid["congestion-controller"] = "vegas"
	if _, err := createShadowQUIC(t, svc, "squic-cc-bad", invalid); err == nil || !strings.Contains(err.Error(), "congestion-controller") {
		t.Fatalf("非法拥塞控制器必须按字段拒绝，实际: %v", err)
	}
}

// TestShadowQUICNoURIMapping 覆盖无 URI 映射协议不伪造 URI。
func TestShadowQUICNoURIMapping(t *testing.T) {
	proto := shadowquicProto(t)
	if proto.LinkMappings.SR || proto.LinkMappings.Generic {
		t.Fatalf("ShadowQUIC 不得声明 URI 映射: %+v", proto.LinkMappings)
	}
}
