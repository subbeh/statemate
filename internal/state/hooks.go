package state

import "strings"

// SetPendingHook records a triggered hook, with its triggering files, until it
// runs. The files are written before hooks run, so a hook that never got to run
// would otherwise not trigger again.
func (d *DB) SetPendingHook(name string, files []string) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO pending_hooks (name, files) VALUES (?, ?)`,
		name, strings.Join(files, "\n"))
	return err
}

func (d *DB) ClearPendingHook(name string) error {
	_, err := d.db.Exec(`DELETE FROM pending_hooks WHERE name = ?`, name)
	return err
}

// PendingHooks returns the triggering files of each pending hook, by name.
func (d *DB) PendingHooks() (map[string][]string, error) {
	rows, err := d.db.Query(`SELECT name, files FROM pending_hooks`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	pending := make(map[string][]string)
	for rows.Next() {
		var name, files string
		if err := rows.Scan(&name, &files); err != nil {
			return nil, err
		}
		pending[name] = strings.Split(files, "\n")
	}
	return pending, rows.Err()
}
