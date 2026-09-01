package xray

import (
	"testing"
	"time"
)

func TestValidateAddr(t *testing.T) {
	if err := ValidateAddr("127.0.0.1:10086"); err != nil {
		t.Fatalf("合法地址被拒绝: %v", err)
	}
	if err := ValidateAddr("example.com:443"); err != nil {
		t.Fatalf("合法域名地址被拒绝: %v", err)
	}
	if err := ValidateAddr(""); err == nil {
		t.Fatal("空地址应拒绝")
	}
	if err := ValidateAddr("127.0.0.1"); err == nil {
		t.Fatal("缺少端口应拒绝")
	}
	if err := ValidateAddr(":10086"); err == nil {
		t.Fatal("缺少 host 应拒绝")
	}
}

func TestTimeoutConstants(t *testing.T) {
	if DialTimeout <= 0 || RPCTimeout <= 0 {
		t.Fatal("超时常量必须为正")
	}
	if DialTimeout >= RPCTimeout {
		t.Fatalf("探测超时应小于普通 RPC 超时: %v vs %v", DialTimeout, RPCTimeout)
	}
	if RPCTimeout > 30*time.Second {
		t.Fatalf("RPC 超时不应超过 30s: %v", RPCTimeout)
	}
}
