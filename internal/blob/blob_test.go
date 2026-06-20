package blob

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestReconcileOrphans(t *testing.T) {
	st, err := New(filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	dek, _ := crypto.GenerateDEK()
	data := []byte("blob")

	kept, _, _ := st.WriteBytes("user1", "kept", dek, data)       // referenced -> spared
	oldOrphan, _, _ := st.WriteBytes("user1", "old", dek, data)   // unreferenced + stale -> reaped
	freshOrphan, _, _ := st.WriteBytes("user2", "new", dek, data) // unreferenced but fresh -> spared

	// A temp file left by a crashed write is reaped once stale, but spared while fresh.
	staleTmp := "user1/.old-blob-abc.tmp"
	freshTmp := "user2/.new-blob-xyz.tmp"
	writeRaw := func(rel string) {
		full, _ := st.abs(rel)
		if err := os.WriteFile(full, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeRaw(staleTmp)
	writeRaw(freshTmp)

	now := time.Now()
	cutoff := now.Add(-time.Hour)
	// Age the stale entries past the cutoff; everything else keeps its fresh write time.
	stale := now.Add(-2 * time.Hour)
	for _, rel := range []string{oldOrphan, staleTmp} {
		full, _ := st.abs(rel)
		if err := os.Chtimes(full, stale, stale); err != nil {
			t.Fatal(err)
		}
	}

	referenced := map[string]struct{}{kept: {}}
	reaped, err := st.ReconcileOrphans(referenced, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if reaped != 2 {
		t.Fatalf("reaped=%d want 2", reaped)
	}

	exists := func(rel string) bool {
		full, _ := st.abs(rel)
		_, err := os.Stat(full)
		return err == nil
	}
	if !exists(kept) {
		t.Error("referenced blob was reaped")
	}
	if exists(oldOrphan) {
		t.Error("stale orphan was not reaped")
	}
	if !exists(freshOrphan) {
		t.Error("fresh orphan was reaped despite grace period")
	}
	if exists(staleTmp) {
		t.Error("stale temp leftover was not reaped")
	}
	if !exists(freshTmp) {
		t.Error("fresh temp leftover was reaped despite grace period")
	}
}

func TestBlobPathTraversal(t *testing.T) {
	st, _ := New(filepath.Join(t.TempDir(), "blobs"))
	if _, err := st.abs("../../etc/passwd"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}
