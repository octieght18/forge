package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRollbackRestoresPinAndKeepsExecutions(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "release.txt"), []byte("bad-release\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	executions := []string{"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"}
	started := time.Now()
	result, err := Rollback(RollbackRequest{
		Root:       root,
		Previous:   "local-overlay",
		Watch:      Watch{Requests: 100, Errors: 40, Latency: 800 * time.Millisecond},
		MaxLatency: 200 * time.Millisecond,
		Executions: executions,
	})
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if result.Restored != "local-overlay" || result.Human != "" {
		t.Fatal(result)
	}
	if len(result.Executions) != 2 || result.Executions[0] != executions[0] || result.Executions[1] != executions[1] {
		t.Fatal(result.Executions)
	}
	body, err := os.ReadFile(filepath.Join(staging, "release.txt"))
	if err != nil || strings.TrimSpace(string(body)) != "local-overlay" {
		t.Fatal(err, string(body))
	}
	if result.Detection < 0 || result.Rollback < 0 || elapsed < result.Rollback {
		t.Fatal(result.Detection, result.Rollback, elapsed)
	}
	t.Logf("detection=%s rollback=%s elapsed=%s", result.Detection, result.Rollback, elapsed)
	executions[0] = "changed-by-caller"
	if result.Executions[0] == "changed-by-caller" {
		t.Fatal("execution list was shared")
	}
}

func TestRollbackStopsForHumanGates(t *testing.T) {
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	pin := filepath.Join(staging, "release.txt")
	if err := os.WriteFile(pin, []byte("bad-release\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	watch := Watch{Requests: 50, Errors: 25, Latency: time.Second}
	for _, gate := range []RollbackRequest{
		{Root: root, Previous: "local-overlay", Watch: watch, MaxLatency: time.Millisecond, MigrationApplied: true, Executions: []string{"33333333-3333-4333-8333-333333333333"}},
		{Root: root, Previous: "local-overlay", Watch: watch, MaxLatency: time.Millisecond, ModelCallCompleted: true, Executions: []string{"33333333-3333-4333-8333-333333333333"}},
		{Root: root, Previous: "", Watch: watch, MaxLatency: time.Millisecond, Executions: []string{"33333333-3333-4333-8333-333333333333"}},
	} {
		result, err := Rollback(gate)
		if err != nil || result.Human == "" || result.Restored != "" {
			t.Fatal(result, err)
		}
		if len(result.Executions) != 1 {
			t.Fatal(result.Executions)
		}
		body, err := os.ReadFile(pin)
		if err != nil || strings.TrimSpace(string(body)) != "bad-release" {
			t.Fatal(err, string(body))
		}
	}
}
