package execution

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"time"

	"github.com/octieght18/forge/internal/mcp"
	"go.temporal.io/sdk/temporal"
)

const (
	MaxAttempts       = 3
	ActivityTimeout   = 30 * time.Second
	HeartbeatTimeout  = 10 * time.Second
	WorkflowTimeout   = 2 * time.Minute
	HeartbeatInterval = 2 * time.Second
)

// Policy is the activity retry limit. Three attempts use 1s, then 2s, then stop
// at a 4s ceiling. Nonretryable errors do not use this budget.
func Policy() *temporal.RetryPolicy {
	return &temporal.RetryPolicy{
		InitialInterval: time.Second, BackoffCoefficient: 2,
		MaximumInterval: 4 * time.Second, MaximumAttempts: MaxAttempts,
	}
}

// Retryable reports whether a failure class may be attempted again.
// tool_denied, model_invalid, and identity mismatches are not retryable.
func Retryable(class string) bool {
	return class == "tool_unavailable" || class == "model_unavailable"
}

// IdempotencyKey identifies one committed activity attempt. A repeat with the
// same key reuses the stored observation.
func IdempotencyKey(runID, step, material string) string {
	sum := sha256.Sum256([]byte(runID + "\x00" + step + "\x00" + material))
	return hex.EncodeToString(sum[:])
}

var (
	commitMu sync.Mutex
	commits  = map[string]Observation{}
)

// ResetCommits clears remembered activity results. Tests use it.
func ResetCommits() {
	commitMu.Lock()
	commits = map[string]Observation{}
	commitMu.Unlock()
}

func recall(key string) (Observation, bool) {
	commitMu.Lock()
	defer commitMu.Unlock()
	obs, ok := commits[key]
	return obs, ok
}

func remember(key string, obs Observation) {
	commitMu.Lock()
	commits[key] = obs
	commitMu.Unlock()
}

func failure(class string, err error) error {
	message := class
	if err != nil {
		message = mcp.Redact(err.Error())
	}
	if Retryable(class) {
		return temporal.NewApplicationError(message, class)
	}
	return temporal.NewNonRetryableApplicationError(message, class, nil)
}

func material(in Correlation) string {
	return in.SnapshotID + "\x00" + in.Question + "\x00" + strings.Join(in.DocumentIDs, ",")
}

// Terminal is the safe end state recorded for a run. Compensation is incomplete
// when that cleanup activity does not finish.
type Terminal struct {
	RunID, WorkflowID, State, Compensation string
}

var (
	terminalMu sync.Mutex
	terminals  []Terminal
)

// ResetTerminals clears recorded end states. Tests use it.
func ResetTerminals() {
	terminalMu.Lock()
	terminals = nil
	terminalMu.Unlock()
}

// Terminals returns the end states recorded in this process.
func Terminals() []Terminal {
	terminalMu.Lock()
	defer terminalMu.Unlock()
	out := make([]Terminal, len(terminals))
	copy(out, terminals)
	return out
}

// RecordTerminal stores a terminal state. It does not write execution status to PostgreSQL.
func RecordTerminal(_ context.Context, state Terminal) error {
	if state.State != "complete" && state.State != "fail_closed" && state.State != "canceled" && state.State != "timed_out" {
		return temporal.NewNonRetryableApplicationError("unknown terminal state", "permanent", nil)
	}
	terminalMu.Lock()
	terminals = append(terminals, state)
	terminalMu.Unlock()
	return nil
}
