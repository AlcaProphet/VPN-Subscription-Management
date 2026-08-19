// Package emergency 提供应急恢复模式（Build3 Step 6）：手动/自动触发判定、一次性操作码防护、
// 重置管理员密码与重新初始化（应急全清）——管理员密码救援与系统灾难恢复的最后手段（Design1 §3.8）。
package emergency

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"vpn-sub/internal/auth"
	"vpn-sub/internal/config"
	"vpn-sub/internal/dataclear"
	"vpn-sub/internal/store"
)

// opCodeCharset 操作码字符集：8 位大写字母+数字，去易混淆字符（无 0/O/1/I/L 等）
const opCodeCharset = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// TriggerReason 应急模式触发原因
type TriggerReason string

const (
	TriggerNone       TriggerReason = ""            // 正常模式
	TriggerManual     TriggerReason = "manual"      // 环境变量手动触发
	TriggerDBCorrupt  TriggerReason = "db_corrupt"  // 数据库无法连接/损坏
	TriggerKeyMissing TriggerReason = "key_missing" // 关键配置损坏（configured=true 但签名密钥缺失）
)

// Service 应急服务
type Service struct {
	store      *store.Store
	cfg        *config.Service
	clearSvc   *dataclear.Service
	dataDir    string
	dbFile     string
	log        *slog.Logger
	mu         sync.Mutex
	opCode     string // 一次性操作码（仅存进程内存，不落库）
	reason     TriggerReason
	dbReadable bool // 数据库可读（决定能力分级）
}

// Detect 启动时触发判定（main 装配时调用，先于路由注册）：
// 手动触发（环境变量 RESET_ADMIN_PASSWORD 非空，优先，但仍探测数据库可读性决定能力分级）；
// 自动触发仅两类：数据库无法连接/损坏（含 integrity_check 不通过）、configured=true 但签名密钥缺失
func Detect(ctx context.Context, st *store.Store, cfg *config.Service, lg *slog.Logger) (TriggerReason, bool) {
	if os.Getenv("RESET_ADMIN_PASSWORD") != "" {
		return TriggerManual, probeDBReadable(ctx, st)
	}
	if !probeDBReadable(ctx, st) {
		return TriggerDBCorrupt, false
	}
	configured := cfg.GetBool(ctx, config.KeyConfigured, false)
	signingKey, _ := cfg.Get(ctx, config.KeySigningKey)
	if configured && signingKey == "" {
		return TriggerKeyMissing, true
	}
	return TriggerNone, true
}

// probeDBReadable 探测数据库可读性（连接可用 + PRAGMA integrity_check 通过）
func probeDBReadable(ctx context.Context, st *store.Store) bool {
	var integrity string
	if err := st.DB().QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&integrity); err != nil {
		return false
	}
	return integrity == "ok"
}

// NewService 进入应急模式时构造——生成操作码并输出到运行日志（docker compose logs 可见）
func NewService(reason TriggerReason, dbReadable bool, st *store.Store, cfg *config.Service, clearSvc *dataclear.Service, dataDir, dbFile string, lg *slog.Logger) *Service {
	s := &Service{
		store: st, cfg: cfg, clearSvc: clearSvc, dataDir: dataDir, dbFile: dbFile,
		log: lg, reason: reason, dbReadable: dbReadable,
	}
	s.regenerateOpCode() // 初始生成
	lg.Warn("应急恢复模式已触发", "reason", string(reason))
	return s
}

// Reason 触发原因（status 端点返回）
func (s *Service) Reason() TriggerReason { return s.reason }

// regenerateOpCode 生成 8 位操作码并输出日志；每次提交（无论成败）即消耗失效并重新生成
func (s *Service) regenerateOpCode() {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		s.log.Error("生成应急操作码失败", "err", err)
		return
	}
	for i := range b {
		b[i] = opCodeCharset[int(b[i])%len(opCodeCharset)]
	}
	s.mu.Lock()
	s.opCode = string(b)
	s.mu.Unlock()
	// 输出到运行日志（docker compose logs 可见）；页面不展示任何词
	s.log.Warn("应急操作码已生成", "opcode", s.opCode, "reason", string(s.reason))
}

// VerifyOpCode 校验操作码——严格一次性：每次提交（无论成功或失败）即消耗失效，立即重新生成新码并输出日志
func (s *Service) VerifyOpCode(input string) bool {
	s.mu.Lock()
	current := s.opCode
	s.mu.Unlock()
	ok := subtle.ConstantTimeCompare([]byte(input), []byte(current)) == 1 // 恒定时间比较防时序侧信道
	s.regenerateOpCode()                                                  // 无论成败均消耗并重新生成
	return ok
}

// --- 能力分级（安全收紧，Design1 §3.8）---

// CanResetPassword 重置管理员密码能力——仅环境变量触发且数据库可读时提供；用户表为空时不可用
func (s *Service) CanResetPassword(ctx context.Context) bool {
	if s.reason != TriggerManual || !s.dbReadable {
		return false
	}
	var n int
	if err := s.store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil || n == 0 {
		return false
	}
	return true
}

// AdminOption 管理员账号选项（验码通过后才返回名单）
type AdminOption struct {
	ID          int64  `json:"id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	HasPassword bool   `json:"has_password"` // 纯 OIDC 管理员重置后仍无法本地登录（标注）
}

// ListAdmins 验码通过后才返回管理员名单（不经验码不暴露）；仅设有本地密码的账号有效标注
func (s *Service) ListAdmins(ctx context.Context) ([]AdminOption, error) {
	if !s.dbReadable { // 数据库不可读（Open/迁移失败）时无重置密码能力，拒绝查询防 nil store panic
		return nil, errors.New("数据库不可读，无法列出管理员")
	}
	rows, err := s.store.DB().QueryContext(ctx,
		`SELECT id, username, COALESCE(email,''), password_hash IS NOT NULL
		 FROM users WHERE role = 'admin' AND status = 'active' ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("查询管理员名单失败: %w", err)
	}
	defer rows.Close()
	out := make([]AdminOption, 0)
	for rows.Next() {
		var a AdminOption
		var hp int
		if err := rows.Scan(&a.ID, &a.Username, &a.Email, &hp); err != nil {
			return nil, err
		}
		a.HasPassword = hp == 1
		out = append(out, a)
	}
	return out, rows.Err()
}

// ResetAdminPassword 选账号 → 设新密码（≥8）→ 确认更新；成功后递增 credential_version；
// 接入层成功响应后进程退出（exit），由 compose restart 拉起
func (s *Service) ResetAdminPassword(ctx context.Context, userID int64, newPassword string) error {
	if err := auth.ValidatePassword(newPassword); err != nil {
		return err
	}
	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return err
	}
	res, err := s.store.DB().ExecContext(ctx,
		`UPDATE users SET password_hash = ?, credential_version = credential_version + 1, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND role = 'admin'`, hash, userID)
	if err != nil {
		return fmt.Errorf("重置管理员密码失败: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		return errors.New("管理员账号不存在或不可用")
	}
	s.log.Warn("应急重置管理员密码已执行", "user_id", userID)
	return nil
}

// Reinitialize 重新初始化（应急全清）——数据库可连接时以 SQL 清空（复用 dataclear 清库逻辑）；
// 无法打开/损坏时降级为删除数据库文件 + 版本文件目录 + /public 资源后重建空库；
// 接入层成功响应后进程退出（exit），重启后进入正常 Setup
func (s *Service) Reinitialize(ctx context.Context) error {
	if s.dbReadable {
		// 路径 A：SQL 清空（复用 dataclear 清库逻辑）+ 删数据文件
		if err := s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
			return s.clearSvc.ClearTablesTx(ctx, tx)
		}); err != nil {
			return fmt.Errorf("SQL 清空失败: %w", err)
		}
	} else {
		// 路径 B：数据库无法打开/损坏 → 删除数据库文件 + 版本文件目录 + /public 资源后重建空库
		if err := os.Remove(filepath.Join(s.dataDir, s.dbFile)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("删除数据库文件失败: %w", err)
		}
	}
	for _, dir := range []string{"contents", "public"} {
		if err := os.RemoveAll(filepath.Join(s.dataDir, dir)); err != nil {
			s.log.Error("删除数据目录失败", "dir", dir, "err", err)
		}
	}
	s.log.Warn("应急重新初始化已执行")
	return nil
}
