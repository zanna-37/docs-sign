// Package store holds all server-side metadata in a SQLite database. It never stores
// plaintext user content or plaintext keys — only usernames, wrapped DEKs, KDF salts,
// and pointers (paths + sizes) to encrypted blobs managed by the blob package.
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("store: not found")

// Store is a handle to the metadata database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if necessary) the SQLite database at path and applies migrations.
func Open(path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite is happiest with a single writer; the WAL journal allows concurrent readers.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return err
	}
	// Idempotent upgrades for databases created before a column existed. ADD COLUMN with a
	// nullable REFERENCES clause is permitted by SQLite because the default value is NULL.
	for _, table := range []string{"signatures", "documents", "exports"} {
		if err := s.ensureColumn(ctx, table, "deleted_at", "INTEGER"); err != nil {
			return err
		}
		if err := s.ensureColumn(ctx, table, "trash_event_id", "TEXT REFERENCES trash_events(id) ON DELETE SET NULL"); err != nil {
			return err
		}
	}
	// Folders organize documents and signatures; exports ride with their parent document.
	for _, table := range []string{"signatures", "documents"} {
		if err := s.ensureColumn(ctx, table, "folder_id", "TEXT REFERENCES folders(id) ON DELETE SET NULL"); err != nil {
			return err
		}
	}
	if err := s.ensureColumn(ctx, "users", "language", "TEXT"); err != nil {
		return err
	}
	return nil
}

// ensureColumn adds a column to a table if it does not already exist.
func (s *Store) ensureColumn(ctx context.Context, table, column, decl string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, ctype string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Close()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	_, err = s.db.ExecContext(ctx, "ALTER TABLE "+table+" ADD COLUMN "+column+" "+decl)
	return err
}

// NewID returns a random 128-bit identifier as a hex string.
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand failure is not recoverable for our purposes.
		panic("store: crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

func unixToTime(sec int64) time.Time { return time.Unix(sec, 0).UTC() }

// querier is satisfied by both *sql.DB and *sql.Tx, so helpers can run inside or outside a
// transaction.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// withTx runs fn inside a transaction, committing on success and rolling back on any error.
// Because the database uses a single connection, the transaction holds that connection for its
// whole duration, fully serializing it against other writers — so check-then-write sequences
// (uniqueness, cycle guards, restore conflict detection) are race-free.
func (s *Store) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// collectStrings runs a single-column string query and returns all values.
func collectStrings(ctx context.Context, q querier, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
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

// ptr returns a non-NULL sql.NullString, or the zero value when s is empty (NULL = root).
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id                    TEXT PRIMARY KEY,
    username              TEXT NOT NULL UNIQUE COLLATE NOCASE,
    is_admin              INTEGER NOT NULL DEFAULT 0,
    status                TEXT NOT NULL DEFAULT 'active',
    kdf_time              INTEGER NOT NULL,
    kdf_memory            INTEGER NOT NULL,
    kdf_threads           INTEGER NOT NULL,
    pw_salt               BLOB NOT NULL,
    pw_wrapped_dek        BLOB NOT NULL,
    rec_salt              BLOB,
    rec_wrapped_dek       BLOB,
    must_change_password  INTEGER NOT NULL DEFAULT 1,
    created_at            INTEGER NOT NULL,
    updated_at            INTEGER NOT NULL,
    language              TEXT
);

-- One trash event per delete action. Trashing a folder records a single event spanning the
-- whole subtree it carried at that moment; every soft-deleted row points at its event so
-- restore/purge can act on the group, and independent deletions stay independent.
CREATE TABLE IF NOT EXISTS trash_events (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    root_kind  TEXT NOT NULL,                       -- 'folder' | 'document' | 'signature' | 'export'
    root_id    TEXT NOT NULL,                       -- the row the user deleted
    label      TEXT NOT NULL,                       -- its name at delete time, for display
    created_at INTEGER NOT NULL
);

-- Folders form a per-kind tree (a folder holds documents OR signatures, never both). parent_id
-- NULL means a top-level folder; folder_id NULL on an item means it lives at the root.
CREATE TABLE IF NOT EXISTS folders (
    id             TEXT PRIMARY KEY,
    user_id        TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind           TEXT NOT NULL,                   -- 'document' | 'signature'
    -- SET NULL (not CASCADE): purging one trash event deletes its own folder rows by
    -- trash_event_id; a child trashed in a *different* event must survive (it just loses its
    -- now-removed parent and floats to that event's top level) rather than be cascade-deleted.
    parent_id      TEXT REFERENCES folders(id) ON DELETE SET NULL,
    name           TEXT NOT NULL,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    deleted_at     INTEGER,
    trash_event_id TEXT REFERENCES trash_events(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS signatures (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    blob_path  TEXT NOT NULL,
    byte_size  INTEGER NOT NULL,
    width      INTEGER NOT NULL DEFAULT 0,
    height     INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    deleted_at INTEGER
);

CREATE TABLE IF NOT EXISTS documents (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    blob_path   TEXT NOT NULL,
    byte_size   INTEGER NOT NULL,
    page_count  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    updated_at  INTEGER NOT NULL,
    deleted_at  INTEGER
);

CREATE TABLE IF NOT EXISTS exports (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_id TEXT REFERENCES documents(id) ON DELETE SET NULL,
    name        TEXT NOT NULL,
    blob_path   TEXT NOT NULL,
    byte_size   INTEGER NOT NULL,
    page_count  INTEGER NOT NULL DEFAULT 0,
    created_at  INTEGER NOT NULL,
    deleted_at  INTEGER
);

CREATE INDEX IF NOT EXISTS idx_signatures_user ON signatures(user_id);
CREATE INDEX IF NOT EXISTS idx_documents_user  ON documents(user_id);
CREATE INDEX IF NOT EXISTS idx_exports_user    ON exports(user_id);
CREATE INDEX IF NOT EXISTS idx_folders_user      ON folders(user_id, kind, parent_id);
CREATE INDEX IF NOT EXISTS idx_trash_events_user ON trash_events(user_id, created_at);
`
