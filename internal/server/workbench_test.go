package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dxn111/EVOLV/internal/bus"
	"github.com/dxn111/EVOLV/internal/workspace"
)

// newWbServer boots a server whose project root is a fresh temp dir.
func newWbServer(t *testing.T) (*Server, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // os.UserHomeDir uses this on Windows
	ws, err := workspace.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	proj := t.TempDir()
	ws.Config.ProjectDir = proj
	return New(ws, bus.New(), nil, nil), proj
}

func wbGET(t *testing.T, s *Server, url string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", url, nil))
	if rec.Code != 200 {
		t.Fatalf("GET %s -> %d: %s", url, rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	return out
}

func wbPOST(t *testing.T, s *Server, url, body string, want int) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("POST %s -> %d (want %d): %s", url, rec.Code, want, rec.Body.String())
	}
	var out map[string]any
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out
}

// TestWbRoot: unbound → organism home; bound → project dir.
func TestWbRoot(t *testing.T) {
	s, proj := newWbServer(t)
	out := wbGET(t, s, "/api/wb/root")
	if out["root"] != proj || out["name"] != filepath.Base(proj) {
		t.Fatalf("root wrong: %v", out)
	}
}

// TestWbTree: dirs first, sorted, skip-list respected, lazy nesting works.
func TestWbTree(t *testing.T) {
	s, proj := newWbServer(t)
	os.MkdirAll(filepath.Join(proj, "src", "deep"), 0o755)
	os.WriteFile(filepath.Join(proj, "README.md"), []byte("hi"), 0o644)
	os.WriteFile(filepath.Join(proj, "src", "main.go"), []byte("package main"), 0o644)
	os.MkdirAll(filepath.Join(proj, "node_modules", "junk"), 0o755)

	out := wbGET(t, s, "/api/wb/tree")
	ents := out["entries"].([]any)
	var names []string
	for _, e := range ents {
		names = append(names, e.(map[string]any)["name"].(string))
	}
	// node_modules skipped, dirs before files
	if len(names) != 2 || names[0] != "src" || names[1] != "README.md" {
		t.Fatalf("tree wrong: %v", names)
	}
	// open=1 nests one level down
	out = wbGET(t, s, "/api/wb/tree?open=1&depth=2")
	src := out["entries"].([]any)[0].(map[string]any)
	if src["name"] != "src" || src["open"] != true {
		t.Fatalf("src not open: %v", src)
	}
	kids := src["kids"].([]any)
	if len(kids) != 2 { // deep/ dir + main.go
		t.Fatalf("kids wrong: %v", kids)
	}
}

// TestWbWriteRead: write creates parents; read returns the content.
func TestWbWriteRead(t *testing.T) {
	s, proj := newWbServer(t)
	wbPOST(t, s, "/api/wb/write", `{"path":"pkg/util/helper.go","content":"package util"}`, 200)
	out := wbGET(t, s, "/api/wb/read?path=pkg/util/helper.go")
	if out["content"] != "package util" {
		t.Fatalf("read wrong: %v", out)
	}
	if _, err := os.Stat(filepath.Join(proj, "pkg", "util")); err != nil {
		t.Fatal("parents not created")
	}
	// the fs:change event fires on write
}

// TestWbBinaryRefused: NUL-bearing files are rejected with 415, not garbled.
func TestWbBinaryRefused(t *testing.T) {
	s, proj := newWbServer(t)
	os.WriteFile(filepath.Join(proj, "img.png"), []byte{'P', 'N', 0, 0, 1}, 0o644)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/wb/read?path=img.png", nil))
	if rec.Code != 415 {
		t.Fatalf("binary not refused: %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "binary") {
		t.Fatalf("no binary hint: %s", rec.Body.String())
	}
}

// TestWbEscapeRejected: path traversal and symlink escapes must fail.
func TestWbEscapeRejected(t *testing.T) {
	s, proj := newWbServer(t)

	t.Run("symlink", func(t *testing.T) {
		outside := t.TempDir()
		os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("top secret"), 0o644)
		if err := os.Symlink(outside, filepath.Join(proj, "leak")); err != nil {
			t.Skip("skipping symlink test on platform without symlink privilege:", err)
		}
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/wb/read?path=leak/secret.txt", nil))
		if rec.Code != 400 {
			t.Fatalf("symlink escape allowed: %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("traversal", func(t *testing.T) {
		// classic traversal shapes
		for _, p := range []string{"../outside.txt", "a/../../b.txt"} {
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/wb/read?path="+p, nil))
			if rec.Code == 200 {
				t.Fatalf("traversal allowed: %s", p)
			}
		}
	})
}

// TestWbRenameDelete: rename refuses clobber; delete refuses the root.
func TestWbRenameDelete(t *testing.T) {
	s, proj := newWbServer(t)
	os.WriteFile(filepath.Join(proj, "a.txt"), []byte("A"), 0o644)
	os.WriteFile(filepath.Join(proj, "b.txt"), []byte("B"), 0o644)
	// rename onto an existing file → 409
	wbPOST(t, s, "/api/wb/rename", `{"from":"a.txt","to":"b.txt"}`, 409)
	// clean rename
	wbPOST(t, s, "/api/wb/rename", `{"from":"a.txt","to":"renamed.txt"}`, 200)
	if _, err := os.Stat(filepath.Join(proj, "renamed.txt")); err != nil {
		t.Fatal("rename didn't happen")
	}
	// delete the root itself → refused
	wbPOST(t, s, "/api/wb/delete", `{"path":""}`, 400)
	wbPOST(t, s, "/api/wb/delete", `{"path":"."}`, 400)
	// normal delete
	wbPOST(t, s, "/api/wb/delete", `{"path":"renamed.txt"}`, 200)
	if _, err := os.Stat(filepath.Join(proj, "renamed.txt")); !os.IsNotExist(err) {
		t.Fatal("delete didn't happen")
	}
}

// TestWbTooBig: write over the 1 MB cap is rejected; read of a huge file refused.
func TestWbTooBig(t *testing.T) {
	s, proj := newWbServer(t)
	big := `{"path":"big.txt","content":"` + strings.Repeat("x", 1<<20+100) + `"}`
	wbPOST(t, s, "/api/wb/write", big, 400)
	huge := strings.Repeat("y", 600<<10)
	os.WriteFile(filepath.Join(proj, "huge.txt"), []byte(huge), 0o644)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/wb/read?path=huge.txt", nil))
	if rec.Code != 400 {
		t.Fatalf("big read not refused: %d", rec.Code)
	}
}

// TestWbMkdir: mkdir creates parents and appears in the tree.
func TestWbMkdir(t *testing.T) {
	s, proj := newWbServer(t)
	wbPOST(t, s, "/api/wb/mkdir", `{"path":"docs/guides"}`, 200)
	if fi, err := os.Stat(filepath.Join(proj, "docs", "guides")); err != nil || !fi.IsDir() {
		t.Fatal("mkdir didn't create parents")
	}
}

// TestWbSearch: project-wide grep — matches across files, line numbers,
// skip-list respected, cap enforced.
func TestWbSearch(t *testing.T) {
	s, proj := newWbServer(t)
	os.MkdirAll(filepath.Join(proj, "src"), 0o755)
	os.WriteFile(filepath.Join(proj, "README.md"), []byte("the quick brown fox\nlazy dog\n"), 0o644)
	os.WriteFile(filepath.Join(proj, "src", "main.go"), []byte("package main\n// the quick brown fox again\n"), 0o644)
	os.MkdirAll(filepath.Join(proj, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(proj, "node_modules", "x", "i.js"), []byte("the quick brown fox in deps\n"), 0o644)

	out := wbGET(t, s, "/api/wb/search?q=quick%20brown")
	hits := out["hits"].([]any)
	if len(hits) != 2 {
		t.Fatalf("want 2 hits (node_modules skipped), got %d: %v", len(hits), out)
	}
	h0 := hits[0].(map[string]any)
	if h0["path"] != "README.md" || h0["line"] != float64(1) {
		t.Fatalf("first hit wrong: %v", h0)
	}
	if out["files"] != float64(2) {
		t.Fatalf("files scanned wrong: %v", out["files"])
	}
	// case-insensitive
	out = wbGET(t, s, "/api/wb/search?q=QUICK")
	if len(out["hits"].([]any)) != 2 {
		t.Fatalf("case-insensitive search failed: %v", out)
	}
	// no query → 400
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/api/wb/search?q=", nil))
	if rec.Code != 400 {
		t.Fatalf("empty q not rejected: %d", rec.Code)
	}
}
