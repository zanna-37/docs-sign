package store

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"docs-sign/internal/blob"
	"docs-sign/internal/crypto"
)

const day = int64(24 * 60 * 60)

func realBlobs(t *testing.T) (*blob.Store, string, []byte) {
	t.Helper()
	dir := t.TempDir()
	bs, err := blob.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatal(err)
	}
	return bs, dir, dek
}

// mkRealDoc writes a real encrypted blob and a document row pointing at it.
func mkRealDoc(t *testing.T, s *Store, bs *blob.Store, uid string, dek []byte, folder sql.NullString, name string) *Document {
	t.Helper()
	id := NewID()
	rel, size, err := bs.WriteBytes(uid, id, dek, []byte("pdf-"+name+"-"+id))
	if err != nil {
		t.Fatal(err)
	}
	d := &Document{ID: id, UserID: uid, Name: name, BlobPath: rel, ByteSize: size, PageCount: 1, FolderID: folder}
	if err := s.CreateDocument(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

// blobExists reports whether the encrypted file for a relative blob path is on disk.
func blobExists(t *testing.T, dir, rel string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel)))
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	t.Fatal(err)
	return false
}

// backdateEvent rewrites an event's delete time so its retention clock can be tested.
func backdateEvent(t *testing.T, s *Store, eventID string, unixSec int64) {
	t.Helper()
	if _, err := s.db.ExecContext(context.Background(),
		`UPDATE trash_events SET created_at=? WHERE id=?`, unixSec, eventID); err != nil {
		t.Fatal(err)
	}
}

// janitorPurge mirrors what the server janitor does: purge expired events, then delete the
// freed blob files from disk. It returns the freed paths.
func janitorPurge(t *testing.T, s *Store, bs *blob.Store, cutoff time.Time) []string {
	t.Helper()
	paths, err := s.PurgeExpired(context.Background(), cutoff)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if err := bs.Delete(p); err != nil {
			t.Fatal(err)
		}
	}
	return paths
}

// Each event expires on its own delete-time clock: PurgeExpired removes exactly the events
// strictly older than the cutoff, frees exactly their blobs, and leaves the rest intact.
func TestAutoPurgeRetentionBoundary(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bs, dir, dek := realBlobs(t)
	uid := mustUser(t, s)

	dOld := mkRealDoc(t, s, bs, uid, dek, root, "old.pdf")
	dMid := mkRealDoc(t, s, bs, uid, dek, root, "mid.pdf")
	dEdge := mkRealDoc(t, s, bs, uid, dek, root, "edge.pdf")
	dNew := mkRealDoc(t, s, bs, uid, dek, root, "new.pdf")

	evOld, _ := s.TrashNode(ctx, uid, KindDocument, dOld.ID)
	evMid, _ := s.TrashNode(ctx, uid, KindDocument, dMid.ID)
	evEdge, _ := s.TrashNode(ctx, uid, KindDocument, dEdge.ID)
	evNew, _ := s.TrashNode(ctx, uid, KindDocument, dNew.ID)

	now := time.Now().Unix()
	backdateEvent(t, s, evOld, now-10*day)
	backdateEvent(t, s, evMid, now-5*day)
	backdateEvent(t, s, evEdge, now-3*day) // exactly at the cutoff
	backdateEvent(t, s, evNew, now-1*day)

	// Cutoff three days ago: strictly-older events (old, mid) go; edge (==cutoff) and new stay.
	paths := janitorPurge(t, s, bs, time.Unix(now-3*day, 0))

	freed := map[string]bool{}
	for _, p := range paths {
		freed[p] = true
	}
	if len(freed) != 2 || !freed[dOld.BlobPath] || !freed[dMid.BlobPath] {
		t.Fatalf("expected exactly old+mid blobs freed, got %v", paths)
	}
	if blobExists(t, dir, dOld.BlobPath) || blobExists(t, dir, dMid.BlobPath) {
		t.Fatal("purged blobs should be gone from disk")
	}
	if !blobExists(t, dir, dEdge.BlobPath) || !blobExists(t, dir, dNew.BlobPath) {
		t.Fatal("not-yet-expired blobs must remain on disk")
	}
	events, _ := s.ListTrashEvents(ctx, uid)
	if len(events) != 2 {
		t.Fatalf("two events should remain (edge, new), got %d", len(events))
	}
	for _, e := range events {
		if e.EventID != evEdge && e.EventID != evNew {
			t.Fatalf("unexpected surviving event %s", e.EventID)
		}
	}
}

// Auto-purge must never touch active (non-trashed) content, however old.
func TestAutoPurgeLeavesActiveContent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bs, dir, dek := realBlobs(t)
	uid := mustUser(t, s)

	active := mkRealDoc(t, s, bs, uid, dek, root, "keep.pdf")
	doomed := mkRealDoc(t, s, bs, uid, dek, root, "gone.pdf")
	ev, _ := s.TrashNode(ctx, uid, KindDocument, doomed.ID)
	backdateEvent(t, s, ev, time.Now().Unix()-100*day)

	paths := janitorPurge(t, s, bs, time.Now()) // everything older than now
	if len(paths) != 1 || paths[0] != doomed.BlobPath {
		t.Fatalf("only the trashed blob should be freed, got %v", paths)
	}
	if !blobExists(t, dir, active.BlobPath) {
		t.Fatal("active document's blob must survive the purge")
	}
	if blobExists(t, dir, doomed.BlobPath) {
		t.Fatal("trashed-and-expired blob should be gone")
	}
	if list, _ := s.ListDocuments(ctx, uid, root); len(list) != 1 || list[0].ID != active.ID {
		t.Fatalf("active document row must remain, got %+v", list)
	}
}

// Purging a folder event removes the whole subtree and the signed copies that ride hidden with
// its documents, freeing every blob.
func TestAutoPurgeFolderSubtreeAndHiddenExports(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bs, dir, dek := realBlobs(t)
	uid := mustUser(t, s)

	f := mkFolder(t, s, uid, KindDocument, root, "F")
	sub := mkFolder(t, s, uid, KindDocument, nullString(f), "Sub")
	d := mkRealDoc(t, s, bs, uid, dek, nullString(sub), "d.pdf")

	// A signed export of d, riding hidden (not itself trashed).
	expID := NewID()
	erel, esize, err := bs.WriteBytes(uid, expID, dek, []byte("signed-pdf"))
	if err != nil {
		t.Fatal(err)
	}
	exp := &Export{ID: expID, UserID: uid, Name: "d (signed)", BlobPath: erel, ByteSize: esize, PageCount: 1}
	exp.DocumentID.Valid, exp.DocumentID.String = true, d.ID
	if err := s.CreateExport(ctx, exp); err != nil {
		t.Fatal(err)
	}

	ev, _ := s.TrashNode(ctx, uid, KindFolder, f)
	backdateEvent(t, s, ev, time.Now().Unix()-100*day)

	paths := janitorPurge(t, s, bs, time.Now())
	freed := map[string]bool{}
	for _, p := range paths {
		freed[p] = true
	}
	if !freed[d.BlobPath] || !freed[erel] {
		t.Fatalf("document and hidden export blobs must be freed, got %v", paths)
	}
	if blobExists(t, dir, d.BlobPath) || blobExists(t, dir, erel) {
		t.Fatal("subtree blobs should be gone from disk")
	}
	if events, _ := s.ListTrashEvents(ctx, uid); len(events) != 0 {
		t.Fatal("the folder event should be fully purged")
	}
	if _, err := s.GetExport(ctx, uid, expID); err != ErrNotFound {
		t.Fatal("the export row should be gone")
	}
}

// The janitor's reconcile pass must retain every referenced blob — including trashed-but-not-
// yet-purged ones — and only reap true orphans past the grace cutoff.
func TestReconcileRetainsReferencedAndReapsOrphans(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bs, dir, dek := realBlobs(t)
	uid := mustUser(t, s)

	active := mkRealDoc(t, s, bs, uid, dek, root, "active.pdf")
	trashed := mkRealDoc(t, s, bs, uid, dek, root, "trashed.pdf")
	if _, err := s.TrashNode(ctx, uid, KindDocument, trashed.ID); err != nil {
		t.Fatal(err) // trashed but NOT purged — its blob is still owned
	}
	orphanRel, _, err := bs.WriteBytes(uid, NewID(), dek, []byte("leaked"))
	if err != nil {
		t.Fatal(err)
	}

	referenced, err := s.AllReferencedBlobPaths(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Cutoff in the future so age is not what protects the live blobs — only their references.
	reaped, err := bs.ReconcileOrphans(referenced, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if reaped != 1 {
		t.Fatalf("only the orphan should be reaped, got %d", reaped)
	}
	if !blobExists(t, dir, active.BlobPath) || !blobExists(t, dir, trashed.BlobPath) {
		t.Fatal("active and trashed blobs must both be retained")
	}
	if blobExists(t, dir, orphanRel) {
		t.Fatal("orphan blob should have been reaped")
	}
}

// The grace cutoff protects a freshly written orphan (an upload mid-commit looks orphaned).
func TestReconcileSparesFreshOrphans(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bs, dir, dek := realBlobs(t)
	uid := mustUser(t, s)

	orphanRel, _, err := bs.WriteBytes(uid, NewID(), dek, []byte("in-flight"))
	if err != nil {
		t.Fatal(err)
	}
	referenced, err := s.AllReferencedBlobPaths(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// Cutoff an hour in the past: the fresh file is newer than it, so it must be spared.
	reaped, err := bs.ReconcileOrphans(referenced, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if reaped != 0 || !blobExists(t, dir, orphanRel) {
		t.Fatalf("fresh orphan must be spared, reaped=%d exists=%v", reaped, blobExists(t, dir, orphanRel))
	}
}

// Restoring an item before its event expires takes it out of the purge's reach entirely.
func TestAutoPurgeSkipsRestoredItems(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bs, dir, dek := realBlobs(t)
	uid := mustUser(t, s)

	a := mkRealDoc(t, s, bs, uid, dek, root, "a.pdf")
	b := mkRealDoc(t, s, bs, uid, dek, root, "b.pdf")
	evA, _ := s.TrashNode(ctx, uid, KindDocument, a.ID)
	evB, _ := s.TrashNode(ctx, uid, KindDocument, b.ID)
	backdateEvent(t, s, evA, time.Now().Unix()-100*day)
	backdateEvent(t, s, evB, time.Now().Unix()-100*day)

	// Restore A; its (now empty) event is gone, so the purge can't reach it.
	if _, err := s.RestoreNode(ctx, uid, KindDocument, a.ID, nil); err != nil {
		t.Fatal(err)
	}
	_ = evA

	paths := janitorPurge(t, s, bs, time.Now())
	if len(paths) != 1 || paths[0] != b.BlobPath {
		t.Fatalf("only B should be purged, got %v", paths)
	}
	if !blobExists(t, dir, a.BlobPath) {
		t.Fatal("restored A's blob must survive")
	}
	if list, _ := s.ListDocuments(ctx, uid, root); len(list) != 1 || list[0].ID != a.ID {
		t.Fatal("restored A should be active")
	}
	if events, _ := s.ListTrashEvents(ctx, uid); len(events) != 0 {
		t.Fatalf("B's event should be purged, got %+v", events)
	}
	_ = evB
}

// Purging is idempotent and per-event: an expired event goes while a fresh sibling stays, and a
// second sweep is a no-op.
func TestAutoPurgeIdempotentAndPerEvent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	bs, _, dek := realBlobs(t)
	uid := mustUser(t, s)

	old := mkRealDoc(t, s, bs, uid, dek, root, "old.pdf")
	fresh := mkRealDoc(t, s, bs, uid, dek, root, "fresh.pdf")
	evOld, _ := s.TrashNode(ctx, uid, KindDocument, old.ID)
	s.TrashNode(ctx, uid, KindDocument, fresh.ID)
	backdateEvent(t, s, evOld, time.Now().Unix()-100*day)

	// Cutoff between them: only the old event purges.
	if paths := janitorPurge(t, s, bs, time.Now().Add(-time.Hour)); len(paths) != 1 || paths[0] != old.BlobPath {
		t.Fatalf("first sweep should free only the old blob, got %v", paths)
	}
	if events, _ := s.ListTrashEvents(ctx, uid); len(events) != 1 {
		t.Fatalf("the fresh event should remain, got %d", len(events))
	}
	// A second identical sweep frees nothing more.
	if paths := janitorPurge(t, s, bs, time.Now().Add(-time.Hour)); len(paths) != 0 {
		t.Fatalf("second sweep should be a no-op, got %v", paths)
	}
}
