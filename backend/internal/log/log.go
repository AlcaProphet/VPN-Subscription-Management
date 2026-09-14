// Package log 提供结构化日志封装：分级输出、console/JSON 双格式、token 脱敏。
package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"

	"vpn-sub/internal/redact"
)

// Runtime 一次日志装配的实例：Logger 与独立 LevelVar 成对持有，不共享可变包级状态。
// 多 Server/multi-runtime 测试可通过各自 Runtime 切换级别而互不影响。
type Runtime struct {
	Logger *slog.Logger
	Level  *slog.LevelVar
}

// SetLevel 运行时切换本实例日志级别（debug/info/warn/error），立即生效。
func (r Runtime) SetLevel(level string) {
	if r.Level != nil {
		setLevel(r.Level, level)
	}
}

func setLevel(lv *slog.LevelVar, level string) {
	switch level {
	case "debug":
		lv.Set(slog.LevelDebug)
	case "warn":
		lv.Set(slog.LevelWarn)
	case "error":
		lv.Set(slog.LevelError)
	default:
		lv.Set(slog.LevelInfo)
	}
}

// NewRuntime 构建分级 + 双格式运行实例：format="json" 用 JSONHandler，否则 TextHandler，均输出 stdout。
// 可选 bufs：传入环形缓冲时日志同时写入内存缓冲（实时日志流 SSE 数据源，Build3 Step 5）。
func NewRuntime(level, format string, bufs ...*RingBuffer) Runtime {
	lv := new(slog.LevelVar)
	setLevel(lv, level)
	opts := &slog.HandlerOptions{Level: lv}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		h = slog.NewTextHandler(os.Stdout, opts)
	}
	if len(bufs) > 0 && bufs[0] != nil {
		h = NewRingHandler(h, bufs[0]) // 缓冲与 stdout 输出并存
	}
	// 外层 Redact 保证 stdout 与缓冲内容均经 token 脱敏
	return Runtime{Logger: slog.New(NewRedactHandler(h)), Level: lv}
}

// New 兼容入口：每次返回独立 Logger，不再共享任何包级 LevelVar。
// 需要运行时切换级别的调用方应使用 NewRuntime 并持有 Runtime.Level。
func New(level, format string, bufs ...*RingBuffer) *slog.Logger {
	return NewRuntime(level, format, bufs...).Logger
}

// --- 请求上下文 Logger 注入（业务 Handler/中间件经 FromContext 取实例 Logger）---

type loggerContextKey struct{}

// WithLogger 把实例 Logger 写入请求上下文。
func WithLogger(ctx context.Context, lg *slog.Logger) context.Context {
	if lg == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerContextKey{}, lg)
}

// FromContext 返回当前请求的实例 Logger；缺失时返回丢弃输出的 logger，避免回退到可变全局状态。
func FromContext(ctx context.Context) *slog.Logger {
	if lg, ok := ctx.Value(loggerContextKey{}).(*slog.Logger); ok && lg != nil {
		return lg
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- token 脱敏：密码重置路径 + 公共 redact 规则（AGENTS §4.3）---

var resetPathRe = regexp.MustCompile(`(?i)(/reset/)[^/?#\s]*`)

// Redact 对字符串中的敏感参数与密码重置路径进行脱敏，规则与 pool 共用 redact.RedactText。
func Redact(s string) string {
	s = redact.RedactText(s)
	return resetPathRe.ReplaceAllString(s, "${1}***")
}

// RedactHandler 包装任意 slog.Handler：消息与字符串属性统一经脱敏（关键约束 AGENTS §4.3）
type RedactHandler struct {
	inner slog.Handler
}

func NewRedactHandler(inner slog.Handler) *RedactHandler { return &RedactHandler{inner: inner} }

func (h *RedactHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h *RedactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &RedactHandler{inner: h.inner.WithAttrs(redactAttrs(attrs))}
}

func (h *RedactHandler) WithGroup(name string) slog.Handler {
	return &RedactHandler{inner: h.inner.WithGroup(name)}
}

func (h *RedactHandler) Handle(ctx context.Context, r slog.Record) error {
	// 收集并脱敏全部属性，再用 NewRecord 重建（slog.Record 不可就地改属性）
	attrs := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, redactAttr(a))
		return true
	})
	newRec := slog.NewRecord(r.Time, r.Level, Redact(r.Message), r.PC)
	newRec.AddAttrs(attrs...)
	return h.inner.Handle(ctx, newRec)
}

// redactAttrs 对属性列表逐个脱敏
func redactAttrs(attrs []slog.Attr) []slog.Attr {
	out := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		out[i] = redactAttr(a)
	}
	return out
}

// redactAttr 仅对 string 值脱敏，其余类型原样返回
func redactAttr(a slog.Attr) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		return slog.String(a.Key, Redact(a.Value.String()))
	}
	return a
}
