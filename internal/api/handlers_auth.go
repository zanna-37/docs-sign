package api

import (
	"net/http"

	"docs-sign/internal/session"
)

type userDTO struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	IsAdmin            bool   `json:"isAdmin"`
	MustChangePassword bool   `json:"mustChangePassword"`
}

func dtoFromSession(s *session.Session) userDTO {
	return userDTO{ID: s.UserID, Username: s.Username, IsAdmin: s.IsAdmin, MustChangePassword: s.MustChangePassword}
}

func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	need, err := s.auth.NeedsSetup(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"needsSetup": need})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	recovery, err := s.auth.Setup(r.Context(), req.Username, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"recoveryCode": recovery})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sess, err := s.auth.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	s.setSessionCookie(w, sess.Token)
	writeJSON(w, http.StatusOK, map[string]any{"user": dtoFromSession(sess)})
}

func (s *Server) handleRecover(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username     string `json:"username"`
		RecoveryCode string `json:"recoveryCode"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sess, err := s.auth.Recover(r.Context(), req.Username, req.RecoveryCode)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	s.setSessionCookie(w, sess.Token)
	writeJSON(w, http.StatusOK, map[string]any{"user": dtoFromSession(sess)})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(cookieName); err == nil {
		s.sessions.Delete(c.Value)
	}
	s.clearSessionCookie(w)
	// Ask the browser to purge any cached resources and site storage on logout. This is
	// honored over HTTPS (i.e. in production behind the reverse proxy).
	w.Header().Set("Clear-Site-Data", `"cache", "storage"`)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"user": dtoFromSession(sess)})
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	sess := sessionFrom(r.Context())
	recovery, err := s.auth.ChangePassword(r.Context(), sess, req.NewPassword)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	resp := map[string]any{"status": "ok"}
	if recovery != "" {
		resp["recoveryCode"] = recovery
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleRegenerateRecovery(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	recovery, err := s.auth.RegenerateRecoveryCode(r.Context(), sess)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"recoveryCode": recovery})
}
