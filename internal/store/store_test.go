package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/cryptox"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	cipher, err := cryptox.Load(dir, "test-master-key-that-is-not-used-in-production")
	if err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(dir, "test.db"), cipher)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestCredentialsAreEncryptedAndRotated(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestStore(t)
	account, err := s.CreateAccount(ctx, AccountInput{
		Label: "primary", Cookie: "SID=one; SAPISID=abcdefghijklmnop", Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if account.Cookie != "" {
		t.Fatal("list-facing account leaked its cookie")
	}
	var stored string
	if err := s.db.QueryRowContext(ctx, `SELECT cookie_cipher FROM accounts WHERE id = ?`, account.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == "SID=one; SAPISID=abcdefghijklmnop" {
		t.Fatal("cookie was stored as plaintext")
	}

	picked, ok, err := s.PickAccount(ctx)
	if err != nil || !ok {
		t.Fatalf("pick account: ok=%v err=%v", ok, err)
	}
	if picked.Cookie != "SID=one; SAPISID=abcdefghijklmnop" {
		t.Fatalf("unexpected decrypted cookie %q", picked.Cookie)
	}
	if picked.LastUsedAt == 0 {
		t.Fatal("rotation did not advance last_used_at")
	}
}

func TestOverviewUsesOnlyMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestStore(t)
	for _, record := range []RequestRecord{
		{RequestID: "a", Endpoint: "chat.completions", Model: "gemini-3.6-flash", StatusCode: 200, LatencyMS: 100, OutputTokens: 10},
		{RequestID: "b", Endpoint: "chat.completions", Model: "gemini-3.6-flash", StatusCode: 502, LatencyMS: 300, OutputTokens: 0},
		{RequestID: "c", Endpoint: "responses", Model: "gemini-3.6-flash", StatusCode: 200, LatencyMS: 200, OutputTokens: 20},
	} {
		if err := s.RecordRequest(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := s.Overview(ctx, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if stats.Requests != 3 || stats.SuccessRate == nil || *stats.SuccessRate < 66 || *stats.SuccessRate > 67 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if stats.P50LatencyMS == nil || *stats.P50LatencyMS != 100 {
		t.Fatalf("unexpected p50: %+v", stats.P50LatencyMS)
	}
	if stats.OutputTokens != 30 {
		t.Fatalf("unexpected output tokens: %d", stats.OutputTokens)
	}
}

func TestSessionExpiry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.CreateSession(ctx, Session{TokenHash: "live", CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Session(ctx, "live"); err != nil || !ok {
		t.Fatalf("live session missing: ok=%v err=%v", ok, err)
	}
	if err := s.CreateSession(ctx, Session{TokenHash: "expired", CSRFToken: "csrf", ExpiresAt: time.Now().Add(-time.Minute).Unix()}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Session(ctx, "expired"); err != nil || ok {
		t.Fatalf("expired session accepted: ok=%v err=%v", ok, err)
	}
}

func TestDeleteAllSessionsRevokesLiveSessions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newTestStore(t)
	if err := s.CreateSession(ctx, Session{TokenHash: "live", CSRFToken: "csrf", ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteAllSessions(ctx); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Session(ctx, "live"); err != nil || ok {
		t.Fatalf("session remained after revocation: ok=%v err=%v", ok, err)
	}
}
