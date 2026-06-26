package store

import (
	"context"
	"database/sql"
	"testing"

	"docs-sign/internal/crypto"
)

// --- test helpers ---

func mustUser(t *testing.T, s *Store) string {
	t.Helper()
	salt, _ := crypto.NewSalt()
	uid := NewID()
	if err := s.CreateUser(context.Background(), &User{
		ID: uid, Username: "u" + uid[:10], Status: StatusActive, KDF: crypto.DefaultKDFParams(),
		PwSalt: salt, PwWrappedDEK: []byte("w"),
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return uid
}

func mkFolder(t *testing.T, s *Store, uid, kind string, parent sql.NullString, name string) string {
	t.Helper()
	f := &Folder{ID: NewID(), UserID: uid, Kind: kind, ParentID: parent, Name: name}
	if err := s.CreateFolder(context.Background(), f); err != nil {
		t.Fatalf("CreateFolder %q: %v", name, err)
	}
	return f.ID
}

func mkDoc(t *testing.T, s *Store, uid string, folder sql.NullString, name string) *Document {
	t.Helper()
	d := &Document{ID: NewID(), UserID: uid, Name: name, BlobPath: "blob-" + NewID(), ByteSize: 100, PageCount: 1, FolderID: folder}
	if err := s.CreateDocument(context.Background(), d); err != nil {
		t.Fatalf("CreateDocument %q: %v", name, err)
	}
	return d
}

func mkSig(t *testing.T, s *Store, uid string, folder sql.NullString, name string) *Signature {
	t.Helper()
	sig := &Signature{ID: NewID(), UserID: uid, Name: name, BlobPath: "blob-" + NewID(), ByteSize: 50, Width: 10, Height: 10, FolderID: folder}
	if err := s.CreateSignature(context.Background(), sig); err != nil {
		t.Fatalf("CreateSignature %q: %v", name, err)
	}
	return sig
}

func docNames(t *testing.T, s *Store, uid string, folder sql.NullString) map[string]bool {
	t.Helper()
	list, err := s.ListDocuments(context.Background(), uid, folder)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, d := range list {
		out[d.Name] = true
	}
	return out
}

func folderNames(t *testing.T, s *Store, uid, kind string, parent sql.NullString) map[string]bool {
	t.Helper()
	list, err := s.ListFolders(context.Background(), uid, kind, parent)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, f := range list {
		out[f.Name] = true
	}
	return out
}

// --- folders ---

func TestFolderTreeAndUniqueness(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	contracts := mkFolder(t, s, uid, KindDocument, root, "Contracts")
	acme := mkFolder(t, s, uid, KindDocument, nullString(contracts), "Acme")
	mkFolder(t, s, uid, KindDocument, nullString(acme), "2026")

	// Breadcrumb path top-down.
	path, err := s.FolderPath(ctx, uid, acme)
	if err != nil || len(path) != 2 || path[0].Name != "Contracts" || path[1].Name != "Acme" {
		t.Fatalf("FolderPath=%+v err=%v", path, err)
	}

	// Same name under the same parent is rejected; under a different parent it is fine.
	if err := s.CreateFolder(ctx, &Folder{ID: NewID(), UserID: uid, Kind: KindDocument, ParentID: root, Name: "Contracts"}); err != ErrNameConflict {
		t.Fatalf("dup sibling: want ErrNameConflict, got %v", err)
	}
	mkFolder(t, s, uid, KindDocument, nullString(acme), "Contracts") // ok: different parent

	// The signature tree is independent — a same-named folder there is fine.
	mkFolder(t, s, uid, KindSignature, root, "Contracts")

	// A folder under a non-existent or wrong-kind parent is not found.
	if err := s.CreateFolder(ctx, &Folder{ID: NewID(), UserID: uid, Kind: KindDocument, ParentID: nullString("nope"), Name: "x"}); err != ErrNotFound {
		t.Fatalf("bad parent: want ErrNotFound, got %v", err)
	}
	sigFolderInDocTree := nullString(mkFolder(t, s, uid, KindSignature, root, "S"))
	if err := s.CreateFolder(ctx, &Folder{ID: NewID(), UserID: uid, Kind: KindDocument, ParentID: sigFolderInDocTree, Name: "x"}); err != ErrNotFound {
		t.Fatalf("wrong-kind parent: want ErrNotFound, got %v", err)
	}
}

func TestFolderRenameAndMove(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	a := mkFolder(t, s, uid, KindDocument, root, "A")
	b := mkFolder(t, s, uid, KindDocument, nullString(a), "B")
	c := mkFolder(t, s, uid, KindDocument, nullString(b), "C")
	mkFolder(t, s, uid, KindDocument, root, "Taken")

	// Rename collision against a root sibling.
	other := mkFolder(t, s, uid, KindDocument, root, "Other")
	if err := s.RenameFolder(ctx, uid, other, "Taken"); err != ErrNameConflict {
		t.Fatalf("rename collision: want ErrNameConflict, got %v", err)
	}
	if err := s.RenameFolder(ctx, uid, other, "Renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}

	// Cannot move a folder into itself or one of its descendants.
	if err := s.MoveFolder(ctx, uid, a, nullString(a)); err != ErrInvalidMove {
		t.Fatalf("move into self: want ErrInvalidMove, got %v", err)
	}
	if err := s.MoveFolder(ctx, uid, a, nullString(c)); err != ErrInvalidMove {
		t.Fatalf("move into descendant: want ErrInvalidMove, got %v", err)
	}

	// Moving B (with C under it) up to root is fine; C rides along.
	if err := s.MoveFolder(ctx, uid, b, root); err != nil {
		t.Fatalf("move to root: %v", err)
	}
	if !folderNames(t, s, uid, KindDocument, root)["B"] {
		t.Fatal("B should now be at root")
	}
	if !folderNames(t, s, uid, KindDocument, nullString(b))["C"] {
		t.Fatal("C should still be under B")
	}

	// Move collision: a sibling with the target name already exists at the destination.
	mkFolder(t, s, uid, KindDocument, nullString(a), "B")
	if err := s.MoveFolder(ctx, uid, b, nullString(a)); err != ErrNameConflict {
		t.Fatalf("move collision: want ErrNameConflict, got %v", err)
	}
}

// --- items in folders ---

func TestItemFolderPlacementAndUniqueness(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	f := nullString(mkFolder(t, s, uid, KindDocument, root, "F"))
	g := nullString(mkFolder(t, s, uid, KindDocument, root, "G"))

	mkDoc(t, s, uid, f, "a.pdf")
	// Root listing excludes foldered items.
	if len(docNames(t, s, uid, root)) != 0 {
		t.Fatal("root should have no documents")
	}
	if !docNames(t, s, uid, f)["a.pdf"] {
		t.Fatal("a.pdf should be in F")
	}

	// Same name in the same folder is rejected; same name in another folder is fine.
	if err := s.CreateDocument(ctx, &Document{ID: NewID(), UserID: uid, Name: "a.pdf", BlobPath: "x", FolderID: f}); err != ErrNameConflict {
		t.Fatalf("dup in folder: want ErrNameConflict, got %v", err)
	}
	mkDoc(t, s, uid, g, "a.pdf") // ok

	// Move into a folder that already holds the name is rejected.
	d2 := mkDoc(t, s, uid, root, "a.pdf")
	if err := s.MoveDocument(ctx, uid, d2.ID, f); err != ErrNameConflict {
		t.Fatalf("move collision: want ErrNameConflict, got %v", err)
	}
	// Moving to a free folder works; rename collision is likewise rejected.
	emptyF := nullString(mkFolder(t, s, uid, KindDocument, root, "Empty"))
	if err := s.MoveDocument(ctx, uid, d2.ID, emptyF); err != nil {
		t.Fatalf("move: %v", err)
	}
	mkDoc(t, s, uid, emptyF, "keep.pdf")
	if err := s.RenameDocument(ctx, uid, d2.ID, "keep.pdf"); err != ErrNameConflict {
		t.Fatalf("rename collision: want ErrNameConflict, got %v", err)
	}

	// A document folder may not hold a signature, and vice versa.
	if err := s.CreateSignature(ctx, &Signature{ID: NewID(), UserID: uid, Name: "s", BlobPath: "y", FolderID: f}); err != ErrNotFound {
		t.Fatalf("signature into document folder: want ErrNotFound, got %v", err)
	}
}

// --- trash: independence + walking ---

func TestTrashEventsAreIndependent(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	docs := nullString(mkFolder(t, s, uid, KindDocument, root, "docs"))
	a := mkDoc(t, s, uid, docs, "a.txt")
	mkDoc(t, s, uid, docs, "b.txt")

	// Trash a.txt, then the whole docs folder — two independent events.
	if _, err := s.TrashNode(ctx, uid, KindDocument, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.TrashNode(ctx, uid, KindFolder, docs.String); err != nil {
		t.Fatal(err)
	}
	events, _ := s.ListTrashEvents(ctx, uid)
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d: %+v", len(events), events)
	}

	// Restoring the docs folder must leave a.txt in the trash (it belongs to the other event).
	var folderEvent TrashEventSummary
	for _, e := range events {
		if e.RootKind == KindFolder {
			folderEvent = e
		}
	}
	conflicts, err := s.RestoreNode(ctx, uid, KindFolder, folderEvent.RootID, nil)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("restore docs: conflicts=%v err=%v", conflicts, err)
	}
	if !folderNames(t, s, uid, KindDocument, root)["docs"] {
		t.Fatal("docs folder should be restored")
	}
	if docNames(t, s, uid, docs)["a.txt"] {
		t.Fatal("a.txt must NOT have been restored")
	}
	if !docNames(t, s, uid, docs)["b.txt"] {
		t.Fatal("b.txt should be restored with its folder")
	}
	if remaining, _ := s.ListTrashEvents(ctx, uid); len(remaining) != 1 || remaining[0].RootKind != KindDocument {
		t.Fatalf("a.txt's event should remain, got %+v", remaining)
	}
}

func TestTrashWalkAndDeepRestore(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	docs := mkFolder(t, s, uid, KindDocument, root, "docs")
	sub := mkFolder(t, s, uid, KindDocument, nullString(docs), "sub")
	deep := mkDoc(t, s, uid, nullString(sub), "deep.txt")
	mkDoc(t, s, uid, nullString(docs), "top.txt")

	ev, err := s.TrashNode(ctx, uid, KindFolder, docs)
	if err != nil {
		t.Fatal(err)
	}

	// Walk the event: docs -> {sub, top.txt}; sub -> {deep.txt}.
	top, _ := s.ListTrashChildren(ctx, uid, ev, nullString(docs))
	var sawSub, sawTop bool
	for _, c := range top {
		if c.IsFolder && c.Name == "sub" {
			sawSub = true
		}
		if !c.IsFolder && c.Name == "top.txt" {
			sawTop = true
		}
	}
	if !sawSub || !sawTop {
		t.Fatalf("walk docs: sub=%v top=%v entries=%+v", sawSub, sawTop, top)
	}
	deepKids, _ := s.ListTrashChildren(ctx, uid, ev, nullString(sub))
	if len(deepKids) != 1 || deepKids[0].Name != "deep.txt" {
		t.Fatalf("walk sub: %+v", deepKids)
	}

	// Restore the deep file independently: it reconstructs /docs/sub as active folders and
	// leaves the rest of the event in the trash.
	conflicts, err := s.RestoreNode(ctx, uid, KindDocument, deep.ID, nil)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("deep restore: conflicts=%v err=%v", conflicts, err)
	}
	if !folderNames(t, s, uid, KindDocument, root)["docs"] {
		t.Fatal("docs should have been recreated active")
	}
	var newDocs string
	for _, f := range mustFolders(t, s, uid, KindDocument, root) {
		if f.Name == "docs" {
			newDocs = f.ID
		}
	}
	subs := mustFolders(t, s, uid, KindDocument, nullString(newDocs))
	if len(subs) != 1 || subs[0].Name != "sub" {
		t.Fatalf("expected active sub under docs, got %+v", subs)
	}
	if !docNames(t, s, uid, nullString(subs[0].ID))["deep.txt"] {
		t.Fatal("deep.txt should be active under the reconstructed sub")
	}
	// The original event still exists (top.txt + the trashed folders remain).
	if ev2, _ := s.ListTrashEvents(ctx, uid); len(ev2) != 1 {
		t.Fatalf("event should remain after deep restore, got %+v", ev2)
	}
}

func mustFolders(t *testing.T, s *Store, uid, kind string, parent sql.NullString) []Folder {
	t.Helper()
	list, err := s.ListFolders(context.Background(), uid, kind, parent)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// --- trash: restore-to-origin + merge ---

func TestRestoreToOrigin(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	f := nullString(mkFolder(t, s, uid, KindDocument, root, "F"))
	d := mkDoc(t, s, uid, f, "a.pdf")
	if _, err := s.TrashNode(ctx, uid, KindDocument, d.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RestoreNode(ctx, uid, KindDocument, d.ID, nil); err != nil {
		t.Fatal(err)
	}
	if !docNames(t, s, uid, f)["a.pdf"] {
		t.Fatal("a.pdf should return to its origin folder F")
	}
}

func TestRestoreMergesFoldersByName(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	uid := mustUser(t, s)

	contracts := mkFolder(t, s, uid, KindDocument, root, "Contracts")
	mkDoc(t, s, uid, nullString(contracts), "nda.pdf")
	if _, err := s.TrashNode(ctx, uid, KindFolder, contracts); err != nil {
		t.Fatal(err)
	}

	// Recreate an active Contracts with a different file, then restore the trashed one.
	contracts2 := nullString(mkFolder(t, s, uid, KindDocument, root, "Contracts"))
	mkDoc(t, s, uid, contracts2, "other.pdf")

	conflicts, err := s.RestoreNode(ctx, uid, KindFolder, contracts, nil)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("merge restore: conflicts=%v err=%v", conflicts, err)
	}
	// Exactly one active Contracts, now holding both files.
	roots := mustFolders(t, s, uid, KindDocument, root)
	if len(roots) != 1 || roots[0].Name != "Contracts" {
		t.Fatalf("expected one merged Contracts, got %+v", roots)
	}
	names := docNames(t, s, uid, contracts2)
	if !names["nda.pdf"] || !names["other.pdf"] {
		t.Fatalf("merged folder should hold both files, got %v", names)
	}
	if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 0 {
		t.Fatalf("event should be cleared after full merge, got %+v", ev)
	}
}

// --- trash: file conflicts on restore ---

func TestRestoreFileConflictResolutions(t *testing.T) {
	ctx := context.Background()

	// helper to set up: a folder F with a trashed a.pdf and an active a.pdf blocking it.
	setup := func(t *testing.T) (*Store, string, sql.NullString, *Document, *Document) {
		s := newTestStore(t)
		uid := mustUser(t, s)
		f := nullString(mkFolder(t, s, uid, KindDocument, root, "F"))
		trashed := mkDoc(t, s, uid, f, "a.pdf")
		if _, err := s.TrashNode(ctx, uid, KindDocument, trashed.ID); err != nil {
			t.Fatal(err)
		}
		blocker := mkDoc(t, s, uid, f, "a.pdf") // now active a.pdf blocks the restore
		return s, uid, f, trashed, blocker
	}

	t.Run("unresolved reports conflict and changes nothing", func(t *testing.T) {
		s, uid, f, trashed, _ := setup(t)
		conflicts, err := s.RestoreNode(ctx, uid, KindDocument, trashed.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(conflicts) != 1 || conflicts[0].ID != trashed.ID || conflicts[0].DestPath != "/F" {
			t.Fatalf("want one conflict at /F, got %+v", conflicts)
		}
		// Nothing changed: still exactly one active a.pdf, trashed one still trashed.
		if got := s.ListDocuments; len(mustList(t, got, uid, f)) != 1 {
			t.Fatal("blocker should be untouched")
		}
		if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 1 {
			t.Fatal("trashed item should remain in trash")
		}
	})

	t.Run("skip leaves the item trashed", func(t *testing.T) {
		s, uid, f, trashed, _ := setup(t)
		conflicts, err := s.RestoreNode(ctx, uid, KindDocument, trashed.ID,
			map[string]Resolution{trashed.ID: {Action: ResolveSkip}})
		if err != nil || len(conflicts) != 0 {
			t.Fatalf("skip: conflicts=%v err=%v", conflicts, err)
		}
		if len(mustList(t, s.ListDocuments, uid, f)) != 1 {
			t.Fatal("folder should still have just the blocker")
		}
		if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 1 {
			t.Fatal("skipped item should remain trashed")
		}
	})

	t.Run("rename restores under a new name", func(t *testing.T) {
		s, uid, f, trashed, _ := setup(t)
		conflicts, err := s.RestoreNode(ctx, uid, KindDocument, trashed.ID,
			map[string]Resolution{trashed.ID: {Action: ResolveRename, NewName: "a-restored.pdf"}})
		if err != nil || len(conflicts) != 0 {
			t.Fatalf("rename: conflicts=%v err=%v", conflicts, err)
		}
		names := docNames(t, s, uid, f)
		if !names["a.pdf"] || !names["a-restored.pdf"] {
			t.Fatalf("want both names present, got %v", names)
		}
		if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 0 {
			t.Fatal("event should be cleared after rename restore")
		}
	})

	t.Run("override displaces the blocker into its own new event", func(t *testing.T) {
		s, uid, f, trashed, blocker := setup(t)
		conflicts, err := s.RestoreNode(ctx, uid, KindDocument, trashed.ID,
			map[string]Resolution{trashed.ID: {Action: ResolveOverride}})
		if err != nil || len(conflicts) != 0 {
			t.Fatalf("override: conflicts=%v err=%v", conflicts, err)
		}
		// The folder holds exactly one active a.pdf, and it is the restored one.
		list := mustList(t, s.ListDocuments, uid, f)
		if len(list) != 1 || list[0].ID != trashed.ID {
			t.Fatalf("restored item should occupy the name, got %+v", list)
		}
		// The displaced blocker now sits in a fresh trash event.
		ev, _ := s.ListTrashEvents(ctx, uid)
		if len(ev) != 1 || ev[0].RootID != blocker.ID {
			t.Fatalf("blocker should be in its own new event, got %+v", ev)
		}
	})
}

func mustList(t *testing.T, fn func(context.Context, string, sql.NullString) ([]Document, error), uid string, folder sql.NullString) []Document {
	t.Helper()
	list, err := fn(context.Background(), uid, folder)
	if err != nil {
		t.Fatal(err)
	}
	return list
}

// Restoring a folder that merges into an existing same-name folder must surface conflicts only
// for the files that collide, leaving the non-colliding ones to merge, and honor per-file
// resolutions inside the merge.
func TestRestoreFolderMergeWithFileConflict(t *testing.T) {
	ctx := context.Background()

	// trashed folder F holding {a.pdf, b.pdf}; active folder F holding a.pdf.
	setup := func(t *testing.T) (*Store, string, sql.NullString, *Document) {
		s := newTestStore(t)
		uid := mustUser(t, s)
		f := mkFolder(t, s, uid, KindDocument, root, "F")
		aTrashed := mkDoc(t, s, uid, nullString(f), "a.pdf")
		mkDoc(t, s, uid, nullString(f), "b.pdf")
		if _, err := s.TrashNode(ctx, uid, KindFolder, f); err != nil {
			t.Fatal(err)
		}
		activeF := nullString(mkFolder(t, s, uid, KindDocument, root, "F"))
		mkDoc(t, s, uid, activeF, "a.pdf")
		return s, uid, activeF, aTrashed
	}

	t.Run("unresolved conflict rolls the whole merge back", func(t *testing.T) {
		s, uid, activeF, aTrashed := setup(t)
		conflicts, err := s.RestoreNode(ctx, uid, KindFolder, folderOf(t, s, uid, aTrashed.ID), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(conflicts) != 1 || conflicts[0].Name != "a.pdf" || conflicts[0].DestPath != "/F" {
			t.Fatalf("want one a.pdf conflict at /F, got %+v", conflicts)
		}
		// Nothing merged: active F still holds only its own a.pdf, event intact.
		if len(mustList(t, s.ListDocuments, uid, activeF)) != 1 {
			t.Fatal("active folder should be unchanged after rollback")
		}
		if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 1 {
			t.Fatal("trashed folder event should remain intact")
		}
	})

	t.Run("skip merges the rest and keeps the conflicting file trashed", func(t *testing.T) {
		s, uid, activeF, aTrashed := setup(t)
		conflicts, err := s.RestoreNode(ctx, uid, KindFolder, folderOf(t, s, uid, aTrashed.ID),
			map[string]Resolution{aTrashed.ID: {Action: ResolveSkip}})
		if err != nil || len(conflicts) != 0 {
			t.Fatalf("skip: conflicts=%v err=%v", conflicts, err)
		}
		names := docNames(t, s, uid, activeF)
		if !names["a.pdf"] || !names["b.pdf"] || len(names) != 2 {
			t.Fatalf("b.pdf should merge in, a.pdf stay, got %v", names)
		}
		// The trashed folder still holds the skipped a.pdf, so its event survives.
		if ev, _ := s.ListTrashEvents(ctx, uid); len(ev) != 1 {
			t.Fatalf("event with the skipped file should remain, got %+v", ev)
		}
	})

	t.Run("override displaces the active file and merges the folder away", func(t *testing.T) {
		s, uid, activeF, aTrashed := setup(t)
		conflicts, err := s.RestoreNode(ctx, uid, KindFolder, folderOf(t, s, uid, aTrashed.ID),
			map[string]Resolution{aTrashed.ID: {Action: ResolveOverride}})
		if err != nil || len(conflicts) != 0 {
			t.Fatalf("override: conflicts=%v err=%v", conflicts, err)
		}
		if names := docNames(t, s, uid, activeF); !names["a.pdf"] || !names["b.pdf"] || len(names) != 2 {
			t.Fatalf("folder should hold both files after override, got %v", names)
		}
		// Original folder event fully merged away; only the displaced file's new event remains.
		ev, _ := s.ListTrashEvents(ctx, uid)
		if len(ev) != 1 || ev[0].RootKind != KindDocument {
			t.Fatalf("only the displaced file's event should remain, got %+v", ev)
		}
	})
}

func TestEnsureFolderPath(t *testing.T) {
	s := newTestStore(t)
	uid := mustUser(t, s)
	ctx := context.Background()
	root := sql.NullString{}

	// A fresh path creates every level and returns the leaf.
	leaf, err := s.EnsureFolderPath(ctx, uid, KindDocument, root, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("ensure fresh path: %v", err)
	}
	path, err := s.FolderPath(ctx, uid, leaf)
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{path[0].Name, path[1].Name, path[2].Name}; got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Fatalf("unexpected leaf path: %+v", got)
	}

	// Re-ensuring the same path reuses the existing folders (no duplicates, same leaf).
	leaf2, err := s.EnsureFolderPath(ctx, uid, KindDocument, root, []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("ensure existing path: %v", err)
	}
	if leaf2 != leaf {
		t.Fatalf("expected the same leaf on re-ensure, got %q want %q", leaf2, leaf)
	}
	if names := folderNames(t, s, uid, KindDocument, root); len(names) != 1 || !names["a"] {
		t.Fatalf("expected a single top-level folder 'a', got %+v", names)
	}

	// A diverging path reuses the shared prefix and only creates the new tail.
	if _, err := s.EnsureFolderPath(ctx, uid, KindDocument, root, []string{"a", "b", "d"}); err != nil {
		t.Fatalf("ensure diverging path: %v", err)
	}
	bID, found, err := findActiveFolderByName(ctx, s.db, uid, KindDocument, root, "a")
	if err != nil || !found {
		t.Fatalf("locate 'a': found=%v err=%v", found, err)
	}
	aChildren := folderNames(t, s, uid, KindDocument, sql.NullString{String: bID, Valid: true})
	if len(aChildren) != 1 || !aChildren["b"] {
		t.Fatalf("expected 'a' to hold only 'b', got %+v", aChildren)
	}

	// Whitespace-only segments are skipped, so an all-blank path resolves to the parent itself.
	parent := mkFolder(t, s, uid, KindDocument, root, "parent")
	leaf3, err := s.EnsureFolderPath(ctx, uid, KindDocument,
		sql.NullString{String: parent, Valid: true}, []string{"  ", ""})
	if err != nil {
		t.Fatalf("ensure blank path: %v", err)
	}
	if leaf3 != parent {
		t.Fatalf("blank path should resolve to the parent, got %q want %q", leaf3, parent)
	}
}

// folderOf returns the (possibly trashed) folder id that contains an item — used to address a
// trashed folder by its root for restore.
func folderOf(t *testing.T, s *Store, uid, itemID string) string {
	t.Helper()
	var folderID sql.NullString
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT folder_id FROM documents WHERE id=? AND user_id=?`, itemID, uid).Scan(&folderID); err != nil {
		t.Fatal(err)
	}
	return folderID.String
}
