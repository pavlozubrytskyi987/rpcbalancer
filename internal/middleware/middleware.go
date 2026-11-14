// Package middleware holds small http.Handler wrappers.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"
)

type ctxKey struct{}

var requestIDKey ctxKey

const headerRequestID = "X-Request-ID"

// WithRequestID reads X-Request-ID if the client sent one, else generates
// 16 hex chars from crypto/rand. Puts it on the context and echoes it back
// on the response so a client can correlate logs across hops.
func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(headerRequestID)
		if id == "" {
			id = newID()
		}

		w.Header().Set(headerRequestID, id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestID pulls the id back off the context. Empty string if not set.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand never fails on linux/darwin
	return hex.EncodeToString(b[:])
}

// Access logs one line per request: method, path, status, size, duration, id.
func Access(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}

		next.ServeHTTP(sw, r)

		logger.Info("req",
			"id", RequestID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.statusOrDefault(),
			"bytes", sw.bytes,
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}

// statusWriter captures the status and byte count the inner handler wrote.
// Not safe for Hijack or Flush, but we don't need those for JSON-RPC.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	w.bytes += int64(n)
	return n, err
}

func (w *statusWriter) statusOrDefault() int {
	if w.status == 0 {
		return http.StatusOK // handler wrote body without calling WriteHeader
	}
	return w.status
}
