package user

import (
	"context"

	"vpn-sub/internal/mail"
)

// userTestMailer 是 user 包测试用密码重置邮件 fake；可用性与派发结果均可注入。
type userTestMailer struct {
	available bool
	reason    mail.DispatchReason
	result    mail.DispatchResult
	err       error
}

func (m *userTestMailer) CheckAvailability(context.Context, string) (mail.Availability, error) {
	if m.err != nil {
		return mail.Availability{}, m.err
	}
	return mail.Availability{Available: m.available, Reason: m.reason}, nil
}

func (m *userTestMailer) DispatchPasswordReset(context.Context, int64, string, string, string) mail.DispatchResult {
	return m.result
}

func defaultTestResetMailer() *userTestMailer {
	return &userTestMailer{
		available: true,
		result:    mail.DispatchResult{Status: mail.DispatchQueued, LogID: 1},
	}
}
