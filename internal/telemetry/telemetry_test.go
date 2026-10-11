package telemetry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProvisioningAndReconciliationView(t *testing.T) {
	Reset()
	ctx := WithRequestID(context.Background(), "0123456789abcdef0123456789abcdef")
	RecordProvisioning(ctx, "ready", 1500*time.Millisecond, "33333333-3333-4333-8333-333333333333")
	RecordProvisioning(ctx, "failed", 20*time.Millisecond, "33333333-3333-4333-8333-333333333333")
	RecordProvisioning(ctx, "other", time.Second, "not a safe id")
	RecordReconciliation(ctx, 5*time.Millisecond, "Provisioning", "NamespacePending", nil)
	RecordReconciliation(ctx, 5*time.Millisecond, "Failed", "OwnerMismatch", nil)
	RecordReconciliation(ctx, 5*time.Millisecond, "Provisioning", "secret-token-value", nil)
	RecordReconciliation(ctx, 5*time.Millisecond, "Ready", "BoundaryReady", nil)
	RecordReconciliation(ctx, time.Millisecond, "", "", errors.New("private database password"))

	snap := Snapshot()
	if snap.Provisioning.Outcomes["ready"] != 1 || snap.Provisioning.Outcomes["failed"] != 1 || snap.Provisioning.Outcomes["unspecified"] != 1 {
		t.Fatalf("provisioning %#v", snap.Provisioning.Outcomes)
	}
	if snap.Provisioning.Duration["ready"].Count != 1 || snap.Provisioning.Duration["ready"].SumSeconds <= 0 {
		t.Fatalf("duration %#v", snap.Provisioning.Duration)
	}
	if snap.Reconciliation.Outcomes["retry"] != 2 || snap.Reconciliation.Outcomes["success"] != 1 || snap.Reconciliation.Outcomes["failure"] != 2 {
		t.Fatalf("reconciliation %#v", snap.Reconciliation.Outcomes)
	}
	if snap.Reconciliation.RetryReasons["NamespacePending"] != 1 || snap.Reconciliation.RetryReasons["unspecified"] != 1 {
		t.Fatalf("retries %#v", snap.Reconciliation.RetryReasons)
	}
	if snap.Reconciliation.FailureReasons["OwnerMismatch"] != 1 || snap.Reconciliation.FailureReasons["reconcile_error"] != 1 {
		t.Fatalf("failures %#v", snap.Reconciliation.FailureReasons)
	}
	found := false
	for _, span := range snap.Spans {
		if span.Name == "forge.provisioning" && span.RequestID == "0123456789abcdef0123456789abcdef" && span.OperationID == "33333333-3333-4333-8333-333333333333" && span.TraceID != "" {
			found = true
		}
		if span.RequestID == "not a safe id" || span.OperationID == "not a safe id" {
			t.Fatalf("unsafe identity stored %#v", span)
		}
	}
	if !found {
		t.Fatalf("spans %#v", snap.Spans)
	}
	if len(snap.Spans) > spanLimit {
		t.Fatal(len(snap.Spans))
	}
}
