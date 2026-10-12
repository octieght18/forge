package execution

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/octieght18/forge/internal/reconcile"
	"github.com/octieght18/forge/internal/store"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// Status is the Temporal execution state the dispatcher is allowed to see.
type Status string

const (
	StatusRunning Status = "running"
	StatusClosed  Status = "closed"
	StatusAbsent  Status = "absent"
)

// Starter is the Temporal client surface used to accept a command.
type Starter interface {
	Start(context.Context, string, Correlation) error
	Describe(context.Context, string) (Status, error)
	Cancel(context.Context, string) error
}

// Backend delivers one product command to Temporal. A receipt means the
// workflow ID was accepted or the cancel was requested. It does not mean the
// research result is ready, and it does not write execution status to PostgreSQL.
type Backend struct{ Client Starter }

func (b Backend) Deliver(ctx context.Context, x store.Delivery) (reconcile.Receipt, error) {
	receipt := reconcile.Receipt{RunID: x.Run.ID, WorkflowID: x.Run.WorkflowID, Kind: x.Kind, Owner: x.Run.Owner}
	if x.Run.WorkflowID != "forge-run/"+x.Run.ID {
		return receipt, ErrIdentity
	}
	switch x.Kind {
	case "start":
		status, err := b.Client.Describe(ctx, x.Run.WorkflowID)
		if err != nil {
			return receipt, reconcile.ErrRetryable
		}
		if status == StatusRunning || status == StatusClosed {
			return receipt, nil
		}
		corr, corrErr := correlation(x)
		if corrErr != nil {
			return receipt, corrErr
		}
		err = b.Client.Start(ctx, x.Run.WorkflowID, corr)
		if err != nil && !errors.Is(err, errAlreadyStarted) {
			return receipt, reconcile.ErrRetryable
		}
		return receipt, nil
	case "cancel":
		status, err := b.Client.Describe(ctx, x.Run.WorkflowID)
		if err != nil {
			return receipt, reconcile.ErrRetryable
		}
		if status == StatusAbsent {
			return receipt, ErrAbsent
		}
		if status == StatusClosed {
			return receipt, nil
		}
		if err = b.Client.Cancel(ctx, x.Run.WorkflowID); err != nil {
			if errors.Is(err, ErrAbsent) {
				return receipt, ErrAbsent
			}
			return receipt, reconcile.ErrRetryable
		}
		return receipt, nil
	default:
		return receipt, ErrIdentity
	}
}

var errAlreadyStarted = errors.New("workflow already started")

func correlation(x store.Delivery) (Correlation, error) {
	out := Correlation{
		RunID: x.Run.ID, WorkflowID: x.Run.WorkflowID, WorkloadID: x.Run.WorkloadID,
		VersionID: x.Run.VersionID, CommandID: x.CommandID,
	}
	var input struct {
		Question  string   `json:"question"`
		Documents []string `json:"document_ids"`
	}
	var spec struct {
		Snapshot    string `json:"corpus_snapshot_id"`
		Permissions struct {
			Documents []string `json:"document_ids"`
		} `json:"permissions"`
		Limits struct {
			Max int `json:"max_passages"`
		} `json:"limits"`
	}
	_ = json.Unmarshal(x.Run.Input, &input)
	_ = json.Unmarshal(x.Spec, &spec)
	allowed := map[string]bool{}
	for _, id := range spec.Permissions.Documents {
		allowed[id] = true
	}
	for _, id := range input.Documents {
		if !allowed[id] {
			return Correlation{}, ErrIdentity
		}
	}
	out.SnapshotID = spec.Snapshot
	out.DocumentIDs = input.Documents
	out.Question = input.Question
	out.MaxPassages = spec.Limits.Max
	return out, nil
}

// SDK adapts the Temporal client. Workflow ID reuse stays rejected.
type SDK struct{ Client client.Client }

func (s SDK) Start(ctx context.Context, workflowID string, in Correlation) error {
	_, err := s.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID: workflowID, TaskQueue: TaskQueue,
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}, WorkflowName, in)
	if temporal.IsWorkflowExecutionAlreadyStartedError(err) {
		return errAlreadyStarted
	}
	return err
}

func (s SDK) Describe(ctx context.Context, workflowID string) (Status, error) {
	response, err := s.Client.DescribeWorkflowExecution(ctx, workflowID, "")
	if err != nil {
		var missing *serviceerror.NotFound
		if errors.As(err, &missing) {
			return StatusAbsent, nil
		}
		return "", err
	}
	switch response.GetWorkflowExecutionInfo().GetStatus() {
	case enums.WORKFLOW_EXECUTION_STATUS_RUNNING, enums.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return StatusRunning, nil
	default:
		return StatusClosed, nil
	}
}

func (s SDK) Cancel(ctx context.Context, workflowID string) error {
	err := s.Client.CancelWorkflow(ctx, workflowID, "")
	var missing *serviceerror.NotFound
	if errors.As(err, &missing) {
		return ErrAbsent
	}
	return err
}
