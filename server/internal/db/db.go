package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (and creates if needed) the SQLite database inside dataDir.
func Open(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	databaseFile := filepath.Join(dataDir, "cracked_saves.db")
	conn, err := sql.Open("sqlite", databaseFile+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// SQLite works best with a single writer; keep connections busy.
	conn.SetMaxOpenConns(1)

	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func migrate(database *sql.DB) error {
	schema := `
CREATE TABLE IF NOT EXISTS games (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL UNIQUE,
	exe_pattern   TEXT NOT NULL DEFAULT '',
	save_dir_hint TEXT NOT NULL DEFAULT '',
	notes         TEXT NOT NULL DEFAULT '',
	created_at    INTEGER NOT NULL,
	updated_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS saves (
	game_id     TEXT NOT NULL REFERENCES games(id) ON DELETE CASCADE,
	filename    TEXT NOT NULL,
	sha256      TEXT NOT NULL,
	size        INTEGER NOT NULL,
	updated_at  INTEGER NOT NULL,
	PRIMARY KEY (game_id, filename)
);
`
	if _, err := database.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}
