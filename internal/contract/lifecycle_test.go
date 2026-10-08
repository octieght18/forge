package contract

import (
	"encoding/json"
	"errors"
	"testing"
)

// These are response consistency checks, not a replacement for Temporal's
// execution state machine or proof that a dispatcher delivered a command.
func TestRunLifecycleConsistency(t *testing.T) {
	v := validator(t)
	states := []string{"running", "succeeded", "failed", "canceled", "timed_out", "terminated"}
	for _, submission := range []string{"pending", "dispatched"} {
		for _, cancellation := range []string{"not_requested", "requested", "delivered"} {
			for _, state := range states {
				t.Run(submission+"/"+cancellation+"/"+state, func(t *testing.T) {
					payload := alter(t, "pending-run", func(r map[string]any) {
						r["submission_state"] = submission
						r["cancellation_state"] = cancellation
						r["execution"] = map[string]any{"source": "temporal", "state": state, "observed_at": "2026-10-08T12:00:00Z", "last_known_state": nil}
					})
					err := v.Validate("Run", payload)
					invalid := submission == "pending" && cancellation == "delivered"
					if invalid && !errors.Is(err, ErrInvalidInput) || !invalid && err != nil {
						t.Fatalf("response consistency: invalid=%v, error=%v", invalid, err)
					}
				})
			}
		}
	}
	for _, schema := range []string{"Run", "RunPage"} {
		payload := alter(t, "pending-run", func(r map[string]any) { r["submission_state"] = "dispatched" })
		if schema == "RunPage" {
			var run any
			_ = json.Unmarshal(payload, &run)
			payload, _ = json.Marshal(map[string]any{"items": []any{run}, "next_cursor": nil})
		}
		if err := v.Validate(schema, payload); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("%s accepted dispatched/not_started: %v", schema, err)
		}
	}
	for _, name := range []string{"ambiguous-start-run", "cancel-completion-race-run"} {
		if err := v.Validate("Run", example(t, name)); err != nil {
			t.Fatalf("valid ambiguity/completion race %s: %v", name, err)
		}
	}
}

func TestUnavailableObservationPair(t *testing.T) {
	v := validator(t)
	for _, state := range []any{nil, "running", "succeeded", "failed", "canceled", "timed_out", "terminated"} {
		for _, observed := range []any{nil, "2026-10-08T12:00:00Z"} {
			payload := alter(t, "unavailable-run", func(r map[string]any) {
				e := r["execution"].(map[string]any)
				e["last_known_state"], e["observed_at"] = state, observed
			})
			invalid := (state == nil) != (observed == nil)
			err := v.Validate("Run", payload)
			if invalid && !errors.Is(err, ErrInvalidInput) || !invalid && err != nil {
				t.Fatalf("cache pair state=%v, observed=%v: %v", state, observed, err)
			}
		}
	}
}
