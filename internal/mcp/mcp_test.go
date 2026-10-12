package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

type denyTool struct{ calls int }

func (d *denyTool) Call(context.Context, string, map[string]any) (json.RawMessage, error) {
	d.calls++
	return nil, errors.New("Bearer super-secret postgres://user:password@db/forge")
}

func TestDeniedScopeDoesNotCallTheTool(t *testing.T) {
	Reset()
	tool := &denyTool{}
	outcome, err := Search(context.Background(), Input{
		RunID: "33333333-3333-4333-8333-333333333333", SnapshotID: strings.Repeat("a", 64),
		Question: "Which service owns execution?", DocumentIDs: []string{"../secrets"}, MaxPassages: 8,
	}, tool)
	if err != nil || !outcome.Failed || outcome.Class != "tool_denied" || tool.calls != 0 {
		t.Fatal(err, outcome, tool.calls)
	}
	if len(EvidenceRecords()) != 0 {
		t.Fatal(EvidenceRecords())
	}
}

func TestFixtureStoresProvenanceAndRedactsSecrets(t *testing.T) {
	Reset()
	leftRead, leftWrite := io.Pipe()
	rightRead, rightWrite := io.Pipe()
	go Serve(leftRead, rightWrite)
	tool := pipeTool{in: leftWrite, out: rightRead}
	outcome, err := Search(context.Background(), Input{
		RunID: "33333333-3333-4333-8333-333333333333", WorkflowID: "forge-run/33333333-3333-4333-8333-333333333333",
		CommandID: "44444444-4444-4444-8444-444444444444", SnapshotID: strings.Repeat("a", 64),
		Question: "Which service owns execution?", DocumentIDs: []string{"platform-brief"}, MaxPassages: 8,
	}, tool)
	if err != nil || outcome.Failed || len(outcome.Passages) != 1 || outcome.Passages[0].DocumentID != "platform-brief" {
		t.Fatal(err, outcome)
	}
	records := EvidenceRecords()
	if len(records) != 1 || records[0].Retention != RetentionWorkflowHistory || records[0].SnapshotID != strings.Repeat("a", 64) || records[0].TextSHA256 == "" || records[0].PassageID != "passage-1" {
		t.Fatalf("%#v", records)
	}
	secretTool := &denyTool{}
	failed, err := Search(context.Background(), Input{
		RunID: "33333333-3333-4333-8333-333333333333", WorkflowID: "forge-run/33333333-3333-4333-8333-333333333333",
		SnapshotID: strings.Repeat("b", 64), Question: "Which service owns execution?", DocumentIDs: []string{"platform-brief"},
	}, secretTool)
	if err == nil || !failed.Failed || strings.Contains(err.Error(), "super-secret") || strings.Contains(err.Error(), "password@") {
		t.Fatal(err, failed)
	}
}

type pipeTool struct {
	in  io.Writer
	out io.Reader
}

func (p pipeTool) Call(context.Context, string, map[string]any) (json.RawMessage, error) {
	return callRPC(p.in, p.out, "corpus.search", map[string]any{
		"tool": "corpus.search", "corpus_snapshot_id": strings.Repeat("a", 64),
		"document_ids": []string{"platform-brief"}, "query": "Which service owns execution?", "limit": 8,
	})
}
