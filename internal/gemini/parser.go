package gemini

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"time"
)

const maxFrameBytes = 8 << 20

var (
	artifactPattern      = regexp.MustCompile("(?s)```(?:python|javascript|text)\\?code_(?:reference|stdout)&code_event_index=\\d+\\n.*?```\\n?")
	upstreamModelPattern = regexp.MustCompile(`\\"[0-9a-f]{16}\\",null,null,\\"([^"\\]{1,64})\\"`)
)

type Frame struct {
	Text      string
	Reasoning string
}

type DeltaTracker struct {
	emitted string
}

func (t *DeltaTracker) Push(cumulative string) string {
	cumulative = CleanText(cumulative)
	if len(cumulative) <= len(t.emitted) || !strings.HasPrefix(cumulative, t.emitted) {
		return ""
	}
	delta := cumulative[len(t.emitted):]
	t.emitted = cumulative
	return delta
}

func (t *DeltaTracker) Emitted() string { return t.emitted }

func ParseFrame(line string) Frame {
	if !strings.Contains(line, `"wrb.fr"`) {
		return Frame{}
	}
	var envelope []any
	if json.Unmarshal([]byte(line), &envelope) != nil || len(envelope) == 0 {
		return Frame{}
	}
	entry, ok := envelope[0].([]any)
	if !ok || len(entry) < 3 {
		return Frame{}
	}
	innerJSON, ok := entry[2].(string)
	if !ok || innerJSON == "" {
		return Frame{}
	}
	var inner []any
	if json.Unmarshal([]byte(innerJSON), &inner) != nil || len(inner) <= 4 {
		return Frame{}
	}
	parts, ok := inner[4].([]any)
	if !ok {
		return Frame{}
	}
	frame := Frame{}
	for index, rawPart := range parts {
		part, ok := rawPart.([]any)
		if !ok {
			continue
		}
		if len(part) > 1 {
			if texts, ok := part[1].([]any); ok {
				for _, rawText := range texts {
					if text, ok := rawText.(string); ok && len(text) > len(frame.Text) {
						frame.Text = text
					}
				}
			}
		}
		if index == 0 && len(part) > 37 {
			frame.Reasoning = nestedString(part[37])
		}
	}
	return frame
}

func nestedString(value any) string {
	first, ok := value.([]any)
	if !ok || len(first) == 0 {
		return ""
	}
	second, ok := first[0].([]any)
	if !ok || len(second) == 0 {
		return ""
	}
	text, _ := second[0].(string)
	return text
}

type scanResult struct {
	raw              string
	text             string
	reasoning        string
	upstreamModel    string
	firstFrame       time.Duration
	contentFrameSeen bool
}

func scanFrames(body io.Reader, started time.Time, onFrame func(Frame)) (scanResult, error) {
	var raw bytes.Buffer
	scanner := bufio.NewScanner(io.TeeReader(body, &raw))
	scanner.Buffer(make([]byte, 64<<10), maxFrameBytes)
	result := scanResult{firstFrame: -1}
	for scanner.Scan() {
		if result.firstFrame < 0 {
			result.firstFrame = time.Since(started)
		}
		line := scanner.Text()
		frame := ParseFrame(line)
		if frame.Text != "" || frame.Reasoning != "" {
			result.contentFrameSeen = true
			if len(frame.Text) >= len(result.text) {
				result.text = frame.Text
			}
			if len(frame.Reasoning) >= len(result.reasoning) {
				result.reasoning = frame.Reasoning
			}
			if onFrame != nil {
				onFrame(frame)
			}
		}
	}
	if result.firstFrame < 0 {
		result.firstFrame = time.Since(started)
	}
	result.raw = raw.String()
	if matches := upstreamModelPattern.FindAllStringSubmatch(result.raw, -1); len(matches) > 0 {
		result.upstreamModel = matches[len(matches)-1][1]
	}
	result.text = CleanText(result.text)
	result.reasoning = CleanText(result.reasoning)
	return result, scanner.Err()
}

func CleanText(value string) string {
	return strings.TrimSpace(artifactPattern.ReplaceAllString(value, ""))
}
