package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"docs-sign/internal/store"
)

type adminUserDTO struct {
	ID                 string `json:"id"`
	Username           string `json:"username"`
	IsAdmin            bool   `json:"isAdmin"`
	Status             string `json:"status"`
	MustChangePassword bool   `json:"mustChangePassword"`
	CreatedAt          string `json:"createdAt"`
}

func adminUserToDTO(u store.User) adminUserDTO {
	return adminUserDTO{
		ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin, Status: u.Status,
		MustChangePassword: u.MustChangePassword, CreatedAt: u.CreatedAt.Format(time.RFC3339),
	}
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	users, err := s.auth.ListUsers(r.Context(), sess)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]adminUserDTO, 0, len(users))
	for _, u := range users {
		out = append(out, adminUserToDTO(u))
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": out})
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var req struct {
		Username     string `json:"username"`
		TempPassword string `json:"tempPassword"`
		IsAdmin      bool   `json:"isAdmin"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.auth.AdminCreateUser(r.Context(), sess, req.Username, req.TempPassword, req.IsAdmin)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, adminUserToDTO(*u))
}

func (s *Server) handleSetUserStatus(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var req struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.auth.AdminSetUserStatus(r.Context(), sess, chi.URLParam(r, "id"), req.Status); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleResetUser(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var req struct {
		TempPassword string `json:"tempPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.auth.AdminResetUser(r.Context(), sess, chi.URLParam(r, "id"), req.TempPassword); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if err := s.auth.AdminDeleteUser(r.Context(), sess, chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
