package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/contract"
	"github.com/octieght18/forge/internal/execution"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

type fakeExecution struct {
	status     execution.LiveState
	statusErr  error
	events     []execution.Event
	historyErr error
	result     execution.ResultView
	resultErr  error
}

func (f *fakeExecution) Status(context.Context, string) (execution.LiveState, error) {
	return f.status, f.statusErr
}
func (f *fakeExecution) History(context.Context, string) ([]execution.Event, error) {
	return f.events, f.historyErr
}
func (f *fakeExecution) Result(context.Context, string) (execution.ResultView, error) {
	return f.result, f.resultErr
}

func TestRunHistoryAndResultHTTP(t *testing.T) {
	db := testsupport.NewDatabase(t)
	issuer := testsupport.NewIdentity(t)
	identity, err := auth.New(context.Background(), issuer.Server.URL, "forge-api")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := LoadCorpusPolicy([]byte(`{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":["platform-brief","architecture-notes"]}`))
	if err != nil {
		t.Fatal(err)
	}
	ready := &Readiness{}
	ready.Set(true)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	fake := &fakeExecution{statusErr: execution.ErrAbsent}
	options := Registration{Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{1}, 32), Timeout: time.Second, Execution: fake}
	handler, err := NewRegistrationHandler(logger, ready, options)
	if err != nil {
		t.Fatal(err)
	}
	closed, err := NewRegistrationHandler(logger, ready, Registration{Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy, CursorKey: bytes.Repeat([]byte{2}, 32), Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	defer server.Close()
	offline := httptest.NewServer(closed)
	defer offline.Close()
	tokens := map[string]string{}
	for _, role := range []string{"a", "b", "operator"} {
		mapped := "developer"
		if role == "operator" {
			mapped = role
		}
		tokens[role] = issuer.Token(t, role, mapped, nil)
	}
	validator, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	call := func(t *testing.T, base, method, path, user, body string, headers map[string]string) (int, []byte, http.Header) {
		t.Helper()
		req, err := http.NewRequest(method, base+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if user != "" {
			req.Header.Set("Authorization", "Bearer "+tokens[user])
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		payload, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, payload, response.Header
	}
	expect := func(t *testing.T, got, want int) {
		t.Helper()
		if got != want {
			t.Fatalf("got status %d, want %d", got, want)
		}
	}
	status, _, headers := call(t, server.URL, "POST", "/api/v1/runs", "", `{}`, nil)
	expect(t, status, 401)
	if headers.Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing bearer challenge")
	}
	status, workloadBody, _ := call(t, server.URL, "POST", "/api/v1/workloads", "a", `{"name":"research","description":"history"}`, nil)
	expect(t, status, 201)
	var workload store.Workload
	json.Unmarshal(workloadBody, &workload)
	versionPayload, err := contract.Examples.ReadFile("examples/create-version.json")
	if err != nil {
		t.Fatal(err)
	}
	status, versionBody, _ := call(t, server.URL, "POST", "/api/v1/workloads/"+workload.ID+"/versions", "a", string(versionPayload), nil)
	expect(t, status, 201)
	var version store.Version
	json.Unmarshal(versionBody, &version)
	runExample, err := contract.Examples.ReadFile("examples/create-run.json")
	if err != nil {
		t.Fatal(err)
	}
	runBody := strings.ReplaceAll(string(runExample), "11111111-1111-4111-8111-111111111111", workload.ID)
	runBody = strings.ReplaceAll(runBody, "22222222-2222-4222-8222-222222222222", version.ID)
	question := "Which components own product state and durable execution?"
	status, createdBody, createdHeaders := call(t, server.URL, "POST", "/api/v1/runs", "a", runBody, map[string]string{"Idempotency-Key": "run-1"})
	expect(t, status, 202)
	if validator.Validate("Run", createdBody) != nil {
		t.Fatalf("run contract: %s", createdBody)
	}
	var created struct {
		RunID     string `json:"run_id"`
		Execution struct {
			Source string `json:"source"`
			State  string `json:"state"`
		} `json:"execution"`
		Submission string `json:"submission_state"`
	}
	json.Unmarshal(createdBody, &created)
	if created.Execution.Source != "not_started" || created.Execution.State != "not_started" || created.Submission != "pending" {
		t.Fatalf("start view %#v", created)
	}
	if createdHeaders.Get("Location") != "/api/v1/runs/"+created.RunID {
		t.Fatal(createdHeaders.Get("Location"))
	}
	status, again, _ := call(t, server.URL, "POST", "/api/v1/runs", "a", runBody, map[string]string{"Idempotency-Key": "run-1"})
	expect(t, status, 202)
	var repeated struct {
		RunID string `json:"run_id"`
	}
	json.Unmarshal(again, &repeated)
	if repeated.RunID != created.RunID {
		t.Fatal("idempotent start created another run")
	}
	changed := strings.Replace(runBody, question, "A different question", 1)
	status, _, _ = call(t, server.URL, "POST", "/api/v1/runs", "a", changed, map[string]string{"Idempotency-Key": "run-1"})
	expect(t, status, 409)
	status, _, _ = call(t, server.URL, "POST", "/api/v1/runs", "operator", runBody, map[string]string{"Idempotency-Key": "operator-start"})
	expect(t, status, 404)
	status, _, _ = call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID, "b", "", nil)
	expect(t, status, 404)
	status, _, _ = call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID, "operator", "", nil)
	expect(t, status, 200)
	status, _, _ = call(t, server.URL, "POST", "/api/v1/runs/"+created.RunID+"/cancel", "operator", "", nil)
	expect(t, status, 404)
	status, _, _ = call(t, server.URL, "POST", "/api/v1/runs/"+created.RunID+"/cancel", "b", "", nil)
	expect(t, status, 404)
	status, cancelBody, _ := call(t, server.URL, "POST", "/api/v1/runs/"+created.RunID+"/cancel", "a", "", nil)
	expect(t, status, 202)
	if validator.Validate("CancelResponse", cancelBody) != nil {
		t.Fatalf("cancel contract: %s", cancelBody)
	}
	status, cancelAgain, _ := call(t, server.URL, "POST", "/api/v1/runs/"+created.RunID+"/cancel", "a", "", nil)
	expect(t, status, 202)
	if string(cancelAgain) != string(cancelBody) && !strings.Contains(string(cancelAgain), `"requested"`) {
		t.Fatalf("repeat cancel %s", cancelAgain)
	}
	status, _, _ = call(t, server.URL, "DELETE", "/api/v1/runs/"+created.RunID, "a", "", nil)
	expect(t, status, 405)
	status, _, _ = call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/history", "b", "", nil)
	expect(t, status, 404)
	status, emptyHistory, _ := call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/history", "a", "", nil)
	expect(t, status, 200)
	if validator.Validate("HistoryPage", emptyHistory) != nil || !strings.Contains(string(emptyHistory), `"items":[]`) {
		t.Fatalf("empty history %s", emptyHistory)
	}
	fake.historyErr = execution.ErrHistoryUnavailable
	fake.statusErr = nil
	status, _, _ = call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/history", "a", "", nil)
	expect(t, status, 503)
	when := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	fake.historyErr = nil
	fake.events = []execution.Event{
		{EventID: "1", Sequence: 1, Step: "workflow", Attempt: 1, EventType: "started", OccurredAt: when},
		{EventID: "2", Sequence: 2, Step: "retrieval", Attempt: 1, EventType: "failed", OccurredAt: when.Add(time.Second), Failure: &execution.SafeFailure{Code: "tool_unavailable", Message: "The tool could not be reached"}},
		{EventID: "3", Sequence: 3, Step: "retrieval", Attempt: 2, EventType: "started", OccurredAt: when.Add(2 * time.Second)},
	}
	fake.events = append(fake.events, execution.Event{EventID: "9", Sequence: 9, Step: "retrieval", Attempt: 2, EventType: "failed", OccurredAt: when, Failure: &execution.SafeFailure{Code: "tool_unavailable", Message: "Bearer super-secret-token " + question}})
	status, firstPage, _ := call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/history?limit=1", "a", "", nil)
	expect(t, status, 200)
	if validator.Validate("HistoryPage", firstPage) != nil {
		t.Fatalf("history contract %s", firstPage)
	}
	var page struct {
		Items []struct {
			Step    string `json:"step"`
			Attempt int    `json:"attempt"`
		} `json:"items"`
		Next *string `json:"next_cursor"`
	}
	json.Unmarshal(firstPage, &page)
	if len(page.Items) != 1 || page.Items[0].Attempt != 1 || page.Items[0].Step != "workflow" || page.Next == nil {
		t.Fatalf("page %#v", page)
	}
	status, _, _ = call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/history?limit=1&cursor="+*page.Next, "operator", "", nil)
	expect(t, status, 400)
	status, fullHistory, _ := call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/history", "operator", "", nil)
	expect(t, status, 200)
	if validator.Validate("HistoryPage", fullHistory) != nil {
		t.Fatalf("full history %s", fullHistory)
	}
	if strings.Contains(string(fullHistory), question) || strings.Contains(string(fullHistory), "Bearer") || strings.Contains(string(fullHistory), "super-secret-token") {
		t.Fatalf("history leaked content %s", fullHistory)
	}
	if !strings.Contains(string(fullHistory), `"attempt":2`) || !strings.Contains(string(fullHistory), `"step":"retrieval"`) || !strings.Contains(string(fullHistory), "The activity failed") {
		t.Fatalf("history missing attempt detail %s", fullHistory)
	}
	fake.resultErr = execution.ErrNotReady
	status, _, _ = call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/result", "a", "", nil)
	expect(t, status, 409)
	status, _, _ = call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/result", "b", "", nil)
	expect(t, status, 404)
	fake.resultErr = nil
	fake.result = execution.ResultView{RunID: created.RunID, Decision: "complete", Reason: "answered", Outcome: "answered", EvidenceIDs: []string{created.RunID, question}}
	status, resultPayload, _ := call(t, server.URL, "GET", "/api/v1/runs/"+created.RunID+"/result", "operator", "", nil)
	expect(t, status, 200)
	if strings.Contains(string(resultPayload), question) || strings.Contains(string(resultPayload), "Bearer") {
		t.Fatalf("result leaked %s", resultPayload)
	}
	var result resultBody
	json.Unmarshal(resultPayload, &result)
	if result.Decision != "complete" || result.Outcome != "answered" || len(result.EvidenceIDs) != 1 || result.EvidenceIDs[0] != created.RunID {
		t.Fatalf("%#v", result)
	}
	status, offlineHistory, _ := call(t, offline.URL, "GET", "/api/v1/runs/"+created.RunID+"/history", "a", "", nil)
	expect(t, status, 503)
	if !strings.Contains(string(offlineHistory), `"unavailable"`) {
		t.Fatalf("offline history %s", offlineHistory)
	}
	status, _, _ = call(t, offline.URL, "GET", "/api/v1/runs/"+created.RunID+"/result", "a", "", nil)
	expect(t, status, 503)
	status, offlineRun, _ := call(t, offline.URL, "GET", "/api/v1/runs/"+created.RunID, "a", "", nil)
	expect(t, status, 200)
	if validator.Validate("Run", offlineRun) != nil || !strings.Contains(string(offlineRun), `"source":"unavailable"`) {
		t.Fatalf("offline run %s", offlineRun)
	}
}
