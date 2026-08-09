package gemini

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
)

func BuildPayload(prompt string, model Model, xsrf string) (string, error) {
	inner := make([]any, 80)
	inner[0] = []any{prompt, 0, nil, nil, nil, nil, 0}
	inner[1] = []any{"en"}
	inner[2] = []any{"", "", "", nil, nil, nil, nil, nil, nil, ""}
	inner[6] = []any{0}
	inner[7] = 1
	inner[10] = 1
	inner[11] = 0
	inner[17] = []any{[]any{0}}
	inner[18] = 0
	inner[27] = 1
	inner[30] = []any{4}
	inner[41] = []any{1}
	inner[53] = 0
	inner[59] = randomUUID()
	inner[61] = []any{}
	inner[68] = 1
	inner[79] = model.Mode

	innerJSON, err := json.Marshal(inner)
	if err != nil {
		return "", fmt.Errorf("encode Gemini inner payload: %w", err)
	}
	outerJSON, err := json.Marshal([]any{nil, string(innerJSON)})
	if err != nil {
		return "", fmt.Errorf("encode Gemini outer payload: %w", err)
	}
	form := url.Values{"f.req": []string{string(outerJSON)}}
	if xsrf != "" {
		form.Set("at", xsrf)
	}
	return form.Encode(), nil
}

func randomUUID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	buffer[6] = (buffer[6] & 0x0f) | 0x40
	buffer[8] = (buffer[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(buffer)
	return encoded[:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:]
}
