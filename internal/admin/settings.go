package admin

import (
	"net/http"
	"strings"

	"github.com/yayitinyu/gemini-web2api/internal/gemini"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

func (h *Handler) Settings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		settings, err := h.store.RuntimeSettings(r.Context())
		if err != nil {
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"settings": settings, "available_models": gemini.ModelIDs(),
			"password_source": "ADMIN_PASSWORD environment variable",
		})
		return
	}
	var settings store.RuntimeSettings
	if !decodeJSON(w, r, 64<<10, &settings) {
		return
	}
	if err := settings.Validate(gemini.ModelIDs()); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_settings", err.Error())
		return
	}
	if err := h.store.SaveRuntimeSettings(r.Context(), settings); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

func (h *Handler) APIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, h.apiKeys.State())
		return
	}
	var request struct {
		Confirm string `json:"confirm"`
	}
	if !decodeJSON(w, r, 8<<10, &request) {
		return
	}
	if strings.ToUpper(strings.TrimSpace(request.Confirm)) != "ROTATE" {
		writeError(w, http.StatusBadRequest, "confirmation_required", "请输入 ROTATE 确认轮换")
		return
	}
	key, err := h.apiKeys.Rotate(r.Context())
	if err != nil {
		writeError(w, http.StatusConflict, "api_key_locked", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"key": key, "hint": h.apiKeys.State().Hint,
		"notice": "此密钥只显示一次，请立即复制并安全保存。",
	})
}
