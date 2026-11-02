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

	"github.com/dxn111/EVOLV/internal/workspace"
)

func TestGenesisRouteAndAPI(t *testing.T) {
	s, _ := newWbServer(t)
	page := filepath.Join(s.ws.Dir(workspace.UIRel()), "genesis.html")
	_ = os.MkdirAll(filepath.Dir(page), 0o755)
	if err := os.WriteFile(page, []byte("<!doctype html><h1>Genesis — The Living AI Workspace</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/genesis", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Genesis") {
		t.Fatalf("genesis page -> %d: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/genesis", strings.NewReader(`{"mission":"build a tiny adder"}`))
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start genesis -> %d: %s", rec.Code, rec.Body.String())
	}
	var start map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &start); err != nil {
		t.Fatal(err)
	}
	if start["status"] != "running" {
		t.Fatalf("start state = %v", start)
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/genesis", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get genesis -> %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/genesis/stop", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("stop genesis -> %d", rec.Code)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && s.execEngine.Snapshot().Status == "running" {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestGenesisRejectsEmptyMission(t *testing.T) {
	s, _ := newWbServer(t)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/genesis", strings.NewReader(`{"mission":" "}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty mission -> %d", rec.Code)
	}
}
