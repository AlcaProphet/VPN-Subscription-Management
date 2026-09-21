package mail

import (
	"context"
	"errors"
	"net"
	"os"
)

// FailureStage 邮件派发与同步测试的封闭安全失败阶段。
// 只允许新增明确阶段；禁止从 error.Error() 文本反推阶段。
type FailureStage string

const (
	FailureQueueFull             FailureStage = "queue_full"
	FailureDispatcherUnavailable FailureStage = "dispatcher_unavailable"
	FailureConfig                FailureStage = "config"
	FailureRender                FailureStage = "render"
	FailureConnect               FailureStage = "connect"
	FailureHandshake             FailureStage = "handshake"
	FailureStartTLS              FailureStage = "starttls"
	FailureAuth                  FailureStage = "auth"
	FailureMailFrom              FailureStage = "mail_from"
	FailureRcptTo                FailureStage = "rcpt_to"
	FailureData                  FailureStage = "data"
	FailureQuit                  FailureStage = "quit"
	FailureTimeout               FailureStage = "timeout"
	FailureCanceled              FailureStage = "canceled"
	FailureInternal              FailureStage = "internal"
)

// message 返回阶段对应的安全中文文案，不含服务商原始响应。
func (s FailureStage) message() string {
	switch s {
	case FailureQueueFull:
		return "邮件队列已满"
	case FailureDispatcherUnavailable:
		return "邮件派发器不可用"
	case FailureConfig:
		return "SMTP 未配置或配置不可用"
	case FailureRender:
		return "邮件内容无效"
	case FailureConnect:
		return "SMTP 连接失败"
	case FailureHandshake:
		return "SMTP 握手失败"
	case FailureStartTLS:
		return "SMTP STARTTLS 失败"
	case FailureAuth:
		return "SMTP 认证失败"
	case FailureMailFrom:
		return "SMTP 发件人失败"
	case FailureRcptTo:
		return "SMTP 收件人失败"
	case FailureData:
		return "SMTP 内容传输失败"
	case FailureQuit:
		return "SMTP 结束会话失败"
	case FailureTimeout:
		return "SMTP 操作超时"
	case FailureCanceled:
		return "SMTP 操作已取消"
	case FailureInternal:
		fallthrough
	default:
		return "SMTP 内部错误"
	}
}

// FailureStageOf 把传输层错误稳定归类为封闭阶段；未知错误统一 internal。
// 取消与超时优先于底层具体阶段，保证停止/清空语义可被调用方识别。
func FailureStageOf(err error) FailureStage {
	if err == nil {
		return FailureInternal
	}
	if errors.Is(err, context.Canceled) {
		return FailureCanceled
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded) {
		return FailureTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return FailureTimeout
	}
	var sendErr *sendError
	if errors.As(err, &sendErr) {
		return sendErr.stage
	}
	if errors.Is(err, ErrInvalidTemplate) {
		return FailureRender
	}
	if errors.Is(err, errSMTPNotConfigured) {
		return FailureConfig
	}
	return FailureInternal
}

// DispatchReason 派发被跳过或拒绝的封闭原因，禁止调用方解析错误文本。
type DispatchReason string

const (
	ReasonScopeDisabled         DispatchReason = "scope_disabled"
	ReasonConfigUnavailable     DispatchReason = "config_unavailable"
	ReasonQueueFull             DispatchReason = "queue_full"
	ReasonDispatcherUnavailable DispatchReason = "dispatcher_unavailable"
	ReasonConfigReadFailed      DispatchReason = "config_read_failed"
	ReasonEmptyRecipient        DispatchReason = "empty_recipient"
)

// Availability 单个邮件 scope 的可用性判断结果；Available=false 时 Reason 必为稳定枚举。
type Availability struct {
	Available bool
	Reason    DispatchReason
}

var errSMTPNotConfigured = errors.New("SMTP 未配置")

// errStartTLSNotSupported 用于给已知的本地协议不满足场景提供稳定安全文案，不携带服务商响应。
var errStartTLSNotSupported = errors.New("SMTP 服务器未提供 STARTTLS，已停止发送")
