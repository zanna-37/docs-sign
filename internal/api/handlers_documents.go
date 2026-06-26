package api

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"docs-sign/internal/crypto"
	"docs-sign/internal/pdfproc"
	"docs-sign/internal/store"
)

// decodeInlinePNG decodes a base64 PNG (optionally a data: URL) into an image.
func decodeInlinePNG(data string) (image.Image, error) {
	if strings.HasPrefix(data, "data:") {
		if i := strings.IndexByte(data, ','); i >= 0 {
			data = data[i+1:]
		}
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(data))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

var pdfMagic = []byte("%PDF-")

type documentDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	FolderID  string `json:"folderId,omitempty"`
	PageCount int    `json:"pageCount"`
	ByteSize  int64  `json:"byteSize"`
	CreatedAt string `json:"createdAt"`
}

func documentToDTO(d store.Document) documentDTO {
	return documentDTO{
		ID: d.ID, Name: d.Name, FolderID: d.FolderID.String, PageCount: d.PageCount,
		ByteSize: d.ByteSize, CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}
}

func (s *Server) handleListDocuments(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	// ?all=true returns every document across folders (used by the signing editor); otherwise
	// the listing is scoped to one folder.
	list, err := s.listDocuments(r, sess.UserID)
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

func (s *Server) listDocuments(r *http.Request, userID string) ([]store.Document, error) {
	if r.URL.Query().Get("all") == "true" {
		return s.store.ListAllDocuments(r.Context(), userID)
	}
	return s.store.ListDocuments(r.Context(), userID, folderParam(r, "folder"))
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

	folder := folderParam(r, "folder")
	if !s.resolveOverwrite(w, r, sess.UserID, store.KindDocument, name) {
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
		ByteSize: size, PageCount: pageCount, FolderID: folder,
	}
	if err := s.store.CreateDocument(r.Context(), doc); err != nil {
		_ = s.blobs.Delete(relPath)
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, documentToDTO(*doc))
}

// resolveOverwrite honors an "overwrite=true" upload by moving any active same-named item in the
// target folder to its own trash event first, so the new upload can take the name. It returns
// false (after writing an error) only on an unexpected failure.
func (s *Server) resolveOverwrite(w http.ResponseWriter, r *http.Request, userID, kind, name string) bool {
	if r.URL.Query().Get("overwrite") != "true" {
		return true
	}
	existingID, found, err := s.store.FindActiveItem(r.Context(), userID, kind, folderParam(r, "folder"), name)
	if err != nil {
		writeServiceError(w, err)
		return false
	}
	if found {
		if _, err := s.store.TrashNode(r.Context(), userID, kind, existingID); err != nil {
			writeServiceError(w, err)
			return false
		}
	}
	return true
}

// handleGetDocument returns a single document's metadata by id. The signing editor uses it to
// resolve the open document's name without listing the whole library.
func (s *Server) handleGetDocument(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	doc, err := s.store.GetDocument(r.Context(), sess.UserID, chi.URLParam(r, "id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, documentToDTO(*doc))
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

// handlePatchDocument renames a document (name) and/or moves it (move.folderId, null = root).
func (s *Server) handlePatchDocument(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	id := chi.URLParam(r, "id")
	var req struct {
		Name *string `json:"name"`
		Move *struct {
			FolderID *string `json:"folderId"`
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
		if err := s.store.RenameDocument(r.Context(), sess.UserID, id, *req.Name); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	if req.Move != nil {
		if err := s.store.MoveDocument(r.Context(), sess.UserID, id, ptrToNull(req.Move.FolderID)); err != nil {
			writeServiceError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDeleteDocument(w http.ResponseWriter, r *http.Request) {
	sess := sessionFrom(r.Context())
	// Trash only the document; its signed copies ride along (hidden) and are removed when the
	// document is permanently deleted or purged.
	if _, err := s.store.TrashNode(r.Context(), sess.UserID, store.KindDocument, chi.URLParam(r, "id")); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type placementReq struct {
	SignatureID string  `json:"signatureId"`
	ImageData   string  `json:"imageData"` // base64 PNG (optionally a data URL) for text boxes
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
	// A larger limit than the default JSON body cap: text boxes ship their rasterized
	// image inline.
	if !decodeJSONLimit(w, r, &req, s.cfg.MaxUploadBytes) {
		return
	}
	if len(req.Placements) == 0 {
		writeError(w, http.StatusBadRequest, "at least one placement is required")
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

	// Resolve each placement's image: a stored signature (decoded once and cached), or an
	// inline rasterized text box sent by the client.
	sigImages := make(map[string]image.Image)
	placements := make([]pdfproc.Placement, 0, len(req.Placements))
	for _, p := range req.Placements {
		var img image.Image
		switch {
		case p.SignatureID != "":
			cached, ok := sigImages[p.SignatureID]
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
				cached = decoded
				sigImages[p.SignatureID] = cached
			}
			img = cached
		case p.ImageData != "":
			decoded, err := decodeInlinePNG(p.ImageData)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid text-box image")
				return
			}
			img = decoded
		default:
			writeError(w, http.StatusBadRequest, "placement needs a signature or image")
			return
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
