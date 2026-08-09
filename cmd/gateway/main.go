package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yayitinyu/gemini-web2api/internal/admin"
	"github.com/yayitinyu/gemini-web2api/internal/auth"
	"github.com/yayitinyu/gemini-web2api/internal/config"
	"github.com/yayitinyu/gemini-web2api/internal/cryptox"
	"github.com/yayitinyu/gemini-web2api/internal/gateway"
	"github.com/yayitinyu/gemini-web2api/internal/gemini"
	"github.com/yayitinyu/gemini-web2api/internal/openai"
	appserver "github.com/yayitinyu/gemini-web2api/internal/server"
	"github.com/yayitinyu/gemini-web2api/internal/store"
	"github.com/yayitinyu/gemini-web2api/webui"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	switch command {
	case "serve":
		if err := serve(); err != nil {
			slog.Error("gateway stopped", "error", err)
			os.Exit(1)
		}
	case "healthcheck":
		url := "http://127.0.0.1:8080/healthz"
		if len(os.Args) > 2 {
			url = os.Args[2]
		}
		if err := healthcheck(url); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "version", "--version", "-v":
		fmt.Printf("gemini-web2api %s (%s, %s)\n", version, commit, buildDate)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q; use serve, healthcheck, or version\n", command)
		os.Exit(2)
	}
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cipher, err := cryptox.Load(cfg.DataDir, cfg.EncryptionKey)
	if err != nil {
		return err
	}
	st, err := store.Open(cfg.DatabasePath, cipher)
	if err != nil {
		return err
	}
	defer st.Close()
	apiKeys, generatedKey, err := auth.NewAPIKeyManager(context.Background(), st, cfg.APIKey)
	if err != nil {
		return err
	}
	sessions, err := auth.NewSessionManager(st, cfg.AdminPassword, cfg.SessionTTL, cfg.CookieSecure)
	if err != nil {
		return err
	}
	// ADMIN_PASSWORD changes require a restart. Revoking sessions here ensures
	// credentials from before that boundary cannot remain authenticated.
	if err := st.DeleteAllSessions(context.Background()); err != nil {
		return fmt.Errorf("revoke existing admin sessions: %w", err)
	}
	transport, err := gemini.NewBrowserTransport()
	if err != nil {
		return err
	}
	defer transport.CloseIdleConnections()
	geminiClient := gemini.NewClient(transport, cfg.UpstreamBaseURL)
	service := gateway.New(st, geminiClient)
	openAIHandler := openai.NewHandler(service, st, cfg.MaxRequestBodyBytes)
	adminHandler := admin.NewHandler(st, service, sessions, apiKeys, cfg.TrustProxyHeaders)
	webRoot, err := fs.Sub(webui.Files, "dist")
	if err != nil {
		return fmt.Errorf("load embedded admin UI: %w", err)
	}
	router := appserver.New(appserver.Dependencies{
		Version: version, Gateway: service, OpenAI: openAIHandler, Admin: adminHandler, APIKeys: apiKeys, WebFS: webRoot,
	})
	httpServer := &http.Server{
		Addr: cfg.Address, Handler: router, ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout,
	}

	if generatedKey != "" {
		slog.Warn("generated initial API key; copy it now because only its hash is stored", "api_key", generatedKey)
	}
	if !cfg.CookieSecure && !isLoopbackAddress(cfg.Address) {
		slog.Warn("admin cookie is not marked Secure; set COOKIE_SECURE=true behind HTTPS")
	}
	slog.Info("Gemini Web2API started", "address", cfg.Address, "version", version, "admin", "/admin/")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go maintenance(ctx, st)
	serverErr := make(chan error, 1)
	go func() { serverErr <- httpServer.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	case err := <-serverErr:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func maintenance(ctx context.Context, st *store.Store) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = st.PurgeSessions(context.WithoutCancel(ctx))
			if settings, err := st.RuntimeSettings(context.WithoutCancel(ctx)); err == nil {
				_ = st.PurgeExpired(context.WithoutCancel(ctx), settings.RetentionDays)
			}
		}
	}
}

func healthcheck(url string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("healthcheck request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", response.StatusCode)
	}
	return nil
}

func isLoopbackAddress(address string) bool {
	return strings.HasPrefix(address, "127.0.0.1:") || strings.HasPrefix(address, "localhost:")
}
