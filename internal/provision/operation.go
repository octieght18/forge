// Package provision tracks asynchronous environment operations.
// An accepted operation returns before the controller finishes. Desired action
// and observed controller phase stay comparable; this package does not create
// namespaces or start runs.
package provision

import (
	"errors"
	"time"
)

const (
	ActionProvision = "provision"
	ActionDelete    = "delete"
	DefaultTimeout  = 30 * time.Second
	MaxTimeout      = 120 * time.Second
)

type Status string

const (
	Accepted        Status = "accepted"
	Provisioning    Status = "provisioning"
	Ready           Status = "ready"
	Failed          Status = "failed"
	CancelRequested Status = "cancel_requested"
	Canceled        Status = "canceled"
	TimedOut        Status = "timed_out"
)

var (
	ErrConflict = errors.New("open provisioning operation")
	ErrVersion  = errors.New("provisioning version conflict")
	ErrTerminal = errors.New("terminal provisioning operation")
	ErrAction   = errors.New("unsupported provisioning action")
)

// Operation is the durable handoff between the API and the environment controller.
type Operation struct {
	ID                 string
	WorkloadID         string
	VersionID          string
	Action             string
	DesiredGeneration  int64
	ObservedGeneration int64
	Status             Status
	ObservedPhase      string
	ErrorCode          string
	ErrorMessage       string
	Deadline           time.Time
}

// Observation is one controller phase. Generation is the controller object
// generation and is not the operation epoch.
type Observation struct {
	Generation int64
	Phase      string
	Reason     string
}

func StatusPath(id string) string { return "/api/v1/operations/" + id }

func (o Operation) Terminal() bool {
	switch o.Status {
	case Ready, Failed, Canceled, TimedOut:
		return true
	default:
		return false
	}
}

// Accept records a new operation. latest must be the workload's current version.
// An open operation or any other version conflicts and changes nothing.
func Accept(latest, requested, action string, generation int64, open bool, now time.Time, timeout time.Duration) (Operation, error) {
	if action != ActionProvision && action != ActionDelete {
		return Operation{}, ErrAction
	}
	if timeout < time.Second || timeout > MaxTimeout {
		return Operation{}, ErrAction
	}
	if generation < 1 {
		return Operation{}, ErrAction
	}
	if open {
		return Operation{}, ErrConflict
	}
	if requested == "" || requested != latest {
		return Operation{}, ErrVersion
	}
	return Operation{
		VersionID: requested, Action: action, DesiredGeneration: generation,
		Status: Accepted, Deadline: now.Add(timeout),
	}, nil
}

// Cancel asks the controller to delete the boundary. A second cancel is unchanged.
// A finished operation is left unchanged and reported as terminal.
func Cancel(op Operation, now time.Time, timeout time.Duration) (Operation, error) {
	if op.Terminal() {
		return op, ErrTerminal
	}
	if op.Status == CancelRequested && op.Action == ActionDelete {
		return op, nil
	}
	if timeout < time.Second || timeout > MaxTimeout {
		timeout = DefaultTimeout
	}
	op.Action = ActionDelete
	op.Status = CancelRequested
	op.DesiredGeneration++
	op.ObservedPhase = ""
	op.ErrorCode = ""
	op.ErrorMessage = ""
	op.Deadline = now.Add(timeout)
	return op, nil
}

// Advance applies one observation, then a deadline. A finished operation is
// stable: a late controller update or a passed clock does not reopen it.
// Ready during deletion, and Absent before Deleting, do not count as observed.
func Advance(op Operation, obs *Observation, now time.Time) Operation {
	if op.Terminal() {
		return op
	}
	if obs != nil {
		if code, message, terminal := controllerFailure(op.Action, obs); terminal {
			op.Status = Failed
			op.ObservedGeneration = op.DesiredGeneration
			op.ObservedPhase = "Failed"
			op.ErrorCode = code
			op.ErrorMessage = message
			return op
		}
		switch {
		case op.Action == ActionProvision && obs.Phase == "Provisioning":
			op.Status = Provisioning
			op.ObservedPhase = "Provisioning"
		case op.Action == ActionProvision && obs.Phase == "Ready":
			op.Status = Ready
			op.ObservedGeneration = op.DesiredGeneration
			op.ObservedPhase = "Ready"
			op.ErrorCode = ""
			op.ErrorMessage = ""
			return op
		case op.Action == ActionDelete && obs.Phase == "Deleting":
			if op.Status != CancelRequested {
				op.Status = Provisioning
			}
			op.ObservedPhase = "Deleting"
		case op.Action == ActionDelete && obs.Phase == "Absent" && op.ObservedPhase == "Deleting":
			op.Status = Canceled
			op.ObservedGeneration = op.DesiredGeneration
			op.ObservedPhase = "Absent"
			op.ErrorCode = "canceled"
			op.ErrorMessage = "The controller finished deleting the environment boundary."
			return op
		}
	}
	if !now.Before(op.Deadline) {
		op.Status = TimedOut
		op.ErrorCode = "operation_timeout"
		op.ErrorMessage = "The operation exceeded its deadline before the controller reached the desired state."
	}
	return op
}

func controllerFailure(action string, obs *Observation) (string, string, bool) {
	if obs.Reason == "OwnershipConflict" {
		return "ownership_conflict", "The controller found a boundary it does not own.", true
	}
	if obs.Phase != "Failed" || (action != ActionProvision && action != ActionDelete) {
		return "", "", false
	}
	switch obs.Reason {
	case "OwnerMismatch", "InvalidIdentity":
		return "identity_mismatch", "The controller could not verify the workload owner.", true
	case "PersistentStoragePresent":
		return "controller_failed", "Deletion is blocked because persistent storage is still present.", true
	default:
		return "controller_failed", "The controller reported a terminal failure.", true
	}
}
