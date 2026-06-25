package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"docs-sign/internal/store"
)

type trashEventDTO struct {
	EventID   string `json:"eventId"`
	RootKind  string `json:"rootKind"`
	RootID    string `json:"rootId"`
	Label     string `json:"label"`
	ByteSize  int64  `json:"byteSize"`
	ItemCount int    `json:"itemCount"`
	DeletedAt string `json:"deletedAt"`
	PurgeAt   string `json:"purgeAt"`
}

func (s *Server) handleListTrash(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	events, err := s.store.ListTrashEvents(r.Context(), sess.UserID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]trashEventDTO, 0, len(events))
	for _, e := range events {
		out = append(out, trashEventDTO{
			EventID:   e.EventID,
			RootKind:  e.RootKind,
			RootID:    e.RootID,
			Label:     e.Label,
			ByteSize:  e.ByteSize,
			ItemCount: e.ItemCount,
			DeletedAt: e.CreatedAt.Format(time.RFC3339),
			PurgeAt:   e.CreatedAt.Add(s.cfg.TrashRetention).Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"events":        out,
		"retentionDays": int(s.cfg.TrashRetention.Hours() / 24),
	})
}

type trashEntryDTO struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	ByteSize int64  `json:"byteSize"`
	IsFolder bool   `json:"isFolder"`
}

// handleWalkTrash lists the trashed children of a folder inside an event — letting the UI walk
// into a trashed folder. With no folder query it lists the event root's contents.
func (s *Server) handleWalkTrash(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	eventID := chi.URLParam(r, "eventId")
	ev, err := s.store.GetTrashEvent(r.Context(), sess.UserID, eventID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	folder := folderParam(r, "folder")
	if !folder.Valid && ev.RootKind == store.KindFolder {
		folder = nullStr(ev.RootID)
	}
	entries, err := s.store.ListTrashChildren(r.Context(), sess.UserID, eventID, folder)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]trashEntryDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, trashEntryDTO{
			Kind: e.Kind, ID: e.ID, Name: e.Name, ByteSize: e.ByteSize, IsFolder: e.IsFolder,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": out})
}

type restoreConflictDTO struct {
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	Name     string `json:"name"`
	DestPath string `json:"destPath"`
}

// handleRestoreTrash restores a trashed node (an event root or any node found by walking). An
// optional body carries per-file conflict resolutions; if unresolved file-name collisions
// remain, the restore is not applied and they are returned with 409 Conflict.
func (s *Server) handleRestoreTrash(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var req struct {
		Resolutions map[string]struct {
			Action  string `json:"action"`
			NewName string `json:"newName"`
		} `json:"resolutions"`
	}
	if ok := decodeOptionalJSON(w, r, &req); !ok {
		return
	}
	res := make(map[string]store.Resolution, len(req.Resolutions))
	for id, v := range req.Resolutions {
		res[id] = store.Resolution{Action: v.Action, NewName: v.NewName}
	}

	conflicts, err := s.store.RestoreNode(r.Context(), sess.UserID,
		chi.URLParam(r, "kind"), chi.URLParam(r, "id"), res)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if len(conflicts) > 0 {
		out := make([]restoreConflictDTO, 0, len(conflicts))
		for _, c := range conflicts {
			out = append(out, restoreConflictDTO{Kind: c.Kind, ID: c.ID, Name: c.Name, DestPath: c.DestPath})
		}
		writeJSON(w, http.StatusConflict, map[string]any{"conflicts": out})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handlePurgeEvent(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	paths, err := s.store.HardDeleteEvent(r.Context(), sess.UserID, chi.URLParam(r, "eventId"))
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

// decodeOptionalJSON decodes a small JSON body into dst, treating an empty body as "no fields".
// It writes a 400 and returns false on a malformed body.
func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
