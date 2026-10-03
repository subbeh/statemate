package secrets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/subbeh/statemate/internal/encrypt"
)

// ErrNoIdentity is returned when the secrets cache is used without an age
// identity to encrypt it with. The cache holds every secret the templates
// reference, so it is never written in the clear.
var ErrNoIdentity = errors.New("the secrets cache is encrypted with your age identity, but none is configured (set age.identity or age.identity_command)")

// errUnencryptedCache marks a cache written in the clear by an older mate. Its
// contents are never parsed or echoed; the next fetch replaces it.
var errUnencryptedCache = errors.New("secrets cache is not encrypted (written by an older mate); run 'mate secrets fetch' to replace it")

type CacheKey struct {
	Provider string `json:"provider"`
	Item     string `json:"item"`
	Type     string `json:"type"`
	Field    string `json:"field"`
}

func (k CacheKey) String() string {
	return fmt.Sprintf("%s:%s:%s:%s", k.Provider, k.Item, k.Type, k.Field)
}

type CachedValue struct {
	Value     string    `json:"value"`
	FetchedAt time.Time `json:"fetched_at"`
}

type Cache struct {
	FetchedAt time.Time                `json:"fetched_at"`
	Items     map[string]*CachedValue  `json:"items"`
}

type Provider interface {
	Name() string
	Available() error
	Fetch(items []FetchItem) (map[string]string, error)
}

type FetchItem struct {
	Key      CacheKey
	Item     string
	Type     string
	Field    string
	Filename string
}

type ProgressFunc func(key CacheKey, changed bool)

type Manager struct {
	providers  map[string]Provider
	enc        *encrypt.AgeEncryptor
	cache      *Cache
	cachePath  string
	onProgress ProgressFunc
}

// NewManager creates a manager whose cache is encrypted to, and decrypted with,
// the identities enc was configured with -- whether they came from age.identity
// or age.identity_command. enc may be nil when no age block is configured; the
// manager then refuses to read or write the cache.
func NewManager(enc *encrypt.AgeEncryptor, cachePath string) (*Manager, error) {
	m := &Manager{
		providers: make(map[string]Provider),
		enc:       enc,
	}

	if cachePath != "" {
		m.cachePath = expandPath(cachePath)
	} else {
		stateDir, err := defaultCacheDir()
		if err != nil {
			return nil, err
		}
		m.cachePath = filepath.Join(stateDir, "secrets.age")
	}

	m.providers["bitwarden"] = NewBitwardenProvider()

	return m, nil
}

func (m *Manager) SetProgress(fn ProgressFunc) {
	m.onProgress = fn
}

func (m *Manager) Fetch(items []FetchItem) (*FetchResult, error) {
	result := &FetchResult{}

	// Check before contacting any provider: without an identity the values
	// could not be stored, so fetching them would only unlock the vault for
	// nothing.
	if !m.canUseCache() {
		return nil, ErrNoIdentity
	}

	if err := m.loadCache(); err != nil {
		if errors.Is(err, errUnencryptedCache) {
			fmt.Fprintf(os.Stderr, "Warning: replacing unencrypted secrets cache %s with an encrypted one\n", m.cachePath)
		}
		m.cache = &Cache{Items: make(map[string]*CachedValue)}
	}

	// Group items by provider
	byProvider := make(map[string][]FetchItem)
	for _, item := range items {
		byProvider[item.Key.Provider] = append(byProvider[item.Key.Provider], item)
	}

	for providerName, provItems := range byProvider {
		provider, ok := m.providers[providerName]
		if !ok {
			return nil, fmt.Errorf("unknown provider: %s", providerName)
		}

		if err := provider.Available(); err != nil {
			return nil, fmt.Errorf("provider %s not available: %w", providerName, err)
		}

		values, err := provider.Fetch(provItems)
		if err != nil {
			return nil, fmt.Errorf("fetching from %s: %w", providerName, err)
		}

		now := time.Now()
		for keyStr, value := range values {
			old, exists := m.cache.Items[keyStr]
			changed := !exists || old.Value != value
			if changed {
				result.Changed++
			} else {
				result.Unchanged++
			}
			m.cache.Items[keyStr] = &CachedValue{
				Value:     value,
				FetchedAt: now,
			}
			result.Total++

			if m.onProgress != nil {
				// Parse key back for progress reporting
				key := parseCacheKeyString(keyStr)
				m.onProgress(key, changed)
			}
		}
	}

	m.cache.FetchedAt = time.Now()
	if err := m.saveCache(); err != nil {
		return nil, fmt.Errorf("saving cache: %w", err)
	}

	result.Unchanged = result.Total - result.Changed
	return result, nil
}

func (m *Manager) Get(key CacheKey) (string, error) {
	if err := m.loadCache(); err != nil {
		return "", fmt.Errorf("secrets cache not available: %w", err)
	}

	cached, ok := m.cache.Items[key.String()]
	if !ok {
		return "", fmt.Errorf("secret not cached: %s (run 'mate secrets fetch')", key.String())
	}
	return cached.Value, nil
}

func (m *Manager) ListCached() map[string]*CachedValue {
	_ = m.loadCache()
	if m.cache == nil {
		return nil
	}
	return m.cache.Items
}

func (m *Manager) CachePath() string {
	return m.cachePath
}

func (m *Manager) canUseCache() bool {
	return m.enc != nil && m.enc.CanDecrypt()
}

func (m *Manager) loadCache() error {
	if m.cache != nil {
		return nil
	}

	if !m.canUseCache() {
		return ErrNoIdentity
	}

	data, err := os.ReadFile(m.cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no secrets cache found")
		}
		return err
	}

	// Older versions wrote the cache as plain JSON when no age.identity was
	// set. Handing that to age fails with an error quoting the first line of
	// the file -- the secrets themselves -- so recognise it up front and never
	// let its contents reach an error message.
	if !isAgeFile(data) {
		return errUnencryptedCache
	}

	plaintext, err := m.enc.Decrypt(data)
	if err != nil {
		return fmt.Errorf("decrypting secrets cache: %w", err)
	}

	cache := &Cache{}
	if err := json.Unmarshal(plaintext, cache); err != nil {
		return fmt.Errorf("parsing secrets cache: %w", err)
	}
	m.cache = cache
	return nil
}

// isAgeFile reports whether data is an age file, binary or armored.
func isAgeFile(data []byte) bool {
	return bytes.HasPrefix(data, []byte("age-encryption.org/")) ||
		bytes.HasPrefix(data, []byte("-----BEGIN AGE ENCRYPTED FILE-----"))
}

func (m *Manager) saveCache() error {
	if err := os.MkdirAll(filepath.Dir(m.cachePath), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(m.cache, "", "  ")
	if err != nil {
		return err
	}

	ciphertext, err := m.enc.EncryptToIdentity(data)
	if err != nil {
		return fmt.Errorf("encrypting secrets cache: %w", err)
	}
	return os.WriteFile(m.cachePath, ciphertext, 0600)
}

type FetchResult struct {
	Total     int
	Changed   int
	Unchanged int
}

func parseCacheKeyString(s string) CacheKey {
	parts := strings.SplitN(s, ":", 4)
	if len(parts) != 4 {
		return CacheKey{}
	}
	return CacheKey{
		Provider: parts[0],
		Item:     parts[1],
		Type:     parts[2],
		Field:    parts[3],
	}
}

func defaultCacheDir() (string, error) {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "statemate"), nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
