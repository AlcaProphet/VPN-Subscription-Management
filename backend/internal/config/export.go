// config/export.go：配置导入导出服务（Build3 Step 4）——仅 Production 模式提供（Design1 §3.4.8/6.2）。
// 导出：Argon2id 派生密钥 + AES-256-GCM 加密整个配置文件；导入：事务内严格整体覆盖（先清空再写入）。
package config

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"vpn-sub/internal/store"
)

const (
	ConfirmWordImport = "IMPORT" // 配置导入确认词（固定，二次确认由前端负责）
	FormatVersion     = 1        // 导出格式版本
	MinExportPassword = 8        // 导出密码 ≥8 字符
)

// ExportPayload 导出内容（不含业务数据与日志）
type ExportPayload struct {
	FormatVersion int               `json:"format_version"`
	ExportedAt    time.Time         `json:"exported_at"`
	SourceMode    string            `json:"source_mode"`
	Config        map[string]string `json:"config"`           // 全部系统配置（含签名密钥与全部敏感密文，原样导出）
	SiteName      string            `json:"site_name"`        // 站点名称
	SiteIconB64   string            `json:"site_icon_base64"` // ICON base64 内嵌（可空）
}

// ExportService 配置导入导出服务
type ExportService struct {
	store   *store.Store
	cfg     *Service
	dataDir string
	mode    string // APP_MODE：导入导出仅 Production 提供
	log     *slog.Logger
	// seedPresets Setup 导入分支预置默认组/平台（由 server.New 注入 setup.SeedPresetsTx；避免 config↔setup 循环依赖）
	seedPresets func(ctx context.Context, tx *sql.Tx, frontendURL string) error
}

func NewExportService(st *store.Store, cfg *Service, dataDir, mode string, lg *slog.Logger) *ExportService {
	return &ExportService{store: st, cfg: cfg, dataDir: dataDir, mode: mode, log: lg}
}

// SetSeedPresets 注入 Setup 预置逻辑（Setup 导入分支使用）
func (s *ExportService) SetSeedPresets(fn func(ctx context.Context, tx *sql.Tx, frontendURL string) error) {
	s.seedPresets = fn
}

// Export 导出：导出密码（≥8）→ Argon2id 派生密钥 + AES-256-GCM 加密整个配置文件 → 返回密文供下载。
// 内容含全部系统配置（含签名密钥与敏感密文——密文原样导出，导入侧原样落库）+ 站点信息（ICON base64）
func (s *ExportService) Export(ctx context.Context, password string) ([]byte, error) {
	if s.mode != "prod" {
		return nil, errors.New("配置导出仅 Production 模式提供")
	}
	if utf8.RuneCountInString(password) < MinExportPassword {
		return nil, fmt.Errorf("%w: 导出密码至少 8 字符", ErrBadRequest)
	}
	rows, err := s.store.DB().QueryContext(ctx, `SELECT key, value FROM system_config`)
	if err != nil {
		return nil, fmt.Errorf("读取系统配置失败: %w", err)
	}
	cfgMap := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("解析配置行失败: %w", err)
		}
		cfgMap[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	payload := ExportPayload{
		FormatVersion: FormatVersion,
		ExportedAt:    time.Now(),
		SourceMode:    s.mode,
		Config:        cfgMap,
		SiteName:      mustStr(s.cfg.Get(ctx, "site_name")),
	}
	// ICON base64 内嵌（/public/site/icon.*）
	if iconData, err := readSiteIcon(s.dataDir); err == nil {
		payload.SiteIconB64 = base64.StdEncoding.EncodeToString(iconData)
	} else {
		s.log.Warn("读取站点 ICON 失败（跳过内嵌）", "err", err)
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("序列化导出内容失败: %w", err)
	}
	// Argon2id 派生密钥 + AES-256-GCM 加密（salt 随机，输出格式：salt ‖ nonce ‖ 密文）
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成 salt 失败: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32) // Argon2id：time=1, memory=64MB, threads=4
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}
	out := append(salt, nonce...)
	out = gcm.Seal(out, nonce, plain, nil)
	s.log.Warn("配置导出已执行") // 记 warn 级日志（不记录密码与文件内容）
	return out, nil
}

// Import 导入：上传文件 + 导出密码解密 → 校验格式与版本 → 事务内整体覆盖（严格整体覆盖语义，Design1 §3.4.8）。
// setupMode=true 时（Setup 入口）同事务创建预置默认组与默认平台
func (s *ExportService) Import(ctx context.Context, data []byte, password, confirmWord string, setupMode bool) error {
	if s.mode != "prod" {
		return errors.New("配置导入仅 Production 模式提供")
	}
	if confirmWord != ConfirmWordImport {
		return errors.New("确认词不正确")
	}
	payload, err := s.decrypt(data, password)
	if err != nil {
		return err // 「密码错误或文件损坏」
	}
	// 校验格式与版本：format_version 不匹配仅警告不阻断；未知键忽略并警告；校验失败不做任何变更
	if payload.FormatVersion != FormatVersion {
		s.log.Warn("导入配置 format_version 不匹配", "got", payload.FormatVersion, "want", FormatVersion)
	}
	// 事务内整体覆盖：先清空全部现有配置键再写入导出内容（导出文件中不存在的键一并清除——严格整体覆盖）
	err = s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM system_config`); err != nil {
			return err
		}
		for k, v := range payload.Config {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO system_config (key, value) VALUES (?, ?)`, k, v); err != nil {
				return fmt.Errorf("写入配置键 %s 失败: %w", k, err)
			}
		}
		// 未配置状态导入（Setup 分支）：同事务创建预置默认组与默认平台（预置数据为导入流程固定动作）
		if setupMode {
			if s.seedPresets == nil {
				return errors.New("Setup 预置逻辑未注入")
			}
			if err := s.seedPresets(ctx, tx, ""); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// 事务提交后：ICON 写入 /public/site/（base64 解码落盘；失败记日志不阻断）
	if payload.SiteIconB64 != "" {
		if err := s.writeSiteIcon(payload.SiteIconB64); err != nil {
			s.log.Warn("导入站点 ICON 失败", "err", err)
		}
	}
	// 导入后效果：签名密钥替换 → 全部现有会话立即失效（含执行导入的管理员，前端清凭据跳登录）；
	// 含前端地址/回调地址时需重启生效——UI 提示「导入完成 → 立即重启容器 → 再重新登录」
	s.log.Warn("配置导入已执行", "setup_mode", setupMode)
	return nil
}

// decrypt 解密导入文件（Argon2id + AES-GCM 逆过程）；失败返回「密码错误或文件损坏」
func (s *ExportService) decrypt(data []byte, password string) (*ExportPayload, error) {
	if len(data) < 16+12 {
		return nil, errors.New("密码错误或文件损坏")
	}
	salt, rest := data[:16], data[16:]
	key := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	plain, err := gcm.Open(nil, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], nil)
	if err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	var payload ExportPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, errors.New("密码错误或文件损坏")
	}
	if payload.Config == nil {
		return nil, errors.New("导入文件内容无效")
	}
	return &payload, nil
}

// readSiteIcon 读取站点 ICON 文件（/public/site/icon.*；不存在返回错误）
func readSiteIcon(dataDir string) ([]byte, error) {
	matches, err := filepath.Glob(filepath.Join(dataDir, "public", "site", "icon.*"))
	if err != nil || len(matches) == 0 {
		return nil, errors.New("站点 ICON 不存在")
	}
	return os.ReadFile(matches[0])
}

// writeSiteIcon base64 解码落盘 /public/site/（固定路径覆盖即更新；版本号递增供前端刷新）
func (s *ExportService) writeSiteIcon(b64 string) error {
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("ICON base64 解码失败: %w", err)
	}
	// 扩展名推断：仅按现有文件/默认 png（导入侧不依赖文件名，固定 icon.png 亦可）
	ext := "png"
	if matches, gerr := filepath.Glob(filepath.Join(s.dataDir, "public", "site", "icon.*")); gerr == nil && len(matches) > 0 {
		if e := filepath.Ext(matches[0]); e != "" {
			ext = strings.TrimPrefix(e, ".")
		}
	}
	full := filepath.Join(s.dataDir, "public", "site", "icon."+ext)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(full, data, 0o644); err != nil {
		return fmt.Errorf("写入 ICON 失败: %w", err)
	}
	ver := s.cfg.GetInt(context.Background(), "site_icon_version", 0) + 1
	if err := s.cfg.Set(context.Background(), "site_icon_version", fmt.Sprint(ver)); err != nil {
		return err
	}
	return s.cfg.Set(context.Background(), "site_icon_url", "/public/site/icon."+ext+"?v="+fmt.Sprint(ver))
}
