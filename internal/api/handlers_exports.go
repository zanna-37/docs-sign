package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"docs-sign/internal/crypto"
	"docs-sign/internal/store"
)

type exportDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	DocumentID string `json:"documentId,omitempty"`
	PageCount  int    `json:"pageCount"`
	ByteSize   int64  `json:"byteSize"`
	CreatedAt  string `json:"createdAt"`
}

func exportToDTO(e store.Export) exportDTO {
	dto := exportDTO{
		ID: e.ID, Name: e.Name, PageCount: e.PageCount,
		ByteSize: e.ByteSize, CreatedAt: e.CreatedAt.Format(time.RFC3339),
	}
	if e.DocumentID.Valid {
		dto.DocumentID = e.DocumentID.String
	}
	return dto
}

func (s *Server) handleListExports(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	list, err := s.store.ListExports(r.Context(), sess.UserID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]exportDTO, 0, len(list))
	for _, e := range list {
		out = append(out, exportToDTO(e))
	}
	writeJSON(w, http.StatusOK, map[string]any{"exports": out})
}

func (s *Server) handleExportFile(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	exp, err := s.store.GetExport(r.Context(), sess.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	dek := sess.DEK()
	defer crypto.Zero(dek)

	// inline=1 renders in the browser (preview); otherwise force a download.
	disposition := "attachment"
	if r.URL.Query().Get("inline") == "1" {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q", disposition, downloadFilename(exp.Name)))
	_ = s.blobs.DecryptTo(exp.BlobPath, dek, w)
}

func (s *Server) handleDeleteExport(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if _, err := s.store.TrashNode(r.Context(), sess.UserID, store.KindExport, chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// downloadFilename produces a safe .pdf filename from a display name.
func downloadFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "document"
	}
	// Drop characters that are awkward in headers/filenames.
	name = strings.Map(func(r rune) rune {
		switch r {
		case '"', '\\', '/', '\n', '\r', '\t':
			return '_'
		}
		return r
	}, name)
	if !strings.HasSuffix(strings.ToLower(name), ".pdf") {
		name += ".pdf"
	}
	return name
}
