// Package subscription 提供订阅地址池业务层：每平台一份订阅条目（装配生成模板或直接上传静态内容）、
// 四类全局标识校验与级联删除。
package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"

	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
	"vpn-sub/internal/version"
)

// 手填标识：小写字母数字连字符，3~64（Design1 §2.2，禁止修改）
var slugRe = regexp.MustCompile(`^[a-z0-9-]{3,64}$`)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrSlugConflict     = errors.New("标识已被使用（四类资源全局唯一）")
	ErrBadRequest       = errors.New("参数错误")
	ErrNotFound         = errors.New("订阅不存在")
	ErrPlatformOccupied = errors.New("该平台已有订阅条目")
)

// Service 订阅地址池服务
type Service struct {
	store    *store.Store
	versions *version.Service
	log      *slog.Logger
	// onTokenDeleted 删订阅 Token 级联回调（由 token 服务注入）
	onTokenDeleted func(ctx context.Context, tx *sql.Tx, subscriptionID int64) error
	// onAfterDelete 删订阅后的候选集重算回调（Build6 Step2）
	onAfterDelete func(ctx context.Context, subscriptionID int64)
}

func NewService(st *store.Store, versions *version.Service, lg *slog.Logger) *Service {
	return &Service{store: st, versions: versions, log: lg}
}

// SetOnTokenDeleted 注入删订阅 Token 级联回调（由 token 服务提供）
func (s *Service) SetOnTokenDeleted(fn func(ctx context.Context, tx *sql.Tx, subscriptionID int64) error) {
	s.onTokenDeleted = fn
}

// SetOnAfterDelete 注入删订阅后的候选集重算回调（Build6 Step2）。
func (s *Service) SetOnAfterDelete(fn func(ctx context.Context, subscriptionID int64)) {
	s.onAfterDelete = fn
}

// Subscription 订阅地址池条目（每平台唯一）
type Subscription struct {
	ID             int64  `json:"id"`
	Slug           string `json:"slug"`
	Name           string `json:"name"`
	PlatformID     int64  `json:"platform_id"`
	ProductType    string `json:"product_type"` // yaml / subs / generic-subs（与平台一致）
	CurrentVersion int64  `json:"current_version"`
	ContentKind    string `json:"content_kind,omitempty"`  // blueprint / upload；无激活版本时省略（前端视为 null）
	PlatformName   string `json:"platform_name,omitempty"` // 列表 JOIN 平台的展示名
}

// CreateInput 创建订阅入参（不再携带关联组与首版本；内容统一经版本管理上传或装配生成入池）
type CreateInput struct {
	PlatformID int64
	Name       string
	Slug       string
}

// CheckSlugAvailable 四类资源全局唯一命名空间交叉校验（跨四表查重，供四类资源共用）；
// 表尚未建立时跳过（sqlite_master 预检），全部建成后全量生效
func (s *Service) CheckSlugAvailable(ctx context.Context, slugVal, excludeOwner string, excludeID int64) (bool, error) {
	if !slugRe.MatchString(slugVal) {
		return false, nil // 格式不合法直接不可用
	}
	for _, table := range []string{"subscriptions", "rules", "custom_subscriptions", "share_subscriptions", "xray_instances"} {
		ok, err := tableExists(ctx, s.store.DB(), table)
		if err != nil {
			return false, err
		}
		if !ok {
			continue // 表未建立（后续 Step 才迁移）→ 跳过该表
		}
		query := `SELECT COUNT(*) FROM ` + table + ` WHERE slug = ?`
		args := []any{slugVal}
		if table == excludeOwner {
			query += ` AND id != ?`
			args = append(args, excludeID)
		}
		var n int
		if err := s.store.DB().QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
			return false, fmt.Errorf("查询 %s 标识失败: %w", table, err)
		}
		if n > 0 {
			return false, nil
		}
	}
	return true, nil
}

// tableExists 检查表是否已存在（sqlite_master 预检）
func tableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		return false, fmt.Errorf("检查表 %s 失败: %w", name, err)
	}
	return n > 0, nil
}

// tableExistsTx 事务内检查表是否存在（自动生成标识用）
func tableExistsTx(ctx context.Context, tx *sql.Tx, name string) (bool, error) {
	var n int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&n); err != nil {
		return false, fmt.Errorf("检查表 %s 失败: %w", name, err)
	}
	return n > 0, nil
}

// slugExistsTx 事务内检查标识是否已被四类资源占用（自动生成标识的 exists 回调）
func slugExistsTx(ctx context.Context, tx *sql.Tx, slugVal string) (bool, error) {
	for _, table := range []string{"subscriptions", "rules", "custom_subscriptions", "share_subscriptions", "xray_instances"} {
		ok, err := tableExistsTx(ctx, tx, table)
		if err != nil {
			return false, err
		}
		if !ok {
			continue // 表未建立 → 跳过该表
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE slug = ?`, slugVal).Scan(&n); err != nil {
			return false, fmt.Errorf("查询 %s 标识失败: %w", table, err)
		}
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// GenerateSlugTx 事务内生成四类资源全局唯一标识（类型前缀 + 8 位随机短码，冲突自动重试；供订阅/规则自动生成复用）
func GenerateSlugTx(ctx context.Context, tx *sql.Tx, prefix string) (string, error) {
	return slug.Generate(ctx, tx, prefix, func(v string) (bool, error) {
		return slugExistsTx(ctx, tx, v)
	})
}

// Create 指定平台 + 名称；product_type 从平台读取；平台唯一占用（事务内查重 + UNIQUE 索引兜底）。
// 不再创建首版本——内容统一经版本管理上传或装配生成入池。
func (s *Service) Create(ctx context.Context, in CreateInput) (*Subscription, error) {
	if in.Slug != "" && !slugRe.MatchString(in.Slug) {
		return nil, fmt.Errorf("%w: 标识须为小写字母数字连字符，长度 3~64", ErrBadRequest)
	}
	if in.PlatformID <= 0 || in.Name == "" {
		return nil, fmt.Errorf("%w: 平台与名称必填", ErrBadRequest)
	}
	if in.Slug != "" {
		ok, err := s.CheckSlugAvailable(ctx, in.Slug, "", 0)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrSlugConflict // 接入层映射 409
		}
	}
	var created *Subscription
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var productType string
		if err := tx.QueryRowContext(ctx,
			`SELECT product_type FROM platforms WHERE id = ?`, in.PlatformID).Scan(&productType); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrBadRequest // 平台不存在
			}
			return err
		}
		var occupied int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM subscriptions WHERE platform_id = ?`, in.PlatformID).Scan(&occupied); err != nil {
			return err
		}
		if occupied > 0 {
			return ErrPlatformOccupied // 事务内查重，防并发绕过 UNIQUE
		}
		if in.Slug == "" {
			// 自动生成：事务内跨四类唯一性检查，冲突自动重试
			slugVal, err := GenerateSlugTx(ctx, tx, "subscription-")
			if err != nil {
				return err
			}
			in.Slug = slugVal
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO subscriptions (slug, name, platform_id, product_type) VALUES (?,?,?,?)`,
			in.Slug, in.Name, in.PlatformID, productType)
		if err != nil {
			// UNIQUE(platform_id) 索引兜底：并发创建冲突同样映射 409
			if isUniqueViolation(err) {
				return ErrPlatformOccupied
			}
			return fmt.Errorf("创建订阅失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Subscription{ID: id, Slug: in.Slug, Name: in.Name, PlatformID: in.PlatformID, ProductType: productType}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.log.Info("订阅已创建", "id", created.ID, "slug", created.Slug)
	return created, nil
}

// isUniqueViolation 识别 UNIQUE 约束冲突（platform 占用 / slug 冲突）
func isUniqueViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "constraint failed")
}

// Update 仅可改名称；平台与 product_type 只读
func (s *Service) Update(ctx context.Context, id int64, name string) error {
	if name == "" {
		return fmt.Errorf("%w: 名称必填", ErrBadRequest)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE subscriptions SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, name, id); err != nil {
			return fmt.Errorf("更新订阅失败: %w", err)
		}
		return nil
	})
}

// Delete 级联删除：全部版本文件 + 指向它的下载 Token（含老库残留显式 Token）+ 订阅行
func (s *Service) Delete(ctx context.Context, id int64) error {
	var files []string
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		// 1) 收集全部版本文件路径 → 删版本记录（文件提交后删）
		collected, err := s.versions.CollectVersionFiles(ctx, tx, version.OwnerSubscription, id)
		if err != nil {
			return err
		}
		files = collected
		if err := s.versions.DeleteVersionsTx(ctx, tx, version.OwnerSubscription, id); err != nil {
			return err
		}
		// 2) 级联删指向它的下载 Token（含显式预览 Token）
		if s.onTokenDeleted != nil {
			if err := s.onTokenDeleted(ctx, tx, id); err != nil {
				return err
			}
		}
		// 3) 删订阅行（装配蓝图随 versions 外键 ON DELETE CASCADE 级联）
		if _, err := tx.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除订阅失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务提交后删版本文件（失败记日志不阻断，与平台删除同模式）
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("删除版本文件失败", "path", f, "err", err)
		}
	}
	if err := s.versions.RemoveOwnerDir(version.OwnerSubscription, id); err != nil {
		s.log.Warn("删除订阅版本目录失败", "id", id, "err", err)
	}
	if s.onAfterDelete != nil {
		s.onAfterDelete(ctx, id)
	}
	return nil
}

// Get 单个订阅（编辑回显：名称 + 平台/格式只读信息 + 内容形态）
func (s *Service) Get(ctx context.Context, id int64) (*Subscription, error) {
	var sub Subscription
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, slug, name, platform_id, COALESCE(current_version,0), product_type
		 FROM subscriptions WHERE id = ?`, id).
		Scan(&sub.ID, &sub.Slug, &sub.Name, &sub.PlatformID, &sub.CurrentVersion, &sub.ProductType)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取订阅失败: %w", err)
	}
	sub.ContentKind, err = s.contentKind(ctx, sub.ID, sub.CurrentVersion)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// List 平铺列表（每平台至多一份条目），含平台名与内容形态
func (s *Service) List(ctx context.Context) ([]Subscription, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT s.id, s.slug, s.name, s.platform_id, COALESCE(s.current_version,0), s.product_type, p.name
		 FROM subscriptions s JOIN platforms p ON p.id = s.platform_id ORDER BY p.id, s.id`)
	if err != nil {
		return nil, fmt.Errorf("读取订阅列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Subscription, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for rows.Next() {
		var sub Subscription
		if err := rows.Scan(&sub.ID, &sub.Slug, &sub.Name, &sub.PlatformID, &sub.CurrentVersion, &sub.ProductType, &sub.PlatformName); err != nil {
			return nil, fmt.Errorf("解析订阅行失败: %w", err)
		}
		out = append(out, sub)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 先关闭/释放外层 Rows，再逐行补充 contentKind，
	// 避免 SetMaxOpenConns(1) 下嵌套查询导致唯一连接死锁。
	if err := rows.Close(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].ContentKind, err = s.contentKind(ctx, out[i].ID, out[i].CurrentVersion)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// contentKind 当前激活版本的内容形态：assembly_blueprints 存在 → blueprint，否则 upload；无激活版本为空
func (s *Service) contentKind(ctx context.Context, id, currentVersion int64) (string, error) {
	if currentVersion <= 0 {
		return "", nil
	}
	ok, err := tableExists(ctx, s.store.DB(), "assembly_blueprints")
	if err != nil || !ok {
		return "", err
	}
	var n int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM assembly_blueprints b
		 JOIN versions v ON v.id = b.version_id
		 WHERE v.owner_type = 'subscription' AND v.owner_id = ? AND v.version_no = ?`,
		id, currentVersion).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		return "blueprint", nil
	}
	return "upload", nil
}

// CheckSlug 便捷包装：标识格式 + 跨四类可用性（供 slug/check 端点）
func (s *Service) CheckSlug(ctx context.Context, slugVal, ownerType string, ownerID int64) (bool, error) {
	return s.CheckSlugAvailable(ctx, slugVal, ownerType, ownerID)
}
