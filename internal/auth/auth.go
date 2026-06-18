// Package auth orchestrates accounts, login, recovery and admin user management on top of
// the store, crypto and session packages. It is the only place that derives keys from
// passwords/recovery codes and decides how the per-user DEK is wrapped.
package auth

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"docs-sign/internal/blob"
	"docs-sign/internal/crypto"
	"docs-sign/internal/session"
	"docs-sign/internal/store"
)

// Sentinel errors surfaced to the API layer (which maps them to HTTP status codes).
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrSetupAlreadyDone   = errors.New("setup already completed")
	ErrUserExists         = errors.New("username already exists")
	ErrForbidden          = errors.New("forbidden")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidUsername    = errors.New("invalid username")
	ErrLastAdmin          = errors.New("cannot remove or disable the last admin")
	ErrCannotDeleteSelf   = errors.New("cannot delete your own account")
	ErrInvalidInput       = errors.New("invalid input")
)

const minPasswordLen = 8

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// Service provides authentication and user-management operations.
type Service struct {
	store    *store.Store
	blobs    *blob.Store
	sessions *session.Manager
	kdf      crypto.KDFParams
}

// NewService builds an auth service.
func NewService(st *store.Store, blobs *blob.Store, sessions *session.Manager, kdf crypto.KDFParams) *Service {
	return &Service{store: st, blobs: blobs, sessions: sessions, kdf: kdf}
}

func validateUsername(u string) error {
	if !usernameRe.MatchString(u) {
		return ErrInvalidUsername
	}
	return nil
}

func validatePassword(p string) error {
	if len(p) < minPasswordLen {
		return ErrWeakPassword
	}
	return nil
}

// wrapWithSecret derives a KEK from secret + a fresh salt and wraps dek under it.
func (s *Service) wrapWithSecret(dek []byte, secret string) (salt, wrapped []byte, err error) {
	salt, err = crypto.NewSalt()
	if err != nil {
		return nil, nil, err
	}
	kek := crypto.DeriveKey([]byte(secret), salt, s.kdf)
	defer crypto.Zero(kek)
	wrapped, err = crypto.WrapKey(kek, dek)
	if err != nil {
		return nil, nil, err
	}
	return salt, wrapped, nil
}

// NeedsSetup reports whether no users exist yet (first run).
func (s *Service) NeedsSetup(ctx context.Context) (bool, error) {
	n, err := s.store.CountUsers(ctx)
	return n == 0, err
}

// Setup creates the initial admin account and returns the one-time recovery code to display.
func (s *Service) Setup(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return "", err
	}
	if err := validatePassword(password); err != nil {
		return "", err
	}
	n, err := s.store.CountUsers(ctx)
	if err != nil {
		return "", err
	}
	if n > 0 {
		return "", ErrSetupAlreadyDone
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		return "", err
	}
	defer crypto.Zero(dek)

	pwSalt, pwWrapped, err := s.wrapWithSecret(dek, password)
	if err != nil {
		return "", err
	}
	display, canonical, err := crypto.GenerateRecoveryCode()
	if err != nil {
		return "", err
	}
	recSalt, recWrapped, err := s.wrapWithSecret(dek, canonical)
	if err != nil {
		return "", err
	}
	u := &store.User{
		ID: store.NewID(), Username: username, IsAdmin: true, Status: store.StatusActive,
		KDF: s.kdf, PwSalt: pwSalt, PwWrappedDEK: pwWrapped,
		RecSalt: recSalt, RecWrappedDEK: recWrapped, MustChangePassword: false,
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return "", err
	}
	return display, nil
}

// Login verifies a password by unwrapping the DEK and starts a session.
func (s *Service) Login(ctx context.Context, username, password string) (*session.Session, error) {
	u, err := s.store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if u.Status != store.StatusActive {
		return nil, ErrAccountDisabled
	}
	kek := crypto.DeriveKey([]byte(password), u.PwSalt, u.KDF)
	defer crypto.Zero(kek)
	dek, err := crypto.UnwrapKey(kek, u.PwWrappedDEK)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	defer crypto.Zero(dek)
	return s.sessions.Create(u.ID, u.Username, u.IsAdmin, u.MustChangePassword, dek), nil
}

// Recover verifies a recovery code, starts a session, and forces a password reset.
func (s *Service) Recover(ctx context.Context, username, recoveryCode string) (*session.Session, error) {
	u, err := s.store.GetUserByUsername(ctx, strings.TrimSpace(username))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if u.Status != store.StatusActive {
		return nil, ErrAccountDisabled
	}
	if len(u.RecWrappedDEK) == 0 || len(u.RecSalt) == 0 {
		return nil, ErrInvalidCredentials
	}
	canonical := crypto.NormalizeRecoveryCode(recoveryCode)
	kek := crypto.DeriveKey([]byte(canonical), u.RecSalt, u.KDF)
	defer crypto.Zero(kek)
	dek, err := crypto.UnwrapKey(kek, u.RecWrappedDEK)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	defer crypto.Zero(dek)
	// Force the user to set a fresh password after recovering.
	return s.sessions.Create(u.ID, u.Username, u.IsAdmin, true, dek), nil
}

// ChangePassword re-wraps the session's DEK under a new password. The DEK itself is
// unchanged, so all existing content remains decryptable. If the user has no recovery code
// yet (first time setting their own password), one is generated and returned for display.
func (s *Service) ChangePassword(ctx context.Context, sess *session.Session, newPassword string) (string, error) {
	if err := validatePassword(newPassword); err != nil {
		return "", err
	}
	u, err := s.store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return "", err
	}
	dek := sess.DEK()
	defer crypto.Zero(dek)

	pwSalt, pwWrapped, err := s.wrapWithSecret(dek, newPassword)
	if err != nil {
		return "", err
	}
	u.KDF = s.kdf
	u.PwSalt = pwSalt
	u.PwWrappedDEK = pwWrapped
	u.MustChangePassword = false

	var display string
	if len(u.RecWrappedDEK) == 0 {
		var canonical string
		display, canonical, err = crypto.GenerateRecoveryCode()
		if err != nil {
			return "", err
		}
		recSalt, recWrapped, werr := s.wrapWithSecret(dek, canonical)
		if werr != nil {
			return "", werr
		}
		u.RecSalt = recSalt
		u.RecWrappedDEK = recWrapped
	}
	if err := s.store.UpdateUserKeys(ctx, u); err != nil {
		return "", err
	}
	s.sessions.SetMustChangePassword(sess.Token, false)
	return display, nil
}

// RegenerateRecoveryCode issues a fresh recovery code (invalidating the old one) and returns
// it for display.
func (s *Service) RegenerateRecoveryCode(ctx context.Context, sess *session.Session) (string, error) {
	u, err := s.store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return "", err
	}
	dek := sess.DEK()
	defer crypto.Zero(dek)
	display, canonical, err := crypto.GenerateRecoveryCode()
	if err != nil {
		return "", err
	}
	recSalt, recWrapped, err := s.wrapWithSecret(dek, canonical)
	if err != nil {
		return "", err
	}
	u.RecSalt = recSalt
	u.RecWrappedDEK = recWrapped
	if err := s.store.UpdateUserKeys(ctx, u); err != nil {
		return "", err
	}
	return display, nil
}

// Logout ends the session.
func (s *Service) Logout(sess *session.Session) { s.sessions.Delete(sess.Token) }

// --- admin operations ---

// AdminCreateUser provisions a new user with a temporary password they must change on first
// login (at which point their recovery code is generated).
func (s *Service) AdminCreateUser(ctx context.Context, actor *session.Session, username, tempPassword string, isAdmin bool) (*store.User, error) {
	if !actor.IsAdmin {
		return nil, ErrForbidden
	}
	username = strings.TrimSpace(username)
	if err := validateUsername(username); err != nil {
		return nil, err
	}
	if err := validatePassword(tempPassword); err != nil {
		return nil, err
	}
	switch _, err := s.store.GetUserByUsername(ctx, username); {
	case err == nil:
		return nil, ErrUserExists
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, err
	}
	defer crypto.Zero(dek)
	pwSalt, pwWrapped, err := s.wrapWithSecret(dek, tempPassword)
	if err != nil {
		return nil, err
	}
	u := &store.User{
		ID: store.NewID(), Username: username, IsAdmin: isAdmin, Status: store.StatusActive,
		KDF: s.kdf, PwSalt: pwSalt, PwWrappedDEK: pwWrapped, MustChangePassword: true,
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// AdminSetUserStatus enables/disables a user, dropping their sessions when disabled.
func (s *Service) AdminSetUserStatus(ctx context.Context, actor *session.Session, userID, status string) error {
	if !actor.IsAdmin {
		return ErrForbidden
	}
	if status != store.StatusActive && status != store.StatusDisabled {
		return ErrInvalidInput
	}
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.IsAdmin && status == store.StatusDisabled {
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return err
		}
	}
	if err := s.store.SetUserStatus(ctx, userID, status); err != nil {
		return err
	}
	if status == store.StatusDisabled {
		s.sessions.DeleteByUser(userID)
	}
	return nil
}

// AdminDeleteUser removes a user account and all of their encrypted content.
func (s *Service) AdminDeleteUser(ctx context.Context, actor *session.Session, userID string) error {
	if !actor.IsAdmin {
		return ErrForbidden
	}
	if actor.UserID == userID {
		return ErrCannotDeleteSelf
	}
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if u.IsAdmin {
		if err := s.ensureNotLastAdmin(ctx); err != nil {
			return err
		}
	}
	s.sessions.DeleteByUser(userID)
	if err := s.store.DeleteUser(ctx, userID); err != nil {
		return err
	}
	return s.blobs.RemoveUserDir(userID)
}

// AdminResetUser re-provisions a user with a new empty vault and temporary password. Because
// the old DEK cannot be recovered, all of the user's existing content is destroyed.
func (s *Service) AdminResetUser(ctx context.Context, actor *session.Session, userID, newTempPassword string) error {
	if !actor.IsAdmin {
		return ErrForbidden
	}
	if err := validatePassword(newTempPassword); err != nil {
		return err
	}
	u, err := s.store.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		return err
	}
	defer crypto.Zero(dek)
	pwSalt, pwWrapped, err := s.wrapWithSecret(dek, newTempPassword)
	if err != nil {
		return err
	}

	// Destroy the now-unrecoverable old content.
	if err := s.store.DeleteUserContent(ctx, userID); err != nil {
		return err
	}
	if err := s.blobs.RemoveUserDir(userID); err != nil {
		return err
	}

	u.KDF = s.kdf
	u.PwSalt = pwSalt
	u.PwWrappedDEK = pwWrapped
	u.RecSalt = nil
	u.RecWrappedDEK = nil
	u.MustChangePassword = true
	if err := s.store.UpdateUserKeys(ctx, u); err != nil {
		return err
	}
	s.sessions.DeleteByUser(userID)
	return nil
}

func (s *Service) ensureNotLastAdmin(ctx context.Context) error {
	admins, err := s.store.CountAdmins(ctx)
	if err != nil {
		return err
	}
	if admins <= 1 {
		return ErrLastAdmin
	}
	return nil
}

// ListUsers returns all users (admin only).
func (s *Service) ListUsers(ctx context.Context, actor *session.Session) ([]store.User, error) {
	if !actor.IsAdmin {
		return nil, ErrForbidden
	}
	return s.store.ListUsers(ctx)
}
