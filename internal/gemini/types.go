package gemini

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Credential struct {
	Cookie  string
	SAPISID string
}

type Options struct {
	Timeout       time.Duration
	RetryAttempts int
	RetryDelay    time.Duration
	PinnedBL      string
	AutoRefreshBL bool
}

type GenerateRequest struct {
	Prompt      string
	Model       Model
	Credential  Credential
	ProxyURL    string
	Options     Options
	OnContent   func(string)
	OnReasoning func(string)
}

type Result struct {
	Text             string
	Reasoning        string
	EmittedText      string
	EmittedReasoning string
	UpstreamModel    string
	TTFB             time.Duration
	Total            time.Duration
	SetCookies       []string
}

type OutboundRequest struct {
	Method   string
	URL      string
	Header   http.Header
	Body     string
	ProxyURL string
}

type OutboundResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

type Sender interface {
	Send(context.Context, OutboundRequest) (*OutboundResponse, error)
}

type UpstreamError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *UpstreamError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("Gemini Web upstream HTTP %d: %s", e.StatusCode, e.Message)
	}
	return "Gemini Web upstream: " + e.Message
}

var ErrNoContent = errors.New("Gemini Web upstream returned no content frame")
