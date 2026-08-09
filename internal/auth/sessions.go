package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/yayitinyu/gemini-web2api/internal/store"
)

const SessionCookieName = "gw_admin_session"

type SessionManager struct {
	store        *store.Store
	passwordHash []byte
	ttl          time.Duration
	secure       bool
}

func NewSessionManager(st *store.Store, password string, ttl time.Duration, secure bool) (*SessionManager, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}
	return &SessionManager{store: st, passwordHash: passwordHash, ttl: ttl, secure: secure}, nil
}

func (m *SessionManager) VerifyPassword(password string) bool {
	return bcrypt.CompareHashAndPassword(m.passwordHash, []byte(password)) == nil
}

func (m *SessionManager) Create(ctx context.Context, w http.ResponseWriter) (store.Session, error) {
	token, err := randomToken("", 32)
	if err != nil {
		return store.Session{}, err
	}
	csrf, err := randomToken("csrf_", 24)
	if err != nil {
		return store.Session{}, err
	}
	expiresAt := time.Now().Add(m.ttl)
	session := store.Session{TokenHash: tokenHash(token), CSRFToken: csrf, ExpiresAt: expiresAt.Unix()}
	if err := m.store.CreateSession(ctx, session); err != nil {
		return store.Session{}, err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expiresAt,
		MaxAge:   int(m.ttl.Seconds()),
	})
	return session, nil
}

func (m *SessionManager) Authenticate(r *http.Request) (store.Session, bool, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if errors.Is(err, http.ErrNoCookie) {
		return store.Session{}, false, nil
	}
	if err != nil || cookie.Value == "" {
		return store.Session{}, false, err
	}
	return m.store.Session(r.Context(), tokenHash(cookie.Value))
}

func (m *SessionManager) VerifyCSRF(session store.Session, value string) bool {
	if session.CSRFToken == "" || len(value) != len(session.CSRFToken) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(session.CSRFToken)) == 1
}

func (m *SessionManager) Delete(r *http.Request, w http.ResponseWriter) error {
	if cookie, err := r.Cookie(SessionCookieName); err == nil && cookie.Value != "" {
		if err := m.store.DeleteSession(r.Context(), tokenHash(cookie.Value)); err != nil {
			return err
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Unix(1, 0),
		MaxAge:   -1,
	})
	return nil
}

func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
