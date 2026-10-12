package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAuditOmitsSecretsAndDeduplicatesUsage(t *testing.T) {
	Reset()
	Record(Event{Actor: "Bearer secret-token", Tenant: "00000000-0000-4000-8000-000000000001", Operation: "POST /api/v1/runs", Target: "33333333-3333-4333-8333-333333333333", Outcome: "success"})
	body, err := json.Marshal(Events())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "Bearer") || strings.Contains(string(body), "secret-token") {
		t.Fatal(string(body))
	}
	NoteModel("key-1", 11, 7)
	NoteModel("key-1", 100, 100)
	NoteTool("tool-1")
	NoteTool("tool-1")
	prompt, completion, tools := Totals()
	if prompt != 11 || completion != 7 || tools != 1 {
		t.Fatal(prompt, completion, tools)
	}
}
