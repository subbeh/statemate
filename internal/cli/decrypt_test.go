package cli

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
	"github.com/subbeh/statemate/internal/encrypt"
)

func testEncryptor(t *testing.T) *encrypt.AgeEncryptor {
	t.Helper()
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt.NewAgeEncryptor(identity.String(), "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

// #enc need not be the last attribute. Trimming it only as a suffix left
// the path unchanged for x#enc#tmpl, so the plaintext was written over
// the ciphertext and then the file was deleted.
func TestDecryptFileAt_EncryptedNotLastAttribute(t *testing.T) {
	enc := testEncryptor(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config#enc#tmpl")

	ciphertext, err := enc.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, ciphertext, 0600); err != nil {
		t.Fatal(err)
	}

	if err := decryptFileAt(path, 0600, enc); err != nil {
		t.Fatalf("decryptFileAt: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "config#tmpl"))
	if err != nil {
		t.Fatalf("decrypted file missing: %v", err)
	}
	if string(got) != "secret" {
		t.Errorf("decrypted content = %q, want %q", got, "secret")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("encrypted file should have been removed, stat err = %v", err)
	}
}

// A file without the #enc attribute has no other name to decrypt to, and
// must never be deleted.
func TestDecryptFileAt_KeepsFileWithoutEncryptedAttribute(t *testing.T) {
	enc := testEncryptor(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config#tmpl")

	ciphertext, err := enc.Encrypt([]byte("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, ciphertext, 0600); err != nil {
		t.Fatal(err)
	}

	if err := decryptFileAt(path, 0600, enc); err == nil {
		t.Fatal("expected an error for a file without the #enc attribute")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file was removed: %v", err)
	}
	if string(got) != string(ciphertext) {
		t.Error("file content was changed")
	}
}

func TestHasEncryptedAttr(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/repo/.matedata/secrets.yaml#enc", true},
		{"/repo/nvim/config#enc#tmpl", true},
		{"/repo/nvim/config#tmpl", false},
		{"/repo/nvim/config#enc-backup", false},
		{"/repo/dir#enc/config", false},
	}
	for _, tc := range tests {
		if got := hasEncryptedAttr(tc.path); got != tc.want {
			t.Errorf("hasEncryptedAttr(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}
