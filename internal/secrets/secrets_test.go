package secrets

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/subbeh/statemate/internal/encrypt"
)

const testSecret = "hunter2-very-secret"

var testKey = CacheKey{Provider: "bitwarden", Item: "github", Type: "field", Field: "token"}

type fakeProvider struct {
	calls int
}

func (f *fakeProvider) Name() string     { return "bitwarden" }
func (f *fakeProvider) Available() error { return nil }
func (f *fakeProvider) Fetch(items []FetchItem) (map[string]string, error) {
	f.calls++
	out := make(map[string]string)
	for _, it := range items {
		out[it.Key.String()] = testSecret
	}
	return out, nil
}

func newTestManager(t *testing.T, enc *encrypt.AgeEncryptor, cachePath string) (*Manager, *fakeProvider) {
	t.Helper()
	m, err := NewManager(enc, cachePath)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	fp := &fakeProvider{}
	m.providers["bitwarden"] = fp
	return m, fp
}

// identityCommandEncryptor configures the identity the way an `identity_command`
// (a password manager, a keychain lookup) does, with no `identity` set at all.
func identityCommandEncryptor(t *testing.T) *encrypt.AgeEncryptor {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	enc, err := encrypt.NewAgeEncryptor("", "echo "+id.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

func assertNoSecret(t *testing.T, what, s string) {
	t.Helper()
	if strings.Contains(s, testSecret) {
		t.Fatalf("%s leaks the secret value: %q", what, s)
	}
}

func TestFetch_EncryptsCacheWithIdentityCommand(t *testing.T) {
	enc := identityCommandEncryptor(t)
	cachePath := filepath.Join(t.TempDir(), "secrets.age")

	m, _ := newTestManager(t, enc, cachePath)
	if _, err := m.Fetch([]FetchItem{{Key: testKey}}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "cache file", string(data))

	// A fresh manager (a later mate run) reads it back.
	m2, _ := newTestManager(t, enc, cachePath)
	got, err := m2.Get(testKey)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != testSecret {
		t.Errorf("Get = %q, want %q", got, testSecret)
	}
}

func TestFetch_RefusesWithoutIdentity(t *testing.T) {
	recipientOnly, err := encrypt.NewAgeEncryptor("", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	for name, enc := range map[string]*encrypt.AgeEncryptor{
		"no age block":    nil,
		"no identity set": recipientOnly,
	} {
		t.Run(name, func(t *testing.T) {
			cachePath := filepath.Join(t.TempDir(), "secrets.age")
			m, fp := newTestManager(t, enc, cachePath)

			_, err := m.Fetch([]FetchItem{{Key: testKey}})
			if !errors.Is(err, ErrNoIdentity) {
				t.Fatalf("Fetch error = %v, want ErrNoIdentity", err)
			}
			// Nothing is fetched from the vault only to be thrown away.
			if fp.calls != 0 {
				t.Errorf("provider called %d times", fp.calls)
			}
			if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
				t.Errorf("cache file written without an identity (stat err %v)", err)
			}
		})
	}
}

// writePlaintextCache writes a cache the way mate did before the cache was always
// encrypted.
func writePlaintextCache(t *testing.T, path string) {
	t.Helper()
	data, err := json.Marshal(&Cache{Items: map[string]*CachedValue{
		testKey.String(): {Value: testSecret},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestGet_PlaintextCacheIsNotReadAndNotEchoed(t *testing.T) {
	enc := identityCommandEncryptor(t)
	cachePath := filepath.Join(t.TempDir(), "secrets.age")
	writePlaintextCache(t, cachePath)

	m, _ := newTestManager(t, enc, cachePath)
	_, err := m.Get(testKey)
	if err == nil {
		t.Fatal("Get succeeded on an unencrypted cache")
	}
	assertNoSecret(t, "error", err.Error())

	if cached := m.ListCached(); cached != nil {
		t.Errorf("ListCached returned %d entries from an unencrypted cache", len(cached))
	}
}

func TestFetch_ReplacesPlaintextCacheWithEncrypted(t *testing.T) {
	enc := identityCommandEncryptor(t)
	cachePath := filepath.Join(t.TempDir(), "secrets.age")
	writePlaintextCache(t, cachePath)

	m, _ := newTestManager(t, enc, cachePath)
	if _, err := m.Fetch([]FetchItem{{Key: testKey}}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	assertNoSecret(t, "cache file", string(data))
}

func TestGet_WithoutIdentityDoesNotReadPlaintext(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "secrets.age")
	writePlaintextCache(t, cachePath)

	m, _ := newTestManager(t, nil, cachePath)
	_, err := m.Get(testKey)
	if !errors.Is(err, ErrNoIdentity) {
		t.Fatalf("Get error = %v, want ErrNoIdentity", err)
	}
	assertNoSecret(t, "error", err.Error())
}
