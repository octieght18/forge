package execution

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"

	"github.com/octieght18/forge/internal/audit"
	"github.com/octieght18/forge/internal/mcp"
	"github.com/octieght18/forge/internal/model"
)

func TestEvaluationFixtures(t *testing.T) {
	if ModeReplay == ModeRetry || ModeRetry == ModeReexecution || ModeReplay == "" {
		t.Fatal(ModeReplay, ModeRetry, ModeReexecution)
	}
	previousTool, previousModel := currentCorpus(), currentModel()
	t.Cleanup(func() {
		SetCorpus(previousTool)
		SetModel(previousModel)
		mcp.Reset()
		model.Reset()
		ResetCommits()
		ResetTerminals()
	})
	t.Run("success", func(t *testing.T) {
		resetEval()
		UseDebugFixtures()
		in := evalInput("55555555-5555-4555-8555-555555555555", []string{"platform-brief"}, strings.Repeat("a", 64))
		got := runEval(t, in)
		if got.Decision != "complete" || got.Outcome != "answered" || len(got.EvidenceIDs) != 1 {
			t.Fatalf("%#v", got)
		}
		records := mcp.EvidenceRecords()
		if len(records) != 1 || records[0].Retention != mcp.RetentionWorkflowHistory || records[0].EvidenceID != got.EvidenceIDs[0] || records[0].RunID != in.RunID {
			t.Fatalf("%#v", records)
		}
		if len(model.Reports()) != 1 || model.Reports()[0].ProviderID != "fixture" {
			t.Fatalf("%#v", model.Reports())
		}
	})
	t.Run("uncertain result", func(t *testing.T) {
		resetEval()
		UseDebugFixtures()
		in := evalInput("66666666-6666-4666-8666-666666666666", []string{"architecture-notes"}, strings.Repeat("b", 64))
		got := runEval(t, in)
		if got.Decision != "complete" || got.Outcome != "insufficient_evidence" || len(got.EvidenceIDs) != 0 {
			t.Fatalf("%#v", got)
		}
		if len(mcp.EvidenceRecords()) != 0 {
			t.Fatalf("%#v", mcp.EvidenceRecords())
		}
		reports := model.Reports()
		if len(reports) != 1 || reports[0].Outcome != "insufficient_evidence" || len(reports[0].Limitations) == 0 {
			t.Fatalf("%#v", reports)
		}
	})
	t.Run("failed step", func(t *testing.T) {
		resetEval()
		counter := &denyCounter{}
		SetCorpus(counter)
		SetModel(model.Fixture{})
		in := evalInput("77777777-7777-4777-8777-777777777777", []string{"platform-brief"}, "short")
		got := runEval(t, in)
		if got.Decision != "fail_closed" || got.Reason != "tool_denied" || counter.calls != 0 || len(mcp.EvidenceRecords()) != 0 {
			t.Fatalf("%#v calls=%d", got, counter.calls)
		}
	})
}

func TestFixtureUsageReconcilesOnce(t *testing.T) {
	audit.Reset()
	previousTool, previousModel := currentCorpus(), currentModel()
	t.Cleanup(func() {
		SetCorpus(previousTool)
		SetModel(previousModel)
		mcp.Reset()
		model.Reset()
		ResetCommits()
	})
	resetEval()
	UseDebugFixtures()
	got := runEval(t, evalInput("45454545-4545-4455-8455-454545454545", []string{"platform-brief"}, strings.Repeat("e", 64)))
	reports := model.Reports()
	prompt, completion, tools := audit.Totals()
	if len(reports) != 1 || prompt != reports[0].Usage.PromptTokens || completion != reports[0].Usage.CompletionTokens || prompt == 0 || tools != 1 || got.Usage.PromptTokens != prompt {
		t.Fatalf("meter %d %d tools %d report %#v result %#v", prompt, completion, tools, reports, got)
	}
	encoded, _ := json.Marshal(audit.Events())
	if strings.Contains(string(encoded), "Which service owns execution?") || strings.Contains(string(encoded), "Bearer") {
		t.Fatal(string(encoded))
	}
}

func TestReplayDoesNotCallToolsAndReexecutionDoes(t *testing.T) {
	previousTool, previousModel := currentCorpus(), currentModel()
	t.Cleanup(func() {
		SetCorpus(previousTool)
		SetModel(previousModel)
		mcp.Reset()
		model.Reset()
		ResetCommits()
	})
	calls := 0
	SetCorpus(countingTool{&calls})
	SetModel(model.Fixture{})
	ResetActivityEntries()
	first := evalInput("88888888-8888-4888-8888-888888888888", []string{"platform-brief"}, strings.Repeat("c", 64))
	before := ActivityEntries()
	runEval(t, first)
	afterFirst := ActivityEntries()
	if afterFirst <= before || calls == 0 {
		t.Fatalf("re-execution did not run activities entries=%d calls=%d", afterFirst, calls)
	}
	second := evalInput("99999999-9999-4999-8999-999999999999", []string{"platform-brief"}, strings.Repeat("c", 64))
	runEval(t, second)
	if second.WorkflowID == first.WorkflowID || ActivityEntries() <= afterFirst || calls < 2 {
		t.Fatal("second execution did not use a new workflow")
	}
	ResetActivityEntries()
	calls = 0
	in := evalInput("33333333-3333-4333-8333-333333333333", []string{"platform-brief"}, strings.Repeat("a", 64))
	history := successHistory(t, in)
	var decoded Correlation
	if err := converter.GetDefaultDataConverter().FromPayloads(history.Events[0].GetWorkflowExecutionStartedEventAttributes().GetInput(), &decoded); err != nil || decoded.WorkflowID != in.WorkflowID {
		t.Fatal(err, decoded)
	}
	if err := Replay(history, in.WorkflowID); err != nil {
		t.Fatal(err)
	}
	if ActivityEntries() != 0 || calls != 0 {
		t.Fatalf("replay called activities entries=%d calls=%d", ActivityEntries(), calls)
	}
}

type countingTool struct{ calls *int }

func (c countingTool) Call(ctx context.Context, name string, args map[string]any) (json.RawMessage, error) {
	*c.calls++
	return mcp.FixtureTool{}.Call(ctx, name, args)
}

type denyCounter struct{ calls int }

func (d *denyCounter) Call(context.Context, string, map[string]any) (json.RawMessage, error) {
	d.calls++
	return nil, nil
}

func resetEval() {
	mcp.Reset()
	model.Reset()
	ResetCommits()
	ResetTerminals()
	ResetActivityEntries()
}

func evalInput(runID string, documents []string, snapshot string) Correlation {
	return Correlation{
		RunID: runID, WorkflowID: "forge-run/" + runID,
		WorkloadID: "11111111-1111-4111-8111-111111111111", VersionID: "22222222-2222-4222-8222-222222222222",
		CommandID: "44444444-4444-4444-8444-444444444444", SnapshotID: snapshot,
		Question: "Which service owns execution?", DocumentIDs: documents, MaxPassages: 8,
	}
}

func runEval(t *testing.T, in Correlation) Result {
	t.Helper()
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(Retrieve)
	env.RegisterActivity(Generate)
	env.RegisterActivity(Compensate)
	env.RegisterActivity(RecordTerminal)
	env.SetStartWorkflowOptions(client.StartWorkflowOptions{ID: in.WorkflowID})
	env.ExecuteWorkflow(Research, in)
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() != nil {
		t.Fatal(env.GetWorkflowError())
	}
	var got Result
	if err := env.GetWorkflowResult(&got); err != nil {
		t.Fatal(err)
	}
	return got
}

func successHistory(t *testing.T, in Correlation) *historypb.History {
	t.Helper()
	history, err := SyntheticSuccessHistory(in)
	if err != nil {
		t.Fatal(err)
	}
	return history
}
