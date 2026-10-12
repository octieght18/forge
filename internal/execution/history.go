package execution

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

const maxHistoryEvents = 4096

// ErrNotReady means the workflow has no terminal result yet.
var ErrNotReady = errors.New("execution result is not ready")

// ErrHistoryUnavailable means Temporal did not provide a usable history or result.
var ErrHistoryUnavailable = errors.New("execution history unavailable")

// LiveState is one Temporal state reading. It is not stored in PostgreSQL.
type LiveState struct {
	State      string
	ObservedAt time.Time
}

// Event is one sanitized history row. It has no prompt, corpus text, or token.
type Event struct {
	EventID    string
	Sequence   int64
	Step       string
	Attempt    int
	EventType  string
	OccurredAt time.Time
	Failure    *SafeFailure
}

// SafeFailure is the public error shape for one history event.
type SafeFailure struct {
	Code    string
	Message string
}

// ResultView is the terminal workflow value a caller may read.
// Evidence IDs are identifiers. Prompts and excerpts are not included.
type ResultView struct {
	RunID       string
	Decision    string
	Reason      string
	Outcome     string
	EvidenceIDs []string
}

// Reader loads execution state from Temporal. A nil reader means the API
// process has no Temporal client, which is not the same as an absent workflow.
type Reader interface {
	Status(context.Context, string) (LiveState, error)
	History(context.Context, string) ([]Event, error)
	Result(context.Context, string) (ResultView, error)
}

// SDKReader reads the workflow ID forge-run/<run UUID> from a Temporal client.
type SDKReader struct{ Client client.Client }

func (s SDKReader) Status(ctx context.Context, workflowID string) (LiveState, error) {
	if s.Client == nil || workflowID == "" {
		return LiveState{}, ErrHistoryUnavailable
	}
	response, err := s.Client.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		var missing *serviceerror.NotFound
		if errors.As(err, &missing) {
			return LiveState{}, ErrAbsent
		}
		return LiveState{}, ErrHistoryUnavailable
	}
	info := response.GetWorkflowExecutionInfo()
	observed := time.Now().UTC()
	switch info.GetStatus() {
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING, enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return LiveState{State: "running", ObservedAt: observed}, nil
	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED:
		return LiveState{State: "canceled", ObservedAt: observed}, nil
	case enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return LiveState{State: "timed_out", ObservedAt: observed}, nil
	case enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return LiveState{State: "terminated", ObservedAt: observed}, nil
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED:
		return LiveState{State: "failed", ObservedAt: observed}, nil
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		view, viewErr := s.completed(ctx, workflowID)
		if viewErr != nil {
			return LiveState{}, ErrHistoryUnavailable
		}
		state := "failed"
		if view.Decision == "complete" {
			state = "succeeded"
		}
		return LiveState{State: state, ObservedAt: observed}, nil
	default:
		return LiveState{}, ErrHistoryUnavailable
	}
}

func (s SDKReader) History(ctx context.Context, workflowID string) ([]Event, error) {
	if s.Client == nil || workflowID == "" {
		return nil, ErrHistoryUnavailable
	}
	iter := s.Client.GetWorkflowHistory(ctx, workflowID, "", false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	raw := make([]*historypb.HistoryEvent, 0)
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			var missing *serviceerror.NotFound
			if errors.As(err, &missing) {
				return nil, ErrAbsent
			}
			return nil, ErrHistoryUnavailable
		}
		raw = append(raw, event)
		if len(raw) > maxHistoryEvents {
			return nil, ErrHistoryUnavailable
		}
	}
	return SanitizeHistory(raw), nil
}

func (s SDKReader) Result(ctx context.Context, workflowID string) (ResultView, error) {
	status, err := s.Status(ctx, workflowID)
	if err != nil {
		return ResultView{}, err
	}
	switch status.State {
	case "running":
		return ResultView{}, ErrNotReady
	case "canceled":
		return ResultView{Decision: "canceled", Reason: "canceled"}, nil
	case "timed_out":
		return ResultView{Decision: "timed_out", Reason: "timed_out"}, nil
	case "terminated":
		return ResultView{Decision: "fail_closed", Reason: "terminated"}, nil
	case "failed":
		return ResultView{Decision: "fail_closed", Reason: "failed"}, nil
	case "succeeded":
		return s.completed(ctx, workflowID)
	default:
		return ResultView{}, ErrHistoryUnavailable
	}
}

func (s SDKReader) completed(ctx context.Context, workflowID string) (ResultView, error) {
	var result Result
	if err := s.Client.GetWorkflow(ctx, workflowID, "").Get(ctx, &result); err != nil {
		return ResultView{}, ErrHistoryUnavailable
	}
	return PublicResult(workflowID, result)
}

// PublicResult keeps the decision, a known reason, and evidence identifiers.
func PublicResult(workflowID string, result Result) (ResultView, error) {
	runID := strings.TrimPrefix(workflowID, "forge-run/")
	if result.RunID != runID || result.WorkflowID != workflowID {
		return ResultView{}, ErrHistoryUnavailable
	}
	switch result.Decision {
	case "complete", "fail_closed", "canceled", "timed_out":
	default:
		return ResultView{}, ErrHistoryUnavailable
	}
	view := ResultView{
		RunID: result.RunID, Decision: result.Decision, Reason: safeReason(result.Decision, result.Reason),
		EvidenceIDs: EvidenceIDs(result.EvidenceIDs),
	}
	if allowedOutcome(result.Outcome) {
		view.Outcome = result.Outcome
	}
	return view, nil
}

// SanitizeHistory maps Temporal events onto the public step vocabulary.
// Activity inputs and results are ignored, including prompts and excerpts.
// Compensate has no public step and is omitted.
func SanitizeHistory(events []*historypb.HistoryEvent) []Event {
	steps := map[int64]string{}
	attempts := map[int64]int{}
	out := make([]Event, 0)
	for _, event := range events {
		if event == nil || event.GetEventId() < 1 || event.GetEventTime() == nil {
			continue
		}
		when := event.GetEventTime().AsTime().UTC()
		switch event.GetEventType() {
		case enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_STARTED:
			out = append(out, newEvent(event, "workflow", 1, "started", when, nil))
		case enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_COMPLETED:
			out = append(out, newEvent(event, "workflow", 1, "succeeded", when, nil))
		case enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED:
			out = append(out, newEvent(event, "workflow", 1, "failed", when, failureFrom(event.GetWorkflowExecutionFailedEventAttributes().GetFailure(), "The workflow failed")))
		case enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_TIMED_OUT:
			out = append(out, newEvent(event, "workflow", 1, "timed_out", when, &SafeFailure{Code: "timed_out", Message: "The workflow timed out"}))
		case enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_CANCELED:
			out = append(out, newEvent(event, "workflow", 1, "canceled", when, nil))
		case enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_TERMINATED:
			out = append(out, newEvent(event, "workflow", 1, "terminated", when, &SafeFailure{Code: "terminated", Message: "The workflow was terminated"}))
		case enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_CANCEL_REQUESTED:
			out = append(out, newEvent(event, "workflow", 1, "cancellation_requested", when, nil))
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_SCHEDULED:
			step := activityStep(event.GetActivityTaskScheduledEventAttributes().GetActivityType().GetName())
			if step != "" {
				steps[event.GetEventId()] = step
			}
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_STARTED:
			scheduled := event.GetActivityTaskStartedEventAttributes().GetScheduledEventId()
			step, ok := steps[scheduled]
			if !ok {
				continue
			}
			attempt := int(event.GetActivityTaskStartedEventAttributes().GetAttempt())
			if attempt < 1 {
				attempt = 1
			}
			attempts[scheduled] = attempt
			out = append(out, newEvent(event, step, attempt, "started", when, nil))
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_COMPLETED:
			if row, ok := activityRow(event, steps, attempts, event.GetActivityTaskCompletedEventAttributes().GetScheduledEventId(), "succeeded", when, nil); ok {
				out = append(out, row)
			}
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_FAILED:
			attrs := event.GetActivityTaskFailedEventAttributes()
			kind := "failed"
			if attrs.GetRetryState() == enumspb.RETRY_STATE_IN_PROGRESS {
				kind = "retry_scheduled"
			}
			failure := failureFrom(attrs.GetFailure(), "The activity failed")
			if row, ok := activityRow(event, steps, attempts, attrs.GetScheduledEventId(), kind, when, failure); ok {
				out = append(out, row)
			}
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_TIMED_OUT:
			attrs := event.GetActivityTaskTimedOutEventAttributes()
			failure := &SafeFailure{Code: "timed_out", Message: "The attempt passed its deadline"}
			if row, ok := activityRow(event, steps, attempts, attrs.GetScheduledEventId(), "timed_out", when, failure); ok {
				out = append(out, row)
			}
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_CANCEL_REQUESTED:
			scheduled := event.GetActivityTaskCancelRequestedEventAttributes().GetScheduledEventId()
			if row, ok := activityRow(event, steps, attempts, scheduled, "cancellation_requested", when, nil); ok {
				out = append(out, row)
			}
		case enumspb.EVENT_TYPE_ACTIVITY_TASK_CANCELED:
			scheduled := event.GetActivityTaskCanceledEventAttributes().GetScheduledEventId()
			if row, ok := activityRow(event, steps, attempts, scheduled, "canceled", when, nil); ok {
				out = append(out, row)
			}
		}
	}
	return out
}

func activityRow(event *historypb.HistoryEvent, steps map[int64]string, attempts map[int64]int, scheduled int64, kind string, when time.Time, failure *SafeFailure) (Event, bool) {
	step, ok := steps[scheduled]
	if !ok {
		return Event{}, false
	}
	attempt := attempts[scheduled]
	if attempt < 1 {
		attempt = 1
	}
	return newEvent(event, step, attempt, kind, when, failure), true
}

func newEvent(event *historypb.HistoryEvent, step string, attempt int, kind string, when time.Time, failure *SafeFailure) Event {
	id := event.GetEventId()
	return Event{EventID: formatID(id), Sequence: id, Step: step, Attempt: attempt, EventType: kind, OccurredAt: when, Failure: failure}
}

func formatID(id int64) string {
	return strconv.FormatInt(id, 10)
}

func activityStep(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	switch name {
	case "Retrieve":
		return "retrieval"
	case "Generate":
		return "generation"
	case "RecordTerminal":
		return "persistence"
	default:
		return ""
	}
}

func failureFrom(failure *failurepb.Failure, fallback string) *SafeFailure {
	code := applicationCode(failure)
	if code == "" {
		return &SafeFailure{Code: "failed", Message: fallback}
	}
	return &SafeFailure{Code: code, Message: safeMessages[code]}
}

func applicationCode(failure *failurepb.Failure) string {
	for failure != nil {
		if info := failure.GetApplicationFailureInfo(); info != nil {
			if _, ok := safeMessages[info.GetType()]; ok {
				return info.GetType()
			}
		}
		failure = failure.GetCause()
	}
	return ""
}

var safeMessages = map[string]string{
	"tool_denied":          "The document or passage is outside the frozen scope",
	"tool_unavailable":     "The tool could not be reached",
	"tool_invalid":         "The tool response was rejected",
	"model_unavailable":    "The model route could not be reached",
	"model_invalid":        "The model output was rejected",
	"citation_invalid":     "A citation did not resolve",
	"persistence_conflict": "The stored attempt conflicted",
	"cancelled":            "The attempt was canceled",
	"timed_out":            "The attempt passed its deadline",
	"identity_mismatch":    "The execution identity did not match",
}

func safeReason(decision, reason string) string {
	if allowedReason(reason) {
		return reason
	}
	if decision == "complete" {
		return "complete"
	}
	return "failed"
}

func allowedReason(reason string) bool {
	switch reason {
	case "complete", "failed", "canceled", "timed_out", "terminated", "none",
		"answered", "insufficient_evidence", "conflicting_evidence",
		"compensation_completed", "compensation_incomplete":
		return true
	default:
		_, ok := safeMessages[reason]
		return ok
	}
}

func allowedOutcome(outcome string) bool {
	return outcome == "answered" || outcome == "insufficient_evidence" || outcome == "conflicting_evidence"
}

var evidenceID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// EvidenceIDs keeps lowercase UUID version-4 identifiers and drops everything else.
func EvidenceIDs(ids []string) []string {
	out := make([]string, 0)
	for _, id := range ids {
		if evidenceID.MatchString(id) {
			out = append(out, id)
		}
	}
	return out
}
