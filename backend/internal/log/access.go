// log/access.go：访问日志服务（Build3 Step 5）——按日期范围分页查询与清空（Design1 §3.4.9）。
// 资源标识记录口径（Build2 Step 4 写入时遵循）：显式/自定义 Token 记订阅标识；无标识 Token 记解析出的
// 订阅标识；解析失败（unassigned）记平台标识。90 天定时清理由 cron 后台任务负责（Build2 已建，确认接通）。
// 依赖注入采用 *sql.DB（而非 store.Store）避免 store→log 循环依赖。
package log

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// AccessService 访问日志服务
// 依赖注入采用 *sql.DB（而非 store.Store）避免 store→log 循环依赖。
type AccessService struct {
	db  *sql.DB
	log *slog.Logger
}

func NewAccessService(db *sql.DB, lg *slog.Logger) *AccessService {
	return &AccessService{db: db, log: lg}
}

// AccessLog 访问日志记录
type AccessLog struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"` // 0 = 空（分享/规则下载）
	Username     string    `json:"username"`
	IP           string    `json:"ip"`
	DownloadType string    `json:"download_type"`
	Platform     string    `json:"platform"`
	ResourceSlug string    `json:"resource_slug"`
	Status       string    `json:"status"` // success/fail
	FailReason   string    `json:"fail_reason"`
	CreatedAt    time.Time `json:"created_at"` // UTC
}

// Query 按日期范围查询（后端分页，默认 20 条/页；日期格式 YYYY-MM-DD，为空表示不限）
func (s *AccessService) Query(ctx context.Context, from, to string, page, size int) ([]AccessLog, int64, error) {
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	if page <= 0 {
		page = 1
	}
	// 日期范围解析（空 = 不限）；to 取当日 23:59:59 止
	start, end, err := parseRange(from, to)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := []any{}
	if start != nil {
		where += " AND a.created_at >= ?"
		args = append(args, start.Format("2006-01-02 15:04:05"))
	}
	if end != nil {
		where += " AND a.created_at <= ?"
		args = append(args, end.Format("2006-01-02 15:04:05"))
	}
	var total int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM access_logs a WHERE 1=1`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("统计访问日志失败: %w", err)
	}
	args = append(args, size, (page-1)*size)
	rows, err := s.db.QueryContext(ctx,
		`SELECT a.id, COALESCE(a.user_id,0), COALESCE(u.username,''), a.ip, a.download_type,
		        COALESCE(a.platform,''), a.resource_slug, a.status, COALESCE(a.fail_reason,''), a.created_at
		 FROM access_logs a LEFT JOIN users u ON u.id = a.user_id
		 WHERE 1=1`+where+` ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询访问日志失败: %w", err)
	}
	defer rows.Close()
	out := make([]AccessLog, 0) // 空列表返回 [] 而非 null
	for rows.Next() {
		var l AccessLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.IP, &l.DownloadType,
			&l.Platform, &l.ResourceSlug, &l.Status, &l.FailReason, &l.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("解析访问日志行失败: %w", err)
		}
		out = append(out, l)
	}
	return out, total, rows.Err()
}

// Clear 清空全部访问日志记录（二次确认由前端 ConfirmModal 负责）
func (s *AccessService) Clear(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM access_logs`); err != nil {
		return fmt.Errorf("清空访问日志失败: %w", err)
	}
	s.log.Warn("访问日志已清空")
	return nil
}

// parseRange 解析日期范围（YYYY-MM-DD）；空串返回 nil（不限）；非法返回错误
func parseRange(from, to string) (*time.Time, *time.Time, error) {
	var start, end *time.Time
	if from != "" {
		t, err := time.ParseInLocation("2006-01-02", from, time.UTC)
		if err != nil {
			return nil, nil, errors.New("起始日期格式无效（YYYY-MM-DD）")
		}
		start = &t
	}
	if to != "" {
		t, err := time.ParseInLocation("2006-01-02", to, time.UTC)
		if err != nil {
			return nil, nil, errors.New("结束日期格式无效（YYYY-MM-DD）")
		}
		t = t.Add(24*time.Hour - time.Second) // 含当日 23:59:59
		end = &t
	}
	return start, end, nil
}
