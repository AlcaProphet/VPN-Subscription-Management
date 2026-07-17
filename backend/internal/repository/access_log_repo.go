package repository

import (
	"time"
)

// AccessLogRepo provides access to the access_logs table.
type AccessLogRepo struct{}

func NewAccessLogRepo() *AccessLogRepo {
	return &AccessLogRepo{}
}

type AccessLogRecord struct {
	UserID              string `json:"user_id"`
	IP                  string `json:"ip"`
	DownloadType        string `json:"download_type"`
	Platform            string `json:"platform"`
	ShareSubscriptionID string `json:"share_subscription_id"`
	RuleID              string `json:"rule_id"`
	Status              string `json:"status"`
	ErrorReason         string `json:"error_reason"`
	CreatedAt           string `json:"created_at"`
}

// Insert records a new access log entry.
func (r *AccessLogRepo) Insert(record *AccessLogRecord) error {
	_, err := DB.Exec(
		`INSERT INTO access_logs (user_id, ip, download_type, platform, share_subscription_id, rule_id, status, error_reason, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.UserID, record.IP, record.DownloadType, record.Platform,
		record.ShareSubscriptionID, record.RuleID, record.Status, record.ErrorReason,
		time.Now().UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

// InsertAccessLog is a package-level helper for writing access log entries
// from contexts that cannot import the handler package (e.g. middleware).
// Failures are silently ignored so they never affect the response.
func InsertAccessLog(record *AccessLogRecord) {
	_ = NewAccessLogRepo().Insert(record)
}

// ListByDate retrieves access logs for a specific date (format: "2006-01-02").
func (r *AccessLogRepo) ListByDate(date string) ([]AccessLogRecord, error) {
	rows, err := DB.Query(
		`SELECT user_id, ip, download_type, platform, share_subscription_id, rule_id, status, error_reason, created_at
		 FROM access_logs WHERE date(created_at) = ? ORDER BY created_at DESC LIMIT 1000`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []AccessLogRecord
	for rows.Next() {
		var rec AccessLogRecord
		if err := rows.Scan(
			&rec.UserID, &rec.IP, &rec.DownloadType, &rec.Platform,
			&rec.ShareSubscriptionID, &rec.RuleID, &rec.Status, &rec.ErrorReason, &rec.CreatedAt,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
