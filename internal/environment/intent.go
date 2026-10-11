package environment

import (
	"context"
	"errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Request is one open provisioning operation the controller can see.
// The database role remains read-only, so this record cannot be updated here.
type Request struct {
	WorkloadID string
	Action     string
	Issuer     string
	Subject    string
}

var errDifferentOwner = errors.New("environment owner differs")

// Intent is the cluster object a developer provision request asks for.
func Intent(workloadID, issuer, subject string) *unstructured.Unstructured {
	e := Object()
	e.SetName("workload-" + workloadID)
	e.Object["spec"] = map[string]any{
		"workloadID": workloadID,
		"owner":      map[string]any{"issuer": issuer, "subject": subject},
		"profile":    "small-v1",
	}
	return e
}

// Submit creates the environment when it is absent. A repeat with the same
// owner and profile leaves the existing object in place.
func Submit(ctx context.Context, c client.Client, workloadID, issuer, subject string) error {
	desired := Intent(workloadID, issuer, subject)
	current := Object()
	err := c.Get(ctx, client.ObjectKeyFromObject(desired), current)
	if apierrors.IsNotFound(err) {
		return c.Create(ctx, desired)
	}
	if err != nil {
		return err
	}
	if !current.GetDeletionTimestamp().IsZero() {
		return nil
	}
	gotID, _, _ := unstructured.NestedString(current.Object, "spec", "workloadID")
	gotIssuer, _, _ := unstructured.NestedString(current.Object, "spec", "owner", "issuer")
	gotSubject, _, _ := unstructured.NestedString(current.Object, "spec", "owner", "subject")
	gotProfile, _, _ := unstructured.NestedString(current.Object, "spec", "profile")
	if gotID != workloadID || gotIssuer != issuer || gotSubject != subject || gotProfile != "small-v1" {
		return errDifferentOwner
	}
	return nil
}

// Remove asks the controller to delete the environment. A missing object is already removed.
func Remove(ctx context.Context, c client.Client, workloadID string) error {
	current := Intent(workloadID, "https://issuer.example", "pending")
	err := c.Delete(ctx, current)
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// Sync applies every open request once. Provision submits intent. Delete removes it.
func Sync(ctx context.Context, c client.Client, requests []Request) error {
	var joined error
	for _, request := range requests {
		var err error
		if request.Action == "delete" {
			err = Remove(ctx, c, request.WorkloadID)
		} else if request.Action == "provision" {
			err = Submit(ctx, c, request.WorkloadID, request.Issuer, request.Subject)
		}
		joined = errors.Join(joined, err)
	}
	return joined
}
