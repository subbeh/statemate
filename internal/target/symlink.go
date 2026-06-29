package target

import (
	"os"
	"path/filepath"

	"github.com/subbeh/statemate/internal/source"
)

func createSymlink(entry *source.Entry) error {
	linkTarget, err := os.Readlink(entry.SourcePath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(entry.TargetPath), 0755); err != nil {
		return err
	}

	if _, err := os.Lstat(entry.TargetPath); err == nil {
		if err := os.Remove(entry.TargetPath); err != nil {
			return err
		}
	}

	return os.Symlink(linkTarget, entry.TargetPath)
}
