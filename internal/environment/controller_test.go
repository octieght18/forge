package environment

import (
	"context"
	"errors"
	"testing"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

type lookup struct {
	match bool
	err   error
	calls int
}

func (l *lookup) Verify(_ context.Context, _ Identity) (bool, error) {
	l.calls++
	return l.match, l.err
}
func fixture(t *testing.T) (*Reconciler, *unstructured.Unstructured, Identity) {
	t.Helper()
	e := Object()
	i := Identity{"12345678-1234-4234-8234-123456789abc", "http://issuer.local/realms/forge", "subject-a"}
	e.SetName("workload-" + i.WorkloadID)
	e.SetUID(types.UID("environment-uid"))
	e.SetGeneration(1)
	e.Object["spec"] = map[string]interface{}{"workloadID": i.WorkloadID, "owner": map[string]interface{}{"issuer": i.Issuer, "subject": i.Subject}, "profile": "small-v1"}
	s := runtime.NewScheme()
	if err := core.AddToScheme(s); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(e).WithStatusSubresource(e).Build()
	return &Reconciler{Client: c, Lookup: &lookup{match: true}}, e, i
}
func pass(t *testing.T, r *Reconciler, e *unstructured.Unstructured) {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(e)}); err != nil {
		t.Fatal(err)
	}
}
func current(t *testing.T, r *Reconciler, e *unstructured.Unstructured) *unstructured.Unstructured {
	t.Helper()
	got := Object()
	if err := r.Client.Get(context.Background(), client.ObjectKeyFromObject(e), got); err != nil {
		t.Fatal(err)
	}
	return got
}
func reason(t *testing.T, r *Reconciler, e *unstructured.Unstructured) string {
	t.Helper()
	cs, _, _ := unstructured.NestedSlice(current(t, r, e).Object, "status", "conditions")
	if len(cs) == 0 {
		return ""
	}
	return cs[0].(map[string]interface{})["reason"].(string)
}
func ready(t *testing.T, r *Reconciler, e *unstructured.Unstructured, i Identity) {
	t.Helper()
	pass(t, r, e)
	pass(t, r, e)
	n := &core.Namespace{}
	if err := r.Client.Get(context.Background(), types.NamespacedName{Name: Namespace(i)}, n); err != nil {
		t.Fatal(err)
	}
	n.Status.Phase = core.NamespaceActive
	if err := r.Client.Status().Update(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	f := false
	if err := r.Client.Create(context.Background(), &core.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: n.Name}, AutomountServiceAccountToken: &f}); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if reason(t, r, e) != "BoundaryReady" {
		t.Fatal(reason(t, r, e))
	}
}

func TestOwnerMismatchAndOutageDoNotProvision(t *testing.T) {
	for _, outage := range []bool{false, true} {
		t.Run(map[bool]string{false: "mismatch", true: "outage"}[outage], func(t *testing.T) {
			r, e, i := fixture(t)
			l := r.Lookup.(*lookup)
			l.match = false
			if outage {
				l.err = errors.New("private credential-bearing error")
			}
			pass(t, r, e)
			pass(t, r, e)
			n := &core.Namespace{}
			if err := r.Client.Get(context.Background(), types.NamespacedName{Name: Namespace(i)}, n); err == nil {
				t.Fatal("unauthorized namespace")
			}
			want := "OwnerMismatch"
			if outage {
				want = "IdentityUnavailable"
			}
			if reason(t, r, e) != want {
				t.Fatal(reason(t, r, e))
			}
			l.match = true
			l.err = nil
			pass(t, r, e)
			if err := r.Client.Get(context.Background(), types.NamespacedName{Name: Namespace(i)}, n); err != nil {
				t.Fatal("did not recover", err)
			}
		})
	}
}
func TestReplayAndDriftRepair(t *testing.T) {
	r, e, i := fixture(t)
	ready(t, r, e, i)
	before := current(t, r, e).GetResourceVersion()
	for range 4 {
		pass(t, r, e)
	}
	if current(t, r, e).GetResourceVersion() != before {
		t.Fatal("stable reconcile rewrites status")
	}
	q := &core.ResourceQuota{}
	key := types.NamespacedName{Name: "boundary", Namespace: Namespace(i)}
	if err := r.Client.Get(context.Background(), key, q); err != nil {
		t.Fatal(err)
	}
	q.Spec.Hard = core.ResourceList{}
	if err := r.Client.Update(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if err := r.Client.Get(context.Background(), key, q); err != nil {
		t.Fatal(err)
	}
	if q.Spec.Hard.Pods().Value() != 2 {
		t.Fatal("quota not repaired")
	}
	sa := &core.ServiceAccount{}
	key.Name = "worker"
	if err := r.Client.Get(context.Background(), key, sa); err != nil {
		t.Fatal(err)
	}
	f := true
	sa.AutomountServiceAccountToken = &f
	if err := r.Client.Update(context.Background(), sa); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if err := r.Client.Get(context.Background(), key, sa); err != nil {
		t.Fatal(err)
	}
	if *sa.AutomountServiceAccountToken {
		t.Fatal("token default not repaired")
	}
}
func TestPartialCreationAndForeignResource(t *testing.T) {
	r, e, i := fixture(t)
	pass(t, r, e)
	for _, o := range Desired(e, i)[:2] {
		if err := r.Client.Create(context.Background(), o); err != nil {
			t.Fatal(err)
		}
	}
	pass(t, r, e)
	sa := &core.ServiceAccount{}
	key := types.NamespacedName{Namespace: Namespace(i), Name: "worker"}
	if err := r.Client.Get(context.Background(), key, sa); err != nil {
		t.Fatal("partial not recovered", err)
	}
	sa.OwnerReferences = nil
	if err := r.Client.Update(context.Background(), sa); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if reason(t, r, e) != "OwnershipConflict" {
		t.Fatal(reason(t, r, e))
	}
	if err := r.Client.Get(context.Background(), key, sa); err != nil {
		t.Fatal(err)
	}
	if len(sa.OwnerReferences) != 0 {
		t.Fatal("foreign resource adopted")
	}
}
func TestForeignNamespaceNeverModified(t *testing.T) {
	r, e, i := fixture(t)
	n := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: Namespace(i), Labels: map[string]string{"foreign": "yes"}}}
	if err := r.Client.Create(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	pass(t, r, e)
	if reason(t, r, e) != "OwnershipConflict" {
		t.Fatal(reason(t, r, e))
	}
	if err := r.Client.Get(context.Background(), client.ObjectKeyFromObject(n), n); err != nil {
		t.Fatal(err)
	}
	if n.Labels["foreign"] != "yes" || len(n.OwnerReferences) > 0 {
		t.Fatal("adopted namespace")
	}
	if err := r.Client.Delete(context.Background(), current(t, r, e)); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if err := r.Client.Get(context.Background(), client.ObjectKeyFromObject(n), n); err != nil {
		t.Fatal("foreign namespace deleted")
	}
}
func TestDeletionBlocksStorageAndPreservesForeignFinalizers(t *testing.T) {
	r, e, i := fixture(t)
	ready(t, r, e, i)
	n := &core.Namespace{}
	key := types.NamespacedName{Name: Namespace(i)}
	if err := r.Client.Get(context.Background(), key, n); err != nil {
		t.Fatal(err)
	}
	n.Finalizers = []string{"test.example/hold"}
	if err := r.Client.Update(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	p := &core.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "retained", Namespace: n.Name}}
	if err := r.Client.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := r.Client.Delete(context.Background(), current(t, r, e)); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if reason(t, r, e) != "PersistentStoragePresent" {
		t.Fatal(reason(t, r, e))
	}
	if err := r.Client.Delete(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if err := r.Client.Get(context.Background(), key, n); err != nil {
		t.Fatal(err)
	}
	if !has(n.Finalizers, "test.example/hold") || n.DeletionTimestamp.IsZero() {
		t.Fatal("foreign finalizer stripped")
	}
	if !has(current(t, r, e).GetFinalizers(), Finalizer) {
		t.Fatal("cleanup released too early")
	}
	// Only the test's finalizer owner releases its own hold.
	n.Finalizers = nil
	if err := r.Client.Update(context.Background(), n); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if err := r.Client.Get(context.Background(), client.ObjectKeyFromObject(e), Object()); err == nil {
		t.Fatal("environment not finalized")
	}
}
func TestPersistentVolumeReferenceBlocksDeletion(t *testing.T) {
	r, e, i := fixture(t)
	ready(t, r, e, i)
	p := &core.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "retained"}, Spec: core.PersistentVolumeSpec{ClaimRef: &core.ObjectReference{Namespace: Namespace(i), Name: "gone"}}}
	if err := r.Client.Create(context.Background(), p); err != nil {
		t.Fatal(err)
	}
	if err := r.Client.Delete(context.Background(), current(t, r, e)); err != nil {
		t.Fatal(err)
	}
	pass(t, r, e)
	if reason(t, r, e) != "PersistentStoragePresent" {
		t.Fatal(reason(t, r, e))
	}
}

func TestConcurrentOwnershipChangeFencesRepair(t *testing.T) {
	r, e, i := fixture(t)
	ready(t, r, e, i)
	ctx := context.Background()
	q := &core.ResourceQuota{}
	key := types.NamespacedName{Namespace: Namespace(i), Name: "boundary"}
	if err := r.Client.Get(ctx, key, q); err != nil {
		t.Fatal(err)
	}
	q.Spec.Hard["pods"] = resource.MustParse("1")
	if err := r.Client.Update(ctx, q); err != nil {
		t.Fatal(err)
	}
	raced := false
	r.Client = interceptor.NewClient(r.Client.(client.WithWatch), interceptor.Funcs{Patch: func(ctx context.Context, c client.WithWatch, o client.Object, p client.Patch, options ...client.PatchOption) error {
		if _, ok := o.(*core.ResourceQuota); ok && !raced {
			raced = true
			live := &core.ResourceQuota{}
			if err := c.Get(ctx, key, live); err != nil {
				return err
			}
			live.OwnerReferences = nil
			if err := c.Update(ctx, live); err != nil {
				return err
			}
		}
		return c.Patch(ctx, o, p, options...)
	}})
	pass(t, r, e)
	if !raced {
		t.Fatal("repair was not exercised")
	}
	if err := r.Client.Get(ctx, key, q); err != nil {
		t.Fatal(err)
	}
	if len(q.OwnerReferences) != 0 || q.Spec.Hard.Pods().Value() != 1 {
		t.Fatal("stale repair overwrote foreign ownership/resource")
	}
	pass(t, r, e)
	if reason(t, r, e) != "OwnershipConflict" {
		t.Fatal(reason(t, r, e))
	}
}
