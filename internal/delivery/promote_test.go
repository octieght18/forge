package delivery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromoteCopiesLocalPinToStaging(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "local", "release.txt"), []byte("local-overlay\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pin, err := Promote(root, "local", "staging", ConfirmPhrase)
	if err != nil || pin != "local-overlay" {
		t.Fatal(err, pin)
	}
	body, err := os.ReadFile(filepath.Join(root, "staging", "release.txt"))
	if err != nil || strings.TrimSpace(string(body)) != "local-overlay" {
		t.Fatal(err, string(body))
	}
}

func TestPromoteRefusesCloudAndLeavesStagingUntouched(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "local", "release.txt"), []byte("local-overlay\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, to := range []string{"cloud", "eks", "gke"} {
		if _, err := Promote(root, "local", to, ConfirmPhrase); err == nil || !strings.Contains(err.Error(), "selected cloud environment was not created") {
			t.Fatal(to, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "staging", "release.txt")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if _, err := Promote(root, "local", "staging", "ship it"); err == nil {
		t.Fatal("expected refusal")
	}
}

func TestCanaryCeilingAndRollbackLimits(t *testing.T) {
	if !CanaryAccepts(1000, 10) || CanaryAccepts(1000, 11) || CanaryAccepts(0, 0) {
		t.Fatal("ceiling")
	}
	text := RollbackLimits()
	for _, phrase := range []string{"cannot be unsent", "Temporal history is not rewritten", "not deleted"} {
		if !strings.Contains(text, phrase) {
			t.Fatal(text)
		}
	}
}
