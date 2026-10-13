package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandRefusesCloud(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "local"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "local", "release.txt"), []byte("local-overlay\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"--overlays", root, "--from", "local", "--to", "eks", "--confirm", "promote-local-to-staging"})
	if err == nil || !strings.Contains(err.Error(), "selected cloud environment was not created") {
		t.Fatal(err)
	}
}
