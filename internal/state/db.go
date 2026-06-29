package state

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DB struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS managed_files (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_path TEXT NOT NULL,
	target_path TEXT NOT NULL UNIQUE,
	source_hash TEXT NOT NULL,
	applied_hash TEXT NOT NULL,
	applied_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	mode INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_target_path ON managed_files(target_path);

CREATE TABLE IF NOT EXISTS script_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	script_path TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	run_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_script_path ON script_runs(script_path);
`

func Open(path string) (*DB, error) {
	if path == "" {
		var err error
		path, err = defaultPath()
		if err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating state directory: %w", err)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &DB{db: db}, nil
}

func (d *DB) Close() error {
	return d.db.Close()
}

func defaultPath() (string, error) {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "statemate", "state.db"), nil
}
