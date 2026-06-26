package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrNameConflict is returned when an operation would leave two active siblings (folders or
// items) sharing the same name in the same container.
var ErrNameConflict = errors.New("store: name already in use")

// Signature is an encrypted PNG signature owned by a user.
type Signature struct {
	ID        string
	UserID    string
	Name      string
	BlobPath  string
	ByteSize  int64
	Width     int
	Height    int
	FolderID  sql.NullString // NULL = root of the signature tree
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PDFContentType is the MIME type of the only document kind that can be signed.
const PDFContentType = "application/pdf"

// Document is an encrypted document owned by a user. Any file type may be stored; only a
// parseable PDF (ContentType PDFContentType with PageCount > 0) can be signed.
type Document struct {
	ID          string
	UserID      string
	Name        string
	BlobPath    string
	ByteSize    int64
	PageCount   int
	ContentType string
	FolderID    sql.NullString // NULL = root of the document tree
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Signable reports whether the document can be signed: only a PDF that parsed at upload time
// (yielding a page count) qualifies. Non-PDFs — and files that claim to be PDF but failed to
// parse (PageCount 0) — are stored as-is and cannot be signed.
func (d Document) Signable() bool {
	return d.ContentType == PDFContentType && d.PageCount > 0
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

// Item kinds used by the trash API; each maps to one table.
const (
	KindSignature = "signature"
	KindDocument  = "document"
	KindExport    = "export"
	KindFolder    = "folder"
)

func tableForKind(kind string) (string, bool) {
	switch kind {
	case KindSignature:
		return "signatures", true
	case KindDocument:
		return "documents", true
	case KindExport:
		return "exports", true
	case KindFolder:
		return "folders", true
	default:
		return "", false
	}
}

// folderKindForItem maps an item kind to the folder tree it belongs to.
func folderKindForItem(itemKind string) string {
	if itemKind == KindSignature {
		return KindSignature
	}
	return KindDocument
}

// --- shared item helpers ---

// validateFolder checks that folderID (when set) names an active folder owned by userID in the
// tree for kind. A root placement (invalid/NULL folderID) is always valid.
func validateFolder(ctx context.Context, q querier, userID, kind string, folderID sql.NullString) error {
	if !folderID.Valid {
		return nil
	}
	var k string
	err := q.QueryRowContext(ctx,
		`SELECT kind FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`,
		folderID.String, userID).Scan(&k)
	if err == sql.ErrNoRows || (err == nil && k != kind) {
		return ErrNotFound
	}
	return err
}

// itemNameTaken reports whether an active row in table (other than excludeID) already uses name
// in the same folder for this user.
func itemNameTaken(ctx context.Context, q querier, table, userID string, folderID sql.NullString, name, excludeID string) (bool, error) {
	var n int
	var err error
	if folderID.Valid {
		err = q.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM `+table+`
			 WHERE user_id=? AND folder_id=? AND name=? AND deleted_at IS NULL AND id<>?`,
			userID, folderID.String, name, excludeID).Scan(&n)
	} else {
		err = q.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM `+table+`
			 WHERE user_id=? AND folder_id IS NULL AND name=? AND deleted_at IS NULL AND id<>?`,
			userID, name, excludeID).Scan(&n)
	}
	return n > 0, err
}

// renameItem renames an active item, refusing a name already taken by an active sibling.
func (s *Store) renameItem(ctx context.Context, table, userID, id, name string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var folderID sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT folder_id FROM `+table+` WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			id, userID).Scan(&folderID)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		taken, err := itemNameTaken(ctx, tx, table, userID, folderID, name, id)
		if err != nil {
			return err
		}
		if taken {
			return ErrNameConflict
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE `+table+` SET name=?, updated_at=? WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			name, time.Now().Unix(), id, userID)
		return err
	})
}

// moveItem reparents an active item into folderID (root when invalid), refusing a destination
// where the name is already taken.
func (s *Store) moveItem(ctx context.Context, table, kind, userID, id string, folderID sql.NullString) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := validateFolder(ctx, tx, userID, folderKindForItem(kind), folderID); err != nil {
			return err
		}
		var name string
		err := tx.QueryRowContext(ctx,
			`SELECT name FROM `+table+` WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			id, userID).Scan(&name)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		taken, err := itemNameTaken(ctx, tx, table, userID, folderID, name, id)
		if err != nil {
			return err
		}
		if taken {
			return ErrNameConflict
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE `+table+` SET folder_id=?, updated_at=? WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			folderID, time.Now().Unix(), id, userID)
		return err
	})
}

// --- Signatures ---

func (s *Store) CreateSignature(ctx context.Context, sig *Signature) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := validateFolder(ctx, tx, sig.UserID, KindSignature, sig.FolderID); err != nil {
			return err
		}
		taken, err := itemNameTaken(ctx, tx, "signatures", sig.UserID, sig.FolderID, sig.Name, "")
		if err != nil {
			return err
		}
		if taken {
			return ErrNameConflict
		}
		now := time.Now().Unix()
		sig.CreatedAt, sig.UpdatedAt = unixToTime(now), unixToTime(now)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO signatures (id, user_id, name, blob_path, byte_size, width, height, folder_id, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			sig.ID, sig.UserID, sig.Name, sig.BlobPath, sig.ByteSize, sig.Width, sig.Height, sig.FolderID, now, now)
		return err
	})
}

const signatureSelect = `
	SELECT id, user_id, name, blob_path, byte_size, width, height, folder_id, created_at, updated_at
	FROM signatures`

// ListSignatures lists active signatures directly inside folderID (root when invalid).
func (s *Store) ListSignatures(ctx context.Context, userID string, folderID sql.NullString) ([]Signature, error) {
	rows, err := s.queryItemsInFolder(ctx, signatureSelect, userID, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSignatures(rows)
}

// ListAllSignatures lists every active signature for a user across all folders — the flat list
// the signing editor needs to place any signature regardless of where it is filed.
func (s *Store) ListAllSignatures(ctx context.Context, userID string) ([]Signature, error) {
	rows, err := s.db.QueryContext(ctx,
		signatureSelect+` WHERE user_id=? AND deleted_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSignatures(rows)
}

func scanSignatures(rows *sql.Rows) ([]Signature, error) {
	var out []Signature
	for rows.Next() {
		var sig Signature
		var created, upd int64
		if err := rows.Scan(&sig.ID, &sig.UserID, &sig.Name, &sig.BlobPath, &sig.ByteSize,
			&sig.Width, &sig.Height, &sig.FolderID, &created, &upd); err != nil {
			return nil, err
		}
		sig.CreatedAt, sig.UpdatedAt = unixToTime(created), unixToTime(upd)
		out = append(out, sig)
	}
	return out, rows.Err()
}

func (s *Store) GetSignature(ctx context.Context, userID, id string) (*Signature, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, blob_path, byte_size, width, height, folder_id, created_at, updated_at
		FROM signatures WHERE id=? AND user_id=? AND deleted_at IS NULL`, id, userID)
	var sig Signature
	var created, upd int64
	err := row.Scan(&sig.ID, &sig.UserID, &sig.Name, &sig.BlobPath, &sig.ByteSize,
		&sig.Width, &sig.Height, &sig.FolderID, &created, &upd)
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
	return s.renameItem(ctx, "signatures", userID, id, name)
}

func (s *Store) MoveSignature(ctx context.Context, userID, id string, folderID sql.NullString) error {
	return s.moveItem(ctx, "signatures", KindSignature, userID, id, folderID)
}

// --- Documents ---

func (s *Store) CreateDocument(ctx context.Context, d *Document) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if err := validateFolder(ctx, tx, d.UserID, KindDocument, d.FolderID); err != nil {
			return err
		}
		taken, err := itemNameTaken(ctx, tx, "documents", d.UserID, d.FolderID, d.Name, "")
		if err != nil {
			return err
		}
		if taken {
			return ErrNameConflict
		}
		now := time.Now().Unix()
		d.CreatedAt, d.UpdatedAt = unixToTime(now), unixToTime(now)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO documents (id, user_id, name, blob_path, byte_size, page_count, content_type, folder_id, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?,?,?,?)`,
			d.ID, d.UserID, d.Name, d.BlobPath, d.ByteSize, d.PageCount, d.ContentType, d.FolderID, now, now)
		return err
	})
}

const documentSelect = `
	SELECT id, user_id, name, blob_path, byte_size, page_count, content_type, folder_id, created_at, updated_at
	FROM documents`

// ListDocuments lists active documents directly inside folderID (root when invalid).
func (s *Store) ListDocuments(ctx context.Context, userID string, folderID sql.NullString) ([]Document, error) {
	rows, err := s.queryItemsInFolder(ctx, documentSelect, userID, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocuments(rows)
}

// ListAllDocuments lists every active document for a user across all folders.
func (s *Store) ListAllDocuments(ctx context.Context, userID string) ([]Document, error) {
	rows, err := s.db.QueryContext(ctx,
		documentSelect+` WHERE user_id=? AND deleted_at IS NULL ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanDocuments(rows)
}

func scanDocuments(rows *sql.Rows) ([]Document, error) {
	var out []Document
	for rows.Next() {
		var d Document
		var created, upd int64
		if err := rows.Scan(&d.ID, &d.UserID, &d.Name, &d.BlobPath, &d.ByteSize,
			&d.PageCount, &d.ContentType, &d.FolderID, &created, &upd); err != nil {
			return nil, err
		}
		d.CreatedAt, d.UpdatedAt = unixToTime(created), unixToTime(upd)
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) GetDocument(ctx context.Context, userID, id string) (*Document, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, blob_path, byte_size, page_count, content_type, folder_id, created_at, updated_at
		FROM documents WHERE id=? AND user_id=? AND deleted_at IS NULL`, id, userID)
	var d Document
	var created, upd int64
	err := row.Scan(&d.ID, &d.UserID, &d.Name, &d.BlobPath, &d.ByteSize,
		&d.PageCount, &d.ContentType, &d.FolderID, &created, &upd)
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
	return s.renameItem(ctx, "documents", userID, id, name)
}

func (s *Store) MoveDocument(ctx context.Context, userID, id string, folderID sql.NullString) error {
	return s.moveItem(ctx, "documents", KindDocument, userID, id, folderID)
}

// queryItemsInFolder lists active items directly inside folderID (root when invalid), newest
// first. The select must end at the table name; this appends the folder predicate.
func (s *Store) queryItemsInFolder(ctx context.Context, selectSQL, userID string, folderID sql.NullString) (*sql.Rows, error) {
	if folderID.Valid {
		return s.db.QueryContext(ctx,
			selectSQL+` WHERE user_id=? AND folder_id=? AND deleted_at IS NULL ORDER BY created_at DESC`,
			userID, folderID.String)
	}
	return s.db.QueryContext(ctx,
		selectSQL+` WHERE user_id=? AND folder_id IS NULL AND deleted_at IS NULL ORDER BY created_at DESC`,
		userID)
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

// DeleteExportsForDocument permanently removes every export of a document (any state) and
// returns their blob paths. Used when a document is permanently deleted or purged.
func (s *Store) DeleteExportsForDocument(ctx context.Context, userID, documentID string) ([]string, error) {
	paths, err := collectStrings(ctx, s.db,
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

// FindActiveItem returns the id of the active item named name inside folderID (root when
// invalid), reporting whether one exists. Used to resolve an "overwrite" upload.
func (s *Store) FindActiveItem(ctx context.Context, userID, kind string, folderID sql.NullString, name string) (string, bool, error) {
	table, ok := tableForKind(kind)
	if !ok {
		return "", false, ErrNotFound
	}
	var id string
	var err error
	if folderID.Valid {
		err = s.db.QueryRowContext(ctx,
			`SELECT id FROM `+table+` WHERE user_id=? AND folder_id=? AND name=? AND deleted_at IS NULL LIMIT 1`,
			userID, folderID.String, name).Scan(&id)
	} else {
		err = s.db.QueryRowContext(ctx,
			`SELECT id FROM `+table+` WHERE user_id=? AND folder_id IS NULL AND name=? AND deleted_at IS NULL LIMIT 1`,
			userID, name).Scan(&id)
	}
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// AllReferencedBlobPaths returns the set of every blob path referenced by a content row, in any
// state. The blob reconciler diffs this against what is on disk to find orphans — blobs whose
// row was already deleted but whose file was never removed (e.g. a hard delete whose best-effort
// blob delete failed or was interrupted). No deleted_at filter is applied: a trashed-but-not-yet-
// purged item still owns its blob and must be retained.
func (s *Store) AllReferencedBlobPaths(ctx context.Context) (map[string]struct{}, error) {
	refs := make(map[string]struct{})
	for _, table := range []string{"signatures", "documents", "exports"} {
		paths, err := collectStrings(ctx, s.db, `SELECT blob_path FROM `+table)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			refs[p] = struct{}{}
		}
	}
	return refs, nil
}

// DeleteUserContent removes all of a user's content (any state) without deleting the user row.
// Used by destructive admin reset.
func (s *Store) DeleteUserContent(ctx context.Context, userID string) error {
	for _, table := range []string{"exports", "documents", "signatures", "folders", "trash_events"} {
		if _, err := s.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id=?`, userID); err != nil {
			return err
		}
	}
	return nil
}
