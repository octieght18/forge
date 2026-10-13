// Package execution runs the research workflow on Temporal.
// PostgreSQL keeps the run and the start/cancel command. Temporal keeps
// execution history. Model execution stays a later ticket. A configured corpus
// tool is called only after the frozen document scope is checked.
package execution

import (
	"context"
	"errors"
	"sync"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/octieght18/forge/internal/audit"
	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
)

// corpus is the worker-owned tool. A nil tool performs no corpus call.
// generator is the worker-owned model. A nil model reports model_unavailable.
// A cancelled activity can still read these from Temporal's heartbeat goroutine
// after the test environment has returned, so every swap takes the lock.
var (
	dependencyMu sync.RWMutex
	corpus       mcp.Tool
	generator    model.Generator
)

// SetCorpus installs the tool activities call.
func SetCorpus(tool mcp.Tool) {
	dependencyMu.Lock()
	corpus = tool
	dependencyMu.Unlock()
}

// SetModel installs the generator activities call.
func SetModel(next model.Generator) {
	dependencyMu.Lock()
	generator = next
	dependencyMu.Unlock()
}

func currentCorpus() mcp.Tool {
	dependencyMu.RLock()
	defer dependencyMu.RUnlock()
	return corpus
}

func currentModel() model.Generator {
	dependencyMu.RLock()
	defer dependencyMu.RUnlock()
	return generator
}

const (
	TaskQueue    = "forge-research"
	WorkflowName = "research.v1"
)

// Correlation is the identity copied from the product command into Temporal.
type Correlation struct {
	RunID       string   `json:"run_id"`
	WorkflowID  string   `json:"workflow_id"`
	WorkloadID  string   `json:"workload_id"`
	VersionID   string   `json:"version_id"`
	CommandID   string   `json:"command_id"`
	SnapshotID  string   `json:"snapshot_id,omitempty"`
	Question    string   `json:"question,omitempty"`
	DocumentIDs []string `json:"document_ids,omitempty"`
	MaxPassages int      `json:"max_passages,omitempty"`
}

// Observation is one activity's view of that identity.
type Observation struct {
	RunID      string        `json:"run_id"`
	WorkflowID string        `json:"workflow_id"`
	CommandID  string        `json:"command_id"`
	Step       string        `json:"step"`
	Outcome    string        `json:"outcome"`
	Class      string        `json:"class,omitempty"`
	Passages   []mcp.Passage `json:"passages,omitempty"`
	Usage      model.Usage   `json:"usage,omitempty"`
}

// Result is the workflow's durable decision. It is not a product report.
type Result struct {
	RunID       string      `json:"run_id"`
	WorkflowID  string      `json:"workflow_id"`
	CommandID   string      `json:"command_id"`
	Decision    string      `json:"decision"`
	Reason      string      `json:"reason"`
	Outcome     string      `json:"outcome,omitempty"`
	RequestedID string      `json:"requested_id,omitempty"`
	ActualID    string      `json:"actual_id,omitempty"`
	ProviderID  string      `json:"provider_id,omitempty"`
	EvidenceIDs []string    `json:"evidence_ids,omitempty"`
	Usage       model.Usage `json:"usage,omitempty"`
}

// Research replays from Temporal history. External work stays in activities.
func Research(ctx workflow.Context, in Correlation) (Result, error) {
	info := workflow.GetInfo(ctx)
	if !sameIdentity(info.WorkflowExecution.ID, in) {
		return Result{}, temporal.NewNonRetryableApplicationError("workflow identity mismatch", "identity_mismatch", nil)
	}
	ctx = workflow.WithActivityOptions(ctx, activityOptions())
	var retrieved Observation
	if err := workflow.ExecuteActivity(ctx, Retrieve, in).Get(ctx, &retrieved); err != nil {
		return canceled(ctx, in, err)
	}
	if retrieved.WorkflowID != info.WorkflowExecution.ID || retrieved.RunID != in.RunID || retrieved.CommandID != in.CommandID {
		return Result{}, temporal.NewNonRetryableApplicationError("correlation mismatch", "identity_mismatch", nil)
	}
	if retrieved.Outcome == "failed" {
		return Result{RunID: in.RunID, WorkflowID: info.WorkflowExecution.ID, CommandID: in.CommandID, Decision: "fail_closed", Reason: retrieved.Class}, nil
	}
	var generated Observation
	if err := workflow.ExecuteActivity(ctx, Generate, in, retrieved.Passages).Get(ctx, &generated); err != nil {
		return canceled(ctx, in, err)
	}
	if generated.WorkflowID != info.WorkflowExecution.ID {
		return Result{}, temporal.NewNonRetryableApplicationError("correlation mismatch", "identity_mismatch", nil)
	}
	if generated.Outcome == "succeeded" {
		if err := recordTerminal(ctx, in, "complete", "none"); err != nil {
			return Result{}, err
		}
		return Result{
			RunID: in.RunID, WorkflowID: info.WorkflowExecution.ID, CommandID: in.CommandID,
			Decision: "complete", Reason: generated.Class, Outcome: generated.Class,
			RequestedID: model.RequestedModel, EvidenceIDs: evidenceIDs(retrieved.Passages), Usage: generated.Usage,
		}, nil
	}
	if generated.Outcome != "failed" || generated.Class != "model_unavailable" {
		return Result{}, temporal.NewNonRetryableApplicationError("unexpected generation result", "permanent", nil)
	}
	compensation := "compensation_completed"
	if err := workflow.ExecuteActivity(ctx, Compensate, in).Get(ctx, nil); err != nil {
		compensation = "compensation_incomplete"
	}
	if err := recordTerminal(ctx, in, "fail_closed", compensation); err != nil {
		return Result{}, err
	}
	reason := "model_unavailable"
	if compensation == "compensation_incomplete" {
		reason = compensation
	}
	return Result{RunID: in.RunID, WorkflowID: info.WorkflowExecution.ID, CommandID: in.CommandID, Decision: "fail_closed", Reason: reason}, nil
}

// Retrieve checks correlation. With a configured corpus tool it resolves the
// frozen context and calls corpus.search. Without one it does not start a tool.
func Retrieve(ctx context.Context, in Correlation) (Observation, error) {
	entered()
	tool := currentCorpus()
	if tool != nil {
		key := IdempotencyKey(in.RunID, "retrieval", material(in))
		if saved, ok := recall(key); ok {
			return saved, nil
		}
	}
	if err := activityIdentity(ctx, in); err != nil {
		return Observation{}, err
	}
	if tool == nil {
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "succeeded"}, nil
	}
	key := IdempotencyKey(in.RunID, "retrieval", material(in))
	beat(ctx, in.RunID)
	outcome, err := mcp.Search(ctx, mcp.Input{
		RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, SnapshotID: in.SnapshotID,
		Question: in.Question, DocumentIDs: in.DocumentIDs, MaxPassages: in.MaxPassages,
	}, tool)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Observation{}, err
		}
		return Observation{}, failure("tool_unavailable", err)
	}
	if outcome.Failed {
		if Retryable(outcome.Class) {
			return Observation{}, failure(outcome.Class, nil)
		}
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "failed", Class: outcome.Class}, nil
	}
	obs := Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "succeeded", Passages: outcome.Passages}
	remember(key, obs)
	audit.NoteTool(key)
	return obs, nil
}

// Generate asks the configured model for a report. Without a model it does not
// invent a result.
func Generate(ctx context.Context, in Correlation, passages []mcp.Passage) (Observation, error) {
	entered()
	if err := activityIdentity(ctx, in); err != nil {
		return Observation{}, err
	}
	gen := currentModel()
	if gen == nil {
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "failed", Class: "model_unavailable"}, nil
	}
	key := IdempotencyKey(in.RunID, "generation", material(in))
	if saved, ok := recall(key); ok {
		return saved, nil
	}
	beat(ctx, in.RunID)
	report, err := gen.Generate(ctx, in.RunID, in.Question, passages)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return Observation{}, err
		}
		return Observation{}, failure("model_unavailable", err)
	}
	report.WorkflowID = in.WorkflowID
	if err = model.Validate(report, passages); err != nil {
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "failed", Class: "model_invalid"}, nil
	}
	obs := Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "succeeded", Class: report.Outcome, Passages: passages, Usage: report.Usage}
	remember(key, obs)
	audit.NoteModel(key, report.Usage.PromptTokens, report.Usage.CompletionTokens)
	return obs, nil
}

// Compensate withdraws evidence and reports left behind when a run stops after retrieval.
// A second compensation for the same run does not repeat the withdrawal.
func Compensate(ctx context.Context, in Correlation) error {
	entered()
	key := IdempotencyKey(in.RunID, "compensate", "partial-evidence")
	if _, ok := recall(key); ok {
		return nil
	}
	if err := activityIdentity(ctx, in); err != nil {
		return err
	}
	mcp.Compensate(in.RunID)
	model.Compensate(in.RunID)
	remember(key, Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "compensate", Outcome: "succeeded"})
	return nil
}

func activityOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		StartToCloseTimeout: ActivityTimeout,
		HeartbeatTimeout:    HeartbeatTimeout,
		RetryPolicy:         Policy(),
	}
}

func recordTerminal(ctx workflow.Context, in Correlation, state, compensation string) error {
	return workflow.ExecuteActivity(ctx, RecordTerminal, Terminal{RunID: in.RunID, WorkflowID: in.WorkflowID, State: state, Compensation: compensation}).Get(ctx, nil)
}

func canceled(ctx workflow.Context, in Correlation, err error) (Result, error) {
	if err != nil && !temporal.IsCanceledError(err) {
		return Result{}, err
	}
	disconnected, stop := workflow.NewDisconnectedContext(ctx)
	defer stop()
	disconnected = workflow.WithActivityOptions(disconnected, workflow.ActivityOptions{StartToCloseTimeout: ActivityTimeout, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	compensation := "compensation_completed"
	if compErr := workflow.ExecuteActivity(disconnected, Compensate, in).Get(disconnected, nil); compErr != nil {
		compensation = "compensation_incomplete"
	}
	_ = recordTerminal(disconnected, in, "canceled", compensation)
	return Result{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Decision: "canceled", Reason: compensation}, temporal.NewCanceledError(compensation)
}

func beat(ctx context.Context, runID string) {
	activity.RecordHeartbeat(ctx, runID)
}

func activityIdentity(ctx context.Context, in Correlation) error {
	info := activity.GetInfo(ctx)
	if info.WorkflowExecution.ID != in.WorkflowID || !sameIdentity(in.WorkflowID, in) {
		return temporal.NewNonRetryableApplicationError("activity identity mismatch", "identity_mismatch", nil)
	}
	return nil
}

func sameIdentity(workflowID string, in Correlation) bool {
	return workflowID == in.WorkflowID && in.WorkflowID == "forge-run/"+in.RunID && in.RunID != "" && in.CommandID != ""
}

func evidenceIDs(passages []mcp.Passage) []string {
	ids := make([]string, 0, len(passages))
	for _, passage := range passages {
		if passage.EvidenceID != "" {
			ids = append(ids, passage.EvidenceID)
		}
	}
	return ids
}

// ErrAbsent means Temporal has no execution for this workflow ID.
var ErrAbsent = errors.New("workflow execution is absent")

// ErrIdentity means the product workflow ID does not match the run.
var ErrIdentity = errors.New("workflow identity mismatch")
