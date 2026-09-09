package xray

import (
	"testing"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/common/serial"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/vless"
	vlessinbound "github.com/xtls/xray-core/proxy/vless/inbound"
)

func TestBuildProtocolJSONExtractsFlowAndCipher(t *testing.T) {
	vlessCfg := &vlessinbound.Config{Clients: []*protocol.User{{
		Email:   "a@b.c",
		Account: serial.ToTypedMessage(&vless.Account{Id: "uuid", Flow: "xtls-rprx-vision", Encryption: "none"}),
	}}}
	ssCfg := &shadowsocks.ServerConfig{Users: []*protocol.User{{
		Email:   "a@b.c",
		Account: serial.ToTypedMessage(&shadowsocks.Account{Password: "p", CipherType: shadowsocks.CipherType_AES_256_GCM}),
	}}}

	vout := buildProtocolJSON(serial.ToTypedMessage(vlessCfg), nil)
	if vout["flow"] != "xtls-rprx-vision" {
		t.Fatalf("vless flow 未提取: %+v", vout)
	}
	sout := buildProtocolJSON(serial.ToTypedMessage(ssCfg), nil)
	if sout["cipher"] != "aes-256-gcm" {
		t.Fatalf("ss cipher 未提取: %+v", sout)
	}
}

func TestProtocolFromTypeUnknown(t *testing.T) {
	cases := map[string]string{
		"type.googleapis.com/xray.proxy.http.inbound.Config":     "http",
		"type.googleapis.com/xray.proxy.dokodemo.inbound.Config": "dokodemo",
		"type.googleapis.com/xray.proxy.blackhole.Config":        "blackhole",
	}
	for typ, want := range cases {
		if got := protocolFromType(typ); got != want {
			t.Errorf("protocolFromType(%q)=%q want %q", typ, got, want)
		}
	}
}

func TestNormalizeDetectedFields(t *testing.T) {
	m := map[string]any{
		"ws-path":     "/legacy",
		"ws-host":     "cdn.example.com",
		"serviceName": "svc",
		"path":        "/legacy",
		"host":        "cdn.example.com",
	}
	normalizeDetectedFields(m)
	ws := m["ws-opts"].(map[string]any)
	if ws["path"] != "/legacy" {
		t.Fatalf("ws-path 未映射: %#v", ws)
	}
	headers := ws["headers"].(map[string]any)
	if headers["Host"] != "cdn.example.com" {
		t.Fatalf("ws-host 未映射: %#v", headers)
	}
	grpc := m["grpc-opts"].(map[string]any)
	if grpc["grpc-service-name"] != "svc" {
		t.Fatalf("serviceName 未映射: %#v", grpc)
	}
	for _, key := range []string{"ws-path", "ws-host", "serviceName", "path", "host"} {
		if _, exists := m[key]; exists {
			t.Fatalf("旧字段未清理: %s", key)
		}
	}
}
