// Package httpapi owns HTTP transport behavior, not application execution or storage.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
)

type requestIDKey struct{}

func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Readiness is process readiness today. Dependency checks belong in later integrations.
type Readiness struct{ ready atomic.Bool }

func (r *Readiness) Set(ready bool) { r.ready.Store(ready) }
func (r *Readiness) Ready() bool    { return r.ready.Load() }

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id"`
}

// WriteError publishes an explicit safe message; internal errors stay out of responses.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	writeJSON(w, status, ErrorResponse{ErrorBody{code, message}, RequestID(r.Context())})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func NewHandler(logger *slog.Logger, readiness *Readiness) http.Handler {
	return Middleware(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" && r.URL.Path != "/readyz" {
			WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
			return
		}
		if r.URL.Path == "/readyz" && !readiness.Ready() {
			WriteError(w, r, http.StatusServiceUnavailable, "not_ready", "Service is not ready")
			return
		}
		writeJSON(w, http.StatusOK, struct {
			Status string `json:"status"`
		}{"ok"})
	}))
}

// Middleware provides correlation and a safe panic boundary. Do not log tokens,
// query strings, request bodies, or arbitrary panic values.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			logger.Error("request ID generation failed")
			writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: ErrorBody{"unavailable", "Service unavailable"}})
			return
		}
		id := hex.EncodeToString(random[:])
		w.Header().Set("X-Request-ID", id)
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		r = r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id))
		observed := &responseWriter{ResponseWriter: w}
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("HTTP handler panic", "request_id", id)
				if observed.status != 0 {
					// A partial response cannot be replaced by a JSON error. Abort the connection.
					panic(http.ErrAbortHandler)
				}
				WriteError(observed, r, http.StatusInternalServerError, "internal_error", "Internal server error")
			}
			status := observed.status
			if status == 0 {
				status = http.StatusOK
			}
			logger.Info("HTTP request", "request_id", id, "status", status)
		}()
		next.ServeHTTP(observed, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(p)
}

// ResponseController can reach the underlying writer through Unwrap.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseWriter) ReadFrom(r io.Reader) (int64, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return io.Copy(w.ResponseWriter, r)
}
