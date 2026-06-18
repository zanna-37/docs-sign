package store

import (
	"context"
	"path/filepath"
	"testing"

	"docs-sign/internal/crypto"
)

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
	list, err := s.ListSignatures(ctx, uid)
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
	// Soft delete moves it to trash: it disappears from active queries but appears in trash.
	if err := s.SoftDeleteSignature(ctx, uid, sig.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSignature(ctx, uid, sig.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after soft delete, got %v", err)
	}
	if list, _ := s.ListSignatures(ctx, uid); len(list) != 0 {
		t.Fatalf("active list should be empty, got %d", len(list))
	}
	trash, err := s.ListTrash(ctx, uid)
	if err != nil || len(trash) != 1 || trash[0].Kind != KindSignature {
		t.Fatalf("trash=%v err=%v", trash, err)
	}

	// Restore brings it back.
	if err := s.RestoreItem(ctx, uid, KindSignature, sig.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListSignatures(ctx, uid); len(list) != 1 {
		t.Fatalf("expected 1 active signature after restore, got %d", len(list))
	}

	// Soft delete again, then permanently delete from trash.
	_ = s.SoftDeleteSignature(ctx, uid, sig.ID)
	paths, err := s.HardDeleteItem(ctx, uid, KindSignature, sig.ID)
	if err != nil || len(paths) != 1 || paths[0] != "p" {
		t.Fatalf("hard delete paths=%v err=%v", paths, err)
	}
	if trash, _ := s.ListTrash(ctx, uid); len(trash) != 0 {
		t.Fatal("trash should be empty after permanent delete")
	}
}
