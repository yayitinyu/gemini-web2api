package openai

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yayitinyu/gemini-web2api/internal/gateway"
	"github.com/yayitinyu/gemini-web2api/internal/gemini"
)

type mockGateway struct {
	text string
	err  error
}

func (m *mockGateway) Models(context.Context) ([]gemini.Model, error) {
	return gemini.Models(false), nil
}

func (m *mockGateway) Generate(_ context.Context, input gateway.GenerateInput) (gateway.Execution, error) {
	if m.err != nil {
		return gateway.Execution{RequestedModel: input.Model}, m.err
	}
	for _, delta := range []string{"hel", "lo"} {
		if input.OnContent != nil {
			input.OnContent(delta)
		}
	}
	return gateway.Execution{RequestedModel: input.Model, Result: gemini.Result{Text: m.text, EmittedText: m.text}}, nil
}

func TestChatStreamingSequence(t *testing.T) {
	t.Parallel()
	handler := NewHandler(&mockGateway{text: "hello"}, nil, 1<<20)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
  "model":"gemini-3.6-flash","stream":true,"stream_options":{"include_usage":true},
  "messages":[{"role":"user","content":"hi"}]
}`))
	response := httptest.NewRecorder()
	handler.ChatCompletions(response, request)
	body := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(body, `"role":"assistant"`) || !strings.Contains(body, `"content":"hel"`) || !strings.Contains(body, `"choices":[]`) || !strings.HasSuffix(body, "data: [DONE]\n\n") {
		t.Fatalf("unexpected SSE response (%d):\n%s", response.Code, body)
	}
}

func TestResponsesLifecyclePrecedesTextDelta(t *testing.T) {
	t.Parallel()
	handler := NewHandler(&mockGateway{text: "hello"}, nil, 1<<20)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
  "model":"gemini-3.6-flash","stream":true,"input":"hi"
}`))
	response := httptest.NewRecorder()
	handler.Responses(response, request)
	body := response.Body.String()
	created := strings.Index(body, "event: response.created")
	itemAdded := strings.Index(body, "event: response.output_item.added")
	partAdded := strings.Index(body, "event: response.content_part.added")
	delta := strings.Index(body, "event: response.output_text.delta")
	completed := strings.Index(body, "event: response.completed")
	if !(created >= 0 && created < itemAdded && itemAdded < partAdded && partAdded < delta && delta < completed) {
		t.Fatalf("invalid Responses lifecycle order:\n%s", body)
	}
}

func TestResponsesNonStreamingToolCall(t *testing.T) {
	t.Parallel()
	text := "```tool_call\n{\"name\":\"lookup\",\"arguments\":{\"id\":1}}\n```"
	handler := NewHandler(&mockGateway{text: text}, nil, 1<<20)
	request := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
  "model":"gemini-3.6-flash","input":"hi",
  "tools":[{"type":"function","name":"lookup","parameters":{"type":"object"}}]
}`))
	response := httptest.NewRecorder()
	handler.Responses(response, request)
	body, _ := io.ReadAll(response.Result().Body)
	if response.Code != http.StatusOK || !strings.Contains(string(body), `"type":"function_call"`) || !strings.Contains(string(body), `"name":"lookup"`) {
		t.Fatalf("unexpected tool response (%d): %s", response.Code, body)
	}
}
