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
	path, err := s.DeleteSignature(ctx, uid, sig.ID)
	if err != nil || path != "p" {
		t.Fatalf("delete path=%q err=%v", path, err)
	}
	if _, err := s.GetSignature(ctx, uid, sig.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
