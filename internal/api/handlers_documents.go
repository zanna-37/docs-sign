package api

import (
	"bytes"
	"fmt"
	"image"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"docs-sign/internal/crypto"
	"docs-sign/internal/pdfproc"
	"docs-sign/internal/store"
)

var pdfMagic = []byte("%PDF-")

type documentDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	PageCount int    `json:"pageCount"`
	ByteSize  int64  `json:"byteSize"`
	CreatedAt string `json:"createdAt"`
}

func documentToDTO(d store.Document) documentDTO {
	return documentDTO{
		ID: d.ID, Name: d.Name, PageCount: d.PageCount,
		ByteSize: d.ByteSize, CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	list, err := s.store.ListDocuments(r.Context(), sess.UserID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	out := make([]documentDTO, 0, len(list))
	for _, d := range list {
		out = append(out, documentToDTO(d))
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": out})
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	data, name, ok := s.readUpload(w, r)
	if !ok {
		return
	}
	if !bytes.HasPrefix(data, pdfMagic) {
		writeError(w, http.StatusBadRequest, "file must be a PDF")
		return
	}
	pageCount, err := s.pdf.PageCount(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read PDF")
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
	doc := &store.Document{
		ID: id, UserID: sess.UserID, Name: name, BlobPath: relPath,
		ByteSize: size, PageCount: pageCount,
	}
	if err := s.store.CreateDocument(r.Context(), doc); err != nil {
		_ = s.blobs.Delete(relPath)
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, documentToDTO(*doc))
}

func (s *Server) handleDocumentFile(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	doc, err := s.store.GetDocument(r.Context(), sess.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	dek := sess.DEK()
	defer crypto.Zero(dek)
	w.Header().Set("Content-Type", "application/pdf")
	_ = s.blobs.DecryptTo(doc.BlobPath, dek, w)
}

func (s *Server) handleRenameDocument(w http.ResponseWriter, r *http.Request) {
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
	if err := s.store.RenameDocument(r.Context(), sess.UserID, chi.URLParam(r, "id"), req.Name); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	relPath, err := s.store.DeleteDocument(r.Context(), sess.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	_ = s.blobs.Delete(relPath)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type placementReq struct {
	SignatureID string  `json:"signatureId"`
	Page        int     `json:"page"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	W           float64 `json:"w"`
	H           float64 `json:"h"`
	Rotation    float64 `json:"rotation"`
}

func (s *Server) handleSignDocument(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	var req struct {
		Name       string         `json:"name"`
		DPI        int            `json:"dpi"`
		Placements []placementReq `json:"placements"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.Placements) == 0 {
		writeError(w, http.StatusBadRequest, "at least one signature placement is required")
		return
	}

	doc, err := s.store.GetDocument(r.Context(), sess.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	dek := sess.DEK()
	defer crypto.Zero(dek)

	pdfBytes, err := s.blobs.ReadAll(doc.BlobPath, dek)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	// Decode each referenced signature once.
	sigImages := make(map[string]image.Image)
	placements := make([]pdfproc.Placement, 0, len(req.Placements))
	for _, p := range req.Placements {
		img, ok := sigImages[p.SignatureID]
		if !ok {
			sig, err := s.store.GetSignature(r.Context(), sess.UserID, p.SignatureID)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			pngBytes, err := s.blobs.ReadAll(sig.BlobPath, dek)
			if err != nil {
				writeServiceError(w, err)
				return
			}
			decoded, _, err := image.Decode(bytes.NewReader(pngBytes))
			if err != nil {
				writeError(w, http.StatusBadRequest, "failed to decode signature image")
				return
			}
			img = decoded
			sigImages[p.SignatureID] = img
		}
		placements = append(placements, pdfproc.Placement{
			Page: p.Page, X: p.X, Y: p.Y, W: p.W, H: p.H, RotationDeg: p.Rotation, Image: img,
		})
	}

	dpi := req.DPI
	if dpi <= 0 {
		dpi = s.cfg.ExportDPI
	}
	signed, pageCount, err := s.pdf.Sign(pdfBytes, placements, pdfproc.SignOptions{DPI: dpi})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := req.Name
	if name == "" {
		name = fmt.Sprintf("%s (signed)", doc.Name)
	}
	exportID := store.NewID()
	relPath, size, err := s.blobs.WriteBytes(sess.UserID, exportID, dek, signed)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	exp := &store.Export{
		ID: exportID, UserID: sess.UserID, Name: name, BlobPath: relPath,
		ByteSize: size, PageCount: pageCount,
	}
	exp.DocumentID.Valid = true
	exp.DocumentID.String = doc.ID
	if err := s.store.CreateExport(r.Context(), exp); err != nil {
		_ = s.blobs.Delete(relPath)
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, exportToDTO(*exp))
}
