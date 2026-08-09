package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/yayitinyu/gemini-web2api/internal/admin"
	"github.com/yayitinyu/gemini-web2api/internal/auth"
	"github.com/yayitinyu/gemini-web2api/internal/gateway"
	"github.com/yayitinyu/gemini-web2api/internal/openai"
)

type Dependencies struct {
	Version string
	Gateway *gateway.Service
	OpenAI  *openai.Handler
	Admin   *admin.Handler
	APIKeys *auth.APIKeyManager
	WebFS   fs.FS
}

func New(deps Dependencies) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.Recoverer)
	router.Use(securityHeaders)

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		models, err := deps.Gateway.Models(r.Context())
		if err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "version": deps.Version})
			return
		}
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": deps.Version, "models": ids})
	})
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": deps.Version})
	})

	router.Route("/v1", func(api chi.Router) {
		api.Use(openAICORS)
		api.Options("/*", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		api.Group(func(protected chi.Router) {
			protected.Use(requireAPIKey(deps.APIKeys))
			protected.Get("/models", deps.OpenAI.Models)
			protected.Post("/chat/completions", deps.OpenAI.ChatCompletions)
			protected.Post("/responses", deps.OpenAI.Responses)
		})
	})

	router.Route("/api/admin", func(api chi.Router) {
		api.Post("/auth/login", deps.Admin.Login)
		api.Group(func(protected chi.Router) {
			protected.Use(deps.Admin.Protected)
			protected.Get("/auth/session", deps.Admin.Session)
			protected.Post("/auth/logout", deps.Admin.Logout)
			protected.Get("/overview", deps.Admin.Overview)
			protected.Get("/requests", deps.Admin.Requests)
			protected.MethodFunc(http.MethodGet, "/accounts", deps.Admin.Accounts)
			protected.MethodFunc(http.MethodPost, "/accounts", deps.Admin.Accounts)
			protected.MethodFunc(http.MethodPut, "/accounts/{id}", deps.Admin.Account)
			protected.MethodFunc(http.MethodDelete, "/accounts/{id}", deps.Admin.Account)
			protected.Post("/accounts/{id}/test", deps.Admin.TestAccount)
			protected.MethodFunc(http.MethodGet, "/proxies", deps.Admin.Proxies)
			protected.MethodFunc(http.MethodPost, "/proxies", deps.Admin.Proxies)
			protected.MethodFunc(http.MethodPut, "/proxies/{id}", deps.Admin.Proxy)
			protected.MethodFunc(http.MethodDelete, "/proxies/{id}", deps.Admin.Proxy)
			protected.Post("/proxies/{id}/reset", deps.Admin.ResetProxy)
			protected.Post("/proxies/{id}/test", deps.Admin.TestProxy)
			protected.MethodFunc(http.MethodGet, "/settings", deps.Admin.Settings)
			protected.MethodFunc(http.MethodPut, "/settings", deps.Admin.Settings)
			protected.MethodFunc(http.MethodGet, "/api-key", deps.Admin.APIKey)
			protected.MethodFunc(http.MethodPost, "/api-key", deps.Admin.APIKey)
			protected.Post("/probe", deps.Admin.Probe)
		})
	})

	spa := NewSPA(deps.WebFS)
	router.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	router.Handle("/admin/*", spa)
	return router
}

func requireAPIKey(keys *auth.APIKeyManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			value := strings.TrimSpace(r.Header.Get("Authorization"))
			if scheme, token, ok := strings.Cut(value, " "); ok && strings.EqualFold(scheme, "Bearer") {
				value = strings.TrimSpace(token)
			} else if header := strings.TrimSpace(r.Header.Get("X-API-Key")); header != "" {
				value = header
			}
			if !keys.Authenticate(value) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": map[string]any{
					"message": "Invalid API key", "type": "authentication_error", "param": nil, "code": "invalid_api_key",
				}})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func openAICORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(r.URL.Path, "/admin") {
			w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type spaHandler struct {
	root  fs.FS
	index []byte
}

func NewSPA(root fs.FS) http.Handler {
	index, _ := fs.ReadFile(root, "index.html")
	return &spaHandler{root: root, index: index}
}

func (s *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requested := strings.TrimPrefix(r.URL.Path, "/admin/")
	requested = strings.TrimPrefix(path.Clean("/"+requested), "/")
	if requested != "." && requested != "" {
		if info, err := fs.Stat(s.root, requested); err == nil && !info.IsDir() {
			if strings.HasPrefix(requested, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.FileServer(http.FS(s.root)).ServeHTTP(w, rWithPath(r, "/"+requested))
			return
		}
	}
	if len(s.index) == 0 {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "admin UI has not been built"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeContent(w, r, "index.html", time.Time{}, strings.NewReader(string(s.index)))
}

func rWithPath(r *http.Request, value string) *http.Request {
	clone := r.Clone(r.Context())
	clone.URL.Path = value
	return clone
}

var _ http.Handler = (*spaHandler)(nil)
