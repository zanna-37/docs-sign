package store

import (
	"context"
	"database/sql"
	"time"
)

// Signature is an encrypted PNG signature owned by a user.
type Signature struct {
	ID        string
	UserID    string
	Name      string
	BlobPath  string
	ByteSize  int64
	Width     int
	Height    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Document is an encrypted source PDF owned by a user.
type Document struct {
	ID        string
	UserID    string
	Name      string
	BlobPath  string
	ByteSize  int64
	PageCount int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Export is an encrypted, flattened (signed) PDF produced from a document.
type Export struct {
	ID         string
	UserID     string
	DocumentID sql.NullString
	Name       string
	BlobPath   string
	ByteSize   int64
	PageCount  int
	CreatedAt  time.Time
}

// Item kinds used by the trash API; each maps to one content table.
const (
	KindSignature = "signature"
	KindDocument  = "document"
	KindExport    = "export"
)

func tableForKind(kind string) (string, bool) {
	switch kind {
	case KindSignature:
		return "signatures", true
	case KindDocument:
		return "documents", true
	case KindExport:
		return "exports", true
	default:
		return "", false
	}
}

// --- Signatures ---

func (s *Store) CreateSignature(ctx context.Context, sig *Signature) error {
	now := time.Now().Unix()
	sig.CreatedAt, sig.UpdatedAt = unixToTime(now), unixToTime(now)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO signatures (id, user_id, name, blob_path, byte_size, width, height, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		sig.ID, sig.UserID, sig.Name, sig.BlobPath, sig.ByteSize, sig.Width, sig.Height, now, now)
	return err
}

func (s *Store) ListSignatures(ctx context.Context, userID string) ([]Signature, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, blob_path, byte_size, width, height, created_at, updated_at
		FROM signatures WHERE user_id=? AND deleted_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Signature
	for rows.Next() {
		var sig Signature
		var created, upd int64
		if err := rows.Scan(&sig.ID, &sig.UserID, &sig.Name, &sig.BlobPath, &sig.ByteSize,
			&sig.Width, &sig.Height, &created, &upd); err != nil {
			return nil, err
		}
		sig.CreatedAt, sig.UpdatedAt = unixToTime(created), unixToTime(upd)
		out = append(out, sig)
	}
	return out, rows.Err()
}

func (s *Store) GetSignature(ctx context.Context, userID, id string) (*Signature, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, blob_path, byte_size, width, height, created_at, updated_at
		FROM signatures WHERE id=? AND user_id=? AND deleted_at IS NULL`, id, userID)
	var sig Signature
	var created, upd int64
	err := row.Scan(&sig.ID, &sig.UserID, &sig.Name, &sig.BlobPath, &sig.ByteSize,
		&sig.Width, &sig.Height, &created, &upd)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	sig.CreatedAt, sig.UpdatedAt = unixToTime(created), unixToTime(upd)
	return &sig, nil
}

func (s *Store) RenameSignature(ctx context.Context, userID, id, name string) error {
	return s.renameRow(ctx, "signatures", userID, id, name)
}

func (s *Store) SoftDeleteSignature(ctx context.Context, userID, id string) error {
	return s.softDeleteRow(ctx, "signatures", userID, id)
}

// --- Documents ---

func (s *Store) CreateDocument(ctx context.Context, d *Document) error {
	now := time.Now().Unix()
	d.CreatedAt, d.UpdatedAt = unixToTime(now), unixToTime(now)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO documents (id, user_id, name, blob_path, byte_size, page_count, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		d.ID, d.UserID, d.Name, d.BlobPath, d.ByteSize, d.PageCount, now, now)
	return err
}

func (s *Store) ListDocuments(ctx context.Context, userID string) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, blob_path, byte_size, page_count, created_at, updated_at
		FROM documents WHERE user_id=? AND deleted_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var d Document
		var created, upd int64
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.BlobPath, &d.ByteSize,
			&d.PageCount, &created, &upd); err != nil {
			return nil, err
		}
		d.CreatedAt, d.UpdatedAt = unixToTime(created), unixToTime(upd)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDocument(ctx context.Context, userID, id string) (*Document, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, blob_path, byte_size, page_count, created_at, updated_at
		FROM documents WHERE id=? AND user_id=? AND deleted_at IS NULL`, id, userID)
	var d Document
	var created, upd int64
	err := row.Scan(&d.ID, &d.UserID, &d.Name, &d.BlobPath, &d.ByteSize,
		&d.PageCount, &created, &upd)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	d.CreatedAt, d.UpdatedAt = unixToTime(created), unixToTime(upd)
	return &d, nil
}

func (s *Store) RenameDocument(ctx context.Context, userID, id, name string) error {
	return s.renameRow(ctx, "documents", userID, id, name)
}

func (s *Store) SoftDeleteDocument(ctx context.Context, userID, id string) error {
	return s.softDeleteRow(ctx, "documents", userID, id)
}

// --- Exports ---

func (s *Store) CreateExport(ctx context.Context, e *Export) error {
	now := time.Now().Unix()
	e.CreatedAt = unixToTime(now)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO exports (id, user_id, document_id, name, blob_path, byte_size, page_count, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		e.ID, e.UserID, e.DocumentID, e.Name, e.BlobPath, e.ByteSize, e.PageCount, now)
	return err
}

func (s *Store) ListExports(ctx context.Context, userID string) ([]Export, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, document_id, name, blob_path, byte_size, page_count, created_at
		FROM exports WHERE user_id=? AND deleted_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Export
	for rows.Next() {
		var e Export
		var created int64
		if err := rows.Scan(&e.ID, &e.UserID, &e.DocumentID, &e.Name, &e.BlobPath,
			&e.ByteSize, &e.PageCount, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = unixToTime(created)
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *Store) GetExport(ctx context.Context, userID, id string) (*Export, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, document_id, name, blob_path, byte_size, page_count, created_at
		FROM exports WHERE id=? AND user_id=? AND deleted_at IS NULL`, id, userID)
	var e Export
	var created int64
	err := row.Scan(&e.ID, &e.UserID, &e.DocumentID, &e.Name, &e.BlobPath,
		&e.ByteSize, &e.PageCount, &created)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.CreatedAt = unixToTime(created)
	return &e, nil
}

func (s *Store) SoftDeleteExport(ctx context.Context, userID, id string) error {
	return s.softDeleteRow(ctx, "exports", userID, id)
}

// DeleteExportsForDocument permanently removes every export of a document (any state) and
// returns their blob paths. Used when a document is permanently deleted or purged.
func (s *Store) DeleteExportsForDocument(ctx context.Context, userID, documentID string) ([]string, error) {
	paths, err := s.collectStrings(ctx,
		`SELECT blob_path FROM exports WHERE user_id=? AND document_id=?`, userID, documentID)
	if err != nil {
		return nil, err
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM exports WHERE user_id=? AND document_id=?`, userID, documentID); err != nil {
		return nil, err
	}
	return paths, nil
}

// --- Trash ---

// TrashItem is one soft-deleted content item across all kinds.
type TrashItem struct {
	ID        string
	Kind      string
	Name      string
	ByteSize  int64
	DeletedAt time.Time
}

// ListTrash returns all soft-deleted items for a user, newest first.
func (s *Store) ListTrash(ctx context.Context, userID string) ([]TrashItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, 'signature' AS kind, name, byte_size, deleted_at FROM signatures WHERE user_id=? AND deleted_at IS NOT NULL
		UNION ALL
		SELECT id, 'document', name, byte_size, deleted_at FROM documents WHERE user_id=? AND deleted_at IS NOT NULL
		UNION ALL
		SELECT id, 'export', name, byte_size, deleted_at FROM exports WHERE user_id=? AND deleted_at IS NOT NULL
		ORDER BY deleted_at DESC`, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashItem
	for rows.Next() {
		var it TrashItem
		var del int64
		if err := rows.Scan(&it.ID, &it.Kind, &it.Name, &it.ByteSize, &del); err != nil {
			return nil, err
		}
		it.DeletedAt = unixToTime(del)
		out = append(out, it)
	}
	return out, rows.Err()
}

// RestoreItem brings a soft-deleted item back to active.
func (s *Store) RestoreItem(ctx context.Context, userID, kind, id string) error {
	table, ok := tableForKind(kind)
	if !ok {
		return ErrNotFound
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE `+table+` SET deleted_at=NULL WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// HardDeleteItem permanently removes a single trashed item and returns the blob paths to
// delete (for a document, this includes all of its exports).
func (s *Store) HardDeleteItem(ctx context.Context, userID, kind, id string) ([]string, error) {
	table, ok := tableForKind(kind)
	if !ok {
		return nil, ErrNotFound
	}
	var blobPath string
	err := s.db.QueryRowContext(ctx,
		`SELECT blob_path FROM `+table+` WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`, id, userID).Scan(&blobPath)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	var paths []string
	if kind == KindDocument {
		exp, err := s.DeleteExportsForDocument(ctx, userID, id)
		if err != nil {
			return nil, err
		}
		paths = append(paths, exp...)
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM `+table+` WHERE id=? AND user_id=?`, id, userID); err != nil {
		return nil, err
	}
	return append(paths, blobPath), nil
}

// EmptyTrash permanently removes every trashed item for a user, returning blob paths.
func (s *Store) EmptyTrash(ctx context.Context, userID string) ([]string, error) {
	items, err := s.ListTrash(ctx, userID)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, it := range items {
		p, err := s.HardDeleteItem(ctx, userID, it.Kind, it.ID)
		if err != nil && err != ErrNotFound {
			return nil, err
		}
		paths = append(paths, p...)
	}
	return paths, nil
}

// PurgeExpired permanently removes items trashed before cutoff (across all users) and
// returns their blob paths so the caller can delete the encrypted files.
func (s *Store) PurgeExpired(ctx context.Context, cutoff time.Time) ([]string, error) {
	c := cutoff.Unix()
	var paths []string

	// Documents first, cascading their exports regardless of the exports' state.
	type doc struct{ id, blob string }
	var docs []doc
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, blob_path FROM documents WHERE deleted_at IS NOT NULL AND deleted_at < ?`, c)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var d doc
		if err := rows.Scan(&d.id, &d.blob); err != nil {
			rows.Close()
			return nil, err
		}
		docs = append(docs, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, d := range docs {
		exp, err := s.collectStrings(ctx, `SELECT blob_path FROM exports WHERE document_id=?`, d.id)
		if err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM exports WHERE document_id=?`, d.id); err != nil {
			return nil, err
		}
		paths = append(paths, exp...)
		paths = append(paths, d.blob)
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM documents WHERE deleted_at IS NOT NULL AND deleted_at < ?`, c); err != nil {
		return nil, err
	}

	// Standalone trashed signatures and exports.
	for _, table := range []string{"signatures", "exports"} {
		p, err := s.collectStrings(ctx,
			`SELECT blob_path FROM `+table+` WHERE deleted_at IS NOT NULL AND deleted_at < ?`, c)
		if err != nil {
			return nil, err
		}
		if _, err := s.db.ExecContext(ctx,
			`DELETE FROM `+table+` WHERE deleted_at IS NOT NULL AND deleted_at < ?`, c); err != nil {
			return nil, err
		}
		paths = append(paths, p...)
	}
	return paths, nil
}

// DeleteUserContent removes all of a user's content (any state) without deleting the user
// row. Used by destructive admin reset.
func (s *Store) DeleteUserContent(ctx context.Context, userID string) error {
	for _, table := range []string{"exports", "documents", "signatures"} {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id=?`, userID); err != nil {
			return err
		}
	}
	return nil
}

// --- shared helpers ---

func (s *Store) collectStrings(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// renameRow updates the name of an active, user-owned row.
func (s *Store) renameRow(ctx context.Context, table, userID, id, name string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE `+table+` SET name=?, updated_at=? WHERE id=? AND user_id=? AND deleted_at IS NULL`,
		name, time.Now().Unix(), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// softDeleteRow moves an active, user-owned row to trash.
func (s *Store) softDeleteRow(ctx context.Context, table, userID, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE `+table+` SET deleted_at=? WHERE id=? AND user_id=? AND deleted_at IS NULL`,
		time.Now().Unix(), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
