package store

import (
	"context"
	"testing"
	"time"
)

func contains(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func TestHardDeleteEventReturnsAllBlobs(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	d := mkDoc(t, s, uid, root, "d.pdf")
	exp := &Export{ID: NewID(), UserID: uid, Name: "d (signed)", BlobPath: "blob-exp", ByteSize: 200, PageCount: 1}
	exp.DocumentID.Valid, exp.DocumentID.String = true, d.ID
	if err := s.CreateExport(ctx, exp); err != nil {
		t.Fatal(err)
	}

	ev, err := s.TrashNode(ctx, uid, KindDocument, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	paths, err := s.HardDeleteEvent(ctx, uid, ev)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(paths, d.BlobPath) || !contains(paths, "blob-exp") {
		t.Fatalf("expected document and export blobs, got %v", paths)
	}
	if ev2, _ := s.ListTrashEvents(ctx, uid); len(ev2) != 0 {
		t.Fatal("trash should be empty after purge")
	}
}

func TestEmptyAndPurgeExpired(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	d1 := mkDoc(t, s, uid, root, "1.pdf")
	d2 := mkDoc(t, s, uid, root, "2.pdf")
	if _, err := s.TrashNode(ctx, uid, KindDocument, d1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TrashNode(ctx, uid, KindDocument, d2.ID); err != nil {
		t.Fatal(err)
	}

	// A cutoff in the past purges nothing.
	if paths, _ := s.PurgeExpired(ctx, time.Now().Add(-time.Hour)); len(paths) != 0 {
		t.Fatalf("nothing should be expired yet, got %v", paths)
	}
	if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 2 {
		t.Fatalf("both events should remain, got %d", len(ev))
	}
	// A cutoff in the future expires both.
	paths, err := s.PurgeExpired(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !contains(paths, d1.BlobPath) || !contains(paths, d2.BlobPath) {
		t.Fatalf("expected both blobs, got %v", paths)
	}
	if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 0 {
		t.Fatal("all events should be purged")
	}

	// EmptyTrash clears everything for a user.
	d3 := mkDoc(t, s, uid, root, "3.pdf")
	if _, err := s.TrashNode(ctx, uid, KindDocument, d3.ID); err != nil {
		t.Fatal(err)
	}
	if paths, _ := s.EmptyTrash(ctx, uid); !contains(paths, d3.BlobPath) {
		t.Fatalf("empty trash should free 3.pdf blob, got %v", paths)
	}
	if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 0 {
		t.Fatal("trash should be empty")
	}
}

// Purging one event must never reach into another event's rows, even when one trashed folder
// is nested under another that was trashed separately.
func TestCrossEventPurgeSafety(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	parent := mkFolder(t, s, uid, KindDocument, root, "parent")
	child := mkFolder(t, s, uid, KindDocument, nullString(parent), "child")
	x := mkDoc(t, s, uid, nullString(child), "x.pdf")

	eventA, err := s.TrashNode(ctx, uid, KindFolder, child) // child + x
	if err != nil {
		t.Fatal(err)
	}
	eventB, err := s.TrashNode(ctx, uid, KindFolder, parent) // parent only (child already trashed)
	if err != nil {
		t.Fatal(err)
	}

	paths, err := s.HardDeleteEvent(ctx, uid, eventB)
	if err != nil {
		t.Fatal(err)
	}
	if contains(paths, x.BlobPath) {
		t.Fatal("purging the parent event must not touch the child event's blob")
	}
	// Event A survives and is still walkable; restoring it lands at root (parent is gone).
	events, _ := s.ListTrashEvents(ctx, uid)
	if len(events) != 1 || events[0].EventID != eventA {
		t.Fatalf("child event should survive, got %+v", events)
	}
	kids, _ := s.ListTrashChildren(ctx, uid, eventA, nullString(child))
	if len(kids) != 1 || kids[0].Name != "x.pdf" {
		t.Fatalf("child event should still hold x.pdf, got %+v", kids)
	}
	if _, err := s.RestoreNode(ctx, uid, KindFolder, child, nil); err != nil {
		t.Fatal(err)
	}
	if !folderNames(t, s, uid, KindDocument, root)["child"] {
		t.Fatal("child should restore to root after its parent was purged")
	}
}

func TestExportTrashAndRestore(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	d := mkDoc(t, s, uid, root, "d.pdf")
	exp := &Export{ID: NewID(), UserID: uid, Name: "d (signed)", BlobPath: "blob-exp", ByteSize: 200, PageCount: 1}
	exp.DocumentID.Valid, exp.DocumentID.String = true, d.ID
	if err := s.CreateExport(ctx, exp); err != nil {
		t.Fatal(err)
	}

	ev, err := s.TrashNode(ctx, uid, KindExport, exp.ID)
	if err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListExports(ctx, uid); len(list) != 0 {
		t.Fatal("trashed export should not be listed")
	}
	events, _ := s.ListTrashEvents(ctx, uid)
	if len(events) != 1 || events[0].RootKind != KindExport || events[0].ByteSize != 200 || events[0].ItemCount != 1 {
		t.Fatalf("export event summary wrong: %+v", events)
	}
	if _, err := s.RestoreNode(ctx, uid, KindExport, exp.ID, nil); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListExports(ctx, uid); len(list) != 1 {
		t.Fatal("export should be restored")
	}
	if events, _ := s.ListTrashEvents(ctx, uid); len(events) != 0 {
		t.Fatal("event should be cleared")
	}
	_ = ev
}

func TestTrashEventAggregation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	docs := mkFolder(t, s, uid, KindDocument, root, "docs")
	mkDoc(t, s, uid, nullString(docs), "a.pdf") // 100 bytes
	mkDoc(t, s, uid, nullString(docs), "b.pdf") // 100 bytes

	if _, err := s.TrashNode(ctx, uid, KindFolder, docs); err != nil {
		t.Fatal(err)
	}
	events, _ := s.ListTrashEvents(ctx, uid)
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].RootKind != KindFolder || events[0].ItemCount != 2 || events[0].ByteSize != 200 {
		t.Fatalf("aggregation wrong: %+v", events[0])
	}
}

func TestOwnershipIsolation(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	a := mustUser(t, s)
	b := mustUser(t, s)

	fA := mkFolder(t, s, a, KindDocument, root, "A")
	dA := mkDoc(t, s, a, root, "x.pdf")

	if _, err := s.GetFolder(ctx, b, fA); err != ErrNotFound {
		t.Fatalf("foreign GetFolder: want ErrNotFound, got %v", err)
	}
	if len(mustFolders(t, s, b, KindDocument, root)) != 0 {
		t.Fatal("b should see no folders")
	}
	if _, err := s.TrashNode(ctx, b, KindFolder, fA); err != ErrNotFound {
		t.Fatalf("foreign trash folder: want ErrNotFound, got %v", err)
	}
	if _, err := s.TrashNode(ctx, b, KindDocument, dA.ID); err != ErrNotFound {
		t.Fatalf("foreign trash doc: want ErrNotFound, got %v", err)
	}
	if err := s.MoveFolder(ctx, b, fA, root); err != ErrNotFound {
		t.Fatalf("foreign move: want ErrNotFound, got %v", err)
	}

	ev, _ := s.TrashNode(ctx, a, KindDocument, dA.ID)
	if _, err := s.RestoreNode(ctx, b, KindDocument, dA.ID, nil); err != ErrNotFound {
		t.Fatalf("foreign restore: want ErrNotFound, got %v", err)
	}
	if _, err := s.HardDeleteEvent(ctx, b, ev); err != ErrNotFound {
		t.Fatalf("foreign purge: want ErrNotFound, got %v", err)
	}
}

func TestDeleteUserContentClearsFoldersAndEvents(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	f := mkFolder(t, s, uid, KindDocument, root, "F")
	mkDoc(t, s, uid, nullString(f), "a.pdf")
	d := mkDoc(t, s, uid, root, "b.pdf")
	if _, err := s.TrashNode(ctx, uid, KindDocument, d.ID); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteUserContent(ctx, uid); err != nil {
		t.Fatal(err)
	}
	if len(mustFolders(t, s, uid, KindDocument, root)) != 0 {
		t.Fatal("folders should be gone")
	}
	if len(mustList(t, s.ListDocuments, uid, nullString(f))) != 0 {
		t.Fatal("documents should be gone")
	}
	if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 0 {
		t.Fatal("trash events should be gone")
	}
}
