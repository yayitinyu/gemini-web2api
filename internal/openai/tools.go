package openai

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
)

const (
	toolFenceOpen  = "```tool_call"
	toolFenceClose = "```"
)

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

var toolCallPattern = regexp.MustCompile("(?s)```tool_call\\s*\\n(.*?)\\n```")

func parseToolCalls(text string) (string, []ToolCall) {
	calls := make([]ToolCall, 0)
	for _, match := range toolCallPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		var payload struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if json.Unmarshal([]byte(strings.TrimSpace(match[1])), &payload) != nil || payload.Name == "" {
			continue
		}
		arguments := strings.TrimSpace(string(payload.Arguments))
		if arguments == "" || arguments == "null" {
			arguments = "{}"
		}
		if !json.Valid([]byte(arguments)) {
			continue
		}
		calls = append(calls, ToolCall{
			ID: "call_" + randomHex(12), Type: "function",
			Function: ToolFunction{Name: payload.Name, Arguments: arguments},
		})
	}
	cleaned := strings.TrimSpace(toolCallPattern.ReplaceAllString(text, ""))
	return cleaned, calls
}

type toolFenceGate struct {
	emit    func(string)
	buffer  string
	sent    strings.Builder
	inFence bool
}

func newToolFenceGate(emit func(string)) *toolFenceGate {
	return &toolFenceGate{emit: emit}
}

func (g *toolFenceGate) Push(delta string) {
	g.buffer += delta
	for {
		if g.inFence {
			index := strings.Index(g.buffer, toolFenceClose)
			if index < 0 {
				return
			}
			g.buffer = g.buffer[index+len(toolFenceClose):]
			g.inFence = false
			continue
		}
		if index := strings.Index(g.buffer, toolFenceOpen); index >= 0 {
			g.send(g.buffer[:index])
			g.buffer = g.buffer[index+len(toolFenceOpen):]
			g.inFence = true
			continue
		}
		keep := partialPrefixLength(g.buffer, toolFenceOpen)
		g.send(g.buffer[:len(g.buffer)-keep])
		g.buffer = g.buffer[len(g.buffer)-keep:]
		return
	}
}

func (g *toolFenceGate) Sent() string { return g.sent.String() }

func (g *toolFenceGate) send(value string) {
	if value == "" {
		return
	}
	g.sent.WriteString(value)
	g.emit(value)
}

func partialPrefixLength(value, marker string) int {
	limit := min(len(value), len(marker)-1)
	for length := limit; length > 0; length-- {
		if strings.HasPrefix(marker, value[len(value)-length:]) {
			return length
		}
	}
	return 0
}

func remainingText(full, sent string) string {
	if strings.HasPrefix(full, sent) {
		return full[len(sent):]
	}
	return ""
}

func randomHex(length int) string {
	buffer := make([]byte, (length+1)/2)
	if _, err := rand.Read(buffer); err != nil {
		return strings.Repeat("0", length)
	}
	return hex.EncodeToString(buffer)[:length]
}
