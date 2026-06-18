package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

type trashItemDTO struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	ByteSize  int64  `json:"byteSize"`
	DeletedAt string `json:"deletedAt"`
	PurgeAt   string `json:"purgeAt"`
}

func (s *Server) handleListTrash(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	items, err := s.store.ListTrash(r.Context(), sess.UserID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]trashItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, trashItemDTO{
			ID:        it.ID,
			Kind:      it.Kind,
			Name:      it.Name,
			ByteSize:  it.ByteSize,
			DeletedAt: it.DeletedAt.Format(time.RFC3339),
			PurgeAt:   it.DeletedAt.Add(s.cfg.TrashRetention).Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":         out,
		"retentionDays": int(s.cfg.TrashRetention.Hours() / 24),
	})
}

func (s *Server) handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if err := s.store.RestoreItem(r.Context(), sess.UserID, chi.URLParam(r, "kind"), chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePurgeTrashItem(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	paths, err := s.store.HardDeleteItem(r.Context(), sess.UserID, chi.URLParam(r, "kind"), chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	for _, p := range paths {
		_ = s.blobs.Delete(p)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleEmptyTrash(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	paths, err := s.store.EmptyTrash(r.Context(), sess.UserID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	for _, p := range paths {
		_ = s.blobs.Delete(p)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
