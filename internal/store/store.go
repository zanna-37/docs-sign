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
	// Idempotent upgrade for databases created before soft-delete was added.
	for _, table := range []string{"signatures", "documents", "exports"} {
		if err := s.ensureColumn(ctx, table, "deleted_at", "INTEGER"); err != nil {
			return err
		}
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
    updated_at            INTEGER NOT NULL
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
`
