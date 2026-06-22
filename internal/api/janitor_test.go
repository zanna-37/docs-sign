package api

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"docs-sign/internal/pdfproc"
)

// countBlobs counts the encrypted blob files on disk.
func countBlobs(t *testing.T, dataDir string) int {
	t.Helper()
	n := 0
	_ = filepath.WalkDir(filepath.Join(dataDir, "blobs"), func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".enc") {
			n++
		}
		return nil
	})
	return n
}

// The wired trash janitor purges events past their retention window and deletes their encrypted
// blobs from disk; fresh events (within retention) are left alone.
func TestTrashJanitorPurgesExpiredOnly(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	ctx := context.Background()

	t.Run("expired event is purged and its blob freed", func(t *testing.T) {
		e := newTestEnv(t, renderer)
		// Negative retention makes any trashed event immediately past its window.
		e.srv.cfg.TrashRetention = -time.Hour
		e.setupAndLogin(t)

		doc := decode[documentDTO](t, e.upload(t, "/api/documents", "d.pdf", makePDF(t)), 201)
		if countBlobs(t, e.dataDir) != 1 {
			t.Fatalf("expected one blob after upload, got %d", countBlobs(t, e.dataDir))
		}
		mustStatus(t, e.doJSON(t, "DELETE", "/api/documents/"+doc.ID, nil), 200)
		if countBlobs(t, e.dataDir) != 1 {
			t.Fatal("trashing must keep the blob (soft delete)")
		}

		e.srv.purgeTrash(ctx)

		if ev := listTrash(t, e); len(ev.Events) != 0 {
			t.Fatalf("expired event should be purged, got %+v", ev.Events)
		}
		if countBlobs(t, e.dataDir) != 0 {
			t.Fatalf("purged blob should be deleted from disk, got %d", countBlobs(t, e.dataDir))
		}
	})

	t.Run("unexpired event and its blob are kept", func(t *testing.T) {
		e := newTestEnv(t, renderer)
		e.srv.cfg.TrashRetention = 30 * 24 * time.Hour
		e.setupAndLogin(t)

		doc := decode[documentDTO](t, e.upload(t, "/api/documents", "d.pdf", makePDF(t)), 201)
		mustStatus(t, e.doJSON(t, "DELETE", "/api/documents/"+doc.ID, nil), 200)

		e.srv.purgeTrash(ctx)

		if ev := listTrash(t, e); len(ev.Events) != 1 {
			t.Fatalf("fresh event must survive, got %+v", ev.Events)
		}
		if countBlobs(t, e.dataDir) != 1 {
			t.Fatalf("its blob must remain, got %d", countBlobs(t, e.dataDir))
		}
	})
}

// The janitor's reconcile pass must not delete a trashed-but-not-yet-purged blob — it is still
// referenced by a row and owned until the event is actually purged.
func TestTrashJanitorReconcileKeepsTrashedBlob(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()

	e := newTestEnv(t, renderer)
	e.srv.cfg.TrashRetention = 30 * 24 * time.Hour
	e.setupAndLogin(t)

	doc := decode[documentDTO](t, e.upload(t, "/api/documents", "d.pdf", makePDF(t)), 201)
	mustStatus(t, e.doJSON(t, "DELETE", "/api/documents/"+doc.ID, nil), 200)

	e.srv.reconcileBlobs(context.Background())

	if countBlobs(t, e.dataDir) != 1 {
		t.Fatalf("trashed-but-not-purged blob must be retained, got %d", countBlobs(t, e.dataDir))
	}
}
