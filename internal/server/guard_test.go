package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestAllowedHost(t *testing.T) {
	allowed := []string{"localhost", "localhost:7373", "127.0.0.1", "127.0.0.1:7373", "[::1]:7373", "::1"}
	for _, host := range allowed {
		if !allowedHost(host) {
			t.Errorf("allowedHost(%q) = false", host)
		}
	}
	denied := []string{"", "evil.example", "evil.example:7373", "192.168.1.10:7373", "localhost.evil.example", "127.0.0.1.evil.example"}
	for _, host := range denied {
		if allowedHost(host) {
			t.Errorf("allowedHost(%q) = true", host)
		}
	}
}

func TestAllowedOrigin(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://localhost:7373/api/health", nil)
	req.Host = "localhost:7373"
	for _, origin := range []string{"", "http://localhost:7373", "https://localhost:7373"} {
		req.Header.Set("Origin", origin)
		if !allowedOrigin(req) {
			t.Errorf("origin %q rejected", origin)
		}
	}
	for _, origin := range []string{"https://evil.example", "http://127.0.0.1:7373", "null", "file:///tmp/x"} {
		req.Header.Set("Origin", origin)
		if allowedOrigin(req) {
			t.Errorf("origin %q accepted", origin)
		}
	}
}

func TestLocalOnlyRejectsBadHostAndOrigin(t *testing.T) {
	called := false
	h := LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	cases := []struct {
		name   string
		host   string
		origin string
		want   int
	}{
		{"same origin", "localhost:7373", "http://localhost:7373", http.StatusOK},
		{"no origin", "127.0.0.1:7373", "", http.StatusOK},
		{"bad host", "evil.example", "", http.StatusMisdirectedRequest},
		{"bad origin", "localhost:7373", "https://evil.example", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, "http://localhost:7373/api/health", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
			if tc.want == http.StatusOK && !called {
				t.Fatal("allowed request did not reach the handler")
			}
			if tc.want != http.StatusOK && called {
				t.Fatal("blocked request reached the handler")
			}
		})
	}
}

func TestLocalOnlyRateLimitsChat(t *testing.T) {
	h := LocalOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	var last int
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "http://localhost:7373/api/chat", strings.NewReader(`{"message":"hi"}`))
		req.Host = "localhost:7373"
		req.RemoteAddr = "127.0.0.1:50000"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("sixth chat request = %d, want 429", last)
	}
}

func TestRateLimiterRefills(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := newRateLimiter(1, 2)
	l.now = func() time.Time { return now }

	if ok, _ := l.allow("k"); !ok {
		t.Fatal("first token denied")
	}
	if ok, _ := l.allow("k"); !ok {
		t.Fatal("second token denied")
	}
	if ok, _ := l.allow("k"); ok {
		t.Fatal("third token allowed with burst 2")
	}
	now = now.Add(time.Second)
	if ok, _ := l.allow("k"); !ok {
		t.Fatal("token did not refill after one second")
	}
}

func TestWebSocketRejectsForeignOrigin(t *testing.T) {
	s, _ := newAgentServer(t)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	url := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	d := websocket.Dialer{}
	conn, resp, err := d.Dial(url, http.Header{"Origin": []string{"https://evil.example"}})
	if err == nil {
		_ = conn.Close()
		t.Fatal("foreign origin completed the websocket upgrade")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("upgrade status = %v, want 403", resp)
	}
}

func TestLocalOnlyRateLimitResponseShape(t *testing.T) {
	if got := clientKey(&http.Request{RemoteAddr: "[::1]:1234"}); got != "::1" {
		t.Fatalf("clientKey IPv6 = %q", got)
	}
	if got := clientKey(&http.Request{}); got != "local" {
		t.Fatalf("clientKey empty = %q", got)
	}
}
