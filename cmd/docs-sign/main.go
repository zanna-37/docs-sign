// Command docs-sign is a self-hosted, zero-knowledge PDF signing server. It serves a
// React SPA (embedded in this binary) and a JSON API, storing all user content encrypted
// on disk with per-user keys that exist in plaintext only in memory during a session.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"docs-sign/internal/api"
	"docs-sign/internal/auth"
	"docs-sign/internal/blob"
	"docs-sign/internal/config"
	"docs-sign/internal/pdfproc"
	"docs-sign/internal/session"
	"docs-sign/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("docs-sign: %v", err)
	}
}

func run() error {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return err
	}
	blobs, err := blob.New(filepath.Join(cfg.DataDir, "blobs"))
	if err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(cfg.DataDir, "meta.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	pdf, err := pdfproc.New()
	if err != nil {
		return err
	}
	defer pdf.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	sessions := session.NewManager(cfg.IdleTimeout, cfg.SessionTimeout)
	sessions.StartJanitor(ctx)

	authSvc := auth.NewService(st, blobs, sessions, cfg.KDF)
	srv := api.NewServer(cfg, st, blobs, sessions, authSvc, pdf)
	srv.StartTrashJanitor(ctx)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if need, err := authSvc.NeedsSetup(ctx); err == nil && need {
		log.Printf("first run: open http://%s/ to create the admin account", cfg.Addr)
	}
	log.Printf("listening on %s (serve behind a TLS-terminating reverse proxy)", cfg.Addr)
	if cfg.Dev {
		log.Printf("dev mode: cookies are not marked Secure")
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Printf("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}
