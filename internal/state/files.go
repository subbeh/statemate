package state

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

type FileEntry struct {
	ID          int64
	SourcePath  string
	TargetPath  string
	SourceHash  string
	AppliedHash string
	AppliedAt   time.Time
	Mode        os.FileMode
}

func (d *DB) GetFile(targetPath string) (*FileEntry, error) {
	row := d.db.QueryRow(`
		SELECT id, source_path, target_path, source_hash, applied_hash, applied_at, mode
		FROM managed_files WHERE target_path = ?
	`, targetPath)

	var e FileEntry
	var appliedAt string
	err := row.Scan(&e.ID, &e.SourcePath, &e.TargetPath, &e.SourceHash, &e.AppliedHash, &appliedAt, &e.Mode)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	e.AppliedAt, _ = time.Parse(time.RFC3339, appliedAt)
	return &e, nil
}

func (d *DB) SaveFile(e *FileEntry) error {
	_, err := d.db.Exec(`
		INSERT INTO managed_files (source_path, target_path, source_hash, applied_hash, mode)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(target_path) DO UPDATE SET
			source_path = excluded.source_path,
			source_hash = excluded.source_hash,
			applied_hash = excluded.applied_hash,
			applied_at = CURRENT_TIMESTAMP,
			mode = excluded.mode
	`, e.SourcePath, e.TargetPath, e.SourceHash, e.AppliedHash, e.Mode)
	return err
}

func (d *DB) DeleteFile(targetPath string) error {
	_, err := d.db.Exec(`DELETE FROM managed_files WHERE target_path = ?`, targetPath)
	return err
}

func (d *DB) ListFiles() ([]*FileEntry, error) {
	rows, err := d.db.Query(`
		SELECT id, source_path, target_path, source_hash, applied_hash, applied_at, mode
		FROM managed_files ORDER BY target_path
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*FileEntry
	for rows.Next() {
		var e FileEntry
		var appliedAt string
		if err := rows.Scan(&e.ID, &e.SourcePath, &e.TargetPath, &e.SourceHash, &e.AppliedHash, &appliedAt, &e.Mode); err != nil {
			return nil, err
		}
		e.AppliedAt, _ = time.Parse(time.RFC3339, appliedAt)
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hashing file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
