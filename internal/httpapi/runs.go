package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/octieght18/forge/internal/execution"
	"github.com/octieght18/forge/internal/store"
)

type runBody struct {
	RunID             string        `json:"run_id"`
	WorkloadID        string        `json:"workload_id"`
	VersionID         string        `json:"version_id"`
	Owner             store.Owner   `json:"owner"`
	Question          string        `json:"question"`
	DocumentIDs       []string      `json:"document_ids"`
	CreatedAt         time.Time     `json:"created_at"`
	RerunOf           *string       `json:"rerun_of"`
	SubmissionState   string        `json:"submission_state"`
	CancellationState string        `json:"cancellation_state"`
	Execution         executionBody `json:"execution"`
}

type executionBody struct {
	Source         string     `json:"source"`
	State          string     `json:"state"`
	ObservedAt     *time.Time `json:"observed_at"`
	LastKnownState *string    `json:"last_known_state"`
}

type historyBody struct {
	EventID    string     `json:"event_id"`
	Sequence   int64      `json:"sequence"`
	Step       string     `json:"step"`
	Attempt    int        `json:"attempt"`
	EventType  string     `json:"event_type"`
	OccurredAt time.Time  `json:"occurred_at"`
	Failure    *ErrorBody `json:"failure"`
}

type resultBody struct {
	RunID       string   `json:"run_id"`
	Decision    string   `json:"decision"`
	Reason      string   `json:"reason"`
	Outcome     string   `json:"outcome,omitempty"`
	EvidenceIDs []string `json:"evidence_ids"`
}

// serveRuns implements start, read, cancel, history, result, and rerun.
// List, evidence excerpts, and the OpenAPI report stay closed.
func (h *registration) serveRuns(w http.ResponseWriter, r *http.Request) bool {
	kind, id, ok := runRoute(r.URL.Path)
	if !ok || r.URL.Path != r.URL.EscapedPath() {
		return false
	}
	if kind == "unknown" {
		WriteError(w, r, http.StatusNotFound, "not_found", "Resource not found")
		return true
	}
	allow := "GET"
	method := http.MethodGet
	if kind == "collection" || kind == "cancel" || kind == "rerun" {
		allow = "POST"
		method = http.MethodPost
	}
	if r.Method != method {
		w.Header().Set("Allow", allow)
		WriteError(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed")
		return true
	}
	body, err := h.readBody(w, r)
	if err != nil {
		return true
	}
	principal, err := h.Auth.Authenticate(r.Context(), r)
	if err != nil {
		if r.Context().Err() != nil {
			err = r.Context().Err()
		}
		h.fail(w, r, err)
		return true
	}
	if !h.allow(w, r, principal) {
		return true
	}
	if id != "" {
		encoded, _ := json.Marshal(id)
		if h.validator.Validate("ID", encoded) != nil {
			WriteError(w, r, http.StatusBadRequest, "invalid_request", "Invalid resource identifier")
			return true
		}
	}
	switch kind {
	case "collection":
		h.startRun(w, r, principal, body)
	case "run":
		h.readRun(w, r, principal, id)
	case "cancel":
		h.cancelRun(w, r, principal, id, body)
	case "history":
		h.readHistory(w, r, principal, id)
	case "result":
		h.readResult(w, r, principal, id)
	case "rerun":
		h.rerun(w, r, principal, id, body)
	}
	return true
}

func runRoute(path string) (kind, id string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "runs" {
		return "", "", false
	}
	switch len(parts) {
	case 3:
		return "collection", "", true
	case 4:
		if parts[3] == "" {
			return "", "", false
		}
		return "run", parts[3], true
	case 5:
		if parts[3] == "" {
			return "", "", false
		}
		switch parts[4] {
		case "cancel", "history", "result", "rerun":
			return parts[4], parts[3], true
		default:
			return "unknown", parts[3], true
		}
	default:
		return "unknown", "", true
	}
}

func (h *registration) startRun(w http.ResponseWriter, r *http.Request, principal store.Principal, body []byte) {
	if r.URL.RawQuery != "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "Unexpected query parameters")
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || keys[0] == "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "Idempotency-Key is required")
		return
	}
	if err := h.validateBody(w, r, "CreateRunRequest", body); err != nil {
		return
	}
	run, err := h.Repository.SubmitRun(r.Context(), principal, keys[0], body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	view, err := h.runView(r, principal, run)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/runs/"+run.ID)
	writeJSON(w, http.StatusAccepted, view)
}

func (h *registration) readRun(w http.ResponseWriter, r *http.Request, principal store.Principal, id string) {
	if r.URL.RawQuery != "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "Unexpected query parameters")
		return
	}
	run, err := h.Repository.GetRun(r.Context(), principal, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	view, err := h.runView(r, principal, run)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (h *registration) cancelRun(w http.ResponseWriter, r *http.Request, principal store.Principal, id string, body []byte) {
	if r.URL.RawQuery != "" || len(strings.TrimSpace(string(body))) != 0 {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "Cancel accepts no body")
		return
	}
	command, err := h.Repository.Cancel(r.Context(), principal, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	state := "requested"
	if command.State == "delivered" {
		state = "delivered"
	}
	writeJSON(w, http.StatusAccepted, struct {
		RunID             string `json:"run_id"`
		CancellationState string `json:"cancellation_state"`
	}{id, state})
}

func (h *registration) rerun(w http.ResponseWriter, r *http.Request, principal store.Principal, id string, body []byte) {
	if r.URL.RawQuery != "" || len(strings.TrimSpace(string(body))) != 0 {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "Rerun accepts no body")
		return
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || keys[0] == "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "Idempotency-Key is required")
		return
	}
	run, err := h.Repository.Rerun(r.Context(), principal, id, keys[0], h.confirmedTerminal)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	view, err := h.runView(r, principal, run)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/runs/"+run.ID)
	writeJSON(w, http.StatusAccepted, view)
}

func (h *registration) confirmedTerminal(ctx context.Context, run store.Run) error {
	if h.Execution == nil {
		return store.ErrUnavailable
	}
	view, err := h.Execution.Result(ctx, run.WorkflowID)
	if errors.Is(err, execution.ErrNotReady) || errors.Is(err, execution.ErrAbsent) {
		return store.ErrConflict
	}
	if err != nil {
		return store.ErrUnavailable
	}
	switch view.Decision {
	case "complete", "fail_closed", "canceled", "timed_out":
		return nil
	default:
		return store.ErrConflict
	}
}

func (h *registration) readHistory(w http.ResponseWriter, r *http.Request, principal store.Principal, id string) {
	limit, encoded, ok := h.query(w, r)
	if !ok {
		return
	}
	if _, err := h.Repository.GetRun(r.Context(), principal, id); err != nil {
		h.fail(w, r, err)
		return
	}
	if h.Execution == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "Execution history is unavailable")
		return
	}
	var after int64
	if encoded != "" {
		position, err := h.cursors.decode(principal, "history", id, encoded)
		if err != nil || position.Number < 1 {
			WriteError(w, r, http.StatusBadRequest, "invalid_request", "Invalid cursor")
			return
		}
		after = position.Number
	}
	events, err := h.Execution.History(r.Context(), "forge-run/"+id)
	if err != nil {
		if errors.Is(err, execution.ErrAbsent) {
			writeJSON(w, http.StatusOK, struct {
				Items []historyBody `json:"items"`
				Next  *string       `json:"next_cursor"`
			}{[]historyBody{}, nil})
			return
		}
		WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "Execution history is unavailable")
		return
	}
	page, more := pageHistory(events, after, limit)
	items := make([]historyBody, 0, len(page))
	for _, event := range page {
		item := historyBody{
			EventID: event.EventID, Sequence: event.Sequence, Step: event.Step, Attempt: event.Attempt,
			EventType: event.EventType, OccurredAt: event.OccurredAt.UTC(),
		}
		item.Failure = publicFailure(event.Failure)
		items = append(items, item)
	}
	var next *string
	if more {
		last := page[len(page)-1]
		token := h.cursors.encode(principal, cursor{Collection: "history", Parent: id, Number: last.Sequence})
		next = &token
	}
	writeJSON(w, http.StatusOK, struct {
		Items []historyBody `json:"items"`
		Next  *string       `json:"next_cursor"`
	}{items, next})
}

func pageHistory(events []execution.Event, after int64, limit int) ([]execution.Event, bool) {
	start := 0
	if after > 0 {
		start = len(events)
		for i, event := range events {
			if event.Sequence > after {
				start = i
				break
			}
		}
	}
	rest := events[start:]
	if len(rest) > limit {
		return rest[:limit], true
	}
	return rest, false
}

func (h *registration) readResult(w http.ResponseWriter, r *http.Request, principal store.Principal, id string) {
	if r.URL.RawQuery != "" {
		WriteError(w, r, http.StatusBadRequest, "invalid_request", "Unexpected query parameters")
		return
	}
	run, err := h.Repository.GetRun(r.Context(), principal, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if h.Execution == nil {
		WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "Execution result is unavailable")
		return
	}
	view, err := h.Execution.Result(r.Context(), run.WorkflowID)
	if err != nil {
		if errors.Is(err, execution.ErrNotReady) || errors.Is(err, execution.ErrAbsent) {
			WriteError(w, r, http.StatusConflict, "conflict", "Result is not available")
			return
		}
		WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "Execution result is unavailable")
		return
	}
	if view.RunID != "" && view.RunID != run.ID {
		WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "Execution result is unavailable")
		return
	}
	if !safeCode.MatchString(view.Decision) || !safeCode.MatchString(view.Reason) || secretish(view.Reason) {
		WriteError(w, r, http.StatusServiceUnavailable, "unavailable", "Execution result is unavailable")
		return
	}
	outcome := ""
	switch view.Outcome {
	case "answered", "insufficient_evidence", "conflicting_evidence":
		outcome = view.Outcome
	}
	writeJSON(w, http.StatusOK, resultBody{RunID: run.ID, Decision: view.Decision, Reason: view.Reason, Outcome: outcome, EvidenceIDs: execution.EvidenceIDs(view.EvidenceIDs)})
}

var safeCode = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func publicFailure(failure *execution.SafeFailure) *ErrorBody {
	if failure == nil {
		return nil
	}
	if !safeCode.MatchString(failure.Code) || failure.Message == "" || len(failure.Message) > 512 || secretish(failure.Message) {
		return &ErrorBody{Code: "failed", Message: "The activity failed"}
	}
	return &ErrorBody{Code: failure.Code, Message: failure.Message}
}

func secretish(value string) bool {
	lower := strings.ToLower(value)
	return strings.Contains(lower, "bearer") || strings.Contains(lower, "password") || strings.Contains(lower, "postgres") || strings.Contains(lower, "://")
}

func (h *registration) runView(r *http.Request, principal store.Principal, run store.Run) (runBody, error) {
	var input struct {
		Question    string   `json:"question"`
		DocumentIDs []string `json:"document_ids"`
	}
	if json.Unmarshal(run.Input, &input) != nil || input.Question == "" || len(input.DocumentIDs) == 0 {
		return runBody{}, store.ErrUnavailable
	}
	commands, err := h.Repository.Commands(r.Context(), principal, run.ID)
	if err != nil {
		return runBody{}, err
	}
	submission := "pending"
	cancellation := "not_requested"
	for _, command := range commands {
		if command.Kind == "start" && command.State == "delivered" {
			submission = "dispatched"
		}
		if command.Kind == "cancel" {
			cancellation = "requested"
			if command.State == "delivered" {
				cancellation = "delivered"
			}
		}
	}
	observed := executionBody{Source: "unavailable", State: "unknown"}
	if h.Execution != nil {
		status, statusErr := h.Execution.Status(r.Context(), run.WorkflowID)
		switch {
		case statusErr == nil && knownExecutionState(status.State) && !status.ObservedAt.IsZero():
			at := status.ObservedAt.UTC()
			observed = executionBody{Source: "temporal", State: status.State, ObservedAt: &at}
		case errors.Is(statusErr, execution.ErrAbsent) && submission == "pending":
			observed = executionBody{Source: "not_started", State: "not_started"}
		}
	}
	if submission == "dispatched" && observed.Source == "not_started" {
		observed = executionBody{Source: "unavailable", State: "unknown"}
	}
	return runBody{
		RunID: run.ID, WorkloadID: run.WorkloadID, VersionID: run.VersionID, Owner: run.Owner,
		Question: input.Question, DocumentIDs: input.DocumentIDs, CreatedAt: run.CreatedAt.UTC(),
		RerunOf: run.RerunOf, SubmissionState: submission, CancellationState: cancellation, Execution: observed,
	}, nil
}

func knownExecutionState(state string) bool {
	switch state {
	case "running", "succeeded", "failed", "canceled", "timed_out", "terminated":
		return true
	default:
		return false
	}
}
