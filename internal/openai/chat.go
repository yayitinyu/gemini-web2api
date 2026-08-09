package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/gateway"
)

func (h *Handler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	request, ok := h.decode(w, r)
	if !ok {
		return
	}
	if err := unsupportedN(request); err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "unsupported_parameter", err.Error())
		return
	}
	tools, _ := request["tools"].([]any)
	prompt, invalid, ok := ensurePrompt(request, tools, request["tool_choice"])
	if !ok {
		writeMappedError(w, invalid)
		return
	}
	model := modelOrDefault(request, h.store, r.Context())
	stream, _ := request["stream"].(bool)
	requestID := randomID("chatcmpl_", 24)
	started := time.Now()
	if stream {
		h.streamChat(w, r, request, tools, model, prompt, requestID, started)
		return
	}

	execution, err := h.gateway.Generate(r.Context(), gateway.GenerateInput{Model: model, Prompt: prompt})
	if err != nil {
		mapped := mapGatewayError(err)
		h.record("chat.completions", requestID, model, prompt, "", execution, mapped.Status, err, false, started)
		writeMappedError(w, mapped)
		return
	}
	text := execution.Result.Text
	var calls []ToolCall
	if len(tools) > 0 {
		text, calls = parseToolCalls(text)
	}
	message := map[string]any{"role": "assistant", "content": text}
	finishReason := "stop"
	if execution.Result.Reasoning != "" {
		message["reasoning_content"] = execution.Result.Reasoning
	}
	if len(calls) > 0 {
		message["tool_calls"] = calls
		finishReason = "tool_calls"
		if text == "" {
			message["content"] = nil
		}
	}
	response := map[string]any{
		"id": requestID, "object": "chat.completion", "created": started.Unix(), "model": execution.RequestedModel,
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finishReason, "logprobs": nil}},
		"usage":   usage(prompt, text, false),
	}
	h.record("chat.completions", requestID, execution.RequestedModel, prompt, text, execution, http.StatusOK, nil, false, started)
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) streamChat(w http.ResponseWriter, r *http.Request, request map[string]any, tools []any, model, prompt, requestID string, started time.Time) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	writeChunk := func(payload any) {
		encoded, _ := json.Marshal(payload)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", encoded)
		if flusher != nil {
			flusher.Flush()
		}
	}
	chunk := func(delta map[string]any, finish any, usageValue any) {
		payload := map[string]any{
			"id": requestID, "object": "chat.completion.chunk", "created": started.Unix(), "model": model,
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish, "logprobs": nil}},
		}
		if usageValue != nil {
			payload["usage"] = usageValue
		}
		writeChunk(payload)
	}
	chunk(map[string]any{"role": "assistant", "content": ""}, nil, nil)

	var visible strings.Builder
	emitContent := func(delta string) {
		if delta == "" {
			return
		}
		visible.WriteString(delta)
		chunk(map[string]any{"content": delta}, nil, nil)
	}
	var gate *toolFenceGate
	onContent := emitContent
	if len(tools) > 0 {
		gate = newToolFenceGate(emitContent)
		onContent = gate.Push
	}
	onReasoning := func(delta string) {
		if delta != "" {
			chunk(map[string]any{"reasoning_content": delta}, nil, nil)
		}
	}
	execution, err := h.gateway.Generate(r.Context(), gateway.GenerateInput{
		Model: model, Prompt: prompt, OnContent: onContent, OnReasoning: onReasoning,
	})
	if err != nil {
		mapped := mapGatewayError(err)
		h.record("chat.completions", requestID, model, prompt, visible.String(), execution, mapped.Status, err, true, started)
		writeChunk(map[string]any{"error": map[string]any{"message": mapped.Text, "type": mapped.Type, "code": mapped.Code}})
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		return
	}

	text := execution.Result.Text
	var calls []ToolCall
	if len(tools) > 0 {
		text, calls = parseToolCalls(text)
	}
	if rest := remainingText(text, visible.String()); rest != "" {
		emitContent(rest)
	}
	finishReason := "stop"
	if len(calls) > 0 {
		finishReason = "tool_calls"
		toolDeltas := make([]map[string]any, 0, len(calls))
		for index, call := range calls {
			toolDeltas = append(toolDeltas, map[string]any{
				"index": index, "id": call.ID, "type": "function",
				"function": map[string]any{"name": call.Function.Name, "arguments": call.Function.Arguments},
			})
		}
		chunk(map[string]any{"tool_calls": toolDeltas}, nil, nil)
	}
	chunk(map[string]any{}, finishReason, nil)
	includeUsage := false
	if streamOptions, ok := request["stream_options"].(map[string]any); ok {
		includeUsage, _ = streamOptions["include_usage"].(bool)
	}
	if includeUsage {
		payload := map[string]any{
			"id": requestID, "object": "chat.completion.chunk", "created": started.Unix(), "model": execution.RequestedModel,
			"choices": []any{}, "usage": usage(prompt, text, false),
		}
		writeChunk(payload)
	}
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	h.record("chat.completions", requestID, execution.RequestedModel, prompt, text, execution, http.StatusOK, nil, true, started)
}
