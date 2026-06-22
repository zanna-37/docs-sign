package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"docs-sign/internal/crypto"
)

// root is the zero NullString — an item or folder at the top level.
var root = sql.NullString{}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "meta.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestUserLifecycle(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	n, err := s.CountUsers(ctx)
	if err != nil || n != 0 {
		t.Fatalf("CountUsers=%d err=%v", n, err)
	}

	salt, _ := crypto.NewSalt()
	u := &User{
		ID:                 NewID(),
		Username:           "Alice",
		IsAdmin:            true,
		Status:             StatusActive,
		KDF:                crypto.DefaultKDFParams(),
		PwSalt:             salt,
		PwWrappedDEK:       []byte("wrapped"),
		MustChangePassword: true,
	}
	if err := s.CreateUser(ctx, u); err != nil {
		t.Fatal(err)
	}

	// Case-insensitive lookup.
	got, err := s.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID || !got.IsAdmin || !got.MustChangePassword {
		t.Fatalf("unexpected user: %+v", got)
	}

	admins, _ := s.CountAdmins(ctx)
	if admins != 1 {
		t.Fatalf("CountAdmins=%d", admins)
	}

	if _, err := s.GetUserByUsername(ctx, "nobody"); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSignatureCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	salt, _ := crypto.NewSalt()
	uid := NewID()
	if err := s.CreateUser(ctx, &User{
		ID: uid, Username: "bob", Status: StatusActive, KDF: crypto.DefaultKDFParams(),
		PwSalt: salt, PwWrappedDEK: []byte("w"),
	}); err != nil {
		t.Fatal(err)
	}

	sig := &Signature{ID: NewID(), UserID: uid, Name: "My sig", BlobPath: "p", ByteSize: 123, Width: 200, Height: 80}
	if err := s.CreateSignature(ctx, sig); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSignatures(ctx, uid, root)
	if err != nil || len(list) != 1 || list[0].Name != "My sig" {
		t.Fatalf("list=%v err=%v", list, err)
	}
	if err := s.RenameSignature(ctx, uid, sig.ID, "Renamed"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSignature(ctx, uid, sig.ID)
	if got.Name != "Renamed" {
		t.Fatalf("rename failed: %q", got.Name)
	}
	// Trashing moves it to trash: it disappears from active queries but appears as one event.
	eventID, err := s.TrashNode(ctx, uid, KindSignature, sig.ID)
	if err != nil || eventID == "" {
		t.Fatalf("TrashNode err=%v event=%q", err, eventID)
	}
	if _, err := s.GetSignature(ctx, uid, sig.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after trashing, got %v", err)
	}
	if list, _ := s.ListSignatures(ctx, uid, root); len(list) != 0 {
		t.Fatalf("active list should be empty, got %d", len(list))
	}
	events, err := s.ListTrashEvents(ctx, uid)
	if err != nil || len(events) != 1 || events[0].RootKind != KindSignature || events[0].ItemCount != 1 {
		t.Fatalf("events=%+v err=%v", events, err)
	}

	// Restore brings it back.
	conflicts, err := s.RestoreNode(ctx, uid, KindSignature, sig.ID, nil)
	if err != nil || len(conflicts) != 0 {
		t.Fatalf("restore conflicts=%v err=%v", conflicts, err)
	}
	if list, _ := s.ListSignatures(ctx, uid, root); len(list) != 1 {
		t.Fatalf("expected 1 active signature after restore, got %d", len(list))
	}
	// The event is gone once nothing references it.
	if events, _ := s.ListTrashEvents(ctx, uid); len(events) != 0 {
		t.Fatalf("expected no events after restore, got %d", len(events))
	}

	// Trash again, then permanently delete the event from trash.
	ev2, _ := s.TrashNode(ctx, uid, KindSignature, sig.ID)
	paths, err := s.HardDeleteEvent(ctx, uid, ev2)
	if err != nil || len(paths) != 1 || paths[0] != "p" {
		t.Fatalf("hard delete paths=%v err=%v", paths, err)
	}
	if events, _ := s.ListTrashEvents(ctx, uid); len(events) != 0 {
		t.Fatal("trash should be empty after permanent delete")
	}
}
