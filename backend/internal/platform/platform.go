// Package platform 提供平台资源业务层：CRUD、scheme 排序、附加响应头校验、安装包分发与级联删除。
package platform

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"vpn-sub/internal/slug"
	"vpn-sub/internal/store"
	"vpn-sub/internal/version"
)

// 关键参数（Design1 §6.3/3.4.4，禁止修改）
const (
	MaxInstallerSize = 300 << 20 // 安装包 ≤300MB
	MaxInstallerName = 200       // 展示名长度上限（安装包原始文件名/外链展示名）
	installerDir     = "public/installers"

	// 平台/订阅产物格式（Design2 §4.4/§5.9）
	ProductYAML        = "yaml"
	ProductSubs        = "subs"
	ProductGenericSubs = "generic-subs"
)

// 业务错误（接入层映射 HTTP 状态码）
var (
	ErrBadRequest         = errors.New("参数错误")
	ErrNotFound           = errors.New("平台不存在")
	ErrInstallerTooLarge  = errors.New("安装包超过 300MB 限制")
	ErrUnsafeInstallerExt = errors.New("安装包扩展名不安全，仅允许可下载安装包格式")
	ErrProductTypeInUse   = errors.New("该平台已有订阅条目，请先处理后再变更产物格式")
)

// dangerousInstallerExts 上传安装包时拒绝的危险/可执行/可被浏览器同源解析的扩展名。
// 该黑名单为安全收紧项，附件下载与 nosniff 仍作为纵深防御保留。
var dangerousInstallerExts = map[string]bool{
	".html": true, ".htm": true, ".xhtml": true,
	".svg": true, ".svgz": true,
	".js": true, ".mjs": true, ".xml": true,
}

// productTypeInUseError 平台产物格式变更冲突（携带既有订阅条目的 product_type 插值文案）
type productTypeInUseError struct {
	existing string
}

func (e *productTypeInUseError) Error() string {
	return "该平台已有 " + e.existing + " 订阅条目，请先处理后再变更产物格式"
}

func (e *productTypeInUseError) Unwrap() error { return ErrProductTypeInUse }

func validProductType(v string) bool {
	return v == ProductYAML || v == ProductSubs || v == ProductGenericSubs
}

// Service 平台服务
type Service struct {
	store         *store.Store
	dataDir       string           // 安装包落盘根目录（/data）
	versions      *version.Service // 版本组件（Step 5 起用于平台删除完整级联）
	log           *slog.Logger
	onAfterDelete func(ctx context.Context)
}

func NewService(st *store.Store, dataDir string, versions *version.Service, lg *slog.Logger) *Service {
	return &Service{store: st, dataDir: dataDir, versions: versions, log: lg}
}

// SetOnAfterDelete 注入平台删除后的候选集重算回调（Build6 Step2）。
func (s *Service) SetOnAfterDelete(fn func(ctx context.Context)) {
	s.onAfterDelete = fn
}

// InstallerFileItem 本地上传安装包条目：name=原始文件名（展示用），file=磁盘文件名（时间戳）
type InstallerFileItem struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// InstallerURLItem 外部下载链接条目：name=展示名（可空，空则前端展示 URL 本身）
type InstallerURLItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Platform 平台资源
type Platform struct {
	ID             int64               `json:"id"`
	Slug           string              `json:"slug"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	ProductType    string              `json:"product_type"`    // yaml / subs / generic-subs
	IsDefault      bool                `json:"is_default"`      // 内置默认平台，产物格式锁定
	Schemes        []string            `json:"schemes"`         // 有序数组；一键导入取首项；含 {url} 占位符
	ExtraHeaders   map[string]string   `json:"extra_headers"`   // 附加响应头；值支持 {frontend_url} 占位符
	InstallerFiles []InstallerFileItem `json:"installer_files"` // 多个本地安装包
	InstallerURLs  []InstallerURLItem  `json:"installer_urls"`  // 多个外部下载链接
	Cascade        CascadeCounts       `json:"cascade"`         // 删除预览用影响统计
}

// CascadeCounts 删除平台的影响统计（订阅/Token/自定义数量；表未建立时计 0）
type CascadeCounts struct {
	Subscriptions int64 `json:"subscriptions"`
	Tokens        int64 `json:"tokens"`
	Customs       int64 `json:"customs"`
}

// Create 创建平台：slug 由生成器自动生成（platform- 前缀）；名称不强制唯一；product_type 默认 yaml；
// 可携带外部下载链接列表
func (s *Service) Create(ctx context.Context, name, description, productType string, schemes []string, headers map[string]string, installerURLs []InstallerURLItem) (*Platform, error) {
	if productType == "" {
		productType = ProductYAML
	}
	if !validProductType(productType) {
		return nil, fmt.Errorf("%w: product_type 仅支持 yaml/subs/generic-subs", ErrBadRequest)
	}
	if err := ValidateSchemes(schemes); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := ValidateExtraHeaders(headers); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	installerURLs, err := ValidateInstallerURLs(installerURLs)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	var created *Platform
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		value, err := slug.Generate(ctx, tx, "platform-", func(v string) (bool, error) {
			return slug.TableHasSlug(ctx, tx, "platforms", v)
		})
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx,
			`INSERT INTO platforms (slug, name, description, product_type, schemes, extra_headers, installer_urls) VALUES (?,?,?,?,?,?,?)`,
			value, name, description, productType, toJSON(schemes), toJSON(headers), toJSON(installerURLs))
		if err != nil {
			return fmt.Errorf("创建平台失败: %w", err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		created = &Platform{ID: id, Slug: value, Name: name, Description: description,
			ProductType: productType, Schemes: schemes, ExtraHeaders: headers, InstallerURLs: installerURLs}
		return nil
	})
	return created, err
}

// Update 编辑平台：创建后 slug 不可修改（接入层不接收 slug 字段）；可改名称/描述/product_type/scheme/附加头/外部下载链接列表。
// product_type 变更时校验与既有订阅条目一致（有冲突返回 ErrProductTypeInUse，接入层 400）。
func (s *Service) Update(ctx context.Context, id int64, name, description, productType string, schemes []string, headers map[string]string, installerURLs []InstallerURLItem) error {
	if productType == "" {
		productType = ProductYAML
	}
	if !validProductType(productType) {
		return fmt.Errorf("%w: product_type 仅支持 yaml/subs/generic-subs", ErrBadRequest)
	}
	if err := ValidateSchemes(schemes); err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	if err := ValidateExtraHeaders(headers); err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	installerURLs, err := ValidateInstallerURLs(installerURLs)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadRequest, err)
	}
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var exists int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM platforms WHERE id = ?`, id).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		var isDefault int
		var currentProductType string
		if err := tx.QueryRowContext(ctx,
			`SELECT is_default, product_type FROM platforms WHERE id = ?`, id).Scan(&isDefault, &currentProductType); err != nil {
			return err
		}
		if isDefault == 1 && productType != currentProductType {
			return fmt.Errorf("%w: 默认平台产物格式不可修改", ErrBadRequest)
		}
		var existing string
		err := tx.QueryRowContext(ctx,
			`SELECT s.product_type FROM subscriptions s WHERE s.platform_id = ? AND s.product_type != ? LIMIT 1`,
			id, productType).Scan(&existing)
		if err == nil {
			return &productTypeInUseError{existing: existing}
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE platforms SET name = ?, description = ?, product_type = ?, schemes = ?, extra_headers = ?, installer_urls = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			name, description, productType, toJSON(schemes), toJSON(headers), toJSON(installerURLs), id); err != nil {
			return fmt.Errorf("更新平台失败: %w", err)
		}
		return nil
	})
}

// Get 读取单个平台（编辑回显）
func (s *Service) Get(ctx context.Context, id int64) (*Platform, error) {
	var p Platform
	var schemesJSON, headersJSON, filesJSON, urlsJSON sql.NullString
	var isDefault int
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT id, slug, name, description, product_type, is_default, schemes, extra_headers, installer_files, installer_urls FROM platforms WHERE id = ?`, id).
		Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.ProductType, &isDefault, &schemesJSON, &headersJSON, &filesJSON, &urlsJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("读取平台失败: %w", err)
	}
	p.IsDefault = isDefault == 1
	p.Schemes = parseJSONSlice(schemesJSON.String)
	p.ExtraHeaders = parseJSONMap(headersJSON.String)
	p.InstallerFiles = parseInstallerFiles(filesJSON.String)
	p.InstallerURLs = parseInstallerURLs(urlsJSON.String)
	return &p, nil
}

// List 平台列表（附删除预览影响统计；订阅/Token/自定义表未建立时跳过统计）
func (s *Service) List(ctx context.Context) ([]Platform, error) {
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, slug, name, description, product_type, is_default, schemes, extra_headers, installer_files, installer_urls FROM platforms ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("读取平台列表失败: %w", err)
	}
	defer rows.Close()
	out := make([]Platform, 0) // 空列表返回 [] 而非 null（前端 .map 安全）
	for rows.Next() {
		var p Platform
		var isDefault int
		var schemesJSON, headersJSON, filesJSON, urlsJSON sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.Description, &p.ProductType, &isDefault, &schemesJSON, &headersJSON, &filesJSON, &urlsJSON); err != nil {
			return nil, fmt.Errorf("解析平台行失败: %w", err)
		}
		p.IsDefault = isDefault == 1
		p.Schemes = parseJSONSlice(schemesJSON.String)
		p.ExtraHeaders = parseJSONMap(headersJSON.String)
		p.InstallerFiles = parseInstallerFiles(filesJSON.String)
		p.InstallerURLs = parseInstallerURLs(urlsJSON.String)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// 删除预览影响统计（表缺失跳过，Step 2/4/5 迁移后自动回填）
	for i := range out {
		c, err := s.cascadeCounts(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Cascade = c
	}
	return out, nil
}

// cascadeCounts 统计平台下订阅/Token/自定义数量；表未建立时计 0
func (s *Service) cascadeCounts(ctx context.Context, platformID int64) (CascadeCounts, error) {
	var c CascadeCounts
	var err error
	if c.Subscriptions, err = s.countIfTableExists(ctx, "subscriptions", `SELECT COUNT(*) FROM subscriptions WHERE platform_id = ?`, platformID); err != nil {
		return c, err
	}
	if c.Tokens, err = s.countIfTableExists(ctx, "download_tokens", `SELECT COUNT(*) FROM download_tokens WHERE platform_id = ?`, platformID); err != nil {
		return c, err
	}
	if c.Customs, err = s.countIfTableExists(ctx, "custom_subscriptions", `SELECT COUNT(*) FROM custom_subscriptions WHERE platform_id = ?`, platformID); err != nil {
		return c, err
	}
	return c, nil
}

// countIfTableExists 表存在时执行 COUNT，否则返回 0（sqlite_master 预检）
func (s *Service) countIfTableExists(ctx context.Context, table, query string, arg int64) (int64, error) {
	var n int
	if err := s.store.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, nil
	}
	var count int64
	if err := s.store.DB().QueryRowContext(ctx, query, arg).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// --- 校验 ---

// ValidateSchemes scheme 列表：有序数组，值含 {url} 占位符；拒绝控制字符与空项
func ValidateSchemes(schemes []string) error {
	for _, v := range schemes {
		if strings.TrimSpace(v) == "" {
			return errors.New("scheme 不能为空")
		}
		if containsControl(v) {
			return errors.New("scheme 含控制字符")
		}
	}
	return nil
}

// ValidateExtraHeaders 键与值均禁止 \r\n 等控制字符；键另须符合 HTTP 头名规范（防响应头注入）
var headerNameRe = regexp.MustCompile(`^[!#$%&'*+\-.^_` + "`" + `|~0-9A-Za-z]+$`) // RFC 7230 token

func ValidateExtraHeaders(h map[string]string) error {
	for k, v := range h {
		if !headerNameRe.MatchString(k) {
			return fmt.Errorf("附加头键 %q 不符合 HTTP 头名规范", k)
		}
		if containsControl(k) || containsControl(v) {
			return fmt.Errorf("附加头 %q 含控制字符", k)
		}
	}
	return nil
}

func containsControl(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// ValidateInstallerURLs 外部下载链接列表校验：地址仅允许 http/https（防 javascript: 等伪协议注入）、
// 禁止控制字符；展示名长度受限（可空）
func ValidateInstallerURLs(items []InstallerURLItem) ([]InstallerURLItem, error) {
	out := make([]InstallerURLItem, 0, len(items))
	for _, it := range items {
		if strings.TrimSpace(it.URL) == "" {
			return nil, errors.New("外部下载链接地址不能为空")
		}
		if containsControl(it.Name) || containsControl(it.URL) {
			return nil, errors.New("外部下载链接含控制字符")
		}
		u, err := url.Parse(strings.TrimSpace(it.URL))
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return nil, fmt.Errorf("外部下载链接仅支持 http/https 地址: %s", it.URL)
		}
		if len(it.Name) > MaxInstallerName {
			return nil, errors.New("外部下载链接展示名过长")
		}
		out = append(out, InstallerURLItem{Name: strings.TrimSpace(it.Name), URL: strings.TrimSpace(it.URL)})
	}
	return out, nil
}

// --- 安装包：流式追加上传 + 事务内列表更新 + 单独删除（防并发互删，Design1 §4.7）---

// installerAbs 安装包绝对路径：文件名必须为基本文件名（防 DB 篡改后路径逃逸，AGENTS §4.1）
func (s *Service) installerAbs(name string) (string, error) {
	if filepath.Base(name) != name {
		return "", fmt.Errorf("安装包文件名非法: %s", name)
	}
	return filepath.Join(s.dataDir, installerDir, name), nil
}

// UploadInstaller ≤300MB 流式落盘（禁止整读内存）；文件名带时间戳（URL 变化突破 CDN 缓存）；
// 追加式（多安装包并存）；BEGIN IMMEDIATE 事务内：读列表 → 生成唯一新名（O_EXCL 防并发同名）→ 写新文件 →
// 更新 DB 列表 → 提交后返回更新后列表（任一步失败完整清理，Design1 §4.7）
func (s *Service) UploadInstaller(ctx context.Context, id int64, body io.Reader, filename string) ([]InstallerFileItem, error) {
	ext := filepath.Ext(filepath.Base(filename)) // 路径穿越防护：仅取基名扩展名，丢弃任何目录部分
	ext = sanitizeExt(ext)
	if dangerousInstallerExts[ext] {
		return nil, ErrUnsafeInstallerExt
	}
	name := sanitizeInstallerName(filepath.Base(filename)) // 展示名：原始文件名（控制字符剥离 + 长度上限）
	if err := os.MkdirAll(filepath.Join(s.dataDir, installerDir), 0o755); err != nil {
		return nil, fmt.Errorf("创建安装包目录失败: %w", err)
	}
	var updated []InstallerFileItem
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var raw string
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(installer_files,'[]') FROM platforms WHERE id = ?`, id).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		list := parseInstallerFiles(raw)
		// 事务内生成唯一文件名（事务串行化 + O_EXCL 双重保证，防并发互删）
		var newName string
		for attempt := 0; attempt < 3; attempt++ {
			candidate := fmt.Sprintf("installer-%d%s", time.Now().UnixNano(), ext)
			f, err := os.OpenFile(filepath.Join(s.dataDir, installerDir, candidate),
				os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err != nil {
				if errors.Is(err, os.ErrExist) {
					continue // 同名冲突（极端并发），重试新名
				}
				return fmt.Errorf("创建安装包文件失败: %w", err)
			}
			// 流式落盘：io.Copy 限流包装，超限即中止并清理
			written, copyErr := io.Copy(f, io.LimitReader(body, MaxInstallerSize+1))
			if closeErr := f.Close(); copyErr == nil {
				copyErr = closeErr
			}
			if copyErr != nil {
				_ = os.Remove(f.Name()) // 失败清理
				return fmt.Errorf("安装包写入失败: %w", copyErr)
			}
			if written > MaxInstallerSize {
				_ = os.Remove(f.Name())
				return ErrInstallerTooLarge // 接入层映射 400
			}
			newName = candidate
			break
		}
		if newName == "" {
			return errors.New("安装包文件名连续冲突，请重试")
		}
		updated = append(list, InstallerFileItem{Name: name, File: newName})
		if _, err := tx.ExecContext(ctx,
			`UPDATE platforms SET installer_files = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, toJSON(updated), id); err != nil {
			_ = os.Remove(filepath.Join(s.dataDir, installerDir, newName))
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// DeleteInstallerFile 单独删除一个本地安装包（按磁盘文件名定位）；事务内读列表 → 清条目 → 提交后删文件
func (s *Service) DeleteInstallerFile(ctx context.Context, id int64, file string) error {
	if filepath.Base(file) != file || file == "" {
		return ErrBadRequest
	}
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var raw string
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(installer_files,'[]') FROM platforms WHERE id = ?`, id).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		list := parseInstallerFiles(raw)
		kept := make([]InstallerFileItem, 0, len(list))
		found := false
		for _, it := range list {
			if it.File == file {
				found = true
				continue
			}
			kept = append(kept, it)
		}
		if !found {
			return nil // 无此安装包，幂等成功
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE platforms SET installer_files = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, toJSON(kept), id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 提交后删文件（删除失败仅记日志，不影响列表结果）
	if path, err := s.installerAbs(file); err != nil {
		s.log.Warn("安装包文件名非法，跳过删除", "file", file, "err", err)
	} else if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.log.Warn("删除安装包文件失败", "file", file, "err", err)
	}
	return nil
}

// sanitizeInstallerName 清洗安装包展示名（原始文件名）：剥高控制字符、限制长度（防 JSON/日志注入与超长列）
func sanitizeInstallerName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	if b.Len() > MaxInstallerName {
		return b.String()[:MaxInstallerName]
	}
	return b.String()
}

// sanitizeExt 清洗安装包扩展名：小写化 + 仅保留安全字符（字母/数字/点），剥除路径分隔符与控制字符，
// 防路径穿越与危险文件名（Design1 §6.3 明确「扩展名不做白名单限制」，仅大小校验）；空扩展名返回空串
func sanitizeExt(ext string) string {
	ext = strings.ToLower(ext)
	var b strings.Builder
	for _, r := range ext {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Delete 级联删除（Design1 §4.4，关键约束）：全部安装包文件 + 全部订阅（含版本文件）+ 指向它们的下载 Token
// + 全部自定义订阅（含版本文件与 Token）；订阅/自定义订阅按平台唯一条目模型级联清理（R14-22 新语义）
func (s *Service) Delete(ctx context.Context, id int64) error {
	var installers []InstallerFileItem
	var files []string
	var subIDs, customIDs []int64
	err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		var err error
		var raw string
		if err := tx.QueryRowContext(ctx,
			`SELECT COALESCE(installer_files,'[]') FROM platforms WHERE id = ?`, id).Scan(&raw); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		installers = parseInstallerFiles(raw)
		// 1) 收集该平台全部订阅/自定义订阅 ID
		subIDs, err = s.collectIDs(ctx, tx, `SELECT id FROM subscriptions WHERE platform_id = ?`, id)
		if err != nil {
			return err
		}
		customIDs, err = s.collectIDs(ctx, tx, `SELECT id FROM custom_subscriptions WHERE platform_id = ?`, id)
		if err != nil {
			return err
		}
		// 2) 删指向该平台的下载 Token（含自定义 Token 与显式 Token）
		if _, err := tx.ExecContext(ctx, `DELETE FROM download_tokens WHERE platform_id = ?`, id); err != nil {
			return err
		}
		// 3) 删订阅版本（文件提交后统一删）与自定义版本
		for _, sid := range subIDs {
			collected, err := s.versions.CollectVersionFiles(ctx, tx, version.OwnerSubscription, sid)
			if err != nil {
				return err
			}
			files = append(files, collected...)
			if err := s.versions.DeleteVersionsTx(ctx, tx, version.OwnerSubscription, sid); err != nil {
				return err
			}
		}
		for _, cid := range customIDs {
			collected, err := s.versions.CollectVersionFiles(ctx, tx, version.OwnerCustom, cid)
			if err != nil {
				return err
			}
			files = append(files, collected...)
			if err := s.versions.DeleteVersionsTx(ctx, tx, version.OwnerCustom, cid); err != nil {
				return err
			}
		}
		// 4) 删订阅（订阅-组关联由外键 ON DELETE CASCADE 清理）
		if _, err := tx.ExecContext(ctx, `DELETE FROM subscriptions WHERE platform_id = ?`, id); err != nil {
			return err
		}
		// 5) 删自定义订阅
		if _, err := tx.ExecContext(ctx, `DELETE FROM custom_subscriptions WHERE platform_id = ?`, id); err != nil {
			return err
		}
		// 6) 删平台（订阅/自定义订阅已先级联清理；无旧“组选定”模型，R14-22）
		if _, err := tx.ExecContext(ctx, `DELETE FROM platforms WHERE id = ?`, id); err != nil {
			return fmt.Errorf("删除平台失败: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务提交后删文件（失败仅记日志，不影响删除结果）
	for _, it := range installers {
		if full, err := s.installerAbs(it.File); err != nil {
			s.log.Warn("安装包文件名非法，跳过删除", "file", it.File, "err", err)
		} else if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("删除安装包文件失败", "file", it.File, "err", err)
		}
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.log.Warn("删除版本文件失败", "path", f, "err", err)
		}
	}
	// 版本目录清理（订阅与自定义）
	for _, sid := range subIDs {
		if err := s.versions.RemoveOwnerDir(version.OwnerSubscription, sid); err != nil {
			s.log.Warn("删除订阅版本目录失败", "id", sid, "err", err)
		}
	}
	for _, cid := range customIDs {
		if err := s.versions.RemoveOwnerDir(version.OwnerCustom, cid); err != nil {
			s.log.Warn("删除自定义版本目录失败", "id", cid, "err", err)
		}
	}
	if s.onAfterDelete != nil {
		s.onAfterDelete(ctx)
	}
	return nil
}

// collectIDs 收集平台下资源 ID 列表
func (s *Service) collectIDs(ctx context.Context, tx *sql.Tx, query string, arg int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, query, arg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// --- JSON 助手 ---

func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func parseJSONSlice(raw string) []string {
	var out []string
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func parseJSONMap(raw string) map[string]string {
	out := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

// parseInstallerFiles 解析安装包列表 JSON；非法条目直接跳过（DB 数据异常不阻断读取）
func parseInstallerFiles(raw string) []InstallerFileItem {
	var out []InstallerFileItem
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	kept := make([]InstallerFileItem, 0, len(out))
	for _, it := range out {
		if it.File != "" && filepath.Base(it.File) == it.File {
			kept = append(kept, it)
		}
	}
	return kept
}

// parseInstallerURLs 解析外部下载链接列表 JSON；非法条目直接跳过（DB 数据异常不阻断读取）
func parseInstallerURLs(raw string) []InstallerURLItem {
	var out []InstallerURLItem
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	kept := make([]InstallerURLItem, 0, len(out))
	for _, it := range out {
		if it.URL != "" {
			kept = append(kept, it)
		}
	}
	return kept
}
