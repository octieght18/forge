package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthAndErrorContracts(t *testing.T) {
	var logs bytes.Buffer
	ready := &Readiness{}
	h := NewHandler(slog.New(slog.NewJSONHandler(&logs, nil)), ready)
	for _, tc := range []struct {
		method, path string
		ready        bool
		status       int
		code         string
	}{
		{"GET", "/healthz", false, 200, ""},
		{"GET", "/readyz", false, 503, "not_ready"},
		{"GET", "/readyz", true, 200, ""},
		{"GET", "/unknown?token=secret-query", true, 404, "not_found"},
		{"POST", "/healthz", true, 405, "method_not_allowed"},
	} {
		ready.Set(tc.ready)
		r := httptest.NewRequest(tc.method, tc.path, nil)
		r.Header.Set("Authorization", "Bearer secret-token")
		r.Header.Set("X-Request-ID", "untrusted-id")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s %s: status %d", tc.method, tc.path, w.Code)
		}
		id := w.Header().Get("X-Request-ID")
		if len(id) != 32 || id == "untrusted-id" {
			t.Fatalf("invalid server request ID %q", id)
		}
		if w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("missing response headers")
		}
		if tc.code != "" {
			var body ErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Error.Code != tc.code || body.RequestID != id {
				t.Fatalf("unexpected error: %+v", body)
			}
		}
		if tc.status == 405 && w.Header().Get("Allow") != "GET, HEAD" {
			t.Fatal("missing Allow header")
		}
	}
	if strings.Contains(logs.String(), "secret-") || strings.Contains(logs.String(), "untrusted-id") {
		t.Fatal("sensitive request data logged")
	}
}

func TestPanicBeforeWriteReturnsSafeError(t *testing.T) {
	var logs bytes.Buffer
	h := Middleware(slog.New(slog.NewJSONHandler(&logs, nil)), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret-panic") }))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 || !strings.Contains(w.Body.String(), "internal_error") {
		t.Fatalf("unexpected response %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String()+logs.String(), "secret-panic") {
		t.Fatal("panic contents leaked")
	}
}

func TestPanicAfterWriteAbortsResponse(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	h := Middleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		panic("secret")
	}))
	defer func() {
		if recover() != http.ErrAbortHandler {
			t.Fatal("partial response should abort")
		}
	}()
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))
}
