package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yayitinyu/gemini-web2api/internal/cryptox"
	"github.com/yayitinyu/gemini-web2api/internal/gemini"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

func TestGenerateUsesAnonymousAndRejectsOversizedPrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cipher, _ := cryptox.Load(dir, "gateway-test-key")
	st, err := store.Open(filepath.Join(dir, "gateway.db"), cipher)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	settings := store.DefaultRuntimeSettings()
	settings.GeminiBLAuto = false
	settings.MaxPromptBytes = 8_192
	if err := st.SaveRuntimeSettings(context.Background(), settings); err != nil {
		t.Fatal(err)
	}
	sender := &staticSender{body: frame(t, "hello")}
	service := New(st, gemini.NewClient(sender, "https://gemini.example"))
	execution, err := service.Generate(context.Background(), GenerateInput{Model: "gemini-3.7-flash", Prompt: "hi"})
	if err != nil || execution.Result.Text != "hello" || execution.Account != nil {
		t.Fatalf("unexpected execution=%+v err=%v", execution, err)
	}
	_, err = service.Generate(context.Background(), GenerateInput{Model: "gemini-3.7-flash", Prompt: strings.Repeat("x", 8_193)})
	var contextErr *ContextLengthError
	if !errors.As(err, &contextErr) {
		t.Fatalf("expected ContextLengthError, got %v", err)
	}
}

func frame(t *testing.T, text string) string {
	t.Helper()
	part := make([]any, 2)
	part[1] = []any{text}
	inner := make([]any, 5)
	inner[4] = []any{part}
	innerJSON, _ := json.Marshal(inner)
	envelope, _ := json.Marshal([]any{[]any{"wrb.fr", nil, string(innerJSON)}})
	return string(envelope) + "\n"
}

type staticSender struct{ body string }

func (s *staticSender) Send(_ context.Context, request gemini.OutboundRequest) (*gemini.OutboundResponse, error) {
	return &gemini.OutboundResponse{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(s.body))}, nil
}
