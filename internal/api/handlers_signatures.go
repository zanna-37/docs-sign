package api

import (
	"bytes"
	"image/png"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"docs-sign/internal/crypto"
	"docs-sign/internal/store"
)

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}

type signatureDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	ByteSize  int64  `json:"byteSize"`
	CreatedAt string `json:"createdAt"`
}

func signatureToDTO(s store.Signature) signatureDTO {
	return signatureDTO{
		ID: s.ID, Name: s.Name, Width: s.Width, Height: s.Height,
		ByteSize: s.ByteSize, CreatedAt: s.CreatedAt.Format(time.RFC3339),
	}
}

// readUpload parses a multipart form, enforces the upload size limit, and returns the file
// bytes plus the chosen display name.
func (s *Server) readUpload(w http.ResponseWriter, r *http.Request) (data []byte, name string, ok bool) {
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadBytes)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "upload too large or malformed")
		return nil, "", false
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file field")
		return nil, "", false
	}
	defer file.Close()
	data, err = io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read upload")
		return nil, "", false
	}
	name = r.FormValue("name")
	if name == "" {
		name = hdr.Filename
	}
	if name == "" {
		name = "untitled"
	}
	return data, name, true
}

func (s *Server) handleListSignatures(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	list, err := s.store.ListSignatures(r.Context(), sess.UserID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]signatureDTO, 0, len(list))
	for _, sig := range list {
		out = append(out, signatureToDTO(sig))
	}
	writeJSON(w, http.StatusOK, map[string]any{"signatures": out})
}

func (s *Server) handleUploadSignature(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	data, name, ok := s.readUpload(w, r)
	if !ok {
		return
	}
	if !bytes.HasPrefix(data, pngMagic) {
		writeError(w, http.StatusBadRequest, "signature must be a PNG image")
		return
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PNG image")
		return
	}

	dek := sess.DEK()
	defer crypto.Zero(dek)
	id := store.NewID()
	relPath, size, err := s.blobs.WriteBytes(sess.UserID, id, dek, data)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	sig := &store.Signature{
		ID: id, UserID: sess.UserID, Name: name, BlobPath: relPath,
		ByteSize: size, Width: cfg.Width, Height: cfg.Height,
	}
	if err := s.store.CreateSignature(r.Context(), sig); err != nil {
		_ = s.blobs.Delete(relPath)
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, signatureToDTO(*sig))
}

func (s *Server) handleSignatureImage(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	sig, err := s.store.GetSignature(r.Context(), sess.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	dek := sess.DEK()
	defer crypto.Zero(dek)
	w.Header().Set("Content-Type", "image/png")
	if err := s.blobs.DecryptTo(sig.BlobPath, dek, w); err != nil {
		// Headers may already be sent; nothing more we can do but log-less fail.
		return
	}
}

func (s *Server) handleRenameSignature(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := s.store.RenameSignature(r.Context(), sess.UserID, chi.URLParam(r, "id"), req.Name); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteSignature(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	if err := s.store.SoftDeleteSignature(r.Context(), sess.UserID, chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
