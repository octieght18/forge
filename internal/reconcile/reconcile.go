// Package reconcile delivers durable product commands through a narrow backend
// boundary. It does not own execution status or implement a Temporal adapter.
package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/store"
)

var ErrRetryable = errors.New("retryable backend delivery failure")

type Receipt struct {
	RunID, WorkflowID, Kind string
	Owner                   store.Owner
}

// Backend must honor cancellation/deadlines and reconcile ambiguous responses
// using the original identity. A receipt means verified start/cancel acceptance,
// not workflow completion. It must not blindly restart a missing historical ID.
type Backend interface {
	Deliver(context.Context, store.Delivery) (Receipt, error)
}

type Queue interface {
	Claim(context.Context, time.Duration) (*store.Delivery, error)
	Acknowledge(context.Context, store.Delivery) error
	Fail(context.Context, store.Delivery, string) error
}

type Options struct {
	PollInterval, Lease, OperationTimeout time.Duration
}

func Defaults() Options { return Options{time.Second, 30 * time.Second, 5 * time.Second} }

type Reconciler struct {
	queue     Queue
	backend   Backend
	options   Options
	validator *contract.Validator
}

func New(queue Queue, backend Backend, options Options) (*Reconciler, error) {
	if queue == nil || backend == nil {
		return nil, errors.New("queue and verified backend are required")
	}
	if options.PollInterval < time.Millisecond || options.PollInterval > time.Minute ||
		options.OperationTimeout < time.Millisecond || options.OperationTimeout > 20*time.Second || options.Lease > time.Minute ||
		options.Lease < 3*options.OperationTimeout {
		return nil, errors.New("invalid reconciliation deadlines")
	}
	v, err := contract.New()
	if err != nil {
		return nil, err
	}
	return &Reconciler{queue, backend, options, v}, nil
}

// Step handles at most one command, using independent bounded database and
// delivery operations. Cancellation leaves an unfinished lease to expire safely.
func (r *Reconciler) Step(ctx context.Context) (bool, error) {
	claimCtx, cancel := context.WithTimeout(ctx, r.options.OperationTimeout)
	x, err := r.queue.Claim(claimCtx, r.options.Lease)
	cancel()
	if err != nil || x == nil {
		return false, err
	}
	code := "invalid_payload"
	if r.validate(*x) == nil {
		// Copy the envelope: backend mutations cannot change the expected receipt
		// identity or fencing token used after the call.
		request := *x
		request.Run.Input = append(json.RawMessage(nil), x.Run.Input...)
		request.Spec = append(json.RawMessage(nil), x.Spec...)
		if x.Run.RerunOf != nil {
			parent := *x.Run.RerunOf
			request.Run.RerunOf = &parent
		}
		deliveryCtx, end := context.WithTimeout(ctx, r.options.OperationTimeout)
		receipt, backendErr := r.backend.Deliver(deliveryCtx, request)
		deadlineErr := deliveryCtx.Err()
		end()
		if ctx.Err() != nil {
			return true, ctx.Err()
		}
		switch {
		case deadlineErr != nil:
			code = "deadline"
		case backendErr == nil:
			if receipt.RunID == x.Run.ID && receipt.WorkflowID == x.Run.WorkflowID && receipt.Kind == x.Kind && receipt.Owner == x.Run.Owner {
				ackCtx, end := context.WithTimeout(ctx, r.options.OperationTimeout)
				defer end()
				return true, r.queue.Acknowledge(ackCtx, *x)
			}
			code = "identity_mismatch"
		case errors.Is(backendErr, ErrRetryable):
			code = "retryable"
		default:
			// Unclassified failures do not silently acquire an endless retry policy.
			code = "permanent"
		}
	}
	failedCtx, end := context.WithTimeout(ctx, r.options.OperationTimeout)
	defer end()
	return true, r.queue.Fail(failedCtx, *x, code)
}

// Run is single-flight. Callers own logging/restart policy; raw backend failures
// are never returned. Database/fencing failures stop this loop for inspection.
func (r *Reconciler) Run(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if _, err := r.Step(ctx); err != nil {
			return err
		}
		timer := time.NewTimer(r.options.PollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *Reconciler) validate(x store.Delivery) error {
	if x.Kind != "start" && x.Kind != "cancel" || x.Attempt < 1 || x.Attempt > 8 || x.LeaseUntil.IsZero() || x.Run.WorkflowID != "forge-run/"+x.Run.ID {
		return contract.ErrInvalidInput
	}
	for _, id := range []string{x.CommandID, x.Token, x.Run.ID, x.Run.WorkloadID, x.Run.VersionID} {
		b, _ := json.Marshal(id)
		if err := r.validator.Validate("ID", b); err != nil {
			return err
		}
	}
	owner, _ := json.Marshal(x.Run.Owner)
	if err := r.validator.Validate("Owner", owner); err != nil {
		return err
	}
	if err := r.validator.ValidateRequest("CreateRunRequest", x.Run.Input); err != nil {
		return err
	}
	var input struct {
		WorkloadID string   `json:"workload_id"`
		VersionID  string   `json:"version_id"`
		Documents  []string `json:"document_ids"`
	}
	_ = json.Unmarshal(x.Run.Input, &input)
	if input.WorkloadID != x.Run.WorkloadID || input.VersionID != x.Run.VersionID {
		return contract.ErrInvalidInput
	}
	spec, err := json.Marshal(map[string]any{"spec": x.Spec})
	if err != nil {
		return contract.ErrInvalidInput
	}
	hash, err := r.validator.VersionFingerprint(spec)
	if err != nil || hash != x.SpecSHA256 {
		return contract.ErrInvalidInput
	}
	var scope struct {
		Permissions struct {
			Documents []string `json:"document_ids"`
		} `json:"permissions"`
	}
	_ = json.Unmarshal(x.Spec, &scope)
	allowed := make(map[string]bool, len(scope.Permissions.Documents))
	for _, id := range scope.Permissions.Documents {
		allowed[id] = true
	}
	for _, id := range input.Documents {
		if !allowed[id] {
			return contract.ErrInvalidInput
		}
	}
	return nil
}
