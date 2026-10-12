// Package model turns retrieved passages into a validated research report.
// The hosted backend is OpenRouter's free Gemma route. Tests use Fixture and
// never read FORGE_MODEL_API_KEY.
package model

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/mcp"
)

const RequestedModel = "google/gemma-4-26b-a4b-it:free"

// Usage is the token count reported for one generation. It is not a bill.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// Report is the persisted research result.
type Report struct {
	RunID       string    `json:"run_id"`
	WorkflowID  string    `json:"workflow_id"`
	Outcome     string    `json:"outcome"`
	RequestedID string    `json:"requested_id"`
	ActualID    string    `json:"actual_id"`
	ProviderID  string    `json:"provider_id"`
	EvidenceIDs []string  `json:"evidence_ids"`
	Usage       Usage     `json:"usage"`
	Limitations []string  `json:"limitations"`
	GeneratedAt time.Time `json:"generated_at"`
	ClaimText   string    `json:"claim_text,omitempty"`
}

// Generator produces one report from already authorized passages.
type Generator interface {
	Generate(context.Context, string, string, []mcp.Passage) (Report, error)
}

var (
	mu      sync.Mutex
	reports []Report
)

// Reset clears persisted reports. Tests use it.
func Reset() {
	mu.Lock()
	reports = nil
	mu.Unlock()
}

// Reports returns the reports stored in this process.
func Reports() []Report {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Report, len(reports))
	copy(out, reports)
	return out
}

// Compensate drops a report that was stored before the run stopped.
func Compensate(runID string) {
	mu.Lock()
	defer mu.Unlock()
	kept := reports[:0]
	for _, report := range reports {
		if report.RunID != runID {
			kept = append(kept, report)
		}
	}
	reports = kept
}

func remember(report Report) {
	mu.Lock()
	reports = append(reports, report)
	mu.Unlock()
}

// Fixture is the deterministic backend. It cites the first passage or records
// that the context has no evidence.
type Fixture struct{}

func (Fixture) Generate(_ context.Context, runID, question string, passages []mcp.Passage) (Report, error) {
	if strings.TrimSpace(question) == "" {
		return Report{}, errors.New("question is empty")
	}
	report := Report{
		RunID: runID, RequestedID: RequestedModel, ActualID: RequestedModel,
		ProviderID: "fixture", GeneratedAt: time.Now().UTC(),
	}
	if len(passages) == 0 || passages[0].EvidenceID == "" {
		report.Outcome = "insufficient_evidence"
		report.Limitations = []string{"The frozen document scope contained no supporting passage."}
		return report, nil
	}
	report.Outcome = "answered"
	report.ClaimText = "The cited passage is the retrieved source for this question."
	report.EvidenceIDs = []string{passages[0].EvidenceID}
	report.Usage = Usage{PromptTokens: len(question), CompletionTokens: len(report.ClaimText)}
	return report, nil
}

// OpenRouter calls the selected free model. The API key stays in the Authorization
// header and is removed if it appears in an error.
type OpenRouter struct {
	Key      string
	Endpoint string
	HTTP     *http.Client
}

func (c OpenRouter) Generate(ctx context.Context, runID, question string, passages []mcp.Passage) (Report, error) {
	if c.Key == "" {
		return Report{}, errors.New("model credentials are not configured")
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = "https://openrouter.ai/api/v1/chat/completions"
	}
	var excerpts []string
	for _, passage := range passages {
		excerpts = append(excerpts, passage.DocumentID+": "+passage.Excerpt)
	}
	payload := map[string]any{
		"model":    RequestedModel,
		"messages": []map[string]string{{"role": "user", "content": question + "\n" + strings.Join(excerpts, "\n")}},
		"provider": map[string]any{"data_collection": "deny", "allow_fallbacks": false},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Report{}, err
	}
	if strings.Contains(string(body), c.Key) {
		return Report{}, errors.New("model request included a credential")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Report{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		return Report{}, errors.New(redact(err.Error(), c.Key))
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return Report{}, errors.New(redact(err.Error(), c.Key))
	}
	if response.StatusCode != http.StatusOK {
		return Report{}, errors.New(redact("model route returned "+response.Status, c.Key))
	}
	var decoded struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage Usage `json:"usage"`
	}
	if json.Unmarshal(raw, &decoded) != nil || len(decoded.Choices) == 0 {
		return Report{}, errors.New("model response was not usable")
	}
	report, err := Parse(runID, decoded.Choices[0].Message.Content, passages)
	if err != nil {
		return Report{}, err
	}
	report.Usage = decoded.Usage
	return report, nil
}

// Parse checks a model payload against the report schema and the retrieved evidence.
func Parse(runID, content string, passages []mcp.Passage) (Report, error) {
	validator, err := contract.New()
	if err != nil {
		return Report{}, err
	}
	if validator.Validate("Report", []byte(content)) != nil {
		return Report{}, errors.New("model output did not match the report schema")
	}
	var body struct {
		RunID       string `json:"run_id"`
		GeneratedAt string `json:"generated_at"`
		Model       struct {
			RequestedID string `json:"requested_id"`
			ActualID    string `json:"actual_id"`
			ProviderID  string `json:"provider_id"`
		} `json:"model"`
		Outcome string `json:"outcome"`
		Claims  []struct {
			Text        string   `json:"text"`
			EvidenceIDs []string `json:"evidence_ids"`
		} `json:"claims"`
		Limitations []string `json:"limitations"`
	}
	if json.Unmarshal([]byte(content), &body) != nil || body.RunID != runID || body.Model.RequestedID != RequestedModel {
		return Report{}, errors.New("model output did not match the report schema")
	}
	known := map[string]bool{}
	for _, passage := range passages {
		if passage.EvidenceID != "" {
			known[passage.EvidenceID] = true
		}
	}
	var ids []string
	for _, claim := range body.Claims {
		for _, id := range claim.EvidenceIDs {
			if !known[id] {
				return Report{}, errors.New("citation is outside the retrieved evidence")
			}
			ids = append(ids, id)
		}
	}
	when, err := time.Parse(time.RFC3339, body.GeneratedAt)
	if err != nil {
		return Report{}, errors.New("model output did not match the report schema")
	}
	text := ""
	if len(body.Claims) > 0 {
		text = body.Claims[0].Text
	}
	return Report{
		RunID: runID, GeneratedAt: when.UTC(), Outcome: body.Outcome, RequestedID: body.Model.RequestedID,
		ActualID: body.Model.ActualID, ProviderID: body.Model.ProviderID, EvidenceIDs: ids,
		Limitations: body.Limitations, ClaimText: text,
	}, nil
}

// Validate checks the report schema and stores it. An invalid report is not stored.
func Validate(report Report, passages []mcp.Passage) error {
	limitations := report.Limitations
	if limitations == nil {
		limitations = []string{}
	}
	body := map[string]any{
		"run_id": report.RunID, "generated_at": report.GeneratedAt.UTC().Format(time.RFC3339),
		"model":   map[string]string{"requested_id": report.RequestedID, "actual_id": report.ActualID, "provider_id": report.ProviderID},
		"outcome": report.Outcome, "claims": []any{}, "limitations": limitations,
	}
	if report.Outcome == "answered" {
		body["claims"] = []any{map[string]any{"text": report.ClaimText, "evidence_ids": report.EvidenceIDs}}
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	parsed, err := Parse(report.RunID, string(raw), passages)
	if err != nil {
		return err
	}
	parsed.WorkflowID = report.WorkflowID
	parsed.Usage = report.Usage
	remember(parsed)
	return nil
}

func redact(message, key string) string {
	if key != "" {
		message = strings.ReplaceAll(message, key, "[redacted]")
	}
	return mcp.Redact(message)
}
