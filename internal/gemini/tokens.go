package gemini

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	xsrfPattern = regexp.MustCompile(`"SNlM0e":"([^"]{10,240})"`)
	blPattern   = regexp.MustCompile(`"cfb2h":"([^"]{10,120})"`)
	blShape     = regexp.MustCompile(`^boq_assistant-bard-web-server_\d{8}\.\d{2}_p\d+$`)
)

type pageTokens struct {
	xsrf      string
	bl        string
	fetchedAt time.Time
}

type TokenCache struct {
	sender  Sender
	baseURL string
	mu      sync.Mutex
	entries map[string]pageTokens
}

func NewTokenCache(sender Sender, baseURL string) *TokenCache {
	return &TokenCache{sender: sender, baseURL: strings.TrimRight(baseURL, "/"), entries: make(map[string]pageTokens)}
}

func (c *TokenCache) Resolve(ctx context.Context, credential Credential, proxyURL, pinnedBL string, autoBL bool) (string, string, error) {
	if credential.Cookie == "" && !autoBL {
		return "", pinnedBL, nil
	}
	key := tokenCacheKey(credential.Cookie, proxyURL)
	c.mu.Lock()
	entry, ok := c.entries[key]
	if ok && time.Since(entry.fetchedAt) < 20*time.Minute && (credential.Cookie == "" || entry.xsrf != "") {
		c.mu.Unlock()
		return entry.xsrf, chooseBL(entry.bl, pinnedBL, autoBL), nil
	}
	c.mu.Unlock()

	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	header := make(http.Header)
	header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	header.Set("Accept-Language", "en-US,en;q=0.9")
	if credential.Cookie != "" {
		header.Set("Cookie", credential.Cookie)
	}
	response, err := c.sender.Send(requestCtx, OutboundRequest{
		Method: http.MethodGet, URL: c.baseURL + "/app", Header: header, ProxyURL: proxyURL,
	})
	if err != nil {
		if credential.Cookie == "" {
			return "", pinnedBL, nil
		}
		return "", "", fmt.Errorf("fetch Gemini app tokens: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		if credential.Cookie == "" {
			return "", pinnedBL, nil
		}
		return "", "", fmt.Errorf("fetch Gemini app tokens: HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return "", "", fmt.Errorf("read Gemini app page: %w", err)
	}
	entry = pageTokens{fetchedAt: time.Now()}
	if match := xsrfPattern.FindSubmatch(body); len(match) > 1 {
		entry.xsrf = string(match[1])
	}
	if match := blPattern.FindSubmatch(body); len(match) > 1 && blShape.Match(match[1]) {
		entry.bl = string(match[1])
	}
	if credential.Cookie != "" && entry.xsrf == "" {
		return "", "", fmt.Errorf("signed-in cookie was not accepted: Gemini app page did not contain an XSRF token")
	}
	c.mu.Lock()
	c.entries[key] = entry
	c.mu.Unlock()
	return entry.xsrf, chooseBL(entry.bl, pinnedBL, autoBL), nil
}

func (c *TokenCache) Invalidate(credential Credential, proxyURL string) {
	c.mu.Lock()
	delete(c.entries, tokenCacheKey(credential.Cookie, proxyURL))
	c.mu.Unlock()
}

func tokenCacheKey(cookie, proxy string) string {
	sum := sha256.Sum256([]byte(cookie + "\x00" + proxy))
	return hex.EncodeToString(sum[:12])
}

func chooseBL(discovered, pinned string, auto bool) string {
	if auto && discovered != "" {
		return discovered
	}
	return pinned
}
