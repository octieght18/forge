package reconcile

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/store"
)

type memoryQueue struct {
	x           *store.Delivery
	ack, failed int
	code        string
	ackToken    string
}

func (q *memoryQueue) Claim(context.Context, time.Duration) (*store.Delivery, error) { return q.x, nil }
func (q *memoryQueue) Acknowledge(_ context.Context, x store.Delivery) error {
	q.ack++
	q.ackToken = x.Token
	return nil
}
func (q *memoryQueue) Fail(_ context.Context, _ store.Delivery, code string) error {
	q.failed++
	q.code = code
	return nil
}

func envelope(t *testing.T) store.Delivery {
	t.Helper()
	input, _ := contract.Examples.ReadFile("examples/create-run.json")
	version, _ := contract.Examples.ReadFile("examples/create-version.json")
	var v struct {
		Spec json.RawMessage `json:"spec"`
	}
	_ = json.Unmarshal(version, &v)
	validator, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	hash, err := validator.VersionFingerprint(version)
	if err != nil {
		t.Fatal(err)
	}
	return store.Delivery{CommandID: "44444444-4444-4444-8444-444444444444", Token: "55555555-5555-4555-8555-555555555555", Kind: "start", Attempt: 1, LeaseUntil: time.Now().Add(30 * time.Second),
		Run: store.Run{ID: "33333333-3333-4333-8333-333333333333", WorkloadID: "11111111-1111-4111-8111-111111111111", VersionID: "22222222-2222-4222-8222-222222222222", Owner: store.Owner{Issuer: "https://identity.example/realms/forge", Subject: "fixture"}, WorkflowID: "forge-run/33333333-3333-4333-8333-333333333333", Input: input}, Spec: v.Spec, SpecSHA256: hash}
}

func TestInvalidEnvelopeNeverReachesBackend(t *testing.T) {
	changes := map[string]func(*store.Delivery){
		"workflow identity":  func(x *store.Delivery) { x.Run.WorkflowID = "other" },
		"invalid owner":      func(x *store.Delivery) { x.Run.Owner.Issuer = "not-an-issuer" },
		"mismatched version": func(x *store.Delivery) { x.Run.VersionID = "99999999-9999-4999-8999-999999999999" },
		"spec mismatch":      func(x *store.Delivery) { x.SpecSHA256 = "wrong" },
		"extra tool":         func(x *store.Delivery) { x.Spec = []byte(`{"workflow_type":"shell"}`) },
		"unapproved source": func(x *store.Delivery) {
			var input map[string]any
			_ = json.Unmarshal(x.Run.Input, &input)
			input["document_ids"] = []string{"other-owner-document"}
			x.Run.Input, _ = json.Marshal(input)
		},
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			x := envelope(t)
			change(&x)
			q := &memoryQueue{x: &x}
			called := false
			r := newReconciler(t, q, backendFunc(func(context.Context, store.Delivery) (Receipt, error) { called = true; return Receipt{}, nil }))
			if handled, err := r.Step(context.Background()); err != nil || !handled || called || q.failed != 1 || q.code != "invalid_payload" || q.ack != 0 {
				t.Fatal("invalid envelope delivered", err)
			}
		})
	}
}

func TestBackendCannotMutateAcknowledgementIdentity(t *testing.T) {
	x := envelope(t)
	q := &memoryQueue{x: &x}
	r := newReconciler(t, q, backendFunc(func(_ context.Context, request store.Delivery) (Receipt, error) {
		rc := receipt(request)
		request.Token = "replacement"
		request.Run.Input[0] = 'x'
		request.Spec[0] = 'x'
		return rc, nil
	}))
	if _, err := r.Step(context.Background()); err != nil || q.ack != 1 || q.ackToken != x.Token || x.Run.Input[0] != '{' || x.Spec[0] != '{' {
		t.Fatal("backend corrupted stored envelope", err)
	}
}

func TestShutdownLeavesLeaseAndDeadlineRetries(t *testing.T) {
	for _, shutdown := range []bool{true, false} {
		t.Run(map[bool]string{true: "shutdown", false: "deadline"}[shutdown], func(t *testing.T) {
			x := envelope(t)
			q := &memoryQueue{x: &x}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			options := Defaults()
			options.OperationTimeout = 15 * time.Millisecond
			r, err := New(q, backendFunc(func(delivery context.Context, x store.Delivery) (Receipt, error) {
				if shutdown {
					cancel()
				}
				<-delivery.Done()
				return receipt(x), nil
			}), options)
			if err != nil {
				t.Fatal(err)
			}
			_, err = r.Step(ctx)
			if shutdown {
				if !errors.Is(err, context.Canceled) || q.failed != 0 || q.ack != 0 {
					t.Fatal("shutdown rewrote or acknowledged intent", err)
				}
			} else if err != nil || q.failed != 1 || q.code != "deadline" || q.ack != 0 {
				t.Fatal("deadline falsely acknowledged", err)
			}
		})
	}
}

func TestLoopCancellationAndConfiguration(t *testing.T) {
	q := &memoryQueue{}
	b := backendFunc(func(context.Context, store.Delivery) (Receipt, error) {
		t.Fatal("idle loop invoked backend")
		return Receipt{}, nil
	})
	for _, options := range []Options{{}, {time.Second, 10 * time.Second, 5 * time.Second}, {time.Second, 30 * time.Second, time.Hour}} {
		if _, err := New(q, b, options); err == nil {
			t.Fatal("unsafe deadline configuration accepted")
		}
	}
	if _, err := New(nil, b, Defaults()); err == nil {
		t.Fatal("missing queue accepted")
	}
	if _, err := New(q, nil, Defaults()); err == nil {
		t.Fatal("missing backend accepted")
	}
	r := newReconciler(t, q, b)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("loop ignored shutdown")
	}
}
