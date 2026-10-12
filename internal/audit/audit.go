// Package audit records who did what and counts usage without prompts or secrets.
package audit

import (
	"strings"
	"sync"
	"time"
)

// Event is one product action. It has no token, prompt, or corpus text.
type Event struct {
	Actor     string    `json:"actor"`
	Tenant    string    `json:"tenant"`
	Operation string    `json:"operation"`
	At        time.Time `json:"at"`
	Target    string    `json:"target"`
	Outcome   string    `json:"outcome"`
}

// Usage is model and tool consumption for one idempotency key.
type Usage struct {
	Key              string
	PromptTokens     int
	CompletionTokens int
	ToolCalls        int
}

var (
	mu      sync.Mutex
	events  []Event
	meter   = map[string]Usage{}
	toolUse = map[string]Usage{}
)

// Reset clears the process record. Tests use it.
func Reset() {
	mu.Lock()
	events = nil
	meter = map[string]Usage{}
	toolUse = map[string]Usage{}
	mu.Unlock()
}

// Record stores one event after removing secret-shaped text.
func Record(event Event) {
	event.Actor = clean(event.Actor)
	event.Tenant = clean(event.Tenant)
	event.Operation = clean(event.Operation)
	event.Target = clean(event.Target)
	event.Outcome = clean(event.Outcome)
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	mu.Lock()
	events = append(events, event)
	mu.Unlock()
}

// Events returns a copy of the recorded actions.
func Events() []Event {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Event, len(events))
	copy(out, events)
	return out
}

// NoteModel counts prompt and completion tokens once per idempotency key.
func NoteModel(key string, prompt, completion int) {
	if key == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := meter[key]; ok {
		return
	}
	meter[key] = Usage{Key: key, PromptTokens: prompt, CompletionTokens: completion}
}

// NoteTool counts one tool call once per idempotency key.
func NoteTool(key string) {
	if key == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := toolUse[key]; ok {
		return
	}
	toolUse[key] = Usage{Key: key, ToolCalls: 1}
}

// Totals sums the deduplicated model and tool meters.
func Totals() (prompt, completion, tools int) {
	mu.Lock()
	defer mu.Unlock()
	for _, item := range meter {
		prompt += item.PromptTokens
		completion += item.CompletionTokens
	}
	for _, item := range toolUse {
		tools += item.ToolCalls
	}
	return prompt, completion, tools
}

func clean(value string) string {
	lower := strings.ToLower(value)
	if strings.Contains(lower, "bearer") || strings.Contains(lower, "password") || strings.Contains(lower, "postgres") || strings.Contains(value, "://") && strings.Contains(lower, "token") {
		return "redacted"
	}
	return value
}
