package encrypt

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"
)

type AgeEncryptor struct {
	identities []age.Identity
	recipients []age.Recipient
}

func NewAgeEncryptor(identitySource string, identityCommand string, recipientStrs []string) (*AgeEncryptor, error) {
	enc := &AgeEncryptor{}

	if identityCommand != "" {
		identity, err := loadIdentityFromCommand(identityCommand)
		if err != nil {
			return nil, fmt.Errorf("loading identity from command: %w", err)
		}
		enc.identities = append(enc.identities, identity)
	} else if identitySource != "" {
		identities, err := loadIdentity(identitySource)
		if err != nil {
			return nil, fmt.Errorf("loading identity: %w", err)
		}
		enc.identities = identities
	}

	for _, r := range recipientStrs {
		recipient, err := age.ParseX25519Recipient(r)
		if err != nil {
			return nil, fmt.Errorf("parsing recipient %q: %w", r, err)
		}
		enc.recipients = append(enc.recipients, recipient)
	}

	return enc, nil
}

func loadIdentity(source string) ([]age.Identity, error) {
	if strings.HasPrefix(source, "AGE-SECRET-KEY-") {
		identity, err := age.ParseX25519Identity(source)
		if err != nil {
			return nil, err
		}
		return []age.Identity{identity}, nil
	}

	path := expandPath(source)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return age.ParseIdentities(f)
}

func loadIdentityFromCommand(cmd string) (age.Identity, error) {
	out, err := exec.Command("sh", "-c", cmd).Output()
	if err != nil {
		return nil, err
	}

	identityStr := strings.TrimSpace(string(out))
	return age.ParseX25519Identity(identityStr)
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func (e *AgeEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(e.identities) == 0 {
		return nil, fmt.Errorf("no identities configured for decryption")
	}

	var reader io.Reader = bytes.NewReader(ciphertext)

	if bytes.HasPrefix(ciphertext, []byte("-----BEGIN AGE ENCRYPTED FILE-----")) {
		reader = armor.NewReader(reader)
	}

	r, err := age.Decrypt(reader, e.identities...)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return io.ReadAll(r)
}

func (e *AgeEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	if len(e.recipients) == 0 {
		return nil, fmt.Errorf("no recipients configured for encryption")
	}

	var buf bytes.Buffer
	armorWriter := armor.NewWriter(&buf)

	w, err := age.Encrypt(armorWriter, e.recipients...)
	if err != nil {
		return nil, fmt.Errorf("creating encryptor: %w", err)
	}

	if _, err := w.Write(plaintext); err != nil {
		return nil, fmt.Errorf("writing plaintext: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("closing encryptor: %w", err)
	}

	if err := armorWriter.Close(); err != nil {
		return nil, fmt.Errorf("closing armor: %w", err)
	}

	return buf.Bytes(), nil
}

func (e *AgeEncryptor) DecryptFile(path string) ([]byte, error) {
	ciphertext, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return e.Decrypt(ciphertext)
}

func (e *AgeEncryptor) EncryptToFile(plaintext []byte, path string) error {
	ciphertext, err := e.Encrypt(plaintext)
	if err != nil {
		return err
	}
	return os.WriteFile(path, ciphertext, 0600)
}

func (e *AgeEncryptor) CanDecrypt() bool {
	return len(e.identities) > 0
}

func (e *AgeEncryptor) CanEncrypt() bool {
	return len(e.recipients) > 0
}
