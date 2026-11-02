package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestHealthAndMetrics(t *testing.T) {
	s, _ := newWbServer(t)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health -> %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("health content-type = %q", got)
	}
	var health map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatalf("health json: %v", err)
	}
	for _, key := range []string{"status", "version", "uptime_seconds", "provider", "model", "agent", "agents", "tools", "plugins", "components", "model_calls", "model_errors", "last_model_latency_ms", "live_connections", "bus_subscribers", "bus_dropped", "goroutines", "requests", "errors", "snapshots"} {
		if _, ok := health[key]; !ok {
			t.Fatalf("health missing %q: %v", key, health)
		}
	}
	if health["status"] != "alive" {
		t.Fatalf("health status = %v", health["status"])
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics -> %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("metrics content-type = %q", got)
	}
	for _, want := range []string{"evolv_up 1", "evolv_http_requests_total", "evolv_ws_connections", "evolv_uptime_seconds", "evolv_http_request_duration_seconds_bucket", "evolv_model_calls_total", "evolv_tools", "evolv_bus_subscribers", "evolv_goroutines"} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("metrics missing %q:\n%s", want, rec.Body.String())
		}
	}
}

func TestMiddlewareSecurityHeadersAndRequestID(t *testing.T) {
	s, _ := newWbServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-ID", "req-abc_123")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "req-abc_123" {
		t.Fatalf("request id = %q", got)
	}
	for header, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "SAMEORIGIN",
		"Referrer-Policy":        "no-referrer",
		"Cache-Control":          "no-store",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Fatalf("%s = %q, want %q", header, got, want)
		}
	}

	// A hostile request id is replaced, never reflected into logs/headers.
	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("X-Request-ID", "bad\r\nid")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got == "" || strings.ContainsAny(got, "\r\n") {
		t.Fatalf("unsafe request id reflected: %q", got)
	}
}

func TestOversizedAPIBodyRejected(t *testing.T) {
	s, _ := newWbServer(t)
	body := strings.Repeat("x", maxAPIRequestBody+1)
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body -> %d (want 413): %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "too large") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestMiddlewareRecoversPanic(t *testing.T) {
	s := &Server{}
	h := s.withMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/panic", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic -> %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("panic body: %s", rec.Body.String())
	}
	if got := s.errors.Load(); got != 1 {
		t.Fatalf("errors = %d, want 1", got)
	}
	if got := s.requests.Load(); got != 1 {
		t.Fatalf("requests = %d, want 1", got)
	}
}

// The middleware must preserve http.Hijacker or the live socket dies at
// upgrade time. This is the end-to-end check for the response wrapper.
func TestWebSocketUpgradeThroughMiddleware(t *testing.T) {
	s, _ := newAgentServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("websocket upgrade through middleware: %v", err)
	}
	_ = conn.Close()
	s.Close()
}

func TestChatRejectsConcurrentGeneration(t *testing.T) {
	s, _ := newAgentServer(t)
	s.genCancel = func() {} // simulate an in-flight turn
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("concurrent chat -> %d (want 409): %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "already running") {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestHistoryPagination(t *testing.T) {
	s, _ := newAgentServer(t)
	_ = s.ag.Chat(context.Background(), "hello")

	get := func(query string) (*httptest.ResponseRecorder, map[string]any) {
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/history"+query, nil))
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec, out
	}

	rec, out := get("?limit=1&offset=0")
	if rec.Code != http.StatusOK {
		t.Fatalf("history -> %d: %s", rec.Code, rec.Body.String())
	}
	if out["total"] != float64(2) || out["limit"] != float64(1) || out["offset"] != float64(0) {
		t.Fatalf("bad history meta: %v", out)
	}
	msgs := out["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["role"] != "user" {
		t.Fatalf("bad first page: %v", msgs)
	}

	_, out = get("?limit=1&offset=1")
	msgs = out["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["role"] != "assistant" {
		t.Fatalf("bad second page: %v", msgs)
	}

	for _, bad := range []string{"?limit=0", "?limit=201", "?limit=x", "?offset=-1"} {
		rec, _ := get(bad)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s -> %d, want 400", bad, rec.Code)
		}
	}
}
