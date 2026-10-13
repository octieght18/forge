package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandRemovesSyntheticFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "demo-synthetic-note.txt")
	if err := os.WriteFile(path, []byte("synthetic"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"--dir", dir, "--confirm", "delete-synthetic-demo-records"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestCommandRefusesRetainedDatabase(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "forge-kubernetes", "data", "postgres")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "PG_VERSION"), []byte("18"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"--dir", dir, "--confirm", "delete-synthetic-demo-records"})
	if err == nil || !strings.Contains(err.Error(), "retained product state") {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "PG_VERSION")); statErr != nil {
		t.Fatal(statErr)
	}
}
