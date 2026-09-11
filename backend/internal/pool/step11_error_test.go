package pool

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
)

func insertRunningTask(t *testing.T, svc *Service, poolID int64) int64 {
	t.Helper()
	res, err := svc.store.DB().Exec(
		`INSERT INTO pool_sync_tasks (pool_id, status, per_url_json, error) VALUES (?, 'running', '[]', '')`, poolID)
	if err != nil {
		t.Fatalf("插入 running 任务失败: %v", err)
	}
	id, _ := res.LastInsertId()
	return id
}

// TestApplyParseResultMarshalFailureRollsBack 序列化失败必须回滚，不写入半成品快照、不修改 active/pending 指针。
func TestApplyParseResultMarshalFailureRollsBack(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "marshal失败池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	boom := errors.New("marshal boom")
	err = st.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, _, err := applyParseResultTxWithMarshal(ctx, tx, p.ID, p.Sources[0].ID,
			&ParseResult{Format: FormatLegacyDomainText, Profile: "common"},
			func(any) ([]byte, error) { return nil, boom })
		return err
	})
	if !errors.Is(err, boom) {
		t.Fatalf("序列化失败应原样返回: %v", err)
	}
	var snapshots int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_source_snapshots`).Scan(&snapshots); err != nil {
		t.Fatalf("统计快照失败: %v", err)
	}
	if snapshots != 0 {
		t.Fatalf("序列化失败不应留下快照: %d", snapshots)
	}
	if activeSnapshotID(t, st, p.ID) != 0 {
		t.Fatal("序列化失败不应修改 active 指针")
	}
}

// TestRecordFailedSnapshotMarshalFailure 失败快照自身序列化失败必须返回错误，不得伪报成功。
func TestRecordFailedSnapshotMarshalFailure(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "失败快照序列化池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	boom := errors.New("marshal boom")
	err = st.TxImmediate(ctx, func(tx *sql.Tx) error {
		return recordFailedSnapshotTxWithReasonAndMarshal(ctx, tx, p.ID, p.Sources[0].ID, "失败", "request_invalid",
			func(any) ([]byte, error) { return nil, boom })
	})
	if !errors.Is(err, boom) {
		t.Fatalf("失败快照序列化错误应返回: %v", err)
	}
	var snapshots int
	if err := st.DB().QueryRow(`SELECT COUNT(*) FROM pool_source_snapshots WHERE status='failed'`).Scan(&snapshots); err != nil {
		t.Fatalf("统计失败快照失败: %v", err)
	}
	if snapshots != 0 {
		t.Fatalf("序列化失败不应写入 failed 快照: %d", snapshots)
	}
}

// TestFailedSnapshotOldActiveQueryFailure 旧 active 查询错误必须显式返回，不能当作无旧 active 继续。
func TestFailedSnapshotOldActiveQueryFailure(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	p, err := svc.Create(ctx, "旧active查询池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	tx, err := st.DB().BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		t.Fatalf("开启事务失败: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("回滚事务失败: %v", err)
	}
	err = recordFailedSnapshotTxWithReasonAndMarshal(ctx, tx, p.ID, p.Sources[0].ID, "失败", "request_invalid", json.Marshal)
	if err == nil || !strings.Contains(err.Error(), "读取失败快照旧 active 状态失败") {
		t.Fatalf("旧 active 查询失败应返回定位错误: %v", err)
	}
}

// TestFinishTaskSerializationErrorIsLogged 任务结果序列化失败要可见，并仍以空数组写入终态避免任务卡死。
func TestFinishTaskSerializationErrorIsLogged(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	var buf bytes.Buffer
	svc.log = slog.New(slog.NewTextHandler(&buf, nil))
	p, err := svc.Create(ctx, "终态序列化池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	taskID := insertRunningTask(t, svc, p.ID)
	svc.marshal = func(any) ([]byte, error) { return nil, errors.New("marshal boom") }
	svc.finishTask(ctx, st, p.ID, taskID, "succeeded", "", nil)

	var status, perJSON string
	if err := st.DB().QueryRow(`SELECT status, per_url_json FROM pool_sync_tasks WHERE id=?`, taskID).Scan(&status, &perJSON); err != nil {
		t.Fatalf("查询终态任务失败: %v", err)
	}
	if status != "succeeded" || perJSON != "[]" {
		t.Fatalf("序列化失败仍应写入安全终态: status=%s per=%s", status, perJSON)
	}
	if !strings.Contains(buf.String(), "序列化同步任务结果失败") || !strings.Contains(buf.String(), "task_id") || !strings.Contains(buf.String(), "pool_id") {
		t.Fatalf("序列化失败日志缺少定位上下文: %q", buf.String())
	}
}

// TestFinishTaskCleanupErrorIsLogged 终态任务清理 Exec 失败必须写结构化日志，且不覆盖已提交终态。
func TestFinishTaskCleanupErrorIsLogged(t *testing.T) {
	st, svc := newTestService(t)
	ctx := context.Background()
	var buf bytes.Buffer
	svc.log = slog.New(slog.NewTextHandler(&buf, nil))
	p, err := svc.Create(ctx, "终态清理池", []SourceInput{{URL: "https://example.com/rules", SourceMode: SourceModeAuto}}, false, "04:00")
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	taskID := insertRunningTask(t, svc, p.ID)
	// 插入一条 8 天前终态任务，使清理 DELETE 命中行并触发失败触发器。
	if _, err := st.DB().Exec(
		`INSERT INTO pool_sync_tasks (pool_id, status, per_url_json, error, finished_at, created_at)
		 VALUES (?, 'succeeded', '[]', '', datetime('now','-8 days'), datetime('now','-8 days'))`, p.ID); err != nil {
		t.Fatalf("插入历史终态任务失败: %v", err)
	}
	if _, err := st.DB().Exec(`
		CREATE TRIGGER fail_terminal_cleanup BEFORE DELETE ON pool_sync_tasks
		BEGIN
			SELECT RAISE(ABORT, 'cleanup fail');
		END;`); err != nil {
		t.Fatalf("创建清理失败触发器失败: %v", err)
	}
	svc.finishTask(ctx, st, p.ID, taskID, "succeeded", "", nil)

	var status string
	if err := st.DB().QueryRow(`SELECT status FROM pool_sync_tasks WHERE id=?`, taskID).Scan(&status); err != nil {
		t.Fatalf("查询终态任务失败: %v", err)
	}
	if status != "succeeded" {
		t.Fatalf("清理失败不应破坏已提交终态: %s", status)
	}
	if !strings.Contains(buf.String(), "清理终态同步任务失败") || !strings.Contains(buf.String(), "task_id") || !strings.Contains(buf.String(), "pool_id") {
		t.Fatalf("清理失败日志缺少定位上下文: %q", buf.String())
	}
}
