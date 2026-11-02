package server

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// LocalOnly wraps the organism's handler with the protections a localhost app
// actually needs: a valid loopback Host (DNS rebinding), a same-origin browser
// Origin (cross-site request forgery / WebSocket hijack), and a small token
// budget per client/route so a runaway tab cannot melt the kernel.
//
// It is deliberately separate from Handler so unit tests can exercise the API
// without faking Host headers; main always installs it.
func LocalOnly(next http.Handler) http.Handler {
	general := newRateLimiter(20, 40) // 20 req/s, burst 40
	chat := newRateLimiter(0.25, 5)   // 15 req/min, burst 5
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowedHost(r.Host) {
			writeJSONError(w, http.StatusMisdirectedRequest, "invalid host")
			return
		}
		if !allowedOrigin(r) {
			writeJSONError(w, http.StatusForbidden, "origin not allowed")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			limiter := general
			if r.URL.Path == "/api/chat" && r.Method == http.MethodPost {
				limiter = chat
			}
			if ok, wait := limiter.allow(clientKey(r) + "|" + r.URL.Path); !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(wait/time.Second)+1))
				writeJSONError(w, http.StatusTooManyRequests, "too many requests")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// allowedHost accepts only the loopback names the server can actually be
// reached from. A Host of evil.example (DNS rebinding) is rejected.
func allowedHost(hostport string) bool {
	host := strings.Trim(strings.ToLower(loopbackHost(hostport)), "[]")
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func loopbackHost(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	if host, _, err := net.SplitHostPort(hostport); err == nil {
		return host
	}
	return hostport
}

// allowedOrigin accepts a missing Origin (curl, native clients) and an Origin
// whose host exactly matches the request Host. A browser page on another
// origin cannot drive the agent.
func allowedOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	return allowedHost(u.Host) && strings.EqualFold(u.Host, r.Host)
}

func clientKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	if r.RemoteAddr == "" {
		return "local"
	}
	return r.RemoteAddr
}
