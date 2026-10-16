package util

import (
	"os"
	"path/filepath"
	"testing"
)

// An encrypted file may be named without its #enc suffix; the old #encrypted
// form is not looked for.
func TestEncryptedVariant(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"long#encrypted", "short#enc"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for base, want := range map[string]string{"long": "", "short": "short#enc", "none": ""} {
		got := EncryptedVariant(filepath.Join(dir, base))
		if want != "" {
			want = filepath.Join(dir, want)
		}
		if got != want {
			t.Errorf("EncryptedVariant(%s) = %q, want %q", base, got, want)
		}
	}

	for path, want := range map[string]string{"a.yaml#encrypted": "a.yaml#encrypted", "a.yaml#enc": "a.yaml", "a.yaml": "a.yaml"} {
		if got, _ := TrimEncryptedSuffix(path); got != want {
			t.Errorf("TrimEncryptedSuffix(%q) = %q, want %q", path, got, want)
		}
	}
}
