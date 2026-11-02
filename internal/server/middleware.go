package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"
)

const (
	// maxAPIRequestBody bounds JSON/file payloads before a handler can allocate
	// an unbounded body. Large exports are capped at the same size by the
	// export handler; Workbench writes are capped at 1 MiB deeper in.
	maxAPIRequestBody = 8 << 20
	requestIDHeader   = "X-Request-ID"
)

type requestIDKey struct{}

var requestSeq atomic.Uint64

// responseRecorder captures the status and size for access logs while keeping
// the optional interfaces a WebSocket upgrade and streaming response need.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.status != 0 {
		return
	}
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += n
	return n, err
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return h.Hijack()
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *responseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// withMiddleware wraps the mux with request IDs, security headers, a body
// ceiling, panic recovery, counters and one access-log line per request.
func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := sanitizeRequestID(r.Header.Get(requestIDHeader))
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set(requestIDHeader, id)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}

		rec := &responseRecorder{ResponseWriter: w}
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
		panicked := false

		defer func() {
			if p := recover(); p != nil {
				panicked = true
				log.Printf("[http] panic method=%s path=%s id=%s panic=%v\n%s", r.Method, r.URL.Path, id, p, debug.Stack())
				if rec.status == 0 {
					writeJSONError(rec, http.StatusInternalServerError, "internal server error")
				}
			}
			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}
			if status >= 500 || panicked {
				s.errors.Add(1)
			}
			s.requests.Add(1)
			elapsed := time.Since(start)
			s.latency.observe(elapsed)
			log.Printf("[http] %s %s %d %dB %dms id=%s", r.Method, r.URL.Path, status, rec.bytes, elapsed.Milliseconds(), id)
		}()

		if r.Method != http.MethodGet && r.Method != http.MethodHead && strings.HasPrefix(r.URL.Path, "/api/") {
			if r.ContentLength > maxAPIRequestBody {
				writeJSONError(rec, http.StatusRequestEntityTooLarge, "request body too large")
				return
			}
			r.Body = http.MaxBytesReader(rec, r.Body, maxAPIRequestBody)
		}

		next.ServeHTTP(rec, r)
	})
}

func newRequestID() string {
	return fmt.Sprintf("%x-%x", time.Now().UnixNano(), requestSeq.Add(1))
}

func sanitizeRequestID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 {
		return ""
	}
	for _, r := range id {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.' {
			continue
		}
		return ""
	}
	return id
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
