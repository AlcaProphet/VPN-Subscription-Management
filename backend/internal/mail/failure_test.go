package mail

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestFailureStageOfClosedEnum(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want FailureStage
	}{
		{name: "canceled", err: context.Canceled, want: FailureCanceled},
		{name: "deadline", err: context.DeadlineExceeded, want: FailureTimeout},
		{name: "net timeout", err: timeoutError{}, want: FailureTimeout},
		{name: "auth stage", err: &sendError{stage: FailureAuth, err: errors.New("535 secret")}, want: FailureAuth},
		{name: "render", err: ErrInvalidTemplate, want: FailureRender},
		{name: "config", err: errSMTPNotConfigured, want: FailureConfig},
		{name: "unknown", err: errors.New("unexpected"), want: FailureInternal},
		{name: "nil", err: nil, want: FailureInternal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := FailureStageOf(tc.err); got != tc.want {
				t.Fatalf("FailureStageOf(%v)=%s, want %s", tc.err, got, tc.want)
			}
		})
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

var _ net.Error = timeoutError{}

func TestAvailabilityStrict(t *testing.T) {
	st, svc := newTestMail(t)
	ctx := context.Background()

	got, err := svc.availability(ctx, ScopeWelcome)
	if err != nil {
		t.Fatalf("availability 失败: %v", err)
	}
	if got.Available || got.Reason != ReasonScopeDisabled {
		t.Fatalf("scope 未启用应 skipped/scope_disabled: %+v", got)
	}

	if err := svc.cfg.Set(ctx, KeyScopes, `["welcome"]`); err != nil {
		t.Fatalf("写 scope 失败: %v", err)
	}
	got, err = svc.availability(ctx, ScopeWelcome)
	if err != nil {
		t.Fatalf("availability 失败: %v", err)
	}
	if got.Available || got.Reason != ReasonConfigUnavailable {
		t.Fatalf("scope 启用但 SMTP 不完整应 skipped/config_unavailable: %+v", got)
	}

	for k, v := range map[string]string{
		KeyHost: "127.0.0.1", KeyPort: "2525", KeyFrom: "sender@example.com",
		KeySecurity: "plain", KeyAuth: "false",
	} {
		if err := svc.cfg.Set(ctx, k, v); err != nil {
			t.Fatalf("配置失败: %v", err)
		}
	}
	got, err = svc.availability(ctx, ScopeWelcome)
	if err != nil {
		t.Fatalf("availability 失败: %v", err)
	}
	if !got.Available {
		t.Fatalf("完整配置应可用: %+v", got)
	}

	// 严格读取路径遇到存储故障应返回错误，而不是 skipped。
	if err := st.Close(); err != nil {
		t.Fatalf("关闭测试库失败: %v", err)
	}
	if _, err := svc.availability(ctx, ScopeWelcome); err == nil {
		t.Fatal("存储读取失败时严格 availability 应返回错误")
	}
}
