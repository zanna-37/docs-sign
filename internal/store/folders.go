package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrInvalidMove is returned when a folder would be moved into itself or one of its own
// descendants, which would detach a cycle from the tree.
var ErrInvalidMove = errors.New("store: cannot move a folder into itself or its descendant")

// Folder is a node in a user's per-kind organization tree. A folder holds documents OR
// signatures (never both), decided by Kind.
type Folder struct {
	ID        string
	UserID    string
	Kind      string         // KindDocument | KindSignature
	ParentID  sql.NullString // NULL = top level
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// folderNameTaken reports whether an active folder (other than excludeID) already uses name
// among the siblings under parentID in this user's tree for kind.
func folderNameTaken(ctx context.Context, q querier, userID, kind string, parentID sql.NullString, name, excludeID string) (bool, error) {
	var n int
	var err error
	if parentID.Valid {
		err = q.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM folders
			 WHERE user_id=? AND kind=? AND parent_id=? AND name=? AND deleted_at IS NULL AND id<>?`,
			userID, kind, parentID.String, name, excludeID).Scan(&n)
	} else {
		err = q.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM folders
			 WHERE user_id=? AND kind=? AND parent_id IS NULL AND name=? AND deleted_at IS NULL AND id<>?`,
			userID, kind, name, excludeID).Scan(&n)
	}
	return n > 0, err
}

// requireActiveFolder loads an active folder owned by userID, returning its kind. ErrNotFound if
// it does not exist (or is trashed).
func requireActiveFolder(ctx context.Context, q querier, userID, id string) (kind string, err error) {
	err = q.QueryRowContext(ctx,
		`SELECT kind FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`, id, userID).Scan(&kind)
	if err == sql.ErrNoRows {
		return "", ErrNotFound
	}
	return kind, err
}

// activeSubtreeFolderIDs returns rootID plus every active descendant folder id.
func activeSubtreeFolderIDs(ctx context.Context, q querier, userID, rootID string) ([]string, error) {
	return collectStrings(ctx, q, `
		WITH RECURSIVE sub(id) AS (
			SELECT id FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL
			UNION ALL
			SELECT f.id FROM folders f JOIN sub ON f.parent_id = sub.id
			WHERE f.deleted_at IS NULL
		)
		SELECT id FROM sub`, rootID, userID)
}

// CreateFolder inserts a new active folder, validating its parent and refusing a name already
// used by an active sibling.
func (s *Store) CreateFolder(ctx context.Context, f *Folder) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		if f.ParentID.Valid {
			pk, err := requireActiveFolder(ctx, tx, f.UserID, f.ParentID.String)
			if err != nil {
				return err
			}
			if pk != f.Kind {
				return ErrNotFound
			}
		}
		taken, err := folderNameTaken(ctx, tx, f.UserID, f.Kind, f.ParentID, f.Name, "")
		if err != nil {
			return err
		}
		if taken {
			return ErrNameConflict
		}
		now := time.Now().Unix()
		f.CreatedAt, f.UpdatedAt = unixToTime(now), unixToTime(now)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO folders (id, user_id, kind, parent_id, name, created_at, updated_at)
			VALUES (?,?,?,?,?,?,?)`,
			f.ID, f.UserID, f.Kind, f.ParentID, f.Name, now, now)
		return err
	})
}

// ListFolders returns the active folders directly under parentID (root when invalid) for the
// given kind, alphabetically.
func (s *Store) ListFolders(ctx context.Context, userID, kind string, parentID sql.NullString) ([]Folder, error) {
	var rows *sql.Rows
	var err error
	const sel = `SELECT id, user_id, kind, parent_id, name, created_at, updated_at FROM folders`
	if parentID.Valid {
		rows, err = s.db.QueryContext(ctx,
			sel+` WHERE user_id=? AND kind=? AND parent_id=? AND deleted_at IS NULL ORDER BY name COLLATE NOCASE`,
			userID, kind, parentID.String)
	} else {
		rows, err = s.db.QueryContext(ctx,
			sel+` WHERE user_id=? AND kind=? AND parent_id IS NULL AND deleted_at IS NULL ORDER BY name COLLATE NOCASE`,
			userID, kind)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFolders(rows)
}

func scanFolders(rows *sql.Rows) ([]Folder, error) {
	var out []Folder
	for rows.Next() {
		var f Folder
		var created, upd int64
		if err := rows.Scan(&f.ID, &f.UserID, &f.Kind, &f.ParentID, &f.Name, &created, &upd); err != nil {
			return nil, err
		}
		f.CreatedAt, f.UpdatedAt = unixToTime(created), unixToTime(upd)
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetFolder returns one active folder owned by the user.
func (s *Store) GetFolder(ctx context.Context, userID, id string) (*Folder, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, kind, parent_id, name, created_at, updated_at
		FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`, id, userID)
	var f Folder
	var created, upd int64
	err := row.Scan(&f.ID, &f.UserID, &f.Kind, &f.ParentID, &f.Name, &created, &upd)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	f.CreatedAt, f.UpdatedAt = unixToTime(created), unixToTime(upd)
	return &f, nil
}

// FolderPath returns the chain of active folders from the top level down to id (inclusive),
// for building a breadcrumb.
func (s *Store) FolderPath(ctx context.Context, userID, id string) ([]Folder, error) {
	var chain []Folder
	cur := nullString(id)
	for cur.Valid {
		f, err := s.GetFolder(ctx, userID, cur.String)
		if err != nil {
			return nil, err
		}
		chain = append([]Folder{*f}, chain...)
		cur = f.ParentID
	}
	return chain, nil
}

// RenameFolder renames an active folder, refusing a name already taken by an active sibling.
func (s *Store) RenameFolder(ctx context.Context, userID, id, name string) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var kind string
		var parentID sql.NullString
		err := tx.QueryRowContext(ctx,
			`SELECT kind, parent_id FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			id, userID).Scan(&kind, &parentID)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		taken, err := folderNameTaken(ctx, tx, userID, kind, parentID, name, id)
		if err != nil {
			return err
		}
		if taken {
			return ErrNameConflict
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE folders SET name=?, updated_at=? WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			name, time.Now().Unix(), id, userID)
		return err
	})
}

// MoveFolder reparents an active folder under newParentID (root when invalid), guarding against
// cycles and destination name collisions.
func (s *Store) MoveFolder(ctx context.Context, userID, id string, newParentID sql.NullString) error {
	return s.withTx(ctx, func(tx *sql.Tx) error {
		var kind string
		var name string
		err := tx.QueryRowContext(ctx,
			`SELECT kind, name FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			id, userID).Scan(&kind, &name)
		if err == sql.ErrNoRows {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if newParentID.Valid {
			if newParentID.String == id {
				return ErrInvalidMove
			}
			pk, err := requireActiveFolder(ctx, tx, userID, newParentID.String)
			if err != nil {
				return err
			}
			if pk != kind {
				return ErrNotFound
			}
			// The destination must not sit inside the subtree being moved.
			subtree, err := activeSubtreeFolderIDs(ctx, tx, userID, id)
			if err != nil {
				return err
			}
			for _, sid := range subtree {
				if sid == newParentID.String {
					return ErrInvalidMove
				}
			}
		}
		taken, err := folderNameTaken(ctx, tx, userID, kind, newParentID, name, id)
		if err != nil {
			return err
		}
		if taken {
			return ErrNameConflict
		}
		_, err = tx.ExecContext(ctx,
			`UPDATE folders SET parent_id=?, updated_at=? WHERE id=? AND user_id=? AND deleted_at IS NULL`,
			newParentID, time.Now().Unix(), id, userID)
		return err
	})
}
