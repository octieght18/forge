package environment

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/provision"
	"github.com/octieght18/forge/internal/telemetry"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestFailureRecoveryConvergesWithoutDuplicates(t *testing.T) {
	telemetry.Reset()
	r, e, i := fixture(t)
	lookup := r.Lookup.(*lookup)
	lookup.err = errors.New("product api unavailable")
	pass(t, r, e)
	pass(t, r, e)
	if reason(t, r, e) != "IdentityUnavailable" {
		t.Fatal(reason(t, r, e))
	}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: Namespace(i)}, &core.Namespace{}); err == nil {
		t.Fatal("namespace created during the API outage")
	}
	now := time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC)
	failed, err := provision.Accept(i.WorkloadID, i.WorkloadID, provision.ActionProvision, 1, false, now, provision.DefaultTimeout)
	if err != nil {
		t.Fatal(err)
	}
	failed.ID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	failed = provision.Advance(failed, &provision.Observation{Phase: "Failed", Reason: "IdentityUnavailable"}, now)
	if failed.Status != provision.Failed || failed.ErrorCode != "controller_failed" || failed.Terminal() != true {
		t.Fatalf("%#v", failed)
	}

	lookup.err = nil
	// A one-millisecond lookup keeps the Ready sample above a clock that
	// reports a faster pass as zero. Production lookup does not wait.
	lookup.delay = time.Millisecond
	started := time.Now()
	pass(t, r, e)
	for _, object := range []client.Object{
		&core.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "boundary", Namespace: Namespace(i)}},
		&core.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "containers", Namespace: Namespace(i)}},
	} {
		if err = r.Client.Delete(context.Background(), object); err != nil {
			t.Fatal(err)
		}
	}
	restarted := &Reconciler{Client: r.Client, Lookup: r.Lookup}
	pass(t, restarted, e)
	namespace := &core.Namespace{}
	if err = restarted.Client.Get(context.Background(), types.NamespacedName{Name: Namespace(i)}, namespace); err != nil {
		t.Fatal(err)
	}
	namespace.Status.Phase = core.NamespaceActive
	if err = restarted.Client.Status().Update(context.Background(), namespace); err != nil {
		t.Fatal(err)
	}
	disabled := false
	if err = restarted.Client.Create(context.Background(), &core.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: namespace.Name}, AutomountServiceAccountToken: &disabled}); err != nil {
		t.Fatal(err)
	}
	pass(t, restarted, e)
	recovered := time.Since(started)
	if reason(t, restarted, e) != "BoundaryReady" || recovered <= 0 || recovered > 5*time.Second {
		t.Fatalf("reason %s recovery %s", reason(t, restarted, e), recovered)
	}
	namespaceUID := namespace.UID
	assertSingleOwnedBoundary(t, restarted, e, i)

	before := current(t, restarted, e).GetResourceVersion()
	pass(t, restarted, e)
	if current(t, restarted, e).GetResourceVersion() != before {
		t.Fatal("stable pass rewrote status")
	}
	if err = restarted.Client.Get(context.Background(), types.NamespacedName{Name: Namespace(i)}, namespace); err != nil {
		t.Fatal(err)
	}
	if namespace.UID != namespaceUID || namespace.Status.Phase != core.NamespaceActive {
		t.Fatalf("namespace changed after recovery %#v", namespace)
	}
	assertSingleOwnedBoundary(t, restarted, e, i)

	late := provision.Advance(failed, &provision.Observation{Phase: "Ready", Reason: "BoundaryReady"}, now.Add(time.Minute))
	if late.Status != provision.Failed {
		t.Fatal("failed operation reopened after recovery")
	}
	next, err := provision.Accept(i.WorkloadID, i.WorkloadID, provision.ActionProvision, 2, false, now, provision.DefaultTimeout)
	if err != nil {
		t.Fatal(err)
	}
	next = provision.Advance(next, &provision.Observation{Phase: "Provisioning", Reason: "Reconciling"}, now)
	next = provision.Advance(next, &provision.Observation{Phase: "Ready", Reason: "BoundaryReady"}, now)
	if next.Status != provision.Ready || next.ObservedGeneration != next.DesiredGeneration {
		t.Fatalf("%#v", next)
	}
	view := telemetry.Snapshot()
	if view.Reconciliation.Outcomes["failure"] < 1 || view.Reconciliation.Outcomes["success"] < 1 {
		t.Fatalf("%#v", view.Reconciliation.Outcomes)
	}
	if view.Reconciliation.FailureReasons["IdentityUnavailable"] < 1 {
		t.Fatalf("%#v", view.Reconciliation.FailureReasons)
	}
	if view.Reconciliation.Duration["success"].Count < 1 {
		t.Fatalf("%#v", view.Reconciliation.Duration)
	}
	t.Logf("recovery %s failure %d success %d", recovered, view.Reconciliation.Outcomes["failure"], view.Reconciliation.Outcomes["success"])
}

func assertSingleOwnedBoundary(t *testing.T, r *Reconciler, e *unstructured.Unstructured, i Identity) {
	t.Helper()
	environmentUID := current(t, r, e).GetUID()
	var namespaces core.NamespaceList
	if err := r.Client.List(context.Background(), &namespaces); err != nil {
		t.Fatal(err)
	}
	ownedNamespaces := 0
	for _, namespace := range namespaces.Items {
		if namespace.Labels[Workload] == i.WorkloadID {
			ownedNamespaces++
			if !controlled(&namespace, current(t, r, e), i) {
				t.Fatal("namespace ownership")
			}
		}
	}
	if ownedNamespaces != 1 {
		t.Fatal(ownedNamespaces)
	}
	quota := &core.ResourceQuotaList{}
	limits := &core.LimitRangeList{}
	accounts := &core.ServiceAccountList{}
	if err := r.Client.List(context.Background(), quota, client.InNamespace(Namespace(i))); err != nil {
		t.Fatal(err)
	}
	if err := r.Client.List(context.Background(), limits, client.InNamespace(Namespace(i))); err != nil {
		t.Fatal(err)
	}
	if err := r.Client.List(context.Background(), accounts, client.InNamespace(Namespace(i))); err != nil {
		t.Fatal(err)
	}
	if len(quota.Items) != 1 || quota.Items[0].Name != "boundary" || !controlled(&quota.Items[0], current(t, r, e), i) {
		t.Fatalf("quota %#v", quota.Items)
	}
	if len(limits.Items) != 1 || limits.Items[0].Name != "containers" || !controlled(&limits.Items[0], current(t, r, e), i) {
		t.Fatalf("limits %#v", limits.Items)
	}
	workers := 0
	for _, account := range accounts.Items {
		if account.Name != "worker" {
			continue
		}
		workers++
		if account.AutomountServiceAccountToken == nil || *account.AutomountServiceAccountToken || !controlled(&account, current(t, r, e), i) {
			t.Fatalf("worker %#v", account)
		}
		if metav1.GetControllerOf(&account).UID != environmentUID {
			t.Fatal("worker owner")
		}
	}
	if workers != 1 {
		t.Fatal(workers)
	}
}
