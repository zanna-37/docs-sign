package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"docs-sign/internal/pdfproc"
)

// doJSON issues a request with an optional JSON body and the CSRF header.
func (e *testEnv) doJSON(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var rdr *bytes.Buffer = &bytes.Buffer{}
	if body != nil {
		if err := json.NewEncoder(rdr).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, _ := http.NewRequest(method, e.ts.URL+path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Requested-With", "fetch")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (e *testEnv) setupAndLogin(t *testing.T) {
	t.Helper()
	decode[map[string]string](t, e.postJSON(t, "/api/setup", map[string]string{"username": "admin", "password": "adminpassword"}), 201)
	decode[map[string]any](t, e.postJSON(t, "/api/login", map[string]string{"username": "admin", "password": "adminpassword"}), 200)
}

type folderListResp struct {
	Folders []folderDTO `json:"folders"`
	Path    []folderDTO `json:"path"`
}

func listFolders(t *testing.T, e *testEnv, query string) folderListResp {
	t.Helper()
	return decode[folderListResp](t, e.postReq(t, http.MethodGet, "/api/folders"+query), 200)
}

func TestFolderCRUDAPI(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()

	newAuthed := func(t *testing.T) *testEnv {
		e := newTestEnv(t, renderer)
		e.setupAndLogin(t)
		return e
	}

	t.Run("create, nest, list, breadcrumb", func(t *testing.T) {
		e := newAuthed(t)
		contracts := decode[folderDTO](t, e.postJSON(t, "/api/folders",
			map[string]string{"kind": "document", "name": "Contracts"}), 201)
		acme := decode[folderDTO](t, e.postJSON(t, "/api/folders",
			map[string]string{"kind": "document", "parentId": contracts.ID, "name": "Acme"}), 201)

		if roots := listFolders(t, e, "?kind=document"); len(roots.Folders) != 1 || roots.Folders[0].Name != "Contracts" {
			t.Fatalf("root folders: %+v", roots)
		}
		kids := listFolders(t, e, "?kind=document&parent="+contracts.ID)
		if len(kids.Folders) != 1 || kids.Folders[0].ID != acme.ID {
			t.Fatalf("children: %+v", kids)
		}
		if len(kids.Path) != 1 || kids.Path[0].Name != "Contracts" {
			t.Fatalf("breadcrumb: %+v", kids.Path)
		}
	})

	t.Run("validation and conflicts", func(t *testing.T) {
		e := newAuthed(t)
		decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "A"}), 201)
		// Invalid kind -> 400; duplicate sibling -> 409.
		mustStatus(t, e.postJSON(t, "/api/folders", map[string]string{"kind": "bogus", "name": "X"}), 400)
		mustStatus(t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "A"}), 409)
	})

	t.Run("rename and move", func(t *testing.T) {
		e := newAuthed(t)
		a := decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "A"}), 201)
		b := decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "B"}), 201)
		child := decode[folderDTO](t, e.postJSON(t, "/api/folders",
			map[string]string{"kind": "document", "parentId": a.ID, "name": "Child"}), 201)

		// Rename collision against B.
		mustStatus(t, e.doJSON(t, http.MethodPatch, "/api/folders/"+a.ID, map[string]any{"name": "B"}), 409)
		mustStatus(t, e.doJSON(t, http.MethodPatch, "/api/folders/"+a.ID, map[string]any{"name": "A2"}), 200)

		// Move A into its own descendant -> 400.
		mustStatus(t, e.doJSON(t, http.MethodPatch, "/api/folders/"+a.ID,
			map[string]any{"move": map[string]any{"parentId": child.ID}}), 400)
		// Move B under A is fine.
		mustStatus(t, e.doJSON(t, http.MethodPatch, "/api/folders/"+b.ID,
			map[string]any{"move": map[string]any{"parentId": a.ID}}), 200)
		if kids := listFolders(t, e, "?kind=document&parent="+a.ID); len(kids.Folders) != 2 {
			t.Fatalf("A should now have 2 children, got %+v", kids.Folders)
		}
	})
}

func TestDocumentsInFoldersAPI(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	e := newTestEnv(t, renderer)
	e.setupAndLogin(t)

	folder := decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "F"}), 201)
	pdf := makePDF(t)

	// Upload into the folder.
	doc := decode[documentDTO](t, e.upload(t, "/api/documents?folder="+folder.ID, "a.pdf", pdf), 201)
	if doc.FolderID != folder.ID {
		t.Fatalf("uploaded document should carry folderId, got %+v", doc)
	}
	// Folder-scoped listing sees it; root does not.
	if list := listDocs(t, e, "?folder="+folder.ID); len(list) != 1 {
		t.Fatalf("folder should hold 1 document, got %d", len(list))
	}
	if list := listDocs(t, e, ""); len(list) != 0 {
		t.Fatalf("root should hold 0 documents, got %d", len(list))
	}

	// Duplicate name without overwrite -> 409; with overwrite -> 201, original trashed.
	mustStatus(t, e.upload(t, "/api/documents?folder="+folder.ID, "a.pdf", pdf), 409)
	decode[documentDTO](t, e.upload(t, "/api/documents?folder="+folder.ID+"&overwrite=true", "a.pdf", pdf), 201)
	if list := listDocs(t, e, "?folder="+folder.ID); len(list) != 1 {
		t.Fatalf("folder should still hold 1 active document, got %d", len(list))
	}
	if ev := listTrash(t, e); len(ev.Events) != 1 {
		t.Fatalf("overwrite should have trashed the original, got %+v", ev.Events)
	}

	// Move the surviving document to root, then a collision is rejected.
	cur := listDocs(t, e, "?folder="+folder.ID)[0]
	mustStatus(t, e.doJSON(t, http.MethodPatch, "/api/documents/"+cur.ID,
		map[string]any{"move": map[string]any{"folderId": nil}}), 200)
	if list := listDocs(t, e, ""); len(list) != 1 {
		t.Fatalf("root should now hold the moved document, got %d", len(list))
	}
}

func TestTrashEventsAPI(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	e := newTestEnv(t, renderer)
	e.setupAndLogin(t)
	pdf := makePDF(t)

	folder := decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "docs"}), 201)
	inFolder := decode[documentDTO](t, e.upload(t, "/api/documents?folder="+folder.ID, "deep.pdf", pdf), 201)
	loose := decode[documentDTO](t, e.upload(t, "/api/documents", "loose.pdf", pdf), 201)

	// Trash the loose doc, then the folder -> two independent events.
	mustStatus(t, e.doJSON(t, http.MethodDelete, "/api/documents/"+loose.ID, nil), 200)
	mustStatus(t, e.doJSON(t, http.MethodDelete, "/api/folders/"+folder.ID, nil), 200)
	trash := listTrash(t, e)
	if len(trash.Events) != 2 {
		t.Fatalf("want 2 trash events, got %+v", trash.Events)
	}

	// Walk the folder event: its child deep.pdf is visible.
	var folderEvent trashEventDTO
	for _, ev := range trash.Events {
		if ev.RootKind == "folder" {
			folderEvent = ev
		}
	}
	walk := decode[struct {
		Entries []trashEntryDTO `json:"entries"`
	}](t, e.postReq(t, http.MethodGet, "/api/trash/events/"+folderEvent.EventID), 200)
	if len(walk.Entries) != 1 || walk.Entries[0].Name != "deep.pdf" {
		t.Fatalf("walk entries: %+v", walk.Entries)
	}

	// Restore the folder event; the loose doc stays trashed.
	mustStatus(t, e.postJSON(t, "/api/trash/folder/"+folderEvent.RootID+"/restore", map[string]any{}), 200)
	if list := listDocs(t, e, "?folder="+folder.ID); len(list) != 1 || list[0].ID != inFolder.ID {
		t.Fatalf("deep.pdf should be restored into its folder, got %+v", list)
	}
	if trash := listTrash(t, e); len(trash.Events) != 1 || trash.Events[0].RootID != loose.ID {
		t.Fatalf("only the loose doc event should remain, got %+v", trash.Events)
	}

	// Purge the remaining event; trash is empty.
	remaining := listTrash(t, e).Events[0]
	mustStatus(t, e.doJSON(t, http.MethodDelete, "/api/trash/events/"+remaining.EventID, nil), 200)
	if trash := listTrash(t, e); len(trash.Events) != 0 {
		t.Fatalf("trash should be empty after purge, got %+v", trash.Events)
	}
}

func TestRestoreConflictAPI(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	e := newTestEnv(t, renderer)
	e.setupAndLogin(t)
	pdf := makePDF(t)

	folder := decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "F"}), 201)
	orig := decode[documentDTO](t, e.upload(t, "/api/documents?folder="+folder.ID, "a.pdf", pdf), 201)
	mustStatus(t, e.doJSON(t, http.MethodDelete, "/api/documents/"+orig.ID, nil), 200)
	// A new active a.pdf now blocks the restore.
	decode[documentDTO](t, e.upload(t, "/api/documents?folder="+folder.ID, "a.pdf", pdf), 201)

	// Restoring without a resolution returns 409 + the conflict.
	resp := e.postJSON(t, "/api/trash/document/"+orig.ID+"/restore", map[string]any{})
	conf := decode[struct {
		Conflicts []restoreConflictDTO `json:"conflicts"`
	}](t, resp, 409)
	if len(conf.Conflicts) != 1 || conf.Conflicts[0].ID != orig.ID || conf.Conflicts[0].DestPath != "/F" {
		t.Fatalf("expected one conflict at /F, got %+v", conf.Conflicts)
	}

	// Resolve with a rename; both files end up in the folder.
	mustStatus(t, e.postJSON(t, "/api/trash/document/"+orig.ID+"/restore", map[string]any{
		"resolutions": map[string]any{orig.ID: map[string]any{"action": "rename", "newName": "a-restored.pdf"}},
	}), 200)
	if list := listDocs(t, e, "?folder="+folder.ID); len(list) != 2 {
		t.Fatalf("folder should hold both files after rename-restore, got %d", len(list))
	}
	if trash := listTrash(t, e); len(trash.Events) != 0 {
		t.Fatalf("trash should be empty, got %+v", trash.Events)
	}
}

// --- small helpers ---

func mustStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	defer resp.Body.Close()
	if resp.StatusCode != want {
		t.Fatalf("status=%d want=%d", resp.StatusCode, want)
	}
}

func listDocs(t *testing.T, e *testEnv, query string) []documentDTO {
	t.Helper()
	return decode[struct {
		Documents []documentDTO `json:"documents"`
	}](t, e.postReq(t, http.MethodGet, "/api/documents"+query), 200).Documents
}

func listSigs(t *testing.T, e *testEnv, query string) []signatureDTO {
	t.Helper()
	return decode[struct {
		Signatures []signatureDTO `json:"signatures"`
	}](t, e.postReq(t, http.MethodGet, "/api/signatures"+query), 200).Signatures
}

// The signing editor needs every signature and document regardless of folder; ?all=true must
// return foldered items that folder-scoped listing (root) hides.
func TestFlatListingIncludesFolderedItems(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	e := newTestEnv(t, renderer)
	e.setupAndLogin(t)

	sf := decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "signature", "name": "Sigs"}), 201)
	decode[signatureDTO](t, e.upload(t, "/api/signatures?folder="+sf.ID, "s.png", e.sigPNG), 201)
	df := decode[folderDTO](t, e.postJSON(t, "/api/folders", map[string]string{"kind": "document", "name": "Docs"}), 201)
	decode[documentDTO](t, e.upload(t, "/api/documents?folder="+df.ID, "d.pdf", makePDF(t)), 201)

	// Folder-scoped root listings hide the foldered items.
	if len(listSigs(t, e, "")) != 0 || len(listDocs(t, e, "")) != 0 {
		t.Fatal("root listings should be empty")
	}
	// The flat listing surfaces them for the editor.
	if len(listSigs(t, e, "?all=true")) != 1 {
		t.Fatal("?all=true should list the foldered signature")
	}
	if len(listDocs(t, e, "?all=true")) != 1 {
		t.Fatal("?all=true should list the foldered document")
	}
}

func TestEnsureFolderPathAPI(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()

	e := newTestEnv(t, renderer)
	e.setupAndLogin(t)

	// Recreate a nested path and file a document into the leaf — the folder-upload flow.
	leaf := decode[folderDTO](t, e.postJSON(t, "/api/folders/ensure",
		map[string]any{"kind": "document", "path": []string{"reports", "2026"}}), 201)
	if leaf.Name != "2026" {
		t.Fatalf("leaf should be the deepest segment, got %+v", leaf)
	}
	decode[documentDTO](t, e.upload(t, "/api/documents?folder="+leaf.ID, "q1.pdf", makePDF(t)), 201)
	if docs := listDocs(t, e, "?folder="+leaf.ID); len(docs) != 1 || docs[0].Name != "q1.pdf" {
		t.Fatalf("document should land in the recreated leaf, got %+v", docs)
	}

	// Re-ensuring a path that shares the prefix reuses "reports" and only adds the new leaf.
	other := decode[folderDTO](t, e.postJSON(t, "/api/folders/ensure",
		map[string]any{"kind": "document", "path": []string{"reports", "2025"}}), 201)
	if other.ID == leaf.ID {
		t.Fatal("diverging leaf should be a distinct folder")
	}
	roots := listFolders(t, e, "?kind=document")
	if len(roots.Folders) != 1 || roots.Folders[0].Name != "reports" {
		t.Fatalf("prefix folder should be reused, got %+v", roots)
	}
	reports := roots.Folders[0]
	if kids := listFolders(t, e, "?kind=document&parent="+reports.ID); len(kids.Folders) != 2 {
		t.Fatalf("reports should hold two year folders, got %+v", kids.Folders)
	}

	// An empty path is rejected.
	if resp := e.postJSON(t, "/api/folders/ensure",
		map[string]any{"kind": "document", "path": []string{}}); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty path should be 400, got %d", resp.StatusCode)
	}
}

func listTrash(t *testing.T, e *testEnv) struct {
	Events        []trashEventDTO `json:"events"`
	RetentionDays int             `json:"retentionDays"`
} {
	t.Helper()
	return decode[struct {
		Events        []trashEventDTO `json:"events"`
		RetentionDays int             `json:"retentionDays"`
	}](t, e.postReq(t, http.MethodGet, "/api/trash"), 200)
}
