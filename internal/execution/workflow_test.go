package execution

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/octieght18/forge/internal/reconcile"
	"github.com/octieght18/forge/internal/store"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

func TestWorkflowPropagatesCorrelationAndStopsBeforeModel(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{})
	env.RegisterActivity(Retrieve)
	env.RegisterActivity(Generate)
	in := Correlation{
		RunID: "33333333-3333-4333-8333-333333333333", WorkflowID: "forge-run/33333333-3333-4333-8333-333333333333",
		WorkloadID: "11111111-1111-4111-8111-111111111111", VersionID: "22222222-2222-4222-8222-222222222222",
		CommandID: "44444444-4444-4444-8444-444444444444",
	}
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: in.WorkflowID})
	env.ExecuteWorkflow(Research, in)
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() != nil {
		t.Fatal(env.GetWorkflowError())
	}
	var got Result
	if err := env.GetWorkflowResult(&got); err != nil {
		t.Fatal(err)
	}
	if got.Decision != "fail_closed" || got.Reason != "model_unavailable" || got.WorkflowID != in.WorkflowID || got.CommandID != in.CommandID || got.RunID != in.RunID {
		t.Fatalf("%#v", got)
	}
}

type fakeTemporal struct {
	status  Status
	starts  int
	cancels int
}

func (f *fakeTemporal) Start(context.Context, string, Correlation) error {
	f.starts++
	f.status = StatusRunning
	return nil
}
func (f *fakeTemporal) Describe(context.Context, string) (Status, error) { return f.status, nil }
func (f *fakeTemporal) Cancel(context.Context, string) error {
	f.cancels++
	f.status = StatusClosed
	return nil
}

func TestDeliveryUsesOneWorkflowID(t *testing.T) {
	fake := &fakeTemporal{status: StatusAbsent}
	backend := Backend{Client: fake}
	delivery := store.Delivery{Kind: "start", CommandID: "44444444-4444-4444-8444-444444444444", Run: store.Run{
		ID: "33333333-3333-4333-8333-333333333333", WorkflowID: "forge-run/33333333-3333-4333-8333-333333333333",
		WorkloadID: "11111111-1111-4111-8111-111111111111", Owner: store.Owner{Issuer: "https://issuer.example", Subject: "owner-a"},
	}}
	receipt, err := backend.Deliver(context.Background(), delivery)
	if err != nil || receipt.WorkflowID != delivery.Run.WorkflowID || fake.starts != 1 {
		t.Fatal(err, receipt, fake.starts)
	}
	if _, err = backend.Deliver(context.Background(), delivery); err != nil || fake.starts != 1 {
		t.Fatal(err, fake.starts)
	}
	delivery.Kind = "cancel"
	if _, err = backend.Deliver(context.Background(), delivery); err != nil || fake.cancels != 1 {
		t.Fatal(err, fake.cancels)
	}
	delivery.Run.WorkflowID = "other"
	if _, err = backend.Deliver(context.Background(), delivery); !errors.Is(err, ErrIdentity) {
		t.Fatal(err)
	}
	delivery.Run.WorkflowID = delivery.Run.ID
	delivery.Run.WorkflowID = "forge-run/" + delivery.Run.ID
	delivery.Kind = "cancel"
	fake.status = StatusAbsent
	if _, err = backend.Deliver(context.Background(), delivery); !errors.Is(err, ErrAbsent) || errors.Is(err, reconcile.ErrRetryable) {
		t.Fatal(err)
	}
	delivery.Kind = "start"
	delivery.Run.Input = []byte(`{"question":"Which service owns execution?","document_ids":["not-approved"]}`)
	delivery.Spec = []byte(`{"corpus_snapshot_id":"` + strings.Repeat("a", 64) + `","permissions":{"document_ids":["platform-brief"]},"limits":{"max_passages":8}}`)
	fake.starts = 0
	if _, err = backend.Deliver(context.Background(), delivery); !errors.Is(err, ErrIdentity) || fake.starts != 0 {
		t.Fatal(err, fake.starts)
	}
}
