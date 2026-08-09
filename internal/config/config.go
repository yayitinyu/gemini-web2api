package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const defaultUpstreamBase = "https://gemini.google.com"

type Config struct {
	Address             string
	DataDir             string
	DatabasePath        string
	AdminPassword       string
	APIKey              string
	EncryptionKey       string
	CookieSecure        bool
	TrustProxyHeaders   bool
	SessionTTL          time.Duration
	ReadHeaderTimeout   time.Duration
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	ShutdownTimeout     time.Duration
	UpstreamBaseURL     string
	MaxRequestBodyBytes int64
}

func Load() (Config, error) {
	dataDir := envOr("DATA_DIR", "./data")
	cookieSecure, err := envBool("COOKIE_SECURE", false)
	if err != nil {
		return Config{}, err
	}
	trustProxyHeaders, err := envBool("TRUST_PROXY_HEADERS", false)
	if err != nil {
		return Config{}, err
	}
	sessionTTL, err := envDuration("SESSION_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	maxRequestBodyBytes, err := envInt64("MAX_REQUEST_BODY_BYTES", 16<<20)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Address:             envOr("LISTEN_ADDR", ":8080"),
		DataDir:             dataDir,
		DatabasePath:        filepath.Join(dataDir, "gateway.db"),
		AdminPassword:       os.Getenv("ADMIN_PASSWORD"),
		APIKey:              os.Getenv("API_KEY"),
		EncryptionKey:       os.Getenv("DATA_ENCRYPTION_KEY"),
		CookieSecure:        cookieSecure,
		TrustProxyHeaders:   trustProxyHeaders,
		SessionTTL:          sessionTTL,
		ReadHeaderTimeout:   10 * time.Second,
		ReadTimeout:         30 * time.Second,
		WriteTimeout:        0,
		IdleTimeout:         90 * time.Second,
		ShutdownTimeout:     15 * time.Second,
		UpstreamBaseURL:     strings.TrimRight(envOr("UPSTREAM_BASE_URL", defaultUpstreamBase), "/"),
		MaxRequestBodyBytes: maxRequestBodyBytes,
	}
	if path := strings.TrimSpace(os.Getenv("DATABASE_PATH")); path != "" {
		cfg.DatabasePath = path
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.Address) == "" {
		return errors.New("LISTEN_ADDR cannot be empty")
	}
	if len(c.AdminPassword) < 12 {
		return errors.New("ADMIN_PASSWORD must contain at least 12 characters")
	}
	if c.SessionTTL < 5*time.Minute || c.SessionTTL > 30*24*time.Hour {
		return errors.New("SESSION_TTL must be between 5m and 720h")
	}
	if c.MaxRequestBodyBytes < 1<<20 || c.MaxRequestBodyBytes > 64<<20 {
		return errors.New("MAX_REQUEST_BODY_BYTES must be between 1 MiB and 64 MiB")
	}
	if !strings.HasPrefix(c.UpstreamBaseURL, "https://") && !strings.HasPrefix(c.UpstreamBaseURL, "http://") {
		return fmt.Errorf("UPSTREAM_BASE_URL must be an http(s) URL")
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func envDuration(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return parsed, nil
}

func envInt64(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}
