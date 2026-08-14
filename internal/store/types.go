package store

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type RuntimeSettings struct {
	DefaultModel      string `json:"default_model"`
	RequestTimeoutSec int    `json:"request_timeout_sec"`
	RetryAttempts     int    `json:"retry_attempts"`
	RetryDelayMS      int    `json:"retry_delay_ms"`
	MaxPromptBytes    int    `json:"max_prompt_bytes"`
	FallbackAnonymous bool   `json:"fallback_anonymous"`
	FallbackDirect    bool   `json:"fallback_direct"`
	GeminiBL          string `json:"gemini_bl"`
	GeminiBLAuto      bool   `json:"gemini_bl_auto"`
	RetentionDays     int    `json:"retention_days"`
}

func DefaultRuntimeSettings() RuntimeSettings {
	return RuntimeSettings{
		DefaultModel:      "gemini-3.7-flash",
		RequestTimeoutSec: 120,
		RetryAttempts:     2,
		RetryDelayMS:      750,
		MaxPromptBytes:    128_000,
		FallbackAnonymous: false,
		FallbackDirect:    false,
		GeminiBL:          "boq_assistant-bard-web-server_20260805.16_p0",
		GeminiBLAuto:      true,
		RetentionDays:     14,
	}
}

func (s RuntimeSettings) Validate(availableModels []string) error {
	if !slices.Contains(availableModels, s.DefaultModel) {
		return fmt.Errorf("unsupported default model %q", s.DefaultModel)
	}
	if s.RequestTimeoutSec < 10 || s.RequestTimeoutSec > 600 {
		return errors.New("request_timeout_sec must be between 10 and 600")
	}
	if s.RetryAttempts < 1 || s.RetryAttempts > 4 {
		return errors.New("retry_attempts must be between 1 and 4")
	}
	if s.RetryDelayMS < 0 || s.RetryDelayMS > 10_000 {
		return errors.New("retry_delay_ms must be between 0 and 10000")
	}
	if s.MaxPromptBytes != 0 && (s.MaxPromptBytes < 8_192 || s.MaxPromptBytes > 1_000_000) {
		return errors.New("max_prompt_bytes must be 0 or between 8192 and 1000000")
	}
	if s.RetentionDays < 1 || s.RetentionDays > 365 {
		return errors.New("retention_days must be between 1 and 365")
	}
	if !strings.HasPrefix(s.GeminiBL, "boq_assistant-bard-web-server_") || len(s.GeminiBL) > 120 {
		return errors.New("gemini_bl has an unexpected format")
	}
	return nil
}

type Account struct {
	ID            int64  `json:"id"`
	Label         string `json:"label"`
	Cookie        string `json:"-"`
	CookieSummary string `json:"cookie_summary"`
	Enabled       bool   `json:"enabled"`
	Status        string `json:"status"`
	Note          string `json:"note"`
	ProxyID       int64  `json:"proxy_id"`
	FailureCount  int    `json:"failure_count"`
	LastUsedAt    int64  `json:"last_used_at"`
	LastSuccessAt int64  `json:"last_success_at"`
	LastError     string `json:"last_error"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type AccountInput struct {
	Label   string `json:"label"`
	Cookie  string `json:"cookie"`
	Enabled bool   `json:"enabled"`
	Note    string `json:"note"`
	ProxyID int64  `json:"proxy_id"`
}

type AccountUpdate struct {
	Label   string  `json:"label"`
	Cookie  *string `json:"cookie,omitempty"`
	Enabled bool    `json:"enabled"`
	Note    string  `json:"note"`
	ProxyID int64   `json:"proxy_id"`
}

type Proxy struct {
	ID            int64  `json:"id"`
	Label         string `json:"label"`
	URL           string `json:"-"`
	URLSummary    string `json:"url_summary"`
	Enabled       bool   `json:"enabled"`
	Status        string `json:"status"`
	FailureCount  int    `json:"failure_count"`
	LastUsedAt    int64  `json:"last_used_at"`
	LastSuccessAt int64  `json:"last_success_at"`
	LastError     string `json:"last_error"`
	CooldownUntil int64  `json:"cooldown_until"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type ProxyInput struct {
	Label   string `json:"label"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type ProxyUpdate struct {
	Label   string  `json:"label"`
	URL     *string `json:"url,omitempty"`
	Enabled bool    `json:"enabled"`
}

type RequestRecord struct {
	RequestID     string
	CreatedAt     int64
	Endpoint      string
	Model         string
	UpstreamModel string
	StatusCode    int
	LatencyMS     int64
	TTFBMS        int64
	InputTokens   int
	OutputTokens  int
	Stream        bool
	AccountID     int64
	AccountLabel  string
	ProxyID       int64
	ProxyLabel    string
	ErrorCode     string
	ErrorMessage  string
}

type RequestRow struct {
	ID            int64  `json:"id"`
	RequestID     string `json:"request_id"`
	CreatedAt     int64  `json:"created_at"`
	Endpoint      string `json:"endpoint"`
	Model         string `json:"model"`
	UpstreamModel string `json:"upstream_model"`
	StatusCode    int    `json:"status_code"`
	LatencyMS     int64  `json:"latency_ms"`
	TTFBMS        int64  `json:"ttfb_ms"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	Stream        bool   `json:"stream"`
	AccountID     int64  `json:"account_id"`
	AccountLabel  string `json:"account_label"`
	ProxyID       int64  `json:"proxy_id"`
	ProxyLabel    string `json:"proxy_label"`
	ErrorCode     string `json:"error_code"`
	ErrorMessage  string `json:"error_message"`
}

type RequestFilter struct {
	Limit  int
	Offset int
	Model  string
	Status string
}

type RequestPage struct {
	Items  []RequestRow `json:"items"`
	Total  int64        `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}

type OverviewStats struct {
	Requests     int64    `json:"requests"`
	SuccessRate  *float64 `json:"success_rate"`
	P50LatencyMS *int64   `json:"p50_latency_ms"`
	OutputTokens int64    `json:"output_tokens"`
	Accounts     int64    `json:"accounts"`
	Healthy      int64    `json:"healthy_accounts"`
	Proxies      int64    `json:"proxies"`
}

type TimePoint struct {
	Bucket    int64 `json:"bucket"`
	Requests  int64 `json:"requests"`
	Failures  int64 `json:"failures"`
	LatencyMS int64 `json:"latency_ms"`
}

type Session struct {
	TokenHash string
	CSRFToken string
	ExpiresAt int64
}

func unixNow() int64 { return time.Now().Unix() }
