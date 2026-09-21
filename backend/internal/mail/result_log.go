package mail

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// ResultLogQueueSize 终态结果旁路写入缓冲容量。
const ResultLogQueueSize = 128

// ErrResultLogStopped 表示终态结果服务已经停止。
var ErrResultLogStopped = errors.New("邮件结果日志服务已停止")

// ResultRecord 是 SQLite 中的一封邮件终态结果；只包含安全字段。
type ResultRecord struct {
	ID              int64          `json:"id"`
	Kind            TemplateKind   `json:"kind"`
	Source          string         `json:"source"`
	UserID          *int64         `json:"user_id"`
	RecipientMasked string         `json:"recipient_masked"`
	Result          ActivityStatus `json:"result"`
	FailureStage    *FailureStage  `json:"failure_stage"`
	RecordedAt      time.Time      `json:"recorded_at"`
}

// ResultRecorder 只接收已经形成的安全终态；调用方不得等待或处理写入结果。
type ResultRecorder interface {
	TryRecord(ResultRecord)
}

// DBProvider 只暴露邮件结果服务装配所需的数据库句柄。
type DBProvider interface {
	DB() *sql.DB
}

type resultCommand struct {
	record  *ResultRecord
	clear   *clearResultCommand
	barrier chan struct{}
	stop    chan struct{}
}

type clearResultCommand struct {
	cutoff time.Time
	done   chan error
}

// ResultLog 提供终态结果的有界旁路写入、查询和有序清空。
// SQLite 故障只记录 warn，不会反向影响邮件发送。
type ResultLog struct {
	db  *sql.DB
	log *slog.Logger
	cmd chan resultCommand

	mu      sync.RWMutex
	stopped bool
}

// NewResultLog 创建并启动单 writer 终态结果服务。
func NewResultLog(db *sql.DB, lg *slog.Logger) (*ResultLog, error) {
	if db == nil {
		return nil, errors.New("邮件结果日志缺少数据库")
	}
	if lg == nil {
		lg = slog.Default()
	}
	r := &ResultLog{
		db:  db,
		log: lg,
		cmd: make(chan resultCommand, ResultLogQueueSize),
	}
	go r.run()
	return r, nil
}

// NewResultLogFromProvider 通过存储适配接口装配，避免接入层直接访问数据库。
func NewResultLogFromProvider(provider DBProvider, lg *slog.Logger) (*ResultLog, error) {
	if provider == nil {
		return nil, errors.New("邮件结果日志缺少数据库提供者")
	}
	return NewResultLog(provider.DB(), lg)
}

// TryRecord 非阻塞提交一条终态结果；缓冲满或服务停止时允许丢弃。
func (r *ResultLog) TryRecord(record ResultRecord) {
	if r == nil {
		return
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.stopped {
		return
	}
	copyRecord := cloneResultRecord(record)
	select {
	case r.cmd <- resultCommand{record: &copyRecord}:
	default:
		r.log.Warn("邮件结果日志缓冲区已满，记录已丢弃",
			"kind", record.Kind, "result", record.Result, "failure_stage", record.FailureStage)
	}
}

// Query 按终态时间倒序查询历史结果。
func (r *ResultLog) Query(ctx context.Context, page, size int, kind string, result ActivityStatus) ([]ResultRecord, int64, error) {
	where := " WHERE 1=1"
	args := make([]any, 0, 4)
	if kind != "" {
		where += " AND kind = ?"
		args = append(args, kind)
	}
	if result != "" {
		where += " AND result = ?"
		args = append(args, result)
	}
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM mail_result_logs"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any(nil), args...), size, (page-1)*size)
	rows, err := r.db.QueryContext(ctx, `SELECT id, kind, source, user_id, recipient_masked, result, failure_stage, recorded_at
		FROM mail_result_logs`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list := make([]ResultRecord, 0)
	for rows.Next() {
		var rec ResultRecord
		var userID sql.NullInt64
		var stage sql.NullString
		if err := rows.Scan(&rec.ID, &rec.Kind, &rec.Source, &userID, &rec.RecipientMasked,
			&rec.Result, &stage, &rec.RecordedAt); err != nil {
			return nil, 0, err
		}
		if userID.Valid {
			rec.UserID = cloneInt64Ptr(&userID.Int64)
		}
		if stage.Valid {
			failureStage := FailureStage(stage.String)
			rec.FailureStage = &failureStage
		}
		list = append(list, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Clear 删除调用时已经形成的历史终态结果。
// cutoff 之前形成但排在 clear 命令后的并发迟到记录也会被 writer 丢弃。
func (r *ResultLog) Clear(ctx context.Context) error {
	clear := &clearResultCommand{cutoff: time.Now(), done: make(chan error, 1)}
	if err := r.sendControl(ctx, resultCommand{clear: clear}); err != nil {
		return err
	}
	select {
	case err := <-clear.done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Flush 等待此前已接收的旁路记录处理完成；仅供生命周期与测试使用。
func (r *ResultLog) Flush(ctx context.Context) error {
	done := make(chan struct{})
	if err := r.sendControl(ctx, resultCommand{barrier: done}); err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop 拒绝新记录，并在上下文期限内排空此前已接收的记录后停止 writer。
func (r *ResultLog) Stop(ctx context.Context) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	if r.stopped {
		r.mu.Unlock()
		return nil
	}
	r.stopped = true
	done := make(chan struct{})
	select {
	case r.cmd <- resultCommand{stop: done}:
		r.mu.Unlock()
	case <-ctx.Done():
		r.mu.Unlock()
		return ctx.Err()
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *ResultLog) sendControl(ctx context.Context, cmd resultCommand) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.stopped {
		return ErrResultLogStopped
	}
	select {
	case r.cmd <- cmd:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *ResultLog) run() {
	var clearCutoff time.Time
	defer func() {
		if recover() != nil {
			r.log.Error("邮件结果日志 writer 异常退出")
		}
	}()
	for cmd := range r.cmd {
		switch {
		case cmd.record != nil:
			if !clearCutoff.IsZero() && !cmd.record.RecordedAt.After(clearCutoff) {
				continue
			}
			r.insert(*cmd.record)
		case cmd.clear != nil:
			err := r.clear(cmd.clear.cutoff)
			if err == nil {
				clearCutoff = cmd.clear.cutoff
			}
			cmd.clear.done <- err
		case cmd.barrier != nil:
			close(cmd.barrier)
		case cmd.stop != nil:
			close(cmd.stop)
			return
		}
	}
}

func (r *ResultLog) insert(record ResultRecord) {
	_, err := r.db.Exec(`INSERT INTO mail_result_logs
		(kind, source, user_id, recipient_masked, result, failure_stage, recorded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, record.Kind, record.Source, record.UserID,
		record.RecipientMasked, record.Result, record.FailureStage, record.RecordedAt)
	if err != nil {
		r.log.Warn("持久化邮件结果日志失败", "kind", record.Kind,
			"result", record.Result, "failure_stage", record.FailureStage, "err", err)
	}
}

func (r *ResultLog) clear(cutoff time.Time) error {
	_, err := r.db.Exec(`DELETE FROM mail_result_logs WHERE recorded_at <= ?`, cutoff)
	return err
}

func cloneResultRecord(record ResultRecord) ResultRecord {
	out := record
	out.UserID = cloneInt64Ptr(record.UserID)
	out.FailureStage = cloneStagePtr(record.FailureStage)
	return out
}
