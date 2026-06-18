package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"docs-sign/internal/crypto"
)

// User is an account row. Wrapped DEKs and salts are the only key-related material stored;
// the plaintext DEK exists solely in server memory during a session.
type User struct {
	ID                 string
	Username           string
	IsAdmin            bool
	Status             string // "active" | "disabled"
	KDF                crypto.KDFParams
	PwSalt             []byte
	PwWrappedDEK       []byte
	RecSalt            []byte // may be nil until a recovery code is established
	RecWrappedDEK      []byte
	MustChangePassword bool
	Language           string // "" = follow browser, otherwise a BCP-47 code like "en"/"it"
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

const (
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CreateUser inserts a new user.
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	now := time.Now().Unix()
	u.CreatedAt = unixToTime(now)
	u.UpdatedAt = unixToTime(now)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (id, username, is_admin, status, kdf_time, kdf_memory, kdf_threads,
			pw_salt, pw_wrapped_dek, rec_salt, rec_wrapped_dek, must_change_password,
			created_at, updated_at, language)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		u.ID, u.Username, boolToInt(u.IsAdmin), u.Status,
		u.KDF.Time, u.KDF.Memory, u.KDF.Threads,
		u.PwSalt, u.PwWrappedDEK, u.RecSalt, u.RecWrappedDEK, boolToInt(u.MustChangePassword),
		now, now, u.Language)
	return err
}

func scanUser(row interface{ Scan(...any) error }) (*User, error) {
	var (
		u             User
		isAdmin, must int
		created, upd  int64
	)
	var language sql.NullString
	err := row.Scan(&u.ID, &u.Username, &isAdmin, &u.Status,
		&u.KDF.Time, &u.KDF.Memory, &u.KDF.Threads,
		&u.PwSalt, &u.PwWrappedDEK, &u.RecSalt, &u.RecWrappedDEK, &must,
		&created, &upd, &language)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin != 0
	u.MustChangePassword = must != 0
	u.Language = language.String
	u.CreatedAt = unixToTime(created)
	u.UpdatedAt = unixToTime(upd)
	return &u, nil
}

const userCols = `id, username, is_admin, status, kdf_time, kdf_memory, kdf_threads,
	pw_salt, pw_wrapped_dek, rec_salt, rec_wrapped_dek, must_change_password,
	created_at, updated_at, language`

// GetUserByUsername looks up a user by (case-insensitive) username.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ? COLLATE NOCASE`, username)
	return scanUser(row)
}

// GetUserByID looks up a user by id.
func (s *Store) GetUserByID(ctx context.Context, id string) (*User, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// ListUsers returns all users ordered by username.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY username COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// UpdateUserKeys updates the wrapped DEKs, salts, KDF params and must-change flag. Used on
// password change and when (re)establishing a recovery code.
func (s *Store) UpdateUserKeys(ctx context.Context, u *User) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
		UPDATE users SET kdf_time=?, kdf_memory=?, kdf_threads=?,
			pw_salt=?, pw_wrapped_dek=?, rec_salt=?, rec_wrapped_dek=?,
			must_change_password=?, updated_at=?
		WHERE id=?`,
		u.KDF.Time, u.KDF.Memory, u.KDF.Threads,
		u.PwSalt, u.PwWrappedDEK, u.RecSalt, u.RecWrappedDEK,
		boolToInt(u.MustChangePassword), now, u.ID)
	return err
}

// SetUserLanguage updates a user's preferred language ("" follows the browser).
func (s *Store) SetUserLanguage(ctx context.Context, id, language string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET language=?, updated_at=? WHERE id=?`,
		language, time.Now().Unix(), id)
	return err
}

// SetUsername changes a user's username. The unique index is enforced separately by the
// caller (which checks for an existing holder first).
func (s *Store) SetUsername(ctx context.Context, id, username string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET username=?, updated_at=? WHERE id=?`,
		username, time.Now().Unix(), id)
	return err
}

// SetUserStatus enables or disables a user.
func (s *Store) SetUserStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET status=?, updated_at=? WHERE id=?`,
		status, time.Now().Unix(), id)
	return err
}

// DeleteUser removes a user. Content rows cascade; blob files must be deleted by the caller.
func (s *Store) DeleteUser(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// CountUsers returns the total number of users (used for first-run detection).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CountAdmins returns the number of active admin accounts (used to prevent removing the
// last admin).
func (s *Store) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE is_admin=1 AND status=?`, StatusActive).Scan(&n)
	return n, err
}
