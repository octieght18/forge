package failure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/octieght18/forge/internal/auth"
	"github.com/octieght18/forge/internal/delivery"
	"github.com/octieght18/forge/internal/execution"
	"github.com/octieght18/forge/internal/httpapi"
	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
	"github.com/octieght18/forge/internal/store"
	"github.com/octieght18/forge/internal/testsupport"
)

func TestFailureInjection(t *testing.T) {
	report := Report{Scenarios: []Scenario{
		workerLoss(t),
		toolAndModelOutage(t),
		badRollout(t),
	}}
	if os.Getenv("FORGE_TEST_ADMIN_DATABASE_URL") != "" || os.Getenv("FORGE_REQUIRE_DB_TESTS") == "1" {
		report.Scenarios = append(report.Scenarios, databaseOutage(t), quotaExhaustion(t))
	} else {
		t.Log("database outage and quota exhaustion need FORGE_TEST_ADMIN_DATABASE_URL")
	}
	for _, scenario := range report.Scenarios {
		if !scenario.Recovered || !scenario.Isolated || scenario.DuplicateSideEffects != 0 {
			t.Fatalf("%#v", scenario)
		}
		t.Logf("SCENARIO %s duration_us=%d side_effects=%d", scenario.Name, scenario.DurationUS, scenario.SideEffects)
	}
	if path := os.Getenv("FORGE_FAILURE_OUTPUT"); path != "" {
		if err := WriteFile(path, report); err != nil {
			t.Fatal(err)
		}
	}
}

func workerLoss(t *testing.T) Scenario {
	t.Helper()
	t.Cleanup(func() {
		execution.SetCorpus(nil)
		execution.SetModel(nil)
		execution.ResetActivityEntries()
	})
	tool := &countingTool{}
	execution.SetCorpus(tool)
	execution.SetModel(model.Fixture{})
	execution.ResetActivityEntries()
	in := execution.Correlation{
		RunID: "abababab-abab-4bab-8bab-abababababab", WorkflowID: "forge-run/abababab-abab-4bab-8bab-abababababab",
		WorkloadID: "11111111-1111-4111-8111-111111111111", VersionID: "22222222-2222-4222-8222-222222222222",
		CommandID: "44444444-4444-4444-8444-444444444444", SnapshotID: strings.Repeat("a", 64),
		Question: "Which service owns execution?", DocumentIDs: []string{"platform-brief"}, MaxPassages: 8,
	}
	history, err := execution.SyntheticSuccessHistory(in)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	if err := execution.Replay(history, in.WorkflowID); err != nil {
		t.Fatal(err)
	}
	if tool.calls != 0 || execution.ActivityEntries() != 0 {
		t.Fatalf("calls=%d entries=%d", tool.calls, execution.ActivityEntries())
	}
	return Scenario{
		Name: "worker_loss", Recovered: true, Isolated: true, SideEffects: 0, DuplicateSideEffects: 0,
		DurationUS: elapsed(started),
		Limit:      "No worker process was killed and no Temporal server was started. Resume is replay of constructed history, which does not call the tool or model.",
	}
}

func toolAndModelOutage(t *testing.T) Scenario {
	t.Helper()
	mcp.Reset()
	model.Reset()
	t.Cleanup(func() {
		mcp.Reset()
		model.Reset()
	})
	tool := &flakyTool{}
	input := mcp.Input{
		RunID: "bcbcbcbc-bcbc-4cbc-8cbc-bcbcbcbcbcbc", WorkflowID: "forge-run/bcbcbcbc-bcbc-4cbc-8cbc-bcbcbcbcbcbc",
		SnapshotID: strings.Repeat("b", 64), Question: "Which service owns execution?",
		DocumentIDs: []string{"platform-brief"}, MaxPassages: 8,
	}
	started := time.Now()
	denied := input
	denied.DocumentIDs = []string{"../secrets"}
	deniedOutcome, err := mcp.Search(context.Background(), denied, tool)
	if err != nil || !deniedOutcome.Failed || deniedOutcome.Class != "tool_denied" || tool.calls != 0 {
		t.Fatalf("%#v %v calls=%d", deniedOutcome, err, tool.calls)
	}
	down, err := mcp.Search(context.Background(), input, tool)
	if err == nil || !down.Failed || down.Class != "tool_unavailable" || strings.Contains(err.Error(), "postgres://") || strings.Contains(err.Error(), "secret@") {
		t.Fatalf("%#v %v", down, err)
	}
	up, err := mcp.Search(context.Background(), input, tool)
	if err != nil || up.Failed || len(mcp.EvidenceRecords()) != 1 {
		t.Fatalf("%#v %v records=%d", up, err, len(mcp.EvidenceRecords()))
	}
	generator := &flakyModel{}
	_, err = generator.Generate(context.Background(), input.RunID, input.Question, nil)
	if err == nil || strings.Contains(mcp.Redact(err.Error()), "secret-token") {
		t.Fatal(err)
	}
	report, err := generator.Generate(context.Background(), input.RunID, input.Question, up.Passages)
	if err != nil {
		t.Fatal(err)
	}
	report.WorkflowID = input.WorkflowID
	if err := model.Validate(report, up.Passages); err != nil {
		t.Fatal(err)
	}
	if generator.calls != 2 || len(model.Reports()) != 1 || tool.calls != 2 {
		t.Fatalf("model calls=%d reports=%d tool calls=%d", generator.calls, len(model.Reports()), tool.calls)
	}
	return Scenario{
		Name: "tool_model_outage", Recovered: true, Isolated: true, SideEffects: 1, DuplicateSideEffects: 0,
		DurationUS: elapsed(started),
		Limit:      "The outage is the in-process fixture. No OpenRouter call and no MCP process were started.",
	}
}

func badRollout(t *testing.T) Scenario {
	t.Helper()
	root := t.TempDir()
	staging := filepath.Join(root, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "release.txt"), []byte("bad-release\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	executions := []string{"33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"}
	started := time.Now()
	result, err := delivery.Rollback(delivery.RollbackRequest{
		Root: root, Previous: "local-overlay",
		Watch:      delivery.Watch{Requests: 100, Errors: 40, Latency: 800 * time.Millisecond},
		MaxLatency: 200 * time.Millisecond, Executions: executions,
	})
	if err != nil || result.Restored != "local-overlay" || len(result.Executions) != 2 {
		t.Fatal(err, result)
	}
	again, err := delivery.Rollback(delivery.RollbackRequest{
		Root: root, Previous: "local-overlay",
		Watch: delivery.Watch{Requests: 100, Errors: 0}, Executions: result.Executions,
	})
	if err != nil || again.Human == "" || len(again.Executions) != 2 {
		t.Fatal(err, again)
	}
	body, err := os.ReadFile(filepath.Join(staging, "release.txt"))
	if err != nil || strings.TrimSpace(string(body)) != "local-overlay" {
		t.Fatal(err, string(body))
	}
	return Scenario{
		Name: "bad_rollout", Recovered: true, Isolated: true, SideEffects: 1, DuplicateSideEffects: 0,
		DurationUS: elapsed(started),
		Limit:      "Rollback rewrites the staging pin file. It does not reverse a migration, unsend a model call, or change a live cluster.",
	}
}

func databaseOutage(t *testing.T) Scenario {
	t.Helper()
	db := testsupport.NewDatabase(t)
	issuer := testsupport.NewIdentity(t)
	identity, err := auth.New(context.Background(), issuer.Server.URL, "forge-api")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := httpapi.LoadCorpusPolicy([]byte(`{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":["platform-brief","architecture-notes"]}`))
	if err != nil {
		t.Fatal(err)
	}
	ready := &httpapi.Readiness{}
	ready.Set(true)
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler, err := httpapi.NewRegistrationHandler(logger, ready, httpapi.Registration{
		Repository: db.Repo, Auth: identity, Pool: db.Pool, Policy: policy,
		CursorKey: bytes.Repeat([]byte{7}, 32), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	owner := issuer.Token(t, "failure-owner", "developer", nil)
	other := issuer.Token(t, "failure-other", "developer", nil)
	call := func(method, path, token, body string) (int, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		payload, _ := io.ReadAll(response.Body)
		return response.StatusCode, payload
	}
	status, payload := call(http.MethodPost, "/api/v1/workloads", owner, `{"name":"failure-injection","description":"synthetic injection"}`)
	if status != http.StatusCreated {
		t.Fatal(status, string(payload))
	}
	var created store.Workload
	if err := json.Unmarshal(payload, &created); err != nil || created.ID == "" {
		t.Fatal(err, string(payload))
	}
	var runtime string
	if err := db.Pool.QueryRow(context.Background(), `SELECT current_user`).Scan(&runtime); err != nil {
		t.Fatal(err)
	}
	quoted := pgx.Identifier{runtime}.Sanitize()
	restore := func() {
		_, _ = db.Admin.Exec(context.Background(), "ALTER ROLE "+quoted+" LOGIN")
	}
	t.Cleanup(restore)
	started := time.Now()
	if _, err := db.Admin.Exec(context.Background(), "ALTER ROLE "+quoted+" NOLOGIN"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Admin.Exec(context.Background(), `SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE usename=$1`, runtime); err != nil {
		t.Fatal(err)
	}
	status, payload = call(http.MethodGet, "/api/v1/workloads/"+created.ID, owner, "")
	if status != http.StatusServiceUnavailable || strings.Contains(string(payload), "synthetic injection") {
		t.Fatal(status, string(payload))
	}
	status, payload = call(http.MethodGet, "/healthz", "", "")
	if status != http.StatusOK {
		t.Fatal(status, string(payload))
	}
	status, _ = call(http.MethodGet, "/readyz", "", "")
	if status != http.StatusServiceUnavailable {
		t.Fatal(status)
	}
	restore()
	var recovered bool
	for attempt := 0; attempt < 10; attempt++ {
		status, payload = call(http.MethodGet, "/api/v1/workloads/"+created.ID, owner, "")
		if status == http.StatusOK && strings.Contains(string(payload), created.ID) {
			recovered = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !recovered {
		t.Fatal(status, string(payload))
	}
	status, payload = call(http.MethodGet, "/api/v1/workloads/"+created.ID, other, "")
	if status != http.StatusNotFound || strings.Contains(string(payload), "synthetic injection") {
		t.Fatal(status, string(payload))
	}
	var count int
	if err := db.Migration.QueryRow(context.Background(), `SELECT count(*) FROM forge.workloads WHERE name='failure-injection'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(err, count)
	}
	return Scenario{
		Name: "database_outage", Recovered: true, Isolated: true, SideEffects: 1, DuplicateSideEffects: 0,
		DurationUS: elapsed(started),
		Limit:      "The outage disables login for the disposable test role and terminates its sessions. The retained product database was not stopped.",
	}
}

func quotaExhaustion(t *testing.T) Scenario {
	t.Helper()
	db := testsupport.NewDatabase(t)
	db.Repo.SetLimits(1, 1)
	owner := store.Principal{Issuer: "https://failure.example", Subject: "owner", Role: "developer"}
	other := store.Principal{Issuer: "https://failure.example", Subject: "other", Role: "developer"}
	started := time.Now()
	first, err := db.Repo.CreateWorkload(context.Background(), owner, []byte(`{"name":"quota-one","description":"synthetic quota"}`))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Repo.CreateWorkload(context.Background(), owner, []byte(`{"name":"quota-two","description":"synthetic quota"}`))
	if !errors.Is(err, store.ErrLimited) {
		t.Fatal(err)
	}
	got, err := db.Repo.GetWorkload(context.Background(), owner, first.ID)
	if err != nil || got.ID != first.ID {
		t.Fatal(err, got.ID)
	}
	_, err = db.Repo.GetWorkload(context.Background(), other, first.ID)
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatal(err)
	}
	var count int
	if err := db.Migration.QueryRow(context.Background(), `SELECT count(*) FROM forge.workloads WHERE name LIKE 'quota-%'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(err, count)
	}
	return Scenario{
		Name: "quota_exhaustion", Recovered: true, Isolated: true, SideEffects: 1, DuplicateSideEffects: 0,
		DurationUS: elapsed(started),
		Limit:      "The cap is the in-process tenant workload limit of one. It is not a distributed quota and it does not change small-v1.",
	}
}

type countingTool struct{ calls int }

func (c *countingTool) Call(context.Context, string, map[string]any) (json.RawMessage, error) {
	c.calls++
	return nil, errors.New("worker lost")
}

type flakyTool struct{ calls int }

func (f *flakyTool) Call(context.Context, string, map[string]any) (json.RawMessage, error) {
	f.calls++
	if f.calls == 1 {
		return nil, errors.New("postgres://user:secret@db/forge down")
	}
	return json.RawMessage(`{"passages":[{"document_id":"platform-brief","passage_id":"passage-1","excerpt":"PostgreSQL owns product records."}]}`), nil
}

type flakyModel struct{ calls int }

func (f *flakyModel) Generate(ctx context.Context, runID, question string, passages []mcp.Passage) (model.Report, error) {
	f.calls++
	if f.calls == 1 {
		return model.Report{}, errors.New("bearer secret-token")
	}
	return model.Fixture{}.Generate(ctx, runID, question, passages)
}
