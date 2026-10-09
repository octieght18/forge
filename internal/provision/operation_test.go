package provision

import (
	"errors"
	"testing"
	"time"
)

func sample(action string) Operation {
	return Operation{
		ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", WorkloadID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		VersionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", Action: action, DesiredGeneration: 1,
		Status: Accepted, Deadline: time.Date(2026, 10, 9, 18, 0, 30, 0, time.UTC),
	}
}

func TestAcceptRejectsConflictAndStaleVersion(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 0, 0, 0, time.UTC)
	if _, err := Accept("latest", "older", ActionProvision, 1, false, now, DefaultTimeout); !errors.Is(err, ErrVersion) {
		t.Fatal(err)
	}
	if _, err := Accept("latest", "latest", ActionProvision, 2, true, now, DefaultTimeout); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := Accept("latest", "latest", "apply", 1, false, now, DefaultTimeout); !errors.Is(err, ErrAction) {
		t.Fatal(err)
	}
	if _, err := Accept("latest", "latest", ActionDelete, 1, false, now, 0); !errors.Is(err, ErrAction) {
		t.Fatal(err)
	}
	got, err := Accept("latest", "latest", ActionProvision, 3, false, now, time.Second)
	if err != nil || got.Status != Accepted || got.DesiredGeneration != 3 || !got.Deadline.Equal(now.Add(time.Second)) {
		t.Fatalf("%#v %v", got, err)
	}
}

func TestObservationKeepsDesiredAheadUntilControllerAgrees(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 0, 1, 0, time.UTC)
	op := sample(ActionProvision)
	op = Advance(op, &Observation{Generation: 1, Phase: "Provisioning", Reason: "Reconciling"}, now)
	if op.Status != Provisioning || op.ObservedGeneration != 0 || op.ObservedPhase != "Provisioning" || op.Terminal() {
		t.Fatalf("%#v", op)
	}
	stale := Advance(op, &Observation{Generation: 9, Phase: "Deleting", Reason: "CleanupPending"}, now)
	if stale.Status != Provisioning || stale.ObservedGeneration != 0 {
		t.Fatalf("delete observation changed a provision %#v", stale)
	}
	ready := Advance(op, &Observation{Generation: 4, Phase: "Ready", Reason: "BoundaryReady"}, now)
	if ready.Status != Ready || ready.ObservedGeneration != ready.DesiredGeneration || ready.ErrorCode != "" {
		t.Fatalf("%#v", ready)
	}
	late := Advance(ready, &Observation{Phase: "Failed", Reason: "OwnerMismatch"}, ready.Deadline.Add(time.Hour))
	if late.Status != Ready || late.ErrorCode != "" {
		t.Fatalf("finished operation changed %#v", late)
	}
}

func TestFailureCancelAndTimeout(t *testing.T) {
	now := time.Date(2026, 10, 9, 18, 0, 1, 0, time.UTC)
	failed := Advance(sample(ActionProvision), &Observation{Phase: "Failed", Reason: "OwnershipConflict"}, now)
	if failed.Status != Failed || failed.ErrorCode != "ownership_conflict" || failed.ErrorMessage == "" || failed.ObservedGeneration != 1 {
		t.Fatalf("%#v", failed)
	}
	identity := Advance(sample(ActionDelete), &Observation{Phase: "Failed", Reason: "InvalidIdentity"}, now)
	if identity.ErrorCode != "identity_mismatch" {
		t.Fatal(identity.ErrorCode)
	}

	canceled, err := Cancel(sample(ActionProvision), now, DefaultTimeout)
	if err != nil || canceled.Status != CancelRequested || canceled.Action != ActionDelete || canceled.DesiredGeneration != 2 || canceled.ObservedPhase != "" {
		t.Fatalf("%#v %v", canceled, err)
	}
	again, err := Cancel(canceled, now.Add(time.Minute), DefaultTimeout)
	if err != nil || again.DesiredGeneration != canceled.DesiredGeneration || !again.Deadline.Equal(canceled.Deadline) {
		t.Fatalf("cancel was not stable %#v %v", again, err)
	}
	if _, err = Cancel(failed, now, DefaultTimeout); !errors.Is(err, ErrTerminal) {
		t.Fatal(err)
	}
	waiting := Advance(canceled, &Observation{Phase: "Absent", Reason: "CleanupComplete"}, now)
	if waiting.Status != CancelRequested || waiting.ObservedGeneration != 0 {
		t.Fatalf("absent before deleting completed cancel %#v", waiting)
	}
	deleting := Advance(canceled, &Observation{Phase: "Deleting", Reason: "CleanupPending"}, now)
	if deleting.Status != CancelRequested || deleting.ObservedPhase != "Deleting" || deleting.ObservedGeneration != 0 {
		t.Fatalf("%#v", deleting)
	}
	done := Advance(deleting, &Observation{Phase: "Absent", Reason: "CleanupComplete"}, now)
	if done.Status != Canceled || done.ErrorCode != "canceled" || done.ErrorMessage == "" || done.ObservedGeneration != done.DesiredGeneration {
		t.Fatalf("%#v", done)
	}
	blocked := Advance(deleting, &Observation{Phase: "Deleting", Reason: "OwnershipConflict"}, now)
	if blocked.Status != Failed || blocked.ErrorCode != "ownership_conflict" {
		t.Fatalf("%#v", blocked)
	}

	expired := Advance(sample(ActionProvision), nil, sample(ActionProvision).Deadline)
	if expired.Status != TimedOut || expired.ErrorCode != "operation_timeout" || expired.ErrorMessage == "" {
		t.Fatalf("%#v", expired)
	}
	atDeadline := Advance(sample(ActionProvision), &Observation{Phase: "Ready", Reason: "BoundaryReady"}, sample(ActionProvision).Deadline)
	if atDeadline.Status != Ready {
		t.Fatalf("matching observation lost to the deadline %#v", atDeadline)
	}
}
