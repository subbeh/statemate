package util

import (
	"os"
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
