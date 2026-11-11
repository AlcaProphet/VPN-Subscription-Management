package assembly

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"vpn-sub/internal/config"
	"vpn-sub/internal/store"
)

const storedPlanPrefix = `"` + config.ClashPlanCipherPrefix

func encryptRenderPlan(plan string, key []byte) (string, error) {
	ciphertext, err := config.Encrypt([]byte(plan), key)
	if err != nil {
		return "", fmt.Errorf("加密 Clash 渲染计划失败: %w", err)
	}
	// 保持 render_plan_json 为合法 JSON，密文只含 base64url 字符。
	return storedPlanPrefix + ciphertext + `"`, nil
}

// DecodeRenderPlan 在内部恢复生成时冻结的计划，也兼容升级前的原始 JSON。
func DecodeRenderPlan(ctx context.Context, cfg *config.Service, stored string) (string, error) {
	encoded, encrypted := strings.CutPrefix(stored, storedPlanPrefix)
	if !encrypted {
		return stored, nil
	}
	ciphertext, closed := strings.CutSuffix(encoded, `"`)
	if !closed {
		return "", errors.New("Clash 渲染计划密文格式非法")
	}
	plan, err := cfg.DecryptWithKey(ctx, ciphertext)
	if err != nil {
		return "", fmt.Errorf("解密 Clash 渲染计划失败: %w", err)
	}
	return string(plan), nil
}

// MigrateRenderPlans 在服务开放请求前加密既有 Clash 蓝图。保留历史计划字节和版本关联。
func MigrateRenderPlans(ctx context.Context, st *store.Store, cfg *config.Service) error {
	migrated := false
	err := st.TxImmediate(ctx, func(tx *sql.Tx) error {
		var key []byte
		var lastID int64
		for {
			var id int64
			var plan string
			err := tx.QueryRowContext(ctx, `SELECT id, render_plan_json FROM assembly_blueprints
				WHERE id > ? AND target_syntax = ? AND render_plan_json NOT GLOB ? ORDER BY id LIMIT 1`,
				lastID, ClashYAML, storedPlanPrefix+"*").Scan(&id, &plan)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			if key == nil {
				key, err = cfg.GetSigningKeyTx(ctx, tx)
				if err != nil {
					return err
				}
				// 覆盖当前行的旧明文页，提交后再截断 WAL。
				if _, err := tx.ExecContext(ctx, `PRAGMA secure_delete = ON`); err != nil {
					return err
				}
			}
			sealed, err := encryptRenderPlan(plan, key)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `UPDATE assembly_blueprints SET render_plan_json = ? WHERE id = ?`, sealed, id); err != nil {
				return err
			}
			migrated = true
			lastID = id
		}
	})
	if err != nil {
		return err
	}
	// 重启也完成上次可能因进程中断而未落盘的迁移。
	if !migrated {
		var exists bool
		err := st.DB().QueryRowContext(ctx, `SELECT EXISTS(
			SELECT 1 FROM assembly_blueprints WHERE target_syntax = ?)`, ClashYAML).Scan(&exists)
		if err != nil || !exists {
			return err
		}
	}
	var busy, frames, checkpointed int
	if err := st.DB().QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &frames, &checkpointed); err != nil {
		return fmt.Errorf("截断蓝图迁移 WAL 失败: %w", err)
	}
	if busy != 0 {
		return errors.New("蓝图迁移 WAL 被其他数据库连接占用")
	}
	return nil
}
