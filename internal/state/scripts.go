package state

import (
	"database/sql"
	"time"
)

type ScriptRun struct {
	ID          int64
	ScriptPath  string
	ContentHash string
	RunAt       time.Time
}

func (d *DB) GetScriptRun(scriptPath string) (*ScriptRun, error) {
	row := d.db.QueryRow(`
		SELECT id, script_path, content_hash, run_at
		FROM script_runs WHERE script_path = ?
		ORDER BY run_at DESC LIMIT 1
	`, scriptPath)

	var r ScriptRun
	var runAt string
	err := row.Scan(&r.ID, &r.ScriptPath, &r.ContentHash, &runAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// SQLite's CURRENT_TIMESTAMP is UTC; callers display it, so hand back local time.
	r.RunAt, _ = time.Parse(time.RFC3339, runAt)
	r.RunAt = r.RunAt.Local()
	return &r, nil
}

func (d *DB) RecordScriptRun(scriptPath, contentHash string) error {
	_, err := d.db.Exec(`
		INSERT INTO script_runs (script_path, content_hash)
		VALUES (?, ?)
	`, scriptPath, contentHash)
	if err != nil {
		return err
	}
	_, err = d.db.Exec(`DELETE FROM script_failures WHERE script_path = ?`, scriptPath)
	return err
}

// RecordScriptFailure marks a script as failed until its next recorded run.
func (d *DB) RecordScriptFailure(scriptPath string) error {
	_, err := d.db.Exec(`INSERT OR IGNORE INTO script_failures (script_path) VALUES (?)`, scriptPath)
	return err
}

func (d *DB) HasScriptFailed(scriptPath string) (bool, error) {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM script_failures WHERE script_path = ?`, scriptPath).Scan(&n)
	return n > 0, err
}

func (d *DB) HasScriptRun(scriptPath string) (bool, error) {
	run, err := d.GetScriptRun(scriptPath)
	if err != nil {
		return false, err
	}
	return run != nil, nil
}

func (d *DB) HasScriptRunWithHash(scriptPath, contentHash string) (bool, error) {
	run, err := d.GetScriptRun(scriptPath)
	if err != nil {
		return false, err
	}
	return run != nil && run.ContentHash == contentHash, nil
}
