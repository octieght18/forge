// Package delivery promotes a release pin between local and staging.
// It does not call a cloud API and it does not change PostgreSQL or Temporal.
package delivery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConfirmPhrase is the only confirmation that copies a pin into staging.
const ConfirmPhrase = "promote-local-to-staging"

// MaxCanaryErrorRate is the canary success ceiling. It is not a production SLO.
const MaxCanaryErrorRate = 0.01

// Refusal is a promotion that was not performed.
type Refusal struct {
	Reason string
}

func (e *Refusal) Error() string { return e.Reason }

// CanaryAccepts reports whether the observed error rate is within the canary ceiling.
// A window with no requests is not a success.
func CanaryAccepts(requests, errors int) bool {
	if requests <= 0 || errors < 0 || errors > requests {
		return false
	}
	return float64(errors)/float64(requests) <= MaxCanaryErrorRate
}

// Promote copies the local release pin to staging.
// A cloud, EKS, or GKE destination is refused because that environment was not created.
func Promote(root, from, to, confirm string) (string, error) {
	if to == "cloud" || to == "eks" || to == "gke" {
		return "", &Refusal{Reason: "selected cloud environment was not created"}
	}
	if confirm != ConfirmPhrase || from != "local" || to != "staging" {
		return "", &Refusal{Reason: "promotion refused"}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", &Refusal{Reason: "promotion directory is not usable"}
	}
	local := filepath.Join(abs, "local", "release.txt")
	staging := filepath.Join(abs, "staging", "release.txt")
	if !inside(abs, local) || !inside(abs, staging) {
		return "", &Refusal{Reason: "promotion refused"}
	}
	version, err := os.ReadFile(local)
	if err != nil {
		return "", &Refusal{Reason: "local release pin is missing"}
	}
	pin := strings.TrimSpace(string(version))
	if pin == "" || strings.ContainsAny(pin, "\r\n") || strings.Contains(pin, "://") {
		return "", &Refusal{Reason: "local release pin is not usable"}
	}
	if err := os.MkdirAll(filepath.Dir(staging), 0o755); err != nil {
		return "", &Refusal{Reason: "promotion directory is not usable"}
	}
	if err := os.WriteFile(staging, []byte(pin+"\n"), 0o644); err != nil {
		return "", &Refusal{Reason: "promotion directory is not usable"}
	}
	return pin, nil
}

func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// RollbackLimits is the operator reminder stored with the procedure.
func RollbackLimits() string {
	return fmt.Sprintf("Git can point staging back at the previous pin. A forward-only migration is not reversed. A completed model call cannot be unsent. Temporal history is not rewritten. Retained execution rows are not deleted. Canary acceptance is an error rate at or below %g.", MaxCanaryErrorRate)
}
