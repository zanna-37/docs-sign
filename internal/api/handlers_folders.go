package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"docs-sign/internal/store"
)

type folderDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	ParentID  string `json:"parentId,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func folderToDTO(f store.Folder) folderDTO {
	return folderDTO{
		ID: f.ID, Name: f.Name, Kind: f.Kind, ParentID: f.ParentID.String,
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
	}
}

func folderDTOs(list []store.Folder) []folderDTO {
	out := make([]folderDTO, 0, len(list))
	for _, f := range list {
		out = append(out, folderToDTO(f))
	}
	return out
}

// validFolderKind reports whether kind names one of the two organization trees.
func validFolderKind(kind string) bool {
	return kind == store.KindDocument || kind == store.KindSignature
}

// handleListFolders returns the active subfolders under a parent (root when omitted), plus the
// breadcrumb path to that parent.
func (s *Server) handleListFolders(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	kind := r.URL.Query().Get("kind")
	if !validFolderKind(kind) {
		writeError(w, http.StatusBadRequest, "invalid folder kind")
		return
	}
	parent := folderParam(r, "parent")
	folders, err := s.store.ListFolders(r.Context(), sess.UserID, kind, parent)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	var path []folderDTO
	if parent.Valid {
		chain, err := s.store.FolderPath(r.Context(), sess.UserID, parent.String)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		path = folderDTOs(chain)
	}
	writeJSON(w, http.StatusOK, map[string]any{"folders": folderDTOs(folders), "path": path})
}

func (s *Server) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var req struct {
		Kind     string `json:"kind"`
		ParentID string `json:"parentId"`
		Name     string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validFolderKind(req.Kind) {
		writeError(w, http.StatusBadRequest, "invalid folder kind")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	f := &store.Folder{
		ID: store.NewID(), UserID: sess.UserID, Kind: req.Kind,
		ParentID: nullStr(req.ParentID), Name: req.Name,
	}
	if err := s.store.CreateFolder(r.Context(), f); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, folderToDTO(*f))
}

// handlePatchFolder renames a folder (name) and/or moves it (move.parentId, null = root).
func (s *Server) handlePatchFolder(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Name *string `json:"name"`
		Move *struct {
			ParentID *string `json:"parentId"`
		} `json:"move"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name != nil {
		if *req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if err := s.store.RenameFolder(r.Context(), sess.UserID, id, *req.Name); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	if req.Move != nil {
		if err := s.store.MoveFolder(r.Context(), sess.UserID, id, ptrToNull(req.Move.ParentID)); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleDeleteFolder moves a folder and its whole subtree to the trash as one event.
func (s *Server) handleDeleteFolder(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if _, err := s.store.TrashNode(r.Context(), sess.UserID, store.KindFolder, chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
