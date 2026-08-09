package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"sync"

	"github.com/yayitinyu/gemini-web2api/internal/store"
)

const (
	apiKeyHashSetting = "api_key_hash"
	apiKeyHintSetting = "api_key_hint"
)

type APIKeyManager struct {
	store    *store.Store
	external bool
	mu       sync.RWMutex
	hash     [sha256.Size]byte
	hint     string
}

type APIKeyState struct {
	Hint     string `json:"hint"`
	External bool   `json:"external"`
}

func NewAPIKeyManager(ctx context.Context, st *store.Store, configured string) (*APIKeyManager, string, error) {
	manager := &APIKeyManager{store: st}
	configured = strings.TrimSpace(configured)
	if configured != "" {
		if len(configured) < 16 {
			return nil, "", errors.New("API_KEY must contain at least 16 characters")
		}
		manager.external = true
		manager.hash = sha256.Sum256([]byte(configured))
		manager.hint = keyHint(configured)
		return manager, "", nil
	}

	storedHash, ok, err := st.GetSetting(ctx, apiKeyHashSetting)
	if err != nil {
		return nil, "", err
	}
	if ok {
		decoded, err := hex.DecodeString(storedHash)
		if err != nil || len(decoded) != sha256.Size {
			return nil, "", errors.New("stored API key hash is invalid")
		}
		copy(manager.hash[:], decoded)
		manager.hint, _, err = st.GetSetting(ctx, apiKeyHintSetting)
		return manager, "", err
	}

	plain, err := manager.rotate(ctx)
	if err != nil {
		return nil, "", err
	}
	return manager, plain, nil
}

func (m *APIKeyManager) Authenticate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	candidate := sha256.Sum256([]byte(value))
	m.mu.RLock()
	defer m.mu.RUnlock()
	return subtle.ConstantTimeCompare(candidate[:], m.hash[:]) == 1
}

func (m *APIKeyManager) State() APIKeyState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return APIKeyState{Hint: m.hint, External: m.external}
}

func (m *APIKeyManager) Rotate(ctx context.Context) (string, error) {
	if m.external {
		return "", errors.New("API key is managed by the API_KEY environment variable")
	}
	return m.rotate(ctx)
}

func (m *APIKeyManager) rotate(ctx context.Context) (string, error) {
	plain, err := randomToken("sk-gw_", 32)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(plain))
	hint := keyHint(plain)
	if err := m.store.SetSetting(ctx, apiKeyHashSetting, hex.EncodeToString(hash[:])); err != nil {
		return "", err
	}
	if err := m.store.SetSetting(ctx, apiKeyHintSetting, hint); err != nil {
		return "", err
	}
	m.mu.Lock()
	m.hash = hash
	m.hint = hint
	m.mu.Unlock()
	return plain, nil
}

func keyHint(value string) string {
	if len(value) <= 12 {
		return "••••"
	}
	return value[:8] + "••••" + value[len(value)-4:]
}

func randomToken(prefix string, size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(buffer), nil
}
