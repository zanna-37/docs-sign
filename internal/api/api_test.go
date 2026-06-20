package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-pdf/fpdf"

	"docs-sign/internal/auth"
	"docs-sign/internal/blob"
	"docs-sign/internal/config"
	"docs-sign/internal/crypto"
	"docs-sign/internal/pdfproc"
	"docs-sign/internal/session"
	"docs-sign/internal/store"
)

type testEnv struct {
	ts      *httptest.Server
	client  *http.Client
	dataDir string
	sigPNG  []byte
}

func newTestEnv(t *testing.T, renderer *pdfproc.Renderer) *testEnv {
	t.Helper()
	dataDir := t.TempDir()
	blobs, err := blob.New(filepath.Join(dataDir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dataDir, "meta.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	cfg := &config.Config{
		Dev:            true,
		ExportDPI:      96,
		MaxUploadBytes: 64 << 20,
		KDF:            crypto.KDFParams{Time: 1, Memory: 8 * 1024, Threads: 1},
	}
	sessions := session.NewManager(time.Hour, 24*time.Hour)
	authSvc := auth.NewService(st, blobs, sessions, cfg.KDF)
	srv := NewServer(cfg, st, blobs, sessions, authSvc, renderer)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)

	jar, _ := cookiejar.New(nil)
	return &testEnv{ts: ts, client: &http.Client{Jar: jar}, dataDir: dataDir, sigPNG: makeSignaturePNG()}
}

func makeSignaturePNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 200, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 30, B: 180, A: 200})
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func makePDF(t *testing.T) []byte {
	t.Helper()
	doc := fpdf.New("P", "pt", "A4", "")
	doc.AddPage()
	doc.SetFont("Arial", "B", 28)
	doc.SetXY(72, 100)
	doc.Cell(400, 30, "Sample page")
	var buf bytes.Buffer
	if err := doc.Output(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (e *testEnv) postJSON(t *testing.T, path string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(body)
	req, _ := http.NewRequest(http.MethodPost, e.ts.URL+path, &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-With", "fetch")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func (e *testEnv) upload(t *testing.T, path, name string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(content)
	_ = mw.WriteField("name", name)
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, e.ts.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Requested-With", "fetch")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response, want int) T {
	t.Helper()
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		t.Fatalf("status=%d want=%d body=%s", resp.StatusCode, want, body)
	}
	var v T
	if len(body) > 0 {
		if err := json.Unmarshal(body, &v); err != nil {
			t.Fatalf("decode: %v body=%s", err, body)
		}
	}
	return v
}

func TestAPIFullFlow(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	e := newTestEnv(t, renderer)

	// First run requires setup.
	st := decode[map[string]bool](t, e.postReq(t, http.MethodGet, "/api/setup/status"), 200)
	if !st["needsSetup"] {
		t.Fatal("expected needsSetup true")
	}

	// Setup admin.
	setup := decode[map[string]string](t, e.postJSON(t, "/api/setup", map[string]string{"username": "admin", "password": "adminpassword"}), 201)
	if setup["recoveryCode"] == "" {
		t.Fatal("expected recovery code")
	}

	// Login.
	decode[map[string]any](t, e.postJSON(t, "/api/login", map[string]string{"username": "admin", "password": "adminpassword"}), 200)

	// Upload signature.
	sig := decode[signatureDTO](t, e.upload(t, "/api/signatures", "sig.png", e.sigPNG), 201)
	if sig.ID == "" || sig.Width != 200 || sig.Height != 80 {
		t.Fatalf("unexpected signature dto: %+v", sig)
	}

	// Upload document.
	doc := decode[documentDTO](t, e.upload(t, "/api/documents", "doc.pdf", makePDF(t)), 201)
	if doc.ID == "" || doc.PageCount != 1 {
		t.Fatalf("unexpected document dto: %+v", doc)
	}

	// Sign it.
	signBody := map[string]any{
		"placements": []map[string]any{
			{"signatureId": sig.ID, "page": 0, "x": 60, "y": 80, "w": 180, "h": 70, "rotation": 12},
		},
	}
	exp := decode[exportDTO](t, e.postJSON(t, "/api/documents/"+doc.ID+"/sign", signBody), 201)
	if exp.ID == "" || exp.PageCount != 1 {
		t.Fatalf("unexpected export dto: %+v", exp)
	}

	// Download the export.
	resp := e.postReq(t, http.MethodGet, "/api/exports/"+exp.ID+"/file")
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("download status %d", resp.StatusCode)
	}
	out, _ := io.ReadAll(resp.Body)
	if !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatal("export is not a PDF")
	}
	// Flattened: original text and the raw signature bytes must not survive.
	if bytes.Contains(out, []byte("Sample page")) {
		t.Fatal("flattened export still contains original text")
	}
	if bytes.Contains(out, e.sigPNG) {
		t.Fatal("flattened export still contains the raw signature PNG")
	}
	// And it must be a valid, single-page PDF.
	if pc, err := renderer.PageCount(out); err != nil || pc != 1 {
		t.Fatalf("flattened reload pc=%d err=%v", pc, err)
	}

	assertBlobsEncrypted(t, e.dataDir)
}

// assertBlobsEncrypted walks the on-disk blob store and verifies every blob is in the
// encrypted container format and never contains plaintext document/signature markers.
func assertBlobsEncrypted(t *testing.T, dataDir string) {
	t.Helper()
	root := filepath.Join(dataDir, "blobs")
	count := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		count++
		data, _ := os.ReadFile(path)
		if !bytes.HasPrefix(data, []byte("DSB1")) {
			t.Fatalf("blob %s is not encrypted (bad magic)", path)
		}
		if bytes.Contains(data, []byte("%PDF")) || bytes.Contains(data, []byte("Sample page")) {
			t.Fatalf("blob %s appears to contain plaintext", path)
		}
		return nil
	})
	if count == 0 {
		t.Fatal("expected at least one blob on disk")
	}
}

func TestVersionEndpoint(t *testing.T) {
	renderer, err := pdfproc.New()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	e := newTestEnv(t, renderer)

	// Public: reachable without authentication and returns the stamped build info
	// (defaults when the test binary is built without ldflags).
	v := decode[map[string]string](t, e.postReq(t, http.MethodGet, "/api/version"), 200)
	if v["version"] == "" {
		t.Fatalf("expected a version, got %+v", v)
	}
	if _, ok := v["commit"]; !ok {
		t.Fatalf("expected a commit field, got %+v", v)
	}
	// repoURL is a build constant (not stamped in tests), so it carries its default and lets
	// the SPA link the version to its GitHub release.
	if v["repoURL"] == "" {
		t.Fatalf("expected a repoURL, got %+v", v)
	}
}

// postReq issues a request with the CSRF header and no body (used for GETs here).
func (e *testEnv) postReq(t *testing.T, method, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, e.ts.URL+path, nil)
	req.Header.Set("X-Requested-With", "fetch")
	resp, err := e.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}
