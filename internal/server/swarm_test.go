package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dxn111/EVOLV/internal/bus"
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
	// Let the run goroutine observe cancellation and finish its persistence
	// write before TempDir cleanup removes the workspace.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && s.swarm.Snapshot().Status == "running" {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
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

func TestSwarmStatePersists(t *testing.T) {
	s, _ := newWbServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/swarm", strings.NewReader(`{"mission":"persist this colony","agents":10}`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start -> %d", rec.Code)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && s.swarm.Snapshot().Status == "running" {
		time.Sleep(20 * time.Millisecond)
	}
	if got := s.swarm.Snapshot().Status; got != "done" {
		t.Fatalf("run did not finish: %s", got)
	}

	// A fresh server pointed at the same genome must restore the last run.
	reloaded := New(s.ws, bus.New(), nil, nil)
	st := reloaded.swarm.Snapshot()
	if st.ID == "" || st.Mission != "persist this colony" || len(st.Messages) == 0 {
		t.Fatalf("restored state = %+v", st)
	}
	if len(st.Agents) != 10 {
		t.Fatalf("restored agents = %d", len(st.Agents))
	}
}

func TestSwarmRunHistory(t *testing.T) {
	s, _ := newWbServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/swarm", strings.NewReader(`{"mission":"archive this run","agents":5}`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start -> %d", rec.Code)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && s.swarm.Snapshot().Status == "running" {
		time.Sleep(20 * time.Millisecond)
	}
	id := s.swarm.Snapshot().ID
	time.Sleep(30 * time.Millisecond)

	var list struct {
		Runs []swarmRunSummary `json:"runs"`
	}
	// Archiving happens in the state publish callback; wait for it instead of
	// relying on a fixed sleep under the race detector.
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		rec = httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/swarm/runs", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("runs -> %d", rec.Code)
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatal(err)
		}
		if len(list.Runs) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(list.Runs) == 0 || list.Runs[0].ID != id {
		t.Fatalf("runs = %+v, want %s", list.Runs, id)
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/swarm/history?id="+id, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("history -> %d: %s", rec.Code, rec.Body.String())
	}
	var st map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &st); err != nil {
		t.Fatal(err)
	}
	if st["id"] != id || st["status"] != "done" {
		t.Fatalf("history state = %v", st)
	}
}
