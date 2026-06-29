package encrypt

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	enc, err := NewAgeEncryptor(identity.String(), "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatalf("NewAgeEncryptor failed: %v", err)
	}

	plaintext := []byte("secret data")

	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if string(ciphertext) == string(plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted data doesn't match: got %q, want %q", decrypted, plaintext)
	}
}

func TestLoadIdentityFromFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.txt")

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(keyFile, []byte(identity.String()+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	enc, err := NewAgeEncryptor(keyFile, "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatalf("NewAgeEncryptor failed: %v", err)
	}

	if !enc.CanDecrypt() {
		t.Error("expected CanDecrypt to be true")
	}
	if !enc.CanEncrypt() {
		t.Error("expected CanEncrypt to be true")
	}

	plaintext := []byte("test data")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestLoadIdentityInline(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	enc, err := NewAgeEncryptor(identity.String(), "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatalf("NewAgeEncryptor with inline identity failed: %v", err)
	}

	if !enc.CanDecrypt() {
		t.Error("expected CanDecrypt to be true with inline identity")
	}
}

func TestLoadIdentityFromCommand(t *testing.T) {
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	enc, err := NewAgeEncryptor("", "echo "+identity.String(), []string{identity.Recipient().String()})
	if err != nil {
		t.Fatalf("NewAgeEncryptor with command failed: %v", err)
	}

	if !enc.CanDecrypt() {
		t.Error("expected CanDecrypt to be true with command identity")
	}

	plaintext := []byte("secret from command")
	ciphertext, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := enc.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip with command identity failed")
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	dir := t.TempDir()
	encryptedFile := filepath.Join(dir, "secret.age")

	identity, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}

	enc, err := NewAgeEncryptor(identity.String(), "", []string{identity.Recipient().String()})
	if err != nil {
		t.Fatalf("NewAgeEncryptor failed: %v", err)
	}

	plaintext := []byte("file content")

	if err := enc.EncryptToFile(plaintext, encryptedFile); err != nil {
		t.Fatalf("EncryptToFile failed: %v", err)
	}

	info, err := os.Stat(encryptedFile)
	if err != nil {
		t.Fatalf("encrypted file not created: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}

	decrypted, err := enc.DecryptFile(encryptedFile)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("file round-trip failed")
	}
}

func TestNoIdentityDecryptFails(t *testing.T) {
	enc := &AgeEncryptor{}

	_, err := enc.Decrypt([]byte("dummy"))
	if err == nil {
		t.Error("expected error when decrypting without identity")
	}
}

func TestNoRecipientsEncryptFails(t *testing.T) {
	identity, _ := age.GenerateX25519Identity()
	enc, _ := NewAgeEncryptor(identity.String(), "", nil)

	_, err := enc.Encrypt([]byte("test"))
	if err == nil {
		t.Error("expected error when encrypting without recipients")
	}
}
