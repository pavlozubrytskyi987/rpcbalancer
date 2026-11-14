package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestID_generatesWhenMissing(t *testing.T) {
	var gotID string

	h := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = RequestID(r.Context())
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if gotID == "" {
		t.Fatal("expected generated id, got empty")
	}
	if len(gotID) != 16 {
		t.Errorf("id len = %d, want 16", len(gotID))
	}
	if rec.Header().Get(headerRequestID) != gotID {
		t.Errorf("response header = %q, want %q", rec.Header().Get(headerRequestID), gotID)
	}
}

func TestWithRequestID_preservesClientID(t *testing.T) {
	const want = "client-supplied-id"
	var seen string

	h := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestID(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(headerRequestID, want)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != want {
		t.Errorf("ctx id = %q, want %q", seen, want)
	}
	if rec.Header().Get(headerRequestID) != want {
		t.Errorf("echoed header = %q, want %q", rec.Header().Get(headerRequestID), want)
	}
}

func TestAccess_logsStatusAndBytes(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	Access(logger, inner).ServeHTTP(rec, req)

	line := buf.String()
	if !strings.Contains(line, `"status":418`) {
		t.Errorf("log missing status=418: %s", line)
	}
	if !strings.Contains(line, `"bytes":5`) {
		t.Errorf("log missing bytes=5: %s", line)
	}

	// sanity: it is valid JSON
	var tmp map[string]any
	if err := json.Unmarshal(buf.Bytes(), &tmp); err != nil {
		t.Errorf("log line is not json: %v", err)
	}
}

func TestAccess_defaultStatusWhenHandlerOnlyWrites(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("no explicit header"))
	})

	Access(logger, inner).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !strings.Contains(buf.String(), `"status":200`) {
		t.Errorf("expected default status 200: %s", buf.String())
	}
}
