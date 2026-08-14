package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yayitinyu/gemini-web2api/internal/gateway"
	"github.com/yayitinyu/gemini-web2api/internal/gemini"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

type Gateway interface {
	Models(context.Context) ([]gemini.Model, error)
	Generate(context.Context, gateway.GenerateInput) (gateway.Execution, error)
}

type Handler struct {
	gateway      Gateway
	store        *store.Store
	maxBodyBytes int64
}

func NewHandler(gw Gateway, st *store.Store, maxBodyBytes int64) *Handler {
	return &Handler{gateway: gw, store: st, maxBodyBytes: maxBodyBytes}
}

func (h *Handler) Models(w http.ResponseWriter, r *http.Request) {
	models, err := h.gateway.Models(r.Context())
	if err != nil {
		writeOpenAIError(w, http.StatusInternalServerError, "server_error", "models_unavailable", err.Error())
		return
	}
	data := make([]map[string]any, 0, len(models))
	for _, model := range models {
		data = append(data, map[string]any{
			"id": model.ID, "object": "model", "created": 1_700_000_000,
			"owned_by": "google-web", "description": model.Description,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	var request map[string]any
	if err := decoder.Decode(&request); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid_json", "request body must be valid JSON")
		return nil, false
	}
	return request, true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeOpenAIError(w http.ResponseWriter, status int, errorType, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"message": message, "type": errorType, "param": nil, "code": code,
	}})
}

type mappedError struct {
	Status int
	Type   string
	Code   string
	Text   string
}

func mapGatewayError(err error) mappedError {
	mapped := mappedError{Status: http.StatusBadGateway, Type: "api_error", Code: "upstream_error", Text: err.Error()}
	var lengthError *gateway.ContextLengthError
	var proxyError *gateway.ProxyUnavailableError
	var accountError *gateway.NoUsableAccountError
	var upstreamError *gemini.UpstreamError
	switch {
	case errors.As(err, &lengthError):
		mapped.Status, mapped.Type, mapped.Code = http.StatusBadRequest, "invalid_request_error", "context_length_exceeded"
	case errors.As(err, &proxyError):
		mapped.Status, mapped.Type, mapped.Code = http.StatusTooManyRequests, "rate_limit_error", "proxy_pool_unavailable"
	case errors.As(err, &accountError):
		mapped.Status, mapped.Type, mapped.Code = http.StatusServiceUnavailable, "api_error", "account_pool_unavailable"
	case errors.As(err, &upstreamError):
		if upstreamError.StatusCode == http.StatusTooManyRequests {
			mapped.Status, mapped.Type, mapped.Code = http.StatusTooManyRequests, "rate_limit_error", "upstream_rate_limit"
		}
	case strings.Contains(err.Error(), "unknown model") || strings.Contains(err.Error(), "requires a signed-in"):
		mapped.Status, mapped.Type, mapped.Code = http.StatusBadRequest, "invalid_request_error", "model_not_available"
	case errors.Is(err, context.Canceled):
		mapped.Status, mapped.Type, mapped.Code = 499, "api_error", "request_cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		mapped.Status, mapped.Type, mapped.Code = http.StatusGatewayTimeout, "api_error", "upstream_timeout"
	}
	return mapped
}

func (h *Handler) record(endpoint, requestID, model, prompt, output string, execution gateway.Execution, status int, err error, stream bool, started time.Time) {
	if h.store == nil {
		return
	}
	record := store.RequestRecord{
		RequestID: requestID, CreatedAt: started.Unix(), Endpoint: endpoint, Model: model,
		UpstreamModel: execution.Result.UpstreamModel, StatusCode: status,
		LatencyMS: execution.Result.Total.Milliseconds(), TTFBMS: execution.Result.TTFB.Milliseconds(),
		InputTokens: estimateTokens(prompt), OutputTokens: estimateTokens(output), Stream: stream,
	}
	if record.LatencyMS == 0 {
		record.LatencyMS = time.Since(started).Milliseconds()
	}
	if execution.Account != nil {
		record.AccountID, record.AccountLabel = execution.Account.ID, execution.Account.Label
	}
	if execution.Proxy != nil {
		record.ProxyID, record.ProxyLabel = execution.Proxy.ID, execution.Proxy.Label
	}
	if err != nil {
		mapped := mapGatewayError(err)
		record.ErrorCode, record.ErrorMessage = mapped.Code, mapped.Text
	}
	_ = h.store.RecordRequest(context.WithoutCancel(context.Background()), record)
}

func estimateTokens(value string) int {
	if value == "" {
		return 0
	}
	ascii, nonASCII := 0, 0
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		value = value[size:]
		if r <= 0x7f {
			ascii++
		} else {
			nonASCII++
		}
	}
	return (ascii+3)/4 + nonASCII
}

func usage(prompt, output string, responses bool) map[string]int {
	inputTokens, outputTokens := estimateTokens(prompt), estimateTokens(output)
	if responses {
		return map[string]int{"input_tokens": inputTokens, "output_tokens": outputTokens, "total_tokens": inputTokens + outputTokens}
	}
	return map[string]int{"prompt_tokens": inputTokens, "completion_tokens": outputTokens, "total_tokens": inputTokens + outputTokens}
}

func numberValue(value any) float64 {
	switch number := value.(type) {
	case json.Number:
		parsed, _ := number.Float64()
		return parsed
	case float64:
		return number
	default:
		return 0
	}
}

func randomID(prefix string, length int) string { return prefix + randomHex(length) }

func ensurePrompt(request map[string]any, tools []any, choice any) (string, mappedError, bool) {
	messages, ok := request["messages"].([]any)
	if !ok || len(messages) == 0 {
		return "", mappedError{Status: 400, Type: "invalid_request_error", Code: "messages_required", Text: "messages must be a non-empty array"}, false
	}
	result, err := messagesToPrompt(messages, tools, choice)
	if err != nil {
		return "", mappedError{Status: 400, Type: "invalid_request_error", Code: "invalid_tools", Text: err.Error()}, false
	}
	if result.Unsupported != "" {
		return "", mappedError{Status: 400, Type: "invalid_request_error", Code: "unsupported_input", Text: result.Unsupported}, false
	}
	if strings.TrimSpace(result.Prompt) == "" {
		return "", mappedError{Status: 400, Type: "invalid_request_error", Code: "empty_prompt", Text: "messages contain no text"}, false
	}
	return result.Prompt, mappedError{}, true
}

func writeMappedError(w http.ResponseWriter, mapped mappedError) {
	writeOpenAIError(w, mapped.Status, mapped.Type, mapped.Code, mapped.Text)
}

func modelOrDefault(request map[string]any, st *store.Store, ctx context.Context) string {
	if model := stringValue(request["model"]); model != "" {
		return model
	}
	if st != nil {
		if settings, err := st.RuntimeSettings(ctx); err == nil {
			return settings.DefaultModel
		}
	}
	return "gemini-3.7-flash"
}

func unsupportedN(request map[string]any) error {
	if n := int(numberValue(request["n"])); n > 1 {
		return fmt.Errorf("n=%d is not supported because Gemini Web returns one candidate", n)
	}
	return nil
}
