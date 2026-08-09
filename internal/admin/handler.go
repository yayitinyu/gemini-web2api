package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/auth"
	"github.com/yayitinyu/gemini-web2api/internal/gateway"
	"github.com/yayitinyu/gemini-web2api/internal/store"
)

type Handler struct {
	store             *store.Store
	gateway           *gateway.Service
	sessions          *auth.SessionManager
	apiKeys           *auth.APIKeyManager
	trustProxyHeaders bool
	loginLimiter      *loginLimiter
}

func NewHandler(st *store.Store, gw *gateway.Service, sessions *auth.SessionManager, apiKeys *auth.APIKeyManager, trustProxyHeaders bool) *Handler {
	return &Handler{
		store: st, gateway: gw, sessions: sessions, apiKeys: apiKeys,
		trustProxyHeaders: trustProxyHeaders, loginLimiter: newLoginLimiter(),
	}
}

type sessionContextKey struct{}

func (h *Handler) Protected(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok, err := h.sessions.Authenticate(r)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "session_error", "无法读取管理会话")
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "not_authenticated", "请先登录管理面板")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if !h.sessions.VerifyCSRF(session, r.Header.Get("X-CSRF-Token")) {
				writeError(w, http.StatusForbidden, "csrf_failed", "安全令牌无效，请刷新页面后重试")
				return
			}
		}
		ctx := context.WithValue(r.Context(), sessionContextKey{}, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ip := h.clientIP(r)
	if retryAfter, ok := h.loginLimiter.Allow(ip); !ok {
		w.Header().Set("Retry-After", retryAfter)
		writeError(w, http.StatusTooManyRequests, "login_rate_limited", "登录尝试过多，请稍后再试")
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, 16<<10, &request) || len(request.Password) > 512 {
		if len(request.Password) > 512 {
			writeError(w, http.StatusBadRequest, "invalid_password", "密码格式无效")
		}
		return
	}
	if !h.sessions.VerifyPassword(request.Password) {
		h.loginLimiter.Fail(ip)
		writeError(w, http.StatusUnauthorized, "invalid_password", "管理密码不正确")
		return
	}
	h.loginLimiter.Success(ip)
	session, err := h.sessions.Create(r.Context(), w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", "无法创建管理会话")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true, "csrf_token": session.CSRFToken, "expires_at": session.ExpiresAt,
	})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if err := h.sessions.Delete(r, w); err != nil {
		writeError(w, http.StatusInternalServerError, "session_error", "无法结束管理会话")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Session(w http.ResponseWriter, r *http.Request) {
	session, ok := r.Context().Value(sessionContextKey{}).(store.Session)
	if !ok {
		writeError(w, http.StatusUnauthorized, "not_authenticated", "请先登录管理面板")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true, "csrf_token": session.CSRFToken, "expires_at": session.ExpiresAt,
	})
}

func (h *Handler) clientIP(r *http.Request) string {
	if h.trustProxyHeaders {
		if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
			return forwarded
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func decodeJSON(w http.ResponseWriter, r *http.Request, limit int64, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "请求内容必须是有效 JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func handleStoreError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.Canceled) {
		return
	}
	writeError(w, http.StatusInternalServerError, "storage_error", "数据操作失败")
}

type loginAttempt struct {
	windowStart time.Time
	failures    int
	blockedTill time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
}

func newLoginLimiter() *loginLimiter { return &loginLimiter{attempts: make(map[string]loginAttempt)} }

func (l *loginLimiter) Allow(ip string) (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	attempt := l.attempts[ip]
	if now.Before(attempt.blockedTill) {
		seconds := int(time.Until(attempt.blockedTill).Seconds()) + 1
		return strconv.Itoa(seconds), false
	}
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) > 5*time.Minute {
		delete(l.attempts, ip)
	}
	return "", true
}

func (l *loginLimiter) Fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	attempt := l.attempts[ip]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) > 5*time.Minute {
		attempt = loginAttempt{windowStart: now}
	}
	attempt.failures++
	if attempt.failures >= 5 {
		attempt.blockedTill = now.Add(5 * time.Minute)
	}
	l.attempts[ip] = attempt
}

func (l *loginLimiter) Success(ip string) {
	l.mu.Lock()
	delete(l.attempts, ip)
	l.mu.Unlock()
}
