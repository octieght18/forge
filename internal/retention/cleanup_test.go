package retention

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassesAndSecretPlacesStayOutOfGit(t *testing.T) {
	classes := Classes()
	want := []string{"executions", "evidence", "audit", "attachments"}
	if len(classes) != len(want) {
		t.Fatalf("classes %d", len(classes))
	}
	for i, class := range classes {
		if class.Name != want[i] || class.Store == "" || class.Retention == "" || class.Deletion == "" {
			t.Fatalf("class %+v", class)
		}
		if strings.Contains(class.Store, "://") || strings.Contains(strings.ToLower(class.Deletion), "password=") {
			t.Fatal(class)
		}
	}
	places := map[string]string{}
	for _, place := range SecretPlaces() {
		places[place.Name] = place.Place
		if place.Place == "git" || strings.Contains(place.Name, "://") || strings.Contains(place.Name, "password=") {
			t.Fatal(place)
		}
	}
	if places["FORGE_DATABASE_URL"] != "environment" || places["api-db"] != "kubernetes-secret" || places["secrets.json"] != "local-file" {
		t.Fatal(places)
	}
}

func TestCleanupRemovesOnlySyntheticFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "demo-synthetic-note.txt"), "synthetic")
	write(t, filepath.Join(dir, "workload.txt"), "kept")
	removed, err := Cleanup(dir, ConfirmPhrase)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != "demo-synthetic-note.txt" {
		t.Fatal(removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "workload.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "demo-synthetic-note.txt")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func TestCleanupRefusesWithoutDeleting(t *testing.T) {
	dir := t.TempDir()
	note := filepath.Join(dir, "demo-synthetic-note.txt")
	write(t, note, "synthetic")
	if _, err := Cleanup(dir, "delete everything"); err == nil {
		t.Fatal("expected refusal")
	}
	if _, err := os.Stat(note); err != nil {
		t.Fatal(err)
	}

	retained := filepath.Join(t.TempDir(), "forge-native", "postgres")
	if err := os.MkdirAll(retained, 0o700); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(retained, "demo-synthetic-note.txt"), "synthetic")
	if _, err := Cleanup(retained, ConfirmPhrase); err == nil || !strings.Contains(err.Error(), "retained product state") {
		t.Fatal(err)
	}

	database := t.TempDir()
	write(t, filepath.Join(database, "PG_VERSION"), "18")
	write(t, filepath.Join(database, "demo-synthetic-note.txt"), "synthetic")
	if _, err := Cleanup(database, ConfirmPhrase); err == nil || !strings.Contains(err.Error(), "database") {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(database, "demo-synthetic-note.txt")); err != nil {
		t.Fatal("database file was removed")
	}

	secrets := t.TempDir()
	write(t, filepath.Join(secrets, "secrets.json"), "{}")
	write(t, filepath.Join(secrets, "demo-synthetic-note.txt"), "synthetic")
	if _, err := Cleanup(secrets, ConfirmPhrase); err == nil || !strings.Contains(err.Error(), "secrets") {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(secrets, "secrets.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(secrets, "demo-synthetic-note.txt")); err != nil {
		t.Fatal("synthetic file removed beside secrets")
	}

	shaped := t.TempDir()
	write(t, filepath.Join(shaped, "demo-synthetic-token.txt"), "no")
	if _, err := Cleanup(shaped, ConfirmPhrase); err == nil {
		t.Fatal("expected secret-shaped refusal")
	}
	if _, err := os.Stat(filepath.Join(shaped, "demo-synthetic-token.txt")); err != nil {
		t.Fatal(err)
	}

	t.Run("symlink", func(t *testing.T) {
		linked := t.TempDir()
		target := filepath.Join(linked, "elsewhere.txt")
		write(t, target, "kept")
		link := filepath.Join(linked, "demo-synthetic-link.txt")
		if err := os.Symlink(target, link); err != nil {
			t.Skip("symlink creation is not permitted on this machine")
		}
		if _, err := Cleanup(linked, ConfirmPhrase); err == nil || !strings.Contains(err.Error(), "link") {
			t.Fatal(err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatal(err)
		}
	})
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
