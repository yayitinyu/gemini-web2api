package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/store"
)

func (h *Handler) Overview(w http.ResponseWriter, r *http.Request) {
	hours := queryInt(r, "hours", 24, 1, 720)
	since := time.Now().Add(-time.Duration(hours) * time.Hour)
	bucket := time.Hour
	if hours > 48 {
		bucket = 6 * time.Hour
	}
	if hours > 24*14 {
		bucket = 24 * time.Hour
	}
	stats, err := h.store.Overview(r.Context(), since)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	series, err := h.store.TimeSeries(r.Context(), since, bucket)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	recent, err := h.store.Requests(r.Context(), store.RequestFilter{Limit: 8})
	if err != nil {
		handleStoreError(w, err)
		return
	}
	accounts, err := h.store.ListAccounts(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"stats": stats, "timeseries": series, "recent": recent.Items,
		"accounts": accounts, "api_key": h.apiKeys.State(), "range_hours": hours,
	})
}

func (h *Handler) Requests(w http.ResponseWriter, r *http.Request) {
	filter := store.RequestFilter{
		Limit: queryInt(r, "limit", 50, 1, 200), Offset: queryInt(r, "offset", 0, 0, 1_000_000),
		Model: r.URL.Query().Get("model"), Status: r.URL.Query().Get("status"),
	}
	page, err := h.store.Requests(r.Context(), filter)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

func queryInt(r *http.Request, key string, fallback, minimum, maximum int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil {
		return fallback
	}
	if value < minimum {
		return minimum
	}
	if value > maximum {
		return maximum
	}
	return value
}
