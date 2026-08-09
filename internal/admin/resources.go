package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/yayitinyu/gemini-web2api/internal/gateway"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

func (h *Handler) Accounts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		accounts, err := h.store.ListAccounts(r.Context())
		if err != nil {
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": accounts})
		return
	}
	var input store.AccountInput
	if !decodeJSON(w, r, 2<<20, &input) {
		return
	}
	account, err := h.store.CreateAccount(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_account", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, account)
}

func (h *Handler) Account(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input store.AccountUpdate
		if !decodeJSON(w, r, 2<<20, &input) {
			return
		}
		account, err := h.store.UpdateAccount(r.Context(), id, input)
		if err != nil {
			resourceError(w, err, "account")
			return
		}
		writeJSON(w, http.StatusOK, account)
	case http.MethodDelete:
		if err := h.store.DeleteAccount(r.Context(), id); err != nil {
			resourceError(w, err, "account")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的操作")
	}
}

func (h *Handler) TestAccount(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	h.probe(w, r, gateway.ProbeInput{AccountID: id, Model: "gemini-3.6-flash"})
}

func (h *Handler) Proxies(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		proxies, err := h.store.ListProxies(r.Context())
		if err != nil {
			handleStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": proxies})
		return
	}
	var input store.ProxyInput
	if !decodeJSON(w, r, 64<<10, &input) {
		return
	}
	proxy, err := h.store.CreateProxy(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_proxy", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, proxy)
}

func (h *Handler) Proxy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPut:
		var input store.ProxyUpdate
		if !decodeJSON(w, r, 64<<10, &input) {
			return
		}
		proxy, err := h.store.UpdateProxy(r.Context(), id, input)
		if err != nil {
			resourceError(w, err, "proxy")
			return
		}
		writeJSON(w, http.StatusOK, proxy)
	case http.MethodDelete:
		if err := h.store.DeleteProxy(r.Context(), id); err != nil {
			resourceError(w, err, "proxy")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "不支持的操作")
	}
}

func (h *Handler) ResetProxy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.store.ResetProxy(r.Context(), id); err != nil {
		resourceError(w, err, "proxy")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) TestProxy(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	h.probe(w, r, gateway.ProbeInput{ProxyID: id, Model: "gemini-3.6-flash"})
}

func (h *Handler) Probe(w http.ResponseWriter, r *http.Request) {
	var input gateway.ProbeInput
	if r.ContentLength != 0 && !decodeJSON(w, r, 32<<10, &input) {
		return
	}
	if input.AccountID == 0 && input.ProxyID == 0 {
		execution, err := h.gateway.Generate(r.Context(), gateway.GenerateInput{Model: input.Model, Prompt: "Reply with exactly: OK"})
		writeProbeResult(w, execution, err)
		return
	}
	h.probe(w, r, input)
}

func (h *Handler) probe(w http.ResponseWriter, r *http.Request, input gateway.ProbeInput) {
	execution, err := h.gateway.Probe(r.Context(), input)
	writeProbeResult(w, execution, err)
}

func writeProbeResult(w http.ResponseWriter, execution gateway.Execution, err error) {
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"ok": false, "error": err.Error(), "model": execution.RequestedModel,
			"latency_ms": execution.Result.Total.Milliseconds(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "model": execution.RequestedModel, "upstream_model": execution.Result.UpstreamModel,
		"latency_ms": execution.Result.Total.Milliseconds(), "ttfb_ms": execution.Result.TTFB.Milliseconds(),
	})
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_id", "资源 ID 无效")
		return 0, false
	}
	return id, true
}

func resourceError(w http.ResponseWriter, err error, kind string) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", kind+" not found")
		return
	}
	writeError(w, http.StatusBadRequest, "invalid_"+kind, err.Error())
}
