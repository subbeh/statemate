package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ShortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~" + path[len(home):]
	}
	return path
}

// EncryptedVariant returns path#enc when that file exists, so an encrypted file
// can be named without its suffix, or "" when there is none.
func EncryptedVariant(path string) string {
	if _, err := os.Stat(path + "#enc"); err == nil {
		return path + "#enc"
	}
	return ""
}

// TrimEncryptedSuffix strips a trailing #enc from path and reports whether there
// was one.
func TrimEncryptedSuffix(path string) (string, bool) {
	return strings.CutSuffix(path, "#enc")
}

// AbsPath expands a leading ~ and resolves the path against the current
// directory, returning a cleaned absolute path.
func AbsPath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving home directory: %w", err)
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, path[2:]), nil
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolving path %s: %w", path, err)
	}
	return abs, nil
}
