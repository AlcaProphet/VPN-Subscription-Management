package xray

import (
	"strings"
	"testing"

	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy/shadowsocks"
	"github.com/xtls/xray-core/proxy/trojan"
	"github.com/xtls/xray-core/proxy/vless"
	"github.com/xtls/xray-core/proxy/vmess"
)

func TestUserEmailLowerCase(t *testing.T) {
	got := UserEmail(42)
	if got != "user-42@vpn.local" {
		t.Fatalf("UserEmail(42) = %q", got)
	}
	if got != strings.ToLower(got) {
		t.Fatalf("UserEmail 必须全小写: %q", got)
	}
}

func TestBuildUserVless(t *testing.T) {
	u, err := BuildUser(1, "UUID", "secret", NodeView{Protocol: "vless", Flow: "xtls-rprx-vision", Host: "h", Port: 443})
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "user-1@vpn.local" {
		t.Fatalf("email = %q", u.Email)
	}
	if u.Level != 0 {
		t.Fatalf("level = %d", u.Level)
	}
	if u.Account == nil || !strings.HasSuffix(u.Account.Type, "vless.Account") {
		t.Fatalf("account type = %q", u.Account.GetType())
	}
	inst, err := u.Account.GetInstance()
	if err != nil {
		t.Fatal(err)
	}
	acc, ok := inst.(*vless.Account)
	if !ok {
		t.Fatalf("instance type %T", inst)
	}
	if acc.Id != "uuid" || acc.Flow != "xtls-rprx-vision" || acc.Encryption != "none" {
		t.Fatalf("vless account = %+v", acc)
	}
}

func TestBuildUserVmessNoAlterID(t *testing.T) {
	u, err := BuildUser(2, "uuid", "secret", NodeView{Protocol: "vmess"})
	if err != nil {
		t.Fatal(err)
	}
	inst, err := u.Account.GetInstance()
	if err != nil {
		t.Fatal(err)
	}
	acc, ok := inst.(*vmess.Account)
	if !ok {
		t.Fatalf("instance type %T", inst)
	}
	if acc.Id != "uuid" {
		t.Fatalf("vmess id = %q", acc.Id)
	}
}

func TestBuildUserTrojan(t *testing.T) {
	u, err := BuildUser(3, "uuid", "proxy-secret", NodeView{Protocol: "trojan"})
	if err != nil {
		t.Fatal(err)
	}
	inst, err := u.Account.GetInstance()
	if err != nil {
		t.Fatal(err)
	}
	acc, ok := inst.(*trojan.Account)
	if !ok || acc.Password != "proxy-secret" {
		t.Fatalf("trojan account = %+v", inst)
	}
}

func TestBuildUserShadowsocksCipherMapping(t *testing.T) {
	cases := map[string]shadowsocks.CipherType{
		"chacha20-ietf-poly1305": shadowsocks.CipherType_CHACHA20_POLY1305,
		"aes-256-gcm":            shadowsocks.CipherType_AES_256_GCM,
		"aes-128-gcm":            shadowsocks.CipherType_AES_128_GCM,
		"none":                   shadowsocks.CipherType_NONE,
	}
	for cipher, want := range cases {
		u, err := BuildUser(4, "uuid", "proxy-secret", NodeView{Protocol: "shadowsocks", Cipher: cipher})
		if err != nil {
			t.Fatalf("cipher %s: %v", cipher, err)
		}
		inst, err := u.Account.GetInstance()
		if err != nil {
			t.Fatal(err)
		}
		acc, ok := inst.(*shadowsocks.Account)
		if !ok {
			t.Fatalf("instance type %T", inst)
		}
		if acc.CipherType != want {
			t.Fatalf("cipher %s -> %v, want %v", cipher, acc.CipherType, want)
		}
	}
	if _, err := BuildUser(5, "uuid", "secret", NodeView{Protocol: "shadowsocks", Cipher: "unknown-cipher"}); err == nil {
		t.Fatal("未知 cipher 应返回错误")
	}
}

func TestBuildUserUnsupportedProtocol(t *testing.T) {
	if _, err := BuildUser(6, "uuid", "secret", NodeView{Protocol: "http"}); err == nil {
		t.Fatal("不支持协议应返回错误")
	}
}

func TestAccountFromNodeTypedMessage(t *testing.T) {
	msg, err := AccountFromNode("vless", "uuid", "secret", NodeView{Protocol: "vless"})
	if err != nil {
		t.Fatal(err)
	}
	if msg == nil || !strings.HasSuffix(msg.Type, "vless.Account") {
		t.Fatalf("msg = %+v", msg)
	}
}

func TestProtocolUserType(t *testing.T) {
	u := &protocol.User{}
	if u.GetAccount() != nil {
		t.Fatal("空 user account 应为 nil")
	}
}
