package openai

import (
	"strings"
	"testing"
)

func TestMessagesToPromptPreservesRolesAndToolChoice(t *testing.T) {
	t.Parallel()
	messages := []any{
		map[string]any{"role": "system", "content": "Be exact."},
		map[string]any{"role": "user", "content": "Find it."},
		map[string]any{"role": "tool", "name": "lookup", "content": "42"},
	}
	tools := []any{map[string]any{"type": "function", "function": map[string]any{
		"name": "lookup", "description": "Lookup", "parameters": map[string]any{"type": "object"},
	}}}
	result, err := messagesToPrompt(messages, tools, "required")
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"[System instruction]", "[User]", "[Tool result: lookup]", "must call exactly one"} {
		if !strings.Contains(result.Prompt, expected) {
			t.Fatalf("prompt missing %q:\n%s", expected, result.Prompt)
		}
	}
}

func TestToolFenceGateNeverLeaksToolBlockAcrossChunks(t *testing.T) {
	t.Parallel()
	var output strings.Builder
	gate := newToolFenceGate(func(delta string) { output.WriteString(delta) })
	for _, chunk := range []string{"before `", "``tool_", "call\n{\"name\":\"x\",\"arguments\":{}}\n`", "`` after"} {
		gate.Push(chunk)
	}
	if output.String() != "before  after" {
		t.Fatalf("unexpected visible output %q", output.String())
	}
	if strings.Contains(output.String(), "tool_call") {
		t.Fatal("tool fence leaked to visible content")
	}
}

func TestParseToolCalls(t *testing.T) {
	t.Parallel()
	clean, calls := parseToolCalls("hello\n```tool_call\n{\"name\":\"lookup\",\"arguments\":{\"id\":7}}\n```")
	if clean != "hello" || len(calls) != 1 || calls[0].Function.Name != "lookup" || calls[0].Function.Arguments != `{"id":7}` {
		t.Fatalf("unexpected parse clean=%q calls=%+v", clean, calls)
	}
}
