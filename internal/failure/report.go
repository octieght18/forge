// Package failure records the operational failure-injection scenarios.
// The suite does not kill a host process and does not start a Temporal server.
package failure

import (
	"encoding/json"
	"os"
	"time"
)

// Scenario is one injected failure and the observed recovery.
type Scenario struct {
	Name                 string `json:"name"`
	Recovered            bool   `json:"recovered"`
	Isolated             bool   `json:"isolated"`
	SideEffects          int    `json:"side_effects"`
	DuplicateSideEffects int    `json:"duplicate_side_effects"`
	DurationUS           int64  `json:"duration_us"`
	Limit                string `json:"limit"`
}

// Report is the raw suite result.
type Report struct {
	Scenarios []Scenario `json:"scenarios"`
}

// WriteFile stores the raw report. The caller chooses a path outside the retained database.
func WriteFile(path string, report Report) error {
	body, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return os.WriteFile(path, body, 0o600)
}

func elapsed(start time.Time) int64 {
	return time.Since(start).Microseconds()
}
