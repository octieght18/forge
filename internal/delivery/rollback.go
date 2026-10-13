package delivery

import (
	"os"
	"path/filepath"
	"time"
)

// Watch is one canary observation window.
type Watch struct {
	Requests int
	Errors   int
	Latency  time.Duration
}

// RollbackRequest restores the previous pin after a regression.
// Executions are copied and never removed.
type RollbackRequest struct {
	Root               string
	Previous           string
	Watch              Watch
	MaxLatency         time.Duration
	Executions         []string
	MigrationApplied   bool
	ModelCallCompleted bool
}

// RollbackResult is the measured restore. Human is set when the pin must not move automatically.
type RollbackResult struct {
	Restored   string
	Detection  time.Duration
	Rollback   time.Duration
	Executions []string
	Human      string
}

// NeedsRollback is true when the error rate is above the canary ceiling or latency is above the supplied maximum.
func NeedsRollback(watch Watch, maxLatency time.Duration) bool {
	if watch.Requests > 0 && watch.Errors >= 0 && watch.Errors <= watch.Requests {
		if float64(watch.Errors)/float64(watch.Requests) > MaxCanaryErrorRate {
			return true
		}
	}
	return maxLatency > 0 && watch.Latency > maxLatency
}

// Rollback writes the previous pin when the watch is a regression and no human gate applies.
func Rollback(req RollbackRequest) (RollbackResult, error) {
	detect := time.Now()
	result := RollbackResult{Executions: append([]string(nil), req.Executions...)}
	regressed := NeedsRollback(req.Watch, req.MaxLatency)
	result.Detection = time.Since(detect)
	if !regressed {
		result.Human = "no regression was observed"
		return result, nil
	}
	switch {
	case req.MigrationApplied:
		result.Human = "forward-only migration already applied"
		return result, nil
	case req.ModelCallCompleted:
		result.Human = "a completed model call cannot be unsent"
		return result, nil
	case req.Previous == "":
		result.Human = "previous pin is missing"
		return result, nil
	}
	if req.Root == "" {
		return result, &Refusal{Reason: "rollback directory is required"}
	}
	staging := filepath.Join(req.Root, "staging", "release.txt")
	abs, err := filepath.Abs(staging)
	if err != nil || !inside(req.Root, abs) && !inside(mustAbs(req.Root), abs) {
		return result, &Refusal{Reason: "rollback refused"}
	}
	roll := time.Now()
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return result, &Refusal{Reason: "rollback directory is not usable"}
	}
	if err := os.WriteFile(abs, []byte(req.Previous+"\n"), 0o644); err != nil {
		return result, &Refusal{Reason: "rollback directory is not usable"}
	}
	result.Rollback = time.Since(roll)
	result.Restored = req.Previous
	return result, nil
}

func mustAbs(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
