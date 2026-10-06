package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/store"
)

type Registration struct {
	Repository *store.Repository
	Auth       *auth.Verifier
	Pool       *pgxpool.Pool
	Policy     *CorpusPolicy
	CursorKey  []byte
	Timeout    time.Duration
}
type registration struct {
	Registration
	validator *contract.Validator
	cursors   cursors
	ready     *Readiness
}

func NewRegistrationHandler(logger *slog.Logger, ready *Readiness, c Registration) (http.Handler, error) {
	if logger == nil || ready == nil || c.Repository == nil || c.Auth == nil || c.Pool == nil || c.Policy == nil || len(c.CursorKey) < 32 || c.Timeout <= 0 {
		return nil, errors.New("registration dependencies/configuration missing")
	}
	v, err := contract.New()
	if err != nil {
		return nil, err
	}
	h := &registration{c, v, cursors{append([]byte(nil), c.CursorKey...), time.Now}, ready}
	return Middleware(logger, http.HandlerFunc(h.serve)), nil
}

func (h *registration) fail(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrUnauthenticated):
		w.Header().Set("WWW-Authenticate", "Bearer")
		WriteError(w, r, 401, "unauthenticated", "Valid access token required")
	case errors.Is(err, store.ErrForbidden):
		WriteError(w, r, 403, "forbidden", "Role is not permitted")
	case errors.Is(err, store.ErrNotFound):
		WriteError(w, r, 404, "not_found", "Resource not found")
	case errors.Is(err, store.ErrConflict):
		WriteError(w, r, 409, "conflict", "Resource conflicts with existing state")
	case errors.Is(err, store.ErrPrecondition):
		WriteError(w, r, 412, "precondition_failed", "Revision does not match")
	case errors.Is(err, contract.ErrBodyTooLarge):
		WriteError(w, r, 413, "request_too_large", "Request body exceeds 64 KiB")
	case errors.Is(err, contract.ErrInvalidJSON):
		WriteError(w, r, 400, "invalid_request", "Invalid JSON document")
	case errors.Is(err, contract.ErrInvalidInput):
		WriteError(w, r, 422, "invalid_input", "Input does not match the supported policy")
	default:
		WriteError(w, r, 503, "unavailable", "Service dependency unavailable")
	}
}

func (h *registration) serve(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.Timeout)
	defer cancel()
	r = r.WithContext(ctx)
	if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
		if r.Method != "GET" && r.Method != "HEAD" {
			w.Header().Set("Allow", "GET, HEAD")
			WriteError(w, r, 405, "method_not_allowed", "Method not allowed")
			return
		}
		if r.URL.Path == "/readyz" && (!h.ready.Ready() || store.CheckSchema(ctx, h.Pool) != nil) {
			WriteError(w, r, 503, "not_ready", "Service is not ready")
			return
		}
		writeJSON(w, 200, struct {
			Status string `json:"status"`
		}{"ok"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
	if r.URL.Path != r.URL.EscapedPath() || len(parts) < 3 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "workloads" || len(parts) > 6 || len(parts) > 4 && parts[4] != "versions" || len(parts) == 4 && parts[3] == "" || len(parts) == 6 && parts[5] == "" {
		WriteError(w, r, 404, "not_found", "Resource not found")
		return
	}
	methods := "GET, POST"
	if len(parts) == 4 {
		methods = "GET, PATCH"
	}
	if len(parts) == 6 {
		methods = "GET"
	}
	if !strings.Contains(","+strings.ReplaceAll(methods, ", ", ",")+",", ","+r.Method+",") {
		w.Header().Set("Allow", methods)
		WriteError(w, r, 405, "method_not_allowed", "Method not allowed")
		return
	}
	p, err := h.Auth.Authenticate(ctx, r)
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		h.fail(w, r, err)
		return
	}
	for _, i := range []int{3, 5} {
		if len(parts) > i {
			encoded := []byte(strconv.Quote(parts[i]))
			if h.validator.Validate("ID", encoded) != nil {
				WriteError(w, r, 400, "invalid_request", "Invalid resource identifier")
				return
			}
		}
	}
	if len(parts) == 3 && r.Method == "GET" {
		h.listWorkloads(w, r, p)
		return
	}
	if len(parts) == 5 && r.Method == "GET" {
		h.listVersions(w, r, p, parts[3])
		return
	}
	if r.URL.RawQuery != "" {
		WriteError(w, r, 400, "invalid_request", "Unexpected query parameters")
		return
	}
	if r.Method == "GET" {
		if len(parts) == 4 {
			value, err := h.Repository.GetWorkload(ctx, p, parts[3])
			if err != nil {
				h.fail(w, r, err)
				return
			}
			w.Header().Set("ETag", etag(value.ID, value.Revision))
			writeJSON(w, 200, value)
		} else {
			value, err := h.Repository.GetVersion(ctx, p, parts[3], parts[5])
			if err != nil {
				h.fail(w, r, err)
				return
			}
			writeJSON(w, 200, value)
		}
		return
	}
	schema := "CreateWorkloadRequest"
	if len(parts) == 5 {
		schema = "CreateVersionRequest"
	}
	if r.Method == "PATCH" {
		schema = "UpdateWorkloadRequest"
	}
	body, err := h.body(w, r, schema)
	if err != nil {
		return
	}
	if r.Method == "PATCH" {
		// Authorize before exposing revision/header preconditions.
		if _, err := h.Repository.GetWorkload(ctx, store.Principal{Issuer: p.Issuer, Subject: p.Subject, Role: "developer"}, parts[3]); err != nil {
			h.fail(w, r, err)
			return
		}
		values := r.Header.Values("If-Match")
		if len(values) == 0 {
			WriteError(w, r, 428, "precondition_required", "If-Match is required")
			return
		}
		revision, err := parseETag(parts[3], values)
		if err != nil {
			WriteError(w, r, 400, "invalid_request", "Invalid If-Match")
			return
		}
		value, err := h.Repository.UpdateWorkload(ctx, p, parts[3], revision, body)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		w.Header().Set("ETag", etag(value.ID, value.Revision))
		writeJSON(w, 200, value)
		return
	}
	if len(parts) == 3 {
		value, err := h.Repository.CreateWorkload(ctx, p, body)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		w.Header().Set("Location", "/api/v1/workloads/"+value.ID)
		w.Header().Set("ETag", etag(value.ID, value.Revision))
		writeJSON(w, 201, value)
		return
	}
	if _, err := h.Repository.GetWorkload(ctx, store.Principal{Issuer: p.Issuer, Subject: p.Subject, Role: "developer"}, parts[3]); err != nil {
		h.fail(w, r, err)
		return
	}
	if !h.Policy.approve(body) {
		h.fail(w, r, contract.ErrInvalidInput)
		return
	}
	value, err := h.Repository.CreateVersion(ctx, p, parts[3], body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/api/v1/workloads/%s/versions/%s", parts[3], value.ID))
	writeJSON(w, 201, value)
}
func etag(id string, revision int64) string {
	return strconv.Quote(id + ":" + strconv.FormatInt(revision, 10))
}
func parseETag(id string, values []string) (int64, error) {
	if len(values) != 1 {
		return 0, errors.New("invalid ETag")
	}
	v := values[0]
	if len(v) < 3 || v[0] != '"' || v[len(v)-1] != '"' {
		return 0, errors.New("invalid ETag")
	}
	if !strings.HasPrefix(v, "\""+id+":") {
		return 0, errors.New("invalid ETag")
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(v[:len(v)-1], "\""+id+":"), 10, 64)
	if err != nil || n < 1 || etag(id, n) != v {
		return 0, errors.New("invalid ETag")
	}
	return n, nil
}
func (h *registration) body(w http.ResponseWriter, r *http.Request, schema string) ([]byte, error) {
	content := r.Header.Values("Content-Type")
	media := ""
	var params map[string]string
	var err error
	if len(content) == 1 {
		media, params, err = mime.ParseMediaType(content[0])
	}
	if len(content) != 1 || err != nil || media != "application/json" || len(params) > 1 || len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8") {
		WriteError(w, r, 415, "unsupported_media_type", "Use application/json with UTF-8")
		return nil, errors.New("content type")
	}
	deadline, _ := r.Context().Deadline()
	controller := http.NewResponseController(w)
	_ = controller.SetReadDeadline(deadline)
	defer controller.SetReadDeadline(time.Time{})
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, contract.MaxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		var timeout net.Error
		if errors.As(err, &tooLarge) {
			err = contract.ErrBodyTooLarge
		} else if r.Context().Err() != nil || errors.As(err, &timeout) && timeout.Timeout() {
			err = store.ErrUnavailable
		} else {
			err = contract.ErrInvalidJSON
		}
		h.fail(w, r, err)
		return nil, err
	}
	if err = h.validator.ValidateRequest(schema, body); err != nil {
		h.fail(w, r, err)
		return nil, err
	}
	return body, nil
}
func (h *registration) query(w http.ResponseWriter, r *http.Request) (int, string, bool) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		WriteError(w, r, 400, "invalid_request", "Invalid query")
		return 0, "", false
	}
	for key, values := range q {
		if (key != "limit" && key != "cursor") || len(values) != 1 || values[0] == "" {
			WriteError(w, r, 400, "invalid_request", "Invalid query parameters")
			return 0, "", false
		}
	}
	limit := 20
	if s := q.Get("limit"); s != "" {
		limit, err = strconv.Atoi(s)
		if err != nil || limit < 1 || limit > 100 || strconv.Itoa(limit) != s {
			WriteError(w, r, 400, "invalid_request", "Invalid page limit")
			return 0, "", false
		}
	}
	return limit, q.Get("cursor"), true
}
func (h *registration) listWorkloads(w http.ResponseWriter, r *http.Request, p store.Principal) {
	limit, encoded, ok := h.query(w, r)
	if !ok {
		return
	}
	var after *store.Position
	if encoded != "" {
		c, err := h.cursors.decode(p, "workloads", "", encoded)
		if err != nil {
			WriteError(w, r, 400, "invalid_request", "Invalid cursor")
			return
		}
		after = &store.Position{CreatedAt: c.At, ID: c.ID}
	}
	values, err := h.Repository.ListWorkloads(r.Context(), p, limit, after)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var next *string
	if len(values) == limit {
		last := values[len(values)-1]
		position := store.Position{CreatedAt: last.CreatedAt, ID: last.ID}
		extra, err := h.Repository.ListWorkloads(r.Context(), p, 1, &position)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		if len(extra) > 0 {
			token := h.cursors.encode(p, cursor{Collection: "workloads", ID: last.ID, At: last.CreatedAt})
			next = &token
		}
	}
	writeJSON(w, 200, struct {
		Items []store.Workload `json:"items"`
		Next  *string          `json:"next_cursor"`
	}{values, next})
}
func (h *registration) listVersions(w http.ResponseWriter, r *http.Request, p store.Principal, parent string) {
	limit, encoded, ok := h.query(w, r)
	if !ok {
		return
	}
	var before int64
	if encoded != "" {
		c, err := h.cursors.decode(p, "versions", parent, encoded)
		if err != nil || c.Number < 1 {
			WriteError(w, r, 400, "invalid_request", "Invalid cursor")
			return
		}
		before = c.Number
	}
	values, err := h.Repository.ListVersions(r.Context(), p, parent, limit, before)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var next *string
	if len(values) == limit {
		last := values[len(values)-1]
		extra, err := h.Repository.ListVersions(r.Context(), p, parent, 1, last.Number)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		if len(extra) > 0 {
			token := h.cursors.encode(p, cursor{Collection: "versions", Parent: parent, Number: last.Number})
			next = &token
		}
	}
	writeJSON(w, 200, struct {
		Items []store.Version `json:"items"`
		Next  *string         `json:"next_cursor"`
	}{values, next})
}
