package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSwarmRoute(t *testing.T) {
	s, _ := newWbServer(t)
	page := filepath.Join(s.ws.Root, "workspace/ui/swarm.html")
	if err := os.WriteFile(page, []byte("<!doctype html><h1>swarm mode</h1>"), 0o644); err != nil {
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
		if !strings.Contains(strings.ToLower(rec.Body.String()), "swarm mode") {
			t.Fatalf("GET %s body = %q", path, rec.Body.String())
		}
	}
}

func TestSwarmAPIStartStatusStop(t *testing.T) {
	s, _ := newWbServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/swarm", strings.NewReader(`{"mission":"build a tiny timer"}`))
	req.Header.Set("Content-Type", "application/json")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start swarm -> %d: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}
	if start["status"] != "running" || start["mission"] != "build a tiny timer" {
		t.Fatalf("bad start state: %v", start)
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/swarm", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get swarm -> %d", rec.Code)
	}
	var current map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &current); err != nil {
		t.Fatal(err)
	}
	if current["id"] != start["id"] {
		t.Fatalf("swarm id changed: %v vs %v", current["id"], start["id"])
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/swarm/stop", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("stop swarm -> %d", rec.Code)
	}
}

func TestSwarmAPIRejectsEmptyMission(t *testing.T) {
	s, _ := newWbServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/swarm", strings.NewReader(`{"mission":"  "}`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty mission -> %d", rec.Code)
	}
}
