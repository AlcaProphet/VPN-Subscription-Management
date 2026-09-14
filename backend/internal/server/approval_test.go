package server

import "testing"

// TestResolveSMTPTestRecipient 默认/指定地址与单一邮箱校验。
func TestResolveSMTPTestRecipient(t *testing.T) {
	for _, tc := range []struct {
		name, input, admin, wantTo, wantSource string
		wantErr                                bool
	}{
		{"默认管理员", "", "admin@example.com", "admin@example.com", "default", false},
		{"指定地址", " target@example.com ", "admin@example.com", "target@example.com", "specified", false},
		{"管理员无邮箱但有指定地址", "target@example.com", "", "target@example.com", "specified", false},
		{"均无地址", "", "", "", "", true},
		{"多个地址", "a@example.com,b@example.com", "admin@example.com", "", "", true},
		{"显示名称", "Test <a@example.com>", "admin@example.com", "", "", true},
		{"注入换行", "a@example.com\r\nBcc: x@example.com", "admin@example.com", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			to, source, err := resolveSMTPTestRecipient(tc.input, tc.admin)
			if (err != nil) != tc.wantErr || to != tc.wantTo || source != tc.wantSource {
				t.Fatalf("to=%q source=%q err=%v", to, source, err)
			}
		})
	}
}
