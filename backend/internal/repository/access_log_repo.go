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
	UserID              string
	IP                  string
	DownloadType        string
	Platform            string
	ShareSubscriptionID string
	RuleID              string
	Status              string
	ErrorReason         string
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
		var createdAt string
		if err := rows.Scan(
			&rec.UserID, &rec.IP, &rec.DownloadType, &rec.Platform,
			&rec.ShareSubscriptionID, &rec.RuleID, &rec.Status, &rec.ErrorReason, &createdAt,
		); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}
