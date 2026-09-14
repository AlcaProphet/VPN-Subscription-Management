// log/stream.go：实时日志流服务（Build3 Step 5）——内存环形缓冲（最近 500 条）+ SSE 订阅管理 +
// 全局 8 连接上限（Design1 §4.8）。认证由管理员路由会话 + 角色中间件承担，不再使用一次性查询 Token。
package log

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"
)

const (
	RingBufferSize    = 500 // 环形缓冲最近 500 条
	MaxSSEConnections = 8   // 全局 8 连接（不按管理员计）
)

// Entry 缓冲日志条目（SSE 推送 JSON 结构）
type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Attrs   string    `json:"attrs"` // 预格式化键值对串
}

// RingBuffer 内存环形缓冲：满后覆盖最旧；广播给活跃订阅者（非阻塞，消费慢则丢弃防阻塞日志管道）
type RingBuffer struct {
	mu      sync.RWMutex
	entries []Entry
	subs    map[chan Entry]struct{}
}

func NewRingBuffer() *RingBuffer {
	return &RingBuffer{subs: map[chan Entry]struct{}{}}
}

func (b *RingBuffer) Append(e Entry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) >= RingBufferSize {
		b.entries = b.entries[1:] // 覆盖最旧
		// 定期紧凑化：底层数组因切片左移无限增长，容量超过 4 倍缓冲大小时拷贝紧凑（防内存缓慢膨胀）
		if cap(b.entries) > RingBufferSize*4 {
			b.entries = append([]Entry(nil), b.entries...)
		}
	}
	b.entries = append(b.entries, e)
	for ch := range b.subs { // 广播给活跃订阅者（非阻塞）
		select {
		case ch <- e:
		default: // 订阅者消费慢则丢弃，防阻塞日志管道
		}
	}
}

// History 返回缓冲历史快照（供新订阅者先推历史）
func (b *RingBuffer) History() []Entry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return append([]Entry(nil), b.entries...)
}

// RingHandler slog Handler 包装——记录同时输出 inner（stdout）与内存环形缓冲（内容经外层 Redact 脱敏）
type RingHandler struct {
	inner slog.Handler
	buf   *RingBuffer
}

func NewRingHandler(inner slog.Handler, buf *RingBuffer) *RingHandler {
	return &RingHandler{inner: inner, buf: buf}
}

func (h *RingHandler) Enabled(ctx context.Context, l slog.Level) bool { return h.inner.Enabled(ctx, l) }

func (h *RingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &RingHandler{inner: h.inner.WithAttrs(attrs), buf: h.buf}
}

func (h *RingHandler) WithGroup(name string) slog.Handler {
	return &RingHandler{inner: h.inner.WithGroup(name), buf: h.buf}
}

func (h *RingHandler) Handle(ctx context.Context, r slog.Record) error {
	if err := h.inner.Handle(ctx, r); err != nil {
		return err
	}
	h.buf.Append(Entry{
		Time:    r.Time,
		Level:   r.Level.String(),
		Message: r.Message,
		Attrs:   formatAttrs(r),
	})
	return nil
}

// formatAttrs 预格式化键值对串（"key=value key2=value2"）
func formatAttrs(r slog.Record) string {
	var sb strings.Builder
	first := true
	r.Attrs(func(a slog.Attr) bool {
		if !first {
			sb.WriteString(" ")
		}
		first = false
		sb.WriteString(a.Key)
		sb.WriteString("=")
		sb.WriteString(a.Value.String())
		return true
	})
	return sb.String()
}

// StreamService 实时日志流服务（SSE 连接管理；认证由管理员路由中间件承担）
type StreamService struct {
	buf       *RingBuffer
	log       *slog.Logger
	mu        sync.Mutex
	connCount int // 当前活跃 SSE 连接数
}

func NewStreamService(buf *RingBuffer, lg *slog.Logger) *StreamService {
	return &StreamService{buf: buf, log: lg}
}

// Subscribe 注册订阅者：先返回缓冲历史快照；超 8 连接上限返回 false（接入层拒绝并提示）
func (s *StreamService) Subscribe() (chan Entry, []Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connCount >= MaxSSEConnections {
		return nil, nil, false // 「连接数已达上限，请关闭其他日志页后重试」
	}
	s.connCount++
	ch := make(chan Entry, 64)
	s.buf.mu.Lock()
	s.buf.subs[ch] = struct{}{}
	history := append([]Entry(nil), s.buf.entries...) // 先推送缓冲历史
	s.buf.mu.Unlock()
	return ch, history, true
}

// Unsubscribe 连接断开自动清理订阅
func (s *StreamService) Unsubscribe(ch chan Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.mu.Lock()
	delete(s.buf.subs, ch)
	s.buf.mu.Unlock()
	close(ch)
	s.connCount--
}

// Reset 一键清空数据时内存态复位（Build3 Step 4 的 resetRuntimeState 回调）：清空缓冲与全部活跃连接。
// 查询 Token 状态已删除，不再需要复位。
func (s *StreamService) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.mu.Lock()
	s.buf.entries = nil
	for ch := range s.buf.subs { // 断开全部活跃连接
		close(ch)
		delete(s.buf.subs, ch)
	}
	s.buf.mu.Unlock()
	s.connCount = 0
}
