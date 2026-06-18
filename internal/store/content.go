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
		FROM signatures WHERE user_id=? ORDER BY created_at DESC`, userID)
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
		FROM signatures WHERE id=? AND user_id=?`, id, userID)
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

func (s *Store) DeleteSignature(ctx context.Context, userID, id string) (string, error) {
	return s.deleteRow(ctx, "signatures", userID, id)
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
		FROM documents WHERE user_id=? ORDER BY created_at DESC`, userID)
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
		FROM documents WHERE id=? AND user_id=?`, id, userID)
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

func (s *Store) DeleteDocument(ctx context.Context, userID, id string) (string, error) {
	return s.deleteRow(ctx, "documents", userID, id)
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
		FROM exports WHERE user_id=? ORDER BY created_at DESC`, userID)
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
		FROM exports WHERE id=? AND user_id=?`, id, userID)
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

func (s *Store) DeleteExport(ctx context.Context, userID, id string) (string, error) {
	return s.deleteRow(ctx, "exports", userID, id)
}

// DeleteUserContent removes all signatures, documents and exports for a user without
// deleting the user row. Used by destructive admin reset; the caller is responsible for
// removing the user's blob files from disk.
func (s *Store) DeleteUserContent(ctx context.Context, userID string) error {
	for _, table := range []string{"exports", "documents", "signatures"} {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id=?`, userID); err != nil {
			return err
		}
	}
	return nil
}

// --- shared helpers ---

// renameRow updates the name of a user-owned row in the given table.
func (s *Store) renameRow(ctx context.Context, table, userID, id, name string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE `+table+` SET name=?, updated_at=? WHERE id=? AND user_id=?`,
		name, time.Now().Unix(), id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// deleteRow removes a user-owned row and returns the blob_path so the caller can delete the
// encrypted file from disk.
func (s *Store) deleteRow(ctx context.Context, table, userID, id string) (string, error) {
	var blobPath string
	err := s.db.QueryRowContext(ctx,
		`SELECT blob_path FROM `+table+` WHERE id=? AND user_id=?`, id, userID).Scan(&blobPath)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM `+table+` WHERE id=? AND user_id=?`, id, userID); err != nil {
		return "", err
	}
	return blobPath, nil
}
