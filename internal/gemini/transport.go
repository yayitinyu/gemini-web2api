package gemini

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const chromeUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"

type BrowserTransport struct {
	direct  tls_client.HttpClient
	proxies sync.Map
}

func NewBrowserTransport() (*BrowserTransport, error) {
	direct, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(600),
		tls_client.WithClientProfile(profiles.Chrome_146),
		tls_client.WithRandomTLSExtensionOrder(),
		tls_client.WithNotFollowRedirects(),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize Chrome TLS transport: %w", err)
	}
	return &BrowserTransport{direct: direct}, nil
}

func (t *BrowserTransport) Send(ctx context.Context, outbound OutboundRequest) (*OutboundResponse, error) {
	if outbound.ProxyURL != "" {
		return t.sendProxy(ctx, outbound)
	}
	request, err := fhttp.NewRequestWithContext(ctx, outbound.Method, outbound.URL, strings.NewReader(outbound.Body))
	if err != nil {
		return nil, err
	}
	for key, values := range outbound.Header {
		for _, value := range values {
			request.Header.Add(key, value)
		}
	}
	request.Header.Set("User-Agent", chromeUA)
	request.Header.Set("Sec-CH-UA", `"Chromium";v="146", "Google Chrome";v="146", "Not?A_Brand";v="24"`)
	request.Header.Set("Sec-CH-UA-Mobile", "?0")
	request.Header.Set("Sec-CH-UA-Platform", `"Windows"`)
	response, err := t.direct.Do(request)
	if err != nil {
		return nil, err
	}
	return &OutboundResponse{StatusCode: response.StatusCode, Header: copyHeader(response.Header), Body: response.Body}, nil
}

func (t *BrowserTransport) sendProxy(ctx context.Context, outbound OutboundRequest) (*OutboundResponse, error) {
	client, err := t.proxyClient(outbound.ProxyURL)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, outbound.Method, outbound.URL, strings.NewReader(outbound.Body))
	if err != nil {
		return nil, err
	}
	request.Header = outbound.Header.Clone()
	applyChromeHeaders(request.Header)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	return &OutboundResponse{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: response.Body}, nil
}

func (t *BrowserTransport) proxyClient(proxyURL string) (*http.Client, error) {
	if cached, ok := t.proxies.Load(proxyURL); ok {
		return cached.(*http.Client), nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("parse proxy URL: %w", err)
	}
	transport := &http.Transport{
		Proxy:               http.ProxyURL(parsed),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   false,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	actual, _ := t.proxies.LoadOrStore(proxyURL, client)
	return actual.(*http.Client), nil
}

func (t *BrowserTransport) CloseIdleConnections() {
	t.direct.CloseIdleConnections()
	t.proxies.Range(func(_, value any) bool {
		value.(*http.Client).CloseIdleConnections()
		return true
	})
}

func applyChromeHeaders(header http.Header) {
	header.Set("User-Agent", chromeUA)
	header.Set("Sec-CH-UA", `"Chromium";v="146", "Google Chrome";v="146", "Not?A_Brand";v="24"`)
	header.Set("Sec-CH-UA-Mobile", "?0")
	header.Set("Sec-CH-UA-Platform", `"Windows"`)
	header.Set("Sec-Fetch-Dest", "empty")
	header.Set("Sec-Fetch-Mode", "cors")
	header.Set("Sec-Fetch-Site", "same-origin")
}

func copyHeader(source fhttp.Header) http.Header {
	destination := make(http.Header, len(source))
	for key, values := range source {
		destination[key] = append([]string(nil), values...)
	}
	return destination
}
