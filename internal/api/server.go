// Package api wires the HTTP layer: JSON endpoints for auth, signatures, documents,
// exports and admin, plus serving the embedded SPA. Decryption keys are read from the
// in-memory session and zeroized after each request that uses them.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"docs-sign/internal/auth"
	"docs-sign/internal/blob"
	"docs-sign/internal/config"
	"docs-sign/internal/pdfproc"
	"docs-sign/internal/session"
	"docs-sign/internal/store"
	"docs-sign/internal/web"
)

const cookieName = "ds_session"

// Server holds the dependencies shared by all handlers.
type Server struct {
	cfg      *config.Config
	store    *store.Store
	blobs    *blob.Store
	sessions *session.Manager
	auth     *auth.Service
	pdf      *pdfproc.Renderer
}

// NewServer constructs a Server.
func NewServer(cfg *config.Config, st *store.Store, blobs *blob.Store, sessions *session.Manager, authSvc *auth.Service, pdf *pdfproc.Renderer) *Server {
	return &Server{cfg: cfg, store: st, blobs: blobs, sessions: sessions, auth: authSvc, pdf: pdf}
}

// StartTrashJanitor periodically purges trashed items older than the retention window,
// deleting their encrypted blobs. It runs once immediately, then hourly until ctx is done.
func (s *Server) StartTrashJanitor(ctx context.Context) {
	go func() {
		s.purgeTrash(ctx)
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.purgeTrash(ctx)
			}
		}
	}()
}

func (s *Server) purgeTrash(ctx context.Context) {
	cutoff := time.Now().Add(-s.cfg.TrashRetention)
	paths, err := s.store.PurgeExpired(ctx, cutoff)
	if err != nil {
		log.Printf("trash purge: %v", err)
		return
	}
	for _, p := range paths {
		_ = s.blobs.Delete(p)
	}
	if len(paths) > 0 {
		log.Printf("trash purge: removed %d expired item blob(s)", len(paths))
	}
}

type ctxKey int

const sessionCtxKey ctxKey = iota

func sessionFrom(ctx context.Context) *session.Session {
	s, _ := ctx.Value(sessionCtxKey).(*session.Session)
	return s
}

// Router returns the configured HTTP handler.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Route("/api", func(r chi.Router) {
		// Decrypted content and API data must never be written to any cache (disk or
		// shared). Static SPA assets, served below, are deliberately *not* no-store so
		// the app loads fast — they contain no secrets.
		r.Use(noStore)
		r.Use(csrfGuard)

		r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		})
		r.Get("/version", s.handleVersion)
		r.Get("/setup/status", s.handleSetupStatus)
		r.Post("/setup", s.handleSetup)
		r.Post("/login", s.handleLogin)
		r.Post("/recover", s.handleRecover)
		r.Post("/logout", s.handleLogout)

		r.Group(func(r chi.Router) {
			r.Use(s.requireAuth)
			r.Get("/me", s.handleMe)
			r.Post("/account/password", s.handleChangePassword)
			r.Post("/account/username", s.handleChangeUsername)
			r.Post("/account/language", s.handleSetLanguage)

			// Everything below requires the user to have set their own password.
			r.Group(func(r chi.Router) {
				r.Use(s.requirePasswordSet)
				r.Post("/account/recovery-code", s.handleRegenerateRecovery)

				r.Get("/signatures", s.handleListSignatures)
				r.Post("/signatures", s.handleUploadSignature)
				r.Get("/signatures/{id}/image", s.handleSignatureImage)
				r.Patch("/signatures/{id}", s.handleRenameSignature)
				r.Delete("/signatures/{id}", s.handleDeleteSignature)

				r.Get("/documents", s.handleListDocuments)
				r.Post("/documents", s.handleUploadDocument)
				r.Get("/documents/{id}/file", s.handleDocumentFile)
				r.Patch("/documents/{id}", s.handleRenameDocument)
				r.Delete("/documents/{id}", s.handleDeleteDocument)
				r.Post("/documents/{id}/sign", s.handleSignDocument)

				r.Get("/exports", s.handleListExports)
				r.Get("/exports/{id}/file", s.handleExportFile)
				r.Delete("/exports/{id}", s.handleDeleteExport)

				r.Get("/trash", s.handleListTrash)
				r.Post("/trash/empty", s.handleEmptyTrash)
				r.Post("/trash/{kind}/{id}/restore", s.handleRestoreTrash)
				r.Delete("/trash/{kind}/{id}", s.handlePurgeTrashItem)

				r.Route("/admin", func(r chi.Router) {
					r.Use(s.requireAdmin)
					r.Get("/users", s.handleListUsers)
					r.Post("/users", s.handleCreateUser)
					r.Post("/users/{id}/status", s.handleSetUserStatus)
					r.Post("/users/{id}/reset", s.handleResetUser)
					r.Delete("/users/{id}", s.handleDeleteUser)
				})
			})
		})
	})

	// Unknown /api paths are JSON 404s; everything else is the SPA.
	r.Handle("/*", web.Handler())
	return r
}

// --- middleware ---

// securityHeaders sets conservative headers on every response.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// noStore forbids caching of the response anywhere (browser disk cache, proxies, etc.).
// This is the primary control that keeps decrypted signatures and PDFs out of the
// browser's on-disk HTTP cache.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Cache-Control", "no-store, max-age=0")
		h.Set("Pragma", "no-cache")
		next.ServeHTTP(w, r)
	})
}

// csrfGuard requires a custom header on state-changing requests. Browsers cannot set custom
// headers on cross-origin requests without a CORS preflight (which the server never grants),
// so this blocks classic CSRF while the SameSite=Lax cookie covers the rest.
func csrfGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if r.Header.Get("X-Requested-With") == "" {
				writeError(w, http.StatusForbidden, "missing X-Requested-With header")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(cookieName)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		sess, ok := s.sessions.Get(c.Value)
		if !ok {
			s.clearSessionCookie(w)
			writeError(w, http.StatusUnauthorized, "session expired")
			return
		}
		ctx := context.WithValue(r.Context(), sessionCtxKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sess := sessionFrom(r.Context()); sess == nil || !sess.IsAdmin {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requirePasswordSet(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if sess := sessionFrom(r.Context()); sess != nil && sess.MustChangePassword {
			writeError(w, http.StatusForbidden, "password change required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- cookies ---

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   !s.cfg.Dev,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   !s.cfg.Dev,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// --- JSON helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeJSON reads a JSON body with a small size limit. It writes a 400 on failure.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	return decodeJSONLimit(w, r, dst, 1<<20)
}

// decodeJSONLimit reads a JSON body with an explicit size limit (used by the sign endpoint,
// which carries inline rasterized text-box images).
func decodeJSONLimit(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// writeServiceError maps domain/sentinel errors to HTTP responses.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid credentials")
	case errors.Is(err, auth.ErrAccountDisabled):
		writeError(w, http.StatusForbidden, "account disabled")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.Is(err, auth.ErrSetupAlreadyDone):
		writeError(w, http.StatusConflict, "setup already completed")
	case errors.Is(err, auth.ErrUserExists):
		writeError(w, http.StatusConflict, "username already exists")
	case errors.Is(err, auth.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, auth.ErrWeakPassword.Error())
	case errors.Is(err, auth.ErrInvalidUsername):
		writeError(w, http.StatusBadRequest, "invalid username (use letters, digits, . _ -)")
	case errors.Is(err, auth.ErrLastAdmin):
		writeError(w, http.StatusConflict, auth.ErrLastAdmin.Error())
	case errors.Is(err, auth.ErrCannotDeleteSelf):
		writeError(w, http.StatusBadRequest, auth.ErrCannotDeleteSelf.Error())
	case errors.Is(err, auth.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid input")
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
