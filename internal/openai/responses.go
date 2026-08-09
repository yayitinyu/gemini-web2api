package openai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/gateway"
)

func (h *Handler) Responses(w http.ResponseWriter, r *http.Request) {
	request, ok := h.decode(w, r)
	if !ok {
		return
	}
	messages, err := responsesInputToMessages(stringValue(request["instructions"]), request["input"])
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid_input", err.Error())
		return
	}
	tools, _ := request["tools"].([]any)
	promptResult, err := messagesToPrompt(messages, tools, request["tool_choice"])
	if err != nil {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "invalid_tools", err.Error())
		return
	}
	if promptResult.Unsupported != "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "unsupported_input", promptResult.Unsupported)
		return
	}
	prompt := strings.TrimSpace(promptResult.Prompt)
	if prompt == "" {
		writeOpenAIError(w, http.StatusBadRequest, "invalid_request_error", "empty_input", "input contains no text")
		return
	}
	model := modelOrDefault(request, h.store, r.Context())
	stream, _ := request["stream"].(bool)
	responseID := randomID("resp_", 24)
	started := time.Now()
	if stream {
		h.streamResponses(w, r, tools, model, prompt, responseID, started)
		return
	}
	execution, err := h.gateway.Generate(r.Context(), gateway.GenerateInput{Model: model, Prompt: prompt})
	if err != nil {
		mapped := mapGatewayError(err)
		h.record("responses", responseID, model, prompt, "", execution, mapped.Status, err, false, started)
		writeMappedError(w, mapped)
		return
	}
	text := execution.Result.Text
	var calls []ToolCall
	if len(tools) > 0 {
		text, calls = parseToolCalls(text)
	}
	output := responseOutput(text, calls)
	response := completedResponse(responseID, execution.RequestedModel, started, output, usage(prompt, text, true))
	h.record("responses", responseID, execution.RequestedModel, prompt, text, execution, http.StatusOK, nil, false, started)
	writeJSON(w, http.StatusOK, response)
}

type responseEventWriter struct {
	w        http.ResponseWriter
	flusher  http.Flusher
	sequence int
}

func newResponseEventWriter(w http.ResponseWriter) *responseEventWriter {
	flusher, _ := w.(http.Flusher)
	return &responseEventWriter{w: w, flusher: flusher}
}

func (e *responseEventWriter) event(eventType string, payload map[string]any) {
	payload["type"] = eventType
	payload["sequence_number"] = e.sequence
	e.sequence++
	encoded, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(e.w, "event: %s\ndata: %s\n\n", eventType, encoded)
	if e.flusher != nil {
		e.flusher.Flush()
	}
}

func (h *Handler) streamResponses(w http.ResponseWriter, r *http.Request, tools []any, model, prompt, responseID string, started time.Time) {
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	events := newResponseEventWriter(w)
	base := inProgressResponse(responseID, model, started)
	events.event("response.created", map[string]any{"response": base})
	events.event("response.in_progress", map[string]any{"response": base})

	// Function-call fences require complete text before they can be parsed. For
	// tool requests we keep the transport streaming upstream, then emit a valid
	// ordered Responses lifecycle once the item type is known.
	if len(tools) > 0 {
		execution, err := h.gateway.Generate(r.Context(), gateway.GenerateInput{Model: model, Prompt: prompt})
		if err != nil {
			h.failedResponse(events, responseID, model, started, err)
			mapped := mapGatewayError(err)
			h.record("responses", responseID, model, prompt, "", execution, mapped.Status, err, true, started)
			return
		}
		text, calls := parseToolCalls(execution.Result.Text)
		output := make([]map[string]any, 0, len(calls)+1)
		outputIndex := 0
		for _, call := range calls {
			item := functionCallItem(call)
			events.event("response.output_item.added", map[string]any{"output_index": outputIndex, "item": pendingFunctionCallItem(call)})
			events.event("response.function_call_arguments.delta", map[string]any{
				"item_id": call.ID, "output_index": outputIndex, "delta": call.Function.Arguments,
			})
			events.event("response.function_call_arguments.done", map[string]any{
				"item_id": call.ID, "output_index": outputIndex, "name": call.Function.Name, "arguments": call.Function.Arguments,
			})
			events.event("response.output_item.done", map[string]any{"output_index": outputIndex, "item": item})
			output = append(output, item)
			outputIndex++
		}
		if text != "" || len(calls) == 0 {
			message := h.emitBufferedMessage(events, text, outputIndex)
			output = append(output, message)
		}
		completed := completedResponse(responseID, execution.RequestedModel, started, output, usage(prompt, text, true))
		events.event("response.completed", map[string]any{"response": completed})
		h.record("responses", responseID, execution.RequestedModel, prompt, text, execution, http.StatusOK, nil, true, started)
		return
	}

	messageID := randomID("msg_", 18)
	pendingMessage := map[string]any{
		"id": messageID, "type": "message", "status": "in_progress", "role": "assistant", "content": []any{},
	}
	pendingPart := map[string]any{"type": "output_text", "text": "", "annotations": []any{}, "logprobs": []any{}}
	events.event("response.output_item.added", map[string]any{"output_index": 0, "item": pendingMessage})
	events.event("response.content_part.added", map[string]any{
		"item_id": messageID, "output_index": 0, "content_index": 0, "part": pendingPart,
	})
	var sent strings.Builder
	onContent := func(delta string) {
		if delta == "" {
			return
		}
		sent.WriteString(delta)
		events.event("response.output_text.delta", map[string]any{
			"item_id": messageID, "output_index": 0, "content_index": 0, "delta": delta, "logprobs": []any{},
		})
	}
	execution, err := h.gateway.Generate(r.Context(), gateway.GenerateInput{Model: model, Prompt: prompt, OnContent: onContent})
	if err != nil {
		h.failedResponse(events, responseID, model, started, err)
		mapped := mapGatewayError(err)
		h.record("responses", responseID, model, prompt, sent.String(), execution, mapped.Status, err, true, started)
		return
	}
	text := execution.Result.Text
	if rest := remainingText(text, sent.String()); rest != "" {
		onContent(rest)
	}
	part := map[string]any{"type": "output_text", "text": text, "annotations": []any{}, "logprobs": []any{}}
	message := map[string]any{
		"id": messageID, "type": "message", "status": "completed", "role": "assistant", "content": []any{part},
	}
	events.event("response.output_text.done", map[string]any{
		"item_id": messageID, "output_index": 0, "content_index": 0, "text": text, "logprobs": []any{},
	})
	events.event("response.content_part.done", map[string]any{
		"item_id": messageID, "output_index": 0, "content_index": 0, "part": part,
	})
	events.event("response.output_item.done", map[string]any{"output_index": 0, "item": message})
	completed := completedResponse(responseID, execution.RequestedModel, started, []map[string]any{message}, usage(prompt, text, true))
	events.event("response.completed", map[string]any{"response": completed})
	h.record("responses", responseID, execution.RequestedModel, prompt, text, execution, http.StatusOK, nil, true, started)
}

func (h *Handler) emitBufferedMessage(events *responseEventWriter, text string, outputIndex int) map[string]any {
	messageID := randomID("msg_", 18)
	pending := map[string]any{"id": messageID, "type": "message", "status": "in_progress", "role": "assistant", "content": []any{}}
	pendingPart := map[string]any{"type": "output_text", "text": "", "annotations": []any{}, "logprobs": []any{}}
	events.event("response.output_item.added", map[string]any{"output_index": outputIndex, "item": pending})
	events.event("response.content_part.added", map[string]any{
		"item_id": messageID, "output_index": outputIndex, "content_index": 0, "part": pendingPart,
	})
	if text != "" {
		events.event("response.output_text.delta", map[string]any{
			"item_id": messageID, "output_index": outputIndex, "content_index": 0, "delta": text, "logprobs": []any{},
		})
	}
	part := map[string]any{"type": "output_text", "text": text, "annotations": []any{}, "logprobs": []any{}}
	message := map[string]any{"id": messageID, "type": "message", "status": "completed", "role": "assistant", "content": []any{part}}
	events.event("response.output_text.done", map[string]any{
		"item_id": messageID, "output_index": outputIndex, "content_index": 0, "text": text, "logprobs": []any{},
	})
	events.event("response.content_part.done", map[string]any{
		"item_id": messageID, "output_index": outputIndex, "content_index": 0, "part": part,
	})
	events.event("response.output_item.done", map[string]any{"output_index": outputIndex, "item": message})
	return message
}

func (h *Handler) failedResponse(events *responseEventWriter, responseID, model string, started time.Time, err error) {
	mapped := mapGatewayError(err)
	response := inProgressResponse(responseID, model, started)
	response["status"] = "failed"
	response["error"] = map[string]any{"code": mapped.Code, "message": mapped.Text}
	events.event("response.failed", map[string]any{"response": response})
}

func responseOutput(text string, calls []ToolCall) []map[string]any {
	output := make([]map[string]any, 0, len(calls)+1)
	for _, call := range calls {
		output = append(output, functionCallItem(call))
	}
	if text != "" || len(calls) == 0 {
		output = append(output, map[string]any{
			"id": randomID("msg_", 18), "type": "message", "status": "completed", "role": "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}, "logprobs": []any{}}},
		})
	}
	return output
}

func functionCallItem(call ToolCall) map[string]any {
	return map[string]any{
		"id": call.ID, "type": "function_call", "status": "completed", "call_id": call.ID,
		"name": call.Function.Name, "arguments": call.Function.Arguments,
	}
}

func pendingFunctionCallItem(call ToolCall) map[string]any {
	return map[string]any{
		"id": call.ID, "type": "function_call", "status": "in_progress", "call_id": call.ID,
		"name": call.Function.Name, "arguments": "",
	}
}

func inProgressResponse(id, model string, started time.Time) map[string]any {
	return map[string]any{
		"id": id, "object": "response", "created_at": started.Unix(), "status": "in_progress",
		"model": model, "output": []any{}, "error": nil, "incomplete_details": nil,
	}
}

func completedResponse(id, model string, started time.Time, output []map[string]any, usageValue map[string]int) map[string]any {
	return map[string]any{
		"id": id, "object": "response", "created_at": started.Unix(), "status": "completed",
		"model": model, "output": output, "usage": usageValue, "error": nil, "incomplete_details": nil,
	}
}
