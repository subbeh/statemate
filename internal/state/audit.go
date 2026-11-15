package state

import (
	"encoding/json"
	"time"
)

// AuditRun is one mate run that changed something, as `mate audit` lists it.
type AuditRun struct {
	Time    time.Time
	Args    []string
	Changes []AuditChange
}

// AuditChange is one info-or-higher log record of a run. Attrs keep the
// record's order, as [key, value] pairs.
type AuditChange struct {
	Level string      `json:"level"`
	Msg   string      `json:"msg"`
	Attrs [][2]string `json:"attrs,omitempty"`
}

// RecordAuditRun stores run, then drops all but the newest keep runs.
func (d *DB) RecordAuditRun(run AuditRun, keep int) error {
	args, err := json.Marshal(run.Args)
	if err != nil {
		return err
	}
	changes, err := json.Marshal(run.Changes)
	if err != nil {
		return err
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`INSERT INTO audit_runs (run_at, args, changes) VALUES (?, ?, ?)`,
		run.Time.UTC(), string(args), string(changes)); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM audit_runs WHERE id NOT IN (SELECT id FROM audit_runs ORDER BY id DESC LIMIT ?)`, keep); err != nil {
		return err
	}
	return tx.Commit()
}

// AuditRuns returns the newest n runs, newest first.
func (d *DB) AuditRuns(n int) ([]AuditRun, error) {
	rows, err := d.db.Query(`SELECT run_at, args, changes FROM audit_runs ORDER BY id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var runs []AuditRun
	for rows.Next() {
		var run AuditRun
		var args, changes string
		if err := rows.Scan(&run.Time, &args, &changes); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(args), &run.Args); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(changes), &run.Changes); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}
