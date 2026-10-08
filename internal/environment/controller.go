// Package environment reconciles operator-owned execution boundaries, not runs.
package environment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	core "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Group     = "platform.forge.local"
	Version   = "v1alpha1"
	Finalizer = Group + "/environment-cleanup"
	Managed   = Group + "/managed"
	Workload  = Group + "/workload"
)

var GVK = schema.GroupVersionKind{Group: Group, Version: Version, Kind: "ForgeEnvironment"}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

type Identity struct{ WorkloadID, Issuer, Subject string }
type Lookup interface {
	Verify(context.Context, Identity) (bool, error)
}
type Database struct{ Pool *pgxpool.Pool }

func (d Database) Verify(ctx context.Context, i Identity) (bool, error) {
	var match bool
	err := d.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT FROM forge.workloads w JOIN forge.principals p ON p.id=w.owner_id WHERE w.id=$1 AND p.issuer=$2 AND p.subject=$3)`, i.WorkloadID, i.Issuer, i.Subject).Scan(&match)
	return match, err
}

func Object() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(GVK)
	return u
}

func Decode(e *unstructured.Unstructured) (Identity, error) {
	i := Identity{}
	i.WorkloadID, _, _ = unstructured.NestedString(e.Object, "spec", "workloadID")
	i.Issuer, _, _ = unstructured.NestedString(e.Object, "spec", "owner", "issuer")
	i.Subject, _, _ = unstructured.NestedString(e.Object, "spec", "owner", "subject")
	profile, _, _ := unstructured.NestedString(e.Object, "spec", "profile")
	if !uuid.MatchString(i.WorkloadID) || e.GetName() != "workload-"+i.WorkloadID || e.GetNamespace() != "" || i.Issuer == "" || len(i.Issuer) > 2048 || i.Subject == "" || len(i.Subject) > 1020 || profile != "small-v1" || e.GetUID() == "" {
		return i, errors.New("invalid environment identity")
	}
	return i, nil
}

func Namespace(i Identity) string { return "forge-w-" + i.WorkloadID }
func owner(e *unstructured.Unstructured) metav1.OwnerReference {
	return *metav1.NewControllerRef(e, GVK)
}
func labels(i Identity) map[string]string {
	h := sha256.Sum256([]byte(i.Issuer + "\x00" + i.Subject))
	return map[string]string{Managed: "environment", Workload: i.WorkloadID, Group + "/owner": hex.EncodeToString(h[:])[:63]}
}

// Desired contains only boundary resources; no executable image or credentials.
func Desired(e *unstructured.Unstructured, i Identity) []client.Object {
	m := metav1.ObjectMeta{Name: Namespace(i), Labels: labels(i), OwnerReferences: []metav1.OwnerReference{owner(e)}}
	m.Labels["pod-security.kubernetes.io/enforce"] = "restricted"
	m.Labels["pod-security.kubernetes.io/enforce-version"] = "v1.37"
	m.Labels["pod-security.kubernetes.io/warn"] = "restricted"
	m.Labels["pod-security.kubernetes.io/audit"] = "restricted"
	ns := &core.Namespace{ObjectMeta: m}
	child := func(name string) metav1.ObjectMeta {
		return metav1.ObjectMeta{Name: name, Namespace: Namespace(i), Labels: labels(i), OwnerReferences: []metav1.OwnerReference{owner(e)}}
	}
	q := &core.ResourceQuota{ObjectMeta: child("boundary"), Spec: core.ResourceQuotaSpec{Hard: core.ResourceList{}}}
	for k, v := range map[core.ResourceName]string{"pods": "2", "requests.cpu": "1", "requests.memory": "1Gi", "limits.cpu": "2", "limits.memory": "2Gi", "persistentvolumeclaims": "0", "services.loadbalancers": "0", "services.nodeports": "0"} {
		q.Spec.Hard[k] = resource.MustParse(v)
	}
	l := &core.LimitRange{ObjectMeta: child("containers"), Spec: core.LimitRangeSpec{Limits: []core.LimitRangeItem{{Type: core.LimitTypeContainer,
		Default:        core.ResourceList{core.ResourceCPU: resource.MustParse("500m"), core.ResourceMemory: resource.MustParse("512Mi")},
		DefaultRequest: core.ResourceList{core.ResourceCPU: resource.MustParse("100m"), core.ResourceMemory: resource.MustParse("64Mi")},
		Max:            core.ResourceList{core.ResourceCPU: resource.MustParse("1"), core.ResourceMemory: resource.MustParse("1Gi")}}}}}
	f := false
	sa := &core.ServiceAccount{ObjectMeta: child("worker"), AutomountServiceAccountToken: &f}
	return []client.Object{ns, q, l, sa}
}

type Reconciler struct {
	Client client.Client
	Lookup Lookup
}

func (r *Reconciler) Setup(m ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(m).For(Object()).Owns(&core.Namespace{}).Owns(&core.ResourceQuota{}).Owns(&core.LimitRange{}).Owns(&core.ServiceAccount{}).Complete(r)
}

// Every pass is bounded; writes use live resourceVersions and deletion uses UID.
func (r *Reconciler) Reconcile(parent context.Context, key ctrl.Request) (ctrl.Result, error) {
	ctx, cancel := context.WithTimeout(parent, 10*time.Second)
	defer cancel()
	e := Object()
	if err := r.Client.Get(ctx, key.NamespacedName, e); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	i, err := Decode(e)
	if err != nil {
		return r.state(ctx, e, "Failed", "InvalidIdentity", "")
	}
	if !e.GetDeletionTimestamp().IsZero() {
		return r.remove(ctx, e, i)
	}
	if !has(e.GetFinalizers(), Finalizer) {
		before := e.DeepCopy()
		e.SetFinalizers(append(e.GetFinalizers(), Finalizer))
		if err := r.Client.Patch(ctx, e, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Millisecond}, nil
	}
	matched, err := r.Lookup.Verify(ctx, i)
	if err != nil {
		return r.state(ctx, e, "Failed", "IdentityUnavailable", "")
	}
	if !matched {
		return r.state(ctx, e, "Failed", "OwnerMismatch", "")
	}
	phase, _, _ := unstructured.NestedString(e.Object, "status", "phase")
	if phase == "" || phase == "Pending" {
		if err := r.setState(ctx, e, "Provisioning", "Reconciling", ""); err != nil {
			return ctrl.Result{}, err
		}
	}
	for _, desired := range Desired(e, i) {
		if err := r.ensure(ctx, e, i, desired); err != nil {
			reason := "ResourceUnavailable"
			if errors.Is(err, errOwnership) {
				reason = "OwnershipConflict"
			}
			return r.state(ctx, e, "Failed", reason, "")
		}
	}
	ns := &core.Namespace{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: Namespace(i)}, ns); err != nil {
		return ctrl.Result{}, err
	}
	if ns.Status.Phase != core.NamespaceActive || !ns.DeletionTimestamp.IsZero() {
		return r.state(ctx, e, "Provisioning", "NamespacePending", string(ns.UID))
	}
	// Kubernetes creates the default account. Disable its implicit token mount too;
	// it remains Kubernetes-owned rather than being adopted by Forge.
	def := &core.ServiceAccount{}
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: ns.Name, Name: "default"}, def); err != nil {
		return r.state(ctx, e, "Provisioning", "DefaultAccountPending", string(ns.UID))
	}
	if def.AutomountServiceAccountToken == nil || *def.AutomountServiceAccountToken {
		before := def.DeepCopy()
		f := false
		def.AutomountServiceAccountToken = &f
		if err := r.Client.Patch(ctx, def, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{})); err != nil {
			return ctrl.Result{}, err
		}
	}
	return r.state(ctx, e, "Ready", "BoundaryReady", string(ns.UID))
}

var errOwnership = errors.New("ownership conflict")

func controlled(o client.Object, e *unstructured.Unstructured, i Identity) bool {
	ref := metav1.GetControllerOf(o)
	return ref != nil && ref.UID == e.GetUID() && ref.Kind == GVK.Kind && ref.APIVersion == GVK.GroupVersion().String() && ref.Name == e.GetName() && o.GetLabels()[Managed] == "environment" && o.GetLabels()[Workload] == i.WorkloadID
}

func (r *Reconciler) ensure(ctx context.Context, e *unstructured.Unstructured, i Identity, want client.Object) error {
	current := want.DeepCopyObject().(client.Object)
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(want), current)
	if apierrors.IsNotFound(err) {
		return r.Client.Create(ctx, want)
	}
	if err != nil {
		return err
	}
	if !controlled(current, e, i) || !current.GetDeletionTimestamp().IsZero() {
		return errOwnership
	}
	before := current.DeepCopyObject().(client.Object)
	// Preserve unrelated metadata and never take over foreign owner references.
	labs := current.GetLabels()
	if labs == nil {
		labs = map[string]string{}
	}
	for k, v := range want.GetLabels() {
		labs[k] = v
	}
	current.SetLabels(labs)
	switch c := current.(type) {
	case *core.ResourceQuota:
		c.Spec = want.(*core.ResourceQuota).Spec
	case *core.LimitRange:
		c.Spec = want.(*core.LimitRange).Spec
	case *core.ServiceAccount:
		c.AutomountServiceAccountToken = want.(*core.ServiceAccount).AutomountServiceAccountToken
	}
	if reflect.DeepEqual(before, current) {
		return nil
	}
	return r.Client.Patch(ctx, current, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}

func (r *Reconciler) remove(ctx context.Context, e *unstructured.Unstructured, i Identity) (ctrl.Result, error) {
	if !has(e.GetFinalizers(), Finalizer) {
		return ctrl.Result{}, nil
	}
	ns := &core.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: Namespace(i)}, ns)
	if apierrors.IsNotFound(err) {
		before := e.DeepCopy()
		finals := []string{}
		for _, f := range e.GetFinalizers() {
			if f != Finalizer {
				finals = append(finals, f)
			}
		}
		e.SetFinalizers(finals)
		return ctrl.Result{}, r.Client.Patch(ctx, e, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if !controlled(ns, e, i) {
		return r.state(ctx, e, "Deleting", "OwnershipConflict", string(ns.UID))
	}
	claims := &core.PersistentVolumeClaimList{}
	if err := r.Client.List(ctx, claims, client.InNamespace(ns.Name)); err != nil {
		return ctrl.Result{}, err
	}
	volumes := &core.PersistentVolumeList{}
	if err := r.Client.List(ctx, volumes); err != nil {
		return ctrl.Result{}, err
	}
	if len(claims.Items) > 0 {
		return r.state(ctx, e, "Deleting", "PersistentStoragePresent", string(ns.UID))
	}
	for _, v := range volumes.Items {
		if v.Spec.ClaimRef != nil && v.Spec.ClaimRef.Namespace == ns.Name {
			return r.state(ctx, e, "Deleting", "PersistentStoragePresent", string(ns.UID))
		}
	}
	if ns.DeletionTimestamp.IsZero() {
		uid := ns.UID
		rv := ns.ResourceVersion
		if err := r.Client.Delete(ctx, ns, &client.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &rv}}); err != nil {
			return ctrl.Result{}, err
		}
	}
	return r.state(ctx, e, "Deleting", "CleanupPending", string(ns.UID))
}

func (r *Reconciler) state(ctx context.Context, e *unstructured.Unstructured, phase, reason, uid string) (ctrl.Result, error) {
	return ctrl.Result{RequeueAfter: 15 * time.Second}, r.setState(ctx, e, phase, reason, uid)
}
func (r *Reconciler) setState(ctx context.Context, e *unstructured.Unstructured, phase, reason, uid string) error {
	before := e.DeepCopy()
	conditions := []metav1.Condition{}
	if raw, found, _ := unstructured.NestedSlice(e.Object, "status", "conditions"); found {
		for _, v := range raw {
			m, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			var c metav1.Condition
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, &c); err == nil {
				conditions = append(conditions, c)
			}
		}
	}
	status := metav1.ConditionFalse
	if phase == "Ready" {
		status = metav1.ConditionTrue
	}
	meta.SetStatusCondition(&conditions, metav1.Condition{Type: "Ready", Status: status, Reason: reason, Message: fmt.Sprintf("Environment boundary: %s", reason), ObservedGeneration: e.GetGeneration()})
	arr := []interface{}{}
	for _, c := range conditions {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&c)
		if err != nil {
			return err
		}
		arr = append(arr, m)
	}
	state := map[string]interface{}{"phase": phase, "observedGeneration": e.GetGeneration(), "conditions": arr}
	if uid != "" {
		state["namespace"] = map[string]interface{}{"name": Namespace(Identity{WorkloadID: specID(e)}), "uid": uid}
	}
	e.Object["status"] = state
	if reflect.DeepEqual(before.Object["status"], state) {
		return nil
	}
	return r.Client.Status().Patch(ctx, e, client.MergeFromWithOptions(before, client.MergeFromWithOptimisticLock{}))
}
func specID(e *unstructured.Unstructured) string {
	s, _, _ := unstructured.NestedString(e.Object, "spec", "workloadID")
	return s
}
func has(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
