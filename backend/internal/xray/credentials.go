package xray

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

// ErrAdvancedOff 高级模式未开启时中止凭据/同步操作。
var ErrAdvancedOff = errors.New("高级模式未开启")

// ErrIncompleteCredentials 用户凭据只存在一半时视为数据异常。
var ErrIncompleteCredentials = errors.New("用户凭据数据不完整")

// CredentialService 负责用户 UUID/代理密码的生成、加密与读取。
type CredentialService struct {
	store *store.Store
	cfg   *config.Service
}

func NewCredentialService(st *store.Store, cfg *config.Service) *CredentialService {
	return &CredentialService{store: st, cfg: cfg}
}

// EnsureCredentials 在 BEGIN IMMEDIATE 事务内首建用户凭据；两字段同生同灭。
func (s *CredentialService) EnsureCredentials(ctx context.Context, userID int64) error {
	return s.store.TxImmediate(ctx, func(tx *sql.Tx) error {
		advanced, err := s.cfg.GetTx(ctx, tx, config.KeyAdvancedMode)
		if err != nil {
			return err
		}
		if advanced != "true" {
			return ErrAdvancedOff
		}
		signingKey, err := s.cfg.GetSigningKeyTx(ctx, tx)
		if err != nil {
			return err
		}
		uuidVal := uuid.NewString()
		secret := randomSecret()
		uuidEnc, err := config.Encrypt([]byte(uuidVal), signingKey)
		if err != nil {
			return fmt.Errorf("加密 UUID 失败: %w", err)
		}
		secretEnc, err := config.Encrypt([]byte(secret), signingKey)
		if err != nil {
			return fmt.Errorf("加密代理密码失败: %w", err)
		}
		res, err := tx.ExecContext(ctx,
			`UPDATE users SET uuid_encrypted = ?, proxy_secret_encrypted = ? WHERE id = ? AND uuid_encrypted IS NULL AND proxy_secret_encrypted IS NULL`,
			uuidEnc, secretEnc, userID)
		if err != nil {
			return fmt.Errorf("写入用户凭据失败: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 1 {
			return nil
		}
		// 并发已建或历史数据：读回校验完整性。
		var ue, se sql.NullString
		if err := tx.QueryRowContext(ctx,
			`SELECT uuid_encrypted, proxy_secret_encrypted FROM users WHERE id = ?`, userID).Scan(&ue, &se); err != nil {
			return err
		}
		if !ue.Valid || !se.Valid || ue.String == "" || se.String == "" {
			return ErrIncompleteCredentials
		}
		return nil
	})
}

// Credentials 解密返回用户 UUID 与代理密码。
func (s *CredentialService) Credentials(ctx context.Context, userID int64) (string, string, error) {
	var ue, se string
	err := s.store.DB().QueryRowContext(ctx,
		`SELECT COALESCE(uuid_encrypted,''), COALESCE(proxy_secret_encrypted,'') FROM users WHERE id = ?`, userID).
		Scan(&ue, &se)
	if err != nil {
		return "", "", err
	}
	if ue == "" || se == "" {
		return "", "", ErrIncompleteCredentials
	}
	key, err := s.cfg.GetSigningKey(ctx)
	if err != nil {
		return "", "", err
	}
	uuidBytes, err := config.Decrypt(ue, key)
	if err != nil {
		return "", "", fmt.Errorf("解密 UUID 失败: %w", err)
	}
	secretBytes, err := config.Decrypt(se, key)
	if err != nil {
		return "", "", fmt.Errorf("解密代理密码失败: %w", err)
	}
	return string(uuidBytes), string(secretBytes), nil
}

func randomSecret() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic("生成代理密码失败: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
