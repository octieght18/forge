package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const unsafePod = `
apiVersion: v1
kind: Pod
metadata:
  name: unsafe
spec:
  hostNetwork: true
  containers:
    - name: app
      image: example
      securityContext:
        privileged: true
        allowPrivilegeEscalation: true
        capabilities:
          add: ["SYS_ADMIN"]
      ports:
        - containerPort: 80
          hostPort: 80
  volumes:
    - name: host
      hostPath:
        path: /var/run
`

const safePod = `
apiVersion: v1
kind: Pod
metadata:
  name: safe
spec:
  hostNetwork: false
  containers:
    - name: app
      image: example
      securityContext:
        runAsNonRoot: true
        allowPrivilegeEscalation: false
        privileged: false
        capabilities:
          drop: ["ALL"]
`

func TestUnsafePodIsRejectedAndSafePodIsNot(t *testing.T) {
	found, err := CheckYAML([]byte(unsafePod))
	if err != nil {
		t.Fatal(err)
	}
	reasons := map[string]bool{}
	for _, item := range found {
		reasons[item.Reason] = true
	}
	for _, reason := range []string{"hostNetwork", "privileged", "allowPrivilegeEscalation", "added capability", "hostPath", "hostPort", "runAsNonRoot", "capabilities drop ALL"} {
		if !reasons[reason] {
			t.Fatal(found)
		}
	}
	safe, err := CheckYAML([]byte(safePod))
	if err != nil || len(safe) != 0 {
		t.Fatal(err, safe)
	}
}

func TestRepositoryManifestsAndNetworkAssumptions(t *testing.T) {
	root := filepath.Join("..", "..", "deploy")
	found, err := CheckTree(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 0 {
		t.Fatal(found)
	}
	kyverno, err := os.ReadFile(filepath.Join(root, "policy", "kyverno-privileged.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(kyverno)
	if !strings.Contains(text, "validationFailureAction: Enforce") || !strings.Contains(text, "privileged") {
		t.Fatal("kyverno policy missing enforce rule")
	}
	network, err := os.ReadFile(filepath.Join(root, "policy", "network-default-deny.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	intent, err := NetworkIntent(network)
	if err != nil {
		t.Fatal(err)
	}
	if !intent.DefaultDeny || !intent.SameNamespace || !intent.DNS || !intent.ModelTCP443 {
		t.Fatalf("%+v", intent)
	}
	if intent.ToolNetwork || intent.Enforced || intent.DeniedObserved {
		t.Fatalf("network enforcement was not observed: %+v", intent)
	}
	if Admission == "" || CNI == "" || !strings.Contains(CNI, "does not enforce NetworkPolicy") {
		t.Fatal(Admission, CNI)
	}
}
