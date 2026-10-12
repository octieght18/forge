// Package execution runs the research workflow on Temporal.
// PostgreSQL keeps the run and the start/cancel command. Temporal keeps
// execution history. This package does not call a model or an MCP tool.
package execution

import (
	"context"
	"errors"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	TaskQueue    = "forge-research"
	WorkflowName = "research.v1"
)

// Correlation is the identity copied from the product command into Temporal.
type Correlation struct {
	RunID      string `json:"run_id"`
	WorkflowID string `json:"workflow_id"`
	WorkloadID string `json:"workload_id"`
	VersionID  string `json:"version_id"`
	CommandID  string `json:"command_id"`
}

// Observation is one activity's view of that identity.
type Observation struct {
	RunID      string `json:"run_id"`
	WorkflowID string `json:"workflow_id"`
	CommandID  string `json:"command_id"`
	Step       string `json:"step"`
	Outcome    string `json:"outcome"`
	Class      string `json:"class,omitempty"`
}

// Result is the workflow's durable decision. It is not a product report.
type Result struct {
	RunID      string `json:"run_id"`
	WorkflowID string `json:"workflow_id"`
	CommandID  string `json:"command_id"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason"`
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
	var generated Observation
	if err := workflow.ExecuteActivity(ctx, Generate, in).Get(ctx, &generated); err != nil {
		return Result{}, err
	}
	if generated.Outcome != "failed" || generated.Class != "model_unavailable" || generated.WorkflowID != info.WorkflowExecution.ID {
		return Result{}, temporal.NewNonRetryableApplicationError("unexpected generation result", "permanent", nil)
	}
	return Result{RunID: in.RunID, WorkflowID: info.WorkflowExecution.ID, CommandID: in.CommandID, Decision: "fail_closed", Reason: "model_unavailable"}, nil
}

// Retrieve checks correlation and does not call a corpus tool.
func Retrieve(ctx context.Context, in Correlation) (Observation, error) {
	if err := activityIdentity(ctx, in); err != nil {
		return Observation{}, err
	}
	return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "retrieval", Outcome: "succeeded"}, nil
}

// Generate reports that the model adapter is not in this worker.
func Generate(ctx context.Context, in Correlation) (Observation, error) {
	if err := activityIdentity(ctx, in); err != nil {
		return Observation{}, err
	}
	return Observation{RunID: in.RunID, WorkflowID: in.WorkflowID, CommandID: in.CommandID, Step: "generation", Outcome: "failed", Class: "model_unavailable"}, nil
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

// ErrAbsent means Temporal has no execution for this workflow ID.
var ErrAbsent = errors.New("workflow execution is absent")

// ErrIdentity means the product workflow ID does not match the run.
var ErrIdentity = errors.New("workflow identity mismatch")
