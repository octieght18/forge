// Package execution runs the research workflow on Temporal.
// PostgreSQL keeps the run and the start/cancel command. Temporal keeps
// execution history. Model execution stays a later ticket. A configured corpus
// tool is called only after the frozen document scope is checked.
package execution

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
)

// Corpus is the worker-owned tool. A nil tool performs no corpus call.
var Corpus mcp.Tool

// Model is the worker-owned generator. A nil model reports model_unavailable.
var Model model.Generator

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
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	})
	var retrieved Observation
	if err := workflow.ExecuteActivity(ctx, Retrieve, in).Get(ctx, &retrieved); err != nil {
		return Result{}, err
	}
	if retrieved.WorkflowID != info.WorkflowExecution.ID || retrieved.RunID != in.RunID || retrieved.CommandID != in.CommandID {
		return Result{}, temporal.NewNonRetryableApplicationError("correlation mismatch", "identity_mismatch", nil)
	}
	if retrieved.Outcome == "failed" {
		return Result{RunID: in.RunID, WorkflowID: info.WorkflowExecution.ID, CommandID: in.CommandID, Decision: "fail_closed", Reason: retrieved.Class}, nil
	}
	var generated Observation
	if err := workflow.ExecuteActivity(ctx, Generate, in, retrieved.Passages).Get(ctx, &generated); err != nil {
		return Result{}, err
	}
	if generated.WorkflowID != info.WorkflowExecution.ID {
		return Result{}, temporal.NewNonRetryableApplicationError("correlation mismatch", "identity_mismatch", nil)
	}
	if generated.Outcome == "succeeded" {
		return Result{
			RunID: in.RunID, WorkflowID: info.WorkflowExecution.ID, CommandID: in.CommandID,
			Decision: "complete", Reason: generated.Class, Outcome: generated.Class,
			RequestedID: model.RequestedModel, EvidenceIDs: evidenceIDs(retrieved.Passages), Usage: generated.Usage,
		}, nil
	}
	if generated.Outcome != "failed" || generated.Class != "model_unavailable" {
		return Result{}, temporal.NewNonRetryableApplicationError("unexpected generation result", "permanent", nil)
	}
	return Result{RunID: in.RunID, WorkflowID: info.WorkflowExecution.ID, CommandID: in.CommandID, Decision: "fail_closed", Reason: "model_unavailable"}, nil
}

// Retrieve checks correlation. With a configured corpus tool it resolves the
// frozen context and calls corpus.search. Without one it does not start a tool.
func Retrieve(ctx context.Context, in Correlation) (Observation, error) {
	if err := activityIdentity(ctx, in); err != nil {
		return Observation{}, err
	}
	if Corpus == nil {
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "succeeded"}, nil
	}
	outcome, err := mcp.Search(ctx, mcp.Input{
		RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, SnapshotID: in.SnapshotID,
		Question: in.Question, DocumentIDs: in.DocumentIDs, MaxPassages: in.MaxPassages,
	}, Corpus)
	if err != nil {
		return Observation{}, temporal.NewNonRetryableApplicationError(mcp.Redact(err.Error()), "tool_unavailable", nil)
	}
	if outcome.Failed {
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "failed", Class: outcome.Class}, nil
	}
	return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "succeeded", Passages: outcome.Passages}, nil
}

// Generate asks the configured model for a report. Without a model it does not
// invent a result.
func Generate(ctx context.Context, in Correlation, passages []mcp.Passage) (Observation, error) {
	if err := activityIdentity(ctx, in); err != nil {
		return Observation{}, err
	}
	if Model == nil {
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "failed", Class: "model_unavailable"}, nil
	}
	report, err := Model.Generate(ctx, in.RunID, in.Question, passages)
	if err != nil {
		return Observation{}, temporal.NewNonRetryableApplicationError(mcp.Redact(err.Error()), "model_unavailable", nil)
	}
	report.WorkflowID = in.WorkflowID
	if err = model.Validate(report, passages); err != nil {
		return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "failed", Class: "model_invalid"}, nil
	}
	return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "succeeded", Class: report.Outcome, Passages: passages, Usage: report.Usage}, nil
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
