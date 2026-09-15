package urlguard

import (
	"strings"
	"testing"
)

func TestValidateHTTPS(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		{name: "HTTPS 根地址", rawURL: "https://idp.example.com"},
		{name: "HTTPS 带端口与路径", rawURL: "https://idp.example.com:8443/realms/master"},
		{name: "scheme 大小写归一", rawURL: "HTTPS://idp.example.com"},
		{name: "HTTP 拒绝", rawURL: "http://idp.example.com", wantErr: true},
		{name: "空地址拒绝", rawURL: "", wantErr: true},
		{name: "缺少主机名拒绝", rawURL: "https://", wantErr: true},
		{name: "相对地址拒绝", rawURL: "/.well-known/openid-configuration", wantErr: true},
		{name: "解析失败拒绝", rawURL: "https://idp.example.com/%zz", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHTTPS(tt.rawURL)
			if tt.wantErr && err == nil {
				t.Fatalf("ValidateHTTPS(%q) 应失败", tt.rawURL)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("ValidateHTTPS(%q) 不应失败: %v", tt.rawURL, err)
			}
			if tt.rawURL == "http://idp.example.com" && err != nil && !strings.Contains(err.Error(), "仅支持 HTTPS 地址") {
				t.Fatalf("HTTP 地址应返回明确 HTTPS 错误，实际: %v", err)
			}
		})
	}
}
