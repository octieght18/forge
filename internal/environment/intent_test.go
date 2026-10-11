package environment

import (
	"context"
	"errors"
	"testing"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSubmitIsIdempotentAndRemoveIsRepeatable(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := core.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	ctx := context.Background()
	const id = "12345678-1234-4234-8234-123456789abc"
	if err := Submit(ctx, c, id, "https://issuer.example/realms/forge", "owner-a"); err != nil {
		t.Fatal(err)
	}
	first := Object()
	if err := c.Get(ctx, client.ObjectKey{Name: "workload-" + id}, first); err != nil {
		t.Fatal(err)
	}
	if err := Submit(ctx, c, id, "https://issuer.example/realms/forge", "owner-a"); err != nil {
		t.Fatal(err)
	}
	second := Object()
	if err := c.Get(ctx, client.ObjectKey{Name: "workload-" + id}, second); err != nil {
		t.Fatal(err)
	}
	if first.GetResourceVersion() != "" && second.GetResourceVersion() != first.GetResourceVersion() {
		t.Fatalf("repeat submit changed the environment %s %s", first.GetResourceVersion(), second.GetResourceVersion())
	}
	listed := &unstructured.UnstructuredList{}
	listed.SetGroupVersionKind(GVK.GroupVersion().WithKind("ForgeEnvironmentList"))
	if err := c.List(ctx, listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatal(len(listed.Items))
	}
	if err := Submit(ctx, c, id, "https://issuer.example/realms/forge", "owner-b"); !errors.Is(err, errDifferentOwner) {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKey{Name: "workload-" + id}, second); err != nil {
		t.Fatal(err)
	}
	subject, _, _ := unstructured.NestedString(second.Object, "spec", "owner", "subject")
	if subject != "owner-a" {
		t.Fatal(subject)
	}
	requests := []Request{{WorkloadID: id, Action: "provision", Issuer: "https://issuer.example/realms/forge", Subject: "owner-a"}}
	if err := Sync(ctx, c, requests); err != nil {
		t.Fatal(err)
	}
	if err := Sync(ctx, c, []Request{{WorkloadID: id, Action: "delete"}}); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(ctx, client.ObjectKey{Name: "workload-" + id}, second); err == nil {
		t.Fatal("environment still present")
	}
	if err := Remove(ctx, c, id); err != nil {
		t.Fatal(err)
	}
}
