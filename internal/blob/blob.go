// Package blob stores encrypted user content as files on disk. Plaintext never touches
// the filesystem: callers hand in a plaintext reader and the per-user DEK, and the blob is
// encrypted (chunked AES-256-GCM) on its way to a temp file that is fsync'd and atomically
// renamed into place. Reads decrypt streaming back to the caller.
package blob

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"docs-sign/internal/crypto"
)

// Store manages encrypted blobs rooted at a directory.
type Store struct {
	root string
}

// New creates a blob store rooted at root, creating the directory if needed.
func New(root string) (*Store, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Store{root: abs}, nil
}

// RelPath returns the canonical relative storage path for a user's blob.
func RelPath(userID, blobID string) string {
	return userID + "/" + blobID + ".enc"
}

func (s *Store) abs(relPath string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(relPath))
	full := filepath.Join(s.root, clean)
	// Guard against path traversal from unexpected input.
	if full != s.root && !strings.HasPrefix(full, s.root+string(os.PathSeparator)) {
		return "", errors.New("blob: path escapes store root")
	}
	return full, nil
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// Write encrypts everything from r under dek and stores it at RelPath(userID, blobID).
// It returns the relative path and the plaintext byte size.
func (s *Store) Write(userID, blobID string, dek []byte, r io.Reader) (string, int64, error) {
	rel := RelPath(userID, blobID)
	full, err := s.abs(rel)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
		return "", 0, err
	}

	tmp, err := os.CreateTemp(filepath.Dir(full), "."+blobID+"-*.tmp")
	if err != nil {
		return "", 0, err
	}
	tmpName := tmp.Name()
	// Best-effort cleanup if we don't make it to the rename.
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return "", 0, err
	}

	cr := &countingReader{r: r}
	if err := crypto.EncryptStream(dek, cr, tmp); err != nil {
		return "", 0, err
	}
	if err := tmp.Sync(); err != nil {
		return "", 0, err
	}
	if err := tmp.Close(); err != nil {
		return "", 0, err
	}
	if err := os.Rename(tmpName, full); err != nil {
		return "", 0, err
	}
	committed = true
	return rel, cr.n, nil
}

// WriteBytes is a convenience wrapper around Write for in-memory data.
func (s *Store) WriteBytes(userID, blobID string, dek, data []byte) (string, int64, error) {
	return s.Write(userID, blobID, dek, bytes.NewReader(data))
}

// DecryptTo streams the decrypted contents of relPath to w.
func (s *Store) DecryptTo(relPath string, dek []byte, w io.Writer) error {
	full, err := s.abs(relPath)
	if err != nil {
		return err
	}
	f, err := os.Open(full)
	if err != nil {
		return err
	}
	defer f.Close()
	return crypto.DecryptStream(dek, f, w)
}

// ReadAll decrypts relPath fully into memory (used by the PDF pipeline, which needs the
// whole document).
func (s *Store) ReadAll(relPath string, dek []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := s.DecryptTo(relPath, dek, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Delete removes the encrypted file at relPath. A missing file is not an error.
func (s *Store) Delete(relPath string) error {
	full, err := s.abs(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// RemoveUserDir deletes all blobs for a user (used when an account is deleted).
func (s *Store) RemoveUserDir(userID string) error {
	full, err := s.abs(userID)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
