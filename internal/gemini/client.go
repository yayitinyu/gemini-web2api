package gemini

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	sender  Sender
	baseURL string
	tokens  *TokenCache
	now     func() time.Time
}

func NewClient(sender Sender, baseURL string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{sender: sender, baseURL: baseURL, tokens: NewTokenCache(sender, baseURL), now: time.Now}
}

func (c *Client) Generate(ctx context.Context, request GenerateRequest) (Result, error) {
	if strings.TrimSpace(request.Prompt) == "" {
		return Result{}, &UpstreamError{Code: "empty_prompt", Message: "prompt is empty"}
	}
	if request.Model.ID == "" || request.Model.HexID == "" {
		return Result{}, &UpstreamError{Code: "invalid_model", Message: "model configuration is incomplete"}
	}
	if request.Options.Timeout <= 0 {
		request.Options.Timeout = 120 * time.Second
	}
	if request.Options.RetryAttempts < 1 {
		request.Options.RetryAttempts = 1
	}

	started := c.now()
	textTracker := &DeltaTracker{}
	reasoningTracker := &DeltaTracker{}
	result := Result{}
	var lastErr error
	xsrfRetried := false

	for attempt := 0; attempt < request.Options.RetryAttempts; attempt++ {
		xsrf, bl, err := c.tokens.Resolve(ctx, request.Credential, request.ProxyURL, request.Options.PinnedBL, request.Options.AutoRefreshBL)
		if err != nil {
			return result, err
		}
		body, err := BuildPayload(request.Prompt, request.Model, xsrf)
		if err != nil {
			return result, err
		}
		requestCtx, cancel := context.WithTimeout(ctx, request.Options.Timeout)
		response, sendStarted, err := c.send(requestCtx, request, body, bl)
		if err != nil {
			cancel()
			lastErr = err
			if textTracker.Emitted() != "" || reasoningTracker.Emitted() != "" {
				break
			}
			if attempt+1 < request.Options.RetryAttempts {
				if err := waitRetry(ctx, request.Options.RetryDelay); err != nil {
					return result, err
				}
			}
			continue
		}

		result.SetCookies = append(result.SetCookies, response.Header.Values("Set-Cookie")...)
		scanned, scanErr := scanFrames(response.Body, sendStarted, func(frame Frame) {
			if frame.Reasoning != "" && request.OnReasoning != nil {
				if delta := reasoningTracker.Push(frame.Reasoning); delta != "" {
					request.OnReasoning(delta)
				}
			}
			if frame.Text != "" && request.OnContent != nil {
				if delta := textTracker.Push(frame.Text); delta != "" {
					request.OnContent(delta)
				}
			}
		})
		response.Body.Close()
		cancel()
		result.TTFB = scanned.firstFrame
		result.UpstreamModel = scanned.upstreamModel
		result.Text = scanned.text
		result.Reasoning = scanned.reasoning
		result.EmittedText = textTracker.Emitted()
		result.EmittedReasoning = reasoningTracker.Emitted()
		if scanErr != nil {
			lastErr = fmt.Errorf("read Gemini Web stream: %w", scanErr)
			if result.EmittedText != "" || result.EmittedReasoning != "" {
				break
			}
			continue
		}
		if response.StatusCode != http.StatusOK {
			message := truncateForError(scanned.raw, 300)
			if response.StatusCode == http.StatusBadRequest && strings.Contains(scanned.raw, `"xsrf"`) && request.Credential.Cookie != "" && !xsrfRetried {
				xsrfRetried = true
				c.tokens.Invalidate(request.Credential, request.ProxyURL)
				attempt--
				continue
			}
			lastErr = &UpstreamError{StatusCode: response.StatusCode, Code: "upstream_http_error", Message: message}
			if result.EmittedText != "" || result.EmittedReasoning != "" {
				break
			}
			if attempt+1 < request.Options.RetryAttempts {
				if err := waitRetry(ctx, request.Options.RetryDelay); err != nil {
					return result, err
				}
			}
			continue
		}
		if !scanned.contentFrameSeen || scanned.text == "" {
			lastErr = ErrNoContent
			if result.EmittedText != "" || result.EmittedReasoning != "" {
				break
			}
			if attempt+1 < request.Options.RetryAttempts {
				if err := waitRetry(ctx, request.Options.RetryDelay); err != nil {
					return result, err
				}
			}
			continue
		}
		result.Total = c.now().Sub(started)
		return result, nil
	}
	result.Total = c.now().Sub(started)
	if lastErr == nil {
		lastErr = ErrNoContent
	}
	return result, lastErr
}

func (c *Client) send(ctx context.Context, request GenerateRequest, body, bl string) (*OutboundResponse, time.Time, error) {
	reqid := strconv.FormatInt(c.now().Unix()%1_000_000, 10)
	endpoint := c.baseURL + "/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate?" + url.Values{
		"bl": []string{bl}, "hl": []string{"en"}, "_reqid": []string{reqid}, "rt": []string{"c"},
	}.Encode()
	header := geminiHeaders(request.Credential, request.Model)
	started := c.now()
	response, err := c.sender.Send(ctx, OutboundRequest{
		Method: http.MethodPost, URL: endpoint, Header: header, Body: body, ProxyURL: request.ProxyURL,
	})
	return response, started, err
}

func geminiHeaders(credential Credential, model Model) http.Header {
	header := make(http.Header)
	header.Set("Accept", "*/*")
	header.Set("Accept-Language", "en-US,en;q=0.9")
	header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	header.Set("Origin", "https://gemini.google.com")
	header.Set("Referer", "https://gemini.google.com/app")
	header.Set("X-Same-Domain", "1")
	header.Set("X-Goog-AuthUser", "0")
	header.Set("x-goog-ext-525001261-jspb", fmt.Sprintf(`[1,null,null,null,%q]`, model.HexID))
	if credential.Cookie != "" {
		header.Set("Cookie", credential.Cookie)
	}
	if credential.SAPISID != "" {
		header.Set("Authorization", makeSAPISIDHash(credential.SAPISID))
	}
	return header
}

func makeSAPISIDHash(sapisid string) string {
	timestamp := time.Now().Unix()
	digest := sha1.Sum([]byte(fmt.Sprintf("%d %s https://gemini.google.com", timestamp, sapisid)))
	return fmt.Sprintf("SAPISIDHASH %d_%s", timestamp, hex.EncodeToString(digest[:]))
}

func ExtractSAPISID(cookie string) string {
	for _, part := range strings.Split(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && name == "SAPISID" {
			return value
		}
	}
	return ""
}

func MergeSetCookies(cookie string, setCookies []string) string {
	values := make(map[string]string)
	order := make([]string, 0)
	for _, part := range strings.Split(cookie, ";") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || name == "" {
			continue
		}
		if _, exists := values[name]; !exists {
			order = append(order, name)
		}
		values[name] = value
	}
	for _, raw := range setCookies {
		pair := strings.SplitN(raw, ";", 2)[0]
		name, value, ok := strings.Cut(strings.TrimSpace(pair), "=")
		if !ok || name == "" {
			continue
		}
		if _, exists := values[name]; !exists {
			order = append(order, name)
		}
		values[name] = value
	}
	parts := make([]string, 0, len(order))
	for _, name := range order {
		parts = append(parts, name+"="+values[name])
	}
	return strings.Join(parts, "; ")
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func truncateForError(value string, limit int) string {
	value = strings.TrimSpace(value)
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}
