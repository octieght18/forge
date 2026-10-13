package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

func TestGatewayIsLocalAndUnauthenticatedPathStaysOnTheAPI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "deploy", "gateway", "envoy.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{`prefix: "/api/"`, "max_tokens: 200", "type: ClusterIP", "envoy.filters.http.local_ratelimit"} {
		if !strings.Contains(text, want) {
			t.Fatal(want)
		}
	}
	for _, banned := range []string{"PRIVATE KEY", "BEGIN CERTIFICATE", "LoadBalancer", "hostNetwork: true", "type: NodePort"} {
		if strings.Contains(text, banned) {
			t.Fatal(banned)
		}
	}
	found, err := CheckYAML(data)
	if err != nil || len(found) != 0 {
		t.Fatal(err, found)
	}
}

func TestGitOpsPinsExistingDigests(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "deploy", "kubernetes", "images.json"))
	if err != nil {
		t.Fatal(err)
	}
	var images map[string]string
	if err := json.Unmarshal(raw, &images); err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, ref := range images {
		if i := strings.Index(ref, "sha256:"); i >= 0 {
			known[ref[i:]] = true
		}
	}
	root := filepath.Join("..", "..", "deploy", "gitops")
	digest := regexp.MustCompile(`sha256:[a-f0-9]{64}`)
	seen := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.Contains(text, "password") || strings.Contains(text, "BEGIN ") || strings.Contains(strings.ToLower(text), "token") {
			t.Fatalf("credential-shaped text in %s", path)
		}
		for _, item := range digest.FindAllString(text, -1) {
			seen++
			if !known[item] {
				t.Fatalf("digest not in images.json: %s", item)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen < 3 {
		t.Fatal(seen)
	}
	for _, name := range []string{"forge-local.yaml", "forge-staging.yaml"} {
		text, err := os.ReadFile(filepath.Join(root, "applications", name))
		if err != nil {
			t.Fatal(err)
		}
		body := string(text)
		if !strings.Contains(body, "selfHeal: true") || strings.Contains(body, "terraform") {
			t.Fatal(name)
		}
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
