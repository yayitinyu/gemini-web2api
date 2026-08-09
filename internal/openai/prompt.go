package openai

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type promptResult struct {
	Prompt      string
	Unsupported string
}

func messagesToPrompt(messages []any, tools []any, toolChoice any) (promptResult, error) {
	sections := make([]string, 0, len(messages)+1)
	toolSection, err := buildToolSection(tools, toolChoice)
	if err != nil {
		return promptResult{}, err
	}
	if toolSection != "" {
		sections = append(sections, toolSection)
	}
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		role, _ := message["role"].(string)
		content, unsupported := contentText(message["content"])
		if unsupported != "" {
			return promptResult{Unsupported: unsupported}, nil
		}
		role = strings.ToLower(strings.TrimSpace(role))
		switch role {
		case "system", "developer":
			if content != "" {
				sections = append(sections, "[System instruction]\n"+content)
			}
		case "assistant":
			block := "[Assistant]"
			if content != "" {
				block += "\n" + content
			}
			if calls, ok := message["tool_calls"].([]any); ok {
				for _, call := range calls {
					if serialized := serializePriorToolCall(call); serialized != "" {
						block += "\n" + serialized
					}
				}
			}
			sections = append(sections, block)
		case "tool":
			name, _ := message["name"].(string)
			if name == "" {
				name, _ = message["tool_call_id"].(string)
			}
			sections = append(sections, fmt.Sprintf("[Tool result: %s]\n%s", name, content))
		default:
			if content != "" {
				sections = append(sections, "[User]\n"+content)
			}
		}
	}
	return promptResult{Prompt: strings.TrimSpace(strings.Join(sections, "\n\n"))}, nil
}

func responsesInputToMessages(instructions string, input any) ([]any, error) {
	messages := make([]any, 0)
	if strings.TrimSpace(instructions) != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
	}
	switch value := input.(type) {
	case string:
		messages = append(messages, map[string]any{"role": "user", "content": value})
	case []any:
		for _, raw := range value {
			switch item := raw.(type) {
			case string:
				messages = append(messages, map[string]any{"role": "user", "content": item})
			case map[string]any:
				typeName, _ := item["type"].(string)
				if typeName == "function_call_output" {
					messages = append(messages, map[string]any{
						"role": "tool", "tool_call_id": stringValue(item["call_id"]),
						"name": stringValue(item["name"]), "content": stringValue(item["output"]),
					})
					continue
				}
				role := stringValue(item["role"])
				if role == "" {
					role = "user"
				}
				messages = append(messages, map[string]any{"role": role, "content": item["content"], "tool_calls": item["tool_calls"]})
			default:
				return nil, errors.New("input array contains an unsupported item")
			}
		}
	case nil:
		return nil, errors.New("input is required")
	default:
		return nil, errors.New("input must be a string or an array")
	}
	return messages, nil
}

func contentText(content any) (string, string) {
	switch value := content.(type) {
	case nil:
		return "", ""
	case string:
		return value, ""
	case []any:
		parts := make([]string, 0, len(value))
		for _, rawPart := range value {
			part, ok := rawPart.(map[string]any)
			if !ok {
				continue
			}
			typeName := stringValue(part["type"])
			switch typeName {
			case "text", "input_text", "output_text":
				parts = append(parts, stringValue(part["text"]))
			case "image_url", "input_image":
				return "", "image input is not implemented in this release"
			case "input_audio":
				return "", "audio input is not supported"
			}
		}
		return strings.Join(parts, "\n"), ""
	default:
		return "", ""
	}
}

func buildToolSection(tools []any, toolChoice any) (string, error) {
	if len(tools) == 0 {
		return "", nil
	}
	mode, forced := parseToolChoice(toolChoice)
	if mode == "none" {
		return "", nil
	}
	definitions := make([]map[string]any, 0, len(tools))
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName := stringValue(tool["type"])
		if typeName != "" && typeName != "function" {
			return "", fmt.Errorf("tool type %q is not supported; only function tools are compatible", typeName)
		}
		function := tool
		if nested, ok := tool["function"].(map[string]any); ok {
			function = nested
		}
		name := stringValue(function["name"])
		if name == "" || (forced != "" && forced != name) {
			continue
		}
		definitions = append(definitions, map[string]any{
			"name": name, "description": stringValue(function["description"]), "parameters": function["parameters"],
		})
	}
	if forced != "" && len(definitions) == 0 {
		return "", fmt.Errorf("tool_choice requested unknown function %q", forced)
	}
	if len(definitions) == 0 {
		return "", nil
	}
	encoded, err := json.MarshalIndent(definitions, "", "  ")
	if err != nil {
		return "", err
	}
	rule := "Use a tool only when it is needed. Otherwise answer normally."
	if mode == "required" {
		rule = "You must call exactly one listed tool and must not answer the request yourself."
	}
	if forced != "" {
		rule = fmt.Sprintf("You must call %q and must not answer the request yourself.", forced)
	}
	return "[Tool protocol]\n" +
		"To call a tool, output exactly one fenced block in this form and no prose inside it:\n" +
		"```tool_call\n{\"name\":\"function_name\",\"arguments\":{}}\n```\n" + rule +
		"\nAvailable functions:\n" + string(encoded), nil
}

func parseToolChoice(value any) (mode, forced string) {
	if text, ok := value.(string); ok {
		switch text {
		case "none", "required", "auto":
			return text, ""
		}
	}
	if object, ok := value.(map[string]any); ok {
		if function, ok := object["function"].(map[string]any); ok {
			if name := stringValue(function["name"]); name != "" {
				return "required", name
			}
		}
	}
	return "auto", ""
}

func serializePriorToolCall(raw any) string {
	call, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	function, _ := call["function"].(map[string]any)
	name := stringValue(function["name"])
	arguments := stringValue(function["arguments"])
	if name == "" {
		return ""
	}
	if arguments == "" {
		arguments = "{}"
	}
	return "```tool_call\n{\"name\":" + mustJSONString(name) + ",\"arguments\":" + arguments + "}\n```"
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func mustJSONString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
