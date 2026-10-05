package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/octieght18/forge/internal/contract"
)

// This checks the running foundation against the independently compiled API
// schemas and prevents the contract ticket from accidentally enabling an
// unauthenticated product endpoint before the implementation/auth tickets.
func TestFoundationMatchesPublishedContract(t *testing.T) {
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	ready := &Readiness{}
	h := NewHandler(logger, ready)
	for _, tc := range []struct {
		method, path, schema string
		ready                bool
		status               int
	}{
		{"GET", "/healthz", "HealthResponse", false, 200},
		{"GET", "/readyz", "ErrorResponse", false, 503},
		{"GET", "/readyz", "HealthResponse", true, 200},
		{"POST", "/healthz", "ErrorResponse", true, 405},
		{"POST", "/api/v1/workloads", "ErrorResponse", true, 404},
		{"POST", "/api/v1/runs", "ErrorResponse", true, 404},
	} {
		ready.Set(tc.ready)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s %s returned %d", tc.method, tc.path, w.Code)
		}
		if err := v.Validate(tc.schema, w.Body.Bytes()); err != nil {
			t.Fatalf("%s %s violates contract: %v", tc.method, tc.path, err)
		}
		if w.Header().Get("Cache-Control") != "no-store" || len(w.Header().Get("X-Request-ID")) != 32 {
			t.Fatal("missing contract response headers")
		}
	}
	panicHandler := Middleware(logger, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("sensitive details") }))
	w := httptest.NewRecorder()
	panicHandler.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 500 || v.Validate("ErrorResponse", w.Body.Bytes()) != nil {
		t.Fatal("panic response violates error contract")
	}
}
