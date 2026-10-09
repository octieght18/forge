package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/provision"
	"github.com/octieght18/forge/internal/store"
)

type operationBody struct {
	OperationID        string     `json:"operation_id"`
	WorkloadID         string     `json:"workload_id"`
	VersionID          string     `json:"version_id"`
	Action             string     `json:"action"`
	Status             string     `json:"status"`
	StatusURL          string     `json:"status_url"`
	DesiredGeneration  int64      `json:"desired_generation"`
	ObservedGeneration int64      `json:"observed_generation"`
	ObservedPhase      string     `json:"observed_phase,omitempty"`
	Deadline           time.Time  `json:"deadline"`
	Error              *ErrorBody `json:"error,omitempty"`
}

func operationView(op provision.Operation) operationBody {
	view := operationBody{
		OperationID: op.ID, WorkloadID: op.WorkloadID, VersionID: op.VersionID, Action: op.Action,
		Status: string(op.Status), StatusURL: provision.StatusPath(op.ID), DesiredGeneration: op.DesiredGeneration,
		ObservedGeneration: op.ObservedGeneration, ObservedPhase: op.ObservedPhase, Deadline: op.Deadline.UTC(),
	}
	if op.ErrorCode != "" {
		view.Error = &ErrorBody{Code: op.ErrorCode, Message: op.ErrorMessage}
	}
	return view
}

// serveOperation handles asynchronous provisioning routes. False means the
// request belongs to another route.
func (h *registration) serveOperation(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Path != r.URL.EscapedPath() || r.URL.RawQuery != "" {
		return false
	}
	kind, workloadID, operationID := operationRoute(r.URL.Path)
	if kind == "" {
		return false
	}
	allow := "POST"
	if kind == "get" {
		allow = "GET"
	}
	method := "POST"
	if kind == "get" {
		method = "GET"
	}
	if r.Method != method {
		w.Header().Set("Allow", allow)
		WriteError(w, r, 405, "method_not_allowed", "Method not allowed")
		return true
	}
	body, err := h.readBody(w, r)
	if err != nil {
		return true
	}
	if kind != "get" {
		if err = h.operationMedia(w, r); err != nil {
			return true
		}
	}
	principal, err := h.Auth.Authenticate(r.Context(), r)
	if err != nil {
		h.fail(w, r, err)
		return true
	}
	now := time.Now().UTC()
	var op provision.Operation
	switch kind {
	case "create":
		op, err = h.acceptOperation(r, principal, workloadID, body, now)
	case "get":
		op, err = h.Repository.GetOperation(r.Context(), principal, operationID, now)
	default:
		op, err = h.Repository.CancelOperation(r.Context(), principal, operationID, now)
	}
	if err != nil {
		h.operationError(w, r, err)
		return true
	}
	view := operationView(op)
	if kind != "get" {
		w.Header().Set("Location", view.StatusURL)
		writeJSON(w, http.StatusAccepted, view)
		return true
	}
	writeJSON(w, http.StatusOK, view)
	return true
}

func operationRoute(path string) (kind, workloadID, operationID string) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "workloads" && parts[4] == "operations" && parts[3] != "":
		return "create", parts[3], ""
	case len(parts) == 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "operations" && parts[3] != "":
		return "get", "", parts[3]
	case len(parts) == 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "operations" && parts[4] == "cancel" && parts[3] != "":
		return "cancel", "", parts[3]
	default:
		return "", "", ""
	}
}

func (h *registration) operationMedia(w http.ResponseWriter, r *http.Request) error {
	content := r.Header.Values("Content-Type")
	media := ""
	var params map[string]string
	var err error
	if len(content) == 1 {
		media, params, err = mime.ParseMediaType(content[0])
	}
	if len(content) != 1 || err != nil || media != "application/json" || len(params) > 1 || len(params) == 1 && !strings.EqualFold(params["charset"], "utf-8") {
		WriteError(w, r, 415, "unsupported_media_type", "Use application/json with UTF-8")
		return errors.New("content type")
	}
	return nil
}

func (h *registration) acceptOperation(r *http.Request, principal store.Principal, workloadID string, body []byte, now time.Time) (provision.Operation, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return provision.Operation{}, contract.ErrInvalidJSON
	}
	var input struct {
		Action         string `json:"action"`
		VersionID      string `json:"version_id"`
		TimeoutSeconds *int   `json:"timeout_seconds"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		return provision.Operation{}, contract.ErrInvalidJSON
	}
	if dec.More() {
		return provision.Operation{}, contract.ErrInvalidJSON
	}
	timeout := provision.DefaultTimeout
	if input.TimeoutSeconds != nil {
		if *input.TimeoutSeconds < 1 || *input.TimeoutSeconds > int(provision.MaxTimeout/time.Second) {
			return provision.Operation{}, provision.ErrAction
		}
		timeout = time.Duration(*input.TimeoutSeconds) * time.Second
	}
	return h.Repository.AcceptOperation(r.Context(), principal, workloadID, input.VersionID, input.Action, timeout, now)
}

func (h *registration) operationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, provision.ErrConflict):
		WriteError(w, r, 409, "conflict", "An operation for this workload is already open.")
	case errors.Is(err, provision.ErrVersion):
		WriteError(w, r, 409, "conflict", "The version does not match the workload's current version.")
	case errors.Is(err, provision.ErrTerminal):
		WriteError(w, r, 409, "conflict", "A finished operation cannot be canceled.")
	case errors.Is(err, provision.ErrAction):
		WriteError(w, r, 422, "invalid_input", "Action or timeout is not supported.")
	default:
		h.fail(w, r, err)
	}
}
