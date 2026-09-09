package pool

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"vpn-sub/internal/redact"
)

// SnapshotStats 是 v1 强类型快照统计。
type SnapshotStats struct {
	SchemaVersion        int              `json:"schema_version"`
	SourceMode           string           `json:"source_mode"`
	Detection            *DetectionStats  `json:"detection"`
	RuleCounts           []RuleCountStat  `json:"rule_counts"`
	UnclassifiedRejected int              `json:"unclassified_rejected"`
	Comparison           *ComparisonStats `json:"comparison"`
	Decision             *DecisionStats   `json:"decision"`
}

// DetectionStats 固定保存证据码与识别率门槛。
type DetectionStats struct {
	EvidenceCodes              []string `json:"evidence_codes"`
	RecognitionRequiredPercent *int     `json:"recognition_required_percent"`
}

// ComparisonStats 固定保存与旧 active 的比较。
type ComparisonStats struct {
	PreviousActive               *PreviousActiveSnapshot `json:"previous_active"`
	FormatChanged                bool                    `json:"format_changed"`
	ProfileChanged               bool                    `json:"profile_changed"`
	AcceptedDropThresholdPercent int                     `json:"accepted_drop_threshold_percent"`
	AcceptedDropTriggered        bool                    `json:"accepted_drop_triggered"`
}

// PreviousActiveSnapshot 是旧 active 摘要。
type PreviousActiveSnapshot struct {
	SnapshotID int64  `json:"snapshot_id"`
	Format     string `json:"format"`
	Profile    string `json:"profile"`
	Accepted   int64  `json:"accepted"`
}

// DecisionStats 固定保存创建时初始状态与原因码。
type DecisionStats struct {
	InitialStatus string   `json:"initial_status"`
	ReasonCodes   []string `json:"reason_codes"`
}

// SourceSnapshot 是快照读模型。
type SourceSnapshot struct {
	ID          int64             `json:"id"`
	SourceID    int64             `json:"source_id"`
	Format      string            `json:"format"`
	Profile     string            `json:"profile"`
	Status      string            `json:"status"`
	Input       int               `json:"input"`
	Recognized  int               `json:"recognized"`
	Accepted    int               `json:"accepted"`
	Excluded    int               `json:"excluded"`
	Rejected    int               `json:"rejected"`
	Duplicates  int               `json:"duplicates"`
	Diagnostics []ParseDiagnostic `json:"diagnostics"`
	Stats       SnapshotStats     `json:"stats"`
	Error       string            `json:"error,omitempty"`
	ActivatedAt *time.Time        `json:"activated_at,omitempty"`
	CreatedAt   *time.Time        `json:"created_at,omitempty"`
}

// SourceStatus 是每 URL 来源当前状态。
type SourceStatus struct {
	SourceID      int64           `json:"source_id"`
	DisplayURL    string          `json:"display_url"`
	SourceMode    SourceMode      `json:"source_mode"`
	NeverSynced   bool            `json:"never_synced"`
	LatestAttempt *SourceSnapshot `json:"latest_attempt,omitempty"`
	Active        *SourceSnapshot `json:"active,omitempty"`
	Pending       *SourceSnapshot `json:"pending,omitempty"`
	LatestFailed  *SourceSnapshot `json:"latest_failed,omitempty"`
}

const snapshotSelect = `SELECT id, source_id, format, profile, status, input_count, recognized_count, accepted_count, excluded_count, rejected_count, duplicate_count, diagnostic_json, stats_json, created_at, activated_at FROM pool_source_snapshots`

func parseStats(raw string) SnapshotStats {
	var s SnapshotStats
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return legacySnapshotStats()
	}
	if s.SchemaVersion == 0 {
		return legacySnapshotStats()
	}
	if s.RuleCounts == nil {
		s.RuleCounts = []RuleCountStat{}
	}
	return s
}

func legacySnapshotStats() SnapshotStats {
	return SnapshotStats{
		SchemaVersion: 0,
		RuleCounts:    []RuleCountStat{},
	}
}

func snapshotErrorFromDiagnostics(status string, diags []ParseDiagnostic) string {
	if status != "failed" || len(diags) == 0 {
		return ""
	}
	if diags[0].Kind == "error" || diags[0].Kind == "reject" || diags[0].Kind == "truncated" {
		return diags[0].Message
	}
	return ""
}

func scanSnapshot(row rowScanner) (*SourceSnapshot, error) {
	var s SourceSnapshot
	var diagRaw, statsRaw string
	var created, activated sql.NullString
	err := row.Scan(&s.ID, &s.SourceID, &s.Format, &s.Profile, &s.Status, &s.Input, &s.Recognized,
		&s.Accepted, &s.Excluded, &s.Rejected, &s.Duplicates, &diagRaw, &statsRaw, &created, &activated)
	if err != nil {
		return nil, err
	}
	var diags []ParseDiagnostic
	_ = json.Unmarshal([]byte(diagRaw), &diags)
	s.Diagnostics = NormalizeDiagnostics(diags)
	s.Stats = parseStats(statsRaw)
	s.Error = snapshotErrorFromDiagnostics(s.Status, s.Diagnostics)
	if created.Valid && created.String != "" {
		if t, err := parseDBTime(created.String); err == nil {
			s.CreatedAt = &t
		}
	}
	if activated.Valid && activated.String != "" {
		if t, err := parseDBTime(activated.String); err == nil {
			s.ActivatedAt = &t
		}
	}
	return &s, nil
}

func (s *Service) fetchSnapshot(ctx context.Context, id int64) (*SourceSnapshot, error) {
	row := s.store.DB().QueryRowContext(ctx, snapshotSelect+` WHERE id=?`, id)
	snap, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// ListSourceStatuses 返回当前池全部 URL 来源状态列表，统一包裹由 server 层完成。
func (s *Service) ListSourceStatuses(ctx context.Context, poolID int64) ([]SourceStatus, error) {
	if err := s.ensureExists(ctx, poolID); err != nil {
		return nil, err
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, COALESCE(url,''), source_mode, active_snapshot_id, pending_snapshot_id
		 FROM rule_pool_sources WHERE pool_id=? AND kind='url' ORDER BY sort_order,id`, poolID)
	if err != nil {
		return nil, err
	}
	type sourceRow struct {
		id              int64
		url             string
		mode            string
		active, pending sql.NullInt64
	}
	var srcs []sourceRow
	for rows.Next() {
		var r sourceRow
		if err := rows.Scan(&r.id, &r.url, &r.mode, &r.active, &r.pending); err != nil {
			_ = rows.Close()
			return nil, err
		}
		srcs = append(srcs, r)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]SourceStatus, 0, len(srcs))
	for _, r := range srcs {
		st := SourceStatus{
			SourceID:   r.id,
			DisplayURL: redact.RedactDisplayURL(r.url),
			SourceMode: SourceMode(r.mode),
		}
		latest, err := s.latestAttemptSnapshot(ctx, r.id)
		if err != nil {
			return nil, err
		}
		if latest != nil {
			st.LatestAttempt = latest
		}
		if r.active.Valid && r.active.Int64 > 0 {
			if snap, err := s.fetchSnapshot(ctx, r.active.Int64); err == nil {
				st.Active = snap
			} else if !errors.Is(err, ErrNotFound) {
				return nil, err
			}
		}
		if r.pending.Valid && r.pending.Int64 > 0 {
			if snap, err := s.fetchSnapshot(ctx, r.pending.Int64); err == nil {
				st.Pending = snap
			} else if !errors.Is(err, ErrNotFound) {
				return nil, err
			}
		}
		lf, err := s.latestFailedSnapshot(ctx, r.id)
		if err != nil {
			return nil, err
		}
		if lf != nil {
			st.LatestFailed = lf
		}
		st.NeverSynced = st.LatestAttempt == nil
		out = append(out, st)
	}
	return out, nil
}

func (s *Service) latestAttemptSnapshot(ctx context.Context, sourceID int64) (*SourceSnapshot, error) {
	row := s.store.DB().QueryRowContext(ctx,
		snapshotSelect+` WHERE source_id=? ORDER BY created_at DESC, id DESC LIMIT 1`, sourceID)
	snap, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snap, nil
}

func (s *Service) latestFailedSnapshot(ctx context.Context, sourceID int64) (*SourceSnapshot, error) {
	row := s.store.DB().QueryRowContext(ctx,
		snapshotSelect+` WHERE source_id=? AND status='failed' ORDER BY created_at DESC, id DESC LIMIT 1`, sourceID)
	snap, err := scanSnapshot(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return snap, nil
}

// ListSourceSnapshots 返回某 URL 来源的快照历史分页。
func (s *Service) ListSourceSnapshots(ctx context.Context, poolID, sourceID, page, pageSize int64) ([]SourceSnapshot, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	if err := s.ensureExists(ctx, poolID); err != nil {
		return nil, 0, err
	}
	var n int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM rule_pool_sources WHERE id=? AND pool_id=? AND kind='url'`, sourceID, poolID).Scan(&n); err != nil {
		return nil, 0, err
	}
	if n == 0 {
		return nil, 0, ErrNotFound
	}
	var total int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pool_source_snapshots WHERE source_id=?`, sourceID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.store.DB().QueryContext(ctx,
		snapshotSelect+` WHERE source_id=? ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`,
		sourceID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]SourceSnapshot, 0, pageSize)
	for rows.Next() {
		snap, err := scanSnapshot(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *snap)
	}
	return out, total, rows.Err()
}

var _ = fmt.Sprintf
