package repository

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	_ "modernc.org/sqlite"
)

var DB *sql.DB

// InitDB opens (or creates) the SQLite database at the given path, enables WAL mode,
// creates all tables, and inserts default platforms if they don't exist.
func InitDB(dbPath string) error {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := DB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return err
	}
	// Enable foreign keys
	if _, err := DB.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return err
	}

	// Configure connection pool
	DB.SetMaxOpenConns(1) // SQLite only supports a single writer
	DB.SetMaxIdleConns(1)
	DB.SetConnMaxLifetime(0)

	if err := createTables(); err != nil {
		return err
	}

	if err := insertDefaultPlatforms(); err != nil {
		return err
	}

	// Start background goroutine for periodic cleanup tasks
	go periodicCleanup()

	log.Info().Msg("Database initialized successfully")
	return nil
}

// CloseDB closes the database connection.
func CloseDB() {
	if DB != nil {
		DB.Close()
	}
}

func createTables() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS system_config (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS users (
			user_id     TEXT PRIMARY KEY,
			username    TEXT NOT NULL DEFAULT '',
			email       TEXT NOT NULL DEFAULT '',
			role        TEXT NOT NULL DEFAULT 'user' CHECK(role IN ('admin', 'user')),
			is_advanced INTEGER NOT NULL DEFAULT 0,
			groups      TEXT NOT NULL DEFAULT '[]'
		)`,

		`CREATE TABLE IF NOT EXISTS platforms (
			id             TEXT PRIMARY KEY,
			name           TEXT NOT NULL DEFAULT '',
			description    TEXT NOT NULL DEFAULT '',
			client_schemes TEXT NOT NULL DEFAULT '[]',
			download_url   TEXT NOT NULL DEFAULT ''
		)`,

		`CREATE TABLE IF NOT EXISTS subscriptions (
			id       TEXT PRIMARY KEY,
			name     TEXT NOT NULL DEFAULT '',
			platform TEXT NOT NULL,
			type     TEXT NOT NULL CHECK(type IN ('default', 'advanced')),
			versions TEXT NOT NULL DEFAULT '[]',
			UNIQUE(platform, type)
		)`,

		`CREATE TABLE IF NOT EXISTS rules (
			id          TEXT PRIMARY KEY,
			name        TEXT NOT NULL DEFAULT '',
			client_type TEXT NOT NULL DEFAULT 'shadowrocket',
			versions    TEXT NOT NULL DEFAULT '[]',
			created_at  TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS access_logs (
			id                   INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id              TEXT NOT NULL DEFAULT '',
			ip                   TEXT NOT NULL DEFAULT '',
			download_type        TEXT NOT NULL DEFAULT '' CHECK(download_type IN ('subscription', 'share', 'custom', 'rule')),
			platform             TEXT NOT NULL DEFAULT '',
			share_subscription_id TEXT NOT NULL DEFAULT '',
			rule_id              TEXT NOT NULL DEFAULT '',
			status               TEXT NOT NULL DEFAULT 'success' CHECK(status IN ('success', 'failed')),
			error_reason         TEXT NOT NULL DEFAULT '',
			created_at           TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS oidc_state (
			state         TEXT PRIMARY KEY,
			code_verifier TEXT NOT NULL DEFAULT '',
			nonce         TEXT NOT NULL DEFAULT '',
			created_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS download_tokens (
			token         TEXT PRIMARY KEY,
			user_id       TEXT NOT NULL,
			platform      TEXT NOT NULL,
			type          TEXT CHECK(type IS NULL OR type IN ('default', 'advanced')),
			custom_sub_id TEXT,
			created_at    TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS custom_subscriptions (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			platform   TEXT NOT NULL,
			versions   TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(user_id, platform)
		)`,

		`CREATE TABLE IF NOT EXISTS share_subscriptions (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL DEFAULT '',
			versions   TEXT NOT NULL DEFAULT '[]',
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,

		`CREATE TABLE IF NOT EXISTS share_tokens (
			token                 TEXT PRIMARY KEY,
			share_subscription_id TEXT NOT NULL,
			created_at            TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (share_subscription_id) REFERENCES share_subscriptions(id) ON DELETE CASCADE
		)`,

		`CREATE TABLE IF NOT EXISTS rule_tokens (
			token      TEXT PRIMARY KEY,
			rule_id    TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE
		)`,
	}

	for _, ddl := range tables {
		if _, err := DB.Exec(ddl); err != nil {
			return err
		}
	}

	// Partial unique indexes for download_tokens — SQLite treats NULLs as distinct
	// in UNIQUE constraints, so we use two partial indexes to enforce uniqueness:
	// one for regular tokens (custom_sub_id IS NULL, unique on user+platform+type)
	// and one for custom tokens (custom_sub_id IS NOT NULL, unique on user+platform+custom_sub_id).
	indexes := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_download_tokens_regular
		 ON download_tokens(user_id, platform, type) WHERE custom_sub_id IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_download_tokens_custom
		 ON download_tokens(user_id, platform, custom_sub_id) WHERE custom_sub_id IS NOT NULL`,
	}
	for _, idx := range indexes {
		if _, err := DB.Exec(idx); err != nil {
			return err
		}
	}

	return nil
}

func insertDefaultPlatforms() error {
	defaults := []struct {
		ID            string
		Name          string
		Description   string
		ClientSchemes string
	}{
		{
			ID:            "clash-verge",
			Name:          "Clash Verge",
			Description:   "Clash Verge 客户端",
			ClientSchemes: `["clash://install-config?url="]`,
		},
		{
			ID:            "v2rayng",
			Name:          "v2rayNG",
			Description:   "v2rayNG 客户端",
			ClientSchemes: `["v2rayng://install-config?url="]`,
		},
		{
			ID:            "shadowrocket",
			Name:          "Shadowrocket",
			Description:   "Shadowrocket 客户端",
			ClientSchemes: `["shadowrocket://install-config?url="]`,
		},
	}

	for _, p := range defaults {
		_, err := DB.Exec(
			`INSERT OR IGNORE INTO platforms (id, name, description, client_schemes) VALUES (?, ?, ?, ?)`,
			p.ID, p.Name, p.Description, p.ClientSchemes,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func periodicCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		// Clean expired OIDC state records (older than 10 minutes)
		_, _ = DB.Exec(`DELETE FROM oidc_state WHERE created_at < datetime('now', '-10 minutes')`)
		// Clean access logs older than 90 days
		_, _ = DB.Exec(`DELETE FROM access_logs WHERE created_at < datetime('now', '-90 days')`)
	}
}
