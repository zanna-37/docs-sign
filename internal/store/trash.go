package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Resolution actions for a restore (or upload) name conflict.
const (
	ResolveOverride = "override" // trash the blocking active file as its own new event, then restore
	ResolveSkip     = "skip"     // leave the conflicting item in the trash
	ResolveRename   = "rename"   // restore under NewName
)

// errDeferConflicts unwinds the restore transaction when unresolved conflicts remain, so the
// whole restore is atomic: either it fully applies or nothing changes and the caller is handed
// the conflicts to resolve.
var errDeferConflicts = errors.New("store: restore has unresolved conflicts")

// TrashEventSummary is one top-level trash entry — the root of a single delete action.
type TrashEventSummary struct {
	EventID   string
	RootKind  string // KindFolder | KindDocument | KindSignature | KindExport
	RootID    string
	Label     string
	ByteSize  int64 // total encrypted size of the files the event carries
	ItemCount int   // number of files (documents+signatures+exports) in the event
	CreatedAt time.Time
}

// TrashEntry is one child encountered while walking a trashed folder.
type TrashEntry struct {
	Kind     string // KindFolder | KindDocument | KindSignature
	ID       string
	Name     string
	ByteSize int64
	IsFolder bool
}

// RestoreConflict reports a file whose name is already taken at its restore destination.
type RestoreConflict struct {
	Kind     string // KindDocument | KindSignature
	ID       string // the trashed item awaiting a decision
	Name     string
	DestPath string // human path of the destination folder ("/" for the root)
}

// Resolution chooses how to settle one restore conflict.
type Resolution struct {
	Action  string
	NewName string
}

// placeholders returns "?,?,?" with n marks.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(",?", n)[1:]
}

// markTrashed stamps deleted_at + trash_event_id onto every active row of table whose matchCol
// is one of ids. matchCol is "id" for folders, "folder_id" for items.
func markTrashed(ctx context.Context, tx *sql.Tx, table, matchCol, userID, eventID string, now int64, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	args := []any{now, eventID, userID}
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := tx.ExecContext(ctx,
		`UPDATE `+table+` SET deleted_at=?, trash_event_id=? WHERE user_id=? AND `+
			matchCol+` IN (`+placeholders(len(ids))+`) AND deleted_at IS NULL`, args...)
	return err
}

// TrashNode soft-deletes a node as a single new trash event. For a folder it captures the
// folder's whole active subtree (folders and the items they hold) into that one event; items
// already trashed in another event are left untouched, keeping events independent. Returns the
// new event id.
func (s *Store) TrashNode(ctx context.Context, userID, kind, id string) (string, error) {
	var eventID string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		now := time.Now().Unix()
		switch kind {
		case KindDocument, KindSignature, KindExport:
			table, _ := tableForKind(kind)
			var name string
			err := tx.QueryRowContext(ctx,
				`SELECT name FROM `+table+` WHERE id=? AND user_id=? AND deleted_at IS NULL`,
				id, userID).Scan(&name)
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			eventID = NewID()
			if err := insertTrashEvent(ctx, tx, eventID, userID, kind, id, name, now); err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx,
				`UPDATE `+table+` SET deleted_at=?, trash_event_id=? WHERE id=? AND user_id=?`,
				now, eventID, id, userID)
			return err
		case KindFolder:
			var name, fkind string
			err := tx.QueryRowContext(ctx,
				`SELECT name, kind FROM folders WHERE id=? AND user_id=? AND deleted_at IS NULL`,
				id, userID).Scan(&name, &fkind)
			if err == sql.ErrNoRows {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			eventID = NewID()
			if err := insertTrashEvent(ctx, tx, eventID, userID, KindFolder, id, name, now); err != nil {
				return err
			}
			subtree, err := activeSubtreeFolderIDs(ctx, tx, userID, id)
			if err != nil {
				return err
			}
			if err := markTrashed(ctx, tx, "folders", "id", userID, eventID, now, subtree); err != nil {
				return err
			}
			itemTable, _ := tableForKind(fkind)
			return markTrashed(ctx, tx, itemTable, "folder_id", userID, eventID, now, subtree)
		default:
			return ErrNotFound
		}
	})
	return eventID, err
}

func insertTrashEvent(ctx context.Context, tx *sql.Tx, eventID, userID, rootKind, rootID, label string, now int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO trash_events (id, user_id, root_kind, root_id, label, created_at) VALUES (?,?,?,?,?,?)`,
		eventID, userID, rootKind, rootID, label, now)
	return err
}

// ListTrashEvents returns the user's trash entries (one per delete action), newest first, each
// annotated with the size and count of the files it carries.
func (s *Store) ListTrashEvents(ctx context.Context, userID string) ([]TrashEventSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, root_kind, root_id, label, created_at FROM trash_events WHERE user_id=? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashEventSummary
	idx := map[string]int{}
	for rows.Next() {
		var e TrashEventSummary
		var created int64
		if err := rows.Scan(&e.EventID, &e.RootKind, &e.RootID, &e.Label, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = unixToTime(created)
		idx[e.EventID] = len(out)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	agg, err := s.db.QueryContext(ctx, `
		SELECT trash_event_id, COUNT(1), COALESCE(SUM(byte_size),0) FROM (
			SELECT trash_event_id, byte_size FROM documents  WHERE user_id=? AND trash_event_id IS NOT NULL
			UNION ALL
			SELECT trash_event_id, byte_size FROM signatures WHERE user_id=? AND trash_event_id IS NOT NULL
			UNION ALL
			SELECT trash_event_id, byte_size FROM exports     WHERE user_id=? AND trash_event_id IS NOT NULL
		) GROUP BY trash_event_id`, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer agg.Close()
	for agg.Next() {
		var eid string
		var count int
		var size int64
		if err := agg.Scan(&eid, &count, &size); err != nil {
			return nil, err
		}
		if i, ok := idx[eid]; ok {
			out[i].ItemCount = count
			out[i].ByteSize = size
		}
	}
	return out, agg.Err()
}

// GetTrashEvent returns one trash event (without size aggregates), or ErrNotFound.
func (s *Store) GetTrashEvent(ctx context.Context, userID, eventID string) (*TrashEventSummary, error) {
	var e TrashEventSummary
	var created int64
	err := s.db.QueryRowContext(ctx,
		`SELECT id, root_kind, root_id, label, created_at FROM trash_events WHERE id=? AND user_id=?`,
		eventID, userID).Scan(&e.EventID, &e.RootKind, &e.RootID, &e.Label, &created)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	e.CreatedAt = unixToTime(created)
	return &e, nil
}

// ListTrashChildren returns the trashed children directly inside parentID that belong to
// eventID — letting the UI walk a trashed folder. Children that belong to a different event are
// intentionally excluded (they are their own top-level entries).
func (s *Store) ListTrashChildren(ctx context.Context, userID, eventID string, parentID sql.NullString) ([]TrashEntry, error) {
	var out []TrashEntry

	folders, err := s.queryTrashChildren(ctx,
		`SELECT id, name, 0 FROM folders`, "parent_id", userID, eventID, parentID)
	if err != nil {
		return nil, err
	}
	for i := range folders {
		folders[i].Kind = KindFolder
		folders[i].IsFolder = true
	}
	out = append(out, folders...)

	for _, t := range []struct{ table, kind string }{{"documents", KindDocument}, {"signatures", KindSignature}} {
		items, err := s.queryTrashChildren(ctx,
			`SELECT id, name, byte_size FROM `+t.table, "folder_id", userID, eventID, parentID)
		if err != nil {
			return nil, err
		}
		for i := range items {
			items[i].Kind = t.kind
		}
		out = append(out, items...)
	}
	return out, nil
}

func (s *Store) queryTrashChildren(ctx context.Context, selectSQL, matchCol, userID, eventID string, parentID sql.NullString) ([]TrashEntry, error) {
	var rows *sql.Rows
	var err error
	if parentID.Valid {
		rows, err = s.db.QueryContext(ctx,
			selectSQL+` WHERE user_id=? AND trash_event_id=? AND `+matchCol+`=? ORDER BY name COLLATE NOCASE`,
			userID, eventID, parentID.String)
	} else {
		rows, err = s.db.QueryContext(ctx,
			selectSQL+` WHERE user_id=? AND trash_event_id=? AND `+matchCol+` IS NULL ORDER BY name COLLATE NOCASE`,
			userID, eventID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashEntry
	for rows.Next() {
		var e TrashEntry
		if err := rows.Scan(&e.ID, &e.Name, &e.ByteSize); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// --- restore ---

// restoreState carries the per-restore context through the recursion.
type restoreState struct {
	tx        *sql.Tx
	userID    string
	eventID   string
	treeKind  string // KindDocument | KindSignature (the item tree)
	itemTable string
	res       map[string]Resolution
	conflicts []RestoreConflict
}

// RestoreNode restores a trashed node (an event root or any node reached by walking) to its
// original location, recreating and merging missing ancestor folders by name. File-name
// collisions are reported as conflicts; when any are unresolved the restore is rolled back and
// the conflicts are returned so the caller can prompt and retry with resolutions.
func (s *Store) RestoreNode(ctx context.Context, userID, kind, id string, res map[string]Resolution) ([]RestoreConflict, error) {
	if res == nil {
		res = map[string]Resolution{}
	}
	var conflicts []RestoreConflict
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		st := &restoreState{tx: tx, userID: userID, res: res}
		if err := st.restore(ctx, kind, id); err != nil {
			return err
		}
		conflicts = st.conflicts
		if len(conflicts) > 0 {
			return errDeferConflicts
		}
		return nil
	})
	if errors.Is(err, errDeferConflicts) {
		return conflicts, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (st *restoreState) restore(ctx context.Context, kind, id string) error {
	switch kind {
	case KindExport:
		return st.restoreExport(ctx, id)
	case KindDocument, KindSignature:
		return st.restoreItem(ctx, kind, id)
	case KindFolder:
		return st.restoreFolder(ctx, id)
	default:
		return ErrNotFound
	}
}

func (st *restoreState) restoreExport(ctx context.Context, id string) error {
	var eventID sql.NullString
	err := st.tx.QueryRowContext(ctx,
		`SELECT trash_event_id FROM exports WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`,
		id, st.userID).Scan(&eventID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err := st.tx.ExecContext(ctx,
		`UPDATE exports SET deleted_at=NULL, trash_event_id=NULL WHERE id=? AND user_id=?`,
		id, st.userID); err != nil {
		return err
	}
	return st.cleanupEvent(ctx, eventID.String)
}

func (st *restoreState) restoreItem(ctx context.Context, kind, id string) error {
	table, _ := tableForKind(kind)
	var name string
	var folderID, eventID sql.NullString
	err := st.tx.QueryRowContext(ctx,
		`SELECT name, folder_id, trash_event_id FROM `+table+` WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`,
		id, st.userID).Scan(&name, &folderID, &eventID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	st.eventID = eventID.String
	st.treeKind = folderKindForItem(kind)
	st.itemTable = table

	ancestors, err := st.ancestorNames(ctx, folderID)
	if err != nil {
		return err
	}
	dest, err := st.resolvePath(ctx, ancestors)
	if err != nil {
		return err
	}
	if err := st.placeItem(ctx, kind, id, name, dest); err != nil {
		return err
	}
	return st.cleanupEvent(ctx, st.eventID)
}

func (st *restoreState) restoreFolder(ctx context.Context, id string) error {
	var name, kind string
	var parentID, eventID sql.NullString
	err := st.tx.QueryRowContext(ctx,
		`SELECT name, kind, parent_id, trash_event_id FROM folders WHERE id=? AND user_id=? AND deleted_at IS NOT NULL`,
		id, st.userID).Scan(&name, &kind, &parentID, &eventID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	st.eventID = eventID.String
	st.treeKind = kind
	st.itemTable, _ = tableForKind(kind)

	ancestors, err := st.ancestorNames(ctx, parentID)
	if err != nil {
		return err
	}
	destParent, err := st.resolvePath(ctx, ancestors)
	if err != nil {
		return err
	}
	if err := st.placeFolder(ctx, id, name, destParent); err != nil {
		return err
	}
	return st.cleanupEvent(ctx, st.eventID)
}

// ancestorNames returns the names of the original ancestor folders from the top level down to
// (and including) startID, walking parent_id across rows in any state. A purged ancestor stops
// the walk (the remainder is treated as root-relative).
func (st *restoreState) ancestorNames(ctx context.Context, startID sql.NullString) ([]string, error) {
	var names []string
	cur := startID
	for cur.Valid {
		var name string
		var parent sql.NullString
		err := st.tx.QueryRowContext(ctx,
			`SELECT name, parent_id FROM folders WHERE id=? AND user_id=?`,
			cur.String, st.userID).Scan(&name, &parent)
		if err == sql.ErrNoRows {
			break
		}
		if err != nil {
			return nil, err
		}
		names = append([]string{name}, names...)
		cur = parent
	}
	return names, nil
}

// resolvePath turns a chain of folder names into an active destination folder, finding an
// existing active folder at each level or creating a new one (merge-by-name).
func (st *restoreState) resolvePath(ctx context.Context, names []string) (sql.NullString, error) {
	cur := sql.NullString{}
	for _, name := range names {
		id, found, err := st.activeFolderByName(ctx, cur, name)
		if err != nil {
			return sql.NullString{}, err
		}
		if !found {
			id, err = st.createActiveFolder(ctx, cur, name)
			if err != nil {
				return sql.NullString{}, err
			}
		}
		cur = nullString(id)
	}
	return cur, nil
}

func (st *restoreState) activeFolderByName(ctx context.Context, parentID sql.NullString, name string) (string, bool, error) {
	var id string
	var err error
	if parentID.Valid {
		err = st.tx.QueryRowContext(ctx,
			`SELECT id FROM folders WHERE user_id=? AND kind=? AND parent_id=? AND name=? AND deleted_at IS NULL LIMIT 1`,
			st.userID, st.treeKind, parentID.String, name).Scan(&id)
	} else {
		err = st.tx.QueryRowContext(ctx,
			`SELECT id FROM folders WHERE user_id=? AND kind=? AND parent_id IS NULL AND name=? AND deleted_at IS NULL LIMIT 1`,
			st.userID, st.treeKind, name).Scan(&id)
	}
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func (st *restoreState) createActiveFolder(ctx context.Context, parentID sql.NullString, name string) (string, error) {
	id := NewID()
	now := time.Now().Unix()
	_, err := st.tx.ExecContext(ctx,
		`INSERT INTO folders (id, user_id, kind, parent_id, name, created_at, updated_at) VALUES (?,?,?,?,?,?,?)`,
		id, st.userID, st.treeKind, parentID, name, now, now)
	return id, err
}

func (st *restoreState) activeItemByName(ctx context.Context, table string, folderID sql.NullString, name string) (string, bool, error) {
	var id string
	var err error
	if folderID.Valid {
		err = st.tx.QueryRowContext(ctx,
			`SELECT id FROM `+table+` WHERE user_id=? AND folder_id=? AND name=? AND deleted_at IS NULL LIMIT 1`,
			st.userID, folderID.String, name).Scan(&id)
	} else {
		err = st.tx.QueryRowContext(ctx,
			`SELECT id FROM `+table+` WHERE user_id=? AND folder_id IS NULL AND name=? AND deleted_at IS NULL LIMIT 1`,
			st.userID, name).Scan(&id)
	}
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

// placeItem un-trashes one item into dest. On a name collision it consults the supplied
// resolution (override/skip/rename) or records a conflict to defer.
func (st *restoreState) placeItem(ctx context.Context, kind, id, name string, dest sql.NullString) error {
	table, _ := tableForKind(kind)
	blockerID, conflict, err := st.activeItemByName(ctx, table, dest, name)
	if err != nil {
		return err
	}
	if !conflict {
		return st.untrashItem(ctx, table, id, dest, name)
	}

	res, ok := st.res[id]
	if !ok {
		st.recordConflict(ctx, kind, id, name, dest)
		return nil
	}
	switch res.Action {
	case ResolveSkip:
		return nil
	case ResolveRename:
		newName := strings.TrimSpace(res.NewName)
		if newName == "" {
			st.recordConflict(ctx, kind, id, name, dest)
			return nil
		}
		if _, taken, err := st.activeItemByName(ctx, table, dest, newName); err != nil {
			return err
		} else if taken {
			st.recordConflict(ctx, kind, id, newName, dest)
			return nil
		}
		return st.untrashItem(ctx, table, id, dest, newName)
	case ResolveOverride:
		if err := st.trashLeafAsEvent(ctx, kind, blockerID); err != nil {
			return err
		}
		return st.untrashItem(ctx, table, id, dest, name)
	default:
		st.recordConflict(ctx, kind, id, name, dest)
		return nil
	}
}

func (st *restoreState) untrashItem(ctx context.Context, table, id string, dest sql.NullString, name string) error {
	_, err := st.tx.ExecContext(ctx,
		`UPDATE `+table+` SET deleted_at=NULL, trash_event_id=NULL, folder_id=?, name=?, updated_at=? WHERE id=? AND user_id=?`,
		dest, name, time.Now().Unix(), id, st.userID)
	return err
}

func (st *restoreState) recordConflict(ctx context.Context, kind, id, name string, dest sql.NullString) {
	path, _ := st.pathLabel(ctx, dest)
	st.conflicts = append(st.conflicts, RestoreConflict{Kind: kind, ID: id, Name: name, DestPath: path})
}

// trashLeafAsEvent moves a blocking active item to its own brand-new trash event, so an
// "override" never destroys data.
func (st *restoreState) trashLeafAsEvent(ctx context.Context, kind, id string) error {
	table, _ := tableForKind(kind)
	var name string
	if err := st.tx.QueryRowContext(ctx,
		`SELECT name FROM `+table+` WHERE id=? AND user_id=? AND deleted_at IS NULL`,
		id, st.userID).Scan(&name); err != nil {
		return err
	}
	eventID := NewID()
	now := time.Now().Unix()
	if err := insertTrashEvent(ctx, st.tx, eventID, st.userID, kind, id, name, now); err != nil {
		return err
	}
	_, err := st.tx.ExecContext(ctx,
		`UPDATE `+table+` SET deleted_at=?, trash_event_id=? WHERE id=? AND user_id=?`,
		now, eventID, id, st.userID)
	return err
}

// placeFolder restores a trashed folder under destParent. If an active folder of the same name
// already lives there the two are merged; otherwise the whole in-event subtree is un-trashed in
// place.
func (st *restoreState) placeFolder(ctx context.Context, nodeID, nodeName string, destParent sql.NullString) error {
	mergeID, merge, err := st.activeFolderByName(ctx, destParent, nodeName)
	if err != nil {
		return err
	}
	if !merge {
		return st.untrashFolderWholesale(ctx, nodeID, destParent)
	}

	mergeDest := nullString(mergeID)
	childFolders, err := st.inEventChildFolders(ctx, nodeID)
	if err != nil {
		return err
	}
	for _, cf := range childFolders {
		if err := st.placeFolder(ctx, cf.ID, cf.Name, mergeDest); err != nil {
			return err
		}
	}
	childItems, err := st.inEventChildItems(ctx, nodeID)
	if err != nil {
		return err
	}
	for _, ci := range childItems {
		if err := st.placeItem(ctx, st.treeKind, ci.ID, ci.Name, mergeDest); err != nil {
			return err
		}
	}

	// If skipped children still sit under this folder, keep it (and the event) so their
	// trashed location stays intact; otherwise the merged-away folder is removed, re-homing any
	// stray cross-event children onto the merge target.
	remaining, err := st.inEventChildCount(ctx, nodeID)
	if err != nil {
		return err
	}
	if remaining > 0 {
		return nil
	}
	if _, err := st.tx.ExecContext(ctx,
		`UPDATE folders SET parent_id=? WHERE user_id=? AND parent_id=?`, mergeDest, st.userID, nodeID); err != nil {
		return err
	}
	if _, err := st.tx.ExecContext(ctx,
		`UPDATE `+st.itemTable+` SET folder_id=? WHERE user_id=? AND folder_id=?`, mergeDest, st.userID, nodeID); err != nil {
		return err
	}
	_, err = st.tx.ExecContext(ctx, `DELETE FROM folders WHERE id=? AND user_id=?`, nodeID, st.userID)
	return err
}

func (st *restoreState) untrashFolderWholesale(ctx context.Context, nodeID string, destParent sql.NullString) error {
	ids, err := st.inEventSubtreeFolderIDs(ctx, nodeID)
	if err != nil {
		return err
	}
	args := []any{st.userID}
	for _, id := range ids {
		args = append(args, id)
	}
	if _, err := st.tx.ExecContext(ctx,
		`UPDATE folders SET deleted_at=NULL, trash_event_id=NULL WHERE user_id=? AND id IN (`+placeholders(len(ids))+`)`,
		args...); err != nil {
		return err
	}
	if _, err := st.tx.ExecContext(ctx,
		`UPDATE folders SET parent_id=?, updated_at=? WHERE id=? AND user_id=?`,
		destParent, time.Now().Unix(), nodeID, st.userID); err != nil {
		return err
	}
	itemArgs := append([]any{st.userID}, args[1:]...)
	itemArgs = append(itemArgs, st.eventID)
	_, err = st.tx.ExecContext(ctx,
		`UPDATE `+st.itemTable+` SET deleted_at=NULL, trash_event_id=NULL WHERE user_id=? AND folder_id IN (`+
			placeholders(len(ids))+`) AND trash_event_id=?`, itemArgs...)
	return err
}

func (st *restoreState) inEventChildFolders(ctx context.Context, parentID string) ([]TrashEntry, error) {
	rows, err := st.tx.QueryContext(ctx,
		`SELECT id, name FROM folders WHERE user_id=? AND parent_id=? AND trash_event_id=?`,
		st.userID, parentID, st.eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashEntry
	for rows.Next() {
		var e TrashEntry
		if err := rows.Scan(&e.ID, &e.Name); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (st *restoreState) inEventChildItems(ctx context.Context, parentID string) ([]TrashEntry, error) {
	rows, err := st.tx.QueryContext(ctx,
		`SELECT id, name FROM `+st.itemTable+` WHERE user_id=? AND folder_id=? AND trash_event_id=?`,
		st.userID, parentID, st.eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TrashEntry
	for rows.Next() {
		var e TrashEntry
		if err := rows.Scan(&e.ID, &e.Name); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (st *restoreState) inEventChildCount(ctx context.Context, parentID string) (int, error) {
	var folders, items int
	if err := st.tx.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM folders WHERE user_id=? AND parent_id=? AND trash_event_id=?`,
		st.userID, parentID, st.eventID).Scan(&folders); err != nil {
		return 0, err
	}
	if err := st.tx.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM `+st.itemTable+` WHERE user_id=? AND folder_id=? AND trash_event_id=?`,
		st.userID, parentID, st.eventID).Scan(&items); err != nil {
		return 0, err
	}
	return folders + items, nil
}

func (st *restoreState) inEventSubtreeFolderIDs(ctx context.Context, rootID string) ([]string, error) {
	return collectStrings(ctx, st.tx, `
		WITH RECURSIVE sub(id) AS (
			SELECT id FROM folders WHERE id=? AND user_id=? AND trash_event_id=?
			UNION ALL
			SELECT f.id FROM folders f JOIN sub ON f.parent_id = sub.id WHERE f.trash_event_id=?
		)
		SELECT id FROM sub`, rootID, st.userID, st.eventID, st.eventID)
}

// pathLabel renders the destination folder as a slash path for conflict display.
func (st *restoreState) pathLabel(ctx context.Context, dest sql.NullString) (string, error) {
	if !dest.Valid {
		return "/", nil
	}
	names, err := st.ancestorNames(ctx, dest)
	if err != nil {
		return "/", err
	}
	return "/" + strings.Join(names, "/"), nil
}

// cleanupEvent removes the trash_event row once nothing references it any longer.
func (st *restoreState) cleanupEvent(ctx context.Context, eventID string) error {
	if eventID == "" {
		return nil
	}
	var n int
	for _, table := range []string{"folders", "documents", "signatures", "exports"} {
		var c int
		if err := st.tx.QueryRowContext(ctx,
			`SELECT COUNT(1) FROM `+table+` WHERE trash_event_id=?`, eventID).Scan(&c); err != nil {
			return err
		}
		n += c
	}
	if n > 0 {
		return nil
	}
	_, err := st.tx.ExecContext(ctx,
		`DELETE FROM trash_events WHERE id=? AND user_id=?`, eventID, st.userID)
	return err
}

// --- permanent deletion ---

// HardDeleteEvent permanently removes one trash event (all the rows it carries, their exports
// and blobs). Returns the blob paths to delete from disk.
func (s *Store) HardDeleteEvent(ctx context.Context, userID, eventID string) ([]string, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM trash_events WHERE id=? AND user_id=?`, eventID, userID).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, ErrNotFound
	}
	return s.purgeEvents(ctx, []string{eventID})
}

// EmptyTrash permanently removes every trash event for a user.
func (s *Store) EmptyTrash(ctx context.Context, userID string) ([]string, error) {
	ids, err := collectStrings(ctx, s.db, `SELECT id FROM trash_events WHERE user_id=?`, userID)
	if err != nil {
		return nil, err
	}
	return s.purgeEvents(ctx, ids)
}

// PurgeExpired permanently removes every event created before cutoff (across all users) and
// returns the blob paths to delete. An event's age is its delete time, so everything trashed
// together expires together.
func (s *Store) PurgeExpired(ctx context.Context, cutoff time.Time) ([]string, error) {
	ids, err := collectStrings(ctx, s.db, `SELECT id FROM trash_events WHERE created_at < ?`, cutoff.Unix())
	if err != nil {
		return nil, err
	}
	return s.purgeEvents(ctx, ids)
}

// purgeEvents hard-deletes the given events in one transaction, returning every blob path freed.
func (s *Store) purgeEvents(ctx context.Context, eventIDs []string) ([]string, error) {
	var paths []string
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		for _, eid := range eventIDs {
			// Documents carry their signed exports (which ride hidden, outside any event) — purge
			// those by document first, collecting their blobs.
			docIDs, err := collectStrings(ctx, tx, `SELECT id FROM documents WHERE trash_event_id=?`, eid)
			if err != nil {
				return err
			}
			for _, d := range docIDs {
				p, err := collectStrings(ctx, tx, `SELECT blob_path FROM exports WHERE document_id=?`, d)
				if err != nil {
					return err
				}
				paths = append(paths, p...)
				if _, err := tx.ExecContext(ctx, `DELETE FROM exports WHERE document_id=?`, d); err != nil {
					return err
				}
			}
			for _, table := range []string{"documents", "signatures", "exports"} {
				p, err := collectStrings(ctx, tx, `SELECT blob_path FROM `+table+` WHERE trash_event_id=?`, eid)
				if err != nil {
					return err
				}
				paths = append(paths, p...)
			}
			// Items before folders so an item's folder_id is gone before the folder row (whose
			// FK would otherwise SET NULL) is removed.
			for _, table := range []string{"documents", "signatures", "exports", "folders"} {
				if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE trash_event_id=?`, eid); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM trash_events WHERE id=?`, eid); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}
