package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwarmComingSoonRoute(t *testing.T) {
	s, _ := newWbServer(t)
	page := filepath.Join(s.ws.Root, "workspace/ui/swarm.html")
	if err := os.WriteFile(page, []byte("<!doctype html><h1>coming soon</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/swarm", "/swarm/", "/swarm.html"} {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s -> %d: %s", path, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("GET %s content-type = %q", path, got)
		}
		if !strings.Contains(rec.Body.String(), "coming soon") {
			t.Fatalf("GET %s body = %q", path, rec.Body.String())
		}
	}
}
