package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/octieght18/forge/internal/mcp"
)

func TestFixtureRecordsEvidenceOrUncertainty(t *testing.T) {
	Reset()
	empty, err := Fixture{}.Generate(context.Background(), "33333333-3333-4333-8333-333333333333", "Which service owns execution?", nil)
	if err != nil || empty.Outcome != "insufficient_evidence" || len(empty.Limitations) != 1 {
		t.Fatal(err, empty)
	}
	empty.WorkflowID = "forge-run/33333333-3333-4333-8333-333333333333"
	if err = Validate(empty, nil); err != nil {
		t.Fatal(err)
	}
	passages := []mcp.Passage{{DocumentID: "platform-brief", PassageID: "passage-1", EvidenceID: "44444444-4444-4444-8444-444444444444", Excerpt: "PostgreSQL owns product records."}}
	answered, err := Fixture{}.Generate(context.Background(), "33333333-3333-4333-8333-333333333333", "Which service owns execution?", passages)
	if err != nil || answered.Outcome != "answered" || len(answered.EvidenceIDs) != 1 || answered.RequestedID != RequestedModel || answered.Usage.PromptTokens == 0 {
		t.Fatal(err, answered)
	}
	answered.WorkflowID = "forge-run/33333333-3333-4333-8333-333333333333"
	if err = Validate(answered, passages); err != nil {
		t.Fatal(err)
	}
	got := Reports()
	if len(got) != 2 || got[1].ProviderID != "fixture" || got[1].EvidenceIDs[0] != passages[0].EvidenceID {
		t.Fatalf("%#v", got)
	}
}

func TestOpenRouterKeepsTheKeyOutOfTheBody(t *testing.T) {
	const key = "test-model-key"
	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = string(body)
		if r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, "missing", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": reportJSON()}}},
			"usage":   map[string]int{"prompt_tokens": 3, "completion_tokens": 5},
		})
	}))
	defer server.Close()
	passages := []mcp.Passage{{EvidenceID: "44444444-4444-4444-8444-444444444444", Excerpt: "PostgreSQL owns product records."}}
	report, err := (OpenRouter{Key: key, Endpoint: server.URL, HTTP: server.Client()}).Generate(context.Background(), "33333333-3333-4333-8333-333333333333", "Which service owns execution?", passages)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(seen, key) || report.Usage.PromptTokens != 3 || report.Usage.CompletionTokens != 5 || report.Outcome != "answered" {
		t.Fatal(seen, report)
	}
	failed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad "+key, http.StatusBadGateway)
	}))
	defer failed.Close()
	_, err = (OpenRouter{Key: key, Endpoint: failed.URL, HTTP: failed.Client()}).Generate(context.Background(), "33333333-3333-4333-8333-333333333333", "Which service owns execution?", passages)
	if err == nil || strings.Contains(err.Error(), key) {
		t.Fatal(err)
	}
}

func reportJSON() string {
	return `{"run_id":"33333333-3333-4333-8333-333333333333","generated_at":"2026-10-12T18:00:00Z","model":{"requested_id":"google/gemma-4-26b-a4b-it:free","actual_id":"google/gemma-4-26b-a4b-it","provider_id":"google-ai-studio"},"outcome":"answered","claims":[{"text":"The cited passage is the retrieved source for this question.","evidence_ids":["44444444-4444-4444-8444-444444444444"]}],"limitations":[]}`
}
