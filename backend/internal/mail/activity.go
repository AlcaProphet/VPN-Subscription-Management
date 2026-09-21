package mail

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	stdmail "net/mail"
)

// ActivityLogCapacity 短期邮件发送日志固定容量。
const ActivityLogCapacity = 500

// ActivityStatus 邮件发送日志状态。
type ActivityStatus string

const (
	ActivityQueued   ActivityStatus = "queued"
	ActivitySending  ActivityStatus = "sending"
	ActivityAccepted ActivityStatus = "accepted"
	ActivityFailed   ActivityStatus = "failed"
)

// ActivityRecord 短期邮件发送日志记录；只包含安全字段，不保存完整邮箱、主题、正文或 URL。
type ActivityRecord struct {
	ID              int64          `json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	StartedAt       *time.Time     `json:"started_at"`
	FinishedAt      *time.Time     `json:"finished_at"`
	Kind            TemplateKind   `json:"kind"`
	Source          string         `json:"source"`
	UserID          *int64         `json:"user_id"`
	RecipientMasked string         `json:"recipient_masked"`
	Status          ActivityStatus `json:"status"`
	FailureStage    *FailureStage  `json:"failure_stage"`
	QueueDurationMS *int64         `json:"queue_duration_ms"`
	SendDurationMS  *int64         `json:"send_duration_ms"`
}

// ActivityLog 并发安全的进程内短期发送日志。
// 满容量后淘汰最旧记录；ID 单调递增，Clear/Resume 均不归零。
type ActivityLog struct {
	mu       sync.Mutex
	max      int
	nextID   int64
	items    []ActivityRecord // 按创建顺序保存，最新在尾部
	recorder ResultRecorder
}

// SetResultRecorder 注入终态结果旁路记录器；应在开始派发邮件前完成装配。
func (l *ActivityLog) SetResultRecorder(recorder ResultRecorder) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.recorder = recorder
}

// NewActivityLog 创建短期日志；max<=0 时使用固定容量。
func NewActivityLog(max int) *ActivityLog {
	if max <= 0 {
		max = ActivityLogCapacity
	}
	return &ActivityLog{max: max, items: make([]ActivityRecord, 0, max)}
}

// BeginQueued 创建 queued 记录并返回日志 ID。
func (l *ActivityLog) BeginQueued(kind TemplateKind, source string, userID *int64, recipient string) int64 {
	return l.append(ActivityRecord{
		CreatedAt:       time.Now(),
		Kind:            kind,
		Source:          source,
		UserID:          cloneInt64Ptr(userID),
		RecipientMasked: MaskRecipient(recipient),
		Status:          ActivityQueued,
	})
}

// BeginSending 创建 sending 记录（同步 SMTP 测试用），返回日志 ID。
func (l *ActivityLog) BeginSending(kind TemplateKind, source string, userID *int64, recipient string) int64 {
	now := time.Now()
	return l.append(ActivityRecord{
		CreatedAt:       now,
		StartedAt:       timePtr(now),
		Kind:            kind,
		Source:          source,
		UserID:          cloneInt64Ptr(userID),
		RecipientMasked: MaskRecipient(recipient),
		Status:          ActivitySending,
	})
}

// RecordTerminalFailed 创建终态 failed 记录（队列/派发前拒绝、SMTP 测试前已知失败），返回日志 ID。
func (l *ActivityLog) RecordTerminalFailed(kind TemplateKind, source string, userID *int64, recipient string, stage FailureStage) int64 {
	now := time.Now()
	rec := ActivityRecord{
		CreatedAt:       now,
		FinishedAt:      timePtr(now),
		Kind:            kind,
		Source:          source,
		UserID:          cloneInt64Ptr(userID),
		RecipientMasked: MaskRecipient(recipient),
		Status:          ActivityFailed,
		FailureStage:    stagePtr(stage),
	}
	id := l.append(rec)
	rec.ID = id
	l.tryRecordTerminal(rec)
	return id
}

// MarkSending queued→sending；已完成/不存在的 ID 静默跳过。
func (l *ActivityLog) MarkSending(id int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	rec := l.findLocked(id)
	if rec == nil || rec.Status != ActivityQueued {
		return
	}
	now := time.Now()
	rec.Status = ActivitySending
	rec.StartedAt = timePtr(now)
	rec.QueueDurationMS = durationMS(rec.CreatedAt, now)
}

// MarkAccepted 仅允许 sending→accepted；重复终态、非法前置状态均无效果。
func (l *ActivityLog) MarkAccepted(id int64) {
	l.mu.Lock()
	rec := l.findLocked(id)
	if rec == nil || rec.Status != ActivitySending {
		l.mu.Unlock()
		return
	}
	now := time.Now()
	rec.Status = ActivityAccepted
	rec.FinishedAt = timePtr(now)
	if rec.StartedAt != nil {
		rec.SendDurationMS = durationMS(*rec.StartedAt, now)
	}
	rec.FailureStage = nil
	terminal := cloneRecord(*rec)
	recorder := l.recorder
	l.mu.Unlock()
	tryRecordResult(recorder, terminal)
}

// MarkSendFailed 仅允许 sending→failed；不伪造未进入阶段的耗时。
func (l *ActivityLog) MarkSendFailed(id int64, stage FailureStage) {
	l.mu.Lock()
	rec := l.findLocked(id)
	if rec == nil || rec.Status != ActivitySending {
		l.mu.Unlock()
		return
	}
	now := time.Now()
	rec.Status = ActivityFailed
	rec.FinishedAt = timePtr(now)
	rec.FailureStage = stagePtr(stage)
	if rec.StartedAt != nil {
		rec.SendDurationMS = durationMS(*rec.StartedAt, now)
	}
	terminal := cloneRecord(*rec)
	recorder := l.recorder
	l.mu.Unlock()
	tryRecordResult(recorder, terminal)
}

// MarkQueuedFailed 仅允许 queued→failed（队列满/暂停/停止前拒绝）；开始时间与耗时保持 null。
func (l *ActivityLog) MarkQueuedFailed(id int64, stage FailureStage) {
	l.mu.Lock()
	rec := l.findLocked(id)
	if rec == nil || rec.Status != ActivityQueued {
		l.mu.Unlock()
		return
	}
	now := time.Now()
	rec.Status = ActivityFailed
	rec.FinishedAt = timePtr(now)
	rec.FailureStage = stagePtr(stage)
	rec.StartedAt = nil
	rec.QueueDurationMS = nil
	rec.SendDurationMS = nil
	terminal := cloneRecord(*rec)
	recorder := l.recorder
	l.mu.Unlock()
	tryRecordResult(recorder, terminal)
}

// Snapshot 返回最新在前的深拷贝快照。
func (l *ActivityLog) Snapshot() []ActivityRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]ActivityRecord, 0, len(l.items))
	for i := len(l.items) - 1; i >= 0; i-- {
		out = append(out, cloneRecord(l.items[i]))
	}
	return out
}

// Clear 清空当前可见记录；ID 计数器保持单调，迟到更新因 ID 不存在而静默跳过。
func (l *ActivityLog) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.items = nil
}

func (l *ActivityLog) append(rec ActivityRecord) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.nextID++
	rec.ID = l.nextID
	if len(l.items) >= l.max {
		copy(l.items, l.items[1:])
		l.items[len(l.items)-1] = rec
		return rec.ID
	}
	l.items = append(l.items, rec)
	return rec.ID
}

func (l *ActivityLog) findLocked(id int64) *ActivityRecord {
	for i := range l.items {
		if l.items[i].ID == id {
			return &l.items[i]
		}
	}
	return nil
}

// MaskRecipient 只保留邮箱本地部分首字符、固定 *** 和域名；无法安全解析时返回 ***。
func MaskRecipient(raw string) string {
	trimmed := strings.TrimSpace(raw)
	addr, err := stdmail.ParseAddress(trimmed)
	if err != nil || addr.Address == "" || addr.Address != trimmed {
		return "***"
	}
	at := strings.LastIndex(addr.Address, "@")
	if at <= 0 || at == len(addr.Address)-1 {
		return "***"
	}
	local, domain := addr.Address[:at], addr.Address[at+1:]
	first, size := utf8.DecodeRuneInString(local)
	if first == utf8.RuneError && size <= 1 {
		return "***"
	}
	if strings.ContainsAny(domain, "\r\n\t ") {
		return "***"
	}
	return string(first) + "***@" + strings.ToLower(domain)
}

func cloneInt64Ptr(v *int64) *int64 {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

func timePtr(t time.Time) *time.Time {
	n := t
	return &n
}

func stagePtr(s FailureStage) *FailureStage {
	n := s
	return &n
}

func durationMS(start, end time.Time) *int64 {
	if end.Before(start) {
		return nil
	}
	n := end.Sub(start).Milliseconds()
	return &n
}

func cloneRecord(rec ActivityRecord) ActivityRecord {
	out := rec
	out.StartedAt = cloneTimePtr(rec.StartedAt)
	out.FinishedAt = cloneTimePtr(rec.FinishedAt)
	out.UserID = cloneInt64Ptr(rec.UserID)
	out.QueueDurationMS = cloneInt64Ptr(rec.QueueDurationMS)
	out.SendDurationMS = cloneInt64Ptr(rec.SendDurationMS)
	out.FailureStage = cloneStagePtr(rec.FailureStage)
	return out
}

func cloneTimePtr(v *time.Time) *time.Time {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

func cloneStagePtr(v *FailureStage) *FailureStage {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

func (l *ActivityLog) tryRecordTerminal(rec ActivityRecord) {
	l.mu.Lock()
	recorder := l.recorder
	l.mu.Unlock()
	tryRecordResult(recorder, rec)
}

func tryRecordResult(recorder ResultRecorder, rec ActivityRecord) {
	if recorder == nil || rec.FinishedAt == nil || (rec.Status != ActivityAccepted && rec.Status != ActivityFailed) {
		return
	}
	recorder.TryRecord(ResultRecord{
		Kind:            rec.Kind,
		Source:          rec.Source,
		UserID:          cloneInt64Ptr(rec.UserID),
		RecipientMasked: rec.RecipientMasked,
		Result:          rec.Status,
		FailureStage:    cloneStagePtr(rec.FailureStage),
		RecordedAt:      *rec.FinishedAt,
	})
}
