package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/auth"
	"github.com/yayitinyu/gemini-web2api/internal/cryptox"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

func newAdminHandler(t *testing.T) *Handler {
	t.Helper()
	dir := t.TempDir()
	cipher, err := cryptox.Load(dir, "admin-test-key")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "admin.db"), cipher)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	sessions, err := auth.NewSessionManager(st, "correct-horse-battery-staple", time.Hour, false)
	if err != nil {
		t.Fatal(err)
	}
	keys, _, err := auth.NewAPIKeyManager(context.Background(), st, "")
	if err != nil {
		t.Fatal(err)
	}
	return NewHandler(st, nil, sessions, keys, false)
}

func TestLoginSessionAndCSRFGate(t *testing.T) {
	t.Parallel()
	handler := newAdminHandler(t)
	login := httptest.NewRequest(http.MethodPost, "/api/admin/auth/login", strings.NewReader(`{"password":"correct-horse-battery-staple"}`))
	login.RemoteAddr = "127.0.0.1:1234"
	loginResponse := httptest.NewRecorder()
	handler.Login(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", loginResponse.Code, loginResponse.Body.String())
	}
	cookies := loginResponse.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("missing session cookie: %+v", cookies)
	}

	protected := handler.Protected(http.HandlerFunc(handler.APIKey))
	withoutCSRF := httptest.NewRequest(http.MethodPost, "/api/admin/api-key", strings.NewReader(`{"confirm":"ROTATE"}`))
	withoutCSRF.AddCookie(cookies[0])
	denied := httptest.NewRecorder()
	protected.ServeHTTP(denied, withoutCSRF)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("mutation without CSRF was accepted: %d %s", denied.Code, denied.Body.String())
	}

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/admin/auth/session", nil)
	sessionRequest.AddCookie(cookies[0])
	sessionResponse := httptest.NewRecorder()
	handler.Protected(http.HandlerFunc(handler.Session)).ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), "csrf_token") {
		t.Fatalf("session endpoint failed: %d %s", sessionResponse.Code, sessionResponse.Body.String())
	}
}

func TestLoginRateLimit(t *testing.T) {
	t.Parallel()
	handler := newAdminHandler(t)
	for attempt := 0; attempt < 6; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/admin/auth/login", strings.NewReader(`{"password":"wrong-password"}`))
		request.RemoteAddr = "192.0.2.1:1000"
		response := httptest.NewRecorder()
		handler.Login(response, request)
		if attempt == 5 && response.Code != http.StatusTooManyRequests {
			t.Fatalf("sixth attempt should be rate limited, got %d", response.Code)
		}
	}
}
