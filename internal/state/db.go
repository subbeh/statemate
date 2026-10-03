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

// ResolveRelativePaths rewrites script and source paths that were recorded
// relative, joining them onto sourceDir.
//
// Until the source dir was made absolute at config load, running mate from the
// repository root (the config found in, or passed as, a relative path) recorded
// paths such as ".matescripts/setup#once". Scripts are looked up by path, so
// without this every once and onchange script would run again after upgrading.
// Such paths were relative to the working directory of that run, which in the
// case that produced them -- mate run from the repository root -- is the source
// dir. (A relative --config pointing elsewhere, such as ../dotfiles/mate.yaml,
// cannot be recovered; those scripts run once more.) Target paths, the key for
// managed files, were always absolute and are not touched.
func (d *DB) ResolveRelativePaths(sourceDir string) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, col := range []struct{ table, column string }{
		{"script_runs", "script_path"},
		{"managed_files", "source_path"},
	} {
		rows, err := tx.Query(fmt.Sprintf(`SELECT DISTINCT %s FROM %s WHERE %s NOT LIKE '/%%'`, col.column, col.table, col.column))
		if err != nil {
			return err
		}
		var paths []string
		for rows.Next() {
			var p string
			if err := rows.Scan(&p); err != nil {
				_ = rows.Close()
				return err
			}
			paths = append(paths, p)
		}
		_ = rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		for _, p := range paths {
			if p == "" || filepath.IsAbs(p) {
				continue
			}
			if _, err := tx.Exec(fmt.Sprintf(`UPDATE %s SET %s = ? WHERE %s = ?`, col.table, col.column, col.column), filepath.Join(sourceDir, p), p); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
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
