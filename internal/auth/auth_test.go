package auth

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/cryptox"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	cipher, err := cryptox.Load(dir, "auth-test-key")
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "auth.db"), cipher)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestAPIKeyPersistsAsHashAndRotates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := testStore(t)
	manager, generated, err := NewAPIKeyManager(ctx, st, "")
	if err != nil {
		t.Fatal(err)
	}
	if generated == "" || !manager.Authenticate(generated) {
		t.Fatal("generated API key was not accepted")
	}
	if manager.Authenticate("wrong") {
		t.Fatal("wrong API key was accepted")
	}
	rotated, err := manager.Rotate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if manager.Authenticate(generated) || !manager.Authenticate(rotated) {
		t.Fatal("rotation did not invalidate the old key")
	}
}

func TestSessionCookieAndCSRF(t *testing.T) {
	t.Parallel()
	st := testStore(t)
	manager, err := NewSessionManager(st, "a-secure-password", time.Hour, true)
	if err != nil {
		t.Fatal(err)
	}
	if !manager.VerifyPassword("a-secure-password") || manager.VerifyPassword("wrong") {
		t.Fatal("password verification mismatch")
	}

	response := httptest.NewRecorder()
	session, err := manager.Create(context.Background(), response)
	if err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || !cookies[0].Secure {
		t.Fatalf("unexpected session cookie: %+v", cookies)
	}
	request := httptest.NewRequest("GET", "/api/admin/session", nil)
	request.AddCookie(cookies[0])
	loaded, ok, err := manager.Authenticate(request)
	if err != nil || !ok || loaded.CSRFToken != session.CSRFToken {
		t.Fatalf("session lookup failed: ok=%v err=%v", ok, err)
	}
	if !manager.VerifyCSRF(loaded, session.CSRFToken) || manager.VerifyCSRF(loaded, "bad") {
		t.Fatal("CSRF verification mismatch")
	}
}
