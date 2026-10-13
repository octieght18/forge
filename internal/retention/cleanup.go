package retention

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ConfirmPhrase is the only accepted cleanup confirmation.
const ConfirmPhrase = "delete-synthetic-demo-records"

// SyntheticPrefix marks a scratch file the demo cleanup may remove.
const SyntheticPrefix = "demo-synthetic-"

// Refusal is a cleanup that was not performed.
type Refusal struct {
	Reason string
}

func (e *Refusal) Error() string { return e.Reason }

func refuse(reason string) error { return &Refusal{Reason: reason} }

// Cleanup removes regular files in dir whose names start with SyntheticPrefix.
// The directory must be named explicitly and the confirmation must match.
// A retained product state directory, a database directory, a secret file,
// a link, or any other name is left in place and the job is refused.
func Cleanup(dir, confirm string) ([]string, error) {
	if confirm != ConfirmPhrase {
		return nil, refuse("confirmation does not match delete-synthetic-demo-records")
	}
	if strings.TrimSpace(dir) == "" {
		return nil, refuse("cleanup directory is required")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, refuse("cleanup directory is not usable")
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return nil, refuse("cleanup directory is not usable")
	}
	if retainedPath(resolved) {
		return nil, refuse("cleanup refuses the retained product state directory")
	}
	info, err := os.Lstat(resolved)
	if err != nil || !info.IsDir() {
		return nil, refuse("cleanup directory is not usable")
	}
	if _, err := os.Lstat(filepath.Join(resolved, "PG_VERSION")); err == nil {
		return nil, refuse("cleanup refuses a database directory")
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, refuse("cleanup directory is not usable")
	}
	var synthetic []string
	for _, entry := range entries {
		name := entry.Name()
		if secretName(name) {
			return nil, refuse("cleanup refuses a directory that holds secrets")
		}
		if strings.HasPrefix(name, SyntheticPrefix) {
			synthetic = append(synthetic, name)
		}
	}
	sort.Strings(synthetic)
	removed := make([]string, 0, len(synthetic))
	for _, name := range synthetic {
		path := filepath.Join(resolved, name)
		listed, err := os.Lstat(path)
		if err != nil {
			return nil, refuse("cleanup directory is not usable")
		}
		if listed.Mode()&os.ModeSymlink != 0 || !listed.Mode().IsRegular() {
			return nil, refuse("cleanup refuses a link or directory")
		}
		rel, err := filepath.Rel(resolved, path)
		if err != nil || rel != name {
			return nil, refuse("cleanup refuses a path outside the directory")
		}
		if err := os.Remove(path); err != nil {
			return nil, refuse("cleanup could not remove a synthetic file")
		}
		removed = append(removed, name)
	}
	return removed, nil
}

func retainedPath(path string) bool {
	parts := strings.Split(filepath.Clean(path), string(os.PathSeparator))
	for _, part := range parts {
		if part == "forge-native" || part == "forge-kubernetes" {
			return true
		}
	}
	return false
}

func secretName(name string) bool {
	lower := strings.ToLower(name)
	switch lower {
	case "secrets.json", "cursor.key", "kubeconfig", ".env", "pg_version":
		return true
	}
	if strings.HasPrefix(lower, ".env.") || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".key") {
		return true
	}
	if strings.Contains(lower, "password") || strings.Contains(lower, "token") || strings.Contains(lower, "kubeconfig") {
		return true
	}
	return false
}
