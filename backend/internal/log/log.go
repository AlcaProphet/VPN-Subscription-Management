// Package log 提供结构化日志封装：分级输出、console/JSON 双格式、token 脱敏。
package log

import (
	"context"
	"log/slog"
	"os"
	"regexp"
)

// 包级默认 logger（仅日志设施自身例外；业务服务实例一律构造注入）
var (
	defaultLogger = slog.New(slog.NewTextHandler(os.Stdout, nil))

	// levelVar 全局级别控制器：运行时切换立即生效（Build3 Step 3 接通，持久化由调用方写配置键）
	levelVar = new(slog.LevelVar)
)

func SetDefault(l *slog.Logger) { defaultLogger = l }

// SetLevel 运行时切换日志级别（debug/info/warn/error），立即生效
func SetLevel(level string) {
	switch level {
	case "debug":
		levelVar.Set(slog.LevelDebug)
	case "warn":
		levelVar.Set(slog.LevelWarn)
	case "error":
		levelVar.Set(slog.LevelError)
	default:
		levelVar.Set(slog.LevelInfo)
	}
}

func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }
func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }

// New 构建分级 + 双格式 logger：format="json" 用 JSONHandler，否则 TextHandler，均输出 stdout。
// 内部以 *slog.LevelVar 代替固定 Level，并暴露 SetLevel：运行时切换立即生效。
// 可选 bufs：传入环形缓冲时日志同时写入内存缓冲（实时日志流 SSE 数据源，Build3 Step 5）
func New(level, format string, bufs ...*RingBuffer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: levelVar}
	SetLevel(level)
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
	return slog.New(NewRedactHandler(h))
}

// 当前默认 logger（供测试与特殊场景取用）
func Default() *slog.Logger { return defaultLogger }

// --- token 脱敏：?token=xxx / &token=xxx 的值一律替换为 ***（AGENTS §4.3）---

var tokenValueRe = regexp.MustCompile(`([?&]token=)[^&\s]*`)

// Redact 对字符串中的 token 查询参数值脱敏
func Redact(s string) string {
	return tokenValueRe.ReplaceAllString(s, "${1}***")
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
