package config

import (
	"strings"
	"testing"
)

func validEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("ADMIN_PASSWORD", "correct-horse-battery-staple")
	t.Setenv("COOKIE_SECURE", "")
	t.Setenv("TRUST_PROXY_HEADERS", "")
	t.Setenv("SESSION_TTL", "")
	t.Setenv("MAX_REQUEST_BODY_BYTES", "")
}

func TestLoadRejectsInvalidSecurityBoolean(t *testing.T) {
	validEnvironment(t)
	t.Setenv("COOKIE_SECURE", "tru")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "COOKIE_SECURE") {
		t.Fatalf("expected COOKIE_SECURE parse error, got %v", err)
	}
}

func TestLoadRejectsInvalidDuration(t *testing.T) {
	validEnvironment(t)
	t.Setenv("SESSION_TTL", "tomorrow")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "SESSION_TTL") {
		t.Fatalf("expected SESSION_TTL parse error, got %v", err)
	}
}

func TestLoadAcceptsExplicitSecureProxySettings(t *testing.T) {
	validEnvironment(t)
	t.Setenv("COOKIE_SECURE", "true")
	t.Setenv("TRUST_PROXY_HEADERS", "true")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.CookieSecure || !cfg.TrustProxyHeaders {
		t.Fatalf("secure settings were not applied: %+v", cfg)
	}
}
