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

func TestParseAbsoluteHTTPURL(t *testing.T) {
	valid := []string{
		"http://localhost:8080",
		"https://app.example.com",
		"https://app.example.com/subpath/",
	}
	for _, raw := range valid {
		if _, err := ParseAbsoluteHTTPURL(raw); err != nil {
			t.Fatalf("ParseAbsoluteHTTPURL(%q) 不应失败: %v", raw, err)
		}
	}
	invalid := []string{
		"",
		"/relative",
		"ftp://app.example.com",
		"https://",
		"https://user:pass@app.example.com",
		"https://app.example.com/?a=1",
		"https://app.example.com/#frag",
	}
	for _, raw := range invalid {
		if _, err := ParseAbsoluteHTTPURL(raw); err == nil {
			t.Fatalf("ParseAbsoluteHTTPURL(%q) 应失败", raw)
		}
	}
}

func TestValidateOIDCCallbackURL(t *testing.T) {
	const callbackPath = "/api/auth/oidc/callback"
	if _, err := ValidateOIDCCallbackURL("https://callback.example.com"+callbackPath, callbackPath); err != nil {
		t.Fatalf("合法回调地址不应失败: %v", err)
	}
	for _, raw := range []string{
		"https://callback.example.com/wrong",
		"https://callback.example.com" + callbackPath + "/",
		"https://callback.example.com" + callbackPath + "?x=1",
		callbackPath,
	} {
		if _, err := ValidateOIDCCallbackURL(raw, callbackPath); err == nil {
			t.Fatalf("非法回调地址 %q 应失败", raw)
		}
	}
}
