// Package subscription 提供订阅池业务层：CRUD、四类全局标识校验与级联删除。
package subscription

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"

	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
	"vpn-sub/internal/version"
)

// 手填标识：小写字母数字连字符，3~64（Design1 §2.2，禁止修改）
var slugRe = regexp.MustCompile(`^[a-z0-9-]{3,64}$`)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrSlugConflict = errors.New("标识已被使用（四类资源全局唯一）")
	ErrBadRequest   = errors.New("参数错误")
	ErrNotFound     = errors.New("订阅不存在")
)

// Service 订阅池服务
type Service struct {
	store    *store.Store
	versions *version.Service
	log      *slog.Logger
	// onSubDeleted 删订阅级联回调（Build2 Step 3 由 group 服务注入；回调注入防包级循环依赖）
	onSubDeleted func(ctx context.Context, tx *sql.Tx, subscriptionID int64) error
	// onTokenDeleted 删订阅 Token 级联回调（Build2 Step 4 由 token 服务注入）
	onTokenDeleted func(ctx context.Context, tx *sql.Tx, subscriptionID int64) error
}

func NewService(st *store.Store, versions *version.Service, lg *slog.Logger) *Service {
	return &Service{store: st, versions: versions, log: lg}
}

// SetOnSubscriptionDeleted 注入删订阅级联回调（清组选定 + 置 needs_reselect，由 group 服务提供）
func (s *Service) SetOnSubscriptionDeleted(fn func(ctx context.Context, tx *sql.Tx, subscriptionID int64) error) {
	s.onSubDeleted = fn
}

// SetOnTokenDeleted 注入删订阅 Token 级联回调（由 token 服务提供）
func (s *Service) SetOnTokenDeleted(fn func(ctx context.Context, tx *sql.Tx, subscriptionID int64) error) {
	s.onTokenDeleted = fn
}

// GroupBrief 关联组摘要
type GroupBrief struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// Subscription 订阅
type Subscription struct {
	ID             int64        `json:"id"`
	Slug           string       `json:"slug"`
	Name           string       `json:"name"`
	PlatformID     int64        `json:"platform_id"`
	CurrentVersion int64        `json:"current_version"`
	Groups         []GroupBrief `json:"groups"`        // 关联用户组
	SelectedBy     int64        `json:"selected_by"`   // 被多少组选定中（Step 3 接通）
}

// PlatformGroup 按平台分组的订阅列表
type PlatformGroup struct {
	PlatformID   int64          `json:"platform_id"`
	PlatformName string         `json:"platform_name"`
	Subscriptions []Subscription `json:"subscriptions"`
}

// CreateInput 创建订阅入参（首版本内容可选）
type CreateInput struct {
	PlatformID   int64
	Name         string
	Slug         string
	GroupIDs     []int64
	FirstContent version.ContentProvider // 可选：创建时同时建立首版本
}

// CheckSlugAvailable 四类资源全局唯一命名空间交叉校验（跨四表查重，供四类资源共用）；
// 表尚未建立时跳过（sqlite_master 预检），全部建成后全量生效
func (s *Service) CheckSlugAvailable(ctx context.Context, slugVal, excludeOwner string, excludeID int64) (bool, error) {
	if !slugRe.MatchString(slugVal) {
		return false, nil // 格式不合法直接不可用
	}
	for _, table := range []string{"subscriptions", "rules", "custom_subscriptions", "share_subscriptions"} {
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
	for _, table := range []string{"subscriptions", "rules", "custom_subscriptions", "share_subscriptions"} {
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

// Create 指定平台 + 名称 + 关联组多选（可空）+ 首版本内容（可选）；标识为空时自动生成（subscription- 前缀，见 Design1 §2.2）
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
		var plat int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM platforms WHERE id = ?`, in.PlatformID).Scan(&plat); err != nil {
			return err
		}
		if plat == 0 {
			return ErrBadRequest // 平台不存在
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
			`INSERT INTO subscriptions (slug, name, platform_id) VALUES (?,?,?)`, in.Slug, in.Name, in.PlatformID)
		if err != nil {
			return fmt.Errorf("创建订阅失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		for _, gid := range in.GroupIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO subscription_group_rel (subscription_id, group_id) VALUES (?,?)`, id, gid); err != nil {
				return fmt.Errorf("写入订阅-组关联失败: %w", err)
			}
		}
		created = &Subscription{ID: id, Slug: in.Slug, Name: in.Name, PlatformID: in.PlatformID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	// 首版本内容（独立事务调用版本组件；失败回滚订阅创建，失败清理模式）
	if in.FirstContent != nil {
		v, err := s.versions.CreateVersion(ctx, version.OwnerSubscription, created.ID, in.FirstContent)
		if err != nil {
			s.rollbackCreate(ctx, created.ID)
			return nil, err
		}
		created.CurrentVersion = v.No
	}
	s.log.Info("订阅已创建", "id", created.ID, "slug", created.Slug)
	return created, nil
}

// rollbackCreate 首版本创建失败时回滚订阅创建（删关联 + 订阅行）
func (s *Service) rollbackCreate(ctx context.Context, id int64) {
	if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		_, _ = tx.ExecContext(ctx, `DELETE FROM subscription_group_rel WHERE subscription_id = ?`, id)
		_, err := tx.ExecContext(ctx, `DELETE FROM subscriptions WHERE id = ?`, id)
		return err
	}); err != nil {
		s.log.Error("回滚订阅创建失败", "id", id, "err", err)
	}
}

// Update 仅可改名称与关联组；平台只读（创建后不可修改）；
// 取消关联受「该组正在选定此订阅则拒绝」约束（Step 3 接通 group_selections 校验，本 Step 预留）
func (s *Service) Update(ctx context.Context, id int64, name string, groupIDs []int64) error {
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
		// 重建关联（TODO(Build2 Step 3)：校验被移除的组是否在 group_selections 中选定此订阅，是则拒绝并提示先改选）
		if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_group_rel WHERE subscription_id = ?`, id); err != nil {
			return err
		}
		for _, gid := range groupIDs {
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO subscription_group_rel (subscription_id, group_id) VALUES (?,?)`, id, gid); err != nil {
				return fmt.Errorf("写入订阅-组关联失败: %w", err)
			}
		}
		return nil
	})
}

// Delete 级联删除（Design1 §4.4）：全部版本文件 + 指向它的下载 Token（Step 4 接入）
// + 所有组的关联与选定（Step 3 接入）；受影响组置空不回退并置 needs_reselect
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
		// 3) 删订阅-组关联（Step 3 起由 OnSubscriptionDeleted 统一清理：关联 + 选定 + needs_reselect）
		if s.onSubDeleted != nil {
			if err := s.onSubDeleted(ctx, tx, id); err != nil {
				return err
			}
		} else {
			if _, err := tx.ExecContext(ctx, `DELETE FROM subscription_group_rel WHERE subscription_id = ?`, id); err != nil {
				return err
			}
		}
		// 4) 删订阅行
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
	return nil
}

// Get 单个订阅（编辑回显：含关联组）
func (s *Service) Get(ctx context.Context, id int64) (*Subscription, error) {
	var sub Subscription
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, slug, name, platform_id, COALESCE(current_version,0) FROM subscriptions WHERE id = ?`, id).
		Scan(&sub.ID, &sub.Slug, &sub.Name, &sub.PlatformID, &sub.CurrentVersion)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取订阅失败: %w", err)
	}
	sub.Groups, err = s.groupRel(ctx, id)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

// List 按平台分组的订阅列表，含关联组与「被哪些组选定中」数量（Step 3 接通）
func (s *Service) List(ctx context.Context) ([]PlatformGroup, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT s.id, s.slug, s.name, s.platform_id, COALESCE(s.current_version,0), p.name
		 FROM subscriptions s JOIN platforms p ON p.id = s.platform_id ORDER BY p.id, s.id`)
	if err != nil {
		return nil, fmt.Errorf("读取订阅列表失败: %w", err)
	}
	defer rows.Close()
	type row struct{ sub Subscription; platformName string }
	var items []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.sub.ID, &r.sub.Slug, &r.sub.Name, &r.sub.PlatformID, &r.sub.CurrentVersion, &r.platformName); err != nil {
			return nil, fmt.Errorf("解析订阅行失败: %w", err)
		}
		items = append(items, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 分组 + 关联组 + 选定数
	out := make([]PlatformGroup, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	index := map[int64]int{}
	for _, it := range items {
		idx, ok := index[it.sub.PlatformID]
		if !ok {
			idx = len(out)
			index[it.sub.PlatformID] = idx
			out = append(out, PlatformGroup{PlatformID: it.sub.PlatformID, PlatformName: it.platformName})
		}
		sub := it.sub
		if sub.Groups, err = s.groupRel(ctx, sub.ID); err != nil {
			return nil, err
		}
		sub.SelectedBy, err = s.selectedByCount(ctx, sub.ID)
		if err != nil {
			return nil, err
		}
		out[idx].Subscriptions = append(out[idx].Subscriptions, sub)
	}
	return out, nil
}

// groupRel 订阅的关联组列表
func (s *Service) groupRel(ctx context.Context, id int64) ([]GroupBrief, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT g.id, g.name FROM subscription_group_rel r JOIN groups g ON g.id = r.group_id
		 WHERE r.subscription_id = ? ORDER BY g.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]GroupBrief, 0) // 空列表返回 [] 而非 null
	for rows.Next() {
		var g GroupBrief
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// selectedByCount 被多少组选定中（group_selections 表 Step 3 建立，未建立时计 0）
func (s *Service) selectedByCount(ctx context.Context, id int64) (int64, error) {
	ok, err := tableExists(ctx, s.store.DB(), "group_selections")
	if err != nil || !ok {
		return 0, err
	}
	var n int64
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM group_selections WHERE subscription_id = ?`, id).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CheckSlug 便捷包装：标识格式 + 跨四类可用性（供 slug/check 端点）
func (s *Service) CheckSlug(ctx context.Context, slugVal, ownerType string, ownerID int64) (bool, error) {
	return s.CheckSlugAvailable(ctx, slugVal, ownerType, ownerID)
}
