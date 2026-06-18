package blob

import (
	"bytes"
	"path/filepath"
	"testing"

	"docs-sign/internal/crypto"
)

func TestBlobRoundTrip(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	dek, _ := crypto.GenerateDEK()
	data := bytes.Repeat([]byte("signature-png-bytes"), 10000)

	rel, size, err := st.WriteBytes("user1", "blobA", dek, data)
	if err != nil {
		t.Fatal(err)
	}
	if size != int64(len(data)) {
		t.Fatalf("size=%d want %d", size, len(data))
	}
	got, err := st.ReadAll(rel, dek)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("round-trip mismatch")
	}

	// Wrong key must fail.
	other, _ := crypto.GenerateDEK()
	if _, err := st.ReadAll(rel, other); err == nil {
		t.Fatal("expected error decrypting with wrong key")
	}

	if err := st.Delete(rel); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ReadAll(rel, dek); err == nil {
		t.Fatal("expected error reading deleted blob")
	}
}

func TestBlobPathTraversal(t *testing.T) {
	st, _ := New(filepath.Join(t.TempDir(), "blobs"))
	if _, err := st.abs("../../etc/passwd"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}
