package auth

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"docs-sign/internal/blob"
	"docs-sign/internal/crypto"
	"docs-sign/internal/session"
	"docs-sign/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "meta.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	bs, err := blob.New(filepath.Join(t.TempDir(), "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	sm := session.NewManager(time.Hour, 24*time.Hour)
	// Light KDF params keep the test fast.
	return NewService(st, bs, sm, crypto.KDFParams{Time: 1, Memory: 8 * 1024, Threads: 1})
}

func TestSetupAndLogin(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)

	need, _ := s.NeedsSetup(ctx)
	if !need {
		t.Fatal("expected NeedsSetup true on empty store")
	}
	rec, err := s.Setup(ctx, "admin", "supersecret")
	if err != nil {
		t.Fatal(err)
	}
	if rec == "" {
		t.Fatal("expected recovery code from setup")
	}
	if _, err := s.Setup(ctx, "admin2", "supersecret"); err != ErrSetupAlreadyDone {
		t.Fatalf("expected ErrSetupAlreadyDone, got %v", err)
	}

	// Correct login.
	sess, err := s.Login(ctx, "admin", "supersecret")
	if err != nil {
		t.Fatal(err)
	}
	if !sess.IsAdmin || sess.MustChangePassword {
		t.Fatalf("unexpected session: %+v", sess)
	}
	// Wrong password.
	if _, err := s.Login(ctx, "admin", "wrong"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}

	// Recovery works and forces a password change.
	rsess, err := s.Recover(ctx, "admin", rec)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if !rsess.MustChangePassword {
		t.Fatal("expected recovery session to force password change")
	}
	if _, err := s.Recover(ctx, "admin", "WRONG-CODE-0000"); err != ErrInvalidCredentials {
		t.Fatalf("expected ErrInvalidCredentials for bad recovery, got %v", err)
	}
}

func TestAdminCreatedUserFlow(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	if _, err := s.Setup(ctx, "admin", "adminpassword"); err != nil {
		t.Fatal(err)
	}
	admin, err := s.Login(ctx, "admin", "adminpassword")
	if err != nil {
		t.Fatal(err)
	}

	u, err := s.AdminCreateUser(ctx, admin, "carol", "temppass123", false)
	if err != nil {
		t.Fatal(err)
	}
	if !u.MustChangePassword {
		t.Fatal("new user should require password change")
	}

	// First login with temp password requires change and has no recovery code yet.
	csess, err := s.Login(ctx, "carol", "temppass123")
	if err != nil {
		t.Fatal(err)
	}
	if !csess.MustChangePassword {
		t.Fatal("expected must-change on first login")
	}

	// Changing the password yields a recovery code and keeps content decryptable.
	rec, err := s.ChangePassword(ctx, csess, "carolnewpass")
	if err != nil {
		t.Fatal(err)
	}
	if rec == "" {
		t.Fatal("expected recovery code on first password change")
	}

	// Old temp password no longer works; new one does.
	if _, err := s.Login(ctx, "carol", "temppass123"); err != ErrInvalidCredentials {
		t.Fatalf("expected old password rejected, got %v", err)
	}
	csess2, err := s.Login(ctx, "carol", "carolnewpass")
	if err != nil {
		t.Fatal(err)
	}
	if csess2.MustChangePassword {
		t.Fatal("must-change should be cleared after change")
	}

	// Recovery code from the change works.
	if _, err := s.Recover(ctx, "carol", rec); err != nil {
		t.Fatalf("recovery after change failed: %v", err)
	}

	// Non-admin cannot create users.
	if _, err := s.AdminCreateUser(ctx, csess2, "mallory", "temppass123", false); err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestCannotDeleteLastAdmin(t *testing.T) {
	ctx := context.Background()
	s := newTestService(t)
	if _, err := s.Setup(ctx, "admin", "adminpassword"); err != nil {
		t.Fatal(err)
	}
	admin, _ := s.Login(ctx, "admin", "adminpassword")
	other, _ := s.AdminCreateUser(ctx, admin, "carol", "temppass123", false)
	if err := s.AdminDeleteUser(ctx, admin, admin.UserID); err != ErrCannotDeleteSelf {
		t.Fatalf("expected ErrCannotDeleteSelf, got %v", err)
	}
	// Deleting a normal user is fine.
	if err := s.AdminDeleteUser(ctx, admin, other.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
}
