package gemini

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBuildPayloadUsesBrowserVerifiedSlots(t *testing.T) {
	t.Parallel()
	model, err := ResolveModel("gemini-3.6-flash", false)
	if err != nil {
		t.Fatal(err)
	}
	body, err := BuildPayload("hello", model, "xsrf-token-value")
	if err != nil {
		t.Fatal(err)
	}
	form, err := url.ParseQuery(body)
	if err != nil {
		t.Fatal(err)
	}
	if form.Get("at") != "xsrf-token-value" {
		t.Fatalf("missing XSRF token: %q", form.Get("at"))
	}
	var outer []any
	if err := json.Unmarshal([]byte(form.Get("f.req")), &outer); err != nil {
		t.Fatal(err)
	}
	var inner []any
	if err := json.Unmarshal([]byte(outer[1].(string)), &inner); err != nil {
		t.Fatal(err)
	}
	if len(inner) != 80 || inner[79].(float64) != float64(model.Mode) {
		t.Fatalf("unexpected payload shape len=%d mode=%v", len(inner), inner[79])
	}
	if inner[41].([]any)[0].(float64) != 1 {
		t.Fatalf("browser-verified slot 41 drifted: %#v", inner[41])
	}
}

func TestParseFrameAndCumulativeDelta(t *testing.T) {
	t.Parallel()
	line1 := testFrame(t, "你", "思")
	line2 := testFrame(t, "你好", "思考")
	var deltas []string
	tracker := &DeltaTracker{}
	result, err := scanFrames(strings.NewReader(line1+"\n"+line2+"\n"), time.Now(), func(frame Frame) {
		if delta := tracker.Push(frame.Text); delta != "" {
			deltas = append(deltas, delta)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.text != "你好" || result.reasoning != "思考" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if strings.Join(deltas, "|") != "你|好" {
		t.Fatalf("unexpected deltas: %#v", deltas)
	}
}

func TestClientRetriesOnlyBeforeEmission(t *testing.T) {
	t.Parallel()
	model, _ := ResolveModel("gemini-3.6-flash", false)
	sender := &fakeSender{responses: []*OutboundResponse{
		response(200, "[[\"wrb.fr\",null,\"[]\"]]\n"),
		response(200, testFrame(t, "done", "")+"\n"),
	}}
	client := NewClient(sender, "https://gemini.example")
	var emitted strings.Builder
	result, err := client.Generate(context.Background(), GenerateRequest{
		Prompt: "hello", Model: model,
		Options:   Options{Timeout: time.Second, RetryAttempts: 2, PinnedBL: "boq_assistant-bard-web-server_20260805.16_p0"},
		OnContent: func(delta string) { _, _ = emitted.WriteString(delta) },
	})
	if err != nil {
		t.Fatal(err)
	}
	if sender.calls != 2 || result.Text != "done" || emitted.String() != "done" {
		t.Fatalf("unexpected retry result calls=%d result=%+v emitted=%q", sender.calls, result, emitted.String())
	}
}

func TestCookieMergeKeepsUnchangedValues(t *testing.T) {
	t.Parallel()
	merged := MergeSetCookies("SID=one; SIDCC=old", []string{"SIDCC=new; Path=/; Secure", "NEW=value; Path=/"})
	if merged != "SID=one; SIDCC=new; NEW=value" {
		t.Fatalf("unexpected merged cookie: %q", merged)
	}
}

func testFrame(t *testing.T, text, reasoning string) string {
	t.Helper()
	part := make([]any, 38)
	part[1] = []any{text}
	if reasoning != "" {
		part[37] = []any{[]any{reasoning}}
	}
	inner := make([]any, 43)
	inner[4] = []any{part}
	innerJSON, err := json.Marshal(inner)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := json.Marshal([]any{[]any{"wrb.fr", nil, string(innerJSON)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(envelope)
}

func response(status int, body string) *OutboundResponse {
	return &OutboundResponse{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}

type fakeSender struct {
	mu        sync.Mutex
	responses []*OutboundResponse
	calls     int
}

func (s *fakeSender) Send(_ context.Context, request OutboundRequest) (*OutboundResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if request.Method == http.MethodGet {
		return response(200, `{"cfb2h":"boq_assistant-bard-web-server_20260805.16_p0"}`), nil
	}
	index := s.calls - 1
	if index >= len(s.responses) {
		index = len(s.responses) - 1
	}
	return s.responses[index], nil
}
